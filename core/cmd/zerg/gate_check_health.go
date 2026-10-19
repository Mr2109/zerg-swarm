// gate_check_health.go —— 判据件 `scripts/gates/gate-check-health.py` 的 **Go 真搬**（v2.5.14 · M2 路B · 首字母 f–p 批）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（《设计-判据 CLI 化-v1.0-20260930.md》第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2 路B（并行第三批 · 候选区间首字母 `f`–`p`）的真搬一枚：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（M2-0 = 「无输入物料、纯静态检查的判据（读仓内件即可）」）：
//
//	✓ 选 `scripts/gates/gate-check-health.py`（210 行 · 纯 stdlib argparse/json/os/re/subprocess/importlib/sys）：
//	  · **无外部树依赖**：只读仓内两件 `<根>/core/cmd/zerg/family_gate_check.go` 与
//	    `<根>/scripts/gates/judge-registry.json`（不碰文档仓/公开树/生成物）；
//	  · **输出确定有序**：checks 键序固定（bash → python），problems/notes 按键发生序 append，
//	    名录计数走递归 `scripts/` 全子树（Go `WalkDir` 与 py `os.walk` 均只做计数 ⇒ 序遍历差异不致假差）；
//	  · **退码 0/1/2 三档齐**：0 通过 / 1 违例 / 2 不给结论（壳件读不到 · `--self-test` 不过 · 用法错）；
//	  · **纯静态无写面**：本件一个字节都不写（`--json` 只印 stdout；✗ 无 `--dump` 类写旗标）；
//	  · 210 行（**行数适中**）· `git status` 现读**不含它**（无人正在改）。
//	✗ 不选同区间另四件（逐条）：
//	  · `gate-report.py`（318 行）：全绿/红档**必需真跑各门并计时**（`elapsed_s` 三位小数）⇒
//	    **输出非确定**，逐字节对拍必假差 ⇒ 不满足「输出确定有序」。
//	  · `input-fingerprint.py`（165 行）：**默认档带写面**（`MISS` 就 `json.dump` 落缓存件）＋
//	    `--cmd` 现跑 shell ⇒ 不属「纯静态无写面」，且**要输入物料**。
//	  · `judge-lock.py`（262 行）：**并发原语件**（`flock` 取件级锁 ＋ 槽池排队）⇒ 输出随**本机锁态/
//	    时钟**变（`--wait` 排队、锁目录 `$TMPDIR`）、**带写面**（就地 `O_CREAT` 落 `.lock`）⇒ 非确定 + 非纯静态；
//	    且它是**禁改共享件**（并行另一路口径）。
//	  · `preview-gate-steps.py`（526 行）：**已列重件** ＋ 真源＝现跑 `precommit-gates.sh --list`（**禁改共享件**）
//	    ⇒ 有外部件依赖且重。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `REPO = dirname(dirname(dirname(abspath(__file__))))`（`scripts/gates` 上溯三层
//	    ⇒ 恒为真仓根）；Go 入口**无 `__file__` 语境**（未来接线后是 CLI 子命令，更没有脚本路径）⇒ 本件默认从
//	    **当前工作目录（cwd）** 取根。落在**同一真仓、cwd = 仓根**时两者读数逐字相同；但「脚本位置 ≠ cwd」时
//	    **路径推导面不同** ⇒ 这一面**不属逐字对齐承诺**，对拍一律显式传 `--root`（或 cwd 置仓根）。
//	(b) **自指检查**：本件的**扫描面 = 单件 `<根>/core/cmd/zerg/family_gate_check.go`（读文本）+ `<根>/scripts/`
//	    名录计数（只数 `.py`/`.sh` 件数）**；本件自身落点 `core/cmd/zerg/gate_check_health.go` **既不在那个单件里、
//	    也不在 `scripts/` 子树下** ⇒ **✗ 本件不在自己的扫描面内**（自指为假）。因此件头/自检/夹具里的**标记字面**
//	    不会自伤；尽管如此本件**仍全用解释型字符串、✗ 不用反引号原串直写**标记（参 `check_wall_layer.go` 血证），
//	    以免日后扫描面扩到 `core/cmd/zerg/*.go` 时自伤。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/gate-check-health.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· 入口 `gateCheckHealthMain` 目前**无调用点** —— 这是 M2 的**刻意**状态：先证明「搬得逐字相同」。
//	· 缺口承接：`GAP-20261001-13`（M2 真搬 Go 线 · 在册 · 本单引为 `--evidence`）· ✗ 不新占号。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func gateCheckHealthMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 达底线 / 1 违例 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字）：全绿（合成根·壳件合规·注册表计数相符 → 0）· 四类违例各红（按名名单标记 /
// 解释器闭集被改宽 / 缺 `filepath.WalkDir` / 写死 `"scripts/…"` → 1）· 注册表计数不符（真仓现读 → 1）·
// 注册表不在盘（只报告格 · 不判红 → 0）· 空转（root 无壳件 → 2）· 取不到输入（root 不存在 → 2）·
// `--json`（人读面同源 · 同结构多渲染）· `--self-test` 五档 · `-h` · 未知旗标 → 2。
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **py 版本读数**：py 取**自身解释器** `sys.version_info[:3]`；本件 **exec `python3 -c`** 取解析器版本。
//	   同一台机、`PATH` 里 `python3` = 跑 py 原件那个解释器时逐字相同（已对拍）；但「跑 py 用的解释器 ≠
//	   `PATH` 的 `python3`」/ `PATH` 无 `python3` 时，本件的 python 读数会**取不到**（本件落「不给结论」行）——
//	   py 无此路径。此面**不属逐字对齐承诺**。
//	② **注册表模块定位**：py `load_registry_module()` 从**本件 `__file__` 目录**（`scripts/gates`）找
//	   `check-judge-registry.py` 复用其 `scan_roster`；本件无 `__file__` ⇒ 取 `<根>/scripts/gates/check-judge-registry.py`。
//	   **根 = 真仓**时两者相同；**根 ≠ 真仓且夹具自带注册表**时，py 仍加载真仓模块、本件按夹具根找 ⇒
//	   夹具须自带同名模块副本（本单对拍夹具 C2 即如此），该面**不属逐字对齐承诺**。
//	③ **异常路径**：壳件存在但**读不到**（权限）/ 注册表 JSON **解不动 / 顶层非对象** —— py 抛
//	   `UnicodeDecodeError`/`AttributeError` 等（壳件那条**未捕获** ⇒ 进程退 1 带 traceback；注册表那条
//	   **被 except 捕** ⇒ 落「读数取不到」note，但 note 正文含 py 异常串）⇒ 本件落「读不到」失败行 / 通用 note，
//	   **退码同、正文不同**。此面不属逐字对齐承诺。
//	④ **argparse 全等**：本件手写迷你解析器，覆盖 `-h/--help` · `--root V` · `--root=V` · `--json` ·
//	   `--self-test` · **唯一前缀缩写**（`--se` ⇒ `--self-test`）· 未知旗标 `unrecognized arguments:` ·
//	   缺值 `argument --root: expected one argument`；**不覆盖**长旗标**歧义前缀**的 `ambiguous option`
//	   正文、`--json=x` 的 `ignored explicit argument` 正文、`-hx` 短旗标粘连。此面不属逐字对齐承诺。
//	⑤ **文本换行**：py `text=True` 做通用换行翻译（`\r\n`→`\n`）；本件读子进程原始字节 ⇒ 子进程输出含
//	   `\r` 时正文不同（本机 `bash --version` / `python3` 输出恒 `\n`，已对拍）。此面不属逐字对齐承诺。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ── 常量真源（与 py 逐字对齐 · 名字前缀 `gch` 防包内同名）──────────────
const (
	gchProg       = "gate-check-health.py"
	gchGoShellRel = "core/cmd/zerg/family_gate_check.go"
	// ★ 门80 扫描面（`GAP-20261001-75` 路A）：inproc 分发表件也须在扫描面内。
	gchGoShellRelTbl = "core/cmd/zerg/gate_inproc_table.go"
	gchRegistryRel   = pathJudgeRegistry
	gchRosterDir     = "scripts"
	gchModuleRel     = "scripts/gates/check-judge-registry.py"
)

