// check_idspace_map.go —— 判据件 `scripts/gates/check-idspace-map.py`（328 行）的 **Go 真搬**
// （v2.5.14 · M2 · 候选区间首字母 q–z · 入口 `checkIdspaceMapGateMain`）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳（`zerg gate check <判据名>`）；
//	M2 = 按价值真搬 Go（落点 `core/cmd/zerg/`）。本件 = M2 一枚（承接 GAP-20261001-13）。
//	验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 本件与新口径（仓外树改显式入参旋钮）：
//
//	py 的真源是**仓外**一棵树里的一件（Zerg-内部文档 下的登记件），默认路径由 `__file__` 推导。
//	本件按新口径把这层落成**显式入参旋钮** `--doc`（旋钮名照抄 py 已有的同名旗标）：
//	  ① 旋钮**缺省值 = py 默认推导逐字同**（见 (a) 的逐字对比）—— 不传旋钮时与 py 逐字对齐
//	     （两态对拍已证）；
//	  ② **取不到登记件 ⇒ 退 2「不给结论」✗ 不静默绿** —— py 原件本来就在「登记件不在盘」
//	     一档给 rc=2（照 §17.6 E-M6 口径），本件**逐字沿用**、**不额外加严**。
//	     边界判准（照 who_references.go 的做法）：本件只有**一棵**外部树（Zerg-内部文档 侧的一件），
//	     故「都取不到才退 2」在本件退化为「登记件取不到即退 2」；**✗ 禁**把「映射到/真源件这几列
//	     提到的其它件不在盘」也算取不到 —— 那会让真仓恒红（那些件在 Zerg-文档树里，非本件判面）。
//	     取不到一律 rc=2、**不当绿**（✗ 不把「查不到」静默读成「不存在」）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-idspace-map.py` 原件（退场规则：**对拍通过后才**退场）；
//	· 入口 `checkIdspaceMapGateMain` 目前**无调用点**（M2 的刻意状态）。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `HERE = dirname(abspath(__file__))` → `REPO_ROOT = dirname(dirname(HERE))`
//	    （**脚本位置**）；本件**无 `__file__` 语境** ⇒ 不给 `--doc` 时从 **cwd** 上溯找含 `scripts/gates`
//	    的目录、找不到回落 `abspath(cwd)`。落在同一真仓、cwd=仓根（或 cwd 在仓内任意层）时两者
//	    读数逐字相同（已对拍）；「脚本位置 ≠ cwd 上溯根」时路径推导面不同 ⇒ **不属逐字对齐承诺**，
//	    对拍一律 cwd=脚本所在树根 / 显式传 `--doc`。
//	    **旋钮缺省值 vs py 默认推导逐字对比**：py
//	      `DEFAULT_DOC = os.path.join(REPO_ROOT, "..", "Zerg-内部文档", "项目文档", "v2.5.12",
//	                                  "登记-号段映射-v2.5.12-20260924.md")`
//	    ⟷ 本件 `idmDefaultDoc()`：`root + "/../Zerg-内部文档/项目文档/v2.5.12/" + 同件名` —— 表达式逐字
//	    同构（**都用字符串拼接、都不做路径规范化**，故中间那段 `..` 原样留在打印串里，与 py 一致；
//	    ✗ 不用 filepath.Join，它会吃掉 `..` ⇒ 打印面就差一格）。
//	(b) **自指检查（先真判）**：① 本件**不在**自己扫描面内 —— py 只 `open()` **一件仓外登记件**
//	    （`judge(doc_path)` 全流程零目录遍历、零 glob），本门**永不自扫本件**（量化依据：judge 里
//	    `os.path.isfile` 1 处、`open` 1 处，皆作用于调用方给定的单件路径）。② 本件**落在**「他门」
//	    扫描面内（`check-hardcoded-private-paths` 的 SCAN_DIRS 含 `core/`；`check-error-kinds` 的
//	    R3 扫 `core/cmd/zerg/*.go`）⇒ 已按血证处置：正文**零私有绝对路径字面量**（盘根/家目录前缀
//	    一概不写；仓根与外部树一律运行时取）、**不写** `check-error-kinds` 的被扫用法形态（其错误
//	    构造调用原串此处**不复写**）、**不写**反引号包裹的门扫描标记原串。落仓前后已跑他门取证：
//	    `check-error-kinds` 与 `check-hardcoded-private-paths` 两门输出 sha16 落仓前后逐字相同。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse **长选项前缀缩写**（如 `--js`⇒`--json`、`--do`⇒`--doc`）本件不实现；`--json=x`
//	   等号形本件支持（与 py 同）。
//	② 本件**新增**旋钮 `--doc` 与 py 的 `--doc` **同名同义** ⇒ 无旋钮面差异；py 无 env 旋钮可用。
//	③ py `open(..., encoding="utf-8")` 遇非 UTF-8 字节抛 `UnicodeDecodeError`（进程 rc=1 + 回溯正文）；
//	   本件按「读不了 ⇒ rc=2 不给结论」处理 ⇒ 该角落 rc/stderr 两面不同（正常 UTF-8 登记件不受影响）。
//	④ py argparse 依 `shutil.get_terminal_size()` 折行（本机非 tty ⇒ 80 列）；本件按 80 列**逐字固化**。
//	⑤ `-h`/`--help` 正文逐字复刻（含 `optional arguments:` 段，即 py 运行在 argparse 旧版语义上）。
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
	"unicode/utf8"
)

