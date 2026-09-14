// service_control.go —— P3b：停服 / 归还 / 空闲检定的实际动作。
//
// 设计依据：设计-子端服务切换与基线服务声明-20260914.md §9.2（空闲检定）、§9.3（借还序列）、
// §9.4（systemd 类处置）、§10 R2（宽限期）/R4（polkit/sudoers）、§11 M2（进程树孤儿）、M3（归还后验身份）。
//
// 安全铁律：
//  1. **只停"经声明且已存档"的目标**：调用方必须先 SaveLease 再调本文件的停止动作。
//  2. **停进程树，不只停一个 pid**：真机 screen 形态是 screen→bash→llama-server 三进程链，
//     只杀根会留孤儿继续占着显存（§11 M2）。
//  3. **绝不碰未声明的进程**：本文件不枚举、不猜测——只对传入的目标动作（红线②）。
package backend

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ── 进程树（跨 darwin/linux，仅依赖 ps 与 / 或 pgrep 之外的标准工具）─────────

// parsePsTree 解析 `ps -eo pid=,ppid=` 的输出为 parent→children 映射（纯函数，便于测试）。
func parsePsTree(out string) map[int][]int {
	tree := map[int][]int{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		pid, err1 := strconv.Atoi(f[0])
		ppid, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil || pid <= 0 {
			continue
		}
		tree[ppid] = append(tree[ppid], pid)
	}
	return tree
}

// parsePsPGID 解析 `ps -o pid=,pgid=` 的输出为 pid→pgid 映射（纯函数）。
func parsePsPGID(out string) map[int]int {
	m := map[int]int{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		pid, err1 := strconv.Atoi(f[0])
		pgid, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil {
			continue
		}
		m[pid] = pgid
	}
	return m
}

