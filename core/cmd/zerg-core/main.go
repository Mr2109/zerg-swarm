// zerg-core 是虫族模型群主控 v2 的 Go 实现入口。
// 阶段 1：骨架 —— fleet 路由表 + 心跳接收 + status/models API。
//
// 用法:
//
//	cd core && go run ./cmd/zerg-core
//
// 监听端口 8580（API）；网关 8082。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
	"github.com/Mr2109/zerg-swarm/core/internal/version"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core"
	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/api"
	"github.com/Mr2109/zerg-swarm/core/internal/chat"
	"github.com/Mr2109/zerg-swarm/core/internal/compressor"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/gateway"
	"github.com/Mr2109/zerg-swarm/core/internal/plugin"
	"github.com/Mr2109/zerg-swarm/core/internal/plugin/adapters"
	"github.com/Mr2109/zerg-swarm/core/internal/selfupdate"
	"github.com/Mr2109/zerg-swarm/core/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// fleet.yaml 路径由配置指定（defaultFleetYAML 已删——2026-08-13 死代码清理）

func main() {
	// 启动期拨号诊断（2026-09-14 临时排查）：ZERG_DIAG_STARTUP_DIAL="host:port,host:port"
	// 挂在进程出生处，用于区分"二进制静态属性/策略"与"运行时状态"两类原因。
	if t := os.Getenv("ZERG_DIAG_STARTUP_DIAL"); t != "" {
		gateway.StartupDialDiagnostics(t)
	}

	// 身份/帮助（升级模块：每件都必须能自报"跑的是哪份代码"；此前 --version 会被当配置文件路径静默吞掉）
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Println(version.Line("zerg-core"))
			return
		case "update":
			// 源码式自更新（B3）：`zerg update` —— 与 hermes update 同形。
			// 它自己不是换装者：构建后 spawn 独立进程（scripts/build/zerg-upgrade.sh 六阶段内核）。
			os.Exit(selfupdate.CLIMain(os.Args[2:]))
		case "--help", "-h", "help":
			fmt.Println("usage: zerg-core [config-file]")
			fmt.Println("      zerg-core --version     # print code identity (machine-readable)")
			fmt.Println("      zerg update [--check]   # source-based self-update (fetch→build→six-phase swap)")
			fmt.Println("config defaults to ./fleet.yaml (or use -c); environment variables in docs/CONFIGURATION")
			return
		}
	}
	// 结构化日志（slog + lumberjack 轮转）：分级/JSON/大小轮转防无限增长
	// 写入 /tmp/zerg-core.log（供 /api/core/logs 读取，UI 日志面板用）
	setupLogger("/tmp")

	// v2.3 B1: 心跳专用日志（分离到 /tmp/zerg-heartbeat.log，避免撑满主日志）
	heartbeatLogger := setupHeartbeatLogger()

	// 解析 fleet.yaml 配置
	fleetYAML := resolveFleetYAML()
	fmt.Printf("🦠 Zerg Core %s (internal task engine)\n", version.Tag)
	fmt.Printf("📋 Config file: %s\n", fleetYAML)
	slog.Info("core started", "config", fleetYAML)

	// P4-49 统一工具库: 对话 deferred 工具注册到 agent 层（CA/对话通用——Mr2109 2026-09-02）
	chat.RegisterChatExtraTools()
	slog.Info("unified tool library registered", "count", len(chat.ChatExtraToolDefs()))

	// P4-49 工具调用统一计数（对话+CA 同一计数器——甲批 T2: ~/.zerg/state/tool_uses.json）
	agent.InitToolUses()
	// P4-49 工具版本表（tools/versions.json——进化可追溯）
	agent.InitToolVersions()
	// P4-50 web_search v1.0.1: 搜索缓存加载（TTL 5min——同 query 秒回）
	agent.LoadSearchCache()
	// P4-50 工具错误桶加载（聚合错误记录——驱动工具进化——/tmp/zerg-tool-errors.json）
	chat.LoadToolErrors()

	cfg, err := config.LoadFleetConfig(fleetYAML)
	if err != nil {
		log.Fatalf("❌ failed to load the config file: %v", err)
	}
	// 2026-09-11 A 批（库内零明文）：令牌缺失必须启动即失败——不得静默放行
	// （空令牌会让所有 /api/* 请求 401，而日志看起来一切正常，属最难排查的一类）
	if cfg.Auth.Token == "" {
		log.Fatalf("❌ no shared token configured: set ZERG_AUTH_TOKEN, or write it to %s (outside the repo — recommended), or the repo-root .env (see .env.example)", config.TokenFilePath())
	}

	fmt.Printf("🔐 Auth token: %s\n", config.MaskToken(cfg.Auth.Token))
	fmt.Printf("🖥️  Fleet nodes: %d\n", len(cfg.Fleet))
	fmt.Printf("🤖 Known models: %d\n", len(cfg.Models))
	slog.Info("config loaded", "models", len(cfg.Models), "fleet", len(cfg.Fleet))

	// M0 配置校验（v2.4）：启动时跑 Validate——Fatal 模型报错 + Warn 提示
	// 铁律：无模块模型拒绝调用——启动时暴露配置问题（接入时发现而非运行才失败）
	validateModels(cfg)

	// 3c（2026-09-16）：本机后端（localback）初始化与"自收养本机已跑模型"已删 —— 本机角色退役后
	// 本机 = 名为 Mr2109 的普通子端，由 launchd 托管、经心跳上报；主控不再自己起/接管引擎。
	// 初始化存储和处理器
	fleetStore := store.NewStore()
	// 3a（2026-09-16 Mr2109 拍：所有可推理的计算机都是子端）：**本机角色退役** ⇒
	// 不再维护 store 的 local 行（原先靠 startLocalSnapshotRefresh 首写+周期刷）。
	// 本机 = 名字叫 Mr2109 的普通子端，它的资源与身份**经心跳上报**，与 x3 同形。
	handlers := &api.Handlers{
		Config:          cfg,
		ConfigPath:      fleetYAML, // B11: 热加载用
		Store:           fleetStore,
		HeartbeatLogger: heartbeatLogger, // v2.3 B1: 传入心跳专用 logger
	}

	// 配置 chi 路由器
	r := chi.NewRouter()

	// 基础中间件
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// 认证中间件：所有 /api/* 端点需要 X-Auth-Token
	r.Use(api.AuthMiddleware(cfg.Auth.Token))

	// v2.5.7 对话模块（C1 存储 + C2 推理——借鉴 Hermes——Mr2109——顶部导航第一板块）
	chatStore, chatErr := chat.OpenChatStore()
	// 乙批（2026-09-10）：把对话库注入记忆工具接线（session_search 用）
	if chatErr == nil && chatStore != nil {
		chat.SetWireStore(chatStore)
	}
	if chatErr != nil {
		fmt.Printf("⚠️  chat module init failed: %v (continuing without chat)\n", chatErr)
	} else {
		chatLifecycle := chat.NewLifecycle(chatStore)
		chatLifecycle.Start() // 90 天硬删定时（启动扫 + 每天 3 点）
		chatInfer := chat.NewChatInfer(statepath.GatewayBaseURL(), cfg.Auth.Token)
		chatHandlers := api.NewChatHandlers(chatStore, chatInfer)
		chatHandlers.RegisterChatRoutes(r)
		handlers.ChatStore = chatStore // v2.5.7 对话→任务: 总调度器 handler 可查来源对话内容（派任务带上下文）
		fmt.Printf("💬 Chat module: ready (SQLite + gateway inference — 90-day hard delete)\n")
	}

	// 路由注册
	// POST /api/fleet/heartbeat - 接收子端心跳
	r.Post("/api/fleet/heartbeat", handlers.HeartbeatHandler)

	// GET /api/fleet/status - 集群状态
	r.Get("/api/fleet/status", handlers.StatusHandler)

	// GET /api/fleet/models - 模型列表
	r.Get("/api/fleet/models", handlers.ModelsHandler)
	// v2.5.6 模型详情+启停（Mr2109 2026-08-27——UI 模型库右栏: 适配器选项 + 手动启动开关）
	r.Get("/api/models/{name}", handlers.ModelDetailHandler)
	r.Post("/api/models/{name}/start", handlers.ModelStartHandler)
	r.Post("/api/models/{name}/stop", handlers.ModelStopHandler)
	// v2.5.6 适配器参数编辑（Mr2109 2026-08-27——实时生效——各模型各自参数集）
	r.Get("/api/models/{name}/adapter-opts", handlers.AdapterSchemaHandler)
	r.Put("/api/models/{name}/adapter-opts", handlers.UpdateAdapterOptionsHandler)

	// GET /api/fleet/tasks - 任务列表（初始为空）
	r.Get("/api/fleet/tasks", handlers.TasksHandler)

	// POST /api/fleet/logs - 子端日志上报（v1 agent 兼容；v2 agent 可后续扩展）
	r.Post("/api/fleet/logs", handlers.LogsHandler)

	// POST /api/config/reload - 热加载配置（B11：加模型不用重启）
	r.Post("/api/config/reload", handlers.ReloadConfigHandler)

	// 正式端口 8580（API）+ 8082（网关）——已从测试端口 8680/8682 切换
	port := statepath.CorePort()
	// 用 0.0.0.0 显式监听 IPv4（Go 的 ":8580" 默认 IPv6-only，子端 IPv4 连不上）
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	fmt.Printf("🚀 Server listening on %s\n", addr)

	// 控制端点（加载/卸载/退出主控）
	ctrl := api.NewControlHandlers(cfg.Auth.Token, cfg) // 3a：本机角色退役
	r.Post("/api/control/load", ctrl.LoadHandler)
	r.Post("/api/control/unload", ctrl.UnloadHandler)
	r.Post("/api/control/stop", ctrl.StopHandler)
	r.Get("/api/core/status", ctrl.CoreStatusHandler)
	r.Get("/api/core/logs", ctrl.CoreLogsHandler)

	// v2.5.4.10 模型适配器注册表（方案 B——适配器完整路由）
	// 加载插件管理器 + 注册 6 个模型适配器（example-35b-v2/ds4/nemotron/qwen38/qwable/gemma）
	// 无适配器的模型 → 回退旧路由（fleet.yaml + 打分）——兼容
	adapterRegistry := map[string]plugin.Plugin{
		"example-35b":             adapters.NewOrnithAdapter(),
		"example-35b-v2":             adapters.NewOrnithAdapter(), // 2026-08-20 接入——新版——不替换1.0
		"deepseek-v4-flash":          adapters.NewDs4Adapter(),
		"Nemotron-3.5-Lightning":     adapters.NewNemotronAdapter(),
		"Qwen3.8-27B":                adapters.NewQwen38Adapter(),
		"Qwen3.8-Flash-Next":         adapters.NewQwen38FlashAdapter(), // 2026-08-29 接入——125B MoE qwen4架构
		"Qwen3.8-Flash-Next-IQ4_XS":  adapters.NewQwen38FlashAdapter(), // 2026-08-30 X3 三档量化——IQ4_XS 93.7GB（内存更宽）
		"Qwen3.8-Flash-Next-Q3_K_XL": adapters.NewQwen38FlashAdapter(), // 2026-08-30 X3 三档量化——Q3_K_XL 89.9GB（内存最宽）
		"example-8b-quant":           adapters.NewQwableAdapter(),
		"Qwable-v1.Q5_K_M":           adapters.NewQwableAdapter(),
		"gemma-4-26B":                adapters.NewGemmaAdapter(),
		"gemma-4-12B":                adapters.NewGemmaAdapter(),
		"GLM-4.7-Flash":              adapters.NewGlm47Adapter(),       // 2026-08-28 补全——智谱30B-A3B MoE
		"Qwen3.6-35B-A3B":            adapters.NewQwen36Adapter(),      // 2026-08-28 补全——阿里35B/3B MoE
		"example-30b":           adapters.NewMuseGlimmerAdapter(), // 2026-08-28 补全——Meta 30B 多模态
	}
	// 初始化适配器配置（Init——默认值——后续 config.yaml 覆盖）
	for name, adp := range adapterRegistry {
		if err := adp.Init(nil); err != nil {
			log.Printf("⚠️ adapter %s init failed: %v", name, err)
		}
	}
	log.Printf("🧩 Model adapter registry: %d adapters (full adapter routing)", len(adapterRegistry))

	// 同时启动网关（:8082），三标准透传 + 本机子端
	gw := gateway.NewGateway(cfg.Auth.Token, cfg, fleetStore, adapterRegistry) // 3a/3c：本机角色退役（网关已不带本机后端）
	// v2.5.6 2026-08-28 治本: 网关先启动并等待就绪——再恢复任务/派发（之前 goroutine 晚启动——任务调 8082 connection refused 全失败→熔断连锁）
	go func() {
		if err := gw.Start(8082); err != nil {
			log.Fatalf("❌ gateway start failed: %v", err)
		}
	}()
	for i := 0; i < 30; i++ {
		if conn, err := net.DialTimeout("tcp", "127.0.0.1:8082", 500*time.Millisecond); err == nil {
			conn.Close()
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Printf("🚦 Gateway readiness check done (8082)\n")

	// v2.5.5 #9 补充5: 网关引用注入 handlers（心跳健康清零熔断用）
	handlers.Gateway = gw

	// 网关卡表（circuit breaker）手动复位 + 原因可见（Mr2109痛点: 熔断后看不到原因/无手动入口）
	// 复用全局 AuthMiddleware（挂在本函数顶部的 r.Use）——两接口均走 /api/* 鉴权，不是免鉴权旁路
	r.Get("/api/gateway/breakers", handlers.BreakersHandler)             // 只读快照
	r.Post("/api/gateway/breakers/reset", handlers.BreakersResetHandler) // 手动复位（host 缺省=全部）

	// 模型目录（modelreg）只读快照——UI"模型库"页数据源（Mr2109）
	// 只读：不建目录不写文件；空目录/根不存在 = 200 + count=0；坏记录只计入该条 errors
	r.Get("/api/models/registry", handlers.ModelRegistryHandler)

	// 资源管理器观测面（《设计-资源管理器》批 4；§八 Q8：需鉴权 + 只读）
	//   GET  /api/resources/ledger  —— 每机账本 + 逐模型"跑得动吗"（严格只读：不建目录、不写文件）
	//   POST /api/resources/pin|unpin —— 显式动作：只对"已在驻留清单内且托管"的项生效；
	//        非驻留/未托管一律明确拒绝（§八 Q5/Q6：绝不因 pin 去启动/接管任何进程）
	// 引擎参数（KV dtype / 开销）是运行参数、GGUF 不记录——由环境变量提供；未给则估算标 estimated=true。
	// chi 里静态段优先于参数段，故本三路不会被既有的 GET /api/resources/{type} 抢走。
	handlers.KvCacheBytesPerElem = parseEnvFloat("ZERG_KV_CACHE_BYTES_PER_ELEM")
	handlers.EngineOverheadGb = parseEnvFloat("ZERG_ENGINE_OVERHEAD_GB")
	handlers.ResourcePins = api.NewSubEndPinController(cfg, cfg.Auth.Token)
	r.Get("/api/resources/ledger", handlers.ResourceLedgerHandler)
	r.Post("/api/resources/pin", handlers.ResourcePinHandler)
	r.Post("/api/resources/unpin", handlers.ResourceUnpinHandler)

	// v2.5.5 T3 主控总调度器（两级调度——Mr2109原理）
	// 主控总调度: 管所有内部任务 + 接入的外部任务（全局决策/派发）
	// CA 子调度: 管分派任务的执行（agent 侧已有 scheduler）
	// CA 二进制解析（待修补 #44：不再写死本机私有路径——公开快照会脱敏替换它，机群上就指向不存在的目录）：
	//   ① ZERG_AGENT_BIN 显式指定（与改造前逻辑一致，保持最高优先）；
	//   ② 否则取「主控自身可执行文件的同级」——本机从 <仓库>/bin/zerg-core 启动 ⇒ <仓库>/bin/zerg-agent（与旧字面量逐字相同）；
	//   ③ 只有 os.Executable() 失败时才回退到 <工作区>/bin/zerg-agent。
	agentBin := ""
	if env := os.Getenv("ZERG_AGENT_BIN"); env != "" {
		agentBin = env
	} else if exe, err := os.Executable(); err == nil {
		agentBin = filepath.Join(filepath.Dir(exe), "zerg-agent")
	} else {
		agentBin = filepath.Join(statepath.WorkspaceRoot(), "bin", "zerg-agent")
	}
	masterSched := api.NewMasterScheduler(agentBin, 1, &api.StoreSnapshotReader{Store: fleetStore}) // 单槽——串行——v2.5.6 注入 store（ping 快照优先）
	handlers.Scheduler = masterSched
	// 2026-09-13: 显式启动（派发恢复的排队任务 + 启动故障自愈扫描）——构造期不再自动执行任务
	masterSched.Start()
	fmt.Printf("🔄 Master scheduler started (two-level scheduling)\n")

	// v2.5.5 P1-5 治本: 启动清理残留任务 worktree（上次崩溃/重启悬空——状态丢——worktree 堆积）
	// 扫描 git worktree list——所有 task-* 分支——尝试 merge（有报告）或强制清理（无报告——任务已死）
	cleaned := api.CleanupStaleWorktrees(statepath.WorkspaceRoot())
	if cleaned > 0 {
		fmt.Printf("🧹 Cleaned %d stale worktrees at startup\n", cleaned)
	}

	// v2.5.5 T3 任务 API（总调度器入口——外部任务接入/内部任务触发）
	r.Post("/api/tasks", handlers.SubmitTaskHandler)
	r.Get("/api/tasks", handlers.ListTasksHandler)
	// v2.5.5 虫族UI: 任务详情（点击任务看全部数据——含跟踪）
	r.Get("/api/tasks/{id}", handlers.TaskDetailHandler)
	// v2.5.5 任务操作（右键功能——2026-08-21 Mr2109）: 重跑/置顶置底/上移下移/删除
	r.Post("/api/tasks/{id}/retry", handlers.TaskRetryHandler)
	r.Post("/api/tasks/{id}/move", handlers.TaskMoveHandler)
	r.Post("/api/tasks/{id}/pause", handlers.TaskPauseHandler)
	r.Post("/api/tasks/{id}/terminate", handlers.TaskTerminateHandler)
	r.Post("/api/tasks/{id}/requeue", handlers.TaskRequeueHandler)
	r.Delete("/api/tasks/{id}", handlers.TaskDeleteHandler)
	// v2.5.5 资源信任度（2026-08-21 Mr2109）: 加载资源状态表
	api.LoadResourceTrust()
	// 存量工具注册为正式（一直在用的——不是新入库——Mr2109）
	api.RegisterExistingTools()
	// 存量模型注册为正式（fleet 配置里的模型——一直在用——不是新接入）
	api.RegisterExistingModels(cfg)

	// v2.5.5 任务目录归档器（2026-08-20 设计——Mr2109）: 定时扫描——30 天归档 + 90 天删除
	go func() {
		archiveTicker := time.NewTicker(6 * time.Hour)
		defer archiveTicker.Stop()
		api.RunTaskArchive() // 启动先跑一次
		for range archiveTicker.C {
			api.RunTaskArchive()
		}
	}()

	// v2.5.5 虫族UI: git 端点（分支/worktree/diff——git 显示）
	r.Get("/api/git/status", handlers.GitStatusHandler)
	r.Get("/api/tasks/{id}/git", handlers.TaskGitHandler)
	r.Get("/api/tasks/{id}/diff", handlers.TaskDiffHandler)
	// v2.5.5 虫族UI: 资源库/日志/文档
	r.Get("/api/resources/{type}", handlers.ResourcesHandler)
	r.Get("/api/logs/{kind}", handlers.LogsHandler2)
	// 2026-09-13（C9 第 4 步）：宿主只留**读/浏览**——文档界面 + 它的写后端（原
	// docs_ops.go 的 mkdir/rename/delete/copy/save 五端点）**整块迁进文档茧**
	// （独立仓 zerg-cocoon/文档，自带 Go 服务，端口默认 8610）⇒ 那五条 POST 路由已删。
	// 剩下的 /api/docs 是**通用文件读取**（白名单根 + 类型闸门）：
	//   GET /api/docs?root=&path=  参数化读（文件浏览器用）
	//   GET /api/docs/<rel>        缺省 docs 根的老路径读（同上，文档茧迁移后仍被宿主文件浏览器用）
	r.Get("/api/docs", handlers.DocsHandler)
	r.Get("/api/docs/*", handlers.DocsHandler) // v2.5.6 catch-all 多段路径（00-总览/xxx.md）
	// 2026-09-13 文件/目录浏览器 阶段 1（《设计-文件浏览器虫茧-20260913》§4.2）
	//   GET  /api/fileroots         —— 五根白名单 + 可配置项（只读：不探目录、不建目录）
	//   POST /api/fileroots/open    —— 用默认应用打开（目录 / text_exts 内类型）
	//   POST /api/fileroots/reveal  —— 在访达中显示
	// 参数化读取走既有 GET /api/docs?root=&path=（不带查询参数时仍是改造前的 docs 行为）
	r.Get("/api/fileroots", handlers.FileRootsHandler)
	r.Post("/api/fileroots/open", handlers.FileOpenHandler)
	r.Post("/api/fileroots/reveal", handlers.FileRevealHandler)
	// v2.5.5 接口可发现性（2026-08-22 Mr2109）: 其他智能体调用虫族——能力/规范/示例
	r.Get("/api/capabilities", handlers.CapabilitiesHandler)
	r.Get("/api/openapi.json", handlers.OpenAPIHandler)
	r.Get("/api/help", handlers.HelpHandler)
	// v2.5.5 归档检索（2026-08-22 Mr2109补充）: 归档列表/检索
	r.Get("/api/archive", handlers.ArchiveHandler)
	// v2.5.5 内部任务分类（2026-08-22 Mr2109补充）: 清单 + 手动执行
	r.Get("/api/internal-tasks", handlers.InternalTasksHandler)
	r.Post("/api/internal-tasks/{id}/run", handlers.InternalTaskRunHandler)
	// v2.5.6 周期调度（Mr2109 2026-08-27——任务循环周期）
	r.Post("/api/internal-tasks/{id}/interval", handlers.InternalIntervalHandler)
	r.Get("/api/internal-tasks/intervals", handlers.InternalIntervalsHandler)
	r.Post("/api/internal-tasks/{id}/mode", handlers.InternalModeHandler)
	r.Get("/api/internal-tasks/modes", handlers.InternalModesHandler)
	// v2.5.6 内部任务启停（Mr2109 2026-08-27——UI 内部任务板块最上面停止/启动按钮）
	// 2026-09-10 治本（APP-A07）：引擎状态查询——UI 轮询/运维 curl 的单一真相源
	r.Get("/api/internal-tasks/state", handlers.InternalEngineHandler)
	r.Post("/api/internal-tasks/stop", api.InternalTasksStopHandler)
	r.Post("/api/internal-tasks/start", api.InternalTasksStartHandler)

	// v2.5.5 T3 内部任务引擎（空闲检测——挂主控）
	// 外部任务队列空 + 资源空闲 → 触发内部任务（进化）
	// 仓库内路径一律经 statepath 解析器（待修补 #44：写死的私有路径在机群上指向不存在的目录）——
	// 本机解析结果与旧字面量逐字相同（<仓库>/docs/issues、<仓库>/core）。
	coreDir := filepath.Join(statepath.WorkspaceRoot(), "core")
	idleDetector := agent.NewIdleDetector(statepath.IssuesDir())
	api.InitInternalModes()                       // v2.5.6: 内部任务运行模式开关持久化恢复（自动/手动——Mr2109 2026-08-28）
	idleDetector.SetAutoCheck(api.IsInternalAuto) // v2.5.6: 运行模式开关——手动任务不自动触发（空闲检测跳过）
	idleDetector.SetExternalQueue(func() int { return masterSched.RunningCount() })
	idleDetector.SetResourceIdle(func() bool {
		// 资源空闲: X3 GPU idle（快照 active 请求 0）
		if snap := gw.Snapshot("x3"); snap != nil && snap.ActiveRequests == 0 {
			return true
		}
		return false
	})
	// v2.5.5 测试6修复: 空闲触发→直接调总调度器 Submit（内部任务闭环——不只建单）
	// v2.5.5 P1-3: 触发回调带 issuePath——总调度器完成时更新任务单状态
	// v2.5.5 模型轮换（Mr2109 2026-08-21 升级）: 每个任务换一个模型——多轮循环——各模型执行同一种任务
	// 对比时间/质量——模型评测数据积累（执行慢根因=Qwen3.8 推理慢——轮换公平对比）
	internalModelPool := []string{
		"Qwen3.8-27B", "example-35b-v2", "gemma-4-26B", "GLM-4.7-Flash", "example-8b-quant", "deepseek-v4-flash", "example-30b", "Nemotron-3.5-Lightning",
	}
	internalModelIdx := 0
	internalTriggerCount := 0 // 触发计数（每任务换——多轮循环）
	idleDetector.SetOnTrigger(func(def agent.InternalTask, issuePath string) {
		// 每任务换模型（Mr2109 2026-08-21——多轮循环——各模型执行同一种任务——对比时间/质量）
		if internalTriggerCount > 0 {
			internalModelIdx = (internalModelIdx + 1) % len(internalModelPool)
			fmt.Printf("🔄 Internal task model rotated: %s (trigger #%d)\n", internalModelPool[internalModelIdx], internalTriggerCount)
		}
		internalTriggerCount++
		model := internalModelPool[internalModelIdx]
		fmt.Printf("🕐 Internal task %s using model %s (trigger #%d)\n", def.ID, model, internalTriggerCount)
		masterSched.Submit(&api.Task{
			ID:          "internal-" + def.ID + "-" + fmt.Sprintf("%d", time.Now().UnixNano()),
			Description: def.Template,
			Priority:    api.PriorityInternal,
			Type:        "internal",
			Model:       model,
			Workdir:     coreDir,
			Status:      "queued",
			IssuePath:   issuePath, // P1-3: 任务单路径——完成时更新状态
			Flow:        "zerg",    // v2.5.6 Mr2109 2026-08-27: 内部任务走新机制（程序定量驱动——内部任务=最佳测试任务）
			SkillKey:    def.ID,    // v2.5.6 Mr2109: skill 归属=任务独属（def.ID——沉淀后成为该任务自己的 skill）
		})
	})
	// ===== 2026-09-06 内部任务引擎停用 =====
	// 事故: 内部任务(tool-check/mem-disk-alert 等清理类)经旧 bash 工具(黑名单无 $HOME/变量展开防护——P6 洞)误删用户主目录与 ~/.hermes
	// 恢复: bash_v101 沙盒+危险命令双检(bashExpandDangerScan)合入并重编 bin/zerg-core 验证后, 启动 zerg-core 前设 ZERG_INTERNAL_TASKS=1
	internalEngineOn := os.Getenv("ZERG_INTERNAL_TASKS") == "1"
	// 2026-09-10 治本（APP-A07）：先恢复用户意图（持久化），再如实登记环境门控
	api.InitInternalEngine()
	if internalEngineOn {
		api.SetEngineEnabled(true, "引擎已启用（ZERG_INTERNAL_TASKS=1）")
	} else {
		api.SetEngineEnabled(false, "引擎未启用：2026-09-06 误删事故后默认关闭（启动前设 ZERG_INTERNAL_TASKS=1）")
	}
	idleStop := make(chan struct{})
	if internalEngineOn {
		go idleDetector.Run(idleStop)
		fmt.Printf("🕐 Internal task engine started (idle detection on the core)\n")
	} else {
		fmt.Printf("🛑 Internal task engine disabled: ZERG_INTERNAL_TASKS not set to 1 (all automatic triggers off)\n")
	}
	// v2.5.6 周期调度（Mr2109 2026-08-27——任务循环周期——每 60s 检查到点触发）
	// 触发复用 onTrigger 的提交逻辑（模型轮换 + Flow=zerg + SkillKey=def.ID）
	if internalEngineOn {
		go func() {
			for {
				time.Sleep(60 * time.Second)
				// 2026-09-10 治本（APP-A07）：停止对**周期调度**也生效——原先该循环不看停止标志，按钮形同虚设
				if api.InternalTasksStopped() {
					api.EngineSkippedTick()
					continue
				}
				due := api.RunIntervalTick()
				api.EngineTick(60 * time.Second) // 心跳：running=true 但心跳停滞 → UI 显示异常
				for _, defID := range due {
					// v2.5.6 运行模式开关（Mr2109 2026-08-28）: 手动运行任务——周期调度跳过（只手动触发）
					if !api.IsInternalAuto(defID) {
						fmt.Printf("⏰ Interval trigger skipped: %s (manual-only)\n", defID)
						continue
					}
					for _, d := range agent.ListInternalTasks() {
						if d.ID != defID {
							continue
						}
						if internalTriggerCount > 0 {
							internalModelIdx = (internalModelIdx + 1) % len(internalModelPool)
						}
						internalTriggerCount++
						model := internalModelPool[internalModelIdx]
						fmt.Printf("⏰ Interval trigger: internal task %s using model %s\n", defID, model)
						masterSched.Submit(&api.Task{
							ID:          "internal-" + d.ID + "-" + fmt.Sprintf("%d", time.Now().UnixNano()),
							Description: d.Template,
							Priority:    api.PriorityInternal,
							Type:        "internal",
							Model:       model,
							Workdir:     coreDir,
							Status:      "queued",
							Flow:        "zerg",
							SkillKey:    d.ID,
						})
						break
					}
				}
			}
		}()
		fmt.Printf("⏰ Internal task interval scheduler started (checks every 60s)\n")
	}

	// v2.5.6 内部任务启停控制（UI 按钮——停止=发信号停检测——启动=重启检测）
	internalStopCh := idleStop
	if !internalEngineOn {
		api.SetInternalTasksControl(func() {}, func() {}) // 停用态: 启停按钮置空——防 UI /start 绕过门控
	} else {
		api.SetInternalTasksControl(
			func() { // onStop: 停 idle 检测（发停止信号）
				select {
				case <-internalStopCh:
					// 已停止
				default:
					close(internalStopCh)
				}
				fmt.Printf("🛑 Internal task stopped (UI button)\n")
			},
			func() { // onStart: 重启 idle 检测（新通道 + Run）
				internalStopCh = make(chan struct{})
				go idleDetector.Run(internalStopCh)
				fmt.Printf("▶️ Internal task started (UI button)\n")
			},
		)
	}

	// 主控重启恢复开关状态（排除本机——UI 同步不丢）
	gw.LoadExcludeLocal()

	// 丙批 C2 补齐（2026-09-10）：退出前把前缀命中率统计落盘（否则末批样本随进程一起丢）
	{
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		go func() {
			<-sigCh
			gw.FlushPrefixCache()
			fmt.Printf("💾 Prefix-cache hit stats flushed (exit)\n")
			os.Exit(0)
		}()
	}

	// V22-压缩：LLMLingua-2 一体化压缩器（ONNX，纯 Go 进程内）
	// 路径：<压缩模型目录>/llmlingua2-onnx/（模型已转换 ONNX）——目录经 statepath.CompressModelsDir 解析，尊重 ZERG_COMPRESS_MODELS
	compressorPath := filepath.Join(statepath.CompressModelsDir(), "llmlingua2-onnx")
	linguaCompressor := compressor.New(compressor.Config{
		ModelPath: filepath.Join(compressorPath, "model.onnx"),
		TokPath:   filepath.Join(compressorPath, "tokenizer.json"),
	})
	gw.SetCompressor(linguaCompressor)
	// 丙批 §4.2（2026-09-10）：压缩单一入口的 LLMLingua-2 依赖——与网关同一个进程内 ONNX 实例
	chat.SetLinguaCompressor(func(_ context.Context, text string) (string, error) {
		out, _, _, err := linguaCompressor.Compress(text)
		return out, err
	})

	// B4 v2：排除本机模式切换（用户工作时——路由跳过 local）
	r.Post("/api/fleet/exclude-local", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Exclude bool `json:"exclude"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "参数错误", http.StatusBadRequest)
			return
		}
		gw.SetExcludeLocal(req.Exclude)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"exclude_local":%v}`, req.Exclude)
	})
	r.Get("/api/fleet/exclude-local", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"exclude_local":%v}`, gw.ExcludeLocal())
	})

	// B13/#31: 本机心跳周期任务已上移到 fleetStore 创建处（startLocalSnapshotRefresh）——
	// 那里在网关/调度器之前完成同步首写，启动窗口内 store 即有 local 行；此处不再重复注册。

	// B4 监看面板（状态灯 + 模型矩阵 + 告警——单页仪表盘 8581）
	go func() {
		mon := core.NewMonitor(statepath.CoreBaseURL(), cfg.Auth.Token)
		if err := mon.Start("8581"); err != nil {
			fmt.Printf("⚠️ monitor panel failed to start: %v\n", err)
		}
	}()

	log.Fatal(http.ListenAndServe(addr, r))
}

// validateModels 启动时校验所有模型配置（M0 v2.4——V001-V015 规则）
// Fatal 模型：打印错误（不阻塞启动——网关仍会拒绝调用无模块模型）
// Warn 模型：打印提示（存量兼容——默认值/推断）
func validateModels(cfg *config.FleetConfig) {
	fatalCount, warnCount := 0, 0
	for name, candidates := range cfg.Models {
		if len(candidates) == 0 {
			continue
		}
		res := config.Validate(name, candidates[0])
		if res.HasFatal() {
			fatalCount++
			slog.Warn("model config validation failed", "model", name, "errors", len(res.Errors))
		} else if len(res.Warnings) > 0 {
			warnCount++
		}
	}
	if fatalCount > 0 {
		fmt.Printf("⚠️  M0 validation: %d model configs have fatal errors (the gateway will refuse them)\n", fatalCount)
	}
	if warnCount > 0 {
		fmt.Printf("📝 M0 validation: %d models have warnings (legacy compat — defaults/inference)\n", warnCount)
	}
	if fatalCount == 0 && warnCount == 0 {
		fmt.Printf("✅ M0 validation: all model configs passed\n")
	}
}

// resolveFleetYAML 解析 fleet.yaml 路径：优先用命令行参数，其次用默认路径。
func resolveFleetYAML() string {
	// 检查命令行参数
	if len(os.Args) > 1 {
		return os.Args[1]
	}

	// 默认路径：相对于工作目录的 gateway/fleet.yaml
	defaultPath := filepath.Join("gateway", "fleet.yaml")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}

	// 默认路径不存在：回退到「工作区根」下的 gateway/fleet.yaml（经解析器取根——不再写死本机私有路径）
	return filepath.Join(statepath.WorkspaceRoot(), "gateway", "fleet.yaml")
}

// setupLogFile 已废弃（2026-08-11 改由 setupLogger 的 lumberjack 轮转接管，
// 避免 Dup2 与轮转文件冲突——轮转后旧 fd 写不到新文件）。

// 监看面板（B4）——goroutine 启动，8581 端口

// loadToCpuPct 从 load 换算 CPU 使用率近似（load/核数——macOS load 1 分钟平均）
func loadToCpuPct(load float64) float64 {
	cores := runtime.NumCPU()
	if cores == 0 {
		cores = 1
	}
	pct := load / float64(cores) * 100
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	return pct
}

// collectGpuPct 本机 GPU 使用率（macOS Apple Silicon：ioreg gpu-perf-tgt-utilization——无需root）
func collectGpuPct() float64 {
	out, err := exec.Command("ioreg", "-l").Output()
	if err != nil {
		return -1
	}
	// 找 gpu-perf-tgt-utilization = <46 00 00 00>（hex little-endian uint32——0-100 百分比）
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "gpu-perf-tgt-utilization") {
			// 提取 <46000000>
			start := strings.Index(line, "<")
			end := strings.Index(line, ">")
			if start >= 0 && end > start {
				hexStr := strings.ReplaceAll(line[start+1:end], " ", "")
				if len(hexStr) >= 2 {
					val, err := strconv.ParseInt(hexStr[:2], 16, 32)
					if err == nil {
						return float64(val)
					}
				}
			}
		}
	}
	return -1
}

// parseEnvFloat 读一个十进制浮点环境变量（资源管理器观测面的引擎参数用）。
// 未设置 / 空白 / 非法 / <=0 一律返回 0——0 表示"未提供"，估算侧会回退常量并标 estimated=true，
// 绝不因为配错就假装拿到了真值（放宽=冒充实测）。
func parseEnvFloat(name string) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}
