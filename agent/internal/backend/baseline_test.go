// baseline_test.go —— 模型级身份探针的回归测试（P4 退场清理后的残余保留面）。
//
// 原属本文件的 L3 托管方式分类 / L2 命令行解析 / M8 准入扣减用例随基线机制
// 一起退场（设计 §10.1 baseline.go 行 / 附录 C·C8）。
// 这些用例全部不依赖真机状态（纯输入→输出）。
package backend

import "testing"

// ── L1：身份解析（端口活 ≠ 身份对，今天实测的教训）──────────────────────────

func TestParseModelIdentity_BothShapes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"llama.cpp 系（models[].name = 权重路径）",
			`{"models":[{"name":"/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf","model":"/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf"}]}`,
			"/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf",
		},
		{
			"ds4 系（data[].id）",
			`{"object":"list","data":[{"id":"deepseek-v4-flash","name":"DeepSeek V4 Flash Vision Experimental","context_length":1048576}]}`,
			"deepseek-v4-flash",
		},
		{
			"只有 model 字段也能取",
			`{"models":[{"model":"/data/models/k2/k2horizon-q4_k_m.gguf"}]}`,
			"/data/models/k2/k2horizon-q4_k_m.gguf",
		},
		{"非 JSON ⇒ 不编造", "not json at all", ""},
		{"空对象 ⇒ 空", `{}`, ""},
	}
	for _, c := range cases {
		if got := parseModelIdentity([]byte(c.body)); got != c.want {
			t.Errorf("%s: got %q，期望 %q", c.name, got, c.want)
		}
	}
}