const idmProg = "check-idspace-map.py"

// idmUsage —— argparse 80 列下的 usage 头两行（错误档与 `-h` 共用）。
const idmUsage = `usage: check-idspace-map.py [-h] [--doc DOC] [--self-test] [--list-rules]
                            [--json JSON]
`

// idmHelp —— `-h`/`--help` 全文（Python 3.9 argparse · 80 列 · 逐字固化）。
const idmHelp = idmUsage + `
optional arguments:
  -h, --help    show this help message and exit
  --doc DOC
  --self-test
  --list-rules
  --json JSON
`

const (
	idmHdrMap   = "| 记号 | 号线 | 真源件 | 映射到 | 同记号几处 | 依据（现读） |"
	idmHdrCross = "| 记号 | 两处（件 · 号线） | 判 | 登记理由（逐字引一句 · 内容锚） |"
	idmNoPrefix = "无 Q 号"
	idmDupOK    = "≥2：登记行 + 入账行"
	// idmDocLeaf —— 登记件件名（默认根下的相对尾段）。
	idmDocLeaf = "登记-号段映射-v2.5.12-20260924.md"
)

var (
	idmReID    = regexp.MustCompile(`^[A-Z]{1,3}-\d{1,3}$`)
	idmReMapTo = regexp.MustCompile(`^Q-\d{3}$`)
)

// idmJSONFields —— 本门机器读面字段表（唯一真源：印表与拒收同读这一处）。
var idmJSONFields = []string{"verdict", "rows", "lines", "cross", "reds"}

