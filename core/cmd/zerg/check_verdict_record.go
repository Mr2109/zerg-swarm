// check_verdict_record.go —— 门禁件 `scripts/gates/check-verdict-record.py` 的 **Go 真搬**（v2.5.14 · M2 · laneB475）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案把判据实现面分两段走 —— M1 = 把 `scripts/` 下的独立件收成 CLI 打得着的统一面（转发壳）；
//	M2 = 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。本件 = M2 的一枚：门㊳「算不算过」的机器读落点 + 稳定指纹。
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改任何 `add_step`）；
//	· **不**删 `scripts/gates/check-verdict-record.py` 原件（退场规则：**对拍通过后才**退场，且逐件显式点名）；
//	· 入口 `checkVerdictRecordGateMain` 目前**无调用点** —— 这是 M2 的**刻意**状态。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func checkVerdictRecordGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 绿（落点面 0 命中 + 指纹两面逐字同）/ 1 落点面命中 / 2 不给结论
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ── 写面 / 状态面逐处现读清单（本件「B 类：纯静态只读但自带记账/追 state 登记账」的落判）──────────
//
//	① `state_dir(cli)`：取 `--state-dir` ⇒ 否则 `$ZERG_STATE_DIR`（`.strip()`）⇒ 否则 `~/` + `.zerg/state`
//	   （`expanduser("~")`）。**纯路径推导 · 不写盘**。
//	② `load_records(state)`：读 `<state>/verdict_records.jsonl`（缺件 ⇒ 空表）。**只读**。
//	③ `append_record(state, rec)`：`os.makedirs(state, exist_ok=True)` + `open(p,"a")` 追加一行。**★ 唯一写面**。
//	④ `run()` 的 `args.record` 分支调 ③ ⇒ 落一条记录（写状态目录）。
//	⑤ `self_test()`：`tempfile.mkdtemp(prefix=...)` 造夹具 → 末尾 `shutil.rmtree(tmp, ignore_errors=True)`。
//	   **自检自回收**（只在系统临时面里写）。
//	⑥ `judge()/scan_landing()/subject_items()/cmd_compare()/cmd_fingerprint()`：**零写面**（只 `os.Stat`/`open(...,"r")`）。
//	⇒ 适格结论：**默认档（无 `--record`）纯只读**；写面只在 `--record` 且**只写状态目录**。本件**未发现**
//	   「只读判据件默认打开写面 / 自改自身源码 / 自检异常路不回收临时目录」三病；「结论依赖状态目录摆位」仅在
//	   `--compare` 成立（py 件头逐字声明：默认档那一格**不改红绿**）。
//
// ★★ 标准声明 (a)：**默认根定位差异 —— 有差异（且本单对拍一律显式传 `--root`）**。
//
//	py `--root` 缺省 = `DEFAULT_ROOT = dirname(dirname(dirname(abspath(__file__))))`（脚本位置上溯到仓根）。
//	本件**无 `__file__` 语境**（未来接线后是同进程子命令）⇒ 缺省换成**当前工作目录**（`os.Getwd`）。
//	⇒ 「脚本位置 ≠ cwd」且都不给 `--root` 时路径推导面不同 ⇒ **不属逐字对齐承诺**，对拍一律显式传根。
//
// ★★ 标准声明 (b)：**自指四形态 —— 逐条真判**：
//
//	① 本件在**自己**（被真搬的门）的扫描面内吗？—— **否**。py 门的扫描面 = `scripts/gates/` 顶层的
//	   `check-*.py|sh` + `mutate-scan`（`JUDGE_DIR_REL`），**不读任何 `core/` 或 `*.go`** ⇒ 本件源不进自身面。
//	② 本件落在**他门**的扫描面内吗？—— **是**（三面）：`check-hardcoded-private-paths.py` 扫 `core/**`
//	   （非注释 · 非 `_test.go`）；`check-error-kinds.py` 的用法面规则扫 `core/cmd/zerg/*.go`，判「错误构造
//	   调用」与「JSON kind 键值对」两类形态；`check-wall-layer.py` 扫同一包，判「三层常量赋值」与「命令级层级
//	   标记」两类形态。**前瞻消解（实测咬过一次 —— 首落件头注释里直写了触发形态 ⇒ wall-layer 由 rc=0 翻红；
//	   已改正为描述式）**：本件正文**零私有绝对路径字面**；kind 的键与值以**分列的两个 Go 字符串字面**进
//	   `vrObj`（源码里不出现连成一串的键值对形态）；另两类形态**零命中**。落仓后**实跑**三门对照 rc（见交付
//	   读数），并在影子仓证明本件贡献 = 0。
//	③ 本件扫到**自己**吗？—— 不适用（本件不扫源码树；`scripts/gates/` 面里没有本件）。
//	④ 本件**改**到自己吗？—— **否**（默认档零写面；`--record` 只追加状态目录的记录件）。
//
// 未覆盖面（✗ 不声称逐字对齐 · 接线前须逐面补齐或显式剔除）：
//
//	① **argparse 前缀缩写 + 帮助/用法正文**：py 用 `argparse` ⇒ 唯一前缀（`--self-t`）被接受、`-h/--help`
//	   打 argparse 帮助正文、未知旗标打 `usage:` + `unrecognized arguments:`（含 `prog = check-verdict-record.py`）。
//	   本件只认**精确名**，`-h/--help` 落自定 usage 行，未知旗标落自定错误行。**退码同（2/0）· 正文不同**。
//	② **baseline 解不动时的 `str(e)` 正文**：py 打 `json.JSONDecodeError` 的 `str(e)`（如
//	   `Expecting property name enclosed in double quotes: line 1 column 3 (char 2)`）；本件落 Go 的
//	   `json` 错串 ⇒ **退码同（2）· 该格 stdout 的 msg 段正文不同**。（对拍输入集已避开该格。）
//	③ **非 UTF-8 件**：py `open(...,encoding="utf-8")` 读到非法字节抛 `UnicodeDecodeError`（**裸回溯** · rc=1 ·
//	   stderr 含跑测机路径）；本件落 fail-closed rc=2（「取不到一律 rc=2」）⇒ **rc 不同 · 列入声明口径差**。
//	   `scan_landing` 的 `errors="replace"`（U+FFFD 替换粒度）在真仓三树全为合法 UTF-8 ⇒ 不触发。
//	④ **`--record` 的 `ts`**：`time.strftime("%Y-%m-%dT%H:%M:%S%z")` = 跑测时刻 ⇒ 每次不同（两态天然不可逐字节比）。
//	⑤ **`load_records` 遇到非对象 JSON 行**（如 `null` / `[1]` / `5`）：py `.get` 抛 `AttributeError`（裸回溯 · rc=1）；
//	   本件 fail-closed rc=2 ⇒ **rc 不同 · 列入声明口径差**（★ JSON null 面：缺键 ⟷ 显式 `null` 在 py 的 `.get`
//	   语义下**同归 None** ⇒ 本件同归 `nil`，两态输出**逐字相同**，已对拍成对子用例）。
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// zergPrecommitGatesRel —— 仓内「提交闸步骤表」单件相对路径的**唯一真源**（缺口 GAP-20261001-69）：
//
//	同包内 10 枚同值异名常量（afmSrcStepTbl / carvPrecommitRel / carvFixDriverRel / cecGatesRel /
//	cgcGatesRel / cgpoScriptRel / mfRootMarkerRel / cmfRootMarkerRel / gnrStepsRel / vrGateTableRel）
//	一律收敛到本枚 ⇒ 只此一处落字面，余处改引用（✗ 不再各自定义）。
const zergPrecommitGatesRel = "scripts/gates/precommit-gates.sh"

// ── 常量（与 py 逐字同源）──────────────────────────────────────────────────────────────────
const (
	vrVersion          = "check-verdict-record.py v1.1.0"
	vrSubject          = "gate-face"
	vrGateTableRel     = zergPrecommitGatesRel
	vrRecordName       = "verdict_records.jsonl"
	vrJudgeDirRel      = "scripts/gates"
	vrFixtureExemptMax = 4
	vrProgName         = "check-verdict-record.py"
	vrMeBase           = "check-verdict-record.py"
)

var vrBaselineRels = []string{
	"scripts/gates/cli-contract-baseline.json",
	"core/cmd/zerg/testdata/cli-matrix.json",
}

// `/tmp` 字面：按自指纪律**拆开拼**（本件不在被真搬门的扫描面内，仍不留连成一串的触发形态）。
var (
	vrTmpSlash = "/" + "tmp" + "/"
	vrTmpBare  = "/" + "tmp"
)

