package adapter

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// 取出 SSE 里第一个 data: 行的 JSON 体（去掉前缀 "data: " 与空行）
func firstSSEData(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	for _, ln := range strings.Split(raw, "\n") {
		ln = strings.TrimRight(ln, "\r")
		if !strings.HasPrefix(ln, "data: ") {
			continue
		}
		p := strings.TrimPrefix(ln, "data: ")
		if p == "" || p == "[DONE]" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(p), &m); err != nil {
			t.Fatalf("SSE data 帧不是合法 JSON: %v\n  原样: %q", err, p)
		}
		return m
	}
	t.Fatalf("没找到 data: 帧\n  原样: %q", raw)
	return nil
}

func delta0(t *testing.T, m map[string]interface{}) map[string]interface{} {
	t.Helper()
	choices, _ := m["choices"].([]interface{})
	if len(choices) == 0 {
		t.Fatalf("帧里没有 choices: %#v", m)
	}
	d, _ := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	if d == nil {
		t.Fatalf("帧里没有 delta: %#v", m)
	}
	return d
}

// 回归（2026-09-19 实测）：流式下 tool_call 的 arguments 曾被**双重转义** ✗
//
//	escapeJSON(s) 先转一层，Sprintf 的 %q 再转一层 ⇒ 客户端收到 {\"command\":\"date\"}
//	⇒ json.loads 失败（Hermes 日志逐字：Unrepairable tool_call arguments）⇒ 调用被丢弃。
//
// 本用例断言：帧反序列化后，arguments 必须能**原样**解析回参数对象。
func TestWriteChatToolCalls_ArgumentsNotDoubleEscaped(t *testing.T) {
	want := `{"command":"date -u","nested":{"path":"/tmp/a b.json"}}`
	tcs := []interface{}{
		map[string]interface{}{
			"id":   "call_1",
			"type": "function",
			"function": map[string]interface{}{
				"name":      "terminal",
				"arguments": want,
			},
		},
	}
	rec := httptest.NewRecorder()
	writeChatToolCalls(rec, nil, tcs, "tool_calls")

	d := delta0(t, firstSSEData(t, rec.Body.String()))
	tcArr, _ := d["tool_calls"].([]interface{})
	if len(tcArr) == 0 {
		t.Fatalf("delta 里没有 tool_calls: %#v", d)
	}
	got, _ := tcArr[0].(map[string]interface{})["function"].(map[string]interface{})["arguments"].(string)

	if got != want {
		t.Fatalf("arguments 被改写了（疑似双重转义）\n  want: %s\n  got : %s", want, got)
	}
	var back map[string]interface{}
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Fatalf("arguments 不是可解析的 JSON（客户端会判 Unrepairable）: %v\n  原样: %s", err, got)
	}
	if back["command"] != "date -u" {
		t.Fatalf("参数内容不对: %#v", back)
	}
}

// 同类：正文/思考帧里的文本也不得被叠加转义（换行/引号是重灾区）
func TestWriteChatDelta_TextNotDoubleEscaped(t *testing.T) {
	want := "第一行\n第二行 \"带引号\" 与 \\ 反斜杠"
	rec := httptest.NewRecorder()
	writeChatDelta(rec, nil, want, "")

	d := delta0(t, firstSSEData(t, rec.Body.String()))
	got, _ := d["content"].(string)
	if got != want {
		t.Fatalf("content 被改写了（疑似双重转义）\n  want: %q\n  got : %q", want, got)
	}
	if !strings.Contains(got, "\n") {
		t.Fatalf("换行没还原成真换行（说明 \\n 被当两字符透传）: %q", got)
	}
}

func TestStrOrEmpty_PassesRawValue(t *testing.T) {
	in := `{"a":"b\n\"c\""}`
	if got := strOrEmpty(in); got != in {
		t.Fatalf("strOrEmpty 应原样返回（转义交给 %%q）：\n  want: %q\n  got : %q", in, got)
	}
	if got := strOrEmpty(42); got != "" {
		t.Fatalf("非字符串应返回空串，got %q", got)
	}
}
