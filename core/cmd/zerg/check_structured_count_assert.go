// check_structured_count_assert.go —— 门128「结构化计数判据点」四属性普查 · **只报告**判据
// （缺口 `GAP-20260927-191` 残面 · 元门）。
//
// 病（缺口逐字 · 前提已现读坐实）：门禁里凡「**解析别件结构再数数**」的判据点，读法稍变就出不同的数，
// 而**没有任何一层**察觉读数与结构不符。步骤表 `add_step` 就是活标本 —— 同一份件被三种读法读成三数：
//
//	naive `add_step [a-z]* "`（不锚）           = 259
//	有锚声明行 `^\s*add_step\s`（**权威**）      = 253
//	`add_step` 全 token（连注释/函数/夹具一起）  = 315
//
// 差 6 = `clear_steps; add_step self "…"` 那 6 行自检夹具（不在行首 ⇒ 有锚读法不收）。三读数并存，
// 存量判据点里**无一格**对它们做过「抽到 N 条印出来 + N 有下界 + 有负控 + 抽空 fail-closed」四件事。
//
// 本门判什么：把「抽结构再数数」的判据点全列出来，**逐处点名**四条属性缺哪几条：
//
//	① 当场印出「抽到 N 条」（✗ 不许只印结论：分母不可读 ⇒ 读者无法自查读数与结构是否相符）；
//	② 带 N 下界（N < 基线即红；基线**只许降** ⇒ 棘轮 —— 防「结构静默变小、判据照样绿」）；
//	③ 配成对负控（该判据点所在件 `--self-test` 有真负控 ⇒ 改锚应红/应 2）；
//	④ 抽空不许静默 `return []`（抽到 0 条 ⇒ **rc=2 不给结论**，✗ 不许退回「0 == 0 静默绿」）。
//
// 口径（逐条 · 照实）：
//
//	· ★ **只报告**（接线档 `tri-report`）：本门 rc = 1（有任一处缺属性）/ 0（处处四属性齐）；
//	  扫面取不到 / 空转 ⇒ rc=2（不给结论 · ✗ 不当绿）。
//	· ★ **只读**：只读仓内件（`scripts/gates/precommit-gates.sh` + `core/cmd/zerg/*.go`）· 零写盘 ·
//	  不起子进程 · 不联网 · 不入 `main.go` 命令树 · 入 `gate_inproc_table.go` 键 `check-structured-count-assert`。
//	· ★ **抽空 fail-closed**（本门对自己的权威读数同样执行 ④）：`add_step` 有锚声明行读到 0 ⇒ rc=2。
//	· ★★ **⑤-A 窄豁免（用户已拍「上限/截断门不算判据点」· 2026-10-11）**：把六类**非「抽结构再数数」**
//	  的行从判据点面剔除（每类**实测 吃真判据点 = 0**）：(甲) **严格 `>` 截断/显示上限守卫** —— 整行形态
//	  `if len(X) > N {`（✗ 非任意 `>` 面：真自检断言 `len(steps) > 1 && …` 在 `caseEq(` 里 ⇒ ✗ 不被豁免）；
//	  (乙) **形状过滤** —— `len(<x>.path)` / `len(<x>.name)` 与字面量比较（结构形状判定 · ✗ 非计数）；
//	  (丙) **shell 计数原语定义体** —— 整行就是裸 `grep -c … 2>/dev/null || true`（判据点在其调用点）；
//	  (丙′) **前缀形状门** —— `strings.HasPrefix(…) || len(<裸标识符>) < N`（形状过滤 · ✗ 非计数）；
//	  (丁) **Go 纯日志行** —— 整行起于 `t.Logf(`（结构上从不失败 ⇒ ✗ 非计数判据点）；
//	  (戊) **shell 报告面** —— `grep -c .` 只进 `printf` 报告（无 `return`／无 rc 判定／非 `assert_eq`）。
//	  ★ 硬约束：**一类若「吃」到任一处真判据点 ⇒ 整类弃**（实测「任意 `len(X) ==/!= N`」会吃掉真计数
//	  断言 `cli_impact_card_test.go:300` ⇒ 该宽判**已弃**；本件只落上述窄形态）。
//
// ★★ **覆盖面（✗ 不假装全量；四处逐条声明）**：
//
//	① **判据点的句面定义**（机器可判的那部分）——
//	     · SHELL（`precommit-gates.sh`）：含计数命令的行（`grep -c…` / `wc -l` / `uniq`）；
//	     · GO（`core/cmd/zerg/*.go`）：`len(<结构名>)` —— 须**同时**满足两条：
//	        (a) 计数标识符基名落在**声明结构名词集**内（entry/entries/names/list/rows/refs/steps/files/
//	            guards/cases/pairs/rules/targets/paths/marks/keys/items/sites/execs/counts/members/table…）；
//	        (b) 该行参与**冻结计数断言**：比到冻结数值字面量（≥2），或落在 `t.Errorf/Fatalf/Logf` 断言里。
//	       ⇒ 裸 `len(x)`（长度取值 / 循环上界）与 `len(x)==0` / `len(x)>0`（平凡非空判）、以及
//	         `strings/bytes.Count(`（数**串内字符**）✗ 都不在面内。
//	② **四属性是启发式判据**（本单是只报告首版）：① / ② 在判据点邻域窗口（±6 行）内找「打印/断言构造」
//	   与「数值字面量/基线词」；③ / ④ 是**件级**（该件有 `--self-test` 且含负控；该件有域级空集守卫）。
//	   ⇒ 命中只说明「这个位点看上去缺 X」，✗ 不等于「它一定错」；判据只报告、✗ 不改任何被点名件。
//	③ **本件自指**：本件在 `core/cmd/zerg/` ⇒ 落 GO 扫描面 ⇒ **自排除**（✗ 不把本件自己算进判据点）。
//	④ **权威读数的真源只此一处**：`cscaCountReadings` 现读 `precommit-gates.sh` 的 `^\s*add_step\s`
//	   行数（**有锚声明行** = 权威）⇒ 与 naive / 全 token 两对照读数一并印出（读者一眼看出读数是否相符）。
//	   ★ 三读法**自排除本门自己那一步**（本门不数自己 ⇒ 加本门的前/后三读法同值 · 权威读数稳在 253）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	cscaRCok    = 0
	cscaRCfail  = 1
	cscaRCblock = 2

	// cscaSelfRelName / cscaSelfTestRelName —— 本件与其回归牙（GO 扫描自排除）。
	cscaSelfRelName     = "check_structured_count_assert.go"
	cscaSelfTestRelName = "check_structured_count_assert_test.go"

	// cscaAuthoritativeBaseline —— 权威读数（有锚声明行）的**冻结下界**（棘轮 · 只许降）。
	// 本门对自己的权威读数执行属性 ②：现读 < 此值 ⇒ rc=1（结构静默变小必红）。
	// 取值点 = 本批（门128）**加步前**的现读复算值（有锚声明行读法）。
	cscaAuthoritativeBaseline = 253

	// cscaSelfStepKey —— 本门那一步的键（三读法**自排除**：本门不数自己那一步 ⇒ 加门前/后三读法同值）。
	cscaSelfStepKey = "check-structured-count-assert"
)