// ── 入口 ─────────────────────────────────────────────────────────────
func checkIdspaceMapGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176 用法档：`--json <字段>` 里有不认的名 ⇒ rc=2（在 argparse **之前**）。
	if rc := idmRefuseUnknownJSONFields(stderr, argv); rc >= 0 {
		return rc
	}
	a, rc := idmParse(stdout, stderr, argv)
	if rc >= 0 {
		return rc
	}
	if a.ListRules {
		for i, r := range idmRules {
			fmt.Fprintf(stdout, "  %d. %s\n", i+1, r)
		}
		return 0
	}
	if a.SelfTest {
		return idmSelfTest(stdout)
	}

	v := idmJudge(a.Doc)
	if a.JSON != "" {
		fmt.Fprintln(stdout, idmJSONOut(a.JSON, v))
		return idmVerdictRC[v.verdict]
	}
	fmt.Fprintf(stdout, "── 号段映射一格（Q-073）· 判对象 = %s\n", a.Doc)
	if v.verdict == "noconc" {
		fmt.Fprintf(stdout, "   不给结论：%s\n", v.why)
		fmt.Fprint(stdout, "⇒ rc=2（缺件 / 表头缺 / 空转一律不当绿）\n")
		return 2
	}
	fmt.Fprintf(stdout, "   映射表 %d 行 · 号线 %d 条 · 跨线重名 %d 个 · §三 登记 %d 条\n",
		v.rows, v.lines, v.cross, v.crossReg)
	if len(v.reds) > 0 {
		fmt.Fprintf(stdout, "── 红 %d 条（逐条点名 · 不合并）──\n", len(v.reds))
		for _, r := range v.reds {
			fmt.Fprintf(stdout, "   ✗ %s :%d %s\n", r.rule, r.lineno, r.why)
		}
		fmt.Fprint(stdout, "⇒ rc=1：有红（同线重号未登记 / 形态越域 / 跨线同名未登记）\n")
		return 1
	}
	fmt.Fprint(stdout, "── 六格全过：K1 记号同线唯一 · K2/K3/K4 形态在域 · K5 跨线重名全登记 · K6 非空转\n")
	fmt.Fprint(stdout, "⇒ rc=0\n")
	return 0
}

// ── Q-176：`--json <字段>` 不认的名 ⇒ rc=2 ＋ 印可选字段表 ──────────────
func idmRefuseUnknownJSONFields(stderr io.Writer, argv []string) int {
	var want []string
	for i, x := range argv {
		if x == "--json" {
			v := ""
			if i+1 < len(argv) {
				v = argv[i+1]
			}
			want = append(want, strings.Split(v, ",")...)
		} else if strings.HasPrefix(x, "--json=") {
			want = append(want, strings.Split(x[len("--json="):], ",")...)
		}
	}
	fields := map[string]bool{}
	for _, f := range idmJSONFields {
		fields[f] = true
	}
	var bad []string
	for _, f := range want {
		f = strings.TrimSpace(f)
		if f != "" && !fields[f] {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return -1
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", idmJoinTick(bad))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", idmJoinTick(idmJSONFields))
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开\n")
	return 2
}

func idmJoinTick(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, "`"+x+"`")
	}
	return strings.Join(parts, " · ")
}

// ── argparse 子集 ───────────────────────────────────────────────────
type idmArgs struct {
	Doc       string
	HasDoc    bool
	SelfTest  bool
	ListRules bool
	JSON      string
}

// idmParse —— 返回 (args, rc)。rc>=0 ⇒ 已出结论（帮助/用法错）；rc<0 ⇒ 继续。
func idmParse(stdout, stderr io.Writer, argv []string) (idmArgs, int) {
	a := idmArgs{}
	var positional, unknown []string
	afterDD := false
	i := 0
	for i < len(argv) {
		tok := argv[i]
		i++
		if afterDD {
			positional = append(positional, tok)
			continue
		}
		if tok == "--" {
			afterDD = true
			continue
		}
		if tok == "-h" || tok == "--help" {
			fmt.Fprint(stdout, idmHelp)
			return a, 0
		}
		if strings.HasPrefix(tok, "--") {
			name, val, hasEq := tok, "", false
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				name, val, hasEq = tok[:eq], tok[eq+1:], true
			}
			needVal := func() (string, bool) {
				if hasEq {
					return val, true
				}
				if i < len(argv) && !strings.HasPrefix(argv[i], "-") {
					v := argv[i]
					i++
					return v, true
				}
				return "", false
			}
			switch name {
			case "--doc":
				v, ok := needVal()
				if !ok {
					return a, idmArgErr(stderr, "argument --doc: expected one argument")
				}
				a.Doc, a.HasDoc = v, true
			case "--json":
				v, ok := needVal()
				if !ok {
					return a, idmArgErr(stderr, "argument --json: expected one argument")
				}
				a.JSON = v
			case "--self-test":
				if hasEq {
					return a, idmArgErr(stderr, fmt.Sprintf("argument --self-test: ignored explicit argument '%s'", val))
				}
				a.SelfTest = true
			case "--list-rules":
				if hasEq {
					return a, idmArgErr(stderr, fmt.Sprintf("argument --list-rules: ignored explicit argument '%s'", val))
				}
				a.ListRules = true
			default:
				unknown = append(unknown, tok)
			}
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			unknown = append(unknown, tok)
			continue
		}
		positional = append(positional, tok)
	}
	if len(unknown) > 0 || len(positional) > 0 {
		bad := append(append([]string{}, unknown...), positional...)
		return a, idmArgErr(stderr, "unrecognized arguments: "+strings.Join(bad, " "))
	}
	if !a.HasDoc {
		a.Doc = idmDefaultDoc()
	}
	return a, -1
}

