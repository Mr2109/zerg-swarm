// family_code.go —— 码面只读一条（§九 I1 · 缺口-命令面-20260921 §十一 P0-1）。
//
// 为什么它排 P0 第一位：今天手搓的 **168 行里 `grep` / `git grep` 占 37 行（最大一类）**，
// 而命令面**零覆盖**；设计稿 §二 2.3 逐字「AI agent 不许绕过命令面直拼 curl/shell」——
// 「读码」正是每一次自开发的第一环。所以它不是「顺手加一个」，是**入口**。
//
// 口径（照 §十一 P0-1 的形态，不自造）：
//
//	形态 `zerg code find <正则> [--path <子目录>] [--glob <模式>] [--json <字段>]`
//	输出 `path/line/text` 逐条 + 命中数（人面表格 · 机器面六键包封）
//	退码 `0` 有命中 / `1` 零命中（**不是错**，是「没有」）/ `2` 用法错（正则坏）/ `8` 仓根/面读不到
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// codeScanSkipDirs —— 扫码排除表（与 §十七/§二十一 各处复跑命令**同一套**口径；
// 另把 `bin` 与对象仓排除掉：它们不是「码」，扫进去只会把命中数灌水）。
var codeScanSkipDirs = map[string]bool{
	"target": true, "node_modules": true, "dist": true, "bin": true, "vendor": true,
	"data": true, ".git": true, ".venv": true, "venv": true, ".build": true,
}

// codeFindRowCap —— 一页最多列多少条（**明说**上限，不静默截断）。
const codeFindRowCap = 200

// codeFindMaxFileBytes —— 单件超过这么大的不进扫描面（件不是码/会拖慢），并如实报出跳过数。
const codeFindMaxFileBytes = 2 << 20

func cmdCodeFind(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_pattern", "缺正则")
		fmt.Fprintf(stderr, "%s: `code find` 要给正则（例：zerg code find 'func cmdGate' --path core/cmd/zerg）\n", progName)
		fmt.Fprintf(stderr, "用法：zerg code find <正则> [--path <子目录>] [--glob <模式>] [--json <字段>]\n")
		return exitUsage
	}
	pat := inv.args[0]
	re, err := regexp.Compile(pat)
	if err != nil {
		inv.setErr("usage", "bad_regex", "正则编译不过")
		fmt.Fprintf(stderr, "%s: 正则编译不过：%v\n", progName, err)
		fmt.Fprintf(stderr, "下一步：换一个正则（例：把 `a(` 写成 `a\\(`）\n")
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 扫不了码（不给结论 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return exitBlocked
	}
	base := root
	if sub := strings.TrimSpace(inv.flagVal("--path")); sub != "" {
		base = filepath.Join(root, filepath.FromSlash(sub))
		if !strings.HasPrefix(base, root) {
			inv.setErr("usage", "path_outside_repo", "--path 出仓")
			fmt.Fprintf(stderr, "%s: --path 出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, sub, root)
			return exitUsage
		}
		if st, err := os.Stat(base); err != nil || !st.IsDir() {
			inv.setErr("usage", "path_not_dir", "--path 不是目录")
			fmt.Fprintf(stderr, "%s: --path 指的不是目录：%s（先 ls 确认目录名 · 退码 2）\n", progName, sub)
			return exitUsage
		}
	}
	glob := strings.TrimSpace(inv.flagVal("--glob"))

	rows := []map[string]string{}
	hits, scanned, skippedBig := 0, 0, 0
	walkErr := filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 读不动的子树跳过（只读面不许因为一件读不到就整命令失败）
		}
		if info.IsDir() {
			if codeScanSkipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if glob != "" {
			if ok, _ := filepath.Match(glob, filepath.Base(p)); !ok {
				return nil
			}
		}
		if info.Size() > codeFindMaxFileBytes {
			skippedBig++
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil || !utf8.Valid(b) {
			return nil // 二进制/读不到：不进码面（照实跳过，不猜）
		}
		scanned++
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)
		for i, ln := range strings.Split(string(b), "\n") {
			if !re.MatchString(ln) {
				continue
			}
			hits++
			if len(rows) < codeFindRowCap {
				rows = append(rows, map[string]string{
					"path": rel,
					"line": strconv.Itoa(i + 1),
					"text": truncateDisplay(strings.TrimRight(ln, "\r"), 200),
				})
			}
		}
		return nil
	})
	if walkErr != nil {
		inv.setErr("blocked", "walk_failed", "扫不动")
		fmt.Fprintf(stderr, "%s: 扫不动（%v）⇒ 不给结论（退码 8）\n", progName, walkErr)
		return exitBlocked
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i]["path"] != rows[j]["path"] {
			return rows[i]["path"] < rows[j]["path"]
		}
		return atoiSafe(rows[i]["line"]) < atoiSafe(rows[j]["line"])
	})
	fmt.Fprintf(stderr, "%s: 扫了 %d 件 · 命中 %d 条", progName, scanned, hits)
	if len(rows) < hits {
		fmt.Fprintf(stderr, "（本页只列前 %d 条 —— 收窄：--path / --glob）", len(rows))
	}
	if skippedBig > 0 {
		fmt.Fprintf(stderr, " · 跳过 >%dMB 的件 %d 个", codeFindMaxFileBytes>>20, skippedBig)
	}
	fmt.Fprintln(stderr)
	if hits == 0 {
		inv.setErr("failed", "no_match", "零命中")
		fmt.Fprintf(stderr, "%s: 零命中 —— 这不是错，是「没有」（退码 1）；要当错用得自己判\n", progName)
		return exitFail
	}
	return listCmd(inv, stdout, stderr, []string{"path", "line", "text"}, rows)
}

