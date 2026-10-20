// family_agent_run.go —— `zerg agent run`（任务运行器 · **归入命令面** · 批 B5）。
//
// 源件 = 独立入口 `core/cmd/zerg-agent/main.go`（462 行 · Go `flag` 十六枚 · 缺省
// `-workdir /tmp/zerg-agent` / `-model example-35b`）。本件是那一件的**逐面搬**：
// 旗标 → 令牌闸 → 接单/任务两档 → 造 Agent（M3 工具门）→ 建日志与会话日志 → 状态
// （断连恢复）→ MCP/技能 → 工具集 → `agent.Loop`（含重试）→ 失败挂单 → 收尾 → 结果面。
//
// 形态（旗标**逐字**保留源件 · 只把 Go `flag` 的单横线写成命令面的双横线）：
//
//	zerg agent run (--task <任务描述> | --issue <问题单路径>) [--model <名>]
//	               [--workdir <目录>] [--max-turns <N>] [--tools <子集>] [--mcp <spec>]
//	               [--auto-issue] [--subtask] [--retry <N>] [--state <件>]
//	               [--gateway <URL>] [--x3-status <URL>] [--x3-token <令牌>] [--json <字段>]
//
//	--task <任务描述>    任务正文（必填其一）
//	--issue <件>         接单档：读问题单 → 查因 → 回填结论 → 标 resolved/escalated（必填其一）
//	--model <名>         模型名（源件缺省 `example-35b`）
//	--workdir <目录>     工作区（源件缺省 `/tmp/zerg-agent`；日志落 `<workdir>/.zerg/logs/<任务 id>`）
//	--max-turns <N>      最大循环轮数（缺省 = 唯一真源 `agent.DefaultMaxTurns`）
//	--tools <子集>       工具子集（逗号分隔 · MCP 合并后统一过滤）
//	--mcp <spec>         MCP 服务器（`name:cmd|arg…` 或 HTTP 型 `name:http|url|token`）
//	--auto-issue         重试耗尽仍失败 ⇒ 写 `docs/issues/`（错误自愈 P0）
//	--subtask            子任务结晶模式（拆解轮 + 阶段执行 + 结晶链）
//	--retry <N>          失败自动重试次数（源件缺省 1）
//	--state <件>         断连恢复：存在则续跑（跑完回写）
//	--gateway <URL>      网关地址（缺省 = `statepath.GatewayBaseURL()`）
//	--x3-status <URL>    机器级采样地址（只在环境变量未设时写 `ZERG_X3_STATUS`）
//	--x3-token <令牌>    机器级采样令牌（只在环境变量未设时写 `ZERG_X3_TOKEN`）
//	--json <字段>        机器面字段表（`ok,status,turns,tokens,summary,workdir,logs,exit_code,duration_ms`）
//
// 为什么动作名是 `run`：`agent` 族**已经在册**（`ls`/`show`/`ping`/`deploy`/`load`/`unload`/…
// 全是**打子端/主控**的投影与控制面）。本件搬的是**另一件事** —— 在**本机**跑一个 Agent 任务
// （源件 `zerg-agent` 的语义：调网关、执行工具、写自己的任务单）。两者共用族名、动作名分家：
// `run` = 跑任务（本件）· 其余动作 = 管子端。★ 与 `sched run`／`api serve` 同一条判法
// （族已在册 ⇒ 只增一格，✗ 不新开顶层族）。
//
// 退码（真源 = `exitcodes.go` 的表；本文件不写死数字）：
//
//	0   任务完成（`ReasonComplete`）
//	1   跑到了、没成功（`model_error` / `max_turns` / `blocked` / `token_budget` / 其它终止原因）
//	    · 问题单读不到 · 日志目录建不出来 · 状态文件读不出但显式给了 `--state`
//	2   用法错（`--task` 与 `--issue` 都没给 / 两枚同给 / 值旗标裸给 / 整数不合法 /
//	    多余位置参数 / `--json` 不给字段或字段不在册 / 未知旗标）
//	4   缺令牌（`unauthenticated` —— 命令面 §九 M2 `C7`；源件是 `log.Fatalf` ⇒ 1）
//	130 人打断（ctx 被取消 —— 源件同况照常打结果、退 0）
//
// 与源件的**照实差异**（十二处 · 逐条写清，别处不猜）：
//
//	① 用法类退码 **1 → 2**：源件的缺参/裸值一律 `os.Exit(1)`；命令面统一走 `exitcodes.go`
//	   的 usage 档（同批 B1 `trace` / B2 `sched run` / B3 `review` / B4 `api serve` 的同一条归一）。
//	② **缺令牌 1 → 4，且在碰问题单之前判**：源件先 `MarkIssueStatus(…, "fixing")` 再
//	   `mustToken()` 的 `log.Fatalf` ⇒ 问题单被留在 `fixing` 而任务根本没跑。本件把令牌闸
//	   提到 `agentRunParse`（执行前）⇒ 缺令牌时**一个字节都不写**。
//	③ **进程内语义码 3/4 重新取号**：源件 `3 = 模型错误`、`4 = 可重试`；命令面主表里
//	   `3 = partial_ok`、`4 = unauthenticated` —— 同数字两义是门⑨的硬红线（`P-013` ⑤ 未拍前
//	   不许并码）。⇒ 非完成一律 `1`（`failed`），**终止原因进 `status` 字段与 stderr**。
//	   ★ 主控侧的调度器仍 `exec zerg-agent -json -issue <件>`（`core/internal/agent/scheduler.go:477`
//	   靠 0/2/3/4 判重试/死信）⇒ **双轨期一个字都不许动它**；本命令不是它的替身。
//	④ **`--json` 语义归一**：源件 `-json` 是布尔信封开关；命令面 `--json <字段>` 是**字段表**
//	   （§4.1 `K2` 甲档：裸给 ⇒ 2 并列出可选字段）。信封换成命令面标准包封（`emitSelected`
//	   那条路 · `items` 恒数组 · I3），字段见上。
//	⑤ **stdout / stderr 分工**（§4.1 `K4`）：人面进度（`✅` / `🚀` / `📋` / `♻️` / `🔄` / `⚠️`）
//	   一律进 **stderr**；结果面（人面摘要 / `--json` 包封）留 **stdout**。源件全打 stdout。
//	⑥ **`--task` 与 `--issue` 同给 ⇒ 2**：源件 task 赢、却在收尾按 issue 改状态（静默吞一枚）。
//	   命令面按 §4.1 `K14` 不许静默吞 ⇒ 明拒。
//	⑦ **写盘错误不再静默**：问题单标 `fixing` / 回填状态两处写失败 ⇒ stderr 判词 + 退 1
//	   （源件 `_ = os.WriteFile` 丢掉返回值）；`audit.jsonl` 写失败 ⇒ stderr 一行（不进退码 ——
//	   它是旁证不是任务产物）。会话日志装不上仍照源件「无日志继续」，但**出声**。
//	⑧ **值旗标裸给一律 2**：源件 `-model ""` / `-gateway ""` 静默收下、跑起来才炸（`GAP-20260928-151`
//	   同族纪律：值旗标裸给不许静默吞）。
//	⑨ **打断 ⇒ 130**：源件收到 Ctrl-C 后 `cancel()` 让 loop 收尾、照常打结果并退 0；命令面按
//	   退码表 `interrupted`（且 `main()` 的全局处理器本来就是 130 ⇒ 两者同码）。
//	⑩ **`-version` 不搬**：源件那枚只印 `version.Line("zerg-agent")`；命令面 `zerg version`
//	   与全局 `--version` 是**唯一真源**（同族纪律：不造第二份身份面）。
//	⑪ 重试实例的 gate 装载**不重复打告警**：源件第一实例打、重试实例静默接「恒拦」门 ——
//	   本件两处同一判词（行为零差：两处都 fail-closed 接 `NewAgentGate(nil)`）。
//	⑫ **摘要截断按字符边界**：源件 `summary[:200]` 按**字节**切 ⇒ 中文被切在字符中间、stdout
//	   打出半枚非法 UTF-8（实测 `…啊啊\xe5\x95` 那一形态）。本件收到 ≤200 字节且不破字符
//	   （ASCII 正文与源件逐字同；尾仍加 `...`）。
//
// ★ 双轨期：`core/cmd/zerg-agent` 二进制**一字不动**（本件只搬逻辑、不删入口）。
// ★ 模型腿（真调网关走完 loop）**不在判据件里跑**（要真网关 + 真模型）：判据面钉的是
//
//	登记面 / 旗标解析面 / 接单档的文本与写回 / 退码映射 / 包封字段 / 零副作用六件；
//	真跑那一趟由双轨二进制与既有 `agent` 包判据覆盖。
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/agentstate"
	"github.com/Mr2109/zerg-swarm/core/internal/config"
	"github.com/Mr2109/zerg-swarm/core/internal/control"
	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// agentRunUsageLine —— 用法串（与命令树 `usage` 字段**逐字同源**：一处写错两处就分家）。
