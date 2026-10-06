// check_step_switch.go —— 门禁件 `scripts/gates/check-step-switch.py` 的 Go 真搬（M2 · 批-3 搬运粒 8）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案把判据实现面分两段走 —— M1 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面；
//	M2 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。本件 = M2 搬运面在**候选区间首字母 p–z** 内
//	选中并真搬的一枚：验收判据 = 对同一件跑两态（py 原件 ↔ 新入口），stdout / stderr / 退码三面逐字相同。
//
// 选它 / 不选它的理由（纯静态只读 · 退码仅 0/1/2 · 无子进程/无计时/无随机/无写面 · 行数最小）：
//
//	✓ 选 `check-step-switch.py`（183 行 · 键名 `step-switch` ⇒ 首字母 `s` ∈ p–z）：
//	  只读**一个仓内件**（默认 `<根>/scripts/gates/precommit-gates.sh`，可 `--file` 换）；
//	  零 `subprocess` · 零 `time` · 零 `random` · 零 `socket` · **零写面**（只有读 open）；
//	  退码仅 0/1/2；输出定序（按文件出现序 ＋ 定序名册）。
//	✗ 不选同桶（p–z）内其余未搬件（现读 import/危险形态面后的排除理由）：
//	  `check-partial-run-acceptance.py`（subprocess+mkdtemp）· `check-selfcount.py`（subprocess+mkdtemp）·
//	  `check-stale-frozen.py`（subprocess+mkdtemp+importlib）· `check-write-face-gate.py`（importlib+写面）·
//	  `check-verify-cmd-real.py`（mkdtemp+写面）· `check-registry-key-unique.py` / `check-selftest-knobs.py` /
//	  `check-write-authz.py`（tempfile+写面）· `check-stepcount-drift.py`（mkdtemp+写面）·
//	  `check-repo-symlink-farm.py`（tempfile+environ）· `check-public-marks-tracked-parity.py`（subprocess+写面）·
//	  `check-py-go-pairing-drift.py`（**另路独占 · 本单明令禁碰**）。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-step-switch.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `stepSwitchGateMain` 目前**无调用点** —— 这是 M2 的刻意状态。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func stepSwitchGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（**不含程序名**）
//	  · 退码   —— 0 表自洽 / 1 有违例 / 2 不给结论（缺件 · 空转 · 自检不过 · 用法错）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见件尾「已对拍面 / 未覆盖面」）
//
// ── 两行标准声明（本单必写）────────────────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py 的 `DEFAULT_FILE = os.path.join(REPO, "scripts","gates","precommit-gates.sh")`，
//	    其中 `REPO = dirname(dirname(dirname(abspath(__file__))))`（起于**脚本目录**三级上溯）；
//	    Go 入口**无 `__file__` 语境** ⇒ 本件从 **cwd** 上溯 ≤6 层找**含 `scripts/gates` 目录**的那个根，
//	    找不到回落 cwd。落在**同一真仓、cwd=仓根**时两者路径推导**无差异**（对拍 c01 已逐字证）；
//	    「脚本位置 ≠ cwd」时**路径推导面不同** ⇒ 不属逐字对齐承诺。
//	(b) **自指检查要真判**：本件**在**自己的扫描面内吗？—— **否**（自指为假）。本门判据面恒为**被扫的那**一个
//	    表件（默认 `scripts/gates/precommit-gates.sh`）；本件落 `core/cmd/zerg/check_step_switch.go`，
//	    不在被扫件内 ⇒ 本门读不到本件自己。依据（两条）：① py `main()` 只有一处读面 `open(args.file)`，
//	    Go 侧同构（`sswReadText(fileVal)`）；② 判据点名的「宽松出口」名册 `sswLooseExit` 只匹配**被扫行文本**，
//	    不扫本件源码。⇒ 据此外本件正文可按原字面量直写判据要匹配的形态（含 `add_step …` 样例）；
//	    反过来说：若日后有人把本件挪进被扫件的位置，须立刻按自指消解改造（拆串/转义）。
//
// ⑤ `--evidence GAP-20261001-13`（在册 M2 承接 · 先回读，语义对口）。
//
// 实现要点（与 py 逐处对齐、✗ 不靠「看起来等价」）：
//
//	· `STEP_RE` 全 ASCII 字符类 ⇒ 无 py `\s`/`\d` 的 Unicode 语义差；Go RE2 的 `^`（无 `(?m)`）在**单行**上
//	  与 py `re.match` 同义。
//	· `text.splitlines()` 与 `open(encoding=).read()` 的 universal newlines 均自实现（`sswPySplitLines` /
//	  `sswReadText`）—— ✗ 不用 `strings.Split(text,"\n")`（会漏 `\r`/`\v`/`\f`/`\u2028` 等 py 行界）。
//	· 路径拼接走自写 `sswPyJoin`（✗ 不用 `filepath.Join`：`..`/绝对后段/空段三态不同）。
//	· argparse 面**含前缀缩写**（`--f`/`--self-` 等唯一前缀收；`--s`/`--self` 歧义报错），逐格与 py 对齐。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const sswProg = "check-step-switch.py"

