// who_references.go —— 判据件 `scripts/gates/who-references.py`（397 行）的 **Go 真搬**
// （v2.5.14 · M2 · 候选区间首字母 q–z · 入口 `whoReferencesGateMain`）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳（`zerg gate check <判据名>`）；
//	M2 = 按价值真搬 Go（落点 `core/cmd/zerg/`）。本件 = M2 一枚。
//	验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 本件与新口径（仓外树改显式入参旋钮）：
//
//	py `resolve_trees()` 指向两棵**仓外树**（Zerg-内部文档 / Zerg-归档）：默认 = 环境变量
//	ZERG_DOCS_ROOT / ZERG_ARCHIVE_ROOT，否则 = 主仓根的**兄弟目录**。本件按新口径把这层改成
//	**显式入参旋钮** `--docs-root` / `--archive-root`：
//	  ① 旋钮**缺省值 = py 的默认推导结果逐字同**（env 优先、否则兄弟目录）—— 故不传旋钮时与 py
//	     逐字对齐（两态对拍已证）；
//	  ② **取不到外部树 ⇒ 退 2「不给结论」✗ 不静默绿**：
//	     · 显式给了旋钮、但那路径不是目录 ⇒ rc=2；
//	     · 有 pending（指向仓外/已断的引用）且两棵外部树**至少一棵不在场** ⇒ rc=2（防把「已归档」
//	       误判成「已断」—— 这是 py 的静默面：树不在场时 py 照 rc=1）。
//	     注：py **无**这两个旗标（只有同名环境变量）⇒ 凡带旋钮的输入格 py 侧落 argparse「未知旗标」
//	     档（rc 相同=2、stderr 不同），该面**不属逐字对齐承诺**（见未覆盖面①）。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3448`（门55 · tri 档））；
//	· **不**删 `scripts/gates/who-references.py` 原件（退场规则：**对拍通过后才**退场）；
//	· **已接线**：入口 `whoReferencesGateMain` 由 `gateCheckGoEntry["who-references"]` 分派（`zerg gate check who-references`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `DEFAULT_ROOT = abspath(dirname(__file__)/../..)`（脚本位置）；本件
//	    **无 `__file__` 语境** ⇒ 不给 `--root` 时从 **cwd** 上溯找含 `scripts/gates` 的目录、找不到
//	    回落 abspath(cwd)。落在同一真仓、cwd=仓根时两者读数逐字相同（已对拍）；「脚本位置 ≠ cwd」
//	    时路径推导面不同 ⇒ **不属逐字对齐承诺**，对拍一律显式传根。
//	    **旋钮缺省值 vs py 默认推导逐字对比**：py `docs = os.environ.get("ZERG_DOCS_ROOT") or
//	    os.path.join(dirname(root), "Zerg-内部文档")`（arch 同构、名 `ZERG_ARCHIVE_ROOT`/`Zerg-归档`）
//	    ⟷ 本件 `wrResolveTrees`：flag 空 ⇒ env 名逐字同 ⇒ 空 ⇒ `filepath.Join(filepath.Dir(root),
//	    "Zerg-内部文档"|"Zerg-归档")` —— 表达式逐字同构、同机同 cwd 下逐字同。
//	(b) **自指检查（先真判）**：① 本件**不在**自己扫描面内 —— py `SKIP_DIRS` 含 `"core"` ⇒
//	    `core/cmd/zerg/who_references.go` 的路径段 `core` 命中跳过集，本门**永不自扫本件**（量化
//	    依据：SKIP_DIRS 共 15 项 + SKIP_PREFIX 1 项，`core` 在列）。② 本件**落在**「他门」扫描面内
//	    （`check-hardcoded-private-paths` 等的 SCAN_DIRS 含 `core/`）⇒ 已按血证处置：正文**零绝对
//	    私有路径字面量**（盘根绝对路径、用户家目录前缀一概不写；仓根一律运行时取）、**不写**
//	    `check-error-kinds` 的被扫用法形态（其错误构造调用原串此处**不复写**）、**不写**反引号包裹的
//	    自检魔法串（本件自检只造 `wr-selftest-` 临时前缀，非门扫描标记）。落仓前后已跑他门取证。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① `--docs-root` / `--archive-root`：py **无**此两旗标（只有同名 env）⇒ 带旋钮的输入格 py 落
//	   argparse「未知旗标」档（rc 同为 2、stderr 正文不同）。缺省档（不传旋钮）逐字对齐。
//	② argparse **长选项前缀缩写**（`--js`⇒`--json`）与 `--json=x` 等边界：本件不实现缩写匹配；
//	   `store_true` 型带 `=值` 报「ignored explicit argument」正文可能微差。
//	③ py argparse 依 `shutil.get_terminal_size()` 换行（本机非 tty ⇒ 80 列）；`COLUMNS` 非 80 时
//	   帮助正文换行不同 —— 本件按 80 列**逐字固化**。
//	④ py `\w`/`\s` 为 **Unicode 语义**；本件正则以 `[\p{L}\p{N}_]`（≈`\w`）与 ASCII `\s` 复刻 ⇒
//	   对含 Unicode 空白 / 组合字符的极端行，裸路径形态的匹配边界可能微差。
//	⑤ py `git ls-files` 不可用（无 git）时抛异常正文为 CPython 文案；本件打自有说明行（rc 同为 2）。
//	⑥ py `--json` 旗标**解析后从未被读取**（原件真口子）⇒ 行为与「无旗标」完全相同（人读表）；
//	   本件**逐字保留**此行为（✗ 不自行修，交回入账）。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const wrProg = "who-references.py"

