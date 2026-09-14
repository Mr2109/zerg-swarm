// vitals.go —— 体征器：五类采集 + 频率分层 + 环形缓冲 + 事件钩子。
//
// 设计真源：《设计-子端隔离化-20260914》（仓库内文档标题含「沙箱」二字，为避免
// 代码里新增该词，此处以「隔离化」指代同一文件）。
//   - §8.2 度量口径：全局 GTT（mem_info_gtt_used）是账的主口径（F5），
//     逐进程 fdinfo 降为交叉校验 + 归因展示（回答"是谁占的"）。
//   - §8.5：收卵后必须校验 GTT 归零 ⇒ OnCollect 的 After 采样就是归零校验的读数来源。
//   - §5.4：体征器是只读观测面——本文件只读数、只入内存环形缓冲，**绝不持续写盘**。
//   - §11 P3：全局 GTT 读取是本批核心工作量。
//
// 五类采集（频率分层，快采 1-2s 一轮）：① 全局 GTT ② 内存 ③ CPU（/proc/stat 差分）
// ④ GPU 忙（gpu_busy_percent）⑤ 盘空间（statfs /data）。
// 慢采（10-15s 一次）：逐进程 fdinfo 归因（drm-memory-gtt，交叉校验口径）。
//
// 平台分层：Linux 专有读数在 vitals_readers_linux.go（build tag linux）；
// darwin/其它平台一律 ok=false 的诚实桩（学 vram.go「未知不冒充」的纪律）。
// 本文件全部平台无关，两个平台都编译、都能单测。
package monitor

import (
	"sync"
	"time"
)

// ══════════════ 五类采样值（每类都带 Ok——拿不到就如实说拿不到） ══════════════

// GttSample 全局 GTT（APU 上"显存"的主口径，§8.2）。字节为真源单位（sysfs 原文）。
type GttSample struct {
	UsedBytes  uint64
	TotalBytes uint64
	Ok         bool // false = 该平台/该机器读不到（绝不编 0 冒充）
}

// UsedGb 把字节换算成 GiB（与本仓 GB 用法一致：KiB/1024/1024 同口径）。
func (g GttSample) UsedGb() float64 { return byteToGb(g.UsedBytes) }

// TotalGb 把字节换算成 GiB。
func (g GttSample) TotalGb() float64 { return byteToGb(g.TotalBytes) }

// MemSample 内存账（CPU 侧口径，真源 MemAvailable，§8.3）。
type MemSample struct {
	AvailGb float64
	TotalGb float64
	Ok      bool
}

// CpuSample CPU 忙闲（/proc/stat 差分；差分需要两次采样，首轮 Ok=false）。
type CpuSample struct {
	Pct float64
	Ok  bool
}

// GpuBusySample GPU 忙闲（amdgpu gpu_busy_percent；多卡取最大值）。
type GpuBusySample struct {
	Pct float64
	Ok  bool
}

// DiskSample 盘空间（statfs 数据盘——X3 上是 /data，权重与 KV 盘所在）。
type DiskSample struct {
	FreeGb  float64
	TotalGb float64
	Ok      bool
}

// Vitals 一轮快采的完整体征（五类各一）。
type Vitals struct {
	At      time.Time
	Gtt     GttSample     // ① 全局 GTT（主口径）
	Mem     MemSample     // ② 内存（MemAvailable）
	Cpu     CpuSample     // ③ CPU
	GpuBusy GpuBusySample // ④ GPU 忙
	Disk    DiskSample    // ⑤ 盘空间
}

// ══════════════ 逐进程归因（慢采，交叉校验口径） ══════════════

// ProcAttrib 单个进程的 GTT 归因（fdinfo drm-memory-gtt，只读）。
// 红线（§12.1）：只读数，绝不代表可以对它动手。
type ProcAttrib struct {
	PID   int
	Argv0 string
	GttGb float64
}

// AttribSample 一轮逐进程归因（慢采）。
// 注意口径：逐进程之和 ≠ 全局总量是官方明确的行为（§8.2）⇒ SumGb 只作交叉校验，
// 总账永远以 GttSample（全局读数）为准。
type AttribSample struct {
	At    time.Time
	Procs []ProcAttrib
	SumGb float64
	Ok    bool
}

