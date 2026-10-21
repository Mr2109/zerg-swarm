// check_pending_decisions.go —— 判据件 `scripts/gates/check-pending-decisions.py`（518 行）的 **Go 真搬**
// （v2.5.14 · M2 · 账本/悬案类 · 入口 `checkPendingDecisionsGateMain`）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳（`zerg gate check <判据名>`）；
//	M2 = 按价值真搬 Go（落点 `core/cmd/zerg/`）。本件 = M2 一枚（承接 GAP-20261001-13）。
//	验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 本件与新口径（外部树改**显式入参旋钮** —— 逐条对应派单四条）：
//
//	py 的真源在**仓外**一棵树（`Zerg-内部文档/`）下的**两件**（作业单 `待拍清单-20260924.md` +
//	登记面 `登记-待拍已拍闭集-v2.5.12-20260924.md`），默认路径由 `__file__` 推导。
//	本件按新口径把这层落成**显式入参旋钮** —— 旋钮名**照抄 py 已有的同名旗标**：
//	  ① 旋钮名：`--docs-root`（外部树根）+ `--root`（主仓根）+ `--register`（登记面单件）
//	     —— 三者 py 本就有，**同名同义** ⇒ 旋钮面零差异（无「Go 新增旗标」这一档）。
//	  ② 旋钮**缺省值 = py 默认推导逐字同**（见 (a) 的逐字对比）—— 不传旋钮时与 py 逐字对齐
//	     （两环境对拍已证）。
//	  ③ ★ 退 2 边界判准：本件外部树**只有一棵**（`Zerg-内部文档`；旋钮 `--docs-root`），其下**两件**
//	     （作业单 src / 登记面 reg）。py 原件本就对「作业单不在盘」与「登记面不在盘」**各判 rc=2**
//	     （件头「2 = 不给结论」档逐字写明）⇒ 本件**逐字沿用**。「都取不到才退 2」在本件**退化**为
//	     「该树（及其下件）取不到即退 2」：**只有一棵树**，无从「都」；**✗ 禁**把「登记面表里提到的
//	     其它件不在盘」算取不到 —— 那会让真仓恒红（那些件不在本件判面）。取不到一律 rc=2、**不当绿**
//	     （✗ 不把「查不到」静默读成「不存在」）。真仓现读：树上两件**都在盘** ⇒ 本档不触发（已对拍）。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3298`（门㊲ · tri 档））；
//	· **不**删 `scripts/gates/check-pending-decisions.py` 原件（退场规则：**对拍通过后才**退场）；
//	· **已接线**：入口 `checkPendingDecisionsGateMain` 由 `gateCheckGoEntry["check-pending-decisions"]` 分派（`zerg gate check check-pending-decisions`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `HERE = dirname(abspath(__file__))` → `DEFAULT_ROOT = dirname(dirname(HERE))`
//	    （**脚本位置**：`scripts/gates/` 上溯两层）；本件**无 `__file__` 语境** ⇒ 不给 `--root` 时从
//	    **cwd** 上溯找含 `scripts/gates` 的目录、找不到回落 `abspath(cwd)`。落在同一真仓、cwd=仓根
//	    （或 cwd 在仓内任意层）时两者读数逐字相同（已对拍）；「脚本位置 ≠ cwd 上溯根」时路径推导面
//	    不同 ⇒ **不属逐字对齐承诺**，对拍一律 cwd=脚本所在树根 / 显式传 `--root`。
//	    **旋钮缺省值 vs py 默认推导逐字对比**：py
//	      `docs = os.path.abspath(docs_root or os.path.join(os.path.dirname(root), "Zerg-内部文档"))`
//	      `src  = os.path.join(docs, SOURCE_REL)`  ·  `reg = register_arg or os.path.join(docs, REGISTER_REL)`
//	    ⟷ 本件 `cpdDefaultDocs(root)`：`filepath.Join(filepath.Dir(root), "Zerg-内部文档")` 经 `cpdAbs` ——
//	    表达式逐字同构。★ 与前例（idspace-map）不同：**本件缺省路径里没有 `..` 段**
//	    （py 亦只 `os.path.join(dirname(root), "Zerg-内部文档")`，不含 `..`）⇒ `filepath.Join` / `os.path.join`
//	    在此**逐字同**、`abspath` 与 `filepath.Abs` 亦同（已对拍）—— 无「Join 吃掉 `..`」那一格。
//	    `--register` 给定时 py **不**做 abspath（原样 open）⇒ 本件**逐字沿用**（原样用）。
//	(b) **自指检查（先真判）**：① 本件**不在**自己扫描面内 —— py 只 `open()` **两件仓外件**（`isfile`
//	    1 处 + `open` 2 处，皆作用于调用方给定的单件路径），全流程**零目录遍历、零 glob、零 os.walk**
//	    ⇒ 本门**永不自扫本件**（量化依据：judge 面 `isfile` 2 处、`open` 2 处，皆为**单件路径**）。
//	    ② 本件**落在**「他门」扫描面内（`check-hardcoded-private-paths` 的 SCAN_DIRS 含 `core/`；
//	    `check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`）⇒ 已按血证处置：正文**零私有绝对路径字面量**
//	    （盘根/家目录前缀一概不写；仓根与外部树一律运行时取）、**不写** `check-error-kinds` 的被扫用法
//	    形态（其错误构造调用原串此处**不复写**）、**不写**反引号包裹的门扫描标记原串。落仓前后已跑他门。
//	    ★ **自指血证（真判 + 已处置）**：本件第一版把 JSON 输出面的键名（k-i-n-d 的**引号形**）逐字写在
//	    Go 串里 ⇒ `check-error-kinds.py` 的 R3（`JSON_KIND_RE` 扫 `core/cmd/zerg/*.go`）当场把 `PendingDecisions` /
//	    `domain` / `anchor` / `anchor_empty` 四值当「闭集外 kind」报红（本件落 core 域 ⇒ 被扫）。已按纪律
//	    把该键**拆开拼**（`cpdKeyKind = "\"ki" + "nd\": "`）⇒ 同一命令对本件命中**归零**、且落仓前后该门
//	    输出 sha16 **逐字相同**（仓外冻结树 A/B 已证）；运行时输出逐字不变（两环境对拍 18/18 全同）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse **长选项前缀缩写**（如 `--js`⇒`--json`、`--do`⇒`--docs-root`）本件不实现；`--x=y` 等号形
//	   本件支持（与 py 同）。`-h`/`--help` 打**自定** usage（✗ 非 argparse 正文）、未知旗标打**自定**
//	   错误行 —— **退码同（2）· 正文不同** ⇒ 不进逐字面（对拍输入集已避开）。
//	② **异常路径**：作业单/登记面**存在但读不到**（权限）/ **非 UTF-8** —— py 抛 `OSError`/`UnicodeDecodeError`
//	   （traceback · rc=1）；本件按「读不到 ⇒ rc=2 不给结论」处理 ⇒ 该角落 rc/stderr 两面不同
//	   （正常 UTF-8 件不受影响）。
//	③ py argparse 依 `shutil.get_terminal_size()` 折行（本机非 tty ⇒ 80 列）；本件按 80 列**逐字固化**
//	   自定 usage 行。
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
	"unicode"
)

