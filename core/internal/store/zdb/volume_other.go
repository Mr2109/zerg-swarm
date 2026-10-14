//go:build !darwin

package zdb

import (
	"fmt"
	"syscall"
)

// devOf —— 路径所在卷的设备号（非 darwin 平台 Stat_t.Dev 多为 uint64）。
func devOf(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Dev), nil
}

// readVolumeFacts —— 非 darwin 平台的采集：只给设备号，FSType 留空（跳过盘型档）。
// 硬规矩 ① 的「卷未挂载」判据靠设备号比对，与平台无关；盘型附加约束（须 APFS/HFS+）
// 是 macOS 专有约束（syscall.Statfs_t 在 linux 无 Fstypename），此处如实留空、✗ 不假装判过。
func readVolumeFacts(dbPath string) (volumeFacts, error) {
	dir, err := nearestExistingDir(dbPath)
	if err != nil {
		return volumeFacts{}, err
	}
	mountDev, err := devOf(dir)
	if err != nil {
		return volumeFacts{}, fmt.Errorf("%w：探测卷失败（%s）: %v", ErrVolume, dir, err)
	}
	bootDev, err := devOf("/")
	if err != nil {
		return volumeFacts{}, fmt.Errorf("%w：探测根卷设备号失败: %v", ErrVolume, err)
	}
	return volumeFacts{
		Path:     dbPath,
		Mount:    "/",
		MountDev: mountDev,
		BootDev:  bootDev,
		FSType:   "",
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