// gchGoShellRels —— 门80 扫描面（壳逻辑 ＋ inproc 分发表 · `GAP-20261001-75` 路A）：逐件读、拼接后判一次。
var gchGoShellRels = []string{gchGoShellRel, gchGoShellRelTbl}

// gchMinTool —— py `MIN_TOOL`（唯二真源：bash 3.2 · python 3.9）。
var gchMinTool = map[string][2]int{"bash": {3, 2}, "python": {3, 9}}

// gchNameListMarkers —— py `NAME_LIST_MARKERS`（**唯一真源**：按名名单/库类特殊化的形态标记）。
var gchNameListMarkers = []string{
	"gateCheckAllow", "gateCheckDeny", "gateSkipNames", "gateExcludeNames",
	"gateLibrary", "gateSpecialCase",
}

// gchUsageLine / gchHelpText —— py argparse 的 usage 行与 `-h` 正文字节（逐字 · Python 3.9 口径
// `optional arguments:`）。✗ 不用反引号原串直写（见件头 (b)）。
const gchUsageLine = "usage: gate-check-health.py [-h] [--root ROOT] [--json] [--self-test]\n"

const gchHelpText = "usage: gate-check-health.py [-h] [--root ROOT] [--json] [--self-test]\n" +
	"\n" +
	"`zerg gate check` 转发壳体检件（M1-07 · 平台底线 ＋ 库类不特殊化）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --json       机器面（只印 JSON）\n" +
	"  --self-test\n"

