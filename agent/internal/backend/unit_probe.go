// unit_probe.go —— 卵单元状态探针（孵化路径的就绪等待靠它看出「单元已经死了」）。
//
// 为什么必须有它（2026-09-15 第一枚卵真机实测 · 报告 §12 缺陷 9）：
// 孵化路径下引擎不是子端的子进程（sp.proc 恒 nil）⇒ 既有 waitForReady 的两条出口
// （健康检查通过 / 本端进程已退出）都不成立 ⇒ 单元 1 秒死（status=127/1）时它只能**干等满
// 120s** 才回「等待后端就绪超时」（真机实测 120.16s）。探针读 systemd 的
// ActiveState/SubState/Result/ExecMainStatus ⇒ 秒级明确报错；死的时候把单元日志尾巴
// 一并带出来（否则「为什么死」还得再上一趟机器——真机那次的死因就在日志里：
// `bwrap: setenv failed`）。
//
// 口径（写死，不许放宽）：
//   - **探不到就不下结论**（非 Linux / 没有 systemctl / 命令失败）：调用方继续等，绝不把
//     「探不到」当成「已经死了」——那会把慢启动的正确卵误杀；
//   - 只有**明确读到终态**（ActiveState ∈ {inactive, failed}）才判死；activating / reloading /
//     deactivating 这类过渡态与空值一律**不判死**；
//   - 本文件只**读**（systemctl show / journalctl），不发任何动作——动作只有孵化与收卵两处。
package backend

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// unitCmdRunnerFunc 执行一条只读单元命令（systemctl / journalctl）并回原文。
// 抽成类型 + 注入点只为单测：开发机上没有 systemd 用户实例（macOS），真依赖只在 X3 上。
type unitCmdRunnerFunc func(timeout time.Duration, name string, args ...string) (string, error)

// unitCmdRunner 生产实现：跑一条命令，带超时，超时/失败都把原文回给调用方（不许吞）。
var unitCmdRunner unitCmdRunnerFunc = func(timeout time.Duration, name string, args ...string) (string, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}

// unitState 一枚卵单元此刻的状态（只读快照）。
type unitState struct {
	// ActiveState systemd 的活跃态：active / activating / deactivating / reloading / inactive / failed。
	ActiveState string
	// SubState 子态（running / dead / failed / start-pre …）。
	SubState string
	// Result 单元最近一次的结局（success / exit-code / signal / timeout / start-limit-hit …）。
	Result string
	// ExecMainStatus 主进程退出码（127 = 可执行文件不存在，1 = 通用失败；0 且 inactive = 正常退出）。
	ExecMainStatus int
	// Slice 单元所属切片（llm.slice = 本子端孵的卵；它同时是「这枚卵是不是我管的」的归属凭据）。
	Slice string
	// ControlGroup 单元的 cgroup 路径（含 llm.slice 的完整归属链，用于交叉校验 Slice）。
	ControlGroup string
	// JournalTail 单元日志尾部（只在**已判死**时才去抓；抓不到就是空串，不假装有）。
	JournalTail string
}

// inSlice 单元是否归属某个切片（Slice 或 cgroup 链上出现 ⇒ 是）。
//
// 为什么要两道：`systemctl show -p Slice` 给的是切片名，`ControlGroup` 给的是完整链
// （/user.slice/user-1000.slice/user@1000.service/llm.slice/zerg-x.service）——
// 任一条读到都算归属确认；**两条都读不到 ⇒ 不是「确认归属」，调用方不许对它动手**。
func (s unitState) inSlice(slice string) bool {
	slice = strings.TrimSpace(slice)
	if slice == "" {
		return false
	}
	if strings.TrimSpace(s.Slice) == slice {
		return true
	}
	return strings.Contains(s.ControlGroup, "/"+slice+"/")
}

