package control

import "testing"

// testRules 测试用规则
const testRules = `
version: 1
rules:
  default: allow
  tools:
    - name: "patch_fleet_yaml"
      action: block
    - name: "terminal"
      action: require_approval
      args_deny:
        - "rm -rf /"
    - name: "git_commit"
      action: allow
`

func newTestGate(t *testing.T) *Gate {
	t.Helper()
	g, err := NewGateFromYAML([]byte(testRules))
	if err != nil {
		t.Fatalf("规则加载失败: %v", err)
	}
	return g
}

// TestGate_BlockFleetYaml 模型改 fleet.yaml → block（M3 铁律）
func TestGate_BlockFleetYaml(t *testing.T) {
	g := newTestGate(t)
	d := g.Check("patch_fleet_yaml", "", "codex")
	if d.Action != ActionBlock {
		t.Fatalf("期望 block，得到 %s", d.Action)
	}
}

// TestGate_TerminalApproval terminal → require_approval
func TestGate_TerminalApproval(t *testing.T) {
	g := newTestGate(t)
	d := g.Check("terminal", "ls -la", "codex")
	if d.Action != ActionRequireApproval {
		t.Fatalf("期望 require_approval，得到 %s", d.Action)
	}
}

// TestGate_TerminalDenyArgs 危险参数命中 → block
func TestGate_TerminalDenyArgs(t *testing.T) {
	g := newTestGate(t)
	d := g.Check("terminal", "rm -rf /tmp/x", "codex")
	if d.Action != ActionBlock {
		t.Fatalf("期望 block（危险参数），得到 %s", d.Action)
	}
}

// TestGate_AllowGitCommit git_commit → allow
func TestGate_AllowGitCommit(t *testing.T) {
	g := newTestGate(t)
	d := g.Check("git_commit", "", "codex")
	if d.Action != ActionAllow {
		t.Fatalf("期望 allow，得到 %s", d.Action)
	}
}

// TestGate_DefaultAllow 未匹配工具 → 默认 allow
func TestGate_DefaultAllow(t *testing.T) {
	g := newTestGate(t)
	d := g.Check("read_file", "/tmp/x", "codex")
	if d.Action != ActionAllow {
		t.Fatalf("期望默认 allow，得到 %s", d.Action)
	}
}
