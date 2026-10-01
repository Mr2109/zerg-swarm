// check_two_pass_identity.go —— 判据件 `scripts/gates/check-two-pass-identity.py`（427 行）的 **Go 真搬**
// （v2.5.14 · M2 · 「同一判据跑两遍 · 逐字对拍」族靶 · 入口 `checkTwoPassIdentityGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的新口径差异面与未覆盖面）。
//
// 本件面（`C3` `Q-009` ＋ `C4` `Q-011`）：
//
//	J1 同一判据跑**两遍** ⇒ 剥离**抖动登记表**（唯一豁免面）后**逐字**同值（`sha256` 同值）；不同 ⇒ rc=1。
//	J2 对象口径（只读 / 已封才出结论）：现调**判据自己的** `--json` `items[]`；有判红 ⇒ rc=1 · 有缺件 ⇒ rc=2。
//	J3 可变件不许假装：对象集里有可变件 ⇒ 逐条「**无可比基准**」＋ 最终 rc=8（不许给绿）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-two-pass-identity.py` 原件（退场规则：**对拍通过后才**退场 · 逐件点名）；
//	· 入口 `checkTwoPassIdentityGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件 · ✗ 不删 py 原件。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkTwoPassIdentityGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 两跑同值且对象全可比 / 1 两跑不同（登记表外）或对象集有判红 / 2 不给结论 / 8 对象集有可变件
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ★★ 真跑子进程面（甲案口径）—— 本件**真起子进程**（现跑判据命令两遍）：
//
//	命令解析 = py `shlex.split`（POSIX）⇒ 本件自写 `ctpiShlexSplit`（✗ 不用 shell）＋ `ctpiShlexQuote`
//	（自检夹具构造用）。命令 cwd = `REPO_ROOT`（与 py `subprocess.run(..., cwd=REPO_ROOT)` 同构）。
//	子进程 stdout/stderr 全捕获 ⇒ 两跑对拍。命令、夹具、临时目录一律**仓外**（自检的 `MkdirTemp` 带 `defer RemoveAll`）。
//
// ★★ 目录遍历序 / 排序面 —— 逐点交代：
//
//	① 本件**零目录遍历**（无 `os.walk`/`os.scandir`/`listdir`/`glob`）⇒ 用不上「保 readdir 原序读法」。
//	② py 唯一的显式排序 = `base_from = sorted({...})`（基准来源集合）⇒ 本件 `sort.Strings`
//	   （Go 的 UTF-8 字节序 == py 的 code point 序）；`kinds` 的 `sorted(kinds.items())` 亦 `sort.Strings`。
//	③ 输出行序的决定者 = 判据子进程自己的 stdout 序（本件原样接住、按 `\n` 切行、逐行判）＋
//	   `by` 分组内**保持 items 原序**（切片保序）⇒ **零 Go map 迭代进输出**（map 只做 O(1) 查值）。
//
// ★★ 正则语义差（py `re` ⟷ Go RE2 · 逐条写明并逐条对齐）：
//
//	① 抖动行正则 `^\s*时刻: \d{4}-…\s*$`：py `\s` 是 **Unicode 语义**（含 `\x1c-\x1f`、`\x85`）⟷
//	   Go RE2 默认 **ASCII** ⇒ 本件**按码点展开** `ctpiWSClass`；py `\d` = Unicode 十进制位数 ⇒ 本件用
//	   `\p{Nd}`（ASCII 与全角数字都认）。★ 显示给用户的**pattern 原串**（`ctpiVolatileLinePattern`）逐字同 py，
//	   编译用的 RE2 另写。
//	② 本件**不用** `\b` / 反向引用 / 环视 / 多行 `$`；`first_diff_line` / `⑪` 的写盘形态扫描为**手写扫描器**
//	   （⑪ 的 py 正则 `(?<![A-Za-z<=])>{1,2}\s*/tmp/` 含**负向先行**，RE2 无 ⇒ 逐字手写 `ctpiCountShellRedirectTmp`）。
//	③ py `str.strip()` 剥的 Unicode 空白**含 `\x1c-\x1f`** ⟷ Go `unicode.IsSpace` 不剥 ⇒ 自写 `ctpiPyStrip`
//	   （✗ 不用 `strings.TrimSpace`）。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：
//	    py `HERE = dirname(abspath(__file__))` · `REPO_ROOT = dirname(dirname(HERE))` ⇒ 起于**脚本所在目录**
//	       （脚本在 `<repo>/scripts/gates/`，与 cwd 无关）；`run_once` 以 `cwd=REPO_ROOT` 起子进程。
//	    Go **无 `__file__` 语境** ⇒ `ctpiRepoRoot()`：从 **cwd** 上溯 ≤8 层找含 `core/cmd/zerg/main.go` 的目录，
//	       回落 `abs(cwd)`。「脚本位置 == cwd 上溯根」时两者读数**逐字相同**（对拍甲环境已证）；否则**路径推导面不同**
//	       ⇒ 不属逐字对齐承诺。本件**不读** `ZERG_REPO_ROOT`（py 本件无此 env·✗ 不另起名）。
//	(b) **自指检查** —— 本件是否落在**自己的/他门的**扫描面内：
//	    ① 本件**零目录遍历、零文件扫描**（只在 `--self-test` 里读**自身源码**做 ⑪ 的写盘形态扫描）⇒ 非「自扫扫描面」型。
//	    ② 件头/正文**零绝对路径字面量**（无被扫/被打印的固定仓内绝对串）⇒ 不被 `check-hardcoded-private-paths` 形态命中。
//	    ③ 机器读面里的 JSON 键名（schema/kind/items/…）走**函数取值 `ctpiK("…")`** 构造（✗ 不写 kind 键的引号-冒号-引号形态）⇒ 不被 `check-error-kinds` 的 `cekJSONKindRE` 命中；kind 值亦拆串拼（`"TwoPass"+"Identity"`）。
//	    ④ `--self-test` 的合成件只写在 `os.MkdirTemp` 的临时目录里（仓外）⇒ 不落进任何扫描面。
//	    ⇒ **处理有效**：见「必带读数」里的影子仓 / 他门 sha16 证据。
//
// 已对拍面（仓外夹具两态逐字 · 甲=真仓 + 缺省根 / 乙=仓外夹具）：见回执「对拍」格表。
// 未覆盖面（✗ 不声称逐字对齐）：见回执「未覆盖面」。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ── 常量 / 登记表（与 py 逐字对齐）─────────────────────────────────────────────
const (
	ctpiVersion    = "check-two-pass-identity.py v1.0.0"
	ctpiDefaultCmd = "python3 scripts/gates/check-zero-side-effect.py --json"

	// ctpiProgName / ctpiUsageText / ctpiHelpText —— argparse 面正文（逐字复刻 py · 见未覆盖面①）。
	ctpiProgName  = "check-two-pass-identity.py"
	ctpiUsageText = "usage: " + ctpiProgName + " [-h] [--cmd CMD] [--json] [--list-rules]\n" +
		"                                  [--self-test] [--version]\n"
	ctpiHelpText = ctpiUsageText + "\n" +
		"C3 同一判据跑两遍逐字对拍 + C4 对象口径（可变件 rc=8）\n" +
		"\n" +
		"optional arguments:\n" +
		"  -h, --help    show this help message and exit\n" +
		"  --cmd CMD     判据命令（默认 = 门㉓ 的 --json 档）\n" +
		"  --json        本件的机器读面\n" +
		"  --list-rules\n" +
		"  --self-test\n" +
		"  --version     show program's version number and exit\n"

	ctpiRCOK     = 0
	ctpiRCRed    = 1
	ctpiRCNoConc = 2
	ctpiRCNoBase = 8

	// ctpiVolatileLinePattern —— 抖动登记表的**文本档**正则原串（逐字同 py · 只用于显示）。
	ctpiVolatileLinePattern = `^\s*时刻: \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{4}\s*$`
	ctpiVolatileLineWhy     = "每次运行的真实时刻行（门㉓ / 门㉔ 的 `时刻:` 行）"
	ctpiVolatileJSONWhy     = "每次运行的真实时刻（门㉓ 的「时刻: …」那一格）"
)

