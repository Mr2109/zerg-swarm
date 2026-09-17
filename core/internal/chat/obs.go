// obs.go — 对话链路观测面（v2.5.10 前置档②：OBS-1/2/3/4 的公共底座）
//
// 设计稿：docs/01-设计/设计-v2.5.10-对话观测面-20260916.md（Mr2109 拍板：都按建议）
//
// 三条铁律（写死在这里，改观测面时先读它）：
//  1. **观测绝不改变对话行为语义** —— 本文件所有写盘都是 best-effort：写失败只记一条日志，
//     绝不向调用方返回错误、绝不阻塞请求、绝不 panic。
//  2. **判活只看证据，不看表** —— 因此这里只"记录事实"（首字节/最长停顿/总时长/结束原因），
//     不在任何地方对"是否卡死/超时"下判断（判断留给上层与看门狗）。
//  3. **错误必须有分类** —— 非正常收尾一律落到 ChatErrXxx 之一，不许无分类。
//
// 落点：`statepath.Dir()/chat_obs.jsonl`（解析器给出，禁写死私有路径）。
// 轮转：单文件 8MB × 保留 5 份（与既有轮转口径一致）。
//
// ⚠ 0 与"未知"必须可分（实测教训）：turn 的时序字段放在**嵌套对象**里且**内部不带 omitempty** ——
// 首字节 0ms 是**有效测量值**（瞬时），若用 omitempty 抹掉，长跑报告会把"瞬时"读成"没有数据"。
package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/infergeom"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/toolobs"
	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"
)

// ── 分类码（OBS-2）：任何"非正常收尾"都必须落到这三类之一 ──
const (
	ChatErrUpstreamTimeout = "upstream_timeout" // 上游模型超时（client.Timeout / RoundTimeout）
	ChatErrOther           = "other_error"      // 兜底分类（有名字，不再是无分类）
	ChatErrUpstreamFail    = "upstream_fail"    // 上游 5xx/upstream
	ChatErrStreamBroken    = "stream_broken"    // 流中断
	ChatErrBadRequest      = "bad_request"      // 请求不合法
	ChatErrClientAborted   = "client_aborted"   // 客户端主动中断（abort 端点 / 断连）
	ChatErrStreamTruncated = "stream_truncated" // 流被截断（未收到 [DONE]/finish_reason 即结束）
)

// ChatInferError — 带分类码的推理错误（OBS-2）。
// 判据：任何"非正常收尾"都必须是这三类之一；调用方按 Code 分类处置/上报。
type ChatInferError struct {
	Code string // ChatErrUpstreamTimeout / ChatErrClientAborted / ChatErrStreamTruncated
	Err  error
}

func (e *ChatInferError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("chat: %s", e.Code)
	}
	return fmt.Sprintf("chat: %s: %v", e.Code, e.Err)
}

// Unwrap — 让 errors.Is/As 能穿透到根因（便于上层按超时/取消分别处置）
func (e *ChatInferError) Unwrap() error { return e.Err }

// obsFileName — 观测文件（与 state 目录同源）
const (
	obsFileName  = "chat_obs.jsonl"
	obsMaxBytes  = 8 * 1024 * 1024 // 单文件 8MB
	obsMaxBackup = 5               // 保留 5 份
)

// TurnObs — turn 记录的时序子对象（OBS-1）。
// ⚠ 内部字段**一律不带 omitempty**：0 是有效测量值（瞬时），不得与"未知"混同。
type TurnObs struct {
	FirstByteMS int64 `json:"first_byte_ms"` // 首字节耗时
	MaxGapMS    int64 `json:"max_gap_ms"`    // 最长空档（**含**首字节前的等待）
	StallMS     int64 `json:"stall_ms"`      // 首字节**之后**的最大空档（真正区分"慢"与"卡"）
	TotalMS     int64 `json:"total_ms"`      // 总时长
	Chunks      int   `json:"chunks"`        // 分块数
	// 丁（超时留痕，2026-09-17 Mr2109 拍「都做」）：把"闸多少、等了多久、排队多久"记下来，
	// 让"超时"这件事不必翻日志就能在观测面看清；乙（排队分离）：排队时长**只观测、不进任何闸**。
	QueuedMS int64  `json:"queued_ms"`          // 排队时长（乙：排队≠推理，只进观测）
	ErrText  string `json:"err_text,omitempty"` // 非正常收尾的错误原文（治"无名氏"）
	GateSec  int    `json:"gate_sec"`           // 首 token 闸（秒）——本次生效值（含按卵放宽后的结果）
	WaitedMS int64  `json:"waited_ms"`          // 首字节实际等待（与 GateSec 对照即知"差多少被掐"）
	Verdict  string `json:"verdict"`            // 丙：卡 / 慢 / 正常（看门狗结论，一眼可读）
}

