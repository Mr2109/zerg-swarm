// obs_compaction.go — T3.1：compaction **三态事件**（started/completed/failed），写进**既有** chat_obs.jsonl
//
// 设计依据：docs/01-设计/设计-内建调试版-v1.2-20260917.md 第〇节 H1 ——
//
//	「事件表里完全没有 compaction 事件（而长对话档必然触发它）⇒ 我们怀疑的失败路径
//	 （压缩后丢关键约束 / 重复劳动 / 悬空 tool_call）恰好不可观测」。
//
// 任务表：docs/项目文档/v2.5.10/任务表-内建调试版实施-20260917.md T3.1。
//
// 要治的盲区（改本文件前先读）：一次压缩在旧观测面里只有**一条终态行**
// （`kind=compact` + `result=ok/fail` + `summary_chars`），于是——
//   - 压到什么程度（tokens/ratio）、这是第几次压（depth）、压掉了哪些消息、留下的与压掉的是不是
//     恰好把全部消息分完 —— 一个都答不上；
//   - 压出来的摘要长什么样（sha256）、结构化模板的六段到底有几段真在（field_coverage）—— 也答不上；
//   - 「开始压了却没有终态」（半途而废 / 悬空）与「根本没压」在事件流里**同形** —— 最危险的形态正好不可见。
//
// 三态事件把这三类问题变成可判定观测。
//
// 口径（与 obs.go 文件头三条铁律同源）：
//  1. **观测绝不改变压缩行为**：本文件只组装/写盘；触发、阈值、冷却、熔断、布局改造仍全在
//     chat_compact_v2.go 的 MaybeCompact 里，一行判定都不搬过来。
//  2. **best-effort**：写失败只记日志（obsWrite 已保证），绝不外抛/阻塞/panic。
//  3. **0 与「未知」可分**（实测教训，见 obs.go）：只有**终态**才知道的量
//     （tokens_after / messages_after / compression_ratio / summary_sha256 / latency_ms）一律
//     **指针 + omitempty** ⇒ 字段缺席 = 未知；started 当下就确定的量（tokens_before /
//     messages_before / depth / 两个 id 集合…）用裸值 ⇒ 0 就是**测到的 0**。
//     绝不把「不知道」写成 0（那会把「没测到」读成「压到 0 token」）。
//  4. **成对要可校验，不靠自称**：started 与它的终态（completed|failed）**共用同一个 span_id**
//     （trace/parent 同源）⇒ 「有没有成对、有没有悬空」在事件流里能直接查；handle 内部保证
//     **一个原子里只发一条终态**（重复调用被吞掉，不会出现两条 completed）。
//  5. **只落 id 与指纹，不落原文**（隐私，与 T1.2 的 args_digest 同一纪律）：kept/summarized 只给
//     消息 id；摘要只给 sha256 与结构覆盖 —— 摘要正文/消息正文一个字都不落。
//
// 与旧 OBS-4 行的关系（读侧必读）：旧的 `kind=compact` 终态行（`result`/`summary_chars`/`fail_reason`
// **原样保留**（老用例、老读侧一律不动，也不删任何已有信息）。三态事件是同一份文件里**新增**的
// `event_name`：`compaction_started` / `compaction_completed` / `compaction_failed`。
// ⇒ 数本族事件请按 **event_name**（例：`grep -c '"event_name":"compaction_completed"'`），
// 不要按 `kind=compact` 数（那会把旧行与三态行一起数上）。终态三态行**同时**带上旧的
// `result`/`summary_chars`/`fail_reason` 字段 ⇒ 「按 result 找成败」的老口径在新行上照样成立。
package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"sync"
	"time"
)

// ── 事件名（非空 event_name ⇒ 这条是事件；本族三态是**唯一**口径）──
const (
	obsEventCompactionStarted   = "compaction_started"
	obsEventCompactionCompleted = "compaction_completed"
	obsEventCompactionFailed    = "compaction_failed"
)

