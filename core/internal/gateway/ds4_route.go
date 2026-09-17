package gateway

// ds4_route.go — DS4 大模型让位机制（2026-08-23 Mr2109）
// Hermes 调 DS4 熔断（81G 超大——X3 内存不够）——路由 DS4 时自动清场（卸载其他模型只留 DS4）
// 其他模型请求时——按 DS4 状态三分支（未加载→X3 / 闲置→卸 DS4 / 繁忙→local）
// 对齐 llama.cpp router models-memory-margin（动态卸载——业界标准）
//
// 批 3 改动（《设计-资源管理器》§3.3d / §4.3 / §八 Q2）：
//   1. **只卸够**：不再"把 X3 上别的全卸掉"，而是由共享裁决（resources.EvictPlanForAction）
//      给出"按五档顺序、刚好腾出缺口的那几个"，逐个点名让子端卸载；卸一个够就只卸一个。
//   2. **去硬编码 URL**：地址从 fleet.yaml 的 fleet 段解析（fleetNode/agentURLFor），
//      不再写死 `http://<worker-ip>:8100/unload`。
//   3. **红线**：有在飞请求时绝不强卸（既有铁律，保留并加注释）；未托管项（子端如实报
//      managed=false，如手工 screen 起的服务）绝不出现在让位计划里——只报告，不接管不杀（Q6）。
//
// 兼容性（重要，如实写明）：主控先升级、子端还是批 2 之前的版本时，快照里没有 resident[]
// （驻留账本为空）——此时**无法公平裁决**，退回旧口径"请子端全卸"并记日志（见 yieldX3To）。
// 子端升级后（有账本）自动走"只卸够"。

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/resources"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"
)

// DS4 模型名（fleet.yaml 注册名）
const ds4ModelName = "deepseek-v4-flash"

// ds4MemGB DS4 内存需求（模型 81G + 上下文余量——fleet.yaml mem_gb: 86）
const ds4MemGB = 86

// ds4State DS4 状态（路由三分支用）
type ds4State int

const (
	ds4NotLoaded ds4State = iota // DS4 未加载（X3 自由）
	ds4Idle                      // DS4 已加载 + 无请求（可卸载让位）
	ds4Busy                      // DS4 已加载 + 有请求（繁忙——其他走 local）
)

// ds4Status 判断 DS4 状态（加载/闲置/繁忙）
// 信号: X3 快照 Model（加载）+ ActiveRequests（请求）
func (g *Gateway) ds4Status() ds4State {
	snap := g.snapshotFor("x3")
	if snap == nil {
		return ds4NotLoaded
	}
	// DS4 是否加载（X3 快照 model == DS4）
	loaded := snap.Model != nil && *snap.Model == ds4ModelName
	if !loaded {
		return ds4NotLoaded
	}
	// 加载了——有请求 = 繁忙，无请求 = 闲置
	if snap.ActiveRequests > 0 {
		return ds4Busy
	}
	return ds4Idle
}

// ensureDS4Room 确保 DS4 可加载（路由目标=DS4 时——X3 让位）
// 只卸够：算出"还差多少内存"，再按五档裁决只卸需要的那些（见 yieldX3To）。
// 返回: 让位是否成功（可继续路由）
func (g *Gateway) ensureDS4Room() bool {
	snap := g.snapshotFor("x3")
	if snap == nil {
		return true // 无快照——不阻塞（熔断逻辑兜底）
	}
	// DS4 已加载——直接可用（不用让位）
	if snap.Model != nil && *snap.Model == ds4ModelName {
		return true
	}
	// X3 有其他模型占用（或内存不够）——需要让位
	if snap.Model != nil || snap.MemAvailableGb < ds4MemGB {
		needGb := ds4MemGB - snap.MemAvailableGb
		if needGb < 0 {
			needGb = 0
		}
		// 单槽现实：X3 上已有别的模型驻留时也要腾出那个占用者（两个模型不能同时占 X3）
		if used := x3UsedGb(snap); used > needGb {
			needGb = used
		}
		return g.yieldX3To(snap, needGb, "ds4")
	}
	return true
}