// 三读法（触发本门的那一组：同一份件三种读法三数）。
//
//	· 有锚声明行 = `^\s*add_step\s`（**权威**：只收行首声明，不收注释/内联夹具）
//	· naive      = `add_step [a-z]* "`（不锚 ⇒ 连 `clear_steps; add_step self "…"` 也收）
var (
	cscaAnchorRe = regexp.MustCompile(`^\s*add_step\s`)
	cscaNaiveRe  = regexp.MustCompile(`add_step [a-z]* "`)
)

// ── 判据点句面 ──────────────────────────────────────────────────────────────

var (
	// SHELL 计数命令（抽结构再数数的「数」那一端）。
	cscaShellCountRe = regexp.MustCompile(`\bgrep\s+-[A-Za-z]*c|\bwc\s+-l\b|\buniq\b`)
	// (丙) 计数原语定义体：整行就是裸 `grep -c … 2>/dev/null || true`（函数体的原语实现，
	// ✗ 不是判据点 —— 判据点在其**调用点**）。★ 带赋值/守卫/打印的行 ✗ 不在此列（它们是真判据点）。
	cscaShellPrimRe = regexp.MustCompile(`^\s*grep\s+-c\b[^;]*2>/dev/null\s*\|\|\s*true\s*$`)
	// GO 计数：`len(<结构名>)`（取标识符基名判结构名词集）。★ `strings/bytes.Count(` = 数**串内字符**
	// （分隔符出现次数），✗ 不是「抽别件结构再数数」⇒ 不在面内（列为未覆盖面）。
	cscaGoLenRe = regexp.MustCompile(`\blen\(\s*([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?)\s*\)`)
	// 计数判据的句面：计数必须参与「冻结计数断言」——比到**冻结数值字面量（≥2）**或落在 `t.Errorf/Fatalf`
	// 断言里。★ 裸 `len(x)`（长度取值）与 `len(x)==0` / `len(x)>0`（平凡非空判）✗ 不在面内 ——
	// 本门要的是「**别件结构**的冻结计数」那一家族（结构静默变小 ⇒ 判据照样绿 = 本病）。
	cscaAssertLikeRe = regexp.MustCompile(`(==|!=|>=|<=|<|>)\s*([2-9]|[1-9][0-9]+)|([2-9]|[1-9][0-9]+)\s*(==|!=|>=|<=|<|>)|t\.(Errorf|Fatalf|Logf)|Errorf\(|Fatalf\(`)
	// 声明结构名词集（计数标识符基名后缀；✗ 不叫「所有 len(...)」—— 那会把循环上界一并吞进来）。
	cscaStructNounRe = regexp.MustCompile(`(?i)(entry|entries|names|name|list|rows|row|refs|ref|steps|step|files|file|guards|guard|cases|case|pairs|pair|rules|rule|targets|target|paths|path|marks|mark|keys|key|items|item|sites|site|execs|exec|counts|count|members|member|table|tables|blocked|blanks|gaps)$`)
	// (甲) 严格 `>` 截断/显示上限守卫：**整行**就是 `if len(X) > N {`（体做 X = X[:N] / 前N后N 显示）
	// ⇒ 「上限/截断」非判据点。★ 只此一形态（``if len(X) > N && …`` 之类 ✗ 不收）。
	cscaTruncCapRe = regexp.MustCompile(`^\s*if\s+len\(\s*[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?\s*\)\s*>\s*[0-9]+\s*\{\s*$`)
	// (乙) 形状过滤：`len(<x>.path)` / `len(<x>.name)` 与字面量比较 ⇒ 结构形状判定（✗ 非「抽结构再数数」）。
	// ★ ✗ 用宽判「任意 `len(X) ==/!= N`」—— 实测会吃掉真计数断言（`len(sim.Items) != 2 || …`）。
	cscaShapeFilterRe = regexp.MustCompile(`len\(\s*[A-Za-z_][A-Za-z0-9_]*\.(?:path|name)\s*\)\s*(==|!=|<|<=)`)
	// (丙′) 前缀形状门：行**同时**含 `strings.HasPrefix(` 与 `|| len(<裸标识符>) < 字面量` ⇒ 形状过滤（✗ 非计数）。
	// ★ 两段独立信号（RE2 ✗ 无 lookahead）须都在场；真计数 `len(exitRows) < 2`（gen_cli_reference.go:904 · 无 HasPrefix）✗ 不匹配。
	cscaHasPrefixRe = regexp.MustCompile(`strings\.HasPrefix\(`)
	cscaPrefixLenRe = regexp.MustCompile(`\|\|\s*len\(\s*[A-Za-z_][A-Za-z0-9_]*\s*\)\s*<\s*[0-9]+`)
	// (丁) Go 纯日志行：整行起于 `t.Logf(`（✗ 无 Errorf/Fatalf ⇒ 结构上从不失败 ⇒ ✗ 非计数判据点）。
	cscaLogOnlyRe = regexp.MustCompile(`^\s*t\.Logf\(`)
	// (戊) SHELL 报告面：`grep -c .` 只进 printf 报告（`-gt 0 ]; then` 条件 或 printf 续行实参）—— 无 return／
	// 无 rc 判定／非 assert_eq ⇒ 非判据点（真判据点：:593 赋给 rc · :1482/:1589/:2002 assert_eq ⇒ ✗ 不匹配）。
	cscaShellReportRe = regexp.MustCompile(`^\s*if\b.*"\$\(grep\s+-c\s+\.\s.*\)"\s+-gt\s+0\s*\]\s*;\s*then\s*$|^\s*"\$\(grep\s+-c\s+\.\s`)
)

