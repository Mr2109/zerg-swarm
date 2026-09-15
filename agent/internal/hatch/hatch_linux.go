//go:build linux

// hatch_linux.go —— 孵化器在 Linux 上的真正执行路径（X3 用）。
//
// 分工（§6.1）：
//
//	systemd-run --user  ⇒ 建瞬态单元 + 独立 slice（归属：不被 x3-agent.service 的 KillMode 带走）
//	bwrap              ⇒ 建封闭空间（文件系统视图 + 进程视图 + 只读）
//
// 收卵 = 停那个单元（幂等：二次停 rc=5 视为成功）；停完整棵树一起收（KillMode=control-group）。
package hatch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Hatcher 孵化器（无状态；超时可由调用方通过 ctx 控制）。
type Hatcher struct {
	// StopTimeout 收卵时等 systemctl 返回的上限（0 ⇒ 20s）。
	StopTimeout time.Duration
}

// Hatch 孵一枚卵：创建瞬态单元（内含 bwrap 封闭空间 + 引擎）。
// 返回单元名（后续收卵/观测都用它）。
func (h Hatcher) Hatch(ctx context.Context, spec Spec) (string, error) {
	unit := UnitName(spec.EggID)
	argv, err := BuildSystemdRunArgv(spec, unit)
	if err != nil {
		return unit, err
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// 失败必须回原文：孵化失败的原因（polkit/userns/权限）都在这里
		return unit, fmt.Errorf("孵化单元创建失败（%s）：%v：%s", unit, err, strings.TrimSpace(string(out)))
	}
	return unit, nil
}

// Collect 收卵：停单元（幂等）。返回错误仅当确实是"停不掉"。
func (h Hatcher) Collect(ctx context.Context, unit string) error {
	timeout := h.StopTimeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "systemctl", "--user", "stop", unit)
	out, err := cmd.CombinedOutput()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = -1
		}
	}
	return collectOutcome(rc, string(out))
}

// Active 单元是否还在跑（收卵后用于核验"真的收干净了"）。
func (h Hatcher) Active(ctx context.Context, unit string) (bool, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-active", unit)
	out, _ := cmd.CombinedOutput()
	switch strings.TrimSpace(string(out)) {
	case "active", "activating", "reloading":
		return true, nil
	case "inactive", "failed", "deactivating", "":
		// "" = 单元不存在 ⇒ 视为不在跑（收干净了）
		return false, nil
	default:
		return false, nil
	}
}

// UnitCgroup 单元的 cgroup 路径（观测面：核"它确实不在 x3-agent.service 里"）。
func (h Hatcher) UnitCgroup(ctx context.Context, unit string) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "--user", "show", unit, "-p", "ControlGroup", "--value")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读单元 cgroup 失败（%s）：%v：%s", unit, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// VerifyEnclosure 运行时封闭性核验（§6.9 硬要求）：读 `/proc/<pid>/mountinfo` 实地核，
// **不得以单元状态为凭**。pid 可由调用方从单元内进程取（引擎进程）。
func (h Hatcher) VerifyEnclosure(pid int) (EnclosureReport, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/mountinfo", pid))
	if err != nil {
		return EnclosureReport{}, fmt.Errorf("读 /proc/%d/mountinfo 失败：%w", pid, err)
	}
	rep := CheckEnclosure(string(b))
	// 进程视图隔离的间接证据：空间内 /proc 里进程数极少（宿主宿主看不到）
	if n, err := countProcs(pid); err == nil {
		rep.NewPIDNamespace = n <= 8 // bwrap+引擎的正常值；宿主上是几百上千
	}
	return rep, nil
}

// countProcs 数该 pid 命名空间里可见的进程数（读 /proc/<pid>/root/proc 或 /proc 下的子目录）。
func countProcs(pid int) (int, error) {
	entries, err := os.ReadDir(fmt.Sprintf("/proc/%d/root/proc", pid))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err == nil {
			n++
		}
	}
	return n, nil
}
