package startup

import "testing"

// 判据：阶段流水如实、就绪幂等、快照能答"到哪一步了"。
func TestMarkReadySnapshot(t *testing.T) {
	Mark("第一步")
	phase, ready, _, _ := Snapshot()
	if phase != "第一步" || ready {
		t.Fatalf("刚 Mark 未 Ready 时应是 (第一步,false)，实得 (%s,%v)", phase, ready)
	}
	Ready()
	phase, ready, _, hist := Snapshot()
	if !ready || phase != "第一步" {
		t.Fatalf("Ready 后应是 (第一步,true)，实得 (%s,%v)", phase, ready)
	}
	if len(hist) != 1 || hist[0].Name != "第一步" {
		t.Fatalf("阶段流水应恰好 1 条，实得 %+v", hist)
	}
	// 幂等：重复 Ready 不改流水、仍为就绪
	Ready()
	if _, r2, _, h2 := Snapshot(); !r2 || len(h2) != 1 {
		t.Fatalf("Ready 必须幂等，实得 ready=%v 条数=%d", r2, len(h2))
	}
	Mark("第二步")
	if p, _, _, h3 := Snapshot(); p != "第二步" || len(h3) != 2 {
		t.Fatalf("再 Mark 后应 (第二步,2 条)，实得 (%s,%d)", p, len(h3))
	}
}
