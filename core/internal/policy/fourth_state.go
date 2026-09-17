// fourth_state.go — B 项④-B：**第 4 态「追问/阻滞」**（设计稿 v2.1 §4.7 增的第 4 态）。
//
// 要治的缺口（§4.7 / §4.11 / 设计-任务模块骨架 v1.0 §1.7）：
//
//	原三态「通过 / 打回 / 升级」里**没有「信息不足」的位置**：片缺判据、缺上下文、落点不存在时，
//	执行者只能二选一 —— 硬猜（= 假绿 ✗，§4.11「缺信息硬猜 = 假绿」）或自作主张把一切都判红。
//	本文件给的是第三条路：**停下来，退回上游补规格**，且这次停顿是**有期限、可枚举、超时会被结算**的。
//
// ── 挂哪一类挂单（**选择理由写在这里，也写进回报**）──
//
// 映射结果：**`interaction`（交互点）**，即本包的 `FourthStateKind`。
//
//	· 选的为什么是 interaction：它的**超时语义是「取消」**——`timeoutOutcome` 给的是
//	  `Result=cancel` + 「本次交互作废，**不得当作已确认**；要接着做必须重新发起交互」。
//	  这正好是「阻滞」该有的结算：**没人补规格 ⇒ 片不许自行解除阻滞**（超时不放行）。
//	· 为什么不选 `ask_user`：它的超时语义是 **`continue`（继续）**。对一条「因缺规格而阻滞」的片，
//	  「继续」就是**超时即静默放行**（片带着没补齐的规格往下跑 = 硬猜）—— 与 §4.7「超时一律 fail closed
//	  （禁静默通过）」「禁无限等待」直接冲突。ask_user 面向的是「模型顺手问一句，问不到也能往下走」，
//	  而本态的语义是「**不问到就不能往下走**」。
//	· 为什么不选 `tool_approval`：它批的是**工具执行**，超时给的是 `Effect=deny`（对**工具判定**的结论）。
//	  追问一个片并不批任何工具 ⇒ 用它会把「信息不足」记成「工具被拒」，污染工具判定面（同名/同因不同事）。
//
// ⇒ 于是本态**天然获得**：① 逐类超时语义（不是第二套逻辑）；② `ListOpen` 的可枚举性（谁在等、等什么）；
//
//	③ `PendingLoadReport` 的可回答性（重启后仍然数得清有几条没答）；④ 落盘/重启的既有纪律。
//
// ── 三条硬口径（写死在代码里，由用例逐条钉住）──
//
//	① **必须有期限**（`Deadline` 零值 ⇒ 不开单）：§4.7「禁无限等待」。`interaction` 在 pending.go 里
//	   「可以零值 = 不设期限」（人工会话允许无限等），**但第 4 态不吃这一档** —— 阻滞可以无限期停着的
//	   前提是「有人看得见它在等」，而无人值守场景下没人看得见 ⇒ 我们这层强制要期限（本层的不变式，
//	   不改 pending.go 一个字）。
//	② **必须有原因码且过 Validate**：阻滞也要说清「缺什么、该谁改」（与 sendback_reason.go 同一口径）。
//	③ **超时一律 fail closed**：结算只走 `Expire`（唯一的超时实现）与其逐类语义；`AllowedToProceed()` 在
//	   超时 / 未答 / 上游拒答 三种情形下**一律 false** —— 只有「上游真的补了规格」（`approve`）才放行。
//
// ── 本批边界（与 pending.go / sendback_reason.go 同一纪律）──
//
//	· **不建第二套超时逻辑**：到期判定、结算、逐类结局全部来自 pending.go（`Expire` + `timeoutOutcome`），
//	  本文件**一行到期判据都没有**。
//	· **不改 pending.go / decide.go / human_decision.go**：本文件只新增（零回归）。
//	· **不实现整个状态机**（设计-任务模块骨架 v1.0 §1.1/§1.2 的 10 态/转移表/validator）—— 本批只做
//	  「追问/阻滞」这一条衔接：进态 = 开一张挂单；出态 = 上游补规格 / 上游拒答 / 超时作废。
//	· 叶子包：只 import 标准库（fmt / strings / time）。
package policy

