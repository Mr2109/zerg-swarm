// check_docs_orphans_links_test.go —— Go 真搬的**成对负控**定向单测（M2 · 文仓/外部树依赖类）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然出红明细」「不给结论」三态，且钉住**退 2 边界的牙**
// （逐字沿用 py · 缺件 ≠ 绿 · 承 GAP-20260927-82 / GAP-20261005-69）：
// 「两棵外部树都取不到 ⇒ rc=2」「只剩一棵（导出树缺）⇒ 仍 rc=2」「第二棵树在盘但真扫面 0 件 ⇒ rc=2」
// 与「三棵树全好 ⇒ rc=0」**必须不同码**（✗ 恒 2 / ✗ 恒 0）。
// 用例名：TestCheckDocsOrphansLinksGatePort（定向：go test -run TestCheckDocsOrphansLinksGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cdoWrite —— 造件（自动建父目录）。
func cdoWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cdoFixture —— 造一棵合成夹具（仓外）：repo 主仓 + docs 第二棵树 + pub 导出树。
// mutate ∈ {"", "broken", "orphan", "stale"}。
func cdoFixture(t *testing.T, mutate string) (repo, docs, pub string) {
	t.Helper()
	base := t.TempDir()
	repo = filepath.Join(base, "Zerg")
	docs = filepath.Join(base, "Zerg-内部文档")
	pub = filepath.Join(base, "pub")
	cdoWrite(t, filepath.Join(repo, "a.md"), "# a\n\n[好](b.md)\n")
	cdoWrite(t, filepath.Join(repo, "b.md"), "# b\n\n[回](a.md)\n")
	cdoWrite(t, filepath.Join(docs, "d1.md"), "# d1\n\n[去](d2.md)\n")
	cdoWrite(t, filepath.Join(docs, "d2.md"), "# d2\n\n[回](d1.md)\n")
	cdoWrite(t, filepath.Join(pub, "p1.md"), "# p1\n\n[去](p2.md)\n")
	cdoWrite(t, filepath.Join(pub, "p2.md"), "# p2\n\n[回](p1.md)\n")
	switch mutate {
	case "broken":
		cdoWrite(t, filepath.Join(repo, "x.md"), "# x\n\n[坏](nope.md)\n")
	case "orphan":
		cdoWrite(t, filepath.Join(pub, "loose.md"), "# loose\n")
	case "stale":
		// 登记件在域内、却**一条原位错都没有** ⇒ 登记过期（基准偏移登记不再需要）。
		cdoWrite(t, filepath.Join(repo, "publish", "docs", "QUICKSTART.en.md"), "# q\n")
		cdoWrite(t, filepath.Join(repo, "publish", "docs", "README.md"), "# r\n\n见 [q](QUICKSTART.en.md)。\n")
	}
	return repo, docs, pub
}

func cdoRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkDocsOrphansLinksGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckDocsOrphansLinksGatePort(t *testing.T) {
	// ── 正控（必然判绿）：三棵树全好 ⇒ rc=0 · 各格 0 件 ────────────────────
	repo, docs, pub := cdoFixture(t, "")
	rcGreen, outGreen, errGreen := cdoRun(t, "--repo-root", repo, "--docs-root", docs, "--tree", pub)
	if rcGreen != 0 || errGreen != "" {
		t.Fatalf("正控失败：好树应 rc=0/stderr 空，得 rc=%d stderr=%q", rcGreen, errGreen)
	}
	for _, want := range []string{
		"本地断链 · 主仓：0 件", "本地断链 · 第二棵树：0 件",
		"孤儿 · 导出树：0 件（入链 = 0）", "入链 = 0 · 第二棵树整棵：0 件",
		"⇒ rc=0（命中不改码 ✗）",
	} {
		if !strings.Contains(outGreen, want) {
			t.Fatalf("正控失败：缺 %q\nout=%s", want, outGreen)
		}
	}

	// ── 负控①（有红明细）：主仓一条真断链 ⇒ 主仓 1 件；★ 「只报不拦」⇒ rc 仍 0 ─────────
	repoB, docB, pubB := cdoFixture(t, "broken")
	rcB, outB, _ := cdoRun(t, "--repo-root", repoB, "--docs-root", docB, "--tree", pubB)
	if rcB != 0 {
		t.Fatalf("负控①：本门『只报不拦』命中不改码 ⇒ 应 rc=0，得 rc=%d", rcB)
	}
	if !strings.Contains(outB, "本地断链 · 主仓：1 件") || !strings.Contains(outB, "[nope.md] ⇒ nope.md（指不到）") {
		t.Fatalf("负控①失败：应点名主仓 1 件真断链，得 out=%s", outB)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 正文必须不同（同 rc 下靠正文分辨）
	if outB == outGreen {
		t.Fatalf("成对断言失败：负控①正文与正控逐字相同 ⇒ 判据无鉴别力")
	}

	// ── 负控②（孤儿格有牙）：导出树里一件没人链 ⇒ 孤儿 1 件 ──────────────────
	repoO, docO, pubO := cdoFixture(t, "orphan")
	rcO, outO, _ := cdoRun(t, "--repo-root", repoO, "--docs-root", docO, "--tree", pubO)
	if rcO != 0 || !strings.Contains(outO, "孤儿 · 导出树：1 件（入链 = 0）") ||
		!strings.Contains(outO, "      loose.md（入链 0）") {
		t.Fatalf("负控②失败：孤儿格应数 1 件并点名，得 rc=%d out=%s", rcO, outO)
	}

	// ── 退 2 边界（★ 缺件 ≠ 绿 · 逐字沿用 py）：两棵外部树**都取不到** ⇒ rc=2 ────────────
	missDocs := filepath.Join(t.TempDir(), "no-docs")
	missTree := filepath.Join(t.TempDir(), "no-tree")
	rcBoth, outBoth, _ := cdoRun(t, "--repo-root", repo, "--docs-root", missDocs, "--tree", missTree)
	if rcBoth != 2 || !strings.Contains(outBoth, "第二棵树是本门 ①③ 两格的域") {
		t.Fatalf("退2边界失败：都取不到应 rc=2 且点名第二棵树域 ⇒ rc=%d out=%s", rcBoth, outBoth)
	}
	if rcBoth == rcGreen {
		t.Fatalf("成对断言失败：都取不到 rc=%d 与正控 rc=%d 同码", rcBoth, rcGreen)
	}

	// ── 退 2 边界（成对 · 缺件 ≠ 绿）：只剩**一棵**外部树在场（导出树缺）⇒ 仍 rc=2 ──────────
	rcOne, outOne, _ := cdoRun(t, "--repo-root", repo, "--docs-root", docs, "--tree", missTree)
	if rcOne != 2 || !strings.Contains(outOne, "结论：**不给结论（rc=2）** —— 导出树读不到") {
		t.Fatalf("退2边界（单棵）失败：docs 在场、导出树缺 ⇒ 缺件 ≠ 绿应 rc=2，得 rc=%d out=%s", rcOne, outOne)
	}
	if !strings.Contains(outOne, "孤儿 · 导出树：判不了") || !strings.Contains(outOne, "★读不到") {
		t.Fatalf("退2边界（单棵）失败：应照实打印导出树判不了，得 out=%s", outOne)
	}
	if rcOne == rcGreen {
		t.Fatalf("成对断言失败：单棵缺 rc=%d 与正控 rc=%d 同码 ⇒ 缺件 ≠ 绿无牙", rcOne, rcGreen)
	}

	// ── 空树（★ GAP-20261005-69 的牙）：第二棵树在盘但**真扫面 0 件 .md** ⇒ 判不了 ⇒ rc=2 ────
	rcEmpty, outEmpty, _ := cdoRun(t, "--repo-root", repo, "--docs-root", t.TempDir(), "--tree", pub)
	if rcEmpty != 2 || !strings.Contains(outEmpty, "本地断链 · 第二棵树：判不了（扫面 0 件 .md）") {
		t.Fatalf("空树失败：第二棵树 0 件 .md ⇒ 判不了应 rc=2，得 rc=%d out=%s", rcEmpty, outEmpty)
	}
	if rcEmpty == rcGreen {
		t.Fatalf("成对断言失败：空树 rc=%d 与正控 rc=%d 同码 ⇒ 空扫描无牙", rcEmpty, rcGreen)
	}

	// ── 不给结论：主仓不在盘 ⇒ rc=2（py 逐字对齐档）─────────────────────────
	rcNoRepo, outNoRepo, _ := cdoRun(t, "--repo-root", filepath.Join(t.TempDir(), "nope"),
		"--docs-root", docs, "--tree", pub)
	if rcNoRepo != 2 || !strings.Contains(outNoRepo, "仓根不在盘") {
		t.Fatalf("主仓缺失败：应 rc=2，得 rc=%d out=%s", rcNoRepo, outNoRepo)
	}

	// ── 不给结论（登记体检的牙）：登记件在域内却 0 原位错 ⇒ 登记过期 ⇒ rc=2 ──────────
	repoS, docS, pubS := cdoFixture(t, "stale")
	rcS, outS, _ := cdoRun(t, "--repo-root", repoS, "--docs-root", docS, "--tree", pubS)
	if rcS != 2 || !strings.Contains(outS, "基准偏移登记过期") || !strings.Contains(outS, "请撤登记") {
		t.Fatalf("登记过期失败：应 rc=2 且点名，得 rc=%d out=%s", rcS, outS)
	}

	// ── 机器面：合法 ⇒ 一行 JSON（与 py json.dumps(ensure_ascii=False) 同形）──────────
	rcJ, outJ, _ := cdoRun(t, "--repo-root", repo, "--docs-root", docs, "--tree", pub, "--json")
	if rcJ != 0 || !strings.Contains(outJ, `"schema": "zerg-gate/v1"`) ||
		!strings.Contains(outJ, `"gate": "check-docs-orphans-links"`) ||
		!strings.Contains(outJ, `"mode": "report-only"`) {
		t.Fatalf("机器面失败：得 rc=%d out=%s", rcJ, outJ)
	}
	// 空集合三形态：导出树在场且无孤儿 ⇒ orphans.count=0 且 count 键在（★ 不是缺键）。
	if !strings.Contains(outJ, `"orphans": {"count": 0, "domain": "`+pub+`"}`) {
		t.Fatalf("机器面失败：orphans 格形态不符 out=%s", outJ)
	}
	// 导出树缺 ⇒ 不给结论（rc=2）⇒ **不打印**机器面行（逐字沿用 py：`json` 只在 rc=0 出口）。
	rcJN, outJN, _ := cdoRun(t, "--repo-root", repo, "--docs-root", docs, "--tree", missTree, "--json")
	if rcJN != 2 || strings.Contains(outJN, `"schema": "zerg-gate/v1"`) {
		t.Fatalf("机器面（缺导出树）失败：应 rc=2 且无机器面行，得 rc=%d out=%s", rcJN, outJN)
	}

	// ── 判据清单 / 自检 / 帮助 / 用法错 ─────────────────────────────────────
	rcLR, outLR, _ := cdoRun(t, "--list-rules")
	if rcLR != 0 || !strings.Contains(outLR, "判据（两格 · 只报不拦）") {
		t.Fatalf("--list-rules 失败：得 rc=%d", rcLR)
	}
	rcST, outST, _ := cdoRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检 30 格 · 失败 0 格") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%s", rcST, outST)
	}
	rcH, outH, _ := cdoRun(t, "-h")
	if rcH != 0 || !strings.Contains(outH, "usage: check-docs-orphans-links.py") {
		t.Fatalf("-h 失败：得 rc=%d", rcH)
	}
	rcU, outU, errU := cdoRun(t, "--zzz")
	if rcU != 2 || outU != "" || !strings.Contains(errU, "unrecognized arguments: --zzz") {
		t.Fatalf("未知旗标失败：得 rc=%d stdout=%q stderr=%q", rcU, outU, errU)
	}
}

