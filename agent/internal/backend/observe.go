// observe.go —— P7 批 1：观测面只读装配（/eggs、/services 共用的真源出口）。
//
// 设计真源：设计稿 §5.4（观测面）——「权威状态只放一处：状态机在子端，观测面只读」。
//
// 红线（本文件自我约束）：
//   - 只读：不写任何 Manager 状态，不改 P2 判定逻辑
//     （acquireInflight / releaseInflight / ReapIdle 语义原样，p2_lifecycle.go 未动）；
//   - 如实：字段取不到就缺席（零值 + omitempty），不编造；
//   - 排序：按 egg_id 排序，输出可复现。
package backend

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
)

// EngineImplOf 实际会执行的引擎实现名（设计 §1.2：引擎实现/变体是卵的必需字段）。
//
// 判据与 serviceKind 同源（residency.go），按「实际会执行什么」给真值：
//  1. 卵声明了 engine_impl → 原样采用（声明即事实；与实际执行的相符性由孵化映射校验）；
//  2. cmd: → 取首词的可执行「可辨识短名」（见 shortEngineName）；
//  3. backend 声明且非 llama 家族（如 "ds4-server"）→ 原样返回（那是引擎家族名）；
//  4. 其余（llama 家族 / 未声明）→ **按实际探测到的可执行文件给真值**：
//     例：X3 上是 `build-hip-flash/llama-server` —— 这正是「卵要体现出用的是什么引擎」：
//     只取基名会把 hip-flash 与主线 llama-server 混成一个名字，两枚不同的卵看起来一样。
//     探测不到任何可执行文件时退回 "llama-server"。
func EngineImplOf(entry *registry.ModelEntry) string {
	if entry == nil {
		return shortEngineName(detectLlamaServerPath())
	}
	if v := strings.TrimSpace(entry.EngineImpl); v != "" {
		return v
	}
	if cmd := strings.TrimSpace(string(entry.Cmd)); cmd != "" {
		if f := strings.Fields(cmd); len(f) > 0 {
			return shortEngineName(f[0])
		}
	}
	if b := strings.TrimSpace(entry.Backend); b != "" && b != "llama-server" {
		return b
	}
	return shortEngineName(detectLlamaServerPath())
}

// engineImplGenericDirs 命名时跳过的通用可执行目录：这些目录名不携带「哪个引擎实现」的信息。
var engineImplGenericDirs = map[string]bool{
	"bin": true, "sbin": true, "lib": true, "libexec": true,
	"usr": true, "local": true, "opt": true, "homebrew": true, ".brew": true,
}

// shortEngineName 把引擎可执行路径压成「可辨识短名」：跳过通用目录，
// 取最近的一个非通用目录名 + 基名 ⇒ **不同的 build / fork 彼此可区分**。
//
//	/home/g01/llama.cpp-src/build-hip-flash/bin/llama-server → build-hip-flash/llama-server
//	/home/g01/llama-k2/build-k2/bin/llama-server             → build-k2/llama-server
//	/home/g01/agent/run-k2.sh                                → agent/run-k2.sh
//	/opt/homebrew/bin/llama-server                           → llama-server
//	llama-server                                             → llama-server
func shortEngineName(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	clean := filepath.Clean(p)
	base := filepath.Base(clean)
	dir := filepath.Dir(clean)
	for i := 0; i < 8; i++ { // 有界上溯：最多看 8 层，避免病态路径下空转
		d := filepath.Base(dir)
		if d == "" || d == "." || d == string(filepath.Separator) {
			return base
		}
		if !engineImplGenericDirs[d] {
			return d + "/" + base
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return base
		}
		dir = parent
	}
	return base
}

// EggObservation 观测面单枚卵的只读事实卡（server 层 /eggs 与 /services 的装配输入）。
//
// 字段缺席 = 本端真没有（零值缺席，不编造）：
//   - IdleArmedRemainS nil = 不在空窗计时中（或已到期待巡检收走）；
//   - SchemaVersion 0 = 卵声明未声明格式版本号（遗留条目，如实标出）；
//   - Unit 空 = 孵化器（P1 单元化）尚未落地，无单元归属可报；
//   - RssGb 是进程实测 RSS——逐进程 GTT 归因按 pid 匹配由 server 层接线（§8.7）。
type EggObservation struct {
	EggID      string // 注册表卵名（=驻留键，真 egg_id）
	Model      string
	EngineImpl string // 实际会执行的引擎实现名（EngineImplOf）
	Unit       string // 孵化器单元归属（P1 单元化落地前恒空）
	State      string // ready / loading / idle_armed / draining / crashed / sleeping
	Port       int
	PID        int // 引擎进程 pid（0 = 无进程句柄）；逐进程 GTT 归因与排除的键
	Inflight   int // P2 在飞引用计数（唯一真源）
	// IdleArmedRemainS 空窗计时剩余秒数；nil = 不在空窗计时中。
	// 与 IdleArmedRemainingS 同口径（同一把锁内的状态 + 每卵阈值），只是随快照一次读出。
	IdleArmedRemainS *float64
	SchemaVersion    int     // 卵声明格式版本号；0 = 未声明（如实标出）
	RssGb            float64 // 进程实测 RSS（GB）；GTT 逐进程口径由 server 层按 PID 接线
}

// PIDSet 当前托管卵的引擎进程 pid 集合（只读快照；§8.7 排除本端托管项用）。
func (m *Manager) PIDSet() map[int]struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[int]struct{}, len(m.procs))
	for _, sp := range m.procs {
		if sp != nil && sp.proc != nil && sp.proc.Process != nil {
			out[sp.proc.Process.Pid] = struct{}{}
		}
	}
	return out
}

// EggObservations 当前托管卵的只读快照（按 egg_id 排序，可复现）。
// 单次持锁出一份自洽快照；空窗剩余秒数在同一临界区内按同一 now 计算（与状态一致）。
func (m *Manager) EggObservations() []EggObservation {
	m.mu.Lock()
	now := time.Now()
	out := make([]EggObservation, 0, len(m.procs))
	for name, sp := range m.procs {
		if sp == nil || name == "" {
			continue
		}
		o := EggObservation{
			EggID:      name,
			Model:      name,
			EngineImpl: EngineImplOf(sp.entry),
			State:      sp.state,
			Port:       sp.port,
			Inflight:   sp.inflight,
		}
		if sp.entry != nil {
			o.SchemaVersion = sp.entry.SchemaVersion
		}
		if sp.proc != nil && sp.proc.Process != nil {
			o.PID = sp.proc.Process.Pid
			o.RssGb = readProcessRssGb(sp.proc.Process.Pid)
		}
		if sp.state == StateIdleArmed {
			// 与 IdleArmedRemainingS 同口径：阈值 − 距最后活动；已到期如实给 0
			//（到期但巡检尚未收走的短暂窗口，不谎报为正值）。
			remain := eggIdleUnloadThreshold(sp.entry) - now.Sub(sp.lastUsed)
			if remain < 0 {
				remain = 0
			}
			r := remain.Seconds()
			o.IdleArmedRemainS = &r
		}
		out = append(out, o)
	}
	m.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].EggID < out[j].EggID })
	return out
}