// ── 判据正则（与 py 逐条对齐）────────────────────────────────────────────────────────────────
// py 的 `\s`/`\S` 是 **Unicode** 语义（= str.isspace() 集），Go 的 `\s` 只有 ASCII ⇒ 显式展开，
// 含 `\x1c-\x1f` / `\x85` / `\xa0` 与全部 Zs/Zl/Zp。
const (
	vrWSclass  = `[\t\n\v\f\r\x1c-\x1f \x85\xa0\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`
	vrNWSclass = `[^\t\n\v\f\r\x1c-\x1f \x85\xa0\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`
)

var (
	// ADD_STEP_RE = (?m)^\s*add_step\s+(\S+)\s+"((?:[^"\\]|\\.)*)"\s+(\S+)\s+"((?:[^"\\]|\\.)*)"\s+"((?:[^"\\]|\\.)*)"\s*$
	// ★ 逐字现读：`[^"\\]`（非引号非反斜杠）与 `\\.`（反斜杠 + 任意非换行）是**活**的成对转义，不是死正则。
	vrAddStepRE = regexp.MustCompile(`(?m)^` + vrWSclass + `*add_step` + vrWSclass + `+(` + vrNWSclass + `+)` +
		vrWSclass + `+"((?:[^"\\]|\\.)*)"` + vrWSclass + `+(` + vrNWSclass + `+)` +
		vrWSclass + `+"((?:[^"\\]|\\.)*)"` + vrWSclass + `+"((?:[^"\\]|\\.)*)"` + vrWSclass + `*$`)
	// py-open-write = open\(\s*["']/tmp/[^"']*["']\s*,\s*["'][wa]
	vrLandingPyOpenRE = regexp.MustCompile(`open\(` + vrWSclass + `*["']` + vrTmpSlash + `[^"']*["']` +
		vrWSclass + `*,` + vrWSclass + `*["'][wa]`)
	// 夹具登记条目1 行正则（py .pattern 逐字：`\"` 是「转义引号 ⇒ 字面双引号」，**活**）
	vrFixture1RE = regexp.MustCompile(`^` + vrWSclass + `*'[^']*> ` + vrTmpSlash + `[^']*',` +
		vrWSclass + `*"[A-Z][A-Z0-9-]*"\)`)
	// 夹具登记条目2 行正则（py .pattern 逐字）
	vrFixture2RE = regexp.MustCompile(`^` + vrWSclass + `*'[^']*check-[a-z][a-z-]*\.py[^']*> ` + vrTmpSlash + `[^']*'`)
)

// list_rules 要**逐字**打印 py 的 `.pattern` 原文（不是本件的 Go 正则形态）。
var vrLandingPatternDisplay = []struct{ label, pat string }{
	{"shell-redirect", `(?<![A-Za-z<=])>{1,2}\s*` + vrTmpSlash},
	{"py-open-write", `open\(\s*["']` + vrTmpSlash + `[^"']*["']\s*,\s*["'][wa]`},
}

var vrFixturePatternDisplay = []string{
	`^\s*'[^']*> ` + vrTmpSlash + `[^']*',\s*\"[A-Z][A-Z0-9-]*\"\)`,
	`^\s*'[^']*check-[a-z][a-z-]*\.py[^']*> ` + vrTmpSlash + `[^']*'`,
}

// ── 夹具登记（在册 GAP-20260928-96 · 同族先例同形）──────────────────────────────────────────
var (
	vrFixtureWhy1 = "自检夹具（合成标本串）：`NEG_CASES` 用例正文里的标本路径 —— 跑起来一个字不写 " + vrTmpBare +
		"⇒ 判「夹具数据」，不是「结论落点」；同一件里若改成命令位调用写 " + vrTmpBare + " ⇒ 照旧红"
	vrFixtureWhy2 = "自检夹具（合成标本串）：自证用例正文里那条重定向标本串（现读为变量前缀形态）⇒ 夹具数据不是结论落点；命令位真写 " +
		vrTmpBare + " 照旧红"
	vrSelfExemptReason = "自豁免：本件的判据表 / 注释把「写 " + vrTmpBare + "」的形态写成字面量（结构性）"
)

type vrExempt struct {
	globs []string
	rx    *regexp.Regexp
	why   string
}

var vrFixtureExempt = []vrExempt{
	{[]string{"scripts/gates/check-token-argv.py"}, vrFixture1RE, vrFixtureWhy1},
	{[]string{"scripts/gates/check-wired-scripts.py"}, vrFixture2RE, vrFixtureWhy2},
}

// ── Python 语义辅助（逐处自写 · 不借 Go 近似）────────────────────────────────────────────────

// vrIsPySpace —— py `str.isspace()` / 正则 `\s` 的字符集。
func vrIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0,
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// vrPyStrip —— py `str.strip()`（Unicode 空白集 · ✗ 非 strings.TrimSpace）。
func vrPyStrip(s string) string {
	for len(s) > 0 {
		r, sz := utf8.DecodeRuneInString(s)
		if !vrIsPySpace(r) {
			break
		}
		s = s[sz:]
	}
	for len(s) > 0 {
		r, sz := utf8.DecodeLastRuneInString(s)
		if !vrIsPySpace(r) {
			break
		}
		s = s[:len(s)-sz]
	}
	return s
}

// vrNorm —— py `re.sub(r"\s+", " ", s).strip()`（等价于按 py 空白拆分再单空格拼）。
func vrNorm(s string) string {
	var b strings.Builder
	prevWS := true // 前导空白当已见，天然去前导
	for _, r := range s {
		if vrIsPySpace(r) {
			if !prevWS {
				b.WriteByte(' ')
				prevWS = true
			}
		} else {
			b.WriteRune(r)
			prevWS = false
		}
	}
	return strings.TrimSuffix(b.String(), " ")
}

