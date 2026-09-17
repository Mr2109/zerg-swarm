// human_decision.go — T5.7「人的五态 ⇒ 给模型的结果」：**语义边界硬守 + 更正结构化留痕 + 全程可审计**。
//
// 要治的两个缺口（设计稿 docs/01-设计/设计-内建调试版-v1.2-20260917.md §〇 F10/F11）：
//
//	F11（★★★）**`respond` 与 `reject` 的语义边界没写死** —— 拿 `respond` 去表达拒绝一个有副作用的工具，
//	    等于**告诉模型「做成了」**：工具一步没跑，而模型的认知里那次写/删/装包已经成功，后续全程建在假前提上
//	    （它会跳过校验、不再重试、把不存在的产物当既有事实引用 —— 这就是「污染认知」的样子）。
//	F10（★★）**缺「更正」的结构化支持** —— 「更正」改的是**工具参数**（不是改草稿），必须有标准形态、
//	    必须留痕（谁改的、改前是什么），否则事后说不清「这条命令是模型给的还是人改的」。
//
// ── 人的五态 ⇒ 给模型的结果（**写死在代码里**，由用例逐条钉住；`Materialize` 是唯一入口）──
//
//	决定        工具执行                 给模型的（Result）                        放行执行（ShouldExecute）
//	respond     **不执行**（人充当了工具）  **成功型** ToolMessage（正文 = Response 原样）    false
//	reject      **不执行**                **失败型**：「未执行」+「不要重试」+ 为什么        false
//	approve     **放行执行**              不给（真实结果稍后由工具产出）                  **true**
//	correct     **放行执行**，但**换成人给的参数**  不给                                  **true**
//	escalate    **不执行**                **不给任何能喂模型的东西**（交回上层）            false
//
// 两条铁律（F11 的全部要点，也是本文件存在的理由）：
//
//	★ **`respond` 绝不是「拒绝」的另一种说法**。respond = 人**充当工具**并把它本该产出的结果交回来
//	  ⇒ 它只能**产出**一条成功结果，绝不能改变机器状态。因此「有副作用的工具 + respond」是**非法组合**：
//	  既没有执行、又宣布成功 = 认知污染。要拒绝就写 `reject`（它的语义恰好是「不执行 + 反馈」）。
//	  **`reject` 在有副作用的工具上不但合法，而且是它的主用途** —— 闸轨（F9）存在的意义就是拦住
//	  write / rm / git push / 装包 这类动作；把 reject 也一并禁掉等于把闸拆了。
//	★ **`approve` 与 `reject` 不是同一动作的两种说法**：approve 是「给你做」，reject 是「不执行 + 反馈」。
//	  混用会让「批准」这道闸在最需要它的时候变成放行。
//
// ── fail-closed 六条（每条都有用例钉住，反例也钉）──
//
//	① **五态认不出 ⇒ 报错**（`ErrUnknownDecisionKind`）：不猜、不降级、不产出任何结果。
//	② **`respond` 用在有副作用的工具上 ⇒ 报错**（`ErrRespondOnSideEffect`）。副作用事实**缺省按「有」处理**：
//	   不传 `ToolFacts` = 不知道 ⇒ 就不给「宣布成功」的权利（这是本文件唯一一处「不知道 ⇒ 更严」的默认，
//	   方向与 T5.1 的 fail-closed 一致）。
//	③ **批错对象 ⇒ 报错**：决定里的工具名必须与挂单**逐字一致**，参数指纹两侧都非空且不同即报错；
//	   挂单带指纹而决定没写也报错。理由与 grants 的「批准 `cat a.txt` 不许覆盖 `cat b.txt`」同源 ——
//	   批错对象比不批更危险。
//	④ **不许替人签字**：挂单必须是**已答复**的（`open` ⇒ 报错）。唯一例外是 `escalate`（它不产出任何给模型
//	   的东西，故不冒充谁签了字）；但**已按超时结算**的挂单一律报错（迟到的答复不改变结局 —— 与 pending.go ② 同向）。
//	⑤ **决定必须与账本里记的答复同族**（approve↔approve / reject↔reject / correct↔correct）。
//	   `respond` 在 `Answer` 枚举里**没有对应值**（pending.go 只有 approve|reject|correct|cancel，本批不改它）
//	   ⇒ 不查同族，但把账本答复**原样抄进 outcome**（`LedgerAnswer`）供审计对照。
//	⑥ **更正必须留痕**：形态必须是 `{"type":"edit","edited_action":{…}}`、更正后的工具名非空、
//	   **`EditedBy` 非空**（谁是改动人）—— 缺一样就报错（说不清「谁改的」的更正不算留痕）。
//	   另：`reject` 必须给出「为什么」（`Rejection` 与 `Reason` 至少一个非空）——
//	   不解释的拒绝会让模型反复试同一条路。
//
// ── 告警（F10 要求显式告警，故它不是散文而是常量 + outcome 字段）──
//
//	`CorrectReplanWarning` —— 更正改动过大 ⇒ 模型会**重新规划** ⇒ 同一步动作**可能被重复执行**。
//	它同时进 `ToolCallOutcome.Warning`（机器可判）与 `Note`（人可读）。
//
// ── 本批边界（与 T5.1/T5.3/T5.4/T5.8 同一纪律）──
//
//	· 叶子包：只 import 标准库（crypto/sha256 / encoding/hex / encoding/json / errors / fmt / strings），
//	  零内部依赖（不 import audit —— 与 pending.go 对 `ArgsFingerprint` 的同一纪律）。
//	· **不接任何执行路径**：本文件只做「决定 + 挂单 ⇒ 交接单」的翻译；真正执行 / 弹窗 / 落盘属接线批。
//	  toolobs / chat / gateway / agent 一行不动。
//	· **不重写 pending.go**：它管「谁在等、等到什么时候、到点怎么结算」；本文件在旁边补
//	  「人的答复翻译成什么」这一层，两者**不互相改写**（本文件不调用 Open/Answer，只读挂单事实）。
//	· `ToolCallOutcome.Note` 是**给人/审计**看的（与 `Verdict.Note` 同一纪律）：**永不喂给模型**；
//	  给模型的话只在 `Result.Content` 与 `Result.ModelNote` 里。
//	· 更正后的调用**必须重新过一遍策略闸**：原批准只覆盖原参数（T5.3 的逐字匹配纪律），
//	  换过工具名时（`ToolChanged=true`）更是另一件动作 —— 本包不替接线批判，只把事实标出来。
package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ── 专项错误（可判：接线批按 errors.Is 分派，不解析文本）──────────────────────

