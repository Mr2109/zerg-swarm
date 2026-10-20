// family_sched_run.go —— `zerg sched run`（调度器常驻档 · **归入命令面** · 批 B2）。
//
// 源件 = 独立入口 `core/cmd/zerg-scheduler/main.go`（104 行 · Go `flag` 五枚 · 缺省 `-workdir .`）。
// 本件是那一件的**逐面搬**：解析工作目录 → 造 `agent.Scheduler` → 容器/开发两档 → 起轮询 →
// 每 10s 报一次状态 → SIGINT/SIGTERM 优雅停。
//
// 形态（旗标**逐字**保留源件那五枚 · 只把 Go `flag` 的单横线写成命令面的双横线）：
//
//	zerg sched run [--workdir <目录>] [--container] [--agent-bin <路径>] [--dev] [--repo <路径>]
//
//	--workdir <目录>   工作目录（源件缺省 `"."` ⇒ 本件同义：不给 = 当前目录）
//	--container        容器模式（docker run 完整环境 —— CA 脱离沙箱 · v2.5.3）
//	--agent-bin <路径>  Linux zerg-agent 路径（容器模式挂载 · 源件缺省 `/tmp/zerg-agent-linux-arm64`）
//	--dev              开发模式（C4：容器内分支开发 → commit → PR —— 配合 --container）
//	--repo <路径>      开发仓库路径（DevMode 用 —— 纯代码 git 管理）
//
// 为什么动作名是 `run`：`sched` 族**已经在册**（`show` / `pause` / `resume` / `set` = 打控制面
// `/api/sched*` 的**控制档**）。本件搬的是**另一件事** —— 在本机**起**一个调度器进程（源件
// `zerg-scheduler` 的语义）。两者共用族名、动作名分家：`run` = 起进程（本件）· 其余四条 = 管在跑的那个。
// ✗ 不新开顶层族（`scheduler` 仍留独立二进制；`sched` 已是既有族 ⇒ 只增一格）。
//
// 口径（与源件逐条一致，✗ 不改一处语义）：
//
//	· 工作目录解析：`""` / `"."` ⇒ `os.Getwd()`；其余 ⇒ `filepath.Abs`（源件同式 · 不判存在性）。
//	· 容器档：`ContainerMode=true` + `ContainerAgentBin=--agent-bin`；开发档：`DevMode=true`；
//	  `--repo` 给了 ⇒ `RepoDir=--repo`；只给 `--dev` 不给 `--repo` ⇒ `RepoDir=工作目录` + 一行告警。
//	· 启动行 / 状态行 文案与源件逐字同（`📂` / `🐳` / `🔀` / `📦` / `⚠️` / `⚙️` / `📊` / `🛑`）—— 停止面那一行见差异 ③，
//	  只把源件走 `log` 的 stdout/stderr 分工按命令面 §4.1 K4 收到 **stderr**（人面进度不进 stdout）。
//	· 状态循环 = 每 10s 一行 `📊 scheduler status: active=%d/%d`（源件同间隔、同格式）。
//
// 退码（真源 = `exitcodes.go` 的表；本文件不写死数字）：
//
//	0  常驻档跑到收到 SIGINT/SIGTERM 并停干净
//	2  用法错（`--workdir` 裸给/空值 —— 见差异 ①）
//	1  工作目录解析不出来（源件 `log.Fatalf` ⇒ 同档：跑到了、没成功）
//
// 与源件的**照实差异**（四处 · 逐条写清，别处不猜）：
//
//	① `--workdir` **裸给 / 空值** ⇒ 用法错 2（源件 `-workdir ""` 静默当「当前目录」）。
//	   理由 = 命令面同族纪律（`GAP-20260928-151`：值旗标裸给不许静默吞）；`--workdir .` 与
//	   不给这一枚仍是「当前目录」，语义一字不改。
//	② 源件缺参/未知旗标由 Go `flag` 打用法并退 **2**（`flag.ExitOnError` 的档）⇒ 本件同档，一致。
//	③ 停止判词不带具体信号名：本件用 `signal.NotifyContext`（只关心「要不要停」），源件是
//	   `📡 signal received: interrupt`。文案差异只在这一行，退码同为 0。
//	④ 源件第 47-60 行把「`--repo` / 只给 `--dev`」那一段**逐字写了两遍**（同一个 if/else 两连）——
//	   效果逐字相同（第二遍盖的是同一个值）⇒ 本件只留一段，**行为零差**。照实记一笔：
//	   这不是「顺手清理」，是搬的时候同一条 if 的两份拷贝只能落一份。
//
// ★ 双轨期：`core/cmd/zerg-scheduler` 二进制**一字不动**（本件只搬逻辑、不删入口）。
// ★ 常驻档的取数面：矩阵一格 = 一条 argv = 一跑，而常驻档**一跑不返回** ⇒ 判据面落在
//
//	`family_sched_run_test.go`（`schedRunServe` 是同一实现的入口，测试用已取消的 ctx 走
//	「起 → 停」一整趟）。同族先例 = `impact` 的 A2/A4/A5 那几笔（一条 argv 写不出的档落 go test）。
//
// ★ 本件**不是** `family_sched.go`：那个名字已被 `sched` 写面确认门占着（`pause|resume|set`
//
//	的 D2 门 · 缺口 `GAP-20261005-10`）。同族一件一事，本件独立成文。
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// schedRunDefaultAgentBin —— 源件 `-agent-bin` 的缺省值（容器模式挂载的 Linux zerg-agent 路径）。
const schedRunDefaultAgentBin = "/tmp/zerg-agent-linux-arm64"

