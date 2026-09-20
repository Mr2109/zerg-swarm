// failclosed_test.go — §二十一 已红第 9 条（T-31「三处 fail-open」）的**成对负控 + 列全棘轮**。
//
// 为什么要有它：fail-open 的病征是「规则写了、电没接」——**不报错、不告警、也不红**。
// 所以判据只能反过来查：默认档必须是拦、三态必须闭集、**每个真在用的 CA 工具名都必须显式在册**
// （最后一个尤其重要：默认 block 之后，「新增一个工具忘了登记」的后果是那个工具当场不可用 ——
// 与其让人肉发现，不如让这个测试红）。
package control

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// repoRoot 从核心包往上找到仓根（认 core/internal/control 这棵树）。
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatalf("找不到仓根（从 %s 往上）", wd)
	return ""
}

// TestRealRulesYaml_DefaultIsBlockAndListsEveryCATool —— 判据①的机检 + 「显式列全」棘轮。
func TestRealRulesYaml_DefaultIsBlockAndListsEveryCATool(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "core", "internal", "control", "rules.yaml")
	g, err := NewGateFromFile(path)
	if err != nil {
		t.Fatalf("装载真规则表：%v", err)
	}
	// ① 默认档 = block（已红第 9 条判据①）
	if got := g.rules.Default; got != ActionBlock {
		t.Errorf("rules.yaml 的默认档必须是 %q（P-099「升强制」），得到 %q", ActionBlock, got)
	}
	// ①′ 成对：拿一个谁也没登记的工具名问一次，必须**拦**
	if d := g.Check("zz_not_registered_tool", "", "zerg"); d.Action != ActionBlock {
		t.Errorf("未登记工具必须被默认档拦下，得到 %q", d.Action)
	}
	// ② 显式列全：扫 core/internal/agent 里所有 `gate.Check("X"` 的 X，逐个要求在册且 action=allow
	used := caToolNames(t, filepath.Join(root, "core", "internal", "agent"))
	if len(used) == 0 {
		t.Fatalf("空转：扫不到任何 gate.Check 调用（判据不可判）")
	}
	for _, name := range used {
		d := g.Check(name, "", "zerg")
		if d.Action != ActionAllow {
			t.Errorf("CA 工具 %q 在 rules.yaml 里没有显式 allow（默认 block 之后它会被拦掉）：action=%s rule=%q",
				name, d.Action, d.Rule)
		}
	}
	t.Logf("CA 工具名 %d 个逐条在册：%v", len(used), used)
	// ③ 通配兜底：MCP 动态名必须被 mcp_* 兜住（精确条目不存在也能过）
	if d := g.Check("mcp_codegraph_query", "", "zerg"); d.Action != ActionAllow {
		t.Errorf("mcp_* 通配没兜住动态工具名：%q", d.Action)
	}
	// ④ 危险参数仍然拦得住（默认档换挡不许把 args_deny 弄丢）
	if d := g.Check("bash", "rm -rf / ", "zerg"); d.Action != ActionBlock {
		t.Errorf("bash 的危险参数必须仍被拦：%q", d.Action)
	}
	if d := g.Check("bash", "git push --force origin main", "zerg"); d.Action != ActionBlock {
		t.Errorf("bash 的 force push 必须仍被拦：%q", d.Action)
	}
	if d := g.Check("write", "path=core/internal/control/rules.yaml", "zerg"); d.Action != ActionBlock {
		t.Errorf("write 命中核心配置必须仍被拦：%q", d.Action)
	}
}