// x3UsedGb 快照口径下 X3 当前被占用的内存（GiB）；快照缺总量时返回 0（不编造）。
func x3UsedGb(snap *store.FleetSnapshot) float64 {
	if snap == nil || snap.MemTotalGb <= 0 {
		return 0
	}
	used := snap.MemTotalGb - snap.MemAvailableGb
	if used < 0 {
		return 0
	}
	return used
}

// yieldX3To —— 主控让位（批 3：只卸够 + 只动我们的）。
//
// needGb = "还差多少内存才算够"；计划由共享裁决给出：按五档顺序（① 崩溃/僵尸 →
// ③ 空闲 LRU 最旧、同档权重更大者先 → ⑤ 被别处等待/粘性者最后）取"够用的最小前缀"。
// 红线：有在飞请求时绝不强卸（子端侧还会再拦一道——它在飞/未到期 pin 项会跳过并报原因）。
// 未托管项（managed=false）永远不进计划：别人的进程我们不接管、不杀（Q6）。
//
// 返回 true 表示"没有阻塞"（无事可做或让位请求已发出），false 表示腾不出来（调用方只记日志，
// 不改变路由决策——与既有实现一致）。
func (g *Gateway) yieldX3To(snap *store.FleetSnapshot, needGb float64, why string) bool {
	if snap == nil {
		return true
	}
	// 既有铁律：有在飞请求时绝不强卸（会杀活跃推理）——路由打分自会转 local/等待
	if snap.ActiveRequests > 0 {
		log.Printf("🧹 X3 yield skipped (%s): %d in-flight request(s) — never unload active inference", why, snap.ActiveRequests)
		return false
	}
	if needGb <= 0 {
		return true // 内存够——无需让位（子端按需装载自会单驻留）
	}
	if len(snap.Resident) == 0 {
		// 驻留账本缺席（子端版本早于批 2）：没有账本就无法公平裁决——退回旧口径"全卸"，
		// 并如实记日志（不冒充"已按五档让位"）。
		log.Printf("🧹 X3 yield (%s): need %.0fG but resident ledger absent (子端未上报驻留明细) — falling back to legacy full unload", why, needGb)
		return g.unloadX3Models()
	}
	plan := resources.EvictPlanForAction(snap.Resident, needGb)
	if len(plan) == 0 {
		// 无可动作项：全在飞 / 全 pin 未到期 / 全是未托管进程——一条都不许动
		log.Printf("🧹 X3 yield denied (%s): need %.0fG but no evictable resident (inflight/pinned/unmanaged) — leaving X3 untouched", why, needGb)
		return false
	}
	freed := resources.PlannedFreeGb(snap.Resident, plan)
	log.Printf("🧹 X3 yield (%s): need %.0fG, unload %d/%d resident %v (freed≈%.0fG) — just enough, not everything",
		why, needGb, len(plan), len(snap.Resident), plan, freed)
	return g.postX3Unload(plan)
}

// unloadX3Models 调 X3 agent 卸载模型（全卸——旧口径）。
// 保留给两条路径：① 驻留账本缺席时的兼容回退；② 任何仍需要"清场"的显式调用。
// 地址来自配置（不硬编码）；认证 X-Auth-Token。
func (g *Gateway) unloadX3Models() bool {
	return g.postX3Unload(nil)
}

