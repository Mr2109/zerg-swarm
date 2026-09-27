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
	text := strings.TrimSpace(inv.flagVal("--text"))

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
	if text == "" {
		inv.setErr("usage", "missing_required", "缺 --text")
		fmt.Fprintf(stderr, "%s: `gap note` 缺必填旗标：--text\n", progName)
		fmt.Fprintf(stderr, "用法：zerg gap note <GAP id> --text <一句话> [--by <谁>] [--dry-run | --yes]\n")
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
		gapStatePlanBlock(stderr, "缺 `--yes`（D2 档）", "note", gapLedgerPath(), -1,
			id, "", "（未读）", "",
			"notes        : （在当前那一条之后追加 1 条 · 不改 state）", "text         : "+text)
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
