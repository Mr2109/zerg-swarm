// run.go — 循环内核主循环（唯一实现——流式/非流式同一份代码）
// 沉淀自 chat_handlers.go 流式循环: 五重防护（LoopGuard/空参数/坏格式/重复搜索/搜索无进展）+
// [成功·N字] 标注 + 三层时限 + 心跳事件 + 引导式收尾
package loopcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/ffp"
)

// Run — 运行工具循环（唯一实现——Deps.Infer 内部决定流式与否，内核只透传 delta 回调）
// appendReasoning — 多轮思考累积（2026-09-10 修复"思考内容不全"）
// 根因: 每轮 res.Reasoning = result.Reasoning 覆盖 → 多轮工具调用只留最后一轮思考。
// UI 实时缓冲是逐轮累加的，落库/刷新后却只剩末轮 → 用户看到"思考不全"。
func appendReasoning(acc, round string) string {
	round = strings.TrimSpace(round)
	if round == "" {
		return acc
	}
	if strings.TrimSpace(acc) == "" {
		return round
	}
	return acc + "\n\n" + round
}

func Run(ctx context.Context, cfg Config, model, sysPrompt string, msgs []map[string]any, d Deps) *Result {
	res := &Result{}
	maxRounds := cfg.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 10
	}
	emit := func(event, payload string) {
		if d.Events != nil {
			d.Events(event, payload)
		}
	}
	// 上下文轻量化（旧工具结果压缩——keepRecent 3）
	compact := func() {
		if cfg.KeepRecent > 0 && len(msgs) > 30 {
			msgs = CompactToolResults(msgs, cfg.KeepRecent)
		}
	}

	loopStart := time.Now()
	emptyArgsStreak := 0
	badFormatStreak := 0
	searchStreak := map[string]int{}
	guard := loopguardNew()

	for round := 1; round <= maxRounds; round++ {
		// 墙钟
		if cfg.WallClock > 0 && time.Since(loopStart) > cfg.WallClock {
			res.ExitKind = "wall_clock"
			emit("loop_hint", `{"kind":"wall_clock"}`)
			break
		}
		compact()

		// 单轮推理（RoundTimeout 限时——超时=引导收尾）
		var roundCtx context.Context
		var cancel context.CancelFunc
		if cfg.RoundTimeout > 0 {
			roundCtx, cancel = context.WithTimeout(ctx, cfg.RoundTimeout)
		} else {
			roundCtx, cancel = context.WithCancel(ctx)
		}
		var deltaOnce bool
		sink := func(deltaType, text string) {
			deltaOnce = true
			emit("delta", mustJSON(map[string]any{"type": deltaType, "text": text}))
		}
		result, err := d.Infer(roundCtx, model, sysPrompt, msgs, sink, d.Tools)
		cancel()
		if err != nil && isTimeout(err) {
			res.ExitKind = "round_timeout"
			emit("loop_hint", `{"kind":"round_timeout"}`)
			msgs = append(msgs, map[string]any{"role": "user", "content": "（时间到——请立即把已获得的信息整理成最终回答——不要继续调用工具或推理——直接给出结论——信息不足就说明没找到——绝不编造。）"})
			result, err = d.Infer(ctx, model, sysPrompt, msgs, nil, nil) // 收尾轮不带工具
		}
		if err != nil {
			// 瞬时故障重试（502/500——只在未流出 delta 时——防重复输出）
			if !deltaOnce && isTransient(err) {
				retried := false
				for attempt := 1; attempt <= 2; attempt++ {
					time.Sleep(2 * time.Second)
					emit("retry", mustJSON(map[string]any{"attempt": attempt, "reason": "X3 瞬时故障"}))
					result, err = d.Infer(ctx, model, sysPrompt, msgs, sink, d.Tools)
					if err == nil {
						retried = true
						break
					}
				}
				if retried {
					emit("retry_done", "{}")
				}
			}
			if err != nil {
				if deltaOnce {
					res.ExitKind = "stream_broken"
					emit("loop_hint", `{"kind":"stream_broken_midway"}`)
					break
				}
				res.Err = err.Error()
				return res
			}
		}
		// 2026-09-10 修复"思考内容不全": 多轮时每轮覆盖 → 落库只剩末轮(UI 实时看到的过程全丢)。
		// 中间轮(带工具调用)的正文=模型的思考/过程文本 → 归入思考；末轮才是最终回答。
		if len(result.ToolCalls) > 0 {
			mid := result.Content
			if i := strings.Index(mid, "<tool_call>"); i > 0 {
				mid = mid[:i]
			}
			res.Reasoning = appendReasoning(res.Reasoning, appendReasoning(result.Reasoning, mid))
		} else {
			res.Content = result.Content
			res.Reasoning = appendReasoning(res.Reasoning, result.Reasoning)
		}
		res.Usage.TotalTokens += result.TotalTokens

		// 无工具调用
		if len(result.ToolCalls) == 0 {
			// Terminator 仲裁（CA 契约判定/对话 nil=自然终止）
			if d.Terminator != nil {
				done, feedback := d.Terminator.OnNoToolCall(result)
				if feedback != "" {
					msgs = append(msgs, map[string]any{"role": "user", "content": feedback})
				}
				if done {
					res.ExitKind = "natural"
					return res
				}
				continue // 引导消息已追加——继续循环
			}
			if strings.Contains(result.Content, "<tool_call>") {
				// 坏格式（连续 3 次→收尾）
				badFormatStreak++
				if badFormatStreak >= 3 {
					msgs = append(msgs, map[string]any{"role": "user", "content": "你的工具调用格式一直无效（已 " + fmt.Sprint(badFormatStreak) + " 次）。请停止调用工具——用中文把已知信息整理成最终回答。"})
					res.ExitKind = "bad_format"
					break
				}
				msgs = append(msgs, map[string]any{"role": "user", "content": "工具调用格式无效（<tool_call> 内必须是一个 JSON 对象 {\"name\": \"工具名\", \"arguments\": {...}}）——请重新输出格式正确的工具调用"})
				continue
			}
			res.ExitKind = "natural"
			return res // 最终回复
		}
		badFormatStreak = 0

		// 执行工具（逐个）
		for _, tc := range result.ToolCalls {
			normalizeToolArgs(&tc)
			argsJSON := mustJSON(tc.Args)
			emit("tool_start", mustJSON(map[string]any{"name": tc.Name, "args": argsJSON}))
			startT := time.Now()

			var content, dur string
			var execErr error
			hidden := d.Hooks.IsHidden != nil && d.Hooks.IsHidden(tc.Name) && tc.Name != "tool_search"
			switch {
			case hidden:
				content = fmt.Sprintf("【系统】工具 %s 本对话已隐藏（连续 3 次执行失败）。请换其他工具或 tool_search 搜索替代。", tc.Name)
			default:
				content, dur, execErr = d.Exec(ctx, tc.Name, tc.Args)
			}
			// 耗时兜底（2026-09-17 实测：部分工具 d.Exec 不回耗时 ⇒ 轨迹里 Duration 恒空、观测面拿不到「工具耗了多久」）；空则用真实墙钟补，有值（bash 自带）则尊重原值。
			if dur == "" {
				dur = time.Since(startT).Round(time.Millisecond).String()
			}
			if d.Hooks.RecordOutcome != nil && tc.Name != "tool_search" {
				if execErr != nil {
					typ := errTypeOf(execErr.Error())
					if hint := d.Hooks.RecordOutcome(tc.Name, typ, execErr.Error()); hint != "" {
						content = hint + "\n" + content
					}
				} else if !strings.HasPrefix(content, "【bash") && !strings.HasPrefix(content, "【系统】") {
					d.Hooks.RecordOutcome(tc.Name, "", "")
				}
			}
			if execErr != nil {
				e := execErr.Error()
				if ffp.In(execErr.Error()) {
					// FFP 2026-09-08: 格式错误≠执行失败——教学文本直通,不加"执行失败"包装(否则与首行断言矛盾)
					content = e
				} else {
					content = fmt.Sprintf("工具执行失败: %s（%s）", e, truncateStr(content, 500))
				}
			}
			res.Traces = append(res.Traces, Trace{Round: round, CallID: tc.ID, Name: tc.Name, Args: argsJSON, Result: content, Error: errStr(execErr), Duration: dur})
			emit("tool", mustJSON(map[string]any{"name": tc.Name, "args": argsJSON, "result": truncateStr(content, 300)}))
			if d.OnToolResult != nil {
				d.OnToolResult(tc, content, execErr)
			}

			// 回传（Hermes 包装 + [成功·N字] 标注）
			assistantContent := tc.RawArgs
			if assistantContent == "" {
				assistantContent = fmt.Sprintf("<tool_call>\n{\"name\": \"%s\", \"arguments\": %s}\n</tool_call>", tc.Name, argsJSON)
			}
			status := "[失败]"
			if execErr != nil {
				if ffp.In(execErr.Error()) {
					status = "[格式反馈]" // FFP: 教学轮——非执行失败
				} else {
					status = "[失败]"
				}
			} else {
				status = fmt.Sprintf("[成功·%d字]", len([]rune(content)))
			}
			contentJSON := mustJSON(content)
			toolResp := fmt.Sprintf("<tool_response>\n{\"name\": \"%s\", \"content\": %s}\n</tool_response>", tc.Name, contentJSON)
			msgs = append(msgs,
				map[string]any{"role": "assistant", "content": assistantContent},
				map[string]any{"role": "tool", "tool_call_id": tc.ID, "content": status + " " + toolResp},
			)

			// 空参数计数
			if argsJSON == "{}" || argsJSON == "null" || argsJSON == "" {
				emptyArgsStreak++
			} else {
				emptyArgsStreak = 0
			}
			// 指纹守卫
			guard.Record(tc.Name, tc.Args)
			if ok, reason := guard.Detect(); ok {
				guide, upgrade := guard.BuildGuide(reason, toolNameList(d.Tools))
				if upgrade {
					emit("loop_hint", `{"kind":"loopguard_escalate"}`)
					msgs = append(msgs, map[string]any{"role": "user", "content": guide})
					// 收尾轮——不带工具让它总结
					final, ferr := d.Infer(ctx, model, sysPrompt, msgs, nil, nil)
					if ferr == nil {
						res.Content = final.Content
						res.Reasoning = appendReasoning(res.Reasoning, final.Reasoning)
						res.Usage.TotalTokens += final.TotalTokens
					}
					res.ExitKind = "loopguard_escalate"
					return res
				}
				msgs = append(msgs, map[string]any{"role": "user", "content": guide})
			}
			if emptyArgsStreak >= 3 {
				emit("loop_hint", `{"kind":"empty_args"}`)
				msgs = append(msgs, map[string]any{"role": "user", "content": "（连续多次空参数调用。请把到目前为止获得的信息整理成最终回答——信息不足就说明没找到——绝不编造。）"})
				final, ferr := d.Infer(ctx, model, sysPrompt, msgs, nil, nil)
				if ferr == nil {
					res.Content = final.Content
					res.Usage.TotalTokens += final.TotalTokens
				}
				res.ExitKind = "empty_args"
				return res
			}
			// 重复搜索计数（同一 query ≥2 强制提示）
			if tc.Name == "tool_search" {
				if q, _ := tc.Args["query"].(string); q != "" {
					searchStreak[q]++
					if d.Hooks.SearchStreak != nil {
						d.Hooks.SearchStreak(q)
					}
					if searchStreak[q] >= 2 {
						msgs = append(msgs, map[string]any{"role": "user", "content": "（你已搜索过 \"" + q + "\"——结果已在对话中。直接使用搜到的工具，或换一个思路——不要重复搜索。）"})
					}
				}
			}
			_ = startT
		}
	}
	if res.ExitKind == "" {
		res.ExitKind = "max_rounds"
	}
	return res
}

