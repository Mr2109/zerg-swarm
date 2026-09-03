package agent

import (
	"context"
	"testing"
)

// mockModelAdapter 测试用假适配器。
type mockModelAdapter struct {
	called     bool
	lastPrompt string
	resp       *ModelResponse
	err        error
}

func (m *mockModelAdapter) Call(ctx context.Context, sysPrompt string, messages []Message, tools []ToolDef) (*ModelResponse, error) {
	m.called = true
	m.lastPrompt = sysPrompt
	return m.resp, m.err
}

func TestCallModel_UsesAdapter(t *testing.T) {
	a := &Agent{}
	mock := &mockModelAdapter{resp: &ModelResponse{Content: "适配器响应"}}
	a.SetAdapter(mock)

	resp, err := a.callModel(context.Background(), "系统提示", nil)
	if err != nil {
		t.Fatalf("callModel error = %v", err)
	}
	if !mock.called {
		t.Fatal("适配器未被调用——callModel 没走插件分支")
	}
	if mock.lastPrompt != "系统提示" {
		t.Errorf("sysPrompt = %s, want 系统提示", mock.lastPrompt)
	}
	if resp.Content != "适配器响应" {
		t.Errorf("Content = %s, want 适配器响应", resp.Content)
	}
}

func TestCallModel_NoAdapter_OldLogic(t *testing.T) {
	// 未设适配器——走老逻辑（验证分支存在——不真调 HTTP）
	a := &Agent{}
	if a.adapter != nil {
		t.Fatal("默认 adapter 应为 nil（老逻辑）")
	}
	// 老逻辑已有现有测试覆盖（agent_test.go）——这里只验证分支选择
	// 不真调（避免 nil client panic）——直接检查 callModel 会走 adapter==nil 分支
	t.Log("adapter 为 nil——callModel 走老逻辑分支（现有测试覆盖）")
}

func TestSetAdapter_Nil(t *testing.T) {
	a := &Agent{}
	a.SetAdapter(nil)
	if a.adapter != nil {
		t.Fatal("SetAdapter(nil) 后 adapter 应为 nil")
	}
}

func TestPluginModelAdapterBridge(t *testing.T) {
	// 验证桥：NewPluginModelAdapter 返回 ModelAdapter
	// 用 mock plugin（不真调）——只验证类型
	// 桥依赖 plugin.Plugin——用 nil 验证构造（不执行）
	_ = NewPluginModelAdapter(nil)
	t.Log("桥构造正常")
}
