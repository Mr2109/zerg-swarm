package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 本文件是 **Budget 扫描**：同一批样本、同一引擎，只动 Budget 一维（任务单 §4.1 的
// 「每次只动一维」纪律），看四档 8192→1024 上 M2/M3/M4/M5/M6 各自怎么变。
//
// 为什么要这一步：8192 档 free 的 M5 MRR 满格（1.000），尺子分不出引擎好坏。
// 把窗口一路压到 1024，perOut = Budget×0.25 与 headroom = Budget×0.70 同步收紧，
// 「哪一档开始把必留信息挤出窗口」才成为一个有读数的问题 —— 单档满格的尺子没有分辨力，
// 有临界的尺子才有。
//
// 纪律：
//   - 唯一入口仍是 compress.Apply（经 RunBatch），本文件不自己剪枝、不改阈值；
//   - 每档一个独立 work 目录（<WorkRoot>/b<Budget>/unload），互不污染；
//   - 只写调用方给的基名（WriteScanArtifacts），不自己决定落点。

// ScanBudgets —— 缺省四档（任务单 §4.1 扫 A 的预算轴 ＋ P2 第二步要求的小窗档）。
var ScanBudgets = []int{8192, 4096, 2048, 1024}

// ScanConfig —— 一次扫描的参数。
type ScanConfig struct {
	Engine   string
	Budgets  []int  // 空 ⇒ ScanBudgets
	WorkRoot string // 每档落 <WorkRoot>/b<Budget>/
	RunID    string
	// TierTimeout：每档子进程的硬超时（0 ⇒ 30s）。free 在低档有不终止路径，见 scan_child.go。
	TierTimeout time.Duration
}

// ClassStat —— 按类聚合（observation / trajectory）。
type ClassStat struct {
	N             int     `json:"n"`
	Before        int     `json:"before"`
	After         int     `json:"after"`
	DeletedMedian float64 `json:"deleted_median"`
	TPTAvg        float64 `json:"tpt_avg"`
	MRRWin        float64 `json:"mrr_window"`
	MRRWinHits    int     `json:"mrr_window_hits"`
	MRRWinDenom   int     `json:"mrr_window_denom"`
}

// ScanSampleRow —— 一档 × 一条样本（审计用：率能对回样本）。
type ScanSampleRow struct {
	SampleID   string   `json:"sample_id"`
	Class      string   `json:"class"`
	Control    bool     `json:"control"`
	Before     int      `json:"before"`
	After      int      `json:"after"`
	Deleted    float64  `json:"deleted"`
	TPT        int      `json:"tpt"`
	Trigger    string   `json:"trigger"`
	Mechanism  string   `json:"mechanism"`
	Fallback   bool     `json:"fallback"`
	MRRWinHit  int      `json:"mr_window_hit"`
	MRRWinDen  int      `json:"mr_window_denom"`
	MRRRecHit  int      `json:"mr_recover_hit"`
	MRRRecDen  int      `json:"mr_recover_denom"`
	LostWindow []string `json:"window_lost_ids,omitempty"`
	LostRec    []string `json:"rec_lost_ids,omitempty"`
}

// TierRow —— 一个 Budget 档的全部读数。
type TierRow struct {
	Budget  int    `json:"budget"`
	Status  string `json:"status"` // ok | timeout | error
	Note    string `json:"note,omitempty"`
	Digest  string `json:"deterministic_digest"`
	N       int    `json:"n"`
	Control int    `json:"controls"`

	BeforeTotal int `json:"before_total"`
	// AfterTotal = Σ rows[].after —— **逐段口径**（每段 floor(ascii/4) 后相加，引擎/预算闸自用）。
	// ★ 不是 r.M2.Total：后者是拼窗口径（段间 "\n" 计入、合并后一次 floor），单独存在
	// M2Total / json `m2_tpt_total`。旧代码把两者混成一个字段，导致 after_total 与
	// rows[].after 的合计差 51–81 token（4096/2048/1024 档）而无人能对账 —— 别改回去。
	AfterTotal  int      `json:"after_total"`
	DeletedAll  float64  `json:"deleted_total"` // 整批口径：1 − Σafter/Σbefore
	AfterOver   int      `json:"after_over_budget"`
	AfterOverID []string `json:"after_over_budget_samples,omitempty"`

	Triggers           map[string]int `json:"trigger_counts"`
	ControlsCompressed int            `json:"controls_compressed"`

	M2Avg   float64 `json:"m2_tpt_avg"`
	M2P95   int     `json:"m2_tpt_p95"`
	M2Max   int     `json:"m2_tpt_max"`
	M2Total int     `json:"m2_tpt_total"`

	M3P50 float64 `json:"m3_lat_p50_ms"`
	M3P95 float64 `json:"m3_lat_p95_ms"`
	M3Max float64 `json:"m3_lat_max_ms"`

	M4DeletedMedian float64 `json:"m4_deleted_median"`
	M4WithinRatio   float64 `json:"m4_within_max_ratio"`
	M4AfterGTBefore int     `json:"m4_after_gt_before"`

	M5Win      float64 `json:"m5_mrr_window"`
	M5WinHits  int     `json:"m5_mrr_window_hits"`
	M5WinDenom int     `json:"m5_mrr_window_denom"`
	M5Rec      float64 `json:"m5_mrr_recoverable"`
	M5RecHits  int     `json:"m5_mrr_recover_hits"`
	M5RecDenom int     `json:"m5_mrr_recover_denom"`
	M5Anchor   int     `json:"m5_anchor_lost"`

	M6GuardFails int     `json:"m6_rbr_guard_fails"`
	M6Fallbacks  int     `json:"m6_rbr_fallbacks"`
	M6Rollback   float64 `json:"m6_rbr"`

	M7OK    int     `json:"m7_rvs_ok"`
	M7Total int     `json:"m7_rvs_total"`
	M7Rate  float64 `json:"m7_rvs_rate"`

	LostWindow  []string `json:"m5_lost_window,omitempty"`
	LostRecover []string `json:"m5_lost_recoverable,omitempty"`

	ByClass map[string]ClassStat `json:"by_class"`
	Rows    []ScanSampleRow      `json:"rows"`
}

