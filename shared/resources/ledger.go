// Package resources 是《设计-资源管理器》的纯计算地基（共享模块）：
// 资源账本（"机器 × 模型"两层）的数据结构与聚合查询、"跑得动吗"的三要素可解释估算、
// 以及驱逐优先级五档的可测排序表达与"只卸够"的动作计划。
//
// 为什么住在 shared/ 而不是 core/internal/：主控（core）与子端（agent）是两个独立 Go 模块，
// 而 Go 的 internal 规则只允许 core 模块内的路径导入 core/internal/...；
// 子端的驻留/驱逐裁决必须与主控用**同一份**裁决逻辑（否则两侧规则会各自漂移），
// 故实现上移到第三方共享模块，core 侧以 core/internal/resources 转发层保持既有导入路径不变。
//
// 本包纪律（对应设计稿 §三/§四/§八 与 §五 非目标）：
//   - 纯函式：不接线、不做任何 IO、不调系统命令、不联网、不读时钟（时间一律用相对量入参）。
//   - 诚实优先：凡用经验常量回退，结果必标 Estimated=true；估不出即 fail-closed（判"装不下/不可判定"）。
//   - 边界：本包只回答"这台机器 + 这个上下文长度，跑不跑得动"；不选端点、不改路由（§3.5）。
//   - 只出计划，不动手：本包只表达"该卸谁/卸几个够"，真正的卸载由调用方（子端 manager / 主控 gateway）执行。
//
// 本包不含任何真机数据读取——账本字段由调用方（心跳/本机快照）填入，本包只做聚合与判定。
package resources

import "fmt"

// DefaultMaxResident 是默认驻留上限（§八 Q1：默认单槽，多槽需显式开）。
// 它取代既有硬编码的 maxResident=3——默认 1 才与"实测单槽"这一事实一致。
const DefaultMaxResident = 1

// 驻留状态取值（对齐既有引擎侧状态机 loading/ready/crashed/idle；zombie 表达"僵尸"进程）。
const (
	StateLoading = "loading"
	StateReady   = "ready"
	StateCrashed = "crashed"
	StateIdle    = "idle"
	StateZombie  = "zombie"
)

// bytesPerGiB 是 GiB→字节换算基数。
const bytesPerGiB = 1 << 30

// gbToBytes 把 GiB 换算为字节。
func gbToBytes(gb float64) int64 { return int64(gb * float64(bytesPerGiB)) }

// bytesToGb 把字节换算为 GiB。
func bytesToGb(b int64) float64 { return float64(b) / float64(bytesPerGiB) }

// ResidentEntry 是一台机器上的一个驻留模型（身份用摘要，名字作别名）。
// 字段取值来源见设计稿 §3.1「驻留明细」：身份/路径/状态/最后使用/并发/实测内存/是否托管。
type ResidentEntry struct {
	Digest       string  `json:"digest"`                  // 模型身份（内容摘要），对齐登记库
	Alias        string  `json:"alias,omitempty"`         // 名字仅作别名
	File         string  `json:"file"`                    // 权重路径
	State        string  `json:"state"`                   // loading|ready|crashed|idle|zombie
	LastUsedAgoS float64 `json:"last_used_ago_s"`         // 距最后使用的秒数（LRU 判据）
	ReqCount     int     `json:"req_count"`               // 活跃请求数（>0 = 有在飞请求）
	RssGb        float64 `json:"rss_gb,omitempty"`        // 进程实测内存
	Managed      bool    `json:"managed"`                 // 是否在托管清单（Q6：手工服务=False）
	MemGb        float64 `json:"mem_gb,omitempty"`        // 模型声明内存需求
	CtxWindow    int     `json:"ctx_window,omitempty"`    // 上下文上限
	WeightsBytes int64   `json:"weights_bytes,omitempty"` // 权重（量化后）字节数——驱逐"腾得多"判据
	Source       string  `json:"source,omitempty"`        // 由谁装载（managed/manual/external）
	Pinned       bool    `json:"pinned,omitempty"`        // Q5：人工 pin
	PinRemainS   float64 `json:"pin_ttl_s,omitempty"`     // Q5：pin 剩余 TTL 秒（<=0 视为已到期，可被驱逐）
	WaitBound    bool    `json:"wait_bound,omitempty"`    // Q2⑤：被别处等待/会话粘性绑定
}

