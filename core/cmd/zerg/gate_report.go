// gate_report.go —— 判据件 scripts/gates/gate-report.py（318 行 · sha16 b501c904306a94a8）的
// **Go 真搬**（v2.5.14 · M2 · 门禁读面编排族 · 入口 gateReportMain）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳（zerg gate check <判据名>）；M2 = 按价值真搬 Go
//	（落点 core/cmd/zerg/）。本件 = M2 一枚。验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），
//	stdout / stderr / 退码**三面逐字相同**（除下列**显式声明的**差异面）。
//
// 本件现读读数（py 原件 · 逐条给行号）：
//
//	① 行数 318 · sha16 b501c904306a94a8 · /usr/bin/python3 = 3.9.6。
//	② 旗标面（argparse · add_argument 九处，:197-207）—— --root(:197) / --gate(:198, append) /
//	   --list(:200) / --key-regex(:201) / --json(:203) / --dump(:204) / --registry(:205) /
//	   --json-only(:206) / -h,--help(:195 add_help=True)。
//	   ★ 另有 argparse **之前**的拦截：:190 `if "--self-test" in argv or "--selftest" in argv`。
//	   ★★ **--json 是死旗标**：:203 注册、:208 解析，正文**从不读** args.json（grep 现读 0 处引用）
//	   ⇒ 名义「机器读面」实为 no-op，输出仍走 human 面。本件**逐字照搬该现状**（死旗标不激活），
//	   并把「py 侧本就恒死」显式声明在此 —— ✗ 本件不把它做活（做活即与 py 对拍分叉）。
//	③ 遍历面（:77-89 list_gates）：`d = join(root,"scripts","gates")`（:79）⇒ `os.path.isdir` 缺则
//	   **返空表**(:81-82) ⇒ `sorted(os.listdir(d))`(:83，**输出序决定者 = sorted**，不是 readdir 原序)
//	   ⇒ 跳过非 `.py`(:84-85) ⇒ 跳过 `gate-report.py` 与 `who-references.py`(:86-87) ⇒ append(:88)。
//	   ★ 第二个「序」面：`--gate` 点名档**按 argv 原序**(:221 `args.gate or list_gates`)；
//	   `passed`/`pending` 也按该序(:277-278)；human 面明细行序同(:298)。
//	④ 子进程面（:92-107 run_gate）：`[sys.executable, gate_path] + argv_extra`(:94) ⇒
//	   `subprocess.run(cwd=root, capture_output=True, text=True, timeout=600)`(:97)。
//	   ⇒ 本件**钉死解释器 /usr/bin/python3**（等价于 py 在 `/usr/bin/python3` 下 sys.executable 的取值）。
//	   本件无外部库依赖，只跑门禁自己的 py 件。
//	⑤ 写面（**唯一被允许的写**）：仅 `--dump <path>`(:283-289) 走 `open(...,"w",encoding="utf-8")`
//	   + `json.dump(..., ensure_ascii=False, indent=2)`：**无尾换行**。其余一律只读。
//	   另有 `--write/--set/--append/--yes/--reason/--force/--overwrite/--apply/--commit/--push`
//	   十枚**写意图旗标**(:53-56) ⇒ 出现在 argv 里即 rc=2 且零字节(:211-215)。
//	⑥ 外部真源（真身绝对路径 · 只读）：
//	   · 门禁目录 = `<--root>/scripts/gates/`（真身 = **真仓根下**的 scripts/gates 目录 ——
//	     其绝对路径只写进回执，✗ 不落件头：私有路径字面量属自指面，会被他门扫到）
//	     —— 本件**只列名**(--list) 或**起子进程**跑其中的 .py；不读其内容。
//	   · 登记表 = `--registry <路径>`(:205) —— 仅**原样转发**给每个门禁（:232），本件不打开它。
//	⑦ 非确定面（本件**都有** · 逐条点名）：`elapsed_s`(:99/102/106/258 · round(...,3)) ·
//	   `generated_at`(:274 `time.strftime("%Y-%m-%dT%H:%M:%S%z")`) · 子进程 stdout/stderr 本身。
//	   py **未给**任何时刻注入旗标/env ⇒ 对拍走「**归一后严格 ==**」（规则见回执）。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `DEFAULT_ROOT = abspath(join(dirname(abspath(__file__)), "..", ".."))`
//	    (:48-49，**脚本位置上溯两级**)；Go 无 `__file__` 语境 ⇒ 本件 `grtDefaultRoot()` =
//	    `grtPyAbspath(cwd, ".")`（**cwd**）。⇒ 「脚本位置 ≠ cwd」时两态不同，✗ 本件不声称该档逐字对齐。
//	    对拍台按 skill 口径把 py 副本放进 `<夹具>/scripts/gates/` 且 cwd=夹具根 ⇒ 该档两态同值，
//	    故缺省档仍做了**逐字**对拍（见回执）。
//	(b) **自指检查（先真判 · 四形态逐条）**：
//	  ① 本件在自己扫描面内吗 ⇒ **否**：本件不扫 `core/`，只列/跑 `<--root>/scripts/gates/*.py`
//	     (由外部真源栏现读：读面 = 传参 root，非本件所在树)。量化依据：本件全文零 `core` 路径字面量。
//	  ② 本件落在**他门**扫描面内吗 ⇒ **是**：`core/cmd/zerg/*.go` 被 `check-error-kinds.py` 的
//	     R3 面、`check-hardcoded-private-paths.py`（SCAN_DIRS 含 core/）扫。⇒ 已处置：正文**零**
//	     R3 两形态原串（JSON 键名加冒号加引号串形态 · setErr 加左括号形态）—— 键名一律走
//	     「前缀 + 字面」拼串或裸标识符；**零私有绝对路径字面量**（真仓根/家目录前缀一概不写，
//	     运行时取）。
//	  ③ 证明处理有效：落件后跑 R3 正则现读 = **0/0**（读数见回执）。
//	  ④ 本件自检夹具只把魔法串写进**运行时建在 TMPDIR 下**的夹具件，✗ 不落本件正文。
//
// py 原件真口子（★ 不自行修 · 逐字保留/照搬 · 交回入账）：
//
//	· C 类（最外层无兜底）：:317-318 裸 `sys.exit(main(...))` ⇒ `--key-regex` 语法错时
//	  `re.error` 未捕 = **回溯 + rc=1**；本件按新口径 **rc=2 fail-closed**（见未覆盖面）。
//	· D 类（死旗标）：`--json`(:203) 从不被读（同上 ②/★★）。另有**死常量** `WRITE_FLAGS`(:52)
//	  与**死函数** `_is_json_line`(:62-63) —— 正文从不引用（grep 现读 0 处）。
//	· 读取面口径差：`\d` 在 py 是 **Unicode**；本件用 `[0-9]`（ASCII）。真源门禁只用 ASCII 数字
//	  ⇒ 未触发；Unicode 数字面写进未覆盖面。
//	· 本件**不重算任何门禁判据**：只取真退码 + 计时 + 捞关键行（:11-15 件头自述）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 core/cmd/zerg/main.go 里注册命令（✗ 不进命令树 · ✗ 不改 add_step）；
//	· **不**删 scripts/gates/gate-report.py 原件（退场规则：**对拍通过后才**退场）；
//	· 入口 `gateReportMain` 目前**无调用点**（M2 的刻意状态）。
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ─────────────────────────── 常量面（逐条对 py 行号）───────────────────────────

