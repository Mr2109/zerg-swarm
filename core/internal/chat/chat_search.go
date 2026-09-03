// chat_search.go — v2.5.7 对话搜索（FTS5 全文索引——借鉴 Hermes messages_fts）

package chat

import (
	"fmt"
	"strings"
)

// SearchResult — 搜索结果
type SearchResult struct {
	SessionID string  `json:"session_id"`
	MessageID int64   `json:"message_id"`
	Role      string  `json:"role"`
	Content   string  `json:"content"`
	SessionTitle string `json:"session_title"`
	Timestamp float64 `json:"timestamp"`
}

// SearchMessages — FTS5 全文搜索（跨会话）
func (s *ChatStore) SearchMessages(query string, limit int) ([]*SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	// FTS5 MATCH——查询词转引用
	q := fmt.Sprintf(`"%s"`, strings.ReplaceAll(strings.TrimSpace(query), `"`, `""`))
	rows, err := s.db.Query(
		`SELECT m.session_id, m.id, m.role, m.content,
		        COALESCE(se.title, ''), m.timestamp
		 FROM messages_fts f
		 JOIN messages m ON m.id = f.rowid
		 JOIN sessions se ON se.id = m.session_id
		 WHERE messages_fts MATCH ? AND se.archived = 0
		 ORDER BY m.timestamp DESC LIMIT ?`,
		q, limit,
	)
	if err != nil {
		// FTS 语法错误等——降级为 LIKE 搜索
		return s.searchLike(query, limit)
	}
	defer rows.Close()
	var out []*SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.SessionID, &r.MessageID, &r.Role, &r.Content, &r.SessionTitle, &r.Timestamp); err != nil {
			return nil, fmt.Errorf("chat: 搜索扫描失败: %w", err)
		}
		out = append(out, &r)
	}
	return out, nil
}

// searchLike — FTS 失败降级（LIKE 模糊）
func (s *ChatStore) searchLike(query string, limit int) ([]*SearchResult, error) {
	like := "%" + query + "%"
	rows, err := s.db.Query(
		`SELECT m.session_id, m.id, m.role, m.content,
		        COALESCE(se.title, ''), m.timestamp
		 FROM messages m JOIN sessions se ON se.id = m.session_id
		 WHERE m.content LIKE ? AND se.archived = 0
		 ORDER BY m.timestamp DESC LIMIT ?`,
		like, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("chat: 搜索降级失败: %w", err)
	}
	defer rows.Close()
	var out []*SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.SessionID, &r.MessageID, &r.Role, &r.Content, &r.SessionTitle, &r.Timestamp); err != nil {
			return nil, fmt.Errorf("chat: 搜索扫描失败: %w", err)
		}
		out = append(out, &r)
	}
	return out, nil
}
