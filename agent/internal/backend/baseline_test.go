// baseline_test.go —— P2 基线服务检查的回归测试（纯函数 + 准入扣减）。
//
// 设计依据：docs/01-设计/设计-子端服务切换与基线服务声明-20260914.md §9.1（L1/L2/L3）、
// §11 M8（配额显式化）。这些用例全部不依赖真机状态（纯输入→输出）。
package backend

import (
	"testing"
	"time"
)

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

// ── L3：托管方式分类（决定"怎么停"）────────────────────────────────────────

func TestServiceClassFrom_SystemdWinsByCgroup(t *testing.T) {
	cgroup := "12:devices:/\n1:name=systemd:/system.slice/ds4-server.service\n0::/system.slice/ds4-server.service\n"
	class, unit, screen := serviceClassFrom(cgroup, [][]string{{"/home/g01/ds4-server", "--rocm"}})
	if class != "systemd" {
		t.Fatalf("应判为 systemd，实得 %q", class)
	}
	if unit != "ds4-server.service" {
		t.Fatalf("单元名应为 ds4-server.service，实得 %q", unit)
	}
	if screen != "" {
		t.Fatalf("systemd 类不该给 screen 会话名，实得 %q", screen)
	}
}

func TestServiceClassFrom_ScreenViaAncestry(t *testing.T) {
	// 真机形态：SCREEN -dmS k2p9001 bash -c llama-server …
	ancestry := [][]string{
		{"/home/g01/llama-k2/build-k2/bin/llama-server", "-m", "/data/models/k2/k2horizon-q4_k_m.gguf", "--port", "9001"},
		{"bash", "-c", "/home/g01/llama-k2/build-k2/bin/llama-server -m …"},
		{"SCREEN", "-dmS", "k2p9001", "bash", "-c", "/home/g01/llama-k2/build-k2/bin/llama-server -m …"},
	}
	class, unit, screen := serviceClassFrom("0::/user.slice/user-1000.slice/session-2.scope\n", ancestry)
	if class != "screen" {
		t.Fatalf("应判为 screen，实得 %q", class)
	}
	if screen != "k2p9001" {
		t.Fatalf("会话名应为 k2p9001，实得 %q", screen)
	}
	if unit != "" {
		t.Fatalf("screen 类不该给单元名，实得 %q", unit)
	}
}

func TestServiceClassFrom_ScreenShortFormAndBare(t *testing.T) {
	if c, _, s := serviceClassFrom("", [][]string{{"screen", "-S", "abc"}, {"sh"}}); c != "screen" || s != "abc" {
		t.Fatalf("screen -S 形式应识别，实得 class=%q screen=%q", c, s)
	}
	if c, u, s := serviceClassFrom("0::/user.slice/user-1000.slice/session-9.scope\n",
		[][]string{{"/home/g01/llama.cpp-src/build-hip-flash/bin/llama-server", "-m", "x", "--port", "9000"}}); c != "bare" || u != "" || s != "" {
		t.Fatalf("裸进程应判为 bare，实得 class=%q unit=%q screen=%q", c, u, s)
	}
}

// ── L2：命令行解析（端口 / 是否推理进程）────────────────────────────────────

