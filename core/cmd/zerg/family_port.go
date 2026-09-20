// family_port.go —— 端口面只读一条（§一 A2 · 缺口-命令面-20260921 §十一 P0-7）。
//
// 为什么它排 P0：**就绪判据必须绑自己的新 pid**（血泪第 2 条）唯一的手段就是
// `lsof -nP -iTCP:8580 -sTCP:LISTEN`（1 行，但不可替代）—— 而「有进程在应答 ≠ 我的进程在应答」
// 正是批 A 撞过的坑。命令化之后：换件后起点判据、混版/双实例/幽灵件排查、`cocoon open`
// 起的服务、`F5 日志面` 修好后验端点归属，都有真源可读。
//
// 口径（照 §十一 P0-7 的形态，不自造）：
//
//	形态 `zerg port ls [<端口>]`（缺省列**声明面**端口：主控/网关/子端）
//	输出 `port/pid/ppid/process/sock/path`（path = 在跑件路径 · sock = 内核 socket 地址，见 sockAddr 的偏离说明）
//	退码 `0` 读到 · `2` 端口非法 · `8` **取不到属主**（含「指名问的那一个没人听」）
//
// ★ 一处刻意的不对称，明写在人面里：**无名调查**（`port ls`）全都没人听 ⇒ 仍退 0
// （它是「列清单」这个问法，空清单是答案）；**指名问**（`port ls 8580`）没人听 ⇒ 退 8
// （它是「就绪判据」这个问法，「没有属主」正是要报的那个结果）。两种问法两个语义，不许混。
package main

import (
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// declaredPorts —— 声明面端口（**真源 = `core/internal/statepath`**，命令面不另写一份）。
func declaredPorts() []struct {
	name string
	port int
} {
	return []struct {
		name string
		port int
	}{
		{"主控 core", statepath.CorePort()},
		{"网关 gateway", statepath.GatewayPort()},
		{"子端 agent", statepath.AgentPort()},
	}
}

func cmdPortLs(inv *invocation, stdout, stderr io.Writer) int {
	// 指名问 / 无名调查：**两种问法两个退码语义**（见文件头）。
	named := false
	ports := declaredPorts()
	if len(inv.args) > 0 && strings.TrimSpace(inv.args[0]) != "" {
		named = true
		n, err := strconv.Atoi(strings.TrimSpace(inv.args[0]))
		if err != nil || n < 1 || n > 65535 {
			inv.setErr("usage", "bad_port", "端口不在 1–65535")
			fmt.Fprintf(stderr, "%s: 端口 %q 不合法（要 1–65535 的整数 · 退码 2）\n", progName, inv.args[0])
			return exitUsage
		}
		ports = []struct {
			name string
			port int
		}{{"（指名）", n}}
	}
	if _, err := exec.LookPath("lsof"); err != nil {
		inv.setErr("blocked", "lsof_absent", "lsof 不在")
		fmt.Fprintf(stderr, "%s: 本机没有 `lsof` ⇒ 属主查不出来（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}

	rows := []map[string]string{}
	failed := 0
	for _, p := range ports {
		owners, err := lsofListeners(p.port)
		if err != nil {
			failed++
			fmt.Fprintf(stderr, "%s: %d（%s）的属主查不到：%v\n", progName, p.port, p.name, err)
			continue
		}
		if len(owners) == 0 {
			fmt.Fprintf(stderr, "%s: %d（%s）**没有进程在听**\n", progName, p.port, p.name)
			continue
		}
		for _, pid := range owners {
			ppid, cmdline := psInfo(pid)
			rows = append(rows, map[string]string{
				"port":    strconv.Itoa(p.port),
				"pid":     pid,
				"ppid":    ppid,
				"process": p.name,
				"sock":    sockAddr(pid, p.port),
				"path":    cmdline,
			})
		}
	}
	fmt.Fprintf(stderr, "%s: 查了 %d 个端口 · 有属主的 %d 个", progName, len(ports), len(rows))
	if failed > 0 {
		fmt.Fprintf(stderr, " · 查不出的 %d 个", failed)
	}
	fmt.Fprintln(stderr)
	if failed > 0 {
		inv.setErr("blocked", "lsof_failed", "部分端口属主查不到")
		fmt.Fprintf(stderr, "%s: 有端口**属主读不到** ⇒ 不给结论（退码 8）—— 不许把「读不到」当「没人听」\n", progName)
		return exitBlocked
	}
	if named && len(rows) == 0 {
		inv.setErr("blocked", "no_listener", "指名问的端口没有属主")
		fmt.Fprintf(stderr, "%s: 这个端口**没有属主** —— 就绪判据的答案是「没起来」（退码 8）；"+
			"要看看别的端口：`zerg port ls`（无名调查）\n", progName)
		return exitBlocked
	}
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "%s: 声明面三个端口一个都没人听 —— 这是「清单为空」，不是读不到（退码 0）\n", progName)
		return exitOK
	}
	return listCmd(inv, stdout, stderr, []string{"port", "pid", "ppid", "process", "sock", "path"}, rows)
}

// lsofListeners 取某端口上的 LISTEN 进程号（去重排序 · 现读不缓存）。
//
// 退码语义：`lsof` 的 1 = **没有匹配**（正常答案，不是错）；别的非零 ⇒ 真错误（上层报 8）。
func lsofListeners(port int) ([]string, error) {
	cmd := exec.Command("lsof", "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN")
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil, nil // 无匹配
		}
		return nil, fmt.Errorf("lsof：%v（%s）", err, strings.TrimSpace(errb.String()))
	}
	seen := map[string]bool{}
	out2 := []string{}
	for i, ln := range strings.Split(out.String(), "\n") {
		if i == 0 || strings.TrimSpace(ln) == "" {
			continue // 表头
		}
		f := strings.Fields(ln)
		if len(f) < 2 {
			continue
		}
		if !seen[f[1]] {
			seen[f[1]] = true
			out2 = append(out2, f[1])
		}
	}
	return out2, nil
}

