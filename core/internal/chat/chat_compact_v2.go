// chat_compact_v2.go — 丙批（2026-09-10 Mr2109拍板）：压缩单一入口 MaybeCompact
// 设计依据: docs/01-设计/设计-虫族记忆体系-20260910.md §4.2「压缩单一入口 + 冷却 + 召回指针」
//
//	① 单一入口: 阈值 max(min(ctx×0.5, 8000), 2000)（沿用 chat_compact.go 的 CompactTriggerTokens）
//	② 算法选路: LLMLingua-2（注入式 CompactFn——删除式、快）优先；
//	            失败/未加载 → LLM 结构化摘要（注入式 SummarizeFn——现有 6 段模板）
//	③ 会话级失败冷却: 60s → 300s → 900s（连续失败递增）——落 ~/.zerg/state/compact_cooldown.json
//	            冷却内不再重试；force（手动触发）或 provider 明确返回上下文超限可无视冷却一次
//	④ 3 次连续失败硬熔断: 停用该会话压缩（直到冷却到期 / 手动重置），写日志
//	⑤ 召回指针: 压缩时向 ~/.zerg/state/compact_journal.jsonl 追写 {session_id, from_id, to_id, summary_id, at}
//	            摘要消息文本尾部附 (可搜回:session_search(session_id=..., around_id=...))
//	⑥ 软归档语义保持: MarkCompacted(active=0, compacted=1) 不动
//	⑦ 失败冷却按错误类型分流（§9.4 R6）：4xx 不可重试→直接硬熔断；429→冷却不小于 Retry-After；
//	    5xx/网络/超时→正常递进 + full jitter。分类结果写入 LastError 同级的 Class 字段。
//	⑧ 硬熔断/冷却的手动重置入口（ResetCompactState）与只读状态快照（CompactStateSnapshot）。
//
// 说明: 本文件只做「编排」——真正的 LLMLingua/LLM 调用由调用方注入，便于选路与测试。
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// CompactFn — LLMLingua-2 类删除式压缩函数（可空——未加载/不可用时传 nil）
type CompactFn func(ctx context.Context, text string) (string, error)

// SummarizeFn — LLM 结构化摘要函数（现有 6 段模板；可空——空且无 LLMLingua 则本次压缩判失败）
type SummarizeFn func(ctx context.Context, msgs []map[string]any) (string, error)

// compactHardTripStreak — 连续失败达到该次数即硬熔断（对齐 Claude Code 的 3 次）
const compactHardTripStreak = 3

// compactRecallPointerFmt — 摘要尾部召回指针模板（与 §4.2 逐字一致）
// 语义: around_id 取被压缩段的 from_id——session_search 读窗口可搜回该段
const compactRecallPointerFmt = "\n\n(可搜回:session_search(session_id=%s, around_id=%d))"

// compactCooldownSteps — 失败冷却阶梯（60s → 300s → 900s；超出取末档）
var compactCooldownSteps = []time.Duration{60 * time.Second, 300 * time.Second, 900 * time.Second}

// compactNow — 冷却计时时钟（测试可替换——验证 60/300/900 递增与到期恢复）
var compactNow = time.Now

// compactStateMu — 冷却状态 + journal 落盘互斥（进程内单写者）
var compactStateMu sync.Mutex

// ── 失败错误类型分流（§9.4 R6）────────────────────────────────────────
//
// 分类标签（写入 compactSessionFail.Class）:
//
//	"4xx"     —— 客户端/参数类错误（400/401/403/404/422…，429 除外）：不可重试 → 直接硬熔断
//	"429"     —— 限流：冷却取 max(full jitter base, Retry-After)
//	"5xx"     —— 服务端错误 / 网络 / 超时：正常递进 + full jitter（现状）
//	"unknown" —— 无法判定（含零值）：按 5xx 处理（正常递进）
const (
	compactClass4xx     = "4xx"
	compactClass429     = "429"
	compactClass5xx     = "5xx"
	compactClassUnknown = "unknown"
)

// CompactErrClass — 失败元信息（调用方/执行函数提供；零值=未知→按 5xx 处理）
type CompactErrClass struct {
	HTTPStatus int           // 0=未知；否则为 HTTP 状态码（如 429/403/500）
	RetryAfter time.Duration // 429 时的服务端要求（0=无）
}

