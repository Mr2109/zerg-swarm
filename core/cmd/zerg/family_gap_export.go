// `zerg gap export` —— 缺口真源 → 「结转账 markdown 片段」导出面（**只读**：不写真源、不写审计）。
//
// 病根（设计稿 `设计-缺口账视图分离-v1.1/v1.2/v1.3` §1 · 真源 `GAP-20260925-02`）：
// CLI 真源（`<状态目录>/zerg-cli-gaps.jsonl`）与文档侧的结转账（`Zerg-内部文档/…/缺口总账-*.md`）
// 之间**没有机器同步面** ⇒ 两处账必然漂移、全靠人手抄（本日已亲历一次：助手手改结账单绕过 CLI 写面）。
// 本命令 = 那道导出面：把真源里**未闭**的条目按「面」分小节打印成 markdown 片段，让文档侧的结转账
// **由生成而来**（不再手抄）。
//
// 三条口径（设计稿 §3 写死）：
//  1. **只列未闭**（`仍缺`/`已派`/`已立项`/`回归`）；已闭（`已解`/`不做`）**不进正文** ✗（只进件头计数）。
//  2. 件头 = **仪表盘**（真源条数 · 未闭/已闭 · 逐状态与逐面计数 · 最老未闭天数），不是叙事。
//  3. **格式纪律**：单行 ≤ `gapExportLineMax` 字符（超出以 `…` 截断）—— 巨型单行在任何编辑器都看不动 ✗
//     （现读的血证：旧的结转账最长单行 11745 字符）。
//
// fail-closed（与 `gap ls` 同一口径，`Q-138`）：读不到真源 ⇒ 退 8 + 打印真源路径；真源不在盘上 ⇒ 退 8。
// 「读不到」**不许当绿** ✗，也不许静默输出一份空账单 ✗。
package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// gapExportFields —— `--json` 可取字段（与命令树里的 `fields` 同一份口径）。
var gapExportFields = []string{"id", "prio", "impact", "state", "want", "summary", "fp", "verify_cmd", "found_at"}

// gapOpenStates —— 「未闭」的四态（`已解`/`不做` 是闭态 ⇒ 不进正文）。
var gapOpenStates = []string{"仍缺", "已派", "已立项", "回归"}

// gapExportLineMax —— 单行字符上限（设计稿 §3 格式纪律 R-02）。
const gapExportLineMax = 200

// gapExportStaleAfterDays —— 「陈旧」的宽限天数（仅用于件头提示；折叠/关闭由文档侧按版执行）。
const gapExportStaleAfterDays = 14