// trigger_reason 闭集（H1 逐字：token_budget / manual / error_recovery）
const (
	// CompactTriggerTokenBudget —— 阈值触发（唯一自动路径：max(min(ctx×0.5, 8000), 2000)）
	CompactTriggerTokenBudget = "token_budget"
	// CompactTriggerManual —— 人工/接口显式触发（force=true）
	CompactTriggerManual = "manual"
	// CompactTriggerErrorRecovery —— 上游明确报「上下文超限」之后的补救性压缩
	//（IsContextLimitError 判到 ⇒ 调用方 force + WithTriggerReason(CompactTriggerErrorRecovery)）。
	// 必须与 manual 分得开：前者是"我们主动压"，后者是"被上游逼着压"。
	CompactTriggerErrorRecovery = "error_recovery"
)

// CompactionFieldCoverage — 结构化摘要模板（compactSystemPrompt 的六段）的**逐字段存在与否**。
//
// ⚠ 弱判据（写死，勿当强证据用）：只判「这一段名在摘要里出现了没有」，**不**判内容对不对
// （内容级的约束保留率是 T3.2 的事）。六段一个都没匹配上 ⇒ 整块**缺席**（无法解析 ≠ 六段全缺，
// 与「拿不到就缺席、不编造」同源）。
type CompactionFieldCoverage struct {
	Goal            bool `json:"goal"`
	Progress        bool `json:"progress"`
	KeyDecisions    bool `json:"key_decisions"`
	RelevantFiles   bool `json:"relevant_files"`
	NextSteps       bool `json:"next_steps"`
	CriticalContext bool `json:"critical_context"`
	Present         int  `json:"present"` // 命中的段数（保留率的分子）
	Total           int  `json:"total"`   // 模板段数（保留率的分母；= 六段）
}

// CompactionObs — 三态事件共用的压缩事实块（T3.1）。
//
// 三个状态各带什么（读侧按这个表判读，勿凭字段名猜）：
//
//	started   ：tokens_before / messages_before / turns_summarized / depth / 两个 id 集合 /
//	            trigger_reason / summarizer_model —— 终态量**全部缺席**（此刻还不知道，不编造）
//	completed ：上面的全部 + tokens_after / messages_after / compression_ratio / summary_sha256 /
//	            field_coverage（仅结构化摘要）/ latency_ms
//	failed    ：上面的全部（除摘要相关） + latency_ms + err_text（错误原文，≤300 字）
type CompactionObs struct {
	// ── started 起就确定（裸值：0 就是测到的 0）──
	TokensBefore    int     `json:"tokens_before"`              // 压缩前估算 token（与触发阈值**同一估算器** ⇒ 可直接对照）
	MessagesBefore  int     `json:"messages_before"`            // 压缩前活动消息条数
	TurnsSummarized int     `json:"turns_summarized"`           // 被压段里的**轮数**（见 compactObsFactsOf 口径）
	Depth           int     `json:"compaction_depth"`           // 本会话第几次压缩（含失败尝试，见 obsNextCompactDepth）
	SummarizedIDs   []int64 `json:"summarized_message_ids"`     // 被摘要（软归档）的消息 id —— 只给 id，不落原文
	KeptIDs         []int64 `json:"kept_message_ids"`           // 保留（未参与摘要）的消息 id —— 与上行互补
	TriggerReason   string  `json:"trigger_reason"`             // token_budget | manual | error_recovery
	SummarizerModel string  `json:"summarizer_model,omitempty"` // 本次摘要用的模型（拿不到 ⇒ 缺席）

	// ── 只有终态才知道（指针：缺席 = 未知）──
	TokensAfter      *int                     `json:"tokens_after,omitempty"`      // 压缩后估算 token（保留段 + 本次产出的摘要）
	MessagesAfter    *int                     `json:"messages_after,omitempty"`    // 压缩后活动消息条数（保留段 + 1 条摘要）
	CompressionRatio *float64                 `json:"compression_ratio,omitempty"` // = tokens_before / tokens_after（见 completed 注释）
	SummarySHA256    string                   `json:"summary_sha256,omitempty"`    // 落库摘要正文的 sha256（见 completed 注释）
	FieldCoverage    *CompactionFieldCoverage `json:"field_coverage,omitempty"`    // 结构化六段覆盖（拿不到 ⇒ 缺席）
	LatencyMS        *int64                   `json:"latency_ms,omitempty"`        // started → 终态 的墙上时间
	ErrText          string                   `json:"err_text,omitempty"`          // 失败原文（trunca ≤300，与 turn.err_text 同口径）
}

