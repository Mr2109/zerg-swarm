// chat_store.go — v2.5.7 对话模块存储（借鉴 Hermes sessions/messages 双表——Mr2109 Go SQLite）
// SQLite: modernc.org/sqlite（纯 Go 无 CGO——跨平台 X3/mini）
// 表: sessions（会话元数据）+ messages（消息流）+ messages_fts（FTS5 全文索引）+ schema_version

package chat

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ChatStore — 对话 SQLite 存储（会话 + 消息）
type ChatStore struct {
	db           *sql.DB
	compactLocks sync.Map // P4-39 T6: 会话级压缩锁（sessionID→struct{}——压缩中防并发双压）
	rtLocks      sync.Map // P4-50 渐进式常驻: 会话级运行时锁（sessionID→*sync.Mutex）
	rtMap        sync.Map // P4-50 渐进式常驻: sessionID → *ToolRuntime（对话级状态——活跃期内存）
}

// GetToolRuntime — 取会话渐进式常驻状态（lazy 创建——不跨会话背债）
func (s *ChatStore) GetToolRuntime(sessionID string) *ToolRuntime {
	if v, ok := s.rtMap.Load(sessionID); ok {
		return v.(*ToolRuntime)
	}
	lockI, _ := s.rtLocks.LoadOrStore(sessionID, &sync.Mutex{})
	lock := lockI.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	if v, ok := s.rtMap.Load(sessionID); ok {
		return v.(*ToolRuntime)
	}
	rt := NewToolRuntime()
	s.rtMap.Store(sessionID, rt)
	return rt
}

// schemaVersion — 当前 schema 版本（升级迁移用）
// v1: 初始（C1）
// v2: D3 多模态——messages 加 image_path 列（图片消息存储）
// v3: P4-32 对话 ID 系统升级——sessions 加 source/parent_session_id + messages 加 parent_message_id（分支 DAG 预留）+ 索引
const schemaVersion = 3

// schemaSQL — 建表语句（借鉴 Hermes 精简）
const schemaSQL = `
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    title TEXT DEFAULT '',
    model TEXT DEFAULT 'example-35b-v2',
    created_at REAL NOT NULL,
    last_activity_at REAL NOT NULL,
    message_count INTEGER DEFAULT 0,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    reasoning_tokens INTEGER DEFAULT 0,
    pinned INTEGER DEFAULT 0,
    archived INTEGER DEFAULT 0,
    parent_task_id TEXT,
    source TEXT NOT NULL DEFAULT 'desktop',
    parent_session_id TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_activity ON sessions(last_activity_at DESC);
-- 注: idx_sessions_source/parent 在 init() 迁移后创建（旧表无新列时 CREATE INDEX 会失败——P4-32 实测坑）

CREATE TABLE IF NOT EXISTS messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    role TEXT NOT NULL,
    content TEXT,
    reasoning TEXT,
    tool_calls TEXT,
    tool_name TEXT,
    model TEXT,
    token_count INTEGER DEFAULT 0,
    active INTEGER DEFAULT 1,
    compacted INTEGER DEFAULT 0,
    image_path TEXT,
    parent_message_id INTEGER,
    timestamp REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, timestamp);

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(content, tokenize='unicode61');
`

// OpenChatStore — 打开/初始化对话数据库（/tmp/zerg-chat/chat.db——90 天销毁同区）
func OpenChatStore() (*ChatStore, error) {
	dir := "/tmp/zerg-chat"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("chat: 创建目录失败: %w", err)
	}
	path := filepath.Join(dir, "chat.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("chat: 打开数据库失败: %w", err)
	}
	// WAL 模式（崩溃恢复 + 读写并发）
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("chat: 设置 WAL 失败: %w", err)
	}
	s := &ChatStore{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// init — 建表 + schema 版本检查
func (s *ChatStore) init() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("chat: 建表失败: %w", err)
	}
	// schema 版本
	var v int
	err := s.db.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&v)
	if err == sql.ErrNoRows {
		_, err = s.db.Exec("INSERT INTO schema_version(version) VALUES (?)", schemaVersion)
		if err != nil {
			return fmt.Errorf("chat: 写 schema 版本失败: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("chat: 读 schema 版本失败: %w", err)
	} else if v != schemaVersion {
		// 版本不匹配——逐级迁移
		if v < 2 {
			// v1→v2: messages 加 image_path 列（D3 多模态）
			if _, err := s.db.Exec("ALTER TABLE messages ADD COLUMN image_path TEXT"); err != nil {
				return fmt.Errorf("chat: 迁移 v2 失败: %w", err)
			}
		}
		if v < 3 {
			// v2→v3: P4-32 对话 ID 系统升级——sessions source/parent_session_id + messages parent_message_id + 索引
			if _, err := s.db.Exec("ALTER TABLE sessions ADD COLUMN source TEXT NOT NULL DEFAULT 'desktop'"); err != nil {
				return fmt.Errorf("chat: 迁移 v3 source 失败: %w", err)
			}
			if _, err := s.db.Exec("ALTER TABLE sessions ADD COLUMN parent_session_id TEXT"); err != nil {
				return fmt.Errorf("chat: 迁移 v3 parent_session_id 失败: %w", err)
			}
			if _, err := s.db.Exec("ALTER TABLE messages ADD COLUMN parent_message_id INTEGER"); err != nil {
				return fmt.Errorf("chat: 迁移 v3 parent_message_id 失败: %w", err)
			}
			if _, err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_source ON sessions(source)"); err != nil {
				return fmt.Errorf("chat: 迁移 v3 source 索引失败: %w", err)
			}
			if _, err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions(parent_session_id)"); err != nil {
				return fmt.Errorf("chat: 迁移 v3 parent 索引失败: %w", err)
			}
		}
		_, err = s.db.Exec("UPDATE schema_version SET version = ?", schemaVersion)
		if err != nil {
			return fmt.Errorf("chat: 更新 schema 版本失败: %w", err)
		}
	}
	// 新列索引（迁移后创建——旧表先 ALTER 加列再建索引——P4-32）
	if _, err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_source ON sessions(source)"); err != nil {
		return fmt.Errorf("chat: 建 source 索引失败: %w", err)
	}
	if _, err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions(parent_session_id)"); err != nil {
		return fmt.Errorf("chat: 建 parent 索引失败: %w", err)
	}
	return nil
}

// Close — 关闭数据库
func (s *ChatStore) Close() error {
	return s.db.Close()
}

// now — 当前时间戳（秒）
func now() float64 {
	return float64(time.Now().UnixMilli()) / 1000.0
}