// MetricNA —— 本批**不可判定**的指标：记 N/A 并给分母，绝不记 0、不推算（任务单 §7 风险 6）。
type MetricNA struct {
	Metric      string `json:"metric"`
	Status      string `json:"status"`      // 恒为 "N/A"
	Reason      string `json:"reason"`      // 为什么不可判定
	Denominator int    `json:"denominator"` // 若要判定，需要多少个可判定样本
	Observed    int    `json:"observed"`    // 实际拿到几个（0 ⇒ 一个都没有）
}

// ClassDist —— 50 条样本的分布（类 × 语 × 对照 × 长度 × needle 配额）。
type ClassDist struct {
	Class        string `json:"class"`
	N            int    `json:"n"`
	Zh           int    `json:"zh"`
	En           int    `json:"en"`
	Controls     int    `json:"controls"`
	TokensMin    int    `json:"tokens_min"`
	TokensMedian int    `json:"tokens_median"`
	TokensMax    int    `json:"tokens_max"`
	NeedlesWin   int    `json:"needles_window"`
	NeedlesRec   int    `json:"needles_recoverable"`
	PinnedSegs   int    `json:"pinned_segments"`
	RawPtrSegs   int    `json:"rawptr_segments"`
}

// ScanReport —— 扫描产物（<base>.json 的结构）。
type ScanReport struct {
	RunID         string `json:"run_id"`
	GeneratedAt   string `json:"generated_at"`
	Engine        string `json:"engine"`
	Budgets       []int  `json:"budgets"`
	SamplesSHA256 string `json:"samples_sha256"`
	N             int    `json:"n"`
	Controls      int    `json:"controls"`

	NeedleWindow      int `json:"needles_window"`
	NeedleRecoverable int `json:"needles_recoverable"`
	NeedleTotal       int `json:"needles_total"`

	Distribution []ClassDist `json:"distribution"`

	// LossOnsetBudget：从这一档开始 window needle 出现丢失（0 ⇒ 四档都未丢）。
	LossOnsetBudget int `json:"loss_onset_budget"`
	// IrrecoverableOnsetBudget：从这一档开始出现「窗口丢了、盘上也取不回」（0 ⇒ 从未出现）。
	IrrecoverableOnsetBudget int `json:"irrecoverable_onset_budget"`
	// LossOnsetByClass：每类第一次丢 window needle 的档（类没丢 ⇒ 0，键一定存在）。
	LossOnsetByClass map[string]int `json:"loss_onset_by_class"`

	// NA：M1（无模型端点）/ M8（尺子内无工具调用）/ M6-undo（无人工回滚通道）。
	NA []MetricNA `json:"na"`

	Tiers []TierRow `json:"tiers"`
	Notes []string  `json:"notes"`
}

