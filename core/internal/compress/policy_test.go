package compress

import (
	"context"
	"strings"
	"testing"
)

func seg(k Kind, text string, pinned bool, raw string) Segment {
	return Segment{Kind: k, Text: text, Pinned: pinned, Source: "t", RawPtr: raw}
}

// ① 未撞 rot 线 ⇒ 一个字节都不动（v3 拍板「未撞窗口不压」）。
func TestApplyBelowRotUntouched(t *testing.T) {
	p := DefaultPolicy(1000)
	segs := []Segment{seg(KindTrajectory, strings.Repeat("a", 200), false, "")}
	res, err := Apply(context.Background(), segs, p)
	if err != nil {
		t.Fatal(err)
	}
	if res.Trigger != TriggerNone || res.Segments[0].Text != segs[0].Text {
		t.Fatalf("未撞线却动了：trigger=%s", res.Trigger)
	}
}

// ② 单条观察超 25% ⇒ 卸载为指针（可逆）。
func TestApplyUnloadsOversizedObservation(t *testing.T) {
	p := DefaultPolicy(1000)
	big := strings.Repeat("x", 4000) // ~1000 token
	segs := []Segment{
		seg(KindObservation, big, false, "/tmp/raw-1.txt"),
		seg(KindTrajectory, strings.Repeat("b", 400), false, ""),
	}
	res, err := Apply(context.Background(), segs, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Segments[0].Text, "/tmp/raw-1.txt") {
		t.Fatalf("未留指针：%q", res.Segments[0].Text)
	}
	if res.After >= res.Before {
		t.Fatalf("没变小：%d → %d", res.Before, res.After)
	}
	if res.Mechanism != "mask" && res.Mechanism != "prune" {
		t.Fatalf("机制名不对：%s", res.Mechanism)
	}
}

// ③ 必留片段被改写 ⇒ 整次作废（ErrGuard）。
func TestGuardRejectsModifiedPinned(t *testing.T) {
	g := NewGuard()
	before := []Segment{seg(KindPlan, "计划：A→B", true, "")}
	after := []Segment{seg(KindPlan, "计划：A→C", true, "")}
	if err := g.Check(before, after); err == nil {
		t.Fatal("必留被改写却放行")
	}
}

// ④ 锚点丢失 ⇒ 拒绝（路径永不丢）。
func TestGuardRejectsLostAnchor(t *testing.T) {
	g := NewGuard()
	before := []Segment{{Kind: KindObservation, Text: "读了 core/cmd/zerg/main.go", Source: "t", Anchors: []string{"core/cmd/zerg/main.go"}}}
	after := []Segment{{Kind: KindObservation, Text: "读了某个文件", Source: "t"}}
	if err := g.Check(before, after); err == nil {
		t.Fatal("锚点丢了却放行")
	}
}

// ⑤ rot 线到窗口线之间：只剪枝、不摘要。
func TestApplyMiddleBandPruneOnly(t *testing.T) {
	p := DefaultPolicy(1000)
	segs := []Segment{
		seg(KindObservation, strings.Repeat("o", 3000), false, "/tmp/o.txt"),
		seg(KindTrajectory, strings.Repeat("t", 1200), false, ""),
	}
	res, err := Apply(context.Background(), segs, p)
	if err != nil {
		t.Fatal(err)
	}
	if res.Mechanism == "summarize" {
		t.Fatalf("窗口线以下不许摘要：%s", res.Mechanism)
	}
	if res.Trigger == TriggerNone {
		t.Fatal("应触发 pressure")
	}
}

// ⑥ 去重：完全相同的两条只留一份（非必留）。
func TestPrunerDedup(t *testing.T) {
	p := DefaultPolicy(100)
	segs := []Segment{
		seg(KindObservation, strings.Repeat("z", 400), false, ""),
		seg(KindObservation, strings.Repeat("z", 400), false, ""),
		seg(KindTrajectory, "尾", false, ""),
	}
	out, _, err := (&PrunerFree{}).Compress(context.Background(), segs, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("去重后应 2 条，得 %d", len(out))
	}
}
