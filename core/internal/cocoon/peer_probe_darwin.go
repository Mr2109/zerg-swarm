//go:build darwin

// peer_probe_darwin.go — macOS 的对端身份：`LOCAL_PEERPID` ⇒ weak。
//
// ★ 三条刻意的取舍（v3 §A）：
//
//	① **不用 `LOCAL_PEERCRED`**。它打在 TCP fd 上会"成功但输出 0 字节" ⇒ 全零结构读出来
//	  正好是 uid 0 / root。本层从调用面上就不给它机会（见 peer_probe.go 的两道前置自检）。
//	② `LOCAL_PEERPID` 的返回值**必须 > 0** 才认：0 是"没取到"的同形读数，把它当 pid 等于
//	  把"谁都能冒充"当"pid 0 连进来了"。取不到 ⇒ none，✗ 不降权成 weak。
//	③ `LOCAL_PEERTOKEN` 用来补 `pidversion`（抗 pid 复用那一维）。拿不到就留 0 并**如实**
//	  标 weak —— macOS 上这个层级本来就只能到 VM 级归属（v3 §A：宿主看不到 guest 的 pid）。
package cocoon

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// auditToken — `LOCAL_PEERTOKEN` 给的 `audit_token_t`：8 × uint32，**字段顺序是 OpenBSM 的**：
//
//	auid, euid, egid, ruid, rgid, pid, asid, pidversion
//
// ★ 这里踩过一次：漏掉 `ruid` 之后整段错位一格 —— 读出来的 `PID` 其实是 `rgid`（20/staff），
// 而真 pid（41360）落在 `ASID` 上。错位的读数**看起来像"取到了"**，只有和 `LOCAL_PEERPID`
// 交叉比对才暴露（下面的 mismatch 闸就是为这个留的：宁可判 none，也不认一份对不上的 token）。
//
// ★ `pidversion` 是"这个 pid 号被复用了几次"的计数 ⇒ 与 pid 一起构成抗复用的一对。
type auditToken struct {
	AUID       uint32
	EUID       uint32
	EGID       uint32
	RUID       uint32
	RGID       uint32
	PID        uint32
	ASID       uint32
	PIDVersion uint32
}

// platformPeerProbe — darwin 实现：LOCAL_PEERPID（必须 >0）+ 尽力取 pidversion。
func platformPeerProbe(fd int) (PeerIdentityRecord, bool) {
	pid, err := unix.GetsockoptInt(fd, unix.SOL_LOCAL, unix.LOCAL_PEERPID)
	if err != nil {
		return nonePeer("macos-localpeerpid-failed"), false
	}
	// ★ 零值闸：全零/负数在 macOS 上正是"打错 fd"的假读数。判 none，✗ 不判 strong。
	if pid <= 0 {
		return nonePeer("macos-localpeerpid-zero"), false
	}

	rec := PeerIdentityRecord{
		Kind:       "macos-localpeerpid",
		ID:         peerIDText(pid),
		Confidence: PeerIdentityWeak,
	}
	if tok, ok := peerToken(fd); ok {
		// pid 与 token 里的 pid 必须一致（不一致说明这枚 fd 在读的过程中被换了 ⇒ 不认）。
		if tok.PID != uint32(pid) {
			return nonePeer("macos-peertoken-mismatch"), false
		}
		rec.StartTime = int64(tok.PIDVersion)
	}
	return rec, true
}

// peerToken — 尽力取 `audit_token_t`（失败不是错误：weak 档不依赖它）。
func peerToken(fd int) (auditToken, bool) {
	var tok auditToken
	size := uint32(unsafe.Sizeof(tok))
	_, _, errno := unix.Syscall6(
		unix.SYS_GETSOCKOPT,
		uintptr(fd),
		uintptr(unix.SOL_LOCAL),
		uintptr(unix.LOCAL_PEERTOKEN),
		uintptr(unsafe.Pointer(&tok)),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	if errno != 0 {
		return auditToken{}, false
	}
	if size != uint32(unsafe.Sizeof(tok)) {
		return auditToken{}, false
	}
	return tok, true
}
