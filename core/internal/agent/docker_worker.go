package agent

// docker_worker.go — v2.5.3 T3: worker 容器化
// 设计：DockerWorker 替代 exec zerg-agent，容器内完整执行
// 职责：构造 docker run 命令 → 容器内跑 zerg-agent --issue → 返回退出码
// 状态机流转不变（成功/失败退出码回传调度器）
// 可注入 cmdRunner 函数（测试 mock——生产默认 docker exec.CommandContext）

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// 常量

const (
	defaultImage      = "zerg-dev"      // 默认镜像名
	defaultTimeout    = 10 * time.Minute // 容器执行超时
	defaultNetwork    = "host"           // 容器网络模式
	agentBinaryInHost = "/tmp/zerg-agent-linux-arm64" // 宿主机 zerg-agent 路径
	agentBinaryInCont = "/zerg-agent"                   // 容器内 zerg-agent 路径
)

// DockerWorker — 容器化 worker
// 封装 docker run 命令构造 + 执行 + 退出码解析
// cmdRunner 可注入（测试 mock——生产默认 docker exec.CommandContext）
type DockerWorker struct {
	image    string   // Docker 镜像名
	agentBin string   // 宿主机 zerg-agent 路径
	timeout  time.Duration
	network  string
	cmdRunner func(ctx context.Context, name string, args ...string) (int, error) // 可注入 runner
}

// NewDockerWorker — 创建容器化 worker
// image: 镜像名（默认 "zerg-dev"）
// agentBin: 宿主机 zerg-agent 路径（默认 /tmp/zerg-agent-linux-arm64）
func NewDockerWorker(image, agentBin string) *DockerWorker {
	if image == "" {
		image = defaultImage
	}
	if agentBin == "" {
		agentBin = agentBinaryInHost
	}
	return &DockerWorker{
		image:   image,
		agentBin: agentBin,
		timeout: defaultTimeout,
		network: defaultNetwork,
	}
}

// SetCmdRunner — 注入命令执行器（测试 mock——生产不设置，默认 docker exec）
func (dw *DockerWorker) SetCmdRunner(fn func(ctx context.Context, name string, args ...string) (int, error)) {
	dw.cmdRunner = fn
}

// RunDev — C4 开发模式：容器内双目录挂载——开发仓库(/workspace) + 任务区(/tasks)
// repoDir: 开发仓库路径（宿主机——纯代码——git管理——容器内 /workspace）
// workDir: 任务区路径（宿主机——docs/issues/——容器内 /tasks）
// task: 开发任务描述
// branch: 分支名（如 task-<id>）
// 返回: exitCode（0=成功）, error
func (dw *DockerWorker) RunDev(repoDir, workDir, task, branch string) (int, error) {
	if _, err := os.Stat(dw.agentBin); err != nil {
		return -1, fmt.Errorf("zerg-agent 二进制不存在 %s: %w", dw.agentBin, err)
	}
	if _, err := os.Stat(repoDir); err != nil {
		return -1, fmt.Errorf("开发仓库不存在 %s: %w", repoDir, err)
	}

	// issue 文件名（instanceID）——branch 是 task-<instanceID>——取 branch 去掉 task- 前缀
	issueBase := strings.TrimPrefix(branch, "task-")
	// 容器内命令：
	//   /workspace = repoDir（开发仓库——checkout 分支/改代码/commit/PR）
	//   /tasks     = workDir（任务区——容器内读 issue：/tasks/docs/issues/xxx.md）
	// 挂载：-v repoDir:/workspace + -v workDir:/tasks
	// zerg-agent: --issue /tasks/docs/issues/<instanceID>.md --workdir /workspace
	shellCmd := fmt.Sprintf(
		"cd /workspace && git checkout -b %s 2>/dev/null; "+
			"/zerg-agent -issue /tasks/docs/issues/%s.md -gateway http://host.docker.internal:8082 -workdir /workspace 2>&1; "+
			"git add -A && git -c user.email=zerg@local -c user.name='Zerg AI' commit -m 'task: %s' 2>/dev/null; "+
			"echo '## PR: %s' > pr-%s.md && git diff main...HEAD --stat >> pr-%s.md 2>/dev/null; echo 'PR 已生成'",
		branch, issueBase, branch, branch, branch, branch)

	cmd := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/zerg-agent:ro", dw.agentBin),
		"-v", fmt.Sprintf("%s:/workspace", repoDir),
		"-v", fmt.Sprintf("%s:/tasks", workDir),
		"--network", dw.network,
		"--add-host", "host.docker.internal:host-gateway",
		"--name", fmt.Sprintf("zerg-worker-%d", time.Now().UnixNano()),
		dw.image,
		"sh", "-c", shellCmd,
	}

	log.Printf("🐳 DockerWorker[dev]: docker %s", strings.Join(cmd, " "))
	exitCode, execErr := dw.runCommand(cmd)
	if execErr != nil {
		log.Printf("❌ DockerWorker[dev] 执行异常: %v", execErr)
		return exitCode, execErr
	}
	log.Printf("✅ DockerWorker[dev] 完成: 退出码=%d", exitCode)
	return exitCode, nil
}

