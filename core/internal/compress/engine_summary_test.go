package compress

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// summaryFixture —— 造一份「剪枝后仍在摘要线以上」的输入：
//
//	1 条必留 plan（小）· 9 条轨迹（各 ~100 token）· 1 条超大观察（free 会把它卸成指针）。
//
// 预算 1000 ⇒ 剪枝后仍 ≈0.92×Budget（> CompactPct 0.80），所以阶段二一定会被叫到。
func summaryFixture() []Segment {
	segs := []Segment{seg(KindPlan, "计划：A→B→C", true, "")}
	for i := 0; i < 9; i++ {
		segs = append(segs, seg(KindTrajectory, strings.Repeat(fmt.Sprintf("t%d", i), 200), false, ""))
	}
	segs = append(segs, seg(KindObservation, strings.Repeat("x", 4000), false, "/tmp/big.txt"))
	return segs
}

func summaryPolicy(sum SummarizerFunc) Policy {
	p := DefaultPolicy(1000)
	p.Engine = "summary"
	p.Summarize = sum
	return p
}

// ① Summarizer 未注入（nil）⇒ 降级为 free 行为，并在 Result 里标 Fallback。
func TestSummaryEngineNilSummarizerFallsBackToFree(t *testing.T) {
	segs := summaryFixture()
	p := summaryPolicy(nil)

	res, err := Apply(context.Background(), segs, p)
	if err != nil {
		t.Fatalf("nil Summarizer 不该报错（应降级）：%v", err)
	}
	if !res.Fallback {
		t.Fatal("未注入 Summarizer 却没标 Fallback")
	}
	if !strings.Contains(res.Reason, "降级") {
		t.Fatalf("Fallback 原因缺失：%q", res.Reason)
	}
	if res.Mechanism == "summarize" {
		t.Fatalf("降级轮不许冒充 summarize：%s", res.Mechanism)
	}
	if res.After >= res.Before {
		t.Fatalf("降级后应仍是 free 的删减结果：%d → %d", res.Before, res.After)
	}
	// free 行为本身要做到位：大观察卸成指针（可逆），轨迹不动
	if !strings.Contains(res.Segments[len(res.Segments)-1].Text, "/tmp/big.txt") {
		t.Fatalf("大观察未卸载为指针：%q", res.Segments[len(res.Segments)-1].Text)
	}
}

// ② 注入伪造 summarizer ⇒ 成功路径：机制 summarize、Fallback=false、必留原样。
func TestSummaryEngineInjectedSummarizerSummarizes(t *testing.T) {
	segs := summaryFixture()
	var calls int
	var gotBatch int
	fake := func(ctx context.Context, in []Segment) ([]Segment, error) {
		calls++
		gotBatch = len(in)
		return []Segment{{Kind: KindRepresentation, Text: "〔摘要〕轨迹已压缩为一段"}}, nil
	}

	res, err := Apply(context.Background(), segs, summaryPolicy(fake))
	if err != nil {
		t.Fatalf("注入 summarizer 后不该报错：%v", err)
	}
	if calls == 0 {
		t.Fatal("注入的 summarizer 一次都没被叫到（阶段二没跑）")
	}
	if gotBatch != 8 {
		t.Fatalf("只该压「非必留 ∧ 非最近 2 条」的 8 条，实得 %d", gotBatch)
	}
	if res.Fallback {
		t.Fatalf("成功路径不该标 Fallback：%s", res.Reason)
	}
	if res.Mechanism != "summarize" || res.Aggressive != "balanced" {
		t.Fatalf("机制/档位不对：%s / %s", res.Mechanism, res.Aggressive)
	}
	if res.After >= res.Before {
		t.Fatalf("摘要后没变小：%d → %d", res.Before, res.After)
	}
	var pinnedKept bool
	for _, s := range res.Segments {
		if s.Pinned && s.Text == "计划：A→B→C" {
			pinnedKept = true
		}
	}
	if !pinnedKept {
		t.Fatal("必留项被摘要吞了（必留闸也没拦住）")
	}
}

// ③ 摘要器报错 ⇒ 退回 free 结果：Apply 不返回错误、Fallback=true、Reason 留痕。
func TestSummaryEngineErrorFallsBackToFree(t *testing.T) {
	segs := summaryFixture()
	boom := errors.New("模型端点 502")
	fake := func(ctx context.Context, in []Segment) ([]Segment, error) {
		return nil, boom
	}

	res, err := Apply(context.Background(), segs, summaryPolicy(fake))
	if err != nil {
		t.Fatalf("摘要失败不该把整次压缩弄挂（应退回 free）：%v", err)
	}
	if !res.Fallback {
		t.Fatal("摘要报错却没标 Fallback")
	}
	if !strings.Contains(res.Reason, boom.Error()) {
		t.Fatalf("Reason 没留摘要器错误：%q", res.Reason)
	}
	if res.Mechanism == "summarize" {
		t.Fatalf("失败轮不许报 summarize：%s", res.Mechanism)
	}
	if res.After >= res.Before {
		t.Fatalf("退回 free 后应仍有删减：%d → %d", res.Before, res.After)
	}
	if !strings.Contains(res.Segments[len(res.Segments)-1].Text, "/tmp/big.txt") {
		t.Fatalf("退回的 free 结果没保留卸载指针：%q", res.Segments[len(res.Segments)-1].Text)
	}
}
