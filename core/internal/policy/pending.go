// pending.go — T5.8「长暂停」：**持久化的开放交互对象** + **逐类超时语义**（不靠挂住进程等人）。
//
// 要治的缺口（设计稿 docs/01-设计/设计-内建调试版-v1.2.md §〇 F12）：
// **"长暂停"缺工程手段** —— 一次审批/提问可能停放几分钟到几天（人下班了、在开会、在另一个时区），
// 而进程**不能挂着线程等人**：重启、容器回收、网关超时、连接断开，都会把"挂着等"变成"永远等不到"，
// 而且挂着的进程什么都说不清（既枚举不出"还有哪些没答"，也无法在新进程里接着答）。
//
// 工程手段两条（F12）：**defer**（进程先退出，之后**从持久化会话恢复**）+
// **持久化开放交互对象**（可枚举未答项）。本文件做的是第二条：
//
//	`Pending` —— 一等公民的"挂单"：谁问的（会话）/ 问什么（工具 + 入参指纹）/ 凭什么问（rule_id）/
//	             什么时候开的、什么时候到期、现在是什么状态；可枚举、可序列化、**跨重启存活**。
//
// ── 三类挂单 × 逐类超时语义（**写死在代码里**，由用例逐条钉住；`timeoutOutcome` 是唯一实现处）──
//
//	kind           超时结果（Result）      对工具判定（Effect）  给模型的话（ModelNote）
//	tool_approval  reject（=拒绝）        deny（**绝不是 allow**）"批准超时 ⇒ 拒绝，超时绝不被读作同意"
//	ask_user       continue（继续）        ——（不涉及工具判定）   "无人应答"
//	interaction    cancel（按取消处理）    ——（不涉及工具判定）   "交互点已取消，不得当作已确认"
//
// **本文件最重要的那一句：批准超时绝不被读作同意。**
// 超时 = 没有人同意过。把它读成 allow（"等了半天没人反对，那就做吧"）等于**把沉默当签字**，
// 而沉默恰恰是无人值守场景下唯一必然出现的东西 ⇒ 于是"批准"这道闸在最需要它的时候变成放行。
// 因此 tool_approval 的超时结局写死为 `EffectDeny`（与 T5.1 的 fail-closed 同向：判不了 ⇒ 不放行）。
// 另两类**不涉及工具判定**（它们的 Effect 为空串，不是 allow）：ask_user 超时是"我没问到人"（继续），
// 交互点超时是"这个交互作废"（取消）。三类都不许被读成"同意"。
//
// ── 六条 fail-closed 语义（每条都有用例钉住）──
//
//	① **到点即超时**：判据是 `now.Before(ExpiresAt)` ⇒ now 恰等于到期时刻也算超时；
//	   且**不依赖 `Expire` 被调用** —— `Answer` 自己会先把到点的挂单结算掉。
//	   （否则"挂单过期了但没人扫"就能被后来的批准翻案，超时语义形同虚设。）
//	② **超时后迟到的答复不改变结局**：已按超时结算的挂单再 `Answer` ⇒ **报错**，状态一字不改
//	   （批准超时=拒绝这条一旦生效就不可回溯）。
//	③ **不可重答**：已答复的挂单再 `Answer` ⇒ 报错（第二个人不许改第一个人的话）。
//	④ **不许无名挂单**：工具名/交互点名归一化后为空 ⇒ 拒绝开单（无名的挂单枚举不出、回指不到）。
//	⑤ **工具批准必须带 ExpiresAt**（零值 ⇒ 拒绝开单）：没人答的批准挂单不设期限 = 永久挂着
//	   ⇒ 在无人值守场景下既等不到人也不判拒绝。另两类可以零值（= 不设期限，人工会话允许无限等）。
//	⑥ **`correct`（更正参数，F10）只对工具批准有意义**：对 ask_user / 交互点用 `correct` ⇒ 报错
//	   （不猜语义：更正一个"提问"没有定义）。
//
// ── 序列化与重启（F12 的"持久化"）──
//
//	`Snapshot()` / `MarshalJSON()` 写；`LoadPending()` 读。载入侧 fail-closed：
//	  · 版本不认识 / JSON 坏 / 状态或类别取值认不出 / 编号水位不自洽 ⇒ **报错**（绝不当"空账本"接着跑）
//	  · 载入时点已到期的 open 项 ⇒ **立刻按超时结算**（状态落成 timed_out 并计数），
//	    **不复活成"还能答"** —— 重启这段时间也是时间，期间到期就该按超时算
//	  · 已答 / 已超时的项原样保留（它们是**发生过的事实**，不该在重启时被抹掉）
//
// ── 本批边界（与 T5.1/T5.2/T5.3/T5.4 同一纪律）──
//
//	· 叶子包：只 import 标准库（encoding/json / fmt / sort / strings / sync / time），零内部依赖。
//	· **不接任何执行路径**：不碰 toolobs / chat / gateway / agent（一行不动）。本文件只提供
//	  "开单 / 答复 / 枚举 / 结算"的本体；`defer` 调度、落盘路径、弹窗 UI 属接线批。
//	· **判定不被本文件改写**：超时结局是**给接线批用的事实**（Effect=deny / Result=reject…），
//	  本包不替调用方去改任何 Verdict（那是接线批的显式动作，且在地板命中时永远不生效 —— T5.1）。
//	· 入参指纹（ArgsDigest）的**算法口径**不在本文件：由接线批统一（`core/internal/audit` 的
//	  ArgsFingerprint 是同一口径的实现）；policy 是叶包，不 import audit（见两包各自的纪律）。
package policy

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// PendingStateVersion — 挂单账本的序列化格式版本。读到的版本不认识 ⇒ **报错**
// （fail-closed：绝不按"空账本"载入 —— 那等于把没答的挂单一次静默丢掉，用户以为"没人问过我"）。
const PendingStateVersion = 1

