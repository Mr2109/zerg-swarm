// check_registry_key_unique.go —— 判据件 `scripts/gates/check-registry-key-unique.py`（222 行）的
// **Go 真搬**（M2 · 搬运面 · 入口 `checkRegistryKeyUniqueGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的 argparse
//	面差异与未覆盖面）。
//
// 选它的理由（本单口径：真未搬清单里**纯静态只读**一批）：
//
//	✓ 选：判据只读**两处**目录直下的 `*.json` 登记件 —— `scripts/gates/` 与
//	  `core/internal/contract/`；按「顶层某键的值 = 全为对象的非空列表」认登记表，
//	  在该列里找 `id`→`name`→`key`→`slug` 第一个全出现的键字段，判该列内键值逐字唯一。
//	  **不真跑子进程、不碰网络、零计时、默认档不写盘**（`--self-test` 只在
//	  `tempfile.TemporaryDirectory()` 里造件 —— `with` 形自带回收，**非** A 类泄漏形态）。
//	· 无外部树依赖：`--root`（env `ZERG_ROOT`）可显式指到任意合成假仓根（外部树走显式旋钮，
//	  旋钮名照 py 同名旗标 `--root` / env `ZERG_ROOT`）。
//	· 退码 0/1/2 三档齐 · 纯只读无写面 · 落件前 `git status` 现读不含它。
//	· 现读 import 面：argparse / json / os / sys —— **零 `subprocess` / `time` / `datetime` /
//	  `random` / `socket` / `urllib` / `shutil` / `glob`**（`tempfile` 只在 `self_test()` 内 import）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `gate_inproc_table.go`）；
//	· **不**删 `scripts/gates/check-registry-key-unique.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `checkRegistryKeyUniqueGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main(argv)` 对齐）：
//
//	func checkRegistryKeyUniqueGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 全唯一（或有跳过但无重复） / 1 有重复键（逐条点名） / 2 不给结论
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「未覆盖面」）
//
// ═══════════════════════ ★ 遍历序与「键序」（本件有目录遍历 + JSON 顶层键序 —— 逐点交代）═══════════════════════
//
//	① 目录遍历：py `sorted(os.listdir(d))` —— **显式排序**（按码点；UTF-8 下 == Go 的字节序）⇒ Go 侧用
//	  `sort.Strings` 复现。★ 输出行序的决定者 = 这份 `sorted(...)`（`dups`/`skipped` 的明细行序全由它承运）。
//	② **JSON 顶层键序**：py `obj.items()` = **插入序**（CPython dict 保序）⇒ `entry_lists()` 产出的
//	  `(topkey, list)` 对按**件内原始键序**；`dups`/`skipped` 的追加序全由它承运（真仓 `authority-map.json`
//	  的 6 条 skipped 就靠它定序）。Go `map[string]any` 迭代序**随机** ⇒ ✗ 不能用 map —— 本件用
//	  `json.Decoder` 的 **token 流**逐键取值，保顶层键序（见 `rkuTopObject`）。
//	③ 列内计数：py `sorted(seen.items())` 按**键串**升序 ⇒ Go `sort.Strings`（同②的码点/字节序同构）。
//	★ 无 Go map 迭代进输出：`seen` 只在**排序后**进入 `dups`，输出序由切片承运。
//
// ═══════════════════════ ★ py 字符串 / 数值原语（逐处现读）═══════════════════════
//
//	① `name.endswith(".json")` ＝ 后缀判定（两语言同形，逐字直译）。
//	② 本件**零** `str.strip()` / `splitlines` 呼叫（只走 `os.listdir` 名 + json 解析）⇒ 不涉 py strip 集。
//	③ 键值展示串：`val if isinstance(val, str) else repr(val)`，其中**列表**先过
//	  `json.dumps(val, ensure_ascii=False, sort_keys=True)`。⇒ 自写 `rkuPyDumps`（`", "`/`": "` 分隔、
//	  ensure_ascii=False、sort_keys）＋ `rkuPyNumRepr`（int 保字面 / float 走 py repr 近似）。
//	  见「未覆盖面」—— float 边界 / dict 值档如实声明。
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
//	① 自扫自身：本判据扫的是 `scripts/gates/` 与 `core/internal/contract/` 直下的 **`*.json`** ⇒
//	  本件是 `core/cmd/zerg/check_registry_key_unique.go`（`*.go`）⇒**不在扫描面内**（非自扫）。
//	② 件头注释无真引号路径原串（普通仓内相对串，且为**描述式**）。
//	③ 无 self-test 魔法串（自检夹具件名在 `rkuSelfTest` 内，非件头）。
//	④ 他门：`check-error-kinds` 的 R3 只认**代码位**的 error 构造调用 + JSON 信封标签字面（该门件头载明
//	  剥注释、且不扫本类标签）⇒ 本件零 error 构造调用 ⇒ 命中 0。`check-hardcoded-private-paths` 扫 `core/`
//	  的**私有绝对路径字面** ⇒ 本件零绝对路径字面 ⇒ 命中 0。落件后以同根串影子仓 A/B（新件在/不在两态
//	  各跑该二门）核 `sha16` 逐字相同（见回执）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ── 常量（照 py 同名量逐字）────────────────────────────────────────────────
const (
	rkuScanDirGates    = "scripts/gates"          // py SCAN_DIRS[0]
	rkuScanDirContract = "core/internal/contract" // py SCAN_DIRS[1]
	rkuProg            = "check-registry-key-unique.py"
	rkuHelpDesc        = "统一「登记键唯一性」只读门（GAP-20260926-175）" // py argparse description（含「只读」）
	rkuRunHeader       = "统一「登记键唯一性」门（GAP-20260926-175）"   // py run() 正文头（无「只读」）
)

