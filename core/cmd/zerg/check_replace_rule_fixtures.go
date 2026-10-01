// check_replace_rule_fixtures.go —— 判据件 `scripts/gates/check-replace-rule-fixtures.py`（551 行）的
// **Go 真搬**（v2.5.14 · M2 · 入口 `checkReplaceRuleFixturesGateMain`）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳；M2 = 按价值真搬 Go（落点 `core/cmd/zerg/`）。
//	本件 = M2 一枚，验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码 **三面逐字相同**
//	（除下列**显式声明的**差异面）。★ 本件**只落码、不接线**：不注册命令、不删 py 原件、不动登记面。
//
// 外部树走显式入参旋钮（照 py 同名旗标）：
//
//	py 本就带 `--root`（判哪棵树 · 默认 = 仓根）· `--rules`（规则表路径）· `--whitelist`（白名单路径）
//	⇒ 本件**同名照搬、不另起名**（✗ 不造第三个互相不认识的名字）。三者都是 py 原生旗标 ⇒ **无旋钮面差**。
//
// 缺省推导（逐字同构）：
//
//	py  `REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))`（脚本位置 scripts/gates 上溯两层）
//	Go  `crrfDefaultRepo()` = 从 **cwd** 上溯找含 `scripts/gates/` 的目录（无 `__file__` 语境）
//	py  `rules_path = args.rules or os.path.join(root, "publish/replace-rules.tsv")`
//	Go  `crrfRulesPath = rules or crrfPyJoin(root, crrfRulesRel)`（★ 字符串拼接，✗ filepath.Join —— 缺省串
//	    无 `..` 段 ⇒ 两法同值，仍用串拼接保「`..` 原样留在打印串」的 py 语义）
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py 从 `__file__` 上溯（与 cwd 无关）⟷ Go 无 `__file__` 语境、只能从 cwd 上溯。
//	    「脚本位置 == cwd 上溯根」时两者读数逐字相同（对拍甲/乙已证）；否则**路径推导面不同** ⇒ 不属
//	    逐字对齐承诺，对拍一律显式传根（甲=真仓显式传根）或在夹具里让两者重合（乙=仓外夹具 cwd 上溯
//	    根 == py `__file__` 默认根 == 夹具根）。
//	(b) **自指检查（先真判 · 四形态逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否（实现件）**。本门件面 = 白名单前缀下的 `*_test.go`。本件
//	      `.go` 落 `core/cmd/zerg/` ⇒ 不以 `_test.go` 结尾 ⇒ **不进件面**。量化：件面过滤只有
//	      `strings.HasSuffix(name, crrfFixtureSuffix)` 一处（`crrfFixtureSuffix = "_test.go"`）。
//	    ② **本件的测试件在件面内吗 ⇒ 是**（`check_replace_rule_fixtures_test.go` 以 `_test.go` 结尾、
//	      落 `core/` 前缀下）⇒ 必须**保证它零命中任何替换规则**（否则真仓本门读数被我方自伤污染）。
//	      处置：测试件正文**零私有面字面量**（家目录 / 用户名 / 内网地址 / 私有项目名 / 仓名一律不写：
//	      夹具正文用合成串 `demo` · `userdemo` · `/srv/demo` · `198.51.100.7`，与 py 自检同一套合成值）；
//	      需要引用被扫形态时**拆串**（如 `"Zerg-" + "docs"`）。**证明有效**见交付读数（真仓本门输出
//	      sha16 落仓前后逐字相同）。
//	    ③ **self-test 魔法串是否连续字面落地** ⇒ 否：本件无「self-test 开关魔法串」机检形态。
//	    ④ **是否落在任何他门扫描面内** ⇒ 会（`core/cmd/zerg/*.go` 落 `check-error-kinds` 等门的扫描面）
//	      ⇒ 处置同②：正文零绝对路径字面量、零 `kind` 键名字面量。**证明有效**见「同根串影子仓两态 sha16 相同」。
//
// py 原件真口子（★ 不自行修 · 逐字保留 · **交回入账**）：
//
//	① **B 类读失败静默**（py `:221` `except OSError: continue`）：`collect_fixtures` 对**存在但读不了**的
//	  `*_test.go` 静默 `continue` —— 该件已被 `scanned += 1` 计入件面、但**一行都没进 `fixtures`**，
//	  于是「命中/不命中」都可能被这份静默掩盖（少判且无任何落点）。本件**逐字照搬**此行为（保两态对齐）：
//	  `crrfReadText` 出错 ⇒ `continue`，账另行入册（✗ 不在本单改 py、✗ 不改本件行为）。
//	② **最外层无兜底**：py `__main__` 是裸 `sys.exit(main())` ⇒ 规则表/白名单「存在但读不了」（权限/解码）
//	  会抛未捕异常 ⇒ 回溯 + **rc=1**（不是「不给结论 rc=2」）。本件按新口径改为 **fail-closed rc=2** 并
//	  写进未覆盖面（口径差 · 交回入账）。
//	③ **白名单缺失处的档位**：py 对「白名单缺件」本来就走 rc=2（`whitelist_prefixes` 返回 None ⇒ main 印
//	  「白名单缺件」rc=2）⇒ 本件**逐字沿用 rc=2**（零口径差）。
//
// 遍历 / 排序点逐点交代（★ 决定输出行序的那一条已点名）：
//
//	T1 `collect_fixtures` 的前缀循环：按 `prefixes` **入参序**（= 白名单文件行序）。
//	T2 `os.walk(base)`：**readdir 原序**、topdown（先当前层文件、再按 `dirnames` 原序下钻）、
//	  `followlinks=False`。Go 侧一律 `f.ReadDir(-1)`（★ ✗ 不用会排序的 `os.ReadDir` / `WalkDir`）。
//	T3 目录分类用 **`os.Stat`**（跟随符号链接 = py `entry.is_dir()`），✗ 不用 `entry.IsDir()`（Lstat 语义，
//	  对「指向目录的符号链接」会判反）；递归资格另判「非符号链接」（同 `os.walk` followlinks=False）。
//	T4 `dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]`：**目录名**成员判定（保序）。
//	T5 `hits` = 命中行明细，**保 os.walk 产出的原序**（前缀序 → 目录序 → 文件 readdir 序 → 行号升序）。
//	T6 `iso_files`：收集后**显式 `sorted()`**（`sort.Strings`）。
//	T7 `judge`：`per_file` 是 py dict ⇒ 用**切片对 + 索引 map** 保插入序（★ 零 map 迭代进输出）；
//	  输出用到的两个序 = `sorted(per_file)`（件名字典序，`sort.Strings`）与「每件前 2 行」（行号升序）。
//	T8 `json_payload` 的 `dedup = sorted(set(...))`（`sort.Strings`）；`per_rule[].lines` =
//	  `for rel in sorted(detail)`（件序）→ 件内**行号升序**。
//	★★ **输出行序的决定者**：明细行序由 **T5 的 fixtures 原序**给出（因 T7 里每件行号本已升序，故与
//	  T5 的具体目录顺序无关）；而 `hits[]` 的**规则序** = 规则表**文件行序** + 命令行合成规则序（切片保序）。
//	  `sorted()` 只决定**件名**的列举次序（T6/T7/T8），**不**决定 `hits` 明细的行序。
//
// 正则语义差（py `re` ⟷ Go RE2；涉及面逐条写明）：
//
//	R1 `\d`：py Unicode ⟷ Go ASCII ⇒ 译 `\p{Nd}`（本件 `crrfPyRegexToGo`）。涉及：规则表 `\d{1,3}`（两条
//	  内网兜底）· `\d{4}`（版权行）。
//	R2 `\b`：py Unicode 词界 ⟷ Go ASCII ⇒ 译 `(?:[^\p{L}\p{N}_]|\A|\z)`（消费式等价，仅判在场）。
//	  剩余分叉（写进未覆盖面）：`\b` 紧邻非 ASCII 词字符时两侧理论仍可能不同。
//	R3 `\u200b` 等 `\uXXXX`：py `re` 支持 ⟷ Go RE2 不支持 `\u` ⇒ 译 `\x{XXXX}`。
//	R4 py 非多行 `$` = 「串尾或串尾换行之前」⟷ Go `$` 只匹配串尾：本件输入行**无尾换行**（判据前已
//	  `rstrip("\n")`）⇒ 该面不触发；仍写进未覆盖面。
//	R5 `errors="replace"` 非 UTF-8：py 逐序列替 U+FFFD ⟷ Go `strings.ToValidUTF8` 按无效**串**替换
//	  ⇒ 粒度不同（真仓 Go 件皆合法 UTF-8 ⇒ 不触发），写进未覆盖面。
//	R6 正则**写坏**时 py `re.error` 正文 ⟷ Go `regexp` 错误正文不同 ⇒ 该格只保 rc=2，正文不同（写进未覆盖面）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse **长选项前缀缩写**本件**实现**（`--ro`⇒`--root`）与 `--opt=值`，歧义报话逐字复刻；
//	  但 argparse 的极端内部形态（`store_true` 带 `=值` 的 "ignored explicit argument" 文案）未逐一验证。
//	② 见 (a)：**默认根定位**「脚本位置 ≠ cwd 上溯根」时不属逐字对齐承诺。
//	③ 见口子②：规则表/白名单「存在但读不了」py 未捕异常 rc=1 ⟷ 本件 fail-closed rc=2（**有意口径差**）。
//	④ 见 R1–R6。
//	⑤ `--self-test` 的 `tempfile.mkdtemp` 随机后缀**不进输出**（输出只打 ✓/✗ 行）⇒ 两态逐字可对齐；
//	   Go 侧 `os.MkdirTemp` + `defer os.RemoveAll` 回收（py `shutil.rmtree(ignore_errors=True)`）。
//
// `--evidence GAP-20261001-13`（在册 M2 承接 · 先已回读 · 语义对口 · 状态目录 `~/.zerg/state/zerg-cli-gaps.jsonl`）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── 常量与数据（逐字照 py）────────────────────────────────────────────────

