// obs_behavior.go — T1.3「早退/空转」行为信号的落盘入口（写进**既有** chat_obs.jsonl，不另起文件）
//
// 设计稿：docs/01-设计/设计-v2.5.10-对话观测面-20260916.md（H8/I5）
// 事实来源：internal/behaviorobs（叶子包——轮次循环只上报事实，chat 决定落到哪一行）
//
// 为什么必须落盘（实测坐实）：1:1 真考题里「执行模型零改动、只读不写就交差」与「正常干完活」
// 在观测面**同形**（都只有若干条 OBS-3 工具行 + 一条正常收尾的 turn 记录）⇒ 只能事后翻 git 才知道它没干活。
// 本文件把这一组反模式信号变成可判定观测：轮级一条（kind=behavior，event_name=behavior_round）+
// 会话级一条结论（kind=behavior，event_name=behavior_session，带 early_exit_suspected）。
//
// 三条纪律（与 obs.go 文件头三条铁律同源）：
//  1. **观测绝不改变行为**：这里只写盘；不参与任何判定/收尾/闸门。
//  2. **best-effort**：写失败只记日志（obsWrite 已保证），leaf 侧的 sink panic 另有兜底。
//  3. **0 与「未知」可分**：轮级块内部字段**不带 omitempty**（0 是有效测量值：本轮零工具调用就是 0）；
//     会话级结论用 tracked 表达"本包到底收到过轮次事实没有"——没收到就不下结论（不编造）。
package chat

import (
	"github.com/Mr2109/zerg-swarm/core/internal/behaviorobs"
)

// obsKindBehavior — T1.3 记录种类。**不占用既有 kind**（turn/compact/tool）：
// 这些是"行为反模式信号"，与"这一轮跑了多久"（turn）、"调用了什么工具"（tool/decision）是不同的问题，
// 按 kind 过滤的既有读侧（长跑报告、工具统计）不受影响；按 event_name 找事件的一律能找到。
const obsKindBehavior = "behavior"

const (
	obsEventBehaviorRound   = "behavior_round"   // 轮级：本轮调用数 + 累计行为信号
	obsEventBehaviorSession = "behavior_session" // 会话级结论（early_exit_suspected）
)

// BehaviorObs — 轮级行为信号块（T1.3 a/b/c/d）。
//
// ⚠ 与 TurnObs 同口径：内部字段**一律不带 omitempty** —— 0 是有效测量值
// （"本轮零工具调用"就是 0，"从未写过"= turns_since_last_write 递增 + wrote_ever:false），
// 用 omitempty 抹掉会把"测到 0"读成"没有数据"。
type BehaviorObs struct {
	ToolCallsThisTurn        int  `json:"tool_calls_this_turn"`         // a) 本轮工具调用数
	TurnsWithoutToolCall     int  `json:"turns_without_tool_call"`      // b) 累计**连续**零工具调用轮数
	TurnsSinceLastWrite      int  `json:"turns_since_last_write"`       // c) 距上次「写类工具成功」的轮数
	TurnsBeforeFirstToolCall int  `json:"turns_before_first_tool_call"` // d) 首次工具调用之前的轮数
	FirstToolCallSeen        bool `json:"first_tool_call_seen"`         // d) false ⇒ 上面那个数是"至今仍无调用"的轮数
	WroteEver                bool `json:"wrote_ever"`                   // 本会话到目前有没有写过（写类工具成功）
	WriteCallsThisTurn       int  `json:"write_calls_this_turn"`        // 本轮**成功**的写类工具数
	TotalToolCalls           int  `json:"total_tool_calls"`             // 至今累计工具调用数
	TotalWriteOK             int  `json:"total_write_ok"`               // 至今累计写类工具成功数
}

