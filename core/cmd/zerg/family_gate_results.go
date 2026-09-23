// family_gate_results.go —— `zerg gate results --last [--json]`（缺口 `Q-111` / `B-8` · 2026-09-23）。
//
// 病灶（`缺口-命令面-20260923.md` 那一族逐字）：**没有** `zerg gate results --last [--json]` ——
// 今天查「上趟门禁四数（通过/失败/不给结论/只报告）」只能人手去 `/tmp/…` 翻 `results.tsv` 与
// `.out` 尾巴。本命令把那次翻找**变成一条命令**。
//
// 读的是**现成的**产物（一次门禁跑完留在盘上的三件）：
//
//	那一趟目录 = `<TMPDIR>/zerg-gates-<YYYYmmdd-HHMMSS>`（脚本自己的默认命名；
//	               `--candidate` 档落在 `dist/candidates/<id>/gate` ⇒ 用 `--dir` 点名）
//	  ├── results.tsv   结果表：`状态 \t 步名 \t rc \t 耗时 \t 日志路径`（脚本 `run_step` 逐行落）
//	  └── NN-<步名>.log 每步**自己的**日志（脚本硬规矩①：一步一个文件，谁不覆盖谁）
//
// 四数（人面）与 `items`（机器面）**同源**：都从 `results.tsv` 数出来，口径与脚本自己的
// `count_pass/count_fail/count_blocked/count_report` 与 `_exit_rc` **逐字对齐**（脚本是唯一真源；
// 这里不另立一套判定 —— 四个状态字样就是脚本 `_judge` 的四档出口）。
//
// ★ 本命令的退码判的是**读**，不判那一趟的**红绿**：
//
//		0  读通了（**哪怕那一趟里全是 FAIL** —— 那一趟的总退码在数据里：人面那一行 / `meta.exit_code`）
//		1  结果表里有**认不出状态**的行（判不出 ⇒ 不许当绿）
//		2  用法错（`--last` 与 `--dir` 同给 / 多余位置参数 / `--json` 字段表外）
//		8  **读不到那一趟**（没有现成产物 / `--dir` 指不到 / 结果表读不到 / 空表）⇒ 不给结论
//
//	  —— 为什么不让本命令的退码 = 那一趟的总退码：那一趟早已结束，它的红绿是**历史事实**，
//	  不是本次调用成功与否。混在一个码里，「读到了但上趟红」与「压根没读到」就分不开了。
//
// ★ `--json` **不给字段** ⇒ 出全部字段（与 `gate show --json` 同一条理由：本命令的字段面是**闭集**
//
//	且恒全有值 ⇒ 不给字段没有歧义；给了字段就是投影 —— 未知字段 ⇒ 2 + stderr 列字段表）。
//
// ★ 只读：`ReadDir` / `Stat` / `ReadFile` 三件，**不写不删不改**任何日志或结果表（跑完仓内件 sha
//
//	不变 —— 负控在 `cli_gate_results_test.go` 里）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// gateRunPrefix —— 脚本自己的默认日志目录前缀（逐字取自 `--outdir` 的缺省式：
// `${TMPDIR:-/tmp}/zerg-gates-$(date +%Y%m%d-%H%M%S)`）。
const gateRunPrefix = "zerg-gates-"

// gateResultsTSV —— 结果表的文件名（逐字取自脚本：`results = ${outdir}/results.tsv`）。
const gateResultsTSV = "results.tsv"

// gateResultsFields —— `--json` 的字段表（每条一行的五格：步名 / 状态 / 退码 / 耗时 / 日志路径）。
var gateResultsFields = []string{"step", "status", "rc", "secs", "log"}

// gateResultsStatuses —— 状态列的四档闭集（逐字取自脚本 `_judge` 的四个出口）。
var gateResultsStatuses = []string{"PASS", "FAIL", "BLOCKED", "REPORT"}

// gateResultsRow —— 结果表的一行（五列逐字照抄，不翻译）。
type gateResultsRow struct {
	Status string
	Name   string
	RC     string
	Secs   string
	Log    string
}

func gateResultsFieldLegal(f string) bool {
	for _, ok := range gateResultsFields {
		if f == ok {
			return true
		}
	}
	return false
}

func gateResultsStatusLegal(s string) bool {
	for _, ok := range gateResultsStatuses {
		if s == ok {
			return true
		}
	}
	return false
}

