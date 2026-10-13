// family_gate_explain.go —— `zerg gate explain <步名>`（§四 D6 · 缺口-命令面-20260921 §十一 P0-4）。
//
// 为什么它排 P0：今天读「这一步判什么」靠 `sed -n '/^# 退出码：/p' precommit-gates.sh`（**5 行**），
// 而 `gate show` **只给命令串** —— 实测 `zerg gate show "门⑪"` ⇒ 只输出
// `python3 scripts/gates/check-cli-contract.py`，`scope/mode/判据/出处` 一个都不给。
// 命令化之后：`dev verify` 证据单的 `criterion` 一列有真源、文档-命令一致性门能把「判据」
// 也纳入比对、AI 判「这一步为什么红」不必读源码。
//
// 口径（照 §十一 P0-4 的形态，不自造）：
//
//	形态 `zerg gate explain <步名> [--json <字段>]`
//	输出 `scope/mode/判据(脚本自述)/退码口径/日志路径/出处文件:行`
//	退码 `0` / `2` 步名不存在或歧义
//
// ★ **精确匹配**（针对 `B-4` 那条实据：`gate show` 的名字匹配是**子串** ⇒ 「精确名查不到、
// 模糊名给一大坨」）：本命令拿 `add_step` 行里的名字做**逐字**比对；重名（歧义）⇒ 退 2 并
// 逐条列出 —— 判据要的是**一个**答案，不是一坨。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// gateStepDecl —— 脚本里一行 `add_step <scope> "<名>" <模式> "<目录>" "<命令>"` 的声明。
type gateStepDecl struct {
	Scope, Name, Mode, Dir, Cmd string
	Line                        int
	// ★ GAP-20260929-28（门面「看得见」那一半）：第五格**三分类** + 变量/命令替换**坐实值集**。
	//   Kind 取闭集三值 `字面命令` / `变量间接` / `命令替换`（见 gateCommandKind）；CmdName = 变量名或函数名。
	//   Values = 变量间接形态**坐实到的可能值集**（空 = 坐不实 ⇒ 该声明行另进 unc，明确报 UNCERTAIN）。
	Kind    string
	CmdName string
	Values  []gateCmdValue
}

// gateCmdValue —— 坐实到的一个可能值（值文本 + 它的**定义行**，行号 1 起）。
type gateCmdValue struct {
	Text    string
	DefLine int
}

// addStepRE —— `add_step` 声明的四种引法（名字与命令必带引号；scope/mode 不带）。
// 口径：脚本里**逐字**怎么写，这里就怎么读（不解析变量 —— 读不出的位置明说「脚本里是变量」）。
//
// ★ 第五格**必须吃下内层转义引号**（本批量修的唯一一处解析真源）：
//
//	旧形态 `"([^"]*)"` 在命令串里第一个 `\"` 的那个引号上就停 —— `[^"]` 不含 `"`，
//	而 `\"` 里**有**一个真引号字符。于是 `precommit-gates.sh:2866` 的第五格只捕到
//	`[ -d \`，**件名整段落在捕获串之外** ⇒ `gate find check-placeholder-residue.py`
//	把「已挂在这一步上的件」读成「不在步骤表里」（假阴性 · 同族另有 4 件）。
//
//	新形态 `((?:[^"\\]|\\.)*)` = 「非引号非反斜杠的普通字符」或「反斜杠 + 任意一字符（含 `\"`/`\\`）」
//	—— 逐对吃转义，等同 bash 双引号内的解析规则；捕获的是**源文形态**，故 `parseAddSteps`
//	捕获后按 `\"`→`"`、`\\`→`\` 单遍 unescape（**只认这两个**：bash 双引号里 `\n` 原样保留
//	两字符，不许一并吃掉 —— 那才是另一处静默改字）。
//
// ★ 尾部可选格：`add_step` 的签名自带**一个可选的 `<环境前缀>`**（`precommit-gates.sh` 的
//
//	`add_step() {  # add_step <scope> <名> <模式> <工作目录> <命令> [<环境前缀>] }` · 危险档 4 步给
//	`ZERG_TRUSTED_RUNNER=1`）⇒ 正则尾部**必须**吃下这**一个**可选引号串（且只吃一个）。
//	不吃 ⇒ 合法 6 格声明的尾巴非空 ⇒ 「匹配没走到行尾」那颗牙把**合法声明**误报成解析不确定。
//	★ 牙未松：第 7 个及以后的非空白尾（多出来的 token）仍落在匹配之外 ⇒ 照样报（见 parseAddStepsStrict ③）。
var addStepRE = regexp.MustCompile(`add_step\s+(\S+)\s+"([^"]*)"\s+(\S+)\s+"([^"]*)"\s+"((?:[^"\\]|\\.)*)"(?:\s+"(?:[^"\\]|\\.)*")?`)

