package chatlog

import (
	"bytes"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// 波4 · 压缩片 · 四条不变量判据（能红能绿）
//
//	① 压后**近轮逐字未变**
//	② 压后**稳定层 sha256 不变**（并测「易变层只追加在尾 ⇒ 稳定前缀逐字节不变」）
//	③ 压下后**不丢未摘要事实**（早期哨兵事实：要么被保留、要么被摘要带上）
//	④ **确定性：同输入 ⇒ 同输出**（无时间 / 无随机注入）
//
// 每条的变异自证见文件末注释块 + 交付报告的 overlay 读数。
// ─────────────────────────────────────────────────────────────────────────────

// padded —— 造一条带唯一标签、约 30 token 的消息（够长以触发压缩）。
func padded(label string) string {
	return label + "-" + strings.Repeat("x", 100)
}

// echoSummarizer —— 确定性「忠实」摘要器：把旧轮内容拼进来 ⇒ 哨兵事实被带上。
func echoSummarizer(old []Message) (string, error) {
	parts := make([]string, 0, len(old))
	for _, m := range old {
		parts = append(parts, m.Content)
	}
	return "要点：" + strings.Join(parts, " ｜ "), nil
}

// ───────────────────── ① 压后近轮逐字未变 ─────────────────────

// TestCompact_RecentTurnsVerbatim（不变量①）：压缩后尾部近轮**逐条逐字未变**。
//
// 变异自证（能红）：把 Compact 组装尾部那一行 `next = append(next, msgs[start:]...)`
// 改成丢弃尾部（`append(next, msgs[:0]...)`）⇒ 近轮消失 ⇒ 本条当场红。
func TestCompact_RecentTurnsVerbatim(t *testing.T) {
	roles := []string{"user", "assistant"}
	var msgs []Message
	for i := 0; i < 10; i++ {
		msgs = append(msgs, Message{Role: roles[i%2], Kind: KindMessage, Content: padded("M" + itoa(i))})
	}
	p := Prompt{System: Layers{Stable: "身份"}, Messages: msgs}

	res, err := Compact(p, CompactOptions{
		WindowTokens: 100, TriggerRatio: 0.5, RecentRatio: 0.2,
		ProtectHead: 1, ProtectTail: 3,
		Summarizer: SummarizerFunc(echoSummarizer),
	})
	if err != nil {
		t.Fatalf("Compact 失败: %v", err)
	}
	if res.SummarizedCount == 0 {
		t.Fatalf("未发生摘要 ⇒ 用例空转（action=%s reason=%s）", res.Action, res.Reason)
	}
	got := res.Prompt.Messages
	if len(got) >= len(msgs) {
		t.Fatalf("压缩后消息数未减少（前 %d → 后 %d）—— 没真压", len(msgs), len(got))
	}

	want := msgs[len(msgs)-3:]
	gotTail := got[len(got)-3:]
	for i := range want {
		if gotTail[i] != want[i] {
			t.Fatalf("近轮逐字被改：第 %d 条\n  期望 %+v\n  实得 %+v", i, want[i], gotTail[i])
		}
	}
	// 逐字（字节级）断言：内容串完整出现在渲染结果里。
	r := string(res.Prompt.Render())
	for _, w := range want {
		if !strings.Contains(r, w.Content) {
			t.Fatalf("近轮内容未逐字出现在结果里：%q", w.Content)
		}
	}
}

// ───────────────────── ② 压后稳定层 sha256 不变 ─────────────────────

// TestCompact_StableLayerSHA256Unchanged（不变量②）：
//   - 压缩**不动稳定层 / 上下文层** ⇒ 稳定前缀 sha256 与字节**都不变**；
//   - 易变层（记忆 / 技能 / 时间）**只追加在尾** ⇒ 稳定前缀仍逐字节不变、渲染序仍 tools→system→messages。
//
// 变异自证（能红）：让 Compact 往稳定层里塞一格标记（`res.Prompt.System.Stable = "已压" + …`）
// ⇒ sha256 变 ⇒ 本条当场红。
func TestCompact_StableLayerSHA256Unchanged(t *testing.T) {
	var msgs []Message
	roles := []string{"user", "assistant"}
	for i := 0; i < 8; i++ {
		msgs = append(msgs, Message{Role: roles[i%2], Kind: KindMessage, Content: padded("M" + itoa(i))})
	}
	p := Prompt{
		Tools: []ToolDef{{Name: "zerg task submit", Description: "派任务"}, {Name: "zerg help"}},
		System: Layers{
			Stable:   "虫族身份 ＋ 收尾契约（一字不动）",
			Context:  "工作区快照 v1",
			Volatile: []string{"技能索引: a,b"},
		},
		Messages: msgs,
	}

	shaBefore := p.StableSHA256()
	prefixBefore := p.StablePrefix()
	if !p.RenderOrderOK() {
		t.Fatal("压缩前渲染序不满足 tools→system→messages")
	}

	res, err := Compact(p, CompactOptions{
		WindowTokens: 100, TriggerRatio: 0.5, RecentRatio: 0.2,
		ProtectHead: 1, ProtectTail: 2,
		Summarizer: SummarizerFunc(echoSummarizer),
	})
	if err != nil {
		t.Fatalf("Compact 失败: %v", err)
	}
	if res.SummarizedCount == 0 {
		t.Fatalf("未发生摘要 ⇒ 用例空转（action=%s）", res.Action)
	}

	// ★ 稳定层 sha256 不变（不变量②）。
	if got := res.Prompt.StableSHA256(); got != shaBefore {
		t.Fatalf("压缩后稳定层 sha256 变了：\n  前 %s\n  后 %s", shaBefore, got)
	}
	if !bytes.Equal(res.Prompt.StablePrefix(), prefixBefore) {
		t.Fatal("压缩后稳定前缀字节被改（稳定层应逐字节不动）")
	}

	// 易变层只追加在尾 ⇒ 稳定前缀逐字节不变。
	p2 := res.Prompt.AppendVolatile("记忆: 端口=7788", "技能索引: c,d", "时间: 2026-10-14T01:50:00Z")
	if !bytes.Equal(p2.StablePrefix(), prefixBefore) {
		t.Fatal("追加易变层后稳定前缀字节被改（易变层应只追加在尾）")
	}
	if p2.StableSHA256() != shaBefore {
		t.Fatal("追加易变层后稳定层 sha256 变了")
	}
	if !p2.RenderOrderOK() {
		t.Fatal("追加易变层后渲染序不再是 tools→system→messages")
	}
	if !bytes.HasPrefix(p2.Render(), prefixBefore) {
		t.Fatal("渲染结果不再以稳定前缀打头（易变层未追加在尾）")
	}
}

// ───────────────────── ③ 压下后不丢未摘要事实 ─────────────────────

// TestCompact_SentinelFactSurvives（不变量③）：
// 埋一条**只在早期**出现的哨兵事实；它落在被摘要的旧轮里 ⇒ 要么逐字保留、要么被摘要带上。
//
// 变异自证（能红）：让 Compact 忽略摘要器输出（`sum, err := "", nil`）⇒ 旧轮被折叠成空摘要
// ⇒ 哨兵事实丢失 ⇒ 本条当场红。
func TestCompact_SentinelFactSurvives(t *testing.T) {
	const sentinel = "SENTINEL-端口7788-唯一真源"
	msgs := []Message{
		{Role: "user", Kind: KindMessage, Content: padded("M0")},
		{Role: "assistant", Kind: KindMessage, Content: padded("M1")},
		{Role: "user", Kind: KindMessage, Content: sentinel}, // ← 早期 · 落在被摘要的旧轮里
		{Role: "assistant", Kind: KindMessage, Content: padded("M3")},
		{Role: "user", Kind: KindMessage, Content: padded("M4")},
		{Role: "user", Kind: KindMessage, Content: padded("M5")},
	}
	p := Prompt{System: Layers{Stable: "身份"}, Messages: msgs}

	res, err := Compact(p, CompactOptions{
		WindowTokens: 100, TriggerRatio: 0.5, RecentRatio: 0.2,
		ProtectHead: 1, ProtectTail: 1,
		Summarizer: SummarizerFunc(echoSummarizer),
	})
	if err != nil {
		t.Fatalf("Compact 失败: %v", err)
	}
	if res.SummarizedCount == 0 {
		t.Fatalf("未发生摘要 ⇒ 用例空转（action=%s）", res.Action)
	}
	r := string(res.Prompt.Render())
	if !strings.Contains(r, sentinel) {
		t.Fatalf("早期哨兵事实丢失（既没逐字保留、也没被摘要带上）：%q\n渲染结果：\n%s", sentinel, r)
	}
}

// ───────────────────── ④ 确定性：同输入 ⇒ 同输出 ─────────────────────

// TestCompact_DeterministicSameInputSameOutput（不变量④）：
// 同一输入跑两次 ⇒ 动作 / 计数 / 渲染字节**逐字相同**（✗ 无时间、✗ 无随机注入）。
//
// 变异自证（能红）：在摘要内容里注入 `time.Now()` ⇒ 两次渲染不同 ⇒ 本条当场红。
func TestCompact_DeterministicSameInputSameOutput(t *testing.T) {
	mk := func() Prompt {
		roles := []string{"user", "assistant"}
		var msgs []Message
		for i := 0; i < 8; i++ {
			msgs = append(msgs, Message{Role: roles[i%2], Kind: KindMessage, Content: padded("M" + itoa(i))})
		}
		return Prompt{
			Tools:    []ToolDef{{Name: "zerg help"}},
			System:   Layers{Stable: "身份", Context: "ctx"},
			Messages: msgs,
		}
	}
	opt := CompactOptions{
		WindowTokens: 100, TriggerRatio: 0.5, RecentRatio: 0.2,
		ProtectHead: 1, ProtectTail: 2,
		Summarizer: SummarizerFunc(echoSummarizer),
	}
	r1, err := Compact(mk(), opt)
	if err != nil {
		t.Fatalf("Compact#1 失败: %v", err)
	}
	r2, err := Compact(mk(), opt)
	if err != nil {
		t.Fatalf("Compact#2 失败: %v", err)
	}
	if r1.SummarizedCount == 0 {
		t.Fatalf("未发生摘要 ⇒ 用例空转（action=%s）", r1.Action)
	}
	if r1.Action != r2.Action || r1.RemovedCount != r2.RemovedCount || r1.SummarizedCount != r2.SummarizedCount {
		t.Fatalf("两次读数不同：%+v vs %+v", r1, r2)
	}
	if b1, b2 := r1.Prompt.Render(), r2.Prompt.Render(); !bytes.Equal(b1, b2) {
		t.Fatalf("同输入两次渲染字节不同（非确定性）：\n#1 %s\n#2 %s", b1, b2)
	}
}

// itoa —— 不走 strconv 的极简十进制（避免额外依赖 · 保持测试自足）。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
