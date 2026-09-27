// family_find.go —— `zerg find <名字片段>`：按**文件名片段**在仓（或指定根）下找件。
//
// 出处（缺口 序138 · GAP-13）：今天「查文件名只能 `find` 全仓递归，跑满超时」—— 本命令
// 走**受控遍历**（不是裸 `find .`）：① 显式上限（`--limit`，默认 200，明说上限、不静默截断）；
// ② 排除表（`findSkipDirs`）跳过 `bin` / `vendor` / `.git` 一类非件目录；③ 按**路径段**判出仓，
// 不靠字符串前缀蒙混。输出**相对路径 + 大小 + 修改时间**（契约要的那三列）。
//
// 口径（不自造，照 `code find` 的形态与退码族）：
//
//	形态 `zerg find <名字片段> [--type file|dir] [--root <根>] [--limit N] [--json <字段>]`
//	输出 `relpath · size · mtime` 逐条
//	退码 `0` 有命中 / `1` 零命中（**不是错**，是「没有」）/ `2` 用法错（类型坏 / 根读不到）/
//	`8` 根读不到
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// findSkipDirs —— `find` 排除表（与 `codeScanSkipDirs` 同一套口径）。
var findSkipDirs = map[string]bool{
	"target": true, "node_modules": true, "dist": true, "bin": true, "vendor": true,
	".git": true, ".venv": true, "venv": true, ".build": true, "data": true,
}

// findRowCap —— `find` 一页最多列多少件（明说上限，不静默截断）。
const findRowCap = 200

// cmdFind —— `zerg find <名字片段>` 的实现（按文件名命中，受控遍历）。
func cmdFind(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_pattern", "缺名字片段")
		fmt.Fprintf(stderr, "%s: `find` 要给名字片段（例：zerg find version.go --type file）\n", progName)
		fmt.Fprintf(stderr, "用法：zerg find <名字片段> [--type file|dir] [--root <根>] [--limit N] [--json <字段>]\n")
		return exitUsage
	}
	pat := strings.ToLower(inv.args[0])
	if pat == "" {
		inv.setErr("usage", "empty_pattern", "空名字片段")
		fmt.Fprintf(stderr, "%s: 名字片段不能为空（退码 2）\n", progName)
		return exitUsage
	}

	// 根：默认仓根；--root 可显式指向另一根。
	root := ""
	if sub := strings.TrimSpace(inv.flagVal("--root")); sub != "" {
		root = filepath.FromSlash(sub)
		if !filepath.IsAbs(root) {
			// 相对根：拼到当前工作目录下（不是仓根下——`find` 的根可以是任意目录）。
			cwd, err := os.Getwd()
			if err != nil {
				inv.setErr("blocked", "cwd_absent", "读不到当前目录")
				fmt.Fprintf(stderr, "%s: 读不到当前目录（%v）⇒ 退码 8\n", progName, err)
				return exitBlocked
			}
			root = filepath.Join(cwd, root)
		}
	} else {
		root = repoRoot()
	}
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根（也没给 --root）⇒ 扫不了（不给结论 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 --root <绝对路径>\n")
		return exitBlocked
	}
	// 根必须是目录。
	st, err := os.Stat(root)
	if err != nil {
		inv.setErr("usage", "root_stat_failed", "根读不到")
		fmt.Fprintf(stderr, "%s: 根读不到：%s（%v）⇒ 退码 2\n", progName, root, err)
		return exitUsage
	}
	if !st.IsDir() {
		inv.setErr("usage", "root_not_dir", "根不是目录")
		fmt.Fprintf(stderr, "%s: --root 指的不是目录：%s ⇒ 退码 2\n", progName, root)
		return exitUsage
	}

	// --type 校验（file / dir / 都不给）。
	wantType := strings.TrimSpace(strings.ToLower(inv.flagVal("--type")))
	if wantType != "" && wantType != "file" && wantType != "dir" {
		inv.setErr("usage", "bad_type", "--type 值非法")
		fmt.Fprintf(stderr, "%s: --type 只能是 file 或 dir（得到 %q）⇒ 退码 2\n", progName, wantType)
		return exitUsage
	}

	// --limit 校验（正整数，默认 200）。
	limit := findRowCap
	if lim := strings.TrimSpace(inv.flagVal("--limit")); lim != "" {
		n, err := strconv.Atoi(lim)
		if err != nil || n <= 0 {
			inv.setErr("usage", "bad_limit", "--limit 非法")
			fmt.Fprintf(stderr, "%s: --limit 必须是正整数（得到 %q）⇒ 退码 2\n", progName, lim)
			return exitUsage
		}
		limit = n
	}

	rows := []map[string]string{}
	hits, dirsScanned := 0, 0
	isFile := wantType == "" || wantType == "file"
	isDir := wantType == "" || wantType == "dir"
	walkErr := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 读不动的子树跳过（只读面不许因一件读不到就整命令失败）
		}
		if info.IsDir() {
			dirsScanned++
			if findSkipDirs[info.Name()] {
				return filepath.SkipDir
			}
			// 目录命中。
			if isDir {
				if strings.Contains(strings.ToLower(info.Name()), pat) {
					hits++
					rel, rerr := filepath.Rel(root, p)
					if rerr != nil {
						rel = p
					}
					rel = filepath.ToSlash(rel)
					if len(rows) < limit {
						rows = append(rows, map[string]string{
							"relpath": rel,
							"size":    "0", // 目录无字节数，占位
							"mtime":   info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
						})
					}
				}
			}
			return nil
		}
		if !isFile {
			return nil // 只要目录：普通文件一律跳过
		}
		name := strings.ToLower(info.Name())
		if !strings.Contains(name, pat) {
			return nil
		}
		hits++
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)
		if len(rows) < limit {
			rows = append(rows, map[string]string{
				"relpath": rel,
				"size":    strconv.FormatInt(info.Size(), 10),
				"mtime":   info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
			})
		}
		return nil
	})
	if walkErr != nil {
		inv.setErr("blocked", "walk_failed", "扫不动")
		fmt.Fprintf(stderr, "%s: 扫不动（%v）⇒ 不给结论（退码 8）\n", progName, walkErr)
		return exitBlocked
	}

	// 排序（按 relpath）。
	sort.Slice(rows, func(i, j int) bool {
		return rows[i]["relpath"] < rows[j]["relpath"]
	})

	fmt.Fprintf(stderr, "%s: 扫了 %d 个子目录 · 命中 %d 件\n", progName, dirsScanned, hits)
	if len(rows) < hits {
		fmt.Fprintf(stderr, "（本页只列前 %d 件 —— 收窄：--root / --type / --limit）\n", len(rows))
	}

	if hits == 0 {
		inv.setErr("failed", "no_match", "零命中")
		fmt.Fprintf(stderr, "%s: 零命中 —— 这不是错，是「没有」（退码 1）\n", progName)
		return exitFail
	}
	return listCmd(inv, stdout, stderr, []string{"relpath", "size", "mtime"}, rows)
}

// findHumanSize —— `find` 人面用的体积（只展示，判据一律用字节数）。
func findHumanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	f := float64(n)
	i := -1
	for f >= unit && i < len(units)-1 {
		f /= unit
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}
