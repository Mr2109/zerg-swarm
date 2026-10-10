// check_guard_forms.go —— 「手搓形态 ⇒ 正门替身」两形态对照判据（缺口 `GAP-20260927-37` · P1）。
//
// 病（缺口逐字 · 已现读坐实）：Hermes 家目录守卫 `~/.hermes/agent-hooks/zerg-cli-guard.py` 的 4 条
// BLOCK 规则全是**空白分隔的 shell 文本**形态（`git grep` · `grep -rR` · `sed -n` ·
// `python3 scripts/gates/`）；而 `subprocess.run(["git","grep",…])` 这类 **argv 参数数组**形态在词间夹
// `", "`（渲染成文本是 `git", "grep`）⇒ 词间非单空格 ⇒ 4 条规则**零命中** ⇒ 绕道写法整条直穿守卫。
// 现读：仓内**零处**镜像该守卫（`zerg code find 'zerg-cli-guard|shell_hooks|pre_tool_call'` = 0 命中）
// ⇒ 这条牙只在家目录件里、进不了仓。
//
// 本判据（候选 A · 仓内侧）：在同一套归一化 token 流上表达「危险形态」的**两形态对照** ——
//
//	① shell 文本形态：`git grep -n x -- .`（按空白切词 ⇒ token 流）
//	② argv 参数数组形态：`["git","grep","-n","x","--","."]`（数组元素**直接**即 token 流）
//
// 两形态喂**同一套** `guardFormsMatch`（同一组危险规则）⇒ argv 形态一旦被喂进来，就与 shell 形态
// 得到逐字同读的判定 ⇒ 绕道写法在**仓内 CLI 判据层**有牙。正门替身随规则逐条给出。
//
// ★ 第三形态（本单补 · 缺口 ② 前半「把守卫匹配面从 shell 文本拓到 python 子进程形态」）：`execute_code`
// 里直接写 `subprocess.run(['git','grep','-n','x'])` —— 这恰是喂料面最常拿到的那一发（`tool_input.code`
// 就是这段源码文本），既不是纯 shell 文本、也没被预先拆成 argv 数组。只喂 `--check`（壳文本路）⇒ 整段是
// **一个 token** ⇒ 4 条规则零命中（`guardFormsCheckPySourceAsShell` 留存这条改前错路的读数）。本形态直接
// 吃**源码文本**：`--check` 先按 shell 文本判、判不出且文本像 python 子进程调用时（`guardFormsCheckText`）
// 再从列表字面量里现取 argv（`guardFormsTokensPySource`），喂同一套 `guardFormsMatch`；`--py` 是显式入口
// （取不到可判的 argv 字面量 ⇒ **不给结论** rc=2）。改前只走壳文本路 ⇒ 整段是一个 token ⇒ 4 条规则零命中。
//
// 边界（照实）：本件只落码（Go 半 · 无 py 半）—— ✗ 不入 `main.go` 命令树；入
// `gate_inproc_table.go` 键 `check-guard-forms`（进程内路由）。✗ 不真跑任何被守的危险形态
// （只判 token 流 · 零子进程 · 零写盘 · 零网络），✗ 不碰任何家目录件。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ── 危险规则（照守卫 4 条 BLOCK 逐条对映 · 两形态同吃）────────────────────────────────────

// guardFormsRule —— 一条「手搓危险形态」及其**正门替身**。
type guardFormsRule struct {
	Key        string // 规则名（稳定标识 · 供对照与接替）
	Desc       string // 危险形态（手搓写法）
	Substitute string // 正门替身（仓内正解）
}

// guardFormsRules —— 守卫那 4 条 BLOCK 的仓内侧表达（顺序＝守卫 4 条）。
var guardFormsRules = []guardFormsRule{
	{"git-grep", "git grep（裸仓全文搜索）", "zerg code find <正则>"},
	{"sed-n", "sed -n（按行切片打印）", "zerg code show <件:行> [--ctx N]"},
	{"grep-recursive", "grep -r / -R（递归搜索）", "zerg code find <正则>"},
	{"direct-gate-script", "python3 scripts/gates/…（直跑门脚本）", "zerg gate check <判据名>"},
}