// ── A 类（tempdir 泄漏）回归牙：--self-test 跑一趟后，TMPDIR 下一级目录里前缀
// coan-selftest- 的条数**增量必须为 0**。主件 coanSelfTest 的自清句
// `defer os.RemoveAll(tmp)`（check_docs_orphans_links.go:872）被删掉时本用例必红。
func cdoCountSelftestTmpDirs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatalf("读 TMPDIR（%s）失败：%v", os.TempDir(), err)
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() && strings.HasPrefix(e.Name(), "coan-selftest-") {
			n++
		}
	}
	return n
}

func TestCheckDocsOrphansLinksSelfTestNoTempLeak(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir()) // GAP-20261003-16：私有计数面（只数本用例自建 tmp 目录）
	before := cdoCountSelftestTmpDirs(t)
	rc, out, errb := cdoRun(t, "--self-test")
	if rc != 0 || !strings.Contains(out, "自检 30 格 · 失败 0 格") {
		t.Fatalf("自检未过：rc=%d stderr=%q out=%s", rc, errb, out)
	}
	after := cdoCountSelftestTmpDirs(t)
	if after != before {
		t.Fatalf("A 类 tempdir 泄漏：TMPDIR（%s）下 coan-selftest-* 一级目录 before=%d after=%d（delta=%d ≠ 0）⇒ 主件 coanSelfTest 少了自清句 defer os.RemoveAll(tmp)（check_docs_orphans_links.go:872）",
			os.TempDir(), before, after, after-before)
	}
}

