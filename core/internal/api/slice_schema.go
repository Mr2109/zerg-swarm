package api

// ============ B 项③ 片（单子）最小 schema 与挂板校验（2026-09-18）============
//
// 依据（只引用、不改它们）:
//
//	docs/01-设计/切片合同-模板.md                      — 字段名 / 必须字段 / 冲突优先级
//	docs/01-设计/设计-任务模块骨架-v1.0.md §0 — 三层术语: 任务(mission_id) → 片/单子(slice_id) → 步
//
// 本轮范围（**只做这一块**）:
//
//	① 最小 schema 四字段: slice_id / depends_on / owner / acceptance（定义见 master_scheduler.go 的 Task）
//	② 挂板校验: 缺 slice_id · 缺 acceptance 声明 · depends_on 有环 · depends_on 悬空 ⇒ **拒绝入队**
//	③ 拒绝时给**可行动错误**（点明缺什么 + 怎么写对）并打一条观测事件
//
// ✗ 本轮**不**实现（后续 B 项）: 状态机 / 四态 / 取消 / 转移表 / validator 集合。
//
//	设计稿 §1.1（状态集合）与 §1.2（转移表）本轮**只作参考**，一行都不落。
//
// **零回归口径（硬要求）**: 校验**只对「声明了片字段」的任务生效**。
//
//	Declared() == false（= 现有全部任务）⇒ 校验函数第一行就返回 nil、观测面不落任何事件 ⇒
//	提交行为与改动前逐字节等价（用例 TestSliceMount_UndeclaredTaskUnchanged 守卫这条）。
//
// 观测出口: 本包原先没有「片/挂板」类事件；沿用 api 包既有习惯（log.Printf + 状态目录下
// append-only JSONL，同 tasks_persist.go / CA 侧 events.jsonl），**不新造框架**，也不借用
// toolobs（那是「工具调用判定」的专用形状: Tool/ArgsDigest/allow-deny，语义不对口）。
//
// ── B 项 B5（2026-09-18）: 黑板三字段 —— Lead / 分工方案 / 等确认闸 ──
//
// 出处（逐字，只引用、**不改稿面**）:
//
//	设计-协作骨架-v2.1.md:190  「| **Lead（负责的模型）** | 由 Mr2109 指定（或按任务特征从池中选） | **拥有会话**；…**提出分工**并**等确认**；**汇总**；据验证结果**重规划** | 团体 |」
//	同稿:211                            「- `/api/tasks`：扩为**黑板**（含：单子/片 + 状态 + 依赖 + 归属 Lead/Member + 验收标准）；」
//	同稿:203                            「接任务 ⇒ **组队**…⇒ 提分工方案 ⇒ **等 Mr2109 确认**（可配置为自动）⇒ 挂板 + 发单子…」（**确认在挂板之前**）
//	同稿:69（s13）                      「**Lead 拥有用户会话** ✓ 提出分工 ✓ **等确认** ✓；…`shutdown` 与 `plan approval` 做成**可追踪、可强制的协议**」
//	同稿:239（nano_agent_team）          「…`confirmation_callback`…」（确认闸属**环境层强制**）
//	设计-任务模块骨架-v1.0.md:59「须由 Lead/人声明」（与 v2.1 §4.3「等确认」一致）
//
// 命名口径（**稿面只给了中文名/术语，没给字段键** ⇒ 下列键名是本轮命名，逐条登记待 Mr2109 定）:
//
//	lead          ← 「Lead（负责的模型）」（稿面逐字就用 `Lead` 这个英文词；键名照现有 slice_id/depends_on 风格小写）
//	members       ← 「成员（专家）」/「Member Agents」（稿面逐字用 `Member`）；本键 = **分工方案**：「每个成员干什么」
//	plan_approved ← 「等确认」闸（稿面逐字 `plan approval`）；用 `*bool` ⇒「没写」与「显式 false」可分（同 acceptance 之规）
//	成员条目 id   ← §4.6-2「**成员是带 id 的实例**」（逐字）
//	成员条目 duty ← 稿面只有中文「分工」/「干什么」⇒ **键名是本轮命名**（待定，见回报⑥）
//
// 「等确认闸」的接法与不变量:
//
//	① 判定点**不新建**——落进本文件同一个 ValidateSliceMount（挂板闸的唯一实现点，
//	   Submit / SubmitSlice / HTTP 三处共用），Error 形状沿用同一 *SliceValidationError；
//	② 触发面**只对「声明了分工方案」的片**（members 非空，或显式写了 plan_approved ⇒ fail closed）；
//	③ 未确认 ⇒ **拒**（不入队），可行动错误给「怎么改」；已确认 ⇒ 放（与既有四拒同一形状）。
//
// ✗ 本轮**不**实现（登记，不自造）: 「可配置为自动」（v2.1 §4.3）的自动确认档 / 团规模上限 4 /
// Lead 选举与「池中哪枚卵」合法性（池清单未给，v2.1 §8-U9 ⇒ 只判「有没有给」，不判「在不在池里」）。
//
// ── B 项⑤（2026-09-18）: 事件名扩成设计稿 §6.1 闭集，**机制仍只有这一套** ──
//
//	出口下沉到 `core/internal/sliceobs`（唯一写入函数 Emit、唯一落点 slice-mount-events.jsonl、
//	唯一形状 sliceobs.Event）。本文件现在的 SliceMountEvent / SliceMountEventsFile 是它的
//	**别名与转发**（见下）；挂板放行发 `slice_created`、拒绝发 `slice_rejected`，
//	旧名 `slice_mount` 归并成 `alias` 字段（不再作为 event 值发出）。
//	打回（loopcore）与第 4 态（policy）两处接同一套出口 —— 详见 sliceobs 的包注释。

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/sliceobs"
)

