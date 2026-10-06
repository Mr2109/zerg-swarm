// check_dryrun_yes_conflict_coverage.go —— 判据件
// `scripts/gates/check-dryrun-yes-conflict-coverage.py`（170 行）的 **Go 真搬**（M2 · 搬运面 ·
// 入口 `checkDryrunYesConflictCoverageGateMain`）。
//
//	M2 = 按价值真搬 Go（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码 三面逐字相同**（除下列显式声明的 argparse
//	面差异与未覆盖面）。
//
// 本件做的事（与 py 同构 · 只读 · 不开子进程 · 不改任何状态）：扫描 `core/cmd/zerg/*.go`
//
//	（去测试件 · 去 main.go），认「件读【inv 点 dryRun】」为**应产者**，认「件含共享判定
//	dryRunYesConflict 或执行门 cmdGuarded」为**真产者**；应产者 ∖ 真产者 里落在冻结名单
//	（只读干跑面 READ_ONLY_FACES）之外的 ⇒ 报红；空域（0 应产者）⇒ fail-closed rc=2。
//
// 适格（照实）：纯静态只读 —— 零 subprocess / 零联网 / 零 TTY / 零计时 / 默认档零写面；
//
//	`tempfile` 只在 `--self-test` 里造件（py 用 `mkdtemp` 无回收 = A 类 tempdir 泄漏；Go 半
//	按「建点成功那一支」`defer os.RemoveAll` 止血，输出不可见差异 ⇒ 已实测 delta=0）。
//	外部树走显式旋钮 `--root`（缺省 = cwd，与 py `a.root or os.getcwd()` 逐字同构）。
//	退码 0/1/2 三档齐 · 现读 py import 面：argparse / json / os / re / sys / tempfile。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· ✗ 不在 `main.go` 里注册命令 · ✗ 不改 `gate_inproc_table.go` · ✗ 删 py 原件；
//	· 入口 `checkDryrunYesConflictCoverageGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面 / `family_*.go` / 已落旧件。
//
// ═══════════════════ ★ 两条出厂声明 ═══════════════════
//
//	(a) 默认根定位差异：本 py 缺省根 = `a.root or os.getcwd()`（**cwd**，非 `__file__`）⟷ Go 同为
//	  cwd（`os.Getwd`，后者优先 `$PWD`）⇒ 两态同构，唯一残留是「符号链接 cwd」：py `os.getcwd()`
//	  返词法真路径、Go `os.Getwd()` 可能读 `$PWD` ⇒ 对拍/夹具一律落**非符号链接路径**（真仓 cwd
//	  非软链 ⇒ 不触发）。
//	(b) 自指检查：★ 本件**落在它自己的扫描面内**（本件就在 `core/cmd/zerg/`）⇒ **凡 `inv` 接 `.`
//	  接 `dryRun` 的连续字面一律拆串**（`dycInvDry` = 两段拼接），并把两处运行时文案（空域 stderr
//	  ＋ 报红 stdout）同法拆串。已用单测 `TestDycSourceSelfRefClean` **现读本件源**断言拆串有效
//	  （零命中）。判据件本体（py）住 `scripts/gates/`、不在扫描面内，故不受此条影响。
//
// ═══════════════════ ★ 遍历序（逐点交代）═══════════════════
//
//	py `sorted(os.listdir(d))` —— **显式排序**（按码点；UTF-8 下 == Go 字节序）⇒ Go 用
//	`os.ReadDir(-1)` 收全名 ＋ `sort.Strings` 复现。输出行序（reads / covered / exempt / missing
//	明细）的决定者 = 这份排序。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	dycVersion = "0.1.0（已接线 · 门98 · tri-report 只报告 · 缺口 GAP-20260927-392/374/389）"
	dycScanRel = "core/cmd/zerg"
	dycProg    = "check-dryrun-yes-conflict-coverage.py"
	dycSchema  = "zerg/dryrun-yes-conflict-coverage/v1"
	// 拆串：避免本件源码被自己扫成「应产者」（见件头声明 (b)）。
	dycInvDry = "inv." + "dryRun"
)

// dycReadOnlyFaces —— py READ_ONLY_FACES（冻结名单：只读干跑面 · 有序）。
var dycReadOnlyFaces = []string{"export.go"}

// dycReadsRe —— py READS_RE `\binv\.dryRun\b`（拆串写，见声明 (b)）。
var dycReadsRe = regexp.MustCompile("\\binv\\." + "dryRun\\b")

// dycHelperRe —— py HELPER_RE `\bdryRunYesConflict\b|\bcmdGuarded\b`（拆串写）。
var dycHelperRe = regexp.MustCompile("\\bdryRun" + "YesConflict\\b|\\bcmd" + "Guarded\\b")

