package agent

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ============ v2.5.5 T3 内部任务引擎——最小实现 ============
// 空闲检测 → 选内部任务 → 建 issue → 调度器派 CA → 脑检查
// Mr2109: 内部任务 = 进化（为自己）——编排自动运行——不影响外部任务

// InternalTask 内部任务定义
type InternalTask struct {
	ID          string        // 任务类型 ID（如 "tool-check"）
	Description string        // 人类可读描述
	Template    string        // 任务描述模板（派 CA 用——含工作区/要求）
	Cooldown    time.Duration // 冷却时间（同类型防重复）
}

// ListInternalTasks 导出内部任务清单（2026-08-22 Mr2109——UI 分类显示）
func ListInternalTasks() []InternalTask {
	return internalTaskDefs
}

// 预定义内部任务清单（第一批——4 类——2026-08-18 Mr2109：测试期不间断运行）
// 优先级逻辑: 防瘫（系统健康）→ 保底（代码质量）→ 积累（知识沉淀）→ 进化（工具丰富）
var internalTaskDefs = []InternalTask{
	{
		ID:          "health-check",
		Description: "系统健康巡检——残留进程/内存/模型驻留（防瘫——凌晨教训）",
		Template:    "检查系统健康。工作区=项目core。要求: 1. 查X3残留llama-server进程(ssh g01@<worker-ip> ps aux|grep llama-server——超过1个=残留) 2. 查X3内存(free -g——used超过90%=告警) 3. 查模型驻留(加新卸老——违反=告警) 4. 发现异常→记录到 internal-health-report.md 5. 贴输出。注意: 工作区=项目core——相对路径——真查真记录。有异常才处理——正常就报告健康",
		Cooldown:    6 * time.Hour, // 健康巡检高频（防瘫）
	},
	{
		ID:          "tool-check",
		Description: "工具库检查——未用工具/依赖清理",
		Template:    "检查项目工具库和依赖。工作区=项目core。要求: 1. grep 未用的工具/函数 2. go mod tidy 检查依赖 3. 发现可清理项→清理 4. 报告到 internal-task-report.md 5. 贴输出。注意: 工作区=项目core——相对路径——真检查真分析",
		Cooldown:    24 * time.Hour,
	},
	{
		ID:          "code-quality",
		Description: "代码质量检查——测试覆盖/死代码",
		Template:    "检查代码质量。工作区=项目core。要求: 1. go test 看测试覆盖 2. 找漏测模块 3. 找死代码(未用函数/变量) 4. 发现可修复→修复 5. 报告到 internal-task-report.md 6. 贴输出。注意: 工作区=项目core——相对路径——真检查真分析",
		Cooldown:    24 * time.Hour,
	},
	{
		ID:          "knowledge-digest",
		Description: "经验沉淀——最近任务经验提炼",
		Template:    "提炼最近任务的经验。工作区=项目core。要求: 1. 读 .zerg/logs/ 最近任务跟踪数据 2. 提炼经验教训 3. 输出到 internal-task-report.md(结构化——现象/根因/修复/教训) 4. 贴输出。注意: 工作区=项目core——相对路径——真读真提炼",
		Cooldown:    24 * time.Hour,
	},
	// === 第二批（v2.5.5 稳定期扩展——全部配置）===
	{
		ID:          "proc-cleanup",
		Description: "残留进程自动清理——X3 llama-server 残留（防瘫）",
		Template:    "清理X3残留进程。工作区=项目core。要求: 1. ssh g01@<worker-ip> ps aux|grep llama-server 2. 超过1个=残留——启动时间早于agent重启=残留(防PID复用) 3. 残留→SIGTERM等5s→仍活SIGKILL 4. 记录到 internal-task-report.md 5. 贴输出。注意: 不误杀正在跑的(agent刚启动的=在用)——只清残留",
		Cooldown:    6 * time.Hour,
	},
	{
		ID:          "mem-disk-alert",
		Description: "内存/磁盘告警自动处理",
		Template:    "检查X3和本机内存磁盘。工作区=项目core。要求: 1. 查X3内存(ssh g01@<worker-ip> free -g——used>90%=告警) 2. 查本机内存(free -g) 3. 查X3磁盘(df -h /data——>90%=告警) 4. 告警→清理(日志/tmp/模型缓存) 5. 记录到 internal-task-report.md 6. 贴输出。注意: 真查真处理——正常就报告健康",
		Cooldown:    6 * time.Hour,
	},
	{
		ID:          "circuit-inspect",
		Description: "熔断状态巡检——异常熔断诊断",
		Template:    "巡检熔断状态。工作区=项目core。要求: 1. 查主控日志熔断记录(grep /tmp/zerg-core.log) 2. 熔断计数异常(频繁)=诊断——查X3健康/网络 3. 发现根因→记录到 internal-task-report.md 4. 贴输出。注意: 真查真分析——正常就报告无异常",
		Cooldown:    24 * time.Hour,
	},
	{
		ID:          "test-coverage",
		Description: "测试覆盖补全——核心模块补测",
		Template:    "补全测试覆盖。工作区=项目core。要求: 1. go test -cover 看各包覆盖 2. 找覆盖<60%的核心模块 3. 补核心测试(真实断言——非空测试) 4. go test 全过 5. 报告到 internal-task-report.md(覆盖前后对比) 6. 贴输出。注意: 工作区=项目core——真补真测",
		Cooldown:    48 * time.Hour,
	},
	{
		ID:          "deadcode-clean",
		Description: "死代码清理——未用函数/变量",
		Template:    "清理死代码。工作区=项目core。要求: 1. go vet/静态分析 2. 找未用函数/变量(grep确认无引用) 3. 确认安全→删除 4. go build + go test 过 5. 报告到 internal-task-report.md 6. 贴输出。注意: 工作区=项目core——真查真删——删前确认无引用",
		Cooldown:    48 * time.Hour,
	},
	{
		ID:          "dep-slim",
		Description: "依赖精简——未用依赖移除",
		Template:    "精简依赖。工作区=项目core。要求: 1. go mod tidy 2. 看未用依赖(grep确认无引用) 3. 确认安全→移除 4. go build 过 5. 报告到 internal-task-report.md(依赖前后对比) 6. 贴输出。注意: 工作区=项目core——真查真精简",
		Cooldown:    48 * time.Hour,
	},
	// === 第三批（进化引擎——全部配置）===
	{
		ID:          "kb-digest",
		Description: "任务经验自动入库——知识库沉淀",
		Template:    "提炼经验入库。工作区=项目core。要求: 1. 读最近任务报告(docs/issues/ 或 .zerg/logs/) 2. 提炼经验教训(现象/根因/修复/教训) 3. 写入知识库(kb_add——learned域——用MCP工具knowledge_kb) 4. 报告到 internal-task-report.md(入库记录) 5. 贴输出。注意: 真读真提炼——每条经验结构化",
		Cooldown:    24 * time.Hour,
	},
	{
		ID:          "capture-fix",
		Description: "排障经验结构化——capture_fix",
		Template:    "结构化排障经验。工作区=项目core。要求: 1. 读最近修复记录(git log 或 docs/issues/) 2. 提炼排障过程(现象/根因/修复/教训) 3. 结构化写入 knowledge 4. 报告到 internal-task-report.md 5. 贴输出。注意: 真读真提炼——知识库可检索",
		Cooldown:    24 * time.Hour,
	},
	{
		ID:          "lesson-list",
		Description: "教训清单维护——防再犯",
		Template:    "维护教训清单。工作区=项目core。要求: 1. 读最近任务失败记录(git log/docs/issues/) 2. 找反复出现的错误 3. 更新教训清单(新增教训——防再犯) 4. 报告到 internal-task-report.md 5. 贴输出。注意: 真读真分析——教训具体可执行",
		Cooldown:    48 * time.Hour,
	},
	{
		ID:          "doc-consistency",
		Description: "文档一致性检查——防项目失忆（Mr2109）",
		Template:    "检查文档一致性。工作区=项目core。要求: 1. 扫描关键文档(README/设计/CHANGELOG) 2. 对照代码(组件/接口/功能——核心模块) 3. 发现不一致→记录/修复 4. 报告到 internal-task-report.md(列出实际不一致或已修复) 5. 贴输出。注意: 工作区=项目core——真读真对照——文档脱节=项目失忆",
		Cooldown:    24 * time.Hour,
	},
	{
		ID:          "resource-grow",
		Description: "资源增长引擎——真正创建工具/skill/MCP（Mr2109核心——无冷却——持续进化）",
		Template:    "调研吸收优秀资源并真正创建。工作区=项目core。要求: 1. 调研开源社区(VoltAgent/awesomeclaude/microsoft agent-skills/awesome-mcp等) 2. 判断有用性(开源后使用者觉得有用=有用——普适/可复制/文档化/生态位/未来价值) 3. 设计适配(对虫族有用吗——吸收精华) 4. 真正创建（必须——Mr21092026-08-20）: 工具→接入core/internal/agent/tools.go的DefaultTools+exec.go分发; skill→创建目录+SKILL.md; MCP→创建配置/服务 5. 验证(真实任务测试——UI资源库能扫到: curl http://127.0.0.1:8580/api/resources/tools|skills|mcp 出现新资源) 6. 登记(tools/资源登记.md——更新清单) 7. 报告到 internal-task-report.md(创建了什么/如何用/验证结果——含curl输出) 8. 贴输出。注意: 工作区=项目core——真创建真入库——只调研不创建=任务失败——每轮至少创建1个资源——这是核心引擎——持续进化不停",
		Cooldown:    0, // Mr2109: 无冷却——资源增长引擎持续跑（进化核心）
	},
	{
		ID:          "model-eval",
		Description: "模型评估——任务 git 打分",
		Template:    "评估模型表现。工作区=项目core。要求: 1. 读最近任务git(worktree分支/任务记录) 2. 按7维度打分(完成度/代码质量/验证/效率/成功率/自主性/协作) 3. 输出到 internal-task-report.md(打分表——各模型对比) 4. 贴输出。注意: 工作区=项目core——真读真打分——数据支撑",
		Cooldown:    72 * time.Hour,
	},
}

