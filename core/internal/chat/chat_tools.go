// chat_tools.go — v2.5.7 对话轻量工具循环（C4b——决策定稿: 对话可用任何工具/skill/mcp）
// 复用 agent 工具链（DefaultTools + ExecContext.ExecuteTool——不重复造轮子）
// 流程: 调网关(chat 格式+tools) → 响应含 tool_calls → 执行 → tool 结果追加 → 再调 → 循环（最多 3 轮）
// 注意: 会话存储里 tool 消息落库（tool_calls JSON + tool_name）——C5 前端展示工具调用

package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"zerg/core/internal/agent"
)

// 工具循环上限（防失控——对话是交流——不是任务）
// MaxToolRounds — 工具循环最大轮数（P4-37 3→10——多步任务需多轮工具——Hermes agent loop 无硬上限）
const MaxToolRounds = 10

// ToolTrace — 工具调用记录（落库展示）
type ToolTrace struct {
	Round    int    `json:"round"`
	CallID   string `json:"call_id"`
	Name     string `json:"name"`
	Args     string `json:"args"`
	Result   string `json:"result"`
	Error    string `json:"error,omitempty"`
	Duration string `json:"duration,omitempty"`
}

// chatToolCall — chat 格式工具调用（llama-server 响应）
type chatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// RunToolLoop — 工具循环主入口（Infer 后调用——含 tool_calls 时）
// 返回: 最终推理结果 + 工具轨迹（落库）
// gate: 安全门（C7 危险命令黑名单接入——nil = 不拦截）
// rt: P4-50 渐进式常驻（nil = 老行为不记录——非 nil = 成功即常驻/exec 失败计数/错误桶）
func RunToolLoop(ctx context.Context, infer *ChatInfer, model, sysPrompt string,
	msgs []map[string]any, gate agent.ToolGater, rt *ToolRuntime) (*InferResult, []ToolTrace, error) {

	var traces []ToolTrace
	cur := msgs
	toolsWorkDir := "/tmp/zerg-chat/tools"
	// 2026-09-03 代码优化(skill 日志查漏): 工作区创建失败原静默——目录不存在 bash 工具全挂且无提示——返回错误
	if err := os.MkdirAll(toolsWorkDir, 0o755); err != nil { // 工具工作区（bash/read 等需要真实目录）
		return nil, nil, fmt.Errorf("chat: 工具工作区创建失败 %s: %w", toolsWorkDir, err)
	}
	ec := agent.NewExecContext(toolsWorkDir)
	ec.AgentName = "chat"
	tools := BuildToolsParam()

	for round := 1; round <= MaxToolRounds; round++ {
		// 调网关（带工具）
		res, err := infer.Infer(ctx, model, sysPrompt, cur, tools)
		if err != nil {
			return nil, traces, err
		}
		if len(res.ToolCalls) == 0 {
			// 无工具调用——最终回复
			return res, traces, nil
		}
		// 执行工具
		for _, tc := range res.ToolCalls {
			// P4-50 参数统一解包（模型 Hermes 风格嵌套——bash arguments 双层——见 NormalizeToolArgs）
			NormalizeToolArgs(&tc)
			argsJSON, _ := json.Marshal(tc.Args)
			var content, dur string
			var execErr error
			// P4-50 隐藏工具拦截（3 次 exec 失败后——本对话不再执行——除非 tool_search 类查询器）
			if rt != nil && rt.IsHidden(tc.Name) && tc.Name != "tool_search" {
				content = fmt.Sprintf("【系统】工具 %s 本对话已隐藏（连续 3 次执行失败）。请换其他工具或 tool_search 搜索替代。", tc.Name)
			} else if tc.Name == "kb_search" {
				// C6 知识库搜索（chat 包内实现——不经过 agent ExecContext）
				query, _ := tc.Args["query"].(string)
				limit := 10
				if l, ok := tc.Args["limit"].(float64); ok {
					limit = int(l)
				}
				content, execErr = KbSearchExecute(query, limit)
			} else {
				result := ec.ExecuteTool(ctx, tc.Name, tc.Args, gate)
				content = result.Content
				if result.Error != "" {
					execErr = fmt.Errorf("%s", result.Error)
				}
				dur = result.Duration
			}
			// P4-50 渐进式常驻钩子（成败判定只看执行层——Mr2109点破: 工具报错exec=失败/参数错=调用方锅/有输出=成功）
			if rt != nil && tc.Name != "tool_search" {
				if execErr != nil {
					typ := ErrTypeOf(execErr.Error())
					hint := rt.RecordOutcome(tc.Name, typ, execErr.Error())
					RecordToolError(tc.Name, execErr.Error(), tc.Args)
					if hint != "" {
						content = hint + "\n" + content
					}
				} else if !strings.HasPrefix(content, "【bash") && !strings.HasPrefix(content, "【系统】") {
					// 有输出 = 成功（拦截引导不算——bash lsof 引导不常驻）
					rt.RecordOutcome(tc.Name, "", "")
				}
			}
			if execErr != nil {
				content = fmt.Sprintf("工具执行失败: %s（%s）", execErr.Error(), truncateArgs(content, 500))
			}
			traces = append(traces, ToolTrace{
				Round: round, CallID: tc.ID, Name: tc.Name,
				Args: string(argsJSON), Result: content,
				Error: func() string { if execErr != nil { return execErr.Error() }; return "" }(),
				Duration: dur,
			})
			// tool 结果追加到消息流（chat 格式: role=tool + tool_call_id）
			cur = append(cur,
				map[string]any{"role": "assistant", "content": "", "tool_calls": []map[string]any{{
					"id": tc.ID, "type": "function",
					"function": map[string]any{"name": tc.Name, "arguments": string(argsJSON)},
				}}},
				map[string]any{"role": "tool", "tool_call_id": tc.ID, "content": content},
			)
		}
	}
	// 超过轮数——最后再调一次拿最终回复
	res, err := infer.Infer(ctx, model, sysPrompt, cur)
	if err != nil {
		return nil, traces, err
	}
	return res, traces, nil
}

