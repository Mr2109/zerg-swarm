// unit_probe_test.go —— 卵单元状态探针 + 就绪等待的「单元已死」判据
// （2026-09-15 第一枚卵真机实测缺陷 9）。
//
// 缺陷原文（报告 §12 缺陷 9）：孵化路径下 sp.proc == nil（引擎不是子端的子进程）⇒
// waitForReady 只能**干等满 120s**（真机：单元 status=127/1 秒死，`/load` 硬等 120.16s）。
// 修法：孵化路径增加**单元状态判据**（ActiveState/SubState/Result/ExecMainStatus）⇒ 秒级明确报错；
// **不许**把 120s 硬等改成更短的硬等（那只是把「慢」当「死」）。
//
// 本文件钉住五件事：
//
//	① 单元明确已死（failed/127）⇒ 秒级报错，理由里带够归因信息（含单元日志尾部）；
//	② 单元还活着（active）⇒ **不判死**，继续按健康检查等（真快照摆在面前也不许误杀）；
//	③ 探不到（非 Linux / 没 systemctl）⇒ **不下结论**，继续等（与既有行为一致）；
//	④ 120s 兜底仍是 120s（结构性断言，防「改成更短硬等」的偷懒修法）；
//	⑤ 解析口径：Key=Value / 过渡态 / 空值的判定逐条钉住。
package backend

import (
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// withUnitStateProbe 注入单元状态探针（开发机上没有 systemd 用户实例，真依赖只在 X3 上）。
func withUnitStateProbe(t *testing.T, fn func(string) (unitState, error)) {
	t.Helper()
	old := unitStateProbe
	unitStateProbe = fn
	t.Cleanup(func() { unitStateProbe = old })
}

// closedPortForTest 拿一个「刚才还是本进程的、现在没人听」的端口（健康检查必失败）。
func closedPortForTest(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return p
}

// ① 单元明确已死 ⇒ 秒级报错（真机那种 status=127 的形态），不许等满 120s。
func TestWaitForReady_UnitDead_FailsFast(t *testing.T) {
	withUnitStateProbe(t, func(unit string) (unitState, error) {
		if unit != "zerg-dead-egg" {
			t.Errorf("探针收到了意外的单元名 %q", unit)
		}
		return unitState{
			ActiveState:    "failed",
			SubState:       "failed",
			Result:         "exit-code",
			ExecMainStatus: 127,
			JournalTail:    "bwrap: setenv failed\nllama-server: error while loading shared libraries",
		}, nil
	})

	m := newEvictTestManager(1, map[string]*subproc{})
	sp := &subproc{model: "dead-egg", port: closedPortForTest(t), state: StateLoading, Unit: "zerg-dead-egg"}

	start := time.Now()
	err := m.waitForReady(sp)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("单元已死必须报错——把死单元当就绪是最坏的一种（对外声称可用）")
	}
	if elapsed > 15*time.Second {
		t.Fatalf("单元已死应秒级报错，实测 %s（真机那种「硬等 120s」又回来了）", elapsed)
	}
	msg := err.Error()
	for _, want := range []string{"孵化单元已退出", "ActiveState=failed", "Result=exit-code", "ExecMainStatus=127", "setenv failed"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("报错要能归因（缺 %q），实得 %q", want, msg)
		}
	}
}