// IdleDetector 空闲检测器——定时检查外部任务队列 + 资源空闲
type IdleDetector struct {
	mu            sync.Mutex
	issueDir      string                                   // 任务单目录（docs/issues/）
	lastTriggered map[string]time.Time                     // 任务类型 → 上次触发时间（冷却）
	externalQueue func() int                               // 外部任务队列长度（注入——测试用）
	resourceIdle  func() bool                              // 资源是否空闲（注入——X3 GPU idle）
	onTrigger     func(def InternalTask, issuePath string) // 触发回调（v2.5.5 测试6——触发后调总调度器 Submit——闭环；P1-3 带任务单路径）
	autoCheck     func(defID string) bool                  // 运行模式检查（注入——api.IsInternalAuto——手动任务不自动触发——2026-08-28 Mr2109）
	enabled       bool
}

// SetAutoCheck 注入运行模式检查（main.go 注入 api.IsInternalAuto——避免 agent→api 循环依赖）
func (d *IdleDetector) SetAutoCheck(f func(defID string) bool) { d.autoCheck = f }

// NewIdleDetector 创建空闲检测器
func NewIdleDetector(issueDir string) *IdleDetector {
	d := &IdleDetector{
		issueDir:      issueDir,
		lastTriggered: make(map[string]time.Time),
		enabled:       true,
	}
	// v2.5.5 断点续跑（Mr2109）: 加载上次触发状态——重启后从暂停处继续（不从头重复）
	d.loadIdleState()
	return d
}