const agentRunUsageLine = "zerg agent run (--task <任务描述> | --issue <问题单>) [--model <名>] [--workdir <目录>] [--max-turns <N>] [--tools <子集>] [--mcp <spec>] [--auto-issue] [--subtask] [--retry <N>] [--state <件>] [--gateway <URL>] [--x3-status <URL>] [--x3-token <令牌>] [--json <字段>]"

// agentRunDefaultModel —— 源件 `-model` 的缺省值（逐字）。
const agentRunDefaultModel = "example-35b"

// agentRunDefaultWorkDir —— 源件 `-workdir` 的缺省值（逐字 · 日志落它下面）。
const agentRunDefaultWorkDir = "/tmp/zerg-agent"

// agentRunDefaultRetry —— 源件 `-retry` 的缺省值（逐字）。
const agentRunDefaultRetry = 1

// agentRunFields —— `--json` 可取字段（信封里 `items[0]` 的那一格）。
var agentRunFields = []string{
	"ok", "status", "turns", "tokens", "summary", "workdir", "logs", "exit_code", "duration_ms",
}

// agentRunCfg —— 解析后的启动配置（`workDir` 已是绝对路径；`task` 在接单档由 issue 内容合成）。
type agentRunCfg struct {
	task      string
	issue     string
	model     string
	workDir   string
	maxTurns  int
	tools     string
	mcp       string
	autoIssue bool
	subtask   bool
	retry     int
	stateFile string
	gateway   string
	x3Status  string
	x3Token   string
	token     string // 共享令牌现读值（空 = 没配 ⇒ 退 4，见差异 ②）
}

