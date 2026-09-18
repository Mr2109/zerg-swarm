// team_events.go — B 项 B9：**派活/小队层事件族**（设计稿 v2.1 §6.2 逐字三名 + 出口）。
//
// ── 逐字来源（本件现读，不凭记忆）────────────────────────────────────────
//
//	docs/01-设计/设计-协作骨架-v2.1-20260918.md
//	  第 374 行：`### 6.2 派活/小队侧事件（A §3.4）`
//	  第 376 行：「新增 `squad` / `work_order` / `handoff` 记录（沿用 OBS 格式，可统计）；
//	              「**对每个 agent 操作与交接都埋点**」（§2.3-14）；打回/升级/追问必留痕（§4.7）。」
//	  第 212 行（§4.4-3 与现有实现的接点）：「- 观测面：新增 `squad` / `work_order` / `handoff` 记录
//	              （沿用 OBS 格式，可统计）；**外接后端需另做 OTel GenAI 映射，见 §6.5**；」
//	  第 400 行（§6.5）：「- **本仓落点**：§6.1 的自研事件 + §6.2 的 `squad`/`work_order`/`handoff`
//	              **保持不变**；OTel 这层是**外接后端的映射**（只映射，**不改自研事件语义**）。」
//
// ── 稿面**给了什么 / 没给什么**（这是本件的全部取值依据，逐条列清）─────────────
//
//	给了：
//	  · 三个**记录名**，逐字、带反引号：`squad` / `work_order` / `handoff`（§6.2 第 376 行；§4.4-3 第 212 行同句）；
//	  · 「**沿用 OBS 格式**」⇒ 形状用既有 `Event`（本包的同一个结构体，不新建第二份）；
//	  · 「对每个 agent 操作与交接都**埋点**」（§2.3-14，第 87 行；原文 "Instrument all agent operations and handoffs."）
//	    ⇒ 记录必须**按 agent 归因** ⇒ 用逐字字段名 `agent_name`（§4.6-8，第 239 行：`… / agent_name`）；
//	  · 交接对象的一个**逐字取值** `user`（§4.6-7，第 238 行：交接白名单 + `handoffs=["user"]`，
//	    并明写「**「人」是一等交接对象** ⇒ **「升级给人」统一为 handoff(user)**」；
//	    同一取值另有一处独立来源：docs/01-设计/设计-内建调试版-v1.1-20260917.md 第 81 行
//	    「与 autogen 的 **`handoffs=["user"]`** 同源 ⇒ 两条独立来源印证，采之」）；
//	  · 挂板 id 的硬规矩：§4.2-2（第 197 行）「**无 task_id 的委派一律拒绝**」；
//	    §2.1-1（第 66 行）同一硬规矩的上游原文逐字含 `team_task_id`。
//
//	**没给**（⇒ 本件一律**不自造**，只登记待定；清单见本文件末与回报）：
//	  · 这三个记录名**下面的分步/动作事件名**（例如 `squad_created` 之类）—— §6.2 **没有**；
//	    对照 §6.1（第 371 行）是明写「新增事件：`slice_created` / `slice_rejected` …」⇒ 两侧体例不同，
//	    不能把 §6.1 的后缀惯例**移植**过来当 §6.2 的名字（移植 = 自造 ✗）。
//	  · 这三个记录各自的**字段清单**（§6.2 只写了「沿用 OBS 格式」，没像 §6.1 那样写「带 原因码 + R 编号 + 判据版本」）；
//	  · 「结局 / ok / 覆盖率 / 理由码」在这三名上的口径；
//	  · 交接对象的**其它取值形态**（白名单是成员**声明式**声明的 ⇒ 成员名形态稿面未给）；
//	    ⇒ 本件只认逐字的 `user`，闭集外**不落行**（与升级信号「不在闭集内不落」同规），登记待定。
//
// ── 落点 / 形状 / 出口：**与片事件是同一套，不新建第二套**（硬规则）────────────
//
//	· 形状 = `Event`（本包唯一结构体，本节不新建形状）；
//	· 落点 = `EventsFile()`（= B 项③ 的同一个 `slice-mount-events.jsonl`，路径一行不动）；
//	· 出口 = `Emit`（本包唯一写入函数；本节只组装事件，不自己开文件）。
//
// ── 硬规则（与 sliceobs.go 同规，由用例逐条钉住）────────────────────────────
//
//	① 闭集：认不出的 `event` 名一律**不落**；
//	② 判据版本无来源 ⇒ 显式「未标定」（沿用 `CriteriaVersionUncalibrated`；**不编造**）；
//	③ best-effort：本文件的三个函数**都没有返回值** ⇒ 判定侧无从依赖、也就无从被它改变；
//	④ **不回显**请求体原文 / 凭据 / 路径机密 —— 只落低基数值（agent_name / 记录名 / 低基数结局 / 交接对象），
//	   且 detail 一律由本文件用固定句式拼出，**不接受调用方给的自由文本**。
package sliceobs

