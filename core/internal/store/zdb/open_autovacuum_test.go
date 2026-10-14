package zdb

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// ─────────────── T4c · auto_vacuum 开库路径接线（四态真跑） ───────────────
//
// 接线面：`Open()` 对**新建库**（库文件不存在 / 为空 · 见 isFreshDBFile）走 BuildDSN
// （把 `auto_vacuum(2)` 排在 `journal_mode(WAL)` **之前**）＋ 回读自证 ensureIncrementalAutoVacuum；
// 对**既有库**走 BuildExistingDSN（不带该 pragma）⇒ 存量库行为逐字不变。
//
// 四态：① 新建空库走开库路径 ⇒ auto_vacuum=2 ＋ journal_mode=wal；
//       ② 新建库建表→填数→DROP→incremental_vacuum ⇒ **文件真缩**（贴前后字节）；
//       ③ 既有库（auto_vacuum=0）走**同一条开库路径** ⇒ 行为逐字不变（回读仍 0）；
//       ④ 库文件不存在 ⇒ 建库路径不下报错。

// avReadBack —— 经**独立裸连接**回读 auto_vacuum / journal_mode（✗ 不借写池 —— 免得看到的是
// 写池那条连接的 per-connection 态）。query 形态只读，✗ 不改库。
func avReadBack(t *testing.T, path string) (autoVacuum int, journalMode string) {
	t.Helper()
	raw, err := sql.Open("sqlite", "file:"+dsnEscapePath(path)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("裸开 %q 失败: %v", path, err)
	}
	defer raw.Close()
	if err := raw.QueryRow("PRAGMA auto_vacuum").Scan(&autoVacuum); err != nil {
		t.Fatalf("读 auto_vacuum 失败: %v", err)
	}
	if err := raw.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("读 journal_mode 失败: %v", err)
	}
	return autoVacuum, journalMode
}

// avFileSize —— 库主文件字节数（✗ 不含 -wal / -shm 侧车）。
func avFileSize(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q 失败: %v", path, err)
	}
	return fi.Size()
}

// TestOpenAutoVacuum_NewDBIncrementalWAL —— 态①：新建空库走开库路径 ⇒ **一次开库**即成
// auto_vacuum=2（INCREMENTAL）＋ journal_mode=wal。
func TestOpenAutoVacuum_NewDBIncrementalWAL(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zerg.db")
	if _, err := os.Stat(p); err == nil {
		t.Fatalf("前置：新建库文件本不该存在（%s）", p)
	}
	db, err := Open(p)
	if err != nil {
		t.Fatalf("新建库开库失败: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}
	av, jm := avReadBack(t, p)
	t.Logf("态① 新建库：auto_vacuum=%d journal_mode=%s", av, jm)
	if av != 2 {
		t.Fatalf("新建库 auto_vacuum=%d（期望 2=INCREMENTAL）—— 开库路径没把 auto_vacuum(2) 排在 WAL 之前 / 没生效", av)
	}
	if jm != "wal" {
		t.Fatalf("新建库 journal_mode=%q（期望 wal）", jm)
	}
}

// TestOpenAutoVacuum_NewDBReclaimShrinksFile —— 态②：INCREMENTAL 新库上建表 → 填数 →
// `DROP TABLE` → `PRAGMA incremental_vacuum` ⇒ **文件真缩**（逐格贴前后字节）。
func TestOpenAutoVacuum_NewDBReclaimShrinksFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zerg.db")
	db, err := Open(p)
	if err != nil {
		t.Fatalf("开库失败: %v", err)
	}
	defer db.Close()
	w := db.SQL()
	if _, err := w.Exec("CREATE TABLE av_blob(id INTEGER PRIMARY KEY, b BLOB)"); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	ins := "INSERT INTO av_blob(b) WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x<2000) SELECT randomblob(4000) FROM c"
	if _, err := w.Exec(ins); err != nil {
		t.Fatalf("填数失败: %v", err)
	}
	if err := db.CheckpointTruncate(); err != nil {
		t.Fatalf("checkpoint(TRUNCATE) 失败: %v", err)
	}
	before := avFileSize(t, p)
	if _, err := w.Exec("DROP TABLE av_blob"); err != nil {
		t.Fatalf("DROP TABLE 失败: %v", err)
	}
	if err := ReclaimSpace(w); err != nil {
		t.Fatalf("incremental_vacuum 失败: %v", err)
	}
	if err := db.CheckpointTruncate(); err != nil {
		t.Fatalf("checkpoint(TRUNCATE) 2 失败: %v", err)
	}
	after := avFileSize(t, p)
	t.Logf("态② 空间回收：填后 %d B → DROP+incremental_vacuum 后 %d B（真缩 %d B）", before, after, before-after)
	if after >= before {
		t.Fatalf("incremental_vacuum 后文件未变小（before=%d after=%d）—— 还盘链没生效（先查 auto_vacuum 是不是 2）", before, after)
	}
}

// TestOpenAutoVacuum_ExistingDBUnchanged —— 态③：既有库（auto_vacuum=0）走**同一条开库路径**
// ⇒ 行为逐字不变：回读仍 0（✗ 不被强改），且既有数据仍在。
func TestOpenAutoVacuum_ExistingDBUnchanged(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zerg.db")
	// 造「既有库」：老形态（不带 auto_vacuum 的裸连接）建库 → 建表 → 填数 ⇒ 库头 auto_vacuum 恒 0。
	seed, err := sql.Open("sqlite", "file:"+dsnEscapePath(p)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("造既有库失败: %v", err)
	}
	if _, err := seed.Exec("CREATE TABLE legacy_marker(x INTEGER); INSERT INTO legacy_marker VALUES(1);"); err != nil {
		seed.Close()
		t.Fatalf("造既有表/行失败: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("关建库连接失败: %v", err)
	}
	avBefore, _ := avReadBack(t, p)
	if avBefore != 0 {
		t.Fatalf("夹具前提不成立：既有库 auto_vacuum 应为 0，实得 %d", avBefore)
	}
	db, err := Open(p)
	if err != nil {
		t.Fatalf("既有库应能开（行为逐字不变），实得错: %v", err)
	}
	var n int
	if err := db.Reader().QueryRow("SELECT count(*) FROM legacy_marker").Scan(&n); err != nil {
		t.Fatalf("读既有表失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("既有数据应仍在（legacy_marker 应 1 行），实得 %d", n)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}
	avAfter, jm := avReadBack(t, p)
	t.Logf("态③ 既有库：auto_vacuum 前=%d 后=%d journal_mode=%s", avBefore, avAfter, jm)
	if avAfter != 0 {
		t.Fatalf("既有库被强改：auto_vacuum=%d（期望仍 0 · §四.3「既有真库本批不动」）", avAfter)
	}
}

// TestOpenAutoVacuum_MissingFileCreates —— 态④：库文件不存在（父目录也缺）⇒ 建库路径不下报错。
func TestOpenAutoVacuum_MissingFileCreates(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "deeper", "zerg.db")
	if _, err := os.Stat(filepath.Dir(p)); err == nil {
		t.Fatalf("前置：父目录本不该存在（%s）", filepath.Dir(p))
	}
	db, err := Open(p)
	if err != nil {
		t.Fatalf("库文件不存在时应能建库（不下报错），实得: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("开库后库文件应在盘: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatalf("开库后库文件不该为 0 字节")
	}
	t.Logf("态④ 库文件不存在 ⇒ 已建（%d B）· 无错", fi.Size())
}
