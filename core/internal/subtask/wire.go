package subtask

// wire.go — S3/S4: 从任务装配调度器 + crystallization.jsonl 持久化/断点恢复（2026-09-05）

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TaskSpec — 任务输入（调用方从 api.Task 提取——避免 import cycle）
type TaskSpec struct {
	ID            string
	Description   string
	PlannerModel  string // 拆解模型（空=DS4 默认）
	ExecutorModel string // 执行模型
	WorktreeDir   string
	Workdir       string
	// PlanningModels/ExecutionModels: 调用方从 fleet.yaml planning_models 读入
	PlanningModels  []string
	ExecutionModels []string
}

// BuildConfig — 从 TaskSpec 组装 Config（S3——fleet.yaml 规划模型接线）
// planningModels 按序取第一个非空；全空回退 executionModels[0]
func BuildConfig(spec TaskSpec) Config {
	planner := spec.PlannerModel
	if planner == "" {
		for _, m := range spec.PlanningModels {
			if m != "" {
				planner = m
				break
			}
		}
	}
	if planner == "" && len(spec.ExecutionModels) > 0 {
		planner = spec.ExecutionModels[0]
	}
	executor := spec.ExecutorModel
	if executor == "" && len(spec.ExecutionModels) > 0 {
		executor = spec.ExecutionModels[0]
	}
	return Config{
		WorktreeDir:   spec.WorktreeDir,
		Workdir:       spec.Workdir,
		PlannerModel:  planner,
		ExecutorModel: executor,
	}
}

// ── S4: crystallization.jsonl 持久化 + 断点恢复 ──

// SavePlan — plan.json 落盘
func SavePlan(taskDir string, p *Plan) error {
	return os.WriteFile(filepath.Join(taskDir, "plan.json"), []byte(p.ToJSON()), 0o644)
}

// LoadPlan — 断点恢复: 读 plan
func LoadPlan(taskDir string) (*Plan, error) {
	b, err := os.ReadFile(filepath.Join(taskDir, "plan.json"))
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// SaveCrystal — 结晶追加落盘（crystallization.jsonl——JSONL 每行一个）
func SaveCrystal(taskDir string, c *Crystal) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(taskDir, "crystallization.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// LoadCrystals — 断点恢复: 读全部结晶（按 stepID 去重——取最新）
func LoadCrystals(taskDir string) (map[string]*Crystal, error) {
	b, err := os.ReadFile(filepath.Join(taskDir, "crystallization.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*Crystal{}, nil
		}
		return nil, err
	}
	out := map[string]*Crystal{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var c Crystal
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue // 坏行容错
		}
		out[c.StepID] = &c // 后写覆盖
	}
	return out, nil
}

// RemainingSteps — 断点恢复: 计算未完成步骤（结晶非 done 的都算）
func RemainingSteps(p *Plan, crystals map[string]*Crystal) []Step {
	var out []Step
	for _, id := range p.SortedStepIDs() {
		if c, ok := crystals[id]; ok && c.ExitKind == ExitDone {
			continue
		}
		for _, s := range p.Steps {
			if s.ID == id {
				out = append(out, s)
			}
		}
	}
	return out
}

// ResumeSummary — 恢复摘要（日志/审计）
func ResumeSummary(p *Plan, crystals map[string]*Crystal) string {
	done, total := 0, len(p.Steps)
	for _, s := range p.Steps {
		if c, ok := crystals[s.ID]; ok && c.ExitKind == ExitDone {
			done++
		}
	}
	return fmt.Sprintf("断点恢复: %d/%d 阶段已完成——续 %d 阶段", done, total, total-done)
}
