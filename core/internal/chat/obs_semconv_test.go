// obs_semconv_test.go — T6.1（对外对齐：命名映射表 / semconv 版本 / 内容捕获四档→标准枚举）
// 与 T6.2（补齐标准字段）的实证。
//
// 要治的盲区（改本文件前先读）：
//   - 设计稿 B14：私名与标准名**混着用**会污染后端的语义解析（指纹/工具数/字节数必须留在 zerg.*）；
//   - B15：gen_ai.* 全是 Development（随时可变）⇒ 事件必须带"对齐的 semconv 版本/commit"，
//     否则后端升级会**静默丢字段**，而"丢了"与"本来就没有"在观测面同形；
//   - B4：error.type 只在末事件记 ⇒ 中间 span 出错时答不出"哪个 span 出的什么错"；
//   - B10：conversation.compacted 被写成 false ⇒ 后端把"没压过"与"不知道"混同（规范 SHOULD NOT set false）；
//   - B6：内容类属性被默认采下来（含正文）⇒ 隐私与体量双爆。
//
// 纪律（与 obs_test.go / obs_trace_test.go 同源）：
//   - 判据分两层：**wire 层**（键真的在 JSONL 原文里 —— 键名带点，只能按原文/地图查）+ **read 层**（值是真的）；
//   - 只断 Go 字段 = 自证 ⇒ 一律读回 JSONL；
//   - 观测一律落隔离目录：t.Setenv("ZERG_STATE_DIR", t.TempDir())，绝不写用户真实状态；
//   - 涉及包级状态的用例（推理对端 / 压缩记忆）自带清理，避免污染同包其它用例。
package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ── 读侧工具 ──────────────────────────────────────────────────────────────

// semconvLines — 读回全部记录并解析成 map。
// 为什么不用结构体：本任务断的就是**点分键名**（gen_ai.* / server.* / zerg.*），
// 结构体解码会把"键写错了"这件事吞掉（认不出的键直接丢）。
func semconvLines(t *testing.T) []map[string]any {
	t.Helper()
	raw := strings.TrimSpace(readObs(t))
	if raw == "" {
		return nil
	}
	var out []map[string]any
	for _, ln := range strings.Split(raw, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("观测行不是合法 JSON：%v\n原文：%s", err, ln)
		}
		out = append(out, m)
	}
	return out
}

// semconvFind — 按谓词挑记录
func semconvFind(lines []map[string]any, pred func(map[string]any) bool) []map[string]any {
	var out []map[string]any
	for _, m := range lines {
		if pred(m) {
			out = append(out, m)
		}
	}
	return out
}

// semconvField — 取字段值（缺席 ⇒ ("", false)）
func semconvField(m map[string]any, key string) (any, bool) {
	v, ok := m[key]
	return v, ok
}