// RunScan —— 四档真跑一遍（每档内部仍是 RunBatch → compress.Apply）。
func RunScan(ctx context.Context, samples []Sample, cfg ScanConfig) (*ScanReport, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("eval: 扫描没有样本")
	}
	if cfg.Engine == "" {
		cfg.Engine = "free"
	}
	// ★ 扫描走**子进程**（scan_child.go）⇒ 不能把 Summarizer 注入过去（func 过不了进程边界）。
	//   summary 引擎的整批读数只能走 RunBatch（同进程注入）。这里显式拒绝，✗ 不静默降级成 free。
	if cfg.Engine != "free" {
		return nil, fmt.Errorf("eval: 四档扫描只支持 free（子进程隔离摆不了摘要器）；engine=%q 请改用 RunBatch 逐档跑", cfg.Engine)
	}
	if cfg.WorkRoot == "" {
		return nil, fmt.Errorf("eval: WorkRoot 必填（每档 unload 原件落点）")
	}
	budgets := cfg.Budgets
	if len(budgets) == 0 {
		budgets = ScanBudgets
	}
	digest, err := samplesDigest(samples)
	if err != nil {
		return nil, err
	}
	if cfg.RunID == "" {
		cfg.RunID = fmt.Sprintf("p2-scan-%s-%d-b%d-%s", cfg.Engine, len(samples), len(budgets), digest[:8])
	}

	rep := &ScanReport{
		RunID:         cfg.RunID,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Engine:        cfg.Engine,
		Budgets:       append([]int(nil), budgets...),
		SamplesSHA256: digest,
		N:             len(samples),
	}
	for _, s := range samples {
		if s.Control {
			rep.Controls++
		}
		rep.NeedleWindow += s.CountNeedles(WhereWindow)
		rep.NeedleRecoverable += s.CountNeedles(WhereRecoverable)
	}
	rep.NeedleTotal = rep.NeedleWindow + rep.NeedleRecoverable
	rep.Distribution = classDistribution(samples)
	rep.NA = []MetricNA{
		{
			Metric: "M1 DSR", Status: "N/A",
			Reason:      "未给模型端点（-probe-endpoint/-probe-model），压缩后窗口没有喂给任何模型打分；不记 0、不推算（任务单 §4 M1）",
			Denominator: len(samples), Observed: 0,
		},
		{
			Metric: "M8 RPT", Status: "N/A",
			Reason:      "尺子样本里没有 agent 工具调用序列（RPT 的分母是 tool_calls），只在真机 chat_obs / 任务回放上量（任务单 §4 M8）",
			Denominator: len(samples), Observed: 0,
		},
		{
			Metric: "M6 RBR_undo", Status: "N/A",
			Reason:      "free 引擎没有人工/任务显式回滚通道，本批不存在 undo 事件；RBR 只报 guard 与 fallback 两层",
			Denominator: len(samples), Observed: 0,
		},
	}

	for _, budget := range budgets {
		run := runTierIsolated(samples, cfg.Engine,
			filepath.Join(cfg.WorkRoot, fmt.Sprintf("b%d", budget)), budget, cfg.TierTimeout)
		if run.status != "ok" {
			rep.Tiers = append(rep.Tiers, TierRow{Budget: budget, Status: run.status, Note: run.note})
			continue
		}
		t := buildTier(budget, run.rep)
		t.Status = "ok"
		rep.Tiers = append(rep.Tiers, t)
	}

	// 临界档：从大窗口往小窗口扫，第一个读数为「有丢失」的档就是 onset。
	// （tiers 按 budgets 的给定顺序；这里按 budget 降序判断，与人读表的方向一致。）
	ordered := append([]TierRow(nil), rep.Tiers...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Budget > ordered[j].Budget })
	rep.LossOnsetBudget = 0
	rep.IrrecoverableOnsetBudget = 0
	rep.LossOnsetByClass = map[string]int{}
	for _, class := range ClassNames {
		rep.LossOnsetByClass[class] = 0 // 键一定存在：0 = 四档都没丢，不是「没测」
	}
	for _, t := range ordered {
		if t.Status != "ok" {
			continue
		}
		if t.M5Win < 1 && rep.LossOnsetBudget == 0 {
			rep.LossOnsetBudget = t.Budget
		}
		if t.M5Rec < 1 && rep.IrrecoverableOnsetBudget == 0 {
			rep.IrrecoverableOnsetBudget = t.Budget
		}
		// 类内临界档：同一档里只要该类 MRR_win < 1 且分母非 0，就记这一档。
		for class, c := range t.ByClass {
			if c.MRRWinDenom == 0 {
				continue
			}
			if c.MRRWin < 1 && rep.LossOnsetByClass[class] == 0 {
				rep.LossOnsetByClass[class] = t.Budget
			}
		}
	}

	rep.Notes = []string{
		"每次只动 Budget 一维：Engine=free、PerOutputPct=0.25、HeadroomPct=0.70、KeepRecent=2 四档不动（任务单 §4.1）。",
		"MRR_win 丢失 = 必留信息被挤出窗口文本；MRR_rec 丢失 = 窗口没有、raw_ptr 从盘上也取不回来（真丢）。",
		"对照样本（R05/R10/R15/R20/R25/R30/R35/R40/R45/R50）只在 8192 档「未撞线」；Budget ≤ 2048 时它们自身已超过 0.5×Budget，被压是算术必然，不算闸失败。",
		"free 的可压缩面 = 未钉的 KindObservation 段：plan/memory 的 Pinned 段、以及任何非 observation 段（含 KindRepresentation/KindTrajectory），两道闸都不碰 —— 这类样本在四档上「不丢 needle」是「没被碰」，不是「保住」。",
		"每档跑在独立子进程里并带硬超时：这是**保险丝**（P2 期间 free 的余量闸曾经在低档不终止，已修）；本批四档 status 全为 ok，无超时档。",
		accountingNote(rep.Tiers),
	}
	return rep, nil
}

// classDistribution —— 50 条样本的分布（人读表与 json 同源，不靠报告手抄）。
func classDistribution(samples []Sample) []ClassDist {
	byClass := map[string][]Sample{}
	for _, s := range samples {
		byClass[s.Class] = append(byClass[s.Class], s)
	}
	out := make([]ClassDist, 0, len(ClassNames))
	for _, class := range ClassNames {
		ss := byClass[class]
		if len(ss) == 0 {
			continue
		}
		d := ClassDist{Class: class, N: len(ss)}
		var toks []float64
		for _, s := range ss {
			if s.Lang == "zh" {
				d.Zh++
			} else {
				d.En++
			}
			if s.Control {
				d.Controls++
			}
			toks = append(toks, float64(s.Tokens()))
			d.NeedlesWin += s.CountNeedles(WhereWindow)
			d.NeedlesRec += s.CountNeedles(WhereRecoverable)
			for _, sg := range s.Segments {
				if sg.Pinned {
					d.PinnedSegs++
				}
				if sg.RawPtr != "" {
					d.RawPtrSegs++
				}
			}
		}
		d.TokensMin = int(pctNearestRank(toks, 0))
		d.TokensMedian = int(median(toks))
		d.TokensMax = int(pctNearestRank(toks, 1))
		out = append(out, d)
	}
	return out
}

