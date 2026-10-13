package zdb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	_ "modernc.org/sqlite"
)

// DBFileName —— 库文件名（设计稿 §三.6 已拍：<仓>/.runtime/zerg.db）。
const DBFileName = "zerg.db"

// RuntimeDirName —— 仓内运行态目录（.gitignore 已登记 .runtime/）。
const RuntimeDirName = ".runtime"

// DefaultPath —— 库默认落点：<仓>/.runtime/zerg.db（ZERG_DB_PATH 可覆盖，供测试/第二实例隔离）。
func DefaultPath() string {
	if p := os.Getenv("ZERG_DB_PATH"); p != "" {
		return p
	}
	return filepath.Join(statepath.WorkspaceRoot(), RuntimeDirName, DBFileName)
}

// BackupDir —— 备份目录（硬规矩 ③：落**另一卷**（内盘）`~/.zerg/backup/`）。
func BackupDir() string {
	if d := os.Getenv("ZERG_BACKUP_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zerg", "backup")
}

// DB —— 虫族库句柄（写池 MaxOpenConns=1 ⇒ 进程内并发写串行化）。
type DB struct {
	db   *sql.DB
	path string
}

// Path —— 库文件绝对路径。
func (d *DB) Path() string { return d.path }

// SQL —— 暴露底层句柄（同包迁移/测试用）。
func (d *DB) SQL() *sql.DB { return d.db }

// Close —— 关库。
func (d *DB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

// Open —— 打开指定路径的库，逐条落三条硬规矩：
//
//	① 卷不在 ⇒ 拒启拒写（✗ 禁回退内盘新建空库）：readVolumeFacts + checkVolumeFacts。
//	② 开库先体检：integrity_check 须回单行 ok → 确认 WAL 生效 → 自管 checkpoint。
//	   （③ 备份落另一卷是备份动作的守卫，见 backup.go。）
//
// 之后跑迁移（PRAGMA user_version）。取不到 / 体检不过 / 版本过高 ⇒ 返回错误，调用方 fail-closed。
func Open(dbPath string) (*DB, error) {
	// 硬规矩 ①：卷不可用 ⇒ 立刻拒（在任何 mkdir / 建库之前）。
	facts, err := readVolumeFacts(dbPath)
	if err != nil {
		return nil, err
	}
	if err := checkVolumeFacts(facts, volumesRoot); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("zdb: 创建库目录失败: %w", err)
	}
	raw, err := sql.Open("sqlite", BuildDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("zdb: 打开数据库失败: %w", err)
	}
	// 写池 MaxOpenConns=1：进程内把并发写串行化（冲突变成等连接，而非 SQLITE_BUSY）。
	raw.SetMaxOpenConns(1)
	raw.SetMaxIdleConns(1)

	d := &DB{db: raw, path: dbPath}
	// 硬规矩 ② 第一步：开库先体检。
	if err := d.integrityCheck(); err != nil {
		raw.Close()
		return nil, err
	}
	// 硬规矩 ② 第二步：确认 WAL 生效（DSN 已写 _pragma=journal_mode(WAL)，此处回读核）。
	if err := d.ensureWAL(); err != nil {
		raw.Close()
		return nil, err
	}
	// 迁移（PRAGMA user_version · 幂等重跑）。
	if err := d.migrate(); err != nil {
		raw.Close()
		return nil, err
	}
	// 硬规矩 ② 第三步：自管 checkpoint（✗ 不依赖默认 wal_autocheckpoint 放任 -wal 涨）。
	if err := d.Checkpoint(); err != nil {
		raw.Close()
		return nil, err
	}
	return d, nil
}

// OpenDefault —— 打开默认库（<仓>/.runtime/zerg.db）。
func OpenDefault() (*DB, error) { return Open(DefaultPath()) }

// integrityCheck —— PRAGMA integrity_check 须回**单行** ok（✗ 不校验外键——那是 foreign_key_check 的事）。
func (d *DB) integrityCheck() error {
	rows, err := d.db.Query("PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("%w：跑 integrity_check 失败: %v", ErrIntegrity, err)
	}
	defer rows.Close()
	var msgs []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return fmt.Errorf("%w：读 integrity_check 结果失败: %v", ErrIntegrity, err)
		}
		msgs = append(msgs, s)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w：integrity_check 结果读取中断: %v", ErrIntegrity, err)
	}
	if len(msgs) != 1 || msgs[0] != "ok" {
		return fmt.Errorf("%w：integrity_check 未回单行 ok（实得 %v）", ErrIntegrity, msgs)
	}
	return nil
}

// ensureWAL —— 确认 journal_mode 真为 wal（DSN 的 _pragma 若被漏写，这里当场露）。
func (d *DB) ensureWAL() error {
	var mode string
	if err := d.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		return fmt.Errorf("zdb: 读 journal_mode 失败: %w", err)
	}
	if mode != "wal" {
		return fmt.Errorf("zdb: WAL 未生效（journal_mode=%q）—— 查 DSN 的 _pragma=journal_mode(WAL)", mode)
	}
	return nil
}

// Checkpoint —— 自管 checkpoint（PASSIVE；TRUNCATE 会把 -wal 截回零）。
func (d *DB) Checkpoint() error {
	if _, err := d.db.Exec("PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
		return fmt.Errorf("zdb: wal_checkpoint 失败: %w", err)
	}
	return nil
}

// CheckpointTruncate —— 截断式 checkpoint（-wal 归零；备份/收工前用）。
func (d *DB) CheckpointTruncate() error {
	if _, err := d.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("zdb: wal_checkpoint(TRUNCATE) 失败: %w", err)
	}
	return nil
}
