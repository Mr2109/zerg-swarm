package adapters

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// TestQwable_Interface — 接口断言
func TestQwable_Interface(t *testing.T) {
	var _ plugin.Plugin = NewQwableAdapter()
}

// TestQwable_DefaultConfig — 默认配置（双角色）
func TestQwable_DefaultConfig(t *testing.T) {
	a := NewQwableAdapter()
	if a.config.Temperature != 0.35 {
		t.Errorf("编码温度应 0.35——实际 %v", a.config.Temperature)
	}
	if a.config.MoATemp != 0.7 {
		t.Errorf("MoA 温度应 0.7——实际 %v", a.config.MoATemp)
	}
	if a.config.Variant != "q5_k_m" {
		t.Errorf("variant 应 q5_k_m——实际 %s", a.config.Variant)
	}
}

// TestQwable_Execute_Role — 角色温度切换（编码 vs MoA）
func TestQwable_Execute_Role(t *testing.T) {
	a := NewQwableAdapter()
	// 编码（默认）
	out, _ := a.Execute(plugin.PluginInput{Data: map[string]any{}})
	res := out.Result.(map[string]any)
	if res["temperature"] != 0.35 {
		t.Errorf("编码温度应 0.35——实际 %v", res["temperature"])
	}
	if res["tool_calling"] != true {
		t.Error("编码应支持工具")
	}
	// MoA
	out2, _ := a.Execute(plugin.PluginInput{Data: map[string]any{"role": "moa"}})
	res2 := out2.Result.(map[string]any)
	if res2["temperature"] != 0.7 {
		t.Errorf("MoA 温度应 0.7——实际 %v", res2["temperature"])
	}
	if res2["max_tokens"] != 2000 {
		t.Errorf("MoA max_tokens 应 2000——实际 %v", res2["max_tokens"])
	}
	if res2["tool_calling"] != false {
		t.Error("MoA 应无工具")
	}
}

// TestGemma_Interface — 接口断言
func TestGemma_Interface(t *testing.T) {
	var _ plugin.Plugin = NewGemmaAdapter()
}

// TestGemma_DefaultConfig — 默认配置（双角色）
func TestGemma_DefaultConfig(t *testing.T) {
	a := NewGemmaAdapter()
	if a.config.Temperature != 0.7 {
		t.Errorf("MoA 温度应 0.7——实际 %v", a.config.Temperature)
	}
	if a.config.JudgeTemp != 0.2 {
		t.Errorf("Judge 温度应 0.2——实际 %v", a.config.JudgeTemp)
	}
	if a.config.Size != "26B" {
		t.Errorf("size 应 26B——实际 %s", a.config.Size)
	}
}

// TestGemma_Execute_Role — 角色温度切换（MoA vs Judge）
func TestGemma_Execute_Role(t *testing.T) {
	a := NewGemmaAdapter()
	out, _ := a.Execute(plugin.PluginInput{Data: map[string]any{"role": "judge"}})
	res := out.Result.(map[string]any)
	if res["temperature"] != 0.2 {
		t.Errorf("Judge 温度应 0.2——实际 %v", res["temperature"])
	}
	if res["judge_capable"] != true {
		t.Error("应标记 judge_capable")
	}
	if res["tool_calling"] != false {
		t.Error("应无工具")
	}
}
