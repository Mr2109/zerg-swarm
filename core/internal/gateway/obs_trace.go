// obs_trace.go —— T1.6：把**传播事实**（入站采纳 / 出站注入）写进 chat_obs.jsonl。
//
// 为什么要写：B16 的验收是"主控→子端一次调用 ⇒ 两侧事件同 trace_id"。头里带了还不够——
// 主控这侧必须留下**它用的到底是哪条 trace**的凭证，否则两侧无法对账（子端说收到 X，主控说不知道）。
//
// 独立性：与 obs_failover.go 同源——本文件**不 import chat 包**（避免与 chat→gateway 方向成环），
// 按同一份 JSONL（<state>/chat_obs.jsonl）追加一行；best-effort：写不进去不影响转发。
//
// 字段口径（重要）：
//   - `dir=in` 的事件：traceparent/baggage 是**上游来的原文**（可能与我们所写不同）；
//     `dir=out` 的事件：是我们**实际写进请求头**的原文（子端收到什么，这里就记什么）。
//   - trace_id 是 32 位十六进制，与 T1.1 的内部 trace_id **同域** ⇒ 这就是两侧能对上的键。
//   - span_id / parent_span_id 是**线上域**（16 位）：本仓内部 span_id 是 32 位，两者不是同一个 id 空间，
//     禁止互相截断/填充来"凑一个"。跨进程对账只用 trace_id（父子层级在主控内部另有 32 位那一套）。
//   - `root=true` = 这条 trace 是本侧生成的（上游与本会话都没有可用上下文）；
//     `upstream` 三态 present/absent/invalid —— 用三态而不是布尔，因为"上游给了个非法值"必须与
//     "上游没给"分开（前者说明对端实现有问题，是真实缺陷信号）。
package gateway

import (
	"encoding/json"
	"os"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"
)

// ObsTraceFact —— 一条传播事实（入站或出站）。
type ObsTraceFact struct {
	Dir          string // in | out
	Session      string
	Model        string
	PeerHost     string // 出站对端机器名（入站为空）
	PeerPath     string // 出站对端路径（如 /infer）
	TraceID      string
	SpanID       string
	ParentSpanID string
	ParentSource string // header | session | ""
	Traceparent  string // 实际使用/发出的原文
	Baggage      string // 实际使用/发出的原文
	Root         bool
	Upstream     string // present | absent | invalid
	RejectReason string // upstream=invalid 时的拒绝原因（不沿用非法值）
	GenErr       string // 本侧生成失败（随机源不可用）——此时不得当真值用
}

// obsTraceRecord —— 落盘的 JSONL 行（kind=trace，event_name=trace_propagation ⇒ 非空即事件）。
type obsTraceRecord struct {
	TS           string `json:"ts"`
	Kind         string `json:"kind"`
	EventName    string `json:"event_name"`
	Dir          string `json:"dir"`
	Session      string `json:"session,omitempty"`
	Model        string `json:"model,omitempty"`
	PeerHost     string `json:"peer_host,omitempty"`
	PeerPath     string `json:"peer_path,omitempty"`
	TraceID      string `json:"trace_id,omitempty"`
	SpanID       string `json:"span_id,omitempty"`
	ParentSpanID string `json:"parent_span_id,omitempty"`
	ParentSource string `json:"parent_source,omitempty"`
	Traceparent  string `json:"traceparent,omitempty"`
	Baggage      string `json:"baggage,omitempty"`
	// 两个判定字段**恒写**（不是 omitempty）：false/absent 是有信息量的结论，不能被"缺席"吞掉
	Root         bool   `json:"root"`
	Upstream     string `json:"upstream"`
	RejectReason string `json:"reject_reason,omitempty"`
	GenErr       string `json:"gen_error,omitempty"`
}

// ObsTrace — 记一条传播事实（kind=trace）。best-effort：失败只返回，不影响转发。
func ObsTrace(f ObsTraceFact) {
	p := obsFailoverPath() // 与 chat 观测同一份文件（<state>/chat_obs.jsonl）
	if p == "" {
		return
	}
	up := f.Upstream
	if up == "" {
		up = tracectx.UpstreamAbsent
	}
	rec := obsTraceRecord{
		TS: time.Now().Format(time.RFC3339), Kind: "trace", EventName: "trace_propagation",
		Dir: f.Dir, Session: f.Session, Model: f.Model,
		PeerHost: f.PeerHost, PeerPath: f.PeerPath,
		TraceID: f.TraceID, SpanID: f.SpanID, ParentSpanID: f.ParentSpanID, ParentSource: f.ParentSource,
		Traceparent: f.Traceparent, Baggage: f.Baggage,
		Root: f.Root, Upstream: up, RejectReason: f.RejectReason, GenErr: f.GenErr,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	file, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(b, '\n'))
}

// traceErrText — 错误 → 事件里的短文本（空 error ⇒ 空串）。不截断也没有超长风险（都是哨兵短句）。
func traceErrText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// traceUpstreamSpanID — 入站事件里的**父** span（wire 域）：上游头里的 span-id（我们沿用了它的 trace
// ⇒ 它的 span 就是本跳的父）；本侧新生成 root ⇒ 没有父（空串 = 缺席，不是编一个）。
func traceUpstreamSpanID(in tracectx.Inbound) string {
	if in.Upstream == tracectx.UpstreamPresent {
		return in.Trace.SpanID
	}
	return ""
}

// traceParentSourceOf — 父 span 的来源：只有"沿用上游"才算 header 父；本侧 root ⇒ 无来源。
func traceParentSourceOf(in tracectx.Inbound) string {
	if in.Upstream == tracectx.UpstreamPresent {
		return tracectx.ParentFromHeader
	}
	return ""
}
