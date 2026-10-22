package eval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// 跑批器：把样本喂给 compress.Apply，再用 score.go 的算式算 M2/M3/M5。
//
// 纪律（任务单 §2.4）：
//   - 唯一入口是 compress.Apply，本文件不自己剪枝、不改引擎阈值；
//   - 样本原文落盘到 <work>/unload/<sha16>.txt（内容寻址，与 ctx_unload.go 同规），
//     RVS 拿盘上的字节比原文——不拿内存里的字符串自证；
//   - 结果只写调用方给的基名，绝不写 ~/.zerg/state 真身（ZERG_STATE_DIR 由人给）。

// Config —— 一次跑批的全部参数。
type Config struct {
	Budget  int    // 目标窗口（token）；缺省 8192
	Engine  string // free（缺省）/ summary（须同时给 Summarizer，否则拒跑而不是编读数）
	WorkDir string // unload/ 与 samples.json 的落点
	OutBase string // 报告基名（无扩展名）：<OutBase>.json / .md / .jsonl
	Only    []string
	RunID   string
	// Summarizer —— summary 引擎的注入面（P3 整批跑）：nil 且 Engine=summary ⇒ 直接报错，
	// **不降级**（降级会静默产出 free 的读数、再贴上 summary 的名 —— 那就是编读数）。
	// 本包不读 env、不建 HTTP 客户端：调用方拿到端点后自己造，注入进来。
	Summarizer compress.SummarizerFunc
	// SummaryMaxSegs/MaxTokens 只用于报告头（让人看得见这批摘要器拿到的预算）。
	SummaryMaxTokens int
	// RotPctOverride > 0 时覆盖 Policy.RotPct —— 只给「对照样本断言不是空的」负控用。
	RotPctOverride float64
	// PerOutputOverride > 0 时覆盖 Policy.PerOutputPct —— 同上，只给负控用。
	PerOutputOverride float64
}

// RunRow —— 一条样本 × 一个引擎的读数（字段对齐任务单 §2.5，只留本笔要用的）。
type RunRow struct {
	SampleID string `json:"sample_id"`
	Class    string `json:"class"`
	Lang     string `json:"lang"`
	Control  bool   `json:"control"`
	Engine   string `json:"engine"`
	Budget   int    `json:"budget"`
	Before   int    `json:"before"`
	// After —— 压缩后窗口的 **逐段** token 和 = compress.TotalTokens(res.Segments)
	// （policy.go:195：每段各自 EstimateTokens ⇒ floor(ascii/4) 后相加）。
	// 这是引擎与预算闸（after>budget、M4 比值）自用的口径。
	// ★ 与 TPT 不是一个口径：TPT 把段拼成整窗后**一次**估算，还含段间 "\n"，
	// 所以同一行恒有 TPT ≥ After，差 1–5 token 是整数截断，不是漏记。详见 TPT 注释。
	After      int     `json:"after"`
	Ratio      float64 `json:"ratio"`
	Deleted    float64 `json:"deleted"`
	LatencyMS  float64 `json:"latency_ms"`
	Trigger    string  `json:"trigger"`
	Mechanism  string  `json:"mechanism"`
	Aggressive string  `json:"aggressive"`
	Fallback   bool    `json:"fallback"`
	Reason     string  `json:"reason"`

	GuardOK  bool   `json:"guard_ok"`
	GuardErr string `json:"guard_err,omitempty"`

	NeedlesTotal       int `json:"needles_total"`
	NeedlesWindow      int `json:"needles_window"`
	MRWindowHit        int `json:"mr_window_hit"`
	NeedlesRecoverable int `json:"needles_recoverable"`
	MRRecoverHit       int `json:"mr_recover_hit"`
	// MRAnyWinHit / MRDiskHit：M5 的 rec 口径 = 「窗口 ∨ 盘上」，对**全部** needle 生效
	// （不是只看 recoverable 类）。前者是两类 needle 落在窗口里的总数，后者是从盘上取回的。
	MRAnyWinHit int `json:"mr_any_window_hit"`
	MRDiskHit   int `json:"mr_disk_hit"`

	// 丢的是哪几条（Budget 扫描要指名道姓，不能只报一个率）
	WindowLostIDs []string `json:"window_lost_ids,omitempty"` // 被挤出窗口文本
	RecLostIDs    []string `json:"rec_lost_ids,omitempty"`    // 窗口与盘上都取不回（真丢）

	AnchorsTotal int `json:"anchors_total"`
	AnchorsLost  int `json:"anchors_lost"`

	UnloadedSegments int `json:"unloaded_segments"`
	RVSOK            int `json:"rvs_ok"`
	RVSTotal         int `json:"rvs_total"`
	RVSBad           int `json:"rvs_bad"`

	// TPT —— M2 口径 = EstimateTokens(joinTexts(Segments)) + RecallTokens（runner.go:370）。
	// 即把保留段用 "\n" 拼成整窗后**一次** floor(ascii/4)，是「模型实际看到的窗口」
	// 口径，含段间分隔符。★ 与 After 不同源：After 是逐段 floor 再求和，TPT 是合并后
	// floor。逐行差 = ⌊(Σa+k)/4⌋ − Σ⌊aᵢ/4⌋（k = 段数−1 个 "\n" 的 ASCII 字符），
	// 恒 ≥ 0、量级 ~1 token/段 —— 是整数截断，不是未记账的 token。
	// 汇总时两份各存一份：TierRow.AfterTotal（=Σ after）与 TierRow.M2Total（=Σ tpt）。
	TPT          int `json:"tpt"`
	RecallTokens int `json:"recall_tokens"`
}

