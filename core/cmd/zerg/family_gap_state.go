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
// ★ 本件另含两条**排期态**写面（缺口 `GAP-20261005-61` · 乙案 · 2026-10-05 · 三段链）：
//
//	⒢ `zerg gap project <GAP id> --evidence …`    —— **立项待拍**（源 {`仍缺`, `不做`} → 目标恒 `已立项`）
//	   `zerg gap unproject <GAP id> --evidence …` —— **撤立项**（源 {`已立项`} → 目标恒 `仍缺` · 三段链回退面）
//	   两动作**都不收 `--state`**（与 `assign` 同一条「各锁一个目标态」的纪律）。★ `已立项` 是**排期态**
//	   ⇒ 本件仍**不**把它并进 `gapStateSettable`（`set-state` / `bulk set-state` 的三值闭集与 usage
//	   **一字不动** ✗ —— 甲案已否，见设计稿 §2.3 / §7.1）。

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// gapStateSettable —— `set-state` 的**闭集**（人的口径面：三值 —— 「还在不在 / 还要不要做」都能答）。
// 为什么不是 `gapStateClosed`（六值）：`已派` / `已立项` 是「排期」面、`回归` 是 `verify` 跑判据判出来的
// ⇒ 本动作**不替这两类开口子** ✗；而 `不做` 是**人拍板**的处置结论（与 `仍缺` / `已解` 同属人的口径面）
// ⇒ 2026-09-27 起收进闭集（原注「`不做` 另有其面」已不成立，逐字改掉）。
var gapStateSettable = []string{gapStOpen, gapStSolved, gapStWontDo}

// gapSolvedKindClosed —— `已解` 的**分栏闭集**（缺口账 `GAP-20260927-249` · 「销案 ≠ 修」的机制化；
// 2026-10-10 · 缺口 `GAP-20260927-244` 方案乙加第四值 `部分`）：
// 「**实现完成**」与「**销案**（存量 / 账滞后 / 前提不成立）」此前压进同一个 `state=已解` ⇒ 已解计数
// **看不出哪些是真修的**。本闭集给 `--solved-kind` 四值：`实现` / `销案` / `前提不成立` / `部分`。
// ★ `部分`（方案乙）：**本条的已治面已销、残面另记** —— 它**仍是** `state=已解`：读者面的未闭/已闭
// 分档（`gapOpenStates` / `glvOpenStates` / `gsaOpenStates` · 门75 分子分母 · 门101）**一字不动**；
// 未治的残面一句话走 `set-state --remainder` 另落一格（`gapRecord.Remainder`）。
// gapSolvedKindPremiseInvalid —— "**前提不成立**"那一档（本枚 · 语义防护的靶）：它是**例外通道**
// （不受「未落提交面禁销」硬线约束）⇒ 填它时证据须点名一个**可现读的物**（见 `gapPremiseAnchorWhy`）。
const gapSolvedKindPremiseInvalid = "前提不成立"

var gapSolvedKindClosed = []string{"实现", "销案", gapSolvedKindPremiseInvalid, gapSolvedKindPartial}

// gapSolvedKindPartial —— 「**有残面的已解条**」那一档（缺口 `GAP-20260927-244` 方案乙）：本条的已治面
// 已销、残面另记（残面文字走 `set-state --remainder`）。★ 它与 `实现`/`销案`/`前提不成立` 平级在
// `gapSolvedKindClosed` 里 —— `state` 仍是 `已解`，读者面（未闭/已闭分档）一处分支都不动。
const gapSolvedKindPartial = "部分"

// gapSolvedKindUnclassified —— 老账（本格之前写的行）的**缺省读法**。★ **禁改存量行** —— 存量行没有
// `solved_kind` 格时，读面一律作 `未分类`（不写盘）。它与「真的一格都没有」同义。
const gapSolvedKindUnclassified = "未分类"

// gapSolvedKindFilterClosed —— `gap ls --solved-kind` 的**筛值闭集** = 分栏闭集 + `未分类`（老账缺省）。
// 只做**读数侧收窄**（「筛出口」：哪一批 `已解` 要列出来 ⇒ 能单列「有残面的已解条」）；★ 它**不进**
// 任何「未闭/已闭」判据（那是 `state` 那一维 —— 本值集只收窄打印面，不动任何分档）。
var gapSolvedKindFilterClosed = []string{"实现", "销案", "前提不成立", gapSolvedKindPartial, gapSolvedKindUnclassified}

// gapSolvedKindOf —— 账行的 `solved_kind`（空串 ⇒ `未分类`）。`gap ls` / `gap show` / `set-state`
// 三处**同一取值口**（不自创第二套）。
func gapSolvedKindOf(r gapRecord) string {
	// ★ state 闸（缺口账 `GAP-20261005-55`）：**非 `已解` 的行恒回 `未分类`**（构造性恒真 ——
	//   一格都不看 `SolvedKind`）。与同件写面「改到**非** `已解` 的态 ⇒ 本格一个字节不动」同款
	//   追加式口径配套：写面有意保留旧值（与 `solved_at` 同处置），读面凭 `state` 归一 ⇒
	//   `gap show --json` / `gap ls --json` 对退账残留行不再原样吐旧 kind（只 `--json` 面变，
	//   人面分栏 / 逐行 / `gap show` 人面早已自带 `state=已解` 闸，逐字节不变）。
	if r.State != gapStSolved {
		return gapSolvedKindUnclassified
	}
	if strings.TrimSpace(r.SolvedKind) == "" {
		return gapSolvedKindUnclassified
	}
	return r.SolvedKind
}

// gapSolvedKindSplitLine —— `gap ls` 人面的**报数分栏**（缺口账 `GAP-20260927-249`）：把筛后那一批里
// `state=已解` 的行按 `solved_kind` 数成「实现 N · 销案 N · 前提不成立 N · 部分 N · 未分类 N」。
// 返回 `(行, true)`；**筛后一条 `已解` 都没有 ⇒ `("", false)`**（老查询输出逐字节不变）。
// 只印**非零**栏（零栏不占位）：栏序固定照 `gapSolvedKindClosed`，`未分类` 收尾。
func gapSolvedKindSplitLine(out []gapRecord) (string, bool) {
	n := map[string]int{}
	total := 0
	for _, r := range out {
		if r.State != gapStSolved {
			continue
		}
		n[gapSolvedKindOf(r)]++
		total++
	}
	if total == 0 {
		return "", false
	}
	parts := []string{}
	for _, k := range gapSolvedKindClosed {
		if n[k] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", k, n[k]))
		}
	}
	if n[gapSolvedKindUnclassified] > 0 {
		parts = append(parts, fmt.Sprintf("%s %d", gapSolvedKindUnclassified, n[gapSolvedKindUnclassified]))
	}
	return fmt.Sprintf("  已解分栏（solved_kind）：%s（已解 %d 条）", strings.Join(parts, " · "), total), true
}

// gapSolvedKindCountsText —— `recs` 里 `state=已解` 的行按 `solved_kind` 的计数文本
// （`实现 N · 销案 N · 前提不成立 N · 部分 N · 未分类 N` · 只印非零栏）。`gap export` 件头用（缺口账
// `GAP-20260927-249`）。一条已解都没有 ⇒ `（无已解）`。
func gapSolvedKindCountsText(recs []gapRecord) string {
	n := map[string]int{}
	total := 0
	for _, r := range recs {
		if r.State != gapStSolved {
			continue
		}
		n[gapSolvedKindOf(r)]++
		total++
	}
	if total == 0 {
		return "（无已解）"
	}
	parts := []string{}
	for _, k := range gapSolvedKindClosed {
		if n[k] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", k, n[k]))
		}
	}
	if n[gapSolvedKindUnclassified] > 0 {
		parts = append(parts, fmt.Sprintf("%s %d", gapSolvedKindUnclassified, n[gapSolvedKindUnclassified]))
	}
	return strings.Join(parts, " · ")
}

// gapNoteSep —— 一条注的形态：`<时刻> · <谁>：<文本>`（时刻那一格**不参与幂等比对**）。
const gapNoteSep = "："

// gapNoteLine —— 一条注的**成文**：`<时刻> · <谁>：<文本>`。落注面的**唯一真源** ——
// `gap note <id> --text` 与 `gap add` / `gap idea promote` **建条顺带落注**（缺口 `GAP-20260927-259`）
// 都读它 ⇒ 形态不分家（✗ 不造第二套）。
func gapNoteLine(by, text string) string {
	return fmt.Sprintf("%s · %s%s%s", gapNow(), by, gapNoteSep, text)
}

// gapSetStateFields / gapNoteFields —— `--json` 可取字段（与命令树里的 `fields` 同一份口径）。
// ★ 批4 第四片（2026-09-28）：销案两态对拍 ⇒ `--json` 多两格（`two_state` 齐/缺/不适用 · `exempt` 豁免与否）。
var gapSetStateFields = []string{"id", "fp", "state", "solved_kind", "remainder", "changed", "two_state", "exempt"}
var gapNoteFields = []string{"id", "fp", "notes", "changed"}

