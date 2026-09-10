// chat_include_archived_test.go — 丙批 C2 补（2026-09-10 Mr2109拍板）：归档会话可搜
// 覆盖: 默认排除归档 / include_archived=true 纳入 / browse 两种范围 / ssBoolArg 容错
package chat

import (
	"strings"
	"testing"
)

// TestSessionSearchIncludeArchived — 归档≠搜不到
func TestSessionSearchIncludeArchived(t *testing.T) {
	st := newTestStore(t)
	se, err := st.CreateSession("example-35b-v2", "desktop", "", "归档检索验证会话")
	if err != nil {
		t.Fatal(err)
	}
	ssAddMsgs(t, st, se.ID, 1757600000.0,
		"紫色海豚的代号是 ZQ-7742，请记牢",
		"已记录。",
	)
	if err := st.ArchiveSession(se.ID); err != nil {
		t.Fatalf("归档失败: %v", err)
	}

	// ① 默认：归档会话搜不到
	base, err := SessionSearchExecute(map[string]any{"query": "紫色海豚"}, st)
	if err != nil {
		t.Fatalf("默认 search 失败: %v", err)
	}
	if strings.Contains(base, "ZQ-7742") {
		t.Fatalf("默认不应搜到归档会话内容:\n%s", base)
	}
	if !strings.Contains(base, "无命中") || !strings.Contains(base, "include_archived=true") {
		t.Fatalf("无命中时应提示 include_archived 逃生门:\n%s", base)
	}

	// ② include_archived=true：搜到了
	withArch, err := SessionSearchExecute(map[string]any{"query": "紫色海豚", "include_archived": true}, st)
	if err != nil {
		t.Fatalf("含归档 search 失败: %v", err)
	}
	if !strings.Contains(withArch, "ZQ-7742") {
		t.Fatalf("include_archived=true 应搜到归档会话内容:\n%s", withArch)
	}
	if !strings.Contains(withArch, "含已归档") {
		t.Fatalf("结果应标注「含已归档」范围:\n%s", withArch)
	}

	// ③ 字符串形态 true 也认（模型常把布尔写成字符串）
	asStr, err := SessionSearchExecute(map[string]any{"query": "紫色海豚", "include_archived": "true"}, st)
	if err != nil {
		t.Fatalf("字符串布尔 search 失败: %v", err)
	}
	if !strings.Contains(asStr, "ZQ-7742") {
		t.Fatalf("include_archived=\"true\" 应等同布尔 true:\n%s", asStr)
	}

	// ④ browse：默认不含归档；显式纳入则含
	st2 := newTestStore(t)
	sActive, _ := st2.CreateSession("example-35b-v2", "desktop", "", "活跃会话")
	if _, err := st2.CreateSession("example-35b-v2", "desktop", "", "将被归档的会话"); err != nil {
		t.Fatal(err)
	}
	sArch, _ := st2.CreateSession("example-35b-v2", "desktop", "", "归档目标")
	if err := st2.ArchiveSession(sArch.ID); err != nil {
		t.Fatal(err)
	}
	_ = sActive
	b1, _ := SessionSearchExecute(map[string]any{}, st2)
	if strings.Contains(b1, "归档目标") {
		t.Fatalf("browse 默认应排除归档:\n%s", b1)
	}
	b2, _ := SessionSearchExecute(map[string]any{"include_archived": true}, st2)
	if !strings.Contains(b2, "归档目标") {
		t.Fatalf("browse include_archived=true 应含归档会话:\n%s", b2)
	}

	// ⑤ store 层直连（UI 端点复用 SearchMessages——默认行为不得变）
	hits, err := st.SearchMessages("ZQ-7742", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("SearchMessages 默认应 0 命中，实得 %d", len(hits))
	}
	hits2, err := st.SearchMessagesArchived("ZQ-7742", 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits2) == 0 {
		t.Fatal("SearchMessagesArchived(true) 应命中归档消息")
	}
}

// TestSsBoolArg — 布尔参数容错（bool/字符串/数字）
func TestSsBoolArg(t *testing.T) {
	cases := []struct {
		in   any
		want bool
	}{
		{true, true}, {false, false}, {"true", true}, {"TRUE", true}, {"1", true},
		{"yes", true}, {"是", true}, {"0", false}, {"false", false}, {float64(1), true}, {float64(0), false}, {nil, false},
	}
	for _, c := range cases {
		if got := ssBoolArg(map[string]any{"k": c.in}, "k"); got != c.want {
			t.Fatalf("ssBoolArg(%#v) = %v，期望 %v", c.in, got, c.want)
		}
	}
	if ssBoolArg(nil, "k") {
		t.Fatal("nil args 应为 false")
	}
}