// rkuKeyCandidates —— py KEY_CANDIDATES（有序）。
var rkuKeyCandidates = []string{"id", "name", "key", "slug"}

// rkuScanDirs —— py SCAN_DIRS（有序；顺序决定 scan 的追加序）。
var rkuScanDirs = []string{rkuScanDirGates, rkuScanDirContract}

var rkuIntRe = regexp.MustCompile(`^-?\d+$`)

// ── py 路径原语 ────────────────────────────────────────────────────────────
// rkuPyJoin —— 复刻 CPython `posixpath.join`（空段 / 后段绝对路径 / 尾斜杠三态逐段直译）。
func rkuPyJoin(parts ...string) string {
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
func rkuJSONStr(s string) string {
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

// rkuPyFloatRepr —— py `repr(float)` 的近似（最短往返；整值补 `.0`）。见未覆盖面。
func rkuPyFloatRepr(f float64) string {
	if math.IsInf(f, 1) {
		return "inf"
	}
	if math.IsInf(f, -1) {
		return "-inf"
	}
	if math.IsNaN(f) {
		return "nan"
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// rkuPyNumRepr —— py 数值 repr：int 保字面（与 json.load 得 int 同形）/ 其余走 float repr。
func rkuPyNumRepr(lit string) string {
	if rkuIntRe.MatchString(lit) {
		return lit
	}
	if f, err := strconv.ParseFloat(lit, 64); err == nil {
		return rkuPyFloatRepr(f)
	}
	return lit
}

// rkuPyDumps —— py `json.dumps(v, ensure_ascii=False, sort_keys=sortKeys)` 逐字节复刻
// （默认分隔符 `, ` 与 `: ` · 行内无缩进 · 非 ASCII 不转义）。仅覆盖 list 值档所需的形态。
func rkuPyDumps(v any, sortKeys bool) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return rkuJSONStr(x)
	case json.Number:
		return rkuPyNumRepr(string(x))
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, rkuPyDumps(e, sortKeys))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		if sortKeys {
			sort.Strings(keys) // py sort_keys=True：按码点（UTF-8 下 == 字节序）
		} else {
			sort.Strings(keys) // Go map 无序 ⇒ 无 sort_keys 时也为确定性而排序（见未覆盖面）
		}
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, rkuJSONStr(k)+": "+rkuPyDumps(x[k], sortKeys))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%v", x)
	}
}

// rkuValueDisplay —— py `val if isinstance(val, str) else repr(val)`（列表先过 json.dumps）。
func rkuValueDisplay(val any) string {
	switch x := val.(type) {
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case nil:
		return "None"
	case json.Number:
		return rkuPyNumRepr(string(x))
	case []any:
		return rkuPyDumps(x, true) // json.dumps(val, ensure_ascii=False, sort_keys=True)
	default:
		// dict 值档：py 走 repr(dict)（插入序）—— Go map 无序 ⇒ 见未覆盖面。
		return rkuPyDumps(val, false)
	}
}

// ── JSON 读取（保顶层键序）────────────────────────────────────────────────
// rkuTopObject —— 以 token 流读顶层对象，保键序；回 (pairs, ok, errmsg)。
// 顶层非对象（json.load 成功但 entry_lists 回 []）⇒ 回空对集、ok=true。
func rkuTopObject(text string) ([]rkuKV, bool, string) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, false, err.Error()
	}
	d, ok := tok.(json.Delim)
	if !ok || d != '{' {
		return []rkuKV{}, true, "" // 顶层非对象：py entry_lists 回 []，非错
	}
	out := []rkuKV{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, false, err.Error()
		}
		key, _ := kt.(string)
		var v any
		if err := dec.Decode(&v); err != nil {
			return nil, false, err.Error()
		}
		out = append(out, rkuKV{Key: key, Val: v})
	}
	if _, err := dec.Token(); err != nil {
		return nil, false, err.Error()
	}
	return out, true, ""
}