// gateResults —— `gate results [--last] [--dir <目录>] [--json <字段>]`。
func gateResults(inv *invocation, stdout, stderr io.Writer, root string) int {
	dir := strings.TrimSpace(inv.flagVal("--dir"))
	if dir != "" && inv.last {
		inv.setErr("usage", "both_sources", "两个来源不许混")
		fmt.Fprintf(stderr, "%s: `--last`（最近一趟）与 `--dir <目录>`（点名那一趟）是**两个来源** ⇒ 同给 = 用法错 2\n", progName)
		fmt.Fprintf(stderr, "口径：不猜谁优先 —— 猜错就报错那一趟\n")
		return exitUsage
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `gate results` 不收位置参数（那一趟由 `--last` / `--dir <目录>` 指）—— 多给了 %q\n", progName, inv.args[0])
		return exitUsage
	}
	// 字段面（**执行前判**）：不给字段 = 闭集全量（见文件头；与 `gate show --json` 同一条理由）。
	fields := inv.fields
	if len(fields) == 0 {
		fields = gateResultsFields
	}
	for _, f := range fields {
		if !gateResultsFieldLegal(f) {
			inv.setErr("usage", "unknown_field", "字段不在本命令的字段表里")
			return reportBadField(stderr, inv.path, f)
		}
	}

	scanned := 0
	if dir == "" {
		var ok bool
		dir, scanned, ok = gateResultsFindLast()
		if !ok {
			inv.setErr("blocked", "no_gate_run", "本机没有现成的门禁产物")
			fmt.Fprintf(stderr, "%s: **读不到那一趟** —— 找过 %s 下所有 `%s*` 目录，没有一份带 %s 的 ⇒ 不给结论（退码 8）\n",
				progName, os.TempDir(), gateRunPrefix, gateResultsTSV)
			fmt.Fprintf(stderr, "口径：「没回执」不是绿（与门⑬ 同一条）—— 要么先跑一趟（`zerg gate run`），要么用 `--dir <目录>` 点名\n")
			return exitBlocked
		}
	}
	tsv := filepath.Join(dir, gateResultsTSV)
	rows, bad, err := gateResultsReadTSV(tsv)
	if err != nil {
		inv.setErr("blocked", "results_unreadable", "结果表读不到")
		fmt.Fprintf(stderr, "%s: 读不到 %s（%v）⇒ 不给结论（退码 8）—— 那一趟的目录名 `--dir` 是否给对了？\n", progName, tsv, err)
		return exitBlocked
	}
	if len(rows) == 0 {
		inv.setErr("blocked", "results_empty", "结果表没有可判的行")
		fmt.Fprintf(stderr, "%s: %s 里**一行可判的都没有**（认不出的行 %d 条）—— 空转/写了半趟？⇒ 不给结论（退码 8）\n",
			progName, tsv, len(bad))
		return exitBlocked
	}
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Status]++
	}
	exit := gateResultsExitCode(counts)
	var warns []string
	if len(bad) > 0 {
		warns = append(warns, fmt.Sprintf("结果表里有 %d 行**认不出状态**（不是 %s 四档之一）⇒ 那几行没进计数，本条命令退 1（判不出不许当绿）：%s",
			len(bad), strings.Join(gateResultsStatuses, "/"), strings.Join(bad, "、")))
	}
	missing := []string{}
	for _, r := range rows {
		if r.Log == "" {
			continue
		}
		if _, err := os.Stat(r.Log); err != nil {
			missing = append(missing, r.Log)
		}
	}
	if len(missing) > 0 {
		warns = append(warns, fmt.Sprintf("结果表点名了 %d 个日志文件，其中 %d 个**不在了**（本命令只读，不删不改 —— 是别人挪走了）：%s",
			len(rows), len(missing), strings.Join(missing, "、")))
	}
	for _, w := range warns {
		inv.warnf("%s", w)
	}

	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			inv.fields = fields
		}
		inv.metaAddStr("run_dir", dir)
		inv.metaAddStr("results_tsv", tsv)
		inv.metaAddJSON("steps", fmt.Sprintf("%d", len(rows)))
		inv.metaAddJSON("pass", fmt.Sprintf("%d", counts["PASS"]))
		inv.metaAddJSON("fail", fmt.Sprintf("%d", counts["FAIL"]))
		inv.metaAddJSON("blocked", fmt.Sprintf("%d", counts["BLOCKED"]))
		inv.metaAddJSON("report", fmt.Sprintf("%d", counts["REPORT"]))
		inv.metaAddJSON("exit_code", fmt.Sprintf("%d", exit))
		if scanned > 0 {
			inv.metaAddJSON("runs_scanned", fmt.Sprintf("%d", scanned))
		}
		items := make([]map[string]string, 0, len(rows))
		for _, r := range rows {
			items = append(items, map[string]string{
				"step": r.Name, "status": r.Status, "rc": r.RC, "secs": r.Secs, "log": r.Log,
			})
		}
		rc := selectJSONList(stdout, stderr, inv, inv.path, fields, items)
		if rc != exitOK {
			return rc
		}
		if len(bad) > 0 {
			return exitFail
		}
		return exitOK
	}

	// ── 人面（人读四数 + 步数 + 总退码；非 PASS 逐条给日志路径 —— 要取证就去点它）──
	fmt.Fprintf(stdout, "那一趟：%s\n", dir)
	fmt.Fprintf(stdout, "结果表：%s（%d 行）\n", tsv, len(rows))
	fmt.Fprintf(stdout, "── 状态计数：PASS %d · FAIL %d · BLOCKED(不给结论) %d · REPORT(只报告) %d ──\n",
		counts["PASS"], counts["FAIL"], counts["BLOCKED"], counts["REPORT"])
	fmt.Fprintf(stdout, "步数 %d · 总退码 %d（脚本自己那条出口口径：有失败项 ⇒ 1；无失败项但有 BLOCKED ⇒ 2；全绿 ⇒ 0）\n",
		len(rows), exit)
	if counts["PASS"] != len(rows) {
		fmt.Fprintf(stdout, "非 PASS 逐条（状态 / 步名 / rc / 耗时 / 日志路径 —— 照抄结果表）：\n")
		for _, r := range rows {
			if r.Status == "PASS" {
				continue
			}
			fmt.Fprintf(stdout, "  %-7s %-46s rc=%-3s %-5s %s\n", r.Status, r.Name, r.RC, r.Secs, r.Log)
		}
	}
	fmt.Fprintf(stdout, "（本命令的退码判的是**读**：读通 0 · 结果表认不出状态 1 · 读不到 8 —— 那一趟自身的红绿在上面那两行，不在本命令的退码里）\n")
	for _, w := range warns {
		fmt.Fprintf(stderr, "%s: ⚠ %s\n", progName, w)
	}
	if len(bad) > 0 {
		return exitFail
	}
	return exitOK
}