import (
	"log"
	"strings"
)

// ── 事件名闭集（派活/小队层）：设计稿 §6.2 第 376 行**逐字三名**，不许自造分步名 ─────
//
// 体例说明（为什么这里只有三个「光名字」）：§6.1 的切片侧是「动词后缀」四名（`slice_created`…），
// §6.2 的派活侧稿面**只写了三个记录名**（`squad` / `work_order` / `handoff`，无后缀）。
// 逐字用稿面的三个名字；**不把 §6.1 的后缀惯例移植过来**（那样造出来的名字稿面没有 ⇒ 硬规则 ✗）。
const (
	// EventSquad — 小队侧记录（§6.2 逐字）。
	EventSquad = "squad"
	// EventWorkOrder — 派活/单子侧记录（§6.2 逐字）。
	EventWorkOrder = "work_order"
	// EventHandoff — 交接记录（§6.2 逐字）。
	EventHandoff = "handoff"
)

// TeamEventNames — 派活/小队层记录名的枚举形态（顺序 = §6.2 行文顺序：`squad` / `work_order` / `handoff`）。
//
// **与 `EventNames()` 是同一个闭集的两节**（§6.1 切片侧四名 + §6.2 派活侧三名），
// 合起来才是 `Emit` 认的**全部**合法 `event` 值 —— 见 `AllEventNames()`。
func TeamEventNames() []string {
	return []string{EventSquad, EventWorkOrder, EventHandoff}
}

// ── 交接对象：稿面逐字只有一个取值 `user`（+ 两处独立来源印证）──────────────────

// HandoffTargetUser — 「人」是一等交接对象。
//
// 逐字来源（两处独立）：
//   - docs/01-设计/设计-协作骨架-v2.1-20260918.md 第 238 行（§4.6-7）：「**交接白名单 + `handoffs=["user"]`**……
//     **「人」是一等交接对象** ⇒ **「升级给人」统一为 handoff(user)**」；
//   - docs/01-设计/设计-内建调试版-v1.1-20260917.md 第 81 行：「与 autogen 的 **`handoffs=["user"]`** 同源 ⇒
//     两条独立来源印证，采之。」
const HandoffTargetUser = "user"

// SourcedHandoffTargets — **有稿面出处**的交接对象闭集。
//
// **目前只有一个成员**（`user`）。为什么不做成开放取值：稿面把白名单定成「成员**声明式**声明」，
// 但**没给成员名的形态**（§4.6-7 只逐字给了 `user`）⇒ 认不出的对象**不落行**（与升级信号闭集同规：
// 认不出时不猜），并把「成员名形态」登记为待 Mr2109 定的口径项（见文件头「没给」清单）。
// 将来稿面给出成员名形态时，只需在此处与 `IsSourcedHandoffTarget` 一处扩展（形状不动）。
func SourcedHandoffTargets() []string { return []string{HandoffTargetUser} }

// IsSourcedHandoffTarget — 交接对象是否有出处（**逐字比较**：不归一化、不大小写折叠、不认近义词）。
func IsSourcedHandoffTarget(target string) bool {
	for _, t := range SourcedHandoffTargets() {
		if target == t {
			return true
		}
	}
	return false
}

// ── 低基数结局的**共同口径**（三名共用；登记：稿面未给这三名的结局口径）──────────

// teamOutcomeOK — `ok` 取值口径（**本仓口径，登记**）。
//
// 稿面没写这三名记录怎么算「通过」（对照片侧：`slice_created` = ok true，其余 = false —— 见 api 侧组装）。
// 本件沿用片侧同一形状：**结局 = `OutcomeCreated` ⇒ ok=true，其余结局 ⇒ ok=false**。
// 结局本身**必须**由调用方给出、且必须落在既有六值闭集内（`created` / `rejected` / `raised` /
// `refused` / `timeout` / `skipped`）；**不给或认不出 ⇒ 不落行**（`Emit` 的兜底：
// 这三名**没有**稿面来源的固定结局 ⇒ 不猜一个结局出来 —— 见 sliceobs.go 的 `defaultOutcome`）。
func teamOutcomeOK(outcome string) bool { return outcome == OutcomeCreated }

// teamDetailMissing — 归因字段缺失时的固定句式尾注（低基数、可行动，不含任何自由文本）。
func teamDetailMissing(field, why string) string {
	return "（本行缺 " + field + " ⇒ " + why + "，属观测缺陷，须补齐。）"
}

