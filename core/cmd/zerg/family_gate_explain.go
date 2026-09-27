// family_gate_explain.go —— `zerg gate explain <步名>`（§四 D6 · 缺口-命令面-20260921 §十一 P0-4）。
//
// 为什么它排 P0：今天读「这一步判什么」靠 `sed -n '35p' precommit-gates.sh`（**5 行**），
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
var addStepRE = regexp.MustCompile(`add_step\s+(\S+)\s+"([^"]*)"\s+(\S+)\s+"([^"]*)"\s+"((?:[^"\\]|\\.)*)"`)

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
	Why  string
	Raw  string
}

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
	hits := []gateStepDecl{}
	for _, d := range decls {
		if d.Name == want {
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
	row := map[string]string{
		"scope":      d.Scope,
		"mode":       d.Mode,
		"criterion":  gateStepCriterion(d, string(b)),
		"exit":       gateExitLine,
		"log":        logNote,
		"source":     fmt.Sprintf("%s:%d", gateScriptRel, d.Line),
		"command":    d.Cmd,
		"verdict":    gateModeVerdict(d.Mode),
		"script_say": gateScriptSelfSay(root, d.Cmd),
	}
	fmt.Fprintf(stderr, "%s: 步骤 %q 的判据讲解（真源 = 脚本自己那两行 · 本命令不另写一份步骤表）\n", progName, want)
	return listCmd(inv, stdout, stderr,
		[]string{"scope", "mode", "criterion", "verdict", "exit", "log", "source", "command"}, []map[string]string{row})
}

// parseAddSteps 读出脚本里所有 `add_step` 声明（行号从 1 数）。
//
// 口径（同 `gate explain`/`gate find`/`gate run-step` 共用这一处，不各写一份）：
//
//	① addStepRE 吃得下 ⇒ 收；第五格按 `\"`→`"`、`\\`→`\` **单遍** unescape（源文形态 ⇒ 运行期形态）；
//	② 粗筛说「这是一条声明」而 addStepRE 吃不下 ⇒ 进「解析不确定」清单，**不许静默跳过**；
//	③ 吃下了、但匹配**没走到行尾**（尾巴还有非空白字符 —— 说明捕获串与真源不符）⇒ 同样进清单：
//	   宁可报「解析不确定」，不许拿一条截断的命令串当真源（旧形态在这里是静默的，本批量修的第二处）。
func parseAddStepsStrict(src string) ([]gateStepDecl, []gateStepUncertain) {
	out := []gateStepDecl{}
	unc := []gateStepUncertain{}
	for i, ln := range strings.Split(src, "\n") {
		loc := addStepRE.FindStringSubmatchIndex(ln)
		if loc == nil {
			if addStepLineRE.MatchString(ln) {
				unc = append(unc, gateStepUncertain{
					Line: i + 1,
					Why:  "行首是 add_step，但四格引号串不闭合（命令串多半是**多行/heredoc**）⇒ 读不出这一条",
					Raw:  clipLine(ln, 160),
				})
			}
			continue
		}
		if tail := strings.TrimSpace(ln[loc[1]:]); tail != "" {
			unc = append(unc, gateStepUncertain{
				Line: i + 1,
				Why:  fmt.Sprintf("第五格捕获串与行尾不符（捕获后行尾还留着 %q）⇒ 捕获串可能是截断的，不当真源", clipLine(tail, 60)),
				Raw:  clipLine(ln, 160),
			})
		}
		out = append(out, gateStepDecl{
			Scope: ln[loc[2]:loc[3]],
			Name:  addStepUnescape(ln[loc[4]:loc[5]]),
			Mode:  ln[loc[6]:loc[7]],
			Dir:   addStepUnescape(ln[loc[8]:loc[9]]),
			Cmd:   addStepUnescape(ln[loc[10]:loc[11]]),
			Line:  i + 1,
		})
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
	cmd := exec.Command("bash", filepath.Join(root, gateScriptRel), "--list")
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
		// 行形如 `scope 模式 名称`（空白分隔）——名称可能含空格，故剥掉前两个字段再比。
		parts := strings.Fields(t)
		if len(parts) < 3 {
			continue
		}
		idx++
		if strings.Join(parts[2:], " ") == name {
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
