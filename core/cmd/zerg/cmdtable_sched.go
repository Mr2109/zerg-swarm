package main

// 命令表 · sched 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_sched() []*command {
	return []*command{
		// ---- 面 G · R7 · 新族 `sched`（暂停派活 / 并发旋钮 / 调度器状态）----
		// 缺口 GAP-20260927-210（P0）+ GAP-20260929-26（P1）。show 只读；pause/resume/set 改调度器
		// 状态（可逆面：再 resume/set 回去即可）。实现件已落：family_task.go（命令面）+
		// core/internal/api/master_scheduler.go（服务端 /api/sched* 四处理器）。
		// ★ 新顶层族 ⇒ 同批随动：cli-matrix must-fail 格 ＋ cli-contract-baseline ＋ 门69 两半。
		{
			path:     []string{"sched", "show"},
			kind:     "SchedShow",
			summary:  "调度器状态（只读 · 并发 x/y · 队列 n · 在跑领取/心跳面）",
			usage:    "zerg sched show [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数）"},
			fields:   []string{"kind", "id", "status", "model", "concurrency", "queued", "waiting_retry", "claimed_by", "claimed_at", "heartbeat_at"},
			endpoint: "GET /api/sched",
			run:      cmdSchedShow,
		},
		{
			path:     []string{"sched", "pause"},
			kind:     "SchedPause",
			summary:  "暂停派活（D2 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · **在跑的不动** —— 要停在跑的走 task terminate）",
			usage:    "zerg sched pause [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数）"},
			fields:   []string{"kind", "id", "status", "concurrency", "queued", "waiting_retry"},
			danger:   &dangerSpec{dangerD2, "本机名", "暂停派活（不再起新的任务；**在跑的不动** —— 要停在跑的走 `task terminate`）—— 可逆：`sched resume` 即回原态", "缺口 GAP-20261005-10 · 同族写面先例 `task cancel`（D2 + --confirm + --yes）· §九 M3 C1", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/sched/pause",
			run:      cmdSchedPauseGuarded,
		},
		{
			path:     []string{"sched", "resume"},
			kind:     "SchedResume",
			summary:  "恢复派活（D2 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · 回到设定并发并立即派发积压）",
			usage:    "zerg sched resume [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数）"},
			fields:   []string{"kind", "id", "status", "concurrency", "queued", "waiting_retry"},
			danger:   &dangerSpec{dangerD2, "本机名", "恢复派活（回到设定并发并立即派发积压）—— 可逆：`sched pause` 即回原态", "缺口 GAP-20261005-10 · 同族写面先例 `task cancel`（D2 + --confirm + --yes）· §九 M3 C1", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/sched/resume",
			run:      cmdSchedResumeGuarded,
		},
		{
			path:     []string{"sched", "set"},
			kind:     "SchedSet",
			summary:  "设并发上限（D2 写面 · `--dry-run` 零副作用 · 缺 `--confirm=<本机名>`/`--yes` ⇒ 2 · 非负整数 N 走位置参数）",
			usage:    "zerg sched set <N> [--dry-run | --confirm=<本机名> --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"并发上限 N（非负整数）"},
			fields:   []string{"kind", "id", "status", "concurrency", "queued", "waiting_retry"},
			danger:   &dangerSpec{dangerD2, "本机名", "把调度器并发上限设成 N（显式旋钮）—— 可逆：再 `sched set <旧 N>` 即回原态", "缺口 GAP-20261005-10 · 同族写面先例 `task cancel`（D2 + --confirm + --yes）· §九 M3 C1", true, dangerScopeRemote},
			opened:   true,
			endpoint: "POST /api/sched/set?max=<N>",
			run:      cmdSchedSetGuarded,
		},
		// ---- 批 B2（2026-10-20 · `zerg sched run`）：独立入口 `core/cmd/zerg-scheduler` 归入命令面 ----
		// 源件 104 行 · 旗标五枚**逐字**保留（`--workdir` / `--container` / `--agent-bin` / `--dev` /
		// `--repo`），只把 Go `flag` 的单横线写成双横线。★ 与上面四条的**分家**：`show|pause|resume|set`
		// = 打控制面 `/api/sched*`（管**在跑的那个**）；本格 `run` = **在本机起进程**（源件语义）。
		// **双轨期**：`core/cmd/zerg-scheduler` 二进制一字不动。实现在 `family_sched_run.go`
		// （件头逐条写清与源件的四处差异 —— ✗ 不是 `family_sched.go`：那个名字已被 sched 写面确认门占着）。
		// 写面（真派单：起 worker · 改任务单状态）但**不进危险档**，理由照源件口径写清：
		// 它是「操作者当场在本机起的那个进程」（本来由 launchd/systemd 拉起），Ctrl-C/SIGTERM 即停
		// ⇒ 本机可逆、无远端效应；与 `trace` 那格同一种「不进危险表」的判法。
		{
			path:     []string{"sched", "run"},
			kind:     "SchedRun",
			summary:  "在本机起一个调度器（**常驻档**：轮询任务单目录 → 按优先级派单 → 每 10s 报状态 → SIGINT/SIGTERM 优雅停；容器/开发两档照源件 `zerg-scheduler`）",
			usage:    "zerg sched run [--workdir <目录>] [--container] [--agent-bin <路径>] [--dev] [--repo <路径>]",
			arity:    "none",
			args:     []string{"（不收位置参数：工作目录走 `--workdir`）"},
			endpoint: "",
			opened:   true,
			run:      cmdSchedRun,
		},
	}
}
