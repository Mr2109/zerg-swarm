// obs_semconv.go — T6.1「对外对齐」：OTel GenAI 语义约定的**命名映射表 + semconv 版本标记 +
// 内容捕获四档 → 标准枚举映射**；T6.2「补齐标准字段」：error.type / tool.* / conversation.* /
// server.* / TTFC / response.status / output.type / reasoning.level / prompt.*。
//
// 设计依据（逐字，取自 docs/01-设计/设计-内建调试版-v1.2-20260917.md 第〇节 B 组）：
//
//	B4 「`error.type` 只在 S6 记；规范要求**每个出错 span** 都要，且它是全表唯一 Stable 之一」
//	B5 「S4 span 名/kind 不合规；缺 `tool.type`/`tool.description`/`tool.call.id`」
//	B6 「缺内容捕获的标准开关：规范是 `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT ∈
//	    {NO_CONTENT, SPAN_ONLY, EVENT_ONLY, SPAN_AND_EVENT}`；我们自造 OFF/BASIC/VERBOSE/TRACE
//	    ⇒ **映射**四档到标准枚举，而非另造；内容类属性一律 Opt-In」
//	B10「缺 `gen_ai.conversation.id` 落地（规范禁止用新 UUID/trace id/内容 hash 兜底）；
//	    缺 `conversation.compacted`（只能设 true，不设 false）」
//	B11「缺 `server.address`/`server.port`（Stable 且 sampling-relevant）」
//	B12「缺 TTFC 与生命周期状态的标准落点：`gen_ai.response.time_to_first_chunk`、
//	    `gen_ai.response.status`（queued/in_progress/completed/incomplete/failed/cancelled）；
//	    四终局/verdict 保留为 `zerg.*`」
//	B14「指纹/工具数/字节数一律留在 `zerg.*` 命名空间，别伪装 `gen_ai.*`」
//	B15「`gen_ai.*` 全部 Development（随时可变）；全表唯一 Stable 是 error.type/server.address/
//	    server.port ⇒ 记录"对齐的 semconv 版本/commit"，升级时做字段兼容检查（本次基准
//	    `semantic-conventions-genai@c88d504, 2026-09-16`）」
//	B17「缺 `output.type` / `request.reasoning.level` / `prompt.name`+`prompt.version`」
//	B19「根 span（我们的 `run`）本质是 workflow ⇒ 应为 `invoke_workflow {gen_ai.workflow.name}`
//	    且 name **MUST 低基数**」
//
// 属性清单与要求级别逐条照抄调研子报告（docs/调研/子报告-内建调试版-20260917/subagent-summary-1-*.txt，
// 核实基准 semantic-conventions-genai@c88d504, 2026-09-16）。
//
// 三条口径（改本文件前先读）：
//
//	① **只对齐，不冒充**：键名逐字用标准点分名（`gen_ai.*` / `error.type` / `server.*`）；
//	   凡是规范里**没有**这个名字的，一律留在 `zerg.*` 命名空间（zerg.semconv.* / zerg.capture.* /
//	   zerg.span.name / zerg.terminal / zerg.verdict）。映射表是唯一真相源 ⇒
//	   obs_semconv_test.go 用**反射**逐条校对"表行 ↔ 结构体 tag"（手滑即红）。
//	② **拿不到就缺席，绝不编造**：conversation.id 拿不到 ⇒ 键不出现（不许用新 UUID/trace_id/
//	   内容 hash 兜底）；server.address 拿不到 ⇒ 键不出现；tool.description 查不到 ⇒ 键不出现。
//	③ **只 true 不得设 false**：conversation.compacted 只在**能可靠判定压缩发生**时置 true；
//	   显式 false 会在唯一写入口被**剥掉**（规范原文：SHOULD NOT set false）。
//
// 与 obs.go 三条铁律同源：本文件只读入参/环境、只补字段；不参与任何判定；best-effort，绝不外抛、绝不 panic。
package chat

import (
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
)

// ══════════════ ① semconv 基准（B15：常量字段，供日后兼容检查）══════════════

const (
	// SemconvGenAIProject — 被对齐的语义约定项目
	SemconvGenAIProject = "semantic-conventions-genai"
	// SemconvGenAICommit — 被对齐的 commit（升级时改这里 + 跑一遍字段兼容检查）
	SemconvGenAICommit = "c88d504"
	// SemconvGenAIDate — 该 commit 的日期（调研核实基准，2026-09-16）
	SemconvGenAIDate = "2026-09-16"
	// SemconvGenAIVersion — 完整基准串（写进每条记录的 `zerg.semconv.version`）
	SemconvGenAIVersion = SemconvGenAIProject + "@" + SemconvGenAICommit + ", " + SemconvGenAIDate
)

// 记录 kind 取值（与 obs.go / obs_compaction.go 既有**同值字面量**；给名字只为映射表可读，不改既有写侧）
const (
	obsKindTurn    = "turn"
	obsKindTool    = "tool"
	obsKindCompact = "compact"
)

// ProviderZergLocal — 自研本地集群的 provider **自定义值**。
//
// 规范：provider 枚举只列厂商（openai/deepseek/gcp.gemini…），**没有任何"自建/本地集群"值**；
// 枚举未命中时 MAY 用自定义值，但**须在该系统的 semconv 里写文档**。今日该值只在代码里声明，
// 文档附录待补（已如实登记在交付说明；不写进 gen_ai.* 冒充任何厂商）。
const ProviderZergLocal = "zerg.local"

// GenAIOutputTypeText — gen_ai.output.type 取值：文本（我们无 image/speech 通路）
const GenAIOutputTypeText = "text"

// ══════════════ ② 官方属性名闭集（映射表的 OTel 列；`gen_ai.*` 只允许这里出现）══════════════

