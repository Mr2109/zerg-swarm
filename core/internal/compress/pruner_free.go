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
	//
	// ★ 本闸曾经死循环：带 raw_ptr 的段卸过一次后 Tokens 仍 >0、段也仍在窗口里，
	// 于是每轮都被当成「候选」再卸一次 —— 而占位符里的数字位数越缩越小、token 几乎不降，
	// 循环体又不看 ctx，整批跑批就闷死在一档里。三重防呆：
	//  1. 每轮先看 ctx：上游取消/超时要能立刻收手，不许闷在循环里；
	//  2. unload 幂等：文本已是目标占位符/折叠形态就返回 false（见 unload）；
	//  3. 进展检测：一轮下来 token 没降 ⇒ break，绝不空转。
	stalled := make([]bool, len(out))
	for total := TotalTokens(out); total > headroomLimit; {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		idx := -1
		for i := 0; i < protectedFrom; i++ {
			if stalled[i] || out[i].Pinned || out[i].Kind != KindObservation || out[i].Tokens <= 0 {
				continue
			}
			if idx < 0 || out[i].Tokens > out[idx].Tokens {
				idx = i
			}
		}
		if idx < 0 {
			break
		}
		before := total
		if !unload(&out[idx]) {
			stalled[idx] = true // 该段已卸载到位（幂等 false）：本轮别再选它
			continue
		}
		total = TotalTokens(out)
		if total >= before {
			break // 没释放任何 token ⇒ 余下都是不可压缩面（钉住/会话段），不空转
		}
		mechanism = "mask"
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
// 返回是否真改了内容；**幂等**：文本已经是目标形态就返回 false。
//
// ★ 幂等的意义不只是「省一次写」：余量闸按 Tokens 选最大段，卸载后的占位符 Tokens 仍 >0
// 且段仍在窗口里，没有这条判定就会被反复选中、反复「卸载」而一个 token 都不掉（老死循环）。
// 判定用「目标文本 == 当前文本」而不是另加标志位：目标形态由 Tokens/RawPtr/正文唯一决定，
// 一旦相等，再写一遍必然一字不变 ⇒ 天然收敛。
func unload(s *Segment) bool {
	if s.RawPtr != "" {
		next := unloadedText(s.Tokens, s.RawPtr)
		if s.Text == next {
			return false // 已卸载过：再写只换占位符里的数字，不释放 token
		}
		s.Text = next
		s.Tokens = EstimateTokens(s.Text)
		return true
	}
	// 无指针 ⇒ 不许整段丢（否则不可逆）：保头尾
	r := []rune(s.Text)
	if len(r) <= unloadHeadN+unloadTailN {
		return false
	}
	next := foldedText(r)
	if s.Text == next {
		return false // 已折叠过
	}
	s.Text = next
	s.Tokens = EstimateTokens(s.Text)
	return true
}

const (
	unloadHeadN = 400
	unloadTailN = 200
)

// unloadedText —— 带指针段的卸载形态（唯一构造点：写入与幂等判定必须同一份）。
func unloadedText(tokens int, rawPtr string) string {
	return fmt.Sprintf("〔已卸载 %d token · 原文可回查：%s〕", tokens, rawPtr)
}

// foldedText —— 无指针段的折叠形态（同上，唯一构造点）。
func foldedText(r []rune) string {
	return string(r[:unloadHeadN]) +
		fmt.Sprintf("\n…〔中段 %d 字已折叠〕…\n", len(r)-unloadHeadN-unloadTailN) +
		string(r[len(r)-unloadTailN:])
}

// joinAnchors 便于适配器复用：把锚点集合并成一行。
func joinAnchors(a []string) string {
	if len(a) == 0 {
		return ""
	}
	return strings.Join(a, " · ")
}
