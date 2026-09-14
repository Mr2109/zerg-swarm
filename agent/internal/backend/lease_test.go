// lease_test.go —— P3 租约存档内核的回归测试（存储 / 到期判定 / 脱敏）。
//
// 设计依据：设计-子端服务切换与基线服务声明-20260914.md §9.3、§10 R5/R7、§11 M1/M5。
// 全部用例不碰真机状态：租约目录用 ZERG_LEASE_DIR 指向 t.TempDir()。
package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── 存储：写/读/列/删 ────────────────────────────────────────────────────────

func TestLeaseStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(LeaseDirEnv, dir)

	l := ServiceLease{
		Port:       9001,
		Kind:       kindLlama,
		Class:      "screen",
		ScreenName: "k2p9001",
		Identity:   "/data/models/k2/k2horizon-q4_k_m.gguf",
		PID:        1129430,
		Pipeline:   []int{1129426, 1129428, 1129430},
		Argv:       []string{"/home/g01/llama-k2/build-k2/bin/llama-server", "-m", "x", "--port", "9001"},
		Cwd:        "/home/g01/llama-k2/build-k2/bin",
		EnvFiltered: map[string]string{
			"LD_LIBRARY_PATH": "/home/g01/ds4-pr670/rocm-libs",
			"ZERG_TOKEN":      "[REDACTED]",
		},
		StartTime: 12345678,
		Note:      "测试",
	}
	if err := SaveLease(l); err != nil {
		t.Fatalf("SaveLease 失败：%v", err)
	}

	got, ok := LoadLease(9001)
	if !ok {
		t.Fatal("LoadLease 未命中")
	}
	if got.State != LeaseBorrowed {
		t.Errorf("未指定 state 时应落为 borrowed，实得 %q", got.State)
	}
	if got.TTLS != DefaultLeaseTTLS || got.MaxHoldS != DefaultMaxHoldS {
		t.Errorf("未指定时长应落默认值，实得 ttl=%d max_hold=%d", got.TTLS, got.MaxHoldS)
	}
	if got.LeaseID == "" || got.AcquiredAt.IsZero() {
		t.Error("LeaseID / AcquiredAt 应被补齐")
	}
	if got.Class != "screen" || got.ScreenName != "k2p9001" || len(got.Pipeline) != 3 {
		t.Errorf("字段回读不一致：%+v", got)
	}

	list := ListLeases()
	if len(list) != 1 || list[0].Port != 9001 {
		t.Fatalf("ListLeases 应返回 1 条 9001，实得 %+v", list)
	}

	// 权限必须是 0600（存档含命令行与 cwd，属敏感）
	info, err := os.Stat(filepath.Join(dir, "9001.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("租约文件权限应为 0600，实得 %o", perm)
	}

	if err := DeleteLease(9001); err != nil {
		t.Fatalf("DeleteLease 失败：%v", err)
	}
	if _, ok := LoadLease(9001); ok {
		t.Fatal("删除后仍能读到")
	}
	// 重复删除不应报错（幂等）
	if err := DeleteLease(9001); err != nil {
		t.Fatalf("重复删除应幂等：%v", err)
	}
}

func TestLeaseStore_ListSortsAndIgnoresJunk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(LeaseDirEnv, dir)
	for _, p := range []int{9002, 9000, 9001} {
		if err := SaveLease(ServiceLease{Port: p, Argv: []string{"x"}}); err != nil {
			t.Fatal(err)
		}
	}
	// 垃圾文件不该让列举出错
	_ = os.WriteFile(filepath.Join(dir, "notaport.json"), []byte("{"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("ignore me"), 0o600)

	got := ListLeases()
	want := []int{9000, 9001, 9002}
	if len(got) != len(want) {
		t.Fatalf("应列出 %d 条，实得 %d：%+v", len(want), len(got), got)
	}
	for i, p := range want {
		if got[i].Port != p {
			t.Fatalf("应按端口排序 %v，实得 %+v", want, got)
		}
	}
}

func TestLeaseStore_InvalidPortRejected(t *testing.T) {
	t.Setenv(LeaseDirEnv, t.TempDir())
	if err := SaveLease(ServiceLease{Port: 0}); err == nil {
		t.Fatal("port=0 应被拒绝")
	}
}

// ── 到期判定（wall-clock，崩溃后别的进程也能算）──────────────────────────────

func TestLeaseActionable_MaxHoldWinsOverTTL(t *testing.T) {
	now := time.Now()
	l := ServiceLease{
		State:      LeaseBorrowed,
		AcquiredAt: now.Add(-2 * time.Hour),
		LastActive: now.Add(-2 * time.Hour),
		TTLS:       300,
		MaxHoldS:   1800, // 30min 绝对上限
	}
	should, why := LeaseActionable(l, now)
	if !should {
		t.Fatal("已超 max_hold 应判为需归还")
	}
	if !strings.Contains(why, "max_hold") {
		t.Errorf("原因应指明 max_hold，实得 %q", why)
	}
}