// PinActive 报告该驻留项的 pin 是否仍然有效（Q5：pin 必须带 TTL，无 TTL 的 pin 视为已到期）。
func (r ResidentEntry) PinActive() bool { return r.Pinned && r.PinRemainS > 0 }

// ChurnSensitive 报告该驻留项是否属于"最不该赶"的一类（Q2⑤：被别处等待/粘性绑定）。
func (r ResidentEntry) ChurnSensitive() bool { return r.WaitBound }

// occupiedGb 返回驻留项的占用估计（GiB）：声明内存优先，缺失时回退实测 RSS。
func occupiedGb(r ResidentEntry) float64 {
	if r.MemGb > 0 {
		return r.MemGb
	}
	return r.RssGb
}

// OccupiedGb 是 occupiedGb 的对外口径：一个驻留项的占用估计（GiB），
// 声明内存（mem_gb）优先，缺失时回退实测 RSS。调用方（子端腾退记账）用它，
// 保证"腾出多少"两侧算法一致——不许多算，也不许少算。
func OccupiedGb(r ResidentEntry) float64 { return occupiedGb(r) }

// UnmanagedProcess 是"未托管但占着资源"的进程/端口（覆盖实测 E2：手工 screen 起的服务）。
// 本包只如实呈现，不自动接管、不自动杀（§八 Q6）。Managed 恒 false——如实标注。
type UnmanagedProcess struct {
	PID     int     `json:"pid,omitempty"`
	Port    int     `json:"port,omitempty"`
	Command string  `json:"command,omitempty"`
	Model   string  `json:"model,omitempty"`
	RssGb   float64 `json:"rss_gb,omitempty"`
	Note    string  `json:"note,omitempty"`
	Managed bool    `json:"managed"` // 恒 false：未托管项如实标注（Q6）
}

// MachineLedger 是一台机器的资源账本（§四.2 形状）。
// 本机（local）与远程子端共用同一口径（§八 Q7）——调用方负责填字段，本包不区分来源。
type MachineLedger struct {
	Machine      string             `json:"machine"`
	MemTotalGb   float64            `json:"mem_total_gb"`
	MemAvailGb   float64            `json:"mem_available_gb"`
	VramTotalGb  float64            `json:"vram_total_gb,omitempty"`
	VramUsedGb   float64            `json:"vram_used_gb,omitempty"`
	VramFreeGb   float64            `json:"vram_free_gb,omitempty"`
	GpuPct       float64            `json:"gpu_pct"`
	Load         float64            `json:"load,omitempty"`
	BackendState string             `json:"backend_state"`
	Resident     []ResidentEntry    `json:"resident"`
	Unmanaged    []UnmanagedProcess `json:"unmanaged,omitempty"`

	// EngineOverheadGb 是引擎运行时/临时缓冲的固定开销（§3.2 overhead，"可配置"）。
	// <=0 表示未提供，估算时回退经验常量并标 estimated=true。
	EngineOverheadGb float64 `json:"engine_overhead_gb,omitempty"`

	// UnifiedMemory 报告该机器的显存与内存是不是同一个池（Apple Silicon 统一内存：显存即内存）。
	// 设计稿 §3.1：Apple Silicon 走统一内存，显存即内存 —— 此时无独立显存额度，按内存口径判。
	// 为假**且**显存未知（VramKnown=false）时是"真未知"：不能排除显存不足，
	// 判"装不下"（fail-closed，绝不把"未知"当"无限"默默放行）。
	UnifiedMemory bool `json:"unified_memory,omitempty"`
}

// VramKnown 报告该机器是否提供独立显存额度（§4.1：字段缺省=该机器未提供）。
// 有独立显存（如 X3 的 ROCm 卡）→ true，此时"装得下吗"必须与内存口径取严。
func (m MachineLedger) VramKnown() bool { return m.VramTotalGb > 0 }

