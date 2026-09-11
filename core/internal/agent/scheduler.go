package agent

// scheduler.go — v2.5.2 调度器：异步派单引擎
// 设计：P0 — 轮询扫描 → 优先级排序 → 异步派单 → 冷却 → 死信 → 并发控制
// 核心原则：永不等（worker 卡了只影响自己）

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// 常量

const (
	defaultMaxWorkers = 2                     // 默认并发上限
	defaultCooldown   = 30 * time.Second      // 冷却时间（失败后重派最小间隔）
	maxRetries        = 3                     // 最大重试次数（超过标记 dead）
	pollInterval      = 5 * time.Second       // 轮询间隔
	workersConfigPath = "config/workers.yaml" // 并发配置文件
)

// Priority — 任务优先级（调度器排序用）
type Priority int

const (
	PriorityLow    Priority = iota // low
	PriorityNormal                 // normal
	PriorityHigh                   // high
)

// String — 优先级字符串
func (p Priority) String() string {
	switch p {
	case PriorityHigh:
		return "high"
	case PriorityNormal:
		return "normal"
	case PriorityLow:
		return "low"
	default:
		return "normal"
	}
}

// ParsePriority — 解析优先级字符串
func ParsePriority(s string) Priority {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high", "h":
		return PriorityHigh
	case "low", "l":
		return PriorityLow
	default:
		return PriorityNormal
	}
}

// PriorityValue — 优先级排序值（数字越小越优先）
func (p Priority) Value() int {
	switch p {
	case PriorityHigh:
		return 0
	case PriorityNormal:
		return 1
	case PriorityLow:
		return 2
	default:
		return 1
	}
}

// Issue 解析

// ParsedIssue — 调度器视角的 issue（从 markdown 文件解析）
type ParsedIssue struct {
	Path        string    // 文件绝对路径
	InstanceID  string    // 从文件名提取
	Status      string    // 当前状态
	Priority    Priority  // 优先级
	RetryCount  int       // 当前重试次数
	LastAttempt time.Time // 最后尝试时间
	CreatedAt   time.Time // 创建时间
	Content     string    // 原始 markdown 内容
}

// 并发配置

// WorkersConfig — config/workers.yaml 配置
type WorkersConfig struct {
	MaxWorkers int `yaml:"max_workers"`
}

// LoadWorkersConfig — 加载 workers.yaml 配置（v2.5.2 设备感知版）
// 1. 尝试读取 config/workers.yaml
// 2. 文件不存在 → 按 hostname 匹配内置默认值（mini1=1/X3=3/local=2）
// 3. 解析失败 → 回退到内置默认值
// 4. max_workers <= 0 → 回退默认值
func LoadWorkersConfig(workDir string) WorkersConfig {
	// 尝试读取配置文件
	cfg, err := loadDeviceMaxWorkers(workDir)
	if err != nil {
		// 配置文件加载失败 → 用内置默认值
		return WorkersConfig{MaxWorkers: resolveByHostname(defaultDeviceMap, defaultMaxWorkers)}
	}

	// 获取当前 hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}

	// 按 hostname 匹配设备配置
	mw := resolveByDevice(cfg.Devices, hostname, defaultDeviceMap, defaultMaxWorkers)
	if mw <= 0 {
		return WorkersConfig{MaxWorkers: defaultMaxWorkers}
	}
	return WorkersConfig{MaxWorkers: mw}
}

// 调度器

// WorkerRunner — worker 执行函数（可注入 mock——测试用——生产默认 exec zerg-agent）
// 返回退出码（0=成功）
type WorkerRunner func(issuePath string) int

// Scheduler — 任务调度器
type Scheduler struct {
	mu           sync.Mutex
	workDir      string
	maxWorkers   int
	polling      bool
	cancelPoll   context.CancelFunc
	activeCount  int                  // 当前活跃 worker 数
	lastDispatch map[string]time.Time // 上次派单时间（issue path → 时间）
	wg           sync.WaitGroup       // 等待所有 worker 退出
	doneCh       chan struct{}        // 停止信号
	runner       WorkerRunner         // v2.5.2 可注入 runner（测试 mock——生产默认 exec）
	logger       *Logger              // v2.5.4.9 结构化日志（可选——nil 不记录）
	// v2.5.3 容器模式（CA 脱离沙箱——docker run 完整环境）
	ContainerMode     bool   // true=容器内执行 worker（默认 false=exec 兼容）
	ContainerAgentBin string // Linux zerg-agent 路径（容器内挂载）
	DevMode           bool   // C4: 开发模式（容器内分支开发→commit→PR——主仓库=workDir）
	RepoDir           string // v2.5.3: 开发仓库路径（纯代码——git管理——DevMode用）
}

