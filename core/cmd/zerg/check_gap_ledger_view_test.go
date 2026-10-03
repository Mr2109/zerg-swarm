package main

// check_gap_ledger_view_test.go —— `check_gap_ledger_view.go` 的**成对负控**测试件。
//
// 记法：正控（本门该绿）＋ 负控（本门该红/该 rc=2）各成子用例；每条「顺手止血」都配一条能抳住它的回归牙。
// 测件里 python3 一律钉成 /usr/bin/python3（GAP-20261001-61）。夹具一律落 t.TempDir()（仓外），跑后计数 0。

import (
	"io"
	"os"
	"strings"
	"testing"
)

const glvTestPython = "/usr/bin/python3"

// glvTestFixture —— 合成夹具：仓根（含 scripts/gates + bin/zerg 占位）+ 状态目录 + 文档仓。
func glvTestFixture(t *testing.T, srcRecs []glvFixtureRec, expMd string, hasExp bool, ledLines []string, hasLed bool, exportRC int) (repo, docs, src, exportBin, fixturePy string) {
	t.Helper()
	base := t.TempDir()
	repo = glvPyJoin(base, "repo")
	if err := os.MkdirAll(glvPyJoin(repo, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	exportBin = glvPyJoin(repo, "bin", "zerg")
	if err := os.MkdirAll(glvPyJoin(repo, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 占位件：run() 只查「导出器在盘」，真执行走 exportCmd ⇒ 内容无关。
	if err := os.WriteFile(exportBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := glvPyJoin(base, "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	src = glvPyJoin(state, glvSourceName)
	var lines []string
	for _, r := range srcRecs {
		lines = append(lines, glvFixtureSrcLine(r))
	}
	if err := os.WriteFile(src, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(glvPyJoin(state, "export-rc"), []byte(cglvItoa(exportRC)), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasExp {
		if err := os.WriteFile(glvPyJoin(state, "export-fixture.md"), []byte(expMd), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fixturePy = glvPyJoin(base, "fixture-export.py")
	if err := os.WriteFile(fixturePy, []byte(glvFixtureExportPy), 0o644); err != nil {
		t.Fatal(err)
	}
	docs = glvPyJoin(base, "Zerg-内部文档")
	vdir := glvPyJoin(docs, glvLedgerDir, "vX")
	if err := os.MkdirAll(vdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if hasLed {
		if err := os.WriteFile(glvPyJoin(vdir, "缺口总账-20260926.md"), []byte(strings.Join(ledLines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return
}

func cglvItoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func glvTestCall(repo, docs, src, exportBin, fixturePy string) (int, string) {
	var buf strings.Builder
	rc := glvRun(repo, docs, src, false, []string{glvTestPython, fixturePy}, repo, exportBin, &buf)
	return rc, buf.String()
}

// 正控：三方一致 + 体积/单行合规 ⇒ rc=0。
func TestGapLedgerViewPositiveControl(t *testing.T) {
	A := glvMkRec("GAP-20260926-01")
	A.Symptom = "甲"
	B := glvMkRec("GAP-20260926-02")
	B.State, B.Symptom = "已解", "乙"
	exp := glvMkExportMd([]glvFixtureRec{A}, 2, 1, 0, 200)
	repo, docs, src, exportBin, fx := glvTestFixture(t, []glvFixtureRec{A, B}, exp, true,
		glvLedOf([]glvFixtureRec{A}, true, nil), true, 0)
	rc, out := glvTestCall(repo, docs, src, exportBin, fx)
	if rc != 0 {
		t.Fatalf("正控该 rc=0，实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "三方一致 + 体积/单行合规（rc=0）") {
		t.Fatalf("正控缺关键句：\n%s", out)
	}
}

// 负控 R1a（有牙）：真源未闭多一条、导出缺 ⇒ rc=1 且**点名**缺哪个。
func TestGapLedgerViewNegR1aMissing(t *testing.T) {
	A := glvMkRec("GAP-20260926-01")
	A.Symptom = "甲"
	B := glvMkRec("GAP-20260926-02")
	B.State, B.Symptom = "已解", "乙"
	C := glvMkRec("GAP-20260926-03")
	C.Symptom = "丙"
	exp := glvMkExportMd([]glvFixtureRec{A}, 2, 1, 0, 200) // 只有 A
	repo, docs, src, exportBin, fx := glvTestFixture(t, []glvFixtureRec{A, C, B}, exp, true,
		glvLedOf([]glvFixtureRec{A}, true, nil), true, 0)
	rc, out := glvTestCall(repo, docs, src, exportBin, fx)
	if rc != 1 {
		t.Fatalf("负控该 rc=1，实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "[R1a]") || !strings.Contains(out, "`GAP-20260926-03`") {
		t.Fatalf("负控未点名缺失号 `GAP-20260926-03`：\n%s", out)
	}
	// 负控配对：只把导出补齐 ⇒ 回绿（证明牙咬的是这一格、不是恒红）
	exp2 := glvMkExportMd([]glvFixtureRec{A, C}, 3, 1, 0, 200)
	repo2, docs2, src2, eb2, fx2 := glvTestFixture(t, []glvFixtureRec{A, C, B}, exp2, true,
		glvLedOf([]glvFixtureRec{A, C}, true, nil), true, 0)
	if rc2, out2 := glvTestCall(repo2, docs2, src2, eb2, fx2); rc2 != 0 {
		t.Fatalf("补齐后该回绿 rc=0，实 rc=%d\n%s", rc2, out2)
	}
}

// 负控 R1b：账件缺生成标记 ⇒ rc=1「生成件未接管」。
func TestGapLedgerViewNegR1bNoMarker(t *testing.T) {
	A := glvMkRec("GAP-20260926-01")
	A.Symptom = "甲"
	B := glvMkRec("GAP-20260926-02")
	B.State, B.Symptom = "已解", "乙"
	exp := glvMkExportMd([]glvFixtureRec{A}, 2, 1, 0, 200)
	repo, docs, src, exportBin, fx := glvTestFixture(t, []glvFixtureRec{A, B}, exp, true,
		glvLedOf([]glvFixtureRec{A}, false, nil), true, 0)
	rc, out := glvTestCall(repo, docs, src, exportBin, fx)
	if rc != 1 || !strings.Contains(out, "生成件未接管") {
		t.Fatalf("该 rc=1 且含「生成件未接管」，实 rc=%d\n%s", rc, out)
	}
}

// 负控 R2：账件 300+ 行 ⇒ rc=1（逐字打实际值）。
func TestGapLedgerViewNegR2TooManyLines(t *testing.T) {
	A := glvMkRec("GAP-20260926-01")
	A.Symptom = "甲"
	B := glvMkRec("GAP-20260926-02")
	B.State, B.Symptom = "已解", "乙"
	exp := glvMkExportMd([]glvFixtureRec{A}, 2, 1, 0, 200)
	led := glvLedOf([]glvFixtureRec{A}, true, nil)
	for i := 0; i < 305; i++ {
		led = append(led, "<!-- pad -->")
	}
	repo, docs, src, exportBin, fx := glvTestFixture(t, []glvFixtureRec{A, B}, exp, true, led, true, 0)
	rc, out := glvTestCall(repo, docs, src, exportBin, fx)
	if rc != 1 || !strings.Contains(out, "[R2]") {
		t.Fatalf("该 rc=1 且含 [R2]，实 rc=%d\n%s", rc, out)
	}
}

// 负控 R5：真源不在盘 ⇒ rc=2（不拿读不到当没问题）。
func TestGapLedgerViewR5SourceAbsent(t *testing.T) {
	A := glvMkRec("GAP-20260926-01")
	A.Symptom = "甲"
	exp := glvMkExportMd([]glvFixtureRec{A}, 1, 0, 0, 200)
	repo, docs, src, exportBin, fx := glvTestFixture(t, []glvFixtureRec{A}, exp, true,
		glvLedOf([]glvFixtureRec{A}, true, nil), true, 0)
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	rc, out := glvTestCall(repo, docs, src, exportBin, fx)
	if rc != 2 || !strings.Contains(out, "不给结论") {
		t.Fatalf("该 rc=2 且含「不给结论」，实 rc=%d\n%s", rc, out)
	}
}

// 负控 R5：导出跑不动（rc=3）⇒ rc=2（不许当绿）。
func TestGapLedgerViewR5ExportFails(t *testing.T) {
	A := glvMkRec("GAP-20260926-01")
	A.Symptom = "甲"
	exp := glvMkExportMd([]glvFixtureRec{A}, 1, 0, 0, 200)
	repo, docs, src, exportBin, fx := glvTestFixture(t, []glvFixtureRec{A}, exp, true,
		glvLedOf([]glvFixtureRec{A}, true, nil), true, 3)
	rc, out := glvTestCall(repo, docs, src, exportBin, fx)
	if rc != 2 || !strings.Contains(out, "跑不动") {
		t.Fatalf("该 rc=2 且含「跑不动」，实 rc=%d\n%s", rc, out)
	}
}

// 负控 R5：文档仓一件账都没有 ⇒ rc=2。
func TestGapLedgerViewR5NoLedger(t *testing.T) {
	A := glvMkRec("GAP-20260926-01")
	A.Symptom = "甲"
	exp := glvMkExportMd([]glvFixtureRec{A}, 1, 0, 0, 200)
	repo, docs, src, exportBin, fx := glvTestFixture(t, []glvFixtureRec{A}, exp, true, nil, false, 0)
	rc, out := glvTestCall(repo, docs, src, exportBin, fx)
	if rc != 2 || !strings.Contains(out, "一件当期") {
		t.Fatalf("该 rc=2 且含「一件当期」，实 rc=%d\n%s", rc, out)
	}
}

// B 类真口子钉法：读不了的件（chmod 000）⇒ 本门照 rc=2，不静默绿。
func TestGapLedgerViewUnreadableSourcePinned(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 下 chmod 000 仍可读 ⇒ 该子用例无意义")
	}
	A := glvMkRec("GAP-20260926-01")
	A.Symptom = "甲"
	exp := glvMkExportMd([]glvFixtureRec{A}, 1, 0, 0, 200)
	repo, docs, src, exportBin, fx := glvTestFixture(t, []glvFixtureRec{A}, exp, true,
		glvLedOf([]glvFixtureRec{A}, true, nil), true, 0)
	if err := os.Chmod(src, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(src, 0o644)
	rc, out := glvTestCall(repo, docs, src, exportBin, fx)
	if rc != 2 || !strings.Contains(out, "不给结论") {
		t.Fatalf("读不了的件该 rc=2，实 rc=%d\n%s", rc, out)
	}
}

// 正控 + 回归牙：--self-test 全过，且**自检夹具零残留**（钉住 defer os.RemoveAll 这一止血动作）。
func TestGapLedgerViewSelfTestAndTempdirDelta(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
	before := countGlvSelfTestDirs()
	var buf strings.Builder
	rc := gapLedgerViewGateMain(&buf, io.Discard, []string{"--self-test"})
	after := countGlvSelfTestDirs()
	out := buf.String()
	if rc != 0 {
		t.Fatalf("--self-test 该 rc=0，实 rc=%d\n%s", rc, out)
	}
	if !strings.Contains(out, "自检结论: 全过") {
		t.Fatalf("--self-test 该「全过」：\n%s", out)
	}
	if after != before {
		t.Fatalf("自检夹具泄漏：跑前 %d 个 · 跑后 %d 个（defer os.RemoveAll 失效？）", before, after)
	}
}

func countGlvSelfTestDirs() int {
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "glv-selftest-") {
			n++
		}
	}
	return n
}