// gateResultsFindLast —— 最近一趟的日志目录：扫 `<TMPDIR>` 下所有 `zerg-gates-*`，取 `results.tsv`
// 的 mtime 最新的那一份。返回（目录 · 扫过几份 · 找没找到）。
func gateResultsFindLast() (string, int, bool) {
	base := os.TempDir()
	ents, err := os.ReadDir(base)
	if err != nil {
		return "", 0, false
	}
	best, scanned := "", 0
	var bestT time.Time
	for _, e := range ents {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), gateRunPrefix) {
			continue
		}
		p := filepath.Join(base, e.Name())
		st, err := os.Stat(filepath.Join(p, gateResultsTSV))
		if err != nil {
			continue // 没有结果表 ⇒ 不是「一趟」（跑了半趟/已清走），不算候选
		}
		scanned++
		if best == "" || st.ModTime().After(bestT) {
			best, bestT = p, st.ModTime()
		}
	}
	if best == "" {
		return "", scanned, false
	}
	return best, scanned, true
}

// gateResultsReadTSV —— 读结果表：返回（可判的行 · 认不出的行号 · 读文件错）。
// 行形逐字照抄脚本 `run_step` 的落盘式：`状态 \t 步名 \t rc \t 耗时 \t 日志路径`。
func gateResultsReadTSV(tsv string) ([]gateResultsRow, []string, error) {
	b, err := os.ReadFile(tsv)
	if err != nil {
		return nil, nil, err
	}
	rows := []gateResultsRow{}
	bad := []string{}
	for i, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		parts := strings.Split(ln, "\t")
		if len(parts) < 5 || !gateResultsStatusLegal(parts[0]) {
			bad = append(bad, fmt.Sprintf("第 %d 行", i+1))
			continue
		}
		rows = append(rows, gateResultsRow{
			Status: parts[0], Name: parts[1], RC: parts[2], Secs: parts[3], Log: parts[4],
		})
	}
	return rows, bad, nil
}

// gateResultsExitCode —— 那一趟的**总退码**（重算，不是读出来的 —— 结果表里不记它）。
// 判据逐字对齐脚本的 `_exit_rc`：有失败项 ⇒ 1；无失败项但有 BLOCKED ⇒ 2；全绿 ⇒ 0
// （REPORT「只报告」不进这三档：它既不是错也不是没结论 ⇒ 不影响退码）。
func gateResultsExitCode(counts map[string]int) int {
	if counts["FAIL"] != 0 {
		return 1
	}
	if counts["BLOCKED"] != 0 {
		return 2
	}
	return 0
}
