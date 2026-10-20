//go:build linux

// rm_linux.go — Linux 平台的删除原语：openat2 + RESOLVE_* + unlinkat。
//
// 口径（v3 §B）：开根用 `O_PATH|O_DIRECTORY|O_NOFOLLOW` 配
// `RESOLVE_NO_SYMLINKS|RESOLVE_NO_XDEV|RESOLVE_BENEATH`；逐层用
// `O_RDONLY|O_DIRECTORY|O_CLOEXEC|O_NOFOLLOW` 配同一套 resolve；删除只用
// `unix.Unlinkat(dirFd, name, 0|AT_REMOVEDIR)`——**永不拼路径**。
//
// 降级（内核 <5.6 / seccomp 拦 ENOSYS|EPERM）：逐分量 `openat(O_NOFOLLOW)` + 自比 st_dev，
// 并且**显式声明放弃 bind-mount 防护**（bind mount 与被挂载点同 st_dev，只有 NO_XDEV 拦得住）。
// 声明落在 degrade() 的原话里，经 RemoveReport.Degraded 与每条错误消息出去。
package cocoon

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sys/unix"
)

// linuxResolve — 三层防护一次给全（见 rm.go 抬头 ★三层防护）。
const linuxResolve = unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_XDEV | unix.RESOLVE_BENEATH

// linuxNoFollow — 每层都要的两条：close-on-exec + 末分量不跟链接。
const linuxNoFollow = unix.O_CLOEXEC | unix.O_NOFOLLOW

// openat2 — 唯一一处给 openat2 收口的地方（flags 由调用方拼，resolve 恒为本平台那三条）。
func openat2At(dirFd int, name string, flags int) (int, error) {
	how := unix.OpenHow{Flags: uint64(flags), Resolve: linuxResolve}
	return unix.Openat2(dirFd, name, &how)
}

// openBase — 打开茧根父目录（base 由配置给出，是**唯一**一次按路径打开；
// 之后一切都在它底下 fd 相对进行）。base 自身可以是链接（如 /var → /private/var），
// 不套 NOFOLLOW：约束从茧根往下的每一层生效，base 之上的路径不值得也不该在这里判。
func openBase(path string) (node, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return node{}, fmt.Errorf("cocoon: 打开茧根父目录 %q 失败: %w", path, err)
	}
	return node{fd: fd}, nil
}