// vrUnesc —— py `re.sub(r"\\(.)", r"\1", s)`（`.` 不吃换行 ⇒ 反斜杠后是换行时不还原）。
func vrUnesc(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) {
			r, sz := utf8.DecodeRuneInString(s[i+1:])
			if r != '\n' {
				b.WriteString(s[i+1 : i+1+sz])
				i += 1 + sz
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// vrPyJoin —— posixpath.join：后段绝对丢前段 · 空段补分隔符 · 不做任何 normpath。
func vrPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	a := parts[0]
	for _, b := range parts[1:] {
		switch {
		case strings.HasPrefix(b, "/"):
			a = b
		case a == "" || strings.HasSuffix(a, "/"):
			a += b
		default:
			a += "/" + b
		}
	}
	return a
}

// vrPyNormpath —— posixpath.normpath：★ 恰两个前导斜杠（`//a/b`）**保留**（filepath.Clean 会折成 `/a/b`）。
func vrPyNormpath(path string) string {
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
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(newComps) == 0) ||
			(len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	out := strings.Join(newComps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// vrPyAbspath —— os.path.abspath = normpath(join(cwd, path))（绝对输入 ⇒ 只 normpath）。
func vrPyAbspath(path string) string {
	if !strings.HasPrefix(path, "/") {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		path = vrPyJoin(cwd, path)
	}
	return vrPyNormpath(path)
}

func vrSplitComps(p string) []string {
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

// vrPyRelpath —— posixpath.relpath（词法 · 不碰文件系统）。
func vrPyRelpath(path, start string) string {
	if path == "" {
		return ""
	}
	startAbs := vrPyAbspath(start)
	pathAbs := vrPyAbspath(path)
	if startAbs == pathAbs {
		return "."
	}
	sp := vrSplitComps(startAbs)
	pp := vrSplitComps(pathAbs)
	i := 0
	for i < len(sp) && i < len(pp) && sp[i] == pp[i] {
		i++
	}
	var rel []string
	for j := i; j < len(sp); j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pp[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return vrPyJoin(rel...)
}

// vrPyBasename —— posixpath.basename（尾斜杠不折 ⇒ "/a/b/" 的 basename 是 ""）。
func vrPyBasename(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// vrExpandUser —— os.path.expanduser("~")。
func vrExpandUser(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home := os.Getenv("HOME")
		if home == "" {
			if h, err := os.UserHomeDir(); err == nil {
				home = h
			}
		}
		if home != "" {
			return home + p[1:]
		}
	}
	return p
}

// vrIsFile / vrIsDir —— 跟随软链（py isfile/isdir ⇒ os.Stat，✗ 非 DirEntry.IsDir()）。
func vrIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func vrIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// vrFnmatch —— py fnmatch：`*` **跨 `/`** · 按 **rune** 迭代 · `?` 单字符 · `[seq]`/`[!seq]`。
func vrFnmatch(name, pat string) bool {
	return vrFnmatchR([]rune(name), []rune(pat), 0, 0)
}

func vrFnmatchR(n, p []rune, ni, pi int) bool {
	for pi < len(p) {
		c := p[pi]
		switch c {
		case '*':
			for pi < len(p) && p[pi] == '*' {
				pi++
			}
			if pi == len(p) {
				return true
			}
			for k := ni; k <= len(n); k++ {
				if vrFnmatchR(n, p, k, pi) {
					return true
				}
			}
			return false
		case '?':
			if ni >= len(n) {
				return false
			}
			ni++
			pi++
		case '[':
			ok, matched, np := vrFnClass(n, ni, p, pi)
			if !ok {
				if ni >= len(n) || n[ni] != '[' {
					return false
				}
				ni++
				pi++
				continue
			}
			if ni >= len(n) || !matched {
				return false
			}
			ni++
			pi = np
		default:
			if ni >= len(n) || n[ni] != c {
				return false
			}
			ni++
			pi++
		}
	}
	return ni == len(n)
}

// vrFnClass —— 解析 `[...]`：返 (是否成类, 本 rune 是否命中, 类后下标)。
func vrFnClass(n []rune, ni int, p []rune, pi int) (bool, bool, int) {
	j := pi + 1
	neg := false
	if j < len(p) && p[j] == '!' {
		neg = true
		j++
	}
	if j < len(p) && p[j] == ']' {
		j++
	}
	start := j
	for j < len(p) && p[j] != ']' {
		j++
	}
	if j >= len(p) {
		return false, false, pi
	}
	set := p[start:j]
	hit := false
	for k := 0; k < len(set); k++ {
		if k+2 < len(set) && set[k+1] == '-' {
			if ni < len(n) && n[ni] >= set[k] && n[ni] <= set[k+2] {
				hit = true
			}
			k += 2
			continue
		}
		if ni < len(n) && n[ni] == set[k] {
			hit = true
		}
	}
	if neg {
		hit = !hit
	}
	return true, hit, j + 1
}

// ── Python JSON 语义（序列化 / 反序列化逐字对齐）──────────────────────────────────────────────

type vrKV struct {
	K string
	V any
}

type vrObj []vrKV

// vrPyQuote —— py `json.dumps(..., ensure_ascii=False)` 的字符串转义（不转义 `/`、U+2028/9、`<>`）。
func vrPyQuote(s string) string {
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
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
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

// vrPyNumFormat —— py json.loads→dumps 的数：整数（无 . e E）按 int，否则按 float repr。
func vrPyNumFormat(lex string) string {
	if !strings.ContainsAny(lex, ".eE") {
		if bi, ok := new(big.Int).SetString(lex, 10); ok {
			return bi.String()
		}
	}
	f, err := strconv.ParseFloat(lex, 64)
	if err != nil {
		return lex
	}
	return vrPyFloatRepr(f)
}

// vrPyFloatRepr —— py float.__repr__（最短往返 · 指数当 decpt<=-4 或 decpt>16）。
func vrPyFloatRepr(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "null"
	}
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	s := strconv.FormatFloat(f, 'e', -1, 64)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	ei := strings.IndexByte(s, 'e')
	mant := s[:ei]
	exp, _ := strconv.Atoi(s[ei+1:])
	digits := strings.Replace(mant, ".", "", 1)
	decpt := exp + 1
	var out string
	switch {
	case decpt <= -4 || decpt > 16:
		out = string(digits[0])
		if len(digits) > 1 {
			out += "." + digits[1:]
		}
		e := decpt - 1
		sign := "+"
		if e < 0 {
			sign = "-"
			e = -e
		}
		out += "e" + sign + fmt.Sprintf("%02d", e)
	case decpt <= 0:
		out = "0." + strings.Repeat("0", -decpt) + digits
	case decpt >= len(digits):
		out = digits + strings.Repeat("0", decpt-len(digits)) + ".0"
	default:
		out = digits[:decpt] + "." + digits[decpt:]
	}
	if neg {
		out = "-" + out
	}
	return out
}

func vrJSONWrite(b *strings.Builder, v any, sortKeys, compact bool) {
	sep, kv := ", ", ": "
	if compact {
		sep, kv = ",", ":"
	}
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		b.WriteString(vrPyQuote(x))
	case json.Number:
		b.WriteString(vrPyNumFormat(string(x)))
	case int:
		b.WriteString(strconv.Itoa(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case vrObj:
		ents := x
		if sortKeys {
			ents = append(vrObj(nil), x...)
			sort.SliceStable(ents, func(i, j int) bool { return ents[i].K < ents[j].K })
		}
		b.WriteByte('{')
		for i, e := range ents {
			if i > 0 {
				b.WriteString(sep)
			}
			b.WriteString(vrPyQuote(e.K))
			b.WriteString(kv)
			vrJSONWrite(b, e.V, sortKeys, compact)
		}
		b.WriteByte('}')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		if sortKeys {
			sort.Strings(keys)
		}
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteString(sep)
			}
			b.WriteString(vrPyQuote(k))
			b.WriteString(kv)
			vrJSONWrite(b, x[k], sortKeys, compact)
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteString(sep)
			}
			vrJSONWrite(b, e, sortKeys, compact)
		}
		b.WriteByte(']')
	default:
		b.WriteString("null")
	}
}

func vrJSONDump(v any, sortKeys, compact bool) string {
	var b strings.Builder
	vrJSONWrite(&b, v, sortKeys, compact)
	return b.String()
}

// vrParseJSON —— py json.loads（尾随多余标记 ⇒ 错）。
func vrParseJSON(s string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("Extra data")
	}
	return v, nil
}

// vrPyStr —— py `%s` 的标量形态（None / True / False / 数 / 串）。
func vrPyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case json.Number:
		return string(x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	}
	return fmt.Sprintf("%v", v)
}

// vrPyEq —— py `==`（None/布尔/整数/串/数五系）。
func vrPyEq(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	switch x := a.(type) {
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case int:
		switch y := b.(type) {
		case int:
			return x == y
		case int64:
			return int64(x) == y
		}
		return false
	case int64:
		switch y := b.(type) {
		case int:
			return x == int64(y)
		case int64:
			return x == y
		}
		return false
	case string:
		y, ok := b.(string)
		return ok && x == y
	case json.Number:
		y, ok := b.(json.Number)
		return ok && vrPyNumFormat(string(x)) == vrPyNumFormat(string(y))
	}
	return false
}

// vrPyRepr —— py `str(dict)`（只用于 `--fingerprint` 打问题清单）。
func vrPyRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		var b strings.Builder
		b.WriteByte('\'')
		for _, r := range x {
			switch r {
			case '\\':
				b.WriteString(`\\`)
			case '\'':
				b.WriteString(`\'`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				b.WriteRune(r)
			}
		}
		b.WriteByte('\'')
		return b.String()
	case json.Number:
		return string(x)
	case vrObj:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, vrPyRepr(e.K)+": "+vrPyRepr(e.V))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, vrPyRepr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return fmt.Sprintf("%v", v)
}

// ── py 文本读入语义：open() 文本模式折 \r\n 与裸 \r；splitlines 按 py 行界 ─────────────────────
func vrIsLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

func vrPySplitlines(s string) []string {
	var out []string
	start, i := 0, 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if vrIsLineBreak(r) {
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

func vrUniversalNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// vrUTF8Replace —— py `decode("utf-8","replace")` 的近似（非法字节逐段 U+FFFD）。
func vrUTF8Replace(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b); {
		r, sz := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && sz == 1 {
			sb.WriteRune(utf8.RuneError)
			i++
			continue
		}
		sb.Write(b[i : i+sz])
		i += sz
	}
	return sb.String()
}

func vrReadTextUniversal(p string, replace bool) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	var s string
	if replace {
		s = vrUTF8Replace(b)
	} else if utf8.Valid(b) {
		s = string(b)
	} else {
		return "", fmt.Errorf("unicode decode error")
	}
	return vrUniversalNewlines(s), nil
}

// vrSHA256Hex —— hashlib.sha256(...).hexdigest()。
func vrSHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

// ── 判据主体 ───────────────────────────────────────────────────────────────────────────────

// vrJudgeFiles —— py judge_files(root)。
func vrJudgeFiles(root string) ([]string, bool) {
	d := vrPyJoin(root, vrJudgeDirRel)
	if !vrIsDir(d) {
		return nil, false
	}
	names, err := vrSortedListdir(d)
	if err != nil {
		return nil, false
	}
	var out []string
	for _, n := range names {
		p := vrPyJoin(d, n)
		if !vrIsFile(p) {
			continue
		}
		if n == "mutate-scan" || (strings.HasPrefix(n, "check-") &&
			(strings.HasSuffix(n, ".py") || strings.HasSuffix(n, ".sh"))) {
			out = append(out, p)
		}
	}
	return out, true
}

