// gate_run_partial_test.go —— 部分趟**纯判定面**的定向单测（T-PR-03·04）。
//
// ⚠ 候件（仓外）：拟落路径 `core/cmd/zerg/gate_run_partial_test.go`（**新件**）。只测**不依赖 git/
// 文件系统**的纯函数（选步次序 / 五真源 / 非 PASS / 叠并 / 闭包匹配 / 对齐 / 退码边界）。真执行面
// （`git diff`、逐步 `--only-step`、台账落盘）不在单测内 —— 那属片④ 的成对正负控（`check-partial-run-acceptance.py`）。
package main

import "testing"

func names(xs ...string) []string { return xs }

func TestGatePartialSinceArgs(t *testing.T) {
	cases := []struct {
		tail       []string
		given      bool
		arg        string
		strippedNo int
	}{
		{names(), false, "", 0},
		{names("--since"), true, "", 0},
		{names("--since", "/x/y"), true, "/x/y", 0},
		{names("--since", "--outdir", "Z"), true, "", 1}, // 裸 --since（下一枚是旗标）⇒ --last
		{names("--since=/x"), true, "/x", 0},
		{names("--outdir", "Z", "--since", "/x"), true, "/x", 1},
	}
	for i, c := range cases {
		if got := gatePartialSinceGiven(c.tail); got != c.given {
			t.Fatalf("case %d given=%v want %v", i, got, c.given)
		}
		if got := gatePartialSinceArg(c.tail); got != c.arg {
			t.Fatalf("case %d arg=%q want %q", i, got, c.arg)
		}
	}
	// strip：`--since`（连值）从 tail 里去掉，其余逐字保留。
	if got := gatePartialStripSince(names("--since", "/x", "--outdir", "Z")); !pEq(got, names("--outdir", "Z")) {
		t.Fatalf("strip=%v", got)
	}
	if got := gatePartialStripSince(names("--since")); len(got) != 0 {
		t.Fatalf("strip bare=%v", got)
	}
	if got := gatePartialStripSince(names("--since=/x")); len(got) != 0 {
		t.Fatalf("strip eq=%v", got)
	}
}

func TestGatePartialOverlap(t *testing.T) {
	if got := gatePartialOverlap(names("--since", "--scope", "go")); got != "--scope" {
		t.Fatalf("overlap scope=%q", got)
	}
	if got := gatePartialOverlap(names("--since", "--fast")); got != "--fast" {
		t.Fatalf("overlap fast=%q", got)
	}
	if got := gatePartialOverlap(names("--since", "--step", "x")); got != "--step" {
		t.Fatalf("overlap step=%q", got)
	}
	if got := gatePartialOverlap(names("--since", "--outdir", "Z")); got != "" {
		t.Fatalf("overlap clean=%q", got)
	}
}

func TestGatePartialFiveTruthsHit(t *testing.T) {
	ch := names("core/x.go", "scripts/gates/precommit-gates.sh", "scripts/gates/judge-registry.json", "docs/a.md")
	got := gatePartialFiveTruthsHit(ch)
	if !pEq(got, names("scripts/gates/precommit-gates.sh", "scripts/gates/judge-registry.json")) {
		t.Fatalf("fiveTruths=%v", got)
	}
	if len(gatePartialFiveTruths) != 5 {
		t.Fatalf("白名单必须恰 5 件（P6），实得 %d", len(gatePartialFiveTruths))
	}
}

func TestGatePartialNonPassP2(t *testing.T) {
	rows := []gateResultsRow{
		{Name: "a", Status: "PASS"},
		{Name: "b", Status: "FAIL"},
		{Name: "c", Status: "BLOCKED"},
		{Name: "d", Status: "REPORT"},
	}
	steps, fail, blocked, report := gatePartialNonPass(rows)
	if !pEq(steps, names("b", "c", "d")) || fail != 1 || blocked != 1 || report != 1 {
		t.Fatalf("nonPass=%v f=%d b=%d r=%d", steps, fail, blocked, report)
	}
}

func TestGatePartialOrderR(t *testing.T) {
	full := names("a", "b", "c")
	set := map[string]bool{"c": true, "a": true, "zz": true}
	if got := gatePartialOrderR(full, set); !pEq(got, names("a", "c", "zz")) {
		t.Fatalf("orderR=%v", got)
	}
}

