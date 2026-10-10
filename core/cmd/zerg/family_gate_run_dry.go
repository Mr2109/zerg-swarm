// family_gate_run_dry.go —— `zerg gate run --dry-run`：门跑的**干跑档**（缺口 `GAP-20260927-02` · P1 · 命令面）。
//
// 病（现读实测 · 2026-09-27 立账）：`gate run` **没有干跑面** —— `--help` 的旗标只有
// `--scope/--fast/--outdir/--step/--self-test/--only-step/--json/--verify-live/--show-log`，想先看
// 「这一趟会跑哪些步、各步什么档位/形态」只能真跑一遍（全量 ~206 步、快速档 43 步 ≈18 分钟）
// ⇒ 这是本仓「跑门贵」体感的一个直接来源。本档把「本趟将跑 vs 被跳过」在**真跑之前**摆出来。
//
// 口径（照同族 `gate check --dry-run` 的形态：只出计划件 · 零副作用；本档**更硬** —— 一个子进程都不起）：
//
//	· **步骤表真源 = 脚本自己的 `add_step` 声明**（`parseAddSteps` —— 与 `gate ls`/`gate check`/
//	  `gate explain`/`gate find`/`gate run --step` **同一份解析**）⇒ 命令面**不另写第二份步骤表**
//	  （`gate.go` 开头那条「只转发、不翻译」不下岗）；
//	· 选步规则与脚本 `build_steps`/`filter_fast` **同一规则**：scope 归属读自声明；默认 scope 集读自
//	  脚本 `DEFAULT_SCOPES`；快速档名单读自脚本 `FAST_STEPS` —— 三者都只**读**脚本、**不手抄**任何名单；
//	· **不起任何子进程**（纯文件读 + 打印）⇒ 立即返回；既不占单实例锁、也不落趟目录/日志/指纹件；
//	· 退码按**既有档**（✗ 不新造档位）：计划件 = `0`（与 `gate check --dry-run` 同档）·
//	  用法错（`--fast` 与 `--scope` 同用 / 未知 scope 值 / 步名不存在·歧义）= `2` ·
//	  脚本读不到 = `8`（与 `gate run --step` 三档同规）。
//
// 与「照实登记」的关系：本件出现干跑旗标字样 ⇒ 同批登进 `core/internal/contract/dryrun-semantics.json`
// 的门⑩ 登记面（R1 未登记件）。
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// gateRunDryFlag —— 干跑旗标（**命令面自己的**旗标；不透传给自举件 —— 它不认识它）。
const gateRunDryFlag = "--dry-run"

// gateRunWantsDryRun —— 本发是不是干跑请求：`--dry-run` 出现在 tail 的 `--` 透传口**之前**。
//
// 为什么看 `--`：`--` 之后是**透传口**（缺口 `GAP-20260927-276`）—— 那一段里的 token 是要交给
// 下游命令串的，不是本命令的干跑请求 ⇒ 只认 `--` 之前那一枚。
func gateRunWantsDryRun(tail []string) bool {
	for _, a := range tail {
		if a == "--" {
			return false
		}
		if a == gateRunDryFlag {
			return true
		}
	}
	return false
}

// gateDryRow —— 步表一行的四格（scope / 档位 / 步名 / 跳过理由）。
type gateDryRow struct{ Scope, Mode, Name, Why string }

