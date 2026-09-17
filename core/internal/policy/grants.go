// grants.go — T5.3「授权粒度」：once / session / project（+ TTL）+ 可枚举 + 可吊销 + 可序列化重启继承。
//
// 要治的缺口（设计稿 docs/01-设计/设计-内建调试版-v1.2-20260917.md §〇 F3）：
// **缺授权粒度/记忆** ⇒ 用户被反复问同一件事，最后被迫全开。成熟系统的形态是三档：
//
//	once    —— **本次**：就这一次调用，用完即作废（绝不复用）
//	session —— **本会话**：本次会话内任意次有效；换一个会话 ⇒ 不认（授权不跨会话漂移）
//	project —— **本项目/仓库**：本项目内有效，**可选 TTL**（到期即不认）
//
// 并且三条硬要求（少了任何一条，用户就会被迫全开）：
//
//	① **可枚举**：用户/审计必须能问「现在到底放了哪些权限」（List）
//	② **可吊销**：每一条都有稳定 id，随时能撤（Revoke，**立刻生效**）
//	③ **带失效时间**：过期即不认（用**注入的 now** 判定，不用 time.Now —— 否则没法测、也没法回放）
//
// ── 三条写死的语义（每条都有用例钉住）──
//
//	· **到期即不认**：判据是 `now.Before(ExpiresAt)` ⇒ **now 恰等于到期时刻也算失效**
//	  （边界取"不认"，与 fail-closed 同向）。
//	· **归属不符即不认**：session 授权只在**签它的那个会话**有效，project 授权只在**签它的那个项目**有效；
//	  没有归属（空会话/空项目）时该授权**不生效**（宁可重问一次，不可默默放行）。
//	· **once 用一次就作废**：`Consume` 判定通过的同时把它标成已用 ⇒ 第二次必不认。
//	  只有 `IsGranted` 是**只读**查询；"要用掉一次"必须走 `Consume`（判定与记账是同一个动作，
//	  否则接线批会出现 TOCTOU：查完再执行中间被吊销）。
//
// ── 匹配口径：**逐字、不放宽**（这是本文件最重要的取舍）──
//
//	(grant.Tool, grant.Specifier) 与请求 (tool, specifier) **逐字相等**才算命中（工具名归一化后比，
//	Specifier 去首尾空白后比）；`Specifier == ""` 表示"该工具的**任何**调用"（即用户在问人弹窗里选了
//	"这个工具以后都别问了"）。
//
//	故意**不做**参数级宽匹配（不用 match.go 的前缀/路径/域名形态去覆盖"形状相似"的调用）：
//	授权是"用户逐条批准"的**事实记录**，把 `bash(cat a.txt)` 的批准扩成 `bash(cat *)`，
//	等于替用户签字 —— 而用户以为自己只批了那一条。宽匹配的位置在**规则**（T5.1），
//	不在授权记账。宁可多问一次，不可默默扩大授权面。
//
// ── 本批边界（与 T5.1/T5.2 同一纪律）──
//
//	· 叶子包：只 import 标准库（time / encoding/json / sync / fmt / strings / sort），零内部依赖。
//	· **不接任何执行路径**：不碰 toolobs / chat / gateway / agent（一行不动）。
//	  本包只提供"记账 + 查询"的本体；"ask 的判定结果被授权改写成 allow"属于接线批，
//	  且那条改写**在地板命中时永远不生效**（地板压过档位与授权，T5.1 语义）。
//	· 与姿态（T5.4，posture.go）**不互相改写**：dontAsk 拒的是"会弹窗"，
//	  不是"已经批过的" —— 已有授权的前提下根本不会弹窗，两件事实互不冲突。
package policy

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// GrantStateVersion — 序列化格式版本。读到的版本不认识 ⇒ **报错**（fail-closed：绝不按"空账本"载入，
// 那等于把已吊销/已过期的授权一次静默恢复或一次静默丢弃）。
const GrantStateVersion = 1

// ── 作用域上下文与授权粒度 ───────────────────────────────────────────────────

// GrantContext — 记账的归属上下文：这批授权属于**哪个会话**、**哪个项目/仓库**。
// 重启后只要上下文相同（同一会话 id / 同一项目 id），session/project 授权就能**继承**。
type GrantContext struct {
	SessionID string `json:"session_id,omitempty"` // 会话标识（空 = 没有会话可归属）
	ProjectID string `json:"project_id,omitempty"` // 项目/仓库标识（建议用仓根路径或 project id）
}

// GrantScope — 授权粒度（三档，无第四档）。
type GrantScope string

