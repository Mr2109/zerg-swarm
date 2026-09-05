// loop.go - 虫族 v2.5 Agent 循环状态机（M1c：灵魂循环）
// 对齐：Claude Code query() 循环 + Loop Engineering 十四循环
// 设计：for 循环 → callModel → 解析工具调用 → 执行 → 回喂 → 终止判断
// 轮次上限（MaxTurns）+ token 预算（Budget）+ 无进展检测 + 错误重试

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"zerg/core/internal/agentstate"
	"zerg/core/internal/compressor"
	"zerg/core/internal/loopguard"
)

// 循环结果

// LoopResult - 循环执行完毕后的结果
type LoopResult struct {
	Reason    TerminateReason // 终止原因
	Turns     int             // 执行轮数
	Tokens    int64           // 累计 token
	Content   string          // 最终内容
	ToolTrace []string        // v2.5.1 工具调用轨迹（挂单诊断用）
}

// 循环状态机

// Loop - 启动 Agent 循环状态机
// 流程：调模型 → 解析工具调用 → 执行 → 回喂 → 重复
// 终止条件：complete/user_abort/token_budget/max_turns/stop_hook/max_retries/model_error/tool_error/escalate/blocked
// newLoop — 初始化循环状态（#11 拆解——原 Loop 内联初始化）
func newLoop(agent *Agent, tools []ToolDef, logger *Logger, state *agentstate.HarnessState, maxTurns int, budget int64, noProgressThresh int) *loopState {
	if maxTurns <= 0 {
		maxTurns = 100 // v2.5：30→100——安全兜底；自主停止主导（研究类任务需 100+）
	}
	if noProgressThresh <= 0 {
		noProgressThresh = 3 // v2.5.5 修复（2026-08-24）: 2→3——模型思考/准备阶段无工具调用被误判无进展（1a 3轮正常被终止）——3 轮更合理
	}
	if budget <= 0 {
		budget = agent.cfg.Budget
	}
	// v2.5.4.9 机器级采样（每轮——sysmetrics.jsonl——X3 状态地址从环境取）
	var sm *SysMetricsCollector
	if logger != nil {
		if c, err := NewSysMetricsCollector(logger.dir, os.Getenv("ZERG_X3_STATUS"), os.Getenv("ZERG_X3_TOKEN")); err == nil {
			sm = c
		}
	}
	lg := loopguard.New(loopguard.Config{})
	return &loopState{
		agent:            agent,
		tools:            tools,
		logger:           logger,
		state:            state,
		maxTurns:         maxTurns,
		budget:           budget,
		noProgressThresh: noProgressThresh,
		sysmetrics:       sm,
		guard:            lg,
	}
}