// NewScheduler — 创建调度器
func NewScheduler(workDir string) *Scheduler {
	cfg := LoadWorkersConfig(workDir)
	return &Scheduler{
		workDir:      workDir,
		maxWorkers:   cfg.MaxWorkers,
		lastDispatch: make(map[string]time.Time),
		doneCh:       make(chan struct{}),
	}
}

// 核心：扫描

// ScanIssues — 扫描 docs/issues/ 下所有 open/queued 状态的 issue
// 按优先级排序返回（高优先在前）
func (s *Scheduler) ScanIssues() ([]ParsedIssue, error) {
	issuesDir := filepath.Join(s.workDir, "docs", "issues")
	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 目录不存在 = 无 issue
		}
		return nil, fmt.Errorf("failed to scan issue directory: %w", err)
	}

	var issues []ParsedIssue
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(issuesDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // 跳过无法读取的文件
		}
		issue := parseIssue(path, string(data))
		if issue == nil {
			continue
		}
		// 只返回 open/queued 状态的
		if issue.Status == StatusOpen || issue.Status == StatusQueued {
			issues = append(issues, *issue)
		}
	}

	// 按优先级排序（高优先在前，同优先级按创建时间早的在前）
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Priority != issues[j].Priority {
			return issues[i].Priority.Value() < issues[j].Priority.Value()
		}
		return issues[i].CreatedAt.Before(issues[j].CreatedAt)
	})

	return issues, nil
}

// parseIssue — 从 markdown 内容解析 issue
func parseIssue(path, content string) *ParsedIssue {
	// 提取 instance_id（文件名去掉 .md）
	base := filepath.Base(path)
	instanceID := strings.TrimSuffix(base, ".md")

	// 提取状态
	status := extractField(content, "- **状态**:")
	if !IsValidStatus(status) {
		return nil
	}

	// 提取优先级（从 HTML comment 或字段）
	priorityStr := extractPriority(content)
	priority := ParsePriority(priorityStr)

	// 提取重试次数
	retryCount := extractRetryCount(content)

	// 提取时间
	createdAt := extractTimeField(content, "创建时间")
	lastAttempt := extractTimeField(content, "最后尝试")

	return &ParsedIssue{
		Path:        path,
		InstanceID:  instanceID,
		Status:      status,
		Priority:    priority,
		RetryCount:  retryCount,
		LastAttempt: lastAttempt,
		CreatedAt:   createdAt,
		Content:     content,
	}
}

// extractField — 提取 markdown 字段值
func extractField(content, prefix string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

// extractPriority — 提取优先级（从 <!-- priority: x --> 或 - **优先级**:)
func extractPriority(content string) string {
	// 先从 HTML comment 提取
	for _, line := range strings.Split(content, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "priority:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				// 去掉 HTML comment 尾巴（-->）和空格
				v := strings.TrimSpace(parts[1])
				v = strings.TrimSuffix(v, "-->")
				return strings.TrimSpace(v)
			}
		}
	}
	// 再从字段提取
	return extractField(content, "- **优先级**:")
}