const (
	// ScopeOnce — 本次：用完即废（绝不复用）。
	ScopeOnce GrantScope = "once"
	// ScopeSession — 本会话：本会话内有效，跨会话不认。
	ScopeSession GrantScope = "session"
	// ScopeProject — 本项目/仓库：项目内有效，可带 TTL（到期即不认）。
	ScopeProject GrantScope = "project"
)

// Valid — 是否三档之一（取值非法一律不认，见 Issue/validAt）。
func (s GrantScope) Valid() bool {
	switch s {
	case ScopeOnce, ScopeSession, ScopeProject:
		return true
	}
	return false
}

// ParseGrantScope — 解析粒度文本（大小写不敏感）；认不出一律**报错**（不猜粒度）。
func ParseGrantScope(raw string) (GrantScope, error) {
	s := GrantScope(strings.ToLower(strings.TrimSpace(raw)))
	if !s.Valid() {
		return "", fmt.Errorf("未知授权粒度「%s」（只认 once|session|project）", strings.TrimSpace(raw))
	}
	return s, nil
}

// ── 授权记录 ────────────────────────────────────────────────────────────────

// Grant — 一条授权记录。字段全部导出：用户界面要能如实回显"谁批的、批了什么、还有多久作废"。
type Grant struct {
	// ID — 稳定标识（吊销、审计、用例都按它点名）。由记账侧统一编号（见 MemoryGrantStore.nextID），
	// **不是随机数也不是时间戳**：同样一串操作必然得到同样的 id（可测、可回放）。
	ID        string     `json:"id"`
	Tool      string     `json:"tool"`                // 归一化后的工具名（小写、去首尾空白）
	Specifier string     `json:"specifier,omitempty"` // 匹配式；"" = 该工具的任何调用
	Scope     GrantScope `json:"scope"`               // once | session | project
	ExpiresAt time.Time  `json:"expires_at"`          // 失效时间；**零值 = 无 TTL**（只对 session/project 有意义）
	IssuedBy  string     `json:"issued_by"`           // 谁批的（用户标识 / "user" / "ci-bootstrap"…）：空 ⇒ 不成账
	RuleID    string     `json:"rule_id,omitempty"`   // 对应规则 id（内置条目 `any.*` / `never.*`；文本规则为空）

	// 归属（由 Issue 按上下文填；跨会话/跨项目一律不认）
	SessionID string `json:"session_id,omitempty"` // scope=session 时的归属会话
	ProjectID string `json:"project_id,omitempty"` // scope=project 时的归属项目

	IssuedAt time.Time `json:"issued_at"`      // 何时批的（取证/审计）
	Used     bool      `json:"used,omitempty"` // scope=once：是否已用掉
	Note     string    `json:"note,omitempty"` // 人类可读补充（仅取证；程序判据只看上面的字段）
	Seq      int       `json:"seq,omitempty"`  // 记账序号（1 起，单调；取证排序用，不参与判定）
}

// validAt — 这条授权在 now / 上下文下是否有效。**唯一的有效性判据**（查询、消费、载入、清理都走它）。
func (g Grant) validAt(now time.Time, ctx GrantContext) bool {
	if !g.Scope.Valid() {
		return false
	}
	// 到期即不认（含"恰等于到期时刻"）。
	if !g.ExpiresAt.IsZero() && !now.Before(g.ExpiresAt) {
		return false
	}
	switch g.Scope {
	case ScopeOnce:
		return !g.Used
	case ScopeSession:
		return g.SessionID != "" && ctx.SessionID != "" && g.SessionID == ctx.SessionID
	case ScopeProject:
		return g.ProjectID != "" && ctx.ProjectID != "" && g.ProjectID == ctx.ProjectID
	}
	return false
}

// matches — (tool, specifier) 是否被这条授权覆盖（逐字、不放宽；见文件头"匹配口径"）。
func (g Grant) matches(tool, specifier string) bool {
	if g.Tool != NormalizeToolName(tool) {
		return false
	}
	if g.Specifier == "" {
		return true // 无匹配式 = 该工具的任何调用
	}
	return strings.TrimSpace(g.Specifier) == strings.TrimSpace(specifier)
}

// expiredAt — 是否已到期（零值 = 无 TTL ⇒ 永不到期）。
func (g Grant) expiredAt(now time.Time) bool {
	return !g.ExpiresAt.IsZero() && !now.Before(g.ExpiresAt)
}

// ── 账本状态（序列化形态）────────────────────────────────────────────────────

