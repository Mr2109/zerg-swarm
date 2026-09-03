// tool_conventions_test.go — D2「执行接口约定进 schema」自证 + 执行前断言端到端（多语言 L3 验收）
//
// 两道：
//
//	① schema 侧：可枚举参数必须声明 enum（值域即协议——不给模型猜的余地）
//	② 执行侧：中文值塞进英文枚举 → 执行前被拦、回 FFP 教学、不执行；合法值不被误伤
package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/ffp"
)

// ① schema 侧：关键工具的可枚举参数必须声明 enum
func TestSchemaDeclaresEnums(t *testing.T) {
	want := map[string][]string{
		"todo":        {"action", "status"},
		"spawn_agent": {"type"},
		"web_search":  {"lang", "time_range"},
		"read":        {"format"},
		"write":       {"format", "line_end"},
		"ls":          {"sort_by"},
	}
	for tool, params := range want {
		def, ok := ToolDefOf(tool)
		if !ok {
			t.Errorf("工具 %s 不存在（枚举声明的载体丢了）", tool)
			continue
		}
		props, _ := def.Function.Parameters["properties"].(map[string]any)
		if props == nil {
			t.Errorf("%s: 无 properties", tool)
			continue
		}
		for _, p := range params {
			spec, _ := props[p].(map[string]any)
			if spec == nil {
				t.Errorf("%s.%s 无 schema", tool, p)
				continue
			}
			ev, hasEnum := spec["enum"].([]any)
			if !hasEnum || len(ev) == 0 {
				t.Errorf("%s.%s 未声明 enum（值域即协议——D2）", tool, p)
				continue
			}
			// 枚举值必须是英文小写 ASCII（否则"英文枚举"约定自相矛盾）
			for _, e := range ev {
				s := e.(string)
				if s != strings.ToLower(s) || !isASCII(s) {
					t.Errorf("%s.%s 枚举值 %q 非英文小写 ASCII", tool, p, s)
				}
			}
		}
	}
}

// ② 执行侧：中文值塞进英文枚举 → 拦下 + FFP 教学 + 不执行
func TestExecuteToolBlocksLanguageMismatch(t *testing.T) {
	ec := &ExecContext{}
	res := ec.ExecuteTool(context.Background(), "todo", map[string]any{"action": "创建"}, nil)
	if res.Error == "" {
		t.Fatal("中文值 action=创建 应被拦下（不执行），实得空错误")
	}
	for _, want := range []string{
		ffp.RuleLangMismatch,            // 分类单列
		"todo.action",                   // 定位
		"create",                        // 期望里列出英文枚举
		`{"action": "create"}`,          // 最小合法示例
		"[格式分类]", "[最小合法示例]", "[guide]", // FFP 文本结构
	} {
		if !strings.Contains(res.Error, want) {
			t.Errorf("FFP 教学文本缺 %q：\n%s", want, res.Error)
		}
	}
	if !ffp.In(res.Error) {
		t.Error("应能被 ffp.In() 识别（否则工具循环不会按格式错误处理、可能当执行失败重试）")
	}
}

// 合法值不得被误拦（防呆不能变成防正常）
func TestExecuteToolAllowsValidEnum(t *testing.T) {
	ec := &ExecContext{}
	res := ec.ExecuteTool(context.Background(), "todo", map[string]any{"action": "list"}, nil)
	if strings.Contains(res.Error, ffp.RuleLangMismatch) || strings.Contains(res.Error, ffp.RuleEnumInvalid) {
		t.Errorf("合法枚举值 action=list 被误拦：%s", res.Error)
	}
}

// 中文路径 / 中文查询词不得被误拦（未声明约定的参数一律放行）
func TestExecuteToolAllowsChinesePathAndQuery(t *testing.T) {
	ec := &ExecContext{}
	for _, c := range []struct {
		tool string
		args map[string]any
	}{
		{"read", map[string]any{"path": "00 项目/模型类/报告.md"}},
		{"glob", map[string]any{"pattern": "**/设计*.md"}},
	} {
		res := ec.ExecuteTool(context.Background(), c.tool, c.args, nil)
		if strings.Contains(res.Error, ffp.RuleLangMismatch) || strings.Contains(res.Error, ffp.RuleEnumInvalid) {
			t.Errorf("%s 的中文参数被误拦：%s", c.tool, res.Error)
		}
	}
}

// spawn_agent.machine 声明了 x-zerg-format: ascii → 中文机器名被拦
func TestExecuteToolBlocksNonAsciiMachine(t *testing.T) {
	ec := &ExecContext{}
	res := ec.ExecuteTool(context.Background(), "spawn_agent", map[string]any{
		"prompt": "测试", "machine": "本机",
	}, nil)
	if !strings.Contains(res.Error, ffp.RuleLangMismatch) {
		t.Errorf("machine=本机 应触发 %q（x-zerg-format: ascii），实得：%s", ffp.RuleLangMismatch, res.Error)
	}
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}