import (
	"fmt"
	"strings"
	"time"
)

// FourthStateKind — 第 4 态「追问/阻滞」挂的挂单类别。
//
// **写死为 `interaction`**（理由见文件头：interaction 的超时语义 = 取消 = 不得当作已确认，
// 而 ask_user 的超时 = 继续 = 对阻滞而言就是静默放行）。改这一行 = 改第 4 态的语义，评审点就在这一行。
const FourthStateKind PendingKind = PendingInteraction

// ── 入参 ────────────────────────────────────────────────────────────────────

// ClarificationRequest — 进第 4 态所需的全部事实（无默认值可省）。
type ClarificationRequest struct {
	// SliceID — 哪一片阻滞了（退回上游补规格的对象；空 ⇒ 不开单：回指不到片）。
	SliceID string
	// Upstream — **退给谁**（上游切片者 / 规格所有者 / 缺陷条目负责人；空 ⇒ 不开单：
	// 「退回上游」却不写明退回谁，等于把片丢在半路 —— §1.2 的拒绝语要求「可行动」）。
	Upstream string
	// Point — 交互点/节点名（挂单的 `Tool` 字段，三类挂单都**必须非空**）。
	// 空 ⇒ 由 `clarify:<SliceID>` 派生（确定性：同样一串操作必然得到同样的挂单 id）。
	Point string
	// Code — **主原因码**（为什么阻滞：哪条判据/哪项输入没齐）。必须过 `Validate`（含非空建议）。
	Code SendBackCode
	// RuleID — 凭什么开这张挂单（命中的规则/判据定位串；仅供审计回查，可不填）。
	// 注意：**挂单上的 `RuleID` 被本文件用来承载原因码的「码」部分**（低基数、可聚合、可回查），
	// 故这条原始定位串会拼进挂单 `Note` 而不占 `RuleID` 字段（见 buildClarificationPending 的注释）。
	RuleID string
	// Deadline — **期限**（零值 ⇒ 不开单：§4.7「禁无限等待」）。
	Deadline time.Time
	// SessionID — 归属会话（可选）。
	SessionID string
	// Note — 追加在人可读说明后面的一句补充（可选）。
	Note string
}

// interactionPoint — 挂单的交互点名（空 ⇒ 从片 id 派生，确定性的）。
func (r ClarificationRequest) interactionPoint() string {
	if p := strings.TrimSpace(r.Point); p != "" {
		return p
	}
	return "clarify:" + strings.TrimSpace(r.SliceID)
}

// Validate — 开单前的校验（全部 fail-closed：认不出/缺件 ⇒ 报错、不开单）。
func (r ClarificationRequest) Validate() error {
	if strings.TrimSpace(r.SliceID) == "" {
		return fmt.Errorf("第 4 态（追问/阻滞）：缺 SliceID ⇒ 回指不到是哪一片阻滞了（不开单）")
	}
	if strings.TrimSpace(r.Upstream) == "" {
		return fmt.Errorf("第 4 态（追问/阻滞）：片 %s 缺 Upstream（退给谁）⇒ 「退回上游补规格」必须点明上游是谁（不开单）",
			strings.TrimSpace(r.SliceID))
	}
	if err := r.Code.Validate(); err != nil {
		return fmt.Errorf("第 4 态（追问/阻滞）：片 %s 的原因码不可用（%w）⇒ 阻滞也要说清「缺什么、该谁改」（不开单）",
			strings.TrimSpace(r.SliceID), err)
	}
	if r.Deadline.IsZero() {
		return fmt.Errorf("第 4 态（追问/阻滞）：片 %s 缺 Deadline（期限是零值）⇒ §4.7「禁无限等待」："+
			"不设期限的阻滞在无人值守下既等不到人、也不会被结算（不开单）。形态：Deadline = now.Add(期限)",
			strings.TrimSpace(r.SliceID))
	}
	return nil
}

// ── 出态：四种处置 ──────────────────────────────────────────────────────────