// gapFrontJSONFields —— `--json <字段>` 的**字段面前置判定**（缺口账 `GAP-20261005-26` · 本枚）。
//
// 口径**一字不新立**：与出口 `selectJSON → emitSelected → marshalObject` 的坏字段判**同 kind 同
// detail 同退码**（`json_field_unknown:<字段>` + `reportBadField` 那句「未知字段 %q」+ 合法字段表），
// 只把位置挪到**第一个真写动作之前**（与同族 `family_dev_edit.go:643` / `family_code.go:314` 同一条口子）。
//
// 修前（本枚现读）：本族这几条写面把字段名判定**只留在出口** `selectJSON` 上 ⇒ 点到非产出格时，写盘
// 已经发生（审计先落盘 → 整件重写 → 写后读回对拍），才在末段退 2 ⇒ **同一发「既写了真源、又退 2 报
// usage」**，调用方按 rc 判会误以为「没写」（安全 / 数据类 · 与 `GAP-20260928-09` 同病）。
//
// 合法形态（字段都在产出集 `produced` 里）⇒ 与改前**逐字节同**（不进这一块、不改任何合法路径）。
func gapFrontJSONFields(inv *invocation, stderr io.Writer, produced []string) int {
	if !inv.jsonGiven {
		return exitOK
	}
	ok := make(map[string]bool, len(produced))
	for _, k := range produced {
		ok[k] = true
	}
	for _, f := range inv.fields {
		if !ok[f] {
			inv.setErr("usage", "json_field_unknown:"+f, fmt.Sprintf("未知字段 %q", f))
			return reportBadField(stderr, inv.path, f)
		}
	}
	return exitOK
}

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
// 返回 `(detail, rc, msg)`：`rc==0` 即真写成功（`detail` 为空 · `msg` = **账写回执行**，由调用方在**人面**打到 stdout；见 `gapLedgerReceipt`）。
// **未动的行逐字节照原样写回**（`led.Lines` 是原文，不是重序列化）—— 与 `verify` 同一条口径。
func gapRewriteOne(inv *invocation, led gapLedger, idx int, rec gapRecord, cmdName, before, after string) (string, int, string) {
	// ⓪ **账写排他锁 + 锁内现读重放**（缺口 `GAP-20261005-66` · 本笔）：
	//    把「读 → 闸 → 审计 → 写 → 读回」**整段**串行化 —— 内核 `flock(LOCK_EX)` 独占，
	//    锁件 `<真源>.lock` 落**状态目录**（✗ 不落仓内 · 与真源同生共死）。
	//
	//   病（本笔修 · 实测）：改态 = 「读整件 → 改一行 → 乐观闸现读比对 → 审计先落盘（一次 fsync）
	//   → 整件重写」。闸与重写**非原子**（中间恰夹着那次 fsync）⇒ 两个写者都能过闸、后写整件覆盖
	//   先写 ⇒ **静默丢更新**（仓外夹具 8 路 × 12 轮：96 发 / 81 `rc=0` / 实落 12 / 净丢 69）。
	//
	//   治：锁一取到就**现读**盘上真源为新基线（「读」被包进锁），本发的整条记录按 **id 重放**到
	//   新基线上（未动的行逐字节取新基线原文）—— ✗ 不是拿旧基线整件覆盖 ⇒ 别的写者已落的更新
	//   不被覆盖（零丢更新）；同一 id 在两次读之间被别人改过 ⇒ 仍按**原口径拒写**（退 14 · 并发冲突）。
	//   ★ 取不到锁 / 现读不出 ⇒ 按原口径退 2 / 8 且**不给结论**（✗ 不静默继续）。
	// ⓪ **写面落点**（缺口 `GAP-20260929-41` 残留 · 本笔）：写面只认 `gapLedgerWritePath`，
	//    ✗ 不拿**读面**落点（`led.Path`）去取锁。只读借用档在册 ⇒ 写面**没有落点** ⇒ 这里在
	//    **取锁之前**硬拒（rc=2）：借源旁边不再留 0 字节 `<借源>.lock`（账没变、盘也不许变 ——
	//    「借用 = 只读」要在盘面上成立，不是只在账内容上成立）。
	wpath, werr := gapLedgerWritePath()
	if werr != nil {
		return "borrow_read_only", exitUsage, werr.Error()
	}
	lk, lerr := gapLedgerLock(wpath)
	if lerr != nil {
		return "ledger_locked", exitUsage,
			fmt.Sprintf("账写排他锁取不到（%v） ⇒ 拿不到独占写权，本次拒写", lerr)
	}
	defer gapLedgerUnlock(lk)

	// ① 锁内**现读**（拿到锁这一刻盘上的真源 = 新基线）。
	fresh, ferr := readGapLedger()
	if ferr != nil {
		return "ledger_unreadable", exitBlocked, ferr.Error()
	}
	// ② 锁内**闸**（原乐观闸的本意：写前对一次身份 —— 现在它排在整个锁区里，「对身份」与「重写」
	//    之间再无别人可插）+ 按 id **重放**本发到新基线上。
	tidx := gapFindIdx(fresh, rec.ID)
	if tidx < 0 {
		return "concurrent_write", exitConflict,
			fmt.Sprintf("锁内现读：目标 id %s 不在账内（`%s`） ⇒ 本次拒写", rec.ID, cmdName)
	}
	if idx >= 0 && idx < len(led.Recs) && !gapRecJSONEqual(led.Recs[idx], fresh.Recs[tidx]) {
		return "concurrent_write", exitConflict,
			fmt.Sprintf("同一 id %s 在「本进程读入」与「锁内现读」之间被别人改过 ⇒ 本次拒写（重跑一次即好）", rec.ID)
	}
	enc, err := json.Marshal(rec)
	if err != nil {
		return "ledger_encode_failed", exitFail, err.Error()
	}
	lines := make([]string, 0, len(fresh.Lines))
	for i, raw := range fresh.Lines {
		if i == tidx {
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
		GapLedgerPath: fresh.Path, GapBeforeSHA: sha256Of(fresh.Raw), GapAfterSHA: sha256Of(body),
		GapBeforeLines: len(fresh.Lines), GapAfterLines: len(lines),
		By: gapByOf(inv), Confirm: "--yes",
	}
	// ★ 账写回执（设计稿 `设计-流程规则程序化-v1.0-20260928.md` §2.2 `A3` · §4 第 3 件）：真写真写
	//   这一刻，把「改前账件 sha256」与「改后账件 sha256」两格**真算**出来 —— 取自本笔审计行的同一
	//   对值（`gap_ledger_before_sha256` / `gap_ledger_after_sha256` · **不另算、不自填**），生成一行
	//   **人可读回执行**（两格 `sha16`）。成功那一态经 `msg` 回到调用方打到 stdout。
	receipt := gapLedgerReceipt(audit.GapBeforeSHA, audit.GapAfterSHA)
	if err := appendGapAudit(editAuditPath(), audit); err != nil {
		return "audit_unwritable", exitBlocked, err.Error()
	}
	if err := gapWriteLedger(wpath, lines); err != nil {
		return "ledger_unwritable", exitBlocked, err.Error()
	}
	back, err := os.ReadFile(wpath)
	if err != nil || sha256Of(back) != audit.GapAfterSHA {
		return "ledger_readback_mismatch", exitBlocked, "写回读对不上"
	}
	return "", exitOK, receipt
}

// ── 账写排他锁（缺口 `GAP-20261005-66`）────────────────────────────────────────────
//
// 锁原语照同仓先例 `core/internal/agent/tool_uses.go` 的 flock 用法（`O_CREATE|O_RDWR` 开件 +
// `syscall.Flock(LOCK_EX)` 阻塞独占）—— **不新造第五套**。锁件 = `<真源>.lock`（落**状态目录** ·
// 与真源同目录/同生共死），**不是**锁真源本身：真源走「临时件 + rename」原子替换 ⇒ rename 之后
// 是**另一个 inode**，锁真源会让并发的两发各锁各的 inode、锁形同虚设。锁件常驻不删（跟先例一致）。

// gapLedgerLockPath —— 账写锁件落点（与真源**同目录** · 状态目录下 · ✗ 不落仓内）。
func gapLedgerLockPath(ledgerPath string) string { return ledgerPath + ".lock" }

// gapLedgerLock —— 取「账写排他锁」（内核 `flock(LOCK_EX)` · **阻塞**等到独占）。取到 ⇒ 返持锁的
// `*os.File`（调用方 defer `gapLedgerUnlock`）；取不到（开件失败 / flock 出错）⇒ 返 err（fail-closed）。
func gapLedgerLock(ledgerPath string) (*os.File, error) {
	lockPath := gapLedgerLockPath(ledgerPath)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// gapLedgerUnlock —— 释放账写排他锁（解锁 + 关件 —— 关件时内核兜底再释放一次）。
func gapLedgerUnlock(f *os.File) {
	if f == nil {
		return
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

// ── 账写**通用**收口（本笔 · 缺口 `GAP-20261010-12`）────────────────────────────────────
//
// gapApplyFail —— `gapLedgerApply` 的 `build` 回调用来**提前拒写**并指名 `(detail, rc, msg)` 的小载体
// （与 `gapRewriteOne` 的返回形态同源 —— 保住各写面原有的「kind / detail / 退码」语义，✗ 不塌陷成单一码）。
type gapApplyFail struct {
	Detail string
	RC     int
	Msg    string
}

// gapLedgerApply —— 真源写面**通用唯一收口**：把「取账写排他锁 → 锁内现读 → 在新基线上构建整件新行 →
// 临时件原子替换 → 写后读回对拍」**整段**串行化。
//
// 病（本笔靶 · 缺口 `GAP-20261010-12`）：本族仍有写路（`add` / `verify` / `backfill-unit`）**不走**
// `gapRewriteOne`，只靠乐观闸 `gapConcurGate`（写前对一次身份 = check-then-act）—— 闸与「审计先落盘
// （fsync）」+「整件重写」之间**无锁**，两个写者可先后过闸、后写覆盖先写 ⇒ **静默丢更新**。
//
// 治（照本族已成体系的先例 `gapRewriteOne` / `gapPoolApply` —— **同一套协议** · ✗ 不新造）：本函数
// **复用**现成助手 —— `gapLedgerLock`（内核 flock 排他 · 阻塞取锁）· `readGapLedger`（锁内现读）·
// `gapWriteLedger`（临时件 + `os.Rename` 原子替换）—— 它只做**编排**，✗ 不复制任何锁 / 写实现。
//
// `build` 收**锁内现读**的真源；返回整件新行（未动的行逐字节取新基线原文）。返回 `(detail, rc, msg)`
// 与 `gapRewriteOne` 同形：`rc==0` ⇒ 真写成功（`detail` 空）。取锁失败 ⇒ 退 2（`ledger_locked`，与
// `gapRewriteOne` 同码）；锁内读不到 / 写不进 / 回读对不上 ⇒ 退 8（不给结论）。
func gapLedgerApply(build func(fresh gapLedger) ([]string, *gapApplyFail)) (string, int, string) {
	// ★ 缺口 `GAP-20260929-41` 残留（本笔）：写面落点只认 `gapLedgerWritePath`（✗ 不用读面的
	//   `gapLedgerPath` —— 借用档下那是**借来的那一件**，在其旁边取锁 = 0 字节 `.lock` 写别人的盘）。
	//   借用档在册 ⇒ 无落点 ⇒ **取锁之前**就拒（rc=2）。
	wpath, werr := gapLedgerWritePath()
	if werr != nil {
		return "borrow_read_only", exitUsage, werr.Error()
	}
	lk, lerr := gapLedgerLock(wpath)
	if lerr != nil {
		return "ledger_locked", exitUsage,
			fmt.Sprintf("账写排他锁取不到（%v） ⇒ 拿不到独占写权，本次拒写", lerr)
	}
	defer gapLedgerUnlock(lk)

	fresh, ferr := readGapLedger()
	if ferr != nil {
		return "ledger_unreadable", exitBlocked, ferr.Error()
	}
	lines, fail := build(fresh)
	if fail != nil {
		return fail.Detail, fail.RC, fail.Msg
	}
	if werr := gapWriteLedger(wpath, lines); werr != nil {
		return "ledger_unwritable", exitBlocked, werr.Error()
	}
	want := ""
	if len(lines) > 0 {
		want = strings.Join(lines, "\n") + "\n"
	}
	back, rerr := os.ReadFile(wpath)
	if rerr != nil || string(back) != want {
		return "ledger_readback_mismatch", exitBlocked, "写回读对不上"
	}
	return "", exitOK, ""
}

// gapRecJSONEqual —— 两条账记录按**规范 JSON** 逐字节相等？（判「同一 id 在两次读之间被别人改过」）。
// `gapRecord` 带切片字段（`Notes`/`Voids`）⇒ 不能用 `==`，故比序列化字节。
func gapRecJSONEqual(a, b gapRecord) bool {
	ab, ea := json.Marshal(a)
	bb, eb := json.Marshal(b)
	return ea == nil && eb == nil && string(ab) == string(bb)
}

// ── 账写回执（设计稿 `设计-流程规则程序化-v1.0-20260928.md` §2.2 `A3` · §4 第 3 件）──────────
//
// 病（A3 逐字）：真账一个字节都不许被只读面碰 —— 但「这一发到底碰没碰真账」此前全靠人记（或事后比）。
// 治：唯一写路每写真写一次，就把**改前账件 sha256** 与**改后账件 sha256** 两格记进那一笔审计行
// （`gap_ledger_before_sha256` / `gap_ledger_after_sha256` —— 本族审计行自始即有此两格 · **本笔不新增
// 审计键**），并在**成功的回执行**里逐字印出两格的 `sha16`（`改前 sha16=… → 改后 sha16=…`）。
//
// ★ 两格一律**真算**：取自本笔审计行的同一对值（`sha256Of(led.Raw)` 与 `sha256Of(body)`），
//   不另算、不自填、不拿别的数强凑。
// ★ 改前 == 改后（真什么都没改）⇒ 如实印「未变」，**不编假差**。

// gapLedgerReceipt —— 「账写回执」那一行（逐字含两格 `sha16` · 本面唯一落点）。
func gapLedgerReceipt(beforeSHA, afterSHA string) string {
	b16, a16 := sha16Of(beforeSHA), sha16Of(afterSHA)
	if beforeSHA == afterSHA {
		return fmt.Sprintf("  账写回执 : 改前 sha16=%s → 改后 sha16=%s（未变）", b16, a16)
	}
	return fmt.Sprintf("  账写回执 : 改前 sha16=%s → 改后 sha16=%s", b16, a16)
}

// sha16Of —— 取 sha256 十六进制串的**前 16 位**（不足 16 ⇒ 原样 —— 现算现取，不补齐、不估）。
func sha16Of(sha string) string {
	if len(sha) <= 16 {
		return sha
	}
	return sha[:16]
}

// gapReceiptPrint —— 写面成功时把那一行「账写回执」打到 stdout（空串 ⇒ 一个字不打 · 幂等面从不经此）。
func gapReceiptPrint(w io.Writer, receipt string) {
	if strings.TrimSpace(receipt) != "" {
		fmt.Fprintf(w, "%s\n", receipt)
	}
}

// gapConcurGate —— 改态的**乐观并发闸**（批4 第七片 · 设计稿 §三十五 `O-14`：「多会话读-改-写会丢更新」）。
//
// 病：本族改态是「读整件 → 改一行 → 整件重写（`gapWriteLedger` · 临时件 + rename）」。两个写者各自读旧账、
// 各自整件重写 ⇒ **后写的那次覆盖先写的那次**（丢更新；实测到同一秒内出现三个不同 sha）。
//
// 闸法（乐观并发 —— 只在写入前一刻对一次身份：不加锁、不排序、不改真源格式）：
//
//	· 基线 = 本进程**读入那一刻**的整件字节（`led.Raw`）的 `sha256`；
//	· 写前**现读**盘上那一件，现算 `sha256`；
//	· 两者不符 ⇒ 退 `concurrent_write`（调用方 `gapWriteFail` 译成**退码 14**（`kind=conflict`） · 逐字点名两个 `sha16`）；
//	· 写前**现读不到** ⇒ 分两档（甲案 · 2026-10-03 拍定 · 缺口 `GAP-20260927-191` 修）：
//	  **「件不在」**（`os.IsNotExist`）⇒ 视为**空账**：空账本来就该能写第一条（首次 `add` / 首次改态）
//	  ⇒ **放行**；**「件在、但读不出来」**（损坏 / 无权限 · 非 `IsNotExist`）⇒ 拿不到「现在」这一格
//	  ⇒ **拒写**（退 `concurrent_write` · 退码 14）。★ 这道闸的本意只是「**别覆盖别人的写**」——
//	  盘上没有件，就没有可覆盖的对象；原先「读不到一律拒写」把**空账首写**误伤了。
//	· 相符 ⇒ 放行，此后仍是原路（审计先落盘 → 整件重写 → 读回对拍）。
//
// 为什么捏在**这一处**：本族所有改态面（`set-state` / `note` / `note --retract` / `assign` /
// `bulk set-state` / `verify-one` / `receipt apply`）都只走唯一收口 `gapRewriteOne` ⇒ 闸加在收口上，
// 这些面**自动**被护住（不另开写路 · 也不是每个命令里各写一遍）。
//
// ★ 退码取 14（**2026-10-10 改定 · 缺口 `GAP-20260928-192`**）：语义是「**并发冲突**」—— 本族口径
// 「并发拒写 ⇒ 退 14」（`exitConflict` · `kind=conflict`），与同族 `repo commit` 的 `repo_busy` /
// `guard` 的 `rebuild_blocked` 同码。★ 改前取 2（用法错）⇒ 机器消费方把「并发冲突（**可重试**）」
// 误当「用法错（**不可重试**）」；本次改取 14（改动只收敛「并发拒写」这一路，用法错照旧退 2）。
func gapConcurGate(led gapLedger, cmdName string) (string, int, string) {
	base := sha256Of(led.Raw)
	now, err := os.ReadFile(led.Path)
	if err != nil {
		// ★ 甲案（缺口 `GAP-20260927-191` 修 · 2026-10-03 拍定）：
		//   **账件不存在**（`os.IsNotExist`）⇒ 视为**空账** ⇒ **放行** —— 空账本来就该能写第一条
		//   （首次 `add` / 首次改态那一发：测试夹具是新建空真源，首次写时文件还不存在 ⇒ 旧闸恒拒）。
		//   只有「**存在但读不出来**」（损坏 / 无权限 · 非 `IsNotExist`）才拒写：那时拿不到「现在」这一格。
		if os.IsNotExist(err) {
			return "", exitOK, ""
		}
		return "concurrent_write", exitConflict,
			fmt.Sprintf("写前**现读**不到账件（%v） ⇒ 拿不到「现在」这一格，拒写", err)
	}
	cur := sha256Of(now)
	if cur == base {
		return "", exitOK, ""
	}
	return "concurrent_write", exitConflict,
		fmt.Sprintf("基线 sha16=%s（`%s` 读入那一刻） · 现状 sha16=%s（写前现读）", base[:16], cmdName, cur[:16])
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

// ── 批4 第四片：销案前的**两态对拍**闸（新账强制 · 存量豁免）────────────────────────────────────
//
// 设计出处（唯一真源 · `设计-缺口账与自进化-v2.0-20260928.md`）：
//
//	· §11.5 两态对拍（销案口径）逐字：「销案时自动跑 ① 改前态（仓外同源 …）② 改后态（当前制品）
//	  ③ 两态输出差异 ⇒ 自动写进 `solved_evidence`（散文只能**补充**，不能替代）。生效面：
//	  `solved_at` 晚于生效时刻的**新账强制**，存量豁免」；
//	· 表 `D6`（两态对拍）· `M-33`（「判据过」不等于「真修好」⇒ 防「判据绿但真行为未变」）。
//
// 本片落的是**入口闸**，不是自动跑（两态怎么跑另立一条；本片不改写路、不引新依赖）：
//
//	① 「新账」的判据 = **缺口号里的日期 ≥ `2026-09-28`**（号形 `GAP-YYYYMMDD-NN`）。
//	   号内取不出日期 ⇒ 按**存量**办（判据是「号内日期」，取不出就没有证据说它是新账 —— 取定 · 回执点名）。
//	② 新账 + 改到 `已解` + 两态读数**不齐** ⇒ **退 2**，并**逐段点名**缺哪一样（改前读数 / 改后读数）。
//	   真源一个字节不写、审计一行不落（闸排在「审计先落盘」之前 —— 与并发闸同一条位置纪律）。
//	③ 存量账 + 未带两态 ⇒ **豁免**，但**不许静默豁免**：出力表尾逐字印「本发为存量豁免（未带两态对拍）」。
//	④ 改到**非** `已解` 的状态 ⇒ 本闸**不适用**：一个字都不多打（与改前逐字同 —— 本片对拍项）。
//
// ★ 两态读数的传递面（本片取定 · 回执点名）：两枚取值旗标 `--before-read` / `--after-read`，
// 每枚的值 = 同族派单登记那种 `k=v` 逐格串（分隔 ` · ` · 见 `gapAssignLine` / `gapAssignParse` 先例）：
//
//	--before-read "cmd=<命令> · rc=<真退码> · reading=<关键读数一行>"
//	--after-read  "cmd=<命令> · rc=<真退码> · reading=<关键读数一行>"
//
// ★ 批4 第五片（施工清单 `4-5`「销案证据形态钉死：命令 + rc + 关键读数 · 证据缺 rc 或读数 ⇒ 拒收」）
// —— **不再只查空串**，四格逐格**真判**（缺一即拒 · 一律点名到格）：
//
//	① `cmd` 须能承载真修的两态读数：首词必须是 `zerg`（本族正门）；**或**「测试件面」命令 —— 真修
//	   只由测试件（`go test …` / `python3 <件>.py …`）承载、CLI 行为一字不变时，`cmd=` 允许是测试命令，
//	   但 `reading=` **必须**注明这是等价读数（须含 `gapTwoStateTestFaceMark`）；**或**「仓外夹具命令」
//	   —— 真实差异只有仓外夹具才暴露（真仓无该形态 ⇒ 正门命令两态读数必然相同）时，`cmd=` 允许是仓外
//	   夹具脚本/命令，但 `reading=` **必须**注明这是夹具读数（须含 `gapTwoStateFixtureMark`）
//	   ⇒ ✗ 不许拿测试/夹具命令冒充正门读数；
//	② `rc` 必须是**十进制整数**（空 / 带壳引号 / 非数字 ⇒ 拒收）；
//	③ `reading` 非空、**不得含换行**（读数是一行）；
//	④ 三格之外**多一格键** ⇒ 拒收（点名多出来那一格）。
//
// `reading` 之后**整段**都算读数行（关键读数行自己可能含 ` · ` —— 例如同族 `gapVerifyOneReading`
// 的形态 `rc=0 · 首行=…`）：尾段只把**裸 ASCII 键**里不在读数白名单（`rc` / `reading`）的那些当「多出来的一格」。
//
// ★ 销案**落账**（同一片）：两态**齐** ⇒ 两态读数按写死的字段名落进该条账行的 `solved_evidence`
// （设计稿 §11.5 逐字「② 改后态 ③ 两态输出差异 ⇒ 自动写进 `solved_evidence`」· `O-13`「钉成可解析形态」
// · `D4`「判据 + rc + 读数」三件由程序写、散文只能**补充**）—— 落点取该行**已有**的那一格 ⇒ `gap show`
// 现读即见、不新增真源键（故本片**只动本件**，一个字都不越到别的件）。

// gapTwoStateDateMin —— 「新账」的分界（号内日期 ≥ 它 ⇒ 新账 · 设计稿 §11.5 生效面）。
const gapTwoStateDateMin = "20260928"

// gapTwoStateSep —— 两态读数里逐格的分隔（与同族派单登记同一种形态：`k=v` 逐格 · 分隔 ` · `）。
const gapTwoStateSep = " · "

// gapTwoStateExemptText —— 存量豁免的**表尾**那一行（逐字 · 不许静默豁免）。
const gapTwoStateExemptText = "本发为存量豁免（未带两态对拍）"

// gapIDDate —— 从缺口号里取日期（`GAP-YYYYMMDD-NN` ⇒ `YYYYMMDD`）；取不出 ⇒ 空串。
func gapIDDate(id string) string {
	for _, p := range strings.Split(id, "-") {
		if len(p) != 8 {
			continue
		}
		allDigit := true
		for _, r := range p {
			if r < '0' || r > '9' {
				allDigit = false
				break
			}
		}
		if allDigit {
			return p
		}
	}
	return ""
}

// gapTwoStateNewLedger —— 这条是不是「新账」（号内日期 ≥ 阈值）。取不出日期 ⇒ false（按存量办）。
func gapTwoStateNewLedger(id string) bool {
	d := gapIDDate(id)
	return d != "" && d >= gapTwoStateDateMin
}

// gapTwoStateKeysTxt —— 三格形态的逐字吐法（点名里共用一处，免得四处手写漂掉）。
const gapTwoStateKeysTxt = "cmd=<命令> · rc=<真退码> · reading=<关键读数一行>"

// gapTwoStateTestFaceMark —— 「测试件面」读数在 `reading=` 里**必须**自带的等价读数标注（销案两态放宽 · 拍板口径）。
//
// 病（修前）：`cmd=` 首词必须是裸 `zerg` ⇒ 凡真修**只由测试件（Go 测试件 / py 脚本）承载、CLI 行为一字
// 不变**的账条，一律产不出合法两态 ⇒ **销案永久卡死**。治：`cmd=` 允许是「测试件面」命令，但读数行
// 必须注明它是**等价读数**（`reading=` 须含本串）⇒ 拿测试命令冒充正门读数的照样拒。
const gapTwoStateTestFaceMark = "等价读数"

// gapTwoStateTestFaceIs —— 该 `cmd=` 是不是**测试件面**命令（真修只由测试件承载 · CLI 行为零变化）。
//
// 闭集（逐字钉死 · ✗ 不比子串、✗ 不收 shell 展开 ⇒ 与既有「首词裸 `zerg`」同一种逐 token 口径）：
//
//	· `go test …`                              （Go 测试件：`go test -run <名> ./cmd/zerg/`）
//	· `python3 <件>.py …` / `python <件>.py …`  （py 脚本件）
//
// 其余一律不是 ⇒ 仍按「首词必须是 `zerg`」拒收（✗ 不放宽到任意非 `zerg` 命令）。
func gapTwoStateTestFaceIs(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "go":
		return len(fields) >= 2 && fields[1] == "test"
	case "python3", "python":
		for _, f := range fields[1:] {
			if strings.HasSuffix(f, ".py") {
				return true
			}
		}
	}
	return false
}

// gapTwoStateFixtureMark —— 「仓外夹具命令」读数在 `reading=` 里**必须**自带的夹具读数标注（销案两态放宽 · 拍板口径）。
//
// 病（修前）：`cmd=` 只认本族正门（裸 `zerg`）与「测试件面」（`go test …` / `python3 <件>.py …`）⇒ 凡真修
// 的**真实差异只有仓外夹具才暴露**（排序面 / 路径面一类；真仓无该形态 ⇒ 正门命令两态读数必然相同）的
// 账条，一律产不出合法两态 ⇒ 销案卡死，助手被迫把夹具真读数挤进 `--evidence` 散文。治：`cmd=` 允许是
// 「仓外夹具命令」（仓外临时目录里的夹具脚本/命令），但读数行必须注明这是**夹具读数**（须含本串）
// ⇒ 拿夹具命令冒充正门读数的照样拒。
const gapTwoStateFixtureMark = "夹具"

// gapTwoStateFixtureIs —— 该 `cmd=` 是不是**仓外夹具命令**（真实差异证据只由仓外夹具暴露）。
//
// 闭集（逐字钉死 · ✗ 不比子串、✗ 不收 shell 展开 ⇒ 与「测试件面」同一逐 token 口径）：
//
//	· `<路径形夹具脚本>` …            （首 token 是路径形 ⇒ 直接跑盘上那枚夹具脚本件）
//	· `bash|sh|zsh <路径形夹具脚本>` …（经解释器跑仓外夹具脚本）
//
// 「路径形夹具脚本」= token 以 `/`（绝对）· `./` · `../`（相对）· `~/`（家目录）起头，**且 basename 带
// 脚本后缀**（`.sh` / `.bash` / `.zsh`）—— 指向盘上一枚夹具脚本件（脚本后缀先认 `.py`/`go test` 归
// 「测试件面」，故本档只收 shell 脚本 ⇒ ✗ 不与测试件面重复）。
// 裸命令名（`ls` / `go` / `go build`）一律不算 ⇒ 仍按「首词必须是 `zerg`」拒收（✗ 不放宽到任意非 `zerg` 命令）。
func gapTwoStateFixtureIs(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	if gapTwoStateFixtureScript(fields[0]) {
		return true
	}
	switch fields[0] {
	case "bash", "sh", "zsh":
		return len(fields) >= 2 && gapTwoStateFixtureScript(fields[1])
	}
	return false
}

// gapTwoStateFixtureScript —— 该 token 是不是**路径形夹具脚本**（路径形 + 脚本后缀）。
func gapTwoStateFixtureScript(tok string) bool {
	if !(strings.HasPrefix(tok, "/") || strings.HasPrefix(tok, "./") ||
		strings.HasPrefix(tok, "../") || strings.HasPrefix(tok, "~/")) {
		return false
	}
	switch {
	case strings.HasSuffix(tok, ".sh"), strings.HasSuffix(tok, ".bash"), strings.HasSuffix(tok, ".zsh"):
		return true
	}
	return false
}

// gapTwoStateParse —— 一条两态读数**逐格读回 + 逐格真判**（批4 第五片 · 施工清单 `4-5`）。
//
// 四格判据（缺一即拒 · 一律**点名**到格）：
//
//	① `cmd`：首词必须是 `zerg`；**或**「测试件面」命令（`go test …` / `python3 <件>.py …`）且
//	   `reading=` 含等价读数标注（`gapTwoStateTestFaceMark`）；**或**「仓外夹具命令」（仓外夹具脚本/命令）
//	   且 `reading=` 含夹具读数标注（`gapTwoStateFixtureMark`）；
//	② `rc` ：必须是**十进制整数**（空 / 带壳引号 / 非数字 ⇒ 拒收）；
//	③ `reading`：非空、**不得含换行**；
//	④ 三格之外**多一格键** ⇒ 拒收（点名多出来那一格）。
//
// 形态 = `cmd=<命令> · rc=<真退码> · reading=<关键读数一行>`（分隔 ` · ` · 与 `gapAssignLine` 同族）。
// `reading` 之后**整段**都算读数行（它自己可能含 ` · ` —— 同族 `gapVerifyOneReading` 的形态 `rc=0 · 首行=…`）；
// 故尾段只把**裸 ASCII 键**里不在读数白名单（`rc` / `reading`）的那些当「多出来的一格」。
//
// 返回 (cmd, rc, reading, why, ok)：`ok=false` ⇒ `why` = 那一段的点名（哪一格 · 怎么不对）。
func gapTwoStateParse(s string) (cmd, rc, reading, why string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", "", "这一段是空的（没给取值）", false
	}
	ri, tail := -1, ""
	if strings.HasPrefix(s, "reading=") {
		ri, tail = 0, s[len("reading="):]
	} else if i := strings.Index(s, gapTwoStateSep+"reading="); i >= 0 {
		ri = i + len(gapTwoStateSep)
		tail = s[ri+len("reading="):]
	}
	if ri < 0 {
		return "", "", "", "缺 `reading=` 那一格（只认三格：" + gapTwoStateKeysTxt + "）", false
	}
	head := strings.TrimSuffix(s[:ri], gapTwoStateSep)
	segs := []string{}
	if head != "" {
		segs = strings.Split(head, gapTwoStateSep)
	}
	if len(segs) < 2 {
		return "", "", "", "前两格不齐（要 `cmd=` · `rc=`；只认三格：" + gapTwoStateKeysTxt + "）", false
	}
	if len(segs) > 2 {
		for _, extra := range segs[2:] {
			if k := gapTwoStateBareKey(extra); k != "" {
				return "", "", "", "多了一格：`" + k + "=`（只认三格：" + gapTwoStateKeysTxt + "）", false
			}
		}
		return "", "", "", "前两格之后还有 `" + segs[2] + "`（只认三格：" + gapTwoStateKeysTxt + "）", false
	}
	if !strings.HasPrefix(segs[0], "cmd=") {
		return "", "", "", "第一格不是 `cmd=`（得到 `" + segs[0] + "`）", false
	}
	if !strings.HasPrefix(segs[1], "rc=") {
		if k := gapTwoStateBareKey(segs[1]); k != "" && k != "rc" {
			return "", "", "", "多了一格：`" + k + "=`（只认三格：" + gapTwoStateKeysTxt + "）", false
		}
		return "", "", "", "第二格不是 `rc=`（得到 `" + segs[1] + "`）", false
	}
	cmd = strings.TrimSpace(segs[0][len("cmd="):])
	rc = strings.TrimSpace(segs[1][len("rc="):])
	reading = strings.TrimSpace(tail)
	// ④ 尾段里的「多出来的一格」（读数行自己含 ` · ` 是允许的：白名单键 `rc` / `reading` 不算多）。
	if parts := strings.Split(reading, gapTwoStateSep); len(parts) > 1 {
		for _, part := range parts[1:] {
			if k := gapTwoStateBareKey(part); k != "" && k != "rc" && k != "reading" {
				return "", "", "", "多了一格：`" + k + "=`（只认三格：" + gapTwoStateKeysTxt + "）", false
			}
		}
	}
	// ③ 读数行：非空、不含换行
	if reading == "" {
		return "", "", "", "`reading=` 是空的（销案证据必须带关键读数一行）", false
	}
	if strings.ContainsAny(reading, "\n\r") {
		return "", "", "", "`reading=` 含换行（读数必须是**一行**）", false
	}
	// ① 命令：首词必须是 `zerg`（本族正门）；**或**「测试件面」命令 —— 真修只由测试件承载、CLI 行为
	//    一字不变时（销案两态放宽 · 拍板），`cmd=` 允许是测试命令（`go test …` / `python3 <件>.py …`），
	//    但 `reading=` **必须**注明这是等价读数（须含 `gapTwoStateTestFaceMark`）；**或**「仓外夹具命令」
	//    —— 真实差异只有仓外夹具才暴露时，`cmd=` 允许是仓外夹具脚本/命令，但 `reading=` **必须**注明
	//    这是夹具读数（须含 `gapTwoStateFixtureMark`）⇒ ✗ 不许拿测试/夹具命令冒充正门读数、
	//    ✗ 也不放宽到任意非 `zerg` 命令。
	if fields := strings.Fields(cmd); len(fields) == 0 {
		return "", "", "", "`cmd=` 是空的（销案证据必须带命令）", false
	} else if fields[0] != "zerg" {
		switch {
		case gapTwoStateTestFaceIs(fields):
			if !strings.Contains(reading, gapTwoStateTestFaceMark) {
				return "", "", "", "`cmd=` 是「测试件面」命令（`" + cmd + "`），但 `reading=` 没注明这是等价读数 —— 须含 `" + gapTwoStateTestFaceMark + "`（如 `reading=" + gapTwoStateTestFaceMark + " 改前 FAIL 改后 PASS`）", false
			}
		case gapTwoStateFixtureIs(fields):
			if !strings.Contains(reading, gapTwoStateFixtureMark) {
				return "", "", "", "`cmd=` 是「仓外夹具命令」（`" + cmd + "`），但 `reading=` 没注明这是夹具读数 —— 须含 `" + gapTwoStateFixtureMark + "`（如 `reading=" + gapTwoStateFixtureMark + "读数 改前全红 改后全绿`）", false
			}
		default:
			return "", "", "", "`cmd=` 首词不是 `zerg`，也不是「测试件面」/「仓外夹具」命令（得到 `" + cmd + "` · 销案证据只认本族正门命令，或承载真修的测试件命令 `go test …` / `python3 <件>.py …`，或仓外夹具脚本/命令）", false
		}
	}
	// ② 退码：十进制整数
	if !gapTwoStateDecimal(rc) {
		return "", "", "", "`rc=` 不是十进制整数（得到 `" + rc + "`）", false
	}
	return cmd, rc, reading, "", true
}

// gapTwoStateBareKey —— 一段 `k=v` 里的**裸 ASCII 键**（不是这种形态 ⇒ 空串）。
func gapTwoStateBareKey(seg string) string {
	t := strings.TrimSpace(seg)
	i := strings.Index(t, "=")
	if i <= 0 {
		return ""
	}
	k := t[:i]
	for j, r := range k {
		okc := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (j > 0 && r >= '0' && r <= '9')
		if !okc {
			return ""
		}
	}
	return k
}

// gapTwoStateDecimal —— 十进制整数（可带前导 `-` · 至少一位数字）＝**裸**退码（带壳引号自然落空）。
func gapTwoStateDecimal(v string) bool {
	if v == "" {
		return false
	}
	if v[0] == '-' {
		v = v[1:]
	}
	if v == "" {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// gapTwoStateAutoReading —— 一条「自动跑」命令串 ⇒ 三格读数串
// （`cmd=<原样命令串> · rc=<真退码> · reading=<stdout 首行或摘要>`）。
//
// 形态**照拄** `gateFourStatesRun`（切词 · 无 shell 展开 · 60s 上限 · 收 stdout+stderr）
// —— ✗ 不另造 runner：跑不起来（命令不存在 / 不可执行 ⇒ `Empty`）或 `rc∈{126,127}`
// ⇒ `ok=false`，调用方按本族既有档**判不了**（✗ 不许当 0 用）。
//
// `reading` 取 stdout **首个非空行**（无输出 ⇒ 摘要 `(无 stdout 输出 · rc=N)`）；行内若含分段符
// ` · ` ⇒ 换成 ` / `（该符与三格分隔同形，留着会被唯一判处读成「多了一格」）。
func gapTwoStateAutoReading(cmdStr string) (string, bool) {
	cell := gateFourStatesRun(cmdStr, nil)
	if cell.Empty || cell.RC == 126 || cell.RC == 127 {
		return "", false
	}
	return gapJudgeCellReading(cmdStr, cell), true
}

// gapTwoStateFirstLine —— 一段输出的**首个非空行**（去首尾空白 · 去行尾 `\r`）。
func gapTwoStateFirstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(strings.TrimSuffix(ln, "\r")); t != "" {
			return t
		}
	}
	return ""
}

// gapTwoStateResolution —— 两态读数 resolver 的一次判决（**判与落同源**的唯一取值面）：
//
//	Before / After = 两侧的**最终**三格读数串（自动跑出的 · 或手传的）；
//	Note           = 「同给 cmd 与 read」那一档的**明报**（空 ⇒ 无）；
//	RC             = `exitOK`；或**判不了**要退的码（跑不起来 ⇒ `exitBlocked`=8）。
type gapTwoStateResolution struct {
	Before, After string
	Note          string
	RC            int
}

// gapTwoStateResolve —— 两态读数的**唯一 resolver**：裁决（`gapTwoStateJudge`）与落账
// （`gapTwoStateLanded`）都只认它拼出的三格（✗ 不许两处各拼一套 · 缺口 `GAP-20260928-196` 甲）。
//
// 每侧取值优先级（取定 · **明报**）：给了 `--<侧>-cmd` ⇒ **现读语义优先** —— CLI 自己跑那一条命令、
// 把 `cmd=/rc=/reading=` 三格按**真退码**填死（✗ 不伪造 rc）；同给 `--<侧>-read` ⇒ 手传那一枚
// **不取**、并把「现读优先」写进 `Note`（✗ 不静默取一）。两侧都没给 cmd ⇒ 照旧用手传 read（口径不动）。
// 任一侧**跑不起来** ⇒ `RC=exitBlocked`（判不了 · 该族既有档）。
//
// 结果**缓存**在 `inv.twoStateCache` 上：同一发里裁决与落账（被调多次）**只跑一趟**、且逐字同源。
func gapTwoStateResolve(inv *invocation) *gapTwoStateResolution {
	if inv == nil {
		return &gapTwoStateResolution{RC: exitOK}
	}
	if inv.twoStateCache != nil {
		return inv.twoStateCache
	}
	res := &gapTwoStateResolution{RC: exitOK}
	sides := []struct {
		label string
		cmd   string
		read  string
		dst   *string
	}{
		{"改前", "--before-cmd", "--before-read", &res.Before},
		{"改后", "--after-cmd", "--after-read", &res.After},
	}
	for _, s := range sides {
		cmdStr := strings.TrimSpace(inv.flagVal(s.cmd))
		readStr := strings.TrimSpace(inv.flagVal(s.read))
		if cmdStr == "" {
			*s.dst = readStr
			continue
		}
		got, ok := gapTwoStateAutoReading(cmdStr)
		if !ok {
			res.RC = exitBlocked
			res.Note = fmt.Sprintf("%s：`%s` 的命令**跑不起来**（命令不存在 / 不可执行 · rc=126/127）⇒ %s 现读拿不到三格 ⇒ 按本族既有档**判不了**（退码 %d · ✗ 不当 0 用）", s.label, s.cmd, s.label, exitBlocked)
			inv.twoStateCache = res
			return res
		}
		if readStr != "" {
			if res.Note != "" {
				res.Note += "\n"
			}
			res.Note += fmt.Sprintf("%s：`%s`（手传读数）与 `%s`（现读）**同给** ⇒ 按**现读语义优先**取 `%s` 跑出的三格；手传那一枚**不取**（明报 · ✗ 不静默取一）", s.label, s.read, s.cmd, s.cmd)
		}
		*s.dst = got
	}
	inv.twoStateCache = res
	return res
}

// gapTwoStateMissing —— 两态读数**逐段真判**（批4 第五片：不只查空串 ⇒ 逐格点出**哪一格 · 怎么不对**）。
func gapTwoStateMissing(beforeRaw, afterRaw string) []string {
	miss := []string{}
	if _, _, _, why, ok := gapTwoStateParse(beforeRaw); !ok {
		miss = append(miss, "改前读数（--before-read）："+why)
	}
	if _, _, _, why, ok := gapTwoStateParse(afterRaw); !ok {
		miss = append(miss, "改后读数（--after-read）："+why)
	}
	return miss
}

// gapTwoStateLanded —— 销案落账的**两态对拍那一块**（字段名写死 · 批4 第五片）。
// 落点 = 该条账行的 `solved_evidence`（设计稿 §11.5 逐字：「两态输出差异 ⇒ 自动写进 `solved_evidence`」
// · `D4`：「判据 + rc + 读数」三件由程序写、散文只能**补充**）—— 故 `gap show` 现读即见，不新增真源键。
// 形态（分隔与读数同族 ` · ` · 大括号钉死每段边界 ⇒ 读数行自己含 ` · ` 也不歧义）：
//
//	销案两态对拍 · 改前{cmd=… · rc=… · reading=…} · 改后{cmd=… · rc=… · reading=…} · 散文=<evidence>
func gapTwoStateLanded(evidence, beforeRaw, afterRaw string) string {
	out := "销案两态对拍" + gapTwoStateSep +
		"改前{" + strings.TrimSpace(beforeRaw) + "}" + gapTwoStateSep +
		"改后{" + strings.TrimSpace(afterRaw) + "}"
	if evidence != "" {
		out += gapTwoStateSep + "散文=" + evidence
	}
	return out
}

// gapTwoStateJudge —— 两态读数的三件取值（本片**唯一**判处）：
// `pair` = `齐` / `缺` / `不适用`（非 `已解` 面）· `exempt` = 真表示**存量豁免**（新账缺读数走拒收，不是豁免）
// · `miss` = 逐段缺名（空 ⇒ 齐）。**读面动作**：一个字都不写。
func gapTwoStateJudge(inv *invocation, id, stateWant string) (pair string, exempt bool, miss []string) {
	if stateWant != gapStSolved {
		return "不适用", false, nil
	}
	// ★ GAP-20260928-196（甲）：取值走**唯一 resolver**（`--before-cmd`/`--after-cmd` 自动跑 ⇒ 三格
	//   现读填死；手传 `--before-read`/`--after-read` 照旧）—— 与落账同源（✗ 不各拼一套）。
	res := gapTwoStateResolve(inv)
	if res.RC != exitOK {
		// 自动跑那两条**跑不起来** ⇒ **判不了**（退码 8 · 该族既有档）—— 调用方翻成退码，✗ 不当 0 用。
		return "判不了", false, []string{res.Note}
	}
	beforeRaw, afterRaw := res.Before, res.After
	miss = gapTwoStateMissing(beforeRaw, afterRaw)
	if len(miss) == 0 {
		return "齐", false, nil
	}
	// GAP-20260929-17：**区分「给了值但形态不符」与「真的一个字没给」** ——
	// 任一段非空（= 给了值）却读不回来 ⇒ 属形态错，**不降级成存量豁免**（一律拒收，逐段点名）。
	// 只有两段都空（= 真没给）时，存量账才走豁免分支。
	malformed := beforeRaw != "" || afterRaw != ""
	return "缺", !malformed && !gapTwoStateNewLedger(id), miss
}

// ═══ 「是否已在 HEAD」一档（承接项 ⑨ · 缺口 `GAP-20260927-62`）════════════════════════════════
//
// 病（本档要治的**假阳**）：账目对「这条修了没」此前**只看件脏不脏**（工作树改动面）——
// 共件（多路共改的热件）被**并发他路**改动时，工作树照样是脏的 ⇒ 会被**误报成「已修」**。
//
// 治（★ 两者**并存 · 分层展示** ⇒ ✗ 不取消原有「脏不脏」信号）：在原「销案两态对拍」
// （`two_state`）之上**再加一档**「是否已在 HEAD」—— 用**只读 git** 手段判「本账修」在不在 HEAD：
//
//	已在HEAD        件在 HEAD **且**工作树相对 HEAD 无改动（最强正读：修已进提交面）
//	HEAD有·工作树改  件在 HEAD **但**工作树又改（★ 共件假阳场景 ⇒ **不判「已修」**：多半是别路在途改动）
//	不在HEAD        件不在 HEAD（⇒ **不判「已修」**：修只活在工作树里，提交面还没有它）
//	不适用          无件兜底档（无标识串 ⇒ 无件面落点可判）
//	判不了          仓根取不到 / 无 HEAD / git 读不到（fail-closed · ✗ 不当「已在 HEAD」用）
//
// ★★ **「本账修的唯一标识串」取法（v1 · 本档口径真源）**：
//
//	① **主取 = 该账行的 `unit`**（仓内相对件路径）——「本账修」= 修落在**哪一件**；判 HEAD
//	   只按这一段（git 路径面口径：`ls-tree -r --name-only HEAD -- <unit>` ＋ `status --porcelain -- <unit>`）；
//	② **无标识串** = 兜底档（`unit` 空 / `无件…` / `拟(…)`）⇒ 无件面落点可判 ⇒ 判「不适用」；
//	③ **组合形**（面级展示用）= `<id>@<unit>`（`id` 逐账唯一 ⇒ 一行一修；`unit` 定「修在哪件」）——
//	   ✗ 判 HEAD 时**不**拿组合串去拼 git pathspec（`id` 不是路径）；
//	④ 取不到该账行（账读不到 / 号不在账里）⇒ 判「判不了」（**读不到 ≠ 不在 HEAD** · ✗ 不许当绿）。
//
// 口径边界：本档**只报告**（机器面 `meta.head_tier` ＋ 人面一行）—— ✗ 不动判据（放行 / 拒收仍由
// `gapTwoStateJudge` 那三件定）· ✗ 不进退出码 · ✗ 不取消原有「脏不脏」（`two_state`）信号。
const (
	// gapHeadInHead —— 件在 HEAD **且**工作树相对 HEAD 无改动（最强正读）。
	gapHeadInHead = "已在HEAD"
	// gapHeadHeadDirty —— 件在 HEAD **但**工作树又改（★ 共件假阳场景 ⇒ 不判「已修」）。
	gapHeadHeadDirty = "HEAD有·工作树改"
	// gapHeadNotInHead —— 件不在 HEAD（⇒ 不判「已修」）。
	gapHeadNotInHead = "不在HEAD"
	// gapHeadNotApplic —— 无件兜底档（无标识串 ⇒ 无件面落点可判）。
	gapHeadNotApplic = "不适用"
	// gapHeadUnknown —— 仓根取不到 / 无 HEAD / git 读不到（fail-closed）。
	gapHeadUnknown = "判不了"
)

// gapHeadGitRO —— 跑一条**只读** `git -C <root> …`（rc≠0 / 起不来 ⇒ ok=false）。✗ 不写任何东西。
// `--no-optional-locks` 硬带（与同族 `gapPremiseTrackedBasenames` 同一纪律：一个可选锁都不取）。
func gapHeadGitRO(root string, args ...string) (string, bool) {
	full := append([]string{"-C", root, "-c", "core.quotepath=false", "--no-optional-locks"}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}

// gapHeadTierOf —— 一件（仓内相对路径）的「是否已在 HEAD」判定（本档**唯一判处** · 纯只读 · 零写盘）。
// 入参取**显式 root**（✗ 不吃全局 `repoRoot()`）⇒ 可在合成夹具仓上逐态跑（三例可测）。
func gapHeadTierOf(unit, root string) string {
	u := strings.TrimSpace(unit)
	// ② 无标识串（兜底档 / 空）⇒ 无件面落点可判。
	if u == "" || gapUnitIsNoFileSel(u) {
		return gapHeadNotApplic
	}
	// 仓根取不到 / 无 HEAD（空仓 / 非 git 仓）⇒ 判不了（fail-closed · ✗ 不当「已在 HEAD」用）。
	if root == "" {
		return gapHeadUnknown
	}
	if _, ok := gapHeadGitRO(root, "rev-parse", "--verify", "-q", "HEAD"); !ok {
		return gapHeadUnknown
	}
	// ① 在 HEAD？—— 只按 `unit` 这一段问 git（✗ 不拿 `<id>@<unit>` 去拼 pathspec）。
	out, ok := gapHeadGitRO(root, "ls-tree", "-r", "--name-only", "HEAD", "--", u)
	if !ok {
		return gapHeadUnknown
	}
	inHead := false
	for _, ln := range strings.Split(out, "\n") {
		if strings.TrimSpace(ln) == u {
			inHead = true
			break
		}
	}
	if !inHead {
		return gapHeadNotInHead
	}
	// 件在 HEAD ⇒ 再看工作树相对 HEAD 有没有改动（★ 共件假阳就在这一格：别路在途改动也会让它是脏的）。
	dirty, ok := gapHeadGitRO(root, "status", "--porcelain", "--", u)
	if !ok {
		return gapHeadUnknown
	}
	if strings.TrimSpace(dirty) != "" {
		return gapHeadHeadDirty
	}
	return gapHeadInHead
}

// gapHeadUnitForID —— 从真源（**只读**）取该账行的 `unit`（= 本账修的唯一标识串 · 取法见上）。
// 取不到（账读不到 / 号不在账里）⇒ ok=false ⇒ 上层判「判不了」（✗ 不许当「不在 HEAD」）。
func gapHeadUnitForID(ledgerPath, id string) (string, bool) {
	b, err := os.ReadFile(ledgerPath)
	if err != nil {
		return "", false
	}
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(ln), &m) != nil {
			continue
		}
		if s, _ := m["id"].(string); s == id {
			u, _ := m["unit"].(string)
			return strings.TrimSpace(u), true
		}
	}
	return "", false
}

// gapHeadTierForGate —— 共用闸那一格：「按账行取标识串（`unit`）⇒ 判 HEAD 档」。
func gapHeadTierForGate(ledgerPath, id string) string {
	unit, ok := gapHeadUnitForID(ledgerPath, id)
	if !ok {
		return gapHeadUnknown
	}
	return gapHeadTierOf(unit, repoRoot())
}

// gapHeadTierLine —— 人面那一行（与机器面 `meta.head_tier` **同源** · 口径随行印出）。
func gapHeadTierLine(tier string) string {
	return "  是否已在HEAD : " + tier + "（本账修的唯一标识串 = 该行 `unit` 件路径 · 只读 git 判 `ls-tree HEAD` ＋ `status --porcelain` · 只报告不进退出码 · ✗ 不取消原有「脏不脏」两态信号）"
}

// gapTwoStateGate —— 「新账销案必须带两态读数」的**共用闸**（缺口 `GAP-20260928-195`）。
//
// 病（该缺口逐字）：闸此前**只内联**在单条 `set-state` 一处 ⇒ `gap bulk set-state`（批量改态）与
// 判据批量面 `gap verify`（跑判据改态）同样能把号改到 `已解`，却**绕道**不受闸约束 ⇒ 「无两态读数
// 直接销案」依然可行，闸形同虚设。回执 `receipt apply`（`已派` → `已解`）同理是把号改到 `已解` 的第四条面。
//
// 治：把判决抽到**这一处共用函数** —— 所有能把号改到 `已解` 的写面在**真写之前**一律调它。判决本身
// **不另造口径**：仍走本片唯一判处 `gapTwoStateJudge`（形 `齐`/`缺`/`不适用` · 存量豁免 · 逐段缺名）。
// 放行（非 `已解` 面 / 存量豁免 / 两态齐）⇒ rc=`exitOK`（一个字不打）；拒收 ⇒ rc=`exitUsage`
// （逐段缺名 + 形态说明 + 生效面打到 stderr · `--json` 信封面落 `two_state=缺`）。
// **读面动作**：本函数一个字都不写（拒收时真源一个字节未改、审计一行未落 —— 闸在任何「审计先落盘」之前）。
func gapTwoStateGate(inv *invocation, id, stateWant, cmdLabel, ledgerPath string, ledgerLines int, stderr io.Writer) int {
	pair, exempt, miss := gapTwoStateJudge(inv, id, stateWant)
	// ★ 承接项 ⑨（缺口 `GAP-20260927-62`）：在原「两态」之上**再加一档**「是否已在 HEAD」
	//   （只读 · 只报告 · **分层展示** ⇒ ✗ 不取消原有「脏不脏」信号 —— 口径见本函数上方那一节）。
	headTier := gapHeadTierForGate(ledgerPath, id)
	inv.metaAddStr("head_tier", headTier)
	// ★ GAP-20260928-196（甲）：自动跑那两条（`--before-cmd`/`--after-cmd`）**跑不起来** ⇒ **判不了**
	//   （退码 8 · 该族既有档）—— 真源一个字节未改、审计一行未落；✗ 绝不当 0 用。
	if pair == "判不了" {
		inv.setErr("blocked", "two_state_unrunnable", "两态读数现读跑不起来（判不了）")
		for _, m := range miss {
			fmt.Fprintf(stderr, "%s: `gap %s` **判不了**（退码 %d）—— %s\n", progName, cmdLabel, exitBlocked, m)
		}
		fmt.Fprintf(stderr, "  真源         : %s（%d 行 · **本发一个字节未改**、审计一行未落 —— 闸在「审计先落盘」之前）\n", orDash(ledgerPath), ledgerLines)
		gapTwoStateMeta(inv, "判不了", false)
		fmt.Fprintln(stderr, gapHeadTierLine(headTier))
		return exitBlocked
	}
	if stateWant != gapStSolved || exempt || len(miss) == 0 {
		// ★ 承接项 ⑨：放行档也把「是否已在 HEAD」摆出来（分层展示 · 只报告 ⇒ **不进退出码**）。
		//   ★ 共件假阳场景（件在 HEAD 但工作树又改 / 件不在 HEAD）⇒ 明写「不判已修」；
		//   ✗ 但**不拦**（拦不拦由两态那三件定 —— 本档只治「误报已修」这个**读数**）。
		if stateWant == gapStSolved && (headTier == gapHeadHeadDirty || headTier == gapHeadNotInHead) {
			fmt.Fprintln(stderr, gapHeadTierLine(headTier))
		}
		return exitOK
	}
	inv.setErr("usage", "two_state_missing", "销案缺两态读数（缺 "+strconv.Itoa(len(miss))+" 段）")
	if gapTwoStateNewLedger(id) {
		fmt.Fprintf(stderr, "%s: `gap %s` **销案拒收** —— %s（号内日期 %s ≥ %s ⇒ **新账**：拿不出两态读数就不给销）\n",
			progName, cmdLabel, id, orDash(gapIDDate(id)), gapTwoStateDateMin)
	} else {
		// GAP-20260929-17：存量账但**给了值却形态不符** ⇒ 同样拒收（不许静默降级成存量豁免）。
		fmt.Fprintf(stderr, "%s: `gap %s` **销案拒收** —— %s（存量账：`--before-read`/`--after-read` **给了值但形态不符** ⇒ 按 GAP-20260929-17 不降级成存量豁免）\n",
			progName, cmdLabel, id)
	}
	for _, m := range miss {
		fmt.Fprintf(stderr, "    ✗ %s\n", m)
	}
	fmt.Fprintf(stderr, "  两态读数形态 : --before-read \"cmd=<命令> · rc=<真退码> · reading=<关键读数一行>\" · --after-read 同形（改前/改后**成对**）\n")
	fmt.Fprintf(stderr, "  生效面       : 新账（号内日期 ≥ %s）强制 · 存量账豁免**仅当两段真没给**（给了值却形态不符 ⇒ 拒收 · GAP-20260929-17）（设计稿 §11.5 两态对拍 · 共用闸取自缺口 `GAP-20260928-195`）\n", gapTwoStateDateMin)
	fmt.Fprintf(stderr, "  真源         : %s（%d 行 · **本发一个字节未改**、审计一行未落 —— 闸在「审计先落盘」之前）\n",
		orDash(ledgerPath), ledgerLines)
	fmt.Fprintln(stderr, gapHeadTierLine(headTier))
	gapTwoStateMeta(inv, "缺", false)
	return exitUsage
}

// gapTwoStateMeta —— `--json` 同族信封面上的两格（本片新增：回显**两态是否齐**、**豁免与否**）。
func gapTwoStateMeta(inv *invocation, pair string, exempt bool) {
	inv.metaAddStr("two_state", pair)
	inv.metaAddJSON("exempt", boolWord(exempt))
}

// gapTwoStateExemptLine —— 存量豁免的**表尾**那一行（不许静默豁免）。
func gapTwoStateExemptLine(w io.Writer) {
	fmt.Fprintf(w, "%s\n", gapTwoStateExemptText)
}

// ── ⒜ `zerg gap set-state <GAP id> --state <仍缺|已解> --evidence <一句话>` ──────────────────────

// ── 「前提不成立」销案的**可现读锚**闸（本枚）────────────────────────────────────────────────
//
// 病（本单靶心 · 上一单负控实测）：`--state 已解 --solved-kind 前提不成立` 是**不受「未落提交面禁销」
// 硬线约束的例外通道** —— 而这个值**谁都能填**：存量账零门槛（无两态闸），新账也只机械卡「两态读数」
// ⇒ 一条**本该真做**的账条，只要填个 `前提不成立` 就能销掉，硬线被这条旁路架空（正门对「误销」零防护）。
//
// 治：销案要**语义可核** —— 填 `前提不成立` 时，证据必须点名**一个可现读的物**（✗ 不收纯自然语言）：
//
//	① **两态读数段**（证据里含 `cmd=` ⇒ `gap reread` 能复跑，读数可核）；
//	② **`件:行` 引用**（`<件>:<行>` 且该件**现读可得**：绝对 / cwd 相对 / 仓根相对 / 仓内跟踪件的 basename）；
//	③ **命令串**（`zerg …` 且能在命令树里解析到 —— 复用 `gapJudgeArgv`，✗ 不新造解析面）。
//
// ★ 只**加严这一档**：其它 `--solved-kind` 值 / 非 `已解` 态 / 两态闸 / 幂等 / 写路**一字不动**
//   （不动的判据 = 正控用例：`--solved-kind 销案` + 散文 ⇒ 仍 rc=0）。
// ★ 排在**用法面**（读真源之前、`--dry-run` 与 `--yes` 都要过）⇒ 拒收时真源一个字节未改、审计一行
//   未落，且 `--dry-run` 那一态**也**拒（病正是 `--dry-run` 就放行）。
// ★ 取值面 = `--evidence` ∪ `--before-read` ∪ `--after-read` ∪ `--before-cmd` ∪ `--after-cmd`
//   （五者**同批**构成落库的 `solved_evidence` ⇒ 锚在其中任一处即算点名了物）。

// gapPremiseAnchorBlob —— 「前提不成立」销案证据的**取值面**（五枚旗标的并集）。
func gapPremiseAnchorBlob(inv *invocation) string {
	return strings.Join([]string{
		inv.flagVal("--evidence"),
		inv.flagVal("--before-read"), inv.flagVal("--after-read"),
		inv.flagVal("--before-cmd"), inv.flagVal("--after-cmd"),
	}, " ")
}

// gapPremiseAnchorWhy —— `前提不成立` 销案的**可现读锚**闸：空串 = 有锚（放行）；否则 = 拒收理由（逐条点名缺什么）。
func gapPremiseAnchorWhy(inv *invocation) string {
	blob := strings.TrimSpace(gapPremiseAnchorBlob(inv))
	if blob == "" {
		return "证据是空的（「前提不成立」销案必须点名一个**可现读的物**）"
	}
	// ① 两态读数段：含 `cmd=` ⇒ 读数可被 `gap reread` 复跑、可核。
	if strings.Contains(blob, "cmd=") {
		return ""
	}
	// ② `件:行` 引用（该件现读可得）
	root := repoRoot()
	var tracked map[string]bool
	trackedDone := false
	for _, tok := range gapPremiseTokens(blob) {
		f, ok := gapPremiseFileLine(tok)
		if !ok {
			continue
		}
		if gapPremiseFilePathExists(f) {
			return ""
		}
		if !strings.Contains(f, "/") && root != "" {
			if !trackedDone {
				tracked = gapPremiseTrackedBasenames(root)
				trackedDone = true
			}
			if tracked[f] {
				return ""
			}
		}
	}
	// ③ 命令串：`zerg …` 且能在命令树里解析到
	if gapPremiseHasResolvableCmd(blob) {
		return ""
	}
	return "「前提不成立」销案必须点名一个**可现读的物**（① 两态读数段 `cmd=…`；② `件:行` 且该件现读可得；" +
		"③ 命令树里解析得到的 `zerg …` 命令）—— 只写「前提不成立」一类自然语言**不收**（该值是例外通道，会架空「未落提交面禁销」硬线）"
}

// gapPremiseTokens —— 证据串分词（空白 + 常见中英标点；**留** `:` / `/` / `.` / `-` / `_` ⇒ `件:行` 不被切断）。
func gapPremiseTokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ',', ';', '(', ')', '[', ']', '{', '}', '"', '\'', '`', '·', '<', '>', '|', '\\',
			'，', '。', '；', '、', '：', '（', '）', '【', '】', '《', '》', '「', '」', '『', '』',
			'“', '”', '‘', '’', '…', '！', '？':
			return true
		}
		return false
	})
}

