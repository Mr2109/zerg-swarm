//go:build unix

// residency_proc_unix.go —— 巡检用的两枚**只读探针**（进程在不在 / 端口在不在听）。
//
// 口径（保守方向，写死）：
//
//	· 进程：`kill(pid, 0)` —— 返回 nil 或 EPERM（进程在、只是不归我）都算**在活**；
//	  ESRCH ⇒ 不在。**只有明确内核说没有**才算死（同 `unit_liveness.go`「只有明确读到
//	  Terminal 态才算死了」）。pid <= 0 ⇒ 根本不问（「不知道」不当「死了」）。
//	· 端口：对 `127.0.0.1:<port>` 拨一次（200ms 上限）。子端引擎一律只听回环
//	  （见 `eggBackendProbeLive` 的注释），所以这一拨就是「它那一面还活着吗」的最小实测。
package backend

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

// residencyPortDialTimeout 单次端口拨号的超时（回环 ⇒ 200ms 足够；宁可判「不听」也不拖慢巡检）。
const residencyPortDialTimeout = 200 * time.Millisecond

// pidAlive 该 pid 此刻是否在活（unix 实现）。
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM：进程在、只是不是我名下的（巡检只读，不因权限改判）。
	return errors.Is(err, syscall.EPERM)
}

// portAlive `127.0.0.1:<port>` 此刻是否有人听（拨号成功即算）。
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
