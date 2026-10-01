// check_approve_freeze.go —— 门禁件 `scripts/gates/check-approve-freeze.py` 的 **Go 真搬**（M2 · 路B448 · 批准档位族）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一件跑两态
//	（py 原件 ↔ 本入口），**stdout / stderr / 退码三面逐字相同**。
//
// 选它 / ✗ 不选 `check-approve-tier.py` 的理由（同族「批准档位族」内换件 · 父单口径「靠真跑子进程
// ⇒ 停手换同族纯静态件」）：
//
//	✓ 选 `check-approve-freeze.py`（335 行）：判据是**纯字符串 / 正则 / JSON 读**（读仓内一份 Go 件 +
//	  一份矩阵 JSON + 仓外文档仓两份 md），**零子进程 / 零计时读数 / 零网络 / 零外部工具 / 无写面**（只
//	  读消费，一个字节都不改）· 输出**定序**（帮助面行序 + 矩阵原序 + 台账 max 现算）· 退码 0/1/2 三档齐。
//	✗ 不选 `check-approve-tier.py`（原派件 · 460 行）：**判据靠真跑子进程** —— 它 `resolve_bin` 现场
//	  `go build` 一枚二进制、再 `subprocess.run` 跑 `approve ls/show/keygen` 取 T1/T2/T4/T6 读数 ⇒ 读数
//	  是「真跑 CLI」的产物，**无逐字可复现性**（且 `--root` 指向缺 `core` 的夹具时 `subprocess.run(cwd=…)`
//	  抛 `FileNotFoundError` ⇒ C 类无兜底 rc=1）⇒ 按父单口径**停手换同族纯静态件**（见回执「选件」一节）。
//	✗ 不选同族 `check-approve-ci.py` / `check-approve-closedset.py` / `check-approve-merge.py` /
//	  `check-approve-time.py`：均含 `subprocess`（真起子进程）⇒ 同族里**只有 tier 与 freeze** 是纯静态，
//	  tier 已由上一段排除。`check-approve-backfill.py`（683 行）同为纯静态但行数更大 ⇒ 取 freeze。
//
// ⚠ 本单边界（照实）：只落码、不接线 ——
//
//	✗ 不在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	✗ 不删 py 原件（退场规则：对拍通过后才退场 · 逐件显式点名）；
//	入口目前**无调用点**（M2 的刻意状态）；✗ 不碰登记面六面。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func checkApproveFreezeGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 全绿（漂移 0 行）/ 1 有红（漂移 ≥1 行）/ 2 不给结论（任一面取不到 / 自检不过 / 用法错）
//	  · 输出 —— stdout 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 出厂声明行（两条 · 每枚必写）：
//
//	(a) **默认根定位差异**：py `DEFAULT_ROOT = dirname(dirname(dirname(abspath(__file__))))`（脚本位置上溯两级）；
//	    Go 入口**无 `__file__` 语境** ⇒ 默认取**当前工作目录**（`cafrDefaultRoot`）。「脚本位置 ≠ cwd」时
//	    路径推导面不同 ⇒ **✗ 不声称逐字对齐**；对拍一律**显式传 `--root`**（乙环境另令 cwd==夹具根）。
//	(b) **自指检查**：本件**不落在**它自己扫描的面内 —— 本件只读 ① `core/cmd/zerg/family_approve.go`
//	    ② `core/cmd/zerg/testdata/cli-matrix.json` ③ `<Zerg-内部文档>/项目文档/v2.5.11/导出-命令面帮助-*.md`
//	    ④ 同目录 `缺口-命令面-20260922.md`；本件自身是 `check_approve_freeze.go`（既不在矩阵 cases 里，
//	    也不被任何一面的逐字串匹配）。四形态自查：① 件内**无**带真引号的仓内绝对路径原串（全文只有
//	    相对 `core/cmd/zerg/…` 串，读点是 `cafrPyJoin(repo, rel)`）；② 夹具/自检标记串（`导出-命令面帮助-`、
//	    九键名、`R-11`、`G-\p{Nd}\p{Nd}`）**均为运行期拼串 / 正则**，✗ 不以「会被扫到的形态」直写他门扫描面
//	    （九键名是 F1/F2 的**判据常量**，与 `family_approve.go` 现读一致，属**跨件口径**而非自指）；
//	    ③ self-test 无「魔法环境变量串」；④ 本件**未接线**、不在任何 `add_step` 步骤表内。
//	    另：本件 read-only，**零写面** ⇒ 即使被别门扫到也不改他门读数（见回执 (b) 一节）。
//
// 遍历 / 排序点（★ 逐处交代 · 保 py 原序）：
//
//	① 帮助导出面文件选取：py `glob.glob(dir + "导出-命令面帮助-*.md")` = **readdir 原序**（CPython `_glob1`
//	   用 `os.scandir`，**不排序**）→ 再 `sorted(..., key=os.path.getmtime)`（**稳定**）→ 取末位。
//	   Go 用 `os.Open` + `f.ReadDir(-1)`（**保 readdir 原序**，✗ 不用 `os.ReadDir`/`WalkDir` 那类**会排序**的读法）
//	   → 逐名过 `cafrGlobMatch` → `sort.SliceStable` 按 mtime（py 同款稳定序、同款 tie 破法）。见 `cafrHelpFiles`。
//	② `judge_matrix`：`fam` / `mustfail` 用**切片**保**矩阵原序**（✗ 零 Go map 迭代进输出）。输出只取**条数**与前
//	   must-fail 的 id 串（顺序 = 矩阵原序）。
//	③ `judge_ledger` / scan 打印的 G 编号：py `sorted({int(...) for ...})` ⇒ Go `sort.Ints`（输出只取 `max`）。
//	④ **输出行序决定者** = ①的 mtime 排序（决定读哪份帮助面）＋ `judge_help_faces` 的**帮助面行序**（readings 按
//	   行号升序、同行内按 `HELP_ROWS` 原序）；scan 的 ①②③④ 打印行是**固定序**，✗ 无 map 参与。
//
// 已对拍面（两环境 · 见回执「对拍全表」）：甲（真仓 + 显式传根 · py 走真仓原件绝对路径）· 乙（仓外夹具 +
// 不传根 · 两态同路径）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 帮助正文 / 用法错：py 用 `argparse.ArgumentParser` ⇒ `-h/--help` 打 argparse 帮助正文、未知旗标
//	   打 `usage…error: unrecognized arguments` —— 本入口按「未知旗标 ⇒ rc=2 · `-h/--help` ⇒ 打一行简短 usage」处理，
//	   **正文与 py 不同**（对拍一律不喂帮助 / 未知旗标）。
//	② 「不给根参数」时的**默认根定位**：见声明行 (a)。
//	③ 异常路径（py **C 类无最外层兜底** · ✗ 不改 py）：读不到的件（权限 / 是目录）/ 坏 JSON 矩阵 / 读不了的帮助面 ——
//	   py 抛 traceback（rc=1）；本入口 **fail-closed rc=2**（正文不同）⇒ 该格只保「不给结论」口径，**标注口径差**。
//	④ 正则语义：py `G-\d\d` 的 `\d` 是 **Unicode 十进制数字** ⇒ Go 手写对齐为 `G-\p{Nd}\p{Nd}`；但 `int(n[2:])`
//	   对**非 ASCII 数字**（如 `G-١٢`）py 能算、Go `strconv.Atoi` 失败 ⇒ 本入口**跳过该号**（真仓台账只含 ASCII 数字 ⇒ 未触发）。
//	⑤ `glob` 的完整 `fnmatch` 语义（`[`/`?`/前导点）本入口按**固定 pattern**（`导出-命令面帮助-` 前缀 + `.md` 后缀）
//	   等价判定；本件 pattern 是常量 ⇒ 等价（✗ 未实现通用 fnmatch）。
//	⑥ `tempfile.gettempdir()`（自检⑥）⟷ Go `os.TempDir()`：两者都读 `TMPDIR`；py 另做**可写性**校验，Go 不 ⇒ TMPDIR
//	   指向不可写目录时取值可能不同（对拍时 TMPDIR=/var/tmp 两态同）。
//	⑦ `os.path.abspath`（词法）⟷ `filepath.Abs`（`Clean`）在 `//a`（POSIX 保留前导双斜杠）一类极端串上不同 ⇒ 未触发。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const cafrVersion = "check-approve-freeze.py v1.0.0"

