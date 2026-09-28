// family_gap_state.go —— `gap` 族的两条**状态写面**（`set-state` / `note` · 2026-09-26）。
//
// 病（本笔补的就是这一格 · 账内 `Q-239` 已登记）：`gap` 族此前只有 `ls` / `add` / `verify` /
// `export` 四条 ⇒ **改态与写口径没有正门**。`verify` 改态的**唯一**通路是「跑 `verify_cmd` 看 rc」
// —— 于是「这条其实已经由别的活儿解掉了」这类**人给的口径**无处可写，只剩手改真源（违规 ✗）。
// 本件只补这条正门，**不重写** `add` / `verify` / `ls` / `export` 的任何一段。
//
// 两条新动作（**不新增顶层命令**：仍旧挂 `gap` 族）：
//
//	⒜ `zerg gap set-state <GAP id> --state <仍缺|已解> --evidence <一句话> [--by <谁>] [--dry-run | --yes]`
//	   —— **改态**：写 `state` / `solved_at`（只在转 `已解` 时取）/ `solved_evidence`。
//	⒝ `zerg gap note <GAP id> --text <一句话> [--by <谁>] [--dry-run | --yes]`
//	   —— **口径注**：往该条的 `notes` 数组**追加**一条（**不改 `state`** ✗）。
//
// 四条照同族写面（`add` / `verify`）逐字沿用的纪律：
//
//	① **三态**（本仓 `repo commit` / `calib run` / `gap add` 先例）：`--dry-run` 恒 0（零副作用，
//	   计划件走 **stdout**）· 缺 `--yes` fail-closed **2**（计划件走 **stderr**，一行真源都不读）
//	   · `--yes` 才真写。**从不提问、从不交互**（§4.1 K6）。
//	② **审计先落盘**（取 `dev edit` 那一侧）：写不进审计 ⇒ 真源**一个字节不写** ⇒ `8`。
//	   审计 = 与 `dev edit` **同一件** `edit_audit.jsonl`（`editAuditPath()` · 事件名沿用
//	   `gap_ledger_written` · 格位沿用 `gapAuditLine`）—— 不另开第二件、不另造事件名。
//	③ **写后读回对拍**：整件重写之后现算 sha256，必须等于审计里那一格 ⇒ 对不上不给结论（`8`）。
//	④ **闭集三值**：`--state` 只认 `gapStateSettable`（`仍缺` / `已解` / `不做`）；闭集外 ⇒ **2 并逐字印闭集**。
//	   机器态 `回归` 与排期态 `已派` / `已立项` **不在本动作的闭集里** —— 它只回答
//	   「这条**还在不在** / **还要不要做**」这一问（`已派` / `已立项` 各有各的排期面，本笔不替它们开口子 ✗）。
//
// 退码（一律引现有表 `exitcodes.go` · 本族**不取新号**）：
//
//	`set-state` : 0 落账或幂等「无变化」· 2 用法错（缺 id / id 多于一条 / 缺 `--evidence` /
//	              `--state` 不在闭集 / 收到 `--text`）· 2 点名点在账外（**取定见下**）·
//	              8 真源读不到 / 不在盘上 / 审计写不进 / 真源写不进 / 写回读不对拍
//	`note`      : 0 落账或幂等「无变化」· 2 用法错（缺 id / id 多于一条 / 缺 `--text` /
//	              收到 `--state` —— 「注不得改态」）· 2 点名点在账外 · 8 同 `set-state`
//
// ★ **「不存在的 id」为什么退 2**（设计稿未钉死、本件按最小惊讶取定，回执里照实点名）：
//
//	同族的 `gap verify` 在这一格退 `8`（`blocked` · 逐字「取数不到」）。本件**不跟它**，理由两条：
//	  · `8` 在本族的口径是「**真源那一层**不行」（不在盘上 / 在盘上读不出来 / 审计写不进）——
//	    而「账**读得到**、只是点名点到账外」不是那一类 ⇒ 借 `8` 会让「读不到」与「名给错」同码；
//	  · 「名给错」= **用法错**（改用法即可重试 · `exitcodes.go` 逐字「改用法后可重试」）⇒ 取 `2`。
//	  也不取 `4`（`unauthenticated` = 未认证）：语义不符，占号纪律不是「随便挑一个空号」。
//
// ★ **幂等**（`H` 档口径 · 本件取定）：**同一 id** 上「要写的值逐字已在位」⇒ 报「无变化」
//
//	（`rc=0` · `meta.changed=false`）且**不写真源、不写审计**（不造历史噪声）。
//	  · `set-state`：「状态**与**证据都逐字相同」才算无变化；**状态同而证据不同** ⇒ 真写
//	    （状态没动、口径变了 ⇒ 仍是一次有信息的写入 · `changed=true`）。
//	  · `note`：目标条里已有**同 by 同 text** 的一条（时刻那一格**不比** —— 它每次都不同）⇒ 无变化。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// gapStateSettable —— `set-state` 的**闭集**（人的口径面：三值 —— 「还在不在 / 还要不要做」都能答）。
// 为什么不是 `gapStateClosed`（六值）：`已派` / `已立项` 是「排期」面、`回归` 是 `verify` 跑判据判出来的
// ⇒ 本动作**不替这两类开口子** ✗；而 `不做` 是**人拍板**的处置结论（与 `仍缺` / `已解` 同属人的口径面）
// ⇒ 2026-09-27 起收进闭集（原注「`不做` 另有其面」已不成立，逐字改掉）。
var gapStateSettable = []string{gapStOpen, gapStSolved, gapStWontDo}

// gapNoteSep —— 一条注的形态：`<时刻> · <谁>：<文本>`（时刻那一格**不参与幂等比对**）。
const gapNoteSep = "："

// gapSetStateFields / gapNoteFields —— `--json` 可取字段（与命令树里的 `fields` 同一份口径）。
var gapSetStateFields = []string{"id", "fp", "state", "changed"}
var gapNoteFields = []string{"id", "fp", "notes", "changed"}

// gapIDsOf —— 位置参数里的 id 集（空串丢掉；与 `verify` 同一条收法）。
func gapIDsOf(inv *invocation) []string {
	ids := []string{}
	for _, a := range inv.args {
		if s := strings.TrimSpace(a); s != "" {
			ids = append(ids, s)
		}
	}
	return ids
}