// SetExternalQueue 注入外部队列检查函数
func (d *IdleDetector) SetExternalQueue(f func() int) { d.externalQueue = f }

// SetResourceIdle 注入资源空闲检查函数
func (d *IdleDetector) SetResourceIdle(f func() bool) { d.resourceIdle = f }

// SetOnTrigger 注入触发回调（v2.5.5 测试6——触发后调总调度器 Submit——闭环）
// v2.5.5 P1-3: 回调带 issuePath（任务单路径——总调度器完成时更新状态）
func (d *IdleDetector) SetOnTrigger(f func(InternalTask, string)) { d.onTrigger = f }

// SetEnabled 开关（外部任务多时禁用）
func (d *IdleDetector) SetEnabled(on bool) { d.enabled = on }

// Run 启动空闲检测循环（每 5 分钟检查一次）
func (d *IdleDetector) Run(stop <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	log.Printf("🕐 Internal task engine started — idle check every 5 minutes (v2.5.5 T3)")
	for {
		select {
		case <-stop:
			log.Printf("🕐 Internal task engine stopped")
			return
		case <-ticker.C:
			d.checkAndTrigger()
		}
	}
}

// checkAndTrigger 检查空闲 + 触发内部任务
func (d *IdleDetector) checkAndTrigger() {
	if !d.enabled {
		return
	}
	// 外部任务队列空（无外部任务在跑）
	if d.externalQueue != nil && d.externalQueue() > 0 {
		log.Printf("🕐 External tasks running (%d) — internal tasks yielding", d.externalQueue())
		return
	}
	// 资源空闲（X3/本机 GPU idle）
	if d.resourceIdle != nil && !d.resourceIdle() {
		log.Printf("🕐 Resources busy — internal tasks waiting for idle")
		return
	}

	// 空闲 → 选一个内部任务（冷却期内跳过；手动运行任务跳过——Mr2109 2026-08-28）
	lastIssuePath := "" // v2.5.5 P1-3: 本次触发的任务单路径（传给总调度器——完成时更新状态）
	for _, def := range internalTaskDefs {
		// v2.5.6 运行模式开关: 手动运行任务——空闲检测跳过（只手动触发）
		if d.autoCheck != nil && !d.autoCheck(def.ID) {
			continue
		}
		d.mu.Lock()
		last, ok := d.lastTriggered[def.ID]
		d.mu.Unlock()
		if ok && time.Since(last) < def.Cooldown {
			continue // 冷却中
		}
		// 触发该任务
		issuePath, err := d.triggerTask(def)
		if err != nil {
			log.Printf("🕐 internal task %s trigger failed: %v", def.ID, err)
			continue
		}
		lastIssuePath = issuePath
		// v2.5.5 测试6修复: 触发后调总调度器 Submit（不只建单——直接派发——闭环）
		if d.onTrigger != nil {
			d.onTrigger(def, lastIssuePath)
		}
		d.mu.Lock()
		d.lastTriggered[def.ID] = time.Now()
		d.mu.Unlock()
		// v2.5.5 断点续跑: 触发状态落盘（重启后从暂停处继续）
		d.saveIdleState()
		log.Printf("🕐 internal task triggered: %s (%s)", def.ID, def.Description)
		return // 一次只触发一个（避免并发内部任务）
	}
	log.Printf("🕐 all internal tasks cooling down — none triggered")
}