// cafrExpectedFields —— 九键（冻结面 · 与 py `EXPECTED_FIELDS` 逐字）。
var cafrExpectedFields = []string{"tool", "approver", "approved_at", "scope", "note", "sig_alg", "key_id", "state", "path"}

// cafrHelpRows —— 帮助导出面必须出现的两行（与 py `HELP_ROWS` 逐字 · 序即判据顺序）。
var cafrHelpRows = []string{"zerg approve ls", "zerg approve show"}

const (
	cafrDocsVer      = "v2.5.11"
	cafrGapDoc       = "缺口-命令面-20260922.md"
	cafrLedgerAnchor = "R-11"
	cafrGapMax       = 16 // `v2.5.11` 顶账 16 条（`G-01`–`G-16`）
)

// cafrGapNumRE —— py `re.compile(r"G-\d\d")`。★ `\d` 按**码点**展开为 `\p{Nd}`（Unicode 十进制数字）。
var cafrGapNumRE = regexp.MustCompile(`G-\p{Nd}\p{Nd}`)

var (
	cafrApproveFieldsRE = regexp.MustCompile(`var approveFields = \[\]string\{([^}]*)\}`)
	cafrQuotedRE        = regexp.MustCompile(`"([^"]+)"`)
	cafrLowerIdentRE    = regexp.MustCompile(`^[a-z_]+$`)
)