// idmArgErr —— argparse 错误档（usage + `prog: error: msg`）⇒ rc=2。
func idmArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, idmUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", idmProg, msg)
	return 2
}

// ── 默认根 / 登记件路径（声明 (a)）────────────────────────────────────
// idmDefaultDoc —— py 用 `__file__` 上溯两层；本件无此语境 ⇒ 从 cwd 上溯找含 `scripts/gates`
// 的目录，找不到回落 abspath(cwd)。拼串**不做规范化**（`..` 原样留在串里，与 py 逐字同）。
func idmDefaultDoc() string {
	root := idmDefaultRoot()
	return root + "/../Zerg-内部文档/项目文档/v2.5.12/" + idmDocLeaf
}

func idmDefaultRoot() string {
	start, err := os.Getwd()
	if err != nil {
		return "."
	}
	d, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	cur := d
	for {
		if idmIsDir(filepath.Join(cur, "scripts", "gates")) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return d
}

// ── 判据（逐字复刻 py judge()）────────────────────────────────────────
type idmRed struct {
	lineno int
	rule   string
	why    string
}

type idmVerdict struct {
	verdict  string // ok/red/noconc
	why      string // noconc 专用
	rows     int
	lines    int
	cross    int
	crossReg int
	reds     []idmRed
}

var idmVerdictRC = map[string]int{"ok": 0, "red": 1, "noconc": 2}

type idmRow struct {
	lineno int
	cells  []string // nil ⇒ 坏行（列数不足）
	bad    bool
}

func idmCell(s string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "`"))
}

// idmParseTable —— py parse_table(lines, header, width)。ok=false ⇔ 表头找不到。
func idmParseTable(lines []string, header string, width int) (int, []idmRow, bool) {
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == header {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, nil, false
	}
	rows := []idmRow{}
	for i := start + 1; i < len(lines); i++ {
		ln := lines[i]
		if !strings.HasPrefix(strings.TrimLeft(ln, " \t\n\r\v\f"), "|") {
			break
		}
		body := strings.Trim(ln, " \t\n\r\v\f")
		body = strings.Trim(body, "|")
		cells := strings.Split(body, "|")
		sep := true
		for _, c := range cells {
			if !idmSepCell(c) {
				sep = false
				break
			}
		}
		if sep {
			continue
		}
		if len(cells) < width {
			rows = append(rows, idmRow{lineno: i + 1, bad: true})
			continue
		}
		out := make([]string, width)
		for j := 0; j < width; j++ {
			out[j] = idmCell(cells[j])
		}
		rows = append(rows, idmRow{lineno: i + 1, cells: out})
	}
	return start + 1, rows, true
}

// idmSepCell —— py `set(c.strip()) <= set("-: ")`（空串也判真）。
func idmSepCell(s string) bool {
	for _, r := range strings.TrimSpace(s) {
		if r != '-' && r != ':' && r != ' ' {
			return false
		}
	}
	return true
}

