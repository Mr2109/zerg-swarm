// check_error_kinds.go —— 门禁件 `scripts/gates/check-error-kinds.py` 的 **Go 真搬样板**（v2.5.14 · M2-0 第一枚）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据）的**第一枚样板**：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// ⚠ 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:2863`（门⑤ · tri 档））；
//	· **不**删 `scripts/gates/check-error-kinds.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· **已接线**：入口 `errorKindsGateMain` 由 `gateCheckGoEntry["check-error-kinds"]` 分派（`zerg gate check check-error-kinds`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func errorKindsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 全绿 / 1 门禁红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字）：目录根输入（默认根 · 指定根 · 成对负控 · 空闭集 · 取不到输入）·
// `--json` 拒收（rc=2 · stderr）· `--self-test` 成对负控（6 格 · 含 R3 代码位/注释位成对对拍）。
// 未覆盖面（✗ 不声称逐字对齐）：`-h/--help` 的 argparse 帮助正文 · 未知旗标的 argparse usage 正文 ·
// `exitcodes.go` 缺件时的异常路径（py 抛 traceback ⇒ rc=1；本件落「退码表空」⇒ rc=2）。
//
//	—— 这三面均**不在** M2-0 的样板对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// ── 判据正则（与 py 逐条对齐 · 名字前缀 `cek` 防包内同名）──────────────
var (
	// KIND_RE = ^[a-z][a-z0-9_]*$
	cekKindRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	// ROW_RE = ^\s*\{\s*"([^"]+)"\s*,\s*(-?\d+)\s*,\s*(true|false)\s*,   （re.M）
	cekRowRE = regexp.MustCompile(`(?m)^\s*\{\s*"([^"]+)"\s*,\s*(-?\d+)\s*,\s*(true|false)\s*,`)
	// USE_RE = setErr\(\s*"([^"]+)"
	cekUseRE = regexp.MustCompile(`setErr\(\s*"([^"]+)"`)
	// 退码表行 = ^\s*\{(\d+),\s*"
	cekDeclRE = regexp.MustCompile(`^\s*\{(\d+),\s*"`)
)

// cekRow —— `errorKinds` 闭集表的一行（与 py dict 三键同形）。
type cekRow struct {
	Kind      string
	Code      int
	Retryable bool
}

// errorKindsGateMain —— 入口（= py `main()`）。
func errorKindsGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后 rc=2（先于位置参数解析）。
	if rc := errorKindsGateRefuseJSON(stderr, argv); rc != nil {
		return *rc
	}
	// ② 位置参数等价解析（py argparse：root 可选 · 默认 "." · --self-test 布尔）。
	root := "."
	selfTest := false
	emitBaseline := false
	baselinePath := ""
	var positional []string
	for _, a := range argv {
		switch {
		case a == "--self-test":
			selfTest = true
		case a == "--emit-baseline":
			emitBaseline = true
		case strings.HasPrefix(a, "--baseline="):
			baselinePath = strings.TrimPrefix(a, "--baseline=")
		case strings.HasPrefix(a, "-") && a != "-":
			// 未知旗标：py argparse 也 rc=2（文案另论 · 见件头「未覆盖面」）。
			fmt.Fprintf(stderr, "usage: check-error-kinds.py [-h] [--self-test] [--baseline=PATH] [--emit-baseline] [root]\n")
			fmt.Fprintf(stderr, "check-error-kinds.py: error: unrecognized arguments: %s\n", a)
			return 2
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) > 0 {
		root = positional[len(positional)-1]
	}
	if emitBaseline {
		return errorKindsGateDetailEmit(stdout, stderr, root)
	}
	if selfTest {
		return errorKindsGateSelfTest(stdout)
	}
	// ③ 找真源树。
	r, ok := errorKindsGateFindRoot(root)
	if !ok {
		fmt.Fprintln(stdout, "⛔ 不给结论：找不到 core/cmd/zerg/errors.go（本门只认命令面真源所在的那棵树）")
		return 2
	}
	rc, lines := errorKindsGateCheck(r)
	// ★ B 类真口子修：读失败 ⇒ 响亮 BLOCKED rc=2 + 逐件点名（✗ 不再静默少判）。
	if len(cekReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(cekReadFails))
		for _, u := range cekReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return 2
	}
	// ── N2 子判据（§3.3 `N2` · 缺口 GAP-20261012-20）：把 `selfupdate_split` 的近义动词扫描
	//    接成**门面**并进退码（此前只由 `go test` 兜 ⇒ 门面零落点）。
	n2rc, n2lines := errorKindsGateN2Entry()
	rc = cekMergeRC(rc, n2rc)
	// ── detail 取值口径子判据（甲·登记式棘轮 · 缺口 GAP-20260928-25）：新开一面。
	//    真源 = 现盘 core/cmd/zerg/*.go 的 setErr 第二参 ＋ 登记基线件；
	//    基线取不到 / 现读空转 ⇒ rc=2（不给结论 · ✗ 不假绿）。
	drc, dlines := errorKindsGateDetailEntry(r, baselinePath)
	rc = cekMergeRC(rc, drc)
	fmt.Fprintln(stdout, "── 命令面 error.kind 闭集（§九 M7）──")
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
	fmt.Fprintln(stdout, "── 命令面命名纪律 N2（§3.3：禁用近义动词分指两事）──")
	for _, l := range n2lines {
		fmt.Fprintln(stdout, l)
	}
	fmt.Fprintln(stdout, "── 命令面 error.detail 取值口径（甲·登记式棘轮 · §九 M7 detail · GAP-20260928-25）──")
	for _, l := range dlines {
		fmt.Fprintln(stdout, l)
	}
	switch rc {
	case 0:
		fmt.Fprintln(stdout, "⇒ 全绿（rc=0）")
	case 1:
		fmt.Fprintln(stdout, "⇒ 门禁红（rc=1）：kind 闭集 / N2 命名纪律 / detail 取值口径 被破坏")
	default:
		fmt.Fprintln(stdout, "⇒ 不给结论（rc=2）")
	}
	return rc
}

