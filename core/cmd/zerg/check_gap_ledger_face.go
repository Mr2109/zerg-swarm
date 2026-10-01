// check_gap_ledger_face.go —— 门禁件 `scripts/gates/check-gap-ledger-face.py` 的 **Go 真搬**（M2 · 账面面貌类 · 路B续9）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	M1 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面（转发壳）；M2 按价值**真搬 Go**
//	（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。
//	本件 = M2「账面面貌类」的一枚：验收判据 = 对同一件跑两态（原件 ↔ 新入口），
//	stdout / stderr / 退码三面逐字相同。
//
// 选它 / 不选它的理由（本件适格面）：
//
//	✓ 选 `check-gap-ledger-face.py`（447 行 · 键名 `gap-ledger-face` · 首字母 g）：
//	  退码仅 0/1/2 · **纯静态只读**（主路径无写盘 · 无 subprocess · 无 env 旋钮）·
//	  输出**确定有序**（面表写死 4 面 · 行序 = 盘上行序 · 值集合按 key 排序或按首见序）·
//	  外部树依赖**已由 py 自带显式旋钮**（`--docs-root` / `--register`）承接 ⇒ 本件照名搬旋钮。
//	✗ 本批其余候选（`check-gap-ledger-view.py` 1030 行等）多为「行数偏重 / 读 git 现跑 /
//	  os.walk 遍历序 / 写面」⇒ 不属本单。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-gap-ledger-face.py` 原件（退场规则：对拍通过后才退场，逐件点名）；
//	· 入口 `checkGapLedgerFaceGateMain` 目前**无调用点** —— 这是 M2 的**刻意**状态。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkGapLedgerFaceGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 值域内 / 1 值域外或登记件不符 / 2 不给结论（缺件·表头找不到·空转·用法错）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ★★ 标准声明 (a)：**默认根定位差异 —— 有差异（且本单对拍显式规避）**。
//
//	py 的 `--root` 缺省 = `DEFAULT_ROOT = dirname(dirname(abspath(__file__)))` = **脚本所在
//	目录（`scripts/gates/`）上溯两层的仓根**；`--docs-root` 缺省 = `dirname(root)/Zerg-内部文档`。
//	本件**无 `__file__` 语境**（未来接线后是 CLI 子命令）⇒ `--root` 缺省换成**当前工作目录**
//	（`filepath.Abs("")` → cwd；与 check-judge-ledger / check-private-path-forms 两枚样板同一条口径）。
//	落在**同一真仓、cwd = 仓根**时两者对 `docs` / `register` 的推导**逐字相同**（甲组对拍已证）；
//	「脚本位置 ≠ cwd」且都不给 `--root` 时**路径推导面不同** ⇒ 这一面**不属逐字对齐承诺**。
//	★ 为令「不传根」这一态也可对拍：把 py 原件副本放 `/…/fx/scripts/gates/` 并以 `cwd = fx`
//	  运行 ⇒ py 由 `__file__` 得 root=fx、Go 由 cwd 得 root=fx，**默认根重合**（乙组对拍）。
//	★ `--docs-root` / `--register` / `--no-register` 三枚**照 py 同名旗标**逐字搬：缺省推导
//	  （sibling `Zerg-内部文档` · `docs_root/REGISTER_REL`）与 py 逐字同构。
//
// ★★ 标准声明 (b)：**自指四形态 —— 逐条真判**：
//
//	① 本件在**自己**的扫描面内吗？—— **否**。本门扫描面 = 调用方**显式点名的外部文档仓**
//	   （缺省 = `dirname(root)/Zerg-内部文档`）里那**两件账 + 一件登记件**（三条相对路径写死）。
//	   本件落 `core/cmd/zerg/check_gap_ledger_face.go`，**不在**文档仓内；本件只有三类读点
//	   （`os.Stat` / `os.ReadFile` 各按显式路径），**零 `WalkDir` / 零 `os.ReadDir` / 零 glob**
//	   ⇒ 本门遍历**走不到本件自己**（py 原件同理：只 `isfile` + `open` 三件）。
//	② 本件落在**任何他门**的扫描面内吗？—— **是**，已实测处置。`scripts/gates/check-hardcoded-private-paths.py`
//	   的 `SCAN_DIRS = ("core","agent","ui/src")` 含 `core/` ⇒ 本件在其扫描面内（注释行豁免 ·
//	   `*_test.go` 整体豁免）。该门 `NEEDLES` 判**三个私有卷/家目录绝对路径前缀**，本件正文
//	   **零该三字面**（本件要判的是**仓外文档仓相对路径**与**值域字面量**，不含任何绝对私有路径）
//	   ⇒ 该门对本件 0 命中。**证明**：落仓后 `check-hardcoded-private-paths.py` 对 `core/` 的
//	   命中数**逐字不变**（见交付读数）。★ 本条纪律：**后续动本件者不得把私有绝对路径原串写进来**。
//	③ 本件**产出**回喂自身输入面（回环）吗？—— **否**。本件**只读**（py 件头「本件只读」）：
//	   stdout/stderr **不落任何文件**、不追加任何账 ⇒ 无回环面。
//	④ 本件**测试件/夹具**里的标记字面被他门命中吗？—— **否（且被豁免）**。`check-hardcoded-private-paths.py`
//	   对 `*_test.go` **整体跳过**；且本件测试夹具只用 `t.TempDir()` 下的**仓外**路径与合成面
//	   （`账.md` / `登记-合成.md`），**零私有卷字面**。
//
// 其余未覆盖面（✗ 不声称逐字对齐 · 接线前须逐面补齐或显式剔除）：
//
//	① **argparse 前缀缩写 + 用法/帮助正文**：py 用 `argparse` ⇒ `--js` / `--list-r` 这类**唯一前缀**
//	   被接受、`-h/--help` 打 argparse 帮助正文（含 `prog = check-gap-ledger-face.py` 与描述段）、
//	   未知旗标打 `usage:` + `unrecognized arguments:` 正文 —— 本件只认**精确名**，`-h`/`--help`
//	   落自定 usage 行，未知旗标落自定错误行。**退码同（0 / 2）· 正文不同** ⇒ 不进逐字面（对拍输入集已避开）。
//	② **异常路径正文**：账件**存在但读不到**（权限）时 py `open` 抛 `OSError`（rc=1 · traceback）；
//	   本件落 `no_ledger`（rc=2 · fail-closed）—— 退码**不同**，属新口径「取不到一律 rc=2」，单列见回执。
//	③ **★ `--json` × 面缺席 = py 真口子（本件不自行修 · 保留对齐形态并交回入账）**：py 的 json 分支
//	   取 `rd["headers"]` / `rd["rows"]`，而 `no_ledger` / `no_header` 态的 `read` 只有 `{"path": …}`
//	   ⇒ 触发 **`KeyError: 'headers'`**：**stdout 空 · stderr 打 Python traceback · 进程 rc=1**（实测）。
//	   本件同功能分支取「非 full ⇒ 落 Go 错误行 + rc=1 · **stdout 同样为空**」⇒ **退码同（1）· stderr 正文不同**
//	   （Go 无 traceback）。非 json 档的同一输入两态**逐字相同**（rc=2 · 见对拍）。
//	④ **`\s` 宽度**：白名单正则 `^同\s*§` 里 py `\s` 含 Unicode 空白（如 U+00A0），Go RE2 `\s` 仅 ASCII 空白
//	   ⇒ `同<nbsp>§` 这类输入两态判定可能不同；真账现值用**普通空格**，对拍输入集不含该形态。
//	⑤ **行切分**：py `str.splitlines()` 认 `\v \f \x1c-\x1e \x85 \u2028 \u2029` 亦为行界 · 本件已按同集合实现
//	   （`glfSplitLines`）⇒ 这一面**已对齐**（非未覆盖，列此以备忘）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ── py 常量（**唯一真源**：与 py 逐字对齐 · 名字前缀 `glf` 防包内同名）──────────────
const glfVersion = "check-gap-ledger-face.py v1.0.0"