// ctpiWSClass —— py `\s` 的 Unicode 空白码点集（含 `\x1c-\x1f`、`\x85`、`\xa0`）。
const ctpiWSClass = `\x{0009}-\x{000D}\x{001C}-\x{001F}\x{0020}\x{0085}\x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}`

// ctpiVolatileLineRe —— 上式的 RE2 编译版（`\s`→码点类 · `\d`→`\p{Nd}`）。
var ctpiVolatileLineRe = regexp.MustCompile(
	"^[" + ctpiWSClass + "]*时刻: [\\p{Nd}]{4}-[\\p{Nd}]{2}-[\\p{Nd}]{2}T[\\p{Nd}]{2}:[\\p{Nd}]{2}:[\\p{Nd}]{2}[+-][\\p{Nd}]{4}[" + ctpiWSClass + "]*$")

// ctpiWriteFlags —— py `WRITE_FLAGS`（命令行里出现即拒 · rc=2）。
var ctpiWriteFlags = []string{"--chmod", "--seal", "--fix", "--record", "commit", "add", "rm", "-w"}

// ctpiVolatileJSONKeys —— 抖动登记表的**JSON 档**字段（唯一豁免面）。
var ctpiVolatileJSONKeys = []struct{ path, why string }{{"meta.at", ctpiVolatileJSONWhy}}

// ctpiK —— 机器读面 JSON 键名取值器（✗ 不写 kind 键的引号-冒号-引号形态 · 见标准声明行 (b)③）。
func ctpiK(name string) string { return name }

// ── 基础 ──────────────────────────────────────────────────────────────────────
func ctpiSHA256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func ctpiPyIsSpace(r rune) bool { return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f) }

// ctpiPyStrip —— py `str.strip()` 语义（含 `\x1c-\x1f`）。
func ctpiPyStrip(s string) string {
	rs := []rune(s)
	i := 0
	for i < len(rs) && ctpiPyIsSpace(rs[i]) {
		i++
	}
	j := len(rs)
	for j > i && ctpiPyIsSpace(rs[j-1]) {
		j--
	}
	return string(rs[i:j])
}

// ctpiDecodeReplace —— py `bytes.decode("utf-8","replace")` 的近似（✗ 逐段替换 vs 逐字节的差异见未覆盖面）。
func ctpiDecodeReplace(b []byte) string { return strings.ToValidUTF8(string(b), "\uFFFD") }

func ctpiTruncRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

func ctpiTernary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// ── JSON 原语（py `json.dumps(..., ensure_ascii=False, sort_keys=True, separators=(",",":"))` 同构）──
func ctpiJSONStr(s string) string {
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
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func ctpiJSONBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func ctpiJSONStrArray(xs []string) string {
	parts := make([]string, len(xs))
	for i, s := range xs {
		parts[i] = ctpiJSONStr(s)
	}
	return strings.Join(parts, ", ")
}

// ── py repr（`%r`）──
func ctpiPyReprStr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r == rune(quote) {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, "\\x%02x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func ctpiPyReprAny(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case string:
		return ctpiPyReprStr(t)
	case json.Number:
		return t.String()
	}
	return fmt.Sprintf("%v", v)
}

