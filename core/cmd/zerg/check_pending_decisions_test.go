// check_pending_decisions_test.go —— Go 真搬的**成对负控**定向单测（M2 · 「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckPendingDecisionsGatePort（定向：go test -run TestCheckPendingDecisionsGatePort）。
//
// 外部树一律**显式入参旋钮**：夹具落 `t.TempDir()`（仓外合成树），根/文档树经 `--root`/`--docs-root`
// 显式给 ⇒ 不碰真作业单、不依赖 cwd、不读仓外真 `Zerg-内部文档/`。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cpdTestFixture —— 造一份合成夹具（假仓 root + 假文档树 docs），返回两者。
func cpdTestFixture(t *testing.T, name, src string, rows [][2]string, regText *string) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	root, docs, _, err := cpdMkFixture(tmp, name, src, rows, nil, regText)
	if err != nil {
		t.Fatalf("夹具建不了：%v", err)
	}
	return root, docs
}

func cpdTestRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkPendingDecisionsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func cpdTestMust(t *testing.T, cond bool, format string, args ...interface{}) {
	t.Helper()
	if !cond {
		t.Fatalf(format, args...)
	}
}

func TestCheckPendingDecisionsGatePort(t *testing.T) {
	baseRows := [][2]string{{"A", "已入账"}, {"B", "待拍"}, {"1", "待拍"}, {"2", "待拍"}}

	// ── 正控（必然判绿）：4 条全在闭集内 + 条号对拍 + 依据唯一 ⇒ rc=0 ────────────
	gRoot, gDocs := cpdTestFixture(t, "green", cpdFixSrc, baseRows, nil)
	rcGreen, outGreen, errGreen := cpdTestRun(t, "--root", gRoot, "--docs-root", gDocs)
	cpdTestMust(t, rcGreen == 0 && strings.Contains(outGreen, "全在位"),
		"正控失败：好件应 rc=0，得 rc=%d out=%q err=%q", rcGreen, outGreen, errGreen)
	cpdTestMust(t, strings.Contains(outGreen, "条 4：已拍 0 · 已入账 1 · 待拍 3"),
		"正控条数逐字失败：out=%q", outGreen)
	cpdTestMust(t, strings.Contains(outGreen, "行命中 1（**对照格**"),
		"正控行命中对照格缺：out=%q", outGreen)

	// ── 负控（必然判红）①：拍板态值域外 `已派` ⇒ rc=1 + 点名「件:行 + 原样值」──────
	dRoot, dDocs := cpdTestFixture(t, "domain", cpdFixSrc,
		[][2]string{{"A", "已入账"}, {"B", "已派"}, {"1", "待拍"}, {"2", "待拍"}}, nil)
	rcDom, outDom, _ := cpdTestRun(t, "--root", dRoot, "--docs-root", dDocs)
	cpdTestMust(t, rcDom == 1 && strings.Contains(outDom, "值域外") &&
		strings.Contains(outDom, "`已派`") && strings.Contains(outDom, filepath.Base(cpdRegisterRel)+":"),
		"负控-值域外 失败：得 rc=%d out=%q", rcDom, outDom)
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	cpdTestMust(t, rcGreen != rcDom, "成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcDom)

	// ── 负控②：登记件缺条（源 4 / 登记 3）⇒ rc=1 且条号点名 ────────────────────
	cRoot, cDocs := cpdTestFixture(t, "count", cpdFixSrc, baseRows[:3], nil)
	rcCnt, outCnt, _ := cpdTestRun(t, "--root", cRoot, "--docs-root", cDocs)
	cpdTestMust(t, rcCnt == 1 && (strings.Contains(outCnt, "条号 `2`") || strings.Contains(outCnt, "条数")),
		"负控-缺条 失败：得 rc=%d out=%q", rcCnt, outCnt)

	// ── 负控③：依据引文 0 命中 ⇒ rc=1 + 命中 0 处 ─────────────────────────────
	reg0 := strings.Replace(cpdBuildRegister(baseRows, nil), "已入账 ✓（`Q-901`） |", "这句在作业单里找不到-zz |", 1)
	r0Root, r0Docs := cpdTestFixture(t, "anchor0", cpdFixSrc, baseRows, &reg0)
	rcA0, outA0, _ := cpdTestRun(t, "--root", r0Root, "--docs-root", r0Docs)
	cpdTestMust(t, rcA0 == 1 && strings.Contains(outA0, "命中 0 处"),
		"负控-锚0命中 失败：得 rc=%d out=%q", rcA0, outA0)

	// ── 负控④：依据引文多命中（`现况` 4 处）⇒ rc=1 + 命中 4 处 ──────────────────
	reg4 := strings.Replace(cpdBuildRegister(baseRows, nil), "已入账 ✓（`Q-901`）", "现况", 1)
	r4Root, r4Docs := cpdTestFixture(t, "anchor4", cpdFixSrc, baseRows, &reg4)
	rcA4, outA4, _ := cpdTestRun(t, "--root", r4Root, "--docs-root", r4Docs)
	cpdTestMust(t, rcA4 == 1 && strings.Contains(outA4, "命中 4 处"),
		"负控-锚多命中 失败：得 rc=%d out=%q", rcA4, outA4)

	// ── 负控⑤：头部声明「**9 条**」≠ 现读 2 行 ⇒ rc=1 ────────────────────────
	src5 := strings.Replace(cpdFixSrc, "**2 条**", "**9 条**", 1)
	d5Root, d5Docs := cpdTestFixture(t, "decl", src5, baseRows, nil)
	rcDecl, outDecl, _ := cpdTestRun(t, "--root", d5Root, "--docs-root", d5Docs)
	cpdTestMust(t, rcDecl == 1 && strings.Contains(outDecl, "≠ §一–§四 现读表行数"),
		"负控-声明不符 失败：得 rc=%d out=%q", rcDecl, outDecl)

	// ── 不给结论（≠ 绿）：空转（条表 0 行）⇒ rc=2 ─────────────────────────────
	src7 := "# 合成作业单\n\n> **性质**：0 条。\n\n## 一 · 机器读面\n\n| # | 事项 | 现读实况 | 我的推荐 |\n|---|---|---|---|\n"
	eRoot, eDocs := cpdTestFixture(t, "empty", src7, baseRows, nil)
	rcEmp, outEmp, _ := cpdTestRun(t, "--root", eRoot, "--docs-root", eDocs)
	cpdTestMust(t, rcEmp == 2 && strings.Contains(outEmp, "不给结论"),
		"负控-空转 失败：得 rc=%d out=%q", rcEmp, outEmp)
	cpdTestMust(t, rcEmp != rcGreen && rcEmp != rcDom, "成对断言失败：不给结论 rc=%d 与其它态同码 ⇒ 边界无牙", rcEmp)

	// ── 不给结论：作业单不在盘 ⇒ rc=2（✗ 不静默绿）─────────────────────────────
	nsRoot, nsDocs := cpdTestFixture(t, "nosrc", cpdFixSrc, baseRows, nil)
	cpdTestMust(t, os.Remove(filepath.Join(nsDocs, filepath.FromSlash(cpdSourceRel))) == nil, "删源件失败")
	rcNS, outNS, _ := cpdTestRun(t, "--root", nsRoot, "--docs-root", nsDocs)
	cpdTestMust(t, rcNS == 2 && strings.Contains(outNS, "作业单不在盘"),
		"负控-作业单不在盘 失败：得 rc=%d out=%q", rcNS, outNS)

	// ── 不给结论：登记件不在盘 ⇒ rc=2 ────────────────────────────────────────
	nrRoot, nrDocs := cpdTestFixture(t, "noreg", cpdFixSrc, baseRows, nil)
	cpdTestMust(t, os.Remove(filepath.Join(nrDocs, filepath.FromSlash(cpdRegisterRel))) == nil, "删登记件失败")
	rcNR, outNR, _ := cpdTestRun(t, "--root", nrRoot, "--docs-root", nrDocs)
	cpdTestMust(t, rcNR == 2 && strings.Contains(outNR, "登记件不在盘"),
		"负控-登记件不在盘 失败：得 rc=%d out=%q", rcNR, outNR)

	// ── 不给结论：登记面表头逐字句找不到 ⇒ rc=2 ──────────────────────────────
	bRoot, bDocs := cpdTestFixture(t, "badhdr", cpdFixSrc, baseRows, nil)
	cpdTestMust(t, os.WriteFile(filepath.Join(bDocs, filepath.FromSlash(cpdRegisterRel)),
		[]byte("# 登记 · 合成\n\n> 闭集：`已拍` / `已入账` / `待拍`\n"), 0o644) == nil, "写坏表头失败")
	rcBH, outBH, _ := cpdTestRun(t, "--root", bRoot, "--docs-root", bDocs)
	cpdTestMust(t, rcBH == 2 && strings.Contains(outBH, "表头逐字句找不到"),
		"负控-表头找不到 失败：得 rc=%d out=%q", rcBH, outBH)

	// ── 不给结论：计数句与现读不符 ⇒ rc=2 ────────────────────────────────────
	reg6 := strings.Replace(cpdBuildRegister(baseRows, nil), "条 4：已拍 0 · 已入账 1 · 待拍 3",
		"条 99：已拍 0 · 已入账 0 · 待拍 0", 1)
	s6Root, s6Docs := cpdTestFixture(t, "stale", cpdFixSrc, baseRows, &reg6)
	rcSt, outSt, _ := cpdTestRun(t, "--root", s6Root, "--docs-root", s6Docs)
	cpdTestMust(t, rcSt == 2 && strings.Contains(outSt, "缺现读计数句"),
		"负控-计数句陈腐 失败：得 rc=%d out=%q", rcSt, outSt)

	// ── 退 2 边界（★ 退 2 判准的牙）：根不在盘（无 scripts/gates）⇒ rc=2 判不了根 ──
	rcNRoot, outNRoot, _ := cpdTestRun(t, "--root", filepath.Join(gRoot, "no-such-root"), "--docs-root", gDocs)
	cpdTestMust(t, rcNRoot == 2 && strings.Contains(outNRoot, "判不了根"),
		"退2边界-根不在盘 失败：得 rc=%d out=%q", rcNRoot, outNRoot)
	// ★ 树在、件在 ⇒ **不**退 2（照判）—— 与本件「退化版」判准成对：rc=0 ≠ rc=2
	cpdTestMust(t, rcGreen != rcNRoot, "成对断言失败：树在件在 rc=%d 与取不到 rc=%d 同码 ⇒ 边界无牙", rcGreen, rcNRoot)

	// ── `--json`：形状自证（kind/items/line_hits/count_line）⇒ rc=0 ────────────
	rcJ, outJ, _ := cpdTestRun(t, "--root", gRoot, "--docs-root", gDocs, "--json")
	cpdTestMust(t, rcJ == 0 && strings.Contains(outJ, `"kind": "PendingDecisions"`) &&
		strings.Contains(outJ, `"items": 4`) && strings.Contains(outJ, `"count_line": "条 4：已拍 0 · 已入账 1 · 待拍 3"`) &&
		strings.Contains(outJ, `"warnings": []`) && strings.Contains(outJ, `"truncated": false`),
		"--json 失败：得 rc=%d out=%q", rcJ, outJ)

	// ── `--counts`：人面两行 ⇒ rc=0 ──────────────────────────────────────────
	rcC, outC, _ := cpdTestRun(t, "--root", gRoot, "--docs-root", gDocs, "--counts")
	cpdTestMust(t, rcC == 0 && strings.HasPrefix(outC, "条 4：已拍 0 · 已入账 1 · 待拍 3\n") &&
		strings.Contains(outC, "行命中 1（对照："),
		"--counts 失败：得 rc=%d out=%q", rcC, outC)

	// ── `--list-rules` ⇒ rc=0 ────────────────────────────────────────────────
	rcL, outL, _ := cpdTestRun(t, "--list-rules")
	cpdTestMust(t, rcL == 0 && strings.Contains(outL, "判据与登记面") &&
		strings.Contains(outL, cpdRegisterHeader),
		"--list-rules 失败：得 rc=%d out=%q", rcL, outL)

	// ── 未知旗标 ⇒ rc=2（argparse 用法档 · 正文不属逐字面）──────────────────────
	rcU, _, errU := cpdTestRun(t, "--bogus-flag-zz")
	cpdTestMust(t, rcU == 2 && strings.Contains(errU, "unrecognized arguments"),
		"未知旗标 失败：得 rc=%d err=%q", rcU, errU)

	// ── `--self-test` 成对负控自身必须全过（防恒绿/恒红）───────────────────────
	rcST, outST, _ := cpdTestRun(t, "--self-test")
	cpdTestMust(t, rcST == 0 && strings.Contains(outST, "自检结论: 全过") &&
		strings.Contains(outST, "⑧ 负控：作业单不在盘 ⇒ rc=2") &&
		strings.Contains(outST, "⑩ 负控：灌 5 行关键词"),
		"--self-test 失败：得 rc=%d out=%q", rcST, outST)
}
