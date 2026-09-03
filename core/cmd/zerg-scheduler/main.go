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

	"zerg/core/internal/agent"
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
		log.Fatalf("❌ 解析 workdir 失败: %v", err)
	}
	log.Printf("📂 工作目录: %s", absWorkDir)

	// 创建调度器（不注入 runner → 生产默认 exec zerg-agent；-container 切换容器模式）
	sched := agent.NewScheduler(absWorkDir)
	if *container {
		sched.ContainerMode = true
		sched.ContainerAgentBin = *agentBin
		log.Printf("🐳 容器模式: docker run zerg-dev（CA 脱离沙箱——Linux agent=%s）", *agentBin)
	}
	if *dev {
		sched.DevMode = true
		log.Printf("🔀 开发模式: 容器内分支开发→commit→PR（C4——等待脑确认合并）")
	}
	if *repo != "" {
		sched.RepoDir = *repo
		log.Printf("📦 开发仓库: %s", *repo)
	} else if *dev {
		log.Printf("⚠️ DevMode 但未指定 -repo，将用 workDir 作为开发仓库（兼容）")
		sched.RepoDir = absWorkDir
	}
	if *repo != "" {
		sched.RepoDir = *repo
		log.Printf("📦 开发仓库: %s", *repo)
	} else if *dev {
		log.Printf("⚠️ DevMode 但未指定 -repo，将用 workDir 作为开发仓库（兼容）")
		sched.RepoDir = absWorkDir
	}
	log.Printf("⚙️  并发上限: %d workers", sched.MaxWorkers())

	// 启动调度器（后台 goroutine——让 main 能监听信号）
	ctx, cancel := context.WithCancel(context.Background())
	go sched.Start(ctx)

	// 状态打印 goroutine（每轮打印一次扫描结果摘要）
	go printStatusLoop(sched)

	// 等待 SIGINT / SIGTERM → 优雅停止
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("\n📡 收到信号: %v，开始优雅停止...", sig)

	// 1. 停止轮询
	sched.Stop()
	// 2. 取消 context（Stop 已 close doneCh，cancel 确保 goroutine 退出）
	cancel()

	log.Println("🛑 调度器已停止")
}

// resolveWorkDir — 解析 workdir 为绝对路径
func resolveWorkDir(wd string) (string, error) {
	if wd == "" || wd == "." {
		return os.Getwd()
	}
	abs, err := filepath.Abs(wd)
	if err != nil {
		return "", fmt.Errorf("路径解析失败: %w", err)
	}
	return abs, nil
}

// printStatusLoop — 每 10 秒打印一次调度器状态
func printStatusLoop(sched *agent.Scheduler) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		log.Printf("📊 调度器状态: 活跃=%d/%d", sched.ActiveCount(), sched.MaxWorkers())
	}
}
