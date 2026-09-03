package localback

// sample_parse_test.go —— #33：本机采样解析的**表驱动**测试（纯函数，任意平台可跑）。
//
// 为什么这样测：开发机是 macOS，本机没有 Linux 环境，/proc 那份代码在开发机上跑不起来。
// 所以"文本 → 数值"的全部判定都放在纯函数里，用真实格式样本在 macOS 上验（含缺字段、
// 数值异常、读不懂的输出）。用例里的期望值一律写字面量（kB÷1024÷1024、MiB÷1024、B÷1024³
// 的换算结果），不调用被测函数，避免自证。

import (
	"math"
	"testing"
)

// almostEq 采样值对比：GiB 量级下 1e-9 的容差足够严（实际换算都是二进制精确运算）。
func almostEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// TestParseMeminfo —— /proc/meminfo 解析：正常、缺字段、缺 MemAvailable（老内核回退）、数值异常。
func TestParseMeminfo(t *testing.T) {
	cases := []struct {
		name                     string
		in                       string
		wantTotal, wantAvail     float64
		wantTotalOK, wantAvailOK bool
	}{
		{
			name: "正常样本（15.5 GiB 机器，真实 /proc/meminfo 片段）",
			in: `MemTotal:       16267648 kB
MemFree:          842584 kB
MemAvailable:    7162464 kB
Buffers:          283516 kB
Cached:          5770912 kB
SwapCached:            0 kB
Active:          7823744 kB
Inactive:        3201536 kB
SwapTotal:             0 kB
HugePages_Total:       0
Hugepagesize:       2048 kB`,
			wantTotal:   15.5140380859375,  // 16267648 kB ÷ 1024 ÷ 1024
			wantAvail:   6.830657958984375, // 7162464 kB ÷ 1024 ÷ 1024
			wantTotalOK: true, wantAvailOK: true,
		},
		{
			name: "缺 MemAvailable（内核 < 3.14）→ 回退 MemFree+Buffers+Cached",
			in: `MemTotal:        1048576 kB
MemFree:          524288 kB
Buffers:           65536 kB
Cached:           262144 kB`,
			wantTotal:   1.0,    // 1048576 kB
			wantAvail:   0.8125, // (524288+65536+262144) kB = 851968 kB
			wantTotalOK: true, wantAvailOK: true,
		},
		{
			name: "MemAvailable=0 → 不当可用量，仍走回退求和",
			in: `MemTotal:        1048576 kB
MemAvailable:          0 kB
MemFree:          524288 kB
Buffers:           65536 kB
Cached:           262144 kB`,
			wantTotal: 1.0, wantAvail: 0.8125,
			wantTotalOK: true, wantAvailOK: true,
		},
		{
			name: "SwapCached 不得被当成 Cached（缺 MemAvailable 时只算 MemFree）",
			in: `MemTotal:        1048576 kB
MemFree:          524288 kB
SwapCached:       999999 kB`,
			wantTotal: 1.0, wantAvail: 0.5,
			wantTotalOK: true, wantAvailOK: true,
		},
		{
			name: "缺 MemTotal → 总量缺席，可用量仍如实在（两字段各自独立）",
			in: `MemAvailable:     262144 kB
MemFree:          524288 kB`,
			wantTotal: 0, wantAvail: 0.25,
			wantTotalOK: false, wantAvailOK: true,
		},
		{
			name: "MemFree 也缺 → 可用量缺席（不拿 0 冒充）",
			in: `MemTotal:        1048576 kB
Buffers:           65536 kB`,
			wantTotal: 1.0, wantAvail: 0,
			wantTotalOK: true, wantAvailOK: false,
		},
		{
			name: "MemFree=0 且 Buffers/Cached 缺席 → 可用量缺席（不把 0 当已知值）",
			in: `MemTotal:        1048576 kB
MemFree:               0 kB`,
			wantTotal: 1.0, wantAvail: 0,
			wantTotalOK: true, wantAvailOK: false,
		},
		{
			name: "MemTotal 非数 → 该字段缺席",
			in: `MemTotal:        abc kB
MemAvailable:    1048576 kB`,
			wantTotal: 0, wantAvail: 1.0,
			wantTotalOK: false, wantAvailOK: true,
		},
		{
			name:      "MemTotal 为负 → 该字段缺席",
			in:        `MemTotal:            -5 kB`,
			wantTotal: 0, wantAvail: 0,
			wantTotalOK: false, wantAvailOK: false,
		},
		{
			name:      "MemTotal 单位不是 kB → 不硬套，缺席",
			in:        `MemTotal:       16331236 MB`,
			wantTotal: 0, wantAvail: 0,
			wantTotalOK: false, wantAvailOK: false,
		},
		{
			name: "数值为空 → 该行忽略",
			in: `MemTotal:
MemFree:          524288 kB`,
			wantTotal: 0, wantAvail: 0.5,
			wantTotalOK: false, wantAvailOK: true,
		},
		{
			name: "垃圾行（无冒号）忽略，不影响其它字段",
			in: `garbage line without colon
MemTotal:        1048576 kB
MemFree:          524288 kB`,
			wantTotal: 1.0, wantAvail: 0.5,
			wantTotalOK: true, wantAvailOK: true,
		},
		{
			name:      "空内容 → 全缺席",
			in:        "",
			wantTotal: 0, wantAvail: 0,
			wantTotalOK: false, wantAvailOK: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			total, avail, totalOK, availOK := parseMeminfo(c.in)
			if totalOK != c.wantTotalOK || availOK != c.wantAvailOK {
				t.Fatalf("ok 标记不符：totalOK=%v(want %v) availOK=%v(want %v)",
					totalOK, c.wantTotalOK, availOK, c.wantAvailOK)
			}
			if !almostEq(total, c.wantTotal) {
				t.Fatalf("总量不符：got %v want %v", total, c.wantTotal)
			}
			if !almostEq(avail, c.wantAvail) {
				t.Fatalf("可用量不符：got %v want %v", avail, c.wantAvail)
			}
		})
	}
}