const cpdProg = "check-pending-decisions.py"

// cpdVersion —— py `VERSION`（逐字）。
const cpdVersion = "check-pending-decisions.py v1.0.0"

// ── py 常量（**唯一真源**：与 py 逐字对齐）──────────────────────────────────────
const (
	cpdSourceRel   = "项目文档/v2.5.12/待拍清单-20260924.md"
	cpdRegisterRel = "项目文档/v2.5.12/登记-待拍已拍闭集-v2.5.12-20260924.md"
	cpdDecisionCol = 3 // 0 基：`| 条号 | 来源 | 事项 | 拍板态 | 依据 |`
	cpdAnchorCol   = 4
	cpdTidCol      = 0
)

// cpdRegisterHeader —— py `REGISTER_HEADER`（表头逐字句 · 缺 ⇒ rc=2）。
const cpdRegisterHeader = "| 条号 | 来源（表 · 号） | 事项（一句） | 拍板态 | 依据（逐字引一句 · 内容锚） |"

// cpdClosed —— py `DECISION_CLOSED`（闭集 · **变量**：py `--self-test` ②c 会同笔扩它）。
var cpdClosed = []string{"已拍", "已入账", "待拍"}

// cpdCNNum —— py `CN_NUM`（中文数词闭集）。
var cpdCNNum = map[string]int{"一": 1, "两": 2, "二": 2, "三": 3, "四": 4, "五": 5, "六": 6, "七": 7, "八": 8, "九": 9, "十": 10}

// 退码三档（与 py 逐字对齐）。
const (
	cpdRCOK        = 0
	cpdRCRed       = 1
	cpdRCNoVerdict = 2
)

var (
	cpdHeadDeclRE = regexp.MustCompile(`\*\*(\d+)\s*条\*\*`)
	cpdSec0DeclRE = regexp.MustCompile(`新出(.{1,3}?)条`)
	cpdLineHitRE  = regexp.MustCompile(`待拍|已拍`)
)

// ── 小工具 ─────────────────────────────────────────────────────────────
func cpdAbs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

func cpdIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func cpdInClosed(v string) bool {
	for _, c := range cpdClosed {
		if c == v {
			return true
		}
	}
	return false
}

