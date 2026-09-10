// chat_message.go — v2.5.7 对话消息读写（借鉴 Hermes messages 表精简）
// 字段: role/content/reasoning（思考分离）/tool_calls/token_count/active/compacted

package chat

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Message — 对话消息
type Message struct {
	ID         int64   `json:"id"`
	SessionID  string  `json:"session_id"`
	Role       string  `json:"role"` // user / assistant / tool / system
	Content    string  `json:"content"`
	Reasoning  string  `json:"reasoning,omitempty"`  // 思考内容（独立显示——Hermes 式）
	ToolCalls  string  `json:"tool_calls,omitempty"` // JSON 数组（工具调用）
	ToolName   string  `json:"tool_name,omitempty"`  // 工具消息的工具名
	Model      string  `json:"model,omitempty"`
	TokenCount int     `json:"token_count"`
	Active     bool    `json:"active"`               // 上下文窗口内（Hermes 式）
	Compacted  bool    `json:"compacted"`            // 压缩摘要标记
	ImagePath  string  `json:"image_path,omitempty"` // D3 多模态：图片文件路径
	Timestamp  float64 `json:"timestamp"`
}

// AddMessage — 追加消息（事务写——崩溃恢复）
func (s *ChatStore) AddMessage(m *Message) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO messages(session_id, role, content, reasoning, tool_calls, tool_name,
		 model, token_count, active, compacted, image_path, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.SessionID, m.Role, m.Content, m.Reasoning, m.ToolCalls, m.ToolName,
		m.Model, m.TokenCount, boolToInt(m.Active), boolToInt(m.Compacted), m.ImagePath, m.Timestamp,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: 写消息失败: %w", err)
	}
	id, _ := res.LastInsertId()
	// FTS5 索引同步（应用层管理——触发器在 modernc 不稳）
	if m.Content != "" {
		_, _ = s.db.Exec("INSERT INTO messages_fts(rowid, content) VALUES (?, ?)", id, m.Content)
	}
	// 更新会话活动
	_ = s.TouchSession(m.SessionID, 0, 0, 0)
	return id, nil
}

