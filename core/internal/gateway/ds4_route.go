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
