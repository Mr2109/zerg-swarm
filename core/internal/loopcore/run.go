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

	"github.com/Mr2109/zerg-swarm/core/internal/behaviorobs"
	"github.com/Mr2109/zerg-swarm/core/internal/ffp"
	"github.com/Mr2109/zerg-swarm/core/internal/toolobs"
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
	// T1.3 观测（会话级结论）：用 defer 在**全部**返回路径上收口 —— 一处接线覆盖所有终局
	// （自然收尾/轮数用尽/墙钟/超时/坏格式/守卫升级/推理失败），且不改任何一条既有 return 的语义：
	// 只**读** res 上报事实，不抛错、不阻塞、不参与判定（叶包 best-effort，见 internal/behaviorobs）。
	// 判据（在叶包里写死）：整会话零「写类工具**成功**」且已给出终答 ⇒ early_exit_suspected。
	// Rounds 用本层的真实轮次计数（不猜：由轮次循环自己数）。
	obsRound := 0
	defer func() {
		behaviorobs.EmitSession(behaviorobs.SessionFact{
			Session:  d.Session,
			Rounds:   obsRound,
			ExitKind: res.ExitKind,
			// 「已给出终答」的唯一判据：正常收尾（模型自己停了工具、交了正文）。
			// **不含**超时/轮数用尽/坏格式等终局 —— 那些是"没干完"而不是"早退交差"（防误报）。
			FinalAnswer: res.ExitKind == "natural" && strings.TrimSpace(res.Content) != "",
		})
	}()
	maxRounds := cfg.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 10
	}
	nowFn := d.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	emit := func(event, payload string) {
		if d.Events != nil {
			d.Events(event, payload)
		}
	}

	// ── T5.6 并发恢复互斥：**执行前置门**（位置本身是语义的一部分）──
	// 这一段必须在**任何节点执行之前**：本函数里第一次推理、第一次工具执行都在它之后。
	// 落败者在这里**直接返回** —— 不推理、不执行工具、**也不写检查点**（不去踩赢家的 run 状态；
	// 否则两个进程会往同一个 JSONL 里交错写，把恢复点写坏）。
	// 依据（实测）：k 个恢复者 ⇒ k 次受闸副作用，窗口 = 节点自身执行时间 ⇒ 事后去重救不回来。
	if d.Lease != nil && strings.TrimSpace(d.Session) != "" {
		holder := strings.TrimSpace(d.Holder)
		if holder == "" {
			holder = defaultHolder()
		}
		ttl := d.LeaseTTL
		if ttl <= 0 {
			ttl = DefaultLeaseTTL
		}
		release, _, err := d.Lease.Enter(d.Session, holder, ttl, nowFn())
		if err != nil {
			// 专项错误码（errors.Is ErrLeaseHeld）+ 当前持有者：调用方能答"谁挡住了我"
			res.ExitKind = "lease_rejected"
			res.Err = err.Error()
			return res
		}
		defer release()
	}

	// ── T5.5 恢复：从最后一份快照继续（**先认领、再恢复**，顺序不能反）──
	startRound := 1
	completedStep := 0
	if d.Resume != nil {
		if !Resumable(d.Resume.State.ExitKind) {
			// 完成过的 run 不再跑第二遍（"并发两次恢复 ⇒ 效果恰好一次"的第二道闸：即使两次恢复
			// 在时间上错开、lease 早已释放，也不重复执行已给出终答的那些节点）。
			res.ExitKind = "already_done"
			res.Err = fmt.Sprintf("loopcore: run=%s 已完成（终局 %q，无待办）⇒ 拒绝再恢复（防重复劳动/重复副作用）",
				d.Resume.RunID, d.Resume.State.ExitKind)
			return res
		}
		msgs = append([]map[string]any(nil), d.Resume.State.Messages...)
		res.Traces = append(res.Traces, d.Resume.State.Traces...)
		res.Usage.TotalTokens = d.Resume.State.TotalTokens
		completedStep = d.Resume.State.StepIndex
		startRound = completedStep + 1
		emit("resume", mustJSON(map[string]any{
			"run_id": d.Resume.RunID, "from_step": completedStep, "start_round": startRound,
			"version_marker": d.Resume.VersionMarker,
		}))
	}
	// 恢复的轮次预算：maxRounds 是**本次运行**可用的轮数（不是全程总数）——
	// 否则 max_rounds 退出后再恢复会因为 startRound > maxRounds 而一步都跑不动（那是死循环式失效）。
	lastRound := startRound - 1 + maxRounds

	// ── T5.5 每步快照（粒度 = 步：一轮收尾后的完整状态；步内不落盘）──
	// 只在"过了前置门"之后接线：落败者/已完成者上面就返回了，一步都不写。
	// 存不下 ⇒ 如实记进 res.CheckpointErr 并发事件，**不改变循环行为**（快照不该变成新的失败点）。
	persist := func(step int, exitKind string) {
		if d.Checkpoints == nil || strings.TrimSpace(d.RunID) == "" || step <= 0 {
			return
		}
		st := CheckpointState{
			RunID: d.RunID, Session: d.Session, StepIndex: step, Round: step,
			Messages: msgs, Traces: res.Traces, ExitKind: exitKind,
			TotalTokens: res.Usage.TotalTokens, Model: model,
		}
		if _, err := d.Checkpoints.Save(st); err != nil {
			res.CheckpointErr = fmt.Sprintf("step=%d: %v", step, err)
			emit("checkpoint_failed", mustJSON(map[string]any{
				"run_id": d.RunID, "step": step, "error": err.Error(),
			}))
		}
	}
	if d.Checkpoints != nil && strings.TrimSpace(d.RunID) != "" {
		// 终局快照：一处收口**全部**返回路径（与 T1.3 的观测 defer 同一纪律），
		// 带上 ExitKind ⇒ 下次恢复据此判"还有没有活要干"（Resumable）。
		// 步号的取法（只在这一处分岔，别处不再判）：
		//   · 可继续的终局（max_rounds/wall_clock/…）⇒ 停在**最后一个完整走完的步**上，
		//     恢复时从下一步继续（崩在半路的那一轮会被重跑 —— 跨崩溃 at-least-once）。
		//   · 已给答的终局（natural 等）⇒ 当前这一轮也已收尾 ⇒ 步号跟到本轮（不谎报"还差一步"）。
		defer func() {
			step := completedStep
			if res.ExitKind != "" && !Resumable(res.ExitKind) && obsRound > step {
				step = obsRound
			}
			persist(step, res.ExitKind)
		}()
	}
	// 上下文轻量化（旧工具结果压缩——keepRecent 3）
	compact := func() {
		if cfg.KeepRecent > 0 && len(msgs) > 30 {
			msgs = CompactToolResults(msgs, cfg.KeepRecent)
		}
	}

	loopStart := time.Now()
	seenCalls := map[string]int{} // 本轮「同工具+同参数」出现次数（治重复劳动：实测 15 轮里同一份设计稿被读 6 次）
	emptyArgsStreak := 0
	badFormatStreak := 0
	searchStreak := map[string]int{}
	guard := loopguardNew()

	for round := startRound; round <= lastRound; round++ {
		obsRound = round // T1.3：本层真实轮次（会话级结论用它当 Rounds，不猜）
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
			// T1.3 观测（**一处覆盖本分支全部出口**：终结仲裁引导/坏格式重试/自然收尾/坏格式收尾）：
			// 本轮工具调用数恒为 0 ⇒ 连续零调用轮数 +1、首次调用之前轮数 +1（由叶包累加）。
			obsBehaviorRound(d.Session, round, 0, 0)
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
		// T1.3 观测：本轮**成功**的写类工具计数（只数、不改行为；判据 = 判定层没挡 + 执行无错 + 名字在写类白名单）
		writeOK := 0
		for _, tc := range result.ToolCalls {
			normalizeToolArgs(&tc)
			argsJSON := mustJSON(tc.Args)
			callKey := tc.Name + "|" + argsJSON
			seenCalls[callKey]++
			emit("tool_start", mustJSON(map[string]any{"name": tc.Name, "args": argsJSON}))
			startT := time.Now()

			var content, dur string
			var execErr error
			hidden := d.Hooks.IsHidden != nil && d.Hooks.IsHidden(tc.Name) && tc.Name != "tool_search"
			switch {
			case hidden:
				// T1.2 观测：**内核侧拒绝分支** —— 工具被隐藏 ⇒ 本次调用不执行。
				// 没有这一条，"被系统挡下"与"模型根本没调"在观测面同形（本任务要治的盲区）。
				// 注意：allow 一律由工具执行路径（agent.ExecuteTool）记，此处**只记拒绝**，避免同一次调用两条判定。
				toolobs.Emit(toolobs.Decision{
					Session: d.Session, Tool: tc.Name, Decision: toolobs.DecisionDeny,
					Reason: toolobs.ReasonToolHidden, ArgsDigest: toolobs.Digest(tc.Args),
				})
				content = fmt.Sprintf("【系统】工具 %s 本对话已隐藏（连续 3 次执行失败）。请换其他工具或 tool_search 搜索替代。", tc.Name)
			default:
				content, dur, execErr = d.Exec(ctx, tc.Name, tc.Args)
			}
			// T1.3 观测（只数、不改行为）：本次调用是**成功的写类工具**吗？
			// 三条件缺一不可 —— ① 没被隐藏（被系统挡下不算干活）② 执行无错（失败不算写成功）
			// ③ 工具名在写类白名单里（闭集，见 behaviorobs.IsWriteTool；bash 不在其中 ⇒ 本信号取"写"的下界）。
			if !hidden && execErr == nil && behaviorobs.IsWriteTool(tc.Name) {
				writeOK++
			}
			// 耗时兜底（2026-09-17 实测：部分工具 d.Exec 不回耗时 ⇒ 轨迹里 Duration 恒空、观测面拿不到「工具耗了多久」）；空则用真实墙钟补，有值（bash 自带）则尊重原值。
			if dur == "" {
				dur = time.Since(startT).Round(time.Millisecond).String()
			}
			// 重复调用提示（2026-09-17 实测：模型反复读同一文件/同一段代码 ⇒ 轮数被「找东西」吃光）
			// 只做提示、不改结果内容；第 2 次起提示，避免噪声。
			if n := seenCalls[callKey]; n > 1 && execErr == nil {
				content = fmt.Sprintf("【提示】这是本轮第 %d 次完全相同的调用（前次已执行，结果同上）。若信息已足够，请直接给结论或换一种做法；不要重复调用。\n%s", n, content)
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
					// T1.3 观测：本轮确实执行过工具 ⇒ 照常上报（否则这条终局会在行为观测面留一个空档）
					obsBehaviorRound(d.Session, round, len(result.ToolCalls), writeOK)
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
				// T1.3 观测：同上——本轮执行过工具，不能因为提前收尾就少一轮信号
				obsBehaviorRound(d.Session, round, len(result.ToolCalls), writeOK)
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
		// T1.3 观测：本轮工具全部执行完 ⇒ 上报轮级行为信号（本轮几次调用 + 其中几次写成功）。
		// 这是**每轮**都要落的一条：它让"跑到第几轮了、到目前为都没写过东西"在观测面直接可读。
		obsBehaviorRound(d.Session, round, len(result.ToolCalls), writeOK)
		// T5.5：**一步收尾**（本轮推理 + 本轮全部工具执行都完成了）⇒ 落这一份快照。
		// 位置是语义的一部分：只在这一步的边界上落，绝不在步内落（半个状态不可执行也不可解释）。
		completedStep = round
		persist(round, "")
	}
	if res.ExitKind == "" {
		res.ExitKind = "max_rounds"
	}
	// ⑤ 四终局（2026-09-17）：预算/轮数用尽**不得只是 return 半成品** ✗ ——
	// 依业界口径：「agent 永不直接崩，它要**做出决定**」；只有"返回答案/抛异常"两种终局是生产级缺口。
	// 依据只取**本层可知的事实**（不猜）：已完成步数=工具轨迹条数；是否还有未完成工作=是否因轮数用尽而退出。
	if res.ExitKind != "natural" && res.ExitKind != "" {
		in := TerminalInput{
			CompletedSteps:   len(res.Traces),
			HasRemainingWork: res.ExitKind == "max_rounds",
			Checkpointable:   len(res.Traces) > 0,
			NeedsHumanJudgment: res.ExitKind == "loopguard_escalate" || res.ExitKind == "round_timeout" ||
				res.ExitKind == "wall_clock" || res.ExitKind == "stream_broken",
			// HasIrreversible：本层看不到"是否已改文件"（在工具层）⇒ 保守留 false，**不猜**；
			// 待 v2.5.11 由工具轨迹带"写类"标记后启用（承接项 S3）。
		}
		st, reason := ChooseTerminal(in)
		note := TerminalNote(st, reason+"（退出原因："+res.ExitKind+"）")
		// 实验开关（默认关）：ZERG_SENDBACK=1 ⇒ 改为"打回重做"（供 A/B 实测"打回是帮助还是伤害"）
		if SendBackEnabled() && res.ExitKind == "max_rounds" {
			note = SendBackNote(reason + "（退出原因：" + res.ExitKind + "）")
		}
		if res.Content == "" {
			res.Content = note
		} else {
			res.Content = res.Content + "\n\n" + note
		}
	}
	return res
}

// ── 小工具 ──

// obsBehaviorRound — T1.3：把「本轮调了几次工具 / 其中几次是**成功**的写类工具」上报行为观测面
// （internal/behaviorobs 叶子包）。
//
// 纪律（与 T1.2 的 toolobs.Emit 同源）：**只上报事实、没有任何返回值** ⇒ 结构上不可能因它改变循环行为；
// 无会话（chat 侧一定给，CA 侧可能为空）时叶包忽略——行为信号按会话归因才有意义（不编造会话）。
func obsBehaviorRound(session string, round, toolCalls, writeOK int) {
	behaviorobs.EmitRound(behaviorobs.Fact{
		Session: session, Round: round, ToolCalls: toolCalls, WriteOK: writeOK,
	})
}

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
