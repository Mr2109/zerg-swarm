// escalation_signal.go — B 项⑥：**升级信号闭集**（设计稿 v2.1 §4.7 逐字）+「不在闭集内必须给理由码」的校验
// + 升级事件的唯一入口（沿用**同一套**片事件机制，见 sliceobs.go 的包注释）。
//
// ── 逐字来源（本件现读，不凭记忆）────────────────────────────────────────
//
//	docs/01-设计/设计-协作骨架-v2.1.md 第 255 行（§4.7 打回与四态（含超时）/ fail closed）：
//
//	  「- **升级信号闭集**（不在枚举内的「无需升级」必须给理由码）：① 不可逆/删除/越权 ② 两轮无进展或预算耗尽
//	    ③ 自报低置信或自相矛盾 ④ 输入含可疑指令（注入）⑤ 触网/外发 ⑥ 同一片二次打回。」
//
//	同句在《Zerg-内部文档/项目文档/v2.5.10/任务表-A路批次计划-20260918.md（2026-09-19 二次分家）》里被拆成 §4.7-9a…§4.7-9f（六条信号，
//	逐条同字）+ §4.7-9g（**不在枚举内的「无需升级」必须给理由码**，判据「理由码字段可判」）。
//
//	⇒ 六条**逐字**落成下面的常量（顺序 = 稿面 ①→⑥ 顺序）；**不自增、不自删、不改字**（硬规则 ✗）。
//
// ── 编辑口径登记（逐字之外只做过这一处，写在这里、也写进回报）──────────────
//
//	末条在稿面行文里是「同一片二次打回**。**」——**句末句号是句读，不属于条目本身**（前五条稿面本就无句号），
//	故常量值取「同一片二次打回」。除此之外**一个字符没动**（含全角括号、ASCII 斜杠 `/`）。
//
// ── 与本包「事件名闭集」的关系：**两个不同的闭集，都闭，且互不派生**──────────
//
//	· 事件名闭集（§6.1 四名）= `event` 字段的值域 —— 本件**不动它**（仍是四名，**没有**第五个）；
//	· 升级信号闭集（§4.7 六条）= `signal` 字段的值域 —— 本件新增的就是它。
//	信号**不是**事件名，事件名**不是**信号：`EscalationSignals()` 与 `EventNames()` 的交集必须为空
//	（用例 `TestEscalationSignals_DisjointFromEventNameClosedSet` 钉住这条，防两个闭集被串成一个名字空间）。
//
// ── 校验口径（§4.7-9g 逐字：不在枚举内的「无需升级」必须给理由码）──────────
//
//	· 信号**在**闭集内 ⇒ **升级**（§4.12：「`escalate_if` 与 §4.7 升级信号闭集 ⇒ **ask/升级档**」）
//	  ⇒ **不要求**理由码（带了也照收，见下）；
//	· 信号**不在**闭集内（**含空串** = 一条都没命中 ⇒ 即稿面说的「无需升级」）⇒ **必须**给理由码；
//	  缺（去空白后为空）⇒ `ErrMissingEscalationReasonCode`（**不默认放行、不默认成某条信号、不猜** ✗）。
//	两种情形各有用例（正反两侧都钉住，见 escalation_signal_test.go）。
//
// ── 理由码的**语法**：本件**不做**格式校验（登记口径，不替设计稿发明语法）────────
//
//	设计稿只说「必须给理由码」，**没给理由码的语法**（对比 §4.7 的打回原因码：稿面明写
//	`PRE_/POST_/INV_` 前缀 + R 编号 ⇒ 那套在 `core/internal/policy/sendback_reason.go` 里有严格解析与校验）。
//	⇒ 本件只要求**非空**，**不发明**一套编号体系；将来稿面给出语法，再在此处加校验（只改这一个函数）。
//
// ── 观测口径（本仓口径 —— 稿面没写「观测面拿到非法信号该怎么办」，这里写死并回报）──
//
//	升级事件（`slice_escalated`）**必须**带闭集内的信号（否则「升级」这件事无从解释）；
//	信号不在闭集内 ⇒ 与**事件名闭集**同规（sliceobs.go 硬规则 ①「认不出的不落」）：
//	**记日志、不落这一行** —— 绝不猜成某一条信号，也绝不落一条无法解释的升级行。
//	「不落行」**不改任何判定**（`Emit` 无返回值、本函数也无返回值）⇒ 与 fail-closed 同向：
//	不静默通过，但**也绝不伪造记录**（谁都没法靠观测面把非法信号洗白）。
//
// ── 本件**不做**（不编造触发条件；登记为未实现项）────────────────────────────
//
//	· **不做信号检测**：本文件只判「调用方报上来的信号是不是闭集内的那六条」，**不推断**
//	  「什么情况算不可逆 / 两轮无进展 / 注入 / 触网 / 二次打回」——那些检测链路（`side_effects` 枚举、
//	  停滞计数、confidence 自报字段、注入检测、外发判定、同原因码计数）本仓**都还没有落点**
//	  （登记见 Zerg-内部文档/项目文档/v2.5.10/任务表-协作骨架v2.1仓内核对结论-20260918.md 的 B7 / §3.2-inv2）。（2026-09-19 二次分家）
//	· **不加第七个结局值**：信号触发的升级沿用既有低基数结局 `OutcomeRaised`（升级类结局；
//	  §6.1 未给第七个结局值 ⇒ 不新增，见 EmitSliceEscalatedSignal 注释）。
package sliceobs

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

