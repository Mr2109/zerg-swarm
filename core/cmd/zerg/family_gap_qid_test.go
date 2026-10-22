// family_gap_qid_test.go —— 缺口 `GAP-20260926-70`（真源无 `qid` 字段 ⇒ 缺口 ↔ 任务无机器可读对应）的**回归牙**。
//
// 病（改前现读）：
//
//	① `zerg gap ls --json qid` ⇒ rc=2「未知字段」（合法字段表里没有 `qid`）—— 缺口账自己的判据串
//	   就是这么写的，等于判据现读就答不对；
//	② 真源那一行**没有 qid 这一格** ⇒ 版账里的 `Q-NNN` 号与真源的 `GAP-…` id 之间没有任何机器面
//	   的对应，「映射完整」判不出来（设计稿 `设计-缺口账合一-Q行搬jsonl-20261008.md` §四「将加」）。
//
// 治（本件钉住的四件事）：
//
//	① **只读面**：`gap ls --json qid` / `gap show <id> --json qid` 收下并回吐；老行（没有这一格）
//	   照旧 **rc=0**、键**不出**（空值整键略去 ⇒ 老账的既有 `--json` 档逐字节不变）；
//	② **反查轴**：`gap ls --qid <Q-NNN>`（号 → 缺口）；裸给 / 空值 / 形态不对 ⇒ 2（不静默当「不过滤」）；
//	③ **写面**：`gap add --qid <Q-NNN> [--qid-source <锚>]` 真把号落进新行（形态 + 全局唯一两闸，
//	   不合 ⇒ 2 且**一个字节不写**）；不给 `--qid` 的老调用方**行为一字不动**；
//	④ **不改存量**：新行落 qid 时，账内既有行**逐字节不变**；`gapFingerprint` / `gapSameContent`
//	   都不吃这两格（老账的 fp 全部不变 ⇒ 幂等面不因加格而漂）。
//
// ★ 夹具全部落 `t.TempDir()`（`ZERG_STATE_DIR`）⇒ **不碰真账 `~/.zerg/state`**、不碰任何仓内件。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// gapQIDTestSeed —— 造一份合成账（每行一条 JSON 原串），返回账件路径。**不碰真账**。
func gapQIDTestSeed(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	p := gapLedgerPath()
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("seed 真源失败：%v", err)
	}
	return p
}

// gapQIDTestRun —— 走**同一条** `run` 入口真跑（真退码 · 真 stdout / stderr）。
func gapQIDTestRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := run(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// gapQIDTestJSON —— 解析机器面信封，返回 `items` 那一串对象。
func gapQIDTestJSON(t *testing.T, s string) []map[string]any {
	t.Helper()
	var env struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(s), &env); err != nil {
		t.Fatalf("机器面不是合法 JSON：%v\n原文=%s", err, s)
	}
	return env.Items
}

// gapQIDTestReadLines —— 现读账件全文（逐字节对拍用）。
func gapQIDTestReadLines(t *testing.T, p string) []string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读账失败：%v", err)
	}
	all := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	return all
}

// 一行**老账**（没有 qid / qid_source 两格）—— 本单所有兼容判据的基线行。
const gapQIDTestOldRow = `{"id":"GAP-20260926-70","fp":"fp70","symptom":"真源无 qid 字段","handmade":"现读 rows 无 qid","impact":"文档面","prio":"P2","state":"仍缺","want_family":"zerg","want_action":"gap-qid-mapping","repro_cmd":"zerg task ls","verify_cmd":"zerg gap ls --qid Q-216","found_at":"2026-09-26T02:09:50+08:00","unit":"无件(命令面)","module":"无件","unit_source":"hand"}`