const crrfProg = "check-replace-rule-fixtures.py"
const crrfRulesRel = "publish/replace-rules.tsv"
const crrfWhitelistRel = "publish/whitelist.txt"
const crrfIsoPrefix = "!夹具"
const crrfFixtureSuffix = "_test.go"

// crrfJSONFields —— 「机器面字段表」**唯一真源**（装包封与「不认的名 ⇒ rc=2」两处同读）。
var crrfJSONFields = []string{"rules", "hits", "files", "fixtures", "files_dedup", "files_sum", "per_rule"}

// crrfExcludeDirs —— 通用排除表（目录**名**成员判定；读表不打序）。
var crrfExcludeDirs = map[string]bool{
	"target": true, "node_modules": true, "dist": true, "bin": true, "vendor": true,
	"data": true, ".git": true, ".venv": true, "venv": true, ".build": true, "_history": true,
}

type crrfRule struct {
	raw  string
	isRe bool
	pat  string
	rep  string
	note string
}

type crrfFixture struct {
	rel    string
	lineno int
	line   string
}

type crrfLineTxt struct {
	ln  int
	txt string
}

type crrfDetailEntry struct {
	rel   string
	lines []crrfLineTxt
}

type crrfExample struct {
	rel string
	ln  int
	txt string
}

type crrfHit struct {
	raw    string
	files  []string
	ex     []crrfExample
	total  int
	detail []crrfDetailEntry
}

// ── py 语义薄壳 ─────────────────────────────────────────────────────────

// crrfPyJoin —— `os.path.join` 逐字（保 `..` 段）。
func crrfPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		if strings.HasPrefix(p, "/") || out == "" {
			out = p
			continue
		}
		if strings.HasSuffix(out, "/") {
			out = out + p
		} else {
			out = out + "/" + p
		}
	}
	return out
}

// crrfPyDir —— `os.path.dirname`（词法）。
func crrfPyDir(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	if i == 0 {
		return "/"
	}
	return p[:i]
}

// crrfJoin —— 斜杠拼串（rel 只作打印/前缀匹配，✗ 不落盘）。
func crrfJoin(a, b string) string {
	if a == "" {
		return b
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// crrfNormPath —— `os.path.normpath` 纯词法版（折叠 `//` / `.` / `..`）。
func crrfNormPath(p string) string {
	if p == "" {
		return "."
	}
	abs := strings.HasPrefix(p, "/")
	out := []string{}
	for _, c := range strings.Split(p, "/") {
		switch c {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
				continue
			}
			if abs {
				continue
			}
			out = append(out, "..")
		default:
			out = append(out, c)
		}
	}
	res := strings.Join(out, "/")
	if abs {
		res = "/" + res
	}
	if res == "" {
		res = "."
	}
	return res
}

// crrfAbs —— `os.path.abspath`（相对 ⇒ 拼 cwd；再 normpath）。
func crrfAbs(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		p = wd + "/" + p
	}
	return crrfNormPath(p)
}

// crrfRelPath —— `os.path.relpath(path, start)` 纯词法版。
func crrfRelPath(path, start string) string {
	p := crrfAbs(path)
	s := crrfAbs(start)
	if p == s {
		return "."
	}
	pp := []string{}
	if p != "/" {
		pp = strings.Split(strings.TrimPrefix(p, "/"), "/")
	}
	ss := []string{}
	if s != "/" {
		ss = strings.Split(strings.TrimPrefix(s, "/"), "/")
	}
	i := 0
	for i < len(pp) && i < len(ss) && pp[i] == ss[i] {
		i++
	}
	out := []string{}
	for j := i; j < len(ss); j++ {
		out = append(out, "..")
	}
	out = append(out, pp[i:]...)
	if len(out) == 0 {
		return "."
	}
	return strings.Join(out, "/")
}

// crrfDefaultRepo —— 见 (a)：cwd 上溯找含 `scripts/gates/` 的目录。
func crrfDefaultRepo() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := crrfAbs(wd)
	for i := 0; i < 12; i++ {
		if st, e := os.Stat(crrfJoin(crrfJoin(d, "scripts"), "gates")); e == nil && st.IsDir() {
			return d
		}
		parent := crrfPyDir(d)
		if parent == "" || parent == d {
			break
		}
		d = parent
	}
	return crrfAbs(wd)
}

// crrfIsDir —— `os.path.isdir`（跟随符号链接）。
func crrfIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// crrfIsFile —— `os.path.isfile`（跟随符号链接）。
func crrfIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// crrfIsSymlink —— `os.path.islink`（Lstat）。
func crrfIsSymlink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

// crrfUniversalNewlines —— py 文本模式默认 `newline=None` 的换行归一（`\r\n`/`\r` ⇒ `\n`）。
func crrfUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// crrfReadText —— 读件 ⇒ 逐字近似 py `open(..., encoding="utf-8", errors="replace")` + 通用换行。
func crrfReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return crrfUniversalNewlines(strings.ToValidUTF8(string(b), "\uFFFD")), nil
}