// TPTBlock —— M2（任务单 §4 M2）。
type TPTBlock struct {
	Total       int     `json:"total"`
	Avg         float64 `json:"avg"`
	P95         int     `json:"p95"`
	Min         int     `json:"min"`
	Max         int     `json:"max"`
	TotalBefore int     `json:"total_before"`
	RecallTotal int     `json:"recall_total"`
	Note        string  `json:"note"`
}

// LATBlock —— M3（任务单 §4 M3：p50/p95/max，p95 用最近秩）。
type LATBlock struct {
	N     int     `json:"n"`
	P50MS float64 `json:"p50_ms"`
	P95MS float64 `json:"p95_ms"`
	MaxMS float64 `json:"max_ms"`
}

// MRRBlock —— M5（任务单 §4 M5）。
type MRRBlock struct {
	MRRWin      float64 `json:"mrr_window"`
	MRRWinHits  int     `json:"mrr_window_hits"`
	MRRWinDenom int     `json:"mrr_window_denom"`
	MRRRec      float64 `json:"mrr_recoverable"`
	MRRRecHits  int     `json:"mrr_recover_hits"`
	MRRRecDenom int     `json:"mrr_recover_denom"`
	AnchorTotal int     `json:"anchors_total"`
	AnchorLost  int     `json:"anchors_lost"`
	Note        string  `json:"note"`
}

// RVSBlock —— M7（本笔顺带算，用于证明卸载可逆；任务单只要求 M2/M3/M5）。
type RVSBlock struct {
	OK    int      `json:"ok"`
	Total int      `json:"total"`
	Rate  float64  `json:"rate"`
	Bad   []string `json:"bad,omitempty"`
}

// Gate —— 一条硬闸的判定（值/线/通过）。
type Gate struct {
	Name string `json:"name"`
	Want string `json:"want"`
	Got  string `json:"got"`
	Pass bool   `json:"pass"`
}

// Report —— 跑批产物（report.json 的结构）。
type Report struct {
	RunID         string         `json:"run_id"`
	GeneratedAt   string         `json:"generated_at"`
	Engine        string         `json:"engine"`
	Budget        int            `json:"budget"`
	Policy        map[string]any `json:"policy"`
	SamplesSHA256 string         `json:"samples_sha256"`
	N             int            `json:"n"`
	Controls      int            `json:"controls"`
	TriggerCounts map[string]int `json:"trigger_counts"`
	Fallbacks     int            `json:"fallbacks"`
	GuardFails    int            `json:"guard_fails"`

	M2  TPTBlock `json:"m2_tpt"`
	M3  LATBlock `json:"m3_lat"`
	M5  MRRBlock `json:"m5_mrr"`
	M7  RVSBlock `json:"m7_rvs"`
	RAT struct {
		DeletedMedian float64 `json:"deleted_median"`
		AfterGTBefore int     `json:"after_gt_before"`
	} `json:"m4_rat_informational"`

	Gates []Gate   `json:"gates"`
	NA    []string `json:"na"`
	Notes []string `json:"notes"`
	Rows  []RunRow `json:"rows"`
}

