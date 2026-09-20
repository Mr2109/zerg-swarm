// chat_compact.go — v2.5.7 对话上下文压缩（C4——借鉴 Hermes 参数：threshold 0.5 / protect_last_n 20 / protect_first_n 3）
// 策略: 历史超阈值时——保留首 3 条 + 尾 20 条——中间旧消息调网关 /v1/context/compact 生成摘要
// 摘要插在保留段之后（role=system——对话模板 system 须开头——llama-server Jinja 要求 system 唯一且在开头）
// 注: llama-server 要求 system 唯一且在开头——摘要用 role=user 的【历史摘要】标记（兼容多轮）

package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/tracectx"
)

// 压缩参数（Hermes config.yaml 实测——2026-08-29——threshold 0.5/protect_last_n 20/protect_first_n 3）
// 对话场景: example-35b-v2 ctx 262144 太大——0.5=131K token 对话永远达不到——用活跃窗口目标值 8000 token
// （≈20 条长消息——超过即压缩中间段——响应速度优先）
// P4-39 升级: 比例阈值 + per-model——触发 = max(min(ctx×ThresholdRatio, ActiveWindow), MinWindow)
//
//	example-35b-v2 262K  → min(131K, 8000) = 8000（活跃窗口优先——现状不变）
//	8K 小模型    → min(4K, 8000)   = 4000（窗口小提前压——本地小模型关键）
const (
	CompactThresholdRatio = 0.5  // 阈值比例（模型上下文窗口 × 0.5）
	CompactProtectLastN   = 20   // 保护最近 N 条
	CompactProtectFirstN  = 3    // 保护最早 N 条
	CompactMinMessages    = 30   // 最少消息数才压缩（防频繁压缩小会话）
	CompactActiveWindow   = 8000 // 对话活跃窗口目标 token（超此压缩——对话实用阈值）
	CompactMinWindow      = 2000 // 触发下限（窗口再小也不低于此）
)

// compactModelCtxs — 模型→上下文窗口注册表（main 启动时从 fleet.yaml 构建）
// 未注册模型默认 example-35b-v2 262144（Mr2109默认）
var compactModelCtxs = map[string]int{
	"example-35b-v2": 262144,
	"example-35b": 262144,
	"Qwen3.8-27B":    32768,
	"qwen3.8-27b":    32768,
	"gemma-12B":      8192,
}

// RegisterModelCtx — 注册模型上下文窗口（main 启动时从 fleet.yaml 调）
func RegisterModelCtx(model string, ctx int) {
	if ctx > 0 {
		compactModelCtxs[model] = ctx
	}
}

// CompactTriggerTokens — 计算模型的实际压缩触发阈值（比例 + 活跃窗口 + 下限）
func CompactTriggerTokens(model string) int {
	ctx := compactModelCtxs[model]
	if ctx <= 0 {
		ctx = 262144
	}
	t := int(float64(ctx) * CompactThresholdRatio)
	if t > CompactActiveWindow {
		t = CompactActiveWindow
	}
	if t < CompactMinWindow {
		t = CompactMinWindow
	}
	return t
}

// chatCompactReq — 压缩请求体
type chatCompactReq struct {
	SessionID string        `json:"session_id"`
	Messages  []interface{} `json:"messages"`
}

// ShouldCompact — 判断历史是否需要压缩（token 估算 + 消息数双条件——P4-39 按模型阈值）
func ShouldCompact(msgs []Message, model string) bool {
	if len(msgs) < CompactMinMessages {
		return false
	}
	return compactOverThreshold(msgs, model)
}

// ShouldCompactPtr — 指针版（Store 返回 []*Message）
func ShouldCompactPtr(msgs []*Message, model string) bool {
	if len(msgs) < CompactMinMessages {
		return false
	}
	out := make([]Message, len(msgs))
	for i, m := range msgs {
		out[i] = *m
	}
	return compactOverThreshold(out, model)
}

