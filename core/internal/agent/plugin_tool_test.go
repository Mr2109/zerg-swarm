package agent

import "testing"

// TestPluginToolReversible — P2 可逆副作用（2026-08-21 Mr2109）
// 插件注册工具 → 卸载撤销（无僵尸）
func TestPluginToolReversible(t *testing.T) {
	before := len(AllTools())
	// 插件注册
	RegisterPluginTool("test-plugin", ToolDef{Type: "function", Function: FunctionDef{Name: "plugin_test_tool"}})
	if len(AllTools()) != before+1 {
		t.Fatalf("注册后应多 1 个工具: got %d want %d", len(AllTools()), before+1)
	}
	// 卸载撤销
	n := UnregisterPluginTools("test-plugin")
	if n != 1 {
		t.Fatalf("应撤销 1 个: got %d", n)
	}
	if len(AllTools()) != before {
		t.Fatalf("撤销后应恢复: got %d want %d", len(AllTools()), before)
	}
}
