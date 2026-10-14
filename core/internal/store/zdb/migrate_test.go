package zdb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ═════════════════ T3 · 迁移面（迁移器改形态 / 0002 改路线 / 前置快照硬闸 / 真库抄件演练） ═════════════════
//
// 出处（Zerg-内部文档/01-设计/设计-虫族数据库-20261013.md）：
//
//	§三.7 附 I.3（阻断级实测：原路 `0002` 在 COMMIT 报 `FOREIGN KEY constraint failed`）· I.4（官方十二步逐格）
//	§四.1 附 J.3（破坏性迁移前置快照）· J.4（现读缺口：闸没编进迁移路径）
//	§六 Q15（取甲：另开一条不带 `foreign_keys(ON)` 的临时连接）· Q19（`0002` 可改 · ✗ 不追加 0003）·
//	    Q20（事务外先关 FK ＋ 补两处细节）· Q21（① 前置快照硬闸 ② 迁移须先在真库抄件上真跑一遍）
//
// 四态：① 空库 ② **1 条 `task_events` 行**的库（旧路必红 / 新路必绿）③ 多行 ＋ 多表 ④ 真库抄件。

// TestMain —— 本包单测的**隔离前置**：破坏性迁移会落一份前置快照（落点 = `BackupDir()`），
// ✗ 不许单测往用户真备份目录（`~/.zerg/backup/`）里丢件 ⇒ 未显式指定 `ZERG_BACKUP_DIR` 时，
// 把落点钉到测试自己的临时目录（跑完清掉）。显式设了该变量的人自己负责 —— 本前置不动那一档。
func TestMain(m *testing.M) {
	if os.Getenv("ZERG_BACKUP_DIR") == "" {
		dir, err := os.MkdirTemp("", "zdb-test-backup-")
		if err != nil {
			fmt.Fprintf(os.Stderr, "zdb 测试隔离失败：建临时备份目录: %v\n", err)
			os.Exit(1)
		}
		if err := os.Setenv("ZERG_BACKUP_DIR", dir); err != nil {
			fmt.Fprintf(os.Stderr, "zdb 测试隔离失败：钉 ZERG_BACKUP_DIR: %v\n", err)
			os.Exit(1)
		}
		code := m.Run()
		_ = os.RemoveAll(dir)
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// ───────────────────── 夹具（✗ 不重复造一套：复用同包 newTestDB / legacyStateDDLWidened / openRawNoFK） ─────────────────────

// legacyRow —— 造旧库一行用的事实（只带本片要核的那几格）。
type legacyRow struct {
	id     string
	state  string
	steer  any // last_steered_at（**旧列名**）
	paused any // paused_at
	retry  int
}

// buildLegacyDB —— 造一枚「旧库」（`user_version=1` · 0001 的**放宽版** DDL · FK **不开**）并逐行塞入。
// ★ 用不带 `foreign_keys(ON)` 的裸连接：`0002` 的归一 CASE 要对「旧两代血脉」都成立，而其中
// `steered` / `paused` / `waiting_retry` / `cancelled` 不在 0001 的窄 CHECK 里 ⇒ 夹具须放宽才造得出真前态。
func buildLegacyDB(t *testing.T, path string, rows []legacyRow) {
	t.Helper()
	raw := openRawNoFK(t, path)
	if _, err := raw.Exec(legacyStateDDLWidened(t)); err != nil {
		raw.Close()
		t.Fatalf("落旧 DDL 失败: %v", err)
	}
	if _, err := raw.Exec("PRAGMA user_version = 1"); err != nil {
		raw.Close()
		t.Fatalf("钉旧版本号失败: %v", err)
	}
	for _, r := range rows {
		if _, err := raw.Exec(`INSERT INTO tasks(id, slice_id, state, paused_at, last_steered_at, retry_count, created_at, updated_at)
            VALUES(?,?,?,?,?,?,?,?)`, r.id, "slice-legacy", r.state, r.paused, r.steer, r.retry, 1.0, 1.0); err != nil {
			raw.Close()
			t.Fatalf("造旧行 %s(%s) 失败: %v", r.id, r.state, err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("关裸连接失败: %v", err)
	}
}

// addEventRow —— 往库里塞一条 `task_events` 行（`taskID` 悬空 ⇒ 造孤儿，用来验外键闸的牙）。
func addEventRow(t *testing.T, path, taskID string, seq int) {
	t.Helper()
	raw := openRawNoFK(t, path)
	if _, err := raw.Exec(`INSERT INTO task_events(task_id, audit_seq, to_state, ts) VALUES(?,?,?,?)`,
		taskID, seq, "running", 1.0); err != nil {
		raw.Close()
		t.Fatalf("造 task_events 行（task_id=%s）失败: %v", taskID, err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("关裸连接失败: %v", err)
	}
}

// addIdemRow —— 往库里塞一条 `idempotency_keys` 行（多表数据态用）。
func addIdemRow(t *testing.T, path, key string) {
	t.Helper()
	raw := openRawNoFK(t, path)
	if _, err := raw.Exec(`INSERT INTO idempotency_keys(slice_id, idempotency_key, created_at, expires_at) VALUES(?,?,?,?)`,
		"slice-legacy", key, 1.0, 2.0); err != nil {
		raw.Close()
		t.Fatalf("造 idempotency_keys 行（%s）失败: %v", key, err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("关裸连接失败: %v", err)
	}
}

// openRawFK —— 开一条**带** `foreign_keys(ON)` 的裸连接（写侧 DSN 同形）—— 复现「旧路」用。
func openRawFK(t *testing.T, path string) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite", BuildDSN(path))
	if err != nil {
		t.Fatalf("带 FK 裸开 %q 失败: %v", path, err)
	}
	return raw
}

// dbFacts —— 一枚库的现读事实（**全部经只读句柄**读：`openSnapshotRO` = `mode=ro`，✗ 不开可写连接）。
type dbFacts struct {
	UserVersion  int
	Tables       []string
	TaskCols     []string
	Rows         map[string]int
	FkViolations int
	States       map[string]string // task.id → state
	StateDist    map[string]int
}

// readFacts —— 只读读库（旧 schema / 新 schema **都能读**：只碰 user_version / 表 / 行数 / id+state）。
func readFacts(t *testing.T, path string) dbFacts {
	t.Helper()
	h, err := openSnapshotRO(path)
	if err != nil {
		t.Fatalf("只读开 %q 失败: %v", path, err)
	}
	defer h.Close()
	f := dbFacts{States: map[string]string{}, StateDist: map[string]int{}}
	if err := h.QueryRow("PRAGMA user_version").Scan(&f.UserVersion); err != nil {
		t.Fatalf("读 user_version 失败: %v", err)
	}
	tr, err := h.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("读表清单失败: %v", err)
	}
	for tr.Next() {
		var n string
		if err := tr.Scan(&n); err != nil {
			tr.Close()
			t.Fatalf("扫表名失败: %v", err)
		}
		f.Tables = append(f.Tables, n)
	}
	tr.Close()
	cr, err := h.Query(`SELECT name FROM pragma_table_info('tasks') ORDER BY cid`)
	if err != nil {
		t.Fatalf("读 tasks 列清单失败: %v", err)
	}
	for cr.Next() {
		var n string
		if err := cr.Scan(&n); err != nil {
			cr.Close()
			t.Fatalf("扫列名失败: %v", err)
		}
		f.TaskCols = append(f.TaskCols, n)
	}
	cr.Close()
	if f.Rows, err = countTableRows(h); err != nil {
		t.Fatalf("数逐表行数失败: %v", err)
	}
	f.FkViolations = fkViolationCount(t, h)
	rows, err := h.Query(`SELECT id, state FROM tasks`)
	if err != nil {
		t.Fatalf("读 tasks 失败: %v", err)
	}
	for rows.Next() {
		var id, state string
		if err := rows.Scan(&id, &state); err != nil {
			rows.Close()
			t.Fatalf("扫 tasks 失败: %v", err)
		}
		f.States[id] = state
		f.StateDist[state]++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("读 tasks 中断: %v", err)
	}
	return f
}

// reworkFields —— **迁移后** schema 的三字段现读（`last_steer_at` / `paused` / `next_attempt_at`）。
type reworkFields struct {
	SteerAt     sql.NullFloat64
	Paused      int
	NextAttempt sql.NullFloat64
}

func readReworkFields(t *testing.T, path, taskID string) reworkFields {
	t.Helper()
	h, err := openSnapshotRO(path)
	if err != nil {
		t.Fatalf("只读开 %q 失败: %v", path, err)
	}
	defer h.Close()
	var rf reworkFields
	if err := h.QueryRow(`SELECT last_steer_at, paused, next_attempt_at FROM tasks WHERE id=?`, taskID).
		Scan(&rf.SteerAt, &rf.Paused, &rf.NextAttempt); err != nil {
		t.Fatalf("读 %s 的三字段失败: %v", taskID, err)
	}
	return rf
}

// snapshotFiles —— 备份目录里**匹配命名**的快照件（名 → mtime）；目录不在 ⇒ 空。
func snapshotFiles(t *testing.T, dir string) map[string]time.Time {
	t.Helper()
	out := map[string]time.Time{}
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out
		}
		t.Fatalf("读备份目录 %s 失败: %v", dir, err)
	}
	for _, e := range ents {
		if e.IsDir() || !snapshotNameRE.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			t.Fatalf("取 %s 信息失败: %v", e.Name(), err)
		}
		out[e.Name()] = info.ModTime()
	}
	return out
}

func hasStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ─────────── ① 类别判定（走原路还是重路的**唯一依据**）· 两态：判据真读 SQL（✗ 不是恒真） ───────────

func TestMigration_ClassDecision(t *testing.T) {
	migs, max, err := loadMigrations()
	if err != nil {
		t.Fatalf("读内嵌迁移集失败: %v", err)
	}
	if max != 3 {
		t.Fatalf("内嵌迁移集上限应为 3，实得 %d", max)
	}
	if isDestructiveMigration(migs[1]) {
		t.Error("0001 是纯新增类（只 CREATE / 建索引）⇒ ✗ 不该判成破坏性（别把简单路也变复杂）")
	}
	if !isDestructiveMigration(migs[2]) {
		t.Error("0002 含 `DROP TABLE` ＋ `ALTER … RENAME TO` ⇒ 必判成「破坏性 / 重建表类」")
	}
	// 两态：判据真读 SQL（✗ 不是恒真）——
	// ① 摘掉 `DROP TABLE`、留着 `RENAME` ⇒ 仍判破坏性（RENAME 那一支也真读）；
	// ② 纯新增 SQL（只 CREATE / 建索引）⇒ 不判破坏性。
	noDrop := strings.Replace(migs[2], "DROP TABLE tasks;", "", 1)
	if noDrop == migs[2] {
		t.Fatal("夹具锚失效：0002 里找不到 `DROP TABLE tasks;`（判据锚需随动）")
	}
	if !isDestructiveMigration(noDrop) {
		t.Error("摘掉 `DROP TABLE` 后仍应判破坏性（该件还留着 `ALTER … RENAME TO`）⇒ 判据漏了 RENAME 那一支")
	}
	additive := "CREATE TABLE x (id TEXT PRIMARY KEY);\nCREATE INDEX IF NOT EXISTS i_x ON x(id);\n"
	if isDestructiveMigration(additive) {
		t.Error("纯新增 SQL（只 CREATE / 建索引）被误判成破坏性 ⇒ 判据是恒真的")
	}
}

// ─────────── ② 0002 改路线：defer 路线已剥离 · 官方十二步的 SQL 面与归一 CASE 逐字在位 ───────────

func TestMigration_0002DeferRouteRemoved(t *testing.T) {
	migs, _, err := loadMigrations()
	if err != nil {
		t.Fatalf("读内嵌迁移集失败: %v", err)
	}
	text := migs[2]
	if strings.Contains(text, "defer_foreign_keys") {
		t.Error("0002 里仍留着 `defer_foreign_keys` 路线（事务内成立不了 ⇒ 有 event 行的库必在 COMMIT 炸）")
	}
	// 官方十二步（4→8）逐段在位 —— ✗ 改路线不许把重建体本身改掉。
	for _, must := range []string{
		"CREATE TABLE tasks_new (",
		"\nINSERT INTO tasks_new (",
		"\nFROM tasks;",
		"DROP TABLE tasks;",
		"ALTER TABLE tasks_new RENAME TO tasks;",
		"CREATE INDEX IF NOT EXISTS idx_tasks_state ",
	} {
		if !strings.Contains(text, must) {
			t.Errorf("0002 缺官方十二步的一段: %q", must)
		}
	}
	// 归一语义的**机器面**逐字在位（语义未变的证据之一；另一处是逐行对 NormalizeLegacyState()）。
	for _, must := range []string{
		"WHEN 'paused'        THEN 'queued'",
		"WHEN 'waiting_retry' THEN 'queued'",
		"WHEN 'steered'       THEN 'running'",
		"        ELSE state",
		"CASE WHEN state = 'paused' THEN 1 ELSE COALESCE(paused, 0) END",
		"paused_at, paused_reason, last_steered_at, idempotency_key, request_hash,",
	} {
		if !strings.Contains(text, must) {
			t.Errorf("0002 的归一语义被改动了（缺: %q）", must)
		}
	}
}

// ─────────── ③ 四态①空库：新路必绿；且**只有破坏性那一步**产快照（纯新增路一字未动） ───────────

func TestMigration_EmptyDB_GreenAndOnlyDestructiveSnapshots(t *testing.T) {
	bdir := filepath.Join(t.TempDir(), "backup")
	t.Setenv("ZERG_BACKUP_DIR", bdir)
	p := filepath.Join(t.TempDir(), "zerg.db")

	db, err := Open(p) // uv=0 → 0001（纯新增 · 原路）→ 0002（破坏性 · 重路）
	if err != nil {
		t.Fatalf("空库 Open 失败: %v", err)
	}
	_ = db.Close()

	f := readFacts(t, p)
	t.Logf("空库迁移后：user_version=%d · 表=%v · 行=%v · 外键违例=%d 行 · 态分布=%v",
		f.UserVersion, f.Tables, f.Rows, f.FkViolations, f.StateDist)
	if f.UserVersion != 3 {
		t.Errorf("空库迁移后 user_version 应 3，实得 %d", f.UserVersion)
	}
	if f.FkViolations != 0 {
		t.Errorf("空库迁移后 foreign_key_check 应 0 行，实得 %d", f.FkViolations)
	}
	if got := len(snapshotFiles(t, bdir)); got != 1 {
		t.Errorf("空库首建应**恰产 1 份**前置快照（只有 0002 那一步是破坏性；0001 走原路 ⇒ ✗ 不产快照），实得 %d", got)
	}
}

// ─────────── ④ 四态②-a 旧路**必红**：FK 开着重建 ⇒ COMMIT 报外键违例（机理层 · 与 0002 文件无关的那一半） ───────────

// oldRouteRebuildSQL —— 原路（改前形态）的重建体：官方十二步的 4→7 段（含 0002 的归一 CASE），
// **FK 开着**跑。要复现的就是「`DROP TABLE` 的隐式 DELETE 种下延迟违规、`RENAME` 不消解、
// COMMIT 才炸」这条机理（§三.7 附 I.3 的变体 A / B / G）。
// ★ 这里是**最小重现体**；「照改前那份 0002 原件逐字跑」的对照在仓外 sqlite3 演练里（见报告 T3★ ①）。
const oldRouteRebuildSQL = `
CREATE TABLE tasks_new (
    id          TEXT PRIMARY KEY,
    slice_id    TEXT NOT NULL,
    state       TEXT NOT NULL CHECK (state IN ('queued','running','done','failed','cancelled')),
    retry_count INTEGER NOT NULL DEFAULT 0
);
INSERT INTO tasks_new (id, slice_id, state, retry_count)
SELECT id, slice_id, CASE state WHEN 'steered' THEN 'running' ELSE state END, retry_count FROM tasks;
DROP TABLE tasks;
ALTER TABLE tasks_new RENAME TO tasks;
`

func TestMigration_OldRouteRebuildWithFKOn_CommitsRed(t *testing.T) {
	p := filepath.Join(t.TempDir(), "legacy.db")
	buildLegacyDB(t, p, []legacyRow{{id: "t1", state: "steered", steer: 1.0}})
	addEventRow(t, p, "t1", 1) // ★ 就是这一行把原路打死

	raw := openRawFK(t, p)
	defer raw.Close()
	var fkOn int
	if err := raw.QueryRow("PRAGMA foreign_keys").Scan(&fkOn); err != nil {
		t.Fatalf("回读 foreign_keys 失败: %v", err)
	}
	if fkOn != 1 {
		t.Fatalf("前置：旧路那条连接应带 foreign_keys=ON，实得 %d", fkOn)
	}

	tx, err := raw.Begin()
	if err != nil {
		t.Fatalf("开事务失败: %v", err)
	}
	var gotErr error
	if _, err := tx.Exec(oldRouteRebuildSQL); err != nil {
		gotErr = err
	} else if _, err := tx.Exec("PRAGMA user_version = 2"); err != nil {
		gotErr = err
	} else if err := tx.Commit(); err != nil {
		gotErr = err
	}
	if gotErr == nil {
		t.Fatal("原路（FK 开着重建）竟过了 —— 机理层阻断没复现（库里有 task_events 行时它必炸）")
	}
	if !strings.Contains(strings.ToUpper(gotErr.Error()), "FOREIGN KEY CONSTRAINT FAILED") {
		t.Fatalf("原路报错应为外键违例，实得: %v", gotErr)
	}
	t.Logf("旧路读数（FK 开着重建）= %v", gotErr)
	_ = tx.Rollback()
	if f := readFacts(t, p); f.UserVersion != 1 {
		t.Errorf("原路炸了却把 user_version 推到了 %d（应回滚在 1）", f.UserVersion)
	}
}

// ─────────── ⑤ T3-a 新路（Go 迁移器）：①1 条 task_events 行的库 ⇒ 必绿 ＋ 收尾外键检查 0 行 ───────────

func TestMigration_NewRoute_OneEventRowGreen(t *testing.T) {
	t.Setenv("ZERG_BACKUP_DIR", filepath.Join(t.TempDir(), "backup"))
	p := filepath.Join(t.TempDir(), "legacy.db")
	buildLegacyDB(t, p, []legacyRow{{id: "t1", state: "steered", steer: 22.0, retry: 3}})
	addEventRow(t, p, "t1", 1)

	db, err := Open(p) // 事务外先关 FK ⇒ 0002 过得去（改前形态在这里必红）
	if err != nil {
		t.Fatalf("新路 Open 失败（1 条 task_events 行就炸 = 还没走事务外先关 FK）: %v", err)
	}
	defer db.Close()

	f := readFacts(t, p)
	t.Logf("新路迁移后：user_version=%d · 表=%v · 行=%v · 外键违例=%d 行 · 态分布=%v",
		f.UserVersion, f.Tables, f.Rows, f.FkViolations, f.StateDist)
	if f.UserVersion != 3 {
		t.Errorf("新路迁移后 user_version 应 3，实得 %d", f.UserVersion)
	}
	if f.FkViolations != 0 {
		t.Errorf("新路迁移后 foreign_key_check 应 0 行，实得 %d", f.FkViolations)
	}
	if f.States["t1"] != "running" {
		t.Errorf("steered 旧行应归一为 running，实得 %q", f.States["t1"])
	}
	if f.Rows["task_events"] != 1 || f.Rows["tasks"] != 1 {
		t.Errorf("行数应保持（tasks=1 / task_events=1），实得 %v", f.Rows)
	}
	if rf := readReworkFields(t, p, "t1"); !rf.SteerAt.Valid || rf.SteerAt.Float64 != 22.0 {
		t.Errorf("last_steered_at 的值得随行搬进 last_steer_at（要 22.0），实得 %+v", rf.SteerAt)
	}
}

// ─────────── ⑥ T3-a 专用连接两态：DSN 不带 foreign_keys(ON) · 事务外关得掉 ⇒ 回读 0 · 复位 ⇒ 回读 1 ───────────

func TestMigration_BuildMigrationDSN_NoFKPragma(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zerg.db")
	dsn, err := buildMigrationDSN(p)
	if err != nil {
		t.Fatalf("生成专用迁移连接 DSN 失败: %v", err)
	}
	if strings.Contains(dsn, "foreign_keys") {
		t.Errorf("专用迁移连接 DSN 竟带 foreign_keys（那就在**事务外**关不掉了）: %s", dsn)
	}
	if want := strings.Replace(BuildDSN(p), fkPragmaEntry, "", 1); dsn != want {
		t.Errorf("专用迁移连接 DSN 应 = BuildDSN 摘掉那一枚 pragma：\n got=%s\nwant=%s", dsn, want)
	}
	if !strings.Contains(dsn, "&_txlock=immediate") {
		t.Errorf("其余 pragma / `_txlock=immediate` 必须保住，实得: %s", dsn)
	}
	// 真开一条：初读 0 ⇒ 事务外关 ⇒ 回读 0 ⇒ 复位 ⇒ 回读 1（两态）。
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("开专用迁移连接失败: %v", err)
	}
	defer raw.Close()
	raw.SetMaxOpenConns(1)
	var fk int
	if err := raw.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("回读 foreign_keys 失败: %v", err)
	}
	if fk != 0 {
		t.Errorf("不带 foreign_keys(ON) 的连接初读应 0，实得 %d", fk)
	}
	if _, err := raw.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("事务外关 FK 失败: %v", err)
	}
	if err := raw.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("回读 foreign_keys 失败: %v", err)
	}
	if fk != 0 {
		t.Errorf("事务外关 FK 后回读应 0，实得 %d", fk)
	}
	if _, err := raw.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("复位 FK 失败: %v", err)
	}
	if err := raw.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("回读 foreign_keys 失败: %v", err)
	}
	if fk != 1 {
		t.Errorf("复位后回读应 1（Q20 细节①：不复位则该连接后续查询全失 FK 检查），实得 %d", fk)
	}
}