// ── 两形态 → 归一化 token 流 ────────────────────────────────────────────────────────────

// guardFormsTokensShell —— shell 文本形态：按空白（含 \t\n）切词（照守卫「空白分隔的 shell 文本」口径）。
func guardFormsTokensShell(blob string) []string {
	return strings.Fields(blob)
}

// guardFormsTokensArgv —— argv 参数数组形态：**数组元素直接即 token**（这是本单新增的那条牙）。
// ★ 与「把数组先渲染成文本再按空白切词」是两条路：后者词间夹 `", "` ⇒ token 变成 `["git",` 一类
// ⇒ 4 条规则零命中（＝本缺口病症，见 guardFormsCheckArgvAsShell）。
func guardFormsTokensArgv(argv []string) []string {
	out := make([]string, 0, len(argv))
	for _, a := range argv {
		if a == "" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// guardFormsRenderArgv —— 把 argv 数组渲染成它的**文本形态**（`["git","grep"]` ⇒ `[git, grep]`）。
// 用途：① 两形态对照报告里展示 argv 的文本写法；② 说明「改前」那条错路（渲染后再按 shell 切词）。
func guardFormsRenderArgv(argv []string) string {
	return "[" + strings.Join(argv, ", ") + "]"
}

// ── 同一套危险规则在归一化 token 流上的匹配（两形态共走此函数 ⇒ 逐字同读）──────────────────

// guardFormsMatch —— 归一化 token 流 ⇒ 命中的规则（命中返回规则 + true；干净返回零值 + false）。
func guardFormsMatch(tokens []string) (guardFormsRule, bool) {
	if len(tokens) == 0 {
		return guardFormsRule{}, false
	}
	cmd := guardFormsBase(tokens[0])
	switch cmd {
	case "git":
		if len(tokens) >= 2 && guardFormsBase(tokens[1]) == "grep" {
			return guardFormsRules[0], true // git-grep
		}
	case "sed":
		if len(tokens) >= 2 && tokens[1] == "-n" {
			return guardFormsRules[1], true // sed-n
		}
	case "grep":
		for _, t := range tokens[1:] {
			if guardFormsIsRecursiveFlag(t) {
				return guardFormsRules[2], true // grep-recursive
			}
		}
	}
	if guardFormsIsPython(tokens[0]) {
		for _, t := range tokens[1:] {
			if strings.Contains(t, "scripts/gates/") {
				return guardFormsRules[3], true // direct-gate-script
			}
		}
	}
	return guardFormsRule{}, false
}

// guardFormsCheckShell —— shell 文本形态 → 判定（审计喂料面 `--check` 的真身）。
func guardFormsCheckShell(blob string) (guardFormsRule, bool) {
	return guardFormsMatch(guardFormsTokensShell(blob))
}

// guardFormsCheckArgv —— argv 参数数组形态 → 判定（`--argv` 的真身 · **本单新增的那条牙**）。
func guardFormsCheckArgv(argv []string) (guardFormsRule, bool) {
	return guardFormsMatch(guardFormsTokensArgv(argv))
}

// guardFormsCheckArgvAsShell —— **改前那条错路**（照实留存 · 供自检与测试证出病症）：
// 把 argv 先渲染成文本、再按「空白分隔的 shell 文本」切词 —— 词间夹 `", "` ⇒ 4 条负控**零命中**。
// ✗ 这一路**不进**生产入口（生产走 guardFormsCheckArgv），只作两态对照的「改前」读数与自检负控。
func guardFormsCheckArgvAsShell(argv []string) (guardFormsRule, bool) {
	return guardFormsCheckShell(guardFormsRenderArgv(argv))
}

// ── 第三形态：python 子进程形态（`execute_code` 里的 subprocess 调用源码 · 本单新增）──────────────────

// guardFormsPyVerbs —— 认得的 python 子进程调用名（稳定闭集 · ✗ 不收裸 `run(`，防把普通调用/下标误当 argv）。
var guardFormsPyVerbs = []string{
	"subprocess.run(", "subprocess.Popen(", "subprocess.call(", "subprocess.check_call(",
	"subprocess.check_output(", "Popen(", "check_output(",
}

// guardFormsLooksPySubprocess —— 源码文本里有没有「python 子进程调用名」的形态骨架（没有 ⇒ 不判）。
func guardFormsLooksPySubprocess(blob string) bool {
	for _, v := range guardFormsPyVerbs {
		if strings.Contains(blob, v) {
			return true
		}
	}
	return false
}

// guardFormsTokensPySource —— python 子进程形态：从源码文本里取「调用的 argv 列表字面量」作 token 流。
// 取**第一个** `[ … ]` 配对内的元素（元素按「引号外的逗号」切分 · 再去外层引号）⇒ 元素直接即 token（照
// argv 形态口径）。返回 (tokens, ok)：ok=false ⇒ 取不到可判的 argv 列表字面量（调用方据此**不给结论**）。
func guardFormsTokensPySource(blob string) ([]string, bool) {
	if !guardFormsLooksPySubprocess(blob) {
		return nil, false
	}
	open := strings.IndexByte(blob, '[')
	if open < 0 {
		return nil, false
	}
	rest := blob[open+1:]
	closeIdx := guardFormsPyListClose(rest)
	if closeIdx < 0 {
		return nil, false
	}
	out := make([]string, 0, 8)
	for _, it := range guardFormsSplitPyList(rest[:closeIdx]) {
		if t := guardFormsUnquotePy(it); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// guardFormsPyListClose —— 在 `rest`（`[` 之后）里找**配平的** `]` 下标（认引号 · 认嵌套 `[`）。找不到 ⇒ -1。
func guardFormsPyListClose(rest string) int {
	depth := 0
	var q byte
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '\'' || c == '"':
			q = c
		case c == '[':
			depth++
		case c == ']':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// guardFormsSplitPyList —— 按**引号外的逗号**切分列表字面量内部（认引号内逗号，如 `'1,10p'` 不拆）。
func guardFormsSplitPyList(inner string) []string {
	out := []string{}
	var cur strings.Builder
	var q byte
	for i := 0; i < len(inner); i++ {
		c := inner[i]
		switch {
		case q != 0:
			cur.WriteByte(c)
			if c == q {
				q = 0
			}
		case c == '\'' || c == '"':
			q = c
			cur.WriteByte(c)
		case c == ',':
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if x := strings.TrimSpace(cur.String()); x != "" {
		out = append(out, x)
	}
	return out
}

// guardFormsUnquotePy —— 去 python 字面量外层引号（`'x'` / `"x"` ⇒ `x`）；无引号原样返回。
func guardFormsUnquotePy(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// guardFormsCheckPy —— python 子进程形态 → 判定；ok=false ⇒ 取不到 argv 字面量 ⇒ **不给结论**（rc=2）。
func guardFormsCheckPy(blob string) (rule guardFormsRule, hit bool, ok bool) {
	toks, ok := guardFormsTokensPySource(blob)
	if !ok {
		return guardFormsRule{}, false, false
	}
	r, h := guardFormsMatch(toks)
	return r, h, true
}

// guardFormsCheckPySourceAsShell —— **改前那条错路**（照实留存 · 供自检证出病症）：把 python 源码文本当
// **壳文本**按空白切词 ⇒ 整段是一个 token ⇒ 4 条规则**零命中**（本形态未被识别时的漏判）。
// ✗ 这一路**不进**生产入口（生产走 guardFormsCheckPy）。
func guardFormsCheckPySourceAsShell(blob string) (guardFormsRule, bool) {
	return guardFormsCheckShell(blob)
}

// guardFormsCheckText —— `--check` 的真身（★ 本单把匹配面从 shell 文本拓到 python 子进程形态）：
// 先按 **shell 文本形态**判；判不出且文本像 python 子进程调用时，再按 **python 子进程形态**取 argv 判。
// 两路都判不出 ⇒ 干净（rc=0）。⇒ 喂料面把 `tool_input.code`（python 源码）整段交给 `--check` 也能判。
func guardFormsCheckText(blob string) (guardFormsRule, bool) {
	if r, hit := guardFormsCheckShell(blob); hit {
		return r, hit
	}
	if toks, ok := guardFormsTokensPySource(blob); ok {
		if r, hit := guardFormsMatch(toks); hit {
			return r, hit
		}
	}
	return guardFormsRule{}, false
}

// guardFormsRenderPyCall —— 把 argv 渲染成「python 子进程调用」的源码写法（自检/报告的文本形态）。
func guardFormsRenderPyCall(argv []string) string {
	q := make([]string, 0, len(argv))
	for _, a := range argv {
		q = append(q, "'"+a+"'")
	}
	return "subprocess.run([" + strings.Join(q, ",") + "])"
}

// guardFormsBase —— 取 token 的基名（去目录 · 供 `git` / `/usr/bin/git` 同判）。
func guardFormsBase(tok string) string {
	if i := strings.LastIndexByte(tok, '/'); i >= 0 {
		return tok[i+1:]
	}
	return tok
}

// guardFormsIsRecursiveFlag —— grep 的递归旗标：短旗标（单个 `-`）体里含 r/R（`-r` · `-R` · `-rn` · `-nr`）。
// ★ 不误伤长旗标 `--color`（`--` 开头直接排除）、也不误伤 `-n` / `-e` 这类无 r/R 的短旗标。
func guardFormsIsRecursiveFlag(tok string) bool {
	if !strings.HasPrefix(tok, "-") || strings.HasPrefix(tok, "--") {
		return false
	}
	body := tok[1:]
	if body == "" {
		return false
	}
	for _, r := range body {
		if r == 'r' || r == 'R' {
			return true
		}
	}
	return false
}

// guardFormsIsPython —— 解释器 token 是不是 python 家族（python / python3 / /usr/bin/python3.11 …）。
func guardFormsIsPython(tok string) bool {
	return strings.HasPrefix(guardFormsBase(tok), "python")
}

// ── 两形态对照的 6 条 canonical 靶子（4 条 argv 负控 ＋ 2 条只读 argv 正控）──────────────────

// guardFormsPair —— 一组「同一操作的两形态写法」＋ 期望判定（WantDanger="" ⇒ 期望干净）。
type guardFormsPair struct {
	Name       string
	Shell      string
	Argv       []string
	WantDanger string // 期望命中的规则 key；"" = 期望不被判危险
}

// guardFormsPairs —— 6 条靶子：4 条 argv 负控（照守卫 4 条逐条对映）＋ 2 条只读正控（防误判）。
var guardFormsPairs = []guardFormsPair{
	{"git-grep", "git grep -n x -- .", []string{"git", "grep", "-n", "x", "--", "."}, "git-grep"},
	{"sed-n", "sed -n 1,10p f", []string{"sed", "-n", "1,10p", "f"}, "sed-n"},
	{"grep-recursive", "grep -r x .", []string{"grep", "-r", "x", "."}, "grep-recursive"},
	{"direct-gate-script", "python3 scripts/gates/precommit-gates.sh",
		[]string{"python3", "scripts/gates/precommit-gates.sh"}, "direct-gate-script"},
	{"git-status(只读)", "git status --porcelain", []string{"git", "status", "--porcelain"}, ""},
	{"zerg-code-show(只读)", "zerg code show f:1", []string{"zerg", "code", "show", "f:1"}, ""},
}

// ── 自检面（成对正负控 · 合成靶子 · 不碰真目标 · 零副作用）──────────────────────────────

// checkGuardFormsSelfTest —— 同型自检面（逐格断言 · 失败打 `✗ 格N …` · 末行 `自检 PASS/FAIL（N 格）`）。
//
// 变绿 = 4 条 argv 负控 ＋ 4 条 python 负控皆命中对应规则 ＋ 只读正控不误判（＋ 多形态逐格同读 ＋ 病症负控）。
// 变红（正是本缺口病症）= 任一负控不命中，或只读靶子被误判，或某形态判定分歧。
func checkGuardFormsSelfTest(stdout io.Writer) int {
	ok := true
	n := 0
	chk := func(label string, pass bool) {
		n++
		if !pass {
			fmt.Fprintf(stdout, "  ✗ 格%d %s\n", n, label)
			ok = false
		}
	}
	// 格1..6：4 条 argv 负控必命中对应规则 · 2 条只读 argv 正控必不误判。
	for _, p := range guardFormsPairs {
		got, hit := guardFormsCheckArgv(p.Argv)
		if p.WantDanger == "" {
			chk(fmt.Sprintf("正控·argv 只读形态 %q 不误判（得 %q/%v）", p.Name, got.Key, hit), !hit)
		} else {
			chk(fmt.Sprintf("负控·argv 形态 %q 命中规则 %q（得 %q/%v）", p.Name, p.WantDanger, got.Key, hit),
				hit && got.Key == p.WantDanger)
		}
	}
	// 格7..12：两形态对照 —— 每组 shell 形态与 argv 形态判定必须**逐字同读**（同规则 / 同干净）。
	for _, p := range guardFormsPairs {
		gots, hits := guardFormsCheckShell(p.Shell)
		gota, hita := guardFormsCheckArgv(p.Argv)
		chk(fmt.Sprintf("对照·两形态同读 %q（shell=%v/%q argv=%v/%q）", p.Name, hits, gots.Key, hita, gota.Key),
			hits == hita && gots.Key == gota.Key)
	}
	// 格13：证出病症 —— **改前那条错路**（argv 渲染成文本再切词）对 4 条负控**零命中**；
	// 若它居然命中 ⇒ 说明两形态已无差别、本对照失去意义。
	allEscape := true
	for _, p := range guardFormsPairs {
		if p.WantDanger == "" {
			continue
		}
		if _, hit := guardFormsCheckArgvAsShell(p.Argv); hit {
			allEscape = false
		}
	}
	chk("病症·改前错路（argv 渲染成文本再切词）对 4 条负控零命中", allEscape)
	// 格14..：python 子进程形态（第三形态）—— 4 条负控必命中对应规则 · 只读正控不误判 · 取不到 ⇒ 不给结论。
	for _, p := range guardFormsPairs {
		if p.WantDanger == "" {
			continue
		}
		src := guardFormsRenderPyCall(p.Argv)
		got, hit, okp := guardFormsCheckPy(src)
		chk(fmt.Sprintf("负控·python 形态 %q 命中规则 %q（得 %q/%v/%v）", p.Name, p.WantDanger, got.Key, hit, okp),
			okp && hit && got.Key == p.WantDanger)
	}
	gotP, hitP, okP := guardFormsCheckPy("subprocess.run(['git','status','--porcelain'])")
	chk(fmt.Sprintf("正控·python 形态只读靶子不误判（得 %q/%v/%v）", gotP.Key, hitP, okP), okP && !hitP)
	_, _, okMiss := guardFormsCheckPy("subprocess.run(cmd)")
	chk("负控·python 形态取不到 argv ⇒ 不给结论（ok=false）", !okMiss)
	pyEscape := true
	for _, p := range guardFormsPairs {
		if p.WantDanger == "" {
			continue
		}
		if _, hit := guardFormsCheckPySourceAsShell(guardFormsRenderPyCall(p.Argv)); hit {
			pyEscape = false
		}
	}
	chk("病症·改前错路（python 源码当壳文本切词）对 4 条负控零命中", pyEscape)
	// 格：`--check` 单一入口吃两形态（shell 文本 ＋ python 子进程源码）—— 缺口 ② 前半的可判面。
	_, hitPC := guardFormsCheckText("subprocess.run(['git','grep','-n','x'])")
	chk("对照·--check 路认 python 子进程形态（git grep 命中）", hitPC)
	_, hitPS := guardFormsCheckText("subprocess.run(['git','status'])")
	chk("正控·--check 路 python 只读形态不误判", !hitPS)
	res := "FAIL"
	if ok {
		res = "PASS"
	}
	fmt.Fprintf(stdout, "自检 %s（%d 格）\n", res, n)
	if ok {
		return 0
	}
	return 1
}

// ── 多形态对照报告（默认档 · 只读靶子 · 零副作用）───────────────────────────────────────

// checkGuardFormsReport —— 打印「危险规则」表 ＋ 6 条靶子的多形态对照读数（shell/argv/py 同读）。
// rc=0 = 两形态处处同读；rc=1 = 任一组两形态判定**分歧**（＝argv 形态未被识别的病症读数）。
func checkGuardFormsReport(stdout io.Writer) int {
	fmt.Fprintf(stdout, "── 危险规则（手搓形态 ⇒ 正门替身）——共 %d 条\n", len(guardFormsRules))
	for _, r := range guardFormsRules {
		fmt.Fprintf(stdout, "  · %-20s %s  ⇒ %s\n", r.Key, r.Desc, r.Substitute)
	}
	fmt.Fprintf(stdout, "── 多形态对照（shell 文本 ⟷ argv 参数数组 ⟷ python 子进程源码）\n")
	divergent := 0
	for _, p := range guardFormsPairs {
		gots, hits := guardFormsCheckShell(p.Shell)
		gota, hita := guardFormsCheckArgv(p.Argv)
		gotp, hitp, okp := guardFormsCheckPy(guardFormsRenderPyCall(p.Argv))
		pyVerdict := guardFormsVerdict(hitp, gotp)
		if !okp {
			pyVerdict = "取不到"
		}
		mark := "同读"
		if hits != hita || gots.Key != gota.Key || !okp || hitp != hits || gotp.Key != gots.Key {
			mark = "✗ 分歧"
			divergent++
		}
		fmt.Fprintf(stdout, "  [%s] %-22s shell=%s · argv=%s · py=%s · argv 文本写法=%s\n",
			mark, p.Name, guardFormsVerdict(hits, gots), guardFormsVerdict(hita, gota), pyVerdict, guardFormsRenderArgv(p.Argv))
	}
	if divergent != 0 {
		fmt.Fprintf(stdout, "⇒ 多形态 %d 组分歧（rc=1）：某形态未被同读 —— 绕道写法在判据层漏判\n", divergent)
		return 1
	}
	fmt.Fprintf(stdout, "⇒ 多形态处处同读（rc=0）：%d 组皆同判\n", len(guardFormsPairs))
	return 0
}

// guardFormsVerdict —— 判定的可读串（干净 / 危险:<key>）。
func guardFormsVerdict(hit bool, r guardFormsRule) string {
	if !hit {
		return "干净"
	}
	return "危险:" + r.Key
}

// ── 入口（与其它 Go 真搬门同形：rc 0 全过 / 1 有红 / 2 不给结论）────────────────────────────

const guardFormsUsage = "usage: check-guard-forms [--check <blob> | --argv <json> | --py <code>] [--self-test] [--help]\n" +
	"  --check <blob>  判一条文本形态（shell 文本 或 python 子进程源码 · 审计喂料）\n" +
	"  --argv <json>   判一条 argv 参数数组形态（JSON 字符串数组 · 审计喂料）\n" +
	"  --py <code>     判一条 python 子进程形态（subprocess.run([...]) 源码文本 · 取不到 argv ⇒ rc=2）\n" +
	"  --self-test     多形态对照自检（4 argv 负控 ＋ 2 只读正控 ＋ 多形态同读 ＋ python 形态负控 ＋ 取不到负控）\n" +
	"  无旗标          打印危险规则表 ＋ 6 组多形态对照读数\n"

// checkGuardFormsGateMain —— 进程内入口（`gate_inproc_table.go` 键 `check-guard-forms`）。
func checkGuardFormsGateMain(stdout, stderr io.Writer, argv []string) int {
	var (
		selfTest  bool
		haveCheck bool
		checkBlob string
		haveArgv  bool
		argvJSON  string
		havePy    bool
		pyCode    string
	)
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, guardFormsUsage)
			return 0
		case a == "--self-test":
			selfTest = true
		case a == "--check":
			if i+1 >= len(argv) {
				fmt.Fprintf(stderr, "check-guard-forms: 参数 %s 缺值 ⇒ rc=2\n%s", a, guardFormsUsage)
				return 2
			}
			i++
			haveCheck = true
			checkBlob = argv[i]
		case a == "--argv":
			if i+1 >= len(argv) {
				fmt.Fprintf(stderr, "check-guard-forms: 参数 %s 缺值 ⇒ rc=2\n%s", a, guardFormsUsage)
				return 2
			}
			i++
			haveArgv = true
			argvJSON = argv[i]
		case a == "--py":
			if i+1 >= len(argv) {
				fmt.Fprintf(stderr, "check-guard-forms: 参数 %s 缺值 ⇒ rc=2\n%s", a, guardFormsUsage)
				return 2
			}
			i++
			havePy = true
			pyCode = argv[i]
		default:
			fmt.Fprintf(stderr, "check-guard-forms: error: unrecognized arguments: %s\n%s", a, guardFormsUsage)
			return 2
		}
		i++
	}
	if selfTest {
		return checkGuardFormsSelfTest(stdout)
	}
	if haveCheck {
		r, hit := guardFormsCheckText(checkBlob)
		return guardFormsVerdictOut(stdout, "壳文本/源码", checkBlob, hit, r)
	}
	if haveArgv {
		var arr []string
		if err := json.Unmarshal([]byte(argvJSON), &arr); err != nil {
			fmt.Fprintf(stderr, "check-guard-forms: --argv 不是合法 JSON 字符串数组：%v（rc=2）\n", err)
			return 2
		}
		r, hit := guardFormsCheckArgv(arr)
		return guardFormsVerdictOut(stdout, "argv 数组", guardFormsRenderArgv(arr), hit, r)
	}
	if havePy {
		rule, hit, ok := guardFormsCheckPy(pyCode)
		if !ok {
			fmt.Fprintf(stderr, "check-guard-forms: --py 取不到可判的 argv 字面量（非 subprocess 列表调用形态）⇒ **不给结论** rc=2\n")
			return 2
		}
		return guardFormsVerdictOut(stdout, "python 子进程", pyCode, hit, rule)
	}
	return checkGuardFormsReport(stdout)
}

// guardFormsVerdictOut —— 单靶判定输出（命中 ⇒ 打印规则 + 正门替身；干净 ⇒ 打印干净）；命中退 1、否则 0。
func guardFormsVerdictOut(stdout io.Writer, form, shown string, hit bool, r guardFormsRule) int {
	if !hit {
		fmt.Fprintf(stdout, "干净：%s %s 未命中任何危险形态\n", form, shown)
		return 0
	}
	fmt.Fprintf(stdout, "危险：%s %s 命中规则 %q（%s）⇒ 正门替身：%s\n", form, shown, r.Key, r.Desc, r.Substitute)
	return 1
}