// gapFindIdx —— 账内按 id 找下标（找不到 ⇒ -1）。**只按 id**：`fp` 是身份指纹，
// 对外身份是 `id`（设计句见 `family_gap.go` 头部 · `Q-nnn` / 本机序号那一档）。
func gapFindIdx(led gapLedger, id string) int {
	for i, r := range led.Recs {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// gapRewriteOne —— 两条新写面的**唯一收口**：算新件 → 审计先落盘 → 整件重写 → 写后读回对拍。
//
// 返回 `(detail, rc, msg)`：`rc==0` 即真写成功（`detail`/`msg` 为空）。
// **未动的行逐字节照原样写回**（`led.Lines` 是原文，不是重序列化）—— 与 `verify` 同一条口径。
func gapRewriteOne(inv *invocation, led gapLedger, idx int, rec gapRecord, cmdName, before, after string) (string, int, string) {
	enc, err := json.Marshal(rec)
	if err != nil {
		return "ledger_encode_failed", exitFail, err.Error()
	}
	lines := make([]string, 0, len(led.Lines))
	for i, raw := range led.Lines {
		if i == idx {
			lines = append(lines, string(enc))
			continue
		}
		lines = append(lines, raw)
	}
	body := []byte{}
	for _, ln := range lines {
		body = append(body, []byte(ln+"\n")...)
	}
	audit := gapAuditLine{
		At: time.Now().Format(time.RFC3339), Event: gapEventName, GapCmd: cmdName,
		GapID: rec.ID, GapFP: rec.FP, GapStateBefore: before, GapStateAfter: after,
		GapLedgerPath: led.Path, GapBeforeSHA: sha256Of(led.Raw), GapAfterSHA: sha256Of(body),
		GapBeforeLines: len(led.Lines), GapAfterLines: len(lines),
		By: gapByOf(inv), Confirm: "--yes",
	}
	if err := appendGapAudit(editAuditPath(), audit); err != nil {
		return "audit_unwritable", exitBlocked, err.Error()
	}
	if err := gapWriteLedger(led.Path, lines); err != nil {
		return "ledger_unwritable", exitBlocked, err.Error()
	}
	back, err := os.ReadFile(led.Path)
	if err != nil || sha256Of(back) != audit.GapAfterSHA {
		return "ledger_readback_mismatch", exitBlocked, "写回读对不上"
	}
	return "", exitOK, ""
}

// gapStatePlanBlock —— 两条新写面的计划件（`--dry-run` 走 stdout · 缺 `--yes` 走 stderr）。
// 形状照 `gapPlanBlock`（同族的 `add` 那一版）：先点真源与读没读，再逐格摊开**要写的那几键**。
func gapStatePlanBlock(w io.Writer, title, cmdName, ledgerPath string, lines int, id, fp, before, after, line1, line2 string) {
	fmt.Fprintf(w, "计划件（%s · 零副作用 —— 未改真源、未写审计）\n", title)
	if lines < 0 {
		fmt.Fprintf(w, "  真源     : %s（未读 —— 缺 `--yes` ⇒ 不执行）\n", ledgerPath)
	} else {
		fmt.Fprintf(w, "  真源     : %s（现有 %d 行）\n", ledgerPath, lines)
	}
	fmt.Fprintf(w, "  目标     : %s\n", id)
	if fp != "" {
		fmt.Fprintf(w, "  fp       : %s\n", fp)
	}
	fmt.Fprintf(w, "  要落的键（逐格）：\n")
	if after != "" {
		fmt.Fprintf(w, "    state        : %s → %s\n", before, after)
	} else {
		fmt.Fprintf(w, "    state        : %s（本动作**不改**这一格）\n", before)
	}
	fmt.Fprintf(w, "    %s\n", line1)
	if line2 != "" {
		fmt.Fprintf(w, "    %s\n", line2)
	}
	fmt.Fprintf(w, "  审计     : 计划写一行 `event=%s` + `gap_cmd=%s`（这一态**不写**）\n", gapEventName, cmdName)
}

// gapReadLedgerOrDie —— 两条新写面共用的读真源两态（照 `verify`：**读得到**是前提，
// 缺件与读不出来**各自**退 8 且 `reason` 机器可辨 —— `Q-138` 那一档）。
// 返回 `(led, rc)`：`rc==0` ⇒ 读到了；否则 `rc` 就是要退的码（错已印完）。
func gapReadLedgerOrDie(inv *invocation, stderr io.Writer) (gapLedger, int) {
	led, err := readGapLedger()
	if err != nil {
		gapLedgerErr(inv, gapReasonPrecondition, err.Error(), gapLedgerPath())
		gapLedgerErrFirstLine(stderr, gapReasonPrecondition)
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "真源 = %s；「读不到」不许当绿（退码 8）\n", gapLedgerPath())
		gapLedgerUnreadableHint(stderr, gapLedgerPath())
		return led, exitBlocked
	}
	if !led.Exists {
		gapLedgerErr(inv, gapReasonLedgerAbsent, "真源不在盘上", led.Path)
		gapLedgerErrFirstLine(stderr, gapReasonLedgerAbsent)
		fmt.Fprintf(stderr, "%s: 真源不在盘上：%s（退码 8 —— 「读不到」不许当绿）\n", progName, led.Path)
		gapLedgerAbsentHint(stderr, led.Path)
		return led, exitBlocked
	}
	return led, exitOK
}

// gapTargetOne —— 两条新写面共用的**点名面**（恰好一条 id）。
// `rc==0` ⇒ `idx` 可用；否则 `rc` 为要退的码（错已印完）：
// 位置参数条数不为 1 ⇒ 2（用法面）· 账内没有这个 id ⇒ 2（**取定 · 见件头 ★**）。
func gapTargetOne(inv *invocation, led gapLedger, stderr io.Writer, cmdName string) (int, int) {
	ids := gapIDsOf(inv)
	if len(ids) != 1 {
		inv.setErr("usage", "target_arity", "要点名**恰好一条**缺口 id")
		if len(ids) == 0 {
			fmt.Fprintf(stderr, "%s: `gap %s` 要给目标：`zerg gap %s <GAP id> …`（恰好一条）\n", progName, cmdName, cmdName)
		} else {
			fmt.Fprintf(stderr, "%s: `gap %s` 只收**恰好一条**缺口 id（收到 %d 条：%s）\n",
				progName, cmdName, len(ids), strings.Join(ids, " "))
			fmt.Fprintf(stderr, "  一次只改一条 —— 要改多条就逐条跑（本族不做批量改态：一条一审计行才追得回）\n")
		}
		return -1, exitUsage
	}
	idx := gapFindIdx(led, ids[0])
	if idx < 0 {
		inv.setErr("usage", "target_not_found", "账内没有 "+ids[0])
		fmt.Fprintf(stderr, "%s: 账内没有 %s（账里 %d 条）—— 「名给错」是**用法错** ⇒ 退码 2\n",
			progName, ids[0], len(led.Recs))
		fmt.Fprintf(stderr, "  看账 : `zerg gap ls`（本命令**不建**新条目 —— 记新缺口走 `zerg gap add`）\n")
		fmt.Fprintf(stderr, "  ⚠ 本格与 `gap verify` **不同码**（那里是 `8`）：`8` 在本族是「真源那一层不行」，\n")
		fmt.Fprintf(stderr, "     而这里是「账读得到、点名点到账外」⇒ 归用法面（件头 ★ 逐条写明）\n")
		return -1, exitUsage
	}
	return idx, exitOK
}

// ── ⒜ `zerg gap set-state <GAP id> --state <仍缺|已解> --evidence <一句话>` ──────────────────────

func cmdGapSetState(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	stateWant := strings.TrimSpace(inv.flagVal("--state"))
	evidence := strings.TrimSpace(inv.flagVal("--evidence"))

	// ① 用法面（在任何盘面动作之前 —— 与 `add` / `verify` 同一条位置）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapSetStateFields, ","))
		return exitUsage
	}
	if len(inv.flagVals("--text")) > 0 {
		v := strings.TrimSpace(inv.flagVal("--text"))
		inv.setErr("usage", "text_not_in_shape", "--text 不在 set-state 的形状里")
		fmt.Fprintf(stderr, "%s: `gap set-state` 不收 `--text %s`（要写口径注 ⇒ `zerg gap note <GAP id> --text …`）\n",
			progName, v)
		return exitUsage
	}
	if stateWant == "" {
		inv.setErr("usage", "missing_required", "缺 --state")
		fmt.Fprintf(stderr, "%s: `gap set-state` 缺必填旗标：--state\n", progName)
		fmt.Fprintf(stderr, "用法：zerg gap set-state <GAP id> --state <%s> --evidence <一句话> [--by <谁>] [--dry-run | --yes]\n",
			strings.Join(gapStateSettable, "|"))
		return exitUsage
	}
	if !gapIn(gapStateSettable, stateWant) {
		inv.setErr("usage", "bad_state", "state 值不在本动作闭集里")
		fmt.Fprintf(stderr, "%s: `--state %s` 不在闭集里 —— 只认 %s\n",
			progName, stateWant, gapClosedText(gapStateSettable))
		fmt.Fprintf(stderr, "  闭集三值（`仍缺` / `已解` / `不做`）：本动作只回答「这条**还在不在**」（`回归` 由 `zerg gap verify` 跑判据转）\n")
		return exitUsage
	}
	if evidence == "" {
		inv.setErr("usage", "missing_required", "缺 --evidence")
		fmt.Fprintf(stderr, "%s: `gap set-state` 缺必填旗标：--evidence（**改态必留一句证据** —— 与 `add` 的「留证据」同一条纪律）\n", progName)
		return exitUsage
	}

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed **rc=2**，计划件走 stderr（同族 ④）。
	//    ⚠ 判在**读真源之前**：确认档不齐 ⇒ 一行都不读、一行都不写。
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if !inv.dryRun && !inv.yes {
		ids := gapIDsOf(inv)
		id := "（未给：缺必填）"
		if len(ids) == 1 {
			id = ids[0]
		}
		gapStatePlanBlock(stderr, "缺 `--yes`（D2 档）", "set-state", gapLedgerPath(), -1,
			id, "", "（未读）", stateWant,
			"evidence     : "+evidence, "")
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		fmt.Fprintf(stderr, "  ⚠ `--yes` 是**命令行确认档**，不是 `approve` 件（不产生 `approver` / `approval` 两格）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	// ③ 读真源（`--dry-run` 与 `--yes` 都要它：读不到 ⇒ 8）
	led, rc := gapReadLedgerOrDie(inv, stderr)
	if rc != exitOK {
		return rc
	}

	// ④ 点名（恰好一条 · 账内没有 ⇒ 2）
	idx, rc := gapTargetOne(inv, led, stderr, "set-state")
	if rc != exitOK {
		return rc
	}
	r := led.Recs[idx]

	// ⑤ 幂等：状态**与**证据都逐字已在位 ⇒ 「无变化」（0 · 不写真源、不写审计）
	if r.State == stateWant && r.Evidence == evidence {
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				map[string]string{"id": r.ID, "fp": r.FP, "state": r.State, "changed": "false"})
		}
		fmt.Fprintf(stdout, "无变化 %s · state=%s（同 id 同态同证据 ⇒ 幂等命中：不写真源、不写审计）\n", r.ID, r.State)
		return exitOK
	}

	// ⑥ `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		if inv.jsonGiven {
			inv.changed = boolPtr(false)
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				map[string]string{"id": r.ID, "fp": r.FP, "state": stateWant, "changed": "false"})
		}
		gapStatePlanBlock(stdout, "--dry-run", "set-state", led.Path, len(led.Lines),
			r.ID, r.FP, r.State, stateWant,
			fmt.Sprintf("solved_at    : %s", gapSolvedAtPlan(r.State, stateWant)),
			"evidence     : "+evidence)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}

	// ⑦ 真写：`state` + `solved_at`（只在**转** `已解` 时取）+ `solved_evidence`。
	//    `solved_at` 在转回 `仍缺` 时**不删**（与 `verify` 记 `回归` 同一条追加式口径：
	//    `state` 是**现态**、`solved_at` 是**历史时刻**，两格不是同一维；历史不抹）。
	before := r.State
	r.State = stateWant
	r.Evidence = evidence
	if stateWant == gapStSolved {
		r.SolvedAt = gapNow()
	}
	detail, rc, msg := gapRewriteOne(inv, led, idx, r, "set-state", before, stateWant)
	if rc != exitOK {
		gapWriteFail(inv, stderr, detail, msg, "set-state")
		return rc
	}
	inv.changed = boolPtr(!(before == stateWant && led.Recs[idx].Evidence == evidence))
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
			map[string]string{"id": r.ID, "fp": r.FP, "state": r.State, "changed": boolWord(before != stateWant || led.Recs[idx].Evidence != evidence)})
	}
	fmt.Fprintf(stdout, "已改态 %s · state %s → %s（真源 %d 行不变 · 只重写目标那一行 · 审计已落 1 行）\n",
		r.ID, before, r.State, len(led.Lines))
	fmt.Fprintf(stdout, "  solved_evidence : %s\n", r.Evidence)
	if stateWant == gapStSolved {
		fmt.Fprintf(stdout, "  solved_at       : %s\n", r.SolvedAt)
	} else {
		fmt.Fprintf(stdout, "  solved_at       : %s（历史时刻**不删** —— 追加式口径）\n", orDash(r.SolvedAt))
	}
	return exitOK
}

