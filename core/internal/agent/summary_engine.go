package agent

// summary_engine.go — P3「summary」摘要引擎在 agent 侧的**唯一装配点**：配置驱动、永不自动激活。
//
// 三层分工（动这个文件前先读）：
//   - core/internal/compress —— 只管编排（剪枝 → 摘要 → 必留闸 → 预算闸）与降级，**不读 env**；
//   - 本文件 —— 把 env 读成 Policy：Engine=free（缺省）或 Engine=summary + 注入 SummarizerFunc；
//   - context_window.go —— 消费面（compressOneRound）。引擎没启用 / 摘要失败 / 结果为空 ⇒ 一律
//     走 summarizeRound（原 free 口径）——**主流程永远拿得到摘要**，失败不出本文件（铁律 ③）。
//
// ★ 永不自动激活：只有 `ZERG_SUMMARY_ENGINE=1` 且 `ZERG_SUMMARY_ENDPOINT` 与 `ZERG_SUMMARY_MODEL`
//   同时非空，才构造 HTTPSummarizer 并注入 Policy。任一项缺失 ⇒ **不建、不报错**、启动日志明写
//   「摘要引擎未启用（默认 free）」。构造失败（URL 非法等）同样只降级不抛错：压缩是旁路，
//   不许因为旁路配置错把进程带崩。
//
// ★ 为什么不学 chat 层那样「有网关地址就默认摘要」：默认路径必须是零算力、可逆的 free ——
//   一旦默认烧模型，压缩就从「省 token」变成「烧 token 的隐藏成本」，且不可预期。

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/compress"
)

const (
	// envSummaryEngine —— 总闸：1/true/yes/on 才继续看下面两项（其余取值一律视为未启用）。
	envSummaryEngine = "ZERG_SUMMARY_ENGINE"
	// envSummaryEndpoint —— OpenAI 兼容端点基址（网关 / llama-server / DS4 都是这条面）。
	envSummaryEndpoint = "ZERG_SUMMARY_ENDPOINT"
	// envSummaryModel —— 必填模型名（空 ⇒ 视同未配；缺它每次调用都白烧一次 400）。
	envSummaryModel = "ZERG_SUMMARY_MODEL"
	// envSummaryToken —— 可选令牌（只进请求头，见 summarizer_http.go 的 redact）。
	envSummaryToken = "ZERG_SUMMARY_TOKEN"
	// envSummaryTimeoutS —— 可选：单次摘要超时（秒）。缺省 30s。
	envSummaryTimeoutS = "ZERG_SUMMARY_TIMEOUT_S"
	// envSummaryMaxTokens —— 可选：单次输出预算（token）。缺省交给 compress 的 1200。
	envSummaryMaxTokens = "ZERG_SUMMARY_MAX_TOKENS"

	// summaryEngineOffStatus —— 未启用时启动日志的**原文**（默认 free：压缩行为一个字都不变）。
	summaryEngineOffStatus = "摘要引擎未启用（默认 free）"
	// summaryEngineOnStatus —— 启用时启动日志的原文。
	summaryEngineOnStatus = "摘要引擎已启用（summary）"

	// defaultSummaryTimeout —— 缺省单次摘要超时。
	// 比 compress.DefaultSummarizerTimeout(120s) 小一档：这里是**主循环内**的旁路调用，
	// 一次轮转最多等 30s；等到 120s 就等于把主循环冻住了（铁律 ③）。
	defaultSummaryTimeout = 30 * time.Second

	// summaryRoundMinTokens —— 一轮估算达到这个量才值得烧一次模型调用。
	// 低于它 ⇒ 直接 free：小轮的「摘要」收益抵不过一次往返的延迟与算力。
	summaryRoundMinTokens = 600
)

// summaryEngine —— Agent 上的摘要引擎装配结果。**零值必须是「未启用」**：
// 包里一堆测试直接 `&Agent{history: ...}` 造实例，零值退 free 才不会有隐藏行为。
type summaryEngine struct {
	policy  compress.Policy // Engine=free 且 Summarize=nil ⇒ 消费面走 free
	enabled bool
	status  string // 启动日志原文（见 summaryEngineOffStatus）
	reason  string // 未启用/启用失败的原因（只进日志，不抛错）

	// 观测面（只加不删；测试与审计读它）
	calls        int    // 真正走引擎的轮次
	fallbacks    int    // 其中退回 free 的轮次
	lastFallback bool   // 最近一轮是否降级
	lastReason   string // 最近一次降级原因
}

