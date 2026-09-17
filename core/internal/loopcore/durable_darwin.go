//go:build darwin

// durable_darwin.go — T5.5：macOS 的"写就落盘"实现。
//
// 为什么不能只调 f.Sync()：macOS 上普通 fsync 只把数据交给**磁盘缓存**（掉电仍会丢），
// 要真正落盘得走 F_FULLFSYNC（与 internal/memory 的写入安全层同一口径、同一理由）。
// darwin 的 syscall 包没有 FcntlInt（那是 linux 的），所以走 SYS_FCNTL 原始调用；
// 不支持该命令的文件系统（部分网络卷）回退普通 fsync —— 不因"同步强度"问题让写入失败。
//
// 这正是 sync 档与 async 档的**唯一分界**：sync 档每步都经过这里，async 档一次都不经过。
package loopcore

import (
	"os"
	"syscall"
)

func fullSync(f *os.File) error {
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), uintptr(syscall.F_FULLFSYNC), 0)
	if errno != 0 {
		return f.Sync()
	}
	return nil
}