// gapSolvedAtPlan —— 计划件里 `solved_at` 那一行（真写那一刻才取）。
func gapSolvedAtPlan(_, want string) string {
	if want == gapStSolved {
		return "（真写那一刻取 · RFC3339Nano）"
	}
	return "（转 `仍缺` 时**不删**历史值）"
}

// ── ⒝ `zerg gap note <GAP id> --text <一句话>` ─────────────────────────────────────────────────

func cmdGapNote(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	text := strings.TrimSpace(inv.flagVal("--text"))
	// ★ 2026-09-28（缺口账 `GAP-20260926-233`）：`--retract <n|指纹>` = **作废**一条已落的注。
	//   纪律：**禁真删任何注** —— 被作废那条**原样留在 `notes` 里**，只往 `notes_void` 追加标记。
	retract := strings.TrimSpace(inv.flagVal("--retract"))

	// ① 用法面（在任何盘面动作之前）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapNoteFields, ","))
		return exitUsage
	}
	// ★ 「注**不得改态**」的显式闸：形状里根本没有 `--state` ⇒ 收到就拒（不静默忽略）。
	if len(inv.flagVals("--state")) > 0 {
		v := strings.TrimSpace(inv.flagVal("--state"))
		inv.setErr("usage", "state_not_in_shape", "--state 不在 note 的形状里")
		fmt.Fprintf(stderr, "%s: `gap note` 不收 `--state %s`（形状 = `<GAP id> --text <一句话> [--by <谁>] [--dry-run | --yes]`）\n", progName, v)
		fmt.Fprintf(stderr, "  注**不改态**：改态走 `zerg gap set-state <GAP id> --state <%s> --evidence <一句话>`\n",
			strings.Join(gapStateSettable, "|"))
		return exitUsage
	}
	if retract != "" && text != "" {
		inv.setErr("usage", "retract_with_text", "--retract 与 --text 同给")
		fmt.Fprintf(stderr, "%s: `--retract` 与 `--text` **不许同给**（一条命令一个动作：作废 = 不写新注 · 写新注 = 不作废）\n", progName)
		fmt.Fprintf(stderr, "  作废：zerg gap note <GAP id> --retract <n|指纹> [--by <谁>] [--dry-run | --yes]\n")
		fmt.Fprintf(stderr, "  写注：zerg gap note <GAP id> --text <一句话> [--by <谁>] [--dry-run | --yes]\n")
		return exitUsage
	}
	if text == "" && retract == "" {
		inv.setErr("usage", "missing_required", "缺 --text")
		fmt.Fprintf(stderr, "%s: `gap note` 缺必填旗标：--text（或 `--retract <n|指纹>`）\n", progName)
		fmt.Fprintf(stderr, "用法：zerg gap note <GAP id> --text <一句话> [--by <谁>] [--dry-run | --yes]\n")
		fmt.Fprintf(stderr, "   或：zerg gap note <GAP id> --retract <n|指纹> [--by <谁>] [--dry-run | --yes]（作废一注 · 不删原文）\n")
		return exitUsage
	}

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed **rc=2**，计划件走 stderr
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	if !inv.dryRun && !inv.yes {
		ids := gapIDsOf(inv)
		id := "（未给：缺必填）"
		if len(ids) == 1 {
			id = ids[0]
		}
		plan1, plan2 := "notes        : （在当前那一条之后追加 1 条 · 不改 state）", "text         : "+text
		if retract != "" {
			plan1 = "notes_void   : （追加 1 条作废标记 · **注一条不删** · 不改 state）"
			plan2 = "void         : --retract " + retract + "（名字在真源读进来之后才判）"
		}
		gapStatePlanBlock(stderr, "缺 `--yes`（D2 档）", "note", gapLedgerPath(), -1,
			id, "", "（未读）", "",
			plan1, plan2)
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		fmt.Fprintf(stderr, "  ⚠ `--yes` 是**命令行确认档**，不是 `approve` 件（不产生 `approver` / `approval` 两格）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	// ③ 读真源（读不到 ⇒ 8）
	led, rc := gapReadLedgerOrDie(inv, stderr)
	if rc != exitOK {
		return rc
	}

	// ④ 点名（恰好一条 · 账内没有 ⇒ 2）
	idx, rc := gapTargetOne(inv, led, stderr, "note")
	if rc != exitOK {
		return rc
	}
	r := led.Recs[idx]
	by := gapByOf(inv)

	// ⑤′ `--retract`：**只追加一条作废标记**（禁真删 · 已被作废 ⇒ 幂等「无变化」）
	if retract != "" {
		return gapNoteRetract(inv, stdout, stderr, led, idx, r, retract)
	}

	// ⑤ 幂等：目标条里已有**同 by 同 text** 的一条（时刻那一格不比）⇒ 「无变化」
	for _, n := range r.Notes {
		if strings.HasSuffix(n, by+gapNoteSep+text) {
			inv.changed = boolPtr(false)
			if inv.jsonGiven {
				return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
					map[string]string{"id": r.ID, "fp": r.FP, "notes": fmt.Sprintf("%d", len(r.Notes)), "changed": "false"})
			}
			fmt.Fprintf(stdout, "无变化 %s · notes=%d 条（同 by 同 text 已在位 ⇒ 幂等命中：不写真源、不写审计）\n",
				r.ID, len(r.Notes))
			return exitOK
		}
	}

	// ⑥ `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		if inv.jsonGiven {
			inv.changed = boolPtr(false)
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				map[string]string{"id": r.ID, "fp": r.FP, "notes": fmt.Sprintf("%d", len(r.Notes)+1), "changed": "false"})
		}
		gapStatePlanBlock(stdout, "--dry-run", "note", led.Path, len(led.Lines),
			r.ID, r.FP, r.State, "",
			fmt.Sprintf("notes        : %d → %d 条（追加在末尾）", len(r.Notes), len(r.Notes)+1),
			fmt.Sprintf("新那一条     : %s · %s%s%s", "（真写那一刻取）", by, gapNoteSep, text))
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}

	// ⑦ 真写：**只**追加 `notes` 一条；`state` 逐字不动（改前改后现算对拍，破 ⇒ 不给结论）。
	before := r.State
	r.Notes = append(append([]string{}, r.Notes...), fmt.Sprintf("%s · %s%s%s", gapNow(), by, gapNoteSep, text))
	if r.State != before {
		inv.setErr("failed", "state_mutated", "注这一路改了 state")
		fmt.Fprintf(stderr, "%s: 内部对拍破了：注这一路改了 `state`（%s → %s）⇒ 不给结论（退码 1）\n", progName, before, r.State)
		return exitFail
	}
	detail, rc, msg := gapRewriteOne(inv, led, idx, r, "note", before, before)
	if rc != exitOK {
		gapWriteFail(inv, stderr, detail, msg, "note")
		return rc
	}
	inv.changed = boolPtr(true)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
			map[string]string{"id": r.ID, "fp": r.FP, "notes": fmt.Sprintf("%d", len(r.Notes)), "changed": "true"})
	}
	fmt.Fprintf(stdout, "已落注 %s · notes %d → %d 条（真源 %d 行不变 · 只重写目标那一行 · 审计已落 1 行）\n",
		r.ID, len(r.Notes)-1, len(r.Notes), len(led.Lines))
	fmt.Fprintf(stdout, "  state    : %s（**未改** —— 注不改态）\n", r.State)
	fmt.Fprintf(stdout, "  新那一条 : %s\n", r.Notes[len(r.Notes)-1])
	return exitOK
}

// ── ⒞ `zerg gap note <GAP id> --retract <n|指纹>`（作废一条注 · 缺口账 `GAP-20260926-233`）──────