const glfLedgerA = "项目文档/v2.5.10/缺口-命令面-20260921.md"
const glfLedgerB = "项目文档/v2.5.11/缺口-命令面-20260922.md"
const glfRegisterRel = "项目文档/v2.5.12/登记-两套账面闭集-v2.5.12-20260924.md"

// 退码三档（与 py 逐字对齐）。
const (
	glfRCOK        = 0
	glfRCRed       = 1
	glfRCNoVerdict = 2
)

// glfFace —— py `FACES` 一格。
type glfFace struct {
	id     string
	label  string
	ledger string
	col    int
	header string
	closed []string
}

var glfFaces = []glfFace{
	{id: "A", label: "v2.5.10 §一–§十 · 现有命令能吗", ledger: glfLedgerA, col: 3,
		header: "| 编号 | 我们要干的事（人话一句） | 今天怎么干的（手搓了什么 · 原样引一句） | 现有命令能吗 | 建议的新命令（形态 + 输入 + 输出 + 退码） | 优先级 |",
		closed: []string{"✓", "半", "无"}},
	{id: "B", label: "v2.5.10 §十三 · 状态", ledger: glfLedgerA, col: 3,
		header: "| 编号 | 事（人话一句） | 今天怎么手搓的（**原样引一句**） | 状态 | 建议命令（形态） | 优先级 |",
		closed: []string{"✓已解", "⟳本轮补", "✗仍缺"}},
	{id: "C", label: "v2.5.11 §一 · 状态", ledger: glfLedgerB, col: 6,
		header: "| 编号 | 现象 | 今天怎么手搓的（必填） | 影响面 | 建议形态（已落地 / 在册） | 优先级 | 状态 |",
		closed: []string{"已解", "已派", "已立项", "仍缺"}},
	{id: "D", label: "v2.5.11 §二/§三 · 状态（表头变体）", ledger: glfLedgerB, col: 6,
		header: "| 编号 | 现象 | 今天怎么手搓的（必填） | 影响面 | 建议形态 | 优先级 | 状态 |",
		closed: []string{"已解", "已派", "已立项", "仍缺"}},
}