// ② 单元还活着 ⇒ 不判死（继续等）。第三次读才转死 ⇒ 报错，且用时至少跨过一个轮询间隔。
func TestWaitForReady_UnitAlive_KeepsWaiting(t *testing.T) {
	var calls int32
	withUnitStateProbe(t, func(string) (unitState, error) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			return unitState{ActiveState: "active", SubState: "running", Result: "success"}, nil
		}
		return unitState{ActiveState: "inactive", SubState: "dead", Result: "exit-code", ExecMainStatus: 1}, nil
	})

	m := newEvictTestManager(1, map[string]*subproc{})
	sp := &subproc{model: "slow-egg", port: closedPortForTest(t), state: StateLoading, Unit: "zerg-slow-egg"}

	start := time.Now()
	err := m.waitForReady(sp)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("单元转为已死之后必须报错")
	}
	if !strings.Contains(err.Error(), "孵化单元已退出") {
		t.Fatalf("转死之后应报「孵化单元已退出」，实得 %q", err.Error())
	}
	if elapsed < 3*time.Second {
		t.Fatalf("前两次读数还是 active 就报死 = 误杀慢启动的卵（实测 %s）", elapsed)
	}
	if got := atomic.LoadInt32(&calls); got < 3 {
		t.Fatalf("应至少读三次单元状态（前两次活、第三次死），实得 %d 次", got)
	}
}

// ③ 探不到（非 Linux / 没 systemctl / 命令失败）⇒ 不下结论：既不说死，也不说好了。
func TestWaitForReady_ProbeUnreadable_DoesNotDeclareDead(t *testing.T) {
	var calls int32
	withUnitStateProbe(t, func(string) (unitState, error) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			return unitState{}, errors.New("本机没有 systemctl（非 Linux）")
		}
		return unitState{ActiveState: "failed", SubState: "failed", Result: "exit-code", ExecMainStatus: 127}, nil
	})

	m := newEvictTestManager(1, map[string]*subproc{})
	sp := &subproc{model: "probe-err-egg", port: closedPortForTest(t), state: StateLoading, Unit: "zerg-probe-err"}

	start := time.Now()
	err := m.waitForReady(sp)
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "孵化单元已退出") {
		t.Fatalf("探到死之后必须报错，实得 %v", err)
	}
	if elapsed < 3*time.Second {
		t.Fatalf("「探不到」不许当成「已经死了」（实测 %s 就返回）", elapsed)
	}
}

// ④ 结构性断言：120s 兜底仍是 120s —— 修法是「多一条判据」，不是「把硬等改短」。
func TestWaitForReady_TimeoutStill120s(t *testing.T) {
	src, err := readFileForAssertion("manager.go")
	if err != nil {
		t.Fatalf("读 manager.go 失败: %v", err)
	}
	code := codeOnly(src)
	if !strings.Contains(code, "const timeout = 120 * time.Second") {
		t.Fatal("waitForReady 的 120s 兜底被改动了：缺陷 9 只许**增加单元状态判据**，" +
			"不许把 120s 硬等改成更短的硬等（慢启动的正确卵会被误杀）")
	}
	if !strings.Contains(code, "unitStateProbe(sp.Unit)") {
		t.Fatal("waitForReady 里的单元状态判据不见了（孵化路径又会干等满 120s）")
	}
}