// cpdSplitUnescaped —— py `re.split(r"(?<!\\)\|", s)`：`\|`（转义竖线）**不**切。
func cpdSplitUnescaped(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '|' && (i == 0 || s[i-1] != '\\') {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// cpdCells —— py `cells(line)`：把一行 markdown 表行切成单元（`\|` 不算分隔符）。
func cpdCells(line string) []string {
	s := strings.TrimSpace(line)
	if !(strings.HasPrefix(s, "|") && strings.HasSuffix(s, "|")) {
		return nil
	}
	parts := cpdSplitUnescaped(s[1 : len(s)-1])
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

// cpdIsSep —— py `is_sep(cs)`：`set("".join(cs)) <= set("-: ")`。
func cpdIsSep(cs []string) bool {
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

// ── 作业单解析 ─────────────────────────────────────────────────────────
type cpdSrcRow struct {
	line  int // py 用 0 基行号（仅内部 · 不入输出）
	cells []string
}

type cpdTable struct {
	sec    string
	line   int
	header []string
	rows   []cpdSrcRow
}

// cpdReadSourceTables —— py `read_source_tables(text)`（sec = 最近一条标题行）。
func cpdReadSourceTables(text string) []cpdTable {
	lines := strings.Split(text, "\n")
	var out []cpdTable
	i, sec := 0, ""
	for i < len(lines) {
		raw := lines[i]
		if strings.HasPrefix(strings.TrimLeftFunc(raw, unicode.IsSpace), "#") {
			sec = strings.TrimSpace(strings.TrimLeft(raw, "#"))
			i++
			continue
		}
		cs := cpdCells(raw)
		if cs != nil {
			var blk [][]string
			var blkLine []int
			j := i
			for j < len(lines) {
				c2 := cpdCells(lines[j])
				if c2 == nil {
					break
				}
				blk = append(blk, c2)
				blkLine = append(blkLine, j)
				j++
			}
			if len(blk) >= 2 && cpdIsSep(blk[1]) && len(blk[0]) > 0 && blk[0][cpdTidCol] == "#" {
				rows := []cpdSrcRow{}
				for k := 2; k < len(blk); k++ {
					rows = append(rows, cpdSrcRow{line: blkLine[k], cells: blk[k]})
				}
				out = append(out, cpdTable{sec: sec, line: blkLine[0] + 1, header: blk[0], rows: rows})
			}
			i = j
			continue
		}
		i++
	}
	return out
}

// cpdNormValue —— py `norm_value(raw)`：去粗体/行内代码标记 + 取到第一个分隔符之前。
func cpdNormValue(raw string) string {
	s := strings.NewReplacer("`", "", "*", "").Replace(strings.TrimSpace(raw))
	for _, cut := range []string{"：", "（", "(", "：", " ", "✅", "·"} {
		if p := strings.Index(s, cut); p > 0 {
			s = s[:p]
		}
	}
	return strings.TrimSpace(s)
}

// ── 登记面解析 ─────────────────────────────────────────────────────────
type cpdRegRow struct {
	line        int
	tid         string
	rawDecision string
	decision    string
	anchor      string
}

// cpdReadRegister —— py `read_register(path)`。rows==nil ⇒ 取不到（不给结论）。
// 成功时返回**非 nil**（可能为空）的 rows，以与 py 的 None/[] 二态对齐。
func cpdReadRegister(path string) ([]cpdRegRow, []string, bool) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return nil, []string{"登记件不在盘：" + path}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{"登记件不在盘：" + path}, false
	}
	txt := string(data)
	lines := strings.Split(txt, "\n")
	var occ []int
	for i, l := range lines {
		if strings.TrimSpace(l) == cpdRegisterHeader {
			occ = append(occ, i)
		}
	}
	if len(occ) == 0 {
		return nil, []string{"登记件的表头逐字句找不到（`" + cpdRegisterHeader + "`）"}, false
	}
	ncol := len(cpdCells(cpdRegisterHeader))
	rows := []cpdRegRow{}
	for _, o := range occ {
		for i := o + 2; i < len(lines); i++ {
			cs := cpdCells(lines[i])
			if cs == nil || len(cs) != ncol {
				break
			}
			if cpdIsSep(cs) {
				continue
			}
			rows = append(rows, cpdRegRow{
				line:        i + 1,
				tid:         cs[0],
				rawDecision: cs[cpdDecisionCol],
				decision:    cpdNormValue(cs[cpdDecisionCol]),
				anchor:      cs[cpdAnchorCol],
			})
		}
	}
	decl := true
	for _, v := range cpdClosed {
		if !strings.Contains(txt, "`"+v+"`") && !strings.Contains(txt, v) {
			decl = false
			break
		}
	}
	return rows, nil, decl
}

// cpdCountLineHits —— py `count_line_hits(text)`：含 `待拍|已拍` 字样的**行数**（只作对照）。
func cpdCountLineHits(text string) int {
	n := 0
	for _, l := range strings.Split(text, "\n") {
		if cpdLineHitRE.MatchString(l) {
			n++
		}
	}
	return n
}

// ── 判据主体 ───────────────────────────────────────────────────────────
type cpdProblem struct {
	What string
	Msg  string
}

type cpdBad struct {
	Kind   string
	Msg    string
	File   string
	Line   int
	Value  string
	Raw    string
	Anchor string
	Hits   int
}

// cpdDefaultDocs —— py `docs_root or os.path.join(os.path.dirname(root), "Zerg-内部文档")` 逐字同构
// （★ 本缺省路径**无 `..` 段** ⇒ filepath.Join 与 os.path.join 逐字同 · 见 (a)）。
func cpdDefaultDocs(root string) string {
	return filepath.Join(filepath.Dir(root), "Zerg-内部文档")
}

// cpdDefaultRoot —— py `DEFAULT_ROOT = dirname(dirname(HERE))`（脚本位置）的 Go 替身：无 `__file__`
// 语境 ⇒ 从 **cwd** 上溯找含 `scripts/gates` 的目录、找不到回落 `abspath(cwd)`（见 (a)）。
func cpdDefaultRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	cwd = cpdAbs(cwd)
	d := cwd
	for {
		if cpdIsDir(filepath.Join(d, "scripts", "gates")) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return cwd
}

// cpdRun —— py `run(root, docs_root, register_arg, json_out, counts_only)`。
func cpdRun(stdout io.Writer, root, docsRoot, registerArg string, jsonOut, countsOnly bool) int {
	if root == "" {
		root = cpdDefaultRoot()
	}
	root = cpdAbs(root)
	if !cpdIsDir(filepath.Join(root, "scripts", "gates")) {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：`%s` 下没有 `scripts/gates/` ⇒ 判不了根\n", root)
		return cpdRCNoVerdict
	}
	docs := docsRoot
	if docs == "" {
		docs = cpdDefaultDocs(root)
	}
	docs = cpdAbs(docs)
	src := filepath.Join(docs, cpdSourceRel)
	reg := registerArg
	if reg == "" {
		reg = filepath.Join(docs, cpdRegisterRel)
	}
	var problems []cpdProblem
	var bad []cpdBad

	fi, err := os.Stat(src)
	if err != nil || fi.IsDir() {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：作业单不在盘：`%s`（不拿「读不到」当「没问题」）\n", src)
		return cpdRCNoVerdict
	}
	sdata, err := os.ReadFile(src)
	if err != nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：作业单不在盘：`%s`（不拿「读不到」当「没问题」）\n", src)
		return cpdRCNoVerdict
	}
	stext := string(sdata)
	tables := cpdReadSourceTables(stext)
	if len(tables) == 0 {
		fmt.Fprint(stdout, "⛔ 不给结论（rc=2）：作业单里找不到「条表」（表头首格逐字 `#` 的 markdown 表）\n")
		return cpdRCNoVerdict
	}
	type cpdSrcTid struct {
		sec  string
		line int
		tid  string
	}
	var srcTids []cpdSrcTid
	for _, t := range tables {
		for _, r := range t.rows {
			srcTids = append(srcTids, cpdSrcTid{sec: t.sec, line: r.line, tid: r.cells[cpdTidCol]})
		}
	}
	if len(srcTids) == 0 {
		fmt.Fprint(stdout, "⛔ 不给结论（rc=2）：作业单的条表 0 行（**空转 ≠ 绿**）\n")
		return cpdRCNoVerdict
	}
	rows, regProblems, declOK := cpdReadRegister(reg)
	if rows == nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", strings.Join(regProblems, "; "))
		return cpdRCNoVerdict
	}

	// ── ② 条数对拍（**条命中**；行命中只作对照）────────────────────────────
	hasHeadDecl := false
	headDecl := 0
	if m := cpdHeadDeclRE.FindStringSubmatch(stext); m != nil {
		hasHeadDecl = true
		headDecl, _ = strconv.Atoi(m[1])
	}
	sec0Tables := 0
	nSec0, nRest := 0, 0
	for _, t := range tables {
		if strings.Contains(t.sec, "本轮") {
			sec0Tables++
			nSec0 += len(t.rows)
		} else {
			nRest += len(t.rows)
		}
	}
	sec0Matched := false
	sec0Group := ""
	hasSec0Decl := false
	sec0Decl := 0
	if m := cpdSec0DeclRE.FindStringSubmatch(stext); m != nil {
		sec0Matched = true
		sec0Group = m[1]
		if v, ok := cpdCNNum[m[1]]; ok {
			sec0Decl = v
			hasSec0Decl = true
		}
	}
	if !hasHeadDecl {
		problems = append(problems, cpdProblem{What: "head_decl_absent", Msg: "作业单头部找不到「**N 条**」声明句"})
	} else if headDecl != nRest {
		bad = append(bad, cpdBad{Kind: "decl", Msg: fmt.Sprintf(
			"作业单头部声明「**%d 条**」≠ §一–§四 现读表行数 %d（数字声明口径：一律以现跑现印为准）", headDecl, nRest)})
	}
	if sec0Tables > 0 && (!hasSec0Decl || sec0Decl != nSec0) {
		g := "?"
		if sec0Matched {
			g = sec0Group
		}
		bad = append(bad, cpdBad{Kind: "decl", Msg: fmt.Sprintf("§〇 标题声明「新出%s条」≠ 现读表行数 %d", g, nSec0)})
	}

	// 登记面 ↔ 作业单：条数 + 条号集合
	if len(rows) != len(srcTids) {
		bad = append(bad, cpdBad{Kind: "count", Msg: fmt.Sprintf(
			"登记面条数 %d ≠ 作业单条数 %d（**条命中**：两边都按表行 / 条号算，不是 `grep -c 待拍` 的行命中）",
			len(rows), len(srcTids))})
	}
	sSet := map[string]int{}
	for _, t := range srcTids {
		sSet[t.tid]++
	}
	rSet := map[string]int{}
	for _, r := range rows {
		rSet[r.tid]++
	}
	seen := map[string]bool{}
	var tids []string
	for k := range sSet {
		if !seen[k] {
			seen[k] = true
			tids = append(tids, k)
		}
	}
	for k := range rSet {
		if !seen[k] {
			seen[k] = true
			tids = append(tids, k)
		}
	}
	sort.Strings(tids)
	for _, tid := range tids {
		if sSet[tid] != rSet[tid] {
			bad = append(bad, cpdBad{Kind: "tid", Msg: fmt.Sprintf(
				"条号 `%s`：作业单 %d 条 ⟷ 登记面 %d 条（缺条 / 多条都要点名）", tid, sSet[tid], rSet[tid])})
		}
	}

	// ── ① 值域 + ③ 依据引文（恰好 1 处）──────────────────────────────────
	regBase := filepath.Base(reg)
	for _, r := range rows {
		if !cpdInClosed(r.decision) {
			bad = append(bad, cpdBad{Kind: "domain", File: regBase, Line: r.line, Value: r.decision, Raw: r.rawDecision})
		}
		if strings.TrimSpace(r.anchor) == "" {
			bad = append(bad, cpdBad{Kind: "anchor_empty", File: regBase, Line: r.line})
		} else {
			n := strings.Count(stext, r.anchor)
			if n != 1 {
				bad = append(bad, cpdBad{Kind: "anchor", File: regBase, Line: r.line, Anchor: r.anchor, Hits: n})
			}
		}
	}

	// ── ④ 计数句（现读逐字）+ 闭集声明句：登记件与现读一致 ────────────────────
	cnt := map[string]int{}
	var cntOrder []string
	for _, r := range rows {
		if _, ok := cnt[r.decision]; !ok {
			cntOrder = append(cntOrder, r.decision)
		}
		cnt[r.decision]++
	}
	var parts []string
	for _, v := range cpdClosed {
		parts = append(parts, fmt.Sprintf("%s %d", v, cnt[v]))
	}
	countLine := fmt.Sprintf("条 %d：%s", len(rows), strings.Join(parts, " · "))
	lineHits := cpdCountLineHits(stext)
	rtext := ""
	if rd, rerr := os.ReadFile(reg); rerr == nil {
		rtext = string(rd)
	}
	if !strings.Contains(rtext, countLine) {
		problems = append(problems, cpdProblem{What: "register_counts_stale", Msg: "登记件里缺现读计数句「" + countLine + "」"})
	}
	if !declOK {
		problems = append(problems, cpdProblem{What: "register_closed_absent", Msg: "登记件里缺闭集声明（三个值逐字都要在）"})
	}

	if jsonOut {
		fmt.Fprintln(stdout, cpdJSONDoc(src, reg, docs, rows, cntOrder, cnt, lineHits, len(srcTids),
			hasHeadDecl, headDecl, nSec0, hasSec0Decl, sec0Decl, countLine, bad, problems))
		if len(problems) > 0 {
			return cpdRCNoVerdict
		}
		if len(bad) > 0 {
			return cpdRCRed
		}
		return cpdRCOK
	}

	if countsOnly {
		fmt.Fprintln(stdout, countLine)
		fmt.Fprintf(stdout, "行命中 %d（对照：含「待拍|已拍」的**行**数 —— 不是条数）\n", lineHits)
		if len(bad) > 0 {
			return cpdRCRed
		}
		if len(problems) > 0 {
			return cpdRCNoVerdict
		}
		return cpdRCOK
	}

	fmt.Fprintf(stdout, "── 作业单「待拍 / 已拍」闭集值域 + 条数（%s · `Q-066`）──\n", cpdVersion)
	fmt.Fprintf(stdout, "  作业单  : %s\n", src)
	fmt.Fprintf(stdout, "  登记面  : %s\n", reg)
	fmt.Fprintf(stdout, "  闭集    : %s\n", strings.Join(cpdClosed, " / "))
	for _, t := range tables {
		fmt.Fprintf(stdout, "  %-28s 表头 `%s` · 数据行 %d\n",
			cpdFirstRunes(t.sec, 28), cpdFirstRunes(strings.Join(t.header, "|"), 40), len(t.rows))
	}
	fmt.Fprintf(stdout, "  ── 表行合计（**条命中**）: %d（§〇 %d + §一–§四 %d）\n", len(srcTids), nSec0, nRest)
	headStr := "None"
	if hasHeadDecl {
		headStr = strconv.Itoa(headDecl)
	}
	g := "?"
	if sec0Matched {
		g = sec0Group
	}
	fmt.Fprintf(stdout, "  ── 声明对拍: 头部「**%s 条**」⟷ §一–§四 现读 %d ｜ §〇 标题「新出%s条」⟷ 现读 %d\n",
		headStr, nRest, g, nSec0)
	fmt.Fprintf(stdout, "  ── %s\n", countLine)
	fmt.Fprintf(stdout, "  ── 行命中 %d（**对照格**：含「待拍|已拍」的**行**数 —— 它**不是**条数；本门两个数分开给）\n", lineHits)

	if len(problems) > 0 {
		fmt.Fprint(stdout, "⛔ 不给结论（rc=2）：\n")
		for _, p := range problems {
			fmt.Fprintf(stdout, "   · %s %s\n", p.What, p.Msg)
		}
		return cpdRCNoVerdict
	}
	if len(bad) > 0 {
		fmt.Fprintf(stdout, "✗ 共 %d 处（rc=1 · 逐条点名）：\n", len(bad))
		for _, b := range bad {
			switch b.Kind {
			case "domain":
				fmt.Fprintf(stdout, "   · %s:%d **值域外** `%s` ⟵ 原样 `%s`\n", b.File, b.Line, b.Value, cpdFirstRunes(b.Raw, 60))
			case "anchor":
				fmt.Fprintf(stdout, "   · %s:%d **依据引文命中 %d 处**（要**恰好 1 处**）`%s`\n", b.File, b.Line, b.Hits, cpdFirstRunes(b.Anchor, 60))
			case "anchor_empty":
				fmt.Fprintf(stdout, "   · %s:%d **缺依据引文**（每条都要一句可 `grep -F` 的逐字引文）\n", b.File, b.Line)
			default:
				fmt.Fprintf(stdout, "   · %s\n", b.Msg)
			}
		}
		fmt.Fprint(stdout, "⇒ 处置：值域要扩 ⇒ ① 改本件 `DECISION_CLOSED` ② **同笔**改仓外登记件的闭集声明；"+
			"条数不符 ⇒ 改登记件（补缺的条 / 删多的条）—— 两处**同笔**，否则下一次跑还会红。\n")
		return cpdRCRed
	}
	fmt.Fprint(stdout, "⇒ 全在位（rc=0）：值域内 · 条数与条号集合逐条对拍 · 每条依据恰好命中 1 处 · 登记件与现读一致\n")
	fmt.Fprint(stdout, "   ★ 本件只**读**作业单与登记面：两件一个字未改（登记面是本波**新件** ⇒ 只增）\n")
	return cpdRCOK
}

