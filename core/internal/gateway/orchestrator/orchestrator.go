// Package orchestrator 实现脑手编排器：将复合模型请求分解为脑（规划）+ 手（执行）子任务，
// 并行调度执行，最后汇总结果。
//
// 架构：
//
//	请求 → decompose（脑模型分析，拆成子任务）→ dispatch（goroutine 并行调用不同模型）→ synthesize（汇总）
//
// 对外注册复合模型名（如 'zerg-composite'），客户端无感。
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"zerg/core/internal/agentstate"
)

// Strategy 编排策略。
type Strategy int

const (
	StrategySimple   Strategy = iota // 顺序执行
	StrategyParallel                 // 并行执行
	StrategyHybrid                   // 混合执行
)

// Mode 编排模式。
type Mode int

const (
	ModeSequential Mode = iota // 顺序模式
	ModePipeline               // 流水线模式
)

// Task 子任务。
type Task struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	SubType      string   `json:"sub_type"`
	Description  string   `json:"description"`
	Prompt       string   `json:"prompt"`
	Result       string   `json:"result,omitempty"`
	Error        string   `json:"error,omitempty"`
	StartTime    time.Time `json:"start_time,omitempty"`
	EndTime      time.Time `json:"end_time,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Model        string   `json:"model,omitempty"`
}

// DecomposeResult 分解结果。
type DecomposeResult struct {
	OriginalRequest string   `json:"original_request"`
	Tasks           []Task   `json:"tasks"`
	Strategy        Strategy `json:"strategy"`
	Reasoning       string   `json:"reasoning,omitempty"`
}

// SynthesizeResult 汇总结果。
type SynthesizeResult struct {
	CombinedResult string                  `json:"combined_result"`
	Summary        string                  `json:"summary,omitempty"`
	TaskResults    map[string]TaskResult   `json:"task_results"`
	TotalTokens    int                     `json:"total_tokens"`
	TotalTime      time.Duration           `json:"total_time"`
}

// TaskResult 子任务执行结果。
type TaskResult struct {
	Content  string        `json:"content"`
	Duration time.Duration `json:"duration"`
	Tokens   int           `json:"tokens"`
	Success  bool          `json:"success"`
	Error    string        `json:"error,omitempty"`
}

// ModelResponse 模型完整响应。
type ModelResponse struct {
	Content   string
	Tokens    int
	Reasoning string
}

// ModelExecutor 模型执行器接口。
type ModelExecutor interface {
	Execute(ctx context.Context, model string, prompt string, maxTokens int) (string, error)
	ExecuteWithResponse(ctx context.Context, model string, prompt string, maxTokens int) (*ModelResponse, error)
}

// OrchestratorConfig 编排器配置。
type OrchestratorConfig struct {
	BrainModel     string        `json:"brain_model"`
	HandModel      string        `json:"hand_model"`
	Strategy       Strategy      `json:"strategy"`
	Mode           Mode          `json:"mode"`
	MaxTasks       int           `json:"max_tasks"`
	Timeout        time.Duration `json:"timeout"`
	EnableParallel bool          `json:"enable_parallel"`
	CompositeName  string        `json:"composite_name"`
}

// DefaultOrchestratorConfig 默认配置。
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		// 2026-08-12: Orn+Nanbeige 临时测试（脑 example-35b-v2 规划 / 手 nanbeige 执行）
		// Nanbeige 3B 已注册 X3（双机候选——路由自动分配）
		BrainModel:     "example-35b",
		HandModel:      "nanbeige-4.2-3b",
		Strategy:       StrategySimple,
		Mode:           ModeSequential,
		MaxTasks:       10,
		Timeout:        5 * time.Minute,
		EnableParallel: true,
		CompositeName:  "zerg-baiyan",
	}
}

// Orchestrator 脑手编排器。
type Orchestrator struct {
	config   *OrchestratorConfig
	executor ModelExecutor
	mu       sync.RWMutex
	stateDir string // M2 harness_state 状态目录（v2.4——任务状态持久化）
}

// NewOrchestrator 创建编排器。
func NewOrchestrator(cfg *OrchestratorConfig, executor ModelExecutor) *Orchestrator {
	if cfg == nil {
		cfg = DefaultOrchestratorConfig()
	}
	return &Orchestrator{config: cfg, executor: executor, stateDir: "<repo>/.zerg/states"}
}

// Executor 返回执行器（网关接线用——设置认证等）。
func (o *Orchestrator) Executor() ModelExecutor {
	return o.executor
}

// Execute 执行复合模型请求（白眼 zerg-baiyan——MoA：并行参考 + 聚合提炼）。
func (o *Orchestrator) Execute(ctx context.Context, request string, maxTokens int) (*SynthesizeResult, error) {
	startTime := time.Now()
	log.Printf("[orchestrator] 白眼开始执行 (%d 字符)", len(request))

	// M2 harness_state 接入（v2.4）：任务状态记录——供断连恢复/M4 蒸馏
	statePath := filepath.Join(o.stateDir, fmt.Sprintf("moa_%d.json", time.Now().Unix()))
	hs := agentstate.NewState("MoA 执行: "+truncate(request, 100), "zerg-baiyan", []agentstate.Todo{
		{ID: "todo_refs", Text: "参考模型并行调用", Priority: "P0", Status: agentstate.TodoInProgress},
		{ID: "todo_agg", Text: "example-35b-v2 聚合", Priority: "P0", Status: agentstate.TodoOpen},
	})
	hs.AddEvidence("开始", "FanOutMoA 并行参考", "参考可能失败(重试2)", "聚合")
	_ = hs.Save(statePath)

	// Step 1: Fan-out（参考模型并行——gemma + Qwable 无工具纯文本）
	refs := o.FanOutMoA(ctx, request)
	hs.SetTodoStatus("todo_refs", agentstate.TodoCompleted)
	hs.AddEvidence("参考完成", fmt.Sprintf("%d 个参考(成功%d)", len(refs), countSuccess(refs)), "参考质量未验证", "聚合")
	_ = hs.Save(statePath)

	// Step 2: Aggregate（example-35b-v2 = 主模型——带工具，最终决策）
	aggCtx, aggCancel := context.WithTimeout(ctx, o.config.Timeout)
	defer aggCancel()
	aggResp, err := o.AggregateMoA(aggCtx, request, refs, maxTokens)
	if err != nil {
		// 聚合失败 → best-effort：返回最成功参考输出（不空手）
		log.Printf("[orchestrator] 聚合失败: %v——返回最成功参考", err)
		hs.SetTodoStatus("todo_agg", agentstate.TodoBlocked)
		hs.AddEvidence("聚合失败", err.Error(), "参考输出可用", "重试或降级")
		hs.Status = "failed"
		_ = hs.Save(statePath)
		for _, r := range refs {
			if r.Success {
				return &SynthesizeResult{
					CombinedResult: r.Output,
					Summary:        fmt.Sprintf("聚合失败——返回参考 %s 输出", r.Model),
					TotalTime:      time.Since(startTime),
				}, nil
			}
		}
		return nil, fmt.Errorf("aggregate 失败（参考全失败）: %w", err)
	}

	result := &SynthesizeResult{
		CombinedResult: aggResp.Content,
		Summary:        fmt.Sprintf("白眼 MoA: %d 参考 → example-35b-v2 聚合", len(refs)),
		TotalTokens:    aggResp.Tokens,
		TotalTime:      time.Since(startTime),
	}
	hs.SetTodoStatus("todo_agg", agentstate.TodoCompleted)
	hs.AddEvidence("聚合完成", fmt.Sprintf("耗时 %v tokens %d", result.TotalTime, result.TotalTokens), "无", "归档")
	hs.Status = "completed"
	_ = hs.Save(statePath)
	log.Printf("[orchestrator] 白眼完成: 参考 %d 个, 总耗时 %v", len(refs), result.TotalTime)
	return result, nil
}

// truncate 截断字符串（状态文件 objective 用）
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// countSuccess 统计成功参考数
func countSuccess(refs []MoAReference) int {
	n := 0
	for _, r := range refs {
		if r.Success {
			n++
		}
	}
	return n
}

func (o *Orchestrator) decompose(ctx context.Context, request string) (*DecomposeResult, error) {
	decomposePrompt := fmt.Sprintf(`你是一个任务分解专家。请将以下用户请求分解为可执行的子任务。

