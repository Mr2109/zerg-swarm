package subtask

// scheduler.go — SubtaskScheduler 阶段调度器（2026-09-05 设计 S2——结晶模式主循环）
// 拆解轮(强模型) → for 每阶段: 依赖检查→loopcore执行→契约核验→结晶 → 任务收尾
// 分相计量（G6）: decompose/execute/crystallize 三列 token
// Mr2109可观测（G7）: Stages() 返回阶段进度卡（任务详情 API 渲染）

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ModelCall — 模型调用抽象（调用方适配——网关/直连）
// 返回: 回复文本 + 输入 token + 输出 token
type ModelCall func(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (text string, inTok, outTok int64, err error)

// PhaseRunner — 单阶段执行抽象（调用方适配——内部走 loopcore.Run + CA 工具集）
// 返回: 阶段正文（最终回复/摘要）+ 总 token + 轮数
type PhaseRunner func(ctx context.Context, step Step, seedMsgs []map[string]any) (body string, tokens int64, rounds int, err error)

// Config — 调度配置（R7 停止条件参数化）
type Config struct {
	WorktreeDir    string // worktree（契约核验用）
	Workdir        string // 工作区
	TaskDir        string // 任务目录（S8——断点数据+UI stages 数据源）
	PlannerModel   string // 拆解模型（DS4 默认）
	ExecutorModel  string // 执行模型
	MaxReplan      int    // Replan 上限（默认 2）
	MaxDecompose   int    // 拆解校验重试上限（默认 2）
	CrystalMaxTok  int    // 结晶链膨胀阈值（默认 6000——token 估算）
	VerifyHighRisk bool   // 高险步骤抽查 LLM verify（R6——预留）
}

// StageView — 阶段进度卡（G7 UI 渲染）
type StageView struct {
	ID       string `json:"id"`
	Goal     string `json:"goal"`
	Status   string `json:"status"` // pending/running/done/partial/blocked
	Contract string `json:"contract_summary"`
	Crystal  string `json:"crystal_summary,omitempty"`
}

// Scheduler — 子任务调度器
type Scheduler struct {
	cfg      Config
	plan     *Plan
	crystals map[string]*Crystal // stepID → 结晶
	stages   map[string]string   // stepID → status（G7）
	call     ModelCall
	runner   PhaseRunner
	mu       sync.Mutex
	// 分相计量（G6）
	DecomposeTokens   int64
	ExecuteTokens     int64
	CrystallizeTokens int64
	Rounds            int
	// S6 续作
	ResumedSkipped []string // 断点恢复跳过的阶段（审计）
	Resumed        bool     // 本次 Run 是否为续作
}

// SetPlan — S6: 注入已持久化的 plan（续作不重拆）
func (s *Scheduler) SetPlan(p *Plan) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plan = p
}

// Resume — S6 续作模式: 注入断点数据（已恢复的结晶+plan）——Run 跳过已 done 阶段
func (s *Scheduler) Resume(crystals map[string]*Crystal) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, c := range crystals {
		s.crystals[id] = c
	}
	s.Resumed = true
}

// NewScheduler — 创建（plan 可 nil=由拆解轮生成）
func NewScheduler(cfg Config, call ModelCall, runner PhaseRunner, plan *Plan) *Scheduler {
	// 默认停止条件（R7 参数化——零值兜底）
	if cfg.MaxReplan <= 0 {
		cfg.MaxReplan = 2
	}
	if cfg.MaxDecompose <= 0 {
		cfg.MaxDecompose = 2
	}
	if cfg.CrystalMaxTok <= 0 {
		cfg.CrystalMaxTok = 6000
	}
	s := &Scheduler{cfg: cfg, call: call, runner: runner, crystals: map[string]*Crystal{}, stages: map[string]string{}}
	if plan != nil {
		s.plan = plan
		for _, st := range plan.Steps {
			s.stages[st.ID] = "pending"
		}
	}
	return s
}