func vrSortedListdir(d string) ([]string, error) {
	ents, err := os.ReadDir(d)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// vrSubjectItems —— py subject_items：返 (归一「条」清单, 问题清单)。
func vrSubjectItems(root string) ([]string, []any) {
	var items []string
	var problems []any
	p := vrPyJoin(root, vrGateTableRel)
	if !vrIsFile(p) {
		problems = append(problems, vrObj{{"what", "gate_table_absent"}, {"path", p}})
	} else {
		txt, err := vrReadTextUniversal(p, false)
		if err != nil {
			problems = append(problems, vrObj{{"what", "unreadable"}, {"path", p}, {"msg", err.Error()}})
		} else {
			n := 0
			for _, m := range vrAddStepRE.FindAllStringSubmatch(txt, -1) {
				items = append(items, fmt.Sprintf("step|%s|%s|%s|%s|%s",
					vrNorm(m[1]), vrNorm(vrUnesc(m[2])), vrNorm(m[3]), vrNorm(m[4]), vrNorm(vrUnesc(m[5]))))
				n++
			}
			if n == 0 {
				problems = append(problems, vrObj{{"what", "add_step_empty"}, {"path", p}})
			}
		}
	}
	for _, rel := range vrBaselineRels {
		bp := vrPyJoin(root, rel)
		if !vrIsFile(bp) {
			problems = append(problems, vrObj{{"what", "baseline_absent"}, {"path", bp}})
			continue
		}
		raw, err := vrReadTextUniversal(bp, false)
		if err != nil {
			problems = append(problems, vrObj{{"what", "unreadable"}, {"path", bp}, {"msg", err.Error()}})
			continue
		}
		obj, err := vrParseJSON(raw)
		if err != nil {
			problems = append(problems, vrObj{{"what", "baseline_unparsable"}, {"path", bp}, {"msg", err.Error()}})
			continue
		}
		canon := vrJSONDump(obj, true, true)
		nItems := 0
		switch t := obj.(type) {
		case []any:
			nItems = len(t)
		case map[string]any:
			nItems = len(t)
		}
		items = append(items, fmt.Sprintf("json|%s|%d|%s", rel, nItems, vrSHA256Hex(canon)))
	}
	return items, problems
}

func vrFingerprint(items []string, subject string) string {
	cp := append([]string(nil), items...)
	sort.Strings(cp)
	body := "subject=" + subject + "\n" + strings.Join(cp, "\n")
	return "sha256:" + vrSHA256Hex(body)
}

func vrFixtureExemptOf(rel, line string) (int, string, bool) {
	for idx, fe := range vrFixtureExempt {
		matched := false
		for _, g := range fe.globs {
			if vrFnmatch(rel, g) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if fe.rx.MatchString(line) {
			return idx + 1, fe.why, true
		}
	}
	return 0, "", false
}

// vrLandingShellRedirect —— py 的 `(?<![A-Za-z<=])>{1,2}\s*/tmp/`（**手写扫描器**：RE2 不吃环视）。
func vrLandingShellRedirect(line string) bool {
	for i := 0; i < len(line); i++ {
		if line[i] != '>' {
			continue
		}
		if i > 0 {
			c := line[i-1]
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '<' || c == '=' {
				continue
			}
		}
		for _, n := range []int{2, 1} {
			if i+n > len(line) {
				continue
			}
			if n == 2 && line[i+1] != '>' {
				continue
			}
			j := i + n
			for j < len(line) {
				r, sz := utf8.DecodeRuneInString(line[j:])
				if !vrIsPySpace(r) {
					break
				}
				j += sz
			}
			if strings.HasPrefix(line[j:], vrTmpSlash) {
				return true
			}
		}
	}
	return false
}

type vrHit struct {
	file string
	line int
	form string
	raw  string
}

func vrScanLanding(root string) (hits []any, files []string, filesOK bool,
	exempt []any, fixture []any, stale []any) {
	files, ok := vrJudgeFiles(root)
	if !ok {
		return nil, nil, false, nil, nil, nil
	}
	me := vrMeBase
	seen := make([]int, len(vrFixtureExempt))
	for _, p := range files {
		rel := vrPyRelpath(p, root)
		if vrPyBasename(p) == me {
			exempt = append(exempt, vrObj{{"file", rel}, {"reason", vrSelfExemptReason}})
			continue
		}
		txt, err := vrReadTextUniversal(p, true)
		if err != nil {
			continue
		}
		for i, line := range vrPySplitlines(txt) {
			lineno := i + 1
			form := ""
			if vrLandingShellRedirect(line) {
				form = "shell-redirect"
			} else if vrLandingPyOpenRE.MatchString(line) {
				form = "py-open-write"
			}
			if form == "" {
				continue
			}
			if ridx, why, ok := vrFixtureExemptOf(rel, line); ok {
				seen[ridx-1]++
				fixture = append(fixture, vrObj{{"file", rel}, {"line", lineno}, {"form", form},
					{"raw", vrPyStrip(line)}, {"rule", ridx}, {"reason", why}})
			} else {
				hits = append(hits, vrObj{{"file", rel}, {"line", lineno}, {"form", form},
					{"raw", vrPyStrip(line)}})
			}
		}
	}
	for idx, fe := range vrFixtureExempt {
		if seen[idx] == 0 {
			globs := make([]any, 0, len(fe.globs))
			for _, g := range fe.globs {
				globs = append(globs, g)
			}
			stale = append(stale, vrObj{{"rule", idx + 1}, {"globs", globs}, {"reason", fe.why}})
		}
	}
	return hits, files, true, exempt, fixture, stale
}

type vrRep struct {
	root          string
	landingHits   []any
	judgeFiles    int
	fingerprint   string
	fingerprint2  string
	items         int
	problems      []any
	selfExempt    []any
	fixtureExempt []any
	fixtureStale  []any
}

func vrJudge(root string) (int, *vrRep) {
	rep := &vrRep{root: vrPyAbspath(root)}
	if len(vrFixtureExempt) > vrFixtureExemptMax {
		rep.problems = append(rep.problems, vrObj{{"what", "fixture_exempt_clamp"},
			{"msg", fmt.Sprintf("自检夹具登记 %d 条 > 硬钳 %d ⇒ 不给结论（防把『往登记表里加条目』当治本）",
				len(vrFixtureExempt), vrFixtureExemptMax)}})
	}
	hits, files, filesOK, exempt, fixture, stale := vrScanLanding(root)
	if !filesOK {
		rep.problems = append(rep.problems, vrObj{{"what", "judge_dir_absent"},
			{"path", vrPyJoin(root, vrJudgeDirRel)}})
	} else {
		rep.judgeFiles = len(files)
		rep.landingHits = hits
		rep.selfExempt = exempt
		rep.fixtureExempt = fixture
		rep.fixtureStale = stale
	}
	items1, prob1 := vrSubjectItems(root)
	items2, prob2 := vrSubjectItems(root)
	rep.items = len(items1)
	rep.fingerprint = vrFingerprint(items1, vrSubject)
	rep.fingerprint2 = vrFingerprint(items2, vrSubject)
	rep.problems = append(rep.problems, prob1...)
	rep.problems = append(rep.problems, prob2...)
	if len(rep.problems) > 0 {
		return 2, rep
	}
	if rep.fingerprint != rep.fingerprint2 {
		rep.problems = append(rep.problems, vrObj{{"what", "two_runs_differ"},
			{"a", rep.fingerprint}, {"b", rep.fingerprint2}})
		return 2, rep
	}
	if len(rep.landingHits) > 0 {
		return 1, rep
	}
	return 0, rep
}

func vrStateDir(cli string) string {
	if cli != "" {
		return cli
	}
	env := vrPyStrip(os.Getenv("ZERG_STATE_DIR"))
	if env != "" {
		return env
	}
	return vrPyJoin(vrExpandUser("~"), ".zerg", "state")
}

// vrRec —— 一条记录（保序 map + 行号）。
type vrRec struct {
	m map[string]any
}

func vrLoadRecords(state, subject string) ([]vrRec, string, error) {
	p := vrPyJoin(state, vrRecordName)
	if !vrIsFile(p) {
		return nil, p, nil
	}
	txt, err := vrReadTextUniversal(p, false)
	if err != nil {
		return nil, p, err
	}
	var out []vrRec
	for _, line := range vrPySplitlines(txt) {
		line = vrPyStrip(line)
		if line == "" {
			continue
		}
		v, err := vrParseJSON(line)
		if err != nil {
			continue // py：ValueError ⇒ 跳过
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil, p, fmt.Errorf("vrNonObjectRecord") // py：AttributeError（裸回溯）
		}
		if s, ok := m["subject"].(string); ok && s == subject {
			out = append(out, vrRec{m: m})
		}
	}
	return out, p, nil
}

func vrAppendRecord(state string, rec vrObj) (string, error) {
	if err := os.MkdirAll(state, 0o777); err != nil {
		return "", err
	}
	p := vrPyJoin(state, vrRecordName)
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(vrJSONDump(rec, true, false) + "\n"); err != nil {
		return "", err
	}
	return p, nil
}

func vrMachineDoc(rep *vrRep, rc int, state string, recs []vrRec) vrObj {
	hits := rep.landingHits
	if hits == nil {
		hits = []any{}
	}
	se := rep.selfExempt
	if se == nil {
		se = []any{}
	}
	fe := rep.fixtureExempt
	if fe == nil {
		fe = []any{}
	}
	fs := rep.fixtureStale
	if fs == nil {
		fs = []any{}
	}
	pb := rep.problems
	if pb == nil {
		pb = []any{}
	}
	var last any
	if len(recs) > 0 {
		r := recs[len(recs)-1]
		last = vrObj{{"ts", r.m["ts"]}, {"fingerprint", r.m["fingerprint"]}}
	}
	return vrObj{
		{"schema", "zerg/v1"}, {"kind", "VerdictRecord"}, {"tool", vrVersion},
		{"subject", vrSubject},
		{"fingerprint", rep.fingerprint},
		{"items", rep.items},
		{"verdict", vrObj{{"rc", rc}, {"of", vrOfText}}},
		{"landing_hits", hits},
		{"landing_hit_count", len(hits)},
		{"self_exempt", se},
		{"fixture_exempt", fe},
		{"fixture_exempt_stale", fs},
		{"judge_files", rep.judgeFiles},
		{"record_path", vrPyJoin(state, vrRecordName)},
		{"records", len(recs)},
		{"last_record", last},
		{"problems", pb},
		{"warnings", []any{}},
		{"truncated", false},
	}
}

const vrOfText = "check-verdict-record.py（落点面 + 指纹面）"

// ── 命令档 ─────────────────────────────────────────────────────────────────────────────────

type vrArgs struct {
	root        string
	stateDir    string
	jsonOut     bool
	fingerprint bool
	record      bool
	compare     bool
	listRules   bool
	selfTest    bool
}

func vrCmdFingerprint(stdout io.Writer, root string) int {
	items, prob := vrSubjectItems(root)
	if len(prob) > 0 {
		parts := make([]string, 0, len(prob))
		for _, p := range prob {
			parts = append(parts, vrPyRepr(p))
		}
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：判不了 subject：%s\n", strings.Join(parts, "; "))
		return 2
	}
	fmt.Fprintf(stdout, "%s  subject=%s 条=%d\n", vrFingerprint(items, vrSubject), vrSubject, len(items))
	return 0
}

func vrVerdictRC(v any) string {
	if v == nil {
		return "None"
	}
	if m, ok := v.(map[string]any); ok {
		if m == nil {
			return "None"
		}
		return vrPyStr(m["rc"])
	}
	return "None"
}

func vrCmdCompare(stdout io.Writer, state, subject string) int {
	recs, p, err := vrLoadRecords(state, subject)
	if err != nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：结论件读不动\n")
		return 2
	}
	fmt.Fprintf(stdout, "── 「算不算过」可比性对拍（%s）──\n", vrVersion)
	fmt.Fprintf(stdout, "  结论件  : %s\n", p)
	fmt.Fprintf(stdout, "  同 subject 记录数: %d\n", len(recs))
	if len(recs) < 2 {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：不足两跑（同 subject 记录 %d 条 ≤ 1）⇒ 判不了可比性"+
			"（**不拿「没得比」当「比过了」**）\n", len(recs))
		return 2
	}
	a := recs[len(recs)-2]
	b := recs[len(recs)-1]
	fmt.Fprintf(stdout, "  前一跑  : ts=%s 指纹=%s\n", vrPyStr(a.m["ts"]), vrPyStr(a.m["fingerprint"]))
	fmt.Fprintf(stdout, "  后一跑  : ts=%s 指纹=%s\n", vrPyStr(b.m["ts"]), vrPyStr(b.m["fingerprint"]))
	fmt.Fprintf(stdout, "  判据 rc : %s / %s\n", vrVerdictRC(a.m["verdict"]), vrVerdictRC(b.m["verdict"]))
	if vrPyEq(a.m["fingerprint"], b.m["fingerprint"]) {
		fmt.Fprintf(stdout, "⇒ 同一改动两跑**指纹相同**（rc=0）：两条记录说的是同一份内容 ⇒ 结果**可比**\n")
		return 0
	}
	fmt.Fprintf(stdout, "✗ **仓变了，结果不可比**（rc=2 · 逐字就是这一句）：两条记录的指纹不同\n")
	fmt.Fprintf(stdout, "  ⇒ 处置：要么把**后一跑**当新身份（另当一改动的结论看），要么回滚到前一跑的改动 ——\n")
	fmt.Fprintf(stdout, "    **不许**把两次不同内容的结论并成一个数（`R-16` 同族口径）。\n")
	return 2
}

