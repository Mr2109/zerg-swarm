// family_fs.go —— `zerg fs` 族的**判据核心**（F38 · 半前置）：`fs unprotected`。
//
// ★ 本件**只落判据核心**，✗ **不接命令树**（`core/cmd/zerg/main.go` 本单禁碰 —— F30 在制）⇒
// 现读进不了 `zerg` 命令面；四态读数由同目录 `family_fs_test.go` 直调 `fsUnprotectedCore` 取得。
//
// 出处（✗ 不新造口径 · 逐字取账面 want）：
//
//	· `GAP-20260927-223`（P0 · ③ 长线）want 逐字：「补命令面 `zerg fs unprotected`：一条列出
//	  「工作树里未被跟踪、未被忽略、且**体积>阈值**的件」，让这种漏网再也藏不住。」
//	· `GAP-20260927-224`（P1 · ③ 长线）want 逐字：「`zerg fs hidden-faces` —— 一条列出「**盘上有 /
//	  版本面无 / 未被忽略**」的路径差，替掉手搓 `ls -a` + `git ls-files` + `check-ignore`。」
//	· `GAP-20260927-225`（P1 · ③ 长线）want 逐字：「`zerg fs unprotected` 覆盖这类「被门点名但版本面无」的件。」
//
// 用户已拍（2026-10-15 · 逐字 · 账内 `GAP-20260927-225` 口径注 [5][6][7]）：
//
//	① 乙档＝加整族；族名取 `fs`；子动作照账面点名 `unprotected` / `hidden-faces`；起步只落 `unprotected`。
//	② (a) 体量面取「降序前 **50** 件」而非固定字节阈值；覆盖树＝**全仓**（✗ 不限 `scripts`），
//	   显式排除 `bin/` · `dist/` · `.git/` 与**已忽略面**；
//	③ (b) 分工：`zerg repo status` 管「本仓面现状」（七栏不动），`zerg fs` 管「盘 ⟷ 版本面的差」
//	   （含被忽略面 ＋ 体量排行）。
//
// # 判据（三类条件 ∩ ＋ 体量口径 · 真源全部是 git 自己那一面）
//
//	盘上有   —— `git ls-files --others`（未跟踪 ∩ 盘上在）
//	版本面无 —— 同上（不在索引 / 版本面）· ★ 未跟踪即「版本面无」· ✗ 不自己比 mtime/sha
//	未被忽略 —— `git check-ignore` **不命中**（★ 忽略面真进判据 · ✗ 不出现在结果里）
//	体量     —— 逐件 `lstat` 取**字节** ＋ **类型**，按字节**降序取前 50**
//	覆盖树   —— 全仓；**排除**首段为 `bin` / `dist` / `.git` 的路径
//
// ★ 硬纪律（`main.go:1767-1769` 逐字）：判「哪些件脏」的真源**仍是 git porcelain** ⇒ 本件
// **✗ 不造第二套判脏口径**：只**读** git 自己的面（`ls-files --others` = 未跟踪真源 ·
// `check-ignore` = 忽略真源）再加工。★ 复用既有位点：`repo status` 的 `untracked` 栏（同读法 ·
// `family_repo.go`）· `check_wired_scripts.go` 的 `git ls-files -o --exclude-standard`（同真源）。
//
// # 退码（判据三态）
//
//	0 —— 判据跑通 · **无**未保护件
//	1 —— 判据跑通 · **有**未保护件（逐件点名）
//	2 —— **不给结论**（仓根解析不到 / 不是 git 工作树 / git 真源取不到）—— ✗ 不假绿
//
// ★ 退码 2 与 `repo status`（那里读不到 ⇒ `exitBlocked` 8）**不同档**：本件是**判据核心**，
// 照本仓既有 `exitUsage = 2 // 用法错 / 不给结论` 的口径用 2；命令面接入时取哪一档**待拍**（待定项）。
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// fsUnprotectedDefaultTop —— 体量面降序取前 N（用户 2026-10-15 拍定 **50**）。
const fsUnprotectedDefaultTop = 50

// fsUnprotectedExcludeDirs —— 显式排除的首段目录（用户拍定 `bin/` · `dist/` · `.git/`）。
//
// ★ `.git/` 本就不会出现在 `git ls-files --others` 里（git 自身跳过）⇒ 留它是**显式**声明；
// `bin/` · `dist/` 现读已被本仓 `.gitignore` 命中 ⇒ 排除树与忽略面**部分重叠**，两处都留着
// 是「显式点名」＋「忽略面兜底」双保险（口径面即用户拍的那句）。
var fsUnprotectedExcludeDirs = []string{"bin", "dist", ".git"}

