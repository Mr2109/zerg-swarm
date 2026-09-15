// hatch_pid.go —— 「核验对象是不是**空间内进程**」的判据（缺陷 2 的根因与解法）。
//
// 真机事实（X3，2026-09-15，第一枚卵孵化时的两条 /proc 读数）：
//
//	pid=1379471  systemd MainPID   mnt-ns=mnt:[4026531832]  → /models 不存在、/data 可见、可见 550 pids（宿主视图）
//	pid=1379473  MainPID 的子进程   mnt-ns=mnt:[4026532767]  → /models=ro、/work=rw、/data 隐藏、2 pids（空间视图）
//
// ⇒ **MainPID 是 bwrap 的父进程，它留在原命名空间**：拿它去读 /proc/<pid>/mountinfo 读到的是宿主
// 视图，四判据里必然有好几项"不符"；而上层的处置是「不符 ⇒ 收卵 + 502」——**每一枚卵都会被
// 自己的核验当场杀掉**（真机第一次孵化就是这么死的，日志里写着「核验不符：/models 不是只读；
// 宿主权重树 /data 在空间内可见；/proc 不是新的」，而空间其实完全正确）。
//
// 所以核验对象必须换成**空间内进程**，判据（顺序即优先级，全部可复核）：
//
//	① 同一单元 cgroup（单元边界；bwrap 父进程与空间内进程同属该单元 cgroup）；
//	② 不是给定的那个 pid（实机上是 bwrap 父进程）；
//	③ **挂载命名空间与宿主不同** ⇒ 它进了 bwrap 新建的挂载命名空间（bwrap 必自建）；
//	④ 有 comm != "bwrap" 的候选就优先取它（引擎/空间内的父进程），否则取 pid 最小的候选。
//	   ④ 只是**确定性**，不是正确性：mountinfo 是**每个挂载命名空间一份**，同 ns 内取谁都得到
//	   同一份答案；pid 最小只为让同一情形每次给同一个结论（可复现）。
//
// 本文件是平台无关的**纯判据**（可在 macOS 上单测）；读 /proc 的部分在 hatch_linux.go。
package hatch

import (
	"fmt"
	"strings"
)

// spaceProc 一个候选进程在 /proc 里读到的、够判定「它是不是空间内进程」的几项事实。
type spaceProc struct {
	PID    int    // 进程号
	Cgroup string // /proc/<pid>/cgroup 原文（cgroup v2 = "0::/path"；v1 = 多行）
	MntNS  string // /proc/<pid>/ns/mnt 的链接目标，如 "mnt:[4026532767]"
	Comm   string // /proc/<pid>/comm（bwrap 外壳靠它认出来）
}

// cgroupPath 从 /proc/<pid>/cgroup 原文里取出 cgroup 路径（用于判"同一个单元"）。
//
// 为什么不用原文直接比：cgroup v1 是多行（`N:ctrl:/path`），v2 是单行（`0::/path`），
// 同一单元在不同机器上原文不同、路径相同 ⇒ 取路径段比。第一行就够（v1 各控制器同属一个路径）。
func cgroupPath(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.SplitN(line, ":", 3)
		if len(f) < 3 {
			continue
		}
		return strings.TrimSpace(f[2])
	}
	return ""
}

// pickSpacePID 从候选进程里挑出**空间内**那一个（纯函数，可单测）。
//
//   - given       = 其 cgroup 定义了"同一个单元"的那个 pid（实机上是单元 MainPID / bwrap 父进程）
//   - givenCgroup = 该 pid 的 /proc/<pid>/cgroup 原文
//   - hostMntNS   = **宿主**挂载命名空间标识（实机上 = bwrap 父进程 / 子端自己的 /proc/self/ns/mnt）
//   - procs       = 扫 /proc 得到的候选事实
//
// 返回 (空间内 pid, 人类可读的判定依据, 错误)。**找不到就报错**（上层据此记「未核验」），
// 绝不退回宿主视图 —— 那正是缺陷 2 的形态。
func pickSpacePID(given int, givenCgroup, hostMntNS string, procs []spaceProc) (int, string, error) {
	want := cgroupPath(givenCgroup)
	if want == "" {
		return 0, "", fmt.Errorf("给定的 pid=%d 读不出 cgroup 路径（%q）⇒ 界不定单元边界，不猜、不核验", given, givenCgroup)
	}
	inUnit := 0
	var space, plain []spaceProc // plain = 空间内且 comm 不是 bwrap 的（引擎/空间内父进程）
	for _, p := range procs {
		if p.PID <= 0 || p.PID == given {
			continue
		}
		if cgroupPath(p.Cgroup) != want {
			continue
		}
		inUnit++
		if p.MntNS == "" || p.MntNS == hostMntNS {
			continue // 还在宿主挂载命名空间里 ⇒ 不是空间内进程（正是 MainPID 的形态）
		}
		space = append(space, p)
		if p.Comm != "bwrap" {
			plain = append(plain, p)
		}
	}
	if len(space) == 0 {
		return 0, "", fmt.Errorf("单元 cgroup（%s）里除了给定的 pid=%d 之外没有第二个挂载命名空间的进程"+
			"（同单元共 %d 个进程）⇒ 空间内进程一个都没找到，按「读不到」处置；不用宿主视图当核验凭据", want, given, inUnit)
	}
	// 优先非 bwrap（引擎/空间内的父进程）；mountinfo 按挂载命名空间，同 ns 内取谁都得到同一份
	// 答案，取最小 pid 只为让同一情形每次给同一个结论（可复现）。
	pick := space
	if len(plain) > 0 {
		pick = plain
	}
	chosen := pick[0]
	for _, p := range pick[1:] {
		if p.PID < chosen.PID {
			chosen = p
		}
	}
	why := fmt.Sprintf("给定 pid=%d 在宿主挂载命名空间（%s，bwrap 父进程）⇒ 改验空间内进程 pid=%d（mnt-ns=%s，comm=%s，同单元 cgroup=%s）",
		given, hostMntNS, chosen.PID, chosen.MntNS, chosen.Comm, want)
	return chosen.PID, why, nil
}