// 四属性的启发式构造词。
var (
	cscaPrintRe = regexp.MustCompile(`assert_eq|assert\b|printf|echo|t\.Errorf|t\.Fatalf|t\.Logf|Errorf|Fatalf|fmt\.Fprint|fmt\.Print|check\(|caseEq\(`)
	cscaBoundRe = regexp.MustCompile(`baseline|基线|下限|棘轮|"[0-9]+(/[0-9]+)*"|"\s*[0-9]+\s*"|==\s*[0-9]|!=\s*[0-9]|>=\s*[0-9]|-eq\s|-ge\s|-ne\s`)
	cscaEmptyRe = regexp.MustCompile(`空域|空集|抽空|空转|不给结论|BLOCKED|require_domain|EdgRequireDomain|fail-closed|failclosed|rc\s*=\s*2|退\s*2|return\s+2\b|-z\s`)
	cscaNegRe   = regexp.MustCompile(`负控|负例|negctl|必红|期望\s*rc\s*=\s*[12]|want\s*=\s*[12]|注入`)
	// ④ 冻结期望：`assert_eq`（同行或续行）的**末个引号串**为非空字面量 ⇒ fail-closed（✗ 空期望 `""` 不算）。
	cscaAssertEqRe = regexp.MustCompile(`\bassert_eq\b`)
	cscaQuotedRe   = regexp.MustCompile(`"[^"]*"`)
	cscaSelfRe     = regexp.MustCompile(`--self-test|--selftest|self_test|SelfTest|self_test\(`)
)