const (
	AttrGenAIOperationName            = "gen_ai.operation.name"
	AttrGenAIProviderName             = "gen_ai.provider.name"
	AttrGenAIRequestModel             = "gen_ai.request.model"
	AttrGenAIRequestStream            = "gen_ai.request.stream"
	AttrGenAIRequestReasoningLevel    = "gen_ai.request.reasoning.level"
	AttrGenAIResponseModel            = "gen_ai.response.model"
	AttrGenAIResponseID               = "gen_ai.response.id"
	AttrGenAIResponseFinishReasons    = "gen_ai.response.finish_reasons"
	AttrGenAIResponseStatus           = "gen_ai.response.status"
	AttrGenAIResponseTimeToFirstChunk = "gen_ai.response.time_to_first_chunk"
	AttrGenAIUsageInputTokens         = "gen_ai.usage.input_tokens"
	AttrGenAIUsageOutputTokens        = "gen_ai.usage.output_tokens"
	AttrGenAIUsageReasoningTokens     = "gen_ai.usage.reasoning.output_tokens"
	AttrGenAIConversationID           = "gen_ai.conversation.id"
	AttrGenAIConversationCompacted    = "gen_ai.conversation.compacted"
	AttrGenAIToolName                 = "gen_ai.tool.name"
	AttrGenAIToolType                 = "gen_ai.tool.type"
	AttrGenAIToolDescription          = "gen_ai.tool.description"
	AttrGenAIToolCallID               = "gen_ai.tool.call.id"
	AttrGenAIToolCallArguments        = "gen_ai.tool.call.arguments"
	AttrGenAIToolCallResult           = "gen_ai.tool.call.result"
	AttrGenAIAgentName                = "gen_ai.agent.name"
	AttrGenAIWorkflowName             = "gen_ai.workflow.name"
	AttrGenAIPromptName               = "gen_ai.prompt.name"
	AttrGenAIPromptVersion            = "gen_ai.prompt.version"
	AttrGenAIPromptVariablePrefix     = "gen_ai.prompt.variable." // 后接变量名（Opt-In）
	AttrGenAIOutputType               = "gen_ai.output.type"
	AttrGenAIInputMessages            = "gen_ai.input.messages"
	AttrGenAIOutputMessages           = "gen_ai.output.messages"
	AttrGenAISystemInstructions       = "gen_ai.system_instructions"
	AttrGenAIRequestTemperature       = "gen_ai.request.temperature"
	AttrGenAIRequestTopP              = "gen_ai.request.top_p"
	AttrGenAIRequestTopK              = "gen_ai.request.top_k"
	AttrGenAIRequestMaxTokens         = "gen_ai.request.max_tokens"
	AttrGenAIRequestSeed              = "gen_ai.request.seed"
	AttrGenAIRequestStopSequences     = "gen_ai.request.stop_sequences"
	AttrGenAIRequestFrequencyPen      = "gen_ai.request.frequency_penalty"
	AttrGenAIRequestPresencePen       = "gen_ai.request.presence_penalty"
	AttrGenAIRequestChoiceCount       = "gen_ai.request.choice.count"
	AttrGenAIRequestPrevResponseID    = "gen_ai.request.previous_response.id"
	AttrErrorType                     = "error.type" // Stable
	AttrServerAddress                 = "server.address"
	AttrServerPort                    = "server.port"
)

// Requirement 取值（照抄子报告的口径字串，读侧按字串对账）
const (
	ReqRequired     = "Required"
	ReqCondRequired = "Conditionally Required"
	ReqRecommended  = "Recommended"
	ReqOptIn        = "Opt-In"
)

// ObsSemconvAttr — 映射表一行：**我们的私名 → OTel 标准名**。
//
// Emit=true ⇒ 我们**当前真的会落到事件里**（必须与 ObsRecord 的 json tag 逐字一致，
// 由 obs_semconv_test.go 的反射用例锁死）；Emit=false ⇒ 表里有、我们暂未产（缺数据源或未接线），
// 这是**已知缺口清单**，不是"假装有"。
type ObsSemconvAttr struct {
	Private     string // 我们的私名（字段/概念）
	OTel        string // 标准属性名（gen_ai.* / error.type / server.*）
	Type        string // 规范类型
	Requirement string // 规范要求级别
	Emit        bool   // 我们当前是否真的落这个键
	Note        string // 口径/缺口说明
}

