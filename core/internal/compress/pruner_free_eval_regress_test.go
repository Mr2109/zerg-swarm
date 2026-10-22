package compress_test

// 回归：余量闸死循环（eval 侧 2048/1024 档曾在 10s 内不终止）。
//
// 两份证据缺一不可：
//  1. 带「不可卸载段」的真实 trajectory 样本，在 2048/1024 档必须 10s 内终止并给出读数；
//  2. 原本就能终止的 8192/4096 档，逐样本读数必须与修前**逐字节一致**（下表是修前基线，
//     不是修后抄回来的数）—— 修不终止的路径，不许顺手改掉能终止场次的输出。

import (
	"context"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/compress/eval"
)

// 修前基线：8192/4096 档 20 条样本的逐样本 after（顺序 = AllSamples 顺序 R01..R20）。
var (
	base8192After = []int{927, 1340, 342, 849, 2229, 781, 1265, 1032, 627, 1911, 3472, 3351, 3466, 5069, 1978, 3329, 5168, 3484, 4779, 1990}
	base4096After = []int{927, 1340, 342, 849, 444, 781, 1265, 1032, 627, 1911, 2344, 2334, 2233, 2184, 1978, 2251, 2156, 2302, 2064, 1990}
	base8192TPT   = []int{927, 1340, 342, 849, 2229, 781, 1266, 1033, 627, 1911, 3477, 3354, 3469, 5073, 1981, 3333, 5170, 3487, 4783, 1992}
	base4096TPT   = []int{927, 1340, 342, 849, 444, 781, 1266, 1033, 627, 1911, 2348, 2337, 2235, 2187, 1981, 2254, 2158, 2305, 2067, 1992}
)

func TestPrunerFree_TwoTierOutputUnchanged(t *testing.T) {
	cases := []struct {
		budget     int
		after, tpt []int
		m2, m4     float64
		winHits    int
		recHits    int
	}{
		{8192, base8192After, base8192TPT, 47424, 0.4000, 60, 100},
		{4096, base4096After, base4096TPT, 31384, 0.6130, 52, 100},
	}
	for _, c := range cases {
		c := c
		t.Run(strconv.Itoa(c.budget), func(t *testing.T) {
			rep, err := eval.RunBatch(context.Background(), eval.TwoClassSamples(), eval.Config{ // ★ 冻结 cohort（R01–R20）：本件基线按修前 20 条钉死，不随尺子扩容漂移
				Budget: c.budget, Engine: "free", WorkDir: t.TempDir(),
			})
			if err != nil {
				t.Fatalf("RunBatch: %v", err)
			}
			if len(rep.Rows) != len(c.after) {
				t.Fatalf("rows=%d，基线 %d", len(rep.Rows), len(c.after))
			}
			for i, row := range rep.Rows {
				if row.After != c.after[i] {
					t.Errorf("%s after=%d，修前基线 %d", row.SampleID, row.After, c.after[i])
				}
				if row.TPT != c.tpt[i] {
					t.Errorf("%s TPT=%d，修前基线 %d", row.SampleID, row.TPT, c.tpt[i])
				}
			}
			if float64(rep.M2.Total) != c.m2 {
				t.Errorf("M2 total=%d，基线 %v", rep.M2.Total, c.m2)
			}
			if math.Abs(rep.RAT.DeletedMedian-c.m4) > 1e-6 {
				t.Errorf("M4 deleted 中位=%.6f，基线 %.4f", rep.RAT.DeletedMedian, c.m4)
			}
			if rep.M5.MRRWinHits != c.winHits || rep.M5.MRRWinDenom != 60 {
				t.Errorf("M5 win=%d/%d，基线 %d/60", rep.M5.MRRWinHits, rep.M5.MRRWinDenom, c.winHits)
			}
			if rep.M5.MRRRecHits != c.recHits || rep.M5.MRRRecDenom != 100 {
				t.Errorf("M5 rec=%d/%d，基线 %d/100", rep.M5.MRRRecHits, rep.M5.MRRRecDenom, c.recHits)
			}
			if rep.RAT.AfterGTBefore != 0 {
				t.Errorf("after>before 的样本 %d 条（基线 0）", rep.RAT.AfterGTBefore)
			}
			t.Logf("budget=%-5d after=%d M2均=%.1f M4中位=%.4f M5win=%.4f(%d/%d) M5rec=%.4f(%d/%d) —— 与修前基线逐字节一致",
				c.budget, rep.M2.Total, rep.M2.Avg, rep.RAT.DeletedMedian,
				rep.M5.MRRWin, rep.M5.MRRWinHits, rep.M5.MRRWinDenom,
				rep.M5.MRRRec, rep.M5.MRRRecHits, rep.M5.MRRRecDenom)
		})
	}
}

// 真复现面：trajectory 10 条（既有钉住轮次、也有带 raw_ptr 的工具输出段），
// 低档必须在 10s 内终止并给出读数；不终止就是余量闸又在同一段上空转。
func TestPrunerFree_TrajectoryTerminatesAtLowBudget(t *testing.T) {
	samples := eval.TrajectorySamples()
	if len(samples) != 10 {
		t.Fatalf("trajectory 样本应 10 条，得 %d", len(samples))
	}
	for _, budget := range []int{2048, 1024} {
		budget := budget
		t.Run(strconv.Itoa(budget), func(t *testing.T) {
			type out struct {
				rep *eval.Report
				err error
			}
			done := make(chan out, 1)
			work := t.TempDir()
			go func() {
				rep, err := eval.RunBatch(context.Background(), samples, eval.Config{
					Budget: budget, Engine: "free", WorkDir: work,
				})
				done <- out{rep, err}
			}()
			select {
			case r := <-done:
				if r.err != nil {
					t.Fatalf("RunBatch: %v", r.err)
				}
				rep := r.rep
				if len(rep.Rows) != len(samples) {
					t.Fatalf("rows=%d，样本 %d", len(rep.Rows), len(samples))
				}
				for _, row := range rep.Rows {
					if row.Before <= 0 || row.After <= 0 {
						t.Errorf("%s before=%d after=%d（低档也应出非零读数）", row.SampleID, row.Before, row.After)
					}
				}
				if rep.M5.MRRWinDenom == 0 || rep.M5.MRRRecDenom == 0 {
					t.Errorf("MRR 分母为 0：M5 没有读数")
				}
				lost := 0
				for _, row := range rep.Rows {
					lost += len(row.WindowLostIDs)
				}
				if rep.M3.MaxMS >= 10000 {
					t.Errorf("单条最长 %.1fms，逼近 10s 闸", rep.M3.MaxMS)
				}
				t.Logf("budget=%-5d before=%d after=%d M2均=%.1f M3p95=%.3fms M3max=%.3fms M5win=%.4f(%d/%d) M5rec=%.4f(%d/%d) 丢失=%d",
					budget, rep.M2.TotalBefore, rep.M2.Total, rep.M2.Avg, rep.M3.P95MS, rep.M3.MaxMS,
					rep.M5.MRRWin, rep.M5.MRRWinHits, rep.M5.MRRWinDenom,
					rep.M5.MRRRec, rep.M5.MRRRecHits, rep.M5.MRRRecDenom, lost)
			case <-time.After(10 * time.Second):
				t.Fatalf("budget=%d：10s 内没终止 —— 余量闸死循环复发", budget)
			}
		})
	}
}
