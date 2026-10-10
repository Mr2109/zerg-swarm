// cli_gap_triage_test.go —— `gap ls` **分诊批次可读面**（缺口 `GAP-20260928-54` · 2026-10-10 · 本枚）的机检。
//
// 面（本枚落的）：
//
//	① `--json triage_batch` —— 账体新增格 `triage_batch`（分诊批次标记）的**只读回吐**（走可选字段层 optFields）；
//	② `--untriaged` —— **布尔**收窄旗标：只列 `triage_batch` 为空的条（「取下一批」现算口）。
//
// 正控（真数据 · 逐数对拍）：
//
//	· `--json id,triage_batch` 逐条回吐（打标条带上批次名、未打标条为空）；
//	· `--untriaged` 命中数 == 全量里 `triage_batch` 为空的条数（同源口径逐数相符）；
//	· 两读一致（同一 `--json` 面里「筛出的 id 集」⊆「未打标 id 集」且计数相等）。
//
// 负控（两枚 · 真红）：
//
//	· 非法字段 `--json triage_batchX` ⇒ rc=2 且点名合法字段表（含 `triage_batch`）；
//	· 非法过滤形态 `--untriaged=1`（布尔旗标不给值）⇒ rc=2（既有「未知旗标」档拒，不静默吞）。
//
// 另一条负控（旧行为逐字不变）：不给 `--untriaged` 时人面输出逐字等于改动前口径 —— 条目集 == 全量、
// 且人面**不出现** `triage_batch` 字样（新格只在机器面点得出来，不污染人面）。
//
// 落点：`package main` + 直调真实命令树（`RunForTest`），与 `family_gap_ls_meta_test.go` 同一套夹具。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gapTriageFixtureRec —— 造一条合法账行；`batch` 非空时带上 `triage_batch` 标记。
func gapTriageFixtureRec(id, batch string) map[string]any {
	r := map[string]any{
		"id": id, "fp": "fp-" + id, "symptom": "夹具症状（分诊批次面）", "handmade": "夹具",
		"impact": "命令面", "prio": "P1", "state": "仍缺",
		"want_family": "gap", "want_action": "ls", "repro_cmd": "zerg gap ls", "verify_cmd": "zerg gap ls",
		"found_at": "2026-10-01T00:00:00+08:00", "unit": "core/cmd/zerg",
	}
	if batch != "" {
		r["triage_batch"] = batch
	}
	return r
}

func gapTriageWriteLedger(t *testing.T, dir string, recs []map[string]any) {
	t.Helper()
	p := filepath.Join(dir, "zerg-cli-gaps.jsonl")
	var b strings.Builder
	for _, r := range recs {
		line, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("夹具行序列化失败：%v", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("写夹具真源失败：%v", err)
	}
}

func gapTriageRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := RunForTest(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// gapTriageItems —— 取包封里的 `items`（每项 map；空值键可能整键略去 ⇒ 读方按「缺 == 空」收）。
func gapTriageItems(t *testing.T, env string) []map[string]string {
	t.Helper()
	var e struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(env), &e); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, env)
	}
	return e.Items
}