// ObsSemconvAttrs — **唯一映射表**（T6.1 ①）。新增/改动观测字段时先改这里。
func ObsSemconvAttrs() []ObsSemconvAttr {
	return []ObsSemconvAttr{
		{"Kind（turn|compact|prompt）", AttrGenAIOperationName, "string", ReqRequired, true,
			"枚举命中 MUST 用枚举值（我们一律用 chat）；sampling-relevant ⇒ 建 span 时写"},
		{"Model", AttrGenAIRequestModel, "string", ReqCondRequired + "（If available）", true,
			"只落**推理类**记录。agent span 支持动态路由时 SHOULD NOT 写死 model（B3）；我们无 agent span"},
		{"（自研本地集群）", AttrGenAIProviderName, "string", ReqRequired, true,
			"provider 枚举里**没有**自建集群值 ⇒ 用自定义值 " + ProviderZergLocal + "（规范允许 custom value，但须在本系统 semconv 里登记）"},
		{"t.chunks>0", AttrGenAIRequestStream, "boolean", ReqCondRequired + "（iff streaming）", true,
			"未设置即被假定非流式 ⇒ 后端无法解释 TTFC 缺失。证据口径：收到过流式分块才算流式"},
		{"reasoningEffortChat", AttrGenAIRequestReasoningLevel, "string", ReqRecommended, true,
			"值 SHOULD 是发给 provider 的原始字符串 —— 直接引用 chat_infer 请求体里的同一常量（同源）"},
		{"EndReason/Result", AttrGenAIResponseStatus, "string", ReqCondRequired, true,
			"生命周期状态（queued/in_progress/completed/incomplete/failed/cancelled）。我们的四终局=四个**终态**"},
		{"Turn.FirstByteMS", AttrGenAIResponseTimeToFirstChunk, "double（秒）", ReqRecommended + "（流式）", true,
			"首字节闸的标准落点（我们内部仍是 ms；对外换算成秒）"},
		{"（usage）", AttrGenAIUsageInputTokens, "int", ReqRecommended, true,
			"计费口径（含缓存命中）；上游不给 ⇒ 缺席（不写 0 顶替）"},
		{"（usage）", AttrGenAIUsageOutputTokens, "int", ReqRecommended, true, "同上；细化输出 token 是其子集"},
		{"（usage.reasoning）", AttrGenAIUsageReasoningTokens, "int", ReqRecommended + "（When applicable）", true,
			"思考/CoT token（不是字符数）；值含在 output_tokens 内"},
		{"Session", AttrGenAIConversationID, "string", ReqCondRequired + "（iff readily available）", true,
			"拿不到就 SHOULD NOT 填：**不许**用新 UUID / trace_id / 内容 hash 兜底（B10）"},
		{"压缩发生", AttrGenAIConversationCompacted, "boolean", ReqRecommended, true,
			"只 true，**不得设 false**（B10/口径③）"},
		{"Tool", AttrGenAIToolName, "string", ReqRequired + "（execute_tool）", true,
			"sampling-relevant，且是 span 名的一部分（execute_tool {gen_ai.tool.name}）"},
		{"Tool", AttrGenAIToolType, "string", ReqRecommended + "（If available）", true,
			"枚举 function/extension/datastore；未登记的工具 ⇒ 缺席（不猜）"},
		{"Tool", AttrGenAIToolDescription, "string", ReqRecommended + "（If available）", true,
			"查得到才落（含敏感信息警告）；拿不到就缺席"},
		{"ToolTrace.CallID", AttrGenAIToolCallID, "string", ReqRecommended + "（If available）", true,
			"把 model 侧 tool_call 与执行侧 span 对上号的唯一标准键"},
		{"PromptName", AttrGenAIPromptName, "string", ReqCondRequired + "（when a named prompt template is used）", true,
			"我们显然是模板拼装；名字由调用方声明（声明不了 ⇒ 缺席）"},
		{"PromptVersion", AttrGenAIPromptVersion, "string", ReqCondRequired, true,
			"与 T3.4 的 prompt_version 同源（模板 sha256 前 8 字节）"},
		{"（对话只出文本）", AttrGenAIOutputType, "string", ReqCondRequired, true,
			"枚举 text/json/image/speech，描述**请求的输出模态**；我们无 image/speech 通路 ⇒ text"},
		{"ChatErrXxx", AttrErrorType, "string", ReqCondRequired + "（出错才有）", true,
			"**Stable**。每个出错 span 都要（不只末事件）；低基数闭集，未知兜底 _OTHER（B4）"},
		{"（拨号对端）", AttrServerAddress, "string", ReqRecommended, true,
			"**Stable** 且 sampling-relevant（建 span 时给）。多节点定位的标准位置"},
		{"（拨号对端）", AttrServerPort, "int", ReqCondRequired + "（server.address 已设时）", true, "同上"},

		// ── 表里有、我们**暂未产**（缺口清单：不是"假装有"）──
		{"（无）", AttrGenAIRequestTemperature, "double", ReqRecommended, false, "未接线：对话请求体的采样参数未进入观测面"},
		{"（无）", AttrGenAIRequestTopP, "double", ReqRecommended, false, "未接线"},
		{"（无）", AttrGenAIRequestTopK, "int", ReqCondRequired, false,
			"未接线：注意 OpenAI 的 top_logprobs MUST NOT 报成本属性"},
		{"（无）", AttrGenAIRequestMaxTokens, "int", ReqRecommended, false, "未接线：动态额度折算出的上限应落这里"},
		{"（无）", AttrGenAIRequestSeed, "int", ReqCondRequired, false,
			"未接线：T3.5 的 request_seed 是**我们自己**推导的种子（zerg 命名空间），不等于发给 provider 的 request.seed"},
		{"（无）", AttrGenAIRequestStopSequences, "string[]", ReqRecommended, false, "未接线"},
		{"（无）", AttrGenAIRequestFrequencyPen, "double", ReqRecommended, false, "未接线（采样参数全量的一角）"},
		{"（无）", AttrGenAIRequestPresencePen, "double", ReqRecommended, false, "未接线（同上）"},
		{"（无）", AttrGenAIRequestChoiceCount, "int", ReqCondRequired + "（≠1 时才记）", false, "未接线：我们不产多候选"},
		{"（无）", AttrGenAIRequestPrevResponseID, "string", ReqRecommended, false, "未接线"},
		{"（无）", AttrGenAIResponseModel, "string", ReqRecommended, false,
			"未接线：上游响应没回传实际模型名 ⇒ **不拿 request.model 冒充** response.model"},
		{"（无）", AttrGenAIResponseID, "string", ReqRecommended, false, "未接线：上游响应没回传 completion id"},
		{"（无）", AttrGenAIResponseFinishReasons, "string[]", ReqRecommended, false,
			"未接线：上游 finish_reason 未回传（loopcore 适配器把 Finish 写死 stop ⇒ 不能拿它冒充标准字段）"},
		{"（无）", AttrGenAIInputMessages, "any", ReqOptIn, false,
			"内容类，默认不采（B6）。我们的事件流不落正文（指纹/计数走 zerg.*）"},
		{"（无）", AttrGenAIOutputMessages, "any", ReqOptIn, false, "同上"},
		{"（无）", AttrGenAISystemInstructions, "any", ReqOptIn, false,
			"内容类；规范要求 system 指令**单独**进这里（我们的分段账已有 system 段，但落的是长度不是正文）"},
		{"（无）", AttrGenAIToolCallArguments, "any", ReqOptIn, false, "内容类，默认不采（我们落参数摘要 args_digest，留 zerg 命名）"},
		{"（无）", AttrGenAIToolCallResult, "any", ReqOptIn, false, "内容类，默认不采"},
		{"（无）", AttrGenAIAgentName, "string", ReqCondRequired + "（When available）", false,
			"未接线：我们不是 agent 框架，且规范 NOT RECOMMENDED 用类型名当 name ⇒ 缺席（不编造）"},
		{"（无）", AttrGenAIWorkflowName, "string", ReqCondRequired + "（When available）", false,
			"未接线：根 span（run）尚未发记录；MUST 低基数、不得用类型名（B19）"},
		{"（无）", AttrGenAIPromptVariablePrefix + "<key>", "string", ReqOptIn, false, "内容类，默认不采"},
	}
}