// GrantState — 账本的**全部事实**（含已过期/已用掉的条目：它们是发生过的事实，不该在序列化时被抹掉）。
// 载入时按当前 now / 上下文过滤（见 LoadGrants），过滤掉多少会写进 GrantLoadReport。
type GrantState struct {
	Version int          `json:"version"`  // 必须 == GrantStateVersion
	Ctx     GrantContext `json:"context"`  // 序列化时的归属上下文（载入方**以自己传入的 ctx 为准**）
	NextSeq int          `json:"next_seq"` // 下一个编号（必须 > 0，且 > 所有条目的 Seq —— 否则报错）
	Grants  []Grant      `json:"grants"`
}

// GrantLoadReport — 载入时的过滤记账（"丢了几条、为什么丢"必须能回答，否则就是静默失效）。
type GrantLoadReport struct {
	Total          int // 序列化里有几条
	Kept           int // 恢复了几条
	DroppedExpired int // 载入时点已到期（不恢复）
	DroppedScope   int // 归属别的会话/项目，或没有归属
	DroppedUsed    int // scope=once 且已用掉
}

// String — 一行摘要（进日志/用例断言都读得懂）。
func (r GrantLoadReport) String() string {
	return fmt.Sprintf("载入授权账本：共 %d 条 ⇒ 恢复 %d 条（丢弃：过期 %d / 归属不符 %d / 已用掉 %d）",
		r.Total, r.Kept, r.DroppedExpired, r.DroppedScope, r.DroppedUsed)
}

// ── 账本接口 ────────────────────────────────────────────────────────────────

// GrantStore — 授权账本。**只做记账与查询**：不执行、不弹窗、不认识工具本体。
//
// 接口刻意保持最小（接线批要能换成落盘/落库实现，而不改调用方）：
//
//	Issue     —— 记一条授权（校验不过 ⇒ 报错，不入账）
//	IsGranted —— 只读查询：现在这条授权在不在（**不消费**）
//	Lookup    —— 只读查询并返回命中的那条记录（UI 要能回显"是哪一条在放行"）
//	Consume   —— 判定并**用掉**（once 走这里；返回命中的那条记录）
//	Revoke    —— 按 id 吊销（立刻生效）
//	List      —— 枚举全部记录（含已过期/已用掉 —— 事实可查）
//	Prune     —— 清掉已过期/已用掉/归属不符的条目，返回清掉几条
//	Snapshot  —— 导出可序列化状态（重启继承用）
type GrantStore interface {
	Issue(g Grant, now time.Time) (Grant, error)
	IsGranted(tool, specifier string, now time.Time) bool
	Lookup(tool, specifier string, now time.Time) (Grant, bool)
	Consume(tool, specifier string, now time.Time) (Grant, bool)
	Revoke(id string) bool
	List() []Grant
	Prune(now time.Time) int
	Snapshot() GrantState
}

// 编译期保证内存实现满足接口（换实现时这一行最先报错）。
var _ GrantStore = (*MemoryGrantStore)(nil)

// ── 内存实现 ────────────────────────────────────────────────────────────────

// MemoryGrantStore — 内存账本（本批唯一实现；落盘/落库实现另批，接口已经留好）。
// 并发安全（RWMutex）；**判定是确定性的**：不看系统时间，now 一律由调用方注入。
type MemoryGrantStore struct {
	mu      sync.RWMutex
	ctx     GrantContext
	nextSeq int
	grants  []Grant
}

// NewMemoryGrantStore — 建账本并绑定归属上下文（会话/项目）。空上下文允许（只对 once 授权有意义）。
func NewMemoryGrantStore(ctx GrantContext) *MemoryGrantStore {
	return &MemoryGrantStore{ctx: ctx, nextSeq: 1}
}

// Context — 回显绑定上下文（只读；重启继承的判据就是它）。
func (s *MemoryGrantStore) Context() GrantContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ctx
}