// Stages — 阶段进度卡（G7）
func (s *Scheduler) Stages() []StageView {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []StageView
	if s.plan == nil {
		return out
	}
	for _, st := range s.plan.Steps {
		v := StageView{ID: st.ID, Goal: st.Goal, Status: s.stages[st.ID]}
		if st.Contract != nil {
			if len(st.Contract.MustPassCmds) > 0 {
				v.Contract = "cmds: " + fmt.Sprint(st.Contract.MustPassCmds)
			} else if len(st.Contract.MustWriteFiles) > 0 {
				v.Contract = "files: " + fmt.Sprint(st.Contract.MustWriteFiles)
			}
		}
		if c, ok := s.crystals[st.ID]; ok {
			v.Crystal = c.SummaryLine()
		}
		out = append(out, v)
	}
	return out
}

// EnsurePlan — 获取计划（nil 则拆解轮生成——含校验重试 G3/R9）
func (s *Scheduler) EnsurePlan(ctx context.Context, taskDesc string) (*Plan, []string, error) {
	if s.plan != nil {
		return s.plan, nil, nil
	}
	// S6 续作: 拆解是语义工作不重复——续作时调用方 LoadPlan 注入（无 plan 则报错）
	if s.Resumed {
		return nil, nil, fmt.Errorf("续作模式但 plan 未注入（LoadPlan 失败?）——拆解轮不重跑")
	}
	var lastStats string
	for attempt := 1; attempt <= s.cfg.MaxDecompose+1; attempt++ {
		out, inTok, outTok, err := s.call(ctx, "你是任务架构师——只输出 JSON。", DecomposePrompt(taskDesc, "", lastStats), 0)
		if err != nil {
			return nil, nil, fmt.Errorf("拆解模型调用失败: %w", err)
		}
		s.DecomposeTokens += inTok + outTok
		p, perr := ParsePlanJSON(out)
		if perr != nil {
			lastStats = fmt.Sprintf("第 %d 次拆解输出非合法 JSON（%v）——请严格按 schema 输出", attempt, perr)
			continue
		}
		if errs := p.Validate(taskDesc); len(errs) > 0 {
			// R9: 错误清单回喂（validate-loop 同构）
			lastStats = fmt.Sprintf("第 %d 次拆解校验未过:\n- %s", attempt, joinN(errs, "\n- ", 6))
			continue
		}
		s.mu.Lock()
		s.plan = p
		for _, st := range p.Steps {
			s.stages[st.ID] = "pending"
		}
		s.mu.Unlock()
		return p, nil, nil
	}
	return nil, nil, fmt.Errorf("拆解 %d 次未过校验——任务放弃（不烧执行 token）", s.cfg.MaxDecompose+1)
}