// crrfSplitLines —— py 文件行迭代的等价（`\n` 分隔；末尾唯一空段丢弃；空串 ⇒ 0 行）。
func crrfSplitLines(s string) []string {
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// crrfIsPySpace —— py `str.isspace()` 认的空白（含 `\x1c-\x1f`；✗ 不是 Go 的 `unicode.IsSpace` 全等）。
func crrfIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	if r > 0x7f {
		return unicode.IsSpace(r)
	}
	return false
}

// crrfPyStrip —— py `str.strip()`（Unicode 空白）。
func crrfPyStrip(s string) string {
	return strings.TrimFunc(s, crrfIsPySpace)
}

// crrfPyLStrip —— py `str.lstrip()`。
func crrfPyLStrip(s string) string {
	return strings.TrimLeftFunc(s, crrfIsPySpace)
}

// crrfRuneCut —— py 串切片 `s[:n]`（按**码点**，✗ 不是字节）。
func crrfRuneCut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// crrfPyReprStr —— py `repr(str)`（错误正文用；非 ASCII 可打印字符原样保留）。
func crrfPyReprStr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case rune(quote):
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// ── 正则：py `re` ⇒ Go RE2（R1–R3 的对齐；不可对齐者已在件头声明）─────────────────

// crrfPyRegexToGo —— 把 py 正则源串译成 Go RE2 等价串（`\d`→`\p{Nd}` · `\b`→消费式词界 · `\uXXXX`→`\x{..}`）。
func crrfPyRegexToGo(pat string) string {
	var b strings.Builder
	rs := []rune(pat)
	i := 0
	for i < len(rs) {
		c := rs[i]
		if c != '\\' || i+1 >= len(rs) {
			b.WriteRune(c)
			i++
			continue
		}
		n := rs[i+1]
		switch n {
		case 'd':
			b.WriteString(`\p{Nd}`)
			i += 2
		case 'D':
			b.WriteString(`\P{Nd}`)
			i += 2
		case 's':
			b.WriteString(crrfPyWSClass)
			i += 2
		case 'S':
			b.WriteString(`[^` + crrfPyWSBody + `]`)
			i += 2
		case 'w':
			b.WriteString(`[\p{L}\p{N}_]`)
			i += 2
		case 'W':
			b.WriteString(`[^\p{L}\p{N}_]`)
			i += 2
		case 'b':
			b.WriteString(`(?:[^\p{L}\p{N}_]|\A|\z)`)
			i += 2
		case 'Z':
			b.WriteString(`\z`)
			i += 2
		case 'u':
			if i+5 < len(rs) && crrfAllHex(rs[i+2:i+6], 4) {
				b.WriteString(`\x{` + string(rs[i+2:i+6]) + `}`)
				i += 6
			} else {
				b.WriteRune(c)
				i++
			}
		case 'U':
			if i+9 < len(rs) && crrfAllHex(rs[i+2:i+10], 8) {
				b.WriteString(`\x{` + string(rs[i+2:i+10]) + `}`)
				i += 10
			} else {
				b.WriteRune(c)
				i++
			}
		default:
			b.WriteRune(c)
			b.WriteRune(n)
			i += 2
		}
	}
	return b.String()
}

const crrfPyWSBody = `\t\n\v\f\r \x{001c}-\x{001f}\x{0085}\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`
const crrfPyWSClass = `[` + crrfPyWSBody + `]`

