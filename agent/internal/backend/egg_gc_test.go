// egg_gc_test.go —— 启动 GC：任何启动先把遗留卵清干净（2026-09-15 第一枚卵真机实测缺陷 10）。
//
// 缺陷原文（报告 §12 缺陷 10 / §9 的 E1）：卵落 llm.slice 独立单元、刻意不被子端 KillMode 带走
// ⇒ 子端重启后卵还活着（GTT 仍占着 96 GB），而新实例账本是空的（`/eggs=[]`、`/status=idle`）
// ⇒ 孤儿卵没人认领。设计 §6.5 已拍「任何启动先把遗留卵清干净」，此前未实现。
//
// 本文件钉住六件事：
//
//	① 单元名解析：只认 `zerg-*.service`（用户自己的 x3-agent.service / k2.service 一律跳过）；
//	② 归属铁律：**逐个核**单元自己的 Slice / ControlGroup —— 不是 llm.slice 就不许动手
//	   （真机实测：systemd 259 的 `list-units` 压根不认识 `--slice=`，所以这条不能靠过滤参数）；
//	③ 归属读不到 ⇒ 不许动手（fail-closed：宁留孤儿要人接手，不误停别人的单元）；
//	④ 幂等：单元本来就不存在（stop 报 not loaded）算成功，不算清不掉；
//	⑤ 复核：stop 返回成功但单元仍在跑 ⇒ 进 stuck（§6.9：命令成功 ≠ 清干净了）；
//	⑥ 接线：孵化开关开的子端**启动时**先清一遍；开关关则不碰任何单元（离线路径逐字不变）。
package backend

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// gcRecorder 记录 GC 期间发出的每条命令，并按预置结果回应（不碰真 systemd）。
//
// 它按真实语义回话：`show` 拿的是某个单元的状态；被 `stop` 过的单元之后 `show` 就变 inactive。
type gcRecorder struct {
	calls    [][]string
	listOut  string
	listErr  error
	stopErr  error
	stopOut  string
	resetErr error
	// showByUnit 每个单元 `systemctl show` 的输出；没列到的单元用 defaultShow（空 ⇒ 报「探不到」）。
	showByUnit  map[string]string
	defaultShow string
	stopped     map[string]bool // stop 过的单元（之后 show 一律 inactive/dead）
}

func (g *gcRecorder) run(_ time.Duration, name string, args ...string) (string, error) {
	g.calls = append(g.calls, append([]string{name}, args...))
	verb := ""
	if len(args) > 1 {
		verb = args[1] // ["--user", <verb>, ...]
	}
	if g.stopped == nil {
		g.stopped = map[string]bool{}
	}
	switch {
	case name == "systemctl" && verb == "list-units":
		return g.listOut, g.listErr
	case name == "systemctl" && verb == "stop":
		if len(args) > 2 {
			g.stopped[args[2]] = true
		}
		return g.stopOut, g.stopErr
	case name == "systemctl" && verb == "reset-failed":
		return "", g.resetErr
	case name == "systemctl" && verb == "show":
		if len(args) < 3 {
			return "", errors.New("show 缺单元名")
		}
		unit := args[2]
		if g.stopped[unit] {
			return sliceStateShow(unit, "llm.slice", "inactive", "dead", "success", 0), nil
		}
		if out, ok := g.showByUnit[unit]; ok {
			return out, nil
		}
		if g.defaultShow == "" {
			return "", errors.New("show 未预置：" + unit)
		}
		return g.defaultShow, nil
	}
	return "", errors.New("未预置的命令：" + strings.Join(append([]string{name}, args...), " "))
}

// sliceStateShow 造一段真实形态的 `systemctl --user show` 输出（字段与 X3 实测一致）。
func sliceStateShow(unit, slice, active, sub, result string, execStatus int) string {
	line := fmt.Sprintf("ActiveState=%s\nSubState=%s\nResult=%s\nExecMainStatus=%d\n", active, sub, result, execStatus)
	if slice == "" {
		return line
	}
	return line + fmt.Sprintf("Slice=%s\nControlGroup=/user.slice/user-1000.slice/user@1000.service/%s/%s\n",
		slice, slice, unit)
}

func (g *gcRecorder) unitsTouched(verb string) []string {
	var out []string
	for _, c := range g.calls {
		if len(c) > 2 && c[2] == verb {
			out = append(out, c[len(c)-1])
		}
	}
	return out
}