func TestGatePartialAlignOK(t *testing.T) {
	mp := &gatePartialAffectedMap{Faces: map[string][]string{"core": {"a", "b"}, "docs": {"c"}}}
	if ok, why := gatePartialAlignOK(mp, names("a", "b", "c")); !ok {
		t.Fatalf("align want ok, got %v (%s)", ok, why)
	}
	mp2 := &gatePartialAffectedMap{Faces: map[string][]string{"core": {"a", "b"}}}
	if ok, _ := gatePartialAlignOK(mp2, names("a", "b", "c")); ok {
		t.Fatal("align D!=N 必须不过")
	}
	mp3 := &gatePartialAffectedMap{Faces: map[string][]string{"core": {"a", "b"}, "docs": {"zzz"}}}
	if ok, _ := gatePartialAlignOK(mp3, names("a", "b", "c")); ok {
		t.Fatal("align 幻影名必须不过")
	}
}

func TestGatePartialEntryMatch(t *testing.T) {
	core := gatePartialMapEntry{Pattern: "core/**", Kind: "prefix"}
	if !gatePartialEntryMatch(core, "core/x/y.go") || !gatePartialEntryMatch(core, "core") {
		t.Fatal("prefix 不匹配")
	}
	if gatePartialEntryMatch(core, "corex/y.go") {
		t.Fatal("prefix 误匹配")
	}
	lit := gatePartialMapEntry{Pattern: "core/cmd/zerg/main.go", Kind: "literal"}
	if !gatePartialEntryMatch(lit, "core/cmd/zerg/main.go") || gatePartialEntryMatch(lit, "core/cmd/zerg/main.go2") {
		t.Fatal("literal 口径错")
	}
	g := gatePartialMapEntry{Pattern: "scripts/**/*.py", Kind: "glob"}
	if !gatePartialEntryMatch(g, "scripts/gates/check-x.py") || gatePartialEntryMatch(g, "scripts/gates/check-x.sh") {
		t.Fatal("glob 口径错")
	}
}

func TestGatePartialDecideOrderAndThreshold(t *testing.T) {
	full := names("s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9")

	// ① undeclared 命中 ⇒ 全量，且**先于**五真源并列印（两因都在 ⇒ 两条 reason）。
	p1 := &gatePartialPlan{N: len(full), Undeclared: names("docs/x"), FiveTruths: names("scripts/gates/affected-map.json")}
	gatePartialDecide(p1, nil, nil, full)
	if !p1.Full || len(p1.Reasons) != 2 {
		t.Fatalf("① undeclared+五真源 应全量且并列两条：full=%v reasons=%v", p1.Full, p1.Reasons)
	}
	if !pHasSub(p1.Reasons[0], "未声明件") || !pHasSub(p1.Reasons[1], "五真源") {
		t.Fatalf("并列次序错：%v", p1.Reasons)
	}

	// ② 仅五真源 ⇒ 全量（不判阀）。
	p2 := &gatePartialPlan{N: len(full), FiveTruths: names("scripts/gates/precommit-gates.sh")}
	gatePartialDecide(p2, nil, nil, full)
	if !p2.Full || len(p2.Reasons) != 1 {
		t.Fatalf("② 五真源 应全量：%v", p2.Reasons)
	}

	// ③ 恰 60.0%（6/10）⇒ **转全量**（P5 边界已拍）。
	p3 := &gatePartialPlan{N: len(full)}
	gatePartialDecide(p3, nil, names("s0", "s1", "s2", "s3", "s4", "s5"), full)
	if !p3.Full {
		t.Fatal("③ 恰 60.0% 必须转全量（P5）")
	}

	// ⑤ 5/10 = 50% < 60% ⇒ 出部分趟。
	p5 := &gatePartialPlan{N: len(full)}
	gatePartialDecide(p5, nil, names("s0", "s1", "s2", "s3", "s4"), full)
	if p5.Full {
		t.Fatalf("⑤ 50%% 应出部分趟：%v", p5.Reasons)
	}
	if !pEq(p5.R, names("s0", "s1", "s2", "s3", "s4")) {
		t.Fatalf("R=%v", p5.R)
	}
}

func TestGatePartialExitNeverZero(t *testing.T) {
	if got := gatePartialExit(nil); got != 3 {
		t.Fatalf("全绿必须退 3（部分趟独有 · 绝不退 0），实得 %d", got)
	}
	if got := gatePartialExit([]gatePartialRow{{Status: "FAIL"}}); got != 1 {
		t.Fatalf("有 FAIL 退 1，实得 %d", got)
	}
	if got := gatePartialExit([]gatePartialRow{{Status: "BLOCKED"}}); got != 2 {
		t.Fatalf("有 BLOCKED 退 2，实得 %d", got)
	}
	if got := gatePartialExit([]gatePartialRow{{Status: "PASS"}, {Status: "PASS"}}); got != 3 {
		t.Fatalf("全 PASS ⇒ 3，实得 %d", got)
	}
}

// ── 小工具 ──
func pEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func pHasSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