// wrUsage —— argparse 在 80 列下的 usage 头三行（错误档与 `-h` 共用）。
const wrUsage = `usage: who-references.py [-h] [--root ROOT] [--kind {auto,file,dir,prefix}]
                         [--json] [--json-only] [--dump DUMP] [--self-test]
                         [path]
`

// wrHelp —— `-h`/`--help` 全文（Python 3.9 argparse · 80 列 · 逐字固化）。
const wrHelp = wrUsage + `
给一件路径 ⇒ 哪些件引用了它、引用数、多少指向已归档/已断

positional arguments:
  path                  仓内路径（件 / 目录 / 路径前缀）

optional arguments:
  -h, --help            show this help message and exit
  --root ROOT           查的仓根（默认本仓根）
  --kind {auto,file,dir,prefix}
                        目标形态（默认 auto：按是否存在目录优先判 file/dir）
  --json                机器读面
  --json-only           只 stdout 输出 JSON 本体
  --dump DUMP           把结果显式落盘（唯一被允许的写行为）
  --self-test           跑成对负控（合成夹具）
`

// wrWriteIntentFlags —— py `WRITE_INTENT_FLAGS`（逐字）。
var wrWriteIntentFlags = []string{
	"--write", "--set", "--append", "--yes", "--reason", "--force",
	"--overwrite", "--apply", "--commit", "--push",
}

// wrSkipDirs —— py `SKIP_DIRS`（逐字）。★ 含 `core` ⇒ 本件永不自扫（标准声明 (b)①）。
var wrSkipDirs = []string{"vendor", "bin", "dist", "node_modules", "target", ".git", ".venv", "venv",
	"docs", "ui", "mcp", "wall", "deploy", "zerg-evals", "compress_models",
	"styles", "data", "core", "agent"}

// wrSkipPrefix —— py `SKIP_PREFIX`（逐字）。
var wrSkipPrefix = []string{"tools/ocr/venv/"}

// wrStrLitRE —— py RE_STR_LIT 逐字复刻（内层类排除：换行 / 反斜杠 / 引号族）。
// py 编译后 pattern：["'`‘’“”]([^\n"'`‘’“”]{1,400})["'`‘’“”]
var wrStrLitRE = regexp.MustCompile("[\"'" + "`" + "‘’“”]([^\\n\"'" + "`" + "‘’“”]{1,400})[\"'" + "`" + "‘’“”]")

// wrHit —— py scan_for 每条 hit 的 dict（键序即 py dict 插入序）。
type wrHit struct {
	File        string
	Line        int
	Quote       string
	Target      string
	FoundInRepo bool
	FoundInTree bool
	TreeLabel   *string // nil = py None
}

// wrByName —— py `by_tree` 的**保序**小计（py dict 保插入序）。
type wrByName struct {
	Key string
	Val int
}

// wrSummary —— py `summary`（键序即 py dict 插入序）。
type wrSummary struct {
	Target   string
	Repo     string
	Total    int
	RefFiles int
	Pending  int
	ByTree   []wrByName
	Hits     []wrHit
}