// truncateArgs — 截断工具结果（对话展示——防巨型输出）
func truncateArgs(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + fmt.Sprintf("…（共 %d 字符——已截断）", len(s))
}

// BuildToolsParam — 组装 chat 格式 tools 参数（P4-42: 只出 L0 常驻 + deferred 由 tool_search 发现注入）
func BuildToolsParam() []map[string]any {
	defs := agent.AllTools()
	out := make([]map[string]any, 0, len(defs)+1)
	for _, d := range defs {
		// P4-42 deferred: 非 L0 工具不进主提示（tool_search 发现后注入）——注册中心有或 L0 名单才出
		if !L0ToolNames[d.Function.Name] {
			continue
		}
		desc := d.Function.Description
		// P4-38 T4 few-shot 示例（LangChain 官方——few-shot 大幅提升小模型工具调用——照抄格式防空参数）
		switch d.Function.Name {
		case "bash":
			desc += "\n【参数示例】{\"command\":\"ls -la\"} 或 {\"command\":\"python3 test.py\"}"
		case "read":
			desc += "\n【参数示例】{\"path\":\"core/internal/chat/chat_compact.go\",\"limit\":80}"
		case "glob":
			desc += "\n【参数示例】{\"pattern\":\"core/**/*.go\"}"
		case "grep":
			desc += "\n【参数示例】{\"pattern\":\"MaxToolRounds\",\"path\":\"core/internal\"}"
		case "ls":
			desc += "\n【参数示例】{\"path\":\"core\"}"
		case "write":
			desc += "\n【参数示例】{\"path\":\"file.txt\",\"content\":\"文件内容\"}"
		case "edit":
			desc += "\n【参数示例】{\"path\":\"file.go\",\"search\":\"旧文本\",\"replace\":\"新文本\"}"
		}
		out = append(out, map[string]any{
			"type": d.Type,
			"function": map[string]any{
				"name":        d.Function.Name,
				"description": desc,
				"parameters":  d.Function.Parameters,
			},
		})
	}
	// tool_search（P4-42 deferred 入口——主提示常驻）
	out = append(out, BuildToolSearchParam())
	// C6 kb_search（知识库主动查——Mr2109铁律"不确定先查库"）
	out = append(out, map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "kb_search",
			"description": "搜索虫族知识库（BM25 相关度——中文友好）——不确定的问题先查知识库再回答。参数: query 搜索关键词, limit 返回条数(默认10)。",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "搜索关键词（中文友好）"},
					"limit": map[string]any{"type": "integer", "description": "返回条数（默认10）"},
				},
				"required": []string{"query"},
			},
		},
	})
	return out
}

// BuildToolSearchParam — tool_search 工具定义（P4-42 deferred 入口）
func BuildToolSearchParam() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "tool_search",
			"description": "搜索发现更多工具（deferred 按需加载）。【什么时候用】当前工具列表没有合适工具时——如查系统状态搜\"系统\"、查影音搜\"剪辑\"、查音乐搜\"音乐\"、查效率搜\"计算\"。发现后直接用工具名调用。",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "搜索词（中文——如 系统/剪辑/音乐/知识库/效率）"},
				},
				"required": []string{"query"},
			},
		},
	}
}