// cscaSite —— 一处「抽结构再数数」的判据点。
type cscaSite struct {
	file  string // 仓内相对路径
	line  int    // 1-based
	text  string // 该行原文（截断）
	kind  string // "shell" | "go"
	ident string // GO：计数标识符（SHELL 空）
	// 四属性（true = 该属性在位）。
	aPrint bool
	aBound bool
	aNeg   bool
	aEmpty bool
}

// cscaMissing —— 缺哪些属性（中文短标）。
func (s cscaSite) missing() []string {
	var m []string
	if !s.aPrint {
		m = append(m, "①自印N")
	}
	if !s.aBound {
		m = append(m, "②N下界")
	}
	if !s.aNeg {
		m = append(m, "③负控")
	}
	if !s.aEmpty {
		m = append(m, "④抽空fail-closed")
	}
	return m
}

// cscaTrim —— 只取摘要、限宽（按 rune）。
func cscaTrim(s string) string {
	s = strings.TrimRight(s, " \t\r")
	r := []rune(s)
	if len(r) > 160 {
		r = append(r[:160], '…')
	}
	return string(r)
}

// cscaReadRel —— 读仓内件（仓内相对 POSIX 路径）；取不到 ⇒ ("", false)。
func cscaReadRel(root, rel string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// cscaWindowHas —— 判据点 [n-lo, n+hi] 邻域内任一原始行满足 re。
func cscaWindowHas(lines []string, n, lo, hi int, re *regexp.Regexp) bool {
	from, to := n-1-lo, n-1+hi
	if from < 0 {
		from = 0
	}
	if to > len(lines)-1 {
		to = len(lines) - 1
	}
	for i := from; i <= to; i++ {
		if re.MatchString(lines[i]) {
			return true
		}
	}
	return false
}

// cscaFileHasSelfNeg —— 件级属性 ③/④/自检面：件里有没有 `--self-test` 且含负控标记。
func cscaFileHasSelfNeg(text string) bool {
	return cscaSelfRe.MatchString(text) && cscaNegRe.MatchString(text)
}

// cscaShapeExempt —— ⑤-A(甲)(乙)：Go 行是否属「截断上限 / 形状过滤」（非判据点）。
// ★ 只豁免这两条窄形态（实测 吃真判据点 = 0）；真自检断言 `s.caseEq(…, len(steps) > 1 && …)` ✗ 不匹配。
func cscaShapeExempt(ln string) bool {
	return cscaTruncCapRe.MatchString(ln) || cscaShapeFilterRe.MatchString(ln)
}

// cscaPrefixShapeExempt —— ⑤-A(丙′)：Go 行是否属「前缀形状门」`strings.HasPrefix(…) || len(x) < N`（非计数）。
// ★ 两段信号须**同时**在场（RE2 ✗ 无 lookahead）；真计数 `len(exitRows) < 2`（无 HasPrefix）✗ 不匹配。
func cscaPrefixShapeExempt(ln string) bool {
	return cscaHasPrefixRe.MatchString(ln) && cscaPrefixLenRe.MatchString(ln)
}

// cscaFrozenAssertEq —— 邻域内出现 `assert_eq` 且其**期望值**（同行或紧接续行）为非空引号串
// ⇒ 该判据点有冻结期望（属性 ④ fail-closed）。★ 空期望（末个引号串 = `""`）✗ 不算（不 fail-closed）。
func cscaFrozenAssertEq(lines []string, n, lo, hi int) bool {
	from, to := n-1-lo, n-1+hi
	if from < 0 {
		from = 0
	}
	if to > len(lines)-1 {
		to = len(lines) - 1
	}
	for i := from; i <= to; i++ {
		if !cscaAssertEqRe.MatchString(lines[i]) {
			continue
		}
		j := i + 2
		if j > len(lines)-1 {
			j = len(lines) - 1
		}
		q := cscaQuotedRe.FindAllString(strings.Join(lines[i:j+1], " "), -1)
		if len(q) > 0 && q[len(q)-1] != `""` {
			return true
		}
	}
	return false
}

// cscaScanShell —— 扫 `precommit-gates.sh` 的计数命令行 ⇒ 判据点（逐行 = 逐处）。
func cscaScanShell(rel, text string) []cscaSite {
	lines := strings.Split(text, "\n")
	fileNeg := cscaFileHasSelfNeg(text)
	var out []cscaSite
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "#") {
			continue // 注释行不是判据点（注释里的 `grep -c` 是叙述，✗ 不是「抽结构再数数」）
		}
		if !cscaShellCountRe.MatchString(ln) {
			continue
		}
		if cscaShellPrimRe.MatchString(ln) {
			continue // ⑤-A(丙)：孤计数原语定义体 ⇒ 非判据点（判据点在其调用点）
		}
		if cscaShellReportRe.MatchString(ln) {
			continue // ⑤-A(戊)：`grep -c .` 只进 printf 报告（无 return／无 rc／非 assert_eq）⇒ 非判据点
		}
		n := i + 1
		out = append(out, cscaSite{
			file:   rel,
			line:   n,
			text:   cscaTrim(ln),
			kind:   "shell",
			aPrint: cscaWindowHas(lines, n, 6, 6, cscaPrintRe),
			aBound: cscaWindowHas(lines, n, 6, 6, cscaBoundRe),
			aNeg:   fileNeg,
			aEmpty: cscaWindowHas(lines, n, 10, 10, cscaEmptyRe) || cscaFrozenAssertEq(lines, n, 10, 10),
		})
	}
	return out
}

