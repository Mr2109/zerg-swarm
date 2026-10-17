// chat_infer_loose_test.go — GAP-20261017-14：文本工具调用漏解析（P0）回归牙。
//
// fixture 全部是**真字节**（逐字取自会话库 ~/.zerg-chat/chat.db 的 messages.content）：
//
//	fx9938cd = chat_20261017_153351_9938cd 末轮（id 663）——无标签 ＋ 裸参数对象
//	fx603050 = chat_20261017_144532_603050 末轮（id 657）——同上，且值里有未转义引号
//	fx06     = chat_20261017_123145_b9ae07 末轮（id 647）——多开缺闭（GAP-20261017-06）
//
// 反向断言（缺一不可）：
//
//	① 正常单标签仍解析为 1 调用（格式 1 回归锁）
//	② 真无 JSON 的真坏格式仍判坏格式（0 调用 ＋ 原文不动 ⇒ 上游 bad_format 阶梯照旧接管）
//	③ 多开缺闭的真字节 fixture 必须解析出 1 调用
package chat

import (
	"strings"
	"testing"
)

const fx9938cd = "先看 family_agent.go 100-110 附近的列定义。已经有 recent_requests 在列里了。但需要确认\"来源\"列。判据要求断言 recent_requests 非空。让我看 main.go 1130-1140。\n{\"command\":\"zerg code show core/cmd/zerg/main.go:1133\"}"

const fx603050 = "计划件无误：路径逐字对，块新建在 models 段末，不覆盖别人，校验通过，未标 verified（我没给 --verified）。现在真写加 --yes。\n{\"command\":\"zerg model add --model Nemotron-3.5-Super-VL-120B-A12B --host edge-node --backend llama-server --file /data/models/nemotron/NVIDIA-Nemotron-3.5-Super-VL-120B-A12B/Q4_0/NVIDIA-Nemotron-3.5-Super-VL-120B-A12B-Q4_0.gguf --size-bytes 71803752384 --mem-gb 72 --ctx 32768 --mmproj /data/models/nemotron/NVIDIA-Nemotron-3.5-Super-VL-120B-A12B/Q4_0/mmproj-NVIDIA-Nemotron-3.5-Super-VL-120B-A12B-BF16.gguf --confirm Nemotron-3.5-Super-VL-120B-A12B --yes 2>&1 | head -120; echo \"RC=$?\"\"}"

const fx06 = "- want_family: repo\n- want_action: docs write 文档落件专口\n- verify_cmd: zerg code find repo[[:space:]]docs --path core/cmd/zerg/main.go\n- state: 仍缺\n\nNow I need to run the verify_cmd for GAP-20261015-09: `zerg code find repo[[:space:]]docs --path core/cmd/zerg/main.go`\n<tool_call>\n<tool_call>\n<tool_call>\n<tool_call>\n<tool_call>\n{\"command\":\"zerg code find repo[[:space:]]docs --path core/cmd/zerg/main.go\"}\n</tool_call>\n\n【已保存，可续跑】 有进展且有明确待办 ⇒ 落检查点后保存状态续跑（退出原因：bad_format）"

// zergFace — 与线上一致的唯一工具面（真源：ChatZergToolParam）
func zergFace() *toolFace {
	return soleToolFace([]map[string]any{ChatZergToolParam()})
}

// TestLooseToolCalls_Fixture9938cd — 无标签 ＋ 裸参数对象（线上漏解析的那一形态）
func TestLooseToolCalls_Fixture9938cd(t *testing.T) {
	face := zergFace()
	if face == nil || face.name != "zerg" {
		t.Fatalf("唯一工具面取错: %+v", face)
	}
	calls, rest, broken := parseTextToolCalls(fx9938cd, face)
	if len(calls) != 1 {
		t.Fatalf("期望 1 调用，得 %d（broken=%v rest=%q）", len(calls), broken, rest)
	}
	if calls[0].Name != "zerg" {
		t.Errorf("工具名错: %q", calls[0].Name)
	}
	if got, _ := calls[0].Args["command"].(string); got != "zerg code show core/cmd/zerg/main.go:1133" {
		t.Errorf("command 不逐字: %q", got)
	}
	if strings.Contains(rest, `{"command"`) {
		t.Errorf("正文里仍留着参数对象: %q", rest)
	}
	if !strings.HasPrefix(rest, "先看 family_agent.go") {
		t.Errorf("正文（模型自述）丢了: %q", rest)
	}
}

