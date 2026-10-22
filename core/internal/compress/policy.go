// Package compress 是虫族「上下文压缩」的独立能力层。
//
// 职责边界（设计稿 v5 §3）：
//   - 只定义「片段 / 策略 / 结果 / 事件」与引擎 seam；
//   - 不认识会话、任务、网关的任何类型（只 import stdlib）；
//   - 引擎可插拔：free（缺省，零算力）/ summary（P3）/ external（P4，默认关）。
//
// ★ 消费者只填 Policy 参数，不得自带阈值常量。
package compress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Kind —— 片段类别（对齐 EMNLP 2026 综述五分类）。
type Kind string

const (
	KindObservation    Kind = "observation"    // 工具输出 / 环境观察
	KindTrajectory     Kind = "trajectory"     // 对话与推理轨迹
	KindPlan           Kind = "plan"           // 计划 / 未完成 todo
	KindMemory         Kind = "memory"         // 长期记忆 / state.md 段落
	KindRepresentation Kind = "representation" // 结构表征（表格、索引、摘要块）
)

// Trigger —— 触发来源（审计三维修订之一）。
type Trigger string

const (
	TriggerPressure Trigger = "pressure" // 到阈值
	TriggerOverflow Trigger = "overflow" // 真溢出后恢复
	TriggerManual   Trigger = "manual"   // 人工 compact
	TriggerNone     Trigger = "none"     // 未触发
)

// Segment —— 一段可压缩内容。
type Segment struct {
	Kind    Kind
	Pinned  bool // 必留（七类之一）
	Text    string
	Tokens  int      // 0 = 未测，由 Apply 用 EstimateTokens 补
	Source  string   // 会话 id / 任务 id / 件路径
	RawPtr  string   // 原文落盘点（软隐藏，可回查）
	Anchors []string // 文件路径 / 命令 / 标识符 —— 永不丢
}

// Policy —— 一个场景的全部参数；消费者只填它，不写压缩逻辑。
type Policy struct {
	Budget       int           // 目标窗口（token）
	RotPct       float64       // 0.50 —— 到线只做无损剪枝
	CompactPct   float64       // 0.80 —— 到线允许摘要
	PerOutputPct float64       // 0.25 —— 单条工具输出卸载闸
	HeadroomPct  float64       // 0.70 —— 累计余量卸载闸
	KeepRecent   int           // 最近 N 条原文保留
	KeepPinned   bool          // 必留七类强制保留（恒 true）
	FocusTopic   string        // 用户指的重点
	Engine       string        // free（缺省）| summary | external
	MaxRatio     float64       // 输出/输入上限（0.25）
	Deadline     time.Duration // 超时即降级

	// Summarize —— summary 引擎的注入面（P3）：nil ⇒ 降级 free 并在 Result.Fallback 标记。
	// 放这里而不是塞全局变量：一个场景一个 Policy，注入随参数走，并发安全。
	Summarize SummarizerFunc
}

// DefaultPolicy —— 缺省参数（对齐设计稿 v5 §5 两线三档）。
func DefaultPolicy(budget int) Policy {
	return Policy{
		Budget:       budget,
		RotPct:       0.50,
		CompactPct:   0.80,
		PerOutputPct: 0.25,
		HeadroomPct:  0.70,
		KeepRecent:   2,
		KeepPinned:   true,
		Engine:       "free",
		MaxRatio:     0.25,
		Deadline:     120 * time.Second,
	}
}

// Result —— 一次压缩的结果；三维字段供审计分流。
type Result struct {
	Segments   []Segment
	Before     int // 压缩前 token
	After      int // 压缩后 token
	Ratio      float64
	Fallback   bool
	Reason     string
	Trigger    Trigger
	Mechanism  string // prune | mask | summarize | hybrid | none
	Aggressive string // light | balanced | hard
}

// Event —— 审计三态。
type Event struct {
	Phase      string // started | completed | failed
	At         time.Time
	Trigger    Trigger
	Mechanism  string
	Aggressive string
	Model      string
	Before     int
	After      int
	Fallback   bool
	Reason     string
}

// Engine —— seam：给片段与策略，返回压缩后的片段。
type Engine interface {
	Name() string
	Compress(ctx context.Context, segs []Segment, p Policy) ([]Segment, string, error) // (结果, 机制名, err)
}