// gapPremiseFileLine —— `<件>:<行>` 形态（行号起首为数字）⇒ 返回件名面；否则 ok=false。
func gapPremiseFileLine(tok string) (string, bool) {
	i := strings.Index(tok, ":")
	if i <= 0 || i == len(tok)-1 {
		return "", false
	}
	f, rest := tok[:i], tok[i+1:]
	if rest[0] < '0' || rest[0] > '9' {
		return "", false
	}
	// 件名面至少要有一个像「后缀」的点、或含路径分隔（挡 `rc:1` 一类误判）。
	if !strings.Contains(f, ".") && !strings.Contains(f, "/") {
		return "", false
	}
	return f, true
}

// gapPremiseFilePathExists —— 该件名面**路径形**现读可得（绝对 / cwd 相对 / 仓根相对）。
func gapPremiseFilePathExists(f string) bool {
	if f == "" {
		return false
	}
	cands := []string{}
	if filepath.IsAbs(f) {
		cands = append(cands, f)
	} else {
		if wd, err := os.Getwd(); err == nil {
			cands = append(cands, filepath.Join(wd, f))
		}
		if root := repoRoot(); root != "" {
			cands = append(cands, filepath.Join(root, f))
		}
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

// gapPremiseTrackedBasenames —— 仓内**跟踪件**的 basename 集（一次 `git ls-files`；取不到 ⇒ nil ⇒ 该档不认）。
func gapPremiseTrackedBasenames(root string) map[string]bool {
	out, err := exec.Command("git", "-C", root, "-c", "core.quotepath=false", "ls-files").Output()
	if err != nil {
		return nil
	}
	m := map[string]bool{}
	for _, ln := range strings.Split(string(out), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			m[filepath.Base(ln)] = true
		}
	}
	return m
}

// gapPremiseHasResolvableCmd —— 证据里有没有一条能在**命令树**里解析到的 `zerg …` 命令（复用 `gapJudgeArgv`）。
func gapPremiseHasResolvableCmd(blob string) bool {
	toks := strings.Fields(blob)
	for i, t := range toks {
		if t != "zerg" {
			continue
		}
		if _, why := gapJudgeArgv(strings.Join(toks[i:], " ")); why == "" {
			return true
		}
	}
	return false
}

// gapJudgeCellReading —— 由**已跑出的**一格 `cell` 拼三格读数串（`cmd=… · rc=… · reading=…`）。
// 形态与 `gapTwoStateAutoReading` 逐字同源（stdout 首个非空行 · 段内 ` · ` 换 ` / `）；抽出来是为让
// `--judge-cmd` 真跑那一路**复用同一条拼法**（✗ 不另写第二个读数格式器，免两处各拼一套）。
func gapJudgeCellReading(cmdStr string, cell gateFourStatesCell) string {
	reading := gapTwoStateFirstLine(string(cell.Out))
	if reading == "" {
		reading = fmt.Sprintf("(无 stdout 输出 · rc=%d)", cell.RC)
	}
	reading = strings.ReplaceAll(reading, gapTwoStateSep, " / ")
	return "cmd=" + cmdStr + gapTwoStateSep + "rc=" + strconv.Itoa(cell.RC) + gapTwoStateSep + "reading=" + reading
}

// gapPremiseJudgeRun —— 真跑 `--judge-cmd` 一次（本枚 · main.go 开窗第 4 条 · 「前提不成立」销案的
// **更强一路**）：返回 (拒收理由, 三格读数) —— 理由空 = **判据通过**（读数是非空的三格串，供落 `solved_evidence`）。
//
// 形态**照拄**同族既有 runner `gateFourStatesRun`（切词 · **无 shell** · 60s 上限 · 收 rc/stdout）
// —— ✗ 不另造第二套执行器：
//
//	· `rc=0`           ⇒ **判据成立**（`why=""` · 读数三格 = `cmd=… · rc=0 · reading=…`）；
//	· `rc≠0`           ⇒ **判据未通过**（销案拒收 · 逐字报 rc 与读数）；
//	· `Empty`/126/127  ⇒ **跑不起来**（命令不存在 / 不可执行 ⇒ 判不了 · 拒收 · ✗ 不当通过用）。
//
// ★ 命令串的**危险形态**照本仓既有口径处理（`--before-cmd`/`--after-cmd` 同款）：`strings.Fields` 切词 ＋ `exec.CommandContext`（**无 shell 展开**）⇒ `;` / `|` / `&&` 只当字面 argv token（✗ 不新造策略）。
func gapPremiseJudgeRun(cmdStr string) (why, reading string) {
	cell := gateFourStatesRun(cmdStr, nil)
	if cell.Empty {
		return "`--judge-cmd` 的命令**跑不起来**（命令不存在 / 不可执行 ⇒ 现读拿不到读数）—— 判据取证不成立（✗ 不当通过用）", ""
	}
	if cell.RC == 126 || cell.RC == 127 {
		return fmt.Sprintf("`--judge-cmd` 的命令**跑不起来**（rc=%d = 命令不可执行 / 不存在）—— 判据取证不成立（✗ 不当通过用）", cell.RC), ""
	}
	reading = gapJudgeCellReading(cmdStr, cell)
	if cell.RC != 0 {
		return fmt.Sprintf("`--judge-cmd` 判据**未通过**（rc=%d ≠ 0）—— 前提状态未被读数证明（%s）", cell.RC, reading), ""
	}
	return "", reading
}

func cmdGapSetState(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	stateWant := strings.TrimSpace(inv.flagVal("--state"))
	evidence := strings.TrimSpace(inv.flagVal("--evidence"))
	// ★ 本枚（缺口账 `GAP-20260927-249`）：「销案 ≠ 修」的**分栏** —— `--state 已解` 时给这一类。
	solvedKind := strings.TrimSpace(inv.flagVal("--solved-kind"))
	// ★ 缺口 `GAP-20260927-244` 方案乙：`--remainder <残面描述>` —— `部分` 档的**残面另记**
	//   （本条 `state=已解` 且 `solved_kind=部分` 时，把**未治的残面**一句话落 `gapRecord.Remainder`）。
	remainder := strings.TrimSpace(inv.flagVal("--remainder"))
	// ★ 本枚（main.go 开窗第 4 条）：`--judge-cmd <命令串>` —— 「前提不成立」销案的**可跑判据**。
	//   给了 ⇒ 由正门**真跑**它、用 rc/读数当证据（比 DEV-2443「只查提到了某个物」更强的**那一路**）。
	judgeCmd := strings.TrimSpace(inv.flagVal("--judge-cmd"))

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
		fmt.Fprintf(stderr, "用法：zerg gap set-state <GAP id> --state <%s> --evidence <一句话> [--solved-kind <实现|销案|前提不成立|部分>] [--remainder <残面描述>] [--before-cmd <命令串>] [--after-cmd <命令串>] [--judge-cmd <命令串>] [--before-read <cmd=… · rc=… · reading=…>] [--after-read <cmd=… · rc=… · reading=…>] [--by <谁>] [--dry-run | --yes]\n",
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
	// ★ 本枚（缺口账 `GAP-20260927-249`）：`--solved-kind` 的三道用法闸（裸给 / 闭集外 / 只对已解）。
	if inv.hasFlag("--solved-kind") && solvedKind == "" {
		inv.setErr("usage", "solved_kind_missing_value", "--solved-kind 没给值")
		fmt.Fprintf(stderr, "%s: `--solved-kind` 要给一个值（只认 %s）—— 裸给不降级（用法错 2）\n",
			progName, gapClosedText(gapSolvedKindClosed))
		return exitUsage
	}
	if solvedKind != "" && !gapIn(gapSolvedKindClosed, solvedKind) {
		inv.setErr("usage", "bad_solved_kind", "solved_kind 值不在闭集里")
		fmt.Fprintf(stderr, "%s: `--solved-kind %s` 不在闭集里 —— 只认 %s\n",
			progName, solvedKind, gapClosedText(gapSolvedKindClosed))
		return exitUsage
	}
	if solvedKind != "" && stateWant != gapStSolved {
		inv.setErr("usage", "solved_kind_only_for_solved", "--solved-kind 只对 --state 已解 有效")
		fmt.Fprintf(stderr, "%s: `--solved-kind` 只对 `--state %s` 有效（本发 --state %s）—— 改别的态不带它\n",
			progName, gapStSolved, stateWant)
		return exitUsage
	}
	// ★ 缺口 `GAP-20260927-244`：`--remainder` 两道用法闸（裸给 / 只对已解）—— 与 `--solved-kind` 同款。
	if inv.hasFlag("--remainder") && remainder == "" {
		inv.setErr("usage", "remainder_missing_value", "--remainder 没给值")
		fmt.Fprintf(stderr, "%s: `--remainder` 要给一段**残面描述**（裸给不降级 —— 用法错 2）\n", progName)
		return exitUsage
	}
	if remainder != "" && stateWant != gapStSolved {
		inv.setErr("usage", "remainder_only_for_solved", "--remainder 只对 --state 已解 有效")
		fmt.Fprintf(stderr, "%s: `--remainder` 只对 `--state %s` 有效（本发 --state %s）—— 改别的态不带它\n",
			progName, gapStSolved, stateWant)
		return exitUsage
	}
	// ★ 本枚（main.go 开窗第 4 条）：`--judge-cmd` 两道用法闸（裸给 / 只对「前提不成立」销案）—— 与
	//   `--remainder` 同款；防**静默吞**（本旗标只这一档有语义，别的档给它是用法错、不许当无事发生）。
	if inv.hasFlag("--judge-cmd") && judgeCmd == "" {
		inv.setErr("usage", "judge_cmd_missing_value", "--judge-cmd 没给值")
		fmt.Fprintf(stderr, "%s: `--judge-cmd` 要给一条**可跑命令串**（裸给不降级 —— 用法错 2）\n", progName)
		return exitUsage
	}
	if judgeCmd != "" && (stateWant != gapStSolved || solvedKind != gapSolvedKindPremiseInvalid) {
		inv.setErr("usage", "judge_cmd_only_for_premise", "--judge-cmd 只对 --state 已解 --solved-kind 前提不成立 有效")
		fmt.Fprintf(stderr, "%s: `--judge-cmd` 只对 `--state %s --solved-kind %s` 有效（本发 --state %s --solved-kind %s）—— 其它档不带它\n",
			progName, gapStSolved, gapSolvedKindPremiseInvalid, stateWant, orDash(solvedKind))
		return exitUsage
	}

	// ★ 本枚：「**前提不成立**」销案的**语义防护** —— 该值是**例外通道**（不受「未落提交面禁销」硬线约束）；
	//   此前谁都能填 ⇒ 本该真做的账条填它就能销掉。这里在**用法面**加一道：证据必须点名一个**可现读的物**
	//   （两态读数段 / `件:行`（件现读可得）/ 命令树里解析得到的 `zerg …` 命令）⇒ `--dry-run` 与真写**都**拒
	//   （病正是 `--dry-run` 就放行）；其它 kind / 非已解态**一字不动**。
	// ★ 本枚（main.go 开窗第 4 条 · **更强的一路**）：给了 `--judge-cmd` ⇒ 由正门**真跑它**、用 rc/读数当
	//   证据（`rc=0` = 判据成立 · rc≠0 / 跑不起来 ⇒ 拒收并逐字报 rc 与读数）；没给 ⇒ **维持**上面的
	//   DEV-2443 现判（向后兼容 · ✗ 不把已有的合法「前提不成立」销案判红）。
	judgeReading := ""
	if solvedKind == gapSolvedKindPremiseInvalid {
		var why string
		if judgeCmd != "" {
			why, judgeReading = gapPremiseJudgeRun(judgeCmd)
		} else {
			why = gapPremiseAnchorWhy(inv)
		}
		if why != "" {
			inv.setErr("usage", "premise_evidence_missing", why)
			fmt.Fprintf(stderr, "%s: `--solved-kind %s` 销案**拒收** —— %s\n", progName, gapSolvedKindPremiseInvalid, why)
			fmt.Fprintf(stderr, "  取值面   : --evidence ∪ --before-read ∪ --after-read ∪ --before-cmd ∪ --after-cmd ∪ --judge-cmd（同批构成落库的 solved_evidence）\n")
			fmt.Fprintf(stderr, "  为何     : `%s` 是**例外通道**（不受「未落提交面禁销」硬线约束）⇒ 销案要语义可核、✗ 不收纯自然语言\n", gapSolvedKindPremiseInvalid)
			fmt.Fprintf(stderr, "  真源     : %s（**本发一个字节未改**、审计一行未落 —— 闸在「审计先落盘」之前）\n", gapLedgerPath())
			return exitUsage
		}
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

	// ④★★★ 本枚（缺口账 `GAP-20260927-249`）：**销案分栏**取值 —— 改到 `已解` 时落这一类。
	//   取定：给了 `--solved-kind` ⇒ 用它；没给而该行**已有** kind ⇒ **保留旧值**（幂等向）；
	//   没给且该行**没有** kind（老账首改）⇒ 落 `未分类`（与「一格都没有」同义，不报错）。
	//   ★ 改到**非** `已解` 的态 ⇒ 本格**一个字节不动**（`r.SolvedKind` 原样带回）。
	kindWant := r.SolvedKind
	if stateWant == gapStSolved {
		switch {
		case solvedKind != "":
			kindWant = solvedKind
		case strings.TrimSpace(r.SolvedKind) == "":
			kindWant = gapSolvedKindUnclassified
		}
	}
	// ★ 缺口 `GAP-20260927-244`：**残面文字**取值 —— 给了就写；没给 ⇒ 保留旧值（幂等向）。
	//   ★ 改到**非** `已解` 的态 ⇒ 本格**一个字节不动**（`r.Remainder` 原样带回，与 `solved_kind` 同处置）。
	remainderWant := r.Remainder
	if stateWant == gapStSolved && inv.hasFlag("--remainder") {
		remainderWant = remainder
	}

	// ④★ 批4 第四片：两态读数**取值**（只在改到 `已解` 时有语义；其余状态 ⇒ 「不适用」· 一个字都不多打）。
	twoStatePair, twoStateExempt, twoStateMiss := gapTwoStateJudge(inv, r.ID, stateWant)
	// ★ GAP-20260928-196（甲）：自动跑那两条（`--before-cmd`/`--after-cmd`）的取值走**唯一 resolver**
	//   （与裁决同源）：跑不起来 ⇒ **判不了**（退码 8 · 该族既有档）—— 真源一个字节未改、审计一行未落；
	//   同给 cmd 与 read ⇒ **明报**「现读优先」（✗ 不静默取一）。非销案态 ⇒ 不碰 resolver（不跑命令）。
	var twoStateRes *gapTwoStateResolution
	if stateWant == gapStSolved {
		twoStateRes = gapTwoStateResolve(inv)
		if twoStateRes.Note != "" {
			fmt.Fprintf(stderr, "%s: %s\n", progName, twoStateRes.Note)
		}
		if twoStatePair == "判不了" {
			inv.setErr("blocked", "two_state_unrunnable", "两态读数现读跑不起来（判不了）")
			for _, m := range twoStateMiss {
				fmt.Fprintf(stderr, "    ✗ %s\n", m)
			}
			fmt.Fprintf(stderr, "  真源         : %s（%d 行 · **本发一个字节未改**、审计一行未落）\n", orDash(led.Path), len(led.Lines))
			return exitBlocked
		}
	}

	// ④★★ 批4 第五片：**销案落账** —— 两态**齐** ⇒ 把两态读数按写死的字段名落进该条账行的
	// `solved_evidence`（设计稿 §11.5 逐字「自动写进 solved_evidence」· `O-13`「钉成可解析形态」）。
	// **不许静默丢弃**：只有两段都过上面四格真判（`齐`）才落；不齐 / 非销案 ⇒ 取值逐字 = `--evidence`
	// （即：现有一切调用路径的行为逐字不变）。`landed` 同时是**幂等比对**与**真写**共用的那一格取值。
	landed := evidence
	if twoStatePair == "齐" {
		landed = gapTwoStateLanded(evidence, twoStateRes.Before, twoStateRes.After)
	}
	// ★ 本枚：给了 `--judge-cmd` 且判据**通过** ⇒ 把**真跑出的三格读数**落进 `solved_evidence`
	//   （「用读数证明前提状态」· ✗ 不再是「写一段话」；判据未通过者在上面的闸就拒收了、到不了这里）。
	if judgeReading != "" {
		landed += gapTwoStateSep + "判据{" + judgeReading + "}"
	}

	// ⑤ 幂等：状态**与**证据（改到 `已解` 时另加 `solved_kind` / `remainder`）都逐字已在位 ⇒ 「无变化」（0）。
	if r.State == stateWant && r.Evidence == landed &&
		(stateWant != gapStSolved || (gapSolvedKindOf(r) == kindWant && r.Remainder == remainderWant)) {
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			gapTwoStateMeta(inv, twoStatePair, twoStateExempt)
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				map[string]string{"id": r.ID, "fp": r.FP, "state": r.State, "solved_kind": gapSolvedKindOf(r), "remainder": r.Remainder, "changed": "false",
					"two_state": twoStatePair, "exempt": boolWord(twoStateExempt)})
		}
		fmt.Fprintf(stdout, "无变化 %s · state=%s（同 id 同态同证据 ⇒ 幂等命中：不写真源、不写审计）\n", r.ID, r.State)
		return exitOK
	}

	// ⑥ `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		if inv.jsonGiven {
			inv.changed = boolPtr(false)
			gapTwoStateMeta(inv, twoStatePair, twoStateExempt)
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				map[string]string{"id": r.ID, "fp": r.FP, "state": stateWant, "solved_kind": kindWant, "remainder": remainderWant, "changed": "false",
					"two_state": twoStatePair, "exempt": boolWord(twoStateExempt)})
		}
		plan2 := "evidence     : " + evidence
		if stateWant == gapStSolved {
			plan2 += fmt.Sprintf("\n    solved_kind  : %s → %s", gapSolvedKindOf(r), kindWant)
			plan2 += fmt.Sprintf("\n    remainder    : %s → %s", gapShowText(r.Remainder), gapShowText(remainderWant))
		}
		if judgeReading != "" {
			plan2 += "\n    judge        : " + judgeReading
		}
		gapStatePlanBlock(stdout, "--dry-run", "set-state", led.Path, len(led.Lines),
			r.ID, r.FP, r.State, stateWant,
			fmt.Sprintf("solved_at    : %s", gapSolvedAtPlan(r.State, stateWant)),
			plan2)
		if twoStateExempt {
			gapTwoStateExemptLine(stdout)
		}
		if stateWant == gapStSolved && gapTwoStateNewLedger(r.ID) && len(twoStateMiss) > 0 {
			fmt.Fprintf(stderr, "⚠ 预演：新账销案缺两态读数（缺 %d 段）—— 真写那一发会被拒（退 2）\n", len(twoStateMiss))
		}
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}

	// ⑤★ 批4 第四片：**销案两态对拍闸**（设计稿 §11.5：「新账强制 · 存量豁免」）—— 只挡真写这一态（`--dry-run` 恒 0）。
	//    新账（号内日期 ≥ `2026-09-28`）+ 改到 `已解` + 两态读数不齐 ⇒ **拒收**（退 2 · 逐段点名）。
	//    ★ 缺口 `GAP-20260928-195`（本枚）：判决已**抽成共用闸** `gapTwoStateGate` —— 与批量
	//      `bulk set-state`、判据批量面 `verify`、回执面 `receipt apply` **同一道判**（✗ 不再只内联在本一处）。
	//    ⚠ 排在这一处：幂等「无变化」与 `--dry-run` 都在上面先行返回（预演不该被拦）；
	//    闸只挡真写；预演由上面的提示行先告知会被拒。
	//    真源在这之前只被读过 —— 拒收时**一个字节未改、审计一行未落**。
	if rc := gapTwoStateGate(inv, r.ID, stateWant, "set-state", led.Path, len(led.Lines), stderr); rc != exitOK {
		return rc
	}

	// ★ 本枚（缺口账 `GAP-20261005-26`）：**字段面前置判定** —— 把 `--json` 字段名校验从出口
	//    `selectJSON`（真写**之后**）前移到第一个真写动作之前：坏字段 ⇒ 同 kind 同 detail 同退码，
	//    而真源**一个字节不落、审计一行不落**（改前「先写后校」⇒ 写得成也退得了 2）。
	if rc := gapFrontJSONFields(inv, stderr, gapSetStateFields); rc != exitOK {
		return rc
	}

	// ⑦ 真写：`state` + `solved_at`（只在**真变入终态**时取）+ `solved_evidence`。
	//    `solved_at` = **历史时刻**（这一条**头一回**落进 `已解` / `不做` 的那一刻），不是「现态那一格」
	//    ⇒ 已已解 / 已已不做**再补写**（补 `solved_kind` / 补证据 / 补注）一律**继承旧值**、一律不重写
	//    （与同族 `verify` 路 `v.After == gapStSolved && v.Before != gapStSolved` 同一句口径 ·
	//    `state` 是现态、`solved_at` 是历史，两格不是同一维；历史不抹也不改）。
	//    ★ 修（缺口账见本件头 ★）：此前这一格**只看 `stateWant`** ⇒ 已是 `已解` / `不做` 的条再补写，
	//      `solved_at` 会被**改写成 now**（而回执那一行又自述「历史时刻**不删**」⇒ 口实不符）。
	before := r.State
	r.State = stateWant
	r.Evidence = landed
	if stateWant == gapStSolved {
		r.SolvedKind = kindWant
		r.Remainder = remainderWant
	}
	if (stateWant == gapStSolved || stateWant == gapStWontDo) && before != stateWant {
		r.SolvedAt = gapNow()
	}
	detail, rc, msg := gapRewriteOne(inv, led, idx, r, "set-state", before, stateWant)
	if rc != exitOK {
		gapWriteFail(inv, stderr, detail, msg, "set-state")
		return rc
	}
	inv.changed = boolPtr(!(before == stateWant && led.Recs[idx].Evidence == landed))
	if inv.jsonGiven {
		gapTwoStateMeta(inv, twoStatePair, twoStateExempt)
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
			map[string]string{"id": r.ID, "fp": r.FP, "state": r.State, "solved_kind": gapSolvedKindOf(r), "remainder": r.Remainder, "changed": boolWord(before != stateWant || led.Recs[idx].Evidence != landed),
				"two_state": twoStatePair, "exempt": boolWord(twoStateExempt)})
	}
	fmt.Fprintf(stdout, "已改态 %s · state %s → %s（真源 %d 行不变 · 只重写目标那一行 · 审计已落 1 行）\n",
		r.ID, before, r.State, len(led.Lines))
	fmt.Fprintf(stdout, "  solved_evidence : %s\n", r.Evidence)
	if stateWant == gapStSolved {
		fmt.Fprintf(stdout, "  solved_kind     : %s（销案分栏 · 缺口账 `GAP-20260927-249`）\n", gapSolvedKindOf(r))
		if r.Remainder != "" {
			fmt.Fprintf(stdout, "  remainder       : %s（残面另记 · 缺口账 `GAP-20260927-244`）\n", r.Remainder)
		}
	}
	if twoStatePair == "齐" {
		fmt.Fprintf(stdout, "  两态·改前    : %s\n", strings.TrimSpace(twoStateRes.Before))
		fmt.Fprintf(stdout, "  两态·改后    : %s\n", strings.TrimSpace(twoStateRes.After))
	}
	if stateWant == gapStSolved {
		fmt.Fprintf(stdout, "  solved_at       : %s\n", r.SolvedAt)
	} else {
		fmt.Fprintf(stdout, "  solved_at       : %s（历史时刻**不删** —— 追加式口径）\n", orDash(r.SolvedAt))
	}
	if twoStateExempt {
		gapTwoStateExemptLine(stdout)
	}
	gapReceiptPrint(stdout, msg)
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
	// ★ 缺口 `GAP-20260928-58`（本枚 · R1 已登记旗标 `--supersede` 的**读者**）：`--supersede <n|指纹> --text <新文>`
	//   = 一条命令内**作废旧注 + 落新注**（新注**回指**被替注）；读面只显示新注（被替注走 `notes_void`）。
	supersede := strings.TrimSpace(inv.flagVal("--supersede"))
	// ★ 本枚（缺口账 `GAP-20260929-27`）：`--retract` **给了旗标却不给值** ⇒ 不许静默降级成普通注。
	//   旧行为：裸 `--retract` + `--text` ⇒ `flagVal` 得空串 ⇒ 落到「写注」那一支 ⇒ rc=0 且当真注落库
	//   （调用方以为在撤回，实际在追加注）。这里把「**裸给（形态不符）**」与「**真的一个字没给**」分家：
	//   裸给 ⇒ 用法错 2 逐字点名（后者仍走「缺 --text」那一支）。
	if inv.hasFlag("--retract") && retract == "" {
		inv.setErr("usage", "retract_missing_value", "--retract 没给值")
		fmt.Fprintf(stderr, "%s: `--retract` 要给一个值（序号 / 注指纹）—— 裸给**不降级**成普通注（用法错 2）\n", progName)
		fmt.Fprintf(stderr, "  作废：zerg gap note <GAP id> --retract <n|指纹> [--by <谁>] [--dry-run | --yes]\n")
		return exitUsage
	}
	// ★ GAP-20260928-58（本枚）：`--supersede` 同样「裸给不降级」。
	if inv.hasFlag("--supersede") && supersede == "" {
		inv.setErr("usage", "supersede_missing_value", "--supersede 没给值")
		fmt.Fprintf(stderr, "%s: `--supersede` 要给一个值（序号 / 注指纹）—— 裸给**不降级**成普通注（用法错 2）\n", progName)
		fmt.Fprintf(stderr, "  替代：zerg gap note <GAP id> --supersede <n|指纹> --text <新文> [--by <谁>] [--dry-run | --yes]\n")
		return exitUsage
	}

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
	// ★ GAP-20260928-58（本枚）：`--retract` 与 `--supersede` 是**两种替代关系**，不许同给。
	if retract != "" && supersede != "" {
		inv.setErr("usage", "retract_supersede_conflict", "--retract 与 --supersede 同给")
		fmt.Fprintf(stderr, "%s: `--retract` 与 `--supersede` **不许同给**（一条命令一个动作：作废 vs 替代）\n", progName)
		fmt.Fprintf(stderr, "  作废：zerg gap note <GAP id> --retract <n|指纹> [--by <谁>] [--dry-run | --yes]\n")
		fmt.Fprintf(stderr, "  替代：zerg gap note <GAP id> --supersede <n|指纹> --text <新文> [--by <谁>] [--dry-run | --yes]\n")
		return exitUsage
	}
	if retract != "" && text != "" {
		inv.setErr("usage", "retract_with_text", "--retract 与 --text 同给")
		fmt.Fprintf(stderr, "%s: `--retract` 与 `--text` **不许同给**（一条命令一个动作：作废 = 不写新注 · 写新注 = 不作废）\n", progName)
		fmt.Fprintf(stderr, "  作废：zerg gap note <GAP id> --retract <n|指纹> [--by <谁>] [--dry-run | --yes]\n")
		fmt.Fprintf(stderr, "  写注：zerg gap note <GAP id> --text <一句话> [--by <谁>] [--dry-run | --yes]\n")
		return exitUsage
	}
	// ★ GAP-20260928-58（本枚）：`--supersede` **必须配 `--text`**（替代 = 作废旧注 + 落新注 · 新文必填）。
	if supersede != "" && text == "" {
		inv.setErr("usage", "missing_required", "缺 --text")
		fmt.Fprintf(stderr, "%s: `--supersede` 要配 `--text <新文>`（替代必须给新文 ⇒ 本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "用法：zerg gap note <GAP id> --supersede <n|指纹> --text <新文> [--by <谁>] [--dry-run | --yes]\n")
		return exitUsage
	}
	if text == "" && retract == "" && supersede == "" {
		inv.setErr("usage", "missing_required", "缺 --text")
		fmt.Fprintf(stderr, "%s: `gap note` 缺必填旗标：--text（或 `--retract <n|指纹>` / `--supersede <n|指纹>`）\n", progName)
		fmt.Fprintf(stderr, "用法：zerg gap note <GAP id> --text <一句话> [--retract <n|指纹>] [--by <谁>] [--dry-run | --yes]\n")
		fmt.Fprintf(stderr, "   或：zerg gap note <GAP id> --retract <n|指纹> [--by <谁>] [--dry-run | --yes]（作废一注 · 不删原文）\n")
		fmt.Fprintf(stderr, "   或：zerg gap note <GAP id> --supersede <n|指纹> --text <新文> [--by <谁>] [--dry-run | --yes]（作废旧注 + 落新注）\n")
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
		switch {
		case retract != "":
			plan1 = "notes_void   : （追加 1 条作废标记 · **注一条不删** · 不改 state）"
			plan2 = "void         : --retract " + retract + "（名字在真源读进来之后才判）"
		case supersede != "":
			plan1 = "notes_void + notes: （追加 1 条作废标记 + 追加 1 条新注 · **注一条不删** · 不改 state）"
			plan2 = "supersede    : --supersede " + supersede + " ⇒ 新注 --text " + text + "（名字在真源读进来之后才判）"
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
	// ★ GAP-20260928-58（本枚）：`--supersede` 的真写面（作废旧注 + 落新注 · 新注回指被替注）。
	if supersede != "" {
		return gapNoteSupersede(inv, stdout, stderr, led, idx, r, supersede, text)
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

	// ★ 本枚（缺口账 `GAP-20261005-26`）：**字段面前置判定** —— 把 `--json` 字段名校验从出口
	//    `selectJSON`（真写**之后**）前移到第一个真写动作之前：坏字段 ⇒ 同 kind 同 detail 同退码，
	//    而真源**一个字节不落、审计一行不落**（改前「先写后校」⇒ 写得成也退得了 2）。
	if rc := gapFrontJSONFields(inv, stderr, gapNoteFields); rc != exitOK {
		return rc
	}

	// ⑦ 真写：**只**追加 `notes` 一条；`state` 逐字不动（改前改后现算对拍，破 ⇒ 不给结论）。
	before := r.State
	r.Notes = append(append([]string{}, r.Notes...), gapNoteLine(by, text))
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
	gapReceiptPrint(stdout, msg)
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
	// 幂等（★ 本枚 · 缺口账 `GAP-20261005-25` 改口径）：这条注已在作废位 ⇒ 「无变化」（**只认作废位**，
	//   不再看 `state` —— 新口径：注面只追加 / 作废标记，**一律不改** `state`；旧 `GAP-20260928-167` 那
	//   半「一并置 `不做`」已被新口径推翻，见该条账注）。
	for _, v := range r.Voids {
		if v.N == n {
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
			fmt.Sprintf("notes_void   : %d → %d 条（追加作废标记 · **注一条不删**）· state: %s（**不改** —— 注面只追加，一律不改 `state`）", len(r.Voids), len(r.Voids)+1, r.State),
			fmt.Sprintf("作废那一条   : 第 %d 条 · fp=%s（void:true · 原文在真源里逐字节留着）", n, gapNoteFP(r.Notes[n-1])))
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}
	// ★ 本枚（缺口账 `GAP-20261005-26`）：**字段面前置判定** —— 把 `--json` 字段名校验从出口
	//    `selectJSON`（真写**之后**）前移到第一个真写动作之前：坏字段 ⇒ 同 kind 同 detail 同退码，
	//    而真源**一个字节不落、审计一行不落**（改前「先写后校」⇒ 写得成也退得了 2）。
	if rc := gapFrontJSONFields(inv, stderr, gapNoteFields); rc != exitOK {
		return rc
	}

	// 真写（★ 本枚 · 缺口账 `GAP-20261005-25` 改口径）：**只落一条作废标记**（`notes_void`）——
	//   **不改 `state`**（口径：注面只追加 / 作废标记，一动 `state` 都不是；旧 `GAP-20260928-167` 那半
	//   「一并置 `不做`」已被新口径推翻）；`notes` 逐字不动（真删注 ✗ · 改前改后现算对拍，破 ⇒ 不给结论）。
	before := r.State
	snapNotes := strings.Join(r.Notes, "\x00")
	r.Voids = append(append([]gapNoteVoid{}, r.Voids...), gapNoteVoid{
		Void: true, N: n, FP: gapNoteFP(r.Notes[n-1]), By: gapByOf(inv), At: gapNow(), Why: "撤回",
	})
	if r.State != before {
		inv.setErr("failed", "state_mutated", "作废这一路动了 state")
		fmt.Fprintf(stderr, "%s: 内部对拍破了：作废这一路动了 `state` ⇒ 不给结论（退码 1）\n", progName)
		return exitFail
	}
	if strings.Join(r.Notes, "\x00") != snapNotes {
		inv.setErr("failed", "note_mutated", "作废这一路动了 notes/state")
		fmt.Fprintf(stderr, "%s: 内部对拍破了：作废这一路动了 `notes` ⇒ 不给结论（退码 1）\n", progName)
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
	fmt.Fprintf(stdout, "已作废 %s · 第 %d 条注（fp=%s · notes_void %d → %d 条）\n",
		r.ID, n, gapNoteFP(r.Notes[n-1]), len(r.Voids)-1, len(r.Voids))
	fmt.Fprintf(stdout, "  state    : %s（**未改** —— 作废标记不改态）\n", r.State)
	fmt.Fprintf(stdout, "  真源     : %s（%d 行不变 · **注一条没删** —— 只重写目标那一行）\n", led.Path, len(led.Lines))
	fmt.Fprintf(stdout, "  读面     : 该条已不显示（默认）· `--json void_notes` 带原文出来\n")
	gapReceiptPrint(stdout, msg)
	return exitOK
}

// gapNoteSupersede —— `--supersede <n|指纹>` 的**真写面**（★ 缺口 `GAP-20260928-58` · 本枚）。
//
// 一条命令内做两件事（**同一次写盘** · 走唯一收口 `gapRewriteOne` · 不另开写路）：
//
//	① **作废旧注**：往 `notes_void` 追加一条 `why=替代` 的标记（**禁真删任何注** —— 被替注逐字节
//	   留在 `notes` 里）；
//	② **落新注**：`notes` 追加一条，**回指被替注**（文本尾带 `（替代第 n 条 · fp=…）`）。
//
// `state` 逐字不动（替代本就不改态 · 与 `--text` 那条路同款 · 改前改后现算对拍，破 ⇒ 不给结论）。
// 读面只显示新注（被替注走 `notes_void` / `--json void_notes`）。退码同 `note`（0 · 2 点名不到/命中多条/
// 已被作废 · 8 读写不进）。
func gapNoteSupersede(inv *invocation, stdout, stderr io.Writer, led gapLedger, idx int, r gapRecord, arg, text string) int {
	hits := gapNoteRetractPick(r, arg)
	if len(hits) == 0 {
		inv.setErr("usage", "supersede_target_not_found", "点名点不到注")
		fmt.Fprintf(stderr, "%s: `--supersede %s` 在 %s 的 %d 条注里**一条都对不上**（本族口径 = 用法错 2）\n",
			progName, arg, r.ID, len(r.Notes))
		fmt.Fprintf(stderr, "  点名三种写法：序号（1 起）/ 指纹（sha256 前 12 位）/ 原文前缀\n")
		return exitUsage
	}
	if len(hits) > 1 {
		inv.setErr("usage", "supersede_target_ambiguous", "点名命中多于一条注")
		fmt.Fprintf(stderr, "%s: `--supersede %s` 在 %s 里命中 %d 条（%v）⇒ 收窄到唯一（用序号或指纹）\n",
			progName, arg, r.ID, len(hits), hits)
		return exitUsage
	}
	n := hits[0]
	// 已被作废的注不许再被替代（防重复作废 / 双写标记）。
	for _, v := range r.Voids {
		if v.N == n {
			inv.setErr("usage", "note_already_voided", "该注已在作废位")
			fmt.Fprintf(stderr, "%s: 第 %d 条注已在作废位（`void:true`）⇒ 不再替代（本族口径 = 用法错 2）\n", progName, n)
			fmt.Fprintf(stderr, "  现读：zerg gap show %s --json notes,notes_void,void_notes\n", r.ID)
			return exitUsage
		}
	}
	by := gapByOf(inv)
	oldFP := gapNoteFP(r.Notes[n-1])
	newNote := fmt.Sprintf("%s · %s%s%s（替代第 %d 条 · fp=%s）", gapNow(), by, gapNoteSep, text, n, oldFP)
	// `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		if inv.jsonGiven {
			inv.changed = boolPtr(false)
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields,
				map[string]string{"id": r.ID, "fp": r.FP, "notes": fmt.Sprintf("%d", len(r.Notes)+1), "changed": "false"})
		}
		gapStatePlanBlock(stdout, "--dry-run", "note", led.Path, len(led.Lines), r.ID, r.FP, r.State, "",
			fmt.Sprintf("notes_void   : %d → %d 条（作废标记 · **注一条不删**）· notes: %d → %d 条（新注）· state 不动",
				len(r.Voids), len(r.Voids)+1, len(r.Notes), len(r.Notes)+1),
			fmt.Sprintf("替代那一条   : 第 %d 条 · fp=%s ⇒ 新注 %s", n, oldFP, newNote))
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}
	// ★ 本枚（缺口账 `GAP-20261005-26`）：**字段面前置判定** —— 把 `--json` 字段名校验从出口
	//    `selectJSON`（真写**之后**）前移到第一个真写动作之前：坏字段 ⇒ 同 kind 同 detail 同退码，
	//    而真源**一个字节不落、审计一行不落**（改前「先写后校」⇒ 写得成也退得了 2）。
	if rc := gapFrontJSONFields(inv, stderr, gapNoteFields); rc != exitOK {
		return rc
	}

	// 真写（同一次写盘）：① 作废标记（why=替代）② 新注；`state` 逐字不动（改前改后现算对拍，破 ⇒ 不给结论）。
	before := r.State
	r.Voids = append(append([]gapNoteVoid{}, r.Voids...), gapNoteVoid{
		Void: true, N: n, FP: oldFP, By: by, At: gapNow(), Why: "替代",
	})
	r.Notes = append(append([]string{}, r.Notes...), newNote)
	if r.State != before {
		inv.setErr("failed", "state_mutated", "替代这一路改了 state")
		fmt.Fprintf(stderr, "%s: 内部对拍破了：替代这一路改了 `state`（%s → %s）⇒ 不给结论（退码 1）\n", progName, before, r.State)
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
	fmt.Fprintf(stdout, "已替代 %s · 作废第 %d 条注（fp=%s）→ 落新注（notes %d → %d 条 · notes_void %d → %d 条）\n",
		r.ID, n, oldFP, len(r.Notes)-1, len(r.Notes), len(r.Voids)-1, len(r.Voids))
	fmt.Fprintf(stdout, "  state    : %s（**未改** —— 替代不改态）\n", r.State)
	fmt.Fprintf(stdout, "  读面     : 只显示新注（第 %d 条）· 被替注走 `--json void_notes`\n", len(r.Notes))
	gapReceiptPrint(stdout, msg)
	return exitOK
}

// ── 两条新写面共用的报错面（审计写不进 / 真源写不进 / 写回读不对拍）──────────────────────────

// gapWriteFail —— 真写那三步失败的**唯一报面**（`set-state` / `note` 共用）。
func gapWriteFail(inv *invocation, stderr io.Writer, detail, msg, cmdName string) {
	switch detail {
	case "concurrent_write":
		inv.setErr("conflict", detail, msg)
		fmt.Fprintf(stderr, "%s: **并发冲突** —— 账件在本进程读入之后被别人改过 ⇒ 本次**拒写**（退码 14 · kind=conflict）\n", progName)
		fmt.Fprintf(stderr, "  %s\n", msg)
		fmt.Fprintf(stderr, "  真源     : %s（一个字节未动 · 审计一行未落 —— 闸在「审计先落盘」**之前**）\n", gapLedgerPath())
		fmt.Fprintf(stderr, "  下一步   : **重跑一次即可**（重跑以盘上现状为新基线）—— **不要手改账件**（手改正是这条闸要挡的形态）\n")
	case "borrow_read_only":
		inv.setErr("usage", detail, msg)
		fmt.Fprintf(stderr, "%s: **只读借用档** ⇒ 写面拒执（退码 2 · 取锁之前就拒 —— 借源目录零新增文件，连 .lock 一枚都不造）\n", progName)
		fmt.Fprintf(stderr, "  %s\n", msg)
		fmt.Fprintf(stderr, "  真源     : %s（一字未动 · 那件是**借来**的，借用 = 只读）\n", gapLedgerPath())
		fmt.Fprintf(stderr, "  下一步   : 要写就把旋钮撤掉（`env -u %s`）并把状态目录指回真源\n", gapBorrowEnv)
	case "ledger_locked":
		inv.setErr("usage", detail, msg)
		fmt.Fprintf(stderr, "%s: **账写排他锁取不到** ⇒ 拿不到独占写权，本次**拒写**（退码 2 · 不给结论）\n", progName)
		fmt.Fprintf(stderr, "  %s\n", msg)
		fmt.Fprintf(stderr, "  真源     : %s（一个字节未动 · 审计一行未落 —— 锁排在「读」之前）\n", gapLedgerPath())
		fmt.Fprintf(stderr, "  下一步   : 看锁件能否建/可写（`%s`）—— **不要手改账件**\n", gapLedgerLockPath(gapLedgerPath()))
	case "ledger_unreadable":
		inv.setErr("blocked", detail, msg)
		fmt.Fprintf(stderr, "%s: 锁内**现读**真源读不出来 ⇒ **不给结论**（退码 8 · 真源一个字节不改）：%v\n", progName, msg)
		fmt.Fprintf(stderr, "  真源     : %s\n", gapLedgerPath())
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

// gapAssignFrom —— 「可派状态」闭集（`仍缺` / `已立项` —— 三段链 `仍缺 → 已立项(立项待拍) → 已派(派单)`
// · 缺口 `GAP-20261005-61` §3）：`已立项` = 立项待拍 ⇒ **可派**；`已派` ⇒ 已在飞 · `已解` ⇒ 已闭环 ·
// `回归` ⇒ 机器态 · `不做` ⇒ 已拍死 ⇒ 均不可派。
var gapAssignFrom = []string{gapStOpen, gapStPlanned}

// gapAssignMarker —— 派单登记那一行的**机读标记**（读面按它筛出作业单登记 · 不改 `notes` 的形状）。
const gapAssignMarker = "派单登记 "

var gapAssignFields = []string{"id", "fp", "state", "egg", "allow", "forbid", "occupies",
	"criterion", "deadline", "receipt", "changed"}
var gapAssignListFields = []string{"id", "state", "egg", "allow", "forbid", "occupies",
	"criterion", "deadline", "receipt", "assigned_at", "unit", "module",
	// ★ 本枚（缺口账 `GAP-20260928-186`）：逐行 aging / overdue_days（超时读数进机器面）。
	"aging", "overdue_days"}

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

	// ⑧b 占用件存在性自查（缺口账 `GAP-20260928-182`）：`--occupies` 逐条对 `git ls-files` 现读校验。
	//   口径取「**告警不阻断**」：占用件可能是尚待创建的件（且本族 `occupies` 也见过占位取值），
	//   拒收会误伤 ⇒ 只把对不上的逐条点名到 stderr（**不改退码、不写盘、不阻断**）。
	//   取不到清单（不在 git 仓 / git 起不来 ⇒ `gapTrackedFiles()` 空）⇒ **不报**（「读不到」不当「都没有」）。
	if tracked := gapTrackedFiles(); len(tracked) > 0 {
		set := make(map[string]bool, len(tracked))
		for _, t := range tracked {
			set[t] = true
		}
		for _, occ := range gapAssignOccupies(f.Occupies) {
			if !set[occ] {
				fmt.Fprintf(stderr, "%s: ⚠ 占用件 `%s` 不在入库清单里（`git ls-files` %d 件）—— `--occupies` 逐条现读校验（缺口账 `GAP-20260928-182`）\n",
					progName, occ, len(tracked))
			}
		}
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

	// ★ 本枚（缺口账 `GAP-20261005-26`）：**字段面前置判定** —— 把 `--json` 字段名校验从出口
	//    `selectJSON`（真写**之后**）前移到第一个真写动作之前：坏字段 ⇒ 同 kind 同 detail 同退码，
	//    而真源**一个字节不落、审计一行不落**（改前「先写后校」⇒ 写得成也退得了 2）。
	if rc := gapFrontJSONFields(inv, stderr, append(append([]string{}, gapAssignFields...), "assigned_at")); rc != exitOK {
		return rc
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
	gapReceiptPrint(stdout, msg)
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

// ── ⒢ `zerg gap project <GAP id> --evidence …` / `unproject`（立项登记 · 缺口 `GAP-20261005-61`）──────
//
// 设计出处（唯一真源 · `~/.zerg/state/设计-gap立项正门-20261005.md` §2 乙案 · §3 三段链 · §7 U1）：
//
//	`仍缺 ──project──▶ 已立项（立项待拍） ──assign──▶ 已派（派单）`
//
// 语义：`project` 源 = {`仍缺`, `不做`} → 目标**恒** `已立项`；`unproject` 源 = {`已立项`} → 目标**恒**
// `仍缺`（三段链的回退面 —— §7 U1）。两动作**都不收 `--state`**：目标态由动作名唯一确定 —— 与
// `assign`（源 {`仍缺`, `已立项`} → 目标恒 `已派`）同一条「各锁一个目标态」的纪律（甲案「扩
// `gapStateSettable`」已否 ⇒ 本件**不扩** `set-state` / `bulk` 的闭集，那两条一字不动 ✗）。
//
// 三条纪律（与同族写面逐字同款）：① 只走唯一收口 `gapRewriteOne`（审计先落盘 → 整件重写 → 读回对拍）；
// ② 缺 `--yes` fail-closed `2`（D2 档）· `--dry-run` 零副作用恒 `0`；③ `--evidence` 必填（改态留证）。
// ★ **转入 `已立项` 不取 `solved_at`**（`已立项` 非终态 —— §5 面1）· `solved_kind` 原样带回（`state` 非
// `已解` ⇒ 读面恒回「未分类」）。
//
// 退码（一律引现有表 `exitcodes.go` · 本族**不取新号**）：`0` 真写或幂等「无变化」· `2` 用法错（缺
// id / id 多于一条 / 缺 `--evidence` / 收到 `--state` / **源态不在源闭集** / 点名点在账外）· `8` 真源
// 读不到 / 不在盘上 / 审计写不进 / 真源写不进 / 写回读不对拍。

// gapProjectFrom —— `project` 的源闭集（{`仍缺`, `不做`} → 目标恒 `已立项`）。
var gapProjectFrom = []string{gapStOpen, gapStWontDo}

// gapUnprojectFrom —— `unproject` 的源闭集（{`已立项`} → 目标恒 `仍缺`）= 三段链回退面（§7 U1）。
var gapUnprojectFrom = []string{gapStPlanned}

// gapProjectFields —— `gap project` / `gap unproject` 的 `--json <字段>` 产出集（写面与读面同一份口径）。
var gapProjectFields = []string{"id", "fp", "state", "evidence", "changed", "solved_at"}

// cmdGapProject —— `zerg gap project <GAP id> --evidence …`：立项待拍（源 {仍缺, 不做} → 目标恒 已立项）。
func cmdGapProject(inv *invocation, stdout, stderr io.Writer) int {
	return gapProjectRun(inv, stdout, stderr, "project", gapStPlanned, gapProjectFrom)
}

// cmdGapUnproject —— `zerg gap unproject <GAP id> --evidence …`：撤立项（源 {已立项} → 目标恒 仍缺）。
func cmdGapUnproject(inv *invocation, stdout, stderr io.Writer) int {
	return gapProjectRun(inv, stdout, stderr, "unproject", gapStOpen, gapUnprojectFrom)
}

// gapProjectRun —— `project` / `unproject` 的**共用实现**（两动作只差「目标态」与「源闭集」两格）。
func gapProjectRun(inv *invocation, stdout, stderr io.Writer, cmdName, target string, from []string) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	evidence := strings.TrimSpace(inv.flagVal("--evidence"))

	// ① 用法面（在任何盘面动作之前 —— 与 `set-state` / `assign` 同一条位置）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapProjectFields, ","))
		return exitUsage
	}
	if len(inv.flagVals("--state")) > 0 {
		v := strings.TrimSpace(inv.flagVal("--state"))
		inv.setErr("usage", "state_not_in_shape", "--state 不在 "+cmdName+" 的形状里")
		fmt.Fprintf(stderr, "%s: `gap %s` 不收 `--state %s` —— 本动作落的态**恒为 `%s`**（目标态由动作名唯一确定）\n",
			progName, cmdName, v, target)
		return exitUsage
	}
	if evidence == "" {
		inv.setErr("usage", "missing_required", "缺 --evidence")
		fmt.Fprintf(stderr, "%s: `gap %s` 缺必填旗标：--evidence（**改态必留一句证据**）\n", progName, cmdName)
		fmt.Fprintf(stderr, "用法：zerg gap %s <GAP id> --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]\n", cmdName)
		return exitUsage
	}

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed **rc=2**，计划件走 stderr（判在读真源之前）。
	if !inv.dryRun && !inv.yes {
		ids := gapIDsOf(inv)
		id := "（未给：缺必填）"
		if len(ids) == 1 {
			id = ids[0]
		}
		gapStatePlanBlock(stderr, "缺 `--yes`（D2 档）", cmdName, gapLedgerPath(), -1,
			id, "", "（未读）", target,
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
	idx, rc := gapTargetOne(inv, led, stderr, cmdName)
	if rc != exitOK {
		return rc
	}
	r := led.Recs[idx]

	// ⑤ 幂等：已是目标态**且**证据逐字相同 ⇒ 「无变化」（不写真源、不写审计）
	if r.State == target && r.Evidence == evidence {
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapProjectRow(r, "false"))
		}
		fmt.Fprintf(stdout, "无变化 %s · state=%s（同 id 同态同证据 ⇒ 幂等命中：不写真源、不写审计）\n", r.ID, r.State)
		return exitOK
	}

	// ⑥ 源闭集预检：源态不在 `from` ⇒ 2 并点名当前 state + 逐字印闭集
	if !gapIn(from, r.State) {
		inv.setErr("usage", "state_not_"+cmdName, "当前 state 不在源闭集里")
		fmt.Fprintf(stderr, "%s: `%s` 当前 state = **%s** ⇒ 不可 `%s`（本动作源闭集 = %s）\n",
			progName, r.ID, r.State, cmdName, gapClosedText(from))
		fmt.Fprintf(stderr, "  为什么：`已解` / `回归` ⇒ 已闭环 / 机器态 · 已是 `%s` ⇒ 幂等命中（同态同证据不写）\n", target)
		return exitUsage
	}

	// ⑦ `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapProjectRow(r, "false"))
		}
		gapStatePlanBlock(stdout, "--dry-run", cmdName, led.Path, len(led.Lines),
			r.ID, r.FP, r.State, target,
			"evidence     : "+evidence,
			"solved_at    : 不取（`"+target+"` 非终态 ⇒ 本动作不碰 solved_at / solved_kind）")
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}

	// ⑧ **字段面前置判定**（把 `--json` 坏字段挡在真写之前 —— 同族 `GAP-20261005-26` 口径）
	if rc := gapFrontJSONFields(inv, stderr, gapProjectFields); rc != exitOK {
		return rc
	}

	// ⑨ 真写：`state` → target + `solved_evidence` = evidence（**不碰** `solved_at` / `solved_kind`）
	before := r.State
	r.State = target
	r.Evidence = evidence
	detail, wrc, msg := gapRewriteOne(inv, led, idx, r, cmdName, before, target)
	if wrc != exitOK {
		gapWriteFail(inv, stderr, detail, msg, cmdName)
		return wrc
	}
	// 写后读回自证：`state` 与 `solved_evidence` 都在（读回对拍外再点一次名）
	back, err := readGapLedger()
	if err != nil {
		inv.setErr("failed", "readback_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写后读回真源失败（退码 1）：%v\n", progName, err)
		return exitFail
	}
	bidx := gapFindIdx(back, r.ID)
	if bidx < 0 || back.Recs[bidx].State != target || back.Recs[bidx].Evidence != evidence {
		inv.setErr("failed", "readback_mismatch", "写后读回与要写的不一致")
		fmt.Fprintf(stderr, "%s: 写后读回不一致（退码 1）：state=%s\n", progName, r.ID)
		return exitFail
	}
	inv.changed = boolPtr(true)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapProjectRow(back.Recs[bidx], "true"))
	}
	label := map[string]string{"project": "立项", "unproject": "撤立项"}[cmdName]
	fmt.Fprintf(stdout, "已%s %s · state %s → %s（真源 %d 行不变 · 只重写目标那一行 · 审计已落 1 行）\n",
		label, r.ID, before, target, len(led.Lines))
	fmt.Fprintf(stdout, "  evidence  : %s\n", evidence)
	fmt.Fprintf(stdout, "  solved_at : %s（本动作不碰 —— `%s` 非终态）\n", back.Recs[bidx].SolvedAt, target)
	gapReceiptPrint(stdout, msg)
	return exitOK
}