// ── 类别（逐类超时语义的判据）────────────────────────────────────────────────

// PendingKind — 开放交互对象的类别（三类，无第四类）。
type PendingKind string

const (
	// PendingToolApproval — **工具批准**（闸轨：不依赖模型意愿的强制暂停，F9）。
	// 超时 ⇒ **拒绝**（Effect=deny）。
	PendingToolApproval PendingKind = "tool_approval"
	// PendingAskUser — **模型主动问人**（工具轨：模型自主提问，F9）。
	// 超时 ⇒ 告知模型"无人应答"后**继续**（不执行任何工具的"拒绝"，因为没有工具要批）。
	PendingAskUser PendingKind = "ask_user"
	// PendingInteraction — **交互点**（图执行里的 interrupt / 等待用户确认的节点，F8）。
	// 超时 ⇒ 按**取消**处理（本次交互作废，不得当作已确认）。
	PendingInteraction PendingKind = "interaction"
)

// Valid — 是否三类之一（认不出一律不猜，见 Open / LoadPending）。
func (k PendingKind) Valid() bool {
	switch k {
	case PendingToolApproval, PendingAskUser, PendingInteraction:
		return true
	}
	return false
}

// ParsePendingKind — 解析类别文本（大小写不敏感、去首尾空白）；认不出一律**报错**。
func ParsePendingKind(raw string) (PendingKind, error) {
	k := PendingKind(strings.ToLower(strings.TrimSpace(raw)))
	if !k.Valid() {
		return "", fmt.Errorf("未知挂单类别「%s」（只认 tool_approval|ask_user|interaction）", strings.TrimSpace(raw))
	}
	return k, nil
}

// ── 状态 ────────────────────────────────────────────────────────────────────

// PendingStatus — 挂单的状态（三态；**没有"已同意"这种状态** —— 同意是一种答复，不是超时结果）。
type PendingStatus string

const (
	// StatusOpen — 未答：还等着人（这是 `ListOpen` 枚举的对象）。
	StatusOpen PendingStatus = "open"
	// StatusAnswered — 已答：有人给了答复（**只能答一次**）。
	StatusAnswered PendingStatus = "answered"
	// StatusTimedOut — 已超时：按类别结算过（批准=拒绝 / 提问=无人应答 / 交互点=取消）。
	StatusTimedOut PendingStatus = "timed_out"
)

// Valid — 是否三态之一（序列化里读到别的取值 ⇒ 报错，不猜）。
func (s PendingStatus) Valid() bool {
	switch s {
	case StatusOpen, StatusAnswered, StatusTimedOut:
		return true
	}
	return false
}

// ── 答复 ────────────────────────────────────────────────────────────────────