// ── 入口 ─────────────────────────────────────────────────────────────
func whoReferencesGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① argparse 子集解析（逐字复刻退码与错误正文）。
	a, rc := wrParse(stdout, stderr, argv)
	if rc >= 0 {
		return rc
	}

	// ② `--self-test`（py：解析后**第一支**）。
	if a.SelfTest {
		return wrSelfTest(stdout)
	}

	// ③ 写意图一律拒（py：raw argv 逐字 `in` 判）。
	for _, f := range wrWriteIntentFlags {
		if wrContains(argv, f) {
			if !a.JSONOnly {
				fmt.Fprintf(stderr, "✗ %s：写意图一律拒 ⇒ rc=2 且一个字节不写\n", f)
			}
			return 2
		}
	}

	// ④ 根定位 + 仓根校验（py 在 path 判空**之前**）。
	root := a.Root
	if !a.HasRoot {
		root = wrDefaultRoot()
	}
	root, _ = wrAbs(root)
	if !wrIsDir(filepath.Join(root, "scripts", "gates")) {
		fmt.Fprintf(stderr, "✗ 不在仓根：%s ⇒ 拒绝静默空转\n", root)
		return 2
	}

	// ⑤ `path` 缺参（py：ap.error ⇒ 用法码 2）。
	if a.Path == nil {
		fmt.Fprint(stderr, wrUsage)
		fmt.Fprintf(stderr, "%s: error: 必给一件路径（用法：who-references.py <件> [--root <根>]）\n", wrProg)
		return 2
	}
	path := *a.Path

	// ⑥ 空串 / 纯空白（甲案 GAP-20260928-46 ⇒ 用法码 2）。
	if strings.TrimSpace(path) == "" {
		fmt.Fprint(stderr, "✗ 路径是空串/纯空白（参数值无效）⇒ 用法码 rc=2："+
			"空串会匹配**全部件**（空转 ≠ 零引用）\n")
		return 2
	}

	// ⑦ 路径不在仓根（py：非 `--json-only` 才打 stdout）。
	if !wrExists(filepath.Join(root, path)) {
		if !strings.Contains(path, "/") && !strings.HasSuffix(path, ".md") {
			if !a.JSONOnly {
				fmt.Fprintf(stdout, "✗ 该路径在当前仓根不存在（它可能已归档到 Zerg-内部文档/Zerg-归档）：%s\n", path)
			}
			return 2
		}
	}

	// ⑧ 外部两树：显式旋钮优先，缺省 = py 默认推导（env 或兄弟目录）。★ 新口径②：取不到 ⇒ 退 2。
	docs, arch := wrResolveTrees(root, a.DocsRoot, a.ArchiveRoot)
	if a.HasDocs && !wrIsDir(a.DocsRoot) {
		fmt.Fprintf(stderr, "✗ --docs-root 不是目录：%s ⇒ 不给结论（退 2 · 外部树取不到）\n", a.DocsRoot)
		return 2
	}
	if a.HasArch && !wrIsDir(a.ArchiveRoot) {
		fmt.Fprintf(stderr, "✗ --archive-root 不是目录：%s ⇒ 不给结论（退 2 · 外部树取不到）\n", a.ArchiveRoot)
		return 2
	}

	// ⑨ kind：auto ⇒ 按 root/path 是否目录判 dir/file。
	kind := a.Kind
	if kind == "auto" {
		if wrIsDir(filepath.Join(root, path)) {
			kind = "dir"
		} else {
			kind = "file"
		}
	}

	query := wrNormpath(wrLstripDotSlash(path))
	hits, ok := wrScanFor(stderr, root, query, kind, docs, arch)
	if !ok {
		return 2
	}
	if len(hits) == 0 {
		if !a.JSONOnly {
			fmt.Fprintf(stdout, "谁引用了 `%s`：零引用 ⇒ rc=2（不给结论：路径不在仓内面）\n", path)
		}
		return 2
	}

	pending := []wrHit{}
	for _, h := range hits {
		if !h.FoundInRepo {
			pending = append(pending, h)
		}
	}

	// ★ 新口径②：有 pending 需外部树判类、而两棵外部树**都不在场** ⇒ 退 2（✗ 不静默绿）。
	// 只判「都取不到」而非「任一取不到」：真仓 `Zerg-归档` 是**悬空符号链接**（未挂载卷）⇒ 单棵
	// 缺席是常态、py 亦照常按另一棵判；「都取不到」才真正无从判类。
	if len(pending) > 0 && !wrIsDir(docs) && !wrIsDir(arch) {
		fmt.Fprintf(stderr, "✗ 外部树取不到（%s / %s 两处都不是目录）⇒ 不给结论（退 2 · "+
			"防把「已归档」误判成「已断」）\n", docs, arch)
		return 2
	}

	byTree := []wrByName{}
	for _, h := range pending {
		key := "三树都无"
		if h.TreeLabel != nil {
			key = *h.TreeLabel
		}
		hit := false
		for i := range byTree {
			if byTree[i].Key == key {
				byTree[i].Val++
				hit = true
				break
			}
		}
		if !hit {
			byTree = append(byTree, wrByName{Key: key, Val: 1})
		}
	}

	sum := wrSummary{
		Target:   wrNormpath(path),
		Repo:     root,
		Total:    len(hits),
		RefFiles: wrCountRefFiles(hits),
		Pending:  len(pending),
		ByTree:   byTree,
		Hits:     hits,
	}

	if a.Dump != "" {
		if err := os.WriteFile(a.Dump, []byte(wrJSONIndent(sum)), 0o644); err != nil {
			fmt.Fprintf(stderr, "✗ 落不了盘 %s：%s ⇒ 不给结论\n", a.Dump, err)
			return 2
		}
	}

	if a.JSONOnly {
		fmt.Fprintln(stdout, wrJSONCompact(sum))
		if len(pending) > 0 {
			return 1
		}
		return 0
	}

	// 人读面（逐字复刻）。
	fmt.Fprintf(stdout, "谁引用了 `%s`（仓 %s）\n", path, root)
	fmt.Fprintln(stdout, strings.Repeat("─", 72))
	fmt.Fprintf(stdout, "引用总数：%d 处 · 涉及 %d 件 · 其中 %d 处将指向**已归档/已不存在的**目标\n",
		len(hits), sum.RefFiles, len(pending))
	if len(byTree) > 0 {
		parts := make([]string, 0, len(byTree))
		for _, kv := range byTree {
			parts = append(parts, fmt.Sprintf("%s=%d", kv.Key, kv.Val))
		}
		fmt.Fprintf(stdout, "  未命中落点分类：%s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintln(stdout, strings.Repeat("─", 72))
	n := len(hits)
	if n > 30 {
		n = 30
	}
	for _, h := range hits[:n] {
		status := "→已断（三树都无）"
		if h.FoundInRepo {
			status = "在仓"
		} else if h.TreeLabel != nil {
			status = "→" + *h.TreeLabel
		}
		fmt.Fprintf(stdout, "%s:%d · 引文 `%s` ⇒ %s\n", h.File, h.Line, h.Quote, status)
	}
	if len(hits) > 30 {
		fmt.Fprintf(stdout, "  …另 %d 处\n", len(hits)-30)
	}
	fmt.Fprintln(stdout, strings.Repeat("─", 72))
	if len(pending) > 0 {
		fmt.Fprintf(stdout, "结论：%d 处引用指向**已归档/已断**目标 ⇒ rc=1（有待办：改指真源现落点或登记）\n", len(pending))
		return 1
	}
	fmt.Fprint(stdout, "结论：引用全部指向仍存在的真目标 ⇒ rc=0（无待办\n")
	return 0
}

// ── argparse 子集 ───────────────────────────────────────────────────
type wrArgs struct {
	Root        string
	HasRoot     bool
	Kind        string
	JSON        bool
	JSONOnly    bool
	Dump        string
	SelfTest    bool
	Path        *string
	DocsRoot    string
	HasDocs     bool
	ArchiveRoot string
	HasArch     bool
}

// wrParse —— 返回 (args, rc)。rc>=0 ⇒ 已出结论（帮助/用法错）；rc<0 ⇒ 继续。
func wrParse(stdout, stderr io.Writer, argv []string) (wrArgs, int) {
	a := wrArgs{Kind: "auto"}
	var positional []string
	var unknown []string
	afterDD := false
	i := 0
	for i < len(argv) {
		tok := argv[i]
		i++
		if afterDD {
			positional = append(positional, tok)
			continue
		}
		if tok == "--" {
			afterDD = true
			continue
		}
		if tok == "-h" || tok == "--help" {
			fmt.Fprint(stdout, wrHelp)
			return a, 0
		}
		if strings.HasPrefix(tok, "--") {
			name := tok
			val := ""
			hasEq := false
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				name, val, hasEq = tok[:eq], tok[eq+1:], true
			}
			needVal := func() (string, bool) {
				if hasEq {
					return val, true
				}
				if i < len(argv) && !strings.HasPrefix(argv[i], "-") {
					v := argv[i]
					i++
					return v, true
				}
				return "", false
			}
			switch name {
			case "--root":
				v, ok := needVal()
				if !ok {
					return a, wrArgErr(stderr, "argument --root: expected one argument")
				}
				a.Root, a.HasRoot = v, true
			case "--docs-root":
				v, ok := needVal()
				if !ok {
					return a, wrArgErr(stderr, "argument --docs-root: expected one argument")
				}
				a.DocsRoot, a.HasDocs = v, true
			case "--archive-root":
				v, ok := needVal()
				if !ok {
					return a, wrArgErr(stderr, "argument --archive-root: expected one argument")
				}
				a.ArchiveRoot, a.HasArch = v, true
			case "--dump":
				v, ok := needVal()
				if !ok {
					return a, wrArgErr(stderr, "argument --dump: expected one argument")
				}
				a.Dump = v
			case "--kind":
				v, ok := needVal()
				if !ok {
					return a, wrArgErr(stderr, "argument --kind: expected one argument")
				}
				switch v {
				case "auto", "file", "dir", "prefix":
					a.Kind = v
				default:
					return a, wrArgErr(stderr, fmt.Sprintf(
						"argument --kind: invalid choice: '%s' (choose from 'auto', 'file', 'dir', 'prefix')", v))
				}
			case "--json":
				if hasEq {
					return a, wrArgErr(stderr, fmt.Sprintf("argument --json: ignored explicit argument '%s'", val))
				}
				a.JSON = true
			case "--json-only":
				if hasEq {
					return a, wrArgErr(stderr, fmt.Sprintf("argument --json-only: ignored explicit argument '%s'", val))
				}
				a.JSONOnly = true
			case "--self-test":
				if hasEq {
					return a, wrArgErr(stderr, fmt.Sprintf("argument --self-test: ignored explicit argument '%s'", val))
				}
				a.SelfTest = true
			default:
				unknown = append(unknown, tok)
			}
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			unknown = append(unknown, tok)
			continue
		}
		positional = append(positional, tok)
	}
	if len(unknown) > 0 {
		return a, wrArgErr(stderr, "unrecognized arguments: "+strings.Join(unknown, " "))
	}
	if len(positional) > 1 {
		return a, wrArgErr(stderr, "unrecognized arguments: "+strings.Join(positional[1:], " "))
	}
	if len(positional) == 1 {
		p := positional[0]
		a.Path = &p
	}
	return a, -1
}