// compactObsFacts — 三态事件的**公共事实**（在「确定要压」那一刻组装一次，终态原样复用）。
//
// 为什么在这里算而不是在终态算：tokens_before / 两个 id 集合描述的是**压缩前的布局**——
// 终态时库里已经被改过（软归档 + 插入摘要）⇒ 只有起点才算得准。
type compactObsFacts struct {
	Session string
	Model   string // 会话模型（本路径摘要模型同源，见 compactObsFactsOf 注释）

	Trigger         string
	MessagesBefore  int
	TokensBefore    int
	KeptTokens      int // 保留段的 token（终态算 tokens_after 用）
	TurnsSummarized int
	KeptIDs         []int64
	SummarizedIDs   []int64
}

// compactObsHandle — 一次压缩的三态句柄（started 已落；completed/failed 二选一，只能落一次）。
type compactObsHandle struct {
	f        compactObsFacts
	depth    int
	start    time.Time
	done     bool // 终态已落标记 ⇒ 同一个原子里不会有第二条终态
	traceID  string
	spanID   string
	parentID string
}

// ── compaction_depth（第几次压缩）──────────────────────────────────────
//
// 口径（写死）：**进程内**按会话计数，每次「真的开始压」（即每发出一条 compaction_started）加一，
// **含失败尝试** —— 失败的尝试也是第 N 次压缩；把它排除会把「同一段反复压」
// （H13 明列的病态：循环压缩）读成一帆风顺。
//
// 局限（如实，不掩盖）：进程重启即归零；跨重启的耐久真值请看 <state>/compact_journal.jsonl
// （成功压缩的召回指针账）与 <state>/compact_cooldown.json 的 streak（连续失败）。表有界
// （超限淘汰最久未出现的会话）—— 长跑守护进程里会话数无界，内存不可无界。
const obsCompactDepthSessionsMax = 4096

type obsCompactDepthEntry struct {
	depth int
	seq   int64
}

var (
	obsCompactDepthMu  sync.Mutex
	obsCompactDepthSeq int64
	obsCompactDepthTab = map[string]*obsCompactDepthEntry{}
)

// obsNextCompactDepth — 取（并 +1）该会话的压缩序次。无会话 ⇒ 返回 1 且不写表（不编造跨会话共享计数）。
func obsNextCompactDepth(session string) int {
	if session == "" {
		return 1
	}
	obsCompactDepthMu.Lock()
	defer obsCompactDepthMu.Unlock()
	e := obsCompactDepthTab[session]
	if e == nil {
		if len(obsCompactDepthTab) >= obsCompactDepthSessionsMax {
			obsCompactDepthEvictOldestLocked()
		}
		e = &obsCompactDepthEntry{}
		obsCompactDepthTab[session] = e
	}
	obsCompactDepthSeq++
	e.seq = obsCompactDepthSeq
	e.depth++
	return e.depth
}

// obsCompactDepthEvictOldestLocked — 表满淘汰最久未出现者。调用方须持 obsCompactDepthMu。
func obsCompactDepthEvictOldestLocked() {
	var oldestKey string
	var oldestSeq int64 = -1
	for k, v := range obsCompactDepthTab {
		if oldestSeq < 0 || v.seq < oldestSeq {
			oldestKey, oldestSeq = k, v.seq
		}
	}
	if oldestKey != "" {
		delete(obsCompactDepthTab, oldestKey)
	}
}

// obsCompactionStart — 落 **compaction_started** 并返回句柄。
//
// ⚠ 调用时机就是语义：只能在**确定要压**之后调用（阈值已过 / 冷却闸已过 / 中间段非空）。
// 提前到 MaybeCompact 入口处会造出「不压缩也落事件」的误报 —— 那正是本族事件要消灭的盲区的镜像。
func obsCompactionStart(f compactObsFacts) *compactObsHandle {
	h := &compactObsHandle{
		f:        f,
		depth:    obsNextCompactDepth(f.Session),
		start:    time.Now(),
		traceID:  "",
		spanID:   "",
		parentID: "",
	}
	// 挂点：当前轮（无轮次 ⇒ 会话根）；**不推进**轮次指针（父链只由轮次推进 —— 见 obsCurrentSpan）
	h.traceID, h.parentID = obsCurrentSpan(f.Session)
	h.spanID = obsNewID() // 三态共用同一个 span：成对/悬空可校验（口径 ④）
	obsWrite(h.rec(obsEventCompactionStarted))
	return h
}

