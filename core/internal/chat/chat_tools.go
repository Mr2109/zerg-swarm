// chat_tools.go — v2.5.7 对话轻量工具循环（C4b——决策定稿: 对话可用任何工具/skill/mcp）
// 复用 agent 工具链（DefaultTools + ExecContext.ExecuteTool——不重复造轮子）
// 流程: 调网关(chat 格式+tools) → 响应含 tool_calls → 执行 → tool 结果追加 → 再调 → 循环（最多 3 轮）
// 注意: 会话存储里 tool 消息落库（tool_calls JSON + tool_name）——C5 前端展示工具调用

package chat

import (
	"encoding/json"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"strings"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/ffp"
)

// 工具循环上限（防失控——对话是交流——不是任务）
// MaxToolRounds — 工具循环最大轮数（P4-37 3→10——多步任务需多轮工具——Hermes agent loop 无硬上限）
const MaxToolRounds = 10

// ChatToolsWorkDir — 对话工具工作目录（唯一常量——2026-09-05 统一: 非流式 /tmp/zerg-chat/tools
// 与流式项目根曾分叉致路径行为不一致——两条循环路径共用此值）
var ChatToolsWorkDir = statepath.WorkspaceRoot()

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

// toolNameListOf — tools 参数 → 工具名列表（LoopGuard 引导用——chat 包内版——api 包 toolNameList 同构）
func toolNameListOf(tools []map[string]any) []string {
	var names []string
	for _, t := range tools {
		if fn, ok := t["function"].(map[string]any); ok {
			if nm, ok := fn["name"].(string); ok {
				names = append(names, nm)
			}
		}
	}
	return names
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
			"description": "搜索发现更多工具（按需加载）。当前工具列表没有合适工具时用——如 查系统状态搜\"系统\"/system、查影音搜\"剪辑\"/video、查音乐搜\"音乐\"/music、查效率搜\"计算\"/calc。发现后直接用工具名调用。",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "搜索词（中文或英文——如 系统/system、剪辑/video、音乐/music、知识库/knowledge）"},
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
//
//	rt == nil → 老行为（L0 全列表）
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
		}{"tool_search", "搜索发现工具（query 描述需求，中英均可——如 搜\"系统\"/system 查状态、搜\"文件\"/file 查文件操作、搜\"端口\"/port 查服务；发现后调用，成功使用的工具自动加入下方常驻列表）", map[string]any{"query": map[string]any{"type": "string", "description": "描述你需要的工具能力（中文或英文）"}}, []string{"query"}})
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
	b.WriteString("格式（name/arguments 必填，arguments 内参数按 schema 必填、不可为空）:\n")
	b.WriteString("<tool_call>\n{\"name\": \"工具名\", \"arguments\": {\"参数名\": \"参数值\"}}\n</tool_call>\n\n")
	b.WriteString("工具结果会以 <tool_response> 标签回传。\n")
	b.WriteString(ffp.Conventions + "\n")
	if rt != nil {
		b.WriteString("【渐进式工具】上方常驻列表外没有你要的工具能力时，先 tool_search 搜索（中英均可，如 \"系统\"/system、\"剪辑\"/video、\"端口\"/port、\"文件\"/file、\"知识库\"/knowledge）。工具成功使用后自动加入常驻列表（本对话无需再搜）；已列出的工具直接调用，勿反复搜索。\n")
	}
	b.WriteString("【专用工具优先】系统状态/任务队列/集群/模型/素材库/音乐下载/文件统计/字幕/OCR 等都有专用工具，先 tool_search 搜索（如 \"系统\"/\"任务\"/\"素材\"/\"音乐\"/\"统计\"）——搜到的工具与上方 <tools> 同等可用，直接用工具名调用。不要用 bash 硬做专用工具的事（如文件统计用 file_count、素材查询用 footage_search）。\n")
	b.WriteString("调用工具时不要解释——直接输出 <tool_call>。\n")
	b.WriteString("【工具帮助】不确定参数/用法时给该工具加 help:true（如 {\"help\":true}），返回详细文档（参数示例/变更记录）后再调用。\n")
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
		{"doc_search", "项目文档检索——关键词→docs 命中文件+片段（当前版优先——query 必填——scope 可选限定如 v2.5.9/常青——查文档先调本工具不 open 全文）", map[string]any{"query": map[string]any{"type": "string", "description": "关键词（空格分词——AND）"}, "scope": map[string]any{"type": "string", "description": "限定范围——如 v2.5.9/常青（默认全部）"}}, []string{"query"}},
	}
}