// Loop 运行 Agent 主循环——每轮: 模型调用→工具执行→无进展检测，直到终止条件或 maxTurns 用尽
// v2.5.1 挂单诊断: defer 捕获 ToolTrace 补全（所有出口都带完整工具轨迹）
func Loop(ctx context.Context, agent *Agent, tools []ToolDef, logger *Logger, state *agentstate.HarnessState, maxTurns int, budget int64, noProgressThresh int) (result LoopResult) {
	// 2026-09-05 内核开关: ZERG_LOOPCORE=1 走 loopcore 内核（CATerminator 仲裁）——默认关（对照验证——稳定后转正）
	if os.Getenv("ZERG_LOOPCORE") == "1" {
		return agent.RunWithKernel(ctx, tools, logger, state, maxTurns, budget, noProgressThresh)
	}
	var ls *loopState // v2.5.1 defer 捕获（ToolTrace 补全）
	defer func() {
		// v2.5.1 挂单诊断：所有出口补 ToolTrace
		if len(result.ToolTrace) == 0 && ls != nil && len(ls.toolTrace) > 0 {
			result.ToolTrace = ls.toolTrace
		}
	}()
	ls = newLoop(agent, tools, logger, state, maxTurns, budget, noProgressThresh)

	// 1. 启动循环日志
	ls.logger.LogEvent(string(EventLoopStart), "info", "loop_start",
		"", fmt.Sprintf("循环启动: maxTurns=%d, budget=%d, noProgress=%d",
			ls.maxTurns, ls.budget, ls.noProgressThresh),
		nil, "", "", "")

	// 2. 进入循环
	for {
		// v2.5.4.9 滚动窗口压缩：每轮开始——压缩超窗口的最早轮（Mr2109思路——保留5轮+滚动压最早）
		agent.maybeCompact()
		// 终止条件检查
		if reason := ls.checkStop(ctx); reason != "" {
			// v2.5 系统回馈（Reflexion）：max_turns/blocked 且未诊断过——回馈诊断让模型纠正
			if (reason == ReasonMaxTurns || reason == ReasonBlocked) && !ls.diagnosed && ls.turn >= 3 {
				ls.diagnosed = true
				feedback := ls.diagnoseFailure()
				ls.logger.LogEvent(string(EventLoopEnd), "warn", "diagnose_feedback",
					"", fmt.Sprintf("系统回馈: %s", feedback), nil, "", "", "")
				fmt.Fprintf(os.Stderr, "🔄 %s\n", feedback)
				agent.AppendMessage(Message{Role: "user", Content: feedback})
				ls.noProgressCount = 0 // 重置无进展（模型收到新指引）
				ls.exploreCount = 0
				ls.verifyCount = 0
				ls.noActionCount = 0
				ls.lastReadPath = ""
				continue
			}
			ls.logger.LogEvent(string(EventLoopEnd), "warn", "loop_terminate",
				"", fmt.Sprintf("循环终止: reason=%s", reason),
				nil, "", "", "")
			return LoopResult{Reason: reason, Turns: ls.turn, Tokens: ls.totalTokens}
		}

		ls.turn++

		// v2.5.1: 上下文压缩（C 方案——onnx 语义压缩——用户定 50% 触发）
		// Agent 用 orn 模型——遵循最大上下文 50% 压缩（131072 窗口 → 65536 触发）
		if ls.shouldCompact() {
			if err := ls.compactHistory(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️ 上下文压缩失败（忽略继续）: %v\n", err)
			}
		}

		// 3. 调模型（带重试）
		resp, err := ls.callModelWithRetry(ctx)
		if err != nil {
			ls.logger.LogEvent(string(EventToolCall), "error", "model_failed",
				"", fmt.Sprintf("模型调用失败: %v", err),
				nil, err.Error(), "", "")
			return LoopResult{Reason: ReasonModelError, Turns: ls.turn - 1, Tokens: ls.totalTokens}
		}

		// 4. 更新 token 计数
		ls.totalTokens += resp.Usage.TotalTokens

		// 5. 无工具调用 → 检查 checker + 确认完成（v2.5.4.9 拆小——可维护性）
		if len(resp.ToolCalls) == 0 {
			if done, result := ls.handleNoToolCalls(resp); done {
				return result
			}
		}

		// 6. 执行工具（逐个）
		var toolResults []ToolCallResult
		for _, tc := range resp.ToolCalls {
			// 2026-09-05 LoopGuard 接入（对齐对话系统——重复/交替→引导换招→升级收尾）
			if ls.guard != nil {
				ls.guard.Record(tc.Name, tc.Args)
				if ok, reason := ls.guard.Detect(); ok {
					guide, upgrade := ls.guard.BuildGuide(reason, toolNames(ls.tools))
					if upgrade {
						ls.logger.LogEvent(string(EventLoopEnd), "warn", "loopguard_escalate",
							"", "循环守卫升级收尾: "+reason, nil, "", "", "")
						ls.agent.AppendMessage(Message{Role: "user", Content: guide})
						// 升级=终止（引导消息已进历史——外层 CLI 会话恢复可续——这里按 blocked 走 Reflexion）
						return LoopResult{Reason: ReasonBlocked, Turns: ls.turn, Tokens: ls.totalTokens}
					}
					ls.logger.LogEvent(string(EventToolCall), "warn", "loopguard_guide",
						tc.Name, "循环守卫引导: "+reason, nil, "", "", "")
					ls.agent.AppendMessage(Message{Role: "user", Content: guide})
				}
			}
			result, execErr := ls.executeTool(ctx, tc)
			if execErr != nil {
				ls.logger.LogEvent(string(EventToolError), "error", "tool_exec_failed",
					tc.Name, fmt.Sprintf("第 %d 轮: %s 执行失败: %v", ls.turn, tc.Name, execErr),
					nil, execErr.Error(), "", "")
				// 工具执行失败，回喂错误信息给模型继续（含纠正提示——v2.5）
				errMsg := fmt.Sprintf("工具 %s 执行失败: %v", tc.Name, execErr)
				if tc.Name == "write" || tc.Name == "edit" || tc.Name == "read" {
					errMsg += "（提示：请用相对路径，如 src/main.py——不要用绝对路径）"
				}
				errMsgMsg := Message{Role: "user", Content: errMsg}
				agent.AppendMessage(errMsgMsg)
				continue
			}
			toolResults = append(toolResults, result)
			ls.toolTrace = append(ls.toolTrace, tc.Name) // v2.5.1 挂单诊断轨迹
			ls.logger.LogEvent(string(EventToolCall), "info", "tool_execute",
				tc.Name, fmt.Sprintf("第 %d 轮: 调用 %s", ls.turn, tc.Name),
				tc.Args, result.Content, result.Error, result.Duration)
			// 调试：打印工具执行结果（v2.5 排查——写文件失败）
			fmt.Fprintf(os.Stderr, "[tool] %s args=%v → ok=%s err=%s\n", tc.Name, tc.Args,
				truncate(result.Content, 80), result.Error)

			// v2.5 系统回馈统计（探索/验证分类）
			switch tc.Name {
			case "ls", "glob", "grep":
				ls.exploreCount++
			case "read":
				// 验证计数：重复读同一文件才算验证（正常读文件定位不算）
				path, _ := tc.Args["path"].(string)
				if path != "" && path == ls.lastReadPath {
					ls.verifyCount++
				}
				ls.lastReadPath = path
			}

			// v2.5 系统回馈：探索过度中途触发（不等 max_turns——Codex 阈值 5）
			if ls.exploreCount >= 5 && !ls.diagnosed {
				ls.diagnosed = true
				feedback := "【系统诊断】你已调用 " + fmt.Sprintf("%d", ls.exploreCount) + " 次探索工具（ls/glob/grep）但未执行实际修改。任务需要动作：read 读目标文件 → edit/write 修改 → bash 验证。请立即停止探索，直接修改目标。"
				ls.logger.LogEvent(string(EventLoopEnd), "warn", "diagnose_feedback",
					"", fmt.Sprintf("系统回馈: %s", feedback), nil, "", "", "")
				fmt.Fprintf(os.Stderr, "🔄 %s\n", feedback)
				agent.AppendMessage(Message{Role: "user", Content: feedback})
			}
		}

		// 8. 记录助手消息（v2.5：assistant 先于 tool——Responses API 要求 call 先 output 后）
		assistantMsg := Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		}
		agent.AppendMessage(assistantMsg)

		// 7. 回喂工具结果到历史（独立索引——失败的工具调用无结果不回喂 tool 消息）
		for i, tc := range resp.ToolCalls {
			if i >= len(toolResults) {
				break // 该工具调用执行失败（错误已回喂为 user 消息）——无结果
			}
			toolMsg := Message{
				Role:       "tool",
				Content:    toolResults[i].Content,
				ToolCallID: tc.ID,
			}
			if tc.Name == "read" && toolResults[i].Error == "" {
				// v2.5 读后行动引导（对齐 Codex——模型读后需明确下一步）
				toolMsg.Content += "\n\n[读后引导] 已读取文件内容。请基于内容采取行动：若任务要求修改/创建文件——立即用 edit/write；若需要进一步信息——用 grep/glob 定位；若已理解——直接生成所需内容。"
			}
			if (tc.Name == "write" || tc.Name == "edit") && toolResults[i].Error == "" {
				// v2.5 完成验证（Completeness Verifier——研究对齐：确定性验证 > 提示引导）
				// write/edit 成功后验证文件真实存在——验证通过记录"产出已验证"
				if path, ok := tc.Args["path"].(string); ok {
					abs := filepath.Join(ls.agent.execContext.WorkDir, path)
					if info, err := os.Stat(abs); err == nil && !info.IsDir() {
						ls.verifiedOutput = true
						toolMsg.Content += fmt.Sprintf("\n\n[验证] 文件 %s 已确认存在（%d 字节）——产出已验证。任务核心目标已达成——请输出总结结束任务（除非还有明确的后续步骤）。", path, info.Size())
					}
				}
			}
			if toolResults[i].Error != "" {
				toolMsg.Content = toolResults[i].Error
			}
			agent.AppendMessage(toolMsg)
		}

		// 9. 更新无进展计数
		ls.updateNoProgress(toolResults)

		// 10. 写状态
		if state != nil {
			state.AddEvidence(
				fmt.Sprintf("第 %d 轮调用工具: %s", ls.turn, ls.summarizeToolCalls(resp.ToolCalls)),
				"工具已执行，结果已回喂", "", "继续循环")
		}

		ls.logger.LogEvent(string(EventLoopEnd), "info", "loop_iteration_end",
			"", fmt.Sprintf("第 %d 轮完成, tokens=%d, tools=%d", ls.turn, ls.totalTokens, len(toolResults)),
			nil, "", "", "")

		// v2.5.4.9 机器级采样（每轮——CA 全量追踪——sysmetrics.jsonl）
		if ls.sysmetrics != nil {
			ls.sysmetrics.Sample(ls.turn)
		}

		ls.turn++
	}
}

