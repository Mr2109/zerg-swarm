// check_mkdir_temp_cleanup.go —— 判据件 `scripts/gates/check-mkdir-temp-cleanup.py`（210 行 · M2 搬运粒 21）
// 的 **Go 真搬**（落点 `core/cmd/zerg/`）。判「凡 `os.MkdirTemp`（建临时目录）调用点：同函数内是否有
// `os.RemoveAll`（回收）」，
// 分三档：T1 同函数内干净 / T2 同件有 RemoveAll（helper/调用方清 · 只报告）/ T3 同件内一个 RemoveAll 都没有（判漏）。
// 白名单（逐行带理由）压 T3。退码：0 零 T3 / 1 有 T3 / 2 不给结论（0 命中 = 空转 = 假覆盖）。
//
// 为什么有这件（不是「顺手 Go 化」）：v2.5.14 定案把判据实现面分两段 —— M1 先把 `scripts/` 下的独立件收成
// CLI 打得着的统一面；M2 按价值**真搬 Go**。本件 = M2 搬运面在**候选区间**内选中并真搬的一枚：验收判据 =
// 对同一件跑两态（py 原件 ↔ 新入口），stdout / stderr / 退码三面逐字相同。
//
// 选它 / 不选它的理由（纯静态只读 · 退码仅 0/1/2 · 无子进程/无计时/无随机/无联网）：
//
//	✓ 选 `check-mkdir-temp-cleanup.py`（210 行）：**纯静态只读**
//	  —— `main` 路径零 `subprocess`/`socket`/`urllib`/`time`/`datetime`/`random`；
//	  只 `os.walk` 读 `.go` 件、正则计数、打印；**默认档零写面**（只有自检才在 `tempfile.mkdtemp` 造夹具）。
//	✗ 不选同批其余未搬件（现读 import / 危险形态面后的排除理由）：`check-bypass-idioms.py` /
//	  `check-exec-two-condition.py` / `check-delegation-selfreport.py` / `check-gap-state-vs-artifact.py` /
//	  `check-model-size-anchor.py` 均带 `subprocess`（真起子进程取读数）⇒ 出局。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改任何 `add_step`）；
//	· **不**删 `scripts/gates/check-mkdir-temp-cleanup.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `checkMkdirTempCleanupGateMain` 目前**无调用点** —— 这是 M2 的刻意状态。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkMkdirTempCleanupGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（**不含程序名**）
//	  · 退码   —— 0 零 T3 / 1 有 T3 / 2 不给结论（0 命中 · 用法错 · 自检不过）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见件尾「已对拍面 / 未覆盖面」）
//
// ── 两行标准声明（本单必写）──────────────────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py 的 `--root` 默认 `"."`、`main` 里 `root = os.path.abspath(a.root)`
//	    （相对 → 拼 **cwd** 再 `normpath`）；Go 入口**无 `__file__` 语境** ⇒ 同构：相对 root 拼 cwd 再词法
//	    `normpath`（自写 `cmkPyAbspath`，✗ 不用 `filepath.Abs`：后者会 `Clean` 且不保留 `//` 前导语义）。
//	    本件**不像**别的件那样「从 cwd 上溯找仓根」—— 它照 py 语义**直接吃 `--root`**（默认 cwd）。
//	    落在**同一仓、同 cwd** 时两者路径推导**逐字同**（对拍 C03/C06 已证）；cwd 不同则读数随 cwd 变（与 py 同）。
//	(b) **自指检查 —— 判定为「真」**：本件落 `core/cmd/zerg/`，而本门默认扫描面 = 仓根下 `core,agent,shared` 的
//	    `*.go` ⇒ **本件在被扫面内**。处置（消解 + 自证）：
//	      ① 本件**全部** `os.MkdirTemp`（真实调用）站点（本体只有自检那几处）都落在**同一个函数**
//	         `cmkRunSelfTest` 内，且该函数内有 `defer os.RemoveAll` ⇒ 一律计入 **T1**、**不产 T3**（判档不变）。
//	      ② 自检夹具的字面串用**拆串拼装**（`"os.Mkdir"+"Temp("` / `"os.Remove"+"All("`）落地，源码正文
//	         **不含**连续可被本门正则命中的原串 ⇒ 夹具串不额外造站点（✗ 直接写连续原串会凭空多出站点）。
//	      ③ 净效果 = 本件落件后真仓读数 `sites`/`T1` 各 +K（K = 本件命中行数）、T2/T3 不变、判档不变；
//	         落件前后 py 侧同命令「T3 明细」逐字相同（见回执）。
//
// ⑤ `--evidence GAP-20261001-13`（在册 M2 承接 · 先回读，语义对口）。
//
// 实现要点（与 py 逐处对齐、✗ 不靠「看起来等价」）：
//
//	· 正则 `MKDIR = os\.MkdirTemp\(` / `RMALL = os\.RemoveAll\(` / `FUNCRE = ^func\s` 逐字直译（全 ASCII
//	  字符类 ⇒ 无 py `\s`/`\d` 的 Unicode 语义差；`^func` 在**单行**上与 py `re.match` 同义）。
//	· `text.splitlines()`、`open(encoding=).read()` 的 universal newlines、非法 UTF-8 ⇒ 整件跳过（py 侧
//	  strict decode 抛 `UnicodeDecodeError` 被 `except Exception: continue` 吞掉）—— 全部自实现。
//	· `func_bounds` 的**花括号朴素计数**（含字符串字面量里的 `{`/`}`）逐字复刻（✗ 不换成 AST：那会与 py 分叉）。
//	· 目录遍历复刻 `os.walk`：`File.ReadDir(-1)` 保 **readdir 原序**（✗ 不用会排序的 `os.ReadDir`）；
//	  **先出当前层全部文件、再按子目录 readdir 原序下钻**；`entry.is_dir()` 跟随符号链接 ⇒ 用 `os.Stat` 补判
//	  （✗ 不用 `entry.IsDir()` 的 lstat 语义）；而 `os.walk(followlinks=False)`（py 默认）**不下钻指向目录的
//	  符号链接**（它仍归 dirnames、不当文件扫）⇒ Go 侧递归前 `cmkEntryIsSymlink` 跳过（血证 D02 夹具）。
//	· 路径拼接走自写 `cmkPyJoin`（✗ 不用 `filepath.Join`：`..`/绝对后段/空段三态不同）；`relpath`/`abspath`/
//	  `normpath` 亦自写（✗ 不用 `filepath.Rel`/`Abs`/`Clean`）。
//	· argparse 面**含前缀缩写**（`--ro`⇒`--root` 等唯一前缀收；本例无歧义档），逐格与 py 对齐。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const cmkProg = "check-mkdir-temp-cleanup.py"

