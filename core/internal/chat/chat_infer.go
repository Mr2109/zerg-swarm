// chat_infer.go — v2.5.7 对话推理（直接调网关 8082——轻量——不进 agent 子进程——Mr2109）
// 决策: 对话优先模型槽——对话活跃时任务排队（网关已有排队机制）
// C3: 流式支持（SSE——token 逐字——借鉴 Hermes/Codex）

package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"zerg/core/internal/agent"
)

// ChatInfer — 对话推理器（网关客户端）
type ChatInfer struct {
	GatewayURL string // http://127.0.0.1:8082
	AuthToken  string // X-Auth-Token
	client     *http.Client
}

// NewChatInfer — 创建推理器
func NewChatInfer(gatewayURL, authToken string) *ChatInfer {
	return &ChatInfer{
		GatewayURL: gatewayURL,
		AuthToken:  authToken,
		client:     &http.Client{Timeout: 120 * time.Second},
	}
}

// InferResult — 推理结果（思考 + 正文分离——Hermes/Claude 式）
type InferResult struct {
	Content         string `json:"content"`
	Reasoning       string `json:"reasoning"`
	InputTokens     int    `json:"input_tokens"`
	OutputTokens    int    `json:"output_tokens"`
	ReasoningTokens int    `json:"reasoning_tokens"`
	// C4b 工具循环: 响应含 tool_calls 时填充（chat 格式 choices[].message.tool_calls）
	ToolCalls []agent.ToolCall `json:"tool_calls,omitempty"`
}

