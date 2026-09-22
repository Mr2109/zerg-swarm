// cli_impact_docindex_test.go —— `zerg impact` 的 **`C3` 文档面与反向索引**判据
// （任务单-影响面实施-20260922 §四 `C3` 八字段判据①–⑤ · 设计-变更影响面-v1.6 §6.3/§6.4/§6.6 第 4 条 ·
// 契约真源 `core/internal/contract/impact-docindex.json` = 登记表 `S-k`）。
//
// 落在 `package main_test`：`ImpactDocFaceForTest` 跑的是**当前源码**的取数路径（不是盘上旧制品）。
//
// 五格判据的机检口（**都在本文件里真跑**，语境全走**合成文档面**（临时 git 仓 + 几个 md）⇒
// 不依赖真文档树在不在、也不碰真状态目录）：
//
//	① **倒排可跑**：一次扫全量 md ⇒ 件 / 行 / 块 / 词 / 对 + **耗时** 都读得出来（正控）；
//	② **产物键含文档仓 `head_sha`**：`head_sha` 变了 ⇒ **旧件不许命中**（负控：合成文档面上再打一笔提交）；
//	③ **三档口径可切**：A（裸词）⊇ B（反引号包住的符号名 · 默认）⊇ C（与「件:行」同行）—— 逐档现跑；
//	④ **`N = 0` 必写「面内未见」+ 面内定义**（负控：把定义那半抹掉 ⇒ 判定口必须报错）；
//	⑤ **通用词假阳必须排除**（四个词逐条 + 负控：不排除 ⇒ 判定口报错）。
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// docFaceFix 造一片**合成文档面**（临时 git 仓 + 两个 md）：一篇用反引号包住目标符号（B 档命中）、
// 一篇只有裸词（A 档命中 · B 档不命中）、另有「件:行」同行的那一行（C 档）。通用词（`fetch` 一族）
// 刻意也写进反引号里 ⇒ 判据⑤ 的排除面有东西可排。
func docFaceFix(t *testing.T, root, target string) {
	t.Helper()
	docs := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(docs, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("建目录失败：%v", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("写件失败：%v", err)
		}
	}
	// ① B 档命中：反引号包住的符号名 + 通用词（判据⑤ 的排除面）。
	write("01-设计/甲.md", "# 甲\n\n改 `"+target+"` 要看调用者。\n\n通用词：`fetch` · `Retry` · `writeError` · `WebSearch`。\n")
	// ② A 档命中、B 档不命中：裸词（不带反引号）。
	write("02-调研/乙.md", "# 乙\n\n这一篇写 "+target+" 但**不带反引号** ⇒ 只有 A 档看得见。\n")
	// ③ C 档命中：与「件:行」同一行 + 反引号包住。
	write("03-实测/丙.md", "# 丙\n\n`"+target+"` 在 `core/cmd/zerg/main.go:1` 那一行有引用。\n")
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = docs
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("合成文档面的 git %v 失败：%v · %s", args, err, out)
		}
	}
	git("init", "-q", ".")
	git("add", "-A")
	git("commit", "-qm", "init")
	t.Setenv("ZERG_DOCS_ROOT", docs)
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
}

// judgeDocFaceZero 判「面内未见」那一句：**必须**同时带 `N = 0` 措辞与**面内定义**（判据④）。
// 负控：把定义那半抹掉 ⇒ 必须报错（证明这一格有牙）。
func judgeDocFaceZero(detail string) error {
	if !strings.Contains(detail, "⇒ **面内未见**") {
		return errStr("`N = 0` 没写「面内未见」（判据④：零命中必须写成面内未见这一句）")
	}
	if strings.Contains(detail, "⇒ **无文档受影响**") {
		return errStr("判据④ 明文不许的写法：零命中的结论句写成了「无文档受影响」（读到的是「面内这片没人提」）")
	}
	if !strings.Contains(detail, "面内定义：") {
		return errStr("`N = 0` 没带**面内定义**（判据④：不许写成「无文档受影响」）")
	}
	if !strings.Contains(detail, "全量 `.md`") {
		return errStr("面内定义没写清「面 = 该树全部 `.md`」（判据④）")
	}
	return nil
}