// semconvStr — 取字符串字段（缺席/非字符串 ⇒ ""）
func semconvStr(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

// ── ① 四档 → 标准枚举（逐条映射 + 归一 + EMIT_EVENT 推导）────────────────

func TestObsSemconv_CaptureLevelMapping(t *testing.T) {
	cases := []struct {
		level string
		std   string
		ok    bool
		why   string
	}{
		{ObsCaptureOFF, GenAICaptureNoContent, true, "OFF：不采内容"},
		{ObsCaptureBASIC, GenAICaptureNoContent, true, "BASIC：只有决策/工具判定，正文类属性一律 Opt-In ⇒ 仍是不采内容"},
		{ObsCaptureVERBOSE, GenAICaptureEventOnly, true, "VERBOSE：完整提示与响应只写事件（规范：结构化内容优先放事件）"},
		{ObsCaptureTRACE, GenAICaptureSpanAndEvent, true, "TRACE：含内部状态 ⇒ 两边都写"},
		{"  trace ", GenAICaptureSpanAndEvent, true, "大小写/空白归一"},
		{"Verbose", GenAICaptureEventOnly, true, "大小写归一"},
		{"loud", GenAICaptureNoContent, false, "非法值 ⇒ 回最严"},
		{"", GenAICaptureNoContent, false, "空串不是四档之一 ⇒ 回最严"},
		{GenAICaptureSpanAndEvent, GenAICaptureNoContent, false, "拿标准枚举当我们的档位 ⇒ 认不出 ⇒ 回最严（不默认放开）"},
	}
	for _, c := range cases {
		std, ok := CaptureLevelStd(c.level)
		if std != c.std || ok != c.ok {
			t.Errorf("CaptureLevelStd(%q) = (%q,%v)，期望 (%q,%v) —— %s", c.level, std, ok, c.std, c.ok, c.why)
		}
		if got := CaptureLevelToStd(c.level); got != c.std {
			t.Errorf("CaptureLevelToStd(%q) = %q，期望 %q", c.level, got, c.std)
		}
	}

	// EMIT_EVENT 推导（照抄子报告：NO_CONTENT/SPAN_ONLY ⇒ 默认 false；EVENT_ONLY/SPAN_AND_EVENT ⇒ true）
	for _, c := range []struct {
		std  string
		want bool
	}{
		{GenAICaptureNoContent, false}, {GenAICaptureSpanOnly, false},
		{GenAICaptureEventOnly, true}, {GenAICaptureSpanAndEvent, true},
		{"nonsense", false}, // 认不出 ⇒ 不发事件（fail-closed）
	} {
		if got := GenAIEmitEvent(c.std); got != c.want {
			t.Errorf("GenAIEmitEvent(%q) = %v，期望 %v", c.std, got, c.want)
		}
	}

	// 内容类属性总闸：只有 NO_CONTENT 是本任务默认档
	for _, c := range []struct {
		std  string
		want bool
	}{
		{GenAICaptureNoContent, false}, {GenAICaptureSpanOnly, true},
		{GenAICaptureEventOnly, true}, {GenAICaptureSpanAndEvent, true},
		{"nonsense", false},
	} {
		if got := ObsContentAllowed(c.std); got != c.want {
			t.Errorf("ObsContentAllowed(%q) = %v，期望 %v", c.std, got, c.want)
		}
	}
}

// ── ② 映射表 ↔ 结构体 tag ↔ 调研子报告（三向对账，防手滑）────────────────

func TestObsSemconv_TableMatchesRecordAndReport(t *testing.T) {
	// ① 反射：表里 Emit=true 的每一行，ObsRecord 必须有**逐字同名**的 json tag
	tags := map[string]string{} // json 键名 → Go 字段名
	rt := reflect.TypeOf(ObsRecord{})
	for i := 0; i < rt.NumField(); i++ {
		name := strings.Split(rt.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			tags[name] = rt.Field(i).Name
		}
	}
	emitted := 0
	for _, a := range ObsSemconvAttrs() {
		if !a.Emit {
			continue
		}
		emitted++
		if _, ok := tags[a.OTel]; !ok {
			t.Errorf("映射表说我们会落 %s（私名 %s），但 ObsRecord 里没有同名的 json tag ⇒ 表与实现脱节（手滑/漏加字段）",
				a.OTel, a.Private)
		}
	}
	if emitted < 20 {
		t.Errorf("Emit=true 的表行只有 %d 条（应 ≥20）—— 映射表是不是被改残了？", emitted)
	}
	if _, ok := tags[AttrErrorType]; !ok {
		t.Errorf("error.type 必须有承载字段（规范里唯一 Stable 的属性之一）")
	}
	if _, ok := tags[AttrServerAddress]; !ok {
		t.Errorf("server.address 必须有承载字段（规范里唯一 Stable 的属性之一）")
	}

	// ② 反向：每条带点的 json tag，要么是标准名（在表里），要么是 zerg.* 私有名
	for name, field := range tags {
		if !strings.Contains(name, ".") || strings.HasPrefix(name, "zerg.") {
			continue
		}
		if !ObsSemconvEmitted(name) {
			t.Errorf("字段 %s 的键 %q 既不在标准名闭集里、也不带 zerg. 前缀 ⇒ 自造字段伪装成了标准字段", field, name)
		}
	}

	// ③ 表里每个 OTel 名必须**逐字**出现在调研子报告里（"照抄子报告"这件事本身要被验证，
	//    否则标准名写错一位后端就静默丢字段）。报告不在（比如只带了 core/）⇒ 跳过，不算通过。
	report := readOTelSubreport(t)
	if report == "" {
		t.Skip("调研子报告不在工作区（只带了 core/？）⇒ 跳过标准名对账")
	}
	for _, a := range ObsSemconvAttrs() {
		if !strings.Contains(report, a.OTel) {
			t.Errorf("映射表里的标准名 %q 在调研子报告里找不到逐字出处 ⇒ 可能是手抄错了", a.OTel)
		}
		if a.Requirement == "" {
			t.Errorf("映射表行 %s 缺规范要求级别", a.OTel)
		}
	}
	// 基准串本身也必须与报告一致（B15：升级时按这个做字段兼容检查）
	if !strings.Contains(report, SemconvGenAICommit) || !strings.Contains(report, SemconvGenAIDate) {
		t.Errorf("semconv 基准 %s 与调研子报告不一致（报告里应同时出现 commit 与日期）", SemconvGenAIVersion)
	}
}

// readOTelSubreport — 读 OTel 调研子报告（T6.1 的"照抄"依据）；找不到返回空串。
func readOTelSubreport(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "docs", "调研", "子报告-内建调试版-20260917")
	files, err := filepath.Glob(filepath.Join(dir, "subagent-summary-1-*.txt"))
	if err != nil || len(files) == 0 {
		return ""
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		return ""
	}
	return string(b)
}

// ── ③ 非法分档回最严（且**留痕**：不能静默降级）────────────────────────────