// TestParseProcLoadavg —— /proc/loadavg（Linux 负载口径：第 1 字段）。
func TestParseProcLoadavg(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   float64
		wantOK bool
	}{
		{name: "正常", in: "0.52 0.58 0.59 1/1234 5678", want: 0.52, wantOK: true},
		{name: "零负载", in: "0.00 0.00 0.00 1/1 1", want: 0, wantOK: true},
		{name: "前后空白", in: "  1.25 1.30 1.10 2/500 9999\n", want: 1.25, wantOK: true},
		{name: "空内容", in: "", want: 0, wantOK: false},
		{name: "非数", in: "abc def ghi", want: 0, wantOK: false},
		{name: "NaN → 视为没采到", in: "NaN 0 0 1/1 1", want: 0, wantOK: false},
		{name: "负数 → 视为没采到", in: "-1.0 0 0 1/1 1", want: 0, wantOK: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseProcLoadavg(c.in)
			if ok != c.wantOK || !almostEq(got, c.want) {
				t.Fatalf("parseProcLoadavg(%q) = (%v,%v), want (%v,%v)", c.in, got, ok, c.want, c.wantOK)
			}
		})
	}
}

// TestParseDarwinLoadavg —— macOS vm.loadavg 输出（"{ 1.79 2.03 2.14 }"）：第 1 字段是花括号，取第 2 个。
func TestParseDarwinLoadavg(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   float64
		wantOK bool
	}{
		{name: "正常", in: "{ 1.79 2.03 2.14 }", want: 1.79, wantOK: true},
		{name: "带前后空白", in: "  { 0.52 0.58 0.59 } \n", want: 0.52, wantOK: true},
		{name: "空花括号", in: "{ }", want: 0, wantOK: false},
		{name: "只有 1 个字段（取不到第 2 个）", in: "1.79", want: 0, wantOK: false},
		{name: "空内容", in: "", want: 0, wantOK: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseDarwinLoadavg(c.in)
			if ok != c.wantOK || !almostEq(got, c.want) {
				t.Fatalf("parseDarwinLoadavg(%q) = (%v,%v), want (%v,%v)", c.in, got, ok, c.want, c.wantOK)
			}
		})
	}
}

