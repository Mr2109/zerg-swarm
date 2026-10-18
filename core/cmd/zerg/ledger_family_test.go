// ledger_family_test.go —— 「多件族」登记口径（GAP-20260927-323）的回归牙。
//
// 口径一句话：族 = 同前缀 `缺口总账-<日期>`、后缀落在闭集
// { 空（索引件）· `-<面>-pNN` · `-页` · `-附` · `-1..-9` } 内的件集；
// 门（check_gap_ledger_view.go）与生成器（gen_gap_ledger.go）**共用** ledger_family.go 一份规则。
//
// 本件断四件事：
//
//	① 闭集判据 —— 件名拆解（index/page/member）与「族外件」逐格；
//	② 两态 —— 同一夹具下「旧单件名规则」只看见 1 件、「共用族规则」看见 3 件；
//	③ 两处真调 —— 门 glvCollectFamily 与生成器 gglLedgerCandidates/gglPickFamily 都见族里每一件；
//	④ 零回归 —— 单件名（族里后缀=空那一件）照旧被看见。
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// lfOldLedgerNameRE —— 改前态的单件名规则**逐字副本**（HEAD cbb67c6a7 的
// `core/cmd/zerg/gen_gap_ledger.go:89` `gglLedgerNameRE`，与门件 `glvLedgerNameRE` 同规则）。
// 只用于两态读数：同一夹具下它只认 1 件。
var lfOldLedgerNameRE = regexp.MustCompile(`^缺口总账-[\p{Nd}]{8}\.md$`)

// lfDocsRoot —— 造夹具并交出 docs 根（`<tmp>/Zerg-内部文档`）。
func lfDocsRoot(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "Zerg-内部文档", "项目文档", "v2.5.13")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("造夹具目录失败：%v", err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("# 面 · 缺口总账 20261018\n"), 0o644); err != nil {
			t.Fatalf("造夹具件 `%s` 失败：%v", n, err)
		}
	}
	return filepath.Join(root, "Zerg-内部文档")
}

func TestLedgerFamilySplitClosedSet(t *testing.T) {
	cases := []struct {
		name string
		kind string
		ok   bool
		face string
		page string
	}{
		{"缺口总账-20261018.md", "index", true, "", ""},
		{"缺口总账-2026-10-18.md", "index", true, "", ""},
		{"缺口总账-20261018-命令面-p01.md", "page", true, "命令面", "01"},
		{"缺口总账-2026-10-18-归档面-p12.md", "page", true, "归档面", "12"},
		{"缺口总账-20261018-页.md", "member", true, "", ""},
		{"缺口总账-20261018-附.md", "member", true, "", ""},
		{"缺口总账-20261018-3.md", "member", true, "", ""},
		// 族外：面不在闭集 / 页号非两位 / 后缀不在闭集 / 前缀不同 / 日期坏
		{"缺口总账-20261018-杂面-p01.md", "", false, "", ""},
		{"缺口总账-20261018-命令面-p1.md", "", false, "", ""},
		{"缺口总账-20261018-尾注.md", "", false, "", ""},
		{"缺口总账-20261018-0.md", "", false, "", ""},
		{"缺口总账-2026-9-27.md", "", false, "", ""},
		{"别的总账-20261018.md", "", false, "", ""},
	}
	for _, c := range cases {
		p, ok := ledgerFamilySplit(c.name)
		if ok != c.ok {
			t.Fatalf("`%s` 是否族成员 = %v，want %v", c.name, ok, c.ok)
		}
		if !ok {
			continue
		}
		if p.Kind != c.kind || p.Face != c.face || p.Page != c.page {
			t.Fatalf("`%s` 拆解 = (%q,%q,%q)，want (%q,%q,%q)", c.name, p.Kind, p.Face, p.Page, c.kind, c.face, c.page)
		}
		if c.name != p.Key+".md" && !strings.HasPrefix(c.name, p.Key+"-") {
			t.Fatalf("`%s` 族键 %q 不是件名的真前缀", c.name, p.Key)
		}
	}
}

