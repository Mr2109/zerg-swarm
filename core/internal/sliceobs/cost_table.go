// cost_table.go — B 项 **B10（其二）**：**四指标同表可聚合**（设计稿 v2.1 §3.1 第 131 行 + §6.3 第 380 行）。
//
// ── 逐字来源（本件现读稿面，不凭记忆）────────────────────────────────────
//
//	docs/01-设计/设计-协作骨架-v2.1.md 第 131 行（§3.1 切片者输入可见集 + 成本账）：
//
//	  「**成本**：切片者在 **Mr2109** 跑、执行者在 **X3** 跑 ⇒ 物理分开、不抢显存；成本表除显存外记
//	    「**调用次数 / token / 墙钟**」，并把「**分解模型 vs 执行模型算力分配**」登记为**实验变元**……」
//
//	同一文件第 380 行（§6.3 成本与预算可追溯）：
//
//	  「成本表记「显存 + **调用次数 / token / 墙钟**」；全部预算项标注**定层**（档案／声明／默认），
//	    与既有 ctx 三态同规；预算数值来自标定档案……闸门只读档案。」
//
//	同一文件第 469 行（改造采纳表）：
//
//	  「……· 成本表（显存→显存+调用+token+墙钟）。（落点：§4.8 / §5.6-3 / §3.5+§6.1 / §3.1+§6.3）」
//
//	⇒ 四项**名字**是稿面字面：**显存 / 调用次数 / token / 墙钟**（本件不增删第五项）。
//	  B10 的验收判据（任务表-协作骨架v2.1仓内核对结论-20260918.md:85 逐字）：「四指标**同表可聚合**
//	  （脚本可复算）；每项带 `definition_layer`；闸门**只读档案**（写侧无路径）」。
//
// ── 数据来源（**只读**；本件不改这些件的任何一行）─────────────────────────
//
//	· token / 墙钟 / 调用次数：`~/.zerg/state/chat_obs.jsonl`（core/internal/chat/obs.go 的既有落点）；
//	  键名逐字现读：kind=obs.go:118 / session=:119 / tool 记录=:142 / tokens_in,tokens_out=:133-134 /
//	  gen_ai.usage.input_tokens,output_tokens=:249-251 / turn.total_ms=:80（obs.go:136 自述
//	  「压缩耗时（turn 的耗时在 turn.total_ms）」）。
//	  本件**只读键名**（自带一个最小解析结构），**不 import chat** ⇒ 不成环、零回归。
//	· 显存：core/internal/api/resources_ledger.go:59-63 的资源台账（vram_known / vram_total_gb /
//	  vram_used_gb / vram_free_gb）。本件**不解析台账、不猜是哪一个字段** ⇒ 由调用方把读数传进来
//	  （`*float64`，GB）；台账没给 ⇒ nil ⇒ 该指标**未标定**。
//	· 档案读数（定层那一半）：复用同包 B 项⑥ 的 `DefaultSliceCalibPath` / `calibReadErr*`，
//	  **不另造第二个真源**；口径见 budget_layer.go。
//
// ── 聚合口径（**登记口径**：稿面只给四项的名字，没给聚合口径 ⇒ 写死在这里 + 用例钉住 + 回报登记）──
//
//	① **调用次数** = `kind=="tool"` 的记录条数（一行 tool 记录 = 一次工具调用）；
//	② **token** = Σ(gen_ai.usage.input_tokens + gen_ai.usage.output_tokens)，**缺席的记录不计、不补 0**；
//	   同一行两套键都在时**取 gen_ai.usage**（obs.go:249 自述那是「**计费口径**」）；
//	③ **墙钟** = Σ `turn.total_ms`（obs.go:136 自述 turn 的耗时就在这个键上）；
//	④ **没读数 ⇔ 未标定**：整个输入里**一条**记录都没带该项的键 ⇒ 该项 `nil` ⇒ 文本「未标定」；
//	   空输入集 ⇒ 四项全 nil（**空的输入不是「实测到 0」**）。**数的 0 是定值**（真数出来的 0 次调用
//	   照写 `0`），**没读到的 0 不许冒充**；
//	⑤ 口径外的 `kind` 名（不属于 obs.go:118 自述的 `turn|compact|tool`）**单列计数**（`OtherKind`），
//	   不静默、也不计进调用次数；
//	⑥ 输出**确定性**：`BySession` 按 `session` 升序（空 id 排最前）；同输入必得同输出（可复算）；
//	   不读时间、不改入参（`samples` 只读；`calib` 按值传）。
//
// ── 硬规则（✗ 不做）────────────────────────────────────────────────────────
//
//	· **不生成**任何事件、**不写**任何文件（写侧无路径）—— 本件只有读入口（`LoadCostSamplesFrom`）
//	  与纯函数（`AggregateCost`）；
//	· **不发明**第五个指标名、**不发明**档案键名（见 budget_layer.go 文件头）；
//	· 显存按会话拆不出来（机器级读数）⇒ `BySession` 行的显存**恒 nil**（写清原因，不硬凑）。
package sliceobs

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"strconv"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// ── 输入的三种 kind（obs.go:118 注释逐字：turn | compact | tool）────────────
const (
	obsKindTurn    = "turn"
	obsKindCompact = "compact"
	obsKindTool    = "tool"
)