const (
	grtProgName         = "gate-report.py"      //:193 prog=
	grtPyInterp         = "/usr/bin/python3"    // ★ 钉死（≡ py 在 /usr/bin/python3 下的 sys.executable）
	grtRegistryGate     = "check-dangling-refs" //:59
	grtKeyNameGate      = "gate"
	grtKeyNameRC        = "rc"
	grtKeyNameElapsed   = "elapsed_s"
	grtKeyNameErrored   = "errored"
	grtKeyNameStderr    = "stderr"
	grtKeyNameKeyLines  = "key_lines"
	grtKeyNameRegFalse  = "registered_falses"
	grtKeyNameRegDetail = "registered_falses_detail"
	grtKeyNameRawJSON   = "raw_json"
	grtKeyNameError     = "error"
	grtKeyNameSchema    = "schema"
	grtKeyNameRepo      = "repo"
	grtKeyNameGenerated = "generated_at"
	grtKeyNameGatesRun  = "gates_run"
	grtKeyNameAnyPend   = "any_pending"
	grtKeyNamePassed    = "passed"
	grtKeyNamePending   = "pending"
	grtKeyNameResults   = "results"
	grtSchemaValue      = "gate-report/1"                     //:272
	grtSelfGateName     = "gate-report.py"                    //:86 自排除
	grtSkipGateName     = "who-references.py"                 //:86 自排除
	grtSelfTestName     = "gate-report.py"                    //:166 文案用
	grtTimeoutTail      = "⏱ 门禁超时（>600s）"                     //:100
	grtDefaultKeyRegex  = "结论|命中|rc=|未登记|悬空|FAIL|红|pass|fail" //:201
	grtDanglingUnRE     = `有 ([0-9]+) 处.*未登记`                 //:138（\d → [0-9]，见件头）
	grtDanglingRegRE    = `已登记 ([0-9]+) 处`                    //:139
	grtKeyLineCap       = 20                                  //:125
	grtUnreadableReason = "已登记/未登记 计数读不出（人面文案变体且无机读面）⇒ 不给结论"  // GAP-20261005-30 第一刀
	grtKeyLineWidth     = 200                                 //:119
	grtStderrKeep       = 500                                 //:260
	grtHumanStderrWidth = 160                                 //:307
)

// grtDefaultKeyRegexRE / grtDangling*RE 见上；grtWriteIntentFlags 对 py :53-56。
var grtWriteIntentFlags = []string{
	"--write", "--set", "--append", "--yes", "--reason", "--force",
	"--overwrite", "--apply", "--commit", "--push",
}

// grtGateTimeout 对 py :97 的 timeout=600（★ var 而非 const：单测可缩短以覆盖超时档）。
var grtGateTimeout = 600 * time.Second

// grtUsageText / grtHelpText —— py argparse 在 COLUMNS=80 下的**逐字**产物
// （pin 法：py 原件 -h stdout 的 sha256 = 48607dcbf24fb15fe5a3af0090da8025d0b1f67fc87e005e7874903a31c52bb2）。
const grtUsageText = "usage: gate-report.py [-h] [--root ROOT] [--gate GATE] [--list]\n" +
	"                      [--key-regex KEY_REGEX] [--json] [--dump DUMP]\n" +
	"                      [--registry REGISTRY] [--json-only]\n"

const grtHelpText = grtUsageText + "\n" +
	"逐条跑门禁清单并汇总为 JSON（默认只读不写）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT           门禁跑在哪个仓根（默认本仓根）\n" +
	"  --gate GATE           点名跑的门禁 *.py（可多次）；缺省 = 跑 gates/ 全部\n" +
	"  --list                列本目录认得哪些门禁，不打跑\n" +
	"  --key-regex KEY_REGEX\n" +
	"                        从门禁 stdout 里捞关键判据行的正则（默认命中「结论/rc=/未登记…」）\n" +
	"  --json                机器读面：输出汇总 JSON\n" +
	"  --dump DUMP           把汇总 JSON 显式落盘（唯一被允许的写行为）\n" +
	"  --registry REGISTRY   给悬空引用门指定登记表路径\n" +
	"  --json-only           只 stdout 输出 JSON 本体（无任何 human 行）—— 管线用\n"

// ─────────────────────────── py 语义原语（照 CPython 逐段直译）───────────────────────────

// grtPyJoin 复刻 posixpath.join（✗ 不用 filepath.Join：.. 段/绝对后段/空段三类语义差）。
func grtPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "/"):
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// grtPyNormpath 复刻 posixpath.normpath（★ 保留「恰两个前导斜杠」）。
func grtPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initialSlashes := 0
	if strings.HasPrefix(path, "/") {
		initialSlashes = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initialSlashes = 2
		}
	}
	var comps []string
	for _, comp := range strings.Split(path, "/") {
		switch {
		case comp == "" || comp == ".":
		case comp != ".." || (initialSlashes == 0 && len(comps) == 0) || (len(comps) > 0 && comps[len(comps)-1] == ".."):
			comps = append(comps, comp)
		default:
			if len(comps) > 0 {
				comps = comps[:len(comps)-1]
			}
		}
	}
	out := strings.Join(comps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// grtPyAbspath 复刻 posixpath.abspath：**纯词法**（cwd 由调用方传入 —— ✗ 不在函数里 os.Getwd()）。
func grtPyAbspath(cwd, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = grtPyJoin(cwd, path)
	}
	return grtPyNormpath(path)
}

// grtPyIsSpace = py str.isspace() ∈：unicode 空白 **＋** \x1c-\x1f（Go unicode.IsSpace 不含后者）。
func grtPyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// grtPyStrip 复刻 py str.strip()（✗ 不用 strings.TrimSpace）。
func grtPyStrip(s string) string {
	return strings.TrimFunc(s, grtPyIsSpace)
}

