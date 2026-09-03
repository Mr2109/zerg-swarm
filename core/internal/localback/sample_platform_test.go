package localback

// sample_platform_test.go —— #33：平台采样分支的接线测试。
//
// 本机是 macOS，没有 Linux 环境，/proc 分支无法端到端跑真 /proc；但"读文件 + 解析 + 缺席/带值
// 语义"这一整条链路可以**注入样本文件**在 macOS 上真跑一遍（见 TestSampleProcMemFromFixture）。
// 仍无实测的部分（显存探针真跑 nvidia-smi/rocm-smi）如实标注为未覆盖，不假装验过。

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// 真实格式的 /proc/meminfo 样本（15.5 GiB 机器）。
const procMeminfoFixture = `MemTotal:       16267648 kB
MemFree:          842584 kB
MemAvailable:    7162464 kB
Buffers:          283516 kB
Cached:          5770912 kB
SwapCached:            0 kB
Active:          7823744 kB
Inactive:        3201536 kB
SwapTotal:             0 kB
HugePages_Total:       0
Hugepagesize:       2048 kB
`

// TestSampleProcMemFromFixture —— Linux 内存分支：读 /proc/meminfo（注入样本文件）→ GiB 真值。
// 覆盖"读取 + 解析 + 单位换算 + ok 语义"整条链路；未覆盖的部分只有"真的 /proc 文件系统"本身
// （macOS 上没有 /proc，只能注入）。
func TestSampleProcMemFromFixture(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "meminfo")
	if err := os.WriteFile(f, []byte(procMeminfoFixture), 0o644); err != nil {
		t.Fatalf("写样本失败: %v", err)
	}
	old := procMeminfoPath
	procMeminfoPath = f
	t.Cleanup(func() { procMeminfoPath = old })

	total, avail, totalOK, availOK := sampleProcMem()
	if !totalOK || !availOK {
		t.Fatalf("样本可解析，ok 应为 true/true，实得 %v/%v", totalOK, availOK)
	}
	if !almostEq(total, 15.5140380859375) { // 16267648 kB ÷ 1024 ÷ 1024
		t.Fatalf("总量换算不符：got %v want 15.5140380859375", total)
	}
	if !almostEq(avail, 6.830657958984375) { // 7162464 kB ÷ 1024 ÷ 1024
		t.Fatalf("可用量换算不符：got %v want 6.830657958984375", avail)
	}
	if avail > total {
		t.Fatalf("可用量不得大于总量：%v > %v", avail, total)
	}
}

// TestSampleProcMem_MissingFileIsUnknown —— 读不到 /proc（非 Linux / 权限 / 容器里没有）→ 如实缺席。
func TestSampleProcMem_MissingFileIsUnknown(t *testing.T) {
	old := procMeminfoPath
	procMeminfoPath = filepath.Join(t.TempDir(), "definitely-not-there")
	t.Cleanup(func() { procMeminfoPath = old })

	total, avail, totalOK, availOK := sampleProcMem()
	if totalOK || availOK {
		t.Fatalf("读不到文件必须如实缺席，实得 %v/%v", totalOK, availOK)
	}
	if total != 0 || avail != 0 {
		t.Fatalf("缺席时两个值都必须是 0（不填假值），实得 %v/%v", total, avail)
	}
}

// TestSampleProcLoadFromFixture —— Linux 负载分支：读 /proc/loadavg（注入样本）→ 1 分钟负载。
func TestSampleProcLoadFromFixture(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "loadavg")
	if err := os.WriteFile(f, []byte("0.42 0.55 0.61 2/1234 5678\n"), 0o644); err != nil {
		t.Fatalf("写样本失败: %v", err)
	}
	old := procLoadavgPath
	procLoadavgPath = f
	t.Cleanup(func() { procLoadavgPath = old })

	got, ok := sampleProcLoad()
	if !ok || !almostEq(got, 0.42) {
		t.Fatalf("样本负载应为 0.42/true，实得 %v/%v", got, ok)
	}

	procLoadavgPath = filepath.Join(dir, "definitely-not-there")
	if _, ok := sampleProcLoad(); ok {
		t.Fatal("读不到 /proc/loadavg 必须如实缺席")
	}
}

// TestSamplePlatformMem_CurrentPlatformHasValues —— 端到端：当前平台的内存采样必须给出真值。
//
//	darwin：sysctl hw.memsize + vm_stat（本机实测，见下）。
//	linux ：/proc/meminfo（本机无 Linux 环境 —— 该分支**未实测**，只能靠上面的样本注入 +
//	        纯函数表驱动兜住；这条用例在 Linux 上会真正端到端跑一次）。
func TestSamplePlatformMem_CurrentPlatformHasValues(t *testing.T) {
	total, avail, totalOK, availOK := samplePlatformMem()
	if !totalOK || total <= 0 {
		t.Fatalf("%s：本机内存总量应采到（>0/ok=true），实得 %v/%v", runtime.GOOS, total, totalOK)
	}
	if !availOK || avail <= 0 {
		t.Fatalf("%s：本机可用内存应采到（>0/ok=true），实得 %v/%v", runtime.GOOS, avail, availOK)
	}
	if avail > total {
		t.Fatalf("%s：可用量不得大于总量：%v > %v", runtime.GOOS, avail, total)
	}
	t.Logf("[%s 实测采样] mem_total=%.2f GiB mem_available=%.2f GiB", runtime.GOOS, total, avail)
}