用户请求:
%s

要求:
1. 输出 JSON 数组，每个元素包含:
   - "id": 唯一 ID（如 "task_1"）
   - "type": "brain" 或 "hand"
   - "sub_type": 子任务类型（如 "plan", "execute", "review"）
   - "description": 任务描述
   - "prompt": 发送给模型的完整 prompt
2. 任务数量不超过 %d 个。
3. 优先使用 hand 类型任务执行具体操作，brain 类型用于规划。
4. 如果请求本身很简单，可以只生成 1-2 个 hand 任务。
5. 输出纯 JSON 数组，不要包含其他内容。`, request, o.config.MaxTasks)

	response, err := o.executor.ExecuteWithResponse(ctx, o.config.BrainModel, decomposePrompt, 2000)
	if err != nil {
		return nil, fmt.Errorf("脑模型分解调用失败: %w", err)
	}

	tasks := o.parseDecomposeResponse(response.Content)
	if tasks == nil {
		log.Printf("[orchestrator] 脑模型输出解析失败，降级为单任务")
		return &DecomposeResult{
			OriginalRequest: request,
			Tasks: []Task{
				{
					ID:          "task_1",
					Type:        "hand",
					SubType:     "execute",
					Description: request,
					Prompt:      request,
				},
			},
			Strategy:  StrategySimple,
			Reasoning: response.Reasoning,
		}, nil
	}

	if len(tasks) > o.config.MaxTasks {
		tasks = tasks[:o.config.MaxTasks]
	}

	for i := range tasks {
		if tasks[i].ID == "" {
			tasks[i].ID = fmt.Sprintf("task_%d", i+1)
		}
		if tasks[i].Type == "" {
			tasks[i].Type = "hand"
		}
		if tasks[i].Prompt == "" {
			tasks[i].Prompt = tasks[i].Description
		}
	}

	strategy := o.determineStrategy(request, tasks)

	return &DecomposeResult{
		OriginalRequest: request,
		Tasks:           tasks,
		Strategy:        strategy,
		Reasoning:       response.Reasoning,
	}, nil
}

func (o *Orchestrator) parseDecomposeResponse(content string) []Task {
	var tasks []Task
	if err := json.Unmarshal([]byte(content), &tasks); err == nil {
		return tasks
	}

	startIdx := strings.Index(content, "[")
	endIdx := strings.LastIndex(content, "]")
	if startIdx >= 0 && endIdx > startIdx {
		if err := json.Unmarshal([]byte(content[startIdx:endIdx+1]), &tasks); err == nil {
			return tasks
		}
	}

	codeBlockStart := strings.Index(content, "```")
	if codeBlockStart >= 0 {
		codeBlockEnd := strings.Index(content[codeBlockStart:], "\n```")
		if codeBlockEnd > 0 {
			jsonStr := strings.TrimSpace(content[codeBlockStart+3 : codeBlockStart+codeBlockEnd])
			if err := json.Unmarshal([]byte(jsonStr), &tasks); err == nil {
				return tasks
			}
		}
	}

	return nil
}

