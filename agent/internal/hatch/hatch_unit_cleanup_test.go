// hatch_unit_cleanup_test.go —— 2026-09-19 ③：OOM 后 transient unit 残留把孵化堵死。
//
// 病灶（真机逐字）：`systemd-run --user --unit=<unit>` **无 `--collect`** ⇒ 单元退出/失败后仍留在
// systemd 里占名字；卵被 OOM 杀掉（`Active: failed (Result: oom-kill)`）之后，下一次同名 `/load`
// 直接 HTTP 500：
//
//	Failed to start transient service unit: Unit zerg-deepseek-v4-flash.service was already
//	loaded or has a fragment file.
//
// 手工 `systemctl --user stop` + `reset-failed` 之后同一条 load 立刻 200（因果成立）。
// 两半一起改（缺一不可）：① argv 加 `--collect`；② 孵化前对同名单元定点清理（stop + reset-failed）。
//
// 本文件逐条钉住（**不碰真 systemd**：假 runner 记录 argv，判据精确到调用顺序）：
//
//	① argv 里有 `--collect`（且在 `--` 之前的选项区）；
//	② 孵化前置真的先清理同名单元：is-active → stop → reset-failed，按此序、按此 argv；
//	③ 单元不存在（is-active 空）⇒ **不 stop**，只 reset-failed（幂等）；
//	④ 清理失败（非 benign）⇒ 照常往下走（孵化不被清理阻塞）+ 动作序列如实标出（不假装清过）；
//	⑤ run == nil（dry-run：只想看 argv 的调用方）⇒ 一条命令都不发。
package hatch

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeUnitRunner 记录每一条命令（name + args），并按脚本回输出/错误。
type fakeUnitRunner struct {
	calls  []string
	out    func(name string, args []string) string
	errFor func(name string, args []string) error
}

func (f *fakeUnitRunner) run(_ time.Duration, name string, args ...string) (string, error) {
	f.calls = append(f.calls, strings.Join(append([]string{name}, args...), " "))
	if f.errFor != nil {
		if err := f.errFor(name, args); err != nil {
			if f.out != nil {
				return f.out(name, args), err
			}
			return "", err
		}
	}
	if f.out != nil {
		return f.out(name, args), nil
	}
	return "", nil
}

// ① argv 必须含 --collect（选项区，`--` 之前）。
func TestBuildSystemdRunArgv_HasCollect(t *testing.T) {
	unit := UnitName("example-moe-36b")
	argv, err := BuildSystemdRunArgv(goodSpec(), unit)
	if err != nil {
		t.Fatalf("构造 argv 失败: %v", err)
	}
	idx, sep := -1, -1
	for i, a := range argv {
		if a == "--collect" {
			idx = i
		}
		if a == "--" && sep < 0 {
			sep = i
		}
	}
	if idx < 0 {
		t.Fatalf("argv 缺 `--collect`（无它则单元退出/失败后留在 systemd 里占名字 ⇒ 同名卵再也孵不起来）：%v", argv)
	}
	if sep >= 0 && idx > sep {
		t.Fatalf("`--collect` 出现在 `--` 之后的命令区（那是给 bwrap/引擎的参数，systemd-run 认不到）：%v", argv)
	}
	// 同时钉住既有关键选项没被顺手挪掉（--unit / --slice / Type=exec / KillMode）
	joined := strings.Join(argv, " ")
	for _, must := range []string{"--unit=" + unit, "--slice=" + SliceName, "--property=Type=exec", "--property=KillMode=control-group"} {
		if !strings.Contains(joined, must) {
			t.Errorf("argv 少了既有选项 %q：%v", must, argv)
		}
	}
}

// ② 孵化前置：同名单元 failed ⇒ is-active → stop → reset-failed，逐字按此序。
func TestPrepareHatchUnit_CleansLeftoverFailedUnit(t *testing.T) {
	unit := UnitName("example-moe-36b")
	f := &fakeUnitRunner{out: func(name string, args []string) string {
		if name == "systemctl" && len(args) > 1 && args[1] == "is-active" {
			return "failed\n"
		}
		return ""
	}}
	argv, err := PrepareHatchUnit(unit, goodSpec(), f.run)
	if err != nil {
		t.Fatalf("PrepareHatchUnit 失败: %v", err)
	}
	if !strings.Contains(strings.Join(argv, " "), "--collect") {
		t.Errorf("PrepareHatchUnit 产出的 argv 缺 --collect：%v", argv)
	}
	want := []string{
		"systemctl --user is-active " + unit,
		"systemctl --user stop " + unit,
		"systemctl --user reset-failed " + unit,
	}
	if len(f.calls) != len(want) {
		t.Fatalf("清理命令条数不对（要 %d 条，实得 %d）：%v", len(want), len(f.calls), f.calls)
	}
	for i := range want {
		if f.calls[i] != want[i] {
			t.Errorf("第 %d 条命令应为 %q，实得 %q（顺序与 argv 都要对）", i+1, want[i], f.calls[i])
		}
	}
}

