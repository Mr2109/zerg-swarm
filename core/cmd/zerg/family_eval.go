// family_eval.go —— `zerg eval`：评测线（`scripts/evals/` 22 件）的命令面（T-56 余项 · 2026-09-21 · DEV-0010）。
//
// 收编依据（**逐件照抄上一枚的四类归属表，一件都不自编**）：`scripts/evals/` 的 22 件
// 按 `Zerg-内部文档/项目文档/v2.5.10/归属-收编与退役-20260920.md` §④「脚本 27 条」那一组的逐件归属拆成三档：
//
//	① **收编为命令（5 件）** —— 它们本身是**可复用的判据工具**（自带用法、只读或自带 `--self-test`）⇒
//	   有命令面上的名字：`zerg eval show|run <名>`。
//	② **保留为内部实现（4 件）** —— 已经被别的入口接线在跑（`pack-release.sh` / CI / `run_all.sh`）⇒
//	   命令面**只做投影**、**不开 run 面**（开了就是两份真相）。
//	④ **维持待拍（13 件）** —— 2026-08-13 的历史一次性任务集 · 仓内零调用方 · 归属不能由稿单方面决定
//	   ⇒ 本批**不替它拍**，也不许 AI 判退役（✗）。
//
// 形态（对象 × 动作 · 2 段族 · 与 `calib` / `doc meta` / `approve` 同一条形状）：
//
//	zerg eval ls                         22 件 + 逐件归属（一行一件）
//	zerg eval show <名>                  单件的现状（归属 · 理由 · 执行面）
//	zerg eval run <名> [位置参数…] [--dry-run | --yes]    D2 三态：**真调旧脚本**（只对 ① 开放）
//
// 三条硬口径：
//
//	① **不搬实现**：执行面永远是 `bash|python3 scripts/evals/<件>` —— 22 个脚本**一个字节不改**。
//	② **退码原样转出**：脚本退多少就返多少（含异常码）。
//	③ **归属是闸**：`run` 只对 ① 开放；②/④ 的件 ⇒ 退 2 并**逐字给出为什么**（不是「不支持」，是「不归本批」）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 归属三档（命令面上的闭集 —— 与 §5.0 的四类定义逐字对应：① 收编 / ② 内部 / ④ 待定）。
const (
	evalClassAdopt    = "①"
	evalClassInternal = "②"
	evalClassPending  = "④"
)

// evalClassState —— 归属 → 人面一句话（`ls` / `show` / 计划件共用同一句，不各写一份）。
var evalClassState = map[string]string{
	evalClassAdopt:    "① 收编为命令（`zerg eval` 立族 · 2026-09-21）",
	evalClassInternal: "② 保留为内部实现（继续跑 · 不是命令名 · 命令面只做投影）",
	evalClassPending:  "④ 待定（维持待拍：归属不能由稿单方面决定 —— 本批不替它拍）",
}

// evalEntry —— 评测线的一件（命令面上的**对象**）。
type evalEntry struct {
	Name  string // 命令面上的对象名（去扩展名的件名）
	File  string // 件（仓内相对路径）
	Run   string // 执行器（`bash` / `python3`）
	Class string // 归属（①②④ 闭集）
	Why   string // 一句理由（逐字取自上一枚的归属表）
}