func TestLeaseActionable_IdleTTL(t *testing.T) {
	now := time.Now()
	// 空闲刚过 TTL ⇒ 归还
	l := ServiceLease{State: LeaseBorrowed, AcquiredAt: now.Add(-10 * time.Minute),
		LastActive: now.Add(-301 * time.Second), TTLS: 300, MaxHoldS: 1800}
	if should, _ := LeaseActionable(l, now); !should {
		t.Fatal("空闲超过 TTL 应判为需归还")
	}
	// 刚活动过（last_activity 新）⇒ 不归还（有在飞请求时调用方刷新它）
	l2 := ServiceLease{State: LeaseBorrowed, AcquiredAt: now.Add(-10 * time.Minute),
		LastActive: now.Add(-5 * time.Second), TTLS: 300, MaxHoldS: 1800}
	if should, why := LeaseActionable(l2, now); should {
		t.Fatalf("刚活动过不该归还，实得 %q", why)
	}
}

func TestLeaseActionable_RestoreFailedAlwaysRetried(t *testing.T) {
	now := time.Now()
	l := ServiceLease{State: LeaseRestoreFailed, AcquiredAt: now, LastActive: now,
		TTLS: 300, MaxHoldS: 1800}
	should, why := LeaseActionable(l, now)
	if !should {
		t.Fatal("restore_failed 必须无条件重试归还（绝不静默丢）")
	}
	if why == "" {
		t.Error("应给出原因")
	}
}

func TestLeaseActionable_NonBorrowedIgnored(t *testing.T) {
	now := time.Now()
	l := ServiceLease{State: LeaseRestored, AcquiredAt: now.Add(-time.Hour),
		LastActive: now.Add(-time.Hour), TTLS: 1, MaxHoldS: 1}
	if should, _ := LeaseActionable(l, now); should {
		t.Fatal("已归还的租约不该再动作")
	}
}

// ── 脱敏：白名单 + 密钥形态 ─────────────────────────────────────────────────

func TestFilterEnvForLease_WhitelistAndRedaction(t *testing.T) {
	env := []string{
		"PATH=/usr/bin:/bin",
		"LD_LIBRARY_PATH=/home/g01/ds4-pr670/rocm-libs",
		"HSA_OVERRIDE_GFX_VERSION=11.0.0",
		"ROCM_PATH=/opt/rocm",
		"HOME=/home/g01",
		// 非白名单 ⇒ 一律不存
		"FOO=bar",
		"ZERG_MODEL_TTL_S=300",
		// 敏感名但**不在白名单** ⇒ 按设计整体丢弃（比"落盘再脱敏"更安全）
		"ZERG_TOKEN=abc",
		"MY_PASSWORD=hunter2",
		"PATH_EXTRA=aB3xK9mQ2pL7vN4tR8wY1zC6fH0jS5dG",
		// 白名单前缀(HSA_) + 值像密钥 ⇒ 这才是脱敏的真实触发条件
		"HSA_EXTRA_FLAG=aB3xK9mQ2pL7vN4tR8wY1zC6fH0jS5dG",
	}
	got := FilterEnvForLease(env)

	// 白名单之外的键——哪怕名字像密钥——根本不落盘（设计如此，比"落盘再脱敏"更安全）。
	for _, k := range []string{"FOO", "ZERG_MODEL_TTL_S", "ZERG_TOKEN", "MY_PASSWORD", "PATH_EXTRA"} {
		if _, exists := got[k]; exists {
			t.Errorf("非白名单键 %s 不该落进租约（应整体丢弃），实得 %q", k, got[k])
		}
	}
	// 白名单键但值是长随机串 ⇒ 必须脱敏
	if got["HSA_EXTRA_FLAG"] != "[REDACTED]" {
		t.Errorf("白名单键的值像密钥 ⇒ 必须脱敏，实得 %q", got["HSA_EXTRA_FLAG"])
	}
	if got["PATH"] != "/usr/bin:/bin" {
		t.Errorf("PATH 应原样保留，实得 %q", got["PATH"])
	}
	if got["LD_LIBRARY_PATH"] != "/home/g01/ds4-pr670/rocm-libs" {
		t.Errorf("LD_LIBRARY_PATH 应原样保留，实得 %q", got["LD_LIBRARY_PATH"])
	}
	if got["HSA_OVERRIDE_GFX_VERSION"] != "11.0.0" {
		t.Errorf("HSA_ 前缀应放行，实得 %q", got["HSA_OVERRIDE_GFX_VERSION"])
	}
	if _, exists := got["PATH_EXTRA"]; exists {
		t.Error("PATH_EXTRA 不在白名单（长随机值那条只验证 secretLooking，不进白名单）")
	}
}

func TestSecretLooking(t *testing.T) {
	cases := []struct {
		key, val string
		want     bool
	}{
		{"PATH", "/usr/bin:/bin", false},
		{"LD_LIBRARY_PATH", "/home/g01/rocm-libs", false},
		{"ZERG_TOKEN", "abc", true},                     // 键名命中
		{"X", "aB3xK9mQ2pL7vN4tR8wY1zC6fH0jS5dG", true}, // 长随机串
		{"X", "short", false},
		{"X", "a string with spaces that is long enough to exceed thirty-two", false}, // 有空格
		{"X", "", false},
	}
	for _, c := range cases {
		if got := secretLooking(c.key, c.val); got != c.want {
			t.Errorf("secretLooking(%q,%q)=%v，期望 %v", c.key, c.val, got, c.want)
		}
	}
}

func TestFilterEnvForLease_IgnoresMalformed(t *testing.T) {
	got := FilterEnvForLease([]string{"NOEQUALS", "=novalue", "", "PATH=/bin"})
	if len(got) != 1 || got["PATH"] != "/bin" {
		t.Fatalf("畸形条目应被忽略，实得 %+v", got)
	}
}