// gchVersionRE —— py `re.search(r"(\d+)\.(\d+)(?:\.(\d+))?")`。
var gchVersionRE = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// gchInterpRE —— py `re.search(r"gateCheckInterpreter\s*=\s*map\[string\]string\{(.*?)\}", text, re.S)`。
var gchInterpRE = regexp.MustCompile(`(?s)gateCheckInterpreter\s*=\s*map\[string\]string\{(.*?)\}`)

// gchPairRE —— py `re.findall(r'"([^"]*)"\s*:\s*"([^"]*)"', group1)`。
var gchPairRE = regexp.MustCompile(`"([^"]*)"\s*:\s*"([^"]*)"`)

// gchChecks / gchReport —— py `rep` 的**键序**结构（json.dumps 保 insertion order ⇒ 用 struct 定序）。
type gchChecks struct {
	Bash   *string `json:"bash"`
	Python *string `json:"python"`
}

type gchReport struct {
	Root     string    `json:"root"`
	Checks   gchChecks `json:"checks"`
	Problems []string  `json:"problems"`
	Notes    []string  `json:"notes"`
}

// gateCheckHealthMain —— 入口（= py `main()`）。
func gateCheckHealthMain(stdout, stderr io.Writer, argv []string) int {
	ns, rc, done := gchParseArgs(stdout, stderr, argv)
	if done {
		return rc
	}
	if ns.selfTest {
		return gchSelfTest(stdout)
	}
	rc, rep := gchRunHealth(gchAbs(ns.root))
	if ns.jsonOut {
		gchPrintJSON(stdout, rep)
		return rc
	}
	bashDisp := "None"
	if rep.Checks.Bash != nil {
		bashDisp = *rep.Checks.Bash
	}
	pyDisp := "None"
	if rep.Checks.Python != nil {
		pyDisp = *rep.Checks.Python
	}
	fmt.Fprintln(stdout, "gate check 体检件 —— 平台底线 ＋ 库类不特殊化")
	fmt.Fprintf(stdout, "  bash   : %s（底线 %s）\n", bashDisp, gchDotsPair(gchMinTool["bash"]))
	fmt.Fprintf(stdout, "  python : %s（底线 %s）\n", pyDisp, gchDotsPair(gchMinTool["python"]))
	for _, n := range rep.Notes {
		fmt.Fprintln(stdout, "  · "+n)
	}
	if len(rep.Problems) > 0 {
		fmt.Fprintf(stdout, "✗ 违例 %d 条：\n", len(rep.Problems))
		for _, p := range rep.Problems {
			fmt.Fprintln(stdout, "  · "+p)
		}
	} else {
		fmt.Fprintln(stdout, "✓ 全绿（rc=0）：平台达底线 · 库类不特殊化成立")
	}
	return rc
}

