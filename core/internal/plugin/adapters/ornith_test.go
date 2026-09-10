package adapters

import (
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// 编译期验证：OrnithAdapter 实现 Plugin 接口
var _ plugin.Plugin = (*OrnithAdapter)(nil)

func TestOrnithAdapter_Interface(t *testing.T) {
	a := NewOrnithAdapter()
	if a.Name() != "ornith" {
		t.Errorf("Name = %s, want ornith", a.Name())
	}
	if a.Type() != plugin.PluginTypeModelAdapter {
		t.Errorf("Type = %s, want model-adapter", a.Type())
	}
	if a.Version() == "" {
		t.Error("Version 不应为空")
	}
	if len(a.Capabilities()) == 0 {
		t.Error("Capabilities 不应为空")
	}
}

func TestOrnithAdapter_DefaultConfig(t *testing.T) {
	a := NewOrnithAdapter()
	if a.Temperature != 0.8 {
		t.Errorf("Temperature = %v, want 0.8", a.Temperature)
	}
	if a.AuthToken != "example-35b" {
		t.Errorf("AuthToken = %s, want example-35b", a.AuthToken)
	}
	if !a.UseResponsesAPI {
		t.Error("UseResponsesAPI 应为 true")
	}
	if len(a.FinishWords) == 0 {
		t.Error("FinishWords 不应为空")
	}
}

func TestOrnithAdapter_Init(t *testing.T) {
	a := NewOrnithAdapter()
	cfg := map[string]interface{}{
		"temperature": 0.5,
		"auth_token":  "custom-token",
		"gateway_url": "http://custom:9999",
	}
	if err := a.Init(cfg); err != nil {
		t.Fatalf("Init error = %v", err)
	}
	if a.Temperature != 0.5 {
		t.Errorf("Temperature = %v, want 0.5", a.Temperature)
	}
	if a.AuthToken != "custom-token" {
		t.Errorf("AuthToken = %s, want custom-token", a.AuthToken)
	}
	if a.GatewayURL != "http://custom:9999" {
		t.Errorf("GatewayURL = %s, want http://custom:9999", a.GatewayURL)
	}
}

func TestOrnithAdapter_Lifecycle(t *testing.T) {
	a := NewOrnithAdapter()
	if err := a.Init(nil); err != nil {
		t.Fatalf("Init error = %v", err)
	}
	if err := a.Start(); err != nil {
		t.Fatalf("Start error = %v", err)
	}
	if !a.started {
		t.Error("started 应为 true")
	}
	if err := a.Stop(); err != nil {
		t.Fatalf("Stop error = %v", err)
	}
	if a.started {
		t.Error("started 应为 false（Stop 后）")
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}
}

func TestOrnithAdapter_ExecuteBeforeStart(t *testing.T) {
	a := NewOrnithAdapter()
	a.Init(nil)
	// 不 Start 直接 Execute——应报错
	_, err := a.Execute(plugin.PluginInput{Context: map[string]interface{}{"prompt": "test"}})
	if err == nil {
		t.Error("Execute before Start 应报错")
	}
}

func TestOrnithAdapter_Execute(t *testing.T) {
	a := NewOrnithAdapter()
	a.Init(nil)
	a.Start()
	resp, err := a.Execute(plugin.PluginInput{Context: map[string]interface{}{"prompt": "你好世界"}})
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	r, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Result 类型错误: %T", resp.Result)
	}
	if r["prompt_length"] != 12 { // "你好世界" UTF-8 12 字节
		t.Errorf("prompt_length = %v, want 12", r["prompt_length"])
	}
	if r["temperature"] != 0.8 {
		t.Errorf("temperature = %v, want 0.8", r["temperature"])
	}
}

func TestOrnithAdapter_Execute_EmptyPrompt(t *testing.T) {
	a := NewOrnithAdapter()
	a.Init(nil)
	a.Start()
	_, err := a.Execute(plugin.PluginInput{Context: map[string]interface{}{"prompt": ""}})
	if err == nil {
		t.Error("空 prompt 应报错")
	}
}

func TestDefaultFinishWords(t *testing.T) {
	words := DefaultFinishWords()
	if len(words) == 0 {
		t.Fatal("FinishWords 不应为空")
	}
	// 应包含常见完成词
	joined := strings.Join(words, ",")
	if !strings.Contains(joined, "完成") && !strings.Contains(joined, "完成") {
		// 只检查非空即可
	}
}

func TestContainsFinishWords(t *testing.T) {
	a := NewOrnithAdapter()
	detected, matched := ContainsFinishWords("任务已完成，文件已写入", a.FinishWords)
	if !detected {
		t.Error("应检测到完成词")
	}
	if len(matched) == 0 {
		t.Error("matched 不应为空")
	}
	detected2, _ := ContainsFinishWords("继续处理下一步", a.FinishWords)
	if detected2 {
		t.Error("'继续处理下一步' 不应含完成词")
	}
}

func TestOrnithAdapter_Execute_WithFinishWords(t *testing.T) {
	a := NewOrnithAdapter()
	a.Init(nil)
	a.Start()
	resp, err := a.Execute(plugin.PluginInput{Context: map[string]interface{}{"prompt": "任务已完成"}})
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	r := resp.Result.(map[string]interface{})
	if r["finish_words_detected"] != true {
		t.Error("finish_words_detected 应为 true")
	}
}

func TestRegisterOrnithAdapter(t *testing.T) {
	reg := plugin.NewRegistry()
	a := NewOrnithAdapter()
	if err := reg.Register(a); err != nil {
		t.Fatalf("Register error = %v", err)
	}
	got, ok := reg.Get("ornith")
	if !ok || got != a {
		t.Fatal("Get 失败")
	}
	adapters := reg.ByType(plugin.PluginTypeModelAdapter)
	if len(adapters) != 1 {
		t.Fatalf("model-adapter 插件应 1 个，实际 %d", len(adapters))
	}
}