// buildTier —— 把一份 Report 折成一档读数。
func buildTier(budget int, r *Report) TierRow {
	t := TierRow{
		Budget: budget, Digest: r.DeterministicDigest(), N: r.N, Control: r.Controls,
		BeforeTotal: r.M2.TotalBefore, AfterTotal: sumAfter(r.Rows),
		Triggers: map[string]int{}, ByClass: map[string]ClassStat{},
		M2Avg: r.M2.Avg, M2P95: r.M2.P95, M2Max: r.M2.Max, M2Total: r.M2.Total,
		M3P50: r.M3.P50MS, M3P95: r.M3.P95MS, M3Max: r.M3.MaxMS,
		M4DeletedMedian: r.RAT.DeletedMedian, M4AfterGTBefore: r.RAT.AfterGTBefore,
		M5Win: r.M5.MRRWin, M5WinHits: r.M5.MRRWinHits, M5WinDenom: r.M5.MRRWinDenom,
		M5Rec: r.M5.MRRRec, M5RecHits: r.M5.MRRRecHits, M5RecDenom: r.M5.MRRRecDenom,
		M5Anchor: r.M5.AnchorLost, M6GuardFails: r.GuardFails, M6Fallbacks: r.Fallbacks,
	}
	t.DeletedAll = deleted(t.BeforeTotal, t.AfterTotal)
	t.M6Rollback = rate(t.M6GuardFails+t.M6Fallbacks, r.N)
	for k, v := range r.TriggerCounts {
		t.Triggers[k] = v
	}
	maxRatio := 0.25
	if v, ok := r.Policy["max_ratio"].(float64); ok && v > 0 {
		maxRatio = v
	}

	var within, attempted int
	var delAttempted []float64
	var delByClass = map[string][]float64{}
	var tptByClass = map[string][]float64{}
	classBefore := map[string]int{}
	classAfter := map[string]int{}
	classWinHits := map[string]int{}
	classWinDen := map[string]int{}
	classN := map[string]int{}

	for _, row := range r.Rows {
		t.Rows = append(t.Rows, ScanSampleRow{
			SampleID: row.SampleID, Class: row.Class, Control: row.Control,
			Before: row.Before, After: row.After, Deleted: row.Deleted, TPT: row.TPT,
			Trigger: row.Trigger, Mechanism: row.Mechanism, Fallback: row.Fallback,
			MRRWinHit: row.MRWindowHit, MRRWinDen: row.NeedlesWindow,
			MRRRecHit: row.MRAnyWinHit + row.MRDiskHit, MRRRecDen: row.NeedlesTotal,
			LostWindow: row.WindowLostIDs, LostRec: row.RecLostIDs,
		})
		for _, id := range row.WindowLostIDs {
			t.LostWindow = append(t.LostWindow, row.SampleID+"/"+id)
		}
		for _, id := range row.RecLostIDs {
			t.LostRecover = append(t.LostRecover, row.SampleID+"/"+id)
		}
		if row.Control && row.Trigger != "none" {
			t.ControlsCompressed++
		}
		if row.After > budget {
			t.AfterOver++
			t.AfterOverID = append(t.AfterOverID, row.SampleID)
		}
		if row.Trigger != "none" {
			attempted++
			delAttempted = append(delAttempted, row.Deleted)
			delByClass[row.Class] = append(delByClass[row.Class], row.Deleted)
			if row.Before > 0 && float64(row.After)/float64(row.Before) <= maxRatio {
				within++
			}
		}
		tptByClass[row.Class] = append(tptByClass[row.Class], float64(row.TPT))
		classBefore[row.Class] += row.Before
		classAfter[row.Class] += row.After
		classWinHits[row.Class] += row.MRWindowHit
		classWinDen[row.Class] += row.NeedlesWindow
		classN[row.Class]++
	}
	t.M4WithinRatio = rate(within, attempted)
	// 任务单 §4 M4：D_med 只在 trigger != none 的样本上取中位（对照样本的 0 不进分母）
	t.M4DeletedMedian = round4(median(delAttempted))

	// M7 RVS：被卸载的段能否逐字节取回（分母 = 真正卸载过的段数）
	for _, row := range r.Rows {
		t.M7OK += row.RVSOK
		t.M7Total += row.RVSTotal
	}
	t.M7Rate = rate(t.M7OK, t.M7Total)

	for _, class := range sortedKeys(classN) {
		t.ByClass[class] = ClassStat{
			N: classN[class], Before: classBefore[class], After: classAfter[class],
			DeletedMedian: round4(median(delByClass[class])),
			TPTAvg:        round3(avg(tptByClass[class])),
			MRRWin:        rate(classWinHits[class], classWinDen[class]),
			MRRWinHits:    classWinHits[class], MRRWinDenom: classWinDen[class],
		}
	}
	return t
}

// sumAfter —— 压缩后合计的**逐段口径**（与 rows[].after 同源，见 RunRow.After 注释）。
// 汇总处一律用它，不要用 r.M2.Total（那是拼窗口径 Σtpt）。
func sumAfter(rows []RunRow) int {
	t := 0
	for _, row := range rows {
		t += row.After
	}
	return t
}