// gapNoteRetractPick —— `--retract` 的点名面：序号（1 起）/ 指纹（sha256 前 12 位）/ 原文前缀
// 三选一，返回**全部命中**的序号（调用方判「恰好一条」）。序号会随追加漂移，指纹钉住原文。
func gapNoteRetractPick(r gapRecord, arg string) []int {
	hits := []int{}
	if n, err := strconv.Atoi(arg); err == nil {
		if n >= 1 && n <= len(r.Notes) {
			hits = append(hits, n)
		}
		return hits
	}
	for i, s := range r.Notes {
		if strings.HasPrefix(gapNoteFP(s), arg) || strings.HasPrefix(s, arg) {
			hits = append(hits, i+1)
		}
	}
	return hits
}

// gapNoteRetract —— `--retract` 的真写面：**只往 `notes_void` 追加一条作废标记**。
// ★ 真删任何注 ✗（被作废那一条在 `notes` 里**逐字节原样留着**，改前改后现算对拍）。
// 退码：0 落账或幂等「无变化」· 2 点名不到 / 命中多条 / 与 `--text` 同给 · 8 同 `note`。
func gapNoteRetract(inv *invocation, stdout, stderr io.Writer, led gapLedger, idx int, r gapRecord, arg string) int {
	hits := gapNoteRetractPick(r, arg)
	if len(hits) == 0 {
		inv.setErr("usage", "retract_target_not_found", "点名点不到注")
		fmt.Fprintf(stderr, "%s: `--retract %s` 在 %s 的 %d 条注里**一条都对不上**（本族口径 = 用法错 2 · 名给错）\n",
			progName, arg, r.ID, len(r.Notes))
		fmt.Fprintf(stderr, "  点名三种写法：序号（1 起）/ 指纹（sha256 前 12 位）/ 原文前缀 —— 现读：zerg gap ls --json id,notes,notes_void,void_notes\n")
		return exitUsage
	}
	if len(hits) > 1 {
		inv.setErr("usage", "retract_target_ambiguous", "点名命中多于一条注")
		fmt.Fprintf(stderr, "%s: `--retract %s` 在 %s 里命中 %d 条（%v）⇒ 收窄到唯一（用序号或指纹）\n",
			progName, arg, r.ID, len(hits), hits)
		return exitUsage
	}
	n := hits[0]
	// 幂等（★ `GAP-20260928-167`）：这条注已在作废位 **且** state 已是闭集里的 `不做` ⇒ 「无变化」
	// （只作废不改态的老账行会落到下面真写那一路 —— 补态，仍幂等一次到位）。
	for _, v := range r.Voids {
		if v.N == n && r.State == gapStWontDo {
			inv.changed = boolPtr(false)
			if inv.jsonGiven {
				return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
					map[string]string{"id": r.ID, "fp": r.FP, "notes": fmt.Sprintf("%d", len(r.Notes)), "changed": "false"})
			}
			fmt.Fprintf(stdout, "无变化 %s · 第 %d 条注**已在作废位**（void:true · 幂等命中：不写真源、不写审计）\n", r.ID, n)
			return exitOK
		}
	}
	// `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		if inv.jsonGiven {
			inv.changed = boolPtr(false)
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				map[string]string{"id": r.ID, "fp": r.FP, "notes": fmt.Sprintf("%d", len(r.Notes)), "changed": "false"})
		}
		gapStatePlanBlock(stdout, "--dry-run", "note", led.Path, len(led.Lines), r.ID, r.FP, r.State, "",
			fmt.Sprintf("notes_void   : %d → %d 条（追加作废标记 · **注一条不删**）· state: %s → %s（★ `GAP-20260928-167`：同一次写盘里改态）", len(r.Voids), len(r.Voids)+1, r.State, gapStWontDo),
			fmt.Sprintf("作废那一条   : 第 %d 条 · fp=%s（void:true · 原文在真源里逐字节留着）", n, gapNoteFP(r.Notes[n-1])))
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}
	// 真写（★ 缺口账 `GAP-20260928-167`）：**同一次写盘**里落一条作废标记 + 把该条 `state` 置闭集里的
	//   `不做`（撤回在旗舰读数上生效）；`notes` 逐字不动（真删注 ✗ · 改前改后现算对拍，破 ⇒ 不给结论）。
	before := r.State
	snapNotes := strings.Join(r.Notes, "\x00")
	r.Voids = append(append([]gapNoteVoid{}, r.Voids...), gapNoteVoid{
		Void: true, N: n, FP: gapNoteFP(r.Notes[n-1]), By: gapByOf(inv), At: gapNow(), Why: "撤回",
	})
	r.State = gapStWontDo
	if strings.Join(r.Notes, "\x00") != snapNotes {
		inv.setErr("failed", "note_mutated", "作废这一路动了 notes/state")
		fmt.Fprintf(stderr, "%s: 内部对拍破了：作废这一路动了 `notes` ⇒ 不给结论（退码 1）\n", progName)
		return exitFail
	}
	detail, rc, msg := gapRewriteOne(inv, led, idx, r, "note", before, r.State)
	if rc != exitOK {
		gapWriteFail(inv, stderr, detail, msg, "note")
		return rc
	}
	inv.changed = boolPtr(true)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
			map[string]string{"id": r.ID, "fp": r.FP, "notes": fmt.Sprintf("%d", len(r.Notes)), "changed": "true"})
	}
	fmt.Fprintf(stdout, "已作废 %s · 第 %d 条注（fp=%s · notes_void %d → %d 条）\n",
		r.ID, n, gapNoteFP(r.Notes[n-1]), len(r.Voids)-1, len(r.Voids))
	fmt.Fprintf(stdout, "  真源     : %s（%d 行不变 · **注一条没删** —— 只重写目标那一行）\n", led.Path, len(led.Lines))
	fmt.Fprintf(stdout, "  读面     : 该条已不显示（默认）· `--json void_notes` 带原文出来\n")
	return exitOK
}

// ── 两条新写面共用的报错面（审计写不进 / 真源写不进 / 写回读不对拍）──────────────────────────

// gapWriteFail —— 真写那三步失败的**唯一报面**（`set-state` / `note` 共用）。
func gapWriteFail(inv *invocation, stderr io.Writer, detail, msg, cmdName string) {
	switch detail {
	case "audit_unwritable":
		inv.setErr("blocked", detail, msg)
		fmt.Fprintf(stderr, "%s: 审计写不进 ⇒ **真源一个字节不改**（审计先落盘 · 退码 8）：%v\n", progName, msg)
		fmt.Fprintf(stderr, "  审计落点 : %s\n", editAuditPath())
	case "ledger_unwritable":
		inv.setErr("blocked", detail, msg)
		fmt.Fprintf(stderr, "%s: 真源写不进（审计那一行已落盘 · 退码 8）：%v\n", progName, msg)
	case "ledger_readback_mismatch":
		inv.setErr("blocked", detail, msg)
		fmt.Fprintf(stderr, "%s: 写后读回对不上（审计里的 `gap_ledger_after_sha256`）⇒ 不给结论（退码 8）\n", progName)
	default:
		inv.setErr("failed", detail, msg)
		fmt.Fprintf(stderr, "%s: `%s` 真源这一行序列化不过（退码 1）：%v\n", progName, cmdName, msg)
	}
}

