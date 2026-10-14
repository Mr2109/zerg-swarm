package zdb

import (
	"database/sql"
	"fmt"
	"regexp"
	"time"
)

// ───────────────────── T4 · 归档（按时间分表 ＋ 到期 DROP ＋ 空间回收） ─────────────────────
//
// 出处：设计稿 §四.3（第 1–4 条）· 附 L.1（逐字 SQL）· L.2（空间回收三场景实测）· L.3（不可分区六类）。
//
// 四条口径（逐字照 §四.3）：
//
//	① **✗ 不用范围 `DELETE` 做保留** —— 删大量行是 `O(rows × indexes)`、留 free page、碎片化文件；
//	   唯一例外 = 幂等表**行级** `DELETE FROM idempotency_keys WHERE expires_at < now`（走 idx_idem_expires）。
//	② 归档 = **按时间分表 ＋ 到期 `DROP TABLE IF EXISTS`**（毫秒级、O(1)）；表名逐字 `<域>_<表>_<YYYYMM>`
//	   （时间窗**写进表名** · ✗ 不靠 `WHERE`）；滚动 = 跨窗**建下窗表**（`CREATE TABLE IF NOT EXISTS` 照模板 ＋ 同名索引）。
//	③ ★★ **空间回收：`DROP TABLE` ≠ 还盘** —— 官方逐字：`DROP TABLE` 只把该表与其索引的页挂进 freelist，
//	   `auto_vacuum=0` 时**文件逐字节不变**（本件实测 12517376 B → 12517376 B）；真还盘须
//	   **建表前** `PRAGMA auto_vacuum = INCREMENTAL` ＋ 归档后 `PRAGMA incremental_vacuum`（实测 → 4096 B）。
//	④ 保留期 ＋ 自监控：逐表声明保留期（默认 90 天），并把库 / `-wal` 文件大小做成指标。
//
// ★★ **可分表判据**（§四.3 附 L.3）：纯追加 · 按时间查 · 无每实体单调序号 · 无对外 FK 被引 · 年增 >100 MB。
// 本仓**现读「可分区」集合为空**；六类**不可分区**逐条（✗ 别把它们拿去分表）：
//
//	① `tasks`                                  —— FK 父表（`task_events.task_id REFERENCES tasks(id)`）⇒ 分表后 FK 目标碎、重建表迁移更难；
//	② `task_events`                            —— 每任务单调无缺口 `audit_seq` ＋ `UNIQUE(task_id, audit_seq)` ⇒ 一任务跨月即断链；
//	③ `idempotency_keys`                       —— 需**单表全局唯一**才仲裁得住（`UNIQUE(slice_id, idempotency_key)` 跨分表失效）；
//	④ `gaps` / `proposals` / `update_receipts` —— 原地改状态 / 人审要 diff / 极小（26 件 · 104 KB）⇒ 单表；
//	⑤ 任何需跨时间窗 `join` 的表                —— 分表把 join 拆成 `UNION ALL` ＋ 多次 join、计划崩；
//	⑥ 小表（经验阈值）                         —— 年增**远小于**分表开销 ⇒ **年增 >100 MB 才分**（沿用 §十.4 反面情形① 口径）。
//
// ★ 顺序铁律（本件实测）：`auto_vacuum` 必须在 **`journal_mode(WAL)` 之前**、在任何表创建之前设 ——
// 先开 WAL 再设 ⇒ 读回 **0**（**静默失效**，不报错）⇒ 真还盘整条链白做（见 EnableIncrementalAutoVacuum）。

// windowStampLayout —— 时间窗写进表名的格式（`<YYYYMM>` · §四.3 第 2 条 / 附 L.1）。
const windowStampLayout = "200601"

// RetentionDefaultDays —— 逐表保留期默认建议（§四.3 第 4 条：默认建议 **90 天**）。
const RetentionDefaultDays = 90