// errorKindsGateRefuseJSON —— py `refuse_json_without_machine_face()`。
// 给了 `--json` ⇒ 逐字说明后返 2；没给 ⇒ 返 nil（原路照走）。
func errorKindsGateRefuseJSON(stderr io.Writer, argv []string) *int {
	var hit []string
	for _, a := range argv {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			hit = append(hit, a)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	parts := make([]string, len(hit))
	for i, h := range hit {
		parts[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(parts, " · "))
	// JSON_FIELDS 为空 ⇒ 字段表固定为「无（本门只出人读面）」（唯一真源：py 的 JSON_FIELDS）。
	fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	rc := 2
	return &rc
}

// ── N2 子判据（§3.3 `N2` · 缺口 `GAP-20261012-20`）────────────────────────────
//
// 病（缺口逐字）：门⑤ 实判的是 `error.kind` 蛇形命名，与 N2 近义动词禁令**零交集** ——
// 「步名命名纪律 N2 四禁词」只由 `go test` 兜，**门面无落点**。
//
// 真源（**口径不从本件自定** · 一律现读）：`core/internal/contract/selfupdate.json` 的
// `near_synonym_forbidden`（禁词表）/ `near_synonym_rule`（§3.3 `N2` 逐字）/ `semantics_split`
// （两条命令各自的「该说什么」）—— 由 `contract.SelfUpdate()` 解出（唯一真源 · ✗ 不硬编禁词）。
//
// 判定口（**唯一**）：`updateWordSplitViolations`（`selfupdate_split.go`）—— ✗ 不另抄一份判据。
// 被判对象：**真命令树**（`updateSplitShapes()` · 经 `errorKindsGateN2Shapes` 可注入夹具做负控）。
//
// 退码（与本门三码同规）：0 全绿 · 1 命中近义动词（逐条点名 命令·段·词）· 2 真源取不到/空（不给结论）。

// errorKindsGateN2Shapes —— 形状取法（默认 = 真命令树；定向单测可覆盖注入夹具）。
var errorKindsGateN2Shapes = updateSplitShapes

// cekMergeRC —— 两枚子判据的退码合并（本仓严重度序：2（不给结论）> 1（红）> 0）。
func cekMergeRC(a, b int) int {
	if a == 2 || b == 2 {
		return 2
	}
	if a == 1 || b == 1 {
		return 1
	}
	return 0
}

// errorKindsGateN2Check —— N2 判定口：给定真源 + 形状 ⇒ (rc, 报告行)。
func errorKindsGateN2Check(spec *contract.SelfUpdateSpec, cmds []updateCmdShape) (int, []string) {
	if spec == nil || len(spec.NearSynonymForbidden) == 0 {
		return 2, []string{"⛔ 不给结论：N2 真源取不到（`near_synonym_forbidden` 空 ⇒ 空转 = 假覆盖）"}
	}
	v := updateWordSplitViolations(spec, cmds)
	lines := []string{fmt.Sprintf("禁词 %d 枚 · 命令 %d 条 · 命中 %d 条", len(spec.NearSynonymForbidden), len(cmds), len(v))}
	for _, h := range v {
		lines = append(lines, "✗ N2 "+h)
	}
	if len(v) > 0 {
		return 1, lines
	}
	return 0, lines
}

// errorKindsGateN2Entry —— 门面入口：真源现读（`contract.SelfUpdate()`）+ 真命令树形状。
func errorKindsGateN2Entry() (int, []string) {
	spec, err := contract.SelfUpdate()
	if err != nil {
		return 2, []string{"⛔ 不给结论：N2 真源解不动（" + err.Error() + "）"}
	}
	return errorKindsGateN2Check(spec, errorKindsGateN2Shapes())
}

// errorKindsGateFindRoot —— py `find_root()`（自 start 向上 ≤8 层找 core/cmd/zerg/errors.go）。
// ── detail 取值口径子判据（甲·登记式棘轮 · 缺口 `GAP-20260928-25`）────────────────
//
// 病（缺口逐字）：`error.kind` 有闭集判据（R1/R2/R3），`error.detail`（`setErr` 第二参）却
// **零取值口径** —— 想造第二个一模一样的错，得自己发明一枚 detail 字面。
//
// 人拍口径（2026-10-15 · 甲档 = **登记式棘轮**）：**不规定「合法值表」**；把**现盘全部现有取值**
// 登记入基线件 `core/internal/contract/error-detail-baseline.json`，此后 **新增 / 改动即红（须登记）**。
// 硬线：✗ 抬基线来消红 · ✗ 放宽断言 · ✗ 把「登记」改成「全放行」。
//
// 判定口（唯一）：`errorKindsGateDetailCheck`（给定 基线 ＋ 现读落点 ⇒ (rc, 报告行)）。
// 被判对象：现盘 `core/cmd/zerg/*.go`（✗ 不含 `_test.go`）里 `setErr(<kind>, <detail>, <msg>)` 的
// **第二参**；字面量取**内层源码文本**（未反转义 · 基线面与现读面同口径），非字面量取**空白规整化后的
// 表达式文本**（改名 / 改形态都判成「改动」）。
//
// 退码（与本门三码同规）：0 全绿 · 1 有未登记值 / 有登记值现读消失（逐条点名 件:行 ＋ 值）·
// 2 基线件取不到 / 解析失败 / 现读空转（不给结论 ⇒ ✗ 不假绿）。
//
// 生成面（正门 · 打印模式 `--emit-baseline`）：产出 JSON 交**正门** `dev edit --file` 落进基线件 ——
// 本门 main 路径一个字节都不写（✗ 不自己落盘）。

// cekDetailBaselineRel —— 基线件的仓内唯一相对路径（生成面与校核面同用这一处）。
const cekDetailBaselineRel = "core/internal/contract/error-detail-baseline.json"

// cekDetailSite —— 一处 `setErr` 第二参落点。
type cekDetailSite struct {
	Rel     string // 仓内相对路径（core/cmd/zerg/x.go）
	Line    int    // 行号（1 起）
	Literal bool   // 字面量 / 非字面量
	Value   string // 字面量：内层源码文本；非字面量：空白规整化后的表达式
}

// cekDetailEntry —— 基线里的一条登记（值 ＋ 该值的全部落点 `件:行` · 逐条可回溯）。
type cekDetailEntry struct {
	Value string   `json:"value"`
	Sites []string `json:"sites"`
}

// cekDetailScanFace —— 基线件头部自述的取数口径（✗ 不带时间戳 · 排序稳定）。
type cekDetailScanFace struct {
	Glob        string `json:"glob"`
	Arg         string `json:"arg"`
	LiteralRead string `json:"literal_read"`
	NonLitRead  string `json:"non_literal_read"`
}

// cekDetailCounts —— 基线自带计数（＝逐条列出的长度 · 只作自证 · ✗ 不另抄第二真源）。
type cekDetailCounts struct {
	Literals        int `json:"literals"`
	LiteralSites    int `json:"literal_sites"`
	NonLiterals     int `json:"non_literals"`
	NonLiteralSites int `json:"non_literal_sites"`
	SitesTotal      int `json:"sites_total"`
}

// cekDetailBaseline —— 基线件形状（`literals` / `non_literals` 各按值字典序排列）。
type cekDetailBaseline struct {
	Schema      string            `json:"schema"`
	Note        string            `json:"note"`
	Scan        cekDetailScanFace `json:"scan"`
	Counts      cekDetailCounts   `json:"counts"`
	Literals    []cekDetailEntry  `json:"literals"`
	NonLiterals []cekDetailEntry  `json:"non_literals"`
}

// cekDetailWordByte —— 标识符字节判定（`setErr` 前一位是名字字符 ⇒ 不是本调用）。
func cekDetailWordByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// cekSetErrCodeIdx —— 单件里 `setErr` 的**代码位**下标：注释 / 字符串字面量里的同串是散文不是用法
// （与 R3 面同一条口径）；`func … setErr(…)` 的声明行也不算调用点。
// cekSetErrDeclLine —— 该行是否为 **`setErr` 自身的声明行**（`func (inv *invocation) setErr(…` /
// `func setErr(…`）。★ 关键是「**被声明的名字恰是 `setErr`**」：✗ 别用宽判据（「行首是 func」就跳）——
// 那会把 `func f() { setErr("conflict", …) }` 这种**单行函数里的真调用**一起跳掉（实测：落点凭空少一条，
// 且两套探针同错 ⇒ 对拍也看不出来）。
func cekSetErrDeclLine(line string) bool {
	s := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(s, "func") {
		return false
	}
	rest := strings.TrimLeft(s[len("func"):], " \t")
	if strings.HasPrefix(rest, "(") {
		i, depth := 0, 0
		for i < len(rest) {
			if rest[i] == '(' {
				depth++
			}
			if rest[i] == ')' {
				depth--
				if depth == 0 {
					i++
					break
				}
			}
			i++
		}
		rest = strings.TrimLeft(rest[i:], " \t")
	}
	if !strings.HasPrefix(rest, "setErr") {
		return false
	}
	rest = rest[len("setErr"):]
	return rest == "" || strings.HasPrefix(rest, "(")
}

func cekSetErrCodeIdx(text string) []int {
	var idx []int
	i, n := 0, len(text)
	for i < n {
		c := text[i]
		if c == '/' && i+1 < n && text[i+1] == '/' {
			j := strings.IndexByte(text[i:], '\n')
			if j < 0 {
				return idx
			}
			i += j
			continue
		}
		if c == '/' && i+1 < n && text[i+1] == '*' {
			j := strings.Index(text[i+2:], "*/")
			if j < 0 {
				return idx
			}
			i += 2 + j + 2
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			q := c
			i++
			for i < n {
				if q != '`' && text[i] == '\\' {
					i += 2
					continue
				}
				if text[i] == q {
					i++
					break
				}
				i++
			}
			continue
		}
		if strings.HasPrefix(text[i:], "setErr") {
			prev := byte(' ')
			if i > 0 {
				prev = text[i-1]
			}
			ls := strings.LastIndexByte(text[:i], '\n') + 1
			le := len(text)
			if j := strings.IndexByte(text[i:], '\n'); j >= 0 {
				le = i + j
			}
			decl := cekSetErrDeclLine(text[ls:le])
			if !cekDetailWordByte(prev) && !decl {
				idx = append(idx, i)
			}
			i += 6
			continue
		}
		i++
	}
	return idx
}

// cekCallArgs —— 从实参表起始括号起按嵌套深度切实参（字符串 / 注释里的逗号不计 · 未闭合 ⇒ nil）。
func cekCallArgs(text string, open int) []string {
	var args []string
	var cur strings.Builder
	depth := 1
	i, n := open+1, len(text)
	for i < n {
		c := text[i]
		switch {
		case c == '(' || c == '[' || c == '{':
			depth++
			cur.WriteByte(c)
			i++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth == 0 {
				args = append(args, cur.String())
				return args
			}
			cur.WriteByte(c)
			i++
		case c == '"' || c == '\'' || c == '`':
			q := c
			cur.WriteByte(c)
			i++
			for i < n {
				if q != '`' && text[i] == '\\' && i+1 < n {
					cur.WriteString(text[i : i+2])
					i += 2
					continue
				}
				cur.WriteByte(text[i])
				if text[i] == q {
					i++
					break
				}
				i++
			}
		case c == '/' && i+1 < n && text[i+1] == '/':
			j := strings.IndexByte(text[i:], '\n')
			if j < 0 {
				return nil
			}
			cur.WriteString(text[i : i+j])
			i += j
		case c == '/' && i+1 < n && text[i+1] == '*':
			j := strings.Index(text[i+2:], "*/")
			if j < 0 {
				return nil
			}
			cur.WriteString(text[i : i+2+j+2])
			i += 2 + j + 2
		case c == ',' && depth == 1:
			args = append(args, cur.String())
			cur.Reset()
			i++
		default:
			cur.WriteByte(c)
			i++
		}
	}
	return nil
}

// cekDetailLitValue —— 第二参是否**恰好一枚**字符串字面量：是 ⇒ 回**内层源码文本**（未反转义）。
func cekDetailLitValue(s string) (string, bool) {
	n := len(s)
	if n >= 2 && s[0] == '"' && s[n-1] == '"' {
		i := 1
		for i < n {
			if s[i] == '\\' {
				i += 2
				continue
			}
			if s[i] == '"' {
				if i == n-1 {
					return s[1 : n-1], true
				}
				break
			}
			i++
		}
		return "", false
	}
	if n >= 2 && s[0] == '`' && s[n-1] == '`' && !strings.Contains(s[1:n-1], "`") {
		return s[1 : n-1], true
	}
	return "", false
}

// cekCollapseSpace —— 非字面量的「形态」= 空白规整化后的表达式文本（换行 / 缩进不算改动）。
func cekCollapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// cekDetailSitesInFile —— 单件扫描 ⇒ 该件全部 `setErr` 第二参落点（行号按起点现算）。
func cekDetailSitesInFile(rel, text string) []cekDetailSite {
	var out []cekDetailSite
	for _, k := range cekSetErrCodeIdx(text) {
		j := k + len("setErr")
		for j < len(text) && (text[j] == ' ' || text[j] == '\t') {
			j++
		}
		if j >= len(text) || text[j] != '(' {
			continue
		}
		args := cekCallArgs(text, j)
		if len(args) < 2 {
			continue
		}
		flat := cekCollapseSpace(args[1])
		if flat == "" {
			continue
		}
		s := cekDetailSite{Rel: rel, Line: 1 + strings.Count(text[:k], "\n")}
		if v, ok := cekDetailLitValue(flat); ok {
			s.Literal = true
			s.Value = v
		} else {
			s.Value = flat
		}
		out = append(out, s)
	}
	return out
}

// cekScanDetailSites —— **唯一取数处**（现盘 `core/cmd/zerg/*.go` · 非测试件 · 件名排序）。
// 返回 (落点, 读失败件清单)：读失败 ⇒ 真源取不到 ⇒ 由入口判 rc=2（✗ 不假绿）。
func cekScanDetailSites(root string) ([]cekDetailSite, []string) {
	dir := filepath.Join(root, "core", "cmd", "zerg")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []string{"core/cmd/zerg（读不了目录：" + err.Error() + "）"}
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var sites []cekDetailSite
	var fails []string
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			fails = append(fails, "core/cmd/zerg/"+n+"（"+err.Error()+"）")
			continue
		}
		sites = append(sites, cekDetailSitesInFile("core/cmd/zerg/"+n, string(b))...)
	}
	return sites, fails
}

