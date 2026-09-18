package api

// resource_trust.go — 资源信任度标记（2026-08-21 Mr2109）
// 所有资源（模型/工具/skill/mcp）——新入库 🆕 → 实际任务用 100 次无故障 → ✅ 正式（标记消除）
//
// 存储（2026-09-18 修 /tmp 硬编码——口径同本包 tasks_persist.go）:
//
//	原写死 const resTrustFile = "/tmp/zerg-resources.json" —— macOS 重启 /tmp 即清 +
//	tmp_cleaner 3 天未访问即删（实测本机两者都在）⇒ 使用次数/故障次数/转正标记全部归零
//	（用了 99 次的资源又一次回到「🆕」）；多实例还共用同一份文件互相覆盖。
//	改为 statepath 统一状态目录派生（ZERG_STATE_DIR → ~/.zerg/state/zerg-resources.json）。
//	迁移兼容（首次）: 新路径不存在而旧 /tmp/zerg-resources.json 存在 ⇒ **读旧一次**（不丢计数）；
//	**写只写新路径**；旧文件**不删、不改**。新路径已存在 ⇒ 旧路径完全不看。

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

const (
	resTrustMax = 100 // 100 次无故障转正式（Mr2109）

	// resTrustStateFileName 信任表在统一状态目录下的文件名。
	resTrustStateFileName = "zerg-resources.json"
	// resTrustLegacyDefaultPath 旧硬编码落点字面量（2026-08-21 起写死）。
	// 只作**首次迁移读取**来源：读一次；不删、不改、永不写入。
	resTrustLegacyDefaultPath = "/tmp/zerg-resources.json"
)

// legacyResTrustFile 旧路径（包级变量 = 上面的字面量；迁移用例/测试进程隔离可切换）。
var legacyResTrustFile = resTrustLegacyDefaultPath

// resTrustFile 信任表落点（包级变量——测试可切换隔离路径——2026-08-21）。
// 语义（2026-09-18）: 非空 ⇒ 直接用该路径（测试隔离/显式覆盖）；
// 空 ⇒ 走 statepath 统一状态目录派生（生产默认，见 resTrustWritePath）。
var resTrustFile = ""

// resTrustWritePath 写路径：永远是统一状态目录（ZERG_STATE_DIR → ~/.zerg/state/<resTrustStateFileName>）。
// 永不写旧 /tmp 路径——多实例各写各的（不再互相覆盖计数）。
func resTrustWritePath() string {
	if p := strings.TrimSpace(resTrustFile); p != "" {
		return p
	}
	return statepath.File(resTrustStateFileName)
}

// resTrustReadPath 读路径：统一状态目录优先；新路径不存在且旧 /tmp 存在 ⇒ 读旧一次（迁移兼容）。
// 新路径存在 ⇒ 旧路径完全不看（不 stat、不读——旧内容不夹除）。
// 显式覆盖 resTrustFile（测试隔离）时不退旧路径：覆盖即「我已指定唯一来源」，避免测试读真机 /tmp。
func resTrustReadPath() string {
	p := resTrustWritePath()
	if _, err := os.Stat(p); err == nil {
		return p // 新路径已存在——旧路径完全不看
	}
	if strings.TrimSpace(resTrustFile) != "" {
		return p
	}
	legacy := strings.TrimSpace(legacyResTrustFile)
	if legacy == "" {
		return p
	}
	if _, err := os.Stat(legacy); err != nil {
		return p // 无旧文件——首次运行（空信任表）
	}
	log.Printf("📜 资源信任表首次迁移: 读旧 %s（只读一次——旧文件保留不删；写入只落 %s）\n", legacy, p)
	return legacy
}

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
// 读路径（2026-09-18）: 新（统一状态目录）优先；新缺失 + 旧 /tmp 在 ⇒ 读旧一次（迁移兼容）。
func LoadResourceTrust() {
	if b, err := os.ReadFile(resTrustReadPath()); err == nil {
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
// 写只写新路径（统一状态目录，目录首次自动建）——旧 /tmp 路径永不写（2026-09-18）
func saveResourceTrust() {
	b, _ := json.MarshalIndent(resourceTrust, "", "  ")
	dst := resTrustWritePath()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		log.Printf("⚠️ 资源信任表目录不可用（%s）: %v\n", dst, err)
		return
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		log.Printf("⚠️ 资源信任表写盘失败（%s）: %v\n", dst, err)
	}
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