// gchNamespace —— py argparse 结果集。
type gchNamespace struct {
	root     string
	jsonOut  bool
	selfTest bool
}

// gchParseArgs —— py `argparse.ArgumentParser` 的最小等价（见件头未覆盖面④）。
// 返回 (ns, rc, done)：done=true ⇒ 已终局（`-h` 印正文退 0 / 用法错印 stderr 退 2）。
func gchParseArgs(stdout, stderr io.Writer, argv []string) (gchNamespace, int, bool) {
	ns := gchNamespace{root: ""}
	// ① `-h/--help` 命中即印帮助退 0（py：help action 就地 exit ⇒ 先于一切，含未知旗标）。
	for _, a := range argv {
		if a == "-h" || a == "--help" || gchPrefixUnique(a) == "help" {
			fmt.Fprint(stdout, gchHelpText)
			return ns, 0, true
		}
	}
	var unknown []string
	rootSet := false
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "--json":
			ns.jsonOut = true
		case a == "--self-test":
			ns.selfTest = true
		case a == "--root":
			if i+1 >= len(argv) || (strings.HasPrefix(argv[i+1], "-") && argv[i+1] != "-") {
				fmt.Fprint(stderr, gchUsageLine)
				fmt.Fprintf(stderr, "%s: error: argument --root: expected one argument\n", gchProg)
				return ns, 2, true
			}
			ns.root = argv[i+1]
			rootSet = true
			i++
		case strings.HasPrefix(a, "--root="):
			ns.root = strings.TrimPrefix(a, "--root=")
			rootSet = true
		default:
			switch {
			case strings.HasPrefix(a, "--"):
				if got := gchPrefixUnique(a); got == "json" {
					ns.jsonOut = true
				} else if got == "self-test" {
					ns.selfTest = true
				} else {
					unknown = append(unknown, a)
				}
			default:
				unknown = append(unknown, a)
			}
		}
		i++
	}
	if len(unknown) > 0 {
		fmt.Fprint(stderr, gchUsageLine)
		fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", gchProg, strings.Join(unknown, " "))
		return ns, 2, true
	}
	_ = rootSet
	return ns, 0, false
}

// gchPrefixUnique —— argparse 长旗标「唯一前缀缩写」匹配（返回规范名 / 空）。
// 闭集 = {help, root, json, self-test}（首字母互异 ⇒ 前缀不会歧义）。
func gchPrefixUnique(a string) string {
	if !strings.HasPrefix(a, "--") {
		return ""
	}
	name := strings.TrimPrefix(a, "--")
	if strings.Contains(name, "=") {
		return ""
	}
	for _, cand := range []string{"help", "root", "json", "self-test"} {
		if cand != "" && strings.HasPrefix(cand, name) {
			return cand
		}
	}
	return ""
}