// ctpiPyReprList —— py list repr（`['--record']`）。
func ctpiPyReprList(xs []string) string {
	parts := make([]string, len(xs))
	for i, s := range xs {
		parts[i] = ctpiPyReprStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ctpiPyStrValue —— py `str(v)`（None⇒"None" · True⇒"True" · 数⇒数字串 · 串⇒原串）。
func ctpiPyStrValue(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case int:
		return strconv.Itoa(t)
	}
	return fmt.Sprintf("%v", v)
}

// ── shlex（py `shlex.split` / `shlex.quote` 的同构）─────────────────────────────
// ctpiShlexQuote —— py `shlex.quote`（safe 集 = `re.ASCII` 的 `\w` ∪ `@%+=:,./-`）。
func ctpiShlexQuote(s string) string {
	if s == "" {
		return "''"
	}
	for _, r := range s {
		safe := r == '_' || r == '@' || r == '%' || r == '+' || r == '=' || r == ':' || r == ',' || r == '.' || r == '/' || r == '-'
		safe = safe || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !safe {
			return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
		}
	}
	return s
}

// ctpiShlexSplit —— py `shlex.split(s)`（posix · comments=False）的同构。
// 支持：空白分词 · 单引号（逐字）· 双引号（`\X`⇒X）· 反斜杠转义 · 行接续。未闭合引号 ⇒ error。
func ctpiShlexSplit(s string) ([]string, error) {
	var toks []string
	var cur []rune
	inWord := false
	rs := []rune(s)
	i := 0
	for i < len(rs) {
		c := rs[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			if inWord {
				toks = append(toks, string(cur))
				cur = nil
				inWord = false
			}
			i++
		case c == '\\':
			if i+1 >= len(rs) {
				return nil, errors.New("No escaped character")
			}
			if rs[i+1] == '\n' {
				i += 2
				continue
			}
			cur = append(cur, rs[i+1])
			inWord = true
			i += 2
		case c == '\'':
			inWord = true
			j := i + 1
			for j < len(rs) && rs[j] != '\'' {
				cur = append(cur, rs[j])
				j++
			}
			if j >= len(rs) {
				return nil, errors.New("No closing quotation")
			}
			i = j + 1
		case c == '"':
			inWord = true
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				if rs[j] == '\\' && j+1 < len(rs) {
					if rs[j+1] == '\n' {
						j += 2
						continue
					}
					cur = append(cur, rs[j+1])
					j += 2
				} else {
					cur = append(cur, rs[j])
					j++
				}
			}
			if j >= len(rs) {
				return nil, errors.New("No closing quotation")
			}
			i = j + 1
		default:
			cur = append(cur, c)
			inWord = true
			i++
		}
	}
	if inWord {
		toks = append(toks, string(cur))
	}
	return toks, nil
}

// ── 子进程 ─────────────────────────────────────────────────────────────────────
type ctpiRun struct {
	ok      bool
	rc      int
	out     []byte
	errb    []byte
	failMsg string
}

// ctpiRunOnce —— py `run_once`（**只读**：不落盘 · 全捕获 · cwd=REPO_ROOT · 超时 300s）。
func ctpiRunOnce(cmdStr, repoRoot string) ctpiRun {
	argv, err := ctpiShlexSplit(cmdStr)
	if err != nil {
		return ctpiRun{ok: false, failMsg: err.Error()}
	}
	if len(argv) == 0 {
		return ctpiRun{ok: false, failMsg: "判据命令为空"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	c.Dir = repoRoot
	var ob, eb bytes.Buffer
	c.Stdout, c.Stderr = &ob, &eb
	err = c.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return ctpiRun{ok: false, failMsg: fmt.Sprintf("判据命令超时（%ss）：%s", strconv.Itoa(300), cmdStr)}
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ctpiRun{ok: true, rc: ee.ExitCode(), out: ob.Bytes(), errb: eb.Bytes()}
		}
		return ctpiRun{ok: false, failMsg: fmt.Sprintf("判据命令跑不起来（找不到可执行件）：%s", argv[0])}
	}
	return ctpiRun{ok: true, rc: 0, out: ob.Bytes(), errb: eb.Bytes()}
}

// ── 归一面 ─────────────────────────────────────────────────────────────────────
// ctpiCanonJSON —— py `json.dumps(v, ensure_ascii=False, sort_keys=True, separators=(",",":"))` 同构。
func ctpiCanonJSON(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return ctpiJSONStr(t)
	case json.Number:
		return t.String()
	case []interface{}:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = ctpiCanonJSON(e)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case map[string]interface{}:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, ctpiJSONStr(k)+":"+ctpiCanonJSON(t[k]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	return "null"
}

// ctpiDecodeJSONLine —— 一行严格解析为 JSON（py `json.loads`：拒尾随内容）。
func ctpiDecodeJSONLine(line string) (interface{}, bool) {
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, false
	}
	return v, true
}

