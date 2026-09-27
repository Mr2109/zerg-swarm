// family_gate_matrix.go —— `zerg gate matrix`（§四 D4 · 缺口-命令面-20260921 §十一 P0-5）。
//
// 为什么它排 P0：命令面自己的 must-fail 格（**格数以 `core/cmd/zerg/testdata/cli-matrix.json` 的 `cases` 现读为准**）今天只以「计数」形式存在
// （`check-cli-contract.py` 现跑：`命令树 … · 矩阵 case …`）—— **格本身导不出来**，
// 而纪律要求「新增命令**同批**补 must-fail」。命令化之后：新增命令时能照着矩阵补格、
// `M17 T1–T6` 从「脚本自查」升成「可回读的物」、门禁红时能一眼看出是**哪一格**红。
//
// 口径（照 §十一 P0-5 的形态，不自造）：
//
//	形态 `zerg gate matrix [--out <件>] [--dry-run | --yes] [--json <字段>]`
//	输出逐格 `command/case/want_rc/why` + 合计
//	退码 `0` 读到 / `8` 读不到（矩阵读不出）/ `2` 用法错（含 `--out` 档：缺 `--yes` · 两枚同给）
//	三态（`--out <件>` 写面 · §九 M3 C1/C2）：`--dry-run` ⇒ 计划件走 stdout + 零落盘；
//	缺 `--yes` ⇒ 计划件走 stderr + 2；`--dry-run` 与 `--yes` 同给 ⇒ 2（先过门、后落盘）
//	★ 无 `--out` 时本命令是纯读面，这两枚确认档**被静默忽略**（不判、不报未知旗标）。
//
// ★ 真源只有一处：`core/cmd/zerg/testdata/cli-matrix.json`（本命令**只读它**，不重算、
// 不重新生成 —— 生成/合并是 `check-cli-contract.py --emit-matrix` 的活，命令面不抢）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// cliMatrixCase —— 矩阵里一格（字段名逐字照 `cli-matrix.json`；**不许改形状**）。
type cliMatrixCase struct {
	ID              string   `json:"id"`
	Command         string   `json:"command"`
	Argv            []string `json:"argv"`
	WantRC          int      `json:"want_rc"`
	WantStdoutBytes int      `json:"want_stdout_bytes"`
	Why             string   `json:"why"`
}

// ★ 2026-09-23（设计-CI适配-v1.1 §十三 `M2` · 波A `A1`）：矩阵**加了两条可选列**
// `want_rc_public` / `want_stdout_bytes_public`（公开面档 · 见 `cli_matrix_test.go` 的 `matrixTier`）。
// 本命令**有意不读它们**：`gate matrix` 导的是**本机档**那张表（它本来就跑在本机树上；
// 公开面档只在公开树里被 `cli_matrix_test.go` 选中）。「不许改形状」仍指既有六列
// （id/command/argv/want_rc/want_stdout_bytes/why）的名字与语义一字不动 —— 新增可选列
// 不改既有列的读法（`encoding/json` 对未声明键是忽略的）。
type cliMatrixFile struct {
	ID         string            `json:"id"`
	Note       string            `json:"note"`
	Exemptions []json.RawMessage `json:"exemptions"`
	Cases      []cliMatrixCase   `json:"cases"`
}

