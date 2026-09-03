// contract_test.go — D2 执行接口约定单测（多语言 L3 验收项）
//
// 验收标准（设计稿 §5 L3）：接口约定单测。
// 覆盖：英文枚举值域 / 中文值塞进英文枚举（MLCL 主因）/ ASCII 与 ISO 日期标记 /
//
//	未声明约定放行（防误伤中文路径与中文查询词）/ FFP 文本结构可被循环识别。
package ffp

import (
	"strings"
	"testing"
)

func schemaWith(props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props}
}

// 中文值塞进英文枚举 —— MLCL 主因，必须单列为「参数值语言不匹配」并给最小合法示例
func TestEnumLanguageMismatch(t *testing.T) {
	schema := schemaWith(map[string]any{
		"action": map[string]any{"type": "string", "enum": []any{"create", "update", "list"}},
	})
	vs := CheckContract("todo", schema, map[string]any{"action": "创建"})
	if len(vs) != 1 {
		t.Fatalf("应命中 1 条，实得 %d: %+v", len(vs), vs)
	}
	v := vs[0]
	if v.Rule != RuleLangMismatch {
		t.Errorf("分类应为 %q，实得 %q", RuleLangMismatch, v.Rule)
	}
	if v.Param != "action" || v.Tool != "todo" {
		t.Errorf("定位错误: %s.%s", v.Tool, v.Param)
	}
	if !strings.Contains(v.Expected, "create") {
		t.Errorf("期望里应列出英文枚举值，实得 %q", v.Expected)
	}
	if v.Example != `{"action": "create"}` {
		t.Errorf("最小合法示例应为 {\"action\": \"create\"}，实得 %q", v.Example)
	}

	// FFP 文本：结构完整 + 可被工具循环识别为格式错误（走教学路径）
	text := BuildParamContract(v)
	for _, want := range []string{"参数值语言不匹配: todo.action", "创建", "create", "[最小合法示例]", "[guide]"} {
		if !strings.Contains(text, want) {
			t.Errorf("FFP 文本缺少 %q：\n%s", want, text)
		}
	}
	if !In(text) {
		t.Error("FFP 文本应能被 ffp.In() 识别（否则工具循环不会当格式错误处理）")
	}
}

// 合法枚举值放行
func TestEnumValidPasses(t *testing.T) {
	schema := schemaWith(map[string]any{
		"action": map[string]any{"type": "string", "enum": []any{"create", "update", "list"}},
	})
	for _, v := range []string{"create", "update", "list"} {
		if vs := CheckContract("todo", schema, map[string]any{"action": v}); len(vs) != 0 {
			t.Errorf("合法值 %q 不应命中，实得 %+v", v, vs)
		}
	}
}

// ASCII 拼错 → 枚举值非法（区别于语言不匹配）
func TestEnumAsciiInvalid(t *testing.T) {
	schema := schemaWith(map[string]any{
		"action": map[string]any{"type": "string", "enum": []any{"create", "update", "list"}},
	})
	vs := CheckContract("todo", schema, map[string]any{"action": "craete"})
	if len(vs) != 1 || vs[0].Rule != RuleEnumInvalid {
		t.Fatalf("应为 %q，实得 %+v", RuleEnumInvalid, vs)
	}
}

// 类型不符（非字符串）也拦
func TestEnumWrongType(t *testing.T) {
	schema := schemaWith(map[string]any{
		"action": map[string]any{"type": "string", "enum": []any{"create", "list"}},
	})
	if vs := CheckContract("todo", schema, map[string]any{"action": 123}); len(vs) != 1 {
		t.Fatalf("非字符串值应命中，实得 %+v", vs)
	}
}