// GeometryObs — T1.4：**大模型调用**（kind=turn）的批量几何（llama.cpp batch geometry）。
//
// 为什么必须记（docs/调研/实验-批量几何与可复现性-T7.1-T7.2-20260917.md 实测坐实）：
// cache_n（前缀复用 KV）/ prompt_n（本次**实际评估**）**确实改变 logits**——dense 模型在
// 固定几何下逐比特可复现（0 ulp），MoE 跨几何差 2.4e-02；串行独占 5/5 相同，而同批有他人
// 25/25 全不同（差异率 ≥94.7%）。没有这两个字段，回放/回归的差异**无法归因**。
//
// 口径（与文件头三条铁律同源，改这里先读它）：
//  1. **上游不给 ⇒ 字段缺席**：计量一律用**指针**表达存在性——`cache_n:0`（冷缓存，冷启动命不中
//     是有效测量值）与「上游根本没给 cache_n」必须可分；读侧**不许**把缺席读成 0。
//  2. `geometry_recorded` 是**观测面自己的结论**（我们问了、有没有记到什么），不是上游字段：
//     每条 turn 记录都带它——false = 问过了但上游什么都没给（不是"没问"，更不是"没有这回事"）。
//  3. 只挂在**大模型调用**那类记录上；工具/压缩事件不带（拿不到就是拿不到，不硬凑）。
type GeometryObs struct {
	CacheN            *int   `json:"cache_n,omitempty"`            // llama.cpp timings.cache_n（前缀命中，复用 KV）
	PromptN           *int   `json:"prompt_n,omitempty"`           // llama.cpp timings.prompt_n（本次实际评估）
	CachedTokens      *int   `json:"cached_tokens,omitempty"`      // usage.prompt_tokens_details.cached_tokens
	UbatchN           *int   `json:"ubatch_n,omitempty"`           // ubatch 划分规模（上游通常不给 ⇒ 缺席）
	SlotID            *int   `json:"slot_id,omitempty"`            // 命中的槽位（-np 多槽几何要素）
	SystemFingerprint string `json:"system_fingerprint,omitempty"` // 引擎白送（实测形如 b10470-34af94cd9）
	Form              string `json:"form,omitempty"`               // 计量形态（openai/llamacpp/…，便于归因）
	Recorded          bool   `json:"geometry_recorded"`            // 观测面结论：本轮几何记到没有（每条 turn 恒有）
}

