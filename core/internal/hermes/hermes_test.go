package hermes

import (
	"strings"
	"testing"
)

func TestParseHermesJSON(t *testing.T) {
	content := "<tool_call>\n{\"name\": \"write\", \"arguments\": {\"path\": \"a.go\", \"content\": \"code\"}}\n</tool_call>"
	calls := ParseXMLToolCalls(content)
	if len(calls) != 1 || calls[0].Name != "write" {
		t.Fatalf("解析失败: %+v", calls)
	}
	if calls[0].Args["path"] != "a.go" {
		t.Fatalf("args: %+v", calls[0].Args)
	}
	if StripXMLToolCalls(content) != "" {
		t.Fatal("剥离后应只剩空白")
	}
}

func TestParseMultiObject(t *testing.T) {
	content := "<tool_call>{\"name\": \"a1\", \"arguments\": {}}, {\"name\": \"b2\", \"arguments\": {\"x\": 1}}</tool_call>"
	calls := ParseXMLToolCalls(content)
	if len(calls) != 2 {
		t.Fatalf("多对象应解析 2 个: %d", len(calls))
	}
	if calls[0].Name != "a1" || calls[1].Name != "b2" {
		t.Fatalf("顺序: %s %s", calls[0].Name, calls[1].Name)
	}
}

func TestParseLegacyXML(t *testing.T) {
	content := "<tool_call><function=write><parameter=path>x.go</parameter><parameter=content>hi</parameter></tool_call>"
	calls := ParseXMLToolCalls(content)
	if len(calls) != 1 || calls[0].Name != "write" {
		t.Fatalf("旧 XML 解析: %+v", calls)
	}
	if calls[0].Args["path"] != "x.go" {
		t.Fatalf("旧参数: %+v", calls[0].Args)
	}
}

func TestBuildPromptContainsSchema(t *testing.T) {
	p := BuildToolPrompt([]ToolSchema{{Name: "bash", Description: "执行命令", Parameters: map[string]any{"command": map[string]any{"type": "string"}}, Required: []string{"command"}}}, "")
	if !strings.Contains(p, "<tools>") || !strings.Contains(p, "bash") || !strings.Contains(p, "<tool_call>") {
		t.Fatal("提示缺关键节")
	}
}

func TestWrapResponse(t *testing.T) {
	ok := WrapToolResponse("ls", "a\nb")
	if !strings.Contains(ok, "[成功·3字]") || !strings.Contains(ok, "<tool_response>") {
		t.Fatalf("成功包装: %s", ok)
	}
	bad := WrapToolResponseErr("ls", "err")
	if !strings.Contains(bad, "[失败]") {
		t.Fatal("失败包装")
	}
}
