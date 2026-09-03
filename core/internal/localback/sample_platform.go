package localback

// sample_platform.go —— 本机采样的平台分派与 IO 层（文本解析全在 sample_parse.go）。
//
// 【#33 背景】此前本机采样**只走 macOS 专有命令**：
//
//	/usr/sbin/sysctl -n hw.memsize（内存总量）
//	/usr/bin/vm_stat（可用内存）
//	/usr/sbin/sysctl -n vm.loadavg（负载）
//
// 在 Linux 上这三条必然失败 → 本机行 mem_known=false、无负载（那是"如实未知"，不是假值，
// 但也不该是能力缺失）。现在按平台分派：darwin 走 sysctl/vm_stat（行为不变），
// linux 走 /proc/meminfo + /proc/loadavg，显存（只有 Linux 才可能拿到独立显存）试
// nvidia-smi → rocm-smi。
//
// 【统一不变量（两平台一致，不许出现"半真"）】拿到 → 带值 + known/ok=true；
// 拿不到 → ok=false，调用方留 0（值键缺席）+ known=false。绝不用 0 / 估算值冒充"已知"。
//
// 【为什么不按 //go:build 拆文件】这些函数只用 os / os/exec / runtime（都跨平台），
// 而 Linux 分支在开发机（macOS）上唯一能真跑起来的办法就是把 /proc 路径注入成样本文件
// ——见 sample_platform_test.go。按 GOOS 拆文件会让 Linux 分支在本机彻底无法执行。

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// /proc 路径走变量：测试可指向样本文件，从而在 macOS 上端到端跑通 Linux 分支的"读取 + 解析"。
var (
	procMeminfoPath = "/proc/meminfo"
	procLoadavgPath = "/proc/loadavg"
)

// memProbeTimeout 是显存探针的命令超时：工具挂住不能拖住主控（本机可能根本没装这些工具）。
const memProbeTimeout = 3 * time.Second

// vramProbes 是显存采样探针链（依次尝试，第一个拿到真值的胜出）。
// 走变量是为了可测：本机（macOS）没有 GPU 工具，只能注入假探针来验证"拿到 → known=true 带值 /
// 拿不到 → known=false 不带值"这两条接线是否诚实。
var vramProbes = []func() (totalGB, usedGB, freeGB float64, ok bool){
	nvidiaSmiVram,
	rocmSmiVram,
}

// samplePlatformMem 采样本机内存总量与可用量（GiB）：darwin 走 sysctl/vm_stat，linux 走 /proc/meminfo。
func samplePlatformMem() (totalGB, availGB float64, totalOK, availOK bool) {
	switch runtime.GOOS {
	case "darwin":
		return sampleMacMem()
	case "linux":
		return sampleProcMem()
	default:
		return 0, 0, false, false // 其它平台未实现采样 → 如实未知（不猜）
	}
}

// sampleMacMem 采样本机内存（darwin）：总量 sysctl hw.memsize、可用量 vm_stat
// （free + inactive + speculative 页 × 16384）。行为与既有实现一致，只是把"采到了没有"显式化。
func sampleMacMem() (totalGB, availGB float64, totalOK, availOK bool) {
	out, err := exec.Command("/usr/sbin/sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, 0, false, false
	}
	totalGB, totalOK = parseHwMemsizeGiB(string(out))
	if !totalOK {
		return 0, 0, false, false
	}
	vout, err := exec.Command("/usr/bin/vm_stat").Output()
	if err != nil {
		return totalGB, 0, true, false // 总量拿到了、可用量没拿到 —— 两个字段各自如实
	}
	availGB, availOK = parseVMStatAvailableGiB(string(vout))
	return totalGB, availGB, true, availOK
}

// sampleProcMem 采样本机内存（linux）：读 /proc/meminfo（内核导出，无需外部命令）。
func sampleProcMem() (totalGB, availGB float64, totalOK, availOK bool) {
	data, err := os.ReadFile(procMeminfoPath)
	if err != nil {
		return 0, 0, false, false // 读不到（含非 Linux 上根本没有 /proc）→ 如实缺席
	}
	return parseMeminfo(string(data))
}

// samplePlatformLoad 本机 1 分钟负载：darwin 走 sysctl vm.loadavg，linux 走 /proc/loadavg。
func samplePlatformLoad() (float64, bool) {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("/usr/sbin/sysctl", "-n", "vm.loadavg").Output()
		if err != nil {
			return 0, false
		}
		return parseDarwinLoadavg(string(out))
	case "linux":
		return sampleProcLoad()
	default:
		return 0, false
	}
}

// sampleProcLoad 采样本机负载（linux）：读 /proc/loadavg，取 1 分钟负载。
func sampleProcLoad() (float64, bool) {
	data, err := os.ReadFile(procLoadavgPath)
	if err != nil {
		return 0, false
	}
	return parseProcLoadavg(string(data))
}

// samplePlatformVram 本机独立显存采样。
//   - darwin：Apple Silicon 统一内存没有"独立显存额度" → 如实未知（known=false；统一内存形态
//     由 localVramShape() 标注），绝不拿内存量冒充（#29）。
//   - linux：试 nvidia-smi → rocm-smi（探针链）；都拿不到 → known=false 且值留 0（缺席，不填假值）。
//   - 其它平台：未实现 → known=false。
func samplePlatformVram() (known bool, totalGB, usedGB, freeGB float64) {
	if runtime.GOOS != "linux" {
		return false, 0, 0, 0
	}
	return probeVram(vramProbes)
}

// probeVram 依次跑探针，返回第一个拿到真值的结果；全拿不到 → known=false（值全 0 = 缺席）。
func probeVram(probes []func() (totalGB, usedGB, freeGB float64, ok bool)) (bool, float64, float64, float64) {
	for _, p := range probes {
		if total, used, free, ok := p(); ok {
			return true, total, used, free
		}
	}
	return false, 0, 0, 0
}

// nvidiaSmiVram 试 nvidia-smi（NVIDIA 独显）：单位 MiB，多卡取第一块。
func nvidiaSmiVram() (float64, float64, float64, bool) {
	if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return 0, 0, 0, false // 没装 → 不试、不报错（缺席）
	}
	out, err := runProbeCmd("nvidia-smi",
		"--query-gpu=memory.total,memory.used,memory.free",
		"--format=csv,noheader,nounits")
	if err != nil {
		return 0, 0, 0, false
	}
	return parseNvidiaSmiMem(out)
}

// rocmSmiVram 试 rocm-smi（AMD，X3 那类机器）：单位字节。
func rocmSmiVram() (float64, float64, float64, bool) {
	if _, err := exec.LookPath("rocm-smi"); err != nil {
		return 0, 0, 0, false
	}
	out, err := runProbeCmd("rocm-smi", "--showmeminfo", "vram", "--csv")
	if err != nil {
		return 0, 0, 0, false
	}
	return parseRocmSmiMem(out)
}

// runProbeCmd 跑一个采样命令（带超时；失败/超时 → error，调用方按"缺席"处理）。
func runProbeCmd(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), memProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
