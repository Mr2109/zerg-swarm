// family_gap_two_axes_test.go —— 缺口 `GAP-20260928-168`（「同一个件，两条命令报两个数」）的成对自检。
//
// 被测面（收口后）：
//
//	`gap show <件>` 报三个**各自标口径**的数 ——
//	  口径A `unit` 精确等值（与 `gap ls --unit <件>` **同轴** · `items[]` 同集 · `meta.axis`）；
//	  口径B `want` 面逐字含（`want_family`/`want_action`/`want_argv` · `meta.axis_want`）；
//	  口径C 全文子串（**整路径** · symptom/handmade/repro_cmd/verify_cmd/证据/注文 + want 面 ·
//	         `meta.axis_substring` · 与旧版「逐字子串面」判据一字不动）。
//
// 两态（本件两个 Test）：
//
//	① 真件 `core/cmd/zerg/main.go`：`show.meta.axis == ls(--unit).meta.count` 且 `items[]` 同集、
//	   两 `meta.axis_scope` 逐字同值 ⇒ 「同一件两条命令不矛盾」可机检；三个数各自真值正确
//	   （A=2 · B=2 · C=3：三轴互不相等 ⇒ 不是把一边砍成另一边）。
//	② 一件**只出现在 symptom、不在 want 里**：A=0 且 B=0，而 C=1 —— 两命令的「同一个数」只在
//	   同轴（A ⟷ ls.count）上成立，C 面如实多出 1 条 ⇒ 证明两轴真不同（旧口径那种「同件两数不说清」
//	   不会再被误读成两条命令互相打架）。
//
// 落点：`package main`（够得着包级桥 `RunForTest = run` —— 与 `main()` 同一个 `run`）。
// 夹具复用 `family_gap_show_axis_test.go` 的 `gapShowAxisRec` / `gapShowAxisWrite` /
// `gapShowAxisRun` / `gapShowAxisMeta`（不另抄第二套）。
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// gapTwoAxisRec —— 造一条落在指定轴上的合法账行：`unit` 定 A 面、`want_argv` 定 B 面、
// `symptom` 定 C 面（`want_argv` 本身也进 C 面，所以 B ⊆ C 天然成立）。
func gapTwoAxisRec(id, unit, symptom string, wantArgv []string) map[string]any {
	r := gapShowAxisRec(id, unit, "仍缺")
	r["symptom"] = symptom
	if len(wantArgv) > 0 {
		r["want_argv"] = wantArgv
	}
	return r
}

// gapTwoAxisIDs —— 从 `items[]` 取 id 集合（顺序无关）。
func gapTwoAxisIDs(t *testing.T, env string) map[string]bool {
	t.Helper()
	var e struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(env), &e); err != nil {
		t.Fatalf("解不了包封：%v · %q", err, env)
	}
	out := map[string]bool{}
	for _, it := range e.Items {
		id, _ := it["id"].(string)
		out[id] = true
	}
	return out
}

// gapTwoAxisNum —— 取 `meta` 里某一格的真值（数字）。
func gapTwoAxisNum(t *testing.T, meta map[string]any, key string) float64 {
	t.Helper()
	v, ok := meta[key]
	if !ok {
		t.Fatalf("`meta.%s` 缺席（现得：%v）", key, meta)
	}
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("`meta.%s` 要是数字，得到 %T(%v)", key, v, v)
	}
	return f
}

