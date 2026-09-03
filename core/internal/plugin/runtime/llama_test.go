package runtime

// llama_test.go — llama 运行时配置/生命周期测试（2026-08-29 q5 覆盖补齐）
// 目标: Init 配置解析 / Start 未初始化检查 / IsRunning / Stop 未初始化

import (
	"strings"
	"testing"
	"time"
)

// TestLlamaInit_Defaults 默认值
func TestLlamaInit_Defaults(t *testing.T) {
	l := NewLlamaRuntime()
	if err := l.Init(nil); err != nil {
		t.Fatalf("Init(nil) 失败: %v", err)
	}
	if l.Host != "127.0.0.1" || l.Port != "8081" {
		t.Fatalf("默认 host/port = %s:%s，应 127.0.0.1:8081", l.Host, l.Port)
	}
	if l.NGPULayers != 999 {
		t.Fatalf("默认 ngl = %d，应 999（全放 GPU）", l.NGPULayers)
	}
	if !l.initialized {
		t.Fatal("Init 后应 initialized")
	}
}

// TestLlamaInit_Config 完整配置解析
func TestLlamaInit_Config(t *testing.T) {
	l := NewLlamaRuntime()
	cfg := map[string]interface{}{
		"host":            "<worker-ip>",
		"port":            int64(8100), // int64 类型
		"model_file":      "/data/models/test.gguf",
		"ngl":             float64(32), // float64（JSON 反序列化）
		"auto_start":      true,
		"executable":      "/usr/bin/llama-server",
		"startup_timeout": 120, // int
		"request_timeout": 300,
	}
	if err := l.Init(cfg); err != nil {
		t.Fatalf("Init 失败: %v", err)
	}
	if l.Host != "<worker-ip>" || l.Port != "8100" {
		t.Fatalf("host/port = %s:%s，应 <worker-ip>:8100", l.Host, l.Port)
	}
	if l.NGPULayers != 32 {
		t.Fatalf("ngl = %d，应 32", l.NGPULayers)
	}
	if !l.AutoStart || l.ModelFile != "/data/models/test.gguf" {
		t.Fatal("auto_start/model_file 解析失败")
	}
	if l.StartupTimeout != 120*time.Second || l.RequestTimeout != 300*time.Second {
		t.Fatalf("timeout = %v/%v，应 120s/300s", l.StartupTimeout, l.RequestTimeout)
	}
	if !strings.HasPrefix(l.baseURL, "http://<worker-ip>:8100") {
		t.Fatalf("baseURL = %s", l.baseURL)
	}
}

// TestLlamaInit_NglInvalid 非法 ngl 类型 → 报错
func TestLlamaInit_NglInvalid(t *testing.T) {
	l := NewLlamaRuntime()
	err := l.Init(map[string]interface{}{"ngl": "many"})
	if err == nil || !strings.Contains(err.Error(), "ngl must be number") {
		t.Fatalf("非法 ngl 应报错: %v", err)
	}
}

// TestLlamaStart_NotInitialized 未 Init → Start 报错
func TestLlamaStart_NotInitialized(t *testing.T) {
	l := NewLlamaRuntime() // 未 Init
	err := l.Start()
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("未 Init Start 应报错: %v", err)
	}
}

// TestLlamaStop_NotInitialized 未 Init → Stop 幂等成功（不 panic 不报错）
func TestLlamaStop_NotInitialized(t *testing.T) {
	l := NewLlamaRuntime()
	if err := l.Stop(); err != nil {
		t.Fatalf("未 Init Stop 应幂等成功: %v", err)
	}
}

// TestLlamaIsRunning_EmptyProcess 空进程 → false
func TestLlamaIsRunning_EmptyProcess(t *testing.T) {
	p := &LlamaServerProcess{} // 无 cmd——不 running
	if p.IsRunning() {
		t.Fatal("空进程不应 running")
	}
}