// ListMessages — 会话消息（时间正序——完整历史）
func (s *ChatStore) ListMessages(sessionID string) ([]*Message, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, COALESCE(reasoning, ''), COALESCE(tool_calls, ''),
		        COALESCE(tool_name, ''), COALESCE(model, ''), token_count, active, compacted,
		        COALESCE(image_path, ''), timestamp
		 FROM messages WHERE session_id = ? ORDER BY timestamp, id`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: 读消息失败: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

// scanMessages — 公共消息行扫描（ListMessages/ListMessagesByID 共用）
func scanMessages(rows *sql.Rows) ([]*Message, error) {
	var out []*Message
	for rows.Next() {
		var m Message
		var active, compacted int
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Reasoning, &m.ToolCalls,
			&m.ToolName, &m.Model, &m.TokenCount, &active, &compacted, &m.ImagePath, &m.Timestamp); err != nil {
			return nil, fmt.Errorf("chat: 消息扫描失败: %w", err)
		}
		m.Active = active == 1
		m.Compacted = compacted == 1
		out = append(out, &m)
	}
	return out, nil
}

// ListMessagesByID — 按消息 id 查（P0 编辑反查 session——单条）
func (s *ChatStore) ListMessagesByID(id int64) ([]*Message, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, COALESCE(reasoning, ''), COALESCE(tool_calls, ''),
		        COALESCE(tool_name, ''), COALESCE(model, ''), token_count, active, compacted,
		        COALESCE(image_path, ''), timestamp
		 FROM messages WHERE id = ? LIMIT 1`, id)
	if err != nil {
		return nil, fmt.Errorf("chat: 查消息失败: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

// GetActiveMessages — 上下文窗口内消息（active=1——压缩后旧消息被标记 inactive）
func (s *ChatStore) GetActiveMessages(sessionID string) ([]*Message, error) {
	rows, err := s.db.Query(
		`SELECT id, session_id, role, content, COALESCE(reasoning, ''), COALESCE(tool_calls, ''),
		        COALESCE(tool_name, ''), COALESCE(model, ''), token_count, active, compacted,
		        COALESCE(image_path, ''), timestamp
		 FROM messages WHERE session_id = ? AND active = 1 ORDER BY timestamp, id`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: 读活动消息失败: %w", err)
	}
	defer rows.Close()
	var out []*Message
	for rows.Next() {
		var m Message
		var active, compacted int
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Reasoning, &m.ToolCalls,
			&m.ToolName, &m.Model, &m.TokenCount, &active, &compacted, &m.ImagePath, &m.Timestamp); err != nil {
			return nil, fmt.Errorf("chat: 消息扫描失败: %w", err)
		}
		m.Active = active == 1
		m.Compacted = compacted == 1
		out = append(out, &m)
	}
	return out, nil
}

// MarkInactive — 标记旧消息 inactive（压缩时——保护最近 N 条）
func (s *ChatStore) MarkInactive(sessionID string, keepLatest int) (int, error) {
	// 找最新 keepLatest 条之外的 id
	rows, err := s.db.Query(
		`SELECT id FROM messages WHERE session_id = ? AND active = 1
		 ORDER BY timestamp DESC, id DESC LIMIT -1 OFFSET ?`,
		sessionID, keepLatest,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: 查旧消息失败: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("chat: 扫旧消息失败: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if _, err := s.db.Exec("UPDATE messages SET active = 0 WHERE id = ?", id); err != nil {
			return 0, fmt.Errorf("chat: 标记失败: %w", err)
		}
	}
	return len(ids), nil
}

// InsertSummary — 插入压缩摘要（compacted=1——窗口头部）
func (s *ChatStore) InsertSummary(sessionID, summary string, timestamp float64) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO messages(session_id, role, content, active, compacted, timestamp)
		 VALUES (?, 'system', ?, 1, 1, ?)`,
		sessionID, summary, timestamp,
	)
	if err != nil {
		return 0, fmt.Errorf("chat: 写摘要失败: %w", err)
	}
	return res.LastInsertId()
}

// DeleteMessage — 删单条消息（推理失败回滚用——含 FTS 索引）
func (s *ChatStore) DeleteMessage(sessionID string, id int64) error {
	if _, err := s.db.Exec("DELETE FROM messages_fts WHERE rowid = ?", id); err != nil {
		return fmt.Errorf("chat: 删 FTS 索引失败: %w", err)
	}
	if _, err := s.db.Exec("DELETE FROM messages WHERE id = ? AND session_id = ?", id, sessionID); err != nil {
		return fmt.Errorf("chat: 删消息失败: %w", err)
	}
	return nil
}

// UpdateMessageContent — 更新消息内容（P0 点击编辑——Hermes user-edit 借鉴——含 FTS 同步）
func (s *ChatStore) UpdateMessageContent(sessionID string, id int64, content string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("chat: 编辑消息事务失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE messages SET content = ? WHERE id = ? AND session_id = ? AND role = 'user'", content, id, sessionID); err != nil {
		return fmt.Errorf("chat: 编辑消息失败: %w", err)
	}
	// FTS 同步更新（应用层管理索引——FTS5 delete 触发器不稳）
	if _, err := tx.Exec("DELETE FROM messages_fts WHERE rowid = ?", id); err != nil {
		return fmt.Errorf("chat: 编辑 FTS 删旧失败: %w", err)
	}
	if _, err := tx.Exec("INSERT INTO messages_fts(rowid, content) VALUES (?, ?)", id, content); err != nil {
		return fmt.Errorf("chat: 编辑 FTS 插新失败: %w", err)
	}
	return tx.Commit()
}

// SoftDeleteAfter — 批次C3(2026-09-10): 软删某消息之后的所有消息（active=0——可搜可恢复）
// 用于"编辑即截断重跑"/重生成——不硬删（FTS 仍可搜旧分支——对齐 Hermes 软归档哲学）
func (s *ChatStore) SoftDeleteAfter(sessionID string, afterID int64) (int, error) {
	res, err := s.db.Exec(
		"UPDATE messages SET active = 0 WHERE session_id = ? AND id > ? AND active = 1",
		sessionID, afterID)
	if err != nil {
		return 0, fmt.Errorf("chat: 软删截断失败: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// LastUserMessage — 批次C3: 会话最后一条 user 消息（重生成用）
func (s *ChatStore) LastUserMessage(sessionID string) (*Message, error) {
	msgs, err := s.ListMessages(sessionID)
	if err != nil {
		return nil, err
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i], nil
		}
	}
	return nil, nil
}

// MarkCompacted — 把指定消息标记为已压缩（active=false——不进上下文窗口）
func (s *ChatStore) MarkCompacted(sessionID string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.Exec("UPDATE messages SET active = 0, compacted = 1 WHERE id = ? AND session_id = ?", id, sessionID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// InsertSummaryMid — 压缩摘要插入（role=user【历史摘要】——llama-server system 须唯一在开头）
// 插在 beforeID（压缩段首条）之前——顺序: 首 N 条 → 摘要 → 尾 N 条
func (s *ChatStore) InsertSummaryMid(sessionID, summary string, beforeID int64) (int64, error) {
	if summary == "" {
		return 0, nil
	}
	// 取 firstID 的 timestamp 作为基准
	var base float64
	err := s.db.QueryRow("SELECT timestamp FROM messages WHERE id = ?", beforeID).Scan(&base)
	if err != nil {
		base = float64(time.Now().Unix())
	}
	content := "【历史摘要】" + summary
	// P4-39 T7: 摘要角色交替——查 beforeID 前一条 role——摘要用相反（防连续同角色）
	prevRole := ""
	_ = s.db.QueryRow("SELECT role FROM messages WHERE id < ? AND session_id = ? ORDER BY id DESC LIMIT 1",
		beforeID, sessionID).Scan(&prevRole)
	sumRole := "assistant"
	if prevRole == "assistant" {
		sumRole = "user"
	}
	res, err := s.db.Exec(
		"INSERT INTO messages (session_id, role, content, active, compacted, timestamp) VALUES (?, ?, ?, 1, 0, ?)",
		sessionID, sumRole, content, base-0.001)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	// FTS 同步（摘要也搜索）
	if _, err := s.db.Exec("INSERT INTO messages_fts (rowid, content) VALUES (?, ?)", id, content); err != nil {
		return 0, err
	}
	return id, nil
}

// CompressHistory — 一站式压缩（ShouldCompact 为 true 时调用——生成摘要 + 标记 + 插入）
// 返回: (是否执行了压缩)  P4-39 model 参数——按模型 ctx 算阈值
func (s *ChatStore) CompressHistory(ctx context.Context, infer *ChatInfer, sessionID, model string, msgs []*Message) (bool, error) {
	// P4-39 T6: 会话级压缩锁——压缩中防并发双压（第二个请求直接跳过）
	if _, loaded := s.compactLocks.LoadOrStore(sessionID, struct{}{}); loaded {
		return false, nil
	}
	defer s.compactLocks.Delete(sessionID)
	if !ShouldCompactPtr(msgs, model) {
		return false, nil
	}
	var summarySrc []Message
	var compressIDs []int64
	for i := CompactProtectFirstN; i < len(msgs)-CompactProtectLastN; i++ {
		summarySrc = append(summarySrc, *msgs[i])
		compressIDs = append(compressIDs, msgs[i].ID)
	}
	if len(summarySrc) == 0 || len(compressIDs) == 0 {
		return false, nil
	}
	summary, err := CompactRequest(ctx, infer.GatewayURL, infer.AuthToken, sessionID, model, summarySrc)
	if err != nil {
		// 压缩失败不阻塞对话（跳过压缩——下次再试）
		return false, nil
	}
	// P4-39 Phase 1: 剪除中间段旧工具结果（>200 字符——落库占位——省上下文——Hermes 4 阶段）
	if err := s.PruneOldToolResults(sessionID, summarySrc); err != nil {
		return false, nil
	}
	if err := s.MarkCompacted(sessionID, compressIDs); err != nil {
		return false, nil
	}
	// 摘要插在压缩段首条之前
	if _, err := s.InsertSummaryMid(sessionID, summary, compressIDs[0]); err != nil {
		return false, nil
	}
	return true, nil
}

// PruneOldToolResults — P4-39 Phase 1: 剪除旧工具结果（>200 字符→占位——无 LLM 调用——省上下文）
// Hermes 4 阶段 Phase 1 对标——本地小模型上下文金贵——旧工具结果是最大头
const compactPrunePlaceholder = "[旧工具输出已清空以节省上下文]"

func (s *ChatStore) PruneOldToolResults(sessionID string, msgs []Message) error {
	for _, m := range msgs {
		if m.ToolName != "" && len([]rune(m.Content)) > 200 {
			if _, err := s.db.Exec("UPDATE messages SET content = ? WHERE id = ? AND session_id = ?",
				compactPrunePlaceholder, m.ID, sessionID); err != nil {
				return fmt.Errorf("chat: 剪除工具结果失败: %w", err)
			}
		}
	}
	return nil
}

// DeleteSessionMessages — 删会话全部消息（硬删——90 天销毁）
func (s *ChatStore) DeleteSessionMessages(sessionID string) error {
	_, err := s.db.Exec("DELETE FROM messages WHERE session_id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("chat: 删消息失败: %w", err)
	}
	return nil
}

// errNoRows — 供外部判断
var errNoRows = sql.ErrNoRows
