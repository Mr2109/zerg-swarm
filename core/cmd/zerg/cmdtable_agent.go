package main

// 命令表 · agent 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_agent() []*command {
	return []*command{
		{
			path:     []string{"agent", "ls"},
			kind:     "Machine",
			summary:  "子端（机器）清单（投影 /api/fleet/status）",
			usage:    "zerg agent ls [--json <字段>]",
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "origin_sha", "same_origin", "cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "model", "backend_state", "active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen"},
			endpoint: "GET /api/fleet/status",
			run:      cmdAgentLs,
		},
		// ---- 批 B · T-19 茧壁：`agent ping` 默认经主控、`--direct` 才直连（§十五.4 丙案）----
		{
			path:    []string{"agent", "ping"},
			kind:    "AgentPing",
			summary: "探活一台机（默认经主控；`--direct` 直连且回显 via）",
			usage:   "zerg agent ping <机器名> [--direct <host:port>] [--json <字段>]",
			arity:   "any",
			args:    []string{"机器名"},
			fields:  []string{"machine", "healthy", "code_version", "code_sha", "last_seen", "via"},
			// ★ 缺口 GAP-20260925-20（批⑤ · 2026-10-13）：`answered_by`（**谁答的探活**）走**可选字段层**。
			optFields: []string{"answered_by"},
			endpoint:  "GET /api/fleet/status（默认档）",
			run:       cmdAgentPing,
		},
		// ---- 批 D · T-43 `agent` 族收口（§三 B 族 · §十二 `P-038` 族名闭合 · 只读四条）----
		{
			path:     []string{"agent", "show"},
			kind:     "Machine",
			summary:  "看一台子端（投影 /api/fleet/status 的单机条目 —— 与 `agent ls` 同一份载荷）",
			usage:    "zerg agent show <机器名> [--window-min <分钟>] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"machine", "healthy", "code_version", "code_sha", "origin_sha", "same_origin", "cpu_pct", "gpu_pct", "mem_available_gb", "mem_total_gb", "models", "model", "backend_state", "active_requests", "gpu_used_gb", "backend_rss_gb", "gpu_temp_c", "last_seen"},
			endpoint: "GET /api/fleet/status",
			run:      cmdAgentShow,
			// ★ 缺口 `GAP-20261010-16`（want③）：`recent_requests`（最近 N 分钟请求数）与
			//   `request_source`（当前请求来源）走**可选字段层**（`optFields`，与 `code find` 的
			//   `count` 同一体例）—— 登在出口白名单里、`--json` 点得出来，却**不进** `fields` 那份
			//   文档/帮助导出面的冻结表 ⇒ 随动面为零（✗ 不重生成命令页）。
			//   ★ 两格都**只增不改**：载荷那 18 格的字段名/顺序一个字未动（旧消费侧零改动）。
			optFields: []string{"recent_requests", "request_source"},
		},
		{
			path:     []string{"agent", "models"},
			kind:     "Model",
			summary:  "该机上可用的模型（投影 /api/fleet/models 里 host 命中的那些）",
			usage:    "zerg agent models <机器名> [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"id", "host", "backend", "modality", "mem_gb", "file", "declared", "subend"},
			endpoint: "GET /api/fleet/models",
			run:      cmdAgentModels,
		},
		{
			path:     []string{"agent", "logs"},
			kind:     "AgentLogs",
			summary:  "该机的日志（走主控查询面 GET /api/fleet/logs/query?machine=）",
			usage:    "zerg agent logs <机器名> [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   []string{"ts", "machine", "level", "component", "model", "request_id", "message"},
			endpoint: "GET /api/fleet/logs/query?machine=<机>",
			run:      cmdAgentLogs,
		},
		// ---- 缺口 `GAP-20261010-21`（B 单 · CLI 命令面）：`agent forget`（D2 写面）----
		// 打 A 单已落的主控端点 `POST /api/fleet/forget`（复用内核 `store.Forget`）；口径照端点：
		// 干跑 `dry_run:true` / 真删 `confirm:true`。命令面三态照同族写面先例 `model pull`（D2）：
		// `--dry-run` = **本地**计划件（一个请求都不发）· 缺 `--yes` fail-closed 2 · `--yes` 才真打端点。
		// ★ 新命令 ⇒ 同批随动：cli-matrix must-fail 格 ＋ cli-contract-baseline ＋ 门73 两半。
		{
			path:     []string{"agent", "forget"},
			kind:     "AgentForget",
			summary:  "把一台机从**主控运行态名册**里摘掉（`POST /api/fleet/forget` · D2 写面 · `--dry-run` 零副作用出计划件 · 缺 `--yes` fail-closed 2 · 幂等：不在册 ⇒ changed=false 仍退 0）",
			usage:    "zerg agent forget <机器名> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名（要摘掉的那台机 · 例 ms02）"},
			fields:   []string{"dry_run", "machine", "present", "changed", "would_forget"},
			danger:   &dangerSpec{dangerD2, "机器名", "把该机从**主控运行态名册**里摘掉（`store.Forget` —— 该机再来一次心跳即回名册；✗ 不碰子端 / ✗ 不重启主控）", "缺口 GAP-20261010-21 · A 单端点 POST /api/fleet/forget · 同族写面先例 model pull（D2）", false, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/fleet/forget",
			run:      cmdAgentForget,
		},
		// ---- 缺口 `GAP-20261018-20`：`agent deploy`（子端换件的**命令面正门** · D3）----
		// 病（逐字）：换件此前只能手搓 —— `scp` 直覆盖在跑的件 ⇒ `ETXTBSY`；
		// `systemctl stop` ⇒ 要交互认证（AI 不敲提权命令）；`scp` 到 `.new` ＋ `mv -f` ＋ `restart`
		// 才成，但换完 `agent ls` 的 `code_sha` 仍是旧值 ⇒ 运维面判不了远端跑哪版。
		// 语义 = 落地 `.new` → 远端 sha256 对拍 → `mv -f` 换名 → **只 restart**（✗ stop · ✗ 提权）
		// → 直读子端 `/status` 核对新件自报的 `code_sha` / `code_digest`（对不上 ⇒ 退 1）。
		// ★ 新命令的随动面（cli-matrix must-fail 格 / cli-contract-baseline / help 面冻结读数 /
		//   newcmd-paths-baseline / 换件这条 ssh 面的 §十五.4 例外格）见本单报告的「待随动」一节。
		{
			path:    []string{"agent", "deploy"},
			machine: machineNodeName,
			kind:    "AgentDeploy",
			summary: "子端换件正门（D3 · 先落 `.new` → 远端 sha256 对拍 → `mv -f` 换 inode → 只重启 unit；✗ scp 直覆盖（ETXTBSY）· ✗ systemctl stop（要交互认证）· ✗ 提权）＋ 换完直读子端自报身份，`code_sha`/`code_digest` 对不上 ⇒ 退 1",
			usage:   "zerg agent deploy <机器名> --file <本地件> [--dry-run | --confirm=<机器名> --yes] [--by <谁>] [--json <字段>]",
			arity:   "any",
			args:    []string{"机器名", "--file <本地件>"},
			fields:  agentDeployFields,
			danger: &dangerSpec{dangerD3, "机器名",
				"换该子端的守护件并重启它（在跑任务被打断；旧件被 rename 覆盖、本命令不留回退件）",
				"缺口 GAP-20261018-20 · 同族先例 agent restart（D3 · 远端 ssh 落 F-5 那一格）", true, dangerScopeRemote},
			opened: true,
			run:    cmdAgentDeploy,
		},
		// ---- 批 B5（2026-10-20 · `zerg agent run`）：独立入口 `core/cmd/zerg-agent` 归入命令面 ----
		// 源件 462 行 · 十六枚 Go `flag` **逐字**保留（只把单横线写成双横线；`-version` 除外 ——
		// `zerg version`/全局 `--version` 是身份面唯一真源）。★ 并入**既有** `agent` 族
		// （`ls`/`show`/`ping`/`deploy`/`load`/`unload`/… = 打子端/主控的投影与控制面），✗ 不新开
		// 顶层族；动作名分家：`run` = **在本机跑一个 Agent 任务**（调网关、执行工具、写自己的任务单）。
		// 与 `sched run`／`api serve` 同一条判法（族已在册 ⇒ 只增一格）。
		// **双轨期**：`core/cmd/zerg-agent` 二进制**一字不动** —— 主控调度器
		// （`core/internal/agent/scheduler.go:477` 与 `worker.go:157`）仍 `exec zerg-agent`，
		// 且**靠它的进程内语义码** 0/2/3/4 判重试与死信；本命令走命令面退码表（3/4 已另有所指）
		// ⇒ 两条轨的退码空间**不同**，不许互换（差异 ③ 在实现件件头逐字写着）。
		// 写面（真调模型、真执行工具、写工作目录与任务单）但**不进危险表**，理由与 `sched run`
		// 同：它是操作者当场在本机起的那个任务运行器，跑的是自己给的那段任务、写的是自己的工作区；
		// 工具门在**内层**（`control.NewAgentGateFromRepo` 的规则表 · 缺省 `block` · 装不上则恒拦）
		// ⇒ 本机可逆、无远端副作用。
		{
			path:    []string{"agent", "run"},
			kind:    "AgentRun",
			summary: "在本机**跑一个 Agent 任务**（调网关 → 执行工具 → 写工作区；`--issue` 接单档：读问题单→查因→回填结论→标 resolved/escalated；`--auto-issue` 失败自动挂单 · `--subtask` 子任务结晶 · `--mcp` 挂 MCP 工具 · `--state` 断连恢复；缺令牌即拒跑、工具门缺省恒拦）",
			usage:   agentRunUsageLine,
			arity:   "none",
			args:    []string{"（不收位置参数：任务走 `--task`、问题单走 `--issue`）"},
			fields:  agentRunFields,
			opened:  true,
			run:     cmdAgentRun,
		},
		{
			path:    []string{"agent", "probe"},
			kind:    "AgentProbe",
			summary: "探活诊断（默认经主控；直达属 F-2 例外、要显式 `--direct`）",
			usage:   "zerg agent probe <机器名> [--direct] [--json <字段>]",
			arity:   "any",
			args:    []string{"机器名"},
			fields:  []string{"machine", "healthy", "code_version", "code_sha", "last_seen", "via", "direct_gate"},
			// ★ 缺口 GAP-20260925-23（批⑤ · 2026-10-13）：`dial_addr`（本面若持 `--direct` 会拨的地址）走**可选字段层**。
			optFields: []string{"dial_addr"},
			endpoint:  "GET /api/fleet/status（默认档）",
			run:       cmdAgentProbe,
		},
		// ---- 缺口 `GAP-20260925-50` ②③④ · `agent` 族第二组（2026-09-26）----
		{
			path:     []string{"agent", "reload"},
			machine:  machineNodeName,
			danger:   &dangerSpec{dangerD2, "机器名", "让子端热重载（`POST http://<host>:<port>/infer/reload` · 改子端在跑引擎态 · 必带 `X-Auth-Token`）", "GAP-20261005-38 · 同族先例 core reload（D2）· itask stop（D2）", false, dangerScopeRemote},
			opened:   true,
			kind:     "AgentReload",
			summary:  "子端热重载（`POST http://<host>:<port>/infer/reload` · **必带 `X-Auth-Token`**（不带就回 unauthorized）；子端那三格原样透传）",
			usage:    "zerg agent reload <机器名> --yes [--dry-run] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   agentReloadFields,
			endpoint: "POST http://<host>:<port>/infer/reload（子端 HTTP 面 · 令牌只从凭据链取、走 X-Auth-Token）",
			run:      cmdAgentReload,
		},
		{
			path:     []string{"agent", "registry"},
			machine:  machineNodeName,
			danger:   &dangerSpec{dangerD3, "机器名", "改那台子端盘上的注册表件（`--add`：先校验 → `--dry-run` → `--yes` 才写 → 留 `.bak-registry-add-<日期>` → 写完读回再校，不过即回滚）；`--list` 只读", "GAP-20261005-37 · 同族先例 agent restart（D3）· agent load/unload 已挂危险档", false, dangerScopeRemote},
			opened:   true,
			kind:     "AgentRegistry",
			summary:  "改/看那台子端的注册表（`--list` 只读；`--add` 先校验 → `--dry-run` 先行 → `--yes` 才写 → 留 `.bak-registry-add-<日期>` → 写完读回再校，不过即回滚）",
			usage:    "zerg agent registry <机器名> [--list] [--add <模型名> --file <GGUF> --ctx <N> --mem-gb <N>] [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名", "（`--add` 的模型名 = 名册模型 id，**逐字相同**）"},
			fields:   agentRegistryFields,
			endpoint: "本地件面：子端注册表（`zerg-agentd --registry <件>`；落点现读 `deploy/com.zerg.agent-<机>.plist` 的 `--registry` 实参）；远端机 ⇒ kind=blocked",
			run:      cmdAgentRegistry,
		},
		{
			path:     []string{"agent", "bench"},
			kind:     "AgentBench",
			summary:  "推理测速：**按名字/端口从子端回据里认准**在跑的引擎（`--model` 点名；多枚在跑而没点名 ⇒ 退 2，**不许抓碰巧第一个**），对 `/v1/chat/completions` 发一次定形请求（出 predicted_per_second / prompt_per_second / predicted_n）+ **三态判据** `verdict`：**可读正文 / 只有思考（推理预算不够，未到正文）/ 真乱码** —— 三态各自成立、**都退 0**（引擎输出差 ≠ 命令错，写进 warnings[]）",
			usage:    "zerg agent bench <机器名> [--model <名>] [--n-predict <N>] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   agentBenchFields,
			endpoint: "POST http://<host>:<engine_port>/v1/chat/completions（引擎由子端 `/eggs` 按**名字+端口**认准、`/status` 兜底；取不到 / 说不清是哪一枚 ⇒ blocked 或退 2）",
			run:      cmdAgentBench,
		},
		// ---- R6 · 缺口 `GAP-20260927-04` / `GAP-20260927-202`② · `agent` 族：`engines` / `slots` ----
		// 实现件已落（`family_agent.go`：`cmdAgentEngines`/`cmdAgentSlots` + 两个字段表）⇒ 本处只做命令树登记。
		// ★ 新命令 ⇒ 同批随动：cli-matrix must-fail 格 ＋ cli-contract-baseline ＋ 门69 两半。
		{
			path:     []string{"agent", "engines"},
			kind:     "AgentEngines",
			summary:  "一步看该机在跑哪些引擎（投影子端 /eggs：model · state · pid · port · managed · argv；`/status` 兜底）—— 含**引擎真命令行 argv**（GAP-20260926-01）；多机时不必来回 `egg ls` 或手搓 curl",
			usage:    "zerg agent engines <机器名> [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   agentEnginesFields,
			endpoint: "GET http://<host>:<port>/eggs（子端回据 · /status 兜底；取不到 ⇒ blocked 退 8，不编）",
			// ★ `GAP-20260925-45`（2026-10-10）：引擎实时态三格（`inflight` / `last_output_ago_s` /
			//   `watchdog_verdict`）走**可选字段层**（`optFields`）—— 登在出口白名单里、`--json` 点得出来，
			//   但**不进** `agentEnginesFields` 那一份**冻结**的帮助 / 文档导出面字段表（与 `eggOptionalFields` 同一体例）。
			optFields: agentEnginesOptionalFields,
			run:       cmdAgentEngines,
		},
		{
			path:     []string{"agent", "slots"},
			kind:     "AgentSlots",
			summary:  "该机在跑引擎的槽位占用（直连引擎 /slots 摆槽位、/props 取 total_slots；引擎由子端 /eggs 认准）—— 「谁占了几号槽」的命令面读数",
			usage:    "zerg agent slots <机器名> [--model <名>] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名"},
			fields:   agentSlotsFields,
			endpoint: "GET http://<host>:<engine_port>/slots（+ /props 的 total_slots；非 JSON 数组 ⇒ blocked 退 8）",
			run:      cmdAgentSlots,
		},
		// ---- P0 #232 第四/第五面（2026-10-08 · 本枚）：`agent workers set` / `agent scheduler set` 两条 D2 写面 ----
		// 落点 = **本地** `<仓根>/config/workers.yaml`（✗ 不碰主控 / ✗ 不碰远端）；口径逐字照同族写面先例
		// `itask interval set`（D2 + `--confirm=<本机名>` + `--yes` + `--dry-run` 计划件 + 落地值回读）。
		{
			path:     []string{"agent", "workers", "set"},
			kind:     "AgentWorkersSet",
			summary:  "设某机并发上限（D2 写面 · **只写本机** `config/workers.yaml` · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · 远机 ⇒ 拒执）",
			usage:    "zerg agent workers set <机> --max N [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名（**必须是本机**；远机需子端控制端点 · 本版未开 ⇒ 拒执退 2）"},
			fields:   []string{"machine", "max", "landed", "changed", "path"},
			danger:   &dangerSpec{dangerD2, "本机名", "把本机 devices 条目的 max_workers 设成 --max N（**持久化**到 config/workers.yaml —— scheduler 下次 NewScheduler 读到生效；0 = 暂停派活）—— 可逆：再设回旧值即回原态", "P0 #232 · 同族写面先例 `itask interval set`（D2 + --confirm + --yes + 落地值回读）", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdAgentWorkersSetGuarded,
		},
		{
			path:     []string{"agent", "workers", "show"},
			kind:     "AgentWorkersShow",
			summary:  "读子端并发上限（投影本机 `<仓根>/config/workers.yaml` 的 devices 段：每机 hostname · max_workers · state · 是否本机；只读 · 一个字节不写）",
			usage:    "zerg agent workers show [--json <字段>]",
			arity:    "none",
			fields:   []string{"hostname", "max_workers", "state", "local"},
			endpoint: "",
			run:      cmdAgentWorkersShow,
		},
		{
			path:     []string{"agent", "scheduler", "set"},
			kind:     "AgentSchedulerSet",
			summary:  "设调度器旋钮（D2 写面 · 写 `config/workers.yaml` 顶 `scheduler:` 段 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2）",
			usage:    "zerg agent scheduler set [--poll-interval <N>] [--max-retries <N>] [--cooldown <N>] [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "none",
			fields:   []string{"path", "poll_interval", "max_retries", "cooldown", "changed"},
			danger:   &dangerSpec{dangerD2, "本机名", "改本地 `config/workers.yaml` 的 `scheduler:` 段（轮询间隔 / 死信阈值 / 失败冷却 —— scheduler.go 的 Start()/handleFailure() 读侧接线；未给值时用编译期常量兜底）—— 可逆：再设回旧值即回原态", "P0 #232 · 同族写面先例 `itask interval set`（D2 + --confirm + --yes + 落地值回读）", true, dangerScopeRepo},
			opened:   true,
			endpoint: "",
			run:      cmdAgentSchedulerSetGuarded,
		},
		// ---- 缺口 `GAP-20260927-195` · `agent` 族：`agent restart`（D3 · 已开放 · 照同族 `core restart` 形状）----
		{
			path:    []string{"agent", "restart"},
			machine: machineNodeName,
			kind:    "AgentRestart",
			summary: "重启子端进程（危险 D3 · 已开放：确认档齐就真执行 `launchctl kickstart -k`；**改令牌文件对运行中的子端无效，必须重启进程才重读令牌**；远端机走 ssh 属例外清单 ⇒ 退 8）",
			usage:   "zerg agent restart <机器名> --confirm=<机器名> --yes [--dry-run] [--json <字段>]",
			arity:   "any",
			args:    []string{"机器名"},
			fields:  []string{"machine", "host", "port", "branch", "readiness", "changed", "audit"},
			danger: &dangerSpec{dangerD3, "机器名",
				"重启该子端进程（**在跑的任务会被打断** · 起时/中断的连接都回不来）；治的是「改令牌文件对运行中的子端无效」",
				"缺口 GAP-20260927-195 · 照同族已开放的 `core restart`（family_core_restart.go）形状", true, dangerScopeRemote},
			opened: true,
			run:    cmdAgentRestart,
		},
		{
			path:    []string{"agent", "bootstrap"},
			machine: machineNodeName,
			summary: "子端引导的**域选择判定 + 可打印计划**（只读 · D3 真装未开放）—— 用户域 LaunchAgent vs 系统域 LaunchDaemon；取不到夹具 ⇒ 不给结论",
			usage:   "zerg agent bootstrap <机器名> --confirm=<机器名> --yes [--dry-run]",
			arity:   "any",
			args:    []string{"机器名"},
			danger: &dangerSpec{dangerD3, "机器名",
				"在目标机上装/起子端（**F-1 子端引导例外**：不经主控的显式命令；会改目标机状态）",
				"§十五.4 F-1 · §九 M20 · 开工单 T-43", true, dangerScopeRemote},
			run: cmdAgentBootstrap,
		},
		{
			path:    []string{"agent", "unload"},
			machine: machineNodeName,
			danger: &dangerSpec{dangerD3, "机器名",
				"卸掉该子端上的模型（**在跑的任务会被打断**）· 幂等：已在未装载态按「已在该状态」报 · 内部 = 控制面 `POST /api/control/unload`",
				"§三 B 族 · §九 M4 · 开工单 T-43 · 同族已开放先例 `egg stop`（同为控制面卸载）", true, dangerScopeRemote},
			opened:   true,
			summary:  "卸载子端上的模型（D3 写面：`--dry-run` 先行出计划件 · 真跑要 `--confirm=<机器名>` + `--yes` · 内部 = 控制面 `POST /api/control/unload`）",
			usage:    "zerg agent unload <机器名> <模型> --confirm=<机器名> --yes [--dry-run]",
			arity:    "any",
			args:     []string{"机器名", "模型"},
			endpoint: "POST /api/control/unload（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdAgentUnloadGuarded,
		},
		{
			path:     []string{"agent", "load"},
			machine:  machineNodeName,
			danger:   &dangerSpec{dangerD2, "机器名", "把模型装进该子端（占内存/显存槽 · 单槽机是串行的）· 内部 = 控制面 `POST /api/control/load`", "§三 B 族 · §九 M5 · 开工单 T-43 · 同族已开放先例 `egg run`（同为控制面装载）", true, dangerScopeRemote},
			opened:   true,
			summary:  "加载模型到子端（D2 写面：`--dry-run` 先行出计划件 · 真跑要 `--yes` · 内部 = 控制面 `POST /api/control/load`）",
			usage:    "zerg agent load <机器名> <模型> --yes [--dry-run]",
			arity:    "any",
			args:     []string{"机器名", "模型"},
			endpoint: "POST /api/control/load（控制层既有路由 · 命令面不新开一条路）",
			run:      cmdAgentLoadGuarded,
		},
		{
			path:    []string{"agent", "reap"},
			machine: machineNodeName,
			summary: "回收残留干跑单（真回收危险 D3 · 本版未开放；**入库件永不进候选 = 红线**）",
			usage:   "zerg agent reap [--node <机>] --dry-run --min-age-days <n>   # 干跑单（带 plan_id + 逐件证据）\n       zerg agent reap [--node <机>] --confirm=<机器名> --yes [--dry-run]   # 真回收（本版未开放）",
			arity:   "any",
			args:    []string{"机器名"},
			danger: &dangerSpec{dangerD3, "机器名",
				"按「声明树 ↔ 现值树」差集回收闲置资源（**默认干跑**；入库件永不进候选 = 红线）",
				"§九 M9 · §十五.2 三档回收权 · RC11 红线 · 开工单 T-54", true, dangerScopeRemote},
			run: cmdAgentReap,
		},
	}
}
