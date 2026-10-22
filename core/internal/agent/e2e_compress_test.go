package agent

// e2e_compress_test.go — 压缩链的 **agent 侧两段**（与 compress 包的 TestE2E_CompressChain 同一条链）。
//
// 为什么这一段必须在本包：链路的两头只有这里能看到 ——
//   · 「原文真落盘」的写盘口是 ctx_unload.go 的 unloadToolResult（内容寻址目录 + 审计追加），
//     它是未导出的，外部测试包调不到；
//   · 「摘要引擎默认不建实例」的装配点是 summary_engine.go 的 resolveSummaryEngine + NewAgent，
//     enabled/policy 也在未导出的结构体里。
//
// 两件事都不打桩：素材取 eval.AllSamples() 里**真样本的真原文**（不是自己编一串 a），
// 盘上用 t.TempDir() 起一个假 state，审计行数与文件数都是**真读盘**。
//
// 隔离纪律：ZERG_STATE_DIR 一律指 t.TempDir()（t.Setenv 自带还原）⇒ 真机 ~/.zerg/state 一行不写；
// 真机审计件在 cleanup 里前后对比行数，必须相等。
//
// 跑法：`go test -run E2E ./internal/compress ./internal/agent`

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
	"github.com/Mr2109/zerg-swarm/core/internal/compress/eval"
)

// e2eRealAuditCount —— 真机审计件行数（**只读**；不在盘上 = 0）。
//
// 两处都数：外层环境的 ZERG_STATE_DIR（若设了）与父任务点名的 <home>/.zerg/state/compress-audit.jsonl。
// 必须在任何 t.Setenv 之前调用，否则数到的是本测试自己的临时 state。
func e2eRealAuditCount(t *testing.T) map[string]int {
	t.Helper()
	out := map[string]int{}
	var paths []string
	if d := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); d != "" {
		paths = append(paths, filepath.Join(d, "compress-audit.jsonl"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".zerg", "state", "compress-audit.jsonl")
		dup := false
		for _, q := range paths {
			if q == p {
				dup = true
			}
		}
		if !dup {
			paths = append(paths, p)
		}
	}
	for _, p := range paths {
		out[p] = e2eLines(t, p)
	}
	return out
}

// e2eLines —— 数一件 JSONL 的行数（不存在 = 0，不建、不写）。
func e2eLines(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("真机审计件读不回来：%v（%s）", err, path)
	}
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// e2eBigSampleSegment —— 取尺子第一条样本里「可卸载（带 raw_ptr）且超 4 KiB」的那一段原文。
//
// 用真素材的理由：卸载闸是按字节判的（ctxUnloadBytes = 4 KiB），自己造的字符串只能证明
// 「闸会开」，证明不了「尺子标的 raw_ptr 与真落盘的名字是同一套规则」。
func e2eBigSampleSegment(t *testing.T) (text, rawPtr, sampleID string) {
	t.Helper()
	for _, s := range eval.AllSamples() {
		for _, sg := range s.Segments {
			if sg.RawPtr != "" && len(sg.Text) > ctxUnloadBytes {
				return sg.Text, sg.RawPtr, s.ID
			}
		}
	}
	t.Fatal("尺子里没有「带 raw_ptr 且超 4 KiB」的段 —— 本测的前提不成立")
	return "", "", ""
}