// obsSemconvEmittedNames — 表里 Emit=true 的 OTel 名（闭集）。init 时构建，避免手抄第二份。
var obsSemconvEmittedNames = func() map[string]bool {
	m := map[string]bool{}
	for _, a := range ObsSemconvAttrs() {
		if a.Emit {
			m[a.OTel] = true
		}
	}
	return m
}()

// ObsSemconvEmitted — 该标准名是否属于我们**真的会落**的闭集（反射用例与读侧对账用）。
func ObsSemconvEmitted(otelName string) bool { return obsSemconvEmittedNames[otelName] }

// ObsContentAttrNames — 内容类（Opt-In）标准属性名清单：**默认（NO_CONTENT）一个都不许出现**。
func ObsContentAttrNames() []string {
	out := []string{}
	for _, a := range ObsSemconvAttrs() {
		if a.Requirement == ReqOptIn {
			out = append(out, a.OTel)
		}
	}
	return out
}

// ══════════════ ③ span 名 / operation.name 映射（B2/B5/B19）══════════════

// OTel span kind（规范侧取值；我们既有的 span_kind 字段是本地三值约定，**不互相冒充**）
const (
	OTelSpanKindClient   = "CLIENT"
	OTelSpanKindInternal = "INTERNAL"
	OTelSpanKindServer   = "SERVER"
)

// ObsSpanMapping — 我们的 kind → 标准 span 名模板 + gen_ai.operation.name + 规范侧 span kind。
type ObsSpanMapping struct {
	Kind      string // 我们的 kind（obs.go 的 Kind 取值）
	OTelName  string // 标准 span 名模板（{} 处按属性拼）
	Operation string // gen_ai.operation.name 取值
	OTelKind  string // 规范侧 span kind
	Note      string
}

// ObsSpanMappings — span 命名映射表（B19/B5；根 span 是 workflow）。
func ObsSpanMappings() []ObsSpanMapping {
	return []ObsSpanMapping{
		{"turn", "chat {gen_ai.request.model}", GenAIOperationChat, OTelSpanKindClient,
			"推理 span：同进程内跑模型 MAY 为 INTERNAL；我们拨的是本地网关（另一进程）⇒ CLIENT"},
		{"tool", "execute_tool {gen_ai.tool.name}", GenAIOperationExecuteTool, OTelSpanKindInternal,
			"本地工具（B5）。若走 MCP 则应是 client/server 两跳 + params._meta（未接线）"},
		{"compact", "chat {gen_ai.request.model}", GenAIOperationChat, OTelSpanKindInternal,
			"压缩是一次摘要推理（进程内发起）⇒ 归 chat；压缩发生即 gen_ai.conversation.compacted=true"},
		{"prompt", "chat {gen_ai.request.model}", GenAIOperationChat, OTelSpanKindInternal,
			"提示装配/账本是**同一次 chat 操作**的事件（非独立 span）；事件名见 gen_ai.client.inference.operation.details"},
		{"run", "invoke_workflow {gen_ai.workflow.name}", GenAIOperationInvokeWorkflow, OTelSpanKindInternal,
			"根 span（设计稿 §3.1 的 run）。name MUST 低基数、不得用类型名（B19）；**我们尚未发这条记录**"},
	}
}

// operation.name 枚举（照抄子报告：命中枚举 MUST 用枚举值）
const (
	GenAIOperationChat           = "chat"
	GenAIOperationExecuteTool    = "execute_tool"
	GenAIOperationInvokeAgent    = "invoke_agent"
	GenAIOperationInvokeWorkflow = "invoke_workflow"
	GenAIOperationPlan           = "plan"
)

// ObsSpanMappingOf — 按 kind 取映射（无对应项 ⇒ ok=false：**不冒充** —— 私有信号只留 zerg.*）。
func ObsSpanMappingOf(kind string) (ObsSpanMapping, bool) {
	for _, m := range ObsSpanMappings() {
		if m.Kind == kind {
			return m, true
		}
	}
	return ObsSpanMapping{}, false
}

// ObsOperationNameOf — kind → gen_ai.operation.name（无对应项 ⇒ 空串 = 缺席）。
func ObsOperationNameOf(kind string) string {
	if m, ok := ObsSpanMappingOf(kind); ok {
		return m.Operation
	}
	return ""
}

// ObsSpanName — 按模板拼标准 span 名（拿不到模板变量 ⇒ 退化为裸操作名，规范允许：
// 「agent.name 不可得时退化为 invoke_agent」同理；模型名不可得 ⇒ `chat`）。
func ObsSpanName(kind, tool, model string) string {
	if _, ok := ObsSpanMappingOf(kind); !ok {
		return ""
	}
	switch kind {
	case obsKindTool:
		if tool == "" {
			return GenAIOperationExecuteTool
		}
		return GenAIOperationExecuteTool + " " + tool
	case "run":
		return GenAIOperationInvokeWorkflow
	default:
		if model == "" {
			return GenAIOperationChat
		}
		return GenAIOperationChat + " " + model
	}
}