// gchRunHealth —— py `run_health(root, ...)` ⇒ (rc, report)。
func gchRunHealth(root string) (int, gchReport) {
	rep := gchReport{Root: root, Problems: []string{}, Notes: []string{}}

	bashV, bashOK := gchProbeBash()
	pyV, pyOK := gchProbePython()
	if bashOK {
		s := gchDots(bashV)
		rep.Checks.Bash = &s
	}
	if pyOK {
		s := gchDots(pyV)
		rep.Checks.Python = &s
	}

	if !bashOK {
		rep.Problems = append(rep.Problems, "bash 版本读数取不到 ⇒ 不给结论")
	} else if gchLt2(bashV, gchMinTool["bash"]) {
		rep.Problems = append(rep.Problems, fmt.Sprintf("bash %s 低于底线 %s",
			gchDots(bashV), gchDotsPair(gchMinTool["bash"])))
	}
	if !pyOK {
		rep.Problems = append(rep.Problems, "python 版本读数取不到 ⇒ 不给结论")
	} else if gchLt2(pyV, gchMinTool["python"]) {
		rep.Problems = append(rep.Problems, fmt.Sprintf("python %s 低于底线 %s",
			gchDots(pyV), gchDotsPair(gchMinTool["python"])))
	}

	// ★ 扫描面可含多件：逐件读、缺一即「不给结论」，**拼接**后按整壳判一次。
	var shellTexts []string
	for _, rel := range gchGoShellRels {
		goPath := filepath.Join(root, filepath.FromSlash(rel))
		if !gchIsFile(goPath) {
			rep.Problems = append(rep.Problems, fmt.Sprintf("壳件读不到：%s ⇒ 不给结论", rel))
			return 2, rep
		}
		shellTexts = append(shellTexts, gchReadText(goPath))
	}
	rep.Problems = append(rep.Problems, gchCheckGoShell(strings.Join(shellTexts, "\n"))...)

	// 只报告格：注册表条数 ⟷ 现读名录件数
	regPath := filepath.Join(root, filepath.FromSlash(gchRegistryRel))
	modPath := filepath.Join(root, filepath.FromSlash(gchModuleRel))
	if gchIsFile(regPath) && gchIsFile(modPath) {
		regCount, regOK := gchRegCount(gchReadText(regPath))
		if !regOK {
			rep.Notes = append(rep.Notes, "注册表读数取不到（只报告 · 不判红）：注册表不是 JSON 对象或 `count` 不可读")
		} else {
			live := gchScanRoster(root)
			liveN := 0
			for _, v := range live {
				if len(v) == 1 {
					liveN++
				}
			}
			if regCount != liveN {
				rep.Problems = append(rep.Problems, fmt.Sprintf("注册表条目数 %d ≠ 现读名录件数 %d", regCount, liveN))
			} else {
				rep.Notes = append(rep.Notes, fmt.Sprintf("注册表条数 == 现读名录件数（%d）", liveN))
			}
		}
	} else {
		rep.Notes = append(rep.Notes, "注册表不在盘 ⇒ 该格不适用（只报告）")
	}

	if len(rep.Problems) > 0 {
		return 1, rep
	}
	return 0, rep
}

// gchCheckGoShell —— py `check_go_shell(text)`：返回问题清单（空 = 库类不特殊化成立）。
func gchCheckGoShell(text string) []string {
	var problems []string
	m := gchInterpRE.FindStringSubmatch(text)
	if m == nil {
		problems = append(problems, "壳件里找不到 `gateCheckInterpreter` 解释器闭集 ⇒ 判不了")
	} else {
		keys := map[string]bool{}
		for _, p := range gchPairRE.FindAllStringSubmatch(m[1], -1) {
			keys[p[1]] = true
		}
		if !(len(keys) == 2 && keys[".py"] && keys[".sh"]) {
			problems = append(problems, fmt.Sprintf("解释器闭集不是 {.py, .sh}（现读 %s）⇒ 疑似对某类件特殊化", gchPyListRepr(gchSortedKeys(keys))))
		}
	}
	if !strings.Contains(text, "filepath.WalkDir") {
		problems = append(problems, "名录不是**目录派生**（缺 `filepath.WalkDir`）⇒ 疑似按名名单")
	}
	if strings.Contains(text, `"scripts/`) {
		problems = append(problems, "壳件里出现写死的 `\"scripts/…\"` 名件路径 ⇒ 疑似按名名单（把库类升格/剔除）")
	}
	for _, mk := range gchNameListMarkers {
		if strings.Contains(text, mk) {
			problems = append(problems, fmt.Sprintf("壳件里出现按名名单/特殊化标记：%s", mk))
		}
	}
	return problems
}