// boolWord —— 布尔值的人面/机器面同一副词。
func boolWord(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ── ⒟ `zerg gap assign <GAP id> --egg <卵号> …`（批3 第一片 · 作业单 + 派单登记 · 2026-09-28）──────
//
// 设计出处（唯一真源 · `设计-缺口账与自进化-v2.0-20260928.md`）：§11.3（派单登记 = 写 `已派` +
// `notes` 记卵号与回执绝对路径）· §4 判据 2（自派：`已派` 条数 == 在飞卵数；作业单必须带占用状态）·
// §8 `M-32`（「同改文件只许一路」⇒ `assign` 前先查占用）。
//
// 四条硬纪律（与同族写面逐字同款 · 本片的核心判据）：
//
//	① **只走现有改态路径**：`state` 与 `notes` 登记**同一次**重写 —— 只调本族唯一收口
//	   `gapRewriteOne`（审计先落盘 → 整件重写 → 读回对拍），**不另开写路**。
//	② **六栏缺栏拒发**：作业单六栏（① 允许面 ② 禁碰面 ③ 占用件 ④ 出口判据 ⑤ 时限 ⑥ 该条全文）
//	   少一栏 ⇒ 退码 2 并**点名缺哪一栏**。
//	③ `--dry-run` 只出计划件（零副作用 · 恒 0）· 缺 `--yes` fail-closed 2（D2 档）。
//	④ 预检「可派状态」：默认**只认 `仍缺`**（非此 ⇒ 用法错 2 并点名当前 `state`）· 占用冲突 ⇒ 2。

// gapStAssigned —— 派单落的那一格（`已派` · 本片**首次真启用**：现读 0 条）。
const gapStAssigned = "已派"

// gapAssignFrom —— 「可派状态」闭集（默认只 `仍缺`：`已派` ⇒ 已在飞 · `已解` ⇒ 已闭环 · `不做` ⇒ 已拍死）。
var gapAssignFrom = []string{gapStOpen}

// gapAssignMarker —— 派单登记那一行的**机读标记**（读面按它筛出作业单登记 · 不改 `notes` 的形状）。
const gapAssignMarker = "派单登记 "

var gapAssignFields = []string{"id", "fp", "state", "egg", "allow", "forbid", "occupies",
	"criterion", "deadline", "receipt", "changed"}
var gapAssignListFields = []string{"id", "state", "egg", "allow", "forbid", "occupies",
	"criterion", "deadline", "receipt", "assigned_at", "unit", "module"}

// gapAssignForm —— 作业单六栏的机器面（栏序照设计稿 · ① 允许面 ② 禁碰面 ③ 占用件 ④ 出口判据 ⑤ 时限）。
type gapAssignForm struct {
	Egg       string
	Allow     string
	Forbid    string
	Occupies  string
	Criterion string
	Deadline  string
	Receipt   string
}

// gapAssignFlagMissing —— ①~⑤ 五栏的**旗标面**缺面点名（读真源之前就能判 · 栏序 = 设计稿 §11.4）。
func gapAssignFlagMissing(f gapAssignForm) []string {
	miss := []string{}
	if f.Allow == "" {
		miss = append(miss, "① 允许面（--allow）")
	}
	if f.Forbid == "" {
		miss = append(miss, "② 禁碰面（--forbid）")
	}
	if f.Occupies == "" {
		miss = append(miss, "③ 占用件（--occupies）")
	}
	if f.Criterion == "" {
		miss = append(miss, "④ 出口判据（--criterion）")
	}
	if f.Deadline == "" {
		miss = append(miss, "⑤ 时限（--deadline）")
	}
	return miss
}

// gapAssignFullTextMissing —— ⑥「该条全文」那一栏（读真源之后判：两格全空 ⇒ 缺栏）。
func gapAssignFullTextMissing(r gapRecord) bool {
	return strings.TrimSpace(r.Symptom) == "" && strings.TrimSpace(r.Handmade) == ""
}

// gapAssignReceiptDefault —— 回执绝对路径的兜底（`--receipt` 未给时取定 · 本片口径）：
// `<真源所在目录>/assign-receipts/<卵号>.md`（绝对路径 ⇒ 设计稿 §11.3「notes 记卵号与回执绝对路径」）。
func gapAssignReceiptDefault(egg string) string {
	dir := filepath.Dir(gapLedgerPath())
	if strings.TrimSpace(egg) == "" {
		return dir
	}
	return filepath.Join(dir, "assign-receipts", egg+".md")
}

// gapAssignLine —— 落到 `notes` 上的**一行机读登记**（形态照同族注：`<时刻> · <谁>：<文本>`，
// 文本以 `派单登记 ` 起头 · 逐格 `k=v` · 分隔 ` · ` ⇒ `gapAssignParse` 能逐格读回）。
func gapAssignLine(by, at string, f gapAssignForm) string {
	return fmt.Sprintf("%s · %s%s%segg=%s · receipt=%s · allow=%s · forbid=%s · occupies=%s · criterion=%s · deadline=%s",
		at, by, gapNoteSep, gapAssignMarker, f.Egg, f.Receipt, f.Allow, f.Forbid, f.Occupies, f.Criterion, f.Deadline)
}

// gapAssignParse —— 从一条注里逐格读回派单登记（不是登记 ⇒ `ok=false`）。
func gapAssignParse(note string) (gapAssignForm, string, bool) {
	i := strings.Index(note, gapAssignMarker)
	if i < 0 {
		return gapAssignForm{}, "", false
	}
	f := gapAssignForm{}
	at := ""
	if j := strings.Index(note, " · "); j > 0 {
		at = note[:j]
	}
	for _, kv := range strings.Split(note[i+len(gapAssignMarker):], " · ") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "egg":
			f.Egg = v
		case "allow":
			f.Allow = v
		case "forbid":
			f.Forbid = v
		case "occupies":
			f.Occupies = v
		case "criterion":
			f.Criterion = v
		case "deadline":
			f.Deadline = v
		case "receipt":
			f.Receipt = v
		}
	}
	if f.Egg == "" {
		return gapAssignForm{}, "", false
	}
	return f, at, true
}

// gapAssignOf —— 一条账上的**最新**派单登记（从 `notes` 末尾往前找第一条命中登记的注）。
func gapAssignOf(r gapRecord) (gapAssignForm, string, bool) {
	for i := len(r.Notes) - 1; i >= 0; i-- {
		if f, at, ok := gapAssignParse(r.Notes[i]); ok {
			return f, at, true
		}
	}
	return gapAssignForm{}, "", false
}

// gapAssignOccupies —— 占用件那一栏的**件清单**（`无` / 空 / `-` ⇒ 不占件 · 分隔 `,` / `，` / `、` / 空白）。
func gapAssignOccupies(s string) []string {
	out := []string{}
	for _, t := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ' ' || r == '\t'
	}) {
		t = strings.TrimSpace(t)
		if t == "" || t == "无" || t == "-" {
			continue
		}
		out = append(out, t)
	}
	return out
}

// gapAssignConflict —— `M-32` 占用预检：别的 `已派` 条若已占了同一件 ⇒ 报（谁 · 哪件）。
func gapAssignConflict(led gapLedger, selfIdx int, occupies string) (string, string) {
	mine := gapAssignOccupies(occupies)
	if len(mine) == 0 {
		return "", ""
	}
	for i, r := range led.Recs {
		if i == selfIdx || r.State != gapStAssigned {
			continue
		}
		f, _, ok := gapAssignOf(r)
		if !ok {
			continue
		}
		for _, t := range gapAssignOccupies(f.Occupies) {
			for _, m := range mine {
				if t == m {
					return r.ID, m
				}
			}
		}
	}
	return "", ""
}

