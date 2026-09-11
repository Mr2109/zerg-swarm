// chat_session.go — v2.5.7 对话会话 CRUD（借鉴 Hermes sessions 表精简）
// 90 天硬删（chat_lifecycle.go 定时清理）

package chat

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"
)

// cryptoRandHex — CSPRNG 随机 hex（P4-32 防预测——OWASP session ID 建议）
func cryptoRandHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		// CSPRNG 失败兜底（时间戳+纳秒——非常规路径）
		return fmt.Sprintf("%x", time.Now().UnixNano()%0xffffff)
	}
	return hex.EncodeToString(b)
}

// Session — 会话元数据
type Session struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Model           string  `json:"model"`
	CreatedAt       float64 `json:"created_at"`
	LastActivityAt  float64 `json:"last_activity_at"`
	MessageCount    int     `json:"message_count"`
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	ReasoningTokens int     `json:"reasoning_tokens"`
	Pinned          bool    `json:"pinned"`
	Archived        bool    `json:"archived"`
	ParentTaskID    string  `json:"parent_task_id,omitempty"`
	Source          string  `json:"source"`
	ParentSessionID string  `json:"parent_session_id,omitempty"`
}

// CreateSession — 新建会话（P4-32: 新格式 ID chat_YYYYMMDD_HHMMSS_6hex——ULID 思想 + CSPRNG 随机后缀）
// source: 会话来源（desktop/cron/agent/api……）; parentSessionID: 压缩分裂谱系/分支父会话
func (s *ChatStore) CreateSession(model, source, parentSessionID, title string) (*Session, error) {
	id := NewSessionID()
	t := now()
	_, err := s.db.Exec(
		"INSERT INTO sessions(id, title, model, created_at, last_activity_at, source, parent_session_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, title, model, t, t, source, parentSessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: failed to create session: %w", err)
	}
	return &Session{ID: id, Title: title, Model: model, CreatedAt: t, LastActivityAt: t, Source: source, ParentSessionID: parentSessionID}, nil
}

// NewSessionID — 生成会话 ID（chat_YYYYMMDD_HHMMSS_6hex——CSPRNG 随机后缀防预测）
func NewSessionID() string {
	return fmt.Sprintf("chat_%s_%s", time.Now().Format("20060102_150405"), cryptoRandHex(3))
}

// ListSessions — 会话列表（按活动时间倒序）
func (s *ChatStore) ListSessions(limit int) ([]*Session, error) {
	return s.ListSessionsArchived(limit, false)
}

// ListSessionsArchived — 会话列表；includeArchived=true 时含已归档（session_search browse 模式用）
func (s *ChatStore) ListSessionsArchived(limit int, includeArchived bool) ([]*Session, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, title, model, created_at, last_activity_at, message_count,
		        input_tokens, output_tokens, reasoning_tokens, pinned, archived,
		        COALESCE(parent_task_id, ''), COALESCE(source, 'desktop'), COALESCE(parent_session_id, '')
		 FROM sessions WHERE (archived = 0 OR ?) ORDER BY last_activity_at DESC LIMIT ?`,
		includeArchived, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: failed to list sessions: %w", err)
	}
	defer rows.Close()
	var out []*Session
	for rows.Next() {
		var se Session
		var pinned, archived int
		if err := rows.Scan(&se.ID, &se.Title, &se.Model, &se.CreatedAt, &se.LastActivityAt,
			&se.MessageCount, &se.InputTokens, &se.OutputTokens, &se.ReasoningTokens,
			&pinned, &archived, &se.ParentTaskID, &se.Source, &se.ParentSessionID); err != nil {
			return nil, fmt.Errorf("chat: failed to scan sessions: %w", err)
		}
		se.Pinned = pinned == 1
		se.Archived = archived == 1
		out = append(out, &se)
	}
	return out, nil
}

// GetSession — 按 id 取会话
func (s *ChatStore) GetSession(id string) (*Session, error) {
	var se Session
	var pinned, archived int
	err := s.db.QueryRow(
		`SELECT id, title, model, created_at, last_activity_at, message_count,
		        input_tokens, output_tokens, reasoning_tokens, pinned, archived,
		        COALESCE(parent_task_id, ''), COALESCE(source, 'desktop'), COALESCE(parent_session_id, '')
		 FROM sessions WHERE id = ?`,
		id,
	).Scan(&se.ID, &se.Title, &se.Model, &se.CreatedAt, &se.LastActivityAt,
		&se.MessageCount, &se.InputTokens, &se.OutputTokens, &se.ReasoningTokens,
		&pinned, &archived, &se.ParentTaskID, &se.Source, &se.ParentSessionID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("chat: session not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("chat: failed to read session: %w", err)
	}
	se.Pinned = pinned == 1
	se.Archived = archived == 1
	return &se, nil
}

// UpdateSessionTitle — 更新会话标题（自动生成/手动改名）
func (s *ChatStore) UpdateSessionTitle(id, title string) error {
	_, err := s.db.Exec("UPDATE sessions SET title = ? WHERE id = ?", title, id)
	if err != nil {
		return fmt.Errorf("chat: failed to update title: %w", err)
	}
	return nil
}

// UpdateSessionModel — 切换会话模型
func (s *ChatStore) UpdateSessionModel(id, model string) error {
	_, err := s.db.Exec("UPDATE sessions SET model = ? WHERE id = ?", model, id)
	if err != nil {
		return fmt.Errorf("chat: failed to update model: %w", err)
	}
	return nil
}

// TouchSession — 会话活动（更新 last_activity_at + 消息计数 + token 累计）
func (s *ChatStore) TouchSession(id string, inputTokens, outputTokens, reasoningTokens int) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET last_activity_at = ?,
		 message_count = message_count + 1,
		 input_tokens = input_tokens + ?,
		 output_tokens = output_tokens + ?,
		 reasoning_tokens = reasoning_tokens + ?
		 WHERE id = ?`,
		now(), inputTokens, outputTokens, reasoningTokens, id,
	)
	if err != nil {
		return fmt.Errorf("chat: failed to update session activity: %w", err)
	}
	return nil
}

