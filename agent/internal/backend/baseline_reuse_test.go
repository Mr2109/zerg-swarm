// baseline_reuse_test.go —— M10 身份匹配的回归。用例里的身份逐字取自 X3 实测。
package backend

import "testing"

func TestNormalizeIdentity(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/data/models/k2/k2horizon-q4_k_m.gguf", "k2horizon-q4_k_m.gguf"},
		{"  k2horizon-q4_k_m.gguf  ", "k2horizon-q4_k_m.gguf"},
		{"/data/models/k2/K2HORIZON-Q4_K_M.GGUF", "k2horizon-q4_k_m.gguf"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeIdentity(c.in); got != c.want {
			t.Fatalf("normalizeIdentity(%q)=%q，期望 %q", c.in, got, c.want)
		}
	}
}

func TestBaselineReuseMatch(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"路径 vs basename（X3 真实形态）", "/data/models/k2/k2horizon-q4_k_m.gguf", "k2horizon-q4_k_m.gguf", true},
		{"两边都是全路径", "/data/models/k2/k2horizon-q4_k_m.gguf", "/data/models/k2/k2horizon-q4_k_m.gguf", true},
		{"大小写差异不误判", "/data/models/k2/K2Horizon-Q4_K_M.gguf", "k2horizon-q4_k_m.gguf", true},
		{"不同权重绝不匹配", "/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf", "k2horizon-q4_k_m.gguf", false},
		{"前缀相同但不同名（防前缀匹配）", "k2horizon-q4_k_m.gguf", "k2horizon-q4_k_m-extra.gguf", false},
		{"一侧为空不算匹配（防空==空）", "", "k2horizon-q4_k_m.gguf", false},
		{"两侧都为空更不算匹配", "", "", false},
	}
	for _, c := range cases {
		if got := baselineReuseMatch(c.a, c.b); got != c.want {
			t.Fatalf("%s：baselineReuseMatch(%q,%q)=%v，期望 %v", c.name, c.a, c.b, got, c.want)
		}
	}
}

// TestBaselineReuseCandidateLocked 用 X3 实测的两条基线服务，验证"只在真同名时才复用"。
func TestBaselineReuseCandidateLocked(t *testing.T) {
	svcs := []BaselineService{
		{Port: 9000, Identity: "/data/models/qwen/Qwen3.8-27B-Q4_K_M-vcruz305.gguf", Listening: true, Kind: "llama", Class: "bare"},
		{Port: 9001, Identity: "/data/models/k2/k2horizon-q4_k_m.gguf", Listening: true, Kind: "llama", Class: "screen"},
	}

	// ① 命中 K2（请求侧只给 basename）⇒ 应指向 9001
	if got, ok := baselineReuseCandidateLocked(svcs, []string{"k2horizon-q4_k_m.gguf"}, nil); !ok || got.Port != 9001 {
		t.Fatalf("应命中 :9001 的 K2，实得 ok=%v port=%d", ok, got.Port)
	}
	// ② 请求 GLM（本机没跑）⇒ **绝不许**退化成"随便挑一个已加载的"
	if got, ok := baselineReuseCandidateLocked(svcs, []string{"GLM-5.3-Flash-Q2.gguf"}, nil); ok {
		t.Fatalf("不同权重不得复用，实得命中 port=%d", got.Port)
	}
	// ③ 端口已在被本端托管占用 ⇒ 不复用（那不是"别人的手工服务"）
	if _, ok := baselineReuseCandidateLocked(svcs, []string{"k2horizon-q4_k_m.gguf"}, map[int]bool{9001: true}); ok {
		t.Fatal("本端已托管占用的端口不得复用")
	}
	// ④ 没在监听 ⇒ 不复用（复用没意义，但也不去动它）
	svcs[1].Listening = false
	if _, ok := baselineReuseCandidateLocked(svcs, []string{"k2horizon-q4_k_m.gguf"}, nil); ok {
		t.Fatal("未监听的基线服务不得复用")
	}
	// ⑤ 未声明基线（空列表）⇒ 什么都不发生
	if _, ok := baselineReuseCandidateLocked(nil, []string{"k2horizon-q4_k_m.gguf"}, nil); ok {
		t.Fatal("未声明基线时不得复用")
	}
}

func TestBaselineReuseIdentities(t *testing.T) {
	got := baselineReuseIdentities("example-moe-36b", "/data/models/k2/k2horizon-q4_k_m.gguf")
	if len(got) != 2 {
		t.Fatalf("应给出权重路径与模型名两个身份写法，实得 %v", got)
	}
	if got := baselineReuseIdentities("", ""); len(got) != 0 {
		t.Fatalf("两者皆空应返回空，实得 %v", got)
	}
}
