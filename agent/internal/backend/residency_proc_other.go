//go:build !unix

// residency_proc_other.go —— 非 unix 平台的**如实降级**（本仓只在 darwin/linux 上跑，这一支是编译兜底）。
//
// 口径：没有 `kill(pid,0)`，也**不许**拿「读不到」当「死了」⇒ 一律返回 false 并让调用方走
// 「不判」分支（`PatrolResidency` 的 pid<=0 / 未知口径同一精神）—— 缺口 `GAP-20261018-13` 的
// 判据不会在没有进程探针的平台上凭空判一枚卵 vanished（宁可判不了，不可误报掉线）。
package backend

import (
	"fmt"
	"net"
	"time"
)

const residencyPortDialTimeout = 200 * time.Millisecond

// pidAlive 非 unix 平台没有进程探针 ⇒ **恒 false**（调用方据此落「不判」）。
func pidAlive(pid int) bool { return false }

// portAlive 拨号这一条是跨平台的（net 标准库），照常可用。
func portAlive(port int) bool {
	if port <= 0 {
		return false
	}
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), residencyPortDialTimeout)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