func idmJudge(docPath string) idmVerdict {
	if docPath == "" || !idmIsFile(docPath) {
		return idmVerdict{verdict: "noconc", why: fmt.Sprintf("登记件不在盘：%s", docPath)}
	}
	lines, ok := idmReadLines(docPath)
	if !ok {
		// py 此处抛 UnicodeDecodeError（rc=1）；本件按「不给结论」处理（见未覆盖面③）。
		return idmVerdict{verdict: "noconc", why: fmt.Sprintf("登记件不在盘：%s", docPath)}
	}
	_, mapRows, found := idmParseTable(lines, idmHdrMap, 6)
	if !found {
		return idmVerdict{verdict: "noconc", why: "映射表表头逐字句找不到"}
	}
	if len(mapRows) == 0 {
		return idmVerdict{verdict: "noconc", why: "映射表 0 行 ⇒ 空转（不当绿）"}
	}
	_, crossRows, found2 := idmParseTable(lines, idmHdrCross, 4)
	if !found2 {
		return idmVerdict{verdict: "noconc", why: "同号两义登记表表头逐字句找不到"}
	}

	reds := []idmRed{}
	seen := map[string]int{}
	byID := map[string]map[string]bool{}
	for _, r := range mapRows {
		if r.bad {
			reds = append(reds, idmRed{lineno: r.lineno, rule: "K0", why: "列数不足 6（行不完整）"})
			continue
		}
		ident, line, mapto, howmany := r.cells[0], r.cells[1], r.cells[3], r.cells[4]
		if !idmReID.MatchString(ident) {
			reds = append(reds, idmRed{r.lineno, "K2", fmt.Sprintf("记号形态越域：%s", idmRepr(ident))})
		}
		if !(idmReMapTo.MatchString(mapto) || strings.HasPrefix(mapto, idmNoPrefix)) {
			reds = append(reds, idmRed{r.lineno, "K3", fmt.Sprintf("「映射到」越域：%s", idmRepr(mapto))})
		}
		if howmany != "1" && howmany != idmDupOK {
			reds = append(reds, idmRed{r.lineno, "K4", fmt.Sprintf("「同记号几处」越域：%s", idmRepr(howmany))})
		}
		key := line + "\x00" + ident
		if first, dup := seen[key]; dup && howmany != idmDupOK {
			reds = append(reds, idmRed{r.lineno, "K1", fmt.Sprintf(
				"同线重号（%s · %s）—— 首次在 :%d，且本行没写登记形态 %s",
				line, ident, first, idmRepr(idmDupOK))})
		}
		if _, ok := seen[key]; !ok {
			seen[key] = r.lineno
		}
		if byID[ident] == nil {
			byID[ident] = map[string]bool{}
		}
		byID[ident][line] = true
	}

	crossIDs := map[string]bool{}
	for _, r := range crossRows {
		if !r.bad && len(r.cells) > 0 {
			crossIDs[r.cells[0]] = true
		}
	}

	crossN := 0
	idents := make([]string, 0, len(byID))
	for k := range byID {
		idents = append(idents, k)
	}
	sort.Strings(idents)
	for _, ident := range idents {
		lset := byID[ident]
		if len(lset) >= 2 {
			crossN++
			if !crossIDs[ident] {
				ls := make([]string, 0, len(lset))
				for k := range lset {
					ls = append(ls, k)
				}
				sort.Strings(ls)
				reds = append(reds, idmRed{0, "K5", fmt.Sprintf(
					"跨线同名未登记：%s 住 %d 条号线（%s）却不在 §三 表里",
					ident, len(lset), strings.Join(ls, " · "))})
			}
		}
	}

	lineSet := map[string]bool{}
	for _, r := range mapRows {
		if !r.bad {
			lineSet[r.cells[1]] = true
		}
	}
	verdict := "ok"
	if len(reds) > 0 {
		verdict = "red"
	}
	return idmVerdict{verdict: verdict, rows: len(mapRows), lines: len(lineSet),
		cross: crossN, crossReg: len(crossIDs), reds: reds}
}