// x-zerg-format: ascii / iso-date / id
func TestFormatMarkers(t *testing.T) {
	schema := schemaWith(map[string]any{
		"machine": map[string]any{"type": "string", "x-zerg-format": "ascii"},
		"date":    map[string]any{"type": "string", "x-zerg-format": "iso-date"},
		"run_id":  map[string]any{"type": "string", "x-zerg-format": "id"},
	})
	// ascii：中文机器名 → 语言不匹配
	vs := CheckContract("spawn_agent", schema, map[string]any{"machine": "本机"})
	if len(vs) != 1 || vs[0].Rule != RuleLangMismatch {
		t.Errorf("machine=本机 应命中 %q，实得 %+v", RuleLangMismatch, vs)
	}
	if vs := CheckContract("spawn_agent", schema, map[string]any{"machine": "x3"}); len(vs) != 0 {
		t.Errorf("machine=x3 应放行，实得 %+v", vs)
	}
	// iso-date：口语日期 → 日期格式非法
	vs = CheckContract("t", schema, map[string]any{"date": "今天"})
	if len(vs) != 1 || vs[0].Rule != RuleDateInvalid {
		t.Errorf("date=今天 应命中 %q，实得 %+v", RuleDateInvalid, vs)
	}
	if vs := CheckContract("t", schema, map[string]any{"date": "2026-09-11"}); len(vs) != 0 {
		t.Errorf("date=2026-09-11 应放行，实得 %+v", vs)
	}
	// id：含空格 → 拦；原样 → 放
	if vs := CheckContract("t", schema, map[string]any{"run_id": "abc 12"}); len(vs) != 1 {
		t.Errorf("run_id 含空格应命中，实得 %+v", vs)
	}
	if vs := CheckContract("t", schema, map[string]any{"run_id": "abc-12"}); len(vs) != 0 {
		t.Errorf("run_id=abc-12 应放行，实得 %+v", vs)
	}
}

// 未声明约定的参数一律放行 —— 防"系统自己发明的约定"误伤（中文路径/中文查询词是本项目合法输入）
func TestUndeclaredParamsPass(t *testing.T) {
	schema := schemaWith(map[string]any{
		"path":  map[string]any{"type": "string"},
		"query": map[string]any{"type": "string"},
		"limit": map[string]any{"type": "integer"},
	})
	args := map[string]any{"path": "00 项目/模型类/报告.pdf", "query": "系统状态怎么查", "limit": 20}
	if vs := CheckContract("read", schema, args); len(vs) != 0 {
		t.Errorf("未声明约定的中文值不应命中，实得 %+v", vs)
	}
}

// 缺参数 / 空 schema / nil 入参 → 不拦（参数缺失由既有 FFP 分类负责）
func TestMissingAndNilSafe(t *testing.T) {
	if vs := CheckContract("todo", nil, map[string]any{"action": "创建"}); vs != nil {
		t.Errorf("nil schema 应返回 nil，实得 %+v", vs)
	}
	if vs := CheckContract("todo", schemaWith(map[string]any{"action": map[string]any{"enum": []any{"create"}}}), nil); vs != nil {
		t.Errorf("nil args 应返回 nil，实得 %+v", vs)
	}
	schema := schemaWith(map[string]any{"action": map[string]any{"enum": []any{"create"}}})
	if vs := CheckContract("todo", schema, map[string]any{}); len(vs) != 0 {
		t.Errorf("缺参数不应由本校验拦（交由既有分类），实得 %+v", vs)
	}
}

// 确定性：多条违约定按参数名排序
func TestViolationOrderDeterministic(t *testing.T) {
	schema := schemaWith(map[string]any{
		"zeta":  map[string]any{"enum": []any{"a"}},
		"alpha": map[string]any{"enum": []any{"b"}},
	})
	vs := CheckContract("t", schema, map[string]any{"zeta": "错", "alpha": "错"})
	if len(vs) != 2 {
		t.Fatalf("应命中 2 条，实得 %d", len(vs))
	}
	if vs[0].Param != "alpha" || vs[1].Param != "zeta" {
		t.Errorf("应按参数名排序，实得 %s, %s", vs[0].Param, vs[1].Param)
	}
}