// kind — 分类标签（供日志与持久化）
func (c CompactErrClass) kind() string {
	switch {
	case c.HTTPStatus == 429 || (c.HTTPStatus == 0 && c.RetryAfter > 0):
		return compactClass429
	case c.transient4xx():
		// 408/425/499 是「暂时性」4xx（超时/过早/客户端断开），不是请求本身错——按 5xx 递进重试，
		// 否则一次超时就把压缩永久熔断（Mr2109补：分流要按「该不该再试」而不是按状态码段）。
		return compactClass5xx
	case c.HTTPStatus >= 400 && c.HTTPStatus < 500:
		return compactClass4xx
	case c.HTTPStatus >= 500 && c.HTTPStatus < 600:
		return compactClass5xx
	default:
		return compactClassUnknown
	}
}

// nonRetryable — 不可重试错误（4xx 且非 429/非暂时性）→ 直接硬熔断，不等 3 次
func (c CompactErrClass) nonRetryable() bool {
	return c.HTTPStatus >= 400 && c.HTTPStatus < 500 && c.HTTPStatus != 429 && !c.transient4xx()
}

// transient4xx — 暂时性 4xx（可重试）：408 请求超时 / 425 过早 / 499 客户端断开。
// 与「请求本身错」（400/401/403/404/422）相反：重试有意义，故走递进退避而非直接熔断。
func (c CompactErrClass) transient4xx() bool {
	switch c.HTTPStatus {
	case 408, 425, 499:
		return true
	}
	return false
}

// hasRetryFloor — 需要套用「冷却不小于 Retry-After」下界（429 或显式带 Retry-After）
func (c CompactErrClass) hasRetryFloor() bool {
	return c.HTTPStatus == 429 || c.RetryAfter > 0
}

var (
	// reCompactHTTPStatus — 显式状态码："HTTP 429" / "status: 500" / "status_code=422" / "code 404"
	reCompactHTTPStatus = regexp.MustCompile(`(?i)\b(?:http(?:\s*status)?|status(?:\s*_?code)?|code)\b\s*[:=]?\s*([1-5]\d{2})\b`)
	// reCompactBareStatus — 裸 4xx/5xx 三位码（如 "429 Too Many Requests"、"摘要模型 500"）
	reCompactBareStatus = regexp.MustCompile(`\b([45]\d{2})\b`)
	// reCompactRetryAfter — "Retry-After: 30" / "retry_after=12.5"
	reCompactRetryAfter = regexp.MustCompile(`(?i)retry[-_ ]?after\b\s*[:=]?\s*(\d+(?:\.\d+)?)`)
	// reCompactRateLimitWord — 无状态码但明确限流措辞 → 归为 429
	reCompactRateLimitWord = regexp.MustCompile(`(?i)too many requests|rate[-_ ]?limit`)
)

// DefaultErrClass — 默认错误分类器：从 error 文本里抽 "HTTP 4xx/5xx"、"status: NNN"、
// 裸状态码、"Retry-After" 秒数与限流措辞。无法判定时返回零值（→ 按 5xx 处理）。
// 调用方若已知真实 HTTP 状态/Retry-After，可用 WithErrClass / WithErrClassifier 覆盖。
func DefaultErrClass(err error) CompactErrClass {
	var cls CompactErrClass
	if err == nil {
		return cls
	}
	msg := err.Error()
	// 1) Retry-After 秒数（可带小数）
	if m := reCompactRetryAfter.FindStringSubmatch(msg); len(m) == 2 {
		if secs, e := strconv.ParseFloat(m[1], 64); e == nil && secs > 0 {
			cls.RetryAfter = time.Duration(secs * float64(time.Second))
		}
	}
	// 2) HTTP 状态码（先显式，后裸码）
	code := 0
	if m := reCompactHTTPStatus.FindStringSubmatch(msg); len(m) == 2 {
		code, _ = strconv.Atoi(m[1])
	}
	if code == 0 {
		if m := reCompactBareStatus.FindStringSubmatch(msg); len(m) == 2 {
			code, _ = strconv.Atoi(m[1])
		}
	}
	// 3) 无限流码但措辞明确 → 429
	if code == 0 && reCompactRateLimitWord.MatchString(msg) {
		code = 429
	}
	cls.HTTPStatus = code
	return cls
}