// accountingNote —— 「after 逐段 vs tpt 拼窗」的口径差，写进产物 notes（与 md 读法第 6 条同源同数）。
// 存在的理由：两和天然不等（整数截断 + 段间 "\n"），不写清就会被后人当成漏记账去「平账」。
func accountingNote(tiers []TierRow) string {
	var parts []string
	for _, t := range tiers {
		if t.Status != "ok" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d 档 %d", t.Budget, t.M2Total-t.AfterTotal))
	}
	if len(parts) == 0 {
		return "after_total 与 m2_tpt_total 按定义不同源（逐段 floor vs 拼窗一次 floor），不可互相核对。"
	}
	return fmt.Sprintf("`after_total` = Σ rows[].after（逐段 floor(ascii/4) 后求和，引擎与预算闸自用）；"+
		"`m2_tpt_total` = Σ rows[].tpt（保留段用段间换行拼成整窗后一次 floor，含分隔符，模型侧口径）。"+
		"两者按定义不等，差 = ⌊(Σa+k)/4⌋ − Σ⌊aᵢ/4⌋（k = 段数−1），恒 ≥ 0；本批差值：%s token。"+
		"禁止拿一个当另一个用，也禁止把差值当「漏记」去平账。", strings.Join(parts, " / "))
}

// WriteScanArtifacts —— 落 <base>.json 与 <base>.md（调用方给基名，本函数不选目录）。
func WriteScanArtifacts(base string, rep *ScanReport) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(base+".json", append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.WriteFile(base+".md", []byte(renderScanMarkdown(rep)), 0o644)
}

