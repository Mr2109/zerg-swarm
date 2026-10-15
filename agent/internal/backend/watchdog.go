package backend

// watchdog.go —— 活性看门狗（虫须判活）：**两把正交时钟**（liveness ＋ progress）。
//
// 设计真源：Zerg-内部文档/01-设计/设计-虫族子代理-20261014.md
//   · §七.3 J4/J5（两把时钟 · 判卡只看 progress · 两级判卡 · 重连封顶）
//   · §十二 Q4「无进展看门狗的判据与参数」（no_progress / prefill 预算 / max_stale_kills / grace）
// 历史真源：Zerg-内部文档/01-设计/设计-活性看门狗.md（Mr2109 2026-09-15 提出并认可）
// 缺口真源：`GAP-20261014-61`（看门狗 7m30s 结构性误杀长 prefill · 比值 3.07×）
//
// 要解决的问题（三条都是生产实证）：
//   · 非流式：引擎要**等整篇生成完**才回响应头 ⇒ 固定的"90 s 首字节超时"把**正常的长文**误杀；
//   · 长 prefill：几十分钟的预填充期间**一个 token 都不出** ⇒ 同样被误杀；
//   · ★ **结构性误杀**（`GAP-20261014-61`）：旧口径把「无输出窗口」的乘积
//     `Window 90 s × MaxWindows 5 = 450 s` 当**硬上界**，而 132K 上下文的一次**合法** prefill
//     实测约 23 min（1380 s）⇒ 比值 **3.07×** ⇒ 大上下文档（128K 档）的卵在 prefill 阶段被
//     **结构性判死**（它本来算得完，是预算不够，不是卡死）。
//     ⇒ ★ ✗ 不许再用「一个更大的数」替代判据（下一次更大的 ctx 还得再挪一次）。
//
// ★ 判据换形（两把正交时钟 · 本节是口径真源）：
//
//	liveness 时钟（`last_heartbeat_at`）—— **任何活动都续**：
//	    流式 token / SSE 保活注释 / 引擎心跳 / **本单元 CPU 工时在涨**。
//	    它只回答「这枚卵还活着吗」，**✗ 绝不**回答「它有没有推进」。
//	progress 时钟（`last_progress_at`）—— **只由「真实 chunk 计数」续**：
//	    引擎侧已处理 token 计数差分 / 已交付 chunk 计数差分（两者都是**计数**）。
//	    ✗ **绝不由时间戳续**（重试尝试开头会重置时间戳 ⇒ 计数被永久清零、看门狗永不收口）；
//	    ✗ 也不由「推理在跑 / 忙等占 CPU」续（那正是要判死的那一类）。
//
//	**判卡只看 progress**（报告里两把锁都出）：liveness 再密也救不回冻结的 progress。
//
// ★ 预算（✗ 不再有写死的硬上界）：
//
//	· `no_progress` 默认 **45 min**，`0` ＝ **关**（进度判卡关闭 ⇒ 只保留「彻底停摆」那条早杀）；
//	· prefill 预算 = **`est_ctx ÷ 实测 tok/s × 2`**（例：132K ÷ 96 tok/s × 2 ≈ 46 min）——
//	  按**现读窗口 ÷ 实测速率**算出来，✗ 不写死；
//	· **有效预算 = `max(no_progress, prefill 预算, 单卵时限)`**（几把尺同时在场时取宽的那把）。
//	· ★ **单卵时限（`EggMax` · CLI `--timeout`）已降为「最终兜底」**：它**不再按时限直接判死**
//	  （判死只看有没有进度），而是**并入有效预算、只抬不降**（`ProgressBudget`）。
//	  撤掉独立墙钟判死的理由（严格涵盖的证明）见 `ProgressBudget` 的函数头注。
//	· ★ 每一层的硬窗口都必须**可配**（env 旋钮齐备）或**可显式豁免**，且**豁免要留痕**（`Waived`）。
//
// ★ 判卡后**两级、且有界**（J5）：先 **grace**（请求优雅退出，默认 30 s；批作业档 120–300 s）
//   ⇒ grace 到期仍无进展才**硬断**（SIGKILL 语义）；每次重连计数并封顶 **`max_stale_kills = 3`**
//   （超了就 `abandoned`）—— ✗ 无上限的进度重置会变死循环。
//
// 三级证据里**保留**的那条「真停摆」早杀：liveness 与 progress **双双冻结**（CPU 工时也不动、也无 chunk）
// ⇒ 立刻判死（`stuck_cpu_stalled`）。这是"彻底停摆"的证据，与"忙但不推进"区分开（后者走预算）。
//
// 为什么用"本单元 CPU 工时"而不是整机 CPU%/GPU%：整机指标全机共享，隔壁卵/别的进程一忙就会
// 给"这枚卵还在干活"作伪证（见 monitor.EggCPUUsecFromCgroup 注释）。
// ★ 注意口径：CPU 工时是 **liveness** 证据，**不**续 progress。

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// WatchdogVerdict 判词（进观测面与错误响应，供主控 failover 与人工复盘）。
type WatchdogVerdict string