// resolveSummaryEngine —— **纯装配**：把 env 读成 Policy，不建连接、不发请求、不写日志。
//
// env 以函数注入（NewAgent 传 os.Getenv）⇒ 单测能逐态钉死，且不受真机环境影响。
// 返回值的 policy 一定可用：未启用时是 DefaultPolicy + Engine="free"（零值 Policy 也能用，
// 这里显式落 free 是为了让「默认是什么」在读数里可见）。
func resolveSummaryEngine(budget int, env func(string) string) summaryEngine {
	pol := compress.DefaultPolicy(budget)
	pol.Engine = "free"
	st := summaryEngine{policy: pol, status: summaryEngineOffStatus}

	// ① 总闸：没显式说「开」就当没配（✗ 不猜、不按端点存在与否自动开）
	if !summaryGateOn(env(envSummaryEngine)) {
		st.reason = "未设 " + envSummaryEngine + "=1"
		return st
	}

	// ② 端点与模型名必须都有：缺任一项都不建（宁可压不动，也不每次调用烧一次 400）
	endpoint := strings.TrimSpace(env(envSummaryEndpoint))
	model := strings.TrimSpace(env(envSummaryModel))
	if endpoint == "" || model == "" {
		miss := make([]string, 0, 2)
		if endpoint == "" {
			miss = append(miss, envSummaryEndpoint)
		}
		if model == "" {
			miss = append(miss, envSummaryModel)
		}
		st.reason = "缺 " + strings.Join(miss, " / ")
		return st
	}

	timeout := defaultSummaryTimeout
	if v := strings.TrimSpace(env(envSummaryTimeoutS)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	maxTokens := 0 // 0 ⇒ compress 侧缺省（1200）
	if v := strings.TrimSpace(env(envSummaryMaxTokens)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxTokens = n
		}
	}

	// ③ 装配：构造失败也只降级（压缩是旁路，配置错不许带崩主流程）
	h, err := compress.NewHTTPSummarizer(compress.HTTPSummarizerConfig{
		Endpoint:  endpoint,
		Model:     model,
		Token:     strings.TrimSpace(env(envSummaryToken)),
		Timeout:   timeout,
		MaxTokens: maxTokens,
	})
	if err != nil {
		st.reason = "构造失败：" + err.Error()
		return st
	}

	pol.Engine = "summary"
	pol.Summarize = h.Func() // 唯一注入面：引擎本体不认识端点/模型/SDK
	pol.Deadline = timeout   // Apply 里再套一层超时（正交于 HTTP 自身的 timeout）
	st.policy = pol
	st.enabled = true
	st.status = summaryEngineOnStatus
	st.reason = "模型=" + model
	return st
}

