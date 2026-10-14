package zdb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// ── T2 · 备份 / 恢复 / GFS / 侧车 定向判据（§四.2 · §六 Q23–Q28）──
//
// 夹具复用同包 zdb_test.go 的 Open；fkViolationCount 在 open_test.go。

// backupTestMkDB —— 在 dbDir 下开一枚新库（库 / 备份目录分别定位 ⇒ 便于验同卷守卫与跨卷）。
// ★ 不挂 t.Cleanup：调用方按需显式 Close（有的格子要在同一路径上删库 / 恢复）。
func backupTestMkDB(t *testing.T, dbDir string) *DB {
	t.Helper()
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("建库目录失败: %v", err)
	}
	db, err := Open(filepath.Join(dbDir, "zerg.db"))
	if err != nil {
		t.Fatalf("Open(%s) 失败: %v", dbDir, err)
	}
	return db
}

// backupTestSeed —— 派 n 枚任务（每枚带一条首事件 ⇒ tasks = task_events = n）。
func backupTestSeed(t *testing.T, db *DB, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("task-%02d", i)
		if _, err := db.SubmitTask(Task{ID: id, SliceID: "slice-1"}, "", ""); err != nil {
			t.Fatalf("SubmitTask(%s) 失败: %v", id, err)
		}
	}
}