// ① 解析：只认卵单元名，`●` 前缀（failed 单元）要能剥掉。
func TestParseEggUnitNames(t *testing.T) {
	out := "● zerg-dead-egg.service loaded failed failed 卵 dead-egg\n" +
		"zerg-qwen38-27b-egg.service loaded active running 卵 qwen38-27b-egg\n" +
		"x3-agent.service loaded active running systemd user agent\n" +
		"k2.service loaded active running 用户手工服务\n" +
		"zerg.service loaded active running 边界：段名为空\n" +
		"\n" +
		"   \n"
	got := parseEggUnitNames(out)
	want := []string{"zerg-dead-egg.service", "zerg-qwen38-27b-egg.service"}
	if len(got) != len(want) {
		t.Fatalf("应只解析出 %v，实得 %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("应只解析出 %v，实得 %v", want, got)
		}
	}
	if n := parseEggUnitNames(""); len(n) != 0 {
		t.Fatalf("空输入应得空结果，实得 %v", n)
	}
	if n := parseEggUnitNames("zerg-a.service loaded active running\nzerg-a.service loaded active running\n"); len(n) != 1 {
		t.Fatalf("重复行应去重，实得 %v", n)
	}
}

// ②④ 范围与幂等：只清归属 llm.slice 的 zerg-* 卵单元；混进清单的别的单元一律不许动作。
func TestGCLeftoverEggs_OnlyTouchesEggUnits(t *testing.T) {
	g := &gcRecorder{
		listOut: "● zerg-dead-egg.service loaded failed failed 卵 dead-egg\n" +
			"zerg-qwen38-27b-egg.service loaded active running 卵 qwen38-27b-egg\n" +
			// 第三条只给 ControlGroup（Slice 属性读不到）——归属两道判据的交叉校验
			"zerg-cgroup-only.service loaded active running 只读得到 cgroup 的卵\n" +
			// 下面两条是「绝不许碰」的对照：用户手工服务与子端自己（万一被列进来）
			"x3-agent.service loaded active running agent\n" +
			"k2.service loaded active running 用户服务\n",
		showByUnit: map[string]string{
			"zerg-dead-egg.service": sliceStateShow("zerg-dead-egg.service", "llm.slice", "failed", "failed", "exit-code", 127),
			"zerg-qwen38-27b-egg.service": sliceStateShow("zerg-qwen38-27b-egg.service", "llm.slice",
				"active", "running", "success", 0),
			"zerg-cgroup-only.service": "ActiveState=active\nSubState=running\nResult=success\nExecMainStatus=0\n" +
				"ControlGroup=/user.slice/user-1000.slice/user@1000.service/llm.slice/zerg-cgroup-only.service\n",
		},
	}

	stopped, stuck, err := gcLeftoverEggs(g.run)
	if err != nil {
		t.Fatalf("不该报错：%v", err)
	}
	if len(stuck) != 0 {
		t.Fatalf("不该有清不掉的：%v", stuck)
	}
	want := []string{"zerg-dead-egg.service", "zerg-qwen38-27b-egg.service", "zerg-cgroup-only.service"}
	if len(stopped) != len(want) {
		t.Fatalf("应清掉 %v，实得 %v", want, stopped)
	}
	for i := range want {
		if stopped[i] != want[i] {
			t.Fatalf("应清掉 %v，实得 %v", want, stopped)
		}
	}
	// 范围铁律：非卵单元一次都不许出现在 stop / reset-failed 里
	for _, u := range append(g.unitsTouched("stop"), g.unitsTouched("reset-failed")...) {
		if !strings.HasPrefix(u, "zerg-") {
			t.Fatalf("越界：对非卵单元 %s 动了手", u)
		}
	}
	// 清单命令形态：按名字列（**不得**依赖 --slice=，systemd 259 不认识它，真机上会让 GC 静默失效）
	var sawList bool
	for _, c := range g.calls {
		if len(c) > 2 && c[2] == "list-units" {
			sawList = true
			joined := strings.Join(c, " ")
			if !strings.Contains(joined, "zerg-*") {
				t.Fatalf("清单命令必须限定 zerg-*，实得 %v", c)
			}
			if strings.Contains(joined, "--slice=") {
				t.Fatalf("清单命令不得依赖 --slice=（systemd 259 上直接报「未识别的选项」）: %v", c)
			}
		}
	}
	if !sawList {
		t.Fatal("没有发清单命令")
	}
	// 归属判据必须真读过：`show` 要带 -p Slice -p ControlGroup
	var sawSliceProp, sawCgroupProp bool
	for _, c := range g.calls {
		joined := strings.Join(c, " ")
		if strings.Contains(joined, "-p Slice") {
			sawSliceProp = true
		}
		if strings.Contains(joined, "-p ControlGroup") {
			sawCgroupProp = true
		}
	}
	if !sawSliceProp || !sawCgroupProp {
		t.Fatalf("必须实读单元归属（Slice=%v ControlGroup=%v）——否则「只管自己的卵」无从证", sawSliceProp, sawCgroupProp)
	}
	// reset-failed 必须每枚都做过（失败态单元不 reset 会一直挂着）
	if got := g.unitsTouched("reset-failed"); len(got) != len(want) {
		t.Fatalf("每枚卵都该 reset-failed，实得 %v", got)
	}
}