// 终止条件

// checkStop - 每轮开始前检查终止条件
func (ls *loopState) checkStop(ctx context.Context) TerminateReason {
	// 1. 用户中止
	select {
	case <-ctx.Done():
		return ReasonUserAbort
	default:
	}

	// 2. 轮数上限
	if ls.turn >= ls.maxTurns {
		return ReasonMaxTurns
	}

	// 3. token 预算
	if ls.budget > 0 && ls.totalTokens >= ls.budget {
		return ReasonTokenBudget
	}

	// 4. 无进展
	if ls.noProgressCount >= ls.noProgressThresh {
		return ReasonBlocked
	}

	return ""
}

// 模型调用

// callModelWithRetry - 带指数退避重试的模型调用
func (ls *loopState) callModelWithRetry(ctx context.Context) (*ModelResponse, error) {
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			ls.logger.LogEvent(string(EventToolCall), "warn", "model_retry",
				"", fmt.Sprintf("第 %d 次重试，退避 %v", attempt, backoff),
				nil, "", "", "")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		// v2.5.1 系统提示 + 技能描述（渐进式——只 name+description——触发才读正文）
		sysPrompt := systemPrompt()
		if ls.agent.skillMgr != nil {
			if skillDescs := ls.agent.skillMgr.ListDescriptions(); skillDescs != "" {
				sysPrompt += "\n\n" + skillDescs
			}
		}
		resp, err := ls.agent.callModel(ctx, sysPrompt, ls.tools)
		if err == nil {
			ls.retryCount = 0
			return resp, nil
		}

		lastErr = err
		ls.retryCount++
	}

	return nil, fmt.Errorf("模型调用 %d 次后仍失败: %w", maxRetries+1, lastErr)
}

// 工具执行

