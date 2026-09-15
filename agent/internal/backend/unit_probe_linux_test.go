//go:build linux

// unit_probe_linux_test.go —— 真机用例（只在 Linux 上编译/运行）：探针与启动 GC 对**真 systemd**
// 的行为（2026-09-15 第一枚卵真机实测缺陷 9 / 缺陷 10）。
//
// 为什么必须有这一份：单测里的 systemd 输出是我手写的，可能"照着我的解析器写"；
// 这一份把**真 systemd 的输出**喂给同一段解析与判定代码，并让 GC 真去停真单元。
//
// 运行门槛（**默认 skip**，绝不会在普通回归里动任何单元）：
//
//	ZERG_REAL_SYSTEMD_TEST=1 go test ./internal/backend/ -run TestRealSystemd -v
//
// 安全边界（写死）：只创建/清理**本用例自己造的** `zerg-*` 瞬态单元，且都在 llm.slice 里；
// 单元里的进程是 `/bin/false` 与 `/bin/sleep`（不是引擎，不碰 GPU、不占 GTT、不碰权重）；
// 不碰用户的手工服务（它们根本不是 systemd 单元）。
package backend

import (
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/hatch"
)

func requireRealSystemd(t *testing.T) {
	t.Helper()
	if os.Getenv("ZERG_REAL_SYSTEMD_TEST") != "1" {
		t.Skip("真机用例：需显式 ZERG_REAL_SYSTEMD_TEST=1（它会在 llm.slice 里起/停真单元）")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skipf("本机没有 systemctl：%v", err)
	}
}

// startTransientUnit 在 llm.slice 里起一个瞬态单元（--wait 不设：起完就返回）。
func startTransientUnit(t *testing.T, unit string, argv ...string) {
	t.Helper()
	args := append([]string{"--user", "--unit=" + unit, "--slice=llm.slice",
		"--property=Type=exec", "--property=KillMode=control-group", "--"}, argv...)
	out, err := exec.Command("systemd-run", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("起瞬态单元 %s 失败：%v：%s", unit, err, strings.TrimSpace(string(out)))
	}
	t.Cleanup(func() { exec.Command("systemctl", "--user", "stop", unit).Run() })
}

func mustFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return p
}

// 真机①：单元秒死（/bin/false）⇒ 探针必须读成「已死」并给出真实归因；
// 单元活着（/bin/sleep）⇒ 不许误判成死。
func TestRealSystemd_UnitProbeReadsRealStates(t *testing.T) {
	requireRealSystemd(t)

	deadUnit := "zerg-realprobe-dead"
	startTransientUnit(t, deadUnit, "/bin/false")
	// 等 systemd 把它收到终态（真机：几毫秒到几百毫秒）
	var deadST unitState
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st, err := probeUnitState(deadUnit)
		if err != nil {
			t.Fatalf("探真单元 %s 失败：%v", deadUnit, err)
		}
		deadST = st
		if d, _ := st.dead(); d {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if d, why := deadST.dead(); !d {
		t.Fatalf("单元 %s 已秒死，探针必须读成已死；实读 %+v", deadUnit, deadST)
	} else {
		t.Logf("真机① 死单元状态（真 systemd 原文）：%+v 判定理由=%s", deadST, why)
	}

	aliveUnit := "zerg-realprobe-alive"
	startTransientUnit(t, aliveUnit, "/bin/sleep", "60")
	if st, err := probeUnitState(aliveUnit); err != nil {
		t.Fatalf("探真单元 %s 失败：%v", aliveUnit, err)
	} else if d, why := st.dead(); d {
		t.Fatalf("活单元不许被判死（why=%s st=%+v）", why, st)
	} else {
		t.Logf("真机① 活单元状态（真 systemd 原文）：%+v", st)
	}
}

// 真机②：孵化路径的就绪等待在真机上「单元秒死 ⇒ 秒级明确报错」（真机缺陷 9 的正面证据）。
func TestRealSystemd_WaitForReadyFailsFastOnDeadUnit(t *testing.T) {
	requireRealSystemd(t)

	unit := "zerg-realprobe-wait"
	startTransientUnit(t, unit, "/bin/false")

	m := newEvictTestManager(1, map[string]*subproc{})
	sp := &subproc{model: "real-dead", port: mustFreePort(t), state: StateLoading, Unit: unit}

	start := time.Now()
	err := m.waitForReady(sp)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("单元已死必须报错")
	}
	if elapsed > 20*time.Second {
		t.Fatalf("真机上单元秒死应秒级报错，实测 %s（真机缺陷 9 的「硬等 120s」）", elapsed)
	}
	if !strings.Contains(err.Error(), "孵化单元已退出") {
		t.Fatalf("错误信息应说清「孵化单元已退出」，实得 %v", err)
	}
	t.Logf("真机② /load 等价路径用时 %s，报错=%v", elapsed, err)
}

// 真机③：启动 GC 真停得掉 llm.slice 里的遗留卵，且不碰别的单元（真机缺陷 10 的正面证据）。
func TestRealSystemd_GCSweepsLeftoverEggs(t *testing.T) {
	requireRealSystemd(t)

	leftover := "zerg-realgc-leftover"
	startTransientUnit(t, leftover, "/bin/sleep", "120")
	// 造一个「失败态」的遗留卵（reset-failed 的落点）：它自己就死了
	failedEgg := "zerg-realgc-failed"
	startTransientUnit(t, failedEgg, "/bin/false")
	time.Sleep(700 * time.Millisecond)

	stopped, stuck, err := gcLeftoverEggs(nil)
	if err != nil {
		t.Fatalf("真机 GC 读清单失败：%v", err)
	}
	t.Logf("真机③ GC 清掉=%v 清不掉=%v", stopped, stuck)
	if len(stuck) != 0 {
		t.Fatalf("不该有清不掉的：%v", stuck)
	}
	for _, want := range []string{leftover + ".service", failedEgg + ".service"} {
		found := false
		for _, s := range stopped {
			if s == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("GC 应清掉 %s，实得 %v", want, stopped)
		}
		if st, err := probeUnitState(want); err == nil {
			if d, _ := st.dead(); !d {
				t.Fatalf("GC 报「已清掉」但 %s 仍在跑：%+v", want, st)
			}
		}
	}

	// 范围复核：llm.slice 里不该再有 zerg-* 单元（真机复核，不凭命令退出码）。
	out, _ := exec.Command("systemctl", "--user", "list-units", "--slice="+hatch.SliceName,
		"--all", "--no-legend", "--plain", "zerg-*").CombinedOutput()
	if names := parseEggUnitNames(string(out)); len(names) != 0 {
		t.Fatalf("GC 之后 llm.slice 里还剩卵单元：%v（原文=%q）", names, strings.TrimSpace(string(out)))
	}
}
