// Package sliceobs — 片的观测事件（B 项⑤，2026-09-18）。
//
// 一句话：**同一套**片事件机制（B 项③ 落在 api/internal/api/slice_schema.go 的那个出口）
// **扩名不扩机制** —— 事件名从「一个 slice_mount」扩成设计稿 §6.1 的四名闭集
// （`slice_created` / `slice_rejected` / `slice_escalated` / `slice_skipped_by_executor`），
// 落点、形状、best-effort 纪律一字不换。
//
// ── 为什么把出口下沉到这一层（如实登记，不是「第二套机制」）───────────────
//
//	本件要接的三处横跨三个包：挂板 = api · 打回 = loopcore · 第 4 态 = policy。
//	Go 的包依赖在本仓是单向的 `api → loopcore → policy`，故 policy / loopcore
//	**不可能** import api（成环）⇒ 把**同一个**出口下沉到一个只依赖 statepath 的叶子包。
//
//	「同一套」的判据（可核）：
//	  · 形状只有一份 —— 本包的 `Event`；api 侧的 `SliceMountEvent` 现在是它的**别名**；
//	  · 落点只有一份 —— `slice-mount-events.jsonl`（B 项③ 的同一个文件，路径一行不动）；
//	  · 写入函数只有一份 —— `Emit`（api 的 `emitSliceMount` 只是它的调用方）。
//
// ── 每条事件带的字段（设计稿 v2.1 §6.1 + v1.1 §4 逐字，+ 本件任务书）──────
//
//	事件名 · 时间 · 低基数结局 · slice_id / task_id · R 编号（若有）· 判据版本（无来源 ⇒ 未标定）· 可行动 detail
//
// ── B 项⑥（2026-09-18）：**在同一套机制上**加两件东西，事件名闭集**一个都不动** ──
//
//	① **升级信号闭集**（§4.7 六条逐字）+「不在闭集内必须给理由码」的校验 ⇒ `signal` / `reason_code`
//	   两个字段 + `ValidateEscalationSignal` + `EmitSliceEscalatedSignal`（escalation_signal.go）。
//	   **`EventNames()` 仍是四名**：信号闭集是**另一个**闭集（`signal` 的值域），不是第五个事件名。
//	② **越片统计的聚合口径**（§4.8-2）⇒ 纯函数 `AggregateSkips` + 标定读数的两处「未标定」
//	   （skip_aggregate.go）。**不新增事件名、不新增落点、不加第二套出口**。
//
// ── 硬规则（写死在代码里，由用例逐条钉住）────────────────────────────────
//
//	① 事件名闭集：认不出的 `event` 名一律**不落**（不猜、不产生垃圾基数）。
//	   **B9（2026-09-18）扩名**：闭集 = §6.1 切片侧四名 ∪ **§6.2 派活/小队侧三名**
//	   （`squad` / `work_order` / `handoff`，见 team_events.go 的逐字取证）；
//	   **机制不动** —— 形状仍是本文件的 `Event`、落点仍是同一个 JSONL、出口仍是 `Emit`。
//	   `EventNames()` **仍是 §6.1 四名**（那一节的逐字清单，不动、不撑）；全部合法名见 `AllEventNames()`。
//	② 判据版本无来源 ⇒ **显式「未标定」**（常量 CriteriaVersionUncalibrated），绝不编造。
//	③ best-effort：任何写失败只记日志 —— **绝不改变判定、绝不 panic、绝不阻断调用方**
//	   （`Emit` 刻意**没有返回值**：判定侧无从依赖它，也就无从被它改变）。
//	④ **不回显**请求体原文 / 凭据 / 路径机密 —— 只落低基数值（id / 码 / 编号 / 字段名 / 计数）。
package sliceobs

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ── 事件名闭集（设计稿 v2.1 §6.1 逐字；不许自造第五个）─────────────────────
const (
	// EventSliceCreated — 片挂板放行入队（片的诞生）。
	EventSliceCreated = "slice_created"
	// EventSliceRejected — 片被拒：挂板校验拒绝 / 带原因码打回（带 原因码 + R 编号 + 判据版本）。
	EventSliceRejected = "slice_rejected"
	// EventSliceEscalated — 片升级：第 4 态「追问/阻滞」进态或超时结算（fail closed ⇒ 升级给人）。
	EventSliceEscalated = "slice_escalated"
	// EventSliceSkippedByExecutor — 越片动作（执行者的动作相位不在当前片 criteria 允许集内）。
	EventSliceSkippedByExecutor = "slice_skipped_by_executor"
)

