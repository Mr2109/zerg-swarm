package eval

// 真模型整批（P3）：把「真摘要器」注入 RunBatch 的唯一正门。
//
// 为什么单开一个文件：`eval` 包按纪律**不读 env、不建 HTTP 客户端**（见 runner.go Config.Summarizer 注释）。
// 这个文件是**例外且封闭**的——只在测试里、只在显式给 env 时跑：
//
//	ZERG_EVAL_SUMMARY_URL    必填（完整 chat/completions 地址；不给 ⇒ 整个用例 skip，绝不编读数）
//	ZERG_EVAL_SUMMARY_MODEL  必填（网关里的模型 id）
//	ZERG_EVAL_SUMMARY_TOKEN  选填：令牌**文件**路径（缺省 ~/.zerg/token）
//	                            ★ 只收文件路径，不收令牌本体——令牌不进 argv / env / 日志
//	ZERG_EVAL_SUMMARY_BUDGET 选填，缺省 8192
//	ZERG_EVAL_SUMMARY_ONLY   选填，逗号分隔样本 id（缺省 R01,R06,R21,R41 —— 五类各挑一条）
//	ZERG_EVAL_SUMMARY_WORK   必填，跑批落点（samples.json / unload/ / report.json）
//	ZERG_EVAL_SUMMARY_OUT    选填，报告 JSON 落点（缺省 <WORK>/report-summary.json）
//
// 跑法：
//
//	cd core && env TMPDIR="$(getconf DARWIN_USER_TEMP_DIR)" \
//	  ZERG_EVAL_SUMMARY_URL=http://127.0.0.1:8082/v1/chat/completions \
//	  ZERG_EVAL_SUMMARY_MODEL=example-26b-review \
//	  ZERG_EVAL_SUMMARY_BUDGET=8192 \
//	  ZERG_EVAL_SUMMARY_WORK=~/.zerg/state/压缩-尺子-summary-20261023.work \
//	  /opt/homebrew/bin/go test ./internal/compress/eval -run TestEvalSummaryLive -v -timeout 30m

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