// addStepLineRE —— 「行首看起来就是一条 add_step 声明」的**粗筛**（比 addStepRE 松得多）。
// 用途只有一处：addStepRE 吃不下、而粗筛说「这本该是一条声明」的行 ⇒ **解析不确定**，
// 要点名报出来。为什么不许静默跳过：本仓口径是「**读不到不许当健康**」
// （退码 8 那一族）—— 一条声明被静默漏掉，下游 `gate find`/`gate explain`/`gate run-step`
// 就会把「已接线的件」读成「不在步骤表里」，而**输出上看不出任何异常**（这正是本批的病）。
var addStepLineRE = regexp.MustCompile(`^\s*add_step\s`)

// gateStepUncertain —— 一条「看起来是 add_step 声明、但解析器吃不下（或吃得不完整）」的行。
//
//	Line：行号（1 起）；Why：为什么判不确定；Raw：该行原文（截 160 字，够人核对）。
type gateStepUncertain struct {
	Line int
	// ★ 本单（`GAP-20260929-28` 剩面）：`Kind` = 这一条不确定的**分类**（闭集三值，见下）——
	//   供 `gate find --uncertain` 的 `kind` 格**直读**（✗ 不再靠 `Why` 正文反推）。
	Kind string
	Why  string
	Raw  string
}

// gateUncKind* —— 一条「解析不确定」的分类闭集三值（`gate find --uncertain` 的 `kind` 格真源）。
const (
	gateUncKindParse = "解析失败"
	gateUncKindVar   = "变量间接"
	gateUncKindSubst = "命令替换"
)