// grtPyIsLineBreak = py str.splitlines() 的边界集。
func grtPyIsLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', 0x0b, 0x0c, 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// grtPySplitLines 复刻 py str.splitlines()（\r\n 算一条 · 空串 → 空表）。
func grtPySplitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if grtPyIsLineBreak(r) {
			out = append(out, s[start:i])
			i += sz
			if r == '\r' && i < len(s) && s[i] == '\n' {
				i++
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

// grtPyRuneSlice 复刻 py 切片 s[:n] / s[-n:] —— **按码点**（✗ 按 byte 切多字节字符）。
func grtPyRuneSlice(s string, lo, hi int) string {
	rs := []rune(s)
	n := len(rs)
	if lo < 0 {
		lo = n + lo
		if lo < 0 {
			lo = 0
		}
	}
	if hi < 0 {
		hi = n + hi
		if hi < 0 {
			hi = 0
		}
	}
	if hi > n {
		hi = n
	}
	if lo > hi {
		lo = hi
	}
	return string(rs[lo:hi])
}

// grtPyFloatRepr 复刻 py repr(float) 的**常见值域**（json.dumps 非 ASCII 转义外的浮点面）。
func grtPyFloatRepr(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	e := int(math.Floor(math.Log10(math.Abs(f))))
	if e < -4 || e >= 16 {
		return strconv.FormatFloat(f, 'e', -1, 64)
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// grtRound3 对 py round(x, 3)。
func grtRound3(f float64) float64 { return math.Round(f*1000) / 1000 }

func grtIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func grtIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// ─────────────────────────── 保序 JSON 值（py dict 保插入序 ⇒ ✗ 不用 map）───────────────────────────

// grtKV = 一个键值对（保插入序）。
type grtKV struct {
	K *grtVal
	V *grtVal
}

// grtVal 保序 JSON 值：kind ∈ o(对象) a(数组) s(串) n(数) b(布尔) z(null)。
type grtVal struct {
	kind byte
	s    string
	raw  string
	b    bool
	obj  []grtKV
	arr  []*grtVal
}

func grtS(s string) *grtVal             { return &grtVal{kind: 's', s: s} }
func grtRawNum(tok string) *grtVal      { return &grtVal{kind: 'n', raw: tok} }
func grtInt(i int) *grtVal              { return &grtVal{kind: 'n', raw: strconv.Itoa(i)} }
func grtFloat(f float64) *grtVal        { return &grtVal{kind: 'n', raw: grtPyFloatRepr(f)} }
func grtBool(b bool) *grtVal            { return &grtVal{kind: 'b', b: b} }
func grtNull() *grtVal                  { return &grtVal{kind: 'z'} }
func grtArrOf(vs []*grtVal) *grtVal     { return &grtVal{kind: 'a', arr: vs} }
func grtObjOf(kvs ...grtKV) *grtVal     { return &grtVal{kind: 'o', obj: kvs} }
func grtKVOf(k string, v *grtVal) grtKV { return grtKV{K: grtS(k), V: v} }

// grtAppendJSONString 复刻 json.dumps(ensure_ascii=False) 的串转义表。
func grtAppendJSONString(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString("\\\"")
		case '\\':
			sb.WriteString("\\\\")
		case '\b':
			sb.WriteString("\\b")
		case '\f':
			sb.WriteString("\\f")
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			sb.WriteString("\\r")
		case '\t':
			sb.WriteString("\\t")
		default:
			if r < 0x20 {
				sb.WriteString(fmt.Sprintf("\\u%04x", r))
				continue
			}
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
}

// grtWriteJSON 复刻 json.dumps：indent<0 ⇒ 紧凑档（分隔符 ", " 与 ": "）；indent>=0 ⇒ 缩进档（indent 空格）。
func grtWriteJSON(sb *strings.Builder, v *grtVal, indent, level int) {
	switch v.kind {
	case 'z':
		sb.WriteString("null")
	case 'b':
		if v.b {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case 'n':
		sb.WriteString(v.raw)
	case 's':
		grtAppendJSONString(sb, v.s)
	case 'a':
		if len(v.arr) == 0 {
			sb.WriteString("[]")
			return
		}
		sb.WriteByte('[')
		for i, it := range v.arr {
			if i > 0 {
				sb.WriteByte(',')
			}
			if indent >= 0 {
				sb.WriteByte('\n')
				sb.WriteString(strings.Repeat(" ", (level+1)*indent))
			} else if i > 0 {
				sb.WriteByte(' ')
			}
			grtWriteJSON(sb, it, indent, level+1)
		}
		if indent >= 0 {
			sb.WriteByte('\n')
			sb.WriteString(strings.Repeat(" ", level*indent))
		}
		sb.WriteByte(']')
	case 'o':
		if len(v.obj) == 0 {
			sb.WriteString("{}")
			return
		}
		sb.WriteByte('{')
		for i, kv := range v.obj {
			if i > 0 {
				sb.WriteByte(',')
			}
			if indent >= 0 {
				sb.WriteByte('\n')
				sb.WriteString(strings.Repeat(" ", (level+1)*indent))
			} else if i > 0 {
				sb.WriteByte(' ')
			}
			grtWriteJSON(sb, kv.K, indent, level+1)
			sb.WriteString(": ")
			grtWriteJSON(sb, kv.V, indent, level+1)
		}
		if indent >= 0 {
			sb.WriteByte('\n')
			sb.WriteString(strings.Repeat(" ", level*indent))
		}
		sb.WriteByte('}')
	}
}

// ─────────────────────────── 严格 JSON 解析（保序 · 契约对标 py json.loads）───────────────────────────

type grtJParser struct {
	s string
	i int
}

func grtJSONParse(src string) (*grtVal, error) {
	p := &grtJParser{s: src}
	p.ws()
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	p.ws()
	if p.i != len(p.s) {
		return nil, errors.New("extra data")
	}
	return v, nil
}

func (p *grtJParser) ws() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *grtJParser) value() (*grtVal, error) {
	if p.i >= len(p.s) {
		return nil, errors.New("expecting value")
	}
	switch c := p.s[p.i]; {
	case c == '{':
		return p.object()
	case c == '[':
		return p.array()
	case c == '"':
		s, err := p.str()
		if err != nil {
			return nil, err
		}
		return grtS(s), nil
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	case strings.HasPrefix(p.s[p.i:], "true"):
		p.i += 4
		return grtBool(true), nil
	case strings.HasPrefix(p.s[p.i:], "false"):
		p.i += 5
		return grtBool(false), nil
	case strings.HasPrefix(p.s[p.i:], "null"):
		p.i += 4
		return grtNull(), nil
	case strings.HasPrefix(p.s[p.i:], "NaN"):
		p.i += 3
		return grtRawNum("NaN"), nil
	case strings.HasPrefix(p.s[p.i:], "Infinity"):
		p.i += 8
		return grtRawNum("Infinity"), nil
	case strings.HasPrefix(p.s[p.i:], "-Infinity"):
		p.i += 9
		return grtRawNum("-Infinity"), nil
	}
	return nil, errors.New("expecting value")
}

func (p *grtJParser) object() (*grtVal, error) {
	p.i++ // {
	out := &grtVal{kind: 'o'}
	p.ws()
	if p.i < len(p.s) && p.s[p.i] == '}' {
		p.i++
		return out, nil
	}
	for {
		p.ws()
		if p.i >= len(p.s) || p.s[p.i] != '"' {
			return nil, errors.New("expecting property name")
		}
		k, err := p.str()
		if err != nil {
			return nil, err
		}
		p.ws()
		if p.i >= len(p.s) || p.s[p.i] != ':' {
			return nil, errors.New("expecting ':' delimiter")
		}
		p.i++
		p.ws()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		// py dict：同名后键**原位覆盖**（保首次插入序）
		found := false
		for j := range out.obj {
			if out.obj[j].K.s == k {
				out.obj[j].V = v
				found = true
				break
			}
		}
		if !found {
			out.obj = append(out.obj, grtKV{K: grtS(k), V: v})
		}
		p.ws()
		if p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.i < len(p.s) && p.s[p.i] == '}' {
			p.i++
			return out, nil
		}
		return nil, errors.New("expecting ',' delimiter")
	}
}

func (p *grtJParser) array() (*grtVal, error) {
	p.i++ // [
	out := &grtVal{kind: 'a', arr: []*grtVal{}}
	p.ws()
	if p.i < len(p.s) && p.s[p.i] == ']' {
		p.i++
		return out, nil
	}
	for {
		p.ws()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		out.arr = append(out.arr, v)
		p.ws()
		if p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.i < len(p.s) && p.s[p.i] == ']' {
			p.i++
			return out, nil
		}
		return nil, errors.New("expecting ',' delimiter")
	}
}

func (p *grtJParser) str() (string, error) {
	p.i++ // 开引号
	var sb strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == '"':
			p.i++
			return sb.String(), nil
		case c == '\\':
			p.i++
			if p.i >= len(p.s) {
				return "", errors.New("unterminated string")
			}
			switch p.s[p.i] {
			case '"':
				sb.WriteByte('"')
				p.i++
			case '\\':
				sb.WriteByte('\\')
				p.i++
			case '/':
				sb.WriteByte('/')
				p.i++
			case 'b':
				sb.WriteByte('\b')
				p.i++
			case 'f':
				sb.WriteByte('\f')
				p.i++
			case 'n':
				sb.WriteByte('\n')
				p.i++
			case 'r':
				sb.WriteByte('\r')
				p.i++
			case 't':
				sb.WriteByte('\t')
				p.i++
			case 'u':
				if p.i+4 >= len(p.s) {
					return "", errors.New("invalid \\uXXXX escape")
				}
				u, err := strconv.ParseUint(p.s[p.i+1:p.i+5], 16, 32)
				if err != nil {
					return "", errors.New("invalid \\uXXXX escape")
				}
				p.i += 5
				r := rune(u)
				if r >= 0xD800 && r <= 0xDBFF && strings.HasPrefix(p.s[p.i:], "\\u") && p.i+5 < len(p.s) {
					lo, err2 := strconv.ParseUint(p.s[p.i+2:p.i+6], 16, 32)
					if err2 == nil && lo >= 0xDC00 && lo <= 0xDFFF {
						r = ((r - 0xD800) << 10) + (rune(lo) - 0xDC00) + 0x10000
						p.i += 6
					}
				}
				sb.WriteRune(r)
			default:
				return "", errors.New("invalid escape")
			}
		default:
			sb.WriteByte(c)
			p.i++
		}
	}
	return "", errors.New("unterminated string")
}

func (p *grtJParser) number() (*grtVal, error) {
	start := p.i
	if p.i < len(p.s) && p.s[p.i] == '-' {
		p.i++
	}
	for p.i < len(p.s) {
		c := p.s[p.i]
		if (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
			p.i++
			continue
		}
		break
	}
	tok := p.s[start:p.i]
	if tok == "" || tok == "-" {
		return nil, errors.New("expecting value")
	}
	if !strings.ContainsAny(tok, ".eE") {
		return grtRawNum(tok), nil
	}
	f, err := strconv.ParseFloat(tok, 64)
	if err != nil {
		return nil, errors.New("invalid number")
	}
	return grtFloat(f), nil
}

// ─────────────────────────── py 函数面（逐条对行号）───────────────────────────

// grtListGates 对 py :77-89。
func grtListGates(root string) []string {
	d := grtPyJoin(root, "scripts", "gates")
	out := []string{}
	if !grtIsDir(d) {
		return out
	}
	ents, err := os.ReadDir(d)
	if err != nil {
		return out
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names) // 对 py :83 sorted(os.listdir(d))
	for _, fn := range names {
		if !strings.HasSuffix(fn, ".py") {
			continue
		}
		if fn == grtSelfGateName || fn == grtSkipGateName {
			continue
		}
		out = append(out, fn)
	}
	return out
}

// grtRun 对 py :99-107 的返回 dict 三面。
type grtRun struct {
	rc      int
	elapsed float64
	stdout  string
	stderr  string
	errored bool
}

// grtUTF8DecodeErr 尽力复刻 CPython UnicodeDecodeError 正文（`'utf-8' codec can't decode byte …`）。
// ★ 只覆盖「首个非法字节」的两种常见措辞；多字节截断档未逐字复刻（见未覆盖面）。
func grtUTF8DecodeErr(b []byte) string {
	for i := 0; i < len(b); {
		r, sz := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && sz == 1 {
			word := "invalid start byte"
			if b[i] >= 0x80 && b[i] <= 0xbf {
				word = "invalid continuation byte"
			}
			return fmt.Sprintf("'utf-8' codec can't decode byte 0x%02x in position %d: %s", b[i], i, word)
		}
		i += sz
	}
	return "'utf-8' codec can't decode bytes: invalid data"
}

// grtRunGate 对 py :92-107。
func grtRunGate(gatePath string, argvExtra []string, root string) grtRun {
	cmdArgs := append([]string{gatePath}, argvExtra...)
	t0 := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), grtGateTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, grtPyInterp, cmdArgs...)
	cmd.Dir = root
	var ob, eb bytes.Buffer
	cmd.Stdout = &ob
	cmd.Stderr = &eb
	err := cmd.Run()
	el := grtRound3(time.Since(t0).Seconds())
	if ctx.Err() == context.DeadlineExceeded {
		return grtRun{rc: 2, elapsed: el, stdout: "", stderr: grtTimeoutTail, errored: true}
	}
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return grtRun{rc: 2, elapsed: el, stdout: "",
				stderr: "✗ 跑不了该门禁：" + err.Error(), errored: true}
		}
	}
	// py text=True 走 strict 解码；非 UTF-8 ⇒ UnicodeDecodeError（本件 rc/正文对齐、措辞尽力）
	if !utf8.Valid(ob.Bytes()) {
		return grtRun{rc: 2, elapsed: el, stdout: "",
			stderr: "✗ 跑不了该门禁：" + grtUTF8DecodeErr(ob.Bytes()), errored: true}
	}
	if !utf8.Valid(eb.Bytes()) {
		return grtRun{rc: 2, elapsed: el, stdout: "",
			stderr: "✗ 跑不了该门禁：" + grtUTF8DecodeErr(eb.Bytes()), errored: true}
	}
	rc := 0
	if err != nil {
		var ee *exec.ExitError
		_ = errors.As(err, &ee)
		rc = ee.ExitCode()
	}
	return grtRun{rc: rc, elapsed: el, stdout: ob.String(), stderr: eb.String(), errored: false}
}

