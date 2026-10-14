package zdb

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// newTestDB —— 在临时目录开一枚新库（隔离；t.Cleanup 关库）。
func newTestDB(t *testing.T) *DB {
	t.Helper()
	p := filepath.Join(t.TempDir(), "zerg.db")
	db, err := Open(p)
	if err != nil {
		t.Fatalf("Open(%s) 失败: %v", p, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustSubmit(t *testing.T, db *DB, tk Task) {
	t.Helper()
	if _, err := db.SubmitTask(tk, "", ""); err != nil {
		t.Fatalf("SubmitTask(%s) 失败: %v", tk.ID, err)
	}
}

// ───────────────────────── ① 建表 + 状态闭集 CHECK ─────────────────────────

// TestSchema_TablesAndStateClosedSet —— 建表：tasks / task_events / idempotency_keys 三表与索引到齐；
// tasks.state 是**闭集 CHECK**（非法态入表必被 DB 拒，✗ 不靠应用层）。
func TestSchema_TablesAndStateClosedSet(t *testing.T) {
	db := newTestDB(t)

	// 三表到齐。
	for _, tbl := range []string{"tasks", "task_events", "idempotency_keys"} {
		var n int
		if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&n); err != nil {
			t.Fatalf("查表 %s 失败: %v", tbl, err)
		}
		if n != 1 {
			t.Fatalf("表 %s 不在库中（sqlite_master 命中 %d）", tbl, n)
		}
	}
	// 索引到齐（按看板真会问的查询建）。
	for _, idx := range []string{"idx_tasks_state", "idx_tasks_state_hb_created", "idx_tasks_caller", "idx_task_events_task_ts"} {
		var n int
		if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, idx).Scan(&n); err != nil {
			t.Fatalf("查索引 %s 失败: %v", idx, err)
		}
		if n != 1 {
			t.Fatalf("索引 %s 不在库中", idx)
		}
	}

	// 闭集：五个合法态都能入；非法态被 DB CHECK 拒。
	for _, s := range ClosedStates {
		if _, err := db.SQL().Exec(`INSERT INTO tasks(id, slice_id, state, created_at, updated_at) VALUES(?,?,?,?,?)`,
			"task-"+s, "slice-1", s, 1.0, 1.0); err != nil {
			t.Fatalf("合法态 %q 竟被拒: %v", s, err)
		}
	}
	_, err := db.SQL().Exec(`INSERT INTO tasks(id, slice_id, state, created_at, updated_at) VALUES(?,?,?,?,?)`,
		"task-bad", "slice-1", "bogus", 1.0, 1.0)
	if err == nil {
		t.Fatal("非法态 'bogus' 竟被 DB 收下 —— state 闭集 CHECK 没生效")
	}
	if !strings.Contains(strings.ToUpper(err.Error()), "CHECK") && !strings.Contains(strings.ToUpper(err.Error()), "CONSTRAINT") {
		t.Fatalf("非法态被拒但报话不像 CHECK（疑别的错）: %v", err)
	}
	// 应用层自查同口径（✗ 不替代 DB CHECK，只是早退）。
	if IsClosedState("bogus") {
		t.Fatal("IsClosedState('bogus') 竟为真")
	}
}

// ───────────────────────── ② 幂等：DB 约束当唯一仲裁点 ─────────────────────────

func TestIdempotency_DBConstraintIsArbiter(t *testing.T) {
	db := newTestDB(t)

	// (a) 直接向 DB 双插同一 (slice_id, idempotency_key)（**绕过应用代码**）⇒ UNIQUE 必须当场拒。
	//     这一条正是「唯一仲裁点是 DB 约束，✗ 不是应用代码」的硬证。
	if _, err := db.SQL().Exec(`INSERT INTO idempotency_keys(slice_id, idempotency_key, fingerprint, created_at, expires_at) VALUES('s','k','fp',1,2)`); err != nil {
		t.Fatalf("首插竟失败: %v", err)
	}
	_, err := db.SQL().Exec(`INSERT INTO idempotency_keys(slice_id, idempotency_key, fingerprint, created_at, expires_at) VALUES('s','k','fp2',1,2)`)
	if err == nil {
		t.Fatal("同键二插竟被收下 —— (slice_id,idempotency_key) UNIQUE 没生效")
	}
	if !strings.Contains(strings.ToUpper(err.Error()), "UNIQUE") {
		t.Fatalf("二插被拒但报话不像 UNIQUE: %v", err)
	}

	// (b) 三态：首同键 ⇒ 新 task_id；二同键同参 ⇒ 回原 task_id（未重复派单）；同键异参 ⇒ 冲突。
	r1, err := db.SubmitTask(Task{ID: "task-a", SliceID: "s", Caller: "hermes"}, "kk", "fpA")
	if err != nil || !r1.Created || r1.TaskID != "task-a" {
		t.Fatalf("① 首同键应新建：got %+v err=%v", r1, err)
	}
	r2, err := db.SubmitTask(Task{ID: "task-b", SliceID: "s", Caller: "hermes"}, "kk", "fpA")
	if err != nil || r2.Created || r2.TaskID != "task-a" {
		t.Fatalf("② 二同键同参应回原 task_id 且未新建：got %+v err=%v", r2, err)
	}
	if _, err := db.SubmitTask(Task{ID: "task-c", SliceID: "s"}, "kk", "fpB"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("③ 同键异参应明报冲突（ErrIdempotencyConflict）：got %v", err)
	}
	if n, _ := db.CountTasks(); n != 1 {
		t.Fatalf("三态走完应只有 1 行任务，实得 %d", n)
	}
	// (a) 已先塞了一行原始键 ('s','k') ⇒ 这里只点 'kk' 这一枚数。
	var kkRows int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM idempotency_keys WHERE slice_id='s' AND idempotency_key='kk'`).Scan(&kkRows); err != nil {
		t.Fatal(err)
	}
	if kkRows != 1 {
		t.Fatalf("同键三态后 'kk' 应仍只有 1 行，实得 %d", kkRows)
	}

	// (c) 并发同键：恰一枚 Created=true（✗ 不是「先查后插」那种会双插的写法）。
	var created int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := db.SubmitTask(Task{ID: fmt.Sprintf("task-c%d", i), SliceID: "s2", Caller: "hermes"}, "ck", "cfp")
			if err == nil && r.Created {
				atomic.AddInt32(&created, 1)
			}
		}(i)
	}
	wg.Wait()
	if got := atomic.LoadInt32(&created); got != 1 {
		t.Fatalf("并发同键应恰 1 枚新建，实得 %d", got)
	}
	var s2Tasks int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM tasks WHERE slice_id='s2'`).Scan(&s2Tasks); err != nil {
		t.Fatal(err)
	}
	if s2Tasks != 1 {
		t.Fatalf("并发同键应只落 1 行任务，实得 %d", s2Tasks)
	}
}

