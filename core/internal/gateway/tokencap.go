// tokencap.go —— ctx_window / max_tokens 的**声明袋**与输出上限钳位（2026-09-19 ④）。
//
// 现象（真机）：`gateway/fleet.yaml` 该条已声明 `ctx_window: 524288, max_tokens: 262144`；主控日志
// 只有 `temperature override 0.60` 与 `timeout override 300s`，**从来没有**
// `max_tokens override 262144` ⇒ 声明是**死字段**，ctx/2 那条上限无人守。
//
// 病灶（三处都读同一个"声明袋"`res`，而袋里没有这两个键）：
//
//	gateway.go:764  `if res, ok := out.Result.(map[string]any); ok {`   ← 声明袋的构造处（现在也在这里装 fleet 声明）
//	gateway.go:776  动态额度分支（读 res["max_tokens"]）
//	gateway.go:782  ctx 来源（读 res["ctx_window"]）
//	gateway.go:812  **无条件覆盖** max_tokens（本次删掉的那一段）
//
// 语义（Mr2109 2026-09-19 定：ctx 是卵的属性 / max_tokens 是每次调用的事）：
//
//	把 fleet 的 ctx_window 与 max_tokens 装进声明袋；max_tokens 从"无条件覆盖"改成**上限钳位**：
//	  调用方给了且 ≤ 上限  ⇒ 原样放行（不再是"适配器说了算"）；
//	  调用方没给          ⇒ 用默认（现有动态额度 DynamicMaxTokens）；
//	  调用方给了但 > 上限 ⇒ 钳到上限 + 日志 `max_tokens clamped: want=… cap=…`。
//	上限来源优先级：**档案（事实）> 声明（fleet）> 默认**（与「闸门只读实测档案」同一条理）。
//
// 上限只约束**上界**（不许把调用方想要的小额度放大），ctx 仍按
// 档案（事实）⇒ 声明（意图）⇒ 保守默认 三态取值，三态都必须能在日志里分清。
package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/gateway/adapter"
	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
)

// maxTokensCapDefault 谁都没声明时的默认上限：沿用 outbudget.go 的单次输出上限
// （防"一次吐到天荒地老"；它是**上限**口径，不是默认额度——默认额度仍是动态算出来的）。
const maxTokensCapDefault = maxOutCeiling

// ProfileMaxTokens 读卵实测档案里的 max_tokens（**事实**）。无档案/无该字段/坏值 ⇒ (0,false)，
// 由调用方按优先级回落（声明 → 默认）。与 ProfileCtxWindow / ProfileFirstTokenSec 同一读法。
func ProfileMaxTokens(model string) (int, bool) {
	return profileIntField(model, "max_tokens")
}

// FleetDeclared 从 fleet 配置取该模型声明的 (ctx_window, max_tokens)。
//
// 同一模型可能有多个候选（不同 host/量化）⇒ 取**各候选的最大声明**：
// 声明是"这台机器允许的上界"，多候选里任何一个声明的上界都不该被另一个候选压小。
// 别名（Aliases）也认（调用方在别名解析之后拿到的是标准名，这里再兜一层，防上游漏解析）。
// 配置为 nil / 找不到 ⇒ (0, 0)（如实：没声明）。
func FleetDeclared(cfg *config.FleetConfig, model string) (int, int) {
	if cfg == nil || model == "" {
		return 0, 0
	}
	lookup := model
	if c, ok := cfg.Aliases[model]; ok && strings.TrimSpace(c) != "" {
		lookup = c
	}
	cands, ok := cfg.Models[lookup]
	if !ok {
		cands = cfg.Models[model]
	}
	ctx, mt := 0, 0
	for _, c := range cands {
		if c.CtxWindow > ctx {
			ctx = c.CtxWindow
		}
		if c.MaxTokens > mt {
			mt = c.MaxTokens
		}
	}
	return ctx, mt
}