// gchProbeBash —— py `probe_bash()`：`bash --version` 首行取版本元组。
func gchProbeBash() ([]int, bool) {
	out, ok := gchRunCmd("bash", "--version")
	if !ok {
		return nil, false
	}
	first := ""
	if lines := strings.Split(gchPySplitLines(string(out)), "\n"); len(lines) > 0 {
		first = lines[0]
	}
	if first == "" {
		return nil, false
	}
	return gchParseVersion(first)
}

// gchProbePython —— py `probe_python()`（= `tuple(sys.version_info[:3])`）。
// 本件无解释器语境 ⇒ exec `PATH` 上 `python3`（见件头未覆盖面①）。
func gchProbePython() ([]int, bool) {
	out, ok := gchRunCmd("python3", "-c", `import sys;sys.stdout.write("%d.%d.%d"%sys.version_info[:3])`)
	if !ok {
		return nil, false
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil, false
	}
	parts := strings.Split(s, ".")
	var v []int
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		v = append(v, n)
	}
	if len(v) == 0 {
		return nil, false
	}
	return v, true
}

// gchParseVersion —— py `parse_version(text)`：取第一段数字点号版本 ⇒ (major, minor[,…])；取不到 ⇒ nil。
func gchParseVersion(text string) ([]int, bool) {
	if text == "" {
		return nil, false
	}
	m := gchVersionRE.FindStringSubmatch(text)
	if m == nil {
		return nil, false
	}
	var out []int
	for i := 1; i <= 3; i++ {
		if m[i] == "" {
			continue
		}
		n, _ := strconv.Atoi(m[i])
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// gchScanRoster —— 现读名录（名 ⇒ 仓相对路径）：递归 `scripts` 全子树 · `__pycache__` 不进。
//
// ★ 收件口径**与注册表同源**（缺口 GAP-20261012-15）：真源 = `family_gate_check.go` 的
// gateRosterAccept（`.py`/`.sh` ＋ 无后缀活门）—— 本格的「现读名录件数」是**与注册表 `count` 对拍的尺**，
// 尺与 `check-judge-registry` 的扫面必须同口径，否则注册表一收无后缀活门（B∖C ⇒ 0）这一格就会假红。
func gchScanRoster(root string) map[string][]string {
	base := filepath.Join(root, gchRosterDir)
	out := map[string][]string{}
	_ = filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "__pycache__" && p != base {
				return filepath.SkipDir
			}
			return nil
		}
		var mode os.FileMode
		if info, ierr := d.Info(); ierr == nil {
			mode = info.Mode()
		}
		name, _, ok := gateRosterAccept(p, mode)
		if !ok {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		out[name] = append(out[name], filepath.ToSlash(rel))
		return nil
	})
	return out
}

// gchRegCount —— py `int(reg.get("count", -1))`（仅注册表为 JSON 对象且 `count` 可读时成立）。
func gchRegCount(text string) (int, bool) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(text), &obj); err != nil {
		return 0, false
	}
	raw, ok := obj["count"]
	if !ok {
		return -1, true
	}
	switch v := raw.(type) {
	case float64:
		return int(v), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

// gchPrintJSON —— py `json.dumps(rep, ensure_ascii=False, indent=2)` + print 的 `\n`。
func gchPrintJSON(stdout io.Writer, rep gchReport) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep)
	fmt.Fprint(stdout, buf.String())
}