// cafrApproveGoRel / cafrMatrixRel —— py `os.path.join("core", …)` 的逐字同构（见 `cafrPyJoin`）。
var (
	cafrApproveGoRel = cafrPyJoin("core", "cmd", "zerg", "family_approve.go")
	cafrMatrixRel    = cafrPyJoin("core", "cmd", "zerg", "testdata", "cli-matrix.json")
)

// cafrPyJoin —— 逐字复刻 py `posixpath.join`（★ 三类语义差一次写全）：
//
//	① **空段**：`join('a','')` = `'a/'`（py 会补分隔符）· `join('','a')` = `'a'` —— `filepath.Join` 两态都得 `'a'`；
//	② **后段绝对丢前段**：`join('a','/b')` = `'/b'` —— `filepath.Join` 得 `'a/b'`；
//	③ **不归一 `..`**：`join('a','..','b')` = `'a/../b'` —— `filepath.Join` 归一成 `'b'`。
//
// ★ 本件所有 py `os.path.join(...)` 调用点改用本函数（`cafrDocsRootOf` / `cafrReadAll` / 相对串常量 / 自检⑥），
// 逐处见回执「Join 类逐处交代」。本件缺省串**不含 `..` 段** ⇒ 与 `filepath.Join` 在真仓/夹具两态实际同结果，
// 但为**语义对齐**仍全走本函数。
func cafrPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// cafrAbs —— py `os.path.abspath`（词法 · ✗ 不解析符号链接）。见未覆盖面⑦。
func cafrAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

// cafrDefaultRoot —— py `DEFAULT_ROOT`（脚本位置上溯两级）。Go 无 `__file__` ⇒ 取 cwd（声明行 (a)）。
func cafrDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// cafrSplitLines —— py `str.splitlines()`：按 \n / \r / \r\n / \v / \f / \x1c–\x1e / \x85 / \u2028 / \u2029 断行，**不留尾空行**。
func cafrSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, cur.String())
			cur.Reset()
		case '\r':
			out = append(out, cur.String())
			cur.Reset()
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
		default:
			cur.WriteRune(rs[i])
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func cafrStrSliceEq(a, b []string) bool {
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

// cafrParseFieldsFromGo —— py `parse_fields_from_go`：`None` ⟷ 返回 nil；pattern 命中但 0 串 ⇒ 返回空（非 nil）切片。
func cafrParseFieldsFromGo(src string) []string {
	m := cafrApproveFieldsRE.FindStringSubmatch(src)
	if m == nil {
		return nil
	}
	hits := cafrQuotedRE.FindAllStringSubmatch(m[1], -1)
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h[1])
	}
	return out
}

