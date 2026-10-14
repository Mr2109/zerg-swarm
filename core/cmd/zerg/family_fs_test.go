// family_fs_test.go —— `family_fs.go` 的**判据核心**定向测试（F38）。
//
// 落点：`package main`（**内部测试** —— 核心件未接命令树 ⇒ ✗ 走不了 `zerg.RunForTest`；
// 直调 `fsUnprotectedCore`）。夹具全在 `t.TempDir()` 的**仓外**临时仓里 ⇒ ✗ 不碰真仓。
//
// 四态 ＋ 负控（本件把每一格的读数都打出来 · 供报告逐字取）：
//
//	① 干净仓 ⇒ rc=0 · 人面「无未保护件」；
//	② 未跟踪 ∩ 未被忽略 ∩ 大件 ⇒ **必列出且点名**（带字节 ＋ 类型）；
//	③ 未跟踪但**已忽略** ⇒ **必不列**（★ 证据：忽略面真进判据）；
//	④ 真源取不到（非 git 树）⇒ **rc=2 不给结论**（✗ 不假绿）；
//	⑤ 负控① · **哑化忽略判定** ⇒ 已忽略件**必红**（态③ 翻面）；成对正控 = 默认判据必不列；
//	⑥ 负控② · **已跟踪大件必不列**（「版本面无」这条真起作用）；成对 = 同件删出索引（改未跟踪）⇒ 必列；
//	⑦ 负控③ · **排除树（bin/ · dist/）必不列**；成对 = 同一件落 `src/` ⇒ 必列（排除面不过宽）；
//	⑧ 体量降序 ＋ Top 裁（降序前 N 机制）；
//	⑨ 只读自证（夹具）：跑前 / 跑后 HEAD ＋ 状态 ＋ 未跟踪清单**逐字同**；
//	⑩ 真仓只读档（env `FS_UNPROTECTED_REAL_ROOT` 给才跑 · 否则跳过）。
package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fsuGit —— 在夹具仓跑一条 git 命令（夹具自建自管 · 全在 t.TempDir() 里）。
func fsuGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@t",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("夹具 git %v 失败: %v\n%s", args, err, out)
	}
	return string(out)
}

// fsuWrite —— 往夹具仓写一件（自动建父目录）。
func fsuWrite(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// fsuWriteN —— 写一件 n 字节（体量面夹具）。
func fsuWriteN(t *testing.T, path string, n int) {
	t.Helper()
	fsuWrite(t, path, make([]byte, n))
}

// fsuRepo —— 造一枚**仓外夹具仓**：一件已跟踪件 ＋ `.gitignore`（`*.ign` 与 `ignored-dir/`）。
func fsuRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("夹具需要 git")
	}
	root := t.TempDir()
	fsuGit(t, root, "init", "-q")
	fsuGit(t, root, "config", "user.email", "fixture@t")
	fsuGit(t, root, "config", "user.name", "fixture")
	fsuWrite(t, filepath.Join(root, "tracked.txt"), []byte("tracked\n"))
	fsuWrite(t, filepath.Join(root, ".gitignore"), []byte("*.ign\nignored-dir/\n"))
	fsuGit(t, root, "add", "tracked.txt", ".gitignore")
	fsuGit(t, root, "commit", "-qm", "init")
	return root
}

// fsuHas —— 结论里有没有点名这件。
func fsuHas(res fsUnprotectedResult, path string) bool {
	for _, it := range res.Items {
		if it.Path == path {
			return true
		}
	}
	return false
}

// fsuItem —— 取结论里某件（找不到 ⇒ 失败）。
func fsuItem(t *testing.T, res fsUnprotectedResult, path string) fsUnprotectedItem {
	t.Helper()
	for _, it := range res.Items {
		if it.Path == path {
			return it
		}
	}
	t.Fatalf("结论里没有点名 %q（实得 %d 件）", path, len(res.Items))
	return fsUnprotectedItem{}
}

// fsuFingerprint —— 夹具仓的「仓 ＋ 盘」指纹（只读自证用）。
func fsuFingerprint(t *testing.T, root string) string {
	t.Helper()
	return "HEAD=" + strings.TrimSpace(fsuGit(t, root, "rev-parse", "HEAD")) +
		"\nSTATUS:\n" + fsuGit(t, root, "status", "--porcelain", "--untracked-files=all") +
		"\nOTHERS:\n" + fsuGit(t, root, "ls-files", "--others", "-z")
}