// Issue — 记一条授权。**校验不过一律报错、不入账**（fail-closed）：
//
//	· 工具名为空 / 粒度非法 —— 判不了，不记账
//	· IssuedBy 为空 —— 授权必须能回答"谁批的"（否则审计层拿到一条无名授权）
//	· scope=session/project 而上下文里没有对应归属 —— 记了也永不生效 ⇒ 直接报错（不许静默失效）
//	· ExpiresAt 已到期 —— 过期的东西不入账
//
// 返回**落库后的那条**（ID / 归属 / 序号已填），调用方必须用返回值（不要用自己拼的那份）。
func (s *MemoryGrantStore) Issue(g Grant, now time.Time) (Grant, error) {
	tool := NormalizeToolName(g.Tool)
	if tool == "" {
		return Grant{}, fmt.Errorf("授权记账：工具名为空 ⇒ 判不了（不记账）")
	}
	if !g.Scope.Valid() {
		return Grant{}, fmt.Errorf("授权记账：粒度「%s」非法（只认 once|session|project）", string(g.Scope))
	}
	if strings.TrimSpace(g.IssuedBy) == "" {
		return Grant{}, fmt.Errorf("授权记账：issued_by 为空 ⇒ 授权必须能回答「谁批的」（不记账）")
	}
	if g.expiredAt(now) {
		return Grant{}, fmt.Errorf("授权记账：这条授权在记账时点（%s）就已到期 ⇒ 不入账", now.UTC().Format(time.RFC3339))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 归属：按上下文填死（调用方传什么都不影响 —— 归属只能来自账本绑定的上下文）。
	g.SessionID, g.ProjectID = "", ""
	switch g.Scope {
	case ScopeSession:
		if s.ctx.SessionID == "" {
			return Grant{}, fmt.Errorf("授权记账：粒度=session 但当前上下文没有会话归属 ⇒ 记了也永不生效（不记账）")
		}
		g.SessionID = s.ctx.SessionID
	case ScopeProject:
		if s.ctx.ProjectID == "" {
			return Grant{}, fmt.Errorf("授权记账：粒度=project 但当前上下文没有项目归属 ⇒ 记了也永不生效（不记账）")
		}
		g.ProjectID = s.ctx.ProjectID
	}

	g.Tool = tool
	g.Specifier = strings.TrimSpace(g.Specifier)
	g.Seq = s.nextSeq
	g.ID = grantID(g, s.nextSeq)
	s.nextSeq++
	s.grants = append(s.grants, g)
	return g, nil
}

// grantID — 稳定 id：粒度 + 工具名 + 序号（同样一串操作必然得到同样的 id；不含随机数与时间）。
func grantID(g Grant, seq int) string {
	name := g.Tool
	if g.Specifier != "" {
		name += "(" + g.Specifier + ")"
	}
	return fmt.Sprintf("%s:%s#%d", g.Scope, name, seq)
}

// IsGranted — 只读查询：现在有没有一条有效授权覆盖 (tool, specifier)。**不消费**（once 不走这里）。
func (s *MemoryGrantStore) IsGranted(tool, specifier string, now time.Time) bool {
	_, ok := s.Lookup(tool, specifier, now)
	return ok
}

// Lookup — 只读查询，返回命中的那条记录（同工具同匹配式多条有效时取**先入账**的那条，保序确定）。
func (s *MemoryGrantStore) Lookup(tool, specifier string, now time.Time) (Grant, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, g := range s.grants {
		if g.matches(tool, specifier) && g.validAt(now, s.ctx) {
			return g, true
		}
	}
	return Grant{}, false
}

// Consume — 判定并**用掉一次**：命中且粒度是 once ⇒ 立刻标成已用（第二次必不认）。
// session/project 授权只查询、不改写（它们不是"一次性"的）。
// 返回命中的那条记录 —— 接线批据此回显"这次是拿哪一条授权放行的"。
func (s *MemoryGrantStore) Consume(tool, specifier string, now time.Time) (Grant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.grants {
		g := &s.grants[i]
		if !g.matches(tool, specifier) || !g.validAt(now, s.ctx) {
			continue
		}
		if g.Scope == ScopeOnce {
			g.Used = true // 判定与记账在**同一把锁**里完成（不做"查完再执行"的两段式，避免 TOCTOU）
		}
		return *g, true
	}
	return Grant{}, false
}

// Revoke — 按 id 吊销，**立刻生效**（不需要等 TTL、不需要重建账本）。返回是否真的吊销到了一条。
// 吊销是"删记录"而不是"打标记"：可枚举面里就该看不见它了（否则用户会怀疑"撤了还在放行"）。
func (s *MemoryGrantStore) Revoke(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.grants {
		if s.grants[i].ID == id {
			s.grants = append(s.grants[:i], s.grants[i+1:]...)
			return true
		}
	}
	return false
}

// List — 枚举**全部**记录（含已过期/已用掉：它们是事实，用户与审计都要看得见）。
// 返回副本（调用方改返回值不污染账本）；按入账顺序（即 Seq 序）。
func (s *MemoryGrantStore) List() []Grant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Grant, len(s.grants))
	copy(out, s.grants)
	return out
}