func cmdGateExplain(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) == 0 || strings.TrimSpace(inv.args[0]) == "" {
		inv.setErr("usage", "missing_step_name", "缺步名")
		fmt.Fprintf(stderr, "%s: `gate explain` 要给步名（例：zerg gate explain '门⑪'）；先 `zerg gate ls` 看步名\n", progName)
		return exitUsage
	}
	want := inv.args[0]
	root := repoRoot()
	if root == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根 —— 门禁步骤表在脚本里（找 core/internal/version/version.go 失败）\n", progName)
		return exitUsage
	}
	script := filepath.Join(root, gateScriptRel)
	b, err := os.ReadFile(script)
	if err != nil {
		inv.setErr("usage", "gate_script_absent", "门禁最小入口不在")
		fmt.Fprintf(stderr, "%s: 门禁最小入口不在（%s）—— 它是自举件，就地缺席即硬错\n", progName, gateScriptRel)
		return exitUsage
	}
	decls, unc := parseAddStepsStrict(string(b))
	reportUncertainSteps(stderr, unc)
	// ★ 2026-10-05（缺口 `GAP-20261001-33` 剩余半）：步名匹配走 `normStepName`（**与权威面同源**）
	//   —— 脚本自己的 `filter_step` 两侧一律去反斜杠（`want="${want//\\/}"` · `${STEP_NAME[i]//\\/}`），
	//   而 `parseAddSteps`→`addStepUnescape` **只**还原 `\"` 与 `\\`（不还原 `\`` 与 `\$`）⇒ 两个串空间
	//   不等时，`gate ls`/`--list` 打出的含反引号/美元符步名在命令面被**逐字判「找不到」退 2**。
	//   归一后两侧同串空间：`--list` 形态与源文（带反斜杠）形态**都能**命中（对不含反斜杠的名字是恒等 ⇒
	//   既有形态逐字节不变）。`normStepName` 与 `gate run --step` 共用同一函数（不另写第二份口径）。
	hits := []gateStepDecl{}
	wantKey := normStepName(want)
	for _, d := range decls {
		if normStepName(d.Name) == wantKey {
			hits = append(hits, d)
		}
	}
	if len(hits) == 0 {
		inv.setErr("usage", "step_absent", "步名不存在")
		fmt.Fprintf(stderr, "%s: 步骤名 %q **逐字**找不到（现读脚本里的 add_step 行 %d 条）⇒ 退码 2\n", progName, want, len(decls))
		fmt.Fprintf(stderr, "口径：本命令**精确匹配**（`gate show` 的子串匹配查不到精确名 —— 见缺口稿 §十二 B-4）\n")
		for _, s := range nearestSteps(decls, want, 3) {
			fmt.Fprintf(stderr, "  像它的是：%s\n", s)
		}
		return exitUsage
	}
	if len(hits) > 1 {
		inv.setErr("usage", "step_ambiguous", "步名歧义")
		fmt.Fprintf(stderr, "%s: 步骤名 %q 在脚本里出现 %d 次 ⇒ **歧义**（判据要一个答案，不是一坨 · 退码 2）：\n", progName, want, len(hits))
		for _, d := range hits {
			fmt.Fprintf(stderr, "  %s:%d  scope=%s mode=%s 命令=%s\n", gateScriptRel, d.Line, d.Scope, d.Mode, d.Cmd)
		}
		return exitUsage
	}
	d := hits[0]
	// 步骤序号（日志文件名里的 NN）**以脚本 --list 为准**（那是运行期真序；自己按源码行数
	// 数会把 self-test 分支与别的 scope 全算进去 ⇒ 数出来的 NN 与真日志名对不上）。
	logNote, _ := gateStepLogNote(root, want)
	// ★ GAP-20260929-28：末两格 = 第五格形态与前身（变量间接解引用 / 命令替换点名）—— 人面可见。
	row := map[string]string{
		"scope":            d.Scope,
		"mode":             d.Mode,
		"criterion":        gateStepCriterion(d, string(b)),
		"exit":             gateExitLine,
		"log":              logNote,
		"source":           fmt.Sprintf("%s:%d", gateScriptRel, d.Line),
		"command":          d.Cmd,
		"verdict":          gateModeVerdict(d.Mode),
		"script_say":       gateScriptSelfSay(root, d.Cmd),
		"command_kind":     d.Kind,
		"command_resolved": gateCmdResolvedCell(d),
	}
	fmt.Fprintf(stderr, "%s: 步骤 %q 的判据讲解（真源 = 脚本自己那两行 · 本命令不另写一份步骤表）\n", progName, want)
	return listCmd(inv, stdout, stderr,
		[]string{"scope", "mode", "criterion", "verdict", "exit", "log", "source", "command", "command_kind", "command_resolved"}, []map[string]string{row})
}