// 态① 干净仓 ⇒ rc=0 ·「无未保护件」。
func TestFsUnprotectedCleanRepo(t *testing.T) {
	root := fsuRepo(t)
	res, rc := fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	var b strings.Builder
	fsUnprotectedHuman(&b, res)
	t.Logf("态① 干净仓 · rc=%d · items=%d · 人面=%q", rc, len(res.Items), strings.TrimSpace(b.String()))
	if rc != exitOK {
		t.Fatalf("干净仓 rc=%d（期望 0）", rc)
	}
	if len(res.Items) != 0 {
		t.Fatalf("干净仓不该有未保护件 ⇒ 实得 %d 件", len(res.Items))
	}
	if !strings.Contains(b.String(), "无未保护件") {
		t.Fatalf("干净仓人面未报「无未保护件」：%q", b.String())
	}
}

// 态② 未跟踪 ∩ 未被忽略 ∩ 大件 ⇒ 必列出且点名（带字节 ＋ 类型）。
func TestFsUnprotectedListsUntrackedBig(t *testing.T) {
	root := fsuRepo(t)
	fsuWriteN(t, filepath.Join(root, "bundle", "big.bin"), 4096)
	res, rc := fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	t.Logf("态② rc=%d · candidates=%d · items=%v", rc, res.Candidates, fsUnprotectedRows(res))
	if rc != exitFail {
		t.Fatalf("有大件 rc=%d（期望 1）", rc)
	}
	it := fsuItem(t, res, "bundle/big.bin")
	if it.Bytes != 4096 || it.Kind != fsKindRegular {
		t.Fatalf("点名件的字节/类型不符：bytes=%d kind=%q", it.Bytes, it.Kind)
	}
}

// 态③ 未跟踪但已忽略 ⇒ 必不列（忽略面真进判据）。
func TestFsUnprotectedIgnoredNotListed(t *testing.T) {
	root := fsuRepo(t)
	fsuWriteN(t, filepath.Join(root, "secret.ign"), 8192)
	// 成对参照：确认它确在「未跟踪」面（`ls-files --others` 含已忽略件）。
	raw := fsuGit(t, root, "ls-files", "--others", "-z")
	if !strings.Contains(raw, "secret.ign") {
		t.Fatalf("夹具前提破：secret.ign 不在 `ls-files --others` 面里 ⇒ 忽略面无从判")
	}
	res, rc := fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	t.Logf("态③ rc=%d · items=%v（secret.ign 必不在此）", rc, fsUnprotectedRows(res))
	if rc != exitOK {
		t.Fatalf("只剩已忽略件 rc=%d（期望 0 —— 判据跑通无件）", rc)
	}
	if fsuHas(res, "secret.ign") {
		t.Fatalf("已忽略件被列出 ⇒ 忽略面没进判据")
	}
}

// 态④ 真源取不到（非 git 树）⇒ rc=2 不给结论。
func TestFsUnprotectedNoTruthSource(t *testing.T) {
	root := t.TempDir() // 非 git 树（没有 .git）
	res, rc := fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	t.Logf("态④ 非 git 树 · rc=%d · items=%d", rc, len(res.Items))
	if rc != exitUsage {
		t.Fatalf("非 git 树 rc=%d（期望 2 —— 不给结论 · ✗ 不假绿）", rc)
	}
	if len(res.Items) != 0 {
		t.Fatalf("不给结论时不该出示件（实得 %d 件）", len(res.Items))
	}
}

// 负控① 哑化忽略判定 ⇒ 已忽略件必红（成对正控 = 默认判据必不列）。
func TestFsUnprotectedNegIgnoreMuted(t *testing.T) {
	root := fsuRepo(t)
	fsuWriteN(t, filepath.Join(root, "secret.ign"), 8192)

	// 正控：默认判据 ⇒ 必不列。
	res, _ := fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	if fsuHas(res, "secret.ign") {
		t.Fatalf("正控破：默认判据把已忽略件列了出来")
	}

	// 哑化「忽略判定」⇒ 同一件**必红**（证明这条牙真咬在忽略面上）。
	orig := fsUnprotIgnoreFn
	fsUnprotIgnoreFn = func(string, []string) (map[string]bool, bool) { return map[string]bool{}, true }
	defer func() { fsUnprotIgnoreFn = orig }()
	res2, _ := fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	t.Logf("负控① 哑化后 · rc=%d · items=%v", rcOf(res2), fsUnprotectedRows(res2))
	if !fsuHas(res2, "secret.ign") {
		t.Fatalf("负控破：哑化忽略判定后，已忽略件仍未列出 ⇒ 这颗牙没牙（态③ 不会红）")
	}
}