// ── 三个记录的唯一出口（**同一套**机制：唯一写入出口仍是 Emit）──────────────────

// EmitSquad — 落一行 `squad`（§6.2 的小队侧记录）。
//
// ── 参数口径 ──
//
//	agentName = 成员名（§2.3-14「对每个 agent 操作与交接都埋点」⇒ **按 agent 归因**；
//	            字段名 `agent_name` 逐字取自 §4.6-8 第 239 行）。空 ⇒ 存活，但 detail 点明观测缺陷。
//	taskID    = 挂板 id（§4.2-2「无 task_id 的委派一律拒绝」）。可空 ⇒ detail 点明观测缺陷。
//	outcome   = **必填**且必须是既有低基数结局闭集里的值；认不出 ⇒ **不落行**（不猜结局）。
//	criteriaVersion = 空串 ⇒ 写显式「未标定」（由 Emit 兜底），**不编造**版本串。
//
// 不落：请求体原文 / 凭据 / 路径机密 / 调用方自由文本 —— detail 由本函数用固定句式拼出（硬规则 ④）。
//
// **不做**分步事件名（`squad_created` 之类稿面没有 ⇒ 不造）：本函数落的是「§4.3 生命周期里
// 发生过一步」这一条记录，**哪一步由调用方按稿面自己的流程名去描述**（登记待定）。
func EmitSquad(agentName, taskID, outcome, criteriaVersion string, r *int) {
	agentName = strings.TrimSpace(agentName)
	taskID = strings.TrimSpace(taskID)
	detail := "小队侧记录（设计稿 §6.2 的 `squad` 记录；稿面明写「**沿用 OBS 格式**，可统计」⇒ 与片事件同形状同落点）。" +
		"§2.3-14 逐字「对每个 agent 操作与交接都**埋点**」（原文 \"Instrument all agent operations and handoffs.\"）" +
		"⇒ 本行按 `agent_name` 归因；§4.3 的团体生命周期（接任务 ⇒ 组队 ⇒ 提分工方案 ⇒ 等 Mr2109 确认 ⇒ 挂板 + 发单子 ⇒ " +
		"成员执行 ⇒ 验证 ⇒ 汇总 ⇒ 交付 ⇒ **团队解散、能力回池**）里**发生过一步**即落一行。" +
		"（§6.2 **只给了这一个记录名，没给分步事件名与字段清单** ⇒ 分步名一律不自造，登记待 Mr2109 定。）"
	if agentName == "" {
		detail += teamDetailMissing("agent_name", "按 §2.3-14「对每个 agent 操作与交接都埋点」回指不到是哪个成员")
	}
	Emit(Event{
		Event:           EventSquad,
		OK:              teamOutcomeOK(outcome),
		Outcome:         outcome, // 必填：认不出 ⇒ Emit 不落这一行（不猜结局）
		TaskID:          taskID,
		AgentName:       agentName,
		R:               r,
		CriteriaVersion: criteriaVersion, // 空 ⇒ Emit 写「未标定」
		Detail:          detail,
	})
}

// EmitWorkOrder — 落一行 `work_order`（§6.2 的派活/单子侧记录）。
//
// ── 参数口径 ──
//
//	sliceID   = 片 = 单子（§4.10 并轨定论：「片 = 单子」，同一实体、同一 `slice_id`/`task_id`）。
//	taskID    = 挂板 id；§4.2-2 逐字「**无 task_id 的委派一律拒绝**」⇒ 空即观测缺陷（detail 点明）。
//	agentName = 接单成员（同上，按 agent 归因）；空 ⇒ detail 点明观测缺陷。
//	outcome   = 必填、取既有低基数结局闭集；认不出 ⇒ **不落行**。
//	criteriaVersion = 空 ⇒ 「未标定」（不编造）。
//
// 只落低基数值（片 id / 挂板 id / 成员名 / 结局 / R 编号）：detail 由本函数拼固定句式，
// **不接受调用方的自由文本**（免得把单子的目标原文 / 允许工具集清单 / 额度配置带进观测面 —— 硬规则 ④）。
func EmitWorkOrder(sliceID, taskID, agentName, outcome, criteriaVersion string, r *int) {
	sliceID = strings.TrimSpace(sliceID)
	taskID = strings.TrimSpace(taskID)
	agentName = strings.TrimSpace(agentName)
	detail := "派活/单子侧记录（设计稿 §6.2 的 `work_order` 记录；稿面明写「**沿用 OBS 格式**，可统计」）。" +
		"§4.2-1 单子制：每张单子 = 一片（`目标一句话 + 验收标准 + 允许工具集 + 额度（读标定档案）+ 至顶后终局`）；" +
		"§4.2-2 **强制挂板**：Lead 的每一次委派必须挂到任务板，**无 task_id 的委派一律拒绝**。" +
		"⇒ 本行只落「这张单子被挂板/派发/驳回」这一条痕，**不回显单子正文**（目标原文、工具集、额度都不进来）。" +
		"（§6.2 未给本记录的分步事件名与字段清单 ⇒ 不自造，登记待 Mr2109 定。）"
	if taskID == "" {
		detail += teamDetailMissing("task_id", "§4.2-2 逐字「无 task_id 的委派一律拒绝」⇒ 这条记录回指不到挂板项")
	}
	if sliceID == "" {
		detail += teamDetailMissing("slice_id", "§4.10 并轨定论「片 = 单子」⇒ 回指不到是哪一张单子")
	}
	Emit(Event{
		Event:           EventWorkOrder,
		OK:              teamOutcomeOK(outcome),
		Outcome:         outcome,
		SliceID:         sliceID,
		TaskID:          taskID,
		AgentName:       agentName,
		R:               r,
		CriteriaVersion: criteriaVersion,
		Detail:          detail,
	})
}