// TestGapQID_LegacyRowReadsWithoutError —— ① 老行：`--json qid` 收下、键不出、退码 0（**修复前这一发 rc=2**）。
func TestGapQID_LegacyRowReadsWithoutError(t *testing.T) {
	gapQIDTestSeed(t, gapQIDTestOldRow)

	rc, out, errs := gapQIDTestRun(t, "gap", "ls", "--json", "id,qid")
	if rc != exitOK {
		t.Fatalf("`gap ls --json id,qid` 必须收下（缺口 want 的判据串）：rc=%d stdout=%q stderr=%q", rc, out, errs)
	}
	if strings.Contains(errs, "未知字段") {
		t.Fatalf("老行不许把 qid 当未知字段：stderr=%q", errs)
	}
	items := gapQIDTestJSON(t, out)
	if len(items) != 1 {
		t.Fatalf("应恰好 1 条，实得 %d 条：%s", len(items), out)
	}
	if got, ok := items[0]["id"]; !ok || got != "GAP-20260926-70" {
		t.Fatalf("id 格不对：%v（原文 %s）", items[0], out)
	}
	// 空值整键略去（与 API 的 omitempty 同一条铁律）⇒ 老行**不出** qid 键。
	if _, ok := items[0]["qid"]; ok {
		t.Fatalf("老行（无 qid 格）不许吐 qid 键（空值整键略去）：%s", out)
	}

	// 单条面同一条口径。
	rc, out, errs = gapQIDTestRun(t, "gap", "show", "GAP-20260926-70", "--json", "id,qid,qid_source")
	if rc != exitOK {
		t.Fatalf("`gap show … --json qid,qid_source` 必须收下：rc=%d stderr=%q", rc, errs)
	}
	items = gapQIDTestJSON(t, out)
	if len(items) != 1 {
		t.Fatalf("gap show 应恰好 1 条：%s", out)
	}
	if _, ok := items[0]["qid"]; ok {
		t.Fatalf("老行 gap show 不许吐 qid 键：%s", out)
	}

	// 人面：没有号 ⇒ 不许多出「任务号」那一行（老行的人面输出逐字节不变）。
	rc, out, _ = gapQIDTestRun(t, "gap", "show", "GAP-20260926-70")
	if rc != exitOK {
		t.Fatalf("`gap show` 人面 rc=%d", rc)
	}
	if strings.Contains(out, "任务号") {
		t.Fatalf("老行的人面不许印「任务号」：\n%s", out)
	}
}

// TestGapQID_AddWritesAndReverseLookup —— ③ 写面：`gap add --qid` 真落号；反查轴一键回到那条缺口。
func TestGapQID_AddWritesAndReverseLookup(t *testing.T) {
	p := gapQIDTestSeed(t, gapQIDTestOldRow)

	rc, out, errs := gapQIDTestRun(t, "gap", "add",
		"--symptom", "新缺口：qid 要能写进新行",
		"--handmade", "手改 jsonl 加 qid 格",
		"--impact", "文档面",
		"--want-family", "gap",
		"--want-action", "add-qid",
		"--repro-cmd", "zerg gap ls --qid Q-900",
		"--verify-cmd", "zerg gap ls --qid Q-900",
		"--qid", "Q-900",
		"--qid-source", "Zerg-内部文档/项目文档/v2.5.14/缺口总账-20261008.md#Q-900",
		"--note", "本单：补 qid 写面",
		"--yes", "--by", "test")
	if rc != exitOK {
		t.Fatalf("`gap add --qid` 应 rc=0，实得 rc=%d stdout=%q stderr=%q", rc, out, errs)
	}
	if !strings.Contains(out, "Q-900") {
		t.Fatalf("落账回执应点名 qid：stdout=%q", out)
	}

	// 真源那一行：两格都在（逐字）。
	lines := gapQIDTestReadLines(t, p)
	if len(lines) != 2 {
		t.Fatalf("真源应 2 行，实得 %d 行：%v", len(lines), lines)
	}
	newRow := map[string]any{}
	if err := json.Unmarshal([]byte(lines[1]), &newRow); err != nil {
		t.Fatalf("新行不是合法 JSON：%v（%s）", err, lines[1])
	}
	if newRow["qid"] != "Q-900" {
		t.Fatalf("新行 qid 格不对：%v（整行 %s）", newRow["qid"], lines[1])
	}
	if newRow["qid_source"] != "Zerg-内部文档/项目文档/v2.5.14/缺口总账-20261008.md#Q-900" {
		t.Fatalf("新行 qid_source 格不对：%v", newRow["qid_source"])
	}

	// ④ 存量行**逐字节不变**（老行连键序都不许动）。
	if lines[0] != gapQIDTestOldRow {
		t.Fatalf("存量行被改动了：\n改后=%s\n改前=%s", lines[0], gapQIDTestOldRow)
	}

	// 机器面反查：号 → 缺口（且只回那一条）。
	rc, out, errs = gapQIDTestRun(t, "gap", "ls", "--qid", "Q-900", "--json", "id,qid,qid_source")
	if rc != exitOK {
		t.Fatalf("`gap ls --qid Q-900` rc=%d stderr=%q", rc, errs)
	}
	items := gapQIDTestJSON(t, out)
	if len(items) != 1 || items[0]["qid"] != "Q-900" {
		t.Fatalf("反查应恰好命中 Q-900 那一条：%s", out)
	}
	if _, ok := items[0]["qid_source"]; !ok {
		t.Fatalf("反查行应带 qid_source 格：%s", out)
	}

	// 单条面人面：有号 ⇒ 多印「任务号」一行。
	rc, out, _ = gapQIDTestRun(t, "gap", "show", newRow["id"].(string))
	if rc != exitOK || !strings.Contains(out, "任务号   : Q-900") {
		t.Fatalf("有 qid 的行人面应印「任务号 : Q-900」：rc=%d\n%s", rc, out)
	}
}

