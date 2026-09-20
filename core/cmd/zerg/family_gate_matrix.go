// family_gate_matrix.go —— `zerg gate matrix`（§四 D4 · 缺口-命令面-20260921 §十一 P0-5）。
//
// 为什么它排 P0：命令面自己的 **241 条 must-fail 格**今天只以「计数」形式存在
// （`check-cli-contract.py` 现跑：`命令树 … · 矩阵 case …`）—— **格本身导不出来**，
// 而纪律要求「新增命令**同批**补 must-fail」。命令化之后：新增命令时能照着矩阵补格、
// `M17 T1–T6` 从「脚本自查」升成「可回读的物」、门禁红时能一眼看出是**哪一格**红。
//
// 口径（照 §十一 P0-5 的形态，不自造）：
//
//	形态 `zerg gate matrix [--out <件>] [--json <字段>]`
//	输出逐格 `command/case/want_rc/why` + 合计
//	退码 `0` 读到 / `8` 读不到（矩阵读不出）/ `2` 用法错
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
