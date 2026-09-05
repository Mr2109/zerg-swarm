package agent

// ca_loop.go — CA 循环换装 loopcore 内核（2026-09-05 内核第二步）
// Loop() 主循环替换为内核装配——终止判定=Terminator（契约优先——五层启发式退役为 fallback）
// 保留: 压缩/技能/sysmetrics/ToolTrace 补全（CA 域逻辑不动——只换循环骨架）

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"zerg/core/internal/agentstate"
	"zerg/core/internal/loopcore"
)

// CATerminator — CA 终止仲裁（内核 Terminator 实现）
// 判定序: ①done 词缺报告→引导写报告 ②多轮无行动→诊断引导 ③确认完成→verifiedOutput 强制完成
// （契约核验在外层 VerifyTaskOutput/Contract——此处只管循环内「模型说完成」的仲裁——与原 handleNoToolCalls 同语义）
type CATerminator struct {
	ls *loopState
}

func (t *CATerminator) OnNoToolCall(resp *loopcore.Response) (bool, string) {
	ls := t.ls
	ls.noActionCount++

	// ① 产出已验证 + 模型确认 → 强制完成（原 verifiedOutput 快速通道）
	if ls.verifiedOutput && ls.noActionCount >= 2 && ls.hasRealChanges() {
		return true, ""
	}
	// ② 无行动中途引导（3 轮）
	if ls.noActionCount >= 3 && !ls.diagnosed {
		ls.diagnosed = true
		feedback := "【系统诊断】你已多轮未调用任何工具。任务需要实际动作：直接调用工具（read/write/edit/bash）执行任务——不要只输出文本。"
		ls.noActionCount = 0
		return false, feedback
	}
	// ③ done 词 + 报告缺失 → 引导写报告（P1-1 防假完成）
	if ls.confirmDone == 0 && containsDoneWords(resp.Content) {
		if ls.hasReportFile() {
			return true, "" // 报告在——完成
		}
		ls.confirmDone = 0
		return false, "【P1-1 防假完成】你说任务完成——但工作区没有报告文件（internal-task-report.md 或任务指定报告）。任务要求产出报告——请用 write 工具写报告文件（记录: 做了什么/结果/发现）——写完才算完成。"
	}
	// ④ 首次无工具 → 确认完成引导
	if ls.confirmDone == 0 {
		ls.confirmDone = 1
		return false, "如果任务需要产出文件/修改内容，请用 write/edit 工具完成。确认已完成请回复：任务完成。"
	}
	// ⑤ checker 通过或已确认 → 完成
	if ls.agent.checker != nil {
		if passed, evidence := ls.agent.checker.Pass(resp.Content, "循环完成验证"); !passed {
			return false, fmt.Sprintf("验证未通过: %s。请继续工作。", evidence)
		}
	}
	return true, ""
}

// CAToolResult — 工具结果钩子（write/edit 成功→验证文件真实存在→verifiedOutput；探索/验证计数）
func caToolResult(ls *loopState) loopcore.OnToolResult {
	return func(tc loopcore.ToolCall, content string, execErr error) {
		switch tc.Name {
		case "ls", "glob", "grep":
			ls.exploreCount++
		case "read":
			path, _ := tc.Args["path"].(string)
			if path != "" && path == ls.lastReadPath {
				ls.verifyCount++
			}
			ls.lastReadPath = path
		case "write", "edit":
			// v2.5 完成验证（Completeness Verifier——确定性验证 > 提示引导）
			if path, _ := tc.Args["path"].(string); path != "" {
				p := path
				if !filepath.IsAbs(p) && ls.agent.execContext != nil {
					p = filepath.Join(ls.agent.execContext.WorkDir, path)
				}
				if _, err := os.Stat(p); err == nil {
					ls.verifiedOutput = true
				}
			}
		}
	}
}

