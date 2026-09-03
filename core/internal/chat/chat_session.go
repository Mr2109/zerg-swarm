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
		return nil, fmt.Errorf("chat: 新建会话失败: %w", err)
	}
	return &Session{ID: id, Title: title, Model: model, CreatedAt: t, LastActivityAt: t, Source: source, ParentSessionID: parentSessionID}, nil
}

// NewSessionID — 生成会话 ID（chat_YYYYMMDD_HHMMSS_6hex——CSPRNG 随机后缀防预测）
func NewSessionID() string {
	return fmt.Sprintf("chat_%s_%s", time.Now().Format("20060102_150405"), cryptoRandHex(3))
}

// ListSessions — 会话列表（按活动时间倒序）
func (s *ChatStore) ListSessions(limit int) ([]*Session, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, title, model, created_at, last_activity_at, message_count,
		        input_tokens, output_tokens, reasoning_tokens, pinned, archived,
		        COALESCE(parent_task_id, ''), COALESCE(source, 'desktop'), COALESCE(parent_session_id, '')
		 FROM sessions WHERE archived = 0 ORDER BY last_activity_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: 会话列表失败: %w", err)
	}
	defer rows.Close()
	var out []*Session
	for rows.Next() {
		var se Session
		var pinned, archived int
		if err := rows.Scan(&se.ID, &se.Title, &se.Model, &se.CreatedAt, &se.LastActivityAt,
			&se.MessageCount, &se.InputTokens, &se.OutputTokens, &se.ReasoningTokens,
			&pinned, &archived, &se.ParentTaskID, &se.Source, &se.ParentSessionID); err != nil {
			return nil, fmt.Errorf("chat: 会话扫描失败: %w", err)
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
		return nil, fmt.Errorf("chat: 会话不存在: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("chat: 读会话失败: %w", err)
	}
	se.Pinned = pinned == 1
	se.Archived = archived == 1
	return &se, nil
}

// UpdateSessionTitle — 更新会话标题（自动生成/手动改名）
func (s *ChatStore) UpdateSessionTitle(id, title string) error {
	_, err := s.db.Exec("UPDATE sessions SET title = ? WHERE id = ?", title, id)
	if err != nil {
		return fmt.Errorf("chat: 更新标题失败: %w", err)
	}
	return nil
}

// UpdateSessionModel — 切换会话模型
func (s *ChatStore) UpdateSessionModel(id, model string) error {
	_, err := s.db.Exec("UPDATE sessions SET model = ? WHERE id = ?", model, id)
	if err != nil {
		return fmt.Errorf("chat: 更新模型失败: %w", err)
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
		return fmt.Errorf("chat: 会话活动更新失败: %w", err)
	}
	return nil
}

// SetSessionPinned — 固定/取消固定
func (s *ChatStore) SetSessionPinned(id string, pinned bool) error {
	_, err := s.db.Exec("UPDATE sessions SET pinned = ? WHERE id = ?", boolToInt(pinned), id)
	if err != nil {
		return fmt.Errorf("chat: 固定会话失败: %w", err)
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
		return fmt.Errorf("chat: 归档会话失败: %w", err)
	}
	return nil
}

// ArchiveSession — 归档会话（软删——UI 隐藏）
func (s *ChatStore) ArchiveSession(id string) error {
	_, err := s.db.Exec("UPDATE sessions SET archived = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("chat: 归档会话失败: %w", err)
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
		return fmt.Errorf("chat: 查图片失败: %w", err)
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return fmt.Errorf("chat: 扫图片失败: %w", err)
		}
		imgs = append(imgs, p)
	}
	rows.Close()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("chat: 事务开始失败: %w", err)
	}
	defer tx.Rollback()
	// 先删 FTS 索引（按 rowid——消息 id）
	if _, err := tx.Exec(
		`DELETE FROM messages_fts WHERE rowid IN (SELECT id FROM messages WHERE session_id = ?)`,
		id,
	); err != nil {
		return fmt.Errorf("chat: 删 FTS 索引失败: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM messages WHERE session_id = ?", id); err != nil {
		return fmt.Errorf("chat: 删消息失败: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE id = ?", id); err != nil {
		return fmt.Errorf("chat: 删会话失败: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("chat: 提交失败: %w", err)
	}
	// 删图片文件（事务外——失败无害——孤儿由下次清理兜底）
	for _, p := range imgs {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			log.Printf("[chat] 删图片文件失败 %s: %v", p, err)
		}
	}
	return nil
}

// DeleteOldSessions — 90 天硬删（last_activity_at < cutoff 的会话）
func (s *ChatStore) DeleteOldSessions(cutoff float64) (int, error) {
	rows, err := s.db.Query("SELECT id FROM sessions WHERE last_activity_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("chat: 查过期会话失败: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("chat: 扫过期会话失败: %w", err)
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