// ── py 路径原语 ────────────────────────────────────────────────────────────
// dycPyJoin —— 复刻 CPython `posixpath.join`（空段 / 后段绝对路径 / 尾斜杠三态逐段直译）。
func dycPyJoin(parts ...string) string {
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

// ── py json.dumps(ensure_ascii=False) 的字符串字面 ─────────────────────────
func dycJSONStr(s string) string {
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

func dycJSONStrList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = dycJSONStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ── 判据体 ────────────────────────────────────────────────────────────────
type dycRow struct {
	File   string
	Helper bool
}

// dycScan —— py `scan(root)`：返 (rows, errMsg)。errMsg != "" ⇒ 不给结论（rc=2）。
func dycScan(root string) ([]dycRow, string) {
	d := dycPyJoin(root, dycScanRel)
	st, err := os.Stat(d)
	if err != nil || !st.IsDir() {
		return nil, fmt.Sprintf("扫描根不在盘：%s", d)
	}
	entries, err := os.ReadDir(d)
	if err != nil {
		// py `os.listdir` 抛未捕 OSError（C 类无兜底）⇒ 本件按新口径 fail-closed（并入「扫描根不在盘」）。
		return nil, fmt.Sprintf("扫描根不在盘：%s", d)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	var rows []dycRow
	for _, fn := range names {
		if !strings.HasSuffix(fn, ".go") || strings.HasSuffix(fn, "_test.go") {
			continue
		}
		if fn == "main.go" {
			continue
		}
		b, err := os.ReadFile(dycPyJoin(d, fn))
		if err != nil {
			// py `except OSError: continue`（B 类读失败静默少判）—— 逐字照搬。
			continue
		}
		txt := string(b)
		if !dycReadsRe.MatchString(txt) {
			continue
		}
		rows = append(rows, dycRow{File: fn, Helper: dycHelperRe.MatchString(txt)})
	}
	return rows, ""
}

// dycJudge —— py `judge(rows, read_only)`：三列按 rows 序分别追加（缺/真产/豁免）。
func dycJudge(rows []dycRow, readOnly []string) (missing, covered, exempt []string) {
	inRO := func(f string) bool {
		for _, x := range readOnly {
			if x == f {
				return true
			}
		}
		return false
	}
	for _, r := range rows {
		switch {
		case r.Helper:
			covered = append(covered, r.File)
		case inRO(r.File):
			exempt = append(exempt, r.File)
		default:
			missing = append(missing, r.File)
		}
	}
	return missing, covered, exempt
}

// dycSelfTest —— py `self_test(out)`：6 格；建点成功才挂 `defer os.RemoveAll`（A 类止血）。
func dycSelfTest(out io.Writer) bool {
	ok := true
	total := 0
	d, err := os.MkdirTemp("", "drcov-selftest-")
	if err != nil {
		d = ""
	}
	if d != "" {
		defer os.RemoveAll(d)
	}
	scanDir := dycPyJoin(d, dycScanRel)
	_ = os.MkdirAll(scanDir, 0o755)
	put := func(name, body string) {
		_ = os.WriteFile(dycPyJoin(scanDir, name), []byte(body), 0o644)
	}
	put("a.go", "package main\nfunc f(inv *invocation) int { if "+dycInvDry+" { return 0 }; return 1 }\n")
	put("b.go", "package main\nfunc f(inv *invocation) int { if rc := dryRun"+"YesConflict(inv, nil); rc != 0 { return rc }; _ = "+dycInvDry+"; return 0 }\n")
	put("c.go", "package main\nfunc f(inv *invocation) int { if "+dycInvDry+" { return 0 }; return cmd"+"Guarded(inv, nil, nil) }\n")
	put("d_test.go", "package main\nfunc t() { _ = "+dycInvDry+" }\n") // 测试件：排除
	rows, errMsg := dycScan(d)
	files := make([]string, 0, len(rows))
	for _, r := range rows {
		files = append(files, r.File)
	}
	hasFile := func(n string) bool {
		for _, f := range files {
			if f == n {
				return true
			}
		}
		return false
	}
	missing, covered, _ := dycJudge(rows, dycReadOnlyFaces)
	_, _, exemptRO := dycJudge(rows, []string{"a.go"})

	empty, eerr := os.MkdirTemp("", "drcov-empty-")
	if eerr == nil {
		defer os.RemoveAll(empty)
		_ = os.MkdirAll(dycPyJoin(empty, dycScanRel), 0o755)
	}
	rowsE, errE := dycScan(empty)

	type tc struct {
		name string
		good bool
	}
	cases := []tc{
		{"① 应产者被抓住（a.go 无判定）", errMsg == "" && hasFile("a.go")},
		{"② a.go 报红（缺者报红）", len(missing) == 1 && missing[0] == "a.go"},
		{"③ b.go/c.go 真产者不入缺", len(covered) == 2 && covered[0] == "b.go" && covered[1] == "c.go"},
		{"④ 测试件被排除", !hasFile("d_test.go")},
		{"⑤ 冻结名单豁免", len(exemptRO) == 1 && exemptRO[0] == "a.go"},
		{"⑥ 空域：0 应产者（空域 ≠ 全绿）", errE == "" && len(rowsE) == 0},
	}
	nGood := 0
	for _, c := range cases {
		total++
		if c.good {
			nGood++
		}
		mark := "✗"
		if c.good {
			mark = "✓"
		}
		fmt.Fprintf(out, "  %s %s\n", mark, c.name)
		ok = ok && c.good
	}
	fmt.Fprintf(out, "  self-test: %d/%d\n", nGood, total)
	return ok
}

// ── argparse 面（照 py 同名旗标；本件无短旗标除 -h）────────────────────────
func dycUsage() string {
	first := "usage: " + dycProg + " [-h] [--root ROOT] [--json]"
	second := strings.Repeat(" ", len("usage: ")+len(dycProg)+1) + "[--emit-baseline] [--self-test]"
	return first + "\n" + second
}

func dycArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "%s\n%s: error: %s\n", dycUsage(), dycProg, msg)
	return 2
}

// dycHelp —— argparse `-h/--help` 正文（照 py3.9 措辞复刻；★ py≥3.10 印 `options:` —— 见未覆盖面）。
func dycHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, dycUsage())
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "optional arguments:")
	fmt.Fprintln(stdout, "  -h, --help       show this help message and exit")
	fmt.Fprintln(stdout, "  --root ROOT")
	fmt.Fprintln(stdout, "  --json")
	fmt.Fprintln(stdout, "  --emit-baseline")
	fmt.Fprintln(stdout, "  --self-test")
}