// toolSearch — 按需发现工具（v2.5.1——对齐 Codex tool_search）
// MCP 工具默认 deferred（不进初始工具列表）——模型搜索发现后注入可用
func (ls *loopState) toolSearch(query string) (ToolCallResult, error) {
	mcpDefs := ls.agent.mcpMgr.ToolDefs(true)
	// 已在工具列表的跳过（不重复注入）
	have := map[string]bool{}
	for _, t := range ls.tools {
		have[t.Function.Name] = true
	}
	// 匹配查询（描述包含关键词）
	var matched []ToolDef
	for _, t := range mcpDefs {
		if have[t.Function.Name] {
			continue
		}
		// 简单关键词匹配（描述 + 名字——按 server 类型——v2.5.1）
		desc := t.Function.Description + " " + t.Function.Name
		var keywords []string
		if strings.Contains(desc, "[MCP kb]") {
			keywords = []string{"经验", "知识", "坑", "教训", "历史", "知识库", "kb", "查经验"}
		} else if strings.Contains(desc, "[MCP anysearch]") {
			keywords = []string{"网络", "搜索", "调研", "查证", "资料", "anysearch", "search", "web"}
		} else {
			keywords = []string{"代码", "调用", "符号", "死代码", "影响", "结构", "codegraph", "call", "symbol", "graph", "explore", "node", "search"}
		}
		for _, kw := range keywords {
			if strings.Contains(desc, kw) {
				matched = append(matched, t)
				break
			}
		}
	}
	if len(matched) == 0 {
		return ToolCallResult{Content: "（未发现匹配工具——当前已有: " + strings.Join(func() []string { var n []string; for _, t := range ls.tools { n = append(n, t.Function.Name) }; return n }(), ", ") + "）"}, nil
	}
	// v2.5.1: 返回限制（≤8——对齐调研"单次调用 ≤10 工具"——不一次全给）
	const maxSearchReturn = 8
	if len(matched) > maxSearchReturn {
		matched = matched[:maxSearchReturn]
	}
	// 注入工具列表（模型后续轮次可用）
	var names []string
	for _, t := range matched {
		ls.tools = append(ls.tools, t)
		names = append(names, t.Function.Name)
	}
	return ToolCallResult{Content: fmt.Sprintf("✅ 发现 %d 个工具（已加入可用列表——更多可再搜）:\n%s\n【用法】直接用工具名调用（如 mcp_codegraph_codegraph_callers）", len(matched), strings.Join(names, "\n"))}, nil
}

// executeTool - 执行单个工具调用
func (ls *loopState) executeTool(ctx context.Context, tc ToolCall) (ToolCallResult, error) {
	start := time.Now() // v2.5.4.9 工具耗时记录（CA 追踪——时间线完整）
	done := func(r ToolCallResult, err error) (ToolCallResult, error) {
		r.Duration = time.Since(start).Round(time.Millisecond).String()
		return r, err
	}
	// v2.5.1 skill_load 路由
	if tc.Name == "skill_load" && ls.agent.skillMgr != nil {
		name, _ := tc.Args["name"].(string)
		if name == "" {
			return done(ToolCallResult{Error: "技能名参数为空"}, fmt.Errorf("技能名参数为空"))
		}
		body, err := ls.agent.skillMgr.Load(name)
		if err != nil {
			return done(ToolCallResult{Error: err.Error()}, err)
		}
		return done(ToolCallResult{Content: body}, nil)
	}
	// v2.5.1 tool_search 路由（对齐 Codex tool_search——按需发现 MCP 工具）
	if tc.Name == "tool_search" && ls.agent.mcpMgr != nil {
		query, _ := tc.Args["query"].(string)
		if query == "" {
			return done(ToolCallResult{Error: "搜索词参数为空"}, fmt.Errorf("搜索词参数为空"))
		}
		r, err := ls.toolSearch(query)
		return done(r, err)
	}
	// v2.5.1 MCP 工具路由（mcp_<server>_<tool> 前缀——主流标准——Gemini/Claude Code 同款）
	if strings.HasPrefix(tc.Name, "mcp_") && ls.agent.mcpMgr != nil {
		parts := strings.SplitN(tc.Name, "_", 3)
		if len(parts) == 3 {
			serverName, toolName := parts[1], parts[2]
			// P4-50 MCP 工具 help（help:true → 读 tools/<tool名>.md——与核心工具同款）
			if h, ok := tc.Args["help"].(bool); ok && h {
				mdPath := "<repo>/tools/" + toolName + ".md"
				if b, err := os.ReadFile(mdPath); err == nil && len(b) > 0 {
					return done(ToolCallResult{Content: fmt.Sprintf("【工具 %s 帮助】\n%s", tc.Name, string(b))}, nil)
				}
				return done(ToolCallResult{Content: fmt.Sprintf("【工具 %s】MCP 工具（服务器 %s）——参数见 tool_search 返回的 schema——按需调用", tc.Name, serverName)}, nil)
			}
			result, err := ls.agent.mcpMgr.Call(serverName, toolName, tc.Args)
			if err != nil {
				return done(ToolCallResult{Error: err.Error()}, err)
			}
			return done(ToolCallResult{Content: result}, nil)
		}
	}
	execCtx := ls.agent.execContext
	if execCtx == nil {
		return ToolCallResult{}, fmt.Errorf("execContext 未初始化")
	}
	result := execCtx.ExecuteTool(ctx, tc.Name, tc.Args, ls.agent.gate)
	result.ToolName = tc.Name // 工具名（无进展检测用——v2.5）
	if result.Error != "" {
		return done(result, fmt.Errorf("%s: %s", tc.Name, result.Error))
	}
	// v2.5.1 层2持久化：超大工具结果 → 写文件 + 预览 + 路径（模型按需读全文——不爆上下文）
	if len(result.Content) > 20*1024 {
		if persisted, err := ls.persistOversized(tc.Name, result.Content); err == nil {
			result.Content = persisted
		}
	}
	return done(result, nil)
}