// ── 升级信号闭集（设计稿 v2.1 §4.7 第 255 行**逐字**；六条，不许自造第七个）─────

const (
	// SignalIrreversibleOrDeleteOrPrivilege — ① 不可逆/删除/越权。
	SignalIrreversibleOrDeleteOrPrivilege = "不可逆/删除/越权"
	// SignalTwoRoundsNoProgressOrBudgetExhausted — ② 两轮无进展或预算耗尽。
	SignalTwoRoundsNoProgressOrBudgetExhausted = "两轮无进展或预算耗尽"
	// SignalSelfReportedLowConfidenceOrSelfContradiction — ③ 自报低置信或自相矛盾。
	SignalSelfReportedLowConfidenceOrSelfContradiction = "自报低置信或自相矛盾"
	// SignalSuspiciousInstructionInInput — ④ 输入含可疑指令（注入）。
	SignalSuspiciousInstructionInInput = "输入含可疑指令（注入）"
	// SignalNetworkOrOutbound — ⑤ 触网/外发。
	SignalNetworkOrOutbound = "触网/外发"
	// SignalSecondSendbackOfSameSlice — ⑥ 同一片二次打回。
	SignalSecondSendbackOfSameSlice = "同一片二次打回"
)

// EscalationSignals — 升级信号闭集的枚举形态（统计脚本与用例按它遍历；**顺序固定** = 稿面 ①→⑥ 的行文顺序）。
func EscalationSignals() []string {
	return []string{
		SignalIrreversibleOrDeleteOrPrivilege,
		SignalTwoRoundsNoProgressOrBudgetExhausted,
		SignalSelfReportedLowConfidenceOrSelfContradiction,
		SignalSuspiciousInstructionInInput,
		SignalNetworkOrOutbound,
		SignalSecondSendbackOfSameSlice,
	}
}

// knownEscalationSignal — 信号是否在闭集内。**逐字比较**：不归一化、不做模糊匹配、不认近义词
// （「不可逆 或 删除」/「不可逆/删除/越权 」这类写法一律**不**算命中 —— 认不出时不猜）。
func knownEscalationSignal(signal string) bool {
	for _, s := range EscalationSignals() {
		if signal == s {
			return true
		}
	}
	return false
}

// IsEscalationSignal — 闭集判据的导出形态（调用方与统计脚本用；内部同一实现，避免两份）。
func IsEscalationSignal(signal string) bool { return knownEscalationSignal(signal) }