var (
	// ErrEscalated — `escalate` 的**专项信号**：它既不是普通错误、也不是普通结果。
	// 接线批必须**显式接手**（上层处置），既不许把它当成一条工具结果喂给模型，也不许默默丢掉。
	ErrEscalated = errors.New("人的决定是 escalate：交回上层处置（不是可以喂给模型的结果）")
	// ErrRespondOnSideEffect — 反例⑤：`respond` 被用在**有副作用**的工具上。
	ErrRespondOnSideEffect = errors.New("respond 被用在有副作用的工具上：那会把「没做」说成「做成了」（要拒绝请用 reject）")
	// ErrUnknownDecisionKind — 反例⑥：决定的 Kind 认不出（fail-closed，不猜语义）。
	ErrUnknownDecisionKind = errors.New("人的决定认不出")
	// ErrInvalidDecision — 其余校验失败（批错对象 / 未答复 / 已超时 / 缺内容 / 缺留痕…）。
	ErrInvalidDecision = errors.New("人的决定与挂单对不上")
)

// ── 人的五态 ────────────────────────────────────────────────────────────────

// DecisionKind — 人对一次工具调用能做的五种决定（五态，无第六态）。
type DecisionKind string

const (
	// DecisionRespond — **人充当工具**：人不让工具跑，而是自己把「工具本该产出的结果」交回来。
	// 它给模型的是一条**成功型**结果 ⇒ **只对无副作用的工具成立**（见 ErrRespondOnSideEffect）。
	DecisionRespond DecisionKind = "respond"
	// DecisionReject — **拒绝 + 反馈**：工具**不执行**，并把人给的理由与反馈交给模型（超时给的也是它）。
	DecisionReject DecisionKind = "reject"
	// DecisionApprove — **批准**：放行执行（真实工具结果稍后产出）。
	DecisionApprove DecisionKind = "approve"
	// DecisionCorrect — **更正**（F10）：改参数后放行执行；改前的原文与改动人都要留痕。
	DecisionCorrect DecisionKind = "correct"
	// DecisionEscalate — **上交**：人处理不了，交回上层（不得默默丢弃，也不得当成给模型的结果）。
	DecisionEscalate DecisionKind = "escalate"
)

