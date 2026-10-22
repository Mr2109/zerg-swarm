package eval

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// asciiOther —— EstimateTokens 的两个分桶（policy.go:132）。
func asciiOther(s string) (ascii, other int) {
	for _, r := range s {
		if r < 128 {
			ascii++
		} else {
			other++
		}
	}
	return ascii, other
}

// TestAccounting_AfterTotalIsSumOfRows —— 记账口径的守门（P2 报告作者核对出的 56/51/79/81）。
//
// 守三件事：
//  1. `after_total` 必须 = Σ rows[].after（逐段口径）。旧码把它挂了 `r.M2.Total`（拼窗口径），
//     于是 JSON 里 after_total 与 rows[].after 的合计差 51–81 token，而 deleted_total 也跟着用了
//     另一个口径 —— 人对不上账，会以为漏记。
//  2. `m2_tpt_total` 必须 = Σ rows[].tpt（拼窗口径），两份数各存一份，谁也别覆盖谁。
//  3. 两和不等时，逐行差必须**恰好**等于 EstimateTokens 的整数截断差
//     ⌊(Σa+k)/4⌋ − Σ⌊aᵢ/4⌋（k = 段数−1 个 "\n"），且 md 里写清「不是一个口径」。
//     差额只要有一个 token 落不进这个公式，就是真的漏记账，必须红。
func TestAccounting_AfterTotalIsSumOfRows(t *testing.T) {
	const budget = 1024
	samples := AllSamples()
	rep, err := RunBatch(context.Background(), samples, Config{Budget: budget, Engine: "free", WorkDir: t.TempDir()})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	tier := buildTier(budget, rep)
	tier.Status = "ok" // buildTier 不设状态，RunScan 才设；报告渲染依赖它

	var sumAfter, sumTPT int
	for _, row := range tier.Rows {
		sumAfter += row.After
		sumTPT += row.TPT
	}
	if tier.AfterTotal != sumAfter {
		t.Errorf("after_total=%d ≠ Σ rows.after=%d（差 %d）—— 汇总又挂回拼窗口径了",
			tier.AfterTotal, sumAfter, tier.AfterTotal-sumAfter)
	}
	if tier.M2Total != sumTPT {
		t.Errorf("m2_tpt_total=%d ≠ Σ rows.tpt=%d（差 %d）", tier.M2Total, sumTPT, tier.M2Total-sumTPT)
	}

	// 逐行归因：从同一份 res.Segments 重算两个口径，差必须闭合成整数截断公式。
	delta := 0
	for i, s := range samples {
		before := s.toCompress()
		compress.TotalTokens(before)
		pol := compress.DefaultPolicy(budget)
		pol.Engine = "free"
		res, applyErr := compress.Apply(context.Background(), before, pol)
		if applyErr != nil {
			t.Fatalf("%s: Apply: %v", s.ID, applyErr)
		}
		out := res.Segments
		if out == nil {
			out = before
		}
		asciiSum, otherSum, perSegFloor := 0, 0, 0
		for _, sg := range out {
			a, o := asciiOther(sg.Text)
			asciiSum += a
			otherSum += o
			perSegFloor += a / 4
		}
		seps := 0
		if len(out) > 1 {
			seps = len(out) - 1 // joinTexts 的 "\n"
		}
		want := ((asciiSum+seps)/4 + otherSum) - (perSegFloor + otherSum)
		row := rep.Rows[i]
		if row.SampleID != s.ID {
			t.Fatalf("行序错位：rows[%d]=%s samples[%d]=%s", i, row.SampleID, i, s.ID)
		}
		if got := row.TPT - row.After; got != want {
			t.Errorf("%s：tpt−after=%d，整数截断公式算得 %d（段数 %d）—— 差额里有解释不了的 token",
				s.ID, got, want, len(out))
		}
		delta += want
	}
	if delta != sumTPT-sumAfter {
		t.Errorf("逐行归因合计 %d ≠ 两和之差 %d", delta, sumTPT-sumAfter)
	}
	if sumTPT == sumAfter {
		// 两和相等时口径分离断言是空的（引擎/估算器改了？）—— 不能静默通过。
		t.Fatalf("本批 tpt 与 after 合计相等，口径分离与归因断言成了空断言；请重审估算器链路")
	}
	t.Logf("budget=%d Σafter=%d Σtpt=%d 差=%d（逐行归因闭合）", budget, sumAfter, sumTPT, sumTPT-sumAfter)

	// 报告必须写清口径：md 有第 6 条与两和张，JSON notes 有同名口径注。
	scanRep := &ScanReport{
		RunID: "acct", Engine: "free", Budgets: []int{budget},
		SamplesSHA256:    strings.Repeat("0", 64),
		N:                tier.N,
		Controls:         tier.Control,
		Tiers:            []TierRow{tier},
		LossOnsetByClass: map[string]int{},
	}
	md := renderScanMarkdown(scanRep)
	for _, want := range []string{"不是一个口径", "Σ rows.after", "Σ rows.tpt"} {
		if !strings.Contains(md, want) {
			t.Errorf("md 缺口径说明 %q", want)
		}
	}
	if !strings.Contains(md, "| "+strconv.Itoa(budget)+" | "+strconv.Itoa(sumAfter)+" | "+strconv.Itoa(sumTPT)+" |") {
		t.Errorf("md 读法第 6 条没把两和逐档摆出来（要找 | %d | %d | %d |）", budget, sumAfter, sumTPT)
	}
	note := accountingNote([]TierRow{tier})
	if !strings.Contains(note, "m2_tpt_total") || !strings.Contains(note, strconv.Itoa(sumTPT-sumAfter)+" token") {
		t.Errorf("notes 口径注没带差值与字段名：%s", note)
	}
}