// summaryGateOn —— 总闸的取值判定：1 / true / yes / on（大小写与空白不敏感）。
// 其它任何值（含 "0"、空、笔误）一律 false —— 默认关，只有明写才开。
func summaryGateOn(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// logLine —— 启动日志原文（未启用时**必须**含 summaryEngineOffStatus 这一句）。
// 不打印端点：URL 的 query 里可能带凭据（与 summarizer_http.go 的不回显密钥同一条纪律）。
func (s summaryEngine) logLine() string {
	if s.status == "" {
		return summaryEngineOffStatus // 零值也给出这句（防御：装配点被绕过时日志仍可读）
	}
	if s.reason == "" {
		return s.status
	}
	return s.status + "｜" + s.reason
}

// announceSummaryEngine —— 构造期宣告一次（NewAgent 调；一行，含「默认 free」原文）。
func announceSummaryEngine(s summaryEngine) {
	log.Printf("🧩 zerg agent compress: %s", s.logLine())
}

// summarizeRoundForCompaction —— 一轮的摘要文本：能走引擎就走引擎，其余**一律**回 free。
//
// 返回的串直接追加进 rollingSummary（口径与原 summarizeRound 一致：以换行开头）。
// 本函数**永不返回错误**：调用方（compressOneRound）的职责是「这一轮必须被压掉」，
// 摘要质量是旁路问题，不许把主流程挂住或回滚。
func (a *Agent) summarizeRoundForCompaction(msgs []Message) string {
	free := func() string { return summarizeRound(msgs) }

	// 默认路径：引擎未启用（零值 / 缺 env）⇒ 零算力零网络，与接线前逐字一致
	if !a.summaryEngine.enabled || a.summaryEngine.policy.Summarize == nil {
		return free()
	}

	segs := summarySegments(msgs)
	before := compress.TotalTokens(segs)
	if before < summaryRoundMinTokens {
		return free() // 小轮不值得烧一次调用
	}

	// 本轮即窗口：Budget = 本轮估算 ⇒ ratio=1.0（撞线/溢出），引擎才会动手。
	// PerOutputPct / HeadroomPct 抬到 1.0：摘要要在**原文**上做，先折叠会把中段丢掉
	// （PrunerFree 的无指针折叠只留头 400 + 尾 200 字）。
	pol := a.summaryEngine.policy
	pol.Budget = before
	pol.PerOutputPct = 1.0
	pol.HeadroomPct = 1.0
	pol.KeepRecent = 0 // 整轮交给模型；summary 本身就是这一轮的替代物
	if pol.Deadline <= 0 {
		pol.Deadline = defaultSummaryTimeout
	}

	a.summaryEngine.calls++
	res, err := compress.Apply(context.Background(), segs, pol)
	if err != nil {
		a.markSummaryFallback(err.Error())
		return free()
	}
	if res.Fallback {
		// 引擎自报降级（如 guard/预算闸判失败）：结果不可信 ⇒ 用 free，别把半截摘要塞进上下文
		a.markSummaryFallback(res.Reason)
		return free()
	}
	text := joinSummarySegments(res.Segments)
	if strings.TrimSpace(text) == "" {
		a.markSummaryFallback("引擎返回空摘要")
		return free()
	}

	if a.logger != nil {
		a.logger.LogEvent("context", "info", "summary_engine",
			"", fmt.Sprintf("摘要引擎: 机制=%s, 档位=%s, 轮次 token=%d→%d, 超时=%s",
				res.Mechanism, res.Aggressive, before, res.After, pol.Deadline),
			nil, "", "", "")
	}
	return "\n" + text + "\n"
}

// markSummaryFallback —— 记一笔降级（观测用；不改变已经发生的压缩）。
func (a *Agent) markSummaryFallback(why string) {
	a.summaryEngine.fallbacks++
	a.summaryEngine.lastFallback = true
	a.summaryEngine.lastReason = why
	if a.logger != nil {
		a.logger.LogEvent("context", "warn", "summary_engine_fallback",
			"", fmt.Sprintf("摘要引擎降级 free: %s", why), nil, "", "", "")
	}
}

// summarySegments —— Message → compress.Segment（kind 按角色分：tool=观察，其余=轨迹）。
//
// 不设 Anchors / RawPtr：这两个字段是「适配器打标」的活（compress 包契约），
// 这里凭空造锚点只会让必留闸误判；原文仍可经 sessionLog 回查。
func summarySegments(msgs []Message) []compress.Segment {
	segs := make([]compress.Segment, 0, len(msgs))
	for _, m := range msgs {
		text := m.Content
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			// 工具调用名与参数要进摘要：路径/命令大多在这里（模型才知道「当时在做什么」）
			var sb strings.Builder
			sb.WriteString(text)
			for _, tc := range m.ToolCalls {
				sb.WriteString("\n调用工具: ")
				sb.WriteString(tc.Name)
				sb.WriteString("(")
				sb.WriteString(truncateArgs(tc.Args))
				sb.WriteString(")")
			}
			text = sb.String()
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		kind := compress.KindTrajectory
		if m.Role == "tool" {
			kind = compress.KindObservation
		}
		segs = append(segs, compress.Segment{Kind: kind, Text: text, Source: "agent-round"})
	}
	return segs
}

// joinSummarySegments —— 引擎交回的片段拼成一段文本（摘要区只吃一段）。
func joinSummarySegments(segs []compress.Segment) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		if t := strings.TrimSpace(s.Text); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n")
}

// summaryEngineEnvLookup —— NewAgent 用的真 env 读取器（测试可换成假 env）。
func summaryEngineEnvLookup(k string) string { return os.Getenv(k) }