// postX3Unload 向 X3 agent 发卸载请求：targets 非空 = 只卸这几项；为空 = 全卸（旧语义）。
// 子端 POST /unload 带 {"models":[...]} 即定向卸载；不带体即全卸（老子端忽略体，行为不变）。
func (g *Gateway) postX3Unload(targets []string) bool {
	// X3 agent 卸载接口——地址取自 fleet.yaml 的 fleet 段（批 3 去掉硬编码 IP）
	url := g.agentURLFor("x3", "/unload")
	if url == "" {
		log.Printf("⚠️ DS4 quiesce skipped: no fleet node configured for x3")
		return false
	}
	var body io.Reader
	if len(targets) > 0 {
		b, err := json.Marshal(map[string][]string{"models": targets})
		if err != nil {
			log.Printf("⚠️ DS4 quiesce skipped: marshal targets failed: %v", err)
			return false
		}
		body = bytes.NewReader(b)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", config.ResolveAuthToken())
	// T1.6 传播（控制面出站）：/unload 也是"主控→子端"，同样带 traceparent（否则一条链在控制面断掉）
	tracectx.Propagate(req.Header, nil, "", tracectx.ReplayMarked())
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️ DS4 quiesce failed (X3 agent unreachable at %s): %v", url, err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		if len(targets) > 0 {
			log.Printf("✅ DS4 quiesce succeeded — X3 unloaded %d targeted model(s) %v", len(targets), targets)
		} else {
			log.Printf("✅ DS4 quiesce succeeded — X3 model unloaded (DS4 only)")
		}
		return true
	}
	log.Printf("⚠️ DS4 quiesce returned %d — continuing", resp.StatusCode)
	return false
}

// ensureX3RoomForFile — 2026-09-09(诊断 R2/R4): X3 模型驻留无回收→内存打满→实例挂死
// 通用让位(DS4 机制推广): 路由目标=X3 且目标模型未加载 + X3 可用内存 < 所需 → 先只卸够(agent 再按需载)
// 幂等: 已加载/内存够 → no-op(agent 自行按需加载);DS4 场景其专用分支已先让位——此处二次进入自动 no-op
func (g *Gateway) ensureX3RoomForFile(file string, memGB int) {
	if file == "" || memGB <= 0 {
		return
	}
	snap := g.snapshotFor("x3")
	if snap == nil {
		return
	}
	// 有在飞请求时绝不强卸(会杀活跃推理)——路由打分自会转 local/等待
	if snap.ActiveRequests > 0 {
		return
	}
	if snap.Model != nil && modelFileLoaded(snap, file) {
		return // 目标已加载——无需让位
	}
	// 2026-09-09(诊断 R2 深化): GPU 也是单资源——X3 单卡,多实例常驻→gpu_pct=100→
	// 新加载模型推理被饿(实例 0% CPU 假健康)。内存够但 GPU 忙(≥90%)→同样让位到单驻留
	needGb := float64(memGB) - snap.MemAvailableGb
	if snap.MemAvailableGb >= float64(memGB) && snap.GpuPct < 90 {
		return // 内存+GPU 都够——X3 agent 按需加载
	}
	if snap.GpuPct >= 90 {
		// GPU 忙：要腾出的是"当前占用者"（不是全清）——用实测占用做缺口下限
		if used := x3UsedGb(snap); used > needGb {
			needGb = used
		}
	}
	g.yieldX3To(snap, needGb, "generic:"+file)
}

// ds4RouteDecision 其他模型请求时——按 DS4 状态路由决策
// 返回: 是否强制走 local（true=其他模型走本机——不打扰 DS4）
func (g *Gateway) ds4RouteDecision() (forceLocal bool, reason string) {
	switch g.ds4Status() {
	case ds4Busy:
		return true, "DS4 繁忙（X3 推理中——其他模型走本机）"
	case ds4Idle:
		// DS4 闲置——调用其他模型 = 卸 DS4 让位（X3 跑其他——不空置）
		// 批 3：只卸够——让位目标由五档裁决给出（闲置的 DS4 自己就是最该走的那一个），
		// 不是把 X3 上所有驻留一把全清。
		snap := g.snapshotFor("x3")
		needGb := x3UsedGb(snap)
		if needGb <= 0 {
			needGb = ds4MemGB // 快照缺总量/可用时的保守口径：腾出 DS4 所需
		}
		g.yieldX3To(snap, needGb, "ds4-idle")
		return false, "DS4 闲置——已让位（X3 跑其他模型）"
	default:
		return false, "DS4 未加载——X3 正常"
	}
}