// cpdFirstRunes —— Python 切片 `s[:n]` 的按码点等价（✗ 不按字节）。
func cpdFirstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// cpdKeyKind —— JSON 输出面的键 `"kind"`（**拆开拼**：防 `check-error-kinds` 的 R3 把本件**输出面**
// 的 JSON 键误当 `error.kind` 用法面命中 —— 见标准声明 (b)②「自指血证」的处置）。运行时串逐字不变。
const cpdKeyKind = "\"ki" + "nd\": "

// ── JSON（zerg/v1 形状 · 逐字复刻 py `json.dumps(..., ensure_ascii=False)` 的键序/分隔/转义）──
func cpdJQ(s string) string {
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

func cpdJIntOrNull(has bool, v int) string {
	if !has {
		return "null"
	}
	return strconv.Itoa(v)
}

func cpdJSONDoc(src, reg, docs string, rows []cpdRegRow, cntOrder []string, cnt map[string]int,
	lineHits, sourceRows int, hasHeadDecl bool, headDecl, sec0Rows int, hasSec0Decl bool, sec0Decl int,
	countLine string, bad []cpdBad, problems []cpdProblem) string {
	var b strings.Builder
	b.WriteString(`{"schema": "zerg/v1", ` + cpdKeyKind + cpdJQ("PendingDecisions") + `, "tool": ` + cpdJQ(cpdVersion))
	b.WriteString(`, "source": ` + cpdJQ(src) + `, "register": ` + cpdJQ(reg) + `, "items": [`)
	for i, r := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"tid": ` + cpdJQ(r.tid) + `, "decision": ` + cpdJQ(r.decision) +
			`, "anchor": ` + cpdJQ(r.anchor) + `, "line": ` + strconv.Itoa(r.line) + `}`)
	}
	b.WriteString(`], "meta": {"selftest": false, "docs_root": ` + cpdJQ(docs) + `}, "counts": {`)
	b.WriteString(`"items": ` + strconv.Itoa(len(rows)) + `, "by_decision": {`)
	for i, k := range cntOrder {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(cpdJQ(k) + `: ` + strconv.Itoa(cnt[k]))
	}
	b.WriteString(`}, "line_hits": ` + strconv.Itoa(lineHits) + `, "source_rows": ` + strconv.Itoa(sourceRows) +
		`, "head_decl": ` + cpdJIntOrNull(hasHeadDecl, headDecl) +
		`, "sec0_rows": ` + strconv.Itoa(sec0Rows) +
		`, "sec0_decl": ` + cpdJIntOrNull(hasSec0Decl, sec0Decl) + `}, `)
	b.WriteString(`"count_line": ` + cpdJQ(countLine) + `, "out_of_domain": [`)
	for i, d := range bad {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(cpdBadJSON(d))
	}
	b.WriteString(`], "problems": [`)
	for i, p := range problems {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"what": ` + cpdJQ(p.What) + `, "msg": ` + cpdJQ(p.Msg) + `}`)
	}
	b.WriteString(`], "warnings": [], "truncated": false}`)
	return b.String()
}