func TestObsSemconv_InvalidCaptureLevelFallsBackStrictest(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// ① 没设置 ⇒ OFF / NO_CONTENT，且**不**打 fallback 标记（"没设置"与"写错了"必须可分）
	t.Setenv(ObsCaptureEnvLevel, "")
	obsWrite(ObsRecord{Kind: obsKindTurn, Session: "s-cap-unset"})

	// ② 写错了 ⇒ 回最严（NO_CONTENT）且 fallback=true（fail-closed **且**留痕）
	t.Setenv(ObsCaptureEnvLevel, "LOUD")
	obsWrite(ObsRecord{Kind: obsKindTurn, Session: "s-cap-bad"})

	// ③ 我们四档照常映射
	t.Setenv(ObsCaptureEnvLevel, "TRACE")
	obsWrite(ObsRecord{Kind: obsKindTurn, Session: "s-cap-trace"})

	// ④ 标准总开关显式且合法 ⇒ 用它；我们那一档**缺席**（不编造"是哪一档"）
	t.Setenv(ObsCaptureEnvLevel, "OFF")
	t.Setenv(ObsCaptureEnvStd, "span_only")
	obsWrite(ObsRecord{Kind: obsKindTurn, Session: "s-cap-std"})

	lines := semconvLines(t)
	get := func(sess string) map[string]any {
		got := semconvFind(lines, func(m map[string]any) bool { return semconvStr(m, "session") == sess })
		if len(got) != 1 {
			t.Fatalf("会话 %s 应恰好 1 条记录，实际 %d 条", sess, len(got))
		}
		return got[0]
	}

	if got := semconvStr(get("s-cap-unset"), "zerg.capture.std"); got != GenAICaptureNoContent {
		t.Errorf("未设置档位 ⇒ std 应为 %s，实际 %q", GenAICaptureNoContent, got)
	}
	if _, ok := semconvField(get("s-cap-unset"), "zerg.capture.fallback"); ok {
		t.Errorf("未设置档位不该打 fallback 标记（「没设置」≠「写错了」）")
	}
	bad := get("s-cap-bad")
	if got := semconvStr(bad, "zerg.capture.level"); got != ObsCaptureOFF {
		t.Errorf("非法档位应回 OFF，实际 %q", got)
	}
	if got := semconvStr(bad, "zerg.capture.std"); got != GenAICaptureNoContent {
		t.Errorf("非法档位应回最严 %s，实际 %q", GenAICaptureNoContent, got)
	}
	if v, ok := bad["zerg.capture.fallback"]; !ok || v != true {
		t.Errorf("非法档位必须留痕（zerg.capture.fallback=true），实际 %v（存在=%v）", v, ok)
	}
	if got := semconvStr(get("s-cap-trace"), "zerg.capture.std"); got != GenAICaptureSpanAndEvent {
		t.Errorf("TRACE ⇒ %s，实际 %q", GenAICaptureSpanAndEvent, got)
	}
	stdRec := get("s-cap-std")
	if got := semconvStr(stdRec, "zerg.capture.std"); got != GenAICaptureSpanOnly {
		t.Errorf("标准总开关生效时应取标准值 %s，实际 %q", GenAICaptureSpanOnly, got)
	}
	if _, ok := semconvField(stdRec, "zerg.capture.level"); ok {
		t.Errorf("标准总开关生效时我们那一档应**缺席**（不编造）")
	}
}

// ── ④ error.type：每个出错 span 都要（不只末事件）＋ 低基数 ──────────────────

func TestObsSemconv_ErrorTypeOnEveryErrorSpan(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// 出错 span ①：轮次（超时）
	tm := NewObsTimer("s-err-turn", 1, "m")
	tm.MarkChunk()
	tm.Finish(ChatErrUpstreamTimeout)
	// 出错 span ②：工具执行失败
	ObsTool("s-err-turn", 1, "bash", "12ms", false, 1, MaxToolRounds)
	// 出错 span ③：压缩失败（旧 OBS-4 行）
	obsCompact("s-err-turn", "threshold", 0, 0, 0, 0, "fail", "HTTP 500 (streak=2)", false)
	// 出错 span ④：压缩三态事件的 failed
	obsCompactionStart(compactObsFacts{Session: "s-err-turn", Model: "m", Trigger: "threshold"}).failed("boom")
	// 对照（**不许**有 error.type）：正常收尾 / 工具成功 / 压缩成功 / 判定层 deny（工具没跑，不是出错）
	good := NewObsTimer("s-err-turn", 2, "m")
	good.MarkChunk()
	good.Finish("finish")
	ObsTool("s-err-turn", 2, "read", "3ms", true, 2, MaxToolRounds)
	obsCompactionStart(compactObsFacts{Session: "s-err-turn", Model: "m", Trigger: "threshold"}).completed("s", "【历史摘要】s", true)
	ObsToolDecision("s-err-turn", "bash", "deny", "dangerous_command", "0123456789abcdef")

	// 客户端中断：不是"操作出错"（只落 status=cancelled，不落 error.type）
	ab := NewObsTimer("s-err-turn", 3, "m")
	ab.MarkChunk()
	ab.Finish(ChatErrClientAborted)

	lines := semconvLines(t)

	// ① 四条出错记录**逐条**都必须有 error.type
	for _, c := range []struct {
		name string
		pred func(map[string]any) bool
		want string
	}{
		{"轮次超时", func(m map[string]any) bool {
			return semconvStr(m, "kind") == obsKindTurn && semconvStr(m, "end_reason") == ChatErrUpstreamTimeout
		}, ErrTypeUpstreamTimeout},
		{"工具失败", func(m map[string]any) bool {
			return semconvStr(m, "kind") == obsKindTool && semconvStr(m, "result") == "err"
		}, ErrTypeToolExec},
		{"压缩失败(OBS-4)", func(m map[string]any) bool {
			return semconvStr(m, "kind") == obsKindCompact && semconvStr(m, "result") == "fail"
		}, ErrTypeCompactionFail},
		{"压缩失败(三态)", func(m map[string]any) bool { return semconvStr(m, "event_name") == obsEventCompactionFailed }, ErrTypeCompactionFail},
	} {
		got := semconvFind(lines, c.pred)
		if len(got) == 0 {
			t.Fatalf("找不到记录：%s（用例自身失效）", c.name)
		}
		for _, m := range got {
			if s := semconvStr(m, AttrErrorType); s != c.want {
				t.Errorf("%s：error.type = %q，期望 %q（每个出错 span 都要，不只末事件）", c.name, s, c.want)
			}
		}
	}

	// ② 对照：正常收尾 / 成功 / deny 一律**不落** error.type
	for _, m := range semconvFind(lines, func(m map[string]any) bool {
		k := semconvStr(m, "kind")
		return (k == obsKindTurn && semconvStr(m, "end_reason") == "finish") ||
			(k == obsKindTool && semconvStr(m, "result") == "ok") ||
			(k == obsKindTool && semconvStr(m, "event_name") == obsEventToolDecision) ||
			semconvStr(m, "event_name") == obsEventCompactionCompleted
	}) {
		if s := semconvStr(m, AttrErrorType); s != "" {
			t.Errorf("没有出错的记录不该有 error.type，实际 %q（kind=%s event=%s）", s, semconvStr(m, "kind"), semconvStr(m, "event_name"))
		}
	}

	// ③ 客户端中断：status=cancelled 但**不**报 error.type
	aborted := semconvFind(lines, func(m map[string]any) bool { return semconvStr(m, "end_reason") == ChatErrClientAborted })
	if len(aborted) != 1 {
		t.Fatalf("应恰有 1 条中断记录，实际 %d", len(aborted))
	}
	if got := semconvStr(aborted[0], AttrGenAIResponseStatus); got != GenAIStatusCancelled {
		t.Errorf("中断 ⇒ status=%s，实际 %q", GenAIStatusCancelled, got)
	}
	if s := semconvStr(aborted[0], AttrErrorType); s != "" {
		t.Errorf("客户端主动中断不是「操作出错」⇒ 不该有 error.type，实际 %q", s)
	}

	// ④ error.type 必须低基数：闭集内的值 + 认不出的分类码兜底 _OTHER（不许把内部判词/原文塞进去）
	if got := ObsErrorTypeOf("谁也不知道这是什么错误"); got != ErrTypeOther {
		t.Errorf("未登记的分类码应兜底 %q，实际 %q", ErrTypeOther, got)
	}
	for _, m := range semconvFind(lines, func(m map[string]any) bool { return true }) {
		s := semconvStr(m, AttrErrorType)
		if s == "" {
			continue
		}
		if !obsErrTypeClosed(s) {
			t.Errorf("error.type=%q 不在闭集里（高基数风险：原文/判词不许进 error.type）", s)
		}
	}
	for _, v := range obsErrTypeTable {
		if !obsErrTypeClosed(v) {
			t.Errorf("闭集表里有非法值 %q", v)
		}
	}
}