func (o *Orchestrator) dispatch(ctx context.Context, tasks []Task) (map[string]TaskResult, error) {
	switch o.config.Strategy {
	case StrategyParallel:
		return o.dispatchParallel(ctx, tasks)
	case StrategyHybrid:
		return o.dispatchHybrid(ctx, tasks)
	default:
		return o.dispatchSimple(ctx, tasks)
	}
}

func (o *Orchestrator) dispatchSimple(ctx context.Context, tasks []Task) (map[string]TaskResult, error) {
	results := make(map[string]TaskResult)
	for _, task := range tasks {
		result, err := o.executeTask(ctx, task)
		if err != nil {
			results[task.ID] = TaskResult{
				Content: "", Duration: 0, Tokens: 0, Success: false, Error: err.Error(),
			}
			continue
		}
		results[task.ID] = result
	}
	return results, nil
}

func (o *Orchestrator) dispatchParallel(ctx context.Context, tasks []Task) (map[string]TaskResult, error) {
	results := make(map[string]TaskResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			result, err := o.executeTask(ctx, t)
			if err != nil {
				log.Printf("[orchestrator] 并行任务 %s 执行失败: %v", t.ID, err)
				mu.Lock()
				results[t.ID] = TaskResult{
					Content: "", Duration: 0, Tokens: 0, Success: false, Error: err.Error(),
				}
				mu.Unlock()
			} else {
				mu.Lock()
				results[t.ID] = result
				mu.Unlock()
			}
		}(task)
	}

	wg.Wait()
	return results, nil
}

