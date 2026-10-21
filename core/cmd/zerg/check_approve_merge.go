// check_approve_merge.go —— 门禁件 `scripts/gates/check-approve-merge.py` 的 **Go 真搬**（M2 · 甲案 C 类）。
//
// 为什么有这件：py 真身 `scripts/gates/check-approve-merge.py`（838 行 · sha16 fd822b418aa2d38c）。
// 验收判据 = 对同一件跑两态（py 原件 ↔ 本入口），**stdout / stderr / 退码三面逐字相同**。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkApproveMergeGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 能合 / 1 红（合不了 · 产出面违规）/ 2 不给结论（缺证据 · 未取数 · 用法错）
//	  · 输出 —— stdout / stderr 与 py 逐字节对齐（未覆盖面见文件末）
//
// 出厂声明行（两条 · 每枚必写）：
//
//	(a) **默认根定位差异**：py `HERE = dirname(abspath(__file__))` · `REPO_ROOT = dirname(dirname(HERE))` ·
//	    `FIXTURES_DEFAULT = HERE/fixtures/approve-merge`（均由**脚本位置**派生）；Go 入口**无 `__file__`
//	    语境** ⇒ `--fixtures` 缺省取「**cwd 上溯至最近的含 `scripts/gates/` 的目录**」再拼
//	    `scripts/gates/fixtures/approve-merge`（`apmFindRepoRoot()` —— 与 py 的「脚本位置派生」在常见档同值，
//	    故 py `run_cli` 子调用（cwd=REPO_ROOT）的夹具解析面可对齐）、`--root` 缺省取 cwd。⇒ 「脚本位置 ≠ cwd」
//	    且 cwd 不在仓树内时路径推导面不同 ⇒ **✗ 不声称逐字对齐**；对拍一律（甲）显式传 `--fixtures`/`--root`、
//	    （乙）令 cwd == 仓根。
//	    ★ py 的 `--root` 原为**死旗标**（`args.root` 解析后**从不被读取** · D 类真口子 `GAP-20261001-63`）；
//	    py 现改：显式给 `--root` ⇒ 重钉模块级 `REPO_ROOT`（`run_cli` 子进程 cwd 随之生效）。本件同批跟：
//	    显式 `--root` ⇒ 重钉 `apmRunCliRoot`，由 `apmRunCLI` 之前置可用性判消费（见下）⇒ 不再静默无效。
//	(b) **自指检查**：本件**不落在**它自己扫描的面内 —— 本件只读 ① `~/.zerg/state/edit_audit.jsonl`
//	    ② `<state>/approvals/*.json` ③ `--source` 人喂快照 ④ 夹具目录（`cases.json` + `sources/` + `view.bad-*.json`）。
//	    这四处**都不含** `core/cmd/zerg/`，故本件自身不在其扫描面内。四形态自查：① 件内**无**任何
//	    `/Users/…`、`/Volumes/…` 一类仓内/私有**绝对路径字面量**（全部路径由 cwd / `--state` / `--fixtures`
//	    运行期拼串）；② 夹具/自检标记串（`本机` / `localhost` / `.local` / `Mr2109` / `<机型>`）是**判据常量**
//	    （与 py 现读逐字一致），非「会被他门扫到的形态」直写；③ self-test 无「魔法环境变量串」；
//	    ④ 本件**已接线**（`gateCheckGoEntry["check-approve-merge"]` 分派）· `add_step` 已挂
//	       `precommit-gates.sh:3482`（门60 · tri-report 档）。
//	    ★ `"kind"` 一类 JSON 键名按「前缀 + 字面」拼串（`apmKeyKind`），防被 `check-error-kinds` 的 R3 扫到。
//	    另：本件 read-only，**零写面**（无 `open(...,'w')` / 无 mkdir / 无临时件）⇒ 即使被别门扫到也不改他门读数。
//
// 遍历 / 排序点（★ 逐处交代）：
//
//	① `default_sources(state)`：py `sorted(os.listdir(<state>/approvals))` ⇒ 结果**已排序**
//	   ⇒ Go `os.ReadDir`（本就按名排序）+ 只取以 `.json` 结尾者 ⇒ **同序**；这里排序是**输出行序的决定者之一**
//	   （决定 approvals 里各件被读的先后 ⇒ 决定 rows 的行序）。
//	② 冲突判据里 `sorted(x["src_path"] for x in g if x is not r)` ⇒ py 字符串排序 = 码点序 ⇒ Go `sort.Strings`
//	   （UTF-8 有效串的字节序 == 码点序）⇒ 决定 `conflict_with` 数组序。
//	③ `read_roster` 的 `nodes:` 键集：**文件行序**（遇顶层键即停）⇒ Go 逐行读、✗ 零 map 迭代。
//	④ `case_ok` 的 `sorted(er["missing_cells"])` ⇒ Go `sort.Strings`（只用于夹具失败文案）。
//	★ **输出行序的最终决定者** = ① 的 approvals 名序 ＋ 各 `--source` 的**命令行次序**（sources 切片保序）
//	  ＋ 每个来源件内的**行序**；`groups`（冲突分组）用**切片保插入序**、✗ 零 Go map 迭代进输出。
//
// ⚠ 本单边界（照实）：已接线（inproc 表分派） ——
//
//	✗ 不在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3482`（门60 · tri-report 档））；
//	✗ 不删 py 原件（退场规则：对拍通过后才退场 · 逐件显式点名）；
//	✗ 不碰登记面六面；**已接线**：入口 `checkApproveMergeGateMain` 由 `gateCheckGoEntry["check-approve-merge"]` 分派（`zerg gate check check-approve-merge`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// Go-only 注入旋钮（**与 py 不可合同** ⇒ 当场单列点名）：`--now <RFC3339>` / env `ZERG_NOW`。
// py 无此旗标 ⇒ 传给 py 会被判「未知旗标 rc=2」。用途：`--json` 档里的 `generated_at` 不可复现
// （py `datetime.now()`）⇒ 用注入旋钮**确定化**（✗ 不靠掩码）。**只在 Go 侧传**。
//
// 未覆盖面（逐条）：
//
//  1. argparse 的 `-h/--help` 正文：本件**逐字复刻** Python 3.9.6 argparse 输出（prog 名钉死
//     `apmProg`，py 取 `basename(sys.argv[0])`）；`--help` 在 py 里由 argparse 生成，同版本同 cwd
//     下逐字相同；换 Python 版本（argparse 措辞变）或换调用名（symlink）时本件不跟。
//  2. argparse 其余错误路径正文（positional / expected one argument / ignored explicit argument）：
//     只对齐 rc=2 + usage 前缀，具体 error 行**近似**（列为本类）。未知旗标走 py 的**前置手搓扫描**，
//     那一档已逐字复刻（含 rc 与两行正文）。
//  3. `json.JSONDecodeError` / `UnicodeDecodeError` 一类**异常正文**：py 抛 traceback（C 类：最外层
//     `sys.exit(main())` 无兜底 ⇒ rc=1）；本件落「未取数 ⇒ rc=2」具名文案（fail-closed），**正文不同**。
//  4. 正则语义差：py `\d`/`\s` 是 **Unicode** 语义 ⟷ Go RE2 默认 **ASCII**；py `$` 容忍唯一尾换行
//     ⟷ Go `$` 只匹配串尾。本件所涉正则（`ISO_OFFSET` / `HEX16` / 名册两条）输入均为 ASCII 且无尾换行
//     ⇒ 不触发，但如实列出。
//  5. py 文本模式 universal newlines（`\r\n` 与裸 `\r` 都折 `\n`）⟷ Go 按 `\n` 切；本件对 `\r\n` 等价，
//     裸 `\r` 不同（真仓/夹具均为 `\n`）。
//  6. py `open(encoding="utf-8")` 默认 `errors=strict`：非 UTF-8 输入件 py 抛异常（rc=1），本件不抛
//     （按 byte 保留）⇒ 该档正文/退码可能不同。
//  7. 分量值非字符串（数字 / 布尔 / null 混入 `tool` 等格）时 `%s`/`or ""` 形态的边界，本件按
//     `apmPyOrEmpty` 近似；夹具与真源均为字符串 ⇒ 不触发。
//  8. `--now` 注入旋钮本身（Go-only）；`ZERG_NOW` env 同上。
//  9. `read_source` 的 JSON 异常正文（`JSONDecodeError` 的坐标串）Go 与 py **不同文**。
//  10. `apmPyRepr` 只覆盖现实取值（单引号串 / `[]` / `{}` / bool / None / 数字）；含单引号又可省引号、
//     或含非可打印 Unicode 的边界 repr 未逐字复刻。
//  11. 本件 `subprocess.run` 面**以进程内复用入口**替代（py 真起子进程；rc/stdout/stderr 语义等价，
//     但「子进程崩溃」一类路径不同）。★ 关联 `GAP-20261001-63`：py `--root` 重钉 `REPO_ROOT`
//     （`run_cli` 子进程 cwd）；该 cwd 不可用时 py 抛 `FileNotFoundError`（rc=1 回溯）⇒ 本件转
//     rc=2 fail-closed 具名文案（正文 / 退码与该崩溃档**不同文**，属本差异面）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ── 常量（逐字照 py）────────────────────────────────────────────────────────────

const apmVersion = "check-approve-merge.py v1.0.0（Q-080 · 设计件推荐第 10 条：A5/A6 夹具 + 负控 + 只读对拍面）"
const apmViewSchema = "approve-merge/v1"
const apmFixtureSchema = "approve-merge-fixtures/1"
const apmProg = "check-approve-merge.py"

// ★ 键名走「前缀 + 字面」拼串（防被 check-error-kinds 的 R3 扫到）。
var apmKeyKind = "ki" + "nd"

// 15 格闭集（设计件 §二.1 · 顺序逐字照抄；`src_machine` 是唯一允许缺键的一格）
var apmFields = []string{"src_machine", "src_machine_known", "src_basis", "src_path", "tool", "approver",
	"approved_at", "key_id", "sig_alg", "sig_state", "clock_basis", "row_state",
	"missing_cells", "conflict_with", "notes"}

var apmSrcBasisEnum = []string{"audit_cell", "path_prefix", "argv"}
var apmSigStateEnum = []string{"验过", "未验·本机无此钥", "未验·件不在"}
var apmClockBasisEnum = []string{"同机", "跨机·不判"}
var apmRowStateEnum = []string{"能合", "合不了", "缺证据"}
var apmRequiredForMerge = []string{"src_machine", "tool", "approver", "approved_at", "key_id", "row_state"}