// FourthDisposition — 第 4 态的**处置**（四类，无第五类；`AllowedToProceed` 由它派生）。
type FourthDisposition string

const (
	// FourthAwaiting — 挂单未答：片停在 `awaiting`，**不得推进**（等上游补规格；禁硬猜）。
	FourthAwaiting FourthDisposition = "awaiting"
	// FourthSupplied — 上游**补了规格**（挂单答复 `approve`）⇒ 阻滞解除，可回 `running` 重判。
	FourthSupplied FourthDisposition = "supplied"
	// FourthEscalate — 上游**拒答/无规格可补**（答复 `reject`）⇒ 升级给人，**不许硬猜**。
	FourthEscalate FourthDisposition = "escalate"
	// FourthVoid — **超时作废**（interaction 的逐类超时语义 = 取消）或答复 `cancel`
	// ⇒ 本次交互作废、**不得当作已确认**；要接着做必须重新发起或升级给人。
	FourthVoid FourthDisposition = "void"
)

// FourthStateOutcome — 第 4 态的**结算单**（事实，不是建议）：接线批按它分派，审计按它取证。
type FourthStateOutcome struct {
	PendingID string       `json:"pending_id"`
	Kind      PendingKind  `json:"kind"`               // 恒为 FourthStateKind（interaction）
	SliceID   string       `json:"slice_id,omitempty"` // 从挂单交互点名回解（自定 Point 时为空）
	Upstream  string       `json:"upstream,omitempty"` // 退给谁（开单时记进 Note；结算面从 Note 回解，解不出则空）
	Code      SendBackCode `json:"-"`                  // 主原因码（挂单 RuleID 承载其「码」部分）

	// Disposition — 处置（唯一分派依据）。
	Disposition FourthDisposition `json:"disposition"`
	// Allowed — 是否**允许推进**（= Disposition == FourthSupplied）。
	// 冗余存一份：接线批读字段比读函数更难读错方向（超时/未答/拒答一律 false）。
	Allowed bool `json:"allowed_to_proceed"`

	// LedgerState / LedgerAnswer — 挂单落库时的事实（开单时为 open，结算时是答复/超时后的状态）。
	LedgerState  PendingStatus `json:"ledger_state"`
	LedgerAnswer Answer        `json:"ledger_answer,omitempty"`

	// ── 超时结算的事实（直接来自 pending.go 的逐类语义；未超时则全为空）──
	TimedOut      bool          `json:"timed_out"`
	TimeoutResult TimeoutResult `json:"timeout_result,omitempty"` // interaction ⇒ cancel
	TimeoutEffect Effect        `json:"timeout_effect,omitempty"` // 不涉及工具判定 ⇒ 空串（空串**不是** allow）

	// ModelNote — **给模型看的话**（必须能独立读懂：模型只看到这一句）。
	ModelNote string `json:"model_note,omitempty"`
	// Note — 给人/审计看的一句话（含依据，可回查）。**永不喂给模型**。
	Note string `json:"note,omitempty"`
}

// AllowedToProceed — 是否允许片继续推进。**只有「上游真的补了规格」是 true**：
// 未答（还在等）· 超时（作废）· 上游拒答（升级给人）· 认不出的情形（报错）**一律 false**。
func (o FourthStateOutcome) AllowedToProceed() bool {
	return o.Disposition == FourthSupplied
}

// ── 进态：开一张「追问/阻滞」挂单 ───────────────────────────────────────────