// gateRunDryRun —— `gate run --dry-run` 的实现：只摆出「本趟将跑 vs 被跳过」的步表。
//
// **不起任何子进程**（`os.ReadFile` + 打印）⇒ 立即返回。退码见件头。
func gateRunDryRun(inv *invocation, stdout, stderr io.Writer, root, script string, tail []string) int {
	src, err := os.ReadFile(script)
	if err != nil {
		inv.setErr("blocked", "gate_script_unreadable", "门禁脚本读不到")
		fmt.Fprintf(stderr, "%s: 读不到门禁脚本 %s（%v）⇒ 不给结论（退码 8）\n", progName, gateScriptRel, err)
		return exitBlocked
	}
	text := string(src)
	decls, unc := parseAddStepsStrict(text)
	reportUncertainSteps(stderr, unc)

	stepName := gateStepNameFrom(tail)
	fast := gateDryTailHas(tail, "--fast")
	scope := gateDryTailValue(tail, "--scope")
	outdir := gateDryTailValue(tail, "--outdir")

	// 档位自相矛盾：`--fast` 与 `--scope` 不许同用（照脚本同一句判词 · 退码 2）。
	if fast && scope != "" {
		fmt.Fprintln(stderr, "✗ --fast 与 --scope 不许同用（快速档 = 默认集里按 FAST_STEPS 名单挑出的子集）")
		fmt.Fprintf(stderr, "%s: 档位自相矛盾 ⇒ 这一发**没跑任何步骤、也没出计划件**（退码 2）\n", progName)
		inv.setErr("usage", "fast_scope_conflict", "--fast 与 --scope 不许同用")
		return exitUsage
	}
	// `--scope` 的**值**照脚本自己的闭集（取不到闭集 ⇒ 放行，与 `gateRunPrecheckScopeValues` 同规）。
	if scope != "" {
		if scopes, list, ok := gateRunScopeClosedSet(script); ok {
			known := false
			for _, s := range scopes {
				if s == scope {
					known = true
				}
			}
			if !known {
				fmt.Fprintf(stderr, "✗ 未知 scope: %s（可用: %s）\n", scope, list)
				fmt.Fprintf(stderr, "%s: 这一发**没跑任何步骤、也没出计划件** —— `--scope` 的值在命令面就校验（缺口 `GAP-20260928-153`）\n", progName)
				inv.setErr("usage", "scope_unknown", "未知 scope 值")
				return exitUsage
			}
		}
	}

	defaultScopes := gateDryReadWhitespaceArray(text, "DEFAULT_SCOPES")
	fastNames := gateDryReadFastSteps(text)
	fastSet := map[string]bool{}
	for _, n := range fastNames {
		fastSet[normStepName(n)] = true
	}
	inSet := func(set []string, v string) bool {
		for _, s := range set {
			if s == v {
				return true
			}
		}
		return false
	}

	// 档位字样（人面）。
	scopeWord := ""
	switch {
	case stepName != "":
		scopeWord = "单步档（--step）"
	case fast:
		scopeWord = fmt.Sprintf("快速档（FAST_STEPS %d 步）", len(fastNames))
	case scope != "":
		scopeWord = "--scope " + scope
	default:
		scopeWord = fmt.Sprintf("默认集（DEFAULT_SCOPES %d 个：%s）", len(defaultScopes), strings.Join(defaultScopes, " "))
	}

	will, skip := []gateDryRow{}, []gateDryRow{}

	// 单步档：只跑 `--step` 点名的那一步（逐字 + 不猜）。
	if strings.TrimSpace(stepName) != "" {
		d, rc := resolveGateStep(decls, stepName, stderr)
		if rc != exitOK {
			inv.setErr("usage", "step_absent", "步名缺失/不存在/歧义")
			return rc
		}
		will = append(will, gateDryRow{Scope: d.Scope, Mode: d.Mode, Name: normStepName(d.Name)})
		gateDryPrint(stdout, script, scopeWord, outdir, will, skip, len(decls))
		return exitOK
	}

	sel := defaultScopes
	if scope != "" {
		sel = []string{scope}
	}
	for _, d := range decls {
		name := normStepName(d.Name)
		inSel := inSet(sel, d.Scope)
		switch {
		case fast:
			if inSel && fastSet[name] {
				will = append(will, gateDryRow{Scope: d.Scope, Mode: d.Mode, Name: name})
			} else if inSel {
				skip = append(skip, gateDryRow{Scope: d.Scope, Mode: d.Mode, Name: name,
					Why: fmt.Sprintf("不在 FAST_STEPS 名单（快速档 %d 步）", len(fastNames))})
			} else {
				skip = append(skip, gateDryRow{Scope: d.Scope, Mode: d.Mode, Name: name,
					Why: fmt.Sprintf("scope「%s」不在本趟（快速档只取默认集内按名挑出的子集）", d.Scope)})
			}
		default:
			if inSel {
				will = append(will, gateDryRow{Scope: d.Scope, Mode: d.Mode, Name: name})
			} else {
				skip = append(skip, gateDryRow{Scope: d.Scope, Mode: d.Mode, Name: name,
					Why: fmt.Sprintf("scope「%s」不在本趟选择集（%s）", d.Scope, scopeWord)})
			}
		}
	}
	gateDryPrint(stdout, script, scopeWord, outdir, will, skip, len(decls))
	return exitOK
}