// ---- 挂板校验错误码（低基数、可枚举；HTTP 错误码与观测事件共用同一个值，不解析错误原文）----
const (
	SliceErrMissingID         = "SLICE_MISSING_ID"         // ① 缺片真源 id（slice_id 为空）
	SliceErrMissingAcceptance = "SLICE_MISSING_ACCEPTANCE" // ② 缺 acceptance **声明**（缺失 ≠ 显式空）
	SliceErrDependsDangling   = "SLICE_DEPENDS_DANGLING"   // ③ depends_on 指向板上不存在的片
	SliceErrDependsCycle      = "SLICE_DEPENDS_CYCLE"      // ④ depends_on 成环（不可拓扑排序）
	// ---- B 项 B5（2026-09-18）: 黑板三字段的两条声明类拒 + 等确认闸 ----
	SliceErrLeadInvalid     = "SLICE_LEAD_INVALID"     // ⑤ 声明了 lead 但给的是空白（= 没指定谁负责）
	SliceErrMemberInvalid   = "SLICE_MEMBER_INVALID"   // ⑥ 分工方案里有成员缺 id（或无「干什么」）
	SliceErrPlanUnconfirmed = "SLICE_PLAN_UNCONFIRMED" // ⑦ 等确认闸：分工方案未经确认 ⇒ 挂板前拒
)

// 观测事件名（B 项⑤: 设计稿 §6.1 闭集四名；常量本身以 sliceobs 为唯一真源 —— 不在此处抄一份）。
const (
	sliceCreatedEventName   = sliceobs.EventSliceCreated           // 挂板放行
	sliceRejectedEventName  = sliceobs.EventSliceRejected          // 挂板拒绝
	sliceEscalatedEventName = sliceobs.EventSliceEscalated         // 第 4 态（loopcore/policy 侧接）
	sliceSkippedEventName   = sliceobs.EventSliceSkippedByExecutor // 越片动作（执行者侧接）
	// sliceMountEventName — B 项③ 的旧名，**归并后只作 `alias` 字段值**（不再作为 event 值发出）。
	sliceMountEventName = sliceobs.AliasSliceMount
)