// SetSessionPinned — 固定/取消固定
func (s *ChatStore) SetSessionPinned(id string, pinned bool) error {
	_, err := s.db.Exec("UPDATE sessions SET pinned = ? WHERE id = ?", boolToInt(pinned), id)
	if err != nil {
		return fmt.Errorf("chat: failed to pin session: %w", err)
	}
	return nil
}

// SetArchived — 归档/取消归档会话（P4-33 右键菜单归档——archived=1 列表隐藏）
func (s *ChatStore) SetArchived(id string, archived bool) error {
	v := 0
	if archived {
		v = 1
	}
	_, err := s.db.Exec("UPDATE sessions SET archived = ? WHERE id = ?", v, id)
	if err != nil {
		return fmt.Errorf("chat: failed to archive session: %w", err)
	}
	return nil
}

// ArchiveSession — 归档会话（软删——UI 隐藏）
func (s *ChatStore) ArchiveSession(id string) error {
	_, err := s.db.Exec("UPDATE sessions SET archived = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("chat: failed to archive session: %w", err)
	}
	return nil
}

// DeleteSession — 硬删会话（含消息 + FTS 索引 + 图片文件）
// v2.5.7 细节补全: 90 天销毁连图片文件一起清（此前只删 DB 行——images/ 留孤儿文件）
func (s *ChatStore) DeleteSession(id string) error {
	// 先收集图片路径（消息删前——image_path 列）
	var imgs []string
	rows, err := s.db.Query("SELECT image_path FROM messages WHERE session_id = ? AND image_path != ''", id)
	if err != nil {
		return fmt.Errorf("chat: failed to query images: %w", err)
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return fmt.Errorf("chat: failed to scan images: %w", err)
		}
		imgs = append(imgs, p)
	}
	rows.Close()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("chat: failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	// 先删 FTS 索引（按 rowid——消息 id）
	if _, err := tx.Exec(
		`DELETE FROM messages_fts WHERE rowid IN (SELECT id FROM messages WHERE session_id = ?)`,
		id,
	); err != nil {
		return fmt.Errorf("chat: failed to delete FTS index: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM messages WHERE session_id = ?", id); err != nil {
		return fmt.Errorf("chat: failed to delete messages: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE id = ?", id); err != nil {
		return fmt.Errorf("chat: failed to delete session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("chat: commit failed: %w", err)
	}
	// 删图片文件（事务外——失败无害——孤儿由下次清理兜底）
	for _, p := range imgs {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			log.Printf("[chat] failed to delete image file %s: %v", p, err)
		}
	}
	return nil
}

// SoftDeleteExpired — 阶段1：过期会话软删（archived=1 + purge_after=now+grace）
// 甲批 T3（2026-09-10）：软删→宽限→级联硬删；幂等（purge_after 非空者不重复处理，用户已归档者不动）
func (s *ChatStore) SoftDeleteExpired(cutoff, purgeAfter float64) (int, error) {
	res, err := s.db.Exec(
		`UPDATE sessions SET archived = 1, purge_after = ?
		 WHERE last_activity_at < ? AND archived = 0 AND purge_after IS NULL`,
		purgeAfter, cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: failed to soft-delete expired sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// PurgeSoftDeleted — 阶段2：宽限期满 → 级联硬删（消息 + FTS + 图片）；幂等（可重复调用，失败下次重试）
func (s *ChatStore) PurgeSoftDeleted(now float64) (int, error) {
	rows, err := s.db.Query("SELECT id FROM sessions WHERE purge_after IS NOT NULL AND purge_after <= ?", now)
	if err != nil {
		return 0, fmt.Errorf("chat: failed to query sessions pending cleanup: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("chat: failed to scan sessions pending cleanup: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	done := 0
	for _, id := range ids {
		if err := s.DeleteSession(id); err != nil {
			log.Printf("[chat] cascade hard-delete failed %s (retrying next round): %v", id, err)
			continue
		}
		done++
	}
	return done, nil
}

// CountCleanupPlan — 干跑：统计将软删/将硬删的会话数（不修改数据）
func (s *ChatStore) CountCleanupPlan(cutoff, now float64) (soft, purge int, err error) {
	if err = s.db.QueryRow(
		`SELECT count(*) FROM sessions WHERE last_activity_at < ? AND archived = 0 AND purge_after IS NULL`,
		cutoff).Scan(&soft); err != nil {
		return
	}
	err = s.db.QueryRow(
		`SELECT count(*) FROM sessions WHERE purge_after IS NOT NULL AND purge_after <= ?`, now).Scan(&purge)
	return
}

// DeleteOldSessions — 90 天硬删（last_activity_at < cutoff 的会话）
func (s *ChatStore) DeleteOldSessions(cutoff float64) (int, error) {
	rows, err := s.db.Query("SELECT id FROM sessions WHERE last_activity_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("chat: failed to query expired sessions: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("chat: failed to scan expired sessions: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.DeleteSession(id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// timeNowMillis — 毫秒时间戳（会话 id 用）
func timeNowMillis() int64 {
	return int64(now() * 1000)
}

// boolToInt — bool → int
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