// backupTestReadMeta —— 读侧车成通用 map（验键集 / 值）。
func backupTestReadMeta(t *testing.T, snap string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(snap + ".meta.json")
	if err != nil {
		t.Fatalf("读侧车失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("解析侧车失败: %v", err)
	}
	return m
}

// ─────────────────── T2③ 命名 ＋ 侧车八格（§六 Q26） ───────────────────

// TestBackup_MetaSidecar —— 命名 `zerg-<UTC YYYYMMDD-HHMMSS>.db`（**UTC**）＋ 侧车八格逐格对拍：
// 键集恰 8 · sha256/size_bytes 与盘上件一致 · user_version/application_id/page_count 与快照 PRAGMA 一致 ·
// rows 含三表且值 = 逐表行数。
func TestBackup_MetaSidecar(t *testing.T) {
	db := backupTestMkDB(t, filepath.Join(t.TempDir(), "db"))
	defer db.Close()
	backupTestSeed(t, db, 3)

	bk := filepath.Join(t.TempDir(), "bk")
	snap, err := db.backupDefault(bk, true)
	if err != nil {
		t.Fatalf("backupDefault 失败: %v", err)
	}

	base := filepath.Base(snap)
	if !snapshotNameRE.MatchString(base) {
		t.Fatalf("快照命名不合 Q26 形态（zerg-<UTC YYYYMMDD-HHMMSS>.db）：%s", base)
	}
	// ★ 两态判据：时间戳用 **UTC**；若误用本地时区（CST=UTC+8）这里会差 8h。
	stamp := strings.TrimSuffix(strings.TrimPrefix(base, "zerg-"), ".db")
	got, err := time.ParseInLocation(backupStampLayout, stamp, time.UTC)
	if err != nil {
		t.Fatalf("时间戳不可解析: %v", err)
	}
	if d := time.Since(got); d < -3*time.Minute || d > 3*time.Minute {
		t.Fatalf("时间戳不像 UTC 现在（差 %v）—— 疑用了本地时区（Q26 逐字：用 UTC）", d)
	}

	// 侧车键集恰 8 格。
	meta := backupTestReadMeta(t, snap)
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"application_id", "created_at_utc", "integrity_check", "page_count", "rows", "sha256", "size_bytes", "user_version"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("侧车键集不符\n want=%v\n got =%v", want, keys)
	}

	// sha256 / size_bytes 与盘上件逐字对拍。
	sum, size, err := fileSHA256(snap)
	if err != nil {
		t.Fatalf("算 sha256 失败: %v", err)
	}
	if meta["sha256"] != sum {
		t.Fatalf("侧车 sha256（%v）≠ 盘上件 %s", meta["sha256"], sum)
	}
	if int64(meta["size_bytes"].(float64)) != size {
		t.Fatalf("侧车 size_bytes（%v）≠ 盘上 %d", meta["size_bytes"], size)
	}
	if meta["integrity_check"] != "ok" {
		t.Fatalf("侧车 integrity_check 应 ok，实得 %v", meta["integrity_check"])
	}
	if meta["created_at_utc"] == "" {
		t.Fatal("侧车 created_at_utc 不该空")
	}

	// 三格 PRAGMA = 侧车同名键。
	h, err := openSnapshotRO(snap)
	if err != nil {
		t.Fatalf("开快照失败: %v", err)
	}
	defer h.Close()
	var uv, appID, pc int
	if err := h.QueryRow("PRAGMA user_version").Scan(&uv); err != nil {
		t.Fatalf("读 user_version 失败: %v", err)
	}
	if err := h.QueryRow("PRAGMA application_id").Scan(&appID); err != nil {
		t.Fatalf("读 application_id 失败: %v", err)
	}
	if err := h.QueryRow("PRAGMA page_count").Scan(&pc); err != nil {
		t.Fatalf("读 page_count 失败: %v", err)
	}
	if int(meta["user_version"].(float64)) != uv {
		t.Fatalf("侧车 user_version（%v）≠ 快照 PRAGMA（%d）", meta["user_version"], uv)
	}
	if int64(meta["application_id"].(float64)) != int64(appID) {
		t.Fatalf("侧车 application_id（%v）≠ 快照 PRAGMA（%d）", meta["application_id"], appID)
	}
	if int(meta["page_count"].(float64)) != pc {
		t.Fatalf("侧车 page_count（%v）≠ 快照 PRAGMA（%d）", meta["page_count"], pc)
	}
	if int64(appID) != int64(ZergApplicationID) {
		t.Fatalf("快照 application_id 应 = ZergApplicationID（Q28）· 实得 %d", appID)
	}

	// rows = 逐表行数（三表闭集）。
	rows, ok := meta["rows"].(map[string]any)
	if !ok {
		t.Fatalf("侧车 rows 应为对象，实得 %T", meta["rows"])
	}
	if len(rows) != len(backupRowTables) {
		t.Fatalf("rows 应恰含 %d 表，实得 %v", len(backupRowTables), rows)
	}
	for _, tbl := range backupRowTables {
		if _, ok := rows[tbl]; !ok {
			t.Fatalf("rows 缺表 %s", tbl)
		}
	}
	if int(rows["tasks"].(float64)) != 3 || int(rows["task_events"].(float64)) != 3 || int(rows["idempotency_keys"].(float64)) != 0 {
		t.Fatalf("rows 值不符（期望 tasks=3 task_events=3 idempotency_keys=0）：%v", rows)
	}
}

// ─────────────────── T2① Restore 四步 ＋ 自证（§六 Q23 / Q27） ───────────────────