// ── 交叉引用白名单（**只有这一条** · 语法形态，不是取值）──────────────────────────
var glfCrossrefRE = regexp.MustCompile(`^同\s*§`)

// ── 语义小件 ─────────────────────────────────────────────────────────────────
// glfStripMarkup —— py `re.sub(r"[` + "`" + `*]", "", s)`：去反引号与星号。
func glfStripMarkup(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '`' || r == '*' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// glfSplitLines —— py `str.splitlines()` 的最小忠实实现（认 \n \r \r\n \v \f \x1c-\x1e \x85 \u2028 \u2029）。
func glfSplitLines(s string) []string {
	var out []string
	var cur []rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch r {
		case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\x85', '\u2028', '\u2029':
			out = append(out, string(cur))
			cur = cur[:0]
		case '\r':
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, string(cur))
			cur = cur[:0]
		default:
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// glfCells —— py `cells(line)`：仅当 strip 后以 `|` 起止才切；`\|`（转义竖线）不作分隔。
func glfCells(line string) []string {
	s := strings.TrimSpace(line)
	if !(strings.HasPrefix(s, "|") && strings.HasSuffix(s, "|")) {
		return nil
	}
	if len(s) < 2 {
		return nil
	}
	inner := s[1 : len(s)-1]
	rs := []rune(inner)
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(rs); i++ {
		if rs[i] == '|' && (i == 0 || rs[i-1] != '\\') {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(rs[i])
	}
	parts = append(parts, cur.String())
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// glfIsSep —— py `is_sep(cs)`：非空且所有字符 ∈ `-: `。
func glfIsSep(cs []string) bool {
	if len(cs) == 0 {
		return false
	}
	for _, r := range strings.Join(cs, "") {
		if r != '-' && r != ':' && r != ' ' {
			return false
		}
	}
	return true
}

// glfNormValue —— py `norm_value(raw)`：去 “ ` “/`*` ⇒ 取到第一个「：/（/(/空格/✅/·」之前。
func glfNormValue(raw string) string {
	s := glfStripMarkup(strings.TrimSpace(raw))
	for _, cut := range []string{"：", "（", "(", " ", "✅", "·"} {
		if i := strings.Index(s, cut); i > 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

// glfOrdered —— 有序计数器（对齐 py `collections.Counter`：**首见序** · 计数）。
type glfOrdered struct {
	keys []string
	m    map[string]int
}

func newGlfOrdered() *glfOrdered { return &glfOrdered{m: map[string]int{}} }

func (o *glfOrdered) add(k string) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k]++
}

func (o *glfOrdered) sortedKeys() []string {
	ks := append([]string{}, o.keys...)
	sort.Strings(ks)
	return ks
}

// glfJSONStr —— py `json.dumps(..., ensure_ascii=False)` 的字符串转义（不转义非 ASCII · 不转义 `/`）。
func glfJSONStr(s string) string {
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

func (o *glfOrdered) json() string {
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(glfJSONStr(k))
		b.WriteString(": ")
		b.WriteString(strconv.Itoa(o.m[k]))
	}
	b.WriteByte('}')
	return b.String()
}

// ── 读面 ─────────────────────────────────────────────────────────────────────
// glfFaceRead —— py `read`：缺席态只有 `{"path"}`；full 态另有 headers/header_lines/rows/values。
type glfFaceRead struct {
	path        string
	full        bool
	headers     int
	headerLines []int
	rows        int
	values      *glfOrdered
}

func (r glfFaceRead) json() string {
	if !r.full {
		return `{"path": ` + glfJSONStr(r.path) + `}`
	}
	var b strings.Builder
	b.WriteString(`{"path": ` + glfJSONStr(r.path) + `, "headers": ` + strconv.Itoa(r.headers) + `, "header_lines": [`)
	for i, l := range r.headerLines {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Itoa(l))
	}
	b.WriteString(`], "rows": ` + strconv.Itoa(r.rows) + `, "values": ` + r.values.json() + `}`)
	return b.String()
}

// glfBad —— py `bad` 一格（值域外）。
type glfBad struct {
	ledger string
	line   int
	value  string
	raw    string
	face   string
}

func (bd glfBad) json() string {
	return `{"ledger": ` + glfJSONStr(bd.ledger) + `, "line": ` + strconv.Itoa(bd.line) +
		`, "value": ` + glfJSONStr(bd.value) + `, "raw": ` + glfJSONStr(bd.raw) +
		`, "face": ` + glfJSONStr(bd.face) + `}`
}

// glfProblem —— py `problems` 一格：面态（带 read）或登记件态（带 msg）。
type glfProblem struct {
	face  string
	state string
	read  glfFaceRead
	msg   string
}

func (p glfProblem) json() string {
	rd := p.read.json()
	if p.msg != "" {
		rd = `{"msg": ` + glfJSONStr(p.msg) + `}`
	}
	return `{"face": ` + glfJSONStr(p.face) + `, "state": ` + glfJSONStr(p.state) + `, "read": ` + rd + `}`
}

func glfContains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func glfIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func glfIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// glfScanFace —— py `scan_face()`：返回 (状态 ∈ ok/no_ledger/no_header/empty, 读数, 越界清单)。
func glfScanFace(docs string, face glfFace) (string, glfFaceRead, []glfBad) {
	path := filepath.Join(docs, face.ledger)
	if !glfIsFile(path) {
		return "no_ledger", glfFaceRead{path: path}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil { // 见件头未覆盖面②：py 抛 OSError(rc=1)，本件落 rc=2（取不到 ⇒ 不给结论）
		return "no_ledger", glfFaceRead{path: path}, nil
	}
	lines := glfSplitLines(string(data))
	ncol := len(glfCells(face.header))
	var occ []int
	for i, l := range lines {
		if strings.TrimSpace(l) == face.header {
			occ = append(occ, i)
		}
	}
	if len(occ) == 0 {
		return "no_header", glfFaceRead{path: path}, nil
	}
	values := newGlfOrdered()
	rows := 0
	var bad []glfBad
	for _, o := range occ {
		for i := o + 2; i < len(lines); i++ {
			cs := glfCells(lines[i])
			if cs == nil || len(cs) != ncol {
				break
			}
			if glfIsSep(cs) {
				continue
			}
			rows++
			raw := cs[face.col]
			// 白名单判在**归一之前**：`同 §四` 是「跨节指路」这一种**语法形态**。
			if glfCrossrefRE.MatchString(glfStripMarkup(strings.TrimSpace(raw))) {
				values.add("同 §（跨节指路·白名单）")
				continue
			}
			v := glfNormValue(raw)
			values.add(v)
			if !glfContains(face.closed, v) {
				bad = append(bad, glfBad{ledger: face.ledger, line: i + 1, value: v, raw: raw, face: face.id})
			}
		}
	}
	hl := make([]int, len(occ))
	for i, o := range occ {
		hl[i] = o + 1
	}
	read := glfFaceRead{path: path, full: true, headers: len(occ), headerLines: hl, rows: rows, values: values}
	if rows == 0 {
		return "empty", read, bad
	}
	return "ok", read, bad
}

// glfScanRegister —— py `scan_register()`：① 每面闭集值逐字在册；② 每面现读计数句在册。
func glfScanRegister(path string, faces []glfFace, reads map[string]glfFaceRead) []string {
	if !glfIsFile(path) {
		return []string{"登记件不在盘：" + path}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"登记件不在盘：" + path}
	}
	txt := string(data)
	var problems []string
	for _, face := range faces {
		for _, v := range face.closed {
			if !strings.Contains(txt, "`"+v+"`") && !strings.Contains(txt, v) {
				problems = append(problems, fmt.Sprintf("登记件里找不到面 %s 的闭集值 `%s`", face.id, v))
			}
		}
		rd, ok := reads[face.id]
		if ok && rd.full && rd.rows != 0 {
			line := fmt.Sprintf("面 %s · 表头 %d 处 · 数据行 %d", face.id, rd.headers, rd.rows)
			if !strings.Contains(txt, line) {
				problems = append(problems, fmt.Sprintf("登记件与现读不一致：缺计数句「%s」", line))
			}
		}
	}
	return problems
}

// glfReport —— 一面一趟跑的结果。
type glfReport struct {
	face  glfFace
	state string
	read  glfFaceRead
}

func glfLastSeg(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// glfRun —— py `run()`。
func glfRun(stdout io.Writer, root, docsRoot, register string, jsonOut bool) int {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		rootAbs = root
	}
	if !glfIsDir(filepath.Join(rootAbs, "scripts", "gates")) {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：`%s` 下没有 `scripts/gates/` ⇒ 判不了根\n", rootAbs)
		return glfRCNoVerdict
	}
	docs := docsRoot
	if docs == "" {
		docs = filepath.Join(filepath.Dir(rootAbs), "Zerg-内部文档")
	}
	if abs, e := filepath.Abs(docs); e == nil {
		docs = abs
	}
	if !glfIsDir(docs) {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：文档仓根不在盘：`%s`（不拿「读不到」当「没问题」）\n", docs)
		return glfRCNoVerdict
	}

	var reports []glfReport
	var badAll []glfBad
	var problems []glfProblem
	reads := map[string]glfFaceRead{}
	for _, face := range glfFaces {
		st, read, bad := glfScanFace(docs, face)
		if st == "no_ledger" || st == "no_header" || st == "empty" {
			problems = append(problems, glfProblem{face: face.id, state: st, read: read})
		}
		reads[face.id] = read
		badAll = append(badAll, bad...)
		reports = append(reports, glfReport{face: face, state: st, read: read})
	}
	if register != "" {
		for _, m := range glfScanRegister(register, glfFaces, reads) {
			problems = append(problems, glfProblem{face: "-", state: "register", msg: m})
		}
	}

	if jsonOut {
		// ★ 见件头未覆盖面③：py 在此对缺席面取 rd["headers"] ⇒ KeyError（rc=1 · stdout 空）。
		//   本件同退码（1）· stdout 同为空，仅 stderr 正文不同（Go 无 traceback）。
		for _, r := range reports {
			if !r.read.full {
				fmt.Fprintf(os.Stderr, "KeyError: 'headers'（面 %s 态 %s：非 full 读数无 headers 键）\n", r.face.id, r.state)
				return glfRCRed
			}
		}
		fmt.Fprintln(stdout, glfJSONDoc(reports, badAll, problems, rootAbs, register))
		if len(problems) > 0 {
			return glfRCNoVerdict
		}
		if len(badAll) > 0 {
			return glfRCRed
		}
		return glfRCOK
	}

	fmt.Fprintf(stdout, "── 两套手写账「表头 + 状态值域」闭集自检（%s · `Q-036`）──\n", glfVersion)
	fmt.Fprintf(stdout, "  文档仓根: %s\n", docs)
	regShown := register
	if regShown == "" {
		regShown = "（--no-register ⇒ 本跑不对拍登记件）"
	}
	fmt.Fprintf(stdout, "  登记件  : %s\n", regShown)
	fmt.Fprintf(stdout, "  %-3s %-34s %-6s %-8s %s\n", "面", "账件 · 面", "表头", "数据行", "现读值域（闭集内）")
	for _, r := range reports {
		vals := ""
		if r.read.values != nil {
			ks := r.read.values.sortedKeys()
			parts := make([]string, 0, len(ks))
			for _, k := range ks {
				parts = append(parts, fmt.Sprintf("%s×%d", k, r.read.values.m[k]))
			}
			vals = strings.Join(parts, " ")
		}
		label := r.face.label + " ← " + glfLastSeg(r.face.ledger)
		headers, rows := "-", "-"
		if r.read.full {
			headers = strconv.Itoa(r.read.headers)
			rows = strconv.Itoa(r.read.rows)
		}
		fmt.Fprintf(stdout, "  %-3s %-34s %-6s %-8s %s\n", r.face.id, label, headers, rows, vals)
		fmt.Fprintf(stdout, "      闭集: %s\n", strings.Join(r.face.closed, " / "))
	}
	sumH, sumR := 0, 0
	for _, r := range reports {
		if r.read.full {
			sumH += r.read.headers
			sumR += r.read.rows
		}
	}
	fmt.Fprintf(stdout, "  ── 合计：%d 面 · 表头 %d 处 · 数据行 %d\n", len(glfFaces), sumH, sumR)

	if len(problems) > 0 {
		fmt.Fprint(stdout, "⛔ 不给结论（rc=2）：\n")
		for _, p := range problems {
			detail := p.msg
			if detail == "" {
				detail = p.read.path
			}
			fmt.Fprintf(stdout, "   · 面 %s %s：%s\n", p.face, p.state, detail)
		}
		return glfRCNoVerdict
	}
	if len(badAll) > 0 {
		fmt.Fprintf(stdout, "✗ 值域外 %d 处（rc=1 · 逐条点名）：\n", len(badAll))
		for _, b := range badAll {
			raw := b.raw
			if rs := []rune(raw); len(rs) > 70 {
				raw = string(rs[:70])
			}
			fmt.Fprintf(stdout, "   · %s:%d 面 %s 值 `%s` ⟵ 原样 `%s`\n", b.ledger, b.line, b.face, b.value, raw)
		}
		fmt.Fprint(stdout, "⇒ 处置（**不是**改本门）：值域要扩 ⇒ ① 改本件 FACES 的 `closed` ② 同笔改仓外登记件 —— 两处同笔，否则下一次跑还会红。\n")
		return glfRCRed
	}
	fmt.Fprint(stdout, "⇒ 值域内（rc=0）：4 个面的每一个值都在登记闭集内（或命中交叉引用白名单 `^同 §`）\n")
	fmt.Fprint(stdout, "   ★ 本件只**读**这两件账：两件账一个字未改（写它们 = 另一笔的活）\n")
	return glfRCOK
}

// glfJSONDoc —— py json 分支的 `json.dumps(doc, ensure_ascii=False)`（手写以保证键序与 py 逐字一致）。
func glfJSONDoc(reports []glfReport, badAll []glfBad, problems []glfProblem, rootAbs, register string) string {
	var b strings.Builder
	b.WriteString(`{"schema": "zerg/v1", "kind": "GapLedgerFace", "tool": `)
	b.WriteString(glfJSONStr(glfVersion))
	b.WriteString(`, "items": [`)
	for i, r := range reports {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"face": ` + glfJSONStr(r.face.id) + `, "label": ` + glfJSONStr(r.face.label) +
			`, "ledger": ` + glfJSONStr(r.face.ledger) + `, "col": ` + strconv.Itoa(r.face.col) +
			`, "closed": ` + glfJSONStrArr(r.face.closed) + `, "headers": ` + strconv.Itoa(r.read.headers) +
			`, "rows": ` + strconv.Itoa(r.read.rows) + `, "values": ` + r.read.values.json() +
			`, "state": ` + glfJSONStr(r.state) + `}`)
	}
	b.WriteString(`], "meta": {"faces": ` + strconv.Itoa(len(glfFaces)) + `, "register": ` + glfJSONStr(register) +
		`, "docs_root": ` + glfJSONStr(filepath.Join(rootAbs, "Zerg-内部文档")) + `}, "out_of_domain": [`)
	for i, bd := range badAll {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(bd.json())
	}
	b.WriteString(`], "problems": [`)
	for i, p := range problems {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.json())
	}
	b.WriteString(`], "warnings": [], "truncated": false}`)
	return b.String()
}

func glfJSONStrArr(xs []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, x := range xs {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(glfJSONStr(x))
	}
	b.WriteByte(']')
	return b.String()
}

// glfListRules —— py `list_rules()`。
func glfListRules(stdout io.Writer) int {
	fmt.Fprintf(stdout, "── %s 判据与登记面 ──\n", glfVersion)
	for _, f := range glfFaces {
		fmt.Fprintf(stdout, "  面 %s · %s\n", f.id, f.label)
		fmt.Fprintf(stdout, "     账件 : %s\n", f.ledger)
		fmt.Fprintf(stdout, "     列   : 第 %d 列（0 基）\n", f.col)
		fmt.Fprintf(stdout, "     表头 : %s\n", f.header)
		fmt.Fprintf(stdout, "     闭集 : %s\n", strings.Join(f.closed, " / "))
	}
	fmt.Fprintf(stdout, "交叉引用白名单（唯一条）：%s\n", glfCrossrefRE.String())
	fmt.Fprint(stdout, "归一法：去 `` ` ``/`*` ⇒ 取到第一个「：/（/(/空格/✅/·」之前（说明那一截不进判据面）\n")
	fmt.Fprint(stdout, "退码：0 值域内 · 1 值域外或登记件与现读不一致 · 2 不给结论（缺件/表头找不到/空转/自检不过）\n")
	fmt.Fprint(stdout, "★ 本笔不挂门（见件头）：挂门是另一笔的活。\n")
	return glfRCOK
}

// ── 自检（成对负控 · 合成夹具 · 不碰真账）─────────────────────────────────────────
var glfFixtureFace = glfFace{
	id: "A", label: "夹具面", ledger: "项目文档/vX/账.md", col: 1,
	header: "| 号 | 状态 | 备注 |", closed: []string{"已解", "仍缺"},
}

func glfWriteFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

// glfPyBool —— py `%s % bool`（True/False）。
func glfPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// glfSelfTest —— py `self_test()` 的 Go 同功能搬（进程内复用 `glfRun` 取真退码 · ✗ 不猜）。
func glfSelfTest(stdout io.Writer) int {
	ok := true
	tmp, err := os.MkdirTemp("", "glf-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "自检结论: **有断言失败**（建临时目录失败：%v）\n", err)
		return glfRCRed
	}
	defer os.RemoveAll(tmp)

	realFaces := glfFaces
	glfFaces = []glfFace{glfFixtureFace}
	defer func() { glfFaces = realFaces }()

	// mk —— `<case>/repo`（仓根）+ `<case>/Zerg-内部文档/项目文档/vX/账.md`（**兄弟布局**）+ 登记件。
	mk := func(rows []string, header string) (repo, docs, ledger, reg string) {
		base := filepath.Join(tmp, fmt.Sprintf("case-%d", len(rows)))
		docs = filepath.Join(base, "Zerg-内部文档")
		repo = filepath.Join(base, "repo")
		_ = os.MkdirAll(filepath.Join(repo, "scripts", "gates"), 0o755)
		_ = os.MkdirAll(filepath.Join(docs, "项目文档", "vX"), 0o755)
		ledger = filepath.Join(docs, "项目文档", "vX", "账.md")
		var lb strings.Builder
		lb.WriteString("# 合成账\n\n")
		lb.WriteString(header + "\n")
		lb.WriteString("|---|---|---|\n")
		for _, r := range rows {
			lb.WriteString(r + "\n")
		}
		lb.WriteString("\n尾。\n")
		_ = glfWriteFile(ledger, lb.String())
		reg = filepath.Join(docs, "项目文档", "vX", "登记-合成.md")
		n := 0
		for _, r := range rows {
			if strings.TrimSpace(r) != "" {
				n++
			}
		}
		_ = glfWriteFile(reg, "闭集 `已解` / `仍缺`\n"+fmt.Sprintf("面 A · 表头 1 处 · 数据行 %d\n", n))
		return repo, docs, ledger, reg
	}
	// casef —— 逐格打印；want/got 用**展示串**（普通格是退码整数 · ②b 格是 bool，与 py `%s` 同）。
	casef := func(name, wantDisp, gotDisp string, good bool, note string) {
		ok = ok && good
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（rc=%s · 期望 %s）%s\n", mark, name, gotDisp, wantDisp, note)
	}
	// cap —— 捕获 run() 的 stdout。
	cap := func(root, docs, reg string, jsonOut bool) (int, string) {
		var buf strings.Builder
		rc := glfRun(&buf, root, docs, reg, jsonOut)
		return rc, buf.String()
	}

	// ① 正控：全部在闭集内 ⇒ rc=0
	r, d, _, reg := mk([]string{"| x | 已解 | a |", "| y | 仍缺 | b |"}, glfFixtureFace.header)
	rc, out := cap(r, d, reg, false)
	casef("① 正控：两个值都在闭集内 ⇒ rc=0", "0", strconv.Itoa(rc), rc == glfRCOK && strings.Contains(out, "值域内"), "")

	// ② 负控：值域外 ⇒ rc=1 且点名到行
	r2, d2, _, reg2 := mk([]string{"| x | 已解 | a |", "| y | 已派 | b |", "| z | 仍缺 | c |"}, glfFixtureFace.header)
	rc, out = cap(r2, d2, reg2, false)
	casef("② 负控：值域外 `已派` ⇒ rc=1", "1", strconv.Itoa(rc), rc == glfRCRed && strings.Contains(out, "值域外"), "")
	pickOK := strings.Contains(out, "账.md:6") && strings.Contains(out, "已派")
	casef("②b 负控点名到「件:行 + 原样值」", "True", glfPyBool(pickOK), pickOK, "  见输出逐条点名")

	// ②c 真话负控：闭集扩到含 `已派`（登记件同笔扩）⇒ 同一夹具回绿
	saved := glfFaces
	glfFaces = []glfFace{{id: "A", label: "夹具面", ledger: "项目文档/vX/账.md", col: 1, header: "| 号 | 状态 | 备注 |", closed: []string{"已解", "已派", "仍缺"}}}
	_ = glfWriteFile(reg2, "闭集 `已解` / `已派` / `仍缺`\n面 A · 表头 1 处 · 数据行 3\n")
	rc, out = cap(r2, d2, reg2, false)
	casef("②c 真话负控（同一夹具 · 闭集扩到含它）⇒ 回绿", "0", strconv.Itoa(rc), rc == glfRCOK && strings.Contains(out, "值域内"), "")
	glfFaces = saved

	// ③ 交叉引用白名单：`同 §四` ⇒ 不算值域外
	r3, d3, _, reg3 := mk([]string{"| x | 同 §四 | a |", "| y | 仍缺 | b |"}, glfFixtureFace.header)
	rc, out = cap(r3, d3, reg3, false)
	casef("③ 白名单：`同 §四`（跨节指路）⇒ 不算值域外", "0", strconv.Itoa(rc), rc == glfRCOK && strings.Contains(out, "值域内"), "")

	// ④ 空转 ⇒ rc=2
	r4, d4, _, reg4 := mk([]string{}, glfFixtureFace.header)
	rc, out = cap(r4, d4, reg4, false)
	casef("④ 空转（表头在 · 0 行）⇒ rc=2 不给结论", "2", strconv.Itoa(rc), rc == glfRCNoVerdict && strings.Contains(out, "不给结论"), "")

	// ⑤ 表头逐字句找不到 ⇒ rc=2
	r5, d5, _, reg5 := mk([]string{"| x | 已解 | a |"}, "| 号 | 状态 |")
	rc, out = cap(r5, d5, reg5, false)
	casef("⑤ 表头逐字句找不到 ⇒ rc=2（不给结论）", "2", strconv.Itoa(rc), rc == glfRCNoVerdict && strings.Contains(out, "不给结论"), "")

	// ⑥ 账件不在盘 ⇒ rc=2
	r6, d6, led6, reg6 := mk([]string{"| x | 已解 | a |"}, glfFixtureFace.header)
	_ = os.Remove(led6)
	rc, out = cap(r6, d6, reg6, false)
	casef("⑥ 账件不在盘 ⇒ rc=2（不拿「读不到」当「没问题」）", "2", strconv.Itoa(rc), rc == glfRCNoVerdict && strings.Contains(out, "不给结论"), "")

	// ⑦ 登记件对拍
	r7, d7, _, reg7 := mk([]string{"| x | 已解 | a |", "| y | 仍缺 | b |"}, glfFixtureFace.header)
	_ = glfWriteFile(reg7, "面 A · 表头 1 处 · 数据行 2\n") // 少了闭集值 `仍缺`
	rc, out = cap(r7, d7, reg7, false)
	casef("⑦ 登记件缺一个闭集值 ⇒ rc=2", "2", strconv.Itoa(rc), rc == glfRCNoVerdict && strings.Contains(out, "闭集值"), "")
	_ = glfWriteFile(reg7, "闭集 `已解` / `仍缺`\n面 A · 表头 1 处 · 数据行 9\n") // 计数句不符
	rc, out = cap(r7, d7, reg7, false)
	casef("⑦b 登记件计数句与现读不符 ⇒ rc=2", "2", strconv.Itoa(rc), rc == glfRCNoVerdict && strings.Contains(out, "缺计数句"), "")

	// ⑧ 登记件位空（= `--no-register` 形态）⇒ 只判账 ⇒ rc=0
	rc, out = cap(r7, d7, "", false)
	casef("⑧ 登记件位空（`--no-register` 形态）⇒ 只判账 rc=0", "0", strconv.Itoa(rc), rc == glfRCOK && strings.Contains(out, "值域内"), "")

	if ok {
		fmt.Fprint(stdout, "自检结论: 全过\n")
		return glfRCOK
	}
	fmt.Fprint(stdout, "自检结论: **有断言失败**\n")
	return glfRCRed
}

// ── 入口 ─────────────────────────────────────────────────────────────────────
func glfUsageLine() string {
	return "usage: check-gap-ledger-face.py [-h] [--root ROOT] [--docs-root DOCS_ROOT] [--register REGISTER] [--no-register] [--json] [--list-rules] [--self-test] [--version]"
}

// glfArgErr —— argparse 风格用法错（✗ 正文与 py 不同 · 退码同 2 · 见件头未覆盖面①）。
func glfArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, glfUsageLine())
	fmt.Fprintf(stderr, "check-gap-ledger-face.py: error: %s\n", msg)
	return glfRCNoVerdict
}

