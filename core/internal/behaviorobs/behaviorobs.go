// behaviorobs.go — 「早退/空转」行为反模式信号的观测出口（任务表 v2.5.10 / T1.3；设计稿 H8、I5）
//
// 要治的盲区（一句话，已被 1:1 真考题实测坐实）：
//
//	**「执行模型零改动、只读不写就交差」在观测面看不出来** —— 它和「正常干完活」长得一样
//	（都只有若干条 OBS-3 工具行 + 一条正常收尾的 turn 记录）⇒ 我们只能事后翻 git 才知道它没干活。
//
// 本包把「这一轮调了几次工具、到目前为都没写过东西、是不是零工具调用就交了答案」变成**可判定的观测**：
//
//	tool_calls_this_turn        本轮工具调用数
//	turns_without_tool_call     累计**连续**零工具调用轮数（有工具调用即归零）
//	turns_since_last_write      距上一次「写类工具**成功**」的轮数（有写成功即归零）
//	turns_before_first_tool_call 首次工具调用之前的轮数；整会话零工具调用 ⇒ 配 first_tool_call_seen=false 显式标记
//	early_exit_suspected        会话级结论：**整会话零写类工具成功 且 已给出终答**
//
// 为什么是一个**叶子包**（零内部依赖，照抄 internal/toolobs 的依赖方向）：
//
//	事实产生在 internal/loopcore（轮次循环——只有它知道「本轮调了几次工具」「这次调用成没成」），
//	落盘在 internal/chat（obs.go → chat_obs.jsonl）。依赖方向是 chat → agent → loopcore 单向，
//	loopcore **不能**反向 import chat（成环）⇒ 这里只放「事实形状 + 计数器 + 投递口」，
//	由 chat 侧注册 sink 决定落到哪（不注册 = 不落，绝不 panic、绝不阻塞、绝不改变循环行为）。
//
// 三条纪律（与 chat/obs.go 的三条铁律同源，改这里先读它）：
//  1. **观测绝不改变行为**：本包只做纯累加与派生，**不参与任何判定/闸门/收尾决策**；
//     循环侧调用它是"上报事实"，不是"征询意见"。
//  2. **best-effort**：sink 里出任何岔子（panic/写盘失败）都不影响对话与工具执行路径。
//  3. **0 与「未知」必须可分**：轮级块字段**不带 omitempty**（0 是有效测量值：本轮零工具调用就是 0）；
//     会话级结论另用 tracked 表达「本包到底收到过轮次事实没有」，**没有事实就绝不下结论**（不编造）。
package behaviorobs

import (
	"log"
	"sync"
)

// writeTools — 「写类工具」**显式白名单**（闭集，新增写工具必须在此登记）。
//
// 口径（重要，决定了信号能说什么、不能说什么）：
//   - 进白名单 = 「这次调用**成功**就改变了工作区/产出物」的确定性判据（成败另由 execErr 决定）。
//   - bash **不在**白名单里：它既能读也能写（重定向/管道落盘），本层无法判别 ⇒ 宁可不算。
//     同理 media/编码类工具（ffmpeg_transcode、subtitle_generate…）也可能落盘但未登记。
//   - 因此 wrote_ever=false 是「写」的**下界**信号 ⇒ 结论只能叫「**怀疑**早退」（suspected），
//     不是定论：命中只说明"没有任何一次被我们认得的写"，仍要人/上层看证据（git diff、产物时间戳）。
var writeTools = map[string]bool{
	"write":        true, // 写文件
	"edit":         true, // 编辑/替换
	"apply_patch":  true, // 打补丁
	"copy_file":    true, // 复制（产出新文件）
	"move_file":    true, // 移动/重命名
	"delete_file":  true, // 删除
	"batch_rename": true, // 批量重命名
	"backup_file":  true, // 备份（产出新文件）
	"zip_create":   true, // 打包（产出新文件）
	"zip_extract":  true, // 解包（产出新文件）
}