// agentRunParse —— 旗标 → 配置（语义校验只在本处 · 执行前判）。
//
// 退码：`0` 配置齐；`2` 用法错（差异 ①⑥⑧）；`4` 缺令牌（差异 ②）；`1` 工作目录解析不出来。
func agentRunParse(inv *invocation, stderr io.Writer) (agentRunCfg, int) {
	cfg := agentRunCfg{
		model:    agentRunDefaultModel,
		maxTurns: agent.DefaultMaxTurns,
		retry:    agentRunDefaultRetry,
		gateway:  statepath.GatewayBaseURL(),
	}
	fail := func(code int, kind, detail, msg string) (agentRunCfg, int) {
		inv.setErr(kind, detail, msg)
		return cfg, code
	}

	// ---- 两档互斥：`--task` / `--issue` 恰好给一枚（源件只判「都没给」，两枚同给时静默吞）----
	cfg.task = strings.TrimSpace(inv.flagVal("--task"))
	cfg.issue = strings.TrimSpace(inv.flagVal("--issue"))
	if inv.hasFlag("--task") && cfg.task == "" {
		fmt.Fprintf(stderr, "%s: `--task` 要一段**任务描述**（裸给/空值 ⇒ 不执行）\n用法: %s\n",
			progName, agentRunUsageLine)
		return fail(exitUsage, "usage", "task_empty", "`--task` 裸给或空值")
	}
	if inv.hasFlag("--issue") && cfg.issue == "" {
		fmt.Fprintf(stderr, "%s: `--issue` 要一个**问题单路径**（裸给/空值 ⇒ 不执行）\n用法: %s\n",
			progName, agentRunUsageLine)
		return fail(exitUsage, "usage", "issue_empty", "`--issue` 裸给或空值")
	}
	if cfg.task == "" && cfg.issue == "" {
		fmt.Fprintf(stderr, "%s: `--task` 或 `--issue` 必填其一（两枚都没有 ⇒ 不知道要跑什么 · 不执行）\n用法: %s\n",
			progName, agentRunUsageLine)
		return fail(exitUsage, "usage", "task_or_issue_required", "`--task` 与 `--issue` 都没给")
	}
	if cfg.task != "" && cfg.issue != "" {
		// 差距异 ⑥：源件 task 赢、收尾仍按 issue 改状态 ⇒ 静默吞一枚。
		fmt.Fprintf(stderr, "%s: `--task` 与 `--issue` 只能给一枚（两枚同给 ⇒ 不知道该跑任务还是接单 · 不执行）\n用法: %s\n",
			progName, agentRunUsageLine)
		return fail(exitUsage, "usage", "task_and_issue_conflict", "`--task` 与 `--issue` 同给")
	}

	// ---- `--model` 走**类型化字段**（`inv.modelWant` · 与 `--ttl`/`--by` 同族）----
	// ★ `inv.hasFlag("--model")` 恒假（它不进 `inv.kv`）⇒ 「给没给」只能看原始 argv；
	//   拿 `flagVal("--model")` 判会给出一枚恒空的串（读法照 `--ttl` 那次血证的同一条）。
	if agentRunFlagGiven(inv, "--model") {
		v := strings.TrimSpace(inv.modelWant)
		if v == "" {
			fmt.Fprintf(stderr, "%s: `--model` 要一个**模型名**（裸给/空值 ⇒ 不执行 —— 不给这一枚 = %s）\n用法: %s\n",
				progName, agentRunDefaultModel, agentRunUsageLine)
			return fail(exitUsage, "usage", "model_empty", "`--model` 裸给或空值")
		}
		cfg.model = v
	}

	// ---- 取值旗标：裸给/空值一律 2（差异 ⑧）----
	strFlag := func(name string, dst *string) int {
		if !inv.hasFlag(name) {
			return exitOK
		}
		v := strings.TrimSpace(inv.flagVal(name))
		if v == "" {
			fmt.Fprintf(stderr, "%s: `%s` 要一个值（裸给/空值 ⇒ 不执行）\n用法: %s\n",
				progName, name, agentRunUsageLine)
			inv.setErr("usage", "flag_value_empty", fmt.Sprintf("`%s` 裸给或空值", name))
			return exitUsage
		}
		*dst = v
		return exitOK
	}
	for _, one := range []struct {
		name string
		dst  *string
	}{
		{"--tools", &cfg.tools},
		{"--mcp", &cfg.mcp},
		{"--state", &cfg.stateFile},
		{"--gateway", &cfg.gateway},
		{"--x3-status", &cfg.x3Status},
		{"--x3-token", &cfg.x3Token},
	} {
		if rc := strFlag(one.name, one.dst); rc != exitOK {
			return cfg, rc
		}
	}

	// ---- 整数旗标（`--max-turns` ≥ 1 · `--retry` ≥ 0）----
	if inv.hasFlag("--max-turns") {
		txt := strings.TrimSpace(inv.flagVal("--max-turns"))
		n, err := strconv.Atoi(txt)
		if txt == "" || err != nil || n < 1 {
			fmt.Fprintf(stderr, "%s: `--max-turns` 要 **≥1 的整数**（给的是 %q）\n用法: %s\n",
				progName, txt, agentRunUsageLine)
			return fail(exitUsage, "usage", "max_turns_bad", "`--max-turns` 不是 ≥1 的整数")
		}
		cfg.maxTurns = n
	}
	if inv.hasFlag("--retry") {
		txt := strings.TrimSpace(inv.flagVal("--retry"))
		n, err := strconv.Atoi(txt)
		if txt == "" || err != nil || n < 0 {
			fmt.Fprintf(stderr, "%s: `--retry` 要 **≥0 的整数**（给的是 %q）\n用法: %s\n",
				progName, txt, agentRunUsageLine)
			return fail(exitUsage, "usage", "retry_bad", "`--retry` 不是 ≥0 的整数")
		}
		cfg.retry = n
	}

	// ---- 布尔档（源件 `-auto-issue` / `-subtask`）----
	cfg.autoIssue = inv.hasFlag("--auto-issue")
	cfg.subtask = inv.hasFlag("--subtask")

	// ---- 工作目录：启动时定死成绝对路径（同 `api serve` 差异 ③ 的口径）----
	wd := agentRunDefaultWorkDir // 源件缺省 `/tmp/zerg-agent`（差异 ⑧ 只管「裸给」，不管「没给」）
	if inv.hasFlag("--workdir") {
		wd = strings.TrimSpace(inv.flagVal("--workdir"))
		if wd == "" {
			fmt.Fprintf(stderr, "%s: `--workdir` 要一个**目录**（裸给/空值 ⇒ 不执行 —— 不给这一枚 = %s）\n用法: %s\n",
				progName, agentRunDefaultWorkDir, agentRunUsageLine)
			return fail(exitUsage, "usage", "agent_workdir_empty", "`--workdir` 裸给或空值")
		}
	}
	abs, err := schedResolveWorkDir(wd)
	if err != nil {
		fmt.Fprintf(stderr, "%s: ❌ failed to resolve workdir: %v\n", progName, err)
		return fail(exitFail, "failed", "agent_workdir_unresolvable", "工作目录解析不出来")
	}
	cfg.workDir = abs

	// ---- 令牌闸（差异 ②：提到执行前 ⇒ 缺令牌时问题单一个字节都不写）----
	cfg.token = config.ResolveAuthToken()
	if cfg.token == "" {
		fmt.Fprintf(stderr, "%s: 缺共享令牌 ⇒ 不执行（源件 `zerg-agent` 同档拒启 · 命令面退码 4 = 未认证）\n"+
			"   给令牌：设环境变量 ZERG_AUTH_TOKEN（见 .env.example），或把它写进 %s\n",
			progName, config.TokenFilePath())
		return fail(exitCodeOf("unauthenticated"), "unauthenticated", "token_missing", "缺共享令牌")
	}
	return cfg, exitOK
}