// cmdGapAssign —— `zerg gap assign <GAP id> --egg …`：**派单登记**（写面）。
func cmdGapAssign(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}

	// ① 用法面（在任何盘面动作之前 —— 与 `set-state` / `note` 同一条位置）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapAssignFields, ","))
		return exitUsage
	}
	if len(inv.flagVals("--state")) > 0 {
		v := strings.TrimSpace(inv.flagVal("--state"))
		inv.setErr("usage", "state_not_in_shape", "--state 不在 assign 的形状里")
		fmt.Fprintf(stderr, "%s: `gap assign` 不收 `--state %s` —— 本动作落的态**恒为 `%s`**（派单登记即改态）\n", progName, v, gapStAssigned)
		return exitUsage
	}
	f := gapAssignForm{
		Egg:       strings.TrimSpace(inv.flagVal("--egg")),
		Allow:     strings.TrimSpace(inv.flagVal("--allow")),
		Forbid:    strings.TrimSpace(inv.flagVal("--forbid")),
		Occupies:  strings.TrimSpace(inv.flagVal("--occupies")),
		Criterion: strings.TrimSpace(inv.flagVal("--criterion")),
		Deadline:  strings.TrimSpace(inv.flagVal("--deadline")),
		Receipt:   strings.TrimSpace(inv.flagVal("--receipt")),
	}
	if f.Egg == "" {
		inv.setErr("usage", "missing_required", "缺 --egg")
		fmt.Fprintf(stderr, "%s: `gap assign` 缺必填旗标：--egg（派单登记的卵号 —— 设计稿 §11.3 逐字）\n", progName)
		fmt.Fprintf(stderr, "用法：zerg gap assign <GAP id> --egg <卵号> --allow <件清单> --forbid <面> --occupies <件|无> --criterion <出口判据> --deadline <绝对日期> [--receipt <回执绝对路径>] [--by <谁>] [--dry-run | --yes] [--json <字段>]\n")
		return exitUsage
	}
	if miss := gapAssignFlagMissing(f); len(miss) > 0 {
		inv.setErr("usage", "order_missing_section", "作业单缺栏（缺 "+strconv.Itoa(len(miss))+" 栏）")
		fmt.Fprintf(stderr, "%s: 作业单**缺栏拒发** —— 缺 %d 栏：%s\n", progName, len(miss), strings.Join(miss, " · "))
		fmt.Fprintf(stderr, "  六栏（设计稿 §11.4）：① 允许面 `--allow <件清单>` · ② 禁碰面 `--forbid <面>` · ③ 占用件 `--occupies <件|无>` · ④ 出口判据 `--criterion <可机检的一句>` · ⑤ 时限 `--deadline <绝对日期>` · ⑥ 该条全文（账上那条自己）\n")
		fmt.Fprintf(stderr, "  ⚠ 缺一栏就**不发**（一个字节都不写）—— 六栏齐才可能落到真源上\n")
		return exitUsage
	}
	if f.Receipt == "" {
		f.Receipt = gapAssignReceiptDefault(f.Egg)
	}

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed **rc=2**，计划件走 stderr（判在读真源之前）。
	if !inv.dryRun && !inv.yes {
		ids := gapIDsOf(inv)
		id := "（未给：缺必填）"
		if len(ids) == 1 {
			id = ids[0]
		}
		gapStatePlanBlock(stderr, "缺 `--yes`（D2 档）", "assign", gapLedgerPath(), -1,
			id, "", "（未读）", gapStAssigned,
			"作业单六栏  : 齐（①~⑤ 已给 · ⑥ 读真源后判）",
			"登记       : "+gapAssignMarker+"egg="+f.Egg+" · receipt="+f.Receipt)
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		fmt.Fprintf(stderr, "  ⚠ `--yes` 是**命令行确认档**，不是 `approve` 件（不产生 `approver` / `approval` 两格）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	// ③ 读真源（读不到 ⇒ 8）
	led, rc := gapReadLedgerOrDie(inv, stderr)
	if rc != exitOK {
		return rc
	}

	// ④ 点名（恰好一条 · 账内没有 ⇒ 2）
	idx, rc := gapTargetOne(inv, led, stderr, "assign")
	if rc != exitOK {
		return rc
	}
	r := led.Recs[idx]

	// ⑤ 作业单第 ⑥ 栏（该条全文）—— 账上两格全空 ⇒ 缺栏
	if gapAssignFullTextMissing(r) {
		inv.setErr("usage", "order_missing_section", "作业单缺栏（缺 1 栏）")
		fmt.Fprintf(stderr, "%s: 作业单**缺栏拒发** —— 缺 1 栏：⑥ 该条全文（%s 的 `symptom` 与 `handmade` 两格都是空的）\n",
			progName, r.ID)
		return exitUsage
	}

	// ⑥ 幂等：已是 `已派` 且已有**同卵号**登记 ⇒ 「无变化」（不写真源、不写审计）
	//    ★ 判在**可派预检之前**：同卵重派是幂等命中（0），不是「不可派」（2）—— 两档别混。
	if r.State == gapStAssigned {
		if old, _, ok := gapAssignOf(r); ok && old.Egg == f.Egg {
			inv.changed = boolPtr(false)
			if inv.jsonGiven {
				return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
					gapAssignRow(r, old, "", "false"))
			}
			fmt.Fprintf(stdout, "无变化 %s · state=%s · egg=%s（同卵号登记已在位 ⇒ 幂等命中：不写真源、不写审计）\n",
				r.ID, r.State, f.Egg)
			return exitOK
		}
	}

	// ⑦ 可派预检：默认只认 `仍缺`（非此 ⇒ 2 并点名当前 state）
	if !gapIn(gapAssignFrom, r.State) {
		inv.setErr("usage", "state_not_assignable", "当前 state 不可派")
		fmt.Fprintf(stderr, "%s: `%s` 当前 state = **%s** ⇒ 不可派（本片可派闭集 = %s）\n",
			progName, r.ID, r.State, gapClosedText(gapAssignFrom))
		fmt.Fprintf(stderr, "  为什么：`已派` ⇒ 已在飞（要换卵走 `unassign` 再派）· `已解` / `不做` ⇒ 已闭环 ⇒ 不重复派\n")
		return exitUsage
	}

	// ⑧ 占用预检（`M-32`：同改文件只许一路）
	if who, tok := gapAssignConflict(led, idx, f.Occupies); who != "" {
		inv.setErr("usage", "occupies_conflict", "占用件已被另一条已派占住")
		fmt.Fprintf(stderr, "%s: 占用冲突 —— 件 `%s` 已被**已派**条 `%s` 占住（`M-32`：同改文件只许一路）\n",
			progName, tok, who)
		fmt.Fprintf(stderr, "  看在飞 : `zerg gap assign ls`（本动作**不发**）\n")
		return exitUsage
	}

	by := gapByOf(inv)
	line := gapAssignLine(by, gapNow(), f)

	// ⑨ `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				gapAssignRow(r, f, "", "false"))
		}
		gapAssignPlanBlock(stdout, "--dry-run", led, r, f, "（真写那一刻取）", line)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}

	// ⑩ 真写：`state` → `已派` + `notes` 追加登记（**同一次**重写 · **走现有改态路径**）
	before := r.State
	r.State = gapStAssigned
	r.Notes = append(append([]string{}, r.Notes...), line)
	if r.State != gapStAssigned || len(r.Notes) == 0 || !strings.Contains(r.Notes[len(r.Notes)-1], gapAssignMarker) {
		inv.setErr("failed", "state_mutated", "派单这一路没落到 已派")
		fmt.Fprintf(stderr, "%s: 内部对拍破了：派单没落到 `%s` ⇒ 不给结论（退码 1）\n", progName, gapStAssigned)
		return exitFail
	}
	detail, wrc, msg := gapRewriteOne(inv, led, idx, r, "assign", before, gapStAssigned)
	if wrc != exitOK {
		gapWriteFail(inv, stderr, detail, msg, "assign")
		return wrc
	}
	// 写后自证：读回那一条，`state` 与登记都在（读回对拍外再点一次名）
	back, err := readGapLedger()
	if err != nil {
		inv.setErr("failed", "readback_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写后读回真源失败（退码 1）：%v\n", progName, err)
		return exitFail
	}
	bidx := gapFindIdx(back, r.ID)
	if bidx < 0 {
		inv.setErr("failed", "readback_mismatch", "写后读回找不到目标条")
		fmt.Fprintf(stderr, "%s: 写后读回找不到 %s（退码 1）\n", progName, r.ID)
		return exitFail
	}
	bf, _, bok := gapAssignOf(back.Recs[bidx])
	if back.Recs[bidx].State != gapStAssigned || !bok || bf.Egg != f.Egg {
		inv.setErr("failed", "readback_mismatch", "写后读回与要写的不一致")
		fmt.Fprintf(stderr, "%s: 写后读回不一致（退码 1）：state=%s · 登记=%v\n", progName, back.Recs[bidx].State, bok)
		return exitFail
	}
	inv.changed = boolPtr(true)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
			gapAssignRow(back.Recs[bidx], bf, "", "true"))
	}
	nAssigned := 0
	for _, x := range back.Recs {
		if x.State == gapStAssigned {
			nAssigned++
		}
	}
	fmt.Fprintf(stdout, "已派单 %s · state %s → %s（真源 %d 行不变 · 只重写目标那一行 · 审计已落 1 行）\n",
		r.ID, before, gapStAssigned, len(led.Lines))
	fmt.Fprintf(stdout, "  作业单六栏 : ① %s ② %s ③ %s ④ %s ⑤ %s ⑥ %s\n",
		f.Allow, f.Forbid, f.Occupies, f.Criterion, f.Deadline, gapAssignFullText(r))
	fmt.Fprintf(stdout, "  登记那一行 : %s\n", back.Recs[bidx].Notes[len(back.Recs[bidx].Notes)-1])
	fmt.Fprintf(stdout, "  回执       : %s\n", f.Receipt)
	fmt.Fprintf(stdout, "  已派 条数  : %d（判据 2「自派」：== 在飞卵数）\n", nAssigned)
	return exitOK
}

// gapAssignFullText —— 第 ⑥ 栏（该条全文）的人面缩写：症状 + 手工命令（全文见 `zerg gap show`）。
func gapAssignFullText(r gapRecord) string {
	return fmt.Sprintf("symptom=%s | handmade=%s", gapFirstLine(r.Symptom), gapFirstLine(r.Handmade))
}

// gapAssignRow —— `--json` 那一行的逐格取值（写面与读面共用一份口径）。
func gapAssignRow(r gapRecord, f gapAssignForm, at, changed string) map[string]string {
	return map[string]string{
		"id": r.ID, "fp": r.FP, "state": r.State, "egg": f.Egg, "allow": f.Allow,
		"forbid": f.Forbid, "occupies": f.Occupies, "criterion": f.Criterion,
		"deadline": f.Deadline, "receipt": f.Receipt, "assigned_at": at, "changed": changed,
	}
}

// gapAssignPlanBlock —— `assign` 的计划件（`--dry-run` 走 stdout · 缺 `--yes` 走 stderr 的旁证）。
func gapAssignPlanBlock(w io.Writer, title string, led gapLedger, r gapRecord, f gapAssignForm, at, line string) {
	fmt.Fprintf(w, "计划件（派单登记 · %s · 零副作用 —— 未改真源、未写审计）\n", title)
	fmt.Fprintf(w, "  真源     : %s（现有 %d 行）\n", led.Path, len(led.Lines))
	fmt.Fprintf(w, "  目标     : %s（当前 state = %s）\n", r.ID, r.State)
	fmt.Fprintf(w, "  作业单六栏：\n")
	fmt.Fprintf(w, "    ① 允许面   : %s\n", f.Allow)
	fmt.Fprintf(w, "    ② 禁碰面   : %s\n", f.Forbid)
	fmt.Fprintf(w, "    ③ 占用件   : %s\n", f.Occupies)
	fmt.Fprintf(w, "    ④ 出口判据 : %s\n", f.Criterion)
	fmt.Fprintf(w, "    ⑤ 时限     : %s\n", f.Deadline)
	fmt.Fprintf(w, "    ⑥ 该条全文 : %s\n", gapAssignFullText(r))
	fmt.Fprintf(w, "  要落的键（逐格）：\n")
	fmt.Fprintf(w, "    state        : %s → %s\n", r.State, gapStAssigned)
	fmt.Fprintf(w, "    notes        : %d → %d 条（末尾追加那一行登记）\n", len(r.Notes), len(r.Notes)+1)
	fmt.Fprintf(w, "    登记那一行   : %s\n", line)
	fmt.Fprintf(w, "    receipt      : %s\n", f.Receipt)
	fmt.Fprintf(w, "  审计     : 计划写一行 `event=%s` + `gap_cmd=assign`（这一态**不写**）\n", gapEventName)
}

