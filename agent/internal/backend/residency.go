// residency.go —— 子端驻留上限与五档驱逐裁决（《设计-资源管理器》§八 Q1/Q2/Q5，批 3）。
//
// 本文件把 manager.go 的驻留状态**翻译**成共享裁决库（shared/resources）的入参，
// 并把裁决结果变成"该卸哪一个/哪几个"的动作；裁决本身一行都不在这里——
// 在飞的绝不驱逐、未托管的绝不接管、pin 未到期的不驱逐、只卸够不多卸，
// 全部由共享包表达与测试（core 与 agent 同一份）。
//
// 四条红线（本批必须成立，测试逐条有断言）：
//
//	① 有在飞请求（reqCount>0）的驻留绝不驱逐——无论多旧、多大；
//	② 未托管进程绝不接管、绝不杀——本端 procs 只装"我们自己 spawn 的进程"，
//	   外来进程只由 resident.go 只读探测报告，从不进入本文件的任何动作路径；
//	③ pin 且 TTL 未到期者不驱逐（Q5：pin 必须带 TTL——无 TTL 的 Pin 直接报错）；
//	④ 内存预检 fail-closed：腾不出缺口就拒装（507 语义，见 manager.doStart）。
package backend

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
	"github.com/Mr2109/zerg-swarm/shared/resources"
)

// EnvMaxResident 是子端驻留上限的配置项名（环境变量，值为十进制整数）。
//
// 默认值 = 1（单槽）——来源：《设计-资源管理器》§八 Q1 拍板（"默认单槽（max_resident=1），
// 多槽需显式开启；取代现有硬编码 maxResident = 3"），与共享包常量
// resources.DefaultMaxResident 同值。之所以默认 1：实测本机 llama-server 一次只能驻留一个模型
// （第二个一来第一个就退出），而并行度在路由侧也恒为单槽——默认 1 才"如实"；
// 大机器（X3）要并发再显式设 ZERG_MAX_RESIDENT=<n>。
//
// 非法值（非数字/<=0）一律忽略并回落到默认值，且记一条日志——不静默接受怪值，
// 也不因为配错就放开上限（放宽上限等于放开 OOM）。
const EnvMaxResident = "ZERG_MAX_RESIDENT"

// resolveMaxResidentFromEnv 解析驻留上限配置（纯函式，便于测试注入 getenv）。
func resolveMaxResidentFromEnv(getenv func(string) string) int {
	if getenv == nil {
		return resources.DefaultMaxResident
	}
	raw := strings.TrimSpace(getenv(EnvMaxResident))
	if raw == "" {
		return resources.DefaultMaxResident
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Printf("[backend] %s=%q 非法（须为正整数）——回落到默认 %d", EnvMaxResident, raw, resources.DefaultMaxResident)
		return resources.DefaultMaxResident
	}
	return n
}

// resolveMaxResident 读环境变量得到本进程的驻留上限。
func resolveMaxResident() int { return resolveMaxResidentFromEnv(os.Getenv) }

// MaxResident 当前驻留上限（观测面/测试用）。
func (m *Manager) MaxResident() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.residentLimitLocked()
}

// SetMaxResident 显式设置驻留上限（<=0 表示回到默认单槽）。供配置/观测面调用。
func (m *Manager) SetMaxResident(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n <= 0 {
		n = resources.DefaultMaxResident
	}
	m.maxResident = n
}

// residentLimitLocked 取生效上限（<=0 视为默认单槽；调用方需持锁或已独占）。
func (m *Manager) residentLimitLocked() int {
	if m.maxResident <= 0 {
		return resources.DefaultMaxResident
	}
	return m.maxResident
}

// Pin 显式钉住一个驻留模型：TTL 内它不会被任何自动驱逐选中（Q5）。
// ttl<=0 一律拒绝——无 TTL 的 pin 等同内存泄漏（拍板原文）。
func (m *Manager) Pin(model string, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("pin 必须带正 TTL（%s：无 TTL 的 pin 等同内存泄漏）", EnvMaxResident)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	sp, ok := m.procs[model]
	if !ok {
		return fmt.Errorf("模型 %s 未驻留，无法 pin", model)
	}
	sp.pinUntil = time.Now().Add(ttl)
	log.Printf("[backend] pin: %s 锁定 %s（TTL 到期后自动可驱逐）", model, ttl)
	return nil
}