// SliceInput 片的挂板入参（JSON 解码目标；也可由已建 Task 反推 —— 见 SliceInputFromTask）。
//
// **声明语义（本轮的关键判别，缺失与显式空必须分开）**:
//
//	Acceptance == nil                 ⇒ 请求体里**没有** acceptance 键 ⇒ 缺失 ⇒ 拒绝入队
//	Acceptance == &[]string{}         ⇒ 显式声明为空数组 ⇒ 允许（入队后须补齐判据）
//	Acceptance == &[]string{"…","…"}  ⇒ 显式声明判据 ⇒ 允许
//
// 为什么用指针而不是 `[]string`: Go 的 json 解码把「键缺失」与「显式 []」都落成 nil/空切片，
// 用值类型就读不出「写过没写过」——而设计要求二者区分（切片合同-模板 §1 的「留空 ≠ 写无」同规）。
type SliceInput struct {
	SliceID    string    `json:"slice_id"`   // 片真源 id（空 ⇒ 拒）
	DependsOn  []string  `json:"depends_on"` // 依赖的片 id 列表（有环 / 悬空 ⇒ 拒）
	Owner      string    `json:"owner"`      // 片归属者（可选，本轮不参与判定）
	Acceptance *[]string `json:"acceptance"` // 验收判据（**必须显式声明**）
	// ---- B 项 B5（2026-09-18）: 黑板三字段（全可选、全指针/切片 ⇒ 未声明的片逐字节零回归）----
	Lead         *string       `json:"lead,omitempty"`          // Lead（负责的模型）: 声明了就必须非空白（空白 ⇒ 拒）
	Members      []SliceMember `json:"members,omitempty"`       // 分工方案: 每个成员干什么（非空 ⇒ 必须过「等确认闸」）
	PlanApproved *bool         `json:"plan_approved,omitempty"` // 等确认闸: 分工方案是否已确认（nil=没写；显式 false ⇒ 拒）
}

// SliceMember 分工方案里的一条 —— **每个成员干什么**（v2.1 §4.3「提分工方案」）。
//
// 键名口径（稿面只给中文/术语，没给键名 ⇒ 本轮命名，待 Mr2109 定）:
//
//	id   ← §4.6-2 逐字「成员是带 id 的实例」
//	Duty ← 稿面只有中文「分工」/「每个成员干什么」⇒ 键名 `duty` 为本轮命名
type SliceMember struct {
	ID   string `json:"id"`   // 成员（专家）id —— 空/空白 ⇒ 拒（成员是带 id 的实例）
	Duty string `json:"duty"` // 该成员在这片里干什么 —— 空/空白 ⇒ 拒（没写分工就不算分工方案）
}

// Declared 是否声明了**任何**片字段 —— 挂板校验的总开关。
// 返回 false ⇒ 该任务不是片 ⇒ 完全跳过校验（这是「不改现有任务提交行为」的机制本身）。
func (in SliceInput) Declared() bool {
	return in.SliceID != "" || in.Owner != "" || in.DependsOn != nil || in.Acceptance != nil ||
		in.Lead != nil || in.Members != nil || in.PlanApproved != nil
}

// planApproved 等确认闸的判定取值 —— **只有显式 true 才算已确认**（fail closed）:
//
//	nil          ⇒ 没写        ⇒ 未确认
//	&false       ⇒ 显式未确认   ⇒ 未确认
//	&true        ⇒ 已确认       ⇒ 过闸
func (in SliceInput) planApproved() bool {
	return in.PlanApproved != nil && *in.PlanApproved
}

// declaresDivisionOfLabor 是否**声明了分工方案**（= members 非空）—— 决定「等确认闸」开不开。
//
// 口径: 稿面 §4.3 的顺序是「提分工方案 ⇒ 等 Mr2109 确认 ⇒ 挂板」；没提分工方案 ⇒ 没有要确认的东西
// ⇒ 闸不适用（与既有的「只对声明了片字段的任务生效」同一精神，保证未声明新字段的片零回归）。
func (in SliceInput) declaresDivisionOfLabor() bool {
	return len(in.Members) > 0
}

// ApplyTo 把片字段写进 Task（**校验通过后**才调用）。
func (in SliceInput) ApplyTo(t *Task) {
	if t == nil {
		return
	}
	t.SliceID = in.SliceID
	t.DependsOn = in.DependsOn
	t.Owner = in.Owner
	t.Acceptance = in.Acceptance // 指针原样带走 ⇒「缺失/显式空」的区分在任务记录里不失真
	t.Lead = in.Lead             // 同上: 指针原样带走（「没写 lead」与「写了空 lead」不失真）
	t.Members = in.Members
	t.PlanApproved = in.PlanApproved
}

// SliceInputFromTask 从已建 Task 反推挂板入参（入队闸与 SubmitSlice 的校验都走它——单一形状）。
func SliceInputFromTask(t *Task) SliceInput {
	if t == nil {
		return SliceInput{}
	}
	return SliceInput{
		SliceID:      t.SliceID,
		DependsOn:    t.DependsOn,
		Owner:        t.Owner,
		Acceptance:   t.Acceptance,
		Lead:         t.Lead,
		Members:      t.Members,
		PlanApproved: t.PlanApproved,
	}
}