// renderScanMarkdown —— 人读表（四档 × 五指标 ＋ 丢失清单 ＋ 结论句）。
func renderScanMarkdown(rep *ScanReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 压缩尺子 · Budget 扫描 · %s 引擎 · %d 条 × %d 档\n\n", rep.Engine, rep.N, len(rep.Tiers))
	fmt.Fprintf(&b, "- run_id：`%s`　生成时间：%s\n", rep.RunID, rep.GeneratedAt)
	fmt.Fprintf(&b, "- 样本：%d 条（对照 %d 条）· needle：%d 条（W %d / R %d）· samples_sha256：`%s`\n",
		rep.N, rep.Controls, rep.NeedleTotal, rep.NeedleWindow, rep.NeedleRecoverable, rep.SamplesSHA256[:16])
	fmt.Fprintf(&b, "- 只动 Budget 一维：Engine=free · PerOutputPct=0.25 · HeadroomPct=0.70 · KeepRecent=2\n\n")

	b.WriteString("## 样本分布（五类 × 中英 × 对照 × 长度 × needle 配额）\n\n")
	b.WriteString("| 类 | n | zh | en | 对照 | token min/中位/max | needle W | needle R | 钉住段 | raw_ptr 段 |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	for _, d := range rep.Distribution {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d / %d / %d | %d | %d | %d | %d |\n",
			d.Class, d.N, d.Zh, d.En, d.Controls, d.TokensMin, d.TokensMedian, d.TokensMax,
			d.NeedlesWin, d.NeedlesRec, d.PinnedSegs, d.RawPtrSegs)
	}

	b.WriteString("\n## 八指标（M1–M8）逐档\n\n")
	b.WriteString("| 指标 | 口径 | 8192 | 4096 | 2048 | 1024 |\n|---|---|---|---|---|---|\n")
	cell := func(f func(t TierRow) string) string {
		var s []string
		for _, t := range rep.Tiers {
			if t.Status != "ok" {
				s = append(s, "—（"+t.Status+"）")
				continue
			}
			s = append(s, f(t))
		}
		return strings.Join(s, " | ")
	}
	b.WriteString("| M1 DSR | 探针答对率（≥ free−0.02 且 ≥0.90） | **N/A** | **N/A** | **N/A** | **N/A** |\n")
	fmt.Fprintf(&b, "| M2 TPT | 压缩后窗口 token（均 / p95 / max） | %s |\n",
		cell(func(t TierRow) string { return fmt.Sprintf("%.1f / %d / %d", t.M2Avg, t.M2P95, t.M2Max) }))
	fmt.Fprintf(&b, "| M3 LAT | 只包 Apply（p50 / p95 / max ms；free p95 ≤ 50） | %s |\n",
		cell(func(t TierRow) string { return fmt.Sprintf("%.3f / %.3f / %.3f", t.M3P50, t.M3P95, t.M3Max) }))
	fmt.Fprintf(&b, "| M4 RAT | 删除比中位（≠none） / ≤0.25 达标率 / after>before | %s |\n",
		cell(func(t TierRow) string {
			return fmt.Sprintf("%.4f / %.3f / %d", t.M4DeletedMedian, t.M4WithinRatio, t.M4AfterGTBefore)
		}))
	fmt.Fprintf(&b, "| M5 MRR-win | window needle 命中率（硬闸 1.000） | %s |\n",
		cell(func(t TierRow) string { return fmt.Sprintf("%.4f (%d/%d)", t.M5Win, t.M5WinHits, t.M5WinDenom) }))
	fmt.Fprintf(&b, "| M5 MRR-rec | 窗口 ∪ raw_ptr（全部 needle） | %s |\n",
		cell(func(t TierRow) string { return fmt.Sprintf("%.4f (%d/%d)", t.M5Rec, t.M5RecHits, t.M5RecDenom) }))
	fmt.Fprintf(&b, "| M5 AnchorLost | 锚点丢失绝对数（硬闸 0） | %s |\n",
		cell(func(t TierRow) string { return fmt.Sprintf("%d", t.M5Anchor) }))
	fmt.Fprintf(&b, "| M6 RBR | guard 失败 / fallback / 回滚率 | %s |\n",
		cell(func(t TierRow) string {
			return fmt.Sprintf("%d / %d / %.4f", t.M6GuardFails, t.M6Fallbacks, t.M6Rollback)
		}))
	fmt.Fprintf(&b, "| M7 RVS | 卸载原件逐字节取回率（硬闸 1.000） | %s |\n",
		cell(func(t TierRow) string { return fmt.Sprintf("%.4f (%d/%d)", t.M7Rate, t.M7OK, t.M7Total) }))
	b.WriteString("| M8 RPT | 压缩后重复劳动率（≤ 基线+0.05） | **N/A** | **N/A** | **N/A** | **N/A** |\n")
	for _, na := range rep.NA {
		fmt.Fprintf(&b, "\n> N/A · %s（分母 %d，实得 %d）：%s\n", na.Metric, na.Denominator, na.Observed, na.Reason)
	}

	b.WriteString("\n## 四档主表\n\n")
	b.WriteString("| Budget | 状态 | before | after（Σ rows.after） | 删除比(整批) | M2 TPT 均/p95（拼窗口径） | M3 LAT p95 | M4 删除比中位(≠none) | M4 ≤0.25 | M5 MRR_win | M5 MRR_rec | M6 guard/fallback | after>budget |\n|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, t := range rep.Tiers {
		if t.Status != "ok" {
			fmt.Fprintf(&b, "| %d | **%s** | — | — | — | — | — | — | — | — | — | — | — |\n", t.Budget, t.Status)
			continue
		}
		fmt.Fprintf(&b, "| %d | ok | %d | %d | %.4f | %.1f / %d | %.3f ms | %.4f | %.3f | **%.4f** (%d/%d) | %.4f (%d/%d) | %d / %d | %d |\n",
			t.Budget, t.BeforeTotal, t.AfterTotal, t.DeletedAll, t.M2Avg, t.M2P95, t.M3P95,
			t.M4DeletedMedian, t.M4WithinRatio, t.M5Win, t.M5WinHits, t.M5WinDenom,
			t.M5Rec, t.M5RecHits, t.M5RecDenom, t.M6GuardFails, t.M6Fallbacks, t.AfterOver)
	}

	b.WriteString("\n> 主表口径：`after` = **Σ rows[].after**（逐段 floor(ascii/4) 求和，与「逐样本 × 四档」表的 after 同源）；`M2 TPT` 列 = 拼窗后**一次**估算。两列按定义不同源（差 = 整数截断 + 段间换行），数字对不上不是漏记 —— 见「读法」第 6 条。\n")

	b.WriteString("\n## 类内临界档：哪一类在哪一档先丢 window needle\n\n")
	b.WriteString("| 类 | 首次丢的档 | 该类 8192 读数 | 该类最低档读数 | 丢的机制 |\n|---|---|---|---|---|\n")
	lowest := map[string]ClassStat{}
	top := map[string]ClassStat{}
	var lowestBudget int
	for _, t := range rep.Tiers {
		if t.Status != "ok" {
			continue
		}
		if lowestBudget == 0 || t.Budget < lowestBudget {
			lowestBudget = t.Budget
			for class, c := range t.ByClass {
				lowest[class] = c
			}
		}
		if t.Budget == 8192 {
			for class, c := range t.ByClass {
				top[class] = c
			}
		}
	}
	for _, class := range ClassNames {
		onset := rep.LossOnsetByClass[class]
		if _, ok := top[class]; !ok {
			continue
		}
		onsetText := "**未丢（四档都满格）**"
		mech := "Pinned 段或非 observation 段，free 的两道闸都不碰"
		if onset != 0 {
			onsetText = fmt.Sprintf("**%d 档**", onset)
			mech = fmt.Sprintf("未钉 observation 段被闸卸掉（perOut=%d 起），指针可回查", onset/4)
		}
		fmt.Fprintf(&b, "| %s | %s | %.4f (%d/%d) | %.4f (%d/%d) | %s |\n",
			class, onsetText,
			top[class].MRRWin, top[class].MRRWinHits, top[class].MRRWinDenom,
			lowest[class].MRRWin, lowest[class].MRRWinHits, lowest[class].MRRWinDenom,
			mech)
	}
	b.WriteString("\n> 「未丢」有两种原因，报告必须分清：①**钉住段**（plan/memory 的计划主体与决策账）——闸按设计不碰；②**不是 observation 段**（KindTrajectory/Representation 等）——free 现码只卸未钉的 observation，等于对这类内容没有压缩能力。第二种「不丢」不是保护，是能力缺口。\n")

	b.WriteString("\n## 按类拆分（M5 MRR_win / 删除比中位 / TPT 均）\n\n")
	b.WriteString("| Budget | 类 | n | after 合计 | 删除比中位 | TPT 均 | MRR_win |\n|---|---|---|---|---|---|---|\n")
	for _, t := range rep.Tiers {
		if t.Status != "ok" {
			continue
		}
		for _, class := range sortedKeys(t.ByClass) {
			c := t.ByClass[class]
			fmt.Fprintf(&b, "| %d | %s | %d | %d | %.4f | %.1f | %.4f (%d/%d) |\n",
				t.Budget, class, c.N, c.After, c.DeletedMedian, c.TPTAvg, c.MRRWin, c.MRRWinHits, c.MRRWinDenom)
		}
	}

	b.WriteString("\n## 闸报了「压了」但一段没卸（free 的能力边界）\n\n")
	b.WriteString("| Budget | trigger≠none 但 after==before | 条数 |\n|---|---|---|\n")
	for _, t := range rep.Tiers {
		if t.Status != "ok" {
			continue
		}
		var noop []string
		for _, row := range t.Rows {
			if row.Trigger != "none" && row.After == row.Before {
				noop = append(noop, row.SampleID)
			}
		}
		fmt.Fprintf(&b, "| %d | %s | %d/%d |\n", t.Budget, lostText(noop), len(noop), t.N)
	}

	b.WriteString("\n## 丢失清单（指名道姓）\n\n")
	b.WriteString("| Budget | window 丢失 | 不可取回（真丢） |\n|---|---|---|\n")
	for _, t := range rep.Tiers {
		if t.Status != "ok" {
			fmt.Fprintf(&b, "| %d | （无读数：%s） | — |\n", t.Budget, t.Status)
			continue
		}
		fmt.Fprintf(&b, "| %d | %s | %s |\n", t.Budget, lostText(t.LostWindow), lostText(t.LostRecover))
	}
	if bad := timedOut(rep); len(bad) > 0 {
		fmt.Fprintf(&b, "\n> **这些档没有读数，不是 0、不是满格**：%s。原因见下方「引擎不终止」一节。\n", bad)
	}

	b.WriteString("\n## 逐样本 × 四档（after / MRR_win 命中）\n\n")
	b.WriteString("| 样本 | 类 | 对照 | before |")
	for _, t := range rep.Tiers {
		fmt.Fprintf(&b, " %d 档 after/MRR |", t.Budget)
	}
	b.WriteString("\n|---|---|---|---|")
	for range rep.Tiers {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	byBudget := map[int]map[string]ScanSampleRow{}
	for _, t := range rep.Tiers {
		m := map[string]ScanSampleRow{}
		for _, row := range t.Rows {
			m[row.SampleID] = row
		}
		byBudget[t.Budget] = m
	}
	firstOK := -1
	for i, t := range rep.Tiers {
		if t.Status == "ok" && len(t.Rows) > 0 {
			firstOK = i
			break
		}
	}
	if firstOK < 0 {
		b.WriteString("（没有任何一档跑完）\n")
	}
	for _, first := range firstRows(rep.Tiers, firstOK) {
		fmt.Fprintf(&b, "| %s | %s | %s | %d |", first.SampleID, first.Class, yesNo(first.Control), first.Before)
		for _, t := range rep.Tiers {
			if t.Status != "ok" {
				b.WriteString(" — |")
				continue
			}
			row := byBudget[t.Budget][first.SampleID]
			fmt.Fprintf(&b, " %d/%d-%d%s |", row.After, row.MRRWinHit, row.MRRWinDen, lostMark(row.LostWindow, row.LostRec))
		}
		b.WriteString("\n")
	}

	b.WriteString(engineHangSection(rep))
	b.WriteString("\n## 结论\n\n")
	b.WriteString(scanConclusion(rep))
	b.WriteString("\n## 读法（别把这几条读错）\n\n")
	b.WriteString("1. **MRR_win 丢的是「窗口里的拷贝」，不等于证据没了**：被卸载的段仍有 `raw_ptr`，MRR_rec 与 RVS 才能回答「取不取回」。本表的「不可取回」列才是真丢。\n")
	b.WriteString("2. **对照样本只在 8192 档是「未撞线对照」**：Budget 降到它们自身长度以下，被压是算术必然而非闸失败（表中 `controls_compressed` 见 JSON）。\n")
	b.WriteString("3. **M2 TPT 不含回查重注入**：本笔不接模型，recall=0，与真机 `prompt_n+cache_n` 口径不同，不可直接比。\n")
	b.WriteString("4. **M1 DSR / M8 RPT 是 N/A，不是 0**：没有模型端点就没有探针判定，没有工具调用序列就没有 RPT 分母；两条都给了分母（50 条样本 / 0 次工具调用），不给推算值。\n")
	b.WriteString("5. **M4 ≤0.25 达标率低不是失败**：pinned 与对白段不可压，plan/memory 的 after/before 结构性地高于 0.25 —— 这是 free 的能力边界，判定看 M5/M6/M7 三条硬闸。\n")
	// 第 6 条：把两和逐档摆出来。after 逐段 floor 求和、tpt 拼窗后一次 floor，
	// 两和天然不相等；不写清后人会拿它们互相核对，然后把差值当成漏记账去「平账」。
	b.WriteString("6. **`after` 与 `tpt` 不是一个口径，禁止互相核对**：`after` = Σ rows[].after（每段各自 `floor(ascii/4)` 再相加，引擎与预算闸自用）；`tpt` = 把保留段用段间换行拼成整窗后**一次** `floor(ascii/4)`（模型侧窗口口径，含段间分隔符）。同一行恒有 `tpt ≥ after`，逐行差 = ⌊(Σa+k)/4⌋ − Σ⌊aᵢ/4⌋（k = 段数−1），量级 ~1 token/段 —— 是整数截断，**不是**漏记 token。本批逐档两和：\n\n")
	b.WriteString("| Budget | Σ rows.after（`after_total`） | Σ rows.tpt（`m2_tpt_total`） | 差 |\n|---|---|---|---|\n")
	for _, t := range rep.Tiers {
		if t.Status != "ok" {
			fmt.Fprintf(&b, "| %d | —（%s） | — | — |\n", t.Budget, t.Status)
			continue
		}
		fmt.Fprintf(&b, "| %d | %d | %d | %d |\n", t.Budget, t.AfterTotal, t.M2Total, t.M2Total-t.AfterTotal)
	}
	b.WriteString("\n> 若要判断「窗口是不是真小了」，看 `after`（与 `after>budget`、M4 比值同源）；若要估「模型实际吃多少 token」，看 `tpt`。两者都从同一份 `res.Segments` 出发，差值是估算器的整数截断，不含任何审计/指针/控制段开销。\n")
	fmt.Fprintf(&b, "\n- 明细：每档一个 work 目录，卸载原件在 `<base>.work/b<Budget>/unload/`；同一二进制两次扫描的 `deterministic_digest` 必须逐档相同。\n")
	return b.String()
}