// Valid — 是否五态之一（认不出一律不猜，见 Materialize ① 与 ParseDecisionKind）。
func (k DecisionKind) Valid() bool {
	switch k {
	case DecisionRespond, DecisionReject, DecisionApprove, DecisionCorrect, DecisionEscalate:
		return true
	}
	return false
}

// ParseDecisionKind — 解析决定的文本（大小写不敏感、去首尾空白）；认不出一律**报错**（不猜）。
func ParseDecisionKind(raw string) (DecisionKind, error) {
	k := DecisionKind(strings.ToLower(strings.TrimSpace(raw)))
	if !k.Valid() {
		return "", fmt.Errorf("%w：未知决定「%s」（只认 respond|reject|approve|correct|escalate）",
			ErrUnknownDecisionKind, strings.TrimSpace(raw))
	}
	return k, nil
}

// ── 人的决定 ⇒ 低基数原因码（与 policy.go 的枚举**同一个 Reason 类型**，不另造字符串）──

const (
	ReasonHumanRespond  Reason = "human_respond"  // 人充当工具（成功型结果）
	ReasonHumanReject   Reason = "human_reject"   // 人明确拒绝（工具不执行）
	ReasonHumanApprove  Reason = "human_approve"  // 人批准（放行执行）
	ReasonHumanCorrect  Reason = "human_correct"  // 人更正了参数（按新参数放行执行）
	ReasonHumanEscalate Reason = "human_escalate" // 人上交上层
)

// ── 更正的结构化形态（F10 写死）──────────────────────────────────────────────

// EditActionType — 更正动作的固定类型串（F10 标准形态里的 `"type":"edit"`）。认不出的 type ⇒ 报错（不猜）。
const EditActionType = "edit"

// EditedToolCall — 更正后的**工具动作**（工具名 + 参数）。
type EditedToolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// EditedAction — 更正的结构化形态（F10 写死的标准形态）：
//
//	{"type":"edit","edited_action":{"name":…,"args":…}}
type EditedAction struct {
	Type         string         `json:"type"`
	EditedAction EditedToolCall `json:"edited_action"`
}

// NewEdit — 构造一个合法形态的更正动作：`Type` 由这里写死，调用方漏不了。
func NewEdit(name string, args map[string]any) EditedAction {
	return EditedAction{Type: EditActionType, EditedAction: EditedToolCall{Name: name, Args: args}}
}

// ── 给模型看的那条结果 ───────────────────────────────────────────────────────

// ToolResultKind — 工具结果的成败（两态）。**没有结果就没有成败**（见 ToolResult.Emitted）。
type ToolResultKind string

const (
	// ResultSuccess — 成功型（`respond`：人替工具给了结果）。
	ResultSuccess ToolResultKind = "success"
	// ResultFailed — 失败型（`reject`：未执行）。**它绝不许被读成成功**。
	ResultFailed ToolResultKind = "failed"
)

// ToolResult — 一条**回给模型的工具结果**（这里是与模型之间的语义边界所在）。
type ToolResult struct {
	// Emitted — 本 outcome 是否**就是**那条要回给模型的工具结果。
	// false = 还没有工具结果：approve/correct 放行执行、真实结果稍后产出；escalate 交回上层。
	Emitted bool `json:"emitted"`
	// Kind — 成败：success（respond）/ failed（reject）。**Emitted=false 时为空串** ——
	// 没有结果就没有成败，此时读 Kind 是无意义的。
	Kind ToolResultKind `json:"kind,omitempty"`
	// Content — 结果正文：respond 时**就是人给的内容原样**；reject 时是「未执行 + 为什么 + 不要重试」。
	Content string `json:"content,omitempty"`
	// ModelNote — 给模型的一句硬话（reject 时不许重试、不许当已完成）。
	ModelNote string `json:"model_note,omitempty"`
}

// RejectModelNote — `reject` 给模型的硬话（失败型结果的核心一句）。
const RejectModelNote = "这次调用的结果是「拒绝」，**不是**「失败后重试」：不要重试它，也不要把它的结果当成已经发生的事实；" +
	"要推进请换一条做法，或把问题明说后问人。"

// CorrectReplanWarning — F10 要求显式告警的那一条：更正改动过大 ⇒ 模型会**重新规划** ⇒ 同一步动作可能被**重复执行**。
const CorrectReplanWarning = "更正改动过大时，模型会重新规划 ⇒ 同一步动作**可能被重复执行**（请把更正限制在参数层面）"

// ── 交接单（物化结果）───────────────────────────────────────────────────────