const (
	WatchdogOK              WatchdogVerdict = "ok"
	WatchdogExtended        WatchdogVerdict = "extended"
	WatchdogStuckCPUStalled WatchdogVerdict = "stuck_cpu_stalled"
	WatchdogStuckNoProgress WatchdogVerdict = "stuck_no_progress"
	WatchdogDegraded        WatchdogVerdict = "degraded"
	// WatchdogDeadUnit 单元已经**不在活动**（2026-09-19 ②，真机：`/eggs` 报 ready + watchdog ok，
	// 而机器上进程为零 ⇒ 主控把活儿派到一个空壳上）。
	//
	// 与 stuck_* 的区别（两者不许混为一谈）：
	//   - stuck_* = 单元还在、但「无输出且不推进」（活着的卡住）；
	//   - dead_unit = 单元**本身**都没了/失败了（进程已不在，evidence 是单元的 ActiveState）。
	// 判词是**档位**，理由里写清是哪一种（missing / inactive / failed + systemd 原文各项）。
	WatchdogDeadUnit WatchdogVerdict = "dead_unit"

	// ── 判卡后的**两级动作**判词（J5 · 与「判卡」本身分开：判卡是 `stuck_no_progress`，动作看这两格）──
	//
	// WatchdogStalledGrace 第一级：**请求优雅退**（SIGTERM 语义 —— 让引擎自己收尾，不夺它的收尾机会）。
	WatchdogStalledGrace WatchdogVerdict = "stalled_grace"
	// WatchdogStalledHard 第二级：**硬断**（SIGKILL 语义 —— grace 到期仍无进展才动它）。
	WatchdogStalledHard WatchdogVerdict = "stalled_hard"
	// WatchdogAbandoned 重连封顶用尽（`max_stale_kills`）：**放弃**再重置/重连（✗ 无上限会变死循环）。
	WatchdogAbandoned WatchdogVerdict = "abandoned"
)

// 两把时钟与两级动作的**默认值**（设计 §十二 Q4 的参数表 · 逐条取真源，✗ 不自造）。
const (
	// DefaultHeartbeat 心跳间隔的通行区间是 10–30 s：过密无收益、过疏让取消与续租都不灵。
	DefaultHeartbeat = 30 * time.Second
	// DefaultHeartbeatTimeout ＝ 3× 心跳（「心跳超时取心跳间隔的 2–3×」的通行口径）。
	// ★ 它只判「**彻底静默**」（单元没了），**✗ 不判卡** —— 判卡只看 progress。
	DefaultHeartbeatTimeout = 90 * time.Second
	// DefaultNoProgress 无进展默认档：45 min。`0` ＝ **关**（进度判卡关闭）。
	DefaultNoProgress = 45 * time.Minute
	// DefaultGrace 优雅退出预算（批作业档 120–300 s）。
	DefaultGrace = 30 * time.Second
	// DefaultMaxStaleKills 有界重连上限（✗ 无上限的进度重置会变死循环）。
	DefaultMaxStaleKills = 3
	// DefaultPrefillFactor 前缀预算 ＝ **2× 最坏观测值**。
	DefaultPrefillFactor = 2
)

// WatchdogConfig 看门狗参数（环境变量可覆盖；沙箱压秒测试也用它们）。
type WatchdogConfig struct {
	Enabled bool
	// Window 采样窗口（默认 90 s）—— ★ 它只是**检查节奏**（多久取一次证据），
	// ✗ **不再**是判死的硬上界（旧口径 `Window × MaxWindows` ＝ 450 s 正是 `GAP-20261014-61` 的病根）。
	Window time.Duration
	// MaxWindows ⚠ **已退役的硬上界旋钮**（默认 5）。保留它只为**读取兼容**（env 仍认，
	// 观测面仍出），判卡改由下面两把时钟拿主意。✗ 任何路径都不得再拿它的乘积当硬上界。
	MaxWindows int
	// Sample 两次工时读数间隔（默认 5 s）。
	Sample time.Duration
	// DeadFlatSamples 正文阶段（T14）：连续几次采样"无推进且 CPU 工时也不动"就**立刻判死**
	// （提前判死，不必等窗口用满；默认 2 ⇒ 10 s）。这是"彻底停摆"的证据，与"忙但不推进"区分开。
	DeadFlatSamples int
	// EggMax 单卵时限（`GAP-20260926-201`）：本枚卵从起跑到判死的**最终兜底**值，默认 0 = 不设
	// （与旧口径逐字一致，一键回退）。★ 它是**可配**的（env `ZERG_EGG_MAX_S` / 逐枚 flag），
	// 因此不违反"底下不许有不可配的硬 timeout"。
	//
	// ★ **已降为「最终兜底」**（✗ 不再是「到点就判死」的独立判据 —— 判死只看有没有进度）：
	// 它**并入有效进度预算、只抬不降**（见 `ProgressBudget`）；`0` ＝ 本层不参与。
	EggMax time.Duration

	// ── ★ 两把正交时钟（`GAP-20261014-61` 的落地参数）────────────────────────────────
	//
	// Heartbeat liveness 心跳间隔（默认 30 s）。
	Heartbeat time.Duration
	// HeartbeatTimeout 心跳超时（默认 90 s ＝ 3×）：超过它说明**彻底静默**（单元可能没了）。
	// ✗ 它不参与判卡 —— 判卡只看 progress。
	HeartbeatTimeout time.Duration
	// NoProgress progress 冻结多久判卡（默认 45 min）。★ `0` ＝ **关**（不判卡）。
	NoProgress time.Duration
	// PrefillBudget 显式 prefill 预算（默认 0 ＝ 由 `EstCtxTokens ÷ TokSPerSec × 2` **现算**）。
	PrefillBudget time.Duration
	// EstCtxTokens 现读窗口（token）—— prefill 预算的分子（按现读窗口算，✗ 不写死）。
	EstCtxTokens int
	// TokSPerSec 实测速率（token/s）—— prefill 预算的分母（按实测算，✗ 不写死）。
	TokSPerSec float64
	// Grace 优雅退出预算（默认 30 s；批作业档 120–300 s）—— 判卡第一级到第二级之间的宽限。
	Grace time.Duration
	// MaxStaleKills 重连封顶（默认 3；0 ＝ 取默认值）。
	MaxStaleKills int
	// Waived **豁免留痕**：哪一层的硬窗口被显式豁免、为什么（"豁免要留痕"，J5 三条牙之③）。
	Waived string
}

