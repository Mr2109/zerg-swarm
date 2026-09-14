// baseline_reuse_guard_test.go —— M10 的守卫回归（Mr2109 2026-09-14 明确要求"逐条加守卫测试"）。
//
// 三条不变式：
//
//	① 外部复用项（手工起的基线服务）**绝不进驱逐/停服路径**——哪怕它带着真进程句柄；
//	② 外部项的内存占用必须沿用基线实测值，否则内存核算会凭空漏掉一大块；
//	③ 复用项登记的端口会被当作"本端已占"，于是**借用永远看不到它**（结构性守卫）。
package backend

import (
	"os/exec"
	"syscall"
	"testing"
)

// ① 用**真进程**验证：驱逐必须拒绝动手，且驻留表里不删它。
// 这条是 M10 最要紧的：
// 杀错一次 = 停掉 Mr2109 手工起的服务（红线②）。
func TestEvict_ExternalEntryIsNeverStopped(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("无法起测试进程，跳过：%v", err)
	}
	pid := cmd.Process.Pid
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()

	m := newEvictTestManager(1, map[string]*subproc{
		"k2-baseline": {
			proc: cmd, port: 9001, model: "k2-baseline", state: StateReady,
			external: true, baselineGB: 23.98,
		},
	})

	got := m.evictSubprocLocked("k2-baseline", m.procs["k2-baseline"], "内存腾退(只卸够)")

	if got != 0 {
		t.Fatalf("外部项不该释放任何内存（因为它没被停止），实得 %.2f GB", got)
	}
	if _, ok := m.procs["k2-baseline"]; !ok {
		t.Fatal("外部项不得从驻留表里被删除")
	}
	// 进程必须还活着（信号 0 = 只做存在性检查，不真发信号）
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("外部项的进程被杀了（pid=%d）：%v", pid, err)
	}
}

// ② 外部项没有进程句柄，内存核算必须沿用登记时记下的基线实测值。
func TestResidentMem_ExternalUsesBaselineGB(t *testing.T) {
	sp := &subproc{external: true, baselineGB: 23.98}
	if got := residentMemGbOf(sp); got != 23.98 {
		t.Fatalf("外部项应沿用基线实测占用 23.98，实得 %.2f", got)
	}
	// 对照组：非外部项且没有句柄 ⇒ 0（缺席不编造，绝不拿别的数字顶替）
	if got := residentMemGbOf(&subproc{}); got != 0 {
		t.Fatalf("无句柄的普通项应返回 0，实得 %.2f", got)
	}
	// 对照组的对照组：nil 不 panic
	if got := residentMemGbOf(nil); got != 0 {
		t.Fatalf("nil 应返回 0，实得 %.2f", got)
	}
}

// ③ 结构性守卫：复用项登记的端口会被 baselineServicesLocked 当成"本端已占"排除，
// 于是借用的候选里根本不会出现它 —— 不依赖调用方"记得跳过"。
// （用 59990 这种必然空闲的高位端口，绝不碰真机 9000/9001。）
func TestBaselineReusedPortInvisibleToBorrow(t *testing.T) {
	t.Setenv("ZERG_BASELINE_PORTS", "59990")
	m := newEvictTestManager(1, map[string]*subproc{
		"k2-baseline": {port: 59990, model: "k2-baseline", state: StateReady, external: true, baselineGB: 23.98},
	})

	for _, s := range m.baselineServicesLocked() {
		if s.Port == 59990 {
			t.Fatal("复用项占用的端口不得出现在基线列表里（否则借用会把它当可停目标）")
		}
	}
}
