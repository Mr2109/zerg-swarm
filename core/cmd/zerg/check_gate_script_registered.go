// check_gate_script_registered.go —— 判据件 `scripts/gates/check-gate-script-registered.py`（111 行）的
// **Go 真搬**（v2.5.14 · M2 · 搬运面第 3 粒 · 入口 `checkGateScriptRegisteredGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的 argparse
//	面差异与未覆盖面）。
//
// 选它的理由（本单口径：真未搬清单里**行数最小的一批** · 纯静态只读）：
//
//	✓ 选：判据只读**两处** —— `<root>/scripts/gates/` 直下的 `.py`/`.sh` 门件名单 ⟷
//	  `<root>/scripts/公开标记.tsv` 第一列（路径）；差集（未登记门件）逐条点名。**不真跑子进程、
//	  不碰网络、零计时、默认档不写盘**（`--self-test` 只在 `tempfile.TemporaryDirectory()` 里造件 ——
//	  `with` 形自带回收，**非** A 类泄漏形态）。
//	· 无外部树依赖：`--root`（env `ZERG_ROOT`）可显式指到任意合成假仓根（本批新口径：外部树走显式
//	  旋钮，旋钮名照 py 同名旗标 `--root` / env `ZERG_ROOT`）。
//	· 退码 0/1/2 三档齐 · 纯只读无写面 · 行数小（111 行）· 落件前 `git status` 现读不含它。
//	· 现读 import 面：argparse / json / os / sys / tempfile —— **零 `subprocess` / `time` /
//	  `datetime` / `random` / `socket` / `urllib`**。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `gate_inproc_table.go`）；
//	· **不**删 `scripts/gates/check-gate-script-registered.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `checkGateScriptRegisteredGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkGateScriptRegisteredGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 全登记 / 1 有未登记门件 / 2 不给结论（目录不在盘 · 空域 · 标记表读不到）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「未覆盖面」）
//
// ═══════════════════════ ★ 遍历序（本件有目录遍历 —— 逐点交代）═══════════════════════
//
//	py `os.listdir(<root>/scripts/gates)` = **readdir 原序**（CPython 不排序）。Go 侧用
//	`f.ReadDir(-1)`（**不排序**）保原序；✗ 别用 `os.ReadDir` / `filepath.Walk*`（会排序 ⇒ 假失配）。
//	★ 输出行序的决定者 = `missing = [g for g in gs if g not in reg]` —— 直接承运 `gs` 的顺序
//	  （即 readdir 原序）；随后逐条 `print("  ✗ 未登记：%s" % m)` ⇒ **明细行序 = readdir 原序**。
//	  py 里**零** `sorted(...)`。登记表用 **set**（只做 O(1) 成员判定与 `len()`，**不进输出序**）。
//	★ 无 Go map 迭代进输出：`reg` 为 `map[string]bool`，只做 O(1) 查值；输出序全由切片承运。
//
// ═══════════════════════ ★ py 字符串原语（逐处现读）═══════════════════════
//
//	① `ln.strip()`（`registered()` 两处）＝ py `str.strip()` 剥的 **Unicode 空白**（含
//	  `\x1c-\x1f` / `\x85` / `\xa0` …）⟷ Go `strings.TrimSpace` 不剥 `\x1c-\x1f` ⇒ 自写
//	  `gsrPyStrip`（按 py 认可集）。
//	② `open(..., encoding="utf-8")` 文本模式 = **universal newlines**（`\r\n` 与裸 `\r` 折 `\n`）⇒
//	  自写 `gsrTextLines` 显式折；✗ 不按 `\n` 直切。
//	③ `name.endswith((".py", ".sh"))` ＝ 后缀判定（两语言同形，逐字直译）。
//	④ `ln.split("\t", 1)[0]` ＝ 首个制表符前段（无制表符则为整行）。
//
// ═══════════════════════ ★ 声明 (a) 默认根定位差异（本件逐字同构）═══════════════════════
//
//	py `--root` 缺省 = `os.environ.get("ZERG_ROOT", os.getcwd())` ⇒ **起于 cwd**（不是 `__file__` 上溯）。
//	Go 无 `__file__` 语境，但本件 py 本就用 cwd ⇒ 走 `os.LookupEnv("ZERG_ROOT")` + `os.Getwd()`
//	逐字同构（`LookupEnv` 保真「键在但空串」档）。⚠ 未覆盖面：py `os.getcwd()` ⟷ Go `os.Getwd()`
//	（后者优先 `$PWD` 若其 `stat` 命中 `.`）。
//
// ═══════════════════════ ★ 声明 (b) 自指面（本件落在扫描面内 ⇒ 逐条消解）═══════════════════════
//
//	① 自扫自身：本判据扫的是 `scripts/gates/` 与 `scripts/公开标记.tsv`，**不扫 `core/`** ⇒ 非自扫。
//	② 件头注释无真引号路径原串（普通仓内相对串 `scripts/gates` / `scripts/公开标记.tsv`）。
//	③ 无 self-test 魔法串。
//	④ 他门：`check-error-kinds` 的 R3 只认**代码位**的 error 构造调用（**剥注释**）、且该门件头载明
//	  **不扫** JSON 信封标签字面 ⇒ 本件零该调用 ⇒ 命中 0。`check-hardcoded-private-paths` 扫 `core/`
//	  的**私有路径字面**（家目录 / 绝对私有路径）⇒ 本件零绝对路径字面 ⇒ 命中 0。
//	  落件后以同根串影子仓 A/B（新件在/不在两态各跑该二门）核 `sha16` 逐字相同（见回执）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ── 常量（照 py 同名量逐字）────────────────────────────────────────────────
const (
	gsrGatesDir  = "scripts/gates"    // py GATES_DIR = os.path.join("scripts","gates")
	gsrMarksRel  = "scripts/公开标记.tsv" // py MARKS_REL = os.path.join("scripts","公开标记.tsv")
	gsrProg      = "check-gate-script-registered.py"
	gsrKindLbl   = "GateScriptRegistered"
	gsrSchemaTag = "zerg/v1"
)