// TestBackup_RestoreRoundTrip —— 全周期：造库→写行→备份→删库→恢复→自证三件（体检 ＋ user_version ＋
// 逐表行数对拍）→ 还原后经**开库正门**能开。
func TestBackup_RestoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	db := backupTestMkDB(t, filepath.Join(root, "live"))
	backupTestSeed(t, db, 4)
	dbPath := db.Path()

	bk := filepath.Join(root, "bk")
	snap, err := db.backupDefault(bk, true)
	if err != nil {
		t.Fatalf("备份失败: %v", err)
	}
	meta := backupTestReadMeta(t, snap)

	// 删库（模拟「库丢了」）。
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}
	if err := os.Remove(dbPath); err != nil {
		t.Fatalf("删库失败: %v", err)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("前置：库文件应已删，实得 err=%v", err)
	}

	// 恢复。
	if err := Restore(snap, dbPath); err != nil {
		t.Fatalf("Restore 失败: %v", err)
	}

	// 自证三件。
	h, err := openSnapshotRO(dbPath)
	if err != nil {
		t.Fatalf("开还原后库失败: %v", err)
	}
	var ic string
	if err := h.QueryRow("PRAGMA integrity_check").Scan(&ic); err != nil || ic != "ok" {
		_ = h.Close()
		t.Fatalf("还原后 integrity_check 应单行 ok，实得 %q（err=%v）", ic, err)
	}
	if n := fkViolationCount(t, h); n != 0 {
		_ = h.Close()
		t.Fatalf("还原后 foreign_key_check 应 0 行，实得 %d（Q27）", n)
	}
	var uv int
	if err := h.QueryRow("PRAGMA user_version").Scan(&uv); err != nil {
		_ = h.Close()
		t.Fatalf("读还原后 user_version 失败: %v", err)
	}
	if uv != int(meta["user_version"].(float64)) {
		_ = h.Close()
		t.Fatalf("还原后 user_version（%d）应 = 侧车记录值（%v）", uv, meta["user_version"])
	}
	rows, err := countTableRows(h)
	if err != nil {
		_ = h.Close()
		t.Fatalf("数还原后行失败: %v", err)
	}
	srows := meta["rows"].(map[string]any)
	for _, tbl := range backupRowTables {
		if rows[tbl] != int(srows[tbl].(float64)) {
			_ = h.Close()
			t.Fatalf("逐表行数对拍不符：表 %s 还原 %d / 侧车 %v", tbl, rows[tbl], srows[tbl])
		}
	}
	if err := h.Close(); err != nil {
		t.Fatalf("关句柄失败: %v", err)
	}

	// 还原后经开库正门能开（fail-closed 体检三连过）。
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("还原后经 Open 正门应能开: %v", err)
	}
	if err := db2.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}
}

// TestRestore_RefusesTamperedSidecar —— 负控①：侧车 sha256 被改 ⇒ 恢复须拒，且**不静默盖**目标件。
func TestRestore_RefusesTamperedSidecar(t *testing.T) {
	root := t.TempDir()
	db := backupTestMkDB(t, filepath.Join(root, "live"))
	backupTestSeed(t, db, 2)
	dbPath := db.Path()
	bk := filepath.Join(root, "bk")
	snap, err := db.backupDefault(bk, true)
	if err != nil {
		t.Fatalf("备份失败: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}

	// 侧车 sha256 被改（伪造）。
	meta := backupTestReadMeta(t, snap)
	meta["sha256"] = strings.Repeat("0", 64)
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("序列化伪侧车失败: %v", err)
	}
	if err := os.WriteFile(snap+".meta.json", b, 0o644); err != nil {
		t.Fatalf("改侧车失败: %v", err)
	}

	// 目标处放哨兵（证明「不静默盖」）。
	if err := os.WriteFile(dbPath, []byte("sentinel-do-not-touch"), 0o644); err != nil {
		t.Fatalf("放哨兵失败: %v", err)
	}
	before, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("读哨兵失败: %v", err)
	}

	err = Restore(snap, dbPath)
	if err == nil {
		t.Fatal("侧车 sha256 被改竟放行还原（应拒 · 负控①）")
	}
	if !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("拒执理由应点名 sha256，实得:\n%v", err)
	}
	after, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("读目标件失败: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("被拒的还原竟改了目标件（静默盖）")
	}
}