// extractRetryCount — 提取重试次数
func extractRetryCount(content string) int {
	s := extractField(content, "- **重试次数**:")
	if s == "" {
		return 0
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// extractTimeField — 提取 RFC3339 时间字段
func extractTimeField(content, prefix string) time.Time {
	s := extractField(content, prefix)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// 核心：派单

// dispatchIssue — 异步派单（启动 worker 子进程，不等完成）
// 返回: error（派单失败），nil（派单成功，worker 在后台跑）
func (s *Scheduler) dispatchIssue(issue ParsedIssue) error {
	// 1. 并发限制检查
	s.mu.Lock()
	if s.activeCount >= s.maxWorkers {
		s.mu.Unlock()
		log.Printf("⏳ concurrency full: %d/%d, skipping %s", s.activeCount, s.maxWorkers, issue.InstanceID)
		return fmt.Errorf("concurrency full (%d/%d), skipping %s", s.activeCount, s.maxWorkers, issue.InstanceID)
	}
	s.activeCount++
	s.mu.Unlock()
	log.Printf("🐛 [scheduler] dispatched: %s (priority=%d, active=%d/%d)", issue.InstanceID, issue.Priority, s.activeCount, s.maxWorkers)

	// 2. 派单前标记 running（状态机即锁——防抢——open→queued 已在扫描排队——派单=开始执行→running）
	targetStatus := StatusRunning
	var newContent string
	var err error
	if issue.Status == StatusOpen {
		// open 直接派（未排队）→ 先 queued 再 running（两步——状态机要求）
		queuedContent, err := TransitionIssue(issue.Content, StatusOpen, StatusQueued)
		if err != nil {
			s.mu.Lock()
			s.activeCount--
			s.mu.Unlock()
			return fmt.Errorf("failed to mark queued: %w", err)
		}
		newContent, err = TransitionIssue(queuedContent, StatusQueued, StatusRunning)
	} else {
		newContent, err = TransitionIssue(issue.Content, issue.Status, targetStatus)
	}
	if err != nil {
		// 兼容：已经是 queued 的（重派）直接继续
		if issue.Status != StatusQueued {
			s.mu.Lock()
			s.activeCount--
			s.mu.Unlock()
			return fmt.Errorf("failed to mark running: %w", err)
		}
		newContent = issue.Content
	}
	if newContent != issue.Content {
		if err := os.WriteFile(issue.Path, []byte(newContent), 0o644); err != nil {
			s.mu.Lock()
			s.activeCount--
			s.mu.Unlock()
			return fmt.Errorf("failed to write running state: %w", err)
		}
	}

	// 3. 启动 worker 子进程（异步——不等待）
	issuePath := issue.Path
	// 派单即记冷却（v2.5.2——防连续重派——不管成败）
	s.mu.Lock()
	s.lastDispatch[issuePath] = time.Now()
	// v2.5.5: 并发安全修复——读取 runner 时必须持有 mu 锁
	// 防止 SetRunner() 与 dispatchIssue() 并发时产生数据竞争
	runner := s.runner
	s.mu.Unlock()
	go s.runWorker(issuePath, runner)

	return nil
}

// runWorker — 运行 worker（后台 goroutine——runner 可 mock）
// v2.5.3: ContainerMode=true 时用 DockerWorker（容器内完整执行——脱离沙箱）
func (s *Scheduler) runWorker(issuePath string, runner WorkerRunner) {
	defer func() {
		s.mu.Lock()
		s.activeCount--
		s.mu.Unlock()
	}()

	exitCode := 0
	if runner != nil {
		// mock/注入 runner（测试用）
		exitCode = runner(issuePath)
	} else if s.ContainerMode {
		// v2.5.3 容器模式：docker run zerg-dev——容器内跑 zerg-agent
		dw := NewDockerWorker("", "")
		dw.agentBin = s.ContainerAgentBin
		if dw.agentBin == "" {
			dw.agentBin = "/tmp/zerg-agent-linux-arm64" // 默认 Linux 版（交叉编译）
		}
		var err error
		if s.DevMode {
			// C4 开发模式：容器内分支开发 → commit → PR（repoDir=代码仓库——workDir=任务区）
			task := readIssueTask(issuePath)
			exitCode, err = dw.RunDev(s.RepoDir, s.workDir, task, "task-"+baseName(issuePath))
		} else {
			exitCode, err = dw.Run(issuePath, s.workDir)
		}
		if err != nil {
			log.Printf("⚠️ DockerWorker failed %s: %v", issuePath, err)
			exitCode = -1
		} else {
			log.Printf("✅ DockerWorker succeeded: %s", issuePath)
		}
	} else {
		// 生产默认：exec zerg-agent
		// 2026-09-05 止血: 补 -json（2026-08-21 铁律——spawn CA 必带 json 退出码语义——
		// master_scheduler.go 313 行修了——本 issue 路径漏了——模型失败退出 0=假成功依然存在）
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "zerg-agent", "-json", "-issue", issuePath)
		cmd.Dir = s.workDir

		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		} else {
			log.Printf("✅ zerg-agent succeeded: %s (exit code: 0)", issuePath)
		}
		log.Printf("[scheduler] cmd output (stdout): %s", stdout.String())
		log.Printf("[scheduler] cmd output (stderr): %s", stderr.String())
	}

	// 4. 根据结果处理
	if exitCode == 0 {
		// 成功：标记 verified → done
		s.handleSuccess(issuePath)
		log.Printf("✅ [scheduler] issue processed: %s (exit code: 0)", issuePath)
	} else {
		// 失败：冷却 + 重试/死信
		s.handleFailure(issuePath, exitCode, "")
		log.Printf("❌ [scheduler] issue processing failed: %s (exit code: %d)", issuePath, exitCode)
	}
}

// baseName — 取文件基名（无扩展名）
func baseName(path string) string {
	b := filepath.Base(path)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

// readIssueTask — 读 issue 任务描述（- **任务**: xxx）
func readIssueTask(issuePath string) string {
	data, err := os.ReadFile(issuePath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "- **任务**:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "- **任务**:"))
		}
	}
	return ""
}

// SetRunner — 注入 worker runner（测试 mock——生产不需要调）
func (s *Scheduler) SetRunner(r WorkerRunner) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runner = r
}