func (o *Orchestrator) dispatchHybrid(ctx context.Context, tasks []Task) (map[string]TaskResult, error) {
	var independentTasks, dependentTasks []Task
	for _, task := range tasks {
		if len(task.Dependencies) == 0 {
			independentTasks = append(independentTasks, task)
		} else {
			dependentTasks = append(dependentTasks, task)
		}
	}

	results := make(map[string]TaskResult)

	if len(independentTasks) > 0 {
		parResults, err := o.dispatchParallel(ctx, independentTasks)
		if err != nil {
			return nil, err
		}
		for k, v := range parResults {
			results[k] = v
		}
	}

	for _, task := range dependentTasks {
		allMet := true
		for _, depID := range task.Dependencies {
			if _, ok := results[depID]; !ok {
				allMet = false
				break
			}
		}
		if !allMet {
			results[task.ID] = TaskResult{
				Content: "", Duration: 0, Tokens: 0, Success: false, Error: "依赖任务未完成",
			}
			continue
		}
		result, err := o.executeTask(ctx, task)
		if err != nil {
			results[task.ID] = TaskResult{
				Content: "", Duration: 0, Tokens: 0, Success: false, Error: err.Error(),
			}
		} else {
			results[task.ID] = result
		}
	}

	return results, nil
}

func (o *Orchestrator) executeTask(ctx context.Context, task Task) (TaskResult, error) {
	startTime := time.Now()
	task.StartTime = startTime

	model := o.config.HandModel
	if task.Type == "brain" {
		model = o.config.BrainModel
	}

	response, err := o.executor.ExecuteWithResponse(ctx, model, task.Prompt, 4000)
	if err != nil {
		return TaskResult{
			Content: "", Duration: time.Since(startTime), Tokens: 0, Success: false, Error: err.Error(),
		}, err
	}

	task.EndTime = time.Now()

	return TaskResult{
		Content:  response.Content,
		Duration: time.Since(startTime),
		Tokens:   response.Tokens,
		Success:  true,
	}, nil
}

func (o *Orchestrator) synthesize(originalRequest string, taskResults map[string]TaskResult, startTime time.Time) (*SynthesizeResult, error) {
	var combinedContent strings.Builder
	totalTokens := 0
	successCount := 0

	for _, taskResult := range taskResults {
		if taskResult.Success {
			combinedContent.WriteString(taskResult.Content)
			combinedContent.WriteString("\n\n---\n\n")
			totalTokens += taskResult.Tokens
			successCount++
		}
	}

	summary := fmt.Sprintf("复合模型执行完成: %d/%d 任务成功, 总 token: %d, 总耗时: %v",
		successCount, len(taskResults), totalTokens, time.Since(startTime))

	return &SynthesizeResult{
		CombinedResult: combinedContent.String(),
		Summary:        summary,
		TaskResults:    taskResults,
		TotalTokens:    totalTokens,
		TotalTime:      time.Since(startTime),
	}, nil
}

func (o *Orchestrator) determineStrategy(request string, tasks []Task) Strategy {
	if strings.Contains(request, "并行") || strings.Contains(request, "同时") || strings.Contains(request, "同时执行") {
		if o.config.EnableParallel {
			return StrategyParallel
		}
	}
	if len(tasks) > 3 && o.config.EnableParallel {
		return StrategyParallel
	}
	return StrategySimple
}