// dycLongOpts —— 本件全部长旗标（供 argparse 唯一前缀缩写解析）。
var dycLongOpts = []string{"--help", "--root", "--json", "--emit-baseline", "--self-test"}

// dycResolveLong —— argparse 规则：精确名优先，否则取唯一前缀；多匹 = 歧义。
func dycResolveLong(name string) (string, bool, bool) {
	for _, o := range dycLongOpts {
		if o == name {
			return o, false, true
		}
	}
	var matches []string
	for _, o := range dycLongOpts {
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

var dycNegNumRe = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

func dycLooksOption(s string) bool {
	if !strings.HasPrefix(s, "-") || s == "-" {
		return false
	}
	if dycNegNumRe.MatchString(s) {
		return false
	}
	return true
}

// ── 输出组装 ──────────────────────────────────────────────────────────────
func dycRowsFiles(rows []dycRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.File
	}
	return out
}

func dycJSON(rows []dycRow, covered, exempt, missing []string) string {
	return "{" +
		`"schema": ` + dycJSONStr(dycSchema) + ", " +
		`"reads": ` + dycJSONStrList(dycRowsFiles(rows)) + ", " +
		`"covered": ` + dycJSONStrList(covered) + ", " +
		`"exempt": ` + dycJSONStrList(exempt) + ", " +
		`"missing": ` + dycJSONStrList(missing) + "}"
}

// checkDryrunYesConflictCoverageGateMain —— 入口（= py `main(argv)` · argv 不含程序名）。
func checkDryrunYesConflictCoverageGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	jsonOut := false
	emitBaseline := false
	selfTest := false
	var unrecognized []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			dycHelp(stdout)
			return 0
		case strings.HasPrefix(a, "-h") && !strings.HasPrefix(a, "--") && len(a) > 2:
			val := a[2:]
			if strings.HasPrefix(val, "=") {
				val = val[1:]
			}
			return dycArgErr(stderr, "argument -h/--help: ignored explicit argument '"+val+"'")
		case a == "--":
			// argparse：`--` 本身也进「未识别」清单，其后全部 token 原样归入未识别（选项不再解析）。
			unrecognized = append(unrecognized, a)
			unrecognized = append(unrecognized, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := a, "", false
			if j := strings.Index(a, "="); j >= 0 {
				name, val, hasVal = a[:j], a[j+1:], true
			}
			canon, amb, ok := dycResolveLong(name)
			if amb {
				return dycArgErr(stderr, "ambiguous option: "+name)
			}
			if !ok {
				unrecognized = append(unrecognized, a)
				continue
			}
			switch canon {
			case "--help":
				if hasVal {
					return dycArgErr(stderr, "argument -h/--help: ignored explicit argument '"+val+"'")
				}
				dycHelp(stdout)
				return 0
			case "--root":
				if hasVal {
					root = val
				} else {
					if i+1 >= len(argv) || dycLooksOption(argv[i+1]) {
						return dycArgErr(stderr, "argument --root: expected one argument")
					}
					i++
					root = argv[i]
				}
			case "--json":
				if hasVal {
					return dycArgErr(stderr, "argument --json: ignored explicit argument '"+val+"'")
				}
				jsonOut = true
			case "--emit-baseline":
				if hasVal {
					return dycArgErr(stderr, "argument --emit-baseline: ignored explicit argument '"+val+"'")
				}
				emitBaseline = true
			case "--self-test":
				if hasVal {
					return dycArgErr(stderr, "argument --self-test: ignored explicit argument '"+val+"'")
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
		return dycArgErr(stderr, "unrecognized arguments: "+strings.Join(unrecognized, " "))
	}
	if selfTest {
		if dycSelfTest(stdout) {
			return 0
		}
		return 2
	}
	if root == "" {
		// py `a.root or os.getcwd()`（空串也是 falsy ⇒ 回落 cwd）。
		if wd, err := os.Getwd(); err == nil {
			root = wd
		} else {
			root = "."
		}
	}
	rows, errMsg := dycScan(root)
	if errMsg != "" {
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（退码 2）\n", dycVersion, errMsg)
		return 2
	}
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "%s: 空域 —— %s 下 0 件读 `"+dycInvDry+"`（取不到输入 ⇒ 空域 ≠ 全绿）⇒ 不给结论（退码 2）\n", dycVersion, dycScanRel)
		return 2
	}
	missing, covered, exempt := dycJudge(rows, dycReadOnlyFaces)
	if emitBaseline {
		fmt.Fprintln(stdout, "{"+`"read_only_faces": `+dycJSONStrList(dycReadOnlyFaces)+"}")
		return 0
	}
	if jsonOut {
		fmt.Fprintln(stdout, dycJSON(rows, covered, exempt, missing))
		if len(missing) > 0 {
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%s：应产者 %d 件 · 真产者 %d 件 · 冻结名单 %d 件\n", dycVersion, len(rows), len(covered), len(exempt))
	if len(missing) > 0 {
		fmt.Fprintln(stdout, "⇒ 报红（rc=1）：下列件读 `"+dycInvDry+"` 却既无共享判定、也不在只读干跑冻结名单 ——")
		fmt.Fprintln(stdout, "   「--dry-run 与 --yes 同给时静默吞 --yes」的病灶面：")
		for _, f := range missing {
			fmt.Fprintf(stdout, "   - core/cmd/zerg/%s\n", f)
		}
		fmt.Fprintln(stdout, "   治法：在**唯一写盘点之前**插 `if rc := dryRun"+"YesConflict(inv, stderr); rc != exitOK { return rc }`。")
		return 1
	}
	fmt.Fprintln(stdout, "⇒ 全绿（rc=0）：应产者全部落在真产者或只读干跑冻结名单里。")
	return 0
}

// ═══════════════════════ ★ 未覆盖面（逐条如实声明）═══════════════════════
//
//	① **py 版本相关措辞**：`-h` 正文 py≤3.9 印 `optional arguments:`、py≥3.10 印 `options:`；本件按
//	   py3.9（本机 `/usr/bin/python3`）复刻 ⇒ 门跑环境若用 py≥3.10 ⇒ 帮助面 stdout 分叉（rc 恒 0）。
//	   未知旗标 / 缺值 / `--json=1` 的 error 正文同为 py 版本相关措辞。usage 续行缩进 = argparse 按
//	   宽度折行得 `len("usage: ")+len(prog)+1`（= 45 空格）。
//	② **非 UTF-8 件**：py `open(..., encoding="utf-8")` 读到非法字节抛未捕 UnicodeDecodeError ⇒ 裸
//	   回溯 rc=1（C 类无兜底）；本件 Go 以字节当串（RE2 按字节匹配）⇒ 不回溯。真仓件件合法 UTF-8
//	   ⇒ 不触发；声明为未覆盖。
//	③ **符号链接 cwd / 扫描目录自身含链接**：见声明 (a)。
//	④ **`--` 之后的 token**：argparse 视 `--` 与其后全部为「未识别」—— 已逐字照搬。