// TestGapShowFileThreeAxesMatchLsUnit —— 态①：真件 + 三轴各自读数 + 与 `gap ls --unit` 同轴不矛盾。
func TestGapShowFileThreeAxesMatchLsUnit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	target := "core/cmd/zerg/main.go" // 真在仓里的件（`gap show` 会 stat 它）
	other := "core/internal/version/version.go"

	recs := []map[string]any{
		// A 面（unit 精确等值）—— 正文不提件 ⇒ 不进 C。
		gapTwoAxisRec("GAP-20990201-A", target, "甲：正文不提件", nil),
		// A + B + C 三面全中。
		gapTwoAxisRec("GAP-20990201-B", target, "乙：正文也不提件", []string{target}),
		// 只在 symptom 里出现 ⇒ 仅 C。
		gapTwoAxisRec("GAP-20990201-C", other, "丙：症状里提到 "+target, nil),
		// 只在 want 面出现（整路径）⇒ B + C。
		gapTwoAxisRec("GAP-20990201-D", other, "丁：正文不提件", []string{target}),
		// 只在 want 面写**基名** ⇒ 只进 B（口径B 认整路径也认基名；口径C 只认整路径）。
		gapTwoAxisRec("GAP-20990201-F", other, "己：正文不提件", []string{"main.go"}),
		// 三面都不中（负控：什么都不该进来）。
		gapTwoAxisRec("GAP-20990201-E", other, "戊：与目标件无关", nil),
	}
	gapShowAxisWrite(t, dir, recs)

	// ── show 机器面：三轴各自真值 ──
	rc, out, errb := gapShowAxisRun(t, "gap", "show", target, "--json", "id")
	if rc != 0 {
		t.Fatalf("gap show 要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	meta := gapShowAxisMeta(t, out)
	if got := gapTwoAxisNum(t, meta, "axis"); got != 2 {
		t.Errorf("口径A（unit 精确等值）应为 2（A/B 两条），得到 %v", got)
	}
	if got := gapTwoAxisNum(t, meta, "axis_want"); got != 3 {
		t.Errorf("口径B（want 面逐字含 · 整路径或基名）应为 3（B/D/F 三条），得到 %v", got)
	}
	if got := gapTwoAxisNum(t, meta, "axis_substring"); got != 3 {
		t.Errorf("口径C（全文子串）应为 3（B/C/D 三条），得到 %v", got)
	}
	if got := gapTwoAxisNum(t, meta, "axis_open"); got != 2 {
		t.Errorf("A 面未闭数应为 2（两条都 `仍缺`），得到 %v", got)
	}
	scope, _ := meta["axis_scope"].(string)
	if scope != "unit 精确等值" {
		t.Errorf("show 的 meta.axis_scope 要逐字 `unit 精确等值`，得到 %q", scope)
	}
	if sel, _ := meta["axis_sel"].(string); sel != target {
		t.Errorf("meta.axis_sel 要回显归一后的件路径 %q，得到 %q", target, sel)
	}
	showIDs := gapTwoAxisIDs(t, out)
	if len(showIDs) != 2 || !showIDs["GAP-20990201-A"] || !showIDs["GAP-20990201-B"] {
		t.Errorf("`items[]` 要与口径A 同集（A/B），得到 %v", showIDs)
	}

	// ── ls --unit 机器面：同轴同数同集 ──
	rc, lout, lerrb := gapShowAxisRun(t, "gap", "ls", "--unit", target, "--json", "id,want")
	if rc != 0 {
		t.Fatalf("gap ls --unit 要 rc=0，得到 %d · stderr=%q", rc, lerrb)
	}
	lmeta := gapShowAxisMeta(t, lout)
	lsCount := gapTwoAxisNum(t, lmeta, "count")
	if lsCount != 2 {
		t.Errorf("gap ls --unit 的 count 应为 2，得到 %v", lsCount)
	}
	if got, _ := lmeta["axis_scope"].(string); got != "unit 精确等值" {
		t.Errorf("gap ls --unit 的 meta.axis_scope 要逐字 `unit 精确等值`，得到 %q", got)
	}
	lsIDs := gapTwoAxisIDs(t, lout)
	if len(lsIDs) != 2 {
		t.Errorf("gap ls --unit items 应为 2 条，得到 %v", lsIDs)
	}
	for id := range showIDs {
		if !lsIDs[id] {
			t.Errorf("两命令同轴不同集：%s 在 show 有、在 ls --unit 无（show=%v · ls=%v）", id, showIDs, lsIDs)
		}
	}

	// ★ 可机检断言（缺口本体的牙）：同一件，两命令在**同一口径**上必须报同一个数。
	//   任一命令把自己的轴换成另一面（show 回退成子串面 3 / ls 改成 want 面）⇒ 本格必红。
	if axis := gapTwoAxisNum(t, meta, "axis"); axis != lsCount {
		t.Fatalf("同一件两命令矛盾：show.meta.axis=%v ≠ ls.meta.count=%v", axis, lsCount)
	}

	// ── 人面：三个数都出现、各自标口径 ──
	rc, hout, herrb := gapShowAxisRun(t, "gap", "show", target)
	if rc != 0 {
		t.Fatalf("gap show 人面要 rc=0，得到 %d · stderr=%q", rc, herrb)
	}
	for _, want := range []string{
		"口径A `unit` 精确等值（与 `gap ls --unit " + target + "` **同轴**）: 2 条（其中未闭 2 条）",
		"口径B `want` 面逐字含（want_family/want_action/want_argv · 整路径或基名 · 即 `--json want` 那一格）: 3 条",
		"口径C 全文子串（**整路径** · symptom/handmade/repro_cmd/verify_cmd/证据/注文 + want 面）: 3 条",
	} {
		if !strings.Contains(hout, want) {
			t.Errorf("人面缺口径行（逐字）：%q\n实际：\n%s", want, hout)
		}
	}

	// ── 尾斜杠归一：`gap show <件>/` 与 `gap ls --unit <件>` 同数（同一套 TrimRight）──
	rc, sout, serrb := gapShowAxisRun(t, "gap", "show", target+"/", "--json", "id")
	if rc != 0 {
		t.Fatalf("`gap show <件>/` 要 rc=0，得到 %d · stderr=%q", rc, serrb)
	}
	if got := gapTwoAxisNum(t, gapShowAxisMeta(t, sout), "axis"); got != lsCount {
		t.Errorf("`gap show <件>/` 的轴面数应为 %v（尾斜杠归一同 `gap ls --unit`），得到 %v", lsCount, got)
	}
}

// TestGapShowSymptomOnlyFileAxesDiffer —— 态②：只在 symptom 里出现、不在 want 里的件。
// A=0 · B=0 · C=1：两命令的「同一个数」只在同轴上成立，C 面如实多出 1 条 ⇒ 两轴真不同。
func TestGapShowSymptomOnlyFileAxesDiffer(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	target := "core/cmd/zerg/family_gap.go"
	other := "core/internal/version/version.go"

	recs := []map[string]any{
		// 只在 symptom 里点名该件：unit 是别的件、want 面一个字不提。
		gapTwoAxisRec("GAP-20990202-A", other, "只在症状里提到 "+target+" 的毛病", nil),
	}
	gapShowAxisWrite(t, dir, recs)

	rc, out, errb := gapShowAxisRun(t, "gap", "show", target, "--json", "id")
	if rc != 0 {
		t.Fatalf("gap show 要 rc=0（件在仓里 · 命中 0 不是错），得到 %d · stderr=%q", rc, errb)
	}
	meta := gapShowAxisMeta(t, out)
	axis := gapTwoAxisNum(t, meta, "axis")
	wantAxis := gapTwoAxisNum(t, meta, "axis_want")
	subAxis := gapTwoAxisNum(t, meta, "axis_substring")
	if axis != 0 {
		t.Errorf("口径A（unit 精确等值）应为 0，得到 %v", axis)
	}
	if wantAxis != 0 {
		t.Errorf("口径B（want 面）应为 0（该条 want 不提件），得到 %v", wantAxis)
	}
	if subAxis != 1 {
		t.Errorf("口径C（全文子串）应为 1（symptom 提到该件），得到 %v", subAxis)
	}
	if subAxis == axis {
		t.Fatalf("两轴被砍成同一边：axis=%v · axis_substring=%v（子串面必须如实多出这条）", axis, subAxis)
	}
	if wantAxis == subAxis {
		t.Fatalf("B/C 两面被砍成同一边：axis_want=%v · axis_substring=%v", wantAxis, subAxis)
	}
	if len(gapTwoAxisIDs(t, out)) != 0 {
		t.Errorf("`items[]` 按 A 面应为空，得到 %v", gapTwoAxisIDs(t, out))
	}

	// ls 同轴：0 命中 ⇒ 既有退码 1（与 show 的 A=0 同数；C=1 是 show 的**附带口径**，不是矛盾）。
	rc, _, lerrb := gapShowAxisRun(t, "gap", "ls", "--unit", target, "--json", "id")
	if rc != 1 {
		t.Errorf("gap ls --unit 零命中要仍退 1（既有档），得到 %d · stderr=%q", rc, lerrb)
	}
	if !strings.Contains(lerrb, "零命中") {
		t.Errorf("gap ls --unit 零命中的报话应仍含「零命中」：%q", lerrb)
	}

	// 人面：A/B 都是 0、C 是 1 —— 三个数各自标口径（不许只印一个数）。
	rc, hout, herrb := gapShowAxisRun(t, "gap", "show", target)
	if rc != 0 {
		t.Fatalf("gap show 人面要 rc=0，得到 %d · stderr=%q", rc, herrb)
	}
	for _, want := range []string{
		"口径A `unit` 精确等值（与 `gap ls --unit " + target + "` **同轴**）: 0 条（其中未闭 0 条）",
		"口径B `want` 面逐字含（want_family/want_action/want_argv · 整路径或基名 · 即 `--json want` 那一格）: 0 条",
		"口径C 全文子串（**整路径** · symptom/handmade/repro_cmd/verify_cmd/证据/注文 + want 面）: 1 条",
	} {
		if !strings.Contains(hout, want) {
			t.Errorf("人面缺口径行（逐字）：%q\n实际：\n%s", want, hout)
		}
	}
}

// TestGapLsAxisScopeOnlyOnUnit —— `meta.axis_scope` 只在**件面收窄**（`--unit`）时出现：
// 全量档 / 只给别的收窄旗标时没有「件轴」可言 ⇒ 不写这一格（缺席 ≠ 假值，与 meta 同族口径）。
func TestGapLsAxisScopeOnlyOnUnit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	recs := []map[string]any{gapTwoAxisRec("GAP-20990203-A", "core/cmd/zerg/main.go", "甲", nil)}
	gapShowAxisWrite(t, dir, recs)

	rc, out, errb := gapShowAxisRun(t, "gap", "ls", "--json", "id")
	if rc != 0 {
		t.Fatalf("gap ls 全量档要 rc=0，得到 %d · stderr=%q", rc, errb)
	}
	if _, ok := gapShowAxisMeta(t, out)["axis_scope"]; ok {
		t.Errorf("全量档不该写 `meta.axis_scope`（那时没有件轴）：%s", out)
	}
}