// watchdogConfigFromEnv 读环境变量。
//
// 开关口径（与 `ZERG_HATCH` 同一先例）：**默认关，显式开** —— `ZERG_WATCHDOG=1/true/yes/on`。
// 为什么默认关（2026-09-15 实测教训）：开着时若证据读不到（测试环境/非 Linux 没有 cgroup）
// 会走 degraded 回落固定超时，把"对不响应引擎"的既有用例拖到 90s/15min；且本机制属行为变化，
// 按"先沙箱验证、再生产 drop-in 显式开"的纪律更稳。关着时行为与旧口径**逐字一致**（一键回退）。
// 窗口/采样用**秒（可小数）**，便于沙箱压秒验证。
//
// ★ 两把时钟的旋钮（`GAP-20261014-61`）：
//
//	ZERG_WATCHDOG_HEARTBEAT_S             liveness 心跳间隔（默认 30）
//	ZERG_WATCHDOG_HEARTBEAT_TIMEOUT_S     心跳超时（默认 90）
//	ZERG_WATCHDOG_NO_PROGRESS_S           progress 冻结判卡档（默认 2700 ＝ 45 min；**0 ＝ 关**）
//	ZERG_WATCHDOG_PREFILL_BUDGET_S        显式 prefill 预算（默认 0 ＝ 现算）
//	ZERG_WATCHDOG_EST_CTX                 现读窗口（token）
//	ZERG_WATCHDOG_TOK_S                   实测速率（token/s）
//	ZERG_WATCHDOG_GRACE_S                 优雅退出预算（默认 30）
//	ZERG_WATCHDOG_MAX_STALE_KILLS         重连封顶（默认 3）
//	ZERG_WATCHDOG_WAIVE                   豁免留痕（自由文本）
func watchdogConfigFromEnv(get func(string) string) WatchdogConfig {
	cfg := WatchdogConfig{
		Enabled: false, Window: 90 * time.Second, MaxWindows: 5, Sample: 5 * time.Second,
		DeadFlatSamples: 2, EggMax: 0,
		Heartbeat: DefaultHeartbeat, HeartbeatTimeout: DefaultHeartbeatTimeout,
		NoProgress: DefaultNoProgress, Grace: DefaultGrace, MaxStaleKills: DefaultMaxStaleKills,
	}
	switch v := strings.ToLower(strings.TrimSpace(get("ZERG_WATCHDOG"))); v {
	case "1", "true", "yes", "on":
		cfg.Enabled = true
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_WINDOW_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.Window = time.Duration(f * float64(time.Second))
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_MAX_WINDOWS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxWindows = n
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_SAMPLE_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.Sample = time.Duration(f * float64(time.Second))
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_DEAD_FLAT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.DeadFlatSamples = n
		}
	}
	if v := strings.TrimSpace(get("ZERG_EGG_MAX_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.EggMax = time.Duration(f * float64(time.Second))
		}
	}
	// ── 两把时钟的旋钮 ──────────────────────────────────────────────────────────
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_HEARTBEAT_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.Heartbeat = time.Duration(f * float64(time.Second))
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_HEARTBEAT_TIMEOUT_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.HeartbeatTimeout = time.Duration(f * float64(time.Second))
		}
	}
	// ★ 这里必须收 `0`（**0 ＝ 关** 是一档真语义，不是"非法值静默忽略"）。
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_NO_PROGRESS_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			cfg.NoProgress = time.Duration(f * float64(time.Second))
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_PREFILL_BUDGET_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			cfg.PrefillBudget = time.Duration(f * float64(time.Second))
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_EST_CTX")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.EstCtxTokens = n
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_TOK_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.TokSPerSec = f
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_GRACE_S")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.Grace = time.Duration(f * float64(time.Second))
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_MAX_STALE_KILLS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.MaxStaleKills = n
		}
	}
	if v := strings.TrimSpace(get("ZERG_WATCHDOG_WAIVE")); v != "" {
		cfg.Waived = v
	}
	return cfg
}