// windowSegmentRE —— 表名段（域 / 表）的合法形态。DDL 串是**拼接**的 ⇒ 只放白名单段进来（✗ 不给注入面）。
var windowSegmentRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// windowTableRE —— 归档表名逐字形态 `<域>_<表>_<YYYYMM>`（§四.3 第 2 条）。
var windowTableRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*_[A-Za-z_][A-Za-z0-9_]*_[0-9]{6}$`)

// archiveExecer —— `*sql.DB` 与 `*sql.Tx` 共有的最小「执行 ＋ 查询」面（本件归档动作只经它）。
type archiveExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// WindowTableName —— 下窗表名 `<域>_<表>_<YYYYMM>`（§四.3 第 2 条 · 附 L.1）。
// 月窗按**传入时刻自身时区**取（调用方决定本地 / UTC）。
// ★ 段白名单校验：域 / 表只收 `[A-Za-z_][A-Za-z0-9_]*`；非法 ⇒ 拒（✗ 不放任外部串进 DDL）。
func WindowTableName(domain, table string, t time.Time) (string, error) {
	if !windowSegmentRE.MatchString(domain) {
		return "", fmt.Errorf("zdb: 归档表名「域」段非法（%q）—— 只许 [A-Za-z_][A-Za-z0-9_]*（§四.3 第 2 条）", domain)
	}
	if !windowSegmentRE.MatchString(table) {
		return "", fmt.Errorf("zdb: 归档表名「表」段非法（%q）—— 只许 [A-Za-z_][A-Za-z0-9_]*（§四.3 第 2 条）", table)
	}
	return domain + "_" + table + "_" + t.Format(windowStampLayout), nil
}

// WindowIndexName —— 下窗表的**同名索引**名（§四.3 第 2 条：滚动 = 建下窗表 ＋ 同名索引）。
func WindowIndexName(window string) string { return "idx_" + window + "_ts" }

// EnsureWindowTable —— 滚动：建下窗表（`CREATE TABLE IF NOT EXISTS` 照模板 ＋ 同名索引）· 附 L.1(a) 逐字。
// 返回落定的表名（判据面用的就是它）；重跑幂等（IF NOT EXISTS）。
func EnsureWindowTable(db archiveExecer, domain, table string, t time.Time) (string, error) {
	win, err := WindowTableName(domain, table, t)
	if err != nil {
		return "", err
	}
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS " + win + " (id INTEGER PRIMARY KEY, ts TEXT NOT NULL)"); err != nil {
		return "", fmt.Errorf("zdb: 建下窗表 %s 失败: %w", win, err)
	}
	idx := WindowIndexName(win)
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS " + idx + " ON " + win + "(ts)"); err != nil {
		return "", fmt.Errorf("zdb: 建下窗表同名索引 %s 失败: %w", idx, err)
	}
	return win, nil
}

// DropWindowTable —— 到期删除：`DROP TABLE IF EXISTS <下窗表>`（附 L.1(b)）。
//
// ★ 归档是**毫秒级、O(1)**（§四.3 第 2 条）—— ✗ 不用范围 DELETE（O(rows × indexes) ＋ 碎片化）。
// ★ 但 `DROP TABLE` **只把页挂进 freelist**（✗ 不还盘）⇒ 文件真缩见 ReclaimSpace（两步缺一不可）。
func DropWindowTable(db archiveExecer, domain, table string, t time.Time) (string, error) {
	win, err := WindowTableName(domain, table, t)
	if err != nil {
		return "", err
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS " + win); err != nil {
		return "", fmt.Errorf("zdb: 到期删表 %s 失败: %w", win, err)
	}
	return win, nil
}

// ReclaimSpace —— ★ 归档后**真还盘**的那一步：`PRAGMA incremental_vacuum`（§四.3 第 3 条 · 附 L.1(c) / L.2）。
//
// ★ 前置 = 建表前已设 `auto_vacuum=INCREMENTAL`（见 EnableIncrementalAutoVacuum）⇒ ✗ 单靠这一句、在
// `auto_vacuum=0` 的库上跑 = **空动作**（文件不缩）。两句是一对，✗ 别只落一句。
func ReclaimSpace(db archiveExecer) error {
	if _, err := db.Exec("PRAGMA incremental_vacuum"); err != nil {
		return fmt.Errorf("zdb: incremental_vacuum 失败: %w", err)
	}
	return nil
}

// EnableIncrementalAutoVacuum —— ★★ 硬前提：`PRAGMA auto_vacuum = INCREMENTAL`（=2）**必须在任何表创建之前**设
// （官方逐字「It is not possible to enable or disable auto-vacuum after a table has been created.」· §四.3 第 3 条）。
//
// 先**设**再**回读自证**（set-first：对全新空库第一条语句就生效；对既有库是 no-op）。返回 (applied, error) 四态：
//
//	① 回读 == 2（设成 / 本来就 2）      ⇒ (true, nil)  —— 幂等；
//	② 回读 ≠ 2 且库内**已有用户表**      ⇒ (false, nil) —— 按 §四.3「既有真库本批不动」如实跳过（✗ 不 VACUUM 强改）；
//	③ 回读 ≠ 2 且库内**无用户表**        ⇒ (false, err) —— **静默失效**（典型 = 连接已先开 `journal_mode(WAL)`）·
//	   **fail-closed**，✗ 不静默放过（放过 = 还盘链整条白做）。
//
// ★ 调用时机（本件实测的铁律）：必须在**开 WAL 之前、建表之前**。正解次序见同包测试
// TestEnableIncrementalAutoVacuum_DSNPragmaOrdering（DSN 里把 `auto_vacuum(2)` 排在 `journal_mode(WAL)` 之前）。
func EnableIncrementalAutoVacuum(db archiveExecer) (bool, error) {
	// ① 先设（set-first）：全新空库第一条语句即生效；既有库由 SQLite 自己无视。
	if _, err := db.Exec("PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
		return false, fmt.Errorf("zdb: 设 auto_vacuum=INCREMENTAL 失败: %w", err)
	}
	var back int
	if err := db.QueryRow("PRAGMA auto_vacuum").Scan(&back); err != nil {
		return false, fmt.Errorf("zdb: 回读 auto_vacuum 失败: %w", err)
	}
	if back == 2 {
		return true, nil
	}
	n, err := countUserTables(db)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	return false, fmt.Errorf("zdb: auto_vacuum 设了未生效（回读 %d，期望 2）—— 典型成因：连接已先开 journal_mode(WAL)；"+
		"auto_vacuum 必须在开 WAL 之前、建表之前设（§四.3 第 3 条）", back)
}

// countUserTables —— 库内**用户表**数（✗ 不含 sqlite_ 内建表 · ✗ 不含视图 / 索引 / 触发器）。
func countUserTables(db archiveExecer) (int, error) {
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&n); err != nil {
		return 0, fmt.Errorf("zdb: 数用户表失败: %w", err)
	}
	return n, nil
}

// ListWindowTables —— 列库内**归档窗表**名（判据形照任务单 T4②：`name GLOB '*_[0-9]…'` ＋ 本件表名形态双验）。
// 只报**表**（✗ 不含索引 / 视图）；无窗表 ⇒ 空表（✗ 不是错）。
func ListWindowTables(db archiveExecer) ([]string, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name GLOB '*_[0-9][0-9][0-9][0-9][0-9][0-9]' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("zdb: 列归档窗表失败: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("zdb: 读归档窗表名失败: %w", err)
		}
		if windowTableRE.MatchString(name) {
			out = append(out, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("zdb: 归档窗表名读取中断: %w", err)
	}
	return out, nil
}
