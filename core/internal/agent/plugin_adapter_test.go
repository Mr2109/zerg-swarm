package agent

import (
	"context"
	"strings"
	"testing"

	"zerg/core/internal/plugin"
)

// mockPluginBase 提供 plugin.Plugin 接口的默认空实现（mock 复用——不全写）
type mockPluginBase struct{}

func (m *mockPluginBase) Init(cfg map[string]any) error             { return nil }
func (m *mockPluginBase) Start() error                              { return nil }
func (m *mockPluginBase) Stop() error                               { return nil }
func (m *mockPluginBase) Close() error                              { return nil }
func (m *mockPluginBase) Name() string                              { return "mock" }
func (m *mockPluginBase) Version() string                           { return "test" }
func (m *mockPluginBase) Type() plugin.PluginType                  { return plugin.PluginTypeModelAdapter }
func (m *mockPluginBase) Capabilities() []string                    { return []string{"model"} }
func (m *mockPluginBase) Shutdown() error                           { return nil }

// mockTruncPlugin — 返回截断 tool_calls 的假插件（测试截断容错）
type mockTruncPlugin struct{ mockPluginBase }

// Execute — 返回 tool_calls 但 arguments 是截断 JSON（缺右括号）
func (m *mockTruncPlugin) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	return plugin.PluginOutput{
		Result: map[string]any{
			"content": "",
			"tool_calls": []map[string]any{
				{"name": "bash", "arguments": `{"command":"echo hi`}, // 截断——缺 } 结尾
			},
		},
	}, nil
}

// TestPluginAdapter_TruncatedArgs — 截断的 args JSON → 报错（触发重试——不静默）
func TestPluginAdapter_TruncatedArgs(t *testing.T) {
	adapter := NewPluginModelAdapter(&mockTruncPlugin{})
	_, err := adapter.Call(context.Background(), "sys", nil, nil)
	if err == nil {
		t.Fatal("截断的 args JSON 应报错（触发重试）——不能静默成功")
	}
	if !strings.Contains(err.Error(), "JSON 解析失败") {
		t.Fatalf("错误应说明 JSON 解析失败——实际: %v", err)
	}
	t.Logf("✅ 截断容错: args JSON 截断→报错触发重试（%v）", err)
}

// mockGoodPlugin — 正常 tool_calls（对照——不误报）
type mockGoodPlugin struct{ mockPluginBase }

func (m *mockGoodPlugin) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	return plugin.PluginOutput{
		Result: map[string]any{
			"content": "hi",
			"tool_calls": []map[string]any{
				{"name": "bash", "arguments": `{"command":"echo hi"}`},
			},
		},
	}, nil
}

// TestPluginAdapter_GoodArgs — 正常 args → 不误报（对照）
func TestPluginAdapter_GoodArgs(t *testing.T) {
	adapter := NewPluginModelAdapter(&mockGoodPlugin{})
	resp, err := adapter.Call(context.Background(), "sys", nil, nil)
	if err != nil {
		t.Fatalf("正常 args 不应报错: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("应有 1 个工具调用——实际 %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "bash" {
		t.Fatalf("工具名应为 bash——实际 %s", resp.ToolCalls[0].Name)
	}
	t.Log("✅ 正常 args 解析成功（不误报）")
}