// ── 专项错误（可判：调用方按 errors.Is 分派，不解析错误原文）──────────────────

var (
	// ErrMissingEscalationReasonCode — 信号不在闭集内（= 稿面的「无需升级」）却没给理由码。
	// §4.7-9g 逐字：「不在枚举内的「无需升级」必须给理由码」⇒ 缺理由码的裁决**不可采信**。
	ErrMissingEscalationReasonCode = errors.New("升级信号不在 §4.7 闭集内（=「无需升级」）却缺理由码")
)

// ── 校验（§4.7-9g 的唯一实现点）──────────────────────────────────────────

// ValidateEscalationSignal — 「这次升级/无需升级的裁决能不能采信」的唯一判据：
//
//	· 信号在闭集内 ⇒ **升级** ⇒ 通过（§4.7 不要求理由码；带了也不报错 —— 理由码是补充理由，不是必需项）；
//	· 信号不在闭集内（含空串）⇒ **无需升级** ⇒ 理由码必须非空，否则 `ErrMissingEscalationReasonCode`。
//
// 纯函数：不读时间、不读 IO、不落盘。信号按**逐字**比较（首尾空白**不**修剪 —— 修剪等于悄悄放宽闭集）。
func ValidateEscalationSignal(signal, reasonCode string) error {
	if IsEscalationSignal(signal) {
		return nil
	}
	if strings.TrimSpace(reasonCode) == "" {
		return fmt.Errorf("%w：信号 %q（不在 %s 之内 ⇒ 本裁决属「无需升级」）；"+
			"§4.7-9g 逐字「不在枚举内的「无需升级」必须给理由码」⇒ 本裁决**不可采信**"+
			"（**不默认放行、不默认成闭集里的某一条信号**）",
			ErrMissingEscalationReasonCode, signal, escalationSignalsText())
	}
	return nil
}

// escalationSignalsText — 闭集的可读形态（错误信息里给「合法的值域」—— 错误必须可行动）。
func escalationSignalsText() string {
	return "§4.7 升级信号闭集 " + strings.Join(EscalationSignals(), " · ")
}

// ── 裁决（判据面；「该信号⇒升级」这一步是**闭集判定**，不是检测）──────────────

// EscalationVerdict — 一次升级信号求值的结果（§4.7-9g 的判据面形态）。
//
// 语义（全部来自 §4.7 / §4.12，**不推断**任何检测条件）：
//
//	Signal 在闭集内 ⇒ Escalate = true（六条信号 ⇒ ask/升级档）· ReasonCode 可空；
//	Signal 不在闭集内（含空串）⇒ Escalate = false（=「无需升级」）· ReasonCode **必非空**。
type EscalationVerdict struct {
	// Signal — 命中的升级信号（闭集内逐字取值）；空串 = 一条都没命中。
	Signal string `json:"signal"`
	// ReasonCode — 「无需升级」的理由码（Signal 不在闭集内时**必填**）。
	ReasonCode string `json:"reason_code,omitempty"`
	// Escalate — 是否升级（= Signal 在闭集内）。由闭集判定，**不由任何启发式**决定。
	Escalate bool `json:"escalate"`
}

// NewEscalationVerdict — 构造并**当场校验**（构造侧唯一的正规入口：不可能造出一个过不了
// ValidateEscalationSignal 的裁决）。
func NewEscalationVerdict(signal, reasonCode string) (EscalationVerdict, error) {
	if err := ValidateEscalationSignal(signal, reasonCode); err != nil {
		return EscalationVerdict{}, err
	}
	return EscalationVerdict{
		Signal:     signal,
		ReasonCode: strings.TrimSpace(reasonCode),
		Escalate:   IsEscalationSignal(signal),
	}, nil
}

// ── 升级事件的唯一入口（**同一套**片事件机制：唯一写入出口仍是 Emit）──────────