// cekDetailGroup —— 现读落点 → (值 ⇒ 全部 `件:行`, 值的有序集)。
func cekDetailGroup(sites []cekDetailSite, literal bool) (map[string][]string, []string) {
	g := map[string][]string{}
	for _, s := range sites {
		if s.Literal != literal {
			continue
		}
		g[s.Value] = append(g[s.Value], fmt.Sprintf("%s:%d", s.Rel, s.Line))
	}
	keys := make([]string, 0, len(g))
	for k := range g {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return g, keys
}

// cekDetailCount —— 登记列表的落点总数。
func cekDetailCount(entries []cekDetailEntry) int {
	t := 0
	for _, e := range entries {
		t += len(e.Sites)
	}
	return t
}

// cekDetailCountMap —— 现读分组的落点总数。
func cekDetailCountMap(g map[string][]string) int {
	t := 0
	for _, v := range g {
		t += len(v)
	}
	return t
}

// cekDetailEntryMap —— 基线列表 → 映射（值 ⇒ 落点）。
func cekDetailEntryMap(list []cekDetailEntry) map[string][]string {
	m := map[string][]string{}
	for _, e := range list {
		m[e.Value] = e.Sites
	}
	return m
}

// cekDetailSortedKeys —— 映射的键字典序（报告按稳定序出）。
func cekDetailSortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// cekDetailSitesBrief —— 落点清单截断（✗ 不把几十处落点全刷进一行 · 截断处**明报**还剩几处）。
func cekDetailSitesBrief(sites []string, max int) string {
	if max < 1 {
		max = 1
	}
	if len(sites) <= max {
		return strings.Join(sites, " · ")
	}
	return strings.Join(sites[:max], " · ") + fmt.Sprintf(" · …另 %d 处", len(sites)-max)
}

// errorKindsGateDetailCheck —— detail 取值口径判定口：给定 (基线, 现读落点) ⇒ (rc, 报告行)。
//
// 三档：0 全绿 · 1 有未登记值 / 有登记值现读消失 · 2 基线取不到 / 空转（不给结论 ⇒ ✗ 不假绿）。
func errorKindsGateDetailCheck(bl *cekDetailBaseline, sites []cekDetailSite) (int, []string) {
	if bl == nil {
		return 2, []string{"⛔ 不给结论：detail 取值口径基线取不到（" + cekDetailBaselineRel + " 读不到 / 解析失败）⇒ 空转 = 假覆盖，不许当绿"}
	}
	nReg, nRegNon := len(bl.Literals), len(bl.NonLiterals)
	if nReg+nRegNon == 0 {
		return 2, []string{"⛔ 不给结论：detail 取值基线一条登记都没有（空转 = 假覆盖，不许当绿）"}
	}
	nSites := len(sites)
	if nSites == 0 {
		return 2, []string{"⛔ 不给结论：现盘 `setErr` 第二参一个落点都没扫到（真源取不到 / 空转 = 假覆盖，不许当绿）"}
	}
	reg := cekDetailEntryMap(bl.Literals)
	regNon := cekDetailEntryMap(bl.NonLiterals)
	cur, curKeys := cekDetailGroup(sites, true)
	curNon, curNonKeys := cekDetailGroup(sites, false)

	var reds []string
	for _, v := range curKeys {
		if _, ok := reg[v]; !ok {
			reds = append(reds, fmt.Sprintf("✗ 未登记的 detail 字面量 '%s'（%s）—— 新增取值须登记进 %s",
				v, cekDetailSitesBrief(cur[v], 3), cekDetailBaselineRel))
		}
	}
	for _, v := range cekDetailSortedKeys(reg) {
		if _, ok := cur[v]; !ok {
			reds = append(reds, fmt.Sprintf("✗ 已登记的 detail 字面量 '%s' 现读消失（改名 / 改形态 = 改动 ⇒ 须登记）：原落点 %s",
				v, cekDetailSitesBrief(reg[v], 3)))
		}
	}
	for _, v := range curNonKeys {
		if _, ok := regNon[v]; !ok {
			reds = append(reds, fmt.Sprintf("✗ 未登记的 detail 形态 '%s'（%s）—— 新增形态须登记进 %s",
				v, cekDetailSitesBrief(curNon[v], 3), cekDetailBaselineRel))
		}
	}
	for _, v := range cekDetailSortedKeys(regNon) {
		if _, ok := curNon[v]; !ok {
			reds = append(reds, fmt.Sprintf("✗ 已登记的 detail 形态 '%s' 现读消失（改形态 ⇒ 须登记）：原落点 %s",
				v, cekDetailSitesBrief(regNon[v], 3)))
		}
	}

	lines := []string{fmt.Sprintf("登记 %d 字面量 / %d 落点 ＋ %d 非字面量形态 / %d 落点 · 现读 %d 字面量 / %d 落点 ＋ %d 形态 / %d 落点 · 命中 %d 条",
		nReg, cekDetailCount(bl.Literals), nRegNon, cekDetailCount(bl.NonLiterals),
		len(curKeys), cekDetailCountMap(cur), len(curNonKeys), cekDetailCountMap(curNon), len(reds))}
	lines = append(lines, reds...)
	if len(reds) > 0 {
		return 1, lines
	}
	return 0, lines
}

// errorKindsGateDetailScan —— 现读落点取数口（默认 `cekScanDetailSites`；定向单测 / 负控可覆盖 · ✗ 不改语义）。
var errorKindsGateDetailScan = cekScanDetailSites

// errorKindsGateDetailBaselineFile —— 基线件路径覆盖口（默认空 ⇒ 用 <root>/<相对路径>）。
var errorKindsGateDetailBaselineFile = ""

// cekDetailBaselinePathOf —— 基线件路径：`--baseline=` 旋钮 ＞ 覆盖口 ＞ 默认相对路径。
func cekDetailBaselinePathOf(root, override string) string {
	if override != "" {
		return override
	}
	if errorKindsGateDetailBaselineFile != "" {
		return errorKindsGateDetailBaselineFile
	}
	return filepath.Join(root, cekDetailBaselineRel)
}

// cekDetailLoadBaseline —— 读 ＋ 解基线件（读不到 / 解析失败 ⇒ error ⇒ 入口判 rc=2）。
func cekDetailLoadBaseline(root, override string) (*cekDetailBaseline, error) {
	b, err := os.ReadFile(cekDetailBaselinePathOf(root, override))
	if err != nil {
		return nil, err
	}
	var bl cekDetailBaseline
	if err := json.Unmarshal(b, &bl); err != nil {
		return nil, err
	}
	return &bl, nil
}

// errorKindsGateDetailEntry —— 门面入口：现读落点 ＋ 基线 ⇒ (rc, 报告行)。
func errorKindsGateDetailEntry(root, baselineOverride string) (int, []string) {
	sites, fails := errorKindsGateDetailScan(root)
	nFail := len(fails)
	if nFail > 0 {
		lines := []string{fmt.Sprintf("⛔ 不给结论：现盘源码有 %d 件读不到（真源取不到 ⇒ 空转 = 假覆盖，不许当绿）", nFail)}
		for _, f := range fails {
			lines = append(lines, "  [读失败] "+f)
		}
		return 2, lines
	}
	bl, err := cekDetailLoadBaseline(root, baselineOverride)
	if err != nil {
		return 2, []string{"⛔ 不给结论：detail 取值口径基线取不到 / 解析失败（" + err.Error() + "）⇒ 空转 = 假覆盖，不许当绿"}
	}
	return errorKindsGateDetailCheck(bl, sites)
}

// cekDetailBuildBaseline —— 生成面：现读落点 ⇒ 基线件内容（值字典序 · 落点字典序 · ✗ 不带时间戳）。
func cekDetailBuildBaseline(sites []cekDetailSite) *cekDetailBaseline {
	bl := &cekDetailBaseline{
		Schema: "zerg-error-detail-baseline/1",
		Note: "error.detail（setErr 第二参）取值口径的**登记式棘轮**基线（甲档 · 2026-10-15 拍定 · 缺口 GAP-20260928-25）。" +
			"口径：✗ 不规定合法值表 —— 现盘全部现有取值登记在此；**新增 / 改动即红**（须登记）。" +
			"生成：`zerg gate check check-error-kinds --emit-baseline`（打印模式 · ✗ 不写盘），产出交正门 `dev edit --file` 落盘。",
		Scan: cekDetailScanFace{
			Glob:        "core/cmd/zerg/*.go（✗ 不含 _test.go）",
			Arg:         "setErr(<kind>, <detail>, <msg>) 的第二参",
			LiteralRead: "字面量取内层源码文本（未反转义）",
			NonLitRead:  "非字面量取空白规整化后的表达式文本",
		},
	}
	// 空组也写成 []（✗ 不写 null）—— 基线件形状稳定，消费侧不用分流。
	bl.Literals = []cekDetailEntry{}
	bl.NonLiterals = []cekDetailEntry{}
	for _, wantLiteral := range []bool{true, false} {
		g, keys := cekDetailGroup(sites, wantLiteral)
		for _, k := range keys {
			one := append([]string{}, g[k]...)
			sort.Strings(one)
			e := cekDetailEntry{Value: k, Sites: one}
			if wantLiteral {
				bl.Literals = append(bl.Literals, e)
			} else {
				bl.NonLiterals = append(bl.NonLiterals, e)
			}
		}
	}
	lit, non := cekDetailCount(bl.Literals), cekDetailCount(bl.NonLiterals)
	bl.Counts = cekDetailCounts{
		Literals:        len(bl.Literals),
		LiteralSites:    lit,
		NonLiterals:     len(bl.NonLiterals),
		NonLiteralSites: non,
		SitesTotal:      lit + non,
	}
	return bl
}

// errorKindsGateDetailEmit —— 生成面（打印模式 · ✗ 不写盘 · 0 出件 / 2 不给结论）。
func errorKindsGateDetailEmit(stdout, stderr io.Writer, root string) int {
	sites, fails := errorKindsGateDetailScan(root)
	nFail, nSites := len(fails), len(sites)
	if nFail > 0 || nSites == 0 {
		fmt.Fprintf(stderr, "⛔ 不给结论：生成面拒绝产空基线（读失败 %d 件 · 扫到 %d 个落点 ⇒ 空转 = 假覆盖）\n", nFail, nSites)
		for _, f := range fails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", f)
		}
		return 2
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cekDetailBuildBaseline(sites)); err != nil {
		fmt.Fprintf(stderr, "⛔ 不给结论：基线序列化失败（%v）\n", err)
		return 2
	}
	fmt.Fprint(stdout, buf.String())
	return 0
}