// ObsRecord — 一条观测记录（定长字段集：不随轮数膨胀）
type ObsRecord struct {
	TS        string `json:"ts"`                   // 记录时间（RFC3339）
	Kind      string `json:"kind"`                 // turn | compact | tool
	Session   string `json:"session,omitempty"`    // 会话 ID
	Round     int    `json:"round,omitempty"`      // 轮次
	Model     string `json:"model,omitempty"`      // 模型名
	EndReason string `json:"end_reason,omitempty"` // 正常收尾=finish；否则为分类码

	// OBS-1：只在 turn 记录里出现（嵌套 ⇒ 别的种类不带这些字段，也不丢 0）
	Turn *TurnObs `json:"turn,omitempty"`

	// 丁：超时/回落留痕（turn 记录里出现）——不再靠翻日志才知道"回落给谁、为什么"
	FailoverTo     string `json:"failover_to,omitempty"`
	FailoverReason string `json:"failover_reason,omitempty"`

	// OBS-4 压缩专用（本路径不可得的字段留空 ⇒ omitempty = 未知，不编造）
	Cause        string `json:"cause,omitempty"`
	TokensIn     int    `json:"tokens_in,omitempty"`
	TokensOut    int    `json:"tokens_out,omitempty"`
	SummaryChars int    `json:"summary_chars,omitempty"`
	DurMS        int64  `json:"dur_ms,omitempty"` // 压缩耗时（turn 的耗时在 turn.total_ms）
	Result       string `json:"result,omitempty"`
	FailReason   string `json:"fail_reason,omitempty"`
	CounterReset bool   `json:"counter_reset,omitempty"`

	// OBS-3 工具专用
	Tool       string `json:"tool,omitempty"`
	DurText    string `json:"dur,omitempty"` // 工具耗时的原始文本（ToolTrace.Duration 是字符串）
	ToolRounds int    `json:"tool_rounds,omitempty"`
	ToolMax    int    `json:"tool_max,omitempty"`

	// ── T1.2 工具调用判定（允许/拒绝 + 拒绝原因）──
	// 语义（写死，勿混）：decision 是**判定层**结论（安全门/白名单/参数约定/危险命令…），
	// 不是执行结果——执行成败仍在 result（OBS-3）与工具轨迹 Trace.Error 上。
	Decision   string `json:"decision,omitempty"`    // allow | deny
	DenyReason string `json:"deny_reason,omitempty"` // 拒绝原因码（低基数、可枚举——见 internal/toolobs）
	ArgsDigest string `json:"args_digest,omitempty"` // 参数摘要（sha256 前 16 位；**不落参数原文**）

	// ── T1.1 追踪骨架（九字段；任务表 v2.5.10 T1.1 / 设计稿 v1.2 B1）──
	// 目的：让每条记录可拼成父子结构（trace → 本轮 span → parent），从而「父子 span 分层」在后端**可校验**，
	// 而不是只靠自称。全部是**附加键**：既有字段一律不改名、不改语义；读侧宽容 ⇒ 旧行照样能解析。
	// 口径：① 有会话才补骨架（无会话 ⇒ 这几个键**缺席**，绝不编造）② 调用方已给的不覆盖
	//      ③ **event_name 非空即事件**（缺席者为陪衬/属性载体，不是事件）。
	TraceID      string `json:"trace_id,omitempty"`           // 链路 ID（同一会话内不变）
	SpanID       string `json:"span_id,omitempty"`            // 本记录自身的 span（轮次 ⇒ 每轮新生成）
	ParentSpanID string `json:"parent_span_id,omitempty"`     // 父 span（轮次 ⇒ 上一轮；首轮 ⇒ 会话根）
	SpanKind     string `json:"span_kind,omitempty"`          // internal | client | server
	StartedAtMS  int64  `json:"started_at_unix_ms,omitempty"` // span 起点（Unix 毫秒；0=未知 ⇒ 缺席）
	EndedAtMS    int64  `json:"ended_at_unix_ms,omitempty"`   // span 终点（Unix 毫秒；0=未知 ⇒ 缺席）
	// recorded/sampled 用**指针**表达三态：nil=未定（缺席）、显式 false 也照样落盘。
	// 为什么不用裸 bool：裸 bool 分不出「明确的 false」与「没给」，而这两者在本进程里有实际区别
	// （recorded=false 表示"知道这件事但没记"，sampled=false 表示"知道但没采"）。
	Recorded  *bool  `json:"recorded,omitempty"`   // 是否已记录（本进程全量落盘 ⇒ 默认 true）
	Sampled   *bool  `json:"sampled,omitempty"`    // 是否落在采样集合内（首版不采样 ⇒ 默认 true）
	EventName string `json:"event_name,omitempty"` // 事件名（非空 ⇒ 这条是事件）；默认取 kind（kind 已是既有事实）

	// ── T1.4 批量几何（llama.cpp batch geometry）──
	// 挂在**大模型调用**那一类记录（kind=turn）上：见 GeometryObs 的口径（上游不给 ⇒ 字段缺席）。
	// 它与 turn 平级（不是 TurnObs 的成员）：TurnObs 的字段**不带 omitempty**（0 是有效测量值），
	// 而几何字段必须能表达「缺席」⇒ 一律指针 + omitempty，两种语义不混。
	Geometry *GeometryObs `json:"geometry,omitempty"`

	// ── T1.3 早退/空转行为信号（kind=behavior）──
	// 轮级块（event_name=behavior_round）挂 Behavior；会话级结论（event_name=behavior_session）挂
	// BehaviorSession —— 两者**互斥**：轮级块是"截至目前"，会话级结论是"算不算早退"，语义不同不混一个块。
	// 同 TurnObs 的口径：**块内字段不带 omitempty**（0 是有效测量值，不得与"未知"混同）。
	Behavior        *BehaviorObs        `json:"behavior,omitempty"`
	BehaviorSession *BehaviorSessionObs `json:"behavior_session,omitempty"`

	// ── T3.1 compaction 三态事件（kind=compact；event_name=compaction_started/completed/failed）──
	// 为什么独立成块（不改旧 OBS-4 字段）：三态共用一个事实块，字段口径只写一处（obs_compaction.go）；
	// 块内「终态才知道的量」一律指针 + omitempty ⇒ 缺席 = 未知，绝不写 0 顶替（见该文件口径 ③）。
	// 旧 OBS-4 行（Result/SummaryChars/FailReason/Cause）原样保留，新终态行也带这些字段（兼容老读侧）。
	Compaction *CompactionObs `json:"compaction,omitempty"`
}