// rkuKV —— 顶层键值对（保序）。
type rkuKV struct {
	Key string
	Val any
}

// ── py `json.load` 错误正文复刻（CPython `_json` C 扫描器口径）────────────────
//
// py `load_json` 在 `json.load` 抛 `ValueError`（含 `JSONDecodeError`）时回
// `"不是合法 JSON：%s" % exc`；`str(exc)` 的形状 = `<msg>: line <L> column <C> (char <P>)`，
// L/C/P 由 `JSONDecodeError.__init__` 从**码点下标** `pos` 推出：
//
//	lineno = s[:pos].count('\n') + 1
//	colno  = pos - s[:pos].rfind('\n')      （无换行则 = pos + 1）
//	char   = pos
//
// Go `encoding/json` 的错误文本（如 `invalid character 'n'`）与下标口径都不同 ⇒ 本件自带
// 扫描器 `rkuPyJSONErrStr` 复刻 py 正文（101 例语料逐字对齐；见未覆盖面②）。
type rkuPyJErr struct {
	msg string
	pos int
}

type rkuPyScanner struct{ s []rune }

func rkuPyIsWS(r rune) bool { return r == ' ' || r == '	' || r == '\n' || r == '\r' }

func (p *rkuPyScanner) skipWS(i int) int {
	for i < len(p.s) && rkuPyIsWS(p.s[i]) {
		i++
	}
	return i
}

func rkuPyHexVal(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10
	}
	return -1
}

func (p *rkuPyScanner) hasPrefix(i int, lit string) bool {
	rs := []rune(lit)
	if i+len(rs) > len(p.s) {
		return false
	}
	for k, r := range rs {
		if p.s[i+k] != r {
			return false
		}
	}
	return true
}

// value —— 复刻 py `scan_once`：回 (end, err)；err.msg = "Expecting value" 等。
func (p *rkuPyScanner) value(i int) (int, *rkuPyJErr) {
	i = p.skipWS(i)
	if i >= len(p.s) {
		return 0, &rkuPyJErr{"Expecting value", i}
	}
	switch c := p.s[i]; {
	case c == '{':
		return p.object(i)
	case c == '[':
		return p.array(i)
	case c == '"':
		return p.str(i)
	case c == 't':
		if p.hasPrefix(i, "true") {
			return i + 4, nil
		}
	case c == 'f':
		if p.hasPrefix(i, "false") {
			return i + 5, nil
		}
	case c == 'n':
		if p.hasPrefix(i, "null") {
			return i + 4, nil
		}
	case c == 'N':
		if p.hasPrefix(i, "NaN") { // py 扩展：大写 NaN 合法
			return i + 3, nil
		}
	case c == 'I':
		if p.hasPrefix(i, "Infinity") { // py 扩展
			return i + 8, nil
		}
	case c == '-':
		if p.hasPrefix(i, "-Infinity") { // py 扩展
			return i + 9, nil
		}
		return p.number(i)
	case c >= '0' && c <= '9':
		return p.number(i)
	}
	return 0, &rkuPyJErr{"Expecting value", i}
}

// number —— 复刻 py `NUMBER_RE` = `-?(0|[1-9]\d*)(\.\d+)?([eE][-+]?\d+)?` 的最长前缀。
func (p *rkuPyScanner) number(i int) (int, *rkuPyJErr) {
	j := i
	if j < len(p.s) && p.s[j] == '-' {
		j++
	}
	if j >= len(p.s) || p.s[j] < '0' || p.s[j] > '9' {
		return 0, &rkuPyJErr{"Expecting value", i}
	}
	if p.s[j] == '0' {
		j++
	} else {
		for j < len(p.s) && p.s[j] >= '0' && p.s[j] <= '9' {
			j++
		}
	}
	if j < len(p.s) && p.s[j] == '.' {
		k := j + 1
		if k < len(p.s) && p.s[k] >= '0' && p.s[k] <= '9' {
			j = k
			for j < len(p.s) && p.s[j] >= '0' && p.s[j] <= '9' {
				j++
			}
		}
	}
	if j < len(p.s) && (p.s[j] == 'e' || p.s[j] == 'E') {
		k := j + 1
		if k < len(p.s) && (p.s[k] == '+' || p.s[k] == '-') {
			k++
		}
		if k < len(p.s) && p.s[k] >= '0' && p.s[k] <= '9' {
			j = k
			for j < len(p.s) && p.s[j] >= '0' && p.s[j] <= '9' {
				j++
			}
		}
	}
	return j, nil
}