// compactClassifyFn — 错误 → 失败元信息（可注入；默认 DefaultErrClass）
type compactClassifyFn func(error) CompactErrClass

// compactOptions — MaybeCompact 的可选入参聚合
type compactOptions struct {
	classify compactClassifyFn
	trigger  string // T3.1：本次压缩的触发原因（闭集；空=未指定 ⇒ 按 force 推导）
}

// CompactOption — MaybeCompact 新增的「可选入参」：不改签名形态的扩展点（变参）。
type CompactOption func(*compactOptions)

// WithErrClass — 直接注入失败元信息（调用方已从 HTTP 响应得知状态码/Retry-After 时用）
func WithErrClass(cls CompactErrClass) CompactOption {
	return func(o *compactOptions) {
		c := cls
		o.classify = func(error) CompactErrClass { return c }
	}
}

// WithErrClassifier — 注入自定义错误分类器（默认 DefaultErrClass；传 nil 视为默认）
func WithErrClassifier(fn func(error) CompactErrClass) CompactOption {
	return func(o *compactOptions) {
		if fn != nil {
			o.classify = fn
		}
	}
}

// resolveCompactClassify — 取本次调用的分类器（默认 DefaultErrClass）
func resolveCompactClassify(opts []CompactOption) compactClassifyFn {
	o := compactOptions{classify: DefaultErrClass}
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	if o.classify == nil {
		return DefaultErrClass
	}
	return o.classify
}

// WithTriggerReason — T3.1：显式声明本次压缩的触发原因（闭集见 CompactTrigger* 常量）。
//
// 为什么要这个扩展点：`force=true` 一条路要区分两种**完全不同**的事 ——
//   - 人工/接口主动压（manual）；
//   - 上游明确报「上下文超限」后的补救性压缩（error_recovery）。
//
// 两者在观测面上必须分得开（前者是"我们主动"，后者是"被逼的"，H1 的 trigger_reason 闭集里
// 各占一格）。非法/未知取值**忽略**（不编造、不落到事件里），退回默认推导。
func WithTriggerReason(reason string) CompactOption {
	return func(o *compactOptions) {
		switch reason {
		case CompactTriggerTokenBudget, CompactTriggerManual, CompactTriggerErrorRecovery:
			o.trigger = reason
		}
	}
}

// compactTriggerReasonOf — 本次压缩的触发原因（默认推导，写死）：
// 显式声明优先；否则 force ⇒ manual（强制压缩），非 force ⇒ token_budget（阈值）。
func compactTriggerReasonOf(force bool, opts []CompactOption) string {
	o := compactOptions{}
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	if o.trigger != "" {
		return o.trigger
	}
	if force {
		return CompactTriggerManual
	}
	return CompactTriggerTokenBudget
}

// compactObsFactsOf — T3.1：组装三态事件的公共事实（在**确定要压**之后、摘要调用**之前**调用一次）。
//
// 口径（写死，读侧据此复算）:
//   - tokens_before：**全部**活动消息按 compactMessageTokens 加总（与触发阈值同一估算器）
//   - kept/summarized ids：被摘要段 = compactMiddleSegment 的 ids；其余 = 保留段 ——
//     两者**互补且不重叠**，并集 = 本次入参的全部消息（这是事件可校验的不变式，用例 ⑤ 钉住）
//   - turns_summarized：被摘要段里 **role=user** 的条数（一轮以用户消息起算；纯 assistant 段 ⇒ 0）
//   - summarizer_model：取 MaybeCompact 的 model 入参 —— live 路径的摘要器
//     （SummarizeWithInfer(h.infer, id, se.Model)）用的就是同一个值，故同源；拿不到就缺席（omitempty）
func compactObsFactsOf(sessionID, model, trigger string, msgs []*Message, summarized []Message, ids []int64) compactObsFacts {
	f := compactObsFacts{
		Session:        sessionID,
		Model:          model,
		Trigger:        trigger,
		MessagesBefore: len(msgs),
		SummarizedIDs:  append([]int64{}, ids...),
	}
	inSummary := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		inSummary[id] = struct{}{}
	}
	f.KeptIDs = make([]int64, 0, len(msgs)-len(ids))
	for _, m := range msgs {
		t := compactMessageTokens(*m)
		f.TokensBefore += t
		if _, compressed := inSummary[m.ID]; compressed {
			continue
		}
		f.KeptIDs = append(f.KeptIDs, m.ID)
		f.KeptTokens += t
	}
	for _, m := range summarized {
		if m.Role == "user" {
			f.TurnsSummarized++
		}
	}
	return f
}