// TestGapQID_AddRefusalsWriteNothing —— ③ 三道拒收（形态 / 唯一 / 锚无号）**一个字节都不写**。
func TestGapQID_AddRefusalsWriteNothing(t *testing.T) {
	base := []string{
		gapQIDTestOldRow,
		// 占号行：`Q-7` 已被它占着。
		`{"id":"GAP-20260926-71","fp":"fp71","symptom":"占号行","handmade":"x","impact":"命令面","prio":"P2","state":"仍缺","want_family":"gap","want_action":"ls","repro_cmd":"zerg gap ls","verify_cmd":"zerg gap ls","found_at":"2026-09-26T03:00:00+08:00","qid":"Q-7"}`,
	}
	addBase := []string{
		"gap", "add",
		"--symptom", "拒收档的新缺口",
		"--handmade", "手改真源",
		"--impact", "命令面",
		"--want-family", "gap",
		"--want-action", "refuse-qid",
		"--repro-cmd", "zerg gap ls --qid Q-7",
		"--verify-cmd", "zerg gap ls --qid Q-7",
	}
	cases := []struct {
		name string
		more []string
		want string // stderr 里必须点到的词
	}{
		{"形态不对", []string{"--qid", "Q7"}, "形态不对"},
		{"号已被占", []string{"--qid", "Q-7"}, "已被账内 GAP-20260926-71 占着"},
		{"锚不给号", []string{"--qid-source", "x.md#1"}, "无处落锚"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := gapQIDTestSeed(t, base...)
			before := gapQIDTestReadLines(t, p)
			argv := append(append([]string{}, addBase...), c.more...)
			argv = append(argv, "--yes", "--by", "test")
			rc, out, errs := gapQIDTestRun(t, argv...)
			if rc != exitUsage {
				t.Fatalf("应拒收 2，实得 rc=%d stdout=%q stderr=%q", rc, out, errs)
			}
			if !strings.Contains(errs, c.want) {
				t.Fatalf("判词应点名 %q：stderr=%q", c.want, errs)
			}
			after := gapQIDTestReadLines(t, p)
			if strings.Join(before, "\n") != strings.Join(after, "\n") {
				t.Fatalf("拒收档**一个字节都不许写**：\n前=%v\n后=%v", before, after)
			}
		})
	}
}

// TestGapQID_AddWithoutQIDUnchanged —— ③ 兼容：不给 `--qid` 的老调用方，行里**不出现**这两格。
func TestGapQID_AddWithoutQIDUnchanged(t *testing.T) {
	p := gapQIDTestSeed(t, gapQIDTestOldRow)
	rc, out, errs := gapQIDTestRun(t, "gap", "add",
		"--symptom", "老调用方：不给 qid",
		"--handmade", "手改真源",
		"--impact", "命令面",
		"--want-family", "gap",
		"--want-action", "no-qid",
		"--repro-cmd", "zerg gap ls --qid Q-1",
		"--verify-cmd", "zerg gap ls --qid Q-1",
		"--yes", "--by", "test")
	if rc != exitOK {
		t.Fatalf("老调用方（不给 --qid）应照旧 rc=0：rc=%d stderr=%q", rc, errs)
	}
	if strings.Contains(out, "qid") {
		t.Fatalf("不给 --qid 时回执不许提 qid：stdout=%q", out)
	}
	lines := gapQIDTestReadLines(t, p)
	if len(lines) != 2 {
		t.Fatalf("真源应 2 行：%v", lines)
	}
	if strings.Contains(lines[1], `"qid"`) || strings.Contains(lines[1], `"qid_source"`) {
		t.Fatalf("不给 --qid ⇒ 新行不许出现那两格（omitempty）：%s", lines[1])
	}
}

