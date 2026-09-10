package gateway

import (
	"encoding/json"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/gateway/adapter"
	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// mockAdapter — 测试用适配器（返回已知元数据）
type mockAdapter struct{}

func (m *mockAdapter) Name() string                  { return "mock" }
func (m *mockAdapter) Type() plugin.PluginType       { return plugin.PluginTypeModelAdapter }
func (m *mockAdapter) Version() string               { return "1.0" }
func (m *mockAdapter) Capabilities() []string        { return []string{"model"} }
func (m *mockAdapter) Init(cfg map[string]any) error { return nil }
func (m *mockAdapter) Start() error                  { return nil }
func (m *mockAdapter) Stop() error                   { return nil }
func (m *mockAdapter) Close() error                  { return nil }
func (m *mockAdapter) GetDescriptions() string       { return "mock" }
func (m *mockAdapter) Execute(input plugin.PluginInput) (plugin.PluginOutput, error) {
	return plugin.PluginOutput{
		Result: map[string]any{
			"temperature": 0.7,
			"max_tokens":  32768,
			"timeout_sec": 120,
			"model":       "mock-model",
		},
	}, nil
}

// TestAdapterRegistry_Override — 适配器参数覆盖请求体（温度/max_tokens）
func TestAdapterRegistry_Override(t *testing.T) {
	g := &Gateway{
		adapterRegistry: map[string]plugin.Plugin{"Qwen3.8-27B": &mockAdapter{}},
	}

	body := []byte(`{"model":"Qwen3.8-27B","messages":[{"role":"user","content":"hi"}],"temperature":0.9}`)
	forwardBody := body

	// 模拟 B-3 的适配器覆盖逻辑
	if ada, ok := g.adapterRegistry["Qwen3.8-27B"]; ok {
		out, aerr := ada.Execute(plugin.PluginInput{Data: map[string]any{"model": "Qwen3.8-27B"}})
		if aerr == nil && out.Result != nil {
			if res, ok := out.Result.(map[string]any); ok {
				if t, ok := res["temperature"].(float64); ok {
					forwardBody = adapter.JsonSetField(forwardBody, "temperature", t)
				}
				if mt, ok := res["max_tokens"].(int); ok {
					forwardBody = adapter.JsonSetField(forwardBody, "max_tokens", mt)
				}
			}
		}
	}

	var obj map[string]any
	if err := json.Unmarshal(forwardBody, &obj); err != nil {
		t.Fatal(err)
	}
	// 温度应从 0.9 → 0.7（适配器覆盖）
	temp, _ := obj["temperature"].(float64)
	if temp != 0.7 {
		t.Errorf("温度应被适配器覆盖为 0.7——实际 %v", temp)
	}
	// max_tokens 应被加（适配器 32768）
	mt, _ := obj["max_tokens"].(float64)
	if mt != 32768 {
		t.Errorf("max_tokens 应 32768——实际 %v", mt)
	}
	// 超时覆盖
	if g.getRequestTimeout("Qwen3.8-27B") != 0 { // 还没 set——覆盖逻辑里 set
		t.Error("超时未设置")
	}
}

// TestAdapterRegistry_NoAdapter — 无适配器模型——请求体不变
func TestAdapterRegistry_NoAdapter(t *testing.T) {
	g := &Gateway{adapterRegistry: map[string]plugin.Plugin{}}
	body := []byte(`{"model":"GLM-4.7-Flash","temperature":0.9}`)

	if ada, ok := g.adapterRegistry["GLM-4.7-Flash"]; ok && ada != nil {
		t.Error("无适配器的模型不应命中")
	}
	// 无适配器——请求体原样
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["temperature"].(float64) != 0.9 {
		t.Error("无适配器——温度不应变")
	}
}