// OutcomeKind — 物化结果的类别（五类，与五态一一对应）。
type OutcomeKind string

const (
	// OutcomeResponded — 人充当工具 ⇒ 产出**成功型**结果，工具不执行。
	OutcomeResponded OutcomeKind = "responded"
	// OutcomeRejected — 拒绝 ⇒ 产出**失败型**结果，工具不执行。
	OutcomeRejected OutcomeKind = "rejected"
	// OutcomeApproved — 批准 ⇒ 放行执行（本 outcome 不含给模型的结果）。
	OutcomeApproved OutcomeKind = "approved"
	// OutcomeCorrected — 更正 ⇒ 按**新参数**放行执行（本 outcome 不含给模型的结果）。
	OutcomeCorrected OutcomeKind = "corrected"
	// OutcomeEscalated — 上交 ⇒ **不产出给模型的结果**，交回上层（配套返回 ErrEscalated）。
	OutcomeEscalated OutcomeKind = "escalated"
)

// ToolCallOutcome — 把人的答复翻译成的**交接单**：接线批按它分派，审计按它取证。
// 每个分支都带全套痕迹：原始参数指纹（`ArgsDigest`）+ 决定原文（`Decision`）+ 挂单回指（`PendingID`）
// + 账本答复（`LedgerState`/`LedgerAnswer`）+ 低基数原因码（`Reason`）。
type ToolCallOutcome struct {
	// Kind — 类别（唯一的分派依据）。
	Kind OutcomeKind `json:"kind"`
	// Tool — **原始**工具名（挂单上的那一个）。更正换过工具时见 EditedAction（与 ToolChanged）。
	Tool string `json:"tool"`
	// Result — 要回给模型的那条结果（Emitted=false ⇒ 没有这条）。
	Result ToolResult `json:"result"`

	// ── 更正的结构化留痕（F10；只有 correct 分支有值）──
	EditedAction *EditedAction `json:"edited_action,omitempty"` // 更正后的动作（**原样**带给执行侧）
	EditedBy     string        `json:"edited_by,omitempty"`     // 谁改的（留痕的地基）
	ToolChanged  bool          `json:"tool_changed,omitempty"`  // 更正把工具名也换了 ⇒ **必须重新过一遍策略闸**
	// NewArgsLocalDigest — **本包口径**的规范化新参数指纹（json.Marshal 键排序 + sha256 前 8 字节的 hex）。
	// 它**不承诺**与接线批的 audit.ArgsFingerprint 同口径，只用于「更正前后是不是同一个东西」的对照；
	// 算不出（参数不可序列化 / 为空）时是**显式空串**，不编一个假指纹。
	NewArgsLocalDigest string `json:"new_args_local_digest,omitempty"`
	// Warning — 机器可判的告警（correct 时为 CorrectReplanWarning）。
	Warning string `json:"warning,omitempty"`

	// ── 全分支共有的审计痕迹（每个分支都必须有）──
	Decision     DecisionKind  `json:"decision"`                // 人的决定原文（五态）
	PendingID    string        `json:"pending_id,omitempty"`    // 批的是哪一张挂单
	SessionID    string        `json:"session_id,omitempty"`    // 归属会话
	RuleID       string        `json:"rule_id,omitempty"`       // 凭什么要人批
	ArgsDigest   string        `json:"args_digest,omitempty"`   // **原始**参数指纹（批的是哪一次调用）
	LedgerState  PendingStatus `json:"ledger_state,omitempty"`  // 挂单落库时的状态（answered / open）
	LedgerAnswer Answer        `json:"ledger_answer,omitempty"` // 账本里记的答复（respond/escalate 不查同族，故要原样留下对照）
	Reason       Reason        `json:"reason,omitempty"`        // 低基数原因码（可聚合、可告警）
	// Note — 给人/审计看的一句话（含依据，可回查）。**永不喂给模型**（与 Verdict.Note 同一纪律）。
	Note string `json:"note,omitempty"`
}

// ShouldExecute — 接线批是否该**按（可能被更正过的）参数去执行**：approve / correct ⇒ true，其余 false。
// 它是「执行」这件事的唯一判据（respond/reject/escalate 都不执行；respond 只是人替工具交了一份结果）。
func (o ToolCallOutcome) ShouldExecute() bool {
	return o.Kind == OutcomeApproved || o.Kind == OutcomeCorrected
}

// Escalated — 是否交回上层（**不是**可以喂给模型的结果）。
func (o ToolCallOutcome) Escalated() bool { return o.Kind == OutcomeEscalated }

