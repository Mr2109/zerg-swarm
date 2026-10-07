package agent

// knobs.go — GAP-20260927-232：内核 internal 面的运行旋钮登记表。
//
// 病灶：51 处运行旋钮全是编译期常量，命令树里能直接写的 = 0 条 ⇒ 出事只能改源码重编。
// 本件只做**内核 internal 面那一半**（命令面写面归 `core/cmd/zerg` = L2 路；同 `-211` 另一面）：
// 建一张「旋钮表」——每个旋钮记 名字 / 现值 / 出处(件:行) / 有没有面，并配一条判据
// （`knobs_test.go`）：「扫描面内新增编译期常量旋钮必须在本表登记，否则测试红」。
// ★ 本表**不改任何旋钮语义**（51 处语义本批不动）。
// ★ 覆盖扩展（GAP-20260927-232 leg③ · 第二批 · 2026-10-05）：扫描面由最初 6 件扩到
// `core/internal/agent` 与 `core/internal/gateway` 两包里**带顶层运行旋钮的全部件**
// （名单见 knobs_test.go 的 knobsFiles）；本表随之补齐，逐条 `Face:"无面"` = 设计为编译期常量。

// Knob — 一个运行旋钮的登记行。
type Knob struct {
	Name    string // 旋钮名（稳定标识：<面>.<常量名>）
	Current string // 现值（以源码现读为准，序列化形态）
	Source  string // 出处：件:行（现读）
	Face    string // 面：只读面 / 写面（未开放）/ 无面（设计为编译期常量）
	Note    string // 说明
}