// TestE2E_AgentUnloadPersistsAndAudits —— ② 的写盘半边：原文真落盘（内容寻址、同内容只一份）＋ 审计追加。
//
// 四问四答（全部真读盘）：
//  1. 落没落？—— 指针里的路径真在盘上，字节与输入逐字节一致；
//  2. 名字是不是由内容定？—— basename == sha256(原文)[:16].txt，且与**尺子给的 raw_ptr 同名**
//     （两侧同一套规则 ⇒ 「同内容只一份」在跨层也成立）；
//  3. 同内容会不会写两份？—— 连卸两次，ctx-unload 目录里仍然只有 1 个文件；
//  4. 审计有没有记？—— 两次卸载 = 两行 unload 事件（事件按次数记，文件按内容去重），字段可读。
func TestE2E_AgentUnloadPersistsAndAudits(t *testing.T) {
	// 真机基线：早于 t.Setenv 取。
	realBefore := e2eRealAuditCount(t)
	t.Cleanup(func() {
		for p, n := range realBefore {
			if got := e2eLines(t, p); got != n {
				t.Errorf("真机 state 被污染：%s 行数 %d → %d", p, n, got)
			}
			t.Logf("【零污染】%s 行数 %d → %d", p, n, e2eLines(t, p))
		}
	})

	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	text, rawPtr, sampleID := e2eBigSampleSegment(t)

	// ① 落盘 + 指针。
	ptr := unloadToolResult(text)
	if ptr == text {
		t.Fatal("超 4 KiB 的工具输出没被卸载（原样返回）")
	}
	if !strings.Contains(ptr, "已卸载") {
		t.Fatalf("指针没写明卸载：%q", ptr)
	}
	idx := strings.Index(ptr, state)
	if idx < 0 {
		t.Fatalf("指针没带落点路径（%s）：%q", state, ptr)
	}
	landed := strings.TrimSpace(strings.TrimSuffix(ptr[idx:], "〕"))
	b, err := os.ReadFile(landed)
	if err != nil {
		t.Fatalf("原文读不回来：%v（%s）", err, landed)
	}
	if string(b) != text {
		t.Fatalf("原文与输入不一致：%d B vs %d B ⇒ 卸载不可逆", len(b), len(text))
	}
	if !strings.HasPrefix(landed, filepath.Join(state, "ctx-unload")+string(os.PathSeparator)) {
		t.Errorf("卸载件不在 <state>/ctx-unload 下：%s", landed)
	}

	// ② 内容寻址：名字由内容决定，且与尺子（eval.finalize）给的名字同规。
	if got, want := filepath.Base(landed), filepath.Base(rawPtr); got != want {
		t.Errorf("落盘名 %q ≠ 尺子 raw_ptr 的 basename %q（两层不是同一套内容寻址规则）", got, want)
	}

	// ③ 同内容只一份：再卸一次，目录里仍只有 1 个文件、指针逐字相同。
	ptr2 := unloadToolResult(text)
	if ptr2 != ptr {
		t.Fatalf("同内容两次卸载指针不同：%q vs %q", ptr, ptr2)
	}
	files := e2eListFiles(t, filepath.Join(state, "ctx-unload"))
	if len(files) != 1 {
		t.Fatalf("同内容卸两次落成 %d 个文件（应 1 个：内容寻址、只存一份）：%v", len(files), files)
	}

	// ④ 审计：事件按**次数**记（2 次卸载 = 2 行），文件按**内容**去重（1 份）—— 两个口径不能混。
	audit := filepath.Join(state, "compress-audit.jsonl")
	lines := e2eReadAudit(t, audit)
	if len(lines) != 2 {
		t.Fatalf("审计行数 %d ≠ 2（两次卸载两次事件）", len(lines))
	}
	for i, ev := range lines {
		if ev.Mechanism != "unload" {
			t.Errorf("审计第 %d 行 Mechanism=%q，期望 unload", i+1, ev.Mechanism)
		}
		if ev.Phase != "completed" || ev.Trigger != compress.TriggerPressure {
			t.Errorf("审计第 %d 行形态不对：phase=%q trigger=%q", i+1, ev.Phase, ev.Trigger)
		}
		if ev.Before != compress.EstimateTokens(text) {
			t.Errorf("审计第 %d 行 Before=%d，期望原文 token %d", i+1, ev.Before, compress.EstimateTokens(text))
		}
		if !(ev.After < ev.Before) {
			t.Errorf("审计第 %d 行 after=%d 未小于 before=%d", i+1, ev.After, ev.Before)
		}
		if ev.Fallback {
			t.Errorf("审计第 %d 行标了 fallback（无损卸载不该降级）", i+1)
		}
	}

	// ⑤ 闭环：这条真落盘的原文交给 compress，卸载形态里必须带着**真路径**，且能从该路径逐字节取回。
	//    这一段把「尺子 → agent 落盘 → compress 编排」缝在一起（③ 三段各说各话就断在这）。
	segs := []compress.Segment{
		{Kind: compress.KindObservation, Text: text, Source: sampleID, RawPtr: landed,
			Anchors: []string{filepath.Base(landed)}},
		{Kind: compress.KindTrajectory, Text: "决策：读完大文件后继续核对", Source: sampleID},
	}
	res, err := compress.Apply(t.Context(), segs, compress.Policy{Budget: 8192, Engine: "free"})
	if err != nil {
		t.Fatalf("Apply(free)：%v", err)
	}
	if res.Fallback || !(res.After < res.Before) {
		t.Fatalf("free 编排没压成：before=%d after=%d fallback=%v reason=%q", res.Before, res.After, res.Fallback, res.Reason)
	}
	kept := ""
	for _, sg := range res.Segments {
		if strings.Contains(sg.Text, landed) {
			kept = sg.Text
		}
	}
	if kept == "" {
		t.Fatalf("压缩结果里没有带真落盘路径的卸载形态（原文不可回查）：%+v", res.Segments)
	}
	if back, err := os.ReadFile(landed); err != nil || string(back) != text {
		t.Fatalf("按结果里的指针回查不到原文：err=%v len=%d", err, len(back))
	}
	t.Logf("【② 写盘】%s：原文 %d B → 落盘 %s（%d 个文件/2 次卸载，审计 %d 行），free 编排 %d→%d token 仍带指针",
		sampleID, len(text), filepath.Base(landed), len(files), len(lines), res.Before, res.After)
}

