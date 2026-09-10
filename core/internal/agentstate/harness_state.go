package agentstate

// harness_state.go — 虫族 v2.4 M2 Agent 框架：任务状态持久化（2026-08-13）
// 设计：backlog-v2.4.md M2（蓝本 2 local harness_state + LoopX 补强）
// 格式：JSON（Prime Agent 蓝本——Go 程序读写）+ todo 状态机 + evidence 契约 + quota

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TodoStatus todo 状态流转
type TodoStatus string

const (
	TodoOpen       TodoStatus = "open"        // 未开始
	TodoInProgress TodoStatus = "in_progress" // 进行中
	TodoCompleted  TodoStatus = "completed"   // 完成
	TodoBlocked    TodoStatus = "blocked"     // 阻塞（等审批/依赖）
)

// Todo 任务项（LoopX todo 状态机——JSON 字段版）
type Todo struct {
	ID         string     `json:"id"`                    // 唯一标识（todo_xxx）
	Text       string     `json:"text"`                  // 任务描述
	Priority   string     `json:"priority"`              // P0/P1/P2
	Status     TodoStatus `json:"status"`                // open/in_progress/completed/blocked
	ActionKind string     `json:"action_kind,omitempty"` // shell/file/research
	UpdatedAt  string     `json:"updated_at"`            // 时间戳
}

// Evidence 每轮结束契约（LoopX 4 行——防"干了没记录"）
type Evidence struct {
	Round      int    `json:"round"`          // 轮次
	Changed    string `json:"changed"`        // 改了什么
	Validation string `json:"validation"`     // 怎么验证
	Risk       string `json:"risk,omitempty"` // 剩余风险
	Next       string `json:"next"`           // 下一步
	Timestamp  string `json:"timestamp"`
}

// Quota 配额记账（防烧算力）
type Quota struct {
	WindowHours int `json:"window_hours"` // 窗口小时（默认 24）
	SpentSlots  int `json:"spent_slots"`  // 已消耗槽
	MaxSlots    int `json:"max_slots"`    // 上限（默认 720）
}

// HarnessState 任务状态文件（local——session 目录）
type HarnessState struct {
	Status    string     `json:"status"`          // active/completed/failed
	Objective string     `json:"objective"`       // 任务目标
	Agent     string     `json:"agent,omitempty"` // 执行 agent（codex/agent）
	UpdatedAt string     `json:"updated_at"`
	Todos     []Todo     `json:"todos"`
	Evidence  []Evidence `json:"evidence"`
	Quota     Quota      `json:"quota"`
	todoSeq   int        `json:"-"` // 内部序号（防同纳秒 ID 碰撞——flaky test 根因）
}

// defaultQuota 默认配额
func defaultQuota() Quota {
	return Quota{WindowHours: 24, SpentSlots: 0, MaxSlots: 720}
}

// NewState 创建新任务状态（objective + 初始 todos）
func NewState(objective, agent string, todos []Todo) *HarnessState {
	now := time.Now().Format(time.RFC3339)
	return &HarnessState{
		Status:    "active",
		Objective: objective,
		Agent:     agent,
		UpdatedAt: now,
		Todos:     todos,
		Evidence:  []Evidence{},
		Quota:     defaultQuota(),
	}
}

// Save 写状态到文件（原子写——防并发）
func (s *HarnessState) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	s.UpdatedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("写临时文件失败: %w", err)
	}
	return os.Rename(tmp, path)
}

// Load 读状态文件（不存在返回 nil——新任务）
func Load(path string) (*HarnessState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读状态文件失败: %w", err)
	}
	var s HarnessState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("解析状态文件失败: %w", err)
	}
	return &s, nil
}

// AddTodo 添加 todo
func (s *HarnessState) AddTodo(text, priority, actionKind string) *Todo {
	t := Todo{
		ID:         fmt.Sprintf("todo_%d_%d", time.Now().UnixNano(), s.todoSeq),
		Text:       text,
		Priority:   priority,
		Status:     TodoOpen,
		ActionKind: actionKind,
		UpdatedAt:  time.Now().Format(time.RFC3339),
	}
	s.todoSeq++ // 原子递增——防同纳秒 ID 碰撞（flaky test 根因）
	s.Todos = append(s.Todos, t)
	return &t
}

// SetTodoStatus 更新 todo 状态（open→in_progress→completed）
func (s *HarnessState) SetTodoStatus(id string, status TodoStatus) error {
	for i := range s.Todos {
		if s.Todos[i].ID == id {
			s.Todos[i].Status = status
			s.Todos[i].UpdatedAt = time.Now().Format(time.RFC3339)
			return nil
		}
	}
	return fmt.Errorf("todo %s 不存在", id)
}

// AddEvidence 写证据（每轮 4 行契约）
func (s *HarnessState) AddEvidence(changed, validation, risk, next string) {
	e := Evidence{
		Round:      len(s.Evidence) + 1,
		Changed:    changed,
		Validation: validation,
		Risk:       risk,
		Next:       next,
		Timestamp:  time.Now().Format(time.RFC3339),
	}
	s.Evidence = append(s.Evidence, e)
}

// SpendQuota 消耗配额（超限返回 false——等Mr2109批准）
func (s *HarnessState) SpendQuota() bool {
	s.Quota.SpentSlots++
	return s.Quota.SpentSlots <= s.Quota.MaxSlots
}

// NextTodo 下一个可执行 todo（open 优先 P0）
func (s *HarnessState) NextTodo() *Todo {
	prio := map[string]int{"P0": 0, "P1": 1, "P2": 2}
	best := -1
	for i := range s.Todos {
		if s.Todos[i].Status == TodoOpen {
			if best == -1 || prio[s.Todos[i].Priority] < prio[s.Todos[best].Priority] {
				best = i
			}
		}
	}
	if best == -1 {
		return nil
	}
	return &s.Todos[best]
}

// AllCompleted 是否全部完成
func (s *HarnessState) AllCompleted() bool {
	if len(s.Todos) == 0 {
		return false
	}
	for _, t := range s.Todos {
		if t.Status != TodoCompleted {
			return false
		}
	}
	return true
}