// PrefillBudget —— prefill 预算公式：**`est_ctx ÷ 实测 tok/s × 2`**（`GAP-20261014-61` · 设计 §十二 Q4）。
//
// 「前缀预算 ＝ 2× 最坏观测值」：现成模型的加载档位 10–15 min 只够小模型，
// 132K 档按实测约 96 tok/s 算 ⇒ `132000 ÷ 96 × 2 ≈ 2750 s ≈ 46 min`。
// 任一输入缺失（≤ 0）⇒ 返回 0（＝"算不出来"，由调用方回落 no_progress 档；✗ 不许拿 0 当"预算为零"去杀）。
func PrefillBudget(estCtxTokens int, tokSPerSec float64) time.Duration {
	if estCtxTokens <= 0 || tokSPerSec <= 0 {
		return 0
	}
	sec := float64(estCtxTokens) / tokSPerSec * float64(DefaultPrefillFactor)
	return time.Duration(sec * float64(time.Second))
}

// progressBudgetBase 进度预算的**基准档**（✗ 不含单卵时限兜底 —— 兜底在 `ProgressBudget` 里并入）。
//
// 有效预算 = **`max(no_progress, prefill 预算)`**，并给出**为什么是这个值**（进判词/观测面）。
//
// 返回 `(0, 说明)` ＝ **进度判卡关闭**（不是"预算为零"）：
// 此时只保留「彻底停摆」那条早杀（liveness 与 progress 双双冻结），其余一律不判 ——
// 这就是 `no_progress: 0 ＝ 关` 那一档的真语义（✗ 不许把它读成"零秒就杀"）。
func (c WatchdogConfig) progressBudgetBase() (time.Duration, string) {
	np := c.NoProgress
	pb := c.PrefillBudget
	whyPB := ""
	if pb <= 0 {
		pb = PrefillBudget(c.EstCtxTokens, c.TokSPerSec)
		if pb > 0 {
			whyPB = fmt.Sprintf("（现算：est_ctx=%d ÷ %.2f tok/s × %d = %s）",
				c.EstCtxTokens, c.TokSPerSec, DefaultPrefillFactor, pb.Round(time.Second))
		}
	} else {
		whyPB = "（显式给的预填预算）"
	}
	switch {
	case np == 0 && pb == 0:
		return 0, "进度判卡**关闭**：no_progress=0（关）且无 prefill 预算（缺 est_ctx / tok/s）⇒ 只保留『彻底停摆』早杀"
	case np == 0:
		return pb, "no_progress=0（关）⇒ 取 prefill 预算 " + pb.Round(time.Second).String() + whyPB
	case pb == 0:
		return np, "no_progress=" + np.Round(time.Second).String() + "（无现读窗口/速率 ⇒ 无 prefill 预算）"
	case pb > np:
		return pb, "取宽的那把：prefill 预算 " + pb.Round(time.Second).String() + whyPB +
			" > no_progress " + np.Round(time.Second).String()
	default:
		return np, "取宽的那把：no_progress " + np.Round(time.Second).String() +
			" ≥ prefill 预算 " + pb.Round(time.Second).String() + whyPB
	}
}

// ProgressBudget 有效进度预算 ＝ **`max(no_progress, prefill 预算, 单卵时限)`**（进判词/观测面）。
//
// ★ **单卵时限（`EggMax` · CLI `--timeout` / env `ZERG_EGG_MAX_S`）已降为「最终兜底」**
//
//	（用户口径逐字：「DS4 虽然慢，只要他在输出，就应该没问题」）——
//	它**不再是「到点就判死」的独立判据**，而是**并入有效预算、只抬不降**（取 max）。
//
//	为什么撤掉独立的墙钟判死、而不是把它改成「一个更大的数」：
//	「耐心 ≥ 引擎实测档」的墙钟判死被「progress 冻结 ≥ 预算」这一条**严格涵盖**
//	（一次请求里 progress 冻结时长必 ≤ 墙钟时长）⇒ 留着它只能在**预算之前**开火，
//	而那正是「按时间杀**在输出的**慢卵」这条病根（现场：`--timeout 5m` 把一枚
//	6.27 t/s 正在出 token 的 DS4 卵每 6m40s 杀一次）。⇒ 判死只留一把尺：**有没有进度**。
//
//	★ `0` ＝ 三层（no_progress / prefill 预算 / 单卵时限）**都没给** ⇒ 进度判卡关闭。
func (c WatchdogConfig) ProgressBudget() (time.Duration, string) {
	budget, why := c.progressBudgetBase()
	if c.EggMax > budget {
		budget = c.EggMax
		why = "单卵时限兜底 " + c.EggMax.Round(time.Second).String() +
			"（已并入有效预算 · 只抬不降 ∵ 单卵时限降为最终兜底）；原基准档：(" + why + ")"
	}
	return budget, why
}

// ── 单卵时限全通路（GAP-20260926-201）：CLI 旗标 ⇒ 下发体 ⇒ 主控透传 ⇒ 子端逐请求 ──────

// ParseEggMaxS 把请求体里的 `egg_max_s` 读成 time.Duration。
//
// 口径与 env `ZERG_EGG_MAX_S` 同形（**秒 · 可小数**）：空串 ⇒ (0, nil)「未设」（回落 env 兜底）；
// 读不出 / ≤ 0 ⇒ **报错**（调用方据此**明报拒收**，✗ **不静默回落**默认 —— 与本文件 env 侧
// 「非法值静默忽略」刻意不同：这里是**用户逐次显式给的**，静默回落等于骗人）。
func ParseEggMaxS(s string) (time.Duration, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return 0, fmt.Errorf("egg_max_s 读不出来：%q", s)
	}
	if f <= 0 {
		return 0, fmt.Errorf("egg_max_s 必须为正：%q", s)
	}
	return time.Duration(f * float64(time.Second)), nil
}