// wrArgErr —— argparse 错误档（usage + `prog: error: msg`）⇒ rc=2。
func wrArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, wrUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", wrProg, msg)
	return 2
}

// ── 根 / 树定位 ─────────────────────────────────────────────────────
// wrDefaultRoot —— py 用 `__file__`；本件无此语境 ⇒ 从 cwd 上溯找含 `scripts/gates` 的目录（声明 (a)）。
func wrDefaultRoot() string {
	start, err := os.Getwd()
	if err != nil {
		return "."
	}
	d, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	cur := d
	for {
		if wrIsDir(filepath.Join(cur, "scripts", "gates")) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return d
}

// wrResolveTrees —— py `resolve_trees(root)` 逐字同构 + 显式旋钮覆盖。
func wrResolveTrees(root, docsFlag, archFlag string) (string, string) {
	parent := filepath.Dir(root)
	docs := docsFlag
	if docs == "" {
		docs = os.Getenv("ZERG_DOCS_ROOT")
	}
	if docs == "" {
		docs = filepath.Join(parent, "Zerg-内部文档")
	}
	arch := archFlag
	if arch == "" {
		arch = os.Getenv("ZERG_ARCHIVE_ROOT")
	}
	if arch == "" {
		arch = filepath.Join(parent, "Zerg-归档")
	}
	return docs, arch
}

// ── 扫描 ────────────────────────────────────────────────────────────
// wrScanFor —— py `scan_for`。ok=false ⇔ py 返回 None；hits 空 ⇔ py 空列表。
func wrScanFor(stderr io.Writer, root, query, kind, docs, arch string) ([]wrHit, bool) {
	files, ok := wrGitFiles(root, stderr)
	if !ok {
		return nil, false
	}
	return wrScanHits(root, files, query, kind, docs, arch), true
}

// wrGitFiles —— py `git_files(root)`。ok=false ⇔ py 返回 None。
func wrGitFiles(root string, stderr io.Writer) ([]string, bool) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if ee, isEx := err.(*exec.ExitError); isEx {
			fmt.Fprintf(stderr, "✗ git ls-files rc=%d（不是 git 仓？）⇒ 不给结论\n", ee.ExitCode())
			return nil, false
		}
		fmt.Fprintf(stderr, "✗ 跑不了 git ls-files：%s ⇒ 不给结论\n", err)
		return nil, false
	}
	var files []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			files = append(files, p)
		}
	}
	if len(files) == 0 {
		return nil, false
	}
	return files, true
}