// newHTTPSummarizer —— 一个最小的 OpenAI 兼容摘要器：把片段拼成一篇、要模型保留锚点后压缩、回一段。
//
// 契约照 compress.SummarizerFunc：收「已剪枝、非必留、非最近 N 条」的片段，交回等价的更短片段。
// 返回**一段**（Kind 取第一段）—— 摘要把多段揉成一段是正常的；Anchors 合并搬运，
// 由 SummaryEngine 的必留闸复核（它若发现锚点没带回去会整轮降级 free，不会静默放过）。
func newHTTPSummarizer(endpoint, model, token string, maxTokens int) compress.SummarizerFunc {
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	client := &http.Client{Timeout: 180 * time.Second}
	return func(ctx context.Context, segs []compress.Segment) ([]compress.Segment, error) {
		if len(segs) == 0 {
			return segs, nil
		}
		var b strings.Builder
		var anchors []string
		seen := map[string]bool{}
		for _, s := range segs {
			b.WriteString(s.Text)
			b.WriteString("\n\n")
			for _, a := range s.Anchors {
				if !seen[a] {
					seen[a] = true
					anchors = append(anchors, a)
				}
			}
		}
		body, _ := json.Marshal(map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "system", "content": fmt.Sprintf("你是上下文压缩器。只输出摘要正文，不加前言、不加分析。必须逐字保留：文件路径、命令、错误码、判定结论、数字。全文不超过 %d 个 token。", maxTokens)},
				{"role": "user", "content": b.String()},
			},
			"max_tokens":  maxTokens,
			"temperature": 0.3,
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("X-Auth-Token", token)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("summarizer: HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
		}
		var out struct {
			Choices []struct {
				Message struct{ Content string } `json:"message"`
			} `json:"choices"`
			Usage map[string]any `json:"usage"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("summarizer: 解析失败: %v（%s）", err, truncate(string(raw), 160))
		}
		if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
			return nil, fmt.Errorf("summarizer: 空摘要")
		}
		return []compress.Segment{{
			Kind:    segs[0].Kind,
			Text:    out.Choices[0].Message.Content,
			Source:  segs[0].Source,
			RawPtr:  segs[0].RawPtr,
			Anchors: anchors,
		}}, nil
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// TestEvalSummaryLive —— 真模型整批（同一进程内注入摘要器 ⇒ 子进程边界不参与）。
func TestEvalSummaryLive(t *testing.T) {
	url := os.Getenv("ZERG_EVAL_SUMMARY_URL")
	if url == "" {
		t.Skip("未给 ZERG_EVAL_SUMMARY_URL ⇒ 跳过（真模型整批必须显式点名端点，绝不默认编读数）")
	}
	model := os.Getenv("ZERG_EVAL_SUMMARY_MODEL")
	if model == "" {
		t.Fatal("给了端点但没给 ZERG_EVAL_SUMMARY_MODEL")
	}
	work := os.Getenv("ZERG_EVAL_SUMMARY_WORK")
	if work == "" {
		t.Fatal("ZERG_EVAL_SUMMARY_WORK 必填（跑批落点）")
	}
	token := ""
	tokenFile := envOr("ZERG_EVAL_SUMMARY_TOKEN", filepath.Join(os.Getenv("HOME"), ".zerg", "token"))
	if b, err := os.ReadFile(tokenFile); err == nil {
		token = strings.TrimSpace(string(b))
	} else {
		t.Logf("令牌文件读不到（%s）：按无鉴权端点试", tokenFile)
	}

	budget := 8192
	if v := os.Getenv("ZERG_EVAL_SUMMARY_BUDGET"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &budget); err != nil {
			t.Fatalf("ZERG_EVAL_SUMMARY_BUDGET 不是整数: %v", err)
		}
	}
	ids := []string{"R01", "R06", "R21", "R41"}
	if v := os.Getenv("ZERG_EVAL_SUMMARY_ONLY"); v != "" {
		ids = nil
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				ids = append(ids, p)
			}
		}
	}
	// ★ 按 id 从**全量 50 条**里取：`Select()` 只搜 observation 那 10 条（Samples()），
	//   拿它取 R21/R41 会静默少条（本笔实测踩过：n=2 而点的是 4 条）。
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var samples []Sample
	for _, s := range AllSamples() {
		if want[s.ID] {
			samples = append(samples, s)
		}
	}
	if len(samples) == 0 {
		t.Fatalf("样本 id 一个都没选到: %v", ids)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}

	maxTok := 400
	if v := os.Getenv("ZERG_EVAL_SUMMARY_MAXTOK"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &maxTok); err != nil {
			t.Fatalf("ZERG_EVAL_SUMMARY_MAXTOK 不是整数: %v", err)
		}
	}
	sum := newHTTPSummarizer(url, model, token, maxTok)
	start := time.Now()
	rep, err := RunBatch(context.Background(), samples, Config{
		Budget:     budget,
		Engine:     "summary",
		WorkDir:    work,
		OutBase:    filepath.Join(work, "report-summary"),
		Summarizer: sum,
	})
	if err != nil {
		t.Fatalf("RunBatch(summary) 失败: %v", err)
	}
	wall := time.Since(start)

	// 人面：一行一样本的硬读数 + M5/M3/M4 汇总。
	t.Logf("== engine=%s model=%s budget=%d n=%d wall=%.1fs ==", rep.Engine, model, budget, rep.N, wall.Seconds())
	t.Logf("fallbacks=%d guard_fails=%d triggers=%v", rep.Fallbacks, rep.GuardFails, rep.TriggerCounts)
	t.Logf("M5 MRR win=%.4f（%d/%d）· rec=%.4f（%d/%d）· anchors 丢 %d/%d",
		rep.M5.MRRWin, rep.M5.MRRWinHits, rep.M5.MRRWinDenom,
		rep.M5.MRRRec, rep.M5.MRRRecHits, rep.M5.MRRRecDenom,
		rep.M5.AnchorLost, rep.M5.AnchorTotal)
	t.Logf("M3 LAT p50=%.2fms p95=%.2fms max=%.2fms", rep.M3.P50MS, rep.M3.P95MS, rep.M3.MaxMS)
	t.Logf("M4 删除中位=%.4f", rep.RAT.DeletedMedian)
	for _, r := range rep.Rows {
		t.Logf("  %-4s %-18s before=%-6d after=%-6d deleted=%.3f lat=%.1fms fallback=%v trigger=%s",
			r.SampleID, r.Class, r.Before, r.After, r.Deleted, r.LatencyMS, r.Fallback, r.Trigger)
	}

	out := envOr("ZERG_EVAL_SUMMARY_OUT", filepath.Join(work, "report-summary.json"))
	raw, _ := json.MarshalIndent(rep, "", "  ")
	if err := os.WriteFile(out, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("报告落盘: %s（%d B）", out, len(raw))

	// 硬断言：summary 真起过作用（否则这批读数是 free 的，贴 summary 的名就是编读数）。
	if rep.Engine != "summary" {
		t.Fatalf("报告 engine=%q，不是 summary", rep.Engine)
	}
	names := make([]string, 0, len(rep.Rows))
	for _, r := range rep.Rows {
		names = append(names, r.SampleID)
	}
	sort.Strings(names)
	t.Logf("样本: %v", names)
}
