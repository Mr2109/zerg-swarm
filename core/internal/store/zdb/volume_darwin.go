//go:build darwin

package zdb

import (
	"fmt"
	"syscall"
)

// devOf —— 路径所在卷的设备号（macOS Stat_t.Dev 为 int32）。
func devOf(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Dev), nil
}

// cstrInt8 —— 把 syscall 的 [N]int8 定长数组当 C 串读成 Go string。
func cstrInt8(b []int8) string {
	n := len(b)
	for i := 0; i < len(b); i++ {
		if b[i] == 0 {
			n = i
			break
		}
	}
	out := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, byte(b[i]))
	}
	return string(out)
}

// readVolumeFacts —— 采集判卷事实（硬规矩 ① 唯一碰盘处）。
// macOS 上 syscall.Statfs_t 自带 Mntonname / Fstypename ⇒ 挂载点与文件系统类型一步到位。
func readVolumeFacts(dbPath string) (volumeFacts, error) {
	dir, err := nearestExistingDir(dbPath)
	if err != nil {
		return volumeFacts{}, err
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return volumeFacts{}, fmt.Errorf("%w：探测卷失败（%s）: %v", ErrVolume, dir, err)
	}
	mountDev, err := devOf(dir)
	if err != nil {
		return volumeFacts{}, fmt.Errorf("%w：探测卷设备号失败（%s）: %v", ErrVolume, dir, err)
	}
	bootDev, err := devOf("/")
	if err != nil {
		return volumeFacts{}, fmt.Errorf("%w：探测根卷设备号失败: %v", ErrVolume, err)
	}
	return volumeFacts{
		Path:     dbPath,
		Mount:    cstrInt8(st.Mntonname[:]),
		MountDev: mountDev,
		BootDev:  bootDev,
		FSType:   cstrInt8(st.Fstypename[:]),
	}, nil
}

// statfsFreeBytes —— 路径所在卷的**可用**字节数（备份前空间预检 §六 Q24）。
// Bavail（非特权用户可用块）× Bsize。
func statfsFreeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
