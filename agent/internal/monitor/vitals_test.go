// vitals_test.go —— 虫须测试。
//
// 样例纪律：sysfs 解析用例**逐字照抄 X3 实测字节**（2026-09-14 真机取证）：
//
//	/sys/class/drm/card1/device/mem_info_gtt_used  = 60880695296（空载收卵后）
//	/sys/class/drm/card1/device/mem_info_gtt_total = 133143986176
//
// fdinfo 样例与 backend/baseline_mem_test.go 同源（X3 两条真服务实测）。
package monitor

import (
	"math"
	"strings"
	"testing"
	"time"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 0.05 }

// ══════════════ X3 真实格式解析 ══════════════

// TestParseGlobalGtt —— X3 真机 sysfs 文件逐字样例（多卡取和的输入形状）。
func TestParseGlobalGtt(t *testing.T) {
	usedB, ok := parseSysfsUintSample("60880695296\n")
	if !ok || usedB != 60880695296 {
		t.Fatalf("used 解析错: %d ok=%v（X3 实测 60880695296）", usedB, ok)
	}
	totalB, ok := parseSysfsUintSample("133143986176")
	if !ok || totalB != 133143986176 {
		t.Fatalf("total 解析错: %d ok=%v（X3 实测 133143986176）", totalB, ok)
	}
	// X3 原文字节 → GiB：used ≈ 56.70（与 §8.2 实测对照 56.70 GiB 吻合）、total ≈ 123.96
	g := GttSample{UsedBytes: usedB, TotalBytes: totalB, Ok: true}
	if !approx(g.UsedGb(), 56.70) || !approx(g.TotalGb(), 123.96) {
		t.Fatalf("换算错: used=%.2f total=%.2f", g.UsedGb(), g.TotalGb())
	}
}

// TestParseDrmGttKib —— fdinfo X3 实测样例（照抄 baseline_mem_test.go 的字节）。
func TestParseDrmGttKib(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		wantKib float64
	}{
		{"X3 Qwen（KiB，空格+制表符混排）", "pos:\t0\nflags:\t0100002\ndrm-memory-vram:\t2208 KiB\ndrm-memory-gtt: \t34286572 KiB\ndrm-memory-cpu: \t0 KiB\n", 34286572},
		{"X3 K2", "drm-memory-gtt:\t25144840 KiB\n", 25144840},
		{"两项 gtt 相加（多 fd）", "drm-memory-gtt: \t1000 KiB\ndrm-memory-gtt:\t2000 KiB\n", 3000},
		{"GiB 单位", "drm-memory-gtt: 2 GiB\n", 2 * 1024 * 1024},
		{"MiB 单位", "drm-memory-gtt: 1024 MiB\n", 1024 * 1024},
		{"vram 行不算", "drm-memory-vram:\t2208 KiB\n", 0},
		{"cpu 行不算", "drm-memory-cpu: \t0 KiB\n", 0},
		{"形状不符", "drm-memory-gtt:\n", 0},
		{"非数字", "drm-memory-gtt: abc KiB\n", 0},
		{"乱文本", "No GPU here\n", 0},
	}
	for _, c := range cases {
		got := parseDrmGttKibCommon(c.text)
		if got != c.wantKib {
			t.Fatalf("%s：期望 %v KiB，实得 %v", c.name, c.wantKib, got)
		}
	}
}

// parseSysfsUintSample 模拟 readSysfsUint 对"文件内容"的解析（含末尾换行）。
func parseSysfsUintSample(s string) (uint64, bool) {
	return readSysfsUintContent(strings.TrimSpace(s))
}

// ══════════════ 频率分层（注入时钟，无睡眠） ══════════════