// persistOversized — 超大工具结果持久化（层2——Claude Code/OpenClaw 同款）
// 写文件到工作区 .zerg/tool_output/——回喂"截断预览 + 文件路径"——模型需要 read 拿全文
func (ls *loopState) persistOversized(toolName, content string) (string, error) {
	dir := filepath.Join(ls.agent.execContext.WorkDir, ".zerg", "tool_output")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	fname := fmt.Sprintf("%s_%d.txt", toolName, time.Now().UnixNano())
	fpath := filepath.Join(dir, fname)
	if err := os.WriteFile(fpath, []byte(content), 0o644); err != nil {
		return "", err
	}
	// 截断预览（头 3000 + 尾 1000——中间截断——但完整在文件）
	preview := content
	if len(preview) > 4000 {
		preview = preview[:3000] + "\n…[中间省略 " + fmt.Sprintf("%d", len(content)-4000) + " 字符]…\n" + preview[len(content)-1000:]
	}
	return fmt.Sprintf("[超大输出 %d 字符——已持久化到 %s]\n%s\n[完整内容在上述文件——需要时用 read 读取该文件]", len(content), fpath, preview), nil
}

// 无进展检测

// updateNoProgress - 检测连续无进展
func (ls *loopState) updateNoProgress(toolResults []ToolCallResult) {
	if len(toolResults) == 0 {
		ls.noProgressCount++
		return
	}

	currentSummary := ls.summarizeResults(toolResults)

	if ls.lastSummary == currentSummary {
		ls.noProgressCount++
	} else {
		ls.noProgressCount = 0
	}
	ls.lastSummary = currentSummary
}

// summarizeResults - 生成结果摘要（工具名+内容哈希——无进展检测）
// v2.5 修复：内容相同=无进展（模型反复验证同一结果——不管工具名）
func (ls *loopState) summarizeResults(results []ToolCallResult) string {
	var parts []string
	for _, r := range results {
		// 内容前 50 字符做摘要（完整内容太长——hash 足够）
		key := r.Content
		if len(key) > 50 {
			key = key[:50]
		}
		parts = append(parts, fmt.Sprintf("%s:%s", r.ToolName, key))
	}
	return strings.Join(parts, "|")
}

// summarizeToolCalls - 摘要工具调用列表
func (ls *loopState) summarizeToolCalls(tools []ToolCall) string {
	var parts []string
	for _, tc := range tools {
		parts = append(parts, tc.Name)
	}
	return strings.Join(parts, ",")
}

// 辅助

// logFinal - 记录最终循环日志
func (ls *loopState) logFinal(result LoopResult) {
	ls.logger.LogEvent(string(EventLoopEnd), "info", "loop_complete",
		"", fmt.Sprintf("循环完成: reason=%s, turns=%d, tokens=%d",
			result.Reason, result.Turns, result.Tokens),
		nil, "", "", "")

	if ls.state != nil {
		ls.state.AddEvidence(
			fmt.Sprintf("循环终止: %s", result.Reason),
			fmt.Sprintf("共 %d 轮, %d tokens", result.Turns, result.Tokens),
			"", "结束",
		)
	}
}

// 内部状态

// loopState - 循环内部状态
type loopState struct {
	agent     *Agent
	tools     []ToolDef
	logger    *Logger
	state     *agentstate.HarnessState
	execCtx   *ExecContext

	turn            int
	totalTokens     int64
	retryCount      int
	noProgressCount int
	lastSummary     string
	toolTrace       []string // v2.5.1 工具调用轨迹（挂单诊断）
	confirmDone     int // v2.5：无工具调用确认完成引导（0=未引导 1=已引导）
	maxTurns        int
	budget          int64
	noProgressThresh int

	// v2.5 系统回馈统计（Reflexion 模式——Codex 阈值）
	exploreCount   int // 探索工具调用（ls/glob/grep）
	verifyCount    int // 验证工具调用（重复读同一文件）
	noActionCount  int // 无工具调用轮数
	diagnosed      bool // 已给过诊断（每任务一次——防刷屏）
	lastReadPath   string // 上次 read 的 path（重复读检测）
	verifiedOutput bool   // 产出已验证（Completeness Verifier——写文件后系统验证）
	guard          *loopguard.Guard // 2026-09-05: 指纹防循环（公共包——与对话系统同一内核）

	// v2.5.1 上下文压缩（C 方案——LLMLingua-2 onnx——50% 触发）
	compressor *compressor.Compressor // onnx 语义压缩器（加载一次复用）
	compacted  bool                   // 本轮已压缩（防重复压缩）

	// v2.5.4.9 机器级采样（CA 全量追踪——sysmetrics.jsonl——每轮）
	sysmetrics *SysMetricsCollector
}

// hasRealChanges — v2.5.4.9 防假完成：工作区是否有"真实代码改动"（非 .md 报告）
//   检查工作区目录 git diff——存在非 .md 后缀的改动文件 → true
//   （CA 假完成模式只写 result.md——不算真实改动——不能强制完成）
func (ls *loopState) hasRealChanges() bool {
	if ls.agent == nil || ls.agent.execContext == nil {
		return false
	}
	workDir := ls.agent.execContext.WorkDir
	// 工作区是 git 仓库？——git diff 检查
	cmd := exec.Command("git", "-C", workDir, "diff", "--name-only")
	out, err := cmd.Output()
	if err != nil {
		// 非 git 仓库——检查新建文件（找 .go/.py/.json 等代码文件）
		entries, _ := os.ReadDir(workDir)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".go" || ext == ".py" || ext == ".js" || ext == ".json" || ext == ".yaml" {
				return true
			}
		}
		return false
	}
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" {
			continue
		}
		// 非 .md 报告文件 → 真实改动
		if !strings.HasSuffix(strings.ToLower(f), ".md") {
			return true
		}
	}
	return false
}