// cmkUsage —— py argparse 的用法串（80 列内换行成两行，错误档与帮助档共用同一串）。
const cmkUsage = "usage: check-mkdir-temp-cleanup.py [-h] [--root ROOT] [--dirs DIRS]\n" +
	"                                   [--whitelist WHITELIST] [--self-test]\n"

// cmkHelpText —— py argparse（本件无 `description=`）的**逐字**帮助正文（`-h`/`--help` · 对拍 C01 已逐字节证）。
const cmkHelpText = cmkUsage +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --dirs DIRS\n" +
	"  --whitelist WHITELIST\n" +
	"  --self-test\n"

// cmkLongOpts —— argparse 已注册的**长选项串**（按注册序；用于前缀缩写匹配）。
var cmkLongOpts = []string{"--help", "--root", "--dirs", "--whitelist", "--self-test"}

// cmkMkdirRE / cmkRmallRE / cmkFuncRE —— py 三个正则逐字直译。
var (
	cmkMkdirRE = regexp.MustCompile(`os\.MkdirTemp\(`)
	cmkRmallRE = regexp.MustCompile(`os\.RemoveAll\(`)
	cmkFuncRE  = regexp.MustCompile(`^func\s`)
)

// ── py 基础件（join / normpath / abspath / relpath / strip / splitlines / 读件）───────────────

