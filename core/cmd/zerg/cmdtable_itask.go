package main

// 命令表 · itask 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_itask() []*command {
	return []*command{
		// ---- 批 D · T-49 内部任务引擎族 `itask`（§7.1 P8 · §5.1 九条端点）----
		{
			path:     []string{"itask", "ls"},
			kind:     "InternalTask",
			summary:  "内部任务清单（16 类 · 含冷却与最近执行 · 投影 /api/internal-tasks）",
			usage:    "zerg itask ls [--json <字段>]",
			fields:   []string{"id", "description", "cooldown", "default_hours", "auto_run", "last_run", "state"},
			endpoint: "GET /api/internal-tasks",
			run:      cmdItaskLs,
		},
		{
			path:     []string{"itask", "state"},
			kind:     "InternalTaskState",
			summary:  "引擎状态（投影 /api/internal-tasks/state）",
			usage:    "zerg itask state [--json <字段>]",
			fields:   []string{"note", "state"},
			endpoint: "GET /api/internal-tasks/state",
			run:      cmdItaskState,
		},
		{
			path:     []string{"itask", "mode"},
			kind:     "InternalTaskMode",
			summary:  "自动运行开关现值（只读面；写面 `itask mode set` 已开放 · 见下条三格）",
			usage:    "zerg itask mode [--json <字段>]",
			fields:   []string{"modes", "note", "state"},
			endpoint: "GET /api/internal-tasks/modes",
			run:      cmdItaskMode,
		},
		{
			path:     []string{"itask", "interval"},
			kind:     "InternalTaskInterval",
			summary:  "周期现值（只读面；写面 `itask interval set` 已开放 · 见下条三格）",
			usage:    "zerg itask interval [--json <字段>]",
			fields:   []string{"intervals", "note", "state"},
			endpoint: "GET /api/internal-tasks/intervals",
			run:      cmdItaskInterval,
		},
		{
			path:     []string{"itask", "interval", "set"},
			kind:     "InternalTaskIntervalSet",
			summary:  "设内部任务周期（D2 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · 给 `all` 则逐条设）",
			usage:    "zerg itask interval set <任务|all> --hours N [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id（照 `itask ls` 的 id 那一格，或 `all` 逐条设）"},
			fields:   []string{"id", "hours", "landed", "message"},
			danger:   &dangerSpec{dangerD2, "本机名", "把该内部任务的循环周期设成 --hours N 小时（**持久化**到 intervals.json —— 0 = 取消；主控 goroutine 每 60s 检查到点）—— 可逆：再设回旧周期即回原态", "§7.1 P8 · §5.1 POST /api/internal-tasks/{id}/interval · 同族写面先例 `itask stop`（D2 + --confirm + --yes）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/internal-tasks/{id}/interval",
			run:      cmdItaskIntervalSetGuarded,
		},
		{
			path:     []string{"itask", "mode", "set"},
			kind:     "InternalTaskModeSet",
			summary:  "设内部任务的自动运行开关（D2 写面 · `--on`/`--off` 二选一 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2）",
			usage:    "zerg itask mode set <任务 id> [--on | --off] [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id（照 `itask ls` 的 id 那一格）"},
			fields:   []string{"id", "auto_run", "landed", "message"},
			danger:   &dangerSpec{dangerD2, "本机名", "把该内部任务的自动运行开关设成 `--on`（自动运行）/`--off`（手动运行）（**持久化**到 internal_modes.json —— 重启恢复）—— 可逆：再设回原开关即回原态", "§7.1 P8 · §5.1 POST /api/internal-tasks/{id}/mode · 缺口 GAP-20260927-205 · 同族写面先例 `itask interval set`（D2 + --confirm + --yes）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/internal-tasks/{id}/mode",
			run:      cmdItaskModeSetGuarded,
		},
		{
			path:     []string{"itask", "start"},
			kind:     "InternalTaskStart",
			summary:  "起内部任务引擎（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2）",
			usage:    "zerg itask start [--dry-run | --yes] [--json <字段>]",
			fields:   []string{"message", "state"},
			danger:   &dangerSpec{dangerD2, "（群级：开关不分机）", "让内部任务引擎开始按周期跑（会自动占机器与模型槽）", "§7.1 P8 · §5.1 POST /api/internal-tasks/start · 开工单 T-49 · 同族写面先例 `itask stop`（D2）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/internal-tasks/start",
			run:      cmdItaskStartGuarded,
		},
		{
			path:     []string{"itask", "stop"},
			kind:     "InternalTaskStop",
			summary:  "停内部任务引擎（D2 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · 写**持久化**的用户意图 `stopped=true`）",
			usage:    "zerg itask stop [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			fields:   []string{"message", "state"},
			danger:   &dangerSpec{dangerD2, "本机名", "让内部任务引擎停下（把**持久化**的用户意图 `stopped` 置 true —— 周期/空闲触发都**不再起新的**内部任务；**在跑的这一轮不动**）—— 可逆：再起引擎即回原态", "§7.1 P8 · §5.1 POST /api/internal-tasks/stop · 开工单 T-49 · 同族写面先例 `sched pause`（D2 + --confirm + --yes）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/internal-tasks/stop",
			run:      cmdItaskStopGuarded,
		},
		{
			path:     []string{"itask", "run"},
			kind:     "InternalTaskRun",
			summary:  "手动跑一条内部任务（D2 写面 · `--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 绕过它的冷却）",
			usage:    "zerg itask run <任务 id> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"任务 id"},
			fields:   []string{"task_id", "type", "message"},
			danger:   &dangerSpec{dangerD2, "任务 id", "立刻跑一次该内部任务（绕过它的冷却 · 会占机器）", "§7.1 P8 · §5.1 POST /api/internal-tasks/{id}/run · 开工单 T-49 · 同族写面先例 `itask stop`（D2）", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/internal-tasks/{id}/run",
			run:      cmdItaskRunGuarded,
		},
	}
}
