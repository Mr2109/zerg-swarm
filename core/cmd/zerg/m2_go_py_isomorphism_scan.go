// m2_go_py_isomorphism_scan.go —— 门禁件 `scripts/gates/m2-go-py-isomorphism-scan.py` 的 Go 真搬
// （M2 · 抢池底粒 13）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案把判据实现面分两段走 —— M1 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面；
//	M2 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。本件 = M2 搬运面在**池底**选中并真搬的一枚：
//	验收判据 = 对同一件跑两态（py 原件 ↔ 新入口），stdout / stderr / 退码三面逐字相同。
//
// 选它 / 不选它的理由（纯静态只读 · 退码仅 0/2 · 无子进程/无联网/无计时/无 TTY · 行数最小）：
//
//	✓ 选 `m2-go-py-isomorphism-scan.py`（147 行 · 池内现读**最小的纯静态件**）：
//	  只读**仓内** `core/cmd/zerg/*.go`（`--root` 换根，缺省 cwd）；零 `subprocess` · 零 `socket` ·
//	  零 `urllib` · 零 `time` · 零 `random` · **零写面**（主逻辑只读）；退码仅 0/2。
//	✗ 同池其余纯静态件的排除理由（本单现读）：
//	  `check-registry-key-unique.py`（须复刻 CPython `json` 的**报错正文**才能逐字对拍，本单一轮不济）·
//	  `check-loose-exit-registry.py`（`import ast` ⇒ 须复刻 CPython AST 面）·
//	  `check-repo-symlink-farm.py`（吃 `$HOME` ＋ `realpath` 具名语义）·
//	  `check-selftest-knobs.py` / `check-write-authz.py`（自检在**生产路径**带写面/临时目录面）；
//	  `check-manifest-freshness-profile.py`（经 `importlib` 跨件调 `_git(...)` ⇒ 真跑 git）·
//	  其余候选命中「真跑子进程 / 写面 / 计时」。
//
// ⚠ 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `gate_inproc_table.go`）；
//	· **不**删 `scripts/gates/m2-go-py-isomorphism-scan.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· **已接线**：入口 `m2GoPyIsomorphismScanGateMain` 由 `gateCheckGoEntry["m2-go-py-isomorphism-scan"]` 分派（`zerg gate check m2-go-py-isomorphism-scan`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func m2GoPyIsomorphismScanGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（**不含程序名**）
//	  · 退码   —— 0 扫描完成 / 2 不给结论（扫描根不在盘 · 空域 · 自检不过 · 用法错 · 扫描面件读失败）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见件尾「已对拍面 / 未覆盖面」）
//
// ── 两行标准声明（本单必写）────────────────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py `--root` 缺省 `""` ⇒ `main` 里 `root = a.root or os.getcwd()`；
//	    Go 入口同构（`--root` 缺省/空 ⇒ `os.Getwd()`）。**两者皆从 cwd 取根**（py **不**从 `__file__`
//	    上溯）⇒ 本件「脚本位置 ↔ cwd」两侧推导**逐字同构**，无差异项。
//	(b) **自指检查要真判**：本件**在**自己的扫描面内吗？—— **是**（本件落 `core/cmd/zerg/`，
//	    正是判据面）。⇒ 防御：本件正文**不写**任何能被本门计数器**非对称**命中的连续字面
//	    （计数器串本身如 `filepath\.Join\(` 因中间隔 `\` 而不自匹配）；本件自带码面（`os.ReadDir(` 等）
//	    会被**两侧同度量**扫到 ⇒ 对拍恒等，**非**分歧面。已按「一旦被 `git ls-files` 跟踪」复核。
//
// ⑤ `--evidence GAP-20261001-13`（在册 M2 承接 · 语义对口）。
//
// 实现要点（与 py 逐处对齐、✗ 不靠「看起来等价」）：
//
//	· `COUNTERS` 全 ASCII 字符类（`\.`/`\s`/`\b`）；py `\s`/`\b` 是 Unicode 语义、Go RE2 为 ASCII
//	  ⇒ 真 `.go` 源里不出现 NBSP 一类 ⇒ 不触发（列入未覆盖面）。
//	· `sorted(os.listdir(d))` = 码点序；Go `sort.Strings` = 字节序（UTF-8 保码点序 ⇒ 等价）。
//	· 排序键 `(-raw_path_ops, -regex_unexpanded, file)` ⇒ `sort.SliceStable` 同构（✗ 不用 map 迭代）。
//	· `json.dumps(..., ensure_ascii=False)` 的**默认分隔符**（`, ` / `: `）与 **dict 插入序**逐字复刻
//	  （✗ 不用 `encoding/json`：Go 的 `,`/`:` 无空格、且 map 无序）。
//	· 自检 `tempfile.mkdtemp` **无回收**（py A 类泄漏）⇒ 本件按新口径补 `defer os.RemoveAll`
//	  （钉在「建点成功」那一支），输出不含临时路径 ⇒ 对拍无差。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