var obsMu sync.Mutex

// ── T1.1 追踪骨架：id 生成、会话→trace 表、父子链 ──
//
// 三条口径（与文件头三条铁律同源，改这里先读它）：
//  1. **有会话才有骨架**：拿不到 session ⇒ 这几个键缺席（不是编 0、不是编假 id）；读侧应把「缺席」读成"未知"。
//  2. **调用方给了就不覆盖**：观测面只填空缺，不改任何既有字段的语义（既有用例必须继续绿）。
//  3. **span 链只由轮次推进**：turn 每轮开一个新 span（parent=上一轮，首轮=会话根），
//     tool/compact 这类事件挂在**当前轮**之下，不推进「上一轮」指针 —— 否则工具事件会把父子链截断。
const (
	// span_kind 取值（仿 OTel：server=处理外部请求的 span、client=向外发出的调用、internal=进程内部步骤）
	ObsSpanKindInternal = "internal"
	ObsSpanKindClient   = "client"
	ObsSpanKindServer   = "server"

	// obsTraceSessionsMax — 会话→trace 表的容量上限。长跑守护进程里会话数无界，表必须有界：
	// 表满时淘汰**最久未出现**的会话，它此后的新事件会开一条新 trace（观测可容忍；内存不可无界）。
	obsTraceSessionsMax = 4096
)

// obsSessionTrace — 一个会话的追踪骨架（trace 不变、span 每轮新开、parent 指上一轮或会话根）
type obsSessionTrace struct {
	traceID    string
	rootSpanID string // 会话根 span（首轮的 parent 指向它）
	lastSpanID string // 最近一轮的 span（下一轮 parent 的来源）
	seq        int64  // LRU 序号（淘汰最久未出现者）
}

var (
	obsTraceMu  sync.Mutex
	obsTraceSeq int64
	obsTraceTab = map[string]*obsSessionTrace{}
)

// obsNewID — 16 字节 crypto/rand 的十六进制（32 字符）。
// 拿不到随机源 ⇒ 返回空串（骨架**缺席**，不编造一个看着像 id 的值）。
func obsNewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

// obsSessionOfLocked — 取（必要时建）会话骨架 + 更新 LRU 序号。调用方须持 obsTraceMu。
func obsSessionOfLocked(session string) *obsSessionTrace {
	st := obsTraceTab[session]
	if st == nil {
		if len(obsTraceTab) >= obsTraceSessionsMax {
			obsTraceEvictOldestLocked()
		}
		st = &obsSessionTrace{traceID: obsNewID(), rootSpanID: obsNewID()}
		// T1.6 传播：本会话入站时沿用了上游 trace（网关侧 tracectx.BindSession）⇒ 轮次事件必须
		// 落在**同一条** trace 上（这正是"写进本轮事件"）。只在**会话骨架首次创建**时采纳：
		// 此后本会话 trace_id 恒定 —— T1.1 的不变式"同一会话 trace_id 不变"不破。
		if tc, ok := tracectx.SessionTrace(session); ok {
			st.traceID = tc.TraceID
		}
		// 反向也钉一次：把本条 trace 登记成**出站可用**的线上上下文（只对齐 trace-id，线上 span 另开）
		// —— 这样"对话客户端 → 网关 → 子端"每一跳的 traceparent 都是本轮事件这一条 trace，
		// 一次调用从轮次事件到子端全部对得上（否则客户端会另生成一条，两侧永远对不上账）。
		if st.traceID != "" {
			tracectx.BindSessionTraceID(session, st.traceID)
		}
		obsTraceTab[session] = st
	}
	obsTraceSeq++
	st.seq = obsTraceSeq
	return st
}