// RaiseClarification — 进第 4 态（**唯一入口**）：把「信息不足、退回上游补规格」开成一张挂单。
//
// 返回的 outcome 恒为 `FourthAwaiting`（`AllowedToProceed()` == false）——
// 进态这件事本身**不放行任何东西**：片停在 awaiting，等上游补规格。
//
// 开单失败一律**报错且不产生任何挂单**（fail-closed：宁可不进态，也不开一张回指不到的挂单）。
func RaiseClarification(store PendingStore, req ClarificationRequest, now time.Time) (FourthStateOutcome, error) {
	if store == nil {
		return FourthStateOutcome{}, fmt.Errorf("第 4 态（追问/阻滞）：挂单账本为空 ⇒ 不开单（fail-closed，不静默跳过）")
	}
	if err := req.Validate(); err != nil {
		return FourthStateOutcome{}, err
	}
	slice := strings.TrimSpace(req.SliceID)
	code := req.Code

	p, err := store.Open(buildClarificationPending(req, code), now)
	if err != nil {
		return FourthStateOutcome{}, fmt.Errorf("第 4 态（追问/阻滞）：片 %s 开单失败 ⇒ 不静默放行（片不得带着缺规格继续）：%w",
			slice, err)
	}
	out := FourthStateOutcome{
		PendingID:   p.ID,
		Kind:        p.Kind,
		SliceID:     slice,
		Upstream:    strings.TrimSpace(req.Upstream),
		Code:        code,
		Disposition: FourthAwaiting,
		Allowed:     false,
		LedgerState: p.State,
		ModelNote: "片 " + slice + " 因**信息不足**进第 4 态「追问/阻滞」：已退回上游（" +
			strings.TrimSpace(req.Upstream) + "）补规格，**不要硬猜**（缺信息硬猜 = 假绿）。" +
			"等不到答复就按超时结算：到点后本次交互作废，片**不得自行解除阻滞**。",
		Note: fmt.Sprintf("第 4 态（追问/阻滞）开单 %s：片 %s · 退给 %s · 原因码 %s · 期限 %s · 凭什么 %s",
			p.ID, slice, strings.TrimSpace(req.Upstream), code.String(),
			req.Deadline.UTC().Format(time.RFC3339), ruleText(req.RuleID)),
	}
	// ── B 项⑤ 观测（③ 第 4 态进态）: 恰一行 `slice_escalated` ──
	// best-effort：Emit 无返回值 ⇒ 无论写失败与否，返回值与上面的 outcome **逐字段一致**。
	emitSliceEscalatedFromRaise(out, req.Upstream, req.Deadline)
	return out, nil
}

// buildClarificationPending — 组装挂单。字段映射写在这里（一处，便于审计回查）：
//
//	Kind      = FourthStateKind（interaction）
//	Tool      = 交互点名（req.Point，空则 clarify:<SliceID>）
//	RuleID    = **原因码的「码」部分**（如 PRE_R5）—— 它是「凭什么要追问」的机器可读答案，
//	            低基数、可聚合、可枚举；开单侧与结算侧都按它回解原因码（**唯一真源，不另存副本**）。
//	ExpiresAt = req.Deadline（**必填**，见 ClarificationRequest.Validate 的 ①）
//	Note      = 人可读一行（片 / 上游 / 建议 / 原始 RuleID 定位串 / 调用方补充），**格式写死**：
//	            `第 4 态追问/阻滞：片 <片>；退给 <上游>；原因码 <码>（需要补的规格：<建议>）；凭什么 <定位串>；<补充>`
//	            —— 结算侧按它回解**片 id 与建议原文**（见 sliceOfPending/upstreamOfNote/adviceOfNote）。
//
// 为什么片 id 的真源放在 Note 而不是交互点名（实测发现，如实登记）：挂单侧会过
// `pending.go` 的**工具名归一化**（`NormalizeToolName` = 去首尾空白 + 小写）⇒ 交互点名里嵌的片 id
// 会被小写化（`S3-C1` ⇒ `s3-c1`）。那是既有的挂单纪律（本批一行不改），故**片 id 的原始大小写
// 以 Note 标记为准**，交互点名只作回退（解出来是小写，仍可定位）。
func buildClarificationPending(req ClarificationRequest, code SendBackCode) Pending {
	note := fmt.Sprintf("第 4 态追问/阻滞：片 %s；退给 %s；原因码 %s", strings.TrimSpace(req.SliceID),
		strings.TrimSpace(req.Upstream), code.String())
	if advice := strings.TrimSpace(code.Advice); advice != "" {
		note += "（需要补的规格：" + advice + "）"
	}
	note += "；凭什么 " + ruleText(req.RuleID)
	if extra := strings.TrimSpace(req.Note); extra != "" {
		note += "；" + extra
	}
	return Pending{
		Kind:      FourthStateKind,
		Tool:      req.interactionPoint(),
		RuleID:    code.String(),
		ExpiresAt: req.Deadline,
		SessionID: req.SessionID,
		Note:      note,
	}
}