// parseAddSteps 读出脚本里所有 `add_step` 声明（行号从 1 数）。
//
// 口径（同 `gate explain`/`gate find`/`gate run-step` 共用这一处，不各写一份）：
//
//	① addStepRE 吃得下 ⇒ 收；第五格按 `\"`→`"`、`\\`→`\` **单遍** unescape（源文形态 ⇒ 运行期形态）；
//	② 粗筛说「这是一条声明」而 addStepRE 吃不下 ⇒ 进「解析不确定」清单，**不许静默跳过**；
//	③ 吃下了、但匹配**没走到行尾**（尾巴还有非空白字符 —— 说明捕获串与真源不符）⇒ 同样进清单：
//	   宁可报「解析不确定」，不许拿一条截断的命令串当真源（旧形态在这里是静默的，本批量修的第二处）。
//	   ★ 行尾判据**先容掉可选的第 `<环境前缀>` 格**（`add_step` 签名的第 6 参 · 见 `addStepRE`）——
//	   合法 6 格声明的尾巴为空、✗ 不报；那颗牙只咬「**多出来的**非空白尾」（截断 / 多余 token 仍报）。
func parseAddStepsStrict(src string) ([]gateStepDecl, []gateStepUncertain) {
	out := []gateStepDecl{}
	unc := []gateStepUncertain{}
	for i, ln := range strings.Split(src, "\n") {
		loc := addStepRE.FindStringSubmatchIndex(ln)
		if loc == nil {
			if addStepLineRE.MatchString(ln) {
				unc = append(unc, gateStepUncertain{
					Line: i + 1,
					Kind: gateUncKindParse,
					Why:  "行首是 add_step，但四格引号串不闭合（命令串多半是**多行/heredoc**）⇒ 读不出这一条",
					Raw:  clipLine(ln, 160),
				})
			}
			continue
		}
		if tail := strings.TrimSpace(ln[loc[1]:]); tail != "" {
			unc = append(unc, gateStepUncertain{
				Line: i + 1,
				Kind: gateUncKindParse,
				Why:  fmt.Sprintf("第五格捕获串与行尾不符（捕获后行尾还留着 %q）⇒ 捕获串可能是截断的，不当真源", clipLine(tail, 60)),
				Raw:  clipLine(ln, 160),
			})
		}
		d := gateStepDecl{
			Scope: ln[loc[2]:loc[3]],
			Name:  addStepUnescape(ln[loc[4]:loc[5]]),
			Mode:  ln[loc[6]:loc[7]],
			Dir:   addStepUnescape(ln[loc[8]:loc[9]]),
			Cmd:   addStepUnescape(ln[loc[10]:loc[11]]),
			Line:  i + 1,
		}
		// ★ GAP-20260929-28：第五格三分类 ⇒ 变量间接坐实值集；坐不实 / 命令替换 ⇒ 明确进 unc。
		//   为什么必须在这里判：合法五格、正则吃得下的**变量间接**行既不出声、也看不见真身
		//   （`gate find unhealthy_count` 零命中 + 被读成「真没有」）—— 本批的病根就在这条静默路上。
		d.Kind, d.CmdName = gateCommandKind(d.Cmd)
		switch d.Kind {
		case gateCmdKindVarRef:
			d.Values = gateResolveVarValues(src, d.CmdName)
			if len(d.Values) == 0 {
				unc = append(unc, gateStepUncertain{
					Line: i + 1,
					Kind: gateUncKindVar,
					Why: fmt.Sprintf("第五格是**变量间接**（%s）：本脚本里坐实不出它的字面值（赋值右侧是命令替换 / 运行期拼装 / heredoc）"+
						"⇒ **解析不到**（这一步的命令内容读不出来 ⇒ ✗ 别当「步骤表里真没有」）", d.Cmd),
					Raw: clipLine(ln, 160),
				})
			}
		case gateCmdKindSubst:
			unc = append(unc, gateStepUncertain{
				Line: i + 1,
				Kind: gateUncKindSubst,
				Why: fmt.Sprintf("第五格是**命令替换**（%s）：命令串由 `%s()` 的函数体在运行期拼装，读不成一行字面命令"+
					"⇒ **解析不到**（✗ 别当「步骤表里真没有」）", d.Cmd, d.CmdName),
				Raw: clipLine(ln, 160),
			})
		}
		out = append(out, d)
	}
	return out, unc
}

// reportUncertainSteps —— 「解析不确定」清单的唯一打印口（`gate explain` / `gate find` 直接调它；
// `gate show --json` / `gate run --step` 经 parseAddSteps 调它 —— 四处共用一处，各写一份必然漂）。
// **读不到不许当健康**：吃不下就点名行号 + 原文，不许静默跳过。
func reportUncertainSteps(stderr io.Writer, unc []gateStepUncertain) {
	if len(unc) == 0 {
		return
	}
	fmt.Fprintf(stderr, "%s: ★ **解析不确定 %d 条**（这些行读不出来 ⇒ 下游「未点名」结论对这 %d 条不成立，别当健康）：\n",
		progName, len(unc), len(unc))
	for _, u := range unc {
		fmt.Fprintf(stderr, "  %s:%d  %s\n      原文：%s\n", gateScriptRel, u.Line, u.Why, u.Raw)
	}
}

// addStepUnescape —— 源文形态 ⇒ 运行期形态：**只认** `\"`→`"` 与 `\\`→`\`（单遍、不回头替换）。
//
// 为什么不写通用「反斜杠吃掉下一字符」：bash 双引号里只有美元符、反引号、双引号、反斜杠与换行前的反斜杠
// 是特义，`\n` 原样是**两个**字符。通用版会把 `python3 -c "...\n..."` 里的 `\n` 悄悄改成 `n`
// —— 那是比「截断」更坏的错（读数看着像真的）。
func addStepUnescape(s string) string {
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(s)
}