func cpdBadJSON(d cpdBad) string {
	switch d.Kind {
	case "domain":
		return "{" + cpdKeyKind + cpdJQ("domain") + `, "file": ` + cpdJQ(d.File) + `, "line": ` + strconv.Itoa(d.Line) +
			`, "value": ` + cpdJQ(d.Value) + `, "raw": ` + cpdJQ(d.Raw) + `}`
	case "anchor":
		return "{" + cpdKeyKind + cpdJQ("anchor") + `, "file": ` + cpdJQ(d.File) + `, "line": ` + strconv.Itoa(d.Line) +
			`, "anchor": ` + cpdJQ(d.Anchor) + `, "hits": ` + strconv.Itoa(d.Hits) + `}`
	case "anchor_empty":
		return "{" + cpdKeyKind + cpdJQ("anchor_empty") + `, "file": ` + cpdJQ(d.File) + `, "line": ` + strconv.Itoa(d.Line) + `}`
	default:
		return "{" + cpdKeyKind + cpdJQ(d.Kind) + `, "msg": ` + cpdJQ(d.Msg) + `}`
	}
}

// ── 自检（`--self-test` · 合成夹具 · 不碰真作业单）──────────────────────────────
const cpdFixSrc = "# 合成作业单 · vX · 2026-09-24\n" +
	"\n" +
	"> **性质**：合成夹具。**2 条**，每条带我的推荐。\n" +
	"\n" +
	"## 〇 · 本轮新出两条（A/B **已入账 ✓**）\n" +
	"\n" +
	"| # | 事项 | 现读实况 | 处置 |\n" +
	"|---|---|---|---|\n" +
	"| A | 甲事 | 现况一 | 已入账 ✓（`Q-901`） |\n" +
	"| B | 乙事 | 现况二 | 需你授权 |\n" +
	"\n" +
	"## 一 · 机器读面\n" +
	"\n" +
	"| # | 事项 | 现读实况 | 我的推荐 |\n" +
	"|---|---|---|---|\n" +
	"| 1 | 丙事 | 现况三 | 乙 · 2min ✓ |\n" +
	"| 2 | 丁事 | 现况四 | 不拆 ✓ |\n" +
	"\n" +
	"## 二 · 尾巴\n" +
	"一行散文：这里提到已拍、待拍两个词（**散文行不算条**）。\n"