// ── B 类（读失败静默）回归牙：树里一件 .md 锁成 chmod 000 ⇒
// 主件 coanScanTree 的 os.ReadFile 失败登记面（coanReadFails）必响：
// rc=2 + stderr BLOCKED 头 + 逐件点名那份锁死件。登记/判红若删回静默，本用例必红。
func TestCheckDocsOrphansLinksReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 绕过权限位 ⇒ chmod 000 不挡读，本用例无鉴别力")
	}
	repo, docs, pub := cdoFixture(t, "")
	locked := filepath.Join(repo, "locked.md")
	cdoWrite(t, locked, "# locked\n")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o644)
	rc, _, errb := cdoRun(t, "--repo-root", repo, "--docs-root", docs, "--tree", pub)
	if rc != 2 {
		t.Fatalf("读失败回归牙：锁死件在树 ⇒ 应 rc=2（登记 + 判红），得 rc=%d stderr=%q", rc, errb)
	}
	if !strings.Contains(errb, "BLOCKED") {
		t.Fatalf("读失败回归牙：stderr 缺 BLOCKED 头，得 stderr=%q", errb)
	}
	if !strings.Contains(errb, "locked.md") {
		t.Fatalf("读失败回归牙：未逐件点名锁死件 locked.md，得 stderr=%q", errb)
	}
}

// ── GAP-20261001-26 回归牙（本单同步引入的新行为 · 照 py 候件 P3B 同步）：树里一件 .md 读不了 ⇒
// ①该格判不了（rc=2）**但**该树已收的红明细照印（不因一格读不了而丢整树）；
// ②成对反控：同一夹具去掉锁死件 ⇒ 非红、按正常「N 件」打印（✗ 禁恒红）。
// 哑化主件 coanLocalBrokenDetail 的「unreadable 逐条记下 + 判不了态返 hits」⇒ 本用例必红。
func TestCheckDocsOrphansLinksUnreadableKeepsEvidence(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 绕过权限位 ⇒ chmod 000 不挡读，本用例无鉴别力")
	}
	// ① 一棵树里既有真断链（x.md）、又有一件读不了（locked.md）
	repo, docs, pub := cdoFixture(t, "broken")
	locked := filepath.Join(repo, "locked.md")
	cdoWrite(t, locked, "# locked\n")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o644)
	rc, out, _ := cdoRun(t, "--repo-root", repo, "--docs-root", docs, "--tree", pub)
	if rc != 2 {
		t.Fatalf("读不了回归牙：锁死件在树 ⇒ 应 rc=2（判不了），得 rc=%d", rc)
	}
	if !strings.Contains(out, "本地断链 · 主仓：判不了（件读不了 1 件：locked.md）") {
		t.Fatalf("读不了回归牙：判不了文案未带「件读不了 N 件」，得 out=%s", out)
	}
	if !strings.Contains(out, "x.md:3  [nope.md] ⇒ nope.md（指不到）") {
		t.Fatalf("读不了回归牙：判不了态丢了该树已收的红明细（x.md 断链未印），得 out=%s", out)
	}

	// ② 成对反控：同一夹具去掉锁死件 ⇒ 非红、按「N 件」打印（不得恒红）
	repo2, docs2, pub2 := cdoFixture(t, "broken")
	rc2, out2, _ := cdoRun(t, "--repo-root", repo2, "--docs-root", docs2, "--tree", pub2)
	if rc2 != 0 || !strings.Contains(out2, "本地断链 · 主仓：1 件") {
		t.Fatalf("成对反控失败：无锁死件应 rc=0 且「主仓：1 件」，得 rc=%d out=%s", rc2, out2)
	}
	if strings.Contains(out2, "件读不了") {
		t.Fatalf("成对反控失败：无锁死件却出现「件读不了」文案，得 out=%s", out2)
	}
}