// checkGapLedgerFaceGateMain —— 入口（= py `main()`）。
func checkGapLedgerFaceGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	docsRoot := ""
	register := ""
	noRegister := false
	jsonOut := false
	listRules := false
	selfTestFlag := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, glfUsageLine())
			return glfRCOK
		case a == "--version":
			fmt.Fprintln(stdout, glfVersion)
			return glfRCOK
		case a == "--self-test":
			selfTestFlag = true
		case a == "--list-rules":
			listRules = true
		case a == "--json":
			jsonOut = true
		case a == "--no-register":
			noRegister = true
		case a == "--root":
			i++
			if i >= len(argv) {
				return glfArgErr(stderr, "argument --root: expected one argument")
			}
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--docs-root":
			i++
			if i >= len(argv) {
				return glfArgErr(stderr, "argument --docs-root: expected one argument")
			}
			docsRoot = argv[i]
		case strings.HasPrefix(a, "--docs-root="):
			docsRoot = a[len("--docs-root="):]
		case a == "--register":
			i++
			if i >= len(argv) {
				return glfArgErr(stderr, "argument --register: expected one argument")
			}
			register = argv[i]
		case strings.HasPrefix(a, "--register="):
			register = a[len("--register="):]
		default:
			return glfArgErr(stderr, "unrecognized arguments: "+a)
		}
	}

	if selfTestFlag {
		return glfSelfTest(stdout)
	}
	if listRules {
		return glfListRules(stdout)
	}
	reg := ""
	if !noRegister {
		if register != "" {
			reg = register
		} else {
			rootAbs, err := filepath.Abs(root) // root 空 ⇒ cwd（见件头标准声明 (a)）
			if err != nil {
				rootAbs = root
			}
			base := docsRoot
			if base == "" {
				base = filepath.Join(filepath.Dir(rootAbs), "Zerg-内部文档")
			}
			reg = filepath.Join(base, glfRegisterRel)
		}
	}
	return glfRun(stdout, root, docsRoot, reg, jsonOut)
}