// TestLooseToolCalls_Fixture603050 — 同为裸参数对象，且值里有未转义引号（JSON 不合法）
func TestLooseToolCalls_Fixture603050(t *testing.T) {
	calls, rest, broken := parseTextToolCalls(fx603050, zergFace())
	if len(calls) != 1 {
		t.Fatalf("期望 1 调用，得 %d（broken=%v rest=%q）", len(calls), broken, rest)
	}
	if calls[0].Name != "zerg" {
		t.Errorf("工具名错: %q", calls[0].Name)
	}
	if got, _ := calls[0].Args["command"].(string); got != "zerg model add --model Nemotron-3.5-Super-VL-120B-A12B --host edge-node --backend llama-server --file /data/models/nemotron/NVIDIA-Nemotron-3.5-Super-VL-120B-A12B/Q4_0/NVIDIA-Nemotron-3.5-Super-VL-120B-A12B-Q4_0.gguf --size-bytes 71803752384 --mem-gb 72 --ctx 32768 --mmproj /data/models/nemotron/NVIDIA-Nemotron-3.5-Super-VL-120B-A12B/Q4_0/mmproj-NVIDIA-Nemotron-3.5-Super-VL-120B-A12B-BF16.gguf --confirm Nemotron-3.5-Super-VL-120B-A12B --yes 2>&1 | head -120; echo \"RC=$?\"" {
		t.Errorf("command 不逐字: %q", got)
	}
	if !strings.Contains(rest, "现在真写加 --yes") {
		t.Errorf("正文丢了: %q", rest)
	}
}

// TestLooseToolCalls_Fixture06MultiOpen — 多开缺闭（GAP-20261017-06 真字节）必须解析出 1 调用
func TestLooseToolCalls_Fixture06MultiOpen(t *testing.T) {
	calls, rest, broken := parseTextToolCalls(fx06, zergFace())
	if len(calls) != 1 {
		t.Fatalf("期望 1 调用，得 %d（broken=%v）", len(calls), broken)
	}
	if calls[0].Name != "zerg" {
		t.Errorf("工具名错: %q", calls[0].Name)
	}
	if got, _ := calls[0].Args["command"].(string); got != "zerg code find repo[[:space:]]docs --path core/cmd/zerg/main.go" {
		t.Errorf("command 不逐字: %q", got)
	}
	if strings.Contains(rest, "<tool_call>") || strings.Contains(rest, "</tool_call>") {
		t.Errorf("标签没剥干净: %q", rest)
	}
}

// TestLooseToolCalls_NormalSingleTagUnaffected — ① 正常单标签仍解析为 1 调用（回归锁）
func TestLooseToolCalls_NormalSingleTagUnaffected(t *testing.T) {
	content := "我来跑一下。\n<tool_call>{\"name\": \"zerg\", \"arguments\": {\"command\": \"zerg version\"}}</tool_call>"
	calls, rest, broken := parseTextToolCalls(content, zergFace())
	if len(calls) != 1 || calls[0].Name != "zerg" {
		t.Fatalf("单标签回归: calls=%d broken=%v", len(calls), broken)
	}
	if got, _ := calls[0].Args["command"].(string); got != "zerg version" {
		t.Errorf("args 错: %q", got)
	}
	if strings.Contains(rest, "<tool_call>") || !strings.Contains(rest, "我来跑一下") {
		t.Errorf("剥离/正文错: %q", rest)
	}
}