// Unpin 解除 pin（立刻恢复可驱逐）。
func (m *Manager) Unpin(model string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sp, ok := m.procs[model]
	if !ok {
		return fmt.Errorf("模型 %s 未驻留，无法 unpin", model)
	}
	sp.pinUntil = time.Time{}
	return nil
}

// PinRemainS 返回某模型 pin 的剩余秒数；ok=false 表示未 pin 或已到期。
func (m *Manager) PinRemainS(model string) (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sp, ok := m.procs[model]
	if !ok {
		return 0, false
	}
	active, remain := pinState(sp, time.Now())
	if !active {
		return 0, false
	}
	return remain, true
}

// pinState 计算 pin 是否有效及剩余秒数（零值 pinUntil = 未 pin）。
func pinState(sp *subproc, now time.Time) (bool, float64) {
	if sp == nil || sp.pinUntil.IsZero() || !sp.pinUntil.After(now) {
		return false, 0
	}
	return true, sp.pinUntil.Sub(now).Seconds()
}

// ── TTL 卸载（《设计-资源管理器》§3.3(c) 触发①：空闲超过 TTL → 卸载）──────────────
//
// 本项于本次验收才实现（取证：此前只有 pin 的 TTL，没有任何"空闲到期自动卸载"的路径——
// residency.go 只有 Pin/Unpin/pinState，manager.go 无回收循环）。
// 默认秒数由Mr2109 2026-09-13 拍板定为 **300 秒**（DefaultIdleTTL）；ZERG_MODEL_TTL_S 可覆盖，设 0 即关闭。

// EnvModelTTL 是驻留模型空闲 TTL 的配置项名（环境变量，单位：秒）。
const EnvModelTTL = "ZERG_MODEL_TTL_S"

// DefaultIdleTTL 是空闲 TTL 的默认值：**300 秒**（Mr2109 2026-09-13 拍板「300 秒」）。
//
// 语义：驻留模型连续空闲（无在飞请求、非加载中、pin 未生效）超过该时长即被卸载，把显存让出来；
// 想关掉就把 ZERG_MODEL_TTL_S 显式设成 0（逃生门），不要在代码里改这个常量。
const DefaultIdleTTL = 300 * time.Second

// resolveIdleTTLFromEnv 解析空闲 TTL（纯函式，便于测试注入 getenv）。
// 缺省/空（含 getenv 为 nil，即读不到环境）→ DefaultIdleTTL（300 秒）；
// 非法（非数字/负数）→ 记日志并回落 0（不启用）——配错时钟宁可不卸，也不按猜出来的数字卸。
func resolveIdleTTLFromEnv(getenv func(string) string) time.Duration {
	if getenv == nil {
		return DefaultIdleTTL
	}
	raw := strings.TrimSpace(getenv(EnvModelTTL))
	if raw == "" {
		return DefaultIdleTTL
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		log.Printf("[backend] %s=%q 非法（须为非负整数秒）——按未启用处理", EnvModelTTL, raw)
		return 0
	}
	return time.Duration(n) * time.Second
}

// resolveIdleTTL 读环境变量得到本进程的空闲 TTL。
func resolveIdleTTL() time.Duration { return resolveIdleTTLFromEnv(os.Getenv) }

// SetIdleTTL 显式设置空闲 TTL（<=0 = 不启用）。供配置/观测面/测试调用。
func (m *Manager) SetIdleTTL(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d < 0 {
		d = 0
	}
	m.idleTTL = d
}

// IdleTTL 当前空闲 TTL（<=0 表示未启用）。
func (m *Manager) IdleTTL() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.idleTTL
}

