package agent

// tool_decision.go — T1.2：工具调用「允许/拒绝 + 拒绝原因」的可观测化（agent 侧接线）
//
// 设计（为什么长这样）：
//   - 判定散落在工具执行路径的**多个守卫分支**（安全门 gate / 路径白名单 / 参数约定 / 危险命令 / 尺寸 / 类型防呆…），
//     所以观测点必须落在**每个拒绝分支**，而不是在出口处猜（出口只剩一段错误文本，猜=编造原因码）。
//   - 但"每个分支各写一套上报"又会漂：故这里只提供**两个出口**：obsDeny（拒绝）/ obsEndCall（收尾记 allow），
//     分支里只写一行 `ec.obsDeny(toolobs.ReasonXxx)`；安全门那 10 处同构分支进一步由 obsGater 一处集中覆盖。
//   - **只观测**：本文件不参与任何判定、不放宽任何白名单、不改任何错误文本与工具行为。
//     拒绝照样是拒绝，只是"多了一行事实"。
//   - 未登记原因 = 编码缺陷：所有原因码集中在 internal/toolobs 的常量表里（低基数、可枚举、可 grep 审计）。

import (
	"sync"

	"github.com/Mr2109/zerg-swarm/core/internal/toolobs"
)

// toolObsScope — 一次工具调用的观测作用域（ExecContext 内嵌一个；**只观测**，不参与判定）。
//
// 为什么要锁：ExecContext 可被多会话/多 goroutine 复用（chat 流式路径就在 goroutine 里执行工具）。
// 锁只保护观测字段，最坏代价是一条观测挂在相邻调用上——观测可容忍；但绝不因此改变判定与执行。
type toolObsScope struct {
	mu     sync.Mutex
	tool   string // 本次调用的工具名（注册表真名）
	digest string // 本次调用的参数摘要
	denied bool   // 本次调用是否已记过拒绝（决定收尾要不要补 allow）
}

// obsBeginCall — 开启一次调用的观测作用域（在工具名规范化**之前**调用：
// 未知工具名的拒绝也要带得上"模型当时发的是什么名字"）。
func (ec *ExecContext) obsBeginCall(tool string, args map[string]any) {
	digest := toolobs.Digest(args)
	ec.obs.mu.Lock()
	ec.obs.tool = tool
	ec.obs.digest = digest
	ec.obs.denied = false
	ec.obs.mu.Unlock()
}

// obsSetTool — 工具名规范化后回填真名（观测与执行同真名 ⇒ 日志/计数/履历不分裂）。
func (ec *ExecContext) obsSetTool(tool string) {
	ec.obs.mu.Lock()
	ec.obs.tool = tool
	ec.obs.mu.Unlock()
}

// obsDeny — **拒绝分支的统一出口**：记一条 decision=deny（带低基数原因码）。
// 调用点纪律：必须紧贴那个 `return ...`，且**不得**改变返回值/错误文本（观测不改语义）。
func (ec *ExecContext) obsDeny(reason string) {
	ec.obs.mu.Lock()
	ec.obs.denied = true
	tool, digest := ec.obs.tool, ec.obs.digest
	ec.obs.mu.Unlock()
	toolobs.Emit(toolobs.Decision{
		Session:    ec.Session,
		Tool:       tool,
		Decision:   toolobs.DecisionDeny,
		Reason:     reason,
		ArgsDigest: digest,
	})
}

// obsEndCall — 调用收尾：本次**没有任何拒绝** ⇒ 记一条 decision=allow。
//
// 语义（写死，勿混）：allow = **判定层放行**。工具随后因自身原因失败（文件不存在/命令 exit≠0/网络错）
// 不是判定层的事——那由既有工具轨迹（Trace.Error）与 OBS-3 的 result 承担，**不**因此把 allow 改成 deny
// （否则"执行失败"会被读成"被系统拒绝"，又是一种归因错位）。
func (ec *ExecContext) obsEndCall() {
	ec.obs.mu.Lock()
	denied, tool, digest := ec.obs.denied, ec.obs.tool, ec.obs.digest
	ec.obs.mu.Unlock()
	if denied {
		return // 已记拒绝：一次调用只允许一条判定事件
	}
	toolobs.Emit(toolobs.Decision{
		Session:    ec.Session,
		Tool:       tool,
		Decision:   toolobs.DecisionAllow,
		ArgsDigest: digest,
	})
}

// obsGater — 安全门（ToolGater）包装：把 gate 判定变成可观测事件。
//
// 为什么包装而不是在 10 处 gate 分支各写一行：那 10 处（read/write/edit/apply_patch/spawn_agent/todo/
// glob/grep/ls/bash）同构，分散写必漏；包装后 ExecuteTool 一处即可全覆盖。
// 性质：纯透传 —— 返回的 Decision/err 原样交回调用方，工具行为与判定**不变**。
type obsGater struct {
	inner ToolGater
	ec    *ExecContext
}

func (g *obsGater) Check(toolName, args string, agentName string) (Decision, error) {
	d, err := g.inner.Check(toolName, args, agentName)
	switch {
	case err != nil:
		// 门自己报错 ⇒ 调用方一律不执行（现有语义）⇒ 也是"没能允许"，记为拒绝并给出专门原因码
		g.ec.obsDeny(toolobs.ReasonGateError)
	case d.Action == "block":
		g.ec.obsDeny(toolobs.ReasonGateBlock)
	}
	return d, err
}

// ObserveToolAllow — 给**不经过 ExecuteTool** 的工具调用补一条 allow（对话层自执行的 tool_search/kb_search）。
//
// 存在的理由：没有它，"被调用但没记录"会和"没调用"再次混同——正是本任务要消掉的盲区。
// 语义仍是判定层放行（这些工具在 agent 判定层之外，其自身错误照样由轨迹记录）。
func ObserveToolAllow(session, tool string, args map[string]any) {
	toolobs.Emit(toolobs.Decision{
		Session:    session,
		Tool:       tool,
		Decision:   toolobs.DecisionAllow,
		ArgsDigest: toolobs.Digest(args),
	})
}

// ObserveToolDeny — 同上，给判定层之外的调用方补一条 deny（保留给对话层自执行工具的拒绝分支）。
func ObserveToolDeny(session, tool, reason string, args map[string]any) {
	toolobs.Emit(toolobs.Decision{
		Session:    session,
		Tool:       tool,
		Decision:   toolobs.DecisionDeny,
		Reason:     reason,
		ArgsDigest: toolobs.Digest(args),
	})
}
