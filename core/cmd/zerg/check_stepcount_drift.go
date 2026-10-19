// check_stepcount_drift.go —— 判据件 `scripts/gates/check-stepcount-drift.py`（201 行）的 **Go 真搬**
// （v2.5.14 · M2 · 搬运面 搬运粒17 · 入口 `stepcountDriftGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**。
//
// 选它的理由（真未搬清单里**纯静态只读 · 无子进程/联网/TTY** 的一档 · 行数适中）：
//
//	✓ 判据是**单件文本扫描** —— 只读**一个仓内件**（默认 `<cwd>/scripts/gates/precommit-gates.sh`，
//	  可 `--script` 换），解析 `FAST_STEPS=( … )` 数组长度并与「规范计数」对拍（只报告）。
//	  现读 import 面：`argparse` / `json` / `os` / `re` / `sys`；`shutil` / `tempfile` **只在 `--self-test` 内**。
//	  **零 `subprocess` / `socket` / `urllib` / `time` / `datetime` / `random`** · 默认档**零写面**（只有读 open）。
//	  退码仅 0/1/2 三档齐 · `git status` 现读不含它（无人正在改）。
//	✗ 不选同族其余未搬件（现读 import/危险形态面后的排除理由）：`check-public-marks-tracked-parity.py`
//	  （subprocess 起 git）· `check-stale-frozen.py` / `check-selfcount.py` / `check-partial-run-acceptance.py`
//	  （subprocess+mkdtemp）· `check-verify-cmd-real.py` / `check-write-face-gate.py`（mkdtemp+写面）·
//	  `repo-diffstat-check.py`（与在仓 `family_repo_diffstat.go` 同套 ⇒ 走退场，不搬）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 add_step 表）；
//	· **不**删 `scripts/gates/check-stepcount-drift.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `stepcountDriftGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件 / `gate_inproc_table.go`。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func stepcountDriftGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 无漂移 / 1 有漂移（规范计数与数组不符）/ 2 不给结论（缺件 · 数组解析不出 · 用法错）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见件尾「已对拍面 / 未覆盖面」）
//
// ── 两行标准声明（本单必写）────────────────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py `--script` 缺省值 = **字面相对串** `scripts/gates/precommit-gates.sh`
//	    （**cwd 相对** · py **不用 `__file__`** · 也**无任何 `os.path.join`**）。Go 无 `__file__` 语境，
//	    但本件同样按 **cwd 相对**解该串（`os.Stat` / `os.ReadFile` 直接吃该字面）⇒ 两者**逐字同构**，
//	    无「脚本位置 ≠ cwd」的推导差。★ 本件**无路径拼接面** ⇒ 不涉 `pyJoin` 家族语义差（不适用，非省略）。
//	(b) **自指检查要真判**：本件**在**自己的扫描面内吗？—— **否**（自指为假）。本门判据面恒为**被扫的那**一个
//	    文本件（默认 `scripts/gates/precommit-gates.sh` 或 `--script` 指定件）；本件落
//	    `core/cmd/zerg/check_stepcount_drift.go`，不在被扫件内 ⇒ 本门读不到本件自己。依据（两条）：
//	    ① py `main()`/`judge()` 只有一处读面 `open(args.script)`，Go 侧同构（`scdReadScript`）；
//	    ② 判据点名的「规范计数」只匹配**被扫行文本**，不扫本件源码。★ 另按**自指第四/五形态**核他门扫描面：
//	    本件在 `core/cmd/zerg/*.go` ⇒ 落 `check-error-kinds` 的 R3 面（只认**错误构造调用的首参字面**）与
//	    `check-hardcoded-private-paths`（SCAN_DIRS 含 `core/`）—— 本件**不写**该调用式、**不写**任何私有
//	    绝对路径首段字面（此处按**描述式**写）⇒ 命中 0/0。★ 但本件正文确有 `FAST_STEPS` 与反引号等**判据要匹配的形态**
//	    的字面量 —— 那是**被扫件**的字面（`precommit-gates.sh`），本件只把它当**输入文本**比；本件**自身**不在
//	    任何「扫 `precommit-gates.sh`」的门的扫描面内 ⇒ 无害。自证见回执（同根串影子仓两态 sha16 同）。
//
// ⑤ `--evidence GAP-20261001-13`（在册 M2 承接 · 先回读，语义对口）。
//
// 实现要点（与 py 逐处对齐、✗ 不靠「看起来等价」）：
//
//	· 三条 py 正则的 `\d`/`\s` 是 **Unicode 语义** ⇒ 逐字展开为 `\p{Nd}` 与显式 Unicode 空白类（`scdW`），
//	  ✗ 不用 Go RE2 默认的 ASCII `\d`/`\s`。
//	· `open(encoding=).read()` 的 universal newlines（`\r\n` 与裸 `\r` 折成 `\n`）自实现（`scdReadScript`）；
//	  py 后段是 `.split("\n")`（**不是** `splitlines()`）⇒ Go 用 `strings.Split(s,"\n")` 与之同构。
//	· `str.strip()` 剥的是 **py Unicode 空白**（含 `\x1c-\x1f`/`\x85`/`\xa0`）⇒ 自写 `scdPyStrip`，✗ 不用 `strings.TrimSpace`。
//	· argparse 面**含前缀缩写**（`--js`/`--h`/`--self` 等唯一前缀收；`--s` 歧义报错），逐格与 py 对齐。
//	· `--json` 面按 py `json.dumps(..., ensure_ascii=False)` 逐字节构造（`", "`/`": "` 分隔 · 非 ASCII 字面 · 控制符转义）。
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