// sswUsage —— py argparse 的用法串（80 列内单行）。
const sswUsage = "usage: check-step-switch.py [-h] [--file FILE] [--self-test]\n"

// sswHelpText —— py argparse（本件有 argparse）的**逐字**帮助正文（`-h`/`--help` · 对拍 c10/c11 已逐字节证）。
const sswHelpText = sswUsage +
	"\n" +
	"门87：步名↔档位↔严格旗标 表级一致性静态判据\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --file FILE           被扫的表件（默认本仓 scripts/gates/precommit-gates.sh）\n" +
	"  --self-test, --selftest\n" +
	"                        合成夹具自检（三档成对 · 不碰真目标）\n"

// sswLongOpts —— argparse 已注册的**长选项串**（按注册序；用于前缀缩写匹配）。
var sswLongOpts = []string{"--help", "--file", "--self-test", "--selftest"}

// sswStepRE —— py `STEP_RE` 逐字直译。
var sswStepRE = regexp.MustCompile(`^[ \t]*add_step[ \t]+([a-z-]+)[ \t]+"((?:[^"\\]|\\.)*)"[ \t]+(rc|empty|tri|tri-report)[ \t]`)

// sswLooseExit —— py `LOOSE_EXIT`（dict ⇒ 保插入序：用切片对）。
var sswLooseExit = [][2]string{
	{"check-doc-meta.py", "--missing=fail"},
	{"check-wired-scripts.py", "--strict-report"},
	{"check-git-path-outlet.py", "--tier rc"},
}

// sswScopes —— py `SCOPES`（tuple · 只做成员判定）。
var sswScopes = []string{
	"go", "rust", "pub", "tags", "docs", "gates", "tools", "slice", "public", "devdocs", "release",
}

// sswCmdPrefix —— 自检测例里那条「解释器 + 仓内脚本目录」命令前缀。**拆两截写**：
// 避免本件正文出现一条连续可执行的命令原串（自指消解防御写法；拼出的字节与 py 逐字相同）。
const sswCmdPrefix = "python3 " + "scripts/gates/"

// ssw 自检合成夹具 —— 与 py `_GOOD`/`_BAD_FLAG`/`_BAD_NAME`/`_BAD_DOWN` 逐字对齐。
var sswGood = "add_step gates \"门X a（阻断）\" tri \"${REPO_ROOT}\" \"" + sswCmdPrefix + "check-doc-meta.py --missing=fail\"\n" +
	"add_step gates \"门Y b（只报告）\" tri-report \"${REPO_ROOT}\" \"" + sswCmdPrefix + "check-b.py\"\n"