// judgeDocFaceStop 判「通用词假阳已排除」那一句（判据⑤）：
// 四个词要逐条出现，且**三档的排除计数都要有值**（不许只报一个合计）。
func judgeDocFaceStop(detail string) error {
	if !strings.Contains(detail, "通用词假阳已按契约件排除") {
		return errStr("没写通用词假阳已排除（判据⑤）")
	}
	for _, w := range []string{"fetch", "Retry", "writeError", "WebSearch"} {
		if !strings.Contains(detail, w) {
			return errStr("排除名单里少了 " + w + "（判据⑤）")
		}
	}
	if !strings.Contains(detail, "A/B/C 三档上的命中计数") {
		return errStr("排除计数没按三档给（判据⑤：口径要写清）")
	}
	return nil
}

// TestImpactDocIndex_BuildAndQuery —— 判据①③④⑤ 的正控（合成文档面）：
// 一次扫建倒排（件/行/块/词/对 + 耗时可读）· 三档可切且 A ⊇ B ⊇ C · 通用词排除 · 落点不在仓内。
func TestImpactDocIndex_BuildAndQuery(t *testing.T) {
	root := repoRootFromCLI(t)
	target := "impactDocProbe"
	docFaceFix(t, root, target)

	// ① 现建一次（`build=true`）：体量读数 + 耗时。
	built := zerg.ImpactDocFaceForTest(root, "core/cmd/zerg/main.go", true, "B")
	if built.Status != "取值" {
		t.Fatalf("判据① 破：现建后该取值，实得 %q · %s", built.Status, built.Detail)
	}
	if built.Files != 3 || built.Lines == 0 || built.Blocks == 0 || built.Words == 0 || built.Pairs == 0 {
		t.Errorf("判据① 破：体量读数不全（件 %d · 行 %d · 块 %d · 词 %d · 对 %d）",
			built.Files, built.Lines, built.Blocks, built.Words, built.Pairs)
	}
	if built.MS <= 0 {
		t.Errorf("判据① 破：现建耗时没读出来（%d ms）", built.MS)
	}
	if !strings.Contains(built.Detail, "一次扫全量") && !strings.Contains(built.Detail, "现建") {
		t.Errorf("判据① 破：读数里没写「一次扫 / 现建」：%s", built.Detail)
	}
	// ④ 通用词排除（判据⑤）+ 面内定义（判据④ 的门面）都在读数里。
	if err := judgeDocFaceStop(built.Detail); err != nil {
		t.Errorf("判据⑤ 破：%v · %s", err, built.Detail)
	}
	if !strings.Contains(built.Detail, "面内定义：") {
		t.Errorf("判据④ 破：读数里没带面内定义：%s", built.Detail)
	}
	// 落点**不在仓内**（判据中的路径纪律）：索引件都在状态目录下。
	files := zerg.ImpactDocIndexFilesForTest(root)
	if len(files) == 0 {
		t.Fatal("判据① 破：状态目录的 `impact-index/` 里一件都没有")
	}
	for _, f := range files {
		if !strings.HasPrefix(f, "docindex-") {
			t.Errorf("落点里出现了不认识的件：%s（只许 docindex-<head 前 8>-<口径前 8>.<tsv|json>）", f)
		}
	}
	if strings.Contains(built.Index, "/core/") {
		t.Errorf("落点跑到仓里去了（判据：不在仓内、不进公开面、不进 git 跟踪）：%s", built.Index)
	}

	// ③ 三档可切（同一目标 · 逐档现跑）：A ⊇ B ⊇ C。
	a := zerg.ImpactDocFaceForTest(root, "core/cmd/zerg/main.go", false, "A")
	b := zerg.ImpactDocFaceForTest(root, "core/cmd/zerg/main.go", false, "B")
	c := zerg.ImpactDocFaceForTest(root, "core/cmd/zerg/main.go", false, "C")
	for _, v := range []zerg.ImpactDocFaceViewForTest{a, b, c} {
		if v.Status != "取值" {
			t.Fatalf("判据③ 破：档 %s 该取值（索引已建），实得 %q · %s", v.Tier, v.Status, v.Detail)
		}
	}
	if !(a.Docs >= b.Docs && b.Docs >= c.Docs) {
		t.Errorf("判据③ 破：三档命中数该满足 A ⊇ B ⊇ C，实得 A=%d · B=%d · C=%d", a.Docs, b.Docs, c.Docs)
	}
	if b.Docs == 0 {
		t.Errorf("判据③ 破：B 档（默认）一篇都没命中 —— 合成面里 `%s` 明明写在反引号里：%s", target, b.Detail)
	}
	if c.Docs == 0 {
		t.Errorf("判据③ 破：C 档（与「件:行」同行）一篇都没命中：%s", c.Detail)
	}
	if !strings.Contains(a.Detail, "口径 **A 档**") || !strings.Contains(b.Detail, "口径 **B 档**") || !strings.Contains(c.Detail, "口径 **C 档**") {
		t.Error("判据③ 破：三档那一格没把档号与档名打出来")
	}

	// ⑤ 负控：不排除通用词 ⇒ 判定口必须报错（证明这一格有牙）。
	if err := judgeDocFaceStop(strings.ReplaceAll(built.Detail, "通用词假阳已按契约件排除", "（没写排除）")); err == nil {
		t.Error("负控⑤失败：抹掉「通用词假阳已排除」竟判过")
	}
	if err := judgeDocFaceStop(strings.ReplaceAll(built.Detail, "A/B/C 三档上的命中计数", "计数")); err == nil {
		t.Error("负控⑤失败：排除计数不按三档给竟判过")
	}
}