// TargetTool — 真正要执行（或被充当）的工具名：更正换过工具时是新名字，否则是原名字。
func (o ToolCallOutcome) TargetTool() string {
	if o.Kind == OutcomeCorrected && o.EditedAction != nil {
		if n := NormalizeToolName(o.EditedAction.EditedAction.Name); n != "" {
			return n
		}
	}
	return o.Tool
}

// ── 输入 ────────────────────────────────────────────────────────────────────

// HumanDecision — 人做的一个决定（字段全部导出：UI/审计要能如实回显「谁、批了什么、改了什么」）。
type HumanDecision struct {
	// Kind — 五态之一（认不出 ⇒ 报错，fail-closed）。
	Kind DecisionKind
	// Tool — 这个决定指向哪把工具（必须与挂单上的工具名一致，否则视为批错对象）。
	Tool string
	// ArgsDigest — 决定所依据那次调用的**入参指纹**（必须与挂单一致）。
	// 算法口径由接线批统一（audit.ArgsFingerprint）；本包只做逐字比对，不解释它。
	ArgsDigest string
	// Response — `respond` 的内容：人充当工具交回来的那份结果（**原样**喂给模型）。
	Response string
	// Rejection — `reject` 的反馈（人给模型的话；也可以承载 `escalate` 时人要带给上层的话）。
	Rejection string
	// EditedAction — `correct` 的新动作（标准形态见 EditedAction）。
	EditedAction *EditedAction
	// EditedBy — **谁改的**（`correct` 必填：留痕的地基）。
	EditedBy string
	// Reason — 低基数原因码（可聚合）。`reject`/`escalate` 至少要给「为什么」（本字段或 Rejection/Response 之一）；
	// 留空时按 Kind 取默认（human_respond / human_reject / …）。
	Reason Reason
}

// ToolFacts — 判定「人这次的答复翻译成什么」所需的**最小事实**。
//
//	Tool          —— 工具名（可空；非空时必须与挂单上的工具名一致，否则对不上号 ⇒ 报错）
//	HasSideEffect —— 这次调用**会不会改变机器状态**（写文件 / 删 / 发请求 / 装包 / 执行命令…）
//
// **给不出来就别给**：省略 facts = 不知道有没有副作用 ⇒ 按「有」处理（见 Materialize ②）。
type ToolFacts struct {
	Tool          string
	HasSideEffect bool
}

// ── 唯一入口 ────────────────────────────────────────────────────────────────

