package main

// zerg-agent - 虫族 Agent CLI 入口
// 用法: go run ./cmd/zerg-agent -task "写一个 Go HTTP 服务器"
//
// 流程: 参数解析 → 创建 Agent → 建日志 → 调 Loop（7 工具）→ 输出结果
import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/agentstate"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
)

func filterTools(all []agent.ToolDef, names string) []agent.ToolDef {
	want := map[string]bool{}
	for _, n := range strings.Split(names, ",") {
		want[strings.TrimSpace(n)] = true
	}
	var out []agent.ToolDef
	for _, t := range all {
		if want[t.Function.Name] {
			out = append(out, t)
		}
	}
	return out
}

// mustToken 启动期校验共享令牌（2026-09-11 A 批：库内零明文）。
// 解析顺序：环境变量 ZERG_AUTH_TOKEN / ZERG_API_TOKEN → ~/.zerg/token 文件。
// 拿不到即拒绝启动——空令牌会让所有主控调用 403，而日志看不出异常。
func mustToken() string {
	t := config.ResolveAuthToken()
	if t == "" {
		log.Fatalf("❌ no shared token configured: set ZERG_AUTH_TOKEN or write it to %s (see .env.example)", config.TokenFilePath())
	}
	return t
}

func main() {
	startTime := time.Now() // 开始时间（JSON 信封 metrics——v2.5）

	// 参数解析
	var (
		task        string
		model       string
		workdir     string
		maxTurns    int
		toolsFlag   string
		mcpFlag     string
		autoIssue   bool
		issueFile   string
		gateway     string
		x3Status    string // v2.5.4.9 机器级采样——X3 状态地址
		x3Token     string // v2.5.4.9 机器级采样——X3 token
		jsonOut     bool
		subtaskMode bool
		retryN      int
		stateFile   string
	)

	flag.Bool("version", false, "打印代码身份（机器可读）后退出")
	flag.StringVar(&task, "task", "", "任务描述（必填）")
	flag.StringVar(&model, "model", "example-35b", "模型名（默认 example-35b）")
	flag.StringVar(&workdir, "workdir", "/tmp/zerg-agent", "工作区目录（默认 /tmp/zerg-agent）")
	flag.IntVar(&maxTurns, "max-turns", 100, "最大循环轮数（默认 100——安全兜底；自主停止主导——v2.5）")
	flag.StringVar(&toolsFlag, "tools", "", "工具子集（逗号分隔——如 bash,write）")
	flag.StringVar(&mcpFlag, "mcp", "", "MCP 服务器（name:cmd|arg 逗号分隔——如 codegraph:codegraph|serve|--mcp；HTTP 型 name:http|url|token）")
	flag.BoolVar(&autoIssue, "auto-issue", false, "失败自动挂单（错误自愈 P0——重试耗尽写 docs/issues/）")
	flag.BoolVar(&jsonOut, "json", false, "JSON 输出（AI 解析用）")
	flag.BoolVar(&subtaskMode, "subtask", false, "子任务结晶模式（2026-09-05——拆解轮+阶段执行+结晶链）")
	flag.IntVar(&retryN, "retry", 1, "失败自动重试次数（默认 1）")
	flag.StringVar(&stateFile, "state", "", "状态文件路径（断连恢复用——存在则续跑）")
	flag.StringVar(&issueFile, "issue", "", "接单模式：issue 文件路径（读问题单→查因→填修复结论→标记 resolved）")
	flag.StringVar(&gateway, "gateway", statepath.GatewayBaseURL(), "网关地址（容器内用 host.docker.internal:8082——v2.5.3）")
	flag.StringVar(&x3Status, "x3-status", "", "X3 agent 状态地址（机器级采样用——如 http://<worker-ip>:8100/status）")
	flag.StringVar(&x3Token, "x3-token", "", "X3 认证 token（机器级采样用）")
	flag.Parse()

	sh := flag.CommandLine.Lookup("version")
	if sh != nil && sh.Value.String() == "true" {
		fmt.Println(version.Line("zerg-agent"))
		return
	}

	// v2.5.4.9 机器级采样：环境变量兜底（loop 里 NewSysMetricsCollector 读环境）
	if os.Getenv("ZERG_X3_STATUS") == "" && x3Status != "" {
		os.Setenv("ZERG_X3_STATUS", x3Status)
	}
	if os.Getenv("ZERG_X3_TOKEN") == "" && x3Token != "" {
		os.Setenv("ZERG_X3_TOKEN", x3Token)
	}

	// 校验
	if task == "" && issueFile == "" {
		fmt.Fprintln(os.Stderr, "错误: -task 或 -issue 参数必填")
		fmt.Fprintln(os.Stderr, "用法: zerg-agent -task \"任务描述\"  或  zerg-agent -issue \"问题单路径\"（接单模式）")
		os.Exit(1)
	}

	// 接单模式（-issue）：读问题单 → 生成查因任务 → 完成后回填结论
	if issueFile != "" && task == "" {
		issueContent, err := os.ReadFile(issueFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 读 issue 失败: %v\n", err)
			os.Exit(1)
		}
		// 标记为处理中
		issueText := string(issueContent)
		issueText = agent.MarkIssueStatus(issueText, "fixing")
		_ = os.WriteFile(issueFile, []byte(issueText), 0o644)
		task = "接单修复问题单 " + issueFile + "。内容:\n" + issueText + "\n\n任务: 1.查根因（用 codegraph 查代码/kb 查经验）2.能修则修复+测试验证 3.将修复结论（根因/修复/验证）写回该 issue 文件（替换'根因: '等留空处）4.标记状态为 resolved。不能修（环境/设计问题）则写结论并标记 escalated。"
		if !jsonOut {
			fmt.Printf("📋 Task accepted: %s (diagnose → fix → report)\n", issueFile)
		}
	}

	// 创建 Agent
	a := agent.NewAgent(agent.Config{
		GatewayURL: gateway,
		AuthToken:  mustToken(),
		Model:      model,
		MaxTurns:   maxTurns,
		WorkDir:    workdir,
	})
	if !jsonOut {
		fmt.Printf("✅ Agent created (model: %s)\n", model)
	}

	// 建日志
	taskID := time.Now().Format("2006-01-02T15-04-05")
	// v2.5.5 P1-2 修复: 日志目录支持 ZERG_LOG_DIR 覆盖（总调度器 spawn 时设置——写固定位置防丢失）
	// 根因: 默认写 workdir/.zerg/logs——worktree 模式 merge 删 worktree——日志丢（证据链断）
	logDir := filepath.Join(workdir, ".zerg", "logs", taskID)
	if envLogDir := os.Getenv("ZERG_LOG_DIR"); envLogDir != "" {
		logDir = filepath.Join(envLogDir, taskID)
	}
	logger, err := agent.NewLogger(taskID, logDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 日志创建失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()
	// v2.5.5 修复（2026-08-21 Mr2109发现——每轮时间不对）: 写 audit.jsonl 到日志目录——含任务 ID
	// 主控 findTaskLogDir 靠它精确关联任务↔日志（提交时间≠执行时间——目录名是执行时间）
	if ztd := os.Getenv("ZERG_TASK_DIR"); ztd != "" {
		audit := fmt.Sprintf("{\"task_id\":\"%s\",\"started\":\"%s\"}\n", ztd, time.Now().Format(time.RFC3339))
		os.WriteFile(filepath.Join(logDir, "audit.jsonl"), []byte(audit), 0o644)
	}
	// v2.5.5 P1: 会话日志（日志=上下文真相——2026-08-21 Mr2109）
	// 注入 Agent——消息写会话日志——崩溃可从日志恢复上下文
	if sl, err := agent.NewSessionLog(logDir); err == nil {
		a.SetSessionLog(sl)
		defer sl.Close()
		// 恢复历史（日志=权威——续跑/崩溃恢复）
		if n := a.RestoreFromLog(); n > 0 {
			if !jsonOut {
				fmt.Printf("♻️ Restored history from the session log: %d messages\n", n)
			}
		}
	}
	// v2.5.4.9 结构化日志接通：注入 agent（callModel 事件写入）
	a.SetLogger(logger)
	if !jsonOut {
		fmt.Printf("✅ Log dir: %s\n", logDir)
	}

	// 准备状态
	// 状态（断连恢复——v2.5 #9）
	var state *agentstate.HarnessState
	if stateFile != "" {
		if loaded, err := agentstate.Load(stateFile); err == nil && loaded != nil {
			state = loaded
			done := 0
			for _, t := range loaded.Todos {
				if t.Status == "done" {
					done++
				}
			}
			if !jsonOut {
				fmt.Printf("🔄 Restored state: todo %d/%d done (resuming)\n", done, len(loaded.Todos))
			}
		}
	}
	if state == nil {
		state = agentstate.NewState(task, "zerg-agent", nil)
	}
	a.SetState(state)
	mcpMgr := agent.NewMCPManager() // v2.5.1 MCP 管理器（codegraph 等——动态工具）
	defer mcpMgr.Close()
	a.SetMCPManager(mcpMgr) // 注入（executeTool 路由用）
	// v2.5.1 skill 管理器（SKILL.md 技能——渐进式加载）
	skillMgr := agent.NewSkillManager("<repo>/core/internal/agent/skills")
	a.SetSkillManager(skillMgr)

	// 注入任务到 history（关键——模型必须看到任务指令）
	a.AppendMessage(agent.Message{Role: "user", Content: task})

	// 准备工具（7 个默认工具 + MCP 动态）
	tools := agent.AllTools()

	// v2.5.1 MCP：连接服务器（-mcp "codegraph:codegraph|serve|--mcp|-p|/路径"——name:cmd|arg|arg 格式）
	if mcpFlag != "" {
		for _, spec := range strings.Split(mcpFlag, ",") {
			parts := strings.SplitN(spec, ":", 2)
			if len(parts) != 2 {
				continue
			}
			name, cmdSpec := parts[0], parts[1]
			// HTTP 型（v2.5.1——anysearch 等远程 MCP）: name:http|url|token
			if strings.HasPrefix(cmdSpec, "http|") {
				hp := strings.Split(cmdSpec, "|")
				if len(hp) >= 2 {
					headers := map[string]string{}
					if len(hp) >= 3 && hp[2] != "" {
						headers["Authorization"] = "Bearer " + hp[2]
					}
					if err := mcpMgr.ConnectHTTP(name, hp[1], headers); err != nil {
						fmt.Fprintf(os.Stderr, "⚠️ MCP HTTP 连接失败 %s: %v\n", name, err)
						continue
					}
					if !jsonOut {
						fmt.Printf("✅ MCP %s connected (HTTP): %d tools (deferred — discovered via tool_search)\n", name, len(mcpMgr.ToolDefs(true)))
					}
				}
				continue
			}
			// stdio 型: name:cmd|arg|arg（| 分隔参数——路径含空格安全）
			cmdParts := strings.Split(cmdSpec, "|")
			command := cmdParts[0]
			args := cmdParts[1:]
			if err := mcpMgr.Connect(name, command, args...); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️ MCP 连接失败 %s: %v\n", name, err)
				continue
			}
			// v2.5.1: 工具分层（对齐"只展示必要+更多选项"）——kb 核心常驻——其他 deferred
			coreTools := mcpMgr.ToolDefs(false) // false=只核心（kb_search/read/add/capture_fix）
			tools = append(tools, coreTools...) // 核心加进初始列表（查经验是高优先行为）
			if !jsonOut {
				fmt.Printf("✅ MCP %s connected: %d core tools (resident) + deferred extensions (via tool_search)\n", name, len(coreTools))
			}
		}
	}
	// 工具子集（v2.5.1: MCP 合并后统一过滤——-tools 指定时全部工具生效——含 MCP）
	if toolsFlag != "" {
		tools = filterTools(tools, toolsFlag)
	}
	if !jsonOut {
		fmt.Printf("✅ Tools loaded: %d\n", len(tools))
	}

	// 启动循环
	if !jsonOut {
		fmt.Printf("\n🚀 Executing task: %s\n", task)
	}
	if !jsonOut {
		fmt.Println("─────────────────────────────────────")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听 Ctrl+C 中断
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		fmt.Println("\n⚠️  Interrupt signal received, terminating...")
		cancel()
	}()

	// 执行任务（失败自动重试——v2.5 治本：ornith 波动靠重试缓解）
	var result agent.LoopResult
	for attempt := 0; attempt <= retryN; attempt++ {
		if subtaskMode {
			// 2026-09-05 子任务结晶模式（设计-子任务结晶模式-20260905——S5 接线）
			result = agent.RunSubtaskLoop(ctx, a, tools, logger, state, maxTurns)
		} else {
			result = agent.Loop(ctx, a, tools, logger, state, maxTurns, 0, 3)
		}
		// 保存状态（断连恢复——v2.5 #9）
		if stateFile != "" && state != nil {
			_ = state.Save(stateFile)
		}
		if result.Reason == agent.ReasonComplete {
			break
		}
		if attempt < retryN {
			if !jsonOut {
				fmt.Printf("⚠️ Attempt %d incomplete (%s) — retrying...\n", attempt+1, result.Reason)
			}
			// 清历史重来（新实例）
			a = agent.NewAgent(agent.Config{
				GatewayURL: gateway,
				AuthToken:  mustToken(),
				Model:      model,
				MaxTurns:   maxTurns,
				WorkDir:    workdir,
			})
			a.AppendMessage(agent.Message{Role: "user", Content: task})
			state = agentstate.NewState(task, "zerg-agent", nil)
			a.SetState(state)
			a.SetMCPManager(mcpMgr) // v2.5.1 重试实例重新注入 MCP（否则 mcp 工具未知）
		}
	}

	// v2.5.1 错误自愈 P0：重试耗尽仍失败 → 挂单（-auto-issue）
	// 决策矩阵（简单版）: model_error（网关瞬时——重试已处理——无教训价值）不挂单——其他挂单
	if autoIssue && result.Reason != agent.ReasonComplete && result.Reason != agent.ReasonModelError {
		issuePath, err := agent.CreateIssue(workdir, task, result.Reason, retryN, result.ToolTrace)
		if err != nil {
			if !jsonOut {
				fmt.Fprintf(os.Stderr, "⚠️ 挂单失败: %v\n", err)
			}
		} else {
			if !jsonOut {
				fmt.Printf("📋 Failure filed: %s (entered the workflow — pending dispatch)\n", issuePath)
			}
		}
	}

	// v2.5.1 接单模式收尾：任务结束自动标记 issue 状态（不依赖 CA 自觉）
	if issueFile != "" {
		finalStatus := "escalated" // 默认升级（未完成）
		if result.Reason == agent.ReasonComplete {
			finalStatus = "resolved" // 完成 = 修复/结论已回填
		}
		if data, err := os.ReadFile(issueFile); err == nil {
			updated := agent.MarkIssueStatus(string(data), finalStatus)
			_ = os.WriteFile(issueFile, []byte(updated), 0o644)
			if !jsonOut {
				fmt.Printf("📋 Issue status: %s (%s)\n", finalStatus, result.Reason)
			}
		}
	}

	// 输出结果（JSON 模式——AI 解析验收——7 原则信封）
	if jsonOut {
		// 语义退出码（Agentic CLI 标准）: 0=成功 2=任务失败 3=模型错误 4=可重试
		exitCode := 0
		ok := result.Reason == agent.ReasonComplete
		if !ok {
			switch result.Reason {
			case agent.ReasonModelError:
				exitCode = 3
			case agent.ReasonMaxTurns, agent.ReasonBlocked, agent.ReasonTokenBudget:
				exitCode = 4 // 可重试（换提示/换模型）
			default:
				exitCode = 2
			}
		}
		out := map[string]any{
			"ok":             ok,
			"command":        "agent.run",
			"schema_version": "1.0",
			"request_id":     fmt.Sprintf("task_%d", time.Now().Unix()),
			"result": map[string]any{
				"status":  string(result.Reason),
				"turns":   result.Turns,
				"tokens":  result.Tokens,
				"summary": result.Content,
				"workdir": workdir,
				"logs":    logDir,
			},
			"errors":   []string{},
			"warnings": []string{},
			"metrics": map[string]any{
				"exit_code":   exitCode,
				"duration_ms": time.Since(startTime).Milliseconds(),
			},
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		os.Exit(exitCode)
	}

	// 输出结果
	fmt.Println("─────────────────────────────────────")
	fmt.Printf("\n🏁 Result:\n")
	fmt.Printf("   status: %s\n", formatReason(result.Reason))
	fmt.Printf("   turns:  %d\n", result.Turns)
	fmt.Printf("   Token:  %d\n", result.Tokens)
	if result.Content != "" {
		// 摘要：取前 200 字符
		summary := result.Content
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		fmt.Printf("   summary: %s\n", summary)
	}
	fmt.Printf("\n📂 Workspace: %s\n", workdir)
	fmt.Printf("📋 Log:   %s\n", logDir)
}

// formatReason - 将终止原因转为人类可读描述
func formatReason(reason agent.TerminateReason) string {
	switch string(reason) {
	case "complete":
		return "完成 (checker 通过)"
	case "user_abort":
		return "用户中止"
	case "token_budget":
		return "Token 预算耗尽"
	case "max_turns":
		return "达到最大轮数"
	case "model_error":
		return "模型调用失败"
	default:
		return fmt.Sprintf("terminated (%s)", string(reason))
	}
}