// IsWriteTool — 工具名是否属于「写类工具」白名单（闭集；未知工具一律 false ⇒ 不猜）。
func IsWriteTool(name string) bool {
	return writeTools[name]
}

// Fact — **一轮结束**的事实（由轮次循环上报；本包只累加，不判断）。
type Fact struct {
	Session   string // 会话 ID（**空 = 无法归因** ⇒ 本包忽略，见 EmitRound）
	Round     int    // 轮次（1 起，由内核给）
	ToolCalls int    // 本轮工具调用数（含被拒/失败的调用——"想没想调"是模型侧事实）
	WriteOK   int    // 本轮**成功**的写类工具数（判定层放行 + 执行无错 + 名字在白名单里）
}

// Snapshot — 一轮结束后**派生**出的行为信号（写进 chat_obs.jsonl 的 behavior 块）。
// 全部字段都是"截至目前"的累计量：轮级块回答「现在到哪了」，会话级 Summary 才回答「算不算早退」。
type Snapshot struct {
	Session                  string
	Round                    int
	ToolCallsThisTurn        int
	TurnsWithoutToolCall     int
	TurnsSinceLastWrite      int
	TurnsBeforeFirstToolCall int
	FirstToolCallSeen        bool
	WroteEver                bool
	WriteCallsThisTurn       int
	TotalToolCalls           int
	TotalWriteOK             int
}

// SessionFact — **会话收尾**的事实（由轮次循环在 Run 返回处上报）。
type SessionFact struct {
	Session     string
	Rounds      int    // 跑过的轮数（内核口径；0=一轮都没跑完）
	ExitKind    string // 内核退出原因（natural/max_rounds/round_timeout/wall_clock/…）
	FinalAnswer bool   // 是否**已给出终答**（内核口径：正常收尾 natural 且有正文）
}

// Summary — 会话级结论（early_exit_suspected 只在这里出现；轮级块不预判会话结论）。
type Summary struct {
	Session                  string
	Rounds                   int
	ExitKind                 string
	FinalAnswer              bool
	TotalToolCalls           int
	TotalWriteOK             int
	TurnsWithoutToolCall     int
	TurnsSinceLastWrite      int
	FirstToolCallSeen        bool
	TurnsBeforeFirstToolCall int
	WroteEver                bool
	Tracked                  bool // 本包是否收到过该会话的轮次事实（false ⇒ 下列计数**不是测量值**，不下结论）
	EarlyExitSuspected       bool
}

// Sink — 落点（chat 侧在 init 里注册一次；某个字段为 nil = 那类不落）。
// 为什么分成两个回调而不是一个大接口：轮级是**高频**（每轮一条），会话级是**低频**（每次请求一条），
// 落点可能需要分别采样/分别落盘；nil 安全 = 只注册一半也能工作。
type Sink struct {
	Round   func(Snapshot)
	Session func(Summary)
}

// sessionState — 一个会话（= 一次 Run）的行为累加器（常数内存：定长字段，不随轮数膨胀）
type sessionState struct {
	toolCallsThisTurn        int
	writeOKThisTurn          int
	turnsWithoutToolCall     int
	turnsSinceLastWrite      int
	turnsBeforeFirstToolCall int
	firstToolCallSeen        bool
	wroteEver                bool
	totalToolCalls           int
	totalWriteOK             int
	rounds                   int
	seq                      int64 // LRU 序号（防长跑守护进程里表无界）
}

// behaviorStatesMax — 会话表容量上限。正常路径由 EmitSession 回收（会话收尾即删）；
// 这个上限只是**兜底**：万一某个会话从不收尾，也不能让表无界增长（表满淘汰最久未出现者）。
const behaviorStatesMax = 4096

var (
	behaviorMu     sync.Mutex
	behaviorSeq    int64
	behaviorStates = map[string]*sessionState{}
	behaviorSink   Sink
)