func errorKindsGateFindRoot(start string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for i := 0; i < 8; i++ {
		if fi, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg", "errors.go")); err == nil && !fi.IsDir() {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", false
}

// errorKindsGateParseClosedSet —— py `parse_closed_set()`（只读真源，不另抄一份）。
func errorKindsGateParseClosedSet(text string) []cekRow {
	start := strings.Index(text, "var errorKinds = []errorKindRow{")
	if start < 0 {
		return nil
	}
	rel := strings.Index(text[start:], "\n}")
	if rel < 0 {
		return nil
	}
	seg := text[start : start+rel]
	var rows []cekRow
	for _, m := range cekRowRE.FindAllStringSubmatch(seg, -1) {
		code, _ := strconv.Atoi(m[2])
		rows = append(rows, cekRow{Kind: m[1], Code: code, Retryable: m[3] == "true"})
	}
	return rows
}

// cekStripGoComments —— py `strip_go_comments()`：剥掉 Go 的 `//` 行注释与 `/* … */` 块注释
// （**保留字符串 / 字符 / 原生字符串字面量内的字符**）。
//
// 目的 = 让 R3 只认**代码位**的 kind 用法：散文 / 注释里提一句 `setErr("…")` 是文档，不是
// 「用了个闭集外的 kind」（本仓大量门脚本的自指说明正是这种形态 ⇒ 不剥即误报）。
// ★ 不剥字符串字面量：`setErr("<kind>"…)` 的 kind 本身就写在字符串里，剥了就等于空转。
func cekStripGoComments(text string) string {
	var out strings.Builder
	i, n := 0, len(text)
	var quote byte // 0 = 不在字面量内；'"' / '\'' / '`'
	for i < n {
		c := text[i]
		if quote != 0 {
			out.WriteByte(c)
			if quote == '`' {
				if c == '`' {
					quote = 0
				}
			} else if c == '\\' {
				if i+1 < n {
					out.WriteByte(text[i+1])
					i += 2
					continue
				}
			} else if c == quote {
				quote = 0
			}
			i++
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			quote = c
			out.WriteByte(c)
			i++
			continue
		}
		if c == '/' && i+1 < n && text[i+1] == '/' {
			j := strings.IndexByte(text[i:], '\n')
			if j < 0 {
				break
			}
			out.WriteByte('\n')
			i += j + 1
			continue
		}
		if c == '/' && i+1 < n && text[i+1] == '*' {
			j := strings.Index(text[i+2:], "*/")
			if j < 0 {
				i = n
			} else {
				i = i + 2 + j + 2
			}
			out.WriteByte(' ')
			continue
		}
		out.WriteByte(c)
		i++
	}
	return out.String()
}

// errorKindsGateCheck —— py `check()`：返回 (rc, 报告行列表)。rc 0=绿 1=红 2=不给结论。
// cekReadFails —— B 类读失败登记面（R3 用法面扫描里读不了的目录/件 ⇒ 逐件登记 相对路径 + err 正文）。
// ★ B 类真口子修（与 py 有意分叉 · 同族先例 check_doc_cmds.go 的 dcmdReadFails）：
//
//	读失败 ⇒ 登记 + 入口判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（✗ 不再静默少判）。
//	自清点：每次 errorKindsGateCheck 起手清空 ⇒ 不许跨调用残留。
var cekReadFails []string

func errorKindsGateCheck(root string) (int, []string) {
	cekReadFails = nil // ★ 自清：包级登记面不许跨调用残留
	var errs []string

	rows, err := errorKindsGateReadClosedSet(filepath.Join(root, "core", "cmd", "zerg", "errors.go"))
	if err != nil {
		// py 在此处会抛异常 ⇒ rc=1；本件按「闭集表读不到 = 空转」落 rc=2（见件头「未覆盖面」）。
		return 2, []string{"⛔ 不给结论：闭集表为空（`errorKinds` 一个 kind 都没解析到 ⇒ 空转 = 假覆盖）"}
	}
	if len(rows) == 0 {
		return 2, []string{"⛔ 不给结论：闭集表为空（`errorKinds` 一个 kind 都没解析到 ⇒ 空转 = 假覆盖）"}
	}

	seen := map[string]cekRow{}
	for _, r := range rows {
		if !cekKindRE.MatchString(r.Kind) {
			errs = append(errs, fmt.Sprintf("R1 kind 命名不合规（必须 lower_snake_case）：'%s'", r.Kind))
		}
		if _, dup := seen[r.Kind]; dup {
			errs = append(errs, fmt.Sprintf("R2 kind 重复：%s", r.Kind))
		}
		seen[r.Kind] = r
		if r.Code < 0 {
			errs = append(errs, fmt.Sprintf("R2 退码为负：%s → %d", r.Kind, r.Code))
		}
	}

	// R2 的另一半：每个 kind 的码必须在退码表里登记过。
	declared, err := errorKindsGateReadDeclaredCodes(filepath.Join(root, "core", "cmd", "zerg", "exitcodes.go"))
	if err != nil || len(declared) == 0 {
		return 2, []string{"⛔ 不给结论：退码表一个码都没解析到（空转 = 假覆盖）"}
	}
	for _, r := range rows {
		if !declared[r.Code] {
			errs = append(errs, fmt.Sprintf("R2 kind '%s' 用的码 %d **不在退码表里**（先登记号再启用：占号纪律 §九 M7 `E8`）", r.Kind, r.Code))
		}
	}

	// R3 用法面。
	dir := filepath.Join(root, "core", "cmd", "zerg")
	entries, derr := os.ReadDir(dir)
	if derr != nil {
		// ★ B 类真口子修（Go 侧曾丢错 · ✗ 不再静默）：**在盘却读不了**才是真读失败 ⇒ 登记 + 判红 + rc=2。
		if fi, serr := os.Stat(dir); serr == nil && fi.IsDir() {
			cekReadFails = append(cekReadFails, "core/cmd/zerg（读不了目录）")
		}
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			names = append(names, n)
		}
	}
	sort.Strings(names) // py: for name in sorted(os.listdir(dirpath))
	used := map[string]bool{}
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			// ★ B 类真口子修（原 py 静默跳过 · ✗ 不再静默）：登记 + 判红 + rc=2。
			cekReadFails = append(cekReadFails, "core/cmd/zerg/"+n+"（"+err.Error()+"）")
			continue
		}
		// 只认**代码位**：先剥注释（注释里提一句 setErr("…") 是散文不是用法；对应 py 的 strip_go_comments）。
		txt := cekStripGoComments(string(b))
		for _, m := range cekUseRE.FindAllStringSubmatch(txt, -1) {
			used[m[1]] = true
		}
	}
	delete(used, "kind")
	var usedSorted []string
	for u := range used {
		usedSorted = append(usedSorted, u)
	}
	sort.Strings(usedSorted)
	for _, u := range usedSorted {
		if _, ok := seen[u]; !ok {
			errs = append(errs, fmt.Sprintf("R3 用到闭集外的 kind：'%s'（先登记进 `errorKinds` 再用）", u))
		}
	}

	var lines []string
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("  kind=%-22s code=%-4d retryable=%-5s", r.Kind, r.Code, cekPyBool(r.Retryable)))
	}
	lines = append(lines, fmt.Sprintf("闭集 %d 个 kind · 用法面命中 %d 个字面量", len(rows), len(used)))
	if len(errs) > 0 {
		for _, e := range errs {
			lines = append(lines, "✗ "+e)
		}
		return 1, lines
	}
	return 0, lines
}