// rcOf —— 只为日志：给结论配一个直观值（哑化档重算 rc 用）。
func rcOf(res fsUnprotectedResult) int {
	if len(res.Items) == 0 {
		return 0
	}
	return 1
}

// 负控② 已跟踪大件必不列（「版本面无」这条真起作用）；成对 = 删出索引（改未跟踪）⇒ 必列。
func TestFsUnprotectedNegTrackedBigNotListed(t *testing.T) {
	root := fsuRepo(t)
	fsuWriteN(t, filepath.Join(root, "tracked-big.bin"), 65536)
	fsuGit(t, root, "add", "tracked-big.bin")
	fsuGit(t, root, "commit", "-qm", "add big")

	res, _ := fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	t.Logf("负控② 已跟踪 · rc=%d · items=%v", rcOf(res), fsUnprotectedRows(res))
	if fsuHas(res, "tracked-big.bin") {
		t.Fatalf("已跟踪件被当未保护件列出 ⇒「版本面无」这条失效")
	}

	// 成对：同一件删出索引（变成未跟踪）⇒ 必列。
	fsuGit(t, root, "rm", "--cached", "tracked-big.bin")
	res2, _ := fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	t.Logf("负控② 成对 · 删出索引后 · rc=%d · items=%v", rcOf(res2), fsUnprotectedRows(res2))
	if !fsuHas(res2, "tracked-big.bin") {
		t.Fatalf("删出索引后仍未列出 ⇒「未跟踪」这条失效")
	}
}

// 负控③ 排除树（bin/ · dist/）必不列；成对 = 同一件落 src/ ⇒ 必列（排除面不过宽）。
func TestFsUnprotectedNegExcludedDirs(t *testing.T) {
	root := fsuRepo(t)
	fsuWriteN(t, filepath.Join(root, "bin", "big.bin"), 4096)
	fsuWriteN(t, filepath.Join(root, "dist", "big.bin"), 4096)
	fsuWriteN(t, filepath.Join(root, "src", "big.bin"), 4096)

	res, _ := fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	t.Logf("负控③ rc=%d · items=%v", rcOf(res), fsUnprotectedRows(res))
	if fsuHas(res, "bin/big.bin") || fsuHas(res, "dist/big.bin") {
		t.Fatalf("排除树未生效：bin/ 或 dist/ 下的件被列出")
	}
	if !fsuHas(res, "src/big.bin") {
		t.Fatalf("非排除树件漏列 ⇒ 排除面过宽")
	}

	// 成对参照：这两件确在「未跟踪 ∩ 未被忽略」面里（`.gitignore` 没忽略 bin/ · dist/）
	// ⇒ 是**显式排除树**把它们剔掉的。
	raw := fsuGit(t, root, "ls-files", "--others", "--exclude-standard")
	if !strings.Contains(raw, "bin/big.bin") || !strings.Contains(raw, "dist/big.bin") {
		t.Fatalf("夹具前提破：bin/ 或 dist/ 下那件不在未跟踪 ∩ 未被忽略面里\n%s", raw)
	}
}

// ⑧ 体量降序 ＋ Top 裁。
func TestFsUnprotectedOrderAndTop(t *testing.T) {
	root := fsuRepo(t)
	fsuWriteN(t, filepath.Join(root, "small.bin"), 100)
	fsuWriteN(t, filepath.Join(root, "mid.bin"), 2000)
	fsuWriteN(t, filepath.Join(root, "large.bin"), 9000)
	res, _ := fsUnprotectedCore(fsUnprotectedOpts{Root: root, Top: 2})
	t.Logf("⑧ Top=2 · candidates=%d · truncated=%v · items=%v",
		res.Candidates, res.Truncated, fsUnprotectedRows(res))
	if res.Candidates != 3 {
		t.Fatalf("裁前件数=%d（期望 3）", res.Candidates)
	}
	if !res.Truncated || len(res.Items) != 2 {
		t.Fatalf("Top 裁不符：truncated=%v len=%d", res.Truncated, len(res.Items))
	}
	if res.Items[0].Path != "large.bin" || res.Items[1].Path != "mid.bin" {
		t.Fatalf("降序不符：%v", fsUnprotectedRows(res))
	}
}

