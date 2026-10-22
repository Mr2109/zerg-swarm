package compress_test

// e2e_compress_test.go — 一条端到端集成测：把今夜落的四段串成一条链（**只加测试，不动实现**）。
//
// 四段与各自的落地位置（跑法：`go test -run E2E ./internal/compress ./internal/agent`）：
//
//	① 尺子 → 引擎：eval.AllSamples() 前 3 条（R01/R02/R03）→ compress.Apply(ctx, segs,
//	   Policy{Budget:8192, Engine:"free"})。样本经**唯一入口**进来，测试自己不实现任何剪枝。
//	② 卸载 + 审计：卸载段的 RawPtr 逐条按**内容寻址**复核（sha256(原文)[:16]，同内容必同名 ⇒
//	   「同内容只一份」在命名层的牙）；审计走 compress.EventOf + JSONLSink 一行一事件；
//	   Result 的 Fallback/Mechanism/after<before 逐条钉死。
//	   ★ 「原文真落盘」的那一半写盘口在 agent 侧（ctx_unload.go 的 unloadToolResult），
//	    由 agent 包的 TestE2E_AgentUnloadPersistsAndAudits 拿**同一条样本原文**去钉，
//	    同一次 -run E2E 一并跑；本件只钉「指针怎么来的、指向谁」。
//	③ summary：注入 fake Summarizer（零网络、零算力）⇒ Mechanism=summarize；
//	   注入返回 error 的 ⇒ Fallback=true 且**结果仍是 free 的删减**（不阻塞、不回滚）。
//	④ agent 装配默认关：见 internal/agent/e2e_compress_test.go（TestE2E_AgentSummaryEngineDefaultOff）。
//
// 隔离纪律：ZERG_STATE_DIR 一律 t.TempDir()；真机 <state>/compress-audit.jsonl 的**行数前后必须相等**
// （本件只数它，绝不写它）。基线在 t.Helper 里取，早于任何 t.Setenv。
//
// 变异自证（每条都真跑过：用 `go test -overlay` 把变异体放进**仓外**副本，不动仓内文件，各红在指名的子句上）：
//   · ctx_unload.go 的落盘名改成「内容哈希 ＋ 纳秒」⇒ ② 红在「两层不是同一套内容寻址规则」
//     （并在下一行红在「同内容两次卸载指针不同」）；
//   · 去掉 Apply 阶段二的 `e.Name() != "free"` 闸 ⇒ ① 红在「free 路径不该 Fallback」（阶段二被放进 free 里空转）；
//   · 把摘要失败的 `res.Fallback, res.Reason = true, ...` 吞掉 ⇒ ③b 红在「Fallback 必须为 true」；
//   · writeCtxAudit 的 O_APPEND 改 O_TRUNC ⇒ ② 红在「审计行数 1 ≠ 2（两次卸载两次事件）」。
// 四条都红了 ⇒ 断言不是「谁都过」的摆设；复原后与仓内文件逐字节一致（overlay 只改编译输入）。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
	"github.com/Mr2109/zerg-swarm/core/internal/compress/eval"
)

// e2eBudget —— 今夜四段用的同一档（设计与样本按 8192 出的读数）。
const e2eBudget = 8192

// e2eRealAuditPaths —— 真机审计件的全部候选（**必须在任何 t.Setenv 之前算**）。
//
// 两处都要数，宁可多一条判据：
//   - 外层环境眼里的 state：ZERG_STATE_DIR（设了就是它，与 statepath.Dir() 同口径，但不 import statepath
//     —— 借它会把「本测试临时改过的那个」算成真 state）；
//   - 父任务点名的那一件：<home>/.zerg/state/compress-audit.jsonl（未设 ZERG_STATE_DIR 时的默认落点）。
//
// 两处行数前后都必须相等；任一处不在盘上就记 0（不存在也是「没被写」的一种证据）。
func e2eRealAuditPaths() []string {
	var out []string
	if d := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); d != "" {
		out = append(out, filepath.Join(d, "compress-audit.jsonl"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".zerg", "state", "compress-audit.jsonl")
		dup := false
		for _, q := range out {
			if q == p {
				dup = true
			}
		}
		if !dup {
			out = append(out, p)
		}
	}
	return out
}

// e2eCountLines —— 数一件 JSONL 的行数（不在盘上 = 0，不建、不写）。
func e2eCountLines(t *testing.T, path string) int {
	t.Helper()
	if path == "" {
		return 0
	}
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

// e2eContentName —— 内容寻址的名字：sha256(原文)[:16] + ".txt"（与 ctx_unload.go / eval.finalize 同规）。
//
// 独立重算一遍（不调用任一侧的私有函数）⇒ 断言的是「名字确实由内容决定」，
// 而不是「两侧调了同一个函数」。同内容 ⇒ 同名 ⇒ 盘上只可能一份。
func e2eContentName(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:16]) + ".txt"
}

