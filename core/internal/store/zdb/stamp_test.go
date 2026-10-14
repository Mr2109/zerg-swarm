package zdb

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ── E3-a 定向判据：`StampApplicationID`（**只打标不迁移** · 回读自证 · 非 0 他值拒执）──
//
// 四格（夹具全在 `t.TempDir()` 的仓外临时目录里）：
//
//	① 建库路径产出的库（已打标）⇒ 空动作（sha256 一字不动）；
//	② `application_id=0` 且有数据的库 ⇒ 打成常量 · `user_version` 与 schema 一字不动（**证没顺带迁移**）；
//	③ `application_id` = 他值 ⇒ **拒执**（`ErrNotZergDB`）· 库一字未动；
//	④ 库不在盘 ⇒ 拒执且**不种空库**（`sql.Open` 的惰性建库不许被放任）。

// stampSha256Hex —— 一件的 sha256（十六进制小写）。
func stampSha256Hex(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读件 %s 失败: %v", p, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// stampOpenRaw —— 造夹具用的裸连接（默认 journal 模式 · 不带 foreign_keys/WAL 旋钮）。
// 返回值由调用方关闭（✗ 不挂 t.Cleanup —— 同一路径上还要再开别的句柄）。
func stampOpenRaw(t *testing.T, p string) *sql.DB {
	t.Helper()
	h, err := sql.Open("sqlite", "file:"+p)
	if err != nil {
		t.Fatalf("裸开 %q 失败: %v", p, err)
	}
	return h
}

// stampMakeDataDB —— 造一枚「有数据」的库：一张 probe 表 ＋ 3 行 ＋ 指定的 `application_id` / `user_version`。
// `appID == 0` ⇒ 不打身份标（真值就是库头的 0）。
func stampMakeDataDB(t *testing.T, p string, appID, userVersion int64) {
	t.Helper()
	h := stampOpenRaw(t, p)
	defer h.Close()
	if _, err := h.Exec(`CREATE TABLE probe(id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	for i, v := range []string{"alpha", "beta", "gamma"} {
		if _, err := h.Exec(`INSERT INTO probe(id, v) VALUES(?, ?)`, i+1, v); err != nil {
			t.Fatalf("插行失败: %v", err)
		}
	}
	if _, err := h.Exec("PRAGMA user_version = " + strconv.FormatInt(userVersion, 10)); err != nil {
		t.Fatalf("设 user_version 失败: %v", err)
	}
	if appID != 0 {
		if _, err := h.Exec("PRAGMA application_id = " + strconv.FormatInt(appID, 10)); err != nil {
			t.Fatalf("设 application_id 失败: %v", err)
		}
	}
}

// stampReadInt —— 读一条 PRAGMA 的整数标量。
func stampReadInt(t *testing.T, p, pragma string) int64 {
	t.Helper()
	h := stampOpenRaw(t, p)
	defer h.Close()
	var v int64
	if err := h.QueryRow(pragma).Scan(&v); err != nil {
		t.Fatalf("读 %s 失败: %v", pragma, err)
	}
	return v
}

// stampProbeRows —— probe 表的行数（打标不许动数据）。
func stampProbeRows(t *testing.T, p string) int {
	t.Helper()
	h := stampOpenRaw(t, p)
	defer h.Close()
	var n int
	if err := h.QueryRow(`SELECT COUNT(*) FROM probe`).Scan(&n); err != nil {
		t.Fatalf("数 probe 行失败: %v", err)
	}
	return n
}

// stampSchemaNames —— sqlite_master 里的对象名（排序后）——「有没有顺带跑迁移」的现读判据。
func stampSchemaNames(t *testing.T, p string) []string {
	t.Helper()
	h := stampOpenRaw(t, p)
	defer h.Close()
	rows, err := h.Query(`SELECT name FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("读 sqlite_master 失败: %v", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("读 sqlite_master 行失败: %v", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("sqlite_master 结果读取中断: %v", err)
	}
	sort.Strings(out)
	return out
}

// ① 建库路径产出的库（已打标）⇒ 空动作：sha256 逐字不动。
func TestStampApplicationID_CreatePathDBIsNoOp(t *testing.T) {
	p := filepath.Join(t.TempDir(), "zerg.db")
	db, err := Open(p)
	if err != nil {
		t.Fatalf("Open(%s) 失败: %v", p, err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关库失败: %v", err)
	}
	// 建库路径已打标（本特性没变的那条路）。
	if got := stampReadInt(t, p, "PRAGMA application_id"); got != int64(ZergApplicationID) {
		t.Fatalf("建库路径产出的库 application_id = %d（期望本标 %d）—— 建库路径的现状被改了",
			got, int64(ZergApplicationID))
	}
	shaBefore := stampSha256Hex(t, p)
	before, after, err := StampApplicationID(p)
	if err != nil {
		t.Fatalf("已打标的库竟报错: %v", err)
	}
	if before != ZergApplicationID || after != ZergApplicationID {
		t.Fatalf("空动作的回读两值要都 = 本标 %d，实得 before=%d after=%d", ZergApplicationID, before, after)
	}
	if got := stampSha256Hex(t, p); got != shaBefore {
		t.Fatalf("已打标的库被写了（sha256 前 %s ≠ 后 %s）—— 空动作必须一字节不写", shaBefore[:16], got[:16])
	}
}

// ② `application_id=0` 且有数据的库 ⇒ 打成常量且 `user_version` / schema / 数据一字不动（证没顺带迁移）。
func TestStampApplicationID_ZeroWithDataStampsKeepsUserVersion(t *testing.T) {
	p := filepath.Join(t.TempDir(), "old.db")
	stampMakeDataDB(t, p, 0, 1)
	if got := stampReadInt(t, p, "PRAGMA application_id"); got != 0 {
		t.Fatalf("夹具前提不成立：application_id = %d（要 0 = 未打标）", got)
	}
	schemaBefore := strings.Join(stampSchemaNames(t, p), ",")
	rowsBefore := stampProbeRows(t, p)

	before, after, err := StampApplicationID(p)
	if err != nil {
		t.Fatalf("未打标的库补标失败: %v", err)
	}
	if before != 0 {
		t.Fatalf("改前值要 = 0（回读得来），实得 %d", before)
	}
	if after != ZergApplicationID {
		t.Fatalf("改后值要 = 本标 %d，实得 %d", ZergApplicationID, after)
	}
	// 回读三件：身份标已落 · user_version 一字不动 · schema 一字不动（= 没顺带迁移）。
	if got := stampReadInt(t, p, "PRAGMA application_id"); got != int64(ZergApplicationID) {
		t.Fatalf("补标后 application_id = %d（期望 %d）", got, int64(ZergApplicationID))
	}
	if got := stampReadInt(t, p, "PRAGMA user_version"); got != 1 {
		t.Fatalf("补标把 user_version 改了（现读 %d，期望 1）—— 打标不得顺带迁移", got)
	}
	// schema 与数据都要一字不动（迁移若跑过，tasks / task_events / idempotency_keys 会冒出来）。
	schemaAfter := strings.Join(stampSchemaNames(t, p), ",")
	if schemaAfter != schemaBefore {
		t.Fatalf("补标动了 schema：%q → %q（怀疑顺带跑了 migrate）", schemaBefore, schemaAfter)
	}
	if strings.Contains(schemaAfter, "tasks") {
		t.Fatalf("补标后出现迁移件建的表：%q（= 顺带跑了 migrate）", schemaAfter)
	}
	rowsAfter := stampProbeRows(t, p)
	if rowsAfter != rowsBefore {
		t.Fatalf("补标动了数据：probe 行数 %d → %d", rowsBefore, rowsAfter)
	}
	// 幂等：再打一次 ⇒ 空动作（两值都 = 本标）。
	b2, a2, err2 := StampApplicationID(p)
	if err2 != nil {
		t.Fatalf("二次打标报错: %v", err2)
	}
	if b2 != ZergApplicationID || a2 != ZergApplicationID {
		t.Fatalf("二次打标要空动作（两值 = 本标），实得 before=%d after=%d", b2, a2)
	}
}

// ③ `application_id` = 他值 ⇒ 拒执（ErrNotZergDB）· 库一字未动。
func TestStampApplicationID_RefusesForeignID(t *testing.T) {
	p := filepath.Join(t.TempDir(), "foreign.db")
	const foreign int64 = 0x12345678
	stampMakeDataDB(t, p, foreign, 1)
	shaBefore := stampSha256Hex(t, p)

	before, after, err := StampApplicationID(p)
	if err == nil {
		t.Fatalf("挂着别人身份标的库竟被打了标（before=%d after=%d）—— 非 0 他值必须拒执", before, after)
	}
	if !errors.Is(err, ErrNotZergDB) {
		t.Fatalf("拒执的错误类要 ErrNotZergDB，实得: %v", err)
	}
	if got := stampReadInt(t, p, "PRAGMA application_id"); got != foreign {
		t.Fatalf("拒执却把库改了：application_id = %d（期望原样 %d）", got, foreign)
	}
	if got := stampSha256Hex(t, p); got != shaBefore {
		t.Fatalf("拒执却写了库（sha256 前 %s ≠ 后 %s）", shaBefore[:16], got[:16])
	}
}

// ④ 库不在盘 ⇒ 拒执且**不种空库**。
func TestStampApplicationID_MissingFileRefusedWithoutPhantom(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.db")
	_, _, err := StampApplicationID(p)
	if err == nil {
		t.Fatal("库不在盘却报成功 —— 补标入口不许建库")
	}
	if _, serr := os.Stat(p); serr == nil {
		t.Fatalf("拒执却在盘上种出了空库：%s（`sql.Open` 的惰性建库没被拦住）", p)
	}
}