// TestGapQID_LsFilterGuards —— ② 反查轴的用法面：裸给 / 空值 / 形态不对 ⇒ 2（空值不是「不过滤」）。
func TestGapQID_LsFilterGuards(t *testing.T) {
	gapQIDTestSeed(t, gapQIDTestOldRow)
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"形态不对", []string{"gap", "ls", "--qid", "216"}, "形态不对"},
		{"空值", []string{"gap", "ls", "--qid", ""}, "要给一个号"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc, out, errs := gapQIDTestRun(t, c.argv...)
			if rc != exitUsage {
				t.Fatalf("应 2，实得 rc=%d stdout=%q stderr=%q", rc, out, errs)
			}
			if !strings.Contains(errs, c.want) {
				t.Fatalf("判词应含 %q：stderr=%q", c.want, errs)
			}
			if out != "" {
				t.Fatalf("用法错不许有结果面：stdout=%q", out)
			}
		})
	}
}

// TestGapQID_PromoteRefusesQID —— `gap idea promote` 的形状里**没有**这两枚 ⇒ 收到即拒 2（✗ 静默丢）。
// 判在**读池之前**（用法面先于盘面动作）⇒ 池件不在盘也照样退 2，不必造池夹具。
func TestGapQID_PromoteRefusesQID(t *testing.T) {
	gapQIDTestSeed(t, gapQIDTestOldRow)
	for _, f := range []string{"--qid", "--qid-source"} {
		argv := []string{"gap", "idea", "promote", "cid-1",
			"--handmade", "h", "--impact", "命令面", "--want-family", "gap", "--want-action", "x",
			"--repro-cmd", "zerg gap ls", "--verify-cmd", "zerg gap ls --qid Q-1",
			f, "Q-1", "--yes", "--by", "test"}
		rc, out, errs := gapQIDTestRun(t, argv...)
		if rc != exitUsage {
			t.Fatalf("promote 收到 %s 应拒 2，实得 rc=%d stdout=%q stderr=%q", f, rc, out, errs)
		}
		if !strings.Contains(errs, "不收 `"+f+"`") {
			t.Fatalf("判词应点名「不收 %s」：stderr=%q", f, errs)
		}
	}
}

// TestGapQID_IdentityUnaffected —— ④ 两格**不吃**身份面：fp 与「同内容」判据对 qid 不敏感。
// （这条是「加格不动老账」的机制保证：fp 若吃了 qid，全账 1400+ 条的指纹会集体漂。）
func TestGapQID_IdentityUnaffected(t *testing.T) {
	base := gapRecord{
		Symptom: "s", Handmade: "h", Impact: "命令面", Prio: "P1",
		WantFam: "gap", WantAct: "ls", ReproCmd: "zerg gap ls", VerifyCmd: "zerg gap ls",
	}
	withQID := base
	withQID.QID, withQID.QIDSource = "Q-216", "某件.md#Q-216"

	if gapFingerprint(base) != gapFingerprint(withQID) {
		t.Fatalf("fp 不该吃 qid：%s ≠ %s", gapFingerprint(base), gapFingerprint(withQID))
	}
	if !gapSameContent(base, withQID) {
		t.Fatalf("「同内容」判据不该吃 qid（否则加格会让幂等面整体漂）")
	}
}

// TestGapQID_ShapeJudge —— 形态判据与读面 `--qid-check` 同一条（`gglQIDRE`）。
func TestGapQID_ShapeJudge(t *testing.T) {
	ok := []string{"", "Q-1", "Q-216", "Q-9999"}
	bad := []string{"q-216", "Q216", "Q-", "-216", "Q-1a", "Q 1", "Q-1.2"}
	for _, v := range ok {
		if why := gapQIDShapeWhy(v); why != "" {
			t.Fatalf("%q 应合法，判成：%s", v, why)
		}
	}
	for _, v := range bad {
		if why := gapQIDShapeWhy(v); why == "" {
			t.Fatalf("%q 应判形态非法（与 --qid-check 同一条正则）", v)
		}
	}
}