// ── 出态：结算一张「追问/阻滞」挂单 ─────────────────────────────────────────

// SettleClarification — 结算第 4 态（**唯一出口**）：读挂单的既有事实（超时 / 答复）⇒ 处置。
//
// **超时完全走 pending.go 的既有语义**：本函数调用 `Expire(now)`（唯一的超时结算实现，
// 判据是 `!now.Before(ExpiresAt)` —— 恰等于到期时刻也算超时），再用 `timeoutOutcome` 取逐类结局。
// **本文件没有任何到期判据、没有第二个结算分支**（自建第二套超时逻辑是本态的明令禁止项）。
//
// 注意两件事（都由 pending.go 的既有纪律带来，不是本函数的"额外副作用"）：
//
//	· `Expire` 会**顺带结算账本里其它到点挂单**（这正是「到点即超时」的既有语义：不依赖谁记得去扫）；
//	· 若这张挂单**早已**被别人结算成超时（如上一轮 `Expire` 扫过、或 `Answer` 在到点时点被拒），
//	  本函数照样判成 `FourthVoid`（幂等，不复活成「还能答」）。
//
// fail-closed（**认不出一律报错，绝不当作放行**）：
//
//	· 挂单 id 为空 / 账本为空 ⇒ 报错
//	· 没有这张挂单 ⇒ 报错
//	· **类别不是 FourthStateKind**（如拿一张 tool_approval 挂单来结算）⇒ 报错（不猜语义：
//	  tool_approval 的 approve 是「放行工具」，不是「补了规格」）
//	· 挂单的 `RuleID` 不是合法原因码 ⇒ 报错（回解不到原因码 = 这张挂单不是本态开的）
//	· 答复认不出（不在 approve/reject/cancel 内，含对 interaction 非法的 `correct`）⇒ 报错（不猜）
//	· 状态认不出 ⇒ 报错
func SettleClarification(store PendingStore, pendingID string, now time.Time) (FourthStateOutcome, error) {
	if store == nil {
		return FourthStateOutcome{}, fmt.Errorf("第 4 态（追问/阻滞）：挂单账本为空 ⇒ 判不了（不静默放行）")
	}
	id := strings.TrimSpace(pendingID)
	if id == "" {
		return FourthStateOutcome{}, fmt.Errorf("第 4 态（追问/阻滞）：挂单 id 为空 ⇒ 判不了（不静默放行）")
	}

	// ① 唯一的超时结算实现（到点即超时；幂等）。我们只需知道自己那张有没有被这次结算到。
	var timedOut *TimeoutOutcome
	for _, o := range store.Expire(now) {
		if o.PendingID == id {
			oc := o
			timedOut = &oc
		}
	}

	p, ok := store.Lookup(id)
	if !ok {
		return FourthStateOutcome{}, fmt.Errorf("第 4 态（追问/阻滞）：没有这张挂单（id=%s）⇒ 判不了、也不许当作已答复", id)
	}
	if p.Kind != FourthStateKind {
		return FourthStateOutcome{}, fmt.Errorf("第 4 态（追问/阻滞）：挂单 %s 的类别是 %s ⇒ 不是第 4 态的挂单"+
			"（第 4 态只挂 %s；拿别的类别来结算会把它自己的语义读错 —— 如 tool_approval 的 approve 是「放行工具」）",
			p.ID, string(p.Kind), string(FourthStateKind))
	}
	code, err := ParseSendBackCode(p.RuleID)
	if err != nil {
		return FourthStateOutcome{}, fmt.Errorf("第 4 态（追问/阻滞）：挂单 %s 的 RuleID「%s」回解不出原因码 ⇒ "+
			"这张挂单不是本态开的（不猜它的语义）：%w", p.ID, p.RuleID, err)
	}

	out := FourthStateOutcome{
		PendingID:    p.ID,
		Kind:         p.Kind,
		SliceID:      sliceOfPending(p),
		Upstream:     upstreamOfNote(p.Note),
		Code:         code,
		LedgerState:  p.State,
		LedgerAnswer: p.Answer,
	}
	// 建议原文以挂单 Note 的标记为准（`RuleID` 只承载码本身；解不出就保留 RuleID 里带来的那份，不编）。
	if advice := adviceOfNote(p.Note); advice != "" {
		out.Code.Advice = advice
	}
	if timedOut != nil { // 本次调用结算的：直接用逐类结局（事实）
		out.TimedOut = true
		out.TimeoutResult = timedOut.Result
		out.TimeoutEffect = timedOut.Effect
	}

	switch p.State {
	case StatusOpen:
		// 还没到点、也还没人答：片停在 awaiting —— **不放行**。
		out.Disposition = FourthAwaiting
		out.Allowed = false
		out.ModelNote = "片 " + out.SliceID + " 仍在第 4 态「追问/阻滞」：上游（" + out.Upstream +
			"）还没补规格 ⇒ **不要推进、不要硬猜**（缺信息硬猜 = 假绿）。"
		out.Note = fmt.Sprintf("挂单 %s 仍未答（open）⇒ 片停 awaiting；原因码 %s", p.ID, code.String())

	case StatusTimedOut:
		// ★ 超时 fail closed：interaction 的逐类语义是「取消」⇒ 本次交互作废、**不得当作已确认**。
		//   无论逐类结局是什么，本态一律**不放行**（`Allowed=false`）—— 这条不许被任何分支绕过。
		t := timeoutOutcome(p) // 逐类语义的唯一实现处（在 pending.go）；这里只读它的事实
		if timedOut != nil {   // 本次调用结算的那条：直接用它（与上面同源，同一份事实）
			t = *timedOut
		}
		out.TimedOut = true
		out.TimeoutResult = t.Result
		out.TimeoutEffect = t.Effect
		out.Disposition = FourthVoid
		out.Allowed = false
		out.ModelNote = "片 " + out.SliceID + " 的第 4 态「追问/阻滞」**超时**⇒ 按既有逐类语义（" +
			string(FourthStateKind) + "）结算为 **" + string(t.Result) + "**：本次交互作废，**不得当作已确认**。" +
			"该片**不得自行解除阻滞**（既不继续按缺规格往下跑，也不把沉默当批准）——" +
			"要么重新发起追问，要么升级给人。" + t.ModelNote
		out.Note = fmt.Sprintf("挂单 %s 到点未答 ⇒ 超时结算 result=%s（原因码 %s）；"+
			"**超时一律 fail closed：禁静默通过、禁无限等待**（§4.7）", p.ID, string(t.Result), code.String())

	case StatusAnswered:
		disposition, allowed, modelNote, noteErr := dispositionForAnswer(p)
		if noteErr != nil {
			return FourthStateOutcome{}, noteErr
		}
		out.Disposition = disposition
		out.Allowed = allowed
		out.ModelNote = modelNote
		out.Note = fmt.Sprintf("挂单 %s 已答复（answer=%s）⇒ 处置 %s；原因码 %s",
			p.ID, string(p.Answer), string(disposition), code.String())

	default:
		return FourthStateOutcome{}, fmt.Errorf("第 4 态（追问/阻滞）：挂单 %s 的状态「%s」认不出 ⇒ 不裁决、不放行",
			p.ID, string(p.State))
	}
	// ── B 项⑤ 观测（③ 第 4 态超时 / 上游拒答）: 升级类结局恰一行 `slice_escalated` ──
	// 只在 Disposition ∈ {escalate, void} 时落（awaiting 还在等、supplied 是上游补了规格 ——
	// 两者都不是「升级」，落事件会把低基数结局稀释掉）。best-effort：不改下面的返回值。
	emitSliceEscalatedFromSettle(out)
	return out, nil
}