// schedRunStatusEvery —— 状态行间隔（源件 `printStatusLoop` 的 10s）。
const schedRunStatusEvery = 10 * time.Second

// schedRunCfg —— 解析后的启动配置（`workDir` 已是绝对路径；零值不合法）。
type schedRunCfg struct {
	workDir   string
	container bool
	agentBin  string
	dev       bool
	repo      string
}

// schedResolveWorkDir —— 源件 `resolveWorkDir` 的逐字搬：`""` / `"."` ⇒ 当前目录，其余 ⇒ 绝对路径。
func schedResolveWorkDir(wd string) (string, error) {
	if wd == "" || wd == "." {
		return os.Getwd()
	}
	abs, err := filepath.Abs(wd)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	return abs, nil
}

// schedRunParse —— 旗标 → 配置（语义校验只在本处）。
//
// 退码：`0` 配置齐；`2` `--workdir` 裸给/空值（差异 ①）；`1` 工作目录解析不出来。
func schedRunParse(inv *invocation, stderr io.Writer) (schedRunCfg, int) {
	cfg := schedRunCfg{agentBin: schedRunDefaultAgentBin}

	wd := "."
	if inv.hasFlag("--workdir") {
		wd = strings.TrimSpace(inv.flagVal("--workdir"))
		if wd == "" {
			inv.setErr("usage", "sched_workdir_empty", "`--workdir` 裸给或空值")
			fmt.Fprintf(stderr, "%s: `--workdir` 要一个**目录**（裸给/空值 ⇒ 不执行 —— 不给这一枚 = 当前目录）\n", progName)
			fmt.Fprintf(stderr, "用法: zerg sched run [--workdir <目录>] [--container] [--agent-bin <路径>] [--dev] [--repo <路径>]\n")
			return cfg, exitUsage
		}
	}
	abs, err := schedResolveWorkDir(wd)
	if err != nil {
		inv.setErr("failed", "sched_workdir_unresolvable", "工作目录解析不出来")
		fmt.Fprintf(stderr, "%s: ❌ failed to resolve workdir: %v\n", progName, err)
		return cfg, exitFail
	}
	cfg.workDir = abs

	cfg.container = inv.hasFlag("--container")
	if inv.hasFlag("--agent-bin") {
		cfg.agentBin = inv.flagVal("--agent-bin")
	}
	cfg.dev = inv.hasFlag("--dev")
	cfg.repo = strings.TrimSpace(inv.flagVal("--repo"))
	return cfg, exitOK
}

