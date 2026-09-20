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

// truncateDisplay 只为人面好看：超长行截断（**不改机器面已列出的原文之外的东西**）。
func truncateDisplay(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}
