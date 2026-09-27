// family_code.go —— 码面只读一条（§九 I1 · 缺口-命令面-20260921 §十一 P0-1）。
//
// 为什么它排 P0 第一位：今天手搓的 **168 行里 `grep` / `git grep` 占 37 行（最大一类）**，
// 而命令面**零覆盖**；设计稿 §二 2.3 逐字「AI agent 不许绕过命令面直拼 curl/shell」——
// 「读码」正是每一次自开发的第一环。所以它不是「顺手加一个」，是**入口**。
//
// 口径（照 §十一 P0-1 的形态，不自造）：
//
//	形态 `zerg code find <正则> [--path <目录或单件>] [--glob <模式>] [--full] [--json <字段>]`
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

// pathWithinDir —— 出仓判据按**路径段**判（不用 `strings.HasPrefix`：字符串前缀会把
// `…/模型类/Zerg-内部文档` 当成 `…/模型类/Zerg` 的「仓内」—— 那正是 `W-03` 现读的病根）。
// 判据逐字：`filepath.Rel(dir,p)` 落在 `dir` 里 ⇔ 相对路径为 `.` 或**不以 `..` 段开头**。
func pathWithinDir(dir, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(p))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func cmdCodeFind(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_pattern", "缺正则")
		fmt.Fprintf(stderr, "%s: `code find` 要给正则（例：zerg code find 'func cmdGate' --path core/cmd/zerg）\n", progName)
		fmt.Fprintf(stderr, "用法：zerg code find <正则> [--path <目录或单件>] [--glob <模式>] [--full] [--json <字段>]\n")
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
		if filepath.IsAbs(sub) {
			// 绝对路径 = **显式**指向另一根（例：文档仓）—— 不再与仓根拼接。
			// 旧写法一律 `filepath.Join(root, sub)`：`Join` 把绝对段当**相对段**拼到仓根后面
			// ⇒ 拼出来的路径当然不存在 ⇒ 报「--path 指的不是目录」这条**假阴**
			// （`W-03` 现读：路径确是目录，判据要 rc=0）。
			base = filepath.Clean(filepath.FromSlash(sub))
		} else {
			base = filepath.Join(root, filepath.FromSlash(sub))
			// 出仓判据 = **路径段**（不是字符串前缀）：`…/模型类/Zerg` 正是
			// `…/模型类/Zerg-内部文档/…` 的**字符串**前缀 ⇒ 旧写法把**仓外**目录当仓内放行。
			// 仓外的根一律用绝对路径显式指（上面那一支）—— 不靠相对路径蒙混。
			if !pathWithinDir(root, base) {
				inv.setErr("usage", "path_outside_repo", "--path 出仓")
				fmt.Fprintf(stderr, "%s: --path 出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, sub, root)
				fmt.Fprintf(stderr, "下一步：仓外的根用**绝对路径**显式指（例：--path /abs/到/那个根）\n")
				return exitUsage
			}
		}
		// ★ `--path` 收**目录**与**常规件**（单件与目录在扫描面上是同一件事：
		// `filepath.Walk` 对非目录根**恰好**调 walkFn 一次）—— 卡住的只是这一行守卫。
		// 只放宽接受面：kind/detail/退码/用法串文本一律不动（kind 是闭集，见 check-error-kinds.py R3）。
		st, serr := os.Stat(base)
		if serr != nil || (!st.IsDir() && !st.Mode().IsRegular()) {
			inv.setErr("usage", "path_not_dir", "--path 既不是件也不是目录")
			fmt.Fprintf(stderr, "%s: --path 指的既不是件也不是目录：%s（路径不存在、或它不是件也不是目录 · 先 ls 确认路径名 · 退码 2）\n", progName, sub)
			return exitUsage
		}
	}
	glob := strings.TrimSpace(inv.flagVal("--glob"))
	// ★ 非法 `--glob` 模式**当场判住**（2026-09-28 · 本笔）：下面 walk 里那句
	// `if ok, _ := filepath.Match(glob, filepath.Base(p))` 把 `Match` 的**第二返回值
	// （`ErrBadPattern`）丢了** ⇒ 模式写坏时每一件都被 `!ok` 拦掉 ⇒ 扫了 0 件 · 命中 0 条 ·
	// 退码 1，与「真的零命中」**同一个形状**（机器面还照报 kind=failed/detail=no_match/
	// message=零命中）⇒ 使用者分不出「我的模式写坏了」与「真没有」。
	//
	// 治法照本命令既有两处**同一口径**（坏正则 :58 / 坏 --limit :118：用法错 2 + 逐字点名），
	// 判在 walk **之前** ⇒ 零副作用：拿**空名字**试一次 `filepath.Match`，把模式单独验一遍
	// （`Match` 的 `failed` 档会**继续解析模式本身** ⇒ 空名字也验得出坏模式：实测
	// 单字 `[` · `a[` · 单反斜杠 · `*.[` 全报 syntax error in pattern，好模式一律 nil）。
	// 只判**模式本身**：目录/件读不到仍走 walk 里的「跳过」那一支，**不许**把「读不到」读成
	// 「模式坏」；模式好时这一趟 `Match` 不参与扫描 ⇒ 命中面与改前逐字节相同。
	// （同族第二处同类 `family_reap.go:71` 不在本件面上 ⇒ 只记回执，不越面改。）
	if glob != "" {
		if _, gerr := filepath.Match(glob, ""); gerr != nil {
			inv.setErr("usage", "bad_glob", "--glob 模式非法")
			fmt.Fprintf(stderr, "%s: --glob 模式非法（模式 %q）：%v ⇒ 退码 2\n", progName, glob, gerr)
			fmt.Fprintf(stderr, "下一步：换一个模式（例：--glob '*.go' · --glob 'family_*.go'）—— 通配符要成对，[ 要配 ]\n")
			return exitUsage
		}
	}
	// `--limit <N>`（缺口 `GAP-20260927-15` · 2026-09-28）：本页最多列多少条（正整数 · 默认
	// `codeFindRowCap`）。此前 `code find` **没有**这一枚 —— 200 条是硬顶，页脚还建议「收窄」，
	// 真给它一枚在册旗标（`--limit`）却被**静默吞**（`--full` 也不抬顶：它只管长行截断）。
	//
	// 治法取 **ⓐ 接线**（不是退 2 + 从名表里去掉），理由三条（现读坐标见批注）：
	//   ① 截断口**本来就在本命令里**：`len(rows) < codeFindRowCap`（下面 walk 里那一行）
	//      = 一个现成的参数口，接线只是把常量换成这一趟的值；
	//   ② 名表是**跨族共用**的一张（`main.go` 的 `valueFlagName` 里 `--limit` 与 `zerg find`
	//      共用一枚名）⇒ 「从旗标登记表里去掉」会连 `zerg find --limit` 一起弄死；
	//   ③ 给这两族各开一个解析分叉是 `main.go` 明文的禁止项（「解析器不许给两条命令各开一个
	//      分叉」）⇒ 退 2 那条路在此处要么不成立、要么要动共用名表。
	// 默认（不给这一枚）时这一趟的值就是 `codeFindRowCap` ⇒ 不带旗标的 rc/输出**逐字节不变**。
	rowCap := codeFindRowCap
	if lim := strings.TrimSpace(inv.flagVal("--limit")); lim != "" {
		n, lerr := strconv.Atoi(lim)
		if lerr != nil || n <= 0 {
			inv.setErr("usage", "bad_limit", "--limit 非法")
			fmt.Fprintf(stderr, "%s: --limit 必须是正整数（得到 %q）⇒ 退码 2\n", progName, lim)
			return exitUsage
		}
		rowCap = n
	}
	// 本族只认自己那几枚旗标（`--path`/`--glob`/`--full`/`--limit`）。
	// 解析器**收下来**、本命令**不消费**的（例：`--all` 是给 `build show`/`gap verify` 一族用的
	// 全局布尔）过去被**静默吞** —— 同一个命令上因此并存两套命运（`--max` 会被点名退 2、
	// `--all` 却一声不响），这正是缺口 `GAP-20260927-16`。现按 dispatch 那条「未知旗标 2」的
	// **同一形状**归一：rc=2 + 逐字点名。判在扫之前 ⇒ 零副作用。
	if bad := foreignFlag(inv, "--path", "--glob", "--full", "--limit", "--count", "--files-only"); bad != "" {
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, bad)
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}

	// ★ 按件聚合两档（缺口 `GAP-20260928-53` 的 ② · 在册「想要」逐字
	// `zerg code-show-lines-range-and-find-files-only-count` ⇒ 旗标名就照它取：`--count` / `--files-only`）。
	//
	// 为什么这两枚要**上全局户口**（`main.go` 的解析器）：本仓的解析器只有**一张**旗标表，
	// 而且 `main.go` 明文禁止「给两条命令各开一个分叉」⇒ 一族用的旗标只能在那一处登记
	// （与 `--full`/`--last`/`--utf8` 同一种形态：全局布尔、谁用谁读）。「不归本命令用」由本命令 +
	// 同族 `code show` + `find` 三处 own-list 兜住：这三处不认的旗标一律 rc=2 点名（见 `foreignFlag`）。
	//
	// 两档互斥（同给 = 自相矛盾 ⇒ 用法错 2），与同仓 `--dry-run`/`--yes` 那一对的判法同一条口径。
	// 判在 walk 之前 ⇒ 零副作用（不扫一个件）。
	aggFilesOnly := inv.hasFlag("--files-only")
	aggCount := inv.hasFlag("--count")
	if aggFilesOnly && aggCount {
		inv.setErr("usage", "agg_flags_conflict", "--count 与 --files-only 同给")
		fmt.Fprintf(stderr, "%s: --count 与 --files-only 是**两枚互斥的聚合档**（同给 = 自相矛盾 · 只给一枚）⇒ 退码 2\n", progName)
		fmt.Fprintf(stderr, "用法：zerg code find <正则> [--path <目录或单件>] [--glob <模式>] [--limit <N>] [--count | --files-only] [--full] [--json <字段>]\n")
		return exitUsage
	}

	rows := []map[string]string{}
	// 逐件命中数（**不受一页硬顶影响** —— 「数命中」与「列一页」是两件事；下面 walk 里每命中一条就 +1）。
	fileHits := map[string]int{}
	filesOrder := []string{}
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
			// 上面已验过模式：这里的第二返回值（坏模式）此刻必为 nil ⇒ 丢弃不再是吞错。
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
			if fileHits[rel] == 0 {
				filesOrder = append(filesOrder, rel)
			}
			fileHits[rel]++
			if len(rows) < rowCap {
				rows = append(rows, map[string]string{
					"path": rel,
					"line": strconv.Itoa(i + 1),
					"text": codeTextCell(inv, ln),
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
	// ★ 聚合两档的出口（缺口 `GAP-20260928-53` 的 ②）：**件名 + 命中数**（`--count`）或**只列件名**
	// （`--files-only`）。走同一份汇总前缀（「扫了 N 件 · 命中 M 条」）⇒ 两档与默认档的读者拿到同一组底数。
	// 硬顶（本页最多 `rowCap` 行）在聚合面上仍然生效，且**照实自报**（`markTruncated` + `warnings[]` +
	// `meta.truncated_detail` 三件，与默认档同一条路 —— 缺口 `GAP-20260927-273` 的判法不许只覆盖一半）。
	if aggFilesOnly || aggCount {
		files := append([]string{}, filesOrder...)
		sort.Strings(files) // 判词确定性：map 的乱序不许决定输出次序
		aggRows := make([]map[string]string, 0, len(files))
		for _, f := range files {
			if aggFilesOnly {
				aggRows = append(aggRows, map[string]string{"path": f})
			} else {
				aggRows = append(aggRows, map[string]string{"path": f, "count": strconv.Itoa(fileHits[f])})
			}
		}
		if aggCount {
			// 命中多的在前（同数按件名字典序 —— 上面已排好 ⇒ 稳定排序给出的次序是确定的）。
			sort.SliceStable(aggRows, func(i, j int) bool {
				return atoiSafe(aggRows[i]["count"]) > atoiSafe(aggRows[j]["count"])
			})
		}
		totalFiles := len(aggRows)
		cut := totalFiles > rowCap
		if cut {
			aggRows = aggRows[:rowCap]
		}
		fmt.Fprintf(stderr, "%s: 扫了 %d 件 · 命中 %d 条", progName, scanned, hits)
		aggFields := []string{"path"}
		if aggCount {
			aggFields = []string{"path", "count"}
		}
		fmt.Fprintf(stderr, " · 命中件 %d 个", totalFiles)
		if cut {
			fmt.Fprintf(stderr, "（本页只列前 %d 个件 —— 收窄：--path / --glob / --limit）", len(aggRows))
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
		if cut {
			inv.markTruncated()
			inv.warnf("已裁 %d 个件（code find 一页 %d 个件 / 真命中 %d 个件）", totalFiles-len(aggRows), len(aggRows), totalFiles)
			inv.metaAddJSON("truncated_detail", fmt.Sprintf(
				`{"cut_from":"tail","kept_items":%d,"dropped_items":%d,"total_items":%d}`,
				len(aggRows), totalFiles-len(aggRows), totalFiles))
		}
		rcAgg := listCmd(inv, stdout, stderr, aggFields, aggRows)
		if cut {
			fmt.Fprintf(stderr,
				"%s: ⚠ 本页只列前 %d 个件 · 真命中 %d 个件（已裁 %d 个件）—— **这不是全集**，别据此下「有/无」结论；要收窄：--path / --glob / --limit\n",
				progName, len(aggRows), totalFiles, totalFiles-len(aggRows))
		}
		return rcAgg
	}
	fmt.Fprintf(stderr, "%s: 扫了 %d 件 · 命中 %d 条", progName, scanned, hits)
	// cut —— 本跑**真裁了条**（命中数比这一页多）⇒ 两面都要自报（缺口 `GAP-20260927-273`）。
	cut := len(rows) < hits
	if cut {
		fmt.Fprintf(stderr, "（本页只列前 %d 条 —— 收窄：--path / --glob / --limit）", len(rows))
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
	// ★ 截断**自报**（缺口 `GAP-20260927-273` · 2026-09-27）。
	//
	// 病根（父代理现场实测）：命中 250 条时 `items` 只有 200 条，而包封 `truncated` 恒 `false`、
	// `warnings[]` 恒空 ⇒ **机器面**拿到的是一份「看起来完整」的残缺清单 ✗（脚本/子代理据此判
	// 「有/无」正好判错）；**人面** TTY 末行还会说「共 200 条」（同病：看起来是全集）。
	//
	// 口径（**只用既有的面** —— 不加新旗标 ✗ · 不动 `codeFindRowCap` 这个上限值 ✗）：
	//
	//	① 机器面真值 ← 复用既有包封件（顶层六键**一个不多一个不少** ✗ 第七键）：
	//	   `truncated=true`（`inv.markTruncated()` 是**唯一**置位口）
	//	   + `warnings[]` 一条「已裁 N 条」（与 `truncated=true` 同批）
	//	   + `meta.truncated_detail` 四键（块D `K-1` 形状 · 唯一判定口 `truncatedDetailJudge`）。
	//	   `cut_from` 取 `tail`：命中**攒满一页之后的那些被丢掉**（cap 判在 `len(rows) < codeFindRowCap`）；
	//	   三数自校：`kept_items + dropped_items == total_items`。
	//	② 人面末行 ← 见 `listCmd` 之后那一句：表格出完才说，**明说本页不是全集**。
	//
	// 没裁 ⇒ 三件**一律不写**（余量不是「裁了」· 缺席 ≠ 假值）：未截断两面的字节与今日**逐字相同**。
	if cut {
		inv.markTruncated()
		inv.warnf("已裁 %d 条（code find 一页 %d 条 / 真命中 %d 条）", hits-len(rows), len(rows), hits)
		inv.metaAddJSON("truncated_detail", fmt.Sprintf(
			`{"cut_from":"tail","kept_items":%d,"dropped_items":%d,"total_items":%d}`,
			len(rows), hits-len(rows), hits))
	}
	rc := listCmd(inv, stdout, stderr, []string{"path", "line", "text"}, rows)
	if cut {
		// 人面末行：表格之后再钉一句 —— 「这一页不是全集」不许靠读者自己推。
		fmt.Fprintf(stderr,
			"%s: ⚠ 本页只列前 %d 条 · 真命中 %d 条（已裁 %d 条）—— **这不是全集**，别据此下「有/无」结论；要收窄：--path / --glob / --limit\n",
			progName, len(rows), hits, hits-len(rows))
	}
	return rc
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
	// `--count` / `--files-only` 是 `code find` 的**按件聚合**两档（缺口 `GAP-20260928-53` 的 ②）。
	// 它们今天在本命令上是 dispatch 那条「未知旗标 2」（解析器还没有这两个名字）；
	// 现在它们**上了全局户口**（`main.go` 的解析器只有一张旗标表、且明文禁止「给两条命令
	// 各开一个分叉」⇒ 只能在那一处登记）⇒ 必须在这里按**同一形状**把「不归本命令用
	// 的旗标」留住：rc=2 + 逐字点名（与 dispatch 那两行逐字同形）。不这么做，它们会从「rc=2」
	// 退成**静默吞** —— 正是缺口 `GAP-20260927-16` 记的那一类病。判在一切之前 ⇒ 零副作用。
	if inv.hasFlag("--count") || inv.hasFlag("--files-only") {
		bad := "--count"
		if inv.hasFlag("--files-only") {
			bad = "--files-only"
		}
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, bad)
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
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
	rel, spec := target[:colon], target[colon+1:]
	// ★ **区间档**（缺口 `GAP-20260928-53` 的 ① · 2026-09-28）：`件:起-止` 一发读出起止之间的**每一行**
	// （在册那条缺口的实测例逐字就是 `（:53-65 / :67-71）` 这一形态 —— 核一件的分区结构过去只能逐段读、
	// 或退回 read_file）。
	//
	// 旗标选型理由（为什么把区间写在**位置参数**里、而不新开一枚 `--lines <起-止>`）三条：
	//   ① **在册缺口自己把它写成了位置参数**：复现例就是 `:53-65`（同一个位置上从「一个行号」变成「一个行号区间」）；
	//   ② `--ctx` 本就是本命令的**窗口半径**（±N）—— 区间与它在语义上是同一条轴（改起止 vs 改半径），
	//     再开一枚取值旗标会让同一件事有**两个入口**；
	//   ③ 名表（`main.go` 的 `valueFlagName`）是**跨族共用**的一张（`--limit` 就与 `zerg find` 共用）
	//     ⇒ 少响一枚就少一处外溢。
	//
	// 单行（无 `-`）走**原来那一支、一行不动**（rc/输出逐字节不变）；区间走新支。
	lineNo, endNo, isRange := 0, 0, false
	if dash := strings.Index(spec, "-"); dash > 0 && dash < len(spec)-1 {
		if sn, s1 := strconv.Atoi(spec[:dash]); s1 == nil {
			if en, e1 := strconv.Atoi(spec[dash+1:]); e1 == nil && sn >= 1 && en >= 1 {
				isRange = true
				lineNo, endNo = sn, en
			}
		}
	}
	if !isRange {
		n, aerr := strconv.Atoi(spec)
		if aerr != nil || n < 1 {
			inv.setErr("usage", "bad_line", "行号不是正整数")
			fmt.Fprintf(stderr, "%s: 行号要是正整数：%q ⇒ 退码 2\n", progName, spec)
			return exitUsage
		}
		lineNo = n
	} else if lineNo > endNo {
		inv.setErr("usage", "bad_range", "区间起点比终点大")
		fmt.Fprintf(stderr, "%s: 区间的起点比终点大：%s ⇒ 退码 2\n", progName, spec)
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
	// 区间档的窗口默认半径 0（区间本身就是**显式**的起止 ⇒ 不再自动外扩；
	// `--ctx` **给过**才向两端扩）—— 与 `--ttl`/`--prefix` 同一条口径：「给过旗标」与「没给」是两件事。
	if isRange && !inv.hasFlag("--ctx") {
		ctx = 0
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
	if lineNo > len(lines) || (isRange && endNo > len(lines)) {
		inv.setErr("usage", "line_out_of_range", "行号越界")
		if isRange {
			hit := lineNo
			if endNo > len(lines) {
				hit = endNo
			}
			fmt.Fprintf(stderr, "%s: 区间越界：%s 共 %d 行，区间点到了第 %d 行 ⇒ 退码 2\n", progName, rel, len(lines), hit)
		} else {
			fmt.Fprintf(stderr, "%s: 行号越界：%s 共 %d 行，点了第 %d 行 ⇒ 退码 2\n", progName, rel, len(lines), lineNo)
		}
		fmt.Fprintf(stderr, "下一步：先 `zerg code find <正则>` 拿到真行号\n")
		return exitUsage
	}
	lo, hi := lineNo-ctx, lineNo+ctx
	if isRange {
		hi = endNo + ctx
	}
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
		if isRange {
			if i >= lineNo && i <= endNo {
				mark = "yes"
			}
		} else if i == lineNo {
			mark = "yes"
		}
		rows = append(rows, map[string]string{
			"path":   nrel,
			"line":   strconv.Itoa(i),
			"text":   codeTextCell(inv, lines[i-1]),
			"target": mark,
		})
	}
	if isRange {
		fmt.Fprintf(stderr, "%s: %s 第 %d-%d 行（共 %d 行 · 窗口 ±%d）\n", progName, nrel, lineNo, endNo, len(lines), ctx)
	} else {
		fmt.Fprintf(stderr, "%s: %s 第 %d 行（共 %d 行 · 窗口 ±%d）\n", progName, nrel, lineNo, len(lines), ctx)
	}
	return listCmd(inv, stdout, stderr, []string{"line", "text"}, rows)
}

// codeShowDefaultCtx —— `code show` 默认窗口半径（±3 行 = 「一眼看得到上下文」的最小面）。
const codeShowDefaultCtx = 3

// codeTextCell —— `code find` / `code show` 的行文本格（缺口 `GAP-20260927-09` · 2026-09-27）。
//
// 病根（父代理现场实测）：`zerg code show gateway/fleet.yaml:115` 出到 `…mmproj: "~/zerg-models/O…`
// 就断了，而**这一格同时喂人面与机器面**（`listCmd` 同一份 rows）⇒ 长行尾巴在**两面上一起消失** ✗，
// 取证时只能退回手搓 `read_file` ✗ —— 「能走 CLI 就走」在长行上正好走不通。
//
// 口径：默认仍截断 200 字（人面好看的既有行为**不动** · `--full` 才不截断）；不截断是**显式**档，
// 不把默认面加长（既有面的字节数一个不动 · 契约矩阵的既有格逐字保留）。
func codeTextCell(inv *invocation, ln string) string {
	s := strings.TrimRight(ln, "\r")
	if inv.full {
		return s
	}
	return truncateDisplay(s, 200)
}

// truncateDisplay 只为人面好看：超长行截断（**不改机器面已列出的原文之外的东西**）。
func truncateDisplay(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// foreignFlag —— 判「解析器收下来了、但本命令**不消费**」的旗标（一枚都没有 ⇒ 空串）。
// 缺口 `GAP-20260927-16`（未知/无用旗标被静默吞 · 同一个命令上两套命运）。
//
// 为什么要有它：同一个命令上过去并存**两套命运** —— 不在名表里的（例 `code find --max 5`）由
// dispatch 那条「未知旗标 2」**点名拒**；而在名表里（或全局布尔里）、却**不归本命令用**的
// （例 `code find --all`、`find --glob`）被**静默吞** ⇒ rc=0 且毫无可观察效果。于是「给了旗标」
// 与「没给旗标」被并成**同一个形状** —— 正是本仓判为 P0 的那一类病（用户以为收窄了，拿到的却是
// 未收窄的全集）。
//
// 口径：形状**照抄** dispatch 那条既有拒法（`未知旗标 %q` + `See '…--help'。` + 退码 2），
// 不新造退码、不新造词。判定面**只在本族自己的实现件里**（`inv.kv` 的取值旗标 + `--all`/`--full`
// 两枚全局布尔），**不碰解析器** —— `main.go` 明文：解析器不许给两条命令各开一个分叉。
func foreignFlag(inv *invocation, own ...string) string {
	ok := make(map[string]bool, len(own))
	for _, n := range own {
		ok[n] = true
	}
	names := make([]string, 0, len(inv.kv))
	for n := range inv.kv {
		if !ok[n] {
			names = append(names, n)
		}
	}
	if len(names) > 0 {
		sort.Strings(names) // 判词确定性：map 的乱序不许决定报哪一枚
		return names[0]
	}
	if inv.all && !ok["--all"] {
		return "--all"
	}
	if inv.full && !ok["--full"] {
		return "--full"
	}
	return ""
}
