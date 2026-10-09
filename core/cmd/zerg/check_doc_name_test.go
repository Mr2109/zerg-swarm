// check_doc_name_test.go —— Go 真搬的**成对负控定向单测 + 内化金标**（M2 · check_doc_name.go）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态；旧 py 对拍退场后，
// 逐格金标改由 Go 半现役行为现算现冻（见下「内化金标」段）。
//
// 用例名：TestCheckDocNameGatePort_Directed（定向）·
// TestCheckDocNameGatePort_GoldenInternalized（内化金标 · A 态）·
// TestCheckDocNameGatePort_GoldenTwoStates（两态逐字同）·
// TestCheckDocNameGatePort_SelfTestGolden（--self-test 逐字金标）·
// TestCheckDocName_MetaSelfCheckRefuses（元自检回归牙）。
//
// ── 内化缘起（甲类·两件两处内化 · 照 atomic_write_test.go / pack_release_test.go）──
//
// 本件原「py 真身 ⟷ Go 真搬」逐字对拍（旧名 TestCheckDocNameGatePort_Parity /
// TestCheckDocNameGatePort_SelfTestSameRC）的**对拍真身定位**靠 `scripts/gates/check-doc-name.py`
// 在盘（`cdnRepoRoot` 找不到即 `t.Skipf`）；该 py 已退场（`cae5fb90` M2 退役批）⇒ Skip 条件恒真、
// 对拍归零（假绿、零保护）。⇒ 把金标真源**收回 Go 半现役行为本身**：
//
//	① 哨兵改锚 **Go 半** `core/cmd/zerg/check_doc_name.go`（✗ 不锚已退场 py —— 一退役即失锚）；
//	   找不到 ⇒ `cdnAnchorGoHalf` `t.Fatalf` **fail-closed**（✗ 不留 Skip）。
//	② ★ 金标计算式（✗ 不臆造 · 逐格由「Go 半现役行为现算」得到）：逐格「夹具 + argv ⇒
//	   (rc, stdout, stderr)」由 `cdnRunGate` 现跑捕获（一次性现算、只读夹具、✗ 不落仓），
//	   归一后冻成下表 `cdnGoldenTable`；**本机时点/位置类量**在比对前归一化（✗ 不冻结 ——
//	   因其本非常量）：夹具根 `t.TempDir()`（随机路径 + macOS /var⟷/private/var）⇒ `<FX>`；
//	   空夹具根 ⇒ `<FXE>`；`--self-test` 的临时目录 `$TMPDIR/check-doc-name-<digits>` ⇒
//	   `<TMP>check-doc-name-<RND>`。
//	② ′ 四格旧「真仓·*」改为**确定性合成夹具**（`--scope repo` 打合成夹具树）：真仓先读的
//	   `扫描域：N 个文件/目录` 随仓树漂 ⇒ 冻它会「每加一件即红」；打夹具则**跨机跨时刻恒定**
//	   （flag / code-path 覆盖面一字不缩：--scope repo / --json / --max-examples / --summary）。
//	② ″ ★ 外部真源（《清单-命名规范化》）：A 档 15 条 / C 档 9 条等**已在 Go 半冻成常量**
//	   （`cdnExemptA` 等），运行时**不读** `Zerg-内部文档/01-设计/清单-命名规范化.{md,tsv}`
//	   ⇒ 金标取自 Go 半现役行为，**未依赖**该外部真源（该件现读在盘；取到与否不影响本件结论）。
//	③ 两态（✗ 不许 Skip）：A 态 = 夹具无 py（真仓现态）；B 态 = 合成「旧态」夹具（把退役 py
//	   路径 `scripts/gates/check-doc-name.py` 放回夹具 `scripts/gates/`）⇒ 两态**对同一套金标
//	   逐字同命中**，证 py 在/不在盘都不改判据结论（内化彻底）。
//	④ ★ 哑化负控（"牙" · 见交付读数）：`-overlay` 把 `check_doc_name.go` 的退码判据一臂
//	   （`cdnDecide` 的 `rep.live["FAIL"] > 0` → 恒假）哑化 ⇒ `..._GoldenInternalized`
//	   **必 FAIL 且带行号**（✗ 不真改仓内源 · 免 `dev edit` 前的仓内污染）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// cdnGoHalfRel / cdnRetiredPyRel —— 哨兵锚（Go 半）与已退场对拍真身（py）的仓根相对路径。
const (
	cdnGoHalfRel    = "core/cmd/zerg/check_doc_name.go"
	cdnRetiredPyRel = "scripts/gates/check-doc-name.py"
)

// cdnAnchorGoHalf —— 哨兵：锚 **Go 半**在盘（✗ 不锚已退场 py）；找不到 ⇒ fail-closed（缺件 ≠ 绿）。
// 旧 `cdnRepoRoot` 找 py 原件不到就 `t.Skipf`（恒 Skip · 零保护）—— 本改把它换成硬前置。
func cdnAnchorGoHalf(t *testing.T, root string) {
	t.Helper()
	fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(cdnGoHalfRel)))
	if err != nil || fi.IsDir() {
		t.Fatalf("哨兵失守：上溯未见 Go 半 %s（root=%s err=%v）⇒ fail-closed（✗ 不留 Skip）",
			cdnGoHalfRel, root, err)
	}
}

// cdnRepoRoot —— 仓根（`core/cmd/zerg` 上溯三级）；哨兵锚 Go 半 ⇒ fail-closed（✗ 不 Skip）。
func cdnRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	cdnAnchorGoHalf(t, root)
	return root
}

func cdnIsRegular(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func cdnWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cdnMakeGoodTree —— 造一棵好件树（全绿，含允许对 edit-assert）。
func cdnMakeGoodTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "good")
	files := map[string]string{
		"docs/01-设计/设计-文档体系-v1.0.md":  "# x\n",
		"docs/zh/模块-主控.md":            "# x\n",
		"README.md":                   "# x\n",
		"scripts/docs/edit-assert":    "#!/usr/bin/env python3\n",
		"scripts/docs/edit-assert.md": "# 说明书\n",
	}
	for rel, txt := range files {
		cdnWrite(t, filepath.Join(root, rel), txt)
	}
	return root
}

// cdnMakeBadTree —— 造一棵坏件树（含 N6 同基名对 + N1/N2/N3/N4 命中）。
func cdnMakeBadTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "bad")
	bad := []string{
		"docs/INDEX.md",
		"docs/研究 - 自我纠错.md",
		"docs/01-设计/设计-文档体系-20260918.md",
		"docs/01-设计/设计-v2.5.10-卵功能与对话机制.md",
		"docs/01-设计/设计-文档体系.md",
		"docs/01-设计/设计-文档体系.py",
	}
	for _, rel := range bad {
		cdnWrite(t, filepath.Join(root, rel), "# x\n")
	}
	cdnWrite(t, filepath.Join(root, "docs/混用目录/01-a.md"), "# x\n")
	cdnWrite(t, filepath.Join(root, "docs/混用目录/02-b.md"), "# x\n")
	cdnWrite(t, filepath.Join(root, "docs/混用目录/c.md"), "# x\n")
	cdnWrite(t, filepath.Join(root, "docs/混用目录/d.md"), "# x\n")
	return root
}

// cdnMakeN7Tree —— N7 形态分类夹具（词 / 单字母 / 版本收窄）。
func cdnMakeN7Tree(t *testing.T, fx string) string {
	t.Helper()
	n7 := filepath.Join(fx, "n7")
	cdnWrite(t, filepath.Join(n7, "docs", "模块Agent说明.md"), "# x\n") // (file, CJK→LAT, word)
	cdnWrite(t, filepath.Join(n7, "docs", "Agent模块.md"), "# x\n")   // (file, LAT→CJK, word)
	cdnWrite(t, filepath.Join(n7, "docs", "模块A说明.md"), "# x\n")     // (file, CJK→LAT, single)
	cdnWrite(t, filepath.Join(n7, "docs", "设计-模块v2.1.md"), "# x\n") // version ⇒ 收窄，无 N7
	return n7
}