// Run — 在容器中执行 zerg-agent（--issue 接单模式）
// issuePath: issue 文件绝对路径（宿主机）
// workDir: 任务工作区绝对路径（宿主机）
// 返回: exitCode（0=成功）, error
func (dw *DockerWorker) Run(issuePath, workDir string) (int, error) {
	// 1. 验证宿主机文件存在
	if _, err := os.Stat(dw.agentBin); err != nil {
		return -1, fmt.Errorf("zerg-agent 二进制不存在 %s: %w", dw.agentBin, err)
	}
	if _, err := os.Stat(issuePath); err != nil {
		return -1, fmt.Errorf("issue 文件不存在 %s: %w", issuePath, err)
	}

	// 2. 构造 docker run 命令
	cmd := dw.buildCommand(issuePath, workDir)

	log.Printf("🐳 DockerWorker: docker %s", strings.Join(cmd, " "))

	// 3. 执行命令
	exitCode, execErr := dw.runCommand(cmd)
	if execErr != nil {
		log.Printf("❌ DockerWorker 执行异常: %v", execErr)
		return exitCode, execErr
	}

	log.Printf("✅ DockerWorker 完成: 退出码=%d", exitCode)
	return exitCode, nil
}

// buildCommand — 构造 docker run 命令参数
// 设计：容器内完整执行——状态机流转不变
// 命令结构:
//
//	docker run --rm \
//	  -v <agentBin>:/zerg-agent:ro \
//	  -v <workDir>:/workspace \
//	  --network host \
//	  --add-host host.docker.internal:host-gateway \
//	  --name zerg-worker-<timestamp> \
//	  <image> \
//	  sh -c "/zerg-agent --issue <issuePath> --workdir /workspace"
//
// 参数说明:
//   - --rm: 执行完自动清理容器
//   - -v agentBin:ro: 只读挂载 zerg-agent 二进制（容器内 /zerg-agent）
//   - -v workDir:/workspace: 挂载任务工作区
//   - --network host: 容器共享宿主网络（访问网关）
//   - --add-host: 容器内解析 host.docker.internal → 宿主网关
//   - --name: 容器命名（带时间戳防冲突）
//   - sh -c: 容器入口——跑 zerg-agent --issue（接单模式）
func (dw *DockerWorker) buildCommand(issuePath, workDir string) []string {
	timestamp := time.Now().UnixNano()
	containerName := fmt.Sprintf("zerg-worker-%d", timestamp)

	// 容器内工作区路径
	workDirInCont := "/workspace"

	// issue 路径转容器内路径（宿主机 workDir → /workspace——issue 在 workDir 下）
	issueInCont := issuePath
	if workDir != "" && strings.HasPrefix(issuePath, workDir) {
		rel := strings.TrimPrefix(issuePath, workDir)
		issueInCont = workDirInCont + rel
	}

	// 构造 sh -c 命令（单引号转义 issueInCont——容器内路径；-gateway 容器内连主控）
	shellCmd := fmt.Sprintf("/zerg-agent --issue '%s' --workdir %s -gateway http://host.docker.internal:8082", issueInCont, workDirInCont)

	return []string{
		"run",
		"--rm",
		"-v", fmt.Sprintf("%s:/zerg-agent:ro", dw.agentBin),
		"-v", fmt.Sprintf("%s:%s", workDir, workDirInCont),
		"--network", dw.network,
		"--add-host", "host.docker.internal:host-gateway",
		"--name", containerName,
		dw.image,
		"sh", "-c", shellCmd,
	}
}

// runCommand — 执行 docker 命令（可注入 mock）
func (dw *DockerWorker) runCommand(args []string) (int, error) {
	if dw.cmdRunner != nil {
		// 注入 runner（测试 mock）
		return dw.cmdRunner(context.Background(), "docker", args...)
	}

	// 生产默认：docker exec.CommandContext
	ctx, cancel := context.WithTimeout(context.Background(), dw.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil // 退出码不是 0 = 失败（正常路径）
		}
		return -1, fmt.Errorf("docker 执行失败: %w\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	return 0, nil
}

// 辅助：退出码解析

// ExitCode — 退出码含义
// 0: 成功（fixing → verified → done）
// 1: 失败（fixing → failed，触发冷却/重派/死信）
// -1: 执行异常（容器启动失败等）
const (
	ExitSuccess = 0
	ExitFailed  = 1
	ExitError   = -1
)
