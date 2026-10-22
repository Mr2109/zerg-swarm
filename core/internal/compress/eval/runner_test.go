package eval

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var evalOut = flag.String("eval-out", "", "跑批报告基名（无扩展名）：<base>.json/.md/.jsonl；空=只跑不落盘")

// 主跑批：free 引擎 × observation 10 条，真调 compress.Apply。
//
// 断言的是「尺子能用」而不是「数字好看」：
//   - 对照样本必须 trigger=none 且 after==before（未撞线不压）
//   - 非对照样本必须真压下去（after<before，trigger≠none）
//   - M5 硬闸必须满格（任一 window needle 丢了就是引擎/样本 bug）
//   - RVS 必须 1.000（卸载必须可逆）
func TestBatch_FreeBaseline(t *testing.T) {
	// 默认落到临时目录（测试自清）；显式给 -eval-out 时把 unload 原件留在 <base>.work
	// —— RVS 的证据必须能在盘上复看，不能随测试进程一起消失。
	dir := t.TempDir()
	outBase := ""
	if *evalOut != "" {
		dir = *evalOut + ".work"
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("建 work 目录：%v", err)
		}
		outBase = *evalOut
	}
	rep, err := RunBatch(context.Background(), Samples(), Config{
		Budget:  8192,
		Engine:  "free",
		WorkDir: dir,
		OutBase: outBase,
	})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}

	if rep.N != 10 || len(rep.Rows) != 10 {
		t.Fatalf("rows = %d / n = %d，要求 10", len(rep.Rows), rep.N)
	}

	for _, row := range rep.Rows {
		if row.Control {
			if row.Trigger != "none" {
				t.Errorf("%s：对照样本 trigger = %s，应为 none（规2）", row.SampleID, row.Trigger)
			}
			if row.After != row.Before {
				t.Errorf("%s：对照样本 after=%d before=%d，未撞线不许动", row.SampleID, row.After, row.Before)
			}
			continue
		}
		if row.Trigger == "none" {
			t.Errorf("%s：非对照样本没撞线（before=%d budget=%d）", row.SampleID, row.Before, row.Budget)
		}
		if row.After >= row.Before {
			t.Errorf("%s：非对照样本 after=%d ≥ before=%d，闸没生效", row.SampleID, row.After, row.Before)
		}
		if row.Mechanism == "none" {
			t.Errorf("%s：压了但 mechanism=none", row.SampleID)
		}
	}

	// 硬闸逐条（M5/M6/M7 + 对照）
	for _, g := range rep.Gates {
		if !g.Pass {
			t.Errorf("闸失败：%s got=%s want=%s", g.Name, g.Got, g.Want)
		}
	}
	if rep.M5.MRRWin != 1 {
		t.Errorf("MRR_win = %.4f，硬闸要求 1.000", rep.M5.MRRWin)
	}
	if rep.M5.MRRRec != 1 {
		t.Errorf("MRR_rec = %.4f，要求 1.000（recoverable 必须能从盘上取回）", rep.M5.MRRRec)
	}
	if rep.M7.Total == 0 {
		t.Fatalf("RVS 分母为 0：一条都没卸载，样本构造有问题")
	}
	if rep.GuardFails != 0 || rep.Fallbacks != 0 {
		t.Errorf("guard_fails=%d fallbacks=%d，应为 0", rep.GuardFails, rep.Fallbacks)
	}

	t.Logf("M2 TPT 总=%d 均=%.1f p95=%d max=%d（压缩前 %d）",
		rep.M2.Total, rep.M2.Avg, rep.M2.P95, rep.M2.Max, rep.M2.TotalBefore)
	t.Logf("M3 LAT p50=%.3fms p95=%.3fms max=%.3fms", rep.M3.P50MS, rep.M3.P95MS, rep.M3.MaxMS)
	t.Logf("M5 MRR_win=%.4f(%d/%d) MRR_rec=%.4f(%d/%d) AnchorLost=%d/%d",
		rep.M5.MRRWin, rep.M5.MRRWinHits, rep.M5.MRRWinDenom,
		rep.M5.MRRRec, rep.M5.MRRRecHits, rep.M5.MRRRecDenom, rep.M5.AnchorLost, rep.M5.AnchorTotal)
	t.Logf("M7 RVS=%d/%d · M4 删除比中位=%.4f · trigger 分布=%v",
		rep.M7.OK, rep.M7.Total, rep.RAT.DeletedMedian, rep.TriggerCounts)
	for _, row := range rep.Rows {
		t.Logf("  %s %s control=%-5v before=%-5d after=%-5d del=%.3f lat=%.3fms trig=%s",
			row.SampleID, row.Lang, row.Control, row.Before, row.After, row.Deleted, row.LatencyMS, row.Trigger)
	}

	// 真产物：只有显式给了 -eval-out 才写（绝不默认往 state 里写）
	if outBase != "" {
		for _, ext := range []string{".json", ".md", ".jsonl"} {
			st, err := os.Stat(outBase + ext)
			if err != nil {
				t.Fatalf("产物缺失 %s：%v", outBase+ext, err)
			}
			t.Logf("产物 %s（%d 字节）", outBase+ext, st.Size())
		}
		entries, _ := os.ReadDir(filepath.Join(dir, "unload"))
		t.Logf("卸载原件 %d 件，留在 %s", len(entries), filepath.Join(dir, "unload"))
	}
}