// ══════════════ 事件钩子（孵化 / 收卵前后各取一次） ══════════════

// VitalsEventKind 事件类别：孵化 / 收卵。
type VitalsEventKind string

const (
	EventHatch   VitalsEventKind = "hatch"   // 孵化：Before=装前，After=装后（含装载峰值语境）
	EventCollect VitalsEventKind = "collect" // 收卵：Before=停前，After=停后（§8.5 GTT 归零校验读它）
)

// VitalsEvent 一次孵化/收卵事件的前后两次采样。
type VitalsEvent struct {
	Kind      VitalsEventKind
	EggID     string
	Before    Vitals
	After     Vitals
	Completed bool // After 已取
}

// ══════════════ 长期聚合（从快采累计，仍是内存态） ══════════════

// VitalsAggregate 一个聚合窗口（默认 1 分钟）的统计。
type VitalsAggregate struct {
	WindowStart  time.Time
	WindowEnd    time.Time
	GttUsedMaxGb float64 // 窗口内 GTT 峰值
	MemAvailMin  float64 // 窗口内可用内存谷值
	CpuPctAvg    float64
	GpuBusyMax   float64
	Samples      int
}

// ══════════════ 环形缓冲（泛型，内存态，绝不落盘） ══════════════

type ringBuf[T any] struct {
	buf   []T
	head  int // 下一写入位
	count int
}

func newRing[T any](cap int) ringBuf[T] { return ringBuf[T]{buf: make([]T, cap)} }

// Push 写入一格（满则覆盖最旧）。
func (r *ringBuf[T]) Push(v T) {
	r.buf[r.head] = v
	r.head = (r.head + 1) % len(r.buf)
	if r.count < len(r.buf) {
		r.count++
	}
}

// Slice 按旧→新返回副本。
func (r *ringBuf[T]) Slice() []T {
	out := make([]T, 0, r.count)
	start := (r.head - r.count + len(r.buf)) % len(r.buf)
	for i := 0; i < r.count; i++ {
		out = append(out, r.buf[(start+i)%len(r.buf)])
	}
	return out
}

// ══════════════ 体征器本体 ══════════════

// 频率与容量默认值（快采 2s / 慢采 15s / 聚合窗 1min）。
// 标定铁律（§8.4）：这些是采样节奏不是容量阈值，不属"凡数字必实测"的账；可调。
const (
	DefaultFastInterval  = 2 * time.Second
	DefaultSlowInterval  = 15 * time.Second
	DefaultAggWindow     = time.Minute
	recentRawCap         = 180  // 2s × 180 = 6 分钟原始快采
	attribCap            = 240  // 15s × 240 = 1 小时归因
	eventCap             = 64   // 孵化/收卵事件（低频）
	longAggCap           = 1440 // 1min × 1440 = 24 小时长期聚合
	defaultVitalsDataDir = "/data"
)

// VitalsRecorder 体征器：五类采集 + 频率分层 + 环形缓冲 + 事件钩子。线程安全。
//
// 频率分层的驱动方式：调用方（backend 接线，P4/P7）定时调 MaybeCollect(now)——
// 到期才真正采样，不到期立即返回 ⇒ 分层节奏完全由调用方的时钟注入控制，
// 本体不起 goroutine（不阻塞、不偷跑）。测试可注入任意 now 做无睡眠验证。
type VitalsRecorder struct {
	mu sync.Mutex

	fastInterval time.Duration
	slowInterval time.Duration
	aggWindow    time.Duration
	dataDir      string // 盘空间探测点（X3 = /data）

	recent ringBuf[Vitals]       // 近期原始（快采）
	attrib ringBuf[AttribSample] // 逐进程归因（慢采）
	events ringBuf[VitalsEvent]  // 事件钩子
	long   ringBuf[VitalsAggregate]

	lastFast time.Time
	lastSlow time.Time

	// CPU /proc/stat 差分的上一次读数
	lastCpuTotal uint64
	lastCpuBusy  uint64
	hasLastCpu   bool

	// 聚合窗口累计器
	aggStart     time.Time
	aggHasStart  bool
	aggGttMaxGb  float64
	aggMemMinGb  float64
	aggCpuSum    float64
	aggCpuN      int
	aggGpuMax    float64
	aggMemHasVal bool
}

