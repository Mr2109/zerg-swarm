package agent

// session_log.go — 会话日志持久化（v2.5.5 P1 会话日志不变量——2026-08-21 Mr2109）
// dsh 理念: 日志=上下文的唯一真相——模型看到的一切能从日志重建
// 实现: history 每次变更写会话日志——启动/续跑从日志恢复——模型请求从日志派生
// 恢复能力: CA 崩溃 → 从日志恢复上下文续跑（审计完整）

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// SessionLog 会话日志（追加写——JSON Lines）
type SessionLog struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

// NewSessionLog 创建会话日志（写日志目录）
func NewSessionLog(logDir string) (*SessionLog, error) {
	os.MkdirAll(logDir, 0o755)
	path := filepath.Join(logDir, "session.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &SessionLog{path: path, f: f}, nil
}

// Append 追加消息到会话日志（消息=上下文事实——模型可见即已记录）
func (sl *SessionLog) Append(m Message) error {
	if sl == nil || sl.f == nil {
		return nil
	}
	sl.mu.Lock()
	defer sl.mu.Unlock()
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = sl.f.Write(append(b, '\n'))
	return err
}

// AppendAll 批量追加（恢复历史——一次性）
func (sl *SessionLog) AppendAll(msgs []Message) error {
	for _, m := range msgs {
		if err := sl.Append(m); err != nil {
			return err
		}
	}
	return nil
}

// Load 从会话日志恢复历史（日志=权威——重建上下文）
func (sl *SessionLog) Load() ([]Message, error) {
	if sl == nil || sl.path == "" {
		return nil, nil
	}
	f, err := os.Open(sl.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 无日志——空历史
		}
		return nil, err
	}
	defer f.Close()

	var msgs []Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var m Message
		if err := json.Unmarshal(line, &m); err != nil {
			continue // 坏行跳过（不阻塞恢复）
		}
		msgs = append(msgs, m)
	}
	return msgs, scanner.Err()
}

// Close 关闭会话日志
func (sl *SessionLog) Close() {
	if sl != nil && sl.f != nil {
		sl.f.Close()
	}
}