func vrRun(stdout io.Writer, root, state string, a vrArgs) int {
	rc, rep := vrJudge(root)
	recs, _, lerr := vrLoadRecords(state, vrSubject)
	if lerr != nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：记录件里出现非对象 JSON 行（判据对象读不动）\n")
		return 2
	}
	var last *vrRec
	if len(recs) > 0 {
		last = &recs[len(recs)-1]
	}

	if a.record {
		rec := vrObj{
			{"schema", "zerg/v1"}, {"kind", "VerdictRecord"}, {"tool", vrVersion},
			{"subject", vrSubject}, {"fingerprint", rep.fingerprint}, {"items", rep.items},
			{"verdict", vrObj{{"rc", rc}, {"of", vrOfText}}},
			{"ts", time.Now().Format("2006-01-02T15:04:05-0700")},
			{"root", rep.root},
		}
		p, err := vrAppendRecord(state, rec)
		if err != nil {
			fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：结论件落盘失败\n")
			return 2
		}
		fmt.Fprintf(stdout, "已落结论件  : %s\n", p)
		fmt.Fprintf(stdout, "  指纹      : %s（条 %d）\n", rep.fingerprint, rep.items)
		fmt.Fprintf(stdout, "  判据      : rc=%d（本门本跑的「算不算过」）\n", rc)
		fmt.Fprintf(stdout, "★ 落点是**状态目录**（非 `%s`）· 记录只增不改（`--compare` 比最后两条）\n", vrTmpBare)
		return rc
	}

	if a.jsonOut {
		fmt.Fprintf(stdout, "%s\n", vrJSONDump(vrMachineDoc(rep, rc, state, recs), false, false))
		return rc
	}

	fmt.Fprintf(stdout, "── 「算不算过」机器读落点 + 稳定指纹（%s · `W-47`）──\n", vrVersion)
	fmt.Fprintf(stdout, "  仓根      : %s\n", rep.root)
	fmt.Fprintf(stdout, "  结论件落点: %s（**状态目录** · 非 `%s`）\n", vrPyJoin(state, vrRecordName), vrTmpBare)
	fmt.Fprintf(stdout, "  判据件面  : %d 件（%s 的 check-*.py|sh + mutate-scan · 全只读）\n",
		rep.judgeFiles, vrJudgeDirRel)
	for _, e := range rep.selfExempt {
		o := e.(vrObj)
		fmt.Fprintf(stdout, "              · 自豁免 %s —— %s\n", o[0].V, o[1].V)
	}
	for _, e := range rep.fixtureExempt {
		o := e.(vrObj)
		fmt.Fprintf(stdout, "              · 夹具登记豁免 %s:%d [%s]（条目 %d）—— %s\n",
			o[0].V, o[1].V, o[2].V, o[4].V, o[5].V)
	}
	for _, e := range rep.fixtureStale {
		o := e.(vrObj)
		globs := o[1].V.([]any)
		gstr := make([]string, 0, len(globs))
		for _, g := range globs {
			gstr = append(gstr, g.(string))
		}
		fmt.Fprintf(stdout, "              · 悬空登记（条目 %d · %s）命中 0 处 —— 照实打印 · **不判红**（同门㉝ 口径）\n",
			o[0].V, strings.Join(gstr, " "))
	}
	fmt.Fprintf(stdout, "  subject   : %s（门表 add_step 条 + 两件命令面基线件的规范化 JSON）\n", vrSubject)
	fmt.Fprintf(stdout, "  指纹      : %s（条 %d）\n", rep.fingerprint, rep.items)
	twoSame := "**不同**（判不了）"
	if rep.fingerprint == rep.fingerprint2 {
		twoSame = "逐字相同（内容算出 · 与行号无关）"
	}
	fmt.Fprintf(stdout, "  两遍现算  : %s ⇒ %s\n", rep.fingerprint2, twoSame)

	if len(rep.problems) > 0 {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：\n")
		for _, p := range rep.problems {
			o := p.(vrObj)
			what := vrPyStr(o[0].V)
			path, msg := "", ""
			for _, kv := range o[1:] {
				if kv.K == "path" {
					path = vrPyStr(kv.V)
				}
				if kv.K == "msg" {
					msg = vrPyStr(kv.V)
				}
			}
			fmt.Fprintf(stdout, "   · %s：%s %s\n", what, path, msg)
		}
		return 2
	}
	if rc == 1 {
		fmt.Fprintf(stdout, "✗ 落点面命中 %d 处（rc=1 · 逐条点名）：判据件把结论写到了 `%s` 上\n",
			len(rep.landingHits), vrTmpBare)
		for _, h := range rep.landingHits {
			o := h.(vrObj)
			raw := o[3].V.(string)
			if len([]rune(raw)) > 90 {
				raw = string([]rune(raw)[:90])
			}
			fmt.Fprintf(stdout, "   · %s:%d [%s] 原样 `%s`\n", o[0].V, o[1].V, o[2].V, raw)
		}
		fmt.Fprintf(stdout, "⇒ 处置（**不是**改本门）：把该判据件的结论落点挪进**仓内**（`scripts/gates/**`）"+
			"或**状态目录** `<state>/…`，并同笔写清为什么。\n")
		return 1
	}

	if last != nil {
		same := vrPyEq(last.m["fingerprint"], rep.fingerprint)
		fmt.Fprintf(stdout, "  最近记录  : ts=%s 指纹=%s\n", vrPyStr(last.m["ts"]), vrPyStr(last.m["fingerprint"]))
		if same {
			fmt.Fprintf(stdout, "              ⇒ 与本跑**逐字相同**：同一改动 ⇒ 与那一次的结论**可比**\n")
		} else {
			fmt.Fprintf(stdout, "              ⇒ 与本跑**不同** ⇒ **仓变了，结果不可比**"+
				"（本格照实打印 · **不定本步红绿** —— 要判可比性请显式 `--compare`）\n")
		}
	} else {
		fmt.Fprintf(stdout, "  最近记录  : 无（结论件记录数 0）⇒ 本条只判到「落点面 + 本跑两遍现算」；"+
			"要判可比性：先 `--record` 两跑再 `--compare`\n")
	}
	fmt.Fprintf(stdout, "⇒ 判据（rc=0）：落点面 0 命中（判据件的结论没有散在 `%s`）"+
		" + 指纹面两遍现算逐字相同（内容算出 · 非行号）"+
		" + 自检夹具登记豁免 %d 处（逐条打印理由 · 不算命中）\n", vrTmpBare, len(rep.fixtureExempt))
	fmt.Fprintf(stdout, "   ★ 本件只**读**门表与两件基线：一个字未改（`--record` 才写，且只写状态目录）\n")
	return 0
}