// NewVitalsRecorder 创建体征器（默认节奏：快 2s / 慢 15s / 聚合窗 1min）。
func NewVitalsRecorder() *VitalsRecorder {
	return &VitalsRecorder{
		fastInterval: DefaultFastInterval,
		slowInterval: DefaultSlowInterval,
		aggWindow:    DefaultAggWindow,
		dataDir:      defaultVitalsDataDir,
		recent:       newRing[Vitals](recentRawCap),
		attrib:       newRing[AttribSample](attribCap),
		events:       newRing[VitalsEvent](eventCap),
		long:         newRing[VitalsAggregate](longAggCap),
	}
}

// MaybeCollect 频率分层入口：快采到期才采样五类，慢采到期才做逐进程归因。
// 返回本轮是否真的采了样。两次调用之间没有任何阻塞等待（读 sysfs/proc 均为毫秒级）。
func (r *VitalsRecorder) MaybeCollect(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	sampled := false
	if r.lastFast.IsZero() || now.Sub(r.lastFast) >= r.fastInterval {
		v := r.sampleVitalsLocked(now)
		r.recent.Push(v)
		r.accumulateLocked(v)
		r.lastFast = now
		sampled = true
	}
	if r.lastSlow.IsZero() || now.Sub(r.lastSlow) >= r.slowInterval {
		if a := readProcAttribution(); a.Ok {
			a.At = now
			r.attrib.Push(a)
		}
		r.lastSlow = now
	}
	return sampled
}

// sampleVitalsLocked 一轮五类快采（调用方持锁）。
func (r *VitalsRecorder) sampleVitalsLocked(now time.Time) Vitals {
	v := Vitals{At: now}
	v.Gtt = readGlobalGtt()
	avail, total := sampleMem() // 复用既有采样（mem.go：Linux /proc/meminfo，macOS sysctl/vm_stat）
	v.Mem = MemSample{AvailGb: avail, TotalGb: total, Ok: total > 0}
	v.Cpu = r.sampleCpuLocked()
	v.GpuBusy = readGpuBusy()
	v.Disk = readDataDisk(r.dataDir)
	return v
}

// sampleCpuLocked /proc/stat 差分：本轮与上轮的 busy 占比。首轮没有基准 ⇒ Ok=false。
func (r *VitalsRecorder) sampleCpuLocked() CpuSample {
	total, busy, ok := readProcStatCpu()
	if !ok {
		return CpuSample{}
	}
	defer func() {
		r.lastCpuTotal, r.lastCpuBusy, r.hasLastCpu = total, busy, true
	}()
	if !r.hasLastCpu || total <= r.lastCpuTotal {
		return CpuSample{} // 首轮 / 时钟回拨：不给数，不编数
	}
	dTotal := float64(total - r.lastCpuTotal)
	dBusy := float64(busy - r.lastCpuBusy)
	if dTotal <= 0 {
		return CpuSample{}
	}
	pct := dBusy / dTotal * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return CpuSample{Pct: pct, Ok: true}
}

// accumulateLocked 把一轮快采累计进当前聚合窗口；窗口到期则先落盘（内存环）再开新窗。
func (r *VitalsRecorder) accumulateLocked(v Vitals) {
	if r.aggHasStart && v.At.Sub(r.aggStart) >= r.aggWindow {
		r.long.Push(VitalsAggregate{
			WindowStart:  r.aggStart,
			WindowEnd:    r.aggStart.Add(r.aggWindow),
			GttUsedMaxGb: r.aggGttMaxGb,
			MemAvailMin:  r.aggMemMinGb,
			CpuPctAvg:    avgOrZero(r.aggCpuSum, r.aggCpuN),
			GpuBusyMax:   r.aggGpuMax,
			Samples:      r.aggCpuN, // 快采轮数
		})
		r.aggHasStart = false
	}
	if !r.aggHasStart {
		r.aggStart = v.At
		r.aggHasStart = true
		r.aggGttMaxGb, r.aggMemMinGb, r.aggCpuSum, r.aggCpuN, r.aggGpuMax = 0, 0, 0, 0, 0
		r.aggMemHasVal = false
	}
	if v.Gtt.Ok && v.Gtt.UsedGb() > r.aggGttMaxGb {
		r.aggGttMaxGb = v.Gtt.UsedGb()
	}
	if v.Mem.Ok {
		if !r.aggMemHasVal || v.Mem.AvailGb < r.aggMemMinGb {
			r.aggMemMinGb = v.Mem.AvailGb
			r.aggMemHasVal = true
		}
	}
	if v.Cpu.Ok {
		r.aggCpuSum += v.Cpu.Pct
		r.aggCpuN++
	}
	if v.GpuBusy.Ok && v.GpuBusy.Pct > r.aggGpuMax {
		r.aggGpuMax = v.GpuBusy.Pct
	}
}

