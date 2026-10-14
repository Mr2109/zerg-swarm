package zdb

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ── T1 地基 · 定向判据（§六 Q27 体检补外键 · Q28 身份标 · Q16 读侧另开 handle）──
//
// 夹具复用同包 zdb_test.go 的 newTestDB（✗ 不重复造一套）。

// openRawNoFK —— 开一条**不带 foreign_keys(ON)** 的裸连接（默认 foreign_keys=OFF）。
// 用途 = 造「孤儿 task_events 行」这类外键违例：写池带 FK(ON) 会当场拒 ⇒ 造不出来
// （造不出来正是那枚闸生效的旁证，故这里必须绕开它）。
// 返回值由调用方负责 Close（✗ 不挂 t.Cleanup —— 有的格子要在同一路径上再开写池）。
func openRawNoFK(t *testing.T, path string) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("裸开 %q 失败: %v", path, err)
	}
	return raw
}

// countOpenFDs —— 本进程当前打开的 fd 数（macOS/Linux = /dev/fd 的条目数）。
// ★ 用 `Readdirnames`（✗ 不用 `os.ReadDir` —— 后者对 /dev/fd 的每个条目要 fstatat，
// macOS 上直接回 `bad file descriptor`）。取不到 ⇒ 报 ok=false：那一格只跳 fd 断言，✗ 不整格跳过。
func countOpenFDs(t *testing.T) (int, bool) {
	t.Helper()
	f, err := os.Open("/dev/fd")
	if err != nil {
		return 0, false
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return 0, false
	}
	return len(names), true
}

