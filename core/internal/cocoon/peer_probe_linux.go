//go:build linux

// peer_probe_linux.go — Linux 的对端身份：`SO_PEERPIDFD` 首选、`SO_PEERCRED` 回退 ⇒ strong。
//
// ★ 为什么要 pidfd（v3 §A 实测结论）：`SO_PEERCRED` 给的是 **connect 时的 pid 快照**，
// 对端进程退出、pid 号被复用时，这枚快照会绑到**另一个进程**上。`SO_PEERPIDFD`
// （opt=77，内核 ≥6.5）拿的是内核持有的 `struct pid` 引用，复用的 pid 号换不出同一个 pidfd。
//
// ★ 回退档的诚实标注：拿不到 pidfd 时用 SO_PEERCRED，但**同时**读 `/proc/<pid>/stat` 的
// `starttime`（第 22 字段）当抗复用那一维 —— 这是用户态补的，强度弱于 pidfd，故 Kind 里
// 明确写 `linux-peercred`（审计面一眼能看出走的是哪一档）。
package cocoon

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// platformPeerProbe — linux 实现。
func platformPeerProbe(fd int) (PeerIdentityRecord, bool) {
	// ① 首选 pidfd。
	if pidfd, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_PEERPIDFD); err == nil && pidfd >= 0 {
		pid, ok := pidFromPidfd(pidfd)
		unix.Close(pidfd)
		if ok && pid > 0 {
			return PeerIdentityRecord{
				Kind:       "linux-peerpidfd",
				ID:         peerIDText(pid),
				StartTime:  procStartTime(pid),
				Confidence: PeerIdentityStrong,
			}, true
		}
	}

	// ② 回退 SO_PEERCRED（connect 时快照）。
	cred, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return nonePeer("linux-peercred-failed"), false
	}
	if cred.Pid <= 0 {
		return nonePeer("linux-peercred-zero"), false
	}
	// 别的 uid 连进来 ⇒ 身份**不可用**（不是"weak"）。socket 是 0600、目录 0700，
	// 正常情况根本到不了这里；到了就说明文件权限被改过 —— 记为 none，让令牌单独承担授权。
	if int(cred.Uid) != os.Getuid() {
		return PeerIdentityRecord{
			Kind:       "linux-peercred-otheruid",
			ID:         peerIDText(int(cred.Pid)),
			Confidence: PeerIdentityNone,
		}, true
	}
	return PeerIdentityRecord{
		Kind:       "linux-peercred",
		ID:         peerIDText(int(cred.Pid)),
		StartTime:  procStartTime(int(cred.Pid)),
		Confidence: PeerIdentityStrong,
	}, true
}

// pidFromPidfd — pidfd 里没有 pid 号，得从 `/proc/self/fdinfo/<pidfd>` 的 `Pid:` 行读。
func pidFromPidfd(pidfd int) (int, bool) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/self/fdinfo/%d", pidfd))
	if err != nil {
		return 0, false
	}
	for _, ln := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(ln, "Pid:") {
			continue
		}
		v, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(ln, "Pid:")))
		if err != nil || v <= 0 {
			return 0, false
		}
		return v, true
	}
	return 0, false
}

// procStartTime — `/proc/<pid>/stat` 第 22 字段（starttime，单位 jiffies）。
// ★ 不能按空格切整个文件：第 2 字段 comm 允许含空格与括号 ⇒ 从**最后一个** `)` 之后切。
func procStartTime(pid int) int64 {
	if pid <= 0 {
		return 0
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	s := string(data)
	i := strings.LastIndex(s, ")")
	if i < 0 || i+2 >= len(s) {
		return 0
	}
	fields := strings.Fields(s[i+2:]) // fields[0] = state（第 3 字段）
	const startTimeField = 22
	idx := startTimeField - 3
	if idx >= len(fields) {
		return 0
	}
	v, err := strconv.ParseInt(fields[idx], 10, 64)
	if err != nil {
		return 0
	}
	return v
}
