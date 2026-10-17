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
//
// ★ GAP-20261017-09（2026-10-17 用户口径「虫族对话 基于虫族 CLI 干活，只能使用虫族命令」）：
// bash/read/write/edit/glob/grep/ls、引导器 tool_search/kb_search，以及二次收窄的
// web_search/web_fetch/skill_load/zerg_overview **一律不再出**（见 chatForbiddenTools /
// chatPromptExcluded），只留 zerg 一个 CLI 工具（定义见 chat_tool_zergcmd.go）。✗ 别把它们加回来。
func BuildToolsParam() []map[string]any {
	defs := agent.AllTools()
	out := make([]map[string]any, 0, len(defs)+1)
	for _, d := range defs {
		// P4-42 deferred: 非 L0 工具不进主提示（tool_search 发现后注入）——注册中心有或 L0 名单才出
		if !L0ToolNames[d.Function.Name] {
			continue
		}
		// GAP-20261017-09：对话禁用集在**这一层**过滤（✗ 不改 agent.AllTools()、✗ 不动 L0ToolNames 常量本身）
		// —— 同名的 zerg 也在这里跳过：它的 schema 以 ChatZergToolParam 为唯一真源，末尾统一追加。
		if chatPromptHidden(d.Function.Name) || d.Function.Name == ZergCmdToolName {
			continue
		}
		out = append(out, map[string]any{
			"type": d.Type,
			"function": map[string]any{
				"name":        d.Function.Name,
				"description": d.Function.Description,
				"parameters":  d.Function.Parameters,
			},
		})
	}
	// GAP-20261017-09: 对话唯一 CLI 工具（首词必须逐字 zerg——执行面硬闸见 chat_tool_zergcmd.go）
	out = append(out, ChatZergToolParam())
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
//
// ★ GAP-20261017-09（2026-10-17 用户口径「虫族对话 基于虫族 CLI 干活，只能使用虫族命令」）：
//
//	<P4-50 渐进式常驻>**停用** —— 渐进模式靠 tool_search 发现工具，而 tool_search 已从对话工具面撤下
//	⇒ rt 保留在签名里只为不改两处调用点，本轮起**不参与渲染**；<tools> 一律出「撤下面过滤后」的全量清单。
func BuildHermesToolPrompt(rt *ToolRuntime) string {
	tools := HermesToolDefs()
	// ★ 渲染层过滤：撤下面（chatPromptHidden）一律不出——真源仍只有 HermesToolDefs 一处，
	//   ✗ 别在下面另抄一份「可用工具」清单（两份清单必然分家）。
	var kept []struct {
		Name     string
		Desc     string
		Props    map[string]any
		Required []string
	}
	for _, d := range tools {
		if chatPromptHidden(d.Name) {
			continue
		}
		kept = append(kept, d)
	}
	tools = kept
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
	// ★ GAP-20261017-09：示例改成 zerg（旧示例用 read 工具——那个工具已从对话工具面撤下，照抄必被 rc=2 拒执）。
	b.WriteString("\n" + "<tool_call>\n{\"name\": \"zerg\", \"arguments\": {\"command\": \"zerg version\"}}\n</tool_call>\n\n")
	// 示例必须**真可执行**（Poka-yoke）：旧示例用「工具名/参数名/参数值」占位词，实测被模型照抄成工具名
	// ⇒ 调用必然失败 ⇒ 反复重试空转（2026-09-17 观测三条 err 行的 name 就是字面量「工具名」）。
	b.WriteString("【注意】上面是**示例**：把 name 与 arguments 换成你真正要调用的工具与参数，不要照抄示例里的内容。\n")
	b.WriteString("工具结果会以 <tool_response> 标签回传。\n")
	b.WriteString(ffp.Conventions + "\n")
	// ★ GAP-20261017-09：权威面（这一段是「以 BuildToolsParam 为准」的口径——
	//   基础系统提示里若有旧工具清单，一律由这段覆盖，✗ 不另抄一份清单）。
	b.WriteString(zergOnlyPromptSection())
	b.WriteString("调用工具时不要解释——直接输出 <tool_call>。\n")
	return b.String()
}

// zergOnlyPromptSection — 对话工具面权威段（GAP-20261017-09）。
//
// ★ 工具名清单**现读 BuildToolsParam()**（✗ 手写字面量）：这样「提示词里说的工具」与
// 「真发出去的 tools 参数」不可能分家——改工具面只改一处。
func zergOnlyPromptSection() string {
	names := toolNameListOf(BuildToolsParam())
	var b strings.Builder
	b.WriteString("\n【对话工具面（权威 · 以此为准）】本对话可用的工具只有上面 <tools> 里这几个：" + strings.Join(names, "、") + "。\n")
	b.WriteString("- ✗ bash / read / write / edit / glob / grep / ls / tool_search / kb_search / web_search / web_fetch" +
		" / skill_load / zerg_overview 在本对话**不存在**（已从工具面撤下——调用即 rc=2 拒执）。\n")
	b.WriteString("- 要看文件 / 查系统 / 跑脚本 / 查账本：一律用 zerg 工具发虫族命令 —— " +
		"zerg help（全命令树）、zerg help <族名>、zerg code show <件>:<行>、zerg code find '<正则>'、" +
		"zerg gap ls、zerg version。\n")
	b.WriteString("- 要看技能（skill）：先 zerg skill ls（清单：名字 · 触发描述），再 zerg skill show <名>（正文）——" +
		"skill_load 已撤，这两条是唯一正门。\n")
	b.WriteString("- 搜网（web_search / web_fetch）：**本对话没有这个能力** —— 虫族 CLI 本版没有搜网命令。" +
		"用户要搜网就**如实说做不到**，✗ 不许拿记忆冒充搜索结果、✗ 不许编链接。\n")
	b.WriteString("- 基础系统提示里若有「主提示只带基础工具（bash/read/…）」「需要其他能力时用 tool_search」" +
		"「遇到不清楚先 kb_search」这类旧描述 —— **一律作废**，以上面这一条为准。\n")
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
		// ★ GAP-20261017-09（2026-10-17 用户口径「虫族对话 基于虫族 CLI 干活，只能使用虫族命令」）：
		//   bash / read / write / edit / glob / grep / ls（＋引导器 tool_search / kb_search）已从**对话工具面撤下**；
		//   web_search / web_fetch / skill_load / zerg_overview 于同日二次收窄一并撤下（chatForbiddenTools）。
		//   这四行仍留在下表，只因注册表（L0ToolNames 真源）本单不动 ⇒ 两张表必须同步（判据 chat_tool_registry_l0_test.go）；
		//   **真发出去的那一面由 chatPromptHidden 过滤** ⇒ 下面四行对模型不可见、调用被执行面拒执。✗ 别以为还能调。
		//   工具名/描述/schema 的**唯一真源**在 chat_tool_zergcmd.go（✗ 别在这里再写一份字面量）。
		{ZergCmdToolName, zergCmdToolDesc, zergCmdToolProps(), []string{"command"}},
		{"web_search", "网络搜索（searxng——query 必填——可选 lang/time_range/domains/fetch_top——不确定先 help）", map[string]any{"query": map[string]any{"type": "string"}, "lang": map[string]any{"type": "string"}, "time_range": map[string]any{"type": "string"}, "domains": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "fetch_top": map[string]any{"type": "integer"}}, []string{"query"}},
		{"web_fetch", "抓网页内容", map[string]any{"url": map[string]any{"type": "string"}}, []string{"url"}},
		{"skill_load", "加载技能", map[string]any{"name": map[string]any{"type": "string"}}, []string{"name"}},
		// P4-50 虫族系统总览（L0 常驻——模型盲时也要看见——系统任务先调用）
		{"zerg_overview", "虫族系统总览——最新架构/使用/状态/文档（系统任务/改代码/排障先调——无参全景——section=模块 下钻）", map[string]any{"section": map[string]any{"type": "string"}}, []string{}},
		// P5-01 项目文档检索（v2.5.8——查文档先 doc_search——命中片段不全文读）
		{"doc_search", "项目文档检索——关键词→docs 命中文件+片段（当前版优先——query 必填——scope 可选限定如 v2.5.9/常青——查文档先调本工具不 open 全文）", map[string]any{"query": map[string]any{"type": "string", "description": "关键词（空格分词——AND）"}, "scope": map[string]any{"type": "string", "description": "限定范围——如 v2.5.9/常青（默认全部）"}}, []string{"query"}},
	}
}