// TestImpactDocIndex_KeyOnDocHeadSHA —— 判据②：**键含文档仓 `head_sha“** ——
// 负控：合成文档面上再打一笔提交（`head_sha` 变）⇒ **旧件不许命中**（照实写「未建索引」，重扫）。
func TestImpactDocIndex_KeyOnDocHeadSHA(t *testing.T) {
	root := repoRootFromCLI(t)
	docFaceFix(t, root, "impactDocProbe2")

	first := zerg.ImpactDocFaceForTest(root, "core/cmd/zerg/main.go", true, "B")
	if first.Status != "取值" || first.Head == "" {
		t.Fatalf("准备态不对：%s · head=%q", first.Status, first.Head)
	}
	// 默认档（build=false）在 head 未变时**必须**命中（正控的另一半：不是恒不命中）。
	again := zerg.ImpactDocFaceForTest(root, "core/cmd/zerg/main.go", false, "B")
	if again.Status != "取值" {
		t.Fatalf("正控破：head 未变却不命中 ⇒ 判定面恒假：%s · %s", again.Status, again.Detail)
	}
	// 在合成文档面上再打一笔提交 ⇒ head_sha 变。
	docs := os.Getenv("ZERG_DOCS_ROOT")
	if err := os.WriteFile(filepath.Join(docs, "01-设计", "甲.md"), []byte("# 甲\n\n又改了：`impactDocProbe2` 还在。\n"), 0o644); err != nil {
		t.Fatalf("改合成文档失败：%v", err)
	}
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = docs
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add 失败：%v · %s", err, out)
	}
	cmd = exec.Command("git", "commit", "-qm", "second")
	cmd.Dir = docs
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit 失败：%v · %s", err, out)
	}
	stale := zerg.ImpactDocFaceForTest(root, "core/cmd/zerg/main.go", false, "B")
	if stale.Status != "未建索引" {
		t.Errorf("判据② 破：`head_sha` 变了却还命中（旧件当新答案）：状态 %q · %s", stale.Status, stale.Detail)
	}
	// 键含 `head_sha` 的两种表现都算合规：① 文件名换了（键含 head ⇒ 旧键那一件**读不到**）；
	// ② 万一读到了同名件，也要**判成不匹配**。两句话必须出现一句（不许静默）。
	if !strings.Contains(stale.Detail, "索引不在") && !strings.Contains(stale.Detail, "head_sha` 不匹配") {
		t.Errorf("判据② 破：不命中要写明是「键含 head_sha ⇒ 旧件不在 / 不匹配」：%s", stale.Detail)
	}
	if stale.Head == first.Head {
		t.Error("判据② 破：合成文档面的 `head_sha` 没变 ⇒ 这一格没测到东西")
	}
	// 再现建 ⇒ 恢复取值（键换了 ⇒ 新件）。
	rebuilt := zerg.ImpactDocFaceForTest(root, "core/cmd/zerg/main.go", true, "B")
	if rebuilt.Status != "取值" || rebuilt.Head != stale.Head {
		t.Errorf("判据② 破：重扫后该取值（新键），实得 %q · head=%q", rebuilt.Status, rebuilt.Head)
	}
}

