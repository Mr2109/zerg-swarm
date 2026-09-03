package api

import (
	"testing"
)

func TestPhaseFlow(t *testing.T) {
	f := NewZergPhaseFlow()
	if f.Current != PhaseInit {
		t.Fatalf("初始应 init: %s", f.Current)
	}
	// 正常流转: init→plan→select→execute→close→done
	want := []ZergPhase{PhasePlan, PhaseSelect, PhaseExecute, PhaseClose, PhaseDone}
	for i, w := range want {
		got, err := f.Next()
		if err != nil {
			t.Fatalf("第 %d 步流转失败: %v", i, err)
		}
		if got != w {
			t.Fatalf("第 %d 步应 %s 实际 %s", i, w, got)
		}
	}
	// done 后不能再流转
	if _, err := f.Next(); err == nil {
		t.Fatalf("done 后流转应报错")
	}
	// 终态判断
	if !f.IsTerminal() {
		t.Fatalf("done 应终态")
	}
}

func TestPhaseFlowFailed(t *testing.T) {
	f := NewZergPhaseFlow()
	f.To(PhaseFailed)
	if !f.IsTerminal() {
		t.Fatalf("failed 应终态")
	}
}

func TestZergRoundCommit(t *testing.T) {
	dir := t.TempDir()
	z := NewZergTaskFile(dir)
	_ = z.Init("task-r1", "internal", "目标")

	// 方案轮
	r1 := NewZergRound(z, PhasePlan, "")
	n, err := r1.Commit(map[string]interface{}{"plans": []string{"A", "B"}}, "出了2个方案")
	if err != nil {
		t.Fatalf("Commit 失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("首轮应 1（含 init 行——轮次从 init 后计），实际 %d", n)
	}
	// 注意: Init 写了 round 0（init）——所以第一轮 Commit 是第 2 行但 round=1
	// 验证 Round 递增
	r2 := NewZergRound(z, PhaseSelect, "")
	n2, err := r2.Commit(map[string]interface{}{"selected": "A"}, "选定A")
	if err != nil {
		t.Fatalf("Commit2 失败: %v", err)
	}
	if n2 != 2 {
		t.Fatalf("第二轮应 2，实际 %d", n2)
	}
	lines, _ := z.ReadAll()
	if len(lines) != 3 {
		t.Fatalf("应 3 行（init+2轮），实际 %d", len(lines))
	}
}