// reapIdleLegacy 是 P2 之前的 ReapIdle 实现（直判 lastUsed/reqCount，不经「空窗计时中」状态）。
// P2（设计 §7.7 修补 1–3）后生产路径只走 p2_lifecycle.go 的新 ReapIdle：
//   - 新版判据 = 状态==空窗计时中 且 到期 且 在飞引用计数==0（唯一真源），锁内赢权、出锁才停进程；
//   - 旧版直判 lastUsed/reqCount，无显式状态、无赢权语义——仅保留对照与故障回退用，
//     不得再被生产路径调用（reaper 循环已切到新版）。
func (m *Manager) reapIdleLegacy(now time.Time) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ttl := m.idleTTL
	if ttl <= 0 {
		return nil
	}
	var reaped []string
	for name, sp := range m.procs {
		if sp == nil || name == "" {
			continue
		}
		if sp.state == StateLoading {
			continue
		}
		if sp.reqCount > 0 {
			continue
		}
		if pinned, _ := pinState(sp, now); pinned {
			continue
		}
		if sp.lastUsed.IsZero() || now.Sub(sp.lastUsed) < ttl {
			continue // 未超过 TTL
		}
		m.evictSubprocLocked(name, sp, fmt.Sprintf("TTL 到期(空闲>%s)", ttl))
		reaped = append(reaped, name)
	}
	sort.Strings(reaped)
	return reaped
}

// StartIdleReaper 启动后台 TTL 回收循环：每 interval 按当前 TTL 回收一次空闲驻留。
// interval<=0 或 TTL 未启用 → 空操作（默认行为与改动前逐字一致，不产生后台 goroutine）。
// 已启动则不重复启动（reaperStop 非 nil 即视为在跑）。
func (m *Manager) StartIdleReaper(interval time.Duration) {
	if interval <= 0 {
		return
	}
	m.mu.Lock()
	if m.idleTTL <= 0 || m.reaperStop != nil {
		m.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	m.reaperStop = stop
	m.mu.Unlock()

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				m.ReapIdle(time.Now())
			case <-stop:
				return
			}
		}
	}()
}

// StopIdleReaper 停止后台 TTL 回收循环（未启动则空操作）。
func (m *Manager) StopIdleReaper() {
	m.mu.Lock()
	stop := m.reaperStop
	m.reaperStop = nil
	m.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

// residentEntriesLocked 把托管驻留翻译成裁决入参（调用方需持锁）。
//
// 三条如实口径：
//   - Managed 恒 true：procs 里只有本管理器 spawn 的进程；外来进程永远走 resident.go 的只读探测。
//   - 加载中的进程标 WaitBound（"被请求等待"）：它在裁决里排到档⑤，最后才考虑，
//     且共享层的动作准入还会以 loading 拦一道——不许把正在加载的进程杀掉。
//   - 取不到的字段一律缺席（digest/权重读不到就不编造），裁决按缺席处理。
//
// 返回顺序按别名排序（裁决内部会再排序；输入确定才能保证同分项输出可复现）。
func (m *Manager) residentEntriesLocked() []resources.ResidentEntry {
	now := time.Now()
	out := make([]resources.ResidentEntry, 0, len(m.procs))
	for name, sp := range m.procs {
		out = append(out, residentEntryOf(name, sp, now))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Alias < out[j].Alias })
	return out
}

// residentEntryOf 单个托管驻留 → 裁决入参。
func residentEntryOf(name string, sp *subproc, now time.Time) resources.ResidentEntry {
	e := resources.ResidentEntry{
		Alias:    name,
		State:    sp.state,
		ReqCount: sp.reqCount,
		Managed:  true,
		Source:   "managed",
	}
	if !sp.lastUsed.IsZero() {
		e.LastUsedAgoS = now.Sub(sp.lastUsed).Seconds()
	}
	if sp.entry != nil {
		e.File = sp.entry.File
		e.MemGb = sp.entry.MemGB
		e.CtxWindow = entryCtxWindow(sp.entry)
		e.Digest = entryDigest(sp.entry)
		e.WeightsBytes = entryWeightsBytes(sp.entry)
	}
	if sp.state == StateLoading {
		e.WaitBound = true // 正被请求等待（比在飞更早的阶段）——排档⑤
	}
	if pinned, remain := pinState(sp, now); pinned {
		e.Pinned = true
		e.PinRemainS = remain
	}
	if sp.proc != nil && sp.proc.Process != nil {
		e.RssGb = readProcessRssGb(sp.proc.Process.Pid)
	}
	return e
}

