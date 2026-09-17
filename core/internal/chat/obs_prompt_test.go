// obs_prompt_test.go — T3.2 + T3.3 + T3.5 实证：约束保留率 / 分段落账 / 时钟与种子 + 提示纯净度守卫
//
// 要治的盲区（改本文件前先读）：
//   - 「长对话后模型不再照做」只能靠人感觉（H2）—— 本文件把它钉成**可判定的数字**：
//     total / present / missing_ids，且必须能证明「是**压缩**弄丢的」而不是"本来就没说"；
//   - 「提示变了」只有整体哈希 ⇒ 无法定位变了哪一段；且指纹对象错（应对渲染后实际发送的字节取哈希）（H3）；
//   - 时间/随机混进提示 ⇒ 回放与回归在定义上不成立（H6）—— 本文件让"提醒里漏了 now()"**可判定**。
//
// 用例 6 类（+ 2 条口径守卫）+ 反例：
//
//	① 同 clock+seed 两次渲染 ⇒ rendered_prefix_hash 相同（决定论），且换身份不影响哈希（渲染不吃时钟）
//	② 改任意一段 ⇒ **只有该段** sha256 变（其余段逐字节不变）；记忆块注入 ⇒ 两哈希不等且指得出是 memory
//	③【关键】压缩掉一条 must_survive ⇒ missing_ids 非空 + 落告警（含"压缩前 present=1"的对照）
//	④ 无缺失 ⇒ missing_ids 为空**且不告警**（含"只带指纹不带原文"也认作在场）
//	⑤ 时间戳形态混进提醒/模板 ⇒ 落 prompt_impurity；且同一份输入换时钟 ⇒ 渲染哈希随时钟变（非决定论可判）
//	⑥ 反例：正常中文提醒（含数字/版本号/文件名）**不得**被误判；工具结果/记忆里的时间**豁免**（H6 合法通道）
//
// 纪律（与 obs_test.go / obs_compaction_test.go 同源）：
//   - 观测一律落隔离目录：t.Setenv("ZERG_STATE_DIR", t.TempDir())；登记表有**进程内缓存** ⇒ 用例必须
//     resetConstraintRegistryCache()（否则同进程内换目录会读到别的用例的表）。
//   - 判据分两层：**wire 层**（键真的在 JSONL 原文里）+ **read 层**（解回来的值是真的）——只断 Go 字段=自证。
//   - 独立复算：约束 id/指纹、分段哈希、渲染前缀哈希都由**测试侧的独立口径**重算（不用被测 helper）。
package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── 读回结构 ──────────────────────────────────────────────────────────

// promptObsLine — 本族事件的读回结构（键缺席 ⇒ 指针为 nil，这正是"这类量没落"的判据）
type promptObsLine struct {
	TS                string                 `json:"ts"`
	Kind              string                 `json:"kind"`
	Session           string                 `json:"session"`
	Round             int                    `json:"round"`
	Model             string                 `json:"model"`
	EventName         string                 `json:"event_name"`
	TraceID           string                 `json:"trace_id"`
	SpanID            string                 `json:"span_id"`
	ParentSpanID      string                 `json:"parent_span_id"`
	SpanKind          string                 `json:"span_kind"`
	ClockISO          string                 `json:"clock_iso"`
	RequestSeed       *int64                 `json:"request_seed"`
	Constraints       *ConstraintCheckObs    `json:"constraints"`
	ConstraintMissing []ConstraintMissingObs `json:"constraint_missing"`
	Prompt            *PromptLedgerObs       `json:"prompt"`
	Impurity          *PromptImpurityObs     `json:"impurity"`
}

// promptIsolate — 隔离状态目录 + 重置登记表进程内缓存（测试在同一进程里换目录，缓存必须丢）
func promptIsolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	resetConstraintRegistryCache()
	t.Cleanup(resetConstraintRegistryCache)
	return dir
}

// readPromptObsLines — 读回观测文件全部行（读侧宽容：别的 kind 也解，不认识的键忽略）
func readPromptObsLines(t *testing.T) []promptObsLine {
	t.Helper()
	raw := readObs(t)
	if raw == "" {
		return nil
	}
	var out []promptObsLine
	for _, ln := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var r promptObsLine
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			t.Fatalf("观测行不是合法 JSON（读侧必须宽容）：%v\n%s", err, ln)
		}
		out = append(out, r)
	}
	return out
}

// promptEvents — 按事件名挑行（保持落盘顺序）
func promptEvents(lines []promptObsLine, event string) []promptObsLine {
	var out []promptObsLine
	for _, ln := range lines {
		if ln.EventName == event {
			out = append(out, ln)
		}
	}
	return out
}

// ── 测试侧独立复算（不是被测 helper；口径写死在 obs_prompt.go 文件头）────────

// pfRenderTools — 独立复算 tools 段渲染（逐条 JSON + \n）
func pfRenderTools(tools []map[string]any) string {
	var b strings.Builder
	for _, t := range tools {
		j, _ := json.Marshal(t)
		b.Write(j)
		b.WriteByte('\n')
	}
	return b.String()
}