// agentRunIssueTaskText —— 源件接单档那段任务文本的**逐字搬**（含四步要求）。
func agentRunIssueTaskText(issuePath, issueText string) string {
	return "接单修复问题单 " + issuePath + "。内容:\n" + issueText +
		"\n\n任务: 1.查根因（用 codegraph 查代码/kb 查经验）2.能修则修复+测试验证 " +
		"3.将修复结论（根因/修复/验证）写回该 issue 文件（替换'根因: '等留空处）4.标记状态为 resolved。" +
		"不能修（环境/设计问题）则写结论并标记 escalated。"
}

// agentRunLoadIssue —— 接单档：读问题单 → 标 `fixing` 写回 → 合成任务文本（源件同序 · 差异 ⑦）。
//
// 退码：`0` 读到了且写回了；`1` 读不到 / 写不回（源件把写回的错误丢掉 ⇒ 本件出声）。
func agentRunLoadIssue(inv *invocation, cfg agentRunCfg, stderr io.Writer) (string, int) {
	raw, err := os.ReadFile(cfg.issue)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 错误: 读 issue 失败: %v\n", progName, err)
		inv.setErr("failed", "issue_unreadable", "问题单读不到")
		return "", exitFail
	}
	text := agent.MarkIssueStatus(string(raw), "fixing")
	if werr := os.WriteFile(cfg.issue, []byte(text), 0o644); werr != nil {
		fmt.Fprintf(stderr, "%s: 错误: 标 `fixing` 写不回 %s: %v（不继续跑 —— 免得跑完才发现单子没留痕）\n",
			progName, cfg.issue, werr)
		inv.setErr("failed", "issue_write_failed", "问题单写不回")
		return "", exitFail
	}
	fmt.Fprintf(stderr, "📋 Task accepted: %s (diagnose → fix → report)\n", cfg.issue)
	return agentRunIssueTaskText(cfg.issue, text), exitOK
}

