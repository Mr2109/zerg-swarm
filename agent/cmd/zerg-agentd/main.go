// zerg-agent: Go 子端 Agent（替换 Python agent.py）
//
// 子端 Agent 职责：
//   - HTTP 服务器 :8100，提供 /status /load /infer /unload 端点
//   - 按需加载模型（llama-server / ds4-server）
//   - 健康检查 + 崩溃自愈 + 熔断
//   - 每 5 秒心跳上报主控
//   - 认证：X-Auth-Token
//
// 用法：
//
//	./zerg-agent --host 0.0.0.0 --machine <name> --controller http://<controller-host>:8580 --registry agent_models.yaml
//	（令牌只走唯一链 ZERG_TOKEN > ~/.zerg/token > ZERG_AUTH_TOKEN > ZERG_API_TOKEN —— 实现真源
//	 `shared/credential`，与主控/CLI 同一份；见仓库 .env.example；--token 旗标已停用）
package main

import (
	"flag"
	"fmt"
	"github.com/Mr2109/zerg-swarm/agent/internal/version"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
	"github.com/Mr2109/zerg-swarm/agent/internal/fleetlogs"
	"github.com/Mr2109/zerg-swarm/agent/internal/heartbeat"
	"github.com/Mr2109/zerg-swarm/agent/internal/logx"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
	"github.com/Mr2109/zerg-swarm/agent/internal/server"
	"github.com/Mr2109/zerg-swarm/agent/internal/startup"
	"github.com/Mr2109/zerg-swarm/shared/credential"
)

var (
	// 命令行参数
	host         = flag.String("host", "127.0.0.1", "HTTP 监听地址（默认 127.0.0.1，部署时用 0.0.0.0）")
	port         = flag.Int("port", 8100, "HTTP 监听端口（默认 8100）")
	tokenArgv    = flag.String("token", "", "已停用：令牌不许走 argv —— 带此旗标即硬失败（用法错，退出码 2）；请改用环境变量或文件 ~/.zerg/token")
	controller   = flag.String("controller", "http://127.0.0.1:8580", "主控地址（心跳上报目标）")
	machineParam = flag.String("machine", "", "机器标识（默认取主机名）")
	registryPath = flag.String("registry", "agent_models.yaml", "模型注册表 YAML 路径")
	logLevel     = flag.String("log-level", "info", "日志级别（debug/info/warn/error）")
	logFile      = flag.String("log-file", "", "日志文件路径（留空只写 stdout）")
)