// cmkPyJoin —— CPython `posixpath.join` 逐段规则直译（✗ 不用 filepath.Join）。
func cmkPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// cmkPyNormpath —— CPython `posixpath.normpath`（保留**恰**两个前导斜杠；✗ 不用 filepath.Clean）。
func cmkPyNormpath(p string) string {
	if p == "" {
		return "."
	}
	initSlashes := 0
	for initSlashes < len(p) && p[initSlashes] == '/' {
		initSlashes++
	}
	prefix := ""
	switch {
	case initSlashes == 2:
		prefix = "//"
	case initSlashes >= 1:
		prefix = "/"
	}
	var comps []string
	for _, c := range strings.Split(p, "/") {
		if c == "" || c == "." {
			continue
		}
		if c == ".." {
			if len(comps) > 0 && comps[len(comps)-1] != ".." {
				comps = comps[:len(comps)-1]
			} else if prefix == "" {
				comps = append(comps, "..")
			}
			continue
		}
		comps = append(comps, c)
	}
	res := prefix + strings.Join(comps, "/")
	if res == "" {
		return "."
	}
	return res
}

// cmkPyAbspath —— CPython `os.path.abspath`（纯词法：相对则拼 cwd 再 normpath；✗ 不用 filepath.Abs）。
func cmkPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		cwd, _ := os.Getwd()
		p = cmkPyJoin(cwd, p)
	}
	return cmkPyNormpath(p)
}

// cmkPathComponents —— 拆非空路径段（用于 relpath）。
func cmkPathComponents(p string) []string {
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

// cmkPyRelpath —— CPython `posixpath.relpath`（纯词法；✗ 不用 filepath.Rel）。
func cmkPyRelpath(path, start string) string {
	if path == "" {
		return ""
	}
	sp := cmkPyAbspath(path)
	st := cmkPyAbspath(start)
	if sp == st {
		return "."
	}
	sc := cmkPathComponents(sp)
	tc := cmkPathComponents(st)
	i := 0
	for i < len(sc) && i < len(tc) && sc[i] == tc[i] {
		i++
	}
	var parts []string
	for k := i; k < len(tc); k++ {
		parts = append(parts, "..")
	}
	parts = append(parts, sc[i:]...)
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, "/")
}

// cmkPyStrip —— py `str.strip()` 的 Unicode 空白面（含 `\x1c-\x1f`；✗ 不用 strings.TrimSpace）。
func cmkPyStrip(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
	})
}

