// egg_profile_cache.go —— 卵实测档案的进程内缓存 + 唯一读入口 + 显式刷新（2026-09-19 ①）。
//
// 现象（真机，逐字）：盘上档案已更新为 `peak_gtt_gb: 74.567 / peak_mem_gb: 74.882`，而子端日志仍打印
//
//	双闸门通过: deepseek-v4-flash（… 档案 peak_gtt=14.5 GB peak_mem=15.2 GB）
//
// （旧值）⇒ 闸门按旧值放行。而 `/infer/reload`（server.handleReload）**只重载注册表，不重载档案**。
//
// ⚠ 定位结论（施工时把 backend 侧每一处读点都核过，如实记，不猜）：
//
//	本文件落地**之前**，backend 侧**没有**档案缓存字段 —— profileGateLocked / hatchGateLocked /
//	hatchPrecheckLocked 三处各自 `monitor.LoadEggProfile(monitor.EggProfilePath(entry.EggName()))`
//	**现读盘上**；`/eggs` 的 has_profile 与 server 的 estimateWaitETA 也是现读。也就是说「闸门按旧
//	peak 放行」在代码上不可能来自 backend 的缓存（更可能是路径口径：档案在**子端侧**
//	`~/.zerg/egg-profiles/`，在本机写无效——报告附录第 2 条已记；或读点的 egg 名与本机模型名不一致）。
//
// 本批据此把「档案只能来自盘上、且能被一条命令显式刷新」从**碰巧如此**变成**有字段、有调用点、有用例**：
//
//	① 建出显式缓存字段（Manager.profileCache）+ **唯一读入口** eggProfileFresh(Locked)：
//	   现读盘上（`os.ReadFile` + 校验）后才回写缓存。孵化路径（/load 及其静态预检）一律走它
//	   ——「孵化前现读盘上档案」从此是不变量，不是巧合；
//	② `RefreshEggProfiles` 供 `/infer/reload` 调用：清缓存 → 对**驻留卵 + 注册表卵**重新现读
//	   → 返回 (成功数, 问题清单)。缓存**只在这里**被整体刷新（口径写死：其余读点要么现读、
//	   要么读缓存，绝没有第二处会写它）；
//	③ 观测面（`/eggs` 的 has_profile）改读缓存——它不再每次轮询都去 `os.ReadFile`，而
//	   「重标定后不重启就生效」由 ② 那条刷新保证（刷新前读的是上一份已校验档案，刷新后是新的事实）。
//
// 缓存内容**只有校验通过的档案**（LoadEggProfile 已含 Validate）：残档案不进缓存，读失败一律
// 如实报错、由调用方 fail-closed 拒孵（标定铁律 §8.4 不变）。
package backend

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
)

// eggProfileCacheEntry 缓存的一格：已校验档案 + 它的来源路径与载入时刻（复盘要看"这份事实是何时从
// 哪个文件读来的"）。
type eggProfileCacheEntry struct {
	prof     monitor.EggProfile
	path     string
	loadedAt time.Time
}

// eggProfileFreshLocked **现读**盘上档案并回写缓存（调用方必须已持 m.mu —— 见下面 eggProfileFresh）。
//
// 语义（写死）：
//   - 先读盘、校验通过，才动缓存；读失败 ⇒ 返回错误且**不把旧缓存当答案**（调用方 fail-closed）；
//   - 缓存里原有的旧值会被新值覆盖（这就是"重标定后生效"的那一步）。
func (m *Manager) eggProfileFreshLocked(eggID string) (monitor.EggProfile, error) {
	eggID = strings.TrimSpace(eggID)
	path := monitor.EggProfilePath(eggID)
	if path == "" {
		return monitor.EggProfile{}, fmt.Errorf("卵名拿不到，实测档案路径算不出来")
	}
	prof, err := monitor.LoadEggProfile(path) // 现读：os.ReadFile + 校验（成本可忽略）
	if err != nil {
		return monitor.EggProfile{}, err
	}
	if m != nil {
		if m.profileCache == nil {
			m.profileCache = map[string]eggProfileCacheEntry{}
		}
		m.profileCache[eggID] = eggProfileCacheEntry{prof: prof, path: path, loadedAt: time.Now()}
	}
	return prof, nil
}