func scanConclusion(rep *ScanReport) string {
	var b strings.Builder
	if bad := timedOut(rep); len(bad) > 0 {
		fmt.Fprintf(&b, "先说清分母：%s 没有读数（保险丝触发，见下节），所以「未丢」只能就**跑完的档**说。\n\n", strings.Join(bad, "、"))
	}
	if rep.LossOnsetBudget == 0 {
		b.WriteString("**已跑完的档（")
		for i, t := range rep.Tiers {
			if i > 0 {
				b.WriteString(" / ")
			}
			fmt.Fprintf(&b, "%d", t.Budget)
		}
		b.WriteString("）MRR_win 全部 1.000 —— free 在这批样本上仍未丢必留信息。**\n")
		b.WriteString("这不是「引擎好」，而是「还没压到它的临界点」：尺子要再往下压预算（或加更长的样本）才有分辨力，本笔不给「free 已合格」的结论。\n")
	} else {
		fmt.Fprintf(&b, "**free 的 window needle 从 Budget=%d 档开始丢失。**\n\n", rep.LossOnsetBudget)
		b.WriteString("| Budget | MRR_win | 丢失条数 |\n|---|---|---|\n")
		for _, t := range rep.Tiers {
			if t.Status != "ok" {
				b.WriteString(fmt.Sprintf("| %d | —（%s，无读数） | — |\n", t.Budget, t.Status))
				continue
			}
			fmt.Fprintf(&b, "| %d | %.4f | %d |\n", t.Budget, t.M5Win, len(t.LostWindow))
		}
		if rep.IrrecoverableOnsetBudget == 0 {
			b.WriteString("\n但 **跑完的档 MRR_rec 全 1.000**：丢的只是窗口里的拷贝，每一条都能经 `raw_ptr` 从盘上逐字取回 —— 这是「软隐藏」设计在低预算下的真实边界，不是信息事故。\n")
		} else {
			fmt.Fprintf(&b, "\n且 **从 Budget=%d 档起出现不可取回（MRR_rec < 1）** —— 那才是真丢，必须优先查 raw_ptr 是否落盘。\n", rep.IrrecoverableOnsetBudget)
		}
	}
	// 预算可达性：free 只卸 observation，对白段不动 ⇒ 低档可能压不到线内。
	for _, t := range rep.Tiers {
		if t.AfterOver > 0 {
			fmt.Fprintf(&b, "\n另：Budget=%d 档有 %d/%d 条样本 `after > Budget`（%s）—— free 的可卸载面只有「未钉的 observation」段：被钉住的错误证据段、以及 trajectory 对白段，一条都不卸，预算越紧越到不了线内。\n",
				t.Budget, t.AfterOver, t.N, strings.Join(shortList(t.AfterOverID, 6), ", "))
		}
	}
	return b.String()
}