// AliasSliceMount — B 项③ 的旧事件名 `slice_mount`。
//
// **归并口径（本件的选择，写在这里也写进回报）**：旧名**不再单独作为 `event` 值发出**，
// 而是写成 `alias` 字段 —— 因为设计稿 §6.1 的事件名闭集里没有 `slice_mount`（自造闭集外的
// 第五个名字 = 冲突不自作主张 ✗）。保留字段而不是删掉旧名的理由：文件是 append-only 的，
// 历史行就在同一份 JSONL 里，旧读取方按 `alias: "slice_mount"` 仍能把挂板事件的另一半认出来。
const AliasSliceMount = "slice_mount"

// CriteriaVersionUncalibrated — 判据版本在「仓内无来源」时的**显式**取值。
//
// 为什么是「未标定」而不是空串、更不是某个看着像版本号的串：
//   - 空串会被读成「没这个字段」，而设计稿 §6.1 要的是「判据版本号」这一**位**必须存在；
//   - 编一个版本号 = 编造来源（本件硬规则 ✗）。
//
// 本仓现状（现读，非记忆）：`scripts/check-slice.py` **没有规则集版本串**这条落点
// （`--list-rules` 输出里只有规则登记表与口径来源，无版本号）；`docs/01-设计/切片合同-模板.md`
// 抬头是 `v1.0`，但它是**文档**不是机器可读的规则集版本源 ⇒ 本件不拿文档版本冒充规则集版本。
//
// ⇒ 于是：**没有调用方给出真来源时，一律写「未标定」**；将来检查器落版本串时，
// 由调用方把串传进 Event.CriteriaVersion 即可（本包不改结构）。
const CriteriaVersionUncalibrated = "未标定"

// ── 低基数结局闭集（每条事件的「结局」都从这几个值里取）─────────────────────
const (
	OutcomeCreated  = "created"  // 片已挂板放行
	OutcomeRejected = "rejected" // 片被拒（挂板校验 / 带原因码打回）
	OutcomeRaised   = "raised"   // 第 4 态进态（退回上游补规格）
	OutcomeRefused  = "refused"  // 第 4 态：上游拒答 ⇒ 升级给人
	OutcomeTimeout  = "timeout"  // 第 4 态：超时作废 ⇒ fail closed、升级给人
	OutcomeSkipped  = "skipped"  // 越片动作
)

// EventNames — **切片侧**（设计稿 §6.1）事件名闭集的枚举形态（统计脚本与用例按它遍历；
// 顺序固定 = 设计稿 §6.1 的行文顺序）。
//
// **本节只列 §6.1 的四名**（B9 未改动本节：§6.1 的逐字清单就这四条）。
// `Emit` 认的**全部**合法名 = 本节 ∪ `TeamEventNames()`（§6.2 三名）⇒ 见 `AllEventNames()`。
func EventNames() []string {
	return []string{
		EventSliceCreated,
		EventSliceRejected,
		EventSliceEscalated,
		EventSliceSkippedByExecutor,
	}
}

// AllEventNames — `Emit` 认的**全部**合法 `event` 值 = §6.1 切片侧四名 + §6.2 派活/小队侧三名。
//
// 两节各自是设计稿里**逐字**列出的清单（§6.1 第 371 行 / §6.2 第 376 行），本函数只是把两节
// 合并成一个**闭集**给 `knownEvent` 用（闭集只有一个，不建第二套名字空间）。
func AllEventNames() []string {
	names := EventNames()
	return append(names, TeamEventNames()...)
}