// compactSessionFail — 单会话压缩失败态（持久化结构）
type compactSessionFail struct {
	Streak    int     `json:"streak"`               // 连续失败次数（成功即清零）
	Until     float64 `json:"until"`                // 冷却到期时间（unix 秒；0=不在冷却）
	Disabled  bool    `json:"disabled"`             // 硬熔断标记（连续 3 次失败，或 4xx 直接熔断）
	Class     string  `json:"class,omitempty"`      // 错误分类（4xx/429/5xx/unknown）
	LastError string  `json:"last_error,omitempty"` // 最近一次失败原因
	UpdatedAt float64 `json:"updated_at"`           // 最近更新（unix 秒）
}

// compactJournalRecord — 召回指针 journal 记录（§4.2 逐字字段）
type compactJournalRecord struct {
	SessionID string  `json:"session_id"`
	FromID    int64   `json:"from_id"`
	ToID      int64   `json:"to_id"`
	SummaryID int64   `json:"summary_id"`
	At        float64 `json:"at"`
}

// compactCooldownFile — 冷却状态文件路径（statepath 每次调用读 env——测试隔离友好）
func compactCooldownFile() string { return statepath.File("compact_cooldown.json") }

// compactJournalFile — 召回指针 journal 文件路径
func compactJournalFile() string { return statepath.File("compact_journal.jsonl") }

// compactJitterRand — 抖动随机源（[0,1)；测试可注入固定值以保证确定性）
var compactJitterRand = rand.Float64

// compactCooldownBase — 按连续失败次数取冷却 base（不含 jitter）：1→60s，2→300s，≥3→900s
func compactCooldownBase(streak int) time.Duration {
	if streak <= 0 {
		return 0
	}
	if streak <= len(compactCooldownSteps) {
		return compactCooldownSteps[streak-1]
	}
	return compactCooldownSteps[len(compactCooldownSteps)-1]
}

// compactMaxCooldownBase — 冷却最大档 base（900s）——4xx 不可重试错误直接取该档
func compactMaxCooldownBase() time.Duration {
	return compactCooldownSteps[len(compactCooldownSteps)-1]
}

// compactJitter — full jitter（§9.4 R6 / Mr2109 2026-09-10）：random(0, base)。
// 打散「多个会话同时失败→同时重试→再次失败」的同步风暴；随机下界可为 0（下次请求即可再试）。
func compactJitter(base time.Duration) time.Duration {
	f := compactJitterRand()
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return time.Duration(float64(base) * f)
}

// compactCooldownFor — 按连续失败次数取冷却时长（分级 base + full jitter）
func compactCooldownFor(streak int) time.Duration {
	return compactJitter(compactCooldownBase(streak))
}

// compactInCooldown — 是否仍在冷却窗口内（now 为 unix 秒）
func compactInCooldown(f *compactSessionFail, now float64) bool {
	return f != nil && f.Until > now
}

// loadCompactCooldownLocked — 读冷却状态（调用方须持 compactStateMu）
func loadCompactCooldownLocked() map[string]*compactSessionFail {
	sessions := map[string]*compactSessionFail{}
	b, err := os.ReadFile(compactCooldownFile())
	if err != nil {
		return sessions // 无文件/读失败 → 空态（不阻断压缩）
	}
	var wrap struct {
		Sessions map[string]*compactSessionFail `json:"sessions"`
	}
	if json.Unmarshal(b, &wrap) != nil || wrap.Sessions == nil {
		return sessions
	}
	return wrap.Sessions
}