// gchSelfTest —— py `cmd_self_test()`：五档纯计算自检（文案逐字对齐 · 无子进程 ⇒ 无外部依赖）。
func gchSelfTest(stdout io.Writer) int {
	ok := true
	good := "var gateCheckInterpreter = map[string]string{\".py\": \"python3\", \".sh\": \"bash\"}\n" +
		"err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error { return nil })\n"
	bad1 := "var gateCheckInterpreter = map[string]string{\".py\": \"python3\", \".sh\": \"bash\", \"\": \"sh\"}\n" +
		"filepath.WalkDir\n"
	bad2 := "var gateCheckInterpreter = map[string]string{\".py\": \"python3\"}\n\"scripts/gates/foo\"\n"

	if len(gchCheckGoShell(good)) != 0 {
		fmt.Fprintf(stdout, "✗ 正控：合规壳件被判红：%s\n", gchPyListRepr(gchCheckGoShell(good)))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 正控：合规壳件判绿")
	}
	if len(gchCheckGoShell(bad1)) == 0 {
		fmt.Fprintln(stdout, "✗ 负控①：解释器闭集被改宽未被抓")
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控①：解释器闭集被改宽 ⇒ 现红")
	}
	bad2p := gchCheckGoShell(bad2)
	hit := false
	for _, x := range bad2p {
		if strings.Contains(x, "名件路径") || strings.Contains(x, "目录派生") || strings.Contains(x, "闭集") {
			hit = true
			break
		}
	}
	if !(len(bad2p) > 0 && hit) {
		fmt.Fprintln(stdout, "✗ 负控②：写死名件路径未被抓")
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控②：写死名件路径 ⇒ 现红")
	}
	if v, good3 := gchParseVersion("GNU bash，版本 3.2.57(1)-release (x86_64)"); !good3 || !gchEqTriple(v, 3, 2, 57) {
		fmt.Fprintln(stdout, "✗ 负控③：bash 版本解析错")
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控③：bash 版本解析对")
	}
	if _, good4 := gchParseVersion("no version here"); good4 {
		fmt.Fprintln(stdout, "✗ 负控④：取不到版本却给了值")
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控④：取不到版本 ⇒ None")
	}
	if ok {
		return 0
	}
	return 2
}

// ── 小工具 ────────────────────────────────────────────────────────────
func gchDots(v []int) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ".")
}

// gchDotsPair —— 同 `gchDots`，收定长两元组（py `MIN_TOOL` 值）。
func gchDotsPair(v [2]int) string {
	return strconv.Itoa(v[0]) + "." + strconv.Itoa(v[1])
}

// gchLt2 —— py `v[:2] < (maj, min)`（两元组字典序）。
func gchLt2(v []int, min [2]int) bool {
	if len(v) == 0 {
		return true
	}
	if v[0] != min[0] {
		return v[0] < min[0]
	}
	if len(v) < 2 {
		return true
	}
	return v[1] < min[1]
}

func gchEqTriple(v []int, a, b, c int) bool {
	return len(v) == 3 && v[0] == a && v[1] == b && v[2] == c
}

// gchRunCmd —— 跑外部只读命令：起不来（非 ExitError）⇒ (nil,false)；非零退码照收 stdout（py 口径）。
func gchRunCmd(name string, args ...string) ([]byte, bool) {
	c := exec.Command(name, args...)
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = io.Discard
	if err := c.Run(); err != nil {
		if _, isExit := err.(*exec.ExitError); !isExit {
			return nil, false
		}
	}
	return buf.Bytes(), true
}

func gchReadText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func gchAbs(p string) string {
	if p == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "."
		}
		return wd
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func gchIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// gchPySplitLines —— py `str.splitlines()` 的最小等价（`\n`/`\r\n`/`\r` 断行）。
func gchPySplitLines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// gchPySplitExt —— py `os.path.splitext` 对**文件名**的语义（前导点不算扩展名分隔符）。
func gchPySplitExt(name string) (string, string) {
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return name, ""
	}
	i := 0
	for i < len(name) && name[i] == '.' {
		i++
	}
	if dot < i {
		return name, ""
	}
	return name[:dot], name[dot:]
}

// gchSortedKeys —— py `sorted(keys)`（码点序 ⇒ ASCII/UTF-8 下与 Go 字节序同）。
func gchSortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// gchPyListRepr —— py 列表 repr（元素恒为无引号/无转义的简单串 ⇒ 单引号逐字）。
func gchPyListRepr(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = "'" + s + "'"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