// TokenLimit 一次请求的输出额度裁决结果（供日志、观测与用例）。
type TokenLimit struct {
	Value   int    // 最终写进请求体的值
	Source  string // 调用方(原样) / 默认(动态额度) / 钳位(上限)
	Want    int    // 调用方给的值（0 = 没给）
	Cap     int    // 生效上限
	Clamped bool   // 是否发生了钳位（值被压到上限）
}

// ClampMaxTokens 上限钳位（**④ 的唯一一处钳位逻辑**，纯函数）：
//
//	want > 0 且 want ≤ cap ⇒ 原样
//	want > 0 且 want > cap ⇒ 钳到 cap，Clamped=true
//	want ≤ 0（没给）        ⇒ 用 def（默认=动态额度）；def 超过 cap 同样钳到 cap
//
// cap ≤ 0 ⇒ 用 maxTokensCapDefault（默认上限）；def < 0 ⇒ 按 0（不许负额度）。
func ClampMaxTokens(want, cap, def int) TokenLimit {
	if cap <= 0 {
		cap = maxTokensCapDefault
	}
	if want > 0 {
		if want > cap {
			return TokenLimit{Value: cap, Source: "钳位(上限)", Want: want, Cap: cap, Clamped: true}
		}
		return TokenLimit{Value: want, Source: "调用方(原样)", Want: want, Cap: cap}
	}
	if def < 0 {
		def = 0
	}
	if def > cap {
		return TokenLimit{Value: cap, Source: "默认(动态额度→钳到上限)", Cap: cap, Clamped: true}
	}
	return TokenLimit{Value: def, Source: "默认(动态额度)", Cap: cap}
}

