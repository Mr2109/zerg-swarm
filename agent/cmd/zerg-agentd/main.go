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
//	./zerg-agent --host 0.0.0.0 --machine <name> --controller http://<controller-host>:8580 --token <your-token> --registry agent_models.yaml
//	（token 也可用环境变量 ZERG_AUTH_TOKEN，或文件 ~/.zerg/token 提供——见仓库 .env.example）
package main

import (
	"flag"
	"fmt"
	"github.com/Mr2109/zerg-swarm/agent/internal/version"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Mr2109/zerg-swarm/agent/internal/backend"
	"github.com/Mr2109/zerg-swarm/agent/internal/heartbeat"
	"github.com/Mr2109/zerg-swarm/agent/internal/logx"
	"github.com/Mr2109/zerg-swarm/agent/internal/monitor"
	"github.com/Mr2109/zerg-swarm/agent/internal/registry"
	"github.com/Mr2109/zerg-swarm/agent/internal/server"
)

var (
	// 命令行参数
	host         = flag.String("host", "127.0.0.1", "HTTP 监听地址（默认 127.0.0.1，部署时用 0.0.0.0）")
	port         = flag.Int("port", 8100, "HTTP 监听端口（默认 8100）")
	token        = flag.String("token", "", "共享认证令牌（默认取环境变量 ZERG_AUTH_TOKEN 或文件 ~/.zerg/token）——留空则解析环境变量/文件；默认值不用真值，避免 --help 泄漏")
	controller   = flag.String("controller", "http://127.0.0.1:8580", "主控地址（心跳上报目标）")
	machineParam = flag.String("machine", "", "机器标识（默认取主机名）")
	registryPath = flag.String("registry", "agent_models.yaml", "模型注册表 YAML 路径")
	logLevel     = flag.String("log-level", "info", "日志级别（debug/info/warn/error）")
	logFile      = flag.String("log-file", "", "日志文件路径（留空只写 stdout）")
	// 未托管监听探测：覆盖实测 E2（手工 screen 起的服务）——只读 TCP 探测，只标注不接管。
	unmanagedScan = flag.String("unmanaged-scan", "9000-9999", "未托管监听探测端口清单（如 9000-9999 / 8100,8101）；空串关闭")
)

func main() {
	_ = flag.Bool("version", false, "打印代码身份（机器可读）后退出")
	flag.Parse()

	if v := flag.CommandLine.Lookup("version"); v != nil && v.Value.String() == "true" {
		fmt.Println(version.Line("zerg-agentd"))
		return
	}
	// 令牌在解析后补齐：声明期不解析，避免 --help/未知参数把真实令牌打进日志
	if *token == "" {
		*token = resolveToken()
	}

	// 2026-09-11 A 批（库内零明文）：令牌必须可用，否则拒绝启动——
	// 空令牌会让心跳/主控调用全部 401，而进程看起来正常运行。
	if strings.TrimSpace(*token) == "" {
		log.Fatalf("❌ 未配置共享令牌：--token、环境变量 ZERG_AUTH_TOKEN，或文件 ~/.zerg/token（见仓库 .env.example）")
	}

	// 初始化日志系统（级别 + 文件）
	logx.SetLevel(logx.ParseLevel(*logLevel))
	if *logFile != "" {
		if err := logx.SetFile(*logFile); err != nil {
			fmt.Fprintf(os.Stderr, "日志文件设置失败: %v\n", err)
		}
	}
	logx.Infof("main", "zerg-agent 启动", "version", "v2.0", "host", *host, "port", *port, "machine", *machineParam)

	// 机器标识：参数优先，否则取主机名
	m := *machineParam
	if m == "" {
		if h, err := os.Hostname(); err == nil {
			// 取主机名第一部分（如 "evo-x3" → "evo"）
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

	// 启动系统资源采样
	monitor.DefaultSampler.StartLoop()

	// 创建后端管理器
	backendMgr := backend.NewManager(reg, m)

	// 创建应用核心
	agent := server.NewAgent(m, *token, reg, backendMgr, *controller)

	// 创建心跳上报器：
	//   - active=agent：active_requests 取真实在飞计数（真值来自 server.Agent.activeReqs）
	//   - unmanaged：只读探测未托管监听端口（只标注 managed=false，绝不接管/杀）
	probePorts := backend.ParsePortSpec(*unmanagedScan)
	hr := heartbeat.NewRunner(*controller, *token, m, backendMgr, monitor.DefaultSampler, agent, func() []backend.UnmanagedProcess {
		if len(probePorts) == 0 {
			return nil
		}
		return backendMgr.UnmanagedListeners(probePorts)
	})
	hr.Start()

	// 创建 HTTP 服务器
	srv := server.NewServer(agent)
	if err := srv.Start(*host, *port); err != nil {
		log.Fatalf("启动 HTTP 服务器失败: %v", err)
	}

	// 优雅退出：捕获 SIGTERM / SIGINT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("收到信号 %v，正在退出...", sig)

	// 停止心跳
	hr.Stop()

	// 停止后端
	backendMgr.Stop()

	// 停止 HTTP 服务器
	srv.Stop()

	fmt.Println("子端 Agent 已退出")
}

// resolveToken 解析共享令牌（2026-09-11 A 批：库内零明文）。
// 优先级：环境变量 ZERG_AUTH_TOKEN → 兼容旧名 ZERG_API_TOKEN / ZERG_TOKEN → 文件 ~/.zerg/token。
// 均未提供时返回空串，由 main 启动期直接报错退出。
func resolveToken() string {
	for _, k := range []string{"ZERG_AUTH_TOKEN", "ZERG_API_TOKEN", "ZERG_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if b, err := os.ReadFile(filepath.Join(home, ".zerg", "token")); err == nil {
			if v := strings.TrimSpace(string(b)); v != "" {
				return v
			}
		}
	}
	return ""
}