// saveCompactCooldownLocked — 原子写冷却状态（临时文件 + rename；调用方须持 compactStateMu）
func saveCompactCooldownLocked(sessions map[string]*compactSessionFail) error {
	if err := os.MkdirAll(statepath.Dir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(struct {
		Sessions map[string]*compactSessionFail `json:"sessions"`
	}{Sessions: sessions}, "", "  ")
	if err != nil {
		return err
	}
	final := compactCooldownFile()
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// compactGetFail — 读某会话失败态（nil=无记录）
func compactGetFail(sessionID string) *compactSessionFail {
	compactStateMu.Lock()
	defer compactStateMu.Unlock()
	return loadCompactCooldownLocked()[sessionID]
}

// compactRecordFailure — 记一次压缩失败（连续次数 +1；按错误类型分流冷却 / 熔断）；返回**累计连续失败次数**。
//
// 分流规则（§9.4 R6）:
//   - cls.nonRetryable()（4xx 非 429）: 不可重试 → 直接硬熔断 Disabled，冷却取 full jitter 最大档 base（900s）；
//   - cls.hasRetryFloor()（429 或带 Retry-After）: 冷却 = max(full jitter base, Retry-After)；
//   - 其余（5xx/网络/超时/unknown）: 正常递进 + full jitter（现状不变）。
//
// 返回值（T3.1）：streak 供观测面写进 compaction_failed 的失败原文（与旧 OBS-4 行的 streak 同值）。
func compactRecordFailure(sessionID string, cause error, now float64, cls CompactErrClass) int {
	compactStateMu.Lock()
	defer compactStateMu.Unlock()
	sessions := loadCompactCooldownLocked()
	f := sessions[sessionID]
	if f == nil {
		f = &compactSessionFail{}
	}
	f.Streak++
	f.LastError = cause.Error()
	f.Class = cls.kind()
	f.UpdatedAt = now

	switch {
	case cls.nonRetryable():
		// 4xx（非 429）——不可重试：直接熔断，不必等 3 次；冷却取最大档 base
		d := compactJitter(compactMaxCooldownBase())
		f.Disabled = true
		f.Until = now + d.Seconds()
		log.Printf("⛔ session %s compaction failed: HTTP %d non-retryable error → circuit open (cooldown %s, streak %d) cause: %v",
			sessionID, cls.HTTPStatus, d, f.Streak, cause)
	default:
		d := compactCooldownFor(f.Streak) // 正常递进 + full jitter
		if cls.hasRetryFloor() && cls.RetryAfter > d {
			d = cls.RetryAfter // 429/Retry-After：冷却不小于服务端要求
		}
		f.Until = now + d.Seconds()
		switch {
		case f.Streak >= compactHardTripStreak:
			f.Disabled = true
			log.Printf("⛔ session %s compaction hard-breaker: %d consecutive failures — compaction disabled (cooldown %s, recovers on expiry or manual reset) cause: %v",
				sessionID, f.Streak, d, cause)
		case cls.kind() == compactClass429:
			log.Printf("⚠️ session %s compaction failure #%d (HTTP 429/Retry-After=%s) — entering cooldown %s: %v",
				sessionID, f.Streak, cls.RetryAfter, d, cause)
		default:
			log.Printf("⚠️ session %s compaction failure #%d — entering cooldown %s: %v", sessionID, f.Streak, d, cause)
		}
	}
	// OBS-4：压缩失败也落一行（失败不阻塞对话 ✓ 但必须可取证）
	obsCompact(sessionID, "threshold", 0, 0, 0, 0, "fail", fmt.Sprintf("%v (streak=%d)", cause, f.Streak), false)
	sessions[sessionID] = f
	if err := saveCompactCooldownLocked(sessions); err != nil {
		log.Printf("⚠️ failed to persist compaction cooldown state: %v", err)
	}
	return f.Streak
}

// compactClearFail — 压缩成功 → 清该会话失败态（连续计数归零）
func compactClearFail(sessionID string) {
	compactStateMu.Lock()
	defer compactStateMu.Unlock()
	sessions := loadCompactCooldownLocked()
	if _, ok := sessions[sessionID]; !ok {
		return
	}
	delete(sessions, sessionID)
	if err := saveCompactCooldownLocked(sessions); err != nil {
		log.Printf("⚠️ failed to persist compaction cooldown state: %v", err)
	}
}

// compactClearTripLocked — 冷却已到期 → 清硬熔断标记（保留 streak 以便继续递增）
func compactClearTripLocked(sessions map[string]*compactSessionFail, sessionID string) {
	if f := sessions[sessionID]; f != nil && f.Disabled {
		f.Disabled = false
	}
}

// compactAppendJournal — 追加一条召回指针记录（jsonl，O_APPEND）
func compactAppendJournal(rec compactJournalRecord) {
	compactStateMu.Lock()
	defer compactStateMu.Unlock()
	if err := os.MkdirAll(statepath.Dir(), 0o755); err != nil {
		log.Printf("⚠️ failed to create recall-pointer journal directory: %v", err)
		return
	}
	b, err := json.Marshal(rec)
	if err != nil {
		log.Printf("⚠️ failed to serialize recall-pointer journal: %v", err)
		return
	}
	f, err := os.OpenFile(compactJournalFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("⚠️ failed to open recall-pointer journal: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		log.Printf("⚠️ failed to write recall-pointer journal: %v", err)
	}
}

// compactMiddleSegment — 计算待压缩中间段（首 CompactProtectFirstN + 尾 CompactProtectLastN 保留）
func compactMiddleSegment(msgs []*Message) ([]Message, []int64) {
	n := len(msgs)
	start := CompactProtectFirstN
	end := n - CompactProtectLastN
	if start >= end {
		return nil, nil // 没得压
	}
	src := make([]Message, 0, end-start)
	ids := make([]int64, 0, end-start)
	for i := start; i < end; i++ {
		src = append(src, *msgs[i])
		ids = append(ids, msgs[i].ID)
	}
	return src, ids
}

// compactJoinText — 中间段拼成删除式压缩输入文本（带角色前缀——保留对话脉络）
func compactJoinText(src []Message) string {
	var b strings.Builder
	for _, m := range src {
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return b.String()
}

// compactSourceMaps — 中间段 → []map[string]any（role/content——供注入的 SummarizeFn 使用）
func compactSourceMaps(src []Message) []map[string]any {
	out := make([]map[string]any, 0, len(src))
	for _, m := range src {
		out = append(out, map[string]any{"role": m.Role, "content": m.Content})
	}
	return out
}

// MaybeCompact — 压缩单一入口（丙批 §4.2）
// 返回 did=true 表示本次确实执行了压缩（软归档 + 摘要 + journal 均已落盘）。
// 返回 err 仅表示「本次压缩尝试失败」（触发冷却），不代表调用方请求失败——调用方可忽略或记日志。
//
// 参数:
//   - model: 用于阈值计算（CompactTriggerTokens）
//   - msgs:  会话活动消息（时间正序）
//   - lingua: LLMLingua-2 删除式压缩（可空）
//   - sum:    LLM 结构化摘要（可空）
//   - force: 手动触发/上下文超限——无视冷却一次
//   - opts:  可选入参（变参，不改原签名形态）：如 WithErrClass / WithErrClassifier 注入失败元信息
func (s *ChatStore) MaybeCompact(ctx context.Context, sessionID, model string, msgs []*Message,
	lingua CompactFn, sum SummarizeFn, force bool, opts ...CompactOption) (bool, error) {

	// 会话级压缩锁（沿用旧路径——压缩中防并发双压）
	if _, loaded := s.compactLocks.LoadOrStore(sessionID, struct{}{}); loaded {
		return false, nil
	}
	defer s.compactLocks.Delete(sessionID)

	// 阈值判断（不足不压）——沿用 max(min(ctx×0.5, 8000), 2000)
	if !ShouldCompactPtr(msgs, model) {
		return false, nil
	}

	now := float64(compactNow().Unix())

	// 冷却 / 硬熔断闸门
	fail := compactGetFail(sessionID)
	if !force && compactInCooldown(fail, now) {
		if fail != nil && fail.Disabled {
			log.Printf("⛔ session %s compaction hard-breaker active (%d consecutive failures, cooled until %.0f) — skipping", sessionID, fail.Streak, fail.Until)
		}
		return false, nil
	}
	// 冷却已到期（或 force 旁路）→ 清硬熔断标记，给一次干净的重试机会（streak 保留以继续递增）
	if fail != nil && !compactInCooldown(fail, now) {
		compactStateMu.Lock()
		sessions := loadCompactCooldownLocked()
		compactClearTripLocked(sessions, sessionID)
		_ = saveCompactCooldownLocked(sessions)
		compactStateMu.Unlock()
	}

	src, ids := compactMiddleSegment(msgs)
	if len(src) == 0 || len(ids) == 0 {
		return false, nil
	}
	fromID, toID := ids[0], ids[len(ids)-1]

	// T3.1：**确定要压**之后才落 compaction_started —— 阈值不足/冷却内/无可压段三条早退路径
	// 都在上面返回了 ⇒ 一条 compaction 事件都不会落（这是本族事件的防误报底线，用例 ③ 钉住）。
	// 事实块在起点算（终态时库里已被改过：软归档 + 插入摘要 ⇒ 只有此刻的布局才是"压缩前"）。
	h := obsCompactionStart(compactObsFactsOf(sessionID, model, compactTriggerReasonOf(force, opts), msgs, src, ids))
	// T3.2：**即将被压掉的那段里的硬约束先入册** —— 否则"压缩吃掉一条 must_survive"这件事在
	// 首次提示检查之前发生（首轮即超阈值的会话）时，我们连它存在都不知道（关键用例的前提）。
	// 只登记、不改变压缩语义：不重注入、不改摘要、不阻断（观测面铁律①）。
	indexConstraints(sessionID, msgs)

	// 选路: LLMLingua-2 优先
	var summary string
	var callErr error // 注入函数（压缩/摘要）返回的真实错误——供错误类型分流
	if lingua != nil {
		out, lerr := lingua(ctx, compactJoinText(src))
		switch {
		case lerr != nil:
			callErr = lerr
			log.Printf("⚠️ session %s LLMLingua-2 compression failed, falling back to LLM summary: %v", sessionID, lerr)
		case strings.TrimSpace(out) == "":
			log.Printf("⚠️ session %s LLMLingua-2 returned empty — falling back to LLM summary", sessionID)
		default:
			summary = strings.TrimSpace(out)
		}
	}

	// 回退: LLM 结构化摘要
	llmSummary := false // T3.1：摘要是否来自**结构化摘要器**（决定 field_coverage 能不能判——删除式压缩不是结构化的）
	var err error
	if summary == "" {
		if sum == nil {
			err = fmt.Errorf("chat: compaction failed — LLMLingua-2 unavailable and no summarizer injected")
		} else {
			summary, err = sum(ctx, compactSourceMaps(src))
			if err != nil {
				callErr = err // 摘要错误为终判错误——覆盖 lingua 的错误
				summary = ""
			} else {
				summary = strings.TrimSpace(summary)
				llmSummary = summary != ""
			}
		}
	}

	if err != nil || summary == "" {
		if err == nil {
			err = fmt.Errorf("chat: compaction returned an empty summary")
		}
		// 错误类型分流：优先用注入函数返回的真实错误；否则用本地合成错误（未知→按 5xx 处理）
		classSrc := err
		if callErr != nil {
			classSrc = callErr
		}
		cls := resolveCompactClassify(opts)(classSrc)
		streak := compactRecordFailure(sessionID, err, now, cls)
		// T3.1：终态 = failed（与旧 OBS-4 行的 fail_reason 同文本，含 streak）——**不落 completed**
		h.failed(fmt.Sprintf("%v (streak=%d)", err, streak))
		return false, err
	}

	// 成功 → 清失败态 + 软归档 + 召回指针
	compactClearFail(sessionID)

	// Phase 1: 剪除中间段旧工具结果（沿用旧语义）
	if perr := s.PruneOldToolResults(sessionID, src); perr != nil {
		h.failed(perr.Error()) // 终态 = failed：摘要有了但布局没落地 ⇒ 不能报 completed（否则事件流说谎）
		return false, perr
	}
	// 软归档（active=0, compacted=1）——语义不动
	if merr := s.MarkCompacted(sessionID, ids); merr != nil {
		h.failed(merr.Error())
		return false, merr
	}
	// 摘要插在被压缩段首条之前 + 尾部召回指针
	pointer := fmt.Sprintf(compactRecallPointerFmt, sessionID, fromID)
	inserted := summary + pointer
	summaryID, ierr := s.InsertSummaryMid(sessionID, inserted, fromID)
	if ierr != nil {
		h.failed(ierr.Error())
		return false, ierr
	}
	compactAppendJournal(compactJournalRecord{
		SessionID: sessionID, FromID: fromID, ToID: toID, SummaryID: summaryID, At: now,
	})
	log.Printf("✅ session %s compaction done: messages %d..%d → summary #%d (%d chars, source %s)",
		sessionID, fromID, toID, summaryID, len([]rune(summary)), compactSummarySource(lingua))
	// OBS-4（v2.5.10 前置档②）：结构化一行，供长跑批量分析。
	obsCompact(sessionID, "threshold", 0, 0, len([]rune(summary)), 0, "ok", "", false) // 耗时本现场不可得 ⇒ 0（omitempty=未知）
	// T3.1：终态 = completed —— 传**实际插入库里的那段文本**（正文 + 召回指针），
	// 因此 summary_sha256 对的就是库里那条摘要消息（去掉「【历史摘要】」前缀后逐字可复算）。
	h.completed(summary, inserted, llmSummary)
	return true, nil
}

// compactSummarySource — 标注摘要来源（日志用）
func compactSummarySource(lingua CompactFn) string {
	if lingua != nil {
		return "LLMLingua-2/LLM"
	}
	return "LLM"
}

// ResetCompactState — 清该会话压缩冷却与硬熔断（供 UI「重置压缩熔断」按钮/运维）。
//
// 语义：删除该会话的失败态（内存态 + 落盘 compact_cooldown.json），删净后该会话可立即重试。
// 幂等：无记录时直接返回 nil。返回错误仅当落盘失败。
func ResetCompactState(sessionID string) error {
	compactStateMu.Lock()
	defer compactStateMu.Unlock()
	sessions := loadCompactCooldownLocked()
	if _, ok := sessions[sessionID]; !ok {
		return nil // 无失败态即无需重置
	}
	delete(sessions, sessionID)
	if err := saveCompactCooldownLocked(sessions); err != nil {
		return err
	}
	log.Printf("♻️ session %s compaction cooldown/hard-breaker manually reset", sessionID)
	return nil
}

// ResetCompactSession — 兼容旧入口：等价 ResetCompactState（忽略落盘错误，仅记日志）。
//
// Deprecated: 新代码请用 ResetCompactState（可拿到落盘错误）。
func ResetCompactSession(sessionID string) {
	if err := ResetCompactState(sessionID); err != nil {
		log.Printf("⚠️ failed to persist compaction cooldown state: %v", err)
	}
}

// CompactStateSnapshot — 会话压缩状态只读查询（给 UI/运维看：冷却中？熔断？剩多少秒？）。
//
// 返回:
//   - inCooldown: 是否仍在冷却窗口内（Until > now）；
//   - disabled:   当前「生效中」的硬熔断（持久化的 Disabled 且仍在冷却窗口内）——
//     冷却到期后本入口报 false，因 MaybeCompact 会在下次尝试时自动清除熔断标记；
//   - untilUnix:  冷却到期 unix 秒（无记录=0）；
//   - streak:     连续失败次数（无记录=0）。
func CompactStateSnapshot(sessionID string) (inCooldown bool, disabled bool, untilUnix float64, streak int) {
	f := compactGetFail(sessionID)
	if f == nil {
		return false, false, 0, 0
	}
	inCooldown = compactInCooldown(f, float64(compactNow().Unix()))
	return inCooldown, f.Disabled && inCooldown, f.Until, f.Streak
}

// IsContextLimitError — provider 明确返回「上下文超限」判定（调用方可据此传 force=true 无视冷却一次）
func IsContextLimitError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, kw := range []string{
		"context length", "context_length_exceeded", "maximum context", "context window",
		"too many tokens", "reduce the length", "exceed", "上下文超限", "超过上下文", "上下文长度",
	} {
		if strings.Contains(msg, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
