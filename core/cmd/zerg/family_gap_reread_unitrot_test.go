// family_gap_reread_unitrot_test.go —— 「落点腐检」（`--json unit_rot`）的**机检**（勘定单推荐 (iii)）。
//
// 病（勘定单）：账里每条的落点（`unit`）可靠，但**行号会腐烂** —— 正文/证据里的 `件:行` 引用会漂
// （现读实证 `GAP-20260927-301` 正文写 `family_code.go:54`，现读该行是注释）。既有 `gapRereadCiteProbe`
// 只判「那个标识符还在不在件里（全件面）」，**行级**那一档缺一支 ⇒ 本件钉住新加的**只读**一档。
//
// 口径（只读面 · 以「件」为主锚 · 与 `family_gap_reread_test.go` 同一套夹具 helpers · 不复写）：
//
//	① 落点件缺失 —— `unit`（**非**兜底档）现读不在盘 ⇒ 档 `落点件缺失`（对齐门101/门88 的「落点件在盘」口径）
//	② 落点行号已腐 —— `件:行` 指的件在盘，但该行现读**不是内容行**（空/注释）或**越界** ⇒ 档 `落点行号已腐`
//	③ 兜底 `unit`（`无件(命令面)` / 拟件 / 空）**不判**件缺失（✗ 误判＝造红）
//	④ 健康条（件在盘 · 引用行是内容行）⇒ 无异常（`--json unit_rot` = `ok`）
//	⑤ 只读：本面一字节都不写真源（前后 sha256 逐字不变）
//	⑥ `--json unit_rot` 已登记（改前该字段不在命令树 ⇒ rc=2「未知字段」；本件钉住它 rc=0 且**逐字给值**）
//
// 两态 / 负控（哑化）：不带 overlay 应**绿**；带 overlay（把 `gapRereadUnitRot` 哑化成恒 `("","")` 或
// 把 `unit_rot` 从 `gapRereadFields` 摘掉）应**红** —— 证这颗牙长在新档本身（而非某个内部函数）。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gapURWrite —— 造一件夹具件（含父目录）。
func gapURWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("造夹具目录失败：%v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("造夹具件失败：%v", err)
	}
}

// TestGapRereadUnitRotVerdicts —— 判据面（直调 `gapRereadUnitRot` · 合成根 ⇒ 完全封闭）。
func TestGapRereadUnitRotVerdicts(t *testing.T) {
	root := t.TempDir()
	// sub/good.go：行 1 = 注释 · 行 2 = 空行 · 行 3 = 内容行。
	gapURWrite(t, filepath.Join(root, "sub", "good.go"), "// 头注释\n\nvar Alpha = 1\n")
	tracked := map[string]bool{"sub/good.go": true}
	base := gapRereadCitIndex(tracked)

	cases := []struct {
		name, unit, symptom, want string
	}{
		{"兜底 无件(命令面) ⇒ 不判", "无件(命令面)", "", ""},
		{"兜底 拟件 ⇒ 不判", "拟(new.go)", "", ""},
		{"空 unit ⇒ 不判", "", "", ""},
		{"件缺失（unit 不在盘）", "no/such/piece.py", "", gapRereadUnitMissing},
		{"行号已腐（注释行）", "", "现读 sub/good.go:1 是注释", gapRereadUnitRotted},
		{"行号已腐（空行）", "", "现读 sub/good.go:2 是空行", gapRereadUnitRotted},
		{"行号已腐（越界）", "", "sub/good.go:999 越界", gapRereadUnitRotted},
		{"健康（内容行 ⇒ 无异常）", "", "见 sub/good.go:3", ""},
		{"裸件名唯一 ⇒ 解析并判", "", "见 good.go:1", gapRereadUnitRotted},
		{"同名多件 ⇒ 不判（宁可漏报）", "", "见 good.go:1", ""},
	}
	for i, c := range cases {
		tr := tracked
		bs := base
		if c.name == "同名多件 ⇒ 不判（宁可漏报）" {
			// 再造一个同名件 ⇒ 该裸名解析面作废 ⇒ 不判（宁可漏报不误报）。
			gapURWrite(t, filepath.Join(root, "other", "good.go"), "// 另一个 good.go\n")
			tr = map[string]bool{"sub/good.go": true, "other/good.go": true}
			bs = gapRereadCitIndex(tr)
			_ = i
		}
		rec := gapRecord{ID: "GAP-X", Symptom: c.symptom, Unit: c.unit}
		got, note := gapRereadUnitRot(rec, tr, bs, root)
		if got != c.want {
			t.Errorf("%s：期望档 %q，得 %q（note=%q）", c.name, c.want, got, note)
		}
	}
}

