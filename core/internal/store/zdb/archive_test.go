package zdb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── T4 · 调度 / 归档 · 定向判据（§四.3 第 1–4 条 · 附 L.1 / L.2 / L.3）──
//
// 两态铁律（逐格贴字节）：
//	① `auto_vacuum=INCREMENTAL` 的新库：建表 → 填数据 → `DROP TABLE` → `incremental_vacuum` ⇒ **文件真变小**；
//	② 负控：把 `incremental_vacuum` 那句**哑化** ⇒ 文件**逐字节不变**（白证「DROP 不还盘」）。
// 两态共用同一夹具（archiveOpenFresh ＋ archiveFillWindow），只差那一步。

const (
	archiveDomain = "fleet"
	archiveTable  = "logs"
	archiveRows   = 200000
)

// archiveWindow —— 判据用的固定时间窗（2026-10）。
func archiveWindow() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }

// archiveFileSize —— 库**主件**字节数（✗ 不含 -wal / -shm ⇒ 测量前必先 checkpoint(TRUNCATE)）。
func archiveFileSize(t *testing.T, path string) int64 {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat 库件 %s 失败: %v", path, err)
	}
	return st.Size()
}

// archiveCheckpoint —— 把 -wal 里已提交页并回主库并截断（否则文件大小读数会被 -wal 搅乱）。
func archiveCheckpoint(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatalf("wal_checkpoint(TRUNCATE) 失败: %v", err)
	}
}