// RunBatch —— 真跑一遍：压缩、判 needle、落盘、聚合。
func RunBatch(ctx context.Context, samples []Sample, cfg Config) (*Report, error) {
	if cfg.Budget <= 0 {
		cfg.Budget = 8192
	}
	if cfg.Engine == "" {
		cfg.Engine = "free"
	}
	if cfg.Engine != "free" && cfg.Engine != "summary" {
		return nil, fmt.Errorf("eval: 只接 free / summary 引擎（要 %q：未接模型，拒绝编读数）", cfg.Engine)
	}
	// ★ summary 必须真注入摘要器：没有它就**拒跑**，而不是降级成 free 再贴 summary 的名。
	if cfg.Engine == "summary" && cfg.Summarizer == nil {
		return nil, fmt.Errorf("eval: engine=summary 但 Config.Summarizer 为空 ⇒ 拒跑（降级会静默产出 free 的读数贴上 summary 的名）")
	}
	if cfg.WorkDir == "" {
		return nil, fmt.Errorf("eval: WorkDir 必填（unload 原件落点）")
	}
	unloadDir := filepath.Join(cfg.WorkDir, "unload")
	if err := os.MkdirAll(unloadDir, 0o755); err != nil {
		return nil, err
	}

	digest, err := samplesDigest(samples)
	if err != nil {
		return nil, err
	}
	if cfg.RunID == "" {
		cfg.RunID = fmt.Sprintf("p2-%s-obs%d-%s", cfg.Engine, len(samples), digest[:8])
	}

	// 样本整批落盘（审计用：跑出来的数字必须能对回具体样本）
	if err := writeJSON(filepath.Join(cfg.WorkDir, "samples.json"), samples); err != nil {
		return nil, err
	}

	pol := compress.DefaultPolicy(cfg.Budget)
	pol.Engine = cfg.Engine
	pol.Summarize = cfg.Summarizer // P3：真摘要器由调用方注入（本包不读 env）
	if cfg.RotPctOverride > 0 {
		pol.RotPct = cfg.RotPctOverride
	}
	if cfg.PerOutputOverride > 0 {
		pol.PerOutputPct = cfg.PerOutputOverride
	}

	rep := &Report{
		RunID:         cfg.RunID,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Engine:        cfg.Engine,
		Budget:        cfg.Budget,
		SamplesSHA256: digest,
		N:             len(samples),
		TriggerCounts: map[string]int{},
		Policy: map[string]any{
			"budget": pol.Budget, "rot_pct": pol.RotPct, "compact_pct": pol.CompactPct,
			"per_output_pct": pol.PerOutputPct, "headroom_pct": pol.HeadroomPct,
			"keep_recent": pol.KeepRecent, "keep_pinned": pol.KeepPinned,
			"engine": pol.Engine, "max_ratio": pol.MaxRatio, "deadline_ms": pol.Deadline.Milliseconds(),
		},
	}

	var (
		tpts, lats, deleteds []float64
		tptTotal, tptBefore  int
		rvsBad               []string
	)
	// 每条样本：原文先落盘（内容寻址），再压缩，再从盘上取回核对。
	for _, s := range samples {
		for _, sg := range s.Segments {
			if sg.RawPtr == "" {
				continue
			}
			full := filepath.Join(cfg.WorkDir, filepath.FromSlash(sg.RawPtr))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return nil, err
			}
			if st, err := os.Stat(full); err == nil && st.Size() == int64(len(sg.Text)) {
				continue
			}
			if err := os.WriteFile(full, []byte(sg.Text), 0o600); err != nil {
				return nil, err
			}
		}

		before := s.toCompress()
		beforeTokens := compress.TotalTokens(before)

		t0 := time.Now()
		res, applyErr := compress.Apply(ctx, before, pol)
		lat := float64(time.Since(t0).Microseconds()) / 1000.0

		row := RunRow{
			SampleID: s.ID, Class: s.Class, Lang: s.Lang, Control: s.Control,
			Engine: cfg.Engine, Budget: cfg.Budget,
			Before: beforeTokens, After: res.After,
			Ratio: res.Ratio, Deleted: deleted(beforeTokens, res.After),
			LatencyMS: round3(lat),
			Trigger:   string(res.Trigger), Mechanism: res.Mechanism, Aggressive: res.Aggressive,
			Fallback: res.Fallback, Reason: res.Reason,
			RecallTokens: 0, // 本笔不接模型 ⇒ 没有「回查后重注入」，见 rep.NA
		}
		if applyErr != nil && res.Reason == "" {
			row.Reason = applyErr.Error()
		}
		if res.Segments == nil {
			res.Segments = before
		}

		// 必留闸独立复算（不只信 res.Fallback —— 负控 4 的前提）
		gErr := compress.NewGuard().Check(before, res.Segments)
		row.GuardOK, row.GuardErr = gErr == nil, errString(gErr)
		if !row.GuardOK {
			rep.GuardFails++
		}

		windowNorm := norm(joinTexts(res.Segments))
		afterAnchorBlob := windowNorm
		for _, sg := range res.Segments {
			for _, a := range sg.Anchors {
				afterAnchorBlob += "\n" + norm(a)
			}
		}

		// anchors：原文锚点是否仍可达（窗口文本 ∪ 输出段 Anchors）
		for _, sg := range s.Segments {
			for _, a := range sg.Anchors {
				if a == "" {
					continue
				}
				row.AnchorsTotal++
				if !strings.Contains(afterAnchorBlob, norm(a)) {
					row.AnchorsLost++
				}
			}
		}

		// needle：window 口径 + rec 口径（经 raw_ptr 从盘上真读）。
		// ★ rec 口径按任务单 M5 公式对**全部** needle 生效（window ∨ 盘上），
		// 不是只对 recoverable 类 —— 否则「窗口丢了但盘上取回得到」会被误记成信息丢失。
		var rawCache = map[string]string{}
		for _, nd := range s.Needles {
			row.NeedlesTotal++
			win := needleHit(windowNorm, nd.Text)
			if win {
				row.MRAnyWinHit++
			}
			isWindow := nd.Where == WhereWindow
			if isWindow {
				row.NeedlesWindow++
				if win {
					row.MRWindowHit++
				} else {
					row.WindowLostIDs = append(row.WindowLostIDs, nd.ID)
				}
			} else {
				row.NeedlesRecoverable++
			}
			if win {
				if !isWindow {
					row.MRRecoverHit++
				}
				continue
			}
			// 窗口里没有：两类都试盘上取回
			hit := false
			if nd.Segment >= 0 && nd.Segment < len(s.Segments) {
				if ptr := s.Segments[nd.Segment].RawPtr; ptr != "" {
					body, ok := rawCache[ptr]
					if !ok {
						if b, err := os.ReadFile(filepath.Join(cfg.WorkDir, filepath.FromSlash(ptr))); err == nil {
							body = norm(string(b))
						}
						rawCache[ptr] = body
					}
					hit = needleHit(body, nd.Text)
				}
			}
			if hit {
				row.MRDiskHit++
				if !isWindow {
					row.MRRecoverHit++
				}
				continue
			}
			row.RecLostIDs = append(row.RecLostIDs, nd.ID)
		}

		// RVS：被卸载的段（原文已不在窗口）= 必须能从盘上逐字节取回，且指针进了窗口
		for _, sg := range s.Segments {
			if sg.RawPtr == "" {
				continue
			}
			if strings.Contains(windowNorm, norm(sg.Text)) {
				continue // 未卸载（对照样本走这里）
			}
			row.UnloadedSegments++
			row.RVSTotal++
			ok := false
			if b, err := os.ReadFile(filepath.Join(cfg.WorkDir, filepath.FromSlash(sg.RawPtr))); err == nil {
				ok = sha256Hex(b) == sha256Hex([]byte(sg.Text))
			}
			if ok && strings.Contains(windowNorm, sg.RawPtr) {
				row.RVSOK++
			} else {
				row.RVSBad++
				rvsBad = append(rvsBad, fmt.Sprintf("%s %s", s.ID, sg.RawPtr))
			}
		}

		row.TPT = compress.EstimateTokens(joinTexts(res.Segments)) + row.RecallTokens

		rep.TriggerCounts[string(res.Trigger)]++
		if res.Fallback {
			rep.Fallbacks++
		}
		if s.Control {
			rep.Controls++
		}

		rep.Rows = append(rep.Rows, row)
		tpts = append(tpts, float64(row.TPT))
		lats = append(lats, row.LatencyMS)
		deleteds = append(deleteds, row.Deleted)
		tptTotal += row.TPT
		tptBefore += row.Before
	}

	// ---- M2：TPT_s = EstimateTokens(join(Segments)) + Σ recall（任务单 §4 M2）
	rep.M2 = TPTBlock{
		Total: tptTotal, Avg: round3(avg(tpts)), P95: int(pctNearestRank(tpts, 0.95)),
		Min: int(pctNearestRank(tpts, 0)), Max: int(pctNearestRank(tpts, 1)),
		TotalBefore: tptBefore, RecallTotal: 0,
		Note: "本笔不接模型、不做回查重注入 ⇒ recall=0、TPT 只含压缩后窗口；单引擎跑批无对照 ⇒ 只出读数不出判定。TPT 是拼窗口径（段间换行符计入、合并后一次 floor），与逐段 after 的合计不等，属定义差不是漏记（见 ScanReport.notes 与 md「读法」第 6 条）。",
	}

	// ---- M3：LAT_i = 完成 − 开始（只包 compress.Apply），p95 最近秩
	rep.M3 = LATBlock{
		N:     len(lats),
		P50MS: round3(pctNearestRank(lats, 0.50)),
		P95MS: round3(pctNearestRank(lats, 0.95)),
		MaxMS: round3(pctNearestRank(lats, 1.0)),
	}

	// ---- M5：MRR_win 硬闸 + MRR_rec 全部 + AnchorLost 绝对数
	var winHits, winDen, recHits, recDen, anchorTotal, anchorLost int
	for _, r := range rep.Rows {
		winHits += r.MRWindowHit
		winDen += r.NeedlesWindow
		recHits += r.MRAnyWinHit + r.MRDiskHit
		recDen += r.NeedlesTotal
		anchorTotal += r.AnchorsTotal
		anchorLost += r.AnchorsLost
	}
	rep.M5 = MRRBlock{
		MRRWin: rate(winHits, winDen), MRRWinHits: winHits, MRRWinDenom: winDen,
		MRRRec: rate(recHits, recDen), MRRRecHits: recHits, MRRRecDenom: recDen,
		AnchorTotal: anchorTotal, AnchorLost: anchorLost,
		Note: "MRR_win = window needle 命中率（硬闸）；MRR_rec = 命中窗口，或经 raw_ptr 从盘上逐字取回（对全部 needle 生效，任务单 §4 M5）",
	}

	okRVS := 0
	for _, r := range rep.Rows {
		okRVS += r.RVSOK
	}
	totalRVS := 0
	for _, r := range rep.Rows {
		totalRVS += r.RVSTotal
	}
	rep.M7 = RVSBlock{OK: okRVS, Total: totalRVS, Rate: rate(okRVS, totalRVS), Bad: rvsBad}

	rep.RAT.DeletedMedian = round4(median(deleteds))
	for _, r := range rep.Rows {
		if r.After > r.Before {
			rep.RAT.AfterGTBefore++
		}
	}

	// ---- 硬闸（阈值来自任务单 §5.1 的 free 列 / §4 各条合格线，不是我们自定的）
	rep.Gates = []Gate{
		{Name: "M5 MRR_win", Want: "= 1.000", Got: fmt.Sprintf("%.3f", rep.M5.MRRWin), Pass: winDen > 0 && rep.M5.MRRWin == 1},
		{Name: "M5 MRR_rec", Want: "= 1.000", Got: fmt.Sprintf("%.3f", rep.M5.MRRRec), Pass: recDen > 0 && rep.M5.MRRRec == 1},
		{Name: "M5 AnchorLost", Want: "= 0", Got: fmt.Sprintf("%d", anchorLost), Pass: anchorLost == 0},
		{Name: "M6 RBR_guard", Want: "= 0", Got: fmt.Sprintf("%d", rep.GuardFails), Pass: rep.GuardFails == 0},
		{Name: "M7 RVS", Want: "= 1.000", Got: fmt.Sprintf("%.3f (%d/%d)", rep.M7.Rate, rep.M7.OK, rep.M7.Total), Pass: rep.M7.Total > 0 && rep.M7.Rate == 1},
		{Name: "M4 after>before", Want: "= 0 次", Got: fmt.Sprintf("%d", rep.RAT.AfterGTBefore), Pass: rep.RAT.AfterGTBefore == 0},
		{Name: "M3 LAT p95 (free)", Want: "≤ 50 ms", Got: fmt.Sprintf("%.3f ms", rep.M3.P95MS), Pass: rep.M3.P95MS <= 50},
		{Name: "对照样本 trigger", Want: "全部 none", Got: fmt.Sprintf("%d/%d none", countTrigger(rep.Rows, "none", true), rep.Controls), Pass: countTrigger(rep.Rows, "none", true) == rep.Controls},
	}

	rep.NA = []string{
		"M1 DSR：未给模型端点 ⇒ N/A（不记 0、不推算）",
		"M8 RPT：尺子内没有工具调用 ⇒ N/A（只在真机/任务回放上量）",
		"summary / external 引擎：本笔不接模型 ⇒ 未跑，M2 无对照引擎",
	}

	rep.Notes = []string{
		"M3 的 LAT 只包 compress.Apply（不含样本加载、落盘与判分），与任务单 §4 M3 的取数口径一致",
		"Apply 在 before/Budget ≥ RotPct 时就置 Trigger=pressure；pruner 一段都没卸时仍会把 Mechanism 记成 prune（负控实测：RotPct=0.10 下 R05/R10 trigger=pressure 而 after==before）⇒ 尺子不拿 mechanism≠none 当「压过」的证据，只认 after<before",
		"token 估算只用 compress.EstimateTokens 一处（任务单 §7 风险 3）；样例里中文字符按 1 token/字、ASCII 按 4 字符/token",
		"PrunerFree 的 dedup 只在整段文本完全相同时生效，本批 10 条样本无整段重复 ⇒ 全部压缩量来自「单条超限卸载」一条闸",
	}

	if cfg.OutBase != "" {
		if err := writeJSON(cfg.OutBase+".json", rep); err != nil {
			return nil, err
		}
		if err := os.WriteFile(cfg.OutBase+".md", []byte(renderMarkdown(rep)), 0o644); err != nil {
			return nil, err
		}
		if err := writeRunlog(cfg.OutBase+".jsonl", rep); err != nil {
			return nil, err
		}
	}
	return rep, nil
}