// TestLooseToolCalls_BrokenFormatStillBadFormat — ② 真无 JSON 的真坏格式：0 调用 ＋ 原文不动
// （原文含 <tool_call> ⇒ 上游 loopcore 的坏格式阶梯照旧触发，绝不静默当正文）
func TestLooseToolCalls_BrokenFormatStillBadFormat(t *testing.T) {
	content := "换几种写法试试。\n<tool_call>\n<function=bash>\n</tool_call>"
	calls, rest, broken := parseTextToolCalls(content, zergFace())
	if len(calls) != 0 {
		t.Fatalf("坏格式不该解出调用: %d", len(calls))
	}
	if !broken {
		t.Errorf("坏格式必须报 broken（供上游回灌）")
	}
	if rest != content {
		t.Errorf("坏格式原文被改动: %q", rest)
	}
	if !strings.Contains(rest, "<tool_call>") {
		t.Errorf("坏格式标记丢了 ⇒ 上游认不出坏格式")
	}
}

// TestLooseToolCalls_NoFaceDoesNotGuess — 多工具面（face=nil）时裸参数对象不认归属（不猜）
func TestLooseToolCalls_NoFaceDoesNotGuess(t *testing.T) {
	calls, rest, _ := parseTextToolCalls(fx9938cd, nil)
	if len(calls) != 0 {
		t.Fatalf("多工具面不该猜: %d", len(calls))
	}
	if rest != fx9938cd {
		t.Errorf("原文被改动")
	}
}

// TestLooseToolCalls_TruncatedTailDoesNotBecomeBody — 截断的裸参数对象 ⇒ 裹上标签交上游判坏格式
func TestLooseToolCalls_TruncatedTailDoesNotBecomeBody(t *testing.T) {
	content := "现在跑。\n{\"command\":\"zerg version"
	calls, rest, broken := parseTextToolCalls(content, zergFace())
	if len(calls) != 0 {
		t.Fatalf("截断不该解出调用: %d", len(calls))
	}
	if !broken || !strings.Contains(rest, "<tool_call>") {
		t.Errorf("截断残片必须裹成 <tool_call> 交上游（broken=%v rest=%q）", broken, rest)
	}
	if !strings.HasSuffix(rest, "</tool_call>") {
		t.Errorf("残片没被裹住（应以闭标签收尾）: %q", rest)
	}
}

// TestSoleToolFace — 唯一工具面判定（多工具/无函数体 ⇒ nil）
func TestSoleToolFace(t *testing.T) {
	f := soleToolFace([]map[string]any{ChatZergToolParam()})
	if f == nil || f.name != "zerg" || len(f.props) != 1 || f.props[0] != "command" {
		t.Fatalf("单工具面取错: %+v", f)
	}
	if f2 := soleToolFace([]map[string]any{ChatZergToolParam(), {"type": "function"}}); f2 != nil {
		t.Errorf("多工具面应 nil: %+v", f2)
	}
	if f3 := soleToolFace([]map[string]any{{"type": "function", "function": "x"}}); f3 != nil {
		t.Errorf("畸形工具应 nil: %+v", f3)
	}
}

// ── GAP-20261018-08：XML 风格调用 `<invoke name="x"><parameter name="k">v</parameter></invoke>` ──────────
//
// 病灶（对话真跑 · 已亲核）：模型把调用退化成这段 XML 原文 ⇒ 0 调用、原文当正文交付，
// 同一调用还会重发两次。既有四档都不认 `name="…"` 的取属性写法 ⇒ 这里补第五档。
//
// 两态 ＋ 三条负控（负控都是「讲解/举例」形态 —— 拿不准就当文本）：
//
//	态① 真调用（独立成行 ＋ 成对闭合）⇒ 1 条 tool_call，name=zerg、command 逐字；
//	负控1 内联在句子里的「示例：`<invoke …>` 这种格式」⇒ **0 条**，原文一字不动；
//	负控2 ``` 代码栅栏里的标签块 ⇒ **0 条**（讲解常裹栅栏）；
//	负控3 缺闭标签 ⇒ **0 条**（不猜、不补，交上游 bad_format 阶梯）。

