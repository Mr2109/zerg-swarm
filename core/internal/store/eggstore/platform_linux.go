//go:build linux

package eggstore

import (
	"fmt"
	"syscall"
)

// rawVolume 平台原始读数。
type rawVolume struct {
	bsize, blocks, bfree, bavail, files, ffree uint64
	mount, fstype                              string
}

// statfsRaw Linux：可用块取 **`Bavail`**（＝ `statvfs.f_bavail`，非特权可用）。
// ★ 硬线要求「`f_bavail` ＋ **独立**查 `f_favail`」—— 这里 `Ffree` 就是 `f_favail`
// （Linux `statfs.f_ffree` 对应用户态 `statvfs.f_favail`），**不与 Bavail 同源推导**。
// ★ 挂载点与文件系统类型**不报**：`Statfs_t` 在 Linux 上没有 `Mntonname`/`Fstypename`
// ⇒ 留空并出声，✗ 不假装判过（与 `core/internal/store/zdb/volume_other.go` 同口径）。
func statfsRaw(path string) (rawVolume, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return rawVolume{}, fmt.Errorf("statfs(%s): %w", path, err)
	}
	return rawVolume{
		bsize: uint64(st.Bsize), blocks: st.Blocks, bfree: st.Bfree,
		bavail: st.Bavail, files: st.Files, ffree: st.Ffree,
	}, nil
}

// deviceOf 卷设备号（Linux `Stat_t.Dev` 是 uint64）。
func deviceOf(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, fmt.Errorf("stat(%s): %w", path, err)
	}
	return uint64(st.Dev), nil
}

// platformAPFSProbe 非 macOS：APFS 不适用（✗ 不假装有）。
func platformAPFSProbe(mount string) (*uint64, string, string) {
	return nil, "", "本平台不适用（APFS 是 macOS 专有；容器池概念在 Linux 侧由 quota/卷管理器承载）"
}