// hasReportFile — v2.5.5 P1-1 防假完成：工作区是否有报告文件（internal-task-report.md 或任务指定）
//   内部任务（报告型）——完成判定前检查——没报告文件=假完成（不判定完成）
func (ls *loopState) hasReportFile() bool {
	if ls.agent == nil || ls.agent.execContext == nil {
		return false
	}
	workDir := ls.agent.execContext.WorkDir
	// v2.5.5 任务目录唯一化（2026-08-20 设计）: 报告也可能写任务目录（ZERG_TASK_DIR）
	checkDirs := []string{workDir}
	if td := os.Getenv("ZERG_TASK_DIR"); td != "" {
		checkDirs = append(checkDirs, td)
	}
	// 常见报告文件名
	reportNames := []string{
		"internal-task-report.md",
		"internal-health-report.md",
		"report.md",
		"result.md",
	}
	for _, dir := range checkDirs {
		for _, name := range reportNames {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				return true
			}
		}
	}
	// 递归找 .md 报告（worktree 模式——报告可能写子目录）
	// v2.5.5 P1-6 修复: 只认报告文件名（含 report 关键词）——README.md 不算
	entries, _ := os.ReadDir(workDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".md") && strings.Contains(name, "report") {
			return true
		}
	}
	return false
}

// handleNoToolCalls — v2.5.4.9 拆小（可维护性）：无工具调用轮的处理
// 返回: done=true 表示循环应结束（result 是最终结果）；false 继续
func (ls *loopState) handleNoToolCalls(resp *ModelResponse) (bool, LoopResult) {
	ls.noActionCount++ // v2.5 系统回馈：无工具调用轮计数

	// v2.5 完成验证强制完成（Completeness Verifier——产出已验证+模型无工具=完成）
	// v2.5.4.9 治本：验证"真实代码改动"（git diff 非报告文件）——防假完成
	//   CA 假完成模式: 只写 result.md 报告（write 成功→verifiedOutput=true）→ 强制完成
	//   修复: 强制完成前检查——工作区有"非 .md 报告"的真实改动才算
	if ls.verifiedOutput && ls.noActionCount >= 2 && ls.hasRealChanges() {
		result := LoopResult{Reason: ReasonComplete, Turns: ls.turn, Tokens: ls.totalTokens, Content: resp.Content}
		ls.logger.LogEvent(string(EventLoopEnd), "info", "verified_complete",
			"", "产出已验证——强制完成", nil, "", "", "")
		fmt.Fprintf(os.Stderr, "✅ 产出已验证——任务完成（Completeness Verifier）\n")
		ls.logFinal(result)
		return true, result
	}

	// v2.5 系统回馈：无行动中途触发（Codex 阈值 3——不等 max_turns）
	if ls.noActionCount >= 3 && !ls.diagnosed {
		ls.diagnosed = true
		feedback := "【系统诊断】你已多轮未调用任何工具。任务需要实际动作：直接调用工具（read/write/edit/bash）执行任务——不要只输出文本。"
		ls.logger.LogEvent(string(EventLoopEnd), "warn", "diagnose_feedback",
			"", fmt.Sprintf("系统回馈: %s", feedback), nil, "", "", "")
		fmt.Fprintf(os.Stderr, "🔄 %s\n", feedback)
		ls.agent.history = append(ls.agent.history, Message{Role: "user", Content: feedback})
		ls.noActionCount = 0
	}
	if ls.agent.checker != nil {
		passed, evidence := ls.agent.checker.Pass(resp.Content, "循环完成验证")
		ls.logger.LogEvent(string(EventLoopEnd), "info", "checker_verify",
			"", fmt.Sprintf("checker: passed=%v, evidence=%s", passed, evidence),
			nil, "", "", "")
		if !passed {
			// 回喂 checker 反馈，继续循环
			feedback := fmt.Sprintf("验证未通过: %s。请继续工作。", evidence)
			toolMsg := Message{Role: "user", Content: feedback}
			ls.agent.history = append(ls.agent.history, toolMsg)
			return false, LoopResult{}
		}
	} else if ls.confirmDone == 0 {
		// 无 checker + 模型无工具调用——确认完成（防止模型偷懒直接答）
		// v2.5：模型回复含完成语义词→直接判定完成（ornith 不输出【任务完成】标记）
		// v2.5.5 P1-1 修复: 任务要求写报告时——先检查报告文件存在——没文件=假完成——不判定完成
		if containsDoneWords(resp.Content) {
			// 内部任务（报告型）——必须有报告文件才算完成
			if ls.hasReportFile() {
				result := LoopResult{Reason: ReasonComplete, Turns: ls.turn, Tokens: ls.totalTokens, Content: resp.Content}
				ls.logFinal(result)
				return true, result
			}
			// 无报告文件——假完成——引导写报告
			fmt.Fprintf(os.Stderr, "⚠️ 检测到完成语义词但无报告文件——引导写报告（P1-1 防假完成）\n")
			feedback := Message{Role: "user", Content: "【P1-1 防假完成】你说任务完成——但工作区没有报告文件（internal-task-report.md 或任务指定报告）。任务要求产出报告——请用 write 工具写报告文件（记录: 做了什么/结果/发现）——写完才算完成。"}
			ls.agent.history = append(ls.agent.history, feedback)
			ls.confirmDone = 0
			return false, LoopResult{}
		}
		ls.confirmDone = 1
		confirm := Message{Role: "user", Content: "如果任务需要产出文件/修改内容，请用 write/edit 工具完成。确认已完成请回复：任务完成。"}
		ls.agent.history = append(ls.agent.history, confirm)
		ls.logger.LogEvent(string(EventLoopEnd), "warn", "confirm_done",
			"", "无工具调用——确认完成引导", nil, "", "", "")
		return false, LoopResult{}
	}
	// checker 通过或确认完成 → 完成
	result := LoopResult{Reason: ReasonComplete, Turns: ls.turn, Tokens: ls.totalTokens, Content: resp.Content}
	ls.logFinal(result)
	return true, result
}