// ══════════════ ④ 内容捕获：我们四档 → 标准枚举（B6）══════════════

// 我们的四档（设计稿 v1.0/v1.1 「运行期分级」，生产默认 OFF）
const (
	ObsCaptureOFF     = "OFF"
	ObsCaptureBASIC   = "BASIC"
	ObsCaptureVERBOSE = "VERBOSE"
	ObsCaptureTRACE   = "TRACE"
)

// 标准枚举（OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT 的取值集合）
const (
	GenAICaptureNoContent    = "NO_CONTENT"     // 默认：不采内容
	GenAICaptureSpanOnly     = "SPAN_ONLY"      // 只写 span 属性
	GenAICaptureEventOnly    = "EVENT_ONLY"     // 只写事件属性
	GenAICaptureSpanAndEvent = "SPAN_AND_EVENT" // 两边都写
)

// 开关名（我们四档 / 标准总开关）
const (
	ObsCaptureEnvLevel = "ZERG_DEBUG_LEVEL"                                   // 我们的四档
	ObsCaptureEnvStd   = "OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT" // 标准总开关
)

// CaptureLevelToStd — **四档 → 标准枚举**（T6.1 ③）。
//
//	OFF     → NO_CONTENT（不采内容）
//	BASIC   → NO_CONTENT（决策/工具判定**不含正文**；正文类属性一律 Opt-In ⇒ 仍是不采内容）
//	VERBOSE → EVENT_ONLY （完整提示与响应**只写事件**：规范说结构化内容应优先放事件，
//	                       span 上结构化属性可能尚未支持）
//	TRACE   → SPAN_AND_EVENT（含内部状态 ⇒ 两边都写）
//	非法/认不出 → NO_CONTENT（**回最严**，fail-closed；留痕见 CaptureLevelStd）
func CaptureLevelToStd(level string) string {
	std, _ := CaptureLevelStd(level)
	return std
}

// CaptureLevelStd — 同 CaptureLevelToStd，但第二个返回值报告"是否命中闭集"
// （false = 认不出的值已回最严 —— 调用方须留痕，不许静默降级）。
func CaptureLevelStd(level string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case ObsCaptureOFF, ObsCaptureBASIC:
		return GenAICaptureNoContent, true
	case ObsCaptureVERBOSE:
		return GenAICaptureEventOnly, true
	case ObsCaptureTRACE:
		return GenAICaptureSpanAndEvent, true
	default:
		return GenAICaptureNoContent, false // 未知值 ⇒ 最严（不采内容）
	}
}

// NormalizeCaptureLevel — 环境原始值 → 闭集四档。ok=false 表示"给了值但认不出"（已回 OFF）。
// 空/未设置 ⇒ OFF 且 ok=true（**未设置**与**写错了**必须可分，同 obs.go「0 与未知必须可分」）。
func NormalizeCaptureLevel(raw string) (string, bool) {
	v := strings.ToUpper(strings.TrimSpace(raw))
	switch v {
	case "":
		return ObsCaptureOFF, true
	case ObsCaptureOFF, ObsCaptureBASIC, ObsCaptureVERBOSE, ObsCaptureTRACE:
		return v, true
	default:
		return ObsCaptureOFF, false
	}
}

// NormalizeCaptureStd — 标准枚举归一（大小写不敏感）。ok=false ⇒ 给了值但认不出。
func NormalizeCaptureStd(raw string) (string, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case GenAICaptureNoContent:
		return GenAICaptureNoContent, true
	case GenAICaptureSpanOnly:
		return GenAICaptureSpanOnly, true
	case GenAICaptureEventOnly:
		return GenAICaptureEventOnly, true
	case GenAICaptureSpanAndEvent:
		return GenAICaptureSpanAndEvent, true
	default:
		return GenAICaptureNoContent, false
	}
}

// ObsCaptureResolved — 生效档位（唯一判定点）。
//
// 优先级（写死）：**标准总开关显式且合法 ⇒ 用标准值**（我们对外就是 OTel 语义）；
// 否则用我们四档的映射结果。任一开关给了非法值 ⇒ 回最严 + fallback=true（fail-closed 且留痕）。
// 返回: level（我们四档之一；标准开关生效时为空串 = 我们那一档**缺席**，不编造）、std、fallback。
func ObsCaptureResolved() (level, std string, fallback bool) {
	if raw := strings.TrimSpace(os.Getenv(ObsCaptureEnvStd)); raw != "" {
		s, ok := NormalizeCaptureStd(raw)
		if !ok {
			return "", GenAICaptureNoContent, true
		}
		return "", s, false
	}
	lvl, ok := NormalizeCaptureLevel(os.Getenv(ObsCaptureEnvLevel))
	return lvl, CaptureLevelToStd(lvl), !ok
}

// GenAIEmitEvent — OTEL_INSTRUMENTATION_GENAI_EMIT_EVENT 的推导（标准实现口径，照抄子报告）：
// NO_CONTENT / SPAN_ONLY ⇒ false；EVENT_ONLY / SPAN_AND_EVENT ⇒ true（显式设置优先是调用方的事）。
func GenAIEmitEvent(std string) bool {
	switch std {
	case GenAICaptureEventOnly, GenAICaptureSpanAndEvent:
		return true
	default:
		return false
	}
}

// ObsContentAllowed — 内容类属性（Opt-In）是否允许落盘。NO_CONTENT ⇒ 一律不许（B6）。
func ObsContentAllowed(std string) bool {
	s, ok := NormalizeCaptureStd(std)
	if !ok {
		return false // 认不出 ⇒ 最严
	}
	return s != GenAICaptureNoContent
}