// cafrParseFieldCell —— py `parse_field_cell`：从一行里挑出字段表那一格（逗号分隔小写标识符 · ≥5 个）。
func cafrParseFieldCell(row string) []string {
	for _, cell := range strings.Split(row, "|") {
		cell = strings.TrimSpace(cell)
		cell = strings.Trim(cell, "`")
		if !strings.Contains(cell, ",") {
			continue
		}
		parts := strings.Split(cell, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		if len(parts) < 5 {
			continue
		}
		all := true
		for _, p := range parts {
			if !cafrLowerIdentRE.MatchString(p) {
				all = false
				break
			}
		}
		if all {
			return parts
		}
	}
	return nil
}

type cafrHelpReading struct {
	line   int
	name   string
	fields []string
}

// cafrJudgeHelpFaces —— py `judge_help_faces`：返回 (漂移行, 读数)。★ readings 序 = 帮助面**行序**（同行内按 `HELP_ROWS` 原序）。
func cafrJudgeHelpFaces(helpText string, expected []string) ([]string, []cafrHelpReading) {
	if expected == nil {
		expected = cafrExpectedFields
	}
	drifts := []string{}
	readings := []cafrHelpReading{}
	for i, ln := range cafrSplitLines(helpText) {
		lineno := i + 1
		for _, name := range cafrHelpRows {
			if strings.Contains(ln, "`"+name+"`") || strings.HasPrefix(ln, "| "+name+" |") {
				fields := cafrParseFieldCell(ln)
				if fields == nil {
					drifts = append(drifts, fmt.Sprintf("F1 漂移：第 %d 行（%s）里找不到字段表那一格", lineno, name))
					continue
				}
				readings = append(readings, cafrHelpReading{lineno, name, fields})
				if !cafrStrSliceEq(fields, expected) {
					drifts = append(drifts, fmt.Sprintf("F1 漂移：第 %d 行（%s）字段表 = %s ≠ 九键 %s",
						lineno, name, strings.Join(fields, ","), strings.Join(expected, ",")))
				}
			}
		}
	}
	for _, name := range cafrHelpRows {
		found := false
		for _, r := range readings {
			if r.name == name {
				found = true
				break
			}
		}
		if !found {
			drifts = append(drifts, fmt.Sprintf("F1 漂移：帮助导出面里没有 `%s` 那一行", name))
		}
	}
	return drifts, readings
}

// cafrJudgeFieldsFace —— py `judge_fields_face`。
func cafrJudgeFieldsFace(fields []string, expected []string) []string {
	if expected == nil {
		expected = cafrExpectedFields
	}
	if fields == nil {
		return []string{"F2 未取数：`approveFields` 解不出来"}
	}
	if !cafrStrSliceEq(fields, expected) {
		return []string{fmt.Sprintf("F2 漂移：`approveFields` = %s ≠ 九键 %s",
			strings.Join(fields, ","), strings.Join(expected, ","))}
	}
	return nil
}

// cafrCase —— 矩阵一条（`id` / `command` / `want_rc` 用 interface{} 保 py 的「缺键 ⇒ None」语义）。
type cafrCase struct {
	ID      interface{} `json:"id"`
	Command interface{} `json:"command"`
	WantRC  interface{} `json:"want_rc"`
}

func cafrStr(v interface{}) string {
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
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// cafrIsZero —— py `v != 0`：JSON 数字 0 ⇒ true（不是 must-fail）；缺键 / 非数 ⇒ false（是 must-fail）。
func cafrIsZero(v interface{}) bool {
	f, ok := v.(float64)
	return ok && f == 0
}

// cafrJudgeMatrix —— py `judge_matrix`：`approve` 族条数 == `want_rc != 0` 条数（现算 · 不写常量）。★ 保矩阵原序。
func cafrJudgeMatrix(cases []cafrCase) ([]string, [2]int) {
	fam := make([]cafrCase, 0)
	for _, c := range cases {
		if strings.HasPrefix(cafrStr(c.Command), "approve") {
			fam = append(fam, c)
		}
	}
	mustfail := make([]cafrCase, 0)
	for _, c := range fam {
		if !cafrIsZero(c.WantRC) {
			mustfail = append(mustfail, c)
		}
	}
	drifts := []string{}
	if len(fam) != len(mustfail) {
		missing := make([]string, 0)
		for _, c := range fam {
			if cafrIsZero(c.WantRC) {
				missing = append(missing, cafrStr(c.ID))
			}
		}
		drifts = append(drifts, fmt.Sprintf("F3 漂移：`approve` 族 %d 条里有 %d 条不是 must-fail（%s）",
			len(fam), len(fam)-len(mustfail), strings.Join(missing, " · ")))
	}
	return drifts, [2]int{len(fam), len(mustfail)}
}

// cafrGapNums —— py `sorted({int(n[2:]) for n in set(re.findall(...))})`。★ 非 ASCII 数字无法 Atoi ⇒ 跳过（未覆盖面④）。
func cafrGapNums(text string) []int {
	set := map[int]bool{}
	for _, n := range cafrGapNumRE.FindAllString(text, -1) {
		if v, err := strconv.Atoi(n[2:]); err == nil {
			set[v] = true
		}
	}
	out := make([]int, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// cafrJudgeLedger —— py `judge_ledger`。
func cafrJudgeLedger(text *string, anchor string, gapMax int) []string {
	if text == nil {
		return []string{"F4 未取数：台账读不到"}
	}
	drifts := []string{}
	if !strings.Contains(*text, anchor) {
		drifts = append(drifts, fmt.Sprintf("F4 漂移：台账里没有回指 `%s`", anchor))
	}
	nums := cafrGapNums(*text)
	if len(nums) > 0 {
		mx := nums[len(nums)-1]
		if mx > gapMax {
			drifts = append(drifts, fmt.Sprintf("F4 漂移：台账里出现了新编号（最大 G-%02d > G-%02d ⇒ 重开了新编号）", mx, gapMax))
		}
	}
	return drifts
}

// cafrDocsRootOf —— py `docs_root_of`（旋钮名照 py 同名旗标 + env；缺省推导逐字同构）。
func cafrDocsRootOf(repo, given string) string {
	if given != "" {
		return cafrAbs(given)
	}
	if env := os.Getenv("ZERG_DEVDOCS_ROOT"); env != "" {
		return cafrAbs(env)
	}
	sibling := os.Getenv("ZERG_DOCS_ALT")
	if sibling == "" {
		sibling = os.Getenv("ZERG_DOCS")
	}
	if sibling == "" {
		sibling = cafrPyJoin(filepath.Dir(repo), "Zerg-内部文档")
	}
	return cafrPyJoin(sibling, "项目文档", cafrDocsVer)
}

// cafrReadings —— 三面 + 台账（py `read_all` 的返回 dict）。
type cafrReadings struct {
	fields   []string
	cases    []cafrCase
	helpText string
	helpPath string
	gapText  string
	gapPath  string
}

// cafrGlobMatch —— `导出-命令面帮助-*.md` 的等价匹配（固定 pattern ⇒ 前缀 + 后缀，见未覆盖面⑤）。
func cafrGlobMatch(name string) bool {
	return strings.HasPrefix(name, "导出-命令面帮助-") && strings.HasSuffix(name, ".md")
}

// cafrHelpFiles —— py `glob.glob(dir + "导出-命令面帮助-*.md")` 的**保原序**读法：
// `os.Open` + `f.ReadDir(-1)`（readdir 原序 · ✗ 不用会排序的 `os.ReadDir`）→ 逐名 `cafrGlobMatch`。
func cafrHelpFiles(docsRoot string) ([]string, error) {
	f, err := os.Open(docsRoot)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ents, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0)
	for _, e := range ents {
		if cafrGlobMatch(e.Name()) {
			out = append(out, cafrPyJoin(docsRoot, e.Name()))
		}
	}
	return out, nil
}

// cafrReadAll —— py `read_all`：任一面取不到 ⇒ (nil, 理由)。
//
//	★ py **C 类无最外层兜底**：读不到的件 / 坏 JSON ⇒ py 抛 traceback（rc=1）；本入口 fail-closed rc=2（未覆盖面③）。
func cafrReadAll(repo, docsRoot string) (*cafrReadings, string) {
	goPath := cafrPyJoin(repo, cafrApproveGoRel)
	if _, err := os.Stat(goPath); err != nil {
		return nil, fmt.Sprintf("读不到 `%s`", cafrApproveGoRel)
	}
	data, err := os.ReadFile(goPath)
	if err != nil {
		return nil, fmt.Sprintf("读不了的件（%s）：%v", cafrApproveGoRel, err)
	}
	fields := cafrParseFieldsFromGo(string(data))
	if fields == nil {
		return nil, fmt.Sprintf("`approveFields` 在 %s 里解不出来", cafrApproveGoRel)
	}
	mxPath := cafrPyJoin(repo, cafrMatrixRel)
	if _, err := os.Stat(mxPath); err != nil {
		return nil, fmt.Sprintf("读不到 `%s`", cafrMatrixRel)
	}
	raw, err := os.ReadFile(mxPath)
	if err != nil {
		return nil, fmt.Sprintf("读不了的件（%s）：%v", cafrMatrixRel, err)
	}
	// py `json.load(f).get("cases", [])`：缺键 ⇒ `[]`（绿）；**键在场但 `null`/非数组** ⇒ py 迭代 None / 字典键
	// ⇒ `TypeError`/`AttributeError`（C 类无兜底 · py rc=1）⇒ 本入口 **fail-closed rc=2**（未覆盖面③）。
	var topObj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &topObj); err != nil {
		return nil, fmt.Sprintf("读不了的件（%s · 顶层非对象）：%v", cafrMatrixRel, err)
	}
	var cases []cafrCase
	if casesRaw, present := topObj["cases"]; present && string(casesRaw) != "null" {
		if err := json.Unmarshal(casesRaw, &cases); err != nil {
			return nil, fmt.Sprintf("读不了的件（%s · `cases` 非数组）：%v", cafrMatrixRel, err)
		}
	} else if present {
		return nil, fmt.Sprintf("读不了的件（%s · `cases` 是 null）", cafrMatrixRel)
	}
	if !cafrIsDir(docsRoot) {
		return nil, fmt.Sprintf("Zerg-内部文档 根不在（%s）", docsRoot)
	}
	helpFiles, err := cafrHelpFiles(docsRoot)
	if err != nil {
		return nil, fmt.Sprintf("Zerg-内部文档 根不在（%s）", docsRoot)
	}
	if len(helpFiles) == 0 {
		return nil, fmt.Sprintf("帮助导出面不在（%s/导出-命令面帮助-*.md）", docsRoot)
	}
	// py `sorted(..., key=os.path.getmtime)`（稳定）⇒ SliceStable 按 mtime；tie 破法同 py（保 readdir 原序）。
	type afFile struct {
		path  string
		mtime time.Time
	}
	files := make([]afFile, 0, len(helpFiles))
	for _, p := range helpFiles {
		st, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Sprintf("读不了的件（%s）：%v", p, err)
		}
		files = append(files, afFile{p, st.ModTime()})
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].mtime.Before(files[j].mtime) })
	helpPath := files[len(files)-1].path
	hb, err := os.ReadFile(helpPath)
	if err != nil {
		return nil, fmt.Sprintf("读不了的件（%s）：%v", helpPath, err)
	}
	gapPath := cafrPyJoin(docsRoot, cafrGapDoc)
	if _, err := os.Stat(gapPath); err != nil {
		return nil, fmt.Sprintf("台账不在（%s）", gapPath)
	}
	gb, err := os.ReadFile(gapPath)
	if err != nil {
		return nil, fmt.Sprintf("读不了的件（%s）：%v", gapPath, err)
	}
	return &cafrReadings{
		fields:   fields,
		cases:    cases,
		helpText: string(hb),
		helpPath: helpPath,
		gapText:  string(gb),
		gapPath:  gapPath,
	}, ""
}

