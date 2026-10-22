package eval

import (
	"context"
	"strings"
	"testing"
	"time"
)

// 本文件是 P2 收尾的两条硬断言（任务单 §4.1 / §5.1）：
//   - 全量 50 条 × 四档**全部跑完**（不得再有 timeout —— free 的不终止路径已修，
//     保险丝不该被触发；触发了就是回归，必须红）；
//   - 全量 50 条在 8192 主档上八指标齐全、硬闸全绿（MRR 两口径 / AnchorLost /
//     RBR_guard / RVS / after>before / 对照不压）。

// TestScan_AllTiersComplete —— 四档真跑一遍，逐档断言「有读数、分母非空、状态 ok」。
//
// ★ 为什么把「status == ok」写成硬断言：P2 前面那批里低档曾因 free 的余量闸不终止而
// 整档没有读数（标 timeout）。修完之后，四档全 ok 才说明「尺子能覆盖 8192→1024 全区间」；
// 任何一个档再退回 timeout，都说明引擎或样本又引入了不终止/超时的路径。
func TestScan_AllTiersComplete(t *testing.T) {
	rep, err := RunScan(context.Background(), AllSamples(), ScanConfig{
		Engine: "free", WorkRoot: t.TempDir(), TierTimeout: 20 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}
	if rep.N != 50 {
		t.Fatalf("扫描样本 %d 条，要求 50", rep.N)
	}
	if len(rep.Tiers) != 4 {
		t.Fatalf("档数 %d，要求 4", len(rep.Tiers))
	}
	if rep.NeedleTotal != 250 || rep.NeedleWindow != 180 || rep.NeedleRecoverable != 70 {
		t.Fatalf("needle 合计 %d（W %d / R %d），要求 250（180/70）",
			rep.NeedleTotal, rep.NeedleWindow, rep.NeedleRecoverable)
	}

	for _, tr := range rep.Tiers {
		if tr.Status != "ok" {
			t.Fatalf("Budget=%d：status=%s（%s）—— 四档必须全部跑完，不许再有 timeout",
				tr.Budget, tr.Status, tr.Note)
		}
		if len(tr.Rows) != 50 {
			t.Fatalf("Budget=%d：rows=%d，要求 50", tr.Budget, len(tr.Rows))
		}
		// 八指标的分母：M2/M3/M4/M5/M7 都必须在每档上真有数
		if tr.BeforeTotal <= 0 || tr.AfterTotal <= 0 || tr.M2Total <= 0 {
			t.Errorf("Budget=%d：M2 读数为空 before=%d after=%d tpt=%d", tr.Budget, tr.BeforeTotal, tr.AfterTotal, tr.M2Total)
		}
		if tr.N == 0 || tr.M3P95 <= 0 {
			t.Errorf("Budget=%d：M3 没有读数 p95=%.3f", tr.Budget, tr.M3P95)
		}
		if tr.M5WinDenom != 180 || tr.M5RecDenom != 250 {
			t.Errorf("Budget=%d：M5 分母 win=%d rec=%d，要求 180/250", tr.Budget, tr.M5WinDenom, tr.M5RecDenom)
		}
		if tr.M7Total == 0 {
			t.Errorf("Budget=%d：M7 RVS 分母为 0（一条都没卸载，样本构造或引擎没生效）", tr.Budget)
		}
		if tr.M7Rate != 1 {
			t.Errorf("Budget=%d：M7 RVS=%.4f（%d/%d），硬闸要求 1.000", tr.Budget, tr.M7Rate, tr.M7OK, tr.M7Total)
		}
		if tr.M5Anchor != 0 {
			t.Errorf("Budget=%d：AnchorLost=%d，硬闸要求 0", tr.Budget, tr.M5Anchor)
		}
		if tr.M6GuardFails != 0 || tr.M6Fallbacks != 0 {
			t.Errorf("Budget=%d：guard=%d fallback=%d，应为 0", tr.Budget, tr.M6GuardFails, tr.M6Fallbacks)
		}
		if tr.M4AfterGTBefore != 0 {
			t.Errorf("Budget=%d：after>before 出现 %d 次，要求 0", tr.Budget, tr.M4AfterGTBefore)
		}
		t.Logf("Budget=%-5d before=%d after=%d 删除均=%.4f TPT均=%.1f LATp95=%.3fms MRR_win=%.4f(%d/%d) MRR_rec=%.4f RVS=%.4f(%d/%d) guard=%d fallback=%d after>budget=%d",
			tr.Budget, tr.BeforeTotal, tr.AfterTotal, tr.DeletedAll, tr.M2Avg, tr.M3P95,
			tr.M5Win, tr.M5WinHits, tr.M5WinDenom, tr.M5Rec, tr.M7Rate, tr.M7OK, tr.M7Total,
			tr.M6GuardFails, tr.M6Fallbacks, tr.AfterOver)
	}

	// 顶档（8192）必须满格：这是「基线」的定义；低档的下降才有对照意义。
	for _, tr := range rep.Tiers {
		if tr.Budget != 8192 {
			continue
		}
		if tr.M5Win != 1 || tr.M5Rec != 1 {
			t.Fatalf("8192 档 MRR_win=%.4f MRR_rec=%.4f，硬闸要求都 1.000", tr.M5Win, tr.M5Rec)
		}
		if tr.ControlsCompressed != 0 {
			t.Fatalf("8192 档有 %d 条对照样本被压（应全部 trigger=none）", tr.ControlsCompressed)
		}
	}

	// 类内临界档：五类键必须都在；plan/memory 不许丢（钉住段），其余三类必须真丢过一次
	// —— 否则四档扫描对它们没有分辨力，这份基线就只是「一屏好看的 1.000」。
	for _, class := range ClassNames {
		onset, ok := rep.LossOnsetByClass[class]
		if !ok {
			t.Errorf("%s：LossOnsetByClass 缺这个键（0 与「没测」必须分开）", class)
			continue
		}
		switch class {
		case "plan", "memory":
			if onset != 0 {
				t.Errorf("%s：从 %d 档起丢了 window needle —— 钉住段被改写就是引擎 bug", class, onset)
			}
		default:
			if onset == 0 {
				t.Errorf("%s：四档都没丢 needle，扫描对该类没有分辨力", class)
			}
		}
		t.Logf("%-14s 首次丢 needle 的档 = %d（0 = 四档未丢）", class, onset)
	}

	// N/A 三条必须在（M1 / M8 / M6-undo），且都带分母
	if len(rep.NA) != 3 {
		t.Fatalf("N/A 条目 %d 条，要求 3（M1 / M8 / M6-undo）", len(rep.NA))
	}
	seenNA := map[string]bool{}
	for _, na := range rep.NA {
		seenNA[na.Metric] = true
		if na.Status != "N/A" || na.Denominator == 0 || na.Observed != 0 || na.Reason == "" {
			t.Errorf("N/A 条目不成形：%+v", na)
		}
	}
	for _, want := range []string{"M1 DSR", "M8 RPT", "M6 RBR_undo"} {
		if !seenNA[want] {
			t.Errorf("N/A 里缺 %s", want)
		}
	}

	// 分布表：五类各 10 条、中英各半、对照 2 条
	if len(rep.Distribution) != 5 {
		t.Fatalf("分布表 %d 类，要求 5", len(rep.Distribution))
	}
	for _, d := range rep.Distribution {
		if d.N != 10 || d.Zh != 5 || d.En != 5 || d.Controls != 2 {
			t.Errorf("%s：n=%d zh=%d en=%d 对照=%d，要求 10/5/5/2", d.Class, d.N, d.Zh, d.En, d.Controls)
		}
	}
}

// TestBatch_Full50OperatingPoint —— 8192 主档的单跑批（单测不落盘）：八指标齐全、硬闸全绿。
//
// 与扫描顶档的区别：这条走 RunBatch（单进程、单档），是把「引擎在基线档上的行为」
// 与「扫描聚合」分开证一次 —— 两条路的 before/after 必须一致，否则聚合层有 bug。
func TestBatch_Full50OperatingPoint(t *testing.T) {
	rep, err := RunBatch(context.Background(), AllSamples(), Config{
		Budget: 8192, Engine: "free", WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	if rep.N != 50 || len(rep.Rows) != 50 {
		t.Fatalf("rows=%d n=%d，要求 50", len(rep.Rows), rep.N)
	}
	if rep.Controls != 10 {
		t.Fatalf("对照样本 %d 条，要求 10", rep.Controls)
	}
	for _, g := range rep.Gates {
		if !g.Pass {
			t.Errorf("闸失败：%s got=%s want=%s", g.Name, g.Got, g.Want)
		}
	}
	if rep.M5.MRRWin != 1 || rep.M5.MRRWinDenom != 180 {
		t.Errorf("MRR_win=%.4f（%d/%d），要求 1.000（180/180）",
			rep.M5.MRRWin, rep.M5.MRRWinHits, rep.M5.MRRWinDenom)
	}
	if rep.M5.MRRRec != 1 || rep.M5.MRRRecDenom != 250 {
		t.Errorf("MRR_rec=%.4f（%d/%d），要求 1.000（250/250）",
			rep.M5.MRRRec, rep.M5.MRRRecHits, rep.M5.MRRRecDenom)
	}
	if rep.M7.Total == 0 || rep.M7.Rate != 1 {
		t.Errorf("RVS=%.4f（%d/%d），要求 1.000 且分母非 0", rep.M7.Rate, rep.M7.OK, rep.M7.Total)
	}

	// 逐样本：对照必须不压；非对照必须撞线。
	// 「撞线但一段没卸」是 free 的能力边界读数（每段都不到 perOut、总量也不到 headroom），
	// 单独数出来记日志 —— 不把它说成「压了」，也不因此判红（判红会把能力边界当成 bug）。
	var compressed, noop []string
	for _, row := range rep.Rows {
		if row.Control {
			if row.Trigger != "none" || row.After != row.Before {
				t.Errorf("%s：对照样本 trigger=%s after=%d before=%d，必须 none 且不动",
					row.SampleID, row.Trigger, row.After, row.Before)
			}
			continue
		}
		if row.Trigger == "none" {
			t.Errorf("%s：非对照样本没撞线（before=%d）", row.SampleID, row.Before)
		}
		if row.After > row.Before {
			t.Errorf("%s：非对照样本 after=%d > before=%d（压完反而更长）", row.SampleID, row.After, row.Before)
		}
		if row.After == row.Before {
			noop = append(noop, row.SampleID)
			continue
		}
		compressed = append(compressed, row.SampleID)
	}
	if len(compressed) < 37 {
		t.Errorf("真正压下去的样本只有 %d/40 条，要求 ≥37", len(compressed))
	}
	t.Logf("8192 主档：真压 %d/40 条；撞线但一段没卸 %d 条 %v（free 的能力边界）",
		len(compressed), len(noop), noop)

	// 八指标一字排开（回报要用这一行）
	t.Logf("8192 基线：M1=N/A M2总=%d 均=%.1f p95=%d M3 p50=%.3f p95=%.3f max=%.3f ms "+
		"M4 D_med=%.4f after>before=%d M5 win=%.4f rec=%.4f anchor=%d/%d "+
		"M6 guard=%d fallback=%d M7 RVS=%.4f(%d/%d) M8=N/A",
		rep.M2.Total, rep.M2.Avg, rep.M2.P95, rep.M3.P50MS, rep.M3.P95MS, rep.M3.MaxMS,
		rep.RAT.DeletedMedian, rep.RAT.AfterGTBefore, rep.M5.MRRWin, rep.M5.MRRRec,
		rep.M5.AnchorLost, rep.M5.AnchorTotal, rep.GuardFails, rep.Fallbacks,
		rep.M7.Rate, rep.M7.OK, rep.M7.Total)

	// 能力边界（本条是读数，不是判断题）：free 只卸未钉的 KindObservation ⇒
	// plan/memory 的「不丢 needle」是「Pinned 段没被碰」，representation 的载荷按
	// 工具输出形态建模才有压缩量。把这条打印出来，免得报告把「没被碰」写成「保住了」。
	byClass := map[string]int{}
	for _, row := range rep.Rows {
		byClass[row.Class] += row.UnloadedSegments
	}
	t.Logf("被卸载段数（按类）：observation=%d trajectory=%d plan=%d memory=%d representation=%d",
		byClass["observation"], byClass["trajectory"], byClass["plan"], byClass["memory"], byClass["representation"])
	if byClass["plan"] == 0 {
		t.Errorf("plan 类没有卸载任何段（每样本都该有一段未钉的执行细节）")
	}
	if byClass["memory"] == 0 {
		t.Errorf("memory 类没有卸载任何段（每样本都该有一段未钉的过程记录）")
	}
	if !strings.Contains(rep.M2.Note, "recall") {
		t.Errorf("M2 口径说明缺 recall 项：%q", rep.M2.Note)
	}
}