// SetSink — 注册落点（chat 侧在 init 里接一次）。并发安全；重复注册以后者为准。
func SetSink(s Sink) {
	behaviorMu.Lock()
	behaviorSink = s
	behaviorMu.Unlock()
}

// roundSink / sessionSink — 取当前落点（调用点在锁外使用；改动落点与读落点同锁，避免竞态）
func roundSink() func(Snapshot) {
	behaviorMu.Lock()
	defer behaviorMu.Unlock()
	return behaviorSink.Round
}

func sessionSink() func(Summary) {
	behaviorMu.Lock()
	defer behaviorMu.Unlock()
	return behaviorSink.Session
}

// stateOfLocked — 取（必要时建）会话累加器 + 更新 LRU 序号。调用方须持 behaviorMu。
func stateOfLocked(session string) *sessionState {
	st := behaviorStates[session]
	if st == nil {
		if len(behaviorStates) >= behaviorStatesMax {
			evictOldestLocked()
		}
		st = &sessionState{}
		behaviorStates[session] = st
	}
	behaviorSeq++
	st.seq = behaviorSeq
	return st
}

// evictOldestLocked — 表满时淘汰最久未出现的会话。调用方须持 behaviorMu。
func evictOldestLocked() {
	var oldestKey string
	var oldestSeq int64 = -1
	for k, v := range behaviorStates {
		if oldestSeq < 0 || v.seq < oldestSeq {
			oldestKey, oldestSeq = k, v.seq
		}
	}
	if oldestKey != "" {
		delete(behaviorStates, oldestKey)
	}
}

// EmitRound — 上报「一轮结束」的事实并派生出该轮的行为信号。
//
// 纪律：
//   - **无会话 ⇒ 忽略**（不做任何事）：行为信号按会话归因才有意义，空键会把所有会话混成一个计数器
//     —— 与 T1.1「有会话才补骨架」同源（拿不到就缺席，不编造）。
//   - 纯累加、**不返回任何东西**：调用方（轮次循环）拿不到反馈 ⇒ 结构上就不可能因此改变行为。
//   - sink 在锁**外**调用（sink 会写盘，不能占着包锁），且带 panic 兜底。
func EmitRound(f Fact) {
	if f.Session == "" {
		return
	}
	fn := roundSink()
	behaviorMu.Lock()
	st := stateOfLocked(f.Session)

	st.toolCallsThisTurn = f.ToolCalls
	st.writeOKThisTurn = f.WriteOK
	if f.Round > st.rounds {
		st.rounds = f.Round
	}
	st.totalToolCalls += f.ToolCalls
	st.totalWriteOK += f.WriteOK

	// 距上次写成功：本次有写成功 ⇒ 0；否则累计 +1（含"从来没有写过"的情形——
	// 此时 wroteEver=false 同时落盘，读侧据此把"至今没写"与"刚写完"分开，两者绝不混同）
	if f.WriteOK > 0 {
		st.wroteEver = true
		st.turnsSinceLastWrite = 0
	} else {
		st.turnsSinceLastWrite++
	}

	// 连续零工具调用轮数 + 首次工具调用之前的轮数
	if f.ToolCalls > 0 {
		st.turnsWithoutToolCall = 0
		if !st.firstToolCallSeen {
			st.firstToolCallSeen = true
			// 首次工具调用之前的轮数：轮次 1 起 ⇒ 首次出现在第 R 轮 = 之前有 R-1 轮没调工具
			st.turnsBeforeFirstToolCall = f.Round - 1
		}
	} else {
		st.turnsWithoutToolCall++
		if !st.firstToolCallSeen {
			// 还没出现过首次调用 ⇒ 记"至今 N 轮仍无任何工具调用"（**非空**，读侧配 first_tool_call_seen=false 即知
			// 这不是"首次之前的轮数"而是"到现在的轮数"）
			st.turnsBeforeFirstToolCall++
		}
	}

	snap := Snapshot{
		Session:                  f.Session,
		Round:                    f.Round,
		ToolCallsThisTurn:        st.toolCallsThisTurn,
		TurnsWithoutToolCall:     st.turnsWithoutToolCall,
		TurnsSinceLastWrite:      st.turnsSinceLastWrite,
		TurnsBeforeFirstToolCall: st.turnsBeforeFirstToolCall,
		FirstToolCallSeen:        st.firstToolCallSeen,
		WroteEver:                st.wroteEver,
		WriteCallsThisTurn:       st.writeOKThisTurn,
		TotalToolCalls:           st.totalToolCalls,
		TotalWriteOK:             st.totalWriteOK,
	}
	behaviorMu.Unlock()
	callRound(fn, snap)
}

