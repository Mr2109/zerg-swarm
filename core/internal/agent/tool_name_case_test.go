package agent

import (
	"strings"
	"testing"
)

// 事故背景（2026-09-17 实测）：模型发 `Read`（大写）⇒ 区分大小写匹配 ⇒ 报未知工具 ⇒ 白耗一轮。
// 规则（Mr2109 拍）：行为宽容（大小写不敏感 + 去空白）、契约严格（执行用注册表真名）、歧义零容忍。
func TestResolveToolNameCaseInsensitive(t *testing.T) {
	cases := []struct{ in, want string }{
		{"read", "read"},
		{"Read", "read"},
		{"READ", "read"},
		{"  read  ", "read"},
		{"Grep", "grep"},
	}
	for _, c := range cases {
		got, err := resolveToolName(c.in)
		if err != nil {
			t.Errorf("%q 应解析成功，却报错：%v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q ⇒ 期望 %q，得到 %q", c.in, c.want, got)
		}
	}
}

func TestResolveToolNameUnknownIsTeaching(t *testing.T) {
	_, err := resolveToolName("nope_tool_xyz")
	if err == nil {
		t.Fatal("未知名应报错")
	}
	msg := err.Error()
	// 教学式：必须能行动 —— 含可用工具名 + 正确示例
	if !strings.Contains(msg, "read") || !strings.Contains(msg, "arguments") {
		t.Errorf("错误信息缺可行动内容（可用工具/正确示例）：%s", msg)
	}
}