func cmdGapExport(inv *invocation, stdout, stderr io.Writer) int {
	// ① 用法面（在任何盘面动作之前）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapExportFields, ","))
		return exitUsage
	}

	// ② 读真源（读不到 / 不在盘上 ⇒ 8 · **不许当绿**）
	led, err := readGapLedger()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapLedgerPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当「没有」（退码 8）\n", gapLedgerPath())
		gapLedgerUnreadableHint(stderr, gapLedgerPath())
		return exitBlocked
	}
	if !led.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "真源不在盘上", led.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		gapLedgerAbsentHint(stderr, led.Path)
		return exitBlocked
	}

	// ③ 账内闭集自查（与 `gap ls` 同一口径：闭集外的值 ⇒ 判红 1 + 点名到行）
	for i, r := range led.Recs {
		field, val := "", ""
		switch {
		case !gapIn(gapStateClosed, r.State):
			field, val = "state", r.State
		case !gapIn(gapImpactClosed, r.Impact):
			field, val = "impact", r.Impact
		case r.Prio != "" && !gapIn(gapPrioClosed, r.Prio):
			field, val = "prio", r.Prio
		}
		if field == "" {
			continue
		}
		inv.changed = boolPtr(false)
		inv.setErr("failed", "ledger_value_out_of_set", field+" 的值不在闭集里")
		fmt.Fprintf(stderr, "%s: 真源第 %d 行（%s）的 `%s` = %q 不在闭集里 ⇒ 判红（退码 1）\n",
			progName, led.No[i], r.ID, field, val)
		fmt.Fprintf(stderr, "合法值: %s\n", gapClosedText(gapClosureOf(field)))
		fmt.Fprintf(stderr, "真源 = %s（改它请走 CLI 写面：`zerg gap add|verify`）\n", led.Path)
		return exitFail
	}

	// ④ 只留未闭 + 分组（按 impact 六值闭集顺序；空面不出小节）
	open := make([]gapRecord, 0, len(led.Recs))
	closedCount, noDecideCount := 0, 0
	for _, r := range led.Recs {
		if r.State == "不做" {
			noDecideCount++
			closedCount++
			continue
		}
		if r.State == "已解" {
			closedCount++
			continue
		}
		if !gapIn(gapOpenStates, r.State) {
			continue
		}
		open = append(open, r)
	}
	sort.SliceStable(open, func(i, j int) bool {
		if open[i].Prio != open[j].Prio {
			return gapPrioRank(open[i].Prio) < gapPrioRank(open[j].Prio)
		}
		return open[i].ID < open[j].ID
	})

	if len(open) == 0 {
		inv.changed = boolPtr(false)
		inv.setErr("failed", "no_open_gap", "未闭 0 条")
		fmt.Fprintf(stderr, "%s: 真源 %s 里**未闭 0 条**（账内 %d 条）⇒ 判红（退码 1 —— 「没有」不是「导出成功」）\n",
			progName, led.Path, len(led.Recs))
		return exitFail
	}

	// ⑤ `--json` ⇒ 机读（与人面同一份筛选与排序）
	if inv.jsonGiven {
		rows := make([]map[string]string, 0, len(open))
		for _, r := range open {
			rows = append(rows, map[string]string{
				"id": r.ID, "prio": r.Prio, "impact": r.Impact, "state": r.State,
				"want": gapWantText(r), "summary": r.Symptom, "fp": r.FP,
				"verify_cmd": r.VerifyCmd, "found_at": r.FoundAt,
			})
		}
		inv.changed = boolPtr(false)
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}

	// ⑥ 人面 markdown 片段：件头仪表盘（只报数）+ 逐面小节（只列未闭 · 单行 ≤ 上限）
	inv.changed = boolPtr(false)
	perImpact := map[string]int{}
	maxDays := -1
	oldest := ""
	for _, r := range open {
		perImpact[r.Impact]++
		if d, ok := gapFoundDays(r.FoundAt); ok && d > maxDays {
			maxDays, oldest = d, fmt.Sprintf("%s（%s · %d 天）", r.ID, r.FoundAt, d)
		}
	}
	fmt.Fprintf(stdout, "## 缺口真源导出（**只列未闭**） · %s\n\n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(stdout, "> 生成件：由 `zerg gap export` 从真源 `%s` 生成 —— **不许手改** ✗（手改会被门禁判红）。\n", led.Path)
	fmt.Fprintf(stdout, "> 真源条数 **%d** · 未闭 **%d** · 已闭 **%d**（其中「不做」**%d**）\n", len(led.Recs), len(open), closedCount, noDecideCount)
	faces := make([]string, 0, len(gapImpactClosed))
	for _, im := range gapImpactClosed {
		faces = append(faces, fmt.Sprintf("%s %d", im, perImpact[im]))
	}
	fmt.Fprintf(stdout, "> 按面：%s\n", strings.Join(faces, " · "))
	if maxDays >= 0 {
		fmt.Fprintf(stdout, "> 最老未闭：%s\n", oldest)
	}
	if maxDays >= gapExportStaleAfterDays {
		fmt.Fprintf(stdout, "> ⚠ 已有未闭条目存放 ≥ %d 天（陈旧线）—— 按设计稿口径：**先标 → 宽限 1 版 → 才折叠**，且**任一触碰即撤标**。\n", gapExportStaleAfterDays)
	}
	fmt.Fprintf(stdout, "\n")

	for _, im := range gapImpactClosed {
		group := make([]gapRecord, 0, perImpact[im])
		for _, r := range open {
			if r.Impact == im {
				group = append(group, r)
			}
		}
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(stdout, "### %s（%d 条）\n\n", im, len(group))
		for _, r := range group {
			line := fmt.Sprintf("- `%s` %s · **%s** · %s · 想要 `%s` · 判据 `%s`",
				r.ID, r.Prio, r.State, r.Symptom, gapWantText(r), r.VerifyCmd)
			fmt.Fprintf(stdout, "%s\n", gapClip(line, gapExportLineMax))
		}
		fmt.Fprintf(stdout, "\n")
	}
	fmt.Fprintf(stdout, "（本片段由 CLI 生成 · 未闭 %d 条 · 单行上限 %d 字符）\n", len(open), gapExportLineMax)
	return exitOK
}

// gapPrioRankUnknown —— 未知 prio 的排序档（**具名常量**：命令面契约门 T3 禁裸数字 ✗
// —— 裸 `9` 会被当成「不在退码真源表里的退出码」，具名后语义自明）。
const gapPrioRankUnknown = 9

// gapPrioRank —— P0 < P1 < P2（未知值排最后；未知值本会在闭集自查里先判红）。
func gapPrioRank(p string) int {
	switch p {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	}
	return gapPrioRankUnknown
}

// gapClip —— 单行超限则按 **rune** 截断并加省略号（中文一字一 rune，不用字节数）。
func gapClip(s string, max int) string {
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(rs[:max-1]) + "…"
}

// gapFoundDays —— `found_at` 到今天的天数。认 `2006-01-02` 与 RFC3339 两种写法；
// 认不出 ⇒ `ok=false`（调用方照实不报天数，**不猜** ✗）。
func gapFoundDays(foundAt string) (int, bool) {
	s := strings.TrimSpace(foundAt)
	if s == "" {
		return 0, false
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			d := int(time.Since(t).Hours() / 24)
			if d < 0 {
				d = 0
			}
			return d, true
		}
	}
	return 0, false
}
