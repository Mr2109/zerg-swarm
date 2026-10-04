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
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// codeScanSkipDirs —— 扫码排除表（与 §十七/§二十一 各处复跑命令**同一套**口径；
// 另把 `bin` 与对象仓排除掉：它们不是「码」，扫进去只会把命中数灌水）。
//
// ★ 2026-09-29（缺口 `GAP-20260927-66` + `GAP-20260927-72`）：由「固定点名单」改成「规则 + 白名单」，
//
//	三件事（**不新立第三张表** —— 名字与口径直接照 `scripts/gates/check-hardcoded-private-paths.py`
//	的 `SKIP_DIRS` 那一份）：
//	① 运行期/工具态目录**点名**（那份口径里的 `__pycache__`/`.pytest_cache`/`.zerg`/`.codegraph`/
//	   `.hermes`/`.cache` 等；本表只补 `__pycache__` 这一类**不带点**的名字，带点的走规则②）；
//	② 其余**任何以 `.` 开头的目录一律跳过**（= 那份口径注释「以 `.` 开头的目录一律跳过」同一条规则）
//	   —— `.zerg`/`.codegraph`/`.stage`/`.hermes`/`.cache` 由此自动落闸，不必逐个背；
//	③ 白名单 `codeScanKeepDots`：`.github`/`.githooks` 是**随码发布**的仓库配置（tracked、非运行期态）
//	   ⇒ 规则②对它们例外（点名单优先）。
//	件级闸另起（只加目录闸修不掉 `.env`）—— 见下面 `codeScanSkipFile`，理由同批。
var codeScanSkipDirs = map[string]bool{
	"target": true, "node_modules": true, "dist": true, "bin": true, "vendor": true,
	"data": true, ".git": true, ".venv": true, "venv": true, ".build": true,
	"__pycache__": true, ".pytest_cache": true, ".mypy_cache": true, ".idea": true,
}

// codeScanKeepDots —— 以 `.` 开头、但**随码发布**的目录白名单（缺口 `GAP-20260927-66` ②）。
// `.github`（CI/workflow）与 `.githooks`（提交钩子件）是仓库配置、走 git 跟踪、不随一跑就长件
// ⇒ 目录规则②（点项一律跳过）对它们例外。
var codeScanKeepDots = map[string]bool{".github": true, ".githooks": true}

