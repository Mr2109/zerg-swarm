// obs.go — 对话链路观测面（v2.5.10 前置档②：OBS-1/2/4 的公共底座）
//
// 设计稿：docs/01-设计/设计-v2.5.10-对话观测面-20260916.md（Mr2109 拍板：都按建议）
//
// 三条铁律（写死在这里，改观测面时先读它）：
//  1. **观测绝不改变对话行为语义** —— 本文件所有写盘都是 best-effort：写失败只记一条日志，
//     绝不向调用方返回错误、绝不阻塞请求、绝不 panic。
//  2. **判活只看证据，不看表** —— 因此这里只"记录事实"（首字节/最长停顿/总时长/结束原因），
//     不在任何地方对"是否卡死/超时"下判断（判断留给上层与看门狗）。
//  3. **错误必须有分类** —— 非正常收尾一律落到 ChatErrCodeXxx 之一，不许无分类。
//
// 落点：`statepath.Dir()/chat_obs.jsonl`（解析器给出，禁写死私有路径）。
// 轮转：单文件 8MB × 保留 5 份（与既有轮转口径一致）。
package chat

import (
	"encoding/json"
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

// obsFileName — 观测文件（与 state 目录同源）
const (
	obsFileName  = "chat_obs.jsonl"
	obsMaxBytes  = 8 * 1024 * 1024 // 单文件 8MB
	obsMaxBackup = 5               // 保留 5 份
)

// ObsRecord — 一轮的观测记录（定长字段集：不随轮数膨胀）
type ObsRecord struct {
	TS          string `json:"ts"`                      // 记录时间（RFC3339）
	Kind        string `json:"kind"`                    // turn | compact | tool
	Session     string `json:"session,omitempty"`       // 会话 ID
	Round       int    `json:"round,omitempty"`         // 轮次
	Model       string `json:"model,omitempty"`         // 模型名
	FirstByteMS int64  `json:"first_byte_ms,omitempty"` // 首字节耗时（OBS-1）
	MaxGapMS    int64  `json:"max_gap_ms,omitempty"`    // 最长停顿（OBS-1：卡死感的客观来源）
	TotalMS     int64  `json:"total_ms,omitempty"`      // 总时长
	Chunks      int    `json:"chunks,omitempty"`        // 分块数
	EndReason   string `json:"end_reason,omitempty"`    // 正常收尾=finish；否则为分类码

	// 压缩专用（OBS-4）
	Cause        string `json:"cause,omitempty"`
	TokensIn     int    `json:"tokens_in,omitempty"`
	TokensOut    int    `json:"tokens_out,omitempty"`
	SummaryChars int    `json:"summary_chars,omitempty"`
	Result       string `json:"result,omitempty"`
	FailReason   string `json:"fail_reason,omitempty"`
	CounterReset bool   `json:"counter_reset,omitempty"`

	// 工具专用（OBS-3）
	Tool       string `json:"tool,omitempty"`
	ToolRounds int    `json:"tool_rounds,omitempty"`
	ToolMax    int    `json:"tool_max,omitempty"`
}

var obsMu sync.Mutex

// obsWrite — 追加一行 JSONL（best-effort：失败只记日志，绝不外抛）
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

// ── OBS-1：轮次时序采集器（挂在流式回调外层，最小侵入）──
//
// 用法：rec := newObsTimer(session, round, model)
//
//	rec.markChunk()   // 每收到一个分块调用（onDelta 里包一层）
//	rec.finish(end)   // 轮次结束时调用（end=finish 或分类码）⇒ 落一行
type obsTimer struct {
	session   string
	round     int
	model     string
	start     time.Time
	firstByte time.Time
	last      time.Time
	chunks    int
	maxGap    time.Duration
}

func newObsTimer(session string, round int, model string) *obsTimer {
	now := time.Now()
	return &obsTimer{session: session, round: round, model: model, start: now, last: now}
}

// markChunk — 记录一个分块的到达（常数内存：只保留"最大间隔"）
func (t *obsTimer) markChunk() {
	now := time.Now()
	if t.firstByte.IsZero() {
		t.firstByte = now
	}
	if d := now.Sub(t.last); d > t.maxGap {
		t.maxGap = d
	}
	t.last = now
	t.chunks++
}

// finish — 轮次结束：落一行观测（endReason 为空按"正常收尾"处理）
func (t *obsTimer) finish(endReason string) {
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
		TotalMS:   time.Since(t.start).Milliseconds(),
		MaxGapMS:  t.maxGap.Milliseconds(),
		Chunks:    t.chunks,
		EndReason: endReason,
	}
	if !t.firstByte.IsZero() {
		rec.FirstByteMS = t.firstByte.Sub(t.start).Milliseconds()
	}
	obsWrite(rec)
}

// obsCompact — OBS-4：压缩事件一行（成功与失败都调；counter_reset 是"被压垮"判据的核心字段）
func obsCompact(session string, cause string, in, out, summaryChars int, d time.Duration, result, failReason string, counterReset bool) {
	obsWrite(ObsRecord{
		Kind: "compact", Session: session, Cause: cause,
		TokensIn: in, TokensOut: out, SummaryChars: summaryChars,
		TotalMS: d.Milliseconds(), Result: result, FailReason: failReason,
		CounterReset: counterReset,
	})
}

// obsTool — OBS-3：工具轮次一行
func obsTool(session string, round int, tool string, d time.Duration, ok bool, rounds, max int) {
	res := "ok"
	if !ok {
		res = "err"
	}
	obsWrite(ObsRecord{
		Kind: "tool", Session: session, Round: round, Tool: tool,
		TotalMS: d.Milliseconds(), Result: res, ToolRounds: rounds, ToolMax: max,
	})
}

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