// ② 归属不对（名字像卵但不在 llm.slice）⇒ 一枚都不许动，也不进 stuck（跳过要留痕）。
func TestGCLeftoverEggs_SkipsUnitNotInOurSlice(t *testing.T) {
	g := &gcRecorder{
		listOut: "zerg-alien.service loaded active running 别处的同名单元\n" +
			"zerg-ours.service loaded active running 本子端的卵\n",
		showByUnit: map[string]string{
			"zerg-alien.service": sliceStateShow("zerg-alien.service", "other.slice", "active", "running", "success", 0),
			"zerg-ours.service":  sliceStateShow("zerg-ours.service", "llm.slice", "active", "running", "success", 0),
		},
	}
	stopped, stuck, err := gcLeftoverEggs(g.run)
	if err != nil {
		t.Fatalf("不该报错：%v", err)
	}
	if len(stuck) != 0 {
		t.Fatalf("不属于本切片是「跳过」，不是「清不掉」：%v", stuck)
	}
	if len(stopped) != 1 || stopped[0] != "zerg-ours.service" {
		t.Fatalf("只该清本子端的卵，实得 %v", stopped)
	}
	for _, u := range g.unitsTouched("stop") {
		if u == "zerg-alien.service" {
			t.Fatal("越界：停了不属于 llm.slice 的同名单元")
		}
	}
}

// ③ 归属读不到 ⇒ 不许动手（fail-closed），并进 stuck 留痕。
func TestGCLeftoverEggs_UnreadableOwnershipNotTouched(t *testing.T) {
	g := &gcRecorder{
		listOut:     "zerg-mystery.service loaded active running 归属读不到的单元\n",
		defaultShow: "", // 空 ⇒ show 返回错误
	}
	stopped, stuck, err := gcLeftoverEggs(g.run)
	if err != nil {
		t.Fatalf("清单读得到就不该整体报错：%v", err)
	}
	if len(stopped) != 0 {
		t.Fatalf("归属读不到不许当已清掉：%v", stopped)
	}
	if len(stuck) != 1 || !strings.Contains(stuck[0], "zerg-mystery.service") {
		t.Fatalf("归属读不到的必须进 stuck 留痕，实得 %v", stuck)
	}
	if got := g.unitsTouched("stop"); len(got) != 0 {
		t.Fatalf("归属读不到时一次都不许停：%v", got)
	}
}

// ④ 幂等：单元本来就不存在（not loaded）⇒ 算清干净，不算清不掉。
func TestGCLeftoverEggs_MissingUnitIsIdempotentSuccess(t *testing.T) {
	g := &gcRecorder{
		listOut: "zerg-gone.service loaded not-found not-found 已消失\n",
		stopOut: "Failed to stop zerg-gone.service: Unit zerg-gone.service not loaded.\n",
		stopErr: errors.New("exit status 5"),
		showByUnit: map[string]string{
			"zerg-gone.service": sliceStateShow("zerg-gone.service", "llm.slice", "inactive", "dead", "success", 0),
		},
	}
	stopped, stuck, err := gcLeftoverEggs(g.run)
	if err != nil {
		t.Fatalf("不该报错：%v", err)
	}
	if len(stuck) != 0 {
		t.Fatalf("「本来就不存在」是幂等成功，不该进 stuck：%v", stuck)
	}
	if len(stopped) != 1 || stopped[0] != "zerg-gone.service" {
		t.Fatalf("应记一枚已清掉，实得 %v", stopped)
	}
}