// cmdCodeShow —— `zerg code show <件:行>`：读出源码里**某一行的邻域**（只读 · 与 `code find` 同族）。
//
// 为什么与 `find` 同族却单列一条：`find` 答「**在哪**」，`show` 答「**那处长什么样**」——
// 每一次自开发都是「先找、再看」两问；两问都靠手搓 `sed -n` / `awk` 顶着，就没有命令面可言
// （设计稿 §二 2.3 逐字「AI agent 不许绕过命令面直拼 curl/shell」）。
//
// 口径（**照 `code find` 的形态与退码族，不自造**）：
//
//	形态 `zerg code show <件:行> [--ctx <N>] [--json <字段>]`
//	输出 `path/line/text/target` 逐条（`target=yes` 标出被点的那一行 · 机器面六键包封）
//	退码 `0` 出邻域 · `1` 件不在（**不是错**，是「没有」）· `2` 用法错
//	     （缺参 / 没冒号 / 行号非正整数 / 行号越界 / 件出仓 / 点的是目录 / `--ctx` 坏）
//	     · `8` 仓根解析不到 · 件读不动或不是文本（**不给结论**）
//
// ★ 行号越界判 `2`（不是 `1`）：`1` 在本族里的语义是「**真没有**」，而越界是**你点的位置不对**
//
//	—— 用法错（任务单 `波7` 序71 的判据逐字：「负控：行号越界 ⇒ rc=2」）。
func cmdCodeShow(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_target", "缺 <件:行>")
		fmt.Fprintf(stderr, "%s: `code show` 要给 <件:行>（例：zerg code show core/cmd/zerg/main.go:100）\n", progName)
		fmt.Fprintf(stderr, "用法：zerg code show <件:行> [--ctx <N>] [--json <字段>]\n")
		return exitUsage
	}
	target := strings.TrimSpace(inv.args[0])
	colon := strings.LastIndex(target, ":")
	if colon <= 0 || colon == len(target)-1 {
		inv.setErr("usage", "bad_target", "目标不是 <件:行>")
		fmt.Fprintf(stderr, "%s: 目标要写成 <件:行>（没冒号或没给行号）：%s ⇒ 退码 2\n", progName, target)
		fmt.Fprintf(stderr, "例：zerg code show core/go.mod:12\n")
		return exitUsage
	}
	rel, lineStr := target[:colon], target[colon+1:]
	lineNo, aerr := strconv.Atoi(lineStr)
	if aerr != nil || lineNo < 1 {
		inv.setErr("usage", "bad_line", "行号不是正整数")
		fmt.Fprintf(stderr, "%s: 行号要是正整数：%q ⇒ 退码 2\n", progName, lineStr)
		return exitUsage
	}
	ctx := codeShowDefaultCtx
	if s := strings.TrimSpace(inv.flagVal("--ctx")); s != "" {
		n, cerr := strconv.Atoi(s)
		if cerr != nil || n < 0 {
			inv.setErr("usage", "bad_ctx", "--ctx 不是非负整数")
			fmt.Fprintf(stderr, "%s: --ctx 要是非负整数（0 = 只看那一行）：%q ⇒ 退码 2\n", progName, s)
			return exitUsage
		}
		ctx = n
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不了码（不给结论 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return exitBlocked
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		inv.setErr("usage", "path_outside_repo", "件出仓")
		fmt.Fprintf(stderr, "%s: 件出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, rel, root)
		return exitUsage
	}
	abs := filepath.Join(root, clean)
	if !strings.HasPrefix(abs, root) {
		inv.setErr("usage", "path_outside_repo", "件出仓")
		fmt.Fprintf(stderr, "%s: 件出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, rel, root)
		return exitUsage
	}
	st, serr := os.Stat(abs)
	if serr != nil {
		inv.setErr("failed", "file_absent", "件不在")
		fmt.Fprintf(stderr, "%s: 件不在：%s —— **这不是错，是「没有」**（退码 1）\n", progName, rel)
		return exitFail
	}
	if st.IsDir() {
		inv.setErr("usage", "target_is_dir", "点的是目录")
		fmt.Fprintf(stderr, "%s: 点的是目录、不是件：%s（先 `code find` 定位到件 · 退码 2）\n", progName, rel)
		return exitUsage
	}
	b, rerr := os.ReadFile(abs)
	if rerr != nil {
		inv.setErr("blocked", "file_unreadable", "件读不动")
		fmt.Fprintf(stderr, "%s: 件读不动（%v）⇒ 不给结论（退码 8）\n", progName, rerr)
		return exitBlocked
	}
	if !utf8.Valid(b) {
		inv.setErr("blocked", "not_text", "件不是文本")
		fmt.Fprintf(stderr, "%s: 件不是文本（非 UTF-8 ⇒ 不猜它的行号）⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	// 行面口径与 `code find` 同一套（`strings.Split(…, "\n")` · 行号从 1 起）；
	// 另把**结尾那个空段**削掉（"a\nb\n" ⇒ 两行，不是三行）—— 只为人面「共 N 行」说得准。
	lines := strings.Split(string(b), "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if lineNo > len(lines) {
		inv.setErr("usage", "line_out_of_range", "行号越界")
		fmt.Fprintf(stderr, "%s: 行号越界：%s 共 %d 行，点了第 %d 行 ⇒ 退码 2\n", progName, rel, len(lines), lineNo)
		fmt.Fprintf(stderr, "下一步：先 `zerg code find <正则>` 拿到真行号\n")
		return exitUsage
	}
	lo, hi := lineNo-ctx, lineNo+ctx
	if lo < 1 {
		lo = 1
	}
	if hi > len(lines) {
		hi = len(lines)
	}
	nrel := filepath.ToSlash(clean)
	rows := make([]map[string]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		mark := "no"
		if i == lineNo {
			mark = "yes"
		}
		rows = append(rows, map[string]string{
			"path":   nrel,
			"line":   strconv.Itoa(i),
			"text":   truncateDisplay(strings.TrimRight(lines[i-1], "\r"), 200),
			"target": mark,
		})
	}
	fmt.Fprintf(stderr, "%s: %s 第 %d 行（共 %d 行 · 窗口 ±%d）\n", progName, nrel, lineNo, len(lines), ctx)
	return listCmd(inv, stdout, stderr, []string{"line", "text"}, rows)
}

// codeShowDefaultCtx —— `code show` 默认窗口半径（±3 行 = 「一眼看得到上下文」的最小面）。
const codeShowDefaultCtx = 3

// truncateDisplay 只为人面好看：超长行截断（**不改机器面已列出的原文之外的东西**）。
func truncateDisplay(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}