// EmitSession — 上报「会话收尾」并给出会话级结论，随后**回收**该会话的计数器。
//
// early_exit_suspected 判据（写死在这里，改前先读）：
//
//	**整会话零「写类工具成功」（!WroteEver）且 已给出终答（FinalAnswer）** ⇒ 怀疑早退/空转。
//
// 为什么必须带"已给出终答"：没交答案就退出的不是早退，是崩溃/超时/轮数用尽（另有 exit_kind 分类）
// ——少了这半条判据，任何一次超时都会被误报成早退（防误报）。
// 为什么是"怀疑"不是"定论"：写类工具集是**显式白名单**（见 writeTools）⇒ 本信号是"写"的下界，
// 命中只说明"没有任何一次被我们认得的写"，落地前仍要看证据（git diff / 产物）。**不改变任何行为**。
//
// 为什么收尾即回收：本包的计数是"一次 Run（一次对话请求）"口径 —— 下一次请求重新计数（各自判定），
// 同时也让会话表天然有界（防守护进程内存无界）。
func EmitSession(s SessionFact) {
	if s.Session == "" {
		return
	}
	fn := sessionSink()
	behaviorMu.Lock()
	st := behaviorStates[s.Session]
	tracked := st != nil
	sum := Summary{
		Session:     s.Session,
		Rounds:      s.Rounds,
		ExitKind:    s.ExitKind,
		FinalAnswer: s.FinalAnswer,
		Tracked:     tracked,
	}
	if tracked {
		sum.TotalToolCalls = st.totalToolCalls
		sum.TotalWriteOK = st.totalWriteOK
		sum.TurnsWithoutToolCall = st.turnsWithoutToolCall
		sum.TurnsSinceLastWrite = st.turnsSinceLastWrite
		sum.FirstToolCallSeen = st.firstToolCallSeen
		sum.TurnsBeforeFirstToolCall = st.turnsBeforeFirstToolCall
		sum.WroteEver = st.wroteEver
		if sum.Rounds == 0 {
			sum.Rounds = st.rounds // 内核没报轮数 ⇒ 用本包的实测轮数（不是编造：这是收到的轮次事实条数）
		}
		// 没有轮次事实 ⇒ **不下结论**（tracked=false 同时在盘上，读侧可辨"没测到"与"测到没早退"）
		sum.EarlyExitSuspected = !st.wroteEver && s.FinalAnswer
		delete(behaviorStates, s.Session)
	}
	behaviorMu.Unlock()
	callSession(fn, sum)
}

// callRound / callSession — 投递（锁外；sink 是外部代码且会写盘 ⇒ 必须挡住它把异常带回循环）
func callRound(fn func(Snapshot), snap Snapshot) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ behaviorobs: round sink panic（循环与工具执行不受影响）: %v", r)
		}
	}()
	fn(snap)
}

func callSession(fn func(Summary), sum Summary) {
	if fn == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ behaviorobs: session sink panic（循环与工具执行不受影响）: %v", r)
		}
	}()
	fn(sum)
}