// resolveEggMax 求「这一枚卵」的有效单卵时限。
//
// 优先级（`GAP-20260926-201` 口径 · 逐字照账）：**逐枚覆盖（flag）> env `ZERG_EGG_MAX_S` > 现默认 0**。
// `override > 0` 才算「给了」（与 env 同一形态）；给了就同时把看门狗打开 —— 否则
// `EggMax` 在 `Enabled=false` 时根本无牙（`Wait` 走纯固定超时那条路），设了等于没设。
func resolveEggMax(cfg WatchdogConfig, override time.Duration) WatchdogConfig {
	if override > 0 {
		cfg.EggMax = override
		cfg.Enabled = true
	}
	return cfg
}

// WatchdogObservation 观测面（设计 §6 字段名逐字 ＋ 微句 §九.2 的两把锁）。
//
// ★ **两把锁都出**（判卡只看 progress，但报告里两把都要能看见 —— J4 逐字）：
// `last_heartbeat_at`（liveness）与 `last_progress_at`（progress）。
type WatchdogObservation struct {
	WindowIdx      int             `json:"window_idx"`
	Extensions     int             `json:"extensions"`
	LastOutputAgoS float64         `json:"last_output_ago_s"`
	CPUUsecDelta   uint64          `json:"cpu_usage_usec_delta"`
	Verdict        WatchdogVerdict `json:"verdict"`
	Reason         string          `json:"reason"`

	// ── ★ 两把正交时钟（A2 · `GAP-20261014-61`）──────────────────────────────────
	//
	// LastHeartbeatAt liveness 时钟：**任何活动都续**（含流式 token / SSE 保活 / CPU 工时在涨）。
	LastHeartbeatAt string `json:"last_heartbeat_at"`
	// LastProgressAt progress 时钟：**只由真实 chunk 计数续**（✗ 时间戳不算）。
	// ★ 判卡只看它 —— 它就是"该不该判卡"的那把尺。
	LastProgressAt string `json:"last_progress_at"`
	// HeartbeatAgoS / ProgressAgoS 距上一次续钟的秒数（两把锁各自的"多久没动"）。
	HeartbeatAgoS float64 `json:"heartbeat_ago_s"`
	ProgressAgoS  float64 `json:"progress_ago_s"`
	// ProgressChunks 累计真实 chunk 计数（单调不减；它是**计数**，不是时间戳）。
	ProgressChunks uint64 `json:"progress_chunks"`
	// ProgressBudgetS 本次采用的有效进度预算（秒）。`0` ＝ **判卡关闭**。
	ProgressBudgetS float64 `json:"progress_budget_s"`
	// BudgetWhy 预算取值的理由（取宽的那把 / 关 / 无 prefill 预算）。
	BudgetWhy string `json:"budget_why"`
	// StaleKills 已用掉的判卡（重连）次数 —— 封顶 `max_stale_kills`。
	StaleKills int `json:"stale_kills"`
	// Stage 判卡级数：0 未判卡 / 1 第一级（请求优雅退）/ 2 第二级（硬断）/ 3 封顶放弃。
	Stage int `json:"stage"`
	// Action 两级动作的判词（`stalled_grace` / `stalled_hard` / `abandoned`；空 ＝ 未动作）。
	Action WatchdogVerdict `json:"action"`
	// ActionReason 两级动作的理由（**与判卡理由分开写**：`Reason` 永远留判卡那一句，
	// 不许被动作的理由盖掉 —— 否则复盘时看不出"当初是为什么判卡的"）。
	ActionReason string `json:"action_reason,omitempty"`
	// Waived 豁免留痕（哪一层硬窗口被显式豁免、为什么）。
	Waived string `json:"waived,omitempty"`
}

// watchdogDeps 注入式依赖：单测不真 sleep、不真读盘（时间与读数都可控）。
type watchdogDeps struct {
	now      func() time.Time
	after    func(time.Duration) <-chan time.Time
	sleep    func(time.Duration)
	cpuUsec  func() (uint64, error)
	slotBusy func() (bool, string) // 可 nil（引擎不提供 /slots ⇒ 只靠工时判据）
	// progress ★ 真实 chunk 计数源（引擎侧已处理 token 计数差分）。nil ⇒ 没有进度读数 ⇒ **不判卡**。
	progress func() (uint64, error)
}

func realWatchdogDeps(cpuUsec func() (uint64, error), slotBusy func() (bool, string), progress func() (uint64, error)) watchdogDeps {
	return watchdogDeps{now: time.Now, after: time.After, sleep: time.Sleep, cpuUsec: cpuUsec, slotBusy: slotBusy, progress: progress}
}

// Watchdog 单次请求的活动看门狗。零值不可用，用 newWatchdog 构造。
type Watchdog struct {
	cfg  WatchdogConfig
	deps watchdogDeps
	obs  WatchdogObservation

	lastOutput time.Time
	// ★ 两把锁（正交）：liveness 与 progress 各自记「最后一次续钟的时刻」。
	lastHeartbeat time.Time
	lastProgress  time.Time
	// 真实 chunk 计数的**高水位**：只有单调增量才算续钟（✗ 时间戳、✗ 回退值）。
	progressChunks uint64
}