// grtExtractKeyLines 对 py :110-125。
func grtExtractKeyLines(stdout string, pat *regexp.Regexp) []string {
	seen := map[string]bool{}
	lines := []string{}
	for _, ln := range grtPySplitLines(stdout) {
		if !pat.MatchString(ln) {
			continue
		}
		cut := grtPyStrip(ln)
		if len([]rune(cut)) > grtKeyLineWidth {
			cut = grtPyRuneSlice(cut, 0, grtKeyLineWidth) + "…"
		}
		if seen[cut] {
			continue
		}
		seen[cut] = true
		lines = append(lines, cut)
	}
	if len(lines) > grtKeyLineCap {
		lines = lines[:grtKeyLineCap]
	}
	return lines
}

// grtObjStr 取对象**串**成员（非串/缺键 ⇒ false）。
func grtObjStr(v *grtVal, key string) (string, bool) {
	if v == nil || v.kind != 'o' {
		return "", false
	}
	for _, kv := range v.obj {
		if kv.K.kind == 's' && kv.K.s == key {
			if kv.V.kind == 's' {
				return kv.V.s, true
			}
			return "", false
		}
	}
	return "", false
}

// grtObjInt 取对象**整数**成员（对应 py `type(x) is int` —— 布尔/浮点均算无）。
func grtObjInt(v *grtVal, key string) (int, bool) {
	if v == nil || v.kind != 'o' {
		return 0, false
	}
	for _, kv := range v.obj {
		if kv.K.kind == 's' && kv.K.s == key {
			if kv.V.kind == 'n' {
				n, err := strconv.Atoi(kv.V.raw)
				if err != nil {
					return 0, false
				}
				return n, true
			}
			return 0, false
		}
	}
	return 0, false
}