// 件类型（人面 / 机器面共用一份 · ✗ 不在第二处再判一次）。
const (
	fsKindRegular    = "普通件"
	fsKindSymlink    = "符号链接"
	fsKindDir        = "目录"
	fsKindUnreadable = "（读不到）"
)

// fsUnprotectedItem —— 一件「未保护件」：盘上有 · 版本面无 · 未被忽略 · 带体量。
type fsUnprotectedItem struct {
	Path  string // 仓根相对路径
	Bytes int64  // 字节（`lstat` · 符号链接为该链接自身的字节）
	Kind  string // 类型：普通件 / 符号链接 / 目录 / （读不到）
}

// fsUnprotectedOpts —— 判据核心入参。
//
// ★ CLI 旗标命名与默认值**尚未拍**（F38 待定项）⇒ 本结构只供核心内部与测试用，本件**不落**任何旗标名。
type fsUnprotectedOpts struct {
	Root string // 仓根（空 ⇒ 走 `repoRoot()`）；★ 测试恒显式给，✗ 不碰真仓
	Top  int    // 体量降序取前 N（<=0 ⇒ `fsUnprotectedDefaultTop`）
}

// fsUnprotectedResult —— 判据核心结论。
type fsUnprotectedResult struct {
	Root       string              // 生效的仓根
	Candidates int                 // 三条条件 ∩ 后（裁 Top 前）的件数
	Items      []fsUnprotectedItem // 已按字节降序 · 裁到 Top
	Truncated  bool                // 是否被 Top 裁过
}

// fsUnprotIgnoreFn —— 忽略判定的调用缝（默认指向 `fsUnprotCheckIgnore`）。
//
// ★ 仅供测试**哑化**忽略判定，演示负控「哑化忽略判定 ⇒ 态③ 必红」；生产路径恒走 `fsUnprotCheckIgnore`。
var fsUnprotIgnoreFn = fsUnprotCheckIgnore

// fsUnprotectedCore —— `fs unprotected` 判据核心（★ ✗ 不接命令树 · **只读** · 零写盘）。
//
// 返回 (结论, rc)：rc ∈ {`exitOK` 无件 · `exitFail` 有件 · `exitUsage` 不给结论}。
func fsUnprotectedCore(opts fsUnprotectedOpts) (fsUnprotectedResult, int) {
	res := fsUnprotectedResult{}
	root := opts.Root
	if root == "" {
		root = repoRoot()
	}
	if root == "" {
		return res, exitUsage // 2：解析不到仓根 ⇒ 不给结论
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return res, exitUsage // 2：不是 git 工作树 ⇒ 不给结论
	}
	res.Root = root

	// ① 「版本面无」：真源 = git 自己的索引面（`--others` 含**已忽略**件 ⇒ 留给 ③ 剔）。
	out, ok := fsUnprotGitOut(root, []string{"ls-files", "--others", "-z"}, "")
	if !ok {
		return res, exitUsage // 2：真源取不到 ⇒ 不给结论（✗ 不假绿）
	}
	untracked := fsUnprotSplitNUL(out)

	// ② 显式排除树（`bin/` · `dist/` · `.git/`）：首段命中即剔。
	keep := make([]string, 0, len(untracked))
	for _, p := range untracked {
		if fsUnprotFirstSegExcluded(p) {
			continue
		}
		keep = append(keep, p)
	}

	// ③ 「未被忽略」：`git check-ignore` 命中即剔（★ 忽略面真进判据）。
	ignored, ok := fsUnprotIgnoreFn(root, keep)
	if !ok {
		return res, exitUsage // 2：忽略真源取不到 ⇒ 不给结论
	}
	kept := make([]string, 0, len(keep))
	for _, p := range keep {
		if ignored[p] {
			continue
		}
		kept = append(kept, p)
	}

	// ④ 体量：逐件 `lstat` 取字节 ＋ 类型，按字节**降序**（同字节按路径定序 ⇒ 稳定可复现）。
	items := make([]fsUnprotectedItem, 0, len(kept))
	for _, rel := range kept {
		items = append(items, fsUnprotStat(root, rel))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Bytes != items[j].Bytes {
			return items[i].Bytes > items[j].Bytes
		}
		return items[i].Path < items[j].Path
	})
	res.Candidates = len(items)

	top := opts.Top
	if top <= 0 {
		top = fsUnprotectedDefaultTop
	}
	if len(items) > top {
		res.Truncated = true
		items = items[:top]
	}
	res.Items = items

	if len(res.Items) == 0 {
		return res, exitOK // 0：无未保护件
	}
	return res, exitFail // 1：有未保护件（逐件点名）
}