const m2sProg = "m2-go-py-isomorphism-scan.py"

// m2sVersion —— py `VERSION`（逐字；用于 stderr 前缀）。
const m2sVersion = "0.1.0（候件 · 未接线 · 缺口 GAP-20261001-51）"

// m2sUsage —— py argparse 的用法串（对拍 c09/c10 已逐字节证）。
const m2sUsage = "usage: m2-go-py-isomorphism-scan.py [-h] [--root ROOT] [--json] [--self-test]\n"

// m2sHelpText —— py argparse 的**逐字**帮助正文（`-h`/`--help`）。
const m2sHelpText = m2sUsage +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --json\n" +
	"  --self-test\n"

// m2sLongOpts —— argparse 已注册的**长选项串**（按注册序；用于前缀缩写匹配）。
var m2sLongOpts = []string{"--help", "--root", "--json", "--self-test"}

// m2sScanRel —— py `SCAN_REL = os.path.join("core", "cmd", "zerg")`。
var m2sScanRel = []string{"core", "cmd", "zerg"}

// m2sHdr —— py 输出表头（也决定 `raw_path_ops` 之外各列的展示序）。
var m2sHdr = []string{"file", "join", "abs", "rel", "evalsymlinks", "readdir", "walkdir",
	"direntry_isdir", "regex_unexpanded", "pyhelper", "raw_path_ops"}

// ── 计数器（py `COUNTERS` · 按注册序）────────────────────────────────────────────────

var m2sCounters = []struct {
	name string
	re   *regexp.Regexp
}{
	{"join", regexp.MustCompile(`filepath\.Join\s*\(`)},
	{"abs", regexp.MustCompile(`filepath\.Abs\s*\(`)},
	{"rel", regexp.MustCompile(`filepath\.Rel\s*\(`)},
	{"evalsymlinks", regexp.MustCompile(`filepath\.EvalSymlinks\s*\(`)},
	{"readdir", regexp.MustCompile(`os\.ReadDir\s*\(`)},
	{"walkdir", regexp.MustCompile(`filepath\.WalkDir\s*\(`)},
	{"direntry_isdir", regexp.MustCompile(`\.IsDir\s*\(`)},
	{"pyhelper", regexp.MustCompile(`\bPy[A-Z][A-Za-z0-9]*\s*\(`)},
}

// m2sRegexSkel —— py `REGEX_SKEL_RE = re.compile(r"\\(s|w|d|b)\b")`。
var m2sRegexSkel = regexp.MustCompile(`\\(s|w|d|b)\b`)

// m2sRow —— 一件 `.go` 的现读计数。
type m2sRow struct {
	file            string
	join            int
	abs             int
	rel             int
	evalsymlinks    int
	readdir         int
	walkdir         int
	direntryIsdir   int
	regexUnexpanded int
	pyhelper        int
	rawPathOps      int
}

// m2sVal —— 取某列的值（按 `m2sHdr` 名）。
func (r m2sRow) m2sVal(col string) int {
	switch col {
	case "join":
		return r.join
	case "abs":
		return r.abs
	case "rel":
		return r.rel
	case "evalsymlinks":
		return r.evalsymlinks
	case "readdir":
		return r.readdir
	case "walkdir":
		return r.walkdir
	case "direntry_isdir":
		return r.direntryIsdir
	case "regex_unexpanded":
		return r.regexUnexpanded
	case "pyhelper":
		return r.pyhelper
	case "raw_path_ops":
		return r.rawPathOps
	}
	return 0
}

// ── 基础件（与 py 口径对齐）──────────────────────────────────────────────────────────

// m2sJoin —— 复刻 posixpath.join（逐段规则直译；✗ 不用 filepath.Join）。
func m2sJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "/"): // 后段绝对 ⇒ 重置前面所有段
			out = p
		case out == "" || strings.HasSuffix(out, "/"): // 首段 / 上一段落点自带分隔符
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// m2sJSONStr —— 复刻 py `json.dumps(<str>, ensure_ascii=False)` 的字符串编码。
func m2sJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ── 判据体（与 py 同构）──────────────────────────────────────────────────────────────

