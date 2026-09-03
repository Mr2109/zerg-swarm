package orchestrator

// orchestrator_test.go — 编排器纯函数测试（2026-08-29 q5 覆盖补齐）
// 目标: truncate/countSuccess/parseDecomposeResponse/DefaultConfig（0% 包首测）

import (
	"strings"
	"testing"
	"time"
)

// TestTruncate 截断
func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Fatal("短字符串不应截断")
	}
	if truncate("hello world", 5) != "hello" {
		t.Fatal("长字符串应截断")
	}
	if truncate("", 3) != "" {
		t.Fatal("空字符串应保持")
	}
}

// TestCountSuccess 成功计数
func TestCountSuccess(t *testing.T) {
	refs := []MoAReference{
		{Model: "gemma-4-26B", Success: true},
		{Model: "example-8b-quant", Success: false},
		{Model: "gemma-4-12B", Success: true},
	}
	if countSuccess(refs) != 2 {
		t.Fatalf("应 2 成功: %d", countSuccess(refs))
	}
	if countSuccess(nil) != 0 {
		t.Fatal("空列表应 0")
	}
}

// TestParseDecomposeResponse_纯JSON 直接 JSON 数组
func TestParseDecomposeResponse_纯JSON(t *testing.T) {
	o := &Orchestrator{}
	tasks := o.parseDecomposeResponse(`[{"id":"1","desc":"任务1"},{"id":"2","desc":"任务2"}]`)
	if len(tasks) != 2 || tasks[0].ID != "1" {
		t.Fatalf("解析失败: %+v", tasks)
	}
}

// TestParseDecomposeResponse_带解释 模型输出带前后文 → 提取 [] 段
func TestParseDecomposeResponse_带解释(t *testing.T) {
	o := &Orchestrator{}
	content := "好的，我来分解：\n[{\"id\":\"1\",\"desc\":\"任务1\"}]\n以上是分解结果"
	tasks := o.parseDecomposeResponse(content)
	if len(tasks) != 1 || tasks[0].ID != "1" {
		t.Fatalf("提取失败: %+v", tasks)
	}
}

// TestParseDecomposeResponse_代码块 模型输出带 ```json 代码块
func TestParseDecomposeResponse_代码块(t *testing.T) {
	o := &Orchestrator{}
	content := "```json\n[{\"id\":\"1\",\"desc\":\"任务1\"}]\n```"
	tasks := o.parseDecomposeResponse(content)
	if len(tasks) != 1 {
		t.Fatalf("代码块提取失败: %+v", tasks)
	}
}

// TestParseDecomposeResponse_非法 无法解析 → nil
func TestParseDecomposeResponse_非法(t *testing.T) {
	o := &Orchestrator{}
	if tasks := o.parseDecomposeResponse("完全不是 JSON"); tasks != nil {
		t.Fatalf("非法内容应 nil: %+v", tasks)
	}
}

// TestDefaultOrchestratorConfig 默认配置
func TestDefaultOrchestratorConfig(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	if cfg.BrainModel == "" || cfg.HandModel == "" {
		t.Fatal("脑/手模型不应为空")
	}
	if cfg.MaxTasks <= 0 || cfg.Timeout <= 0 {
		t.Fatal("MaxTasks/Timeout 应为正")
	}
	if cfg.CompositeName == "" {
		t.Fatal("CompositeName 不应为空")
	}
}

// TestDefaultMoAConfig MoA 默认配置（参考模型 + 聚合器）
func TestDefaultMoAConfig(t *testing.T) {
	cfg := DefaultMoAConfig()
	if len(cfg.ReferenceModels) < 2 {
		t.Fatalf("参考模型应 ≥2: %v", cfg.ReferenceModels)
	}
	if cfg.AggregatorModel == "" {
		t.Fatal("聚合器不应为空")
	}
	if cfg.MinSuccessful != 1 || cfg.MaxRetries != 2 {
		t.Fatalf("容错参数错误: min=%d retries=%d", cfg.MinSuccessful, cfg.MaxRetries)
	}
	if cfg.RefTemperature != 0.7 || cfg.RefMaxTokens != 2000 {
		t.Fatalf("参考参数错误: temp=%.1f max=%d", cfg.RefTemperature, cfg.RefMaxTokens)
	}
	if cfg.RefTimeout != 3*time.Minute {
		t.Fatalf("参考超时 = %v", cfg.RefTimeout)
	}
}

// TestNewOrchestrator_NilConfig nil → 默认配置
func TestNewOrchestrator_NilConfig(t *testing.T) {
	o := NewOrchestrator(nil, nil)
	if o == nil {
		t.Fatal("应创建编排器")
	}
	if o.config == nil || o.config.BrainModel == "" {
		t.Fatal("nil cfg 应使用默认")
	}
	if !strings.Contains(o.config.CompositeName, "zerg-baiyan") {
		t.Fatalf("CompositeName = %s", o.config.CompositeName)
	}
}
