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

// ZergApplicationID —— 虫族库的 `PRAGMA application_id` 自用身份标（设计稿 §三.4 附 G③ · §六 Q28）。
//
//	0x5A455247 = ASCII "ZERG"（Z=0x5A · E=0x45 · R=0x52 · G=0x47 · 大端序）。
//
// ★ 它落在**库头**（offset 68，官方 pragma.html）：随 `VACUUM INTO` 快照**带走** ——
// ✗ 不是表、✗ 不是 schema ⇒ **✗ 不为它写迁移件**（§六 Q28 逐字）；侧车（Q26）同步记它。
// 用途 = 开库体检时辨「这是不是虫族库」（防把别人的库当本库开）。
const ZergApplicationID uint32 = 0x5A455247

// readerMaxConns —— 读侧只读池的连接上限（§三.3 条目 1 · §六 Q16；照 hollis-labs OpenReader 默认 4）。
const readerMaxConns = 4

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

// DB —— 虫族库句柄（**双句柄**：写池 MaxOpenConns=1 ＋ 有界只读读池 readerMaxConns）。
//
// ★ 谁用写 / 谁用读（§三.3 条目 1 · §六 Q16）：
//   - **写路径**（迁移 / 态变更 / 幂等键 / 事件追加 / 身份标）一律走 SQL() ＝ 写池
//     （1 连接 ＋ `_txlock=immediate`）；
//   - **只读查询**走 Reader() ＝ 另开的只读池（`query_only=ON` · 上限 readerMaxConns）
//     ⇒ 读不再被写池串行化（WAL 该带来的读并发这才回得来）。
//
// ★ `database/sql` 没有「写连接数」旋钮：`SetMaxOpenConns(1)` 是**全池** 1 连接 ——
// 单池时读也被串行化，故读侧必须另开一池。
type DB struct {
	db   *sql.DB
	rdb  *sql.DB
	path string
}

// Path —— 库文件绝对路径。
func (d *DB) Path() string { return d.path }

// SQL —— **写池**句柄（MaxOpenConns=1 ＋ `_txlock=immediate`；语义与 T1 之前逐字同）。
func (d *DB) SQL() *sql.DB { return d.db }

// Reader —— **只读池**句柄（`query_only=ON` · 上限 readerMaxConns）。只读查询用它，✗ 别拿它写。
func (d *DB) Reader() *sql.DB { return d.rdb }