// gapProjectRow —— `--json` 那一行的逐格取值（写面与读面共用一份口径 · `solved_at` 只读回显）。
func gapProjectRow(r gapRecord, changed string) map[string]string {
	return map[string]string{
		"id": r.ID, "fp": r.FP, "state": r.State, "evidence": r.Evidence,
		"changed": changed, "solved_at": r.SolvedAt,
	}
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

	// ★ 缺口 `GAP-20260928-195`（本枚）：**批量销案同走共用两态闸** —— `--state 已解` 时逐条判
	//    （判决与单条面同一处 `gapTwoStateGate`，✗ 不另造口径）；任一条缺读数（新账且未带两态）
	//    ⇒ **整批拒**（退 2 · 一字不写：真源一个字节未改、审计一行未落）。`--state 仍缺` / `不做`
	//    与非销案面 ⇒ 闸判「不适用」，行为一字不动。
	if stateWant == gapStSolved {
		for i := range rows {
			if rows[i].Fail != "" {
				continue
			}
			if rc := gapTwoStateGate(inv, rows[i].ID, stateWant, "bulk set-state", led.Path, len(led.Lines), stderr); rc != exitOK {
				fmt.Fprintf(stderr, "  批量面   : 任一条销案缺两态读数 ⇒ **整批拒**（退码 2 · **一字不写**：真源一个字节未改、审计一行未落）\n")
				return rc
			}
		}
	}

	// ★ 本枚（缺口账 `GAP-20261005-26`）：**字段面前置判定** —— 把 `--json` 字段名校验从出口
	//    `selectJSON`（真写**之后**）前移到第一个真写动作之前：坏字段 ⇒ 同 kind 同 detail 同退码，
	//    而真源**一个字节不落、审计一行不落**（改前「先写后校」⇒ 写得成也退得了 2）。
	if rc := gapFrontJSONFields(inv, stderr, gapBulkSetStateFields); rc != exitOK {
		return rc
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
		// ★ 同单条 `set-state` 那一格（见本件头 ★）：`solved_at` 只在**真变入终态**时取 ——
		//   已是 `已解` / `不做` 的条再补写（补证据）一律**继承旧值**、不重写（历史时刻不改写）。
		if (stateWant == gapStSolved || stateWant == gapStWontDo) && before != stateWant {
			rec.SolvedAt = gapNow()
		}
		detail, rc, msg := gapRewriteOne(inv, fresh, idx, rec, "bulk set-state", before, stateWant)
		if rc != exitOK {
			gapWriteFail(inv, stderr, detail, msg, "bulk set-state")
			r.Fail = fmt.Sprintf("真写失败（%s）⇒ 当场停手：本条与后条一律未改", detail)
			failed, failRC = 1, rc
			break
		}
		if !inv.jsonGiven {
			gapReceiptPrint(stdout, msg)
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

// ── ⒡ `zerg gap verify-one <GAP id>`（跑判据即写 · 批4 第一片 · 2026-09-28）──────────────────────
//
// 设计出处（唯一真源 · `设计-缺口账与自进化-v2.0-20260928.md`）：§11.4 批4（回归巡检：跑 `verify_cmd`
// → 记 `last_verify_rc`）· §11.2 批2 D1/D2（`last_verified_at` 覆盖率 · `verify_tier` 分布可读）。
//
// 病：`gap verify` 是**批量面**（点名多条 / `--all`），且只有 `--yes` 那一态写 `last_verified_at`
// ⇒ 「跑判据即写」没落地（现读真账 918 条里该格**仅 1 条**）。本面 = 一条一次，把三格写回该条。
//
// 四条硬纪律（逐条可对拍）：
//
//	① **不另开写路**：只调本族唯一收口 `gapRewriteOne`（审计先落盘 → 整件重写 → 读回对拍）——
//	   未动的行**逐字节照原样**写回，**只重写目标那一行**。**本面不改 `state`**（改态另有其面）。
//	② `--dry-run` 只出计划件（**不跑判据**、不写账 · 恒 0）· 缺 `--yes` 真写 ⇒ fail-closed **2**。
//	③ 该条**无 `verify_cmd`**（空串 ⇒ 分级=无）⇒ **2 并点名**（不当绿）。
//	④ 判据带 **shell 元字符**（管道 `|` / 重定向 `>` `<` / 分号 `;` / 与或 `&` / 反引号 / `$` / 括号）
//	   ⇒ **一律拒跑并点名**（不当绿也不当红）。只跑账里那一格命令本身，**不拼 shell 串**。
//
// 退码（一律引现有表 `exitcodes.go` · 本族**不取新号**）：0 跑判据过（`verify_cmd` rc=0）或占位落账 ·
// 1 判据跑出来**非 0**（判红 · 三格仍已写回）· 2 用法错（缺 id / id 多于一条 / 账内没有 / 无 `verify_cmd` /
// 判据带 shell 元字符 / 判据是写面 / 判据不可跑 / 缺 `--yes`）· 8 真源读不到 / 审计写不进 / 真源写不进 /
// 写回读不对拍。
//
// ★ 两条取定（设计稿未钉死 · 回执如实点名）：
//   · 命中第 ④ 条「元字符」与写面闸、判据不可跑 —— 都归**用法面 2**（「改用法后可重试」），
//     且**一个字节都不写**（真跑会动盘/会跑危险命令 ⇒ 不当绿也不当红）。
//   · 占位判据**不执行**（占位是「真判据还没建」的标记、不是可跑命令）：落 `last_verify_rc=n/a`，
//     命令本身退 **0**（写回成功）。

const (
	gapVerifyTierReal        = "真判据"
	gapVerifyTierPlaceholder = "占位"
	gapVerifyTierNone        = "无"
)

// gapVerifyNoneRC —— 占位判据不跑时 `last_verify_rc` 落的常量（表示「这一格压根没跑」）。
const gapVerifyNoneRC = "n/a"

var gapVerifyOneFields = []string{"id", "fp", "judge", "verify_tier", "last_verify_rc", "last_verified_at", "changed"}

// gapVerifyOneTier —— 判据分级（三值 · 本面唯一分档处）：含「占位」字样 ⇒ 占位；去空白空串 ⇒ 无；其余 ⇒ 真判据。
func gapVerifyOneTier(cmd string) string {
	if strings.TrimSpace(cmd) == "" {
		return gapVerifyTierNone
	}
	if strings.Contains(cmd, "占位") {
		return gapVerifyTierPlaceholder
	}
	return gapVerifyTierReal
}

// gapVerifyOneMetaHit —— 判据里的 shell 元字符（管道/重定向/分号/与或/反引号/`$`/括号/换行）。
// 命中 ⇒ 返回那一个字符（非空 = 拒跑）；纯命令词（`zerg …` 那种）不含这些 ⇒ 空串。
func gapVerifyOneMetaHit(cmd string) string {
	for _, r := range cmd {
		switch r {
		case '|', '&', ';', '<', '>', '`', '$', '(', ')', '\n', '\r':
			return string(r)
		}
	}
	return ""
}

// gapVerifyOneRun —— 跑一条**真判据**：解析走命令树（`gapJudgeArgv`），**只收 stdout**（关键读数行取它）。
// 解析不过 ⇒ why 非空（调用方译成 2）。
func gapVerifyOneRun(cmdStr string) (int, string, string) {
	args, why := gapJudgeArgv(cmdStr)
	if why != "" {
		rc, out := 0, ""
		return rc, out, why
	}
	out := &gapCapWriter{max: 8192}
	errw := &gapCapWriter{max: 1024}
	rc := run(args, out, errw)
	return rc, strings.TrimSpace(out.String()), ""
}

// gapVerifyOneReading —— 「关键读数行」：stdout 首行**前 120 显示宽** + 该段字节数。
func gapVerifyOneReading(rc int, out string) string {
	if out == "" {
		return fmt.Sprintf("rc=%d · （无 stdout）· 字节 0", rc)
	}
	line := out
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	runes := []rune(line)
	if len(runes) > 120 {
		runes = runes[:120]
	}
	seg := string(runes)
	return fmt.Sprintf("rc=%d · 首行=%q · 首行字节 %d · stdout 字节 %d", rc, seg, len([]byte(seg)), len([]byte(out)))
}

// gapVerifyOnePlanBlock —— 计划件（`--dry-run` 走 stdout · 缺 `--yes` 走 stderr）。
// `lines < 0` ⇒ 真源**未读**（缺 `--yes` 那一档：一个盘面动作都还没做）。
func gapVerifyOnePlanBlock(w io.Writer, title, ledgerPath string, lines int, id, judge, tier string) {
	fmt.Fprintf(w, "计划件（跑判据即写 · %s · 零副作用 —— **未跑判据**、未改真源、未写审计）\n", title)
	if lines < 0 {
		fmt.Fprintf(w, "  真源     : %s（未读 —— 缺 `--yes` ⇒ 不执行）\n", ledgerPath)
	} else {
		fmt.Fprintf(w, "  真源     : %s（现有 %d 行）\n", ledgerPath, lines)
	}
	fmt.Fprintf(w, "  目标     : %s\n", id)
	fmt.Fprintf(w, "  判据分级 : %s\n", tier)
	fmt.Fprintf(w, "  将跑     : %s\n", judge)
	if tier == gapVerifyTierPlaceholder {
		fmt.Fprintf(w, "  ⚠ 占位判据 : **不执行**（占位 = 真判据还没建 · 落 `last_verify_rc=%s`）\n", gapVerifyNoneRC)
	}
	fmt.Fprintf(w, "  要落的键 : last_verified_at（现取时刻 · RFC3339Nano）· last_verify_rc（真退码）· verify_tier（%s）\n", tier)
	fmt.Fprintf(w, "  审计     : 计划写一行 `event=%s` + `gap_cmd=verify-one`（这一态**不写**）\n", gapEventName)
}

// gapVerifyOneEmitJSON —— `--json` 走同族信封面（一格一行 · 与表尾**同一份数据**）。
func gapVerifyOneEmitJSON(stdout, stderr io.Writer, inv *invocation, id, fp, judge, tier, rc, at, changed string) int {
	return selectJSON(stdout, stderr, inv, inv.path, inv.fields, map[string]string{
		"id": id, "fp": fp, "judge": judge, "verify_tier": tier,
		"last_verify_rc": rc, "last_verified_at": at, "changed": changed,
	})
}

func cmdGapVerifyOne(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}

	// ① 用法面（在任何盘面动作之前 —— 与同族写面同一条位置）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapVerifyOneFields, ","))
		return exitUsage
	}
	if len(inv.flagVals("--state")) > 0 {
		v := strings.TrimSpace(inv.flagVal("--state"))
		inv.setErr("usage", "state_not_in_shape", "--state 不在 verify-one 的形状里")
		fmt.Fprintf(stderr, "%s: `gap verify-one` 不收 `--state %s` —— 本面**只写判据三格、不改 `state`**\n", progName, v)
		fmt.Fprintf(stderr, "  改态 : `zerg gap verify <GAP id>…`（机器态）或 `zerg gap set-state <GAP id> --state <…> --evidence <…>`（人的口径）\n")
		return exitUsage
	}

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed **rc=2**，计划件走 stderr（**判在读真源之前**）。
	if !inv.dryRun && !inv.yes {
		ids := gapIDsOf(inv)
		id := "（未给：缺必填）"
		if len(ids) == 1 {
			id = ids[0]
		}
		gapVerifyOnePlanBlock(stderr, "缺 `--yes`（D2 档）", gapLedgerPath(), -1, id, "（未读）", "（未读）")
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
	idx, rc := gapTargetOne(inv, led, stderr, "verify-one")
	if rc != exitOK {
		return rc
	}
	r := led.Recs[idx]
	tier := gapVerifyOneTier(r.VerifyCmd)
	meta := func() {
		inv.metaAddStr("query", "verify-one")
		inv.metaAddStr("query_ts", gapNow())
		inv.metaAddStr("ledger_sha16", gapLsLedgerSHA16(led.Path))
		inv.metaAddStr("verify_tier", tier)
	}

	// ⑤ 无判据（空串 ⇒ 分级=无）⇒ 2 并点名（**不当绿** · 一个字节都不写）
	if tier == gapVerifyTierNone {
		inv.setErr("usage", "verify_cmd_absent", "该条无 verify_cmd")
		fmt.Fprintf(stderr, "%s: `%s` 的 `verify_cmd` 是**空的**（判据分级 = **%s**）⇒ 没有可跑的判据\n", progName, r.ID, gapVerifyTierNone)
		fmt.Fprintf(stderr, "  ⇒ 不跑、不写账（退码 2 —— **不当绿**；空判据不算「验过」）\n")
		fmt.Fprintf(stderr, "  修法 : 先给它一条可机检的判据（`zerg gap add … --verify-cmd <命令>` 或补账）\n")
		return exitUsage
	}

	// ⑥ 口径闸（只对**真判据**）：shell 元字符 / 写面（危险档且没带 --dry-run）⇒ 拒跑并点名（2 · 不写账）
	if tier == gapVerifyTierReal {
		if m := gapVerifyOneMetaHit(r.VerifyCmd); m != "" {
			inv.setErr("usage", "verify_cmd_shell_meta", "判据含 shell 元字符")
			fmt.Fprintf(stderr, "%s: %s 的判据含 shell 元字符 %q —— **拒跑**（本面只跑「那一格命令本身」· 不拼 shell 串）\n",
				progName, r.ID, m)
			fmt.Fprintf(stderr, "  判据 : %s\n", r.VerifyCmd)
			fmt.Fprintf(stderr, "  ⇒ 既**不当绿也不当红**、一个字节都不写（退码 2）\n")
			return exitUsage
		}
		if why := gapWriteFaceWhy(r.VerifyCmd); why != "" {
			inv.setErr("usage", "verify_cmd_write_face", why)
			fmt.Fprintf(stderr, "%s: %s 的判据是**写面**（真跑会真写盘）：%s\n", progName, r.ID, why)
			fmt.Fprintf(stderr, "  ⇒ 拒跑、不给结论、一个字节都不写（退码 2；要拿写面当判据就写它的 `--dry-run` 那一态）\n")
			return exitUsage
		}
	}

	// ⑦ `--dry-run`：只出计划件（**不跑判据** · stdout · rc=0 · 零副作用）
	if inv.dryRun {
		inv.changed = boolPtr(false)
		meta()
		if inv.jsonGiven {
			return gapVerifyOneEmitJSON(stdout, stderr, inv, r.ID, r.FP, r.VerifyCmd, tier, "", "", "false")
		}
		gapVerifyOnePlanBlock(stdout, "--dry-run", led.Path, len(led.Lines), r.ID, r.VerifyCmd, tier)
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— **未跑判据**、未改真源、未写审计）\n")
		return exitOK
	}

	// ⑧ 跑判据（占位**不跑**）
	judgeRC := 0
	reading := ""
	if tier == gapVerifyTierReal {
		c, out, why := gapVerifyOneRun(r.VerifyCmd)
		if why != "" {
			inv.setErr("usage", "verify_cmd_unresolved", why)
			fmt.Fprintf(stderr, "%s: %s 的判据不可跑：%s\n", progName, r.ID, why)
			fmt.Fprintf(stderr, "  ⇒ 不给结论（退码 2）；真源**一个字节未改**\n")
			return exitUsage
		}
		judgeRC, reading = c, gapVerifyOneReading(c, out)
	} else {
		reading = fmt.Sprintf("rc=%s · 占位判据**未执行**", gapVerifyNoneRC)
	}

	// ★ 本枚（缺口账 `GAP-20261005-26`）：**字段面前置判定** —— 把 `--json` 字段名校验从出口
	//    `selectJSON`（真写**之后**）前移到第一个真写动作之前：坏字段 ⇒ 同 kind 同 detail 同退码，
	//    而真源**一个字节不落、审计一行不落**（改前「先写后校」⇒ 写得成也退得了 2）。
	if rc := gapFrontJSONFields(inv, stderr, gapVerifyOneFields); rc != exitOK {
		return rc
	}

	// ⑨ 真写：三格（`state` 逐字不动 · 走本族唯一收口 `gapRewriteOne`）
	now := gapNow()
	before := r.State
	r.Verified = now
	r.VerifyTier = tier
	if tier == gapVerifyTierReal {
		r.VerifyRC = strconv.Itoa(judgeRC)
	} else {
		r.VerifyRC = gapVerifyNoneRC
	}
	if r.State != before {
		inv.setErr("failed", "state_mutated", "verify-one 这一路改了 state")
		fmt.Fprintf(stderr, "%s: 内部对拍破了：本面改了 `state` ⇒ 不给结论（退码 1）\n", progName)
		return exitFail
	}
	detail, wrc, msg := gapRewriteOne(inv, led, idx, r, "verify-one", before, before)
	if wrc != exitOK {
		gapWriteFail(inv, stderr, detail, msg, "verify-one")
		return wrc
	}

	cmdRC := exitOK
	if tier == gapVerifyTierReal && judgeRC != 0 {
		cmdRC = exitFail // 判红：三格已写回，但这一跑**不是绿**
	}
	inv.changed = boolPtr(true)
	meta()
	if inv.jsonGiven {
		if rc := gapVerifyOneEmitJSON(stdout, stderr, inv, r.ID, r.FP, r.VerifyCmd, tier, r.VerifyRC, now, "true"); rc != exitOK {
			return rc
		}
		return cmdRC
	}
	fmt.Fprintf(stdout, "已验一条 %s · 判据分级=%s（真源 %d 行不变 · **只重写目标那一行** · 审计已落 1 行）\n",
		r.ID, tier, len(led.Lines))
	fmt.Fprintf(stdout, "  last_verified_at : %s\n", now)
	fmt.Fprintf(stdout, "  last_verify_rc   : %s\n", r.VerifyRC)
	fmt.Fprintf(stdout, "  verify_tier      : %s\n", tier)
	fmt.Fprintf(stdout, "  关键读数行       : %s\n", reading)
	gapReceiptPrint(stdout, msg)
	return cmdRC
}

// ── ⒢ `zerg gap assign ls [--egg <卵号>]`（只读面 · 批3 第一片）────────────────────────────────

// ── ⒥ `zerg gap set-unit <GAP id> --unit <件路径>`（**改件面三键的正门** · 2026-10-09）────────────
//
// 病（缺口账 `GAP-20261008-06`/`-07` 逐字：「改 unit / 改 want 均无正门」）：缺口账的件面三键
// （`unit` / `module` / `unit_source`）此前只有两个写口 —— `gap add`（新落行那一刻算）与
// `gap backfill-unit`（存量一次性只加三键）⇒ **改某一条已落账的 unit** 无处可走。
//
// 实需：退役后旧 `unit` 指向已退场件（例：`GAP-20261009-03` 的 `scripts/evals/compare-wall-argv.py`
// 已离盘）⇒ 没有正门时只能往门75 存量豁免表加行（**该表只增不删**）绕过。有本正门后，这类条可以
// **归位**（改指还在盘的 Go 半）而不是永久豁免。
//
// 形制照同族五写面（`set-state` / `note` / `project` / `assign` / `verify-one`）逐字同款：
//
//	① 走族内唯一收口 `gapRewriteOne`（只重写目标那一行 · 其余行逐字节不变 · 行序不变 · 末行换行不变）；
//	② `--dry-run` 恒 0 零副作用 · 缺 `--yes` fail-closed 2（D2 档）；
//	③ `--evidence` 必填（**改件留证** —— 落成该条 `notes` 末尾一条注：`<时刻> · <谁>：改件面 …`）；
//	   审计进 `edit_audit.jsonl`（**写不进就不写真源** —— 审计先落盘）；
//	④ 写回执回显「改前 → 改后」的 `unit` / `module` / `unit_source` 三键。
//
// ★ `unit` 存在性预检（本面取定 · 帮助里写清 · ✗ 不许默默放行）：`--unit` 是**仓内相对路径**时逐条对
//   `git ls-files` 现读校验 —— **不在仓内 ⇒ 默认拒 2**（一个字节都不写），要确认改指一个尚待创建的件
//   就得**显式带 `--allow-missing-unit`**（因由写进 `--evidence`）。兜底档（`无件(命令面)` / `无件` /
//   `拟(x)`）不受此闸；**取不到清单**（不在 git 仓）⇒ **不判**（读不到不当「都不存在」）。
//
// 退码（一律引现有表 `exitcodes.go` · 本族**不取新号**）：
//
//	0 真写或幂等「无变化」· 1 真源那一行序列化不过 · 2 用法错（缺 id / id 多于一条 / 账内没有 /
//	缺 `--unit` / 缺 `--evidence` / 收到 `--state` / `--unit-source` 闭集外 / `--unit` 不在仓内且
//	未带 `--allow-missing-unit` / 缺 `--yes`）· 8 真源读不到 / 审计写不进 / 真源写不进 / 写回读不对拍。

// gapUnitSourceClosed —— `--unit-source` 的**闭集**（与 `gap add` 抽取面同源两值：`auto` / `hand`）。
var gapUnitSourceClosed = []string{gapUnitAuto, gapUnitHand}

// gapSetUnitFields —— `--json <字段>` 的可取字段（写面与读面同一份口径）。
var gapSetUnitFields = []string{"id", "fp", "unit", "module", "unit_source", "changed"}

// gapSetUnitSourceDefault —— `--unit-source` 缺省取值：兜底件（`无件(命令面)` / `无件` / `拟(x)`）⇒
// `hand`；仓内相对路径 ⇒ `auto`（判据与 `gapUnitIsNoFileSel` 同源）。
func gapSetUnitSourceDefault(unit string) string {
	if gapUnitIsNoFileSel(unit) || strings.HasPrefix(unit, "拟(") {
		return gapUnitHand
	}
	return gapUnitAuto
}

// gapSetUnitModuleDefault —— `--module` 缺省：由 `unit` 的目录前缀推导（逐字复用 `gapModuleOfUnit`）。
func gapSetUnitModuleDefault(unit string) string { return gapModuleOfUnit(unit) }

// gapSetUnitIsFallback —— 该 unit 是不是「非仓内路径」的兜底档（降预检不判、按 `hand` 缺省）。
func gapSetUnitIsFallback(unit string) bool {
	return gapUnitIsNoFileSel(unit) || strings.HasPrefix(unit, "拟(")
}

// gapSetUnitExistsInRepo —— `unit` 是不是仓内入库件（逐条对 `git ls-files` 现读）。
// 返回 `(found, readable)`：`readable=false` ⇒ 取不到清单（不在 git 仓 / git 起不来）—— 与
// 「真不在盘」分家（✗ 读不到不当「都不存在」）。
func gapSetUnitExistsInRepo(unit string) (found, readable bool) {
	tracked := gapTrackedFiles()
	if len(tracked) == 0 {
		return false, false
	}
	for _, t := range tracked {
		if t == unit {
			return true, true
		}
	}
	return false, true
}

// gapSetUnitNote —— 改件留证那一行（落进该条 `notes` 末尾 · 形态照同族注 `<时刻> · <谁>：<文本>`）。
func gapSetUnitNote(by, beforeUnit, beforeModule, beforeSource, unit, module, unitSource, evidence string) string {
	return fmt.Sprintf("%s · %s%s改件面 unit: %s → %s · module: %s → %s · unit_source: %s → %s · 证据：%s",
		gapNow(), by, gapNoteSep,
		orDash(beforeUnit), unit, orDash(beforeModule), module, orDash(beforeSource), unitSource, evidence)
}

// gapSetUnitRow —— `--json` 那一行的逐格取值（写面与读面同一份口径）。
func gapSetUnitRow(r gapRecord, changed string) map[string]string {
	return map[string]string{
		"id": r.ID, "fp": r.FP, "unit": r.Unit, "module": r.Module,
		"unit_source": r.UnitSource, "changed": changed,
	}
}

// gapSetUnitPlanBlock —— `set-unit` 的计划件（`--dry-run` 走 stdout · 缺 `--yes` 走 stderr）。
// `lines < 0` ⇒ 真源**未读**（缺 `--yes` 那一档：一个盘面动作都还没做）。照 `gapStatePlanBlock` 的
// 形状，但把「要落的键」换成件面三键（本面**不碰** `state`）。
func gapSetUnitPlanBlock(w io.Writer, title, ledgerPath string, lines int, id, fp, unit, module, unitSource, evidence string) {
	fmt.Fprintf(w, "计划件（改件面三键 · %s · 零副作用 —— 未改真源、未写审计）\n", title)
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
	fmt.Fprintf(w, "    unit         : → %s\n", unit)
	fmt.Fprintf(w, "    module       : → %s\n", module)
	fmt.Fprintf(w, "    unit_source  : → %s\n", unitSource)
	fmt.Fprintf(w, "    evidence     : %s（落成 `notes` 末尾一条注）\n", evidence)
	fmt.Fprintf(w, "  审计     : 计划写一行 `event=%s` + `gap_cmd=set-unit`（这一态**不写**）\n", gapEventName)
}

// cmdGapSetUnit —— `zerg gap set-unit <GAP id> --unit <件路径> …`：改件面三键（写面 · D2 档）。
func cmdGapSetUnit(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	unitWant := strings.TrimSpace(inv.flagVal("--unit"))
	moduleWant := strings.TrimSpace(inv.flagVal("--module"))
	unitSourceWant := strings.TrimSpace(inv.flagVal("--unit-source"))
	evidence := strings.TrimSpace(inv.flagVal("--evidence"))

	// ① 用法面（在任何盘面动作之前 —— 与同族写面同一条位置）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapSetUnitFields, ","))
		return exitUsage
	}
	if len(inv.flagVals("--state")) > 0 {
		v := strings.TrimSpace(inv.flagVal("--state"))
		inv.setErr("usage", "state_not_in_shape", "--state 不在 set-unit 的形状里")
		fmt.Fprintf(stderr, "%s: `gap set-unit` 不收 `--state %s` —— 本面**只改件面三键、不改 `state`**\n", progName, v)
		fmt.Fprintf(stderr, "  改态 : `zerg gap set-state <GAP id> --state <…> --evidence <…>`（人的口径）\n")
		return exitUsage
	}
	if unitWant == "" {
		inv.setErr("usage", "missing_required", "缺 --unit")
		fmt.Fprintf(stderr, "%s: `gap set-unit` 缺必填旗标：--unit（要改指的那一件 · 仓内相对路径）\n", progName)
		fmt.Fprintf(stderr, "用法：zerg gap set-unit <GAP id> --unit <件路径> [--module <模块前缀>] [--unit-source <auto|hand>] [--allow-missing-unit] --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]\n")
		return exitUsage
	}
	// ★ 「裸给不降级」：给了 `--unit-source` 却不给值 ⇒ 用法错 2（与同族 `--retract` / `--solved-kind` 同款）。
	if inv.hasFlag("--unit-source") && unitSourceWant == "" {
		inv.setErr("usage", "unit_source_missing_value", "--unit-source 没给值")
		fmt.Fprintf(stderr, "%s: `--unit-source` 要给一个值（只认 %s）—— 裸给不降级（用法错 2）\n",
			progName, gapClosedText(gapUnitSourceClosed))
		return exitUsage
	}
	if unitSourceWant != "" && !gapIn(gapUnitSourceClosed, unitSourceWant) {
		inv.setErr("usage", "bad_unit_source", "unit_source 值不在闭集里")
		fmt.Fprintf(stderr, "%s: `--unit-source %s` 不在闭集里 —— 只认 %s\n",
			progName, unitSourceWant, gapClosedText(gapUnitSourceClosed))
		return exitUsage
	}
	if evidence == "" {
		inv.setErr("usage", "missing_required", "缺 --evidence")
		fmt.Fprintf(stderr, "%s: `gap set-unit` 缺必填旗标：--evidence（**改件必留一句证据** —— 与 `add` 的「留证据」同一条纪律）\n", progName)
		return exitUsage
	}

	// 缺省推导（module 由 unit 目录前缀推；unit_source 由 unit 形态推 —— 与 `gap add` 抽取面同源）。
	module := moduleWant
	if module == "" {
		module = gapSetUnitModuleDefault(unitWant)
	}
	unitSource := unitSourceWant
	if unitSource == "" {
		unitSource = gapSetUnitSourceDefault(unitWant)
	}

	// ② 缺 `--yes`（且非 `--dry-run`）：fail-closed rc=2，计划件走 stderr（判在读真源之前）。
	if !inv.dryRun && !inv.yes {
		ids := gapIDsOf(inv)
		id := "（未给：缺必填）"
		if len(ids) == 1 {
			id = ids[0]
		}
		gapSetUnitPlanBlock(stderr, "缺 `--yes`（D2 档）", gapLedgerPath(), -1, id, "", unitWant, module, unitSource, evidence)
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
	idx, rc := gapTargetOne(inv, led, stderr, "set-unit")
	if rc != exitOK {
		return rc
	}
	r := led.Recs[idx]

	// ④a `--unit` 存在性预检（仓内相对路径 · 兜底档不受此闸 · 读不到清单 ⇒ 不判）
	if !gapSetUnitIsFallback(unitWant) {
		found, readable := gapSetUnitExistsInRepo(unitWant)
		switch {
		case readable && !found && !inv.hasFlag("--allow-missing-unit"):
			inv.setErr("usage", "unit_not_in_repo", "unit 不在仓内入库件清单里")
			fmt.Fprintf(stderr, "%s: `--unit %s` **不在仓内**（`git ls-files` 现读对不上）—— 本面默认**拒写**（一个字节都不写）\n",
				progName, unitWant)
			fmt.Fprintf(stderr, "  要确认改指一个尚待创建的件 ⇒ 显式带 `--allow-missing-unit`（并把因由写进 `--evidence`）\n")
			fmt.Fprintf(stderr, "  ⚠ 默认拒 = 防手滑把 unit 指到不存在的路径（✗ 不许默默放行）；`无件(命令面)` / `拟(x)` 兜底档不受此闸\n")
			return exitUsage
		case readable && !found:
			fmt.Fprintf(stderr, "%s: ⚠ `--unit %s` 不在仓内 —— 显式 `--allow-missing-unit` 放行（因由应已在 `--evidence` 留痕）\n",
				progName, unitWant)
		case !readable:
			fmt.Fprintf(stderr, "%s: ⚠ 取不到入库件清单（不在 git 仓 / git 起不来）—— 本发**不判** `unit` 存在性（读不到不当「都不存在」）\n", progName)
		}
	}

	// ⑤ 幂等：同 id 同三键 ⇒ 「无变化」（不写真源、不写审计、**不落注**）
	if r.Unit == unitWant && r.Module == module && r.UnitSource == unitSource {
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapSetUnitRow(r, "false"))
		}
		fmt.Fprintf(stdout, "无变化 %s · unit=%s（同 id 同三键 ⇒ 幂等命中：不写真源、不写审计）\n", r.ID, r.Unit)
		return exitOK
	}

	// ⑥ `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		inv.changed = boolPtr(false)
		would := gapRecord{ID: r.ID, FP: r.FP, Unit: unitWant, Module: module, UnitSource: unitSource}
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapSetUnitRow(would, "false"))
		}
		gapSetUnitPlanBlock(stdout, "--dry-run", led.Path, len(led.Lines), r.ID, r.FP, unitWant, module, unitSource, evidence)
		fmt.Fprintf(stdout, "  改前     : unit=%s · module=%s · unit_source=%s\n", orDash(r.Unit), orDash(r.Module), orDash(r.UnitSource))
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}

	// ★ 本枚（缺口账 `GAP-20261005-26`）：**字段面前置判定** —— 把 `--json` 字段名校验从出口
	//    `selectJSON`（真写**之后**）前移到第一个真写动作之前：坏字段 ⇒ 同 kind 同 detail 同退码，
	//    而真源**一个字节不落、审计一行不落**（改前「先写后校」⇒ 写得成也退得了 2）。
	if rc := gapFrontJSONFields(inv, stderr, gapSetUnitFields); rc != exitOK {
		return rc
	}

	// ⑦ 真写：三键（`unit` / `module` / `unit_source`）+ `notes` 末尾追加一条**改件留证**注；
	//    `state` 逐字不动（改前改后现算对拍，破 ⇒ 不给结论）。走本族唯一收口 `gapRewriteOne`。
	beforeUnit, beforeModule, beforeSource := r.Unit, r.Module, r.UnitSource
	before := r.State
	r.Unit = unitWant
	r.Module = module
	r.UnitSource = unitSource
	r.Notes = append(append([]string{}, r.Notes...),
		gapSetUnitNote(gapByOf(inv), beforeUnit, beforeModule, beforeSource, unitWant, module, unitSource, evidence))
	if r.State != before {
		inv.setErr("failed", "state_mutated", "set-unit 这一路改了 state")
		fmt.Fprintf(stderr, "%s: 内部对拍破了：本面改了 `state`（%s → %s）⇒ 不给结论（退码 1）\n", progName, before, r.State)
		return exitFail
	}
	detail, wrc, msg := gapRewriteOne(inv, led, idx, r, "set-unit", before, before)
	if wrc != exitOK {
		gapWriteFail(inv, stderr, detail, msg, "set-unit")
		return wrc
	}
	// 写后读回自证：三键都在（读回对拍外再点一次名）
	back, err := readGapLedger()
	if err != nil {
		inv.setErr("failed", "readback_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写后读回真源失败（退码 1）：%v\n", progName, err)
		return exitFail
	}
	bidx := gapFindIdx(back, r.ID)
	if bidx < 0 || back.Recs[bidx].Unit != unitWant || back.Recs[bidx].Module != module || back.Recs[bidx].UnitSource != unitSource {
		inv.setErr("failed", "readback_mismatch", "写后读回与要写的不一致")
		fmt.Fprintf(stderr, "%s: 写后读回不一致（退码 1）\n", progName)
		return exitFail
	}
	inv.changed = boolPtr(true)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapSetUnitRow(back.Recs[bidx], "true"))
	}
	fmt.Fprintf(stdout, "已改件面 %s · unit %s → %s（真源 %d 行不变 · 只重写目标那一行 · 审计已落 1 行）\n",
		r.ID, orDash(beforeUnit), back.Recs[bidx].Unit, len(led.Lines))
	fmt.Fprintf(stdout, "  module      : %s → %s\n", orDash(beforeModule), back.Recs[bidx].Module)
	fmt.Fprintf(stdout, "  unit_source : %s → %s\n", orDash(beforeSource), back.Recs[bidx].UnitSource)
	fmt.Fprintf(stdout, "  evidence    : %s（已落成 `notes` 末尾一条注）\n", evidence)
	gapReceiptPrint(stdout, msg)
	return exitOK
}