func cmdGateMatrix(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 找不到矩阵真源（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	path := filepath.Join(root, "core", "cmd", "zerg", "testdata", "cli-matrix.json")
	b, err := os.ReadFile(path)
	if err != nil {
		inv.setErr("blocked", "matrix_absent", "矩阵读不到")
		fmt.Fprintf(stderr, "%s: 矩阵真源读不到（%s）⇒ 不给结论（退码 8）\n", progName, path)
		fmt.Fprintf(stderr, "真源是 `python3 scripts/gates/check-cli-contract.py --emit-matrix` 写的那一件。\n")
		return exitBlocked
	}
	var mf cliMatrixFile
	if err := json.Unmarshal(b, &mf); err != nil {
		inv.setErr("blocked", "matrix_unparsable", err.Error())
		fmt.Fprintf(stderr, "%s: 矩阵解析不了（%v）⇒ 不给结论（退码 8）\n", progName, err)
		return exitBlocked
	}

	rows := make([]map[string]string, 0, len(mf.Cases))
	nonzero := 0
	for _, c := range mf.Cases {
		if c.WantRC != 0 {
			nonzero++
		}
		rows = append(rows, map[string]string{
			"command": c.Command,
			"case":    c.ID,
			"want_rc": strconv.Itoa(c.WantRC),
			"bytes":   strconv.Itoa(c.WantStdoutBytes),
			"why":     c.Why,
			"argv":    strings.Join(c.Argv, " "),
		})
	}
	fmt.Fprintf(stderr, "%s: 矩阵 %s · 格 %d 条（expect-rc≠0 的 %d 条 · 豁免 %d 条）\n",
		progName, mf.ID, len(rows), nonzero, len(mf.Exemptions))

	// `--out <件>`：把**逐格**导成一份可回读的 TSV（含 argv 与 why 原文 —— 补格时要照着它写）。
	if out := strings.TrimSpace(inv.flagVal("--out")); out != "" {
		// ★ 三态门（D2 · §九 M3 C1/C2 —— 与同族 `gap export` / `gap add|verify` 逐字同一条）：
		//   `--dry-run` ⇒ 计划件走 stdout + rc=0（**一个字节都不落**）；缺 `--yes` ⇒ 计划件走 stderr + 2
		//   （fail-closed：从不提问）；两枚**同给**（自相矛盾）⇒ 2（与 `--out`/`--json` 不许同给同一形状）。
		//   三条都判在**本函数唯一的写盘点**（下面那一行 `os.WriteFile`）**之前** ⇒ 未过门前不建件、不建目录。
		if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
			return rc
		}
		if inv.dryRun {
			gateMatrixOutPlan(stdout, path, out, len(rows), nonzero, len(mf.Exemptions), "--dry-run")
			fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未落件、未建目录；矩阵真源一个字未动）\n")
			return exitOK
		}
		if !inv.yes {
			gateMatrixOutPlan(stderr, path, out, len(rows), nonzero, len(mf.Exemptions), "缺 `--yes`（D2 档）")
			inv.setErr("usage", "yes_required", "缺 --yes")
			return exitUsage
		}
		var sb strings.Builder
		sb.WriteString("# 命令面 must-fail 矩阵（导出自 " + path + " · id=" + mf.ID + "）\n")
		sb.WriteString("command\tcase\twant_rc\twant_stdout_bytes\targv\twhy\n")
		for _, r := range rows {
			sb.WriteString(strings.Join([]string{r["command"], r["case"], r["want_rc"], r["bytes"], r["argv"], r["why"]}, "\t") + "\n")
		}
		if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
			inv.setErr("failed", "out_write_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 落点写不进去（%v）—— 给一个**已存在**的目录里的路径\n", progName, err)
			return exitFail
		}
		fmt.Fprintf(stderr, "%s: 已导出 %d 格 → %s\n", progName, len(rows), out)
	}
	return listCmd(inv, stdout, stderr, []string{"command", "case", "want_rc", "why"}, rows)
}

// gateMatrixOutPlan —— `--out <件>` 落盘面的**计划件**（`--dry-run` ⇒ 走 stdout · 缺 `--yes` ⇒ 走 stderr）。
//
// 纯函数：只**数**格与行、只往 `io.Writer` 写 ⇒ dryrun.v1「零副作用」的落点就在这一行
// （不建目录 · 不落件 · 不写真源 · 不写审计）。件内容与真写**同一份算法**（注释头 1 行 + 列名 1 行 +
// 逐格 N 行 = N+2 行 · 制表符分隔 · 列序 command/case/want_rc/want_stdout_bytes/argv/why）——
// 计划与真跑不许两套数（数不对 = 假预演 ✗）。
func gateMatrixOutPlan(w io.Writer, src, out string, nCases, nonzero, exemptions int, why string) {
	fmt.Fprintf(w, "计划件（%s · `zerg gate matrix --out %s`）\n", why, out)
	fmt.Fprintf(w, "  落点件   : %s\n", out)
	fmt.Fprintf(w, "  真源     : %s（只读：本档不写真源、不写审计）\n", src)
	fmt.Fprintf(w, "  计数     : 格 %d 条（expect-rc≠0 的 %d 条 · 豁免 %d 条）\n", nCases, nonzero, exemptions)
	fmt.Fprintf(w, "  件内容   : 注释头 1 行 + 列名 1 行 + 逐格 %d 行 = %d 行（制表符分隔）\n", nCases, nCases+2)
	fmt.Fprintf(w, "  未执行   : %s —— 本档**一个字节都不落**（真写要 `--yes`）\n", why)
}
