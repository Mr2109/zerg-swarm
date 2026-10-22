package compress

import (
	"context"
	"fmt"
	"strings"
)

// PrunerFree —— 缺省引擎：零算力、可逆（原文靠 RawPtr 软隐藏）。
//
// 规则（设计稿 v5 §4.2）：
//  1. 单条 observation 超过 Budget×PerOutputPct ⇒ 卸载为「摘要 ＋ 指针」；
//  2. 累计到 Budget×HeadroomPct 时，继续卸载最旧的可卸载观察，直到回到线内；
//  3. 完全重复的片段只留第一份（去重）；
//  4. Pinned 与最近 KeepRecent 条一律不动。
type PrunerFree struct {
	PerOutputPct float64
	HeadroomPct  float64
	KeepRecent   int
}

func (p *PrunerFree) Name() string { return "free" }

func (p *PrunerFree) Compress(ctx context.Context, segs []Segment, pol Policy) ([]Segment, string, error) {
	perOut := p.PerOutputPct
	if perOut <= 0 {
		perOut = pol.PerOutputPct
	}
	headroom := p.HeadroomPct
	if headroom <= 0 {
		headroom = pol.HeadroomPct
	}
	keep := p.KeepRecent
	if keep <= 0 {
		keep = pol.KeepRecent
	}
	if pol.Budget <= 0 {
		return segs, "none", nil
	}
	perOutLimit := int(float64(pol.Budget) * perOut)
	headroomLimit := int(float64(pol.Budget) * headroom)

	out := make([]Segment, len(segs))
	copy(out, segs)

	protectedFrom := len(out) - keep
	if protectedFrom < 0 {
		protectedFrom = 0
	}
	mechanism := "none"

	// ① 单条超限卸载（★ 硬闸：不看「最近 N 条」，因为一条 25% 的输出足够压死窗口）
	for i := 0; i < len(out); i++ {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		s := &out[i]
		if s.Pinned || s.Kind != KindObservation || s.Tokens <= perOutLimit || s.Tokens <= 0 {
			continue
		}
		if unload(s) {
			mechanism = "mask"
		}
	}

	// ② 余量闸：累计超线则从最旧的可卸载观察继续卸
	for total := TotalTokens(out); total > headroomLimit; {
		idx := -1
		for i := 0; i < protectedFrom; i++ {
			if out[i].Pinned || out[i].Kind != KindObservation || out[i].Tokens <= 0 {
				continue
			}
			if idx < 0 || out[i].Tokens > out[idx].Tokens {
				idx = i
			}
		}
		if idx < 0 {
			break
		}
		if !unload(&out[idx]) {
			break
		}
		mechanism = "mask"
		total = TotalTokens(out)
	}

	// ③ 去重（保留首次出现）
	seen := map[string]bool{}
	dedup := out[:0]
	for _, s := range out {
		if !s.Pinned && s.Text != "" {
			h := Hash(s.Text)
			if seen[h] {
				mechanism = "prune"
				continue
			}
			seen[h] = true
		}
		dedup = append(dedup, s)
	}
	out = dedup

	if mechanism == "none" {
		mechanism = "prune"
	}
	return out, mechanism, nil
}

// unload —— 把一条观察换成「摘要引子 ＋ 指针」；无指针则保头尾折叠。
// 返回是否真改了内容。
func unload(s *Segment) bool {
	if s.RawPtr != "" {
		s.Text = fmt.Sprintf("〔已卸载 %d token · 原文可回查：%s〕", s.Tokens, s.RawPtr)
		s.Tokens = EstimateTokens(s.Text)
		return true
	}
	// 无指针 ⇒ 不许整段丢（否则不可逆）：保头尾
	const headN, tailN = 400, 200
	r := []rune(s.Text)
	if len(r) <= headN+tailN {
		return false
	}
	s.Text = string(r[:headN]) + fmt.Sprintf("\n…〔中段 %d 字已折叠〕…\n", len(r)-headN-tailN) + string(r[len(r)-tailN:])
	s.Tokens = EstimateTokens(s.Text)
	return true
}

// joinAnchors 便于适配器复用：把锚点集合并成一行。
func joinAnchors(a []string) string {
	if len(a) == 0 {
		return ""
	}
	return strings.Join(a, " · ")
}