// obsTraceEvictOldestLocked — 表满时淘汰最久未出现的会话。调用方须持 obsTraceMu。
func obsTraceEvictOldestLocked() {
	var oldestKey string
	var oldestSeq int64 = -1
	for k, v := range obsTraceTab {
		if oldestSeq < 0 || v.seq < oldestSeq {
			oldestKey, oldestSeq = k, v.seq
		}
	}
	if oldestKey != "" {
		delete(obsTraceTab, oldestKey)
	}
}

// obsNewTurnSpan — T1.1：为**一轮**开新 span（trace 同会话不变；parent=上一轮，首轮=会话根），
// 并把本轮 span 记为「上一轮」供下一轮指回来。无会话 ⇒ 全空（骨架缺席）。
func obsNewTurnSpan(session string) (traceID, spanID, parentSpanID string) {
	if session == "" {
		return "", "", ""
	}
	obsTraceMu.Lock()
	defer obsTraceMu.Unlock()
	st := obsSessionOfLocked(session)
	span := obsNewID()
	parent := st.lastSpanID
	if parent == "" {
		parent = st.rootSpanID
	}
	st.lastSpanID = span
	return st.traceID, span, parent
}

// obsCurrentSpan — T1.1：非轮次事件（tool/compact 等）的挂点 = **当前轮**（无轮次 ⇒ 会话根）。
// 它**不**推进「上一轮」指针：父子链只由轮次推进。
func obsCurrentSpan(session string) (traceID, parentSpanID string) {
	if session == "" {
		return "", ""
	}
	obsTraceMu.Lock()
	defer obsTraceMu.Unlock()
	st := obsSessionOfLocked(session)
	if st.lastSpanID != "" {
		return st.traceID, st.lastSpanID
	}
	return st.traceID, st.rootSpanID
}

// obsTruePtr — 三态布尔用的"明确 true"
func obsTruePtr() *bool { b := true; return &b }

// fillTraceSkeleton — T1.1：把九字段里**空缺**的补上（调用方已给的一律不覆盖），best-effort、不 panic。
// 它是 obsWrite 的第一步 ⇒ 每条落盘记录都带骨架；无会话 ⇒ 整块缺席。
func (r *ObsRecord) fillTraceSkeleton() {
	if r.Session == "" {
		return // 无会话 ⇒ 骨架缺席（不编造）
	}
	traceID, parent := obsCurrentSpan(r.Session)
	if r.TraceID == "" {
		r.TraceID = traceID
	}
	if r.ParentSpanID == "" {
		r.ParentSpanID = parent
	}
	if r.SpanID == "" {
		r.SpanID = obsNewID() // 轮次 span 已在 NewObsTimer 开好 ⇒ 这里只给非轮次事件补
	}
	if r.SpanKind == "" {
		r.SpanKind = ObsSpanKindInternal
	}
	now := time.Now().UnixMilli()
	if r.StartedAtMS == 0 {
		r.StartedAtMS = now // 单点事件：起止同一时刻（时长另有 dur_ms / turn.total_ms）
	}
	if r.EndedAtMS == 0 {
		r.EndedAtMS = now
	}
	if r.Recorded == nil {
		r.Recorded = obsTruePtr() // 本进程全量落盘 ⇒ "已记录"是事实，不是编造
	}
	if r.Sampled == nil {
		r.Sampled = obsTruePtr() // 首版不采样 ⇒ 每条都在采样集合内
	}
	if r.EventName == "" {
		r.EventName = r.Kind // kind 已是既有事实 ⇒ 拿它当事件名不是编造（非空 event_name 即事件）
	}
}

// obsWrite — 追加一行 JSONL（best-effort：失败只记日志，绝不外抛、绝不 panic）
func obsWrite(rec ObsRecord) {
	rec.fillTraceSkeleton() // T1.1：唯一写入口补骨架 ⇒ 每条记录都带 trace/span/parent（有会话时）
	rec.TS = time.Now().Format(time.RFC3339)
	b, err := json.Marshal(rec)
	if err != nil {
		log.Printf("⚠️ chat_obs: marshal failed: %v", err)
		return
	}
	dir := statepath.Dir()
	if dir == "" {
		log.Printf("⚠️ chat_obs: state dir unavailable")
		return
	}
	path := filepath.Join(dir, obsFileName)

	obsMu.Lock()
	defer obsMu.Unlock()

	// 轮转（超限则滚动；失败不影响写入尝试）
	if fi, err := os.Stat(path); err == nil && fi.Size() >= obsMaxBytes {
		obsRotate(path)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("⚠️ chat_obs: open failed (对话不受影响): %v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		log.Printf("⚠️ chat_obs: write failed (对话不受影响): %v", err)
	}
}

// obsRotate — chat_obs.jsonl → .1 … .5（最旧丢弃）
func obsRotate(path string) {
	for i := obsMaxBackup - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", path, i)
		newer := fmt.Sprintf("%s.%d", path, i+1)
		if i == obsMaxBackup-1 {
			_ = os.Remove(newer)
		}
		_ = os.Rename(old, newer)
	}
	_ = os.Rename(path, path+".1")
}