// ─────────── ⑦ T3-a 收尾 foreign_key_check 的牙（★ 负控对象：哑化它 ⇒ 本用例须转 FAIL） ───────────

func TestMigration_ForeignKeyCheckGate_RefusesOrphan(t *testing.T) {
	t.Setenv("ZERG_BACKUP_DIR", filepath.Join(t.TempDir(), "backup"))
	p := filepath.Join(t.TempDir(), "legacy.db")
	buildLegacyDB(t, p, []legacyRow{{id: "t1", state: "queued"}})
	addEventRow(t, p, "ghost", 1) // ★ 悬空 task_id ⇒ 重建后 foreign_key_check 必报 1 行

	// 白盒：直接驱动迁移器（✗ 不经 Open —— 开库体检会**先**把孤儿拦下（§六 Q27），
	// 本格要单独验的是「迁移**收尾**那道闸」的牙。`&DB{}` 的用法与 backup.go 的 Restore 同形。）
	raw := openRawFK(t, p)
	if got := fkViolationCount(t, raw); got != 1 {
		raw.Close()
		t.Fatalf("前置：foreign_key_check 应报 1 行违例，实得 %d", got)
	}
	d := &DB{db: raw, path: p}
	err := d.migrate()
	if err == nil {
		raw.Close()
		t.Fatal("收尾 foreign_key_check 报了 1 行违例，迁移竟成功 —— 那道闸没牙")
	}
	if !strings.Contains(err.Error(), "foreign_key_check") {
		t.Logf("拒因: %v", err)
		raw.Close()
		t.Fatalf("拒因应点名 foreign_key_check，实得: %v", err)
	}
	t.Logf("收尾闸读数（孤儿行）= %v", err)
	raw.Close()

	// 整事务回滚 ⇒ user_version 不动（✗ 不留「已提交的坏库」）。
	f := readFacts(t, p)
	if f.UserVersion != 1 {
		t.Errorf("收尾闸拒了却把 user_version 推到了 %d（应回滚在 1）", f.UserVersion)
	}
	if _, ok := f.States["t1"]; !ok {
		t.Errorf("回滚后旧行应仍在，实得 %v", f.States)
	}
}