// knobs — 内核 internal 面旋钮登记表（GAP-20260927-232）。值域以源码现读为准；
// 出处行号随源码变动须同步（判据 `TestKnobsRegistered` 只查「有没有登记」，不查行号）。
var knobs = []Knob{
	// ── agent 包（调度/worker/遍历/退避/容器） ──
	{Name: "scheduler.defaultMaxWorkers", Current: "2", Source: "core/internal/agent/scheduler.go:25", Face: "无面", Note: "调度默认并发上限（另有 workers.yaml 覆盖）"},
	{Name: "scheduler.defaultCooldown", Current: "30s", Source: "core/internal/agent/scheduler.go:26", Face: "无面", Note: "失败重派冷却"},
	{Name: "scheduler.maxRetries", Current: "3", Source: "core/internal/agent/scheduler.go:27", Face: "无面", Note: "死信阈值"},
	{Name: "scheduler.pollInterval", Current: "5s", Source: "core/internal/agent/scheduler.go:28", Face: "无面", Note: "轮询间隔"},
	{Name: "scheduler.workersConfigPath", Current: "config/workers.yaml", Source: "core/internal/agent/scheduler.go:29", Face: "无面", Note: "并发配置文件路径"},
	{Name: "worker.workerTimeout", Current: "10m", Source: "core/internal/agent/worker.go:23", Face: "无面", Note: "worker 执行超时"},
	{Name: "worker.maxRetryBeforeDead", Current: "3", Source: "core/internal/agent/worker.go:33", Face: "无面", Note: "重试上限（超过标记 dead）"},
	{Name: "worker.cooldownDuration", Current: "30s", Source: "core/internal/agent/worker.go:34", Face: "无面", Note: "worker 失败冷却"},
	{Name: "walk.walkMaxEntries", Current: "200000", Source: "core/internal/agent/walkbudget.go:14", Face: "无面", Note: "单次遍历条数上限"},
	{Name: "walk.walkMaxDur", Current: "20s", Source: "core/internal/agent/walkbudget.go:15", Face: "无面", Note: "单次遍历时间上限"},
	{Name: "recovery.DefaultMaxRetries", Current: "3", Source: "core/internal/agent/recovery.go:23", Face: "无面", Note: "恢复默认重试"},
	{Name: "recovery.DefaultBaseBackoff", Current: "2s", Source: "core/internal/agent/recovery.go:24", Face: "无面", Note: "退避基数"},
	{Name: "docker.defaultImage", Current: "zerg-dev", Source: "core/internal/agent/docker_worker.go:35", Face: "无面", Note: "默认镜像名"},
	{Name: "docker.defaultTimeout", Current: "10m", Source: "core/internal/agent/docker_worker.go:36", Face: "无面", Note: "容器执行超时"},
	{Name: "docker.defaultNetwork", Current: "host", Source: "core/internal/agent/docker_worker.go:37", Face: "无面", Note: "容器网络模式"},
	{Name: "docker.agentBinaryInHost", Current: "/tmp/zerg-agent-linux-arm64", Source: "core/internal/agent/docker_worker.go:38", Face: "无面", Note: "宿主机 zerg-agent 路径"},
	{Name: "docker.agentBinaryInCont", Current: "/zerg-agent", Source: "core/internal/agent/docker_worker.go:39", Face: "无面", Note: "容器内 zerg-agent 路径"},
	// ── gateway 包（熔断/会话/端口） ──
	{Name: "gateway.circuitCooldown", Current: "30s", Source: "core/internal/gateway/gateway.go:230", Face: "无面", Note: "熔断自动恢复冷却"},
	{Name: "gateway.circuitFailThreshold", Current: "8", Source: "core/internal/gateway/gateway.go:234", Face: "无面", Note: "熔断失败阈值"},
	{Name: "gateway.healthyTripLimit", Current: "10", Source: "core/internal/gateway/gateway.go:238", Face: "无面", Note: "健康跳闸上限"},
	{Name: "gateway.startupGrace", Current: "30s", Source: "core/internal/gateway/gateway.go:251", Face: "无面", Note: "启动宽限"},
	{Name: "gateway.sessionTTL", Current: "30m", Source: "core/internal/gateway/gateway.go:262", Face: "无面", Note: "会话 TTL"},
	{Name: "gateway.maxSessionBindings", Current: "512", Source: "core/internal/gateway/gateway.go:267", Face: "无面", Note: "绑定表硬上限（O-7）"},
	{Name: "gateway.maxSessionTokens", Current: "131072", Source: "core/internal/gateway/gateway.go:271", Face: "无面", Note: "会话 token 上限"},
	{Name: "gateway.defaultAgentPort", Current: "8100", Source: "core/internal/gateway/gateway.go:1596", Face: "无面", Note: "子端默认端口"},
	// ── agent 包（本批补齐：bash 输出 / 回合上限 / 热记忆 / 网络检索缓存 / 改端口） ──
	{Name: "bash.bashOutLimit", Current: "6000", Source: "core/internal/agent/bash_v101.go:37", Face: "无面", Note: "bash 默认输出上限（弱模型防淹没）"},
	{Name: "bash.bashHeadLimit", Current: "2000", Source: "core/internal/agent/bash_v101.go:38", Face: "无面", Note: "bash 头部保留"},
	{Name: "bash.bashTailLimit", Current: "4000", Source: "core/internal/agent/bash_v101.go:39", Face: "无面", Note: "bash 尾部保留（错误/日志终点常在尾）"},
	{Name: "maxturns.DefaultMaxTurns", Current: "100", Source: "core/internal/agent/max_turns.go:22", Face: "无面", Note: "主循环回合上限"},
	{Name: "maxturns.SubAgentMaxTurns", Current: "20", Source: "core/internal/agent/max_turns.go:26", Face: "无面", Note: "子代理回合上限（= DefaultMaxTurns/5）"},
	{Name: "memory.hotLimit", Current: "4096", Source: "core/internal/agent/memory.go:22", Face: "无面", Note: "热记忆上限（字节）"},
	{Name: "nettools.x3RewritePort", Current: "9001", Source: "core/internal/agent/net_tools.go:343", Face: "无面", Note: "X3 直连改写端口"},
	{Name: "nettools.searchCacheTTL", Current: "300s", Source: "core/internal/agent/net_tools.go:425", Face: "无面", Note: "网络检索缓存 TTL（5*60 秒）"},
	// ── gateway 包（本批补齐：工具输出压缩 / DS4 路由 / 孵蛋等待 / 路由打分 / 输出预算 / 前缀缓存 / SSE 尾窗 / token 上限兜底） ──
	{Name: "compactor.toolOutputHeadLines", Current: "60", Source: "core/internal/gateway/compactor.go:17", Face: "无面", Note: "工具输出保留开头行数"},
	{Name: "compactor.toolOutputTailLines", Current: "40", Source: "core/internal/gateway/compactor.go:18", Face: "无面", Note: "工具输出保留结尾行数"},
	{Name: "compactor.toolOutputMaxLen", Current: "800", Source: "core/internal/gateway/compactor.go:19", Face: "无面", Note: "单个工具输出最大保留字符"},
	{Name: "ds4route.ds4MemGB", Current: "86", Source: "core/internal/gateway/ds4_route.go:40", Face: "无面", Note: "DS4 路由内存判据（GB）"},
	{Name: "eggwarmup.eggLoadTimeout", Current: "180s", Source: "core/internal/gateway/egg_warmup.go:38", Face: "无面", Note: "孵蛋（预热）等待上限"},
	{Name: "eggwarmup.eggPeekLimit", Current: "4096", Source: "core/internal/gateway/egg_warmup.go:39", Face: "无面", Note: "预热认错只读字节数（4<<10）"},
	{Name: "ledgerroute.ledgerNoFitPenalty", Current: "-6", Source: "core/internal/gateway/ledger_route.go:36", Face: "无面", Note: "路由打分：装不下罚分"},
	{Name: "ledgerroute.ledgerLoadingPenalty", Current: "-3", Source: "core/internal/gateway/ledger_route.go:37", Face: "无面", Note: "路由打分：目标正在加载罚分"},
	{Name: "ledgerroute.ledgerColdPenalty", Current: "-2", Source: "core/internal/gateway/ledger_route.go:38", Face: "无面", Note: "路由打分：目标未驻留（冷/温态）罚分"},
	{Name: "outbudget.bytesPerToken", Current: "4", Source: "core/internal/gateway/outbudget.go:20", Face: "无面", Note: "token↔字节 估算比"},
	{Name: "outbudget.defaultCtxWindow", Current: "32768", Source: "core/internal/gateway/outbudget.go:89", Face: "无面", Note: "卵未声明上下文时的保守默认"},
	{Name: "outbudget.minReserveTokens", Current: "2048", Source: "core/internal/gateway/outbudget.go:90", Face: "无面", Note: "安全余量下限"},
	{Name: "outbudget.maxOutCeiling", Current: "32768", Source: "core/internal/gateway/outbudget.go:91", Face: "无面", Note: "单次输出上限"},
	{Name: "outbudget.minOutTokens", Current: "1024", Source: "core/internal/gateway/outbudget.go:92", Face: "无面", Note: "最小输出额度"},
	{Name: "prefixcache.defaultPrefixWindowN", Current: "50", Source: "core/internal/gateway/prefix_cache.go:41", Face: "无面", Note: "前缀缓存滑动窗口样本数"},
	{Name: "prefixcache.defaultPrefixWindowDur", Current: "10m", Source: "core/internal/gateway/prefix_cache.go:43", Face: "无面", Note: "前缀缓存滑动窗口时长"},
	{Name: "prefixcache.defaultPrefixMinAlertSamples", Current: "3", Source: "core/internal/gateway/prefix_cache.go:45", Face: "无面", Note: "前缀告警最小样本数"},
	{Name: "prefixcache.prefixAlertAbsDrop", Current: "0.30", Source: "core/internal/gateway/prefix_cache.go:47", Face: "无面", Note: "前缀命中率绝对跌幅告警阈值"},
	{Name: "prefixcache.prefixAlertRelFactor", Current: "0.60", Source: "core/internal/gateway/prefix_cache.go:49", Face: "无面", Note: "前缀命中率相对因子告警阈值"},
	{Name: "prefixcache.maxPrefixAlerts", Current: "100", Source: "core/internal/gateway/prefix_cache.go:51", Face: "无面", Note: "前缀告警保留上限"},
	{Name: "prefixcache.maxPrefixModels", Current: "64", Source: "core/internal/gateway/prefix_cache.go:53", Face: "无面", Note: "跟踪的前缀模型数上限"},
	{Name: "prefixcache.defaultFlushEvery", Current: "5s", Source: "core/internal/gateway/prefix_cache.go:56", Face: "无面", Note: "前缀缓存落盘间隔"},
	{Name: "prefixcache.defaultFlushSamples", Current: "10", Source: "core/internal/gateway/prefix_cache.go:57", Face: "无面", Note: "落盘前最小样本数"},
	{Name: "prefixcache.maxUnknownLastKeys", Current: "24", Source: "core/internal/gateway/prefix_cache.go:61", Face: "无面", Note: "未知 last-key 保留上限"},
	{Name: "prefixcache.maxUnknownShapes", Current: "16", Source: "core/internal/gateway/prefix_cache.go:673", Face: "无面", Note: "未知形态保留上限"},
	{Name: "ssestee.sseTeeLimit", Current: "16384", Source: "core/internal/gateway/sse_usage_tee.go:21", Face: "无面", Note: "SSE 尾窗字节数（16<<10）"},
	{Name: "tokencap.maxTokensCapDefault", Current: "32768", Source: "core/internal/gateway/tokencap.go:39", Face: "无面", Note: "token 上限兜底默认（= maxOutCeiling）"},
}

// Knobs — 回读旋钮表（导出 · 供命令/文档面消费）。
func Knobs() []Knob { return append([]Knob(nil), knobs...) }
