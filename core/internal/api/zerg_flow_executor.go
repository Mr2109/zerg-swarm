// Package api - v2.5.6 程序定量驱动 Agent: 流程执行器
// 新流程任务（Flow=zerg）——状态机驱动——内部调模型——不 spawn CA
// 方案轮→选定轮→执行轮（受控循环）→封闭轮——程序固定——定量驱动
// 2026-08-25——设计 docs/01-设计/设计-程序定量驱动Agent-20260824.md
package api

import (
	"encoding/json"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ModelCaller 模型调用接口（网关——注入）
// 返回: 模型输出文本（可能是 JSON——按阶段解析）
// v2.5.6 工具支持: 请求带 tools 时——模型调工具——返回 tool_calls 的 arguments（JSON 文本）
type ModelCaller func(systemPrompt, userPrompt string, maxTokens int) (string, error)

// ZergFlowExecutor 流程执行器（一个任务的全流程驱动）
type ZergFlowExecutor struct {
	Task       *Task
	TaskDir    string        // 任务目录（/tmp/zerg-tasks/<ID>/）
	Git        *ZergGit      // 任务 git
	TaskFile   *ZergTaskFile // task.jsonl
	Skill      *ZergSkill
	TaskType   string      // 模型判断的任务类型（skill 类型库 key——外部任务用）
	CallModel  ModelCaller // 模型调用（注入——网关）
	SkillsRoot string      // Skill 库根
}

// NewZergFlowExecutor 新建执行器（任务建立——git + jsonl + skill 模板）
func NewZergFlowExecutor(task *Task, callModel ModelCaller, skillsRoot string) (*ZergFlowExecutor, error) {
	taskDir := filepath.Join(statepath.TaskRoot(), sanitizeID(task.ID))
	e := &ZergFlowExecutor{
		Task:       task,
		TaskDir:    taskDir,
		CallModel:  callModel,
		SkillsRoot: skillsRoot,
	}
	// 1. git（任务仓库）
	git, err := NewZergGit(taskDir)
	if err != nil {
		return nil, fmt.Errorf("failed to set up task git: %w", err)
	}
	e.Git = git
	// 2. task.jsonl（首行元数据）
	zf := NewZergTaskFile(taskDir)
	if err := zf.Init(task.ID, task.Type, task.Description); err != nil {
		return nil, fmt.Errorf("failed to set up task.jsonl: %w", err)
	}
	e.TaskFile = zf
	// 3. Skill（模板起步——执行中进化）
	// v2.5.6 独属 key: 内部任务=def.ID（沉淀后独属 skill）——外部空=按类型共享
	e.Skill = NewZergSkill(skillsRoot, taskDir, task.SkillKey)
	return e, nil
}

// Run 执行全流程（方案轮→选定轮→执行轮→封闭轮）
func (e *ZergFlowExecutor) Run() error {
	// 0. 模型判断类型（Mr2109——Skill 类型机制）
	taskType, err := e.assessType()
	if err != nil {
		return fmt.Errorf("type detection failed: %w", err)
	}

	// 1. 方案轮
	plan, err := e.planPhase(taskType)
	if err != nil {
		return err
	}

	// 2. 选定轮
	sel, err := e.selectPhase(plan)
	if err != nil {
		return err
	}

	// 3. 执行轮（受控循环——每个 step 一个）
	for _, step := range sel.RefinedSteps {
		if err := e.executeStep(step, taskType); err != nil {
			// 超限——git 回溯重做（带失败反馈——重试一次）
			if isRebuildErr(err) {
				if err2 := e.rebuildAndRetry(step, taskType); err2 != nil {
					return err2
				}
				continue
			}
			return err
		}
	}

	// 4. 封闭轮（读 git 树写报告）
	if err := e.closePhase(); err != nil {
		return err
	}

	// 5. 候选 skill 试跑收尾（v2.5.6 候选机制——Mr2109 2026-08-28）
	// 本次任务命中候选（试跑）且成功——候选转正（覆盖正式版——下次直接用）
	if e.Skill.UsingCandidate {
		promoted, err := e.Skill.PromoteCandidate(e.Task.SkillKey, e.TaskType)
		if err != nil {
			log.Printf("⚠️ task %s candidate skill promotion failed: %v", e.Task.ID, err)
		} else if promoted {
			log.Printf("🧬 task %s candidate skill trial succeeded — promoted (overwrites official — used next time)", e.Task.ID)
		}
	}

	// 6. 完成
	e.Task.Status = "done"
	return nil
}

// assessType 模型判断任务类型（无该类型→建新类型）
func (e *ZergFlowExecutor) assessType() (string, error) {
	prompt := `请判断这个任务属于什么类型（如"代码修bug"/"文档整理"/"调研"——简洁名称——中文）。只输出类型名（不要多余文字）。`
	out, err := e.CallModel(prompt, "任务: "+e.Task.Description, 0)
	if err != nil {
		return "", fmt.Errorf("type detection model call failed: %w", err)
	}
	taskType := strings.TrimSpace(out)
	if taskType == "" {
		taskType = "通用任务"
	}
	e.TaskType = taskType // v2.5.6: 存执行器——进化自省用（外部任务类型库 key）
	// Skill: 有类型库用进化版——没有等执行后注册（Skill 自举）
	skillContent, _ := e.Skill.Get(taskType)
	_ = e.Skill.Create(taskType, skillContent)
	log.Printf("🧠 task %s type: %s (Skill: %s)", e.Task.ID, taskType, skillContent[:30])
	return taskType, nil
}

// planPhase 方案轮（模型出 1-3 方案——工具调用 submit_plan——JSON 校验）
func (e *ZergFlowExecutor) planPhase(taskType string) (*PlanOutput, error) {
	skill, _ := e.Skill.Get(taskType)
	prompt := "任务: " + e.Task.Description + "\n\n【Skill 指导】\n" + truncateStr(skill, 1500) +
		"\n\n请调用 submit_plan 工具提交你的执行方案（1-3 个）。每个方案: id(A/B/C)、title(方案名)、steps(拆解步骤数组)、approach(方法思路)。"
	// 工具调用（模型调 submit_plan——输出结构化——比纯文本 JSON 可靠）
	// "TOOL:" 前缀触发 callGatewayModel 工具模式（带 submit_plan 工具）
	// v2.5.6 效率优化（Mr2109 2026-08-28 纠正）: max_tokens 是上限不是目标——统一 32768
	// （qwen38 适配器默认——reasoning+content 都装——模型自然思考自然停——不掐思考）
	out, err := e.CallModel(prompt, "TOOL:请调用 submit_plan 提交方案", 0)
	if err != nil {
		return nil, fmt.Errorf("plan round model failed: %w", err)
	}
	plan, err := ParsePlan(extractJSON(out))
	if err != nil {
		// v2.5.6 系统适配: 模型没调工具/输出散文——重试 3 次（每次强调工具调用）
		plan = nil
		for attempt := 1; attempt <= 3; attempt++ {
			hint := "请务必调用 submit_plan 工具（不要输出文字——直接调用工具——JSON 参数: plans 数组——每个含 id/title/steps/approach）。"
			out2, err2 := e.CallModel("上次没提交有效方案。"+hint, "TOOL:请调用 submit_plan 提交方案", 0)
			if err2 != nil {
				continue
			}
			p2, err3 := ParsePlan(extractJSON(out2))
			if err3 == nil {
				plan = p2
				break
			}
		}
		// 重试仍失败——程序降级兜底（系统适配——不让任务失败）
		if plan == nil {
			plan = &PlanOutput{Plans: []PlanItem{
				{ID: "A", Title: "直接执行任务", Steps: []string{e.Task.Description}, Approach: "按任务描述直接执行"},
			}}
			log.Printf("⚠️ plan round produced no valid plan — falling back to a generated default (system adaptation — model instability must not block)")
		}
	}
	// 写 task.jsonl + git commit
	if _, err := CommitRoundWithFS(e.TaskFile, e.Git, PhasePlan, "", map[string]interface{}{"plans": plan.Plans}, fmt.Sprintf("出了%d个方案", len(plan.Plans))); err != nil {
		return nil, err
	}
	return plan, nil
}

// selectPhase 选定轮（模型选一个——工具调用 submit_select——JSON 校验）
func (e *ZergFlowExecutor) selectPhase(plan *PlanOutput) (*SelectOutput, error) {
	planJSON, _ := jsonMarshal(plan.Plans)
	prompt := "【方案列表】\n" + planJSON +
		"\n\n请调用 submit_select 工具选定一个方案（selected=方案 id、reason=理由、refined_steps=细化步骤数组——执行轮的小任务列表）。"
	// 工具调用（模型调 submit_select——输出结构化）
	out, err := e.CallModel(prompt, "TOOL:请调用 submit_select 选定方案", 0)
	if err != nil {
		return nil, fmt.Errorf("selection round model failed: %w", err)
	}
	sel, err := ParseSelect(extractJSON(out))
	if err != nil {
		// v2.5.6 系统适配: 模型没调工具/输出散文——重试 3 次
		sel = nil
		for attempt := 1; attempt <= 3; attempt++ {
			out2, err2 := e.CallModel("上次没提交有效选定。请务必调用 submit_select 工具（JSON 参数: selected/reason/refined_steps）。", "TOOL:请调用 submit_select 选定方案", 0)
			if err2 != nil {
				continue
			}
			s2, err3 := ParseSelect(extractJSON(out2))
			if err3 == nil {
				sel = s2
				break
			}
		}
		// 重试仍失败——程序兜底（选第一个方案——步骤=方案标题）
		if sel == nil {
			sel = &SelectOutput{Selected: plan.Plans[0].ID, Reason: "程序兜底（模型未提交有效选定）", RefinedSteps: plan.Plans[0].Steps}
			log.Printf("⚠️ selection round produced no valid choice — falling back to the first plan (system adaptation — model instability must not block)")
		}
	}
	// 校验 selected 在 plans 内
	valid := false
	for _, p := range plan.Plans {
		if p.ID == sel.Selected {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("选定轮 selected=%s 不在方案内（方案: %s）", sel.Selected, planIDs(plan))
	}
	if _, err := CommitRoundWithFS(e.TaskFile, e.Git, PhaseSelect, "", map[string]interface{}{"selected": sel.Selected, "steps": sel.RefinedSteps}, "选定方案"+sel.Selected); err != nil {
		return nil, err
	}
	return sel, nil
}

// planIDs 方案 id 列表（错误信息用）
func planIDs(plan *PlanOutput) string {
	ids := []string{}
	for _, p := range plan.Plans {
		ids = append(ids, p.ID)
	}
	return strings.Join(ids, ",")
}

// ActionFormatPrompt 动作格式约定（提示语告诉模型——程序按此解析）
// v2.5.6 自然语言接口: 模型思考自由——但表达遵守约定格式（程序精准解析）
// 2026-08-28 动作库扩充（Mr2109——10 种动作——模型自然语言表达——程序精准执行）
const ActionFormatPrompt = `
【动作格式约定（重要——请按此格式表达你的动作）】:
- 创建文件: 创建 <文件名> 内容 <内容>（如: 创建 hello.txt 内容 zerg-test）
- 修改文件: 修改 <文件名> 把 <旧文本> 改成 <新文本>（精确替换——旧文本必须与文件原文一致）
- 追加内容: 追加 <文件名> 内容 <内容>（文件末尾加——不覆盖原内容）
- 读文件: 看看 <文件名>（返回文件内容）
- 搜索: 搜索 <关键词>（在任务目录里找——返回命中位置）
- 运行命令: 运行 <命令>（如: 运行 go test ./...）
- 查看目录: 看看当前目录
- 提交进度: 提交任务产出（git 提交当前进度）
- 任务完成: 任务完成（结束——产出已齐）
- 其他想法: 直接说明（程序会理解或询问）
请直接说出你要做的动作（按格式）——不要说 JSON——不要调工具——自然语言即可。`

// executeStep 执行一个步骤（受控循环——验证过退出/反馈重做/超限回溯）
func (e *ZergFlowExecutor) executeStep(step, taskType string) error {
	// v2.5.6 观感优化（2026-08-29——t10）: 执行轮进度日志——任务在跑但模型长思考时——
	// UI/日志能看到"正在执行第几步"（不再看起来卡死——思考是正常的不是卡住）
	log.Printf("▶ task %s executing step: %s\n", e.Task.ID, truncateStr(step, 80))
	// 上步总结（记忆接力——无记忆模型靠它）
	prevSummary, _ := e.TaskFile.Summary(3)
	loop := NewControlledLoop(e.TaskDir)
	loop.MaxRounds = 5
	// 验证命令（按任务类型——代码任务 go test——通用 git status）
	loop.VerifyCmd = defaultVerifyFor(taskType, e.TaskDir)

	res, err := loop.RunStep(step, func(s, feedback string) (string, error) {
		skill, _ := e.Skill.Get(taskType)
		prompt := fmt.Sprintf("【当前小任务】%s\n\n【上步总结】\n%s\n\n【Skill 指导】\n%s\n\n%s%s",
			s, prevSummary, truncateStr(skill, 1200), feedback, ActionFormatPrompt)
		// v2.5.6 观感优化（t10）: 模型调用前后日志——长思考时看到"思考中"不是卡死
		log.Printf("  ⏳ task %s model thinking… (%s — liveness check — thinking is not stuck)\n", e.Task.ID, truncateStr(s, 60))
		// v2.5.6 自然语言模式: 模型按约定格式表达——程序解析执行（不强制工具调用）
		// v2.5.6 效率优化（Mr2109 2026-08-28 纠正）: max_tokens 是上限不是目标——
		// 模型正常完成自然停止（finish_reason=stop）——不会"想满"max_tokens
		// 设 2048 会掐断正常思考（复杂任务想 3000 token 就被砍——质量受损——违背"思考不能关"）
		// 正确: 32768（qwen38 适配器默认——reasoning+content 都装）——模型自然思考自然停
		// 思考深度由 reasoning_effort 控制（Mr2109 low）——活性检测保证思考中不超时
		out, err := e.CallModel(prompt, s, 0)
		if err != nil {
			return "", err
		}
		// v2.5.6 观感优化（t10）: 模型产出——任务在推进
		log.Printf("  ✅ task %s model output (%d chars) — parsing action\n", e.Task.ID, len(out))
		// Semantic Parse: 从模型自然语言提取动作——程序执行（精准）
		act := ParseAction(out, e.TaskDir)
		if act.Type == "other" {
			// v2.5.6 Mr2109: 程序无法识别——回馈模型出错（让模型纠正——不能静默）
			// 反馈错误——下一轮模型按约定格式重新表达
			return "", fmt.Errorf("无法识别你的动作（%s）——请按约定格式重新表达（创建 <文件名> 内容 <内容> / 修改 <文件> 把 <旧> 改成 <新> / 运行 <命令> / 看看 <文件> / 搜索 <关键词> / 看看当前目录 / 任务完成）", truncateStr(out, 80))
		}
		if act.Type == "done" {
			// v2.5.6 2026-08-28: 任务完成——结束受控循环（产出已齐——程序标记）
			return "任务完成（程序识别——产出已提交）", nil
		}
		// v2.5.6 2026-08-28 验证自动化: 探索类动作（read/search/list/answer）不算完成证据
		// 模型必须执行产出动作（write/patch/append/run/git）才能完成步骤——假完成防线（替代"模型响应即过"）
		if act.Type == "read" || act.Type == "search" || act.Type == "list" || act.Type == "answer" {
			return "", fmt.Errorf("本轮只做了探索（%s）——这不算完成——请执行产出动作: 创建/修改/追加文件或运行命令完成本步骤（探索可以——但必须最终产出）", act.Type)
		}
		// v2.5.6 2026-08-28 放宽: list 探索允许（模型需了解环境——受控循环 MaxRounds 已限制轮数）
		// 假完成防靠受控循环验证（后续版本加产出文件检查）
		execOut, execErr := ExecuteActionInDir(act, e.TaskDir)
		if execErr != nil {
			return "", execErr
		}
		return execOut, nil
	})
	if err != nil {
		return err
	}
	if res.NeedRebuild {
		return fmt.Errorf("REBUILD: %s", res.FailReason)
	}
	// 提交（产出 + 总结——git 树记忆）
	if _, err := CommitRoundWithFS(e.TaskFile, e.Git, PhaseExecute, step, map[string]interface{}{"result": truncateStr(res.LastOutput, 200)}, truncateStr(res.LastOutput, 80)); err != nil {
		return err
	}
	return nil
}

// rebuildAndRetry git 回溯重做（超限——干净状态 + 失败反馈）
func (e *ZergFlowExecutor) rebuildAndRetry(step, taskType string) error {
	// 回溯到上一个 commit（干净）
	if _, err := e.Git.RevertTo(max(0, e.Git.CommitCount()-1)); err != nil {
		return err
	}
	// 带失败反馈重做
	loop := NewControlledLoop(e.TaskDir)
	loop.MaxRounds = 5
	loop.VerifyCmd = defaultVerifyFor(taskType, e.TaskDir)
	res, err := loop.RunStep(step, func(s, feedback string) (string, error) {
		skill, _ := e.Skill.Get(taskType)
		prompt := fmt.Sprintf("【重做（上次失败）】%s\n\n【失败反馈】%s\n\n【Skill】%s", s, feedback, truncateStr(skill, 1200))
		return e.CallModel(prompt, "TOOL:"+s, 0)
	})
	if err != nil {
		return err
	}
	if res.NeedRebuild {
		return fmt.Errorf("retry still failed: %s", res.FailReason)
	}
	if _, err := CommitRoundWithFS(e.TaskFile, e.Git, PhaseExecute, step, map[string]interface{}{"result": truncateStr(res.LastOutput, 200)}, "重做成功: "+truncateStr(res.LastOutput, 60)); err != nil {
		return err
	}
	return nil
}

// closePhase 封闭轮（模型读 git 树写报告——最后一轮 commit）
func (e *ZergFlowExecutor) closePhase() error {
	zc := NewZergClose(e.TaskDir, e.Git, e.TaskFile)
	prompt := zc.ReportPrompt()
	out, err := e.CallModel(prompt, "写结论报告（markdown——写入指定路径）", 0)
	if err != nil {
		return fmt.Errorf("closure report model failed: %w", err)
	}
	// 报告（模型写的——程序兜底路径）
	reportPath := filepath.Join(e.TaskDir, "reports", "final.md")
	reportContent := out
	if data, err := os.ReadFile(reportPath); err == nil && len(data) > 0 {
		reportContent = string(data)
	}
	// v2.5.6 修复（2026-08-29）: 报告内容无论合不合格都写文件（reports/final.md）——
	// 之前只在"不合格"分支 SaveReport——合格报告从不落盘→CloseCommit 找不到文件→任务失败
	if _, err := zc.SaveReport(reportContent); err != nil {
		return err
	}
	// v2.5.6 缺口2 修复: 封闭报告验证——有实质内容（非思考文本/非空壳）
	// 思考文本特征: "Let me analyze" / "I need to" / 空壳（<100 字）
	trimmed := strings.TrimSpace(reportContent)
	looksLikeThinking := strings.Contains(trimmed, "Let me") || strings.Contains(trimmed, "I need") ||
		strings.Contains(trimmed, "I'm going") || strings.Contains(trimmed, "Let me analyze")
	if len(trimmed) < 100 || looksLikeThinking {
		// 报告不合格——用模型输出重新生成（或兜底: 从模型输出提取实质部分）
		// 模型输出可能含实质内容（去掉思考前缀）
		cleaned := stripThinkingPrefix(out)
		if len(strings.TrimSpace(cleaned)) < 100 {
			// v2.5.6 修复（2026-08-29 实证——外部任务假完成: 报告26字节只有标题——模型"写文件"写空壳蒙混）:
			// 清理后仍不合格——重试一次（带"报告太短"反馈——让模型重新写实质内容）
			log.Printf("⚠️ closure report too thin (%d chars) — regenerating (model wrote a shell — anti fake-completion)\n", len(trimmed))
			retryPrompt := "上次报告内容太短（不足100字——疑似空壳）。请写一份实质性的结论报告（markdown——至少200字——包含: 做了什么/验证结果/发现的问题/结论——写入指定路径）。"
			out2, err2 := e.CallModel(retryPrompt, "写结论报告（markdown——写入指定路径）", 0)
			if err2 == nil {
				cleaned2 := stripThinkingPrefix(out2)
				if len(strings.TrimSpace(cleaned2)) >= 100 {
					cleaned = cleaned2
					reportContent = cleaned
					if _, err := zc.SaveReport(reportContent); err != nil {
						return err
					}
				}
			}
			// 重试仍不合格——强制失败（假完成拦截——任务不能带着空报告 done）
			if len(strings.TrimSpace(reportContent)) < 100 {
				return fmt.Errorf("closure report unqualified (%d chars only — shell report) — task marked failed (anti fake-completion)", len(strings.TrimSpace(reportContent)))
			}
			// 重试合格——正常走下面 CloseCommit
		} else {
			reportContent = cleaned
			if _, err := zc.SaveReport(reportContent); err != nil {
				return err
			}
		}
	}
	if err := zc.CloseCommit(reportPath, "任务完成——报告提交"); err != nil {
		return err
	}
	// 封闭轮也写 task.jsonl——v2.5.6 修复（2026-08-29 q1）: 失败记日志（不阻塞——报告已提交）
	if _, err := CommitRoundWithFS(e.TaskFile, e.Git, PhaseClose, "", map[string]interface{}{"report": "reports/final.md"}, "封闭——报告提交"); err != nil {
		log.Printf("⚠️ task %s closure round task.jsonl commit failed: %v", e.Task.ID, err)
	}
	// v2.5.6 Skill 进化自省（Mr2109 2026-08-27）: 任务结束提交 skill 前——读当前 skill——模型判断
	// 这次任务是否需要优化 skill——需要则进化（内部任务→独属 skill 写回）
	if err := e.evolveSkillIfNeeded(); err != nil {
		log.Printf("⚠️ task %s skill evolution introspection failed (does not block completion): %v", e.Task.ID, err)
	}
	return nil
}

// evolveSkillIfNeeded skill 进化自省（Mr2109 2026-08-27）
// 读当前 skill → 结合本次任务总结 → 让模型判断是否需要优化 → 需要则产出优化版
// v2.5.6 候选机制（2026-08-28 Mr2109）: 进化产出先存候选（SKILL.candidate.md）——
//
//	不直接覆盖正式版——下次任务用候选试跑——成功转正/失败作废——防"模型自评进化"越进化越差
//
// 任何失败只告警不阻塞（任务已完成——skill 进化是加分项）
func (e *ZergFlowExecutor) evolveSkillIfNeeded() error {
	// 当前 skill（任务内/独属/类型库/模板）
	cur, _ := e.Skill.Get(e.TaskType)
	if strings.TrimSpace(cur) == "" {
		return nil
	}
	// 本次任务总结（task.jsonl 最近几轮）
	summary, _ := e.TaskFile.Summary(6)
	key := e.Task.SkillKey
	owner := "该任务独属"
	if key == "" {
		owner = "类型库共享（" + e.TaskType + "）"
	}
	prompt := fmt.Sprintf(`【Skill 进化自省——任务已完成】
请阅读当前 Skill 和本次任务执行记录，判断这次任务是否学到了值得沉淀的新经验（更好的步骤/方法/坑/注意事项）。

当前 Skill（%s）:
%s

本次任务执行记录:
%s

请判断: 这次任务需要优化这个 Skill 吗？
- 如果不需要（Skill 已足够好——本次执行没有新经验）——只输出: NO
- 如果需要——输出优化后的完整 Skill 内容（保留原有用部分 + 融入本次经验——markdown 全文——不要解释）
输出格式: 第一行必须是 NO 或 SKILL:`+"`"+`
SKILL:
（优化后的完整 SKILL.md 内容）`, owner, truncateStr(cur, 4000), truncateStr(summary, 2000))
	out, err := e.CallModel(prompt, "判断并输出（NO 或 SKILL: 开头）", 0)
	if err != nil {
		return fmt.Errorf("evolution decision model call failed: %w", err)
	}
	trimmed := strings.TrimSpace(out)
	if strings.HasPrefix(trimmed, "NO") || strings.HasPrefix(trimmed, "no") {
		log.Printf("🧬 task %s skill introspection: no evolution needed (keeping as is)", e.Task.ID)
		return nil
	}
	// 提取 SKILL: 之后的内容
	content := trimmed
	if idx := strings.Index(trimmed, "SKILL:"); idx >= 0 {
		content = strings.TrimSpace(trimmed[idx+len("SKILL:"):])
	}
	if len(content) < 50 {
		log.Printf("⚠️ task %s skill evolution output too short (%d chars) — ignored", e.Task.ID, len(content))
		return nil
	}
	// v2.5.6 候选机制: 进化产出存候选——不覆盖正式版——下次任务试跑成功转正
	if err := e.Skill.SaveCandidate(key, e.TaskType, content); err != nil {
		return err
	}
	where := "类型库"
	if key != "" {
		where = "独属 skills/" + key
	}
	log.Printf("🧬 task %s skill evolution done (candidate: %s/SKILL.candidate.md — %d chars — promoted after a successful trial next task)", e.Task.ID, where, len(content))
	return nil
}

// stripThinkingPrefix 去掉模型输出的思考前缀（"Let me..."等——取实质内容）
func stripThinkingPrefix(s string) string {
	// 找第一个"结论报告"或"# "标题——从那里开始
	lines := strings.Split(s, "\n")
	start := 0
	for i, l := range lines {
		if strings.HasPrefix(l, "# ") || strings.Contains(l, "结论") || strings.Contains(l, "报告如下") {
			start = i
			break
		}
	}
	return strings.Join(lines[start:], "\n")
}

// defaultVerifyFor 默认验证命令（按任务类型）
func defaultVerifyFor(taskType, workdir string) []string {
	// 代码任务——go test（若有 go.mod）
	if _, err := os.Stat(filepath.Join(workdir, "go.mod")); err == nil {
		return []string{"go", "test", "./..."}
	}
	// 通用——无命令（git status 非空检查——ControlledLoop 退化逻辑）
	return nil
}

// extractJSON 从模型输出提取 JSON（兼容文字包裹/代码块）
func extractJSON(s string) string {
	// 1. 去代码块（```json ... ```）
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	// 2. 找第一个 { 到最后一个 }
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

// jsonMarshal 简单 JSON 序列化
func jsonMarshal(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// isRebuildErr 判断是否回溯重做错误
func isRebuildErr(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "REBUILD:")
}

// ========== v2.5.6 模型探活 ping（Mr2109 2026-08-28——任务执行前先探活） ==========

// pingModel 模型探活（v2.5.6 三级漏斗——Mr2109 2026-08-28 效率优化）
// 用途: ① 任务派发前探活（坏模型不派发）② 恢复调度器判定（环境恢复才重派）
// 三级漏斗（按耗时从短到长——效率最高时间最短）:
//
//	第1级: 快照优先（0ms）——机器 healthy + 已加载目标模型 → 直接通过（不发请求）
//	第2级: 响应头探测（~1-3s）——发请求只等响应头不等 body——链路通+模型接单=通过
//	第3级: 完整推理（16s+）——只有响应头到了但怀疑干不了活才等完整响应——几乎不用
//
// 返回: ok=模型可推理 / err=失败原因（分类）
func (s *MasterScheduler) pingModel(model string) (bool, error) {
	if model == "" {
		return true, nil // 无模型指定——不探活（放行）
	}
	// 探活缓存（10s——机器状态不会秒变——排队任务不重复 ping）
	s.pingMu.Lock()
	if s.pingCache != nil {
		if hit, ok := s.pingCache[model]; ok && time.Since(hit.at) < 10*time.Second {
			s.pingMu.Unlock()
			return hit.ok, hit.err
		}
	} else {
		s.pingCache = map[string]pingResult{}
	}
	s.pingMu.Unlock()

	// —— 第1级: 快照优先（0ms——不发请求——最快）——
	// 机器 healthy + 已加载目标模型 → 直接通过
	// 快照每 30s 心跳更新——模型常驻时覆盖 90% 场景——ping 零成本
	if s.store != nil {
		// 本机角色改名（3a，2026-09-16 Mr2109 拍：所有可推理的计算机都是子端）：
		// 原先查 "local"（localback 不经心跳上报）⇒ 现在本机 = 名字叫 **Mr2109** 的普通子端，
		// 它的快照经心跳上报，键就是 "Mr2109"。⚠ 该分支是**承重的**
		// （TestPingModel_SnapshotFastPath 钉着它：命中即跳过 ping）⇒ 所以是改名、不是删除。
		if lite := s.store.MachineSnapshot("Mr2109"); lite != nil {
			if lite.Healthy && lite.Model != "" && modelFileLoadedLite(lite, model) {
				s.cachePing(model, true, nil)
				return true, nil
			}
		}
		if lite := s.store.MachineSnapshot("x3"); lite != nil {
			if lite.Healthy && lite.Model != "" && modelFileLoadedLite(lite, model) {
				s.cachePing(model, true, nil)
				return true, nil
			}
		}
	}

	gatewayURL := statepath.GatewayBaseURL()
	token := config.ResolveAuthToken()
	for _, env := range s.agentEnv {
		if strings.HasPrefix(env, "ZERG_GATEWAY_URL=") {
			gatewayURL = strings.TrimPrefix(env, "ZERG_GATEWAY_URL=")
		}
		if strings.HasPrefix(env, "ZERG_AUTH_TOKEN=") {
			token = strings.TrimPrefix(env, "ZERG_AUTH_TOKEN=")
		}
	}
	body := map[string]interface{}{
		"model": model,
		"input": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"stream":            false,
		"max_output_tokens": 1,
	}
	data, _ := json.Marshal(body)
	req, err := httpNewRequest("POST", gatewayURL+"/v1/responses", strings.NewReader(string(data)))
	if err != nil {
		s.cachePing(model, false, err)
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// —— 第2级: 响应头探测（~1-3s——只等响应头不等 body——效率最高）——
	// 响应头到达 = 链路通 + 模型接单（后端开始干活）——不需要等推理完
	// 关键: 用 ResponseHeaderTimeout（等首字节）而非 Timeout（等完整响应）——
	// Qwen3.8 思考模型完整推理 16s+ 但响应头秒级就到——1s 判定 vs 16s 判定
	headerClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil,
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: 3 * time.Second, // 3s 等响应头——链路通=通过
		},
	}
	resp, err := headerClient.Do(req)
	if err != nil {
		// 响应头超时——可能连接问题/模型卡死——判故障（挂起等恢复）
		err = fmt.Errorf("[upstream_fail] ping failed (response-header timeout / connection failure): %w", err)
		s.cachePing(model, false, err)
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		perr := classifyGatewayError(resp.StatusCode, "")
		s.cachePing(model, false, perr)
		return false, perr
	}
	// 响应头 200 到了 = 链路通 + 模型接单——通过（不等 body——推理由任务本身完成）
	s.cachePing(model, true, nil)
	return true, nil
}

// modelFileLoadedLite 快照精简版已加载匹配（复用文件名校验逻辑——FleetSnapshotLite 版）
func modelFileLoadedLite(lite *FleetSnapshotLite, model string) bool {
	if lite == nil || lite.Model == "" {
		return false
	}
	// 快照 Model 可能文件名形式（Qwen3.8-27B-Q4_K_M-vcruz305）或逻辑名（Qwen3.8-27B）
	cur := lite.Model
	if strings.HasSuffix(cur, ".gguf") {
		cur = strings.TrimSuffix(cur, ".gguf")
	}
	if idx := strings.LastIndex(cur, "/"); idx >= 0 {
		cur = cur[idx+1:]
	}
	// 文件名形式含模型名 → 逻辑名匹配（前缀）
	if cur == model || strings.HasPrefix(cur, model) {
		return true
	}
	// 逻辑名形式（可能带量化后缀差异）——精确匹配兜底
	if strings.HasPrefix(model, cur) || strings.Contains(cur, model) {
		return true
	}
	return false
}

// pingResult 探活缓存项
type pingResult struct {
	ok  bool
	err error
	at  time.Time
}

// cachePing 写探活缓存
func (s *MasterScheduler) cachePing(model string, ok bool, err error) {
	s.pingMu.Lock()
	defer s.pingMu.Unlock()
	if s.pingCache == nil {
		s.pingCache = map[string]pingResult{}
	}
	s.pingCache[model] = pingResult{ok: ok, err: err, at: time.Now()}
}

// ========== 网关错误分类（见上——classifyGatewayError 等） ==========

// GatewayErrorCode 网关错误分类码（错误信息前缀——执行器/调度器据此分类处理）
// 错误信息格式: "[circuit_open] 详细消息"——调度器判断可等/可重试/不可重试
type GatewayErrorCode string

const (
	// CircuitOpen 熔断/无候选/被排除——环境故障——可等（waiting_retry——机器恢复自动重派）
	CircuitOpen GatewayErrorCode = "circuit_open"
	// UpstreamFail 转发失败——可重试（换机/换模型）
	UpstreamFail GatewayErrorCode = "upstream_fail"
	// Busy 排队中——可等（稍后重试）
	Busy GatewayErrorCode = "busy"
	// BadRequest 参数错——不可重试（修正后重发）
	BadRequest GatewayErrorCode = "bad_request"
	// ModelNotFound 模型不存在——不可重试（纠正模型名）
	ModelNotFound GatewayErrorCode = "model_not_found"
)

// classifyGatewayError 解析网关错误响应（{"error":{"type":"code",...}}）→ 分类错误
// 返回带 [code] 前缀的错误——调度器按前缀分类（isEnvFault/isRetryable）
func classifyGatewayError(status int, body string) error {
	// 优先解析 JSON 里的 error.type（网关 TransformError 已带）
	code := ""
	var e struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(body), &e) == nil && e.Error.Type != "" {
		code = e.Error.Type
	}
	// 解析不到 JSON code——按 HTTP 状态码兜底
	if code == "" {
		switch {
		case status == http.StatusServiceUnavailable:
			code = string(CircuitOpen)
		case status == http.StatusTooManyRequests:
			code = string(Busy)
		case status == http.StatusBadRequest:
			code = string(BadRequest)
		case status == http.StatusNotFound:
			code = string(ModelNotFound)
		default:
			code = string(UpstreamFail)
		}
	}
	// 归一化（网关可能返回 "not_found_error" 等旧 code——映射到新分类）
	switch code {
	case "not_found_error":
		code = string(ModelNotFound)
	case "api_error":
		code = string(UpstreamFail)
	}
	msg := e.Error.Message
	if msg == "" {
		msg = truncateStr(body, 200)
	}
	return fmt.Errorf("[%s] %s", code, msg)
}

// isCircuitOpen 是否熔断类错误（环境故障——可等——waiting_retry）
func isCircuitOpen(err error) bool {
	return err != nil && strings.Contains(err.Error(), "["+string(CircuitOpen)+"]")
}

// isEnvFault 是否环境故障（熔断/转发失败/排队——可等或可重试——不消耗任务重试次数）
func isEnvFault(err error) bool {
	if err == nil {
		return false
	}
	for _, c := range []GatewayErrorCode{CircuitOpen, UpstreamFail, Busy} {
		if strings.Contains(err.Error(), "["+string(c)+"]") {
			return true
		}
	}
	return false
}

// isRetryable 是否可重试（转发失败/排队——换机换模型重试有意义）
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "["+string(UpstreamFail)+"]") ||
		strings.Contains(err.Error(), "["+string(Busy)+"]")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// runZergFlow 调度器方法——新流程任务（Flow=zerg）——状态机驱动
