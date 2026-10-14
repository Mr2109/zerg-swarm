//go:build darwin

package eggstore

import "golang.org/x/sys/unix"

// fallbackErrnos ★ 「本档不支持 ⇒ 允许回退」的五档 errno（darwin 面）。
func fallbackErrnos() []error {
	return []error{unix.ENOTSUP, unix.EOPNOTSUPP, unix.EXDEV, unix.ENOSYS, unix.ENOTTY}
}

// platformCloneWhole macOS 整文件 `clonefile(2)`。
//
// ★ 语义：克隆与原文件**共享物理块**（APFS CoW）⇒ `df` 不涨；这是 macOS 侧唯一的一档
// 「毫秒级」—— ✗ **只对整文件**成立（见下 `platformCloneRange`）。
func platformCloneWhole(dst, src string) (CloneTier, error) {
	// flags=0；`Clonefile` 要求目标**不存在**（存在 ⇒ EEXIST，非「不支持」档 ⇒ 上抛）。
	if err := unix.Clonefile(src, dst, 0); err != nil {
		return "", err
	}
	return TierClonefile, nil
}

// platformCloneRange ★★ macOS **没有段级 reflink** —— `clonefile(2)/clonefileat/fclonefileat`
// 一律**整文件**、无 `FICLONERANGE` 等价物（`copyfile(3)` 的 `COPYFILE_CLONE` 亦是整文件/目录树）。
//
// ⇒ 本函数**恒返回 `ErrNoRangeClone`**，调用方据此退真拷贝 ⇒ 卵内**段范围**取用在 macOS 上
// **是一次真实拷贝**（同量级秒，**✗ 不是毫秒**）。这是本单刻意写进件内的口径，✗ 不许改口。
func platformCloneRange(dst string, dstOff int64, src string, srcOff, srcLen int64) (CloneTier, error) {
	return "", ErrNoRangeClone
}

// platformCopyRange macOS **无 `copy_file_range(2)`** ⇒ 不适用（退 `io.Copy`）。
func platformCopyRange(dst string, dstOff int64, src string, srcOff, srcLen int64) (int64, error) {
	return 0, ErrCopyRangeUnsupported
}

// hardlinkCount 现读硬链数（`Stat_t.Nlink`）—— 硬链共享判据（U4）。
func hardlinkCount(path string) (uint64, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Nlink), nil
}
