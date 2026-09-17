// Package loopcore — 对话工具循环内核（2026-09-05 第一步: 对话循环两份合一）
// 单一循环实现——流式回调可选注入（nil = 非流式行为）——后续 CA/Flow 换装此内核
// 沉淀自 chat_handlers.go 流式循环（五重防护/心跳/引导收尾）+ chat_tools.go RunToolLoop（历史版本）
package loopcore

import (
	"context"
	"time"
)

// Response — 模型响应（内核自有类型——调用方适配）
type Response struct {
	Content     string
	Reasoning   string
	ToolCalls   []ToolCall
	Finish      string
	TotalTokens int64
}

// ToolCall — 工具调用（内核自有）
type ToolCall struct {
	ID      string
	Name    string
	Args    map[string]any
	RawArgs string
}

// ToolGater — 安全门接口（内核自有——agent.ToolGater 同构）
type ToolGater interface {
	Check(toolName, args string, agentName string) (Decision, error)
}

// Decision — gate 决策
type Decision struct {
	Action  string
	Message string
}

// Infer — 模型调用抽象
// deltaType: "reasoning"/"output"——text: 增量文本
//
// onDelta 契约（2026-09-16 主控 panic 事故后写明，见 delta.go）：
//   - 内核经 Deps.callInfer 统一出口调用，**永远不传裸 nil** —— 无增量消费者的收尾轮传
//     NoopDelta（非 nil、调用安全、不产出任何 delta）。
//   - 实现方**仍应**对 onDelta 判 nil（别的调用方可能直接调本函数），包装可选回调时更必须
//     保留 nil 语义（真 nil 跳过 / 非 nil 原样透传），不得无条件调用。
type Infer func(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
	onDelta func(deltaType, text string), tools []map[string]any) (*Response, error)

// ToolExec — 工具执行抽象（返回 content/error/duration）
type ToolExec func(ctx context.Context, name string, args map[string]any) (content, duration string, execErr error)

// EventSink — 循环事件回调（SSE 透传/日志——可 nil）
// event: tool_start/tool/tool_ping/retry/retry_done/loop_hint/compacting
type EventSink func(event, payload string)

// Config — 循环配置
type Config struct {
	MaxRounds    int           // 最大轮数（对话 10）
	WallClock    time.Duration // 总墙钟（对话 600s——0=不限）
	RoundTimeout time.Duration // 单轮推理限时（对话 120s——0=不限）
	KeepRecent   int           // 上下文轻量化保留工具结果数（对话 3）
	WorkDir      string        // 工具工作目录（ChatToolsWorkDir）
}

// Deps — 依赖注入
type Deps struct {
	Infer        Infer
	Exec         ToolExec         // nil = 不执行工具（纯对话）
	Gate         ToolGater        // 安全门（可 nil）
	Tools        []map[string]any // 工具定义（chat 格式——nil=纯对话）
	Events       EventSink        // 事件回调（可 nil）
	HermesRaw    bool             // 工具结果回传用 Hermes <tool_response> 包装（对话模式）
	Hooks        Hooks            // 渐进式常驻/错误桶钩子（可零值——chat 包适配注入）
	Terminator   Terminator       // 终止仲裁（可 nil——nil=模型无工具调用即自然终止）
	OnToolResult OnToolResult     // 工具结果钩子（可 nil——CA 侧产出验证）
	// T1.2 观测：会话 id（**只用于观测**）。内核侧的拒绝分支（工具被隐藏 ⇒ 不执行）要能落观测面，
	// 且要挂上会话的 trace/span 骨架；空串=无会话（骨架缺席，不编造）。
	Session string

	// ── T5.5 检查点（每步快照 + 耐久分档 + 版本标记；见 checkpoint.go）──
	//
	//	Checkpoints —— 检查点存储；**非 nil 且 RunID 非空**才写（少了任何一个：宁可不写，
	//	              也不写一份"归不了属/取不回来"的快照）。
	//	RunID       —— 检查点归属（与 Session 分开：一个会话可以跑多轮 run，恢复点属于 run）。
	//	Resume      —— 恢复入口：非 nil ⇒ 从该快照继续（消息/轨迹/已完成步数都来自它）。
	//	              **恢复必须与 T5.6 的 Lease 一起用** —— "并发两次恢复 ⇒ 效果恰好一次"
	//	              靠的是 lease 前置门 + 完成判定，不是靠快照本身。
	Checkpoints *CheckpointStore
	RunID       string
	Resume      *Snapshot

	// ── T5.6 并发恢复互斥：session 级 lease（**执行前置门**；见 lease.go）──
	//
	//	Lease    —— 非 nil 且 Session 非空 ⇒ 在**任何节点执行之前**先认领该会话；
	//	            认领失败 ⇒ 立即返回（ExitKind="lease_rejected"，一条节点都不执行）。
	//	Holder   —— 认领者标识（空 ⇒ host/pid 兜底：不猜"我是谁"，也绝不空着当认领成功）。
	//	LeaseTTL —— 租约时长（≤0 ⇒ DefaultLeaseTTL）。
	Lease    *LeaseStore
	Holder   string
	LeaseTTL time.Duration

	// Now —— 时钟注入（nil = time.Now）：只用于租约/快照的时间戳，**不参与任何判定**
	//（判据一律来自显式输入，与 T5.9 审计层同一纪律）。
	Now func() time.Time
}

// Hooks — 渐进式常驻钩子（chat.ToolRuntime 的行为接口——内核不依赖 chat 包）
type Hooks struct {
	IsHidden      func(name string) bool                           // 工具是否被隐藏（3 次 exec 失败）
	RecordOutcome func(name, errType, errMsg string) (hint string) // 成败记录（返回提示或空）
	SearchStreak  func(query string) (count int, forceHint string) // 重复搜索计数（返回次数+强制提示或空）
}

// Result — 循环结果
type Result struct {
	Content   string
	Reasoning string
	ToolCalls []ToolCall
	Finish    string
	Usage     struct {
		TotalTokens int64
	}
	Traces   []Trace
	ExitKind string // natural/bad_format/wall_clock/round_timeout/stream_broken/loopguard_escalate/empty_args/max_rounds/lease_rejected/already_done
	Err      string // 模型调用失败（重试后仍败）——非空=异常终止
	// CheckpointErr —— T5.5：某一步的快照**没写成**（磁盘满/权限/刷盘失败）。
	// 为什么不把它塞进 Err 让整轮失败：快照写不成不该改变循环的行为（它不是新的失败模式），
	// 但也**绝不静默** —— 调用方据此知道"这个 run 的恢复点不成立"（并已同步发 checkpoint_failed 事件）。
	CheckpointErr string
}

// Terminator — 终止仲裁接口（内核在模型无工具调用时征询——CA 传契约判定/对话传 nil=自然终止）
// OnNoToolCall 返回 (终止?, 终止后追加给模型的引导消息——空=不追加)
type Terminator interface {
	OnNoToolCall(result *Response) (done bool, feedback string)
}

// OnToolResult — 工具结果钩子（CA 侧验证文件真实存在→verifiedOutput 等定制点）
type OnToolResult func(tc ToolCall, content string, execErr error)

// Trace — 工具轨迹（落库）
type Trace struct {
	Round    int    `json:"round"`
	CallID   string `json:"call_id"`
	Name     string `json:"name"`
	Args     string `json:"args"`
	Result   string `json:"result"`
	Error    string `json:"error,omitempty"`
	Duration string `json:"duration,omitempty"`
}