func TestLedgerFamilyTwoStateFixture(t *testing.T) {
	// 夹具族：三件同前缀（索引 + 两个真页件 —— 仓里 20261018 族就是这个形状）。
	names := []string{"缺口总账-20261018.md", "缺口总账-20261018-命令面-p01.md", "缺口总账-20261018-门禁面-p01.md"}
	docs := lfDocsRoot(t, names...)

	// ── 改前态：单件名规则（逐字副本）只看见 1 件 ──
	old := 0
	for _, n := range names {
		if lfOldLedgerNameRE.MatchString(n) {
			old++
		}
	}
	if old < 1 || old != 1 {
		t.Fatalf("负控失效：旧单件名规则应只看见下限 1 件、实得 %d 件（改了夹具？）", old)
	}

	// ── 改后态·生成器：候选件 = 族里每一件 ──
	cands := gglLedgerCandidates(docs)
	if len(cands) < 3 {
		t.Fatalf("生成器候选低于下限 3 件（实得 %d）⇒ 族件仍不可见", len(cands))
	}
	if len(cands) != 3 {
		t.Fatalf("生成器候选 = %d 件，want 3（族里每一件）", len(cands))
	}
	fam, ok := gglPickFamily(docs, true)
	if !ok {
		t.Fatalf("gglPickFamily 判不出族（fail-closed 应给结论）")
	}
	if len(fam.Members) < 3 || len(fam.Members) != 3 {
		t.Fatalf("族挑选成员 = %d 件，want 3（旧读法只取 sort[-1] 一件）", len(fam.Members))
	}
	if fam.Ver != "v2.5.13" || fam.Key != "缺口总账-20261018" {
		t.Fatalf("族挑选 = (ver=%q,key=%q)，want (v2.5.13,缺口总账-20261018)", fam.Ver, fam.Key)
	}

	// ── 改后态·门：族里每一件都登记（索引 + 页件）──
	f := glvCollectFamily(docs)
	if f.Index == nil {
		t.Fatalf("门未认索引件 ⇒ 族口径断了")
	}
	got := len(f.Family) + len(f.Members)
	if got < 3 || got != 3 {
		t.Fatalf("门见族件 = %d 件（Family %d + Members %d），want 3", got, len(f.Family), len(f.Members))
	}
}

func TestLedgerFamilySuffixPiecesSeenByGate(t *testing.T) {
	// 闭集后缀件（`-页` / `-1`）：旧门件读法在这两件上 continue ⇒ 整件丢出族外（连 History 都进不去）。
	names := []string{"缺口总账-20261018.md", "缺口总账-20261018-页.md", "缺口总账-20261018-1.md"}
	docs := lfDocsRoot(t, names...)

	old := 0
	for _, n := range names {
		if lfOldLedgerNameRE.MatchString(n) {
			old++
		}
	}
	if old < 1 || old != 1 {
		t.Fatalf("负控失效：旧规则应只看见下限 1 件、实得 %d 件", old)
	}

	f := glvCollectFamily(docs)
	if len(f.Members) < 2 {
		t.Fatalf("门见「族内另件」低于下限 2 件（实得 %d）⇒ 闭集后缀件仍被丢", len(f.Members))
	}
	got := len(f.Family) + len(f.Members)
	if got != 3 {
		t.Fatalf("门见族件 = %d 件（Family %d + Members %d），want 3", got, len(f.Family), len(f.Members))
	}
}

func TestLedgerFamilySingleNameNotBroken(t *testing.T) {
	// 零回归：单件名 = 族里「后缀=空」那一件，门与生成器都照旧看见。
	docs := lfDocsRoot(t, "缺口总账-20260926.md")

	f := glvCollectFamily(docs)
	if f.Index == nil {
		t.Fatalf("单件名族：门未认索引件 ⇒ 零回归破了")
	}
	if f.Index.Date != "20260926" {
		t.Fatalf("单件名族：索引件日期 = %q，want 20260926", f.Index.Date)
	}
	if len(f.Members) != 0 {
		t.Fatalf("单件名族：族内另件应为 0（实得 %d）", len(f.Members))
	}
	cands := gglLedgerCandidates(docs)
	if len(cands) < 1 || len(cands) != 1 {
		t.Fatalf("单件名族：生成器候选 = %d 件，want 1", len(cands))
	}
	fam, ok := gglPickFamily(docs, true)
	if !ok || len(fam.Members) != 1 || fam.Key != "缺口总账-20260926" {
		t.Fatalf("单件名族：族挑选 = (%v, %d, %q)，want (true,1,缺口总账-20260926)", ok, len(fam.Members), fam.Key)
	}
}

func TestLedgerFamilyDashDateFamilySeen(t *testing.T) {
	// 仓里真族 `缺口总账-2026-09-27`（索引 + 六页）—— 旧单件名规则对破折号日期一律不认（0 件）。
	names := []string{"缺口总账-2026-09-27.md", "缺口总账-2026-09-27-命令面-p01.md", "缺口总账-2026-09-27-归档面-p01.md"}
	docs := lfDocsRoot(t, names...)

	old := 0
	for _, n := range names {
		if lfOldLedgerNameRE.MatchString(n) {
			old++
		}
	}
	if old != 0 {
		t.Fatalf("负控失效：旧单件名规则对破折号日期应 0 件、实得 %d 件", old)
	}
	cands := gglLedgerCandidates(docs)
	if len(cands) < 3 || len(cands) != 3 {
		t.Fatalf("破折号日期族：候选 = %d 件，want 3", len(cands))
	}
	if keys, m := ledgerFamilies(names); len(keys) != 1 || len(m[keys[0]]) != 3 {
		t.Fatalf("破折号日期族：ledgerFamilies = (%v, %d 件)，want (1 族, 3 件)", keys, len(m[keys[0]]))
	}
}