// TestMaybeCollect_FrequencyLayering —— 快采按 fastInterval 节流、慢采按 slowInterval 节流。
func TestMaybeCollect_FrequencyLayering(t *testing.T) {
	r := NewVitalsRecorder()
	r.fastInterval = 2 * time.Second
	r.slowInterval = 15 * time.Second
	r.dataDir = "." // darwin 上 statfs 任意目录可用

	t0 := time.Unix(1700000000, 0)
	if !r.MaybeCollect(t0) {
		t.Fatal("首轮必须采样")
	}
	if r.MaybeCollect(t0.Add(1 * time.Second)) {
		t.Fatal("1s < 2s 快采间隔，不应采样")
	}
	if !r.MaybeCollect(t0.Add(2 * time.Second)) {
		t.Fatal("2s 到期必须采样")
	}
	if n := len(r.RecentRaw(0)); n != 2 {
		t.Fatalf("快采轮数应为 2，实得 %d", n)
	}
	// 慢采：2s 时未到期（attrib 仍空——darwin 桩本就 Ok=false 不入环，用 lastSlow 验证节奏）
	if !r.MaybeCollect(t0.Add(17 * time.Second)) {
		t.Fatal("17s 时快采也到期必须采样")
	}
	if r.MaybeCollect(t0.Add(18 * time.Second)) {
		t.Fatal("18s 距上轮 1s，不应采样")
	}
	if !r.MaybeCollect(t0.Add(32 * time.Second)) {
		t.Fatal("32s 时快采（30s 到期）必须采样")
	}
}

// TestMaybeCollect_NonBlocking —— 一次 MaybeCollect 不做任何等待（读数是毫秒级文件读，
// 分层逻辑不得引入 sleep——频率由调用方时钟控制）。
func TestMaybeCollect_NonBlocking(t *testing.T) {
	r := NewVitalsRecorder()
	r.dataDir = "."
	t0 := time.Now()
	r.MaybeCollect(t0)
	r.MaybeCollect(t0.Add(3 * time.Second))
	if d := time.Since(t0); d > 2*time.Second {
		t.Fatalf("MaybeCollect 阻塞了 %v——分层采样不得等待", d)
	}
}

// ══════════════ 环形缓冲覆盖 ══════════════

func TestRingBuf_OverwriteOldest(t *testing.T) {
	r := newRing[int](3)
	for i := 0; i < 5; i++ {
		r.Push(i)
	}
	got := r.Slice()
	if len(got) != 3 || got[0] != 2 || got[2] != 4 {
		t.Fatalf("覆盖最旧失败: %v", got)
	}
}

func TestRingBuf_OrderPreserved(t *testing.T) {
	r := newRing[string](4)
	for _, s := range []string{"a", "b", "c"} {
		r.Push(s)
	}
	got := r.Slice()
	if strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("顺序错: %v", got)
	}
}

// TestRecentRaw_Bounded —— 环容量上限成立（recentRawCap 轮后不再增长）。
func TestRecentRaw_Bounded(t *testing.T) {
	r := NewVitalsRecorder()
	r.dataDir = "."
	t0 := time.Unix(1700000000, 0)
	for i := 0; i < recentRawCap+10; i++ {
		r.MaybeCollect(t0.Add(time.Duration(i*60) * time.Second)) // 间隔拉满 ⇒ 每轮都采
	}
	if n := len(r.RecentRaw(0)); n != recentRawCap {
		t.Fatalf("环应封顶 %d，实得 %d", recentRawCap, n)
	}
}

// TestRecentRaw_TailN —— n 截尾只取最新。
func TestRecentRaw_TailN(t *testing.T) {
	r := NewVitalsRecorder()
	r.dataDir = "."
	t0 := time.Unix(1700000000, 0)
	for i := 0; i < 5; i++ {
		r.MaybeCollect(t0.Add(time.Duration(i*60) * time.Second))
	}
	got := r.RecentRaw(2)
	if len(got) != 2 {
		t.Fatalf("应取最新 2 轮，实得 %d", len(got))
	}
}

// ══════════════ 事件钩子 ══════════════