// ─────────── ⑧ 四态③ 多行 ＋ 多表：新路绿 · 归一读数逐行对（新值分布贴上）· 语义未变的机器证据 ───────────

func TestMigration_MultiRowMultiTable_NormalizationReadings(t *testing.T) {
	t.Setenv("ZERG_BACKUP_DIR", filepath.Join(t.TempDir(), "backup"))
	p := filepath.Join(t.TempDir(), "legacy.db")
	rows := []legacyRow{
		{id: "q", state: "queued"},
		{id: "r", state: "running"},
		{id: "s", state: "steered", steer: 7.0},
		{id: "d", state: "done"},
		{id: "f", state: "failed"},
		{id: "pz", state: "paused", paused: 11.0},
		{id: "wr", state: "waiting_retry", retry: 3},
		{id: "c", state: "cancelled"},
	}
	buildLegacyDB(t, p, rows)
	addEventRow(t, p, "q", 1)
	addEventRow(t, p, "r", 1)
	addEventRow(t, p, "s", 1)
	addIdemRow(t, p, "k1")
	addIdemRow(t, p, "k2")

	before := readFacts(t, p)
	t.Logf("抄件迁移前：user_version=%d · 表=%v · 行=%v · 外键违例=%d 行 · 旧态分布=%v",
		before.UserVersion, before.Tables, before.Rows, before.FkViolations, before.StateDist)
	if before.UserVersion != 1 || len(before.States) != 8 {
		t.Fatalf("前置不成立：%+v", before)
	}

	db, err := Open(p)
	if err != nil {
		t.Fatalf("多行多表库 Open 失败: %v", err)
	}
	defer db.Close()

	after := readFacts(t, p)
	t.Logf("抄件迁移后：user_version=%d · 表=%v · 行=%v · 外键违例=%d 行 · 新态分布=%v",
		after.UserVersion, after.Tables, after.Rows, after.FkViolations, after.StateDist)

	if after.UserVersion != 3 {
		t.Errorf("迁移后 user_version 应 3，实得 %d", after.UserVersion)
	}
	if after.FkViolations != 0 {
		t.Errorf("迁移后 foreign_key_check 应 0 行，实得 %d", after.FkViolations)
	}
	if after.Rows["tasks"] != 8 || after.Rows["task_events"] != 3 || after.Rows["idempotency_keys"] != 2 {
		t.Errorf("三表行数应保持 8 / 3 / 2，实得 %v", after.Rows)
	}
	wantDist := map[string]int{"queued": 3, "running": 2, "done": 1, "failed": 1, "cancelled": 1}
	if !reflect.DeepEqual(after.StateDist, wantDist) {
		t.Errorf("新值分布错：实得 %v，要 %v", after.StateDist, wantDist)
	}
	// ★ 语义未变（机器证据）：0002 的 CASE ⟷ 读侧门 NormalizeLegacyState() **逐行同口径**。
	for _, r := range rows {
		if got := after.States[r.id]; got != NormalizeLegacyState(r.state) {
			t.Errorf("行 %s：0002 的 CASE 归一到 %q，而读侧门 NormalizeLegacyState(%q)=%q ⇒ 两处口径漂了",
				r.id, got, r.state, NormalizeLegacyState(r.state))
		}
	}
	if rf := readReworkFields(t, p, "s"); !rf.SteerAt.Valid || rf.SteerAt.Float64 != 7.0 {
		t.Errorf("steered 行的 last_steer_at 应为 7.0，实得 %+v", rf.SteerAt)
	}
	if rf := readReworkFields(t, p, "pz"); rf.Paused != 1 {
		t.Errorf("旧 paused 行的 paused 旗标应置 1，实得 %d", rf.Paused)
	}
	if rf := readReworkFields(t, p, "wr"); rf.NextAttempt.Valid {
		t.Errorf("waiting_retry 行的 next_attempt_at 应为 NULL（0002 填 NULL），实得 %+v", rf.NextAttempt)
	}
	for _, c := range []string{"last_steer_at", "paused_at", "next_attempt_at"} {
		if !hasStr(after.TaskCols, c) {
			t.Errorf("迁移后缺列 %q（实得 %v）", c, after.TaskCols)
		}
	}
	if hasStr(after.TaskCols, "last_steered_at") {
		t.Error("旧列 last_steered_at 仍在 —— 改名没生效")
	}
}

