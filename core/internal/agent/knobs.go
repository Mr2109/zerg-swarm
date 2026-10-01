package agent

// knobs.go — GAP-20260927-232：内核 internal 面的运行旋钮登记表。
//
// 病灶：51 处运行旋钮全是编译期常量，命令树里能直接写的 = 0 条 ⇒ 出事只能改源码重编。
// 本件只做**内核 internal 面那一半**（命令面写面归 `core/cmd/zerg` = L2 路；同 `-211` 另一面）：
// 建一张「旋钮表」——每个旋钮记 名字 / 现值 / 出处(件:行) / 有没有面，并配一条判据
// （`knobs_test.go`）：「扫描面内新增编译期常量旋钮必须在本表登记，否则测试红」。
// ★ 本表**不改任何旋钮语义**（51 处语义本批不动）。

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
}

// Knobs — 回读旋钮表（导出 · 供命令/文档面消费）。
func Knobs() []Knob { return append([]Knob(nil), knobs...) }
