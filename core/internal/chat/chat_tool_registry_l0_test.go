package chat

import "testing"

// 2026-09-11 一致性审计：堵住「工具清单两处真相源」漂移
// ① 提示词 L0 定义表（HermesToolDefs）里的每个工具，都必须在注册表里存在（否则 tool_search/资源库发现不了）
// ② 两边的 L0 名单必须完全相等（否则 L0 集合会各自漂）
func TestToolRegistryMatchesPromptDefs(t *testing.T) {
	inReg := map[string]ChatToolMeta{}
	for _, m := range chatToolRegistry {
		if _, dup := inReg[m.Name]; dup {
			t.Errorf("注册表重复条目: %s", m.Name)
		}
		inReg[m.Name] = m
	}
	promptSet := map[string]bool{}
	for _, d := range HermesToolDefs() {
		promptSet[d.Name] = true
		if _, ok := inReg[d.Name]; !ok {
			t.Errorf("提示词工具 %s 不在注册表（tool_search/资源库发现不了）", d.Name)
		}
	}
	for _, m := range chatToolRegistry {
		if m.L0 && !promptSet[m.Name] {
			t.Errorf("注册表标 L0 的 %s 不在提示词定义表（模型拿不到 schema）", m.Name)
		}
		if !m.L0 && promptSet[m.Name] {
			t.Errorf("提示词常驻工具 %s 在注册表里未标 L0（口径不一致）", m.Name)
		}
	}
	// 计数据此可信：注册表条数 = L0 + deferred
	l0 := 0
	for _, m := range chatToolRegistry {
		if m.L0 {
			l0++
		}
	}
	if l0 != len(promptSet) {
		t.Errorf("L0 数量 %d != 提示词常驻 %d", l0, len(promptSet))
	}
}