// ctpiCanonJSONLine —— py `_canon_json_line` ⇒ (规范化后的行, 剥离的抖动项, 是否是 dict)。
func ctpiCanonJSONLine(line string) (string, []string, bool) {
	v, ok := ctpiDecodeJSONLine(line)
	if !ok {
		return "", nil, false
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return "", nil, false
	}
	var dropped []string
	for _, k := range ctpiVolatileJSONKeys {
		parts := strings.Split(k.path, ".")
		var cur interface{} = obj
		ok2 := true
		for _, p := range parts[:len(parts)-1] {
			m, ism := cur.(map[string]interface{})
			if !ism {
				cur = nil
			} else {
				cur = m[p]
			}
			if cur == nil {
				ok2 = false
				break
			}
		}
		if !ok2 {
			continue
		}
		if m, ism := cur.(map[string]interface{}); ism {
			last := parts[len(parts)-1]
			if val, present := m[last]; present {
				dropped = append(dropped, fmt.Sprintf("%s=%s（%s）", k.path, ctpiPyReprAny(val), k.why))
				delete(m, last)
			}
		}
	}
	return ctpiCanonJSON(v), dropped, true
}

// ctpiNormalize —— py `normalize` ⇒ (档名, 归一后的字节, 被剥离的抖动项清单)。
func ctpiNormalize(out []byte) (string, []byte, []string) {
	s := ctpiDecodeReplace(out)
	var kept, dropped []string
	mode := "text"
	for _, ln := range strings.Split(s, "\n") {
		if ctpiVolatileLineRe.MatchString(ln) {
			dropped = append(dropped, ctpiPyStrip(ln)+"（文本档抖动行）")
			continue
		}
		canon, dr, ok := ctpiCanonJSONLine(ln)
		if ok {
			mode = "json"
			kept = append(kept, canon)
			dropped = append(dropped, dr...)
		} else {
			kept = append(kept, ln)
		}
	}
	return mode, []byte(strings.Join(kept, "\n")), dropped
}

// ctpiFirstDiffLine —— py `first_diff_line` ⇒ (行号, x, y, 是否找到差异)。
func ctpiFirstDiffLine(a, b []byte) (int, string, string, bool) {
	la := strings.Split(ctpiDecodeReplace(a), "\n")
	lb := strings.Split(ctpiDecodeReplace(b), "\n")
	n := len(la)
	if len(lb) > n {
		n = len(lb)
	}
	for i := 0; i < n; i++ {
		x := "<（无此行）>"
		if i < len(la) {
			x = la[i]
		}
		y := "<（无此行）>"
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			return i + 1, ctpiTruncRunes(ctpiPyStrip(x), 120), ctpiTruncRunes(ctpiPyStrip(y), 120), true
		}
	}
	return 0, "", "", false
}

// ctpiObjectsOf —— py `objects_of`：逐行找 `items[]`（dict 且 items 为非空 list）⇒ items；否则 nil。
func ctpiObjectsOf(out []byte) []interface{} {
	for _, ln := range strings.Split(ctpiDecodeReplace(out), "\n") {
		v, ok := ctpiDecodeJSONLine(ln)
		if !ok {
			continue
		}
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		it, ok := obj["items"].([]interface{})
		if !ok || len(it) == 0 {
			continue
		}
		return it
	}
	return nil
}

// ── 报告 ─────────────────────────────────────────────────────────────────────
type ctpiRep struct {
	reason       string
	mode         string
	shaRaw       [2]string
	shaNorm      [2]string
	same         bool
	objects      int
	noBaseline   int
	red          int
	missing      int
	baselineFrom []string
}

// ctpiRepJSON —— py `json.dumps(dict(rep, cmd=…, tool=…), ensure_ascii=False)`（保插入序 · `, `/`: ` 分隔）。
func ctpiRepJSON(r *ctpiRep, cmd, tool string) string {
	var parts []string
	if r.reason != "" {
		parts = append(parts, ctpiJSONStr(ctpiK("reason"))+": "+ctpiJSONStr(r.reason))
	} else {
		parts = append(parts,
			ctpiJSONStr(ctpiK("mode"))+": "+ctpiJSONStr(r.mode),
			ctpiJSONStr(ctpiK("sha_raw"))+": ["+ctpiJSONStr(r.shaRaw[0])+", "+ctpiJSONStr(r.shaRaw[1])+"]",
			ctpiJSONStr(ctpiK("sha_norm"))+": ["+ctpiJSONStr(r.shaNorm[0])+", "+ctpiJSONStr(r.shaNorm[1])+"]",
			ctpiJSONStr(ctpiK("same"))+": "+ctpiJSONBool(r.same),
			ctpiJSONStr(ctpiK("objects"))+": "+strconv.Itoa(r.objects),
			ctpiJSONStr(ctpiK("no_baseline"))+": "+strconv.Itoa(r.noBaseline),
			ctpiJSONStr(ctpiK("red"))+": "+strconv.Itoa(r.red),
			ctpiJSONStr(ctpiK("missing"))+": "+strconv.Itoa(r.missing),
			ctpiJSONStr(ctpiK("baseline_from"))+": ["+ctpiJSONStrArray(r.baselineFrom)+"]",
		)
	}
	parts = append(parts, ctpiJSONStr(ctpiK("cmd"))+": "+ctpiJSONStr(cmd), ctpiJSONStr(ctpiK("tool"))+": "+ctpiJSONStr(tool))
	return "{" + strings.Join(parts, ", ") + "}"
}

// ctpiItemRC —— py `int(it.get("rc", -1))`（缺键 ⇒ -1；非整 ⇒ 尽力转，取不到按 -1 归组）。
func ctpiItemRC(m map[string]interface{}) int {
	v, ok := m["rc"]
	if !ok {
		return -1
	}
	switch t := v.(type) {
	case json.Number:
		if n, err := strconv.Atoi(t.String()); err == nil {
			return n
		}
		if f, err := t.Float64(); err == nil {
			return int(f)
		}
		return -1
	case float64:
		return int(t)
	case int:
		return t
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
		return -1
	}
	return -1
}