// ── `gap set-want`（缺口 `GAP-20261008-07` · 2026-10-19）────────────────────────────────────
//
// 病（现读实测）：缺口账的 `want_family` / `want_action` 两格**没有任何 CLI 写面** ——
// 同族五写面里 `set-state` 只改 `state` 那一维、`note` 只追加 `notes`、`set-unit` 改件面三键、
// `judge set` 改判据两键、`assign` 走派单；**want 换方向只能手改盘上真源**（绕过审计 / 绕过账写锁 /
// 绕过一切校验），改完也无人知道「谁在什么时候把想要的动作从 A 换成了 B」。
//
// 治法 = 本动作：**只改 want 三格**（`want_family` / `want_action` / `want_argv`），形制照同族
// `gap set-unit`（那是「只改件面三键」的先例，逐条对齐、✗ 不另立一套纪律）：
//
//	① 写前判 = 同族唯一取值判据 `gapWantTargetWhy`（`family_gap.go`）：族名**闭集**
//	   （命令树顶层族 ∪ 账内现用 want_family 原值）· 动作名**形态闭集**（单一 token）；越界 ⇒ 2 并**点名越界值**；
//
// ★ 旗标名取 `--want-family` / `--want-action`（**不取** `--family` / `--action`）：旗标注册表是
// **唯一一张**（`main.go` 的 `valueFlagName` 一族）—— `--family` / `--action` 不在那张表里，真给会被
// 解析器判「未知旗标」（本件实测：rc=2 `未知旗标 "--family"`）；要上户口就得改 `main.go`，而本单边界
// 划到「✗ 不动 `main.go`」。取同族 `gap add` 已在用的那两枚 ⇒ 零新旗标、两处名字也对得上。
//
//	② `--dry-run` 恒 0（计划件走 stdout · 零副作用）；缺 `--yes` **fail-closed 2**（计划件走 stderr ·
//	   判在**读真源之前** ⇒ 一行都不读、一行都不写）；
//	③ 写前留痕：`--evidence` 必填，落成该条 `notes` 末尾一条注（`<时刻> · <谁>：改 want … 证据：…`），
//	   与 `set-unit` / `judge set` 的「改账必留一句证据」同一条纪律；审计行由 `gapRewriteOne` **先**落
//	   （`event=gap_ledger_written` + `gap_cmd=set-want` + 账件前后 sha256）⇒ 审计写不进就不写真源；
//	④ 只走族内唯一收口 `gapRewriteOne`（锁内现读 → 只重写目标那一行 → 写后读回对拍）；
//	⑤ 同 id 同族同动作（且未改 `--want-argv`）⇒ 「无变化」0（不写真源、不写审计、不落注）；
//	⑥ `state` **一个字节不动**（改前改后现算对拍，破 ⇒ 退 1 不给结论）；收到 `--state` / `--text` ⇒ 2。