// CostSample — 一条观测记录里**可计入成本表**的读数（键名逐字取自 core/internal/chat/obs.go）。
//
// 三项读数都是**指针**：nil = 这条记录没带该键（缺就是缺，不是 0）。
type CostSample struct {
	// Kind — obs.go:118 `kind`（turn | compact | tool）。
	Kind string
	// Session — obs.go:119 `session`（空 = 该记录没带会话 id ⇒ 归入空 id 组，单列不静默）。
	Session string
	// TokensIn — 输入 token（gen_ai.usage.input_tokens 优先，无则 tokens_in）；nil = 没读数。
	TokensIn *int64
	// TokensOut — 输出 token（gen_ai.usage.output_tokens 优先，无则 tokens_out）；nil = 没读数。
	TokensOut *int64
	// WallMS — 该记录的墙钟耗时（毫秒；取 turn.total_ms）；nil = 没读数。
	WallMS *int64
}

// CostMetrics — **四指标一式**（§3.1 P131 / §6.3 P380 逐字：显存 + 调用次数 / token / 墙钟）。
//
// 四项都是指针：nil = **未标定**（这项没有读数），不是 0。
type CostMetrics struct {
	// VRAMGb — 显存（GB）。来源 = 资源台账；nil = 台账没给 ⇒ 未标定。
	VRAMGb *float64 `json:"vram_gb,omitempty"`
	// Calls — 调用次数（= kind=="tool" 的记录条数）。nil = 空输入集（不是「实测到 0 次」）。
	Calls *int `json:"calls,omitempty"`
	// Tokens — token（输入+输出）。nil = 没有任何记录带 token 键 ⇒ 未标定。
	Tokens *int64 `json:"tokens,omitempty"`
	// WallMS — 墙钟（毫秒）。nil = 没有任何记录带 turn.total_ms ⇒ 未标定。
	WallMS *int64 `json:"wall_ms,omitempty"`
}