// extractToolCalls — 从 chat 格式响应提取 tool_calls（JSON 字符串）
func extractToolCalls(raw []byte) []chatToolCall {
	var obj struct {
		Choices []struct {
			Message struct {
				ToolCalls []chatToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil || len(obj.Choices) == 0 {
		return nil
	}
	return obj.Choices[0].Message.ToolCalls
}

// hasToolCalls — 判断 chat 响应是否有工具调用（字符串快速判断）
func hasToolCalls(raw []byte) bool {
	return strings.Contains(string(raw), `"tool_calls"`)
}


// BuildHermesToolPrompt — P4-46/47 Hermes 工具指令（治本: 不带 tools 字段——模板 XML 分支不渲染）
// Hermes Function Calling 官方标准（NousResearch）: 工具定义 <tools> + OpenAI JSON schema——模型输出 <tool_call> JSON
// Qwen 官方: Hermes-style tool use 最大化函数调用性能（模型训练过该变体）
// 工具调用轮温度 0.0（P4-47——采样方差破坏 tool_call 内 JSON）
// P4-50 渐进式常驻: rt != nil → 渐进模式（初始 resident 空——<tools> 只 tool_search + 已常驻工具——成功即常驻）
//                 rt == nil → 老行为（L0 全列表）
func BuildHermesToolPrompt(rt *ToolRuntime) string {
	tools := HermesToolDefs()
	if rt != nil {
		// 渐进模式: <tools> = tool_search + resident（动态——成功即常驻——初始空）
		var res []struct {
			Name     string
			Desc     string
			Props    map[string]any
			Required []string
		}
		res = append(res, struct {
			Name     string
			Desc     string
			Props    map[string]any
			Required []string
		}{"tool_search", "搜索发现工具（参数: query——中文描述需求——如 搜\"系统\"查状态/搜\"文件\"查文件操作/搜\"端口\"查服务——发现后调用——成功使用的工具会自动加入下方常驻列表）", map[string]any{"query": map[string]any{"type": "string", "description": "中文描述你需要的工具能力"}}, []string{"query"}})
		for _, name := range rt.Resident() {
			if d := residentToolDesc(name); d != "" {
				res = append(res, struct {
					Name     string
					Desc     string
					Props    map[string]any
					Required []string
				}{name, d, map[string]any{}, []string{}})
			}
		}
		tools = res
	}
	// 转 OpenAI JSON schema（Hermes 标准——<tools> 包 JSON）
	schemas := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		schemas = append(schemas, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Desc,
				"parameters": map[string]any{
					"type":       "object",
					"properties": t.Props,
					"required":   t.Required,
				},
			},
		})
	}
	schemaJSON, _ := json.Marshal(schemas)
	callSchema := `{"properties": {"arguments": {"title": "Arguments", "type": "object"}, "name": {"title": "Name", "type": "string"}}, "required": ["arguments", "name"], "title": "FunctionCall", "type": "object"}`

	var b strings.Builder
	b.WriteString("\n\n# 工具调用（Hermes Function Calling 标准）\n")
	b.WriteString("你是函数调用 AI。以下是可用工具（<tools> 内 JSON schema 定义）:\n")
	b.WriteString("<tools>\n" + string(schemaJSON) + "\n</tools>\n\n")
	b.WriteString("每次函数调用输出一个 JSON 对象（函数名+参数）——包在 <tool_call></tool_call> XML 标签内:\n")
	b.WriteString("{" + callSchema + "}\n")
	b.WriteString("格式（name 与 arguments 必填——arguments 内参数按工具 schema 必填——不能为空）:\n")
	b.WriteString("<tool_call>\n{\"name\": \"工具名\", \"arguments\": {\"参数名\": \"参数值\"}}\n</tool_call>\n\n")
	b.WriteString("工具结果会以 <tool_response> 标签回传。\n")
	if rt != nil {
		b.WriteString("【渐进式工具】上方常驻列表外没有你要的工具能力时——先 tool_search 搜索（中文描述需求——如查系统状态搜\"系统\"——查影音搜\"剪辑\"——查端口/服务搜\"端口\"——查文件操作搜\"文件\"——查知识库搜\"知识库\"——查网络搜\"网络\"）。成功使用某工具后——它会自动加入上方常驻列表（本对话后续无需再搜）。同一工具不要反复搜索——工具已列出就直接调用。\n")
	}
	b.WriteString("【重要——专用工具优先】系统状态/任务队列/集群/模型/素材库/音乐下载/文件统计/字幕/OCR 等——都有专用工具——先 tool_search 搜索（如搜 \"系统\"/\"任务\"/\"素材\"/\"音乐\"/\"统计\"）——【tool_search 发现的工具已加入你的可用工具列表——与上方 <tools> 内工具同等地位——不要怀疑——直接用工具名调用】。不要用 bash 硬做专用工具能做的事（如文件统计用 file_count——素材查询用 footage_search）。\n")
	b.WriteString("调用工具时不要解释——直接输出 <tool_call>。\n")
	b.WriteString("【工具帮助】不确定工具的参数/用法时——给该工具加 help:true（如 {\"help\":true}）——返回该工具详细文档（参数示例/变更记录）——看完再调用。不要反复搜索同一关键词——工具已列出就直接调用。\n")
	return b.String()
}