func TestParsePortFromArgv(t *testing.T) {
	cases := []struct {
		argv []string
		want int
	}{
		{[]string{"llama-server", "-m", "x", "--port", "9000"}, 9000},
		{[]string{"ds4-server", "--rocm", "--ctx", "1048576", "--host", "127.0.0.1", "--port", "9000", "--vision", "v.gguf"}, 9000},
		{[]string{"llama-server", "--port=9001"}, 9001},
		{[]string{"llama-server", "-p", "8100"}, 8100},
		{[]string{"llama-server", "-m", "x"}, 0},
		{[]string{"llama-server", "--port", "99999"}, 0},
		{[]string{"llama-server", "--port", "abc"}, 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := parsePortFromArgv(c.argv); got != c.want {
			t.Errorf("parsePortFromArgv(%v)=%d，期望 %d", c.argv, got, c.want)
		}
	}
}

func TestIsInferenceArgv(t *testing.T) {
	yes := [][]string{
		{"/home/g01/llama.cpp-src/build-hip-flash/bin/llama-server", "-m", "x"},
		{"llama-server"},
		{"/home/g01/ds4-server", "--rocm"},
		{"ds4-server"},
	}
	no := [][]string{
		{}, {"bash"}, {"/usr/bin/python3", "train.py"}, {"llama-cli"},
	}
	for _, a := range yes {
		if !isInferenceArgv(a) {
			t.Errorf("应识别为推理服务：%v", a)
		}
	}
	for _, a := range no {
		if isInferenceArgv(a) {
			t.Errorf("不应识别为推理服务：%v", a)
		}
	}
}

// ── M8：准入扣减（机型配额 / 预留 / 基线占用 / 已驻留）──────────────────────

func TestEffectiveAvailableGb_BudgetAndReserve(t *testing.T) {
	// 无基线声明 ⇒ 不介入基线那一段；配额与预留按环境变量生效。
	t.Setenv(EnvBaselinePorts, "")
	t.Setenv(EnvMachineBudgetGB, "50")
	t.Setenv(EnvBudgetReserveGB, "8")
	m := newEvictTestManager(3, map[string]*subproc{})

	if got := m.EffectiveAvailableGb(200); got != 42 {
		t.Fatalf("配额 50 − 预留 8 = 42，实得 %.1f", got)
	}
	// 系统可用比配额还小 ⇒ 取系统可用
	if got := m.EffectiveAvailableGb(30); got != 22 {
		t.Fatalf("min(30,50)=30 − 预留 8 = 22，实得 %.1f", got)
	}
}

func TestEffectiveAvailableGb_NeverNegative(t *testing.T) {
	t.Setenv(EnvBaselinePorts, "")
	t.Setenv(EnvMachineBudgetGB, "4")
	t.Setenv(EnvBudgetReserveGB, "99")
	m := newEvictTestManager(3, map[string]*subproc{})
	if got := m.EffectiveAvailableGb(200); got != 0 {
		t.Fatalf("扣成负数应夹到 0，实得 %.1f", got)
	}
}

func TestBaselinePorts_ConfigParsing(t *testing.T) {
	t.Setenv(EnvBaselinePorts, "9000,9001")
	if got := baselinePorts(); len(got) != 2 || got[0] != 9000 || got[1] != 9001 {
		t.Fatalf("应解析出 [9000 9001]，实得 %v", got)
	}
	if !baselinePortsConfigured() {
		t.Fatal("已配置却报告未配置")
	}
	t.Setenv(EnvBaselinePorts, "")
	if got := baselinePorts(); len(got) != 0 {
		t.Fatalf("未配置应返回空，实得 %v", got)
	}
	if baselinePortsConfigured() {
		t.Fatal("未配置却报告已配置")
	}
}

func TestBaselineServices_NilWhenUnconfigured(t *testing.T) {
	t.Setenv(EnvBaselinePorts, "")
	m := newEvictTestManager(1, map[string]*subproc{})
	if got := m.BaselineServices(); got != nil {
		t.Fatalf("未声明基线 ⇒ 必须完全不介入（返回 nil），实得 %v", got)
	}
	if got := baselineOccupiedGb(nil); got != 0 {
		t.Fatalf("空清单占用应为 0，实得 %.1f", got)
	}
}

func TestBaselineServices_OwnedPortExcluded(t *testing.T) {
	// 本端自己占的端口不算基线：避免把托管项误报成"未托管的基线服务"。
	t.Setenv(EnvBaselinePorts, "9500")
	m := newEvictTestManager(1, map[string]*subproc{
		"mine": {model: "mine", state: StateReady, port: 9500, entry: nil, lastUsed: time.Now()},
	})
	if got := m.BaselineServices(); len(got) != 0 {
		t.Fatalf("被本端占用的端口不应进基线清单，实得 %v", got)
	}
}
