package chat

import (
	dbsql "database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestChatCompact — 压缩布局与阈值（C4a）
func TestChatCompact(t *testing.T) {
	// 构造 40 条长消息（每条 ~500 token——超 8000 阈值）
	msgs := make([]Message, 40)
	longContent := strings.Repeat("这是一段比较长的对话内容用于触发上下文压缩阈值测试。", 20)
	for i := range msgs {
		msgs[i] = Message{ID: int64(i + 1), Role: "user", Content: longContent, Active: true}
	}
	if !ShouldCompact(msgs, "example-35b-v2") {
		t.Fatal("40 条消息应触发压缩")
	}
	sumSrc, ids := CompactMessages(msgs)
	if len(sumSrc) != 40-CompactProtectFirstN-CompactProtectLastN {
		t.Fatalf("中间段数量错误: %d（期望 %d）", len(sumSrc), 40-CompactProtectFirstN-CompactProtectLastN)
	}
	if len(ids) != len(sumSrc) {
		t.Fatal("ids 与摘要源数量不一致")
	}
	// 小会话不压缩
	small := make([]Message, 10)
	for i := range small {
		small[i] = Message{ID: int64(i + 1), Content: "x"}
	}
	if ShouldCompact(small, "example-35b-v2") {
		t.Fatal("10 条消息不应触发压缩")
	}
}

// TestCompactTriggerTokens — P4-39 三档阈值（262K/32K/8K）
func TestCompactTriggerTokens(t *testing.T) {
	cases := []struct {
		model string
		want  int
	}{
		{"example-35b-v2", 8000}, // min(131K, 8000) = 8000 活跃窗口优先
		{"Qwen3.8-27B", 8000},    // min(16K, 8000) = 8000
		{"gemma-12B", 4096},      // min(4096, 8000) = 4096——小窗口提前压
		{"unknown-model", 8000},  // 未注册默认 262K → 8000
	}
	for _, c := range cases {
		got := CompactTriggerTokens(c.model)
		if got != c.want {
			t.Errorf("%s: 阈值 %d（期望 %d）", c.model, got, c.want)
		}
	}
}

// TestChatStore — C1 验收：建会话→发消息→读消息→搜索→清理
func TestChatStore(t *testing.T) {
	// 用临时目录（不污染 /tmp/zerg-chat）
	dir := filepath.Join(os.TempDir(), "zerg-chat-test")
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0o755)
	os.Setenv("ZERG_CHAT_DIR", dir)
	defer os.RemoveAll(dir)

	// 直接测 store（OpenChatStore 用固定路径——这里手动构造）
	db, err := openTestStore(dir)
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer db.Close()

	// 1. 建会话
	se, err := db.CreateSession("example-35b-v2", "desktop", "", "")
	if err != nil {
		t.Fatalf("建会话失败: %v", err)
	}
	if se.ID == "" {
		t.Fatal("会话 id 为空")
	}

	// 2. 发消息（user + assistant 含 reasoning）
	uid, err := db.AddMessage(&Message{
		SessionID: se.ID, Role: "user", Content: "你好", Active: true, Timestamp: now(),
	})
	if err != nil {
		t.Fatalf("写用户消息失败: %v", err)
	}
	if uid == 0 {
		t.Fatal("消息 id 为 0")
	}
	_, err = db.AddMessage(&Message{
		SessionID: se.ID, Role: "assistant", Content: "你好！", Reasoning: "用户打招呼", Active: true, Timestamp: now(),
	})
	if err != nil {
		t.Fatalf("写助手消息失败: %v", err)
	}

	// 3. 读消息
	msgs, err := db.ListMessages(se.ID)
	if err != nil {
		t.Fatalf("读消息失败: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("消息数应为 2，实际 %d", len(msgs))
	}
	if msgs[1].Reasoning != "用户打招呼" {
		t.Fatalf("reasoning 分离存储失败: %q", msgs[1].Reasoning)
	}

	// 4. 会话活动更新（token 统计）
	if err := db.TouchSession(se.ID, 100, 50, 30); err != nil {
		t.Fatalf("活动更新失败: %v", err)
	}
	se2, _ := db.GetSession(se.ID)
	if se2.InputTokens != 100 || se2.OutputTokens != 50 || se2.ReasoningTokens != 30 {
		t.Fatalf("token 统计失败: %+v", se2)
	}

	// 5. 搜索（FTS5）
	res, err := db.SearchMessages("你好", 10)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("搜索无结果")
	}

	// 6. 会话列表
	list, err := db.ListSessions(10)
	if err != nil || len(list) != 1 {
		t.Fatalf("会话列表失败: %v (n=%d)", err, len(list))
	}

	// 7. 90 天清理（构造过期会话——先写消息再改旧时间，否则 TouchSession 会刷新活动）
	oldID := "chat_old"
	_, err = db.db.Exec("INSERT INTO sessions(id, created_at, last_activity_at) VALUES (?, 1, 1)", oldID)
	if err != nil {
		t.Fatalf("插旧会话失败: %v", err)
	}
	_, err = db.AddMessage(&Message{SessionID: oldID, Role: "user", Content: "旧", Active: true, Timestamp: 1})
	if err != nil {
		t.Fatalf("插旧消息失败: %v", err)
	}
	// 改回旧活动时间（模拟 90 天前）
	_, err = db.db.Exec("UPDATE sessions SET last_activity_at = 1 WHERE id = ?", oldID)
	if err != nil {
		t.Fatalf("改旧时间失败: %v", err)
	}
	n, err := db.DeleteOldSessions(float64(time.Now().Unix()) - 100)
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("应清 1 个过期会话，实际 %d", n)
	}
	// 旧会话消息也删了
	oldMsgs, _ := db.ListMessages(oldID)
	if len(oldMsgs) != 0 {
		t.Fatal("旧会话消息未删干净")
	}

	// 8. 硬删会话
	if err := db.DeleteSession(se.ID); err != nil {
		t.Fatalf("删会话失败: %v", err)
	}
	if _, err := db.GetSession(se.ID); err == nil {
		t.Fatal("删除后会话仍存在")
	}

	t.Log("C1 全部验收通过 ✅")
}