// TestGapRereadUnitRotJSON —— 读面档（`--json unit_rot` · 走 `cmdGapReread` 全链）+ 只读性。
func TestGapRereadUnitRotJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)

	// ① 件缺失条（unit 指一个现读不在盘的件 · 正文无 `件:行` ⇒ 只由 unit 判 · 确定性）
	recA := gapRereadFixtureRec("GAP-20260929-77-A", "仍缺", "")
	recA["unit"] = "scripts/gates/__no_such_piece__.py"
	// ② 兜底条（无件(命令面) · 无 `件:行` ⇒ **不判件缺失**）
	recB := gapRereadFixtureRec("GAP-20260929-77-B", "仍缺", "")
	recB["unit"] = "无件(命令面)"
	// ③ 行号已腐条（真仓件在盘 · 正文引它到**越界行** ⇒ 件在盘 · 行号已腐）
	recC := gapRereadFixtureRec("GAP-20260929-77-C", "仍缺", "")
	recC["unit"] = "core/cmd/zerg/family_gap.go"
	recC["symptom"] = "夹具：family_gap.go:999999 越界（只读对拍）"

	ledger := gapRereadWriteLedger(t, dir, []map[string]any{recA, recB, recC})
	sha0 := gapRereadFileSHA(t, ledger)

	// ⑥ 档已登记：`--json unit_rot` rc=0（改前 = rc=2「未知字段」）
	want := map[string]string{
		"GAP-20260929-77-A": gapRereadUnitMissing,
		"GAP-20260929-77-B": "ok",
		"GAP-20260929-77-C": gapRereadUnitRotted,
	}
	for id, w := range want {
		rc, js, jerr := gapRereadRun(t, id, "--json", "id,unit_rot")
		if rc != 0 {
			t.Fatalf("%s：`--json unit_rot` 要 rc=0（档已登记），得到 %d · stderr=%q", id, rc, jerr)
		}
		if !strings.Contains(js, `"unit_rot":"`+w+`"`) {
			t.Errorf("%s：要逐字给 `\"unit_rot\":\"%s\"`：%q", id, w, js)
		}
	}

	// ③ 兜底条**不许**被判成件缺失（整句反判）
	_, jsB, _ := gapRereadRun(t, "GAP-20260929-77-B", "--json", "id,unit_rot")
	if strings.Contains(jsB, gapRereadUnitMissing) {
		t.Errorf("兜底无件条**不许**判「%s」：%q", gapRereadUnitMissing, jsB)
	}

	// 机器面两档计数（meta 现读现算）
	_, jsA, _ := gapRereadRun(t, "GAP-20260929-77-A", "--json", "id,unit_rot")
	if !strings.Contains(jsA, `"unit_missing":1`) {
		t.Errorf("meta.unit_missing 要为 1：%q", jsA)
	}
	_, jsC, _ := gapRereadRun(t, "GAP-20260929-77-C", "--json", "id,unit_rot")
	if !strings.Contains(jsC, `"unit_rotted":1`) {
		t.Errorf("meta.unit_rotted 要为 1：%q", jsC)
	}

	// 人面：非空档要单列（★ 行号已腐 · 件缺失）
	_, outH, _ := gapRereadRun(t, "GAP-20260929-77-C")
	if !strings.Contains(outH, gapRereadUnitRotted) || !strings.Contains(outH, "落点腐检") {
		t.Errorf("人面要给「落点腐检」汇总 + 单列：%q", outH)
	}

	// ⑤ 只读：本面一字节都不写真源
	if got := gapRereadFileSHA(t, ledger); got != sha0 {
		t.Errorf("本面必须**只读**（真源 sha 变了）：%s → %s", sha0, got)
	}
}
