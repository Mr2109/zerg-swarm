package adapters

import (
	"testing"

	"zerg/core/internal/plugin"
)

// TestNemotron_Interface — 接口断言（防签名回归）
func TestNemotron_Interface(t *testing.T) {
	var _ plugin.Plugin = NewNemotronAdapter()
}

// TestNemotron_DefaultConfig — 默认配置（上下文最大——Mr2109指示）
func TestNemotron_DefaultConfig(t *testing.T) {
	a := NewNemotronAdapter()
	if a.Name() != "Nemotron-3.5-Lightning" {
		t.Errorf("名称应为 Nemotron-3.5-Lightning——实际 %s", a.Name())
	}
	if a.config.CtxWindow != 1048576 {
		t.Errorf("上下文应为 1M(1048576)——实际 %d", a.config.CtxWindow)
	}
	if a.config.MaxTokens != 32768 {
		t.Errorf("max_tokens 默认应 32768——实际 %d", a.config.MaxTokens)
	}
	if a.config.Temperature != 0.7 {
		t.Errorf("温度默认应 0.7——实际 %v", a.config.Temperature)
	}
}

// TestNemotron_Init — 配置覆盖
func TestNemotron_Init(t *testing.T) {
	a := NewNemotronAdapter()
	err := a.Init(map[string]any{"temperature": 0.3, "max_tokens": 10000, "ctx_window": 262144})
	if err != nil {
		t.Fatal(err)
	}
	if a.config.Temperature != 0.3 {
		t.Errorf("温度应为 0.3——实际 %v", a.config.Temperature)
	}
	if a.config.MaxTokens != 10000 {
		t.Errorf("max_tokens 应为 10000——实际 %d", a.config.MaxTokens)
	}
	if a.config.CtxWindow != 262144 {
		t.Errorf("ctx_window 应为 262144——实际 %d", a.config.CtxWindow)
	}
}

// TestNemotron_Execute — Execute 返回模型元数据
func TestNemotron_Execute(t *testing.T) {
	a := NewNemotronAdapter()
	out, err := a.Execute(plugin.PluginInput{Data: map[string]any{"msg": "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	res := out.Result.(map[string]any)
	if res["model"] != "Nemotron-3.5-Lightning" {
		t.Errorf("model 应正确——实际 %v", res["model"])
	}
	if res["reasoning_fallback"] != true {
		t.Error("应标记 reasoning_fallback（content空时兜底）")
	}
}