// fkViolationCount —— `PRAGMA foreign_key_check` 的行数（0 = 合规）。
func fkViolationCount(t *testing.T, h *sql.DB) int {
	t.Helper()
	rows, err := h.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("跑 foreign_key_check 失败: %v", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var tbl, parent string
		var rowid, fkid sql.NullInt64
		if err := rows.Scan(&tbl, &rowid, &parent, &fkid); err != nil {
			t.Fatalf("读 foreign_key_check 结果失败: %v", err)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("foreign_key_check 结果读取中断: %v", err)
	}
	return n
}

// readApplicationID —— 读 `PRAGMA application_id`。
func readApplicationID(t *testing.T, h *sql.DB) int64 {
	t.Helper()
	var v int64
	if err := h.QueryRow("PRAGMA application_id").Scan(&v); err != nil {
		t.Fatalf("读 application_id 失败: %v", err)
	}
	return v
}

// ───────────────────────── T1① 起库体检补 foreign_key_check（§三.7 第 6 条 · §六 Q27） ─────────────────────────

// TestOpen_ForeignKeyCheck_RefusesOrphanRow —— 两态：
//
//	① 干净库 ⇒ Open / HealthCheck 都过；
//	② 同一条库手工塞一条 `task_id` 悬空的 task_events 行（裸连接绕过 FK）⇒ Open **fail-closed**，
//	   且错误串**点名 foreign_key_check**（期望 0 行）。
//
// ★ 同一枚库里 `integrity_check` **照回 ok** —— 这条读数就是「官方：integrity_check 不查外键」的活证
// （缺的正是 Q27 要补的那一条）。
func TestOpen_ForeignKeyCheck_RefusesOrphanRow(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zerg.db")

	db, err := Open(p)
	if err != nil {
		t.Fatalf("干净库应能开: %v", err)
	}
	if err := db.HealthCheck(); err != nil {
		t.Fatalf("干净库体检应过: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}

	raw := openRawNoFK(t, p)
	// 坐实症状：这张库 integrity_check 仍单行 ok……
	var ic string
	if err := raw.QueryRow("PRAGMA integrity_check").Scan(&ic); err != nil {
		t.Fatalf("读 integrity_check 失败: %v", err)
	}
	if ic != "ok" {
		t.Fatalf("前置：本库 integrity_check 应为 ok，实得 %q", ic)
	}
	// ……而外键违例已种下。
	if _, err := raw.Exec(`INSERT INTO task_events(task_id, audit_seq, to_state, ts) VALUES('ghost-task', 1, 'running', 1.0)`); err != nil {
		t.Fatalf("造孤儿 task_events 行失败: %v", err)
	}
	if got := fkViolationCount(t, raw); got != 1 {
		t.Fatalf("前置：foreign_key_check 应报 1 行，实得 %d", got)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("关裸连接失败: %v", err)
	}

	// ② 走开库正门 ⇒ 必须拒（fail-closed）且点名是哪一条体检查出来的。
	_, err = Open(p)
	if err == nil {
		t.Fatal("外键违例库竟被打开 —— foreign_key_check 没兜住（fail-closed 破了）")
	}
	if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("拒启原因应为 ErrIntegrity，实得 %v", err)
	}
	if !strings.Contains(err.Error(), "foreign_key_check") {
		t.Fatalf("错误串应**点名** foreign_key_check（哪一条体检失败），实得:\n%v", err)
	}
	if !strings.Contains(err.Error(), "0 行") {
		t.Fatalf("错误串应写明期望 0 行，实得:\n%v", err)
	}
}

// ───────────────────────── T1② 库身份标 application_id（§三.4 附 G③ · §六 Q28） ─────────────────────────

// TestApplicationID_States —— 身份标逐态一格：
//
//	① 空库首建（值 0 ＋ 库为空）⇒ 体检**放行**（建库途中）；Open 收尾打标 ⇒ 回读 == ZergApplicationID；
//	② 幂等：再开同一枚库 ⇒ 体检过、值一字节不变（ensureApplicationID 空动作）；
//	③′ 值 0 ＋ 库**非**空（本特性之前的旧库）⇒ 体检同样**放行**（「未打标」≠「别人的库」）＋ 写路径补标；
//	④ 非 0 且 ≠ 常量（挂着**别人**的身份标）⇒ 体检 **fail-closed**（ErrNotZergDB）· 走 Open 同样拒启。
func TestApplicationID_States(t *testing.T) {
	dir := t.TempDir()

	// ① 空库首建 ⇒ 放行 ＋ 建后回读常量。
	p := filepath.Join(dir, "fresh.db")
	if _, err := os.Stat(p); err == nil {
		t.Fatal("前置：新鲜库文件本不该存在")
	}
	db, err := Open(p)
	if err != nil {
		t.Fatalf("空库首建应放行: %v", err)
	}
	if got := readApplicationID(t, db.SQL()); got != int64(ZergApplicationID) {
		t.Fatalf("建库后 application_id 应为 %d（0x%X），实得 %d", ZergApplicationID, ZergApplicationID, got)
	}
	if err := db.HealthCheck(); err != nil {
		t.Fatalf("已打标库体检应过: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}

	// ② 幂等：再开 ⇒ 体检过、值原样、ensureApplicationID 空动作。
	db2, err := Open(p)
	if err != nil {
		t.Fatalf("已打标库再开失败: %v", err)
	}
	if got := readApplicationID(t, db2.SQL()); got != int64(ZergApplicationID) {
		t.Fatalf("再开后 application_id 应原样为 %d，实得 %d", ZergApplicationID, got)
	}
	if err := db2.ensureApplicationID(); err != nil {
		t.Fatalf("已打标库 ensureApplicationID 应空动作: %v", err)
	}
	if got := readApplicationID(t, db2.SQL()); got != int64(ZergApplicationID) {
		t.Fatalf("空动作后值不该动，实得 %d", got)
	}
	if err := db2.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}

	// ③「值 0 ＋ 库为空」那一态（同包造裸句柄：空库无用户表 ＋ 值 0）。
	empty := filepath.Join(dir, "empty.db")
	rawEmpty := openRawNoFK(t, empty)
	defer rawEmpty.Close()
	if got := readApplicationID(t, rawEmpty); got != 0 {
		t.Fatalf("前置：空库 application_id 应为 0，实得 %d", got)
	}
	dEmpty := &DB{db: rawEmpty, path: empty}
	if err := dEmpty.applicationIDCheck(); err != nil {
		t.Fatalf("值 0 ＋ 库为空应放行（建库途中），实得 %v", err)
	}
	// 写路径补标：一次写入 ⇒ 回读常量；再调一次 ⇒ 空动作（✗ 不是每次开库无条件写）。
	if err := dEmpty.ensureApplicationID(); err != nil {
		t.Fatalf("补标失败: %v", err)
	}
	if got := readApplicationID(t, rawEmpty); got != int64(ZergApplicationID) {
		t.Fatalf("补标后应回读 %d，实得 %d", ZergApplicationID, got)
	}
	if err := dEmpty.ensureApplicationID(); err != nil {
		t.Fatalf("已打标再调应空动作: %v", err)
	}
	if got := readApplicationID(t, rawEmpty); got != int64(ZergApplicationID) {
		t.Fatalf("空动作后值不该动，实得 %d", got)
	}

	// ③′「值 0 ＋ 库非空」（旧库态）⇒ 同样放行。
	legacy := filepath.Join(dir, "legacy.db")
	rawLegacy := openRawNoFK(t, legacy)
	if _, err := rawLegacy.Exec(`CREATE TABLE legacy_marker(x INTEGER); INSERT INTO legacy_marker VALUES(1);`); err != nil {
		t.Fatalf("造旧库失败: %v", err)
	}
	dLegacy := &DB{db: rawLegacy, path: legacy}
	if err := dLegacy.applicationIDCheck(); err != nil {
		t.Fatalf("值 0 ＋ 库非空（旧库）应放行，实得 %v", err)
	}
	if err := dLegacy.ensureApplicationID(); err != nil {
		t.Fatalf("旧库补标失败: %v", err)
	}
	if got := readApplicationID(t, rawLegacy); got != int64(ZergApplicationID) {
		t.Fatalf("旧库补标后应回读 %d，实得 %d", ZergApplicationID, got)
	}

	// ④ 换成**别人**的非 0 身份标 ⇒ 体检拒启 ＋ 写路径也拒。
	if _, err := rawLegacy.Exec(fmt.Sprintf("PRAGMA application_id = %d", int64(0x12345678))); err != nil {
		t.Fatalf("写异物身份标失败: %v", err)
	}
	if err := dLegacy.applicationIDCheck(); !errors.Is(err, ErrNotZergDB) {
		t.Fatalf("异物身份标应 fail-closed（ErrNotZergDB），实得 %v", err)
	}
	if err := dLegacy.ensureApplicationID(); !errors.Is(err, ErrNotZergDB) {
		t.Fatalf("异物身份标在写路径同样该拒，实得 %v", err)
	}
	if err := rawLegacy.Close(); err != nil {
		t.Fatalf("关裸连接失败: %v", err)
	}
	// 走开库正门同样拒启。
	_, err = Open(legacy)
	if err == nil {
		t.Fatal("异物身份标的库竟被打开 —— application_id 校验没兜住")
	}
	if !errors.Is(err, ErrNotZergDB) {
		t.Fatalf("拒启原因应为 ErrNotZergDB，实得 %v", err)
	}
	if !strings.Contains(err.Error(), "application_id") {
		t.Fatalf("错误串应点名 application_id，实得:\n%v", err)
	}
}

// ───────────────────────── T1③ 读侧另开有界只读 handle（§三.3 条目 1 · §六 Q16） ─────────────────────────

// TestOpenReader_BoundedReadOnlyHandle —— 三态：
//
//	① 写池**逐字不变**：`MaxOpenConnections == 1`（全池 1 连接）＋ DSN 仍带 `_txlock=immediate`；
//	② 读池**另开且只读**：上限 readerMaxConns（4）· DSN **不带 `_txlock`** · `query_only=ON`（真写被拒）
//	   ＋ 4 路并发读全过（读不再排在写池那 1 条连接后面）；
//	③ 关库把**读 / 写两个 handle 一起关**（漏一个就漏 fd）：关后两个句柄都不可用 ＋ 读池连接数归零 ＋ fd 回基线。
func TestOpenReader_BoundedReadOnlyHandle(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zerg.db")

	fdBefore, fdOK := countOpenFDs(t)
	db, err := Open(p)
	if err != nil {
		t.Fatalf("开库失败: %v", err)
	}

	// ① 写池逐字不变。
	if got := db.SQL().Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("写池 MaxOpenConnections 应保持 1（逐字不变），实得 %d", got)
	}
	if w := BuildDSN(p); !strings.Contains(w, "_txlock=immediate") {
		t.Fatalf("写池 DSN 应仍带 _txlock=immediate：%s", w)
	}

	// ② 读池形态。
	if db.Reader() == nil {
		t.Fatal("读 handle 没开（Reader() == nil）")
	}
	if got := db.Reader().Stats().MaxOpenConnections; got != readerMaxConns {
		t.Fatalf("读池连接上限应为 %d，实得 %d", readerMaxConns, got)
	}
	rd := BuildReaderDSN(p)
	if strings.Contains(rd, "_txlock") {
		t.Fatalf("读池 DSN ✗ 不该带 _txlock：%s", rd)
	}
	if !strings.Contains(rd, "query_only(ON)") {
		t.Fatalf("读池 DSN 该带 _pragma=query_only(ON)（只读档）：%s", rd)
	}
	// 只读档真生效：经读 handle 写必须被拒。
	if _, err := db.Reader().Exec(`INSERT INTO tasks(id, slice_id, state, created_at, updated_at) VALUES('r','s','queued',1.0,1.0)`); err == nil {
		t.Fatal("读 handle 竟写进去了 —— 只读档（query_only）没生效")
	}
	// 4 路并发读（另一条路：不再排在写池那 1 条连接后面）。
	var wg sync.WaitGroup
	errs := make([]error, readerMaxConns)
	for i := 0; i < readerMaxConns; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var n int
			errs[i] = db.Reader().QueryRow("SELECT COUNT(*) FROM tasks").Scan(&n)
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("读池并发读 #%d 失败: %v", i, e)
		}
	}
	if fdOK {
		if fdOpen, ok := countOpenFDs(t); ok && fdOpen <= fdBefore {
			t.Fatalf("前置：开库后 fd 应多于基线（读 / 写两 handle 都真开了）· 基线 %d · 现读 %d", fdBefore, fdOpen)
		}
	}

	// ③ 关闭路径：读 / 写两个 handle 一起关。
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}
	var sink int
	if err := db.Reader().QueryRow("SELECT 1").Scan(&sink); err == nil {
		t.Fatal("关库后读 handle 仍能查 —— 读 handle 没跟着关（漏 fd 的口子）")
	}
	if err := db.SQL().QueryRow("SELECT 1").Scan(&sink); err == nil {
		t.Fatal("关库后写 handle 仍能查")
	}
	if got := db.Reader().Stats().OpenConnections; got != 0 {
		t.Fatalf("关库后读池 OpenConnections 应为 0，实得 %d ⇒ 读 handle 没关", got)
	}
	if fdOK {
		if fdAfter, ok := countOpenFDs(t); ok && fdAfter > fdBefore {
			t.Fatalf("关库后 fd 未回基线：开库前 %d · 关库后 %d ⇒ 有 handle 没关", fdBefore, fdAfter)
		}
	}
}
