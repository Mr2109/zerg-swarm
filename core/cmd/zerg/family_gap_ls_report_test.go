// family_gap_ls_report_test.go —— `gap ls` 的**跑级截断自报**（缺口 `GAP-20260928-163`）的机检。
//
// 口径（全量档与收窄档**同尺** · 两档**同源**）：
//
//	正控 ① 全量档（只给闭集旗标 / 不给）：`--json id,truncated,hits,total` 点得出三格，
//	        逐行 `truncated=false` · `hits==total==真命中数`（与实际条数逐数相符）。
//	正控 ② 收窄档（≤ `gapLsRowCap`）：行值 ⟷ `meta.total`/`meta.hits` **同源**（逐字相等）。
//	负控 ①（**造一处截断**）：收窄档命中 > `gapLsRowCap` ⇒ 行 `truncated=true` ·
//	        `hits==gapLsRowCap` · `total==真命中数`（与顶层包封 `truncated`、`meta` 三数逐字相符）。
//	负控 ②（**未截断**）：两档都 `truncated=false` 且 `hits==total`。
//	边界：真正未知的字段名仍要退 2（自报三格不许把出口白名单放宽成「什么都收」）。
//
// 落点：`package main` + 直调真实命令树（`RunForTest`）；夹具/助手件复用同包
// `family_gap_ls_meta_test.go` 的 `gapLsMetaFixtureRec` / `gapLsMetaWriteLedger` / `gapLsMetaRun` /
// `gapLsMetaOf`（不另抄第二套）。
package main

import (
	"encoding/json"
	"strconv"
	"testing"
)

// gapLsReportItems —— 从包封里取 `items`（逐条 `map`）。
func gapLsReportItems(t *testing.T, env string) []map[string]any {
	t.Helper()
	var e struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(env), &e); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, env)
	}
	return e.Items
}

// gapLsReportTopTruncated —— 顶层包封 `truncated`（真值布尔）。
func gapLsReportTopTruncated(t *testing.T, env string) bool {
	t.Helper()
	var e struct {
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(env), &e); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, env)
	}
	return e.Truncated
}

// gapLsReportRowStr —— 行里某一格（字符串；缺格即 Fatal）。
func gapLsReportRowStr(t *testing.T, row map[string]any, key string) string {
	t.Helper()
	v, ok := row[key]
	if !ok {
		t.Fatalf("行里缺格 %q（现得：%v）", key, row)
	}
	s, _ := v.(string)
	return s
}

// gapLsReportMetaNum —— `meta` 里某一格（数键由 `metaAddJSON` 落成 JSON 数 ⇒ 归一成十进制串对拍）。
func gapLsReportMetaNum(t *testing.T, meta map[string]any, key string) string {
	t.Helper()
	v, ok := meta[key]
	if !ok {
		t.Fatalf("meta 缺格 %q（现得：%v）", key, meta)
	}
	switch n := v.(type) {
	case float64:
		return strconv.Itoa(int(n))
	case string:
		return n
	}
	t.Fatalf("meta %q 不是数：%T(%v)", key, v, v)
	return ""
}

// gapLsReportFixture —— 造 n 条同 unit 的合法账行（闭集取值齐 ⇒ 不触账内越界自查）。
func gapLsReportFixture(n int, unit string) []map[string]any {
	recs := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		recs = append(recs, gapLsMetaFixtureRec("GAP-20261010-"+strconv.Itoa(1000+i), unit))
	}
	return recs
}