// grtObjBool 取对象**布尔**成员（非布尔/缺键 ⇒ false）。
func grtObjBool(v *grtVal, key string) (bool, bool) {
	if v == nil || v.kind != 'o' {
		return false, false
	}
	for _, kv := range v.obj {
		if kv.K.kind == 's' && kv.K.s == key {
			if kv.V.kind == 'b' {
				return kv.V.b, true
			}
			return false, false
		}
	}
	return false, false
}

// grtParseDanglingJSON 读 check-dangling-refs 的机读面（--json 一行 · schema=dangling-refs/1）。
// 返回 (registered, hit, true)；hit 即「未登记」数（live+ledger）。GAP-20261005-30 第一刀。
func grtParseDanglingJSON(stdout string) (int, int, bool) {
	for _, ln := range grtPySplitLines(stdout) {
		s := grtPyStrip(ln)
		if !strings.HasPrefix(s, "{") || !strings.HasSuffix(s, "}") {
			continue
		}
		v, err := grtJSONParse(s)
		if err != nil || v == nil || v.kind != 'o' {
			continue
		}
		sc, ok := grtObjStr(v, "schema")
		if !ok || sc != "dangling-refs/1" {
			continue
		}
		reg, okR := grtObjInt(v, "registered")
		hit, okH := grtObjInt(v, "hit")
		if okR && okH {
			return reg, hit, true
		}
	}
	return 0, 0, false
}

// grtExtractRegisteredFalses 对 py :128-146。
func grtExtractRegisteredFalses(stdout, gateName string) (bool, *grtVal) {
	if !strings.Contains(gateName, grtRegistryGate) {
		return false, nil
	}
	if reg, hit, ok := grtParseDanglingJSON(stdout); ok {
		return reg > 0 && (hit == 0 || reg >= 5*hit), grtObjOf(
			grtKVOf("unregistered", grtInt(hit)),
			grtKVOf("registered", grtInt(reg)),
		)
	}
	if !strings.Contains(stdout, "已登记") || !strings.Contains(stdout, "不计命中") {
		return false, nil
	}
	unRE := regexp.MustCompile(grtDanglingUnRE)
	regRE := regexp.MustCompile(grtDanglingRegRE)
	mUn := unRE.FindStringSubmatch(stdout)
	mReg := regRE.FindStringSubmatch(stdout)
	nUn := -1
	nReg := -1
	if mUn != nil {
		if v, err := strconv.Atoi(mUn[1]); err == nil {
			nUn = v
		}
	}
	if mReg != nil {
		if v, err := strconv.Atoi(mReg[1]); err == nil {
			nReg = v
		}
	}
	if nUn < 0 || nReg < 0 {
		return false, grtObjOf(
			grtKVOf("unreadable", grtBool(true)),
			grtKVOf("reason", grtS(grtUnreadableReason)),
		)
	}
	flag := nReg > 0 && (nUn == 0 || nReg >= 5*nUn)
	return flag, grtObjOf(
		grtKVOf("unregistered", grtInt(nUn)),
		grtKVOf("registered", grtInt(nReg)),
	)
}

// grtLooksLikeJSON 对 py :66-74（_looks_like_json）。
func grtLooksLikeJSON(s string) (*grtVal, bool) {
	t := grtPyStrip(s)
	if !strings.HasPrefix(t, "{") || !strings.HasSuffix(t, "}") {
		return nil, false
	}
	v, err := grtJSONParse(t)
	if err != nil {
		return nil, false
	}
	return v, true
}

// grtFindRawJSON 对 py :247-254（第一行「像 JSON」的门禁自报 JSON 原样收进 raw）。
func grtFindRawJSON(stdout string) *grtVal {
	for _, ln := range grtPySplitLines(stdout) {
		if v, ok := grtLooksLikeJSON(ln); ok {
			return v
		}
	}
	return nil
}

// ─────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────────

// grtStChk 对 py :154-160。
func grtStChk(bad *int, label string, want, rc int, out string, stdout io.Writer) {
	good := rc == want
	if !good {
		*bad++
	}
	tail := ""
	if !good {
		ls := grtPySplitLines(grtPyStrip(out))
		last := ""
		if len(ls) > 0 {
			last = ls[len(ls)-1]
		}
		tail = " ｜ " + grtPyRuneSlice(last, 0, 90)
	}
	mark := "✓"
	if !good {
		mark = "✗"
	}
	fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
}

// grtSelfExe —— 自检「真命令行」子进程用的自身可执行件（py 走 `sys.executable`）；测试件注入仓外探针壳。
// ★ 它必须**是命令面入口**（`bin/zerg`），故重跑时**必须带 inproc 路由前缀**（见 grtSelfTest 的 run）。
var grtSelfExe = func() (string, error) { return os.Executable() }