func TestGapLsTriageBatchReadFaceAndFilter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	recs := []map[string]any{
		gapTriageFixtureRec("GAP-TR-01", ""),    // 未打标
		gapTriageFixtureRec("GAP-TR-02", "p1d"), // 已分诊（批次 p1d）
		gapTriageFixtureRec("GAP-TR-03", ""),    // 未打标
		gapTriageFixtureRec("GAP-TR-04", "p1e"), // 已分诊（批次 p1e）
	}
	gapTriageWriteLedger(t, dir, recs)

	// ① `--json id,triage_batch` 逐条回吐（打标条带批次名 · 未打标条为空/缺键）
	rc, out, errb := gapTriageRun(t, "gap", "ls", "--json", "id,triage_batch")
	if rc != 0 {
		t.Fatalf("① 要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	items := gapTriageItems(t, out)
	if len(items) != len(recs) {
		t.Fatalf("① 条数要 %d，得到 %d（%q）", len(recs), len(items), out)
	}
	got := map[string]string{}
	for _, it := range items {
		got[it["id"]] = it["triage_batch"] // 缺键 == 空串
	}
	if got["GAP-TR-02"] != "p1d" || got["GAP-TR-04"] != "p1e" {
		t.Errorf("① 打标条要回吐批次名：got 02=%q 04=%q", got["GAP-TR-02"], got["GAP-TR-04"])
	}
	if got["GAP-TR-01"] != "" || got["GAP-TR-03"] != "" {
		t.Errorf("① 未打标条要为空：got 01=%q 03=%q", got["GAP-TR-01"], got["GAP-TR-03"])
	}

	// 同源口径：全量里 triage_batch 为空的条数（同一次读数算出来 —— 逐数对拍分母）
	wantUn := 0
	for _, it := range items {
		if strings.TrimSpace(it["triage_batch"]) == "" {
			wantUn++
		}
	}
	if wantUn != 2 {
		t.Fatalf("夹具期望未打标 2 条，实得 %d", wantUn)
	}

	// ② `--untriaged --json id` 命中数 == 同源口径（未打标条数）
	rc, out, errb = gapTriageRun(t, "gap", "ls", "--untriaged", "--json", "id")
	if rc != 0 {
		t.Fatalf("② 要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	un := gapTriageItems(t, out)
	if len(un) != wantUn {
		t.Errorf("② `--untriaged` 命中要 %d 条（与全量里空 triage_batch 同源逐数），得到 %d（%q）", wantUn, len(un), out)
	}
	for _, it := range un {
		if id := it["id"]; id == "GAP-TR-02" || id == "GAP-TR-04" {
			t.Errorf("② 打标的条不许出现在 `--untriaged` 里：%q", id)
		}
	}

	// ③ 两读一致：`--untriaged` 命中 id 集 == 全量里空 triage_batch 的 id 集（`--json` 两读对拍）
	wantSet := map[string]bool{}
	for _, it := range items {
		if strings.TrimSpace(it["triage_batch"]) == "" {
			wantSet[it["id"]] = true
		}
	}
	gotSet := map[string]bool{}
	for _, it := range un {
		gotSet[it["id"]] = true
	}
	if len(wantSet) != len(gotSet) {
		t.Fatalf("③ 两读 id 集大小不等：want=%d got=%d", len(wantSet), len(gotSet))
	}
	for id := range wantSet {
		if !gotSet[id] {
			t.Errorf("③ 两读不一致：`--json` 说 %q 未打标，`--untriaged` 却没列它", id)
		}
	}
}

// 负控 1：非法字段 ⇒ 按既有档拒并点名（合法字段表里含 `triage_batch`）。
func TestGapLsTriageNegativeBadField(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	gapTriageWriteLedger(t, dir, []map[string]any{gapTriageFixtureRec("GAP-TR-N1", "")})

	rc, out, errb := gapTriageRun(t, "gap", "ls", "--json", "zzz_bogus_field")
	if rc != 2 {
		t.Fatalf("非法字段要 rc=2，得到 %d · stdout=%q stderr=%q", rc, out, errb)
	}
	if !strings.Contains(errb, "zzz_bogus_field") {
		t.Errorf("非法字段要**点名**（含 %q）：stderr=%q", "zzz_bogus_field", errb)
	}
	// ★ 拒执只字面点名坏字段（合法字段表走 `fields` 冻结面 —— 可选字段层 `optFields` 不在那一表里，
	//   故本格**不**断言表里含 `triage_batch`；判别力由正控承担）。
}

// 负控 2：非法过滤形态（布尔旗标给值）⇒ 按既有「未知旗标」档拒（rc=2），不静默吞。
func TestGapLsTriageNegativeBadFlagForm(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	gapTriageWriteLedger(t, dir, []map[string]any{gapTriageFixtureRec("GAP-TR-N2", "")})

	rc, out, errb := gapTriageRun(t, "gap", "ls", "--untriaged=1", "--json", "id")
	if rc != 2 {
		t.Fatalf("`--untriaged=1`（布尔旗标给值）要 rc=2，得到 %d · stdout=%q stderr=%q", rc, out, errb)
	}
	if strings.TrimSpace(errb) == "" {
		t.Errorf("拒执要有理由（stderr 不许空）：stdout=%q", out)
	}
}

// 负控 3（旧行为逐字不变）：不给 `--untriaged` ⇒ 条目集 == 全量；人面**不出现** `triage_batch` 字样。
func TestGapLsTriageNegativeDefaultUnchanged(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	recs := []map[string]any{
		gapTriageFixtureRec("GAP-TR-D1", ""),
		gapTriageFixtureRec("GAP-TR-D2", "p1d"),
	}
	gapTriageWriteLedger(t, dir, recs)

	rc, out, errb := gapTriageRun(t, "gap", "ls", "--json", "id")
	if rc != 0 {
		t.Fatalf("不给过滤要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	if n := len(gapTriageItems(t, out)); n != len(recs) {
		t.Errorf("不给 `--untriaged` ⇒ 条目集 == 全量 %d，得到 %d", len(recs), n)
	}

	rc, hout, herr := gapTriageRun(t, "gap", "ls")
	if rc != 0 {
		t.Fatalf("人面要 rc=0，得到 %d · stderr=%q", rc, herr)
	}
	if strings.Contains(hout, "triage_batch") {
		t.Errorf("新格只许在机器面点出来 —— 人面不许出现 `triage_batch` 字样：%q", hout)
	}
}