// 注入模型调用（走网关——请求模型）——跑全流程——更新任务状态
// v2.5.6 故障自愈（Mr2109 2026-08-28）: 派发前先探活——模型不可用则不派发（挂起等恢复）
func (s *MasterScheduler) runZergFlow(task *Task) {
	// 0. 模型探活（ping——出发前查车况——坏模型不派发）
	ok, perr := s.pingModel(task.Model)
	if !ok {
		// 模型不可用——环境故障——挂起等恢复（不消耗任务重试次数）
		s.finishTask(task, fmt.Errorf("[%s] model liveness probe failed (task not started — auto re-dispatch after environment recovery): %v", string(CircuitOpen), perr))
		return
	}
	// 模型调用注入（走网关——HTTP 调模型——JSON 输出）
	callModel := func(systemPrompt, userPrompt string, maxTokens int) (string, error) {
		return s.callGatewayModel(task.Model, systemPrompt, userPrompt, maxTokens)
	}
	s.taskDirForCall = filepath.Join(statepath.TaskRoot(), sanitizeID(task.ID))                          // v2.5.6: write_file 写任务目录
	executor, err := NewZergFlowExecutor(task, callModel, filepath.Join(statepath.TaskRoot(), "skills")) // 待修补 #36: 技能库也随任务目录根（不写死）
	if err != nil {
		s.finishTask(task, fmt.Errorf("failed to create flow executor: %w", err))
		return
	}
	if err := executor.Run(); err != nil {
		// v2.5.6 候选机制（Mr2109 2026-08-28）: 本次用了候选（试跑）且任务失败——
		// 候选作废（删候选——保留正式版——下次任务用回正式版）
		if executor.Skill.UsingCandidate {
			_ = executor.Skill.DiscardCandidate(task.SkillKey, executor.TaskType)
			log.Printf("🧬 task %s candidate skill trial failed — discarded (keeping official — next task uses official)", task.ID)
		}
		s.finishTask(task, err)
		return
	}
	// 完成（v2.5.6 修复: 统一走 finishTask——更新状态 + 派发下一个任务）
	s.finishTask(task, nil)
	log.Printf("✅ scheduler: task %s program-driven run completed (git tree intact)", task.ID)
}

