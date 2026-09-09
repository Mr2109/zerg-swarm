package agent

// bash_v101_homeguard_test.go — rm 家目录防护测试（2026-09-07 事故修复验收）
// 事故: rm -rf ~/Desktop / $HOME/... / ~/... 从 endOnly 黑名单漏过 → 清空主目录
// 规则: 展开后 rm 目标 ∈ { /, 家目录, 家目录祖先, 家目录内 } → 拒绝; /tmp/xxx 等放行

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func homeOf(t *testing.T) string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		t.Skip("无家目录可测")
	}
	return h
}

func TestBashRmTargetGuard_blocksHomeDeletions(t *testing.T) {
	home := homeOf(t)
	// 事故形态全集——全部必须拦
	blocked := []string{
		"rm -rf ~/Desktop",
		"rm -rf ~/Desktop ~/Music ~/go",
		"rm -rf ~",  // 裸 ~(旧黑名单拦——展开后守卫仍拦)
		"rm -rf ~/", // 尾斜杠
		"rm -rf $HOME/xxx",
		"rm -rf ${HOME}/Documents/x",
		"rm -rf ~/Desktop/*", // 家目录绝对路径(本机)
		"sudo rm -rf ~/Movies",
		"cd /tmp && rm -rf ~/go",
		"rm -rf " + home,                // 直接展开形态
		"rm -rf " + home + "/Documents", // 家目录内子路径
		"rm -rf /Users",                 // 家目录祖先
		"rm -rf /",                      // 根
		"rm -rf '~/Pictures'",           // 引号包裹
		"rm -rf \"$HOME/Downloads\"",    // 引号+变量
	}
	for _, c := range blocked {
		expanded := strings.ReplaceAll(strings.ReplaceAll(c, "$HOME", home), "~/", home+"/")
		expanded = strings.ReplaceAll(expanded, "${HOME}", home)
		expanded = strings.ReplaceAll(expanded, "~", home)
		if strings.HasSuffix(expanded, "~") {
			expanded = expanded[:len(expanded)-1] + home
		}
		if err := bashRmTargetGuard(expanded); err == nil {
			t.Errorf("应拦截但放行: %q", c)
		}
	}
}

func TestBashRmTargetGuard_allowsSafeDeletions(t *testing.T) {
	home := homeOf(t)
	allowed := []string{
		"rm -rf /tmp/ok", // endOnly 设计意图——必须放行
		"rm /tmp/a.txt",
		"rm -f ./dist/output.bin",        // 工作区相对路径
		"rm -rf $WORKSPACE/tmp",          // 未知名变量→字面量,非家目录
		"ls -la " + home,                 // 非 rm 命令不拦
		"mkdir -p " + home + "/.cache/x", // 非 rm 命令不拦
		"git clean -fd",                  // git 自带清理(工作区内)
		"rm -rf",                         // 无操作数——bash 自报错,不拦
	}
	for _, c := range allowed {
		expanded := strings.ReplaceAll(c, "$WORKSPACE", "/Volumes/work")
		if err := bashRmTargetGuard(expanded); err != nil {
			t.Errorf("不应拦截: %q → %v", c, err)
		}
	}
	// 引用自身文件路径(非家目录)也必须放行
	self := filepath.Join(os.TempDir(), "zerg-homeguard-selfcheck")
	os.WriteFile(self, []byte("x"), 0o644)
	defer os.Remove(self)
	if err := bashRmTargetGuard("rm -f " + self); err != nil {
		t.Errorf("放行 /tmp 文件应通过: %v", err)
	}
}