// DeterministicDigest —— 跨次跑批的确定性指纹：剔除时间与耗时后对 rows 取哈希。
// 两次跑批只要样本与引擎行为相同，这个值就必须相等（TestBatch_Deterministic 用它）。
func (r *Report) DeterministicDigest() string {
	type det struct {
		SampleID           string `json:"sample_id"`
		Before, After      int
		Trigger, Mechanism string
		GuardOK            bool
		MRWindowHit        int
		MRRecoverHit       int
		TPT                int
		RVSOK, RVSTotal    int
		AnchorsLost        int
	}
	ds := make([]det, 0, len(r.Rows))
	for _, row := range r.Rows {
		ds = append(ds, det{
			SampleID: row.SampleID, Before: row.Before, After: row.After,
			Trigger: row.Trigger, Mechanism: row.Mechanism, GuardOK: row.GuardOK,
			MRWindowHit: row.MRWindowHit, MRRecoverHit: row.MRRecoverHit,
			TPT: row.TPT, RVSOK: row.RVSOK, RVSTotal: row.RVSTotal, AnchorsLost: row.AnchorsLost,
		})
	}
	b, _ := json.Marshal(ds)
	return sha256Hex(b)[:16]
}

// ---------------------------------------------------------------- 小工具

func samplesDigest(samples []Sample) (string, error) {
	b, err := json.Marshal(samples)
	if err != nil {
		return "", err
	}
	return sha256Hex(b), nil
}

