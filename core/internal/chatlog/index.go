package chatlog

import (
	"database/sql"
	"fmt"
	"os"
)

// indexSchema —— 索引面 schema（SQLite + FTS5 **外部内容表**）。
//   - chat_messages 是投影表（只是索引用的副本 · ✗ 不是权威）。
//   - chat_messages_fts 用 content='chat_messages'（外部内容表：FTS 只存倒排索引，
//     正文从内容表读 ⇒ ✗ 不存第二份正文）。
//   - (session_id, seq) 唯一索引 ⇒ keyset 分页与去重。
const indexSchema = `
CREATE TABLE IF NOT EXISTS chat_messages (
    session_id TEXT NOT NULL,
    seq        INTEGER NOT NULL,
    type       TEXT NOT NULL,
    role       TEXT,
    content    TEXT,
    ts         REAL NOT NULL,
    PRIMARY KEY (session_id, seq)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_messages_session_seq ON chat_messages(session_id, seq);
CREATE VIRTUAL TABLE IF NOT EXISTS chat_messages_fts USING fts5(
    content,
    content='chat_messages',
    content_rowid='rowid',
    tokenize='trigram'
);
CREATE TRIGGER IF NOT EXISTS chat_messages_ai AFTER INSERT ON chat_messages BEGIN
    INSERT INTO chat_messages_fts(rowid, content) VALUES (new.rowid, new.content);
END;
CREATE TRIGGER IF NOT EXISTS chat_messages_ad AFTER DELETE ON chat_messages BEGIN
    INSERT INTO chat_messages_fts(chat_messages_fts, rowid, content) VALUES('delete', old.rowid, old.content);
END;
CREATE TRIGGER IF NOT EXISTS chat_messages_au AFTER UPDATE ON chat_messages BEGIN
    INSERT INTO chat_messages_fts(chat_messages_fts, rowid, content) VALUES('delete', old.rowid, old.content);
    INSERT INTO chat_messages_fts(rowid, content) VALUES (new.rowid, new.content);
END;
`

// Hit —— 一条检索 / 索引页码命中。
type Hit struct {
	SessionID string  `json:"session_id"`
	Seq       int64   `json:"seq"`
	Role      string  `json:"role,omitempty"`
	TS        float64 `json:"ts"`
	Score     float64 `json:"score,omitempty"` // bm25（越小越相关）
}

// openIndex —— 打开 / 初始化索引库。★ 失败**不返回错**，只记 idxErr（真源不依赖索引）。
func (s *Store) openIndex() {
	db, err := sql.Open("sqlite", s.indexPath)
	if err != nil {
		s.idx, s.idxErr = nil, fmt.Errorf("chatlog: 打开索引库失败: %w", err)
		return
	}
	// 写池 MaxOpenConns=1（与库稿同口径：进程内把并发写串行化）。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		s.idx, s.idxErr = nil, fmt.Errorf("chatlog: 索引库置 WAL 失败（疑库损坏）: %w", err)
		return
	}
	if _, err := db.Exec(indexSchema); err != nil {
		db.Close()
		s.idx, s.idxErr = nil, fmt.Errorf("chatlog: 索引库建表失败（疑库损坏）: %w", err)
		return
	}
	s.idx, s.idxErr = db, nil
}

// indexEvent —— 把一条事件投影进索引面（幂等：ON CONFLICT DO NOTHING ⇒ 重建可重放）。
func (s *Store) indexEvent(ev Event) error {
	if s.idx == nil {
		return ErrIndexUnavailable
	}
	_, err := s.idx.Exec(
		`INSERT INTO chat_messages(session_id, seq, type, role, content, ts) VALUES(?,?,?,?,?,?)
		 ON CONFLICT(session_id, seq) DO NOTHING`,
		ev.SessionID, ev.Seq, ev.Type, nullStr(ev.Role), ev.Content, ev.TS)
	if err != nil {
		return fmt.Errorf("chatlog: 索引事件失败: %w", err)
	}
	return nil
}