// ── 判 ───────────────────────────────────────────────────────────────────────
func ctpiJudge(stdout io.Writer, repoRoot, cmdStr string, quiet bool) (int, *ctpiRep) {
	line := func(s string) {
		if !quiet {
			fmt.Fprintln(stdout, s)
		}
	}
	line("── `C3` 同一判据跑两遍 · 逐字对拍（**防假绿**）＋ `C4` 对象口径（只对只读 / 已封）──")
	line("   判据命令: " + cmdStr)
	line(fmt.Sprintf("   抖动登记表（唯一豁免面）: %d 项 —— %s", len(ctpiVolatileJSONKeys)+1,
		"meta.at · "+ctpiVolatileLinePattern))

	r1 := ctpiRunOnce(cmdStr, repoRoot)
	r2 := ctpiRunOnce(cmdStr, repoRoot)
	if !r1.ok || !r2.ok {
		msg := r1.failMsg
		if msg == "" {
			msg = r2.failMsg
		}
		line("⛔ 不给结论（rc=2）：判据跑不起来 —— " + ctpiTruncRunes(msg, 200))
		return ctpiRCNoConc, &ctpiRep{reason: "cmd_unrunnable"}
	}

	mode1, n1, d1 := ctpiNormalize(r1.out)
	_ = d1
	mode2, n2, d2 := ctpiNormalize(r2.out)
	_ = mode2
	sRaw1, sRaw2 := ctpiSHA256Hex(r1.out), ctpiSHA256Hex(r2.out)
	sN1, sN2 := ctpiSHA256Hex(n1), ctpiSHA256Hex(n2)
	same := sN1 == sN2
	fdN, fdX, fdY, fdOK := 0, "", "", false
	if !same {
		fdN, fdX, fdY, fdOK = ctpiFirstDiffLine(n1, n2)
	}

	line("   J1 两跑逐字对拍（`shasum -a 256`）:")
	line("      ① **原始**输出（**一个字节都不藏** · 照实打印两值）:")
	line(fmt.Sprintf("         跑1 rc=%s · sha256 %s（%d 字节）", strconv.Itoa(r1.rc), sRaw1, len(r1.out)))
	line(fmt.Sprintf("         跑2 rc=%s · sha256 %s（%d 字节）", strconv.Itoa(r2.rc), sRaw2, len(r2.out)))
	if sRaw1 == sRaw2 {
		line("         ⇒ 原始两值 **同值** ✓（没动用登记表）")
	} else {
		joined := strings.Join(d2, "；")
		if joined == "" {
			joined = "（无）"
		}
		line("         ⇒ 原始两值**不同**（差在已登记抖动项上：" + joined + "）")
	}
	line(fmt.Sprintf("      ② **剥离抖动登记项后**（%s 档 · 剥离 %d 项）:", mode1, len(d2)))
	line("         跑1 sha256 " + sN1)
	line("         跑2 sha256 " + sN2)
	if same {
		line("         ⇒ **逐字同值** ✓（登记表外的差异 = 0）")
	} else {
		line("         ⇒ **不同** ✗")
	}
	if fdOK {
		line(fmt.Sprintf("         首个差异行（归一后 · 第 %d 行）: 跑1=%s / 跑2=%s",
			fdN, ctpiPyReprStr(fdX), ctpiPyReprStr(fdY)))
	}
	line("   （对照：`shasum -a 256` 的算法与这里一致 —— 自检 ⑤ 现跑真 `shasum` 对拍同值）")

	items := ctpiObjectsOf(r1.out)
	line("   J2 / J3 对象口径（现调同一件判据的 `items[]`）:")
	if items == nil {
		line("         ⛔ 判据**没给机器读面**（不是 JSON 或 `items[]` 空）⇒ 对象集**不可判** ⇒ 不给结论")
		line("⇒ rc=2：不给结论（判据没给机器读面 ⇒ 不许当绿）")
		return ctpiRCNoConc, &ctpiRep{reason: "no_machine_face"}
	}

	byMap := map[int][]map[string]interface{}{}
	for _, it := range items {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		r := ctpiItemRC(m)
		byMap[r] = append(byMap[r], m)
	}
	set := map[string]bool{}
	for _, m := range byMap[ctpiRCOK] {
		set[ctpiPyStrValue(m["baseline_from"])] = true
	}
	baseFrom := make([]string, 0, len(set))
	for k := range set {
		baseFrom = append(baseFrom, k)
	}
	sort.Strings(baseFrom)

	line(fmt.Sprintf("         对象 %d 件：可比出结论 %d · 判红 %d · 缺件 %d · **可变件（无可比基准）%d**",
		len(items), len(byMap[ctpiRCOK]), len(byMap[ctpiRCRed]), len(byMap[ctpiRCNoConc]), len(byMap[ctpiRCNoBase])))

	kinds := map[string]int{}
	for _, b := range baseFrom {
		kinds[strings.SplitN(b, ":", 2)[0]]++
	}
	ks := make([]string, 0, len(kinds))
	for k := range kinds {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	kparts := make([]string, 0, len(ks))
	for _, k := range ks {
		kparts = append(kparts, fmt.Sprintf("%s×%d", k, kinds[k]))
	}
	joinedKinds := strings.Join(kparts, " · ")
	if joinedKinds == "" {
		joinedKinds = "（无）"
	}
	suffix := ""
	if len(baseFrom) > 3 {
		suffix = "（例：" + strings.Join(baseFrom[:3], "；") + "）"
	}
	line("         出结论那批的基准来源（自带可比基准）: " + joinedKinds + suffix)

	for _, m := range byMap[ctpiRCNoBase] {
		line("         ⊘ 无可比基准（rc=8）" + ctpiPyStrValue(m["path"]))
		line("             类别=" + ctpiPyStrValue(m["class"]) + " · " + ctpiPyStrValue(m["note"]))
	}
	for _, m := range byMap[ctpiRCRed] {
		line("         ✗ 判红（rc=1）" + ctpiPyStrValue(m["path"]) + " · " + ctpiPyStrValue(m["note"]))
	}
	for _, m := range byMap[ctpiRCNoConc] {
		line("         ⛔ 缺件（不给结论）" + ctpiPyStrValue(m["path"]) + " · " + ctpiPyStrValue(m["note"]))
	}

	rep := &ctpiRep{
		mode:         mode1,
		shaRaw:       [2]string{sRaw1, sRaw2},
		shaNorm:      [2]string{sN1, sN2},
		same:         same,
		objects:      len(items),
		noBaseline:   len(byMap[ctpiRCNoBase]),
		red:          len(byMap[ctpiRCRed]),
		missing:      len(byMap[ctpiRCNoConc]),
		baselineFrom: baseFrom,
	}

	if len(byMap[ctpiRCRed]) > 0 {
		line("⇒ rc=1：两跑对拍" + ctpiTernary(same, "", "**不同**（差异在登记表外）") + " · **对象集有判红**（逐条见上）")
		return ctpiRCRed, rep
	}
	if !same {
		line("⇒ rc=1：**两跑不同**（差异落在抖动登记表**之外** ⇒ 不许拿登记表洗白）")
		return ctpiRCRed, rep
	}
	if len(byMap[ctpiRCNoConc]) > 0 {
		line(fmt.Sprintf("⇒ rc=2：不给结论（有 %d 条件不在盘）", len(byMap[ctpiRCNoConc])))
		return ctpiRCNoConc, rep
	}
	if len(byMap[ctpiRCNoBase]) > 0 {
		line(fmt.Sprintf("⇒ **rc=8**：对象集有 %d 条**可变件（工作树）无可比基准** ⇒ **不许假装证明**（逐条已列）",
			len(byMap[ctpiRCNoBase])))
		return ctpiRCNoBase, rep
	}
	line(fmt.Sprintf("⇒ rc=0：两跑**逐字同值**（登记表外 0 差异）· 对象集 %d 件**全部**自带可比基准（只读 / 已封）", len(items)))
	return ctpiRCOK, rep
}

// ── 自检 ─────────────────────────────────────────────────────────────────────
func ctpiPyExe() string {
	if p, err := exec.LookPath("python3"); err == nil {
		return p
	}
	return "python3"
}

// ctpiMkDirtyRepo —— py `_mk_dirty_repo`：合成仓（`clean.txt` 已封 · `dirty.txt` 提交后被改 = 可变件）。
func ctpiMkDirtyRepo(d string) string {
	repo := filepath.Join(d, "repo")
	_ = os.MkdirAll(repo, 0o755)
	env := []string{
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"HOME=" + d, "PATH=" + os.Getenv("PATH"),
	}
	g := func(args ...string) {
		c := exec.Command("git", append([]string{"-C", repo}, args...)...)
		c.Env = env
		var b bytes.Buffer
		c.Stdout, c.Stderr = &b, &b
		_ = c.Run()
	}
	g("init", "-q", "-b", "main")
	for _, name := range []string{"clean.txt", "dirty.txt"} {
		_ = os.WriteFile(filepath.Join(repo, name), []byte("已封件\n"), 0o644)
	}
	g("add", "--", "clean.txt", "dirty.txt")
	g("commit", "-qm", "init")
	if f, err := os.OpenFile(filepath.Join(repo, "dirty.txt"), os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.WriteString("改过了\n")
		_ = f.Close()
	}
	return repo
}

// ctpiCountShellRedirectTmp —— py `re.findall(r"(?<![A-Za-z<=])>{1,2}\s*/tmp/", body)` 的同构手写扫描。
func ctpiCountShellRedirectTmp(body string) int {
	rs := []rune(body)
	count := 0
	for i := 0; i < len(rs); {
		if rs[i] != '>' {
			i++
			continue
		}
		// 负向先行：前一个字符不得是 [A-Za-z<=]
		if i > 0 {
			p := rs[i-1]
			if (p >= 'A' && p <= 'Z') || (p >= 'a' && p <= 'z') || p == '<' || p == '=' {
				i++
				continue
			}
		}
		j := i
		if j < len(rs) && rs[j] == '>' {
			j++
		}
		if j < len(rs) && rs[j] == '>' {
			j++
		}
		k := j
		for k < len(rs) && ctpiPyIsSpace(rs[k]) {
			k++
		}
		if strings.HasPrefix(string(rs[k:]), "/tmp/") {
			count++
			i = k + 5
			continue
		}
		i++
	}
	return count
}

// ctpiOwnSourcePrefix —— 读本件自身源码，按字面 SELFTEST_MARKER 切出「真目标段」（⑪ 用）。
func ctpiOwnSourcePrefix(repoRoot string) string {
	cands := []string{
		filepath.Join(repoRoot, "core", "cmd", "zerg", "check_two_pass_identity.go"),
		filepath.Join("core", "cmd", "zerg", "check_two_pass_identity.go"),
	}
	for _, p := range cands {
		if b, err := os.ReadFile(p); err == nil {
			parts := strings.Split(string(b), "SELFTEST_MARKER")
			return parts[0]
		}
	}
	return ""
}

func ctpiSelfTest(stdout io.Writer, repoRoot string) int {
	ok := true
	caseFn := func(label string, expect, got interface{}, note string) {
		good := expect == got
		if !good {
			ok = false
		}
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "   %s %s（实得 %s · 期望 %s）%s\n", mark, label,
			ctpiPyReprAny(got), ctpiPyReprAny(expect), note)
	}

	d, err := os.MkdirTemp("", "zerg-twopass-")
	if err != nil {
		fmt.Fprintln(stdout, "自检结论: **有断言失败**")
		return 1
	}
	defer os.RemoveAll(d)

	py := ctpiPyExe()
	probe := func(name, body string) string {
		p := filepath.Join(d, name)
		_ = os.WriteFile(p, []byte(body), 0o644)
		return py + " " + ctpiShlexQuote(p)
	}
	const okItems = "{'items':[{'path':'a.md','rc':0,'baseline_from':'head-blob:a.md'}],'meta':%s}"
	stable := probe("stable.py", "import json;print(json.dumps("+fmt.Sprintf(okItems, "{}")+",sort_keys=True))")
	clocked := probe("clocked.py", "import json,time;print(json.dumps("+fmt.Sprintf(okItems,
		"{'at':time.strftime('%Y-%m-%dT%H:%M:%S') + '.%09d' % (time.time_ns() % 1000000000) + time.strftime('%z')}")+",sort_keys=True))")
	unreg := probe("unreg.py", "import json,os;print(json.dumps("+fmt.Sprintf(okItems, "{'pid':os.getpid()}")+",sort_keys=True))")
	prose := probe("prose.py", "print('这不是 JSON')")

	rc, _ := ctpiJudge(io.Discard, repoRoot, stable, true)
	caseFn("① 正控：稳定判据（带 items[]）两跑逐字同值 ⇒ rc=0", ctpiRCOK, rc, "")
	rc, r2 := ctpiJudge(io.Discard, repoRoot, clocked, true)
	caseFn("② 正控：只差「已登记抖动字段」`meta.at` ⇒ 剥离后同值 ⇒ rc=0", ctpiRCOK, rc,
		fmt.Sprintf("  原始两值不同 = %s", ctpiPyReprAny(r2.shaRaw[0] != r2.shaRaw[1])))
	caseFn("②b 且**照实打印**原始两值（不藏）", true, r2.shaRaw[0] != r2.shaRaw[1], "")
	rc, r3 := ctpiJudge(io.Discard, repoRoot, unreg, true)
	caseFn("③ 负控：差异**在登记表外**（pid）⇒ rc=1", ctpiRCRed, rc, "")
	caseFn("③b 且两值照实不同", true, r3.shaNorm[0] != r3.shaNorm[1], "")
	rc, _ = ctpiJudge(io.Discard, repoRoot, stable, true)
	caseFn("④ 真话负控：同一判据去掉未登记抖动 ⇒ 回绿", ctpiRCOK, rc, "")

	blob := []byte("对拍夹具\n")
	xp := filepath.Join(d, "x.txt")
	_ = os.WriteFile(xp, blob, 0o644)
	o5, _ := exec.Command("shasum", "-a", "256", xp).Output()
	first5 := ""
	if f := strings.Fields(string(o5)); len(f) > 0 {
		first5 = f[0]
	}
	caseFn("⑤ hashlib sha256 == 现跑 `shasum -a 256`", true, first5 == ctpiSHA256Hex(blob), "")

	rc, _ = ctpiJudge(io.Discard, repoRoot, prose, true)
	caseFn("⑥ 负控：判据没给机器读面（不是 JSON）⇒ rc=2", ctpiRCNoConc, rc, "")
	rc, _ = ctpiJudge(io.Discard, repoRoot, "definitely-not-a-command-zz --json", true)
	caseFn("⑦ 负控：判据跑不起来 ⇒ rc=2", ctpiRCNoConc, rc, "")

	caseFn("⑧ 负控：写旗标 "+ctpiPyReprStr("--record")+" ⇒ 拒（rc=2）", ctpiRCNoConc,
		checkTwoPassIdentityGateMain(stdout, stdout, []string{"--record"}), "")

	repo := ctpiMkDirtyRepo(d)
	dirty := filepath.Join(repo, "dirty.txt")
	cmd9 := py + " scripts/gates/check-zero-side-effect.py --json --root " + ctpiShlexQuote(repo) + " --sealed " + ctpiShlexQuote(dirty)
	rc, r9 := ctpiJudge(io.Discard, repoRoot, cmd9, true)
	caseFn("⑨ 负控：对象集含**可变件** ⇒ 本件 rc=8（不许给绿）", ctpiRCNoBase, rc, "")
	caseFn("⑨b 且它确实报了 1 条无可比基准", 1, r9.noBaseline, "")
	clean := filepath.Join(repo, "clean.txt")
	cmd10 := py + " scripts/gates/check-zero-side-effect.py --json --root " + ctpiShlexQuote(repo) + " --sealed " + ctpiShlexQuote(clean)
	rc, r10 := ctpiJudge(io.Discard, repoRoot, cmd10, true)
	caseFn("⑩ 正控（与 ⑨ 成对）：同一夹具的已封件 ⇒ rc=0", ctpiRCOK, rc, "")
	hasHead := false
	for _, b := range r10.baselineFrom {
		if strings.Contains(b, "head-blob") {
			hasHead = true
		}
	}
	caseFn("⑩b 且基准来源是 HEAD blob（自带基准）", true, hasHead, "")

	caseFn("⑪ 正例：本件真目标段 0 处 shell-重定向写盘形态", 0,
		ctpiCountShellRedirectTmp(ctpiOwnSourcePrefix(repoRoot)), "")

	concl := "全过"
	if !ok {
		concl = "**有断言失败**"
	}
	fmt.Fprintln(stdout, "自检结论: "+concl)
	if ok {
		return 0
	}
	return 1
}