// e2eSegments —— eval.Sample → []compress.Segment。
//
// 口径照 eval 包内未导出的 Sample.toCompress（外部测试包调不到未导出方法，故按同一字段映射重写）：
// Source 取样本 id（Guard 按 Source 分桶的口径），Tokens 留 0 由 Apply 的 TotalTokens 补。
func e2eSegments(s eval.Sample) []compress.Segment {
	out := make([]compress.Segment, len(s.Segments))
	for i, sg := range s.Segments {
		out[i] = compress.Segment{
			Kind:    sg.Kind,
			Pinned:  sg.Pinned,
			Text:    sg.Text,
			Source:  s.ID,
			RawPtr:  sg.RawPtr,
			Anchors: append([]string(nil), sg.Anchors...),
		}
	}
	return out
}

// TestE2E_CompressChain —— ①②③ 串成一条：free 卸载 → 审计 → summary 注入/失败回退。
func TestE2E_CompressChain(t *testing.T) {
	// 基线（早于任何 t.Setenv）：真机 state 零污染的判据 = 行数前后相等。
	reals := e2eRealAuditPaths()
	realBefore := make([]int, len(reals))
	for i, p := range reals {
		realBefore[i] = e2eCountLines(t, p)
	}
	t.Cleanup(func() {
		for i, p := range reals {
			if got := e2eCountLines(t, p); got != realBefore[i] {
				t.Errorf("真机 state 被污染：%s 行数 %d → %d", p, realBefore[i], got)
			}
			t.Logf("【零污染】%s 行数 %d → %d", p, realBefore[i], e2eCountLines(t, p))
		}
	})

	// 本次测试自己的 state 全在 TempDir：审计只写这里。
	state := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", state)
	auditPath := filepath.Join(state, "compress-audit.jsonl")
	af, err := os.OpenFile(auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("建临时审计件：%v", err)
	}
	defer af.Close()
	sink := compress.NewJSONLSink(af) // 与 ctx_unload.go 的 writeCtxAudit 同一条写面

	samples := eval.AllSamples()
	if len(samples) < 3 {
		t.Fatalf("尺子样本不足 3 条：%d", len(samples))
	}
	first3 := samples[:3]

	ctx := context.Background()
	freePol := compress.Policy{Budget: e2eBudget, Engine: "free"} // ① 今夜那一档（其余字段留零值）
	events := 0
	freeRes := make([]compress.Result, len(first3))

	// ---------------------------------------------------------------- ① + ②
	for i, s := range first3 {
		segs := e2eSegments(s)
		res, err := compress.Apply(ctx, segs, freePol)
		if err != nil {
			t.Fatalf("%s: Apply(free) 返回错误：%v", s.ID, err)
		}
		freeRes[i] = res

		// ②-a：读数关系。before 必须等于尺子自己的口径（同一个 EstimateTokens），after 必须真降。
		if res.Before != s.Tokens() {
			t.Errorf("%s: Before=%d，尺子口径 %d（两把尺子混用）", s.ID, res.Before, s.Tokens())
		}
		if !(res.After < res.Before) {
			t.Errorf("%s: after=%d 未小于 before=%d ⇒ 压了等于没压", s.ID, res.After, res.Before)
		}
		// ②-b：机制与降级。free 路径零算力、必须不降级；撞线（8192×0.5=4096）后是 pressure，
		// 机制为 mask（原文落盘＋指针的无损掩码，或折叠），绝不可能是 summarize。
		if res.Fallback {
			t.Errorf("%s: free 路径不该 Fallback：reason=%q", s.ID, res.Reason)
		}
		if res.Mechanism != "mask" {
			t.Errorf("%s: Mechanism=%q，期望 mask（今夜 free 的实际读数）", s.ID, res.Mechanism)
		}
		if res.Trigger != compress.TriggerPressure {
			t.Errorf("%s: Trigger=%q，期望 pressure（%d/%d 在 0.5–1.0 之间）", s.ID, res.Trigger, res.Before, e2eBudget)
		}
		// Engine=free 时阶段二必须整段跳过（Apply 的 `e.Name() != "free"` 那道闸）：档位永远 light，
		// 绝不会因为「又跑了一遍 free 剪枝」被抬成 balanced。
		if res.Aggressive != "light" {
			t.Errorf("%s: Aggressive=%q，期望 light（free 路径不许进阶段二）", s.ID, res.Aggressive)
		}

		// ②-c：卸载件的内容寻址 —— 逐个卸载段独立重算名字，必须与 RawPtr 的 basename 一致。
		unloaded := 0
		for j, sg := range res.Segments {
			if sg.RawPtr == "" {
				continue // 无指针段（折叠形态）不算「卸载件」，它没有可回查的原文
			}
			if !strings.Contains(sg.Text, sg.RawPtr) {
				t.Errorf("%s seg%d: 卸载形态里没有指针（%q）⇒ 原文不可回查", s.ID, j, e2eTrunc(sg.Text, 40))
			}
			if !strings.Contains(sg.Text, "已卸载") {
				t.Errorf("%s seg%d: 卸载形态没写「已卸载」：%q", s.ID, j, e2eTrunc(sg.Text, 40))
			}
			// 原文取自**输入**的同下标段（PrunerFree 不改顺序、不去重时长度不变；去重另计）
			if j >= len(segs) {
				t.Fatalf("%s: 输出段数 %d 超过输入 %d（下标无法对齐，断言前提不成立）", s.ID, len(res.Segments), len(segs))
			}
			want := e2eContentName(segs[j].Text)
			if got := filepath.Base(sg.RawPtr); got != want {
				t.Errorf("%s seg%d: RawPtr=%q 的内容寻址名 %q ≠ sha256(原文)[:16]=%q（名字不是由内容决定的 ⇒ 同内容会对不上同一份）",
					s.ID, j, sg.RawPtr, got, want)
			}
			unloaded++
		}
		if unloaded == 0 {
			t.Errorf("%s: 一条卸载件都没有（8192 档下 R01–R03 都该卸载大段工具输出）", s.ID)
		}

		// ②-d：审计一行一事件，字段直接搬 Result（EventOf）。
		if err := sink.Emit(compress.EventOf(res, "")); err != nil {
			t.Fatalf("%s: 审计写入失败：%v", s.ID, err)
		}
		events++

		t.Logf("【① 】%s tokens=%d segs=%d → before=%d after=%d mech=%s trigger=%s fallback=%v 卸载件=%d",
			s.ID, s.Tokens(), len(segs), res.Before, res.After, res.Mechanism, res.Trigger, res.Fallback, unloaded)
	}

	// ---------------------------------------------------------------- ③ summary
	// 真源 R01：同一批片段换引擎，只多一个注入面（Summarize），其余 Policy 不变。
	r01 := first3[0]
	r01Free := freeRes[0]

	var mu sync.Mutex
	calls := 0
	fakeSum := func(_ context.Context, segs []compress.Segment) ([]compress.Segment, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		// 契约：交回「等价的更短片段」。锚点/RawPtr 由引擎搬运，这里不许自己造。
		return []compress.Segment{{
			Kind: compress.KindRepresentation,
			Text: "〔摘要〕find 输出已折叠为索引，原文可按指针回查",
		}}, nil
	}
	sumPol := compress.Policy{Budget: e2eBudget, Engine: "summary", Summarize: fakeSum, Deadline: 5 * time.Second}
	sumSegs := e2eSegments(r01)
	sumRes, err := compress.Apply(ctx, sumSegs, sumPol)
	if err != nil {
		t.Fatalf("%s: Apply(summary/注入成功) 返回错误：%v", r01.ID, err)
	}
	if calls == 0 {
		t.Fatal("③ 注入的 Summarizer 一次都没被调用 ⇒ 摘要段根本没走到（CompactPct/阶段二条件被改了？）")
	}
	if sumRes.Mechanism != "summarize" {
		t.Errorf("③ Mechanism=%q，期望 summarize", sumRes.Mechanism)
	}
	if sumRes.Fallback {
		t.Errorf("③ 注入成功却标了 Fallback：reason=%q", sumRes.Reason)
	}
	if !(sumRes.After < r01Free.After) {
		t.Errorf("③ 摘要后 after=%d 未小于 free 的 %d ⇒ 摘要没起作用", sumRes.After, r01Free.After)
	}
	if sumRes.Aggressive != "balanced" {
		t.Errorf("③ 摘要成功档位应为 balanced，实得 %q", sumRes.Aggressive)
	}
	if err := sink.Emit(compress.EventOf(sumRes, "fake")); err != nil {
		t.Fatalf("③ 审计写入失败：%v", err)
	}
	events++
	t.Logf("【③a】%s summary：calls=%d before=%d after=%d mech=%s aggr=%s fallback=%v（free 同批 after=%d）",
		r01.ID, calls, sumRes.Before, sumRes.After, sumRes.Mechanism, sumRes.Aggressive, sumRes.Fallback, r01Free.After)

	// ③-b：注入一个**必失败**的 Summarizer ⇒ 不阻塞、不静默：Fallback=true，但交回的是 free 的删减。
	badSum := func(_ context.Context, _ []compress.Segment) ([]compress.Segment, error) {
		return nil, errors.New("fake 摘要器炸了")
	}
	badPol := compress.Policy{Budget: e2eBudget, Engine: "summary", Summarize: badSum, Deadline: 5 * time.Second}
	badSegs := e2eSegments(r01)
	badRes, err := compress.Apply(ctx, badSegs, badPol)
	if err != nil {
		t.Fatalf("③ 摘要失败不该把 Apply 变成错误（压缩是旁路，主循环必须拿到结果）：%v", err)
	}
	if !badRes.Fallback {
		t.Fatal("③ 摘要器报错，Fallback 必须为 true（否则失败被静默吞掉）")
	}
	if !strings.Contains(badRes.Reason, "fake 摘要器炸了") {
		t.Errorf("③ 失败原因没带出底层错误：%q", badRes.Reason)
	}
	if badRes.Mechanism != r01Free.Mechanism {
		t.Errorf("③ 回退后 Mechanism=%q，应为 free 的 %q", badRes.Mechanism, r01Free.Mechanism)
	}
	if badRes.After != r01Free.After {
		t.Errorf("③ 回退后 after=%d，应等于 free 的 %d ⇒ 结果不是 free 的删减", badRes.After, r01Free.After)
	}
	if len(badRes.Segments) != len(r01Free.Segments) {
		t.Fatalf("③ 回退后段数 %d ≠ free 的 %d", len(badRes.Segments), len(r01Free.Segments))
	}
	for j := range badRes.Segments {
		if badRes.Segments[j].Text != r01Free.Segments[j].Text {
			t.Errorf("③ 回退后 seg%d 文本与 free 不一致（回退不是逐字回退到 free 结果）", j)
		}
	}
	if err := sink.Emit(compress.EventOf(badRes, "fake")); err != nil {
		t.Fatalf("③ 审计写入失败：%v", err)
	}
	events++
	t.Logf("【③b】%s summary 失败回退：after=%d mech=%s fallback=%v reason=%q",
		r01.ID, badRes.After, badRes.Mechanism, badRes.Fallback, badRes.Reason)

	// ---------------------------------------------------------------- ② 审计侧收口
	if err := af.Sync(); err != nil {
		t.Fatalf("审计件落盘：%v", err)
	}
	got := e2eReadEvents(t, auditPath)
	if len(got) != events {
		t.Fatalf("审计件行数 %d ≠ 本次事件数 %d（一行一事件、只增不减）", len(got), events)
	}
	for i, ev := range got {
		if ev.Phase != "completed" || ev.At.IsZero() {
			t.Errorf("审计第 %d 行形态不对：phase=%q at=%v", i+1, ev.Phase, ev.At)
		}
		if ev.Before <= 0 || ev.After < 0 {
			t.Errorf("审计第 %d 行读数不对：before=%d after=%d", i+1, ev.Before, ev.After)
		}
	}
	// 最后一行 = 失败回退那次：审计必须能分辨「降级」与「正常」。
	last := got[len(got)-1]
	if !last.Fallback || last.Mechanism != r01Free.Mechanism || last.Reason == "" {
		t.Errorf("审计末行（失败回退）字段不对：fallback=%v mech=%q reason=%q", last.Fallback, last.Mechanism, last.Reason)
	}
	t.Logf("【② 】审计件 %s：%d 行（%d 次 Apply 一一对应），末行 fallback=%v mech=%s", auditPath, len(got), events, last.Fallback, last.Mechanism)
}

// e2eReadEvents —— 读回审计件（逐行 json.Unmarshal 成 compress.Event，解不开即失败）。
func e2eReadEvents(t *testing.T, path string) []compress.Event {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("审计件读不回来：%v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	evs := make([]compress.Event, 0, len(lines))
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var ev compress.Event
		if err := json.Unmarshal([]byte(ln), &ev); err != nil {
			t.Fatalf("审计第 %d 行不是合法 JSON：%v（%q）", i+1, err, ln)
		}
		evs = append(evs, ev)
	}
	return evs
}

// e2eTrunc —— 报错时截短文本（避免把整段工具输出灌进失败信息）。
func e2eTrunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