// codeScanSkipFile —— 件级闸（缺口 `GAP-20260927-66`：只加目录闸修不掉 `.env`）。
//
// 病根现读（改前）：`.env` 在**仓根、不是目录** ⇒ 目录闸对它零作用；`zerg code find ZERG_AUTH_TOKEN`
// 的默认面把 `.env`（内含 `ZERG_AUTH_TOKEN` 真值）原样扫进 items 并打印命中行。
//
// 口径（与目录规则②同一条轴 —— 「点项默认不进码面」，配 `--hidden` 显式放行）：
// 以 `.` 开头的**件**（`.env`/`.env.example`/`.env.local` 这类凭据与运行期配置）默认不进码面；
// 白名单目录内部的件照进（`.github/workflows/…`、`.githooks/pre-commit`）。判在 walk 之前
// 的件级闸与 walk 内的目录闸**同一套判据**，不各写一份。
func codeScanSkipFile(root, p string) bool {
	base := filepath.Base(p)
	if !strings.HasPrefix(base, ".") {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	if err == nil {
		for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
			if codeScanKeepDots[seg] {
				return false
			}
		}
	}
	return true
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
	// ★ 缺口 `GAP-20260927-252`（本枚 · P1）：`-i`/`--ignore-case` —— 大小写归一档（POSIX grep 惯用）。
	// 先按**原串**编译（错误信息与改前逐字同），再叠 `(?i)` 前缀（上面已验过是合法模式 ⇒ 不会再失败）。
	// 不给这一枚 ⇒ `re` 逐字同改前（默认档命中面/退码/输出一个字节不动）。
	ignoreCase := inv.hasFlag("--ignore-case")
	if ignoreCase {
		re, _ = regexp.Compile("(?i)" + pat)
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
	if bad := foreignFlag(inv, "--path", "--glob", "--full", "--limit", "--count", "--files-only", "--hidden", "--include-runtime", "--ignore-case", "--after", "--before", "--kind", "--timeout"); bad != "" {
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, bad)
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}

	// ★ 缺口 `GAP-20260927-250/251/255`（本枚 · P1）：上下文半径 / 代码-注释分流 / 面内时限三档解析。
	// 全部**判在 walk 之前** ⇒ 零副作用（坏值不扫一个件）；退码族照本命令既有两条（用法错 2 + 逐字点名）。
	afterN, aErr := codeFindCtxN(inv, "--after")
	if aErr != "" {
		inv.setErr("usage", "bad_context_n", aErr)
		fmt.Fprintf(stderr, "%s: %s ⇒ 退码 2\n", progName, aErr)
		return exitUsage
	}
	beforeN, bErr := codeFindCtxN(inv, "--before")
	if bErr != "" {
		inv.setErr("usage", "bad_context_n", bErr)
		fmt.Fprintf(stderr, "%s: %s ⇒ 退码 2\n", progName, bErr)
		return exitUsage
	}
	kindWant := strings.TrimSpace(inv.flagVal("--kind"))
	if kindWant != "" && kindWant != "code" && kindWant != "comment" {
		inv.setErr("usage", "bad_kind", "非法 --kind")
		fmt.Fprintf(stderr, "%s: --kind 只收 `code` 或 `comment`（得到 %q）⇒ 退码 2\n", progName, kindWant)
		return exitUsage
	}
	// ★ `--timeout <秒>`（缺口 `GAP-20260927-255`）：**不在本命令里再实现一遍** —— 它已是**全局墙钟**旗标
	//   （`wallclock.go` · 上级裁定 B · 到点先复原再报 · 退码 `11`），`run` 在派发**之前**统一消费。
	//   本命令只把它记进 own-list（`foreignFlag` 不拒），语义全归 `wallclockGuard`（**一处口径**，
	//   不在这里造第二个消费者 = 两义 ✗）。

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

	// ★ 缺口 `GAP-20260928-141`（2026-10-04 · 本枚）：**字段面前置判定** —— 把 `--json` 的字段名
	//   校验从「walk 之后」（`listCmd` → `selectJSONList` → `marshalObject` 逐行查 row 缺键）**前移**
	//   到扫之前，与同族「用法面先判、零副作用」一致：改前给一枚坏字段（`--json zzz`）或一个字段都
	//   不给（`--json`）都**先扫完整棵树**才报用法错（stderr 先落「扫了 N 件」再落错误）✗。
	//   口径（**只前移判定 · 合法形态一个字节不变**）：合法字段集按**本跑选中的那一档面**的真产出集
	//   取 —— 默认面 `path/line/text` · `--count` 面 `path/count` · `--files-only` 面 `path`；三档都另加
	//   跑级落键 `truncated`/`face`（见 `markCodeFindTruncated`/`markCodeFindFace`）。
	//   命中非法 ⇒ 与 walk 之后那条**同 kind 同 detail 同退码**（`json_field_unknown` · `reportBadField`
	//   逐字同文案）；一个字段都不给 ⇒ 走既有 `requireFields`（同 kind/detail）。合法形态 ⇒ 不进这一块。
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr)
		}
		allowed := map[string]bool{"path": true, "line": true, "text": true, "kind": true, "match": true, "truncated": true, "face": true}
		switch {
		case aggCount:
			allowed = map[string]bool{"path": true, "count": true, "truncated": true, "face": true}
		case aggFilesOnly:
			allowed = map[string]bool{"path": true, "truncated": true, "face": true}
		}
		for _, f := range inv.fields {
			if !allowed[f] {
				inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
				return reportBadField(stderr, inv.path, f)
			}
		}
	}

	// ★ 显式放行两枚（缺口 `GAP-20260927-66` 的 `--hidden` · `GAP-20260927-72` 的 `--include-runtime`）：
	// 点项与运行期态默认不进码面（见 `codeScanSkipDirs`/`codeScanSkipFile` 头注），要扫进来**必须显式点**
	// —— 照 `--full`/`--all` 同一种形态（全局布尔、谁用谁读；本族只认，**不新开解析分叉**）。
	// ★ 这两枚名尚未上全局户口（`main.go` 的 `valueFlagName`）⇒ 今天真给会被 dispatch 判「未知旗标 2」
	//   （**不是静默吞**）；登记片段见回执，由父代理落。
	showHidden := inv.hasFlag("--hidden")
	includeRuntime := inv.hasFlag("--include-runtime")
	rows := []map[string]string{}
	// 逐件命中数（**不受一页硬顶影响** —— 「数命中」与「列一页」是两件事；下面 walk 里每命中一条就 +1）。
	fileHits := map[string]int{}
	filesOrder := []string{}
	hits, scanned, skippedBig := 0, 0, 0
	// ★ 缺口 `GAP-20260927-250`（本枚）：已**列**进一页的命中数（上下文档里「一页行数」≠「命中数」，
	//   截断自报必须以「命中」为分母）。默认档 `emittedMatches == len(rows)` ⇒ 与改前逐字节同。
	emittedMatches := 0
	walkErr := filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 读不动的子树跳过（只读面不许因为一件读不到就整命令失败）
		}
		if info.IsDir() {
			// 目录闸 = 点名单 ∪ 规则②（任何 `.` 开头 · 白名单例外）；`--include-runtime` 放行整档。
			dotDir := strings.HasPrefix(info.Name(), ".") && !codeScanKeepDots[info.Name()]
			if !includeRuntime && (codeScanSkipDirs[info.Name()] || dotDir) {
				return filepath.SkipDir
			}
			return nil
		}
		// 件级闸（缺口 `GAP-20260927-66`）：`.` 开头的件默认不进码面（只加目录闸修不掉 `.env`）。
		if !showHidden && !includeRuntime && codeScanSkipFile(root, p) {
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
		lines := strings.Split(string(b), "\n")
		// 先按「正则 + `--kind` 分流」收集本件命中行下标（缺口 `GAP-20260927-251`）。
		var idxs []int
		for i, ln := range lines {
			if !re.MatchString(ln) {
				continue
			}
			if kindWant != "" && codeLineKind(ln) != kindWant {
				continue
			}
			idxs = append(idxs, i)
		}
		if len(idxs) == 0 {
			return nil
		}
		if fileHits[rel] == 0 {
			filesOrder = append(filesOrder, rel)
		}
		fileHits[rel] += len(idxs)
		hits += len(idxs)
		if afterN == 0 && beforeN == 0 {
			// 默认档：逐命中一行（**与改前逐字节相同** —— 只是行里多带了 `kind` 键，不被投影就不出）。
			for _, i := range idxs {
				if emittedMatches >= rowCap {
					break
				}
				rows = append(rows, inv.codeFindRow(rel, i+1, lines[i], ""))
				emittedMatches++
			}
			return nil
		}
		// 上下文档（缺口 `GAP-20260927-250`）：逐命中带出 ±N 邻域，重叠窗口去重、命中行标 `match=yes`。
		matchSet := make(map[int]bool, len(idxs))
		for _, i := range idxs {
			matchSet[i] = true
		}
		lastEmit := -2
		for _, i := range idxs {
			if emittedMatches >= rowCap {
				break
			}
			lo, hi := i-beforeN, i+afterN
			if lo < 0 {
				lo = 0
			}
			if hi > len(lines)-1 {
				hi = len(lines) - 1
			}
			if lo <= lastEmit {
				lo = lastEmit + 1
			}
			for j := lo; j <= hi; j++ {
				mark := "no"
				if matchSet[j] {
					mark = "yes"
				}
				rows = append(rows, inv.codeFindRow(rel, j+1, lines[j], mark))
			}
			if hi > lastEmit {
				lastEmit = hi
			}
			emittedMatches++
		}
		return nil
	})
	if walkErr != nil {
		inv.setErr("blocked", "walk_failed", "扫不动")
		fmt.Fprintf(stderr, "%s: 扫不动（%v）⇒ 不给结论（退码 8）\n", progName, walkErr)
		return exitBlocked
	}
	// ★ 面标记（缺口 `GAP-20260929-29` · 2026-09-29）：本命令的**机器面**自带「面」这一格读数。
	// 本命令走 `filepath.Walk` **直读工作树**（tracked + 未跟踪件）；同仓两把「词在哪儿」的正门里，
	// `impact` 的 ④a 走 `git grep` —— **索引面(tracked 件的工作树内容)**，两面对同一个词可以给出
	// 不同答案。过去「面」只在散文/口径串里说 ⇒ 机器面拿不到直读口。此处按**只增**落键：
	//   · 只写 `meta` 的**按需子键**（`face`）—— 不加第七个顶层键（`main.go` 口径 2）、
	//     不覆盖任何旧子键（`face` 不在 `envMetaReserved` 里）；
	//   · **只在 `--json` 那一态落键**（人面表格 / 退码 / 既有键逐字节不变，判在打印之前）；
	//   · 零命中那一档同样落键（**零命中 ≠ 没有** —— 正是要知道「读的是哪个面」的场合）。
	if inv.jsonGiven {
		inv.metaAddJSON("face", jstr(codeFaceWorktree))
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
		markCodeFindTruncated(inv, aggRows, cut)
		markCodeFindFace(inv, aggRows)
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
	// ★ 分母是 `emittedMatches`（已列进一页的**命中**数）而非 `len(rows)`：上下文档里一页行数 ≠ 命中数
	//   （缺口 `GAP-20260927-250`）。默认档 `emittedMatches == len(rows)` ⇒ 读数与改前逐字同。
	cut := emittedMatches < hits
	if cut {
		fmt.Fprintf(stderr, "（本页只列前 %d 条 —— 收窄：--path / --glob / --limit）", emittedMatches)
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
		inv.warnf("已裁 %d 条（code find 一页 %d 条 / 真命中 %d 条）", hits-emittedMatches, emittedMatches, hits)
		inv.metaAddJSON("truncated_detail", fmt.Sprintf(
			`{"cut_from":"tail","kept_items":%d,"dropped_items":%d,"total_items":%d}`,
			emittedMatches, hits-emittedMatches, hits))
	}
	markCodeFindTruncated(inv, rows, cut)
	markCodeFindFace(inv, rows)
	rc := listCmd(inv, stdout, stderr, []string{"path", "line", "text"}, rows)
	if cut {
		// 人面末行：表格之后再钉一句 —— 「这一页不是全集」不许靠读者自己推。
		fmt.Fprintf(stderr,
			"%s: ⚠ 本页只列前 %d 条 · 真命中 %d 条（已裁 %d 条）—— **这不是全集**，别据此下「有/无」结论；要收窄：--path / --glob / --limit\n",
			progName, emittedMatches, hits, hits-emittedMatches)
	}
	return rc
}

