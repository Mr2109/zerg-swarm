package chat

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

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

// TestToolRegistryMatchesDispatch — GAP-20260927-174② 的聚焦判据（候件 · 本单补）。
//
// 背景：此前一致性只对拍**两张表**（注册表 ↔ HermesToolDefs[提示词 L0]），而**执行器
// switch 的 case 名集合**（executeChatToolInner · chat_tool_extra.go）无人对拍 ⇒
// 「执行器里能跑 / 注册表里没有（tool_search 发现不了、资源库不列）」这类漂移静默存在。
// 本测试把**第三面**（源码 → 表）纳入对拍，与 failclosed_test.go 的「源码→表」模板同形。
//
// 双向断言：
//
//	① 前向：注册表 deferred 面（非 L0、且非 agentExtraTools——后 4 个走 agent 层执行）
//	     必须有 dispatch case（否则「发现得到、跑不动」）；
//	② 反向：每个 dispatch case 名必须在注册表里有条目（否则「跑得动、发现不了」）。
//
// ★ 已知会被本测试点名的现状（2026-10-03 现读）：port_services · tool_errors 在
// ChatExtraToolDefs() 与执行器里都有，却不在 chatToolRegistry ⇒ 反向断言判红 2 条。
// 要落绿需**同批**把这两名补进 chatToolRegistry（chat_tool_registry.go，属 GAP-174②
// 的另一落点件），或显式立白名单（不推荐——那正是本账要治的「拿表当证据」）。
func TestToolRegistryMatchesDispatch(t *testing.T) {
	dispatched := dispatchCaseNames(t, "chat_tool_extra.go")

	// ① 前向：注册表里 deferred 且真由 chat 执行器跑的工具，必须有 case
	for _, m := range chatToolRegistry {
		if m.L0 || agentExtraTools[m.Name] {
			continue
		}
		if !dispatched[m.Name] {
			t.Errorf("注册表 deferred 工具 %q 无 dispatch case（tool_search 发现得到、执行器却跑不动）", m.Name)
		}
	}
	// ② 反向：执行器能跑的每个 case，必须在注册表里（可被发现）
	for name := range dispatched {
		if _, ok := chatToolMetaByName[name]; !ok {
			t.Errorf("执行器 case %q 不在注册表（tool_search/资源库发现不了——两表漂移）", name)
		}
	}
}

// dispatchCaseNames — 从 executeChatToolInner 的 switch 源码抽出 case 名集合（源码 → 表对拍）。
// 抽取面**收紧**到该函数体，且只取 `switch name` 的 case（该函数体内两个 switch 都判 name）。
// 抽不到任何 case ⇒ t.Fatalf（防「抽取口径失效」伪装成绿）。
func dispatchCaseNames(t *testing.T, srcFile string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(srcFile)
	if err != nil {
		t.Fatalf("读执行器源码 %s 失败: %v（本测试须在包目录内跑）", srcFile, err)
	}
	src := string(b)
	i := strings.Index(src, "func executeChatToolInner")
	if i < 0 {
		t.Fatalf("%s 找不到 executeChatToolInner（判据面失效）", srcFile)
	}
	body := src[i:]
	if j := strings.Index(src[i+10:], "\nfunc "); j >= 0 {
		body = src[i : i+10+j]
	}
	out := map[string]bool{}
	re := regexp.MustCompile(`(?m)^\tcase\s+(.+?):`)
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		lab := m[1]
		if k := strings.Index(lab, "//"); k >= 0 { // 去掉行尾注释（case "x": // …）
			lab = lab[:k]
		}
		for _, nm := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(lab, -1) {
			out[nm[1]] = true
		}
	}
	if len(out) == 0 {
		t.Fatalf("从 %s 未抽到任何 dispatch case（抽取口径失效 ⇒ 假绿）", srcFile)
	}
	return out
}