// rec — 三态记录的唯一构造点（字段口径只写一处；终态量由调用方另行填）
func (h *compactObsHandle) rec(event string) ObsRecord {
	kept, summ := h.f.KeptIDs, h.f.SummarizedIDs
	if kept == nil {
		kept = []int64{} // nil 会被编码成 null；空集合应落成 []（读侧不必区分 null 与空）
	}
	if summ == nil {
		summ = []int64{}
	}
	return ObsRecord{
		Kind:    "compact", // 沿用既有 kind（读侧按 kind=compact 的老过滤照样命中本族事件）
		Session: h.f.Session,
		Model:   h.f.Model,
		// T1.1 追踪骨架：三态共用同一 span（started 与终态成对可校验）；span_kind=internal ——
		// 压缩是进程内的一个步骤，不是处理外部请求（那是 turn 的 server）、也不是向外发的调用。
		TraceID:      h.traceID,
		SpanID:       h.spanID,
		ParentSpanID: h.parentID,
		SpanKind:     ObsSpanKindInternal,
		StartedAtMS:  h.start.UnixMilli(), // 真起点（started 那一刻）——不是落盘时刻
		EventName:    event,
		Compaction: &CompactionObs{
			TokensBefore:    h.f.TokensBefore,
			MessagesBefore:  h.f.MessagesBefore,
			TurnsSummarized: h.f.TurnsSummarized,
			Depth:           h.depth,
			SummarizedIDs:   summ,
			KeptIDs:         kept,
			TriggerReason:   h.f.Trigger,
			SummarizerModel: h.f.Model,
		},
	}
}

// completed — 落终态 **compaction_completed**（成功把布局改造落地之后才允许调用）。
//
// 参数:
//   - summary:  摘要器产出的**正文**（不含「【历史摘要】」角色前缀）
//   - inserted: 实际插入会话的那段文本（正文 + 尾部召回指针）——见 MaybeCompact 的调用点
//   - fromStructuredSummarizer: 摘要是否来自 LLM 结构化摘要器（LLMLingua-2 删除式不是结构化的 ⇒ 覆盖缺席）
//
// 三个终态量的口径（写死，读侧据此复算）:
//
//	tokens_after     = 保留段 token + estimateTokens(inserted)（与 tokens_before 同一估算器；
//	                   触发判定其实是按消息逐条加总的 ⇒ 这里也逐条加，不引入第二种口径）
//	messages_after   = len(kept_message_ids) + 1（保留段 + 插入的那条摘要；软归档的消息已不在活动窗口）
//	compression_ratio = tokens_before / tokens_after（>1 ⇒ 确实压下去了；1 ⇒ 没缩小；<1 ⇒ 变大了）
//	                   tokens_after=0 时**缺席**（不可除；不编造 ∞ 也不写 0）
//	summary_sha256   = sha256(inserted) 的十六进制全串 —— 对**落库的那段文本**取指纹，
//	                   读侧拿库里的摘要消息（去掉「【历史摘要】」前缀）就能逐字复算
func (h *compactObsHandle) completed(summary, inserted string, fromStructuredSummarizer bool) {
	if h == nil || h.done {
		return // 一个原子里只有一条终态（重复调用被吞掉）
	}
	h.done = true
	rec := h.rec(obsEventCompactionCompleted)
	cp := rec.Compaction

	after := h.f.KeptTokens + estimateTokens(inserted)
	msgsAfter := len(h.f.KeptIDs) + 1
	lat := time.Since(h.start).Milliseconds()
	sum := sha256.Sum256([]byte(inserted))

	cp.TokensAfter = &after
	cp.MessagesAfter = &msgsAfter
	cp.LatencyMS = &lat
	cp.SummarySHA256 = hex.EncodeToString(sum[:])
	if after > 0 {
		r := math.Round(float64(h.f.TokensBefore)/float64(after)*10000) / 10000 // 4 位小数：够复算，又不啰嗦
		cp.CompressionRatio = &r
	}
	if fromStructuredSummarizer {
		cp.FieldCoverage = compactionCoverageOf(summary) // 匹配不上 ⇒ nil ⇒ 整块缺席（不编造六段全缺）
	}
	// 旧 OBS-4 兼容字段：老读侧按 result/summary_chars 找成败 ⇒ 新行必须照样答得上
	rec.Result = "ok"
	rec.SummaryChars = len([]rune(summary))
	rec.Cause = h.f.Trigger
	rec.EndedAtMS = time.Now().UnixMilli() // 终态时刻（骨架只填空缺；这里显式给「压缩结束」那一刻）
	obsWrite(rec)
}