// ── 自检（正/负成对 · 合成夹具 · 不碰真目标）────────────────────────────
func idmSelfTest(stdout io.Writer) int {
	dir, err := os.MkdirTemp("", "zerg-idspace-")
	if err != nil {
		dir = os.TempDir()
	}
	defer os.RemoveAll(dir)

	goodMap := [][]string{
		{"`Q-073`", "`Q`", "缺口总账-20260923.md", "`Q-073`", "1", "§一.7 结转行"},
		{"`N-05`", "`N`", "缺口总账-20260923.md", "`Q-121`", "1", "§一.4 逐字"},
		{"`N-06`", "`N`", "缺口总账-20260923.md", "无 Q 号（原记号保留 · 转实）", idmDupOK, "§附录三十二"},
		{"`G-10`", "`G①`", "任务清单-缺口总账-v2.5.12-20260924.md", "无 Q 号（`C-02` 线）", "1", "§一 第二列"},
		{"`G-10`", "`G②`", "缺口总账-20260923.md", "`Q-110`", "1", "§一.7 第三列"},
		{"`T-2`", "`T②`", "任务清单-本版全做-v2.5.12-20260924.md", "无 Q 号（与 `G-05` 同条）", "1", "§一 `G-05` 行"},
	}
	goodCross := [][]string{
		{"`G-10`", "两件（`G①`／`G②`）", "**两义** ✗", "`G①` → `C-02` · `G②` → `Q-110`"},
	}

	var okList, failList []idmAsrt
	chk := func(name, want, got, note string) {
		pass := want == got
		a := idmAsrt{name: name, want: want, got: got, note: note}
		if pass {
			okList = append(okList, a)
		} else {
			failList = append(failList, a)
		}
	}
	bs := func(b bool) string {
		if b {
			return "True"
		}
		return "False"
	}

	// ① 好夹具
	doc := idmFixture(dir, goodMap, goodCross)
	v := idmJudge(doc)
	chk("① 好夹具 ⇒ rc=0", "ok", v.verdict, fmt.Sprintf("红 %d 条", len(v.reds)))
	// ② 同线重号（未登记形态）
	bad := append(append([][]string{}, goodMap...),
		[]string{"`G-10`", "`G①`", "另一件", "无 Q 号（旧线）", "1", "另一行"})
	v2 := idmJudge(idmFixture(dir, bad, goodCross))
	chk("② 负控：同线重号 ⇒ rc=1", "red", v2.verdict, "")
	chk("②b 负控点名到「件:行 + K1」", "True",
		bs(idmHasRed(v2.reds, "K1", true)), idmReprReds1(v2.reds))
	// ③ 真话负控：同线重号但**本行**写了登记形态 ⇒ 回绿
	var3 := append(append([][]string{}, goodMap...),
		[]string{"`N-06`", "`N`", "同一件", "无 Q 号（原记号保留 · 转实）", idmDupOK, "§附录三十二 入账行"})
	v3 := idmJudge(idmFixture(dir, var3, goodCross))
	chk("③ 真话负控：写成登记形态 ⇒ 回绿", "ok", v3.verdict, "")
	// ④ 跨线同名未登记
	v4 := idmJudge(idmFixture(dir, goodMap, nil))
	chk("④ 负控：跨线同名未登记 ⇒ rc=1", "red", v4.verdict, "")
	chk("④b 负控点名 K5 + 记号", "True",
		bs(idmHasRedStr(v4.reds, "K5", "G-10")), idmReprReds1(v4.reds))
	// ⑤ 真话负控：写进 §三 ⇒ 回绿
	v5 := idmJudge(idmFixture(dir, goodMap, goodCross))
	chk("⑤ 真话负控：登记进 §三 ⇒ 回绿", "ok", v5.verdict, "")
	// ⑥ 记号形态越域
	v6 := idmJudge(idmFixture(dir, [][]string{{"`q73`", "`Q`", "件", "`Q-073`", "1", "x"}}, nil))
	chk("⑥ 负控：记号形态越域 ⇒ rc=1", "red", v6.verdict, "")
	chk("⑥b 点名 K2 且带原样值", "True", bs(idmHasRedStr(v6.reds, "K2", "q73")), "")
	// ⑦ 「映射到」越域
	v7 := idmJudge(idmFixture(dir, [][]string{{"`Q-073`", "`Q`", "件", "Q73", "1", "x"}}, nil))
	chk("⑦ 负控：「映射到」越域 ⇒ rc=1", "red", v7.verdict, "")
	chk("⑦b 点名 K3", "True", bs(idmHasRule(v7.reds, "K3")), "")
	// ⑧ 「同记号几处」越域
	v8 := idmJudge(idmFixture(dir, [][]string{{"`Q-073`", "`Q`", "件", "`Q-073`", "2", "x"}}, nil))
	chk("⑧ 负控：「同记号几处」越域 ⇒ rc=1", "red", v8.verdict, "")
	chk("⑧b 点名 K4", "True", bs(idmHasRule(v8.reds, "K4")), "")
	// ⑨ 表头逐字句找不到 ⇒ 不给结论
	p := filepath.Join(dir, "坏件.md")
	_ = os.WriteFile(p, []byte("# 坏件\n\n没有那两行表头\n"), 0o644)
	chk("⑨ 表头缺 ⇒ rc=2", "noconc", idmJudge(p).verdict, "")
	// ⑩ 映射表 0 行 ⇒ 不给结论
	p2 := filepath.Join(dir, "空表.md")
	_ = os.WriteFile(p2, []byte(idmHdrMap+"\n|---|---|---|---|---|---|\n\n"+idmHdrCross+"\n"), 0o644)
	chk("⑩ 映射表空 ⇒ rc=2", "noconc", idmJudge(p2).verdict, "")
	// ⑪ 缺件 ⇒ 不给结论
	chk("⑪ 登记件缺 ⇒ rc=2", "noconc", idmJudge(filepath.Join(dir, "不在盘.md")).verdict, "")
	// ⑫ 真目标登记件在场且规模合理（用本件的默认真目标）
	v12 := idmJudge(idmDefaultDoc())
	chk("⑫ 真目标：登记件在场 + 六格全过", "ok", v12.verdict,
		fmt.Sprintf("行 %d · 红 %d", idmRepRows(v12), len(v12.reds)))
	chk("⑫b 真目标映射表规模 ≥ 30 行", "True", bs(idmRepRows(v12) >= 30),
		fmt.Sprintf("行 %d", idmRepRows(v12)))
	chk("⑫c 真目标跨线重名 ≥ 8（G×7 + R-11）", "True", bs(v12.cross >= 8),
		fmt.Sprintf("cross %d", idmRepCross(v12)))

	for _, a := range okList {
		note := a.note
		if note == "" {
			note = a.got
		}
		fmt.Fprintf(stdout, "   ✓ %s（%s）\n", a.name, note)
	}
	for _, a := range failList {
		fmt.Fprintf(stdout, "   ✗ %s：期望 %s · 实得 %s（%s）\n", a.name, a.want, a.got, a.note)
	}
	concl := "全过"
	if len(failList) > 0 {
		concl = "有失败"
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n",
		concl, len(okList)+len(failList), len(failList))
	if len(failList) == 0 {
		return 0
	}
	return 1
}