// dispositionForAnswer — 把「上游的答复」翻成第 4 态的处置。**四类答复逐条写死，认出一律不猜**：
//
//	approve ⇒ FourthSupplied（**唯一放行**：上游真的补了规格）
//	reject  ⇒ FourthEscalate（上游拒答/无规格可补 ⇒ 升级给人；不许硬猜，也不许当作"没有阻塞"）
//	cancel  ⇒ FourthVoid    （交互作废 ⇒ 不得当作已确认；重新发起或升级）
//	correct / 认不出 ⇒ **报错**（`correct`（更正参数）只对工具批准有意义 —— pending.go 的 ⑥ 本就会拒；
//	                     真出现（如坏数据载入）时我们也不猜：不物化、不放行）
func dispositionForAnswer(p Pending) (FourthDisposition, bool, string, error) {
	if !p.Answer.Valid() {
		return "", false, "", fmt.Errorf("第 4 态（追问/阻滞）：挂单 %s 的答复「%s」认不出 ⇒ 不裁决、**不放行**（不猜语义）",
			p.ID, string(p.Answer))
	}
	switch p.Answer {
	case AnswerApprove:
		return FourthSupplied, true, "上游已补规格（答复 approve）⇒ 阻滞解除：可回 `running` **按新规格重判**" +
			"（重判不是重做：先按判据判一遍再决定要不要动）", nil
	case AnswerReject:
		return FourthEscalate, false, "上游拒答/无规格可补（答复 reject）⇒ **升级给人**：" +
			"缺信息硬猜 = 假绿，故片不许带着缺规格继续", nil
	case AnswerCancel:
		return FourthVoid, false, "本次追问被作废（答复 cancel）⇒ **不得当作已确认**；" +
			"要接着做必须重新发起追问或升级给人", nil
	default: // 只能是 AnswerCorrect：对 interaction 非法（pending.go ⑥），真出现时也不猜
		return "", false, "", fmt.Errorf("第 4 态（追问/阻滞）：挂单 %s 的答复「%s」对本态没有定义"+
			"（`correct`（更正参数）只对工具批准有意义）⇒ 不裁决、**不放行**", p.ID, string(p.Answer))
	}
}