// ⑨ 只读自证（夹具）：跑前 / 跑后「仓 ＋ 盘」指纹逐字同。
func TestFsUnprotectedReadOnlyFixture(t *testing.T) {
	root := fsuRepo(t)
	fsuWriteN(t, filepath.Join(root, "bundle", "big.bin"), 4096)
	fsuWriteN(t, filepath.Join(root, "secret.ign"), 8192)
	before := fsuFingerprint(t, root)
	_, _ = fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	after := fsuFingerprint(t, root)
	t.Logf("⑨ 只读自证 · 指纹逐字同=%v", before == after)
	if before != after {
		t.Fatalf("核心改动了夹具仓/盘 ⇒ ✗ 不是只读\n前:\n%s\n后:\n%s", before, after)
	}
}

// ⑩ 真仓只读档：env `FS_UNPROTECTED_REAL_ROOT` 给才跑（默认跳过 ⇒ CI/包测试不受影响）。
func TestFsUnprotectedRealRepoReadOnly(t *testing.T) {
	root := os.Getenv("FS_UNPROTECTED_REAL_ROOT")
	if root == "" {
		t.Skip("未设 FS_UNPROTECTED_REAL_ROOT ⇒ 跳过真仓只读档")
	}
	before := fsuFingerprint(t, root)
	res, rc := fsUnprotectedCore(fsUnprotectedOpts{Root: root})
	rows := fsUnprotectedRows(res)
	show := rows
	if len(show) > 8 {
		show = show[:8]
	}
	t.Logf("⑩ 真仓 · rc=%d · candidates=%d · truncated=%v · 前几件=%v",
		rc, res.Candidates, res.Truncated, show)
	after := fsuFingerprint(t, root)
	t.Logf("⑩ 只读自证（真仓）· 指纹逐字同=%v · HEAD=%s",
		before == after, strings.TrimSpace(fsuGit(t, root, "rev-parse", "HEAD")))
	if before != after {
		t.Fatalf("真仓被改动 ⇒ ✗ 不是只读（注：并发他路写入也可能触发 · 需人工分辨）")
	}
}

// ============================================================================
// ★ F44 —— `fs hidden-faces` 定向测试（同族同形 · 复用 F38 的夹具 helper）。
// 核心件未接命令树 ⇒ 读数由内测直调 `fsHiddenFacesCore` 取得。
//
//    态① 干净仓 ⇒ rc=0 ·「无隐藏面」；
//    态② 人造多形态（普通件 / 符号链接 / 目录 / 嵌套）⇒ **逐条点名**（含 kind）；
//    态③ 已忽略件（件 ＋ 目录）⇒ **必不列**（成对：确在 `ls-files --others` 面里）；
//    态④ 真源取不到（非 git 树）⇒ **rc=2 不给结论**；
//    负控① 哑化忽略判定 ⇒ 已忽略件**必红**（成对正控 = 默认必不列）；
//    负控② 已跟踪件必不列 / 删出索引 ⇒ 必列；
//    负控③ 排除树（bin/ · dist/）必不列 / 同件落 src/ ⇒ 必列；
//    只读自证（夹具 ＋ 真仓档）。
// ============================================================================

// fhfSymlink —— 在夹具仓造一枚符号链接（目标按**仓内相对**给）。
func fhfSymlink(t *testing.T, root, target, linkRel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, linkRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, linkRel)); err != nil {
		t.Fatal(err)
	}
}

// fhfHas —— 结论里有没有点名这条路径。
func fhfHas(res fsHiddenFacesResult, path string) bool {
	for _, it := range res.Items {
		if it.Path == path {
			return true
		}
	}
	return false
}

