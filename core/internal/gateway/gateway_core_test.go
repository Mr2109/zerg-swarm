package gateway

// gateway_core_test.go — 网关核心逻辑测试（2026-08-29 q5 覆盖补齐）
// 目标: markFailure(0%) / classifyRouteError(0%) / pickRouteLocal(0%) / extractModelWithAction(0%)

import (
	"errors"
	"testing"

	"zerg/core/internal/config"
)

// TestClassifyRouteError_CircuitOpen 熔断/无候选 → circuit_open(503)
func TestClassifyRouteError_CircuitOpen(t *testing.T) {
	cases := []string{
		"模型 X 唯一候选 x3 已熔断——无可用候选",
		"模型 Y 无可用候选（全部熔断或排除: local,x3）",
		"模型 Z 已被排除",
	}
	for _, msg := range cases {
		code, status := classifyRouteError(errors.New(msg))
		if code != "circuit_open" || status != 503 {
			t.Fatalf("%q → %s/%d，应 circuit_open/503", msg, code, status)
		}
	}
}

// TestClassifyRouteError_ModelNotFound 未在路由表 → model_not_found(404)
func TestClassifyRouteError_ModelNotFound(t *testing.T) {
	code, status := classifyRouteError(errors.New("模型 Foo 未在路由表中找到"))
	if code != "model_not_found" || status != 404 {
		t.Fatalf("→ %s/%d，应 model_not_found/404", code, status)
	}
}

// TestClassifyRouteError_Default 其他 → upstream_fail(502)
func TestClassifyRouteError_Default(t *testing.T) {
	code, status := classifyRouteError(errors.New("未知错误"))
	if code != "upstream_fail" || status != 502 {
		t.Fatalf("→ %s/%d，应 upstream_fail/502", code, status)
	}
	// nil → api_error(502)
	code2, status2 := classifyRouteError(nil)
	if code2 != "api_error" || status2 != 502 {
		t.Fatalf("nil → %s/%d，应 api_error/502", code2, status2)
	}
}

// TestMarkFailure_Counts markFailure 累计失败计数（熔断判定输入）
func TestMarkFailure_Counts(t *testing.T) {
	g := NewGateway("test-token", nil, nil, nil, nil)
	g.markFailure("x3")
	g.markFailure("x3")
	if g.failCounts["x3"] != 2 {
		t.Fatalf("failCounts[x3] = %d，应 2", g.failCounts["x3"])
	}
	// markSuccess 清零
	g.markSuccess("x3")
	if g.failCounts["x3"] != 0 {
		t.Fatalf("markSuccess 后 failCounts[x3] = %d，应 0", g.failCounts["x3"])
	}
}

// TestExtractModelWithAction 标准提取优先——动作路由兜底
func TestExtractModelWithAction(t *testing.T) {
	g := NewGateway("test-token", nil, nil, nil, nil)
	// 标准提取（body 有 model）
	model, usedAction, err := g.extractModelWithAction([]byte(`{"model":"Qwen3.8-27B","input":"hi"}`), "/v1/responses")
	if err != nil || model != "Qwen3.8-27B" || usedAction {
		t.Fatalf("标准提取应成功: model=%s usedAction=%v err=%v", model, usedAction, err)
	}
	// 无 model → 动作路由（可能失败——但不应 panic）
	_, _, _ = g.extractModelWithAction([]byte(`{"input":"hi"}`), "/v1/responses")
}

// TestPickRouteLocal 本地路由选择（local 候选存在 → 返回 local）
func TestPickRouteLocal(t *testing.T) {
	cfg := configForTest()
	g := NewGateway("test-token", cfg, nil, nil, nil)
	route, err := g.pickRouteLocal("Qwen3.8-27B")
	if err != nil {
		t.Fatalf("pickRouteLocal 失败: %v", err)
	}
	if route.Host != "local" {
		t.Fatalf("Host = %s，应 local", route.Host)
	}
	// 不存在的模型 → 错误
	if _, err := g.pickRouteLocal("不存在-模型"); err == nil {
		t.Fatal("不存在的模型应报错")
	}
}

// configForTest 测试用 FleetConfig（含 local 候选）
func configForTest() *config.FleetConfig {
	return &config.FleetConfig{
		Models: map[string][]config.ModelCandidate{
			"Qwen3.8-27B": {
				{Host: "local", File: "~/models/Qwen3.8-27B-Q4_K_M-vcruz305.gguf", MemGb: 18},
				{Host: "x3", File: "/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf", MemGb: 18},
			},
		},
	}
}