// gapSetWantFields —— `--json <字段>` 的可取字段（写面与读面同一份口径）。
var gapSetWantFields = []string{"id", "fp", "want_family", "want_action", "want_argv", "changed"}

// gapSetWantNote —— 落进 `notes` 的改口径留痕（形态与 `gapSetUnitNote` 逐字同源）。
// `--want-argv` 一并变动时把它也逐字记进去（argv 是 want 的一半，改了不记 = 留痕不全）。
func gapSetWantNote(by, beforeFam, beforeAct string, beforeArgv []string, fam, act string, argv []string, evidence string) string {
	s := fmt.Sprintf("%s · %s%s改 want: want_family: %s → %s · want_action: %s → %s · 证据：%s",
		gapNow(), by, gapNoteSep,
		orDash(beforeFam), fam, orDash(beforeAct), act, evidence)
	if len(argv) > 0 {
		s += fmt.Sprintf(" · want_argv: %s → %s", orDash(strings.Join(beforeArgv, " ")), strings.Join(argv, " "))
	}
	return s
}

// gapSetWantArgvText —— `--json` / 人面共用的 argv 显示串（空 ⇒ `-`）。
func gapSetWantArgvText(argv []string) string {
	if len(argv) == 0 {
		return "-"
	}
	return strings.Join(argv, " ")
}