// triggerTask 建 issue + 派 CA（复用调度器——写 docs/issues/ 任务单）
// v2.5.5 P1-3: 返回任务单路径（onTrigger 传给总调度器——完成时更新状态）
func (d *IdleDetector) triggerTask(def InternalTask) (string, error) {
	// 建任务单（docs/issues/internal-<id>-<时间>.md）
	ts := time.Now().Format("20060102-150405")
	issueName := "internal-" + def.ID + "-" + ts + ".md"
	if err := os.MkdirAll(d.issueDir, 0o755); err != nil {
		return "", err
	}
	issuePath := filepath.Join(d.issueDir, issueName)
	content := buildInternalIssue(def, ts)
	if err := os.WriteFile(issuePath, []byte(content), 0o644); err != nil {
		return "", err
	}
	log.Printf("🕐 internal task issue filed: %s", issuePath)
	return issuePath, nil
}

// buildInternalIssue 构造任务单内容（复用 issue 格式——状态机兼容）
func buildInternalIssue(def InternalTask, ts string) string {
	// 任务单格式: 状态机（open）——与现有 issue 兼容（调度器可扫）
	return strings.Join([]string{
		"# 内部任务: " + def.Description,
		"",
		"- 状态: open",
		"- 优先级: P2（内部任务——不影响外部）",
		"- 类型: internal（v2.5.5 内部任务）",
		"- 创建: " + ts,
		"- 任务ID: internal-" + def.ID,
		"",
		"## 任务描述",
		"",
		def.Template,
		"",
		"## 完成标准",
		"",
		"- 任务报告必须写文件（internal-task-report.md——工作区根目录）——没文件=任务失败",
		"- 报告内容: 做了什么/结果数据/发现的问题（真实记录——不空话）",
		"- 脑检查确认（git diff 有实际改动——报告文件存在）",
		"",
		"## 铁律（P1-1 防假完成）",
		"",
		"- 只说\"完成\"不算完成——必须 write 报告文件",
		"- 系统检测到无报告文件会说\"完成\"——会引导你写——写才算完成",
		"- 写完报告再输出【任务完成】",
		"",
	}, "\n")
}