// estimateTokens — 按内容中英比例估 token（2026-09-05: 原 len(rune)/2 一刀切对中文偏高估
// ——llama 系分词中文≈0.7 字/token、英文≈0.25 字/token——估准压缩触发时机偏差从 ±40% 收窄）
func estimateTokens(s string) int {
	if s == "" {
		return 0
	}
	cjk := 0
	total := 0
	for _, r := range s {
		total++
		if r >= 0x4e00 && r <= 0x9fff {
			cjk++
		}
	}
	ascii := total - cjk
	return int(float64(cjk)/0.7) + ascii/4 // 中文 1.43 token/字 ÷ 系数表述——即 0.7 字/token；英文 4 字符/token
}

// compactMessageTokens — 单条消息计入压缩阈值的 token 估算（内容 + 思考 + 工具调用）。
//
// 触发判定（compactOverThreshold）与观测（T3.1 事件的 tokens_before/tokens_after）**共用**它
// ⇒ 事件里的 token 数与阈值同一口径，可直接对照（"离阈值还有多远"不必再换算）。
func compactMessageTokens(m Message) int {
	n := estimateTokens(m.Content)
	if m.Reasoning != "" {
		n += estimateTokens(m.Reasoning)
	}
	if m.ToolCalls != "" {
		n += estimateTokens(m.ToolCalls)
	}
	return n
}

// compactOverThreshold — token 估算超阈值（P4-39 含工具 JSON——2026-09-05 用 estimateTokens）
func compactOverThreshold(msgs []Message, model string) bool {
	trigger := CompactTriggerTokens(model)
	total := 0
	for _, m := range msgs {
		total += compactMessageTokens(m) // 与观测同一估算器（见 compactMessageTokens 注释）
	}
	return float64(total) > float64(trigger)
}

// compactSystemPrompt — P4-39 T3: 结构化摘要模板（6 段——Hermes 8 段简化——小模型按结构生成可靠）
const compactSystemPrompt = `你是对话摘要器——把对话压缩为结构化中文摘要。要求:
1. 保留精确值（文件路径/命令/数字/名称/错误信息）——不编造
2. 按以下结构输出:

## Goal
用户想达成什么

## Progress
- 已完成: 
- 进行中: 
- 阻塞: 

## Key Decisions
重要决策及原因

## Relevant Files
相关文件及说明

## Next Steps
下一步

## Critical Context
精确值/错误信息/配置细节

如果提供了【旧摘要】——基于旧摘要更新（把已完成的事移入 Done——补充新进展——不要重写全部）`

// compactResponseFormat — 摘要的受约束解码形状（六段 = 提示词里那六个 `## ` 段）。
//
// 为什么用它（§九 M6「结构性输出不许靠提示词约定」· §二十一 第 11 条「全仓 `response_format` = 0 文件」）：
// 摘要的段名是**契约**（下游按段名取值），而段名今天只写在 `compactSystemPrompt` 的文本里 ⇒
// 模型少写一段、改名一段，下游只会「取不到值」，且看不出来是模型的问题。
// 这里把同一组段名**同时**放进 `response_format`（结构面）与提示词（文本面）⇒ 两边同源。
//
// 兼容口径（诚实说明）：`response_format` 是 OpenAI 兼容面的**可选**成员 ——
// 支持 `json_schema` 的服务端会强约束；不支持的服务端按未知成员忽略（本函数**不**改请求的其余部分，
// 也不在客户端做「服务端不支持就报错」的判 —— 那是 §十二 待拍项，本版只保证**请求面**有落点）。
func compactResponseFormat() map[string]any {
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "zerg_compaction_summary",
			"strict": true,
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"goal":           map[string]any{"type": "string"},
					"progress":       map[string]any{"type": "string"},
					"key_decisions":  map[string]any{"type": "string"},
					"relevant_files": map[string]any{"type": "string"},
					"next_steps":     map[string]any{"type": "string"},
					"critical_context": map[string]any{
						"type": "string",
					},
				},
				"required":             []string{"goal", "progress", "key_decisions", "relevant_files", "next_steps", "critical_context"},
				"additionalProperties": false,
			},
		},
	}
}

