package registry

// ═══ P5 批 2：存储档位 + 装载下限（设计-子端沙箱化-20260914 §9.6）══════════
// 钉住的判据：
//   - 装载下限 = 权重体积 ÷ 实测顺序读带宽（物理下限，不是估计值）；
//   - 缺输入（体积 / 带宽任一缺失或非正）⇒ ok=false，**不猜**（§8.4 标定铁律）；
//   - 档位枚举只有 NVMe / SATA SSD / HDD 三个接口类别；**档位缺失不猜**
//     （X3 = NVMe 是 2026-09-14 的实测结论，别的机器没实测就没有档位）。

import (
	"math"
	"testing"
	"time"
)

// TestStorageTier_Valid 档位枚举就三个接口类别（设计 §9.6：不是型号）。
func TestStorageTier_Valid(t *testing.T) {
	for _, v := range []StorageTier{TierNVMe, TierSataSSD, TierHDD} {
		if !v.Valid() {
			t.Fatalf("档位 %q 应为合法接口类别", v)
		}
	}
	for _, v := range []StorageTier{"", "nvme ssd", "SSD", "机械盘", "tape"} {
		if StorageTier(v).Valid() {
			t.Fatalf("档位 %q 不该是合法接口类别（枚举之外不认）", v)
		}
	}
}

// TestLoadTimeFloor_Conventional 常规值：90 GiB 权重 @ 实测 3.5 GB/s
// ⇒ 27.610504045s（向下取整到纳秒；X3 量级——NVMe ⇒ 几秒~几十秒，§9.6）。
func TestLoadTimeFloor_Conventional(t *testing.T) {
	const weight = int64(90) * (1 << 30) // 96636764160 B
	const bw = int64(3_500_000_000)      // 实测 3.5 GB/s
	d, ok := LoadTimeFloor(weight, bw)
	if !ok {
		t.Fatal("常规输入必须算得出下限")
	}
	want := 27_610_504_045 * time.Nanosecond // 精确到纳秒的向下取整
	if d != want {
		t.Fatalf("装载下限 = %v, 期望 %v（体积÷带宽，向下取整到 ns）", d, want)
	}
	// 量级核对：NVMe ⇒ 秒级~几十秒（§9.6），不允许出现「上百秒」的错档。
	if d > 90*time.Second {
		t.Fatalf("NVMe 量级不该给出 %v 的下限（上百秒是 SATA 档）", d)
	}
}

// TestLoadTimeFloor_ZeroAndNegative 边界：体积或带宽非正 ⇒ ok=false（不猜）。
func TestLoadTimeFloor_ZeroAndNegative(t *testing.T) {
	cases := []struct {
		name        string
		weightBytes int64
		bw          int64
	}{
		{"体积为 0（无实测档案）", 0, 3_500_000_000},
		{"体积为负", -90 << 30, 3_500_000_000},
		{"带宽为 0（未实测）", 90 << 30, 0},
		{"带宽为负", 90 << 30, -1},
		{"两者皆缺", 0, 0},
	}
	for _, c := range cases {
		if d, ok := LoadTimeFloor(c.weightBytes, c.bw); ok {
			t.Fatalf("[%s] 必须 ok=false（不确定就明说，不猜），实得 %v", c.name, d)
		}
	}
}

// TestLoadTimeFloor_Huge 超大值：不溢出、量级正确；荒谬到装不进 Duration ⇒ ok=false。
func TestLoadTimeFloor_Huge(t *testing.T) {
	// 大而合法：1 PB @ 10 MB/s（慢盘大模型）≈ 1.16e14 ns，Duration 装得下。
	const pb = int64(1) << 50 // 1 PiB
	d, ok := LoadTimeFloor(pb, 10_000_000)
	if !ok {
		t.Fatal("1PiB @ 10MB/s 应算得出")
	}
	q, r := pb/10_000_000, pb%10_000_000
	want := time.Duration(q*1e9 + r*1e9/10_000_000)
	if d != want {
		t.Fatalf("1PiB @ 10MB/s = %v, 期望 %v", d, want)
	}
	// 荒谬组合：体积大到 ×1e9 必溢出 ⇒ 如实报不可算，绝不回绕成小值。
	if _, ok := LoadTimeFloor(math.MaxInt64, 1); ok {
		t.Fatal("MaxInt64 字节 @ 1 B/s 必须报不可算（溢出守卫）")
	}
	// 1 B @ 1 B/s：最小合法组合 = 恰好 1s。
	if d, ok := LoadTimeFloor(1, 1); !ok || d != time.Second {
		t.Fatalf("1B@1B/s 应为 1s，实得 %v ok=%v", d, ok)
	}
}

// TestLoadTimeFloor_FloorRoundsDown 取整方向钉死：除不尽时**向下**取整——
// 下限变松等于谎报物理事实；用除不尽的组合验证。
func TestLoadTimeFloor_FloorRoundsDown(t *testing.T) {
	// 100 字节 @ 3 字节/秒 = 33.33…秒 ⇒ 精确 floor = 33_333_333_333ns
	// （100×10⁹ = 33×3×10⁹ + 1 ⇒ 余数只贡献 0ns，必须取不到 33_333_333_334）。
	d, ok := LoadTimeFloor(100, 3)
	if !ok {
		t.Fatal("应算得出")
	}
	if want := time.Duration(100) * time.Second / 3; d != want { // 33.333333333s
		t.Fatalf("100B@3B/s 应向下取整为 %v，实得 %v", want, d)
	}
	if d >= 33_333_333_334*time.Nanosecond {
		t.Fatalf("下限取整方向不许向上：实得 %v", d)
	}
}
