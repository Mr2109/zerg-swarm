package api

// resource_trust.go — 资源信任度标记（2026-08-21 Mr2109）
// 所有资源（模型/工具/skill/mcp）——新入库 🆕 → 实际任务用 100 次无故障 → ✅ 正式（标记消除）
// 存储: /tmp/zerg-resources.json（状态/使用次数/故障次数）

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

const (
	resTrustFile = "/tmp/zerg-resources.json"
	resTrustMax  = 100 // 100 次无故障转正式（Mr2109）
)

// ResTrustEntry 资源信任状态
type ResTrustEntry struct {
	Status string `json:"status"` // 新/正式
	Uses   int    `json:"uses"`   // 使用次数（任务用到）
	Faults int    `json:"faults"` // 故障次数（调用失败）
	Since  string `json:"since"`  // 入库时间
}

// ResourceTrust 资源信任表
type ResourceTrust struct {
	mu     sync.Mutex
	Models map[string]ResTrustEntry `json:"models"`
	Tools  map[string]ResTrustEntry `json:"tools"`
	Skills map[string]ResTrustEntry `json:"skills"`
	Mcps   map[string]ResTrustEntry `json:"mcps"`
}

var resourceTrust = &ResourceTrust{
	Models: map[string]ResTrustEntry{},
	Tools:  map[string]ResTrustEntry{},
	Skills: map[string]ResTrustEntry{},
	Mcps:   map[string]ResTrustEntry{},
}

// LoadResourceTrust 加载信任表（启动时调用——存量资源默认正式）
func LoadResourceTrust() {
	if b, err := os.ReadFile(resTrustFile); err == nil {
		var t ResourceTrust
		if json.Unmarshal(b, &t) == nil {
			resourceTrust.mu.Lock()
			resourceTrust.Models = t.Models
			resourceTrust.Tools = t.Tools
			resourceTrust.Skills = t.Skills
			resourceTrust.Mcps = t.Mcps
			resourceTrust.mu.Unlock()
		}
	}
	// 存量资源默认正式（一直在用的——不是新入库——Mr2109 2026-08-21）
	// 新资源（未来入库）才 🆕——存量工具/模型直接正式
	resourceTrust.mu.Lock()
	now := nowStr()
	for k, m := range resourceTrust.Models {
		if m.Status == "" || m.Status == "未知" {
			m.Status = "正式"
			m.Since = now
			resourceTrust.Models[k] = m
		}
	}
	resourceTrust.mu.Unlock()
}

// saveResourceTrust 保存信任表
func saveResourceTrust() {
	b, _ := json.MarshalIndent(resourceTrust, "", "  ")
	os.WriteFile(resTrustFile, b, 0o644)
}

// ensureEntry 获取条目（不存在则创建——新资源默认 🆕）
func (t *ResourceTrust) ensureEntry(m map[string]ResTrustEntry, name string) ResTrustEntry {
	e, ok := m[name]
	if !ok {
		e = ResTrustEntry{Status: "新", Since: nowStr()}
		m[name] = e
	}
	return e
}

// RegisterResource 注册资源（新入库——默认 🆕）
func (t *ResourceTrust) RegisterResource(kind, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.kindMap(kind)
	if m == nil {
		return
	}
	if _, ok := m[name]; !ok {
		m[name] = ResTrustEntry{Status: "新", Since: nowStr()}
		saveResourceTrust()
	}
}

// RegisterExistingTools 存量工具注册为正式（启动时——一直在用的工具——不是新入库）
func RegisterExistingTools() {
	resourceTrust.mu.Lock()
	defer resourceTrust.mu.Unlock()
	now := nowStr()
	for _, td := range agent.AllTools() {
		name := td.Function.Name
		if _, ok := resourceTrust.Tools[name]; !ok {
			resourceTrust.Tools[name] = ResTrustEntry{Status: "正式", Uses: 100, Since: now} // 存量信任
		}
	}
	saveResourceTrust()
}

// RegisterExistingModels 存量模型注册为正式（启动时——fleet 配置里的模型——不是新接入）
func RegisterExistingModels(cfg *config.FleetConfig) {
	resourceTrust.mu.Lock()
	defer resourceTrust.mu.Unlock()
	now := nowStr()
	if cfg != nil {
		for name := range cfg.Models {
			if _, ok := resourceTrust.Models[name]; !ok {
				resourceTrust.Models[name] = ResTrustEntry{Status: "正式", Uses: 100, Since: now} // 存量信任
			}
		}
	}
	saveResourceTrust()
}

// UseResource 资源被使用（计数 +1——100 次无故障转正式）
func (t *ResourceTrust) UseResource(kind, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.kindMap(kind)
	if m == nil {
		return
	}
	e := t.ensureEntry(m, name)
	e.Uses++
	if e.Status == "新" && e.Uses >= resTrustMax && e.Faults == 0 {
		e.Status = "正式" // 100 次无故障——标记消除
	}
	m[name] = e
	saveResourceTrust()
}

// FaultResource 资源故障（调用失败——计数+1——不转正）
func (t *ResourceTrust) FaultResource(kind, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.kindMap(kind)
	if m == nil {
		return
	}
	e := t.ensureEntry(m, name)
	e.Faults++
	m[name] = e
	saveResourceTrust()
}

// GetResourceStatus 资源状态（UI 显示——🆕/正式/未知）
func (t *ResourceTrust) GetResourceStatus(kind, name string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.kindMap(kind)
	if m == nil {
		return "未知"
	}
	if e, ok := m[name]; ok {
		return e.Status
	}
	return "未知" // 未注册（存量资源——未知状态——不标记）
}

// GetResourceUses 资源使用次数（UI 显示——调用 N 次）
func (t *ResourceTrust) GetResourceUses(kind, name string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.kindMap(kind)
	if m == nil {
		return 0
	}
	if e, ok := m[name]; ok {
		return e.Uses
	}
	return 0
}

// GetResourceFaults 资源故障次数（UI 显示——红色警告）
func (t *ResourceTrust) GetResourceFaults(kind, name string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.kindMap(kind)
	if m == nil {
		return 0
	}
	if e, ok := m[name]; ok {
		return e.Faults
	}
	return 0
}

// GetResourceSince 资源入库时间（UI 显示）
func (t *ResourceTrust) GetResourceSince(kind, name string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.kindMap(kind)
	if m == nil {
		return "-"
	}
	if e, ok := m[name]; ok {
		return e.Since
	}
	return "-"
}

// kindMap 资源类型对应表
func (t *ResourceTrust) kindMap(kind string) map[string]ResTrustEntry {
	switch kind {
	case "models":
		return t.Models
	case "tools":
		return t.Tools
	case "skills":
		return t.Skills
	case "mcp":
		return t.Mcps
	}
	return nil
}

// ResourceTrustSnapshot 信任表快照（API 返回）
func (t *ResourceTrust) Snapshot() map[string]interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return map[string]interface{}{
		"models": t.Models,
		"tools":  t.Tools,
		"skills": t.Skills,
		"mcps":   t.Mcps,
	}
}

func nowStr() string {
	return time.Now().Format("2006-01-02T15:04:05")
}