// CompactRequest — P4-39 T3: 本地调摘要模型（/v1/chat/completions + 结构化模板——不走网关 compact 端点——模板可控）
// T4: 存在旧摘要（【历史摘要】消息）→ 输入带旧摘要——指示更新而非重写
func CompactRequest(ctx context.Context, gatewayURL, authToken, sessionID, model string, msgs []Message) (string, error) {
	// 中间段转 chat messages（保持 role/content——摘要要理解对话脉络）
	chatMsgs := make([]map[string]any, 0, len(msgs)+3)
	chatMsgs = append(chatMsgs, map[string]any{"role": "system", "content": compactSystemPrompt})
	// T4 迭代更新: 查找旧摘要（【历史摘要】开头消息）——优先带上
	for _, m := range msgs {
		if strings.HasPrefix(m.Content, "【历史摘要】") && len(m.Content) > 30 {
			chatMsgs = append(chatMsgs, map[string]any{"role": "system", "content": "【旧摘要（请更新而非重写）】\n" + m.Content})
		}
	}
	chatMsgs = append(chatMsgs, map[string]any{"role": "system", "content": "【待压缩对话】"})
	for _, m := range msgs {
		if strings.HasPrefix(m.Content, "【历史摘要】") {
			continue // 旧摘要不是本次压缩源
		}
		chatMsgs = append(chatMsgs, map[string]any{"role": m.Role, "content": m.Content})
	}

	payload, _ := json.Marshal(map[string]any{
		"model":       model,
		"messages":    chatMsgs,
		"max_tokens":  3000,
		"temperature": 0.3,
		// 受约束解码（§九 M6 · §二十一 第 11 条）：摘要的形状是**固定六段**，
		// 靠提示词约定等于把结构性输出押在模型心情上 ⇒ 在**实现面**用
		// `response_format` 把六段锁成必填字段。服务端支持则强约束，不支持则忽略
		// （`response_format` 是 OpenAI 兼容面的**可选**成员，不是必需成员）。
		"response_format": compactResponseFormat(),
	})

	req, err := http.NewRequestWithContext(ctx, "POST", gatewayURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("chat: failed to build compaction request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("X-Auth-Token", authToken)
	}
	// T1.6 传播：压缩调用（本地摘要模型）同样是这条链上的一跳 ⇒ 带 traceparent + 会话 baggage。
	tracectx.Propagate(req.Header, nil, sessionID, tracectx.ReplayMarked())
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat: compaction call failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("chat: compaction returned %d: %s", resp.StatusCode, string(b))
	}
	raw, _ := io.ReadAll(resp.Body)
	var obj struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("chat: failed to parse compaction response: %w", err)
	}
	if len(obj.Choices) == 0 || obj.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("chat: compaction returned an empty summary")
	}
	summary := strings.TrimSpace(obj.Choices[0].Message.Content)
	if len([]rune(summary)) < 50 {
		return "", fmt.Errorf("chat: summary too short (%d chars) — treated as failure", len([]rune(summary)))
	}
	return summary, nil
}

// CompactMessages — 计算压缩后的消息布局（返回: 摘要文本 + 需压缩的消息 id 集合）
// 规则（Hermes 借鉴）: 首 3 条 + 尾 20 条保留——中间生成摘要
func CompactMessages(msgs []Message) (summarySrc []Message, compressIDs []int64) {
	n := len(msgs)
	if n <= CompactProtectFirstN+CompactProtectLastN+1 {
		return nil, nil // 没得压
	}
	// 中间段 = [firstN, n-lastN)
	start := CompactProtectFirstN
	end := n - CompactProtectLastN
	for i := start; i < end; i++ {
		summarySrc = append(summarySrc, msgs[i])
		compressIDs = append(compressIDs, msgs[i].ID)
	}
	return summarySrc, compressIDs
}