// RunWithKernel — CA 任务用内核跑（Loop() 的内核版——渐进切换: 先并行存在——验证稳定后 Loop 转发此处）
// 返回 LoopResult 兼容现有出口（退出码语义/挂单/复查链不动）
func (a *Agent) RunWithKernel(ctx context.Context, tools []ToolDef, logger *Logger, state *agentstate.HarnessState, maxTurns int, budget int64, noProgressThresh int) (result LoopResult) {
	ls := newLoop(a, tools, logger, state, maxTurns, budget, noProgressThresh)

	logger.LogEvent(string(EventLoopStart), "info", "loop_start",
		"", fmt.Sprintf("循环启动(loopcore内核): maxTurns=%d, budget=%d", ls.maxTurns, ls.budget),
		nil, "", "", "")

	// 消息流转内核格式（a.history → msgs）
	conv := func() []map[string]any {
		msgs := make([]map[string]any, 0, len(a.history)+1)
		for _, m := range a.history {
			if m.Role == "tool" {
				msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": m.ToolCallID, "content": m.Content})
				continue
			}
			if len(m.ToolCalls) > 0 {
				tcs := make([]map[string]any, 0, len(m.ToolCalls))
				for _, tc := range m.ToolCalls {
					tcs = append(tcs, map[string]any{
						"id": tc.ID, "type": "function",
						"function": map[string]any{"name": tc.Name, "arguments": tc.RawArgs},
					})
				}
				msgs = append(msgs, map[string]any{"role": "assistant", "content": m.Content, "tool_calls": tcs})
				continue
			}
			msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
		}
		return msgs
	}

	sysPrompt := systemPrompt()
	if a.skillMgr != nil {
		if d := a.skillMgr.ListDescriptions(); d != "" {
			sysPrompt += "\n\n" + d
		}
	}

	terminator := &CATerminator{ls: ls}
	_ = conv
	kres := loopcoreRun(ctx, a, ls, sysPrompt, terminator, logger)

	// 内核结果 → LoopResult（兼容出口）
	reason := ReasonComplete
	switch kres.ExitKind {
	case "max_rounds":
		reason = ReasonMaxTurns
	case "wall_clock":
		reason = ReasonBlocked
	case "round_timeout", "stream_broken", "bad_format", "empty_args":
		reason = ReasonBlocked
	case "model_error":
		reason = ReasonModelError
	}
	if kres.Err != "" {
		reason = ReasonModelError
	}
	logger.LogEvent(string(EventLoopEnd), "info", "loop_complete",
		"", fmt.Sprintf("内核循环完成: reason=%s turns~%d tokens=%d", kres.ExitKind, len(kres.Traces), kres.Usage.TotalTokens),
		nil, "", "", "")
	return LoopResult{
		Reason:     reason,
		Turns:      ls.turn,
		Tokens:     kres.Usage.TotalTokens,
		Content:    kres.Content,
		ToolTrace:  ls.toolTrace,
	}
}


// loopcoreRun — loopcore.Run 的 agent 侧适配（Infer=a.callModel / Exec=ls.executeTool）
func loopcoreRun(ctx context.Context, a *Agent, ls *loopState, sysPrompt string,
	term *CATerminator, logger *Logger) *loopcore.Result {

	// 消息转换
	msgs := make([]map[string]any, 0, len(a.history)+1)
	for _, m := range a.history {
		if m.Role == "tool" {
			msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": m.ToolCallID, "content": m.Content})
			continue
		}
		if len(m.ToolCalls) > 0 {
			tcs := make([]map[string]any, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				tcs = append(tcs, map[string]any{
					"id": tc.ID, "type": "function",
					"function": map[string]any{"name": tc.Name, "arguments": tc.RawArgs},
				})
			}
			msgs = append(msgs, map[string]any{"role": "assistant", "content": m.Content, "tool_calls": tcs})
			continue
		}
		msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
	}

	inferFn := func(ctx context.Context, model, sysP string, m []map[string]any,
		onDelta func(string, string), tools []map[string]any) (*loopcore.Response, error) {
		ls.turn++
		resp, err := a.callModel(ctx, sysP, ls.tools)
		if err != nil {
			return nil, err
		}
		ls.totalTokens += resp.Usage.TotalTokens
		logger.LogEvent(string(EventModelCall), "info", "model_call_ok",
			"", "", map[string]any{"tokens": resp.Usage.TotalTokens}, "", "", "")
		kr := &loopcore.Response{
			Content: resp.Content, Reasoning: resp.Reasoning, Finish: resp.Finish,
			TotalTokens: resp.Usage.TotalTokens,
		}
		for _, tc := range resp.ToolCalls {
			kr.ToolCalls = append(kr.ToolCalls, loopcore.ToolCall{ID: tc.ID, Name: tc.Name, Args: tc.Args, RawArgs: tc.RawArgs})
		}
		return kr, nil
	}
	execFn := func(ctx context.Context, name string, args map[string]any) (string, string, error) {
		tc := ToolCall{ID: fmt.Sprintf("k%d", ls.turn), Name: name, Args: args}
		result, execErr := ls.executeTool(ctx, tc)
		ls.toolTrace = append(ls.toolTrace, name)
		logger.LogEvent(string(EventToolCall), "info", "tool_execute",
			name, fmt.Sprintf("调用 %s", name), tc.Args, result.Content, result.Error, result.Duration)
		return result.Content, result.Duration, execErr
	}

	return loopcore.Run(ctx, loopcore.Config{
		MaxRounds:  ls.maxTurns,
		KeepRecent: 0, // CA 自有滚动窗口压缩（maybeCompact）——内核不重复压
		WorkDir:    agentChatWorkDir,
	}, a.cfg.Model, sysPrompt, msgs, loopcore.Deps{
		Infer:        inferFn,
		Exec:         loopcore.ToolExec(execFn),
		Gate:         nil, // CA 工具走 ec.ExecuteTool 内建 gate
		Tools:        nil, // CA 工具经 callModel 内部 ls.tools（tools 参数透传 nil）
		Terminator:   term,
		OnToolResult: caToolResult(ls),
	})
}

// agentChatWorkDir — CA 工具工作目录（与 chat.ChatToolsWorkDir 同值——chat 包 import agent 故不能反向引用）
const agentChatWorkDir = "<repo>"

var _ = time.Now