// Answer — 人对挂单的答复（四类，无第五类）。
//
// 与设计稿 F11 的两轨对齐：`approve` 是"给你做"，`reject` 是"拒绝 + 反馈，工具**不执行**"；
// 它们不是同一个动作的两种说法（用"人充当工具返回成功"去表达拒绝 = 告诉模型"做成了"）。
type Answer string

const (
	// AnswerApprove — 同意执行 / 接受这个答复。
	AnswerApprove Answer = "approve"
	// AnswerReject — 拒绝：工具不执行（超时给的也是这一个）。
	AnswerReject Answer = "reject"
	// AnswerCorrect — **更正**（F10）：改参数后继续（只对工具批准有意义，见 Pending 的 ⑥）。
	AnswerCorrect Answer = "correct"
	// AnswerCancel — 取消：整个交互点作废（不执行、也不再等）。
	AnswerCancel Answer = "cancel"
)

// Valid — 是否四类之一。
func (a Answer) Valid() bool {
	switch a {
	case AnswerApprove, AnswerReject, AnswerCorrect, AnswerCancel:
		return true
	}
	return false
}

// ── 超时结局（逐类语义的唯一实现处）──────────────────────────────────────────

// TimeoutResult — 超时的**结局**（可枚举，接线批按它分派处置）。
type TimeoutResult string

const (
	// TimeoutReject — 按拒绝处理（工具不执行）。
	TimeoutReject TimeoutResult = "reject"
	// TimeoutContinue — 按"继续"处理（带上 ModelNote 继续往下走）。
	TimeoutContinue TimeoutResult = "continue"
	// TimeoutCancel — 按取消处理（本次交互作废）。
	TimeoutCancel TimeoutResult = "cancel"
)

