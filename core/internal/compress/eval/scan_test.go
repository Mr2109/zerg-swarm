package eval

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var scanOut = flag.String("scan-out", "", "扫描产物基名（无扩展名）：<base>.json/.md；空=只跑不落盘")

// 子进程入口（由 scan_child.go 的 runTierIsolated 起）：只跑一档、写 tier-report.json。
// 普通跑批时没有 EVAL_SCAN_CHILD ⇒ 直接跳过。
func TestScanTierChild(t *testing.T) {
	if os.Getenv(childFlagEnv) == "" {
		t.Skip("不是子进程调用")
	}
	if err := RunScanTierChild(); err != nil {
		t.Fatalf("子进程跑档失败：%v", err)
	}
}

// 单档复现入口（只读诊断用，默认跳过）：把一个 Budget 档放在**本进程**里跑，
// 用 `-timeout 10s` 就能拿到卡住时的 goroutine 栈 —— 这是定位 free 不终止路径的最小复现。
//
//	PROBE_TIER=2048 PROBE_SET=all go test ./internal/compress/eval -run TestProbeOneTier -timeout 10s -v
func TestProbeOneTier(t *testing.T) {
	tier := os.Getenv("PROBE_TIER")
	if tier == "" {
		t.Skip("未给 PROBE_TIER，跳过单档复现")
	}
	b, err := strconv.Atoi(tier)
	if err != nil {
		t.Fatalf("PROBE_TIER: %v", err)
	}
	samples := AllSamples()
	if os.Getenv("PROBE_SET") == "obs" {
		samples = Samples()
	}
	rep, err := RunBatch(context.Background(), samples, Config{
		Budget: b, Engine: "free", WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("RunBatch: %v", err)
	}
	t.Logf("PROBE_SET=%s budget=%d before=%d after=%d MRR_win=%.4f MRR_rec=%.4f",
		os.Getenv("PROBE_SET"), b, rep.M2.TotalBefore, rep.M2.Total, rep.M5.MRRWin, rep.M5.MRRRec)
}

// 扫描的确定性：同一二进制两次扫，能跑完的档「剔除时间/耗时」指纹必须相同，
// 且 before/after/trigger/TPT/MRR 逐样本相同。红了说明引擎行为或样本带进了不确定性。
func TestScan_Deterministic(t *testing.T) {
	var reports []*ScanReport
	for i := 0; i < 2; i++ {
		rep, err := RunScan(context.Background(), AllSamples(), ScanConfig{
			Engine: "free", WorkRoot: t.TempDir(), TierTimeout: 5 * time.Second,
		})
		if err != nil {
			t.Fatalf("第 %d 次扫描：%v", i+1, err)
		}
		reports = append(reports, rep)
	}

	a, b := reports[0], reports[1]
	if len(a.Tiers) != len(ScanBudgets) || len(b.Tiers) != len(a.Tiers) {
		t.Fatalf("档数 %d/%d，要求 %d", len(a.Tiers), len(b.Tiers), len(ScanBudgets))
	}
	okTiers := 0
	for i := range a.Tiers {
		ta, tb := a.Tiers[i], b.Tiers[i]
		if ta.Budget != tb.Budget {
			t.Fatalf("第 %d 档 budget 不同：%d vs %d", i, ta.Budget, tb.Budget)
		}
		// 状态也必须确定：同一批样本、同一引擎，卡住的档每次都该卡住
		if ta.Status != tb.Status {
			t.Errorf("Budget=%d：两次状态不同 %s vs %s", ta.Budget, ta.Status, tb.Status)
		}
		if ta.Status != "ok" {
			t.Logf("Budget=%-5d status=%s（不出读数）", ta.Budget, ta.Status)
			continue
		}
		okTiers++
		if ta.Digest != tb.Digest {
			t.Errorf("Budget=%d：确定性指纹不同 %s vs %s", ta.Budget, ta.Digest, tb.Digest)
		}
		if ta.BeforeTotal != tb.BeforeTotal || ta.AfterTotal != tb.AfterTotal ||
			ta.M5WinHits != tb.M5WinHits || ta.M5WinDenom != tb.M5WinDenom ||
			ta.M5RecHits != tb.M5RecHits || ta.M6GuardFails != tb.M6GuardFails {
			t.Errorf("Budget=%d：两次读数不同 before %d/%d after %d/%d win %d/%d vs %d/%d",
				ta.Budget, ta.BeforeTotal, tb.BeforeTotal, ta.AfterTotal, tb.AfterTotal,
				ta.M5WinHits, tb.M5WinHits, ta.M5WinDenom, tb.M5WinDenom)
		}
		if len(ta.Rows) != len(tb.Rows) {
			t.Fatalf("Budget=%d：rows %d vs %d", ta.Budget, len(ta.Rows), len(tb.Rows))
		}
		for j := range ta.Rows {
			ra, rb := ta.Rows[j], tb.Rows[j]
			if ra.SampleID != rb.SampleID || ra.Before != rb.Before || ra.After != rb.After ||
				ra.Trigger != rb.Trigger || ra.TPT != rb.TPT || ra.MRRWinHit != rb.MRRWinHit {
				t.Errorf("Budget=%d %s：两次读数不同 after %d/%d trigger %s/%s win %d/%d",
					ta.Budget, ra.SampleID, ra.After, rb.After, ra.Trigger, rb.Trigger, ra.MRRWinHit, rb.MRRWinHit)
			}
		}
		t.Logf("Budget=%-5d digest=%s before=%d after=%d MRR_win=%.4f(%d/%d) MRR_rec=%.4f 丢失=%d",
			ta.Budget, ta.Digest, ta.BeforeTotal, ta.AfterTotal, ta.M5Win, ta.M5WinHits, ta.M5WinDenom,
			ta.M5Rec, len(ta.LostWindow))
	}
	// 四档必须全部跑完：free 的不终止路径已修，任何一档退回 timeout 都是回归。
	if okTiers != len(ScanBudgets) {
		t.Fatalf("跑完的档只有 %d/%d 个（不确定性不是「少跑几档」能躲过去的）", okTiers, len(ScanBudgets))
	}
}

// 低 budget 必须真丢（否则尺子仍无分辨力）—— 若真没丢，本测试**明确报「仍未丢」**并
// 把各档读数打进日志，绝不把「满格」说成「合格」。
//
// 同时证明丢的机制是「闸只动未钉的 observation 段」：凡丢失的 window needle，
// 其所在段必须未钉（钉住的段丢了就是引擎 bug，不是预算问题）；
// 并且不终止的档**不许带任何读数**（不记 0、不推算）。
func TestScan_NeedleLossAtLowBudget(t *testing.T) {
	rep, err := RunScan(context.Background(), AllSamples(), ScanConfig{
		Engine: "free", WorkRoot: t.TempDir(), TierTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunScan: %v", err)
	}

	byID := map[string]Sample{}
	for _, s := range AllSamples() {
		byID[s.ID] = s
	}

	var top, lowest *TierRow
	for i := range rep.Tiers {
		tr := &rep.Tiers[i]
		if tr.Budget == 8192 {
			top = tr
		}
		if tr.Status == "ok" && (lowest == nil || tr.Budget < lowest.Budget) {
			lowest = tr
		}
	}
	if top == nil || top.Status != "ok" {
		t.Fatalf("8192 档没跑完（status=%v），顶档必须可跑", top)
	}
	if lowest == nil {
		t.Fatalf("没有任何一档跑完")
	}
	// 顶档必须满格：低档的丢失才有「从满格掉下来」的意义
	if top.M5Win != 1 || top.M5Rec != 1 {
		t.Fatalf("8192 档 MRR_win=%.4f MRR_rec=%.4f，要求都 1.000（顶档不满格则低档的下降无法归因）",
			top.M5Win, top.M5Rec)
	}
	for _, tr := range rep.Tiers {
		if tr.Status != "ok" {
			if tr.M5WinDenom != 0 || tr.AfterTotal != 0 || tr.M2Total != 0 || len(tr.Rows) != 0 {
				t.Errorf("Budget=%d status=%s 却带了读数（MRR 分母 %d / after %d）—— 不终止的档不许出数",
					tr.Budget, tr.Status, tr.M5WinDenom, tr.AfterTotal)
			}
			t.Logf("Budget=%-5d status=%s：%s", tr.Budget, tr.Status, tr.Note)
		}
	}

	if lowest.M5Win >= 1 {
		t.Logf("最低可跑档 %d：MRR_win=%.4f（%d/%d）—— **仍未丢**：free 到这一档还没把必留信息挤出窗口。",
			lowest.Budget, lowest.M5Win, lowest.M5WinHits, lowest.M5WinDenom)
		for _, tr := range rep.Tiers {
			if tr.Status != "ok" {
				t.Logf("  Budget=%-5d status=%s", tr.Budget, tr.Status)
				continue
			}
			t.Logf("  Budget=%-5d MRR_win=%.4f after=%d/%d", tr.Budget, tr.M5Win, tr.AfterTotal, tr.BeforeTotal)
		}
		return
	}

	// 真有丢失：逐条核对「丢的是未钉段里的 needle」
	checked := 0
	for _, row := range lowest.Rows {
		s, ok := byID[row.SampleID]
		if !ok {
			t.Fatalf("%s：扫描里出现了样本集外的 id", row.SampleID)
		}
		for _, nid := range row.LostWindow {
			var nd *Needle
			for k := range s.Needles {
				if s.Needles[k].ID == nid {
					nd = &s.Needles[k]
					break
				}
			}
			if nd == nil {
				t.Fatalf("%s/%s：丢失清单里的 needle 不在样本里", row.SampleID, nid)
			}
			if s.Segments[nd.Segment].Pinned {
				t.Errorf("%s/%s：钉住的段也丢了 —— 这是引擎 bug，不是预算问题", row.SampleID, nid)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatalf("MRR_win=%.4f < 1 但丢失清单为空（读数与明细不一致）", lowest.M5Win)
	}

	t.Logf("最低可跑档 %d：MRR_win=%.4f（%d/%d），丢失 %d 条（全部落在未钉段）；MRR_rec=%.4f",
		lowest.Budget, lowest.M5Win, lowest.M5WinHits, lowest.M5WinDenom, checked, lowest.M5Rec)
	for _, tr := range rep.Tiers {
		if tr.Status != "ok" {
			t.Logf("  Budget=%-5d status=%s", tr.Budget, tr.Status)
			continue
		}
		t.Logf("  Budget=%-5d MRR_win=%.4f MRR_rec=%.4f after=%d 丢失=%d after>budget=%d",
			tr.Budget, tr.M5Win, tr.M5Rec, tr.AfterTotal, len(tr.LostWindow), tr.AfterOver)
	}
	t.Logf("临界档（从大窗口往下第一个丢的档）：%d", rep.LossOnsetBudget)
}

// 真产物：只有显式给了 -scan-out 才写（绝不默认往 state 里写）。
//
// 全量 50 条 × 四档 = 主产物；每档 unload 原件留在 <base>.work/b<Budget>/unload/，
// 供 RVS 复看。
func TestScan_WriteArtifacts(t *testing.T) {
	if *scanOut == "" {
		t.Skip("未给 -scan-out，跳过产物落盘（默认不写 state）")
	}
	base := *scanOut
	absBase, err := filepath.Abs(base)
	if err != nil {
		t.Fatalf("取绝对路径：%v", err)
	}

	type job struct {
		base    string
		root    string
		samples []Sample
	}
	jobs := []job{
		{base: absBase, root: absBase + ".work", samples: AllSamples()},
	}
	for _, j := range jobs {
		if err := os.MkdirAll(j.root, 0o755); err != nil {
			t.Fatalf("建 work 目录：%v", err)
		}
		rep, err := RunScan(context.Background(), j.samples, ScanConfig{
			Engine: "free", WorkRoot: j.root, TierTimeout: 5 * time.Second,
		})
		if err != nil {
			t.Fatalf("RunScan(%s): %v", j.base, err)
		}
		if err := WriteScanArtifacts(j.base, rep); err != nil {
			t.Fatalf("写产物：%v", err)
		}

		raw, err := os.ReadFile(j.base + ".json")
		if err != nil {
			t.Fatalf("读回 json：%v", err)
		}
		var back ScanReport
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("json 解不开：%v", err)
		}
		if len(back.Tiers) != len(ScanBudgets) {
			t.Fatalf("回读档数 = %d，要求 %d", len(back.Tiers), len(ScanBudgets))
		}
		md, err := os.ReadFile(j.base + ".md")
		if err != nil {
			t.Fatalf("读回 md：%v", err)
		}
		if !containedAll(string(md), "四档主表", "丢失清单", "结论", "读法") {
			t.Errorf("md 缺段落：%s", headString(string(md), 200))
		}
		for _, b := range ScanBudgets {
			entries, err := os.ReadDir(filepath.Join(j.root, "b"+strconv.Itoa(b), "unload"))
			if err != nil || len(entries) == 0 {
				t.Errorf("Budget=%d 的 unload/ 没有原件：%v", b, err)
			}
		}
		for _, ext := range []string{".json", ".md"} {
			st, err := os.Stat(j.base + ext)
			if err != nil {
				t.Fatalf("产物缺失 %s：%v", j.base+ext, err)
			}
			t.Logf("产物 %s（%d 字节）", j.base+ext, st.Size())
		}
		t.Logf("[%s] n=%d LossOnsetBudget=%d IrrecoverableOnsetBudget=%d",
			filepath.Base(j.base), rep.N, rep.LossOnsetBudget, rep.IrrecoverableOnsetBudget)
		for _, tr := range rep.Tiers {
			if tr.Status != "ok" {
				t.Logf("  %-5d status=%s", tr.Budget, tr.Status)
				continue
			}
			t.Logf("  %-5d MRR_win=%.4f(%d/%d) MRR_rec=%.4f after=%d TPT均=%.1f LATp95=%.3fms 删除比中位=%.4f",
				tr.Budget, tr.M5Win, tr.M5WinHits, tr.M5WinDenom, tr.M5Rec, tr.AfterTotal,
				tr.M2Avg, tr.M3P95, tr.M4DeletedMedian)
		}
	}
}

func containedAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func headString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