// TestImpactDocIndex_ZeroIsFaceMiss —— 判据④：**`N = 0` 必写「面内未见」+ 面内定义**。
//
// 负控两条：合成面里没人提的词（真零命中）· 抹掉定义那半 ⇒ 判定口必须报错。
func TestImpactDocIndex_ZeroIsFaceMiss(t *testing.T) {
	root := repoRootFromCLI(t)
	// 合成面里**只**写别的词 ⇒ 目标件的件名 / 仓内路径 / 顶层符号一个都不在（真零命中）。
	docFaceFix(t, root, "impactDocOnlyOtherWord")
	zero := zerg.ImpactDocFaceForTest(root, "core/cmd/zerg/family_impact_docindex.go", true, "C")
	if zero.Status != "取值" {
		t.Fatalf("准备态不对：%s · %s", zero.Status, zero.Detail)
	}
	if zero.Docs != 0 {
		t.Fatalf("这一格要的是**真零命中**，实得 %d 篇（合成面没造好 · %s）", zero.Docs, zero.Detail)
	}
	if err := judgeDocFaceZero(zero.Detail); err != nil {
		t.Errorf("判据④ 破：%v · %s", err, zero.Detail)
	}
	// 负控：抹掉「面内定义：」那半 ⇒ 必须报错。
	if err := judgeDocFaceZero(strings.ReplaceAll(zero.Detail, "面内定义：", "（抹掉）")); err == nil {
		t.Error("负控④失败：零命中却没写面内定义竟判过")
	}
	// 负控：写成「无文档受影响」⇒ 必须报错（这一句是判据④ 点名不许的写法）。
	bad := strings.Replace(zero.Detail, "⇒ **面内未见**", "⇒ **无文档受影响**", 1)
	if err := judgeDocFaceZero(bad); err == nil {
		t.Error("负控④失败：写成「无文档受影响」竟判过")
	}
}

// TestImpactDocIndex_NoAutoDeleteNoSecondScan —— 红线静态自检（实现件层面）：
//
//	· 索引件里**没有删除动作**，也没有「自动删 / 自动回滚 / 自动收窄」这些动作串；
//	· **不逐符号全扫**：实现里没有「逐符号起进程」那一套（对照 0.147s/符号 ⇒ 3.5k 条 ≈ 8.6 分钟）；
//	· 落点只经契约件（`dirs.index`）⇒ 源码里不另写第二份目录名。
func TestImpactDocIndex_NoAutoDeleteNoSecondScan(t *testing.T) {
	src, err := zerg.ImpactDocIndexSourceForTest()
	if err != nil {
		t.Fatalf("读实现件失败：%v", err)
	}
	for _, bad := range []string{"os.Remove", "os.RemoveAll", "os.Truncate"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现件里有删除动作 %q", bad)
		}
	}
	for _, bad := range []string{"自动删", "自动回滚", "自动收窄"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现件里出现了 %q（只许作为契约件的「不许接的动作」被读出）", bad)
		}
	}
	if strings.Contains(src, "code find") {
		t.Errorf("红线破：实现件里出现了 `code find`（`R53` 明文：不扩它扫文档面）")
	}
	// 「一次扫全量 md」的可判形态（`R23`）：**恰好一遍 `filepath.Walk`**，且实现里没有
	// 符号级两器的痕迹（`callgraph` / `ast-grep` 那两套是**逐符号**那条路 ⇒ 不许出现在本件）。
	if n := strings.Count(src, "filepath.Walk"); n != 1 {
		t.Errorf("红线破：实现件里 `filepath.Walk` 出现 %d 次（`R23`：**一次**扫全量 md ⇒ 恰好一遍）", n)
	}
	for _, bad := range []string{"callgraph", "ast-grep", "sg\"", "gopls"} {
		if strings.Contains(src, bad) {
			t.Errorf("红线破：实现件里出现了 %q（逐符号全扫那条路的两器 —— `R23` 明文不许）", bad)
		}
	}
	if !strings.Contains(src, "impactCacheIndexDir(layout)") {
		t.Error("落点没经契约件（`dirs.index`）⇒ 源码里另写了一份目录名")
	}
}