// handleSuccess — 成功处理（running → verified → done）
func (s *Scheduler) handleSuccess(issuePath string) {
	data, err := os.ReadFile(issuePath)
	if err != nil {
		log.Printf("⚠️ failed to read issue %s: %v", issuePath, err)
		return
	}
	content := string(data)

	// running → verified
	newContent, err := TransitionIssue(content, StatusRunning, StatusVerified)
	if err != nil {
		log.Printf("⚠️ failed to mark verified %s: %v", issuePath, err)
		return
	}

	// verified → waiting_review（C4：验收通过——等脑确认复查——不自动 done）
	newContent, err = TransitionIssue(newContent, StatusVerified, StatusWaitReview)
	if err != nil {
		// 兼容：verified→waiting_review 不支持则保持 verified
		log.Printf("⚠️ failed to mark waiting_review %s: %v (staying verified)", issuePath, err)
	}

	if newContent != content {
		if err := os.WriteFile(issuePath, []byte(newContent), 0o644); err != nil {
			log.Printf("⚠️ failed to write done state %s: %v", issuePath, err)
		}
	}
}

// handleFailure — 失败处理（冷却 + 重试/死信）
func (s *Scheduler) handleFailure(issuePath string, exitCode int, stderr string) {
	data, err := os.ReadFile(issuePath)
	if err != nil {
		log.Printf("⚠️ failed to read issue %s: %v", issuePath, err)
		return
	}
	content := string(data)

	// 解析当前重试次数
	issue := parseIssue(issuePath, content)
	if issue == nil {
		return
	}

	retryCount := issue.RetryCount + 1

	// 更新重试次数和时间
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "- **重试次数**:") {
			lines[i] = fmt.Sprintf("- **Retry count**: %d", retryCount)
		}
		if strings.HasPrefix(line, "- **最后尝试**:") {
			lines[i] = fmt.Sprintf("- **Last attempt**: %s", time.Now().Format(time.RFC3339))
		}
	}
	content = strings.Join(lines, "\n")

	// 重试次数 ≥ maxRetries → 死信
	if retryCount >= maxRetries {
		newContent, terr := CanDead(content, StatusRunning)
		if terr != nil {
			log.Printf("⚠️ failed to mark dead %s: %v", issuePath, terr)
		} else {
			if newContent != content {
				if err := os.WriteFile(issuePath, []byte(newContent), 0o644); err != nil {
					log.Printf("⚠️ failed to write dead letter %s: %v", issuePath, err)
				}
			}
		}
		log.Printf("📛 dead letter: %s (retries %d reached limit %d)", issuePath, retryCount, maxRetries)
		return
	}

	// 冷却检查：失败 ≥ cooldown 才重派
	s.mu.Lock()
	lastTime, existed := s.lastDispatch[issuePath]
	s.mu.Unlock()

	if existed && time.Since(lastTime) < defaultCooldown {
		// 冷却中——但先查死信（重试 ≥ maxRetries → dead——不受冷却影响——防无限循环）
		if retryCount >= maxRetries {
			newContent, terr := CanDead(content, StatusRunning)
			if terr != nil {
				log.Printf("⚠️ failed to mark dead during cooldown %s: %v", issuePath, terr)
			} else if newContent != content {
				if err := os.WriteFile(issuePath, []byte(newContent), 0o644); err != nil {
					log.Printf("⚠️ failed to write dead letter during cooldown %s: %v", issuePath, err)
				}
			}
			log.Printf("📛 dead letter during cooldown: %s (retries %d reached limit %d — preventing infinite loop)", issuePath, retryCount, maxRetries)
			return
		}
		// 未达死信——标记 queued（排队等重派——不保持 running——否则扫描跳过）
		q, err := TransitionIssue(content, StatusRunning, StatusRetry)
		if err != nil {
			log.Printf("⚠️ 冷却中标记 retry 失败 %s: %v", issuePath, err)
			return
		}
		q, err = TransitionIssue(q, StatusRetry, StatusQueued)
		if err != nil {
			log.Printf("⚠️ 冷却中标记 queued 失败 %s: %v", issuePath, err)
			return
		}
		if err := os.WriteFile(issuePath, []byte(q), 0o644); err != nil {
			log.Printf("⚠️ failed to write queued %s: %v", issuePath, err)
			return
		}
		log.Printf("❄️ cooling down: %s (since last %v < %v — queued for re-dispatch)", issuePath, time.Since(lastTime), defaultCooldown)
		return
	}

	// 冷却结束 / 首次失败 → 标记 retry → queued
	newContent, err := TransitionIssue(content, StatusRunning, StatusRetry)
	if err != nil {
		log.Printf("⚠️ failed to mark retry %s: %v", issuePath, err)
		return
	}
	newContent, err = TransitionIssue(newContent, StatusRetry, StatusQueued)
	if err != nil {
		log.Printf("⚠️ failed to mark queued %s: %v", issuePath, err)
		return
	}

	// 更新文件
	if newContent != content {
		if err := os.WriteFile(issuePath, []byte(newContent), 0o644); err != nil {
			log.Printf("⚠️ failed to write queued state %s: %v", issuePath, err)
			return
		}
	}

	// 记录派单时间
	s.mu.Lock()
	s.lastDispatch[issuePath] = time.Now()
	s.mu.Unlock()

	log.Printf("🔄 re-dispatched: %s (retry %d)", issuePath, retryCount)
}

