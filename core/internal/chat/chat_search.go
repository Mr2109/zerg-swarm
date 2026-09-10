// chat_search.go — v2.5.7 对话搜索（FTS5 全文索引——借鉴 Hermes messages_fts）

package chat

import (
	"fmt"
	"strings"
)

// SearchResult — 搜索结果
type SearchResult struct {
	SessionID    string  `json:"session_id"`
	MessageID    int64   `json:"message_id"`
	Role         string  `json:"role"`
	Content      string  `json:"content"`
	SessionTitle string  `json:"session_title"`
	Timestamp    float64 `json:"timestamp"`
}

// SearchMessages — FTS5 全文搜索（跨会话；默认排除已归档会话）
func (s *ChatStore) SearchMessages(query string, limit int) ([]*SearchResult, error) {
	return s.SearchMessagesArchived(query, limit, false)
}

// SearchMessagesArchived — 全文搜索；includeArchived=true 时把已归档会话也纳入
// （丙批 C2 补 2026-09-10 Mr2109拍板：归档不该等于"搜不到"——归档只是不占活跃视线）
func (s *ChatStore) SearchMessagesArchived(query string, limit int, includeArchived bool) ([]*SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	// T4（2026-09-10）：trigram 分词器下 **<3 字的 CJK 查询 MATCH 必零命中**（实测），
	// 故查询侧分流：含 CJK 且去空白后 <3 字 → 直接 LIKE（trigram 索引会加速 LIKE，实测 1ms 级）。
	flat := strings.Join(strings.Fields(query), "")
	if hasCJK(flat) && len([]rune(flat)) < 3 {
		return s.searchLikeArchived(strings.TrimSpace(query), limit, includeArchived)
	}
	// FTS5 MATCH——查询词转引用
	q := fmt.Sprintf(`"%s"`, strings.ReplaceAll(strings.TrimSpace(query), `"`, `""`))
	rows, err := s.db.Query(
		`SELECT m.session_id, m.id, m.role, m.content,
		        COALESCE(se.title, ''), m.timestamp
		 FROM messages_fts f
		 JOIN messages m ON m.id = f.rowid
		 JOIN sessions se ON se.id = m.session_id
		 WHERE messages_fts MATCH ? AND (se.archived = 0 OR ?)
		 ORDER BY m.timestamp DESC LIMIT ?`,
		q, includeArchived, limit,
	)
	if err != nil {
		// FTS 语法错误等——降级为 LIKE 搜索
		return s.searchLikeArchived(query, limit, includeArchived)
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
	if len(out) == 0 && hasCJK(query) {
		return s.searchLikeArchived(strings.TrimSpace(query), limit, includeArchived)
	}
	return out, nil
}

// hasCJK — 是否含 CJK 汉字（T4 查询侧分流用）
func hasCJK(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

// searchLike — FTS 失败降级（LIKE 模糊；默认排除归档）
func (s *ChatStore) searchLike(query string, limit int) ([]*SearchResult, error) {
	return s.searchLikeArchived(query, limit, false)
}

// searchLikeArchived — LIKE 降级（可选含归档）
func (s *ChatStore) searchLikeArchived(query string, limit int, includeArchived bool) ([]*SearchResult, error) {
	like := "%" + query + "%"
	rows, err := s.db.Query(
		`SELECT m.session_id, m.id, m.role, m.content,
		        COALESCE(se.title, ''), m.timestamp
		 FROM messages m JOIN sessions se ON se.id = m.session_id
		 WHERE m.content LIKE ? AND (se.archived = 0 OR ?)
		 ORDER BY m.timestamp DESC LIMIT ?`,
		like, includeArchived, limit,
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