// ── ★ GAP-20260929-28：第五格**三分类** + 变量/命令替换**坐实**（门面「看得见」的真身）──────────
//
// 病根（现读）：命令串先整串算进变量、`add_step` 第五格只留变量名（`"${B8_ROSTER_CMD}"`）⇒
// `gate find unhealthy_count`（内容藏在变量里）**零命中**、「零命中」被读成「真没有」、
// `gate explain` 又只回变量名不展开；且该行**合法五格**（正则吃得下）⇒ 既不进 `unc` 清单、也不出声
// ＝**静默通过**（本批的药）。命令替换 `"$(name …)"` 同病。
//
// 治法（本处是唯一真源，`gate find`/`gate explain` 共用）：
//
//	① 三分类：字面命令 / 变量间接 / 命令替换（闭集三值）；
//	② 变量间接 ⇒ 只读本脚本扫 `VAR=` 赋值（**含多行双引号段**）**坐实到可能值集**（if/else 各一支 ⇒ 逐值都收）；
//	③ 坐实不出（右侧是命令替换 / 运行期拼装 / heredoc）或命令替换（函数体运行期拼装）⇒ 该声明行
//	   并入 `unc`（reportUncertainSteps）**明确报 UNCERTAIN** —— ✗ 不再静默过。
const (
	gateCmdKindLiteral = "字面命令"
	gateCmdKindVarRef  = "变量间接"
	gateCmdKindSubst   = "命令替换"
)