// fhfItem —— 取结论里某条（找不到 ⇒ 失败）。
func fhfItem(t *testing.T, res fsHiddenFacesResult, path string) fsUnprotectedItem {
	t.Helper()
	for _, it := range res.Items {
		if it.Path == path {
			return it
		}
	}
	t.Fatalf("结论里没有点名 %q", path)
	return fsUnprotectedItem{}
}

// fhfFingerprint —— 「仓 ＋ 盘」指纹（★ 照本单 SELFPROOF 口径：HEAD ＋ `.git/index` sha256 ＋
// 未跟踪清单 sha256 · **✗ 不用 `git status`** —— 它的计数会因并发会话漂移，不当自证）。
func fhfFingerprint(t *testing.T, root string) string {
	t.Helper()
	head := strings.TrimSpace(fsuGit(t, root, "rev-parse", "HEAD"))
	idx, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatalf("读 .git/index 失败: %v", err)
	}
	others := fsuGit(t, root, "ls-files", "--others", "-z")
	return "HEAD=" + head +
		"\nINDEX=" + fmt.Sprintf("%x", sha256.Sum256(idx)) +
		"\nOTHERS=" + fmt.Sprintf("%x", sha256.Sum256([]byte(others)))
}

// 态① 干净仓 ⇒ rc=0 ·「无隐藏面」。
func TestFsHiddenFacesCleanRepo(t *testing.T) {
	root := fsuRepo(t)
	res, rc := fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	var b strings.Builder
	fsHiddenFacesHuman(&b, res)
	n := len(res.Items)
	t.Logf("态① 干净仓 · rc=%d · items=%d · 人面=%q", rc, n, strings.TrimSpace(b.String()))
	if rc != exitOK {
		t.Fatalf("干净仓 rc=%d（期望 0）", rc)
	}
	if n != 0 {
		t.Fatalf("干净仓不该有隐藏面（实得 %d 条）", n)
	}
	if !strings.Contains(b.String(), "无隐藏面") {
		t.Fatalf("干净仓人面未报「无隐藏面」：%q", b.String())
	}
}

// 态② 人造多形态（普通件 / 符号链接 / 目录 / 嵌套）⇒ **逐条点名**（含 kind）。
func TestFsHiddenFacesListsShapes(t *testing.T) {
	root := fsuRepo(t)
	fsuWrite(t, filepath.Join(root, "plain.txt"), []byte("plain\n"))
	fsuWrite(t, filepath.Join(root, "newdir", "inner.txt"), []byte("inner\n"))
	fsuWrite(t, filepath.Join(root, "newdir", "deep", "x.txt"), []byte("deep\n"))
	if err := os.MkdirAll(filepath.Join(root, "emptydir"), 0o755); err != nil {
		t.Fatal(err)
	}
	fhfSymlink(t, root, "tracked.txt", "alink")

	res, rc := fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	n := len(res.Items)
	t.Logf("态② rc=%d · items=%d · rows=%v", rc, n, fsHiddenFacesRows(res))
	if rc != exitFail {
		t.Fatalf("有隐藏面 rc=%d（期望 1）", rc)
	}
	// 普通件
	if it := fhfItem(t, res, "plain.txt"); it.Kind != fsKindRegular {
		t.Fatalf("plain.txt kind=%q（期望 %q）", it.Kind, fsKindRegular)
	}
	// 符号链接
	if it := fhfItem(t, res, "alink"); it.Kind != fsKindSymlink {
		t.Fatalf("alink kind=%q（期望 %q）", it.Kind, fsKindSymlink)
	}
	// 目录（`--directory` 面）
	if it := fhfItem(t, res, "newdir/"); it.Kind != fsKindDir {
		t.Fatalf("newdir/ kind=%q（期望 %q）", it.Kind, fsKindDir)
	}
	if it := fhfItem(t, res, "emptydir/"); it.Kind != fsKindDir {
		t.Fatalf("emptydir/ kind=%q（期望 %q）", it.Kind, fsKindDir)
	}
	// 嵌套（✗ 不只首层）
	if it := fhfItem(t, res, "newdir/inner.txt"); it.Kind != fsKindRegular {
		t.Fatalf("newdir/inner.txt kind=%q（期望 %q）", it.Kind, fsKindRegular)
	}
	if it := fhfItem(t, res, "newdir/deep/x.txt"); it.Kind != fsKindRegular {
		t.Fatalf("newdir/deep/x.txt kind=%q（期望 %q）", it.Kind, fsKindRegular)
	}
}