// BehaviorSessionObs — 会话级总结块（T1.3 e）。这里才出现 early_exit_suspected：
// 轮级块**不预判**会话结论（那会是编造——会话还没结束，判据还不知道）。
type BehaviorSessionObs struct {
	Rounds                   int    `json:"rounds"`                       // 跑过的轮数（内核口径）
	ExitKind                 string `json:"exit_kind,omitempty"`          // 退出原因（natural/max_rounds/…）
	FinalAnswer              bool   `json:"final_answer"`                 // 是否**已给出终答**（判据的一半）
	TotalToolCalls           int    `json:"total_tool_calls"`             // 整会话工具调用数
	TotalWriteOK             int    `json:"total_write_ok"`               // 整会话写类工具成功数
	TurnsWithoutToolCall     int    `json:"turns_without_tool_call"`      // 收尾时的连续零工具调用轮数
	TurnsSinceLastWrite      int    `json:"turns_since_last_write"`       // 收尾时距上次写的轮数
	FirstToolCallSeen        bool   `json:"first_tool_call_seen"`         // **整会话零工具调用 ⇒ false**（显式标记 d）
	TurnsBeforeFirstToolCall int    `json:"turns_before_first_tool_call"` // 首次调用之前的轮数（零调用时=至今轮数）
	WroteEver                bool   `json:"wrote_ever"`                   // 整会话有没有写过
	Tracked                  bool   `json:"tracked"`                      // 本包是否收到过该会话的轮次事实（false ⇒ 不下结论）
	EarlyExitSuspected       bool   `json:"early_exit_suspected"`         // e) 整会话零写类成功 **且** 已给出终答
}

// obsBehaviorWire — T1.3 唯一接线点：把轮次循环侧的行为事实（internal/behaviorobs）接到观测面。
//
// 与 T1.2 的 obsToolDecisionWire 同一理由：依赖方向 chat → agent → loopcore 单向，loopcore 侧
// **不能** import chat（成环）⇒ loopcore 只上报到无依赖的叶子包，由这里（chat）决定落盘。
// 未接 sink 时事件丢弃（不是错误）；本函数在进程内只注册一次。
func init() {
	behaviorobs.SetSink(behaviorobs.Sink{
		Round:   ObsBehaviorRound,
		Session: ObsBehaviorSession,
	})
}

// ObsBehaviorRound — T1.3 轮级：一行行为信号（best-effort，写失败只记日志）
func ObsBehaviorRound(s behaviorobs.Snapshot) {
	obsWrite(ObsRecord{
		Kind: obsKindBehavior, Session: s.Session, Round: s.Round,
		EventName: obsEventBehaviorRound,
		Behavior: &BehaviorObs{
			ToolCallsThisTurn:        s.ToolCallsThisTurn,
			TurnsWithoutToolCall:     s.TurnsWithoutToolCall,
			TurnsSinceLastWrite:      s.TurnsSinceLastWrite,
			TurnsBeforeFirstToolCall: s.TurnsBeforeFirstToolCall,
			FirstToolCallSeen:        s.FirstToolCallSeen,
			WroteEver:                s.WroteEver,
			WriteCallsThisTurn:       s.WriteCallsThisTurn,
			TotalToolCalls:           s.TotalToolCalls,
			TotalWriteOK:             s.TotalWriteOK,
		},
	})
}

// ObsBehaviorSession — T1.3 会话级结论：一行（early_exit_suspected 的**唯一**出处）
func ObsBehaviorSession(sum behaviorobs.Summary) {
	obsWrite(ObsRecord{
		Kind: obsKindBehavior, Session: sum.Session,
		EventName: obsEventBehaviorSession,
		BehaviorSession: &BehaviorSessionObs{
			Rounds:                   sum.Rounds,
			ExitKind:                 sum.ExitKind,
			FinalAnswer:              sum.FinalAnswer,
			TotalToolCalls:           sum.TotalToolCalls,
			TotalWriteOK:             sum.TotalWriteOK,
			TurnsWithoutToolCall:     sum.TurnsWithoutToolCall,
			TurnsSinceLastWrite:      sum.TurnsSinceLastWrite,
			FirstToolCallSeen:        sum.FirstToolCallSeen,
			TurnsBeforeFirstToolCall: sum.TurnsBeforeFirstToolCall,
			WroteEver:                sum.WroteEver,
			Tracked:                  sum.Tracked,
			EarlyExitSuspected:       sum.EarlyExitSuspected,
		},
	})
}