// textOrUncalibrated — 指标文本化的**唯一实现点**（nil ⇒ 「未标定」，绝不写 0 或空串冒充）。
func textOrUncalibrated(v *float64) string {
	if v == nil {
		return UncalibratedText
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// VRAMText — 显存的可读形态（数字 GB / 「未标定」）。
func (m CostMetrics) VRAMText() string { return textOrUncalibrated(m.VRAMGb) }

// CallsText — 调用次数的可读形态（数字 / 「未标定」）。
func (m CostMetrics) CallsText() string { return int64TextOrUncalibrated(ptrToInt64(m.Calls)) }

// TokensText — token 的可读形态（数字 / 「未标定」）。
func (m CostMetrics) TokensText() string { return int64TextOrUncalibrated(m.Tokens) }

// WallText — 墙钟的可读形态（毫秒 / 「未标定」）。
func (m CostMetrics) WallText() string { return int64TextOrUncalibrated(m.WallMS) }

// ColumnTexts — 四指标**同表**的一行文本（顺序固定 = 稿面顺序：显存 / 调用次数 / token / 墙钟）。
func (m CostMetrics) ColumnTexts() []string {
	return []string{m.VRAMText(), m.CallsText(), m.TokensText(), m.WallText()}
}

// CostRow — 成本表的一行（某一聚合单元的四个指标）。
type CostRow struct {
	// Session — 聚合键（obs.go:119 的 session；空串 = 该组记录没带会话 id）。
	Session string `json:"session"`
	// Metrics — 该单元的四个指标（显存恒 nil：机器级读数不按会话切，见文件头）。
	Metrics CostMetrics `json:"metrics"`
}

// CostTable — **一张成本表**：四指标（总 + 按会话）+ 全部预算项的定层（§6.3 的两句话同表）。
type CostTable struct {
	// Total — 全部输入的四个指标。
	Total CostMetrics `json:"total"`
	// BySession — 按会话拆开（`session` 升序，稳定可复现）。
	BySession []CostRow `json:"by_session,omitempty"`
	// OtherKind — 口径外 kind 名的记录条数（单列不静默；不计进调用次数）。
	OtherKind int `json:"other_kind,omitempty"`
	// Calib — 闸门只读的标定档案读数（每个预算项带 `definition_layer`）。
	Calib BudgetCalib `json:"calib"`
}

// Calibrated — 整表是否有来源（四指标**都**有读数 **且** 预算项**都**有定层）。
//
// 缺一项 ⇒ false ⇒ 调用方不得把这张表当「已标定」用。
func (t CostTable) Calibrated() bool {
	return t.Total.VRAMGb != nil && t.Total.Calls != nil && t.Total.Tokens != nil &&
		t.Total.WallMS != nil && t.Calib.Calibrated()
}

// AggregateCost — **纯函数**：把观测读数聚合成一张成本表（四指标同表 + 定层）。
//
// 口径见文件头 ①–⑥；**不读时间、不读 IO、不生成事件、不改入参**。
//
// vramGb = 资源台账给的显存读数（GB）；nil 或非法值（负数/NaN/Inf）⇒ 该指标「未标定」。
// calib = 闸门只读的档案读数（按值传入，见 `LoadBudgetCalib`）。
func AggregateCost(samples []CostSample, vramGb *float64, calib BudgetCalib) CostTable {
	table := CostTable{Calib: calib}

	totalTokens, hasTokens := int64(0), false
	totalWall, hasWall := int64(0), false
	perSession := map[string]*sessionAcc{}

	for _, s := range samples {
		switch s.Kind {
		case obsKindTurn, obsKindCompact, obsKindTool:
		default:
			table.OtherKind++
		}
		if s.TokensIn != nil {
			totalTokens += *s.TokensIn
			hasTokens = true
		}
		if s.TokensOut != nil {
			totalTokens += *s.TokensOut
			hasTokens = true
		}
		if s.WallMS != nil {
			totalWall += *s.WallMS
			hasWall = true
		}
		acc := perSession[s.Session]
		if acc == nil {
			acc = &sessionAcc{}
			perSession[s.Session] = acc
		}
		acc.add(s)
	}

	table.Total.VRAMGb = nonNegative(vramGb)
	if hasTokens {
		v := totalTokens
		table.Total.Tokens = &v
	}
	if hasWall {
		v := totalWall
		table.Total.WallMS = &v
	}
	// 空输入集 ⇒ 调用次数 nil（「空的输入不是『实测到 0』」）；非空 ⇒ 照数出来的 0 写。
	if len(samples) > 0 {
		calls := 0
		for _, s := range samples {
			if s.Kind == obsKindTool {
				calls++
			}
		}
		table.Total.Calls = &calls
	}

	if len(perSession) > 0 {
		ids := make([]string, 0, len(perSession))
		for id := range perSession {
			ids = append(ids, id)
		}
		sort.Strings(ids) // 空 id 排最前（"" < 任何非空串）；确定性输出
		table.BySession = make([]CostRow, 0, len(ids))
		for _, id := range ids {
			table.BySession = append(table.BySession, CostRow{
				Session: id,
				Metrics: perSession[id].metrics(),
			})
		}
	}
	return table
}

// sessionAcc — 一个会话的累加器（内部件；不导出）。
type sessionAcc struct {
	samples   int
	calls     int
	tokens    int64
	hasTokens bool
	wall      int64
	hasWall   bool
}

// add — 把一条记录累加进本会话（只读 s）。
func (a *sessionAcc) add(s CostSample) {
	a.samples++
	if s.Kind == obsKindTool {
		a.calls++
	}
	if s.TokensIn != nil {
		a.tokens += *s.TokensIn
		a.hasTokens = true
	}
	if s.TokensOut != nil {
		a.tokens += *s.TokensOut
		a.hasTokens = true
	}
	if s.WallMS != nil {
		a.wall += *s.WallMS
		a.hasWall = true
	}
}

// metrics — 本会话的四个指标（显存恒 nil：机器级读数不按会话切）。
func (a *sessionAcc) metrics() CostMetrics {
	m := CostMetrics{}
	if a.samples > 0 {
		calls := a.calls
		m.Calls = &calls
	}
	if a.hasTokens {
		v := a.tokens
		m.Tokens = &v
	}
	if a.hasWall {
		v := a.wall
		m.WallMS = &v
	}
	return m
}

// ── 读入口（只读观测文件）────────────────────────────────────────────────

// DefaultCostObsPath — 成本表默认读的观测文件：`<statepath.Dir()>/chat_obs.jsonl`。
//
// **复用既有解析点**（`statepath.File`；文件名逐字取自 core/internal/chat/obs.go:69 `obsFileName`），
// **不写死私有路径**、不另造第二个落点。
func DefaultCostObsPath() string { return statepath.File("chat_obs.jsonl") }

// obsCostLine — chat_obs.jsonl 一行的最小解析结构（键名**逐字**取自 core/internal/chat/obs.go）。
type obsCostLine struct {
	Kind    string `json:"kind"`    // obs.go:118
	Session string `json:"session"` // obs.go:119
	// 压缩路径的计量（obs.go:133-134）
	TokensIn  *int `json:"tokens_in"`
	TokensOut *int `json:"tokens_out"`
	// 计费口径（obs.go:249-250；自述「计费口径；上游不给 ⇒ 缺席」）
	GenAIInputTokens  *int `json:"gen_ai.usage.input_tokens"`
	GenAIOutputTokens *int `json:"gen_ai.usage.output_tokens"`
	// 墙钟（obs.go:80 的 turn.total_ms）
	Turn *struct {
		TotalMS *int64 `json:"total_ms"`
	} `json:"turn"`
}

// LoadCostSamplesFrom — 从指定观测文件**只读**读出成本表样本（IO 入口，不是纯函数）。
//
// 返回 (samples, skipped, err)：
//   - `skipped` = **坏行**（JSON 解析不了）条数 —— 单列出来，不静默丢弃；
//   - 文件读不到 ⇒ err 非 nil 且 samples 为空（**不创建文件**、不 panic、不阻断）；
//   - 认不出的键一律忽略（**不猜**它们的含义）；缺键照缺（样本里保持 nil）。
func LoadCostSamplesFrom(path string) (samples []CostSample, skipped int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // 观测行可能很长（提示/几何块）
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var raw obsCostLine
		if jerr := json.Unmarshal(line, &raw); jerr != nil {
			skipped++
			continue
		}
		samples = append(samples, raw.toSample())
	}
	if serr := sc.Err(); serr != nil {
		return samples, skipped, serr
	}
	return samples, skipped, nil
}

// toSample — 落成成本表样本。
//
// token 取数口径（登记口径 ②）：**gen_ai.usage 优先**（obs.go:249 自述「计费口径」），
// 该键缺席时取 tokens_in / tokens_out（obs.go:133-134，压缩路径的计量）。
// 两套键在既有观测里**从不同时出现**（obs.go:103 自述「只挂在大模型调用那类记录上；
// 工具/压缩事件不带」）⇒ 这里不累加、不双计。
func (l obsCostLine) toSample() CostSample {
	s := CostSample{Kind: l.Kind, Session: l.Session}
	if l.GenAIInputTokens != nil {
		v := int64(*l.GenAIInputTokens)
		s.TokensIn = &v
	} else if l.TokensIn != nil {
		v := int64(*l.TokensIn)
		s.TokensIn = &v
	}
	if l.GenAIOutputTokens != nil {
		v := int64(*l.GenAIOutputTokens)
		s.TokensOut = &v
	} else if l.TokensOut != nil {
		v := int64(*l.TokensOut)
		s.TokensOut = &v
	}
	if l.Turn != nil && l.Turn.TotalMS != nil {
		v := *l.Turn.TotalMS
		s.WallMS = &v
	}
	return s
}

// ── 小工具（本包内私有）────────────────────────────────────────────────────

// ptrToInt64 — int 指针 → int64 指针（nil 透传）。
func ptrToInt64(v *int) *int64 {
	if v == nil {
		return nil
	}
	n := int64(*v)
	return &n
}

// int64TextOrUncalibrated — 读数的文本化（nil ⇒ 「未标定」，不写 0 冒充）。
func int64TextOrUncalibrated(v *int64) string {
	if v == nil {
		return UncalibratedText
	}
	return strconv.FormatInt(*v, 10)
}
