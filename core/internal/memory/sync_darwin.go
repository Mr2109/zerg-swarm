//go:build darwin

// sync_darwin.go — macOS:优先 F_FULLFSYNC(真正把数据刷到物理介质)。
// 说明:普通 fsync 在 macOS 上只把数据交给磁盘缓存,F_FULLFSYNC 才请求硬件确认(设计稿 §9.2 建议)。
// darwin 的 syscall 包没有 FcntlInt(那是 linux 的),所以走 SYS_FCNTL 原始调用;
// 不支持该命令的文件系统(如部分网络卷)回退普通 fsync,不因同步强度问题让写入失败。
package memory

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