// Infer — 单轮推理（历史 + 新用户消息 → 响应）
// msgs: 已有消息（role/content 顺序——含工具链）
// tools: 可选（chat 格式 tools 参数——C4b 工具循环用——缺省不带工具）
// 返回: 思考 + 正文（reasoning 分离——适配器铁律: 思考不能关——effort low 控深度）
// 2026-08-30 修正: 统一走 /v1/chat/completions——llama-server 流式 responses 对历史
// assistant 消息有 bug（output 空）——chat 格式字符串 content 历史完全兼容（实测）
func (c *ChatInfer) Infer(ctx context.Context, model string, sysPrompt string, msgs []map[string]any,
	tools ...[]map[string]any) (*InferResult, error) {
	// 组装 messages：system + 历史消息（chat 格式——字符串 content）
	msgsAll := make([]map[string]any, 0, len(msgs)+1)
	msgsAll = append(msgsAll, map[string]any{"role": "system", "content": sysPrompt})
	msgsAll = append(msgsAll, msgs...)

	body := map[string]any{
		"model":     model,
		"messages":  msgsAll,
		"stream":    false, // C3 流式走 InferStream——这里保持非流式（C2 兼容）
		"reasoning": map[string]any{"effort": "low"}, // 思考不能关——low 控深度（Mr2109）
	}
	if len(tools) > 0 && len(tools[0]) > 0 {
		// P4-50 修复: __temp__ 是温度标记（收尾轮 0.3）——不是真 tools——必须剥离（X3 拒绝 Missing tool type）
		realTools := make([]map[string]any, 0, len(tools[0]))
		for _, t := range tools[0] {
			if _, isTemp := t["__temp__"]; isTemp {
				continue
			}
			realTools = append(realTools, t)
		}
		if len(realTools) > 0 {
			body["tools"] = realTools
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("chat: 序列化失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.GatewayURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("chat: 构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.AuthToken != "" {
		req.Header.Set("X-Auth-Token", c.AuthToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat: 调网关失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("chat: 网关返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("chat: 读响应失败: %w", err)
	}
	if os.Getenv("ZERG_DEBUG") != "" {
		fmt.Printf("[chat:infer] status=%d raw_len=%d raw_head=%s\n", resp.StatusCode, len(raw), truncateArgs(string(raw), 200))
	}
	return parseChatResult(raw)
}

// InferStream — 流式推理（SSE——reasoning/content 逐段回调）
// onDelta: 回调（type: reasoning/output + text）——nil 回调 = 丢弃
// tools: 可选（chat 格式 tools 参数——D2 流式工具循环——缺省不带）
// 返回: 完整结果（流结束后——含 ToolCalls——新版 llama-server 流式推 tool_calls 完整 JSON）
// 2026-08-30 修正: 走 /v1/chat/completions 流式——llama-server responses 流式对历史
// assistant 消息有 bug（output 空——实测）——chat 格式流式历史兼容 + reasoning_content 思考分离
func (c *ChatInfer) InferStream(ctx context.Context, model string, sysPrompt string, msgs []map[string]any,
	onDelta func(deltaType, text string), tools ...[]map[string]any) (*InferResult, error) {
	// P4-47 Hermes 规范: 温度 0.0（采样方差破坏 tool_call 内 JSON——fast.io 官方: Any sampling variance above zero can corrupt the JSON structure inside tool_call tags）
	// P4-48 收尾轮温度: tools 里带 "__temp__" 特殊项 → 覆盖（收尾轮 0.3——最终回答多样性——工具轮恒 0.0）
	toolTemp := 0.0
	if len(tools) > 0 {
		for _, t := range tools[0] {
			if v, ok := t["__temp__"].(float64); ok {
				toolTemp = v
			}
		}
	}
	msgsAll := make([]map[string]any, 0, len(msgs)+1)
	msgsAll = append(msgsAll, map[string]any{"role": "system", "content": sysPrompt})
	msgsAll = append(msgsAll, msgs...)

	body := map[string]any{
		"model":     model,
		"messages":  msgsAll,
		"stream":    true,
		"reasoning": map[string]any{"effort": "low"},
	}
	body["temperature"] = toolTemp
	if len(tools) > 0 && len(tools[0]) > 0 {
		// P4-50 修复: __temp__ 是温度标记（收尾轮 0.3）——不是真 tools——必须剥离（X3 拒绝 Missing tool type）
		realTools := make([]map[string]any, 0, len(tools[0]))
		for _, t := range tools[0] {
			if _, isTemp := t["__temp__"]; isTemp {
				continue
			}
			realTools = append(realTools, t)
		}
		if len(realTools) > 0 {
			body["tools"] = realTools
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("chat: 序列化失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.GatewayURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("chat: 构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.AuthToken != "" {
		req.Header.Set("X-Auth-Token", c.AuthToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat: 调网关失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("chat: 网关返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	// 解析 chat SSE 流（data: {...delta...}——D2 流式 tool_calls 完整 JSON）
	res := &InferResult{}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		var ev struct {
			Choices []struct {
				Delta struct {
					Role            string         `json:"role"`
					Content         string         `json:"content"`
					ReasoningContent string        `json:"reasoning_content"`
					ToolCalls       []chatToolCall `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		if len(ev.Choices) == 0 {
			continue
		}
		d := ev.Choices[0].Delta
		if d.ReasoningContent != "" {
			res.Reasoning += d.ReasoningContent
			res.ReasoningTokens += len([]rune(d.ReasoningContent)) / 4
			if onDelta != nil {
				onDelta("reasoning", d.ReasoningContent)
			}
		}
		if d.Content != "" {
			res.Content += d.Content
			if onDelta != nil {
				// P4-45 XML 工具调用不推正文（example-35b-v2 训练格式——content 流式是 XML 标签）
				if !strings.Contains(res.Content, "<tool_call>") {
					onDelta("output", d.Content)
				}
			}
		}
		// D2 流式 tool_calls（新版 llama-server 完整 JSON——非增量）
		for _, tc := range d.ToolCalls {
			args := parseToolArgs(tc.Function.Arguments)
			res.ToolCalls = append(res.ToolCalls, agent.ToolCall{
				ID: tc.ID, Name: tc.Function.Name, Args: args, RawArgs: tc.Function.Arguments,
			})
		}
		if ev.Usage.CompletionTokens > 0 {
			res.OutputTokens = ev.Usage.CompletionTokens
		}
	}
	// P4-45 适配器原则（Mr2109: 不同模型不同标准——适配器负责转换）
	// example-35b-v2 训练格式 = XML 工具调用（模板要求 <tool_call><function=X><parameter=K>V</parameter></function></tool_call>）
	// 模型输出 XML（正确行为）——系统只认 JSON tool_calls → 适配器补 XML→JSON 转换
	if len(res.ToolCalls) == 0 && strings.Contains(res.Content, "<tool_call>") {
		xmlCalls := parseXMLToolCalls(res.Content)
		if len(xmlCalls) > 0 {
			res.ToolCalls = xmlCalls
			res.Content = stripXMLToolCalls(res.Content)
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return res, fmt.Errorf("chat: 流读取失败: %w", err)
	}
	res.OutputTokens = len([]rune(res.Content)) / 2
	return res, nil
}

// parseChatResultTolerant — 容错解析（X3 llama-server 对含 tool_calls 响应固定截断——issue-3889 同类）
// 截断点在 timings/usage 处——content/reasoning_content/tool_calls 已完整——字符串提取
// 标准解析失败时调用（故障自愈——不因尾部截断丢整个响应）
func parseChatResultTolerant(raw []byte) *InferResult {
	s := string(raw)
	res := &InferResult{}
	// content（跳过 null）
	if i := strings.Index(s, `"content":"`); i >= 0 {
		if v := extractJSONString(s[i+len(`"content":"`):]); v != nil {
			res.Content = *v
		}
	}
	// reasoning_content
	if i := strings.Index(s, `"reasoning_content":"`); i >= 0 {
		if v := extractJSONString(s[i+len(`"reasoning_content":"`):]); v != nil {
			res.Reasoning = *v
			res.ReasoningTokens = len([]rune(res.Reasoning)) / 4
		}
	}
	// tool_calls（name + arguments）
	for _, tc := range extractToolCallsTolerant(s) {
		args := map[string]any{}
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		res.ToolCalls = append(res.ToolCalls, agent.ToolCall{
			ID: tc.ID, Name: tc.Function.Name, Args: args, RawArgs: tc.Function.Arguments,
		})
	}
	// P4-45 XML fallback（example-35b-v2 训练格式——content 里 <tool_call>）
	if len(res.ToolCalls) == 0 && strings.Contains(res.Content, "<tool_call>") {
		if xmlCalls := parseXMLToolCalls(res.Content); len(xmlCalls) > 0 {
			res.ToolCalls = xmlCalls
			res.Content = stripXMLToolCalls(res.Content)
		}
	}
	res.OutputTokens = len([]rune(res.Content)) / 2
	return res
}

// extractJSONString — 从偏移处提取 JSON 字符串值（处理 \uXXXX 转义）
func extractJSONString(s string) *string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			n := s[i+1]
			if n == 'n' {
				b.WriteByte('\n')
			} else if n == 't' {
				b.WriteByte('\t')
			} else if n == 'r' {
				b.WriteByte('\r')
			} else if n == '"' {
				b.WriteByte('"')
			} else if n == '\\' {
				b.WriteByte('\\')
			} else if n == 'u' && i+6 <= len(s) {
				// 解析 \uXXXX
				var code int
				if _, err := fmt.Sscanf(s[i+2:i+6], "%04x", &code); err == nil {
					b.WriteRune(rune(code))
					i += 6
					continue
				}
			} else {
				b.WriteByte(n)
			}
			i += 2
			continue
		}
		if c == '"' {
			v := b.String()
			return &v
		}
		b.WriteByte(c)
		i++
	}
	return nil
}

// extractToolCallsTolerant — 字符串扫描提取 tool_calls 数组（截断容错）
// JSON 结构: {"function":{"arguments":"...","name":"ls"},"id":"...","type":"function"}
// type:function 在对象末尾——id/name/arguments 全在其前——统一向前找
func extractToolCallsTolerant(s string) []chatToolCall {
	var out []chatToolCall
	for {
		i := strings.Index(s, `"type":"function"`)
		if i < 0 {
			break
		}
		tc := chatToolCall{Type: "function"}
		head := s[:i]
		// 向后找 id（id 在 type 前）
		if j := strings.LastIndex(head, `"id":"`); j >= 0 {
			tc.ID = firstJSONString(head[j+6:])
		}
		// name（在 arguments 后——从 id 位置向后的范围找）
		if j := strings.LastIndex(head, `"name":"`); j >= 0 {
			tc.Function.Name = firstJSONString(head[j+8:])
		}
		// arguments（嵌套转义最复杂——从 function 开始找）
		if j := strings.LastIndex(head, `"arguments":"`); j >= 0 {
			tc.Function.Arguments = firstJSONString(head[j+13:])
		}
		out = append(out, tc)
		s = s[i+len(`"type":"function"`):]
	}
	return out
}

// firstJSONString — 提取首个 JSON 字符串值（不处理嵌套转义——够用）
func firstJSONString(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			n := s[i+1]
			if n == '"' {
				b.WriteByte('"')
			} else if n == '\\' {
				b.WriteByte('\\')
			} else if n == 'n' {
				b.WriteByte('\n')
			} else if n == 'u' && i+6 <= len(s) {
				var code int
				if _, err := fmt.Sscanf(s[i+2:i+6], "%04x", &code); err == nil {
					b.WriteRune(rune(code))
					i += 6
					continue
				}
			} else {
				b.WriteByte(n)
			}
			i += 2
			continue
		}
		if c == '"' {
			return b.String()
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// parseChatResult — 解析 chat completions 响应（content + reasoning_content 分离 + tool_calls）
func parseChatResult(raw []byte) (*InferResult, error) {
	var obj struct {
		Choices []struct {
			Message struct {
				Role            string          `json:"role"`
				Content         string          `json:"content"`
				ReasoningContent string         `json:"reasoning_content"`
				ToolCalls       []chatToolCall  `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		// X3 截断容错（tool_calls 响应在 usage 处断——content/tool_calls 完整）
		tol := parseChatResultTolerant(raw)
		if tol.Content != "" || len(tol.ToolCalls) > 0 || tol.Reasoning != "" {
			return tol, nil
		}
		return nil, fmt.Errorf("chat: 解析响应失败: %w", err)
	}
	res := &InferResult{}
	if len(obj.Choices) > 0 {
		m := obj.Choices[0].Message
		res.Content = m.Content
		res.Reasoning = m.ReasoningContent
		if res.Reasoning != "" {
			res.ReasoningTokens = len([]rune(res.Reasoning)) / 4
		}
		// C4b 工具调用（chat 格式 tool_calls）
		for _, tc := range m.ToolCalls {
			args := parseToolArgs(tc.Function.Arguments)
			res.ToolCalls = append(res.ToolCalls, agent.ToolCall{
				ID: tc.ID, Name: tc.Function.Name, Args: args, RawArgs: tc.Function.Arguments,
			})
		}
	}
	// P4-45 XML fallback（example-35b-v2 训练格式——content 里 <tool_call>）
	if len(res.ToolCalls) == 0 && strings.Contains(res.Content, "<tool_call>") {
		if xmlCalls := parseXMLToolCalls(res.Content); len(xmlCalls) > 0 {
			res.ToolCalls = xmlCalls
			res.Content = stripXMLToolCalls(res.Content)
		}
	}
	res.InputTokens = obj.Usage.PromptTokens
	res.OutputTokens = obj.Usage.CompletionTokens
	if res.OutputTokens == 0 && res.Content != "" {
		res.OutputTokens = len([]rune(res.Content)) / 2
	}
	return res, nil
}

// parseToolArgs — 解析 tool_calls arguments（P4-34 双重转义修复）
// llama-server 流式 arguments 是"被 JSON 转义的 JSON 字符串"（{\"command\":\"ls -la\"}）——
// 单层 Unmarshal 失败（args 空 map → 工具"参数为空"）——先按 JSON 字符串解一层还原真正的 JSON 再解析
func parseToolArgs(raw string) map[string]any {
	args := map[string]any{}
	if raw == "" {
		return args
	}
	if err := json.Unmarshal([]byte(raw), &args); err == nil {
		return args // 正常路径（单层 JSON）
	}
	// 双重转义路径：raw 是 JSON 字符串字面量——解一层得真正 JSON 文本
	var inner string
	if err2 := json.Unmarshal([]byte(raw), &inner); err2 == nil {
		if err3 := json.Unmarshal([]byte(inner), &args); err3 == nil {
			return args
		}
	}
	// 兜底：剥离字面反斜杠（\" → "）
	cleaned := strings.ReplaceAll(raw, `\"`, `"`)
	if err4 := json.Unmarshal([]byte(cleaned), &args); err4 == nil {
		return args
	}
	return args // 解析失败——空参数（工具侧会报参数缺失）
}


// ═══════════════ P4-45 XML 工具调用解析（适配器原则——example-35b-v2 训练格式）═══════════════

// parseXMLToolCalls — 解析模型 XML 工具调用（P4-46/47 适配器原则——多格式兼容）
// 格式1（Hermes JSON——官方）: <tool_call>{"name": "X", "arguments": {"k": "v"}}</tool_call>
// 格式2（多对象并行——模型坏格式）: <tool_call>{"function": "X", "arguments": {...}}, {"name": "Y", ...}</tool_call>——括号平衡逐个提取
// 格式3（旧模板——纯 XML）: <tool_call><function=NAME><parameter=KEY>VALUE</parameter></function></tool_call>
func parseXMLToolCalls(content string) []agent.ToolCall {
	var out []agent.ToolCall
	reCall := regexp.MustCompile(`(?s)<tool_call>(.*?)</tool_call>`)
	for _, m := range reCall.FindAllStringSubmatch(content, -1) {
		inner := strings.TrimSpace(m[1])
		// 格式1: 单个 Hermes JSON 对象
		var obj struct {
			Name      string         `json:"name"`
			Function  string         `json:"function"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(inner), &obj); err == nil && (obj.Name != "" || obj.Function != "") {
			name := obj.Name
			if name == "" {
				name = obj.Function // 兼容 "function" 键（模型并行调用坏格式）
			}
			// 空 arguments 不跳过——无参数工具合法（task_list/fleet_status）——参数有效性由执行层判断
			rawArgs, _ := json.Marshal(obj.Arguments)
			out = append(out, agent.ToolCall{Name: name, Args: obj.Arguments, RawArgs: string(rawArgs), RawCall: m[0]})
			continue
		}
		// 格式2: 多对象（逗号分隔/数组——括号平衡逐个提取）
		objs := extractJSONObjects(inner)
		if len(objs) > 0 {
			parsedAny := false
			for _, rawObj := range objs {
				var o2 struct {
					Name      string         `json:"name"`
					Function  string         `json:"function"`
					Arguments map[string]any `json:"arguments"`
				}
				if err := json.Unmarshal([]byte(rawObj), &o2); err != nil {
					continue
				}
				name := o2.Name
				if name == "" {
					name = o2.Function
				}
				if name == "" {
					continue
				}
				rawArgs, _ := json.Marshal(o2.Arguments)
				out = append(out, agent.ToolCall{Name: name, Args: o2.Arguments, RawArgs: string(rawArgs), RawCall: m[0]})
				parsedAny = true
			}
			if parsedAny {
				continue
			}
		}
		// 格式3: 纯 XML（旧模板）
		reFn := regexp.MustCompile(`<function=([^>\s]+)>`)
		reParam := regexp.MustCompile(`(?s)<parameter=([^>]+)>(.*?)</parameter>`)
		fm := reFn.FindStringSubmatch(inner)
		if len(fm) < 2 {
			continue
		}
		name := strings.TrimSpace(fm[1])
		args := map[string]any{}
		for _, p := range reParam.FindAllStringSubmatch(inner, -1) {
			key := strings.TrimSpace(p[1])
			val := strings.TrimSpace(p[2])
			args[key] = val
		}
		if len(args) == 0 {
			continue // 无参数——无效调用
		}
		rawArgs, _ := json.Marshal(args)
		out = append(out, agent.ToolCall{Name: name, Args: args, RawArgs: string(rawArgs), RawCall: m[0]})
	}
	return out
}

// extractJSONObjects — 括号平衡提取字符串里的所有 JSON 对象（处理逗号分隔的多对象——坏格式兼容）
func extractJSONObjects(s string) []string {
	var out []string
	depth := 0
	inStr := false
	esc := false
	start := -1
	for i, r := range s {
		switch {
		case inStr:
			if esc {
				esc = false
			} else if r == '\\' {
				esc = true
			} else if r == '"' {
				inStr = false
			}
		case r == '"':
			inStr = true
		case r == '{':
			if depth == 0 {
				start = i
			}
			depth++
		case r == '}':
			depth--
			if depth == 0 && start >= 0 {
				out = append(out, s[start:i+1])
				start = -1
			}
		}
	}
	return out
}

// stripXMLToolCalls — 从 content 剥离 XML 工具调用（剩纯正文）
func stripXMLToolCalls(content string) string {
	reCall := regexp.MustCompile(`(?s)<tool_call>.*?</tool_call>\s*`)
	return strings.TrimSpace(reCall.ReplaceAllString(content, ""))
}