// ── ⒡ `zerg gap bulk set-state <清单件> --state <…>`（族级批量改态 · 批3 第五片 · 2026-09-28）──────
//
// 病（缺口账 `GAP-20260926-97` 逐字：想要 `zerg gap-batch-state`）：`gap` 族只有**单条**改态面
// （`set-state` 一次一条）⇒ 结账/收版时逐条改态成本高。本面 = 一次改 N 条。
//
// 四条硬纪律（本片的核心判据 · 逐条可对拍）：
//
//	① **全量预检先行**：清单任一条缺 `id` / `evidence`、或 `id` 不在账、或该条当前 `state` 不可改
//	   ⇒ **整批拒**（退码 2 · 逐条点名到行）· **一字不写**（真源一个字节不写、审计一行不落）。
//	② **全过后才真写**：逐条改态，逐条写**自己那一行**的 `evidence`（不是清单里第一行的口径）。
//	③ **半途不落半截**：任一条写失败 ⇒ **当场停手**并如实报「已改 m 条 / 未改 n 条」（不得静默）。
//	④ **不另开写路**：逐条走本族唯一收口 `gapRewriteOne`（审计先落盘 → 整件重写 → 写后读回对拍），
//	   审计**逐条**一行 —— 与单条 `set-state` 同一条路、同一张退码表，不另造机制。
//
// ★「当前 `state` 不可改」的口径（本片取定 · 回执如实点名）：**当前态必须是 `gapStateSettable`
// 三值之一**（`仍缺` / `已解` / `不做`）—— `已派` / `已立项` 是**排期面**（各有各的面管）、
// `回归` 是 `gap verify` 跑判据判出来的**机器态** ⇒ 批量面**不替它们改**（与单条 `set-state`
// 同一句口径逐字：本动作只回答「这条**还在不在** / **还要不要做**」）。
// ⚠ 单条 `set-state` **不加**这道从态闸（它逐条人点名）⇒ 单条行为**一个字节不动**（本片对拍项）。
//
// 退码（一律引现有表 `exitcodes.go` · 本族**不取新号**）：0 全成事或全幂等「无变化」·
// 1 真源这一行序列化不过 · 2 用法错（缺/多清单件 / `--state` 缺或闭集外 / 清单件空 /
// **任一条预检不合规** / 缺 `--yes`）· 8 清单件读不到 / 真源读不到 / 审计写不进 / 真源写不进。

var gapBulkSetStateFields = []string{"id", "before", "after", "evidence", "changed", "fail"}

// gapBulkRow —— 清单件一行（JSONL · 每行至少 `id` 与 `evidence` 两键）+ 本面的预检产物。
type gapBulkRow struct {
	No      int    // 清单件里的行号（1 起 · 不数空行）
	ID      string // `id` 键
	Ev      string // `evidence` 键（**这一行自己的**证据）
	Before  string // 改前 state（读账之后填）
	After   string // 改后 state
	Changed bool   // 真要写（同 id 同态同证据 ⇒ false = 幂等「无变化」）
	Done    bool   // 真写过（半途停手时用它数「已改 / 未改」）
	Fail    string // 非空 ⇒ 该行拒因（逐条点名）
}

