// family_gate_find.go —— `zerg gate find <片段>`：**门件名 ⇒ 接线位置** 的那座桥（缺口 `GAP-20260927-07`）。
//
// 病根（父代理 2026-09-27 现场 · 两条 rc=2 逐字）：
//
//	`zerg gate show check-gate-coverage`    ⇒ 「步骤名里没有匹配 check-gate-coverage 的」
//	`zerg gate explain check-gate-coverage` ⇒ 「步骤名 %q **逐字**找不到（现读脚本里的 add_step 行 182 条）」
//
// 门族两条既有入口的键都是**步骤名**（`add_step` 的第二个引号里那串），而日常手上的名字是**门件名**
// （`scripts/gates/check-*.py`）—— 两者之间**没有一格把桥搭起来** ⇒ 想从门件名问出「这一步叫什么、
// 跑什么命令、什么档位、日志落哪」只能手搓 `grep add_step` ✗。
//
// ★ 补第二半（同夜 · 同一个缺口的另一半）：**接线面不止一条** —— 一件门件「跑得到」在本仓有三处面，
// 只有第一处**按名字点名**：
//
//	① `precommit-gates.sh` 的 `add_step <scope> "<名>" <模式> "<目录>" "<命令>"` —— 命令串点名；
//	② `all.sh:179` `for f in "${GATEDIR}"/*.py` —— **按通配收**：件在即跑得到，**名字一次都不出现**，
//	   且**只收 `.py`** ⇒ 11 只 `.sh` 门在这一面完全不可见 ✗；
//	③ `real-gates.sh` —— 一份**名录**（`--only <门名>` 的过滤面），名单外的件按「可跑」处理。
//
// ⇒ 只知道①，会把「门⑫根本没进 precommit（却跑得到）」误读成「这门不存在」✗。故本命令出**两种行**：
//
//	**步骤行**（片段命中步名/命令串）：八格 + 第九格 `wiring` 给出它在三处面的接线位置；
//	**门件行**（片段命中 `scripts/gates/` 下的真件名，但没有任何 `add_step` 点名它）：一行说明
//	「在哪跑得到、在哪跑不到」—— 这就是「零命中」与「不存在」的分别 ✓。
//
// 九格与 `gate explain` 的关系（**同源，不另写一份**）：`scope`/`mode`/`verdict`/`log`/`source` 全部
// 从同一份 `add_step` 声明与同一组 helper（`parseAddSteps` / `gateShowModeCell` / `gateModeVerdict` /
// `gateStepLogNote`）读 —— 判据与日志路径在本仓只有**一处**口径（两处各写一份必然漂 · `G1-a`）。
// 第八格 `next` 是**可照抄的下一步**，第九格 `wiring` 是三处接线面的**现读**结论（读脚本得的，不是猜的）。
//
// 退码：0 有命中 · 1 零命中 · 2 用法错（缺片段）· 8 不给结论（门禁脚本读不到）。
// 全程**只读**：只读脚本与件名，不跑任何步骤、不写任何件。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// gateFindFields —— `gate find --json` 的闭集九格。前八格与 `gate explain` 同义字段逐字同名
// （机读侧拿这一格就能直接换命令用），第九格 `wiring` 是**三处接线位置**的现读结论。
var gateFindFields = []string{"name", "scope", "mode", "verdict", "cmd", "log", "source", "next", "wiring"}