// ══════════════ ⑤ 错误分类 → error.type（B4：低基数闭集）══════════════

// error.type 取值（**闭集**；规范：SHOULD 匹配 provider/客户端库错误码、异常规范名，或另一个
// **低基数**错误标识；instrumentation SHOULD 文档化其上报的错误清单；兜底 `_OTHER`）。
// 纪律：**不要把内部判词/自由文本塞进来**（那是高基数 + 会污染后端语义）。
const (
	ErrTypeOther           = "_OTHER"               // 规范兜底值（认不出的分类码）
	ErrTypeUpstreamTimeout = "upstream_timeout"     // 上游超时
	ErrTypeUpstreamFail    = "upstream_fail"        // 上游 5xx/upstream
	ErrTypeStreamBroken    = "stream_broken"        // 流中断
	ErrTypeStreamTruncated = "stream_truncated"     // 流被截断（未收 [DONE]）
	ErrTypeBadRequest      = "bad_request"          // 请求不合法
	ErrTypeClientAborted   = "client_aborted"       // 客户端主动中断
	ErrTypeToolExec        = "tool_execution_error" // 工具执行失败
	ErrTypeCompactionFail  = "compaction_failed"    // 压缩失败
)

// obsErrTypeTable — 分类码 → error.type（逐条登记；未登记 ⇒ _OTHER，**不按前缀猜**）。
var obsErrTypeTable = map[string]string{
	ChatErrUpstreamTimeout: ErrTypeUpstreamTimeout,
	ChatErrUpstreamFail:    ErrTypeUpstreamFail,
	ChatErrStreamBroken:    ErrTypeStreamBroken,
	ChatErrStreamTruncated: ErrTypeStreamTruncated,
	ChatErrBadRequest:      ErrTypeBadRequest,
	ChatErrClientAborted:   ErrTypeClientAborted,
	ChatErrOther:           ErrTypeOther,
}

// ObsErrorTypeOf — 分类码 → error.type（未知 ⇒ `_OTHER`）。
func ObsErrorTypeOf(code string) string {
	if t, ok := obsErrTypeTable[code]; ok {
		return t
	}
	return ErrTypeOther
}

// obsErrorTypeForRecord — **每个出错 span 都要 error.type**（不只末事件）的唯一判定点。
//
// 口径（写死）：
//   - turn：终局是 failed / incomplete ⇒ 落（取分类码）；**cancelled 不落** ——
//     客户端主动中断不是"操作出错"（规范原文：if the operation ended in an error）；
//   - tool：执行失败（result=err）⇒ tool_execution_error（判定层 deny **不算**出错：工具没跑）；
//   - compact：失败终态（三态事件的 compaction_failed 或旧 OBS-4 的 result=fail）⇒ compaction_failed；
//   - 其余（成功/未终局/无错误语义）⇒ 空（缺席，不编造）。
func obsErrorTypeForRecord(r *ObsRecord) string {
	switch r.Kind {
	case obsKindTurn:
		if ObsTerminalOf(r.EndReason) == ObsTerminalFailed || ObsTerminalOf(r.EndReason) == ObsTerminalIncomplete {
			return ObsErrorTypeOf(r.EndReason)
		}
		return ""
	case obsKindTool:
		if r.Result == "err" {
			return ErrTypeToolExec
		}
		return ""
	case obsKindCompact:
		if r.EventName == obsEventCompactionFailed || r.Result == "fail" {
			return ErrTypeCompactionFail
		}
		return ""
	default:
		return ""
	}
}

// ══════════════ ⑥ 四终局 / 生命周期 → gen_ai.response.status（B12）══════════════

// gen_ai.response.status 枚举（标准六值）
const (
	GenAIStatusQueued     = "queued"
	GenAIStatusInProgress = "in_progress"
	GenAIStatusCompleted  = "completed"
	GenAIStatusIncomplete = "incomplete"
	GenAIStatusFailed     = "failed"
	GenAIStatusCancelled  = "cancelled"
)

// **我们的四终局**（终态集合；与标准的非终态 queued/in_progress 相对）。
// 设计稿 v1.0 §3.2 S6 的"四终局"在对话链路上的落点：一个轮次只可能以这四种之一收尾。
const (
	ObsTerminalCompleted  = GenAIStatusCompleted  // 正常收尾
	ObsTerminalFailed     = GenAIStatusFailed     // 出错收尾（超时/上游失败/流中断/坏请求/兜底）
	ObsTerminalCancelled  = GenAIStatusCancelled  // 客户端中断
	ObsTerminalIncomplete = GenAIStatusIncomplete // 未完成收尾（流被截断 / 长度截断）
)

// obsTerminalTable — 收尾原因 → 四终局（**逐条**登记，不按前缀猜；未登记 ⇒ 缺席）。
var obsTerminalTable = map[string]string{
	"finish":               ObsTerminalCompleted,
	ChatErrUpstreamTimeout: ObsTerminalFailed,
	ChatErrUpstreamFail:    ObsTerminalFailed,
	ChatErrStreamBroken:    ObsTerminalFailed,
	ChatErrBadRequest:      ObsTerminalFailed,
	ChatErrOther:           ObsTerminalFailed,
	ChatErrClientAborted:   ObsTerminalCancelled,
	ChatErrStreamTruncated: ObsTerminalIncomplete,
	"length":               ObsTerminalIncomplete, // 长度截断（上游 finish_reason=length 时同义）
}

// ObsTerminalOf — 收尾原因 → 四终局之一；认不出 ⇒ 空串（**缺席，不编造**）。
func ObsTerminalOf(endReason string) string {
	if t, ok := obsTerminalTable[endReason]; ok {
		return t
	}
	return ""
}

