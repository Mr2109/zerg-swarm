// family_gate_baseline.go —— `zerg gate baseline show [<条件名>] [--json <字段>] [--root <目录>]`（Q-125）。
//
// 病根（Q-125 现读）：命令树里**没有 `baseline` 这个节点** ⇒ `zerg gate baseline show` 落到
// `main.go` 的「族名/中间节点」分支（rc=2，首行「… 是族名/中间节点」，最像的合法输入指到
// `zerg gate ls` —— 与基线毫无关系）。读棘轮现状今天只有一条路：`zerg gate check
// baseline-ratchet --view`（名词是**判据件名**，不是命令名），而 `--view` 的逐行只印「现值」+
// 一段散文判词 —— 台账基线值（该条件最后一条记录的 `new`）与差（现读 − 基线）**要人自己从
// 判词里抠**。本命令把这三格变成第一类动作。
//
// ★ 唯一读路径（本件**不另造第二套**）：条件表 / 现值 / 台账 / 判词全部来自
// `baseline_ratchet.go` 的 `brConditions` / `brReadValue`（经 `brCollect`）/ `brLoadLedger` /
// `brLastEntryFor` / `brVerdictOf` —— 与 `zerg gate check baseline-ratchet --view` 是**同一个
// 引擎、同一份条件表、同一条台账**。本件不 parse 任何基线件、不读 JSONL、不认任何键名，
// 只做**呈现与求和**（两列派生：基线值 = 该条件最后一条记录的 `new`；差 = 现读 − 基线值）。
//
// ★ 与 `--view` 的**一处有意差**（写清，别处不许漂）：棘轮 `--view` 对「件不在」档给
// `verdict=n/a` 且 rc 回落 0 —— `baseline_ratchet_test.go:149` 把这个口子逐字钉住
// （「判不了 ≠ 绿 但 rc 不翻红」）。本命令是**新的呈现面**，不继承那个口子：只要有一条判不了
// ⇒ **rc=2** 并逐条点名（「读不到」不许当绿）。棘轮本体一字不动（它的 py 对拍面不许因本件翻红）。
//
// 退码（闭集 0/1/2 —— 与同族 `gate results` / `gate ls` 同一张表，✗ 不新占号）：
//
//	0  每个条件的现读值都在台账里找得到**自己那条**记录且逐字相等（warn 档的「件另有改动」只提示，不改判）
//	1  有件但**无记录**（件里这个数字一条台账记录都没有）或**值与台账不符**（= 没记录的手改）⇒ 棘轮判红
//	2  用法错（多余位置参数 / `--json` 字段表外 / 点名条件不在册）· 仓根解析不到 ·
//	   台账解不动（硬错）· **有条件的现值读不到**（件不在 / 键不在 ⇒ 判不了 ⇒ 不给结论）
//
// 只读：`Stat` / `ReadFile` 两件（全部经棘轮的读取函数），**不写不删不改**任何件。
// 本件**不含**任何写盘站点，也没有干跑档（无第二执行语义）⇒ 干跑语义登记面无落点（不适用）。
package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// gateBaselineShowFields —— `--json` 的字段表（每条一行八格）。
//
// 前六格与棘轮 `--view --json` 的 rows 同名同义（`name`/`gate`/`file`/`value`/`verdict`/
// `verdict_text` —— 同一引擎出的同一批读数），后两格是本命令**派生**的两列
// （`ledger_value` = 台账最后一条的 `new`；`delta` = 现读 − 基线值；取不到 ⇒ null）。
var gateBaselineShowFields = []string{"name", "gate", "file", "value", "ledger_value", "delta", "verdict", "verdict_text"}

func gateBaselineFieldLegal(f string) bool {
	for _, ok := range gateBaselineShowFields {
		if f == ok {
			return true
		}
	}
	return false
}

// gateBaselineNames —— 条件表里的全部条件名（点错名时的「可用」清单）。
// 真源 = 棘轮自己的 `brConditions`（✗ 不另抄一份清单 —— 那正是第一〇六笔的「假过滤」病根）。
func gateBaselineNames() []string {
	out := []string{}
	for _, c := range brConditions {
		out = append(out, c.name)
	}
	sort.Strings(out)
	return out
}

// gateBaselineLine —— 呈现用的一行（棘轮 row + 台账末条派生的两列）。
type gateBaselineLine struct {
	cond     brCond
	cur      *int
	base     *int
	delta    *int
	sha      string
	verdict  string
	verdictT string
}