// obsErrTypeClosed — error.type 值是否属于低基数闭集（形态 + 枚举双判）。
func obsErrTypeClosed(v string) bool {
	if v == "" || strings.ContainsAny(v, " \t\n:/（）()") || len(v) > 40 {
		return false
	}
	for _, allowed := range append([]string{ErrTypeOther}, obsErrTypeTable[ChatErrUpstreamTimeout],
		obsErrTypeTable[ChatErrUpstreamFail], obsErrTypeTable[ChatErrStreamBroken],
		obsErrTypeTable[ChatErrStreamTruncated], obsErrTypeTable[ChatErrBadRequest],
		obsErrTypeTable[ChatErrClientAborted], ErrTypeToolExec, ErrTypeCompactionFail) {
		if v == allowed {
			return true
		}
	}
	return false
}

// ── ⑤ conversation.compacted：只 true，绝不 false ──────────────────────────

func TestObsSemconv_CompactedOnlyTrue(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// ① 调用方硬塞 false ⇒ 唯一写入口必须**剥掉**（规范：SHOULD NOT set false；只 true 才有信息）
	f := false
	obsWrite(ObsRecord{Kind: obsKindTurn, Session: "s-cmp-false", EndReason: "finish", GenAIConversationCompacted: &f})

	// ② 压缩失败 ⇒ 不出现
	obsCompactionStart(compactObsFacts{Session: "s-cmp-fail", Model: "m", Trigger: "threshold"}).failed("boom")

	// ③ 压缩成功（三态 completed）⇒ true；**此后同会话**的推理记录也算"压过"（仍只 true）
	h := obsCompactionStart(compactObsFacts{Session: "s-cmp-ok", Model: "m", Trigger: "threshold"})
	h.completed("摘要正文", "【历史摘要】摘要正文", true)
	tm := NewObsTimer("s-cmp-ok", 1, "m")
	tm.MarkChunk()
	tm.Finish("finish")

	// ④ 没压过的会话 ⇒ 键缺席
	tm2 := NewObsTimer("s-cmp-clean", 1, "m")
	tm2.MarkChunk()
	tm2.Finish("finish")

	raw := readObs(t)
	if strings.Contains(raw, `"gen_ai.conversation.compacted":false`) {
		t.Fatalf("出现了 gen_ai.conversation.compacted:false（规范 SHOULD NOT set false；只 true 才有信息）\n原文：%s", raw)
	}
	lines := semconvLines(t)
	if len(lines) != 7 { // false 注入 1 + 失败(起+终) 2 + 成功(起+终) 2 + 两个 turn 2
		t.Fatalf("应落 7 条记录，实际 %d：%s", len(lines), raw)
	}

	// ① 那三条带 false 的路径（注入了 false 的 compact 行 + 失败行）一个都不能出现该键
	for _, m := range semconvFind(lines, func(m map[string]any) bool {
		s := semconvStr(m, "session")
		return s == "s-cmp-false" || s == "s-cmp-fail"
	}) {
		if _, ok := semconvField(m, AttrGenAIConversationCompacted); ok {
			t.Errorf("会话 %s 不该有 compacted 键（只 true 才落）：%v", semconvStr(m, "session"), m)
		}
	}

	// ③ 成功压缩的**终态行**必须是 true
	ok := semconvFind(lines, func(m map[string]any) bool {
		return semconvStr(m, "session") == "s-cmp-ok" && semconvStr(m, "event_name") == obsEventCompactionCompleted
	})
	if len(ok) != 1 || ok[0][AttrGenAIConversationCompacted] != true {
		t.Errorf("压缩成功 ⇒ gen_ai.conversation.compacted 必须为 true：%v", ok)
	}
	// ③′ started 行**不设**（那一刻还不能可靠判定"压缩发生了"——压可能失败）
	st := semconvFind(lines, func(m map[string]any) bool {
		return semconvStr(m, "session") == "s-cmp-ok" && semconvStr(m, "event_name") == obsEventCompactionStarted
	})
	if len(st) != 1 {
		t.Fatalf("应有 1 条 started，实际 %d", len(st))
	}
	if _, has := st[0][AttrGenAIConversationCompacted]; has {
		t.Errorf("started 那一刻不该断言「压缩已发生」（可能失败）")
	}
	// ③″ 同会话**之后**的轮次也带 true（会话的上下文确实被压过）
	tr := semconvFind(lines, func(m map[string]any) bool {
		return semconvStr(m, "session") == "s-cmp-ok" && semconvStr(m, "kind") == obsKindTurn
	})
	if len(tr) != 1 || tr[0][AttrGenAIConversationCompacted] != true {
		t.Errorf("同会话后续轮次也应带 compacted=true：%v", tr)
	}
	// ④ 没压过的会话 ⇒ 键缺席（不写 false、也不写 true）
	clean := semconvFind(lines, func(m map[string]any) bool { return semconvStr(m, "session") == "s-cmp-clean" })
	if len(clean) != 1 {
		t.Fatalf("应有 1 条干净会话记录，实际 %d", len(clean))
	}
	if _, has := clean[0][AttrGenAIConversationCompacted]; has {
		t.Errorf("没压过的会话不该有 compacted 键：%v", clean[0])
	}
}

