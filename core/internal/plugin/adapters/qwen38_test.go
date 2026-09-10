package adapters

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// TestQwen38_Interface — 接口断言
func TestQwen38_Interface(t *testing.T) {
	var _ plugin.Plugin = NewQwen38Adapter()
}

// TestQwen38_DefaultConfig — 默认配置（官方推荐——交叉调研）
func TestQwen38_DefaultConfig(t *testing.T) {
	a := NewQwen38Adapter()
	if a.Name() != "Qwen3.8-27B" {
		t.Errorf("名称错——实际 %s", a.Name())
	}
	if a.config.Temperature != 0.7 {
		t.Errorf("温度应 0.7（官方推荐）——实际 %v", a.config.Temperature)
	}
	if a.config.CtxWindow != 1048576 {
		t.Errorf("上下文应 1M——实际 %d", a.config.CtxWindow)
	}
	if a.config.TimeoutSec != 120 {
		t.Errorf("超时应 120s（思考模式长）——实际 %d", a.config.TimeoutSec)
	}
	if a.config.TopP != 0.8 || a.config.TopK != 20 {
		t.Errorf("采样参数应 0.8/20——实际 %v/%d", a.config.TopP, a.config.TopK)
	}
}

// TestQwen38_Init — 配置覆盖
func TestQwen38_Init(t *testing.T) {
	a := NewQwen38Adapter()
	err := a.Init(map[string]any{"temperature": 0.3, "reasoning_effort": "low", "timeout_sec": 180})
	if err != nil {
		t.Fatal(err)
	}
	if a.config.Temperature != 0.3 {
		t.Errorf("温度应 0.3——实际 %v", a.config.Temperature)
	}
	if a.config.ReasoningEffort != "low" {
		t.Errorf("reasoning_effort 应 low——实际 %s", a.config.ReasoningEffort)
	}
	if a.config.TimeoutSec != 180 {
		t.Errorf("超时应 180——实际 %d", a.config.TimeoutSec)
	}
}

// TestQwen38_Execute — Execute 返回模型元数据（含 reasoning 兜底/多模态）
func TestQwen38_Execute(t *testing.T) {
	a := NewQwen38Adapter()
	out, err := a.Execute(plugin.PluginInput{Data: map[string]any{"msg": "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	res := out.Result.(map[string]any)
	if res["model"] != "Qwen3.8-27B" {
		t.Errorf("model 错——实际 %v", res["model"])
	}
	if res["reasoning_fallback"] != true {
		t.Error("应标记 reasoning_fallback")
	}
	if res["multimodal"] != true {
		t.Error("应标记 multimodal")
	}
	if res["timeout_sec"] != 120 {
		t.Errorf("timeout 应 120——实际 %v", res["timeout_sec"])
	}
}
