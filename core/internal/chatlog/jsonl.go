package chatlog

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxLineBytes —— 单行读取缓冲上限（正文外置 ⇒ 行天然很小；给宽裕上限防异常行）。
const maxLineBytes = 64 * 1024 * 1024

// jsonlPath —— 会话权威档路径（一文件一会话）。session_id 不许带路径分隔符。
func (s *Store) jsonlPath(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("chatlog: session_id 不能为空")
	}
	if strings.ContainsAny(sessionID, `/\`) || sessionID == "." || sessionID == ".." {
		return "", fmt.Errorf("chatlog: session_id 含非法路径字符（%q）", sessionID)
	}
	return filepath.Join(s.dir, sessionID+JSONLSuffix), nil
}

// Append —— 追加一个事件（**只增不改** —— 会话存储的核心动作）。
//   - seq 缺省 ⇒ 自动取会话内下一个（单写者 + 进程内缓存）。
//   - 正文超内联上限 ⇒ 外置到 blobs/<sha256>.bin，行里只留 blob_ref + sha256。
//   - 写盘用 O_APPEND（✗ 不 truncate · ✗ 不回写旧行）—— 这是 append-only 的硬保证。
//   - 索引面增量更新；索引失败只记错，✗ 不影响真源（真源已落盘）。
func (s *Store) Append(ev Event) (Event, error) {
	if ev.SessionID == "" {
		return Event{}, fmt.Errorf("chatlog: 事件缺 session_id")
	}
	if ev.Type == "" {
		return Event{}, fmt.Errorf("chatlog: 事件缺 type")
	}
	p, err := s.jsonlPath(ev.SessionID)
	if err != nil {
		return Event{}, err
	}

	// 大内容外置：正文落盘，列里只留 blob_ref + sha256（设计稿 §十.1 · 大工具输出 ✗ 塞 content）。
	if s.maxInlineBytes > 0 && len(ev.Content) > s.maxInlineBytes {
		sha, ref, err := s.putBlob(ev.Content)
		if err != nil {
			return Event{}, err
		}
		ev.BlobRef = ref
		ev.SHA256 = sha
		ev.Content = ""
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if ev.Seq == 0 {
		last, err := s.lastSeq(p, ev.SessionID)
		if err != nil {
			return Event{}, err
		}
		ev.Seq = last + 1
	}
	if ev.UUID == "" {
		ev.UUID = newUUID()
	}
	if ev.TS == 0 {
		ev.TS = nowUnix()
	}

	line, err := json.Marshal(ev)
	if err != nil {
		return Event{}, fmt.Errorf("chatlog: 序列化事件失败: %w", err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return Event{}, fmt.Errorf("chatlog: 打开会话档失败: %w", err)
	}
	if _, err := f.Write(line); err != nil {
		f.Close()
		return Event{}, fmt.Errorf("chatlog: 追加会话档失败: %w", err)
	}
	if err := f.Sync(); err != nil { // 追加即用：落盘后再返回
		f.Close()
		return Event{}, fmt.Errorf("chatlog: 会话档落盘失败: %w", err)
	}
	if err := f.Close(); err != nil {
		return Event{}, fmt.Errorf("chatlog: 关会话档失败: %w", err)
	}
	if ev.Seq > s.seq[ev.SessionID] {
		s.seq[ev.SessionID] = ev.Seq
	}

	// 索引面（只增）：失败只记错 —— 真源已安全落盘。
	if s.idx != nil {
		if err := s.indexEvent(ev); err != nil {
			s.idxErr = err
		}
	}
	return ev, nil
}

// lastSeq —— 会话档内最大的 seq（文件不在 ⇒ 0）。进程内缓存优先（单写者前提）。
func (s *Store) lastSeq(p, sessionID string) (int64, error) {
	if v, ok := s.seq[sessionID]; ok {
		return v, nil
	}
	var maxSeq int64
	err := scanFile(p, func(ev Event) bool {
		if ev.Seq > maxSeq {
			maxSeq = ev.Seq
		}
		return true
	})
	if err != nil && !errIsNoSession(err) {
		return 0, err
	}
	s.seq[sessionID] = maxSeq
	return maxSeq, nil
}

// errIsNoSession —— 会话档不存在（首条事件时正常）。
func errIsNoSession(err error) bool { return err == ErrNoSession }

// scanFile —— 逐行解析某会话档，fn 返回 false 即停（真源读的公共底层）。
func scanFile(p string, fn func(Event) bool) error {
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNoSession
		}
		return fmt.Errorf("chatlog: 打开会话档失败: %w", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	ln := 0
	for sc.Scan() {
		ln++
		raw := sc.Bytes()
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			return fmt.Errorf("chatlog: 会话档 %s 第 %d 行不是合法 JSON（权威档损坏）: %w", filepath.Base(p), ln, err)
		}
		if !fn(ev) {
			return nil
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("chatlog: 读会话档失败: %w", err)
	}
	return nil
}

// scan —— 读某会话（真源）。会话不存在 ⇒ ErrNoSession。
func (s *Store) scan(sessionID string, fn func(Event) bool) error {
	p, err := s.jsonlPath(sessionID)
	if err != nil {
		return err
	}
	return scanFile(p, fn)
}

// ReadSession —— 读整会话（文件序 = seq 序）。会话不存在 ⇒ ErrNoSession。
func (s *Store) ReadSession(sessionID string) ([]Event, error) {
	var out []Event
	if err := s.scan(sessionID, func(ev Event) bool { out = append(out, ev); return true }); err != nil {
		return nil, err
	}
	return out, nil
}

// PageAfter —— keyset 分页：取 seq > afterSeq 的前 limit 条（升序）。
// ★ keyset（游标）语义：靠 (session_id, seq) 唯一键翻页 —— ✗ 不用 OFFSET。
func (s *Store) PageAfter(sessionID string, afterSeq int64, limit int) ([]Event, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("chatlog: limit 必须为正（实得 %d）", limit)
	}
	var out []Event
	err := s.scan(sessionID, func(ev Event) bool {
		if ev.Seq <= afterSeq {
			return true
		}
		out = append(out, ev)
		return len(out) < limit
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListSessions —— 权威目录里有哪些会话（按 .jsonl 件名取 · ✗ 不从索引取）。
func (s *Store) ListSessions() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("chatlog: 列会话目录失败: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), JSONLSuffix) {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), JSONLSuffix))
	}
	return out, nil
}