// errorKindsGateReadClosedSet —— 读 errors.go 并解析闭集（py 内联的 open+parse）。
func errorKindsGateReadClosedSet(path string) ([]cekRow, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return errorKindsGateParseClosedSet(string(b)), nil
}

// errorKindsGateReadDeclaredCodes —— py 内联：逐行 `^\s*\{(\d+),\s*"` 取已登记退码集。
func errorKindsGateReadDeclaredCodes(path string) (map[int]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[int]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if m := cekDeclRE.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			out[n] = true
		}
	}
	return out, nil
}

// cekPyBool —— py `str(bool)` ⇒ "True"/"False"（✗ 不是 Go 的 "true"/"false"）。
func cekPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// cekPyReprStr —— 仅覆盖本件用到的字符串形态（含 ' 不含 " ⇒ py repr 用双引号）。
func cekPyReprStr(s string) string {
	esc := strings.ReplaceAll(s, "\\", "\\\\")
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		return "\"" + esc + "\""
	}
	return "'" + esc + "'"
}

// cekPyReprList —— py 列表 repr：`[repr(a), repr(b)]`（供 --self-test 逐字回放）。
func cekPyReprList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = cekPyReprStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// errorKindsGateSelfTest —— py `self_test()`：成对负控（好件必绿 · 坏件必红 · 空转不给结论 · 未登记码必红）。
func errorKindsGateSelfTest(stdout io.Writer) int {
	ok := true
	mk := func(body string) string {
		t, err := os.MkdirTemp("", "cek")
		if err != nil {
			return ""
		}
		p := filepath.Join(t, "core", "cmd", "zerg")
		_ = os.MkdirAll(p, 0o755)
		_ = os.WriteFile(filepath.Join(p, "errors.go"), []byte(body), 0o644)
		_ = os.WriteFile(filepath.Join(p, "exitcodes.go"), []byte("var exitcodeTable = []exitcodeRow{\n"+
			"\t{0, \"ok\", \"成功\", \"—\"},\n\t{1, \"failed\", \"一般失败\", \"看 kind\"},\n"+
			"\t{2, \"usage\", \"用法错\", \"改用法\"},\n\t{14, \"conflict\", \"冲突\", \"排队\"},\n}\n"), 0o644)
		return t
	}
	filter := func(lines []string, sub string) []string {
		var out []string
		for _, l := range lines {
			if strings.Contains(l, sub) {
				out = append(out, l)
			}
		}
		return out
	}

	good := "var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\n"
	rc, _ := errorKindsGateCheck(mk(good))
	fmt.Fprintf(stdout, "  自检① 好件（lower_snake）⇒ rc=%d（要 0）\n", rc)
	ok = ok && rc == 0

	bad := "var errorKinds = []errorKindRow{\n\t{\"CONFIRM_REQUIRED\", 2, false, \"r\", \"m\", \"s\"},\n}\n"
	rc, lines := errorKindsGateCheck(mk(bad))
	fmt.Fprintf(stdout, "  自检② 大写 kind ⇒ rc=%d（要 1）· 命中: %s\n", rc, cekPyReprList(filter(lines, "R1")))
	ok = ok && rc == 1

	empty := "var errorKinds = []errorKindRow{\n}\n"
	rc, _ = errorKindsGateCheck(mk(empty))
	fmt.Fprintf(stdout, "  自检③ 空闭集 ⇒ rc=%d（要 2 —— 空转 = 假覆盖，不给结论）\n", rc)
	ok = ok && rc == 2

	ghost := "var errorKinds = []errorKindRow{\n\t{\"ghost_kind\", 99, false, \"r\", \"m\", \"s\"},\n}\n"
	rc, lines = errorKindsGateCheck(mk(ghost))
	fmt.Fprintf(stdout, "  自检④ 未登记的码 99 ⇒ rc=%d（要 1）· %s\n", rc, cekPyReprList(filter(lines, "R2")))
	ok = ok && rc == 1

	// R3 成对负控（真写错 kind 必须抓到）：代码位写了个闭集外的 kind ⇒ 红。
	r3bad := "var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\nfunc f() { setErr(\"bogus_kind\", \"x\", \"y\") }\n"
	rc, lines = errorKindsGateCheck(mk(r3bad))
	fmt.Fprintf(stdout, "  自检⑤ R3 代码位用了闭集外的 kind ⇒ rc=%d（要 1）· %s\n", rc, cekPyReprList(filter(lines, "R3")))
	ok = ok && rc == 1

	// R3 反误报对拍：同一句只写在**注释**里 ⇒ 不红（注释已剥）。
	r3comment := "var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\n// 说明：勿用 setErr(\"bogus_kind\", ...)（散文，不是用法）\n"
	rc, _ = errorKindsGateCheck(mk(r3comment))
	fmt.Fprintf(stdout, "  自检⑥ R3 注释位出现同句 ⇒ rc=%d（要 0 —— 注释剥掉后不误报）\n", rc)
	ok = ok && rc == 0

	// N2 子判据成对（§3.3 N2 · 缺口 GAP-20261012-20）—— 全合成夹具，不碰真命令树。
	n2spec := &contract.SelfUpdateSpec{
		NearSynonymForbidden: []string{"自更新", "升级", "upgrade", "self-update"},
		SemanticsSplit: map[string]any{
			"zerg update":      map[string]any{"command": "update", "summary_must_contain": "源码式自更新", "danger_source_must_contain": "F-3"},
			"zerg core update": map[string]any{"command": "core update", "summary_must_contain": "主控自身换件", "danger_source_must_contain": "P12"},
		},
	}
	n2good := []updateCmdShape{
		{Path: "update", Summary: "源码式自更新（git 检出 → 构建 → 换件 → 回执）", DangerSource: "F-3"},
		{Path: "core update", Summary: "主控自身换件（只换在跑的那一个制品）", DangerSource: "P12"},
		{Path: "task ls", Summary: "列任务", Usage: "zerg task ls"},
	}
	n2rc, _ := errorKindsGateN2Check(n2spec, n2good)
	fmt.Fprintf(stdout, "  自检⑦ N2 合成好件（两条命令各说各的 · 旁的命令无近义动词）⇒ rc=%d（要 0）\n", n2rc)
	ok = ok && n2rc == 0

	n2bad := append(append([]updateCmdShape{}, n2good...),
		updateCmdShape{Path: "task submit", Summary: "提交任务并顺手升级主控", Usage: "zerg task submit"})
	n2rc2, n2lines2 := errorKindsGateN2Check(n2spec, n2bad)
	fmt.Fprintf(stdout, "  自检⑧ N2 合成坏件（概要塞「升级」）⇒ rc=%d（要 1）· %s\n", n2rc2, strings.Join(n2lines2, " | "))
	ok = ok && n2rc2 == 1

	n2rc3, _ := errorKindsGateN2Check(nil, n2good)
	fmt.Fprintf(stdout, "  自检⑨ N2 真源取不到 ⇒ rc=%d（要 2 —— 空转 = 假覆盖，不给结论）\n", n2rc3)
	ok = ok && n2rc3 == 2
	dBase := cekDetailBuildBaseline([]cekDetailSite{{Rel: "core/cmd/zerg/a.go", Line: 3, Literal: true, Value: "seen_one"}})
	dGood := []cekDetailSite{{Rel: "core/cmd/zerg/a.go", Line: 3, Literal: true, Value: "seen_one"}}
	drc, _ := errorKindsGateDetailCheck(dBase, dGood)
	fmt.Fprintf(stdout, "  自检⑩ detail 合成好件（现读 = 登记）⇒ rc=%d（要 0）\n", drc)
	ok = ok && drc == 0

	dBad := append(append([]cekDetailSite{}, dGood...),
		cekDetailSite{Rel: "core/cmd/zerg/b.go", Line: 9, Literal: true, Value: "brand_new"})
	drc2, dlines2 := errorKindsGateDetailCheck(dBase, dBad)
	fmt.Fprintf(stdout, "  自检⑪ detail 未登记新值 ⇒ rc=%d（要 1）· %s\n", drc2, strings.Join(filter(dlines2, "✗"), " | "))
	ok = ok && drc2 == 1

	drc3, _ := errorKindsGateDetailCheck(nil, dGood)
	fmt.Fprintf(stdout, "  自检⑫ detail 基线取不到 ⇒ rc=%d（要 2 —— 空转 = 假覆盖，不给结论）\n", drc3)
	ok = ok && drc3 == 2

	if ok {
		fmt.Fprintln(stdout, "自检结论: 全过")
	} else {
		fmt.Fprintln(stdout, "自检结论: **有断言失败**")
	}
	if ok {
		return 0
	}
	return 1
}