// ObsTimer — OBS-1：轮次时序采集器（挂在流式回调外层，最小侵入）。
//
// 用法：t := NewObsTimer(session, round, model)
//
//	t.MarkChunk()   // 每收到一个分块调用（onDelta 里包一层）
//	t.Finish(end)   // 轮次结束时调用（end=finish 或分类码）⇒ 落一行
type ObsTimer struct {
	session   string
	round     int
	model     string
	start     time.Time
	firstByte time.Time
	last      time.Time
	chunks    int
	maxGap    time.Duration
	stall     time.Duration // 首字节之后的最大空档
	gateSec   int           // 首 token 闸有效值（秒；0=未知）
	queued    time.Duration // 排队时长（乙：只观测）
	errText   string        // 非正常收尾的错误原文（best-effort）

	// T1.1 追踪骨架：本轮 span 在 NewObsTimer 时就开好（这样 started_at 才是真正的轮次起点），Finish 原样落盘
	traceID      string
	spanID       string
	parentSpanID string

	// T1.4 批量几何：本轮大模型**响应**里抄下来的几何（nil = 上游不给/未取到 ⇒ 记录里字段缺席）
	geometry *infergeom.Geometry
}

// NewObsTimer — 建一个轮次计时器（T1.1：同会话 trace 不变、每轮新开 span、parent 指上一轮/会话根）
func NewObsTimer(session string, round int, model string) *ObsTimer {
	now := time.Now()
	t := &ObsTimer{session: session, round: round, model: model, start: now, last: now}
	t.traceID, t.spanID, t.parentSpanID = obsNewTurnSpan(session)
	return t
}

// MarkChunk — 记录一个分块的到达（常数内存：只保留"最大间隔"）
func (t *ObsTimer) MarkChunk() {
	if t == nil {
		return
	}
	now := time.Now()
	isFirst := t.firstByte.IsZero()
	if isFirst {
		t.firstByte = now
	}
	if d := now.Sub(t.last); d > t.maxGap {
		t.maxGap = d
	}
	// stall 只统计"已经出字之后"的空档：首块之前的等待属于 TTFT（first_byte_ms），两者不可混
	if !isFirst {
		if d := now.Sub(t.last); d > t.stall {
			t.stall = d
		}
	}
	t.last = now
	t.chunks++
}

// Finish — 轮次结束：落一行观测（endReason 为空按"正常收尾"处理）
// stalledAfterMS —— 判「中途卡住」的阈值：首字节之后连续这么久没有新块 ⇒ 判「卡」。
// 取 60s：本地思考型模型出字本就慢，阈值给小了会把「慢」误判成「卡」（业界口径：看门狗而非总闸）。
const stalledAfterMS int64 = 60000

// SetGate —— 记录本轮**首 token 闸的生效值**（秒）⇒ 观测面能算出「差多少被掐」。0=未知（不编造）。
func (t *ObsTimer) SetGate(sec int) {
	if t != nil {
		t.gateSec = sec
	}
}

// SetQueued —— 记录本轮**排队时长**（乙：只观测、不进闸；0=未测到）。
func (t *ObsTimer) SetQueued(d time.Duration) {
	if t != nil {
		t.queued = d
	}
}

// SetErrText — 记录导致本轮非正常收尾的**错误原文**（best-effort；拿不到就不写 ⇒ 不编造）。
func (t *ObsTimer) SetErrText(e string) {
	if t != nil {
		t.errText = e
	}
}

// SetGeometry — T1.4：记录本轮大模型调用的**批量几何**（来自上游响应；nil = 上游不给/未取到 ⇒ 字段缺席）。
// 纪律：只存指针、不做任何判断、不改对话语义（观测面铁律①）——拿不到就缺席，绝不写 0 顶替。
func (t *ObsTimer) SetGeometry(g *infergeom.Geometry) {
	if t != nil {
		t.geometry = g
	}
}