// 核心：轮询循环

// Start — 启动调度器（阻塞直到 Stop）
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.polling {
		s.mu.Unlock()
		return
	}
	s.polling = true
	ctx, cancel := context.WithCancel(ctx)
	s.cancelPoll = cancel
	s.mu.Unlock()

	log.Printf("🚀 scheduler started (max_workers=%d, cooldown=%v, max_retries=%d)",
		s.maxWorkers, defaultCooldown, maxRetries)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// 立即执行一轮
	s.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 scheduler received stop signal")
			s.wg.Wait() // 等待所有 worker 退出
			s.mu.Lock()
			s.polling = false
			s.mu.Unlock()
			return
		case <-ticker.C:
			s.poll(ctx)
		case <-s.doneCh:
			log.Println("🛑 scheduler stopped actively")
			s.wg.Wait()
			s.mu.Lock()
			s.polling = false
			s.mu.Unlock()
			return
		}
	}
}

// Stop — 停止调度器
func (s *Scheduler) Stop() {
	close(s.doneCh)
	if s.cancelPoll != nil {
		s.cancelPoll()
	}
}

// poll — 单次轮询：扫描 → 排序 → 派单
func (s *Scheduler) poll(ctx context.Context) {
	// 检查是否被取消
	select {
	case <-ctx.Done():
		return
	default:
	}

	issues, err := s.ScanIssues()
	if err != nil {
		log.Printf("⚠️ failed to scan issues: %v", err)
		return
	}

	if len(issues) == 0 {
		return // 无待派单 issue
	}

	log.Printf("📋 scanned %d issue(s) pending dispatch", len(issues))

	for _, issue := range issues {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 并发限制
		s.mu.Lock()
		if s.activeCount >= s.maxWorkers {
			s.mu.Unlock()
			log.Printf("⏸️ concurrency full (%d/%d), skipping %d remaining issue(s) this round",
				s.activeCount, s.maxWorkers, len(issues))
			return // 本轮不再派单，等下一轮
		}
		s.mu.Unlock()

		// 检查冷却
		s.mu.Lock()
		lastTime, existed := s.lastDispatch[issue.Path]
		s.mu.Unlock()

		if existed && time.Since(lastTime) < defaultCooldown {
			log.Printf("❄️ skipping due to cooldown: %s", issue.InstanceID)
			continue
		}

		if err := s.dispatchIssue(issue); err != nil {
			log.Printf("⚠️ dispatch failed %s: %v", issue.InstanceID, err)
		}
	}
}

// ActiveCount — 当前活跃 worker 数（调试用）
func (s *Scheduler) ActiveCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeCount
}

// MaxWorkers — 当前并发上限
func (s *Scheduler) MaxWorkers() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxWorkers
}