// ObsResponseStatusOf — 收尾原因 → gen_ai.response.status。
// 终态时等于四终局；非终态只有两个：排队（queued）与进行中（in_progress）。
// 认不出 ⇒ 空串（缺席）。
func ObsResponseStatusOf(endReason string) string {
	switch endReason {
	case GenAIStatusQueued:
		return GenAIStatusQueued
	case GenAIStatusInProgress:
		return GenAIStatusInProgress
	}
	return ObsTerminalOf(endReason)
}

// obsStatusForRecord — 记录级状态判定（终态才落；成功=completed）。
func obsStatusForRecord(r *ObsRecord) string {
	switch r.Kind {
	case obsKindTurn:
		if r.EndReason == "" {
			return ""
		}
		return ObsResponseStatusOf(r.EndReason)
	case obsKindCompact:
		switch {
		case r.EventName == obsEventCompactionCompleted || r.Result == "ok":
			return GenAIStatusCompleted
		case r.EventName == obsEventCompactionFailed || r.Result == "fail":
			return GenAIStatusFailed
		}
		return ""
	default:
		return ""
	}
}

// ══════════════ ⑦ tool.type 分类（B5）══════════════

// gen_ai.tool.type 枚举（照抄规范：function=客户端执行 / extension=agent 侧执行、桥接外部 API /
// datastore=访问结构化或非结构化外部数据）
const (
	GenAIToolTypeFunction  = "function"
	GenAIToolTypeExtension = "extension"
	GenAIToolTypeDatastore = "datastore"
)

// obsToolTypeTable — **逐条登记**（新增工具须在此登记；没登记的一律缺席，不猜）。
var obsToolTypeTable = map[string]string{
	"kb_search":      GenAIToolTypeDatastore,
	"doc_search":     GenAIToolTypeDatastore,
	"memory":         GenAIToolTypeDatastore,
	"session_search": GenAIToolTypeDatastore,
	"skill_load":     GenAIToolTypeDatastore,
	"web_search":     GenAIToolTypeExtension,
	"web_fetch":      GenAIToolTypeExtension,
	"spawn_agent":    GenAIToolTypeExtension,
}

// ObsToolTypeOf — 工具 → gen_ai.tool.type；未登记 ⇒ 空串（缺席，不编造）。
func ObsToolTypeOf(name string) string {
	if name == "" {
		return ""
	}
	if t, ok := obsToolTypeTable[name]; ok {
		return t
	}
	if _, ok := chatToolMetaByName[name]; ok { // 注册表里的本地执行工具
		return GenAIToolTypeFunction
	}
	if IsExtraTool(name) { // chat 层实现的可执行工具
		return GenAIToolTypeFunction
	}
	return ""
}

// obsToolDescription — gen_ai.tool.description（Recommended If available）：查得到才落。
// 口径：查不到（或兜底返回工具名自身）⇒ 空 = 键缺席；截断 300 字符（不撑爆观测）。
// 纪律：本函数只读注册表；任何岔子都吞掉（观测面绝不把对话搞崩）。
func obsToolDescription(name string) (desc string) {
	if name == "" {
		return ""
	}
	defer func() {
		if v := recover(); v != nil {
			log.Printf("⚠️ obs_semconv: tool description lookup panic (忽略): %v", v)
			desc = ""
		}
	}()
	d := residentToolDesc(name)
	if d == name { // 兜底返回工具名自身 ⇒ 视为"没描述"
		return ""
	}
	return trunca(d, 300)
}

// ══════════════ ⑧ server.address / server.port（B11）══════════════

var (
	obsInferEndpointMu sync.RWMutex
	obsInferEndpoint   string // 我们**实际拨号**的对端（网关基址）；空 = 未知 ⇒ 键缺席
)

// ObsSetInferEndpoint — 登记推理对端（best-effort；空串 = 清除）。创建 ChatInfer 时调用一次。
//
// 口径（写死）：值必须是**我们真的会拨**的地址；规范建议报"中间层之后的真实服务端"，
// 而网关之后的推理节点在本进程**不可知** ⇒ 我们只报拨号对端，不猜下游节点（多节点定位
// 由网关侧记录补，见 T1.6 传播）。拿不到 ⇒ 键缺席。
func ObsSetInferEndpoint(raw string) {
	obsInferEndpointMu.Lock()
	obsInferEndpoint = strings.TrimSpace(raw)
	obsInferEndpointMu.Unlock()
}

// obsInferEndpointOf — 当前登记的推理对端。
func obsInferEndpointOf() string {
	obsInferEndpointMu.RLock()
	defer obsInferEndpointMu.RUnlock()
	return obsInferEndpoint
}

// ObsServerFromURL — URL → (server.address, server.port)。端口拿不到 ⇒ nil（键缺席，不写 0）。
func ObsServerFromURL(raw string) (string, *int) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", nil
	}
	host := u.Hostname()
	if host == "" {
		return "", nil // 认不出主机 ⇒ 缺席（不拿整串 URL 冒充地址）
	}
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			return host, &n
		}
	}
	return host, nil
}

// ══════════════ ⑨ conversation.compacted（B10：只 true 不得设 false）══════════════

var (
	obsCompactedMu   sync.RWMutex
	obsCompactedSess = map[string]bool{} // 会话级：压缩发生过（有界，见下）
	obsCompactedMax  = 4096
)

// ObsMarkConversationCompacted — 记下"这个会话的上下文被压过"（只置 true，没有反向操作）。
// 有界：表满时整体清空（观测可容忍一次丢失；内存不可无界 —— 与 obsTraceTab 同纪律）。
func ObsMarkConversationCompacted(session string) {
	if session == "" {
		return
	}
	obsCompactedMu.Lock()
	defer obsCompactedMu.Unlock()
	if len(obsCompactedSess) >= obsCompactedMax {
		obsCompactedSess = map[string]bool{}
	}
	obsCompactedSess[session] = true
}