// archiveOpenFresh —— 开一枚**全新库**，并逐条落「开库路径正解次序」：
//
//	① 先以**不带 WAL** 的裸 DSN 开 ⇒ 设 `auto_vacuum=INCREMENTAL`（此时库内零表）⇒ 回读自证 =2；
//	② 再用**写侧 DSN**（WAL）开（值随库头带走，回读仍 =2）。
//
// ★ 这个次序正是 T4③ 要落进「开库路径」的次序 —— 先开 WAL 再设 ⇒ **静默失效**（同包另一格负控证之）。
func archiveOpenFresh(t *testing.T, path string) *sql.DB {
	t.Helper()
	pre, err := sql.Open("sqlite", "file:"+dsnEscapePath(path)+"?mode=rwc&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("裸开新库失败: %v", err)
	}
	applied, err := EnableIncrementalAutoVacuum(pre)
	if err != nil {
		pre.Close()
		t.Fatalf("全新库设 auto_vacuum=INCREMENTAL 失败: %v", err)
	}
	if !applied {
		pre.Close()
		t.Fatalf("全新库应能设 auto_vacuum=INCREMENTAL（实得未生效）")
	}
	if err := pre.Close(); err != nil {
		t.Fatalf("关裸连接失败: %v", err)
	}
	db, err := sql.Open("sqlite", BuildDSN(path))
	if err != nil {
		t.Fatalf("用写侧 DSN 开库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = db.Close() })
	var av int
	if err := db.QueryRow("PRAGMA auto_vacuum").Scan(&av); err != nil {
		t.Fatalf("回读 auto_vacuum 失败: %v", err)
	}
	if av != 2 {
		t.Fatalf("WAL 开库后 auto_vacuum=%d（期望 2）—— 该值没随库头带走", av)
	}
	return db
}

// archiveFillWindow —— 建下窗表（走 EnsureWindowTable，✗ 不手写 DDL）＋ 灌 archiveRows 行，返回表名。
func archiveFillWindow(t *testing.T, db *sql.DB) string {
	t.Helper()
	win, err := EnsureWindowTable(db, archiveDomain, archiveTable, archiveWindow())
	if err != nil {
		t.Fatalf("建下窗表失败: %v", err)
	}
	q := fmt.Sprintf("INSERT INTO %s(ts) WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x<%d) SELECT '2026-10-01T00:00:00Z' FROM c", win, archiveRows)
	if _, err := db.Exec(q); err != nil {
		t.Fatalf("灌 %d 行进下窗表失败: %v", archiveRows, err)
	}
	var n int
	if err := db.QueryRow("SELECT count(*) FROM " + win).Scan(&n); err != nil {
		t.Fatalf("数下窗表行失败: %v", err)
	}
	if n != archiveRows {
		t.Fatalf("下窗表应有 %d 行，实得 %d", archiveRows, n)
	}
	return win
}

// archiveTableExists —— 该表是否还在库中（表名判据面）。
func archiveTableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

// TestWindowTableName_Shape —— 表名逐字 `<域>_<表>_<YYYYMM>`；同名索引名同源；段白名单两态（✗ 注入面）。
func TestWindowTableName_Shape(t *testing.T) {
	got, err := WindowTableName("fleet", "logs", time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("合法段竟被拒: %v", err)
	}
	if got != "fleet_logs_202610" {
		t.Fatalf("表名应为 fleet_logs_202610，实得 %q", got)
	}
	if idx := WindowIndexName(got); idx != "idx_fleet_logs_202610_ts" {
		t.Fatalf("同名索引名应为 idx_fleet_logs_202610_ts，实得 %q", idx)
	}
	// 段白名单两态：注入形态 / 空段 / 破形段一律拒。
	for _, bad := range [][2]string{
		{"fleet; DROP TABLE tasks", "logs"},
		{"fleet", "logs); DROP TABLE tasks;--"},
		{"", "logs"},
		{"fleet", ""},
		{"fleet-log", "logs"},
		{"1fleet", "logs"},
	} {
		if n, err := WindowTableName(bad[0], bad[1], archiveWindow()); err == nil {
			t.Errorf("非法段 (%q,%q) 竟被收下（表名 %q）—— 白名单没生效", bad[0], bad[1], n)
		}
	}
}

// TestEnsureWindowTable_CreatesTableAndIndex —— 下窗表 ＋ 同名索引到齐；重跑幂等；判据形 GLOB 命中。
func TestEnsureWindowTable_CreatesTableAndIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zerg.db")
	db := archiveOpenFresh(t, path)
	win := archiveFillWindow(t, db)
	archiveCheckpoint(t, db)

	if !archiveTableExists(t, db, win) {
		t.Fatalf("下窗表 %s 不在库中", win)
	}
	idx := WindowIndexName(win)
	var n int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("同名索引 %s 不在库中（命中 %d）", idx, n)
	}
	// 判据形（任务单 T4②）：GLOB 收得上该窗表，且表名形态过关。
	got, err := ListWindowTables(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != win {
		t.Fatalf("ListWindowTables 应恰含 [%s]，实得 %v", win, got)
	}
	// 幂等重跑：再建一次不报错、回同表名、行数不变。
	if again, err := EnsureWindowTable(db, archiveDomain, archiveTable, archiveWindow()); err != nil || again != win {
		t.Fatalf("EnsureWindowTable 重跑应幂等回同表名，实得 (%q,%v)", again, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM " + win).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != archiveRows {
		t.Fatalf("重跑后行数应仍 %d，实得 %d", archiveRows, n)
	}
}

// TestArchiveDropAndReclaim_ShrinksFile —— ★ 必做的那一态：INCREMENTAL 新库上
// 建表 → 填数据 → `DROP TABLE` → `incremental_vacuum` ⇒ **文件真变小**（逐格贴前后字节）。
func TestArchiveDropAndReclaim_ShrinksFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zerg.db")
	db := archiveOpenFresh(t, path)
	win := archiveFillWindow(t, db)
	archiveCheckpoint(t, db)
	before := archiveFileSize(t, path)

	// 到期删除（✗ 范围 DELETE）：DROP TABLE IF EXISTS。
	dropped, err := DropWindowTable(db, archiveDomain, archiveTable, archiveWindow())
	if err != nil {
		t.Fatalf("到期删表失败: %v", err)
	}
	if dropped != win {
		t.Fatalf("删错表：应 %s，实得 %s", win, dropped)
	}
	archiveCheckpoint(t, db)
	afterDrop := archiveFileSize(t, path)

	// 真还盘：PRAGMA incremental_vacuum。
	if err := ReclaimSpace(db); err != nil {
		t.Fatalf("incremental_vacuum 失败: %v", err)
	}
	archiveCheckpoint(t, db)
	afterReclaim := archiveFileSize(t, path)

	t.Logf("T4 空间回收读数：填后 %d B → DROP 后 %d B → incremental_vacuum 后 %d B（真缩 %d B）",
		before, afterDrop, afterReclaim, before-afterReclaim)
	if afterReclaim >= before {
		t.Fatalf("incremental_vacuum 后文件未变小（before=%d after=%d）—— 还盘链没生效（先查 PRAGMA auto_vacuum 是不是 2）", before, afterReclaim)
	}
	if afterReclaim >= afterDrop {
		t.Fatalf("incremental_vacuum 没起作用（after_drop=%d after_reclaim=%d）", afterDrop, afterReclaim)
	}
	if archiveTableExists(t, db, win) {
		t.Fatalf("DROP 后表 %s 应已离库", win)
	}
}

// TestArchiveNegativeControl_NoReclaimKeepsFileSize —— ★ 负控：把 `incremental_vacuum` 那句**哑化**
// ⇒ 文件**逐字节不变**（白证「DROP TABLE ≠ 还盘」）。
func TestArchiveNegativeControl_NoReclaimKeepsFileSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zerg.db")
	db := archiveOpenFresh(t, path)
	win := archiveFillWindow(t, db)
	archiveCheckpoint(t, db)
	before := archiveFileSize(t, path)

	if _, err := DropWindowTable(db, archiveDomain, archiveTable, archiveWindow()); err != nil {
		t.Fatalf("到期删表失败: %v", err)
	}
	// ★ 负控：**不跑** ReclaimSpace（= 把 incremental_vacuum 那句哑化）。
	archiveCheckpoint(t, db)
	afterDrop := archiveFileSize(t, path)

	t.Logf("T4 负控读数（无 incremental_vacuum）：填后 %d B → DROP 后 %d B（差 %d B）", before, afterDrop, afterDrop-before)
	if afterDrop != before {
		t.Fatalf("负控要求「DROP 不还盘」= 文件逐字节不变，实得 %d → %d", before, afterDrop)
	}
	// 表的确已离库（✗ 别把「没删」当成「没缩」）。
	if archiveTableExists(t, db, win) {
		t.Fatalf("DROP 后表 %s 应已离库 —— 否则负控证的不是「DROP 不还盘」", win)
	}
}