type idmAsrt struct {
	name, want, got, note string
}

func idmHasRule(reds []idmRed, rule string) bool {
	for _, r := range reds {
		if r.rule == rule {
			return true
		}
	}
	return false
}

func idmHasRed(reds []idmRed, rule string, needLine bool) bool {
	for _, r := range reds {
		if r.rule == rule && (!needLine || r.lineno > 0) {
			return true
		}
	}
	return false
}

func idmHasRedStr(reds []idmRed, rule, sub string) bool {
	for _, r := range reds {
		if r.rule == rule && strings.Contains(r.why, sub) {
			return true
		}
	}
	return false
}

// idmReprReds1 —— py `"%r" % reds[:1]`：单元素 tuple 列表的 repr。
func idmReprReds1(reds []idmRed) string {
	if len(reds) == 0 {
		return "[]"
	}
	r := reds[0]
	return "[(" + strconv.Itoa(r.lineno) + ", " + idmRepr(r.rule) + ", " + idmRepr(r.why) + ")]"
}

// idmRepRows / idmRepCross —— py `rep.get("rows", -1)`（noconc 档无该键 ⇒ -1）。
func idmRepRows(v idmVerdict) int {
	if v.verdict == "noconc" {
		return -1
	}
	return v.rows
}

func idmRepCross(v idmVerdict) int {
	if v.verdict == "noconc" {
		return -1
	}
	return v.cross
}

