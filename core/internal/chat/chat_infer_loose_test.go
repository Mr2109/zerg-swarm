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