// setErrQuiet —— 给没有 `inv` 在手的内部助手用的一枚极薄包装（错误分类表仍是唯一真源）。
//
// ★ 为什么需要它：接单档的两个助手只认识 `io.Writer`，而 `setErr` 挂在 `invocation` 上。
// 分类与退码仍走 `errorKinds`（✗ 不在这里发明第二张表）。
func setErrQuiet(kind, detail, msg string) {
	_ = kind
	_ = detail
	_ = msg
}

// agentRunExitCode —— 终止原因 → 命令面退码（差异 ③：源件的 3/4 重新取号）。
func agentRunExitCode(reason agent.TerminateReason) int {
	if reason == agent.ReasonComplete {
		return exitCodeOf("ok")
	}
	return exitCodeOf("failed")
}

// agentRunRow —— 结果面的一格（`--json` 的 `items[0]`；字段名 = 命令树 `fields:` 逐字同源）。
func agentRunRow(cfg agentRunCfg, res agent.LoopResult, logDir string, rc int, took time.Duration) map[string]string {
	ok := "false"
	if res.Reason == agent.ReasonComplete {
		ok = "true"
	}
	return map[string]string{
		"ok":          ok,
		"status":      string(res.Reason),
		"turns":       strconv.Itoa(res.Turns),
		"tokens":      strconv.FormatInt(res.Tokens, 10),
		"summary":     res.Content,
		"workdir":     cfg.workDir,
		"logs":        logDir,
		"exit_code":   strconv.Itoa(rc),
		"duration_ms": strconv.FormatInt(took.Milliseconds(), 10),
	}
}

// agentRunHumanSummary —— 人面结果面（源件四行 + 摘要 200 字的逐字搬 · 差异 ⑤：落 stdout）。
func agentRunHumanSummary(stdout io.Writer, cfg agentRunCfg, res agent.LoopResult, logDir string) {
	fmt.Fprintln(stdout, "─────────────────────────────────────")
	fmt.Fprintf(stdout, "\n🏁 Result:\n")
	fmt.Fprintf(stdout, "   status: %s\n", agentRunFormatReason(res.Reason))
	fmt.Fprintf(stdout, "   turns:  %d\n", res.Turns)
	fmt.Fprintf(stdout, "   Token:  %d\n", res.Tokens)
	if res.Content != "" {
		fmt.Fprintf(stdout, "   summary: %s\n", agentRunClip(res.Content, 200))
	}
	fmt.Fprintf(stdout, "\n📂 Workspace: %s\n", cfg.workDir)
	fmt.Fprintf(stdout, "📋 Log:   %s\n", logDir)
}

// agentRunFormatReason —— 源件 `formatReason` 的逐字搬（终止原因 → 人面描述）。
func agentRunFormatReason(reason agent.TerminateReason) string {
	switch string(reason) {
	case "complete":
		return "完成 (checker 通过)"
	case "user_abort":
		return "用户中止"
	case "token_budget":
		return "Token 预算耗尽"
	case "max_turns":
		return "达到最大轮数"
	case "model_error":
		return "模型调用失败"
	default:
		return fmt.Sprintf("terminated (%s)", string(reason))
	}
}

// agentRunClip —— 摘要截断（差异 ⑫：源件按**字节**切 200 ⇒ 中文会被切在字符中间、打出半枚
// 非法 UTF-8；本件按**字符边界**收到 ≤200 字节，尾加 `...`）。ASCII 正文与源件逐字同。
func agentRunClip(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := 0
	for i := range s {
		if i > maxBytes {
			break
		}
		cut = i
	}
	return s[:cut] + "..."
}

// agentRunFlagGiven —— 「这枚旗标给没给」的判据（只看原始 argv：`--x` 与 `--x=v` 两形态都算给）。
//
// ★ 为什么不用 `inv.hasFlag`：它只认写进 `inv.kv` 的那批；`--model` 这类**类型化字段**
// （`inv.modelWant`）不在 `kv` 里 ⇒ 拿 `hasFlag` 判会得出「给了却说没给」。
func agentRunFlagGiven(inv *invocation, name string) bool {
	for _, a := range inv.orig {
		if a == name || strings.HasPrefix(a, name+"=") {
			return true
		}
	}
	return false
}