// wrScanHits —— py `scan_for` 的主体（files 已取）。
func wrScanHits(root string, files []string, query, kind, docs, arch string) []wrHit {
	hits := []wrHit{}
	seen := map[string]bool{}
	bareRE := regexp.MustCompile("(?:^|\\s)(" + regexp.QuoteMeta(query) + ")[\\p{L}\\p{N}_./–—-]*")
	for _, rel := range files {
		parts := strings.Split(rel, "/")
		skip := false
		for _, p := range parts {
			if wrContains(wrSkipDirs, p) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		pref := false
		for _, sp := range wrSkipPrefix {
			if strings.HasPrefix(rel, sp) {
				pref = true
				break
			}
		}
		if pref {
			continue
		}
		full := filepath.Join(root, rel)
		if !wrIsFile(full) {
			continue
		}
		text, ok := wrReadText(full)
		if !ok {
			continue
		}
		for idx, line := range strings.Split(text, "\n") {
			ln := idx + 1
			// 引文形态（反引号/引号包裹）。
			for _, m := range wrStrLitRE.FindAllStringSubmatch(line, -1) {
				cand := m[1]
				if !wrMatches(cand, query, kind) {
					continue
				}
				target := wrAsTargetRel(cand)
				if !wrIsWithinRepo(target, query) {
					continue
				}
				key := rel + "\x00" + strconv.Itoa(ln) + "\x00" + cand
				if seen[key] {
					continue
				}
				seen[key] = true
				inRepo, found, label := wrExistsAnywhere(target, root, docs, arch)
				hits = append(hits, wrHit{File: rel, Line: ln, Quote: cand, Target: target,
					FoundInRepo: inRepo, FoundInTree: found, TreeLabel: label})
			}
			// 裸路径形态（注释 / 散文里的引用）。
			for _, m := range bareRE.FindAllStringSubmatch(line, -1) {
				cand := m[1]
				key := rel + "\x00" + strconv.Itoa(ln) + "\x00" + cand
				if seen[key] {
					continue
				}
				seen[key] = true
				target := wrAsTargetRel(cand)
				inRepo, found, label := wrExistsAnywhere(target, root, docs, arch)
				hits = append(hits, wrHit{File: rel, Line: ln, Quote: cand, Target: target,
					FoundInRepo: inRepo, FoundInTree: found, TreeLabel: label})
			}
		}
	}
	return hits
}

// wrReadText —— py `_read_text`：含 NUL ⇒ None；非 UTF-8 ⇒ None。
func wrReadText(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	if strings.IndexByte(string(raw), 0) >= 0 {
		return "", false
	}
	if !utf8.Valid(raw) {
		return "", false
	}
	return string(raw), true
}

// wrMatches —— py `_matches`。
func wrMatches(cand, query, kind string) bool {
	c := strings.TrimSpace(cand)
	switch kind {
	case "file":
		return c == query || strings.HasPrefix(c, strings.TrimRight(query, "/")+"/")
	case "dir":
		base := strings.TrimRight(query, "/")
		return c == base || strings.HasPrefix(c, base+"/")
	default: // prefix
		base := strings.TrimRight(query, "/")
		return c == query || strings.HasPrefix(c, base+"/") || strings.Contains(c, base)
	}
}

// wrAsTargetRel —— py `_as_target_rel`。
func wrAsTargetRel(cand string) string {
	c := strings.TrimSpace(cand)
	if strings.HasPrefix(c, "./") {
		c = c[2:]
	}
	return c
}

// wrIsWithinRepo —— py `_is_within_repo`（保守：仅前缀）。
func wrIsWithinRepo(target, query string) bool {
	q := strings.TrimRight(query, "/")
	return target == q || strings.HasPrefix(target, q+"/")
}

// wrExistsAnywhere —— py `exists_anywhere`。
func wrExistsAnywhere(target, root, docs, arch string) (bool, bool, *string) {
	if strings.HasPrefix(target, "/") {
		inRepo := wrExists(filepath.Join(root, strings.TrimLeft(target, "/")))
		if inRepo {
			return true, true, nil
		}
		return false, false, nil
	}
	if wrExists(filepath.Join(root, target)) {
		return true, true, nil
	}
	rel2 := target
	if strings.HasPrefix(target, "docs/") {
		rel2 = target[len("docs/"):]
	}
	for _, tl := range []struct{ label, tree string }{{"Zerg-内部文档", docs}, {"Zerg-归档", arch}} {
		if tl.tree != "" && wrIsDir(tl.tree) && wrExists(filepath.Join(tl.tree, target)) {
			l := tl.label
			return false, true, &l
		}
		if rel2 != target && tl.tree != "" && wrIsDir(tl.tree) && wrExists(filepath.Join(tl.tree, rel2)) {
			l := tl.label
			return false, true, &l
		}
	}
	return false, false, nil
}

// ── 工具 ────────────────────────────────────────────────────────────
func wrExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func wrIsDir(p string) bool {
	if p == "" {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func wrIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func wrAbs(p string) (string, error) {
	if a, err := filepath.Abs(p); err == nil {
		return a, nil
	}
	return p, nil
}

func wrNormpath(p string) string {
	if p == "" {
		return "."
	}
	return filepath.Clean(p)
}

// wrLstripDotSlash —— py `s.lstrip("./")`：剥掉前导的 `.` 与 `/` 字符集合。
func wrLstripDotSlash(s string) string {
	i := 0
	for i < len(s) && (s[i] == '.' || s[i] == '/') {
		i++
	}
	return s[i:]
}

func wrContains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func wrCountRefFiles(hits []wrHit) int {
	seen := map[string]bool{}
	for _, h := range hits {
		seen[h.File] = true
	}
	return len(seen)
}

// ── 自检（成对负控 · 合成夹具）──────────────────────────────────────
func wrSelfTest(stdout io.Writer) int {
	tmp, err := os.MkdirTemp("", "wr-selftest-")
	if err != nil {
		tmp = os.TempDir()
	}
	defer os.RemoveAll(tmp)

	repo := filepath.Join(tmp, "Zerg")
	docs := filepath.Join(tmp, "Zerg-内部文档")
	_ = os.MkdirAll(filepath.Join(docs, "调研"), 0o755)
	_ = os.MkdirAll(filepath.Join(repo, "docs"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, "docs", "真件.md"), []byte("真件\n"), 0o644)
	_ = os.WriteFile(filepath.Join(repo, "readme.md"), []byte("见 `docs/真件.md`\n"), 0o644)
	_ = os.WriteFile(filepath.Join(docs, "调研", "旧稿.md"), []byte("旧稿\n"), 0o644)
	_ = os.WriteFile(filepath.Join(repo, "notes.md"), []byte("旧参考 `docs/调研/旧稿.md`\n"), 0o644)

	gitRun := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = repo
		_ = c.Run()
	}
	gitRun("init", "-q")
	gitRun("config", "user.email", "t@t")
	gitRun("add", "-A")
	gitRun("commit", "-q", "-m", "seed")

	arch := filepath.Join(filepath.Dir(repo), "Zerg-归档")
	run := func(query string) int {
		hits, ok := wrScanFor(os.Stderr, repo, query, "auto", docs, arch)
		if !ok {
			return 2
		}
		if len(hits) == 0 {
			return 2
		}
		for _, h := range hits {
			if !h.FoundInRepo {
				return 1
			}
		}
		return 0
	}

	bad := 0
	chk := func(label string, want, r int) {
		mark := "✓"
		if r != want {
			mark = "✗"
			bad++
		}
		fmt.Fprintf(stdout, "  %s [%s] rc=%d（期望 %d）\n", mark, label, r, want)
	}
	chk("真目标", 0, run("docs/真件.md"))
	chk("归档目标", 1, run("docs/调研/旧稿.md"))
	chk("不存在路径", 2, run("docs/不存在.md"))

	if bad == 0 {
		fmt.Fprint(stdout, "自检：通过 ✓（3 格：真目标 rc=0 · 归档目标 rc=1 · 不存在路径 rc=2）\n")
		return 0
	}
	fmt.Fprint(stdout, "自检：**失败** ✗（3 格：真目标 rc=0 · 归档目标 rc=1 · 不存在路径 rc=2）\n")
	return 1
}

// ── JSON（逐字节复刻 Python json.dumps/dump · ensure_ascii=False）────
func wrJStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func wrJBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func wrHitCompact(h wrHit) string {
	tl := "null"
	if h.TreeLabel != nil {
		tl = wrJStr(*h.TreeLabel)
	}
	return "{\"file\": " + wrJStr(h.File) +
		", \"line\": " + strconv.Itoa(h.Line) +
		", \"quote\": " + wrJStr(h.Quote) +
		", \"target\": " + wrJStr(h.Target) +
		", \"found_in_repo\": " + wrJBool(h.FoundInRepo) +
		", \"found_in_tree\": " + wrJBool(h.FoundInTree) +
		", \"tree_label\": " + tl + "}"
}

func wrJSONCompact(s wrSummary) string {
	var b strings.Builder
	b.WriteString("{\"schema\": " + wrJStr("who-references/1"))
	b.WriteString(", \"target\": " + wrJStr(s.Target))
	b.WriteString(", \"repo\": " + wrJStr(s.Repo))
	b.WriteString(", \"total_references\": " + strconv.Itoa(s.Total))
	b.WriteString(", \"referencing_files\": " + strconv.Itoa(s.RefFiles))
	b.WriteString(", \"pointing_to_archived_or_broken\": " + strconv.Itoa(s.Pending))
	if len(s.ByTree) == 0 {
		b.WriteString(", \"by_tree\": {}")
	} else {
		b.WriteString(", \"by_tree\": {")
		for i, kv := range s.ByTree {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(wrJStr(kv.Key) + ": " + strconv.Itoa(kv.Val))
		}
		b.WriteString("}")
	}
	if len(s.Hits) == 0 {
		b.WriteString(", \"hits\": []}")
		return b.String()
	}
	b.WriteString(", \"hits\": [")
	for i, h := range s.Hits {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(wrHitCompact(h))
	}
	b.WriteString("]}")
	return b.String()
}

func wrJSONIndent(s wrSummary) string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString("  \"schema\": " + wrJStr("who-references/1") + ",\n")
	b.WriteString("  \"target\": " + wrJStr(s.Target) + ",\n")
	b.WriteString("  \"repo\": " + wrJStr(s.Repo) + ",\n")
	b.WriteString("  \"total_references\": " + strconv.Itoa(s.Total) + ",\n")
	b.WriteString("  \"referencing_files\": " + strconv.Itoa(s.RefFiles) + ",\n")
	b.WriteString("  \"pointing_to_archived_or_broken\": " + strconv.Itoa(s.Pending) + ",\n")
	if len(s.ByTree) == 0 {
		b.WriteString("  \"by_tree\": {},\n")
	} else {
		b.WriteString("  \"by_tree\": {\n")
		for i, kv := range s.ByTree {
			b.WriteString("    " + wrJStr(kv.Key) + ": " + strconv.Itoa(kv.Val))
			if i < len(s.ByTree)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("  },\n")
	}
	if len(s.Hits) == 0 {
		b.WriteString("  \"hits\": []\n}")
		return b.String()
	}
	b.WriteString("  \"hits\": [\n")
	for i, h := range s.Hits {
		tl := "null"
		if h.TreeLabel != nil {
			tl = wrJStr(*h.TreeLabel)
		}
		b.WriteString("    {\n")
		b.WriteString("      \"file\": " + wrJStr(h.File) + ",\n")
		b.WriteString("      \"line\": " + strconv.Itoa(h.Line) + ",\n")
		b.WriteString("      \"quote\": " + wrJStr(h.Quote) + ",\n")
		b.WriteString("      \"target\": " + wrJStr(h.Target) + ",\n")
		b.WriteString("      \"found_in_repo\": " + wrJBool(h.FoundInRepo) + ",\n")
		b.WriteString("      \"found_in_tree\": " + wrJBool(h.FoundInTree) + ",\n")
		b.WriteString("      \"tree_label\": " + tl + "\n")
		if i < len(s.Hits)-1 {
			b.WriteString("    },\n")
		} else {
			b.WriteString("    }\n")
		}
	}
	b.WriteString("  ]\n}")
	return b.String()
}
