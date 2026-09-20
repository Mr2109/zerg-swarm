package api

import (
	"encoding/json"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/chat"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/gateway"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/Mr2109/zerg-swarm/core/internal/subtask"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handlers 封装所有 HTTP 处理器需要的依赖。
type Handlers struct {
	Config          *config.FleetConfig
	ConfigPath      string // B11: 配置文件路径（热加载用）
	Store           *store.Store
	HeartbeatLogger *slog.Logger     // v2.3 B1: 心跳专用日志，分离到 /tmp/zerg-heartbeat.log
	Gateway         *gateway.Gateway // v2.5.5 #9 补充5: 心跳健康清零熔断用
	Scheduler       *MasterScheduler // v2.5.5 T3: 主控总调度器（两级调度）
	// v2.5.7 对话→任务集成: 来源对话内容查询（派任务带上下文——main.go 对话模块就绪后注入）
	ChatStore *chat.ChatStore
	// 模型目录根覆盖（GET /api/models/registry 用）。空 = 标准解析
	// （ZERG_MODELS_DIR 优先，缺省 ~/.zerg/models）——测试注入 t.TempDir() 用。
	ModelsDir string

	// ── 资源管理器观测面（《设计-资源管理器》批 4）──────────────────────────
	// ResourcePins 是 pin/unpin 的动作侧（转发到子端）。nil = 未接线（接口如实返回 503）。
	ResourcePins ResourcePinController
	// KvCacheBytesPerElem 是 KV cache 每元素字节（引擎 --cache-type-k/v；如 fp16=2）。
	// 0 = 未知（GGUF 不记录）→ 估算回退常量并标 estimated=true。测试/运维可注入真值。
	KvCacheBytesPerElem float64
	// EngineOverheadGb 是引擎运行时/临时缓冲固定开销（GiB）。0 = 未知 → 回退常量并标 estimated。
	EngineOverheadGb float64

	// ── 文件/目录浏览器（《设计-文件浏览器虫茧-20260913》阶段 1）──────────────
	// FileBrowserAudit 仅测试注入：open/reveal 审计的目录与轮转参数（nil = ~/.zerg/logs +
	// 1 MiB/留 5 份/清 30 天）。测试要"造小上限 + 伪时钟"验证轮转与过期清理，又不能碰 ~/.zerg。
	FileBrowserAudit *fileBrowserAuditParams
	// FileBrowserRun 仅测试注入："交给系统"动作的执行器（nil = 真跑 open/xdg-open）。
	// 为什么必须可注入：测试若真执行 open，会弹出访达窗口——那不是一个可重复的测试。
	FileBrowserRun func(action, abs string) error
}

// writeJSON 辅助函数：写入 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError 辅助函数：写入错误 JSON 响应（**兼容路径**——多语言 L4 后 API 一律用 writeErrorCode）。
// v2.5.6 错误码设计（2026-08-29）: 响应格式对齐网关——{"error":{"type":"<code>","message":"<msg>"}}
// 多语言 L4（2026-09-11）: api 包 88 处调用点已全部迁到 writeErrorCode（type=UPPER_SNAKE_CASE），
// 本函数仅保留给外部插件/未来接口兜底（客户端两种形态都能解析——ui parse_api_error）。
func writeError(w http.ResponseWriter, status int, message string) {
	// §九 M2 `R` 面：**先脱敏、再编码**（`obs_bridge.go` 是这条规矩的接电点——已红第 10 条
	// 的病灶就是「规则写了、没人接电」）。
	writeJSON(w, status, map[string]string{"error": redactedMessage(message)})
}

// writeErrorCode 带分类码的错误响应（错误码体系——对齐网关 {"error":{"type":code,"message":msg}}）
// 用法: writeErrorCode(w, http.StatusNotFound, "MODEL_NOT_FOUND", "模型不存在: xxx")
//
// 多语言 L4（2026-09-11）约定：
//   - code 用 UPPER_SNAKE_CASE（对齐 Google AIP-193 reason 与网关 type）
//   - **双写期：只加 code，不改 message**（旧客户端读 message/HTTP 状态照常工作）
//   - UI 侧按 code 渲染本地化文案（ui/src/api.rs parse_api_error + locales 的 apierr.* 键），
//     未收录的 code 原样回显服务端 message——因此新增 code 不必同步发 UI
func writeErrorCode(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{
			"type":    code,
			"message": redactedMessage(message), // 同上：先脱敏、再编码
		},
	})
}