// ── 小工具 ──

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func isTimeout(err error) bool {
	e := err.Error()
	return strings.Contains(e, "context deadline") || strings.Contains(e, "超时")
}

func isTransient(err error) bool {
	e := err.Error()
	return strings.Contains(e, "502") || strings.Contains(e, "500")
}

func errStr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func truncateStr(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + fmt.Sprintf("…（共 %d 字符——已截断）", len(s))
}

func toolNameList(tools []map[string]any) []string {
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

// errTypeOf — 错误分类（exec/param/timeout——对齐 chat.ErrTypeOf 语义）
func errTypeOf(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "timeout") || strings.Contains(m, "超时"):
		return "timeout"
	case strings.Contains(m, "参数") || strings.Contains(m, "argument") || strings.Contains(m, "missing"):
		return "param"
	default:
		return "exec"
	}
}

// normalizeToolArgs — 工具参数统一解包（chat 包 NormalizeToolArgs 同逻辑——模型 Hermes 风格
// {name,arguments} 整个塞进 function.arguments——解一层覆盖+剔混入元键）
func normalizeToolArgs(tc *ToolCall) {
	if tc == nil {
		return
	}
	if v, ok := tc.Args["arguments"]; ok {
		switch a := v.(type) {
		case string:
			var m map[string]any
			if json.Unmarshal([]byte(a), &m) == nil {
				tc.Args = m
			}
		case map[string]any:
			tc.Args = a
		}
	}
	for _, k := range []string{"name", "type", "function"} {
		delete(tc.Args, k)
	}
}
