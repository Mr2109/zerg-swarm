package agent

import (
	"context"
	"strings"
	"testing"
)

// TestBashEmptyCommandFFP — 验收清单第 1 条(实测样本): bash 空 command →
// FFP 教学文本(断言首行 + 分类点名 + 最小合法示例)——不再返回干错误"命令参数为空"
func TestBashEmptyCommandFFP(t *testing.T) {
	ec := &ExecContext{}
	cases := []struct {
		name string
		args map[string]any
	}{
		{"缺失 command", map[string]any{}},
		{"空串 command", map[string]any{"command": ""}},
		{"空白串 command", map[string]any{"command": "   "}},
		{"类型错 command", map[string]any{"command": 123}},
	}
	for _, c := range cases {
		res := ec.ExecuteTool(context.Background(), "bash", c.args, nil)
		if res.Error == "" {
			t.Errorf("%s: 应返回 Error(FFP)——got 空", c.name)
			continue
		}
		if !strings.Contains(res.Error, "⚠️ 工具调用格式错误(系统断言——非执行失败") {
			t.Errorf("%s: 缺系统断言首行——got: %s", c.name, res.Error)
		}
		for _, want := range []string{"[格式分类]", "[你上一条发送]", "[parser 期望]", "[最小合法示例]", `{"command": "ls -la"}`} {
			if !strings.Contains(res.Error, want) {
				t.Errorf("%s: 缺 %q——got:\n%s", c.name, want, res.Error)
			}
		}
	}
	// 旧式干错误文本必须消失(死循环根因)——取最后一次结果验证
	last := ""
	for _, c := range cases {
		res := ec.ExecuteTool(context.Background(), "bash", c.args, nil)
		last = res.Error
	}
	if strings.Contains(last, "命令参数为空") && !strings.Contains(last, "⚠️") {
		t.Error("不应再出现旧式干错误文本")
	}
}