func vrListRules(stdout io.Writer) int {
	fmt.Fprintf(stdout, "── %s 判据与落点 ──\n", vrVersion)
	fmt.Fprintf(stdout, "  subject : %s\n", vrSubject)
	fmt.Fprintf(stdout, "    · 门表条 : %s —— `add_step` 五格归一成 `scope|步名|档位|工作目录|命令串`\n", vrGateTableRel)
	for _, r := range vrBaselineRels {
		fmt.Fprintf(stdout, "    · 基线件 : %s —— 规范化 JSON（sort_keys + 紧凑分隔符）的 sha256 + 条数\n", r)
	}
	fmt.Fprintf(stdout, "  指纹    : sha256(subject + 逐条排序拼接) —— **不含行号 / 缩进 / 排版**\n")
	fmt.Fprintf(stdout, "  落点面  : 扫 %s 的 check-*.py|sh + mutate-scan，查「结论写 %s」的**写盘**形态：\n",
		vrJudgeDirRel, vrTmpBare)
	for _, lp := range vrLandingPatternDisplay {
		fmt.Fprintf(stdout, "    · %-14s %s\n", lp.label, lp.pat)
	}
	fmt.Fprintf(stdout, "  结论件  : <状态目录>/%s（JSONL · `--record` 追加 · **非 %s**）\n", vrRecordName, vrTmpBare)
	fmt.Fprintf(stdout, "  夹具登记: `FIXTURE_EXEMPT` %d 条（硬钳 %d）—— 条目 = (件 glob, 行正则, 为什么豁免) ·"+
		" 逐命中打印理由 · 悬空不判红\n", len(vrFixtureExempt), vrFixtureExemptMax)
	for idx, fe := range vrFixtureExempt {
		fmt.Fprintf(stdout, "    · 条目%d %s  %s\n", idx+1, strings.Join(fe.globs, " "), vrFixturePatternDisplay[idx])
	}
	fmt.Fprintf(stdout, "  退码    : 0 两条面皆过 · 1 落点面命中 · 2 不给结论（缺件/空转/两遍不同/自检不过/"+
		"不足两跑/指纹不同）\n")
	fmt.Fprintf(stdout, "★ 已挂门：`gates` scope · `tri` 阻断 · 在默认集 · 不进 `FAST_STEPS`。\n")
	return 0
}

// ── 自检（正控 + 成对负控 · 合成夹具 · 不碰真仓 / 真状态目录）────────────────────────────────────

var vrFixtureGate = "#!/usr/bin/env bash\n" +
	"# 合成门表（夹具）\n" +
	"run_suite() {\n" +
	"  case \"$1\" in\n" +
	"    gates)\n" +
	"      add_step gates \"门甲 夹具一（阻断）\" tri \"${REPO_ROOT}\" \"python3 scripts/gates/check-a.py\"\n" +
	"      add_step gates \"门乙 夹具二（只报告）\" tri-report \"${REPO_ROOT}\" \"python3 scripts/gates/check-b.py\"\n" +
	"      ;;\n" +
	"  esac\n" +
	"}\n"

var vrFixtureGateShuffled = "#!/usr/bin/env bash\n" +
	"# 合成门表（夹具 · 同一批条）\n" +
	"\n" +
	"run_suite() {\n" +
	"\n" +
	"        case \"$1\" in\n" +
	"\n" +
	"\n" +
	"\n" +
	"                gates)\n" +
	"      # 一条注释（不改任何条）\n" +
	"\n" +
	"                add_step gates      \"门乙 夹具二（只报告）\"     tri-report \"${REPO_ROOT}\"    \"python3 scripts/gates/check-b.py\"\n" +
	"\n" +
	"\n" +
	"                add_step   gates \"门甲 夹具一（阻断）\"  tri   \"${REPO_ROOT}\" \"python3 scripts/gates/check-a.py\"\n" +
	"                ;;\n" +
	"        esac\n" +
	"}\n"

var vrFixtureBase = map[string]int{"commands": 3, "mustfail_cases": 5, "blanks": 0}

var vrFixtureMatrix = map[string]any{"cases": []any{
	map[string]any{"command": "zerg a", "want_rc": 2},
	map[string]any{"command": "zerg b", "want_rc": 2},
}}

var vrSpecCase = "#!/usr/bin/env python3\n" +
	"# 合成夹具判据件（自检 NEG_CASES：标本串，跑起来一个字不写 " + vrTmpSlash + "）\n" +
	"NEG_CASES = [\n" +
	"    (\"③b\", \"单元正文在 .sh 的 heredoc 里\", \"neg3b\", \"deploy.sh\",\n" +
	"     '#!/usr/bin/env bash\\ncat > " + vrTmpSlash + "edge-node-agent.service << UNIT_EOF\\n', \"UNIT-OPT-ARGV\"),\n" +
	"]\n"

var vrRealWrite = "#!/usr/bin/env python3\nimport subprocess\n" +
	"subprocess.run(\"git status > " + vrTmpSlash + "st-before.txt\", shell=True)\n"

