// family_core_db_stamp_test.go —— `zerg core db stamp` 的定向判据（E3-b：薄壳转发 `zdb.StampApplicationID`）。
//
// 落点：`package main_test` + `zerg.RunForTest`（跑**当前源码**行为 · 与 `main()` 同一个 `run`）。
// 夹具全在 `t.TempDir()` 的仓外临时目录里 ⇒ 不碰真库、不碰真仓。
//
// 五格：
//
//	① 缺 `--yes`（写面确认档）⇒ 退 2 且库一字未动；
//	② `--dry-run` ⇒ 退 0 · 只出计划件 · 库一字未动（零副作用）；
//	③ `application_id=0` 且有数据的库 ⇒ `--yes` 真跑：退 0 · 打成常量 · `user_version` 仍 = 1；
//	④ `application_id` = 他值 ⇒ 退 1（拒执）· 库一字未动；
//	⑤ `help` 用法面点得到新动作（闸A「面 ⟷ 命令树 usage 逐字」的可读侧）。
package main_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"

	_ "modernc.org/sqlite"
)

// cdsRun —— 进程内跑一条命令（照同包 `runZergRepo` 的形态 · 本件只需 `--db` 一族旋钮）。
func cdsRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// cdsMakeDB —— 造一枚夹具库：probe 表 ＋ 3 行 ＋ `user_version=1` ＋ 指定 `application_id`。
func cdsMakeDB(t *testing.T, p string, appID int64) {
	t.Helper()
	h, err := sql.Open("sqlite", "file:"+p)
	if err != nil {
		t.Fatalf("开夹具库失败: %v", err)
	}
	defer h.Close()
	if _, err := h.Exec(`CREATE TABLE probe(id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	if _, err := h.Exec(`INSERT INTO probe(id, v) VALUES(1, 'alpha'), (2, 'beta'), (3, 'gamma')`); err != nil {
		t.Fatalf("插行失败: %v", err)
	}
	if _, err := h.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatalf("设 user_version 失败: %v", err)
	}
	if appID != 0 {
		if _, err := h.Exec("PRAGMA application_id = " + cdsItoa(appID)); err != nil {
			t.Fatalf("设 application_id 失败: %v", err)
		}
	}
}

// cdsItoa —— 十进制串（✗ 不引 strconv 只为一次格式化）。
func cdsItoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// cdsReadInt —— 读一条 PRAGMA 的整数标量。
func cdsReadInt(t *testing.T, p, pragma string) int64 {
	t.Helper()
	h, err := sql.Open("sqlite", "file:"+p)
	if err != nil {
		t.Fatalf("开库读 %s 失败: %v", pragma, err)
	}
	defer h.Close()
	var v int64
	if err := h.QueryRow(pragma).Scan(&v); err != nil {
		t.Fatalf("读 %s 失败: %v", pragma, err)
	}
	return v
}

// cdsBytes —— 一件的字节（⩽ 0 = 不在盘）。
func cdsBytes(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return -1
	}
	return st.Size()
}

// ⑤ 用法面：`core db` 的用法串点得到新动作 `stamp`（闸A 的可读侧：面 ⟷ 命令树 usage 逐字）。
func TestCoreDbStamp_UsageFaceDeclaresStamp(t *testing.T) {
	rc, out, errb := cdsRun(t, "core", "db", "--help")
	if rc != 0 {
		t.Fatalf("`core db --help` 退码要 0，实得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(out, "<check|backup|prune|stamp>") {
		t.Errorf("`core db` 的用法串里没有新动作 stamp（实得首行：%q）", cdsFirstLine(out))
	}
	if !strings.Contains(out, "zerg core db") {
		t.Errorf("`core db --help` 没印该条用法串：%q", cdsFirstLine(out))
	}
}

// ① 缺 `--yes` ⇒ 退 2（写面确认档 · fail-closed）且库一字未动。
func TestCoreDbStamp_YesRequired(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.db")
	cdsMakeDB(t, p, 0)
	before := cdsBytes(p)
	rc, _, errb := cdsRun(t, "core", "db", "stamp", "--db", p)
	if rc != 2 {
		t.Fatalf("缺 --yes 要退 2（fail-closed），实得 %d", rc)
	}
	if !strings.Contains(errb, "--yes") {
		t.Errorf("拒执报话要点名 --yes，实得: %q", errb)
	}
	if got := cdsReadInt(t, p, "PRAGMA application_id"); got != 0 {
		t.Fatalf("缺 --yes 却把库改了：application_id = %d（期望 0）", got)
	}
	if after := cdsBytes(p); after != before {
		t.Fatalf("缺 --yes 却动了库件（%d → %d 字节）", before, after)
	}
}

// ② `--dry-run` ⇒ 退 0 · 出计划件 · 零副作用。
func TestCoreDbStamp_DryRunIsNoOp(t *testing.T) {
	p := filepath.Join(t.TempDir(), "b.db")
	cdsMakeDB(t, p, 0)
	before := cdsBytes(p)
	rc, out, errb := cdsRun(t, "core", "db", "stamp", "--db", p, "--dry-run")
	if rc != 0 {
		t.Fatalf("`stamp --dry-run` 要退 0，实得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(out, "计划件") || !strings.Contains(out, "只打标不迁移") {
		t.Errorf("干跑要出计划件（含「只打标不迁移」），实得: %q", cdsFirstLine(out))
	}
	if got := cdsReadInt(t, p, "PRAGMA application_id"); got != 0 {
		t.Fatalf("干跑却打了标：application_id = %d（期望 0）", got)
	}
	if after := cdsBytes(p); after != before {
		t.Fatalf("干跑却动了库件（%d → %d 字节）", before, after)
	}
}

// ③ 真跑（`--yes`）：`application_id=0` ⇒ 打成常量 · `user_version` 仍 = 1 · 机器面读得到。
func TestCoreDbStamp_WritesStampKeepsUserVersion(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.db")
	cdsMakeDB(t, p, 0)
	rc, out, errb := cdsRun(t, "core", "db", "stamp", "--db", p, "--yes")
	if rc != 0 {
		t.Fatalf("`stamp --yes` 要退 0，实得 %d（stderr=%q）", rc, errb)
	}
	want := "1514492487"
	if got := cdsReadInt(t, p, "PRAGMA application_id"); got != 1514492487 {
		t.Fatalf("打标后 application_id = %d（期望 %s = 0x5A455247）", got, want)
	}
	if got := cdsReadInt(t, p, "PRAGMA user_version"); got != 1 {
		t.Fatalf("打标把 user_version 改了（现读 %d，期望 1）—— 打标不得顺带迁移", got)
	}
	if !strings.Contains(out, "改前 / 改后") || !strings.Contains(out, "1514492487") {
		t.Errorf("人面要印「改前 / 改后」两值，实得: %q", cdsHeadLines(out, 6))
	}
	// 机器面（复用既有字段面 —— ✗ 不为本动作新增字段）：身份标已落 · user_version 仍 1 · 判词 ok。
	rc2, out2, _ := cdsRun(t, "core", "db", "stamp", "--db", p, "--yes",
		"--json", "action,db,application_id,application_id_ok,user_version,integrity_check,foreign_key_violations,verdict")
	if rc2 != 0 {
		t.Fatalf("二次真跑（已打标 ⇒ 空动作）要退 0，实得 %d", rc2)
	}
	var doc struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(out2), &doc); err != nil {
		t.Fatalf("`--json` 包封解不动：%v · %q", err, out2)
	}
	if len(doc.Items) != 1 {
		t.Fatalf("包封 items 要 1 条，实得 %d", len(doc.Items))
	}
	it := doc.Items[0]
	if it["action"] != "stamp" {
		t.Errorf("action = %q（要 stamp）", it["action"])
	}
	if it["application_id"] != want {
		t.Errorf("application_id = %q（要 %s）", it["application_id"], want)
	}
	if it["application_id_ok"] != "true" {
		t.Errorf("application_id_ok = %q（要 true）", it["application_id_ok"])
	}
	if it["user_version"] != "1" {
		t.Errorf("user_version = %q（要 1 —— 打标不得顺带迁移）", it["user_version"])
	}
	if it["integrity_check"] != "ok" {
		t.Errorf("integrity_check = %q（要 ok）", it["integrity_check"])
	}
	if it["foreign_key_violations"] != "0" {
		t.Errorf("foreign_key_violations = %q（要 0）", it["foreign_key_violations"])
	}
	if it["verdict"] != "ok" {
		t.Errorf("verdict = %q（要 ok）", it["verdict"])
	}
}

// ④ 他值身份标 ⇒ 退 1（拒执 · fail-closed）· 库一字未动。
func TestCoreDbStamp_RefusesForeignID(t *testing.T) {
	p := filepath.Join(t.TempDir(), "d.db")
	const foreign int64 = 0x12345678
	cdsMakeDB(t, p, foreign)
	before := cdsBytes(p)
	rc, _, errb := cdsRun(t, "core", "db", "stamp", "--db", p, "--yes")
	if rc != 1 {
		t.Fatalf("挂着别人身份标要退 1（判红/拒执），实得 %d（stderr=%q）", rc, errb)
	}
	if got := cdsReadInt(t, p, "PRAGMA application_id"); got != foreign {
		t.Fatalf("拒执却把库改了：application_id = %d（期望原样 %d）", got, foreign)
	}
	if after := cdsBytes(p); after != before {
		t.Fatalf("拒执却动了库件（%d → %d 字节）", before, after)
	}
}

// cdsFirstLine / cdsHeadLines —— 报错时给人看的短样（✗ 不整段回显）。
func cdsFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func cdsHeadLines(s string, n int) string {
	parts := strings.Split(s, "\n")
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, "\\n")
}
