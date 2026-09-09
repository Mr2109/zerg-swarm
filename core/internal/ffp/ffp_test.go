package ffp

import (
	"strings"
	"testing"
)

// TestBuild — 实测样本(2026-09-08 bash 空 command): 结构完整性 + 教学要素齐全
func TestBuild(t *testing.T) {
	msg := Build("参数缺失: bash.command", `{"command":""}`, "字段 command:string 非空(trim 后)",
		`{"command": "ls -la"}`, "命令字段不能为空——先想好要执行什么再调用")
	// 首行系统断言
	if !Is(msg) {
		t.Fatalf("Build 结果应以 %q 开头——got:\n%s", Prefix, msg)
	}
	// 四要素齐全(教学文本: 分类/回显/期望/示例/引导)
	for _, want := range []string{
		"[格式分类] 参数缺失: bash.command",
		"[你上一条发送] " + `{"command":""}`,
		"[parser 期望] 字段 command:string 非空(trim 后)",
		"[最小合法示例] " + `{"command": "ls -la"}`,
		"[guide] 命令字段不能为空",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("FFP 缺要素 %q——完整:\n%s", want, msg)
		}
	}
}

// TestIs — 直通判定(loop 层: FFP 不包装成"执行失败")
func TestIs(t *testing.T) {
	if !Is(Build("参数缺失", "x", "y", "z", "w")) {
		t.Error("Build 产物应被 Is 识别")
	}
	if Is("bash 执行失败: 命令参数为空") {
		t.Error("旧式干错误文本不应被识别为 FFP")
	}
	if Is("") {
		t.Error("空串不识别")
	}
}

// TestEcho — 回显窗口: 短原文原样; 超长截两端+省略标注
func TestEcho(t *testing.T) {
	if Echo(`{"command":"ls"}`, 200) != `{"command":"ls"}` {
		t.Error("短原文应原样回显")
	}
	long := strings.Repeat("x", 1000)
	out := Echo(long, 100)
	if len(out) >= 500 {
		t.Errorf("超长应截断——out len=%d", len(out))
	}
	if !strings.Contains(out, "[省略 800 字符]") {
		t.Errorf("超长应带省略标注——got: %s", out)
	}
	if !strings.HasPrefix(out, strings.Repeat("x", 100)) || !strings.HasSuffix(out, strings.Repeat("x", 100)) {
		t.Error("应保留头尾窗口")
	}
	if Echo("", 200) != "<空/缺失>" {
		t.Error("空原文应标注 <空/缺失>")
	}
}