// Materialize — 把（决定 + 挂单）翻译成给模型的交接单。**纯函数**：不看时间、不读 IO、不改挂单。
//
// 校验不过一律**报错并返回零值 outcome**（零值 = 接线批拿不到任何可以误当结果的东西，fail-closed）。
// 唯一的例外是 `escalate`：它返回**非零值 outcome + 包装了 ErrEscalated 的错误** ——
// 「交回上层」既不是普通结果、也不是普通错误，接线批必须用 `errors.Is(err, ErrEscalated)` 显式接住。
//
// `facts` 可省略（省略 = 按「有副作用」处理）。给两份或以上 ⇒ 报错（不猜哪一份是真的）。
func Materialize(d HumanDecision, p Pending, facts ...ToolFacts) (ToolCallOutcome, error) {
	// ① 五态认不出 ⇒ 报错（fail-closed，不猜语义）。
	if !d.Kind.Valid() {
		return ToolCallOutcome{}, fmt.Errorf("%w：决定「%s」非法（只认 respond|reject|approve|correct|escalate）"+
			"⇒ 不产出任何结果（fail-closed）", ErrUnknownDecisionKind, string(d.Kind))
	}
	if len(facts) > 1 {
		return ToolCallOutcome{}, fmt.Errorf("%w：工具事实只能给一份（收到 %d 份）⇒ 不猜哪一份是真的",
			ErrInvalidDecision, len(facts))
	}

	// ③ 批错对象：工具名与参数指纹都必须逐字对上挂单。
	tool := NormalizeToolName(p.Tool)
	if tool == "" {
		return ToolCallOutcome{}, fmt.Errorf("%w：挂单上的工具名归一化后为空 ⇒ 回指不到批的是哪把工具",
			ErrInvalidDecision)
	}
	if got := NormalizeToolName(d.Tool); got != tool {
		return ToolCallOutcome{}, fmt.Errorf("%w：决定指向的工具「%s」与挂单上的「%s」不一致 ⇒ 批错对象",
			ErrInvalidDecision, strings.TrimSpace(d.Tool), tool)
	}
	if p.ArgsDigest != "" {
		if d.ArgsDigest == "" {
			return ToolCallOutcome{}, fmt.Errorf("%w：挂单带参数指纹（%s），决定里没写 ⇒ 无法确认批的是同一次调用",
				ErrInvalidDecision, p.ArgsDigest)
		}
		if d.ArgsDigest != p.ArgsDigest {
			return ToolCallOutcome{}, fmt.Errorf("%w：决定的参数指纹（%s）与挂单（%s）不一致 ⇒ 批错对象"+
				"（与 grants 的逐字匹配同一纪律：批准 a 不许覆盖 b）", ErrInvalidDecision, d.ArgsDigest, p.ArgsDigest)
		}
	} else if d.ArgsDigest != "" {
		return ToolCallOutcome{}, fmt.Errorf("%w：挂单没有参数指纹，而决定里带了（%s）⇒ 回指不到（不猜）",
			ErrInvalidDecision, d.ArgsDigest)
	}

	// ② 副作用事实：**缺省按「有」**（不知道 ⇒ 不给「宣布成功」的权利）。给的事实必须说的是同一把工具。
	sideEffect := true
	if len(facts) == 1 {
		if ft := NormalizeToolName(facts[0].Tool); ft != "" && ft != tool {
			return ToolCallOutcome{}, fmt.Errorf("%w：工具事实说的是「%s」，挂单上是「%s」⇒ 对不上号",
				ErrInvalidDecision, ft, tool)
		}
		sideEffect = facts[0].HasSideEffect
	}

	// ④ 不许替人签字：必须已答复；已按超时结算的一律拒绝（迟到的答复不改变结局）。
	//    唯一例外是 escalate —— 它不产出任何给模型的东西，故不冒充谁签了字。
	switch p.State {
	case StatusAnswered:
		// 好：有真实答复支撑这次物化。
	case StatusOpen:
		if d.Kind != DecisionEscalate {
			return ToolCallOutcome{}, fmt.Errorf("%w：挂单 %s 还没人答复（open）⇒ 不许替人签字"+
				"（能物化的只有「真的发生过的答复」）", ErrInvalidDecision, p.ID)
		}
	case StatusTimedOut:
		return ToolCallOutcome{}, fmt.Errorf("%w：挂单 %s 已按超时结算 ⇒ 迟到的答复不改变结局"+
			"（超时那一刻的事实已经生效；要做就重新开单）", ErrInvalidDecision, p.ID)
	default:
		return ToolCallOutcome{}, fmt.Errorf("%w：挂单 %s 的状态「%s」认不出 ⇒ 不物化（fail-closed）",
			ErrInvalidDecision, p.ID, string(p.State))
	}

	// ⑤ 决定与账本里记的答复必须同族（respond/escalate 在 Answer 枚举里没有对应值 ⇒ 不查，但会留痕）。
	if want, ok := ledgerAnswerFor(d.Kind); ok && p.State == StatusAnswered && p.Answer != want {
		return ToolCallOutcome{}, fmt.Errorf("%w：决定是 %s，而账本里记的答复是「%s」（应同族：「%s」）⇒ 对不上",
			ErrInvalidDecision, string(d.Kind), string(p.Answer), string(want))
	}

	// 逐态的前置校验（先全部校验、再组装：报错时必须返回**零值** outcome）。
	switch d.Kind {
	case DecisionRespond:
		if sideEffect {
			return ToolCallOutcome{}, fmt.Errorf("%w：工具「%s」有副作用，而 respond 的语义是「人充当工具、**不执行**」"+
				"⇒ 那会把「没做」说成「做成了」（污染认知）。要拒绝请用 reject（它的语义就是「不执行 + 反馈」）",
				ErrRespondOnSideEffect, tool)
		}
		if strings.TrimSpace(d.Response) == "" {
			return ToolCallOutcome{}, fmt.Errorf("%w：respond 的内容为空 ⇒ 成功型工具结果不能是空的"+
				"（模型只会收到一条空结果，反而更容易瞎猜）", ErrInvalidDecision)
		}
	case DecisionReject:
		if strings.TrimSpace(d.Rejection) == "" && strings.TrimSpace(string(d.Reason)) == "" {
			return ToolCallOutcome{}, fmt.Errorf("%w：reject 必须说明「为什么」（Rejection 与 Reason 至少给一个）"+
				"⇒ 不解释的拒绝会让模型反复试同一条路", ErrInvalidDecision)
		}
	case DecisionCorrect:
		if d.EditedAction == nil {
			return ToolCallOutcome{}, fmt.Errorf("%w：correct 必须带 EditedAction"+
				"（标准形态 {\"type\":\"edit\",\"edited_action\":{…}}）⇒ 没有新参数就无从更正", ErrInvalidDecision)
		}
		if t := strings.ToLower(strings.TrimSpace(d.EditedAction.Type)); t != EditActionType {
			return ToolCallOutcome{}, fmt.Errorf("%w：EditedAction.type 是「%s」⇒ 只认 %q（fail-closed，不猜形态）",
				ErrInvalidDecision, strings.TrimSpace(d.EditedAction.Type), EditActionType)
		}
		if NormalizeToolName(d.EditedAction.EditedAction.Name) == "" {
			return ToolCallOutcome{}, fmt.Errorf("%w：更正后的工具名为空 ⇒ 不知道要执行什么", ErrInvalidDecision)
		}
		if strings.TrimSpace(d.EditedBy) == "" {
			return ToolCallOutcome{}, fmt.Errorf("%w：correct 必须带 EditedBy（**谁改的**）⇒ 说不清改动人的更正不算留痕",
				ErrInvalidDecision)
		}
	case DecisionApprove:
		// 无额外前置：批准就是放行执行。**副作用事实在这条路上不参与判定** ——
		// 闸轨的用途正是批有副作用的动作（否则这道闸没有存在意义）。
	case DecisionEscalate:
		// escalate 不产出给模型的东西（故不查账本答复），但**必须带上交的理由或人的话** ——
		// 否则上层接到的是一张白纸（那与「默默丢弃」只差一个字）。
		if strings.TrimSpace(string(d.Reason)) == "" && strings.TrimSpace(d.Rejection) == "" && strings.TrimSpace(d.Response) == "" {
			return ToolCallOutcome{}, fmt.Errorf("%w：escalate 必须说明「为什么上交」或带上人的话"+
				"（Reason/Rejection/Response 至少给一个）", ErrInvalidDecision)
		}
	}

	// ── 组装：全分支共有的审计痕迹先落地，再按五态补差异部分。──
	o := ToolCallOutcome{
		Tool:         tool,
		Decision:     d.Kind,
		PendingID:    p.ID,
		SessionID:    p.SessionID,
		RuleID:       p.RuleID,
		ArgsDigest:   p.ArgsDigest,
		LedgerState:  p.State,
		LedgerAnswer: p.Answer,
		Reason:       decisionReason(d),
	}

	switch d.Kind {
	case DecisionRespond:
		o.Kind = OutcomeResponded
		// ★ 这里**一字都不许**注入拒绝/失败类措辞：正文是人交回来的那份结果**原样**。
		//   （respond = 人充当工具 ⇒ 对模型而言这就是一次成功的工具调用；工具本体一步也没跑。）
		o.Result = ToolResult{Emitted: true, Kind: ResultSuccess, Content: d.Response}
		o.Note = fmt.Sprintf("挂单 %s（工具 %s）：人**充当工具**给出结果 ⇒ 工具不执行；内容原样返回给模型（成败=成功）",
			p.ID, tool)

	case DecisionReject:
		o.Kind = OutcomeRejected
		o.Result = ToolResult{
			Emitted:   true,
			Kind:      ResultFailed,
			Content:   rejectContent(tool, d),
			ModelNote: RejectModelNote,
		}
		o.Note = fmt.Sprintf("挂单 %s（工具 %s）：人**拒绝** ⇒ **工具不执行**（副作用的工具也不执行）；"+
			"结果按失败型交给模型（原因：%s）", p.ID, tool, string(o.Reason))

	case DecisionApprove:
		o.Kind = OutcomeApproved
		o.Note = fmt.Sprintf("挂单 %s（工具 %s）：人**批准** ⇒ 放行执行（真实工具结果稍后产出，本 outcome 不含给模型的结果）",
			p.ID, tool)

	case DecisionCorrect:
		o.Kind = OutcomeCorrected
		o.EditedAction = d.EditedAction
		o.EditedBy = strings.TrimSpace(d.EditedBy)
		newTool := NormalizeToolName(d.EditedAction.EditedAction.Name)
		o.ToolChanged = newTool != tool
		o.NewArgsLocalDigest = localArgsDigest(d.EditedAction.EditedAction.Args)
		o.Warning = CorrectReplanWarning
		o.Note = fmt.Sprintf("挂单 %s（工具 %s）：人**更正** ⇒ 按新参数放行执行（原文留痕：args_digest=%s；改动人=%s）；%s",
			p.ID, tool, p.ArgsDigest, o.EditedBy, CorrectReplanWarning)
		if o.ToolChanged {
			o.Note += fmt.Sprintf("；⚠ 更正把工具名也换了（%s ⇒ %s）⇒ **必须重新过一遍策略闸**（原批准只覆盖原参数、更覆盖不了另一把工具）",
				tool, newTool)
		}

	case DecisionEscalate:
		o.Kind = OutcomeEscalated
		// 上交：**不给模型任何东西**（Result 保持零值），人的话只进给人/审计看的 Note。
		o.Note = escalateNote(p, d)
		// 「交回上层」是**显式决定**：返回专项错误 ⇒ 接线批不可能把它当成一条普通结果（也就不会被默默丢弃）。
		return o, fmt.Errorf("%w：挂单 %s（工具 %s）已交由上层处置 —— 本条**不得**喂给模型，也**不得**丢弃",
			ErrEscalated, p.ID, tool)
	}
	return o, nil
}

