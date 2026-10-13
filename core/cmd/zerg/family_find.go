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

	// 本族只认 `--type` / `--root` / `--limit` 三枚（用法串里写着的那三枚）。
	// 解析器收下来、本命令**不消费**的（例 `--glob`：它归 `code find` 用，`find` 这里从没接过）
	// 过去被**静默吞** ⇒ rc=0、输出与「没给」逐字相同（缺口 `GAP-20260927-16`：同一个命令上
	// 并存两套命运 —— 不在名表里的会被点名退 2，在名表里却不归本命令用的却一声不响）。
	// 现按 dispatch 那条「未知旗标 2」的**同一形状**归一。判在遍历之前 ⇒ 零副作用。
	if bad := foreignFlag(inv, "--type", "--root", "--limit"); bad != "" {
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, bad)
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
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

	// ★ 缺口 `GAP-20260927-14`（2026-10-14 · 真修 ①）：item 级**可选字段** `total` —— 逐行落一格
	//   `total` = 本跑**真命中件数**（跑级真值 · 与包封 `meta.total` **同源同值**）。走可选字段层
	//   （`main.go` 的 `optFields`）：`--json total` 点得出来、逐行真带该键；**不给这一格时输出逐字不变**
	//   （`marshalObjectRegistered` 只投影点名的格 ⇒ 这一段只往行 map 里多写一个键、不动任何既有输出）。
	for _, r := range rows {
		r["total"] = strconv.Itoa(hits)
	}

	fmt.Fprintf(stderr, "%s: 扫了 %d 个子目录 · 命中 %d 件\n", progName, dirsScanned, hits)
	// cut —— 本跑**真裁了件**（命中件数比这一页多 ⇒ 一页硬顶命中）。
	cut := len(rows) < hits
	if cut {
		fmt.Fprintf(stderr, "（本页只列前 %d 件 —— 收窄：--root / --type / --limit）\n", len(rows))
	}

	if hits == 0 {
		inv.setErr("failed", "no_match", "零命中")
		fmt.Fprintf(stderr, "%s: 零命中 —— 这不是错，是「没有」（退码 1）\n", progName)
		// ★ 缺口 `GAP-20260927-14`（2026-10-14 · 真修）：**零命中也要给合计** ——
		//   「空目录 ⇒ 合计 0」是这条面的一条真读数（旧面在零命中那一支 `stdout` 是空的、
		//   `--json` 也没有任何合计键）。两面同出、只增不改：机器面 `meta.total`，人面 `stdout` 一句。
		//   ★ 落笔口径：新增行一律**插在上面那句「（退码 1）」之后** —— 那一行是
		//   `core/internal/contract/exitcode-namespaces.json` 的 R8 落点锚（表记 `family_find.go:192`），
		//   行号不动 ⇒ 连「行号已漂」的提示都不多一条（该锚现按内容判、行号只当提示）。
		if findJSONFieldFaceIntact(inv) {
			inv.metaAddJSON("total", "0")
			inv.metaAddStr("count_basis", findTotalBasis)
		}
		findPrintTotal(stdout, inv, 0, 0)
		return exitFail
	}
	// ★ 截断**自报**（与同族 `code find` 的**同一处判据 · 同一三件**，不自造第二套写法）。
	//
	// 病根（现读实测 · 2026-09-28）：`zerg find a --limit 5` 命中 1183 件时 `items` 只有 5 条，
	// 而包封 `truncated` 恒 `false`、`warnings[]` 恒空、`meta` 无 `truncated_detail`
	// ⇒ 机器面拿到的是一份「看起来完整」的残缺清单，与全量输出**同形**（脚本/子代理据此
	// 判「有/无」正好判错）。人面那一句 `（本页只列前 N 件…）` 只在 stderr ⇒ 机检读不到。
	//
	// 口径（**只用既有的面** —— 不加新旗标 ✗ · 不动 `findRowCap` 这个上限值 ✗）：
	//	① 机器面真值 ← 复用既有包封件（顶层六键**一个不多一个不少** ✗ 第七键）：
	//	   `truncated=true`（`inv.markTruncated()` 是**唯一**置位口）
	//	   + `warnings[]` 一条「已裁 N 件」（与 `truncated=true` 同批）
	//	   + `meta.truncated_detail` 四键（与 `code find` 逐字同形 · 唯一判定口 `truncatedDetailJudge`）。
	//	   `cut_from` 取 `tail`：命中**攒满一页之后的那些被丢掉**（cap 判在 `len(rows) < limit`）；
	//	   三数自校：`kept_items + dropped_items == total_items`。
	//	② 人面末行 ← 见 `listCmd` 之后那一句：表格出完才说，**明说本页不是全集**。
	//
	// 没裁 ⇒ 三件**一律不写**（余量不是「裁了」· 缺席 ≠ 假值）：未截断两面的字节与今日**逐字相同**。
	if cut {
		inv.markTruncated()
		inv.warnf("已裁 %d 件（find 一页 %d 件 / 真命中 %d 件）", hits-len(rows), len(rows), hits)
		inv.metaAddJSON("truncated_detail", fmt.Sprintf(
			`{"cut_from":"tail","kept_items":%d,"dropped_items":%d,"total_items":%d}`,
			len(rows), hits-len(rows), hits))
	}
	// ★ 缺口 `GAP-20260927-14`（2026-10-14 · 真修）：**件数合计** —— 两面同出、只增不改：
	//   ① 机器面 `meta.total` = 本次受控遍历的**真命中件数**（与既有的 `meta.count`＝本页条数
	//      分开写，两个都是真值）；② 人面 `stdout` 末行一句合计（✗ 不落 stderr —— 那一面已有
	//      「命中 N 件」，同一件事再写一遍只添噪声）。**空目录 ⇒ 合计 0**（见上面零命中那一支）。
	if findJSONFieldFaceIntact(inv) {
		inv.metaAddJSON("total", strconv.Itoa(hits))
		inv.metaAddStr("count_basis", findTotalBasis)
	}
	rc := listCmd(inv, stdout, stderr, []string{"relpath", "size", "mtime"}, rows)
	if rc == exitOK {
		findPrintTotal(stdout, inv, hits, len(rows))
	}
	if cut {
		// 人面末行：表格之后再钉一句 —— 「这一页不是全集」不许靠读者自己推。
		fmt.Fprintf(stderr,
			"%s: ⚠ 本页只列前 %d 件 · 真命中 %d 件（已裁 %d 件）—— **这不是全集**，别据此下「有/无」结论；要收窄：--root / --type / --limit\n",
			progName, len(rows), hits, hits-len(rows))
	}
	return rc
}