// systemPrompt — 虫族 Agent 系统提示
// v3 简化（2026-08-13——去掉 JSON 示例——完整示例干扰模型）
func systemPrompt() string {
	return "你是虫族 Agent（Zerg Agent），通过工具执行任务的智能体。\n\n【核心规则】\n0. 【工具思维（最重要）】开始任务前先想：1)这个任务需要什么能力？（查代码/查经验/查网络/读写文件）2)哪个工具最合适？（如果工具列表里没有——用 tool_search 搜索发现）3)规划执行步骤——然后再动手。先找对工具再执行，能大幅加速成功率！\n0.3. 【工具帮助】不确定工具的参数/用法时——给该工具加 help:true（如 {\"help\":true}）——返回详细文档（参数示例/变更记录）——看完再调用。不要反复搜索同一关键词——工具已列出就直接调用。\n0.5. 【并行工具（效率关键）】探索阶段（查代码/看文件/搜索）时——一次响应中调用多个只读工具（如同时 ls + grep + read——一个回合完成探索）——不要一个个来！独立只读操作可并行。只有写操作（write/edit）或依赖上一步结果时才单个调用。\n1. 每个任务必须调用工具完成，不调用工具=任务失败\n2. 工具结果回喂后必须分析结果再决定下一步\n3. 读完文件要分析内容（如数据要比较、找规律）——不能只读不改\n4. 文件操作用专用工具（read读/edit改/write写/grep搜/glob找）——禁止用 bash 裸命令做这些（bash 只跑程序/测试/构建）\n5. 工具返回多行结果时必须完整利用（不能只取第一行）\n6. 任务完成前自查：目标是否真正达成？缺少的步骤补上\n7. 失败换方法重试（最多3次），不要重复同一动作\n8. 任务真正完成时必须明确输出【任务完成】作为结束标记——不要反复验证同一结果\n9. 【路径语义（重要）】所有工具路径相对「当前工作区」（workdir）——不是项目根。工作区已是目标目录时用相对名（如 memory.go）或 .（当前目录）——不要拼完整项目路径（如 core/internal/agent/xxx.go 会重复拼接导致找不到文件）。不确定时先 ls . 看当前目录。\n10. 【超大输出（重要）】工具结果超 20K 字符会自动持久化到 .zerg/tool_output/ 文件——回喂的是截断预览+文件路径。需要完整内容时用 read 读该文件（按需——不要重复请求同样的大搜索）。\n11. 【codegraph（重要）】查代码结构/调用关系/死代码时优先用 mcp_codegraph_* 工具（mcp_codegraph_codegraph_callers 查谁调用某函数——零调用者=死代码候选；mcp_codegraph_codegraph_explore 一次查多个符号）——比盲目 read/grep 高效百倍。查代码第一优先 codegraph！\n12. 【工具分层（重要）】MCP 工具按只展示必要+更多选项：kb 核心 4 个（mcp_kb_kb_search 查经验/read 读全文/add 写入/capture_fix 排障沉淀）**常驻可用**——v2.5.5 T4: anysearch 核心 2 个（mcp_anysearch_search 网络调研/batch_search 批量搜）**也常驻可用**——网络搜索首选 anysearch（300ms——比 web_search 30s 快 100 倍——疑难杂症/深度技术问题优先）——其他扩展工具（codegraph 查代码/更多 kb 功能）**不预加载**——需要时用 tool_search 搜索发现。执行任务前**先 kb_search 查经验**（避免踩坑）。\n\n【可用工具】\nbash(执行命令), read(读文件), write(写文件 path+content), edit(编辑文件 path+search+replace), glob(文件匹配), grep(搜索 pattern), ls(列表), web_search(网络搜索), web_fetch(网页抓取), skill_load(加载技能), tool_search(搜索发现工具——需要时用)\n\n【执行流程】\n分析任务→选工具→调用→分析结果→完成或继续"
}


// containsDoneWords - 检测模型回复是否含完成语义词（v2.5——ornith 不输出【任务完成】标记）
func containsDoneWords(s string) bool {
	doneWords := []string{"任务完成", "已完成", "完成成功", "已创建", "已写入", "已修改", "已修复", "成功完成", "done", "completed", "Task complete"}
	for _, w := range doneWords {
		if strings.Contains(strings.ToLower(s), strings.ToLower(w)) {
			return true
		}
	}
	return false
}