// fsUnprotGitOut —— 只读跑一条 git 命令取 stdout。
//
// ★ 退出码 **1 也算有效读数**（`check-ignore` 无命中 = 1，不是错）⇒ 不拿它当失败。
// 其它非 0（git 缺 / 不是仓 / 用法错）⇒ `ok=false`（✗ 不当绿）；`git` 可执行件找不到同此。
func fsUnprotGitOut(root string, args []string, stdin string) (string, bool) {
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "core.quotepath=false"}, args...)...)
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return out.String(), true // 1 = `check-ignore` 无命中（有效读数）
		}
		return "", false
	}
	return out.String(), true
}

// fsUnprotCheckIgnore —— 对一批候选件判「是否被 `.gitignore` 命中」。
//
// 真源 = `git check-ignore --stdin -z`（NUL 分隔入 / 出 · ✗ 不自己写 `.gitignore` 匹配器）。
// 命中集 = 被忽略者；空命中时 git 退 1 ⇒ `fsUnprotGitOut` 按有效读数收。
func fsUnprotCheckIgnore(root string, rels []string) (map[string]bool, bool) {
	m := map[string]bool{}
	if len(rels) == 0 {
		return m, true
	}
	out, ok := fsUnprotGitOut(root, []string{"check-ignore", "--stdin", "-z"}, strings.Join(rels, "\x00")+"\x00")
	if !ok {
		return nil, false
	}
	for _, p := range fsUnprotSplitNUL(out) {
		m[p] = true
	}
	return m, true
}

// fsUnprotSplitNUL —— 把 `-z` 输出的 NUL 分隔串切成非空项。
func fsUnprotSplitNUL(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "\x00") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// fsUnprotFirstSegExcluded —— 路径首段是否落在排除树里（`bin/` · `dist/` · `.git/`）。
func fsUnprotFirstSegExcluded(rel string) bool {
	seg := rel
	if i := strings.IndexByte(rel, '/'); i >= 0 {
		seg = rel[:i]
	}
	for _, e := range fsUnprotectedExcludeDirs {
		if seg == e {
			return true
		}
	}
	return false
}

// fsUnprotStat —— 取一件的字节数与类型（`lstat` · ✗ 不跟符号链接）。
//
// ★ 读不到 ⇒ 照实标 `fsKindUnreadable` ＋ 字节 -1（✗ 不拿 0 顶替 · ✗ 不静默丢件）。
func fsUnprotStat(root, rel string) fsUnprotectedItem {
	it := fsUnprotectedItem{Path: rel, Bytes: -1, Kind: fsKindUnreadable}
	fi, err := os.Lstat(filepath.Join(root, rel))
	if err != nil {
		return it
	}
	it.Bytes = fi.Size()
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		it.Kind = fsKindSymlink
	case fi.IsDir():
		it.Kind = fsKindDir
	case fi.Mode().IsRegular():
		it.Kind = fsKindRegular
	default:
		it.Kind = fi.Mode().Type().String()
	}
	return it
}

// fsUnprotectedRows —— 机器面行（路径 ＋ 字节 ＋ 类型）。
func fsUnprotectedRows(res fsUnprotectedResult) []map[string]string {
	rows := make([]map[string]string, 0, len(res.Items))
	for _, it := range res.Items {
		rows = append(rows, map[string]string{
			"path":  it.Path,
			"bytes": strconv.FormatInt(it.Bytes, 10),
			"kind":  it.Kind,
		})
	}
	return rows
}

// fsUnprotectedHuman —— 人面读数（★ 输出形**待拍** ⇒ 此处只是可供核对的直读形态）。
func fsUnprotectedHuman(w io.Writer, res fsUnprotectedResult) {
	if len(res.Items) == 0 {
		fmt.Fprintf(w, "无未保护件（盘上有 / 版本面无 / 未被忽略 · 体量降序前 %d）\n", fsUnprotectedDefaultTop)
		return
	}
	fmt.Fprintf(w, "未保护件 %d 件（体量降序 · 裁前 %d · 取前 %d）\n",
		len(res.Items), res.Candidates, fsUnprotectedDefaultTop)
	for _, it := range res.Items {
		fmt.Fprintf(w, "  %s\t%d 字节\t%s\n", it.Path, it.Bytes, it.Kind)
	}
}