// cafrIsDir —— py `os.path.isdir`（**跟随符号链接** ⇒ Go 用 `os.Stat`，✗ 不用 `entry.IsDir()` 的 lstat 语义）。
func cafrIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// cafrJudgeAll —— py `judge_all`。
func cafrJudgeAll(r *cafrReadings) ([]string, []cafrHelpReading, [2]int) {
	drifts := []string{}
	hd, readings := cafrJudgeHelpFaces(r.helpText, nil)
	drifts = append(drifts, hd...)
	drifts = append(drifts, cafrJudgeFieldsFace(r.fields, nil)...)
	md, mf := cafrJudgeMatrix(r.cases)
	drifts = append(drifts, md...)
	gt := r.gapText
	drifts = append(drifts, cafrJudgeLedger(&gt, cafrLedgerAnchor, cafrGapMax)...)
	return drifts, readings, mf
}

// cafrFakeHelp —— py `FAKE_HELP`。
func cafrFakeHelp() string {
	joined := strings.Join(cafrExpectedFields, ",")
	return "# 导出（夹具）\n" +
		fmt.Sprintf("| `zerg approve ls` | 列人签批准件 | %s | 本机 | `host` |\n", joined) +
		fmt.Sprintf("| `zerg approve show` | 看一枚批准件 | %s | 本机 | `host` |\n", joined)
}