// entryWeightsBytes 取该模型权重的"贵重"排序键（§八 Q2 规则④：同档权重更大者先被赶）。
// 来源优先级：注册条目里的显式 weights_bytes → 权重文件实测大小 → 0（缺席，不编造）。
// 注意：多分片模型（...-00001-of-000NN.gguf）此处只看到首片大小——它只用于排序，不是账本真值。
func entryWeightsBytes(e *registry.ModelEntry) int64 {
	if e == nil {
		return 0
	}
	if e.Custom != nil {
		switch v := e.Custom["weights_bytes"].(type) {
		case int:
			return int64(v)
		case int64:
			return v
		case float64:
			return int64(v)
		}
	}
	if e.File == "" {
		return 0
	}
	if fi, err := os.Stat(strings.Replace(e.File, "~", os.Getenv("HOME"), 1)); err == nil {
		return fi.Size()
	}
	return 0
}

// evictVictimsLocked 按五档裁决挑出"为腾出 needGb 内存"该卸的目标（只挑，不动手）。
func (m *Manager) evictVictimsLocked(needGB float64) []string {
	if needGB <= 0 {
		return nil
	}
	entries := m.residentEntriesLocked()
	byTarget := make(map[string]resources.ResidentEntry, len(entries))
	for _, e := range entries {
		if id := resources.TargetID(e); id != "" {
			byTarget[id] = e
		}
	}
	// 共享裁决：按五档顺序取"够用的最小前缀"（不是一把全清）
	ordered := resources.EvictToFree(entries, needGB)
	var out []string
	for _, target := range ordered {
		e, ok := byTarget[target]
		if !ok {
			continue
		}
		// 动作准入再拦一道（红线①/②/③ + 加载中）：即便排进了前缀，不可动作也绝不动手
		if reason, allowed := resources.ActionBlockReason(e); !allowed {
			log.Printf("[backend] 腾退跳过 %s（%s）——红线：不驱逐此状态", target, reason)
			continue
		}
		out = append(out, target)
	}
	return out
}

// rankActionableLocked 返回按五档排序、且**允许动作**的驻留项（调用方需持锁）。
// 这是唯一的"谁能被驱逐"准入点：在飞（红线①）、未托管（红线②）、pin 未到期（红线③）、
// 加载中 一律在这里被拦住——上层（上限淘汰/内存腾退）拿到的列表里不可能有它们。
func (m *Manager) rankActionableLocked() []resources.RankedEviction {
	entries := m.residentEntriesLocked()
	byTarget := make(map[string]resources.ResidentEntry, len(entries))
	for _, e := range entries {
		if id := resources.TargetID(e); id != "" {
			byTarget[id] = e
		}
	}
	var out []resources.RankedEviction
	for _, e := range resources.RankEvictions(entries).Evictable {
		r, ok := byTarget[e.Alias]
		if !ok {
			continue
		}
		if reason, allowed := resources.ActionBlockReason(r); !allowed {
			log.Printf("[backend] 淘汰跳过 %s（%s）——红线：不驱逐此状态", e.Alias, reason)
			continue
		}
		out = append(out, e)
	}
	return out
}

// ── P1：服务类型互斥（llama xor ds4）────────────────────────────────────────
//
// 设计依据：docs/01-设计/设计-子端服务切换与基线服务声明.md §7 S1/S2。
// 单槽/多槽机上一次只允许「当前工作模型」所属的那类推理服务在跑：装 ds4 前必先卸 llama，
// 反之亦然。本机制只作用于**本端 spawn 的**驻留（红线②不变）；未托管的手工服务由
// baseline/借用机制处理（该设计 §9），此处绝不触碰。

const (
	// kindLlama = llama.cpp 系（llama-server 及其 fork）。
	kindLlama = "llama"
	// kindDS4 = DwarfStar 系（ds4-server）。
	kindDS4 = "ds4"
)