// gsrWS —— py `\s` / `str.isspace()` 的 Unicode 认可集（同 CPython `_PyUnicode_IsWhitespace`）。
const gsrWS = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

var (
	gsrLeadWS  = regexp.MustCompile(`^[` + gsrWS + `]+`)
	gsrTrailWS = regexp.MustCompile(`[` + gsrWS + `]+$`)
	// gsrNegNumRe —— argparse 判定「负数不算旗标」的同一正则。
	gsrNegNumRe = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)
)

// gsrPyStrip —— py `str.strip()` 的无参形态（剥两端 py 认可的 Unicode 空白）。
func gsrPyStrip(s string) string {
	s = gsrLeadWS.ReplaceAllString(s, "")
	s = gsrTrailWS.ReplaceAllString(s, "")
	return s
}

// gsrTextLines —— py 文本模式逐行迭代（universal newlines：`\r\n` 与裸 `\r` 折 `\n`）。
func gsrTextLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

// gsrPyJoin —— 复刻 CPython `posixpath.join`（空段 / 后段绝对路径 / 尾斜杠三态逐段直译）。
func gsrPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// gsrJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的字符串字面（保 UTF-8、逃控制符）。
func gsrJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// gsrBuildJSON —— py `json.dumps({…}, ensure_ascii=False)` 逐字节复刻
// （键序 = 插入序 schema→kind→gate_scripts→unregistered · 默认分隔符 `, ` 与 `: ` · 行内无缩进）。
func gsrBuildJSON(n int, missing []string) string {
	var b strings.Builder
	b.WriteString(`{"schema": `)
	b.WriteString(gsrJSONStr(gsrSchemaTag))
	b.WriteString(`, "kind": `)
	b.WriteString(gsrJSONStr(gsrKindLbl))
	b.WriteString(`, "gate_scripts": `)
	b.WriteString(strconv.Itoa(n))
	b.WriteString(`, "unregistered": [`)
	for i, m := range missing {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(gsrJSONStr(m))
	}
	b.WriteString(`]}`)
	return b.String()
}

// gsrGateScripts —— py `gate_scripts(root)`：保 readdir 原序列 `.py`/`.sh` 门件（相对路径）。
//
// 退码 st：0 成表 · 1 目录不在盘（py 返回 None）· 2 目录读不到（py 会裸回溯 rc=1 ⇒ Go fail-closed）。
func gsrGateScripts(root string) ([]string, int) {
	d := gsrPyJoin(root, gsrGatesDir)
	st, err := os.Stat(d) // py os.path.isdir 跟随符号链接 ⇒ 用 os.Stat（✗ 不用 dirent.IsDir）
	if err != nil || !st.IsDir() {
		return nil, 1
	}
	f, err := os.Open(d)
	if err != nil {
		return nil, 2
	}
	defer func() { _ = f.Close() }()
	ents, err := f.ReadDir(-1) // ★ 保 readdir 原序（✗ 不用会排序的 os.ReadDir）
	if err != nil {
		return nil, 2
	}
	out := []string{}
	for _, e := range ents {
		name := e.Name()
		if strings.HasSuffix(name, ".py") || strings.HasSuffix(name, ".sh") {
			out = append(out, gsrGatesDir+"/"+name)
		}
	}
	return out, 0
}