// m2sScan —— py `scan(root)`：⇒ (rows, errMsg, readFails)。★ readFails = B 类读失败登记面
// （扫描面 `.go` 件读不了 ⇒ 逐件登记 仓内相对路径 + 原因；调用方汇总后 rc=2 不给结论）。
func m2sScan(root string) ([]m2sRow, string, []string) {
	d := m2sJoin(append([]string{root}, m2sScanRel...)...)
	if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
		return nil, "扫描根不在盘：" + d, nil
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		return nil, "扫描根不在盘：" + d, nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var rows []m2sRow
	var readFails []string
	for _, fn := range names {
		if !strings.HasSuffix(fn, ".go") || strings.HasSuffix(fn, "_test.go") {
			continue
		}
		b, err := os.ReadFile(m2sJoin(d, fn))
		if err != nil {
			// ★ B 类真口子修（原 py `except OSError: continue` 静默 · ✗ 不再静默）：扫描面 .go 件
			//   读不了（权限 / 竞态）⇒ 登记（仓内相对路径 + 原因），汇总后按 rc=2 不给结论。
			//   体例照同包已治件 check_no_symlink_farm.go / check_lineref_limit.go（登记 + 判红 + rc=2）。
			readFails = append(readFails, m2sJoin("core", "cmd", "zerg", fn)+"（"+err.Error()+"）")
			continue
		}
		txt := string(b)
		r := m2sRow{file: fn}
		for i, c := range m2sCounters {
			n := len(c.re.FindAllStringIndex(txt, -1))
			switch i {
			case 0:
				r.join = n
			case 1:
				r.abs = n
			case 2:
				r.rel = n
			case 3:
				r.evalsymlinks = n
			case 4:
				r.readdir = n
			case 5:
				r.walkdir = n
			case 6:
				r.direntryIsdir = n
			case 7:
				r.pyhelper = n
			}
		}
		r.regexUnexpanded = len(m2sRegexSkel.FindAllStringIndex(txt, -1))
		r.rawPathOps = r.join + r.abs + r.rel + r.evalsymlinks
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.rawPathOps != b.rawPathOps {
			return a.rawPathOps > b.rawPathOps
		}
		if a.regexUnexpanded != b.regexUnexpanded {
			return a.regexUnexpanded > b.regexUnexpanded
		}
		return a.file < b.file
	})
	return rows, "", readFails
}

// m2sTotalsLine —— py 末行 `json.dumps(tot, ensure_ascii=False)`（键序 = 表头去 file）。
func m2sTotalsLine(rows []m2sRow) string {
	var b strings.Builder
	b.WriteByte('{')
	first := true
	for _, col := range m2sHdr {
		if col == "file" {
			continue
		}
		if !first {
			b.WriteString(", ")
		}
		first = false
		sum := 0
		for _, r := range rows {
			sum += r.m2sVal(col)
		}
		fmt.Fprintf(&b, "%s: %d", m2sJSONStr(col), sum)
	}
	b.WriteByte('}')
	return b.String()
}

// m2sJSONDoc —— py `json.dumps({"schema":…, "rows": rows}, ensure_ascii=False)`。
func m2sJSONDoc(rows []m2sRow) string {
	var b strings.Builder
	b.WriteString(`{"schema": `)
	b.WriteString(m2sJSONStr("zerg/m2-go-py-isomorphism/v1"))
	b.WriteString(`, "rows": [`)
	for i, r := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteByte('{')
		for j, col := range m2sHdr {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString(m2sJSONStr(col))
			b.WriteString(": ")
			if col == "file" {
				b.WriteString(m2sJSONStr(r.file))
			} else {
				fmt.Fprintf(&b, "%d", r.m2sVal(col))
			}
		}
		b.WriteByte('}')
	}
	b.WriteString("]}")
	return b.String()
}