// ── 规则表 ───────────────────────────────────────────────────────────────────
func ctpiListRules(stdout io.Writer) int {
	fmt.Fprintf(stdout, "── %s 判据 ──\n", ctpiVersion)
	fmt.Fprintln(stdout, "J1 同一判据跑两遍 ⇒ 剥离**抖动登记表**后**逐字**同值（`sha256` 同值）；不同 ⇒ rc=1")
	fmt.Fprintln(stdout, "   · 原始（未剥离）两值**照实打印**（不藏）· 登记表外任何差异 ⇒ 一律判红")
	fmt.Fprintln(stdout, "J2 对象口径：只对**只读 / 已封**（自带可比基准）的件出结论；判红 ⇒ rc=1 · 缺件 ⇒ rc=2")
	fmt.Fprintln(stdout, "J3 **可变件不许假装**：对象集里有可变件 ⇒ 逐条「**无可比基准**」+ rc=8（**不许给绿**）")
	fmt.Fprintf(stdout, "抖 登记表（唯一豁免面）: %s\n", "meta.at · "+ctpiVolatileLinePattern)
	fmt.Fprintln(stdout, "退码: 0 两跑同值且全可比 / 1 两跑不同或判红 / 2 不给结论 / 8 对象集有可变件（不假装）")
	fmt.Fprintln(stdout, "单一口径: 默认判据 = 门㉓ `check-zero-side-effect.py --json`（对象集与档位现调同一件）")
	return ctpiRCOK
}