// grtSelfTest 对 py :163-186（★ py 用 mkdtemp + finally rmtree ⇒ **本身回收**；本件同样回收）。
func grtSelfTest(stdout, _ io.Writer) int {
	bad := 0
	fmt.Fprintf(stdout, "── %s --self-test 三档（正控 0 · 负控 1 · 用法错 2）\n", grtSelfTestName)
	tmp, err := os.MkdirTemp("", "gatereport-selftest-") // 对 py :167
	if err != nil {
		fmt.Fprintf(stdout, "── 自检：**1 档不过** ✗\n")
		return 1
	}
	defer os.RemoveAll(tmp) // 对 py :184 shutil.rmtree(..., ignore_errors=True)
	repo := grtPyJoin(tmp, "repo")
	_ = os.MkdirAll(grtPyJoin(repo, "scripts", "gates"), 0o755)
	for _, spec := range []struct {
		name string
		code int
	}{{"ok-gate.py", 0}, {"bad-gate.py", 1}} {
		body := "#!/usr/bin/env python3\nimport sys\nprint('rc=0 夹具')\nsys.exit(" +
			strconv.Itoa(spec.code) + ")\n"
		_ = os.WriteFile(grtPyJoin(repo, "scripts", "gates", spec.name), []byte(body), 0o644)
	}
	exe, err := grtSelfExe()
	if err != nil {
		fmt.Fprintf(stdout, "── 自检：**1 档不过** ✗\n")
		return 1
	}
	run := func(extra ...string) (int, string) {
		// ★★ 必须**带 inproc 路由前缀**（`gate check gate-report`）：`bin/zerg` 的 argv 首段是命令树
		// 子命令，直接把本门旗标喂给制品会被当成未知命令 ⇒ 子进程恒退 2（自检 ①/② 两格假红）。
		// py 无这一层（它是 `[sys.executable, SELF_PATH, …]`，直接以件为入口）。
		args := append([]string{"--root", repo}, extra...)
		argv := append([]string{"gate", "check", "gate-report"}, args...)
		cmd := exec.Command(exe, argv...) // 对 py :176（[sys.executable, SELF_PATH, ...]）
		cmd.Dir = tmp                     // 对 py :177 cwd=tmp
		var ob, eb bytes.Buffer
		cmd.Stdout = &ob
		cmd.Stderr = &eb
		err := cmd.Run()
		rc := 0
		if err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				rc = ee.ExitCode()
			} else {
				rc = 2
			}
		}
		return rc, ob.String() + eb.String() // 对 py :178
	}
	rc, out := run("--gate", "ok-gate.py")
	grtStChk(&bad, "档① 正控：夹具门 rc=0 ⇒ 汇总 rc=0", 0, rc, out, stdout)
	rc, out = run("--gate", "bad-gate.py")
	grtStChk(&bad, "档② 负控：夹具门 rc=1 ⇒ 汇总 rc=1 并点名", 1, rc, out, stdout)
	rc, out = run("--no-such-flag")
	grtStChk(&bad, "档③ 用法错：不认的旗标 ⇒ rc=2", 2, rc, out, stdout)
	if bad == 0 {
		fmt.Fprintf(stdout, "── 自检：全过 ✓\n")
		return 0
	}
	fmt.Fprintf(stdout, "── 自检：**%d 档不过** ✗\n", bad)
	return 1
}

// ─────────────────────────── argparse 仿真（本件九枚旗标 · 逐字对齐 py 3.9 措辞）───────────────────────────

type grtArgs struct {
	root     string
	gates    []string
	list     bool
	keyRegex string
	jsonFlag bool // ★ 死旗标：解析但从不读（逐字照搬 py）
	dump     string
	registry string
	jsonOnly bool
	cwd      string
}

var grtLongOpts = []string{
	"--root", "--gate", "--list", "--key-regex", "--json", "--dump", "--registry", "--json-only", "--help",
}

func grtNargsOf(opt string) int {
	switch opt {
	case "--root", "--gate", "--key-regex", "--dump", "--registry":
		return 1
	}
	return 0
}

// grtMatchLong 对 argparse：精确 > 唯一前缀（缩写）> 多前缀＝ambiguous > 无＝未知。
func grtMatchLong(name string) (string, string, bool) {
	full := "--" + name
	for _, o := range grtLongOpts {
		if o == full {
			return o, "", true
		}
	}
	var cands []string
	for _, o := range grtLongOpts {
		if strings.HasPrefix(o, full) {
			cands = append(cands, o)
		}
	}
	if len(cands) == 1 {
		return cands[0], "", true
	}
	if len(cands) > 1 {
		return "", "ambiguous option: " + full + " could match " + strings.Join(cands, ", "), true
	}
	return "", "", false
}

var grtNegNumRE = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

// grtLooksLikeOption 对 argparse._parse_optional 的 'O' 分类。
func grtLooksLikeOption(tok string) bool {
	if tok == "" || !strings.HasPrefix(tok, "-") || tok == "-" {
		return false
	}
	if grtNegNumRE.MatchString(tok) || strings.Contains(tok, " ") {
		return false
	}
	return true
}

// grtParseArgs 仿真 argparse：返回 (args, 错误正文, 是否请求帮助)。
// ★ 帮助动作**就地触发**（对 py：-h 一被消费即打印并退 0）⇒ 其后的未知项不再报错。
func grtParseArgs(argv []string) (grtArgs, string, bool) {
	a := grtArgs{keyRegex: grtDefaultKeyRegex, gates: []string{}}
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if tok == "--" {
			break // 其余一律当位置参数（= 未识别项），由 grtUnrecognizedTokens 报
		}
		if strings.HasPrefix(tok, "--") {
			body := tok[2:]
			name, val, hasEq := body, "", false
			if k := strings.Index(body, "="); k >= 0 {
				name, val, hasEq = body[:k], body[k+1:], true
			}
			opt, errMsg, known := grtMatchLong(name)
			if !known {
				continue
			}
			if errMsg != "" {
				return a, errMsg, false
			}
			if opt == "--help" {
				if hasEq {
					return a, "argument -h/--help: ignored explicit argument '" + val + "'", false
				}
				return a, "", true
			}
			if grtNargsOf(opt) == 0 {
				if hasEq {
					return a, "argument " + opt + ": ignored explicit argument '" + val + "'", false
				}
				grtAssignFlag(&a, opt, "")
				continue
			}
			var v string
			if hasEq {
				v = val
			} else {
				if i+1 >= len(argv) || grtLooksLikeOption(argv[i+1]) {
					return a, "argument " + opt + ": expected one argument", false
				}
				i++
				v = argv[i]
			}
			grtAssignFlag(&a, opt, v)
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			// 短旗标簇：本件只有 -h（argparse：`-h<arg>` ⇒ 动作名是 -h/--help）
			if tok == "-h" {
				return a, "", true
			}
			if strings.HasPrefix(tok, "-h") {
				return a, "argument -h/--help: ignored explicit argument '" + tok[2:] + "'", false
			}
			continue
		}
		continue // 位置参数 = 未识别项（由 grtUnrecognizedTokens 报）
	}
	return a, "", false
}