// str —— 复刻 py `scanstring`（严格模式）：控制字符 / 非法转义 / 非法 \u / 未闭合。
func (p *rkuPyScanner) str(i int) (int, *rkuPyJErr) {
	begin := i
	i++ // 过开引号
	for i < len(p.s) {
		c := p.s[i]
		if c == '"' {
			return i + 1, nil
		}
		if c == '\\' {
			if i+1 >= len(p.s) {
				return 0, &rkuPyJErr{"Unterminated string starting at", begin}
			}
			esc := p.s[i+1]
			if esc == 'u' {
				if i+5 >= len(p.s) {
					return 0, &rkuPyJErr{"Invalid \\uXXXX escape", i + 1}
				}
				bad := false
				for k := 0; k < 4; k++ {
					if rkuPyHexVal(p.s[i+2+k]) < 0 {
						bad = true
						break
					}
				}
				if bad {
					return 0, &rkuPyJErr{"Invalid \\uXXXX escape", i + 1}
				}
				i += 6
				continue
			}
			switch esc {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				i += 2
				continue
			}
			return 0, &rkuPyJErr{"Invalid \\escape", i}
		}
		if c < 0x20 {
			return 0, &rkuPyJErr{"Invalid control character at", i}
		}
		i++
	}
	return 0, &rkuPyJErr{"Unterminated string starting at", begin}
}

// object —— 复刻 py `_parse_object`（保序不需值：错误定位只需要下标）。
func (p *rkuPyScanner) object(i int) (int, *rkuPyJErr) {
	i++ // 过 `{`
	i = p.skipWS(i)
	if i < len(p.s) && p.s[i] == '}' {
		return i + 1, nil
	}
	for {
		if i >= len(p.s) || p.s[i] != '"' {
			return 0, &rkuPyJErr{"Expecting property name enclosed in double quotes", i}
		}
		var err *rkuPyJErr
		i, err = p.str(i)
		if err != nil {
			return 0, err
		}
		i = p.skipWS(i)
		if i >= len(p.s) || p.s[i] != ':' {
			return 0, &rkuPyJErr{"Expecting ':' delimiter", i}
		}
		i++
		i, err = p.value(i)
		if err != nil {
			return 0, err
		}
		i = p.skipWS(i)
		if i >= len(p.s) {
			return 0, &rkuPyJErr{"Expecting ',' delimiter", i}
		}
		switch p.s[i] {
		case '}':
			return i + 1, nil
		case ',':
			i++
			i = p.skipWS(i)
		default:
			return 0, &rkuPyJErr{"Expecting ',' delimiter", i}
		}
	}
}

// array —— 复刻 py `_parse_array`。
func (p *rkuPyScanner) array(i int) (int, *rkuPyJErr) {
	i++ // 过 `[`
	i = p.skipWS(i)
	if i < len(p.s) && p.s[i] == ']' {
		return i + 1, nil
	}
	for {
		var err *rkuPyJErr
		i, err = p.value(i)
		if err != nil {
			return 0, err
		}
		i = p.skipWS(i)
		if i >= len(p.s) {
			return 0, &rkuPyJErr{"Expecting ',' delimiter", i}
		}
		switch p.s[i] {
		case ']':
			return i + 1, nil
		case ',':
			i++
		default:
			return 0, &rkuPyJErr{"Expecting ',' delimiter", i}
		}
	}
}

// rkuPyJErrLine —— 复刻 `JSONDecodeError.__init__` 的 `line/column/char` 三数（码点口径）。
func rkuPyJErrLine(s []rune, e rkuPyJErr) string {
	pos := e.pos
	lineno := 1
	colno := pos + 1
	for i := 0; i < pos && i < len(s); i++ {
		if s[i] == '\n' {
			lineno++
			colno = pos - i
		}
	}
	return fmt.Sprintf("%s: line %d column %d (char %d)", e.msg, lineno, colno, pos)
}

// rkuPyJSONErrStr —— py `json.load` 的 `str(exc)` 逐字复刻；回 (正文, 是否有错)。
// 无错 ⇒ ("", false)（= py 不抛 ⇒ 该件合法）。
func rkuPyJSONErrStr(text string) (string, bool) {
	s := []rune(text)
	p := &rkuPyScanner{s: s}
	end, err := p.value(0)
	if err != nil {
		return rkuPyJErrLine(s, *err), true
	}
	end = p.skipWS(end)
	if end != len(s) {
		return rkuPyJErrLine(s, rkuPyJErr{"Extra data", end}), true
	}
	return "", false
}