// ─────────── ⑨ T3-c 前置快照硬闸（两态：可写 ⇒ 快照真落且是**迁移前**态 · 做不了 ⇒ 拒启 ＋ uv 不动） ───────────

func TestMigration_SnapshotHardGate(t *testing.T) {
	// ① 可写 ⇒ 迁移前落一份快照（T2 体例：zerg-<UTC>.db ＋ 侧车；内容是**迁移前**态）。
	bdir := filepath.Join(t.TempDir(), "backup")
	p := filepath.Join(t.TempDir(), "legacy.db")
	buildLegacyDB(t, p, []legacyRow{{id: "t1", state: "steered", steer: 5.0}})
	addEventRow(t, p, "t1", 1)
	t.Setenv("ZERG_BACKUP_DIR", bdir)
	start := time.Now()
	db, err := Open(p)
	if err != nil {
		t.Fatalf("快照目录可写时 Open 应过，实得: %v", err)
	}
	_ = db.Close()
	snaps := snapshotFiles(t, bdir)
	if len(snaps) != 1 {
		t.Fatalf("应恰落 1 份前置快照，实得 %d（%v）", len(snaps), snaps)
	}
	var name string
	var mtime time.Time
	for n, m := range snaps {
		name, mtime = n, m
	}
	if mtime.Before(start) {
		t.Errorf("快照 mtime（%v）应晚于迁移前（%v）", mtime, start)
	}
	sp := filepath.Join(bdir, name)
	if _, err := os.Stat(sp + ".meta.json"); err != nil {
		t.Errorf("前置快照缺侧车（Restore 会拒）: %v", err)
	}
	if sf := readFacts(t, sp); sf.UserVersion != 1 {
		t.Errorf("前置快照应是**迁移前**态（user_version=1），实得 %d", sf.UserVersion)
	}
	if f := readFacts(t, p); f.UserVersion != 3 {
		t.Errorf("迁移本体也应落定（user_version=3），实得 %d", f.UserVersion)
	}
	t.Logf("① 快照读数：%s（mtime %v · uv=1 迁移前态 ＋ 侧车在位）", name, mtime)

	// ②-a 快照必失败（**目标已存在**：候选名被占满）⇒ 迁移拒启、user_version 不动。
	p2 := filepath.Join(t.TempDir(), "legacy2.db")
	buildLegacyDB(t, p2, []legacyRow{{id: "t2", state: "queued"}})
	addEventRow(t, p2, "t2", 1)
	full := filepath.Join(t.TempDir(), "backup-full")
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	for i := -5; i <= 130; i++ {
		n, _ := snapshotName(base.Add(time.Duration(i) * time.Second))
		if err := os.WriteFile(filepath.Join(full, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ZERG_BACKUP_DIR", full)
	if _, err := Open(p2); err == nil {
		t.Fatal("快照做不了（候选名撞满 · 目标已存在）却把迁移跑过了 —— 硬闸没生效")
	} else {
		t.Logf("②-a 硬闸拒启读数（目标已存在）：%v", err)
	}
	if f := readFacts(t, p2); f.UserVersion != 1 {
		t.Errorf("硬闸拒了却把 user_version 推到了 %d（应不动）", f.UserVersion)
	}

	// ②-b 快照必失败（**卷不在 / 目录不可写**：父路径是普通件）⇒ 同样拒启、uv 不动。
	p3 := filepath.Join(t.TempDir(), "legacy3.db")
	buildLegacyDB(t, p3, []legacyRow{{id: "t3", state: "queued"}})
	addEventRow(t, p3, "t3", 1)
	notADir := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_BACKUP_DIR", filepath.Join(notADir, "backup"))
	if _, err := Open(p3); err == nil {
		t.Fatal("备份目录建不出来（卷不在 / 不可写）却把迁移跑过了 —— 硬闸没生效")
	} else {
		t.Logf("②-b 硬闸拒启读数（卷不可用）：%v", err)
	}
	if f := readFacts(t, p3); f.UserVersion != 1 {
		t.Errorf("硬闸拒了却把 user_version 推到了 %d（应不动）", f.UserVersion)
	}
}

// ─────────── ⑩ T3④ 真库抄件演练（env 驱动 · 未给 ⇒ Skip，✗ 不静默当通过） ───────────

// ZDB_T3_DRILL_SRC —— 真库的**只读抄件**路径（由仓外驱动命令注入：真库一律 `-readonly` 抄出来）。
func TestMigration_DrillRealCopy(t *testing.T) {
	src := os.Getenv("ZDB_T3_DRILL_SRC")
	if src == "" {
		t.Skip("未给 ZDB_T3_DRILL_SRC（真库抄件）⇒ 本格跳过；仓外演练时由驱动命令注入")
	}
	// ★ 落点**不由本格强钉**：认 TestMain 的隔离前置（未显式指定时钉临时目录），
	// 也认调用方注入的 ZERG_BACKUP_DIR —— 这样「抄件演练同样真落一份**迁移前**快照」可被现读。

	before := readFacts(t, src)
	t.Logf("抄件迁移前：user_version=%d · 表=%v · 行=%v · 外键违例=%d 行 · 态分布=%v",
		before.UserVersion, before.Tables, before.Rows, before.FkViolations, before.StateDist)

	// 抄件的抄件：从源件**只读**再 VACUUM INTO 一份（✗ 全程不写源件）。
	dst := filepath.Join(t.TempDir(), "drill-migrated.db")
	h, err := openSnapshotRO(src)
	if err != nil {
		t.Fatalf("只读开源抄件失败: %v", err)
	}
	if _, err := h.Exec("VACUUM INTO '" + strings.ReplaceAll(dst, "'", "''") + "'"); err != nil {
		h.Close()
		t.Fatalf("VACUUM INTO 抄件失败: %v", err)
	}
	h.Close()

	db, err := Open(dst) // ★ 在抄件上**真跑**迁移器
	if err != nil {
		t.Fatalf("抄件上跑迁移失败: %v", err)
	}
	_ = db.Close()

	after := readFacts(t, dst)
	t.Logf("抄件迁移后：user_version=%d · 表=%v · 行=%v · 外键违例=%d 行 · 态分布=%v",
		after.UserVersion, after.Tables, after.Rows, after.FkViolations, after.StateDist)

	if after.UserVersion != 3 {
		t.Errorf("抄件迁移后 user_version 应 3，实得 %d", after.UserVersion)
	}
	if after.FkViolations != 0 {
		t.Errorf("抄件迁移后 foreign_key_check 应 0 行，实得 %d", after.FkViolations)
	}
	if before.Rows["tasks"] != after.Rows["tasks"] || before.Rows["task_events"] != after.Rows["task_events"] {
		t.Errorf("行数应保持：前 %v → 后 %v", before.Rows, after.Rows)
	}
	// 源抄件一字未动（✗ 不写源件）。
	now := readFacts(t, src)
	if now.UserVersion != before.UserVersion || now.Rows["tasks"] != before.Rows["tasks"] ||
		now.Rows["task_events"] != before.Rows["task_events"] {
		t.Errorf("源抄件被动过：前 %+v → 后 %+v", before, now)
	}
}