// ⑤ 解析与判定口径逐条钉住（纯函数）。
func TestUnitState_ParseAndDeadVerdict(t *testing.T) {
	raw := "ActiveState=failed\nSubState=failed\nResult=exit-code\nExecMainStatus=127\n" +
		"Slice=llm.slice\nControlGroup=/user.slice/user-1000.slice/user@1000.service/llm.slice/zerg-x.service\n"
	st := parseUnitState(raw)
	if st.ActiveState != "failed" || st.SubState != "failed" || st.Result != "exit-code" || st.ExecMainStatus != 127 {
		t.Fatalf("解析结果不对：%+v", st)
	}
	if st.Slice != "llm.slice" || !strings.Contains(st.ControlGroup, "/llm.slice/") {
		t.Fatalf("归属字段没解析出来：%+v", st)
	}
	if !st.inSlice("llm.slice") {
		t.Fatal("Slice=llm.slice 必须判为「归属 llm.slice」")
	}
	if dead, why := st.dead(); !dead || !strings.Contains(why, "ExecMainStatus=127") {
		t.Fatalf("failed/127 必须判死并给出理由，实得 dead=%v why=%q", dead, why)
	}

	// 归属判据：Slice 读不到时靠 cgroup 链兜底；两条都读不到 ⇒ **不算归属**（不许动手）。
	if !(unitState{ControlGroup: "/user.slice/user@1000.service/llm.slice/zerg-x.service"}).inSlice("llm.slice") {
		t.Fatal("Slice 属性读不到时应靠 ControlGroup 链兜底")
	}
	if (unitState{}).inSlice("llm.slice") {
		t.Fatal("归属两条都读不到时不许判为「归属 llm.slice」（fail-closed：不许对它动手）")
	}
	if (unitState{Slice: "other.slice"}).inSlice("llm.slice") {
		t.Fatal("别的切片不算归属")
	}
	if (unitState{Slice: "x"}).inSlice("") {
		t.Fatal("空切片名不算归属")
	}

	// 过渡态与空值**一律不判死**（否则会把正在启动的卵误杀）。
	for _, s := range []unitState{
		{ActiveState: "active", SubState: "running"},
		{ActiveState: "activating", SubState: "start-pre"},
		{ActiveState: "deactivating"},
		{ActiveState: "reloading"},
		{ActiveState: ""},
		{ActiveState: "plugged"},
	} {
		if dead, why := s.dead(); dead {
			t.Fatalf("%+v 不该判死（why=%q）", s, why)
		}
	}
	if dead, _ := (unitState{ActiveState: "inactive"}).dead(); !dead {
		t.Fatal("inactive 必须判死（引擎已不在运行）")
	}
}

// 探针实现：命令失败且读不到 ActiveState ⇒ 报错（调用方不下结论）；读到死态才去抓日志尾巴。
func TestProbeUnitStateWith_InjectedRunner(t *testing.T) {
	t.Run("读不到 ActiveState ⇒ 探不到", func(t *testing.T) {
		run := func(timeout time.Duration, name string, args ...string) (string, error) {
			return "Unit zerg-x.service could not be found.\n", errors.New("exit status 1")
		}
		if _, err := probeUnitStateWith(run, "zerg-x"); err == nil {
			t.Fatal("命令失败且没有属性行时必须报「探不到」")
		}
	})

	t.Run("读到死态 ⇒ 附日志尾部", func(t *testing.T) {
		var journalQueried bool
		run := func(timeout time.Duration, name string, args ...string) (string, error) {
			if name == "journalctl" {
				journalQueried = true
				return "bwrap: setenv failed\n", nil
			}
			return "ActiveState=failed\nSubState=failed\nResult=exit-code\nExecMainStatus=127\n", nil
		}
		st, err := probeUnitStateWith(run, "zerg-dead")
		if err != nil {
			t.Fatalf("探针不该报错：%v", err)
		}
		if !journalQueried {
			t.Fatal("已判死时应去抓一次单元日志尾巴（死因几乎总在那行输出里）")
		}
		if !strings.Contains(st.JournalTail, "setenv failed") {
			t.Fatalf("日志尾部没带上：%q", st.JournalTail)
		}
	})

	t.Run("活着不抓日志（少一次命令）", func(t *testing.T) {
		run := func(timeout time.Duration, name string, args ...string) (string, error) {
			if name == "journalctl" {
				t.Error("单元还活着不该去抓日志尾巴")
			}
			return "ActiveState=active\nSubState=running\nResult=success\nExecMainStatus=0\n", nil
		}
		st, err := probeUnitStateWith(run, "zerg-alive")
		if err != nil || st.JournalTail != "" {
			t.Fatalf("活单元：err=%v tail=%q", err, st.JournalTail)
		}
	})

	t.Run("缺单元名 ⇒ 报错", func(t *testing.T) {
		if _, err := probeUnitStateWith(nil, "  "); err == nil {
			t.Fatal("缺单元名必须报错（不许拿空单元去问 systemd）")
		}
	})
}