// SliceBoard 挂板视图: 板上已挂的片 → 其依赖列表（slice_id → depends_on）。
//
// 判据口径:
//
//	悬空依赖 = depends_on 里出现「不在板上、也不是本片自己」的 id（⇒ 该片永远不会就绪）
//	环       = 板上各片的依赖图（含本片）**不可拓扑排序**
//
// 口径边界（如实登记）: 本轮只判「依赖的片**存在**」与「依赖图**无环**」，
// **不判**「依赖是否已完成」——完成度属状态机（本轮 ✗ 不实现，设计稿 §1.1/§1.2 未落）。
type SliceBoard map[string][]string

// SliceValidationError 挂板校验失败（可行动错误: 点明缺什么 + 怎么写对）。
// 结构化字段供 HTTP 错误码（writeErrorCode）与观测事件复用——复用同一个 code，不解析错误原文。
type SliceValidationError struct {
	Code    string // 机器可判错误码（SliceErr*）
	Field   string // 出错字段名（slice_id / acceptance / depends_on）
	SliceID string // 涉及的片（缺 slice_id 时为空——那时确实没有 id 可用）
	Message string // 人可读、可行动（缺什么 + 怎么写对 + 可用的合法取值）
}

func (e *SliceValidationError) Error() string { return e.Message }

// SliceMountEvent 挂板事件的观测形状（append-only JSONL 一行）。
//
// **B 项⑤ 起它是 `sliceobs.Event` 的别名**（同一套机制、同一份形状、同一落点）：
// 事件名扩成设计稿 §6.1 闭集，旧名 `slice_mount` 归并进 `alias` 字段。
// 作用（与 toolobs 的同一条教训同源）: 「被挂板校验拒绝」与「根本没声明片字段」在观测面上
// 原本长得一模一样（都只是「没有这条事件」）⇒ 无法归因；故把判定落成事件。
type SliceMountEvent = sliceobs.Event

// SliceMountEventsFile 挂板事件落点（append-only JSONL，状态目录下——与仓库既有落盘同根）。
// 函数而非常量: 测试经 ZERG_STATE_DIR 切到临时目录（仓库既有测试隔离习惯）。
// **转发**到 sliceobs.EventsFile()（唯一真源）—— 本包不再自己拼路径。
func SliceMountEventsFile() string { return sliceobs.EventsFile() }

// emitSliceMount 打一条挂板观测事件（**唯一观测出口**在 sliceobs.Emit；本函数只组装事件）。
//
//	serr == nil ⇒ `slice_created`（放行入队）；serr != nil ⇒ `slice_rejected`（拒绝入队）。
//
// 调用点只有两处（同一实现点，保证「每次挂板尝试恰好一条事件」）:
// ① 入队闸（Submit 内）② HTTP 侧前置校验拒绝时（那时 Submit 不会被调用）。
//
// best-effort：落盘失败只记日志，**绝不改判定**（Emit 无返回值 ⇒ 本函数也无从据此改判）。
func emitSliceMount(task *Task, serr *SliceValidationError) {
	ev := sliceobs.Event{
		Event:           sliceCreatedEventName,
		Alias:           sliceMountEventName, // 旧名归并：历史读取方按 alias 仍认得出挂板事件
		Time:            time.Now().Format(time.RFC3339),
		OK:              serr == nil,
		Outcome:         sliceobs.OutcomeCreated,
		TaskID:          taskIDOf(task),
		SliceID:         sliceIDOf(task),
		CriteriaVersion: sliceobs.CriteriaVersionUncalibrated, // 无来源 ⇒ 显式「未标定」（不编造）
	}
	if task != nil {
		ev.DependsOn = task.DependsOn
		ev.AcceptanceDeclared = task.Acceptance != nil
	}
	if serr != nil {
		ev.Event = sliceRejectedEventName
		ev.OK = false
		ev.Outcome = sliceobs.OutcomeRejected
		ev.Code = serr.Code
		ev.Field = serr.Field
		ev.Detail = serr.Message
		if ev.SliceID == "" {
			ev.SliceID = serr.SliceID
		}
		log.Printf("⛔ scheduler: slice mount rejected — task %s slice %q [%s]: %s",
			ev.TaskID, ev.SliceID, serr.Code, serr.Message)
	} else {
		ev.Detail = "片已挂板放行入队：其它片的 depends_on 可引用它" +
			"（挂板只判「依赖的片存在」与「依赖图无环」，不判完成度）。"
		log.Printf("🧩 scheduler: slice mounted — task %s slice %q (depends_on=%v acceptance_declared=%v)",
			ev.TaskID, ev.SliceID, ev.DependsOn, ev.AcceptanceDeclared)
	}
	sliceobs.Emit(ev)
}

