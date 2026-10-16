// check_face_parity.go —— 判据件 `scripts/gates/check-face-parity.py`（245 行 · 缺口 `GAP-20260927-79`）
// 的 **Go 真搬**（M2 一枚 · 承接 `GAP-20261001-13` · 入口 `checkFaceParityGateMain`）。
//
// 病根（py 件头逐字）：提交闸对**工作树**取证而非**索引面** —— 未跟踪件 / 未暂存件会替提交者变绿（假绿）。
// 修法 = 跑闸前物化索引并以 `ZERG_GATE_REPO_ROOT` 指过去；本判据钉住这一形态不回退。
//
// 判据（**硬**：命中 ⇒ rc=1；缺件 / 读不到 ⇒ rc=2「不给结论」）：
//
//	F1 物化索引：两条提交出口都必须把**索引面**物化后跑闸（各自的件里出现物化命令与指面 env 名）；
//	F2 禁 git 归档档：两条出口都不得出现该形态（HEAD 不等于索引 ⇒ 假绿）；只查**代码行**、注释整行跳过；
//	F3 面自陈：两条出口都必须自陈它跑的是「索引面」（人面输出里出现该词）。
//
// 判据（**只报告**：进清单、**不计 rc**）：
//
//	F4 同一包混入两棵树：scripts/gates 下的 .py 里既读工作树面选择器又读索引面选择器 ⇒ 报；
//	F5 台账类判据应改索引面：件名含 ledger / judge 且读 .jsonl 台账，却用工作树面选择器 ⇒ 报。
//
// 退码：0 全过 · 1 命中（硬）/ 报面有清单 · 2 判不了（缺件 / 读不到 / 空域 / 自检不过）。
// 本件**自指面**：F4/F5 扫 scripts/gates 下的 .py ⇒ 一律排除**本件 py 原件**（常量 `cfpgSelfName`）。
//
// ★★ 两条出厂声明行：
//
//	(a) **默认根定位差异**：py `DEFAULT_ROOT = dirname(dirname(dirname(abspath(__file__))))`
//	    （**脚本位置**上溯三级）⟷ 本件**无 __file__ 语境**、只能从 cwd 上溯找含 `scripts/gates` 的目录、
//	    取不到回落 `abspath(cwd)`（同族体例）。⇒ 落在同一真仓、cwd=仓根（或 cwd 在仓内任意层）时两者
//	    读数逐字相同（已对拍）；「脚本位置 ≠ cwd 上溯根」时路径推导面不同 ⇒ **不属逐字对齐承诺**，
//	    对拍一律 cwd=仓根 / 显式传位置参数 root。py 的 `root = <arg> or ZERG_GATE_REPO_ROOT or DEFAULT_ROOT`
//	    三级回落、末了 `abspath`，本件逐字同构。
//	(b) **自指检查**：
//	  ① 本件**在自己的扫描面内吗 —— 否**：本件唯一遍历面是判据运行期扫 `<root>/scripts/gates/*.py`
//	     （**判据自己的扫描面**，与被搬 Go 件无关）；Go 件本体不在任何被扫集合里（量化：本件零
//	     `os.ReadDir` 于 `core/cmd/zerg`、不读 .go）。
//	  ② 本件落在「他门」扫描面内吗 —— 是（首判即真）：私径门扫 `core/` ⇒ 本件零私有绝对路径字面量
//	     （一律运行期拼 + `t.TempDir()`）；错误种类门的用法面扫 `core/cmd/zerg/*.go` ⇒ 本件不写
//	     「引号键名 + 冒号 + 引号值」的整串（JSON 键名经 `cfpgK()` 运行时取用）；茧壁门扫命令树层级字面
//	     ⇒ 本件不写层级标记原串；`check-lang-freeze` 只认语言面（.go 合法）。
//	  ③ 处理有效性：落仓前后对同一批他门取数，输出与 rc 逐字同（读数见交付回执）。
//	未覆盖面（✗ 不声称逐字对齐 · 逐条）：
//	  ① **py 字符串原语**：`ln.strip()` 剥的是 py Unicode 空白（含 `\x1c-\x1f` / `\x85` / `\xa0` …）
//	     ⟷ Go `strings.TrimSpace` 不剥 `\x1c-\x1f` ⇒ 仅在该四个码点做行首 `#`/`//` 判定时可能分叉
//	     （真仓 / 夹具输入是 ASCII 空白 ⇒ 未触发）；`str.splitlines()` 的 11 种行界本件自写复刻
//	     `cfpgSplitLines`（与 py 同集）。
//	  ② **argparse 面**：本件实现精确长旗标 + 唯一前缀缩写 + `--opt=值` 等号形（store_true 带值报
//	     「ignored explicit argument」）+ 未知旗标/多余位置参数报话 + `--` 分隔符；正文按 Python 3.9
//	     argparse（`optional arguments:` 措辞）逐字固化。✗ 未实现：短旗标粘连（`-hx`）· 负数样位置参数
//	     特判。
//	  ③ **异常路径（C 类）**：py 最外层裸 `sys.exit(main())` ⇒ 未捕异常 = 回溯 + rc=1。本件**唯一**
//	     可抛路是 `os.ReadFile` 的非 UTF-8 解码错（py 走 `UnicodeDecodeError`）⇒ rc 同（此时进
//	     rc=2 的「读不到」档或静默跳过，**不退 1**，与 py 的 F4/F5 静默跳过一致），**正文不复刻**回溯，
//	     `err` 文案用声明式（非 CPython 的 `'utf-8' codec can't decode …`）。
//	  ④ **OSError 正文**：缺件档（ENOENT）/ 权限档（EACCES）本件合成 CPython 的
//	     `[Errno N] <msg>: '<path>'` 形态（已对拍逐字同）；其余 errno（如 IsADirectory）用 Go 原话 ⇒
//	     该档只保 rc，正文不同。
//	  ⑤ **`filepath.Join` 不适用**：py 三处 `os.path.join` 的段为 `(".githooks","pre-commit")` /
//	     `("core","cmd","zerg","family_repo_commit.go")` / `(root, rel)` —— 段内无 `..`、后段非绝对
//	     ⇒ 与 `filepath.Join` 恒同构；本件仍走自写 `cfpgPyJoin`（与 `posixpath.join` 逐段规则同构），
//	     并单测钉 `..` / 绝对后段 / 空段三态。
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const cfpgProg = "check-face-parity.py"