// TestRestore_RefusesHotJournal —— 负控②：`-wal` 未删就强行恢复 ⇒ 须**检出/拒**（✗ 静默盖），
// 且那枚非空 -wal 不被静默吞掉（须由人按 §四.2 乙 ② 清）。
func TestRestore_RefusesHotJournal(t *testing.T) {
	root := t.TempDir()
	db := backupTestMkDB(t, filepath.Join(root, "live"))
	backupTestSeed(t, db, 2)
	dbPath := db.Path()
	bk := filepath.Join(root, "bk")
	snap, err := db.backupDefault(bk, true)
	if err != nil {
		t.Fatalf("备份失败: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}

	// 造「-wal 未删」态：目标处留一枚非空 -wal（配对原则的禁区）。
	if err := os.WriteFile(dbPath, []byte("stale-db"), 0o644); err != nil {
		t.Fatalf("放哨兵库失败: %v", err)
	}
	if err := os.WriteFile(dbPath+"-wal", []byte("hot-journal-not-checkpointed"), 0o644); err != nil {
		t.Fatalf("放 -wal 失败: %v", err)
	}
	before, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("读哨兵失败: %v", err)
	}

	err = Restore(snap, dbPath)
	if err == nil {
		t.Fatal("-wal 非空竟放行还原（应检出/拒 · 负控②，✗ 静默盖）")
	}
	if !strings.Contains(err.Error(), "-wal") {
		t.Fatalf("拒执理由应点名 -wal，实得:\n%v", err)
	}
	after, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("读目标件失败: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("被拒的还原竟改了目标库件（静默盖）")
	}
	if _, err := os.Stat(dbPath + "-wal"); err != nil {
		t.Fatalf("-wal 不应被静默删除（须由人按 §四.2 乙 ② 清）：%v", err)
	}
}

// ─────────────────── T2② GFS 保留 / 轮转（§六 Q24） ───────────────────

// TestBackup_GFSRetentionPrune —— 三态：
//
//	① 40 天日备 ⇒ prune 后 ≤ 日/周/月并集上限、≥ 保底 3；最近 3 枚（保底）仍在；
//	② **只**删匹配命名件（目录里的不匹配外件一字不碰）；被删件的侧车一并删、留存件侧车仍在；
//	③ 更小档（caps=1/1/1 ＋ 保底 3）⇒ 恰留 3（保底生效）。
func TestBackup_GFSRetentionPrune(t *testing.T) {
	bk := t.TempDir()
	base := time.Date(2026, 1, 1, 3, 30, 0, 0, time.UTC)
	const n = 40
	names := make([]string, 0, n)
	for i := 0; i < n; i++ {
		nm, mt := snapshotName(base.AddDate(0, 0, i))
		if err := os.WriteFile(filepath.Join(bk, nm), []byte("x"), 0o644); err != nil {
			t.Fatalf("造快照失败: %v", err)
		}
		if err := os.WriteFile(filepath.Join(bk, mt), []byte("{}"), 0o644); err != nil {
			t.Fatalf("造侧车失败: %v", err)
		}
		names = append(names, nm)
	}
	// 目录里再放一枚**不匹配命名**的件（prune 必须不碰）。
	foreign := filepath.Join(bk, "README.txt")
	if err := os.WriteFile(foreign, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("造外件失败: %v", err)
	}

	removed, err := PruneBackups(bk, GFSDaily, GFSWeekly, GFSMonthly, GFSFloor)
	if err != nil {
		t.Fatalf("PruneBackups 失败: %v", err)
	}
	left, err := listSnapshots(bk)
	if err != nil {
		t.Fatalf("清点失败: %v", err)
	}
	if len(left) < GFSFloor {
		t.Fatalf("保底破：剩 %d 份 < 保底 %d", len(left), GFSFloor)
	}
	if len(left)+len(removed) != n {
		t.Fatalf("账不平：剩 %d ＋ 删 %d ≠ 总 %d", len(left), len(removed), n)
	}
	if len(left) > GFSDaily+GFSWeekly+GFSMonthly {
		t.Fatalf("GFS 未收敛：留 %d > 日+周+月 上限 %d", len(left), GFSDaily+GFSWeekly+GFSMonthly)
	}
	if len(removed) == 0 {
		t.Fatal("40 天日备竟一份没删 —— 轮转没生效")
	}
	// 最近 3 枚（保底）必须在。
	kept := map[string]bool{}
	for _, s := range left {
		kept[s.Name] = true
	}
	for i := n - GFSFloor; i < n; i++ {
		if !kept[names[i]] {
			t.Fatalf("保底份被删：%s", names[i])
		}
	}
	// 只删匹配命名件。
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("不匹配命名的外件竟被删（prune 越界）: %v", err)
	}
	for _, nm := range removed {
		if !snapshotNameRE.MatchString(nm) {
			t.Fatalf("被删件名不匹配命名式：%s", nm)
		}
		if _, err := os.Stat(filepath.Join(bk, nm+".meta.json")); !os.IsNotExist(err) {
			t.Fatalf("被删件的侧车未删：%s", nm)
		}
	}
	for _, s := range left {
		if _, err := os.Stat(filepath.Join(bk, s.Name+".meta.json")); err != nil {
			t.Fatalf("留存件的侧车丢失：%s（%v）", s.Name, err)
		}
	}

	// ③ 更小档：caps=1/1/1 ＋ 保底 3 ⇒ 恰留 3。
	if _, err := PruneBackups(bk, 1, 1, 1, GFSFloor); err != nil {
		t.Fatalf("小档 prune 失败: %v", err)
	}
	again, err := listSnapshots(bk)
	if err != nil {
		t.Fatalf("清点失败: %v", err)
	}
	if len(again) != GFSFloor {
		t.Fatalf("小档应恰留保底 %d 份，实得 %d", GFSFloor, len(again))
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("小档 prune 后外件也丢了: %v", err)
	}
}