func main() {
	_ = flag.Bool("version", false, "打印代码身份（机器可读）后退出")
	versionFull := flag.Bool("version-full", false, "打印**完整**代码身份（四格 ＋ 源提交 origin 与来路）后退出；-version 的四格格式不变")
	flag.Parse()

	if v := flag.CommandLine.Lookup("version"); v != nil && v.Value.String() == "true" {
		fmt.Println(version.Line("zerg-agentd"))
		return
	}
	if *versionFull {
		// 新增一格**另起一行**：`-version` 的四格行一个字节不动（现有分析脚本按 $1..$4 切）。
		fmt.Println(version.ReadSelf().Identity("zerg-agentd"))
		return
	}
	// 2026-09-28：令牌只走环境变量/文件 —— --token 旗标已停用（令牌五不进：argv 里看得到命令行）。
	// 值在 exec 那一刻已进进程表(ps)与 shell 历史 ⇒ 收下也追不回；此处只看「带没带」，绝不回显其值。
	tokenFlagGiven := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "token" {
			tokenFlagGiven = true
		}
	})
	if tokenFlagGiven {
		fmt.Fprintln(os.Stderr, "❌ 令牌不许走 argv；请用环境变量或 ~/.zerg/token 文件")
		os.Exit(2)
	}
	token, tokenSrc := resolveToken()

	// 2026-09-11 A 批（库内零明文）：令牌必须可用，否则拒绝启动——
	// 空令牌会让心跳/主控调用全部 401，而进程看起来正常运行 ⇒ 不给结论、不假装健康。
	// 报话机读（`token_source=` ＋ `TOKEN-CHAIN:`），✗ 不回显令牌值。
	if strings.TrimSpace(token) == "" {
		fmt.Fprint(os.Stderr, missingTokenMessage(tokenSrc))
		os.Exit(8)
	}

	// 初始化日志系统（级别 + 文件）
	logx.SetLevel(logx.ParseLevel(*logLevel))
	if *logFile != "" {
		if err := logx.SetFile(*logFile); err != nil {
			fmt.Fprintf(os.Stderr, "日志文件设置失败: %v\n", err)
		}
	}
	startup.Mark("参数与令牌校验通过")
	logx.Infof("main", "zerg-agent 启动", "version", version.Version, "host", *host, "port", *port, "machine", *machineParam,
		// 令牌**只报来源**（GAP-20261017-04：日志里必须看得出「读的是哪一路」，否则两处 env 不同时
		// 各读各的令牌而日志无异常）。✗ 永不拼接令牌值。
		"token_source", string(tokenSrc),
		// 源提交（GAP-20261021-08）：与心跳 origin_sha 同一处真源，日志里先点名一次。
		"origin", version.ReadSelf().OriginText())

	// 机器标识：参数优先，否则取主机名
	m := *machineParam
	if m == "" {
		if h, err := os.Hostname(); err == nil {
			// 取主机名第一部分（如 "evo-host" → "evo"）
			for i := 0; i < len(h); i++ {
				if h[i] == '.' || h[i] == '-' {
					m = h[:i]
					break
				}
			}
			if m == "" {
				m = h
			}
		}
	}
	_ = m

	// 加载模型注册表
	reg, err := registry.New(*registryPath)
	if err != nil {
		log.Fatalf("加载模型注册表失败: %v", err)
	}
	log.Printf("模型注册表加载完成: %d 个模型", len(reg.Names()))
	startup.Mark("注册表加载完成")

	// 启动系统资源采样
	monitor.DefaultSampler.StartLoop()
	startup.Mark("资源采样已启动")

	// 创建后端管理器
	backendMgr := backend.NewManager(reg, m)
	startup.Mark("后端/孵化管理器已建")

	// 创建应用核心
	agent := server.NewAgent(m, token, reg, backendMgr, *controller)

	// 创建心跳上报器：active=agent：active_requests 取真实在飞计数（真值来自 server.Agent.activeReqs）。
	// （unmanaged[] 未托管探测已随 P4 退场清理删除——卵之外无引擎，附录 C·C7。）
	hr := heartbeat.NewRunner(*controller, token, m, backendMgr, monitor.DefaultSampler, agent)
	hr.Start()
	startup.Mark("心跳上报已启动")

	// 机群日志批量上报（缺口 `GAP-20260927-202` ① · 段2）：读本机日志文件（logx 的 `--log-file` 源）、
	// 每 5s 一批增量 POST 主控 `/api/fleet/logs`（照 `gateway/protocol.md:62-65`）；上报失败 ⇒ 出声 + 计数
	// （**不静默吞**）、游标不推进（下轮重试、条目不丢）。✗ 不动心跳/看门狗语义。
	fr := fleetlogs.NewReporter(*controller, token, m, *logFile)
	fr.Start()
	startup.Mark("机群日志上报已启动")

	// 创建 HTTP 服务器
	srv := server.NewServer(agent)
	// srv.Start 是**阻塞**的（2026-09-16 实测：写在它后面的两行永远执行不到 ⇒ /ready 恒 503）
	go func() {
		if err := srv.Start(*host, *port); err != nil {
			log.Fatalf("启动 HTTP 服务器失败: %v", err)
		}
	}()
	// 就绪判据 = **自己的监听真的能被连上**（读状态，不睡秒数）：
	// 连得上 ⇒ 监听已建立 ⇒ Mark + Ready；这段时间里的每一次失败都是"还没到"，不是"坏了"。
	addr := net.JoinHostPort(*host, strconv.Itoa(*port))
	for i := 0; i < 400; i++ {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	startup.Mark("HTTP 已监听")
	startup.Ready()

	// 优雅退出：捕获 SIGTERM / SIGINT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("收到信号 %v，正在退出...", sig)

	// 停止心跳
	hr.Stop()

	// 停止机群日志上报
	fr.Stop()

	// 停止后端
	backendMgr.Stop()

	// 停止 HTTP 服务器
	srv.Stop()

	fmt.Println("子端 Agent 已退出")
}

// TOKEN-CHAIN: ZERG_TOKEN > ~/.zerg/token > ZERG_AUTH_TOKEN > ZERG_API_TOKEN
//
// resolveToken 解析共享令牌（2026-09-11 A 批：库内零明文）。
// ★ **不自备优先级**（GAP-20261017-04）：唯一实现 = `shared/credential.ResolveWithSource`
// —— 与主控/CLI 同一份；本件只负责「旧名命中出声一次」。此前本件把 ZERG_TOKEN 排在旧名之后、
// 令牌文件最后 ⇒ 与 UI 链完全相反，两处 env 不同就各读各的令牌而日志无异常。
// 均未提供时返回空 + `SourceMissing`，由 main 启动期报话并退出（不静默放行）。
func resolveToken() (string, credential.Source) {
	v, src := credential.ResolveWithSource()
	if v == "" {
		return "", credential.SourceMissing
	}
	credential.AnnounceLegacy(src) // 只报来源，✗ 不回显令牌
	return v, src
}

// missingTokenMessage —— 缺令牌时的**机器可读**报话（`token_source=` ＋ `TOKEN-CHAIN:`）。
// ✗ 不含令牌值：本函数只拿得到来源标记与文件路径。
func missingTokenMessage(src credential.Source) string {
	return "❌ 未配置共享令牌\n" +
		"   " + strings.ReplaceAll(credential.MissingHint(), "\n", "\n   ") + "\n" +
		"   本进程实际读数：token_source=" + string(src) + "\n" +
		"   修法：export ZERG_TOKEN=<令牌>，或 printf '%s' <令牌> > " + credential.FilePath() + "（chmod 600）\n" +
		"   令牌五不进：不进 argv / URL / 日志 / 聊天 / 入库（见仓库 .env.example）\n"
}