var (
	gateVarRefBraceRE = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)
	gateVarRefBareRE  = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)$`)
	gateSubstNameRE   = regexp.MustCompile(`^\$\(\s*([A-Za-z_][A-Za-z0-9_]*)`)
)

// gateCommandKind —— 第五格三分类；变量/命令替换形态另返回名字（变量名 / 函数名）。
func gateCommandKind(cmd string) (kind, name string) {
	if m := gateVarRefBraceRE.FindStringSubmatch(cmd); m != nil {
		return gateCmdKindVarRef, m[1]
	}
	if m := gateVarRefBareRE.FindStringSubmatch(cmd); m != nil {
		return gateCmdKindVarRef, m[1]
	}
	if strings.HasPrefix(cmd, "$(") {
		if m := gateSubstNameRE.FindStringSubmatch(cmd); m != nil {
			return gateCmdKindSubst, m[1]
		}
		return gateCmdKindSubst, ""
	}
	return gateCmdKindLiteral, ""
}

// gateResolveVarValues —— 只读本脚本：坐实变量 `<name>` 的赋值 ⇒ 可能值集（含多行双引号段 · 单引号段）。
// 同一变量被赋值 N 次（if/else 各一支）⇒ 逐值都收（按「定义行 + 值」去重）。
// 右侧不是可静态取值的引号串（命令替换 / 裸词拼接）⇒ 该次赋值坐不实（不收）。
func gateResolveVarValues(src, name string) []gateCmdValue {
	out := []gateCmdValue{}
	seen := map[string]bool{}
	lines := strings.Split(src, "\n")
	prefix := name + "="
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimLeft(lines[i], " 	")
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		// `=` **之后**那一列（= 前导空白 + 变量名宽度 + 1）。
		col := len(lines[i]) - len(trimmed) + len(name) + 1
		val, endLine, ok := gateReadAssignValue(lines, i, col)
		if !ok {
			continue
		}
		key := fmt.Sprintf("%d\x00%s", i+1, val)
		if !seen[key] {
			seen[key] = true
			out = append(out, gateCmdValue{Text: val, DefLine: i + 1})
		}
		i = endLine // 多行赋值：跳过它的续行
	}
	return out
}

// gateReadAssignValue —— 从 lines[start] 的 col 处起读赋值右侧：只认 `"…"` 与 `'…'` 两种引号串。
func gateReadAssignValue(lines []string, start, col int) (string, int, bool) {
	if col >= len(lines[start]) {
		return "", start, false
	}
	switch lines[start][col] {
	case '"':
		return gateReadDQuote(lines, start, col)
	case '\'':
		return gateReadSQuote(lines, start, col)
	}
	return "", start, false
}

// gateReadDQuote —— 读一个**可跨行**的 bash 双引号串（lines[start][col] == '"'）⇒ 解转义内容 + 结束行。
//
// 解转义只认 bash 双引号里的五个特义：`\"` `\\` `\$` 反引号转义 与「反斜杠 + 换行」的行延续；
// 其余 `\x` **原样留两字符**（✗ 不许一并吃掉 —— 那会把 `python3 -c "...\n..."` 里的 `\n` 静默改成 `n`，
// 与 addStepUnescape 同一条纪律）。
func gateReadDQuote(lines []string, start, col int) (string, int, bool) {
	var b strings.Builder
	line, c := start, col+1
	for {
		if c >= len(lines[line]) {
			// 行末：双引号串可跨行 ⇒ 换行是串内容的一部分（`\<newline>` 行延续由下一轮的分支吃掉）。
			if line+1 >= len(lines) {
				return "", start, false
			}
			b.WriteByte('\n')
			line++
			c = 0
			continue
		}
		ch := lines[line][c]
		if ch == '\\' {
			if c+1 >= len(lines[line]) {
				// 反斜杠 + 换行 = 行延续：两字符都不进串。
				if line+1 >= len(lines) {
					return "", start, false
				}
				line++
				c = 0
				continue
			}
			switch nxt := lines[line][c+1]; nxt {
			case '"', '\\', '$', '`':
				b.WriteByte(nxt)
			default:
				b.WriteByte('\\')
				b.WriteByte(nxt)
			}
			c += 2
			continue
		}
		if ch == '"' {
			return b.String(), line, true
		}
		b.WriteByte(ch)
		c++
	}
}

// gateReadSQuote —— 读一个**可跨行**的 bash 单引号串（无转义）⇒ 内容 + 结束行。
func gateReadSQuote(lines []string, start, col int) (string, int, bool) {
	var b strings.Builder
	line, c := start, col+1
	for {
		if c >= len(lines[line]) {
			if line+1 >= len(lines) {
				return "", start, false
			}
			b.WriteByte('\n')
			line++
			c = 0
			continue
		}
		if lines[line][c] == '\'' {
			return b.String(), line, true
		}
		b.WriteByte(lines[line][c])
		c++
	}
}

// gateValuesHay —— 坐实值集的**搜索面**（供 `gate find` 片段匹配；空值集 ⇒ 空串）。
func gateValuesHay(d gateStepDecl) string {
	if len(d.Values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(d.Values))
	for _, v := range d.Values {
		parts = append(parts, v.Text)
	}
	return strings.Join(parts, "\n")
}

// gateDefLinesNote —— 坐实值的定义行清单（形如 `L4124 · L4153/L4155`；空值集 ⇒ 空串）。
func gateDefLinesNote(d gateStepDecl) string {
	if len(d.Values) == 0 {
		return ""
	}
	ls := make([]string, 0, len(d.Values))
	for _, v := range d.Values {
		ls = append(ls, fmt.Sprintf("L%d", v.DefLine))
	}
	return strings.Join(ls, " · ")
}

// gateCmdResolvedCell —— 第五格「真身」的**人读一格**（`gate explain`/`gate find` 共用一处文案）。
func gateCmdResolvedCell(d gateStepDecl) string {
	switch d.Kind {
	case gateCmdKindVarRef:
		if len(d.Values) == 0 {
			return fmt.Sprintf("%s 坐实不出（读不出赋值 ⇒ 见「解析不确定」清单）", d.Cmd)
		}
		first := strings.ReplaceAll(d.Values[0].Text, "\n", "⏎")
		return fmt.Sprintf("%s ⇒ 定义 @%s · 可能值 %d 个（首值：%s）",
			d.Cmd, gateDefLinesNote(d), len(d.Values), clipLine(first, 80))
	case gateCmdKindSubst:
		return fmt.Sprintf("%s ⇒ 由 %s() 运行期拼装（读不出字面命令）", d.Cmd, d.CmdName)
	default:
		return "—（字面命令：第五格就是命令串本身）"
	}
}

// clipLine —— 取证行原文截断（本仓口径：截断必须**看得出来**，故补 `…`）。
func clipLine(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// parseAddSteps 读出脚本里所有 `add_step` 声明（行号从 1 数）。
//
// ★ 「读不准」的声明**不许静默丢弃**：`gate show --json` 与 `gate run --step` 三档
// （--json / --verify-live / --show-log）的步名真源都是本函数，而 `gate find`/`gate explain`
// 另走 parseAddStepsStrict。丢弃过的 unc 清单在这里**直接上报**到进程 stderr —— 与
// `gate find`/`gate explain` 同一处打印口（reportUncertainSteps），三入口同口径。
// 为什么必须上报：一条声明被静默漏掉，查询者会把「读不出的步」读成「步骤表里没有这一步」，
// 而输出上看不出任何异常 —— 本仓口径「读不到不许当健康」（退码 8 那一族）。
//
// 注：签名保持 []gateStepDecl 不变（调用点零改动，与 GAP-20260927-31 的拍板一致）；
// unc 为空时一个字都不打，正常路径的输出与退码逐字节不变。
func parseAddSteps(src string) []gateStepDecl {
	out, unc := parseAddStepsStrict(src)
	reportUncertainSteps(os.Stderr, unc)
	return out
}

// gateModeVerdict —— 四档模式各自的判据口径（**照脚本自己的四档语义写**，不第四种）。
func gateModeVerdict(mode string) string {
	switch mode {
	case "empty":
		return "empty 模式：**输出必须为空**（列出任何一行即红）—— 判据是「有没有输出」，不是退出码"
	case "rc":
		return "rc 模式：**只看真实退出码**（0 绿 / 非 0 红）—— 不看输出里有没有某个字样"
	case "tri":
		return "tri 模式：三值（0 绿 / 1 失败项 / 2 **不给结论**）—— 2 计入 BLOCKED，**不许当绿**"
	case "tri-report":
		return "tri-report 模式：只报告（rc=1 落 REPORT，既不计失败项、也不影响退出码）"
	default:
		return fmt.Sprintf("模式 %q 不在四档闭集里（rc/empty/tri/tri-report）—— 脚本会硬红；这一步的档位申报有错", mode)
	}
}

// gateStepCriterion —— 判据 = ① 模式那一档的语义 + ② 被调的脚本**自己的自述**（有就引原文首行）。
func gateStepCriterion(d gateStepDecl, src string) string {
	return fmt.Sprintf("[%s 模式] %s；命令串：%s", d.Mode, gateModeVerdict(d.Mode), d.Cmd)
}

// gateScriptSelfSay —— 若命令串里点了一个仓内脚本，引它**自己的抬头注释**（脚本自述 = 判据真源）。
func gateScriptSelfSay(root, cmd string) string {
	fields := strings.Fields(cmd)
	for _, f := range fields {
		if !strings.HasSuffix(f, ".py") && !strings.HasSuffix(f, ".sh") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			return ""
		}
		lines := []string{}
		for _, ln := range strings.Split(string(b), "\n") {
			t := strings.TrimSpace(ln)
			if strings.HasPrefix(t, "#!") {
				continue
			}
			if !strings.HasPrefix(t, "#") {
				break
			}
			t = strings.TrimSpace(strings.TrimPrefix(t, "#"))
			if t != "" {
				lines = append(lines, t)
			}
			if len(lines) >= 2 {
				break
			}
		}
		if len(lines) == 0 {
			return ""
		}
		return fmt.Sprintf("%s 自述：%s", f, strings.Join(lines, " / "))
	}
	return ""
}

// gateStepLogNote —— 「日志路径」那一格（`gate explain` 的 `log` 与 `gate show --json` 的 `log`
// **共用这一处**：两处各写一份必然漂）。
func gateStepLogNote(root, name string) (string, bool) {
	idx, inList := gateStepIndex(root, name)
	note := fmt.Sprintf("<outdir>/%02d-<步名>.log（NN = 该步在 `zerg gate ls` 默认集里的序号 %d；名里的分隔符由脚本替换成下划线）", idx, idx)
	if !inList {
		note = fmt.Sprintf("<outdir>/NN-<步名>.log（NN = 步骤序号）—— **脚本 --list 里没有这一行**（%s）："+
			"它只在某个 scope 里被声明，或名字是运行期拼的 ⇒ NN 以那次运行的清单为准，别猜", name)
	}
	return note, inList
}

// gateStepIndex —— 该步在脚本 `--list`（默认集）里的序号（1 起）与「在不在清单里」。
//
// 为什么以 `--list` 为准而不是数源码行数：整个脚本是**按 scope 分支**声明的（`--scope go`
// 只 add_step 那一支），源码行序 ≠ 运行期步序；而日志文件名 NN 用的是**运行期步序**。
// 这一条**跑脚本自己的清单**（只列不跑 —— `--list` 不执行任何步骤），不另造一份表。
func gateStepIndex(root, name string) (int, bool) {
	// ★ GAP-20261010-25（本片）：机读面优先（`--list --json`）—— 复用同族助手 `pgsParseListJSON`。
	// 机读面在场而形状坏 ⇒ **不给结论**（序号 -1 并响亮出声），✗ 不许静默退回人面。
	mcmd := armChildGroup(exec.Command("bash", filepath.Join(root, gateScriptRel), "--list", "--json"))
	runningChild = mcmd
	defer func() { runningChild = nil }()
	mcmd.Dir = root
	if mout, merr := mcmd.Output(); merr == nil {
		if steps, _, isM, perr := pgsParseListJSON(string(mout)); isM {
			if perr != "" {
				fmt.Fprintf(os.Stderr, "%s: 步表机读面（`%s --list --json`）形状坏：%s ⇒ 步序号**不给结论**（✗ 不退回人面）\n", progName, gateScriptRel, perr)
				return -1, false
			}
			idx := 0
			for _, s := range steps {
				idx++
				if normStepName(s.Name) == normStepName(name) {
					return idx, true
				}
			}
			return idx, false
		}
	}
	// ② 退化路：机读面不在 ⇒ 人面 `--list`（与改前逐字相同）。
	cmd := armChildGroup(exec.Command("bash", filepath.Join(root, gateScriptRel), "--list"))
	runningChild = cmd
	defer func() { runningChild = nil }()
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	// **只读「步骤清单」那一段**（脚本自己用 `── 步骤清单（scope 模式 名称）──` 起、
	// `共 N 步` 收尾）——不这么做就会把前面的自检输出行当成步骤（D1 那条手搓 sed 的同一口径）。
	idx, inSection := 0, false
	for _, ln := range strings.Split(string(out), "\n") {
		t := strings.TrimSpace(ln)
		if strings.Contains(t, "步骤清单") {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(t, "共 ") {
			break
		}
		// 行形如 `scope 模式 名称` —— 名称**取原文、不折叠内部空白**（切法复用 `impactStepListLineRe`：
		//   本仓有步名含连续两个 ASCII 空格，折叠会把它折成**另一个名字** · 缺口 `GAP-20261005-49`）。
		m := impactStepListLineRe.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		idx++
		// ★ 2026-10-05（缺口 `GAP-20261001-33`）：步名**归一后比** —— 脚本 `--list` 打出的名字是**运行期
		//   形态（不带反斜杠）**，而调用方可能给**源文形态**（带反斜杠的 `\``/`\$`）⇒ 不归一会把在册步
		//   误判成「`--list` 里没有这一行」（`log` 那一格错报）。与 `normStepName` 同一函数（权威面同源）。
		if normStepName(m[3]) == normStepName(name) {
			return idx, true
		}
	}
	return idx, false
}

// nearestSteps —— 找不到时的「像它的是」（前 3 个：含同一词根的优先）。
func nearestSteps(decls []gateStepDecl, want string, n int) []string {
	probe := want
	for _, r := range []string{"门", "docs: ", "（阻断）", "（只报告）"} {
		probe = strings.ReplaceAll(probe, r, "")
	}
	probe = strings.TrimSpace(probe)
	cands := []string{}
	for _, d := range decls {
		if probe != "" && strings.Contains(d.Name, probe) {
			cands = append(cands, d.Name)
		}
	}
	if len(cands) == 0 {
		for _, d := range decls {
			cands = append(cands, d.Name)
		}
	}
	sort.Strings(cands)
	if len(cands) > n {
		cands = cands[:n]
	}
	return cands
}