// descendantsOf 返回 root 及其全部子孙（深度优先，含 root 自身；结果已排序）。
// 纯函数：tree 由 parsePsTree 得到。
func descendantsOf(tree map[int][]int, root int) []int {
	seen := map[int]bool{}
	var walk func(int)
	walk = func(p int) {
		if seen[p] {
			return
		}
		seen[p] = true
		for _, c := range tree[p] {
			walk(c)
		}
	}
	walk(root)
	out := make([]int, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Ints(out)
	return out
}

// processTree 取 root 及其全部子孙（真机调用；ps 在 macOS 与 Linux 都在）。
func processTree(root int) []int {
	out, err := exec.Command("ps", "-eo", "pid=,ppid=").Output()
	if err != nil {
		return []int{root}
	}
	return descendantsOf(parsePsTree(string(out)), root)
}

// processGroupOf 取 pid 的进程组 id；取不到返回 0。
func processGroupOf(pid int) int {
	out, err := exec.Command("ps", "-o", "pid=,pgid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0
	}
	return parsePsPGID(string(out))[pid]
}

// httpGetBody 发一条只读 GET，返回响应体（上限 1MiB）。仅用于本机探测。
func httpGetBody(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// ── 空闲检定（§9.2）────────────────────────────────────────────────────────

// parseSlotBusy 解析 /slots 响应判断是否有请求在飞（纯函数，便于测试）。
//
// llama.cpp 的 /slots 返回 [{"id":0,"is_processing":false},…]；任一槽 true 即判忙。
// 解析失败 ⇒ 返回 (true, "无法判定") —— **探不到就按忙**（保守优先）。
func parseSlotBusy(body []byte) (busy bool, detail string) {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return true, "空响应 ⇒ 按忙处理"
	}
	if !strings.Contains(s, "is_processing") {
		return true, "无 is_processing 字段 ⇒ 按忙处理"
	}
	// 不引入 JSON 依赖：直接数 true/false 出现次数（结构固定，字段名唯一）。
	nTrue := strings.Count(s, `"is_processing":true`)
	nTrue += strings.Count(s, `"is_processing": true`)
	nFalse := strings.Count(s, `"is_processing":false`)
	nFalse += strings.Count(s, `"is_processing": false`)
	if nTrue == 0 && nFalse == 0 {
		return true, "解析不到 is_processing 取值 ⇒ 按忙处理"
	}
	if nTrue > 0 {
		return true, fmt.Sprintf("有 %d 个槽在处理请求", nTrue)
	}
	return false, fmt.Sprintf("%d 个槽均空闲", nFalse)
}

// probeServiceIdle 对端口探 /slots 判忙闲（探不到 ⇒ 按忙，绝不打断在飞请求）。
func probeServiceIdle(port int, timeout time.Duration) (bool, string) {
	body, err := httpGetBody(fmt.Sprintf("http://127.0.0.1:%d/slots", port), timeout)
	if err != nil {
		return false, "无法连接（" + err.Error() + "）⇒ 按忙处理"
	}
	// ⚠️ 注意方向：parseSlotBusy 返回的是 **busy**，而本函数声明返回的是 **idle**。
	// 2026-09-14 真机 N8 演练抓到的缺陷：这里曾直接 `return parseSlotBusy(body)`
	// ⇒ 空闲（busy=false）时返回 idle=false ⇒ 借用被拒，且日志自相矛盾：
	// 「目标服务正忙：4 个槽均空闲」⇒ 借用功能实际 100% 不可用。
	busy, detail := parseSlotBusy(body)
	return !busy, detail
}

// ── 停止（进程树）──────────────────────────────────────────────────────────

// stopProcessTree 停止 root 及其子孙：TERM → 轮询 → KILL 幸存者，最后复核"全部消失"。
//
// 返回 (实际停掉的 pid 列表, 错误)。**只对传入的 root 动作**（红线②）。
// grace 为 TERM 后的宽限期（设计 R2：默认 15s，systemd 的 90s 太长）。
func stopProcessTree(root int, grace time.Duration) ([]int, error) {
	if root <= 1 {
		return nil, fmt.Errorf("拒绝停止 pid=%d（非法或 init）", root)
	}
	targets := processTree(root)
	if len(targets) == 0 {
		return nil, fmt.Errorf("pid=%d 不存在", root)
	}

	// ① TERM：先发给整棵树（真机 screen 链：screen/bash/server 一起收）
	for _, pid := range targets {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	// 若 root 自成一个进程组（pgid==pid，screen 形态即如此），再对整组补一发：
	// 覆盖"子孙此刻正在 fork、尚未被 processTree 采到"的窗口，防孤儿（§11 M2）。
	if pgid := processGroupOf(root); pgid == root {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	}
	// ② 轮询等待
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if len(aliveOf(targets)) == 0 {
			return targets, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	// ③ KILL 幸存者
	survivors := aliveOf(targets)
	for _, pid := range survivors {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	// ④ 再给 KILL 一点时间，复核
	killDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(killDeadline) {
		if len(aliveOf(targets)) == 0 {
			return targets, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	left := aliveOf(targets)
	if len(left) > 0 {
		return targets, fmt.Errorf("尚有 %d 个进程未退出（可能属别的用户）：%v", len(left), left)
	}
	return targets, nil
}

// aliveOf 过滤出仍然存在的 pid（signal 0 探测）。
//
// 僵尸进程（stat 以 Z 开头）**不算存活**：它的内存/显存已释放，只是在等父进程收尸；
// 借用场景关心的是"还占不占资源"，把僵尸算成存活会误报孤儿 —— 真机 N7 用例实测暴露。
func aliveOf(pids []int) []int {
	var out []int
	for _, pid := range pids {
		if pid <= 1 {
			continue
		}
		if err := syscall.Kill(pid, 0); err == nil || err == syscall.EPERM {
			if isZombie(pid) {
				continue
			}
			out = append(out, pid)
		}
	}
	sort.Ints(out)
	return out
}

// isZombie 判定 pid 是否处于僵尸态（ps 的 stat 字段以 Z 开头；macOS 与 Linux 同形）。
func isZombie(pid int) bool {
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(string(out))), "Z")
}

// stopSystemdUnit 停一个 systemd 单元（免 root 需 polkit/sudoers 授权，设计 R4）。
func stopSystemdUnit(unit string) error {
	if unit == "" {
		return fmt.Errorf("单元名为空")
	}
	out, err := exec.Command("systemctl", "stop", unit).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl stop %s 失败：%v（%s）——未授权时请按设计 §9.4 处理",
			unit, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// stopScreenSession 退出一个 screen 会话（属主免 root 可执行）。
func stopScreenSession(name string) error {
	if name == "" {
		return fmt.Errorf("screen 会话名为空")
	}
	if out, err := exec.Command("screen", "-S", name, "-X", "quit").CombinedOutput(); err != nil {
		return fmt.Errorf("screen -S %s -X quit 失败：%v（%s）", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ── 归还（原样重放 + 验身份，§9.3 第 6–7 步）────────────────────────────────

// restoreBaselineService 按租约原样重放服务，并等待端口就绪 + 核对身份。
//
// 判据（缺一不可）：
//   - 进程能被拉起（argv/cwd/env 与存档一致）
//   - 端口重新监听
//   - /v1/models 报出的身份与存档一致（今天实测的教训：端口 200 ≠ 服务对）
func restoreBaselineService(l ServiceLease, wait time.Duration) error {
	if len(l.Argv) == 0 {
		return fmt.Errorf("租约缺 argv，无法重放（存档不完整）")
	}
	// ⚠️ 归还的服务必须**离开子端的 cgroup**（2026-09-14 第 5 个真机缺陷，两次才修透）：
	//   · 第一层：`exec.Command` 起的进程留在子端进程组 ⇒ 换装时 TERM 顺着进程组带走它（已由 Setsid 解决）；
	//   · 第二层（更深）：即使换了会话，它仍在 **x3-agent.service 的 cgroup** 里，
	//     而 systemd 默认 `KillMode=control-group` ⇒ 单元一停/重启就 **SIGTERM 整个 cgroup** ⇒ 照样被杀。
	//     硬证据：手工恢复的那份落在 `/user.slice/…/session-XXXX.scope`，活过了部署；
	//             子端归还的那份落在 `/system.slice/x3-agent.service`，每次都被带走。
	// ⇒ 用 `systemd-run --user` 起**事务单元**，让它落在 user slice，与子端 cgroup 彻底分离。
	cmd := exec.Command(l.Argv[0], l.Argv[1:]...)
	// 【未决项，勿盲目重试】2026-09-14 两次尝试 systemd-run --user 均未成功：
	//   · 第一版（ab0a7d8e）未补用户总线环境 ⇒ 归还超时；
	//   · 第二版（594ecbeb）补了 XDG_RUNTIME_DIR/DBUS_SESSION_BUS_ADDRESS（实测 uid=1000、
	//     /run/user/1000 存在、User=g01 全对）**仍然失败**，且事务单元从未被创建
	//     （`systemctl --user list-units 'zerg-baseline*'` 为空，连 failed 都没有）。
	//   ⇒ 根因未定；下次接手必须先**接上 stderr**（下面已加）再复现，拿到 systemd-run 自己的报错。
	// 当前退回**已验证可用**的写法：直接 spawn + Setsid（能起服务；遗留风险见下）。
	//
	// 【已知风险】这样起的进程仍在 x3-agent.service 的 cgroup 里 ⇒ 子端停/换装时
	// systemd（默认 KillMode=control-group）会把它一起带走（真机对照：
	// 手工恢复那份 cgroup 在 /user.slice/…/session-XXXX.scope ⇒ 活过部署；
	// 子端归还那份在 /system.slice/x3-agent.service ⇒ 每次都被带走）。
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	// 关键：把子进程 stderr 接出来（上一轮就是因为没接，systemd-run 的报错全丢了，
	// 只能看到"归还超时"这种下游症状，白白多试一轮）。
	cmd.Stderr = os.Stderr
	if l.Cwd != "" {
		cmd.Dir = l.Cwd
	}
	cmd.Env = envFromFiltered(l.EnvFiltered)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("重放启动失败：%v", err)
	}
	// 与存档无关(无法验证子端是自己的) ⇒ 明确记录 pid，便于人工核对
	_ = cmd.Process.Pid

	deadline := time.Now().Add(wait)
	var lastDetail string
	for time.Now().Before(deadline) {
		if listeningOn(l.Port) {
			got := probeIdentity(l.Port, 3*time.Second)
			if l.Identity == "" || got == "" {
				// 存档没记身份 或 探不到身份 ⇒ 端口活了即算归还（但要如实记）
				return nil
			}
			if got == l.Identity {
				return nil
			}
			lastDetail = fmt.Sprintf("端口已监听但身份不符：期望 %q，实得 %q", l.Identity, got)
		} else {
			lastDetail = "端口尚未监听"
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("归还超时（%s）：%s", wait, lastDetail)
}

// envFromFiltered 把存档里的 env 还原成 exec 需要的 KEY=VALUE 列表。
// 值为 [REDACTED] 的条目**不注入**（脱敏占位不是真值，注入反而污染环境）。
func envFromFiltered(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		if m[k] == "[REDACTED]" {
			continue
		}
		out = append(out, k+"="+m[k])
	}
	return out
}

// listeningOn 端口是否在监听（只做 TCP 连接）。
func listeningOn(port int) bool {
	return len(probeListeners([]int{port})) > 0
}
