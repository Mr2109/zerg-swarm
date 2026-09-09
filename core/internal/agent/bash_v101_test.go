package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bash v1.0.1 测试（2026-09-06——AI 专用执行契约）

func testBashEC(t *testing.T) *ExecContext {
	t.Helper()
	wd := t.TempDir()
	return NewExecContext(wd)
}

func runBash101(t *testing.T, cmd string) string {
	t.Helper()
	ec := testBashEC(t)
	out, err := ec.executeBashV101(context.Background(), cmd, "", 0, nil)
	if err != nil {
		return "ERR: " + err.Error()
	}
	return out
}

// 1. 成功命令 → [exit_code] 0
func TestBashV101Success(t *testing.T) {
	out := runBash101(t, "echo hello-bash101")
	if !strings.Contains(out, "hello-bash101") {
		t.Fatalf("stdout 缺失: %s", out)
	}
	if !strings.Contains(out, "[exit_code] 0") {
		t.Fatalf("exit_code 0 缺失: %s", out)
	}
	if !strings.Contains(out, "[workdir]") || !strings.Contains(out, "[duration_ms]") {
		t.Fatalf("元数据缺失: %s", out)
	}
}

// 2. 失败命令 → 首行 ⚠️ 断言 + guide
func TestBashV101FailureAssertion(t *testing.T) {
	out := runBash101(t, "ls /nonexistent-xyz-20260906")
	if !strings.Contains(out, "⚠️ exit") {
		t.Fatalf("首行失败断言缺失: %s", out)
	}
	if !strings.Contains(out, "[exit_code]") {
		t.Fatalf("exit_code 缺失: %s", out)
	}
}

// 3. command not found → 错误分类引导
func TestBashV101CommandNotFoundGuide(t *testing.T) {
	out := runBash101(t, "nonexistent-cmd-xyz-2026")
	if !strings.Contains(out, "command not found") {
		t.Fatalf("分类未命中: %s", out)
	}
	if !strings.Contains(out, "[guide]") {
		t.Fatalf("guide 引导缺失: %s", out)
	}
}

// 4. 长输出 → 溢出落盘（省略标注含路径）
func TestBashV101Overflow(t *testing.T) {
	out := runBash101(t, "seq 1 50000 | head -50000")
	// seq 输出 ~255K——必然溢出
	if !strings.Contains(out, "中间省略") {
		t.Fatalf("溢出省略标注缺失（输出长=%d）: %s", len(out), out[:minInt(len(out), 200)])
	}
	if !strings.Contains(out, BashOverflowDir) {
		t.Fatalf("溢出落盘路径缺失: %s", out[:minInt(len(out), 300)])
	}
	// 落盘文件真实存在且可读
	files, _ := filepath.Glob(filepath.Join(BashOverflowDir, "bash-overflow-*.log"))
	if len(files) == 0 {
		t.Fatalf("溢出文件未落盘")
	}
}

// 5. 二进制输出 → binary_output 防护
func TestBashV101BinaryGuard(t *testing.T) {
	// /bin/ls 自身是二进制（Mac 上 Mach-O）——cat 它
	out := runBash101(t, "cat /bin/ls")
	if !strings.Contains(out, "binary_output") {
		// 若 cat 被 RTK 压缩或系统拦截则跳过——只在真实二进制场景断言
		if strings.Contains(out, "[exit_code]") && strings.Contains(out, "⚠️") {
			t.Skip("系统拦截——非二进制场景")
		}
		t.Fatalf("二进制防护缺失: %s", out[:minInt(len(out), 200)])
	}
}

// 6. 后台 & → poka-yoke 拦截
func TestBashV101BackgroundBlock(t *testing.T) {
	out := runBash101(t, "sleep 999 &")
	if !strings.Contains(out, "后台") {
		t.Fatalf("后台未拦截: %s", out)
	}
}

// 7. 交互命令 → 拦截引导
func TestBashV101InteractiveBlock(t *testing.T) {
	out := runBash101(t, "vim")
	if !strings.Contains(out, "交互") {
		t.Fatalf("交互命令未拦截: %s", out)
	}
	// python3 带脚本合法
	ec := testBashEC(t)
	_, err := ec.executeBashV101(context.Background(), "python3 --version", "", 0, nil)
	if err != nil {
		t.Fatalf("python3 --version 被误拦: %v", err)
	}
}

// 8. 引号未闭合 → 拦截
func TestBashV101UnbalancedQuote(t *testing.T) {
	out := runBash101(t, `echo "unclosed`)
	if !strings.Contains(out, "引号") {
		t.Fatalf("引号未闭合未拦截: %s", out)
	}
}

// 9. 危险命令（原文）→ 拦截
func TestBashV101Dangerous(t *testing.T) {
	out := runBash101(t, "rm -rf ~")
	if !strings.Contains(out, "危险") {
		t.Fatalf("rm -rf ~ 未拦截: %s", out)
	}
}

// 10. 危险命令（变量展开绕过）→ 拦截
func TestBashV101DangerousExpanded(t *testing.T) {
	out := runBash101(t, "rm -rf $HOME")
	if !strings.Contains(out, "危险") {
		t.Fatalf("rm -rf $HOME 未拦: %s", out)
	}
}

// 11. 解码执行链绕过 → 拦截
func TestBashV101Base64Bypass(t *testing.T) {
	out := runBash101(t, "echo cm0gLXJmIC8= | base64 -d | sh")
	if !strings.Contains(out, "绕过") && !strings.Contains(out, "危险") {
		t.Fatalf("base64 解码执行链未拦截: %s", out)
	}
}

// 12. cwd 参数（合法/逃逸）
func TestBashV101Cwd(t *testing.T) {
	ec := testBashEC(t)
	// 建子目录
	sub := filepath.Join(ec.WorkDir, "sub")
	os.MkdirAll(sub, 0o755)
	out, err := ec.executeBashV101(context.Background(), "pwd", "sub", 0, nil)
	if err != nil {
		t.Fatalf("cwd 合法场景失败: %v", err)
	}
	if !strings.Contains(out, "/sub") {
		t.Fatalf("cwd 未生效: %s", out)
	}
	// 逃逸 → 拒绝
	_, err = ec.executeBashV101(context.Background(), "pwd", "/etc", 0, nil)
	if err == nil || !strings.Contains(err.Error(), "不在工作区") {
		t.Fatalf("cwd 逃逸未拒绝: %v", err)
	}
	// 不存在目录 → guide
	_, err = ec.executeBashV101(context.Background(), "pwd", "no-such-dir", 0, nil)
	if err == nil || !strings.Contains(err.Error(), "[guide]") {
		t.Fatalf("cwd 不存在未给 guide: %v", err)
	}
}

// 13. timeout_s 参数生效
func TestBashV101Timeout(t *testing.T) {
	ec := testBashEC(t)
	start := time.Now()
	out, err := ec.executeBashV101(context.Background(), "sleep 10", "", 1, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("timeout 场景报错(应返回文本): %v", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("timeout_s 未生效——跑了 %v", elapsed)
	}
	if !strings.Contains(out, "timeout") {
		t.Fatalf("timeout 标注缺失: %s", out)
	}
}

// 14. cd/export → state_note
func TestBashV101StateNote(t *testing.T) {
	out := runBash101(t, "cd /tmp && pwd")
	if !strings.Contains(out, "state_note") {
		t.Fatalf("cd state_note 缺失: %s", out)
	}
}