func taskIDOf(t *Task) string {
	if t == nil {
		return ""
	}
	return t.ID
}

func sliceIDOf(t *Task) string {
	if t == nil {
		return ""
	}
	return t.SliceID
}

// ValidateSliceMount 挂板校验（**仅对声明了片字段的任务生效**）。
//
//	in    = 待挂板片的片字段（含「有没有写过 acceptance」「有没有写过 plan_approved」这两处区分）
//	board = 板上**已有**片的视图（调用方给，**不含本片**；本片由本函数自己加进图里）
//
// 返回 nil ⇒ 可入队；返回 *SliceValidationError ⇒ **拒绝入队**。
// 判定顺序固定（先「有没有 id」再「声明齐不齐」再「引用真不真」再「图有没有环」）——
// 报错只报第一条命中的，且每条都点明「缺什么 + 怎么写对」。
//
// B 项 B5 起「声明齐不齐」这一段含三条新判定（lead 非空白 / 分工方案每条成员齐全 / 等确认闸），
// 它们与既有四拒**同属这一个函数、同一个判定点**（不另建第二套校验）。
func ValidateSliceMount(in SliceInput, board SliceBoard) *SliceValidationError {
	if !in.Declared() {
		return nil // ✗ 没声明任何片字段 ⇒ 不是片 ⇒ 不校验（现有任务零回归的机制所在）
	}
	// ① slice_id（片真源 id）必填
	if strings.TrimSpace(in.SliceID) == "" {
		return &SliceValidationError{
			Code:  SliceErrMissingID,
			Field: "slice_id",
			Message: "缺 slice_id：片（单子）必须有真源 id —— 没有 id 就无法被别的片 depends_on 引用，" +
				"也无法与它的 task_id 对齐。怎么改：请求体加 \"slice_id\":\"S3-C1\"（写成该片的真实 id）；" +
				"判据提示：声明了 depends_on/owner/acceptance 中任何一个，就必须同时给出 slice_id。",
		}
	}
	// ② acceptance 必须**显式声明**（缺失 ≠ 显式空；显式空数组允许）
	if in.Acceptance == nil {
		return &SliceValidationError{
			Code:    SliceErrMissingAcceptance,
			Field:   "acceptance",
			SliceID: in.SliceID,
			Message: fmt.Sprintf("片 %s 缺 acceptance 声明：验收判据必须显式给出（设计要求「缺失」与「显式空」区分开 —— 没写 ≠ 写了空）。"+
				"怎么改：加 \"acceptance\":[\"go test ./internal/api/ -count=1\"]（每条 = 可复制命令 + 期望输出）；"+
				"确实还没有判据就显式写 \"acceptance\":[]（= 已声明暂无判据，不算缺声明）。", in.SliceID),
		}
	}
	// ②b lead（B 项 B5）: 声明了就必须**非空白** —— 空白 = 没指定谁负责（v2.1 §4.1「Lead（负责的模型）」）
	// 口径: 没写 lead（nil）本轮**不拒**（Lead 由 Mr2109 指定，不在片里也算片）；写了空白值才拒。
	if in.Lead != nil && strings.TrimSpace(*in.Lead) == "" {
		return &SliceValidationError{
			Code:    SliceErrLeadInvalid,
			Field:   "lead",
			SliceID: in.SliceID,
			Message: fmt.Sprintf("片 %s 声明了 lead 但给的是空白：Lead = **负责的模型**（v2.1 §4.1「Lead（负责的模型）」），"+
				"空白等于没指定谁负责这片。怎么改：填该模型在池里的真源名（形如 \"lead\":\"qwen3-coder\"）；"+
				"确实还没指定就不要写这个键（没写 = 未指定，不拒；写了空 = 声明了却没给值 ⇒ 拒）。", in.SliceID),
		}
	}
	// ②c 分工方案（B 项 B5）: **每个成员干什么** —— 每条都要有 id（「成员是带 id 的实例」）与非空的分工
	if len(in.Members) > 0 {
		for i, m := range in.Members {
			if strings.TrimSpace(m.ID) == "" {
				return &SliceValidationError{
					Code:    SliceErrMemberInvalid,
					Field:   "members",
					SliceID: in.SliceID,
					Message: fmt.Sprintf("片 %s 的分工方案第 %d 条缺成员 id：成员是**带 id 的实例**（v2.1 §4.6-2），没 id 的条目无法认领/无法对齐进度。"+
						"怎么改：给该条补 \"id\"（成员在池里的真源名），形如 \"members\":[{\"id\":\"egg-alpha\",\"duty\":\"…\"}]。"+
						"（稿面的池清单未给 —— 只判「有没有 id」，不判「在不在池里」。）", in.SliceID, i+1),
				}
			}
			if strings.TrimSpace(m.Duty) == "" {
				return &SliceValidationError{
					Code:    SliceErrMemberInvalid,
					Field:   "members",
					SliceID: in.SliceID,
					Message: fmt.Sprintf("片 %s 的分工方案第 %d 条（成员 %s）没写「干什么」：分工方案 = 每个成员干什么（v2.1 §4.3「提分工方案」），"+
						"只有 id 没有分工就不是分工方案。怎么改：给该条补 \"duty\"，形如 \"duty\":\"写用例并跑 -count=1\"；"+
						"确实还没有分工就把该成员从 members 里去掉（去掉 = 未提名，不算缺声明）。", in.SliceID, i+1, m.ID),
				}
			}
		}
	}
	// ②d 等确认闸（B 项 B5）: **声明了分工方案的片，未确认 ⇒ 拒**
	//
	//	出处: v2.1 §4.3「提分工方案 ⇒ **等 Mr2109 确认**（可配置为自动）⇒ **挂板 + 发单子**」
	//	      ⇒ 门在**挂板之前**；本闸就落在挂板闸里（同一个判定点）。
	//	fail closed（v2.1 §4.7「超时一律 fail closed（禁静默通过、禁无限等待）」）:
	//	      plan_approved 没写 / 显式 false ⇒ 都算**未确认** ⇒ 拒。
	if !in.planApproved() && (in.declaresDivisionOfLabor() || in.PlanApproved != nil) {
		return &SliceValidationError{
			Code:    SliceErrPlanUnconfirmed,
			Field:   "plan_approved",
			SliceID: in.SliceID,
			Message: fmt.Sprintf("片 %s 的等确认闸没过：提了分工方案就必须**先等 Mr2109 确认**再挂板（v2.1 §4.3：提分工方案 ⇒ 等 Mr2109 确认（可配置为自动）⇒ 挂板 + 发单子）。"+
				"怎么改：确认后显式写 \"plan_approved\":true（= 该分工方案已确认）；未确认前**不要挂板**——挂板会在这一步被拒且不入队。"+
				"（显式写 false 与不写同判为「未确认」——fail closed，禁静默通过。）", in.SliceID),
		}
	}
	// ③ 悬空依赖: depends_on 指向板上不存在的片（本片自己算「存在」——自依赖留给 ④ 判成环）
	known := make(SliceBoard, len(board)+1)
	for id, deps := range board {
		known[id] = deps
	}
	known[in.SliceID] = in.DependsOn
	for _, dep := range in.DependsOn {
		if _, ok := known[dep]; !ok {
			return &SliceValidationError{
				Code:    SliceErrDependsDangling,
				Field:   "depends_on",
				SliceID: in.SliceID,
				Message: fmt.Sprintf("片 %s 的 depends_on 指向不存在的片 %q：板上没有这个 slice_id（悬空依赖 —— 该依赖永远不会就绪，该片会永久卡住）。"+
					"怎么改：① 先提交那个依赖片（带上它自己的 slice_id）再挂本片；或 ② 把 %q 改成板上真实存在的片 id。"+
					"当前板上的片 id：%s。", in.SliceID, dep, dep, formatSliceIDs(board)),
			}
		}
	}
	// ④ 环: 板上各片（含本片）的依赖图必须可拓扑排序
	if order, err := TopoSortSlices(known); err != nil {
		return &SliceValidationError{
			Code:    SliceErrDependsCycle,
			Field:   "depends_on",
			SliceID: in.SliceID,
			Message: fmt.Sprintf("片 %s 的 depends_on 成环（%v）：环上的片互相等待 ⇒ 全部永远不就绪（设计稿 v2.1 R8「depends_on 可拓扑排序且无环」）。"+
				"怎么改：断开环中任一条边 —— 把环中某个片的 depends_on 里指向环内另一片的 id 删掉（或改成依赖环外的片），使依赖图可拓扑排序后重挂。"+
				"（板上的片共 %d 个，当前不可排序。）", in.SliceID, err, len(known)),
		}
	} else {
		_ = order // 可排序即通过（顺序本身由 TopoSortSlices 的调用方用，如合成态重放）
	}
	return nil
}