// openTestStore — 用指定目录打开测试 store（绕过固定路径）
func openTestStore(dir string) (*ChatStore, error) {
	path := filepath.Join(dir, "chat.db")
	db, err := openRaw(path)
	if err != nil {
		return nil, err
	}
	s := &ChatStore{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// openRaw — 打开 sqlite（WAL 模式）
func openRaw(path string) (*dbsql.DB, error) {
	db, err := dbsql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// TestParseToolArgs — P4-34 双重转义 arguments 解析
func TestParseToolArgs(t *testing.T) {
	// 正常单层 JSON
	got := parseToolArgs(`{"command":"ls -la"}`)
	if got["command"] != "ls -la" {
		t.Errorf("单层解析失败: %v", got)
	}
	// 双重转义（llama-server 流式——arguments 带字面反斜杠 \"——实测真实格式）
	got2 := parseToolArgs(`{\"command\":\"ls -la\"}`)
	if got2["command"] != "ls -la" {
		t.Errorf("双重转义解析失败: %v", got2)
	}
	// 空参数
	got3 := parseToolArgs("")
	if len(got3) != 0 {
		t.Errorf("空参数应返回空 map: %v", got3)
	}
}

// TestParseXMLToolCalls — P4-45 example-35b-v2 XML 工具调用解析（训练格式——适配器原则）
func TestParseXMLToolCalls(t *testing.T) {
	// 单工具
	xml1 := `<tool_call>
<function=bash>
<parameter=command>
ls -la /tmp
</parameter>
</function>
</tool_call>`
	calls := parseXMLToolCalls(xml1)
	if len(calls) != 1 {
		t.Fatalf("单工具解析失败: %d（期望 1）", len(calls))
	}
	if calls[0].Name != "bash" {
		t.Errorf("工具名错误: %s", calls[0].Name)
	}
	cmd, _ := calls[0].Args["command"].(string)
	if !strings.Contains(cmd, "ls -la") {
		t.Errorf("command 提取错误: %q", cmd)
	}
	// 多工具（工具调用前有思考文字）
	xml2 := `我先看一下目录。
<tool_call>
<function=ls>
<parameter=path>
core
</parameter>
</function>
</tool_call>
然后继续。`
	calls2 := parseXMLToolCalls(xml2)
	if len(calls2) != 1 || calls2[0].Name != "ls" {
		t.Fatalf("多工具/带文字解析失败: %d", len(calls2))
	}
	// strip 测试
	stripped := stripXMLToolCalls(xml2)
	if strings.Contains(stripped, "<tool_call>") {
		t.Errorf("strip 未清除 XML: %q", stripped)
	}
	if !strings.Contains(stripped, "我先看一下目录") {
		t.Errorf("strip 丢失正文: %q", stripped)
	}
}