// ⑤ stop 真失败（不是幂等的那种）⇒ 必须进 stuck 并写明原因，且不许记成已清掉。
func TestGCLeftoverEggs_StopFailureGoesToStuck(t *testing.T) {
	g := &gcRecorder{
		listOut: "zerg-stubborn.service loaded active running 收不掉的卵\n",
		stopOut: "Failed to stop zerg-stubborn.service: Access denied\n",
		stopErr: errors.New("exit status 1"),
		showByUnit: map[string]string{
			"zerg-stubborn.service": sliceStateShow("zerg-stubborn.service", "llm.slice", "active", "running", "success", 0),
		},
	}
	stopped, stuck, err := gcLeftoverEggs(g.run)
	if err != nil {
		t.Fatalf("不该报错：%v", err)
	}
	if len(stopped) != 0 {
		t.Fatalf("stop 失败的不许记成已清掉：%v", stopped)
	}
	if len(stuck) != 1 || !strings.Contains(stuck[0], "zerg-stubborn.service") {
		t.Fatalf("停不掉的必须进 stuck 并写明原因，实得 %v", stuck)
	}
}

// ⑤b 复核语义单独钉住：stop 返回成功、但复读仍是 active ⇒ stuck（不是 stopped）。
func TestGCLeftoverEggs_VerifyAfterStopStillRunning(t *testing.T) {
	g := &alwaysActiveRecorder{}
	stopped, stuck, err := gcLeftoverEggs(g.run)
	if err != nil {
		t.Fatalf("不该报错：%v", err)
	}
	if len(stopped) != 0 || len(stuck) != 1 {
		t.Fatalf("stop 成功但复核仍在跑 ⇒ 只许进 stuck：stopped=%v stuck=%v", stopped, stuck)
	}
	if !strings.Contains(stuck[0], "停完仍在跑") {
		t.Fatalf("stuck 理由要写清「停完仍在跑」，实得 %q", stuck[0])
	}
}

// alwaysActiveRecorder：stop 永远「成功」，show 永远 active（模拟「命令成功 ≠ 真收干净」）。
type alwaysActiveRecorder struct{}

func (alwaysActiveRecorder) run(_ time.Duration, name string, args ...string) (string, error) {
	verb := ""
	if len(args) > 1 {
		verb = args[1]
	}
	switch {
	case name == "systemctl" && verb == "list-units":
		return "zerg-zombie.service loaded active running 停不掉的卵\n", nil
	case name == "systemctl" && verb == "stop":
		return "", nil
	case name == "systemctl" && verb == "reset-failed":
		return "", nil
	case name == "systemctl" && verb == "show":
		return sliceStateShow(args[2], "llm.slice", "active", "running", "success", 0), nil
	}
	return "", errors.New("未预置：" + strings.Join(append([]string{name}, args...), " "))
}

// ⑥b 列清单都读不到 ⇒ 报错（调用方如实留痕，绝不静默当「没有遗留卵」）。
func TestGCLeftoverEggs_ListUnreadableIsError(t *testing.T) {
	g := &gcRecorder{listOut: "nope", listErr: errors.New("no systemctl")}
	stopped, stuck, err := gcLeftoverEggs(g.run)
	if err == nil {
		t.Fatal("清单读不到必须报错（不许静默当成没有遗留卵）")
	}
	if len(stopped) != 0 || len(stuck) != 0 {
		t.Fatalf("什么都不该做：stopped=%v stuck=%v", stopped, stuck)
	}
	if verbs := g.unitsTouched("stop"); len(verbs) != 0 {
		t.Fatalf("读不到清单时不许 stop 任何东西，实得 %v", verbs)
	}
}

// ⑥ 接线：孵化开关开 ⇒ 启动先清一遍；开关关 ⇒ 一次都不碰（离线路径逐字不变）。
func TestStartupGC_WiredThroughNewManager(t *testing.T) {
	old := startupEggGC
	t.Cleanup(func() { startupEggGC = old })

	var called int32
	startupEggGC = func() ([]string, []string, error) {
		atomic.AddInt32(&called, 1)
		return []string{"zerg-leftover.service"}, []string{"zerg-stuck.service（停完仍在跑）"}, nil
	}

	t.Setenv(EnvHatch, "")
	NewManager(nil, "x3")
	if got := atomic.LoadInt32(&called); got != 0 {
		t.Fatalf("开关关时启动不许碰任何单元（离线路径逐字不变），却调了 %d 次", got)
	}

	t.Setenv(EnvHatch, "1")
	NewManager(nil, "x3")
	if got := atomic.LoadInt32(&called); got != 1 {
		t.Fatalf("开关开时**启动必须先清遗留卵**（设计 §6.5），调了 %d 次", got)
	}
}