// markCodeFindTruncated —— `GAP-20260928-118`（2026-09-28 · 本枚）：把**跑级**截断真值落到字段面上。
//
// 病灶（现读）：截断此前只有一个读口 —— 顶层包封的 `truncated`（+ `warnings[]` + `meta.truncated_detail`）；
// 字段面（`--json <字段>`）合法名字仅 `path,line,text`，点名 `truncated` 直接报未知字段 ⇒ 只想看截断位
// 的机检脚本在字段面上**拿不到直读口**（本命令三档面：默认 / `--count` / `--files-only`）。
//
// 口径（**只加新键 · 不动既有任何格 · 不新旗标**）：
//
//	· 值 = 本跑**那一档面自己的**截断真值（默认面 = 行数被一页裁掉；聚合两档 = 件数被裁掉）——
//	  与置 `inv.markTruncated()` 的**同一个** `cut` 变量同源，**不另算一遍** ✗；
//	· 逐行写（`items` 里每一格同一值：这是跑级真值，不是一个条目自己的属性 —— 注解写清，不冒充行属性）；
//	· **只在 `--json` 那一态落键**：非 JSON 面（表格）与不给这一格的调用**逐字节不变**；
//	· 值与包封 `truncated` 恒同真（不给这一格时包封那一格照旧，两处不是两套判据）。
func markCodeFindTruncated(inv *invocation, rows []map[string]string, cut bool) {
	if inv == nil || !inv.jsonGiven {
		return
	}
	v := "false"
	if cut {
		v = "true"
	}
	for _, r := range rows {
		r["truncated"] = v
	}
}