// formatSliceIDs 板上片 id 的稳定可读列表（错误信息里给「可用的合法取值」）
func formatSliceIDs(board SliceBoard) string {
	ids := make([]string, 0, len(board))
	for id := range board {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return "（空——板上还没有任何片，先挂依赖片）"
	}
	return strings.Join(ids, ", ")
}

// TopoSortSlices 依赖图拓扑排序（Kahn），**确定性**（同图同序 ⇒ 用例与门禁可复现）。
//
// 返回任一满足 depends_on 先于自身的顺序；**有环 ⇒ 返回错误并点出环上的片**（形如 a → b → a）。
// 两处用途: ① 挂板校验的环判定；② 「合法片 ⇒ depends_on 可拓扑排序」这一条的判据。
//
// 悬空边（指向不在 board 里的 id）**不参与排序**也不报环 —— 悬空有专门的错误码
// （ValidateSliceMount ③），在排序里静默跳过是为了让两个判据各自单义（不会互相冒充）。
func TopoSortSlices(board SliceBoard) ([]string, error) {
	nodes := make([]string, 0, len(board))
	for id := range board {
		nodes = append(nodes, id)
	}
	sort.Strings(nodes)

	indeg := make(map[string]int, len(nodes))
	for _, id := range nodes {
		indeg[id] = 0
	}
	for _, id := range nodes {
		for _, dep := range board[id] {
			if _, ok := board[dep]; ok {
				indeg[id]++ // 边 dep → id（依赖先于被依赖者）
			}
		}
	}
	var ready []string
	for _, id := range nodes {
		if indeg[id] == 0 {
			ready = append(ready, id)
		}
	}
	sort.Strings(ready)

	out := make([]string, 0, len(nodes))
	for len(ready) > 0 {
		n := ready[0]
		ready = ready[1:]
		out = append(out, n)
		var freed []string
		for _, id := range nodes {
			for _, dep := range board[id] {
				if dep == n {
					indeg[id]--
					if indeg[id] == 0 {
						freed = append(freed, id)
					}
				}
			}
		}
		sort.Strings(freed)
		ready = append(ready, freed...)
		sort.Strings(ready)
	}
	if len(out) != len(nodes) {
		var rest []string
		for _, id := range nodes {
			if indeg[id] > 0 {
				rest = append(rest, id)
			}
		}
		return nil, fmt.Errorf("环上的片: %s", strings.Join(cyclePath(rest, board), " → "))
	}
	return out, nil
}