// knownEvent — 事件名是否在闭集内（① 的判据；闭集 = §6.1 ∪ §6.2，见 AllEventNames）。
func knownEvent(name string) bool {
	for _, n := range AllEventNames() {
		if name == n {
			return true
		}
	}
	return false
}

// Event — 一行片观测事件（append-only 单行 JSON，与 B 项③ 同形状同落点）。
//
// 字段与既有 api.SliceMountEvent **逐字段兼容**（旧线照旧可读）：只是把事件名扩成闭集、
// 多带了 `outcome` / `criteria_version` / `r`（见字段注释）。
type Event struct {
	// Event — 事件名（闭集四名之一；认不出的不落）。
	Event string `json:"event"`
	// Alias — 旧事件名（目前只有挂板事件带 `slice_mount`）—— 归并用，不是第二个名字空间。
	Alias string `json:"alias,omitempty"`
	// Time — RFC3339。
	Time string `json:"time"`
	// OK — 判定是否「通过」（挂板放行 = true；拒绝 / 升级 / 越片 = false）。与 outcome 冗余存一份：
	// 旧读取方（B 项③）按 ok 读，新读取方按 outcome 读，两个都不能读错。
	OK bool `json:"ok"`
	// Outcome — 低基数结局（Outcome* 闭集）。
	Outcome string `json:"outcome"`
	// TaskID / SliceID — 归因用。（缺 slice_id 的挂板拒绝里 slice_id 为空 —— 那时确实没有 id 可用。）
	TaskID  string `json:"task_id,omitempty"`
	SliceID string `json:"slice_id,omitempty"`
	// AgentName — 成员名（`agent_name` **逐字**取自设计稿 v2.1 §4.6-8 第 239 行：
	// `workspace_root / blackboard_dir / allowed_write_paths / confirmation_callback / non_interactive / agent_name`）。
	//
	// 为什么派活/小队侧必须有这一位：§2.3-14 逐字「**对每个 agent 操作与交接都埋点**」⇒ 记录要**按 agent 归因**
	// 才算埋到点（否则 §6.2 的「可统计」只剩全局计数）。片侧事件不带它（omitempty ⇒ 字段整个不出现）。
	AgentName string `json:"agent_name,omitempty"`
	// Target — 交接对象（**只有 `handoff` 记录带它**）。
	//
	// 取值口径：**只认有出处的取值**（`HandoffTargetUser` = `user`，§4.6-7 逐字「「升级给人」统一为 handoff(user)」）；
	// 不在出处闭集内 ⇒ `EmitHandoff` **不落行**（成员名形态稿面未给 ⇒ 不发明）。字段名 `target` 属**本仓命名**
	// （稿面逐字给了取值 `handoffs=["user"]`，没给事件里的字段名）—— 登记为待定口径项。
	Target string `json:"target,omitempty"`
	// Code — 低基数原因码（挂板 `SLICE_*` / 打回 `PRE_`/`POST_`/`INV_`_R<n>）。
	Code string `json:"code,omitempty"`
	// R — R 编号（设计稿 §3.4 的机器判据闭集 R0–R13）。**指针**：nil ⇒ 该事件没有 R 编号
	//     （如挂板校验的 SLICE_* 码）⇒ 字段整个不出现 —— 与「R0 是合法编号」不混淆。
	R *int `json:"r,omitempty"`
	// CriteriaVersion — 判据版本（指向规则集版本串）。**无来源 ⇒ 「未标定」**，由 Emit 兜底。
	CriteriaVersion string `json:"criteria_version"`
	// Signal — 升级信号（设计稿 v2.1 §4.7 的**六条闭集**之一，逐字取值；见 escalation_signal.go）。
	// 只有 `slice_escalated` 事件带它；其它事件不带（omitempty ⇒ 字段整个不出现）。
	//
	// 注意：**这是第二个闭集**（`signal` 的值域），与 §6.1 的**事件名闭集**（`event` 的值域）是两回事，
	// 互不派生、交集为空（用例钉住）。放进事件里而不另立一套观测出口：B 项⑥ 要求「扩机制不另建」。
	Signal string `json:"signal,omitempty"`
	// ReasonCode — 升级裁决的**理由码**（§4.7-9g 逐字：「不在枚举内的「无需升级」必须给理由码」）。
	// 空 ⇒ 字段整个不出现。**本件不做语法校验**（稿面没给理由码的语法，登记见 escalation_signal.go）；
	// 闭集内信号**不要求**它（带了就是补充理由），不在闭集内时**必填**（`ValidateEscalationSignal`）。
	ReasonCode string `json:"reason_code,omitempty"`
	// Field — 出错字段名（挂板校验用）。
	Field string `json:"field,omitempty"`
	// DependsOn — 声明的依赖（排障用）。
	DependsOn []string `json:"depends_on,omitempty"`
	// AcceptanceDeclared — 是否**显式声明**了 acceptance（false = 缺失；true + 空列表 = 显式空）。
	AcceptanceDeclared bool `json:"acceptance_declared"`
	// Detail — 可行动 detail（缺什么 / 怎么改 / 下一步）。**必须是低基数值拼出来的一句**，
	// 不回显调用方给的自由文本原文（硬规则 ④）。
	Detail string `json:"detail,omitempty"`
}