// residentToolDesc — resident 工具描述（chat deferred → agent 工具 → 兜底名）
func residentToolDesc(name string) string {
	if def, ok := ChatExtraToolDefs()[name]; ok {
		if fn, ok2 := def["function"].(map[string]any); ok2 {
			if d, ok3 := fn["description"].(string); ok3 {
				return d
			}
		}
	}
	for _, d := range agent.AllTools() {
		if d.Function.Name == name && d.Function.Description != "" {
			return d.Function.Description
		}
	}
	return name
}

// HermesToolDefs — 工具定义表（名称/描述/参数 schema——Hermes 标准 <tools> 用）
func HermesToolDefs() []struct {
	Name     string
	Desc     string
	Props    map[string]any
	Required []string
} {
	return []struct {
		Name     string
		Desc     string
		Props    map[string]any
		Required []string
	}{
		{"bash", "执行 shell 命令（项目/系统操作）", map[string]any{"command": map[string]any{"type": "string", "description": "要执行的命令"}}, []string{"command"}},
		{"read", "读文件内容（分页）", map[string]any{"path": map[string]any{"type": "string", "description": "文件路径"}, "limit": map[string]any{"type": "integer", "description": "行数"}}, []string{"path"}},
		{"write", "写文件（覆盖）", map[string]any{"path": map[string]any{"type": "string", "description": "文件路径"}, "content": map[string]any{"type": "string", "description": "内容"}}, []string{"path", "content"}},
		{"edit", "编辑文件（查找替换）", map[string]any{"path": map[string]any{"type": "string"}, "search": map[string]any{"type": "string", "description": "旧文本"}, "replace": map[string]any{"type": "string", "description": "新文本"}}, []string{"path", "search", "replace"}},
		{"glob", "按模式找文件（如 core/**/*.go）", map[string]any{"pattern": map[string]any{"type": "string"}}, []string{"pattern"}},
		{"grep", "搜文件内容", map[string]any{"pattern": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"}}, []string{"pattern"}},
		{"ls", "列目录", map[string]any{"path": map[string]any{"type": "string"}}, []string{"path"}},
		{"kb_search", "查知识库经验", map[string]any{"query": map[string]any{"type": "string"}, "limit": map[string]any{"type": "integer"}}, []string{"query"}},
		{"web_search", "网络搜索（searxng——query 必填——可选 lang/time_range/domains/fetch_top——不确定先 help）", map[string]any{"query": map[string]any{"type": "string"}, "lang": map[string]any{"type": "string"}, "time_range": map[string]any{"type": "string"}, "domains": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "fetch_top": map[string]any{"type": "integer"}}, []string{"query"}},
		{"web_fetch", "抓网页内容", map[string]any{"url": map[string]any{"type": "string"}}, []string{"url"}},
		{"tool_search", "搜索发现更多工具（参数: query——搜 系统/剪辑/音乐/知识库 等）", map[string]any{"query": map[string]any{"type": "string", "description": "搜 系统/剪辑/音乐/知识库 等"}}, []string{"query"}},
		{"skill_load", "加载技能", map[string]any{"name": map[string]any{"type": "string"}}, []string{"name"}},
		// P4-50 虫族系统总览（L0 常驻——模型盲时也要看见——系统任务先调用）
		{"zerg_overview", "虫族系统总览——最新架构/使用/状态/文档（系统任务/改代码/排障先调——无参全景——section=模块 下钻）", map[string]any{"section": map[string]any{"type": "string"}}, []string{}},
		// P5-01 项目文档检索（v2.5.8——查文档先 doc_search——命中片段不全文读）
		{"doc_search", "项目文档检索——关键词→docs 命中文件+片段（当前版优先——query 必填——scope 可选限定如 v2.5.8/常青——查文档先调本工具不 open 全文）", map[string]any{"query": map[string]any{"type": "string", "description": "关键词（空格分词——AND）"}, "scope": map[string]any{"type": "string", "description": "限定范围——如 v2.5.8/常青（默认全部）"}}, []string{"query"}},
	}
}