// xmlInvokeFixture — 线上真撞的形态：一句正文 ＋ 下一行独立成行的 XML 风格调用。
const xmlInvokeFixture = "读了第一篇，先看门禁那一段。\n" +
	`<invoke name="zerg"><parameter name="command">zerg fs read ../Zerg-内部文档/项目文档/v2.5.15/任务单-虫族数据库-20261014.md --allow-sibling-repo Zerg-内部文档</parameter></invoke>`

// TestXMLInvokeCalls_RealShapeParsed — 态①：真调用必须解析成 1 条，且与既有各档汇合（同一个 ToolCall 出口）。
func TestXMLInvokeCalls_RealShapeParsed(t *testing.T) {
	calls, rest, broken := parseTextToolCalls(xmlInvokeFixture, zergFace())
	if len(calls) != 1 {
		t.Fatalf("期望 1 调用，得 %d（broken=%v rest=%q）", len(calls), broken, rest)
	}
	if calls[0].Name != "zerg" {
		t.Errorf("工具名错: %q", calls[0].Name)
	}
	wantCmd := "zerg fs read ../Zerg-内部文档/项目文档/v2.5.15/任务单-虫族数据库-20261014.md --allow-sibling-repo Zerg-内部文档"
	if got, _ := calls[0].Args["command"].(string); got != wantCmd {
		t.Errorf("command 不逐字:\n得 %q\n要 %q", got, wantCmd)
	}
	if strings.Contains(rest, "<invoke") || strings.Contains(rest, "</invoke>") {
		t.Errorf("调用块没剥干净（会和正文一起回灌模型）: %q", rest)
	}
	if !strings.Contains(rest, "读了第一篇") {
		t.Errorf("正文（模型自述）丢了: %q", rest)
	}
	if broken {
		t.Errorf("真调用不该报 broken")
	}
}

// TestXMLInvokeCalls_InlineExplanationIsText — 负控1：正文里**讲解**这种标签（内联在句子里）⇒ 0 条。
func TestXMLInvokeCalls_InlineExplanationIsText(t *testing.T) {
	content := "示例：`<invoke name=\"zerg\"><parameter name=\"command\">zerg version</parameter></invoke>` 这种格式模型偶尔会吐。"
	calls, rest, broken := parseTextToolCalls(content, zergFace())
	if len(calls) != 0 {
		t.Fatalf("解释性内容被当成调用了（%d 条）：%+v", len(calls), calls)
	}
	if rest != content {
		t.Errorf("原文被改动（解释性内容必须一字不动）：%q", rest)
	}
	if broken {
		t.Errorf("解释性内容不该报 broken")
	}
}

// TestXMLInvokeCalls_FencedExampleIsText — 负控2：``` 代码栅栏里的标签块 = 举例 ⇒ 0 条。
func TestXMLInvokeCalls_FencedExampleIsText(t *testing.T) {
	content := "它偶尔会吐这种原文：\n\n```\n<invoke name=\"zerg\"><parameter name=\"command\">zerg version</parameter></invoke>\n```\n\n我们要把它兜住。"
	calls, rest, broken := parseTextToolCalls(content, zergFace())
	if len(calls) != 0 {
		t.Fatalf("栅栏里的举例被当成调用了（%d 条）：%+v", len(calls), calls)
	}
	if rest != content {
		t.Errorf("原文被改动：%q", rest)
	}
	if broken {
		t.Errorf("栅栏举例不该报 broken")
	}
}

