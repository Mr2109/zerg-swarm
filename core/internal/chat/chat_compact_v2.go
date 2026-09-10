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
//
// 说明: 本文件只做「编排」——真正的 LLMLingua/LLM 调用由调用方注入，便于选路与测试。
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"zerg/core/internal/statepath"
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

// compactSessionFail — 单会话压缩失败态（持久化结构）
type compactSessionFail struct {
	Streak    int     `json:"streak"`               // 连续失败次数（成功即清零）
	Until     float64 `json:"until"`                // 冷却到期时间（unix 秒；0=不在冷却）
	Disabled  bool    `json:"disabled"`             // 硬熔断标记（连续 3 次失败）
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

// compactCooldownFor — 按连续失败次数取冷却时长（1→60s，2→300s，≥3→900s）
func compactCooldownFor(streak int) time.Duration {
	if streak <= 0 {
		return 0
	}
	if streak > len(compactCooldownSteps) {
		return compactCooldownSteps[len(compactCooldownSteps)-1]
	}
	return compactCooldownSteps[streak-1]
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

// compactRecordFailure — 记一次压缩失败（连续次数 +1、进入对应冷却、达 3 次硬熔断并写日志）
func compactRecordFailure(sessionID string, cause error, now float64) {
	compactStateMu.Lock()
	defer compactStateMu.Unlock()
	sessions := loadCompactCooldownLocked()
	f := sessions[sessionID]
	if f == nil {
		f = &compactSessionFail{}
	}
	f.Streak++
	d := compactCooldownFor(f.Streak)
	f.Until = now + d.Seconds()
	f.LastError = cause.Error()
	f.UpdatedAt = now
	if f.Streak >= compactHardTripStreak {
		f.Disabled = true
		log.Printf("⛔ 会话 %s 压缩硬熔断：连续 %d 次失败——停用压缩（冷却 %s，到期或手动重置恢复）原因: %v",
			sessionID, f.Streak, d, cause)
	} else {
		log.Printf("⚠️ 会话 %s 压缩失败第 %d 次——进入冷却 %s: %v", sessionID, f.Streak, d, cause)
	}
	sessions[sessionID] = f
	if err := saveCompactCooldownLocked(sessions); err != nil {
		log.Printf("⚠️ 压缩冷却状态落盘失败: %v", err)
	}
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
		log.Printf("⚠️ 压缩冷却状态落盘失败: %v", err)
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
		log.Printf("⚠️ 召回指针 journal 目录创建失败: %v", err)
		return
	}
	b, err := json.Marshal(rec)
	if err != nil {
		log.Printf("⚠️ 召回指针 journal 序列化失败: %v", err)
		return
	}
	f, err := os.OpenFile(compactJournalFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("⚠️ 召回指针 journal 打开失败: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		log.Printf("⚠️ 召回指针 journal 写入失败: %v", err)
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
func (s *ChatStore) MaybeCompact(ctx context.Context, sessionID, model string, msgs []*Message,
	lingua CompactFn, sum SummarizeFn, force bool) (bool, error) {

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
			log.Printf("⛔ 会话 %s 压缩硬熔断生效中（连续 %d 次失败，冷却至 %.0f）——跳过", sessionID, fail.Streak, fail.Until)
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

	// 选路: LLMLingua-2 优先
	var summary string
	if lingua != nil {
		out, lerr := lingua(ctx, compactJoinText(src))
		switch {
		case lerr != nil:
			log.Printf("⚠️ 会话 %s LLMLingua-2 压缩失败，回退 LLM 摘要: %v", sessionID, lerr)
		case strings.TrimSpace(out) == "":
			log.Printf("⚠️ 会话 %s LLMLingua-2 返回空——回退 LLM 摘要", sessionID)
		default:
			summary = strings.TrimSpace(out)
		}
	}

	// 回退: LLM 结构化摘要
	var err error
	if summary == "" {
		if sum == nil {
			err = fmt.Errorf("chat: 压缩失败——LLMLingua-2 不可用且未注入摘要函数")
		} else {
			summary, err = sum(ctx, compactSourceMaps(src))
			if err != nil {
				summary = ""
			} else {
				summary = strings.TrimSpace(summary)
			}
		}
	}

	if err != nil || summary == "" {
		if err == nil {
			err = fmt.Errorf("chat: 压缩返回空摘要")
		}
		compactRecordFailure(sessionID, err, now)
		return false, err
	}

	// 成功 → 清失败态 + 软归档 + 召回指针
	compactClearFail(sessionID)

	// Phase 1: 剪除中间段旧工具结果（沿用旧语义）
	if perr := s.PruneOldToolResults(sessionID, src); perr != nil {
		return false, perr
	}
	// 软归档（active=0, compacted=1）——语义不动
	if merr := s.MarkCompacted(sessionID, ids); merr != nil {
		return false, merr
	}
	// 摘要插在被压缩段首条之前 + 尾部召回指针
	pointer := fmt.Sprintf(compactRecallPointerFmt, sessionID, fromID)
	summaryID, ierr := s.InsertSummaryMid(sessionID, summary+pointer, fromID)
	if ierr != nil {
		return false, ierr
	}
	compactAppendJournal(compactJournalRecord{
		SessionID: sessionID, FromID: fromID, ToID: toID, SummaryID: summaryID, At: now,
	})
	log.Printf("✅ 会话 %s 压缩完成：消息 %d..%d → 摘要 #%d（%d 字，来源 %s）",
		sessionID, fromID, toID, summaryID, len([]rune(summary)), compactSummarySource(lingua))
	return true, nil
}

// compactSummarySource — 标注摘要来源（日志用）
func compactSummarySource(lingua CompactFn) string {
	if lingua != nil {
		return "LLMLingua-2/LLM"
	}
	return "LLM"
}

// ResetCompactSession — 手动重置某会话压缩失败态（清冷却 + 硬熔断）——§4.2「手动重置」
func ResetCompactSession(sessionID string) {
	compactStateMu.Lock()
	defer compactStateMu.Unlock()
	sessions := loadCompactCooldownLocked()
	if _, ok := sessions[sessionID]; !ok {
		return
	}
	delete(sessions, sessionID)
	if err := saveCompactCooldownLocked(sessions); err != nil {
		log.Printf("⚠️ 压缩冷却状态落盘失败: %v", err)
		return
	}
	log.Printf("♻️ 会话 %s 压缩失败态已手动重置", sessionID)
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