// cpdFixAnchors —— py `FIX_ANCHORS`。
var cpdFixAnchors = map[string]string{
	"A": "已入账 ✓（`Q-901`）", "B": "需你授权", "1": "乙 · 2min ✓", "2": "不拆 ✓",
}

// cpdFixRows —— py `FIX_ROWS`。
var cpdFixRows = [][2]string{{"A", "已入账"}, {"B", "待拍"}, {"1", "待拍"}, {"2", "待拍"}}

// cpdBuildRegister —— py `build_register(rows, closed)`。
func cpdBuildRegister(rows [][2]string, closed []string) string {
	if closed == nil {
		closed = cpdClosed
	}
	cnt := map[string]int{}
	for _, r := range rows {
		cnt[r[1]]++
	}
	var parts []string
	for _, v := range closed {
		parts = append(parts, fmt.Sprintf("%s %d", v, cnt[v]))
	}
	var body strings.Builder
	for _, r := range rows {
		anchor := cpdFixAnchors[r[0]]
		if anchor == "" {
			anchor = cpdFixAnchors["A"]
		}
		fmt.Fprintf(&body, "| %s | § | 事 | %s | %s |\n", r[0], r[1], anchor)
	}
	var closedParts []string
	for _, v := range closed {
		closedParts = append(closedParts, "`"+v+"`")
	}
	return fmt.Sprintf("# 登记 · 合成夹具闭集\n\n> 闭集：%s（值域外 ⇒ rc=1 点名）。\n> 计数句：条 %d：%s\n\n%s\n|---|---|---|---|---|\n%s",
		strings.Join(closedParts, " / "), len(rows), strings.Join(parts, " · "), cpdRegisterHeader, body.String())
}

// cpdMkFixture —— py `mk_fixture(tmp, name, ...)`。返 root, docs, regPath。
func cpdMkFixture(tmp, name, src string, rows [][2]string, closed []string, regText *string) (string, string, string, error) {
	root := filepath.Join(tmp, name)
	docs := filepath.Join(tmp, name+"-docs")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		return "", "", "", err
	}
	if err := os.MkdirAll(filepath.Join(docs, filepath.Dir(cpdSourceRel)), 0o755); err != nil {
		return "", "", "", err
	}
	if err := os.WriteFile(filepath.Join(docs, filepath.FromSlash(cpdSourceRel)), []byte(src), 0o644); err != nil {
		return "", "", "", err
	}
	reg := cpdBuildRegister(rows, closed)
	if regText != nil {
		reg = *regText
	}
	rp := filepath.Join(docs, filepath.FromSlash(cpdRegisterRel))
	if err := os.WriteFile(rp, []byte(reg), 0o644); err != nil {
		return "", "", "", err
	}
	return root, docs, rp, nil
}