// gapSetWantRow —— `--json` 那一行的逐格取值。
func gapSetWantRow(r gapRecord, changed string) map[string]string {
	return map[string]string{
		"id": r.ID, "fp": r.FP, "want_family": r.WantFam, "want_action": r.WantAct,
		"want_argv": gapSetWantArgvText(r.WantArgv), "changed": changed,
	}
}

// gapSetWantPlanBlock —— `set-want` 的计划件（`--dry-run` 走 stdout · 缺 `--yes` 走 stderr）。
// `lines < 0` ⇒ 真源**未读**（缺 `--yes` 那一档：一个盘面动作都还没做）。形状照 `gapSetUnitPlanBlock`。
func gapSetWantPlanBlock(w io.Writer, title, ledgerPath string, lines int, id, fp, fam, act, argvText, evidence string) {
	fmt.Fprintf(w, "计划件（改 want 三格 · %s · 零副作用 —— 未改真源、未写审计）\n", title)
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
	fmt.Fprintf(w, "    want_family  : → %s\n", fam)
	fmt.Fprintf(w, "    want_action  : → %s\n", act)
	if argvText != "-" {
		fmt.Fprintf(w, "    want_argv    : → %s\n", argvText)
	}
	fmt.Fprintf(w, "    state        : （本动作**不改**这一格）\n")
	fmt.Fprintf(w, "    evidence     : %s（落成 `notes` 末尾一条注）\n", evidence)
	fmt.Fprintf(w, "  审计     : 计划写一行 `event=%s` + `gap_cmd=set-want`（这一态**不写**）\n", gapEventName)
}

