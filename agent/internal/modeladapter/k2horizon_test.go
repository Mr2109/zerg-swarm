package modeladapter

import (
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// TestDispatchK2Horizon K2-Horizon 家族 Dispatch 命中测试（2026-09-08 接入）。
func TestDispatchK2Horizon(t *testing.T) {
	cases := []struct {
		model  string
		expect string // 期望命中的适配器 Name
	}{
		{"example-moe-36b", "k2-horizon"},
		{"k2-horizon-mova-36b-a4b", "k2-horizon"},
		{"K2-Horizon-7B", "k2-horizon"},
		{"K2-Horizon-375B-A23B", "k2-horizon"},
		// 不误伤其他模型
		{"Qwen3.8-Flash-Next", "qwen3.8-flash"},
		{"example-35b-v2", "ornith"},
		{"deepseek-v4-flash", "deepseek"},
		{"不认识的模型", ""}, // generic
	}
	// 公开快照会把**本文件的用例名**替换成示例名（K2-Horizon-* → example-*），
	// "按名字派发"的断言前提随之不成立 → 跳过（本机/私有仓照常执行，覆盖不丢）。
	// 判据必须看"用例表自己"：派发代码本身不会被改写，拿它探测判断不出快照形态。
	if !strings.Contains(cases[0].model, "K2-Horizon") {
		t.Skip("用例模型名已被导出规则改写（公开快照形态）——跳过按名派发断言")
	}
	for _, c := range cases {
		got := Dispatch(c.model)
		if got.Name() != c.expect {
			t.Errorf("Dispatch(%q) = %q, want %q", c.model, got.Name(), c.expect)
		}
	}
}

// TestK2HorizonBuildArgs K2 启动参数关键项（显式上下文 + jinja + reasoning-preserve）。
func TestK2HorizonBuildArgs(t *testing.T) {
	a := &K2Horizon{}
	args := a.BuildArgs(&registry.ModelEntry{File: "/data/models/k2/x.gguf"}, 8209)
	joined := strings.Join(args, " ")
	for _, want := range []string{"-c 32768", "--jinja", "--reasoning-preserve", "--port 8209", "-m /data/models/k2/x.gguf"} {
		if !strings.Contains(joined, want) {
			t.Errorf("BuildArgs 缺少 %q: %s", want, joined)
		}
	}
}
