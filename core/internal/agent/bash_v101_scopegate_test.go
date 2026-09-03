package agent

// bash_v101_scopegate_test.go — v1.0.2 删除范围门控测试(2026-09-07 Mr2109确认)
// 规则: rm 目标 Clean 解析后必须落在 { 工作区, /tmp, ExtraAllowDirs } 内——出域即拒(与拼写无关)

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBashRmScopeGate_blocksOutOfDomain(t *testing.T) {
	ws := "/work/zerg-ws"
	roots := []string{ws}
	blocked := []string{
		"rm -rf /etc/hosts",               // 域外绝对路径(现状放行——真修复点)
		"rm -rf /opt/zerg-outside/其他项目/x", // 域外绝对路径（跨平台；此前写私有卷路径，导出后被替换成占位符，前提不成立）
		"rm -rf ~/Documents",    // 家目录(家目录守卫已拦——双保险)
		"rm -f /Library/Preferences/x.plist",
		"cd /tmp && rm -rf /etc", // 复合命令
		"rm -rf ../../escape",    // 相对出域(以 execCwd 为基解析后仍在域外)
	}
	for _, c := range blocked {
		if err := bashRmScopeGate(c, ws+"/sub", roots); err == nil {
			t.Errorf("应拒绝(出域): %q", c)
		}
	}
}

func TestBashRmScopeGate_allowsInDomain(t *testing.T) {
	ws := "/work/zerg-ws"
	roots := []string{ws, "/data/task-xyz"}
	allowed := []string{
		"rm -rf /tmp/ok",                      // /tmp 决策域
		"rm -f /tmp/zerg-bash-overflow/x.log", // 溢出目录(/tmp 内)
		"rm -rf ./dist",                       // 工作区相对
		"rm -f notes.md",                      // 裸文件名→以 execCwd(工作区内)为基
		"rm -rf *",                            // 工作区内通配(git 兜底)
		"rm -rf /data/task-xyz/tmp",           // 白名单任务目录
		"ls /etc/hosts",                       // 非 rm 命令不受影响
		"git clean -fd",
	}
	for _, c := range allowed {
		if err := bashRmScopeGate(c, ws, roots); err != nil {
			t.Errorf("不应拒绝(域内): %q → %v", c, err)
		}
	}
}

// e2e: 拒绝发生在执行前(不会真删 /etc),允许的真实执行
func TestBashRmScopeGate_e2e(t *testing.T) {
	ec := testBashEC(t)
	// 拒绝: /etc 目标 → 报错且不执行
	_, err := ec.executeBashV101(context.Background(), "rm -rf /etc/nonexistent-xyz", "", 0, nil)
	if err == nil || !strings.Contains(err.Error(), "允许删除域") {
		t.Fatalf("域外 rm 未拒: %v", err)
	}
	// 允许: /tmp 下文件真实删除(注意: macOS os.TempDir()=/var/folders 非 /tmp——门控认字面 /tmp)
	f := "/tmp/zerg-scopegate-ok-20260907"
	os.WriteFile(f, []byte("x"), 0o644)
	defer os.Remove(f)
	out, err := ec.executeBashV101(context.Background(), "rm -f "+f, "", 0, nil)
	if err != nil {
		t.Fatalf("/tmp 删除被误拒: %v", err)
	}
	if _, statErr := os.Stat(f); !os.IsNotExist(statErr) {
		t.Fatalf("文件未被真实删除: %s(out=%s)", f, out)
	}
	// 允许: 工作区内相对删除
	wf := filepath.Join(ec.WorkDir, "scopegate-rel.txt")
	os.WriteFile(wf, []byte("y"), 0o644)
	if _, err := ec.executeBashV101(context.Background(), "rm -f scopegate-rel.txt", "", 0, nil); err != nil {
		t.Fatalf("工作区删除被误拒: %v", err)
	}
}