// ───────────────────────── ③ 两表法：改态 + 事件同一事务 ─────────────────────────

func TestApplyStateChange_AtomicTwoTable(t *testing.T) {
	db := newTestDB(t)
	mustSubmit(t, db, Task{ID: "task-1", SliceID: "slice-1", Caller: "hermes"})

	// 正常一改：态与事件同时变。
	seq, hash, err := db.ApplyStateChange(StateChange{TaskID: "task-1", From: "queued", To: "running", Reason: "claim", Actor: "scheduler"})
	if err != nil {
		t.Fatalf("改态失败: %v", err)
	}
	if seq != 2 || hash == "" {
		t.Fatalf("审计锚异常：seq=%d hash=%q（期望 seq=2 且 hash 非空）", seq, hash)
	}
	if st, _ := db.GetTaskState("task-1"); st != "running" {
		t.Fatalf("tasks.state 应为 running，实得 %q", st)
	}
	if n, _ := db.CountEvents("task-1"); n != 2 {
		t.Fatalf("事件应 2 条（submit + claim），实得 %d", n)
	}
	if err := db.VerifyAuditChain("task-1"); err != nil {
		t.Fatalf("审计链四问应过: %v", err)
	}

	// ★ 同事务的牙：摘要函数注入失败 ⇒ 上面已执行的 UPDATE 必须**一并回滚**。
	//   若有人把「改态」与「插事件」拆成两个事务 ⇒ 这里 tasks.state 会变成 done ⇒ 本用例红。
	orig := auditHashFn
	auditHashFn = func(string) (string, error) { return "", errors.New("注入：摘要失败") }
	_, _, err = db.ApplyStateChange(StateChange{TaskID: "task-1", From: "running", To: "done", Reason: "finish", Actor: "worker"})
	auditHashFn = orig
	if err == nil {
		t.Fatal("注入摘要失败后改态竟成功 —— 两表法没在同事务里")
	}
	if st, _ := db.GetTaskState("task-1"); st != "running" {
		t.Fatalf("注入失败后态应回滚为 running，实得 %q（= 两表被拆成两个事务）", st)
	}
	if n, _ := db.CountEvents("task-1"); n != 2 {
		t.Fatalf("注入失败后事件应仍 2 条，实得 %d", n)
	}

	// 非法 to 态 ⇒ 拒，态不变（应用层早退 + DB CHECK 双保险）。
	if _, _, err := db.ApplyStateChange(StateChange{TaskID: "task-1", From: "running", To: "bogus"}); err == nil {
		t.Fatal("非法 to 态竟被接受")
	}
	if st, _ := db.GetTaskState("task-1"); st != "running" {
		t.Fatalf("非法 to 后态应仍 running，实得 %q", st)
	}

	// 乐观冲突：from 与库内不符 ⇒ ErrStateConflict，且不落事件。
	if _, _, err := db.ApplyStateChange(StateChange{TaskID: "task-1", From: "queued", To: "failed"}); !errors.Is(err, ErrStateConflict) {
		t.Fatalf("from 不符应报 ErrStateConflict，实得 %v", err)
	}
	if n, _ := db.CountEvents("task-1"); n != 2 {
		t.Fatalf("乐观冲突不应落事件，实得 %d 条", n)
	}
}