// pfRenderHistory — 独立复算 history 段渲染（role \x00 content [\x00 reasoning_content] \n）
func pfRenderHistory(msgs []map[string]any) string {
	var b strings.Builder
	for _, m := range msgs {
		b.WriteString(strAny(m["role"]))
		b.WriteByte(0)
		b.WriteString(strAny(m["content"]))
		if rc, ok := m["reasoning_content"]; ok {
			b.WriteByte(0)
			b.WriteString(strAny(rc))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// strAny — 任意值 → 稳定字符串（字符串原样；其它走 JSON）
func strAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	j, _ := json.Marshal(v)
	return string(j)
}

// pfSHA — 独立复算 sha256 十六进制
func pfSHA(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// pfPrefixHash — 独立复算渲染后 prefix 哈希（system ‖ history ‖ tools，发送序）
func pfPrefixHash(system, hist, tools string) string {
	return pfSHA(system + "\x00" + hist + "\x00" + tools)
}

// pfConstraintID — 独立复算约束 id（= "uc-" + sha256(canonical) 前 12 位）
func pfConstraintID(canonical string) string {
	return "uc-" + pfSHA(canonical)[:12]
}

// pfCanonical — 独立复算规范化（去零宽、全角空格→半角、空白折叠、trim）——只做测试需要的三种正规化
func pfCanonical(s string) string {
	r := strings.NewReplacer("\u200b", "", "\u3000", " ", "\n", " ", "\t", " ", "\r", " ")
	return strings.Join(strings.Fields(r.Replace(s)), " ")
}

// pfRenderMsgs — 把库里的消息渲染成"实际发给模型"的形态（与 api 侧 chatMessageToReq 的 role/content 口径一致）
func pfRenderMsgs(msgs []*Message) []map[string]any {
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, map[string]any{"role": m.Role, "content": m.Content})
	}
	return out
}

// pfMustSeg — 取某分段账目（缺段即失败：四段永远齐全）
func pfMustSeg(t *testing.T, ledger *PromptLedgerObs, id string) PromptSegmentObs {
	t.Helper()
	if ledger == nil {
		t.Fatal("事件缺 prompt 账目块")
	}
	for _, s := range ledger.Segments {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("分段账目缺 %q 段（四段必须齐全）：%+v", id, ledger.Segments)
	return PromptSegmentObs{}
}

// pfBaseRender — 一份典型的"实际发出的提示"（四段齐全 + 一段工具定义）
func pfBaseRender(session string, round int) PromptRender {
	return PromptRender{
		Session: session, Round: round, Model: "gemma-12B",
		System: "TEMPLATE-BASE\n\n# 你的身份\n- 虫族本地集群",
		Memory: "",
		Tools: []map[string]any{
			{"type": "function", "function": map[string]any{"name": "kb_search", "description": "检索知识库"}},
		},
		History: []map[string]any{
			{"role": "user", "content": "看一下设计稿"},
			{"role": "assistant", "content": "好的", "reasoning_content": "先读文件"},
		},
		Identity: FixedRequestIdentity("2026-09-17T12:00:00Z", 42),
	}
}

// ── 用例 ①：同 clock+seed 两次渲染 ⇒ rendered_prefix_hash 相同（决定论）──────

func TestPromptLedgerDeterministic(t *testing.T) {
	promptIsolate(t)

	r := pfBaseRender("sess-T33-1", 1)
	ObservePromptCheck(r)
	ObservePromptCheck(r) // 同一份字节 + 同一个身份

	lines := readPromptObsLines(t)
	checks := promptEvents(lines, obsEventConstraintCheck)
	if len(checks) != 2 {
		t.Fatalf("每次请求应恰好一条 constraint_check，实得 %d 条", len(checks))
	}
	a, b := checks[0], checks[1]
	if a.Prompt == nil || b.Prompt == nil {
		t.Fatal("constraint_check 必须带 prompt 账目块（T3.3）")
	}
	// 决定论：两次渲染（同身份）逐字段相同
	if a.Prompt.RenderedPrefixHash != b.Prompt.RenderedPrefixHash {
		t.Errorf("同 clock+seed 两次渲染 ⇒ rendered_prefix_hash 必须相同：%s vs %s",
			a.Prompt.RenderedPrefixHash, b.Prompt.RenderedPrefixHash)
	}
	if a.Prompt.TemplateHash != b.Prompt.TemplateHash {
		t.Errorf("同 clock+seed 两次渲染 ⇒ template_hash 必须相同：%s vs %s",
			a.Prompt.TemplateHash, b.Prompt.TemplateHash)
	}
	for _, id := range []string{segmentSystem, segmentTools, segmentHistory, segmentMemory} {
		sa, sb := pfMustSeg(t, a.Prompt, id), pfMustSeg(t, b.Prompt, id)
		if sa != sb {
			t.Errorf("分段 %s 两次渲染应逐字段相同：%+v vs %+v", id, sa, sb)
		}
	}
	// 独立复算：渲染后 prefix 哈希（system ‖ history ‖ tools）
	wantPrefix := pfPrefixHash(r.System, pfRenderHistory(r.History), pfRenderTools(r.Tools))
	if a.Prompt.RenderedPrefixHash != wantPrefix {
		t.Errorf("rendered_prefix_hash=%s，独立复算 %s", a.Prompt.RenderedPrefixHash, wantPrefix)
	}
	// 指纹对象是**渲染后的字节**：改一个字节 ⇒ 哈希必须变（否则说明算的是别的对象）
	moved := r
	moved.System = r.System + " "
	ObservePromptCheck(moved)
	checks = promptEvents(readPromptObsLines(t), obsEventConstraintCheck)
	if checks[len(checks)-1].Prompt.RenderedPrefixHash == wantPrefix {
		t.Error("系统提示改一个字节后 rendered_prefix_hash 未变 ⇒ 指纹对象不是渲染后的字节")
	}
	// 渲染**不吃时钟**：同输入换身份 ⇒ 哈希不变（时间只进事件，不进提示）
	reidentified := r
	reidentified.Identity = FixedRequestIdentity("2031-01-01T00:00:01Z", 999999)
	ObservePromptCheck(reidentified)
	checks = promptEvents(readPromptObsLines(t), obsEventConstraintCheck)
	last := checks[len(checks)-1]
	if last.Prompt.RenderedPrefixHash != wantPrefix {
		t.Errorf("换 clock/seed 不应改变渲染哈希（时间只进事件不进提示）：%s vs %s",
			last.Prompt.RenderedPrefixHash, wantPrefix)
	}
	if last.ClockISO != "2031-01-01T00:00:01Z" || last.RequestSeed == nil || *last.RequestSeed != 999999 {
		t.Errorf("clock_iso/request_seed 必须按注入值落（回放基线）：%q / %v", last.ClockISO, last.RequestSeed)
	}
	// 身份随请求：不同轮次 ⇒ 不同种子（由 NewRequestIdentity 推出）；同三元组 ⇒ 同种子（可复算）
	t0 := mustParseTime(t, "2026-09-17T12:00:00Z")
	if identityFor("s", 1, t0) != identityFor("s", 1, t0) {
		t.Error("request_seed 必须由 (会话,轮次,时钟) 可复算")
	}
	if identityFor("s", 1, t0).Seed == identityFor("s", 2, t0).Seed {
		t.Error("不同轮次应推出不同种子（否则同会话各轮无法区分）")
	}

	// wire 层：任务要求的键名必须真的在 JSONL 原文里
	raw := readObs(t)
	for _, key := range []string{
		`"kind":"prompt"`, `"event_name":"constraint_check"`,
		`"constraints"`, `"total"`, `"present"`, `"missing_ids"`,
		`"segments"`, `"id":"system"`, `"id":"tools"`, `"id":"history"`, `"id":"memory"`,
		`"rendered_prefix_hash"`, `"template_hash"`, `"dynamic_injection"`, `"dynamic_parts"`,
		`"clock_iso"`, `"request_seed"`,
		`"trace_id"`, `"span_id"`, `"parent_span_id"`, `"span_kind"`,
	} {
		if !strings.Contains(raw, key) {
			t.Errorf("观测原文缺字段 %s\n原文：%s", key, raw)
		}
	}
}

// ── 用例 ②：改任意一段 ⇒ 只有该段 sha256 变（定位精确）─────────────────────

func TestPromptSegmentHashesIsolate(t *testing.T) {
	promptIsolate(t)

	base := pfBaseRender("sess-T33-2", 1)
	base.Memory = "MEM-BLOCK"
	base.System = base.System + "\n\n" + base.Memory // 记忆块嵌在系统提示 volatile 层（真实形态）
	ObservePromptCheck(base)
	baseLine := promptEvents(readPromptObsLines(t), obsEventConstraintCheck)[0]
	if baseLine.Prompt == nil {
		t.Fatal("缺 prompt 账目块")
	}
	baseSeg := map[string]PromptSegmentObs{}
	for _, id := range []string{segmentSystem, segmentTools, segmentHistory, segmentMemory} {
		baseSeg[id] = pfMustSeg(t, baseLine.Prompt, id)
	}
	// 独立复算四段哈希
	if got := baseSeg[segmentSystem].SHA256; got != pfSHA(base.System) {
		t.Errorf("system 段指纹应为 sha256(系统提示原文)：%s vs %s", got, pfSHA(base.System))
	}
	if got := baseSeg[segmentTools].SHA256; got != pfSHA(pfRenderTools(base.Tools)) {
		t.Errorf("tools 段指纹复算不一致：%s", got)
	}
	if got := baseSeg[segmentHistory].SHA256; got != pfSHA(pfRenderHistory(base.History)) {
		t.Errorf("history 段指纹复算不一致：%s", got)
	}
	if got := baseSeg[segmentMemory].SHA256; got != pfSHA(base.Memory) {
		t.Errorf("memory 段指纹复算不一致：%s", got)
	}
	// 动态注入：记忆块在系统提示里 ⇒ 两哈希不等，且 dynamic_parts 指得出是它
	if !baseLine.Prompt.DynamicInjection || len(baseLine.Prompt.DynamicParts) != 1 ||
		baseLine.Prompt.DynamicParts[0] != segmentMemory {
		t.Errorf("记忆块注入 ⇒ dynamic_injection=true 且 dynamic_parts=[memory]：%+v", baseLine.Prompt)
	}
	if baseLine.Prompt.MemoryEmbedded != true {
		t.Error("记忆块确实嵌在系统提示里 ⇒ memory_embedded 应为 true")
	}

	// 变体：每次只改一段 ⇒ 只有该段 sha256 变
	// （memory 是**嵌在 system 里**的 ⇒ 改记忆块必然同时改 system —— 这是口径里写死的重叠，
	//   不是缺陷；用例把"该变的"逐段列出来，就是为了不让它变成"反正都会变"的模糊断言）
	variants := []struct {
		name string
		mut  func(r *PromptRender)
		segs []string
	}{
		{"改 system", func(r *PromptRender) { r.System = r.System + "\n- 新增一行" }, []string{segmentSystem}},
		{"改 tools", func(r *PromptRender) {
			r.Tools = append(r.Tools, map[string]any{"type": "function", "function": map[string]any{"name": "new_tool"}})
		}, []string{segmentTools}},
		{"改 history", func(r *PromptRender) {
			r.History = append(r.History, map[string]any{"role": "user", "content": "再问一句"})
		}, []string{segmentHistory}},
		{"改 memory", func(r *PromptRender) {
			r.Memory = r.Memory + "\n- 新增记忆"
			r.System = r.System + "\n- 新增记忆" // 系统提示里的那一份同步改（记忆块 ⊂ 系统提示）
		}, []string{segmentMemory, segmentSystem}},
	}
	for _, v := range variants {
		r := base
		v.mut(&r)
		ObservePromptCheck(r)
		lines := promptEvents(readPromptObsLines(t), obsEventConstraintCheck)
		got := lines[len(lines)-1]
		if got.Prompt == nil {
			t.Fatalf("%s：缺 prompt 账目块", v.name)
		}
		changed := map[string]bool{}
		for id, want := range baseSeg {
			seg := pfMustSeg(t, got.Prompt, id)
			if seg != want {
				changed[id] = true
			}
		}
		for _, id := range v.segs {
			if !changed[id] {
				t.Errorf("%s：%s 段指纹**必须**变，实得未变 %s", v.name, id, baseSeg[id].SHA256)
			}
		}
		for id := range baseSeg {
			if pfHasStr(v.segs, id) {
				continue
			}
			if changed[id] {
				t.Errorf("%s：只有 %v 该变，但 %s 段也变了：%+v → %+v",
					v.name, v.segs, id, baseSeg[id], pfMustSeg(t, got.Prompt, id))
			}
		}
		// 动态注入的归属不因这次改动而漂移（仍指得出是 memory）
		if len(got.Prompt.DynamicParts) != 1 || got.Prompt.DynamicParts[0] != segmentMemory {
			t.Errorf("%s：dynamic_parts 应恒为 [memory]：%+v", v.name, got.Prompt.DynamicParts)
		}
		// 渲染后 prefix 哈希算的是 real 字节 ⇒ 表内的段被改就一定跟着变
		if got.Prompt.RenderedPrefixHash == baseLine.Prompt.RenderedPrefixHash {
			t.Errorf("%s：改了 %v 段却没动 rendered_prefix_hash（它算的就是渲染后字节）", v.name, v.segs)
		}
	}

	// 记忆块注入把 template_hash 拉到"无注入"的样子 ⇒ 两哈希的差就是它（H3 的"看出存在动态注入"）
	noMem := base
	noMem.Memory = ""
	noMem.System = "TEMPLATE-BASE\n\n# 你的身份\n- 虫族本地集群"
	ObservePromptCheck(noMem)
	lines := promptEvents(readPromptObsLines(t), obsEventConstraintCheck)
	noMemLine := lines[len(lines)-1]
	if noMemLine.Prompt.DynamicInjection {
		t.Error("无记忆块 ⇒ 两哈希应相等（无动态注入）")
	}
	if noMemLine.Prompt.MemoryEmbedded {
		t.Error("记忆块为空 ⇒ memory_embedded 应为 false（不是「判准了无注入」）")
	}
	// 记忆块变了但系统提示里找不到（冻结后又取块）⇒ 判不准，必须显式标出（不许当"无注入"用）
	drift := noMem
	drift.Memory = "MEM-未嵌进系统提示"
	ObservePromptCheck(drift)
	lines = promptEvents(readPromptObsLines(t), obsEventConstraintCheck)
	driftLine := lines[len(lines)-1]
	if driftLine.Prompt.MemoryEmbedded {
		t.Error("记忆块不在系统提示里 ⇒ memory_embedded 必须为 false（本次没判准）")
	}
	if driftLine.Prompt.DynamicInjection {
		t.Error("判不准的时候不许报「存在动态注入」（宁可说没判准）")
	}
	if pfMustSeg(t, driftLine.Prompt, segmentMemory).SHA256 != pfSHA("MEM-未嵌进系统提示") {
		t.Error("即便判不准，memory 段自己的指纹仍必须落（账要全）")
	}
}

// ── 用例 ③【关键】：压缩掉一条 must_survive ⇒ missing_ids 非空 + 告警 ────────

// promptSeedWithConstraint — 插 30 条消息（偶数=user/奇数=assistant，同 ssRoleFor 口径），
// 其中 constraintAt 指定的下标用**硬约束原文**（要落在被压缩的中间段里：索引 3..9）。
func promptSeedWithConstraint(t *testing.T, st *ChatStore, sid string, n int, constraintAt int, text string) ([]int64, int64) {
	t.Helper()
	body := strings.Repeat("测", 300) // ≈428 token/条 ⇒ 30 条必超阈值
	ids := make([]int64, 0, n)
	var constraintID int64
	for i := 0; i < n; i++ {
		content := body
		if i == constraintAt {
			content = text
		}
		id, err := st.AddMessage(&Message{
			SessionID: sid, Role: ssRoleFor(i), Content: content, Active: true,
			Timestamp: 1757500000.0 + float64(i),
		})
		if err != nil {
			t.Fatalf("插消息失败: %v", err)
		}
		if i == constraintAt {
			constraintID = id
		}
		ids = append(ids, id)
	}
	return ids, constraintID
}

func TestConstraintMissingAfterCompaction(t *testing.T) {
	constraintText := "必须只用 gofmt 检查代码，禁止 git add -A。"
	canonical := pfCanonical(strings.TrimSuffix(constraintText, "。")) // 抽取是**句级**的 ⇒ 规范化后不带句末标点
	wantID := pfConstraintID(canonical)
	const constraintIdx = 6 // 必须在被压缩的中间段（索引 3..9）且 role=user（偶数——同 ssRoleFor 口径）

	// 子例 A：压缩**前**先检一次（present=1）⇒ 压缩后再检（present=0）—— 变化由压缩引起，不是自证
	t.Run("A_pre_post_compaction", func(t *testing.T) {
		st, _ := newCompactStore(t)
		resetConstraintRegistryCache()
		t.Cleanup(resetConstraintRegistryCache)

		se, err := st.CreateSession("gemma-12B", "desktop", "", "T3.2 关键用例")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = promptSeedWithConstraint(t, st, se.ID, 30, constraintIdx, constraintText)

		// 压缩前：约束在活动窗口里 ⇒ present=1、missing 空、无告警
		before := compactActive(t, st, se.ID)
		ObservePromptCheck(PromptRender{
			Session: se.ID, Round: 1, System: "TEMPLATE-BASE", History: pfRenderMsgs(before),
			Sources: before, Identity: FixedRequestIdentity("2026-09-17T12:00:00Z", 7),
		})
		lines := readPromptObsLines(t)
		pre := promptEvents(lines, obsEventConstraintCheck)
		if len(pre) != 1 || pre[0].Constraints == nil {
			t.Fatalf("压缩前应恰好一条 constraint_check：%+v", pre)
		}
		if pre[0].Constraints.Total != 1 || pre[0].Constraints.Present != 1 {
			t.Errorf("压缩前 total/present 应为 1/1：%+v", pre[0].Constraints)
		}
		if len(pre[0].Constraints.MissingIDs) != 0 || pre[0].Constraints.Alert {
			t.Errorf("压缩前不得有缺失/告警：%+v", pre[0].Constraints)
		}
		if len(promptEvents(lines, obsEventConstraintMissingAlert)) != 0 {
			t.Error("压缩前不应落告警事件")
		}

		// 真压缩（摘要器刻意**不**带约束原文 ⇒ 约束真的被压掉了）
		msgs := compactActive(t, st, se.ID)
		did, cerr := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", msgs, nil,
			func(context.Context, []map[string]any) (string, error) { return obsStructuredSummary(), nil }, false)
		if cerr != nil || !did {
			t.Fatalf("达阈值应压缩成功: did=%v err=%v", did, cerr)
		}
		after := compactActive(t, st, se.ID)
		if len(after) >= len(before) {
			t.Fatalf("压缩后活动消息应变少：before=%d after=%d", len(before), len(after))
		}
		// 前提校验：约束原文确实不在压缩后的活动窗口里（否则本用例就不成立）
		for _, m := range after {
			if strings.Contains(m.Content, canonical) {
				t.Fatalf("约束原文仍在活动窗口里（id=%d）⇒ 本用例前提不成立", m.ID)
			}
		}

		ObservePromptCheck(PromptRender{
			Session: se.ID, Round: 2, System: "TEMPLATE-BASE", History: pfRenderMsgs(after),
			Sources: after, Identity: FixedRequestIdentity("2026-09-17T12:00:02Z", 8),
		})
		lines = readPromptObsLines(t)
		post := promptEvents(lines, obsEventConstraintCheck)
		if len(post) != 2 {
			t.Fatalf("压缩后应共 2 条 constraint_check：%d", len(post))
		}
		pc := post[1].Constraints
		if pc == nil {
			t.Fatal("缺 constraints 块")
		}
		// ★ 关键判据（任务表 T3.2 逐字）：压缩掉一条 must_survive ⇒ missing_ids 非空
		if len(pc.MissingIDs) != 1 || pc.MissingIDs[0] != wantID {
			t.Fatalf("压缩掉 must_survive 约束 ⇒ missing_ids 应为 [%s]，实得 %v（独立复算 id 口径见 pfConstraintID）", wantID, pc.MissingIDs)
		}
		if pc.Total != 1 || pc.Present != 0 || !pc.Alert {
			t.Errorf("total/present/alert 应为 1/0/true：%+v", pc)
		}
		if len(pc.MustSurviveMissingIDs) != 1 || pc.MustSurviveMissingIDs[0] != wantID {
			t.Errorf("must_survive_missing_ids 应含该 id：%+v", pc.MustSurviveMissingIDs)
		}
		// 告警事件：可行动（哪条 / 是否必须活着 / 本该从哪进 / 来源消息）
		alerts := promptEvents(lines, obsEventConstraintMissingAlert)
		if len(alerts) != 1 {
			t.Fatalf("missing 非空 ⇒ 应恰好一条 constraint_missing_alert，实得 %d", len(alerts))
		}
		if len(alerts[0].ConstraintMissing) != 1 {
			t.Fatalf("告警应带可行动明细：%+v", alerts[0].ConstraintMissing)
		}
		miss := alerts[0].ConstraintMissing[0]
		if miss.ID != wantID || !miss.MustSurvive || miss.InjectWhere != injectWhereHistory {
			t.Errorf("告警明细口径不对：%+v", miss)
		}
		if !strings.Contains(miss.Text, "必须只用 gofmt") {
			t.Errorf("告警应看得见是哪条约束（截断原文）：%q", miss.Text)
		}
		// 告警是**观测**：不阻断请求 ⇒ 检测函数本身不返回错误、不 panic（调用返回即证）
		raw := readObs(t)
		for _, key := range []string{
			`"event_name":"constraint_missing_alert"`, `"constraint_missing"`,
			`"must_survive":true`, `"inject_where":"history"`,
		} {
			if !strings.Contains(raw, key) {
				t.Errorf("告警原文缺字段 %s\n原文：%s", key, raw)
			}
		}
	})

	// 子例 B：**不先检查**、直接压缩 ⇒ 压缩开始处登记也要管用（否则首轮即超阈值的会话查不出丢失）
	t.Run("B_register_at_compaction_start", func(t *testing.T) {
		st, _ := newCompactStore(t)
		resetConstraintRegistryCache()
		t.Cleanup(resetConstraintRegistryCache)

		se, err := st.CreateSession("gemma-12B", "desktop", "", "T3.2 压缩处登记")
		if err != nil {
			t.Fatal(err)
		}
		_, cid := promptSeedWithConstraint(t, st, se.ID, 30, constraintIdx, constraintText)
		if got := SessionConstraints(se.ID); len(got) != 0 {
			t.Fatalf("压缩前尚未检查过提示 ⇒ 登记表应为空，实得 %+v", got)
		}
		msgs := compactActive(t, st, se.ID)
		if did, cerr := st.MaybeCompact(context.Background(), se.ID, "gemma-12B", msgs, nil,
			func(context.Context, []map[string]any) (string, error) { return obsStructuredSummary(), nil }, false); cerr != nil || !did {
			t.Fatalf("应压缩成功: did=%v err=%v", did, cerr)
		}
		got := SessionConstraints(se.ID)
		if len(got) != 1 || got[0].ID != wantID || got[0].SourceMsgID != cid {
			t.Fatalf("压缩开始处应把约束登记在册（含来源消息 id=%d）：%+v", cid, got)
		}
		after := compactActive(t, st, se.ID)
		ObservePromptCheck(PromptRender{
			Session: se.ID, Round: 1, System: "TEMPLATE-BASE", History: pfRenderMsgs(after),
			Sources: after, Identity: FixedRequestIdentity("2026-09-17T12:00:00Z", 9),
		})
		pc := promptEvents(readPromptObsLines(t), obsEventConstraintCheck)[0].Constraints
		if pc == nil || len(pc.MissingIDs) != 1 || pc.MissingIDs[0] != wantID {
			t.Fatalf("压缩后必报 missing：%+v", pc)
		}
	})
}

// ── 用例 ④：无缺失 ⇒ missing_ids 为空且不告警（含"只带指纹"也认作在场）──────

func TestConstraintPresentNoAlert(t *testing.T) {
	promptIsolate(t)

	st := newTestStore(t)
	se, err := st.CreateSession("gemma-12B", "desktop", "", "T3.2 在场")
	if err != nil {
		t.Fatal(err)
	}
	constraintText := "不要用 git add -A 提交。"
	canonical := pfCanonical(constraintText)
	wantID := pfConstraintID(canonical)
	msgID, err := st.AddMessage(&Message{SessionID: se.ID, Role: "user", Content: constraintText, Active: true, Timestamp: 1})
	if err != nil {
		t.Fatal(err)
	}
	// 显式登记一条（精确入口）：靠**指纹**在场（提示里只带 sha256，不带原文）
	fpOnly := RegisterConstraint(se.ID, NewConstraint("提交信息必须带 --author", 0, true, injectWhereSystem))
	if !fpOnly {
		t.Fatal("显式登记应成功")
	}
	reg := SessionConstraints(se.ID)
	if len(reg) != 1 {
		t.Fatalf("此刻只有显式登记的那一条（自动抽取要等提示检查）：%+v", reg)
	}
	fpC := reg[0]
	if fpC.InjectWhere != injectWhereSystem || !fpC.MustSurvive {
		t.Fatalf("显式登记的字段应原样保留：%+v", fpC)
	}

	active := compactActive(t, st, se.ID)
	ObservePromptCheck(PromptRender{
		Session: se.ID, Round: 1, Model: "gemma-12B",
		System:  "TEMPLATE-BASE\n约束指纹: " + fpC.SHA256[:16], // 注入方只带指纹
		History: pfRenderMsgs(active), Sources: active,
		Identity: FixedRequestIdentity("2026-09-17T12:00:00Z", 11),
	})

	lines := readPromptObsLines(t)
	checks := promptEvents(lines, obsEventConstraintCheck)
	if len(checks) != 1 {
		t.Fatalf("应恰好一条 constraint_check：%d", len(checks))
	}
	c := checks[0].Constraints
	if c == nil {
		t.Fatal("缺 constraints 块")
	}
	if c.Total != 2 || c.Present != 2 {
		t.Fatalf("两条约束都应在场（一条按原文、一条按指纹）：%+v（登记表 %+v）", c, SessionConstraints(se.ID))
	}
	// wire 层：空集合必须落成 []（不是 null）——否则读侧要区分两种"空"
	if !strings.Contains(readObs(t), `"missing_ids":[]`) {
		t.Errorf("missing_ids 空集合应落成 []（不许 null）：%s", readObs(t))
	}
	if len(c.MissingIDs) != 0 || len(c.MustSurviveMissingIDs) != 0 || c.Alert {
		t.Errorf("无缺失 ⇒ missing 空且不告警：%+v", c)
	}
	if pfHasStr(c.MissingIDs, wantID) {
		t.Errorf("按原文在场的约束（%s）不得出现在 missing 里：%v", wantID, c.MissingIDs)
	}
	if alerts := promptEvents(lines, obsEventConstraintMissingAlert); len(alerts) != 0 {
		t.Errorf("无缺失 ⇒ 不得落告警事件（实得 %d 条）", len(alerts))
	}
	// 约束确实按原文在历史里（防"其实没检索到却报 present"）
	if !strings.Contains(pfRenderHistory(pfRenderMsgs(active)), canonical) {
		t.Fatal("用例前提不成立：历史里应含约束原文")
	}
	if msgID == 0 {
		t.Error("来源消息应落库")
	}
}

// ── 用例 ⑤：时间戳形态混进提醒/模板 ⇒ 落 prompt_impurity（非决定论可判）────

func TestPromptImpurityDetectsTimestamp(t *testing.T) {
	promptIsolate(t)

	ts := "2026-09-17T22:54:52+08:00"
	reminder := "注意：现在是 " + ts + "，请以此刻为准。"
	r := pfBaseRender("sess-T33-5", 3)
	r.System = "TEMPLATE-BASE\n当前时刻 " + ts
	r.Reminders = []string{reminder}
	ObservePromptCheck(r)

	lines := readPromptObsLines(t)
	imps := promptEvents(lines, obsEventPromptImpurity)
	if len(imps) != 2 {
		t.Fatalf("模板与提醒各应命中一条 ⇒ 2 条 prompt_impurity，实得 %d（%+v）", len(imps), imps)
	}
	byHost := map[string]PromptImpurityObs{}
	for _, im := range imps {
		if im.Impurity == nil {
			t.Fatal("prompt_impurity 必须带 impurity 块")
		}
		byHost[im.Impurity.Host] = *im.Impurity
	}
	for _, host := range []string{"system_template", "reminder"} {
		imp, ok := byHost[host]
		if !ok {
			t.Fatalf("应命中 %s（宿主闭集）：%+v", host, byHost)
		}
		if !pfHasStr(imp.Patterns, "rfc3339") {
			t.Errorf("%s：应报 rfc3339 形态，实得 %v", host, imp.Patterns)
		}
		if !strings.Contains(imp.Sample, "2026-09-17T22:54:52") {
			t.Errorf("%s：sample 应含命中片段，实得 %q", host, imp.Sample)
		}
		if len([]rune(imp.Excerpt)) > promptImpurityExcerptMax+3 || len([]rune(imp.Sample)) > promptImpuritySampleMax+3 {
			t.Errorf("%s：片段/上下文必须截断（sample=%d excerpt=%d）", host, len([]rune(imp.Sample)), len([]rune(imp.Excerpt)))
		}
	}
	// 指纹独立复算：system_template 那份 = sha256(系统提示去掉记忆块)
	if got := byHost["system_template"].SHA256; got != pfSHA(r.System) {
		t.Errorf("system_template 指纹应为 sha256(模板)：%s vs %s", got, pfSHA(r.System))
	}
	if got := byHost["reminder"].SHA256; got != pfSHA(reminder) {
		t.Errorf("reminder 指纹应为 sha256(提醒原文)：%s vs %s", got, pfSHA(reminder))
	}
	// 时钟形态（秒级时刻）与日期形态都能命中
	r2 := pfBaseRender("sess-T33-5b", 1)
	r2.Reminders = []string{"定时 09:31:07 触发"}
	ObservePromptCheck(r2)
	imps2 := promptEvents(readPromptObsLines(t), obsEventPromptImpurity)
	if len(imps2) <= len(imps) {
		t.Fatal("秒级时刻（09:31:07）应被检为时间戳形态")
	}

	// 非决定论可判：同一份输入，时间**进了提示** ⇒ 换时钟渲染哈希就不同（正是守卫要抓的事）
	base := pfBaseRender("sess-T33-5c", 1)
	ObservePromptCheck(base)
	withTime := base
	withTime.System = base.System + "\n" + ts
	ObservePromptCheck(withTime)
	withTime2 := base
	withTime2.System = base.System + "\n" + "2026-09-17T22:54:53+08:00"
	ObservePromptCheck(withTime2)
	checks := promptEvents(readPromptObsLines(t), obsEventConstraintCheck)
	n := len(checks)
	if checks[n-2].Prompt.RenderedPrefixHash == checks[n-1].Prompt.RenderedPrefixHash {
		t.Error("提示里带了时间戳 ⇒ 两次渲染的 prefix 哈希必须不同（否则守卫漏了最危险的那类注入）")
	}
}

// pfHasStr — 字符串是否在切片里
func pfHasStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// mustParseTime — 用例里的固定时钟（RFC3339 → time.Time；解析失败即用例写错）
func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("用例时钟写法有误 %q: %v", s, err)
	}
	return tm
}