// ── ⑥ 四终局 → gen_ai.response.status 逐条 ────────────────────────────────

func TestObsSemconv_ResponseStatusFourTerminals(t *testing.T) {
	// ① 四终局逐条（收尾原因 → 终局 → 标准状态）
	four := []struct {
		terminal string
		code     string
		status   string
	}{
		{ObsTerminalCompleted, "finish", GenAIStatusCompleted},
		{ObsTerminalFailed, ChatErrUpstreamTimeout, GenAIStatusFailed},
		{ObsTerminalCancelled, ChatErrClientAborted, GenAIStatusCancelled},
		{ObsTerminalIncomplete, ChatErrStreamTruncated, GenAIStatusIncomplete},
	}
	for _, c := range four {
		if got := ObsTerminalOf(c.code); got != c.terminal {
			t.Errorf("ObsTerminalOf(%q) = %q，期望 %q", c.code, got, c.terminal)
		}
		if got := ObsResponseStatusOf(c.code); got != c.status {
			t.Errorf("ObsResponseStatusOf(%q) = %q，期望 %q", c.code, got, c.status)
		}
		if c.terminal != c.status {
			t.Errorf("终态应**恒等**映射：terminal=%q status=%q", c.terminal, c.status)
		}
	}
	// ② 其余分类码归 failed（不因"没登记"就落空）
	for _, code := range []string{ChatErrUpstreamFail, ChatErrStreamBroken, ChatErrBadRequest, ChatErrOther, "length"} {
		if code == "length" {
			if got := ObsTerminalOf(code); got != ObsTerminalIncomplete {
				t.Errorf("长度截断应归 %s，实际 %q", ObsTerminalIncomplete, got)
			}
			continue
		}
		if got := ObsResponseStatusOf(code); got != GenAIStatusFailed {
			t.Errorf("分类码 %q 应归 %s，实际 %q", code, GenAIStatusFailed, got)
		}
	}
	// ③ 非终态：排队 / 进行中（不是四终局）
	for _, c := range []struct{ code, status string }{
		{GenAIStatusQueued, GenAIStatusQueued},
		{GenAIStatusInProgress, GenAIStatusInProgress},
	} {
		if got := ObsTerminalOf(c.code); got != "" {
			t.Errorf("%s 不是终态，ObsTerminalOf 应为空，实际 %q", c.code, got)
		}
		if got := ObsResponseStatusOf(c.code); got != c.status {
			t.Errorf("ObsResponseStatusOf(%q) = %q，期望 %q", c.code, got, c.status)
		}
	}
	// ④ 认不出 ⇒ 空（**缺席，不编造**）
	if got := ObsResponseStatusOf("谁知道"); got != "" {
		t.Errorf("认不出的收尾原因 ⇒ status 应缺席，实际 %q", got)
	}

	// ⑤ wire 层：四终局各写一条 turn，断言键真的在原文里且与 zerg.terminal 一致
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	for i, c := range four {
		tm := NewObsTimer("s-term", i+1, "m")
		if c.code != ChatErrClientAborted {
			tm.MarkChunk()
		}
		tm.Finish(c.code)
	}
	for _, m := range semconvLines(t) {
		end := semconvStr(m, "end_reason")
		want := ObsResponseStatusOf(end)
		if got := semconvStr(m, AttrGenAIResponseStatus); got != want {
			t.Errorf("end_reason=%q ⇒ gen_ai.response.status=%q，期望 %q", end, got, want)
		}
		if got := semconvStr(m, "zerg.terminal"); got != ObsTerminalOf(end) {
			t.Errorf("end_reason=%q ⇒ zerg.terminal=%q，期望 %q（四终局留在 zerg.* —— B12）", end, got, ObsTerminalOf(end))
		}
	}
}

// ── ⑦ conversation.id：拿不到就缺席，绝不编造 ──────────────────────────────