// ledgerAnswerFor — 决定对应的账本答复（pending.go 的 Answer 枚举）。
// 第二个返回值为 false = 这个决定在账本里**没有对应值**（respond 没有「我替你答」这一档；escalate 落在账本之外），
// 此时不查同族，但账本答复仍会原样抄进 outcome（LedgerAnswer）供审计对照。
func ledgerAnswerFor(k DecisionKind) (Answer, bool) {
	switch k {
	case DecisionApprove:
		return AnswerApprove, true
	case DecisionReject:
		return AnswerReject, true
	case DecisionCorrect:
		return AnswerCorrect, true
	}
	return "", false
}

// decisionReason — 低基数原因码：调用方给了就用它，否则按五态取默认（不编自由文本）。
func decisionReason(d HumanDecision) Reason {
	if r := Reason(strings.TrimSpace(string(d.Reason))); r != "" {
		return r
	}
	switch d.Kind {
	case DecisionRespond:
		return ReasonHumanRespond
	case DecisionReject:
		return ReasonHumanReject
	case DecisionApprove:
		return ReasonHumanApprove
	case DecisionCorrect:
		return ReasonHumanCorrect
	case DecisionEscalate:
		return ReasonHumanEscalate
	}
	return ""
}

// rejectContent — reject 的结果正文：**未执行 + 为什么 + 不要重试**，三样缺一不可（用例②钉住）。
func rejectContent(tool string, d HumanDecision) string {
	s := fmt.Sprintf("【未执行】工具 %s 没有被执行（人拒绝了这次调用）。原因：%s。", tool, string(decisionReason(d)))
	if fb := strings.TrimSpace(d.Rejection); fb != "" {
		s += "人的反馈：" + fb + "。"
	}
	return s + "**不要重试**这次调用，也不要把它的结果当成已经发生的事实。"
}