// gsrRegistered —— py `registered(root)`：读 `公开标记.tsv` 第一列（路径）成集。
//
// 退码 st：0 成集（含空集）· 1 表不在盘 / 非普通件（py 返回 None）· 2 读失败 / 非 UTF-8
// （py 裸回溯 rc=1 ⇒ Go fail-closed）。
func gsrRegistered(root string) (map[string]bool, int) {
	p := gsrPyJoin(root, gsrMarksRel)
	st, err := os.Stat(p) // py os.path.isfile 跟随符号链接
	if err != nil || !st.Mode().IsRegular() {
		return nil, 1
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, 2
	}
	if !utf8.Valid(data) {
		return nil, 3 // py 抛 UnicodeDecodeError（未捕 ⇒ 回溯 rc=1）⇒ Go fail-closed rc=2
	}
	got := map[string]bool{}
	for _, ln := range gsrTextLines(string(data)) {
		if gsrPyStrip(ln) == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		first := ln
		if i := strings.IndexByte(ln, '\t'); i >= 0 {
			first = ln[:i]
		}
		got[gsrPyStrip(first)] = true
	}
	return got, 0
}

// gsrRun —— py `run(root, as_json)` 的逐字复刻。
func gsrRun(stdout, stderr io.Writer, root string, asJSON bool) int {
	dirPath := gsrPyJoin(root, gsrGatesDir)
	gs, st := gsrGateScripts(root)
	if st == 1 {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：门件目录不在盘：%s\n", dirPath)
		return 2
	}
	if st == 2 {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：门件目录读不到（py 原件此档回溯 rc=1 · 见未覆盖面）：%s\n", dirPath)
		return 2
	}
	if len(gs) == 0 { // ★ 空域守卫：空域 ⇒ fail-closed（✗ 不许静默当「全登记」）
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：空域 —— %s 下 0 个 .py/.sh 门件（取不到输入 ⇒ 空域 ≠ 全登记）\n", dirPath)
		return 2
	}
	reg, rst := gsrRegistered(root)
	if rst == 1 {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：标记表读不到：%s\n", gsrPyJoin(root, gsrMarksRel))
		return 2
	}
	if rst != 0 {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：标记表读不到（py 原件此档回溯 rc=1 · 见未覆盖面）：%s\n", gsrPyJoin(root, gsrMarksRel))
		return 2
	}
	missing := []string{}
	for _, g := range gs { // ★ 承运 gs 的 readdir 原序
		if !reg[g] {
			missing = append(missing, g)
		}
	}
	if asJSON {
		fmt.Fprintln(stdout, gsrBuildJSON(len(gs), missing))
	} else {
		fmt.Fprintln(stdout, "── 门件登记对拍（scripts/gates/* ⇕ 公开标记.tsv）──")
		fmt.Fprintf(stdout, "  门件 %d 件 · 填表已登记 %d 件 · **未登记 %d 件**\n", len(gs), len(reg), len(missing))
		for _, m := range missing {
			fmt.Fprintf(stdout, "  ✗ 未登记：%s\n", m)
		}
	}
	if len(missing) > 0 {
		return 1
	}
	return 0
}