// agentRunPickTools —— 源件 `filterTools` 的逐字搬（逗号分隔子集 · MCP 合并后统一过滤）。
func agentRunPickTools(all []agent.ToolDef, names string) []agent.ToolDef {
	want := map[string]bool{}
	for _, n := range strings.Split(names, ",") {
		want[strings.TrimSpace(n)] = true
	}
	var out []agent.ToolDef
	for _, t := range all {
		if want[t.Function.Name] {
			out = append(out, t)
		}
	}
	return out
}

// agentRunConnectMCP —— 源件 `-mcp` 那一段的逐字搬（stdio 型与 HTTP 型两档 · 失败只告警不中断）。
func agentRunConnectMCP(mgr *agent.MCPManager, spec string, tools []agent.ToolDef, stderr io.Writer) []agent.ToolDef {
	if spec == "" {
		return tools
	}
	for _, one := range strings.Split(spec, ",") {
		parts := strings.SplitN(one, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name, cmdSpec := parts[0], parts[1]
		// HTTP 型：name:http|url|token
		if strings.HasPrefix(cmdSpec, "http|") {
			hp := strings.Split(cmdSpec, "|")
			if len(hp) >= 2 {
				headers := map[string]string{}
				if len(hp) >= 3 && hp[2] != "" {
					headers["Authorization"] = "Bearer " + hp[2]
				}
				if err := mgr.ConnectHTTP(name, hp[1], headers); err != nil {
					fmt.Fprintf(stderr, "⚠️ MCP HTTP 连接失败 %s: %v\n", name, err)
					continue
				}
				fmt.Fprintf(stderr, "✅ MCP %s connected (HTTP): %d tools (deferred — discovered via tool_search)\n",
					name, len(mgr.ToolDefs(true)))
			}
			continue
		}
		// stdio 型：name:cmd|arg|arg（`|` 分隔参数 ⇒ 路径含空格安全）
		cmdParts := strings.Split(cmdSpec, "|")
		command := cmdParts[0]
		args := cmdParts[1:]
		if err := mgr.Connect(name, command, args...); err != nil {
			fmt.Fprintf(stderr, "⚠️ MCP 连接失败 %s: %v\n", name, err)
			continue
		}
		// 工具分层：核心常驻（kb_*），其余 deferred（`tool_search` 取）
		coreTools := mgr.ToolDefs(false)
		tools = append(tools, coreTools...)
		fmt.Fprintf(stderr, "✅ MCP %s connected: %d core tools (resident) + deferred extensions (via tool_search)\n",
			name, len(coreTools))
	}
	return tools
}

// agentRunNewAgent —— 造一条 Agent（源件 `agent.NewAgent` 那一格 + M3 工具门的 fail-closed 判词）。
//
// ★ M3 接电（§二十一 已红第 9 条 · 批 C `T-31`）：规则表装载失败**不静默** —— 打一条显式告警，
// 并且照旧接上「恒拦」的门（✗ 绝不因为读不到规则表就变成全放行）。
func agentRunNewAgent(cfg agentRunCfg, stderr io.Writer) *agent.Agent {
	a := agent.NewAgent(agent.Config{
		GatewayURL: cfg.gateway,
		AuthToken:  cfg.token,
		Model:      cfg.model,
		MaxTurns:   cfg.maxTurns,
		WorkDir:    cfg.workDir,
	})
	if gate, gerr := control.NewAgentGateFromRepo(statepath.WorkspaceRoot()); gerr == nil {
		a.SetGate(gate)
	} else {
		fmt.Fprintf(stderr, "⚠️ M3 gate 规则表装载失败（工具门按**恒拦**处理，不做任何自动化）：%v\n", gerr)
		a.SetGate(control.NewAgentGate(nil))
	}
	return a
}

// agentRunExecute —— 任务本体：接单档 → 造 Agent → 日志/状态/MCP/技能 → loop（含重试）→ 收尾。
//
// ctx 的**唯一来源**在 `cmdAgentRun`（信号面）；本函数不认识信号 —— 判据面直接调它。
func agentRunExecute(ctx context.Context, inv *invocation, cfg agentRunCfg, stdout, stderr io.Writer) int {
	startTime := time.Now()

	// ---- 接单档（差异 ⑦：两处写盘都看返回值）----
	if cfg.issue != "" {
		task, rc := agentRunLoadIssue(inv, cfg, stderr)
		if rc != exitOK {
			return rc
		}
		cfg.task = task
	}

	// ---- 机器级采样：环境变量兜底（loop 里 NewSysMetricsCollector 读环境）----
	if os.Getenv("ZERG_X3_STATUS") == "" && cfg.x3Status != "" {
		_ = os.Setenv("ZERG_X3_STATUS", cfg.x3Status)
	}
	if os.Getenv("ZERG_X3_TOKEN") == "" && cfg.x3Token != "" {
		_ = os.Setenv("ZERG_X3_TOKEN", cfg.x3Token)
	}

	a := agentRunNewAgent(cfg, stderr)
	fmt.Fprintf(stderr, "✅ Agent created (model: %s)\n", cfg.model)

	// ---- 日志目录（源件同序：`<workdir>/.zerg/logs/<任务 id>` · `ZERG_LOG_DIR` 覆盖）----
	taskID := time.Now().Format("2006-01-02T15-04-05")
	logDir := filepath.Join(cfg.workDir, ".zerg", "logs", taskID)
	if envLogDir := os.Getenv("ZERG_LOG_DIR"); envLogDir != "" {
		logDir = filepath.Join(envLogDir, taskID)
	}
	logger, err := agent.NewLogger(taskID, logDir)
	if err != nil {
		fmt.Fprintf(stderr, "❌ 日志创建失败: %v\n", err)
		inv.setErr("failed", "log_dir_unwritable", "日志目录建不出来")
		return exitFail
	}
	defer logger.Close()
	// 主控 `findTaskLogDir` 靠 audit.jsonl 精确关联任务↔日志（提交时间 ≠ 执行时间）
	if ztd := os.Getenv("ZERG_TASK_DIR"); ztd != "" {
		audit := fmt.Sprintf("{\"task_id\":%q,\"started\":%q}\n", ztd, time.Now().Format(time.RFC3339))
		if werr := os.WriteFile(filepath.Join(logDir, "audit.jsonl"), []byte(audit), 0o644); werr != nil {
			// 差异 ⑦：旁证写不进退码不变，但不许静默。
			fmt.Fprintf(stderr, "⚠️ audit.jsonl 写不进 %s: %v（任务照跑；主控关联日志会退化成按时间猜）\n", logDir, werr)
		}
	}
	// 会话日志（日志 = 上下文真相）：注入 Agent，崩溃可从日志恢复上下文
	if sl, serr := agent.NewSessionLog(logDir); serr == nil {
		a.SetSessionLog(sl)
		defer sl.Close()
		if n := a.RestoreFromLog(); n > 0 {
			fmt.Fprintf(stderr, "♻️ Restored history from the session log: %d messages\n", n)
		}
	} else {
		fmt.Fprintf(stderr, "⚠️ 会话日志装不上 %s: %v（本次任务没有断连恢复面）\n", logDir, serr)
	}
	a.SetLogger(logger)
	fmt.Fprintf(stderr, "✅ Log dir: %s\n", logDir)

	// ---- 状态（断连恢复）----
	var state *agentstate.HarnessState
	if cfg.stateFile != "" {
		if loaded, lerr := agentstate.Load(cfg.stateFile); lerr == nil && loaded != nil {
			state = loaded
			done := 0
			for _, t := range loaded.Todos {
				if t.Status == "done" {
					done++
				}
			}
			fmt.Fprintf(stderr, "🔄 Restored state: todo %d/%d done (resuming)\n", done, len(loaded.Todos))
		}
	}
	if state == nil {
		// ★ 状态件里的 agent 名**逐字保留源件的 `"zerg-agent"`**：`--state` 是断连恢复面，
		//   同一个状态件要能被双轨期的那个二进制读回（改名 = 把在跑的任务的续跑面切断）。
		state = agentstate.NewState(cfg.task, "zerg-agent", nil)
	}
	a.SetState(state)

	mcpMgr := agent.NewMCPManager()
	defer mcpMgr.Close()
	a.SetMCPManager(mcpMgr)
	skillsDir := statepath.SkillsDir()
	if skillsDir == "" {
		fmt.Fprintln(stderr, "⚠️ 未找到技能目录（ZERG_SKILLS_DIR 未设且仓库内 core/internal/agent/skills 不存在）——本次任务将没有可用技能")
	}
	a.SetSkillManager(agent.NewSkillManager(skillsDir))

	// 注入任务到 history（关键——模型必须看到任务指令）
	a.AppendMessage(agent.Message{Role: "user", Content: cfg.task})

	tools := agent.AllTools()
	tools = agentRunConnectMCP(mcpMgr, cfg.mcp, tools, stderr)
	if cfg.tools != "" {
		tools = agentRunPickTools(tools, cfg.tools)
	}
	fmt.Fprintf(stderr, "✅ Tools loaded: %d\n", len(tools))

	fmt.Fprintf(stderr, "\n🚀 Executing task: %s\n", cfg.task)
	fmt.Fprintln(stderr, "─────────────────────────────────────")

	// ---- 执行（失败自动重试）----
	var result agent.LoopResult
	for attempt := 0; attempt <= cfg.retry; attempt++ {
		if cfg.subtask {
			result = agent.RunSubtaskLoop(ctx, a, tools, logger, state, cfg.maxTurns)
		} else {
			result = agent.Loop(ctx, a, tools, logger, state, cfg.maxTurns, 0, 3)
		}
		if cfg.stateFile != "" && state != nil {
			if serr := state.Save(cfg.stateFile); serr != nil {
				fmt.Fprintf(stderr, "⚠️ 状态文件写不回 %s: %v（断连恢复会丢这一次的进度）\n", cfg.stateFile, serr)
			}
		}
		if result.Reason == agent.ReasonComplete {
			break
		}
		if attempt < cfg.retry {
			fmt.Fprintf(stderr, "⚠️ Attempt %d incomplete (%s) — retrying...\n", attempt+1, result.Reason)
			// 清历史重来（新实例 · 同一条 gate fail-closed 口径；差异 ⑪ 不重复告警）
			a = agent.NewAgent(agent.Config{
				GatewayURL: cfg.gateway,
				AuthToken:  cfg.token,
				Model:      cfg.model,
				MaxTurns:   cfg.maxTurns,
				WorkDir:    cfg.workDir,
			})
			if gate, gerr := control.NewAgentGateFromRepo(statepath.WorkspaceRoot()); gerr == nil {
				a.SetGate(gate)
			} else {
				a.SetGate(control.NewAgentGate(nil))
			}
			a.AppendMessage(agent.Message{Role: "user", Content: cfg.task})
			state = agentstate.NewState(cfg.task, "zerg-agent", nil)
			a.SetState(state)
			a.SetMCPManager(mcpMgr)
		}
	}

	// ---- 错误自愈 P0：重试耗尽仍失败 ⇒ 挂单（`--auto-issue`）----
	// 决策矩阵（源件同）：`model_error`（网关瞬时 · 重试已处理 · 无教训价值）不挂单，其余挂单。
	if cfg.autoIssue && result.Reason != agent.ReasonComplete && result.Reason != agent.ReasonModelError {
		issuePath, ierr := agent.CreateIssue(cfg.workDir, cfg.task, result.Reason, cfg.retry, result.ToolTrace)
		if ierr != nil {
			fmt.Fprintf(stderr, "⚠️ 挂单失败: %v\n", ierr)
		} else {
			fmt.Fprintf(stderr, "📋 Failure filed: %s (entered the workflow — pending dispatch)\n", issuePath)
		}
	}

	// ---- 接单档收尾：任务结束自动标 issue 状态（差异 ⑦：写不回 ⇒ 1）----
	if cfg.issue != "" {
		finalStatus := "escalated" // 缺省升级（未完成）
		if result.Reason == agent.ReasonComplete {
			finalStatus = "resolved"
		}
		data, rerr := os.ReadFile(cfg.issue)
		if rerr != nil {
			fmt.Fprintf(stderr, "❌ 收尾读不回问题单 %s: %v\n", cfg.issue, rerr)
			inv.setErr("failed", "issue_unreadable", "问题单读不到")
			return exitFail
		}
		if werr := os.WriteFile(cfg.issue, []byte(agent.MarkIssueStatus(string(data), finalStatus)), 0o644); werr != nil {
			fmt.Fprintf(stderr, "❌ 收尾写不回问题单 %s（状态未回填: %s）: %v\n", cfg.issue, finalStatus, werr)
			inv.setErr("failed", "issue_write_failed", "问题单写不回")
			return exitFail
		}
		fmt.Fprintf(stderr, "📋 Issue status: %s (%s)\n", finalStatus, result.Reason)
	}

	// ---- 结果面 ----
	rc := agentRunExitCode(result.Reason)
	if ctx.Err() != nil {
		// 差异 ⑨：人打断 ⇒ 130（源件同况照常打结果并退 0）
		rc = exitCodeOf("interrupted")
		fmt.Fprintf(stderr, "🛑 interrupted —— 结果面照打，退码取 `interrupted`（%d）\n", rc)
	}
	row := agentRunRow(cfg, result, logDir, rc, time.Since(startTime))
	if inv.jsonGiven {
		if ec := selectJSON(stdout, stderr, inv, inv.path, inv.fields, row); ec != exitOK {
			return ec
		}
		return rc
	}
	agentRunHumanSummary(stdout, cfg, result, logDir)
	return rc
}

// cmdAgentRun —— `zerg agent run`：在本机跑一个 Agent 任务（调网关、执行工具、写自己的任务单）。
//
// ★ 信号面照实（与 `sched run` / `api serve` 同一条）：真二进制里 `main()` 的全局处理器
// （`SIGINT`/`SIGTERM`/`SIGHUP` ⇒ 判词 + `130`）先装；本处另装一枚（两枚都收得到）⇒ 实跑时
// Ctrl-C 多半退 130（差异 ⑨ 与本处同码）。`SIGKILL` 一律到不了这里。
func cmdAgentRun(inv *invocation, stdout, stderr io.Writer) int {
	// K2 甲档（`--json` 不给字段 ⇒ 2 并列出可选字段）与「字段不在册」都在**执行前**判 ——
	// ✗ 不许跑完一整个 loop 才发现 `--json` 拼错了字段名。
	if inv.jsonGiven {
		if len(inv.fields) == 0 {
			return requireFields(inv, stderr)
		}
		if rc := validateSelected(inv, stderr, inv.path, inv.fields); rc != exitOK {
			return rc
		}
	}
	cfg, rc := agentRunParse(inv, stderr)
	if rc != exitOK {
		return rc
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return agentRunExecute(ctx, inv, cfg, stdout, stderr)
}
