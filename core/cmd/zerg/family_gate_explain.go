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
var addStepRE = regexp.MustCompile(`add_step\s+(\S+)\s+"([^"]*)"\s+(\S+)\s+"([^"]*)"\s+"([^"]*)"`)

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
	decls := parseAddSteps(string(b))
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
func parseAddSteps(src string) []gateStepDecl {
	out := []gateStepDecl{}
	for i, ln := range strings.Split(src, "\n") {
		m := addStepRE.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		out = append(out, gateStepDecl{Scope: m[1], Name: m[2], Mode: m[3], Dir: m[4], Cmd: m[5], Line: i + 1})
	}
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