// 态③ 已忽略件（件 ＋ 目录）⇒ **必不列**（成对：确在未跟踪面里）。
func TestFsHiddenFacesIgnoredNotListed(t *testing.T) {
	root := fsuRepo(t)
	fsuWrite(t, filepath.Join(root, "secret.ign"), []byte("secret\n"))
	fsuWrite(t, filepath.Join(root, "ignored-dir", "hidden.txt"), []byte("hidden\n"))
	// 成对参照：确认它们确在「未跟踪」面（`ls-files --others` 含已忽略件）。
	raw := fsuGit(t, root, "ls-files", "--others", "-z")
	if !strings.Contains(raw, "secret.ign") || !strings.Contains(raw, "ignored-dir/hidden.txt") {
		t.Fatalf("夹具前提破：已忽略件不在 `ls-files --others` 面里 ⇒ 忽略面无从判")
	}
	res, rc := fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	n := len(res.Items)
	t.Logf("态③ rc=%d · items=%d · rows=%v（secret.ign / ignored-dir 必不在此）", rc, n, fsHiddenFacesRows(res))
	if rc != exitOK {
		t.Fatalf("只剩已忽略件 rc=%d（期望 0 —— 判据跑通无条）", rc)
	}
	if fhfHas(res, "secret.ign") || fhfHas(res, "ignored-dir/hidden.txt") || fhfHas(res, "ignored-dir/") {
		t.Fatalf("已忽略件/目录被列出 ⇒ 忽略面没进判据：%v", fsHiddenFacesRows(res))
	}
}

// 态④ 真源取不到（非 git 树）⇒ rc=2 不给结论。
func TestFsHiddenFacesNoTruthSource(t *testing.T) {
	root := t.TempDir() // 非 git 树（没有 .git）
	res, rc := fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	n := len(res.Items)
	t.Logf("态④ 非 git 树 · rc=%d · items=%d", rc, n)
	if rc != exitUsage {
		t.Fatalf("非 git 树 rc=%d（期望 2 —— 不给结论 · ✗ 不假绿）", rc)
	}
	if n != 0 {
		t.Fatalf("不给结论时不该出示条（实得 %d 条）", n)
	}
}

// 负控① 哑化忽略判定 ⇒ 已忽略件必红（成对正控 = 默认判据必不列）。
func TestFsHiddenFacesNegIgnoreMuted(t *testing.T) {
	root := fsuRepo(t)
	fsuWrite(t, filepath.Join(root, "secret.ign"), []byte("secret\n"))

	// 正控：默认判据 ⇒ 必不列。
	res, _ := fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	if fhfHas(res, "secret.ign") {
		t.Fatalf("正控破：默认判据把已忽略件列了出来")
	}

	// 哑化「忽略判定」⇒ 同一件**必红**（证明这条牙真咬在忽略面上）。
	orig := fsUnprotIgnoreFn
	fsUnprotIgnoreFn = func(string, []string) (map[string]bool, bool) { return map[string]bool{}, true }
	defer func() { fsUnprotIgnoreFn = orig }()
	res2, _ := fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	n := len(res2.Items)
	t.Logf("负控① 哑化后 · items=%d · rows=%v", n, fsHiddenFacesRows(res2))
	if !fhfHas(res2, "secret.ign") {
		t.Fatalf("负控破：哑化忽略判定后，已忽略件仍未列出 ⇒ 这颗牙没牙（态③ 不会红）")
	}
}

// 负控② 已跟踪件必不列（「版本面无」这条真起作用）；成对 = 删出索引 ⇒ 必列。
func TestFsHiddenFacesNegTrackedNotListed(t *testing.T) {
	root := fsuRepo(t)
	fsuWrite(t, filepath.Join(root, "tracked-big.bin"), []byte("tracked\n"))
	fsuGit(t, root, "add", "tracked-big.bin")
	fsuGit(t, root, "commit", "-qm", "add tracked-big")

	res, _ := fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	t.Logf("负控② 已跟踪 · rows=%v", fsHiddenFacesRows(res))
	if fhfHas(res, "tracked-big.bin") {
		t.Fatalf("已跟踪件被当隐藏面列出 ⇒「版本面无」这条失效")
	}

	// 成对：同一件删出索引（变成未跟踪）⇒ 必列。
	fsuGit(t, root, "rm", "--cached", "tracked-big.bin")
	res2, _ := fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	t.Logf("负控② 成对 · 删出索引后 · rows=%v", fsHiddenFacesRows(res2))
	if !fhfHas(res2, "tracked-big.bin") {
		t.Fatalf("删出索引后仍未列出 ⇒「未跟踪」这条失效")
	}
}