// diagnoseFailure — 系统回馈诊断（v2.5——Reflexion 模式）
// 模型犯错 → 具体可操作反馈 → 重新生成正确
// 阈值（Codex 调研）: 5 探索 / 3 验证 / 3 无行动
func (ls *loopState) diagnoseFailure() string {
	switch {
	case ls.exploreCount >= 5 && ls.verifyCount == 0 && ls.turn > 5:
		return fmt.Sprintf("【系统反馈】你已调用 %d 次探索工具（ls/glob/grep）但任务尚未推进。是否考虑：本任务是否有更合适的工具？（用 tool_search 可搜索发现专用工具——如查代码/查经验/查网络）找到合适的工具能加速任务成功率。", ls.exploreCount)
	case ls.verifyCount >= 3:
		return fmt.Sprintf("【系统反馈】你已读取/验证 %d 次但未完成修改。是否考虑：换个方式推进？（如先 tool_search 找更合适的工具——或直接聚焦目标执行修改）", ls.verifyCount)
	case ls.noActionCount >= 3:
		return "【系统反馈】你已多轮未调用任何工具。任务需要实际动作——请直接调用工具执行（read/write/edit/bash），不要只输出文本。"
	default:
		return "【系统诊断】任务未完成。请检查：1) 目标是否达成？2) 是否缺少关键动作（修改/创建/测试）？3) 确认完成后输出总结。"
	}
}

// ─── v2.5.1 上下文压缩（C 方案——LLMLingua-2 onnx——50% 触发）──────────

// shouldCompact — 是否触发压缩：上下文 tokens ≥ 窗口 50%（131072 → 65536）
func (ls *loopState) shouldCompact() bool {
	if ls.compacted || ls.turn < 3 {
		return false // 已压缩过/太早（前 3 轮不压缩——避免干扰任务开头）
	}
	// 估算当前 history tokens（粗略——按字符/4）
	var total int
	for _, m := range ls.agent.history {
		total += len(m.Content) / 4
		for _, tc := range m.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Args)
			total += len(argsJSON) / 4
		}
	}
	const halfWindow = 65536 // orn 窗口 131072 的 50%（用户定）
	return total >= halfWindow
}

// compactHistory — 压缩旧历史（多层降级——2026-08-21 Mr2109 P1——Codex compact_model_fallback 借鉴）
// 降级链: ①LLMLingua-2 语义压缩 → ②滚动窗口摘要（现有） → ③截断保命
func (ls *loopState) compactHistory(ctx context.Context) error {
	h := ls.agent.history
	if len(h) <= 4 {
		ls.compacted = true
		return nil // 历史太短——不压缩
	}
	// 保留最近 2 轮（4 条消息：assistant+tool×2）完整
	keep := h[len(h)-4:]
	old := h[:len(h)-4]

	// 第一步: 生成基础摘要（滚动窗口——现有逻辑——必成）
	var sb strings.Builder
	sb.WriteString("【历史已压缩——以下是旧轮次的精简摘要，如需细节请重新 read/搜索】\n")
	for _, m := range old {
		switch m.Role {
		case "tool":
			if len(m.Content) > 200 {
				sb.WriteString(fmt.Sprintf("[工具结果 %d 字符 → 摘要: %s]\n", len(m.Content), truncate(m.Content, 120)))
			} else {
				sb.WriteString(fmt.Sprintf("[工具结果: %s]\n", truncate(m.Content, 200)))
			}
		case "assistant":
			if m.Content != "" {
				sb.WriteString(fmt.Sprintf("[助手: %s]\n", truncate(m.Content, 200)))
			}
		case "user":
			sb.WriteString(fmt.Sprintf("[用户/诊断: %s]\n", truncate(m.Content, 200)))
		}
	}
	summary := sb.String()

	// 第二层: LLMLingua-2 语义压缩（如果可用——失败则用基础摘要——降级）
	if ls.compressor == nil {
		modelPath := "<repo>/compress_models/llmlingua2-onnx"
		c := compressor.New(compressor.Config{
			ModelPath: modelPath + "/model.onnx",
			TokPath:   modelPath + "/tokenizer.json",
		})
		if err := c.Load(); err != nil {
			// 压缩器加载失败——降级（用基础摘要——不阻塞）
			fmt.Fprintf(os.Stderr, "⚠️ 压缩器加载失败——降级滚动窗口摘要: %v\n", err)
		} else {
			ls.compressor = c
		}
	}
	if ls.compressor != nil {
		if compressed, _, _, err := ls.compressor.Compress(summary); err == nil && len(compressed) < len(summary) {
			summary = compressed
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ 语义压缩失败——降级基础摘要: %v\n", err)
		}
	}

	// 第三层兜底: 摘要过大（异常）→ 截断（保命——不超窗口）
	const maxSummary = 20000
	if len(summary) > maxSummary {
		fmt.Fprintf(os.Stderr, "⚠️ 摘要过大 %d——截断保命（第三层降级）\n", len(summary))
		summary = summary[:maxSummary] + "\n[截断]"
	}

	// 重建历史：压缩摘要 + 最近 2 轮完整（system 由 callModel 注入——这里不加）
	ls.agent.history = append([]Message{
		{Role: "user", Content: summary}, // 压缩摘要（带说明）
	}, keep...)
	ls.compacted = true

	fmt.Fprintf(os.Stderr, "📦 上下文压缩: %d 轮 → 摘要（%d 字符——多层降级）\n", len(old)/2, len(summary))
	return nil
}