// obsConversationCompacted — 本会话是否已压缩过（拿不到/没压过 ⇒ false，**不落 false**）。
func obsConversationCompacted(session string) bool {
	if session == "" {
		return false
	}
	obsCompactedMu.RLock()
	defer obsCompactedMu.RUnlock()
	return obsCompactedSess[session]
}

// ══════════════ ⑩ 唯一补齐点（obsWrite 调用；T6.1 + T6.2 的全部落点）══════════════

// fillSemconv — T6.1/T6.2：把标准字段补进记录（调用方已给的一律不覆盖）。
//
// 为什么放在唯一写入口（obsWrite）：范式判据"**每个出错 span 都要 error.type**"、
// "每条记录都带 semconv 版本"必须是**构造性成立**的，不能靠每个调用点自觉。
func (r *ObsRecord) fillSemconv() {
	// ① semconv 基准（B15：常量字段，供日后兼容检查）
	r.ZergSemconvVersion = SemconvGenAIVersion
	r.ZergSemconvCommit = SemconvGenAICommit

	// ② 内容捕获档位（我们四档 → 标准枚举；非法值回最严且**留痕**）
	lvl, std, fallback := ObsCaptureResolved()
	r.ZergCaptureLevel = lvl
	r.ZergCaptureStd = std
	r.ZergCaptureFallback = fallback

	// ③ conversation.id：有会话才有；**不许**用新 UUID / trace_id / 内容 hash 兜底（B10）
	if r.Session != "" && r.GenAIConversationID == "" {
		r.GenAIConversationID = r.Session
	}

	// ④ 操作名 / 标准 span 名（映射表；无对应项的 kind 一律不落 —— 不冒充）
	if m, ok := ObsSpanMappingOf(r.Kind); ok {
		if r.GenAIOperationName == "" {
			r.GenAIOperationName = m.Operation
		}
		if r.ZergOTelSpanName == "" {
			r.ZergOTelSpanName = ObsSpanName(r.Kind, r.Tool, r.Model)
		}
	}

	// ⑤ 推理类记录（turn/compact/prompt）：请求模型 + 思考深度 + 供应商
	if r.Kind == obsKindTurn || r.Kind == obsKindCompact || r.Kind == obsKindPrompt {
		if r.Model != "" && r.GenAIRequestModel == "" {
			r.GenAIRequestModel = r.Model
		}
		if r.GenAIRequestReasoningLevel == "" {
			r.GenAIRequestReasoningLevel = reasoningEffortChat // 与请求体同源（chat_infer.go）
		}
	}
	if (r.Kind == obsKindTurn || r.Kind == obsKindCompact) && r.GenAIProviderName == "" {
		r.GenAIProviderName = ProviderZergLocal
	}
	// ⑤′ 输出模态：对话只产文本（无 image/speech 通路）⇒ text（将来接结构化输出走覆盖字段）
	if r.Kind == obsKindTurn && r.GenAIOutputType == "" {
		r.GenAIOutputType = GenAIOutputTypeText
	}

	// ⑥ tool 三件套（call.id 由调用方给：没有就是没有，不编造）
	if r.Tool != "" {
		if r.GenAIToolName == "" {
			r.GenAIToolName = r.Tool
		}
		if r.GenAIToolType == "" {
			r.GenAIToolType = ObsToolTypeOf(r.Tool)
		}
		if r.GenAIToolDescription == "" {
			r.GenAIToolDescription = obsToolDescription(r.Tool)
		}
	}

	// ⑦ prompt 两件套（名字/版本是调用方声明的事实 ⇒ 声明不了就缺席）
	if r.PromptName != "" && r.GenAIPromptName == "" {
		r.GenAIPromptName = r.PromptName
	}
	if r.PromptVersion != "" && r.GenAIPromptVersion == "" {
		r.GenAIPromptVersion = r.PromptVersion
	}

	// ⑧ 生命周期状态（B12）：终态才落；认不出 ⇒ 缺席
	if r.GenAIResponseStatus == "" {
		r.GenAIResponseStatus = obsStatusForRecord(r)
	}
	// ⑧′ 四终局 / verdict 留在 zerg.*（B12：**不得**塞进 error.type 或冒充标准属性）
	if r.Kind == obsKindTurn {
		if r.ZergTerminal == "" {
			r.ZergTerminal = ObsTerminalOf(r.EndReason)
		}
		if r.ZergVerdict == "" && r.Turn != nil {
			r.ZergVerdict = r.Turn.Verdict
		}
	}

	// ⑨ conversation.compacted：**只能 true**（显式 false 在此被剥掉 —— 规范 SHOULD NOT set false）
	if r.GenAIConversationCompacted != nil && !*r.GenAIConversationCompacted {
		r.GenAIConversationCompacted = nil
	}
	if r.GenAIConversationCompacted == nil {
		if r.Kind == obsKindCompact && (r.EventName == obsEventCompactionCompleted || r.Result == "ok") {
			r.GenAIConversationCompacted = obsTruePtr() // 压缩确实发生 ⇒ 设 true
			ObsMarkConversationCompacted(r.Session)     // 且此后本会话都算"压过"
		} else if obsConversationCompacted(r.Session) {
			r.GenAIConversationCompacted = obsTruePtr() // 本会话压过 ⇒ 仍只设 true
		}
	}

	// ⑩ error.type：**每个出错 span 都要**（B4；不只末事件）
	if r.ErrorType == "" {
		r.ErrorType = obsErrorTypeForRecord(r)
	}

	// ⑪ server.address / server.port（B11：Stable；多节点定位）
	if r.ServerAddress == "" {
		if addr, port := ObsServerFromURL(obsInferEndpointOf()); addr != "" {
			r.ServerAddress = addr
			r.ServerPort = port
		}
	}
}