// cmdGateFind —— `zerg gate find <片段> [--json <字段>]`。
//
// 放在 `cmdGate` 的分支里（与 `results` 同规：**只在命令面自己的旗标上生效**，不改脚本退码）。
// `root`/`script` 由 `cmdGate` 传进来（它已经把仓根与自举件路径核过一遍）。
func cmdGateFind(inv *invocation, stdout, stderr io.Writer, root, script string) int {
	probe := ""
	if len(inv.args) > 0 {
		probe = strings.TrimSpace(inv.args[0])
	}
	if probe == "" {
		inv.setErr("usage", "missing_step_name", "缺片段（门件名或命令串里的一段）")
		fmt.Fprintf(stderr, "%s: `gate find` 要给片段（例：%s gate find check-gate-coverage）\n", progName, progName)
		fmt.Fprintf(stderr, "口径：片段在**步骤名 + 命令串 + scripts/gates/ 下的件名**上做子串匹配（英文大小写不敏感）\n")
		fmt.Fprintf(stderr, "      拿到步名后再 `%s gate show '<步名>' --json`（命令串/档位/判据/日志路径）\n", progName)
		return exitUsage
	}

	b, err := os.ReadFile(script)
	if err != nil {
		inv.setErr("blocked", "gate_script_unreadable", "门禁脚本读不到")
		fmt.Fprintf(stderr, "%s: 读不到门禁脚本 %s（%v）⇒ 不给结论（退码 8）\n", progName, gateScriptRel, err)
		return exitBlocked
	}
	src := string(b)
	decls := parseAddSteps(src)
	needle := strings.ToLower(probe)
	rows := []map[string]string{}

	// ① 步骤行：片段命中**步名**或**命令串**（粘成一段再 contains：门件名在命令串里，
	//    人给的词可能是步名里的词，两处都得能中）。
	for _, d := range decls {
		if !strings.Contains(strings.ToLower(d.Name+"\n"+d.Cmd), needle) {
			continue
		}
		logNote, _ := gateStepLogNote(root, d.Name)
		rows = append(rows, map[string]string{
			"name":    d.Name,
			"scope":   d.Scope,
			"mode":    gateShowModeCell(d.Mode),
			"verdict": gateModeVerdict(d.Mode),
			"cmd":     d.Cmd,
			"log":     logNote,
			"source":  fmt.Sprintf("%s:%d", gateScriptRel, d.Line),
			"next":    fmt.Sprintf("%s gate show '%s' --json", progName, d.Name),
			"wiring":  gateWiringNote(root, d.Cmd, decls),
		})
	}

	// ② 门件行：片段命中 `scripts/gates/` 下的**真件名**。这一行的理由：**「零命中」与「不存在」
	//    是两件事** —— 门⑫（`check-doc-cmds.py`）跑得到，但 `add_step` 里一个字都没有 ⇒ 只回
	//    「零命中」会把人引到「这门不存在」的错结论上 ✗（这正是补第二半的直接起因）。
	for _, rel := range gateFilesMatching(root, probe) {
		rows = append(rows, map[string]string{
			"name":    rel,
			"scope":   "—",
			"mode":    "—",
			"verdict": "—（真件在，但 `add_step` 步骤表里未点名 —— 判据看 `wiring` 格的三处面）",
			"cmd":     "—",
			"log":     "—",
			"source":  "scripts/gates/ 下的真件（命令串面：`add_step` 未点名）",
			"next":    fmt.Sprintf("%s code find '%s'", progName, filepath.Base(rel)),
			"wiring":  gateWiringNote(root, rel, decls),
		})
	}

	if len(rows) == 0 {
		// ★ 零命中的口径与 `zerg code find` **逐字一致**：这不是错，是「没有」（退码 1）。
		// 所以这里**不** setErr（error 面只在真出错时出现）—— 只把「怎么找得着」印给人。
		fmt.Fprintf(stderr, "%s: 零命中 —— 步骤名、命令串与 scripts/gates/ 件名里都没有 %q（现读 add_step 行 %d 条 · 退码 1）\n",
			progName, probe, len(decls))
		fmt.Fprintf(stderr, "口径：零命中不是错，是「没有」；要看全表 `%s gate ls`，要按件名找 `%s find <名字片段>`\n",
			progName, progName)
		for _, s := range nearestSteps(decls, probe, 3) {
			fmt.Fprintf(stderr, "  像它的是：%s\n", s)
		}
		return exitFail
	}

	if len(inv.fields) == 0 {
		inv.fields = gateFindFields
	}
	fmt.Fprintf(stderr, "%s: 片段 %q 命中 %d 行（现读 add_step 行 %d 条 · 三处接线面 = precommit / all.sh / real-gates.sh）\n",
		progName, probe, len(rows), len(decls))
	// 表/JSON 两条路都走 `listCmd`（与 `gate explain` 同一入口）⇒ 默认面与机器面**不各写一份**。
	return listCmd(inv, stdout, stderr, gateFindFields, rows)
}

// gateFilesMatching —— `scripts/gates/` 下件名含片段的真件（仓根相对路径 · 排序后返回）。
// **只列件名，不读件内容**：本格要回答「这门叫什么、挂在哪」，不是「它判什么」。
func gateFilesMatching(root, probe string) []string {
	ents, err := os.ReadDir(filepath.Join(root, "scripts", "gates"))
	if err != nil {
		return nil
	}
	needle := strings.ToLower(probe)
	out := []string{}
	for _, e := range ents {
		if e.IsDir() || !strings.Contains(strings.ToLower(e.Name()), needle) {
			continue
		}
		out = append(out, filepath.ToSlash(filepath.Join("scripts", "gates", e.Name())))
	}
	sort.Strings(out)
	return out
}

// gateWiringNote —— 一件门件在**三处接线面**的现读结论（同夜补的第二半）：
//
//	precommit：`add_step` 的命令串里点到它的那几步（步名 + 出处行）；
//	all.sh  ：`for f in "${GATEDIR}"/*.py`（逐字读自 `all.sh:179`）⇒ **.py 按通配收**、**.sh 不收**；
//	real-gates：名录里点到它的首行（名单外 ⇒ 「名单外（按可跑处理）」）。
//
// `what` 既可能是**命令串**（步骤行）也可能是**件路径**（门件行）⇒ 两种都按基名子串找，宁多勿漏。
func gateWiringNote(root, what string, decls []gateStepDecl) string {
	base := filepath.Base(what)
	parts := []string{}

	named := []string{}
	for _, d := range decls {
		if strings.Contains(d.Cmd, base) {
			named = append(named, fmt.Sprintf("%s@%d", d.Name, d.Line))
		}
	}
	if len(named) > 0 {
		parts = append(parts, "precommit: 点名 "+strings.Join(named, " · "))
	} else {
		parts = append(parts, "precommit: 未点名（不在步骤表里）")
	}

	if strings.HasSuffix(base, ".py") {
		parts = append(parts, "all.sh: 按通配收（`*.py` @179）")
	} else {
		parts = append(parts, "all.sh: **不收**（`all.sh:179` 只收 `*.py` ⇒ 非 `.py` 件（含 `.sh` 门）在这一面不可见）")
	}

	if rb, err := os.ReadFile(filepath.Join(root, "scripts", "gates", "real-gates.sh")); err == nil {
		line := 0
		for i, ln := range strings.Split(string(rb), "\n") {
			if strings.Contains(ln, base) {
				line = i + 1
				break
			}
		}
		if line > 0 {
			parts = append(parts, fmt.Sprintf("real-gates: 名录点名@%d", line))
		} else {
			parts = append(parts, "real-gates: 名单外（按可跑处理）")
		}
	} else {
		parts = append(parts, "real-gates: 读不到该件 ⇒ 判不了")
	}
	return strings.Join(parts, " ｜ ")
}