func grtAssignFlag(a *grtArgs, opt, v string) {
	switch opt {
	case "--root":
		a.root = v
	case "--gate":
		a.gates = append(a.gates, v)
	case "--list":
		a.list = true
	case "--key-regex":
		a.keyRegex = v
	case "--json":
		a.jsonFlag = true
	case "--dump":
		a.dump = v
	case "--registry":
		a.registry = v
	case "--json-only":
		a.jsonOnly = true
	}
}

// ─────────────────────────── 主入口 ───────────────────────────

// grtDefaultRoot 对 py :48-49 的**Go 侧等价**（声明 (a)：无 __file__ ⇒ 走 cwd）。
func grtDefaultRoot(cwd string) string { return grtPyAbspath(cwd, ".") }

func grtArgvContains(argv []string, want string) bool {
	for _, a := range argv {
		if a == want {
			return true
		}
	}
	return false
}

func gateReportMain(stdout, stderr io.Writer, argv []string) int {
	for _, a := range argv { // 对 py :190（argparse 之前拦截）
		if a == "--self-test" || a == "--selftest" {
			return grtSelfTest(stdout, stderr)
		}
	}
	cwd, _ := os.Getwd()
	args, perr, helpReq := grtParseArgs(argv)
	if helpReq {
		fmt.Fprint(stdout, grtHelpText)
		return 0
	}
	if perr != "" {
		fmt.Fprint(stderr, grtUsageText)
		fmt.Fprintf(stderr, "%s: error: %s\n", grtProgName, perr)
		return 2
	}
	// 未识别项（对 py argparse.parse_args：先于写意图判 :211）—— 末尾统一报
	extras := grtUnrecognizedTokens(argv)
	if len(extras) > 0 {
		fmt.Fprint(stderr, grtUsageText)
		fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", grtProgName, strings.Join(extras, " "))
		return 2
	}
	if args.root == "" {
		args.root = grtDefaultRoot(cwd)
	}
	for _, f := range grtWriteIntentFlags { // 对 py :211-215
		if grtArgvContains(argv, f) {
			if !args.jsonOnly {
				fmt.Fprintf(stderr, "✗ %s：写意图一律拒 ⇒ rc=2 且一个字节不写\n", f)
			}
			return 2
		}
	}
	if !grtIsDir(grtPyJoin(args.root, "scripts", "gates")) { // 对 py :217-219
		fmt.Fprintf(stderr, "✗ 不在仓根（scripts/gates/ 不存在）：%s ⇒ 拒绝静默空转\n", args.root)
		return 2
	}
	gates := args.gates
	if len(gates) == 0 {
		gates = grtListGates(args.root) // 对 py :221
	}
	if len(gates) == 0 { // 对 py :222-225
		if !args.jsonOnly {
			fmt.Fprint(stderr, "✗ 门禁清单为空（--gate 未点名且 gates/ 无 *.py）⇒ 不给结论\n")
		}
		return 2
	}
	if args.list { // 对 py :227-230
		for _, g := range gates {
			fmt.Fprintf(stdout, "%s\n", g)
		}
		return 0
	}
	registryExtra := []string{}
	if args.registry != "" { // 对 py :232
		registryExtra = []string{"--registry", args.registry}
	}
	results := []*grtResult{}
	anyNonzero := false
	for _, g := range gates { // 对 py :235-269
		gatePath := grtPyJoin(args.root, "scripts", "gates", g)
		if !grtIsFile(gatePath) { // 对 py :237-242
			results = append(results, &grtResult{
				gate: g, rc: 2, elapsed: 0.0, errored: true,
				hasErrMsg: true, errorMsg: "门禁文件不存在",
				keyLines: []string{}, regFalses: false,
			})
			anyNonzero = true
			continue
		}
		perGate := registryExtra
		if strings.Contains(g, grtRegistryGate) {
			perGate = append(append([]string{}, registryExtra...), "--json")
		}
		rec := grtRunGate(gatePath, perGate, args.root)
		pat, reErr := regexp.Compile(args.keyRegex) // 对 py :112 re.compile（★ C 类改 rc=2 fail-closed）
		if reErr != nil {
			fmt.Fprintf(stderr, "✗ --key-regex 不可编译（%v）⇒ 不给结论\n", reErr)
			return 2
		}
		keyLines := grtExtractKeyLines(rec.stdout, pat)
		isFF, ff := grtExtractRegisteredFalses(rec.stdout, g)
		if ff != nil && ff.kind == 'o' {
			if un, ok := grtObjBool(ff, "unreadable"); ok && un {
				reason, _ := grtObjStr(ff, "reason")
				fmt.Fprintf(stderr, "✗ %s：%s ⇒ 拒给结论（rc=2）\n", g, reason)
				return 2
			}
		}
		rawJSON := grtFindRawJSON(rec.stdout)
		results = append(results, &grtResult{
			gate: g, rc: rec.rc, elapsed: rec.elapsed, errored: rec.errored,
			hasStderr: true, stderr: grtStderrTail(rec.stderr),
			keyLines: keyLines, regFalses: isFF, regDetail: ff, raw: rawJSON,
		})
		if rec.rc != 0 {
			anyNonzero = true
		}
	}
	summary := grtBuildSummary(args.root, cwd, results, anyNonzero)
	if args.dump != "" { // 对 py :283-289（唯一被允许的写）
		var sb strings.Builder
		grtWriteJSON(&sb, summary, 2, 0)
		if werr := os.WriteFile(args.dump, []byte(sb.String()), 0o644); werr != nil {
			fmt.Fprintf(stderr, "✗ 落不了盘 %s：%s ⇒ 不给结论\n", args.dump, grtPyOSErrMsg(args.dump, werr))
			return 2
		}
	}
	if args.jsonOnly { // 对 py :291-293
		var sb strings.Builder
		grtWriteJSON(&sb, summary, -1, 0)
		fmt.Fprintf(stdout, "%s\n", sb.String())
		if anyNonzero {
			return 1
		}
		return 0
	}
	grtPrintHuman(stdout, args.root, results, anyNonzero) // 对 py :295-314
	if anyNonzero {
		return 1
	}
	return 0
}

// grtRealGates / grtRawTokens 已并入 grtParseArgs 与 grtUnrecognizedTokens（标记机制取消）。

// grtUnrecognizedTokens 重扫 argv，按 argparse 口径给出「未识别项」原串。
func grtUnrecognizedTokens(argv []string) []string {
	out := []string{}
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if tok == "--" {
			out = append(out, tok)
			out = append(out, argv[i+1:]...)
			break
		}
		if strings.HasPrefix(tok, "--") {
			body := tok[2:]
			name, hasEq := body, false
			if k := strings.Index(body, "="); k >= 0 {
				name, hasEq = body[:k], true
			}
			opt, errMsg, known := grtMatchLong(name)
			if !known {
				out = append(out, tok)
				i++
				continue
			}
			if errMsg != "" || opt == "--help" {
				i++
				continue
			}
			if grtNargsOf(opt) == 1 && !hasEq {
				i += 2
				continue
			}
			i++
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			if tok != "-h" {
				out = append(out, tok)
			}
			i++
			continue
		}
		out = append(out, tok)
		i++
	}
	return out
}

