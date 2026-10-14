//go:build linux

package eggstore

import (
	"golang.org/x/sys/unix"
)

// fallbackErrnos ★ 「本档不支持 ⇒ 允许回退」的五档 errno（linux 面）。
func fallbackErrnos() []error {
	return []error{unix.ENOTSUP, unix.EOPNOTSUPP, unix.EXDEV, unix.ENOSYS, unix.ENOTTY}
}

// platformCloneWhole Linux 整文件 `FICLONE`（XFS/Btrfs/… 支持；ext4 等 ⇒ `EOPNOTSUPP` ⇒ 回退）。
//
// ★ `FICLONE` 要求目标为**新建空件**，源/目标**同一文件系统**；ioctl 形态
// `ioctl(dest_fd, FICLONE, src_fd)`。
func platformCloneWhole(dst, src string) (CloneTier, error) {
	dfd, err := unix.Open(dst, unix.O_CREAT|unix.O_WRONLY|unix.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	defer func() { _ = unix.Close(dfd) }()
	sfd, err := unix.Open(src, unix.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = unix.Close(sfd) }()
	if err := unix.IoctlFileClone(dfd, sfd); err != nil {
		return "", err
	}
	return TierReflink, nil
}

// platformCloneRange ★ Linux **段级 reflink**：`FICLONERANGE` —— 把卵里那一段（页对齐）范围
// reflink 到目标 ⇒ 与卵**共享块**、零字节拷贝。
//
// 前提/代价：源与目标**同一文件系统**（跨 FS ⇒ `EXDEV`）；XFS/Btrfs 要求 offset/length
// **块对齐**（卵段页对齐 ⇒ 满足）。
func platformCloneRange(dst string, dstOff int64, src string, srcOff, srcLen int64) (CloneTier, error) {
	dfd, err := unix.Open(dst, unix.O_CREAT|unix.O_WRONLY|unix.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	defer func() { _ = unix.Close(dfd) }()
	sfd, err := unix.Open(src, unix.O_RDONLY, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = unix.Close(sfd) }()
	r := &unix.FileCloneRange{
		Src_fd:      int64(sfd),
		Src_offset:  uint64(srcOff),
		Src_length:  uint64(srcLen),
		Dest_offset: uint64(dstOff),
	}
	if err := unix.IoctlFileCloneRange(dfd, r); err != nil {
		return "", err
	}
	return TierReflinkRange, nil
}

// platformCopyRange Linux `copy_file_range(2)`（内核直拷 · 同 FS 内）。
//
// length==0 ⇒ **到源 EOF**（整文件档）；否则拷固定长度（段档）。逐段循环（单次 ≤1 GiB）。
func platformCopyRange(dst string, dstOff int64, src string, srcOff int64, length int64) (int64, error) {
	sfd, err := unix.Open(src, unix.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer func() { _ = unix.Close(sfd) }()
	dfd, err := unix.Open(dst, unix.O_CREAT|unix.O_WRONLY|unix.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	defer func() { _ = unix.Close(dfd) }()
	var total int64
	ro, wo := srcOff, dstOff
	remaining := length // 0 = 到 EOF
	for {
		chunk := int64(1 << 30)
		if remaining > 0 && remaining < chunk {
			chunk = remaining
		}
		n, cerr := unix.CopyFileRange(int(sfd), &ro, int(dfd), &wo, int(chunk), 0)
		total += int64(n)
		if cerr != nil {
			if cerr == unix.EINTR {
				continue
			}
			return total, cerr
		}
		if n == 0 { // EOF
			break
		}
		if remaining > 0 {
			remaining -= int64(n)
			if remaining <= 0 {
				break
			}
		}
	}
	return total, nil
}

// hardlinkCount 现读硬链数（`Stat_t.Nlink`）。
func hardlinkCount(path string) (uint64, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Nlink), nil
}