// Run — 主循环（设计 3.3 伪代码落地——G4/G5/G6 全接）
func (s *Scheduler) Run(ctx context.Context, taskDesc string) (*TaskOutcome, error) {
	plan, _, err := s.EnsurePlan(ctx, taskDesc)
	if err != nil {
		return &TaskOutcome{Status: "failed", Reason: err.Error()}, err
	}
	// plan.json 落盘（3.6 断点恢复数据源——S8 修: 双落盘 Workdir+TaskDir）
	_ = os.MkdirAll(s.cfg.Workdir, 0o755)
	_ = os.WriteFile(filepath.Join(s.cfg.Workdir, "plan.json"), []byte(plan.ToJSON()), 0o644)
	if s.cfg.TaskDir != "" {
		_ = SavePlan(s.cfg.TaskDir, plan)
	}

	start := time.Now()
	outcome := &TaskOutcome{Status: "done"}
	var crystalsJSON []byte

	for _, stepID := range plan.SortedStepIDs() {
		step := s.stepByID(stepID)
		// ⓪ S6 续作: 断点恢复——已 done 的阶段直接跳过（复查打回重做不重复劳动）
		if c, ok := s.crystals[stepID]; ok && c.ExitKind == ExitDone {
			s.setStatus(stepID, "done")
			s.ResumedSkipped = append(s.ResumedSkipped, stepID)
			continue
		}
		// ① 依赖检查（前置 done——G4 partial 不满足依赖→blocked）
		if !s.depsSatisfied(step) {
			s.setStatus(stepID, "blocked")
			outcome.Blocked = append(outcome.Blocked, stepID)
			s.crystallizeBlocked(step)
			continue
		}
		s.setStatus(stepID, "running")
		// ② 种子上下文 = 系统提示 + 结晶链(追加·末尾——R2 recitation) + 本步描述+契约
		seed := s.seedMessages(taskDesc, step)
		// ③ 执行（PhaseRunner——内部 loopcore）
		body, tokens, rounds, err := s.runner(ctx, step, seed)
		s.ExecuteTokens += tokens
		s.Rounds += rounds
		// ④ 契约核验（阶段级——程序判）
		vres := (&Contract{MustWriteFiles: step.Produces}).Verify(s.cfg.WorktreeDir, s.cfg.Workdir)
		if step.Contract != nil {
			vres = step.Contract.Verify(s.cfg.WorktreeDir, s.cfg.Workdir)
		}
		exitKind := ExitDone
		if !vres.Pass {
			switch {
			case step.Critical:
				// 关键步骤契约必须全过——不过=blocked（任务将终止）
				exitKind = ExitBlocked
			case err != nil || body == "":
				exitKind = ExitBlocked
			default:
				exitKind = ExitPartial // G4: 有产出未全过——进度保全
			}
		}
		// ⑤ 结晶（模型提取→核验→失败兜底——G5 所有 ExitKind 都结晶）
		crystal := s.crystallize(ctx, step, exitKind, vres, body, tokens, rounds)
		s.mu.Lock()
		s.crystals[stepID] = crystal
		if s.cfg.TaskDir != "" {
			_ = SaveCrystal(s.cfg.TaskDir, crystal) // S8: 结晶落任务目录（断点+UI 数据源）
		}
		s.setStatusLocked(stepID, string(crystal.ExitKind))
		crystalsJSON = appendCrystals(crystalsJSON, crystal)
		s.mu.Unlock()
		outcome.Tokens += crystal.Tokens
		// ⑥ 失败决策（R5 ADaPT 混合——失败先递归细分？一期简化: 关键失败=任务失败；非关键=partial 继续）
		if crystal.ExitKind == ExitBlocked {
			if step.Critical {
				outcome.Status = "failed"
				outcome.Reason = fmt.Sprintf("关键阶段 %s 失败（blocked）——任务终止", stepID)
				break
			}
			outcome.Partial = append(outcome.Partial, stepID)
		}
	}
	// 任务级收尾
	if outcome.Status == "done" {
		blocked := 0
		for _, st := range s.stages {
			if st == "blocked" {
				blocked++
			}
		}
		if blocked > len(plan.Steps)/2 {
			outcome.Status = "failed"
			outcome.Reason = fmt.Sprintf("blocked 阶段 %d > 半数——拆解质量差", blocked)
		}
	}
	outcome.DurationS = int(time.Since(start).Seconds())
	outcome.CrystalsJSON = string(crystalsJSON)
	return outcome, nil
}

// crystallize — 结晶（模型提取→Validate→失败 Fallback——G5/G11）
func (s *Scheduler) crystallize(ctx context.Context, step Step, exitKind CrystalExitKind, vres *VerifyResult, body string, tokens int64, rounds int) *Crystal {
	cr := &Crystal{StepID: step.ID, Goal: step.Goal, ExitKind: exitKind, Model: s.cfg.ExecutorModel, Tokens: tokens, Timestamp: float64(time.Now().Unix())}
	for _, c := range vres.Checks {
		cr.Results = append(cr.Results, c)
	}
	for _, f := range vres.Failures {
		cr.Results = append(cr.Results, "❌ "+f)
	}
	// 产物实盘（程序 glob——防编造）
	for _, f := range step.Produces {
		if _, err := os.Stat(filepath.Join(s.cfg.Workdir, f)); err == nil {
			cr.Artifacts = append(cr.Artifacts, f)
		}
	}
	// 模型提取（1 次——失败即 Fallback——G11 不重试烧 token）
	out, inTok, outTok, err := s.call(ctx, "你是阶段总结员——只输出 JSON。", CrystallizePrompt(step, string(exitKind), vres.Checks, body), 0)
	s.CrystallizeTokens += inTok + outTok
	if err == nil {
		var parsed struct {
			KeyData   []string `json:"key_data"`
			Lessons   []string `json:"lessons"`
			Influence string   `json:"influence"`
		}
		if jsonUnmarshalStrict(out, &parsed) == nil {
			cr.KeyData = parsed.KeyData
			cr.Lessons = parsed.Lessons
			cr.Influence = parsed.Influence
		}
	}
	if errs := cr.Validate(s.cfg.Workdir); len(errs) > 0 {
		// 兜底结晶（G5/G11——程序数据零模型）
		return Fallback(step, exitKind, cr.Results, s.cfg.Workdir, s.cfg.ExecutorModel, tokens, rounds)
	}
	return cr
}