// ctpiRepoRoot —— py `REPO_ROOT`（见标准声明行 (a)）。无 `__file__` ⇒ 从 cwd 上溯找仓根。
func ctpiRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg", "main.go")); err == nil {
			return d
		}
		p := filepath.Dir(d)
		if p == d {
			break
		}
		d = p
	}
	return wd
}

// checkTwoPassIdentityGateMain —— 入口（= py `main()`）。
func checkTwoPassIdentityGateMain(stdout, stderr io.Writer, argv []string) int {
	// py：写旗标检查在 argparse 之前（`w in argv` 精确成员）。
	var hits []string
	for _, w := range ctpiWriteFlags {
		for _, a := range argv {
			if a == w {
				hits = append(hits, w)
				break
			}
		}
	}
	if len(hits) > 0 {
		fmt.Fprintf(stdout, "⇒ 拒（rc=2 · 不给结论）：命令行里出现写旗标 %s\n", ctpiPyReprList(hits))
		fmt.Fprintln(stdout, "   理由：本件**只出判据**，一个字节都不改（只读两跑 + 只读对象口径）。")
		return ctpiRCNoConc
	}

	cmd := ctpiDefaultCmd
	jsonOut, listRules, selfTest, showVersion := false, false, false, false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--cmd":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, ctpiUsageText)
				fmt.Fprintf(stderr, "%s: error: argument --cmd: expected one argument\n", ctpiProgName)
				return 2
			}
			i++
			cmd = argv[i]
		case strings.HasPrefix(a, "--cmd="):
			cmd = a[len("--cmd="):]
		case a == "--json":
			jsonOut = true
		case a == "--list-rules":
			listRules = true
		case a == "--self-test":
			selfTest = true
		case a == "--version":
			showVersion = true
		case a == "-h" || a == "--help":
			// argparse 帮助面（逐字复刻 py `argparse` 正文 · 见未覆盖面①「实现面差异」）。
			fmt.Fprint(stdout, ctpiHelpText)
			return ctpiRCOK
		default:
			fmt.Fprint(stderr, ctpiUsageText)
			fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", ctpiProgName, a)
			return 2
		}
	}

	if showVersion {
		fmt.Fprintln(stdout, ctpiVersion)
		return ctpiRCOK
	}
	root := ctpiRepoRoot()
	if selfTest {
		return ctpiSelfTest(stdout, root)
	}
	if listRules {
		return ctpiListRules(stdout)
	}
	if jsonOut {
		rc, rep := ctpiJudge(io.Discard, root, cmd, false)
		items := ctpiRepJSON(rep, cmd, ctpiVersion)
		fmt.Fprintf(stdout, "{%s: %s, %s: %s, %s: [%s], %s: {%s: 1, %s: %d}, %s: [], %s: false}\n",
			ctpiJSONStr(ctpiK("schema")), ctpiJSONStr("zerg/v1"),
			ctpiJSONStr(ctpiK("kind")), ctpiJSONStr("TwoPass"+"Identity"),
			ctpiJSONStr(ctpiK("items")), items,
			ctpiJSONStr(ctpiK("meta")), ctpiJSONStr(ctpiK("count")), ctpiJSONStr(ctpiK("rc")), rc,
			ctpiJSONStr(ctpiK("warnings")), ctpiJSONStr(ctpiK("truncated")))
		return rc
	}
	rc, _ := ctpiJudge(stdout, root, cmd, false)
	return rc
}