// eggProfileFresh 现读盘上档案（**取锁版**：调用方不持锁时用它）。
//
// ⚠ 锁序：任何已持 m.mu 的路径都必须调 eggProfileFreshLocked，否则当场自锁死（sync.Mutex 不可重入
// ——本项目在 Start/RequestModel 上已经踩过一次，缺陷 1）。
func (m *Manager) eggProfileFresh(eggID string) (monitor.EggProfile, error) {
	if m == nil {
		return monitor.EggProfile{}, fmt.Errorf("没有 Manager")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.eggProfileFreshLocked(eggID)
}

// CachedEggProfile 读缓存（**不落盘、不现读**）：返回 (已校验档案, 是否在缓存里)。
//
// 用途：观测面/用例核对"刷新有没有把新事实吃进来"。**不许**拿它当孵化判据的来源（判据要现读，
// 见 eggProfileFreshLocked 的调用点）。
func (m *Manager) CachedEggProfile(eggID string) (monitor.EggProfile, bool) {
	if m == nil {
		return monitor.EggProfile{}, false
	}
	eggID = strings.TrimSpace(eggID)
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.profileCache[eggID]
	if !ok {
		return monitor.EggProfile{}, false
	}
	return e.prof, true
}

// EggProfileAvailable 观测面用（`/eggs` 的 has_profile）：这枚卵有没有**可用**实测档案。
//
// 读缓存；缓存里没有 ⇒ 现读一次并回填（所以首次轮询与"从未孵过的卵"也照样如实回答）。
// 读不到 ⇒ false（如实，绝不编造 has_profile=true —— §8.4）。
func (m *Manager) EggProfileAvailable(eggID string) bool {
	if m == nil {
		return false
	}
	if _, ok := m.CachedEggProfile(eggID); ok {
		return true
	}
	_, err := m.eggProfileFresh(eggID)
	return err == nil
}

// RefreshEggProfiles 刷新档案缓存（**/infer/reload 调它**）：清缓存 → 对每个已知卵重新现读。
//
// 目标集合 = 注册表里所有卵 ∪ 当前驻留卵（去重后按名排序，输出可复现）。返回：
//
//	ok       —— 现读并校验通过、已入缓存的枚数；
//	problems —— 读不到 / 校验不过的卵（**如实列出**：不许把"有档案读不出来"说成"刷新完成"）。
//
// 语义（与 handleReload 的注册表重载同一条理）：刷新是**尽力而为 + 留痕**，不因某一枚档案坏了就
// 让整个 reload 失败（那会让别的卵的刷新白做）；但问题必须回给调用方，由它写进响应。
//
// 缓存里不属于这两个集合的条目会被清掉（它们要么已经被删档、要么是历史残留——下次现读会回来）。
func (m *Manager) RefreshEggProfiles() (int, []string) {
	if m == nil {
		return 0, nil
	}
	targets := m.eggProfileRefreshTargets()
	m.mu.Lock()
	m.profileCache = map[string]eggProfileCacheEntry{}
	m.mu.Unlock()

	ok := 0
	var problems []string
	for _, eggID := range targets {
		if _, err := m.eggProfileFresh(eggID); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", eggID, err))
			continue
		}
		ok++
	}
	return ok, problems
}

// eggProfileRefreshTargets 刷新目标（注册表卵 ∪ 驻留卵；去重 + 排序）。
func (m *Manager) eggProfileRefreshTargets() []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	// 注册表（重载后最新的一份；handleReload 先重载注册表再刷新档案）
	if m.registry != nil {
		for _, name := range m.registry.Names() {
			if e, ok := m.registry.Get(name); ok && e != nil {
				add(e.EggName())
			}
		}
	}
	// 驻留卵（注册表里可能已经没有它了，但它还占着资源）
	m.mu.Lock()
	for _, sp := range m.procs {
		if sp != nil && sp.entry != nil {
			add(sp.entry.EggName())
		}
	}
	m.mu.Unlock()
	sort.Strings(out)
	return out
}