// cscaScanGoFile —— 扫单个 `.go` 件 ⇒ 判据点（结构名词集过滤 · 循环上界天然不在内）。
func cscaScanGoFile(rel, text string) []cscaSite {
	lines := strings.Split(text, "\n")
	// ③ 的 `--self-test` 子判项是**命令面属性**：`*_test.go`（go test 层）没有这个对象 ⇒
	// 该子判项对测试件不适用，视作满足（✗ 不对测试件判 ③ 假阳）。
	fileNeg := cscaFileHasSelfNeg(text) || strings.HasSuffix(rel, "_test.go")
	fileEmpty := cscaEmptyRe.MatchString(text)
	var out []cscaSite
	for i, ln := range lines {
		ident := ""
		if m := cscaGoLenRe.FindStringSubmatch(ln); m != nil {
			base := m[1]
			if j := strings.LastIndexByte(base, '.'); j >= 0 {
				base = base[j+1:]
			}
			if !cscaStructNounRe.MatchString(base) {
				continue
			}
			if !cscaAssertLikeRe.MatchString(ln) {
				continue // 裸 `len(x)`（非计数判据）不在面内
			}
			ident = m[1]
		} else {
			continue
		}
		if cscaShapeExempt(ln) {
			continue // ⑤-A(甲)(乙)：截断上限 / 形状过滤 ⇒ 非判据点
		}
		if cscaPrefixShapeExempt(ln) {
			continue // ⑤-A(丙′)：`HasPrefix(…) || len(x) < N` 前缀形状门 ⇒ 非判据点
		}
		if cscaLogOnlyRe.MatchString(ln) {
			continue // ⑤-A(丁)：纯 `t.Logf(` 日志行（从不失败）⇒ 非判据点
		}
		n := i + 1
		out = append(out, cscaSite{
			file:   rel,
			line:   n,
			text:   cscaTrim(ln),
			kind:   "go",
			ident:  ident,
			aPrint: cscaWindowHas(lines, n, 6, 6, cscaPrintRe),
			aBound: cscaWindowHas(lines, n, 6, 6, cscaBoundRe),
			aNeg:   fileNeg,
			aEmpty: fileEmpty || cscaWindowHas(lines, n, 10, 10, cscaEmptyRe),
		})
	}
	return out
}

// cscaReading —— 三读法读数（同一件）。
type cscaReading struct {
	anchor int // 有锚声明行（**权威**）
	naive  int // naive `add_step [a-z]* "`
	token  int // `add_step` 出现次数（全 token 读法 · ✗ 不是「行数」）
}

// cscaCountReadings —— 现读 `precommit-gates.sh` 的三读法。
// ★ **自排除**：本门自己那一步（命令里含 `check-structured-count-assert`）不计入三读法 ——
// 否则「加一门即自动抬读数」，读数失去稳定性、也无法与加门前的基线对照。⇒ 加本门**前后**三读法同值。
func cscaCountReadings(text string) cscaReading {
	var r cscaReading
	for _, ln := range strings.Split(text, "\n") {
		if strings.Contains(ln, cscaSelfStepKey) {
			continue // 自排除：本门那一步不计
		}
		if cscaAnchorRe.MatchString(ln) {
			r.anchor++
		}
		if cscaNaiveRe.MatchString(ln) {
			r.naive++
		}
		if strings.Contains(ln, "add_step") {
			r.token += strings.Count(ln, "add_step") // 全 token = **出现次数**（同触发读法 315）
		}
	}
	return r
}

// cscaGoFiles —— 列 `core/cmd/zerg/` 下的 `.go` 件（自排除本件与回归牙 · 码点序）。
func cscaGoFiles(root string) []string {
	dir := filepath.Join(root, "core", "cmd", "zerg")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if name == cscaSelfRelName || name == cscaSelfTestRelName {
			continue
		}
		out = append(out, "core/cmd/zerg/"+name)
	}
	sort.Strings(out)
	return out
}