// ── 从挂单事实回解小工具（**只读不猜**：解不出就返回空串，绝不编一个值）─────────

// sliceOfPending — 片 id 的真源：挂单 Note 的 `片 <id>；` 标记（**保留原始大小写**）；
// 解不出（如别的路径开的挂单）再退回交互点名里的 `clarify:<id>`（那份已被挂单侧归一为小写）。
// 两条路都解不出 ⇒ 返回空串（**不猜**）。
func sliceOfPending(p Pending) string {
	if s := markerOfNote(p.Note, "片 ", "；"); s != "" {
		return s
	}
	return sliceOfInteractionPoint(p.Tool)
}

// sliceOfInteractionPoint — 从交互点名回解片 id（**回退路径**；挂单侧已把工具名归一为小写）。
func sliceOfInteractionPoint(tool string) string {
	name := NormalizeToolName(tool)
	if s, ok := strings.CutPrefix(name, "clarify:"); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// upstreamOfNote — 从挂单 Note 回解「退给谁」（`退给 <x>；`）。解不出 ⇒ 空串（不猜）。
func upstreamOfNote(note string) string { return markerOfNote(note, "退给 ", "；") }

// adviceOfNote — 从挂单 Note 回解「需要补的规格」（`（需要补的规格：<建议>）`）。
// 取第一个右括号之前的部分 ⇒ **建议文本里不要写全角右括号**（格式约束，写在上面 buildClarificationPending 的注释里）。
func adviceOfNote(note string) string { return markerOfNote(note, "（需要补的规格：", "）") }

// markerOfNote — 在 Note 里取 `marker` 与 `end` 之间的文本（缺失/为空 ⇒ 空串，**不猜**）。
// 只服务于本文件自己写死的格式（开单侧写、结算侧读，同一份格式定义）。
func markerOfNote(note, marker, end string) string {
	i := strings.Index(note, marker)
	if i < 0 {
		return ""
	}
	rest := note[i+len(marker):]
	if j := strings.Index(rest, end); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}