func TestObsSemconv_ConversationIDAbsentNotFabricated(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// ① 无会话 ⇒ 键缺席（不许用新 UUID / trace_id / 内容 hash 兜底 —— B10）
	obsWrite(ObsRecord{Kind: obsKindTool, Tool: "bash", Result: "ok"})

	// ② 有会话 ⇒ 等于会话 id（应用侧确实提供了它 ⇒ 这才是规范允许的来源）
	tm := NewObsTimer("sess-conv-1", 1, "m")
	tm.MarkChunk()
	tm.Finish("finish")

	lines := semconvLines(t)
	if len(lines) != 2 {
		t.Fatalf("应有 2 条记录，实际 %d", len(lines))
	}
	var noSess, withSess map[string]any
	for _, m := range lines {
		if semconvStr(m, "session") == "" {
			noSess = m
		} else {
			withSess = m
		}
	}
	if noSess == nil || withSess == nil {
		t.Fatalf("记录分组不对：%v", lines)
	}
	if _, ok := semconvField(noSess, AttrGenAIConversationID); ok {
		t.Errorf("无会话 ⇒ gen_ai.conversation.id 必须**缺席**（不编造 id 兜底）：%v", noSess)
	}
	if got := semconvStr(withSess, AttrGenAIConversationID); got != "sess-conv-1" {
		t.Errorf("conversation.id = %q，期望会话 id（应用侧给的）", got)
	}
	// ③ 不许拿 trace_id 冒充 conversation.id（两者必须可分）
	if tid := semconvStr(withSess, "trace_id"); tid == "" || tid == "sess-conv-1" {
		t.Errorf("用例前提失效：trace_id 应非空且 ≠ 会话 id，实际 %q", tid)
	}
	if semconvStr(withSess, AttrGenAIConversationID) == semconvStr(withSess, "trace_id") {
		t.Errorf("conversation.id 与 trace_id 同值 ⇒ 有「拿 trace 兜底」的嫌疑")
	}
}

// ── ⑧ 命名空间纪律：gen_ai.* 闭集 / 自造字段一律 zerg.* / 内容类 Opt-In ──────

func TestObsSemconv_NamespaceDiscipline(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	t.Setenv(ObsCaptureEnvLevel, "OFF") // 默认档：不采内容

	tm := NewObsTimer("sess-ns", 1, "gemma-4-26B")
	tm.MarkChunk()
	tm.SetUsage(100, 20, 10)
	tm.Finish("finish")
	ObsTool("sess-ns", 1, "web_search", "9ms", true, 1, MaxToolRounds, "call_x")
	obsCompact("sess-ns", "threshold", 0, 0, 0, 0, "ok", "", false)
	obsWrite(ObsRecord{Kind: obsKindPrompt, Session: "sess-ns", PromptName: "chat.tiered_system", PromptVersion: "0123456789abcdef"})

	lines := semconvLines(t)
	if len(lines) != 4 {
		t.Fatalf("应落 4 条记录，实际 %d", len(lines))
	}
	// ① 带点的键：标准名（闭集内）或 zerg.* —— 二者必居其一
	for _, m := range lines {
		for k := range m {
			if !strings.Contains(k, ".") || strings.HasPrefix(k, "zerg.") {
				continue
			}
			if !ObsSemconvEmitted(k) {
				t.Errorf("键 %q 既不在标准闭集、也不带 zerg. 前缀 ⇒ 自造字段伪装标准字段", k)
			}
		}
		if semconvStr(m, "zerg.semconv.version") == "" {
			t.Errorf("每条记录都必须带 semconv 版本（B15）：%v", m)
		}
	}
	// ② 自造字段**不许**带 gen_ai. 前缀（逐条点名：这些是我们私有的，规范里没有）
	raw := readObs(t)
	for _, bad := range []string{
		"gen_ai.prompt_fingerprint", "gen_ai.rendered_prefix_hash", "gen_ai.template_hash",
		"gen_ai.verdict", "gen_ai.args_digest", "gen_ai.deny_reason", "gen_ai.zerg",
		"gen_ai.request_seed", "gen_ai.clock_iso", "gen_ai.geometry",
	} {
		if strings.Contains(raw, bad) {
			t.Errorf("自造字段不许伪装标准名：出现了 %q", bad)
		}
	}
	// ③ 内容类（Opt-In）标准属性在默认档下一个都不许出现（B6）
	if ObsContentAttrNames() == nil || len(ObsContentAttrNames()) == 0 {
		t.Fatalf("内容类属性清单为空 ⇒ 本用例失去判别力")
	}
	for _, name := range ObsContentAttrNames() {
		if strings.Contains(raw, name) {
			t.Errorf("默认档（NO_CONTENT）不许落内容类属性 %q", name)
		}
	}
	// ④ 我们的四终局/verdict 留在 zerg.*（B12：不得塞进 error.type 或冒充标准属性）
	if !strings.Contains(raw, `"zerg.terminal":"completed"`) {
		t.Errorf("四终局应落在 zerg.terminal：%s", raw)
	}
}

// ── ⑨ 标准字段逐项（turn / tool / prompt 三类现场记录）────────────────────