// TestEnableIncrementalAutoVacuum_States —— 三态：全新库真生效 / 幂等 / 既有库如实跳过（✗ 不动既有库）/
// 先开 WAL 再设 ⇒ fail-closed（静默失效被抓住）。
func TestEnableIncrementalAutoVacuum_States(t *testing.T) {
	// ① + ② 全新空库 ⇒ 真生效；再来一次 ⇒ 幂等仍 (true,nil)。
	p1 := filepath.Join(t.TempDir(), "fresh.db")
	h1, err := sql.Open("sqlite", "file:"+dsnEscapePath(p1)+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	if applied, err := EnableIncrementalAutoVacuum(h1); err != nil || !applied {
		t.Fatalf("全新库应 (true,nil)，实得 (%v,%v)", applied, err)
	}
	if applied, err := EnableIncrementalAutoVacuum(h1); err != nil || !applied {
		t.Fatalf("幂等重设应 (true,nil)，实得 (%v,%v)", applied, err)
	}
	h1.Close()

	// ③ 既有库（**先建表**、auto_vacuum 仍 0）⇒ (false,nil) 如实跳过，且值保持 0（✗ 不动既有库）。
	p2 := filepath.Join(t.TempDir(), "existing.db")
	h2, err := sql.Open("sqlite", "file:"+dsnEscapePath(p2)+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h2.Exec("CREATE TABLE t(a INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if applied, err := EnableIncrementalAutoVacuum(h2); err != nil || applied {
		t.Fatalf("既有库（已有表）应 (false,nil) 如实跳过，实得 (%v,%v)", applied, err)
	}
	var av2 int
	if err := h2.QueryRow("PRAGMA auto_vacuum").Scan(&av2); err != nil {
		t.Fatal(err)
	}
	if av2 != 0 {
		t.Fatalf("既有库不该被改：auto_vacuum=%d（期望 0 · 建表后按官方不可改）", av2)
	}
	h2.Close()

	// ④ 顺序错（先开 WAL 再设）⇒ fail-closed（静默失效被抓住，✗ 不放过）。
	p3 := filepath.Join(t.TempDir(), "wal-first.db")
	h3, err := sql.Open("sqlite", "file:"+dsnEscapePath(p3)+"?mode=rwc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h3.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	if applied, err := EnableIncrementalAutoVacuum(h3); err == nil {
		t.Fatalf("先开 WAL 再设 auto_vacuum 应 fail-closed（实得 applied=%v err=nil）—— 静默失效没被抓住", applied)
	}
	h3.Close()
}

// TestEnableIncrementalAutoVacuum_DSNPragmaOrdering —— ★ 证明「开库路径正解」：把 `auto_vacuum(2)` 排进
// DSN 的 `_pragma` **最前**（在 `journal_mode(WAL)` 之前）⇒ **一次开库即成 INCREMENTAL**。
// 这条正是 T4③ 要落进开库路径的那一处（本单因该件被锁，只在本件把它证成）。
func TestEnableIncrementalAutoVacuum_DSNPragmaOrdering(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zerg.db")
	dsn := "file:" + dsnEscapePath(p) +
		"?_pragma=auto_vacuum(2)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("开库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	var av int
	if err := db.QueryRow("PRAGMA auto_vacuum").Scan(&av); err != nil {
		t.Fatalf("读 auto_vacuum 失败: %v", err)
	}
	var jm string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&jm); err != nil {
		t.Fatalf("读 journal_mode 失败: %v", err)
	}
	t.Logf("DSN 顺序（auto_vacuum(2) 先于 journal_mode(WAL)）⇒ auto_vacuum=%d journal_mode=%s", av, jm)
	if jm != "wal" {
		t.Fatalf("journal_mode 应为 wal，实得 %q", jm)
	}
	if av != 2 {
		t.Fatalf("DSN 里 auto_vacuum 排在 WAL 之前应使其一次开库即成 2，实得 %d ⇒ 该修法不成立，开库路径须改走「先裸开设值、再开 WAL」两段式", av)
	}
}