// EventsFile — 片观测事件的落点（append-only JSONL，状态目录下）。
//
// 与 B 项③ **同一个文件**（归并，不是第二个出口）；函数而非常量：测试经 ZERG_STATE_DIR
// 切到临时目录（仓库既有测试隔离习惯，见 internal/chat/obs_*_test.go）。
func EventsFile() string { return statepath.File("slice-mount-events.jsonl") }

// Emit — **唯一写入出口**（best-effort）。
//
// 纪律（逐条对应文件头的硬规则）：
//
//	① 事件名不在闭集内 ⇒ 不落（记日志，不猜、不产生垃圾基数）；
//	② CriteriaVersion 为空 ⇒ 写 CriteriaVersionUncalibrated（显式「未标定」，不编造）；
//	③ 任何一步失败（MkdirAll / OpenFile / Write）⇒ 记日志后**返回**：
//	   **没有返回值**，也就没有任何路径能把「写日志失败」传成「判定改变」；
//	④ 只落本结构里的字段 —— 调用方给不进任何别的原文。
func Emit(ev Event) {
	if !knownEvent(ev.Event) {
		log.Printf("⚠️ sliceobs: 事件名认不出（不在设计稿 §6.1 闭集内）⇒ 不落这一行：%q", ev.Event)
		return
	}
	if strings.TrimSpace(ev.Time) == "" {
		ev.Time = time.Now().Format(time.RFC3339)
	}
	// ② 判据版本：无来源 ⇒ 显式「未标定」（**不编造**）
	if strings.TrimSpace(ev.CriteriaVersion) == "" {
		ev.CriteriaVersion = CriteriaVersionUncalibrated
	}
	if !knownOutcome(ev.Outcome) {
		// 结局认不出 ⇒ 不猜。
		//  · 片侧四名**有**稿面来源的固定结局 ⇒ 退回该事件的固定结局（低基数、可聚合）——行为一字不变；
		//  · 派活/小队侧三名（§6.2）**没有**稿面来源的固定结局 ⇒ **不落这一行**：
		//    低基数结局这一位必须有值，而「猜一个结局」正是硬规则 ✗ ⇒ 与①「认不出的不落」同规。
		if d := defaultOutcome(ev.Event); d != "" {
			ev.Outcome = d
		} else {
			log.Printf("⚠️ sliceobs: 事件 %q 没有稿面来源的固定结局，且调用方给的结局认不出（%q）⇒ 不落这一行"+
				"（低基数结局这一位必须有值；不猜一个结局出来）", ev.Event, ev.Outcome)
			return
		}
	}
	line, err := json.Marshal(ev)
	if err != nil { // 理论上到不了（结构体全可序列化）——留着是为了「绝不 panic」
		log.Printf("⚠️ sliceobs: 事件序列化失败（判定不受影响）：%v", err)
		return
	}
	path := EventsFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("⚠️ sliceobs: 建状态目录失败（判定不受影响，事件丢弃）：%v", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("⚠️ sliceobs: 打开事件文件失败（判定不受影响，事件丢弃）：%v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		log.Printf("⚠️ sliceobs: 写事件失败（判定不受影响，事件丢弃）：%v", err)
		return
	}
}

// knownOutcome — 结局是否在闭集内。
func knownOutcome(o string) bool {
	switch o {
	case OutcomeCreated, OutcomeRejected, OutcomeRaised, OutcomeRefused, OutcomeTimeout, OutcomeSkipped:
		return true
	}
	return false
}

// defaultOutcome — 事件名 → 固定结局（结局漏填时的兜底，保证「低基数结局」这一位 always 有值）。
//
// **只覆盖 §6.1 的片侧四名**：这四名的固定结局有稿面来源（片诞生/被拒/升级/越片）。
// §6.2 的派活/小队侧三名（`squad` / `work_order` / `handoff`）**不在**这里 —— 稿面没给它们的
// 固定结局 ⇒ 返回空串，由 `Emit` 判成「不落行」（**不猜一个结局出来**；见 Emit 的兜底分支）。
func defaultOutcome(event string) string {
	switch event {
	case EventSliceCreated:
		return OutcomeCreated
	case EventSliceRejected:
		return OutcomeRejected
	case EventSliceEscalated:
		return OutcomeRaised
	case EventSliceSkippedByExecutor:
		return OutcomeSkipped
	}
	return ""
}

// EmitSliceSkippedByExecutor — 越片动作（执行者的动作相位不在当前片 criteria 允许集内）
// 落一行 `slice_skipped_by_executor`（设计稿 v2.1 §6.1 的第四名）。
//
// ── 接线现状（如实登记，**不编造触发条件**）────────────────────────────────
//
//	本仓**没有**越片检测的落点：§4.8-2 的「每步动作归到片的相位、统计越片动作数 + 每 N 步重述」
//	在 core/ 零命中（现读）；§4.5-2 的「片边界 / 越片判定」同样未建（登记见
//	Zerg-内部文档/项目文档/v2.5.10/任务表-协作骨架v2.1仓内核对结论-20260918.md 的 B12）。（2026-09-19 二次分家）
//	⇒ 本件只落**事件出口**：将来执行者侧的相位归属判出「越片」后直接调本函数落痕。
//	  **不在本函数里反向推断「什么算越片」** —— 猜触发条件 = 编造（硬规则 ✗）。
//
// ── 参数口径 ──
//
//	criteriaVersion = 判定这次「越片」所依据的规则集版本串（指向模板/检查器版本）。
//	  **空串 ⇒ 写显式「未标定」**（由 Emit 兜底），绝不编造一个看着像版本号的串。
//
// 只落低基数值（片 id / task_id / R 编号）：detail 由本函数用固定句式拼出，
// **不接受调用方的自由文本** —— 免得把请求体原文 / 凭据 / 私有路径带进观测面（硬规则 ④）。
func EmitSliceSkippedByExecutor(sliceID, taskID, criteriaVersion string, r *int) {
	sliceID = strings.TrimSpace(sliceID)
	taskID = strings.TrimSpace(taskID)
	detail := "执行者出现**越片动作**（其动作相位不在当前片 criteria 允许集内）：按 §4.8-2 计数、" +
		"按 `POST_`（执行者责任）归因 —— 片本身没错 ⇒ 不重切，纠正执行相位后重交。"
	if sliceID == "" {
		detail += "（本行缺 slice_id ⇒ 回指不到是哪一片，属观测缺陷，须补齐。）"
	}
	Emit(Event{
		Event:           EventSliceSkippedByExecutor,
		OK:              false,
		Outcome:         OutcomeSkipped,
		SliceID:         sliceID,
		TaskID:          taskID,
		R:               r,
		CriteriaVersion: criteriaVersion, // 空 ⇒ Emit 写「未标定」
		Detail:          detail,
	})
}