func TestObsSemconv_StandardFieldsOnRecords(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	ObsSetInferEndpoint("http://127.0.0.1:18082")
	defer ObsSetInferEndpoint("")

	tm := NewObsTimer("sess-std", 2, "gemma-4-26B")
	time.Sleep(5 * time.Millisecond)
	tm.MarkChunk() // 首字节（TTFC 的测量点）
	tm.MarkChunk()
	tm.SetUsage(1200, 340, 200)
	tm.Finish("finish")
	ObsTool("sess-std", 2, "web_search", "9ms", true, 2, MaxToolRounds, "call_mszuSIzqtI65i1wAUOE8w5H4")
	ObsTool("sess-std", 2, "kb_search", "3ms", true, 3, MaxToolRounds)
	ObsTool("sess-std", 2, "谁也不是的工具", "1ms", true, 4, MaxToolRounds)
	obsWrite(ObsRecord{Kind: obsKindPrompt, Session: "sess-std", Round: 2, Model: "gemma-4-26B",
		PromptName: "chat.tiered_system", PromptVersion: "0123456789abcdef"})

	lines := semconvLines(t)
	byKind := func(kind string) []map[string]any {
		return semconvFind(lines, func(m map[string]any) bool { return semconvStr(m, "kind") == kind })
	}
	turn := byKind(obsKindTurn)
	if len(turn) != 1 {
		t.Fatalf("应有 1 条 turn，实际 %d", len(turn))
	}
	tmRec := turn[0]
	checks := []struct {
		key   string
		want  any
		why   string
		exact bool
	}{
		{AttrGenAIOperationName, GenAIOperationChat, "kind=turn ⇒ operation.name=chat（B2/映射表）", true},
		{AttrGenAIRequestModel, "gemma-4-26B", "请求模型（推理 span 上才有）", true},
		{AttrGenAIProviderName, ProviderZergLocal, "自研集群 ⇒ 文档化的自定义 provider 值", true},
		{AttrGenAIRequestStream, true, "收到过分块 ⇒ 是流式（未设置会被假定非流式）", true},
		{AttrGenAIRequestReasoningLevel, reasoningEffortChat, "思考模型必记；与请求体同源", true},
		{AttrGenAIOutputType, GenAIOutputTypeText, "输出模态", true},
		{AttrGenAIResponseStatus, GenAIStatusCompleted, "终态", true},
		{AttrGenAIConversationID, "sess-std", "会话 id", true},
		{AttrServerAddress, "127.0.0.1", "**Stable**：多节点定位", true},
		{AttrServerPort, float64(18082), "**Stable**：端口", true},
		{"zerg.span.name", "chat gemma-4-26B", "标准 span 名（映射表算出）", true},
	}
	for _, c := range checks {
		v, ok := semconvField(tmRec, c.key)
		if !ok {
			t.Errorf("turn 记录缺 %s（%s）", c.key, c.why)
			continue
		}
		if c.exact && v != c.want {
			t.Errorf("%s = %v，期望 %v（%s）", c.key, v, c.want, c.why)
		}
	}
	// TTFC：秒且 > 0（我们 sleep 了 5ms）；必须是**数值**（double）
	if v, ok := tmRec[AttrGenAIResponseTimeToFirstChunk]; !ok {
		t.Errorf("流式且收到分块 ⇒ 必须有 %s", AttrGenAIResponseTimeToFirstChunk)
	} else if f, isF := v.(float64); !isF || f <= 0 {
		t.Errorf("%s = %v（应为 >0 的秒数）", AttrGenAIResponseTimeToFirstChunk, v)
	}
	// usage：计费口径三件套都在，且 reasoning 含在 output 内（规范口径）
	for _, k := range []string{AttrGenAIUsageInputTokens, AttrGenAIUsageOutputTokens, AttrGenAIUsageReasoningTokens} {
		if _, ok := tmRec[k]; !ok {
			t.Errorf("turn 记录缺 %s", k)
		}
	}
	if in, _ := tmRec[AttrGenAIUsageInputTokens].(float64); in != 1200 {
		t.Errorf("input_tokens = %v，期望 1200", tmRec[AttrGenAIUsageInputTokens])
	}
	if out, _ := tmRec[AttrGenAIUsageOutputTokens].(float64); out != 340 {
		t.Errorf("output_tokens = %v，期望 340", tmRec[AttrGenAIUsageOutputTokens])
	}
	if r, _ := tmRec[AttrGenAIUsageReasoningTokens].(float64); r != 200 || r > 340 {
		t.Errorf("reasoning.output_tokens = %v（应=200 且 ≤ output_tokens）", tmRec[AttrGenAIUsageReasoningTokens])
	}
	// prompt.name/version 只挂在 prompt 记录上（turn 记录不许编造）
	if _, ok := tmRec[AttrGenAIPromptName]; ok {
		t.Errorf("turn 记录不该有 %s（我们只在提示装配处才知道）", AttrGenAIPromptName)
	}

	// 工具三件套 + call.id（三类：extension / datastore / 未登记）
	tools := byKind(obsKindTool)
	if len(tools) != 3 {
		t.Fatalf("应有 3 条工具记录，实际 %d", len(tools))
	}
	web := semconvFind(lines, func(m map[string]any) bool { return semconvStr(m, "tool") == "web_search" })[0]
	if got := semconvStr(web, AttrGenAIToolName); got != "web_search" {
		t.Errorf("gen_ai.tool.name = %q", got)
	}
	if got := semconvStr(web, AttrGenAIToolType); got != GenAIToolTypeExtension {
		t.Errorf("web_search 的 tool.type = %q，期望 %s（桥接外部 API）", got, GenAIToolTypeExtension)
	}
	if got := semconvStr(web, AttrGenAIToolCallID); got != "call_mszuSIzqtI65i1wAUOE8w5H4" {
		t.Errorf("gen_ai.tool.call.id = %q（model 侧 tool_call ↔ 执行侧 span 的对接键）", got)
	}
	if got := semconvStr(web, AttrGenAIOperationName); got != GenAIOperationExecuteTool {
		t.Errorf("工具记录 operation.name = %q，期望 %s", got, GenAIOperationExecuteTool)
	}
	if got := semconvStr(web, "zerg.span.name"); got != "execute_tool web_search" {
		t.Errorf("工具 span 名 = %q，期望 execute_tool web_search（B5）", got)
	}
	kb := semconvFind(lines, func(m map[string]any) bool { return semconvStr(m, "tool") == "kb_search" })[0]
	if got := semconvStr(kb, AttrGenAIToolType); got != GenAIToolTypeDatastore {
		t.Errorf("kb_search 的 tool.type = %q，期望 %s", got, GenAIToolTypeDatastore)
	}
	if _, ok := semconvField(kb, AttrGenAIToolCallID); ok {
		t.Errorf("没给 call.id 的工具不许编造 gen_ai.tool.call.id")
	}
	unknown := semconvFind(lines, func(m map[string]any) bool { return semconvStr(m, "tool") == "谁也不是的工具" })[0]
	if _, ok := semconvField(unknown, AttrGenAIToolType); ok {
		t.Errorf("未登记的工具 ⇒ tool.type 应缺席（不猜），实际 %v", unknown[AttrGenAIToolType])
	}
	if got := semconvStr(unknown, AttrGenAIToolName); got != "谁也不是的工具" {
		t.Errorf("tool.name = %q（工具名是既有事实，照落）", got)
	}

	// prompt 记录：名字/版本映射到标准名
	prompt := byKind(obsKindPrompt)
	if len(prompt) != 1 {
		t.Fatalf("应有 1 条 prompt 记录，实际 %d", len(prompt))
	}
	if got := semconvStr(prompt[0], AttrGenAIPromptName); got != "chat.tiered_system" {
		t.Errorf("gen_ai.prompt.name = %q", got)
	}
	if got := semconvStr(prompt[0], AttrGenAIPromptVersion); got != "0123456789abcdef" {
		t.Errorf("gen_ai.prompt.version = %q", got)
	}

	// 推理对端被清掉之后 ⇒ server.* **缺席**（拿不到就不写，不编造）
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	ObsSetInferEndpoint("")
	obsWrite(ObsRecord{Kind: obsKindTurn, Session: "sess-nostd", EndReason: "finish"})
	noEnd := semconvLines(t)
	if len(noEnd) != 1 {
		t.Fatalf("应有 1 条记录，实际 %d", len(noEnd))
	}
	if _, ok := noEnd[0][AttrServerAddress]; ok {
		t.Errorf("拿不到推理对端 ⇒ server.address 必须缺席：%v", noEnd[0])
	}
}