// 变异自证说明（定向单测能红能绿）：
//   - 把 ApplyStateChange 的 UPDATE 与 INSERT 拆成两个 tx ⇒ TestApplyStateChange_AtomicTwoTable 红（态未回滚）。
//   - 把幂等插入的 ON CONFLICT DO NOTHING 换成「先 SELECT 再 INSERT」⇒ 并发用例 created 可能 ≠ 1 ⇒ 红。
//   - 去掉 state 的 CHECK ⇒ TestSchema_TablesAndStateClosedSet 红。

// ───────────────────────── ④ 硬规矩①：卷不在 ⇒ 拒启拒写 ─────────────────────────

func TestVolume_RefuseWhenNotMounted(t *testing.T) {
	cases := []struct {
		name    string
		f       volumeFacts
		wantErr bool
	}{
		{"声称在/Volumes下却落根卷（卷未挂载）", volumeFacts{Path: "<repo>/.runtime/zerg.db", Mount: "/", MountDev: 1, BootDev: 1, FSType: "apfs"}, true},
		{"真挂在外部卷上", volumeFacts{Path: "<volume-path>", Mount: "<volume-path>", MountDev: 2, BootDev: 1, FSType: "apfs"}, false},
		{"网络盘（smbfs）", volumeFacts{Path: "~/.zerg/state/zerg.db", Mount: "/", MountDev: 1, BootDev: 1, FSType: "smbfs"}, true},
		{"网络盘（nfs）", volumeFacts{Path: "/Volumes/NET/x.db", Mount: "/Volumes/NET", MountDev: 3, BootDev: 1, FSType: "nfs"}, true},
		{"exFAT（盘型禁区）", volumeFacts{Path: "/Volumes/X/a.db", Mount: "/Volumes/X", MountDev: 2, BootDev: 1, FSType: "exfat"}, true},
		{"卷挂载点与路径不符", volumeFacts{Path: "<volume-path>", Mount: "/Volumes/OTHER", MountDev: 2, BootDev: 1, FSType: "apfs"}, true},
		{"根卷上普通路径（不受①约束）", volumeFacts{Path: "/tmp/a/zerg.db", Mount: "/", MountDev: 1, BootDev: 1, FSType: "apfs"}, false},
	}
	for _, c := range cases {
		err := checkVolumeFacts(c.f, volumesRoot)
		if c.wantErr && err == nil {
			t.Fatalf("%s：应拒（%v），实得 nil", c.name, c.f)
		}
		if !c.wantErr && err != nil {
			t.Fatalf("%s：应放行，实得 %v", c.name, err)
		}
		if c.wantErr && !errors.Is(err, ErrVolume) {
			t.Fatalf("%s：拒因应为 ErrVolume，实得 %v", c.name, err)
		}
	}
}

// ───────────────────────── ⑤ 硬规矩②：开库先体检 + WAL ─────────────────────────