// failed — 落终态 **compaction_failed**（摘要失败 / 布局落地失败都算；errText 为错误原文）。
//
// 为什么失败也必须有终态行：没有它，「压了但没成」与「根本没压」同形 —— 冷却/熔断链上
// 每一次失败都只活在日志里，长跑分析看不见（这正是 H1 的盲区）。
func (h *compactObsHandle) failed(errText string) {
	if h == nil || h.done {
		return
	}
	h.done = true
	rec := h.rec(obsEventCompactionFailed)
	cp := rec.Compaction
	lat := time.Since(h.start).Milliseconds()
	cp.LatencyMS = &lat
	// 错误原文：≤300 字（trunca 与 turn.err_text 同一写法、同一上限）；截断标记自带说明
	cp.ErrText = trunca(errText, 300)
	// 旧 OBS-4 兼容字段（同 completed 的理由）
	rec.Result = "fail"
	rec.FailReason = trunca(errText, 300)
	rec.Cause = h.f.Trigger
	rec.EndedAtMS = time.Now().UnixMilli()
	obsWrite(rec)
}

// ── 结构化摘要的六段存在性（弱判据）────────────────────────────────────

// compactionSectionSpec — 模板段名。只认**规范写法**（英文标题 / 对应中文段名），不做模糊匹配 ——
// 宁可漏判（缺席），不把散文里的普通词读成"这一段在"。
type compactionSectionSpec struct {
	jsonKey string
	names   []string
}

var compactionSectionSpecs = []compactionSectionSpec{
	{jsonKey: "goal", names: []string{"goal", "目标"}},
	{jsonKey: "progress", names: []string{"progress", "进展"}},
	{jsonKey: "key_decisions", names: []string{"key decisions", "关键决策"}},
	{jsonKey: "relevant_files", names: []string{"relevant files", "相关文件"}},
	{jsonKey: "next_steps", names: []string{"next steps", "下一步"}},
	{jsonKey: "critical_context", names: []string{"critical context", "关键上下文"}},
}

// compactionCoverageOf — 逐字段判定模板六段是否出现在摘要里。
// 返回 nil 的两种情形（都必须缺席，不能写"全 false"）：摘要为空、六段一个都没匹配上（=解析不出来）。
func compactionCoverageOf(summary string) *CompactionFieldCoverage {
	if strings.TrimSpace(summary) == "" {
		return nil
	}
	low := strings.ToLower(summary)
	cov := &CompactionFieldCoverage{Total: len(compactionSectionSpecs)}
	for _, spec := range compactionSectionSpecs {
		for _, n := range spec.names {
			if !strings.Contains(low, strings.ToLower(n)) {
				continue
			}
			cov.Present++
			switch spec.jsonKey {
			case "goal":
				cov.Goal = true
			case "progress":
				cov.Progress = true
			case "key_decisions":
				cov.KeyDecisions = true
			case "relevant_files":
				cov.RelevantFiles = true
			case "next_steps":
				cov.NextSteps = true
			case "critical_context":
				cov.CriticalContext = true
			}
			break
		}
	}
	if cov.Present == 0 {
		return nil // 解析不出来 ⇒ 缺席（不编造"六段全缺"）
	}
	return cov
}

// compactSummaryFingerprint — summary_sha256 的独立入口（供读侧/用例按同一口径复算）
func compactSummaryFingerprint(inserted string) string {
	s := sha256.Sum256([]byte(inserted))
	return hex.EncodeToString(s[:])
}