// ============================================================================
// ★ F44 —— `fs hidden-faces` 判据核心（同族同形 · ✗ 不接命令树），与 `fs unprotected` 同件。
// ============================================================================
//
// 出处（✗ 不新造口径 · 逐字取账面 want）：`GAP-20260927-224`（P1 · ③ 长线）逐字——
// 「`zerg fs hidden-faces` —— 一条列出「**盘上有 / 版本面无 / 未被忽略**」的路径差，
//   替掉手搓 `ls -a` + `git ls-files` + `check-ignore`。」
//
// 判据（三类条件 ∩ ＋ **形态面** · 真源全部是 git 自己那一面）
//
//    盘上有   —— `git ls-files --others`（未跟踪 ∩ 盘上在 · **递归** ⇒ 含**嵌套**路径）
//    版本面无 —— 同上（不在索引 / 版本面）· ★ 未跟踪即「版本面无」· ✗ 不自己比 mtime/sha
//    未被忽略 —— `git check-ignore` **不命中**
//    形态面   —— ★ 含**目录** / 符号链接 / 其它（不是只出普通件）· 真源 ＝ git 自己的
//                `git ls-files --others --directory`（★ 见下「两读并集」）
//    覆盖树   —— 全仓；**排除**首段为 `bin` / `dist` / `.git` 的路径（照 `fs unprotected` 同规）
//
// ★ 与 `fs unprotected`（F38）的判据差（逐条 · ✗ 不重复造）
//
//    · **体量面**：`unprotected` 有（逐件 `lstat` 取字节 ⇒ **按字节降序取前 50** ＋ `Truncated` 档）；
//      `hidden-faces` **无体量面** —— 只报**路径差本身**，✗ 不按字节裁、✗ 不排序体量（按**路径**定序）。
//    · **形态面**：`unprotected` 只覆盖 `--others` 能出的**件**；`hidden-faces` **并出目录**
//      （`--directory`）⇒ 覆盖「**非普通件**形态面（目录 / 符号链接 / 嵌套）」。
//    · **元信息**：`hidden-faces` 逐件 `lstat` 取**字节**只作**元信息**（✗ 不作排序 / 裁剪依据）；
//      `unprotected` 的字节是**体量判据**（降序 ＋ Top）。
//    · **同**（照 F38 不另造）：真源同（`ls-files --others` = 未跟踪真源 · `check-ignore` = 忽略真源）·
//      排除树同 · 忽略判定缝同（共用 `fsUnprotIgnoreFn` · ✗ 不另造第二套忽略口径）· 退码同（0/1/2）。
//
// ★ 两读并集（为什么读两条 `ls-files --others`）
//
//    · `--others`（**递归**）：出**件**（含嵌套路径、含符号链接）—— 但**默认不出目录**；
//    · `--others --directory`：出**目录**（整目录未被跟踪时收成一条 `dir/` · **空目录**也只在这里出）
//      —— 但它把整目录**收成一条 ⇒ 丢嵌套**。
//    ⇒ 只读其一都不满足 want 逐字「路径差本身（含目录 / 符号链接 / 嵌套）」⇒ 取**并集**（仍是 git 自己的输出面）。
//
// 退码（判据三态 · 与 `fs unprotected` 同规）
//
//    0 —— 判据跑通 · **无**路径差
//    1 —— 判据跑通 · **有**路径差（逐条点名）
//    2 —— **不给结论**（仓根解析不到 / 不是 git 工作树 / git 真源取不到）—— ✗ 不假绿
//
// ★ ✗ 不接命令树：`main.go` 一字未碰 ⇒ 本单**不落**任何 CLI 旗标名（旗标命名与默认值待定，见报告）。

// fsHiddenFacesOpts —— `fs hidden-faces` 判据核心入参。
//
// ★ 与 `fsUnprotectedOpts` 不同：**✗ 无 `Top`**（本判据**无体量面**）。
// ★ CLI 旗标命名与默认值**尚未拍**（本单待定项）⇒ 本结构只供核心内部与测试用，本件**不落**任何旗标名。
type fsHiddenFacesOpts struct {
	Root string // 仓根（空 ⇒ 走 `repoRoot()`）；★ 测试恒显式给，✗ 不碰真仓
}

// fsHiddenFacesResult —— 判据核心结论（路径差本身）。
//
// ★ 复用 `fsUnprotectedItem`（路径 ＋ 类型 ＋ 字节）：`Kind` 是**必需**输出（形态面），
// `Bytes` 只作**元信息**（✗ 本判据不用它排序 / 裁剪）⇒ 同族同形，✗ 不另造第二套件类型。
type fsHiddenFacesResult struct {
	Root  string              // 生效的仓根
	Items []fsUnprotectedItem // 已按**路径**定序（✗ 不按体量）
}