func newWatchdog(cfg WatchdogConfig, deps watchdogDeps) *Watchdog {
	return &Watchdog{cfg: cfg, deps: deps}
}

// newRealWatchdog 生产构造：工时读数（liveness）与进度读数（progress · 真实 chunk 计数）都由调用方给出。
func newRealWatchdog(cfg WatchdogConfig, cpuUsec func() (uint64, error), slotBusy func() (bool, string), progress func() (uint64, error)) *Watchdog {
	return newWatchdog(cfg, realWatchdogDeps(cpuUsec, slotBusy, progress))
}

// Observation 取观测面（判死后由调用方写进错误响应/事件帧）。
func (w *Watchdog) Observation() WatchdogObservation { return w.obs }

// MarkOutput 记录"有输出"（响应头/分块到达）。一旦有输出，看门狗即刻退场。
//
// ★ 有输出既是 liveness 也是 progress 的证据（它是一段**真实交付**的 chunk），
// 因此它同时续两把钟 —— 与 `MarkHeartbeat`（只续 liveness）刻意分开。
func (w *Watchdog) MarkOutput() {
	t := w.deps.now()
	w.lastOutput = t
	w.lastHeartbeat = t
	w.lastProgress = t
}

// MarkHeartbeat **liveness 续钟**：任何活动都算（流式 token / SSE 保活 / 引擎心跳 / CPU 工时在涨）。
//
// ★ 它**绝不**续 progress（两把时钟正交 · 设计 §七.3 J4 逐字：「推理只续 liveness、绝不续 progress」）。
// 想续 progress 只有一条路：`MarkProgressChunks`。
func (w *Watchdog) MarkHeartbeat() { w.lastHeartbeat = w.deps.now() }

// MarkProgressChunks **progress 续钟**：只由**真实 chunk 计数**续，且必须**单调递增**。
//
// 返回 true ＝ 本次真的续了钟（计数有增量）；false ＝ 计数没动（或回退）⇒ **不算推进**。
//
// ★ ✗ 本函数**不接受时间戳**（签名里就没有时间这回事）—— 这正是 J4 那条牙：
// 「✗ 不许用时间戳做进度重置（重试尝试开头会重置它 ⇒ 计数被永久清零、看门狗永不收口）」。
func (w *Watchdog) MarkProgressChunks(chunks uint64) bool {
	if chunks <= w.progressChunks {
		return false
	}
	w.progressChunks = chunks
	w.lastProgress = w.deps.now()
	return true
}

// LastHeartbeatAt / LastProgressAt 两把锁（供回执的 ⑧ 时刻五件直接取）。
func (w *Watchdog) LastHeartbeatAt() time.Time { return w.lastHeartbeat }

func (w *Watchdog) LastProgressAt() time.Time { return w.lastProgress }

// SetStaleKills 把**跨请求累积**的重连计数灌进来（同一枚卵上的历次判卡一起封顶 —— J5「有界」）。
func (w *Watchdog) SetStaleKills(n int) { w.obs.StaleKills = n }

// maxStaleKills 有效重连封顶（0 ⇒ 取默认）。
func (w *Watchdog) maxStaleKills() int {
	if w.cfg.MaxStaleKills <= 0 {
		return DefaultMaxStaleKills
	}
	return w.cfg.MaxStaleKills
}

// graceOf 有效优雅退出预算。
func (w *Watchdog) graceOf() time.Duration {
	if w.cfg.Grace > 0 {
		return w.cfg.Grace
	}
	return DefaultGrace
}

// markOK 正常收尾（头到了 / 正文读完）：两把锁都在观测面里出，便于事后分辨"是算了很久"还是"真卡过"。
func (w *Watchdog) markOK() (WatchdogVerdict, string) {
	w.obs.Verdict = WatchdogOK
	w.obs.Reason = ""
	w.obs.LastOutputAgoS = w.deps.now().Sub(w.lastOutput).Seconds()
	w.fillClockObs()
	return WatchdogOK, ""
}

// fillClockObs 把两把锁写进观测面（判卡只看 progress，但**两把都出**）。
func (w *Watchdog) fillClockObs() {
	if !w.lastHeartbeat.IsZero() {
		w.obs.LastHeartbeatAt = w.lastHeartbeat.Format(time.RFC3339Nano)
		w.obs.HeartbeatAgoS = w.deps.now().Sub(w.lastHeartbeat).Seconds()
	}
	if !w.lastProgress.IsZero() {
		w.obs.LastProgressAt = w.lastProgress.Format(time.RFC3339Nano)
		w.obs.ProgressAgoS = w.deps.now().Sub(w.lastProgress).Seconds()
	}
	w.obs.ProgressChunks = w.progressChunks
}

// progressValue 取一次真实 chunk 计数读数。
func (w *Watchdog) progressValue() (uint64, error) {
	if w.deps.progress == nil {
		return 0, fmt.Errorf("未接进度源（nil）⇒ 本请求不判卡")
	}
	return w.deps.progress()
}