// cafrSelfTest —— py `self_test`（6 格 · 先自证会红）。返 false ⟷ py 返 None / False（main 都据 `not` 判 rc=2）。
func cafrSelfTest(stdout io.Writer, repo, docsRoot string) bool {
	ok, bad := 0, 0
	r, why := cafrReadAll(repo, docsRoot)
	if r == nil {
		fmt.Fprintf(stdout, "⛔ 自检① 取不到真读数：%s ⇒ 不给结论\n", why)
		return false
	}
	drifts, _, mf := cafrJudgeAll(r)
	if len(drifts) == 0 {
		fmt.Fprintf(stdout, "   ✓ ① 正控：三面 + 台账现读一致 ⇒ 漂移 0 行（帮助面 %s · 矩阵 approve 族 %d 条 / must-fail %d 条）\n",
			filepath.Base(r.helpPath), mf[0], mf[1])
		ok++
	} else {
		fmt.Fprintf(stdout, "   ✗ ① 正控失败：现读就有漂移：%s\n", strings.Join(drifts, "；"))
		bad++
	}
	// ② 负控：帮助面字段表少一格 ⇒ 漂移行 ≥1
	wrong := strings.Join(cafrExpectedFields[:len(cafrExpectedFields)-1], ",")
	d2, _ := cafrJudgeHelpFaces(strings.ReplaceAll(cafrFakeHelp(), strings.Join(cafrExpectedFields, ","), wrong), nil)
	if len(d2) > 0 {
		fmt.Fprintf(stdout, "   ✓ ② 负控：帮助面字段表少一格 ⇒ 出漂移行（%d 条）\n", len(d2))
		ok++
	} else {
		fmt.Fprintln(stdout, "   ✗ ② 负控失败：字段表不一致还判绿")
		bad++
	}
	// ③ 负控：矩阵里混一条 approve 族非 must-fail ⇒ 红
	mixed := append(append([]cafrCase{}, r.cases...), cafrCase{ID: "合成#漏 must-fail", Command: "approve ls", WantRC: float64(0)})
	if d3, _ := cafrJudgeMatrix(mixed); len(d3) > 0 {
		fmt.Fprintln(stdout, "   ✓ ③ 负控：矩阵里混一条 `approve` 族 want_rc=0 ⇒ 判红")
		ok++
	} else {
		fmt.Fprintln(stdout, "   ✗ ③ 负控失败：漏了一条 must-fail 还判绿")
		bad++
	}
	// ④ 负控：台账没有回指 ⇒ 红
	if t := "§一 · 与 `R-12` 同族（没有回指那一枚）\n"; len(cafrJudgeLedger(&t, cafrLedgerAnchor, cafrGapMax)) > 0 {
		fmt.Fprintf(stdout, "   ✓ ④ 负控：台账缺回指 `%s` ⇒ 判红\n", cafrLedgerAnchor)
		ok++
	} else {
		fmt.Fprintln(stdout, "   ✗ ④ 负控失败：缺回指还判绿")
		bad++
	}
	// ⑤ 负控：台账里重开新编号 ⇒ 红
	if t := "回指 R-11 · 另开 G-17 一条\n"; len(cafrJudgeLedger(&t, cafrLedgerAnchor, cafrGapMax)) > 0 {
		fmt.Fprintln(stdout, "   ✓ ⑤ 负控：台账里重开新编号（G-17）⇒ 判红")
		ok++
	} else {
		fmt.Fprintln(stdout, "   ✗ ⑤ 负控失败：重开了新编号还判绿")
		bad++
	}
	// ⑥ 负控：面缺 ⇒ 未取数（rc=2 那条口径）
	nodocs := cafrPyJoin(os.TempDir(), "zerg-approve-freeze-nodocs", "Zerg-内部文档", "项目文档", cafrDocsVer)
	if missing, why2 := cafrReadAll(repo, nodocs); missing == nil && strings.Contains(why2, "Zerg-内部文档") {
		fmt.Fprintf(stdout, "   ✓ ⑥ 负控：Zerg-内部文档 根缺 ⇒ 未取数（%s）⇒ 不许当绿\n", why2)
		ok++
	} else {
		fmt.Fprintln(stdout, "   ✗ ⑥ 负控失败：面缺却没判「未取数」")
		bad++
	}
	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（真目标只读 · 未改任何字节）\n", ok, bad)
	return bad == 0
}