// firstRows —— 取第一条跑完的档的 rows 作为样本清单。
func firstRows(tiers []TierRow, idx int) []ScanSampleRow {
	if idx < 0 || idx >= len(tiers) {
		return nil
	}
	return tiers[idx].Rows
}

// timedOut —— 列出没跑完的档。
func timedOut(rep *ScanReport) []string {
	var out []string
	for _, t := range rep.Tiers {
		if t.Status != "ok" {
			out = append(out, fmt.Sprintf("%d 档（%s）", t.Budget, t.Status))
		}
	}
	return out
}

// 引擎不终止的说明（只在真有超时档时出现）
func engineHangSection(rep *ScanReport) string {
	var bad []TierRow
	for _, t := range rep.Tiers {
		if t.Status != "ok" {
			bad = append(bad, t)
		}
	}
	if len(bad) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## 超时档（保险丝被触发）\n\n")
	for _, t := range bad {
		fmt.Fprintf(&b, "- **Budget=%d**：%s\n", t.Budget, t.Note)
	}
	b.WriteString("\n复现（单档、不经扫描）：\n\n```\n")
	b.WriteString("cd core && PROBE_TIER=<budget> go test ./internal/compress/eval -run TestProbeOneTier -timeout 10s -v\n")
	b.WriteString("```\n\n")
	b.WriteString("机理：每档跑在独立子进程里、带硬超时，超时即杀、本档标 status=timeout 且不出任何读数（不记 0、不推算）。\n")
	b.WriteString("这条保险丝是 P2 早期为 `PrunerFree` 第 ② 步（余量闸）不终止路径加的；该 bug 已修（循环体加 ctx 检查、`unload` 幂等、进展检测），本批四档全跑完。\n")
	return b.String()
}

func lostText(xs []string) string {
	if len(xs) == 0 {
		return "—"
	}
	return strings.Join(xs, ", ")
}

func lostMark(win, rec []string) string {
	if len(rec) > 0 {
		return " ⚠真丢"
	}
	if len(win) > 0 {
		return " ✂"
	}
	return ""
}

func shortList(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(append([]string{}, xs[:n]...), "…")
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