// psInfo 取 pid 的父进程与**在跑件路径**（`ps -o ppid=,command=` 是真源，别处不许再算一遍）。
func psInfo(pid string) (ppid, command string) {
	out, err := exec.Command("ps", "-o", "ppid=,command=", "-p", pid).Output()
	if err != nil {
		return "（读不到）", "（读不到）"
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "（已退出）", "（已退出）"
	}
	f := strings.SplitN(s, " ", 2)
	if len(f) < 2 {
		return f[0], ""
	}
	return strings.TrimSpace(f[0]), strings.TrimSpace(f[1])
}

// sockAddr 取 «pid × 端口» 那条 LISTEN 的**内核 socket 地址**（lsof 默认面的 DEVICE 列，形如 `0x…`）。
//
// ★ 一处**照实记的偏离**（缺口稿 §十一 P0-7 写的是 `inode`）：本机 lsof **不给 inode** ——
// 现跑 `lsof -nP -a -p 51636 -iTCP:8580 -sTCP:LISTEN -F i` 只吐 `p51636` / `f11` 两行、**没有 `i` 行**。
// 与其把别的列硬叫成 inode，不如给一个有真源、语义单一的同效信号：**新进程 = 新 socket 地址**，
// 换件判据要的「在跑的件换没换」它同样答得上（读不到就明说读不到，不编一个数）。
func sockAddr(pid string, port int) string {
	out, err := exec.Command("lsof", "-nP", "-a", "-p", pid, "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN").Output()
	if err != nil {
		return "（读不到）"
	}
	lines := strings.Split(string(out), "\n")
	for i, ln := range lines {
		if i == 0 || strings.TrimSpace(ln) == "" {
			continue
		}
		for _, f := range strings.Fields(ln) {
			if strings.HasPrefix(f, "0x") {
				return f
			}
		}
	}
	return "（本机 lsof 不给）"
}
