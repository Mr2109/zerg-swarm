package compress

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// SummarizerFunc —— summary 引擎的**唯一注入面**（★ 本体不依赖任何具模型 / 端点 / SDK）。
//
// 契约：收「一段连续、已剪枝、非必留、非最近 N 条」的片段，交回「等价的更短片段」。
// 真正调模型由实现方负责（适配器 / 上层装配时注入）；本包只管编排、预算、必留闸与降级。
// nil ⇒ 整轮降级为 free 行为（零算力、可逆），并在 Fallback() 里报出原因。
type SummarizerFunc func(ctx context.Context, segs []Segment) ([]Segment, error)

// SummaryEngine —— P3 结构化摘要引擎；与 PrunerFree **同签名**（落 Engine seam，消费者零改动）。
//
// 四条纪律：
//  1. **不接具模型**：模型只可能从 SummarizerFunc 进来；字段与 Policy 都为 nil ⇒ 降级 free；
//  2. **预算照 Policy**：Budget ≤ 0 不压；剪枝后仍在 CompactPct×Budget 以下就不动用摘要器；
//  3. **必留照 Policy**：Pinned 永不过模型、最近 KeepRecent 条原文不动、Anchor 与 RawPtr 随摘要搬运
//     （否则必留闸会整次作废）；
//  4. **出错绝不静默**：超时 / 摘要器报错 / 返回空 / 超 MaxRatio / 未减小 / 动了必留 ⇒ 返回错误，
//     调用方（Apply）据此退回 free 结果并把 Fallback 标进 Result。
type SummaryEngine struct {
	Summarize SummarizerFunc // nil ⇒ 降级 free
	Guard     Guard          // 必留闸；零值按 NewGuard()（严格查锚点）处理

	mu       sync.Mutex
	fellBack bool
	reason   string
}

func (e *SummaryEngine) Name() string { return "summary" }

// Fallback —— 本轮是否降级、为什么（可选接口 FallbackReporter：Apply 用它填 Result.Fallback/Reason）。
func (e *SummaryEngine) Fallback() (bool, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.fellBack, e.reason
}

func (e *SummaryEngine) reset() {
	e.mu.Lock()
	e.fellBack, e.reason = false, ""
	e.mu.Unlock()
}

func (e *SummaryEngine) markFallback(why string) {
	e.mu.Lock()
	e.fellBack, e.reason = true, why
	e.mu.Unlock()
}

// Compress —— 先无损剪枝、后结构化摘要；任何一步不满足 Policy 就交回错误（不硬压）。
func (e *SummaryEngine) Compress(ctx context.Context, segs []Segment, pol Policy) ([]Segment, string, error) {
	e.reset()
	if len(segs) == 0 {
		return segs, "none", nil
	}
	before := TotalTokens(segs)
	if pol.Budget <= 0 {
		return segs, "none", nil
	}

	// ① 先做无损剪枝（free 的活先干完，模型只处理还在线外的部分）
	prune := &PrunerFree{PerOutputPct: pol.PerOutputPct, HeadroomPct: pol.HeadroomPct, KeepRecent: pol.KeepRecent}
	pruned, mech, err := prune.Compress(ctx, segs, pol)
	if err != nil {
		return nil, "", err
	}

	// ② 注入面：引擎字段优先，其次 Policy（适配器可只经 Policy 传递）
	sum := e.Summarize
	if sum == nil {
		sum = pol.Summarize
	}
	if sum == nil {
		e.markFallback("summary 引擎未注入 Summarizer ⇒ 降级 free（零算力可逆删减，语义不改写）")
		return pruned, mech, nil
	}

	// ③ 剪枝已回到摘要线内 ⇒ 不必动用模型（Policy 说了算）
	if float64(TotalTokens(pruned))/float64(pol.Budget) < pol.CompactPct {
		return pruned, mech, nil
	}

	if pol.Deadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, pol.Deadline)
		defer cancel()
	}

	keep := pol.KeepRecent
	if keep < 0 {
		keep = 0
	}
	protectedFrom := len(pruned) - keep
	if protectedFrom < 0 {
		protectedFrom = 0
	}

	out := make([]Segment, 0, len(pruned))
	var run []Segment
	var runAnchors, runPtrs []string

	// flush —— 把一段连续可压片段交给摘要器，原地换成它交回的片段。
	flush := func() error {
		if len(run) == 0 {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("摘要超时/取消：%w", err)
		}
		got, err := sum(ctx, run)
		if err != nil {
			return fmt.Errorf("摘要器失败：%w", err)
		}
		if len(got) == 0 {
			return fmt.Errorf("摘要器对 %d 条片段返回空结果", len(run))
		}
		for i := range got {
			if strings.TrimSpace(got[i].Text) == "" {
				return fmt.Errorf("摘要器返回空文本")
			}
			if got[i].Kind == "" {
				got[i].Kind = KindRepresentation
			}
			got[i].Tokens = EstimateTokens(got[i].Text)
			got[i].Pinned = false // 必留项只由适配器打标；摘要不制造必留
			if got[i].Source == "" && len(run) > 0 {
				got[i].Source = run[0].Source
			}
		}
		// ★ 锚点与原文指针随摘要搬运：路径/命令/标识符永不丢（guard 逐条查），原文仍可回查（可逆）
		if len(runAnchors) > 0 {
			last := &got[len(got)-1]
			last.Anchors = append(last.Anchors, runAnchors...)
		}
		if len(runPtrs) > 0 {
			last := &got[len(got)-1]
			if last.RawPtr == "" {
				last.RawPtr = strings.Join(runPtrs, " | ")
			}
		}
		out = append(out, got...)
		run, runAnchors, runPtrs = nil, nil, nil
		return nil
	}

	for i := range pruned {
		s := pruned[i]
		// ★ 必留恒不过模型（设计稿 §5.1 硬规则）：即使 Policy.KeepPinned=false 也不放行，
		// 否则必留闸必然整次作废，等于白白烧一次调用。
		if s.Pinned || i >= protectedFrom || s.Text == "" {
			if err := flush(); err != nil {
				return nil, "", err
			}
			out = append(out, s)
			continue
		}
		run = append(run, s)
		runAnchors = append(runAnchors, s.Anchors...)
		if s.RawPtr != "" {
			runPtrs = append(runPtrs, s.RawPtr)
		}
	}
	if err := flush(); err != nil {
		return nil, "", err
	}

	// ④ 必留闸：对**原始输入**逐条比对（剪枝/摘要都不许动必留项与锚点）
	g := e.Guard
	if g == (Guard{}) {
		g = NewGuard()
	}
	if err := g.Check(segs, out); err != nil {
		return nil, "", fmt.Errorf("%w：%v", ErrGuard, err)
	}

	// ⑤ 预算硬闸：MaxRatio（输出/输入上限）与「必须真的变小」
	after := TotalTokens(out)
	if pol.MaxRatio > 0 {
		cap := int(float64(before) * pol.MaxRatio)
		if cap < 1 {
			cap = 1
		}
		if after > cap {
			return nil, "", fmt.Errorf("摘要结果超 MaxRatio(%.2f)：%d → %d token", pol.MaxRatio, before, after)
		}
	}
	if after >= before {
		return nil, "", fmt.Errorf("摘要未减小（%d → %d token）", before, after)
	}

	return out, "summarize", nil
}