// evalRoster —— 评测线 22 件（**逐件归属照抄 · 一件不自编**）。
var evalRoster = []evalEntry{
	// ── ① 收编为命令（5 件）────────────────────────────────────────────────────────────
	{
		Name: "compare-wall-argv", File: "scripts/evals/compare-wall-argv.py", Run: "python3", Class: evalClassAdopt,
		Why: "迁移桥门禁：同一批卵配方两侧 argv 逐条对拍（Go `BuildBwrapArgv` ↔ Rust `zerg-wall plan`）· 判据写在文件名里 · 自带 `--self-test`（自检不过 rc=2）",
	},
	{
		Name: "replay-fidelity", File: "scripts/evals/replay-fidelity.sh", Run: "bash", Class: evalClassAdopt,
		Why: "T4.4 保真度三指标（+ 二分定位 E7）的可执行验收：跨实现复核产物 sha256 · 自检不过拒出结论",
	},
	{
		Name: "chat-harness", File: "scripts/evals/chat-harness.py", Run: "python3", Class: evalClassAdopt,
		Why: "v2.5.10 对话/卵 测试骨架：X1–X5 逐轮断言 · 只建自己的会话（不碰生产）",
	},
	{
		Name: "stability-stats", File: "scripts/evals/stability-stats.py", Run: "python3", Class: evalClassAdopt,
		Why: "T7.3 稳定性统计口径：分歧率 + Clopper-Pearson 单侧 95% 上界 · 纯 stdin→口径 · 零副作用",
	},
	{
		Name: "compare", File: "scripts/evals/compare.py", Run: "python3", Class: evalClassAdopt,
		Why: "两次 run 目录逐层通过率对拍（只读：各层通过率 + 每项 PASS/FAIL 变化）",
	},
	// ── ② 保留为内部实现（4 件）────────────────────────────────────────────────────────
	{
		Name: "make-manifest", File: "scripts/evals/make-manifest.py", Run: "python3", Class: evalClassInternal,
		Why: "发布清单生成器，**已接线**在 `scripts/build/pack-release.sh:87` + CI `release-agent.yml:160` ⇒ 收编即两份真相 ⇒ 只做投影",
	},
	{
		Name: "v25_example-35b-v2_tasks", File: "scripts/evals/v25_example-35b-v2_tasks.py", Run: "python3", Class: evalClassInternal,
		Why: "`scripts/build/run_all.sh:27` 层 2 的**在跑实现** ⇒ 它是任务集、不是命令名",
	},
	{
		Name: "v25_complex_tasks", File: "scripts/evals/v25_complex_tasks.py", Run: "python3", Class: evalClassInternal,
		Why: "`scripts/build/run_all.sh:33` 层 3 的**在跑实现** ⇒ 同上",
	},
	{
		Name: "ab-campaigns", File: "scripts/evals/ab-campaigns.py", Run: "python3", Class: evalClassInternal,
		Why: "实验编排（要**跑着的**生产对话面 + 真任务）⇒ 只做投影、不开 run 面",
	},
	// ── ④ 维持待拍（13 件 · 本批不替它拍 · 也不许 AI 判退役）─────────────────────────────
	{
		Name: "v24_full_test", File: "scripts/evals/v24_full_test.py", Run: "python3", Class: evalClassPending,
		Why: "2026-08-13 历史一次性任务集 · 仓内零调用方 · 工作区写死在临时目录 ⇒ 不该升格成契约面、又不许 AI 判退役",
	},
	{
		Name: "v25_agent_tasks", File: "scripts/evals/v25_agent_tasks.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_benchmark", File: "scripts/evals/v25_benchmark.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_eval_task", File: "scripts/evals/v25_eval_task.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_full_test", File: "scripts/evals/v25_full_test.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_m1_accept", File: "scripts/evals/v25_m1_accept.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_more_classes", File: "scripts/evals/v25_more_classes.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_more_tasks", File: "scripts/evals/v25_more_tasks.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_real10", File: "scripts/evals/v25_real10.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_real10_v2", File: "scripts/evals/v25_real10_v2.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_real_tasks", File: "scripts/evals/v25_real_tasks.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_realproj", File: "scripts/evals/v25_realproj.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
	{
		Name: "v25_research", File: "scripts/evals/v25_research.py", Run: "python3", Class: evalClassPending,
		Why: "同上（历史一次性任务集 · 零调用方）",
	},
}

// evalLsFields / evalShowFields / evalRunFields —— 三条命令的 `--json` 字段（K1：机器面先定）。
var evalLsFields = []string{"name", "script", "class", "state"}
var evalShowFields = []string{"name", "script", "class", "state", "why", "runner"}
var evalRunFields = []string{"name", "script", "class", "mode", "result", "rc", "note"}

// evalNames —— 对象名的现读清单。
func evalNames() []string {
	out := make([]string, 0, len(evalRoster))
	for _, e := range evalRoster {
		out = append(out, e.Name)
	}
	return out
}

// evalFind 按**名字逐字**找（模糊匹配等于没匹配 ⇒ 不猜）。
func evalFind(name string) *evalEntry {
	for i := range evalRoster {
		if evalRoster[i].Name == name {
			return &evalRoster[i]
		}
	}
	return nil
}

// evalClassCounts —— 逐档计数（`ls` 的收尾行用它；**现算不手写**）。
func evalClassCounts() (adopt, internal, pending int) {
	for _, e := range evalRoster {
		switch e.Class {
		case evalClassAdopt:
			adopt++
		case evalClassInternal:
			internal++
		case evalClassPending:
			pending++
		}
	}
	return adopt, internal, pending
}

// cmdEvalLs —— `zerg eval ls`：22 件逐行 + 它的归属。
func cmdEvalLs(inv *invocation, stdout, stderr io.Writer) int {
	rows := []map[string]string{}
	for _, e := range evalRoster {
		rows = append(rows, map[string]string{
			"name": e.Name, "script": e.File, "class": e.Class, "state": evalClassState[e.Class],
		})
	}
	if len(rows) == 0 {
		inv.setErr("blocked", "roster_empty", "评测线声明面为空")
		fmt.Fprintf(stderr, "%s: 评测线声明面一件都没有 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	a, i, p := evalClassCounts()
	fmt.Fprintf(stderr, "%s: 评测线 %d 件 —— ① 收编 %d · ② 保留内部 %d · ③ 退役 0（AI 不许判退役）· ④ 维持待拍 %d\n",
		progName, len(evalRoster), a, i, p)
	return listCmd(inv, stdout, stderr, evalLsFields, rows)
}

// cmdEvalShow —— `zerg eval show <名>`：单件的现状（归属 + 理由 + 执行面）。
func cmdEvalShow(inv *invocation, stdout, stderr io.Writer) int {
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if name == "" {
		return emitNameMiss(inv, stderr, "eval", "show", "", evalNames(), "件")
	}
	e := evalFind(name)
	if e == nil {
		return emitNameMiss(inv, stderr, "eval", "show", name, evalNames(), "件")
	}
	row := map[string]string{
		"name": e.Name, "script": e.File, "class": e.Class,
		"state":  evalClassState[e.Class],
		"why":    e.Why,
		"runner": e.Run + " " + e.File,
	}
	return listCmd(inv, stdout, stderr, evalShowFields, []map[string]string{row})
}

// cmdEvalRun —— `zerg eval run <名> [位置参数…] [--dry-run | --yes]`：D2 三态（**只对 ① 开放**）。
func cmdEvalRun(inv *invocation, stdout, stderr io.Writer) int {
	name := ""
	if len(inv.args) > 0 {
		name = strings.TrimSpace(inv.args[0])
	}
	if name == "" {
		return emitNameMiss(inv, stderr, "eval", "run", "", evalNames(), "件")
	}
	e := evalFind(name)
	if e == nil {
		return emitNameMiss(inv, stderr, "eval", "run", name, evalNames(), "件")
	}
	// 归属是闸：只有 ① 收编的件才在命令面上有 run 面（②/④ 照样能 `show`）。
	if e.Class != evalClassAdopt {
		inv.setErr("usage", "not_adopted", "这一件不归本批收编")
		fmt.Fprintf(stderr, "%s: `eval run` 只对 **① 收编**的件开放 —— %s 的归属是 **%s**（%s）⇒ 退码 2\n",
			progName, e.Name, e.Class, evalClassState[e.Class])
		fmt.Fprintf(stderr, "  理由：%s\n", e.Why)
		fmt.Fprintf(stderr, "  要看它：%s eval show %s；要看全表：%s eval ls\n", progName, e.Name, progName)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	script := filepath.Join(root, filepath.FromSlash(e.File))
	if fi, err := os.Stat(script); err != nil || fi.IsDir() {
		inv.setErr("blocked", "script_absent", "件不在盘上")
		fmt.Fprintf(stderr, "%s: 件不在盘上：%s（%v）⇒ 不给结论（退码 8 · **不假装跑过**）\n", progName, e.File, err)
		return exitBlocked
	}
	extra := []string{}
	if len(inv.args) > 1 {
		extra = inv.args[1:]
	}
	argv := append([]string{e.Run, script}, extra...)

	row := map[string]string{
		"name": e.Name, "script": e.File, "class": e.Class, "mode": "real",
		"result": "planned", "rc": "未跑", "note": "",
	}

	if !inv.dryRun && !inv.yes {
		inv.setErr("usage", "missing_yes", "缺 --yes（D2 写面）")
		emitEvalPlan(stderr, e, argv, true)
		fmt.Fprintf(stderr, "%s: 这一步是 D2 档（真跑一件评测脚本 · 可能占机器与模型槽）—— **缺 `--yes` ⇒ 不执行**（退码 2）；"+
			"先看 `%s eval run %s --dry-run` 的计划件\n", progName, progName, e.Name)
		return exitUsage
	}
	if inv.dryRun {
		planOut := stdout
		if inv.jsonGiven {
			planOut = stderr
		}
		emitEvalPlan(planOut, e, argv, false)
		row["mode"], row["result"], row["rc"] = "dry-run", "planned", "0"
		row["note"] = "（--dry-run：零副作用 · 未 exec 任何脚本）"
		if inv.jsonGiven {
			if !requireFields(inv, stderr) {
				return exitFail
			}
			return selectJSON(stdout, stderr, inv, inv.path, evalRunFields, row)
		}
		return exitOK
	}

	// 真跑那一态：执行面 = 旧脚本自己；命令面只转出退码（**连异常码也不改写**）。
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	var human bytes.Buffer
	cmd.Stdout = &human
	cmd.Stderr = stderr
	runningChild = cmd
	err := cmd.Run()
	runningChild = nil
	rc := exitOK
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode() // ★ 只转发、不翻译
	} else if err != nil {
		inv.setErr("failed", "exec_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 跑不动 %s：%v\n", progName, e.File, err)
		return exitFail
	}
	if inv.jsonGiven {
		_, _ = io.Copy(stderr, &human)
	} else {
		_, _ = io.Copy(stdout, &human)
	}
	row["mode"], row["result"] = "real", "ran"
	row["rc"] = fmt.Sprintf("%d", rc)
	row["note"] = "（退码原样转出：脚本退多少就返多少）"
	if rc != exitOK {
		fmt.Fprintf(stderr, "%s: %s 退码 %d —— **原样转出**（命令面不改写任何码）\n", progName, e.File, rc)
	}
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail
		}
		if jrc := selectJSON(stdout, stderr, inv, inv.path, evalRunFields, row); jrc != exitOK {
			return jrc
		}
	}
	return rc
}

// emitEvalPlan —— 计划件（干跑与 fail-closed 两态共用 · 零副作用）。
func emitEvalPlan(w io.Writer, e *evalEntry, argv []string, blocked bool) {
	fmt.Fprintln(w, "计划件（评测线 · 只列一件要跑的脚本 · 未执行任何东西）")
	fmt.Fprintf(w, "  动作     : %s eval run %s\n", progName, e.Name)
	fmt.Fprintf(w, "  件       : %s（归属：%s %s）\n", e.File, e.Class, evalClassState[e.Class])
	fmt.Fprintf(w, "  理由     : %s\n", e.Why)
	fmt.Fprintf(w, "  执行面   : %s（脚本本体是唯一真源 —— 命令面不搬它的实现）\n", strings.Join(argv, " "))
	fmt.Fprintln(w, "  退码口径 : 脚本的退码**原样转出**；命令面自己只退 2（用法错 · 含未知名/未收编）与 8（件不在盘上）")
	if blocked {
		fmt.Fprintln(w, "  未执行   : D2 档 —— 要 `--yes`（或先 `--dry-run` 看这一份）")
	}
}
