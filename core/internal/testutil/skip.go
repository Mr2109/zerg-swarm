// Package testutil — 测试环境探测助手（2026-09-11，CI 适配）
//
// 为什么需要：一批测试隐含"本机才有的东西"——私有路径、macOS 专有命令（textutil）、
// 未安装的外部工具（pdftotext）、需要网络/外部服务的桥（searx 桥）。
// 这些在公开快照的 Linux CI 上都不具备，硬断言会让整条流水线变红。
//
// 纪律：**先探测可用性，不可用则 t.Skip 并写明原因**；本机/私有仓仍照常执行原断言（不丢覆盖）。
package testutil

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// RequireTool 要求外部命令在 PATH 上，否则跳过。
func RequireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("缺少外部命令 %s（当前环境不具备）——跳过", name)
	}
}

// RequireDarwinTool 要求"macOS 专有命令"（如 textutil），非 darwin 平台直接跳过。
func RequireDarwinTool(t *testing.T, name string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skipf("%s 是 macOS 专有命令（当前 %s）——跳过", name, runtime.GOOS)
	}
	RequireTool(t, name)
}

// RequireEnv 用于"需要外部服务/网络"的集成测试：只有显式设置该环境变量才运行。
// CI 默认不设 → 跳过；本机想跑时 `ZERG_TEST_NET=1 go test ./...`。
func RequireEnv(t *testing.T, env, why string) {
	t.Helper()
	if os.Getenv(env) == "" {
		t.Skipf("需要外部依赖（%s）——设 %s=1 才运行；默认跳过", why, env)
	}
}
