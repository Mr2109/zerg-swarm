//go:build linux

// baseline_proc_linux.go —— Linux 侧只读进程事实（P4 退场清理后的残余保留面）。
//
// 设计依据：设计-子端沙箱化-20260914.md §10.1 baseline_proc_linux.go 行——
// 保留：readDrmMemoryGb / readProcessMemGb（显存账逐进程归因用，P3）、
// findListenerProcess / tcpListenInode / pidOfSocketInode（**只读**发现"谁占着"，§2.2 E4）。
// 退场：readArgv / readStartTime / currentStartTime / collectAncestry 与
// procInfo 的存档用途（停前存档 + 托管方式分类没有对象了——附录 C·C2）。
//
// 全部只读（/proc 读取），绝不发送信号、绝不改状态。
package backend

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// procInfo 一个已定位进程的事实集合（只读发现"谁占着"；不再承载停前存档字段）。
type procInfo struct {
	PID    int
	Argv   []string
	Cgroup string  // /proc/<pid>/cgroup 原文
	MemGB  float64 // 实测占用（GB）：GPU 宿主取 drm fdinfo 的 gtt+vram，取不到才回落 RSS
}

// readProcessMemGb 取一个推理进程的**真实占用**（GB）。
//
// 为什么不能用 RSS：权重在 GTT/VRAM 里，RSS 只统计 CPU 侧页。X3 实测——
// Qwen 32.7 GB / K2 24.0 GB 的服务，RSS 只报 1.41 / 0.77 GB（差 20 倍以上）
// ⇒ 用它做准入扣减几乎失效（M15）。
// 真源：/proc/<pid>/fdinfo/<fd> 的 drm-memory-gtt + drm-memory-vram（amdgpu 逐进程归因；
// 实测两者之和 = 全局 mem_info_gtt_used，归因正确）。取不到（非 GPU / 无权限）回落 RSS。
func readProcessMemGb(pid int) float64 {
	if gb := readDrmMemoryGb(pid); gb > 0 {
		return gb
	}
	return readProcessRssGb(pid)
}

// readDrmMemoryGb 累加 pid 所有 fd 的 drm-memory-gtt + drm-memory-vram（GB）。取不到返回 0。
func readDrmMemoryGb(pid int) float64 {
	paths, err := filepath.Glob(filepath.Join("/proc", strconv.Itoa(pid), "fdinfo", "*"))
	if err != nil || len(paths) == 0 {
		return 0
	}
	var total float64
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		total += parseDrmMemoryGb(string(b))
	}
	return total
}

// parseDrmMemoryGb 的实现在 baseline_mem.go（无平台标签，两平台都能单测）。

// findListenerProcess 定位监听 127.0.0.1:port 的进程。找不到返回 (零值,false)。
func findListenerProcess(port int) (procInfo, bool) {
	inode := tcpListenInode(port)
	if inode == "" {
		return procInfo{}, false
	}
	pid := pidOfSocketInode(inode)
	if pid <= 0 {
		return procInfo{}, false
	}
	info := procInfo{PID: pid}
	info.Argv = readArgv(pid)
	if b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cgroup")); err == nil {
		info.Cgroup = string(b)
	}
	info.MemGB = readProcessMemGb(pid)
	return info, len(info.Argv) > 0
}

// tcpListenInode 从 /proc/net/tcp{,6} 里找本地端口在 LISTEN 状态的 socket inode。
func tcpListenInode(port int) string {
	want := strings.ToUpper(strconv.FormatInt(int64(port), 16))
	if len(want)%2 == 1 {
		want = "0" + want
	}
	for _, p := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(b), "\n") {
			if i == 0 { // 表头
				continue
			}
			f := strings.Fields(line)
			if len(f) < 10 {
				continue
			}
			// f[1]=local_address(HEXIP:HEXPORT) f[3]=state(0A=LISTEN) f[9]=inode
			if !strings.EqualFold(f[3], "0A") {
				continue
			}
			addr := f[1]
			if j := strings.LastIndexByte(addr, ':'); j >= 0 {
				if strings.EqualFold(addr[j+1:], want) {
					return f[9]
				}
			}
		}
	}
	return ""
}

// pidOfSocketInode 在 /proc/*/fd/* 里找指向该 socket inode 的进程（只读）。
func pidOfSocketInode(inode string) int {
	target := "socket:[" + inode + "]"
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		fdDir := filepath.Join("/proc", e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // 别的用户的进程读不到——跳过即可（不报错）
		}
		for _, fd := range fds {
			if readLink(filepath.Join(fdDir, fd.Name())) == target {
				return pid
			}
		}
	}
	return 0
}

// readArgv 读 /proc/<pid>/cmdline（NUL 分隔）——只读身份判别用。
func readArgv(pid int) []string {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return nil
	}
	parts := strings.Split(strings.TrimRight(string(b), "\x00"), "\x00")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func readLink(p string) string {
	s, err := os.Readlink(p)
	if err != nil {
		return ""
	}
	return s
}