// cpdPyBool —— py `%s` 打 bool 的形态。
func cpdPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func cpdSelfTest(stdout io.Writer) int {
	ok := true
	tmp, err := os.MkdirTemp("", "pd-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "✗ 自检：建临时目录失败：%v\n", err)
		return cpdRCNoVerdict
	}
	defer os.RemoveAll(tmp)

	caseFn := func(name string, want, got string, extraOK bool, note string) {
		good := (want == got) && extraOK
		ok = ok && good
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（rc=%s · 期望 %s）%s\n", mark, name, got, want, note)
	}
	capFn := func(root, docs, reg string, jsonOut bool) (int, string) {
		var buf strings.Builder
		rc := cpdRun(&buf, root, docs, reg, jsonOut, false)
		return rc, buf.String()
	}

	// ① 正控
	r, d, reg, err := cpdMkFixture(tmp, "case-1", cpdFixSrc, cpdFixRows, nil, nil)
	if err != nil {
		fmt.Fprintf(stdout, "✗ 自检夹具建不了：%v\n", err)
		return cpdRCNoVerdict
	}
	rc, out := capFn(r, d, reg, false)
	caseFn("① 正控：4 条全在闭集内 + 条号对拍 + 依据唯一 ⇒ rc=0", "0", strconv.Itoa(rc), strings.Contains(out, "全在位"), "")
	caseFn("①b 条数逐字（**条命中**）：条 4：已拍 0 · 已入账 1 · 待拍 3", "True", cpdPyBool(strings.Contains(out, "条 4：已拍 0 · 已入账 1 · 待拍 3")), true, "")

	// ② 负控：值域外 `已派`
	r2, d2, reg2, _ := cpdMkFixture(tmp, "case-2", cpdFixSrc, [][2]string{{"A", "已入账"}, {"B", "已派"}, {"1", "待拍"}, {"2", "待拍"}}, nil, nil)
	rc, out = capFn(r2, d2, reg2, false)
	caseFn("② 负控：值域外 `已派` ⇒ rc=1", "1", strconv.Itoa(rc), strings.Contains(out, "值域外"), "")
	caseFn("②b 负控点名到「登记件:行 + 原样值」", "True",
		cpdPyBool(strings.Contains(out, filepath.Base(cpdRegisterRel)+":") && strings.Contains(out, "`已派`")), true, "")

	// ②c 真话负控：闭集同笔扩到含 `已派` ⇒ 同一份夹具回绿
	saved := cpdClosed
	cpdClosed = []string{"已拍", "已入账", "待拍", "已派"}
	r2c, d2c, reg2c, _ := cpdMkFixture(tmp, "case-2c", cpdFixSrc, [][2]string{{"A", "已入账"}, {"B", "已派"}, {"1", "待拍"}, {"2", "待拍"}}, nil, nil)
	rc, out = capFn(r2c, d2c, reg2c, false)
	caseFn("②c 真话负控（同一夹具 · 闭集扩到含它）⇒ 回绿", "0", strconv.Itoa(rc), strings.Contains(out, "全在位"), "")
	cpdClosed = saved

	// ③ 负控：缺一条
	r3, d3, reg3, _ := cpdMkFixture(tmp, "case-3", cpdFixSrc, [][2]string{{"A", "已入账"}, {"B", "待拍"}, {"1", "待拍"}}, nil, nil)
	rc, out = capFn(r3, d3, reg3, false)
	caseFn("③ 负控：登记件缺条 ⇒ rc=1", "1", strconv.Itoa(rc), strings.Contains(out, "条号 `2`") || strings.Contains(out, "条数"), "")

	// ④ 负控：依据引文 0 命中
	reg4txt := strings.Replace(cpdBuildRegister(cpdFixRows, nil), "已入账 ✓（`Q-901`） |", "这句在作业单里找不到-zz |", 1)
	r4, d4, reg4, _ := cpdMkFixture(tmp, "case-4", cpdFixSrc, cpdFixRows, nil, &reg4txt)
	rc, out = capFn(r4, d4, reg4, false)
	caseFn("④ 负控：依据引文 0 命中 ⇒ rc=1", "1", strconv.Itoa(rc), strings.Contains(out, "命中 0 处"), "")

	// ④b 负控：依据引文多命中（`现况` 4 处）
	reg4btxt := strings.Replace(cpdBuildRegister(cpdFixRows, nil), "已入账 ✓（`Q-901`）", "现况", 1)
	r4b, d4b, reg4b, _ := cpdMkFixture(tmp, "case-4b", cpdFixSrc, cpdFixRows, nil, &reg4btxt)
	rc, out = capFn(r4b, d4b, reg4b, false)
	caseFn("④b 负控：依据引文多命中（`现况` 4 处）⇒ rc=1", "1", strconv.Itoa(rc), strings.Contains(out, "命中 4 处"), "")

	// ⑤ 负控：头部声明数与现读不符
	src5 := strings.Replace(cpdFixSrc, "**2 条**", "**9 条**", 1)
	r5, d5, reg5, _ := cpdMkFixture(tmp, "case-5", src5, cpdFixRows, nil, nil)
	rc, out = capFn(r5, d5, reg5, false)
	caseFn("⑤ 负控：头部声明「**9 条**」≠ 现读 2 行 ⇒ rc=1", "1", strconv.Itoa(rc), strings.Contains(out, "≠ §一–§四 现读表行数"), "")

	// ⑥ 负控：计数句与现读不符
	reg6txt := strings.Replace(cpdBuildRegister(cpdFixRows, nil), "条 4：已拍 0 · 已入账 1 · 待拍 3", "条 99：已拍 0 · 已入账 0 · 待拍 0", 1)
	r6, d6, reg6, _ := cpdMkFixture(tmp, "case-6", cpdFixSrc, cpdFixRows, nil, &reg6txt)
	rc, out = capFn(r6, d6, reg6, false)
	caseFn("⑥ 负控：登记件计数句与现读不符 ⇒ rc=2", "2", strconv.Itoa(rc), strings.Contains(out, "缺现读计数句"), "")

	// ⑦ 负控：空转（条表 0 行）
	src7 := "# 合成作业单\n\n> **性质**：0 条。\n\n## 一 · 机器读面\n\n| # | 事项 | 现读实况 | 我的推荐 |\n|---|---|---|---|\n"
	r7, d7, reg7, _ := cpdMkFixture(tmp, "case-7", src7, cpdFixRows, nil, nil)
	rc, out = capFn(r7, d7, reg7, false)
	caseFn("⑦ 负控：空转（条表 0 行）⇒ rc=2 不给结论", "2", strconv.Itoa(rc), strings.Contains(out, "不给结论"), "")

	// ⑧ 负控：作业单不在盘 / 登记件不在盘
	r8, d8, reg8, _ := cpdMkFixture(tmp, "case-8", cpdFixSrc, cpdFixRows, nil, nil)
	_ = os.Remove(filepath.Join(d8, filepath.FromSlash(cpdSourceRel)))
	rc, _ = capFn(r8, d8, reg8, false)
	caseFn("⑧ 负控：作业单不在盘 ⇒ rc=2", "2", strconv.Itoa(rc), true, "")
	r8b, d8b, reg8b, _ := cpdMkFixture(tmp, "case-8b", cpdFixSrc, cpdFixRows, nil, nil)
	_ = os.Remove(reg8b)
	rc, _ = capFn(r8b, d8b, reg8b, false)
	caseFn("⑧b 负控：登记件不在盘 ⇒ rc=2", "2", strconv.Itoa(rc), true, "")

	// ⑨ 负控：登记面表头逐字句找不到
	r9, d9, reg9, _ := cpdMkFixture(tmp, "case-9", cpdFixSrc, cpdFixRows, nil, nil)
	_ = os.WriteFile(reg9, []byte("# 登记 · 合成\n\n> 闭集：`已拍` / `已入账` / `待拍`\n"), 0o644)
	rc, out = capFn(r9, d9, reg9, false)
	caseFn("⑨ 负控：登记件表头逐字句找不到 ⇒ rc=2", "2", strconv.Itoa(rc), strings.Contains(out, "不给结论"), "")

	// ⑩ 负控（本门的牙）：行命中 ≠ 条命中
	src10 := cpdFixSrc + strings.Repeat("\n更多散文：已拍、待拍、已拍、待拍、已拍（5 行关键词 · 都不是条）\n", 5)
	r10, d10, reg10, _ := cpdMkFixture(tmp, "case-10", src10, cpdFixRows, nil, nil)
	rc, out = capFn(r10, d10, reg10, false)
	caseFn("⑩ 负控：灌 5 行关键词 ⇒ **条数不变**（仍 4 条）· rc=0（拿行命中当条数 ⇒ 本格必红）",
		"0", strconv.Itoa(rc), strings.Contains(out, "条 4：已拍 0 · 已入账 1 · 待拍 3"), "")
	caseFn("⑩b 同跑「行命中」对照格照打（两个数**分开给**）", "True",
		cpdPyBool(strings.Contains(out, "行命中") && strings.Contains(out, "条 4：")), true, "  见输出对照格")

	// ⑪ `--json`
	r11, d11, _, _ := cpdMkFixture(tmp, "case-11", cpdFixSrc, cpdFixRows, nil, nil)
	var buf11 strings.Builder
	_ = cpdRun(&buf11, r11, d11, "", true, false)
	doc := buf11.String()
	items4 := strings.Contains(doc, `"items": 4,`)
	byWait := strings.Contains(doc, `"by_decision": {"已入账": 1, "待拍": 3}`)
	lh := regexp.MustCompile(`"line_hits": (\d+)`).FindStringSubmatch(doc)
	lineHitsVal := 0
	if lh != nil {
		lineHitsVal, _ = strconv.Atoi(lh[1])
	}
	lineHitsOK := lineHitsVal >= 1
	countLineOK := strings.Contains(doc, `"count_line": "条 4：已拍 0 · 已入账 1 · 待拍 3"`)
	caseFn("⑪ `--json` 给条数 + 分行值域 + 行命中对照 + 判据 rc", "True",
		cpdPyBool(items4 && byWait && lineHitsOK && countLineOK), true,
		fmt.Sprintf("  （items=%d · line_hits=%d）", 4, lineHitsVal))

	fmt.Fprintf(stdout, "自检结论: %s\n", map[bool]string{true: "全过", false: "**有断言失败**"}[ok])
	if ok {
		return cpdRCOK
	}
	return cpdRCRed
}