// markCodeFindFace —— `--json` 那一态给每条目落一格 `face`（缺口 `GAP-20260929-29` 的第二半）。
//
// 与 `markCodeFindTruncated` 同一形态（逐行写 · 跑级真值 · 只在 `--json` 落键）⇒ 不给这一格时
// 输出逐字节不变。本命令走 `filepath.Walk` **直读工作树**（tracked + 未跟踪件）⇒ `face` 恒为
// `codeFaceWorktree`；同仓另一把「词在哪儿」的正门（`impact` 的 ④a）走**索引面**（`codeFaceIndex`）。
// 零命中那一档同样落键（**零命中 ≠ 没有** —— 正要知道读的是哪个面）。
func markCodeFindFace(inv *invocation, rows []map[string]string) {
	if inv == nil || !inv.jsonGiven {
		return
	}
	for _, r := range rows {
		r["face"] = codeFaceWorktree
	}
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
	if !inv.hasFlag("--batch") {
		if len(inv.args) == 0 || strings.TrimSpace(inv.args[0]) == "" {
			inv.setErr("usage", "missing_target", "缺 <件:行>")
			fmt.Fprintf(stderr, "%s: `code show` 要给 <件:行>（例：zerg code show core/cmd/zerg/main.go:100）或纯件名整件读\n", progName)
			fmt.Fprintf(stderr, "用法：zerg code show <件:行 | 件:起-止 | 件> [--ctx <N>] [--batch <件:行,…>] [--json <字段>]\n")
			return exitUsage
		}
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
	// ★ `--batch`（缺口 `GAP-20260927-233` · 2026-10-04）：一发取**多处** `件:行`（逗号分隔）。
	if inv.hasFlag("--batch") {
		return cmdCodeShowBatch(inv, stdout, stderr, root, ctx)
	}
	target := strings.TrimSpace(inv.args[0])
	// ★ 缺口 `GAP-20261001-59`（本枚 · P1）：**纯件名 = 整件读**（Hermes 侧 `read_file` 在 CLI 的正门）。
	// 判据：目标里**没有冒号** ⇒ 整件读；有冒号仍走 `<件:行 | 件:起-止>`（既有两档一字不动）。
	if !strings.Contains(target, ":") {
		return codeShowWholeFile(inv, stdout, stderr, root, target)
	}
	rows, header, rc := codeShowResolveOne(inv, stderr, root, target, ctx, inv.hasFlag("--ctx"))
	if rc != exitOK {
		return rc
	}
	fmt.Fprint(stderr, header)
	return listCmd(inv, stdout, stderr, []string{"line", "text"}, rows)
}

// codeShowWholeFile —— `zerg code show <件>`：**整件读**（缺口 `GAP-20261001-59` · 本枚 · P1）。
//
// 为什么：`code show` 此前**强制** `<件:行>`（不给行号退 2，`--full` 也同退 2）；`code find` 只管找、
// 不管读 ⇒ Hermes 侧 `read_file`（整件读）在虫族 CLI **无正门**，子代理读整件只能绕过 CLI，取证链
// 落在 CLI 之外。此举照「能走 CLI 就走」给它一条正门（不加新命令、不新开族）。
//
// 口径（**照本命令既有退码族**，不自造）：`0` 出整件 · `1` 件不在（**不是错**）· `2` 用法错（出仓 /
// 点的是目录）· `8` 读不动 / 不是文本（**不给结论**）。一页上限 `codeShowFileRowCap`（照 `read_file`
// 的 2000 行口径）⇒ 超限截断并**自报**（`truncated` + `warnings[]` + `meta.truncated_detail`）。
// `--ctx` 在整件档无意义（无被点的行）⇒ 忽略（不报错，避免与单行档两套判法）。
func codeShowWholeFile(inv *invocation, stdout, stderr io.Writer, root, rel string) int {
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
	lines := strings.Split(string(b), "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	nrel := filepath.ToSlash(clean)
	total := len(lines)
	cut := total > codeShowFileRowCap
	if cut {
		lines = lines[:codeShowFileRowCap]
	}
	fmt.Fprintf(stderr, "%s: %s 整件读（共 %d 行）\n", progName, nrel, total)
	if cut {
		inv.markTruncated()
		inv.warnf("已裁 %d 行（code show 整件读一页 %d 行 / 全件 %d 行）", total-len(lines), len(lines), total)
		inv.metaAddJSON("truncated_detail", fmt.Sprintf(
			`{"cut_from":"tail","kept_items":%d,"dropped_items":%d,"total_items":%d}`,
			len(lines), total-len(lines), total))
	}
	rows := make([]map[string]string, 0, len(lines))
	for i, ln := range lines {
		rows = append(rows, map[string]string{
			"path":   nrel,
			"line":   strconv.Itoa(i + 1),
			"text":   codeTextCell(inv, ln),
			"target": "no",
		})
	}
	return listCmd(inv, stdout, stderr, []string{"line", "text"}, rows)
}

// codeShowResolveOne —— 解析**一个** `<件:行 | 件:起-止>` 目标并取出它的邻域行（单目标路与
// `--batch` 路**共用这一份口径** —— 两处不许各判一遍 · 缺口 `GAP-20260927-233`）。
//
// 返回 `(rows, header, rc)`：`rc == exitOK` 时 `header` 是那行人面抬头
// （`<件> 第 N 行（共 M 行 · 窗口 ±ctx）`）、`rows` 是邻域逐行；`rc != exitOK` 时已写好 stderr
// 与 `inv.setErr`（退码族与单目标档**逐字同**：`1` 件不在 · `2` 用法错 · `8` 读不动/非文本）。
func codeShowResolveOne(inv *invocation, stderr io.Writer, root, target string, ctx int, ctxGiven bool) ([]map[string]string, string, int) {
	colon := strings.LastIndex(target, ":")
	if colon <= 0 || colon == len(target)-1 {
		inv.setErr("usage", "bad_target", "目标不是 <件:行>")
		fmt.Fprintf(stderr, "%s: 目标要写成 <件:行>（没冒号或没给行号）：%s ⇒ 退码 2\n", progName, target)
		fmt.Fprintf(stderr, "例：zerg code show core/go.mod:12\n")
		return nil, "", exitUsage
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
			return nil, "", exitUsage
		}
		lineNo = n
	} else if lineNo > endNo {
		inv.setErr("usage", "bad_range", "区间起点比终点大")
		fmt.Fprintf(stderr, "%s: 区间的起点比终点大：%s ⇒ 退码 2\n", progName, spec)
		return nil, "", exitUsage
	}
	// 区间档的窗口默认半径 0（区间本身就是**显式**的起止 ⇒ 不再自动外扩；
	// `--ctx` **给过**才向两端扩）—— 与 `--ttl`/`--prefix` 同一条口径：「给过旗标」与「没给」是两件事。
	if isRange && !ctxGiven {
		ctx = 0
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		inv.setErr("usage", "path_outside_repo", "件出仓")
		fmt.Fprintf(stderr, "%s: 件出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, rel, root)
		return nil, "", exitUsage
	}
	abs := filepath.Join(root, clean)
	if !strings.HasPrefix(abs, root) {
		inv.setErr("usage", "path_outside_repo", "件出仓")
		fmt.Fprintf(stderr, "%s: 件出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, rel, root)
		return nil, "", exitUsage
	}
	st, serr := os.Stat(abs)
	if serr != nil {
		inv.setErr("failed", "file_absent", "件不在")
		fmt.Fprintf(stderr, "%s: 件不在：%s —— **这不是错，是「没有」**（退码 1）\n", progName, rel)
		return nil, "", exitFail
	}
	if st.IsDir() {
		inv.setErr("usage", "target_is_dir", "点的是目录")
		fmt.Fprintf(stderr, "%s: 点的是目录、不是件：%s（先 `code find` 定位到件 · 退码 2）\n", progName, rel)
		return nil, "", exitUsage
	}
	b, rerr := os.ReadFile(abs)
	if rerr != nil {
		inv.setErr("blocked", "file_unreadable", "件读不动")
		fmt.Fprintf(stderr, "%s: 件读不动（%v）⇒ 不给结论（退码 8）\n", progName, rerr)
		return nil, "", exitBlocked
	}
	if !utf8.Valid(b) {
		inv.setErr("blocked", "not_text", "件不是文本")
		fmt.Fprintf(stderr, "%s: 件不是文本（非 UTF-8 ⇒ 不猜它的行号）⇒ 不给结论（退码 8）\n", progName)
		return nil, "", exitBlocked
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
		return nil, "", exitUsage
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
	var header string
	if isRange {
		header = fmt.Sprintf("%s: %s 第 %d-%d 行（共 %d 行 · 窗口 ±%d）\n", progName, nrel, lineNo, endNo, len(lines), ctx)
	} else {
		header = fmt.Sprintf("%s: %s 第 %d 行（共 %d 行 · 窗口 ±%d）\n", progName, nrel, lineNo, len(lines), ctx)
	}
	return rows, header, exitOK
}

// cmdCodeShowBatch —— `zerg code show --batch <件:行,…>`（缺口 `GAP-20260927-233` · 2026-10-04）：
// 一发取**多处** `件:行`（逗号分隔，每一项走与单目标路**同一份**口径 `codeShowResolveOne`）——
// 批量取证过去只能靠手搓多次 `sed`/`read_file`（见缺口症状：守卫把多行取件拦成单件正门）。
//
// 口径（照本命令既有的退码族，不自造）：
//
//	· 与**位置参数**二选一（同时给 ⇒ 退码 2）—— 两条路各有一条正门；
//	· 清单里的**空项**（连续逗号 / 收尾逗号）**不许静默跳过** ⇒ 退码 2 并逐字点名；
//	· 任一项目失败 ⇒ **整发不给结论**（返回那一项的退码：`1` 件不在 · `2` 用法错 · `8` 读不动）——
//	  不「部分成功假装成功」（与单目标档同一退码族）；
//	· 输出**带 `path` 列**（多件必须能分辨是哪个件）+ `line/text/target`；`--json <字段>` 照旧投影。
func cmdCodeShowBatch(inv *invocation, stdout, stderr io.Writer, root string, ctx int) int {
	raw := strings.TrimSpace(inv.flagVal("--batch"))
	if raw == "" {
		inv.setErr("usage", "bad_batch", "--batch 的值是空")
		fmt.Fprintf(stderr, "%s: `--batch` 要给一个逗号分隔的 <件:行> 清单（例：zerg code show --batch a.go:1,b.go:2）⇒ 退码 2\n", progName)
		return exitUsage
	}
	if len(inv.args) > 0 && strings.TrimSpace(inv.args[0]) != "" {
		inv.setErr("usage", "batch_with_target", "--batch 与位置参数不许同时给")
		fmt.Fprintf(stderr, "%s: `--batch` 与位置参数 <件:行> 不许同时给（二选一）⇒ 退码 2\n", progName)
		return exitUsage
	}
	items := strings.Split(raw, ",")
	for _, it := range items {
		if strings.TrimSpace(it) == "" {
			inv.setErr("usage", "bad_batch", "清单里有空项")
			fmt.Fprintf(stderr, "%s: `--batch` 清单里有空项（连续逗号 / 收尾逗号）⇒ 退码 2：%q\n", progName, raw)
			return exitUsage
		}
	}
	var all []map[string]string
	var headers []string
	for _, it := range items {
		one := strings.TrimSpace(it)
		rows, header, rc := codeShowResolveOne(inv, stderr, root, one, ctx, inv.hasFlag("--ctx"))
		if rc != exitOK {
			fmt.Fprintf(stderr, "%s: `--batch` 里这一项失败（%s）⇒ 整发不给结论（退码 %d）\n", progName, one, rc)
			return rc
		}
		all = append(all, rows...)
		headers = append(headers, header)
	}
	for _, h := range headers {
		fmt.Fprint(stderr, h)
	}
	return listCmd(inv, stdout, stderr, []string{"path", "line", "text", "target"}, all)
}

// codeShowDefaultCtx —— `code show` 默认窗口半径（±3 行 = 「一眼看得到上下文」的最小面）。
const codeShowDefaultCtx = 3

// codeShowFileRowCap —— `code show <件>`（整件读）一页最多多少行（缺口 `GAP-20261001-59` · 本枚）。
// 照 Hermes 侧 `read_file` 的 2000 行口径；超限**截断并自报**（不静默、不当全集）。
const codeShowFileRowCap = 2000

// codeTextCell —— `code find` / `code show` 的行文本格（缺口 `GAP-20260927-09` · 2026-09-27）。
//
// 病根（父代理现场实测）：`zerg code show gateway/fleet.yaml:115` 出到 `…mmproj: "/Users/<user>/zerg-models/O…`
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

// codeFindCtxN —— 读 `--after` / `--before` 的**非负整数**上下文半径（缺口 `GAP-20260927-250` · 本枚）。
//
// 给过旗标但值为空 / 非非负整数 ⇒ 返回非空错误串（调用方退 2）；没给 ⇒ `(0, "")`。
// 与同族 `--limit`/`--ctx` 同一条口径：判在扫之前 ⇒ 零副作用；坏值逐字点名。
func codeFindCtxN(inv *invocation, name string) (int, string) {
	if !inv.hasFlag(name) {
		return 0, ""
	}
	v := strings.TrimSpace(inv.flagVal(name))
	if v == "" {
		return 0, name + " 要给非负整数（值空）"
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, name + " 要给非负整数（得到 " + strconv.Quote(v) + "）"
	}
	return n, ""
}

// codeLineKind —— 「代码 / 注释」分流面（缺口 `GAP-20260927-251` · 本枚）。
//
// 口径：**按行**判（本命令的命中是「行」级）—— 去掉行首空白后，若以常见注释起始符开头 ⇒ `comment`，
// 否则 `code`。覆盖 Go/C/C++/Rust/JS（`//`·`/*`·`*`·`*/`）· Python/shell/yaml（`#`）· HTML（`<!--`）·
// SQL/Lua/Haskell（`--`）· ini/lisp（`;`）· 纯文本分隔（`---`/`"""` 归 code，不猜块注释边界）。
// 已知边界（如实写）：**行内注释**（`x := 1 // …`）与跨行块注释的中间行判不出 ⇒ 这一格是**行级近似**，
// 不是词法分析；机器面拿它当「大概在代码还是注释」的第一刀，精确判定仍归人眼/编辑器。
func codeLineKind(ln string) string {
	t := strings.TrimSpace(ln)
	if t == "" {
		return "code"
	}
	for _, p := range []string{"//", "#", "/*", "*/", "*", "<!--", "--", ";"} {
		if strings.HasPrefix(t, p) {
			return "comment"
		}
	}
	return "code"
}

// codeFindRow —— `code find` 一行条目（缺口 `GAP-20260927-250/251` · 本枚）。
//
// `path`/`line`/`text` 与改前同形；另落 `kind`（代码/注释分流面 · 恒有）与 `match`（上下文档里
// `yes`=命中行 · `no`=邻域行 · **默认档不落此键**）。`listCmd` 只投影点名的格 ⇒ 不给这些格时
// 输出**逐字节不变**（含人面表格：人面按 `display` 投影）。
func (inv *invocation) codeFindRow(rel string, lineNo int, ln, match string) map[string]string {
	m := map[string]string{
		"path": rel,
		"line": strconv.Itoa(lineNo),
		"text": codeTextCell(inv, ln),
		"kind": codeLineKind(ln),
	}
	if match != "" {
		m["match"] = match
	}
	return m
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

// -------- 「面」的机器格与两面断言（缺口 `GAP-20260929-29` + `GAP-20260927-55`）----------------

// codeFaceWorktree / codeFaceIndex —— 同仓两把「词在哪儿」的正门各自读的**面**（缺口 `GAP-20260929-29`）。
//
//	· 工作树面（`codeFaceWorktree`）：**tracked + 未跟踪**件的**工作树内容**（非快照），
//	  目录闸取 `codeScanSkipDirs` ∪ 点项规则、件级闸取 `codeScanSkipFile`、排 `>2MB` 与非 UTF-8。
//	  本命令 `code find` 与 `codeWorktreeFaceHits` 走这一面。
//	· 索引面（`codeFaceIndex`）：只有 **tracked** 件的**工作树内容**（`git grep` 的实义 ——
//	  不是 `--cached` 快照），未跟踪件不在面内。`impact` 的 ④a 走这一面。
//
// 「面」必须是**显式读数**：同一件、同一个词，两面的答案可以不同 ⇒ 零命中 ≠ 「没有」。二者与
// `codeScanSkipDirs` 同处一件，`GAP-20260927-55` 的 ⑤c『两处共用同一枚排除表、禁抄一份』由此落地。
const (
	codeFaceWorktree = "工作树(tracked+未跟踪)"
	codeFaceIndex    = "索引(tracked)"
)

// codeWorktreeFaceHits —— **工作树面**取「一个词」在仓内的命中行数（缺口 `GAP-20260927-55` 的 ④a 改法用）。
//
// 为什么它在本件里：统一口径要求『两处必须共用同一枚排除表、禁抄一份』⇒ 排除表 `codeScanSkipDirs` 与
// 件级闸 `codeScanSkipFile` 只有**一处**真源（就是本件）；`impact` ④a 要换到工作树面就调这一枚，
// **不再第二次实现**（也就不会漂出第二套口径）。
//
// 口径与 `cmdCodeFind` 的 walk **逐条同一**（目录闸 ∪ 件级闸 ∪ `>2MB` ∪ 非 UTF-8），只数**行命中数**、
// 不建行清单 ⇒ 不碰 `cmdCodeFind` 的任何既有行为（本函数纯增）。
func codeWorktreeFaceHits(root, word string) (int, error) {
	re, err := regexp.Compile(regexpQuote(word))
	if err != nil {
		return 0, err
	}
	hits := 0
	walkErr := filepath.Walk(root, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			return nil // 只读面：读不动的子树跳过（与 `cmdCodeFind` 同一条判法）
		}
		if info.IsDir() {
			dotDir := strings.HasPrefix(info.Name(), ".") && !codeScanKeepDots[info.Name()]
			if codeScanSkipDirs[info.Name()] || dotDir {
				return filepath.SkipDir
			}
			return nil
		}
		if codeScanSkipFile(root, p) {
			return nil
		}
		if info.Size() > codeFindMaxFileBytes {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil || !utf8.Valid(b) {
			return nil
		}
		for _, ln := range strings.Split(string(b), "\n") {
			if re.MatchString(ln) {
				hits++
			}
		}
		return nil
	})
	if walkErr != nil {
		return 0, walkErr
	}
	return hits, nil
}

// codeFaceMismatchNote —— 「两面不一致必须点名」的断言（缺口 `GAP-20260927-55` 的第二半）。
//
// 判据：同一个词在**工作树面**与**索引面**的答案不一致时，必须**点名**说出「差因是面」，
// 而不是把一个 0 当「没有」。两面一致 ⇒ 回空串（不点名 ⇒ 调用方不产生任何输出）。
func codeFaceMismatchNote(word string, worktreeHits, indexHits int) string {
	if worktreeHits == indexHits {
		return ""
	}
	return fmt.Sprintf(
		"⚠ 两面不一致：词 %q 在**工作树面**(%d 条)与**索引面**(%d 条)答案不同 —— 差因是「面」，不是「有没有」；未跟踪件只在工作树面（%s）内 ⇒ 判影响请用 `zerg code find`",
		word, worktreeHits, indexHits, codeFaceWorktree)
}

// -------- E 族 R5 四枚新面（缺口 GAP-20260927-235/236/237 · GAP-20260928-30）----------------
//
// 本节把「码面」的**只读诊断/复算**四档落进实现件。★ 四枚只用**位置参数**与**已在册的全局
// 旗标**（`--path`/`--glob`/`--limit`/`--json`/`--full`），**不新开解析分叉、不动 main.go 的
// 名字表**；命令树登记（main.go 那张 `commands = []*command{…}` 表）由父代理**串行**落
// —— 本件只落实现件（登记片段见回执）。行为面须重编后可见。

// cmdCodeCallers —— `zerg code callers <函数名>`（缺口 `GAP-20260927-237` · P1 · 只读）。
//
// 病（缺口逐字）：包装函数到**全部调用点**的反查只能靠全仓搜加人工。
// 面：`path/line/role/text`，`role` ∈ {`call`（调用点）,`def`（定义行）}。
// 退码：`0` 有命中 · `1` 零命中（**不是错，是「没有」**）· `2` 用法错（缺名 / 出仓 / 坏 glob/limit）· `8` 仓根/面读不到。
//
// 旗标名 `--func`（缺口逐字点名）：与位置参数**两收**（给了 --func 优先）；`--func` 若要真能被
// 解析还须在 main.go 名字表登记（未登记时用位置参数形态即可 —— 与命令树一并落）。
func cmdCodeCallers(inv *invocation, stdout, stderr io.Writer) int {
	name := strings.TrimSpace(inv.flagVal("--func"))
	if name == "" && len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if name == "" {
		inv.setErr("usage", "missing_func", "缺函数名")
		fmt.Fprintf(stderr, "%s: `code callers` 要给函数名（例：%s code callers requireFields 或 %s code callers --func requireFields）⇒ 退码 2\n", progName, progName, progName)
		fmt.Fprintf(stderr, "用法：%s code callers <函数名> [--path <目录或单件>] [--glob <模式>] [--limit <N>] [--json <字段>]\n", progName)
		return exitUsage
	}
	if bad := foreignFlag(inv, "--func", "--path", "--glob", "--limit", "--full", "--timeout"); bad != "" {
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, bad)
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 查不了调用点（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	base := root
	if sub := strings.TrimSpace(inv.flagVal("--path")); sub != "" {
		if filepath.IsAbs(sub) {
			base = filepath.Clean(filepath.FromSlash(sub))
		} else {
			base = filepath.Join(root, filepath.FromSlash(sub))
			if !pathWithinDir(root, base) {
				inv.setErr("usage", "path_outside_repo", "--path 出仓")
				fmt.Fprintf(stderr, "%s: --path 出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, sub, root)
				return exitUsage
			}
		}
		if st, serr := os.Stat(base); serr != nil || (!st.IsDir() && !st.Mode().IsRegular()) {
			inv.setErr("usage", "path_not_dir", "--path 既不是件也不是目录")
			fmt.Fprintf(stderr, "%s: --path 指的既不是件也不是目录：%s ⇒ 退码 2\n", progName, sub)
			return exitUsage
		}
	}
	glob := strings.TrimSpace(inv.flagVal("--glob"))
	if glob != "" {
		if _, gerr := filepath.Match(glob, ""); gerr != nil {
			inv.setErr("usage", "bad_glob", "--glob 模式非法")
			fmt.Fprintf(stderr, "%s: --glob 模式非法（模式 %q）：%v ⇒ 退码 2\n", progName, glob, gerr)
			return exitUsage
		}
	}
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
	// 定义行形态：`func <名>(` 或 `func (recv) <名>(`；调用行形态：`<名>(`（词边界）。
	defRe := regexp.MustCompile(`^\s*func\s*(\([^)]*\)\s*)?` + regexp.QuoteMeta(name) + `\s*\(`)
	callRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\(`)
	rows := []map[string]string{}
	hits, scanned, skippedBig := 0, 0, 0
	walkErr := filepath.Walk(base, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			return nil // 只读面：读不动的子树跳过（与 `cmdCodeFind` 同判）
		}
		if info.IsDir() {
			dotDir := strings.HasPrefix(info.Name(), ".") && !codeScanKeepDots[info.Name()]
			if codeScanSkipDirs[info.Name()] || dotDir {
				return filepath.SkipDir
			}
			return nil
		}
		if codeScanSkipFile(root, p) {
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
			return nil
		}
		scanned++
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)
		for i, ln := range strings.Split(string(b), "\n") {
			if !callRe.MatchString(ln) {
				continue
			}
			hits++
			if len(rows) >= rowCap {
				continue
			}
			role := "call"
			if defRe.MatchString(ln) {
				role = "def"
			}
			rows = append(rows, map[string]string{
				"path": rel,
				"line": strconv.Itoa(i + 1),
				"role": role,
				"text": codeTextCell(inv, ln),
			})
		}
		return nil
	})
	if walkErr != nil {
		inv.setErr("blocked", "walk_failed", "扫不动")
		fmt.Fprintf(stderr, "%s: 扫不动（%v）⇒ 不给结论（退码 8）\n", progName, walkErr)
		return exitBlocked
	}
	fmt.Fprintf(stderr, "%s: 扫了 %d 件 · 命中 %d 条", progName, scanned, hits)
	if hits > len(rows) {
		fmt.Fprintf(stderr, "（本页只列前 %d 条 —— 收窄：--path / --glob / --limit）", len(rows))
	}
	if skippedBig > 0 {
		fmt.Fprintf(stderr, " · 跳过 >%dMB 的件 %d 个", codeFindMaxFileBytes>>20, skippedBig)
	}
	fmt.Fprintln(stderr)
	if hits == 0 {
		inv.setErr("failed", "no_match", "没有调用点")
		fmt.Fprintf(stderr, "%s: 没找到 %q 的调用点 —— 这不是错，是「没有」（退码 1）\n", progName, name)
		return exitFail
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i]["path"] != rows[j]["path"] {
			return rows[i]["path"] < rows[j]["path"]
		}
		return atoiSafe(rows[i]["line"]) < atoiSafe(rows[j]["line"])
	})
	return listCmd(inv, stdout, stderr, []string{"path", "line", "role", "text"}, rows)
}

// codeDenomRecountParse —— 读一份「件清单」：一行一件（去空白；空行与 `#` 开头忽略）。
func codeDenomRecountParse(p string) ([]string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, ln := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(ln)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// cmdCodeDenomRecount —— `zerg code denom-recount <左件> <右件>`（缺口 `GAP-20260927-236` · P1 · 只读）。
//
// 病（缺口逐字）：文件计数出现 873/874 差 1 时，无正门能**定点名**那 1 件。
// 面：两份件清单（一行一件）的**对称差** —— `path/side`，`side` ∈ {`只在左`,`只在右`}。
// 退码：`0` 两清单**同集**（差 0）· `1` 有差（已定点名差的件）· `2` 用法错 · `8` 清单读不到。
func cmdCodeDenomRecount(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) < 2 || strings.TrimSpace(inv.args[0]) == "" || strings.TrimSpace(inv.args[1]) == "" {
		inv.setErr("usage", "missing_lists", "缺两份件清单")
		fmt.Fprintf(stderr, "%s: `code denom-recount` 要给两份件清单（一行一件）：%s code denom-recount <左件> <右件> ⇒ 退码 2\n", progName, progName)
		return exitUsage
	}
	if bad := foreignFlag(inv, "--timeout"); bad != "" {
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, bad)
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	resolve := func(rel string) (string, bool) {
		clean := filepath.Clean(filepath.FromSlash(rel))
		if filepath.IsAbs(clean) {
			return clean, true // 显式绝对路径 = 另一根（例：仓外台账）
		}
		abs := filepath.Join(root, clean)
		return abs, pathWithinDir(root, abs)
	}
	la, ok := resolve(strings.TrimSpace(inv.args[0]))
	if !ok {
		inv.setErr("usage", "path_outside_repo", "左清单出仓")
		fmt.Fprintf(stderr, "%s: 左清单出仓了（%s）⇒ 退码 2\n", progName, inv.args[0])
		return exitUsage
	}
	ra, ok := resolve(strings.TrimSpace(inv.args[1]))
	if !ok {
		inv.setErr("usage", "path_outside_repo", "右清单出仓")
		fmt.Fprintf(stderr, "%s: 右清单出仓了（%s）⇒ 退码 2\n", progName, inv.args[1])
		return exitUsage
	}
	lLines, lerr := codeDenomRecountParse(la)
	if lerr != nil {
		inv.setErr("blocked", "list_unreadable", "左清单读不到")
		fmt.Fprintf(stderr, "%s: 左清单读不到（%v）⇒ 不给结论（退码 8）\n", progName, lerr)
		return exitBlocked
	}
	rLines, rerr := codeDenomRecountParse(ra)
	if rerr != nil {
		inv.setErr("blocked", "list_unreadable", "右清单读不到")
		fmt.Fprintf(stderr, "%s: 右清单读不到（%v）⇒ 不给结论（退码 8）\n", progName, rerr)
		return exitBlocked
	}
	ls := map[string]bool{}
	for _, x := range lLines {
		ls[x] = true
	}
	rs := map[string]bool{}
	for _, x := range rLines {
		rs[x] = true
	}
	onlyL, onlyR := []string{}, []string{}
	for x := range ls {
		if !rs[x] {
			onlyL = append(onlyL, x)
		}
	}
	for x := range rs {
		if !ls[x] {
			onlyR = append(onlyR, x)
		}
	}
	sort.Strings(onlyL)
	sort.Strings(onlyR)
	fmt.Fprintf(stderr, "%s: 左 %d 件 · 右 %d 件 · 只在左 %d · 只在右 %d\n", progName, len(lLines), len(rLines), len(onlyL), len(onlyR))
	if len(onlyL) == 0 && len(onlyR) == 0 {
		fmt.Fprintf(stderr, "%s: 两清单**同集**（差 0）—— 复算一致（退码 0）\n", progName)
		return exitOK
	}
	rows := []map[string]string{}
	for _, x := range onlyL {
		rows = append(rows, map[string]string{"path": x, "side": "只在左"})
	}
	for _, x := range onlyR {
		rows = append(rows, map[string]string{"path": x, "side": "只在右"})
	}
	rc := listCmd(inv, stdout, stderr, []string{"path", "side"}, rows)
	if rc == exitOK {
		rc = exitFail // 有差 ⇒ 1（形如 `diff`：相等才 0）
	}
	return rc
}

// codeGitRun —— 跑一次 git（`-C <root>` + args），返回逐行 stdout（去尾空行）。
func codeGitRun(root string, args []string) ([]string, error) {
	full := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", full...)
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	s := strings.TrimRight(out.String(), "\n")
	if s == "" {
		return []string{}, nil
	}
	return strings.Split(s, "\n"), nil
}

// cmdCodeGitTwoWay —— `zerg code git-two-way <全局选项> -- <git 参数…>`（缺口 `GAP-20260927-235` · P1 · 只读）。
//
// 病（缺口逐字）：无正门做「同一 git 命令带或不带某全局选项两版对比」的诊断复算（现有 `code find`
// 是词法面复算、非此诊断）⇒ 每次都要手搓脚本直调 git。
//
// 口径：第一位置参数 = **全局选项**（`k=v` 走 `git -c k=v …`；以 `-` 开头的开关原样前置）；
// 其余 = **git 参数**（含 git 自身的旗标时，写在 `--` 之后以防被本仓解析器当 zerg 旗标）。
// 面：两跑（无选项 / 带选项）的**差行**：`side/text`，`side` ∈ {`只在无选项`,`只在带选项`}。
// 退码：`0` 两跑一致 · `1` 有差 · `2` 用法错（缺选项/缺 git 参数）· `8` git 不可用 / 仓根读不到。
func cmdCodeGitTwoWay(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) < 2 {
		inv.setErr("usage", "missing_git_args", "缺全局选项或 git 参数")
		fmt.Fprintf(stderr, "%s: `code git-two-way` 要给「全局选项」+「git 参数」（后者建议写在 `--` 之后）⇒ 退码 2\n", progName)
		fmt.Fprintf(stderr, "用法：%s code git-two-way <k=v 或 -开关> -- <git 参数…>（例：%s code git-two-way core.quotePath=false -- ls-files)\n", progName, progName)
		return exitUsage
	}
	if bad := foreignFlag(inv, "--timeout"); bad != "" {
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, bad)
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	opt := strings.TrimSpace(inv.args[0])
	gitArgs := inv.args[1:]
	withArgs := []string{}
	if strings.HasPrefix(opt, "-") {
		withArgs = append(withArgs, opt)
	} else {
		withArgs = append(withArgs, "-c", opt)
	}
	withArgs = append(withArgs, gitArgs...)
	plain, perr := codeGitRun(root, gitArgs)
	if perr != nil {
		inv.setErr("blocked", "git_unavailable", "git 跑不动")
		fmt.Fprintf(stderr, "%s: git 跑不动（%v）⇒ 不给结论（退码 8）\n", progName, perr)
		return exitBlocked
	}
	with, werr := codeGitRun(root, withArgs)
	if werr != nil {
		inv.setErr("blocked", "git_unavailable", "git 跑不动")
		fmt.Fprintf(stderr, "%s: 带选项(%s)那一跑跑不动（%v）⇒ 不给结论（退码 8）\n", progName, opt, werr)
		return exitBlocked
	}
	pset, wset := map[string]bool{}, map[string]bool{}
	for _, l := range plain {
		pset[l] = true
	}
	for _, l := range with {
		wset[l] = true
	}
	onlyP, onlyW := []string{}, []string{}
	for _, l := range plain {
		if !wset[l] {
			onlyP = append(onlyP, l)
		}
	}
	for _, l := range with {
		if !pset[l] {
			onlyW = append(onlyW, l)
		}
	}
	fmt.Fprintf(stderr, "%s: 无选项 %d 行 · 带选项(%s) %d 行 · 只在无选项 %d · 只在带选项 %d\n", progName, len(plain), opt, len(with), len(onlyP), len(onlyW))
	if len(onlyP) == 0 && len(onlyW) == 0 {
		fmt.Fprintf(stderr, "%s: 两跑**一致** —— 该选项对这一跑无差（退码 0）\n", progName)
		return exitOK
	}
	rows := []map[string]string{}
	for _, l := range onlyP {
		rows = append(rows, map[string]string{"side": "只在无选项", "text": truncateDisplay(l, 200)})
	}
	for _, l := range onlyW {
		rows = append(rows, map[string]string{"side": "只在带选项", "text": truncateDisplay(l, 200)})
	}
	rc := listCmd(inv, stdout, stderr, []string{"side", "text"}, rows)
	if rc == exitOK {
		rc = exitFail
	}
	return rc
}

// codeReadingHint —— 一条「注释读数 → 真源取法」提示（缺口 `GAP-20260928-30`）。
type codeReadingHint struct {
	Keyword string // 读数所在行的关键词（用于把读数与真源取法配上）
	Source  string // 该读数的真源取法（怎么现读复算）
}

// codeReadingNumberRe —— 注释里「读数」的形态：一个**独立**整数（前后不被字母/数字/`./:/_#§-`
// 邻接）⇒ 排掉日期 `2026-09-29`、版本 `v1.2`、行参 `:53`、`§十一`、`#2`、`0x1f` 一类假阳骨架。
var codeReadingNumberRe = regexp.MustCompile(`(?:^|[^0-9A-Za-z._:/#§-])([0-9]+)(?:[^0-9A-Za-z._:/#§-]|$)`)

// codeAuditReadingHints —— 通用「真源取法」提示表（family_h.go 那一份由 `familyHCodeReadingHints` 续）。
func codeAuditReadingHints() []codeReadingHint {
	return append([]codeReadingHint{
		{"件", "现跑 `zerg script ls --json path` 数 items（收件口径 scriptInvScan）"},
		{"处", "现跑 `zerg code find <关键词>` 数命中条数"},
		{"行", "现跑 `zerg code show <件>` 读整件行数"},
		{"门", "现跑 `zerg gate list` 数门数（真源 `scripts/gates/`）"},
		{"排除面", "现读 `family_script_inventory.go` 的 `scriptInvExcludeDirs` 键数"},
	}, familyHCodeReadingHints()...)
}

// cmdCodeAuditReadings —— `zerg code audit-readings --path <件或目录>`（缺口 `GAP-20260928-30` · P2 · 只读）。
//
// 病（缺口逐字）：「注释里的时点读数」在机器上完全不可见 —— 门看不见注释 ⇒ 静默腐；同形病已在
// `family_h.go`/`check-*.py` 等连烂 3 轮，每轮靠人肉扫描器才扫得出。
//
// ★ 口径（**只列候选 · 不下判断**）：本命令列出注释行里的**独立整数**（读数）+ 该读数的「真源取法」
// 提示（按关键词配），**绝不**判「烂了 / 没过期」—— 缺口明说「本类判据须先设计 —— 纯启发式扫描
// 会有假阳，别造恒绿/易误报的空转闸」；判断留给后续判据。
// 面：`path/line/reading/hint/text`（`reading` = 读数原文整串）。
// 退码：`0` 列完候选（含 0 条）· `1` 该面没有注释读数（**不是错，是「没有」**）· `2` 用法错（缺 --path / 坏 glob）· `8` 面读不到。
func cmdCodeAuditReadings(inv *invocation, stdout, stderr io.Writer) int {
	sub := strings.TrimSpace(inv.flagVal("--path"))
	if sub == "" {
		inv.setErr("usage", "missing_path", "缺 --path")
		fmt.Fprintf(stderr, "%s: `code audit-readings` 要给 --path <件或目录>（例：%s code audit-readings --path core/cmd/zerg/family_h.go）⇒ 退码 2\n", progName, progName)
		return exitUsage
	}
	if bad := foreignFlag(inv, "--path", "--glob", "--limit", "--full", "--timeout"); bad != "" {
		fmt.Fprintf(stderr, "%s: 未知旗标 %q\n", progName, bad)
		fmt.Fprintf(stderr, "See '%s --help'。\n", progName)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 读不了码（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	base := root
	if filepath.IsAbs(sub) {
		base = filepath.Clean(filepath.FromSlash(sub))
	} else {
		base = filepath.Join(root, filepath.FromSlash(sub))
		if !pathWithinDir(root, base) {
			inv.setErr("usage", "path_outside_repo", "--path 出仓")
			fmt.Fprintf(stderr, "%s: --path 出仓了（%s 不在 %s 下）⇒ 退码 2\n", progName, sub, root)
			return exitUsage
		}
	}
	if st, serr := os.Stat(base); serr != nil || (!st.IsDir() && !st.Mode().IsRegular()) {
		inv.setErr("usage", "path_not_dir", "--path 既不是件也不是目录")
		fmt.Fprintf(stderr, "%s: --path 指的既不是件也不是目录：%s ⇒ 退码 2\n", progName, sub)
		return exitUsage
	}
	glob := strings.TrimSpace(inv.flagVal("--glob"))
	if glob != "" {
		if _, gerr := filepath.Match(glob, ""); gerr != nil {
			inv.setErr("usage", "bad_glob", "--glob 模式非法")
			fmt.Fprintf(stderr, "%s: --glob 模式非法（模式 %q）：%v ⇒ 退码 2\n", progName, glob, gerr)
			return exitUsage
		}
	}
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
	hints := codeAuditReadingHints()
	rows := []map[string]string{}
	scanned, skippedBig, readings := 0, 0, 0
	walkErr := filepath.Walk(base, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			return nil
		}
		if info.IsDir() {
			dotDir := strings.HasPrefix(info.Name(), ".") && !codeScanKeepDots[info.Name()]
			if codeScanSkipDirs[info.Name()] || dotDir {
				return filepath.SkipDir
			}
			return nil
		}
		if codeScanSkipFile(root, p) {
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
			return nil
		}
		scanned++
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)
		for i, ln := range strings.Split(string(b), "\n") {
			if codeLineKind(ln) != "comment" {
				continue
			}
			nums := codeReadingNumberRe.FindAllStringSubmatch(ln, -1)
			if len(nums) == 0 {
				continue
			}
			hint := ""
			for _, h := range hints {
				if strings.Contains(ln, h.Keyword) {
					hint = h.Keyword + " ⇒ " + h.Source
					break
				}
			}
			for _, m := range nums {
				readings++
				if len(rows) >= rowCap {
					continue
				}
				rows = append(rows, map[string]string{
					"path":    rel,
					"line":    strconv.Itoa(i + 1),
					"reading": m[1],
					"hint":    hint,
					"text":    codeTextCell(inv, ln),
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
	fmt.Fprintf(stderr, "%s: 扫了 %d 件 · 注释读数候选 %d 条", progName, scanned, readings)
	if readings > len(rows) {
		fmt.Fprintf(stderr, "（本页只列前 %d 条 —— 收窄：--path / --glob / --limit）", len(rows))
	}
	if skippedBig > 0 {
		fmt.Fprintf(stderr, " · 跳过 >%dMB 的件 %d 个", codeFindMaxFileBytes>>20, skippedBig)
	}
	fmt.Fprintln(stderr)
	fmt.Fprintf(stderr, "（只列候选 · **不下「烂了/没过期」的判断** —— 判断须先设计（缺口 GAP-20260928-30），防假阳）\n")
	if readings == 0 {
		inv.setErr("failed", "no_match", "没有注释读数")
		fmt.Fprintf(stderr, "%s: 该面没有注释读数 —— 这不是错，是「没有」（退码 1）\n", progName)
		return exitFail
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i]["path"] != rows[j]["path"] {
			return rows[i]["path"] < rows[j]["path"]
		}
		return atoiSafe(rows[i]["line"]) < atoiSafe(rows[j]["line"])
	})
	return listCmd(inv, stdout, stderr, []string{"path", "line", "reading", "hint", "text"}, rows)
}