// Search —— FTS5 检索（走索引面）。索引不可用 ⇒ ErrIndexUnavailable（✗ 不静默返空）。
func (s *Store) Search(query string, limit int) ([]Hit, error) {
	if s.idx == nil {
		if s.idxErr != nil {
			return nil, fmt.Errorf("%w：%v", ErrIndexUnavailable, s.idxErr)
		}
		return nil, ErrIndexUnavailable
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.idx.Query(
		`SELECT m.session_id, m.seq, COALESCE(m.role,''), m.ts, bm25(chat_messages_fts) AS score
		   FROM chat_messages_fts
		   JOIN chat_messages m ON m.rowid = chat_messages_fts.rowid
		  WHERE chat_messages_fts MATCH ?
		  ORDER BY score
		  LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("chatlog: 检索失败: %w", err)
	}
	defer rows.Close()
	var out []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.SessionID, &h.Seq, &h.Role, &h.TS, &h.Score); err != nil {
			return nil, fmt.Errorf("chatlog: 读检索结果失败: %w", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatlog: 检索结果读取中断: %w", err)
	}
	return out, nil
}

// PageIndex —— 索引侧 keyset 分页（走 (session_id, seq) 索引）。
func (s *Store) PageIndex(sessionID string, afterSeq int64, limit int) ([]Hit, error) {
	if s.idx == nil {
		if s.idxErr != nil {
			return nil, fmt.Errorf("%w：%v", ErrIndexUnavailable, s.idxErr)
		}
		return nil, ErrIndexUnavailable
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.idx.Query(
		`SELECT session_id, seq, COALESCE(role,''), ts
		   FROM chat_messages
		  WHERE session_id = ? AND seq > ?
		  ORDER BY seq
		  LIMIT ?`, sessionID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("chatlog: 索引分页失败: %w", err)
	}
	defer rows.Close()
	var out []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.SessionID, &h.Seq, &h.Role, &h.TS); err != nil {
			return nil, fmt.Errorf("chatlog: 读索引分页结果失败: %w", err)
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chatlog: 索引分页结果读取中断: %w", err)
	}
	return out, nil
}

// IndexSessions —— 索引面里有哪些会话（对拍权威目录用）。
func (s *Store) IndexSessions() ([]string, error) {
	if s.idx == nil {
		return nil, ErrIndexUnavailable
	}
	rows, err := s.idx.Query(`SELECT DISTINCT session_id FROM chat_messages ORDER BY session_id`)
	if err != nil {
		return nil, fmt.Errorf("chatlog: 列索引会话失败: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var sid string
		if err := rows.Scan(&sid); err != nil {
			return nil, fmt.Errorf("chatlog: 读索引会话失败: %w", err)
		}
		out = append(out, sid)
	}
	return out, rows.Err()
}

// RebuildIndex —— 从**真源**（JSONL）整份重建索引：清库 → 重建 schema → 重灌全部会话。
// ★ 这是「索引可丢」的落地：索引损坏 / 缺失 / 陈旧，一律靠它从权威档复原。
func (s *Store) RebuildIndex() error {
	if err := s.resetIndex(); err != nil {
		return err
	}
	if s.idx == nil {
		return fmt.Errorf("%w：%v", ErrIndexUnavailable, s.idxErr)
	}
	sessions, err := s.ListSessions()
	if err != nil {
		return err
	}
	for _, sid := range sessions {
		var indexErr error
		if err := s.scan(sid, func(ev Event) bool {
			if err := s.indexEvent(ev); err != nil {
				indexErr = err
				return false
			}
			return true
		}); err != nil {
			return err
		}
		if indexErr != nil {
			return indexErr
		}
	}
	return nil
}

// resetIndex —— 丢掉旧索引库（含 -wal/-shm）并重新开一枚空库（✗ 不碰真源）。
func (s *Store) resetIndex() error {
	if s.idx != nil {
		s.idx.Close()
		s.idx = nil
	}
	for _, suf := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(s.indexPath + suf); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("chatlog: 清索引库失败: %w", err)
		}
	}
	s.idxErr = nil
	s.openIndex()
	return nil
}

// nullStr —— 空串存 NULL（可空列的干净语义）。
func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}