// FallbackReporter —— 引擎自报「这一轮降级了」的可选接口。
//
// ★ 不扩 Engine seam：free 不需要实现；summary 在 Summarizer 未注入时降级 free，
// Apply 据 (true, 原因) 把 Result.Fallback / Result.Reason 填上（观测口径 §9）。
type FallbackReporter interface {
	Fallback() (bool, string)
}

// ErrGuard —— 必留闸失败（整次压缩作废、调用方须回退原文）。
var ErrGuard = errors.New("compress: 必留闸失败")

// EstimateTokens —— 粗估：中日韩字符约 1 token/字，其余约 1 token/4 字节。
func EstimateTokens(s string) int {
	ascii, other := 0, 0
	for _, r := range s {
		if r < 128 {
			ascii++
		} else {
			other++
		}
	}
	n := ascii/4 + other
	if n < 1 && len(s) > 0 {
		n = 1
	}
	return n
}

// TotalTokens —— 补全 0 值后求和。
func TotalTokens(segs []Segment) int {
	t := 0
	for i := range segs {
		if segs[i].Tokens <= 0 {
			segs[i].Tokens = EstimateTokens(segs[i].Text)
		}
		t += segs[i].Tokens
	}
	return t
}

// Hash 便于必留闸与审计。
func Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// Apply —— 唯一自动入口：按 Policy 判定该不该压、压到什么程度。
func Apply(ctx context.Context, segs []Segment, p Policy) (Result, error) {
	res := Result{Trigger: TriggerNone, Mechanism: "none", Aggressive: "light"}
	before := TotalTokens(segs)
	res.Before, res.After = before, before
	if p.Budget <= 0 || before == 0 {
		res.Segments = segs
		return res, nil
	}
	ratio := float64(before) / float64(p.Budget)
	if ratio < p.RotPct {
		res.Segments = segs
		return res, nil // ★ 未撞线不压（v3 拍板 2）
	}
	res.Trigger = TriggerPressure
	if ratio >= 1.0 {
		res.Trigger = TriggerOverflow
	}

	// 阶段一：无损剪枝（任何档都先做）
	prune := &PrunerFree{PerOutputPct: p.PerOutputPct, HeadroomPct: p.HeadroomPct, KeepRecent: p.KeepRecent}
	out, mech, err := prune.Compress(ctx, segs, p)
	if err != nil {
		res.Fallback = true
		res.Reason = err.Error()
		res.Segments = segs
		return res, err
	}
	res.Mechanism, res.Aggressive = mech, "light"
	after := TotalTokens(out)
	res.After, res.Ratio = after, ratio

	// 阶段二：仅在窗口线以上才允许摘要（P3：summary 引擎；Summarizer 未注入即降级 free）
	if float64(after)/float64(p.Budget) >= p.CompactPct {
		if e := engineOf(p.Engine, p.Summarize); e != nil && e.Name() != "free" {
			cc, cancel := context.WithTimeout(ctx, p.Deadline)
			defer cancel()
			if s2, m2, e2 := e.Compress(cc, out, p); e2 == nil {
				out, mech, res.Aggressive = s2, m2, "balanced"
				// 引擎自报降级（如 summary 无 Summarizer）：机制仍是 free 的，档位退回 light
				if fb, ok := e.(FallbackReporter); ok {
					if hit, why := fb.Fallback(); hit {
						res.Fallback, res.Reason, res.Aggressive = true, why, "light"
					}
				}
				after = TotalTokens(out)
				res.After, res.Mechanism = after, mech
			} else {
				res.Fallback, res.Reason = true, e2.Error() // 摘要失败 ⇒ 留 free 结果（不阻塞主循环）
			}
		}
	}

	// 必留闸：七类逐条比对，失败即整次作废
	if err := guardPinned(segs, out); err != nil {
		res.Fallback, res.Reason = true, err.Error()
		res.Segments = segs
		return res, fmt.Errorf("%w：%v", ErrGuard, err)
	}
	res.Segments = out
	return res, nil
}

func engineOf(name string, sum SummarizerFunc) Engine {
	switch name {
	case "", "free":
		return &PrunerFree{}
	case "summary":
		return &SummaryEngine{Summarize: sum} // P3：本体零依赖，Summarizer 由 Policy 注入
	default:
		return nil // external（P4）默认关
	}
}