// TestXMLInvokeCalls_UnclosedAndBadNameAreText — 负控3：缺闭标签 / 名字不是工具名形状 ⇒ 0 条（不猜不补）。
func TestXMLInvokeCalls_UnclosedAndBadNameAreText(t *testing.T) {
	for _, content := range []string{
		"现在读。\n<invoke name=\"zerg\"><parameter name=\"command\">zerg version</parameter>",
		"现在读。\n<invoke name=\"zerg fs read\"><parameter name=\"command\">zerg version</parameter></invoke>",
		"现在读。\n<invoke name=\"zerg\"></invoke>", // 空参数表 ⇒ 无效调用
	} {
		calls, rest, _ := parseTextToolCalls(content, zergFace())
		if len(calls) != 0 {
			t.Errorf("不该解出调用（%q）：%+v", content, calls)
		}
		if rest != content {
			t.Errorf("原文被改动（%q）：%q", content, rest)
		}
	}
}

// TestXMLInvokeCalls_TwoBlocksAndMultiParam — 两块各自成条 ＋ 多参数逐字（不被并成一条、不丢第二条）。
func TestXMLInvokeCalls_TwoBlocksAndMultiParam(t *testing.T) {
	content := "先跑两条。\n" +
		`<invoke name="zerg"><parameter name="command">zerg version</parameter></invoke>` + "\n" +
		`<invoke name="zerg"><parameter name="command">zerg fs read a.md --limit 10</parameter><parameter name="path">a.md</parameter></invoke>`
	calls, rest, broken := parseTextToolCalls(content, zergFace())
	if len(calls) != 2 {
		t.Fatalf("期望 2 调用，得 %d（broken=%v rest=%q）", len(calls), broken, rest)
	}
	if got, _ := calls[0].Args["command"].(string); got != "zerg version" {
		t.Errorf("第一条 command 错: %q", got)
	}
	if got, _ := calls[1].Args["command"].(string); got != "zerg fs read a.md --limit 10" {
		t.Errorf("第二条 command 错: %q", got)
	}
	if got, _ := calls[1].Args["path"].(string); got != "a.md" {
		t.Errorf("第二条 path 参数错: %q", got)
	}
	if strings.Contains(rest, "<invoke") {
		t.Errorf("两块没剥干净: %q", rest)
	}
}

// TestXMLInvokeCalls_AcceptanceZergVersion — 验收原句：喂一条 `<invoke name="zerg"><parameter name="command">zerg version</parameter></invoke>`
// 的模型输出 ⇒ 解析成**一条** tool_call，name=zerg、command=zerg version。
func TestXMLInvokeCalls_AcceptanceZergVersion(t *testing.T) {
	content := "我先确认一下版本。\n" +
		`<invoke name="zerg"><parameter name="command">zerg version</parameter></invoke>`
	calls, rest, broken := parseTextToolCalls(content, zergFace())
	if len(calls) != 1 {
		t.Fatalf("期望 1 调用，得 %d（broken=%v）", len(calls), broken)
	}
	if calls[0].Name != "zerg" {
		t.Errorf("name 要 zerg：%q", calls[0].Name)
	}
	if got, _ := calls[0].Args["command"].(string); got != "zerg version" {
		t.Errorf("command 要逐字 zerg version：%q", got)
	}
	if rest != "我先确认一下版本。" {
		t.Errorf("剥离后正文应只剩那句自述：%q", rest)
	}
}

// TestXMLInvokeCalls_EntityUnescape — 值里的 XML 实体还原（&lt; &gt; &amp;），其余逐字保留。
func TestXMLInvokeCalls_EntityUnescape(t *testing.T) {
	content := "跑。\n" +
		`<invoke name="zerg"><parameter name="command">zerg code find a&lt;b&gt;&amp;c</parameter></invoke>`
	calls, _, _ := parseTextToolCalls(content, zergFace())
	if len(calls) != 1 {
		t.Fatalf("期望 1 调用，得 %d", len(calls))
	}
	if got, _ := calls[0].Args["command"].(string); got != "zerg code find a<b>&c" {
		t.Errorf("实体还原错: %q", got)
	}
}