// gateBaselineVerdictShort —— 棘轮判词的**短标签**（人面表格用；原话仍逐条可读 —— 见
// `gateBaselineShow` 的「逐条判词」段，红/判不了两档必印原话）。
func gateBaselineVerdictShort(v string, hasBase bool) string {
	switch v {
	case "ok":
		return "值齐"
	case "warn":
		return "值齐（件另有改动）"
	case "n/a":
		return "判不了"
	case "red":
		if !hasBase {
			return "无记录"
		}
		return "值与台账不符"
	}
	return v
}

// gateBaselineShow —— 本动作的实现体（由 `gate.go` 的 `case "baseline"` 分派）。
func gateBaselineShow(inv *invocation, stdout, stderr io.Writer, root string) int {
	if len(inv.args) > 1 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: `gate baseline show` 只收**一个**条件名（多给了 %d 个）⇒ 用法错 2\n", progName, len(inv.args))
		fmt.Fprintf(stderr, "   可用：%s\n", strings.Join(gateBaselineNames(), " · "))
		return exitUsage
	}
	name := ""
	if len(inv.args) == 1 {
		name = strings.TrimSpace(inv.args[0])
	}
	// 字段面（**执行前判**）：不给字段 = 闭集全量（与 `gate results --json` 同一条口径）。
	fields := inv.fields
	if len(fields) == 0 {
		fields = gateBaselineShowFields
	}
	for _, f := range fields {
		if !gateBaselineFieldLegal(f) {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, inv.path, f)
		}
	}
	// 仓根：缺省 = 命令面现算的仓根（与 `gate` 族其余动作同一枚）；`--root` 走棘轮的同一口径上溯
	// （= 「对仓外副本/夹具根读同一批基线」的正门，与 `gate run --root` 同形）。
	// ★ `--root` **裸给**（给过旗标、没给值）⇒ 2：照 `gate run --root` 的先例**不静默回落本仓**
	//   （回落 = 把「读别处」悄悄读成本仓 ⇒ 读数看着有、对象是错的）。
	if inv.hasFlag("--root") && strings.TrimSpace(inv.flagVal("--root")) == "" {
		fmt.Fprintf(stderr, "%s: `--root` 给了但**没给值** ⇒ 用法错 2（✗ 不静默回落本仓）\n", progName)
		fmt.Fprintf(stderr, "   形态：`zerg gate baseline show [<条件名>] --root <目录>`\n")
		return exitUsage
	}
	broot := strings.TrimSpace(inv.flagVal("--root"))
	if broot == "" {
		broot = root
	}
	// 根判据**与棘轮同一枚**（`brFindRoot`：上溯找 `scripts/gates/precommit-gates.sh` + `core/go.mod`）：
	// 找不到 ⇒ 2 —— ✗ 不拿「随便一个目录」当基线树读（读不到件会被显示成「判不了」，
	// 而真错因是根指错了；先分家再读，别让两种错混成一个）。
	thisRoot, ok := brFindRoot(broot)
	if !ok {
		fmt.Fprintf(stderr, "%s: ⛔ 找不到仓根（从 %s 上溯找不到 %s）⇒ 不给结论（rc=2）\n",
			progName, broot, strings.Join(brRootMarkers, " + "))
		return exitUsage
	}
	broot = thisRoot
	only := []string{}
	if name != "" {
		only = append(only, name)
	}
	rows, hard, soft := brCollect(broot, brConditions, nil, false, only, false)
	if len(rows) == 0 {
		fmt.Fprintf(stderr, "%s: ⛔ 条件名不认：%q ⇒ 不给结论（rc=2）\n   可用：%s\n",
			progName, name, strings.Join(gateBaselineNames(), " · "))
		return exitUsage
	}
	// 台账末条（基线值那一格的**唯一**来源 = `brLastEntryFor`，与 `brVerdictOf` 同一处定义）。
	entries, lerr := brLoadLedger(broot)
	if lerr != nil {
		hard = append(hard, "台账解不动："+*lerr)
	}
	lines := []gateBaselineLine{}
	nOK, nWarn, nMismatch, nNoRec, nNA := 0, 0, 0, 0, 0
	for _, r := range rows {
		l := gateBaselineLine{cond: r.cond, cur: r.cur, verdict: r.verdict, verdictT: r.verdictText}
		if r.sha != nil {
			l.sha = *r.sha
		}
		last := brLastEntryFor(entries, r.cond.name)
		if last != nil {
			if n, ok := brSeqInt(last["new"]); ok {
				v := n
				l.base = &v
			}
		}
		if l.cur != nil && l.base != nil {
			d := *l.cur - *l.base
			l.delta = &d
		}
		switch r.verdict {
		case "ok":
			nOK++
		case "warn":
			nWarn++
		case "n/a":
			nNA++
		case "red":
			if l.base == nil {
				nNoRec++
			} else {
				nMismatch++
			}
		}
		lines = append(lines, l)
	}
	// 退码：棘轮同一判（hard ⇒ 2 · 任一 red ⇒ 1 · 否则 0）+ 本面**不继承**的 n/a 口子。
	rc := brVerdictRC(rows, hard)
	if nNA > 0 && rc == 0 {
		rc = exitUsage
	}
	if inv.jsonGiven {
		gateBaselineJSON(stdout, broot, len(entries), fields, lines, hard, soft, rc,
			map[string]int{"条件": len(lines), "值齐": nOK + nWarn, "不符": nMismatch, "无记录": nNoRec, "判不了": nNA})
	} else {
		gateBaselineText(stdout, broot, len(entries), lines, hard, soft,
			nOK, nWarn, nMismatch, nNoRec, nNA)
	}
	// 判语落在哪一路：机器面（`--json`）**只许 stdout 一个 JSON 包封** ⇒ 判语走 stderr
	//（违者：`… --json | jq` 直接炸「Extra data」—— 棘轮 `--view --json` 今天就是这个毛病）。
	tw := stdout
	if inv.jsonGiven {
		tw = stderr
	}
	if len(hard) > 0 {
		fmt.Fprint(tw, "⇒ **不给结论（rc=2）**：台账本身解不动/形状坏了（逐条见上）\n")
		return exitUsage
	}
	if nNA > 0 {
		fmt.Fprintf(tw, "⇒ **不给结论（rc=2）**：%d 个条件**现值读不到**（件不在 / 键不在）—— 「读不到」不是绿\n", nNA)
		fmt.Fprint(tw, "   判据名词面在棘轮自己的那条路上：`zerg gate check baseline-ratchet --view`（同一条引擎 · 同一条台账）\n")
		return exitUsage
	}
	if rc == 1 {
		fmt.Fprintf(tw, "⇒ **判红（rc=1）**：%d 条无记录 · %d 条与台账不符（逐条见上）\n", nNoRec, nMismatch)
		fmt.Fprint(tw, "   闭洞的唯一通道：`zerg gate check baseline-ratchet --write --reason '…'`（改数 + 留痕）或 `--anchor --reason '…'`（只记现状、不改件）\n")
		return 1
	}
	fmt.Fprint(tw, "⇒ 绿（rc=0）：每个棘轮数字都在台账里找得到**自己那条**记录，且现读 = 基线（差 +0）\n")
	return 0
}