// e2eListFiles —— 目录里的文件名（不存在 = 空；出错即失败）。
func e2eListFiles(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("目录读不回来：%v（%s）", err, dir)
	}
	out := make([]string, 0, len(ents))
	for _, e := range ents {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// TestE2E_AgentSummaryEngineDefaultOff —— ④：两态各跑一次 NewAgent，后者**不建实例**。
//
//	态 A：ZERG_SUMMARY_ENGINE 未设（清空 ⇒ 与未设同判，summaryGateOn 只认 1/true/yes/on）
//	态 B：ZERG_SUMMARY_ENGINE=1 但端点/模型都缺
//
// 两态都必须：enabled=false、policy.Engine=free、policy.Summarize=nil、启动日志明写「摘要引擎未启用（默认 free）」。
// 态 B 还要点名缺的那两个 env —— 现场能判因，而不是看到一句「没启用」去猜。
//
// ★ 这条是「默认零算力」的牙：默认路径若真建了实例，压缩就从「省 token」变成「烧 token 的隐藏成本」。
func TestE2E_AgentSummaryEngineDefaultOff(t *testing.T) {
	realBefore := e2eRealAuditCount(t)
	t.Cleanup(func() {
		for p, n := range realBefore {
			if got := e2eLines(t, p); got != n {
				t.Errorf("真机 state 被污染：%s 行数 %d → %d", p, n, got)
			}
		}
	})

	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	// 先把六项清空：不靠跑测那台机器上的既有取值。
	for _, k := range []string{
		envSummaryEngine, envSummaryEndpoint, envSummaryModel, envSummaryToken,
		envSummaryTimeoutS, envSummaryMaxTokens,
	} {
		t.Setenv(k, "")
	}

	// 态 A：未设总闸。
	a := NewAgent(Config{WorkDir: t.TempDir(), ContextWindow: 1})
	if a.summaryEngine.enabled {
		t.Fatalf("未设 %s 却建了实例（默认必须 free、零网络）", envSummaryEngine)
	}
	if a.summaryEngine.policy.Engine != "free" {
		t.Errorf("态 A policy.Engine=%q，期望 free", a.summaryEngine.policy.Engine)
	}
	if a.summaryEngine.policy.Summarize != nil {
		t.Error("态 A 注入了 Summarizer（未启用却有调用面 = 默认会烧模型）")
	}
	if a.summaryEngine.status != summaryEngineOffStatus {
		t.Errorf("态 A status=%q，期望 %q", a.summaryEngine.status, summaryEngineOffStatus)
	}
	if !strings.Contains(a.summaryEngine.logLine(), summaryEngineOffStatus) {
		t.Errorf("态 A 启动日志没明写「%s」：%q", summaryEngineOffStatus, a.summaryEngine.logLine())
	}
	t.Logf("【④A】未设 %s：enabled=%v engine=%s summarize=nil? %v log=%q",
		envSummaryEngine, a.summaryEngine.enabled, a.summaryEngine.policy.Engine,
		a.summaryEngine.policy.Summarize == nil, a.summaryEngine.logLine())

	// 态 B：总闸开了，但端点/模型都缺 ⇒ 仍不建实例，且点名缺项。
	t.Setenv(envSummaryEngine, "1")
	b := NewAgent(Config{WorkDir: t.TempDir(), ContextWindow: 1})
	if b.summaryEngine.enabled {
		t.Fatalf("%s=1 但缺端点/模型，仍然建了实例（会每次调用白烧一次 400）", envSummaryEngine)
	}
	if b.summaryEngine.policy.Engine != "free" {
		t.Errorf("态 B policy.Engine=%q，期望 free（空引擎回落）", b.summaryEngine.policy.Engine)
	}
	if b.summaryEngine.policy.Summarize != nil {
		t.Error("态 B 注入了 Summarizer（缺配置却建了实例）")
	}
	for _, want := range []string{envSummaryEndpoint, envSummaryModel} {
		if !strings.Contains(b.summaryEngine.reason, want) {
			t.Errorf("态 B 未启用的原因没点名 %s：%q", want, b.summaryEngine.reason)
		}
	}
	if b.summaryEngine.status != summaryEngineOffStatus {
		t.Errorf("态 B status=%q，期望 %q（缺配置 = 未启用，不是半启用）", b.summaryEngine.status, summaryEngineOffStatus)
	}
	t.Logf("【④B】%s=1 缺端点/模型：enabled=%v engine=%s reason=%q",
		envSummaryEngine, b.summaryEngine.enabled, b.summaryEngine.policy.Engine, b.summaryEngine.reason)
}

// e2eReadAudit —— 逐行解审计件（解不开即失败；空件 = nil）。
func e2eReadAudit(t *testing.T, path string) []compress.Event {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("审计件读不回来：%v（%s）", err, path)
	}
	var out []compress.Event
	for i, ln := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var ev compress.Event
		if err := json.Unmarshal([]byte(ln), &ev); err != nil {
			t.Fatalf("审计第 %d 行不是合法 JSON：%v（%q）", i+1, err, ln)
		}
		out = append(out, ev)
	}
	return out
}