// gateDryPrint —— 计划件的人面（只打印 · 零副作用）。
func gateDryPrint(stdout io.Writer, script, scopeWord, outdir string, will, skip []gateDryRow, total int) {
	fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未跑任何步骤、未起任何子进程）")
	fmt.Fprintf(stdout, "  动作     : %s gate run %s\n", progName, gateRunDryFlag)
	fmt.Fprintf(stdout, "  档位     : %s\n", scopeWord)
	fmt.Fprintf(stdout, "  步骤真源 : %s（现读 add_step 声明 %d 条 · 不另写第二份步骤表）\n", gateScriptRel, total)
	fmt.Fprintf(stdout, "  将跑     : %d 步 · 跳过 : %d 步\n", len(will), len(skip))
	if strings.TrimSpace(outdir) != "" {
		fmt.Fprintf(stdout, "  日志落点 : %s（真跑时 results.tsv + 逐步日志 + freshness.json 落这里）\n", outdir)
	} else {
		fmt.Fprintln(stdout, "  日志落点 : （未给 --outdir：真跑时脚本自建趟目录 <TMPDIR>/zerg-gates-*）")
	}
	fmt.Fprintln(stdout, "  步表（将跑）：")
	for _, r := range will {
		fmt.Fprintf(stdout, "    %-8s %-11s %s\n", r.Scope, r.Mode, r.Name)
	}
	if len(skip) > 0 {
		fmt.Fprintln(stdout, "  被跳过：")
		for _, r := range skip {
			fmt.Fprintf(stdout, "    %-8s %-11s %s   —— %s\n", r.Scope, r.Mode, r.Name, r.Why)
		}
	}
	fmt.Fprintf(stdout, "  真跑请去掉 %s；单步读数走 `%s gate run --step <步名> --only-step`。\n", gateRunDryFlag, progName)
}

// gateDryTailHas —— tail 里有没有这一枚旗标（逐字）。
func gateDryTailHas(tail []string, flag string) bool {
	for _, a := range tail {
		if a == flag {
			return true
		}
	}
	return false
}

// gateDryTailValue —— tail 里这一枚**取值**旗标的值（多次给 ⇒ 取**最后一次**，与自举件同名旗标同规）。
func gateDryTailValue(tail []string, flag string) string {
	v := ""
	for i := 0; i < len(tail); i++ {
		if tail[i] == flag && i+1 < len(tail) {
			v = tail[i+1]
		}
	}
	return v
}

// gateDryReadWhitespaceArray —— 从脚本源里读一个 `NAME=(a b c)` 的**空白分隔**壳数组（读不到 ⇒ nil）。
// 只**读**脚本、✗ 不另抄一份名单：`DEFAULT_SCOPES` 是脚本 `build_steps` 默认集的唯一真源。
func gateDryReadWhitespaceArray(src, name string) []string {
	anchor := name + "=("
	i := strings.Index(src, anchor)
	if i < 0 {
		return nil
	}
	rest := src[i+len(anchor):]
	j := strings.Index(rest, ")")
	if j < 0 {
		return nil
	}
	return strings.Fields(rest[:j])
}

// gateDryReadFastSteps —— 读脚本 `FAST_STEPS=( "<名>" … )` 逐条（引号内取原样）。
// 取到行首独行的 `)` 为止（条目缩进 ⇒ 条目里的 `)` 不在行首、不会被当成收尾）。
func gateDryReadFastSteps(src string) []string {
	anchor := "FAST_STEPS=("
	i := strings.Index(src, anchor)
	if i < 0 {
		return nil
	}
	rest := src[i+len(anchor):]
	j := strings.Index(rest, "\n)")
	if j < 0 {
		if k := strings.Index(rest, ")"); k >= 0 {
			j = k
		} else {
			return nil
		}
	}
	out := []string{}
	for _, ln := range strings.Split(rest[:j], "\n") {
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, "\"") {
			continue
		}
		t = t[1:]
		if k := strings.LastIndex(t, "\""); k >= 0 {
			t = t[:k]
		}
		out = append(out, t)
	}
	return out
}