// 负控③ 排除树（bin/ · dist/）必不列；成对 = 同一件落 src/ ⇒ 必列（排除面不过宽）。
func TestFsHiddenFacesNegExcludedDirs(t *testing.T) {
	root := fsuRepo(t)
	fsuWrite(t, filepath.Join(root, "bin", "big.txt"), []byte("b\n"))
	fsuWrite(t, filepath.Join(root, "dist", "big.txt"), []byte("d\n"))
	fsuWrite(t, filepath.Join(root, "src", "big.txt"), []byte("s\n"))

	res, _ := fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	t.Logf("负控③ rows=%v", fsHiddenFacesRows(res))
	if fhfHas(res, "bin/big.txt") || fhfHas(res, "dist/big.txt") ||
		fhfHas(res, "bin/") || fhfHas(res, "dist/") {
		t.Fatalf("排除树未生效：bin/ 或 dist/ 下的路径被列出")
	}
	if !fhfHas(res, "src/big.txt") {
		t.Fatalf("非排除树件漏列 ⇒ 排除面过宽")
	}

	// 成对参照：这两件确在「未跟踪 ∩ 未被忽略」面里（`.gitignore` 没忽略 bin/ · dist/）
	// ⇒ 是**显式排除树**把它们剔掉的。
	raw := fsuGit(t, root, "ls-files", "--others")
	if !strings.Contains(raw, "bin/big.txt") || !strings.Contains(raw, "dist/big.txt") {
		t.Fatalf("夹具前提破：bin/ 或 dist/ 下那件不在未跟踪 ∩ 未被忽略面里\n%s", raw)
	}
}

// 只读自证（夹具）：跑前 / 跑后「仓 ＋ 盘」指纹逐字同。
func TestFsHiddenFacesReadOnlyFixture(t *testing.T) {
	root := fsuRepo(t)
	fsuWrite(t, filepath.Join(root, "plain.txt"), []byte("plain\n"))
	fsuWrite(t, filepath.Join(root, "newdir", "inner.txt"), []byte("inner\n"))
	fhfSymlink(t, root, "tracked.txt", "alink")
	fsuWrite(t, filepath.Join(root, "secret.ign"), []byte("secret\n"))

	before := fhfFingerprint(t, root)
	_, _ = fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	after := fhfFingerprint(t, root)
	t.Logf("只读自证（夹具）· 指纹逐字同=%v", before == after)
	if before != after {
		t.Fatalf("核心改动了夹具仓/盘 ⇒ ✗ 不是只读\n前:\n%s\n后:\n%s", before, after)
	}
}

// 真仓只读档（env `FS_HIDDEN_FACES_REAL_ROOT` 给才跑 · 默认跳过）。
func TestFsHiddenFacesRealRepoReadOnly(t *testing.T) {
	root := os.Getenv("FS_HIDDEN_FACES_REAL_ROOT")
	if root == "" {
		t.Skip("未设 FS_HIDDEN_FACES_REAL_ROOT ⇒ 跳过真仓只读档")
	}
	before := fhfFingerprint(t, root)
	res, rc := fsHiddenFacesCore(fsHiddenFacesOpts{Root: root})
	n := len(res.Items)
	show := fsHiddenFacesRows(res)
	m := len(show)
	if m > 8 {
		show = show[:8]
	}
	t.Logf("真仓 · rc=%d · items=%d · 前几件=%v", rc, n, show)
	after := fhfFingerprint(t, root)
	t.Logf("只读自证（真仓）· 指纹逐字同=%v · HEAD=%s",
		before == after, strings.TrimSpace(fsuGit(t, root, "rev-parse", "HEAD")))
	if before != after {
		t.Fatalf("真仓被改动 ⇒ ✗ 不是只读（注：并发他路写入也可能触发 · 需人工分辨）")
	}
}