// RequestMaxTokens 调用方在请求体里给的输出额度（0 = 没给）。
//
// 两种格式都认（chat 的 `max_tokens` / responses 的 `max_output_tokens`）；两个都给 ⇒ 取**较大**者
// （它们是同一件事的两种写法，取大不会因为一个旧字段把调用方的意图缩小）。解析不了 ⇒ 0（如实）。
func RequestMaxTokens(body []byte) int {
	if len(body) == 0 {
		return 0
	}
	var probe struct {
		MaxTokens       *json.Number `json:"max_tokens"`
		MaxOutputTokens *json.Number `json:"max_output_tokens"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return 0
	}
	best := 0
	for _, n := range []*json.Number{probe.MaxTokens, probe.MaxOutputTokens} {
		if n == nil {
			continue
		}
		v, err := n.Int64()
		if err != nil || v <= 0 {
			continue
		}
		if int(v) > best {
			best = int(v)
		}
	}
	return best
}

// TokenBudgetInput 输出预算裁决的入参（结构化：全是 int，位置参数极易写反）。
type TokenBudgetInput struct {
	Model       string
	ForwardBody []byte
	CtxWindow   int    // 已按优先级取好的上下文（档案 > 声明 > 适配器 > 0）
	CtxSource   string // 来源三态（日志必须能分清）
	Cap         int    // 输出上限（档案 > 声明(fleet) > 默认）
	CapSource   string
}

// TokenBudgetResult 裁决产物。
type TokenBudgetResult struct {
	Body      []byte // 已写回 max_tokens / max_output_tokens 的请求体
	Limit     TokenLimit
	Dyn       int // 动态额度（默认值来源；≤0 表示提示已超上下文）
	CtxUsed   int
	PromptEst int
}

// ApplyTokenBudget 一次请求的输出额度裁决 + 写回请求体（④ 的核心；**纯逻辑**，副作用只有日志）。
//
// 写回两种格式（chat 的 max_tokens / responses 的 max_output_tokens —— v2.5.6 的教训：
// 只写一个字段会让调度器那条路形同虚设）；Value ≤ 0（没额度可给）⇒ 一个字段都不动（如实：
// 不编一个额度，交回引擎自己的默认）。
func ApplyTokenBudget(in TokenBudgetInput) TokenBudgetResult {
	dyn, ctxUsed, promptEst := DynamicMaxTokens(in.Model, in.CtxWindow, EstimatePromptTokens(len(in.ForwardBody)))
	limit := ClampMaxTokens(RequestMaxTokens(in.ForwardBody), in.Cap, dyn)

	res := TokenBudgetResult{Body: in.ForwardBody, Limit: limit, Dyn: dyn, CtxUsed: ctxUsed, PromptEst: promptEst}
	if limit.Clamped {
		// 钳位**必须**留痕（真机症状正是"声明被静默丢弃、没人看得出上限没生效"）。
		log.Printf("⚠️ adapter %s: max_tokens clamped: want=%d cap=%d（上限来源=%s；调用方给=%d 动态额度=%d）",
			in.Model, limit.Want, limit.Cap, in.CapSource, limit.Want, dyn)
	}
	if dyn <= 0 {
		log.Printf("⚠️ 输出预算：提示已超上下文（ctx=%d 来源=%s prompt≈%d）——本次不给输出额度"+
			"（调用方原值仍按上限口径放行；不再无条件覆盖）", ctxUsed, in.CtxSource, promptEst)
	}
	if limit.Value <= 0 {
		return res
	}
	res.Body = setBodyInt(in.ForwardBody, "max_tokens", limit.Value)
	res.Body = setBodyInt(res.Body, "max_output_tokens", limit.Value)
	log.Printf("🎛️ adapter %s: max_tokens = %d（来源=%s；上限=%d[%s] 调用方给=%d 动态额度=%d；ctx=%d[来源=%s] prompt≈%d）",
		in.Model, limit.Value, limit.Source, limit.Cap, in.CapSource, limit.Want, dyn, ctxUsed, in.CtxSource, promptEst)
	return res
}

// ── 声明袋与适配器覆盖（④；原 gateway.go:757-827 的整段抽到这里）────────────────────

// ApplyAdapterOverrides 适配器参数覆盖 + 输出预算裁决（④）。
//
// 为什么从请求处理器里抽出来：这一段（温度 / ctx / max_tokens / reasoning_effort / 超时）原来内联在
// 处理器里，而 ④ 的核心改动（max_tokens 不再无条件覆盖、改成**上限钳位**）**必须能单独验收** ——
// 内联在处理器里就只能靠端到端起假后端来测（那层测的是路由与转发，不是钳位）。
//
// **声明袋（res）的构造处就在这里**：`out.Result.(map[string]any)`（原 gateway.go:764）。fleet 的
// ctx_window / max_tokens 在这一步被装进袋子 —— 装之前它们是**死字段**（fleet.yaml 里写了，
// 处理器却读不到，真机日志里从来没有 max_tokens 相关的一行）。
//
// 兼容口径（写死）：**没有任何 max_tokens 声明**的模型（适配器不声明、fleet 不声明、档案也没有）
// 行为与改动前**逐字一致**（不进预算分支）——本次不动它们的额度口径。
func (g *Gateway) ApplyAdapterOverrides(model string, forwardBody []byte, ada plugin.Plugin) []byte {
	out, aerr := ada.Execute(plugin.PluginInput{Data: map[string]any{"model": model}})
	if aerr != nil {
		// v2.5.6 错误码设计（2026-08-29）: 适配器执行失败不能静默——记日志（覆盖失败用默认参数——不阻塞请求）
		log.Printf("⚠️ adapter %s execution failed (using defaults): %v", model, aerr)
	}
	if aerr != nil || out.Result == nil {
		return forwardBody
	}
	res, ok := out.Result.(map[string]any)
	if !ok {
		return forwardBody
	}
	// 温度/采样参数覆盖
	if t, ok := res["temperature"].(float64); ok {
		forwardBody = adapter.JsonSetField(forwardBody, "temperature", t)
		log.Printf("🎛️ adapter %s: temperature override %.2f", model, t)
	}
	// ④①：声明袋装 fleet 的两格（ctx_window 供动态额度取"声明(意图)"档；max_tokens 作**上限**）。
	// 为什么可以覆盖袋子里的同名键：袋子是"这次请求的声明面"，而 fleet 是这台机器对**这个模型的
	// 部署真源**（适配器配置是另一处声明，两者冲突时以部署声明为准 —— 报告 ④ 的优先级：
	// 档案(事实) > 声明(fleet) > 默认）。
	fleetCtx, fleetMax := FleetDeclared(g.config, model)
	if fleetCtx > 0 {
		res["ctx_window"] = fleetCtx
	}
	if fleetMax > 0 {
		res["max_tokens"] = fleetMax
	}
	// max_tokens 预算分支：只要袋子里有正数声明就进（适配器声明 或 fleet 声明）。
	if mt0, ok0 := res["max_tokens"].(int); ok0 && mt0 > 0 {
		ctxDecl, ctxSrc := g.resolveCtxWindow(model, res)
		capTokens, capSrc := resolveTokenCap(model, fleetMax)
		log.Printf("🎛️ adapter %s: max_tokens 声明值 %d（**上限**口径：调用方 ≤ 上限原样放行 / 没给用动态额度 / 超了钳到 %d[%s]）",
			model, mt0, capTokens, capSrc)
		bud := ApplyTokenBudget(TokenBudgetInput{
			Model:       model,
			ForwardBody: forwardBody,
			CtxWindow:   ctxDecl,
			CtxSource:   ctxSrc,
			Cap:         capTokens,
			CapSource:   capSrc,
		})
		forwardBody = bud.Body
	}
	// reasoning_effort 覆盖（思考深度——适配器声明——Mr2109 low）
	if re, ok := res["reasoning_effort"].(string); ok && re != "" {
		forwardBody = adapter.JsonSetField(forwardBody, "reasoning_effort", re)
	}
	// 超时覆盖（按模型——Qwen3.8 120s / Nemotron 60s——适配器声明）
	if ts, ok := res["timeout_sec"].(int); ok && ts > 0 {
		g.setRequestTimeout(model, ts)
	}
	return forwardBody
}

// resolveCtxWindow 上下文来源三态（④）：档案(事实) > 声明(意图，已在声明袋里) > 默认。
// 实测教训：X3 的 Qwen 配置声明 1M 而实跑 -c 262144 ⇒ 信声明会算错额度，故档案优先。
func (g *Gateway) resolveCtxWindow(model string, res map[string]any) (int, string) {
	ctx, src := 0, "默认（无档案、无声明）"
	if cw, ok := res["ctx_window"].(int); ok && cw > 0 {
		ctx, src = cw, "声明(意图)"
	}
	if v, ok := ProfileCtxWindow(model); ok && v > 0 {
		ctx, src = v, "档案(事实)"
	}
	if ctx == 0 {
		src = "默认（卵未声明）"
	}
	return ctx, src
}

// resolveTokenCap 输出**上限**来源优先级（④）：档案(事实) > 声明(fleet) > 默认。
// 档案里没有 max_tokens 这一格（当前档案格式六字段）⇒ 如实回落到声明/默认，不编造。
func resolveTokenCap(model string, fleetMax int) (int, string) {
	if v, ok := ProfileMaxTokens(model); ok && v > 0 {
		return v, "档案(事实)"
	}
	if fleetMax > 0 {
		return fleetMax, "声明(fleet)"
	}
	return maxTokensCapDefault, "默认"
}

// setBodyInt 把整数字段写进 JSON 请求体（失败 ⇒ 原样返回；调用方不需知道细节，但必须能看出来
// "没写进去"——所以这里把错误打进日志，绝不静默）。
func setBodyInt(body []byte, field string, v int) []byte {
	out, err := jsonSetIntField(body, field, v)
	if err != nil {
		log.Printf("⚠️ 写回请求体字段 %s=%d 失败（请求按原样转发）：%v", field, v, err)
		return body
	}
	return out
}

// jsonSetIntField 写回一个整数字段（保持其余字段原样；解析失败 ⇒ 报错，不猜）。
func jsonSetIntField(body []byte, field string, v int) ([]byte, error) {
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return body, fmt.Errorf("请求体不是 JSON 对象：%w", err)
	}
	obj[field] = v
	return json.Marshal(obj)
}