// openRootAt — 开茧根本体：O_PATH 取 fd → fstat → 以该 fd 为 dirfd 重开 "."（O_RDONLY）供 readdir。
func openRootAt(parent node, name string) (node, RootID, error) {
	if degraded() != "" {
		return openRootDegraded(parent, name)
	}

	pfd, err := openat2At(parent.fd, name, linuxNoFollow|unix.O_PATH|unix.O_DIRECTORY)
	if err != nil {
		return node{}, RootID{}, rootErr(name, err)
	}
	defer unix.Close(pfd) // O_PATH fd 只用来 fstat / 当 dirfd，用完即关。

	pin, err := statFD(pfd)
	if err != nil {
		return node{}, RootID{}, fmt.Errorf("cocoon: fstat 茧根 %q 失败: %w", name, err)
	}

	// O_PATH 的 fd 不能 readdir ⇒ 再开一个可读的。★ 以 pfd 为 dirfd 开 "."，不是拿名字
	// 重新解析一遍：按名字重解等于把身份又绑回名字上，前面的 fstat 就白做了。
	rfd, err := openat2At(pfd, ".", linuxNoFollow|unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil {
		return node{}, RootID{}, fmt.Errorf("cocoon: 以 O_PATH fd 重开茧根 %q 失败: %w", name, err)
	}
	rid, err := statFD(rfd)
	if err != nil {
		unix.Close(rfd)
		return node{}, RootID{}, fmt.Errorf("cocoon: fstat 茧根（可读 fd）失败: %w", err)
	}
	if rid != pin {
		unix.Close(rfd)
		return node{}, RootID{}, fmt.Errorf("%w: %q（O_PATH %s → 可读 %s）", ErrRootChanged, name, pin, rid)
	}
	return node{fd: rfd}, rid, nil
}

// openRootDegraded — 降级档开根：openat(O_NOFOLLOW) + fstat（跨挂载由 rm.go 的 st_dev 比对兜底）。
func openRootDegraded(parent node, name string) (node, RootID, error) {
	fd, err := unix.Openat(parent.fd, name, linuxNoFollow|unix.O_RDONLY|unix.O_DIRECTORY, 0)
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

// openSub — 开子目录（一层，单分量名字）。
//
// 非目录（链接/普通文件/设备/fifo）返回 errNotDir：上层据此删**名字**，绝不跟过去。
func openSub(dir node, name string) (node, RootID, error) {
	var (
		fd  int
		err error
	)
	if degraded() != "" {
		fd, err = unix.Openat(dir.fd, name, linuxNoFollow|unix.O_RDONLY|unix.O_DIRECTORY, 0)
	} else {
		fd, err = openat2At(dir.fd, name, linuxNoFollow|unix.O_RDONLY|unix.O_DIRECTORY)
	}
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

// rootErr / subErr — 把 errno 翻成上层的语义错误。EXDEV 一律 ErrCrossMount（fail-closed）。
func rootErr(name string, err error) error {
	switch {
	case errors.Is(err, unix.EXDEV):
		return fmt.Errorf("%w: 茧根 %q 在别的挂载点上", ErrCrossMount, name)
	case errors.Is(err, unix.ELOOP), errors.Is(err, unix.ENOTDIR):
		return fmt.Errorf("%w: %q（符号链接或非目录）: %v", ErrRootNotDir, name, err)
	default:
		return fmt.Errorf("cocoon: openat2 打开茧根 %q 失败: %w", name, err)
	}
}

func subErr(name string, err error) error {
	switch {
	case errors.Is(err, unix.ELOOP), errors.Is(err, unix.ENOTDIR):
		// 链接 / 普通文件 / 设备 / fifo：签名成"非目录"，由上层删名字。
		return fmt.Errorf("%w: %q: %v", errNotDir, name, err)
	case errors.Is(err, unix.EXDEV):
		return fmt.Errorf("%w: %q 在别的挂载点上", ErrCrossMount, name)
	default:
		return fmt.Errorf("cocoon: 打开 %q 失败: %w", name, err)
	}
}

// statFD — fstat 取身份（O_PATH 与 O_RDONLY 的 fd 都合法）。
func statFD(fd int) (RootID, error) {
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return RootID{}, err
	}
	return RootID{Dev: uint64(st.Dev), Ino: st.Ino}, nil
}

// listNames — 读目录里的名字（不含 . 与 ..）。只拿名字，判型交给 openat2：
// d_type 在部分文件系统上是 DT_UNKNOWN，靠它判型等于把安全判据交给一个不可靠字段。
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

// closeNode — 关目录 fd（rmdir 之前必须先关，见 rm.go removeChildren）。
func closeNode(n node) error {
	if n.fd <= 0 {
		return nil
	}
	return unix.Close(n.fd)
}

var (
	degradeOnce sync.Once
	degradeWhy  string
)

// degraded — openat2 能力探测（一次，结果粘住）。"" = 全防护可用。
//
// 探测刻意**不带 resolve 位**、打 "/"：seccomp 拦的是系统调用号（拦了就是 EPERM），
// 5.6 的内核认这三条 resolve；不带位探测的是"这个系统调用能不能进"。
func degraded() string {
	degradeOnce.Do(func() {
		how := unix.OpenHow{Flags: uint64(unix.O_PATH | unix.O_CLOEXEC | unix.O_DIRECTORY)}
		fd, err := unix.Openat2(unix.AT_FDCWD, "/", &how)
		if err == nil {
			unix.Close(fd)
			return
		}
		degradeWhy = fmt.Sprintf(
			"openat2 不可用（%v）⇒ 已降级为逐分量 openat(O_NOFOLLOW)+自比 st_dev；"+
				"**放弃 bind-mount 防护**（bind mount 与被挂载点同 st_dev，比不出来，只有 RESOLVE_NO_XDEV 拦得住）",
			err)
	})
	return degradeWhy
}

// degradeNote — rm.go 要的降级声明（恒等于 degraded() 的原话）。
func degradeNote() string { return degraded() }
