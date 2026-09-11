// cmd/zerg-scheduler/main.go — v2.5.2 调度器独立命令
// 启动调度器：轮询 docs/issues/ → 按优先级派单 → SIGINT 优雅停止
// 用法: zerg-scheduler [-workdir <path>]

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

func main() {
	workdir := flag.String("workdir", ".", "工作目录（默认当前目录）")
	container := flag.Bool("container", false, "容器模式（docker run 完整环境——CA 脱离沙箱——v2.5.3）")
	agentBin := flag.String("agent-bin", "/tmp/zerg-agent-linux-arm64", "Linux zerg-agent 路径（容器模式挂载）")
	dev := flag.Bool("dev", false, "开发模式（C4：容器内分支开发→commit→PR——配合 -container）")
	repo := flag.String("repo", "", "开发仓库路径（DevMode 用——纯代码 git 管理）")
	flag.Parse()

	// 解析 workdir
	absWorkDir, err := resolveWorkDir(*workdir)
	if err != nil {
		log.Fatalf("❌ failed to resolve workdir: %v", err)
	}
	log.Printf("📂 working directory: %s", absWorkDir)

	// 创建调度器（不注入 runner → 生产默认 exec zerg-agent；-container 切换容器模式）
	sched := agent.NewScheduler(absWorkDir)
	if *container {
		sched.ContainerMode = true
		sched.ContainerAgentBin = *agentBin
		log.Printf("🐳 container mode: docker run zerg-dev (CA outside sandbox — Linux agent=%s)", *agentBin)
	}
	if *dev {
		sched.DevMode = true
		log.Printf("🔀 dev mode: in-container branch work → commit → PR (C4 — merge awaits brain approval)")
	}
	if *repo != "" {
		sched.RepoDir = *repo
		log.Printf("📦 dev repository: %s", *repo)
	} else if *dev {
		log.Printf("⚠️ DevMode without -repo; using workDir as the dev repository (compatibility)")
		sched.RepoDir = absWorkDir
	}
	if *repo != "" {
		sched.RepoDir = *repo
		log.Printf("📦 dev repository: %s", *repo)
	} else if *dev {
		log.Printf("⚠️ DevMode without -repo; using workDir as the dev repository (compatibility)")
		sched.RepoDir = absWorkDir
	}
	log.Printf("⚙️  concurrency limit: %d workers", sched.MaxWorkers())

	// 启动调度器（后台 goroutine——让 main 能监听信号）
	ctx, cancel := context.WithCancel(context.Background())
	go sched.Start(ctx)

	// 状态打印 goroutine（每轮打印一次扫描结果摘要）
	go printStatusLoop(sched)

	// 等待 SIGINT / SIGTERM → 优雅停止
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("\n📡 signal received: %v, shutting down gracefully...", sig)

	// 1. 停止轮询
	sched.Stop()
	// 2. 取消 context（Stop 已 close doneCh，cancel 确保 goroutine 退出）
	cancel()

	log.Println("🛑 scheduler stopped")
}

// resolveWorkDir — 解析 workdir 为绝对路径
func resolveWorkDir(wd string) (string, error) {
	if wd == "" || wd == "." {
		return os.Getwd()
	}
	abs, err := filepath.Abs(wd)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	return abs, nil
}

// printStatusLoop — 每 10 秒打印一次调度器状态
func printStatusLoop(sched *agent.Scheduler) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		log.Printf("📊 scheduler status: active=%d/%d", sched.ActiveCount(), sched.MaxWorkers())
	}
}