// Close —— 关库（**读 / 写两个 handle 一起关**；漏关一个就漏 fd · §六 Q16）。
func (d *DB) Close() error {
	if d == nil {
		return nil
	}
	var first error
	if d.rdb != nil {
		if err := d.rdb.Close(); err != nil {
			first = err
		}
	}
	if d.db != nil {
		if err := d.db.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// isFreshDBFile —— 「新建库」判据（§四.3 第 3 条 · 本件接线）：库文件**尚不存在**，或**存在但为空**
// （0 字节）⇒ 本次开库会**首建库头** ⇒ `auto_vacuum(2)` 此刻设才生效（见 dsn.go 的 BuildDSN）。
// 反之（已有库头 · size>0）⇒ **既有库** ⇒ 走 BuildExistingDSN，✗ 不为它改 auto_vacuum
// （「既有真库本批不动」—— 且实测：对已有库头的文件设 auto_vacuum 本就是静默空动作）。
func isFreshDBFile(dbPath string) bool {
	fi, err := os.Stat(dbPath)
	if err != nil {
		return true // 不存在（含父目录未建）⇒ 新建。
	}
	return fi.Size() == 0
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
	// ★ 「只在新建库施加 auto_vacuum」（§四.3 第 3 条 · 本件接线）：新建库（库文件不存在 / 为空）走
	// BuildDSN（带 `_pragma=auto_vacuum(2)` · 排在 journal_mode(WAL) **之前** · 建表前生效）；
	// 既有库走 BuildExistingDSN（不带该 pragma）⇒ **既有真库本批不动**（✗ 不对存量库强改 auto_vacuum）。
	fresh := isFreshDBFile(dbPath)
	dsn := BuildExistingDSN(dbPath)
	if fresh {
		dsn = BuildDSN(dbPath)
	}
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("zdb: 打开数据库失败: %w", err)
	}
	// 写池 MaxOpenConns=1：进程内把并发写串行化（冲突变成等连接，而非 SQLITE_BUSY）。
	raw.SetMaxOpenConns(1)
	raw.SetMaxIdleConns(1)

	d := &DB{db: raw, path: dbPath}
	// 硬规矩 ② 第一步：开库先体检 —— 三连：integrity_check ＋ foreign_key_check ＋ application_id
	// （§三.7 第 6 条 · §六 Q27 / Q28；起库体检与还原自证**共用** HealthCheck 这一入口）。
	if err := d.HealthCheck(); err != nil {
		raw.Close()
		return nil, err
	}
	// 硬规矩 ② 第二步：确认 WAL 生效（DSN 已写 _pragma=journal_mode(WAL)，此处回读核）。
	if err := d.ensureWAL(); err != nil {
		raw.Close()
		return nil, err
	}
	// ★ 新建库：确认 auto_vacuum 真为 INCREMENTAL（回读自证 · 与 ensureWAL 对称）。
	// 与 DSN 排序同属「只在新建库施加」那一支 —— **既有库不看不改**（态③：存量库 auto_vacuum 逐字不变）。
	// 反序 / 静默失效 ⇒ 回读 ≠ 2 ⇒ 此处 fail-closed（✗ 不放过「还盘链白做」）。
	if fresh {
		if err := d.ensureIncrementalAutoVacuum(); err != nil {
			raw.Close()
			return nil, err
		}
	}
	// 迁移（PRAGMA user_version · 幂等重跑）。
	if err := d.migrate(); err != nil {
		raw.Close()
		return nil, err
	}
	// 身份标：**建库时**打上（值为 0＝未打标 ⇒ 一次性写；已是本标 ⇒ 一字节不写 · §六 Q28）。
	if err := d.ensureApplicationID(); err != nil {
		raw.Close()
		return nil, err
	}
	// 硬规矩 ② 第三步：自管 checkpoint（✗ 不依赖默认 wal_autocheckpoint 放任 -wal 涨）。
	if err := d.Checkpoint(); err != nil {
		raw.Close()
		return nil, err
	}
	// 读侧另开有界只读 handle（写池已是 1 连接 ⇒ 读并发须另开池才回得来 · §六 Q16）。
	// ★ 放在**全部写路径收尾之后**开：读 handle 绝不看见迁移途中 / 未 checkpoint 的中间态。
	rdb, err := OpenReader(dbPath)
	if err != nil {
		raw.Close()
		return nil, err
	}
	d.rdb = rdb
	return d, nil
}

// OpenDefault —— 打开默认库（<仓>/.runtime/zerg.db）。
func OpenDefault() (*DB, error) { return Open(DefaultPath()) }

// integrityCheck —— 体检第 ① 条：PRAGMA integrity_check 须回**单行** ok。
// ★ 它**不查外键**（那是第 ② 条 foreignKeyCheck 的事 · §六 Q27 官方逐字）。
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

// ensureIncrementalAutoVacuum —— **新建库**的 `auto_vacuum` 回读自证（与 ensureWAL 对称 · §四.3 第 3 条）。
//
// ★ 只在**新建库**调（Open 里那支 `if fresh`）：DSN 里 `_pragma=auto_vacuum(2)` 若排错序
// （落到 journal_mode(WAL) **之后**）或没生效 ⇒ 回读 ≠ 2 ⇒ **fail-closed**（✗ 不静默放过 ——
// 放过 = 真还盘整条链白做）。既有库**不调**：存量库 auto_vacuum 恒 0，不为此报错、更不改。
func (d *DB) ensureIncrementalAutoVacuum() error {
	var av int
	if err := d.db.QueryRow("PRAGMA auto_vacuum").Scan(&av); err != nil {
		return fmt.Errorf("zdb: 读 auto_vacuum 失败: %w", err)
	}
	if av != 2 {
		return fmt.Errorf("zdb: 新建库 auto_vacuum=%d（期望 2=INCREMENTAL）—— DSN 里 `auto_vacuum(2)` 必须排在 `journal_mode(WAL)` **之前**（先开 WAL 再设 = 静默失效 · §四.3 第 3 条）", av)
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

// ─────────────────── 体检三连（§三.7 第 6 条 · §六 Q27 / Q28） ───────────────────

// HealthCheck —— **开库体检**（三连；任一条不过 ⇒ fail-closed，错误串**点名**是哪一条）：
//
//	① integrityCheck     —— `PRAGMA integrity_check` 单行 ok
//	② foreignKeyCheck    —— `PRAGMA foreign_key_check` **0 行**
//	③ applicationIDCheck —— `PRAGMA application_id` == ZergApplicationID（0＝未打标 ⇒ 放行）
//
// ★ ② 必须**单跑**：官方逐字「`PRAGMA integrity_check` **does not find FOREIGN KEY errors**.
// Use the `PRAGMA foreign_key_check` command」（sqlite.org/pragma.html#pragma_foreign_key_check）
// ⇒ 整库体检那把伞**不遮**外键 —— 这正是 §六 Q27 要从现读缺口补上的那一条。
// ★ 本入口＝**起库体检**（Open 调它）；**还原自证**（§四.2 丁 自证三件 · §三.7 第 6 条「迁移 / 还原后」）
// 复用**同一入口**，✗ 不另抄一份判据（免得两处口径漂开）。
func (d *DB) HealthCheck() error {
	if err := d.integrityCheck(); err != nil {
		return err
	}
	if err := d.foreignKeyCheck(); err != nil {
		return err
	}
	return d.applicationIDCheck()
}

// foreignKeyCheck —— 体检第 ② 条：`PRAGMA foreign_key_check` 须回 **0 行**。
//
// 任何一行 = 一处外键违例 ⇒ fail-closed 拒启。本仓**唯一入边 FK** =
// `task_events.task_id REFERENCES tasks(id)`（默认 NO ACTION）⇒ 违规会在迁移 / 还原后**静默留存**。
// ★ 它**不要求** `foreign_keys=ON`（官方行为：直接扫库）⇒ ✗ 别据「那连接 FK 关了」就跳过这一条。
func (d *DB) foreignKeyCheck() error {
	rows, err := d.db.Query("PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("%w：跑 foreign_key_check 失败: %v", ErrIntegrity, err)
	}
	defer rows.Close()
	n := 0
	first := ""
	for rows.Next() {
		// 结果四列 = (表名, rowid, 被引父表, fkid)；rowid / fkid 可能为 NULL ⇒ 用 Null* 接。
		var tbl, parent string
		var rowid, fkid sql.NullInt64
		if err := rows.Scan(&tbl, &rowid, &parent, &fkid); err != nil {
			return fmt.Errorf("%w：读 foreign_key_check 结果失败: %v", ErrIntegrity, err)
		}
		if n == 0 {
			first = fmt.Sprintf("表 %s rowid=%v 引用父表 %s (fkid=%v)", tbl, rowid, parent, fkid)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w：foreign_key_check 结果读取中断: %v", ErrIntegrity, err)
	}
	if n != 0 {
		return fmt.Errorf("%w：foreign_key_check 报 %d 行外键违例（首行：%s）—— 期望 0 行（§六 Q27）",
			ErrIntegrity, n, first)
	}
	return nil
}

// applicationIDCheck —— 体检第 ③ 条：`PRAGMA application_id` 身份标（§三.4 附 G③ · §六 Q28）。
//
// 三态（✗ 别把中间那态读成「非约定值一律拒」）：
//   - **== ZergApplicationID** ⇒ 过（我方库）；
//   - **== 0**（**未打标**）⇒ 放行 —— 两路子态都归这里：① 空库**首建途中**（schema 还没建）
//     ② 本特性**之前**建好的**旧库**（值从来没人写过）。打标**不放这里**：体检是**只读**面，
//     补标归写路径 ensureApplicationID（值非 0 ⇒ ✗ 不写）；
//   - **其它非 0 值** ⇒ **拒启** fail-closed —— 该库身上挂着**别人**的身份标 ⇒ 不是虫族库
//     （正是本标要防的「把 A5 库当 fleet_logs 库 / 本库误开」）。
func (d *DB) applicationIDCheck() error {
	var got int64
	if err := d.db.QueryRow("PRAGMA application_id").Scan(&got); err != nil {
		return fmt.Errorf("%w：读 application_id 失败: %v", ErrNotZergDB, err)
	}
	if got == 0 || got == int64(ZergApplicationID) {
		return nil
	}
	return fmt.Errorf("%w：application_id=%d（0x%X）≠ 本库约定 %d（0x%X＝\"ZERG\"）⇒ 拒启（§六 Q28）",
		ErrNotZergDB, got, got, int64(ZergApplicationID), int64(ZergApplicationID))
}

// ensureApplicationID —— **建库时**打身份标（写路径 · §六 Q28）。
//
// ★ 只在 `application_id == 0`（未打标）时写**一次** ⇒ ✗ 不是「每次开库无条件写」：
// 已挂本标 ⇒ 空动作（一字节不写）；挂着**别人**的非 0 标 ⇒ 拒启（与体检同口径，双保险）。
// ★ `application_id` 是**库头字段**（offset 68）：✗ 不是表、✗ 不是 schema ⇒ **✗ 不为它写迁移件**。
func (d *DB) ensureApplicationID() error {
	var got int64
	if err := d.db.QueryRow("PRAGMA application_id").Scan(&got); err != nil {
		return fmt.Errorf("zdb: 读 application_id 失败: %w", err)
	}
	switch {
	case got == int64(ZergApplicationID):
		return nil // 已打标（建库时写过 / 上次开库补过）⇒ 空动作
	case got != 0:
		return fmt.Errorf("%w：application_id=%d（0x%X）≠ 约定 %d（0x%X）⇒ 拒启",
			ErrNotZergDB, got, got, int64(ZergApplicationID), int64(ZergApplicationID))
	}
	// got == 0：未打标（空库首建 / 本特性之前的旧库）⇒ 一次性补标 ＋ 回读自证。
	if _, err := d.db.Exec(fmt.Sprintf("PRAGMA application_id = %d", int64(ZergApplicationID))); err != nil {
		return fmt.Errorf("zdb: 写 application_id 失败: %w", err)
	}
	var back int64
	if err := d.db.QueryRow("PRAGMA application_id").Scan(&back); err != nil {
		return fmt.Errorf("zdb: 回读 application_id 失败: %w", err)
	}
	if back != int64(ZergApplicationID) {
		return fmt.Errorf("zdb: application_id 写入未生效（回读 %d，期望 %d）", back, int64(ZergApplicationID))
	}
	return nil
}

// ─────────────────── 读侧只读 handle（§三.3 条目 1 · §六 Q16） ───────────────────

// OpenReader —— 另开一个有界**只读** handle（照 hollis-labs/go-sqlite 形态：写侧强制 1 连接 ＋
// `_txlock=immediate` · 读侧默认 4 连接、✗ 不带 `_txlock`）。
//
// ★ 只读档取 **`_pragma=query_only(ON)`**（✗ 不取 `mode=ro`），理由两条：
//   - 对齐 §三.3 条目 2：PRAGMA 是 **per-connection** 的，必须写进 DSN 的 `_pragma=` 才对新连接生效；
//     `mode=ro` 只能写进 URI，拿不到这层保证。
//   - `mode=ro` 在 WAL 库上要求 `-shm` 可写（否则 `SQLITE_READONLY_CANTINIT`）、且库文件不在时直接开不了；
//     `query_only=ON` 不引这些前置，同时在语句层封死写（真写 ⇒ `attempt to write a readonly database`）。
//
// ★ 只读池**不带 `_txlock`**（§六 Q16 逐字）：只读面不进写事务，IMMEDIATE 抢锁语义对它无意义。
func OpenReader(dbPath string) (*sql.DB, error) {
	rdb, err := sql.Open("sqlite", BuildReaderDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("zdb: 打开只读句柄失败: %w", err)
	}
	rdb.SetMaxOpenConns(readerMaxConns)
	rdb.SetMaxIdleConns(readerMaxConns)
	// 立刻试连一发：DSN 错 / 只读档不可用 ⇒ 在这里就露（fail-closed），✗ 不拖到第一条业务查询。
	if err := rdb.Ping(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("zdb: 只读句柄不可用: %w", err)
	}
	return rdb, nil
}