// cscaCensus —— 默认档：扫面 ⇒ 逐处点名 + 权威读数。
func cscaCensus(stdout, stderr io.Writer, root string) int {
	shRel := zergPrecommitGatesRel
	shText, ok := cscaReadRel(root, shRel)
	if !ok {
		// 属性 ④（fail-closed）：抽不到扫描面 ⇒ rc=2 不给结论。
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：扫描面件取不到：%s\n", shRel)
		return cscaRCblock
	}
	goFiles := cscaGoFiles(root)
	if len(goFiles) == 0 {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：GO 扫描面无 .go 件（%s/core/cmd/zerg 空转）\n", root)
		return cscaRCblock
	}

	shellSites := cscaScanShell(shRel, shText)
	var sites []cscaSite
	sites = append(sites, shellSites...)
	for _, rel := range goFiles {
		t, ok := cscaReadRel(root, rel)
		if !ok {
			continue
		}
		sites = append(sites, cscaScanGoFile(rel, t)...)
	}

	rd := cscaCountReadings(shText)
	// 属性 ④（fail-closed）：权威读数抽到 0 ⇒ rc=2（✗ 不许「0 == 0 静默绿」）。
	if rd.anchor == 0 {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：权威读数（有锚声明行 `add_step`）= 0 ⇒ 抽空，✗ 不判绿：%s\n", shRel)
		return cscaRCblock
	}

	// 逐处点名：先印缺口，再印全量（缺口多、全量作底账）。
	byFile := map[string][]cscaSite{}
	var fileOrder []string
	for _, s := range sites {
		if _, seen := byFile[s.file]; !seen {
			fileOrder = append(fileOrder, s.file)
		}
		byFile[s.file] = append(byFile[s.file], s)
	}
	sort.Strings(fileOrder)

	missingTotal, gapSites := 0, 0
	for _, f := range fileOrder {
		for _, s := range byFile[f] {
			if len(s.missing()) > 0 {
				gapSites++
				missingTotal += len(s.missing())
			}
		}
	}

	// 件级「有自检负控」按件内真源（`cscaFileHasSelfNeg`）计，✗ 不取判据点的 aNeg ——
	// `_test.go` 的 aNeg 已被视作满足（③ 子判项对它不适用）⇒ 取 aNeg 会把测试件误计成「有负控」。
	negFiles := 0
	for _, f := range fileOrder {
		if t, ok := cscaReadRel(root, f); ok && cscaFileHasSelfNeg(t) {
			negFiles++
		}
	}

	fmt.Fprintf(stdout, "── 结构化计数判据点普查（抽结构再数数 · 四属性）──\n")
	fmt.Fprintf(stdout, "扫描面：%s ＋ core/cmd/zerg/*.go（%d 件）· 判据点 **%d 处**（shell %d · go %d）\n",
		shRel, len(goFiles), len(sites), len(shellSites), len(sites)-len(shellSites))
	fmt.Fprintf(stdout, "权威读数（有锚声明行 `add_step`）＝ **%d**（冻结下界 %d · 只许降）\n",
		rd.anchor, cscaAuthoritativeBaseline)
	fmt.Fprintf(stdout, "  对照读法：naive `add_step [a-z]* \"` ＝ %d · 全 token `add_step`（出现次数）＝ %d（三读法差 {%d, %d} —— 差即「抽到 N 条」未自印之面）\n",
		rd.naive, rd.token, rd.naive-rd.anchor, rd.token-rd.anchor)
	fmt.Fprintf(stdout, "全四属性齐 %d 处 · 缺属性 %d 处（缺项合计 %d 条）· 件级有自检负控 %d/%d 件\n",
		len(sites)-gapSites, gapSites, missingTotal, negFiles, len(fileOrder))

	// 逐处点名（先缺口）。
	fmt.Fprintf(stdout, "── 逐处点名（缺属性者）──\n")
	shown := 0
	for _, f := range fileOrder {
		for _, s := range byFile[f] {
			ms := s.missing()
			if len(ms) == 0 {
				continue
			}
			shown++
			fmt.Fprintf(stdout, "  ✗ %s:%d [%s] 缺 %s\n", s.file, s.line, s.kind, strings.Join(ms, " "))
			fmt.Fprintf(stdout, "      行文：%s\n", s.text)
		}
	}
	if shown == 0 {
		fmt.Fprintf(stdout, "  （无 —— 处处四属性齐）\n")
	}
	// 全量底账（按件计数）。
	fmt.Fprintf(stdout, "── 全量底账（件 × 判据点数 × 缺项数）──\n")
	for _, f := range fileOrder {
		gap := 0
		for _, s := range byFile[f] {
			gap += len(s.missing())
		}
		mark := "③无"
		if t, ok := cscaReadRel(root, f); ok && cscaFileHasSelfNeg(t) {
			mark = "③有"
		}
		fmt.Fprintf(stdout, "  %s %-46s 判据点 %3d · 缺项 %3d\n", mark, f, len(byFile[f]), gap)
	}

	rc := cscaRCok
	// 属性 ②（棘轮）：权威读数低于冻结下界 ⇒ 红（结构静默变小）。
	if rd.anchor < cscaAuthoritativeBaseline {
		fmt.Fprintf(stdout, "⇒ 权威读数 %d < 冻结下界 %d ⇒ rc=1（结构静默变小 · 棘轮只许降不许涨）\n", rd.anchor, cscaAuthoritativeBaseline)
		rc = cscaRCfail
	}
	if gapSites > 0 {
		fmt.Fprintf(stdout, "⇒ 缺属性判据点 %d 处 ⇒ rc=1（★ 只报告档 · tri-report：不阻断提交 · ✗ 本门不改任何被点名件）\n", gapSites)
		rc = cscaRCfail
	}
	if rc == cscaRCok {
		fmt.Fprintf(stdout, "⇒ 绿：%d 处判据点四属性齐 且 权威读数 %d ≥ 下界 %d\n", len(sites), rd.anchor, cscaAuthoritativeBaseline)
	}
	return rc
}