// cyclePath 在「Kahn 之后仍入度 > 0」的节点里找出**一条真实的环路径**（错误信息要给出环，
// 而不是只说「有环」——只说有环不可行动）。
func cyclePath(rest []string, board SliceBoard) []string {
	inRest := make(map[string]bool, len(rest))
	for _, id := range rest {
		inRest[id] = true
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(rest))
	var path []string
	var walk func(id string) []string
	walk = func(id string) []string {
		color[id] = gray
		path = append(path, id)
		deps := append([]string(nil), board[id]...)
		sort.Strings(deps)
		for _, dep := range deps {
			if !inRest[dep] {
				continue
			}
			switch color[dep] {
			case gray:
				for i, p := range path {
					if p == dep {
						return append(append([]string(nil), path[i:]...), dep)
					}
				}
			case white:
				if cyc := walk(dep); cyc != nil {
					return cyc
				}
			}
		}
		path = path[:len(path)-1]
		color[id] = black
		return nil
	}
	sorted := append([]string(nil), rest...)
	sort.Strings(sorted)
	for _, id := range sorted {
		if color[id] == white {
			if cyc := walk(id); cyc != nil {
				return cyc
			}
		}
	}
	return sorted // 兜底（理论上到不了）: 报剩余节点，不编造环
}
