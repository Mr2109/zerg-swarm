// family_repo_diffstat.go —— `zerg repo diffstat --report <件>`（T20）。
//
// 病灶（逐字）：报行数**三格的机检面**没有正门 —— 今天判「一份 diffstat 是不是把
// **原始 diff / 增 / 减** 三格都报齐了」只能肉眼比；而**只给净数**（一个净差）在仓里
// 竟能一路当绿走过去。T20 的口径（逐字）：报数**一律三格**，**只给净数即判未报**。
//
// 三格（判据面逐字，缺一格即**判不了**）：
//
//	raw 原始 diff / add 增 / del 减
//
// ★ 退码（**判不了 ≠ 绿** —— 本命令的退码判的是「报告报齐没有」，不是「净数好不好看」）：
//
//	0  三格**逐字齐**（原始 diff / 增 / 减 都给了）—— 唯一写 stdout 的一档
//	1  输入为空（无读数 —— 也判不了）
//	2  缺格（**含「只给净数」** ⇒ 未报 ⇒ 判不了）· 用法错（缺 `--report` / 多余位置参数 / `--json` 字段表外）
//	8  **判不了**（`--report` 件读不到 ⇒ 不给结论）
//
// ★ 面流口径（照 T09/T11 那批的矩阵格：早判档 `want_stdout_bytes=0`）：
// **所有非 0 档一律只写 stderr · stdout 恒 0 字节**（判据件流在 stderr，机器面不受污染）；
// 只有三格齐（退 0）才写 stdout。
// ★ 只读：`os.ReadFile` 一枚件，**不写不删不改**任何件（跑完仓内件 sha 不变）。
// ★ 与 `repo status` 同族同守：**写旗标一律拒**（`repoWriteWords` · 形态无关）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// repoDiffstatCells —— 三格闭集（逐字；顺序即人面顺序）。
var repoDiffstatCells = []string{"raw", "add", "del"}

// repoDiffstatLabels —— 三格的中文名（报数面逐字）。
var repoDiffstatLabels = map[string]string{"raw": "原始 diff", "add": "增", "del": "减"}

// repoDiffstatFields —— `--json` 的字段表（每格一行四列：格名 / 中文名 / 值 / 命中否）。
var repoDiffstatFields = []string{"cell", "label", "value", "hit"}

// repoDiffstatCellPat —— 三格认法（点名格位 · 不抄行号）：
//
//	`原始 diff N` / 分隔符+`增 N` / 分隔符+`减 N`
//
// ★ 增/减 那两枚**要求前置分隔符**（空白 / `:` / `=` / `·` / `|` / `,` / 括号）——
// 这样 `净增 96`（净数措辞里的「增」）**不会被误当成 add 格**（「只给净数」必须真判成缺格）。
var repoDiffstatCellPat = map[string]*regexp.Regexp{
	"raw": regexp.MustCompile(`原始\s*diff\s*[:：=]?\s*(\d+)`),
	"add": regexp.MustCompile(`(?:^|[\s:：=·|,，、（(【])增\s*[:：=]?\s*(\d+)`),
	"del": regexp.MustCompile(`(?:^|[\s:：=·|,，、（(【])减\s*[:：=]?\s*(\d+)`),
}

// repoDiffstatNetOnly —— 「只给净数」的现读征兆（净 / net 字样）。
var repoDiffstatNetOnly = regexp.MustCompile(`(净|net)`)

// repoDiffstatFieldLegal —— `--json` 字段白名单（照同族口径）。
func repoDiffstatFieldLegal(f string) bool {
	for _, ok := range repoDiffstatFields {
		if f == ok {
			return true
		}
	}
	return false
}

// repoDiffstatScan —— 三格认读：返回（命中格 → 值）。缺的格**不补 0**（缺就是缺）。
func repoDiffstatScan(text string) map[string]string {
	out := map[string]string{}
	for k, re := range repoDiffstatCellPat {
		if m := re.FindStringSubmatch(text); m != nil {
			out[k] = m[1]
		}
	}
	return out
}