// gapBulkReadList —— 读清单件（JSONL · 每行至少 `id` 与 `evidence` 两键）。
// 逐行判：不是 JSON 对象 / 缺键 / 值为空 ⇒ 落该行的 `Fail`（**不在读面退码** —— 退码由全量预检统一给）。
// 件读不到 ⇒ `error`（调用方退 8：本族口径「读不到不许当绿」）。
func gapBulkReadList(path string) ([]gapBulkRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rows := []gapBulkRow{}
	no := 0
	for _, ln := range strings.Split(string(raw), "\n") {
		s := strings.TrimSpace(ln)
		if s == "" {
			continue
		}
		no++
		r := gapBulkRow{No: no}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			r.Fail = fmt.Sprintf("第 %d 行：不是 JSON 对象（%v）", no, err)
			rows = append(rows, r)
			continue
		}
		r.ID = gapBulkStr(obj["id"])
		r.Ev = gapBulkStr(obj["evidence"])
		switch {
		case r.ID == "":
			r.Fail = fmt.Sprintf("第 %d 行：缺 `id` 键（或值不是非空字符串）", no)
		case r.Ev == "":
			r.Fail = fmt.Sprintf("第 %d 行：缺 `evidence` 键（或值不是非空字符串）—— 逐条改态要逐条给证据", no)
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// gapBulkStr —— 从一个 JSON 键取非空字符串（缺键 / 不是字符串 ⇒ 空串）。
func gapBulkStr(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

// gapBulkPrecheck —— **全量预检**（本片硬要求 ①）：逐条判、逐条填 `Before`/`Changed`/`Fail`，
// 返回**全部**拒因（空 ⇒ 整批可写）。**读面动作**：一个字都不写。
func gapBulkPrecheck(rows []gapBulkRow, led gapLedger, stateWant string) []string {
	bad := []string{}
	seen := map[string]int{}
	for i := range rows {
		r := &rows[i]
		if r.Fail != "" {
			bad = append(bad, r.Fail)
			continue
		}
		if n, dup := seen[r.ID]; dup {
			r.Fail = fmt.Sprintf("第 %d 行：id %s 与第 %d 行**重复**（同一条不许在同一批里改两次）", r.No, r.ID, n)
			bad = append(bad, r.Fail)
			continue
		}
		idx := gapFindIdx(led, r.ID)
		if idx < 0 {
			r.Fail = fmt.Sprintf("第 %d 行：id %s **不在账**（账里 %d 条）—— 「名给错」是用法错（同族 `set-state` 同一口径）",
				r.No, r.ID, len(led.Recs))
			bad = append(bad, r.Fail)
			continue
		}
		seen[r.ID] = r.No
		cur := led.Recs[idx]
		r.Before, r.After = cur.State, stateWant
		if !gapIn(gapStateSettable, cur.State) {
			r.Fail = fmt.Sprintf("第 %d 行：%s 当前 state=`%s` **不可改** —— 批量面只改 %s 三态之一（`已派`/`已立项` 是排期面、`回归` 是 `verify` 的机器态）",
				r.No, r.ID, cur.State, gapClosedText(gapStateSettable))
			bad = append(bad, r.Fail)
			continue
		}
		r.Changed = !(cur.State == stateWant && cur.Evidence == r.Ev)
	}
	return bad
}

// gapBulkPlanBlock —— 批量面的计划件（`--dry-run` 走 stdout · 缺 `--yes` 走 stderr）。
// `total < 0` ⇒ 清单件**未读**（缺 `--yes` 那一档：一个盘面动作都还没做）。
func gapBulkPlanBlock(w io.Writer, title, listPath, stateWant string, total int) {
	fmt.Fprintf(w, "计划件（族级批量改态 · %s · 零副作用 —— 未改真源、未写审计）\n", title)
	fmt.Fprintf(w, "  清单件   : %s\n", listPath)
	fmt.Fprintf(w, "  真源     : %s\n", gapLedgerPath())
	fmt.Fprintf(w, "  要落的键（逐条）：state → %s · solved_evidence → 该行自己的 `evidence`\n", stateWant)
	if total < 0 {
		fmt.Fprintf(w, "  条数     : （清单件未读 —— 缺 `--yes` ⇒ 不执行）\n")
	} else {
		fmt.Fprintf(w, "  条数     : %d 条（全量预检已过才真写；任一条不合规 ⇒ 整批拒 + 一字不写）\n", total)
	}
	fmt.Fprintf(w, "  审计     : 逐条计划写一行 `event=%s` + `gap_cmd=bulk set-state`（这一态**不写**）\n", gapEventName)
}

// gapBulkTable —— 表尾回显（总数 / 逐条改前 → 改后 / 失败条数）—— 人面与机器面**同一份数据**。
// `dry==true` ⇒ 这一态没有真写（`--dry-run` 的计划件）：未写的行标「计划改」而不是「半途停手」。
func gapBulkTable(w io.Writer, rows []gapBulkRow, stateWant string, m, n, failed int, dry bool) {
	fmt.Fprintf(w, "批量改态表（清单 %d 条 · 目标 state = %s）：\n", len(rows), stateWant)
	for i := range rows {
		r := &rows[i]
		mark := ""
		switch {
		case r.Fail != "":
			mark = "  ✗ " + r.Fail
		case r.Changed && r.Done:
			mark = "  ✓ 已改"
		case r.Changed && dry:
			mark = "  → 计划改（--dry-run：本态未执行）"
		case r.Changed && !r.Done:
			mark = "  － 未改（半途停手）"
		case !r.Changed:
			mark = "  = 无变化（同 id 同态同证据 ⇒ 幂等：不写真源、不写审计）"
		}
		before, after := r.Before, r.After
		if before == "" {
			before = "（未读）"
		}
		if after == "" {
			after = stateWant
		}
		fmt.Fprintf(w, "  第 %d 行 · %s · 改前 %s → 改后 %s%s\n", r.No, orDash(r.ID), before, after, mark)
	}
	fmt.Fprintf(w, "  表尾：总数 %d 条 · 已改 %d 条 · 未改 %d 条 · 失败 %d 条\n", len(rows), m, n, failed)
}

func cmdGapBulkSetState(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	stateWant := strings.TrimSpace(inv.flagVal("--state"))
	args := gapIDsOf(inv)

	// ① 用法面（在任何盘面动作之前 —— 与同族写面同一条位置）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapBulkSetStateFields, ","))
		return exitUsage
	}
	if stateWant == "" {
		inv.setErr("usage", "missing_required", "缺 --state")
		fmt.Fprintf(stderr, "%s: `gap bulk set-state` 缺必填旗标：--state\n", progName)
		fmt.Fprintf(stderr, "用法：zerg gap bulk set-state <清单件.jsonl> --state <%s> [--by <谁>] [--dry-run | --yes] [--json <字段>]\n",
			strings.Join(gapStateSettable, "|"))
		return exitUsage
	}
	if !gapIn(gapStateSettable, stateWant) {
		inv.setErr("usage", "bad_state", "state 值不在本动作闭集里")
		fmt.Fprintf(stderr, "%s: `--state %s` 不在闭集里 —— 只认 %s（与单条 `set-state` 同一闭集）\n",
			progName, stateWant, gapClosedText(gapStateSettable))
		return exitUsage
	}
	if len(args) != 1 {
		inv.setErr("usage", "target_arity", "要点名**恰好一个**清单件")
		fmt.Fprintf(stderr, "%s: `gap bulk set-state` 的第一位置参 = **清单件路径**（恰好一个 · 收到 %d 个）\n", progName, len(args))
		fmt.Fprintf(stderr, "  清单件 = JSONL：每行至少 `id` 与 `evidence` 两键（`{\"id\":\"GAP-…\",\"evidence\":\"…\"}`）\n")
		return exitUsage
	}
	listPath := args[0]

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed **rc=2**，计划件走 stderr。
	//    ⚠ 判在**读清单件与真源之前**：确认档不齐 ⇒ 一行都不读、一行都不写（与单条 `set-state` 逐字同位置）。
	if !inv.dryRun && !inv.yes {
		gapBulkPlanBlock(stderr, "缺 `--yes`（D2 档）", listPath, stateWant, -1)
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		fmt.Fprintf(stderr, "  ⚠ `--yes` 是**命令行确认档**，不是 `approve` 件（不产生 `approver` / `approval` 两格）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	// ③ 读清单件（读不到 ⇒ 8：本族口径「读不到不许当绿」）
	rows, err := gapBulkReadList(listPath)
	if err != nil {
		inv.setErr("blocked", "list_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 清单件读不到：%v\n", progName, err)
		fmt.Fprintf(stderr, "  清单件 = %s（退码 8 —— 取数不到不许当绿）\n", listPath)
		return exitBlocked
	}
	if len(rows) == 0 {
		inv.setErr("usage", "list_empty", "清单件里一行都没有")
		fmt.Fprintf(stderr, "%s: 清单件是空的（0 行）：%s —— 空清单不是「改完了」（用法错 2）\n", progName, listPath)
		return exitUsage
	}

	// ④ 读真源（读不到 ⇒ 8）
	led, rc := gapReadLedgerOrDie(inv, stderr)
	if rc != exitOK {
		return rc
	}

	// ⑤ **全量预检先行**：任一不合规 ⇒ 整批拒（2 · 逐条点名）· **一字不写**
	if bad := gapBulkPrecheck(rows, led, stateWant); len(bad) > 0 {
		inv.setErr("usage", "precheck_rejected", fmt.Sprintf("%d 条不合规", len(bad)))
		fmt.Fprintf(stderr, "%s: `gap bulk set-state` **整批拒**（%d 条不合规 ⇒ 退码 2 · **一字不写**：真源一个字节未改、审计一行未落）\n",
			progName, len(bad))
		fmt.Fprintf(stderr, "  清单件   : %s（%d 条）\n", listPath, len(rows))
		fmt.Fprintf(stderr, "  真源     : %s（现有 %d 行 · **本发未碰**）\n", led.Path, len(led.Lines))
		for _, b := range bad {
			fmt.Fprintf(stderr, "    ✗ %s\n", b)
		}
		fmt.Fprintf(stderr, "  修法     : 逐条补齐（每行 `id` + `evidence` 两键 · id 用 `zerg gap ls` 现读核对）后重发\n")
		if inv.jsonGiven {
			gapBulkMeta(inv, len(rows), 0, 0, len(bad), stateWant, led.Path)
			return emitGapBulkJSON(stdout, stderr, inv, rows)
		}
		return exitUsage
	}

	// ⑥ `--dry-run`：计划件（stdout · 恒 0 · 零副作用）
	if inv.dryRun {
		inv.changed = boolPtr(false)
		gapBulkMeta(inv, len(rows), 0, len(rows), 0, stateWant, led.Path)
		if inv.jsonGiven {
			return emitGapBulkJSON(stdout, stderr, inv, rows)
		}
		gapBulkPlanBlock(stdout, "--dry-run", listPath, stateWant, len(rows))
		gapBulkTable(stdout, rows, stateWant, 0, len(rows), 0, true)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}

	// ⑦ 真写：逐条走**现有单条改态路径**（`gapRewriteOne`）—— 逐条改态、逐条写自己的 evidence、
	//    审计逐条一行。任一条写失败 ⇒ **当场停手**（不静默 · 如实报「已改 m 条 / 未改 n 条」）。
	m, failed, failRC := 0, 0, exitOK
	for i := range rows {
		r := &rows[i]
		if !r.Changed {
			continue
		}
		fresh, rc := gapReadLedgerOrDie(inv, stderr)
		if rc != exitOK {
			r.Fail = "重读真源失败 ⇒ 当场停手（本条与后条一律未改）"
			failed, failRC = 1, rc
			break
		}
		idx := gapFindIdx(fresh, r.ID)
		if idx < 0 {
			r.Fail = "重读真源：该 id 不见了 ⇒ 当场停手（本条与后条一律未改）"
			failed, failRC = 1, exitBlocked
			break
		}
		rec := fresh.Recs[idx]
		before := rec.State
		rec.State = stateWant
		rec.Evidence = r.Ev
		if stateWant == gapStSolved {
			rec.SolvedAt = gapNow()
		}
		detail, rc, msg := gapRewriteOne(inv, fresh, idx, rec, "bulk set-state", before, stateWant)
		if rc != exitOK {
			gapWriteFail(inv, stderr, detail, msg, "bulk set-state")
			r.Fail = fmt.Sprintf("真写失败（%s）⇒ 当场停手：本条与后条一律未改", detail)
			failed, failRC = 1, rc
			break
		}
		r.Before, r.After, r.Done = before, stateWant, true
		m++
	}
	n := 0
	for i := range rows {
		if !rows[i].Done {
			n++
		}
	}
	inv.changed = boolPtr(m > 0)
	gapBulkMeta(inv, len(rows), m, n, failed, stateWant, led.Path)
	if inv.jsonGiven {
		if rc := emitGapBulkJSON(stdout, stderr, inv, rows); rc != exitOK {
			return rc
		}
	} else {
		gapBulkTable(stdout, rows, stateWant, m, n, failed, false)
		if failed > 0 {
			fmt.Fprintf(stdout, "  半途停手：已改 %d 条 / 未改 %d 条（真源 %s 行 · 审计已落 %d 行）—— 后条**一律未动**，不静默\n",
				m, n, gapBulkNowLines(), m)
		} else {
			fmt.Fprintf(stdout, "  已改 %d 条 / 未改 %d 条（幂等「无变化」不计改）· 真源 %s 行不变 · 只重写目标那几行 · 审计已落 %d 行\n",
				m, n, gapBulkNowLines(), m)
		}
	}
	if failed > 0 {
		return failRC
	}
	return exitOK
}

// gapBulkNowLines —— 真写之后真源的**现读**行数（半途停手时也如实报）。
func gapBulkNowLines() string {
	led, err := readGapLedger()
	if err != nil || !led.Exists {
		return "（读不回）"
	}
	return strconv.Itoa(len(led.Lines))
}

// gapBulkMeta —— 同族信封的元格（五格同形 + 本面四件派生格）。
func gapBulkMeta(inv *invocation, total, changed, unchanged, failed int, stateWant, ledgerPath string) {
	inv.metaAddStr("query", "bulk set-state")
	inv.metaAddStr("query_ts", gapNow())
	inv.metaAddStr("ledger_sha16", gapLsLedgerSHA16(ledgerPath))
	inv.metaAddJSON("total", strconv.Itoa(total))
	inv.metaAddJSON("changed_n", strconv.Itoa(changed))
	inv.metaAddJSON("unchanged_n", strconv.Itoa(unchanged))
	inv.metaAddStr("state", stateWant)
	inv.metaAddJSON("failed_n", strconv.Itoa(failed))
}

// emitGapBulkJSON —— `--json` 走同族信封面（逐条一行 · 与表尾**同一份数据**）。
func emitGapBulkJSON(stdout, stderr io.Writer, inv *invocation, rows []gapBulkRow) int {
	out := []map[string]string{}
	for i := range rows {
		r := &rows[i]
		out = append(out, map[string]string{
			"id": r.ID, "before": r.Before, "after": r.After, "evidence": r.Ev,
			"changed": boolWord(r.Changed && r.Done), "fail": r.Fail,
		})
	}
	return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, out)
}

// ── ⒠ `zerg gap assign ls [--egg <卵号>]`（只读面 · 批3 第一片）───────────────────────────────────
