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
//   ./zerg-agent --host 0.0.0.0 --machine x3 --controller http://<controller-host>:8580 --token x3gw-shared-2026 --registry agent_models.yaml
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"zerg/agent/internal/backend"
	"zerg/agent/internal/heartbeat"
	"zerg/agent/internal/logx"
	"zerg/agent/internal/monitor"
	"zerg/agent/internal/registry"
	"zerg/agent/internal/server"
)

var (
	// 命令行参数
	host         = flag.String("host", "127.0.0.1", "HTTP 监听地址（默认 127.0.0.1，部署时用 0.0.0.0）")
	port         = flag.Int("port", 8100, "HTTP 监听端口（默认 8100）")
	token        = flag.String("token", "x3gw-shared-2026", "共享认证令牌")
	controller   = flag.String("controller", "http://127.0.0.1:8580", "主控地址（心跳上报目标）")
	machineParam = flag.String("machine", "", "机器标识（默认取主机名）")
	registryPath = flag.String("registry", "agent_models.yaml", "模型注册表 YAML 路径")
	logLevel     = flag.String("log-level", "info", "日志级别（debug/info/warn/error）")
	logFile      = flag.String("log-file", "", "日志文件路径（留空只写 stdout）")
)

func main() {
	flag.Parse()

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

	// 创建心跳上报器
	hr := heartbeat.NewRunner(*controller, *token, m, backendMgr, monitor.DefaultSampler)
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
