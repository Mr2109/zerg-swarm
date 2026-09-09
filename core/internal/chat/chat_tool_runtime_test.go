package chat

// chat_tool_runtime_test.go — 渐进式常驻核心机制测试（设计-渐进式常驻工具系统-20260902）
// 覆盖: 成功即常驻 / LRU 上限 10 / 3 次 exec 失败隐藏 / param 错不隐藏 / 拦截不常驻 / 成功清零

import (
	"testing"

	"zerg/core/internal/agent"
)

// 1. 成功即常驻（初始无常驻——成功调用后加入）
func TestProgressiveAddResident(t *testing.T) {
	rt := NewToolRuntime()
	if len(rt.Resident()) != 0 {
		t.Fatalf("初始应无常驻——got %v", rt.Resident())
	}
	rt.RecordOutcome("port_services", "", "")
	if len(rt.Resident()) != 1 || rt.Resident()[0] != "port_services" {
		t.Fatalf("成功后应常驻——got %v", rt.Resident())
	}
}

// 2. 已常驻工具再用——刷新到最新位（不重复）
func TestProgressiveRefreshLRU(t *testing.T) {
	rt := NewToolRuntime()
	rt.RecordOutcome("a", "", "")
	rt.RecordOutcome("b", "", "")
	rt.RecordOutcome("a", "", "") // a 再用——应移到尾
	r := rt.Resident()
	if len(r) != 2 || r[0] != "b" || r[1] != "a" {
		t.Fatalf("LRU 刷新错——got %v", r)
	}
}

// 3. LRU 上限 10——新常驻挤掉最老
func TestProgressiveLRUCap(t *testing.T) {
	rt := NewToolRuntime()
	for i := 0; i < 12; i++ {
		rt.RecordOutcome(string(rune('a'+i)), "", "") // a..l 12 个
	}
	r := rt.Resident()
	if len(r) != MaxResident {
		t.Fatalf("应上限 %d——got %d", MaxResident, len(r))
	}
	// 最老两个（a b）应被挤掉——最新 c..l
	if r[0] != "c" || r[len(r)-1] != "l" {
		t.Fatalf("LRU 淘汰错——got %v", r)
	}
}

// 4. exec 失败计数——3 次隐藏（前 2 次仍在常驻）
func TestProgressiveFailHidden(t *testing.T) {
	rt := NewToolRuntime()
	rt.RecordOutcome("bad", "", "") // 先成功常驻
	if rt.IsHidden("bad") {
		t.Fatal("不应隐藏")
	}
	rt.RecordOutcome("bad", "exec", "服务 502")
	if rt.IsHidden("bad") {
		t.Fatal("1 次失败不应隐藏")
	}
	rt.RecordOutcome("bad", "exec", "服务 502")
	if rt.IsHidden("bad") {
		t.Fatal("2 次失败不应隐藏")
	}
	msg := rt.RecordOutcome("bad", "exec", "服务 502")
	if !rt.IsHidden("bad") {
		t.Fatal("3 次 exec 失败应隐藏")
	}
	if len(rt.Resident()) != 0 {
		t.Fatalf("隐藏后应移出常驻——got %v", rt.Resident())
	}
	if msg == "" {
		t.Fatal("第 3 次失败应返回隐藏提示")
	}
}

// 5. param 错误不隐藏（调用方锅——工具无错）
func TestProgressiveParamNoHide(t *testing.T) {
	rt := NewToolRuntime()
	for i := 0; i < 5; i++ {
		rt.RecordOutcome("good", "param", "参数 query 不能为空")
	}
	if rt.IsHidden("good") {
		t.Fatal("param 错 5 次也不应隐藏（调用方锅）")
	}
}

// 6. 成功清零——exec 失败后成功恢复（连续语义）
func TestProgressiveSuccessReset(t *testing.T) {
	rt := NewToolRuntime()
	rt.RecordOutcome("t", "exec", "err1")
	rt.RecordOutcome("t", "exec", "err2")
	rt.RecordOutcome("t", "", "") // 成功——清零
	rt.RecordOutcome("t", "exec", "err3")
	if rt.IsHidden("t") {
		t.Fatal("成功清零后 1 次失败不应隐藏")
	}
}

// 7. 隐藏列表（tool_search 过滤用）
func TestProgressiveHiddenList(t *testing.T) {
	rt := NewToolRuntime()
	rt.RecordOutcome("x", "exec", "e")
	rt.RecordOutcome("x", "exec", "e")
	rt.RecordOutcome("x", "exec", "e")
	hl := rt.HiddenList()
	if len(hl) != 1 || hl[0] != "x" {
		t.Fatalf("HiddenList 错——got %v", hl)
	}
}

// 8. ErrTypeOf 分类（参数类→param——执行类→exec）
func TestErrTypeOf(t *testing.T) {
	cases := map[string]string{
		"参数 port 不能为空":             "param",
		"命令参数为空":                   "param",
		"格式错误: 期望 JSON":            "param",
		"⚠️ 工具调用格式错误(系统断言——非执行失败——请勿原样重发)": "param",
		"bash: ⚠️ 工具调用格式错误(系统断言——非执行失败——请勿原样重发)": "param",
		"未知工具: xxx（先 tool_search）": "param",
		"服务 502: upstream fail":    "exec",
		"连接超时":                     "exec",
	}
	for msg, want := range cases {
		if got := ErrTypeOf(msg); got != want {
			t.Errorf("ErrTypeOf(%q) = %q——want %q", msg, got, want)
		}
	}
}

// 9. NormalizeToolArgs——bash arguments 双层嵌套解包（模型 Hermes 风格污染）
func TestNormalizeToolArgs(t *testing.T) {
	// 正常对象——不动
	tc1 := &agent.ToolCall{Name: "bash", Args: map[string]any{"command": "ls -la"}}
	NormalizeToolArgs(tc1)
	if tc1.Args["command"] != "ls -la" {
		t.Errorf("正常对象被破坏: %v", tc1.Args)
	}
	// 嵌套字符串（模型把 {name,arguments} 整个塞 arguments——B 场景真凶）
	tc2 := &agent.ToolCall{Name: "bash", Args: map[string]any{
		"arguments": "{\"command\":\"ls -la\"}", "name": "bash",
	}}
	NormalizeToolArgs(tc2)
	if tc2.Args["command"] != "ls -la" || len(tc2.Args) != 1 {
		t.Errorf("嵌套字符串解包失败: %v", tc2.Args)
	}
	// 嵌套对象 + 元键
	tc3 := &agent.ToolCall{Name: "bash", Args: map[string]any{
		"arguments": map[string]any{"command": "ps"}, "type": "function",
	}}
	NormalizeToolArgs(tc3)
	if tc3.Args["command"] != "ps" || len(tc3.Args) != 1 {
		t.Errorf("嵌套对象解包失败: %v", tc3.Args)
	}
}