// TestParseVMPages —— 既有 macOS 解析原样保留（解析不出 → 0，不问 ok）。
func TestParseVMPages(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"Pages free:                                552578.", 552578},
		{"Pages free: 0.", 0},
		{"Pages free: 12345", 12345},
		{"Pages free:", 0},
		{"Pages free: abc.", 0},
		{"garbage", 0},
	}
	for _, c := range cases {
		if got := parseVMPages(c.in); got != c.want {
			t.Fatalf("parseVMPages(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestParseVMStatAvailableGiB —— free + inactive + speculative（页 × 16384 → GiB）。
func TestParseVMStatAvailableGiB(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   float64
		wantOK bool
	}{
		{
			name: "真实 vm_stat 片段",
			in: `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               552578.
Pages active:                            3000000.
Pages inactive:                           300000.
Pages speculative:                         12345.`,
			// (552578+300000+12345) 页 × 16384 ÷ 1024³
			want: 13.197677612304688, wantOK: true,
		},
		{
			name: "三项全 0 → 视为没采到（不把「读不懂」当「可用 0」）",
			in: `Pages free: 0.
Pages inactive: 0.
Pages speculative: 0.`,
			want: 0, wantOK: false,
		},
		{name: "空内容", in: "", want: 0, wantOK: false},
		{name: "缺 free 但有 inactive（仍求和）", in: "Pages inactive: 65536.", want: 1.0, wantOK: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseVMStatAvailableGiB(c.in)
			if ok != c.wantOK || !almostEq(got, c.want) {
				t.Fatalf("got (%v,%v), want (%v,%v)", got, ok, c.want, c.wantOK)
			}
		})
	}
}

// TestParseHwMemsizeGiB —— macOS 内存总量（hw.memsize 字节 → GiB）。
func TestParseHwMemsizeGiB(t *testing.T) {
	cases := []struct {
		in     string
		want   float64
		wantOK bool
	}{
		{"68719476736", 64.0, true},    // 64 GiB
		{" 17179869184\n", 16.0, true}, // 16 GiB
		{"0", 0, false},
		{"-1", 0, false},
		{"", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, ok := parseHwMemsizeGiB(c.in)
		if ok != c.wantOK || !almostEq(got, c.want) {
			t.Fatalf("parseHwMemsizeGiB(%q) = (%v,%v), want (%v,%v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

// TestParseNvidiaSmiMem —— nvidia-smi --query-gpu=memory.total,memory.used,memory.free（MiB）。
func TestParseNvidiaSmiMem(t *testing.T) {
	cases := []struct {
		name                          string
		in                            string
		wantTotal, wantUsed, wantFree float64
		wantOK                        bool
	}{
		{name: "单卡", in: "24564, 1234, 23330",
			wantTotal: 23.98828125, wantUsed: 1.205078125, wantFree: 22.783203125, wantOK: true},
		{name: "多卡取第一块（不跨卡求和）",
			in:        "24564, 1234, 23330\n24564, 20000, 4564",
			wantTotal: 23.98828125, wantUsed: 1.205078125, wantFree: 22.783203125, wantOK: true},
		{name: "含空行/尾换行", in: "24564, 1234, 23330\n",
			wantTotal: 23.98828125, wantUsed: 1.205078125, wantFree: 22.783203125, wantOK: true},
		{name: "[N/A] 列 → 采不到", in: "[N/A], 1234, 23330", wantOK: false},
		{name: "列数不足 → 采不到", in: "24564, 1234", wantOK: false},
		{name: "总量为 0 → 采不到（不填假值）", in: "0, 0, 0", wantOK: false},
		{name: "非数列 → 采不到", in: "24564, abc, 23330", wantOK: false},
		{name: "空内容 → 采不到", in: "", wantOK: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			total, used, free, ok := parseNvidiaSmiMem(c.in)
			if ok != c.wantOK {
				t.Fatalf("ok=%v want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if !almostEq(total, c.wantTotal) || !almostEq(used, c.wantUsed) || !almostEq(free, c.wantFree) {
				t.Fatalf("got %v/%v/%v, want %v/%v/%v",
					total, used, free, c.wantTotal, c.wantUsed, c.wantFree)
			}
		})
	}
}

// TestParseRocmSmiMem —— rocm-smi --showmeminfo vram 的两种真实输出形态（字节）。
func TestParseRocmSmiMem(t *testing.T) {
	csv := `device,VRAM Total Memory (B),VRAM Total Used Memory (B)
card0,17163091968,742391808`
	labelled := `============================ ROCm System Management Interface ============================
=============================== Memory Usage (Bytes) ================================
GPU[0]		: VRAM Total Memory (B): 17163091968
GPU[0]		: VRAM Total Used Memory (B): 742391808
======================================================================================`
	cases := []struct {
		name                          string
		in                            string
		wantTotal, wantUsed, wantFree float64
		wantOK                        bool
	}{
		{name: "CSV 形态（表头定位列号）", in: csv,
			wantTotal: 15.984375, wantUsed: 0.69140625, wantFree: 15.29296875, wantOK: true},
		{name: "人类可读形态（标签: 数值）", in: labelled,
			wantTotal: 15.984375, wantUsed: 0.69140625, wantFree: 15.29296875, wantOK: true},
		{name: "无表头只有数据行 → 不认识，采不到", in: "card0,17163091968,742391808", wantOK: false},
		{name: "总量为 0 → 采不到", in: "device,VRAM Total Memory (B),VRAM Total Used Memory (B)\ncard0,0,0", wantOK: false},
		{name: "空内容 → 采不到", in: "", wantOK: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			total, used, free, ok := parseRocmSmiMem(c.in)
			if ok != c.wantOK {
				t.Fatalf("ok=%v want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if !almostEq(total, c.wantTotal) || !almostEq(used, c.wantUsed) || !almostEq(free, c.wantFree) {
				t.Fatalf("got %v/%v/%v, want %v/%v/%v",
					total, used, free, c.wantTotal, c.wantUsed, c.wantFree)
			}
		})
	}
}