// fsHiddenFacesCore —— `fs hidden-faces` 判据核心（★ ✗ 不接命令树 · **只读** · 零写盘）。
//
// 返回 (结论, rc)：rc ∈ {`exitOK` 无路径差 · `exitFail` 有 · `exitUsage` 不给结论}。
// 忽略判定共用同族那枚缝 `fsUnprotIgnoreFn`（★ ✗ 不另造第二套忽略口径 · 负控就哑化它）。
func fsHiddenFacesCore(opts fsHiddenFacesOpts) (fsHiddenFacesResult, int) {
	res := fsHiddenFacesResult{}
	root := opts.Root
	if root == "" {
		root = repoRoot()
	}
	if root == "" {
		return res, exitUsage // 2：解析不到仓根 ⇒ 不给结论
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return res, exitUsage // 2：不是 git 工作树 ⇒ 不给结论
	}
	res.Root = root

	// ① 「版本面无」：真源 = git 自己的路径面。★ 读**两枚**（都是 git 自己的输出 ⇒ ✗ 不造第二套口径）：
	//     · `ls-files --others`             —— 递归出**件**（含嵌套 · 含符号链接）
	//     · `ls-files --others --directory` —— 出**目录**（整目录未被跟踪 ⇒ 收成一条 `dir/`；空目录在此出）
	//   取并集 ⇒ 才是 want 逐字要的「路径差本身（含目录 / 符号链接 / 嵌套）」。
	filesOut, ok := fsUnprotGitOut(root, []string{"ls-files", "--others", "-z"}, "")
	if !ok {
		return res, exitUsage // 2：真源取不到 ⇒ 不给结论（✗ 不假绿）
	}
	dirsOut, ok := fsUnprotGitOut(root, []string{"ls-files", "--others", "--directory", "-z"}, "")
	if !ok {
		return res, exitUsage // 2：真源取不到 ⇒ 不给结论（✗ 不假绿）
	}

	// ② 并集 ＋ 显式排除树（首段 `bin` / `dist` / `.git` ⇒ 命中即剔 · 目录形态照样判）。
	seen := map[string]bool{}
	cands := make([]string, 0, 16)
	for _, p := range append(fsUnprotSplitNUL(filesOut), fsUnprotSplitNUL(dirsOut)...) {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if fsUnprotFirstSegExcluded(p) {
			continue
		}
		cands = append(cands, p)
	}

	// ③ 「未被忽略」：`git check-ignore` 命中即剔（★ 忽略面真进判据 · 目录形态也同判）。
	ignored, ok := fsUnprotIgnoreFn(root, cands)
	if !ok {
		return res, exitUsage // 2：忽略真源取不到 ⇒ 不给结论
	}

	// ④ 形态面：逐件 `lstat` 取类型（目录 / 符号链接 / 普通件 / 其它）· 字节只作元信息。
	items := make([]fsUnprotectedItem, 0, len(cands))
	for _, rel := range cands {
		if ignored[rel] {
			continue
		}
		items = append(items, fsUnprotStat(root, rel))
	}
	// ★ 按**路径**定序（✗ 不引入体量维度 —— 那是 `fs unprotected` 的口径）。
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	res.Items = items

	if len(res.Items) == 0 {
		return res, exitOK // 0：无路径差
	}
	return res, exitFail // 1：有路径差（逐条点名）
}

// fsHiddenFacesRows —— 机器面行（路径 ＋ 类型 ＋ 字节元信息）。
func fsHiddenFacesRows(res fsHiddenFacesResult) []map[string]string {
	rows := make([]map[string]string, 0, len(res.Items))
	for _, it := range res.Items {
		rows = append(rows, map[string]string{
			"path":  it.Path,
			"kind":  it.Kind,
			"bytes": strconv.FormatInt(it.Bytes, 10),
		})
	}
	return rows
}

// fsHiddenFacesHuman —— 人面读数（★ 输出形**待拍** ⇒ 此处只是可供核对的直读形态）。
func fsHiddenFacesHuman(w io.Writer, res fsHiddenFacesResult) {
	if len(res.Items) == 0 {
		fmt.Fprintln(w, "无隐藏面（盘上有 / 版本面无 / 未被忽略 · 全仓路径差）")
		return
	}
	fmt.Fprintf(w, "隐藏面 %d 条（盘上有 / 版本面无 / 未被忽略 · 全仓路径差）\n", len(res.Items))
	for _, it := range res.Items {
		fmt.Fprintf(w, "  %s\t%s\n", it.Path, it.Kind)
	}
}