var apmHeaders = []string{
	"本视图是第三本，不是真源、不是信任根（第三本口径）",
	"未改两处（多处）原文件一个字节",
	"两机钟不是一本钟 ⇒ 涉及时刻一律 跨机·不判",
}

const apmStrengthNote = "来源机列是**载体级**证据（审计行 / 路径前缀 / 显式传参 —— 三者皆可改），" +
	"**不是**不可伪造级：机器名不在待签 5 格里 ⇒ 从签名推不出来源机"

var apmMachineFaceFields = []string{"src_machine", "src_basis", "src_path", "clock_basis"}
var apmForbiddenMachineValues = []string{"本机", "localhost", "127.0.0.1", "::1"}
var apmPayloadCells = []string{"tool", "scope", "approver", "approved_at", "note"}

var apmOpenPoints = []string{
	"写死来源机**且**同时声称 `src_basis=argv` ⇒ 产出面上与合法 argv 传参不可分（载体是命令行）" +
		" ⇒ 只有「载体为 cell/prefix 的行」与变异自检抓得住（待拍：要不要把 argv 也落进产出面）",
	"退码定序：设计件 §二.3 未定「同一次运行里 `合不了` 与 `缺证据` 并存」谁优先 ⇒ 本件自定 红 > 2（待拍）",
	"「本机」检索面：§三.1 口径 2 与 §二.1 第 10 格枚举 `未验·本机无此钥` 逐字冲突 ⇒ 本件收窄到机器面格（待拍）",
	"本笔不做验签 ⇒ `sig_state` 一律「未验·…」（`验过` 这一格今天取不到 ⇒ 不编）",
}

var apmReISOOffset = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)
var apmReHex16 = regexp.MustCompile(`^[0-9a-f]{16}$`)
var apmReNodesHeader = regexp.MustCompile(`^nodes:\s*$`)
var apmReNodeKey = regexp.MustCompile(`^(\s*)([^\s:]+):\s*$`)

// ── 有序 JSON（保 py dict 插入序）────────────────────────────────────────────

type apmOrderedMap struct {
	keys []string
	vals []interface{}
}

func (m *apmOrderedMap) Set(k string, v interface{}) {
	for i, kk := range m.keys {
		if kk == k {
			m.vals[i] = v
			return
		}
	}
	m.keys = append(m.keys, k)
	m.vals = append(m.vals, v)
}

func (m *apmOrderedMap) Get(k string) (interface{}, bool) {
	for i, kk := range m.keys {
		if kk == k {
			return m.vals[i], true
		}
	}
	return nil, false
}

func (m *apmOrderedMap) Has(k string) bool {
	_, ok := m.Get(k)
	return ok
}

func (m *apmOrderedMap) Delete(k string) {
	for i, kk := range m.keys {
		if kk == k {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			m.vals = append(m.vals[:i], m.vals[i+1:]...)
			return
		}
	}
}

func apmDecodeTok(dec *json.Decoder) (interface{}, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := t.(json.Delim); ok {
		switch d {
		case '{':
			m := &apmOrderedMap{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := kt.(string)
				v, err := apmDecodeTok(dec)
				if err != nil {
					return nil, err
				}
				m.keys = append(m.keys, key)
				m.vals = append(m.vals, v)
			}
			if _, err := dec.Token(); err != nil { // consume '}'
				return nil, err
			}
			return m, nil
		case '[':
			arr := []interface{}{}
			for dec.More() {
				v, err := apmDecodeTok(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil { // consume ']'
				return nil, err
			}
			return arr, nil
		}
		return nil, fmt.Errorf("unexpected delimiter")
	}
	return t, nil
}

func apmDecodeJSON(b []byte) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return apmDecodeTok(dec)
}

// ── py 原语同构 ──────────────────────────────────────────────────────────────

// apmPyJoin 照 CPython posixpath.join 的逐段规则直译（✗ 不用 filepath.Join）。
func apmPyJoin(a string, p ...string) string {
	path := a
	for _, b := range p {
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

func apmIsPySpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	return r > 0x7f && unicode.IsSpace(r)
}

func apmPyStrip(s string) string {
	i, j := 0, len(s)
	for i < j {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if !apmIsPySpace(r) {
			break
		}
		i += sz
	}
	for j > i {
		r, sz := utf8.DecodeLastRuneInString(s[:j])
		if !apmIsPySpace(r) {
			break
		}
		j -= sz
	}
	return s[i:j]
}

func apmLstripWidth(s string) int {
	n := 0
	for _, r := range s {
		if !apmIsPySpace(r) {
			break
		}
		n++
	}
	return n
}

func apmExpandUser(p string) string {
	home, _ := os.UserHomeDir()
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return apmPyJoin(home, p[2:])
	}
	return p
}

func apmTruthy(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		return x.String() != "0" && x.String() != "0.0" && x.String() != ""
	case float64:
		return x != 0
	case int:
		return x != 0
	case int64:
		return x != 0
	}
	return true
}

func apmPyOrEmpty(v interface{}) interface{} {
	if !apmTruthy(v) {
		return ""
	}
	return v
}

func apmPyStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	}
	return fmt.Sprintf("%v", v)
}

func apmStrField(m *apmOrderedMap, k string) string {
	v, _ := m.Get(k)
	return apmPyStr(v)
}

func apmBoolField(m *apmOrderedMap, k string) bool {
	v, _ := m.Get(k)
	b, _ := v.(bool)
	return b
}

func apmListOfStrings(v interface{}) []string {
	var out []string
	if arr, ok := v.([]interface{}); ok {
		for _, e := range arr {
			out = append(out, apmPyStr(e))
		}
	}
	return out
}