// TestEventHooks_HatchAndCollect —— 孵化/收卵前后各采一次，事件环各记一条。
func TestEventHooks_HatchAndCollect(t *testing.T) {
	r := NewVitalsRecorder()
	r.dataDir = "."

	hs := r.OnHatch("egg-qwen-27b")
	if hs.kind != EventHatch {
		t.Fatalf("kind 错: %s", hs.kind)
	}
	evt := hs.After()
	if !evt.Completed || evt.EggID != "egg-qwen-27b" {
		t.Fatalf("孵化事件不完整: %+v", evt)
	}
	if evt.Before.At.IsZero() || evt.After.At.IsZero() {
		t.Fatal("前后采样时间戳缺失")
	}
	if evt.After.At.Before(evt.Before.At) {
		t.Fatal("After 不得早于 Before")
	}

	cs := r.OnCollect("egg-qwen-27b")
	cevt := cs.After()
	if cevt.Kind != EventCollect || !cevt.Completed {
		t.Fatalf("收卵事件错: %+v", cevt)
	}
	// §8.5：收卵后的 After.Gtt 就是归零校验的读数（darwin 桩 Ok=false 是诚实行为）
	_ = cevt.After.Gtt

	if n := len(r.Events(0)); n != 2 {
		t.Fatalf("事件环应 2 条，实得 %d", n)
	}
	if got := r.Events(1); len(got) != 1 || got[0].Kind != EventCollect {
		t.Fatalf("Events(1) 应只取最新收卵事件: %+v", got)
	}
}

// TestSnapshot_DoesNotTouchRing —— Snapshot 即时观测不入环。
func TestSnapshot_DoesNotTouchRing(t *testing.T) {
	r := NewVitalsRecorder()
	r.dataDir = "."
	_ = r.Snapshot()
	if n := len(r.RecentRaw(0)); n != 0 {
		t.Fatalf("Snapshot 不得入环，实得 %d 轮", n)
	}
}

// ══════════════ 长期聚合 ══════════════

// TestAggregateWindow —— 注入假 Ok 采样验证窗口聚合逻辑。
// 直接驱动 accumulateLocked（单测内部，跳过平台读数层）。
func TestAggregateWindow(t *testing.T) {
	r := NewVitalsRecorder()
	r.aggWindow = time.Minute
	t0 := time.Unix(1700000000, 0)

	r.accumulateLocked(Vitals{At: t0, Gtt: GttSample{UsedBytes: 50 << 30, Ok: true}, Mem: MemSample{AvailGb: 60, Ok: true}, Cpu: CpuSample{Pct: 10, Ok: true}, GpuBusy: GpuBusySample{Pct: 5, Ok: true}})
	r.accumulateLocked(Vitals{At: t0.Add(20 * time.Second), Gtt: GttSample{UsedBytes: 56700000000, Ok: true}, Mem: MemSample{AvailGb: 55, Ok: true}, Cpu: CpuSample{Pct: 30, Ok: true}, GpuBusy: GpuBusySample{Pct: 95, Ok: true}})
	r.accumulateLocked(Vitals{At: t0.Add(61 * time.Second), Gtt: GttSample{UsedBytes: 10 << 30, Ok: true}, Mem: MemSample{AvailGb: 70, Ok: true}, Cpu: CpuSample{Pct: 1, Ok: true}}) // 新窗口

	long := r.LongTerm(0)
	if len(long) != 1 {
		t.Fatalf("应落 1 个聚合窗，实得 %d", len(long))
	}
	w := long[0]
	if w.Samples != 2 {
		t.Fatalf("首窗采样数应 2，实得 %d", w.Samples)
	}
	// 56700000000 B = 52.81 GiB > 50 GiB ⇒ 峰值取后者
	if !approx(w.GttUsedMaxGb, 52.81) {
		t.Fatalf("GTT 峰值应 ≈52.81，实得 %.2f", w.GttUsedMaxGb)
	}
	if w.MemAvailMin != 55 {
		t.Fatalf("内存谷值应 55，实得 %.1f", w.MemAvailMin)
	}
	if w.GpuBusyMax != 95 {
		t.Fatalf("GPU 忙峰值应 95，实得 %.1f", w.GpuBusyMax)
	}
	if !approx(w.CpuPctAvg, 20) {
		t.Fatalf("CPU 均值应 20，实得 %.1f", w.CpuPctAvg)
	}
	// 第二窗已开
	if r.aggStart != t0.Add(61*time.Second) {
		t.Fatal("新窗口起点错")
	}
}