var sswBadFlag = "add_step gates \"门X a（阻断）\" tri \"${REPO_ROOT}\" \"" + sswCmdPrefix + "check-wired-scripts.py\"\n"

var sswBadName = "add_step gates \"门Z c（只报告）\" tri \"${REPO_ROOT}\" \"" + sswCmdPrefix + "check-c.py\"\n"

var sswBadDown = "add_step gates \"门W d（阻断）\" tri-report \"${REPO_ROOT}\" \"" + sswCmdPrefix + "check-d.py\"\n"

// ── 基础件（与 py 口径对齐）──────────────────────────────────────────────────────────

// sswContains —— 字符串切片成员判定（替 py `in tuple`）。
func sswContains(list []string, x string) bool {
	for _, s := range list {
		if s == x {
			return true
		}
	}
	return false
}

// sswPyJoin —— 复刻 posixpath.join（逐段规则直译；✗ 不用 filepath.Join）。
func sswPyJoin(parts ...string) string {
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

// sswIsLineBoundary —— py 行界字符集。
func sswIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// sswPySplitLines —— 复刻 py str.splitlines()（`\r\n` 作一个行界；末尾空行丢弃）。
func sswPySplitLines(s string) []string {
	var out []string
	start, i := 0, 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if sswIsLineBoundary(r) {
			out = append(out, s[start:i])
			if r == '\r' && i+sz < len(s) && s[i+sz] == '\n' {
				i += sz + 1
			} else {
				i += sz
			}
			start = i
			continue
		}
		i += sz
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// sswReadText —— 复刻 py `open(path, encoding="utf-8").read()`：universal newlines（`\r\n` 与裸 `\r` 折成 `\n`）。
func sswReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n"), nil
}

// ── 判据体（与 py 同构）──────────────────────────────────────────────────────────────

// sswStep —— py `parse_steps()` 的一格。
type sswStep struct {
	scope string
	name  string
	mode  string
	line  string
}

// sswParseSteps —— py `parse_steps()`：⇒ [{scope,name,mode,line}]（按文件出现序）。
func sswParseSteps(text string) []sswStep {
	var steps []sswStep
	for _, raw := range sswPySplitLines(text) {
		m := sswStepRE.FindStringSubmatch(raw)
		if m != nil {
			steps = append(steps, sswStep{scope: m[1], name: m[2], mode: m[3], line: raw})
		}
	}
	return steps
}

// sswScan —— py `scan()`：⇒ (steps, violations)。
func sswScan(text string) ([]sswStep, []string) {
	steps := sswParseSteps(text)
	var bad []string
	for _, s := range steps {
		inScope := sswContains(sswScopes, s.scope)
		if s.mode == "tri" {
			for _, le := range sswLooseExit {
				if strings.Contains(s.line, le[0]) && !strings.Contains(s.line, le[1]) {
					bad = append(bad, fmt.Sprintf("甲 缺严格旗标：步「%s」调到有宽松出口的 %s，而命令串里无 `%s` （不带它 ⇒ 该步只拦 rc=2/异常码、拦不住命中 = 只挂了个恒绿的步）", s.name, le[0], le[1]))
				}
			}
		}
		saysReport := strings.Contains(s.name, "只报告")
		if inScope && s.mode == "tri" && saysReport {
			bad = append(bad, fmt.Sprintf("乙 名/档矛盾：步「%s」名写「只报告」却申报 `tri`（阻断）—— 两处对不上", s.name))
		}
		if inScope && s.mode == "tri-report" && !saysReport {
			bad = append(bad, fmt.Sprintf("乙 名/档矛盾：步「%s」申报 `tri-report` 而名里没写「只报告」（防静默降档：只报告档必须自报家门）", s.name))
		}
	}
	return steps, bad
}

// sswRepoRootDefault —— 标准声明 (a)：从 cwd 上溯 ≤6 找含 `scripts/gates` 的根，找不到回落 cwd。
func sswRepoRootDefault() string {
	start, err := os.Getwd()
	if err != nil {
		return "."
	}
	d, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	for i := 0; i < 6; i++ {
		if fi, err := os.Stat(sswPyJoin(d, "scripts", "gates")); err == nil && fi.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// sswDefaultFile —— py `DEFAULT_FILE`。
func sswDefaultFile() string {
	return sswPyJoin(sswRepoRootDefault(), "scripts", "gates", "precommit-gates.sh")
}

// ── 自检（合成夹具 · 三档成对 · 不碰真目标）────────────────────────────────────────────

// sswSelfTest —— py `self_test()`（5 格 ＋ 真目标自吃那一格）。
func sswSelfTest(stdout io.Writer) int {
	fails := 0
	chk := func(label, got, want string) {
		ok := got == want
		if !ok {
			fails++
		}
		mark := "✗"
		if ok {
			mark = "✓"
		}
		fmt.Fprintf(stdout, "  %s %s（期望 %s 实际 %s）\n", mark, label, want, got)
	}

	steps, bad := sswScan(sswGood)
	chk("正控：好表 ⇒ 2 步 · 0 违例", fmt.Sprintf("%d/%d", len(steps), len(bad)), "2/0")

	steps, bad = sswScan(sswBadFlag)
	chk("负控甲：tri 步调到 check-wired-scripts.py 却缺 --strict-report ⇒ 1 违例", fmt.Sprintf("%d", len(bad)), "1")

	steps, bad = sswScan(sswBadName)
	chk("负控乙：名写「只报告」却申报 tri ⇒ 1 违例", fmt.Sprintf("%d", len(bad)), "1")

	steps, bad = sswScan(sswBadDown)
	chk("负控乙·降档：申报 tri-report 却名无「只报告」 ⇒ 1 违例", fmt.Sprintf("%d", len(bad)), "1")

	steps, bad = sswScan("# 只有注释，没有 add_step\n")
	chk("空转：扫不到任何 add_step ⇒ steps=0（调用方判 rc=2）", fmt.Sprintf("%d", len(steps)), "0")

	if fi, err := os.Stat(sswDefaultFile()); err == nil && !fi.IsDir() {
		if text, err := sswReadText(sswDefaultFile()); err == nil {
			if _, bad2 := sswScan(text); len(bad2) == 0 {
				fmt.Fprintf(stdout, "  （真目标自吃：%s）\n", "甲/乙 0 违例 ✓")
			} else {
				fmt.Fprintf(stdout, "  （真目标自吃：%s）\n", fmt.Sprintf("违例 %d 条 ✗", len(bad2)))
			}
		}
	} else {
		fmt.Fprint(stdout, "  （真目标不在盘 ⇒ 跳过自吃那一格）\n")
	}

	verdict := "全过 ✓"
	if fails != 0 {
		verdict = fmt.Sprintf("**%d 格不过** ✗", fails)
	}
	fmt.Fprintf(stdout, "── 自检：%s\n", verdict)
	if fails == 0 {
		return 0
	}
	return 2
}

// ── argparse 面（含前缀缩写）────────────────────────────────────────────────────────

// sswArgErr —— 用法错（py argparse）：stderr 印 usage ＋ `prog: error: …`，退码 2。
func sswArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, sswUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", sswProg, msg)
	return 2
}

// stepSwitchGateMain —— 入口（= py `main()`）。
func stepSwitchGateMain(stdout, stderr io.Writer, argv []string) int {
	fileVal := ""
	fileSet := false
	selfTest := false
	var extras []string

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "-h" {
			fmt.Fprint(stdout, sswHelpText)
			return 0
		}
		if a == "--" || a == "-" || !strings.HasPrefix(a, "-") {
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
		for _, o := range sswLongOpts {
			if strings.HasPrefix(o, name) {
				matches = append(matches, o)
			}
		}
		if len(matches) == 0 {
			extras = append(extras, a)
			continue
		}
		if len(matches) > 1 {
			return sswArgErr(stderr, "ambiguous option: "+name+" could match "+strings.Join(matches, ", "))
		}
		switch matches[0] {
		case "--help":
			if hasExplicit {
				return sswArgErr(stderr, "argument -h/--help: ignored explicit argument '"+explicit+"'")
			}
			fmt.Fprint(stdout, sswHelpText)
			return 0
		case "--self-test", "--selftest":
			if hasExplicit {
				return sswArgErr(stderr, "argument --self-test/--selftest: ignored explicit argument '"+explicit+"'")
			}
			selfTest = true
		case "--file":
			if hasExplicit {
				fileVal, fileSet = explicit, true
			} else if i+1 < len(argv) && (argv[i+1] == "-" || !strings.HasPrefix(argv[i+1], "-")) {
				i++
				fileVal, fileSet = argv[i], true
			} else {
				return sswArgErr(stderr, "argument --file: expected one argument")
			}
		}
	}

	if len(extras) > 0 {
		return sswArgErr(stderr, "unrecognized arguments: "+strings.Join(extras, " "))
	}
	if !fileSet {
		fileVal = sswDefaultFile()
	}
	if selfTest {
		return sswSelfTest(stdout)
	}

	if fi, err := os.Stat(fileVal); err != nil || fi.IsDir() {
		fmt.Fprintf(stderr, "✗ 表件不在盘（%s）⇒ 判不了 ⇒ 不给结论（rc=2）\n", fileVal)
		return 2
	}
	text, err := sswReadText(fileVal)
	if err != nil {
		fmt.Fprintf(stderr, "✗ 表件读不了（%s）⇒ 判不了 ⇒ 不给结论（rc=2）\n", fileVal)
		return 2
	}
	steps, bad := sswScan(text)
	if len(steps) == 0 {
		fmt.Fprintf(stderr, "✗ 表件里一个 `add_step` 都读不到（%s）⇒ 空转 = 假覆盖 ⇒ 不给结论（rc=2）\n", fileVal)
		return 2
	}
	tri, rep := 0, 0
	for _, s := range steps {
		if sswContains(sswScopes, s.scope) && s.mode == "tri" {
			tri++
		}
		if sswContains(sswScopes, s.scope) && s.mode == "tri-report" {
			rep++
		}
	}
	nall := len(steps)
	if len(bad) > 0 {
		fmt.Fprintf(stdout, "✗ 步名↔档位↔严格旗标：违例 %d 条（扫到 add_step %d 条 · 默认集 tri %d · tri-report %d）\n", len(bad), nall, tri, rep)
		for _, b := range bad {
			fmt.Fprintf(stdout, "   ✗ %s\n", b)
		}
		return 1
	}
	fmt.Fprintf(stdout, "✓ 步名↔档位↔严格旗标：表自洽（add_step %d 条 · 默认集 tri %d · tri-report %d · 甲/乙 违例 0）\n", nall, tri, rep)
	return 0
}

// 已对拍面（逐字节比 rc/stdout/stderr）· 见同批 `check_step_switch_test.go` 与回执。
// 未覆盖面：
//  ① py argparse 在**非 3.9** 解释器下印 `options:` 而非 `optional arguments:`（本件钉 py 3.9 正文）；
//     该差是**解释器版本差**、非实现差（门跑环境 PATH 的 python3 恒为 /usr/bin/python3=3.9.6）。
//  ② py 无最外层兜底（裸 `sys.exit(main(...))`）⇒ 件读不了时裸回溯 rc=1（C 类）；
//     本件按新口径 fail-closed rc=2（该格只保 rc 之外的正文不同：py 是回溯、本件是具名文案）。
//  ③ argparse 的极角落形态（`-h=x` 短选项带值等）未逐一复刻，只复刻了被扫常见档。
