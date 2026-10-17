package api

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/chat"
)

// TestChatDonePayloadCarriesStoredContent —— GAP-20261017-17 服务端一格：
// `done` 帧的 `content` 必须**与落库那一份逐字节同源**（同取 `msgContent`），
// 且不得是 `result.Content`（含 reasoning 的原始串 —— 客户端攒出来的半截正文就来自它）。
//
// 变异（必红）：把 `chatDonePayload` 里的 `"content": msgContent` 改成 `result.Content`
// ⇒ 本用例第 2/3 条断言当场红（`done.content` ≠ messages 表里那一份）。
func TestChatDonePayloadCarriesStoredContent(t *testing.T) {
	// 夹具＝现读形态：英文思维链开头 + 中文成稿；`stripReasoningFromContent` 会剥掉前段。
	raw := "Let me check the file first. I need to read core/go.mod now.\n" +
		"让我读一下这两个文件，再给你结论。\n\n" +
		"答案：module 名是 zerg-swarm，go 指令是 1.24。"
	result := &chat.InferResult{Content: raw, InputTokens: 11, OutputTokens: 22, ReasoningTokens: 33}
	msgContent, msgReasoning := stripReasoningFromContent(result)

	// 夹具自检：剥离必须真发生过，否则下面两条断言没有牙。
	if msgContent == raw {
		t.Fatalf("夹具不成立：`stripReasoningFromContent` 未剥离前段（原始串与落库串逐字相同）⇒ 本用例无牙")
	}
	if strings.Contains(msgContent, "Let me ") {
		t.Fatalf("夹具不成立：落库正文里仍含思维链：%q", msgContent)
	}

	dir := t.TempDir()
	t.Setenv("ZERG_CHAT_DB_PATH", filepath.Join(dir, "chat.db"))
	store, err := chat.OpenChatStore()
	if err != nil {
		t.Fatalf("打开对话库失败：%v", err)
	}
	defer store.Close()

	se, err := store.CreateSession("test-model", "test", "", "")
	if err != nil {
		t.Fatalf("建会话失败：%v", err)
	}
	// 落库＝处理器里那一段（`Content: msgContent`）
	astID, err := store.AddMessage(&chat.Message{
		SessionID: se.ID, Role: "assistant", Content: msgContent, Reasoning: msgReasoning,
		Model: "test-model", Active: true, Timestamp: chatNow(),
	})
	if err != nil {
		t.Fatalf("落库失败：%v", err)
	}

	payload := chatDonePayload(astID, se.Title, msgContent, msgReasoning, result)

	got, ok := payload["content"].(string)
	if !ok {
		t.Fatalf("`done` 载荷缺 `content` 字段（客户端只能回落到逐帧 delta 拼串）：%#v", payload)
	}
	if got != msgContent {
		t.Fatalf("`done.content` 与落库那一份不同源：\n done=%q\n 落库=%q", got, msgContent)
	}
	// 现读 messages 表（工具面真源）—— 与 `chat show` 读的是同一处
	msgs, err := store.ListMessages(se.ID)
	if err != nil {
		t.Fatalf("读回消息失败：%v", err)
	}
	if len(msgs) == 0 {
		t.Fatalf("读回消息为空（落库未生效）⇒ 不给结论")
	}
	stored := msgs[len(msgs)-1].Content
	if got != stored {
		t.Fatalf("`done.content` 与 messages 表现读不逐字节相等：\n done=%q\n 表内=%q", got, stored)
	}
	// 负控：若照 `result.Content` 填，两化就不等了 —— 证明本位能拦「取原始串」那一档。
	if result.Content == stored {
		t.Fatalf("负控失效：原始串与落库串相同 ⇒ 本用例拦不住「done 取 result.Content」那一档")
	}
	// ★ 同批补的第二格：`reasoning` 也必须是**落库同一份**（两格同源到底 —— 只补 content 会让
	// 「正文同源、思考仍取自逐帧拼串」，两个面各说一套）。取不到该键即为「只修了一半」。
	gr, ok := payload["reasoning"].(string)
	if !ok {
		t.Fatalf("`done` 载荷缺 `reasoning` 字段（只补了 content ⇒ 两格不同源）：%#v", payload)
	}
	if gr != msgReasoning {
		t.Fatalf("`done.reasoning` 与落库那一份不同源：\n done=%q\n 落库=%q", gr, msgReasoning)
	}
	if stored2 := msgs[len(msgs)-1].Reasoning; gr != stored2 {
		t.Fatalf("`done.reasoning` 与 messages 表现读不逐字节相等：\n done=%q\n 表内=%q", gr, stored2)
	}
	// 负控：若照 `result.Reasoning`（原始串）填，就不等于落库那一份 —— 证明本位能拦「取原始串」。
	if result.Reasoning == msgReasoning {
		t.Fatalf("负控失效：原始 reasoning 与落库串相同 ⇒ 本用例拦不住「done 取 result.Reasoning」那一档")
	}
	// token 三格照旧（本次只补 content 一格，别把载荷其余键挤掉）
	if payload["input_tokens"] != 11 || payload["output_tokens"] != 22 || payload["reasoning_tokens"] != 33 {
		t.Fatalf("载荷 token 三格被改动：%#v", payload)
	}
}