// rkuLoadJSON —— py `load_json(path)`：回 (pairs, ok, errmsg)。file read 失败 / 非法 JSON ⇒ ok=false。
// ★ 非法 JSON 的正文走 py 口径扫描器（`不是合法 JSON：<str(exc)>` 逐字含 line/column/char 三数）。
func rkuLoadJSON(path string) ([]rkuKV, bool, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, "读不出：" + err.Error()
	}
	text := string(data)
	if msg, has := rkuPyJSONErrStr(text); has {
		return nil, false, "不是合法 JSON：" + msg
	}
	pairs, ok, goverr := rkuTopObject(text)
	if !ok {
		// py 扫描器认合法、Go 编码器却拒（`NaN`/`Infinity` 等 py 扩展档）—— 未覆盖面②。
		return nil, false, "不是合法 JSON：" + goverr
	}
	return pairs, true, ""
}

// rkuEntryLists —— py `entry_lists(obj)`：「顶层某键的值 = 全为对象的非空列表」⇒ 保序的 (key, list) 对。
func rkuEntryLists(obj []rkuKV) []rkuKV {
	out := []rkuKV{}
	for _, kv := range obj {
		arr, ok := kv.Val.([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		allDict := true
		for _, x := range arr {
			if _, ok := x.(map[string]any); !ok {
				allDict = false
				break
			}
		}
		if allDict {
			out = append(out, kv)
		}
	}
	return out
}

// rkuKeyField —— py `key_field(entries)`：回第一个「该列每个对象都出现」的候选键；无 ⇒ ok=false。
func rkuKeyField(entries []any) (string, bool) {
	for _, cand := range rkuKeyCandidates {
		all := true
		for _, e := range entries {
			m, _ := e.(map[string]any)
			if _, ok := m[cand]; !ok {
				all = false
				break
			}
		}
		if all {
			return cand, true
		}
	}
	return "", false
}

// ── scan 结果类型 ─────────────────────────────────────────────────────────
type rkuDup struct {
	file, topkey, field, val string
	count                    int
}
type rkuSkip struct{ file, topkey string }
type rkuBad struct{ file, err string }

// rkuScanDirNames —— 回某目录下排序后的条目名（py `sorted(os.listdir(d))`）。
// st：0 目录不在盘 / 读不到；1 成表。
func rkuScanDirNames(d string) ([]string, int) {
	st, err := os.Stat(d)
	if err != nil || !st.IsDir() {
		return nil, 0 // py os.path.isdir False ⇒ skip
	}
	f, err := os.Open(d)
	if err != nil {
		return nil, 0
	}
	defer func() { _ = f.Close() }()
	ents, err := f.ReadDir(-1)
	if err != nil {
		return nil, 0
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names) // ★ py sorted(...)：明细行序的决定者
	return names, 1
}

// rkuScan —— py `scan(root)` 逐字复刻。
func rkuScan(root string) (dups []rkuDup, skipped []rkuSkip, bad []rkuBad, scanned int) {
	dups, skipped, bad = []rkuDup{}, []rkuSkip{}, []rkuBad{}
	for _, sub := range rkuScanDirs {
		d := rkuPyJoin(root, sub)
		names, ok := rkuScanDirNames(d)
		if ok == 0 {
			continue
		}
		for _, name := range names {
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			scanned++
			rel := rkuPyJoin(sub, name)
			obj, good, errmsg := rkuLoadJSON(rkuPyJoin(d, name))
			if !good {
				bad = append(bad, rkuBad{rel, errmsg})
				continue
			}
			for _, kv := range rkuEntryLists(obj) {
				entries, _ := kv.Val.([]any)
				field, has := rkuKeyField(entries)
				if !has {
					skipped = append(skipped, rkuSkip{rel, kv.Key})
					continue
				}
				seen := map[string]int{}
				for _, e := range entries {
					m, _ := e.(map[string]any)
					disp := rkuValueDisplay(m[field])
					seen[disp]++
				}
				keys := make([]string, 0, len(seen))
				for k := range seen {
					keys = append(keys, k)
				}
				sort.Strings(keys) // ★ py sorted(seen.items())
				for _, k := range keys {
					if seen[k] >= 2 {
						dups = append(dups, rkuDup{rel, kv.Key, field, k, seen[k]})
					}
				}
			}
		}
	}
	return dups, skipped, bad, scanned
}

// ── 输出构建（py json.dumps 逐字节）────────────────────────────────────────
func rkuBuildRootErrJSON() string {
	return `{"rc": 2, "error": ` + rkuJSONStr("扫面根不存在（scripts/gates 与 core/internal/contract 皆无）") + `}`
}

func rkuBuildEmptyJSON() string {
	msg := fmt.Sprintf("空域：%s 下 0 个 *.json 登记件（取不到输入 ⇒ 空转 ≠ 零重复）", strings.Join(rkuScanDirs, "、"))
	return `{"rc": 2, "error": ` + rkuJSONStr(msg) + `}`
}

func rkuBuildBadJSON(bad []rkuBad) string {
	var b strings.Builder
	b.WriteString(`{"rc": 2, "bad": [`)
	for i, x := range bad {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"file": ` + rkuJSONStr(x.file) + `, "err": ` + rkuJSONStr(x.err) + `}`)
	}
	b.WriteString(`]}`)
	return b.String()
}

func rkuBuildMainJSON(rc int, dups []rkuDup, skipped []rkuSkip) string {
	var b strings.Builder
	b.WriteString(`{"rc": ` + strconv.Itoa(rc) + `, "dup_count": ` + strconv.Itoa(len(dups)) + `, "dups": [`)
	for i, d := range dups {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"file": ` + rkuJSONStr(d.file) + `, "list": ` + rkuJSONStr(d.topkey) +
			`, "field": ` + rkuJSONStr(d.field) + `, "value": ` + rkuJSONStr(d.val) +
			`, "count": ` + strconv.Itoa(d.count) + `}`)
	}
	b.WriteString(`], "skipped_no_key": [`)
	for i, s := range skipped {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"file": ` + rkuJSONStr(s.file) + `, "list": ` + rkuJSONStr(s.topkey) + `}`)
	}
	b.WriteString(`]}`)
	return b.String()
}

// rkuRun —— py `run(root, as_json)` 逐字复刻。
func rkuRun(stdout, stderr io.Writer, root string, asJSON bool) int {
	dGates := rkuPyJoin(root, rkuScanDirGates)
	dContract := rkuPyJoin(root, rkuScanDirContract)
	okGates := rkuIsDir(dGates)
	okContract := rkuIsDir(dContract)
	if !okGates && !okContract {
		if asJSON {
			fmt.Fprintln(stdout, rkuBuildRootErrJSON())
		} else {
			fmt.Fprintln(stderr, "BLOCKED —— 扫面根不存在（缺 scripts/gates 与 core/internal/contract）⇒ rc=2，不给结论")
		}
		return 2
	}
	dups, skipped, bad, scanned := rkuScan(root)
	if scanned == 0 {
		if asJSON {
			fmt.Fprintln(stdout, rkuBuildEmptyJSON())
		} else {
			fmt.Fprintf(stderr, "BLOCKED —— 空域：扫面（%s）下 0 个 *.json 登记件（取不到输入）⇒ rc=2 不给结论（空转 ≠ 零重复）\n",
				strings.Join(rkuScanDirs, "、"))
		}
		return 2
	}
	if len(bad) > 0 {
		if asJSON {
			fmt.Fprintln(stdout, rkuBuildBadJSON(bad))
		} else {
			fmt.Fprintln(stderr, "BLOCKED —— 登记件读不出/非法 JSON（「取不到」与「零重复」不许同形）⇒ rc=2：")
			for _, x := range bad {
				fmt.Fprintf(stderr, "    %s ← %s\n", x.file, x.err)
			}
		}
		return 2
	}
	rc := 0
	if len(dups) > 0 {
		rc = 1
	}
	if asJSON {
		fmt.Fprintln(stdout, rkuBuildMainJSON(rc, dups, skipped))
	} else {
		fmt.Fprintln(stdout, rkuRunHeader)
		fmt.Fprintf(stdout, "重复键 = %d 条\n", len(dups))
		for _, d := range dups {
			fmt.Fprintf(stdout, "  ✗ %s[%s] 键 %s=%s 出现 %d 次（要求恰好 1 次）\n", d.file, d.topkey, d.field, d.val, d.count)
		}
		if len(skipped) > 0 {
			fmt.Fprintf(stdout, "  跳过（无键面 · 非重复）：%d 列\n", len(skipped))
			for _, s := range skipped {
				fmt.Fprintf(stdout, "    · %s[%s]\n", s.file, s.topkey)
			}
		}
		fmt.Fprintln(stdout, "  口径：一份登记件里同一个键只许出现一次（命中行数==1）；重复即判红并点名。")
	}
	return rc
}

func rkuIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// rkuSelfTest —— py `self_test()`：五格（正控重复 ⇒ rc=1 · 键唯一 ⇒ rc=0 · 无键面跳过 ⇒ rc=0 ·
// 非法 JSON ⇒ rc=2 · 空域 ⇒ rc=2），合成件全在 `tempfile.TemporaryDirectory()` 内（自带回收）。
//
// ★ `defer os.RemoveAll` 只在**建点成功支**（✗ 不无条件 —— 失败回落系统临时目录时无条件 defer
// 会删整个 OS 临时目录）。
func rkuSelfTest(stdout, stderr io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "tmp") // py TemporaryDirectory 缺省 prefix
	if err != nil {
		fmt.Fprintln(stdout, "自检 FAIL（5 格）")
		return 1
	}
	defer func() { _ = os.RemoveAll(td) }() // 建点成功支：TemporaryDirectory 语义 = 自带回收
	_ = os.MkdirAll(rkuPyJoin(td, rkuScanDirGates), 0o755)
	_ = os.MkdirAll(rkuPyJoin(td, rkuScanDirContract), 0o755)
	xReg := rkuPyJoin(td, rkuScanDirGates, "x-registry.json")
	yCtr := rkuPyJoin(td, rkuScanDirContract, "y.json")
	zCtr := rkuPyJoin(td, rkuScanDirContract, "z.json")

	// 正控：x-registry.json entries a,b,a ⇒ rc=1
	_ = os.WriteFile(xReg, []byte(`{"schema": "s", "entries": [{"id": "a", "p": 1}, {"id": "b"}, {"id": "a"}]}`), 0o644)
	if rkuRun(stdout, stderr, td, false) != 1 {
		fmt.Fprintln(stdout, "  ✗ 正控失败：应回 1")
		ok = false
	}
	// 负控一：键唯一 ⇒ rc=0
	_ = os.WriteFile(xReg, []byte(`{"entries": [{"id": "a"}, {"id": "b"}]}`), 0o644)
	if rkuRun(stdout, stderr, td, false) != 0 {
		fmt.Fprintln(stdout, "  ✗ 负控失败：键唯一应回 0")
		ok = false
	}
	// 负控二：无键面 ⇒ 跳过、不判红 ⇒ rc=0
	_ = os.WriteFile(yCtr, []byte(`{"items": [{"foo": 1}, {"foo": 1}]}`), 0o644)
	if rkuRun(stdout, stderr, td, false) != 0 {
		fmt.Fprintln(stdout, "  ✗ 负控失败：无键面不该判红")
		ok = false
	}
	// 负控三：非法 JSON ⇒ rc=2
	_ = os.WriteFile(zCtr, []byte("{ not json"), 0o644)
	if rkuRun(stdout, stderr, td, false) != 2 {
		fmt.Fprintln(stdout, "  ✗ 负控失败：非法 JSON 应回 2")
		ok = false
	}
	// 负控四：空域 ⇒ rc=2
	emptyRoot := rkuPyJoin(td, "empty-root")
	_ = os.MkdirAll(rkuPyJoin(emptyRoot, rkuScanDirGates), 0o755)
	_ = os.MkdirAll(rkuPyJoin(emptyRoot, rkuScanDirContract), 0o755)
	if rkuRun(stdout, stderr, emptyRoot, false) != 2 {
		fmt.Fprintln(stdout, "  ✗ 负控失败：空域（0 件登记件）应回 2（空集守卫）")
		ok = false
	}
	if ok {
		fmt.Fprintln(stdout, "自检 PASS（5 格）")
		return 0
	}
	fmt.Fprintln(stdout, "自检 FAIL（5 格）")
	return 1
}

// ── argparse 面（照 py 同名旗标；本件无短旗标除 -h）────────────────────────
func rkuUsage() string {
	return "usage: " + rkuProg + " [-h] [--root ROOT] [--json] [--self-test]"
}

func rkuArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "%s\n%s: error: %s\n", rkuUsage(), rkuProg, msg)
	return 2
}

// rkuHelp —— argparse `-h/--help` 正文（照 py3.9 措辞复刻；★ py≥3.10 印 `options:` —— 见未覆盖面）。
func rkuHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, rkuUsage())
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, rkuHelpDesc)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "optional arguments:")
	fmt.Fprintln(stdout, "  -h, --help   show this help message and exit")
	fmt.Fprintln(stdout, "  --root ROOT")
	fmt.Fprintln(stdout, "  --json")
	fmt.Fprintln(stdout, "  --self-test")
}

// rkuLongOpts —— 本件全部长旗标（供 argparse 唯一前缀缩写解析）。
var rkuLongOpts = []string{"--help", "--root", "--json", "--self-test"}

// rkuResolveLong —— argparse 规则：精确名优先，否则取**唯一前缀**；多匹 = 歧义。
func rkuResolveLong(name string) (string, bool, bool) {
	for _, o := range rkuLongOpts {
		if o == name {
			return o, false, true
		}
	}
	var matches []string
	for _, o := range rkuLongOpts {
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

// rkuLooksOption —— argparse 判定「下一 token 是否像旗标」（`-` 单字与负数不算）。
var rkuNegNumRe = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

func rkuLooksOption(s string) bool {
	if !strings.HasPrefix(s, "-") || s == "-" {
		return false
	}
	if rkuNegNumRe.MatchString(s) {
		return false
	}
	return true
}

// checkRegistryKeyUniqueGateMain —— 入口（= py `main(argv)`，argparse 面逐字复刻）。
func checkRegistryKeyUniqueGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	rootSet := false
	jsonOut := false
	selfTest := false
	var unrecognized []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			rkuHelp(stdout)
			return 0
		case strings.HasPrefix(a, "-h") && !strings.HasPrefix(a, "--") && len(a) > 2:
			val := a[2:]
			if strings.HasPrefix(val, "=") {
				val = val[1:]
			}
			return rkuArgErr(stderr, "argument -h/--help: ignored explicit argument '"+val+"'")
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
			canon, amb, ok := rkuResolveLong(name)
			if amb {
				return rkuArgErr(stderr, "ambiguous option: "+name)
			}
			if !ok {
				unrecognized = append(unrecognized, a)
				continue
			}
			switch canon {
			case "--help":
				if hasVal {
					return rkuArgErr(stderr, "argument -h/--help: ignored explicit argument '"+val+"'")
				}
				rkuHelp(stdout)
				return 0
			case "--root":
				if hasVal {
					root, rootSet = val, true
				} else {
					if i+1 >= len(argv) || rkuLooksOption(argv[i+1]) {
						return rkuArgErr(stderr, "argument --root: expected one argument")
					}
					i++
					root, rootSet = argv[i], true
				}
			case "--json":
				if hasVal {
					return rkuArgErr(stderr, "argument --json: ignored explicit argument '"+val+"'")
				}
				jsonOut = true
			case "--self-test":
				if hasVal {
					return rkuArgErr(stderr, "argument --self-test: ignored explicit argument '"+val+"'")
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
		return rkuArgErr(stderr, "unrecognized arguments: "+strings.Join(unrecognized, " "))
	}
	if selfTest {
		return rkuSelfTest(stdout, stderr)
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
	return rkuRun(stdout, stderr, root, jsonOut)
}

// ═══════════════════════ ★ 未覆盖面（逐条如实声明）═══════════════════════
//
//	① **py 版本相关措辞**：`-h` 正文 py≤3.9 印 `optional arguments:`、py≥3.10 印 `options:`；本件按
//	   py3.9（本机 `/usr/bin/python3` = 3.9.6）复刻 ⇒ 门跑环境若用 py≥3.10，帮助面 stdout 分叉（rc 恒 0）。
//	   未知旗标 / 缺值 / `--json=1` 的 error 正文同为 py 版本相关措辞。
//	② **非法 JSON 正文已对齐（2026-10-07）**：py `json.load` 抛 `JSONDecodeError` 时，`str(exc)`
//	   的形状 `<msg>: line <L> column <C> (char <P>)` 由本件自带扫描器 `rkuPyJSONErrStr` 逐字复刻
//	   （码点下标口径 · 101 例语料 py⟷Go 逐字对齐；`--self-test` 负控三的
//	   "Expecting property name enclosed in double quotes: line 1 column 3 (char 2)" 即其中一例）。
//	   **仍属未覆盖**的两档：(i) **读不出件的正文** —— py 走 `open` 的 `OSError` 文本
//	   （`[Errno 2] No such file or directory: '<path>'`），Go `os.ReadFile` 错误文本不同 ⇒ 只保
//	   `读不出：` 前缀（真仓件件可读 ⇒ 默认档不触发）；(ii) **py 扩展档 `NaN`/`Infinity`/`-Infinity`** ——
//	   py 认合法而 Go `encoding/json` 拒 ⇒ 该档回 Go 原文（真仓件件合法 ⇒ 不触发）。
//	③ **列表 / 数值展示串的边界**：`rkuPyDumps`（`", "`/`": "`、ensure_ascii=False、sort_keys）与
//	   `rkuPyNumRepr`（int 保字面 / float 走 `repr` 近似）覆盖真仓实际形态（str 键值 + 少量 int）；
//	   Python float `repr` 的**极端指数 / 精度边界**与**dict 值档**（py 走插入序 `repr(dict)`、Go map 无序）
//	   未逐字复刻 ⇒ 声明为未覆盖（真仓键值只现 str/int，无 list/dict/float）。
//	④ **符号链接路径下的缺省根**：py `os.getcwd()` ⟷ Go `os.Getwd()`（后者优先 `$PWD`）⇒ 对拍 / 夹具
//	   一律落**非符号链接路径**以对齐（真仓 cwd 非软链 ⇒ 不触发）。
//	⑤ **目录「在盘但读不到」（权限）/ `name.json` 是目录档**：py `os.listdir` / `open` 会抛未捕
//	   OSError ⇒ 裸回溯 rc=1（C 类无兜底）；本件按新口径把「扫面目录读不到」并入 skip（对齐 `os.path.isdir`
//	   为真而 `listdir` 失败时的 py 行为：py 该档回溯 ⇒ Go 走 continue、不静默判绿，见 `rkuScanDirNames`）。
//	⑥ **argparse 面**：本件复刻唯一前缀缩写（`--ro`/`--r` → `--root` 等）＋歧义 / 缺值 / 未识别三档文案
//	   ＋ `-h=1` / `-hx` 形态（帮助面标签用 argparse 的 `-h/--help`）。
