package adapters

import (
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// ─── 测试 Ds4Adapter 实现 Plugin 接口 ────────────────────────────────────────

func TestDs4Adapter_ImplementsPlugin(t *testing.T) {
	var p plugin.Plugin = NewDs4Adapter()
	if p == nil {
		t.Fatal("Ds4Adapter should implement Plugin interface")
	}
}

func TestDs4Adapter_Name(t *testing.T) {
	d := NewDs4Adapter()
	if d.Name() != "ds4" {
		t.Errorf("expected name 'ds4', got '%s'", d.Name())
	}
}

func TestDs4Adapter_Type(t *testing.T) {
	d := NewDs4Adapter()
	if d.Type() != plugin.PluginTypeModelAdapter {
		t.Errorf("expected type 'model-adapter', got '%s'", d.Type())
	}
}

func TestDs4Adapter_Version(t *testing.T) {
	d := NewDs4Adapter()
	if d.Version() != "0.1.0" {
		t.Errorf("expected version '0.1.0', got '%s'", d.Version())
	}
}

func TestDs4Adapter_Capabilities(t *testing.T) {
	d := NewDs4Adapter()
	caps := d.Capabilities()
	if len(caps) < 5 {
		t.Errorf("expected at least 5 capabilities, got %d", len(caps))
	}
	// 检查关键能力
	expected := []string{"ds4-temperature-0.6", "deepseek-v4-flash"}
	for _, e := range expected {
		found := false
		for _, c := range caps {
			if c == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected capability '%s' not found", e)
		}
	}
}

func TestDs4Adapter_DefaultConfig(t *testing.T) {
	d := NewDs4Adapter()
	if d.Temperature != 0.6 {
		t.Errorf("expected temperature 0.6, got %f", d.Temperature)
	}
	if d.AuthToken != "ds4-gateway-token" {
		t.Errorf("expected auth_token 'ds4-gateway-token', got '%s'", d.AuthToken)
	}
	if d.APIFormat != "openai" {
		t.Errorf("expected api_format 'openai', got '%s'", d.APIFormat)
	}
}

func TestDs4Adapter_Init(t *testing.T) {
	d := NewDs4Adapter()
	cfg := map[string]interface{}{
		"temperature":  0.7,
		"auth_token":   "custom-token",
		"finish_words": []string{"完成", "done"},
		"gateway_url":  "http://localhost:9090",
		"api_format":   "responses",
	}
	if err := d.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if d.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", d.Temperature)
	}
	if d.AuthToken != "custom-token" {
		t.Errorf("expected auth_token 'custom-token', got '%s'", d.AuthToken)
	}
	if d.APIFormat != "responses" {
		t.Errorf("expected api_format 'responses', got '%s'", d.APIFormat)
	}
}

func TestDs4Adapter_Init_InvalidTemp(t *testing.T) {
	d := NewDs4Adapter()
	cfg := map[string]interface{}{
		"temperature": "invalid",
	}
	err := d.Init(cfg)
	if err == nil {
		t.Error("expected error for invalid temperature type")
	}
}

func TestDs4Adapter_Init_InvalidAuthToken(t *testing.T) {
	d := NewDs4Adapter()
	cfg := map[string]interface{}{
		"auth_token": 123,
	}
	err := d.Init(cfg)
	if err == nil {
		t.Error("expected error for invalid auth_token type")
	}
}

func TestDs4Adapter_StartStopClose(t *testing.T) {
	d := NewDs4Adapter()
	// 未初始化不能启动
	if err := d.Start(); err == nil {
		t.Error("expected error when starting without init")
	}
	// 初始化后启动
	if err := d.Init(nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := d.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	// 停止
	if err := d.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	// 关闭
	if err := d.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestDs4Adapter_Execute(t *testing.T) {
	d := NewDs4Adapter()
	if err := d.Init(nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 测试普通执行
	input := plugin.PluginInput{
		TaskID: "test-1",
		Data:   "hello world",
	}
	output, err := d.Execute(input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if output.Result == nil {
		t.Error("expected result, got nil")
	}
	result := output.Result.(map[string]interface{})
	if result["model"] != "deepseek-v4-flash" {
		t.Errorf("expected model 'deepseek-v4-flash', got '%v'", result["model"])
	}
	if result["temperature"] != 0.6 {
		t.Errorf("expected temperature 0.6, got '%v'", result["temperature"])
	}
}

func TestDs4Adapter_Execute_FinishWords(t *testing.T) {
	d := NewDs4Adapter()
	d.FinishWords = []string{"任务完成", "完成"}
	if err := d.Init(nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	input := plugin.PluginInput{
		TaskID: "test-2",
		Data:   "任务完成，已写入文件",
	}
	output, err := d.Execute(input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	result := output.Result.(map[string]interface{})
	if result["finish_words_detected"] != true {
		t.Error("expected finish words detected")
	}
}

func TestDs4Adapter_Execute_NotInitialized(t *testing.T) {
	d := NewDs4Adapter()
	input := plugin.PluginInput{
		TaskID: "test-3",
		Data:   "test",
	}
	_, err := d.Execute(input)
	if err == nil {
		t.Error("expected error when executing without init")
	}
}

// ─── 测试 OrnithAdapter 和 Ds4Adapter 并存 ────────────────────────────────────

func TestOrnithAndDs4_Parallel(t *testing.T) {
	o := NewOrnithAdapter()
	d := NewDs4Adapter()

	if o.Name() == d.Name() {
		t.Error("ornith and ds4 should have different names")
	}
	if o.Type() != d.Type() {
		t.Error("both should be model-adapter type")
	}
	if o.Temperature == d.Temperature {
		t.Error("ornith and ds4 should have different temperatures")
	}
}