// 确定性：两次跑批的「剔除时间/耗时」指纹必须相同，卸载原件逐字节相同。
// 这条红了说明压缩行为或样本带进了不确定性——那样的指标不能当基线复跑。
func TestBatch_Deterministic(t *testing.T) {
	var reps []*Report
	var unloadSums []string
	for i := 0; i < 2; i++ {
		dir := t.TempDir()
		rep, err := RunBatch(context.Background(), Samples(), Config{
			Budget: 8192, Engine: "free", WorkDir: dir,
		})
		if err != nil {
			t.Fatalf("第 %d 次跑批：%v", i+1, err)
		}
		reps = append(reps, rep)

		// 卸载原件：内容寻址 ⇒ 名字即指纹，两次必须同集合、同字节
		entries, err := os.ReadDir(filepath.Join(dir, "unload"))
		if err != nil {
			t.Fatalf("读 unload：%v", err)
		}
		sum := ""
		for _, e := range entries {
			b, err := os.ReadFile(filepath.Join(dir, "unload", e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			sum += fmt.Sprintf("%s:%s\n", e.Name(), sha256Hex(b))
		}
		unloadSums = append(unloadSums, sum)
		t.Logf("第 %d 次：samples=%s rows=%d unload 件数=%d", i+1, rep.SamplesSHA256[:16], len(rep.Rows), len(entries))
	}
	if unloadSums[0] != unloadSums[1] {
		t.Errorf("两次卸载原件不一致（内容寻址失效）")
	}

	if reps[0].SamplesSHA256 != reps[1].SamplesSHA256 {
		t.Errorf("两次 samples_sha256 不同")
	}
	if d0, d1 := reps[0].DeterministicDigest(), reps[1].DeterministicDigest(); d0 != d1 {
		t.Fatalf("两次确定性指纹不同：%s vs %s", d0, d1)
	} else {
		t.Logf("确定性指纹 %s 两次一致", d0)
	}
	for i := range reps[0].Rows {
		a, b := reps[0].Rows[i], reps[1].Rows[i]
		if a.Before != b.Before || a.After != b.After || a.Trigger != b.Trigger || a.TPT != b.TPT {
			t.Errorf("%s：两次读数不同 before %d/%d after %d/%d trigger %s/%s",
				a.SampleID, a.Before, b.Before, a.After, b.After, a.Trigger, b.Trigger)
		}
	}
}

// 负控（任务单 §5.3 第 3 条的反向证法）：把 RotPct 压到 0.10、PerOutputPct 压到 0.10，
// 对照样本必须开始压。若它们仍然 trigger=none，说明主跑批里「对照不压」是恒真的空断言，
// 那条闸不算数。
func TestBatch_ControlGateNotVacuous(t *testing.T) {
	dir := t.TempDir()
	rep, err := RunBatch(context.Background(), Samples(), Config{
		Budget: 8192, Engine: "free", WorkDir: dir,
		RotPctOverride: 0.10, PerOutputOverride: 0.10,
	})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	var controls, compressed int
	for _, row := range rep.Rows {
		if !row.Control {
			continue
		}
		controls++
		if row.Trigger == "none" {
			t.Errorf("%s：RotPct=0.10 下仍未触发（before=%d），对照断言是空的", row.SampleID, row.Before)
			continue
		}
		if row.After >= row.Before {
			t.Errorf("%s：触发了但没压下去", row.SampleID)
			continue
		}
		compressed++
	}
	if controls != 2 || compressed != 2 {
		t.Fatalf("对照样本 %d 条，真正被压 %d 条，要求 2/2", controls, compressed)
	}
	t.Logf("RotPct=0.10 负控：对照样本 %d/%d 被压 ⇒ 主跑批的 none 断言非空", compressed, controls)
}
