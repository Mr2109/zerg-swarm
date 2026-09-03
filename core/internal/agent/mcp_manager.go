package agent

// mcp_manager.go — v2.5.1 CA 接入 MCP（P0——mark3labs/mcp-go——codegraph 首接）
// 设计：docs/设计-v2.5.1-CA接入skill-MCP.md
// 架构：CA 作为 MCP 客户端——连 codegraph serve（stdio）——工具动态注册

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPManager — MCP 服务器管理器（注册表 + 工具动态注册）
type MCPManager struct {
	clients map[string]*mcpClient // server 名 → 客户端
}

type mcpClient struct {
	c     *client.Client
	tools []mcp.Tool // 服务器提供的工具（动态注册用）
}

// NewMCPManager — 创建管理器（注册表）
func NewMCPManager() *MCPManager {
	return &MCPManager{clients: make(map[string]*mcpClient)}
}

// Connect — 连接一个 MCP 服务器（stdio——如 codegraph serve）
func (mm *MCPManager) Connect(name string, command string, args ...string) error {
	c, err := client.NewStdioMCPClient(command, []string{}, args...)
	if err != nil {
		return fmt.Errorf("MCP 客户端创建失败 %s: %w", name, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "zerg-agent",
		Version: "v2.5.1",
	}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		c.Close()
		return fmt.Errorf("MCP 初始化失败 %s: %w", name, err)
	}

	toolsReq := mcp.ListToolsRequest{}
	tools, err := c.ListTools(ctx, toolsReq)
	if err != nil {
		c.Close()
		return fmt.Errorf("MCP 工具列表失败 %s: %w", name, err)
	}

	mm.clients[name] = &mcpClient{c: c, tools: tools.Tools}
	return nil
}

// ConnectHTTP — 连接远程 HTTP MCP 服务器（v2.5.1——anysearch 等）
// baseURL: https://api.anysearch.com/mcp——headers: 认证（Authorization Bearer）
func (mm *MCPManager) ConnectHTTP(name string, baseURL string, headers map[string]string) error {
	opts := []transport.StreamableHTTPCOption{}
	if len(headers) > 0 {
		opts = append(opts, transport.WithHTTPHeaders(headers))
	}
	c, err := client.NewStreamableHttpClient(baseURL, opts...)
	if err != nil {
		return fmt.Errorf("MCP HTTP 客户端创建失败 %s: %w", name, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "zerg-agent",
		Version: "v2.5.1",
	}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		c.Close()
		return fmt.Errorf("MCP HTTP 初始化失败 %s: %w", name, err)
	}

	toolsReq := mcp.ListToolsRequest{}
	tools, err := c.ListTools(ctx, toolsReq)
	if err != nil {
		c.Close()
		return fmt.Errorf("MCP HTTP 工具列表失败 %s: %w", name, err)
	}

	mm.clients[name] = &mcpClient{c: c, tools: tools.Tools}
	return nil
}

// ToolDefs — 返回 MCP 工具定义（v2.5.1 分层——"只展示必要+更多选项"）
//   extended=false: 只返回核心工具（当前 kb 的 4 个核心——codegraph/anysearch 无核心→空）
//   extended=true:  返回全部（tool_search 用——模型搜索发现扩展）
func (mm *MCPManager) ToolDefs(extended bool) []ToolDef {
	var defs []ToolDef
	for serverName, mc := range mm.clients {
		for _, t := range mc.tools {
			// 分层（v2.5.1——"只展示必要+更多选项"——模型需要时扩展）
			if !extended && !isCoreTool(serverName, t.Name) {
				continue // 扩展工具 deferred——tool_search 搜到才注入
			}
			// ToolInputSchema 结构体 → map[string]any（JSON 中转）
			params := map[string]any{}
			if raw, err := json.Marshal(t.InputSchema); err == nil {
				json.Unmarshal(raw, &params)
			}
			// 什么时候用按 server 类型区分（codegraph=查代码/kb=查经验/anysearch=查网络）
			whenUse := "按需使用（工具描述含触发条件）"
			examples := ""
			switch serverName {
			case "codegraph":
				whenUse = "查代码结构/调用关系/符号定义/死代码/影响分析时——优先用本工具（比 read/grep 高效百倍）"
				examples = "\n【示例】查 MemoryStore 谁调用 → callers(symbol=MemoryStore) → 返回调用者列表；查死代码 → callers(函数名) 返回空=死代码候选；列文件函数 → node(file=文件名, symbolsOnly=true)；按名找符号 → search(query=符号名)"
			case "kb", "knowledge":
				whenUse = "查知识库经验/坑/教训/历史记录时——用本工具（执行任务前先查经验——避免重复踩坑）"
				examples = "\n【示例】执行任务前查经验 → kb_search(query=任务关键词)；定位后读全文 → kb_read(id=命中id)；写经验 → kb_add(domain=虫族, content=正文)"
			case "anysearch":
				whenUse = "查网络最新信息/调研/查证/找资料时——用本工具（比 web_search 更强——支持垂直领域）"
				examples = "\n【示例】调研某主题 → mcp_anysearch_search(query=主题词)；批量搜多关键词 → batch_search(queries=[词1,词2])"
			}
			defs = append(defs, ToolDef{
				Type: "function",
				Function: FunctionDef{
					Name:        "mcp_" + serverName + "_" + t.Name,
					Description: fmt.Sprintf("[MCP %s] %s\n【什么时候用】%s%s", serverName, t.Description, whenUse, examples),
					Parameters:  params,
				},
			})
		}
	}
	return defs
}

// isCoreTool — 核心工具判定（默认暴露——够 90% 场景）
// kb 核心 4 个（查经验/读/写/排障沉淀）——v2.5.5 T4: anysearch 核心 2 个也常驻（查网络——比 web_search 快 100 倍）
func isCoreTool(serverName, toolName string) bool {
	switch serverName {
	case "kb":
		switch toolName {
		case "kb_search", "kb_read", "kb_add", "kb_capture_fix":
			return true
		}
	case "anysearch":
		// v2.5.5 T4（Mr2109——工具库丰富——非替代）: anysearch 核心常驻——CA 直接用（不用 tool_search 发现）
		// 网络调研/查证/疑难杂症首选——300ms vs web_search 30s
		switch toolName {
		case "search", "batch_search":
			return true
		}
	}
	return false
}

// Call — 调用 MCP 工具（server 名 + 工具名 + 参数）
func (mm *MCPManager) Call(serverName, toolName string, args map[string]any) (string, error) {
	mc, ok := mm.clients[serverName]
	if !ok {
		return "", fmt.Errorf("MCP 服务器 %s 未连接", serverName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = toolName
	// Arguments 直接传 map（mcp-go 序列化为 JSON 对象——不能用 bytes——会变字符串）
	callReq.Params.Arguments = args

	result, err := mc.c.CallTool(ctx, callReq)
	if err != nil {
		return "", fmt.Errorf("MCP 调用失败 %s/%s: %w", serverName, toolName, err)
	}

	// 提取文本内容
	var sb []byte
	for _, content := range result.Content {
		if text, ok := content.(mcp.TextContent); ok {
			sb = append(sb, text.Text...)
			sb = append(sb, '\n')
		}
	}
	if len(sb) == 0 {
		return fmt.Sprintf("（MCP 返回无文本内容——isError=%v）", result.IsError), nil
	}
	return string(sb), nil
}

// Close — 关闭所有连接
func (mm *MCPManager) Close() {
	for _, mc := range mm.clients {
		mc.c.Close()
	}
}