func TestOpen_IntegrityCheckAndWAL(t *testing.T) {
	db := newTestDB(t)

	var mode string
	if err := db.SQL().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("读 journal_mode 失败: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("WAL 未生效：journal_mode=%q", mode)
	}

	// 坏库 ⇒ fail-closed（✗ 不静默重建空库）。
	badPath := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(badPath, []byte("这不是一个 sqlite 文件，只是一堆垃圾字节…………"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(badPath); err == nil {
		t.Fatal("坏库竟被打开 —— integrity_check 没兜住")
	} else if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("坏库拒启原因应为 ErrIntegrity，实得 %v", err)
	}
}

// ───────────────────────── ⑥ 迁移：版本号拄 user_version + 幂等重跑（上限 3） ─────────────────────────

func TestMigration_VersionPinnedAndIdempotentRerun(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zerg.db")
	db, err := Open(p)
	if err != nil {
		t.Fatalf("首开失败: %v", err)
	}
	v, err := db.UserVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v != 3 {
		t.Fatalf("首开 user_version 应为 3（0001+0002+0003），实得 %d", v)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	// 幂等重跑：再开一次不重跑 DDL、版本稳定、不报错。
	db2, err := Open(p)
	if err != nil {
		t.Fatalf("重开（幂等重跑）失败: %v", err)
	}
	if v2, _ := db2.UserVersion(); v2 != 3 {
		t.Fatalf("重开后 user_version 应为 3，实得 %d", v2)
	}
	if n, _ := db2.CountTasks(); n != 0 {
		t.Fatalf("重开不该动数据，tasks 行数=%d", n)
	}
	// 旧二进制碰新库 ⇒ 拒启（✗ 静默用新 schema）。
	if _, err := db2.SQL().Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	_ = db2.Close()
	if _, err := Open(p); !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("库版本 999 应拒启（ErrSchemaTooNew），实得 %v", err)
	}
}

// ───────────────────────── ⑦ 硬规矩③：备份落另一卷 ─────────────────────────

func TestBackup_DifferentVolumeGuard(t *testing.T) {
	db := newTestDB(t)
	mustSubmit(t, db, Task{ID: "task-1", SliceID: "slice-1", Caller: "hermes"})

	// 同卷（本机临时目录都在同一盘）⇒ 守卫必须拒。
	sameVol := filepath.Join(t.TempDir(), "same.db")
	if err := db.Backup(sameVol); !errors.Is(err, ErrSameVolumeBackup) {
		t.Fatalf("同卷备份应被拒（ErrSameVolumeBackup），实得 %v", err)
	}
	if _, err := os.Stat(sameVol); err == nil {
		t.Fatal("同卷守卫拒了，却仍产出了备份件")
	}

	// VACUUM INTO 本身要真能产可打开的快照（跳过同卷守卫，只验机制）。
	dst := filepath.Join(t.TempDir(), "snap.db")
	if err := db.backupTo(dst, true); err != nil {
		t.Fatalf("VACUUM INTO 快照失败: %v", err)
	}
	// 目标已存在 ⇒ 再备份必拒（VACUUM INTO 要求目标不存在）。
	if err := db.backupTo(dst, true); err == nil {
		t.Fatal("目标已存在竟备份成功")
	}
	// 快照可开且数据在。
	snap, err := Open(dst)
	if err != nil {
		t.Fatalf("打开快照失败: %v", err)
	}
	defer snap.Close()
	if n, _ := snap.CountTasks(); n != 1 {
		t.Fatalf("快照里 tasks 应 1 行，实得 %d", n)
	}
	if st, _ := snap.GetTaskState("task-1"); st != "queued" {
		t.Fatalf("快照里任务态应 queued，实得 %q", st)
	}
}

// ───────────────────────── ⑧ DSN 规格（照设计稿 §三.3 逐条） ─────────────────────────

func TestBuildDSN_FrozenSpec(t *testing.T) {
	got := BuildDSN("<repo>/.runtime/zerg.db")
	// ★ T4c 接线：新建库的写侧 DSN 把 `auto_vacuum(2)` **排在 `journal_mode(WAL)` 之前**
	// （§四.3 第 3 条：auto_vacuum 须在开 WAL 之前、建表之前设；反序 ⇒ 静默失效、回读 0）。
	want := "file:<repo>/.runtime/zerg.db" +
		"?_pragma=auto_vacuum(2)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_txlock=immediate"
	if got != want {
		t.Fatalf("DSN 与设计稿 §三.3 配方不符：\n got=%s\nwant=%s", got, want)
	}
	// ★ 既有库的写侧 DSN（BuildExistingDSN）：与上**逐字同形、唯独不带** `auto_vacuum(2)` ⇒ 存量库不改行为。
	gotExisting := BuildExistingDSN("<repo>/.runtime/zerg.db")
	wantExisting := "file:<repo>/.runtime/zerg.db" +
		"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_txlock=immediate"
	if gotExisting != wantExisting {
		t.Fatalf("既有库 DSN 应为「写侧 DSN 去掉 auto_vacuum(2)」逐字同形：\n got=%s\nwant=%s", gotExisting, wantExisting)
	}
	if strings.Contains(gotExisting, "auto_vacuum") {
		t.Fatalf("既有库 DSN ✗ 不该带 auto_vacuum：%s", gotExisting)
	}
}

// ─────────────────── ⑨ 状态重模（0002）：旧值归一 · 终态不可回 · 五值 CHECK · 幂等重跑 ───────────────────

// legacyStateDDLWidened —— 造「旧库」前态用的 DDL：取内嵌 0001 原文，只把 state 的 CHECK
// 放宽到**旧两代值域的并集**（0001 闭集里的 steered ＋ 旧文件机的 paused/waiting_retry/cancelled）。
// 理由：0002 的归一 SQL 必须对两条血脉都成立，而 0001 的窄 CHECK 结构上不允许插入后三种 ⇒
// 夹具必须放宽才造得出真前态。（0001 本体一字不改 —— 已发布迁移只追加。）
func legacyStateDDLWidened(t *testing.T) string {
	t.Helper()
	rawSQL, err := migrationsFS.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatalf("读内嵌 0001 失败: %v", err)
	}
	old := "CHECK (state IN ('queued','running','steered','done','failed'))"
	if !strings.Contains(string(rawSQL), old) {
		t.Fatalf("夹具锚失效：0001 的 state CHECK 不再是 %q（夹具需随动）", old)
	}
	return strings.Replace(string(rawSQL), old,
		"CHECK (state IN ('queued','running','steered','done','failed','paused','waiting_retry','cancelled'))", 1)
}