// cmdGapSetWant —— `zerg gap set-want <GAP id> --want-family <族> --want-action <动作> --evidence <一句话> …`。
func cmdGapSetWant(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	famWant := strings.TrimSpace(inv.flagVal("--want-family"))
	actWant := strings.TrimSpace(inv.flagVal("--want-action"))
	argvWant := inv.flagVals("--want-argv")
	evidence := strings.TrimSpace(inv.flagVal("--evidence"))

	// ① 用法面（在任何盘面动作之前 —— 与同族写面同一条位置）
	if inv.jsonGiven && len(inv.fields) == 0 {
		inv.setErr("usage", "json_fields_required", "--json 不给字段")
		fmt.Fprintf(stderr, "%s: `--json` 要给逗号分隔的字段（本族口径 = 用法错 2）\n", progName)
		fmt.Fprintf(stderr, "可选字段: %s\n", strings.Join(gapSetWantFields, ","))
		return exitUsage
	}
	if len(inv.flagVals("--state")) > 0 {
		v := strings.TrimSpace(inv.flagVal("--state"))
		inv.setErr("usage", "state_not_in_shape", "--state 不在 set-want 的形状里")
		fmt.Fprintf(stderr, "%s: `gap set-want` 不收 `--state %s` —— 本面**只改 want 三格、不改 `state`**\n", progName, v)
		fmt.Fprintf(stderr, "  改态 : `zerg gap set-state <GAP id> --state <…> --evidence <…>`（人的口径）\n")
		return exitUsage
	}
	if len(inv.flagVals("--text")) > 0 {
		v := strings.TrimSpace(inv.flagVal("--text"))
		inv.setErr("usage", "text_not_in_shape", "--text 不在 set-want 的形状里")
		fmt.Fprintf(stderr, "%s: `gap set-want` 不收 `--text %s`（要写口径注 ⇒ `zerg gap note <GAP id> --text …`）\n", progName, v)
		return exitUsage
	}
	missing := []string{}
	if famWant == "" {
		missing = append(missing, "--want-family")
	}
	if actWant == "" {
		missing = append(missing, "--want-action")
	}
	if len(missing) > 0 {
		inv.setErr("usage", "missing_required", "缺必填旗标")
		fmt.Fprintf(stderr, "%s: `gap set-want` 缺必填旗标：%s\n", progName, strings.Join(missing, " · "))
		fmt.Fprintf(stderr, "用法：zerg gap set-want <GAP id> --want-family <族> --want-action <动作> [--want-argv <段>…] --evidence <一句话> [--by <谁>] [--dry-run | --yes] [--json <字段>]\n")
		return exitUsage
	}
	// ② 动作名的形态闭集（不依赖盘面 ⇒ 就先判，让「裸给 / 以 `-` 开头 / 整段散文」当场退 2）。
	//    ★ 族名的闭集要「账内那一半」⇒ 在 ⑥ 读完真源后再判（这里是同一份判据的另一半）。
	if why := gapWantActionShapeWhy(actWant); why != "" {
		inv.setErr("usage", "bad_want_action", why)
		fmt.Fprintf(stderr, "%s: `--want-action` 拒收 —— %s\n", progName, why)
		fmt.Fprintf(stderr, "  本格收**一个**能力名/token（want 是「还没实现的能力」的名字）；整段诉求写 `--evidence`\n")
		return exitUsage
	}
	if evidence == "" {
		inv.setErr("usage", "missing_required", "缺 --evidence")
		fmt.Fprintf(stderr, "%s: `gap set-want` 缺必填旗标：--evidence（**改口径必留一句证据** —— 与 `add` / `set-unit` 同一条纪律）\n", progName)
		return exitUsage
	}

	// ③ 缺 `--yes`（且非 `--dry-run`）：fail-closed rc=2，计划件走 stderr（判在读真源之前）。
	if !inv.dryRun && !inv.yes {
		ids := gapIDsOf(inv)
		id := "（未给：缺必填）"
		if len(ids) == 1 {
			id = ids[0]
		}
		gapSetWantPlanBlock(stderr, "缺 `--yes`（D2 档）", gapLedgerPath(), -1, id, "", famWant, actWant,
			gapSetWantArgvText(argvWant), evidence)
		fmt.Fprintf(stderr, "  未执行   : 缺 `--yes` ⇒ 不执行（fail-closed：从不提问）\n")
		fmt.Fprintf(stderr, "  ⚠ `--yes` 是**命令行确认档**，不是 `approve` 件（不产生 `approver` / `approval` 两格）\n")
		inv.setErr("usage", "yes_required", "缺 --yes")
		return exitUsage
	}

	// ④ 读真源（`--dry-run` 与 `--yes` 都要它：读不到 ⇒ 8）
	led, rc := gapReadLedgerOrDie(inv, stderr)
	if rc != exitOK {
		return rc
	}

	// ⑤ 点名（恰好一条 · 账内没有 ⇒ 2）
	idx, rc := gapTargetOne(inv, led, stderr, "set-want")
	if rc != exitOK {
		return rc
	}
	r := led.Recs[idx]

	// ⑥ 取值判据（**带账内那一半的完整闭集**）—— 越界 ⇒ 2 并点名越界值，真源一个字节未改。
	if why := gapWantTargetWhy(famWant, actWant, gapWantFamilyClosed(&led)); why != "" {
		inv.setErr("usage", "bad_want_target", why)
		fmt.Fprintf(stderr, "%s: `gap set-want` 拒收 —— %s\n", progName, why)
		fmt.Fprintf(stderr, "  真源     : %s（%d 行 · **本发一个字节未改**、审计一行未落）\n", orDash(led.Path), len(led.Lines))
		return exitUsage
	}

	// ⑦ 幂等：同 id 同族同动作（且未给 `--want-argv` 或给的与账内逐字相同）⇒ 「无变化」0。
	argvWantSet := len(argvWant) > 0
	if r.WantFam == famWant && r.WantAct == actWant && (!argvWantSet || gapArgvEqual(r.WantArgv, argvWant)) {
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapSetWantRow(r, "false"))
		}
		fmt.Fprintf(stdout, "无变化 %s · want=%s %s（同 id 同族同动作 ⇒ 幂等命中：不写真源、不写审计）\n", r.ID, r.WantFam, r.WantAct)
		return exitOK
	}

	// ⑧ `--dry-run`：只出计划件（stdout · rc=0 · 零副作用）
	if inv.dryRun {
		inv.changed = boolPtr(false)
		would := r
		would.WantFam, would.WantAct = famWant, actWant
		if argvWantSet {
			would.WantArgv = argvWant
		}
		if inv.jsonGiven {
			return selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapSetWantRow(would, "false"))
		}
		gapSetWantPlanBlock(stdout, "--dry-run", led.Path, len(led.Lines), r.ID, r.FP, famWant, actWant,
			gapSetWantArgvText(would.WantArgv), evidence)
		fmt.Fprintf(stdout, "  改前     : want_family=%s · want_action=%s · want_argv=%s\n",
			orDash(r.WantFam), orDash(r.WantAct), gapSetWantArgvText(r.WantArgv))
		fmt.Fprintf(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未改真源、未写审计）\n")
		return exitOK
	}

	// ⑨ 字段面前置判定（坏 `--json` 字段 ⇒ 真写之前就拒：真源一个字节不落、审计一行不落）
	if rc := gapFrontJSONFields(inv, stderr, gapSetWantFields); rc != exitOK {
		return rc
	}

	// ⑩ 真写：want 三格 + `notes` 末尾追加一条改口径留痕；`state` 逐字不动（改前改后现算对拍）。
	beforeFam, beforeAct, beforeArgv := r.WantFam, r.WantAct, r.WantArgv
	beforeState := r.State
	r.WantFam, r.WantAct = famWant, actWant
	if argvWantSet {
		r.WantArgv = append([]string{}, argvWant...)
	}
	if !gapArgvEqual(r.WantArgv, beforeArgv) || r.WantFam != beforeFam || r.WantAct != beforeAct {
		r.Notes = append(append([]string{}, r.Notes...),
			gapSetWantNote(gapByOf(inv), beforeFam, beforeAct, beforeArgv, r.WantFam, r.WantAct, r.WantArgv, evidence))
	}
	if r.State != beforeState {
		inv.setErr("failed", "state_mutated", "set-want 这一路改了 state")
		fmt.Fprintf(stderr, "%s: 内部对拍破了：本面改了 `state`（%s → %s）⇒ 不给结论（退码 1）\n", progName, beforeState, r.State)
		return exitFail
	}
	detail, wrc, msg := gapRewriteOne(inv, led, idx, r, "set-want", beforeState, beforeState)
	if wrc != exitOK {
		gapWriteFail(inv, stderr, detail, msg, "set-want")
		return wrc
	}
	// ⑪ 写后读回自证：want 三格都在（读回对拍外再点一次名）
	back, err := readGapLedger()
	if err != nil {
		inv.setErr("failed", "readback_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写后读回真源失败（退码 1）：%v\n", progName, err)
		return exitFail
	}
	bidx := gapFindIdx(back, r.ID)
	if bidx < 0 || back.Recs[bidx].WantFam != famWant || back.Recs[bidx].WantAct != actWant ||
		!gapArgvEqual(back.Recs[bidx].WantArgv, r.WantArgv) || back.Recs[bidx].State != beforeState {
		inv.setErr("failed", "readback_mismatch", "写后读回与要写的不一致")
		fmt.Fprintf(stderr, "%s: 写后读回不一致（退码 1）\n", progName)
		return exitFail
	}
	inv.changed = boolPtr(true)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, inv.fields, gapSetWantRow(back.Recs[bidx], "true"))
	}
	fmt.Fprintf(stdout, "已改 want %s（真源 %d 行不变 · 只重写目标那一行 · 审计已落 1 行）\n", r.ID, len(led.Lines))
	if back.Recs[bidx].WantFam != beforeFam {
		fmt.Fprintf(stdout, "  want_family : %s → %s\n", orDash(beforeFam), back.Recs[bidx].WantFam)
	}
	fmt.Fprintf(stdout, "  want_action : %s → %s\n", orDash(beforeAct), back.Recs[bidx].WantAct)
	if !gapArgvEqual(beforeArgv, back.Recs[bidx].WantArgv) {
		fmt.Fprintf(stdout, "  want_argv   : %s → %s\n", gapSetWantArgvText(beforeArgv), gapSetWantArgvText(back.Recs[bidx].WantArgv))
	}
	if back.Recs[bidx].WantFam == beforeFam {
		fmt.Fprintf(stdout, "  want_family : %s（逐字未动）\n", back.Recs[bidx].WantFam)
	}
	fmt.Fprintf(stdout, "  state       : %s（逐字未动）\n", back.Recs[bidx].State)
	fmt.Fprintf(stdout, "  evidence    : %s（已落成 `notes` 末尾一条注）\n", evidence)
	gapReceiptPrint(stdout, msg)
	return exitOK
}

// gapArgvEqual —— 两串 argv 逐字相等（长度 + 每格 · 空与 nil 视同）。
func gapArgvEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
