package api

// ============ B 项③ 片（单子）最小 schema 与挂板校验（2026-09-18）============
//
// 依据（只引用、不改它们）:
//
//	docs/01-设计/切片合同-模板.md                      — 字段名 / 必须字段 / 冲突优先级
//	docs/01-设计/设计-任务模块骨架-v1.0-20260918.md §0 — 三层术语: 任务(mission_id) → 片/单子(slice_id) → 步
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
}

// Declared 是否声明了**任何**片字段 —— 挂板校验的总开关。
// 返回 false ⇒ 该任务不是片 ⇒ 完全跳过校验（这是「不改现有任务提交行为」的机制本身）。
func (in SliceInput) Declared() bool {
	return in.SliceID != "" || in.Owner != "" || in.DependsOn != nil || in.Acceptance != nil
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
}

// SliceInputFromTask 从已建 Task 反推挂板入参（入队闸与 SubmitSlice 的校验都走它——单一形状）。
func SliceInputFromTask(t *Task) SliceInput {
	if t == nil {
		return SliceInput{}
	}
	return SliceInput{
		SliceID:    t.SliceID,
		DependsOn:  t.DependsOn,
		Owner:      t.Owner,
		Acceptance: t.Acceptance,
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
//	in    = 待挂板片的片字段（含「有没有写过 acceptance」这一区分）
//	board = 板上**已有**片的视图（调用方给，**不含本片**；本片由本函数自己加进图里）
//
// 返回 nil ⇒ 可入队；返回 *SliceValidationError ⇒ **拒绝入队**。
// 判定顺序固定（先「有没有 id」再「声明齐不齐」再「引用真不真」再「图有没有环」）——
// 报错只报第一条命中的，且每条都点明「缺什么 + 怎么写对」。
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