// TestGapLsSelfReport_TwoFacesSameScale —— 正控（两档同尺且逐数相符）＋ 负控（未截断 / 造截断）。
func TestGapLsSelfReport_TwoFacesSameScale(t *testing.T) {
	unit := "core/cmd/zerg"

	// ── 态① 全量档（不给收窄旗标）· 3 条 ⇒ hits==total==3 · truncated=false（正控① · 负控②）──
	dirA := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dirA)
	gapLsMetaWriteLedger(t, dirA, gapLsReportFixture(3, unit))
	rc, out, errb := gapLsMetaRun(t, "gap", "ls", "--json", "id,truncated,hits,total")
	if rc != 0 {
		t.Fatalf("全量档要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	items := gapLsReportItems(t, out)
	if len(items) != 3 {
		t.Fatalf("全量档要 3 条，得到 %d", len(items))
	}
	for i, row := range items {
		if got := gapLsReportRowStr(t, row, "truncated"); got != "false" {
			t.Errorf("全量档行 %d：未截断要 truncated=false，得到 %q", i, got)
		}
		h := gapLsReportRowStr(t, row, "hits")
		tot := gapLsReportRowStr(t, row, "total")
		if h != tot || h != "3" {
			t.Errorf("全量档行 %d：未截断要 hits==total==3，得到 hits=%q total=%q", i, h, tot)
		}
	}
	if gapLsReportTopTruncated(t, out) {
		t.Errorf("全量档未截断：顶层 truncated 要 false")
	}
	// ★ 不点这三格时的老输出**逐字节不变**（只点名 `id` ⇒ 行里只有 `id` 一格）。
	_, outOld, _ := gapLsMetaRun(t, "gap", "ls", "--json", "id")
	for i, row := range gapLsReportItems(t, outOld) {
		if len(row) != 1 {
			t.Errorf("只点名 id 时行 %d 不许带别的格（现得 %v）", i, row)
		}
	}

	// ── 态② 收窄档 · 未截断（3 条 ≤ gapLsRowCap）—— 行值 ⟷ meta 同源（正控② · 负控②）──
	dirB := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dirB)
	gapLsMetaWriteLedger(t, dirB, gapLsReportFixture(3, unit))
	rc, out, errb = gapLsMetaRun(t, "gap", "ls", "--unit", unit, "--json", "id,truncated,hits,total")
	if rc != 0 {
		t.Fatalf("收窄档要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	itemsB := gapLsReportItems(t, out)
	metaB := gapLsMetaOf(t, out)
	if len(itemsB) != 3 {
		t.Fatalf("收窄档要 3 条，得到 %d", len(itemsB))
	}
	if mt, mh := gapLsReportMetaNum(t, metaB, "total"), gapLsReportMetaNum(t, metaB, "hits"); mt != "3" || mh != "3" {
		t.Errorf("收窄档 meta 要 total==hits==3，得到 total=%q hits=%q", mt, mh)
	}
	for i, row := range itemsB {
		if got := gapLsReportRowStr(t, row, "truncated"); got != "false" {
			t.Errorf("收窄档行 %d：未截断要 truncated=false，得到 %q", i, got)
		}
		if got, want := gapLsReportRowStr(t, row, "total"), gapLsReportMetaNum(t, metaB, "total"); got != want {
			t.Errorf("收窄档行 %d：行 total=%q 与 meta.total=%q 要同源逐字相等", i, got, want)
		}
		if got, want := gapLsReportRowStr(t, row, "hits"), gapLsReportMetaNum(t, metaB, "hits"); got != want {
			t.Errorf("收窄档行 %d：行 hits=%q 与 meta.hits=%q 要同源逐字相等", i, got, want)
		}
	}

	// ── 态③ 收窄档 · **造一处截断**（201 条 > gapLsRowCap 200）—— 负控① ──
	dirC := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dirC)
	gapLsMetaWriteLedger(t, dirC, gapLsReportFixture(gapLsRowCap+1, unit))
	rc, out, errb = gapLsMetaRun(t, "gap", "ls", "--unit", unit, "--json", "id,truncated,hits,total")
	if rc != 0 {
		t.Fatalf("收窄档（截断）要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	itemsC := gapLsReportItems(t, out)
	wantTotal := strconv.Itoa(gapLsRowCap + 1)
	wantHits := strconv.Itoa(gapLsRowCap)
	if len(itemsC) != gapLsRowCap {
		t.Fatalf("造截断要只列 %d 条，得到 %d", gapLsRowCap, len(itemsC))
	}
	if !gapLsReportTopTruncated(t, out) {
		t.Errorf("造截断：顶层 truncated 要 true")
	}
	metaC := gapLsMetaOf(t, out)
	if mt, mh := gapLsReportMetaNum(t, metaC, "total"), gapLsReportMetaNum(t, metaC, "hits"); mt != wantTotal || mh != wantHits {
		t.Errorf("造截断：meta 要 total=%s hits=%s，得到 total=%q hits=%q", wantTotal, wantHits, mt, mh)
	}
	for i, row := range itemsC {
		if got := gapLsReportRowStr(t, row, "truncated"); got != "true" {
			t.Errorf("造截断：行 %d 要 truncated=true，得到 %q（自报截断位必须与顶层同真）", i, got)
		}
		if got := gapLsReportRowStr(t, row, "hits"); got != wantHits {
			t.Errorf("造截断：行 %d hits 要 ==%s，得到 %q", i, wantHits, got)
		}
		if got := gapLsReportRowStr(t, row, "total"); got != wantTotal {
			t.Errorf("造截断：行 %d total 要 ==%s（真命中数），得到 %q", i, wantTotal, got)
		}
	}

	// ── 边界：真正未知的字段名仍要退 2（自报三格不许把出口白名单放宽成「什么都收」）──
	if rc, _, errb := gapLsMetaRun(t, "gap", "ls", "--json", "zzz-nonexistent-zz"); rc != 2 {
		t.Errorf("未知字段要退 2，得到 %d · stderr=%q", rc, errb)
	}
}
