// chatlog.go — 会话存储（波4 第一片 · 会话存储）
//
// 设计真源：Zerg-内部文档/01-设计/设计-虫族子代理-20261014.md §十.1 / §十.2 ①
//
//	· 对话＝历史（append-only · 只增不改）—— 回答「说过什么」
//	· append-only JSONL 一文件一会话当**权威**（只增不改；老件 zstd 归档）
//	· SQLite + FTS5 外部内容表只当**索引 / 查询面**（✗ 不当第二份权威 · ✗ 不用向量库当首选项）
//	· 落点 ~/.zerg/state/chat/<session_id>.jsonl
//	· 分页 keyset（(session_id, seq) 索引）
//	· 大内容外置（超大工具输出落盘只存 blob_ref + sha256；✗ 正文不进列）
//
// ★ 一条铁律：真源 = JSONL（人可读 · 可归档 · 追加即用）；索引 = SQLite/FTS5（可丢 · 可重建）。
//
//	索引损坏 / 缺失**不许**影响真源读：ReadSession/PageAfter 只走 JSONL；
//	Search/PageIndex 走索引面，索引不可用时明报 ErrIndexUnavailable（✗ 不静默返空）。
package chatlog

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	_ "modernc.org/sqlite"
)

const (
	// DirName — 会话存储目录名（落点 = <ZERG_STATE_DIR>/chat/）。
	DirName = "chat"
	// BlobsDirName — 大内容外置目录（内容寻址：blobs/<sha256>.bin）。
	BlobsDirName = "blobs"
	// IndexDBName — FTS5 索引库文件名（✗ 不是真源）。
	IndexDBName = "index.db"
	// JSONLSuffix — 会话权威档后缀（一文件一会话）。
	JSONLSuffix = ".jsonl"
	// DefaultMaxInlineBytes — 正文内联上限（超过即外置：正文落盘、行里只留 blob_ref + sha256）。
	DefaultMaxInlineBytes = 8 * 1024
)

// 哨兵错误（调用方按错误类分档，✗ 不解析文案）。
var (
	// ErrNoSession —— 该会话权威档不在盘上。
	ErrNoSession = errors.New("chatlog: 会话不存在")
	// ErrIndexUnavailable —— 索引面不可用（损坏 / 缺失）——★ 只影响检索，不影响真源读。
	ErrIndexUnavailable = errors.New("chatlog: 索引面不可用（可 RebuildIndex 重建）")
	// ErrBlobCorrupt —— 外置内容读回后 sha256 与登记的指纹不符。
	ErrBlobCorrupt = errors.New("chatlog: 外置内容指纹不符（sha256 校验失败）")
)

// Usage —— 一次模型调用的用量（可选；设计稿 §二.3 事件字段含 usage）。
type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// Event —— 一行 = 一个会话事件（append-only）。字段照设计稿 §二.3：
// type / uuid / parent / timestamp / session_id / cwd / usage，另加 seq（keyset 分页的键）。
// ★ 大内容：Content 为空、BlobRef + SHA256 给指针与指纹（正文落盘，✗ 不进列）。
type Event struct {
	Seq       int64   `json:"seq"`  // 会话内单调递增（keyset 分页的键）
	UUID      string  `json:"uuid"` //
	Parent    string  `json:"parent,omitempty"`
	Type      string  `json:"type"`           // message / tool_call / tool_result / …
	Role      string  `json:"role,omitempty"` // user / assistant / tool / system
	SessionID string  `json:"session_id"`     //
	CWD       string  `json:"cwd,omitempty"`  //
	TS        float64 `json:"timestamp"`      //
	Content   string  `json:"content,omitempty"`
	BlobRef   string  `json:"blob_ref,omitempty"` // 外置内容的指针（相对存储目录）
	SHA256    string  `json:"sha256,omitempty"`   // 外置内容的指纹（读回必核）
	Usage     *Usage  `json:"usage,omitempty"`    //
}

// DefaultDir —— 会话存储默认落点：<ZERG_STATE_DIR|~/.zerg/state>/chat。
func DefaultDir() string { return filepath.Join(statepath.Dir(), DirName) }

// Store —— 会话存储句柄（真源 = 目录里的 JSONL；索引 = 目录里的 SQLite/FTS5）。
type Store struct {
	dir            string
	indexPath      string
	maxInlineBytes int

	mu     sync.Mutex       // 单写者：追加 + seq 分配在锁内
	seq    map[string]int64 // session_id → 最后分配的 seq（进程内缓存；单写者前提）
	idx    *sql.DB          // nil = 索引不可用（✗ 不影响真源）
	idxErr error            // 索引不可用的原因（idx == nil 时非 nil）
}

// OpenOptions —— Open 的可选项。
type OpenOptions struct {
	// MaxInlineBytes —— 正文内联上限；<=0 ⇒ 用 DefaultMaxInlineBytes。
	MaxInlineBytes int
}

// Open —— 打开 / 初始化某目录下的会话存储（真源目录与索引库都在 dir 里）。
// ★ 索引失败**不阻断**：真源（JSONL）始终可用；索引状态经 IndexStatus 查。
func Open(dir string, opt OpenOptions) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("chatlog: 存储目录不能为空")
	}
	maxInline := opt.MaxInlineBytes
	if maxInline <= 0 {
		maxInline = DefaultMaxInlineBytes
	}
	s := &Store{
		dir:            dir,
		indexPath:      filepath.Join(dir, IndexDBName),
		maxInlineBytes: maxInline,
		seq:            map[string]int64{},
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("chatlog: 创建存储目录失败: %w", err)
	}
	s.openIndex() // 失败只记 idxErr，不返回（真源不依赖索引）
	return s, nil
}

// OpenDefault —— 打开默认落点（<ZERG_STATE_DIR>/chat）。
func OpenDefault() (*Store, error) { return Open(DefaultDir(), OpenOptions{}) }

// Dir —— 存储目录。
func (s *Store) Dir() string { return s.dir }

// IndexPath —— 索引库路径。
func (s *Store) IndexPath() string { return s.indexPath }

// Close —— 关索引库（真源是文件，无需关闭）。
func (s *Store) Close() error {
	if s.idx != nil {
		err := s.idx.Close()
		s.idx = nil
		return err
	}
	return nil
}

// IndexStatus —— 索引面现读：ok=false 时 err 给原因。
func (s *Store) IndexStatus() (bool, error) {
	if s.idx == nil {
		if s.idxErr == nil {
			return false, ErrIndexUnavailable
		}
		return false, s.idxErr
	}
	return true, nil
}

// nowUnix —— 秒级 float 时间戳（与既有 store 同口径 · UnixMilli/1000）。
func nowUnix() float64 { return float64(time.Now().UnixMilli()) / 1000.0 }

// newUUID —— v4 UUID 字符串（自带实现，✗ 不引新直接依赖）。
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// 极度罕见（熵源不可用）：退回「时间 + pid」形态，仍保证非空。
		return fmt.Sprintf("%016x-%04x", time.Now().UnixNano(), os.Getpid())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}