// TimeoutOutcome — 一个挂单超时后的**结算单**（谁、哪类、什么结局、对工具判定是什么、给模型什么话）。
// 它是"事实"，不是"建议"：接线批必须照它走，不许把它读成别的（尤其不许把 TimeoutReject 读成放行）。
type TimeoutOutcome struct {
	PendingID string      `json:"pending_id"`
	Kind      PendingKind `json:"kind"`
	Tool      string      `json:"tool,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
	RuleID    string      `json:"rule_id,omitempty"`

	// Result — 超时结局：reject | continue | cancel。
	Result TimeoutResult `json:"result"`
	// Effect — 超时对**工具判定**等价于什么：tool_approval ⇒ **deny**（绝不是 allow）；
	// 另两类**不涉及工具判定** ⇒ 空串（空串不是 allow：它表示"没有工具判定这回事"）。
	Effect Effect `json:"effect,omitempty"`
	// Reason — 结局的原因码（低基数；只有 tool_approval 有判定语义，故其余类别为空串）。
	Reason Reason `json:"reason,omitempty"`
	// Answer — 超时等价于什么答复：reject / cancel；ask_user 无答复（空串）。
	Answer Answer `json:"answer,omitempty"`
	// ModelNote — **给模型看的话**（必须能独立读懂：模型只看到这一句）。
	ModelNote string `json:"model_note,omitempty"`
	// Note — 给人与审计看的一句话（含依据，可回查）。
	Note string `json:"note,omitempty"`
}

// timeoutOutcome — **逐类超时语义**（本文件唯一实现；`Expire` 与 `Answer` 都走它，不许各写一份）。
//
// 类别非法（理论上被 Open/LoadPending 挡住）：**按最严**走 —— 当批准超时处理（reject/deny）。
// 理由与 T5.1 同源：认不出的东西绝不按"最宽"解释。
func timeoutOutcome(p Pending) TimeoutOutcome {
	o := TimeoutOutcome{
		PendingID: p.ID,
		Kind:      p.Kind,
		Tool:      p.Tool,
		SessionID: p.SessionID,
		RuleID:    p.RuleID,
	}
	switch p.Kind {
	case PendingToolApproval:
		o.Result = TimeoutReject
		o.Effect = EffectDeny // ★ 批准超时 = 拒绝（`EffectDeny`，**绝不是 EffectAllow**）
		o.Reason = ReasonApprovalTimeout
		o.Answer = AnswerReject
		o.ModelNote = "工具批准等不到人（超时）⇒ 按【拒绝】处理，工具不会执行。" +
			"没有发生过任何「同意」——不要把沉默读作批准，也不要因为「没人反对」就重试。"
		o.Note = fmt.Sprintf("挂单 %s（工具 %s）到期未答 ⇒ 按拒绝结算（rule_id=%s）；**批准超时绝不被读作同意**",
			p.ID, p.Tool, ruleText(p.RuleID))
	case PendingAskUser:
		o.Result = TimeoutContinue
		o.Effect = "" // 不涉及工具判定（不写 allow：空串表示"没有工具判定这回事"）
		o.ModelNote = "无人应答（等不到用户的答复）⇒ 按【没有问到人】继续：" +
			"不要重试提问、不要假设得到了答案、也不要把沉默当成默认选项；需要假设就明说假设。"
		o.Note = fmt.Sprintf("挂单 %s（提问）到期未答 ⇒ 告知模型「无人应答」后继续", p.ID)
	case PendingInteraction:
		o.Result = TimeoutCancel
		o.Effect = ""
		o.Answer = AnswerCancel // 超时等价于"取消"（不是"什么都没发生"）
		o.ModelNote = "交互点超时 ⇒ 按【取消】处理：本次交互作废，**不得当作已确认**；" +
			"要接着做必须重新发起交互。"
		o.Note = fmt.Sprintf("挂单 %s（交互点 %s）到期未答 ⇒ 按取消处理", p.ID, p.Tool)
	default:
		// fail-closed：认不出的类别 ⇒ 当批准超时（最严）走。
		o.Result = TimeoutReject
		o.Effect = EffectDeny
		o.Reason = ReasonApprovalTimeout
		o.Answer = AnswerReject
		o.ModelNote = "挂单类别认不出（超时）⇒ 按最严处理：拒绝，不执行任何东西。"
		o.Note = fmt.Sprintf("挂单 %s 的类别「%s」认不出 ⇒ 按最严（批准超时=拒绝）结算", p.ID, string(p.Kind))
	}
	return o
}

// ruleText — 依据回显（空 rule_id 也要能读懂）。
func ruleText(ruleID string) string {
	if strings.TrimSpace(ruleID) == "" {
		return "(无规则 id：判不了/默认档)"
	}
	return ruleID
}

// ── 挂单 ────────────────────────────────────────────────────────────────────

// Pending — 一个**开放交互对象**（挂单）。字段全部导出：UI 要能如实回显"谁在等、等的是什么、还剩多久"。
type Pending struct {
	// ID — 稳定标识（答复、枚举、审计都按它点名）。由开单侧统一编号（见 MemoryPendingStore.nextID）：
	// **不是随机数也不是时间戳** —— 同样一串操作必然得到同样的 id（可测、可回放）。
	ID string `json:"id"`

	// Kind — 类别（三类；逐类超时语义见 timeoutOutcome）。
	Kind PendingKind `json:"kind"`

	// Tool — 工具名；`interaction` 类填**交互点/节点名**（F8 的 interrupt 点）。
	// 三类都**必须非空**（归一化后）：无名的挂单枚举不出、回指不到（见 fail-closed ④）。
	Tool string `json:"tool"`

	// ArgsDigest — **入参指纹**（只存指纹不存原文：审计与去重只需要"同不同"，不需要内容）。
	// 算法口径由接线批统一（audit.ArgsFingerprint）；本包只保管这个字符串。
	ArgsDigest string `json:"args_digest,omitempty"`

	// SessionID — 归属会话（跨会话恢复时按它找回自己的挂单；空 = 无归属，人工会话允许）。
	SessionID string `json:"session_id,omitempty"`

	// RuleID — **凭什么问**：命中的规则 id（内置条目 `any.*` / `never.*`，或文本规则的定位串）。
	// 它是审计侧"为何要人批"的答案，也是超时结算单里带的那一条依据。
	RuleID string `json:"rule_id,omitempty"`

	// CreatedAt — 开单时刻（**由 Open 的 now 参数决定**，不接受调用方回填：事实只能来自记账那一瞬）。
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt — 到期时刻；**零值 = 不设期限**（人工会话允许无限等）。
	// 但 `tool_approval` **必须**有期限（见 fail-closed ⑤）。
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// State — 状态（open | answered | timed_out）。
	State PendingStatus `json:"state"`
	// Answer — 人对它的答复（State=answered 时有值）。
	Answer Answer `json:"answer,omitempty"`
	// AnsweredAt — 答复时刻。
	AnsweredAt time.Time `json:"answered_at,omitempty"`
	// SettledAt — 超时结算时刻（State=timed_out 时有值）。
	SettledAt time.Time `json:"settled_at,omitempty"`

	// Seq — 开单序号（1 起、单调；取证排序用，不参与判定）。
	Seq int `json:"seq,omitempty"`
	// Note — 人类可读补充（仅取证；程序判据只看上面的字段）。
	Note string `json:"note,omitempty"`
}

// expiredAt — 是否已到点（零值 ExpiresAt = 不设期限 ⇒ 永不到期）。
// 判据与 grants.go 同源：`!now.Before(ExpiresAt)` ⇒ **恰等于到期时刻也算到点**（边界取严）。
func (p Pending) expiredAt(now time.Time) bool {
	return !p.ExpiresAt.IsZero() && !now.Before(p.ExpiresAt)
}

// ── 账本状态（序列化形态）────────────────────────────────────────────────────

// PendingState — 挂单账本的**全部事实**（含已答 / 已超时：它们发生过，不该在序列化时被抹掉）。
type PendingState struct {
	Version  int       `json:"version"`  // 必须 == PendingStateVersion
	NextSeq  int       `json:"next_seq"` // 下一个编号（必须 > 0 且 > 所有条目的 Seq —— 否则报错）
	Pendings []Pending `json:"pendings"` // 按开单顺序（Seq 序）
}

// PendingLoadReport — 载入时的记账（"恢复了几条、哪几条被按超时结算了"必须能回答，否则就是静默失效）。
type PendingLoadReport struct {
	Total         int // 序列化里有几条
	Kept          int // 恢复了几条（全部保留在枚举面）
	Open          int // 其中仍未答的（= ListOpen 的条数）
	Answered      int // 其中已答的
	TimedOut      int // 其中已超时的（含载入时结算的）
	SettledOnLoad int // 载入时点已到期的 open 项 ⇒ 立刻按超时结算（不复活）
}

// String — 一行摘要（进日志/用例断言都读得懂）。
func (r PendingLoadReport) String() string {
	return fmt.Sprintf("载入挂单账本：共 %d 条 ⇒ 保留 %d 条（未答 %d / 已答 %d / 已超时 %d，其中载入时结算 %d）",
		r.Total, r.Kept, r.Open, r.Answered, r.TimedOut, r.SettledOnLoad)
}

// ── 账本接口 ────────────────────────────────────────────────────────────────

// PendingStore — 挂单账本。**只做记账与查询**：不执行、不弹窗、不认识工具本体。
//
//	Open      —— 开一个挂单（校验不过 ⇒ 报错，不开单）
//	Answer    —— 答复（到点即超时结算并报错；不可重答；超时后迟到的答复不改变结局）
//	List      —— 枚举**全部**（含已答/已超时 —— 事实可查）
//	ListOpen  —— 枚举**未答**项（F12 的"可枚举未答项"）
//	Lookup    —— 按 id 只读取一条
//	Expire    —— 把到点的 open 项结算成超时，返回**逐类结局**（接线批按它分派处置）
//	Snapshot  —— 导出可序列化状态（重启继承用）
type PendingStore interface {
	Open(p Pending, now time.Time) (Pending, error)
	Answer(id string, decision Answer, now time.Time) (Pending, error)
	List() []Pending
	ListOpen() []Pending
	Lookup(id string) (Pending, bool)
	Expire(now time.Time) []TimeoutOutcome
	Snapshot() PendingState
}

// 编译期保证内存实现满足接口（换实现时这一行最先报错）。
var _ PendingStore = (*MemoryPendingStore)(nil)

// ── 内存实现 ────────────────────────────────────────────────────────────────

// MemoryPendingStore — 内存账本（本批唯一实现；落盘/落库实现另批，接口已经留好）。
// 并发安全（RWMutex）；**判定是确定性的**：不看系统时间，now 一律由调用方注入。
type MemoryPendingStore struct {
	mu       sync.RWMutex
	nextSeq  int
	pendings []Pending
}

// NewMemoryPendingStore — 建账本。
func NewMemoryPendingStore() *MemoryPendingStore {
	return &MemoryPendingStore{nextSeq: 1}
}

// Open — 开一个挂单。校验不过一律**报错、不开单**（fail-closed，见文件头 ④⑤⑥）：
//
//	· 类别认不出 / 工具名（或交互点名）归一化后为空
//	· tool_approval 没有 ExpiresAt（零值）—— 没人答的批准挂单不设期限 = 永久挂着
//	· 开单时点就已到点（`now >= ExpiresAt`）—— 开了也没人能答，等于故意造一个必超时的挂单
//	· 调用方自己填了 ID / Seq / State / Answer / SettledAt —— 这些字段只能由账本写
//	  （否则"超时结算"和"谁答的"可以被人从开单接口绕进来）
//
// CreatedAt **一律取 now**（调用方填什么都不影响：与 grants 的归属同理 —— 事实只能来自记账那一瞬）。
// 返回**落库后的那条**（ID / 序号 / 状态已填），调用方必须用返回值。
func (s *MemoryPendingStore) Open(p Pending, now time.Time) (Pending, error) {
	if !p.Kind.Valid() {
		return Pending{}, fmt.Errorf("开单：类别「%s」非法（只认 tool_approval|ask_user|interaction）", string(p.Kind))
	}
	tool := NormalizeToolName(p.Tool)
	if tool == "" {
		return Pending{}, fmt.Errorf("开单：工具名/交互点名归一化后为空 ⇒ 无名的挂单枚举不出、回指不到（不开单）")
	}
	if p.Kind == PendingToolApproval && p.ExpiresAt.IsZero() {
		return Pending{}, fmt.Errorf("开单：tool_approval 必须带 ExpiresAt（零值=不设期限 ⇒ 没人答就永久挂着；" +
			"批准超时必须能结算成拒绝）")
	}
	if p.expiredAt(now) {
		return Pending{}, fmt.Errorf("开单：这条挂单在开单时点（%s）就已到点（%s）⇒ 开了也没人能答（不开单）",
			now.UTC().Format(time.RFC3339), p.ExpiresAt.UTC().Format(time.RFC3339))
	}
	if p.ID != "" || p.Seq != 0 || p.State != "" || p.Answer != "" || !p.SettledAt.IsZero() || !p.AnsweredAt.IsZero() {
		return Pending{}, fmt.Errorf("开单：ID/Seq/State/Answer/AnsweredAt/SettledAt 只能由账本写（调用方填了 ⇒ 拒绝，" +
			"否则超时结算与答复可以绕过账本被伪造）")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p.Tool = tool
	p.Kind = PendingKind(strings.TrimSpace(string(p.Kind)))
	p.ArgsDigest = strings.TrimSpace(p.ArgsDigest)
	p.RuleID = strings.TrimSpace(p.RuleID)
	p.Note = strings.TrimSpace(p.Note)
	p.SessionID = strings.TrimSpace(p.SessionID)
	p.CreatedAt = now
	p.State = StatusOpen
	p.Answer = ""
	p.AnsweredAt = time.Time{}
	p.SettledAt = time.Time{}
	p.Seq = s.nextSeq
	p.ID = pendingID(p, s.nextSeq)
	s.nextSeq++
	s.pendings = append(s.pendings, p)
	return p, nil
}

// pendingID — 稳定 id：类别 + 工具名（或交互点名）+ 序号（同样一串操作必然得到同样的 id；
// 不含随机数与时间 —— 否则重启后对不上、用例也钉不住）。
func pendingID(p Pending, seq int) string {
	return fmt.Sprintf("%s:%s#%d", p.Kind, p.Tool, seq)
}

// Answer — 答复一个挂单。**先结算超时，再看能不能答**（顺序写死在这里，不许倒过来）：
//
//	① 找不到这个 id ⇒ 报错
//	② 已按超时结算（State=timed_out）⇒ **报错、状态一字不改**（迟到的答复不改变结局 —— 见 fail-closed ②）
//	③ 已答复（State=answered）⇒ **报错**（不可重答 —— 见 fail-closed ③）
//	④ 仍未答但**已到点**（now >= ExpiresAt）⇒ **当场结算成超时**并报错（不依赖 Expire 被调用 —— 见 ①）
//	⑤ 答复与类别不符（`correct` 用在非工具批准上）⇒ 报错（不猜语义 —— 见 ⑥）
//
// 返回**落库后的那条**。第 ② ③ 步报错时返回的是**未被改动的**那条（"这件事没发生"）。
func (s *MemoryPendingStore) Answer(id string, decision Answer, now time.Time) (Pending, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Pending{}, fmt.Errorf("答复：挂单 id 为空 ⇒ 判不了（不改任何东西）")
	}
	if !decision.Valid() {
		return Pending{}, fmt.Errorf("答复：答复值「%s」非法（只认 approve|reject|correct|cancel）", string(decision))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	i := s.indexOfLocked(id)
	if i < 0 {
		return Pending{}, fmt.Errorf("答复：没有这个挂单（id=%s）⇒ 不改任何东西", id)
	}
	p := &s.pendings[i]

	if p.State == StatusTimedOut {
		return *p, fmt.Errorf("答复被拒：挂单 %s 已按超时结算（%s）⇒ **迟到的答复不改变结局**"+
			"（超时那一刻的事实已经生效；要做就重新开单）", p.ID, p.SettledAt.UTC().Format(time.RFC3339))
	}
	if p.State == StatusAnswered {
		return *p, fmt.Errorf("答复被拒：挂单 %s 已答复过（answer=%s @%s）⇒ **不可重答**",
			p.ID, string(p.Answer), p.AnsweredAt.UTC().Format(time.RFC3339))
	}
	if p.expiredAt(now) {
		// 到点即超时：先把结算落库，再报错（顺序反过来的话，"结算了但没有效果"或"没结算却能答"必居其一）。
		s.settleLocked(p, now)
		return *p, fmt.Errorf("答复被拒：挂单 %s 在答复时点（%s）已到点（%s）⇒ 按超时结算：%s",
			p.ID, now.UTC().Format(time.RFC3339), p.ExpiresAt.UTC().Format(time.RFC3339), timeoutOutcome(*p).ModelNote)
	}
	if decision == AnswerCorrect && p.Kind != PendingToolApproval {
		return *p, fmt.Errorf("答复被拒：挂单 %s 的类别是 %s ⇒ `correct`（更正参数）只对工具批准有意义（不猜语义）",
			p.ID, string(p.Kind))
	}

	p.State = StatusAnswered
	p.Answer = decision
	p.AnsweredAt = now
	return *p, nil
}

// settleLocked — 把一条 open 挂单按超时结算（**只改状态与结算时刻**，语义在 timeoutOutcome 里）。
// 调用方必须已持写锁。
func (s *MemoryPendingStore) settleLocked(p *Pending, now time.Time) {
	p.State = StatusTimedOut
	p.SettledAt = now
}

// indexOfLocked — 按 id 找位置（-1 = 没有）。调用方必须已持锁（读或写都行）。
func (s *MemoryPendingStore) indexOfLocked(id string) int {
	for i := range s.pendings {
		if s.pendings[i].ID == id {
			return i
		}
	}
	return -1
}

// Lookup — 按 id 只读取一条（含已答/已超时；第二个返回值为 false = 没有这个挂单）。
func (s *MemoryPendingStore) Lookup(id string) (Pending, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Pending{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i := s.indexOfLocked(id); i >= 0 {
		return s.pendings[i], true
	}
	return Pending{}, false
}

// List — 枚举**全部**挂单（含已答/已超时：它们是发生过的事实，用户与审计都要看得见）。
// 返回副本（调用方改返回值不污染账本）；按开单顺序（即 Seq 序）。
func (s *MemoryPendingStore) List() []Pending {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Pending, len(s.pendings))
	copy(out, s.pendings)
	return out
}

// ListOpen — 枚举**未答**项（F12 的"持久化开放交互对象 ⇒ 可枚举未答项"）。
// 已答 / 已超时的**不在**这里（它们不再是"开放"的）——但仍在 List 里（事实不消失）。
func (s *MemoryPendingStore) ListOpen() []Pending {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Pending
	for _, p := range s.pendings {
		if p.State == StatusOpen {
			out = append(out, p)
		}
	}
	return out
}

// Expire — 把**到点的 open 项**结算成超时，返回逐类结局（保序，与开单顺序一致）。
//
//	· 到点判据 = `!now.Before(ExpiresAt)`（**恰等于到期时刻也算到点**）；
//	· 零值 ExpiresAt（不设期限）= 永不到期 ⇒ 不结算（人工会话允许无限等）；
//	· **幂等**：已经答过 / 已经超时的项不再结算、不再出现在返回值里（重复调用不会出现第二份结局）；
//	· 返回值是**事实**：`Result=reject` 的那一条，接线批必须照"拒绝"处理 —— 见 timeoutOutcome。
func (s *MemoryPendingStore) Expire(now time.Time) []TimeoutOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []TimeoutOutcome
	for i := range s.pendings {
		p := &s.pendings[i]
		if p.State != StatusOpen || !p.expiredAt(now) {
			continue
		}
		s.settleLocked(p, now)
		out = append(out, timeoutOutcome(*p))
	}
	return out
}

// Snapshot — 导出可序列化状态（含全部事实 + 编号水位；重启继承靠它）。
func (s *MemoryPendingStore) Snapshot() PendingState {
	list := s.List()
	sort.SliceStable(list, func(i, j int) bool { return list[i].Seq < list[j].Seq })
	s.mu.RLock()
	next := s.nextSeq
	s.mu.RUnlock()
	return PendingState{Version: PendingStateVersion, NextSeq: next, Pendings: list}
}

// MarshalJSON — 账本 ⇒ JSON（重启继承的写入侧）。经 Snapshot，字段固定（Version/NextSeq/Pendings）。
func (s *MemoryPendingStore) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Snapshot())
}

// LoadPending — JSON ⇒ 账本（重启继承的读取侧）。四条 fail-closed 语义见文件头"序列化与重启"。
// 返回的账本**不绑任何会话上下文**：归属只是 Pending 上的一个字段（调用方按 SessionID 自己过滤）。
func LoadPending(data []byte, now time.Time) (*MemoryPendingStore, PendingLoadReport, error) {
	var st PendingState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, PendingLoadReport{}, fmt.Errorf("挂单账本载入失败（JSON 坏）⇒ 拒绝启用（不回退成「空账本」）：%w", err)
	}
	return loadPendingState(st, now)
}

// loadPendingState — LoadPending 的本体（拆出来便于用例直接喂状态，不经 JSON）。
func loadPendingState(st PendingState, now time.Time) (*MemoryPendingStore, PendingLoadReport, error) {
	if st.Version != PendingStateVersion {
		return nil, PendingLoadReport{}, fmt.Errorf("挂单账本版本 %d 不认识（本实现只认 %d）⇒ 拒绝启用（不猜格式）",
			st.Version, PendingStateVersion)
	}
	// 编号水位必须自洽：否则重启后会发出与旧挂单撞车的 id（答复可能答错一条）。
	maxSeq := 0
	for _, p := range st.Pendings {
		if p.Seq > maxSeq {
			maxSeq = p.Seq
		}
	}
	if st.NextSeq <= maxSeq {
		return nil, PendingLoadReport{}, fmt.Errorf("挂单账本编号水位不自洽（next_seq=%d ≤ 最大序号 %d）⇒ 拒绝启用（否则新挂单会与旧条目撞 id）",
			st.NextSeq, maxSeq)
	}

	store := &MemoryPendingStore{nextSeq: st.NextSeq}
	rep := PendingLoadReport{Total: len(st.Pendings)}
	for _, p := range st.Pendings {
		if !p.Kind.Valid() {
			return nil, PendingLoadReport{}, fmt.Errorf("挂单账本载入失败：条目 %s 的类别「%s」认不出 ⇒ 拒绝启用（不猜类别）",
				p.ID, string(p.Kind))
		}
		if !p.State.Valid() {
			return nil, PendingLoadReport{}, fmt.Errorf("挂单账本载入失败：条目 %s 的状态「%s」认不出 ⇒ 拒绝启用（不猜状态）",
				p.ID, string(p.State))
		}
		if p.State == StatusOpen && p.expiredAt(now) {
			// 重启这段时间也是时间：期间到期的 open 项**当场结算成超时**（不复活成"还能答"）。
			p.State = StatusTimedOut
			p.SettledAt = now
			rep.SettledOnLoad++
		}
		store.pendings = append(store.pendings, p)
		rep.Kept++
		switch p.State {
		case StatusOpen:
			rep.Open++
		case StatusAnswered:
			rep.Answered++
		case StatusTimedOut:
			rep.TimedOut++
		}
	}
	return store, rep, nil
}