// ─────────────────── T2② 空间预检（§六 Q24） ───────────────────

// TestBackup_SpacePrecheck —— 两态纯判据（可用=2×库 ⇒ 过；可用=2×库−1 ⇒ 拒）＋ 真跑一枚常规目录。
func TestBackup_SpacePrecheck(t *testing.T) {
	if err := spacePrecheck(1000, 500); err != nil {
		t.Fatalf("可用 = 2×库 应过：%v", err)
	}
	if err := spacePrecheck(999, 500); err == nil {
		t.Fatal("可用 < 2×库 竟放行（应 fail-closed）")
	}
	db := backupTestMkDB(t, filepath.Join(t.TempDir(), "db"))
	defer db.Close()
	if err := db.backupSpacePrecheck(t.TempDir()); err != nil {
		t.Fatalf("常规目录（空闲远大于 2×小额库）应过预检：%v", err)
	}
}

// ─────────────────── 端到端真跑（仓外 scratch · 跨卷） ───────────────────

// TestBackup_RestoreE2E —— 端到端：造库→写几行→备份→删库→恢复→自证三件→再备份几轮→
// 验 GFS prune 只留该留的、保底那份在、目录里只剩匹配命名件。
//
// ★ 跨卷：给 `ZDB_E2E_BACKUP_DIR`（另一卷路径）⇒ 同卷守卫**真生效**；未给 ⇒ 同卷夹具（跳守卫，
// 只验 VACUUM INTO ＋ 侧车 ＋ GFS ＋ Restore）。验收跑法见报告（TMPDIR 指仓外 scratch）。
func TestBackup_RestoreE2E(t *testing.T) {
	root := t.TempDir()
	db := backupTestMkDB(t, filepath.Join(root, "live"))
	backupTestSeed(t, db, 5)
	dbPath := db.Path()

	bk := os.Getenv("ZDB_E2E_BACKUP_DIR")
	guard := true
	if bk == "" {
		bk = filepath.Join(root, "bk")
		guard = false
	}
	if err := os.MkdirAll(bk, 0o755); err != nil {
		t.Fatalf("建备份目录失败: %v", err)
	}
	t.Logf("E2E：库=%s · 备份目录=%s · 跨卷守卫=%v", dbPath, bk, guard)

	snap1, err := db.backupDefault(bk, !guard)
	if err != nil {
		t.Fatalf("备份#1 失败: %v", err)
	}

	// 删库 → 恢复 → 自证三件。
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}
	if err := os.Remove(dbPath); err != nil {
		t.Fatalf("删库失败: %v", err)
	}
	if err := Restore(snap1, dbPath); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("还原后开库失败: %v", err)
	}
	var n int
	if err := db2.SQL().QueryRow("SELECT COUNT(*) FROM tasks").Scan(&n); err != nil {
		t.Fatalf("数 tasks 失败: %v", err)
	}
	if n != 5 {
		t.Fatalf("还原后 tasks 应 5，实得 %d", n)
	}
	var ic string
	if err := db2.Reader().QueryRow("PRAGMA integrity_check").Scan(&ic); err != nil || ic != "ok" {
		t.Fatalf("还原后 integrity_check 应 ok：%q（err=%v）", ic, err)
	}
	var ruv int
	if err := db2.Reader().QueryRow("PRAGMA user_version").Scan(&ruv); err != nil {
		t.Fatalf("读还原后 user_version 失败: %v", err)
	}
	fkRows := fkViolationCount(t, db2.Reader())
	var et, ie int
	if err := db2.Reader().QueryRow("SELECT count(*) FROM task_events").Scan(&et); err != nil {
		t.Fatalf("数 task_events 失败: %v", err)
	}
	if err := db2.Reader().QueryRow("SELECT count(*) FROM idempotency_keys").Scan(&ie); err != nil {
		t.Fatalf("数 idempotency_keys 失败: %v", err)
	}
	t.Logf("自证三件：integrity_check=%s · foreign_key_check=%d 行 · user_version=%d · rows{tasks:%d,task_events:%d,idempotency_keys:%d}",
		ic, fkRows, ruv, n, et, ie)
	t.Logf("快照#1=%s（侧车=同名+.meta.json）", filepath.Base(snap1))

	// 再备份几轮（同秒撞名 ⇒ 退避 +1s ⇒ 每轮产新名）。
	for i := 0; i < 4; i++ {
		if _, err := db2.backupDefault(bk, !guard); err != nil {
			t.Fatalf("再备份 #%d 失败: %v", i, err)
		}
		time.Sleep(1100 * time.Millisecond)
	}
	left, err := listSnapshots(bk)
	if err != nil {
		t.Fatalf("清点失败: %v", err)
	}
	if len(left) < GFSFloor {
		t.Fatalf("GFS 保底破：剩 %d < %d", len(left), GFSFloor)
	}
	keptNames := make([]string, 0, len(left))
	for _, s := range left {
		keptNames = append(keptNames, s.Name)
	}
	t.Logf("GFS：本轮共备份 %d 轮 ⇒ 留存 %d 份（保底 %d）· 轮转删 %d 份 · 留存件=%v",
		1+4, len(left), GFSFloor, (1+4)-len(left), keptNames)
	// 目录里只剩匹配命名件（prune 把不匹配面一个不留 —— 本夹具里本就不该有别件）。
	ents, err := os.ReadDir(bk)
	if err != nil {
		t.Fatalf("读备份目录失败: %v", err)
	}
	for _, e := range ents {
		nm := e.Name()
		if strings.HasSuffix(nm, ".meta.json") {
			nm = strings.TrimSuffix(nm, ".meta.json")
		}
		if e.IsDir() || snapshotNameRE.MatchString(nm) {
			continue
		}
		t.Fatalf("备份目录里出现不匹配命名的件：%s", e.Name())
	}
	// 保底那份在（最新一枚可开）。
	newest := left[0]
	h, err := sql.Open("sqlite", "file:"+dsnEscapePath(filepath.Join(bk, newest.Name))+"?mode=ro")
	if err != nil {
		t.Fatalf("开最新备份失败: %v", err)
	}
	defer h.Close()
	if err := h.QueryRow("PRAGMA integrity_check").Scan(&ic); err != nil || ic != "ok" {
		t.Fatalf("最新备份件 integrity_check 应 ok：%q（err=%v）", ic, err)
	}
	if err := db2.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}
}