// crystallizeBlocked — blocked 阶段直接程序兜底（依赖未满足——无执行可总结）
func (s *Scheduler) crystallizeBlocked(step Step) {
	cr := Fallback(step, ExitBlocked, []string{"依赖未满足——未执行"}, s.cfg.Workdir, s.cfg.ExecutorModel, 0, 0)
	s.mu.Lock()
	s.crystals[step.ID] = cr
	s.mu.Unlock()
	if s.cfg.TaskDir != "" {
		_ = SaveCrystal(s.cfg.TaskDir, cr)
	}
}

// seedMessages — 种子上下文（R2: 结晶链=追加消息+末尾 recitation——非改写系统提示）
func (s *Scheduler) seedMessages(taskDesc string, step Step) []map[string]any {
	msgs := []map[string]any{
		{"role": "system", "content": "你是虫族 CA——执行当前阶段的子任务。可用工具按需使用。"},
	}
	// 结晶链（按完成序——摘要行紧凑——G4 partial 也注入——后续可补完）
	var chain []string
	for _, id := range s.plan.SortedStepIDs() {
		if c, ok := s.crystals[id]; ok {
			chain = append(chain, c.SummaryLine())
		}
	}
	// 本步描述+契约（recitation——紧邻结尾）
	var cb strings.Builder
	cb.WriteString(fmt.Sprintf("【当前阶段 %s】%s\n", step.ID, step.Goal))
	if step.Contract != nil {
		if len(step.Contract.MustWriteFiles) > 0 {
			cb.WriteString(fmt.Sprintf("【本阶段验收——必须落盘】%v\n", step.Contract.MustWriteFiles))
		}
		if len(step.Contract.MustPassCmds) > 0 {
			cb.WriteString(fmt.Sprintf("【本阶段验收——必须通过】%v\n", step.Contract.MustPassCmds))
		}
	}
	msgs = append(msgs,
		map[string]any{"role": "user", "content": fmt.Sprintf("【任务背景】%s\n\n【已完成阶段结晶】\n%s", truncateRunes(taskDesc, 1500), strings.Join(chain, "\n"))},
		map[string]any{"role": "user", "content": cb.String()},
	)
	return msgs
}

func (s *Scheduler) stepByID(id string) Step {
	for _, st := range s.plan.Steps {
		if st.ID == id {
			return st
		}
	}
	return Step{}
}

func (s *Scheduler) depsSatisfied(step Step) bool {
	for _, dep := range step.Depends {
		if c, ok := s.crystals[dep]; !ok || c.ExitKind != ExitDone {
			return false
		}
	}
	return true
}

func (s *Scheduler) setStatus(id, st string) {
	s.mu.Lock()
	s.stages[id] = st
	s.mu.Unlock()
}

func (s *Scheduler) setStatusLocked(id, st string) {
	s.stages[id] = st
}

// TaskOutcome — 任务结果（G6 分相计量）
type TaskOutcome struct {
	Status       string   `json:"status"` // done/failed
	Reason       string   `json:"reason,omitempty"`
	Partial      []string `json:"partial,omitempty"`
	Blocked      []string `json:"blocked,omitempty"`
	Tokens       int64    `json:"tokens"`
	DurationS    int      `json:"duration_s"`
	CrystalsJSON string   `json:"crystals_json"`
	// 分相计量（G6）
	DecomposeTokens   int64 `json:"decompose_tokens"`
	ExecuteTokens     int64 `json:"execute_tokens"`
	CrystallizeTokens int64 `json:"crystallize_tokens"`
	Rounds            int   `json:"rounds"`
}

func appendCrystals(b []byte, c *Crystal) []byte {
	line, _ := json.Marshal(c)
	if len(b) > 0 {
		b = append(b, '\n')
	}
	return append(b, line...)
}

func jsonUnmarshalStrict(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