func joinTexts(segs []compress.Segment) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		parts = append(parts, s.Text)
	}
	return strings.Join(parts, "\n")
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func rate(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return round4(float64(n) / float64(d))
}

func avg(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	t := 0.0
	for _, x := range xs {
		t += x
	}
	return t / float64(len(xs))
}

func round3(x float64) float64 { return float64(int64(x*1000+0.5)) / 1000 }
func round4(x float64) float64 { return float64(int64(x*10000+0.5)) / 10000 }

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func countTrigger(rows []RunRow, want string, onlyControl bool) int {
	n := 0
	for _, r := range rows {
		if onlyControl && !r.Control {
			continue
		}
		if r.Trigger == want {
			n++
		}
	}
	return n
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func writeRunlog(path string, r *Report) error {
	var b strings.Builder
	for _, row := range r.Rows {
		line, err := json.Marshal(row)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// renderMarkdown —— 一页人读摘要（表格不换行、只用实测值）。
func renderMarkdown(r *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 压缩尺子基线 · %s 引擎 · %d 条\n\n", r.Engine, r.N)
	fmt.Fprintf(&b, "- run_id：`%s`　生成时间：%s\n", r.RunID, r.GeneratedAt)
	fmt.Fprintf(&b, "- 样本：%d 条（对照 %d 条 · %s）· samples_sha256：`%s` · 确定性指纹：`%s`\n",
		r.N, r.Controls, classCountsLine(r.Rows), r.SamplesSHA256[:16], r.DeterministicDigest())
	fmt.Fprintf(&b, "- 引擎：`%s` · Budget %d · RotPct %.2f · CompactPct %.2f · PerOutputPct %.2f · HeadroomPct %.2f · KeepRecent %v · MaxRatio %.2f\n\n",
		r.Engine, r.Budget, r.Policy["rot_pct"], r.Policy["compact_pct"], r.Policy["per_output_pct"],
		r.Policy["headroom_pct"], r.Policy["keep_recent"], r.Policy["max_ratio"])

	fmt.Fprintf(&b, "## 八指标\n\n")
	fmt.Fprintf(&b, "| 指标 | 读数 | 阈值（任务单） | 判定 |\n|---|---|---|---|\n")
	fmt.Fprintf(&b, "| M1 DSR | **N/A**（未给模型端点，分母 %d 条样本，实得 0） | ≥ free−0.02 且 ≥0.90 | N/A |\n", r.N)
	fmt.Fprintf(&b, "| M2 TPT 总 / 均 / p95 / max | %d / %.1f / %d / %d tok | 单引擎无对照 ⇒ 只出读数 | — |\n",
		r.M2.Total, r.M2.Avg, r.M2.P95, r.M2.Max)
	fmt.Fprintf(&b, "| M2 压缩前总量 | %d tok | — | — |\n", r.M2.TotalBefore)
	fmt.Fprintf(&b, "| M3 LAT p50 / p95 / max | %.3f / %.3f / %.3f ms | free p95 ≤ 50 ms | %s |\n",
		r.M3.P50MS, r.M3.P95MS, r.M3.MaxMS, passStr(r.M3.P95MS <= 50))
	fmt.Fprintf(&b, "| M5 MRR_win | %.4f (%d/%d) | = 1.000（硬闸） | %s |\n",
		r.M5.MRRWin, r.M5.MRRWinHits, r.M5.MRRWinDenom, passStr(r.M5.MRRWin == 1 && r.M5.MRRWinDenom > 0))
	fmt.Fprintf(&b, "| M5 MRR_rec | %.4f (%d/%d) | = 1.000 | %s |\n",
		r.M5.MRRRec, r.M5.MRRRecHits, r.M5.MRRRecDenom, passStr(r.M5.MRRRec == 1 && r.M5.MRRRecDenom > 0))
	fmt.Fprintf(&b, "| M5 AnchorLost | %d / %d | = 0（绝对数） | %s |\n",
		r.M5.AnchorLost, r.M5.AnchorTotal, passStr(r.M5.AnchorLost == 0))
	fmt.Fprintf(&b, "| M4 删除比中位（附加） | %.4f | ≥ 0.50 | %s |\n", r.RAT.DeletedMedian, passStr(r.RAT.DeletedMedian >= 0.5))
	fmt.Fprintf(&b, "| M6 RBR guard / fallback | %d / %d | guard = 0、fallback ≤ 1%% | %s |\n",
		r.GuardFails, r.Fallbacks, passStr(r.GuardFails == 0))
	fmt.Fprintf(&b, "| M7 RVS | %d/%d = %.4f | = 1.000 | %s |\n",
		r.M7.OK, r.M7.Total, r.M7.Rate, passStr(r.M7.Total > 0 && r.M7.Rate == 1))
	fmt.Fprintf(&b, "| M7 AnchorLost | %d / %d | = 0（绝对数） | %s |\n",
		r.M5.AnchorLost, r.M5.AnchorTotal, passStr(r.M5.AnchorLost == 0))
	fmt.Fprintf(&b, "| M8 RPT | **N/A**（尺子内无工具调用，分母 0 次调用） | ≤ 基线 + 0.05 | N/A |\n\n")

	fmt.Fprintf(&b, "## 逐样本\n\n")
	fmt.Fprintf(&b, "| 样本 | 语 | 对照 | before | after | 删除比 | LAT ms | trigger | MRR_win | MRR_rec |\n")
	fmt.Fprintf(&b, "|---|---|---|---|---|---|---|---|---|---|\n")
	for _, row := range r.Rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %.4f | %.3f | %s | %d/%d | %d/%d |\n",
			row.SampleID, row.Lang, yesNo(row.Control), row.Before, row.After, row.Deleted, row.LatencyMS,
			row.Trigger, row.MRWindowHit, row.NeedlesWindow,
			row.MRWindowHit+row.MRRecoverHit, row.NeedlesTotal)
	}

	fmt.Fprintf(&b, "\n## 闸与 N/A\n\n")
	for _, g := range r.Gates {
		fmt.Fprintf(&b, "- %s %s：`%s` → %s\n", passStr(g.Pass), g.Name, g.Got, g.Want)
	}
	fmt.Fprintf(&b, "\n")
	for _, n := range r.NA {
		fmt.Fprintf(&b, "- N/A：%s\n", n)
	}
	fmt.Fprintf(&b, "\n## 读法（别把这几条读错）\n\n")
	fmt.Fprintf(&b, "1. **TPT 是「压缩后窗口」的 token，不含回查重注入**：本笔不接模型、没有召回动作，recall=0（任务单 M2 算式里那一项为 0，不是漏算）。真机 M2 用 `prompt_n+cache_n`，口径不同不可直接比。\n")
	fmt.Fprintf(&b, "2. **MRR_rec 走盘**：recoverable needle 是从 `<work>/unload/<sha16>.txt` 真读回来判的，不是拿内存原文自证；RVS 同时校验了「文件字节 == 原文」与「指针进了窗口」。\n")
	fmt.Fprintf(&b, "3. **free 的删除比高不等于信息没丢**：它把大段 observation 换成指针（原文可回查），窗口 needle 靠「闸没碰到的小段/Pinned 段」保住。M5 两个口径分开看才有意义。\n")
	fmt.Fprintf(&b, "4. **对照样本（R05/R10）断言非空**：负控用例（`TestBatch_ControlGateNotVacuous`）把 RotPct 压到 0.10、PerOutputPct 压到 0.10 时它们必须变成 pressure 且真被压 —— 证明「trigger=none」是真判出来的，不是恒真。\n")
	return b.String()
}

// classCountsLine —— 「observation 10 / trajectory 10 / …」一行，报告抬头用。
func classCountsLine(rows []RunRow) string {
	order := []string{"observation", "trajectory", "plan", "memory", "representation"}
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Class]++
	}
	var parts []string
	for _, class := range order {
		if counts[class] == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %d", class, counts[class]))
		delete(counts, class)
	}
	// 未知类按字典序追加（不靠 map 迭代顺序，报告必须可复现）
	for _, class := range sortedKeys(counts) {
		parts = append(parts, fmt.Sprintf("%s %d", class, counts[class]))
	}
	return strings.Join(parts, " / ")
}

func passStr(ok bool) string {
	if ok {
		return "✅ PASS"
	}
	return "❌ FAIL"
}

func yesNo(b bool) string {
	if b {
		return "是"
	}
	return "否"
}