// ── 用例 ⑥【反例】：正常中文提醒不得被误判；工具结果/记忆里的时间豁免 ───────

func TestPromptPurityNoFalsePositive(t *testing.T) {
	promptIsolate(t)

	r := pfBaseRender("sess-T33-6", 1)
	r.Reminders = []string{
		"别忘了：先跑 gofmt -l internal/chat/ 再 go build ./...；提交前不要 git add -A。",
		"提交信息必须带 --author，版本号 v2.5.10，任务 T3.2；128 条用例全绿。",
		"注意：中文冒号后面 10:30 这种写法按约定不算时刻；文件名 go.mod 不许改。",
	}
	// 工具结果/记忆/历史里的时间：H6 明列的**合法通道** ⇒ 不报
	// （注意：工具**定义/描述**是代码写的 schema，属"必须纯净"那一类，不在豁免之列 —— 见下面的正控）
	r.Tools = append(r.Tools, map[string]any{"type": "function", "function": map[string]any{
		"name": "stat", "description": "读文件元信息（返回 mtime 与大小）"}})
	r.Memory = "MEM\n- 上次运行时间 2026-09-17 22:54:52"
	r.System = r.System + "\n\n" + r.Memory
	r.History = append(r.History, map[string]any{"role": "tool", "content": "mtime=2026-09-17T22:54:52+08:00"})

	ObservePromptCheck(r)

	lines := readPromptObsLines(t)
	if imps := promptEvents(lines, obsEventPromptImpurity); len(imps) != 0 {
		for _, im := range imps {
			t.Errorf("反例失败：正常内容被误判 — host=%s patterns=%v sample=%q",
				im.Impurity.Host, im.Impurity.Patterns, im.Impurity.Sample)
		}
		t.Fatalf("正常中文提醒与合法通道里的时间**都不得**落 prompt_impurity")
	}
	if len(promptEvents(lines, obsEventConstraintCheck)) != 1 {
		t.Error("纯净（无杂质）也要落 constraint_check 的账（每请求一条）")
	}
	// 正控：工具**定义**（代码写的 schema）里带时间戳 ⇒ 必须报（豁免只给"工具结果"，不给工具描述）
	dirty := r
	dirty.Tools = append(dirty.Tools, map[string]any{"type": "function", "function": map[string]any{
		"name": "stat2", "description": "返回 mtime，如 2026-09-17T22:54:52+08:00"}})
	ObservePromptCheck(dirty)
	imps := promptEvents(readPromptObsLines(t), obsEventPromptImpurity)
	if len(imps) != 1 || imps[0].Impurity.Host != "tools" {
		t.Fatalf("工具定义里的时间戳必须报（host=tools）：%+v", imps)
	}
	// 中文冒号写法（10:30）与版本号（v2.5.10）都不在闭集里 —— 单点钉住形态口径
	for _, s := range []string{"10:30 这种写法", "版本号 v2.5.10", "128 条用例全绿"} {
		if imp := scanPromptImpurity(PromptPuritySource{Host: "reminder", Text: s}); imp != nil {
			t.Errorf("形态闭集过宽：%q 被误判为 %v", s, imp.Patterns)
		}
	}
	// 时间戳本身必须仍然检得出（守卫不能被"反例"改宽成永远不报）
	if imp := scanPromptImpurity(PromptPuritySource{Host: "reminder", Text: "现在是 2026-09-17T22:54:52+08:00"}); imp == nil {
		t.Error("时间戳必须检出（反例不许把守卫改宽成永远不报）")
	}
}