// schedRunPrepare —— 配置 → 调度器（源件 `agent.NewScheduler` + 两档 + `RepoDir` 那一段的逐字搬）。
func schedRunPrepare(cfg schedRunCfg) *agent.Scheduler {
	sched := agent.NewScheduler(cfg.workDir)
	if cfg.container {
		sched.ContainerMode = true
		sched.ContainerAgentBin = cfg.agentBin
	}
	if cfg.dev {
		sched.DevMode = true
	}
	if cfg.repo != "" {
		sched.RepoDir = cfg.repo
	} else if cfg.dev {
		// 源件同档（`⚠️ DevMode without -repo`）：不给仓库 ⇒ 拿工作目录当开发仓库。
		sched.RepoDir = cfg.workDir
	}
	return sched
}

// schedRunBanner —— 启动面五行（文案逐字照源件；分工按 K4 落 stderr）。
func schedRunBanner(cfg schedRunCfg, sched *agent.Scheduler, stderr io.Writer) {
	fmt.Fprintf(stderr, "📂 working directory: %s\n", cfg.workDir)
	if cfg.container {
		fmt.Fprintf(stderr, "🐳 container mode: docker run zerg-dev (CA outside sandbox — Linux agent=%s)\n", cfg.agentBin)
	}
	if cfg.dev {
		fmt.Fprintln(stderr, "🔀 dev mode: in-container branch work → commit → PR (C4 — merge awaits brain approval)")
	}
	if cfg.repo != "" {
		fmt.Fprintf(stderr, "📦 dev repository: %s\n", cfg.repo)
	} else if cfg.dev {
		fmt.Fprintln(stderr, "⚠️ DevMode without -repo; using workDir as the dev repository (compatibility)")
	}
	fmt.Fprintf(stderr, "⚙️  concurrency limit: %d workers\n", sched.MaxWorkers())
}

// schedStatusLoop —— 源件 `printStatusLoop` 的逐字搬（同间隔、同格式；`done` 关即退）。
func schedStatusLoop(sched *agent.Scheduler, stderr io.Writer, done <-chan struct{}) {
	ticker := time.NewTicker(schedRunStatusEvery)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			fmt.Fprintf(stderr, "📊 scheduler status: active=%d/%d\n", sched.ActiveCount(), sched.MaxWorkers())
		}
	}
}

// schedRunServe —— 常驻档本体：起轮询 → 等 ctx 结束 → 停干净（`0`）。
//
// ctx 的**唯一来源**在 `cmdSchedRun`（信号面）；本函数不认识信号 —— 测试用已取消的 ctx
// 就走完「起 → 停」一整趟（同一条实现，两个入口）。
func schedRunServe(ctx context.Context, cfg schedRunCfg, stderr io.Writer) int {
	sched := schedRunPrepare(cfg)
	schedRunBanner(cfg, sched, stderr)

	runCtx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() {
		sched.Start(runCtx)
		close(stopped)
	}()
	statusDone := make(chan struct{})
	go schedStatusLoop(sched, stderr, statusDone)

	<-ctx.Done()
	fmt.Fprintln(stderr, "📡 stop signal received, shutting down gracefully...")
	// 源件顺序：先 `Stop()`（关 doneCh + 取消轮询），再 `cancel()` 兜 goroutine 退出。
	sched.Stop()
	cancel()
	<-stopped
	close(statusDone)

	fmt.Fprintln(stderr, "🛑 scheduler stopped")
	return exitOK
}

// cmdSchedRun —— `zerg sched run`：本机起一个调度器（常驻到收到 SIGINT/SIGTERM）。
//
// ★ 信号面照实：真二进制里 `main()` 的全局处理器（`SIGINT`/`SIGTERM`/`SIGHUP` ⇒ 判词 + `130`）
// 先装；`sched` 这一档**不改**全局信号面（它为所有族管 Ctrl-C —— 同 `chat` 那条已拍口径），
// 本处也装一枚（两枚都收得到）：谁先赢不保证 ⇒ 实跑时 Ctrl-C **多半退 130**（在跑 worker 不受影响，
// 与 `--cancel-on-interrupt` 缺省「只退订、不取消」同义）。`SIGKILL` 一律到不了这里。
func cmdSchedRun(inv *invocation, stdout, stderr io.Writer) int {
	cfg, rc := schedRunParse(inv, stderr)
	if rc != exitOK {
		return rc
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return schedRunServe(ctx, cfg, stderr)
}