// geometryObsOf — infergeom.Geometry → 观测形态。
// 口径：**上游没给的字段一个都不写**（指针 nil ⇒ 键缺席）；只有 system_fingerprint 是非空才写。
// 返回 nil 只在 g == nil 时（调用方对 turn 记录一律挂 GeometryObs：Recorded 是"我们问过"的结论）。
func geometryObsOf(g *infergeom.Geometry) *GeometryObs {
	out := &GeometryObs{}
	if g == nil {
		return out // 一项几何都没拿到 ⇒ 只有 geometry_recorded:false
	}
	out.Form = g.Form
	if g.HasCacheN {
		v := g.CacheN
		out.CacheN = &v
	}
	if g.HasPromptN {
		v := g.PromptN
		out.PromptN = &v
	}
	if g.HasCachedTokens {
		v := g.CachedTokens
		out.CachedTokens = &v
	}
	if g.HasUbatchN {
		v := g.UbatchN
		out.UbatchN = &v
	}
	if g.HasSlotID {
		v := g.SlotID
		out.SlotID = &v
	}
	out.SystemFingerprint = g.SystemFingerprint // 空 = 上游不给 ⇒ omitempty 抹掉（不写空串）
	out.Recorded = g.Any()
	return out
}

func (t *ObsTimer) Finish(endReason string) {
	if t == nil {
		return
	}
	if endReason == "" {
		endReason = "finish"
	}
	rec := ObsRecord{
		Kind:      "turn",
		Session:   t.session,
		Round:     t.round,
		Model:     t.model,
		EndReason: endReason,
		// T1.1 追踪骨架：span 在 NewObsTimer 开好（本轮 span），started_at = 轮次起点（不是落盘时刻）；
		// span_kind=server —— 本轮 span 处理的是客户端发来的请求（tool/compact 事件为 internal）。
		TraceID:      t.traceID,
		SpanID:       t.spanID,
		ParentSpanID: t.parentSpanID,
		SpanKind:     ObsSpanKindServer,
		StartedAtMS:  t.start.UnixMilli(),
		// T1.4 批量几何：上游响应里抄下来的几何（cache_n/prompt_n/cached_tokens/system_fingerprint…）；
		// nil/一项都没有 ⇒ geometry_recorded:false 且各计量字段**缺席**（不写 0 顶替）。
		Geometry: geometryObsOf(t.geometry),
		Turn: &TurnObs{
			TotalMS:  time.Since(t.start).Milliseconds(),
			MaxGapMS: t.maxGap.Milliseconds(),
			StallMS:  t.stall.Milliseconds(),
			Chunks:   t.chunks,
		},
	}
	if !t.firstByte.IsZero() {
		rec.Turn.FirstByteMS = t.firstByte.Sub(t.start).Milliseconds()
	}
	// 丙：把看门狗结论落进记录（判「卡」必须同时看首字节时长——本引擎突发式送达，stall 常为 0）
	gate := t.gateSec
	if gate == 0 {
		gate = EffectiveGateSec(t.model) // 未回填 ⇒ 按卵推导（档案事实优先 ⇒ 思考型下限 ⇒ 0=未知）
	}
	rec.Turn.GateSec = gate
	rec.Turn.WaitedMS = rec.Turn.FirstByteMS
	if t.errText != "" {
		rec.Turn.ErrText = trunca(t.errText, 300) // 截断：不撑爆观测
	}
	rec.Turn.Verdict = string(StallVerdictOf(rec.Turn.FirstByteMS, rec.Turn.StallMS, gate, stalledAfterMS))
	if t.queued > 0 {
		rec.Turn.QueuedMS = t.queued.Milliseconds() // 乙：排队只观测，不进任何闸
	}
	obsWrite(rec)
}