// m2sPrintText —— py 文本档输出（表头 ＋ 各明细行 ＋ 合计 ＋ 说明）。
func m2sPrintText(stdout io.Writer, rows []m2sRow) {
	fmt.Fprint(stdout, strings.Join(m2sHdr, "\t")+"\n")
	for _, r := range rows {
		vals := make([]string, len(m2sHdr))
		for i, col := range m2sHdr {
			if col == "file" {
				vals[i] = r.file
			} else {
				vals[i] = fmt.Sprintf("%d", r.m2sVal(col))
			}
		}
		fmt.Fprint(stdout, strings.Join(vals, "\t")+"\n")
	}
	fmt.Fprintf(stdout, "# 合计：%s\n", m2sTotalsLine(rows))
	fmt.Fprint(stdout, "# 说明：raw_path_ops = join+abs+rel+evalsymlinks（裸算子用量，新口径应改自写 Py* 一族）；\n")
	fmt.Fprint(stdout, "#       「哪些必须重落」归收口批拍板 —— 本件只出可复现清单。\n")
}

// ── 自检（合成夹具 · 不碰真目标）────────────────────────────────────────────────────

// m2sSelfXGo —— py 自检夹具 `x.go` 的逐字内容（`_` 拼串免连续可执行串）。
const m2sSelfXGo = "package main\nimport \"path/filepath\"\n" +
	"func f(root string) { _ = filepath.Join(root, \"a\"); _ = os.ReadDir(root); " +
	"_ = PyJoin(\"a\",\"b\"); re := \"\\\\s+\"; _ = re }\n"

// m2sSelfTest —— py `self_test()`（5 格合成夹具；✗ 不碰真目标）。
func m2sSelfTest(stdout io.Writer) int {
	d, err := os.MkdirTemp("", "m2scan-selftest-")
	if err != nil {
		fmt.Fprint(stdout, "  ✗ 自检夹具建点失败 ⇒ 不给结论\n")
		return 2
	}
	defer os.RemoveAll(d) // ★ 安全删点：钉在「建点成功」支（py 无回收 · 本件止血，输出不含路径 ⇒ 无差）
	scanDir := m2sJoin(d, "core", "cmd", "zerg")
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		fmt.Fprint(stdout, "  ✗ 自检夹具建目录失败 ⇒ 不给结论\n")
		return 2
	}
	writeErr := os.WriteFile(m2sJoin(scanDir, "x.go"), []byte(m2sSelfXGo), 0o644)
	_ = os.WriteFile(m2sJoin(scanDir, "x_test.go"), []byte("package main\nvar _ = filepath.Join\n"), 0o644)

	rows, errMsg, _ := m2sScan(d)
	if writeErr != nil {
		errMsg = "写夹具失败：" + writeErr.Error()
	}
	has := func(f func() bool) bool { return errMsg == "" && len(rows) > 0 && f() }
	cases := []struct {
		name string
		good bool
	}{
		{"① 扫描出来一件（测试件排除）", errMsg == "" && len(rows) == 1 && rows[0].file == "x.go"},
		{"② filepath.Join 计数=1", has(func() bool { return rows[0].join == 1 })},
		{"③ os.ReadDir 计数=1", has(func() bool { return rows[0].readdir == 1 })},
		{"④ 自写 PyJoin 计数=1", has(func() bool { return rows[0].pyhelper == 1 })},
		{"⑤ 脏正则类计数>=1", has(func() bool { return rows[0].regexUnexpanded >= 1 })},
	}
	ok := true
	pass := 0
	for _, c := range cases {
		mark := "✗"
		if c.good {
			mark = "✓"
			pass++
		}
		fmt.Fprintf(stdout, "  %s %s\n", mark, c.name)
		ok = ok && c.good
	}
	fmt.Fprintf(stdout, "  self-test: %d/%d\n", pass, len(cases))
	if ok {
		return 0
	}
	return 2
}

// ── argparse 面（含前缀缩写）────────────────────────────────────────────────────────

// m2sArgErr —— 用法错（py argparse）：stderr 印 usage ＋ `prog: error: …`，退码 2。
func m2sArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, m2sUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", m2sProg, msg)
	return 2
}