// cmkPySplitLines —— CPython `str.splitlines` 的行界（`\n\r\v\f\x1c\x1d\x1e\x85  `）。
func cmkPySplitLines(s string) []string {
	var out []string
	var cur []rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '\n' || r == '\r' || r == '\v' || r == '\f' ||
			r == 0x1c || r == 0x1d || r == 0x1e || r == 0x85 || r == 0x2028 || r == 0x2029 {
			out = append(out, string(cur))
			cur = cur[:0]
			if r == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// cmkReadLines —— 复刻 `open(p, encoding="utf-8").read().splitlines()`：
// 非法 UTF-8 ⇒ 抛异常（py strict decode）⇒ 返回 ok=false（调用方 `continue` 跳过整件）；
// 合法 ⇒ universal newlines 归一（`\r\n`/裸 `\r` → `\n`）再 splitlines。
func cmkReadLines(p string) ([]string, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	if !utf8.Valid(b) {
		return nil, false
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return cmkPySplitLines(s), true
}

// cmkReadDirRaw —— `os.scandir` 原序（✗ 不用会排序的 os.ReadDir）。
func cmkReadDirRaw(dir string) ([]os.DirEntry, bool) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	ents, err := f.ReadDir(-1)
	if err != nil {
		return nil, false
	}
	return ents, true
}

// cmkEntryIsDir —— py `entry.is_dir()` 跟随符号链接（✗ 不用 lstat 语义的 entry.IsDir()）。
func cmkEntryIsDir(dp string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&os.ModeSymlink != 0 {
		if fi, err := os.Stat(cmkPyJoin(dp, e.Name())); err == nil && fi.IsDir() {
			return true
		}
	}
	return false
}

// cmkEntryIsSymlink —— py `os.path.islink`（✗ 别只看 `e.Type()`：部分文件系统 Type 位为 0，须回落 `os.Lstat`）。
func cmkEntryIsSymlink(dp string, e os.DirEntry) bool {
	if e.Type()&os.ModeSymlink != 0 {
		return true
	}
	if e.Type() == 0 {
		if fi, err := os.Lstat(cmkPyJoin(dp, e.Name())); err == nil {
			return fi.Mode()&os.ModeSymlink != 0
		}
	}
	return false
}

// ── 判据体（与 py 逐处对齐）─────────────────────────────────────────────────────────

// cmkFuncBounds —— py `func_bounds` 逐字直译（花括号朴素计数，含串内括号）。
func cmkFuncBounds(lines []string, idx int) (int, int) {
	s := 0
	for j := idx; j >= 0; j-- {
		if cmkFuncRE.MatchString(lines[j]) {
			s = j
			break
		}
	}
	depth, seen := 0, false
	for j := s; j < len(lines); j++ {
		for _, ch := range lines[j] {
			if ch == '{' {
				depth++
				seen = true
			} else if ch == '}' {
				depth--
				if seen && depth == 0 {
					return s, j
				}
			}
		}
	}
	return s, len(lines) - 1
}

// cmkWalkOne —— 处理一个 dp：先出本层全部文件（readdir 原序），再按子目录原序下钻（复刻 `os.walk`）。
func cmkWalkOne(dp, root string, wlset map[string]bool, sites, t1, t2 *int, t3 *[]string) {
	skipped := strings.Contains(dp, "/vendor") || strings.Contains(dp, "/testdata")
	ents, ok := cmkReadDirRaw(dp)
	if !ok {
		return // os.walk onerror=None ⇒ 读不了的目录静默跳过（B 类：不产结论、也不报错）
	}
	var dirs, files []os.DirEntry
	for _, e := range ents {
		if cmkEntryIsDir(dp, e) {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	if !skipped {
		for _, f := range files {
			name := f.Name()
			if !strings.HasSuffix(name, ".go") {
				continue
			}
			p := cmkPyJoin(dp, name)
			rel := cmkPyRelpath(p, root)
			lines, rok := cmkReadLines(p)
			if !rok {
				continue
			}
			fileHasRall := false
			for _, l := range lines {
				if cmkRmallRE.MatchString(l) {
					fileHasRall = true
					break
				}
			}
			for i, l := range lines {
				if !cmkMkdirRE.MatchString(l) {
					continue
				}
				*sites++
				s, e := cmkFuncBounds(lines, i)
				body := strings.Join(lines[s:e+1], "\n")
				if cmkRmallRE.MatchString(body) {
					*t1++
				} else if fileHasRall {
					*t2++
				} else {
					*t3 = append(*t3, fmt.Sprintf("%s:%d", rel, i+1))
				}
			}
		}
	}
	for _, d := range dirs {
		// ★ os.walk(followlinks=False)：**指向目录的符号链接不进 dirnames 的下钻**（血证：C03/D02 夹具
		// 造 linkdir→real 时，py 不下钻、Go 若下钻会多报一条 T3 ⇒ 假 DIFF）。它仍被 `is_dir()` 归入 dirnames
		// （所以不会被当文件扫），只是不下钻。
		if cmkEntryIsSymlink(dp, d) {
			continue
		}
		cmkWalkOne(cmkPyJoin(dp, d.Name()), root, wlset, sites, t1, t2, t3)
	}
}

// cmkScan —— py `scan` 逐字直译。
func cmkScan(root, dirs string, whitelist []string) (sites, t1, t2 int, t3, t3f []string) {
	wlset := map[string]bool{}
	for _, x := range whitelist {
		wlset[x] = true
	}
	for _, d := range strings.Split(dirs, ",") {
		cmkWalkOne(cmkPyJoin(root, d), root, wlset, &sites, &t1, &t2, &t3)
	}
	for _, x := range t3 {
		if !wlset[strings.SplitN(x, ":", 2)[0]] {
			t3f = append(t3f, x)
		}
	}
	return sites, t1, t2, t3, t3f
}

// cmkJudge —— py `judge`：0 零 T3 / 1 有 T3 / 2 空转。
func cmkJudge(root, dirs string, whitelist []string) (int, int, int, int, []string, []string) {
	sites, t1, t2, t3, t3f := cmkScan(root, dirs, whitelist)
	switch {
	case sites == 0:
		return 2, sites, t1, t2, t3, t3f
	case len(t3f) > 0:
		return 1, sites, t1, t2, t3, t3f
	default:
		return 0, sites, t1, t2, t3, t3f
	}
}

// ── argparse 面（长选项 / -h / 前缀缩写 / 未知旗标 / 缺值）────────────────────────────

// cmkResolveLong —— 把去 `--` 的名字解析成注册的长选项串（精确 > 唯一前缀）；返回 matches 供歧义报错。
func cmkResolveLong(name string) (string, []string) {
	full := "--" + name
	var matches []string
	for _, o := range cmkLongOpts {
		if o == full {
			return o, []string{o}
		}
		if strings.HasPrefix(o, full) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 1 {
		return matches[0], matches
	}
	return "", matches
}

// cmkArgErr —— argparse 的错误面（usage + "prog: error: …"）到 stderr、退 2。
func cmkArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, cmkUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", cmkProg, msg)
	return 2
}

// cmkParseArgs —— 解析 argv（不含程序名）。done=true ⇒ 已由帮助/错误档收场，直接返回 rc。
func cmkParseArgs(argv []string, stdout, stderr io.Writer) (root, dirs, whitelist string, selfTest bool, rc int, done bool) {
	root, dirs, whitelist = ".", "core,agent,shared", ""
	var unrec []string
	for i := 0; i < len(argv); {
		tok := argv[i]
		if tok == "--" {
			unrec = append(unrec, argv[i+1:]...)
			break
		}
		if tok == "-h" || tok == "--help" {
			fmt.Fprint(stdout, cmkHelpText)
			return root, dirs, whitelist, selfTest, 0, true
		}
		if strings.HasPrefix(tok, "--") {
			name := tok[2:]
			val, hasVal := "", false
			if eq := strings.Index(name, "="); eq >= 0 {
				val, name, hasVal = name[eq+1:], name[:eq], true
			}
			opt, matches := cmkResolveLong(name)
			if opt == "" {
				if len(matches) > 1 {
					return root, dirs, whitelist, selfTest, cmkArgErr(stderr, fmt.Sprintf("ambiguous option: --%s could match %s", name, strings.Join(matches, ", "))), true
				}
				unrec = append(unrec, tok)
				i++
				continue
			}
			switch opt {
			case "--help":
				if hasVal {
					return root, dirs, whitelist, selfTest, cmkArgErr(stderr, fmt.Sprintf("argument -h/--help: ignored explicit argument '%s'", val)), true
				}
				fmt.Fprint(stdout, cmkHelpText)
				return root, dirs, whitelist, selfTest, 0, true
			case "--self-test":
				if hasVal {
					return root, dirs, whitelist, selfTest, cmkArgErr(stderr, fmt.Sprintf("argument --self-test: ignored explicit argument '%s'", val)), true
				}
				selfTest = true
				i++
			case "--root", "--dirs", "--whitelist":
				var v string
				if hasVal {
					v = val
					i++
				} else {
					if i+1 >= len(argv) || (strings.HasPrefix(argv[i+1], "-") && argv[i+1] != "-") {
						return root, dirs, whitelist, selfTest, cmkArgErr(stderr, fmt.Sprintf("argument %s: expected one argument", opt)), true
					}
					v = argv[i+1]
					i += 2
				}
				switch opt {
				case "--root":
					root = v
				case "--dirs":
					dirs = v
				case "--whitelist":
					whitelist = v
				}
			}
			continue
		}
		unrec = append(unrec, tok)
		i++
	}
	if len(unrec) > 0 {
		return root, dirs, whitelist, selfTest, cmkArgErr(stderr, "unrecognized arguments: "+strings.Join(unrec, " ")), true
	}
	return root, dirs, whitelist, selfTest, 0, false
}

// ── 入口 ───────────────────────────────────────────────────────────────────────────

func checkMkdirTempCleanupGateMain(stdout, stderr io.Writer, argv []string) int {
	root, dirs, whitelist, selfTest, rc, done := cmkParseArgs(argv, stdout, stderr)
	if done {
		return rc
	}
	if selfTest {
		return cmkRunSelfTest(stdout)
	}
	absRoot := cmkPyAbspath(root)
	var wl []string
	if whitelist != "" {
		if st, err := os.Stat(whitelist); err == nil && !st.IsDir() {
			b, err := os.ReadFile(whitelist)
			if err == nil {
				s := strings.ReplaceAll(string(b), "\r\n", "\n")
				s = strings.ReplaceAll(s, "\r", "\n")
				for _, l := range cmkPySplitLines(s) {
					t := cmkPyStrip(l)
					// py: `if l.strip() and not l.startswith("#")` —— startswith 判**原行**、值取 strip 后。
					if t != "" && !strings.HasPrefix(l, "#") {
						wl = append(wl, t)
					}
				}
			}
		}
	}
	sites, t1, t2, t3, t3f := cmkScan(absRoot, dirs, wl)
	if sites == 0 {
		fmt.Fprint(stderr, "⛔ 不给结论（rc=2）：扫面内 0 个 `"+cmkKTok+"`（空转 = 假覆盖）\n")
		return 2
	}
	fmt.Fprintf(stdout, "站点 %d = T1 同函数干净 %d + T2 同件有 RemoveAll（helper/调用方清 · 只报告）%d + T3 同件无 RemoveAll %d\n",
		sites, t1, t2, len(t3))
	if len(t3f) > 0 {
		fmt.Fprintf(stderr, "✗ T3 %d 处**同件内无任何 RemoveAll**（逐条点名）：\n", len(t3f))
		for _, x := range t3f {
			fmt.Fprintf(stderr, "  %s\n", x)
		}
		return 1
	}
	fmt.Fprint(stdout, "✓ T3 为 0（或已在白名单）\n")
	return 0
}

// ── 自检（合成 Go 夹具 · 一律落仓外临时树 · 成对正负控）────────────────────────────────
//
// ★ 自指消解：夹具里的「站点/回收」原串拆开写（`"os.Mkdir"+"Temp("` / `"os.Remove"+"All("`），
//   使源码正文不含连续可被本门正则命中的原串；运行时拼出的字节与 py 夹具逐字相同。

const cmkKTok = "os.Mkdir" + "Temp("
const cmkRTok = "os.Remove" + "All("

var cmkT1Go = "package x\n\nfunc f() {\n    d, _ := " + cmkKTok + "\"\", \"z\")\n    defer " + cmkRTok + "d)\n}\n"
var cmkT3Go = "package x\n\nfunc g() {\n    d, _ := " + cmkKTok + "\"\", \"z\")\n    _ = d\n}\n"
var cmkT2Go = "package x\n\nfunc h() {\n    d, _ := " + cmkKTok + "\"\", \"z\")\n    _ = d\n}\n\nfunc cleanup(p string) {\n    _ = " + cmkRTok + "p)\n}\n"
var cmkNoopGo = "package x\n\nfunc n() int {\n    return 1\n}\n"

// cmkWriteFixture —— py `_write`：建父目录 + 写件（仅自检用）。
func cmkWriteFixture(tmp, rel, body string) {
	p := cmkPyJoin(tmp, rel)
	if dir := cmkPyDirname(p); dir != "" {
		os.MkdirAll(dir, 0o755)
	}
	os.WriteFile(p, []byte(body), 0o644)
}

// cmkPyDirname —— py `posixpath.dirname`（剥尾斜杠后再取前段；✗ 不用 filepath.Dir）。
func cmkPyDirname(p string) string {
	if p == "" {
		return ""
	}
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	return p[:i]
}

// cmkRunSelfTest —— py `run_self_test` 逐字对齐（5 组 · 9 条断言）。
// 本函数**同函数内**有 `os.MkdirTemp` 与 `defer os.RemoveAll` ⇒ 本件的站点一律 T1（判档不变）。
func cmkRunSelfTest(stdout io.Writer) int {
	fmt.Fprint(stdout, "MkdirTemp↔RemoveAll 机器闸 · 自检（合成 Go 夹具 · 临时树 · 成对正负控）\n")
	total, bad := 0, 0
	assert := func(name string, got, want interface{}) {
		total++
		if !cmkDeepEqual(got, want) {
			bad++
			fmt.Fprintf(stdout, "  ✗ 断言失败：%s —— 得到 %v，期望 %v\n", name, got, want)
		}
	}

	tmp, err := os.MkdirTemp("", "mkdt-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "── 自检结论：断言 %d 条 · 失败 %d 条 ──\n", total, bad+1)
		return 1
	}
	defer os.RemoveAll(tmp)

	// ① 正控 T1：同函数内有 RemoveAll ⇒ rc=0
	cmkWriteFixture(tmp, "core/a.go", cmkT1Go)
	rc, sites, t1, _, _, _ := cmkJudge(tmp, "core,agent,shared", nil)
	assert("① 正控 T1：同函数有 RemoveAll ⇒ rc=0", rc, 0)
	assert("① 正控：sites=1 且 T1=1", []int{sites, t1}, []int{1, 1})

	// ② 负控 T3：同件无 RemoveAll ⇒ rc=1 且点名 件:行
	tmp2, err2 := os.MkdirTemp("", "mkdt-t3-")
	if err2 == nil {
		defer os.RemoveAll(tmp2)
		cmkWriteFixture(tmp2, "core/b.go", cmkT3Go)
		rc, _, _, _, _, t3f := cmkJudge(tmp2, "core,agent,shared", nil)
		assert("② 负控 T3：同件无 RemoveAll ⇒ rc=1", rc, 1)
		assert("② 负控：点名恰 core/b.go:4", t3f, []string{"core/b.go:4"})
	}

	// ③ T2 helper（同件有 RemoveAll 但在别的函数）⇒ 只报告 · rc=0
	tmp3, err3 := os.MkdirTemp("", "mkdt-t2-")
	if err3 == nil {
		defer os.RemoveAll(tmp3)
		cmkWriteFixture(tmp3, "core/c.go", cmkT2Go)
		rc, _, _, t2, t3, _ := cmkJudge(tmp3, "core,agent,shared", nil)
		assert("③ T2 helper：同件有 RemoveAll ⇒ rc=0（只报告）", rc, 0)
		assert("③ T2 计入 T2 档且 T3=0", []int{t2, len(t3)}, []int{1, 0})
	}

	// ④ 白名单负控：T3 件进白名单 ⇒ rc=0（出口有效）
	tmp4, err4 := os.MkdirTemp("", "mkdt-wl-")
	if err4 == nil {
		defer os.RemoveAll(tmp4)
		cmkWriteFixture(tmp4, "core/b.go", cmkT3Go)
		rc, _, _, _, t3, t3f := cmkJudge(tmp4, "core,agent,shared", []string{"core/b.go"})
		assert("④ 白名单负控：T3 件已白名单 ⇒ rc=0", rc, 0)
		assert("④ 白名单负控：t3f 为空而 t3 仍记 1", []int{len(t3f), len(t3)}, []int{0, 1})
	}

	// ⑤ 空转负控：扫面内 0 个 MkdirTemp ⇒ rc=2
	tmp5, err5 := os.MkdirTemp("", "mkdt-empty-")
	if err5 == nil {
		defer os.RemoveAll(tmp5)
		cmkWriteFixture(tmp5, "core/d.go", cmkNoopGo)
		rc, _, _, _, _, _ = cmkJudge(tmp5, "core,agent,shared", nil)
		assert("⑤ 空转负控：0 个 MkdirTemp ⇒ rc=2", rc, 2)
	}

	fmt.Fprintf(stdout, "── 自检结论：断言 %d 条 · 失败 %d 条 ──\n", total, bad)
	if bad == 0 {
		return 0
	}
	return 1
}

// cmkDeepEqual —— 极简深比较（自检断言用；只覆盖 int / string / []int / []string）。
func cmkDeepEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case int:
		bv, ok := b.(int)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case []int:
		bv, ok := b.([]int)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	case []string:
		bv, ok := b.([]string)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if av[i] != bv[i] {
				return false
			}
		}
		return true
	}
	return false
}