// Prune — 清掉**已失效**的条目（已过期 / 已用掉的 once / 归属不符），返回清掉几条。
// 注意：它**不影响**任何有效授权的判定结果 —— 纯粹是"把失效事实从可枚举面收起"。
func (s *MemoryGrantStore) Prune(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.grants[:0:0]
	dropped := 0
	for _, g := range s.grants {
		if g.expiredAt(now) || (g.Scope == ScopeOnce && g.Used) {
			dropped++
			continue
		}
		kept = append(kept, g)
	}
	if dropped > 0 {
		s.grants = kept
	}
	return dropped
}

// Snapshot — 导出可序列化状态（含全部事实 + 编号水位；重启继承靠它）。
func (s *MemoryGrantStore) Snapshot() GrantState {
	list := s.List()
	sort.SliceStable(list, func(i, j int) bool { return list[i].Seq < list[j].Seq })
	s.mu.RLock()
	next := s.nextSeq
	ctx := s.ctx
	s.mu.RUnlock()
	return GrantState{Version: GrantStateVersion, Ctx: ctx, NextSeq: next, Grants: list}
}

// MarshalJSON — 账本 ⇒ JSON（重启继承的写入侧）。经 Snapshot，字段固定（Version/Ctx/NextSeq/Grants）。
func (s *MemoryGrantStore) MarshalJSON() ([]byte, error) {
	st := s.Snapshot()
	return json.Marshal(st)
}

// LoadGrants — JSON ⇒ 账本（重启继承的读取侧）。四条 fail-closed 语义：
//
//	① 版本不认识 / JSON 坏 / 编号水位与条目不一致 ⇒ **报错**（绝不"当空账本接着跑"）
//	② 到期即不认：载入时点已过期的条目**不恢复**（并计数）
//	③ 归属不符即不认：序列化里的 session/project 授权，只在本方传入的 ctx **相同**时恢复；
//	   归属别的会话/项目的、以及没有归属的，一律不恢复（并计数）
//	④ once 已用掉的不恢复（并计数）
//
// 返回的账本归属**以传入的 ctx 为准**（重启用的是"当前会话/当前项目"，不是文件里写的那份）。
func LoadGrants(data []byte, ctx GrantContext, now time.Time) (*MemoryGrantStore, GrantLoadReport, error) {
	var st GrantState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, GrantLoadReport{}, fmt.Errorf("授权账本载入失败（JSON 坏）⇒ 拒绝启用（不回退成「空账本」）：%w", err)
	}
	return loadGrantState(st, ctx, now)
}

// loadGrantState — LoadGrants 的本体（拆出来便于用例直接喂状态，不经 JSON）。
func loadGrantState(st GrantState, ctx GrantContext, now time.Time) (*MemoryGrantStore, GrantLoadReport, error) {
	if st.Version != GrantStateVersion {
		return nil, GrantLoadReport{}, fmt.Errorf("授权账本版本 %d 不认识（本实现只认 %d）⇒ 拒绝启用（不猜格式）",
			st.Version, GrantStateVersion)
	}
	// 编号水位必须自洽：否则重启后会发出与旧条目撞车的 id（吊销可能吊销错一条）。
	maxSeq := 0
	for _, g := range st.Grants {
		if g.Seq > maxSeq {
			maxSeq = g.Seq
		}
	}
	if st.NextSeq <= maxSeq {
		return nil, GrantLoadReport{}, fmt.Errorf("授权账本编号水位不自洽（next_seq=%d ≤ 最大序号 %d）⇒ 拒绝启用（否则新授权会与旧条目撞 id）",
			st.NextSeq, maxSeq)
	}

	store := &MemoryGrantStore{ctx: ctx, nextSeq: st.NextSeq}
	rep := GrantLoadReport{Total: len(st.Grants)}
	for _, g := range st.Grants {
		switch {
		case g.expiredAt(now):
			rep.DroppedExpired++
			continue
		case g.Scope == ScopeOnce && g.Used:
			rep.DroppedUsed++
			continue
		case !g.scopeMatches(ctx):
			rep.DroppedScope++
			continue
		}
		store.grants = append(store.grants, g)
		rep.Kept++
	}
	return store, rep, nil
}

// scopeMatches — 序列化条目的归属是否与本方上下文一致（once 无归属，恒成立）。
func (g Grant) scopeMatches(ctx GrantContext) bool {
	switch g.Scope {
	case ScopeOnce:
		return g.SessionID == "" && g.ProjectID == ""
	case ScopeSession:
		return g.SessionID != "" && ctx.SessionID != "" && g.SessionID == ctx.SessionID
	case ScopeProject:
		return g.ProjectID != "" && ctx.ProjectID != "" && g.ProjectID == ctx.ProjectID
	}
	return false
}