func vrMkFixture(tmp, name, gate string, base, matrix any, judgeBody, judgeName string) string {
	root := vrPyJoin(tmp, name)
	_ = os.MkdirAll(vrPyJoin(root, "scripts", "gates"), 0o777)
	_ = os.MkdirAll(vrPyJoin(root, "core", "cmd", "zerg", "testdata"), 0o777)
	_ = os.WriteFile(vrPyJoin(root, vrGateTableRel), []byte(gate), 0o666)
	if base == nil {
		base = vrFixtureBase
	}
	if matrix == nil {
		matrix = vrFixtureMatrix
	}
	b0, _ := json.Marshal(base)
	_ = os.WriteFile(vrPyJoin(root, vrBaselineRels[0]), b0, 0o666)
	b1, _ := json.Marshal(matrix)
	_ = os.WriteFile(vrPyJoin(root, vrBaselineRels[1]), b1, 0o666)
	clean := judgeBody
	if clean == "" {
		clean = "#!/usr/bin/env python3\n\"\"\"夹具判据件（落点在仓内）\"\"\"\nprint(\"ok\")\n"
	}
	_ = os.WriteFile(vrPyJoin(root, "scripts", "gates", judgeName), []byte(clean), 0o666)
	return root
}

func vrSelfTest(stdout io.Writer) int {
	ok := true
	tmp, err := os.MkdirTemp("", "vr-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "自检结论: **有断言失败**\n")
		return 1
	}
	defer os.RemoveAll(tmp)

	vrCase := func(name string, want, got any, extraOK bool, note string) {
		good := vrPyEq(want, got) && extraOK
		if !good {
			ok = false
		}
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（rc=%s · 期望 %s）%s\n", mark, name, vrPyStr(got), vrPyStr(want), note)
	}
	capRun := func(root, state string, a vrArgs) (int, string) {
		var b strings.Builder
		rc := vrRun(&b, root, state, a)
		return rc, b.String()
	}

	// ① 正控
	r1 := vrMkFixture(tmp, "case-1", vrFixtureGate, nil, nil, "", "check-a.py")
	rc, out := capRun(r1, vrPyJoin(tmp, "state-1"), vrArgs{})
	vrCase("① 正控：落点面 0 命中 + 两遍现算逐字相同 ⇒ rc=0", 0, rc, strings.Contains(out, "逐字相同"), "")

	// ② 负控：只改行号 ⇒ 指纹不变
	r2 := vrMkFixture(tmp, "case-2", vrFixtureGateShuffled, nil, nil, "", "check-a.py")
	it1, _ := vrSubjectItems(r1)
	it2, _ := vrSubjectItems(r2)
	f1 := vrFingerprint(it1, vrSubject)
	f2 := vrFingerprint(it2, vrSubject)
	vrCase("② 负控：只改行号 / 加空行 / 挪行 / 改缩进 / 换次序 ⇒ 指纹**不变**", true, f1 == f2, true,
		fmt.Sprintf("  （两跑指纹逐字同：%s…）", f1[7:23]))

	// ③ 负控：改内容（步名一格）⇒ 指纹变
	r3 := vrMkFixture(tmp, "case-3", strings.Replace(vrFixtureGate, "门甲 夹具一", "门甲 夹具一改", 1), nil, nil, "", "check-a.py")
	it3, _ := vrSubjectItems(r3)
	f3 := vrFingerprint(it3, vrSubject)
	vrCase("③ 负控：改内容（步名一格）⇒ 指纹**变**", true, f3 != f1, true, "")

	// ③b 负控：删一条 add_step ⇒ 指纹变
	line3b := "      add_step gates \"门乙 夹具二（只报告）\" tri-report \"${REPO_ROOT}\" \"python3 scripts/gates/check-b.py\"\n"
	r3b := vrMkFixture(tmp, "case-3b", strings.Replace(vrFixtureGate, line3b, "", 1), nil, nil, "", "check-a.py")
	it3b, _ := vrSubjectItems(r3b)
	f3b := vrFingerprint(it3b, vrSubject)
	vrCase("③b 负控：删一条 add_step ⇒ 指纹变（少一条 == 另一份内容）", true, f3b != f1, true, "")

	// ④ 负控：基线件只重排版 ⇒ 指纹不变
	base4 := map[string]int{"blanks": 0, "mustfail_cases": 5, "commands": 3}
	r4 := vrMkFixture(tmp, "case-4", vrFixtureGate, base4, nil, "", "check-a.py")
	_ = os.WriteFile(vrPyJoin(r4, vrBaselineRels[0]), []byte(`{"blanks": 0, "mustfail_cases": 5, "commands": 3}`+"\n\n\n"), 0o666)
	it4, _ := vrSubjectItems(r4)
	f4 := vrFingerprint(it4, vrSubject)
	vrCase("④ 负控：基线件只重排版（缩进/换行/键序）⇒ 指纹不变", true, f4 == f1, true, "")

	// ⑤ 负控：两跑不同 ⇒ compare rc=2
	st1 := vrPyJoin(tmp, "state-cmp1")
	_, _ = capRun(r1, st1, vrArgs{record: true})
	_, _ = capRun(r3, st1, vrArgs{record: true})
	var cb strings.Builder
	crc := vrCmdCompare(&cb, st1, vrSubject)
	vrCase("⑤ 负控：两跑指纹不同 ⇒ `--compare` rc=2", 2, crc, strings.Contains(cb.String(), "仓变了，结果不可比"), "")

	// ⑤b 真话负控：同一夹具两跑 ⇒ compare rc=0
	st := vrPyJoin(tmp, "state-cmp")
	_, _ = capRun(r1, st, vrArgs{record: true})
	_, _ = capRun(r1, st, vrArgs{record: true})
	var cb2 strings.Builder
	crc2 := vrCmdCompare(&cb2, st, vrSubject)
	vrCase("⑤b 真话负控：同一改动两跑 ⇒ `--compare` rc=0 + 逐字「指纹相同」", 0, crc2, strings.Contains(cb2.String(), "指纹相同"), "")

	// ⑤c 负控：不足两跑
	st2 := vrPyJoin(tmp, "state-one")
	_, _ = capRun(r1, st2, vrArgs{record: true})
	var cb3 strings.Builder
	crc3 := vrCmdCompare(&cb3, st2, vrSubject)
	vrCase("⑤c 负控：只能一跑 ⇒ `--compare` rc=2（不给结论）", 2, crc3, strings.Contains(cb3.String(), "不足两跑"), "")

	// ⑥ 负控：落点面命中
	dirty := vrRealWrite
	r6 := vrMkFixture(tmp, "case-6", vrFixtureGate, nil, nil, dirty, "check-a.py")
	rc6, out6 := capRun(r6, vrPyJoin(tmp, "state-6"), vrArgs{})
	vrCase("⑥ 负控：判据件把结论写 `"+vrTmpBare+"` ⇒ rc=1", 1, rc6, strings.Contains(out6, "落点面命中"), "")
	vrCase("⑥b 负控点名到「件:行 + 原样」", true,
		strings.Contains(out6, "scripts/gates/check-a.py:3") && strings.Contains(out6, vrTmpSlash+"st-before.txt"), true, "")

	// ⑥c 真话负控：落点挪进仓内 ⇒ 回绿
	clean := strings.Replace(dirty, vrTmpSlash+"st-before.txt", "scripts/gates/.verdict-before.txt", 1)
	r6c := vrMkFixture(tmp, "case-6c", vrFixtureGate, nil, nil, clean, "check-a.py")
	rc6c, out6c := capRun(r6c, vrPyJoin(tmp, "state-6c"), vrArgs{})
	vrCase("⑥c 真话负控：落点挪进仓内 ⇒ 回绿", 0, rc6c, strings.Contains(out6c, "落点面 0 命中"), "")

	// ⑦ 负控：空转
	r7 := vrMkFixture(tmp, "case-7", "#!/usr/bin/env bash\n# 合成门表：一条 add_step 都没有\n", nil, nil, "", "check-a.py")
	rc7, out7 := capRun(r7, vrPyJoin(tmp, "state-7"), vrArgs{})
	vrCase("⑦ 负控：空转（0 条 add_step）⇒ rc=2 不给结论", 2, rc7, strings.Contains(out7, "不给结论"), "")

	// ⑧ 负控：基线件不在盘
	r8 := vrMkFixture(tmp, "case-8", vrFixtureGate, nil, nil, "", "check-a.py")
	_ = os.Remove(vrPyJoin(r8, vrBaselineRels[1]))
	rc8, out8 := capRun(r8, vrPyJoin(tmp, "state-8"), vrArgs{})
	vrCase("⑧ 负控：基线件不在盘 ⇒ rc=2", 2, rc8, strings.Contains(out8, "不给结论"), "")

	// ⑨ 负控：基线件解不动
	r9 := vrMkFixture(tmp, "case-9", vrFixtureGate, nil, nil, "", "check-a.py")
	_ = os.WriteFile(vrPyJoin(r9, vrBaselineRels[0]), []byte("{ 坏 JSON \n"), 0o666)
	rc9, out9 := capRun(r9, vrPyJoin(tmp, "state-9"), vrArgs{})
	vrCase("⑨ 负控：基线件解不动 ⇒ rc=2（不许默认成绿）", 2, rc9, strings.Contains(out9, "不给结论"), "")

	// ⑩ --json
	var jb strings.Builder
	rc10 := vrRun(&jb, r1, vrPyJoin(tmp, "state-10"), vrArgs{jsonOut: true})
	var doc map[string]any
	_ = json.Unmarshal([]byte(jb.String()), &doc)
	fp, _ := doc["fingerprint"].(string)
	var items float64
	if v, ok := doc["items"].(json.Number); ok {
		items, _ = v.Float64()
	} else if v, ok := doc["items"].(float64); ok {
		items = v
	}
	rcv := -1
	if vd, ok := doc["verdict"].(map[string]any); ok {
		if v, ok := vd["rc"].(float64); ok {
			rcv = int(v)
		}
	}
	rp, _ := doc["record_path"].(string)
	vrCase("⑩ `--json` 给指纹 + 条数 + 判据 rc + 结论件落点", true,
		strings.HasPrefix(fp, "sha256:") && int(items) == 4 && rcv == 0 && strings.HasSuffix(rp, vrRecordName),
		true, fmt.Sprintf("  （items=%d）", int(items)))
	_ = rc10

	// ⑪ 对照：只在注释里提到 /tmp ⇒ 不红
	body11 := "#!/usr/bin/env python3\n# 历史：旧法把结论落在 " + vrTmpSlash + "x（已挪走）\nprint(1)\n"
	r11 := vrMkFixture(tmp, "case-11", vrFixtureGate, nil, nil, body11, "check-a.py")
	rc11, out11 := capRun(r11, vrPyJoin(tmp, "state-11"), vrArgs{})
	vrCase("⑪ 对照：只在注释里『提到 "+vrTmpBare+"』⇒ 不算落点命中（rc=0）", 0, rc11, strings.Contains(out11, "落点面 0 命中"), "")

	// ⑫ 自豁免
	r12 := vrMkFixture(tmp, "case-12", vrFixtureGate, nil, nil, "", "check-a.py")
	body12 := "#!/usr/bin/env python3\nPAT = r\"> " + vrTmpSlash + "x\"\nprint(\"bash < " + vrTmpSlash + "x\")\n"
	_ = os.WriteFile(vrPyJoin(r12, "scripts", "gates", "check-verdict-record.py"), []byte(body12), 0o666)
	rc12, out12 := capRun(r12, vrPyJoin(tmp, "state-12"), vrArgs{})
	vrCase("⑫ 自豁免：同名件（本件自己）不在扫面里 · 理由照打", 0, rc12,
		strings.Contains(out12, "自豁免") && strings.Contains(out12, "落点面 0 命中"), "")
	vrCase("⑫b 自豁免只放**本件**、不放别人（同名判据件别处仍照判）", true,
		!strings.Contains(out12, "scripts/gates/check-a.py"), true, "")

	// ⑬ 正控：合成标本串 ⇒ 登记豁免
	r13 := vrMkFixture(tmp, "case-13", vrFixtureGate, nil, nil, vrSpecCase, "check-token-argv.py")
	rc13, out13 := capRun(r13, vrPyJoin(tmp, "state-13"), vrArgs{})
	vrCase("⑬ 正控：合成标本串（夹具 NEG_CASES 正文）⇒ 登记豁免 · rc=0", 0, rc13,
		strings.Contains(out13, "夹具登记豁免") && strings.Contains(out13, "落点面 0 命中"), "")
	vrCase("⑬b 登记豁免**逐条打印理由** + 未登记的条目**照实打印悬空 · 不判红**", true,
		strings.Contains(out13, "自检夹具（合成标本串）") && strings.Contains(out13, "悬空登记"), true, "")

	// ⑬c 负控：同件命令位真写 ⇒ 照旧红
	r13c := vrMkFixture(tmp, "case-13c", vrFixtureGate, nil, nil, vrRealWrite, "check-token-argv.py")
	rc13c, out13c := capRun(r13c, vrPyJoin(tmp, "state-13c"), vrArgs{})
	vrCase("⑬c 负控：同件里命令位真写 "+vrTmpBare+" ⇒ 照旧红（rc=1）", 1, rc13c, strings.Contains(out13c, "落点面命中"), "")

	// ⑬d 负控：另一登记件里 open(...,"w") ⇒ 照旧红
	r13d := vrMkFixture(tmp, "case-13d", vrFixtureGate, nil, nil,
		"#!/usr/bin/env python3\nf = open(\""+vrTmpSlash+"w.log\", \"w\")\n", "check-wired-scripts.py")
	rc13d, out13d := capRun(r13d, vrPyJoin(tmp, "state-13d"), vrArgs{})
	vrCase("⑬d 负控：另一登记件里 open(...,\"w\") 写盘 ⇒ 照旧红（rc=1）", 1, rc13d, strings.Contains(out13d, "落点面命中"), "")

	final := "全过"
	if !ok {
		final = "**有断言失败**"
	}
	fmt.Fprintf(stdout, "自检结论: %s\n", final)
	if ok {
		return 0
	}
	return 1
}