// TestSamplePlatformLoad_CurrentPlatform —— 端到端：当前平台的 1 分钟负载必须拿到（口径两平台一致）。
func TestSamplePlatformLoad_CurrentPlatform(t *testing.T) {
	v, ok := samplePlatformLoad()
	if !ok {
		t.Fatalf("%s：1 分钟负载应采到（macOS: sysctl vm.loadavg / Linux: /proc/loadavg）", runtime.GOOS)
	}
	if v < 0 {
		t.Fatalf("负载不得为负，实得 %v", v)
	}
	t.Logf("[%s 实测采样] load1=%.2f", runtime.GOOS, v)
}

// TestSamplePlatformVram_DarwinIsHonestlyUnknown —— macOS（Apple Silicon 统一内存）显存必须如实未知。
// 本机实测；Linux 侧需真机 GPU 才能验，本机未实测（故按平台跳过，不假装）。
func TestSamplePlatformVram_DarwinIsHonestlyUnknown(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skipf("非 darwin（%s）：本用例只断言 macOS 统一内存平台的如实未知", runtime.GOOS)
	}
	known, total, used, free := samplePlatformVram()
	if known || total != 0 || used != 0 || free != 0 {
		t.Fatalf("统一内存平台拿不到独立显存，必须 known=false + 全 0（不冒充），实得 known=%v %v/%v/%v",
			known, total, used, free)
	}
}

// TestProbeVram_Wiring —— 显存探针链接线：依次尝试、第一个拿到真值的胜出、全拿不到则如实未知。
// 用假探针替代 nvidia-smi/rocm-smi（本机没有这些工具，真命令分支未实测）。
func TestProbeVram_Wiring(t *testing.T) {
	miss := func() (float64, float64, float64, bool) { return 0, 0, 0, false }
	hit := func() (float64, float64, float64, bool) { return 24, 6, 18, true }

	// 全拿不到 → known=false 且值全 0（缺席，不填假值）
	known, total, used, free := probeVram([]func() (float64, float64, float64, bool){miss, miss})
	if known || total != 0 || used != 0 || free != 0 {
		t.Fatalf("全拿不到必须 known=false + 全 0，实得 %v %v/%v/%v", known, total, used, free)
	}
	// 第一个失败、第二个成功 → 取第二个的值（顺序即优先级）
	known, total, used, free = probeVram([]func() (float64, float64, float64, bool){miss, hit})
	if !known || total != 24 || used != 6 || free != 18 {
		t.Fatalf("应取第一个成功的探针值，实得 %v %v/%v/%v", known, total, used, free)
	}
	// 第一个成功 → 不再看后面的
	known, _, _, _ = probeVram([]func() (float64, float64, float64, bool){hit, miss})
	if !known {
		t.Fatal("第一个探针成功时应 known=true")
	}
	// 探针链默认为 nvidia-smi → rocm-smi（顺序：NVIDIA 优先；AMD 机器上 nvidia-smi 不存在会直接缺席）
	if len(vramProbes) != 2 {
		t.Fatalf("显存探针链应为 2 个（nvidia-smi → rocm-smi），实得 %d", len(vramProbes))
	}
}

// TestSamplePlatformVram_NonLinuxNeverClaims —— 非 Linux 平台（本机即 macOS）绝不能报"已知显存"。
// 这是 #29 铁律的平台面：只有 Linux 才有可能拿到独立显存额度。
func TestSamplePlatformVram_NonLinuxNeverClaims(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skipf("Linux：显存取决于机器有没有 GPU 工具，本机（macOS）无 Linux 环境，未实测")
	}
	if known, _, _, _ := samplePlatformVram(); known {
		t.Fatalf("%s：非 Linux 平台不得报 vram_known=true", runtime.GOOS)
	}
}

// TestSnapshot_MemAndVramInvariants —— 快照层两平台一致的不变量（#33 第 3 条）。
// 无论哪个平台：拿不到 → 缺席 + known=false；拿到了 → 带值 + known=true。不得"半真"。
func TestSnapshot_MemAndVramInvariants(t *testing.T) {
	lb := &LocalBackend{state: stateIdle, circuit: 3}
	snap := lb.Snapshot()
	if snap == nil {
		t.Fatal("快照不应 nil")
	}
	// 内存：缺席必须是 0（不许负数/编造）；有值时可用量不得大于总量。
	if snap.MemTotalGb < 0 || snap.MemAvailableGb < 0 {
		t.Fatalf("内存值不得为负（缺席 = 0）：%v/%v", snap.MemTotalGb, snap.MemAvailableGb)
	}
	if snap.MemTotalGb > 0 && snap.MemAvailableGb > snap.MemTotalGb {
		t.Fatalf("可用量不得大于总量：%v > %v", snap.MemAvailableGb, snap.MemTotalGb)
	}
	if snap.Load < 0 {
		t.Fatalf("负载不得为负：%v", snap.Load)
	}
	// 显存：known=false ⇒ 三值缺席且 gpu_used_gb=0；known=true ⇒ 三值到场且非统一内存。
	if snap.VramKnown {
		if snap.VramTotalGb <= 0 || snap.VramUsedGb < 0 || snap.VramFreeGb < 0 {
			t.Fatalf("known=true 必须带真值：%v/%v/%v", snap.VramTotalGb, snap.VramUsedGb, snap.VramFreeGb)
		}
		if snap.VramUnified {
			t.Fatal("独立显存与统一内存互斥：known=true 时 vram_unified 必须为 false")
		}
	} else {
		if snap.VramTotalGb != 0 || snap.VramUsedGb != 0 || snap.VramFreeGb != 0 {
			t.Fatalf("known=false 时三值必须缺席（0）：%v/%v/%v", snap.VramTotalGb, snap.VramUsedGb, snap.VramFreeGb)
		}
		if snap.GpuUsedGb != 0 {
			t.Fatalf("known=false 时 gpu_used_gb 必须为 0（不拿内存冒充）：%v", snap.GpuUsedGb)
		}
	}
}