// VramUnknown 报告该机器的显存是"真未知"：既拿不到独立显存额度，也不是统一内存。
// 设计稿 §3.2/§3.3d 的诚实边界：此时不得按"显存无限"放行 —— 必须 fail-closed 判装不下。
// （统一内存不算未知：显存即内存，内存口径已表达该约束。）
func (m MachineLedger) VramUnknown() bool { return !m.VramKnown() && !m.UnifiedMemory }

// ResidentCount 当前驻留项数量。
func (m MachineLedger) ResidentCount() int { return len(m.Resident) }

// FindResident 按摘要（或别名）查一个驻留项。
func (m MachineLedger) FindResident(id string) (ResidentEntry, bool) {
	for _, r := range m.Resident {
		if r.Digest == id || (id != "" && r.Alias == id) {
			return r, true
		}
	}
	return ResidentEntry{}, false
}

// ResidentOccupiedGb 当前驻留的总占用（GiB）——"当前驻留总占用多少"。
func (m MachineLedger) ResidentOccupiedGb() float64 {
	var sum float64
	for _, r := range m.Resident {
		sum += occupiedGb(r)
	}
	return sum
}

// ResidentWeightsBytes 当前驻留项的权重总字节数（"权重大小"聚合）。
func (m MachineLedger) ResidentWeightsBytes() int64 {
	var sum int64
	for _, r := range m.Resident {
		sum += r.WeightsBytes
	}
	return sum
}

// InflightResidents 正在被使用的驻留项（req_count > 0）——"每个模型正在被谁用"。
func (m MachineLedger) InflightResidents() []ResidentEntry {
	var out []ResidentEntry
	for _, r := range m.Resident {
		if r.ReqCount > 0 {
			out = append(out, r)
		}
	}
	return out
}

// HasInflight 报告该机器是否存在任何在飞请求。
func (m MachineLedger) HasInflight() bool {
	for _, r := range m.Resident {
		if r.ReqCount > 0 {
			return true
		}
	}
	return false
}

// ManagedResidents 在托管清单内的驻留项。
func (m MachineLedger) ManagedResidents() []ResidentEntry {
	var out []ResidentEntry
	for _, r := range m.Resident {
		if r.Managed {
			out = append(out, r)
		}
	}
	return out
}

// UnmanagedResidents 不在托管清单内、却被记为驻留的项（Q6：如实标注，不改其状态）。
func (m MachineLedger) UnmanagedResidents() []ResidentEntry {
	var out []ResidentEntry
	for _, r := range m.Resident {
		if !r.Managed {
			out = append(out, r)
		}
	}
	return out
}

// FreeMemGbForNew 是"新增模型可用内存"口径：当前可用 + 当前驻留占用
// （对齐既有 manager 的 availForNew = memAvail + residentGB，§3.2 比较式）。
func (m MachineLedger) FreeMemGbForNew() float64 {
	return m.MemAvailGb + m.ResidentOccupiedGb()
}

// RoomForNeedGb 是账本级的"这台机器还能装下 X 吗"。
// maxResident<=0 时取默认单槽（Q1）。返回 (能否, 原因)。
//
// 口径（§3.3d）：内存与显存**二者取严**（任一不够即判装不下）；
// 显存未知**且非统一内存** → fail-closed 判装不下（不把"未知"当"无限"放行）。
func (m MachineLedger) RoomForNeedGb(needGb float64, maxResident int) (bool, string) {
	if maxResident <= 0 {
		maxResident = DefaultMaxResident
	}
	if n := m.ResidentCount(); n >= maxResident {
		return false, fmt.Sprintf("驻留已满：%d/%d（默认单槽，多槽需显式开）", n, maxResident)
	}
	if m.MemAvailGb < 0 {
		return false, "内存可用量为负：输入非法（fail-closed）"
	}
	if free := m.FreeMemGbForNew(); needGb > free {
		return false, fmt.Sprintf("内存不足：需 %.2f GiB > 可腾退后可用 %.2f GiB", needGb, free)
	}
	if m.VramUnknown() {
		return false, "显存未知且非统一内存：无法排除显存不足（fail-closed，不把未知当无限）"
	}
	if m.VramKnown() && needGb > m.VramFreeGb {
		return false, fmt.Sprintf("显存不足：需 %.2f GiB > 空闲 %.2f GiB", needGb, m.VramFreeGb)
	}
	return true, "装得下"
}
