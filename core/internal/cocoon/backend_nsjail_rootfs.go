//go:build linux

// backend_nsjail_rootfs.go — 真 rootfs 的**动手**那一半（linux 专属）：overlay 挂载/卸载、
// 茧内最小目录与设备文件、挂载点判定。
//
// ★ 为什么单独一件且带 build tag：mount(2) 只有 linux 的 syscall 包里有；而「方案算得对不对」
// 必须能在开发机（macOS）上就判 ⇒ 方案/argv 拼装在 backend_nsjail_shared.go，动手在这里。
//
// ★ 一条铁律：这里**没有一个函数**在失败时退回「宿主根」。挂不上 overlay 就让 Exec 报错
// （宁红不假绿）—— 静默退回会把「茧里跑的是宿主二进制」伪装成「接了真 rootfs」。
package cocoon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// nsjailOverlaySetup — 把 lower=rootfs(只读) + upper(可写) 合成到 merged；**幂等**（多服务共用一次挂载）。
//
// work 与 upper 必须同盘（内核要求）；本层把两者都放在茧目录下，所以只要茧目录这一处
// 选得合适就成立。upper 所在文件系统必须支持作为 overlay 上层：tmpfs/ext4/xfs 可以，
// **overlay-on-overlay 不行**（实测 docker 容器里 /tmp 就是 overlayfs ⇒ 内核报
// 「filesystem on ... not supported as upperdir」，这时得把茧目录放到 tmpfs/卷上）。
func nsjailOverlaySetup(p nsjailRootfsPlan) error {
	if p.Rootfs == "" || p.Merged == "" || p.Upper == "" || p.Work == "" {
		return fmt.Errorf("cocoon: overlay 方案不完整（rootfs=%q upper=%q work=%q merged=%q）",
			p.Rootfs, p.Upper, p.Work, p.Merged)
	}
	// 已经挂过 ⇒ 直接返回：一座茧会有好几次 exec（多个服务），overlay 只该挂一次。
	if nsjailIsMountpoint(p.Merged) {
		return nil
	}
	for _, d := range []string{p.Upper, p.Work, p.Merged, p.TmpDir} {
		if err := os.MkdirAll(d, nsjailRootMode); err != nil {
			return fmt.Errorf("cocoon: 建 overlay 目录 %s 失败: %w", d, err)
		}
	}
	opts := "lowerdir=" + p.Rootfs + ",upperdir=" + p.Upper + ",workdir=" + p.Work
	if err := syscall.Mount("overlay", p.Merged, "overlay", 0, opts); err != nil {
		return fmt.Errorf("cocoon: overlay 挂载失败（%s）: %w —— 不退回宿主根（宁红不假绿）；"+
			"upper/work 所在的盘必须能当 overlay 上层（tmpfs/ext4/xfs 可以，overlay-on-overlay 不行）",
			opts, err)
	}
	return nil
}

// nsjailOverlayTeardown — 卸掉某座茧的合并视图（幂等：没挂过 ⇒ nil）。
func nsjailOverlayTeardown(p nsjailRootfsPlan) error { return nsjailOverlayTeardownAt(p.Merged) }

// nsjailOverlayTeardownAt — 卸一个挂载点。**只卸自己算出来的那个路径**（调用方给的 merged），
// 且只在它真的是挂载点时动手：拿一个不存在的目录去 unmount，内核只会告诉我们 EINVAL。
func nsjailOverlayTeardownAt(merged string) error {
	if merged == "" || !nsjailIsMountpoint(merged) {
		return nil
	}
	// MNT_DETACH（懒卸）：茧里可能还有残留进程攥着文件；懒卸保证命令面不会因为
	// 「有人还开着文件」而永远卸不掉，同时该挂载点立刻从视图里消失。
	if err := syscall.Unmount(merged, syscall.MNT_DETACH); err != nil &&
		!errors.Is(err, syscall.EINVAL) && !errors.Is(err, syscall.ENOENT) {
		return fmt.Errorf("cocoon: 卸 overlay %s 失败: %w", merged, err)
	}
	return nil
}

// nsjailEnsureJailDirs — 补齐 bind mount 的**目标**（mount(2) 要求目标已存在）。
//
// 合并视图是可写的（overlay），所以缺的目录/设备占位会落进 upper —— 这正是我们想要的：
// 镜像本身一个字节没改，茧里该有的面在视图里齐了。
//
// ★ 设备名必须是**普通件**：nsjail 对「源是文件」的 bind 会在目标上做 O_CREAT，
// 目标若是目录会直接失败（实测报 `open('/run/user/nsjail.0.root//tmp', O_CREAT): Is a directory`
// —— 那条报错说的是 /tmp，真因是当时源路径不存在、被当成了文件）。
func nsjailEnsureJailDirs(p nsjailRootfsPlan) error {
	for _, d := range []string{"proc", "dev", "tmp"} {
		target := filepath.Join(p.Chroot, d)
		if err := os.MkdirAll(target, 0o755); err != nil {
			return fmt.Errorf("cocoon: 建茧内目录 %s 失败: %w", target, err)
		}
	}
	for _, f := range []string{"null", "zero", "urandom", "random"} {
		target := filepath.Join(p.Chroot, "dev", f)
		if _, err := os.Lstat(target); err == nil {
			continue
		}
		fh, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("cocoon: 建茧内设备占位 %s 失败: %w", target, err)
		}
		if cerr := fh.Close(); cerr != nil {
			return fmt.Errorf("cocoon: 关茧内设备占位 %s 失败: %w", target, cerr)
		}
	}
	return nil
}

// nsjailIsMountpoint — dir 是不是一个已挂载点（读 /proc/self/mountinfo 的第 5 列）。
//
// ★ 不用「目录非空 / stat 两次比 dev」这类近似判据：overlay 合成出来的视图与普通目录
// 在这些判据下长得一样，误判的后果是「该挂的没挂」或「卸掉别人的挂载」。mountinfo 是内核
// 自己写的表，第 5 列就是挂载点（**已按 octal 转义**，空格写成 \040 —— 茧目录常带空格，
// 不反转义就永远比不上）。
func nsjailIsMountpoint(dir string) bool {
	if dir == "" {
		return false
	}
	want := filepath.Clean(dir)
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		if nsjailUnescapeMountinfo(f[4]) == want {
			return true
		}
	}
	return false
}

// nsjailUnescapeMountinfo — mountinfo 的 octal 转义（\040 空格 · \011 制表 · \012 换行 · \134 反斜杠）。
func nsjailUnescapeMountinfo(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 <= len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