// ── T3.5：时钟与种子挂到**每个请求**的轮次记录上（不只挂提示事件）────────────

func TestObsTimerCarriesRequestIdentity(t *testing.T) {
	promptIsolate(t)

	id := FixedRequestIdentity("2026-09-17T12:00:00Z", 4242)
	tm := NewObsTimer("sess-T35-turn", 1, "gemma-12B")
	tm.SetRequestIdentity(id)
	tm.Finish("finish")

	// 不给身份的一轮：字段必须**缺席**（不编造时钟/不编 0 种子）
	tm2 := NewObsTimer("sess-T35-turn2", 1, "gemma-12B")
	tm2.Finish("finish")

	raw := readObs(t)
	if !strings.Contains(raw, `"clock_iso":"2026-09-17T12:00:00Z"`) ||
		!strings.Contains(raw, `"request_seed":4242`) {
		t.Errorf("轮次记录必须带 clock_iso + request_seed（H6：每个请求都要带）：%s", raw)
	}
	lines := readPromptObsLines(t)
	var withIdent, without int
	for _, ln := range lines {
		if ln.Kind != "turn" {
			continue
		}
		if ln.ClockISO == "" {
			without++
			if ln.RequestSeed != nil {
				t.Error("没给身份却落了种子（编造）")
			}
			continue
		}
		withIdent++
		if ln.RequestSeed == nil || *ln.RequestSeed != 4242 {
			t.Errorf("种子应与注入值一致：%v", ln.RequestSeed)
		}
	}
	if withIdent != 1 || without != 1 {
		t.Errorf("应各有一条（给了身份/没给身份）：with=%d without=%d", withIdent, without)
	}
}