// ③ 幂等：单元不存在（is-active 空 + 非零退出）⇒ 不 stop，只 reset-failed。
func TestPrepareHatchUnit_IdempotentWhenUnitMissing(t *testing.T) {
	unit := UnitName("example-moe-36b")
	f := &fakeUnitRunner{
		out: func(string, []string) string { return "" },
		errFor: func(name string, args []string) error {
			if len(args) > 1 && args[1] == "is-active" {
				return errors.New("exit status 3") // is-active 对 inactive/failed 也非零退出
			}
			return nil
		},
	}
	if _, err := PrepareHatchUnit(unit, goodSpec(), f.run); err != nil {
		t.Fatalf("PrepareHatchUnit 失败: %v", err)
	}
	want := []string{
		"systemctl --user is-active " + unit,
		"systemctl --user reset-failed " + unit,
	}
	if strings.Join(f.calls, "|") != strings.Join(want, "|") {
		t.Fatalf("单元不存在时应只 reset-failed（幂等），实得：%v", f.calls)
	}
}

// ③b 活动中的单元 ⇒ 同样先 stop 再 reset-failed（不把「还在跑」的同名单元留在名字上）。
func TestPrepareHatchUnit_StopsActiveUnit(t *testing.T) {
	unit := UnitName("example-moe-36b")
	f := &fakeUnitRunner{out: func(name string, args []string) string {
		if len(args) > 1 && args[1] == "is-active" {
			return "active\n"
		}
		return ""
	}}
	actions := cleanupUnitBeforeHatch(unit, f.run)
	if strings.Join(actions, ",") != "stop,reset-failed" {
		t.Fatalf("active 单元的动作序列应为 stop,reset-failed，实得 %v（原始命令 %v）", actions, f.calls)
	}
}

// ④ 清理失败不许阻塞孵化，也不许假装清过：benign 错误算成功，非 benign 如实标出。
func TestPrepareHatchUnit_ErrorsDoNotBlockHatch(t *testing.T) {
	unit := UnitName("example-moe-36b")
	// ④a：stop 报「not loaded」（benign，X3 实测二次 stop rc=5 就是这种）⇒ 仍算 stop 成功。
	benign := &fakeUnitRunner{
		out: func(name string, args []string) string {
			switch {
			case len(args) > 1 && args[1] == "is-active":
				return "failed\n"
			case len(args) > 1 && args[1] == "stop":
				return "Failed to stop " + unit + ": Unit " + unit + ".service not loaded."
			}
			return ""
		},
		errFor: func(name string, args []string) error {
			if len(args) > 1 && args[1] == "stop" {
				return errors.New("exit status 5")
			}
			return nil
		},
	}
	if got := cleanupUnitBeforeHatch(unit, benign.run); strings.Join(got, ",") != "stop,reset-failed" {
		t.Fatalf("benign 失败（not loaded）应按成功记：实得 %v（命令 %v）", got, benign.calls)
	}
	// ④b：非 benign 失败 ⇒ 动作序列里如实带 stop-failed / 少一条 reset-failed，但 PrepareHatchUnit 照常回 argv。
	bad := &fakeUnitRunner{
		out: func(name string, args []string) string {
			if len(args) > 1 && args[1] == "is-active" {
				return "failed\n"
			}
			return "some real failure"
		},
		errFor: func(string, []string) error { return errors.New("exit status 1") },
	}
	argv, err := PrepareHatchUnit(unit, goodSpec(), bad.run)
	if err != nil {
		t.Fatalf("清理失败不该阻塞孵化（真清不掉时 systemd-run 自己会报原文）：%v", err)
	}
	if len(argv) == 0 {
		t.Fatal("清理失败时也必须产出 argv")
	}
	got := cleanupUnitBeforeHatch(unit, bad.run)
	if strings.Join(got, ",") == "stop,reset-failed" {
		t.Fatalf("非 benign 失败不许记成成功：实得 %v", got)
	}
	if !strings.Contains(strings.Join(got, ","), "stop-failed") {
		t.Fatalf("stop 真失败时应如实标出 stop-failed：实得 %v", got)
	}
}

// ⑤ dry-run：run == nil ⇒ 一条命令都不发（只想看 argv 的调用方不碰 systemd）。
func TestPrepareHatchUnit_NilRunnerDoesNothing(t *testing.T) {
	unit := UnitName("example-moe-36b")
	argv, err := PrepareHatchUnit(unit, goodSpec(), nil)
	if err != nil {
		t.Fatalf("PrepareHatchUnit(nil runner) 失败: %v", err)
	}
	if !strings.Contains(strings.Join(argv, " "), "--collect") {
		t.Errorf("dry-run 也要产出带 --collect 的 argv：%v", argv)
	}
	if actions := cleanupUnitBeforeHatch("", nil); len(actions) != 0 {
		t.Errorf("空单元名 + nil runner 应什么都不做，实得 %v", actions)
	}
}