// gsrSelfTest —— py `self_test()`：四格（正控未登记 ⇒ rc=1 · 补登回绿 ⇒ rc=0 ·
// 标记表缺 ⇒ rc=2 · 空域 ⇒ rc=2），合成件全在 `tempfile.TemporaryDirectory()` 内（自带回收）。
//
// ★ `defer os.RemoveAll` 只在**建点成功支**（✗ 不无条件 —— 失败回落系统临时目录时无条件 defer
// 会删整个 OS 临时目录）。
func gsrSelfTest(stdout, stderr io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "tmp") // py TemporaryDirectory 缺省 prefix
	if err != nil {
		fmt.Fprintln(stdout, "自检 FAIL（4 格）")
		return 1
	}
	defer func() { _ = os.RemoveAll(td) }() // 建点成功支：TemporaryDirectory 语义 = 自带回收
	_ = os.MkdirAll(gsrPyJoin(td, gsrGatesDir), 0o755)
	aPy := gsrPyJoin(td, gsrGatesDir, "check-a.py")
	bSh := gsrPyJoin(td, gsrGatesDir, "check-b.sh")
	marks := gsrPyJoin(td, gsrMarksRel)
	_ = os.WriteFile(aPy, []byte("# a\n"), 0o644)
	_ = os.WriteFile(bSh, []byte("# b\n"), 0o644)
	_ = os.WriteFile(marks, []byte("scripts/gates/check-a.py\tno\tB 类\n"), 0o644)
	if gsrRun(stdout, stderr, td, false) != 1 {
		fmt.Fprintln(stdout, "  ✗ 正控失败：应报未登记 check-b.sh ⇒ rc=1")
		ok = false
	}
	f, _ := os.OpenFile(marks, os.O_APPEND|os.O_WRONLY, 0o644)
	if f != nil {
		_, _ = f.WriteString("scripts/gates/check-b.sh\tno\tB 类\n")
		_ = f.Close()
	}
	if gsrRun(stdout, stderr, td, false) != 0 {
		fmt.Fprintln(stdout, "  ✗ 成对回绿失败：补登后应 rc=0")
		ok = false
	}
	_ = os.Remove(marks)
	if gsrRun(stdout, stderr, td, false) != 2 {
		fmt.Fprintln(stdout, "  ✗ 负控失败：标记表缺 ⇒ rc=2")
		ok = false
	}
	// 空域负控：目录在盘但 0 门件 ⇒ rc=2（空域 ≠ 全登记）
	_ = os.Remove(aPy)
	_ = os.Remove(bSh)
	_ = os.WriteFile(marks, []byte("scripts/gates/check-a.py\tno\tB 类\n"), 0o644)
	if gsrRun(stdout, stderr, td, false) != 2 {
		fmt.Fprintln(stdout, "  ✗ 空域负控失败：0 门件应 rc=2")
		ok = false
	}
	if ok {
		fmt.Fprintln(stdout, "自检 PASS（4 格）")
		return 0
	}
	fmt.Fprintln(stdout, "自检 FAIL（4 格）")
	return 1
}

// gsrUsage —— argparse 的 usage 行（★ 按终端宽 80 ⇒ 折行宽 78：续行缩进 = len("usage: ") + len(prog) + 1）。
func gsrUsage() string {
	return "usage: " + gsrProg + " [-h] [--root ROOT] [--json]\n" +
		strings.Repeat(" ", len("usage: ")+len(gsrProg)+1) + "[--self-test]"
}

func gsrArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "%s\n%s: error: %s\n", gsrUsage(), gsrProg, msg)
	return 2
}

// gsrHelp —— argparse `-h/--help` 正文（照 py3.9 措辞复刻；★ py≥3.10 印 `options:` —— 见未覆盖面）。
func gsrHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, gsrUsage())
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "门件登记对拍（GAP-20260927-206）")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "optional arguments:")
	fmt.Fprintln(stdout, "  -h, --help   show this help message and exit")
	fmt.Fprintln(stdout, "  --root ROOT")
	fmt.Fprintln(stdout, "  --json")
	fmt.Fprintln(stdout, "  --self-test")
}

// gsrLongOpts —— 本件全部长旗标（供 argparse 唯一前缀缩写解析）。
var gsrLongOpts = []string{"--help", "--root", "--json", "--self-test"}