// cafrScan —— py `scan`。
func cafrScan(stdout io.Writer, repo, docsRoot string) int {
	r, why := cafrReadAll(repo, docsRoot)
	if r == nil {
		fmt.Fprintf(stdout, "⛔ 未取数：%s ⇒ **不出结论**（rc=2）\n", why)
		return 2
	}
	drifts, readings, mf := cafrJudgeAll(r)
	fmt.Fprintln(stdout, "── 三面读数（每处带口径：件 / 行号 / 现算值）")
	for _, rd := range readings {
		fmt.Fprintf(stdout, "   ① 帮助导出面：%s 第 %d 行 · `%s` · 字段 = %s\n",
			filepath.Base(r.helpPath), rd.line, rd.name, strings.Join(rd.fields, ","))
	}
	fmt.Fprintf(stdout, "   ② 件：%s · `approveFields` = %s\n", cafrApproveGoRel, strings.Join(r.fields, ","))
	fmt.Fprintf(stdout, "   ③ 矩阵：%s · `approve` 族 %d 条 · want_rc!=0 %d 条\n", cafrMatrixRel, mf[0], mf[1])
	nums := cafrGapNums(r.gapText)
	gapMaxSeen := 0
	if len(nums) > 0 {
		gapMaxSeen = nums[len(nums)-1]
	}
	back := "无"
	if strings.Contains(r.gapText, cafrLedgerAnchor) {
		back = "有"
	}
	fmt.Fprintf(stdout, "   ④ 台账：%s · 回指 `%s` = %s · G 编号上限 = G-%02d\n",
		filepath.Base(r.gapPath), cafrLedgerAnchor, back, gapMaxSeen)
	if len(drifts) > 0 {
		fmt.Fprintf(stdout, "⇒ 门禁红（rc=1）：漂移 %d 行\n", len(drifts))
		for _, d := range drifts {
			fmt.Fprintf(stdout, "   ✗ %s\n", d)
		}
		return 1
	}
	fmt.Fprintln(stdout, "⇒ 全绿（rc=0）：三面逐字一致 · 漂移 0 行（数字一律现算 · 未写任何常量）")
	return 0
}