// findTotalBasis —— 「合计」的口径串（照 `script ls` 的 `count_basis` 先例：**数＋口径同出**，
// 免得读到一个数却不知道它数的是什么）。
const findTotalBasis = "合计=本次受控遍历（排除表 `findSkipDirs` 跳过的目录不计 · `--root` / `--type` 面同算）里**文件名含该片段**的命中件数"

// findJSONFieldFaceIntact —— 「这一发走到 `--json` 出口时不会先死在校验上」的判据口
// （＝`listCmd` 里那句 `requireFields` 的**前置**）。为什么要有它：给 `--json` 却不给字段那一档
// 一律死在校验上、包封走**既有**的错误面，而那一面是**冻结读数**（矩阵 must-fail 格
// `find#--json 不给字段` / `repo/status#--json 不给字段` 的 `want_stdout_bytes`）⇒ 这一支
// **一个 meta 键都不加**，免得新面把既有冻结字节推着涨（✗ 不改矩阵、✗ 不放宽任何判据）。
func findJSONFieldFaceIntact(inv *invocation) bool {
	return inv == nil || !inv.jsonGiven || len(inv.fields) > 0
}

// findPrintTotal —— `find` 人面那一句合计（**只走 stdout** · `--json` 档一律不印，机器面走 `meta.total`）。
// ★ 一页硬顶（`--limit` 裁过）时 `合计` 与 `本页` 不同 ⇒ 两个数都写出来；没裁时两者相等 ⇒ 只写一个数
// （验收里那句「合计与逐行能对上」就是这一条：没裁时 `合计` == 本页列出的行数）。空目录 ⇒ 逐字 `合计 0 件`。
func findPrintTotal(stdout io.Writer, inv *invocation, hits, page int) {
	if inv != nil && inv.jsonGiven {
		return
	}
	if page != hits {
		fmt.Fprintf(stdout, "合计 %d 件（本页 %d 件）\n", hits, page)
		return
	}
	fmt.Fprintf(stdout, "合计 %d 件\n", hits)
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