// callGatewayModel 调度器调模型（网关 HTTP——v1/responses——JSON 输出）
// 从 agentEnv 取网关地址/token（CA 环境变量——复用）——找不到用默认
// v2.5.6 工具支持: userPrompt 以 "TOOL:" 前缀开头时——请求带 submit_plan 工具——解析 tool_calls
// v2.5.6 参数来源（Mr2109 2026-08-28）: max_tokens 等参数由模型适配器决定——
// 调度器不硬编码（maxTokens 参数传 0 = 不带 max_output_tokens——网关层适配器覆盖成适配器声明值）
// 适配器 = 参数唯一来源——程序（含执行任务每步）调用参数由适配器决定
func (s *MasterScheduler) callGatewayModel(model, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	gatewayURL := statepath.GatewayBaseURL()
	token := config.ResolveAuthToken() // 默认网关 token（虫族标准）
	for _, env := range s.agentEnv {
		if strings.HasPrefix(env, "ZERG_GATEWAY_URL=") {
			gatewayURL = strings.TrimPrefix(env, "ZERG_GATEWAY_URL=")
		}
		if strings.HasPrefix(env, "ZERG_AUTH_TOKEN=") {
			token = strings.TrimPrefix(env, "ZERG_AUTH_TOKEN=")
		}
	}
	// 工具模式（方案轮/选定轮——模型调工具提交结构化输出）
	// v2.5.6 修复: TOOL: 前缀只是触发标记——userPrompt 内容完整（不剥掉——模型需要完整任务描述）
	useTools := strings.HasPrefix(userPrompt, "TOOL:")
	if useTools {
		userPrompt = strings.TrimPrefix(userPrompt, "TOOL:") + "\n（请调用工具提交方案/执行——不要只输出文字）"
	}
	body := map[string]interface{}{
		"model": model,
		"input": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream": false,
	}
	// v2.5.6 参数来源（Mr2109 2026-08-28）: max_tokens 由适配器决定——
	// maxTokens>0 才带（兼容旧调用/无适配器模型）——否则不带（网关适配器覆盖成声明值）
	if maxTokens > 0 {
		body["max_output_tokens"] = maxTokens
	}
	if useTools {
		body["tools"] = []map[string]interface{}{
			{
				"type":        "function",
				"name":        "submit_plan",
				"description": "提交执行方案（1-3 个）",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"plans": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"id":       map[string]interface{}{"type": "string"},
									"title":    map[string]interface{}{"type": "string"},
									"steps":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
									"approach": map[string]interface{}{"type": "string"},
								},
							},
						},
					},
				},
			},
			{
				"type":        "function",
				"name":        "submit_select",
				"description": "选定一个执行方案",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"selected":      map[string]interface{}{"type": "string", "description": "选定的方案 id（A/B/C）"},
						"reason":        map[string]interface{}{"type": "string", "description": "选择理由"},
						"refined_steps": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "细化步骤（执行轮的小任务列表）"},
					},
					"required": []string{"selected", "reason", "refined_steps"},
				},
			},
			{
				"type":        "function",
				"name":        "write_file",
				"description": "在工作区写文件（执行轮——模型实际产出）",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path":    map[string]interface{}{"type": "string", "description": "文件路径（任务工作区相对路径）"},
						"content": map[string]interface{}{"type": "string", "description": "文件内容"},
					},
					"required": []string{"path", "content"},
				},
			},
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := httpNewRequest("POST", gatewayURL+"/v1/responses", strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gateway call failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := ioReadAll(resp.Body)
	if resp.StatusCode != 200 {
		// v2.5.6 故障自愈（Mr2109 2026-08-28）: 解析网关错误码 → 分类错误
		// 调度器按 code 决定: 可等（circuit_open）→ waiting_retry / 可重试（upstream_fail）→ 换机换模型
		// 不可重试（bad_request/model_not_found）→ 任务失败带根因
		return "", classifyGatewayError(resp.StatusCode, string(respBody))
	}
	// 解析 responses 输出（output[0].content[0].text / function_call arguments / output[0].text）
	var r struct {
		Output []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			Text      string `json:"text"`
			Type      string `json:"type"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"output"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return "", fmt.Errorf("failed to parse gateway response: %w", err)
	}
	if len(r.Output) == 0 {
		return "", fmt.Errorf("gateway response had no output")
	}
	// 优先: function_call（工具模式——模型调工具）
	for _, o := range r.Output {
		if o.Type == "function_call" || o.Name != "" {
			if o.Arguments == "" {
				continue
			}
			// v2.5.6: write_file 工具——程序执行写文件（执行轮——模型实际产出）
			if o.Name == "write_file" {
				var wf struct {
					Path    string `json:"path"`
					Content string `json:"content"`
				}
				if err := json.Unmarshal([]byte(o.Arguments), &wf); err == nil && wf.Path != "" {
					// 写到任务工作区（相对路径——任务目录）
					writePath := filepath.Join(s.taskDirForCall, wf.Path)
					if err := os.MkdirAll(filepath.Dir(writePath), 0o755); err == nil {
						_ = os.WriteFile(writePath, []byte(wf.Content), 0o644)
						return "文件已写: " + wf.Path, nil
					}
				}
				return "write_file 执行失败: " + o.Arguments, nil
			}
			return o.Arguments, nil
		}
	}
	// 其次: content 文本（v2.5.6 修复: 跳过 reasoning 思考项——Qwen3.8/ornith 的 output[0] 是思考——
	// 取第一个非 reasoning 的最终答案——否则 taskType/执行输出被思考文本污染）
	for _, o := range r.Output {
		if o.Type == "reasoning" {
			continue
		}
		if len(o.Content) > 0 && strings.TrimSpace(o.Content[0].Text) != "" {
			return o.Content[0].Text, nil
		}
		if strings.TrimSpace(o.Text) != "" {
			return o.Text, nil
		}
	}
	return "", fmt.Errorf("gateway response had no usable text (reasoning only, no answer)")
}

// taskDirForTask zerg 流程任务目录（/tmp/zerg-tasks/<sanitizeID>——统一规则）
func taskDirForTask(task *Task) string {
	return filepath.Join(statepath.TaskRoot(), sanitizeID(task.ID))
}

// finishTask 任务收尾（失败/完成——更新状态——调度器统一出口）
// v2.5.6 故障自愈（Mr2109 2026-08-28）: 环境故障 ≠ 任务失败——
//
//	circuit_open/busy（熔断/排队/资源）→ waiting_retry（挂起——机器恢复自动重派——不消耗重试次数）
//	upstream_fail（转发失败）→ 同 waiting_retry（可重试——恢复后重派）
//	bad_request/model_not_found（参数/配置）→ failed（不可重试——带根因）
//	REBUILD/其他任务逻辑错误 → failed（任务自身失败——重试 3 次后查因）
func (s *MasterScheduler) finishTask(task *Task, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, task.ID)
	task.CompletedAt = time.Now()
	if err != nil {
		if isEnvFault(err) {
			// 环境故障——挂起等恢复（不标 failed——不消耗重试次数——任务永远在队列里等机会）
			task.Status = "waiting_retry"
			task.FailReason = "环境故障（" + err.Error() + "）——等待机器恢复自动重派"
			log.Printf("⏳ scheduler: task %s suspended on environment failure (waiting_retry): %v", task.ID, err)
			s.waiting[task.ID] = task
			// 持久化（重启不丢——恢复调度器继续处理）
			saveTasksLocked(s.queue, s.running, s.history, s.waiting)
			s.dispatchLocked()
			return
		}
		task.Status = "failed"
		task.FailReason = err.Error()
		s.history[task.ID] = task
		log.Printf("❌ scheduler: task %s program-driven run failed: %v", task.ID, err)
	} else {
		// v2.5.6 三层复查（Mr2109 2026-08-29——zerg 流程补复查——之前直接 done 无复查）
		// ① 程序确定性验证（机器复查）: git 有真实改动 + 报告非空（>100 字节）——不过 → failed/重跑
		reportPath := FindTaskReport(task.Workdir)
		if reportPath == "" {
			// zerg 流程报告在任务目录
			reportPath = filepath.Join(taskDirForTask(task), "internal-task-report.md")
		}
		verifyDir := taskDirForTask(task)
		if vres := VerifyTaskOutput(verifyDir, reportPath, task.Description); !vres.Pass {
			task.Status = "failed"
			task.FailReason = "确定性验证不过: " + strings.Join(vres.Failures, "; ")
			s.history[task.ID] = task
			log.Printf("❌ scheduler: task %s failed deterministic validation (fake-completion blocked): %s", task.ID, strings.Join(vres.Failures, "; "))
		} else {
			log.Printf("✅ scheduler: task %s deterministic validation passed (%d checks) — sending to review", task.ID, len(vres.Checks))
			// ② 跨家族模型复查（模型复查）——复查任务完成时 handleReviewDoneLocked 决策
			// 执行任务状态: 待复查（复查通过才 done——Mr2109: 成功与否由复查模型决定）
			task.Status = "reviewing"
			s.history[task.ID] = task
			s.submitReviewTaskLocked(task, reportPath, "")
		}
	}
	// v2.5.6 修复: 收尾后继续派发（下一个任务）
	s.dispatchLocked()
}

// HTTP 辅助
func httpNewRequest(method, url string, body *strings.Reader) (*http.Request, error) {
	return http.NewRequest(method, url, body)
}

func httpClient() *http.Client {
	// v2.5.6 修复: max_tokens 131072 后 ornith 生成慢——超时 120s→600s（完整输出不掐断）
	return &http.Client{Timeout: 600 * time.Second}
}

func ioReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
