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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ── 分类码（OBS-2）：任何"非正常收尾"都必须落到这三类之一 ──
const (
	ChatErrUpstreamTimeout = "upstream_timeout" // 上游模型超时（client.Timeout / RoundTimeout）
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
	QueuedMS int64 `json:"queued_ms"` // 排队时长（乙：排队≠推理，只进观测）
	GateSec  int   `json:"gate_sec"`  // 首 token 闸（秒）——本次生效值（含按卵放宽后的结果）
	WaitedMS int64 `json:"waited_ms"` // 首字节实际等待（与 GateSec 对照即知"差多少被掐"）
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
}

var obsMu sync.Mutex

// obsWrite — 追加一行 JSONL（best-effort：失败只记日志，绝不外抛、绝不 panic）
func obsWrite(rec ObsRecord) {
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
}

// NewObsTimer — 建一个轮次计时器
func NewObsTimer(session string, round int, model string) *ObsTimer {
	now := time.Now()
	return &ObsTimer{session: session, round: round, model: model, start: now, last: now}
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
	return "error"
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
