package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── G6：Go 工具链钉住 ────────────────────────────────────────────────────────

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestToolchainPin_PrefersToolchainDirective(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "core", "go.mod"), "module x\n\ngo 1.23\n\ntoolchain go1.25.5\n")
	if got := ToolchainPin(root); got != "go1.25.5" {
		t.Fatalf("期望 go1.25.5，得 %q", got)
	}
}

func TestToolchainPin_FallsBackToGoDirective(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "core", "go.mod"), "module x\n\ngo 1.24.3\n")
	if got := ToolchainPin(root); got != "go1.24.3" {
		t.Fatalf("期望 go1.24.3，得 %q", got)
	}
}

func TestToolchainPin_DefaultWhenNoGoMod(t *testing.T) {
	if got := ToolchainPin(t.TempDir()); got != goToolchainDefaultPin {
		t.Fatalf("期望默认 %q，得 %q", goToolchainDefaultPin, got)
	}
}

func TestToolchainPin_EnvOverride(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "core", "go.mod"), "module x\n\ntoolchain go1.25.5\n")
	t.Setenv("ZERG_GO_TOOLCHAIN_PIN", "go9.9.9")
	if got := ToolchainPin(root); got != "go9.9.9" {
		t.Fatalf("环境覆盖应生效，得 %q", got)
	}
}

// 语义比较：字符串比较会把 go1.9 判成大于 go1.26（这正是要防的坑）。
func TestCompareGoVersions_SemanticNotLexical(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"go1.9", "go1.26", -1},
		{"go1.26.4", "go1.25.5", 1},
		{"go1.25.5", "go1.25.5", 0},
		{"go1.25", "go1.25.0", 0},
		{"go2.0", "go1.99.99", 1},
		{"garbage", "go1.25", 2},
	}
	for _, c := range cases {
		if got := CompareGoVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareGoVersions(%q,%q) = %d，期望 %d", c.a, c.b, got, c.want)
		}
	}
}

func TestVerifyToolchain_LocalSatisfiesPin(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "core", "go.mod"), "module x\n\ntoolchain go1.1.1\n")
	pin, actual, err := VerifyToolchain(root)
	if err != nil {
		t.Fatalf("本机工具链应满足 go1.1.1，得错误：%v", err)
	}
	if pin != "go1.1.1" || actual == "" {
		t.Fatalf("pin=%q actual=%q", pin, actual)
	}
}

func TestVerifyToolchain_RefusesWhenPinNewer(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "core", "go.mod"), "module x\n\ntoolchain go99.0.0\n")
	if _, _, err := VerifyToolchain(root); err == nil {
		t.Fatal("钉住值高于本机时必须报错（否则「钉住」形同虚设）")
	}
}

// ── 组件集（B5：机群节点的件与主控机不同）──────────────────────────────────

func TestNormalizeComponents_DefaultAndNode(t *testing.T) {
	def, err := NormalizeComponents(nil)
	if err != nil {
		t.Fatal(err)
	}
	if def[0] != CompCore || def[1] != CompAgent {
		t.Fatalf("默认组件应为 core,agent…… 得 %v", def)
	}
	node, err := NormalizeComponents(NodeComponents())
	if err != nil {
		t.Fatal(err)
	}
	if len(node) != 3 || node[0] != CompCore || node[1] != CompAgentd || node[2] != CompWall {
		t.Fatalf("节点组件应为 core,agentd，得 %v", node)
	}
	for _, c := range node {
		if c == CompUI {
			t.Fatal("节点组件不得含 ui（UI 仅 Mac 编译）")
		}
	}
}

func TestNormalizeComponents_UnknownRejected(t *testing.T) {
	if _, err := NormalizeComponents([]string{"core,nonsense"}); err == nil {
		t.Fatal("未知组件必须报错（不静默忽略）")
	}
}

func TestNormalizeComponents_UIOnlyOnDarwin(t *testing.T) {
	got, err := NormalizeComponents([]string{"core, ui ,core"})
	if runtime.GOOS == "darwin" {
		if err != nil {
			t.Fatalf("darwin 上 ui 应可用：%v", err)
		}
		if len(got) != 2 || got[1] != CompUI {
			t.Fatalf("应去重保序得 [core ui]，得 %v", got)
		}
	} else if err == nil {
		t.Fatal("非 darwin 上 ui 必须报错（UI 仅 Mac）")
	}
}