// idmFixture —— py `_fixture(tmpdir, map_rows, cross_rows)` 逐字复刻。
func idmFixture(dir string, mapRows, crossRows [][]string) string {
	p := filepath.Join(dir, "登记件.md")
	body := []string{"# 夹具", "", "## 二 · 映射表", "", idmHdrMap, "|---|---|---|---|---|---|"}
	for _, r := range mapRows {
		body = append(body, "| "+strings.Join(r, " | ")+" |")
	}
	body = append(body, "", "> 口径（夹具）", "", "## 三 · 同号两义 / 跨线同名登记", "",
		idmHdrCross, "|---|---|---|---|")
	for _, r := range crossRows {
		body = append(body, "| "+strings.Join(r, " | ")+" |")
	}
	_ = os.WriteFile(p, []byte(strings.Join(body, "\n")+"\n"), 0o644)
	return p
}

// ── RULES / JSON ────────────────────────────────────────────────────
var idmRules = []string{
	"K1 记号在同一条号线上唯一（唯一白名单：逐字「" + idmDupOK + "」）",
	"K2 记号形态 ^[A-Z]{1,3}-\\d{1,3}$",
	"K3 「映射到」= ^Q-\\d{3}$ 或「" + idmNoPrefix + "…」的一句话",
	"K4 「同记号几处」= 1 或 逐字「" + idmDupOK + "」",
	"K5 跨 ≥2 条号线重名的记号必须在 §三 逐条登记（同号两义 ⇒ 红）",
	"K6 缺件 / 表头缺 / 映射表空 ⇒ rc=2（不给结论 · 不当绿）",
}

// idmJSONOut —— py：`{k: v for k,v in obj.items() if k in want}` + json.dumps(sort_keys=True)。
func idmJSONOut(jsonArg string, v idmVerdict) string {
	want := map[string]bool{}
	for _, k := range strings.Split(jsonArg, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			want[k] = true
		}
	}
	all := map[string]string{
		"verdict": idmJStr(v.verdict),
		"rows":    strconv.Itoa(v.rows),
		"lines":   strconv.Itoa(v.lines),
		"cross":   strconv.Itoa(v.cross),
		"reds":    strconv.Itoa(len(v.reds)),
	}
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if val, ok := all[k]; ok {
			parts = append(parts, idmJStr(k)+": "+val)
		}
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func idmJStr(s string) string {
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

// ── 工具 ────────────────────────────────────────────────────────────
func idmIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func idmIsDir(p string) bool {
	if p == "" {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// idmReadLines —— py `open(..., encoding="utf-8").read().splitlines()`（含通用换行 + 多边界）。
func idmReadLines(path string) ([]string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if !utf8.Valid(raw) {
		return nil, false
	}
	s := string(raw)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return idmSplitLines(s), true
}

func idmSplitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	rs := []rune(s)
	out := []string{}
	start := 0
	for i, r := range rs {
		if idmIsLineBoundary(r) {
			out = append(out, string(rs[start:i]))
			start = i + 1
		}
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

func idmIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\u0085', '\u2028', '\u2029':
		return true
	}
	return false
}

// idmRepr —— py `repr(str)`（引号择一、转义、可打印 Unicode 原样保留）。
func idmRepr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString("\\\\")
		case r < 128 && byte(r) == quote:
			b.WriteByte('\\')
			b.WriteByte(quote)
		case r == '\n':
			b.WriteString("\\n")
		case r == '\r':
			b.WriteString("\\r")
		case r == '\t':
			b.WriteString("\\t")
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, "\\x%02x", r)
		case r >= 0x80 && !unicode.IsPrint(r):
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}