// gsrResolveLong —— argparse 规则：精确名优先，否则取**唯一前缀**；多匹 = 歧义。
func gsrResolveLong(name string) (string, bool, bool) {
	for _, o := range gsrLongOpts {
		if o == name {
			return o, false, true
		}
	}
	var matches []string
	for _, o := range gsrLongOpts {
		if strings.HasPrefix(o, name) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 1 {
		return matches[0], false, true
	}
	if len(matches) > 1 {
		return "", true, false
	}
	return "", false, false
}

// gsrLooksOption —— argparse 判定「下一 token 是否像旗标」（`-` 单字与负数不算）。
func gsrLooksOption(s string) bool {
	if !strings.HasPrefix(s, "-") || s == "-" {
		return false
	}
	if gsrNegNumRe.MatchString(s) {
		return false
	}
	return true
}

// checkGateScriptRegisteredGateMain —— 入口（= py `main()`，argparse 面逐字复刻）。
func checkGateScriptRegisteredGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	rootSet := false
	jsonOut := false
	selfTest := false
	var unrecognized []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			gsrHelp(stdout)
			return 0
		case strings.HasPrefix(a, "-h") && !strings.HasPrefix(a, "--") && len(a) > 2:
			// argparse 短旗标捆绑/显式参：`-hx` / `-h=1` ⇒ 「ignored explicit argument」。
			val := a[2:]
			if strings.HasPrefix(val, "=") {
				val = val[1:]
			}
			return gsrArgErr(stderr, "argument -h/--help: ignored explicit argument '"+val+"'")
		case a == "--":
			unrecognized = append(unrecognized, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := a, "", false
			if j := strings.Index(a, "="); j >= 0 {
				name, val, hasVal = a[:j], a[j+1:], true
			}
			canon, amb, ok := gsrResolveLong(name)
			if amb {
				return gsrArgErr(stderr, "ambiguous option: "+name)
			}
			if !ok {
				unrecognized = append(unrecognized, a)
				continue
			}
			switch canon {
			case "--help":
				if hasVal {
					return gsrArgErr(stderr, "argument -h/--help: ignored explicit argument '"+val+"'")
				}
				gsrHelp(stdout)
				return 0
			case "--root":
				if hasVal {
					root, rootSet = val, true
				} else {
					if i+1 >= len(argv) || gsrLooksOption(argv[i+1]) {
						return gsrArgErr(stderr, "argument --root: expected one argument")
					}
					i++
					root, rootSet = argv[i], true
				}
			case "--json":
				if hasVal {
					return gsrArgErr(stderr, "argument --json: ignored explicit argument '"+val+"'")
				}
				jsonOut = true
			case "--self-test":
				if hasVal {
					return gsrArgErr(stderr, "argument --self-test: ignored explicit argument '"+val+"'")
				}
				selfTest = true
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			unrecognized = append(unrecognized, a)
		default:
			unrecognized = append(unrecognized, a)
		}
	}
	if len(unrecognized) > 0 {
		return gsrArgErr(stderr, "unrecognized arguments: "+strings.Join(unrecognized, " "))
	}
	if selfTest {
		return gsrSelfTest(stdout, stderr)
	}
	if !rootSet {
		if v, ok := os.LookupEnv("ZERG_ROOT"); ok {
			root = v
		} else if wd, err := os.Getwd(); err == nil {
			root = wd
		} else {
			root = "."
		}
	}
	return gsrRun(stdout, stderr, root, jsonOut)
}

// ═══════════════════════ ★ 未覆盖面（逐条如实声明）═══════════════════════
//
//	① **py 版本相关措辞**：`-h` 正文 py≤3.9 印 `optional arguments:`、py≥3.10 印 `options:`；本件按
//	   py3.9（本机 `/usr/bin/python3` = 3.9.6）复刻 ⇒ 门跑环境若用 py≥3.10，帮助面 stdout 分叉（rc 恒 0）。
//	   未知旗标 / 缺值 / `--json=1` 的 error 正文同为 py 版本相关措辞。
//	② **终端宽相关折行**：usage 行的折行位置随 argparse 的 `shutil.get_terminal_size()`（本机非 TTY
//	   ⇒ 80 ⇒ 折行宽 78）⇒ 本件按 80 复刻两行形态；`COLUMNS` 另有值时 py 折行不同（rc 恒 0/2）。
//	③ **目录 / 标记表「在盘但读不到」档**：py 未捕 `OSError` / `UnicodeDecodeError` ⇒ **裸回溯 + rc=1**
//	   （C 类无兜底）；本件按新口径 fail-closed rc=2 ＋ 具名文案 ⇒ 该格只保「不给结论」语义、
//	   正文不逐字对齐（已声明为口径差）。
//	④ **符号链接路径下的缺省根**：py `os.getcwd()` ⟷ Go `os.Getwd()`（后者优先 `$PWD`）⇒ 对拍 / 夹具
//	   一律落**非符号链接路径**以对齐（真仓 cwd 非软链 ⇒ 不触发）。
//	⑤ **argparse 面**：本件复刻唯一前缀缩写（`--ro`/`--r` → `--root` 等）＋歧义 / 缺值 / 未识别三档文案
//	   ＋ `--help=1` / `--h=1` / `-h=1` / `-hx` / `-h1` 五档「ignored explicit argument」短旗标 / 长旗标
//	   显式参形态（帮助面标签用 argparse 的 `-h/--help`）。
//	⑥ **`--self-test` 档非确定面**：临时目录随机后缀（掩码规则 = 掩掉 `tmp<随机>` 整段）⇒ 两态 stderr
//	   掩码后逐字节同；已自证 py 连跑两发该段不同、掩码后相同。
//	⑦ **`str.splitlines` 类原语**：本件只用「按 `\n` 分行（universal newlines）」，未涉 `splitlines`
//	   的其余边界集（`\v`/`\f`/`\x1c-\x1f`/`\u2028` 等）—— `registered()` 的逐行读法两语言同构。