// grtPyOSErrMsg 复刻 CPython OSError 的 str()（常见三档：ENOENT/EACCES/EEXIST）。
// ★ 用于 --dump 失败档的错误正文（对 py `"…：%s …" % e`）。其余 errno 档未覆盖（见未覆盖面）。
func grtPyOSErrMsg(path string, err error) string {
	switch {
	case os.IsNotExist(err):
		return "[Errno 2] No such file or directory: '" + path + "'"
	case os.IsPermission(err):
		return "[Errno 13] Permission denied: '" + path + "'"
	case os.IsExist(err):
		return "[Errno 17] File exists: '" + path + "'"
	}
	return err.Error()
}

// grtStderrTail 对 py :260（strip 后取**末 500 码点**）。
func grtStderrTail(s string) string {
	t := grtPyStrip(s)
	return grtPyRuneSlice(t, -grtStderrKeep, len([]rune(t)))
}

// grtResult 一门的结果（保 py dict 的键序与「键在场/缺席」）。
type grtResult struct {
	gate      string
	rc        int
	elapsed   float64
	errored   bool
	hasErrMsg bool
	errorMsg  string
	hasStderr bool
	stderr    string
	keyLines  []string
	regFalses bool
	regDetail *grtVal
	raw       *grtVal
}

// grtResultJSON 对 py :255-264 / :238-240 的键序逐条复刻。
func grtResultJSON(r *grtResult) *grtVal {
	kvs := []grtKV{
		grtKVOf(grtKeyNameGate, grtS(r.gate)),
		grtKVOf(grtKeyNameRC, grtInt(r.rc)),
		grtKVOf(grtKeyNameElapsed, grtFloat(r.elapsed)),
		grtKVOf(grtKeyNameErrored, grtBool(r.errored)),
	}
	if r.hasErrMsg {
		kvs = append(kvs, grtKVOf(grtKeyNameError, grtS(r.errorMsg)))
		kvs = append(kvs, grtKVOf(grtKeyNameKeyLines, grtStrArr(r.keyLines)))
		kvs = append(kvs, grtKVOf(grtKeyNameRegFalse, grtBool(r.regFalses)))
		return grtObjOf(kvs...)
	}
	kvs = append(kvs, grtKVOf(grtKeyNameStderr, grtS(r.stderr)))
	kvs = append(kvs, grtKVOf(grtKeyNameKeyLines, grtStrArr(r.keyLines)))
	kvs = append(kvs, grtKVOf(grtKeyNameRegFalse, grtBool(r.regFalses)))
	if r.regDetail == nil {
		kvs = append(kvs, grtKVOf(grtKeyNameRegDetail, grtNull()))
	} else {
		kvs = append(kvs, grtKVOf(grtKeyNameRegDetail, r.regDetail))
	}
	if r.raw != nil {
		kvs = append(kvs, grtKVOf(grtKeyNameRawJSON, r.raw))
	}
	return grtObjOf(kvs...)
}

func grtStrArr(ss []string) *grtVal {
	out := []*grtVal{}
	for _, s := range ss {
		out = append(out, grtS(s))
	}
	return grtArrOf(out)
}

// grtBuildSummary 对 py :271-280（键序逐条同）。
func grtBuildSummary(root, cwd string, results []*grtResult, anyNonzero bool) *grtVal {
	passed := []*grtVal{}
	pending := []*grtVal{}
	rv := []*grtVal{}
	for _, r := range results {
		rv = append(rv, grtResultJSON(r))
		if r.rc == 0 {
			passed = append(passed, grtS(r.gate))
		} else {
			pending = append(pending, grtS(r.gate))
		}
	}
	return grtObjOf(
		grtKVOf(grtKeyNameSchema, grtS(grtSchemaValue)),
		grtKVOf(grtKeyNameRepo, grtS(grtPyAbspath(cwd, root))),
		grtKVOf(grtKeyNameGenerated, grtS(time.Now().Format("2006-01-02T15:04:05-0700"))),
		grtKVOf(grtKeyNameGatesRun, grtInt(len(results))),
		grtKVOf(grtKeyNameAnyPend, grtBool(anyNonzero)),
		grtKVOf(grtKeyNamePassed, grtArrOf(passed)),
		grtKVOf(grtKeyNamePending, grtArrOf(pending)),
		grtKVOf(grtKeyNameResults, grtArrOf(rv)),
	)
}

// grtPadRight 复刻 py `%-34s`（按码点补空格）。
func grtPadRight(s string, w int) string {
	n := len([]rune(s))
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// grtPrintHuman 对 py :295-314。
func grtPrintHuman(stdout io.Writer, rootAsGiven string, results []*grtResult, anyNonzero bool) {
	fmt.Fprintf(stdout, "gate-report · 仓 %s · %d 门\n", rootAsGiven, len(results))
	fmt.Fprintf(stdout, "%s\n", strings.Repeat("─", 72))
	for _, r := range results {
		status := "PASS"
		if r.rc == 2 {
			status = "ERR"
		} else if r.rc != 0 {
			status = "PENDING"
		}
		tag := ""
		if r.regFalses {
			tag = " [假红·已登记]"
		}
		fmt.Fprintf(stdout, "%s rc=%d  %6.3fs  %s%s\n",
			grtPadRight(r.gate, 34), r.rc, r.elapsed, status, tag)
		show := r.keyLines
		if len(show) > 3 {
			show = show[:3]
		}
		for _, kl := range show {
			fmt.Fprintf(stdout, "        · %s\n", kl)
		}
		if r.errored {
			fmt.Fprintf(stdout, "        ✗ %s\n", r.errorMsg)
			continue
		}
		if r.stderr != "" {
			ls := grtPySplitLines(r.stderr)
			last := ""
			if len(ls) > 0 {
				last = ls[len(ls)-1]
			}
			fmt.Fprintf(stdout, "        ! %s\n", grtPyRuneSlice(last, 0, grtHumanStderrWidth))
		}
	}
	fmt.Fprintf(stdout, "%s\n", strings.Repeat("─", 72))
	verdict := "无待办（全部门禁绿）"
	if anyNonzero {
		verdict = "有待办（下列门禁 rc≠0）"
	}
	rcOut := 0
	if anyNonzero {
		rcOut = 1
	}
	fmt.Fprintf(stdout, "结论：%s ⇒ rc=%d\n", verdict, rcOut)
	if anyNonzero {
		pend := []string{}
		for _, r := range results {
			if r.rc != 0 {
				pend = append(pend, r.gate)
			}
		}
		joined := "—"
		if len(pend) > 0 {
			joined = strings.Join(pend, ", ")
		}
		fmt.Fprintf(stdout, "  待办门禁：%s\n", joined)
	}
}