// ObsEndReason — OBS-1：把轮次结束原因归一为"分类码或 finish"。
// 判据（与 OBS-2 同源）：非正常收尾必须落到三类之一，不许无分类。
func ObsEndReason(err error) string {
	if err == nil {
		return "finish"
	}
	var cie *ChatInferError
	if errors.As(err, &cie) {
		return cie.Code
	}
	if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
		return ChatErrUpstreamTimeout
	}
	if errors.Is(err, context.Canceled) {
		return ChatErrClientAborted
	}
	// 2026-09-17 修：原来这里 `return "error"` ⇒ 观测里出现「非正常收尾却无分类：error」✗（实测 8 条）。
	// 原则：**凡非正常收尾必须落到一个有名字的分类** ⇒ 兜底也给名字（other_error），且原文另行记录（不丢信息）。
	txt := strings.ToLower(err.Error())
	switch {
	case strings.Contains(txt, "502") || strings.Contains(txt, "500") || strings.Contains(txt, "upstream"):
		return ChatErrUpstreamFail
	case strings.Contains(txt, "stream") || strings.Contains(txt, "unexpected eof"):
		return ChatErrStreamBroken
	case strings.Contains(txt, "400") || strings.Contains(txt, "invalid") || strings.Contains(txt, "bad request"):
		return ChatErrBadRequest
	default:
		return ChatErrOther
	}
}

// obsCompact — OBS-4：压缩事件一行（成功与失败都调）。
// counter_reset 是"被压垮"判据的核心字段；**确证不了就传 false 且不落该字段**（不编造）。
func obsCompact(session string, cause string, in, out, summaryChars int, d time.Duration, result, failReason string, counterReset bool) {
	obsWrite(ObsRecord{
		Kind: "compact", Session: session, Cause: cause,
		TokensIn: in, TokensOut: out, SummaryChars: summaryChars,
		DurMS: d.Milliseconds(), Result: result, FailReason: failReason,
		CounterReset: counterReset,
	})
}

// ObsTool — OBS-3：工具轮次一行（durText 用 ToolTrace 原生的耗时文本）
func ObsTool(session string, round int, tool string, durText string, ok bool, rounds, max int) {
	res := "ok"
	if !ok {
		res = "err"
	}
	obsWrite(ObsRecord{
		Kind: "tool", Session: session, Round: round, Tool: tool,
		DurText: durText, Result: res, ToolRounds: rounds, ToolMax: max,
	})
}

// ── T1.2 工具调用判定（允许/拒绝 + 原因）：事件名与落盘入口 ──

// obsEventToolDecision — T1.2 事件名（非空 event_name ⇒ 这条是事件；与 OBS-3 的 kind=tool 执行行**并存**）。
const obsEventToolDecision = "tool_decision"

// ObsToolDecision — T1.2：工具调用的「允许/拒绝」一行。
//
// 要治的盲区：一个调用**被系统拒绝**（权限/白名单/参数不合法/危险命令/被隐藏…）与**模型根本没想调**，
// 在原观测里同形（都只是"没有那条工具事件"）⇒ 无法归因。有了本事件，三态可分：
//
//	· 有 event_name=tool_decision 且 decision=allow ⇒ 判定层放行（工具可能随后因自身原因失败，那是执行层的事）
//	· 有 event_name=tool_decision 且 decision=deny  ⇒ 判定层拒绝，工具**未执行**，deny_reason 给出可枚举原因
//	· **完全没有这条事件**                          ⇒ 模型没发起这个调用（不是"被拒了但没记"）
//
// 隐私：只落参数**摘要**（sha256 前 16 位），不落参数原文（路径/命令/正文可能含隐私）。
// 纪律：与 obsWrite 同源——best-effort，写失败只记日志，绝不外抛/阻塞/panic。
func ObsToolDecision(session, tool, decision, reason, argsDigest string) {
	obsWrite(ObsRecord{
		Kind: "tool", Session: session, Tool: tool,
		Decision: decision, DenyReason: reason, ArgsDigest: argsDigest,
		EventName: obsEventToolDecision,
	})
}

// obsToolDecisionWire — T1.2 唯一接线点：把工具执行/轮次循环侧的判定（internal/toolobs 上报）接到观测面。
//
// 为什么走回调而不是让 agent 直接写盘：依赖方向是 chat → agent → loopcore 单向，agent 侧**不能** import chat
// （成环）⇒ agent 只上报到无依赖的叶子包 toolobs，由这里（chat）决定落到 chat_obs.jsonl。
// 未接 sink 时事件丢弃（不是错误）；本函数在进程内只注册一次。
func init() {
	toolobs.SetSink(func(d toolobs.Decision) {
		ObsToolDecision(d.Session, d.Tool, d.Decision, d.Reason, d.ArgsDigest)
	})
}

// trunca — 截断（观测用；不动原字符串语义，只在末尾标注省略）
func trunca(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "…（已截断）"
}