// cmdRepoDiffstat —— `repo diffstat`（只读 · 不走脚本 · 不改任何件）。
func cmdRepoDiffstat(inv *invocation, stdout, stderr io.Writer) int {
	// 同族同守（照 `repo status` 那一口径）：写旗标一律拒。
	if hits := repoWriteHits(inv.orig); len(hits) > 0 {
		inv.setErr("usage", "write_flag_refused", "本命令是只读机检 · 写旗标一律拒")
		fmt.Fprintf(stderr, "%s: 拒（退码 2 · 不给结论）：命令行里出现写旗标 %v\n", progName, hits)
		fmt.Fprintf(stderr, "%s: `repo diffstat` **只读** · 要写请用 `%s repo commit`\n", progName, progName)
		return exitUsage
	}
	rep := strings.TrimSpace(inv.flagVal("--report"))
	if rep == "" {
		inv.setErr("usage", "missing_report", "缺 --report（报告件）")
		fmt.Fprintf(stderr, "%s: `repo diffstat` 要给一枚报告件：`--report <件>`\n", progName)
		fmt.Fprintf(stderr, "口径：报行数**一律三格**（原始 diff / 增 / 减）—— **只给净数即判未报**（不给结论）\n")
		return exitUsage
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `repo diffstat` 不收位置参数（报告件由 `--report` 指）—— 多给了 %q\n", progName, inv.args[0])
		return exitUsage
	}
	fields := inv.fields
	if len(fields) == 0 {
		fields = repoDiffstatFields
	}
	for _, f := range fields {
		if !repoDiffstatFieldLegal(f) {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, inv.path, f)
		}
	}

	b, err := os.ReadFile(rep)
	if err != nil {
		// 判不了 ⇒ 不给结论（8）：件读不到既不是「齐」也不是「缺」。
		inv.setErr("blocked", "report_unreadable", "报告件读不到")
		fmt.Fprintf(stderr, "%s: **判不了** ⇒ 不给结论（退码 8）\n", progName)
		fmt.Fprintf(stderr, "  --report %s：%v\n", rep, err)
		fmt.Fprintf(stderr, "口径：读不到既不是「三格齐」也不是「缺格」—— 不许当绿\n")
		return exitBlocked
	}
	text := string(b)
	if strings.TrimSpace(text) == "" {
		inv.setErr("failed", "report_empty", "报告件为空")
		fmt.Fprintf(stderr, "%s: 报告件：%s\n", progName, rep)
		fmt.Fprintf(stderr, "!! 输入为空 ⇒ 无读数 ⇒ 判不了（退 1）· 不许当绿\n")
		return exitFail
	}
	hits := repoDiffstatScan(text)
	missing := []string{}
	for _, c := range repoDiffstatCells {
		if _, ok := hits[c]; !ok {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		inv.setErr("usage", "cells_missing:"+strings.Join(missing, ","), "三格缺格（未报）")
		names := []string{}
		for _, c := range missing {
			names = append(names, repoDiffstatLabels[c])
		}
		fmt.Fprintf(stderr, "%s: 报告件：%s\n", progName, rep)
		fmt.Fprintf(stderr, "✗ 未报：缺 %d 格（%s）—— 报行数一律三格（原始 diff / 增 / 减），只给净数视为未报\n",
			len(missing), strings.Join(names, " · "))
		if repoDiffstatNetOnly.MatchString(text) {
			fmt.Fprintf(stderr, "  （现读含「净」字样 ⇒ 只给了净数）\n")
		}
		fmt.Fprintf(stderr, "结论：判不了 ⇒ 不给结论（退 2）\n")
		return exitUsage
	}

	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			inv.fields = fields
		}
		inv.metaAddStr("report", rep)
		inv.metaAddJSON("cells", fmt.Sprintf("%d", len(repoDiffstatCells)))
		items := make([]map[string]string, 0, len(repoDiffstatCells))
		for _, c := range repoDiffstatCells {
			items = append(items, map[string]string{
				"cell": c, "label": repoDiffstatLabels[c], "value": hits[c], "hit": "yes",
			})
		}
		return selectJSONList(stdout, stderr, inv, inv.path, fields, items)
	}

	fmt.Fprintf(stdout, "报告件：%s\n", rep)
	fmt.Fprintf(stdout, "✓ 三格齐：原始 diff=%s · 增=%s · 减=%s\n", hits["raw"], hits["add"], hits["del"])
	fmt.Fprintf(stdout, "结论：三格逐字齐（退 0）—— 报数面齐了\n")
	fmt.Fprintf(stdout, "（本命令只读 · 不改报告件 · 也不产报告件）\n")
	return exitOK
}