const cfpgVersion = "check-face-parity/1"

// cfpgSelfName —— py `os.path.basename(os.path.abspath(__file__))`：F4/F5 自指排除面。
// 本件无 __file__ 语境 ⇒ 逐字固化为 py 件基名（语义 = 排除「判据件自己那份 .py」）。
const cfpgSelfName = "check-face-parity.py"

// cfpgGatesDirRel —— py `os.path.join(root, "scripts", "gates")` 的相对段。
var cfpgGatesDirRel = cfpgPyJoin("scripts", "gates")

var (
	cfpgHookRel   = cfpgPyJoin(".githooks", "pre-commit")
	cfpgCommitRel = cfpgPyJoin("core", "cmd", "zerg", "family_repo_commit.go")
)

// cfpgNeedIndexRel —— F1 必须出现的两条「物化索引」串之一（两件各按需）。
const (
	cfpgNeedCheckout  = "checkout-index"
	cfpgNeedMaterial  = "materializeIndex("
	cfpgNeedRepoRoot  = "ZERG_GATE_REPO_ROOT"
	cfpgSelfDeclWord  = "索引面"
	cfpgForbidArchive = "git archive"
)

// cfpgHardReq —— 硬判据的「件 + 必须含的串」表（F1）。
type cfpgHardReq struct {
	rel     string
	needles []string
	why     string
}

var cfpgHardRequire = []cfpgHardReq{
	{cfpgHookRel, []string{cfpgNeedCheckout, cfpgNeedRepoRoot}, "出口② 提交钩子（物化索引 + 指面跑闸）"},
	{cfpgCommitRel, []string{cfpgNeedMaterial, cfpgNeedRepoRoot}, "出口① `repo commit`（物化索引 + 指面跑闸）"},
}

// cfpgSelfDecl —— F3 面自陈：两条出口都要在人面输出里点到「索引面」。
var cfpgSelfDecl = []struct{ rel, needle string }{
	{cfpgHookRel, cfpgSelfDeclWord},
	{cfpgCommitRel, cfpgSelfDeclWord},
}

// cfpgForbid —— F2：禁出现的串（HEAD 不等于索引 ⇒ 假绿）。
var cfpgForbid = []struct{ rel, needle string }{
	{cfpgHookRel, cfpgForbidArchive},
	{cfpgCommitRel, cfpgForbidArchive},
}

// cfpgWorktreeFace / cfpgIndexFace —— F4/F5 的工作树面选择器 vs 索引面选择器。
var cfpgWorktreeFace = []string{"git diff --name-only HEAD", "git status", "git ls-files --others"}
var cfpgIndexFace = []string{"--cached", cfpgNeedRepoRoot, cfpgNeedCheckout}