// escalateNote — escalate 的审计说明（人的话与理由都留在**人可读**字段里，不进给模型的字段）。
func escalateNote(p Pending, d HumanDecision) string {
	s := fmt.Sprintf("挂单 %s（工具 %s）：人**上交上层** ⇒ 不执行、不产出给模型的结果；"+
		"接线批必须显式接手（本函数返回 ErrEscalated，不许当成普通结果，也不许丢弃）", p.ID, NormalizeToolName(p.Tool))
	if r := strings.TrimSpace(string(d.Reason)); r != "" {
		s += "；理由：" + r
	}
	if fb := strings.TrimSpace(d.Rejection); fb != "" {
		s += "；人带给上层的话：" + fb
	}
	if t := strings.TrimSpace(d.Response); t != "" {
		s += "；人的补充：" + t
	}
	return s
}

// localArgsDigest — **本包口径**的规范化参数指纹：`json.Marshal`（map 键已排序 ⇒ 同内容必然同字节）
// 后取 sha256 的前 8 字节十六进制；前缀 `sha256local:` 明示它**不是**审计侧那个权威指纹。
// 算不出（参数为空 / 含不可序列化的值）⇒ **显式空串**（不编一个看起来像指纹的东西）。
func localArgsDigest(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	b, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return "sha256local:" + hex.EncodeToString(sum[:8])
}