// ── ⑩ span 命名 + semconv 版本常量 ────────────────────────────────────────

func TestObsSemconv_SpanNamingAndVersion(t *testing.T) {
	// ① 版本常量（B15：事件里的 zerg.semconv.* 就是它）
	if SemconvGenAIVersion != "semantic-conventions-genai@c88d504, 2026-09-16" {
		t.Errorf("semconv 基准串 = %q，期望调研核实的基准", SemconvGenAIVersion)
	}
	if SemconvGenAICommit != "c88d504" || SemconvGenAIDate != "2026-09-16" {
		t.Errorf("commit/日期 = %q/%q", SemconvGenAICommit, SemconvGenAIDate)
	}

	// ② span 名逐条（映射表的 OTelName 与拼装函数必须一致）
	cases := []struct {
		kind, tool, model, wantName, wantOp string
	}{
		{obsKindTurn, "", "gemma-4-26B", "chat gemma-4-26B", GenAIOperationChat},
		{obsKindTurn, "", "", "chat", GenAIOperationChat},
		{obsKindTool, "bash", "", "execute_tool bash", GenAIOperationExecuteTool},
		{obsKindTool, "", "", "execute_tool", GenAIOperationExecuteTool},
		{obsKindCompact, "", "llmlingua", "chat llmlingua", GenAIOperationChat},
		{obsKindPrompt, "", "m", "chat m", GenAIOperationChat},
		{"run", "", "", "invoke_workflow", GenAIOperationInvokeWorkflow}, // B19：根 span 本质是 workflow
	}
	for _, c := range cases {
		if got := ObsSpanName(c.kind, c.tool, c.model); got != c.wantName {
			t.Errorf("ObsSpanName(%q,%q,%q) = %q，期望 %q", c.kind, c.tool, c.model, got, c.wantName)
		}
		if got := ObsOperationNameOf(c.kind); got != c.wantOp {
			t.Errorf("ObsOperationNameOf(%q) = %q，期望 %q", c.kind, got, c.wantOp)
		}
	}

	// ③ 规范里**没有**对应项的私有信号（早退/空转）⇒ 不落 operation.name / span 名（不冒充）
	for _, kind := range []string{obsKindBehavior, "", "未知kind"} {
		if got := ObsSpanName(kind, "bash", "m"); got != "" {
			t.Errorf("kind=%q 无标准对应项 ⇒ span 名应缺席，实际 %q", kind, got)
		}
		if got := ObsOperationNameOf(kind); got != "" {
			t.Errorf("kind=%q 无标准对应项 ⇒ operation.name 应缺席，实际 %q", kind, got)
		}
	}

	// ④ 根 span 行必须在表里且是 workflow（B19：低基数、不得用类型名）
	m, ok := ObsSpanMappingOf("run")
	if !ok || m.Operation != GenAIOperationInvokeWorkflow {
		t.Errorf("映射表缺根 span 行（run ⇒ invoke_workflow）：%+v ok=%v", m, ok)
	}
	if !strings.HasPrefix(m.OTelName, "invoke_workflow") {
		t.Errorf("根 span 名模板 = %q，应为 invoke_workflow {gen_ai.workflow.name}", m.OTelName)
	}
}