// cafrRefuseJSONWithoutMachineFace —— py `refuse_json_without_machine_face`（缺口 `Q-176` 统一口）。
func cafrRefuseJSONWithoutMachineFace(argv []string, stderr io.Writer) *int {
	hit := make([]string, 0)
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, "`"+x+"`")
		}
	}
	if len(hit) == 0 {
		return nil
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(hit, " · "))
	// py `JSON_FIELDS` 为空 ⇒ 走「无（本门只出人读面）」那一支。
	fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	rc := 2
	return &rc
}

// checkApproveFreezeGateMain —— 入口（= py `main()`）。
func checkApproveFreezeGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := cafrRefuseJSONWithoutMachineFace(argv, stderr); rc != nil {
		return *rc
	}
	root := cafrDefaultRoot()
	docsRootFlag := ""
	selfTest := false
	noSelfTest := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprintln(stderr, "用法错（rc=2）：`--root` 缺值")
				return 2
			}
			i++
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--docs-root":
			if i+1 >= len(argv) {
				fmt.Fprintln(stderr, "用法错（rc=2）：`--docs-root` 缺值")
				return 2
			}
			i++
			docsRootFlag = argv[i]
		case strings.HasPrefix(a, "--docs-root="):
			docsRootFlag = a[len("--docs-root="):]
		case a == "--self-test":
			selfTest = true
		case a == "--no-self-test":
			noSelfTest = true
		case a == "-h" || a == "--help":
			// argparse 面不逐字对齐（见未覆盖面①）：打一行简短 usage。
			fmt.Fprintln(stdout, cafrVersion)
			fmt.Fprintln(stdout, "usage: check-approve-freeze.py [--root R] [--docs-root D] [--self-test] [--no-self-test]")
			return 0
		default:
			fmt.Fprintf(stderr, "用法错（rc=2）：未知旗标 %s\n", a)
			return 2
		}
	}
	repo := cafrAbs(root)
	docsRoot := cafrDocsRootOf(repo, docsRootFlag)
	fmt.Fprintln(stdout, cafrVersion)
	if !noSelfTest {
		if !cafrSelfTest(stdout, repo, docsRoot) {
			fmt.Fprintln(stdout, "⛔ 自检不过（读数取不到 或 有格不过）⇒ 不给结论（rc=2）")
			return 2
		}
	}
	if selfTest {
		return 0
	}
	fmt.Fprintf(stdout, "── 真目标：%s（Zerg-内部文档 根 %s）\n", repo, docsRoot)
	return cafrScan(stdout, repo, docsRoot)
}
