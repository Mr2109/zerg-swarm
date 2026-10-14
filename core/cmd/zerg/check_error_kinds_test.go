// check_error_kinds_test.go —— Go 真搬样板的**成对负控**定向单测（M2-0 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckErrorKindsGatePort（定向：go test -run TestCheckErrorKindsGatePort）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// cekFixture —— 在 t.TempDir() 下造一棵 `core/cmd/zerg/` 夹具树（仓外）。
func cekFixture(t *testing.T, errorsGo string, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "errors.go"), []byte(errorsGo), 0o644); err != nil {
		t.Fatal(err)
	}
	ec := "var exitcodeTable = []exitcodeRow{\n" +
		"\t{0, \"ok\", \"成功\", \"—\"},\n\t{1, \"failed\", \"一般失败\", \"看 kind\"},\n" +
		"\t{2, \"usage\", \"用法错\", \"改用法\"},\n\t{14, \"conflict\", \"冲突\", \"排队\"},\n}\n"
	if err := os.WriteFile(filepath.Join(p, "exitcodes.go"), []byte(ec), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range extra {
		if err := os.WriteFile(filepath.Join(p, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// ★ detail 面（甲·登记式棘轮 · GAP-20260928-25）的同源登记：夹具里也备**一处落点**
	// 与**它的基线**，而基线由**同一取数口**（cekDetailSitesInFile）现算 ⇒ ✗ 不手抄值、
	// 既有用例的 detail 面恒绿（rc=0），不吃它们的退码。
	if err := os.WriteFile(filepath.Join(p, cekFixtureDetailFile), []byte(cekFixtureDetailSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	cdir := filepath.Join(root, "core", "internal", "contract")
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatal(err)
	}
	bl := cekDetailBuildBaseline(cekDetailSitesInFile("core/cmd/zerg/"+cekFixtureDetailFile, cekFixtureDetailSrc))
	b, err := json.MarshalIndent(bl, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cdir, "error-detail-baseline.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// cekFixtureDetailFile / cekFixtureDetailSrc —— 夹具里那处 detail 落点（单一真源：基线也由它现算）。
const cekFixtureDetailFile = "zz_cek_detail.go"

const cekFixtureDetailSrc = "package main\n\nfunc zzCekFixtureDetail() { setErr(\"conflict\", \"cek_fixture_detail\", \"p\") }\n"

// ── detail 取值口径子判据（甲·登记式棘轮 · 缺口 GAP-20260928-25）====================
//
// 四态（✗ 禁恒绿）：① 干净态（真基线 ＋ 真源码树）⇒ 0；② 注入未登记值（字面量 ＋ 形态）⇒ 1 且逐字
// 点名「件:行 ＋ 值」；③ 改动一枚已登记值（改名）⇒ 1（新值未登记 ＋ 旧值现读消失，**两向**都点名）；
// ④ 基线件缺 / 真源取不到 / 空转 ⇒ 2（不给结论 · 空转不许当绿）。＋ ⑤ 门面（真跑 errorKindsGateMain）
// 注入坏形状 ⇒ 1 且点名、基线缺 / 真源读不到 ⇒ 2。＋ ⑥ 两处**探针回归**（本轮实测踩到的同型坑：
// 宽判据把单行函数里的真调用一起跳掉；窄判据把声明行又收成调用点）。
func TestCheckErrorKindsDetailGateFace(t *testing.T) {
	root, ok := errorKindsGateFindRoot(".")
	if !ok {
		t.Fatalf("解析不到仓根（找不到 core/cmd/zerg/errors.go）—— go test 的 cwd = 包目录")
	}
	realSites, fails := cekScanDetailSites(root)
	nFails := len(fails)
	if nFails > 0 {
		t.Fatalf("真源码树读失败（真源取不到）：%v", fails)
	}
	bl, err := cekDetailLoadBaseline(root, "")
	if err != nil {
		t.Fatalf("基线件读不出来（%v）—— 先落 %s", err, cekDetailBaselineRel)
	}

	// ① 干净态：真基线 ＋ 真源码树 ⇒ 0 · 命中 0 条
	rcClean, linesClean := errorKindsGateDetailCheck(bl, realSites)
	joinedClean := strings.Join(linesClean, "\n")
	if rcClean != 0 {
		t.Errorf("干净态失败：rc=%d（要 0）：%s", rcClean, joinedClean)
	}
	if !strings.Contains(joinedClean, "命中 0 条") {
		t.Errorf("干净态读数缺「命中 0 条」：%s", joinedClean)
	}

	// ② 注入未登记值：字面量 ＋ 非字面量形态各一枚 ⇒ 1 且逐字点名
	injected := append(append([]cekDetailSite{}, realSites...),
		cekDetailSite{Rel: "core/cmd/zerg/zz_injected.go", Line: 77, Literal: true, Value: "f15_unregistered_detail"},
		cekDetailSite{Rel: "core/cmd/zerg/zz_injected.go", Line: 78, Literal: false, Value: "f15Shape"})
	rcNew, linesNew := errorKindsGateDetailCheck(bl, injected)
	joinedNew := strings.Join(linesNew, "\n")
	if rcNew != 1 {
		t.Errorf("未登记态失败：rc=%d（要 1）：%s", rcNew, joinedNew)
	}
	for _, want := range []string{"f15_unregistered_detail", "zz_injected.go:77", "f15Shape", "zz_injected.go:78"} {
		if !strings.Contains(joinedNew, want) {
			t.Errorf("未登记态没点名 %q：%s", want, joinedNew)
		}
	}
	if rcClean == rcNew {
		t.Errorf("成对断言失败：干净态 rc=%d 与未登记态 rc=%d 同码 ⇒ 无鉴别力", rcClean, rcNew)
	}

	// ③ 改动一枚已登记值（改名）：新值未登记 ＋ 旧值现读消失 —— 两向都要点名
	target := bl.Literals[0].Value
	oldSite := bl.Literals[0].Sites[0]
	renamed := make([]cekDetailSite, 0, len(realSites))
	for _, s := range realSites {
		if s.Literal && s.Value == target {
			s.Value = target + "_f15_renamed"
		}
		renamed = append(renamed, s)
	}
	rcRen, linesRen := errorKindsGateDetailCheck(bl, renamed)
	joinedRen := strings.Join(linesRen, "\n")
	if rcRen != 1 {
		t.Errorf("改名态失败：rc=%d（要 1）：%s", rcRen, joinedRen)
	}
	for _, want := range []string{target + "_f15_renamed", target, oldSite} {
		if !strings.Contains(joinedRen, want) {
			t.Errorf("改名态没点名 %q：%s", want, joinedRen)
		}
	}
	if !strings.Contains(joinedRen, "现读消失") {
		t.Errorf("改名态没报「已登记值现读消失」：%s", joinedRen)
	}

	// ④ 基线取不到 / 真源取不到 / 空转 ⇒ 2（不给结论 · ✗ 不假绿）
	rcNoBase, linesNoBase := errorKindsGateDetailCheck(nil, realSites)
	if rcNoBase != 2 || !strings.Contains(strings.Join(linesNoBase, "\n"), "不给结论") {
		t.Errorf("基线取不到失败：rc=%d（要 2 ＋ 不给结论）", rcNoBase)
	}
	rcNoSite, linesNoSite := errorKindsGateDetailCheck(bl, nil)
	if rcNoSite != 2 || !strings.Contains(strings.Join(linesNoSite, "\n"), "不给结论") {
		t.Errorf("真源空转失败：rc=%d（要 2 ＋ 不给结论）", rcNoSite)
	}
	rcEmptyBase, _ := errorKindsGateDetailCheck(&cekDetailBaseline{}, realSites)
	if rcEmptyBase != 2 {
		t.Errorf("空基线失败：rc=%d（要 2 —— 空转不许当绿）", rcEmptyBase)
	}

	// ⑤ 门面：真跑 errorKindsGateMain（注入坏形状 ⇒ 1 且点名；基线缺 / 真源读不到 ⇒ 2）
	oldScan := errorKindsGateDetailScan
	errorKindsGateDetailScan = func(string) ([]cekDetailSite, []string) { return injected, nil }
	rcFace, outFace, _ := cekRun(t, ".")
	errorKindsGateDetailScan = oldScan
	if rcFace != 1 || !strings.Contains(outFace, "f15_unregistered_detail") {
		t.Errorf("门面注入坏形状失败：rc=%d（要 1 ＋ 点名）：%s", rcFace, outFace)
	}
	if !strings.Contains(outFace, "error.detail 取值口径") {
		t.Errorf("门面没打 detail 块：%s", outFace)
	}
	rcFaceNo, outFaceNo, _ := cekRun(t, ".", "--baseline="+filepath.Join(t.TempDir(), "nope.json"))
	if rcFaceNo != 2 || !strings.Contains(outFaceNo, "不给结论") {
		t.Errorf("门面基线缺失败：rc=%d（要 2）：%s", rcFaceNo, outFaceNo)
	}
	errorKindsGateDetailScan = func(string) ([]cekDetailSite, []string) {
		return nil, []string{"core/cmd/zerg/zz_locked.go（权限）"}
	}
	rcFaceFail, outFaceFail, _ := cekRun(t, ".")
	errorKindsGateDetailScan = oldScan
	if rcFaceFail != 2 || !strings.Contains(outFaceFail, "读失败") {
		t.Errorf("门面真源取不到失败：rc=%d（要 2 ＋ 逐件点名）：%s", rcFaceFail, outFaceFail)
	}

	// ⑥ 探针回归：声明行不算落点；单行函数里的真调用**要**算落点
	declSrc := "package main\n\nfunc (inv *invocation) setErr(kind, detail, msg string) {\n\tif inv.err == nil {\n\t\tinv.err = nil\n\t}\n}\n"
	declSites := cekDetailSitesInFile("d.go", declSrc)
	nDecl := len(declSites)
	if nDecl != 0 {
		t.Errorf("声明行被当成调用点：%d 处（要 0）：%v", nDecl, declSites)
	}
	inlineSrc := "package main\n\nfunc f() { setErr(\"conflict\", \"inline_detail\", \"m\") }\n"
	inlineSites := cekDetailSitesInFile("i.go", inlineSrc)
	nInline := len(inlineSites)
	if nInline != 1 {
		t.Fatalf("单行函数里的真调用落点漏了：%d 处（要 1）：%v", nInline, inlineSites)
	}
	if inlineSites[0].Value != "inline_detail" || !inlineSites[0].Literal {
		t.Errorf("单行函数落点取值不对：%+v（要 inline_detail 且为字面量）", inlineSites[0])
	}
}

func cekRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := errorKindsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckErrorKindsGatePort(t *testing.T) {
	good := "var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\n"

	// ── 正控（必然判绿）：闭集合规 + 用法面全在闭集内 ─────────────────
	rcGreen, outGreen, _ := cekRun(t, cekFixture(t, good, map[string]string{
		"usage_ok.go": "package main\nfunc f(){ setErr(\"conflict\") }\n",
	}))
	if rcGreen != 0 || !strings.Contains(outGreen, "⇒ 全绿（rc=0）") {
		t.Fatalf("正控失败：好件应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控 R1（必然判红）：大写 kind ───────────────────────────────
	rcR1, outR1, _ := cekRun(t, cekFixture(t,
		"var errorKinds = []errorKindRow{\n\t{\"CONFIRM_REQUIRED\", 2, false, \"r\", \"m\", \"s\"},\n}\n", nil))
	if rcR1 != 1 || !strings.Contains(outR1, "R1 kind 命名不合规") {
		t.Fatalf("负控 R1 失败：得 rc=%d out=%q", rcR1, outR1)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcR1 {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcR1)
	}

	// ── 负控 R2 · 重复 kind ─────────────────────────────────────────
	rcDup, outDup, _ := cekRun(t, cekFixture(t,
		"var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\n", nil))
	if rcDup != 1 || !strings.Contains(outDup, "R2 kind 重复") {
		t.Fatalf("负控 R2-重复 失败：得 rc=%d out=%q", rcDup, outDup)
	}

	// ── 负控 R2 · 未登记退码（99 不在退码表） ───────────────────────
	rcGhost, outGhost, _ := cekRun(t, cekFixture(t,
		"var errorKinds = []errorKindRow{\n\t{\"ghost_kind\", 99, false, \"r\", \"m\", \"s\"},\n}\n", nil))
	if rcGhost != 1 || !strings.Contains(outGhost, "不在退码表里") {
		t.Fatalf("负控 R2-未登记 失败：得 rc=%d out=%q", rcGhost, outGhost)
	}

	// ── 负控 R2 · 负退码 ────────────────────────────────────────────
	rcNeg, outNeg, _ := cekRun(t, cekFixture(t,
		"var errorKinds = []errorKindRow{\n\t{\"conflict\", -3, false, \"r\", \"m\", \"s\"},\n}\n", nil))
	if rcNeg != 1 || !strings.Contains(outNeg, "R2 退码为负") {
		t.Fatalf("负控 R2-负码 失败：得 rc=%d out=%q", rcNeg, outNeg)
	}

	// ── 负控 R3 · 用法面命中闭集外 kind ─────────────────────────────
	rcR3, outR3, _ := cekRun(t, cekFixture(t, good, map[string]string{
		"usage_bad.go": "package main\nfunc g(){ setErr(\"outside_kind\") }\n",
	}))
	if rcR3 != 1 || !strings.Contains(outR3, "R3 用到闭集外的 kind") {
		t.Fatalf("负控 R3 失败：得 rc=%d out=%q", rcR3, outR3)
	}

	// ── 空转（≠ 绿）：闭集表空 ⇒ rc=2 不给结论 ──────────────────────
	rcEmpty, outEmpty, _ := cekRun(t, cekFixture(t, "var errorKinds = []errorKindRow{\n}\n", nil))
	if rcEmpty != 2 || !strings.Contains(outEmpty, "不给结论") {
		t.Fatalf("空转失败：空闭集应 rc=2，得 rc=%d out=%q", rcEmpty, outEmpty)
	}

	// ── 取不到输入：根下无 core/cmd/zerg/errors.go ⇒ rc=2 ──────────
	rcNoRoot, outNoRoot, _ := cekRun(t, t.TempDir())
	if rcNoRoot != 2 || !strings.Contains(outNoRoot, "找不到 core/cmd/zerg/errors.go") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcNoRoot, outNoRoot)
	}

	// ── Q-176：本门无机器读面 ⇒ --json 拒收 rc=2（stderr 明说） ────
	rcJSON, _, errJSON := cekRun(t, "--json")
	if rcJSON != 2 || !strings.Contains(errJSON, "本门无机器读面") {
		t.Fatalf("--json 拒收失败：得 rc=%d stderr=%q", rcJSON, errJSON)
	}

	// ── --self-test 成对负控自身必须全过（防恒绿/恒红） ─────────────
	rcST, outST, _ := cekRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}

// TestCheckErrorKindsReadFailTeeth —— B 类读失败不再静默的回归牙：R3 扫描面里一件 .go 锁成
// chmod 000 ⇒ 登记面（cekReadFails）必响：rc=2 + stderr BLOCKED 头 + 逐件点名。
// 删回静默（原 os.ReadFile 失败即 continue）本用例必红。
func TestCheckErrorKindsReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 无视权限位（chmod 000 仍可读）⇒ 跳过")
	}
	good := "var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\n"
	root := cekFixture(t, good, map[string]string{
		"usage_ok.go": "package main\nfunc f(){ setErr(\"conflict\") }\n",
		"locked.go":   "package main\nfunc z(){ setErr(\"conflict\") }\n",
	})
	locked := filepath.Join(root, "core", "cmd", "zerg", "locked.go")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod 000 失败：%v", err)
	}
	defer func() { _ = os.Chmod(locked, 0o644) }()
	rc, out, errb := cekRun(t, root)
	if rc != 2 || !strings.Contains(errb, "BLOCKED") || !strings.Contains(errb, "core/cmd/zerg/locked.go") {
		t.Fatalf("读不到件应 rc=2 + BLOCKED 点名 locked.go，得 rc=%d out=%q err=%q", rc, out, errb)
	}
}

// ── N2 子判据（§3.3 `N2` · 缺口 GAP-20261012-20）====================================
//
// 四态（✗ 禁恒绿）：① 干净态（真源 + 真命令树）⇒ 0；② 人为塞一枚近义动词（概要）⇒ 1 且点名
// 「件·段·词」；③ 真源取不到 / 禁词表空 ⇒ 2（不给结论 · 空转不许当绿）；④ 门面（`errorKindsGateMain`）
// 不回归：干净态仍 `⇒ 全绿（rc=0）`、注入坏形状 ⇒ 1 且 N2 块逐字点名。
func TestCheckErrorKindsN2GateFace(t *testing.T) {
	spec, err := contract.SelfUpdate()
	if err != nil {
		t.Fatalf("N2 真源读不出来：%v", err)
	}

	// ① 干净态：真源 + **真命令树** ⇒ rc=0 · 命中 0 条。
	rcClean, linesClean := errorKindsGateN2Check(spec, updateSplitShapes())
	joinedClean := strings.Join(linesClean, "\n")
	if rcClean != 0 {
		t.Errorf("干净态失败：真命令树上 N2 rc=%d（要 0）：%s", rcClean, joinedClean)
	}
	if !strings.Contains(joinedClean, "命中 0 条") {
		t.Errorf("干净态读数缺「命中 0 条」：%s", joinedClean)
	}

	// ② 超口径值：某条命令的**概要**里塞进近义动词 ⇒ rc=1 且点名「件·段·词」。
	badShape := updateCmdShape{Path: "task submit", Summary: "提交任务并顺手升级主控", Usage: "zerg task submit"}
	rcBad, linesBad := errorKindsGateN2Check(spec, append(updateSplitShapes(), badShape))
	joinedBad := strings.Join(linesBad, "\n")
	if rcBad != 1 {
		t.Errorf("命中态失败：rc=%d（要 1）：%s", rcBad, joinedBad)
	}
	for _, want := range []string{"task submit", "概要", "升级"} {
		if !strings.Contains(joinedBad, want) {
			t.Errorf("命中态没点名 %q：%s", want, joinedBad)
		}
	}

	// 成对（✗ 禁恒绿）：干净态与命中态必须不同码。
	if rcClean == rcBad {
		t.Errorf("成对断言失败：干净态 rc=%d 与命中态 rc=%d 同码 ⇒ 无鉴别力", rcClean, rcBad)
	}

	// ③ 真源取不到 / 禁词表空 ⇒ rc=2（不给结论 · 空转 = 假覆盖）。
	rcNil, linesNil := errorKindsGateN2Check(nil, updateSplitShapes())
	if rcNil != 2 || !strings.Contains(strings.Join(linesNil, "\n"), "不给结论") {
		t.Errorf("真源取不到失败：得 rc=%d · %v（要 2 + 不给结论）", rcNil, linesNil)
	}
	rcNoWord, _ := errorKindsGateN2Check(&contract.SelfUpdateSpec{}, updateSplitShapes())
	if rcNoWord != 2 {
		t.Errorf("禁词表空失败：得 rc=%d（要 2 —— 空转不许当绿）", rcNoWord)
	}

	// ④ 门面不回归：go test 的 cwd = core/cmd/zerg ⇒ errorKindsGateFindRoot 上溯得仓根。
	rcEntry, outEntry, _ := cekRun(t, ".")
	if rcEntry != 0 || !strings.Contains(outEntry, "⇒ 全绿（rc=0）") {
		t.Errorf("门面干净态回归：得 rc=%d out=%q", rcEntry, outEntry)
	}
	if !strings.Contains(outEntry, "命令面命名纪律 N2") {
		t.Errorf("门面没打 N2 块：%q", outEntry)
	}
	oldShapes := errorKindsGateN2Shapes
	errorKindsGateN2Shapes = func() []updateCmdShape {
		return append(updateSplitShapes(), badShape)
	}
	defer func() { errorKindsGateN2Shapes = oldShapes }()
	rcEntryBad, outEntryBad, _ := cekRun(t, ".")
	if rcEntryBad != 1 || !strings.Contains(outEntryBad, "task submit") || !strings.Contains(outEntryBad, "升级") {
		t.Errorf("门面命中态失败：得 rc=%d out=%q（要 1 + 逐字点名）", rcEntryBad, outEntryBad)
	}
	if !strings.Contains(outEntryBad, "⇒ 门禁红（rc=1）") {
		t.Errorf("门面命中态裁决行不对：%q", outEntryBad)
	}
}