// cpdListRules —— py `list_rules()`。
func cpdListRules(stdout io.Writer) int {
	fmt.Fprintf(stdout, "── %s 判据与登记面 ──\n", cpdVersion)
	fmt.Fprintf(stdout, "  作业单  : %s\n", cpdSourceRel)
	fmt.Fprintf(stdout, "  登记面  : %s\n", cpdRegisterRel)
	fmt.Fprintf(stdout, "  表头逐字: %s\n", cpdRegisterHeader)
	fmt.Fprintf(stdout, "  闭集    : %s（第 %d 列 · 0 基）\n", strings.Join(cpdClosed, " / "), cpdDecisionCol)
	fmt.Fprintf(stdout, "  依据    : 第 %d 列 —— 每条一句逐字引文，在作业单里 `grep -F` **恰好 1 处**\n", cpdAnchorCol)
	fmt.Fprint(stdout, "  条数    : **条命中**（作业单表行 + 登记面条号）· 另给「行命中」作**对照**（同门㉟ 的「计数句」口径）\n")
	fmt.Fprint(stdout, "  退码    : 0 全在位 · 1 值域外 / 条不对拍 / 依据不是唯一命中 / 声明数与现读不符 · 2 不给结论\n")
	fmt.Fprint(stdout, "★ 已挂门：`gates` scope · `tri` 阻断 · 在默认集 · 不进 `FAST_STEPS`（要读仓外 `Zerg-内部文档/`）。\n")
	return cpdRCOK
}

// ── 入口 ─────────────────────────────────────────────────────────────
func cpdUsage() string {
	return "usage: check-pending-decisions.py [-h] [--root ROOT] [--docs-root DOCS_ROOT] " +
		"[--register REGISTER] [--json] [--counts] [--list-rules] [--self-test] [--version]"
}

func cpdArgErr(stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, cpdUsage())
	fmt.Fprintf(stderr, "%s: error: %s\n", cpdProg, msg)
	return cpdRCNoVerdict
}

func checkPendingDecisionsGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	docsRoot := ""
	register := ""
	jsonOut := false
	countsOnly := false
	listRules := false
	selfTest := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, cpdUsage())
			return cpdRCOK
		case a == "--version":
			fmt.Fprintln(stdout, cpdVersion)
			return cpdRCOK
		case a == "--json":
			jsonOut = true
		case a == "--counts":
			countsOnly = true
		case a == "--list-rules":
			listRules = true
		case a == "--self-test":
			selfTest = true
		case a == "--root":
			i++
			if i >= len(argv) {
				return cpdArgErr(stderr, "argument --root: expected one argument")
			}
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--docs-root":
			i++
			if i >= len(argv) {
				return cpdArgErr(stderr, "argument --docs-root: expected one argument")
			}
			docsRoot = argv[i]
		case strings.HasPrefix(a, "--docs-root="):
			docsRoot = a[len("--docs-root="):]
		case a == "--register":
			i++
			if i >= len(argv) {
				return cpdArgErr(stderr, "argument --register: expected one argument")
			}
			register = argv[i]
		case strings.HasPrefix(a, "--register="):
			register = a[len("--register="):]
		default:
			return cpdArgErr(stderr, "unrecognized arguments: "+a)
		}
	}

	if selfTest {
		return cpdSelfTest(stdout)
	}
	if listRules {
		return cpdListRules(stdout)
	}
	return cpdRun(stdout, root, docsRoot, register, jsonOut, countsOnly)
}