// serviceKind 判定模型所属推理服务类型（纯函数，便于测试）。
// 判据取「实际会执行的可执行文件」：有 cmd: 覆盖时取它的第一个词，否则按后端类型。
func serviceKind(entry *registry.ModelEntry) string {
	if entry == nil {
		return kindLlama
	}
	if cmd := strings.TrimSpace(string(entry.Cmd)); cmd != "" {
		if f := strings.Fields(cmd); len(f) > 0 {
			return kindFromExecutable(f[0])
		}
	}
	if entry.Backend == "ds4-server" {
		return kindDS4
	}
	return kindLlama
}

// kindFromExecutable 从可执行文件名/路径判定服务类型（不认识的一律按 llama）。
func kindFromExecutable(p string) string {
	b := strings.ToLower(strings.TrimSpace(p))
	if i := strings.LastIndexByte(b, '/'); i >= 0 {
		b = b[i+1:]
	}
	if strings.Contains(b, "ds4") {
		return kindDS4
	}
	return kindLlama
}

// evictOtherKindsLocked 卸下所有「服务类型与 targetKind 不同」的驻留模型（调用方需持锁）。
//
// 复用 rankActionableLocked 的红线过滤：可动作的即刻卸载；被红线挡住的（有在飞请求、
// pin 未到期、加载中）**不硬来**，而是计入 blocked 交给调用方拒装 —— 宁可拒装，
// 也不让两类服务同时在跑。返回的两个清单均已排序，便于测试与日志稳定。
func (m *Manager) evictOtherKindsLocked(exceptModel, targetKind string) (evicted, blocked []string) {
	actionable := make(map[string]bool)
	for _, e := range m.rankActionableLocked() {
		actionable[e.Alias] = true
	}
	for name, sp := range m.procs {
		if name == exceptModel || sp == nil || sp.entry == nil {
			continue
		}
		if serviceKind(sp.entry) == targetKind {
			continue
		}
		if actionable[name] {
			m.evictSubprocLocked(name, sp, "服务类型互斥（目标 "+targetKind+"）")
			evicted = append(evicted, name)
			continue
		}
		blocked = append(blocked, name)
	}
	sort.Strings(evicted)
	sort.Strings(blocked)
	return evicted, blocked
}

// evictSubprocLocked 停止并移除一个驻留项（调用方需持锁），返回它腾出的 GB。
// （M10 external 守卫已随 P4 退场清理删除——外部复用项不再存在，附录 C·C2。）
func (m *Manager) evictSubprocLocked(name string, sp *subproc, why string) float64 {
	gb := 0.0
	if sp != nil {
		gb = resources.OccupiedGb(residentEntryOf(name, sp, time.Now()))
	}
	log.Printf("[backend] %s: 卸载 %s (释放 ≈%.1fGB)", why, name, gb)
	if sp != nil {
		m.stopSubproc(sp)
	}
	delete(m.procs, name)
	return gb
}

// Unload 只卸载指定模型（"只卸够"的动作侧，供主控定向让位）。
// 空清单 = 卸全部（沿用既有 /unload 语义，运维显式动作那条路不变）。
//
// 安全阀（即便调用方点了名也不动）：
//   - 有在飞请求（reqCount>0）的驻留不卸——红线①；主控侧计划本就不会点名，这里防的是坏调用方。
//   - pin 且 TTL 未到期的驻留不卸——红线③。
//
// 未托管的进程根本不在这里（procs 只装我们自己 spawn 的进程）——红线②。
func (m *Manager) Unload(models []string) map[string]interface{} {
	if len(models) == 0 {
		return m.Stop()
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	stopped := []string{}
	skipped := []string{}
	reasons := map[string]string{}
	for _, name := range models {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		sp, ok := m.procs[name]
		if !ok {
			skipped = append(skipped, name)
			reasons[name] = "not_resident"
			continue
		}
		if reason, allowed := resources.ActionBlockReason(residentEntryOf(name, sp, time.Now())); !allowed {
			skipped = append(skipped, name)
			reasons[name] = reason
			continue
		}
		m.evictSubprocLocked(name, sp, "定向卸载")
		stopped = append(stopped, name)
	}
	sort.Strings(stopped)
	sort.Strings(skipped)
	log.Printf("[backend] 定向卸载: stopped=%v skipped=%v", stopped, skipped)
	return map[string]interface{}{
		"ok":      true,
		"stopped": stopped,
		"skipped": skipped,
		"reasons": reasons,
	}
}