// gateBaselineText —— 人面：表头 + 逐条一行 + 合计 + 红/判不了两档的原话段。
func gateBaselineText(w io.Writer, root string, nEntries int, lines []gateBaselineLine, hard, soft []string,
	nOK, nWarn, nMismatch, nNoRec, nNA int) {
	fmt.Fprint(w, "── zerg gate baseline —— 现读 × 基线（只读 · 不写任何件 · 读路径 = baseline-ratchet 同一引擎）──\n")
	state := "**件还不存在**"
	if brIsFile(brLedgerPath(root)) {
		state = "件在"
	}
	fmt.Fprintf(w, "   仓根 %s\n", root)
	fmt.Fprintf(w, "   台账 %s（%d 条记录 · %s）\n", brLedgerRel, nEntries, state)
	nameW, gateW, fileW, curW, baseW := 6, 4, 4, 5, 6
	for _, l := range lines {
		nameW = gateBaselineMax(nameW, l.cond.name)
		gateW = gateBaselineMax(gateW, l.cond.gate)
		fileW = gateBaselineMax(fileW, l.cond.file)
		curW = gateBaselineMax(curW, gateBaselineNum(l.cur))
		baseW = gateBaselineMax(baseW, gateBaselineNum(l.base))
	}
	if fileW > 46 {
		fileW = 46
	}
	row := func(name, gate, file, cur, base, delta, verdict string) {
		fmt.Fprintf(w, "   %s  %s  %s  %s  %s  %s  %s\n",
			gateBaselinePad(name, nameW), gateBaselinePad(gate, gateW), gateBaselinePad(file, fileW),
			gateBaselinePad(cur, curW), gateBaselinePad(base, baseW), gateBaselinePad(delta, 4), verdict)
	}
	row("条件名", "门", "件", "现值", "基线值", "差", "判")
	for _, l := range lines {
		row(l.cond.name, l.cond.gate, l.cond.file,
			gateBaselineNum(l.cur), gateBaselineNum(l.base), gateBaselineDelta(l.delta),
			gateBaselineVerdictShort(l.verdict, l.base != nil))
	}
	fmt.Fprintf(w, "   合计 %d 条 · 值齐 %d（其中件另有改动 %d）· 与台账不符 %d · 无记录 %d · 判不了 %d\n",
		len(lines), nOK+nWarn, nWarn, nMismatch, nNoRec, nNA)
	// 红 / 判不了两档**必印棘轮原话**（短标签不许替掉判词）。
	printed := false
	for _, l := range lines {
		if l.verdict == "red" || l.verdict == "n/a" {
			if !printed {
				fmt.Fprint(w, "   逐条判词（棘轮原话）：\n")
				printed = true
			}
			fmt.Fprintf(w, "     · %s：%s\n", l.cond.name, l.verdictT)
		}
	}
	for _, e := range hard {
		fmt.Fprintf(w, "   ⛔ 台账硬错：%s\n", e)
	}
	for _, e := range soft {
		fmt.Fprintf(w, "   ⚠ 台账提示（不改判）：%s\n", e)
	}
}