// EmitSliceEscalatedSignal — 「信号触发的升级」落一行 `slice_escalated`（§6.1 闭合四名里的第三名）。
//
// ── 参数口径 ──
//
//	signal        = **必须**是闭集内那六条之一（逐字）；不在闭集内 ⇒ **不落行**（见下）。
//	reasonCode    = 可选（闭集内信号**不要求**理由码；给了就如实落进 `reason_code`）。
//	criteriaVersion = 判定这次升级所依据的规则集版本串（指向模板/检查器版本）；
//	                  **空串 ⇒ 写显式「未标定」**（由 Emit 兜底），绝不编造一个看着像版本号的串。
//
// 只落低基数值（片 id / task_id / 信号 / 理由码 / R 编号）：detail 由本函数用固定句式拼出，
// **不接受调用方的自由文本** —— 免得把请求体原文 / 凭据 / 私有路径带进观测面（硬规则 ④）。
//
// ── 为什么不在闭集内就**不落行**（本仓口径，与事件名闭集同规）──
//
//	`slice_escalated` 这个事件名本身就断言「发生了升级」；若它带的信号不在 §4.7 六条里，
//	这行记录**无从解释**（既不能聚合，也不能归因）⇒ 与硬规则 ①「认不出的不落」同一处置：
//	记日志、不落行。**不落行不改变任何判定**（本函数与 Emit 都没有返回值）。
//
// ── 结局取值（本仓口径）──
//
//	沿用既有低基数结局 `OutcomeRaised`（「升级类」结局；§6.1 只闭了事件名，没给第七个结局值
//	⇒ 本件**不新增**结局值，也**不**复用 `OutcomeRefused`/`OutcomeTimeout`（那两个是第 4 态
//	「上游拒答」「到点未答」的专属结局，语义不同 —— 混用会把低基数结局稀释掉）。
func EmitSliceEscalatedSignal(sliceID, taskID, signal, reasonCode, criteriaVersion string, r *int) {
	// **不修剪信号**：闭集是逐字的 ⇒ 带首尾空白 / 带句号的写法**不是**闭集里的条目
	// （修剪会把「写法不对」洗成「对了」，等于悄悄放宽闭集 ⇒ 与「认不出不猜」相反）。
	if !IsEscalationSignal(signal) {
		log.Printf("⚠️ sliceobs: 升级信号认不出（不在 §4.7 六条闭集内）⇒ 不落这一行：signal=%q reason_code=%q "+
			"（与「事件名认不出不落」同规；判定不受影响）", signal, strings.TrimSpace(reasonCode))
		return
	}
	sliceID = strings.TrimSpace(sliceID)
	taskID = strings.TrimSpace(taskID)
	reasonCode = strings.TrimSpace(reasonCode)

	detail := fmt.Sprintf("片 %s 命中**升级信号**「%s」（§4.7 六条闭集之一）⇒ **升级给人/上游**："+
		"§4.12 口径「`escalate_if` 与 §4.7 升级信号闭集 ⇒ **ask/升级档**」；本行 outcome=%s（升级类）⇒ "+
		"片**不放行**（不静默通过、不无限等待）。", sliceID, signal, OutcomeRaised)
	if reasonCode != "" {
		detail += fmt.Sprintf("附理由码「%s」（闭集内信号不要求理由码，这里是调用方给的补充理由）。", reasonCode)
	}
	if sliceID == "" {
		detail += "（本行缺 slice_id ⇒ 回指不到是哪一片，属观测缺陷，须补齐。）"
	}

	Emit(Event{
		Event:           EventSliceEscalated,
		OK:              false,
		Outcome:         OutcomeRaised,
		SliceID:         sliceID,
		TaskID:          taskID,
		Signal:          signal,     // 闭集内逐字取值（上面已校验）
		ReasonCode:      reasonCode, // 可空（§4.7 只对「无需升级」要求理由码）
		R:               r,
		CriteriaVersion: criteriaVersion, // 空 ⇒ Emit 写「未标定」
		Detail:          detail,
	})
}