func crrfAllHex(rs []rune, n int) bool {
	if len(rs) != n {
		return false
	}
	for _, r := range rs {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// crrfCompileRule —— 编译一条规则的正则（译后编译；错误交调用方按「不给结论」处理）。
func crrfCompileRule(pat string) (*regexp.Regexp, error) {
	return regexp.Compile(crrfPyRegexToGo(pat))
}

// ── fnmatch（py `fnmatch.fnmatch` 的等价）──────────────────────────────────

// crrfFnmatchTranslate —— glob ⇒ 全串匹配正则（py `fnmatch.translate` 的 `(?s:...)\Z` 形态）。
func crrfFnmatchTranslate(pat string) string {
	var b strings.Builder
	b.WriteString(`(?s)\A`)
	runes := []rune(pat)
	i := 0
	for i < len(runes) {
		c := runes[i]
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := i + 1
			if j < len(runes) && (runes[j] == '!' || runes[j] == '^') {
				j++
			}
			if j < len(runes) && runes[j] == ']' {
				j++
			}
			for j < len(runes) && runes[j] != ']' {
				j++
			}
			if j >= len(runes) {
				b.WriteString(`\[`)
			} else {
				stuff := runes[i+1 : j]
				b.WriteByte('[')
				k := 0
				if len(stuff) > 0 && (stuff[0] == '!' || stuff[0] == '^') {
					b.WriteByte('^')
					k = 1
				}
				for ; k < len(stuff); k++ {
					if stuff[k] == '\\' {
						b.WriteString(`\\`)
					} else {
						b.WriteRune(stuff[k])
					}
				}
				b.WriteByte(']')
				i = j
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
		i++
	}
	b.WriteString(`\z`)
	return b.String()
}

// crrfFnmatch —— py `fnmatch.fnmatch(name, pat)`（posix 下 case-sensitive）。
func crrfFnmatch(name, pat string) bool {
	re, err := regexp.Compile(crrfFnmatchTranslate(pat))
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

// crrfIsIsolated —— py `is_isolated`。
func crrfIsIsolated(rel string, isolated []string) bool {
	if rel == "" {
		return false
	}
	for _, pat := range isolated {
		if crrfFnmatch(rel, pat) {
			return true
		}
	}
	return false
}

// ── 规则表 ────────────────────────────────────────────────────────────────

// crrfParseRules —— py `parse_rules`（返回 (rules, isolated, error)；error 非空 ⇒ 调用方按「不给结论」处理）。
func crrfParseRules(path string, extra []string) ([]crrfRule, []string, string) {
	if !crrfIsFile(path) {
		return nil, nil, fmt.Sprintf("规则表缺件：%s（替换规则的单一真源不在场 ⇒ 判据不可判）", crrfRulesRel)
	}
	text, err := crrfReadText(path)
	if err != nil {
		// py 此处抛未捕 OSError（rc=1）⇒ 本件 fail-closed rc=2（件头未覆盖面③）。
		return nil, nil, fmt.Sprintf("规则表缺件：%s（替换规则的单一真源不在场 ⇒ 判据不可判）", crrfRulesRel)
	}
	var rules []crrfRule
	var isolated []string
	for i, line := range strings.Split(text, "\n") {
		if crrfPyStrip(line) == "" || strings.HasPrefix(crrfPyLStrip(line), "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			return nil, nil, fmt.Sprintf("规则表第 %d 行不是 `pattern<TAB>replacement...` 形态：%s",
				i+1, crrfPyReprStr(crrfRuneCut(line, 80)))
		}
		pat, rep := parts[0], parts[1]
		if pat == crrfIsoPrefix {
			if strings.TrimSpace(rep) == "" {
				return nil, nil, fmt.Sprintf("规则表第 %d 行是空的夹具隔离（`!夹具` 后面没有 glob ⇒ 判据不可判）：%s",
					i+1, crrfPyReprStr(crrfRuneCut(line, 80)))
			}
			isolated = append(isolated, strings.TrimSpace(rep))
			continue
		}
		note := ""
		if len(parts) > 2 {
			note = parts[2]
		}
		isRe := strings.HasPrefix(pat, "regex:")
		p := pat
		if isRe {
			p = pat[len("regex:"):]
		}
		rules = append(rules, crrfRule{raw: line, isRe: isRe, pat: p, rep: rep, note: note})
	}
	for _, p := range extra {
		isRe := strings.HasPrefix(p, "regex:")
		pp := p
		if isRe {
			pp = p[len("regex:"):]
		}
		rules = append(rules, crrfRule{raw: "（合成规则 · 负控用）" + p, isRe: isRe, pat: pp, rep: "<合成>", note: "命令行注入"})
	}
	return rules, isolated, ""
}

// ── 扫描面 ────────────────────────────────────────────────────────────────

// crrfWhitelistPrefixes —— py `whitelist_prefixes`（(prefixes, ok)；ok=false ⇒ 缺件 = None）。
func crrfWhitelistPrefixes(path string) ([]string, bool) {
	if !crrfIsFile(path) {
		return nil, false
	}
	text, err := crrfReadText(path)
	if err != nil {
		// py 此处抛未捕 OSError（rc=1）⇒ 本件 fail-closed（按缺件 rc=2，件头未覆盖面③）。
		return nil, false
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		s := crrfPyStrip(line)
		if s == "" || strings.HasPrefix(s, "#") || strings.HasSuffix(s, "__EXCLUDES_ABOVE__") {
			continue
		}
		if strings.HasSuffix(s, "/") {
			out = append(out, s)
		}
	}
	return out, true
}

// crrfCollectFixtures —— py `collect_fixtures`（T1–T6）。
func crrfCollectFixtures(root string, prefixes, isolated []string) ([]crrfFixture, []string, int) {
	var hits []crrfFixture
	var isoFiles []string
	scanned := 0
	for _, pre := range prefixes { // T1 前缀入参序
		base := crrfPyJoin(root, strings.TrimSuffix(pre, "/"))
		if !crrfIsDir(base) {
			continue
		}
		crrfWalkInto(&hits, &isoFiles, &scanned, root, base, isolated)
	}
	sort.Strings(isoFiles) // T6 显式 sorted()
	return hits, isoFiles, scanned
}

// crrfWalkInto —— 一层 + 递归（T2 遍历序 / T3 Stat 分类 / T4 排除目录名）。
func crrfWalkInto(hits *[]crrfFixture, isoFiles *[]string, scanned *int, root, dir string, isolated []string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	ents, err := f.ReadDir(-1) // ★ T2 readdir 原序（✗ 排序版 os.ReadDir / ✗ WalkDir）
	_ = f.Close()
	if err != nil {
		return
	}
	var subdirs []string
	var files []string
	for _, e := range ents {
		name := e.Name()
		full := crrfJoin(dir, name)
		st, serr := os.Stat(full) // ★ T3 跟随符号链接 = py entry.is_dir()
		if serr == nil && st.IsDir() {
			if !crrfExcludeDirs[name] { // T4 目录名成员判定（保序）
				if !crrfIsSymlink(full) { // os.walk followlinks=False ⇒ 符号链接目录不下钻
					subdirs = append(subdirs, name)
				}
			}
			continue
		}
		if strings.HasSuffix(name, crrfFixtureSuffix) {
			files = append(files, name)
		}
	}
	for _, name := range files { // 当前层文件先于子目录（os.walk topdown）
		full := crrfJoin(dir, name)
		rel := crrfRelPath(full, root)
		*scanned++
		if crrfIsIsolated(rel, isolated) {
			*isoFiles = append(*isoFiles, rel)
			continue
		}
		text, rerr := crrfReadText(full)
		if rerr != nil {
			continue // ★ py `except OSError: continue`（B 类读失败静默 · 逐字保留 · 见件头口子①）
		}
		for i, ln := range crrfSplitLines(text) {
			*hits = append(*hits, crrfFixture{rel: rel, lineno: i + 1, line: ln})
		}
	}
	for _, d := range subdirs { // T2 子目录原序递归
		crrfWalkInto(hits, isoFiles, scanned, root, crrfJoin(dir, d), isolated)
	}
}

// ── 判决（唯一出口 · 自检直接钉它）────────────────────────────────────────

// crrfJudge —— py `judge`（返回 (命中表, 报错)；报错非空 ⇒ 整表作废 = 返回空命中表）。
func crrfJudge(rules []crrfRule, fixtures []crrfFixture, maxExamples int) ([]crrfHit, string) {
	var hits []crrfHit
	for _, rule := range rules {
		if rule.isRe && rule.pat == "" {
			return nil, fmt.Sprintf("规则表里有一条**空正则**（%s）⇒ 判据不可判", crrfPyReprStr(crrfRuneCut(rule.raw, 60)))
		}
		var detail []crrfDetailEntry
		idx := map[string]int{}
		var re *regexp.Regexp
		var reErr error
		compiled := false
		for _, fx := range fixtures { // T7 原序扫描
			if !strings.HasSuffix(fx.rel, crrfFixtureSuffix) {
				continue
			}
			matched := false
			if rule.isRe {
				if !compiled {
					re, reErr = crrfCompileRule(rule.pat)
					compiled = true
				}
				if reErr != nil {
					return nil, fmt.Sprintf("规则表里有一条**正则写坏了**（%s）：%s",
						crrfPyReprStr(crrfRuneCut(rule.raw, 60)), reErr.Error())
				}
				matched = re.MatchString(fx.line)
			} else {
				matched = strings.Contains(fx.line, rule.pat)
			}
			if matched {
				txt := crrfRuneCut(crrfPyStrip(fx.line), 160)
				if j, ok := idx[fx.rel]; ok {
					detail[j].lines = append(detail[j].lines, crrfLineTxt{fx.lineno, txt})
				} else {
					idx[fx.rel] = len(detail)
					detail = append(detail, crrfDetailEntry{rel: fx.rel, lines: []crrfLineTxt{{fx.lineno, txt}}})
				}
			}
		}
		if len(detail) > 0 {
			total := 0
			for _, d := range detail {
				total += len(d.lines)
			}
			files := make([]string, 0, len(detail))
			for _, d := range detail {
				files = append(files, d.rel)
			}
			sort.Strings(files) // T7 sorted(per_file)
			lim := maxExamples
			if lim < 0 {
				lim = len(files) + lim
			}
			if lim > len(files) {
				lim = len(files)
			}
			if lim < 0 {
				lim = 0
			}
			var ex []crrfExample
			for k := 0; k < lim; k++ {
				rel := files[k]
				for m, lt := range detail[idx[rel]].lines {
					if m >= 2 {
						break
					}
					ex = append(ex, crrfExample{rel, lt.ln, lt.txt})
				}
			}
			hits = append(hits, crrfHit{raw: rule.raw, files: files, ex: ex, total: total, detail: detail})
		}
	}
	return hits, ""
}

// ── 机器面（byte 级复刻 py `json.dumps(obj, ensure_ascii=False)`）─────────────

// crrfJSONStr —— py `json.dumps(str, ensure_ascii=False)` 的字符串格。
func crrfJSONStr(s string) string {
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

// crrfJSONStrList —— py `json.dumps([str, ...])` 的数组格（空 ⇒ `[]`；★ 不 omitempty）。
func crrfJSONStrList(xs []string) string {
	if len(xs) == 0 {
		return "[]"
	}
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = crrfJSONStr(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// crrfJSONPayload —— py `json_payload`：键**固定序**（rules, fixtures, hits, files, files_dedup,
// files_sum, per_rule）按 `want` 逐键决定在场；老四格一字不改（`files` 仍是去重集）。
func crrfJSONPayload(want []string, nRules, nFixtures int, hits []crrfHit) string {
	seen := map[string]bool{}
	dedup := []string{}
	for _, h := range hits {
		for _, f := range h.files {
			if !seen[f] {
				seen[f] = true
				dedup = append(dedup, f)
			}
		}
	}
	sort.Strings(dedup)
	w := map[string]bool{}
	for _, x := range want {
		w[x] = true
	}
	var parts []string
	if w["rules"] {
		parts = append(parts, crrfJSONStr("rules")+": "+strconv.Itoa(nRules))
	}
	if w["fixtures"] {
		parts = append(parts, crrfJSONStr("fixtures")+": "+strconv.Itoa(nFixtures))
	}
	if w["hits"] {
		parts = append(parts, crrfJSONStr("hits")+": "+strconv.Itoa(len(hits)))
	}
	if w["files"] {
		parts = append(parts, crrfJSONStr("files")+": "+crrfJSONStrList(dedup))
	}
	if w["files_dedup"] {
		parts = append(parts, crrfJSONStr("files_dedup")+": "+crrfJSONStrList(dedup))
	}
	if w["files_sum"] {
		sum := 0
		for _, h := range hits {
			sum += len(h.files)
		}
		parts = append(parts, crrfJSONStr("files_sum")+": "+strconv.Itoa(sum))
	}
	if w["per_rule"] {
		var prs []string
		for _, h := range hits {
			rels := make([]string, 0, len(h.detail))
			idx := map[string]int{}
			for i, d := range h.detail {
				rels = append(rels, d.rel)
				idx[d.rel] = i
			}
			sort.Strings(rels) // T8
			var lns []string
			for _, rel := range rels {
				for _, lt := range h.detail[idx[rel]].lines {
					lns = append(lns, "{"+crrfJSONStr("file")+": "+crrfJSONStr(rel)+
						", "+crrfJSONStr("line")+": "+strconv.Itoa(lt.ln)+
						", "+crrfJSONStr("text")+": "+crrfJSONStr(lt.txt)+"}")
				}
			}
			prs = append(prs, "{"+crrfJSONStr("rule")+": "+crrfJSONStr(crrfPyStrip(h.raw))+
				", "+crrfJSONStr("files")+": "+crrfJSONStrList(h.files)+
				", "+crrfJSONStr("hits")+": "+strconv.Itoa(h.total)+
				", "+crrfJSONStr("lines")+": ["+strings.Join(lns, ", ")+"]}")
		}
		parts = append(parts, crrfJSONStr("per_rule")+": ["+strings.Join(prs, ", ")+"]")
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// ── 夹具隔离体检 ──────────────────────────────────────────────────────────

// crrfIsolationAudit —— py `isolation_audit`（(逐条 [(glob, 件数)], 空转 glob 列表, 是否吃光)）。
func crrfIsolationAudit(isolated, isoFiles []string, scanned int) ([]crrfIsoCount, []string, bool) {
	var per []crrfIsoCount
	for _, pat := range isolated {
		n := 0
		for _, rel := range isoFiles {
			if crrfFnmatch(rel, pat) {
				n++
			}
		}
		per = append(per, crrfIsoCount{pat: pat, n: n})
	}
	var dead []string
	for _, p := range per {
		if p.n == 0 {
			dead = append(dead, p.pat)
		}
	}
	eaten := len(isolated) > 0 && scanned > 0 && len(isoFiles) >= scanned
	return per, dead, eaten
}

type crrfIsoCount struct {
	pat string
	n   int
}

// ── 用法 / argparse 复刻 ──────────────────────────────────────────────────

const crrfUsage = "usage: check-replace-rule-fixtures.py [-h] [--root ROOT] [--rules RULES]\n" +
	"                                      [--whitelist WHITELIST]\n" +
	"                                      [--extra-rule EXTRA_RULE]\n" +
	"                                      [--max-examples MAX_EXAMPLES]\n" +
	"                                      [--json JSON] [--list-rules]\n" +
	"                                      [--self-test]\n"

const crrfHelp = crrfUsage + "\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT           判哪棵树（默认仓根）\n" +
	"  --rules RULES         规则表路径（默认 <root>/publish/replace-rules.tsv）\n" +
	"  --whitelist WHITELIST\n" +
	"                        白名单路径（默认 <root>/publish/whitelist.txt）\n" +
	"  --extra-rule EXTRA_RULE\n" +
	"                        合成一条规则（可重复 · **负控**用）\n" +
	"  --max-examples MAX_EXAMPLES\n" +
	"                        每条规则最多列几件（默认 5）\n" +
	"  --json JSON           机器面字段（逗号分隔：rules,hits,files,fixtures,files_dedup,files\n" +
	"                        _sum,per_rule；**不认的名 ⇒ 用法错 rc=2**）\n" +
	"  --list-rules          打判据 / 扫描面 / 档位\n" +
	"  --self-test           成对负控（合成夹具 · 不碰真目标）\n"

type crrfArgs struct {
	root        string
	rules       string
	whitelist   string
	extraRules  []string
	maxExamples int
	json        string
	listRules   bool
	selfTest    bool
}

var crrfLongOpts = []string{"--root", "--rules", "--whitelist", "--extra-rule", "--max-examples",
	"--json", "--list-rules", "--self-test", "--help"}

var crrfNeedsValue = map[string]bool{"--root": true, "--rules": true, "--whitelist": true,
	"--extra-rule": true, "--max-examples": true, "--json": true}

// crrfResolveLong —— 长旗标**前缀匹配**（唯一命中 ⇒ 规范名；多命中 ⇒ 歧义）。
func crrfResolveLong(tok string) (string, string, bool, error) {
	name := tok
	inline := ""
	hasInline := false
	if i := strings.Index(tok, "="); i >= 0 {
		name = tok[:i]
		inline = tok[i+1:]
		hasInline = true
	}
	if name == "-h" {
		name = "--help"
	} else if strings.HasPrefix(name, "--") {
		matches := []string{}
		for _, o := range crrfLongOpts {
			if o == name {
				matches = []string{o}
				break
			}
			if strings.HasPrefix(o, name) {
				matches = append(matches, o)
			}
		}
		if len(matches) == 0 {
			return "", "", false, nil
		}
		if len(matches) > 1 {
			return "", "", false, fmt.Errorf("ambiguous option: %s could match %s", name, strings.Join(matches, ", "))
		}
		name = matches[0]
	} else {
		return "", "", false, nil
	}
	if hasInline {
		return name, inline, true, nil
	}
	return name, "\x00none", false, nil
}

// crrfParseArgs —— argparse 等价解析（返回 (args, helpText, helpWanted, error)）。
func crrfParseArgs(argv []string) (*crrfArgs, string, bool, error) {
	args := &crrfArgs{maxExamples: 5}
	var unrecognized []string
	endOpts := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if endOpts {
			unrecognized = append(unrecognized, tok)
			continue
		}
		if tok == "--" {
			endOpts = true
			continue
		}
		if tok == "-h" || tok == "--help" {
			return nil, crrfHelp, true, nil
		}
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			unrecognized = append(unrecognized, tok)
			continue
		}
		name, inline, hasInline, err := crrfResolveLong(tok)
		if err != nil {
			return nil, "", false, err
		}
		if name == "" {
			unrecognized = append(unrecognized, tok)
			continue
		}
		val := inline
		if !hasInline && inline == "\x00none" {
			val = ""
		}
		if inline == "\x00none" {
			if crrfNeedsValue[name] {
				if i+1 >= len(argv) || (strings.HasPrefix(argv[i+1], "-") && argv[i+1] != "-" && len(argv[i+1]) > 1) {
					return nil, "", false, fmt.Errorf("argument %s: expected one argument", name)
				}
				i++
				val = argv[i]
			}
		}
		switch name {
		case "--root":
			args.root = val
		case "--rules":
			args.rules = val
		case "--whitelist":
			args.whitelist = val
		case "--extra-rule":
			args.extraRules = append(args.extraRules, val)
		case "--max-examples":
			n, cerr := strconv.Atoi(val)
			if cerr != nil || strconv.Itoa(n) != val {
				return nil, "", false, fmt.Errorf("argument --max-examples: invalid int value: '%s'", val)
			}
			args.maxExamples = n
		case "--json":
			args.json = val
		case "--list-rules":
			args.listRules = true
		case "--self-test":
			args.selfTest = true
		}
	}
	if len(unrecognized) > 0 {
		return nil, "", false, fmt.Errorf("unrecognized arguments: %s", strings.Join(unrecognized, " "))
	}
	return args, "", false, nil
}

// ── list-rules 正文 ───────────────────────────────────────────────────────

func crrfListRules() string {
	var b strings.Builder
	p := func(s string) { b.WriteString(s); b.WriteString("\n") }
	p("⑮ 替换规则级机检：命中件不许是测试夹具（§十三 M8 · §八 待拍 3 的落点）")
	p("真源：Zerg-内部文档/项目文档/v2.5.11/设计-CI适配-v1.1.md §七 + §十三 M8")
	p(fmt.Sprintf("  规则面：%s（`pattern<TAB>replacement<TAB>说明` · `regex:` 走正则）", crrfRulesRel))
	p("  夹具隔离：同一件里的 `!夹具<TAB><路径 glob>` ⇒ 该件整段不入件面（§五 第三批；它不是豁免，是规则不再作用于该件）")
	p(fmt.Sprintf("  件面  ：%s 里**会进公开面**的路径前缀下的 `*_test.go`", crrfWhitelistRel))
	p("  判据  ：任一条规则命中 `*_test.go` ⇒ 红（逐条点名规则/件/行号/片段 · **不给豁免表**）")
	p("退码：0 全绿 · 1 有命中 · 2 不给结论（规则表/白名单缺件 · 空转 · 自检不过 · **`--json` 字段名不认**）")
	p("机器面：`--json <字段,逗号分隔>` —— 老四格 `rules,fixtures,hits,files`（`files` = **去重集**）" +
		"+ 新三格 `files_dedup`（同值别名）/ `files_sum`（**未去重和**）/ `per_rule[]`（逐规则" +
		"`{rule, files, hits, lines}` · `lines` = 逐条 `{件,行,原文}`）—— 只加字段，判据与退码不动；" +
		"**不认的字段名 ⇒ 用法错 `rc=2` ＋ 印字段表**（缺口 `Q-150`：打错的名与零命中必须分得开）")
	return b.String()
}

// ── 自检（成对负控 · 合成夹具 · 不碰真目标）─────────────────────────────────

type crrfSelfCase struct {
	name, rulePat, rel, line, want string
}

var crrfSelfCases = []crrfSelfCase{
	{"负控 MF1 合成规则命中 `core/**/*_test.go`（**必红**）",
		"(合成)regex:/srv/demo", "core/internal/demo/fixture_test.go", `home := "/srv/demo/x.key"`, "hit"},
	{"负控 MF2 内网地址规则命中夹具（真实规则 `→ <worker-ip>` 的同形合成）",
		`(合成)regex:198\.51\.100\.7`, "core/internal/demo/obs_test.go", `"host":"198.51.100.7"`, "hit"},
	{"负控 MF3 多字节替换命中夹具长度假设（真实规则 `→ Mr2109` 多 2 字节的同形合成）",
		"(合成)userdemo", "core/internal/demo/limits_test.go", `secret := "peer userdemo run here ok"`, "hit"},
	{"正控 PC1 同一条规则只命中**非夹具**件 ⇒ 绿（判据不许打成「见到那个词就红」）",
		"(合成)userdemo", "core/internal/demo/impl.go", `home := "/srv/demo/" + user`, "green"},
	{"正控 PC2 规则与件完全不沾 ⇒ 绿",
		"(合成)绝不出现的串-zz", "core/internal/demo/fixture_test.go", "want := 1", "green"},
	{"空转 VAC1 扫描面**一件夹具都没有** ⇒ 不给结论（不许当绿）",
		"(合成)userdemo", "", "", "vacuous"},
}

// crrfSelfTest —— py `self_test()`（print 落到 stdout；返回 rc）。
func crrfSelfTest(stdout io.Writer) int {
	ok := 0
	var bad []string
	P := func(s string) { fmt.Fprintln(stdout, s) }
	for _, c := range crrfSelfCases {
		isRe := strings.HasPrefix(c.rulePat, "(合成)regex:")
		pat := ""
		if strings.Contains(c.rulePat, "regex:") {
			pat = strings.SplitN(c.rulePat, "regex:", 2)[1]
		} else {
			pat = strings.ReplaceAll(c.rulePat, "(合成)", "")
		}
		rules := []crrfRule{{raw: c.rulePat, isRe: isRe, pat: pat, rep: "<合成>", note: "自检"}}
		var fixtures []crrfFixture
		if c.rel != "" {
			fixtures = []crrfFixture{{rel: c.rel, lineno: 1, line: c.line}}
		}
		got := ""
		if len(fixtures) == 0 {
			got = "vacuous"
		} else {
			hits, err := crrfJudge(rules, fixtures, 5)
			if err != "" {
				got = "error"
			} else if len(hits) > 0 {
				got = "hit"
			} else {
				got = "green"
			}
		}
		if got == c.want {
			ok++
			P(fmt.Sprintf("   ✓ %s ⇒ %s", c.name, got))
		} else {
			bad = append(bad, c.name)
			P(fmt.Sprintf("   ✗ %s ⇒ 期望 %s，实得 %s", c.name, c.want, got))
		}
	}
	// 区分度
	r := crrfRule{raw: "(合成)userdemo", isRe: false, pat: "userdemo", rep: "<合成>", note: "自检"}
	inFix, _ := crrfJudge([]crrfRule{r}, []crrfFixture{{rel: "core/x/a_test.go", lineno: 1, line: `s := "userdemo"`}}, 5)
	offFix, _ := crrfJudge([]crrfRule{r}, []crrfFixture{{rel: "core/x/a.go", lineno: 1, line: `s := "userdemo"`}}, 5)
	if len(inFix) > 0 && len(offFix) == 0 {
		ok++
		P("   ✓ 区分度：同规则在 `*_test.go` ⇒ 命中、在非测试件 ⇒ 不命中（判据有牙）")
	} else {
		bad = append(bad, "区分度")
		P(fmt.Sprintf("   ✗ 区分度：夹具里 %d 处命中、非夹具里 %d 处命中（判据没牙）", len(inFix), len(offFix)))
	}
	// 夹具隔离（成对负控）
	tmpd, terr := os.MkdirTemp("", "a4-iso-")
	if terr == nil {
		func() {
			defer os.RemoveAll(tmpd)
			rp := crrfJoin(tmpd, "rules.tsv")
			_ = os.WriteFile(rp, []byte("regex:/srv/demo\t<home>\t合成规则\n!夹具\tcore/internal/demo/*_test.go\t合成隔离\n"), 0o644)
			rules2, iso2, err2 := crrfParseRules(rp, nil)
			fx := []crrfFixture{{rel: "core/internal/demo/fixture_test.go", lineno: 1, line: `home := "/srv/demo/x.key"`}}
			var kept []crrfFixture
			for _, x := range fx {
				if !crrfIsIsolated(x.rel, iso2) {
					kept = append(kept, x)
				}
			}
			hitsOn, _ := crrfJudge(rules2, fx, 5)
			hitsOff, _ := crrfJudge(rules2, kept, 5)
			if err2 == "" && len(iso2) > 0 && len(hitsOn) > 0 && len(hitsOff) == 0 {
				ok++
				P(fmt.Sprintf("   ✓ 夹具隔离：同一件同一规则 —— 不隔离 ⇒ 命中 %d 条、隔离 ⇒ 0 条（隔离只许少判、且不许静默）", len(hitsOn)))
			} else {
				bad = append(bad, "夹具隔离")
				P(fmt.Sprintf("   ✗ 夹具隔离：err=%q iso=%v 不隔离命中 %d / 隔离命中 %d", err2, iso2, len(hitsOn), len(hitsOff)))
			}
		}()
	}
	// 机器面「只加字段」成对负控
	rulesM := []crrfRule{
		{raw: "regex:userdemo", isRe: true, pat: "userdemo", rep: "<合成>", note: "自检"},
		{raw: "regex:/srv/demo", isRe: true, pat: "/srv/demo", rep: "<合成>", note: "自检"},
	}
	fxM := []crrfFixture{
		{rel: "core/x/a_test.go", lineno: 3, line: `s := "userdemo"`},
		{rel: "core/x/b_test.go", lineno: 1, line: `u := "userdemo"`},
		{rel: "core/x/a_test.go", lineno: 9, line: `h := "/srv/demo"`},
	}
	hitsM, errM := crrfJudge(rulesM, fxM, 5)
	oldKeys := crrfJSONKeys([]string{"rules", "fixtures", "hits", "files"})
	if errM == "" && strings.Join(oldKeys, ",") == "files,fixtures,hits,rules" {
		ok++
		P("   ✓ 只加字段：老四格请求出来的键集恰 = {files, fixtures, hits, rules}（一字未改）")
	} else {
		bad = append(bad, "老四格键集")
		P(fmt.Sprintf("   ✗ 老四格键集：err=%q keys=%v", errM, oldKeys))
	}
	filesVal := crrfHitDedup(hitsM)
	if strings.Join(filesVal, ",") == "core/x/a_test.go,core/x/b_test.go" && len(filesVal) == 2 {
		ok++
		P("   ✓ `files` 仍是**去重集**（2 个件 · 排序）—— 语义一字不改")
	} else {
		bad = append(bad, "files 去重集")
		P(fmt.Sprintf("   ✗ `files` 去重集：%v", filesVal))
	}
	newKeys := crrfJSONKeys([]string{"files_dedup", "files_sum", "per_rule"})
	newSum := 0
	for _, h := range hitsM {
		newSum += len(h.files)
	}
	newDedup := crrfHitDedup(hitsM)
	if strings.Join(newKeys, ",") == "files_dedup,files_sum,per_rule" && newSum == 3 &&
		strings.Join(newDedup, ",") == strings.Join(filesVal, ",") {
		ok++
		P("   ✓ 新增三格按名请求才出现：`files_dedup` = 去重集（2）· `files_sum` = **未去重和 3**" +
			"（2 + 1 · 「逐条相加」从此机器面读）")
	} else {
		bad = append(bad, "files_sum")
		P(fmt.Sprintf("   ✗ 新三格：keys=%v files_sum=%d dedup=%v", newKeys, newSum, newDedup))
	}
	prOK := len(hitsM) == 2 && hitsM[0].total == 2 && hitsM[1].total == 1 &&
		len(hitsM[0].detail) == 2 &&
		hitsM[0].detail[0].rel == "core/x/a_test.go" && len(hitsM[0].detail[0].lines) == 1 &&
		hitsM[0].detail[0].lines[0] == (crrfLineTxt{3, `s := "userdemo"`}) &&
		hitsM[0].detail[1].rel == "core/x/b_test.go" && len(hitsM[0].detail[1].lines) == 1 &&
		hitsM[0].detail[1].lines[0] == (crrfLineTxt{1, `u := "userdemo"`})
	if prOK {
		ok++
		P("   ✓ `per_rule[]` 逐规则给出 `{rule, files, hits, lines}` · `lines` = 逐条 `{件,行,原文}`" +
			"（末例 `core/x/b_test.go:1`）")
	} else {
		bad = append(bad, "per_rule")
		P(fmt.Sprintf("   ✗ per_rule：%v", hitsM))
	}
	// 隔离体检
	perI, deadI, e0 := crrfIsolationAudit([]string{"core/internal/demo/*_test.go", "nope/*_test.go"},
		[]string{"core/internal/demo/fixture_test.go"}, 3)
	if len(perI) == 2 && perI[0] == (crrfIsoCount{"core/internal/demo/*_test.go", 1}) &&
		perI[1] == (crrfIsoCount{"nope/*_test.go", 0}) &&
		strings.Join(deadI, ",") == "nope/*_test.go" && !e0 {
		ok++
		P("   ✓ 隔离体检：逐条 glob 记件数 · 一件都没对上的条目**点名**（登记了却没生效 ⇒ 看得见）")
	} else {
		bad = append(bad, "隔离体检")
		P(fmt.Sprintf("   ✗ 隔离体检：per=%v dead=%v eaten=%s", perI, deadI, crrfPyBool(e0)))
	}
	_, _, eatenAll := crrfIsolationAudit([]string{"*_test.go"}, []string{"a_test.go", "b_test.go"}, 2)
	_, _, eatenHalf := crrfIsolationAudit([]string{"*_test.go"}, []string{"a_test.go"}, 2)
	if eatenAll && !eatenHalf {
		ok++
		P(fmt.Sprintf("   ✓ 隔离不许吃光：件面全被隔离 ⇒ 拒判（吃光=%s）· 只隔离一半 ⇒ 照判（吃光=%s）", crrfPyBool(eatenAll), crrfPyBool(eatenHalf)))
	} else {
		bad = append(bad, "隔离吃光")
		P(fmt.Sprintf("   ✗ 隔离吃光：全吃=%s 半吃=%s", crrfPyBool(eatenAll), crrfPyBool(eatenHalf)))
	}
	concl := "全过"
	if len(bad) > 0 {
		concl = "**不过**"
	}
	P(fmt.Sprintf("自检结论: %s（断言 %d 条 · 失败 %d 条）", concl, ok+len(bad), len(bad)))
	if len(bad) == 0 {
		return 0
	}
	return 2
}

// crrfJSONKeys —— 只用于自检断言：按 crrfJSONFields 固定序列出 `want` 里的在场键。
func crrfJSONKeys(want []string) []string {
	w := map[string]bool{}
	for _, x := range want {
		w[x] = true
	}
	var out []string
	for _, f := range crrfJSONFields {
		if w[f] {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// crrfPyBool —— py `True`/`False`（自检正文用）。
func crrfPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// crrfHitDedup —— 自检用：命中件去重集（`json_payload` 里 `files`/`files_dedup` 同值）。
func crrfHitDedup(hits []crrfHit) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, h := range hits {
		for _, f := range h.files {
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	sort.Strings(out)
	return out
}

// ── 主流程 ────────────────────────────────────────────────────────────────

// checkReplaceRuleFixturesGateMain —— 入口（与 py `main()` 逐字同构）。
func checkReplaceRuleFixturesGateMain(stdout, stderr io.Writer, argv []string) int {
	args, helpText, helpWanted, err := crrfParseArgs(argv)
	if err != nil {
		fmt.Fprint(stderr, crrfUsage)
		fmt.Fprintf(stderr, "%s: error: %s\n", crrfProg, err.Error())
		return 2
	}
	if helpWanted {
		fmt.Fprint(stdout, helpText)
		return 0
	}

	// 缺口 Q-150：`--json` 字段名打错 ⇒ 用法错 rc=2（✗ 不许静默吐 `{}`）。
	var want []string
	if args.json != "" {
		for _, f := range strings.Split(args.json, ",") {
			s := strings.TrimSpace(f)
			if s != "" {
				want = append(want, s)
			}
		}
		var badF []string
		for _, f := range want {
			if !crrfInFields(f) {
				badF = append(badF, f)
			}
		}
		if len(badF) > 0 {
			quoted := make([]string, len(badF))
			for i, b := range badF {
				quoted[i] = "`" + b + "`"
			}
			fmt.Fprintf(stdout, "⛔ `--json` 不认的字段名：%s\n", strings.Join(quoted, " · "))
			all := make([]string, len(crrfJSONFields))
			for i, f := range crrfJSONFields {
				all[i] = "`" + f + "`"
			}
			fmt.Fprintf(stdout, "   可选字段表：%s\n", strings.Join(all, " · "))
			fmt.Fprint(stdout, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开\n")
			return 2
		}
	}

	if args.listRules {
		fmt.Fprint(stdout, crrfListRules())
		return 0
	}

	if args.selfTest {
		fmt.Fprintln(stdout, "── 替换规则级机检 · 自检（成对负控 · 合成夹具）──")
		return crrfSelfTest(stdout)
	}

	rootArg := args.root
	if rootArg == "" {
		rootArg = crrfDefaultRepo() // 见 (a)
	}
	root := crrfAbs(rootArg)
	rulesPath := args.rules
	if rulesPath == "" {
		rulesPath = crrfPyJoin(root, crrfRulesRel)
	}
	wlPath := args.whitelist
	if wlPath == "" {
		wlPath = crrfPyJoin(root, crrfWhitelistRel)
	}

	rules, isolated, perr := crrfParseRules(rulesPath, args.extraRules)
	if perr != "" {
		fmt.Fprintf(stdout, "⛔ %s\n", perr)
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）：缺件/不可解析 ≠ 绿\n")
		return 2
	}
	if len(rules) == 0 {
		fmt.Fprintf(stdout, "⛔ 空转：规则表里**一条真规则都没有**（%s）—— 没有规则就没有可判的东西\n", rulesPath)
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）\n")
		return 2
	}

	prefixes, ok := crrfWhitelistPrefixes(wlPath)
	if !ok {
		fmt.Fprintf(stdout, "⛔ 白名单缺件：%s（扫描面定义不出来 ⇒ 判据不可判）\n", crrfWhitelistRel)
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）\n")
		return 2
	}
	if len(prefixes) == 0 {
		fmt.Fprintf(stdout, "⛔ 空转：白名单里**一条递归前缀都没有**（%s）\n", wlPath)
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）\n")
		return 2
	}

	fixtures, isoFiles, scanned := crrfCollectFixtures(root, prefixes, isolated)
	fmt.Fprintln(stdout, "── 替换规则级机检：命中件不许是测试夹具（§十三 M8）──")
	fmt.Fprintf(stdout, "   树：%s\n", root)
	relRules := rulesPath
	if strings.HasPrefix(rulesPath, root) {
		relRules = crrfRelPath(rulesPath, root)
	}
	nRe, nLit := 0, 0
	for _, r := range rules {
		if r.isRe {
			nRe++
		} else {
			nLit++
		}
	}
	extraSuffix := ""
	if len(args.extraRules) > 0 {
		extraSuffix = fmt.Sprintf(" · 含命令行合成规则 %d 条", len(args.extraRules))
	}
	fmt.Fprintf(stdout, "   规则面：%s ⇒ **%d 条**规则（%d 条正则 · %d 条字面量%s）\n",
		relRules, len(rules), nRe, nLit, extraSuffix)
	if len(isolated) > 0 {
		fmt.Fprintf(stdout, "   夹具隔离：%d 条 glob（%s）⇒ 排除 **%d 个** `*_test.go` 出件面\n",
			len(isolated), strings.Join(isolated, " · "), len(isoFiles))
		for _, rel := range isoFiles {
			fmt.Fprintf(stdout, "      ── %s\n", rel)
		}
	} else {
		fmt.Fprintf(stdout, "   夹具隔离：**0 条** glob —— 规则表里没有生效的 `!夹具` 条目"+
			" ⇒ 隔离机制在场但**空转**，件面 %d 个夹具**全部在判据内**\n", scanned)
	}
	perIso, _, eatenIso := crrfIsolationAudit(isolated, isoFiles, scanned)
	for _, pc := range perIso {
		mark := ""
		if pc.n == 0 {
			mark = "  ⚠ 一件都没对上（登记了却没生效）"
		}
		fmt.Fprintf(stdout, "      · 隔离条目 `!夹具 %s` ⇒ 对上 %d 件%s\n", pc.pat, pc.n, mark)
	}
	if eatenIso {
		fmt.Fprintf(stdout, "⛔ 夹具隔离把整个件面吃光（%d/%d 件全被隔离）⇒ 判据空转\n", len(isoFiles), scanned)
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）：**隔离吃光 ≠ 绿** —— 那正是本件要防的「把红写进白名单」\n")
		return 2
	}
	distinct := map[string]bool{}
	for _, f := range fixtures {
		distinct[f.rel] = true
	}
	fmt.Fprintf(stdout, "   件面：白名单 %d 条前缀下的测试夹具 ⇒ **%d 个** `*_test.go`（%d 行）\n",
		len(prefixes), len(distinct), len(fixtures))
	if len(fixtures) == 0 {
		fmt.Fprint(stdout, "⛔ 空转：扫描面一件 `*_test.go` 都没扫到（扫描面写错 or 树不对）\n")
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）：**空转 ≠ 绿**\n")
		return 2
	}

	hits, jerr := crrfJudge(rules, fixtures, args.maxExamples)
	if jerr != "" {
		fmt.Fprintf(stdout, "⛔ %s\n", jerr)
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）\n")
		return 2
	}

	if len(want) > 0 {
		fmt.Fprintln(stdout, crrfJSONPayload(want, len(rules), len(fixtures), hits))
	}

	if len(hits) == 0 {
		fmt.Fprintf(stdout, "⇒ 全绿（rc=0）：%d 条规则一条都没命中测试夹具\n", len(rules))
		return 0
	}

	hdistinct := map[string]bool{}
	for _, h := range hits {
		for _, f := range h.files {
			hdistinct[f] = true
		}
	}
	fmt.Fprintf(stdout, "   ✗ 命中：**%d 条**规则打到了测试夹具（共 %d 个件）\n", len(hits), len(hdistinct))
	for _, h := range hits {
		fmt.Fprintf(stdout, "   ── 规则：%s\n", crrfRuneCut(crrfPyStrip(h.raw), 140))
		exFiles := map[string]bool{}
		for _, e := range h.ex {
			exFiles[e.rel] = true
		}
		fmt.Fprintf(stdout, "      命中件 %d 个 · 命中行 %d 处（列前 %d 件）\n", len(h.files), h.total, len(exFiles))
		for _, e := range h.ex {
			fmt.Fprintf(stdout, "      %s:%d: %s\n", e.rel, e.ln, e.txt)
		}
	}
	fmt.Fprint(stdout, "⇒ 有命中（rc=1）：**替换规则命中件不许是测试夹具** —— 规则改的是公开面夹具的"+
		"字面量/长度假设，本机看不出来、公开树一 clone 就红\n")
	return 1
}

// crrfInFields —— `--json` 字段名是否在真源表内。
func crrfInFields(f string) bool {
	for _, x := range crrfJSONFields {
		if x == f {
			return true
		}
	}
	return false
}