// caToolNames 扫 agent 包里 `gate.Check("<名>"` 的名字（去重排序）。
func caToolNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("扫 CA 包：%v", err)
	}
	re := regexp.MustCompile(`gate\.Check\("([A-Za-z0-9_]+)"`)
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			seen[m[1]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestGate_MissingDefaultIsBlock —— 负控：**规则文件缺 default** ⇒ 拦（不是放行）。
func TestGate_MissingDefaultIsBlock(t *testing.T) {
	g, err := NewGateFromYAML([]byte("version: 1\nrules:\n  tools:\n    - name: \"read\"\n      action: allow\n"))
	if err != nil {
		t.Fatal(err)
	}
	if g.rules.Default != ActionBlock {
		t.Fatalf("缺 default ⇒ 必须落到 block（fail-closed），得到 %q", g.rules.Default)
	}
	if d := g.Check("read", "", "x"); d.Action != ActionAllow {
		t.Errorf("显式 allow 的那条必须仍放行：%q", d.Action)
	}
	if d := g.Check("whatever", "", "x"); d.Action != ActionBlock {
		t.Errorf("未列出的必须拦：%q", d.Action)
	}
}

// TestGate_UnknownActionIsBlock —— 负控：action 拼错 ⇒ 那一行按 block 算。
func TestGate_UnknownActionIsBlock(t *testing.T) {
	g, err := NewGateFromYAML([]byte("version: 1\nrules:\n  default: Block\n  tools:\n    - name: \"read\"\n      action: Allow\n"))
	if err != nil {
		t.Fatal(err)
	}
	if g.rules.Default != ActionBlock {
		t.Errorf("default 拼错（Block 不是闭集值）⇒ 必须落到 block：%q", g.rules.Default)
	}
	if d := g.Check("read", "", "x"); d.Action != ActionBlock {
		t.Errorf("action 拼错（Allow）⇒ 那一行按 block 算：%q", d.Action)
	}
}

// TestMatchRuleName_OnlyTrailingWildcard —— 通配只认尾部一个 `*`（别的位置一律字面量）。
func TestMatchRuleName_OnlyTrailingWildcard(t *testing.T) {
	cases := []struct {
		pattern, tool string
		want          bool
	}{
		{"mcp_*", "mcp_x_y", true},
		{"mcp_*", "mcp_", true},
		{"mcp_*", "xmcp_a", false},
		{"*db*", "db_x", false}, // 多通配位 ⇒ 当字面量（不给暗放行面）
		{"mcp_*_x", "mcp_a_x", false},
		{"read", "read", true},
		{"read", "read_file", false},
	}
	for _, c := range cases {
		if got := matchRuleName(c.pattern, c.tool); got != c.want {
			t.Errorf("matchRuleName(%q,%q)=%v，want %v", c.pattern, c.tool, got, c.want)
		}
	}
}

// TestAgentGate_AdapterThreeStates —— 适配器：三态逐字搬 + 认不出的判定按拦 + 空规则表按拦。
func TestAgentGate_AdapterThreeStates(t *testing.T) {
	g, err := NewGateFromYAML([]byte(`version: 1
rules:
  default: block
  tools:
    - name: "read"
      action: allow
    - name: "terminal"
      action: require_approval
    - name: "edit_rules_yaml"
      action: block
`))
	if err != nil {
		t.Fatal(err)
	}
	ag := NewAgentGate(g)
	cases := []struct{ tool, want string }{
		{"read", string(ActionAllow)},
		{"terminal", string(ActionRequireApproval)},
		{"edit_rules_yaml", string(ActionBlock)},
		{"never_listed", string(ActionBlock)}, // 默认 block 搬过来
	}
	for _, c := range cases {
		d, err := ag.Check(c.tool, "", "zerg")
		if err != nil {
			t.Fatalf("适配器返回错误：%v", err)
		}
		if d.Action != c.want {
			t.Errorf("AgentGate.Check(%q).Action = %q，want %q", c.tool, d.Action, c.want)
		}
	}
	// 成对（口径②）：**没有规则表**时是拦，不是放行
	d, err := NewAgentGate(nil).Check("read", "", "zerg")
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != string(ActionBlock) {
		t.Errorf("规则表未装载时必须拦（fail-closed），得到 %q", d.Action)
	}
	// 接口断言：它真的实现了 agent.ToolGater
	var _ agent.ToolGater = (*AgentGate)(nil)
}