// dead 单元是否**明确**已经不在运行（返回 (true, 一句话理由)）。
//
// 保守定义：只有读到 ActiveState ∈ {inactive, failed} 才判死——这是「读到反证」；
// 空值（单元不存在 / 还没被建出来）与过渡态都算「不知道」，交由调用方继续等（§6.9 同一精神：
// 「没读到证据」与「读到反证」不是一回事）。
func (s unitState) dead() (bool, string) {
	as := strings.ToLower(strings.TrimSpace(s.ActiveState))
	if as != "inactive" && as != "failed" {
		return false, ""
	}
	why := fmt.Sprintf("ActiveState=%s SubState=%s Result=%s ExecMainStatus=%d",
		orDash(s.ActiveState), orDash(s.SubState), orDash(s.Result), s.ExecMainStatus)
	return true, why
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return strings.TrimSpace(s)
}

// parseUnitState 解析 `systemctl --user show <unit> -p ActiveState -p SubState -p Result
// -p ExecMainStatus` 的 Key=Value 输出（纯函数，便于单测）。
//
// 不用 `--value`：老版本 systemd 对多属性 + --value 的打印口径不一致（有的只打一行），
// Key=Value 是各版本都稳的形态。
func parseUnitState(out string) unitState {
	var st unitState
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "ActiveState":
			st.ActiveState = strings.TrimSpace(v)
		case "SubState":
			st.SubState = strings.TrimSpace(v)
		case "Result":
			st.Result = strings.TrimSpace(v)
		case "Slice":
			st.Slice = strings.TrimSpace(v)
		case "ControlGroup":
			st.ControlGroup = strings.TrimSpace(v)
		case "ExecMainStatus":
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err == nil {
				st.ExecMainStatus = n
			}
		}
	}
	return st
}

// probeUnitState 读一枚卵单元的状态（生产实现；单测替换 unitStateProbe 这个变量）。
//
// 判死之后**顺带**抓一次单元日志尾巴：死因几乎总在引擎/封闭空间自己那行输出里，
// 「报错了但不说是为什么」是本项目明令不许的形态（§6.9）。
func probeUnitState(unit string) (unitState, error) {
	return probeUnitStateWith(unitCmdRunner, unit)
}

// probeUnitStateWith 带注入执行器的实现（纯逻辑，便于单测一条条喂假输出）。
func probeUnitStateWith(run unitCmdRunnerFunc, unit string) (unitState, error) {
	if strings.TrimSpace(unit) == "" {
		return unitState{}, fmt.Errorf("缺单元名")
	}
	if run == nil {
		run = unitCmdRunner
	}
	out, err := run(10*time.Second, "systemctl", "--user", "show", unit,
		"-p", "ActiveState", "-p", "SubState", "-p", "Result", "-p", "ExecMainStatus",
		"-p", "Slice", "-p", "ControlGroup")
	st := parseUnitState(out)
	if err != nil {
		// 命令非零退出也可能带回有用的属性行（旧 systemd 对「单元不存在」会 rc≠0）：
		// 读到 ActiveState 就照常用，读不到才算探不到。
		if strings.TrimSpace(st.ActiveState) == "" {
			return st, fmt.Errorf("跑 systemctl --user show %s 失败：%v：%s", unit, err, strings.TrimSpace(out))
		}
	}
	if dead, _ := st.dead(); dead {
		st.JournalTail = unitJournalTail(run, unit, 15)
	}
	return st, nil
}

// unitJournalTail 抓单元日志尾部（最多 n 行）。抓不到 ⇒ 空串（不假装有、不报错打断主流程）。
func unitJournalTail(run unitCmdRunnerFunc, unit string, n int) string {
	if run == nil || n <= 0 {
		return ""
	}
	out, err := run(5*time.Second, "journalctl", "--user", "-u", unit, "-n", strconv.Itoa(n), "--no-pager", "-o", "cat")
	if err != nil && strings.TrimSpace(out) == "" {
		return ""
	}
	return strings.TrimSpace(out)
}

// unitStateProbe 探针的注入点（生产 = probeUnitState；单测替换它，不碰真 systemd）。
var unitStateProbe = probeUnitState