// Wait 等「响应头到达」（done 关闭即正常）。返回：
//
//	ok            —— 头到了（正常路径）
//	stuck_*       —— 判卡（调用方应走 `EscalateStall` 的两级动作：先优雅退、grace 后硬断）
//	degraded      —— 读不到证据（调用方回落固定超时，行为与旧口径一致）
//
// ★ 判卡口径（`GAP-20261014-61` 换形）：**只看 progress 冻结是否超过有效预算**；
// liveness（CPU 工时在涨）再多也**不救** progress。
// 另保留一条"真停摆"早杀：liveness 与 progress **双双冻结**（连续 DeadFlatSamples 次）⇒ 提前判死。
func (w *Watchdog) Wait(done <-chan struct{}) (WatchdogVerdict, string) {
	t0 := w.deps.now()
	w.lastOutput, w.lastHeartbeat, w.lastProgress = t0, t0, t0

	if !w.cfg.Enabled {
		<-done
		w.obs.Verdict = WatchdogOK
		w.obs.Reason = "看门狗未启用（ZERG_WATCHDOG=0）⇒ 纯固定超时路径"
		return WatchdogOK, ""
	}

	budget, budgetWhy := w.cfg.ProgressBudget()
	grace := w.graceOf()
	w.obs.ProgressBudgetS = budget.Seconds()
	w.obs.BudgetWhy = budgetWhy
	w.obs.Waived = w.cfg.Waived
	w.obs.Stage = 0

	window := w.cfg.Window
	if window <= 0 {
		window = 90 * time.Second
	}
	sample := w.cfg.Sample
	if sample <= 0 {
		sample = 5 * time.Second
	}
	deadFlat := w.cfg.DeadFlatSamples
	if deadFlat <= 0 {
		deadFlat = 2
	}

	flatBoth := 0
	for windowIdx := 0; ; windowIdx++ {
		// ★ 这里**曾经**有一条「单卵时限到 ⇒ 判死」的墙钟分支（`GAP-20260926-201`）。它已按用户口径
		// **撤除**（「DS4 虽然慢，只要他在输出，就应该没问题」）：判死**只看有没有进度**（下面 ④ 那条），
		// 单卵时限降为**最终兜底**并入有效预算（`ProgressBudget` ⇒ 只抬耐心、不再单独判死）。
		// 为什么撤除、而不是留成「一个更大的墙钟」：见 `ProgressBudget` 的函数头注（严格涵盖的证明）。
		//
		// 现场病根（撤除它要治的那件事）：EDGE-NODE 的 `zerg-agentd` 日志——
		//   `[backend] 看门狗判死: model=DeepSeek-V4-Flash-Vision-Exp verdict=stuck_no_progress
		//    unit=zerg-deepseek-v4-flash-vision-exp 理由=单卵时限到（约 5m0s）⇒ 判卡住（置 crashed）`
		// 而同刻引擎日志里它在**持续输出**（`gen=200 … decoding 6.27 t/s` → `gen=250 … 0.47 t/s`）
		// ⇒ 墙钟分支把**活的慢卵**当了死的，约每 6m40s kill+重启一次。
		// **输出优先**（先做一次非阻塞检查）：头若已经回来，就不再花这个窗口取证据 ——
		// 否则 select 在两个就绪通道间随机选，可能"抢在 done 前"把已经正常的请求判死。
		select {
		case <-done:
			return w.markOK()
		default:
		}

		select {
		case <-done:
			return w.markOK()
		case <-w.deps.after(window):
		}

		// 卡点到（不是死刑）：取"有没有在花力气"（liveness）与"有没有真推进"（progress）两把证据。
		d1, err := w.deps.cpuUsec()
		if err != nil {
			w.obs.Verdict = WatchdogDegraded
			w.obs.Reason = fmt.Sprintf("读不到本单元 CPU 工时（%v）⇒ 降级为固定超时（不误杀）", err)
			return WatchdogDegraded, w.obs.Reason
		}
		w.deps.sleep(sample)
		d2, err := w.deps.cpuUsec()
		if err != nil {
			w.obs.Verdict = WatchdogDegraded
			w.obs.Reason = fmt.Sprintf("第二次读工时失败（%v）⇒ 降级为固定超时（不误杀）", err)
			return WatchdogDegraded, w.obs.Reason
		}

		w.obs.WindowIdx = windowIdx + 1
		w.obs.LastOutputAgoS = w.deps.now().Sub(w.lastOutput).Seconds()

		// ── ① liveness：CPU 工时在涨 ⇒ 续 liveness（**绝不**续 progress）──────────────
		heartbeat := d2 > d1
		if heartbeat {
			w.obs.CPUUsecDelta = d2 - d1
			w.obs.Extensions++
			w.MarkHeartbeat()
		} else {
			w.obs.CPUUsecDelta = 0
		}

		// ── ② progress：只认「真实 chunk 计数」的单调增量（✗ 时间戳不算）──────────────
		chunks, perr := w.progressValue()
		progressReadable := perr == nil
		progressAdvanced := false
		if progressReadable {
			progressAdvanced = w.MarkProgressChunks(chunks)
		}
		w.fillClockObs()

		// ── ③ 真停摆（提前判死）：两把钟**双双冻结** ⇒ 这是"彻底不动"，不是"预算不够" ──
		if !heartbeat && !progressAdvanced {
			flatBoth++
			if flatBoth >= deadFlat {
				w.obs.Stage = 2
				w.obs.Verdict = WatchdogStuckCPUStalled
				w.obs.Reason = fmt.Sprintf(
					"连续 %d 次采样：本单元 CPU 工时不动 且 无真实 chunk 增量（liveness 与 progress 双双冻结）⇒ 判卡住（提前，不必等预算）",
					flatBoth)
				return WatchdogStuckCPUStalled, w.obs.Reason
			}
		} else {
			flatBoth = 0
		}

		// ── ④ 判卡**只看 progress**：冻结超过有效预算 ⇒ 判卡（第一级动作由调用方走 EscalateStall）──
		//
		// ⚠ 两条不许动的口径：
		//   · `budget == 0` ⇒ **判卡关闭**（`no_progress: 0 ＝ 关` 那一档；✗ 别读成"零秒就杀"）；
		//   · **进度读数不可读 ⇒ 绝不按预算判卡**（读不到 ≠ 没推进；否则一次读数失败就把合法的长 prefill 杀掉）。
		if budget > 0 && progressReadable {
			el := w.deps.now().Sub(w.lastProgress)
			if el >= budget {
				w.obs.Stage = 1
				w.obs.Verdict = WatchdogStuckNoProgress
				w.obs.Reason = fmt.Sprintf(
					"progress 冻结已 %s ≥ 预算 %s ⇒ 判卡（第一级：请求优雅退，grace=%s）；预算依据：%s",
					el.Round(time.Second), budget.Round(time.Second), grace.Round(time.Second), budgetWhy)
				return WatchdogStuckNoProgress, w.obs.Reason
			}
		}
	}
}