// m2GoPyIsomorphismScanGateMain —— 入口（= py `main()`）。
func m2GoPyIsomorphismScanGateMain(stdout, stderr io.Writer, argv []string) int {
	rootVal := ""
	rootSet := false
	asJSON := false
	selfTest := false
	var extras []string
	afterDD := false

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if afterDD {
			extras = append(extras, a)
			continue
		}
		if a == "--" {
			afterDD = true
			continue
		}
		if a == "-h" {
			fmt.Fprint(stdout, m2sHelpText)
			return 0
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			extras = append(extras, a)
			continue
		}
		if !strings.HasPrefix(a, "--") {
			extras = append(extras, a) // 非 -h 的短选项
			continue
		}
		name, explicit := a, ""
		hasExplicit := false
		if idx := strings.Index(a, "="); idx >= 0 {
			name, explicit, hasExplicit = a[:idx], a[idx+1:], true
		}
		var matches []string
		for _, o := range m2sLongOpts {
			if strings.HasPrefix(o, name) {
				matches = append(matches, o)
			}
		}
		if len(matches) == 0 {
			extras = append(extras, a)
			continue
		}
		if len(matches) > 1 {
			return m2sArgErr(stderr, "ambiguous option: "+name+" could match "+strings.Join(matches, ", "))
		}
		switch matches[0] {
		case "--help":
			if hasExplicit {
				return m2sArgErr(stderr, "argument -h/--help: ignored explicit argument '"+explicit+"'")
			}
			fmt.Fprint(stdout, m2sHelpText)
			return 0
		case "--json":
			if hasExplicit {
				return m2sArgErr(stderr, "argument --json: ignored explicit argument '"+explicit+"'")
			}
			asJSON = true
		case "--self-test":
			if hasExplicit {
				return m2sArgErr(stderr, "argument --self-test: ignored explicit argument '"+explicit+"'")
			}
			selfTest = true
		case "--root":
			if hasExplicit {
				rootVal, rootSet = explicit, true
			} else if i+1 < len(argv) && (argv[i+1] == "-" || !strings.HasPrefix(argv[i+1], "-")) {
				i++
				rootVal, rootSet = argv[i], true
			} else {
				return m2sArgErr(stderr, "argument --root: expected one argument")
			}
		}
	}

	if len(extras) > 0 {
		return m2sArgErr(stderr, "unrecognized arguments: "+strings.Join(extras, " "))
	}
	if selfTest {
		return m2sSelfTest(stdout)
	}
	root := rootVal
	if !rootSet || root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}
	rows, errMsg, readFails := m2sScan(root)
	if errMsg != "" {
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 2）\n", m2sVersion, errMsg)
		return 2
	}
	if len(readFails) > 0 {
		// ★ B 类读失败不再静默（原 py `except OSError: continue`）：响亮 BLOCKED + 逐件点名 + rc=2。
		fmt.Fprintf(stderr, "BLOCKED —— 扫描面 .go 件读不了 %d 件（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论 · 读不到不许当健康）：\n", len(readFails))
		for _, u := range readFails {
			fmt.Fprintf(stderr, "  ── %s\n", u)
		}
		return 2
	}
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "%s: 空域 —— core/cmd/zerg 下 0 个非测试 .go（取不到输入）⇒ 不给结论（退码 2）\n", m2sVersion)
		return 2
	}
	if asJSON {
		fmt.Fprint(stdout, m2sJSONDoc(rows))
		fmt.Fprint(stdout, "\n")
		return 0
	}
	m2sPrintText(stdout, rows)
	return 0
}

// 已对拍面（逐字节比 rc/stdout/stderr）· 见同批 `m2_go_py_isomorphism_scan_test.go` 与回执。
// 未覆盖面：
//  ① py `\s` / `\b` 是 **Unicode** 语义，Go RE2 为 **ASCII** ⇒ 真 `.go` 源里出现 NBSP 一类
//     非 ASCII 空白/词界字符时两侧计数可能分叉（真仓实测 0 例；已声明）。
//  ② `open(..., encoding="utf-8")` 带 universal newlines（`\r\n`→`\n`）；本件按字节读（计数不受影响，
//     因计数器不跨行；仍声明）。非 UTF-8 源 py 会裸回溯（C 类），本件按字节读不报 ⇒ 该格不逐字对齐。
//  ③ py `os.listdir` 读目录失败在 `scan()` 内**不捕**（裸回溯 rc=1，C 类）；本件 fail-closed rc=2。
//  ④ 自检夹具 `mkdtemp` py **无回收**（A 类泄漏）；本件补 `defer os.RemoveAll`（输出不含路径 ⇒ 对拍无差）。
//  ⑤ argparse 极角落形态（短选项簇 `-hx` 等）未逐一复刻，只复刻被扫常见档。
//  ⑥ ★ 读失败面（B 类真口子已治 · 原 py `except OSError: continue` 静默）：py 扫描面内 `.go` 件读不了
//     时静默跳过（该件既不进清单也不判红）；本件改为登记 + stderr `BLOCKED` 逐件点名 + 退 2（✗ 不再
//     静默）⇒ 此档两态（py 静默 ⟷ 本件退 2）不再逐字对齐（真仓零此形态）。