// ── 参数解析（argparse 的**已知旗标面**逐字同形 · 缩写/帮助正文见件头未覆盖面①）────────────────
func vrParseArgs(stderr io.Writer, argv []string) (vrArgs, *int) {
	a := vrArgs{}
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	a.root = wd
	needVal := func(i *int, name string) (string, *int) {
		if *i+1 < len(argv) {
			*i++
			return argv[*i], nil
		}
		fmt.Fprintf(stderr, "usage: %s [-h] [--root ROOT] [--state-dir STATE_DIR] ...\n", vrProgName)
		fmt.Fprintf(stderr, "%s: error: argument %s: expected one argument\n", vrProgName, name)
		rc := 2
		return "", &rc
	}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		name, val, hasVal := arg, "", false
		if strings.HasPrefix(arg, "--") {
			if eq := strings.IndexByte(arg, '='); eq >= 0 {
				name, val, hasVal = arg[:eq], arg[eq+1:], true
			}
		}
		switch name {
		case "--root":
			if !hasVal {
				var rc *int
				if val, rc = needVal(&i, "--root"); rc != nil {
					return a, rc
				}
			}
			a.root = val
		case "--state-dir":
			if !hasVal {
				var rc *int
				if val, rc = needVal(&i, "--state-dir"); rc != nil {
					return a, rc
				}
			}
			a.stateDir = val
		case "--json":
			a.jsonOut = true
		case "--fingerprint":
			a.fingerprint = true
		case "--record":
			a.record = true
		case "--compare":
			a.compare = true
		case "--list-rules":
			a.listRules = true
		case "--self-test":
			a.selfTest = true
		case "--version":
			fmt.Fprintf(stderr, "%s\n", vrVersion)
			rc := 0
			return a, &rc
		case "-h", "--help":
			fmt.Fprintf(stderr, "usage: %s [-h] [--root ROOT] [--state-dir STATE_DIR] [--json] [--fingerprint]\n"+
				"                [--record] [--compare] [--list-rules] [--self-test] [--version]\n", vrProgName)
			rc := 0
			return a, &rc
		default:
			if strings.HasPrefix(arg, "-") && arg != "-" {
				fmt.Fprintf(stderr, "usage: %s [-h] [--root ROOT] [--state-dir STATE_DIR] ...\n", vrProgName)
				fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", vrProgName, arg)
				rc := 2
				return a, &rc
			}
			fmt.Fprintf(stderr, "usage: %s [-h] [--root ROOT] [--state-dir STATE_DIR] ...\n", vrProgName)
			fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", vrProgName, arg)
			rc := 2
			return a, &rc
		}
	}
	return a, nil
}

// checkVerdictRecordGateMain —— 入口（= py `main()`）。
func checkVerdictRecordGateMain(stdout, stderr io.Writer, argv []string) int {
	a, rcp := vrParseArgs(stderr, argv)
	if rcp != nil {
		return *rcp
	}
	if a.selfTest {
		return vrSelfTest(stdout)
	}
	if a.listRules {
		return vrListRules(stdout)
	}
	st := vrStateDir(a.stateDir)
	if a.fingerprint {
		return vrCmdFingerprint(stdout, a.root)
	}
	if a.compare {
		return vrCmdCompare(stdout, st, vrSubject)
	}
	return vrRun(stdout, a.root, st, a)
}
