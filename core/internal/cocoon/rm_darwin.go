//go:build darwin

// rm_darwin.go — macOS 平台的删除原语：每层 openat(O_NOFOLLOW_ANY) + fstat 比 st_dev。
//
// 口径（v3 §B）：macOS **没有** openat2，也没有 `NO_XDEV` 的等价物。等价物这样拼出来：
//
//	① 不跟链接：每层 `O_NOFOLLOW_ANY`（10.16/macOS 11+）——它管的是**整条路径**，
//	  不是只有末分量，等价于 Linux 的 RESOLVE_NO_SYMLINKS；
//	② 不跨挂载：没有内核侧的开关 ⇒ 每层 fstat 取 st_dev，与茧根的 st_dev 不同即拒
//	  （比对在 rm.go 的 removeChildren 里做，本文件只负责把 st_dev 报上去）；
//	③ 不出茧根：每层只许一个不含 / 的名字（validEntryName 挡）+ O_NOFOLLOW_ANY。
//
// 删除只用 unix.Unlinkat(dirFd, name, 0|AT_REMOVEDIR)——永不拼路径，与 Linux 同一套。
//
// 本文件与 rm_linux.go 同名的几个小原语（statFD/listNames/removeEntry/closeNode/openBase）
// 逐字一致，只差 open 的 flags。故意不抽公共层：安全面要能"单文件读全"，
// 少一层间接就少一次"平台差异被抽没了"的机会。
package cocoon

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

// darwinFlags — 每层统一的两条：末段（NOFOLLOW_ANY 之下是整条路径）不跟链接 + close-on-exec。
const darwinFlags = unix.O_CLOEXEC | unix.O_NOFOLLOW_ANY

// openBase — 打开茧根父目录（唯一一次按路径打开；base 自身允许是链接，约束从茧根往下生效）。
func openBase(path string) (node, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return node{}, fmt.Errorf("cocoon: 打开茧根父目录 %q 失败: %w", path, err)
	}
	return node{fd: fd}, nil
}

// openRootAt — 开茧根本体（一层，fstat 取身份）。
func openRootAt(parent node, name string) (node, RootID, error) {
	fd, err := unix.Openat(parent.fd, name, darwinFlags|unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return node{}, RootID{}, rootErr(name, err)
	}
	id, err := statFD(fd)
	if err != nil {
		unix.Close(fd)
		return node{}, RootID{}, fmt.Errorf("cocoon: fstat 茧根 %q 失败: %w", name, err)
	}
	return node{fd: fd}, id, nil
}

// openSub — 开子目录（一层，单分量名字）。非目录返回 errNotDir ⇒ 上层只删名字。
func openSub(dir node, name string) (node, RootID, error) {
	fd, err := unix.Openat(dir.fd, name, darwinFlags|unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return node{}, RootID{}, subErr(name, err)
	}
	id, err := statFD(fd)
	if err != nil {
		unix.Close(fd)
		return node{}, RootID{}, fmt.Errorf("cocoon: fstat %q 失败: %w", name, err)
	}
	return node{fd: fd}, id, nil
}

// rootErr / subErr — errno → 语义错误。macOS 上 O_NOFOLLOW_ANY 遇链接报 ELOOP，
// 再叠 O_DIRECTORY 时可能先报 ENOTDIR；两种都当"不是目录"处理（照样拒删/删名字）。
func rootErr(name string, err error) error {
	switch {
	case errors.Is(err, unix.ELOOP), errors.Is(err, unix.ENOTDIR):
		return fmt.Errorf("%w: %q（符号链接或非目录）: %v", ErrRootNotDir, name, err)
	case errors.Is(err, unix.EXDEV):
		return fmt.Errorf("%w: 茧根 %q 在别的挂载点上", ErrCrossMount, name)
	default:
		return fmt.Errorf("cocoon: 打开茧根 %q 失败: %w", name, err)
	}
}

func subErr(name string, err error) error {
	switch {
	case errors.Is(err, unix.ELOOP), errors.Is(err, unix.ENOTDIR):
		return fmt.Errorf("%w: %q: %v", errNotDir, name, err)
	case errors.Is(err, unix.EXDEV):
		return fmt.Errorf("%w: %q 在别的挂载点上", ErrCrossMount, name)
	default:
		return fmt.Errorf("cocoon: 打开 %q 失败: %w", name, err)
	}
}

// statFD — fstat 取身份（st_dev 就是"是不是同一个挂载点"的判据）。
func statFD(fd int) (RootID, error) {
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return RootID{}, err
	}
	return RootID{Dev: uint64(uint32(st.Dev)), Ino: st.Ino}, nil
}

// listNames — 只拿名字，判型交给 openat：d_type 可能是 DT_UNKNOWN，不能拿它当安全判据。
func listNames(n node) ([]string, error) {
	buf := make([]byte, 8192)
	var names []string
	for {
		nb, err := unix.ReadDirent(n.fd, buf)
		if err != nil {
			return nil, err
		}
		if nb <= 0 {
			return names, nil
		}
		chunk := buf[:nb]
		for len(chunk) > 0 {
			consumed, _, ns := unix.ParseDirent(chunk, -1, names)
			if consumed <= 0 {
				return nil, fmt.Errorf("cocoon: 目录项解析失败（剩 %d 字节）", len(chunk))
			}
			names, chunk = ns, chunk[consumed:]
		}
	}
}

// removeEntry — 唯一的删除手段：fd 相对 unlinkat，永不拼路径。
func removeEntry(n node, name string, isDir bool) error {
	flags := 0
	if isDir {
		flags = unix.AT_REMOVEDIR
	}
	return unix.Unlinkat(n.fd, name, flags)
}

// closeNode — 关目录 fd（rmdir 之前必须先关）。
func closeNode(n node) error {
	if n.fd <= 0 {
		return nil
	}
	return unix.Close(n.fd)
}

// degradeNote — macOS 无降级档：没有 openat2 可退，O_NOFOLLOW_ANY 恒可用（macOS 11+）。
// 缺 NO_XDEV 这一项用每层的 st_dev 比对补，故不算"放弃防护"，返回空串。
func degradeNote() string { return "" }