func cdnRunGate(argv []string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkDocNameGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckDocNameGatePort_Directed(t *testing.T) {
	good := cdnMakeGoodTree(t)
	bad := cdnMakeBadTree(t)

	if rc, out, _ := cdnRunGate([]string{"--repo-root", good, "--target", good, "--no-self-test"}); rc != 0 || !strings.Contains(out, "OK") || !strings.Contains(out, "A06") {
		t.Fatalf("好件必绿失败：rc=%d\n%s", rc, out)
	}
	if rc, out, _ := cdnRunGate([]string{"--repo-root", bad, "--target", bad, "--no-self-test"}); rc != 1 || !strings.Contains(out, "N6") {
		t.Fatalf("坏件必红失败：rc=%d\n%s", rc, out)
	}
	if rc, out, _ := cdnRunGate([]string{"--repo-root", bad, "--target", filepath.Join(bad, "docs/INDEX.md"), "--no-self-test"}); rc != 1 || !strings.Contains(out, "N1") {
		t.Fatalf("N1 坏件应红：rc=%d\n%s", rc, out)
	}
	if rc, out, _ := cdnRunGate([]string{"--repo-root", bad, "--target", filepath.Join(bad, "no-such"), "--no-self-test"}); rc != 2 || !strings.Contains(out, "不给结论") {
		t.Fatalf("缺件应 rc=2：rc=%d\n%s", rc, out)
	}
	if rc, out, _ := cdnRunGate([]string{"--list-exempt"}); rc != 0 || !strings.Contains(out, "A 档 · 《清单-命名规范化》已登记的「不改」条目：15 条（期望 15）") {
		t.Fatalf("--list-exempt 失败：rc=%d\n%s", rc, out)
	}
	if rc, out, _ := cdnRunGate([]string{"--list-rules"}); rc != 0 || !strings.Contains(out, "N1") {
		t.Fatalf("--list-rules 失败：rc=%d\n%s", rc, out)
	}
}

// ── 内化金标：旧 py 对拍的逐格金标收成 Go 常量 · 现算自证（✗ 不 Skip）──────────────────────

type cdnGoldenCase struct {
	name    string
	wantRC  int
	wantOut string // stdout 归一化后（现算自证 · ✗ 手写）
	wantErr string
}

// cdnGoldenTable —— 金标真源（19 格 · 逐格由 `cdnRunGate` 现跑捕获 · 见件头金标计算式）。
var cdnGoldenTable = []cdnGoldenCase{
	{name: "夹具仓·scope repo", wantRC: 0, wantOut: "check-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）\n扫描域：10 个文件/目录（排除名单同《清单》§0）\n  不合规 0（冻结区 0）· 告警 0（冻结区 0）\n  ★ 仓内冻结区：无 —— `docs/项目文档/` 已于 2026-09-26「批3」分家至仓外 `Zerg-内部文档/项目文档/`（仓内零命中 ⇒ 条目已删，处置同 2026-09-19 的 `docs/issues/`）⇒ 冻结计数恒 0/0（`--count-frozen` 机制保留）\n  N1 [FAIL] ①大小写：文件词干全为大写拉丁字母（≥3） —— 命中 1（可改面 0 · 冻结区 0）\n  N6 [FAIL] ⑥同目录同基名、扩展名不同（去最后一个扩展名后相同） —— 命中 1（可改面 0 · 冻结区 0）\n  豁免命中（逐条可查 · --list-exempt）：A02×1 · A06×1\nOK：无不合规命中（告警 0 · 已豁免 2）\n", wantErr: ""},
	{name: "夹具仓·json", wantRC: 0, wantOut: "{\n  \"rc\": 0,\n  \"verdict\": \"OK：无不合规命中（告警 0 · 已豁免 2）\",\n  \"entries\": 10,\n  \"live\": {\n    \"FAIL\": 0,\n    \"WARN\": 0\n  },\n  \"frozen\": {\n    \"FAIL\": 0,\n    \"WARN\": 0\n  },\n  \"exempt_hits\": {\n    \"A02\": 1,\n    \"A06\": 1\n  },\n  \"per_rule\": {\n    \"N1\": 1,\n    \"N6\": 1\n  },\n  \"per_rule_live\": {},\n  \"per_rule_frozen\": {}\n}\n", wantErr: ""},
	{name: "夹具仓·max-examples 1", wantRC: 1, wantOut: "check-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）\n扫描域：13 个文件/目录（排除名单同《清单》§0）\n  不合规 5（冻结区 0）· 告警 1（冻结区 0）\n  ★ 仓内冻结区：无 —— `docs/项目文档/` 已于 2026-09-26「批3」分家至仓外 `Zerg-内部文档/项目文档/`（仓内零命中 ⇒ 条目已删，处置同 2026-09-19 的 `docs/issues/`）⇒ 冻结计数恒 0/0（`--count-frozen` 机制保留）\n  N1 [FAIL] ①大小写：文件词干全为大写拉丁字母（≥3） —— 命中 1（可改面 1 · 冻结区 0）\n      · docs/INDEX.md → 词干 'INDEX' 全为大写拉丁字母\n  N2 [FAIL] ②空格/全角：名称含 U+0020 / U+3000 / 全角标点 —— 命中 1（可改面 1 · 冻结区 0）\n      · docs/研究 - 自我纠错.md → 名称含 U+0020\n  N3 [FAIL] ③日期后缀：词干含 -YYYYMMDD 或 -YYYYMM —— 命中 1（可改面 1 · 冻结区 0）\n      · docs/01-设计/设计-文档体系-20260918.md → 词干含日期后缀 '-20260918' ⇒ 日期应移入 frontmatter `updated_at`\n  N4 [FAIL] ③版本段不在尾位：-vX.Y 其后还有分隔段 —— 命中 1（可改面 1 · 冻结区 0）\n      · docs/01-设计/设计-v2.5.10-卵功能与对话机制.md → 版本段 '-v2.5.10-' 不在尾位 ⇒ 应移到尾部（`名-…-vX.Y[-YYYYMMDD]`）\n  N5 [WARN] ④前缀序号不一致：同层 NN- 与无前缀并存 —— 命中 1（可改面 1 · 冻结区 0）\n      · docs/混用目录 → 同层混用 NN- 前缀（前缀 2 · 无前缀 2）\n  N6 [FAIL] ⑥同目录同基名、扩展名不同（去最后一个扩展名后相同） —— 命中 1（可改面 1 · 冻结区 0）\n      · docs/01-设计/设计-文档体系 → 同基名不同扩展 .md/.py\n  豁免命中（逐条可查 · --list-exempt）：无\nFAIL：5 个不合规命中（只报告不改）\n", wantErr: ""},
	{name: "list-rules", wantRC: 0, wantOut: "check-doc-name.py 规则表（来源 = 《清单-命名规范化》§1 七类 + §2 风险档 + §5 处置）\n  N1  [FAIL] ①大小写：文件词干全为大写拉丁字母（≥3）\n       出处：《清单》§1 ①（与 mkdocs `index.md` 约定冲突；本卷 APFS 大小写不敏感）\n  N2  [FAIL] ②空格/全角：名称含 U+0020 / U+3000 / 全角标点\n       出处：《清单》§1 ②\n  N3  [FAIL] ③日期后缀：词干含 -YYYYMMDD 或 -YYYYMM\n       出处：《清单》§5 ③（日期移入 frontmatter updated_at）+ §5 ⑦（6 位日期）\n  N4  [FAIL] ③版本段不在尾位：-vX.Y 其后还有分隔段\n       出处：《清单》§5 ③（`名-…-vX.Y[-YYYYMMDD]`，版本段移尾）\n  N5  [WARN] ④前缀序号不一致：同层 NN- 与无前缀并存\n       出处：《清单》§5 ④（结构级 · 只登记不改 · 牵动 nav/白名单）\n  N6  [FAIL] ⑥同目录同基名、扩展名不同（去最后一个扩展名后相同）\n       出处：《清单》§1 ⑥（K8 那类）+ §4.3 允许对\n  N7  [WARN] ⑤中英紧邻：同一分隔段内汉字与拉丁字母直接相邻\n       出处：《清单》§5 ⑤（机械插 `-` 需人审）\n  档位口径：**文件名**命中才计入退码；**目录名**命中一律 WARN（结构级 · 只登记不改）；\n            N5/N7 为 WARN；仓内冻结区**已无**（`docs/项目文档/` 2026-09-26 批3 分家至仓外 `Zerg-内部文档/项目文档/` ⇒ FROZEN_PREFIX 已删；原第二项 `docs/issues/` 2026-09-19 同法已删）。\n  退码：0 全绿 · 1 有不合规项 · 2 不给结论（优先级 2>1>0）。\n", wantErr: ""},
	{name: "list-exempt", wantRC: 0, wantOut: "例外表（四档 · 逐条给理由与出处）\nA 档 · 《清单-命名规范化》已登记的「不改」条目：15 条（期望 15）\n  A01 [N5] 类型 ④ · TSV 第 2 行 · 形态 prefix\n      对象：docs/常青/ , docs/skills/\n      理由：两个正式面候选目录**无 NN- 前缀**，而 01-设计/02-调研/thunderbolt 有 ⇒ 结构级（2026-09-19 分家：原列四项里的 `docs/调研/` `docs/03-评审/` 已迁出仓 ⇒ 从 paths 删除，只留仓内两项）\n      出处：清单 §5 ④ · TSV 第 2 行（建议 = 不改 · 结构级，仅登记）\n  A02 [N1] 类型 ① · TSV 第 3 行 · 形态 path\n      对象：README.md\n      理由：行业约定文件；`publish/whitelist.txt` 公开面入口\n      出处：清单 §4.2 · TSV 第 3 行（建议 = 不改）\n  A03 [N6] 类型 ⑥ · TSV 第 5 行 · 形态 pair\n      对象：core/internal/compat/compat\n      理由：Go 包同名约定（compat.go + compat.json）\n      出处：清单 §4.3 · TSV 第 5 行（建议 = 不改 · 登记为允许对）\n  A04 [N6] 类型 ⑥ · TSV 第 6 行 · 形态 pair\n      对象：core/internal/compat/compat\n      理由：同上（JSON 侧）\n      出处：清单 §4.3 · TSV 第 6 行（同上）\n  A05 [N1] 类型 ① · TSV 第 8 行 · 形态 path\n      对象：AGENTS.md\n      理由：行业约定文件（agent 入口）；`publish/private-paths.txt:29` 明列\n      出处：清单 §4.2 · TSV 第 8 行（建议 = 不改）\n  A06 [N6] 类型 ⑥ · TSV 第 15 行 · 形态 pair\n      对象：scripts/docs/edit-assert\n      理由：无扩展可执行件；改名 = 断全部 `python3 scripts/docs/edit-assert` 调用方\n      出处：清单 §4.3 · TSV 第 15 行（建议 = 不改 · 登记为允许对）\n  A07 [N6] 类型 ⑥ · TSV 第 16 行 · 形态 pair\n      对象：scripts/docs/edit-assert\n      理由：`.md` = 同一脚本的说明书\n      出处：清单 §4.3 · TSV 第 16 行（同上）\n  A08 [N6] 类型 ⑥ · TSV 第 21 行 · 形态 pair\n      对象：tools/doc_index_gen\n      理由：`tools/*.md`（145 个）为工具说明书，是既有约定\n      出处：清单 §4.3 · TSV 第 21 行（同上）\n  A09 [N6] 类型 ⑥ · TSV 第 22 行 · 形态 pair\n      对象：tools/doc_index_gen\n      理由：同上（实现侧）\n      出处：清单 §4.3 · TSV 第 22 行（同上）\n  A10 [N6] 类型 ⑥ · TSV 第 23 行 · 形态 pair\n      对象：tools/kb_docs_sync\n      理由：同上（工具说明书）\n      出处：清单 §4.3 · TSV 第 23 行（同上）\n  A11 [N6] 类型 ⑥ · TSV 第 24 行 · 形态 pair\n      对象：tools/kb_docs_sync\n      理由：同上（实现侧）\n      出处：清单 §4.3 · TSV 第 24 行（同上）\n  A12 [N6] 类型 ⑥ · TSV 第 28 行 · 形态 pair\n      对象：scripts/gates/mutate-scan\n      理由：无扩展可执行件（同 edit-assert）\n      出处：清单 §4.3 · TSV 第 28 行（同上）\n  A13 [N6] 类型 ⑥ · TSV 第 29 行 · 形态 pair\n      对象：scripts/gates/mutate-scan\n      理由：`.md` = 同一脚本的说明书\n      出处：清单 §4.3 · TSV 第 29 行（同上）\n  A20 [N3+N4] 类型 ③ · TSV 第 143 行 · 形态 path\n      对象：docs/常青/白皮书-虫族经济模型-v0.1.md\n      理由：版本号已在尾位，与本仓 `名-…-vX.Y[-YYYYMMDD]` 约定一致\n      出处：清单 §5 · TSV 第 143 行（建议 = 不改 · 版本号已在尾位）\n  A22 [N1] 类型 ① · TSV 第 147 行 · 形态 name\n      对象：LICENSE , NOTICE , SECURITY.md , CONTRIBUTING.md , THIRD_PARTY_LICENSES.md\n      理由：公开面法定/治理文件，全大写是行业约定\n      出处：清单 §5 · TSV 第 147 行（建议 = 不改 · 符合行业约定）\nB 档 · 规则级规范豁免：3 条（期望 3）\n  B01 [N1] 形态 reserved\n      对象：README.md , AGENTS.md , SKILL.md , VERSION , CHANGELOG.md\n      理由：行业/工具约定保留名 —— **只在文档面之外生效**（docs/ 内的同名件照判）。2026-09-21 发布闸 + `CHANGELOG.md`（Keep a Changelog 1.1.0 约定名 · P9 拍板「`CHANGELOG.md` 给机检」）\n  B02 [N6] 形态 pair_ext\n      对象：go/.mod/.sum , Cargo/.toml/.lock\n      理由：构建工具链约定（Go module / Cargo），不属文档命名面。**休眠申报**：N6 的口径（至少一方是文档/脚本面扩展）已把它们挡在门外 ⇒ 本仓当前 0 命中\n  B03 [N6] 形态 pair_doc_machine\n      对象：.md/.markdown/.mdx/.tsv/.csv/.json\n      理由：人读件 + 机读件同基名（本仓既有形态：`清单-命名规范化.md/.tsv` = 设计件 + 机读件）。**我方判断 · 与本脚本自身的差异已登记**：去掉这一条，该对会被判 ⑥（见 `scripts/gates/check-doc-name.md` §差异）\nC 档 · 新增登记（C01–C03 待批准 · C04 已拍 2026-09-18 · C05–C07 第二波任务书登记 2026-09-18 · C08 2026-09-22（门⑮ 主仓悬空引用）登记 · C09 2026-09-24（波11 序125 `W-43` hook git 环境）登记）：9 条（期望 9）\n  C01 [N6] 形态 pair\n      对象：scripts/gates/check-doc-meta\n      理由：本轮 W2 交付：门脚本 + 说明书（同 `scripts/docs/edit-assert` 既有约定）· **待父代理批准**\n  C02 [N6] 形态 pair\n      对象：scripts/gates/check-doc-name\n      理由：同上（本文档脚本 + 说明书）· **待父代理批准**\n  C03 [N6] 形态 pair\n      对象：scripts/gates/check-doc-freshness\n      理由：文档体系 P1 同批交付的第三条门脚本 + 说明书（`check-doc-freshness.{py,md}`）——与 C01/C02、A06/A07、A12/A13 同一形态 · **待父代理批准**\n  C04 [N6] 形态 pair\n      对象：scripts/gates/precommit-gates\n      理由：`scripts/gates/precommit-gates.md` 由**路 D**（门禁挂接 Q14）2026-09-18 20:43 新增，形态 = 「门脚本 + 说明书」（同 C01–C03 / A06/A07 / A12/A13）⇒ 登记为**允许对**。**已拍 2026-09-18（Mr2109）**：认下 C04（拍板来源 = 文档体系 P1 第三批 拍板单 §⑥）。撤法：删本行并把 `EXEMPT_C_EXPECTED` 4→3 ⇒ 该对判红照实回来\n  C05 [N6] 形态 pair\n      对象：scripts/gates/check-gate-coverage\n      理由：**门①（门覆盖自检）**的「门脚本 + 说明书」对（`check-gate-coverage.{py,md}`）——由第二波（2026-09-18）新建并挂进 `scripts/gates/precommit-gates.sh` 的 `gates` scope（模式 `tri`）。形态同 C01–C04 / A06/A07 / A12/A13 ⇒ 登记为**允许对**。撤法：删本行并把 `EXEMPT_C_EXPECTED` 7→6 ⇒ 该对判红照实回来\n  C06 [N6] 形态 pair\n      对象：scripts/gates/check-version-sources\n      理由：**门②（版本单一真源）**的「门脚本 + 说明书」对（`check-version-sources.{py,md}`）——同 C05（第二波新建 · 挂 `gates` scope · 模式 `tri`）。撤法：删本行并把 `EXEMPT_C_EXPECTED` 7→6 ⇒ 该对判红照实回来\n  C07 [N6] 形态 pair\n      对象：scripts/gates/check-wired-scripts\n      理由：**门③（脚本接线自检）**的「门脚本 + 说明书」对（`check-wired-scripts.{py,md}`）——同 C05（第二波新建 · 挂 `gates` scope · 模式 `tri-report` 只报告）。撤法：删本行并把 `EXEMPT_C_EXPECTED` 7→6 ⇒ 该对判红照实回来\n  C08 [N6] 形态 pair\n      对象：scripts/gates/check-dangling-refs\n      理由：**门⑮（主仓悬空引用 `docs/…` 指向不存在）**的「门脚本 + 说明书」对（`check-dangling-refs.{py,md}`）—— 同 C05（2026-09-22 新建 · 挂 `docs` scope · 模式 `tri-report` 只报告 · 同时进快速档）。撤法：删本行并把 `EXEMPT_C_EXPECTED` 8→7 ⇒ 该对判红照实回来\n  C09 [N6] 形态 pair\n      对象：scripts/gates/check-git-env-hygiene\n      理由：**hook / 门禁脚本对外仓·合成仓的 git 调用先清环境**（`W-43` / `R-07`）的「门脚本 + 说明书」对（`check-git-env-hygiene.{py,md}`）—— 同 C05（2026-09-24 波11 序125 新建 · **单跑档、本笔不挂门**（本波件面未含 `precommit-gates.sh` ⇒ 不在 `add_step` 里）· 默认报告档 `tri-report`）。撤法：删本行并把 `EXEMPT_C_EXPECTED` 9→8 ⇒ 该对判红照实回来\nD 档 · 本轮 A 路拍板登记（登记不改）：2 条（期望 2）\n  D01 [N1] 形态 prefix_name\n      对象：docs/ + 基名 README.md\n      理由：`docs/` 内的 `README.md` = **语言层入口件 / 重定向存根**（`docs/README.md` 是并入 `docs/index-nav.md` 后按零删除保留的旧路径；`docs/{zh,en,en/_drafts,site}/README.md` 是目录律入口）。README.md 属**行业约定保留名**（与 B01 同源）⇒ 只在文档面之内开这一个口子（AGENTS.md / SKILL.md / VERSION 在 docs/ 内照判，K8 要的那条没被放松）\n      出处：父代理 2026-09-18 拍板 · 本说明书 §差异 6/§例外表 D 档（(b) 路：加规则级豁免登记，不改文件名）\n  D02 [N3] 形态 path\n      对象：bin/_history/zerg-core.prev-20260912.bak\n      理由：备件残片：`bin/` 被 `.gitignore` 忽略、**未纳入版本控制** ⇒ `git mv` 不可用（本路硬规矩：禁 `mv`/`rm`）；日期是它**作为备件的身份**（哪个二进制的前身）\n      出处：父代理 2026-09-18 拍板 · 本说明书 §差异 7（处置形态同《清单》A19/A21「移出仓」= 父代理动作）\n", wantErr: ""},
	{name: "夹具好件必绿", wantRC: 0, wantOut: "check-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）\n扫描域：10 个文件/目录（排除名单同《清单》§0）\n  不合规 0（冻结区 0）· 告警 0（冻结区 0）\n  ★ 仓内冻结区：无 —— `docs/项目文档/` 已于 2026-09-26「批3」分家至仓外 `Zerg-内部文档/项目文档/`（仓内零命中 ⇒ 条目已删，处置同 2026-09-19 的 `docs/issues/`）⇒ 冻结计数恒 0/0（`--count-frozen` 机制保留）\n  N1 [FAIL] ①大小写：文件词干全为大写拉丁字母（≥3） —— 命中 1（可改面 0 · 冻结区 0）\n  N6 [FAIL] ⑥同目录同基名、扩展名不同（去最后一个扩展名后相同） —— 命中 1（可改面 0 · 冻结区 0）\n  豁免命中（逐条可查 · --list-exempt）：A02×1 · A06×1\nOK：无不合规命中（告警 0 · 已豁免 2）\n", wantErr: ""},
	{name: "夹具坏件必红", wantRC: 1, wantOut: "check-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）\n扫描域：14 个文件/目录（排除名单同《清单》§0）\n  不合规 5（冻结区 0）· 告警 1（冻结区 0）\n  ★ 仓内冻结区：无 —— `docs/项目文档/` 已于 2026-09-26「批3」分家至仓外 `Zerg-内部文档/项目文档/`（仓内零命中 ⇒ 条目已删，处置同 2026-09-19 的 `docs/issues/`）⇒ 冻结计数恒 0/0（`--count-frozen` 机制保留）\n  N1 [FAIL] ①大小写：文件词干全为大写拉丁字母（≥3） —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/INDEX.md → 词干 'INDEX' 全为大写拉丁字母\n  N2 [FAIL] ②空格/全角：名称含 U+0020 / U+3000 / 全角标点 —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/研究 - 自我纠错.md → 名称含 U+0020\n  N3 [FAIL] ③日期后缀：词干含 -YYYYMMDD 或 -YYYYMM —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/01-设计/设计-文档体系-20260918.md → 词干含日期后缀 '-20260918' ⇒ 日期应移入 frontmatter `updated_at`\n  N4 [FAIL] ③版本段不在尾位：-vX.Y 其后还有分隔段 —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/01-设计/设计-v2.5.10-卵功能与对话机制.md → 版本段 '-v2.5.10-' 不在尾位 ⇒ 应移到尾部（`名-…-vX.Y[-YYYYMMDD]`）\n  N5 [WARN] ④前缀序号不一致：同层 NN- 与无前缀并存 —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/混用目录 → 同层混用 NN- 前缀（前缀 2 · 无前缀 2）\n  N6 [FAIL] ⑥同目录同基名、扩展名不同（去最后一个扩展名后相同） —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/01-设计/设计-文档体系 → 同基名不同扩展 .md/.py\n  豁免命中（逐条可查 · --list-exempt）：无\nFAIL：5 个不合规命中（只报告不改）\n", wantErr: ""},
	{name: "夹具前缀混用告警", wantRC: 0, wantOut: "check-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）\n扫描域：5 个文件/目录（排除名单同《清单》§0）\n  不合规 0（冻结区 0）· 告警 1（冻结区 0）\n  ★ 仓内冻结区：无 —— `docs/项目文档/` 已于 2026-09-26「批3」分家至仓外 `Zerg-内部文档/项目文档/`（仓内零命中 ⇒ 条目已删，处置同 2026-09-19 的 `docs/issues/`）⇒ 冻结计数恒 0/0（`--count-frozen` 机制保留）\n  N5 [WARN] ④前缀序号不一致：同层 NN- 与无前缀并存 —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/混用目录 → 同层混用 NN- 前缀（前缀 2 · 无前缀 2）\n  豁免命中（逐条可查 · --list-exempt）：无\nOK：无不合规命中（告警 1 · 已豁免 0）\n", wantErr: ""},
	{name: "单件目标", wantRC: 1, wantOut: "check-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）\n扫描域：1 个文件/目录（排除名单同《清单》§0）\n  不合规 1（冻结区 0）· 告警 0（冻结区 0）\n  ★ 仓内冻结区：无 —— `docs/项目文档/` 已于 2026-09-26「批3」分家至仓外 `Zerg-内部文档/项目文档/`（仓内零命中 ⇒ 条目已删，处置同 2026-09-19 的 `docs/issues/`）⇒ 冻结计数恒 0/0（`--count-frozen` 机制保留）\n  N1 [FAIL] ①大小写：文件词干全为大写拉丁字母（≥3） —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/INDEX.md → 词干 'INDEX' 全为大写拉丁字母\n  豁免命中（逐条可查 · --list-exempt）：无\nFAIL：1 个不合规命中（只报告不改）\n", wantErr: ""},
	{name: "缺件目标", wantRC: 2, wantOut: "BLOCKED：目标不存在 ⇒ 不给结论：<FX>/no-such\n", wantErr: ""},
	{name: "scope docs 缺面", wantRC: 2, wantOut: "BLOCKED：文档面根不存在（缺件不许静默降级）⇒ 不给结论：<FXE>/docs\n  口径：公开树侧（无 docs/）本 scope 为 BLOCKED、不适用；私有树正常判（判据一字未改）。\n", wantErr: ""},
	{name: "devdocs 根不在", wantRC: 2, wantOut: "BLOCKED：版本真源读不到（core/internal/version/version.go 里没有 `const Version = \"…\"`）⇒ 现算不出默认语料根 ⇒ 不给结论\n  口径：读不到真源不当绿（设计稿 §五）。换根请设 ZERG_DEVDOCS_ROOT。\n", wantErr: ""},
	{name: "json 夹具坏件", wantRC: 1, wantOut: "{\n  \"rc\": 1,\n  \"verdict\": \"FAIL：5 个不合规命中（只报告不改）\",\n  \"entries\": 14,\n  \"live\": {\n    \"FAIL\": 5,\n    \"WARN\": 1\n  },\n  \"frozen\": {\n    \"FAIL\": 0,\n    \"WARN\": 0\n  },\n  \"exempt_hits\": {},\n  \"per_rule\": {\n    \"N1\": 1,\n    \"N2\": 1,\n    \"N3\": 1,\n    \"N4\": 1,\n    \"N5\": 1,\n    \"N6\": 1\n  },\n  \"per_rule_live\": {\n    \"N1\": 1,\n    \"N2\": 1,\n    \"N3\": 1,\n    \"N4\": 1,\n    \"N5\": 1,\n    \"N6\": 1\n  },\n  \"per_rule_frozen\": {}\n}\n", wantErr: ""},
	{name: "夹具好件 --list-exempt --hits", wantRC: 0, wantOut: "例外表（四档 · 逐条给理由与出处）\nA 档 · 《清单-命名规范化》已登记的「不改」条目：15 条（期望 15）\n  A01 [N5] 类型 ④ · TSV 第 2 行 · 形态 prefix · 命中 ×0（不可达/未命中）\n      对象：docs/常青/ , docs/skills/\n      理由：两个正式面候选目录**无 NN- 前缀**，而 01-设计/02-调研/thunderbolt 有 ⇒ 结构级（2026-09-19 分家：原列四项里的 `docs/调研/` `docs/03-评审/` 已迁出仓 ⇒ 从 paths 删除，只留仓内两项）\n      出处：清单 §5 ④ · TSV 第 2 行（建议 = 不改 · 结构级，仅登记）\n  A02 [N1] 类型 ① · TSV 第 3 行 · 形态 path · 命中 ×1\n      对象：README.md\n      理由：行业约定文件；`publish/whitelist.txt` 公开面入口\n      出处：清单 §4.2 · TSV 第 3 行（建议 = 不改）\n  A03 [N6] 类型 ⑥ · TSV 第 5 行 · 形态 pair · 命中 ×0（不可达/未命中）\n      对象：core/internal/compat/compat\n      理由：Go 包同名约定（compat.go + compat.json）\n      出处：清单 §4.3 · TSV 第 5 行（建议 = 不改 · 登记为允许对）\n  A04 [N6] 类型 ⑥ · TSV 第 6 行 · 形态 pair · 命中 ×0（不可达/未命中）\n      对象：core/internal/compat/compat\n      理由：同上（JSON 侧）\n      出处：清单 §4.3 · TSV 第 6 行（同上）\n  A05 [N1] 类型 ① · TSV 第 8 行 · 形态 path · 命中 ×0（不可达/未命中）\n      对象：AGENTS.md\n      理由：行业约定文件（agent 入口）；`publish/private-paths.txt:29` 明列\n      出处：清单 §4.2 · TSV 第 8 行（建议 = 不改）\n  A06 [N6] 类型 ⑥ · TSV 第 15 行 · 形态 pair · 命中 ×1\n      对象：scripts/docs/edit-assert\n      理由：无扩展可执行件；改名 = 断全部 `python3 scripts/docs/edit-assert` 调用方\n      出处：清单 §4.3 · TSV 第 15 行（建议 = 不改 · 登记为允许对）\n  A07 [N6] 类型 ⑥ · TSV 第 16 行 · 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/docs/edit-assert\n      理由：`.md` = 同一脚本的说明书\n      出处：清单 §4.3 · TSV 第 16 行（同上）\n  A08 [N6] 类型 ⑥ · TSV 第 21 行 · 形态 pair · 命中 ×0（不可达/未命中）\n      对象：tools/doc_index_gen\n      理由：`tools/*.md`（145 个）为工具说明书，是既有约定\n      出处：清单 §4.3 · TSV 第 21 行（同上）\n  A09 [N6] 类型 ⑥ · TSV 第 22 行 · 形态 pair · 命中 ×0（不可达/未命中）\n      对象：tools/doc_index_gen\n      理由：同上（实现侧）\n      出处：清单 §4.3 · TSV 第 22 行（同上）\n  A10 [N6] 类型 ⑥ · TSV 第 23 行 · 形态 pair · 命中 ×0（不可达/未命中）\n      对象：tools/kb_docs_sync\n      理由：同上（工具说明书）\n      出处：清单 §4.3 · TSV 第 23 行（同上）\n  A11 [N6] 类型 ⑥ · TSV 第 24 行 · 形态 pair · 命中 ×0（不可达/未命中）\n      对象：tools/kb_docs_sync\n      理由：同上（实现侧）\n      出处：清单 §4.3 · TSV 第 24 行（同上）\n  A12 [N6] 类型 ⑥ · TSV 第 28 行 · 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/mutate-scan\n      理由：无扩展可执行件（同 edit-assert）\n      出处：清单 §4.3 · TSV 第 28 行（同上）\n  A13 [N6] 类型 ⑥ · TSV 第 29 行 · 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/mutate-scan\n      理由：`.md` = 同一脚本的说明书\n      出处：清单 §4.3 · TSV 第 29 行（同上）\n  A20 [N3+N4] 类型 ③ · TSV 第 143 行 · 形态 path · 命中 ×0（不可达/未命中）\n      对象：docs/常青/白皮书-虫族经济模型-v0.1.md\n      理由：版本号已在尾位，与本仓 `名-…-vX.Y[-YYYYMMDD]` 约定一致\n      出处：清单 §5 · TSV 第 143 行（建议 = 不改 · 版本号已在尾位）\n  A22 [N1] 类型 ① · TSV 第 147 行 · 形态 name · 命中 ×0（不可达/未命中）\n      对象：LICENSE , NOTICE , SECURITY.md , CONTRIBUTING.md , THIRD_PARTY_LICENSES.md\n      理由：公开面法定/治理文件，全大写是行业约定\n      出处：清单 §5 · TSV 第 147 行（建议 = 不改 · 符合行业约定）\nB 档 · 规则级规范豁免：3 条（期望 3）\n  B01 [N1] 形态 reserved · 命中 ×0（不可达/未命中）\n      对象：README.md , AGENTS.md , SKILL.md , VERSION , CHANGELOG.md\n      理由：行业/工具约定保留名 —— **只在文档面之外生效**（docs/ 内的同名件照判）。2026-09-21 发布闸 + `CHANGELOG.md`（Keep a Changelog 1.1.0 约定名 · P9 拍板「`CHANGELOG.md` 给机检」）\n  B02 [N6] 形态 pair_ext · 命中 ×0（不可达/未命中）\n      对象：go/.mod/.sum , Cargo/.toml/.lock\n      理由：构建工具链约定（Go module / Cargo），不属文档命名面。**休眠申报**：N6 的口径（至少一方是文档/脚本面扩展）已把它们挡在门外 ⇒ 本仓当前 0 命中\n  B03 [N6] 形态 pair_doc_machine · 命中 ×0（不可达/未命中）\n      对象：.md/.markdown/.mdx/.tsv/.csv/.json\n      理由：人读件 + 机读件同基名（本仓既有形态：`清单-命名规范化.md/.tsv` = 设计件 + 机读件）。**我方判断 · 与本脚本自身的差异已登记**：去掉这一条，该对会被判 ⑥（见 `scripts/gates/check-doc-name.md` §差异）\nC 档 · 新增登记（C01–C03 待批准 · C04 已拍 2026-09-18 · C05–C07 第二波任务书登记 2026-09-18 · C08 2026-09-22（门⑮ 主仓悬空引用）登记 · C09 2026-09-24（波11 序125 `W-43` hook git 环境）登记）：9 条（期望 9）\n  C01 [N6] 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/check-doc-meta\n      理由：本轮 W2 交付：门脚本 + 说明书（同 `scripts/docs/edit-assert` 既有约定）· **待父代理批准**\n  C02 [N6] 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/check-doc-name\n      理由：同上（本文档脚本 + 说明书）· **待父代理批准**\n  C03 [N6] 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/check-doc-freshness\n      理由：文档体系 P1 同批交付的第三条门脚本 + 说明书（`check-doc-freshness.{py,md}`）——与 C01/C02、A06/A07、A12/A13 同一形态 · **待父代理批准**\n  C04 [N6] 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/precommit-gates\n      理由：`scripts/gates/precommit-gates.md` 由**路 D**（门禁挂接 Q14）2026-09-18 20:43 新增，形态 = 「门脚本 + 说明书」（同 C01–C03 / A06/A07 / A12/A13）⇒ 登记为**允许对**。**已拍 2026-09-18（Mr2109）**：认下 C04（拍板来源 = 文档体系 P1 第三批 拍板单 §⑥）。撤法：删本行并把 `EXEMPT_C_EXPECTED` 4→3 ⇒ 该对判红照实回来\n  C05 [N6] 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/check-gate-coverage\n      理由：**门①（门覆盖自检）**的「门脚本 + 说明书」对（`check-gate-coverage.{py,md}`）——由第二波（2026-09-18）新建并挂进 `scripts/gates/precommit-gates.sh` 的 `gates` scope（模式 `tri`）。形态同 C01–C04 / A06/A07 / A12/A13 ⇒ 登记为**允许对**。撤法：删本行并把 `EXEMPT_C_EXPECTED` 7→6 ⇒ 该对判红照实回来\n  C06 [N6] 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/check-version-sources\n      理由：**门②（版本单一真源）**的「门脚本 + 说明书」对（`check-version-sources.{py,md}`）——同 C05（第二波新建 · 挂 `gates` scope · 模式 `tri`）。撤法：删本行并把 `EXEMPT_C_EXPECTED` 7→6 ⇒ 该对判红照实回来\n  C07 [N6] 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/check-wired-scripts\n      理由：**门③（脚本接线自检）**的「门脚本 + 说明书」对（`check-wired-scripts.{py,md}`）——同 C05（第二波新建 · 挂 `gates` scope · 模式 `tri-report` 只报告）。撤法：删本行并把 `EXEMPT_C_EXPECTED` 7→6 ⇒ 该对判红照实回来\n  C08 [N6] 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/check-dangling-refs\n      理由：**门⑮（主仓悬空引用 `docs/…` 指向不存在）**的「门脚本 + 说明书」对（`check-dangling-refs.{py,md}`）—— 同 C05（2026-09-22 新建 · 挂 `docs` scope · 模式 `tri-report` 只报告 · 同时进快速档）。撤法：删本行并把 `EXEMPT_C_EXPECTED` 8→7 ⇒ 该对判红照实回来\n  C09 [N6] 形态 pair · 命中 ×0（不可达/未命中）\n      对象：scripts/gates/check-git-env-hygiene\n      理由：**hook / 门禁脚本对外仓·合成仓的 git 调用先清环境**（`W-43` / `R-07`）的「门脚本 + 说明书」对（`check-git-env-hygiene.{py,md}`）—— 同 C05（2026-09-24 波11 序125 新建 · **单跑档、本笔不挂门**（本波件面未含 `precommit-gates.sh` ⇒ 不在 `add_step` 里）· 默认报告档 `tri-report`）。撤法：删本行并把 `EXEMPT_C_EXPECTED` 9→8 ⇒ 该对判红照实回来\nD 档 · 本轮 A 路拍板登记（登记不改）：2 条（期望 2）\n  D01 [N1] 形态 prefix_name · 命中 ×0（不可达/未命中）\n      对象：docs/ + 基名 README.md\n      理由：`docs/` 内的 `README.md` = **语言层入口件 / 重定向存根**（`docs/README.md` 是并入 `docs/index-nav.md` 后按零删除保留的旧路径；`docs/{zh,en,en/_drafts,site}/README.md` 是目录律入口）。README.md 属**行业约定保留名**（与 B01 同源）⇒ 只在文档面之内开这一个口子（AGENTS.md / SKILL.md / VERSION 在 docs/ 内照判，K8 要的那条没被放松）\n      出处：父代理 2026-09-18 拍板 · 本说明书 §差异 6/§例外表 D 档（(b) 路：加规则级豁免登记，不改文件名）\n  D02 [N3] 形态 path · 命中 ×0（不可达/未命中）\n      对象：bin/_history/zerg-core.prev-20260912.bak\n      理由：备件残片：`bin/` 被 `.gitignore` 忽略、**未纳入版本控制** ⇒ `git mv` 不可用（本路硬规矩：禁 `mv`/`rm`）；日期是它**作为备件的身份**（哪个二进制的前身）\n      出处：父代理 2026-09-18 拍板 · 本说明书 §差异 7（处置形态同《清单》A19/A21「移出仓」= 父代理动作）\n★ 逐行命中数为本次扫描现读（--hits）；×0 且注「不可达」= 共用键后行（见 A 档共用键注）。\n", wantErr: ""},
	{name: "夹具坏件 --summary", wantRC: 1, wantOut: "check-doc-name 摘要（一屏 · 命中全貌 + 豁免/误报率）\n  扫描域：14 个文件/目录\n  命中总数：6（可改面 不合规 5 · 告警 1 ｜ 冻结区 不合规 0 · 告警 0）\n  逐规则命中：N1×1 · N2×1 · N3×1 · N4×1 · N5×1 · N6×1\n  豁免命中：0（误报率 = 豁免 ÷ 命中总数 = 0.0%）\nFAIL：5 个不合规命中（只报告不改）\n", wantErr: ""},
	{name: "夹具坏件 --n7-shapes", wantRC: 1, wantOut: "check-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）\n扫描域：14 个文件/目录（排除名单同《清单》§0）\n  不合规 5（冻结区 0）· 告警 1（冻结区 0）\n  ★ 仓内冻结区：无 —— `docs/项目文档/` 已于 2026-09-26「批3」分家至仓外 `Zerg-内部文档/项目文档/`（仓内零命中 ⇒ 条目已删，处置同 2026-09-19 的 `docs/issues/`）⇒ 冻结计数恒 0/0（`--count-frozen` 机制保留）\n  N1 [FAIL] ①大小写：文件词干全为大写拉丁字母（≥3） —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/INDEX.md → 词干 'INDEX' 全为大写拉丁字母\n  N2 [FAIL] ②空格/全角：名称含 U+0020 / U+3000 / 全角标点 —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/研究 - 自我纠错.md → 名称含 U+0020\n  N3 [FAIL] ③日期后缀：词干含 -YYYYMMDD 或 -YYYYMM —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/01-设计/设计-文档体系-20260918.md → 词干含日期后缀 '-20260918' ⇒ 日期应移入 frontmatter `updated_at`\n  N4 [FAIL] ③版本段不在尾位：-vX.Y 其后还有分隔段 —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/01-设计/设计-v2.5.10-卵功能与对话机制.md → 版本段 '-v2.5.10-' 不在尾位 ⇒ 应移到尾部（`名-…-vX.Y[-YYYYMMDD]`）\n  N5 [WARN] ④前缀序号不一致：同层 NN- 与无前缀并存 —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/混用目录 → 同层混用 NN- 前缀（前缀 2 · 无前缀 2）\n  N6 [FAIL] ⑥同目录同基名、扩展名不同（去最后一个扩展名后相同） —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/01-设计/设计-文档体系 → 同基名不同扩展 .md/.py\n  豁免命中（逐条可查 · --list-exempt）：无\nN7 命中形态分类（对象 × 方向 × 拉丁形态）\n  无 N7 命中\nFAIL：5 个不合规命中（只报告不改）\n", wantErr: ""},
	{name: "夹具坏件 --exclude-prefix", wantRC: 1, wantOut: "★ 临时排除（--exclude-prefix · **未登记** · 仅本次分析生效）：bad/docs/01-设计/\ncheck-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）\n扫描域：10 个文件/目录（排除名单同《清单》§0）\n  不合规 2（冻结区 0）· 告警 1（冻结区 0）\n  ★ 仓内冻结区：无 —— `docs/项目文档/` 已于 2026-09-26「批3」分家至仓外 `Zerg-内部文档/项目文档/`（仓内零命中 ⇒ 条目已删，处置同 2026-09-19 的 `docs/issues/`）⇒ 冻结计数恒 0/0（`--count-frozen` 机制保留）\n  N1 [FAIL] ①大小写：文件词干全为大写拉丁字母（≥3） —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/INDEX.md → 词干 'INDEX' 全为大写拉丁字母\n  N2 [FAIL] ②空格/全角：名称含 U+0020 / U+3000 / 全角标点 —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/研究 - 自我纠错.md → 名称含 U+0020\n  N5 [WARN] ④前缀序号不一致：同层 NN- 与无前缀并存 —— 命中 1（可改面 1 · 冻结区 0）\n      · bad/docs/混用目录 → 同层混用 NN- 前缀（前缀 2 · 无前缀 2）\n  豁免命中（逐条可查 · --list-exempt）：无\nFAIL：2 个不合规命中（只报告不改）\n", wantErr: ""},
	{name: "N7 形态分类 + 收窄", wantRC: 0, wantOut: "check-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）\n扫描域：6 个文件/目录（排除名单同《清单》§0）\n  不合规 0（冻结区 0）· 告警 3（冻结区 0）\n  ★ 仓内冻结区：无 —— `docs/项目文档/` 已于 2026-09-26「批3」分家至仓外 `Zerg-内部文档/项目文档/`（仓内零命中 ⇒ 条目已删，处置同 2026-09-19 的 `docs/issues/`）⇒ 冻结计数恒 0/0（`--count-frozen` 机制保留）\n  N7 [WARN] ⑤中英紧邻：同一分隔段内汉字与拉丁字母直接相邻 —— 命中 3（可改面 3 · 冻结区 0）\n      · n7/docs/Agent模块.md → 段 'Agent模块' 内汉字与拉丁字母直接相邻（t模）\n      · n7/docs/模块Agent说明.md → 段 '模块Agent说明' 内汉字与拉丁字母直接相邻（块A）\n      · n7/docs/模块A说明.md → 段 '模块A说明' 内汉字与拉丁字母直接相邻（块A）\n  豁免命中（逐条可查 · --list-exempt）：无\nN7 命中形态分类（对象 × 方向 × 拉丁形态）\n  file   CJK→LAT    single   ×1\n  file   CJK→LAT    word     ×1\n  file   LAT→CJK    word     ×1\n  口径：方向 CJK→LAT（汉字在前）/ LAT→CJK（拉丁在前）；拉丁形态 version（v+数字 · GAP-166 已收窄不告警）/ single（单字母）/ word（≥2 字母）\nOK：无不合规命中（告警 3 · 已豁免 0）\n", wantErr: ""},
	{name: "夹具仓 --summary", wantRC: 1, wantOut: "check-doc-name 摘要（一屏 · 命中全貌 + 豁免/误报率）\n  扫描域：13 个文件/目录\n  命中总数：6（可改面 不合规 5 · 告警 1 ｜ 冻结区 不合规 0 · 告警 0）\n  逐规则命中：N1×1 · N2×1 · N3×1 · N4×1 · N5×1 · N6×1\n  豁免命中：0（误报率 = 豁免 ÷ 命中总数 = 0.0%）\nFAIL：5 个不合规命中（只报告不改）\n", wantErr: ""},
}

// cdnArgvByName —— 金标档名 ⇒ argv（与捕获时逐格同）。
func cdnArgvByName(name, good, bad, fx, fxempty, n7 string) []string {
	switch name {
	case "夹具仓·scope repo":
		return []string{"--repo-root", good, "--scope", "repo", "--no-self-test"}
	case "夹具仓·json":
		return []string{"--repo-root", good, "--scope", "repo", "--json", "--no-self-test"}
	case "夹具仓·max-examples 1":
		return []string{"--repo-root", bad, "--scope", "repo", "--max-examples", "1", "--no-self-test"}
	case "list-rules":
		return []string{"--list-rules"}
	case "list-exempt":
		return []string{"--list-exempt"}
	case "夹具好件必绿":
		return []string{"--repo-root", good, "--target", good, "--no-self-test"}
	case "夹具坏件必红":
		return []string{"--repo-root", fx, "--target", bad, "--no-self-test"}
	case "夹具前缀混用告警":
		return []string{"--repo-root", fx, "--target", filepath.Join(bad, "docs/混用目录"), "--no-self-test"}
	case "单件目标":
		return []string{"--repo-root", fx, "--target", filepath.Join(bad, "docs/INDEX.md"), "--no-self-test"}
	case "缺件目标":
		return []string{"--repo-root", fx, "--target", filepath.Join(fx, "no-such"), "--no-self-test"}
	case "scope docs 缺面":
		return []string{"--repo-root", fxempty, "--scope", "docs", "--no-self-test"}
	case "devdocs 根不在":
		return []string{"--repo-root", fx, "--scope", "devdocs", "--no-self-test"}
	case "json 夹具坏件":
		return []string{"--repo-root", fx, "--target", bad, "--json", "--no-self-test"}
	case "夹具好件 --list-exempt --hits":
		return []string{"--repo-root", good, "--target", good, "--list-exempt", "--hits", "--no-self-test"}
	case "夹具坏件 --summary":
		return []string{"--repo-root", fx, "--target", bad, "--summary", "--no-self-test"}
	case "夹具坏件 --n7-shapes":
		return []string{"--repo-root", fx, "--target", bad, "--n7-shapes", "--no-self-test"}
	case "夹具坏件 --exclude-prefix":
		return []string{"--repo-root", fx, "--target", bad, "--exclude-prefix", "bad/docs/01-设计/", "--no-self-test"}
	case "N7 形态分类 + 收窄":
		return []string{"--repo-root", fx, "--target", n7, "--n7-shapes", "--no-self-test"}
	case "夹具仓 --summary":
		return []string{"--repo-root", bad, "--scope", "repo", "--summary", "--no-self-test"}
	}
	return nil
}

// cdnNormCDN —— 归一本机位置类量（空夹具根 / 夹具根）⇒ 金标跨机可复现。
// ★ 替换次序：先具体空夹具根（`fxempty`）后夹具根（`fx`）。
func cdnNormCDN(s, fx, fxempty string) string {
	if fxempty != "" {
		s = strings.ReplaceAll(s, fxempty, "<FXE>")
	}
	if fx != "" {
		s = strings.ReplaceAll(s, fx, "<FX>")
	}
	return s
}

// cdnClearDevdocsEnv —— 清 devdocs 换根旋钮 ⇒ `devdocs 根不在` 格确定（与捕获同口径）。
func cdnClearDevdocsEnv(t *testing.T) {
	t.Setenv("ZERG_DEVDOCS_ROOT", "")
	t.Setenv("ZERG_DOCS_ALT", "")
	t.Setenv("ZERG_DOCS", "")
}

// cdnRunGolden —— 逐格现建夹具、现跑 Go 半、归一后与冻值逐字比；返回失配格数。
// plant 非 nil 时在跑前往夹具落「旧态」py（两态用）。
func cdnRunGolden(t *testing.T, tag string, plant func(good, bad, fx string)) int {
	t.Helper()
	good := cdnMakeGoodTree(t)
	bad := cdnMakeBadTree(t)
	fx := filepath.Dir(bad)
	fxempty := t.TempDir()
	n7 := cdnMakeN7Tree(t, fx)
	if plant != nil {
		plant(good, bad, fx)
	}
	mism := 0
	for _, c := range cdnGoldenTable {
		rc, out, errb := cdnRunGate(cdnArgvByName(c.name, good, bad, fx, fxempty, n7))
		gotOut := cdnNormCDN(out, fx, fxempty)
		gotErr := cdnNormCDN(errb, fx, fxempty)
		if rc != c.wantRC {
			mism++
			t.Errorf("[%s/%s] rc=%d（冻值 %d）", tag, c.name, rc, c.wantRC)
		}
		if gotOut != c.wantOut {
			mism++
			t.Errorf("[%s/%s] stdout 漂移\n实得=%q\n冻值=%q", tag, c.name, gotOut, c.wantOut)
		}
		if gotErr != c.wantErr {
			mism++
			t.Errorf("[%s/%s] stderr 漂移\n实得=%q\n冻值=%q", tag, c.name, gotErr, c.wantErr)
		}
	}
	return mism
}

// TestCheckDocNameGatePort_GoldenInternalized —— A 态（真仓 · 退役 py 不在盘）金标自证
// （✗ 不 Skip · 逐格真断言 · ✗ 不放宽为「只判非空」）。
func TestCheckDocNameGatePort_GoldenInternalized(t *testing.T) {
	repo := cdnRepoRoot(t) // 哨兵：锚 Go 半在盘 ⇒ fail-closed（✗ 不留 Skip）
	cdnClearDevdocsEnv(t)
	if py := filepath.Join(repo, filepath.FromSlash(cdnRetiredPyRel)); cdnIsRegular(py) {
		t.Fatalf("A 态前提破：退役 py 竟在盘 %s（本件前提 = 内化后无 py）", py)
	}
	if len(cdnGoldenTable) < 19 {
		t.Fatalf("金标格数 %d < 19", len(cdnGoldenTable))
	}
	if n := cdnRunGolden(t, "A真仓无py", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
}

// TestCheckDocNameGatePort_GoldenTwoStates —— 两态逐字同：A 态 = 夹具无 py（真仓现态）；
// B 态 = 合成「旧态」夹具（把退役 py 路径 `scripts/gates/check-doc-name.py` 放回）⇒ 两态对
// 同一套金标逐字同命中（✗ 不 Skip）。证 py 在/不在盘都不改判据结论（内化彻底 · 将来若有人让
// Go 半重新去读 py，B 态当场分家）。
func TestCheckDocNameGatePort_GoldenTwoStates(t *testing.T) {
	cdnRepoRoot(t) // 哨兵锚 Go 半（✗ 不留 Skip）
	cdnClearDevdocsEnv(t)
	if n := cdnRunGolden(t, "A夹具无py", nil); n != 0 {
		t.Fatalf("A 态金标失配 %d 格", n)
	}
	plant := func(good, bad, fx string) {
		pd := filepath.Join(fx, "scripts", "gates")
		if err := os.MkdirAll(pd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pd, "check-doc-name.py"),
			[]byte("# stub：造「旧态」——退役 py 在位（本件只证 py 在/不在盘不改判据结论）\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if n := cdnRunGolden(t, "B旧态有py", plant); n != 0 {
		t.Fatalf("B 态金标失配 %d 格（py 在盘不得改判据结论）", n)
	}
}

// ── --self-test 逐字金标（旧 `..._SelfTestSameRC` 的 py 对拍退场 ⇒ 收款 Go 常量）──────────────

// cdnReSelfTmp —— 归一 `--self-test` 的临时目录随机名 `check-doc-name-<digits>`。
var cdnReSelfTmp = regexp.MustCompile(`check-doc-name-\d+`)

// cdnNormSelfTest —— 归一本机时点量（$TMPDIR / 随机临时名）⇒ `--self-test` 金标可复现。
func cdnNormSelfTest(s string) string {
	if td := os.TempDir(); td != "" {
		s = strings.ReplaceAll(s, strings.TrimRight(td, "/")+"/", "<TMP>")
	}
	return cdnReSelfTmp.ReplaceAllString(s, "check-doc-name-<RND>")
}

// cdnSelfTestGolden —— `--self-test` 输出金标（归一 $TMPDIR / 随机名后 · 现算自证 · ✗ 手写）。
var cdnSelfTestGolden = "check-doc-name 自检（成对负控三层：好件必绿 / 坏件必红 / 缺件必 rc=2 + 元自检·注不进真仓）\n  ✓ 好件必绿（含允许对 edit-assert）（期望 rc=0，实得 rc=0）\n  ✓ 坏件必红 docs/INDEX.md（期望 rc=1，实得 rc=1）\n  ✓ 坏件必红 docs/研究 - 自我纠错.md（期望 rc=1，实得 rc=1）\n  ✓ 坏件必红 docs/01-设计/设计-文档体系-20260918.md（期望 rc=1，实得 rc=1）\n  ✓ 坏件必红 docs/02-调研/调研-虫族同类项目与定位-202608.md（期望 rc=1，实得 rc=1）\n  ✓ 坏件必红 docs/01-设计/设计-v2.5.10-卵功能与对话机制.md（期望 rc=1，实得 rc=1）\n  ✓ 整棵坏树必红且含 N6（同基名）（期望 rc=1，实得 rc=1）\n  ✓ 目录名命中只告警 docs/调研/协作骨架vX-核验/（期望 rc=0，实得 rc=0）\n  ✓ 目录名命中只告警 docs/新目录-20260918/（期望 rc=0，实得 rc=0）\n  ✓ 同基名未登记必红 docs/01-设计/设计-文档体系.md（期望 rc=1，实得 rc=1）\n  ✓ 同基名 .md/.tsv 对按 B03 豁免（期望 rc=0，实得 rc=0）\n  ✓ 前缀序号混用只告警（期望 rc=0，实得 rc=0）\n  ✓ 缺件 · 目标路径不存在（期望 rc=2，实得 rc=2）\n  ✓ 缺件 · 扫描域为空（期望 rc=2，实得 rc=2）\n  ✓ 缺件 · 默认文档面 docs/ 不存在 ⇒ BLOCKED（不是假绿）（期望 rc=2，实得 rc=2）\n  ✓ 缺件 · docs/ 下 0 篇 md（无可扫内容）⇒ BLOCKED（不是假绿）（期望 rc=2，实得 rc=2）\n  ✓ 例外表条数自证（A15 / B3 / C9 / D2）\n  ✓ 只报告不改（夹具树逐字节不变：一致）\n  ✓ 好件树 + 未登记同基名对必红（N6 判据有牙）（期望 rc=1，实得 rc=1）\n  ✓ --scope repo 可用（= 仓根口径）（期望 rc=1，实得 rc=1）\n  ✓ --scope docs 解析到 <仓根>/docs（期望 rc=2，实得 rc=2）\n  ✓ 用法错（未知参数）必 rc=2 不给结论（期望 rc=2，实得 rc=2）\n  ✓ D3① devdocs 根不在 ⇒ BLOCKED（仓外不可达不许当绿）（期望 rc=2，实得 rc=2）\n  ✓ --list-exempt --hits 逐行给命中数（期望 rc=0，实得 rc=0）\n  ✓ --summary 一屏摘要（含误报率）（期望 rc=0，实得 rc=0）\n  ✓ --exclude-prefix 临时排除生效（N6 消失）（期望 rc=1，实得 rc=1）\n  ✓ --n7-shapes N7 形态分类输出（期望 rc=1，实得 rc=1）\n  ✓ N7 收窄：版本标记紧邻不告警（期望 rc=0，实得 rc=0）\n  ✓ N7 收窄后有牙：词形态照告警（期望 rc=0，实得 rc=0）\n  ✓ 自检未过 ⇒ rc=2 且拒绝扫真目标（元自检 · 注入副本）\n  自检用例：期望 30 条 · 实跑 30 条\n  自检临时目录：<TMP>check-doc-name-<RND>（跑完已删）\n"

// TestCheckDocNameGatePort_SelfTestGolden —— `--self-test` 逐字金标（✗ 不 Skip）。
func TestCheckDocNameGatePort_SelfTestGolden(t *testing.T) {
	rc, out, errb := cdnRunGate([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("--self-test rc=%d（要 0）\n%s", rc, out)
	}
	if got := cdnNormSelfTest(out); got != cdnSelfTestGolden {
		t.Fatalf("--self-test 输出漂移\n实得=%q\n冻值=%q", got, cdnSelfTestGolden)
	}
	if errb != "" {
		t.Fatalf("--self-test stderr 应空：%q", errb)
	}
}

// TestCheckDocName_MetaSelfCheckRefuses —— 回归牙（M2 战线①对齐批-3）：
// ⑫ 元自检 —— 自检未过 ⇒ main 必须**拒扫真目标**（rc=2 · 首行=拒扫抬头 · 末行非结论行）。
// 哑化 main 的注入分支 / 去掉拒扫抬头 ⇒ 本用例 FAIL（带行号）。
func TestCheckDocName_MetaSelfCheckRefuses(t *testing.T) {
	t.Setenv("CHECK_DOC_NAME_META_FORCE_FAIL", "1")
	d := t.TempDir()
	var out, errb bytes.Buffer
	rc := checkDocNameGateMain(&out, &errb, []string{"--repo-root", d, "--target", d})
	if rc != 2 {
		t.Fatalf("自检未过 ⇒ 该退 2（拒扫真目标），实得 %d\nout=%q", rc, out.String())
	}
	var ol []string
	for _, l := range strings.Split(out.String(), "\n") {
		if strings.TrimSpace(l) != "" {
			ol = append(ol, l)
		}
	}
	if len(ol) == 0 || ol[0] != "自检未过 ⇒ 拒绝扫真目标（rc=2 不给结论）" {
		t.Errorf("首行该是拒扫抬头，实得 %q", ol)
	}
	if n := len(ol); n > 0 && (strings.HasPrefix(ol[n-1], "OK：") || strings.HasPrefix(ol[n-1], "FAIL：")) {
		t.Errorf("末行不该是结论行：%q", ol[n-1])
	}

	// 同牙另一半（对齐 py 现件）：`--self-test` 清单必须含 ⑫ 元自检格、断言数 30（哑化该格 ⇒ FAIL）。
	var o2, e2 bytes.Buffer
	if rc2 := checkDocNameGateMain(&o2, &e2, []string{"--self-test"}); rc2 != 0 {
		t.Fatalf("--self-test 该退 0，实得 %d\n%s", rc2, o2.String())
	}
	if !strings.Contains(o2.String(), "✓ 自检未过 ⇒ rc=2 且拒绝扫真目标（元自检 · 注入副本）") ||
		!strings.Contains(o2.String(), "自检用例：期望 30 条 · 实跑 30 条") {
		t.Errorf("自检清单缺 ⑫ 元自检格 / 断言数不为 30：\n%s", o2.String())
	}
}