// SubmitTaskHandler 提交任务到总调度器（v2.5.5 T3——外部任务接入/内部任务触发）
func (h *Handlers) SubmitTaskHandler(w http.ResponseWriter, r *http.Request) {
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	var req struct {
		// B 项③ 片（单子）最小 schema: slice_id / depends_on / owner / acceptance。
		// **只在声明了片字段时**才过挂板校验（未声明 ⇒ 与改动前逐字节等价——现有任务零回归）。
		SliceInput
		ID          string `json:"id"`
		Description string `json:"description"`
		Type        string `json:"type"`     // internal/external
		Priority    int    `json:"priority"` // 优先级（默认 internal）
		Model       string `json:"model"`
		Workdir     string `json:"workdir"`
		Flow        string `json:"flow"` // v2.5.6: "zerg"=程序定量驱动新流程（空=旧 CA 流程）
		// v2.5.7 对话→任务集成: 来源对话/消息（UI"派任务"带——任务详情可回溯来源）
		ParentSessionID string `json:"parent_session_id"`
		ParentMessageID string `json:"parent_message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体解析失败: "+err.Error())
		return
	}
	if req.Description == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_DESCRIPTION", "description 不能为空")
		return
	}
	description := req.Description
	// v2.5.7 对话→任务: 带来源会话时——附上对话最后用户请求（CA 任务描述有上下文——不然只知道"处理对话 xxx"不知道干啥）
	if req.ParentSessionID != "" && h.ChatStore != nil {
		if msgs, err := h.ChatStore.ListMessages(req.ParentSessionID); err == nil {
			for i := len(msgs) - 1; i >= 0; i-- {
				if msgs[i].Role == "user" && msgs[i].Content != "" {
					description = description + "\n\n[来源对话 " + req.ParentSessionID + "——用户最后请求]\n" + msgs[i].Content
					break
				}
			}
		}
	}
	taskType := req.Type
	if taskType == "" {
		taskType = "internal"
	}
	priority := TaskPriority(req.Priority)
	if priority == 0 {
		if taskType == "external" {
			priority = PriorityExternal
		} else {
			priority = PriorityInternal
		}
	}
	id := req.ID
	if id == "" {
		id = fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	task := &Task{
		ID:              id,
		Description:     description,
		Priority:        priority,
		Type:            taskType,
		Status:          "queued",
		Model:           req.Model,
		Workdir:         req.Workdir,
		Flow:            req.Flow, // v2.5.6: "zerg"=程序定量驱动新流程
		ParentSessionID: req.ParentSessionID,
		ParentMessageID: req.ParentMessageID,
		CreatedAt:       time.Now(),
	}
	// B 项③ 片（单子）: 片字段落进任务（未声明片字段 ⇒ 全是零值，等价于没这一行）
	req.SliceInput.ApplyTo(task)
	if req.SliceInput.Declared() {
		// 声明了片字段 ⇒ **挂板校验**后入队（缺 slice_id / 缺 acceptance 声明 / depends_on 环 / 悬空依赖
		// ⇒ 400 + 机器可判错误码，**拒绝入队**）。校验与入队共用 SubmitSlice 这一个判定点。
		if err := h.Scheduler.SubmitSlice(task); err != nil {
			if serr, ok := err.(*SliceValidationError); ok {
				writeErrorCode(w, http.StatusBadRequest, serr.Code, serr.Message)
				return
			}
			writeErrorCode(w, http.StatusBadRequest, "SLICE_MOUNT_REJECTED", err.Error())
			return
		}
	} else {
		// 非片（现有全部任务走这条）: 原实现逐字不变——零回归
		h.Scheduler.Submit(task)
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"id":       task.ID,
		"status":   task.Status,
		"priority": task.Priority,
		"type":     task.Type,
	})
}

// ListTasksHandler 列出任务（总调度器视角——所有任务状态——含历史）
// v2.5.5 虫族UI: 返回 running + queue + history（UI 3 组显示）
func (h *Handlers) ListTasksHandler(w http.ResponseWriter, r *http.Request) {
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	h.Scheduler.mu.Lock()
	tasks := make([]*Task, 0, len(h.Scheduler.queue)+len(h.Scheduler.running)+len(h.Scheduler.history))
	// v2.5.5 去重（2026-08-21 发现——失败自动重跑任务同 ID 在 history+queue 重复——列表重复显示）
	seen := map[string]bool{}
	addTask := func(t *Task) {
		if t == nil || seen[t.ID] {
			return
		}
		seen[t.ID] = true
		tasks = append(tasks, t)
	}
	for _, t := range h.Scheduler.queue {
		addTask(t)
	}
	for _, t := range h.Scheduler.running {
		addTask(t)
	}
	for _, t := range h.Scheduler.history {
		addTask(t)
	}
	h.Scheduler.mu.Unlock()
	// v2.5.6 任务详情补设备（Mr2109 2026-08-27——UI 显示跑的设备——按模型所在机器推算）
	machineOf := h.modelMachineMap()
	for _, t := range tasks {
		if m, ok := machineOf[t.Model]; ok {
			t.Machine = m
		} else if t.Machine == "" {
			t.Machine = "本机"
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// TaskDetailHandler 任务详情（v2.5.5 虫族UI: 点击任务看全部数据——含跟踪）
// GET /api/tasks/{id}——基本信息 + 执行时间线（events.jsonl 解析）+ 产物
func (h *Handlers) TaskDetailHandler(w http.ResponseWriter, r *http.Request) {
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	taskID := chi.URLParam(r, "id")
	h.Scheduler.mu.Lock()
	var task *Task
	if t, ok := h.Scheduler.running[taskID]; ok {
		task = t
	} else if t, ok := h.Scheduler.history[taskID]; ok {
		task = t
	} else {
		// v2.5.5 修复（2026-08-22 skill 测试发现）: queued 任务在 queue heap——详情也要查
		for _, t := range h.Scheduler.queue {
			if t.ID == taskID {
				task = t
				break
			}
		}
	}
	h.Scheduler.mu.Unlock()
	if task == nil {
		writeErrorCode(w, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在: "+taskID)
		return
	}
	// 组装详情（基本信息 + 跟踪）
	detail := map[string]interface{}{
		"id":           task.ID,
		"description":  task.Description,
		"type":         task.Type,
		"priority":     task.Priority,
		"status":       task.Status,
		"model":        task.Model,
		"workdir":      task.Workdir,
		"created_at":   task.CreatedAt,
		"completed_at": task.CompletedAt,
		"issue_path":   task.IssuePath,
	}
	// v2.5.6 详情补设备（Mr2109 2026-08-27——UI 显示跑的设备——按模型所在机器推算）
	{
		machine := task.Machine
		if machine == "" {
			if m, ok := h.modelMachineMap()[task.Model]; ok {
				machine = m
			} else {
				machine = "本机"
			}
		}
		detail["machine"] = machine
	}
	// 跟踪数据（events.jsonl 解析——轮次/工具/token）
	trace := parseTaskTrace(task.ID)
	detail["trace"] = trace
	// S8: 结晶模式阶段进度卡（plan.json+crystallization.jsonl——任务目录读）
	{
		taskDir := filepath.Join(statepath.TaskRoot(), sanitizeID(task.ID))
		if plan, err := subtask.LoadPlan(taskDir); err == nil && plan != nil {
			crystals, _ := subtask.LoadCrystals(taskDir)
			type StageView struct {
				ID      string   `json:"id"`
				Goal    string   `json:"goal"`
				Status  string   `json:"status"` // done/partial/blocked/running/pending
				Results []string `json:"results,omitempty"`
			}
			var stages []StageView
			for _, stepID := range plan.SortedStepIDs() {
				step := plan.StepByID(stepID)
				if step == nil {
					continue
				}
				sv := StageView{ID: step.ID, Goal: step.Goal, Status: "pending"}
				if c, ok := crystals[step.ID]; ok {
					sv.Status = string(c.ExitKind)
					sv.Results = c.Results
				}
				stages = append(stages, sv)
			}
			if stages != nil {
				detail["stages"] = stages
				detail["subtask_mode"] = true
			}
		}
	}
	// v2.5.5 虫族UI 任务详情增强（Mr2109 2026-08-20）: 执行报告 + 复查报告
	// 执行报告: workdir/internal-task-report.md（执行模型写的）
	// 复查报告: workdir/review-report.md 或复查任务工作区的报告
	// v2.5.5 修复（2026-08-22 skill 测试发现）: 不依赖 task.Workdir（可能空）——按任务 ID 读任务目录
	{
		// 执行报告（任务目录——CA 写任务目录/work——2026-08-21 修复）
		taskDir := filepath.Join(statepath.TaskRoot(), sanitizeID(task.ID))
		execReport := FindTaskReport(filepath.Join(taskDir, "work"))
		if execReport != "" {
			if content, err := os.ReadFile(execReport); err == nil {
				detail["exec_report"] = string(content)
			}
		}
		// 任务目录根的报告（复制保留——copyReportToTaskDir）
		if _, ok := detail["exec_report"].(string); !ok || detail["exec_report"] == "" {
			rootReport := filepath.Join(taskDir, "internal-task-report.md")
			if content, err := os.ReadFile(rootReport); err == nil {
				detail["exec_report"] = string(content)
			}
		}
		// git 回退（旧任务——worktree merge 后报告进 git）
		if _, ok := detail["exec_report"].(string); !ok || detail["exec_report"] == "" {
			if content := gitShowReport(task.ID, "internal-task-report.md"); content != "" {
				detail["exec_report"] = content
			}
		}
		// 复查报告（review-report.md——复查模型写的——约定文件名——写执行任务目录）
		reviewReport := filepath.Join(taskDir, "review-report.md")
		if content, err := os.ReadFile(reviewReport); err == nil {
			detail["review_report"] = string(content)
		}
		// 复查报告 git 回退（任务目录没有——git 历史找）
		if _, ok := detail["review_report"].(string); !ok || detail["review_report"] == "" {
			if content := gitShowReport(task.ID, "review-report.md"); content != "" {
				detail["review_report"] = content
			}
		}
	}
	writeJSON(w, http.StatusOK, detail)
}

// gitShowReport 从 git 历史找回任务报告（worktree merge 回 main——报告在 git 保留）
// v2.5.5 修复（2026-08-21 Mr2109发现——UI 报告缺失——英文报告写 git）
// 查 task-<ID> 分支/或 main 历史里的报告文件——内容验证（防共享旧报告误显示）
func gitShowReport(taskID, reportName string) string {
	repoDir := statepath.WorkspaceRoot()
	branch := worktreeBranchFor(taskID)
	// 尝试分支（task-<ID>——merge 前）
	cmd := exec.Command("git", "-C", repoDir, "show", branch+":"+reportName)
	if out, err := cmd.CombinedOutput(); err == nil && len(out) > 0 {
		return string(out)
	}
	// 回退: main 历史找（git log 找报告路径）——但验证内容（防同名共享报告误显示）
	cmd2 := exec.Command("git", "-C", repoDir, "log", "--all", "--format=%H", "--", reportName)
	if out, err := cmd2.CombinedOutput(); err == nil {
		commits := strings.Fields(string(out))
		for _, c := range commits {
			cmd3 := exec.Command("git", "-C", repoDir, "show", c+":"+reportName)
			if out3, err3 := cmd3.CombinedOutput(); err3 == nil && len(out3) > 0 {
				content := string(out3)
				// 内容验证: 报告必须含任务 ID 短码（唯一——防共享旧报告误显示）
				short := taskID
				if len(taskID) > 10 {
					short = taskID[len(taskID)-10:] // 任务 ID 后 10 位（唯一）
				}
				if strings.Contains(content, short) {
					return content
				}
			}
		}
	}
	return ""
}

// parseTaskTrace 解析任务跟踪（v2.5.5 虫族UI: /tmp/zerg-ca-logs/<任务ID>/events.jsonl → 轮次）
func parseTaskTrace(taskID string) map[string]interface{} {
	// 找任务日志目录（taskID 是纳秒时间戳——日志目录是日期格式——模糊匹配）
	logDir := findTaskLogDir(taskID)
	if logDir == "" {
		return map[string]interface{}{"rounds": []interface{}{}, "note": "无跟踪数据（日志未找到）"}
	}
	eventsPath := filepath.Join(logDir, "events.jsonl")
	data, err := os.ReadFile(eventsPath)
	if err != nil {
		return map[string]interface{}{"rounds": []interface{}{}, "note": "日志读取失败: " + err.Error()}
	}
	// 解析 events.jsonl（每行 JSON——轮次/模型/工具）
	rounds := []map[string]interface{}{}
	var curRound map[string]interface{}
	var curRoundStart string // 轮次开始时间（算耗时——用 loop_start 首轮/上轮 end 后续——2026-08-21 修复）
	var totalTokens int64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		action, _ := ev["Action"].(string)
		evType, _ := ev["Type"].(string)
		if action == "loop_start" {
			// 整个循环开始——第一轮的开始时间
			curRoundStart, _ = ev["Timestamp"].(string)
		}
		if action == "loop_iteration_end" {
			// 轮次结束——收集（带耗时）
			if curRound != nil {
				rounds = append(rounds, curRound)
			}
			curRound = map[string]interface{}{
				"round": len(rounds),
				"tools": []map[string]interface{}{},
			}
			// 每轮耗时（轮开始→轮结束——Timestamp RFC3339）
			if endTime, ok := ev["Timestamp"].(string); ok && curRoundStart != "" {
				curRound["duration"] = roundDuration(curRoundStart, endTime)
			}
			// 下一轮开始 = 本轮结束（轮间连续——2026-08-21 修复: 之前 model_call_ok 同毫秒不准）
			if endTime, ok := ev["Timestamp"].(string); ok {
				curRoundStart = endTime
			}
		}
		if curRound == nil {
			curRound = map[string]interface{}{"round": 0, "tools": []map[string]interface{}{}}
		}
		if evType == "model_call" && action == "model_call_ok" {
			prompt, _ := ev["Prompt"].(string)
			curRound["model"] = map[string]interface{}{
				"tokens": len(prompt) / 4, // 近似（真实 token 在 Args）
				"time":   ev["Timestamp"],
			}
			// v2.5.5 修复（2026-08-21 Mr2109发现）: events.jsonl 无 loop_iteration_start——用 model_call_ok 时间做轮开始
			// 每轮第一次模型调用 = 轮开始（后续 model_call 不覆盖——只记录首次）
			if curRoundStart == "" {
				if t, ok := ev["Timestamp"].(string); ok {
					curRoundStart = t
				}
			}
		}
		if evType == "tool_call" {
			toolName, _ := ev["ToolName"].(string)
			args, _ := ev["Args"].(map[string]interface{})
			tools, _ := curRound["tools"].([]map[string]interface{})
			curRound["tools"] = append(tools, map[string]interface{}{
				"name": toolName,
				"args": args,
			})
		}
	}
	if curRound != nil {
		rounds = append(rounds, curRound)
	}
	return map[string]interface{}{
		"rounds":       rounds,
		"total_tokens": totalTokens,
		"log_dir":      logDir,
	}
}

// roundDuration 计算轮次耗时（RFC3339 时间差——人性化）
func roundDuration(start, end string) string {
	parse := func(s string) (time.Time, bool) {
		// 尝试 RFC3339（含毫秒/纳秒）
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if t, err := time.Parse(layout, s); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	}
	st, ok1 := parse(start)
	et, ok2 := parse(end)
	if !ok1 || !ok2 {
		return ""
	}
	d := et.Sub(st)
	if d < 0 {
		return ""
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// findTaskLogDir 找任务日志目录（优先 audit.jsonl 精确匹配——回退模糊）
// v2.5.5 修复（2026-08-21 Mr2109发现——每轮时间不对）: 之前 strings.Contains(taskID, e.Name()) 永不匹配
// → 总是返回最新目录（错误日志——duration 算不出）——改读 audit.jsonl 精确关联
func findTaskLogDir(taskID string) string {
	base := statepath.CAEventLogRoot()
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	// 1. 精确匹配: 日志目录 audit.jsonl 含 task_id（CA 启动时写审计——ZERG_TASK_DIR 路径）
	// 2026-08-21 修复: 任务可能重跑多次（多日志目录）——返回最新匹配（不是第一个）
	// 且排除复查任务日志（audit task_id 是 review- 前缀——误匹配执行任务）
	best := ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		auditPath := filepath.Join(base, e.Name(), "audit.jsonl")
		if content, err := os.ReadFile(auditPath); err == nil {
			auditStr := string(content)
			// 2026-08-21 修复: 执行任务查询排除复查日志（audit 里 task_id 是 review-task-...）
			if !strings.Contains(taskID, "review") && strings.Contains(auditStr, "review-task-") {
				continue
			}
			if strings.Contains(auditStr, taskID) ||
				strings.Contains(auditStr, strings.TrimPrefix(taskID, "task-")) {
				best = filepath.Join(base, e.Name()) // 覆盖——最后匹配=最新
			}
		}
	}
	if best != "" {
		return best
	}
	// 2. 任务 ID 纳秒 → 时间戳 → 匹配目录名（2026-08-21 修复——旧格式无 audit 时）
	// 任务 ID 如 task-1787243319269431000——纳秒 → 目录名 2026-08-21T01-10-39
	if idx := strings.LastIndex(taskID, "-"); idx >= 0 {
		if ns, err := strconv.ParseInt(taskID[idx+1:], 10, 64); err == nil {
			t := time.Unix(0, ns).Local() // 本地时区（目录名是本地时间——2026-08-21 修复）
			// 目录名格式 2026-08-21T01-10-39（±2 分钟容差）
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				if dirTime, err := time.Parse("2006-01-02T15-04-05", e.Name()); err == nil {
					diff := dirTime.Sub(t)
					if diff > -2*time.Minute && diff < 2*time.Minute {
						return filepath.Join(base, e.Name())
					}
				}
			}
		}
	}
	// 3. 无匹配——返回最新目录（当前任务——兜底）
	if len(entries) > 0 {
		return filepath.Join(base, entries[len(entries)-1].Name())
	}
	return ""
}

// GitStatusHandler 仓库 git 总览（v2.5.5 虫族UI: 分支/worktree/未merge任务分支）
// GET /api/git/status
func (h *Handlers) GitStatusHandler(w http.ResponseWriter, r *http.Request) {
	repoDir := r.URL.Query().Get("repo")
	if repoDir == "" {
		repoDir = statepath.WorkspaceRoot()
	}
	// git branch --list（全分支）
	branchOut, _ := exec.Command("git", "-C", repoDir, "branch", "--list").Output()
	branches := []string{}
	for _, l := range strings.Split(string(branchOut), "\n") {
		l = strings.TrimSpace(strings.TrimPrefix(l, "*"))
		if l != "" {
			branches = append(branches, l)
		}
	}
	// git worktree list（worktree 状态）
	wtOut, _ := exec.Command("git", "-C", repoDir, "worktree", "list").Output()
	worktrees := []map[string]string{}
	for _, l := range strings.Split(string(wtOut), "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		parts := strings.Fields(l)
		if len(parts) >= 2 {
			worktrees = append(worktrees, map[string]string{
				"path":   parts[0],
				"branch": strings.Trim(strings.TrimPrefix(parts[1], "["), "]"),
			})
		}
	}
	// 未 merge 任务分支（task-* 不在 main）
	unmerged := []string{}
	for _, b := range branches {
		if strings.HasPrefix(b, "task-") {
			mergedOut, _ := exec.Command("git", "-C", repoDir, "branch", "--merged", "main", "--list", b).Output()
			if strings.TrimSpace(string(mergedOut)) == "" {
				unmerged = append(unmerged, b)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"repo":      repoDir,
		"branches":  branches,
		"worktrees": worktrees,
		"unmerged":  unmerged,
	})
}

// TaskGitHandler 任务 git 详情（v2.5.5 虫族UI: 分支/commit 历史/diff 统计）
// GET /api/tasks/{id}/git
func (h *Handlers) TaskGitHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	// 任务 worktree 路径（zerg-wt/task-<id>）
	wtDir := filepath.Join(statepath.WorkspaceRoot(), "zerg-wt", "task-"+sanitizeID(taskID))
	if _, err := os.Stat(wtDir); err != nil {
		writeErrorCode(w, http.StatusNotFound, "WORKTREE_NOT_FOUND", "任务 worktree 不存在: "+taskID)
		return
	}
	// git log（最近 20 commit）
	logOut, _ := exec.Command("git", "-C", wtDir, "log", "--oneline", "-20").Output()
	commits := []string{}
	for _, l := range strings.Split(string(logOut), "\n") {
		if strings.TrimSpace(l) != "" {
			commits = append(commits, strings.TrimSpace(l))
		}
	}
	// git diff --stat（未提交改动）
	statOut, _ := exec.Command("git", "-C", wtDir, "diff", "--stat").Output()
	// git status --short
	statusOut, _ := exec.Command("git", "-C", wtDir, "status", "--short").Output()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"task_id":   taskID,
		"worktree":  wtDir,
		"commits":   commits,
		"diff_stat": strings.TrimSpace(string(statOut)),
		"status":    strings.TrimSpace(string(statusOut)),
	})
}

// TaskDiffHandler 任务 diff 内容（v2.5.5 虫族UI: 查看变更）
// GET /api/tasks/{id}/diff
func (h *Handlers) TaskDiffHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	wtDir := filepath.Join(statepath.WorkspaceRoot(), "zerg-wt", "task-"+sanitizeID(taskID))
	if _, err := os.Stat(wtDir); err != nil {
		writeErrorCode(w, http.StatusNotFound, "WORKTREE_NOT_FOUND", "任务 worktree 不存在: "+taskID)
		return
	}
	diffOut, err := exec.Command("git", "-C", wtDir, "diff").Output()
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "GIT_DIFF_FAILED", "git diff 失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"task_id": taskID,
		"diff":    string(diffOut),
	})
}

// ResourcesHandler 资源库（v2.5.5 虫族UI: 模型/工具/skill/MCP 清单）
// GET /api/resources/{type}（models|tools|skills|mcp）
func (h *Handlers) ResourcesHandler(w http.ResponseWriter, r *http.Request) {
	resType := chi.URLParam(r, "type")
	switch resType {
	case "models":
		// 模型清单（从 fleet/models API 聚合）
		models := h.aggregateModels()
		// v2.5.5 资源信任度（2026-08-21 Mr2109）: 模型状态（🆕新/正式）
		for _, m := range models {
			if name, ok := m["name"].(string); ok {
				m["trust"] = resourceTrust.GetResourceStatus("models", name)
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"type": "models", "items": models})
	case "tools":
		// 工具库——CA 层（agent.DefaultTools）+ 对话层（chat deferred 138）合并
		items := []map[string]interface{}{}
		seen := map[string]bool{}
		// P4-49 统一计数（agent.ToolUses——对话+CA 同一计数器）
		trustOf := func(name string) string {
			u := agent.ToolUses(name)
			if u >= 100 {
				return "正式"
			}
			if u > 0 {
				return "新"
			}
			return "新"
		}
		// 2026-09-06 事件流: 当日事件按工具聚合(账本雏形——时间维度)
		evAgg := map[string]int{}
		for _, ev := range agent.ToolEventsToday() {
			evAgg[ev.Tool]++
		}
		for _, td := range agent.AllTools() {
			seen[td.Function.Name] = true
			items = append(items, map[string]interface{}{
				"name":       td.Function.Name,
				"scope":      "CA",
				"version":    agent.ToolVersion(td.Function.Name), // P4-49 工具版本（进化可追溯）
				"trust":      trustOf(td.Function.Name),
				"uses":       agent.ToolUses(td.Function.Name),
				"today_uses": evAgg[td.Function.Name], // 当日调用数(事件流源)
				"faults":     0,
				"since":      resourceTrust.GetResourceSince("tools", td.Function.Name),
				"desc":       firstLine(td.Function.Description), // 简介（一句话——Mr2109 2026-08-21）
			})
		}
		// P4-48 对话 deferred 工具（chat 层 138——ChatExtraToolDefs）
		for name, def := range chat.ChatExtraToolDefs() {
			if seen[name] {
				continue
			}
			seen[name] = true
			desc := ""
			if fn, ok := def["function"].(map[string]interface{}); ok {
				if d, ok2 := fn["description"].(string); ok2 {
					desc = firstLine(d)
				}
			}
			items = append(items, map[string]interface{}{
				"name":       name,
				"scope":      "对话",
				"version":    agent.ToolVersion(name), // P4-49 工具版本（进化可追溯）
				"trust":      trustOf(name),
				"uses":       agent.ToolUses(name),
				"today_uses": evAgg[name], // 2026-09-06 当日调用数（事件流源）
				"faults":     0,
				"since":      resourceTrust.GetResourceSince("tools", name),
				"desc":       desc,
			})
		}
		// 2026-09-11 一致性审计：补对话 L0 常驻工具
		// 原先只并「CA 工具 + 对话 deferred」→ doc_search / kb_search 这类
		// 「只在提示词 L0 定义表里」的工具在资源库（UI 资源库/版本/履历）里永远查不到。
		for _, d := range chat.HermesToolDefs() {
			if seen[d.Name] {
				continue
			}
			seen[d.Name] = true
			items = append(items, map[string]interface{}{
				"name":       d.Name,
				"scope":      "对话L0",
				"version":    agent.ToolVersion(d.Name),
				"trust":      trustOf(d.Name),
				"uses":       agent.ToolUses(d.Name),
				"today_uses": evAgg[d.Name],
				"faults":     0,
				"since":      resourceTrust.GetResourceSince("tools", d.Name),
				"desc":       firstLine(d.Desc),
			})
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"type": "tools", "items": items})
	case "skills":
		// 技能库 = **CA 实际能加载的那一个目录**（口径唯一）：与 cmd/zerg-agent 共用
		// statepath.SkillsDir()。原先这里另列了 skills/、agent/skills、~/.hermes/skills 等，
		// 结果是「列出来的加载不到、能加载的看不见」——资源库成了不可信的验收面。
		items := []map[string]interface{}{}
		skillDirs := []string{}
		if d := statepath.SkillsDir(); d != "" {
			skillDirs = append(skillDirs, d)
		}
		seen := map[string]bool{}
		for _, dir := range skillDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := e.Name()
				if seen[name] {
					continue
				}
				if e.IsDir() {
					// skill 目录（含 SKILL.md）才算技能——松散 .md 不列（CA 加载器只认目录）
					if _, err := os.Stat(filepath.Join(dir, name, "SKILL.md")); err == nil {
						items = append(items, map[string]interface{}{
							"name":   name,
							"trust":  resourceTrust.GetResourceStatus("skills", name),
							"uses":   resourceTrust.GetResourceUses("skills", name),
							"faults": resourceTrust.GetResourceFaults("skills", name),
							"since":  resourceTrust.GetResourceSince("skills", name),
						})
						seen[name] = true
					}
				}
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"type": "skills", "items": items})
	case "mcp":
		// MCP 库——扫描 MCP 配置（~/.hermes/mcp-servers + 项目 mcp/）
		items := []map[string]interface{}{}
		home, _ := os.UserHomeDir()
		mcpDirs := []string{filepath.Join(home, ".hermes", "mcp-servers"), "mcp", filepath.Join(statepath.TaskRoot(), "mcp")}
		seen := map[string]bool{}
		for _, dir := range mcpDirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				name := e.Name()
				if seen[name] {
					continue
				}
				items = append(items, map[string]interface{}{
					"name":   name,
					"trust":  resourceTrust.GetResourceStatus("mcp", name),
					"uses":   resourceTrust.GetResourceUses("mcp", name),
					"faults": resourceTrust.GetResourceFaults("mcp", name),
					"since":  resourceTrust.GetResourceSince("mcp", name),
				})
				seen[name] = true
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"type": "mcp", "items": items})
	default:
		writeErrorCode(w, http.StatusBadRequest, "UNKNOWN_RESOURCE_TYPE", "未知资源类型: "+resType)
	}
}

// aggregateModels 聚合模型清单（从 store 模型注册表——模型名/状态）
func (h *Handlers) aggregateModels() []map[string]interface{} {
	models := []map[string]interface{}{}
	if h.Store == nil {
		return models
	}
	// store 模型注册表（GetAllModels——各机器模型名）
	all := h.Store.GetAllModels()
	// v2.5.5 模型按设备分类（Mr2109 2026-08-20）: 从 fleet 配置读模型所在设备
	machineOf := h.modelMachineMap()
	for _, name := range all {
		machine := "unknown"
		if m, ok := machineOf[name]; ok {
			machine = m
		}
		models = append(models, map[string]interface{}{
			"name":        name,
			"status":      "available",
			"machine":     machine,
			"description": h.modelDescription(name),
		})
	}
	return models
}

// modelDescription 模型简介（从 fleet 配置读描述——没有则基本信息）
func (h *Handlers) modelDescription(name string) string {
	if h.Config == nil {
		return ""
	}
	if candidates, ok := h.Config.Models[name]; ok && len(candidates) > 0 {
		c := candidates[0]
		// 从候选描述/参数拼简介
		desc := c.Description
		if desc == "" {
			desc = fmt.Sprintf("%s model (%s device — %.0f GB — context %d)", name, c.Host, c.MemGb, c.CtxWindow)
		}
		return desc
	}
	return name + " 模型（设备未配置）"
}

// modelMachineMap 模型 → 设备映射（从 fleet 配置读——模型候选机器）
func (h *Handlers) modelMachineMap() map[string]string {
	m := map[string]string{}
	if h.Config == nil {
		return m
	}
	for name, candidates := range h.Config.Models {
		if len(candidates) > 0 {
			m[name] = candidates[0].Host // 第一个候选的设备
		}
	}
	return m
}

// TaskRetryHandler 重跑任务（failed→queued——右键功能——2026-08-21 Mr2109）
// POST /api/tasks/{id}/retry
func (h *Handlers) TaskRetryHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	if err := h.Scheduler.RetryTask(taskID); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "TASK_RETRY_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "task_id": taskID, "status": "queued"})
}

// TaskMoveHandler 重排任务（置顶/置底/上移/下移——右键功能——2026-08-21 Mr2109）
// POST /api/tasks/{id}/move?action=top|bottom|up|down
func (h *Handlers) TaskMoveHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	action := r.URL.Query().Get("action")
	if action == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_ACTION", "缺少 action 参数（top/bottom/up/down）")
		return
	}
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	if err := h.Scheduler.MoveTask(taskID, action); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "TASK_MOVE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "task_id": taskID, "action": action})
}

// TaskPauseHandler 暂停/继续任务（右键功能——2026-08-21 Mr2109）
// POST /api/tasks/{id}/pause?pause=true|false
func (h *Handlers) TaskPauseHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	pause := r.URL.Query().Get("pause") != "false" // 默认 true（暂停）
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	if err := h.Scheduler.PauseTask(taskID, pause); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "TASK_PAUSE_FAILED", err.Error())
		return
	}
	status := "paused"
	if !pause {
		status = "queued"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "task_id": taskID, "status": status})
}

// TaskDeleteHandler 删除排队任务（右键功能——2026-08-21 Mr2109）
// DELETE /api/tasks/{id}
func (h *Handlers) TaskDeleteHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	if err := h.Scheduler.DeleteTask(taskID); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "TASK_DELETE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "task_id": taskID, "deleted": true})
}

// firstLine 取首行（简介——一句话——2026-08-21 Mr2109）
func firstLine(s string) string {
	if i := strings.IndexAny(s, "\n."); i >= 0 {
		s = s[:i]
	}
	if len(s) > 60 {
		s = s[:60] + "…"
	}
	return s
}

// LogsHandler2 日志 API（v2.5.5 虫族UI: 主控/任务/机器日志）
// GET /api/logs（主控日志 tail）+ GET /api/logs/task/{id} + GET /api/logs/agent/{machine}
func (h *Handlers) LogsHandler2(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	switch kind {
	case "main":
		// 主控日志（/tmp/zerg-core.log tail）
		lines, _ := tailFile(filepath.Join(statepath.RuntimeLogDir(), "zerg-core.log"), limit)
		writeJSON(w, http.StatusOK, map[string]interface{}{"lines": lines})
	case "task":
		// 任务日志（/tmp/zerg-ca-logs/<最新>/events.jsonl）
		taskID := chi.URLParam(r, "id")
		lines, _ := tailFile(filepath.Join(statepath.CAEventLogRoot(), taskID, "events.jsonl"), limit)
		writeJSON(w, http.StatusOK, map[string]interface{}{"lines": lines})
	default:
		writeJSON(w, http.StatusOK, map[string]interface{}{"lines": []string{}, "note": "未知日志源: " + kind})
	}
}

// DocsHandler 文件读取 API（v2.5.5 虫族UI 起；2026-09-13 C9 第 4 步收口为**通用**能力）
//
// 三条入口：
//
//	· 带 root/path 查询参数   → 参数化读取（白名单根 + 类型闸门，实现见 fileroots.go）
//	· URL 里带路径（/api/docs/<rel>）→ 读**缺省 docs 根**的那个文件（宿主文件浏览器在用，§4.5 兼容）
//	· 什么都不带（GET /api/docs）→ 缺省根（docs）的**通用列目录**（与 ?root=docs 同一份实现）
//
// 2026-09-13（C9 第 4 步「拆」）：文档**专属**的那套（只收 .md、排除 issues/thunderbolt 的递归
// 列目录 + 五个写端点）随文档界面整块迁进文档茧（`zerg-cocoon/文档` 自带 Go 服务）。
// 缺省列目录改走 `docsParameterized` ⇒ `listFileRoot`（docs 根本来就是同一套过滤口径），
// 输出与迁移前**逐字节一致**（fileroots_test 的黄金用例仍然钉着它）。
func (h *Handlers) DocsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("root") || r.URL.Query().Has("path") {
		h.docsParameterized(w, r)
		return
	}
	// v2.5.6 多段路径: {path} 单段 → catch-all 用 "*"（00-总览/xxx.md）
	docPath := chi.URLParam(r, "path")
	if docPath == "" {
		docPath = chi.URLParam(r, "*")
	}
	// v2.5.6 URL 解码（reqwest 自动编码中文——后端必须解回原始路径）
	if unescaped, err := url.PathUnescape(docPath); err == nil {
		docPath = unescaped
	}
	if docPath == "" {
		// 不带路径 ⇒ 缺省根列目录（通用实现；缺省 root=docs，见 docsParameterized）
		h.docsParameterized(w, r)
		return
	}
	// 内容（防路径穿越）
	docsDir := docsRootPath()
	if strings.Contains(docPath, "..") {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_PATH", "非法路径")
		return
	}
	content, err := os.ReadFile(filepath.Join(docsDir, docPath))
	if err != nil {
		writeErrorCode(w, http.StatusNotFound, "DOC_NOT_FOUND", "文档不存在: "+docPath)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"path": docPath, "content": string(content)})
}

// tailFile 读文件尾部 N 行（日志 tail）——已定义于 control.go（复用）

// filterDocs 按关键词过滤文件（文档分组——书目录）
func filterDocs(files []string, keywords ...string) []string {
	out := []string{}
	for _, f := range files {
		for _, kw := range keywords {
			if strings.Contains(f, kw) {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

// HeartbeatHandler 处理 POST /api/fleet/heartbeat
// 接收子端心跳，更新集群状态。
func (h *Handlers) HeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 POST 方法")
		return
	}

	var req store.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_BODY", "请求体 JSON 解析失败: "+err.Error())
		return
	}

	// 验证必填字段
	if req.Machine == "" {
		writeErrorCode(w, http.StatusBadRequest, "MISSING_MACHINE", "machine 字段不能为空")
		return
	}

	// 处理心跳
	resp := h.Store.ReceiveHeartbeat(req)

	// v2.5.5 #9 补充5: 心跳 healthy → 自动清零该机器熔断计数（防残留熔断——健康恢复即解除）
	// 根因: 熔断计数只在"转发成功"清零——若健康恢复但无转发——熔断残留 → 路由一直拒绝
	if req.Healthy && h.Gateway != nil {
		h.Gateway.ClearFailures(req.Machine)
	}

	// 记录心跳日志（v2.3 B1: 分离到 /tmp/zerg-heartbeat.log，避免撑满主日志）
	// 打印心跳摘要（解引用指针，避免显示 0x 地址）
	modelName := "<nil>"
	if req.Model != nil {
		modelName = *req.Model
	}
	h.HeartbeatLogger.Info("心跳",
		"machine", req.Machine,
		"model", modelName,
		"state", req.BackendState,
		"healthy", req.Healthy,
		"code_version", req.CodeVersion,
		"code_sha", req.CodeSHA,
	)

	writeJSON(w, http.StatusOK, resp)
}

// StatusHandler 处理 GET /api/fleet/status
// 返回集群当前状态概览。
func (h *Handlers) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 GET 方法")
		return
	}

	snapshots := h.Store.GetAllSnapshots()
	models := h.Store.GetAllModels()

	// 构建状态响应
	// 健康/不健康计数：**唯一实现处** = CountFleetHealth（fleet_health.go，§二十一 已红第 1 条 T-23）——
	// 混版（本机 code_version ≠ 主控版本号）一律计不健康：设计稿「混版必须被判为不健康」+
	// 「混版不许当健康」。旧字段 `healthy`（子端自报）一个字不动，旧消费者零改动。
	fh := CountFleetHealth(MasterCodeVersion(), snapshots)

	totalMemAvailable := 0.0
	totalMemTotal := 0.0
	totalGPUUsed := 0.0
	totalActive := 0

	for _, snap := range snapshots {
		if snap == nil {
			continue
		}
		totalMemAvailable += snap.MemAvailableGb
		totalMemTotal += snap.MemTotalGb
		totalGPUUsed += snap.GpuUsedGb
		totalActive += snap.ActiveRequests
	}

	response := map[string]interface{}{
		"total_machines":         len(snapshots),
		"healthy_count":          fh.Healthy,
		"unhealthy_count":        fh.Unhealthy,
		"version_unknown_count":  fh.VersionUnknown,
		"mixed_version":          fh.Mixed(),
		"mixed_machines":         fh.MixedMachines,
		"master_code_version":    fh.MasterCodeVersion,
		"total_models":           len(models),
		"available_models":       models,
		"total_mem_available_gb": totalMemAvailable,
		"total_mem_total_gb":     totalMemTotal,
		"total_gpu_used_gb":      totalGPUUsed,
		"total_active_requests":  totalActive,
		"machines":               AnnotateVersionMismatch(fh.MasterCodeVersion, snapshots),
	}

	writeJSON(w, http.StatusOK, response)
}

// ModelsHandler 处理 GET /api/fleet/models
// 返回路由表中已知的全部模型（来自 fleet.yaml，含多候选 host）。
func (h *Handlers) ModelsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 GET 方法")
		return
	}

	// 从配置路由表返回全部模型（多候选展开为多条）
	var list []map[string]interface{}
	for name, candidates := range h.Config.Models {
		for _, c := range candidates {
			list = append(list, map[string]interface{}{
				"id":       name,
				"host":     c.Host,
				"backend":  c.Backend,
				"file":     c.File,
				"mem_gb":   c.MemGb,
				"modality": "text",
			})
		}
	}

	response := map[string]interface{}{
		"models": list,
		"count":  len(list),
	}

	writeJSON(w, http.StatusOK, response)
}

// ModelDetailHandler 模型详情（Mr2109 2026-08-27——UI 右栏: 简介 + 适配器所有选项 + 启动状态）
// GET /api/models/{name}——fleet 配置全字段（=适配器选项）+ 本机加载状态
func (h *Handlers) ModelDetailHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if h.Config == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "CONFIG_NOT_LOADED", "配置未加载")
		return
	}
	cands, ok := h.Config.Models[name]
	if !ok || len(cands) == 0 {
		writeErrorCode(w, http.StatusNotFound, "MODEL_NOT_FOUND", "模型不存在: "+name)
		return
	}
	c := cands[0]
	// 3c（2026-09-16）：加载状态改读**该机器的心跳快照**（本机角色退役 ⇒ 不再有 LocalBack 可问；
	// 本机那一台 = 名为 Mr2109 的普通子端，其驻留与 x3 一样经心跳上报）。
	status := "未加载"
	loaded := false
	if h.Store != nil {
		if snap := h.Store.GetSnapshot(c.Host); snap != nil {
			if snap.Model != nil && *snap.Model == name {
				loaded = true
			}
			for _, m := range snap.Models {
				if m == name {
					loaded = true
				}
			}
			if loaded {
				status = "已加载"
			}
		}
	}
	// 架构兜底（architecture 优先——arch 兼容旧 key）
	arch := c.Architecture
	if arch == "" {
		arch = c.Arch
	}
	thinking := false
	if c.Thinking != nil {
		thinking = *c.Thinking
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":         name,
		"family":       c.Family,
		"host":         c.Host,
		"backend":      c.Backend,
		"file":         c.File,
		"mem_gb":       c.MemGb,
		"ssd":          c.SSD,
		"ctx_window":   c.CtxWindow,
		"arch":         arch,
		"architecture": c.Architecture, // v2.5.6 2026-08-27 模型详情补全
		"thinking":     thinking,
		"mmproj":       c.Mmproj,
		"moe":          c.Moe,
		"template":     c.Template,
		"cmd":          c.Cmd,
		"env":          c.Env,
		"modality":     c.Modality,
		"tool_support": c.ToolSupport,
		"description":  c.Description,
		"added":        c.Added,
		"verified":     c.Verified,
		"status":       status,
		"loaded":       loaded,
		"can_start":    true,                   // 3c：有候选机即可启动（由主控转发到子端；原来仅限 host=="local"）
		"adapter_opts": h.AdapterOptions(name), // v2.5.6: 适配器调用选项（Temperature/APIFormat/ReasoningEffort 等——反射读适配器实例）
		"note":         "适配器选项=fleet.yaml 配置字段——启动/停止由主控转发到候选子端执行（3c：引擎一律由子端起）",
	})
}

// ModelStartHandler 手动启动模型（Mr2109 2026-08-27——UI 开关）
// POST /api/models/{name}/start——仅本机模型（LocalBack 直接加载）
func (h *Handlers) ModelStartHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if h.Config == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "CONFIG_NOT_LOADED", "配置未加载")
		return
	}
	cands, ok := h.Config.Models[name]
	if !ok || len(cands) == 0 {
		writeErrorCode(w, http.StatusNotFound, "MODEL_NOT_FOUND", "模型不存在: "+name)
		return
	}
	node, host, ok := h.firstCandidateNode(cands)
	if !ok {
		writeErrorCode(w, http.StatusServiceUnavailable, "NO_CANDIDATE_NODE", "该模型没有可用候选机")
		return
	}
	body, _ := json.Marshal(map[string]string{"model": name})
	status, respBody, err := forwardToNode(h.Config.Auth.Token, node, "/load", body)
	if err != nil {
		writeErrorCode(w, http.StatusBadGateway, "FORWARD_FAILED", "转发到 "+host+" 失败: "+err.Error())
		return
	}
	if status != http.StatusOK {
		writeJSON(w, status, json.RawMessage(respBody))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "name": name, "host": host, "status": "已提交加载（子端接管）"})
}

// firstCandidateNode 取该模型的第一个"在机群里有节点信息"的候选（3c：手动启动/停止改为转发到子端）。
func (h *Handlers) firstCandidateNode(cands []config.ModelCandidate) (config.FleetNode, string, bool) {
	for _, c := range cands {
		if node, ok := h.Config.Fleet[c.Host]; ok {
			return node, c.Host, true
		}
	}
	return config.FleetNode{}, "", false
}

// ModelStopHandler 手动停止模型（Mr2109 2026-08-27——UI 开关）
// POST /api/models/{name}/stop——停该模型（**幂等优先** · §九 M4 · §十二 P-031/P-013 ②）。
//
// 批 B · T-11 的两处修正（都是「幂等语义」的面，出处逐条写在下面）：
//
//	① **体形状**：原来发 `{"model": 名}`，而子端 `/unload` 只认 `{"models":[…]}` ⇒ 反序列化后
//	   `Models` 为空 → 走「全卸」分支（**卸掉该子端全部卵**）。这是调研-M4 §4.4 的**代码级发现**，
//	   本版改成 `{"models": [名]}`（定向卸载）。
//	② **已停目标不报错**：子端对未驻留项返回 `skipped + reasons[name]=not_resident`（不是错误）；
//	   主控**必须**把它报成成功（`changed:false` + 「已在此状态」），**不许**报 5xx/409。
//	   「已在该状态」不是错 —— 幂等重跑必须退 0（调研-M4 §5.2）。
//
// ★ 生效面：本改动是**源码级**；要真在跑的**主控**上生效须换件 + 重启主控（不可逆档）⇒
// 本批**只改源码 + 留判据**，重启待 Mr2109 拍（开工记录「待拍」清单）。
func (h *Handlers) ModelStopHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if h.Config == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "CONFIG_NOT_LOADED", "配置未加载")
		return
	}
	cands, ok := h.Config.Models[name]
	if !ok || len(cands) == 0 {
		writeErrorCode(w, http.StatusNotFound, "MODEL_NOT_FOUND", "模型不存在: "+name)
		return
	}
	node, host, ok := h.firstCandidateNode(cands)
	if !ok {
		writeErrorCode(w, http.StatusServiceUnavailable, "NO_CANDIDATE_NODE", "该模型没有可用候选机")
		return
	}
	// ① 定向卸载：`models` 是数组（子端只认这个名字 —— 发 `model` 会让它退化成「全卸」）。
	body, _ := json.Marshal(map[string]any{"models": []string{name}})
	status, respBody, err := forwardToNode(h.Config.Auth.Token, node, "/unload", body)
	if err != nil {
		writeErrorCode(w, http.StatusBadGateway, "FORWARD_FAILED", "转发到 "+host+" 失败: "+err.Error())
		return
	}
	// ② 幂等优先：未驻留（not_resident）⇒ 成功 + changed:false，**不**把「已在该状态」报成错。
	if status != http.StatusOK {
		if strings.Contains(string(respBody), "not_resident") || strings.Contains(string(respBody), "not loaded") {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"success": true, "changed": false, "name": name, "host": host,
				"status": "已在此状态（未装载）—— 幂等重跑不报错（§九 M4）",
			})
			return
		}
		writeJSON(w, status, json.RawMessage(respBody))
		return
	}
	changed := true
	var result struct {
		Stopped []string          `json:"stopped"`
		Skipped []string          `json:"skipped"`
		Reasons map[string]string `json:"reasons"`
	}
	if jerr := json.Unmarshal(respBody, &result); jerr == nil {
		if result.Reasons != nil && result.Reasons[name] == "not_resident" {
			changed = false
		}
		for _, s := range result.Skipped {
			if s == name {
				changed = false
			}
		}
	}
	state := "已提交卸载（子端接管）"
	if !changed {
		state = "已在此状态（未装载）—— 幂等重跑不改变状态（§九 M4 / §十二 P-031）"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true, "changed": changed, "name": name, "host": host, "status": state,
	})
}

// AdapterOptions 模型适配器配置项（Mr2109 2026-08-27——UI 显示适配器所有选项）
// 反射读取适配器实例的导出字段（Temperature/APIFormat/ReasoningEffort 等调用配置）
func (h *Handlers) AdapterOptions(model string) map[string]interface{} {
	if h.Gateway == nil {
		return nil
	}
	return h.Gateway.AdapterOptions(model)
}

// AdapterSchemaHandler 适配器参数 schema（Mr2109 2026-08-27——UI 编辑控件渲染——各模型各自参数集）
// GET /api/models/{name}/adapter-opts
func (h *Handlers) AdapterSchemaHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if h.Gateway == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "GATEWAY_NOT_READY", "网关未就绪")
		return
	}
	schema := h.Gateway.AdapterSchema(name)
	if schema == nil {
		writeErrorCode(w, http.StatusNotFound, "MODEL_NO_ADAPTER", "模型 "+name+" 无适配器（走旧路由——不可编辑）")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"model":  name,
		"schema": schema,
		"note":   "每个模型的适配器参数集各自不同——编辑后实时生效（不重启）——持久化重启恢复",
	})
}

// UpdateAdapterOptionsHandler 更新适配器配置（Mr2109 2026-08-27——实时生效）
// PUT /api/models/{name}/adapter-opts  body: {"temperature": 0.8, "reasoning_effort": "high"}
func (h *Handlers) UpdateAdapterOptionsHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if h.Gateway == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "GATEWAY_NOT_READY", "网关未就绪")
		return
	}
	var cfg map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "INVALID_PARAMS", "参数解析失败: "+err.Error())
		return
	}
	if len(cfg) == 0 {
		writeErrorCode(w, http.StatusBadRequest, "NO_PARAMS", "无参数提交")
		return
	}
	if err := h.Gateway.UpdateAdapterOptions(name, cfg); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "ADAPTER_OPTIONS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"model":   name,
		"updated": cfg,
		"message": "适配器配置已更新——实时生效（后续请求即用新参数）",
	})
}

// LogsHandler 处理 POST /api/fleet/logs——接收子端日志上报（v1 agent 兼容）。
func (h *Handlers) LogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 POST 方法")
		return
	}
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()
	// v1 agent 上报格式: {machine, level, component, message, timestamp, request_id}
	var logEntry map[string]interface{}
	if err := json.Unmarshal(body, &logEntry); err != nil {
		// 非 JSON 也接收（简单日志行）
		logEntry = map[string]interface{}{"raw": string(body)}
	}
	// 打印到主控日志（聚合展示可后续扩展存储）
	machine, _ := logEntry["machine"].(string)
	level, _ := logEntry["level"].(string)
	msg, _ := logEntry["message"].(string)
	println("子端日志: machine=", machine, " level=", level, " msg=", msg)

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// TasksHandler 处理 GET /api/fleet/tasks —— 返回任务列表（初始为空）。
func (h *Handlers) TasksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorCode(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "仅支持 GET 方法")
		return
	}

	tasks := h.Store.GetTasks()

	response := map[string]interface{}{
		"tasks": tasks,
		"count": len(tasks),
	}

	writeJSON(w, http.StatusOK, response)
}

// ReloadConfigHandler 热加载配置（B11：加模型不用重启主控）。
// POST /api/config/reload — 重读 fleet.yaml，更新路由表；已加载模型不受影响。
func (h *Handlers) ReloadConfigHandler(w http.ResponseWriter, r *http.Request) {
	if h.ConfigPath == "" {
		writeErrorCode(w, http.StatusBadRequest, "CONFIG_PATH_MISSING", "config path 未配置")
		return
	}
	newCfg, err := config.LoadFleetConfig(h.ConfigPath)
	if err != nil {
		writeErrorCode(w, http.StatusInternalServerError, "CONFIG_LOAD_FAILED", "配置加载失败: "+err.Error())
		return
	}
	oldModelCount := len(h.Config.Models)
	h.Config = newCfg
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"models":      len(newCfg.Models),
		"added":       len(newCfg.Models) - oldModelCount,
		"fleet_nodes": len(newCfg.Fleet),
		// 效果指纹（§九 M7 `E4` / §九 M19 `RC10`）：让「日志那一行」与「这次响应」
		// 靠**稳定摘要**对上，不靠人眼比对时间戳。算法取 `audit.ArgsFingerprint`（一处算法）。
		"fingerprint": controlFingerprint("config_reload", map[string]any{
			"config_path": h.ConfigPath,
			"models":      len(newCfg.Models),
			"fleet_nodes": len(newCfg.Fleet),
		}),
	})
}

// TaskTerminateHandler 终止执行中任务（右键功能——2026-08-22 Mr2109）
// POST /api/tasks/{id}/terminate
func (h *Handlers) TaskTerminateHandler(w http.ResponseWriter, r *http.Request) {
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	taskID := chi.URLParam(r, "id")
	if err := h.Scheduler.TerminateTask(taskID); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "TASK_TERMINATE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "任务已终止"})
}

// TaskRequeueHandler 执行中任务重回队列（右键功能——2026-08-22 Mr2109）
// POST /api/tasks/{id}/requeue
func (h *Handlers) TaskRequeueHandler(w http.ResponseWriter, r *http.Request) {
	if h.Scheduler == nil {
		writeErrorCode(w, http.StatusServiceUnavailable, "SCHEDULER_NOT_STARTED", "总调度器未启动")
		return
	}
	taskID := chi.URLParam(r, "id")
	if err := h.Scheduler.RequeueTask(taskID); err != nil {
		writeErrorCode(w, http.StatusBadRequest, "TASK_REQUEUE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "message": "任务已重回队列"})
}
