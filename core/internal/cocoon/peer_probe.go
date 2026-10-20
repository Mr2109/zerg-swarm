// peer_probe.go — 对端身份探测的**公共前半段**：拿到连接 fd、证明它确实是 AF_UNIX socket、
// 再交给分平台的探针。
//
// ★ 为什么先证"是 unix socket"（v3 §A 的 macOS 陷阱）：
//
//	macOS 把 `LOCAL_PEERCRED` 打在 **TCP fd** 上会**返回成功但输出 0 字节** ⇒ 读出来的结构
//	恰好是 uid 0 / root，version 0 又恰好等于 `XUCRED_VERSION` ⇒ 两道自检全过，"谁连进来的"
//	变成"root 连进来的"。这条路本层**根本不走**（不用 LOCAL_PEERCRED），再加两道前置：
//	  ① getsockname 必须是 `*unix.SockaddrUnix`（AF_UNIX）；
//	  ② fstat 必须是 S_IFSOCK。
//	任何一道不过 ⇒ 直接 none，**不调**任何 LOCAL_/SO_ 探测。
//
// ★ 强度口径（v3 §A 定稿）：
//
//	Linux  SO_PEERPIDFD（首选，内核持 struct pid 引用）／ SO_PEERCRED（回退） ⇒ strong
//	macOS  LOCAL_PEERPID ⇒ weak（VM 级归属；pidversion 做抗复用那一维）
//	其余   拿不到 ⇒ none（✗ 不许把"没能力"读成 weak 或 strong）
package cocoon

import (
	"net"
	"strconv"

	"golang.org/x/sys/unix"
)

// nonePeer — 拿不到身份时的唯一形态（强度 none、不留半截字段）。
func nonePeer(kind string) PeerIdentityRecord {
	return PeerIdentityRecord{Kind: kind, Confidence: PeerIdentityNone}
}

// defaultPeerProbe — 生产的对端身份探测：先证 socket 类型，再分平台取。
func defaultPeerProbe(conn *net.UnixConn) PeerIdentityRecord {
	if conn == nil {
		return nonePeer("no-conn")
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		return nonePeer("no-syscallconn")
	}

	rec := nonePeer("probe-unavailable")
	var ok bool
	if ctlErr := raw.Control(func(fd uintptr) {
		rec, ok = probeFD(int(fd))
	}); ctlErr != nil {
		return nonePeer("no-fd")
	}
	if !ok {
		if rec.Kind == "" {
			return nonePeer("none")
		}
		return rec
	}
	return rec
}

// probeFD — 在一个裸 fd 上做"是不是 AF_UNIX socket"的两道自检，再交给平台探针。
// 对外暴露成小函数是为了让测试能在**任意 fd** 上断言这两道闸（见 broker_test.go 的错 fd 格）。
func probeFD(fd int) (PeerIdentityRecord, bool) {
	if fd < 0 {
		return nonePeer("bad-fd"), false
	}
	// ① fstat 必须说它是 socket。
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return nonePeer("fstat-failed"), false
	}
	if st.Mode&unix.S_IFMT != unix.S_IFSOCK {
		return nonePeer("not-socket"), false
	}
	// ② getsockname 必须给出 AF_UNIX 地址（TCP fd 在这一步就被挡住，LOCAL_* 一次都不调）。
	sa, err := unix.Getsockname(fd)
	if err != nil {
		return nonePeer("getsockname-failed"), false
	}
	if _, isUnix := sa.(*unix.SockaddrUnix); !isUnix {
		return nonePeer("not-unix-socket"), false
	}
	// ③ 分平台。
	if rec, ok := platformPeerProbe(fd); ok {
		return rec, true
	}
	return nonePeer("platform-unavailable"), false
}

// peerIDText — 把 pid 一类整数变成 ID 字符串（空 pid ⇒ 空串，✗ 不写 "0"）。
func peerIDText(pid int) string {
	if pid <= 0 {
		return ""
	}
	return strconv.Itoa(pid)
}