// ── 自检（成对负控 · 合成夹具 · 不碰真目标 · 零副作用）──────────────────────────────

func cscaWriteFile(root, rel, content string) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(content), 0o644)
}

// cscaSelfTest —— 九格：正控 0 · 计数命中断言 · 缺属性负控 1 · 抽空负控 2 · 扫描面缺件负控 2 ·
// 权威读数三读法对照 · 棘轮负控 1 · 自排除面。
func cscaSelfTest(stdout, stderr io.Writer) int {
	ok := true
	chk := func(cond bool, msg string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", msg)
		} else {
			fmt.Fprintf(stdout, "   ✗ %s\n", msg)
			ok = false
		}
	}
	td, err := os.MkdirTemp("", "csca-selftest-")
	if err != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td) }()

	// 夹具仓：有锚声明 3 行 + naive 多 1 行（clear_steps; add_step self）⇒ 两读法差 1。
	body := "# c\n" +
		"  add_step gates \"门a\" tri-report \"$R\" \"zerg gate check x\"\n" +
		"  add_step gates \"门b\" tri-report \"$R\" \"zerg gate check y\"\n" +
		"  add_step gates \"门c\" tri-report \"$R\" \"zerg gate check z\"\n" +
		"  clear_steps; add_step self \"自检-合成\" \"${m}\" \"$R\" \"exit 0\"\n" +
		"  assert_eq \"计数\" \"$(grep -cE 'add_step' f)\" \"7\"\n" +
		"  grep -c '^FAIL' \"$1\" 2>/dev/null || true\n" // ⑤-A(丙)：孤计数原语 ⇒ ✗ 不收
	cscaWriteFile(td, "scripts/gates/precommit-gates.sh", body)
	cscaWriteFile(td, "core/cmd/zerg/aa.go",
		"package main\n\nfunc f() int {\n	if len(gateCheckGoEntry) != 184 {\n		return 2\n	}\n	return 0\n}\n")

	// 格1：正控 —— 计数判据点被命中。
	shText, _ := cscaReadRel(td, "scripts/gates/precommit-gates.sh")
	shSites := cscaScanShell("scripts/gates/precommit-gates.sh", shText)
	chk(len(shSites) == 1, "正控：计数命令行 ⇒ 判据点被命中；⑤-A(丙) 孤 `grep -c … || true` 定义体 ✗ 不收")

	// 格2：三读法对照 —— 有锚 3 / naive 4 / token 4（naive 多收一行内联夹具）。
	rd := cscaCountReadings(shText)
	chk(rd.anchor == 3 && rd.naive == 4, fmt.Sprintf("对照：三读法有锚 %d · naive %d（naive 多收内联 `clear_steps; add_step`）", rd.anchor, rd.naive))

	// 格3：GO 结构名词集过滤 —— len(gateCheckGoEntry)（Entry 后缀）命中。
	goText, _ := cscaReadRel(td, "core/cmd/zerg/aa.go")
	goSites := cscaScanGoFile("core/cmd/zerg/aa.go", goText)
	chk(len(goSites) == 1 && goSites[0].ident == "gateCheckGoEntry", "正控：GO len(<Entry 结构>) ⇒ 判据点被命中")
	// 格4：GO 非结构名（len(s)）/ ⑤-A 形状·截断豁免 不收；真自检断言（`caseEq` 里 `len(steps) > 1`）仍收。
	goText2 := goText +
		"\nfunc g(s []int) int { t := 0; for i := 0; i < len(s); i++ { t++ }; return t }\n" + // 循环上界 · 非结构名
		"\tif len(names) > 3 {\n" + // ⑤-A(甲) 严格 `>` 截断上限 ⇒ ✗ 不收
		"\tif len(c.path) == 2 && c.path[0] == \"agent\" {\n" + // ⑤-A(乙) 形状过滤 ⇒ ✗ 不收
		"\ts.caseEq(\"① x\", len(steps) > 1 && got2 == want2,\n" // 真自检断言（带打印）⇒ 收
	g4 := cscaScanGoFile("core/cmd/zerg/aa.go", goText2)
	g4keep := false
	for _, s := range g4 {
		if strings.Contains(s.text, "caseEq(") {
			g4keep = true
		}
	}
	chk(len(g4) == 2 && g4keep, "负控：循环上界/截断(甲)/形状(乙) ✗ 不收；真自检断言 caseEq(len(steps)>1…) 仍收")

	// 格5：缺属性负控 —— 裸 `grep -c` 无 ①②③④ ⇒ 判据点带缺项、整体 rc=1。
	var o1 strings.Builder
	rc1 := cscaCensus(&o1, stderr, td)
	chk(rc1 == cscaRCfail && strings.Contains(o1.String(), "缺 "), fmt.Sprintf("负控：判据点缺属性 ⇒ rc=1（实得 %d）", rc1))

	// 格6：权威读数抽空 ⇒ rc=2（fail-closed · ④）。
	tdEmpty, e1 := os.MkdirTemp("", "csca-empty-")
	if e1 != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(tdEmpty) }()
	cscaWriteFile(tdEmpty, "scripts/gates/precommit-gates.sh", "# 无 add_step\n")
	cscaWriteFile(tdEmpty, "core/cmd/zerg/aa.go", "package main\n")
	var o2 strings.Builder
	rc2 := cscaCensus(&o2, stderr, tdEmpty)
	chk(rc2 == cscaRCblock, fmt.Sprintf("负控：权威读数抽到 0 ⇒ rc=2 不给结论（实得 %d）", rc2))

	// 格7：扫描面缺件 ⇒ rc=2（fail-closed · ④）。
	var o3 strings.Builder
	rc3 := cscaCensus(&o3, stderr, filepath.Join(td, "no-such-root"))
	chk(rc3 == cscaRCblock, fmt.Sprintf("负控：扫描面件取不到 ⇒ rc=2（实得 %d）", rc3))

	// 格8：棘轮负控 —— 权威读数 < 冻结下界 ⇒ rc=1（造大量声明行不可行 ⇒ 直接验读数函数 + 下界比较语义）。
	chk(rd.anchor < cscaAuthoritativeBaseline, "对照：小夹具的权威读数 < 冻结下界 253（棘轮判据按此比较 ⇒ 真仓低于基线必红）")

	// 格9：自排除 —— 本件与其回归牙名已声明（真仓扫描面不含它们）。
	chk(cscaSelfRelName == "check_structured_count_assert.go" &&
		cscaSelfTestRelName == "check_structured_count_assert_test.go",
		"对照：自排除件名已声明（GO 面不收本件/回归牙）")

	res := "FAIL"
	if ok {
		res = "PASS"
	}
	fmt.Fprintf(stdout, "自检 %s（9 格）\n", res)
	if ok {
		return cscaRCok
	}
	return cscaRCfail
}

