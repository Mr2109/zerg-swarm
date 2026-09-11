package agent

// docker_worker_test.go — DockerWorker 测试
// mock docker 命令——不真跑容器
// 测试：命令构造 / 退出码解析 / 文件验证 / 超时处理 / 注入 runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── 测试辅助：临时文件/目录 ──────────────────────────────

func createTempIssue(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	issuePath := filepath.Join(dir, "issue-001.md")
	content := `---
instance_id: "issue-001"
status: "open"
priority: "high"
---

# Issue: 测试任务

- **状态**: open
- **优先级**: high
- **重试次数**: 0
- **创建时间**: 2026-08-15T10:00:00Z
- **最后尝试**: 

这是一个测试 issue，用于验证 DockerWorker。
`
	if err := os.WriteFile(issuePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return issuePath
}

func createTempAgent(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	agentPath := filepath.Join(dir, "zerg-agent-linux-arm64")
	// 创建一个可执行的 dummy 脚本
	content := `#!/bin/sh
echo "zerg-agent --issue $1 --workdir $2"
exit 0
`
	if err := os.WriteFile(agentPath, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return agentPath
}

// mockRunner — 模拟 docker 命令执行
// 返回: 被调用的 cmd, args, 以及预设的退出码
type mockRunner struct {
	called    bool
	args      []string
	exitCode  int
	err       error
	agentBin  string // 期望的宿主机路径
	issuePath string // 期望的 issue 路径
	workDir   string // 期望的工作区
}

func newMockRunner() *mockRunner {
	return &mockRunner{}
}

func (m *mockRunner) run(_ context.Context, name string, args ...string) (int, error) {
	m.called = true
	m.args = args
	return m.exitCode, m.err
}

// ─── 测试 1: 成功退出码 0 ─────────────────────────────────

func TestDockerWorker_Run_Success(t *testing.T) {
	agentBin := createTempAgent(t)
	issuePath := createTempIssue(t)
	workDir := t.TempDir()

	mr := newMockRunner()
	mr.exitCode = 0

	dw := NewDockerWorker("", "")
	dw.agentBin = agentBin
	dw.SetCmdRunner(mr.run)

	exitCode, err := dw.Run(issuePath, workDir)

	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("期望退出码 0，实际 %d", exitCode)
	}
	if !mr.called {
		t.Fatal("mock runner 未被调用")
	}

	// 验证命令参数
	if len(mr.args) < 3 {
		t.Fatalf("命令参数不足: %v", mr.args)
	}
	if mr.args[0] != "run" {
		t.Fatalf("期望 docker run，实际: %v", mr.args[0])
	}

	// 验证关键参数存在
	cmdStr := strings.Join(mr.args, " ")
	if !strings.Contains(cmdStr, "--rm") {
		t.Error("命令应包含 --rm")
	}
	if !strings.Contains(cmdStr, "--network host") {
		t.Error("命令应包含 --network host")
	}
	if !strings.Contains(cmdStr, "--add-host") {
		t.Error("命令应包含 --add-host")
	}
	if !strings.Contains(cmdStr, "/zerg-agent") {
		t.Error("命令应包含 /zerg-agent（容器内路径）")
	}

	// 验证 volume 挂载（-v 分开传：arg=="-v" 后跟值）
	var hasAgentVol, hasWorkVol bool
	for i, arg := range mr.args {
		if arg == "-v" && i+1 < len(mr.args) {
			if strings.HasPrefix(mr.args[i+1], agentBin+":") {
				hasAgentVol = true
			}
			if strings.HasPrefix(mr.args[i+1], workDir+":") {
				hasWorkVol = true
			}
		}
	}
	if !hasAgentVol {
		t.Errorf("命令应挂载 agent 二进制: %v", mr.args)
	}
	if !hasWorkVol {
		t.Errorf("命令应挂载工作区: %v", mr.args)
	}

	// 验证镜像名
	var foundImage bool
	for _, arg := range mr.args {
		if arg == "zerg-dev" {
			foundImage = true
		}
	}
	if !foundImage {
		t.Errorf("命令应包含镜像名 zerg-dev: %v", mr.args)
	}

	// 验证 sh -c 命令
	var foundShellCmd bool
	for i, arg := range mr.args {
		if arg == "-c" && i+1 < len(mr.args) && strings.Contains(mr.args[i+1], "--issue") {
			foundShellCmd = true
		}
	}
	if !foundShellCmd {
		t.Errorf("命令应包含 sh -c --issue: %v", mr.args)
	}

	t.Logf("✅ TestDockerWorker_Run_Success: 退出码=%d, 命令=%v", exitCode, mr.args)
}

// ─── 测试 2: 失败退出码 1 ─────────────────────────────────

func TestDockerWorker_Run_Failure(t *testing.T) {
	agentBin := createTempAgent(t)
	issuePath := createTempIssue(t)
	workDir := t.TempDir()

	mr := newMockRunner()
	mr.exitCode = 1 // 失败（正常路径——状态机处理）

	dw := NewDockerWorker("", "")
	dw.agentBin = agentBin
	dw.SetCmdRunner(mr.run)

	exitCode, err := dw.Run(issuePath, workDir)

	if err != nil {
		t.Fatalf("Run 应返回 nil error（退出码 1 是正常失败路径）: %v", err)
	}
	if exitCode != 1 {
		t.Fatalf("期望退出码 1，实际 %d", exitCode)
	}
	t.Logf("✅ TestDockerWorker_Run_Failure: 退出码=%d（触发冷却/重派/死信）", exitCode)
}

// ─── 测试 3: 文件不存在 ────────────────────────────────────

func TestDockerWorker_Run_FileNotFound(t *testing.T) {
	agentBin := createTempAgent(t)
	issuePath := "/nonexistent/issue-999.md" // 不存在的文件
	workDir := t.TempDir()

	dw := NewDockerWorker("", "")
	dw.agentBin = agentBin

	exitCode, err := dw.Run(issuePath, workDir)

	if err == nil {
		t.Fatal("期望返回错误（文件不存在）")
	}
	if exitCode != -1 {
		t.Fatalf("期望退出码 -1，实际 %d", exitCode)
	}
	if !strings.Contains(err.Error(), "issue file not found") {
		t.Errorf("错误信息应包含 'issue 文件不存在': %v", err)
	}
	t.Logf("✅ TestDockerWorker_Run_FileNotFound: 退出码=%d, 错误=%v", exitCode, err)
}

// ─── 测试 4: 注入 runner 超时 ─────────────────────────────

func TestDockerWorker_Run_CustomImage(t *testing.T) {
	agentBin := createTempAgent(t)
	issuePath := createTempIssue(t)
	workDir := t.TempDir()

	mr := newMockRunner()
	mr.exitCode = 0

	customImage := "my-custom-zerg:latest"
	dw := NewDockerWorker(customImage, agentBin)
	dw.SetCmdRunner(mr.run)

	exitCode, err := dw.Run(issuePath, workDir)

	if err != nil {
		t.Fatalf("Run 返回错误: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("期望退出码 0，实际 %d", exitCode)
	}

	// 验证使用自定义镜像
	var foundImage bool
	for _, arg := range mr.args {
		if arg == customImage {
			foundImage = true
		}
	}
	if !foundImage {
		t.Errorf("应使用自定义镜像 %s: %v", customImage, mr.args)
	}

	// 验证不使用默认镜像
	for _, arg := range mr.args {
		if arg == "zerg-dev" {
			t.Error("不应使用默认镜像 zerg-dev")
		}
	}

	t.Logf("✅ TestDockerWorker_Run_CustomImage: 镜像=%s, 退出码=%d", customImage, exitCode)
}

// ─── 测试 5: 容器启动失败（exec 错误） ────────────────────

func TestDockerWorker_Run_ExecError(t *testing.T) {
	agentBin := createTempAgent(t)
	issuePath := createTempIssue(t)
	workDir := t.TempDir()

	mr := newMockRunner()
	mr.err = fmt.Errorf("docker daemon not running") // 模拟 docker 未启动

	dw := NewDockerWorker("", "")
	dw.agentBin = agentBin
	dw.SetCmdRunner(mr.run)

	exitCode, err := dw.Run(issuePath, workDir)

	if err == nil {
		t.Fatal("期望返回错误（docker 未运行）")
	}
	// mock 返回 (0, err)——exitCode 透传 mock 值（真实 exec 错误才是 -1——由实现处理）
	if !strings.Contains(err.Error(), "docker 执行失败") && !strings.Contains(err.Error(), "daemon") {
		t.Errorf("错误信息应包含 'docker 执行失败': %v", err)
	}
	t.Logf("✅ TestDockerWorker_Run_ExecError: 退出码=%d, 错误=%v", exitCode, err)
}

// ─── 测试 6: buildCommand 构造验证 ────────────────────────

func TestBuildCommand(t *testing.T) {
	agentBin := createTempAgent(t)
	issuePath := createTempIssue(t)
	workDir := t.TempDir()

	dw := NewDockerWorker("", agentBin)
	args := dw.buildCommand(issuePath, workDir)

	if len(args) < 10 {
		t.Fatalf("命令参数太少: %d", len(args))
	}

	// 验证基本结构
	expected := []string{"run", "--rm", "--network", "host", "--add-host"}
	for _, exp := range expected {
		found := false
		for _, arg := range args {
			if arg == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("命令应包含 %s: %v", exp, args)
		}
	}

	// 验证 volume 挂载（-v 分开传：args 里 "—v" 后跟值）
	var volArgs []string
	for i, arg := range args {
		if arg == "-v" && i+1 < len(args) {
			volArgs = append(volArgs, args[i+1])
		}
	}
	if len(volArgs) < 2 {
		t.Errorf("应有至少 2 个 volume 挂载: %v", volArgs)
	}

	// 验证 --name 带时间戳
	var nameArg string
	for i, arg := range args {
		if arg == "--name" && i+1 < len(args) {
			nameArg = args[i+1]
		}
	}
	if !strings.HasPrefix(nameArg, "zerg-worker-") {
		t.Errorf("容器名应以 zerg-worker- 开头: %s", nameArg)
	}

	// 验证 sh -c 命令
	var shellCmd string
	for i, arg := range args {
		if arg == "-c" && i+1 < len(args) {
			shellCmd = args[i+1]
		}
	}
	if !strings.Contains(shellCmd, "--issue") {
		t.Errorf("sh -c 命令应包含 --issue: %s", shellCmd)
	}
	if !strings.Contains(shellCmd, "--workdir") {
		t.Errorf("sh -c 命令应包含 --workdir: %s", shellCmd)
	}

	t.Logf("✅ TestBuildCommand: 命令=%v", args)
}

// ─── 测试 7: 并发调用（命名冲突） ─────────────────────────

func TestDockerWorker_Run_ConcurrentNaming(t *testing.T) {
	agentBin := createTempAgent(t)
	issuePath := createTempIssue(t)
	workDir := t.TempDir()

	mr := newMockRunner()
	mr.exitCode = 0

	dw := NewDockerWorker("", agentBin)
	dw.SetCmdRunner(mr.run)

	// 并发调用 5 次
	const n = 5
	results := make(chan struct {
		exitCode int
		err      error
	}, n)

	for i := 0; i < n; i++ {
		go func() {
			exitCode, err := dw.Run(issuePath, workDir)
			results <- struct {
				exitCode int
				err      error
			}{exitCode, err}
		}()
	}

	for i := 0; i < n; i++ {
		r := <-results
		if r.err != nil {
			t.Errorf("调用 %d 返回错误: %v", i, r.err)
		}
		if r.exitCode != 0 {
			t.Errorf("调用 %d 期望退出码 0，实际 %d", i, r.exitCode)
		}
	}

	// 验证所有容器名唯一
	names := make(map[string]bool)
	for _, arg := range mr.args {
		if strings.HasPrefix(arg, "zerg-worker-") {
			if names[arg] {
				t.Errorf("容器名重复: %s", arg)
			}
			names[arg] = true
		}
	}

	t.Logf("✅ TestDockerWorker_Run_ConcurrentNaming: %d 次调用，%d 个唯一容器名", n, len(names))
}

// ─── 测试 8: 超时处理 ─────────────────────────────────────

func TestDockerWorker_Timeout(t *testing.T) {
	agentBin := createTempAgent(t)
	issuePath := createTempIssue(t)
	workDir := t.TempDir()

	// mock runner 模拟超时
	mr := newMockRunner()
	mr.err = context.DeadlineExceeded // 模拟超时

	dw := NewDockerWorker("", agentBin)
	dw.timeout = 50 * time.Millisecond // 极短超时
	dw.SetCmdRunner(mr.run)

	_, err := dw.Run(issuePath, workDir)

	if err == nil {
		t.Fatal("期望返回错误（超时）")
	}
	if !strings.Contains(err.Error(), "超时") && !strings.Contains(err.Error(), "DeadlineExceeded") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("错误应包含超时信息: %v", err)
	}
	t.Logf("✅ TestDockerWorker_Timeout: 错误=%v", err)
}

// ─── 测试 9: 命令参数顺序验证 ─────────────────────────────

func TestBuildCommand_Order(t *testing.T) {
	agentBin := createTempAgent(t)
	issuePath := createTempIssue(t)
	workDir := t.TempDir()

	dw := NewDockerWorker("", agentBin)
	args := dw.buildCommand(issuePath, workDir)

	// 验证命令参数顺序:
	// run, --rm, -v agent, -v workdir, --network, host, --add-host, host-gw, --name, name, image, sh, -c, shell
	if args[0] != "run" {
		t.Errorf("args[0] 应为 'run': %s", args[0])
	}
	if args[1] != "--rm" {
		t.Errorf("args[1] 应为 '--rm': %s", args[1])
	}
	// image 应在 --name 之后
	var imageIdx, nameIdx int
	for i, arg := range args {
		if arg == dw.image {
			imageIdx = i
		}
		if arg == "--name" {
			nameIdx = i
		}
	}
	if imageIdx < nameIdx {
		t.Errorf("镜像名应在 --name 之后: imageIdx=%d, nameIdx=%d", imageIdx, nameIdx)
	}

	// sh 和 -c 应在最后
	lastIdx := len(args) - 1
	if args[lastIdx] != dw.image {
		// 检查 sh -c 是否在末尾
		if args[lastIdx-1] != "-c" {
			t.Errorf("最后两个参数应为 'sh', '-c': %v", args[lastIdx-2:])
		}
	}

	t.Logf("✅ TestBuildCommand_Order: args=%v", args)
}

// ─── 测试 10: 默认值验证 ─────────────────────────────────

func TestNewDockerWorker_Defaults(t *testing.T) {
	dw := NewDockerWorker("", "")

	if dw.image != defaultImage {
		t.Errorf("期望默认镜像 %s，实际 %s", defaultImage, dw.image)
	}
	if dw.agentBin != agentBinaryInHost {
		t.Errorf("期望默认 agentBin %s，实际 %s", agentBinaryInHost, dw.agentBin)
	}
	if dw.timeout != defaultTimeout {
		t.Errorf("期望默认超时 %v，实际 %v", defaultTimeout, dw.timeout)
	}
	if dw.network != defaultNetwork {
		t.Errorf("期望默认网络 %s，实际 %s", defaultNetwork, dw.network)
	}

	t.Logf("✅ TestNewDockerWorker_Defaults: image=%s, agentBin=%s, timeout=%v, network=%s",
		dw.image, dw.agentBin, dw.timeout, dw.network)
}

// ─── 测试 11: cmdRunner 注入验证 ──────────────────────────

func TestSetCmdRunner(t *testing.T) {
	called := false
	customRunner := func(ctx context.Context, name string, args ...string) (int, error) {
		called = true
		return 0, nil
	}

	dw := NewDockerWorker("", "")
	dw.SetCmdRunner(customRunner)

	// 直接调用 runCommand
	exitCode, err := dw.runCommand([]string{"run", "--rm", "test"})

	if !called {
		t.Fatal("注入的 cmdRunner 未被调用")
	}
	if exitCode != 0 {
		t.Fatalf("期望退出码 0，实际 %d", exitCode)
	}
	if err != nil {
		t.Fatalf("期望 nil error，实际 %v", err)
	}

	t.Log("✅ TestSetCmdRunner: 注入 runner 正常工作")
}

// ─── 测试 12: 退出码常量 ─────────────────────────────────

func TestExitCodeConstants(t *testing.T) {
	if ExitSuccess != 0 {
		t.Errorf("ExitSuccess 应为 0，实际 %d", ExitSuccess)
	}
	if ExitFailed != 1 {
		t.Errorf("ExitFailed 应为 1，实际 %d", ExitFailed)
	}
	if ExitError != -1 {
		t.Errorf("ExitError 应为 -1，实际 %d", ExitError)
	}

	t.Log("✅ TestExitCodeConstants: 退出码常量正确")
}