func TestComponentsForRole(t *testing.T) {
	c, err := componentsForRole(RoleNode, nil)
	if err != nil || len(c) != 3 || c[1] != CompAgentd || c[2] != CompWall {
		t.Fatalf("node 角色应得 core,agentd：%v %v", c, err)
	}
	c, err = componentsForRole(RoleController, nil)
	if err != nil || c[0] != CompCore {
		t.Fatalf("controller 角色应得默认集：%v %v", c, err)
	}
	if _, err := componentsForRole("nope", nil); err == nil {
		t.Fatal("未知角色必须报错")
	}
	c, err = componentsForRole(RoleNode, []string{"core"})
	if err != nil || len(c) != 1 || c[0] != CompCore {
		t.Fatalf("显式组件应优先于角色：%v %v", c, err)
	}
}

// ── Build：工具链不满足 ⇒ 拒绝；同输入两次 ⇒ 同 sha256（G6 断言）────────────

// toyTree 造一棵最小可构建的树（core 模块 + version 真源 + cmd/zerg-core）。
func toyTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "core", "go.mod"), "module toy/core\n\ngo 1.21\n")
	writeFile(t, filepath.Join(root, "core", "internal", "version", "version.go"),
		"package version\n\nconst Version = \"0.0.9\"\n\nvar Commit = \"unknown\"\nvar BuildTime = \"unknown\"\n\nfunc Line() string { return Version + \" \" + Commit + \" \" + BuildTime }\n")
	writeFile(t, filepath.Join(root, "core", "cmd", "zerg-core", "main.go"),
		"package main\n\nimport (\n\t\"fmt\"\n\n\t\"toy/core/internal/version\"\n)\n\nfunc main() { fmt.Println(\"toy-core\", version.Line()) }\n")
	return root
}

func TestBuild_RefusesWhenToolchainBelowPin(t *testing.T) {
	root := toyTree(t)
	t.Setenv("ZERG_GO_TOOLCHAIN_PIN", "go99.0.0")
	_, err := Build(BuildOptions{Source: root, Staging: t.TempDir(), Commit: "abc", Components: []string{CompCore}})
	if err == nil {
		t.Fatal("工具链低于钉住值时必须拒绝构建（不装身份可疑的件）")
	}
}

// agentd 属独立模块：目标树没有 agent/go.mod 时必须**明确报错**，不得静默跳过
// （2026-09-13 沙箱实测踩到：判断写成「不是目录就报错」，把正常文件也判成缺失 ⇒ 节点永远编不出 agentd）。
func TestBuild_AgentdRequiresAgentModule(t *testing.T) {
	root := toyTree(t) // 只有 core 模块
	_, err := Build(BuildOptions{Source: root, Staging: t.TempDir(), Commit: "abc", Components: []string{CompAgentd}})
	if err == nil {
		t.Fatal("目标树缺 agent 模块时必须报错")
	}
	if !strings.Contains(err.Error(), "agent 模块") {
		t.Fatalf("报错应点明缺 agent 模块，得：%v", err)
	}
}

func TestBuild_DeterministicSameInputsSameSHA(t *testing.T) {
	root := toyTree(t)
	commit, bt := "cafebabe", "2026-09-13T00:00:00Z"
	sums := []string{}
	for i := 0; i < 2; i++ {
		st := t.TempDir()
		res, err := Build(BuildOptions{Source: root, Staging: st, Commit: commit,
			Components: []string{CompCore}, BuildTime: bt})
		if err != nil {
			t.Fatalf("第 %d 次构建失败：%v", i+1, err)
		}
		if res.Toolchain.Pin == "" || res.Toolchain.Actual == "" || res.Toolchain.Policy != "local" {
			t.Fatalf("manifest 必须记工具链 pin/actual/policy，得 %+v", res.Toolchain)
		}
		sums = append(sums, res.Artifacts[0].SHA256)
		b, err := os.ReadFile(filepath.Join(st, "manifest.json"))
		if err != nil || len(b) == 0 {
			t.Fatalf("manifest.json 未写：%v", err)
		}
	}
	if sums[0] != sums[1] {
		t.Fatalf("同 commit + 同平台 + 同工具链 + 同构建时间应逐字节相同：%s vs %s", sums[0], sums[1])
	}
	// 反向：换 commit ⇒ 必须不同（否则这条断言是空的）
	st := t.TempDir()
	res, err := Build(BuildOptions{Source: root, Staging: st, Commit: "deadbeef",
		Components: []string{CompCore}, BuildTime: bt})
	if err != nil {
		t.Fatal(err)
	}
	if res.Artifacts[0].SHA256 == sums[0] {
		t.Fatal("换 commit 后 sha256 未变——注入身份失效")
	}
	h := sha256.Sum256([]byte(sums[0]))
	if hex.EncodeToString(h[:]) == "" {
		t.Fatal("unreachable")
	}
}
