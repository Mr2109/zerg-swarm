package gateway

// ds4_route.go — DS4 大模型让位机制（2026-08-23 Mr2109）
// Hermes 调 DS4 熔断（81G 超大——X3 内存不够）——路由 DS4 时自动清场（卸载其他模型只留 DS4）
// 其他模型请求时——按 DS4 状态三分支（未加载→X3 / 闲置→卸 DS4 / 繁忙→local）
// 对齐 llama.cpp router models-memory-margin（动态卸载——业界标准）

import (
	"fmt"
	"log"
	"net/http"
	"time"
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

// ensureDS4Room 确保 DS4 可加载（路由目标=DS4 时——X3 清场）
// 如果 X3 有其他模型占用内存——调 X3 agent 卸载（只留 DS4）
// 返回: 清场是否成功（可继续路由）
func (g *Gateway) ensureDS4Room() bool {
	snap := g.snapshotFor("x3")
	if snap == nil {
		return true // 无快照——不阻塞（熔断逻辑兜底）
	}
	// DS4 已加载——直接可用（不用清场）
	if snap.Model != nil && *snap.Model == ds4ModelName {
		return true
	}
	// X3 有其他模型占用（或空闲）——需要清场（卸载其他——只留 DS4）
	if snap.Model != nil || snap.MemAvailableGb < ds4MemGB {
		log.Printf("🧹 DS4 让位: X3 当前模型=%v 内存余量=%.0fG——卸载腾位（只留 DS4）", 
			snap.Model, snap.MemAvailableGb)
		return g.unloadX3Models()
	}
	return true
}

// unloadX3Models 调 X3 agent 卸载模型（清场——释放内存）
// X3 agent 端点: /unload（agent 服务 8100——认证 X-Auth-Token）
func (g *Gateway) unloadX3Models() bool {
	// X3 agent 卸载接口（<worker-ip>:8100/unload——认证 x3gw-shared-2026）
	url := "http://<worker-ip>:8100/unload"
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", "x3gw-shared-2026")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️ DS4 清场失败（X3 agent 不可达）: %v", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		log.Printf("✅ DS4 清场成功——X3 模型已卸载（只留 DS4）")
		return true
	}
	log.Printf("⚠️ DS4 清场响应 %d——继续尝试", resp.StatusCode)
	return false
}

// ensureX3RoomForFile — 2026-09-09(诊断 R2/R4): X3 模型驻留无回收→内存打满→实例挂死
// 通用让位(DS4 机制推广): 路由目标=X3 且目标模型未加载 + X3 可用内存 < 所需 → 先 unload(agent 清场按需再载)
// 幂等: 已加载/内存够 → no-op(agent 自行按需加载);DS4 场景其专用分支已先清场——此处二次进入自动 no-op
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
	// 新加载模型推理被饿(实例 0% CPU 假健康)。内存够但 GPU 忙(≥90%)→同样清场单驻留
	if snap.MemAvailableGb >= float64(memGB) && snap.GpuPct < 90 {
		return // 内存+GPU 都够——X3 agent 按需加载
	}
	if g.unloadX3Models() {
		log.Printf("🧹 X3 让位(通用): 路由 %s 需 %dG 可用 %.0fG GPU %.0f%%——已清场(X3 agent 按需单驻留)", file, memGB, snap.MemAvailableGb, snap.GpuPct)
	}
}

// ds4RouteDecision 其他模型请求时——按 DS4 状态路由决策
// 返回: 是否强制走 local（true=其他模型走本机——不打扰 DS4）
func (g *Gateway) ds4RouteDecision() (forceLocal bool, reason string) {
	switch g.ds4Status() {
	case ds4Busy:
		return true, "DS4 繁忙（X3 推理中——其他模型走本机）"
	case ds4Idle:
		// DS4 闲置——调用其他模型 = 卸 DS4（X3 跑其他——不空置）
		g.unloadX3Models()
		return false, "DS4 闲置——已卸载让位（X3 跑其他模型）"
	default:
		return false, "DS4 未加载——X3 正常"
	}
}

// debug helper（保留——日志用）
var _ = fmt.Sprintf