// TestState_0002_LegacyValueNormalization —— 旧值归一（单测①）：
// paused/waiting_retry → queued · steered → running · queued/running/done/failed/cancelled 原值保留；
// 三字段（paused_at / last_steer_at / next_attempt_at）在位，且 last_steered_at 已改名（旧列不在），值随行搬过来。
func TestState_0002_LegacyValueNormalization(t *testing.T) {
	p := filepath.Join(t.TempDir(), "legacy.db")
	raw, err := sql.Open("sqlite", BuildDSN(p))
	if err != nil {
		t.Fatalf("开旧库失败: %v", err)
	}
	if _, err := raw.Exec(legacyStateDDLWidened(t)); err != nil {
		t.Fatalf("落旧 DDL 失败: %v", err)
	}
	if _, err := raw.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatalf("钉旧版本号失败: %v", err)
	}
	legacy := []struct{ id, state string }{
		{"t-queued", "queued"}, {"t-running", "running"}, {"t-steered", "steered"},
		{"t-done", "done"}, {"t-failed", "failed"}, {"t-paused", "paused"},
		{"t-waiting", "waiting_retry"}, {"t-cancelled", "cancelled"},
	}
	for _, r := range legacy {
		var pausedAt any
		if r.state == "paused" {
			pausedAt = 11.0
		}
		if _, err := raw.Exec(`INSERT INTO tasks(id, slice_id, state, paused_at, last_steered_at, retry_count, created_at, updated_at)
            VALUES(?,?,?,?,?,?,?,?)`, r.id, "slice-legacy", r.state, pausedAt, 22.0, 3, 1.0, 1.0); err != nil {
			t.Fatalf("造旧行 %s(%s) 失败: %v", r.id, r.state, err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	// 开库 ⇒ 0002 落（旧值归一）。
	db, err := Open(p)
	if err != nil {
		t.Fatalf("Open(旧库) 失败（0002 迁移没过）: %v", err)
	}
	defer db.Close()
	if v, _ := db.UserVersion(); v != 3 {
		t.Fatalf("旧库升到 user_version 应为 3，实得 %d", v)
	}

	want := map[string]string{
		"t-queued": "queued", "t-running": "running", "t-steered": "running",
		"t-done": "done", "t-failed": "failed", "t-paused": "queued",
		"t-waiting": "queued", "t-cancelled": "cancelled",
	}
	for id, w := range want {
		got, err := db.GetTaskState(id)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", id, err)
		}
		if got != w {
			t.Errorf("旧值归一错：%s 期望 %q，实得 %q", id, w, got)
		}
	}

	// 三字段在位（paused_at 保留 · last_steer_at 由 last_steered_at 改名 · next_attempt_at 新加）。
	cols := map[string]bool{}
	rows, err := db.SQL().Query(`SELECT name FROM pragma_table_info('tasks')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		cols[n] = true
	}
	rows.Close()
	for _, c := range []string{"paused_at", "last_steer_at", "next_attempt_at", "retry_count"} {
		if !cols[c] {
			t.Errorf("迁移后缺列 %q", c)
		}
	}
	if cols["last_steered_at"] {
		t.Error("旧列 last_steered_at 仍在 —— 改名没生效")
	}
	// 值随行搬过来（last_steer_at 承接旧 last_steered_at · paused_at 照搬 · 旧 paused 态置旗标）。
	var steer, pausedAt float64
	var pausedFlag int
	if err := db.SQL().QueryRow(`SELECT last_steer_at, paused_at, paused FROM tasks WHERE id='t-paused'`).
		Scan(&steer, &pausedAt, &pausedFlag); err != nil {
		t.Fatal(err)
	}
	if steer != 22.0 || pausedAt != 11.0 || pausedFlag != 1 {
		t.Errorf("搬迁值不符：last_steer_at=%v（要 22）paused_at=%v（要 11）paused=%v（要 1）", steer, pausedAt, pausedFlag)
	}
	// 旧值名一律**不再合法**（闭集换了）⇒ 迁移后再插 steered 必被 CHECK 拒。
	if _, err := db.SQL().Exec(`INSERT INTO tasks(id, slice_id, state, created_at, updated_at) VALUES('t-old','s','steered',1,1)`); err == nil {
		t.Error("迁移后 'steered' 竟还能入表 —— 五值 CHECK 没生效")
	}
}

// TestState_0002_NoTerminalToActive —— **判据「终态不可回活态」**（单测② · 能红能绿）：
// 夹具造终态行（仓外临时目录里的库）⇒ 改回活态 **必拒**且态不变、事件不落；三终态逐格必拒；
// 顺带核配套③（结局词闭集 ⟷ 终态）。哑化 CheckTransition ⇒ 本用例转红。
func TestState_0002_NoTerminalToActive(t *testing.T) {
	db := newTestDB(t)

	// 造终态行：queued → running → done（走真门）。
	mustSubmit(t, db, Task{ID: "t-term", SliceID: "slice-1", Caller: "hermes"})
	if _, _, err := db.ApplyStateChange(StateChange{TaskID: "t-term", From: "queued", To: "running", Reason: "claim", Actor: "scheduler"}); err != nil {
		t.Fatalf("queued→running 应放行: %v", err)
	}
	if _, _, err := db.ApplyStateChange(StateChange{TaskID: "t-term", From: "running", To: "done", Reason: "finish", Actor: "worker"}); err != nil {
		t.Fatalf("running→done 应放行: %v", err)
	}

	// ★ 核心：done → running（判据「终态不可回活态」必红的那一格）。
	_, _, err := db.ApplyStateChange(StateChange{TaskID: "t-term", From: "done", To: "running", Reason: "想把终态拉回活态"})
	if err == nil {
		t.Fatal("done→running 竟被放行 —— 「终态不可回活态」判据没牙")
	}
	if !errors.Is(err, ErrTerminalImmutable) {
		t.Fatalf("done→running 拒因应为 ErrTerminalImmutable，实得 %v", err)
	}
	if st, _ := db.GetTaskState("t-term"); st != "done" {
		t.Fatalf("被拒后态应仍 done，实得 %q", st)
	}
	if n, _ := db.CountEvents("t-term"); n != 3 {
		t.Fatalf("被拒的改态不该落事件：应 3 条（submit+claim+finish），实得 %d", n)
	}

	// 另造两条终态行（failed / cancelled）。
	mustSubmit(t, db, Task{ID: "t-fail", SliceID: "slice-1"})
	if _, _, err := db.ApplyStateChange(StateChange{TaskID: "t-fail", From: "queued", To: "failed", Reason: "timeout"}); err != nil {
		t.Fatalf("queued→failed 应放行: %v", err)
	}
	mustSubmit(t, db, Task{ID: "t-cancel", SliceID: "slice-1"})
	if _, _, err := db.ApplyStateChange(StateChange{TaskID: "t-cancel", From: "queued", To: "cancelled", Reason: "cancel"}); err != nil {
		t.Fatalf("queued→cancelled 应放行: %v", err)
	}

	// 三终态 × 回活态 ＋ 终态→终态：逐格必拒。
	bad := []struct{ id, from, to string }{
		{"t-term", "done", "queued"}, {"t-term", "done", "failed"},
		{"t-fail", "failed", "queued"}, {"t-fail", "failed", "running"},
		{"t-cancel", "cancelled", "queued"}, {"t-cancel", "cancelled", "done"},
	}
	for _, c := range bad {
		_, _, err := db.ApplyStateChange(StateChange{TaskID: c.id, From: c.from, To: c.to})
		if err == nil {
			t.Errorf("%s→%s 竟被放行（终态应只读）", c.from, c.to)
			continue
		}
		if !errors.Is(err, ErrTerminalImmutable) {
			t.Errorf("%s→%s 拒因应为 ErrTerminalImmutable，实得 %v", c.from, c.to, err)
		}
	}
	// 纯判据口（不经 DB）也两态分明。
	if err := CheckTransition("done", "running"); !errors.Is(err, ErrTerminalImmutable) {
		t.Errorf("CheckTransition(done→running) 应判 ErrTerminalImmutable，实得 %v", err)
	}
	if err := CheckTransition("queued", "running"); err != nil {
		t.Errorf("CheckTransition(queued→running) 应放行，实得 %v", err)
	}
	if err := CheckTransition("running", "queued"); err != nil {
		t.Errorf("CheckTransition(running→queued)（软停）应放行，实得 %v", err)
	}
	if err := CheckTransition("paused", "queued"); err == nil {
		t.Error("CheckTransition 竟收旧值名 'paused' —— 写侧只许新闭集")
	}

	// 配套③：结局词 ⟷ 终态（timeout/stalled/max_attempts_exceeded ⇒ failed；cancelled 单独）。
	for _, r := range []string{"timeout", "stalled", "max_attempts_exceeded"} {
		got, ok := TerminalStateForExitReason(r)
		if !ok || got != "failed" {
			t.Errorf("结局词 %q 应映射 failed，实得 (%q,%v)", r, got, ok)
		}
	}
	if got, ok := TerminalStateForExitReason("cancelled"); !ok || got != "cancelled" {
		t.Errorf("结局词 'cancelled' 应映射 cancelled（单独一档），实得 (%q,%v)", got, ok)
	}
	if got, ok := TerminalStateForExitReason("bogus"); ok {
		t.Errorf("闭集外结局词不该给结论，实得 (%q,%v)", got, ok)
	}
}

// TestState_0002_FiveValueCheck —— 五值闭集 CHECK（单测③）：五值各能入；旧值名与任意垃圾一律被 DB 拒；
// 读/写侧门（NormalizeLegacyState / InsertTask）与 DB CHECK 同口径。
func TestState_0002_FiveValueCheck(t *testing.T) {
	db := newTestDB(t)
	want := []string{"queued", "running", "done", "failed", "cancelled"}
	if len(ClosedStates) != len(want) {
		t.Fatalf("闭集应为五值，实得 %d：%v", len(ClosedStates), ClosedStates)
	}
	for i, s := range want {
		if ClosedStates[i] != s {
			t.Errorf("闭集第 %d 格应为 %q，实得 %q", i, s, ClosedStates[i])
		}
		if !IsClosedState(s) {
			t.Errorf("IsClosedState(%q) 应为真", s)
		}
		if _, err := db.SQL().Exec(`INSERT INTO tasks(id, slice_id, state, created_at, updated_at) VALUES(?,?,?,?,?)`,
			"task-"+s, "slice-1", s, 1.0, 1.0); err != nil {
			t.Errorf("合法态 %q 竟被拒: %v", s, err)
		}
	}
	// 旧值名（steered/paused/waiting_retry）与垃圾：DB CHECK 必拒。
	for _, bd := range []string{"steered", "paused", "waiting_retry", "unknown", "cancelling", "bogus", ""} {
		_, err := db.SQL().Exec(`INSERT INTO tasks(id, slice_id, state, created_at, updated_at) VALUES(?,?,?,?,?)`,
			"task-bad-"+bd, "slice-1", bd, 1.0, 1.0)
		if err == nil {
			t.Errorf("非法/旧值态 %q 竟被 DB 收下 —— CHECK 没生效", bd)
			continue
		}
		if !strings.Contains(strings.ToUpper(err.Error()), "CHECK") && !strings.Contains(strings.ToUpper(err.Error()), "CONSTRAINT") {
			t.Errorf("%q 被拒但报话不像 CHECK: %v", bd, err)
		}
	}
	// 终态/活态判定同口径。
	for _, s := range TerminalStates {
		if !IsTerminalState(s) || IsActiveState(s) {
			t.Errorf("%q 应判终态、非活态", s)
		}
	}
	for _, s := range ActiveStates {
		if IsTerminalState(s) || !IsActiveState(s) {
			t.Errorf("%q 应判活态、非终态", s)
		}
	}
	// 旧值归一（纯函数）逐值。
	for _, c := range []struct{ in, out string }{
		{"paused", "queued"}, {"waiting_retry", "queued"}, {"steered", "running"},
		{"queued", "queued"}, {"running", "running"}, {"done", "done"},
		{"failed", "failed"}, {"cancelled", "cancelled"},
	} {
		if got := NormalizeLegacyState(c.in); got != c.out {
			t.Errorf("NormalizeLegacyState(%q) 应为 %q，实得 %q", c.in, c.out, got)
		}
	}
	// 写侧门：InsertTask 只收新闭集（旧值名早退 + 报话点名归一）。
	err := db.InsertTask(Task{ID: "t-legacy", SliceID: "s", State: "paused"})
	if err == nil {
		t.Error("InsertTask 竟收旧值态 'paused'")
	} else if !strings.Contains(err.Error(), "NormalizeLegacyState") {
		t.Errorf("InsertTask 对旧值名的报话应点名 NormalizeLegacyState，实得 %v", err)
	}
	if err := db.InsertTask(Task{ID: "t-ok", SliceID: "s"}); err != nil {
		t.Errorf("默认态（queued）的 InsertTask 不该失败: %v", err)
	}
}

// TestMigration_0002_IdempotentRerun —— 幂等重跑（单测④）：连开三次，user_version 恒 3、数据不丢、
// schema 签名逐字节不变（＝没重跑 DDL）；上限抬到 3 后「库版本高于上限 ⇒ 拒启」口径依旧成立。
func TestMigration_0002_IdempotentRerun(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zerg.db")
	var lastSig string
	for i := 1; i <= 3; i++ {
		db, err := Open(p)
		if err != nil {
			t.Fatalf("第 %d 次 Open 失败: %v", i, err)
		}
		v, err := db.UserVersion()
		if err != nil {
			t.Fatal(err)
		}
		if v != 3 {
			t.Fatalf("第 %d 次 Open 后 user_version 应为 3，实得 %d", i, v)
		}
		if i == 1 {
			mustSubmit(t, db, Task{ID: "t-idem", SliceID: "slice-1"})
			lastSig = tableSig(t, db)
		} else {
			if n, _ := db.CountTasks(); n != 1 {
				t.Fatalf("第 %d 次 Open 后 tasks 应仍 1 行，实得 %d", i, n)
			}
			if st, _ := db.GetTaskState("t-idem"); st != "queued" {
				t.Fatalf("第 %d 次 Open 后态应仍 queued，实得 %q", i, st)
			}
			if sig := tableSig(t, db); sig != lastSig {
				t.Fatalf("幂等重跑改了 schema：第 %d 次签名 ≠ 首次（说明 DDL 被重跑）", i)
			}
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// 版本高于本二进制上限 ⇒ 拒启。
	db, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL().Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	if _, err := Open(p); !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("库版本 999 应拒启（ErrSchemaTooNew），实得 %v", err)
	}
}

// tableSig —— tasks 表的 schema 签名（建表原文 ＋ 列序/类型），用来证「幂等重跑没重跑 DDL」。
func tableSig(t *testing.T, db *DB) string {
	t.Helper()
	var sqlText string
	if err := db.SQL().QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='tasks'`).Scan(&sqlText); err != nil {
		t.Fatalf("读 tasks 建表原文失败: %v", err)
	}
	rows, err := db.SQL().Query(`SELECT name, type FROM pragma_table_info('tasks')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var sb strings.Builder
	sb.WriteString(sqlText)
	for rows.Next() {
		var n, ty string
		if err := rows.Scan(&n, &ty); err != nil {
			t.Fatal(err)
		}
		sb.WriteString("|" + n + ":" + ty)
	}
	return sb.String()
}