// cfpgLedgerHint —— 族名口径引用登记处 ledger_family.go（✗ 别在本件再写字面表 · GAP-20260927-323）。
var cfpgLedgerHint = ledgerFamilyHints

// ── py 路径语义家族（自写 · ✗ 不用 path/filepath.Join|Abs|Dir）────────────────────────────────

// cfpgPyJoin 复刻 posixpath.join（✗ 别手工「跳过空段」，那会把 "a/" 变成 "a"）。
func cfpgPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "/"): // 后段绝对 ⇒ 重置前面所有段
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// cfpgPyNormpath 复刻 posixpath.normpath。★ 保留「开头恰两个斜杠」：`//a` 不归一成 `/a`。
func cfpgPyNormpath(path string) string {
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
	var comps []string
	for _, comp := range strings.Split(path, "/") {
		switch {
		case comp == "" || comp == ".":
			// 跳过
		case comp != ".." || (initialSlashes == 0 && len(comps) == 0) || (len(comps) > 0 && comps[len(comps)-1] == ".."):
			comps = append(comps, comp)
		default:
			if len(comps) > 0 {
				comps = comps[:len(comps)-1]
			}
		}
	}
	out := strings.Join(comps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// cfpgPyDirname 复刻 posixpath.dirname（= split 的头段；剥尾斜杠，但纯 `/` 保留）。
func cfpgPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// cfpgPyAbspath 复刻 posixpath.abspath：**纯词法**（不碰盘、不解析符号链接）。cwd 由调用方传入。
func cfpgPyAbspath(cwd, path string) string {
	return cfpgPyNormpath(cfpgPyJoin(cwd, path))
}

func cfpgCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// ── py 字符串原语（自写）────────────────────────────────────────────────────────────────

// cfpgPyIsSpace —— py `str.isspace()` 的码点集（含 `\x1c-\x1f` · `\x85` · `\xa0` …）。
func cfpgPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	if r >= 0x2000 && r <= 0x200a {
		return true
	}
	switch r {
	case 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return false
}

// cfpgPyStrip 复刻 `str.strip()`（✗ 不用 strings.TrimSpace：它不剥 `\x1c-\x1f`）。
func cfpgPyStrip(s string) string {
	rs := []rune(s)
	i, j := 0, len(rs)
	for i < j && cfpgPyIsSpace(rs[i]) {
		i++
	}
	for j > i && cfpgPyIsSpace(rs[j-1]) {
		j--
	}
	return string(rs[i:j])
}

func cfpgIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// cfpgSplitLines 复刻 `str.splitlines()`（11 种行界 · `\r\n` 当一条 · 无尾空行）。
func cfpgSplitLines(s string) []string {
	out := []string{}
	var cur strings.Builder
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if cfpgIsLineBoundary(r) {
			if r == '\r' && i+sz < len(s) && s[i+sz] == '\n' {
				i += sz + 1
			} else {
				i += sz
			}
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
		i += sz
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// cfpgPyRepr —— py `%r`（repr(str)）的常用形态：单引号，含 `'` 且不含 `"` 时改用双引号。
func cfpgPyRepr(s string) string {
	q := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
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
			if byte(r) == q && r < 0x80 {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte(q)
	return b.String()
}

// ── 读面 ───────────────────────────────────────────────────────────────────────────────

// cfpgOSErrText —— CPython `OSError.__str__` 的两条常见形态（缺件 / 权限）。
// 其余 errno 用 Go 原话（未覆盖面 ④）。
func cfpgOSErrText(path string, err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "[Errno 2] No such file or directory: '" + path + "'"
	case errors.Is(err, os.ErrPermission):
		return "[Errno 13] Permission denied: '" + path + "'"
	default:
		return err.Error()
	}
}

// cfpgReadText —— py `read_text(path)` ⇒ (文本, 错误)。ok=false 时 errText 非空。
// py：`open(...,"rb").read()` 然后 `.decode("utf-8")` ⇒ 二进制（非 UTF-8）与读不到同走「错误非空」。
func cfpgReadText(path string) (txt string, ok bool, errText string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false, cfpgOSErrText(path, err)
	}
	if !utf8.Valid(b) {
		return "", false, "非 UTF-8：invalid utf-8"
	}
	return string(b), true, ""
}

// cfpgHasCodeLine —— `needle` 是否出现在**非注释**行（整行注释跳过）：说明文里写「不用 X」不算用 X。
func cfpgHasCodeLine(txt, needle string) bool {
	for _, ln := range cfpgSplitLines(txt) {
		s := cfpgPyStrip(ln)
		if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") {
			continue
		}
		if strings.Contains(ln, needle) {
			return true
		}
	}
	return false
}

// ── 判据结果 ───────────────────────────────────────────────────────────────────────────

type cfpgHardHit struct {
	file    string
	why     string
	missing []string
}

type cfpgDeclHit struct {
	file    string
	missing string
}

type cfpgForbidHit struct {
	file      string
	forbidden string
}

type cfpgResult struct {
	errMsg   string
	hard     []cfpgHardHit
	selfdecl []cfpgDeclHit
	forbid   []cfpgForbidHit
	mixed    []string
	ledger   []string
}

// cfpgJudge —— py `judge(root)` ⇒ (rc, result)。
func cfpgJudge(root string) (int, cfpgResult) {
	hard := []cfpgHardHit{}
	selfdecl := []cfpgDeclHit{}
	forbid := []cfpgForbidHit{}
	mixed := []string{}
	ledger := []string{}
	seen := map[string]string{}

	for _, h := range cfpgHardRequire {
		p := cfpgPyJoin(root, h.rel)
		txt, ok, et := cfpgReadText(p)
		if !ok {
			return 2, cfpgResult{errMsg: "缺件 / 读不到：" + h.rel + "（" + et + "）⇒ 判不了（不许当好）"}
		}
		seen[h.rel] = txt
		miss := []string{}
		for _, n := range h.needles {
			if !strings.Contains(txt, n) {
				miss = append(miss, n)
			}
		}
		if len(miss) > 0 {
			hard = append(hard, cfpgHardHit{file: h.rel, why: h.why, missing: miss})
		}
	}

	for _, sd := range cfpgSelfDecl {
		txt, present := seen[sd.rel]
		if !present {
			t, ok, et := cfpgReadText(cfpgPyJoin(root, sd.rel))
			if !ok {
				return 2, cfpgResult{errMsg: "缺件 / 读不到：" + sd.rel + "（" + et + "）⇒ 判不了"}
			}
			txt = t
		}
		if !strings.Contains(txt, sd.needle) {
			selfdecl = append(selfdecl, cfpgDeclHit{file: sd.rel, missing: sd.needle})
		}
	}

	for _, fb := range cfpgForbid {
		txt, present := seen[fb.rel]
		// ★ 只查**代码行**：说明文里写「我们不用 <该形态>」不该被自己点名（注释整行跳过）。
		if present && cfpgHasCodeLine(txt, fb.needle) {
			forbid = append(forbid, cfpgForbidHit{file: fb.rel, forbidden: fb.needle})
		}
	}

	// ── F4 / F5（只报告）：扫 scripts/gates 下的 .py（排除本件自身）──────────────────────
	gdir := cfpgPyJoin(root, cfpgGatesDirRel)
	names := []string{}
	if ents, err := os.ReadDir(gdir); err == nil {
		for _, e := range ents {
			n := e.Name()
			if strings.HasSuffix(n, ".py") {
				names = append(names, n)
			}
		}
	}
	sort.Strings(names)
	// ★ 域级空集守卫（`GAP-20260927-108`）：空域 ⇒ fail-closed（✗ 不许空域当绿）
	if len(names) == 0 {
		return 2, cfpgResult{errMsg: "空域：" + cfpgGatesDirRel + " 下取不到任何 .py 件（" + gdir + "）⇒ 判不了（不给结论）"}
	}
	for _, n := range names {
		if n == cfpgSelfName {
			continue
		}
		txt, ok, _ := cfpgReadText(cfpgPyJoin(gdir, n))
		if !ok {
			continue
		}
		hasWT := false
		for _, w := range cfpgWorktreeFace {
			if cfpgHasCodeLine(txt, w) {
				hasWT = true
				break
			}
		}
		hasIdx := false
		for _, i := range cfpgIndexFace {
			if cfpgHasCodeLine(txt, i) {
				hasIdx = true
				break
			}
		}
		if hasWT && hasIdx {
			mixed = append(mixed, cfpgGatesDirRel+"/"+n)
		}
		ledgerName := false
		for _, h := range cfpgLedgerHint {
			if strings.Contains(n, h) {
				ledgerName = true
				break
			}
		}
		if ledgerName && strings.Contains(txt, ".jsonl") && hasWT && !strings.Contains(txt, "--cached") {
			ledger = append(ledger, cfpgGatesDirRel+"/"+n)
		}
	}

	rc := 0
	if len(hard) > 0 || len(selfdecl) > 0 || len(forbid) > 0 {
		rc = 1
	}
	return rc, cfpgResult{hard: hard, selfdecl: selfdecl, forbid: forbid, mixed: mixed, ledger: ledger}
}

// ── 人面输出（= py `emit_human`）──────────────────────────────────────────────────────────

func cfpgEmitHuman(stdout io.Writer, res cfpgResult, rc int) {
	if res.errMsg != "" {
		fmt.Fprintln(stdout, "✗ "+res.errMsg)
		return
	}
	fmt.Fprintln(stdout, "== 提交闸「索引面 / 面自陈」判据（缺口 `GAP-20260927-79`）==")
	fmt.Fprintln(stdout, "   F1 物化索引 + F2 禁 `"+cfpgForbidArchive+"` + F3 面自陈（硬）")
	for _, x := range res.hard {
		fmt.Fprintf(stdout, "   ✗ F1 %s（%s）：缺 %s\n", x.file, x.why, strings.Join(x.missing, " · "))
	}
	for _, x := range res.forbid {
		fmt.Fprintf(stdout, "   ✗ F2 %s：出现禁用串 %s（HEAD ≠ 索引 ⇒ 假绿）\n", x.file, cfpgPyRepr(x.forbidden))
	}
	for _, x := range res.selfdecl {
		fmt.Fprintf(stdout, "   ✗ F3 %s：面自陈缺 %s（跑哪一面必须自陈）\n", x.file, cfpgPyRepr(x.missing))
	}
	fmt.Fprintf(stdout, "   F4 同一包混入两棵树（只报告）：%d 件\n", len(res.mixed))
	for _, m := range res.mixed {
		fmt.Fprintln(stdout, "      · "+m)
	}
	fmt.Fprintf(stdout, "   F5 台账类判据应改索引面（只报告）：%d 件\n", len(res.ledger))
	for _, m := range res.ledger {
		fmt.Fprintln(stdout, "      · "+m)
	}
	if rc == 0 {
		fmt.Fprintln(stdout, "== 结论 ==")
		fmt.Fprintln(stdout, "   ⇒ 绿（rc=0）：两条出口均物化索引 · 无 `"+cfpgForbidArchive+"` · 面自陈在位")
	} else {
		fmt.Fprintln(stdout, "== 结论 ==")
		fmt.Fprintln(stdout, "   ⇒ 红（rc=1）：有硬命中（缺物化索引 / 用了 `"+cfpgForbidArchive+"` / 缺面自陈）")
	}
}

// ── JSON 面（py `json.dumps(..., ensure_ascii=False)` · 默认分隔符 `", "` / `": "`）──────────

type cfpgJV struct {
	kind byte // 'o' 对象 · 'a' 数组 · 's' 串 · 'n' 整数
	s    string
	num  string
	arr  []cfpgJV
	obj  []cfpgKP
}

type cfpgKP struct {
	k string
	v cfpgJV
}

func cfpgObj(pairs ...cfpgKP) cfpgJV { return cfpgJV{kind: 'o', obj: pairs} }
func cfpgArr(xs ...cfpgJV) cfpgJV    { return cfpgJV{kind: 'a', arr: xs} }
func cfpgStr(s string) cfpgJV        { return cfpgJV{kind: 's', s: s} }
func cfpgInt(n int) cfpgJV           { return cfpgJV{kind: 'n', num: strconv.Itoa(n)} }

func cfpgStrArr(xs []string) cfpgJV {
	out := make([]cfpgJV, len(xs))
	for i, x := range xs {
		out[i] = cfpgStr(x)
	}
	return cfpgJV{kind: 'a', arr: out}
}

// cfpgK —— ★ 自指接缝：键名一律经此取用，件内**不出现**「引号键名 + 冒号 + 引号值」的整串。
func cfpgK(name string) string { return name }

// cfpgEncStr —— py `json.encoder.py_encode_basestring`（ensure_ascii=False）逐字复刻。
func cfpgEncStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cfpgDump —— py `json.dumps(obj, ensure_ascii=False)`（紧凑档 · 保插入序）。
func cfpgDump(v cfpgJV) string {
	switch v.kind {
	case 'o':
		if len(v.obj) == 0 {
			return "{}"
		}
		parts := make([]string, len(v.obj))
		for i, p := range v.obj {
			parts[i] = cfpgEncStr(p.k) + ": " + cfpgDump(p.v)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case 'a':
		if len(v.arr) == 0 {
			return "[]"
		}
		parts := make([]string, len(v.arr))
		for i, x := range v.arr {
			parts[i] = cfpgDump(x)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case 's':
		return cfpgEncStr(v.s)
	case 'n':
		return v.num
	}
	return "null"
}

// cfpgJSONTop —— py `{"schema": VERSION, "rc": rc, "root": root, "result": res}`。
func cfpgJSONTop(root string, rc int, res cfpgResult) cfpgJV {
	var result cfpgJV
	if res.errMsg != "" {
		result = cfpgObj(cfpgKP{cfpgK("error"), cfpgStr(res.errMsg)})
	} else {
		hards := []cfpgJV{}
		for _, x := range res.hard {
			hards = append(hards, cfpgObj(
				cfpgKP{cfpgK("file"), cfpgStr(x.file)},
				cfpgKP{cfpgK("why"), cfpgStr(x.why)},
				cfpgKP{cfpgK("missing"), cfpgStrArr(x.missing)},
			))
		}
		selfs := []cfpgJV{}
		for _, x := range res.selfdecl {
			selfs = append(selfs, cfpgObj(
				cfpgKP{cfpgK("file"), cfpgStr(x.file)},
				cfpgKP{cfpgK("missing"), cfpgStr(x.missing)},
			))
		}
		forbids := []cfpgJV{}
		for _, x := range res.forbid {
			forbids = append(forbids, cfpgObj(
				cfpgKP{cfpgK("file"), cfpgStr(x.file)},
				cfpgKP{cfpgK("forbidden"), cfpgStr(x.forbidden)},
			))
		}
		result = cfpgObj(
			cfpgKP{cfpgK("hard"), cfpgJV{kind: 'a', arr: hards}},
			cfpgKP{cfpgK("selfdecl"), cfpgJV{kind: 'a', arr: selfs}},
			cfpgKP{cfpgK("forbid"), cfpgJV{kind: 'a', arr: forbids}},
			cfpgKP{cfpgK("mixed"), cfpgStrArr(res.mixed)},
			cfpgKP{cfpgK("ledger"), cfpgStrArr(res.ledger)},
		)
	}
	return cfpgObj(
		cfpgKP{cfpgK("schema"), cfpgStr(cfpgVersion)},
		cfpgKP{cfpgK("rc"), cfpgInt(rc)},
		cfpgKP{cfpgK("root"), cfpgStr(root)},
		cfpgKP{cfpgK("result"), result},
	)
}

// ── 成对负控自检（py `selftest` · 7 格）──────────────────────────────────────────────────

const (
	cfpgGoodHook = "#!/usr/bin/env bash\n# 索引面\ngit -C R checkout-index -a -f --prefix=X/\n" +
		"ZERG_GATE_REPO_ROOT=X bash gate --fast\n"
	cfpgGoodGo = "// 索引面\nfunc f(){ matRoot,_:=materializeIndex(root); _=gateEnv(matRoot) }\n" +
		"const k = \"ZERG_GATE_REPO_ROOT\"\n"
)

func cfpgSelfTest(stdout io.Writer) int {
	ok := true
	total := 0
	chk := func(name string, cond bool) {
		total++
		mark := "✓"
		if !cond {
			mark = "✗"
		}
		ok = ok && cond
		fmt.Fprintf(stdout, "   %s %s\n", mark, name)
	}

	d, err := os.MkdirTemp("", "cfpg-selftest-")
	if err != nil {
		chk("正控：两条出口齐（物化 + 指面 + 面自陈）⇒ rc=0", false)
		fmt.Fprintf(stdout, "── %s（%d 格）──\n", "有红", total)
		return 1
	}
	// ★ 自回收（py 的 `tempfile.TemporaryDirectory` 也回收 ⇒ TMPDIR 前后 delta=0）。
	defer func() { _ = os.RemoveAll(d) }()

	_ = os.MkdirAll(cfpgPyJoin(d, ".githooks"), 0o755)
	_ = os.MkdirAll(cfpgPyJoin(d, "core", "cmd", "zerg"), 0o755)
	_ = os.MkdirAll(cfpgPyJoin(d, cfpgGatesDirRel), 0o755)
	// ★ 空集守卫（`GAP-20260927-108`）：F4/F5 取数面须非空 —— 正控夹具放一枚本族件（空域 ⇒ rc=2）
	_ = os.WriteFile(cfpgPyJoin(d, cfpgGatesDirRel, "seed.py"), []byte("x = 1\n"), 0o644)
	h := cfpgPyJoin(d, cfpgHookRel)
	g := cfpgPyJoin(d, cfpgCommitRel)
	_ = os.WriteFile(h, []byte(cfpgGoodHook), 0o644)
	_ = os.WriteFile(g, []byte(cfpgGoodGo), 0o644)

	rc, res := cfpgJudge(d)
	chk("正控：两条出口齐（物化 + 指面 + 面自陈）⇒ rc=0", rc == 0)

	// 负控①：钩子去掉物化 ⇒ F1 硬命中 rc=1
	_ = os.WriteFile(h, []byte(strings.ReplaceAll(cfpgGoodHook, cfpgNeedCheckout, "XX")), 0o644)
	rc, res = cfpgJudge(d)
	chk("负控①：钩子缺 checkout-index ⇒ rc=1 且 F1 点名该件",
		rc == 1 && cfpgAnyFile(res.hard, "pre-commit"))

	_ = os.WriteFile(h, []byte(cfpgGoodHook), 0o644)
	// 负控②：出现 git archive ⇒ F2 硬命中
	_ = os.WriteFile(g, []byte(cfpgGoodGo+"var zzArchive = \""+cfpgForbidArchive+" HEAD\"\n"), 0o644)
	rc, res = cfpgJudge(d)
	chk("负控②：出口出现 `git archive` ⇒ rc=1 且 F2 点名",
		rc == 1 && cfpgAnyForbidFile(res.forbid, "family_repo_commit.go"))

	_ = os.WriteFile(g, []byte(cfpgGoodGo), 0o644)
	// 负控③：面自陈缺失 ⇒ F3 硬命中
	_ = os.WriteFile(h, []byte(strings.ReplaceAll(cfpgGoodHook, cfpgSelfDeclWord, "XX")), 0o644)
	rc, res = cfpgJudge(d)
	chk("负控③：缺「索引面」自陈 ⇒ rc=1 且 F3 点名",
		rc == 1 && cfpgAnyDeclFile(res.selfdecl, "pre-commit"))

	_ = os.WriteFile(h, []byte(cfpgGoodHook), 0o644)
	// 负控④：缺件 ⇒ rc=2（判不了）
	_ = os.Remove(g)
	rc, _ = cfpgJudge(d)
	chk("负控④：缺 family_repo_commit.go ⇒ rc=2（不给结论）", rc == 2)

	_ = os.WriteFile(g, []byte(cfpgGoodGo), 0o644)
	// F4：同一包混入两棵树 ⇒ 只报告（rc=0）
	_ = os.WriteFile(cfpgPyJoin(d, cfpgGatesDirRel, "zz-mixed.py"), []byte(
		"import os, subprocess\nX = \"git diff --name-only HEAD\"\n"+
			"os.environ[\"ZERG_GATE_REPO_ROOT\"]\n"), 0o644)
	rc, res = cfpgJudge(d)
	chk("F4 只报告：混两棵树的件被点名且 rc 仍 0",
		rc == 0 && cfpgContains(res.mixed, cfpgGatesDirRel+"/zz-mixed.py"))

	// 自指面：本件自身不许被 F4 点名
	chk("自指面：本件自身不被 F4 点名", !cfpgContains(res.mixed, cfpgGatesDirRel+"/"+cfpgSelfName))

	fmt.Fprintf(stdout, "── %s（%d 格）──\n", cfpgMapBool(ok, "全过", "有红"), total)
	if ok {
		return 0
	}
	return 1
}

func cfpgMapBool(cond bool, t, f string) string {
	if cond {
		return t
	}
	return f
}

func cfpgContains(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}

func cfpgAnyFile(xs []cfpgHardHit, suffix string) bool {
	for _, x := range xs {
		if strings.HasSuffix(x.file, suffix) {
			return true
		}
	}
	return false
}

func cfpgAnyDeclFile(xs []cfpgDeclHit, suffix string) bool {
	for _, x := range xs {
		if strings.HasSuffix(x.file, suffix) {
			return true
		}
	}
	return false
}

func cfpgAnyForbidFile(xs []cfpgForbidHit, suffix string) bool {
	for _, x := range xs {
		if strings.HasSuffix(x.file, suffix) {
			return true
		}
	}
	return false
}

// ── 默认根（声明 (a)）──────────────────────────────────────────────────────────────────

func cfpgDefaultRoot() string {
	d := cfpgPyAbspath(cfpgCwd(), ".")
	cur := d
	for {
		if fi, err := os.Stat(cfpgPyJoin(cur, cfpgGatesDirRel)); err == nil && fi.IsDir() {
			return cur
		}
		parent := cfpgPyDirname(cur)
		if parent == cur || parent == "" {
			break
		}
		cur = parent
	}
	return d
}

// ── argparse 子集 ─────────────────────────────────────────────────────────────────────

type cfpgArgs struct {
	selfTest bool
	json     bool
	root     string
}

func cfpgUsageHead() string {
	return "usage: " + cfpgProg + " [-h] [--self-test] [--json] [root]\n"
}

func cfpgHelpText() string {
	return cfpgUsageHead() + "\n" +
		"positional arguments:\n" +
		"  root\n" +
		"\n" +
		"optional arguments:\n" +
		"  -h, --help   show this help message and exit\n" +
		"  --self-test\n" +
		"  --json\n"
}

func cfpgArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, cfpgUsageHead())
	fmt.Fprintf(stderr, "%s: error: %s\n", cfpgProg, msg)
	return 2
}

// cfpgResolveOpt —— argparse 长选项解析：精确优先，否则唯一前缀（本表无歧义前缀）。
func cfpgResolveOpt(body string) (string, bool) {
	longs := []string{"--help", "--self-test", "--json"}
	full := "--" + body
	for _, o := range longs {
		if o == full {
			return o, true
		}
	}
	hit := ""
	n := 0
	for _, o := range longs {
		if strings.HasPrefix(o, full) {
			hit = o
			n++
		}
	}
	if n == 1 {
		return hit, true
	}
	return "", false
}

// cfpgParseArgs —— py `ap.parse_args(argv)` 的子集。done=true ⇒ 立即以 rc 返回。
func cfpgParseArgs(stdout, stderr io.Writer, argv []string) (cfpgArgs, int, bool) {
	a := cfpgArgs{}
	unknown := []string{}
	pos := []string{}
	afterDD := false
	for _, tok := range argv {
		if afterDD {
			pos = append(pos, tok)
			continue
		}
		if tok == "--" {
			afterDD = true
			continue
		}
		if tok == "-h" {
			fmt.Fprint(stdout, cfpgHelpText())
			return a, 0, true
		}
		if strings.HasPrefix(tok, "--") && len(tok) > 2 {
			body := tok[2:]
			eq := ""
			if j := strings.Index(body, "="); j >= 0 {
				body, eq = body[:j], body[j+1:]
			}
			name, ok := cfpgResolveOpt(body)
			if !ok {
				unknown = append(unknown, tok)
				continue
			}
			switch name {
			case "--help":
				fmt.Fprint(stdout, cfpgHelpText())
				return a, 0, true
			case "--self-test":
				if eq != "" {
					return a, cfpgArgErr(stderr, "argument --self-test: ignored explicit argument '"+eq+"'"), true
				}
				a.selfTest = true
			case "--json":
				if eq != "" {
					return a, cfpgArgErr(stderr, "argument --json: ignored explicit argument '"+eq+"'"), true
				}
				a.json = true
			}
			continue
		}
		if strings.HasPrefix(tok, "-") && len(tok) > 1 {
			unknown = append(unknown, tok)
			continue
		}
		pos = append(pos, tok)
	}
	if len(pos) > 0 {
		a.root = pos[0]
	}
	if len(pos) > 1 {
		unknown = append(unknown, pos[1:]...)
	}
	if len(unknown) > 0 {
		return a, cfpgArgErr(stderr, "unrecognized arguments: "+strings.Join(unknown, " ")), true
	}
	return a, 0, false
}

// ── 入口（= py `main()`）──────────────────────────────────────────────────────────────

func checkFaceParityGateMain(stdout, stderr io.Writer, argv []string) int {
	a, rc, done := cfpgParseArgs(stdout, stderr, argv)
	if done {
		return rc
	}
	if a.selfTest {
		return cfpgSelfTest(stdout)
	}
	root := a.root
	if root == "" {
		root = os.Getenv("ZERG_GATE_REPO_ROOT")
	}
	if root == "" {
		root = cfpgDefaultRoot()
	}
	root = cfpgPyAbspath(cfpgCwd(), root)
	rc, res := cfpgJudge(root)
	if a.json {
		fmt.Fprintln(stdout, cfpgDump(cfpgJSONTop(root, rc, res)))
		return rc
	}
	cfpgEmitHuman(stdout, res, rc)
	return rc
}