const scdProg = "check-stepcount-drift.py"

// scdUsage —— py argparse 的用法串。
const scdUsage = "usage: check-stepcount-drift.py [-h] [--script SCRIPT] [--json] [--self-test]\n"

// scdHelpText —— py argparse（本件有 argparse）的**逐字**帮助正文（`-h`/`--help` · 对拍 c03/c21 已逐字节证）。
const scdHelpText = scdUsage +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help       show this help message and exit\n" +
	"  --script SCRIPT\n" +
	"  --json\n" +
	"  --self-test\n"

// scdLongOpts —— argparse 已注册的**长选项串**（按注册序；用于前缀缩写匹配）。
var scdLongOpts = []string{"--help", "--script", "--json", "--self-test"}

// scdW —— py `\s`（str 语义）的显式展开：`[\t\n\v\f\r \x1c-\x1f\x85\xa0]` ∪ Unicode 空白。
const scdW = `[\t\n\x{0b}\x{0c}\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

// py 正则逐字直译（`\d`→`\p{Nd}`；`\s`→`scdW`）。
var (
	scdNumStep = regexp.MustCompile(`(\p{Nd}{1,4})` + scdW + `*步`)
	scdArrow   = regexp.MustCompile(`→` + scdW + `*(\p{Nd}{1,4})`)
	scdCanon   = regexp.MustCompile("FAST_STEPS`?" + scdW + `*(?:名单)?(?:长度|条数|步数)?` + scdW + "*`?" + scdW + `*[=:：]?` + scdW + `*(\p{Nd}{1,4})` + scdW + `*步`)
	scdFastArr = regexp.MustCompile(`^` + scdW + `*FAST_STEPS=\($`)
)

// scdIsPySpace —— py `str.isspace()` 的字符集（`str.strip()` 用）。
func scdIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', '\x1c', '\x1d', '\x1e', '\x1f', '\x85', '\xa0',
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// scdPyStrip —— 复刻 py `str.strip()`（剥 py Unicode 空白；✗ 不用 strings.TrimSpace）。
func scdPyStrip(s string) string {
	start, end := 0, len(s)
	for start < end {
		r, sz := utf8.DecodeRuneInString(s[start:])
		if !scdIsPySpace(r) {
			break
		}
		start += sz
	}
	for end > start {
		r, sz := utf8.DecodeLastRuneInString(s[:end])
		if !scdIsPySpace(r) {
			break
		}
		end -= sz
	}
	return s[start:end]
}

// scdReadScript —— 复刻 py `open(path, encoding="utf-8").read().split("\n")`：
// universal newlines（`\r\n` 与裸 `\r` 折成 `\n`）后再按 `\n` 切（**不是** `splitlines()`）。
func scdReadScript(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n"), nil
}

// scdHit —— py `canon` 的一格（(line, num, text)）。
type scdHit struct {
	line int
	num  string
	text string
}

// scdResult —— py `judge()` 的返回 dict 的 Go 形态。
type scdResult struct {
	rc      int
	nArr    int
	canon   []scdHit
	drift   []scdHit
	generic int
	reason  string
}

// scdFastArrayLen —— py `fast_array_len()`：FAST_STEPS=( … ) 的元素条数（以行首 `"…"` 计）。
// 取不到（无数组行 / 未闭合）⇒ (0,false) ⇒ 调用方判 rc=2。
func scdFastArrayLen(lines []string) (int, bool) {
	start := -1
	for i, ln := range lines {
		if scdFastArr.MatchString(ln) {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, false
	}
	n := 0
	for _, ln := range lines[start+1:] {
		s := scdPyStrip(ln)
		if s == ")" {
			return n, true
		}
		if strings.HasPrefix(s, "\"") {
			n++
		}
	}
	return 0, false
}

// scdJudge —— py `judge()`：⇒ scdResult（rc: 0/1/2）。
func scdJudge(path string) scdResult {
	d := scdResult{rc: 2}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		d.reason = "script-absent"
		return d
	}
	lines, err := scdReadScript(path)
	if err != nil {
		// ★ py 此处 `read_script` 在 try 之外 ⇒ 读不了会**未捕回溯 rc=1**（C 类）。
		//   本件按新口径 fail-closed rc=2 并具名（该格列未覆盖面）。
		d.reason = "script-absent"
		return d
	}
	nArr, ok := scdFastArrayLen(lines)
	if !ok {
		d.reason = "array-unparseable"
		return d
	}
	d.nArr = nArr
	var canon []scdHit
	gen := 0
	for i, ln := range lines {
		if strings.Contains(ln, "FAST_STEPS=(") || scdPyStrip(ln) == ")" {
			continue
		}
		for _, m := range scdCanon.FindAllStringSubmatch(ln, -1) {
			canon = append(canon, scdHit{line: i + 1, num: m[1], text: scdPyStrip(ln)})
		}
		gen += len(scdNumStep.FindAllStringIndex(ln, -1)) + len(scdArrow.FindAllStringIndex(ln, -1))
	}
	d.canon = canon
	d.generic = gen
	target := strconv.Itoa(nArr)
	for _, c := range canon {
		if c.num != target {
			d.drift = append(d.drift, c)
		}
	}
	if len(d.drift) > 0 {
		d.rc = 1
	} else {
		d.rc = 0
	}
	return d
}

// scdJSONStr —— 复刻 py `json.dumps(..., ensure_ascii=False)` 的**字符串**编码器。
func scdJSONStr(s string) string {
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
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// scdHitJSON —— py `[{"line":…, "num":…, "text":…} …]` 的一格。
func scdHitJSON(h scdHit) string {
	return fmt.Sprintf(`{"line": %d, "num": %s, "text": %s}`, h.line, scdJSONStr(h.num), scdJSONStr(h.text))
}

// ── 自检（合成门件夹具 · 落仓外临时树 · 成对正负控）────────────────────────────────────
// ★ 合成夹具与 py `S_POS`/`S_NEG`/`S_NOISE`/`S_MIXED`/`S_NOARR`/`ARR3` 逐字对齐。
const scdArr3 = "FAST_STEPS=(\n  \"a\"\n  \"b\"\n  \"c\"\n)\n"

var (
	scdSPos   = "#!/bin/sh\n# 快速档：`FAST_STEPS` 名单长度 = 3 步（登记面）\n" + scdArr3
	scdSNeg   = "#!/bin/sh\n# 快速档：`FAST_STEPS` 名单长度 = 4 步（登记面 · 写错）\n" + scdArr3
	scdSNoise = "#!/bin/sh\n" +
		"# 现读 38 步 · 该跑记录 29 步 · 全表 86 → 94 步 · 门㉝ 实测 0.074s\n" + scdArr3
	scdSMixed = "#!/bin/sh\n" +
		"# 现读 38 步 · 全表 86 → 94 步 · 门㉝ 0.074s\n" +
		"# `FAST_STEPS` 名单长度 = 5 步（真声明 · 写错）\n" + scdArr3
	scdSNoArr = "#!/bin/sh\n# 现读 3 步\n"
)

// scdSelfTest —— py `run_self_test()`（断言 12 条 · 只印失败 · 全过 rc=0 / 有失败 rc=1）。
func scdSelfTest(stdout io.Writer) int {
	fmt.Fprint(stdout, "步数常量扫描 · 自检（合成门件夹具 · 临时树 · 成对正负控）\n")
	total, bad := 0, 0
	assertEq := func(name string, got, want string) {
		total++
		if got != want {
			bad++
			fmt.Fprintf(stdout, "  ✗ 断言失败：%s —— 得到 %s，期望 %s\n", name, got, want)
		}
	}
	itoa := func(n int) string { return strconv.Itoa(n) }

	tmp, err := os.MkdirTemp("", "scdrift-selftest-")
	if err != nil {
		fmt.Fprint(stdout, "── 自检结论：断言 0 条 · 失败 1 条 ──\n")
		return 1
	}
	defer os.RemoveAll(tmp) // ★ 钉在「建点成功」支（err==nil）⇒ 安全删点
	write := func(name, body string) string {
		p := tmp + "/" + name
		_ = os.WriteFile(p, []byte(body), 0o644)
		return p
	}

	// ① 正控：规范计数与数组一致 ⇒ rc=0
	d := scdJudge(write("pos.sh", scdSPos))
	assertEq("① 正控：规范计数 3 步 = 数组 3 ⇒ rc=0", itoa(d.rc), "0")
	assertEq("① 正控：恰 1 处规范计数", itoa(len(d.canon)), "1")
	// ② 负控：规范计数写错 ⇒ rc=1 且点名那一行
	d = scdJudge(write("neg.sh", scdSNeg))
	assertEq("② 负控：规范计数 4 步 ≠ 数组 3 ⇒ rc=1", itoa(d.rc), "1")
	assertEq("② 负控：漂移恰 1 条且点名行 2", scdLines(d.drift), "[2]")
	// ③ 降噪负控：满是泛化数字、无规范计数 ⇒ rc=0
	d = scdJudge(write("noise.sh", scdSNoise))
	assertEq("③ 降噪负控：泛化数字一堆但无规范计数 ⇒ rc=0（不判）", itoa(d.rc), "0")
	if d.generic > 0 {
		assertEq("③ 降噪负控：泛化计数>0（确有噪声但被降为只报告）", "True", "True")
	} else {
		assertEq("③ 降噪负控：泛化计数>0（确有噪声但被降为只报告）", "False", "True")
	}
	assertEq("③ 降噪负控：规范计数命中 0", itoa(len(d.canon)), "0")
	// ④ 混合负控：噪声 + 一处写错的规范计数 ⇒ rc=1 且**只**挑规范那一条
	d = scdJudge(write("mixed.sh", scdSMixed))
	assertEq("④ 混合负控：噪声中挑出规范计数错 ⇒ rc=1", itoa(d.rc), "1")
	assertEq("④ 混合负控：漂移恰 1 条（噪声未被判）", itoa(len(d.drift)), "1")
	assertEq("④ 混合负控：点名的是规范计数行 3", scdLines(d.drift), "[3]")
	// ⑤ 缺件负控：门件不在盘 ⇒ rc=2
	assertEq("⑤ 缺件负控：门件不在盘 ⇒ rc=2", itoa(scdJudge(tmp+"/没有.sh").rc), "2")
	// ⑥ 数组解析不出负控：无 FAST_STEPS=( ⇒ rc=2
	d = scdJudge(write("noarr.sh", scdSNoArr))
	assertEq("⑥ 无数组负控：FAST_STEPS 解析不出 ⇒ rc=2", itoa(d.rc), "2")

	fmt.Fprintf(stdout, "── 自检结论：断言 %d 条 · 失败 %d 条 ──\n", total, bad)
	if bad == 0 {
		return 0
	}
	return 1
}

// scdLines —— 自检里再现 py `[x[0] for x in drift]` 的 repr（`[2]` / `[3]`）。
func scdLines(hits []scdHit) string {
	parts := make([]string, 0, len(hits))
	for _, h := range hits {
		parts = append(parts, strconv.Itoa(h.line))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ── argparse 面（含前缀缩写）────────────────────────────────────────────────────────

// scdArgErr —— 用法错（py argparse）：stderr 印 usage ＋ `prog: error: …`，退码 2。
func scdArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, scdUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", scdProg, msg)
	return 2
}

// stepcountDriftGateMain —— 入口（= py `main()`）。
func stepcountDriftGateMain(stdout, stderr io.Writer, argv []string) int {
	scriptVal := pathPrecommitGates
	scriptSet := false
	jsonOut := false
	selfTest := false
	var extras []string

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			// py argparse：`--` 与其后全部原样进 extras（无位置参数 ⇒ 报「unrecognized arguments」）。
			extras = append(extras, argv[i:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			extras = append(extras, a)
			continue
		}
		if a == "-h" {
			fmt.Fprint(stdout, scdHelpText)
			return 0
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
		for _, o := range scdLongOpts {
			if strings.HasPrefix(o, name) {
				matches = append(matches, o)
			}
		}
		if len(matches) == 0 {
			extras = append(extras, a)
			continue
		}
		if len(matches) > 1 {
			return scdArgErr(stderr, "ambiguous option: "+name+" could match "+strings.Join(matches, ", "))
		}
		switch matches[0] {
		case "--help":
			if hasExplicit {
				return scdArgErr(stderr, "argument -h/--help: ignored explicit argument '"+explicit+"'")
			}
			fmt.Fprint(stdout, scdHelpText)
			return 0
		case "--self-test":
			if hasExplicit {
				return scdArgErr(stderr, "argument --self-test: ignored explicit argument '"+explicit+"'")
			}
			selfTest = true
		case "--json":
			if hasExplicit {
				return scdArgErr(stderr, "argument --json: ignored explicit argument '"+explicit+"'")
			}
			jsonOut = true
		case "--script":
			if hasExplicit {
				scriptVal, scriptSet = explicit, true
			} else if i+1 < len(argv) && (argv[i+1] == "-" || !strings.HasPrefix(argv[i+1], "-")) {
				i++
				scriptVal, scriptSet = argv[i], true
			} else {
				return scdArgErr(stderr, "argument --script: expected one argument")
			}
		}
	}

	if len(extras) > 0 {
		return scdArgErr(stderr, "unrecognized arguments: "+strings.Join(extras, " "))
	}
	if selfTest {
		return scdSelfTest(stdout)
	}
	_ = scriptSet

	d := scdJudge(scriptVal)
	if d.reason == "script-absent" {
		fmt.Fprintf(stderr, "读不到门禁件：%s ⇒ 不给结论（退码 2）\n", scriptVal)
		return 2
	}
	if d.reason == "array-unparseable" {
		fmt.Fprint(stderr, "FAST_STEPS 数组解析不出 ⇒ 不给结论（退码 2）\n")
		return 2
	}

	if jsonOut {
		var hits, dh []string
		for _, h := range d.canon {
			hits = append(hits, scdHitJSON(h))
		}
		for _, h := range d.drift {
			dh = append(dh, scdHitJSON(h))
		}
		driftB := "false"
		if len(d.drift) > 0 {
			driftB = "true"
		}
		fmt.Fprintf(stdout, `{"script": %s, "fast_array_len": %d, "canonical_hits": [%s], "drift_vs_array": [%s], "generic_count_suppressed": %d, "drift": %s}`+"\n",
			scdJSONStr(scriptVal), d.nArr, strings.Join(hits, ", "), strings.Join(dh, ", "), d.generic, driftB)
	} else {
		fmt.Fprintf(stdout, "步数常量扫描（只读 · 规范计数白名单）· 件=%s\n", scriptVal)
		fmt.Fprintf(stdout, "  FAST_STEPS 数组现读长度 = %d\n", d.nArr)
		fmt.Fprintf(stdout, "  规范计数命中 %d 处（只认「FAST_STEPS 名单长度 = N 步」形态）；与数组**不一致** %d 处：\n",
			len(d.canon), len(d.drift))
		for _, h := range d.drift {
			fmt.Fprintf(stdout, "    %s:%d  数=%s  ← %s\n", scriptVal, h.line, h.num, h.text)
		}
		if len(d.drift) == 0 {
			fmt.Fprint(stdout, "    （无 —— 规范计数与数组一致）\n")
		}
		fmt.Fprintf(stdout, "  泛化计数（`N 步` / `→ N`）%d 条 —— 白名单外 · 只报告不判（降噪）\n", d.generic)
	}
	if len(d.drift) > 0 {
		return 1
	}
	return 0
}

// 已对拍面（逐字节比 rc/stdout/stderr · 16 格）· 见同批 `check_stepcount_drift_test.go` 与回执。
// 未覆盖面：
//  ① py argparse 在**非 3.9** 解释器下印 `options:` 而非 `optional arguments:`（本件钉 py 3.9 正文 ·
//     门跑环境 PATH 的 python3 为 /usr/bin/python3=3.9.6）；该差是**解释器版本差**、非实现差。
//  ② py 无最外层兜底（裸 `sys.exit(main())`）**且 `read_script` 落在 try 之外** ⇒ 件存在但**读不了**
//     （权限/编码非法）时 py 裸回溯 rc=1（C 类）；本件按新口径 fail-closed rc=2 并具名 ⇒ 该格只保
//     rc 之外的正文不同（py 是回溯、本件是具名文案）。★ 该格未实测（夹具不便造不可读件），如实列此。
//  ③ argparse 极角落形态（`-hx` 短选项聚簇 · 负号数值当值等）未逐一复刻，只复刻被扫常见档。
//  ④ py `open(encoding="utf-8")` 遇**非法 UTF-8** 抛 UnicodeDecodeError（未捕 ⇒ rc=1）；本件按字节读、
//     不校验 ⇒ 该档行为不同（真仓件为合法 UTF-8 ⇒ 未触发）。