// EmitHandoff — 落一行 `handoff`（§6.2 的交接记录）。
//
// ── 参数口径 ──
//
//	fromAgent = 发起交接的成员（存进 `agent_name`：§2.3-14「对每个 agent 操作与交接都埋点」）；
//	            空 ⇒ detail 点明观测缺陷。
//	target    = 交接对象。**只认有出处的取值**（`HandoffTargetUser` = `user`，§4.6-7 逐字
//	            「「人」是一等交接对象 ⇒ 「升级给人」统一为 handoff(user)」）。
//	            **不在出处闭集内 ⇒ 记日志、不落这一行**（成员名形态稿面未给 ⇒ 不发明；
//	            与「升级信号不在闭集内不落」同规）。
//	sliceID / taskID = 片 / 挂板 id（可空 ⇒ detail 点明观测缺陷）。
//	outcome   = 必填、取既有低基数结局闭集；认不出 ⇒ **不落行**。
//	criteriaVersion = 空 ⇒ 「未标定」（不编造）。
//
// 不落自由文本（硬规则 ④）：detail 固定句式；`target` 也只可能是出处闭集里的那一个字面值。
func EmitHandoff(fromAgent, target, sliceID, taskID, outcome, criteriaVersion string, r *int) {
	// **不修剪 target**：出处闭集是逐字的 ⇒ 带首尾空白的写法**不是**那个条目
	// （修剪会把「写法不对」洗成「对了」，等于悄悄放宽闭集 ⇒ 与「认不出不猜」相反）。
	if !IsSourcedHandoffTarget(target) {
		log.Printf("⚠️ sliceobs: 交接对象认不出（稿面逐字只给了 %q 这一个取值）⇒ 不落这一行：target=%q "+
			"（成员名形态稿面未给 ⇒ 不发明；判定不受影响）", HandoffTargetUser, target)
		return
	}
	fromAgent = strings.TrimSpace(fromAgent)
	sliceID = strings.TrimSpace(sliceID)
	taskID = strings.TrimSpace(taskID)
	detail := "交接记录（设计稿 §6.2 的 `handoff` 记录；稿面明写「**沿用 OBS 格式**，可统计」）。" +
		"§4.6-7 逐字：「交接白名单 + `handoffs=[\"user\"]`：成员**声明式**声明可交接对象；" +
		"**「人」是一等交接对象** ⇒ **「升级给人」统一为 handoff(user)**」。本行交接对象 = " + target +
		"（出处闭集内唯一取值）⇒ 属于「升级给人」这一类交接；§6.2 尾「打回/升级/追问必留痕（§4.7）」" +
		"⇒ 与 `slice_escalated`（片侧升级留痕）配套读，两条痕各记一侧。" +
		"（§6.2 未给本记录的分步事件名与字段清单 ⇒ 不自造，登记待 Mr2109 定。）"
	if fromAgent == "" {
		detail += teamDetailMissing("agent_name", "按 §2.3-14「对每个 agent 操作与交接都埋点」回指不到是哪个成员发起的交接")
	}
	if sliceID == "" {
		detail += teamDetailMissing("slice_id", "回指不到是哪一片涉及的交接")
	}
	Emit(Event{
		Event:           EventHandoff,
		OK:              teamOutcomeOK(outcome),
		Outcome:         outcome,
		SliceID:         sliceID,
		TaskID:          taskID,
		AgentName:       fromAgent,
		Target:          target,
		R:               r,
		CriteriaVersion: criteriaVersion,
		Detail:          detail,
	})
}