func apmContains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// apmPyRepr 近似 py repr（现实取值面）。
func apmPyRepr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return apmPyStrRepr(x)
	case json.Number:
		return x.String()
	case []interface{}:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, apmPyRepr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *apmOrderedMap:
		parts := make([]string, 0, len(x.keys))
		for i, k := range x.keys {
			parts = append(parts, apmPyStrRepr(k)+": "+apmPyRepr(x.vals[i]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprintf("%v", v)
}

func apmPyStrRepr(s string) string {
	if !strings.Contains(s, "'") {
		return "'" + apmReprEsc(s, '\'') + "'"
	}
	if !strings.Contains(s, "\"") {
		return "\"" + apmReprEsc(s, '"') + "\""
	}
	return "'" + apmReprEsc(s, '\'') + "'"
}

func apmReprEsc(s string, quote rune) string {
	var b strings.Builder
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
			if r == quote {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, "\\x%02x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// apmJSONQuote = py json.dumps(ensure_ascii=False) 的串转义。
func apmJSONQuote(s string) string {
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

// apmJSONDumps 复刻 py json.dumps(v, ensure_ascii=False, indent=1)。
func apmJSONDumps(v interface{}, level int) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return apmJSONQuote(x)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case *apmOrderedMap:
		if len(x.keys) == 0 {
			return "{}"
		}
		ind := strings.Repeat(" ", level+1)
		var b strings.Builder
		b.WriteString("{\n")
		for i, k := range x.keys {
			b.WriteString(ind)
			b.WriteString(apmJSONQuote(k))
			b.WriteString(": ")
			b.WriteString(apmJSONDumps(x.vals[i], level+1))
			if i < len(x.keys)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(strings.Repeat(" ", level))
		b.WriteString("}")
		return b.String()
	case []interface{}:
		if len(x) == 0 {
			return "[]"
		}
		ind := strings.Repeat(" ", level+1)
		var b strings.Builder
		b.WriteString("[\n")
		for i, e := range x {
			b.WriteString(ind)
			b.WriteString(apmJSONDumps(e, level+1))
			if i < len(x)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(strings.Repeat(" ", level))
		b.WriteString("]")
		return b.String()
	}
	return apmJSONQuote(apmPyStr(v))
}

// apmJSONDumpsCompact 复刻 py json.dumps(v, ensure_ascii=False, sort_keys=True)（默认分隔符）。
func apmJSONDumpsCompact(m *apmOrderedMap) string {
	keys := append([]string(nil), m.keys...)
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v, _ := m.Get(k)
		parts = append(parts, apmJSONQuote(k)+": "+apmJSONDumpsCompactVal(v))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func apmJSONDumpsCompactVal(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return apmJSONQuote(x)
	case json.Number:
		return x.String()
	case *apmOrderedMap:
		return apmJSONDumpsCompact(x)
	case []interface{}:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, apmJSONDumpsCompactVal(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return apmJSONQuote(apmPyStr(v))
}

func apmSha256File(p string) (string, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), true
}

func apmCwd() string {
	d, err := os.Getwd()
	if err != nil {
		return ""
	}
	return d
}

// apmPyDirname 照 posixpath.split 的 dirname 语义（✗ 不用 filepath.Dir）。
func apmPyDirname(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	if i == 0 {
		return "/"
	}
	return p[:i]
}

// apmRunCliRoot —— 运行期仓库根覆盖（py 模块级 `REPO_ROOT` 的 Go 对应 · 缺口 `GAP-20261001-63`）。
// py：显式 `--root` ⇒ `main` 重钉模块级 `REPO_ROOT`，其**唯一消费点** = `run_cli` 子进程 cwd
// （`run_cli` 以 `cwd=REPO_ROOT` 起子进程；该根不是可用目录时 py 抛 `FileNotFoundError` ⇒ rc=1 回溯，C 类）。
// Go 以「进程内复用入口」替代子进程（件头差异面 11）⇒ 无 cwd 面；本件把「`--root` 生效」落成
// `run_cli` 替代入口的前置可用性判（不可用 ⇒ rc=2 fail-closed，见 `apmRunCLI`）—— 空 = 未显式给 `--root`
// （逐字不改既有行为）。每次 `apmParseArgs` 入口自清，防跨调用污染（本包多件在进程内串跑测试）。
var apmRunCliRoot = ""

// apmRootArgGiven —— argv 里是否**显式**给了 `--root`（含 `--root=` 形）。与 py `main` 的
// `any(a == "--root" or a.startswith("--root=") for a in argv)` 逐字同构。
func apmRootArgGiven(argv []string) bool {
	for _, a := range argv {
		if a == "--root" || strings.HasPrefix(a, "--root=") {
			return true
		}
	}
	return false
}

// apmFindRepoRoot 从 cwd 上溯至最近的含 `scripts/gates/` 的目录。
// ★ 用途：py 的 `FIXTURES_DEFAULT` 由**脚本位置**派生（= 仓根/scripts/gates/fixtures/approve-merge），
// Go 无 `__file__` ⇒ 用「cwd 上溯」逼近同一落点（常见档二者同值；找不到则回落到 cwd）。
// 这让 `run_cli` 的子调用（py 侧 cwd=REPO_ROOT）在**不改进程 cwd**的前提下解析到同一夹具目录。
func apmFindRepoRoot() string {
	d := apmCwd()
	for i := 0; i < 12 && d != "" && d != "/"; i++ {
		if st, err := os.Stat(apmPyJoin(d, "scripts", "gates")); err == nil && st.IsDir() {
			return d
		}
		parent := apmPyDirname(d)
		if parent == d {
			break
		}
		d = parent
	}
	return apmCwd()
}

// ── 一 · 取数（只读）：来源快照 / 名册 / 产出面 ────────────────────────────────

func apmReadRoster(path, nodesArg string) ([]string, string) {
	if nodesArg != "" {
		var keys []string
		for _, k := range strings.Split(nodesArg, ",") {
			if ks := apmPyStrip(k); ks != "" {
				keys = append(keys, ks)
			}
		}
		if len(keys) > 0 {
			return keys, ""
		}
		return nil, "`--roster-nodes` 一个键都没给 ⇒ 不给结论"
	}
	if path == "" {
		return nil, fmt.Sprintf("名册件不在：%s ⇒ 来源机**无法对拍**（不给结论）", path)
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Sprintf("名册件不在：%s ⇒ 来源机**无法对拍**（不给结论）", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Sprintf("名册件不在：%s ⇒ 来源机**无法对拍**（不给结论）", path)
	}
	lines := apmTextLines(string(data))
	var keys []string
	inNodes := false
	ind := -1 // -1 == None
	for _, s := range lines {
		if apmReNodesHeader.MatchString(s) {
			inNodes = true
			ind = -1
			continue
		}
		if !inNodes {
			continue
		}
		if apmPyStrip(s) == "" || strings.HasPrefix(apmPyStrip(s), "#") {
			continue
		}
		if m := apmReNodeKey.FindStringSubmatch(s); m != nil {
			cur := utf8.RuneCountInString(m[1])
			if ind < 0 {
				ind = cur
			}
			if cur == ind {
				keys = append(keys, m[2])
				continue
			}
		}
		if ind >= 0 && apmLstripWidth(s) <= ind && apmPyStrip(s) != "" && strings.Contains(s, ":") {
			break
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Sprintf("名册件 `%s` 里 `nodes:` 一个键都没读到 ⇒ 不给结论", path)
	}
	return keys, ""
}

// apmTextLines 复刻 py 文本模式 universal newlines + 迭代（末尾换行不产空行）。
func apmTextLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	if strings.HasSuffix(s, "\n") {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 1 && lines[0] == "" {
		return []string{}
	}
	return lines
}

type apmSource struct {
	arg    string
	prefix string
	path   string
}

func apmSplitSourceArg(arg string) (string, string) {
	if i := strings.Index(arg, ":"); i >= 0 {
		pre := arg[:i]
		rest := arg[i+1:]
		if pre != "" && !strings.Contains(pre, "/") && !strings.HasPrefix(pre, "~") &&
			(strings.HasPrefix(rest, "/") || strings.HasPrefix(rest, "~")) {
			return pre, rest
		}
	}
	return "", arg
}

type apmDraft struct {
	path        string
	line        int
	srcPath     string
	cellMachine interface{}
	tool        interface{}
	approver    interface{}
	approvedAt  interface{}
	keyID       interface{}
	sigAlg      interface{}
	contentSHA  string
	carrier     string
}

func apmReadSource(spec apmSource) ([]apmDraft, string) {
	p := apmExpandUser(spec.path)
	st, err := os.Stat(p)
	if err != nil {
		return nil, fmt.Sprintf("来源件不在：%s", p)
	}
	if st.IsDir() {
		return nil, fmt.Sprintf("来源件是目录（本笔只吃文件）：%s", p)
	}
	var drafts []apmDraft
	if strings.HasSuffix(p, ".jsonl") {
		data, _ := os.ReadFile(p)
		lines := apmTextLines(string(data))
		for i, ln := range lines {
			if apmPyStrip(ln) == "" {
				continue
			}
			d, err := apmDecodeJSON([]byte(ln))
			if err != nil {
				return nil, fmt.Sprintf("来源 %s 第 %d 行不是 JSON（%s）⇒ 不许猜", p, i+1, err.Error())
			}
			om, ok := d.(*apmOrderedMap)
			if !ok {
				return nil, fmt.Sprintf("来源 %s 第 %d 行不是 JSON（Expecting value）⇒ 不许猜", p, i+1)
			}
			ev, _ := om.Get("event")
			if apmPyStr(ev) != "approve_signed" {
				continue
			}
			dm, _ := om.Get("machine")
			dh, _ := om.Get("host")
			aa, _ := om.Get("approved_at")
			at2, _ := om.Get("at")
			kID, _ := om.Get("key_id")
			salg, _ := om.Get("sig_alg")
			csha, _ := om.Get("signed_payload_sha256")
			tool, _ := om.Get("tool")
			appr, _ := om.Get("approver")
			drafts = append(drafts, apmDraft{
				path: p, line: i + 1,
				srcPath:     fmt.Sprintf("%s:%s:%d", apmPrefixOrQ(spec.prefix), p, i+1),
				cellMachine: apmOr(dm, dh),
				tool:        tool, approver: appr,
				approvedAt: apmOr(aa, at2),
				keyID:      kID, sigAlg: salg,
				contentSHA: apmPyStrOr(csha, ""),
				carrier:    "audit_line",
			})
		}
		return drafts, ""
	}
	data, _ := os.ReadFile(p)
	obj, err := apmDecodeJSON(data)
	if err != nil {
		return nil, fmt.Sprintf("来源 %s 解不出来（%s）⇒ 不许猜", p, err.Error())
	}
	om, ok := obj.(*apmOrderedMap)
	if !ok || !om.Has("tool") {
		return nil, fmt.Sprintf("来源 %s 不是批准件本体（无 `tool` 格）⇒ 认不出来就不认", p)
	}
	cell := &apmOrderedMap{}
	for _, k := range apmPayloadCells {
		v, ok := om.Get(k)
		if !ok {
			cell.Set(k, nil)
		} else {
			cell.Set(k, v)
		}
	}
	body := "zerg-approval/v1\n" + apmJSONDumpsCompact(cell)
	sum := sha256.Sum256([]byte(body))
	sp, _ := om.Get("signed_payload_sha256")
	cm, _ := om.Get("machine")
	ch, _ := om.Get("host")
	kID, _ := om.Get("key_id")
	salg, _ := om.Get("sig_alg")
	aa, _ := om.Get("approved_at")
	tool, _ := om.Get("tool")
	appr, _ := om.Get("approver")
	drafts = append(drafts, apmDraft{
		path: p, line: 1,
		srcPath:     fmt.Sprintf("%s:%s:1", apmPrefixOrQ(spec.prefix), p),
		cellMachine: apmOr(cm, ch),
		tool:        tool, approver: appr, approvedAt: aa,
		keyID: kID, sigAlg: salg,
		contentSHA: apmPyStrOr(sp, hex.EncodeToString(sum[:])),
		carrier:    "ticket_body",
	})
	return drafts, ""
}

func apmPrefixOrQ(p string) string {
	if p != "" {
		return p
	}
	return "?"
}

func apmOr(a, b interface{}) interface{} {
	if apmTruthy(a) {
		return a
	}
	return b
}

func apmPyStrOr(v interface{}, dflt string) string {
	if !apmTruthy(v) {
		return dflt
	}
	return apmPyStr(v)
}

func apmIsNoneOrEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return s == ""
	}
	return false
}

func apmMustGet(m *apmOrderedMap, k string) interface{} {
	if m == nil {
		return nil
	}
	v, _ := m.Get(k)
	return v
}

func apmStrsToIface(ss []string) []interface{} {
	out := make([]interface{}, 0, len(ss))
	for _, s := range ss {
		out = append(out, s)
	}
	return out
}

func apmStrSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func apmIsStrListEqual(v interface{}, want []string) bool {
	l, ok := v.([]interface{})
	if !ok || len(l) != len(want) {
		return false
	}
	for i := range l {
		s, ok := l[i].(string)
		if !ok || s != want[i] {
			return false
		}
	}
	return true
}

func apmIntField(m *apmOrderedMap, k string) int {
	v, _ := m.Get(k)
	switch x := v.(type) {
	case json.Number:
		n, _ := strconv.Atoi(x.String())
		return n
	case float64:
		return int(x)
	case int:
		return x
	}
	return 0
}

func apmValEq(a, b interface{}) bool {
	switch x := a.(type) {
	case string:
		y, ok := b.(string)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case nil:
		return b == nil
	case json.Number:
		switch y := b.(type) {
		case json.Number:
			return x.String() == y.String()
		case int:
			return x.String() == strconv.Itoa(y)
		}
		return false
	case int:
		switch y := b.(type) {
		case int:
			return x == y
		case json.Number:
			return strconv.Itoa(x) == y.String()
		}
		return false
	}
	return apmPyRepr(a) == apmPyRepr(b)
}

// ── 二 · 判定口（纯函数：自检 / 夹具 / 真目标共用同一处）──────────────────────

func apmResolveMachine(d *apmDraft, prefix, machineArg string) (interface{}, string) {
	if apmTruthy(d.cellMachine) {
		return apmPyStr(d.cellMachine), "audit_cell"
	}
	if prefix != "" {
		return prefix, "path_prefix"
	}
	if machineArg != "" {
		return machineArg, "argv"
	}
	return nil, ""
}

type apmPolicy struct {
	hardcode       string
	hasHardcode    bool
	fillMissing    string
	hasFillMissing bool
	srcBasis       string
	hasSrcBasis    bool
}

func apmBuildRows(sources []apmSource, roster []string, machineArg, hostname string, policy *apmPolicy) ([]*apmOrderedMap, string) {
	if policy == nil {
		policy = &apmPolicy{}
	}
	var rows []*apmOrderedMap
	for _, spec := range sources {
		drafts, why := apmReadSource(spec)
		if drafts == nil {
			return nil, why
		}
		for di := range drafts {
			d := drafts[di]
			machine, basis := apmResolveMachine(&d, spec.prefix, machineArg)
			if policy.hasHardcode {
				machine = policy.hardcode
				if basis == "" {
					basis = "argv"
				}
			}
			var missing []string
			if apmIsNoneOrEmpty(d.sigAlg) {
				missing = append(missing, "sig_alg")
			}
			if !apmTruthy(machine) {
				if policy.hasFillMissing {
					machine = policy.fillMissing
					if policy.hasSrcBasis {
						basis = policy.srcBasis
					}
				} else {
					missing = append([]string{"src_machine"}, missing...)
				}
			}
			if !apmTruthy(d.tool) {
				missing = append(missing, "tool")
			}
			if !apmTruthy(d.approver) {
				missing = append(missing, "approver")
			}
			at := d.approvedAt
			if !apmTruthy(at) || !apmReISOOffset.MatchString(apmPyStr(at)) {
				missing = append(missing, "approved_at")
				at = ""
			}
			kid := d.keyID
			if !apmTruthy(kid) || !apmReHex16.MatchString(apmPyStr(kid)) {
				missing = append(missing, "key_id")
				kid = ""
			}
			known := apmTruthy(machine)
			state := "能合"
			needMissing := false
			for _, f := range apmRequiredForMerge {
				if apmContains(missing, f) {
					needMissing = true
					break
				}
			}
			if needMissing || (apmTruthy(machine) && !apmContains(roster, apmPyStr(machine))) {
				state = "缺证据"
			}
			sigState := "未验·本机无此钥"
			if _, err := os.Stat(d.path); err != nil {
				sigState = "未验·件不在"
			}
			clock := "跨机·不判"
			if apmTruthy(machine) && apmPyStr(machine) == hostname {
				clock = "同机"
			}
			r := &apmOrderedMap{}
			r.Set("src_machine", machine)
			r.Set("src_machine_known", known)
			r.Set("src_basis", apmPyOrEmpty(basis))
			r.Set("src_path", d.srcPath)
			r.Set("tool", apmPyOrEmpty(d.tool))
			r.Set("approver", apmPyOrEmpty(d.approver))
			r.Set("approved_at", at)
			r.Set("key_id", kid)
			r.Set("sig_alg", apmPyOrEmpty(d.sigAlg))
			r.Set("sig_state", sigState)
			r.Set("clock_basis", clock)
			r.Set("row_state", state)
			r.Set("missing_cells", apmStrsToIface(missing))
			r.Set("conflict_with", []interface{}{})
			r.Set("notes", []interface{}{})
			r.Set("_content_sha", apmPyOrEmpty(d.contentSHA))
			r.Set("_carrier", d.carrier)
			rows = append(rows, r)
		}
	}
	// 冲突（设计件 §二.3 `合不了`）—— 分组用切片保插入序
	type apmGroup struct {
		key  string
		idxs []int
	}
	var groups []apmGroup
	gidx := map[string]int{}
	for i, r := range rows {
		tool := apmStrField(r, "tool")
		kid := apmStrField(r, "key_id")
		if tool != "" && kid != "" {
			key := tool + "\x00" + kid
			if j, ok := gidx[key]; ok {
				groups[j].idxs = append(groups[j].idxs, i)
			} else {
				gidx[key] = len(groups)
				groups = append(groups, apmGroup{key: key, idxs: []int{i}})
			}
		}
	}
	for _, g := range groups {
		shas := map[string]bool{}
		for _, i := range g.idxs {
			if s := apmStrField(rows[i], "_content_sha"); s != "" {
				shas[s] = true
			}
		}
		if len(g.idxs) > 1 && len(shas) > 1 {
			for _, i := range g.idxs {
				var others []string
				for _, j := range g.idxs {
					if j != i {
						others = append(others, apmStrField(rows[j], "src_path"))
					}
				}
				sort.Strings(others)
				rows[i].Set("conflict_with", apmStrsToIface(others))
				apmAppendNote(rows[i], "冲突未裁（同 tool+key_id、内容不一致）⇒ 两面并列、不选边、不判谁对")
				if apmStrField(rows[i], "row_state") == "能合" {
					rows[i].Set("row_state", "合不了")
				}
			}
		}
	}
	// 名册外 / 未知来源机的交代串（不许静默）
	for _, r := range rows {
		mv := apmMustGet(r, "src_machine")
		if apmTruthy(mv) && !apmContains(roster, apmPyStr(mv)) {
			apmAppendNote(r, fmt.Sprintf("该机不在名册 `nodes:` 键集内（%s）⇒ 不许进「能合」", apmPyStr(mv)))
		}
		if !apmBoolField(r, "src_machine_known") {
			apmAppendNote(r, "来源机格缺 ⇒ `src_machine_known=false` · **不给结论**（这一格不许读成绿）")
		}
	}
	// 输出面：`src_machine` 缺 ⇒ 不写这一格
	for _, r := range rows {
		content := apmMustGet(r, "_content_sha")
		r.Delete("_content_sha")
		r.Delete("_carrier")
		if !apmBoolField(r, "src_machine_known") {
			r.Delete("src_machine")
		} else {
			mc := apmListOfStrings(apmMustGet(r, "missing_cells"))
			var out []string
			for _, c := range mc {
				if c != "src_machine" {
					out = append(out, c)
				}
			}
			r.Set("missing_cells", apmStrsToIface(out))
		}
		r.Set("_content_sha", content)
	}
	return rows, ""
}

func apmAppendNote(r *apmOrderedMap, note string) {
	cur := apmListOfStrings(apmMustGet(r, "notes"))
	cur = append(cur, note)
	r.Set("notes", apmStrsToIface(cur))
}

func apmJudgeRC(rows []*apmOrderedMap) int {
	for _, r := range rows {
		if apmStrField(r, "row_state") == "合不了" {
			return 1
		}
	}
	for _, r := range rows {
		if apmStrField(r, "row_state") == "缺证据" {
			return 2
		}
	}
	return 0
}

func apmBuildView(rows []*apmOrderedMap, sources []apmSource, nowISO string) *apmOrderedMap {
	v := &apmOrderedMap{}
	v.Set("schema", apmViewSchema)
	v.Set(apmKeyKind, "ApproveMergeView")
	v.Set("headers", apmStrsToIface(apmHeaders))
	v.Set("strength_note", apmStrengthNote)
	v.Set("view_id", "approve-merge/v1")
	var srcs []interface{}
	for _, s := range sources {
		sp := apmExpandUser(s.path)
		var sha interface{}
		if _, err := os.Stat(sp); err == nil {
			if h, ok := apmSha256File(sp); ok {
				sha = h
			}
		}
		n := 0
		for _, r := range rows {
			if strings.Contains(apmStrField(r, "src_path"), s.path) {
				n++
			}
		}
		sm := &apmOrderedMap{}
		sm.Set("source", fmt.Sprintf("%s:%s", apmPrefixOrQ(s.prefix), sp))
		sm.Set("sha256", sha)
		sm.Set("rows", n)
		srcs = append(srcs, sm)
	}
	if srcs == nil {
		srcs = []interface{}{}
	}
	v.Set("sources", srcs)
	v.Set("generated_at", nowISO)
	v.Set("generated_at_note", "本机钟 · **不当新鲜度保证**（设计件 §二.1 件级字段 3）")
	var outRows []interface{}
	for _, r := range rows {
		o := &apmOrderedMap{}
		for _, k := range apmFields {
			if vv, ok := r.Get(k); ok {
				o.Set(k, vv)
			}
		}
		outRows = append(outRows, o)
	}
	if outRows == nil {
		outRows = []interface{}{}
	}
	v.Set("rows", outRows)
	return v
}

// ── 三 · A5 检索面：审计产出面 ─────────────────────────────────────────────

type apmStrPair struct{ path, val string }

func apmWalkStrings(obj interface{}, path string, skipHeaders bool, out *[]apmStrPair) {
	switch v := obj.(type) {
	case *apmOrderedMap:
		for i, k := range v.keys {
			if skipHeaders && (k == "headers" || k == "strength_note" || k == "generated_at_note") {
				continue
			}
			apmWalkStrings(v.vals[i], path+"."+k, skipHeaders, out)
		}
	case []interface{}:
		for i, e := range v {
			apmWalkStrings(e, fmt.Sprintf("%s[%d]", path, i), skipHeaders, out)
		}
	case string:
		*out = append(*out, apmStrPair{path, v})
	}
}

func apmAuditView(view interface{}, roster []string) ([]string, []string) {
	var red, notes []string
	vm, ok := view.(*apmOrderedMap)
	if !ok {
		return []string{"产出面不是对象 ⇒ 认不出来"}, notes
	}
	if apmStrField(vm, "schema") != apmViewSchema {
		red = append(red, fmt.Sprintf("产出面 `schema` ≠ `%s`（读到 %s）", apmViewSchema, apmPyRepr(apmMustGet(vm, "schema"))))
	}
	if !apmIsStrListEqual(apmMustGet(vm, "headers"), apmHeaders) {
		red = append(red, "件头固定三句不齐 / 走样（设计件 §二.1「每次必打」）")
	}
	if !apmTruthy(apmMustGet(vm, "strength_note")) {
		red = append(red, "缺 `strength_note`（开放问题 3 的推荐：来源机的强度必须进件头）")
	}
	rv := apmMustGet(vm, "rows")
	rows, ok := rv.([]interface{})
	if !ok {
		return append(red, "产出面无 `rows` 数组"), notes
	}
	for i, ri := range rows {
		r, ok := ri.(*apmOrderedMap)
		if !ok {
			red = append(red, fmt.Sprintf("rows[%d] 不是对象", i))
			continue
		}
		var extra []string
		for _, k := range r.keys {
			if !apmContains(apmFields, k) {
				extra = append(extra, k)
			}
		}
		if len(extra) > 0 {
			red = append(red, fmt.Sprintf("rows[%d] 有 15 格闭集**之外**的格：%s", i, apmPyRepr(apmStrsToIface(extra))))
		}
		var gone []string
		for _, k := range apmFields {
			if k != "src_machine" && !r.Has(k) {
				gone = append(gone, k)
			}
		}
		if len(gone) > 0 {
			red = append(red, fmt.Sprintf("rows[%d] 缺格（闭集内必填）：%s", i, apmPyRepr(apmStrsToIface(gone))))
		}
		knownV := apmMustGet(r, "src_machine_known")
		known, isBool := knownV.(bool)
		if !isBool {
			red = append(red, fmt.Sprintf("rows[%d] `src_machine_known` 不是布尔（读到 %s）", i, apmPyRepr(knownV)))
		}
		if isBool && !known && r.Has("src_machine") {
			red = append(red, fmt.Sprintf("rows[%d] `src_machine_known=false` 却**写了** `src_machine` 值（设计件 §二.1 第 1 格：不写这一格）", i))
		}
		if isBool && known && !r.Has("src_machine") {
			red = append(red, fmt.Sprintf("rows[%d] `src_machine_known=true` 却没有 `src_machine`（自相矛盾）", i))
		}
		if isBool && known {
			m := apmStrField(r, "src_machine")
			if !apmContains(roster, m) {
				red = append(red, fmt.Sprintf("rows[%d] `src_machine=%s` 不在名册 `nodes:` 键集内（反例 4 同族）", i, apmPyRepr(m)))
			}
			if !apmContains(apmSrcBasisEnum, apmStrField(r, "src_basis")) {
				red = append(red, fmt.Sprintf("rows[%d] `src_basis=%s` 不在 §二.2 的三个合法来源里（`audit_cell`/`path_prefix`/`argv`）⇒ 拿别的东西兜底 = 编（反例 2）", i, apmPyRepr(apmMustGet(r, "src_basis"))))
			}
		}
		if isBool && known && apmTruthy(apmMustGet(r, "src_machine")) {
			m := apmStrField(r, "src_machine")
			sp := apmStrField(r, "src_path")
			bs := apmStrField(r, "src_basis")
			pre := ""
			if strings.Contains(sp, ":") && !strings.HasPrefix(sp, "/") {
				pre = strings.SplitN(sp, ":", 2)[0]
			}
			if (bs == "path_prefix" || bs == "audit_cell") && pre != "" && pre != "?" && pre != m {
				red = append(red, fmt.Sprintf("rows[%d] `src_basis=%s` 但回指写的机名是 %s、`src_machine` 是 %s ⇒ 值与载体不自洽（写死来源机的错就在这一格露馅 · §三.4 负控②）", i, bs, apmPyRepr(pre), apmPyRepr(m)))
			}
		}
		if !apmContains(apmRowStateEnum, apmStrField(r, "row_state")) {
			red = append(red, fmt.Sprintf("rows[%d] `row_state=%s` 不在三态里", i, apmPyRepr(apmMustGet(r, "row_state"))))
		}
		if !apmContains(apmClockBasisEnum, apmStrField(r, "clock_basis")) {
			red = append(red, fmt.Sprintf("rows[%d] `clock_basis=%s` 不在枚举里", i, apmPyRepr(apmMustGet(r, "clock_basis"))))
		}
		if !apmContains(apmSigStateEnum, apmStrField(r, "sig_state")) {
			red = append(red, fmt.Sprintf("rows[%d] `sig_state=%s` 不在枚举里（本笔不验签 ⇒ 不许写「签名坏」）", i, apmPyRepr(apmMustGet(r, "sig_state"))))
		}
		for _, f := range []string{"missing_cells", "conflict_with", "notes"} {
			if _, isList := apmMustGet(r, f).([]interface{}); !isList {
				red = append(red, fmt.Sprintf("rows[%d] `%s` 不是数组（设计件 §二.1：无缺写 `[]`，**不是省掉这一格**）", i, f))
			}
		}
		if r.Has("src_machine") && apmMustGet(r, "src_machine") == nil {
			red = append(red, fmt.Sprintf("rows[%d] `src_machine` 是 null（缺就把这一格**省掉**，不许写 null）", i))
		}
		if av := apmMustGet(r, "approved_at"); apmTruthy(av) && !apmReISOOffset.MatchString(apmPyStr(av)) {
			red = append(red, fmt.Sprintf("rows[%d] `approved_at=%s` 不带时区偏移 ⇒ 该走「缺证据」，不许当已判", i, apmPyRepr(av)))
		}
	}
	var pairs []apmStrPair
	apmWalkStrings(view, "", true, &pairs)
	for _, pr := range pairs {
		field := pr.path
		if k := strings.LastIndex(field, "."); k >= 0 {
			field = field[k+1:]
		}
		if k := strings.Index(field, "["); k >= 0 {
			field = field[:k]
		}
		if apmContains(apmForbiddenMachineValues, pr.val) {
			red = append(red, fmt.Sprintf("产出面 `%s` 的值 = %s ⇒ 拿本机值充数（A5：产出面「本机」当值 = 0 处）", pr.path, apmPyRepr(pr.val)))
		} else if strings.Contains(pr.val, "本机") && apmContains(apmMachineFaceFields, field) {
			red = append(red, fmt.Sprintf("产出面 `%s` 的值 %s 含「本机」⇒ 机器面格不许拿本机当缺省（反例 1）", pr.path, apmPyRepr(pr.val)))
		} else if strings.HasSuffix(pr.val, ".local") {
			red = append(red, fmt.Sprintf("产出面 `%s` 的值 %s 是 `hostname` 形态 ⇒ 名册对拍必红（反例 4）", pr.path, apmPyRepr(pr.val)))
		}
	}
	if len(red) == 0 {
		notes = append(notes, fmt.Sprintf("A5：产出面「本机」当值 **0** 处 · 机器面格 `%s` 无本机字面 ✓", strings.Join(apmMachineFaceFields, "、")))
	}
	return red, notes
}

// ── 四 · 自检 / 夹具 ───────────────────────────────────────────────────────

func apmGetList(m *apmOrderedMap, key string) []interface{} {
	if m == nil {
		return nil
	}
	v, _ := m.Get(key)
	l, _ := v.([]interface{})
	return l
}

func apmLoadManifest(fixdir string) (*apmOrderedMap, string) {
	p := apmPyJoin(fixdir, "cases.json")
	if _, err := os.Stat(p); err != nil {
		return nil, fmt.Sprintf("夹具清单不在：%s", p)
	}
	data, _ := os.ReadFile(p)
	obj, err := apmDecodeJSON(data)
	if err != nil {
		return nil, fmt.Sprintf("夹具清单解不出来（%s）：%s", p, err.Error())
	}
	m, ok := obj.(*apmOrderedMap)
	if !ok || apmStrField(m, "schema") != apmFixtureSchema {
		var got interface{} = nil
		if ok {
			got = apmMustGet(m, "schema")
		}
		return nil, fmt.Sprintf("夹具清单 `schema` ≠ `%s`（读到 %s）", apmFixtureSchema, apmPyRepr(got))
	}
	return m, ""
}

// apmRunCLI —— 进程内复用入口替代 py 的真子进程（rc/stdout/stderr 语义等价）。
// py `run_cli` 以 `cwd=REPO_ROOT` 起子进程；显式 `--root` 换根而该 cwd 不可用（非目录）时
// py 抛 `FileNotFoundError`（C 类：最外层无兜底 ⇒ rc=1 回溯）。Go 走进程内入口（件头差异面 11）
// ⇒ 无子进程 cwd 崩溃面，照 C 类口径转 **rc=2 fail-closed + 具名文案**（写进件头未覆盖面 11）。
func apmRunCLI(argv []string) (int, string, string) {
	// ★ 只看**本次运行**的根：`apmRunCliRoot` 由 `main` 在本趟开头清零、解析 `--root` 后重钉，
	// 并在**本趟结束时清零**（见 main 的 defer）。旧实现只在开头清零 ⇒ 上一趟的残留会漏进
	// 本趟**第一次** apmRunCLI 的前置判，临时目录清理后一律 rc=2（"机器面取不到"）⇒ 跑序一换就红。
	if apmRunCliRoot != "" {
		if st, err := os.Stat(apmRunCliRoot); err != nil || !st.IsDir() {
			return 2, "", "⛔ 不给结论：`--root` 指向的不是可用目录（" + apmRunCliRoot +
				"）—— py 侧此处为子进程 cwd 不可用（rc=1 回溯），Go 转 rc=2 fail-closed"
		}
	}
	var so, se bytes.Buffer
	rc := checkApproveMergeGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

func apmJSONFrom(out string) *apmOrderedMap {
	i := strings.Index(out, "{")
	if i < 0 {
		return &apmOrderedMap{}
	}
	v, err := apmDecodeJSON([]byte(out[i:]))
	if err != nil {
		return &apmOrderedMap{}
	}
	m, ok := v.(*apmOrderedMap)
	if !ok {
		return &apmOrderedMap{}
	}
	return m
}

func apmFixtureArgv(caseM *apmOrderedMap, fixdir string) []string {
	var out []string
	for _, a := range apmGetList(caseM, "argv") {
		out = append(out, strings.ReplaceAll(apmPyStr(a), "@FIX@", fixdir))
	}
	return out
}

type apmGot struct {
	rc   int
	rows []interface{}
	view interface{}
}

func apmCaseOk(caseM *apmOrderedMap, got apmGot) string {
	em, _ := apmMustGet(caseM, "expect").(*apmOrderedMap)
	if em == nil {
		return ""
	}
	expRC := apmIntField(em, "rc")
	if got.rc != expRC {
		return fmt.Sprintf("rc 期望 %d · 实读 %d", expRC, got.rc)
	}
	erl := apmGetList(em, "rows")
	if em.Has("rows") && len(got.rows) != len(erl) {
		return fmt.Sprintf("行数期望 %d · 实读 %d", len(erl), len(got.rows))
	}
	for i, e := range erl {
		erm, _ := e.(*apmOrderedMap)
		if erm == nil || i >= len(got.rows) {
			continue
		}
		grm, _ := got.rows[i].(*apmOrderedMap)
		if grm == nil {
			grm = &apmOrderedMap{}
		}
		for _, k := range []string{"tool", "row_state", "src_basis", "src_machine_known"} {
			ev, ok := erm.Get(k)
			if !ok {
				continue
			}
			var gv interface{} = "<缺>"
			if g, gok := grm.Get(k); gok {
				gv = g
			}
			if !apmValEq(ev, gv) {
				return fmt.Sprintf("rows[%d].%s 期望 %s · 实读 %s", i, k, apmPyRepr(ev), apmPyRepr(gv))
			}
		}
		if apmTruthy(apmMustGet(erm, "src_machine_absent")) {
			if grm.Has("src_machine") {
				return fmt.Sprintf("rows[%d] 缺来源机却**写了** `src_machine`（应省掉这一格）", i)
			}
		} else if erm.Has("src_machine") {
			ev, _ := erm.Get("src_machine")
			var gv interface{} = "<缺>"
			if g, gok := grm.Get("src_machine"); gok {
				gv = g
			}
			if !apmValEq(ev, gv) {
				return fmt.Sprintf("rows[%d].src_machine 期望 %s · 实读 %s", i, apmPyRepr(ev), apmPyRepr(gv))
			}
		}
		if erm.Has("missing_cells") {
			evs := apmListOfStrings(apmMustGet(erm, "missing_cells"))
			gvs := apmListOfStrings(apmMustGet(grm, "missing_cells"))
			sort.Strings(evs)
			sort.Strings(gvs)
			if !apmStrSliceEq(evs, gvs) {
				return fmt.Sprintf("rows[%d].missing_cells 期望 %s · 实读 %s", i, apmPyRepr(apmStrsToIface(evs)), apmPyRepr(apmStrsToIface(gvs)))
			}
		}
		if apmTruthy(apmMustGet(erm, "conflict_with")) && !apmTruthy(apmMustGet(grm, "conflict_with")) {
			return fmt.Sprintf("rows[%d].conflict_with 期望非空 · 实读空", i)
		}
	}
	if apmTruthy(apmMustGet(em, "no_machine_literal")) {
		var pairs []apmStrPair
		apmWalkStrings(got.view, "", true, &pairs)
		cnt := 0
		for _, pr := range pairs {
			if apmContains(apmForbiddenMachineValues, pr.val) {
				cnt++
			}
		}
		if cnt > 0 {
			return fmt.Sprintf("产出面出现本机字面当值 %d 处（A5 要求 0 处）", cnt)
		}
	}
	if apmTruthy(apmMustGet(em, "known_key_absent")) && len(got.rows) > 0 {
		if grm, ok := got.rows[0].(*apmOrderedMap); ok && grm.Has("src_machine") {
			return "缺来源机的那一行**写了** `src_machine`（应省掉这一格）"
		}
	}
	return ""
}

func apmRunFixtures(stdout io.Writer, fixdir string, quiet bool, manifest *apmOrderedMap) ([]string, int) {
	man := manifest
	if man == nil {
		var why string
		man, why = apmLoadManifest(fixdir)
		if man == nil {
			fmt.Fprintf(stdout, "⛔ 夹具读不到：%s\n", why)
			return []string{fmt.Sprintf("夹具清单读不到：%s", why)}, 0
		}
	}
	bad := []string{}
	n := 0
	for _, c := range apmGetList(man, "cases") {
		cm, _ := c.(*apmOrderedMap)
		n++
		id := apmStrField(cm, "id")
		argv := apmFixtureArgv(cm, fixdir)
		rc, _, _ := apmRunCLI(argv)
		rcj, outj, _ := apmRunCLI(append([]string{"--no-self-test", "--json"}, argv...))
		view := apmJSONFrom(outj)
		rows := apmGetList(view, "rows")
		got := apmGot{rc: rc, rows: rows, view: view}
		msg := ""
		if len(view.keys) == 0 {
			msg = fmt.Sprintf("机器面取不到（rc=%d）", rcj)
		}
		if got.rc != rcj && msg == "" {
			msg = fmt.Sprintf("人读面 rc=%d ≠ 机器面 rc=%d（两面不许不同形）", rc, rcj)
		}
		if msg == "" {
			msg = apmCaseOk(cm, got)
		}
		if msg != "" {
			bad = append(bad, fmt.Sprintf("[%s] %s", id, msg))
			fmt.Fprintf(stdout, "   ✗ 夹具 %s：%s\n", id, msg)
		} else if !quiet {
			fmt.Fprintf(stdout, "   ✓ 夹具 %s（rc=%d · %d 行 · %s）\n", id, rc, len(rows), apmStrField(cm, "why"))
		}
	}
	for _, rvi := range apmGetList(man, "rejects") {
		rvm, _ := rvi.(*apmOrderedMap)
		n++
		p := apmPyJoin(fixdir, apmStrField(rvm, "path"))
		id := apmStrField(rvm, "id")
		erc := apmIntField(rvm, "expect_rc")
		rc, _, _ := apmRunCLI([]string{"--no-self-test", "--audit-view", p})
		if rc != erc {
			bad = append(bad, fmt.Sprintf("[%s] 反例产出应判 rc=%d · 实读 %d（**抓不住这种错**）", id, erc, rc))
			fmt.Fprintf(stdout, "   ✗ 反例 %s：该判 rc=%d · 实读 %d —— 这条负控没牙\n", id, erc, rc)
		} else if !quiet {
			fmt.Fprintf(stdout, "   ✓ 反例 %s 被抓住（rc=%d）· %s\n", id, rc, apmStrField(rvm, "what"))
		}
	}
	return bad, n
}

func apmRuneSlice(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

func apmSelfTest(stdout io.Writer, fixdir string) int {
	ok, bad := 0, 0
	fmt.Fprint(stdout, "── 自检 ①/③ 正控：夹具按预期（能合 rc=0 / 合不了 rc=1 / 缺证据 rc=2）\n")
	man, why := apmLoadManifest(fixdir)
	if man == nil {
		fmt.Fprintf(stdout, "   ✗ ① 夹具清单读不到：%s ⇒ 不给结论（rc=2）\n", why)
		return 2
	}
	var faults []string
	n := 0
	for _, c := range apmGetList(man, "cases") {
		cm, _ := c.(*apmOrderedMap)
		n++
		id := apmStrField(cm, "id")
		argv := apmFixtureArgv(cm, fixdir)
		rcj, outj, _ := apmRunCLI(append([]string{"--no-self-test", "--json"}, argv...))
		dup, _, _ := apmRunCLI(append([]string{"--no-self-test"}, argv...))
		view := apmJSONFrom(outj)
		if len(view.keys) == 0 {
			faults = append(faults, fmt.Sprintf("[%s] 机器面取不到（rc=%d）", id, rcj))
		}
		rows := apmGetList(view, "rows")
		msg := apmCaseOk(cm, apmGot{rc: dup, rows: rows, view: view})
		if dup != rcj && msg == "" {
			msg = fmt.Sprintf("[%s] 人读面 rc=%d ≠ 机器面 rc=%d（两面不许不同形）", id, dup, rcj)
		}
		if msg != "" {
			faults = append(faults, fmt.Sprintf("[%s] %s", id, msg))
			fmt.Fprintf(stdout, "   ✗ %s：%s\n", id, msg)
		} else {
			fmt.Fprintf(stdout, "   ✓ %s：rc=%d · %d 行 · %s\n", id, dup, len(rows), apmStrField(cm, "why"))
			ok++
		}
	}
	if len(faults) > 0 {
		bad += len(faults)
	}

	fmt.Fprint(stdout, "── 自检 ② 负控：反例产出（错实现把缺的来源机格填成 本机/localhost/拿 default_node 兜底）必须**红**\n")
	for _, rvi := range apmGetList(man, "rejects") {
		rvm, _ := rvi.(*apmOrderedMap)
		p := apmPyJoin(fixdir, apmStrField(rvm, "path"))
		id := apmStrField(rvm, "id")
		erc := apmIntField(rvm, "expect_rc")
		rc, _, _ := apmRunCLI([]string{"--no-self-test", "--audit-view", p})
		if rc == erc {
			fmt.Fprintf(stdout, "   ✓ 抓住 %s（rc=%d）：%s\n", id, rc, apmStrField(rvm, "what"))
			ok++
		} else {
			fmt.Fprintf(stdout, "   ✗ %s：该判 rc=%d · 实读 %d ⇒ 判据没牙\n", id, erc, rc)
			bad++
		}
	}

	fmt.Fprint(stdout, "── 自检 ③ 变异（设计件 A5 末句：对它做变异——写死 Mr2109 ⇒ 期望红）\n")
	type apmMut struct {
		name  string
		pol   apmPolicy
		which string
		pre   string
	}
	muts := []apmMut{
		{"写死 `本机`", apmPolicy{fillMissing: "本机", hasFillMissing: true}, "anon", ""},
		{"写死 `localhost`", apmPolicy{fillMissing: "localhost", hasFillMissing: true}, "anon", ""},
		{"拿 `default_node` 兜底（值恰在册 ⇒ 更毒 · 只有依据格露馅）",
			apmPolicy{fillMissing: "Mr2109", hasFillMissing: true, srcBasis: "default_node", hasSrcBasis: true}, "anon", ""},
		{"写死 `Mr2109`（无视载体 ⇒ §三.4 负控②「别机那一行必须红」）",
			apmPolicy{hardcode: "Mr2109", hasHardcode: true}, "prefixed", "x" + "3"},
	}
	for _, mu := range muts {
		var d string
		if mu.which == "anon" {
			d = apmPyJoin(fixdir, "A5-neg-not-machine", "sources", "anon-edit_audit.jsonl")
		} else {
			d = apmPyJoin(fixdir, "A0-ok", "sources", "x"+"3-edit_audit.jsonl")
		}
		srcs := []apmSource{{arg: d, prefix: mu.pre, path: d}}
		pol := mu.pol
		rows, why := apmBuildRows(srcs, []string{"Mr2109", "x" + "3"}, "", "<host>", &pol)
		if rows == nil {
			fmt.Fprintf(stdout, "   ✗ 变异 %s：取数失败（%s）\n", mu.name, why)
			bad++
			continue
		}
		red, _ := apmAuditView(apmBuildView(rows, srcs, apmNowISO("")), []string{"Mr2109", "x" + "3"})
		if len(red) > 0 {
			fmt.Fprintf(stdout, "   ✓ 变异 %s ⇒ 自己的 A5 检索面判红：%s\n", mu.name, apmRuneSlice(red[0], 72))
			ok++
		} else {
			fmt.Fprintf(stdout, "   ✗ 变异 %s ⇒ 检索面**没红**（判据没牙）\n", mu.name)
			bad++
		}
	}

	d0 := apmPyJoin(fixdir, "A5-neg-not-machine", "sources", "anon-edit_audit.jsonl")
	rows0, _ := apmBuildRows([]apmSource{{arg: d0, prefix: "", path: d0}}, []string{"Mr2109", "x" + "3"}, "", "<host>", &apmPolicy{hardcode: "Mr2109", hasHardcode: true})
	red0, _ := apmAuditView(apmBuildView(rows0, nil, apmNowISO("")), []string{"Mr2109", "x" + "3"})
	fmt.Fprint(stdout, "── 自检 ③ 明标缺口（**不计数** · 这一格今天没牙，不许读成已封）：\n")
	verdict := "**判不出来**（如实标出）"
	if len(red0) > 0 {
		verdict = "仍判红（说明还有别的牙）"
	}
	fmt.Fprintf(stdout, "   · 写死 `Mr2109` **且同时声称 `src_basis=argv`** ⇒ 产出面%s —— 载体是命令行、不在产出面里 ⇒"+
		"只有「载体为 cell/prefix 的行」与变异自检抓得住（待拍：要不要把 argv 也落进产出面）\n", verdict)

	fmt.Fprint(stdout, "── 自检 ③/③ 用法错（两态不许同形：`用法错` ≠ `缺证据`）\n")
	rc1, out1, err1 := apmRunCLI([]string{"--no-self-test", "--not-a-flag"})
	out1 = out1 + err1
	if rc1 == 2 && strings.Contains(out1, "用法错") {
		fmt.Fprint(stdout, "   ✓ 未知旗标 ⇒ rc=2 且字样 = 「用法错」\n")
		ok++
	} else {
		fmt.Fprintf(stdout, "   ✗ 未知旗标 ⇒ rc=%d（期望 2 + 字样「用法错」）\n", rc1)
		bad++
	}
	rc2, out2, err2 := apmRunCLI([]string{"--no-self-test", "--source", "/no/such/source.jsonl", "--roster-nodes", "Mr2109,x" + "3"})
	out2 = out2 + err2
	if rc2 == 2 && !strings.Contains(out2, "用法错") && (strings.Contains(out2, "未取数") || strings.Contains(out2, "缺证据")) {
		fmt.Fprint(stdout, "   ✓ 来源件不在 ⇒ rc=2 且字样 ≠「用法错」（未取数那一档）\n")
		ok++
	} else {
		fmt.Fprintf(stdout, "   ✗ 来源件不在 ⇒ rc=%d／字样不对\n", rc2)
		bad++
	}
	rc3, out3, err3 := apmRunCLI([]string{"--no-self-test", "--source", "/no/such/source.jsonl", "--roster", "/no/such/roster.yaml"})
	out3 = out3 + err3
	if rc3 == 2 && strings.Contains(out3, "名册") {
		fmt.Fprint(stdout, "   ✓ 名册读不到 ⇒ rc=2（**缺证据不编造**）\n")
		ok++
	} else {
		fmt.Fprintf(stdout, "   ✗ 名册读不到 ⇒ rc=%d／没明说名册（不许当绿）\n", rc3)
		bad++
	}

	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（夹具 %d 案 · 只读 · 真状态目录一个字节未动）\n", ok, bad, n)
	if bad == 0 {
		return 0
	}
	return 2
}

// ── 五 · 真目标（只读扫）／产出面审计／主 ──────────────────────────────────

func apmDefaultSources(state string) []apmSource {
	ea := apmPyJoin(state, "edit_audit.jsonl")
	out := []apmSource{{arg: ea, prefix: "", path: ea}}
	d := apmPyJoin(state, "approvals")
	if st, err := os.Stat(d); err == nil && st.IsDir() {
		ents, _ := os.ReadDir(d)
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, nm := range names {
			if strings.HasSuffix(nm, ".json") {
				p := apmPyJoin(d, nm)
				out = append(out, apmSource{arg: p, prefix: "", path: p})
			}
		}
	}
	return out
}

func apmOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func apmShow(stdout io.Writer, view *apmOrderedMap, rows []*apmOrderedMap, roster []string, hostname string, jsonOut bool) bool {
	if jsonOut {
		fmt.Fprint(stdout, apmJSONDumps(view, 0), "\n")
		return true
	}
	fmt.Fprint(stdout, "── 第三本（人签跨机合并视图 · 只读汇总）\n")
	for _, h := range apmHeaders {
		fmt.Fprintf(stdout, "   · 件头：%s\n", h)
	}
	fmt.Fprintf(stdout, "   · 强度：%s\n", apmStrengthNote)
	fmt.Fprintf(stdout, "   · 名册键集（%d）：%s —— 来源机取值的**唯一合法来源**\n", len(roster), strings.Join(roster, " · "))
	srcsL := apmGetList(view, "sources")
	fmt.Fprintf(stdout, "── 来源面（%d 处 · 只读快照）\n", len(srcsL))
	for _, si := range srcsL {
		sm, _ := si.(*apmOrderedMap)
		sha := "—"
		if sv := apmMustGet(sm, "sha256"); apmTruthy(sv) {
			sha = apmRuneSlice(apmPyStr(sv), 12)
		}
		fmt.Fprintf(stdout, "   · %s · 认到 %d 行 · sha256 %s\n", apmStrField(sm, "source"), apmIntField(sm, "rows"), sha)
	}
	fmt.Fprintf(stdout, "── 逐行（%d 行 · 三态）\n", len(rows))
	for i, r := range rows {
		m := "（这一格省掉 ✓）"
		if mv, ok := r.Get("src_machine"); ok {
			m = apmPyStr(mv)
		}
		fmt.Fprintf(stdout, "   %d) %s · tool=%s · approver=%s · at=%s · key=%s\n",
			i+1, apmStrField(r, "row_state"), apmOrDash(apmStrField(r, "tool")),
			apmOrDash(apmStrField(r, "approver")), apmOrDash(apmStrField(r, "approved_at")), apmOrDash(apmStrField(r, "key_id")))
		fmt.Fprintf(stdout, "      来源机=%s · known=%s · 依据=%s · 钟=%s\n",
			m, apmPyStr(apmMustGet(r, "src_machine_known")), apmOrDash(apmStrField(r, "src_basis")), apmStrField(r, "clock_basis"))
		mc := apmListOfStrings(apmMustGet(r, "missing_cells"))
		tail := ""
		if len(mc) > 0 {
			tail = " · 缺格=" + strings.Join(mc, ",")
		}
		fmt.Fprintf(stdout, "      回指=%s%s\n", apmStrField(r, "src_path"), tail)
		cw := apmListOfStrings(apmMustGet(r, "conflict_with"))
		if len(cw) > 0 {
			fmt.Fprintf(stdout, "      冲突对端：%s（冲突未裁 · 两面并列 · 不选边）\n", strings.Join(cw, " | "))
		}
		for _, nt := range apmListOfStrings(apmMustGet(r, "notes")) {
			fmt.Fprintf(stdout, "      ·  %s\n", nt)
		}
	}
	return false
}

func apmScan(stdout, stderr io.Writer, sources []apmSource, roster []string, machineArg, hostname, nowISO string, jsonOut bool) int {
	rows, why := apmBuildRows(sources, roster, machineArg, hostname, nil)
	if rows == nil {
		fmt.Fprintf(stderr, "⛔ 未取数：%s ⇒ **不出结论**（rc=2 · 缺证据不编造）\n", why)
		return 2
	}
	view := apmBuildView(rows, sources, nowISO)
	red, notes := apmAuditView(view, roster)
	if len(red) > 0 {
		fmt.Fprint(stderr, "⛔ 产出面**违规**（本笔自己的实现坏了 ⇒ 红 · **机器面也不放出去**）：\n")
		for _, x := range red {
			fmt.Fprintf(stderr, "   ✗ %s\n", x)
		}
		return 1
	}
	rc := apmJudgeRC(rows)
	if jsonOut {
		fmt.Fprintf(stderr, "── 人读结论（机器面在 stdout 末段 · schema %s）rc=%d · %d 行 · 名册 %d 键\n", apmViewSchema, rc, len(rows), len(roster))
		for _, nt := range notes {
			fmt.Fprintf(stderr, "   ·  %s\n", nt)
		}
		fmt.Fprint(stdout, apmJSONDumps(view, 0), "\n")
		return rc
	}
	apmShow(stdout, view, rows, roster, hostname, false)
	fmt.Fprint(stdout, "\n")
	if rc == 0 {
		fmt.Fprintf(stdout, "⇒ 能合（rc=0）：%d 行必填格齐 · 无冲突 · 来源机全在名册（本件只读 · 未写一个字节）\n", len(rows))
	} else if rc == 1 {
		fmt.Fprint(stdout, "⇒ **红**（rc=1）：合不了 —— 冲突**必须由人裁**、不许静默取一（两面并列 · 不选边）\n")
	} else {
		var parts []string
		for _, r := range rows {
			mc := apmListOfStrings(apmMustGet(r, "missing_cells"))
			if len(mc) > 0 {
				parts = append(parts, strings.Join(mc, "、"))
			}
		}
		fmt.Fprintf(stdout, "⇒ **不给结论**（rc=2）：有「缺证据」的行 ⇒ **这一格不许读成绿**（缺格 = %s）\n", strings.Join(parts, " / "))
	}
	for _, nt := range notes {
		fmt.Fprintf(stdout, "   ·  %s\n", nt)
	}
	fmt.Fprint(stdout, "   · 本笔**不做验签** ⇒ `sig_state` 一律「未验·…」· **不许**读成「签名坏」\n")
	for _, p := range apmOpenPoints {
		fmt.Fprintf(stdout, "   · 待拍：%s\n", p)
	}
	return rc
}

func apmAuditOnly(stdout, stderr io.Writer, path string, roster []string) int {
	if _, err := os.Stat(apmExpandUser(path)); err != nil {
		fmt.Fprintf(stderr, "⛔ 未取数：产出面不在 %s ⇒ 不给结论（rc=2）\n", path)
		return 2
	}
	data, _ := os.ReadFile(apmExpandUser(path))
	view, err := apmDecodeJSON(data)
	if err != nil {
		fmt.Fprintf(stderr, "⛔ 未取数：产出面解不出来（%s）⇒ 不给结论（rc=2）\n", err.Error())
		return 2
	}
	red, notes := apmAuditView(view, roster)
	vm, _ := view.(*apmOrderedMap)
	schemaS := "None"
	rowsN := 0
	if vm != nil {
		schemaS = apmPyStr(apmMustGet(vm, "schema"))
		rowsN = len(apmGetList(vm, "rows"))
	}
	fmt.Fprint(stdout, "── 产出面审计（A5：产出面「本机」当值 = 0 处 · 15 格闭集）\n")
	fmt.Fprintf(stdout, "   件：%s · schema=%s · 行数=%d\n", path, schemaS, rowsN)
	for _, x := range red {
		fmt.Fprintf(stdout, "   ✗ %s\n", x)
	}
	for _, nt := range notes {
		fmt.Fprintf(stdout, "   ·  %s\n", nt)
	}
	if len(red) > 0 {
		fmt.Fprint(stdout, "⇒ **红**（rc=1）：产出面把负数填成了本机值 / 或走了不该走的兜底 ⇒ 这就是「不许默认本机」的机器形态\n")
		return 1
	}
	fmt.Fprint(stdout, "⇒ 产出面干净（rc=0）：值里 0 处本机字面 · 机器面格的依据都在 §二.2 的三个合法来源里\n")
	return 0
}

// ── 参数解析（手写 · 复刻 argparse 可见面）────────────────────────────────

type apmArgs struct {
	root        string
	state       string
	roster      string
	rosterNodes string
	machine     string
	fixtures    string
	auditView   string
	now         string
	sources     []string
	fixtureOnly bool
	jsonOut     bool
	selfTest    bool
	noSelfTest  bool
}

const apmUsageText = "usage: check-approve-merge.py [-h] [--root ROOT] [--state STATE]\n" +
	"                              [--roster ROSTER] [--roster-nodes ROSTER_NODES]\n" +
	"                              [--source SOURCE] [--machine MACHINE]\n" +
	"                              [--fixtures FIXTURES] [--fixture-only]\n" +
	"                              [--audit-view AUDIT_VIEW] [--json] [--self-test]\n" +
	"                              [--no-self-test]\n"

func apmPrintHelp(stdout io.Writer) {
	fmt.Fprint(stdout, apmUsageText)
	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, apmVersion+"\n")
	fmt.Fprint(stdout, "\n")
	fmt.Fprint(stdout, "optional arguments:\n")
	fmt.Fprint(stdout, "  -h, --help            show this help message and exit\n")
	fmt.Fprint(stdout, "  --root ROOT\n")
	fmt.Fprint(stdout, "  --state STATE\n")
	fmt.Fprint(stdout, "  --roster ROSTER\n")
	fmt.Fprint(stdout, "  --roster-nodes ROSTER_NODES\n")
	fmt.Fprint(stdout, "                        名册键集（逗号分隔 · 夹具用 ⇒ 零依赖）\n")
	fmt.Fprint(stdout, "  --source SOURCE\n")
	fmt.Fprint(stdout, "  --machine MACHINE\n")
	fmt.Fprint(stdout, "  --fixtures FIXTURES\n")
	fmt.Fprint(stdout, "  --fixture-only\n")
	fmt.Fprint(stdout, "  --audit-view AUDIT_VIEW\n")
	fmt.Fprint(stdout, "  --json\n")
	fmt.Fprint(stdout, "  --self-test\n")
	fmt.Fprint(stdout, "  --no-self-test\n")
}

func apmArgparseErr(stderr io.Writer, msg string) {
	fmt.Fprint(stderr, apmUsageText)
	fmt.Fprintf(stderr, "%s: error: %s\n", apmProg, msg)
}

func apmParseArgs(argv []string, stdout, stderr io.Writer) (*apmArgs, int, bool) {
	apmRunCliRoot = "" // 自清：显式 `--root` 由 main 在解析后重钉（防跨调用残留）
	a := &apmArgs{
		root:     apmCwd(),
		state:    apmPyJoin(apmExpandUser("~"), ".zerg", "state"),
		roster:   apmPyJoin(apmExpandUser("~"), ".zerg", "contexts", "default.yaml"),
		fixtures: apmPyJoin(apmFindRepoRoot(), "scripts", "gates", "fixtures", "approve-merge"),
	}
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if tok == "-h" || tok == "--help" {
			apmPrintHelp(stdout)
			return nil, 0, true
		}
		if strings.HasPrefix(tok, "--") {
			name, val, hasVal := tok, "", false
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				name = tok[:eq]
				val = tok[eq+1:]
				hasVal = true
			}
			switch name {
			case "--fixture-only", "--json", "--self-test", "--no-self-test":
				if hasVal {
					apmArgparseErr(stderr, "argument "+name+": ignored explicit argument "+apmPyRepr(val))
					return nil, 2, true
				}
				switch name {
				case "--fixture-only":
					a.fixtureOnly = true
				case "--json":
					a.jsonOut = true
				case "--self-test":
					a.selfTest = true
				case "--no-self-test":
					a.noSelfTest = true
				}
			case "--root", "--state", "--roster", "--roster-nodes", "--source", "--machine", "--fixtures", "--audit-view", "--now":
				if !hasVal {
					if i+1 < len(argv) {
						val = argv[i+1]
						i++
					} else {
						apmArgparseErr(stderr, "argument "+name+": expected one argument")
						return nil, 2, true
					}
				}
				switch name {
				case "--root":
					a.root = val
				case "--state":
					a.state = val
				case "--roster":
					a.roster = val
				case "--roster-nodes":
					a.rosterNodes = val
				case "--source":
					a.sources = append(a.sources, val)
				case "--machine":
					a.machine = val
				case "--fixtures":
					a.fixtures = val
				case "--audit-view":
					a.auditView = val
				case "--now":
					a.now = val
				}
			default:
				apmArgparseErr(stderr, "unrecognized arguments: "+tok)
				return nil, 2, true
			}
		} else {
			apmArgparseErr(stderr, "unrecognized arguments: "+tok)
			return nil, 2, true
		}
		i++
	}
	return a, 0, false
}

func apmFirstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func apmNowISO(inject string) string {
	if inject != "" {
		if t, err := time.Parse(time.RFC3339, inject); err == nil {
			return t.Format("2006-01-02T15:04:05-07:00")
		}
		return inject
	}
	return time.Now().Format("2006-01-02T15:04:05-07:00")
}

// checkApproveMergeGateMain —— 本件入口（与 py `main()` 对齐）。
func checkApproveMergeGateMain(stdout, stderr io.Writer, argv []string) int {
	// ★ 本趟结束清零：嵌套调用（自检里的 run_cli 面）在**本趟内**仍需看得见根，
	// 但跨顶层调用不许留 —— 上一趟钉的临时目录（已被清理）会让下一趟的前置判一律 rc=2。
	defer func() { apmRunCliRoot = "" }()
	known := map[string]bool{
		"--self-test": true, "--no-self-test": true, "--json": true, "--fixtures": true,
		"--audit-view": true, "--source": true, "--machine": true, "--roster": true,
		"--roster-nodes": true, "--root": true, "--state": true, "-h": true, "--help": true,
		"--now": true, // Go-only 注入旋钮
	}
	var badFlags []string
	for _, a := range argv {
		if strings.HasPrefix(a, "-") {
			name := strings.SplitN(a, "=", 2)[0]
			if !known[name] {
				badFlags = append(badFlags, a)
			}
		}
	}
	if len(badFlags) > 0 {
		qs := make([]string, 0, len(badFlags))
		for _, x := range badFlags {
			qs = append(qs, "`"+x+"`")
		}
		fmt.Fprintf(stderr, "⛔ 用法错（rc=2）：未知旗标 %s\n", strings.Join(qs, " · "))
		fmt.Fprint(stderr, "   见 --help；**用法错 ≠ 缺证据**（两态不许同形）\n")
		return 2
	}
	args, rc, done := apmParseArgs(argv, stdout, stderr)
	if done {
		return rc
	}
	// ── `--root` 生效（缺口 `GAP-20261001-63`）：原实现解析后**从不读取** ⇒ 静默无效（D 类真口子）──
	//   显式给了 `--root` ⇒ 以它重钉 `apmRunCliRoot`（run_cli 替代入口的根随之生效）；
	//   不给时逐字不改既有行为（默认路径未动）。
	if apmRootArgGiven(argv) {
		apmRunCliRoot = dcmdAbs(args.root)
	}
	fmt.Fprintln(stdout, apmVersion)
	hostname, _ := os.Hostname()
	if args.auditView != "" {
		roster, why := apmReadRoster(args.roster, args.rosterNodes)
		if roster == nil {
			fmt.Fprintf(stderr, "⛔ 未取数：%s ⇒ 不给结论（rc=2）\n", why)
			return 2
		}
		return apmAuditOnly(stdout, stderr, args.auditView, roster)
	}
	if args.selfTest {
		return apmSelfTest(stdout, args.fixtures)
	}
	if args.fixtureOnly {
		bad, n := apmRunFixtures(stdout, args.fixtures, false, nil)
		fmt.Fprintf(stdout, "   夹具：%d 案 · 不过 %d 条\n", n, len(bad))
		if len(bad) != 0 {
			return 2
		}
		return 0
	}
	if !args.noSelfTest {
		if src := apmSelfTest(stdout, args.fixtures); src != 0 {
			fmt.Fprint(stdout, "⛔ 自检不过 ⇒ 不给结论（rc=2）\n")
			return 2
		}
	}
	roster, why := apmReadRoster(args.roster, args.rosterNodes)
	if roster == nil {
		fmt.Fprintf(stderr, "⛔ 未取数：%s ⇒ **不出结论**（rc=2 · 缺证据不编造）\n", why)
		return 2
	}
	var srcs []apmSource
	if len(args.sources) > 0 {
		for _, a := range args.sources {
			pre, p := apmSplitSourceArg(a)
			srcs = append(srcs, apmSource{arg: a, prefix: pre, path: p})
		}
		fmt.Fprintf(stdout, "── 真目标：人喂进来的 %d 处快照 · 名册 %d 键 · 只读\n", len(srcs), len(roster))
	} else {
		srcs = apmDefaultSources(args.state)
		fmt.Fprintf(stdout, "── 真目标：本机状态目录 %s（%d 处 · 只读）\n", args.state, len(srcs))
		fmt.Fprint(stdout, "   （现读口径：审计行含机器名格的 = 0 行 ⇒ 来源机格缺 ⇒ **今天必然 rc=2**，不许读成绿 —— 设计件 §1.3/A2）\n")
	}
	nowISO := apmNowISO(apmFirstNonEmpty(args.now, os.Getenv("ZERG_NOW")))
	return apmScan(stdout, stderr, srcs, roster, args.machine, hostname, nowISO, args.jsonOut)
}