// gateBaselineCellW —— 表格一格的**显示宽度**（CJK / 全角 / 圈号按 2 计）：
// 按 rune 数补空格会让带汉字的列在终端里错位半格（同族的 `brPrintRows` 就是这么歪的）。
func gateBaselineCellW(s string) int {
	w := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || (r >= 0x2460 && r <= 0x24FF) || (r >= 0x3000 && r <= 0x303F) || (r >= 0xFF00 && r <= 0xFF60) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

func gateBaselinePad(s string, w int) string {
	if d := w - gateBaselineCellW(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// gateBaselineMax —— 该列宽（按显示宽度取最大；`gateBaselinePad` 与它同一口径）。
func gateBaselineMax(cur int, s string) int {
	if n := gateBaselineCellW(s); n > cur {
		return n
	}
	return cur
}

// gateBaselineNum —— 数字格：取不到 ⇒ 「—」（✗ 不印 0 冒充「读到了 0」）。
func gateBaselineNum(p *int) string {
	if p == nil {
		return "—"
	}
	return strconv.Itoa(*p)
}

// gateBaselineDelta —— 差这一格：带符号（+0 也是 0 ⇒ 印 `+0`，让「读到了且相等」与「取不到」形不混）。
func gateBaselineDelta(p *int) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%+d", *p)
}

// gateBaselineJSON —— 机器面：`rows[]` 按 `--json <字段>` 投影（不给字段 = 八格全量）。
func gateBaselineJSON(w io.Writer, root string, nEntries int, fields []string, lines []gateBaselineLine,
	hard, soft []string, rc int, counts map[string]int) {
	rowsArr := []any{}
	for _, l := range lines {
		full := map[string]any{
			"name":         l.cond.name,
			"gate":         l.cond.gate,
			"file":         l.cond.file,
			"value":        brIntOrNil(l.cur),
			"ledger_value": brIntOrNil(l.base),
			"delta":        brIntOrNil(l.delta),
			"verdict":      l.verdict,
			"verdict_text": l.verdictT,
		}
		o := brNewObj()
		for _, f := range fields {
			o.put(f, full[f])
		}
		rowsArr = append(rowsArr, o)
	}
	co := brNewObj()
	for _, k := range []string{"条件", "值齐", "不符", "无记录", "判不了"} {
		co.put(k, counts[k])
	}
	payload := brNewObj(
		"mode", "show",
		"root", root,
		"ledger", brLedgerRel,
		"ledger_entries", nEntries,
		"rc", rc,
		"counts", co,
		"rows", rowsArr,
		"ledger_hard_errors", brStrsAny(hard),
		"ledger_soft_notes", brStrsAny(soft),
	)
	brWriteJSON(w, payload)
}