// StallPlan 判卡后的**两级**落地（J5）：先请求优雅退，grace 到期才硬断。
//
// ★ 「有界」：每次调用计一次 stale kill，超过 `max_stale_kills` 立刻 `abandoned`，
// ✗ 绝不无上限地重置/重连（那会变死循环）。
type StallPlan struct {
	// Graceful 第一级：请求**优雅退出**（SIGTERM 语义 —— 让引擎自己收尾；nil ⇒ 不调）。
	Graceful func()
	// Hard 第二级：**硬断**（SIGKILL 语义）—— **只在 grace 到期后**调。
	Hard func()
	// Grace 优雅退出预算（0 ⇒ 取 cfg.Grace ⇒ 取默认 30 s；批作业档 120–300 s）。
	Grace time.Duration
	// Exited 它自己退了的信号（可 nil ⇒ 只能靠 grace 计时）。grace 内关掉 ⇒ **不硬断**。
	Exited <-chan struct{}
}

// EscalateStall 判卡后的两级动作。返回判词：
//
//	stalled_grace —— 第一级（请求优雅退）已发出，且它在 grace 内自己退了（**不硬断**）
//	stalled_hard  —— grace 到期仍无进展 ⇒ 第二级（硬断）已发
//	abandoned     —— 重连封顶（`max_stale_kills`）用尽 ⇒ 放弃（✗ 不硬断也不重置）
func (w *Watchdog) EscalateStall(plan StallPlan) (WatchdogVerdict, string) {
	grace := plan.Grace
	if grace <= 0 {
		grace = w.graceOf()
	}
	w.obs.StaleKills++
	if w.obs.StaleKills > w.maxStaleKills() {
		w.obs.Stage = 3
		w.obs.Action = WatchdogAbandoned
		w.obs.ActionReason = fmt.Sprintf(
			"重连封顶用尽（max_stale_kills=%d · 已 %d 次）⇒ abandoned：不再重置/重连（✗ 无上限的进度重置会变死循环）",
			w.maxStaleKills(), w.obs.StaleKills)
		return WatchdogAbandoned, w.obs.ActionReason
	}

	// 第一级：请求优雅退出。
	w.obs.Stage = 1
	w.obs.Action = WatchdogStalledGrace
	if plan.Graceful != nil {
		plan.Graceful()
	}
	if plan.Exited != nil {
		// **它自己退了优先**（先做一次非阻塞检查）：grace 到点与"退出信号"同时就绪时，
		// 不许在 select 里随机选到 after 那一支 —— 那会把"grace 内已优雅退"误报成硬断。
		select {
		case <-plan.Exited:
			w.obs.ActionReason = fmt.Sprintf("判卡后 grace（%s）内自己退出 ⇒ 不硬断（第一级够了）", grace.Round(time.Second))
			return WatchdogStalledGrace, w.obs.ActionReason
		default:
		}
		select {
		case <-plan.Exited:
			w.obs.ActionReason = fmt.Sprintf("判卡后 grace（%s）内自己退出 ⇒ 不硬断（第一级够了）", grace.Round(time.Second))
			return WatchdogStalledGrace, w.obs.ActionReason
		case <-w.deps.after(grace):
		}
	} else {
		<-w.deps.after(grace)
	}

	// 第二级：硬断。
	w.obs.Stage = 2
	w.obs.Action = WatchdogStalledHard
	if plan.Hard != nil {
		plan.Hard()
	}
	w.obs.ActionReason = fmt.Sprintf("grace（%s）到期仍无进展 ⇒ 硬断（SIGKILL 语义；第二级）", grace.Round(time.Second))
	return WatchdogStalledHard, w.obs.ActionReason
}

// ── 小工具：环境变量读取（与既有代码同一风格）────────────────────────────────

func watchdogEnv(key string) string { return os.Getenv(key) }
