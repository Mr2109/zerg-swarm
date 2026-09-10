package adapters

// 新适配器测试（2026-08-28 补全——GLM-4.7-Flash/Qwen3.6-35B-A3B/example-30b）
// 验证: 插件接口实现 / 默认配置 / Init 覆盖 / Execute 元数据 / OptionSchema

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// 编译期检查——三个新适配器都实现 plugin.Plugin 接口
var (
	_ plugin.Plugin = (*Glm47Adapter)(nil)
	_ plugin.Plugin = (*Qwen36Adapter)(nil)
	_ plugin.Plugin = (*MuseGlimmerAdapter)(nil)
)

func TestGlm47AdapterDefault(t *testing.T) {
	a := NewGlm47Adapter()
	if a.Name() != "GLM-4.7-Flash" {
		t.Fatalf("Name = %s", a.Name())
	}
	if a.config.MaxTokens != 32768 {
		t.Fatalf("MaxTokens = %d", a.config.MaxTokens)
	}
	if a.config.CtxWindow != 131072 {
		t.Fatalf("CtxWindow = %d", a.config.CtxWindow)
	}
	if !a.config.Thinking {
		t.Fatal("思考不能关——默认必须开")
	}
	if len(a.OptionSchema()) != 11 {
		t.Fatalf("OptionSchema 参数数 = %d", len(a.OptionSchema()))
	}
	// Init 覆盖（实时生效）
	if err := a.Init(map[string]any{"temperature": 0.9, "reasoning_effort": "high"}); err != nil {
		t.Fatal(err)
	}
	if a.config.Temperature != 0.9 || a.config.ReasoningEffort != "high" {
		t.Fatalf("Init 未生效: %+v", a.config)
	}
	// Execute 元数据
	out, err := a.Execute(plugin.PluginInput{Data: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	res := out.Result.(map[string]any)
	if res["model"] != "GLM-4.7-Flash" {
		t.Fatalf("model = %v", res["model"])
	}
	if res["reasoning_fallback"] != true {
		t.Fatal("思考模型必须 reasoning_fallback")
	}
}

func TestQwen36AdapterDefault(t *testing.T) {
	a := NewQwen36Adapter()
	if a.Name() != "Qwen3.6-35B-A3B" {
		t.Fatalf("Name = %s", a.Name())
	}
	if a.config.CtxWindow != 262144 {
		t.Fatalf("CtxWindow = %d", a.config.CtxWindow)
	}
	if len(a.OptionSchema()) != 10 {
		t.Fatalf("OptionSchema 参数数 = %d", len(a.OptionSchema()))
	}
	out, err := a.Execute(plugin.PluginInput{Data: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	res := out.Result.(map[string]any)
	if res["tool_support"] != true {
		t.Fatal("Qwen3.6 工具调用强——tool_support 必须 true")
	}
	if res["coding"] != true {
		t.Fatal("Qwen3.6 agentic 编码——coding 必须 true")
	}
}

func TestMuseGlimmerAdapterDefault(t *testing.T) {
	a := NewMuseGlimmerAdapter()
	if a.Name() != "example-30b" {
		t.Fatalf("Name = %s", a.Name())
	}
	if a.config.ReasoningEffort != "medium" {
		t.Fatalf("ReasoningEffort = %s", a.config.ReasoningEffort)
	}
	if !a.config.Multimodal {
		t.Fatal("Muse 原生多模态——Multimodal 必须 true")
	}
	if len(a.OptionSchema()) != 11 {
		t.Fatalf("OptionSchema 参数数 = %d", len(a.OptionSchema()))
	}
	// 四档推理强度
	opts := a.OptionSchema()
	for _, o := range opts {
		if o.Key == "reasoning_effort" {
			if len(o.Options) != 4 {
				t.Fatalf("推理强度档数 = %d", len(o.Options))
			}
		}
	}
	out, err := a.Execute(plugin.PluginInput{Data: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	res := out.Result.(map[string]any)
	if res["judge_capable"] != true {
		t.Fatal("Muse LLM-as-a-judge——judge_capable 必须 true")
	}
}

func TestNewAdaptersUpdateOptions(t *testing.T) {
	// UpdateOptions = Init 覆盖式——实时生效
	a := NewMuseGlimmerAdapter()
	if err := a.UpdateOptions(map[string]interface{}{"temperature": 0.5}); err != nil {
		t.Fatal(err)
	}
	if a.config.Temperature != 0.5 {
		t.Fatalf("UpdateOptions 未生效: %+v", a.config)
	}
}
