// chat_infer_args_string_test.go —— 缺口 GAP-20261019-07 的判据件。
//
// 病（实测）：本地卵会发「arguments 是已拼好的命令串」这种工具调用：
//
//	<tool_call>{"name": "zerg", "arguments": "zerg code find 'x' --path y"}</tool_call>
//
// 旧字段类型是 map[string]any ⇒ 解码失败 ⇒ 整轮 bad_format、任务一步不走。
//
// 治法：toolArgs 三态解码 —— 对象照收；字符串且自身是 JSON 对象则解析；否则包成 {"command": 串}。
// 两态：① 字符串形态必须解析出 1 条调用且 command 逐字等于原串；② 标准对象形态零回归。
package chat

import "testing"

func TestParseXMLToolCallsArgsAsString(t *testing.T) {
	content := `<tool_call>{"name": "zerg", "arguments": "zerg code find 'frpReaderFace' --path core/cmd/zerg"}</tool_call>`
	calls := parseXMLToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("字符串形态应解析出 1 条调用，实得 %d 条", len(calls))
	}
	if calls[0].Name != "zerg" {
		t.Fatalf("工具名 = %q，期望 zerg", calls[0].Name)
	}
	got, _ := calls[0].Args["command"].(string)
	want := "zerg code find 'frpReaderFace' --path core/cmd/zerg"
	if got != want {
		t.Fatalf("command = %q，期望逐字 %q", got, want)
	}
}

func TestParseXMLToolCallsArgsAsObjectUnchanged(t *testing.T) {
	content := `<tool_call>{"name": "zerg", "arguments": {"command": "zerg gap ls"}}</tool_call>`
	calls := parseXMLToolCalls(content)
	if len(calls) != 1 {
		t.Fatalf("对象形态应解析出 1 条调用，实得 %d 条", len(calls))
	}
	got, _ := calls[0].Args["command"].(string)
	if got != "zerg gap ls" {
		t.Fatalf("对象形态 command = %q，期望 zerg gap ls（零回归）", got)
	}
	if _, ok := calls[0].Args["arguments"]; ok {
		t.Fatalf("对象形态被多包了一层：%#v", calls[0].Args)
	}
}