func avgOrZero(sum float64, n int) float64 {
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// ══════════════ 事件钩子：孵化 / 收卵（本批只提供 API，backend 接线归 P4/P7） ══════════════

// EggSpan 一次孵化/收卵事件的前半程：创建时已采 Before。
type EggSpan struct {
	kind   VitalsEventKind
	eggID  string
	before Vitals
	r      *VitalsRecorder
}

// OnHatch 孵化钩子：调用即采 Before（装载前）。装载完成后调 span.After() 采 After 并落事件环。
// 用法（backend 接线时）：
//
//	span := rec.OnHatch(eggID)
//	... 装载 ...
//	evt := span.After()
func (r *VitalsRecorder) OnHatch(eggID string) *EggSpan {
	return r.beginSpan(EventHatch, eggID)
}

// OnCollect 收卵钩子：调用即采 Before（停止前——§8.4 教训 2：释放量要在 stop 之前记下来）。
// 停完后调 span.After() 采 After——After 的 Gtt 就是 §8.5「GTT 归零校验」的读数。
func (r *VitalsRecorder) OnCollect(eggID string) *EggSpan {
	return r.beginSpan(EventCollect, eggID)
}

func (r *VitalsRecorder) beginSpan(kind VitalsEventKind, eggID string) *EggSpan {
	r.mu.Lock()
	before := r.sampleVitalsLocked(time.Now())
	r.recent.Push(before)
	r.mu.Unlock()
	return &EggSpan{kind: kind, eggID: eggID, before: before, r: r}
}

// After 采事件后半程（孵化=装好后；收卵=停完后）并落事件环。重复调用只记最后一次。
func (s *EggSpan) After() VitalsEvent {
	s.r.mu.Lock()
	defer s.r.mu.Unlock()
	after := s.r.sampleVitalsLocked(time.Now())
	s.r.recent.Push(after)
	evt := VitalsEvent{Kind: s.kind, EggID: s.eggID, Before: s.before, After: after, Completed: true}
	s.r.events.Push(evt)
	return evt
}

// ══════════════ 只读访问面（观测面 §5.4 的数据来源） ══════════════

// Snapshot 立即采一轮五类体征（即时观测，不入环——周期环只归 MaybeCollect 管）。
func (r *VitalsRecorder) Snapshot() Vitals {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sampleVitalsLocked(time.Now())
}

// RecentRaw 近期原始快采（旧→新，至多 n 轮；n<=0 给全部）。
func (r *VitalsRecorder) RecentRaw(n int) []Vitals {
	r.mu.Lock()
	defer r.mu.Unlock()
	return tail(r.recent.Slice(), n)
}

// RecentAttrib 近期逐进程归因（旧→新，至多 n 轮）。
func (r *VitalsRecorder) RecentAttrib(n int) []AttribSample {
	r.mu.Lock()
	defer r.mu.Unlock()
	return tail(r.attrib.Slice(), n)
}

// Events 近期孵化/收卵事件（旧→新，至多 n 条）。
func (r *VitalsRecorder) Events(n int) []VitalsEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return tail(r.events.Slice(), n)
}

// LongTerm 长期聚合窗口（旧→新，至多 n 个）。
func (r *VitalsRecorder) LongTerm(n int) []VitalsAggregate {
	r.mu.Lock()
	defer r.mu.Unlock()
	return tail(r.long.Slice(), n)
}

func tail[T any](s []T, n int) []T {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// byteToGb 字节 → GiB（与仓内 GB 口径一致）。
func byteToGb(b uint64) float64 { return float64(b) / (1024 * 1024 * 1024) }
