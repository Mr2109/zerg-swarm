package main

import (
	"bytes"
	"strings"
	"testing"
)

// chatGap17SSE — 造一条最小 SSE 事件流（`delta{output}` ×2 ＋ 可选 `done`）。
func chatGap17SSE(doneData string) string {
	s := "event: delta\ndata: {\"type\":\"output\",\"text\":\"Let me check\"}\n\n" +
		"event: delta\ndata: {\"type\":\"output\",\"text\":\" the file.\"}\n\n"
	if doneData != "" {
		s += "event: done\ndata: " + doneData + "\n\n"
	}
	return s
}

// TestChatSendPrefersDoneContent —— GAP-20261017-17 客户端一格：
// `done.payload.content` 存在且非空 ⇒ `turn` 的正文取它（＝服务端落库那一份），
// ✗ 不取逐帧 `delta` 拼出来的串（含 reasoning 且 `max_rounds` 时被截断）。
//
// 变异（必红）：把 `chatHandleEvent` 的 `done` 分支里那段 `FinalContent` 赋值删掉
// ⇒ 本用例第 1 条断言当场红（正文回落到半截拼串）。
func TestChatSendPrefersDoneContent(t *testing.T) {
	const final = "答案：先 `net.Listen`，再 `http.Server`。"
	sse := chatGap17SSE(`{"message_id":700,"title":"t","content":"` + final +
		`","input_tokens":1,"output_tokens":2,"reasoning_tokens":3}`)
	var out, errb bytes.Buffer
	turn, err := chatReadSSE(strings.NewReader(sse), false, false, &out, &errb)
	if err != nil {
		t.Fatalf("读事件流失败：%v", err)
	}
	if got := turn.reportContent(); got != final {
		t.Fatalf("`done.content` 未被优先采纳：\n 得=%q\n 期望=%q", got, final)
	}
	// 负控：拼串那一路确实是半截（本位必须拦得住「回落」那一档）
	if turn.Output.String() == final {
		t.Fatalf("负控失效：逐帧 delta 拼出来的串与 done.content 相同 ⇒ 本用例无牙")
	}
	if !strings.Contains(turn.Output.String(), "Let me") {
		t.Fatalf("夹具不成立：拼串里没有思维链残留：%q", turn.Output.String())
	}
	// 其余载荷键照旧
	if turn.MessageID != "700" || turn.InputTokens != "1" || turn.OutputTokens != "2" || turn.ReasonTokens != "3" {
		t.Fatalf("done 其余键被改动：%+v", turn)
	}
}

// TestChatSendFallsBackWhenDoneHasNoContent —— 向后兼容一格：
// 旧主控（`done` 不带 `content`）/ `interrupted` 档 ⇒ 正文回落逐帧 `delta` 拼串，读数不变。
func TestChatSendFallsBackWhenDoneHasNoContent(t *testing.T) {
	sse := chatGap17SSE(`{"message_id":701,"title":"t","input_tokens":1,"output_tokens":2,"reasoning_tokens":3}`)
	var out, errb bytes.Buffer
	turn, err := chatReadSSE(strings.NewReader(sse), false, false, &out, &errb)
	if err != nil {
		t.Fatalf("读事件流失败：%v", err)
	}
	if turn.FinalContent != "" {
		t.Fatalf("缺 `content` 时不该填 FinalContent：%q", turn.FinalContent)
	}
	if got := turn.reportContent(); got != "Let me check the file." {
		t.Fatalf("回落档读数变了：%q", got)
	}
}

// TestChatSendStreamingStillEmitsDeltas —— 回归一格：流式档仍**逐块**上 stdout（正文只由 delta 打），
// 且 `done.content` 只进机器面（✗ 不重复打印一次成稿）。
func TestChatSendStreamingStillEmitsDeltas(t *testing.T) {
	const final = "答案：先监听后服务。"
	sse := chatGap17SSE(`{"message_id":702,"title":"t","content":"` + final + `"}`)
	var out, errb bytes.Buffer
	turn, err := chatReadSSE(strings.NewReader(sse), true, false, &out, &errb)
	if err != nil {
		t.Fatalf("读事件流失败：%v", err)
	}
	if got := out.String(); got != "Let me check the file." {
		t.Fatalf("流式档 stdout 应与逐帧 delta 逐字相同（✗ 不重复打成稿）：%q", got)
	}
	if got := turn.reportContent(); got != final {
		t.Fatalf("流式档机器面正文应取 done.content：%q", got)
	}
}