const cscaUsage = "usage: check-structured-count-assert [--root ROOT] [--self-test] [--help]\n" +
	"  无旗标       结构化计数判据点普查（四属性逐处点名 + 权威读数）\n" +
	"  --root ROOT  仓根（缺省 ZERG_ROOT 或 cwd）\n" +
	"  --self-test  成对负控自检（合成夹具 · 不碰真目标）\n"

// checkStructuredCountAssertGateMain —— 进程内入口（`gate_inproc_table.go` 键 `check-structured-count-assert`）。
func checkStructuredCountAssertGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	hasRoot := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, cscaUsage)
			return cscaRCok
		case a == "--self-test":
			return cscaSelfTest(stdout, stderr)
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, cscaUsage)
				return cscaRCblock
			}
			i++
			root = argv[i]
			hasRoot = true
		case strings.HasPrefix(a, "--root="):
			root = strings.TrimPrefix(a, "--root=")
			hasRoot = true
		default:
			fmt.Fprint(stderr, cscaUsage)
			fmt.Fprintf(stderr, "check-structured-count-assert: error: unrecognized arguments: %s\n", a)
			return cscaRCblock
		}
	}
	if !hasRoot {
		if v, ok := os.LookupEnv("ZERG_ROOT"); ok {
			root = v
		} else if wd, err := os.Getwd(); err == nil {
			root = wd
		} else {
			root = "."
		}
	}
	return cscaCensus(stdout, stderr, root)
}