// ── 口径守卫：事件名/闭集是公开契约（改动即红）────────────────────────────

func TestPromptObsContractStrings(t *testing.T) {
	if obsKindPrompt != "prompt" || obsEventConstraintCheck != "constraint_check" ||
		obsEventConstraintMissingAlert != "constraint_missing_alert" || obsEventPromptImpurity != "prompt_impurity" {
		t.Fatal("本族事件名是公开契约（读侧按 event_name 挑行）：改动必须是有意的并同步文档")
	}
	// 分段 id 闭集（四段固定、顺序固定）
	got := []string{segmentSystem, segmentTools, segmentHistory, segmentMemory}
	want := []string{"system", "tools", "history", "memory"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("分段 id 闭集/顺序变了：%v（期望 %v）", got, want)
		}
	}
	// inject_where 闭集
	for _, w := range []string{injectWhereSystem, injectWhereMemory, injectWhereHistory, injectWherePinned} {
		if normalizeInjectWhere(w) != w {
			t.Errorf("%q 应在闭集内且原样保留", w)
		}
	}
	if normalizeInjectWhere("") != injectWhereUnknown || normalizeInjectWhere("bogus") != injectWhereUnknown {
		t.Error("非法 inject_where 必须归为 unknown（不猜、不落到闭集之外）")
	}
	// 登记表落盘路径（读侧找得到）
	if !strings.HasSuffix(constraintRegistryFile(), "constraint_registry.json") {
		t.Errorf("登记表落点应可预期：%s", constraintRegistryFile())
	}
	// identityFor 决定论：同 (会话,轮次,时钟) ⇒ 同种子；种子非负
	t1 := identityFor("s", 1, mustParseTime(t, "2026-09-17T12:00:00.123456789Z"))
	t2 := identityFor("s", 1, mustParseTime(t, "2026-09-17T12:00:00.123456789Z"))
	if t1 != t2 {
		t.Error("同三元组 ⇒ 同一身份（回放才可能复现）")
	}
	if t1.Seed < 0 {
		t.Error("种子必须非负（LLM seed 惯例）")
	}
	if t1.ClockISO != "2026-09-17T12:00:00.123456789Z" {
		t.Errorf("clock_iso 应落 UTC RFC3339Nano：%q", t1.ClockISO)
	}
	// 登记表文件真的落在隔离目录里（不是内存孤岛）
	dir := os.Getenv("ZERG_STATE_DIR")
	if _, err := os.Stat(filepath.Join(dir, "constraint_registry.json")); err != nil {
		// 本用例没登记过任何约束 ⇒ 文件可以不存在；这里只钉住"路径在对的目录下"
		if !strings.HasPrefix(constraintRegistryFile(), dir) {
			t.Errorf("登记表路径应在 state 目录内：%s", constraintRegistryFile())
		}
	}
}
