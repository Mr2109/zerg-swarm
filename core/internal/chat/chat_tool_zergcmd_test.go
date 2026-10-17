// chat_tool_zergcmd_test.go — GAP-20261017-09 的验收判据（对话工具面只剩虫族命令）。
//
// 用户口径（Mr2109）：「虫族对话 基于虫族 CLI 干活，只能使用虫族命令」。
// 实测铁证：会话 chat_20261017_144221_647a5a —— 模型明知 `fs run` 拒 ls，仍转手用 bash 工具
// （原话「The fs run whitelist doesn't allow ls. … But I have bash tool directly.」）⇒ 必须从工具面撤掉。
//
// 本件五面（与任务书的五条验收一一对应）：
//
//	① 工具清单（chat 参数 ＋ Hermes <tools> 段 ＋ L0 集合）里没有 bash/read/write/edit/glob/grep/ls/tool_search/kb_search
//	② zerg 工具能真跑：`zerg version` ⇒ rc=0 且 stdout 含版本行
//	③ 带管道/串联/命令替换的串 ⇒ rc=2 拒执
//	④ 首词非 zerg ⇒ rc=2 拒执
//	⑤ 撤下工具兑旧名字直接调 ⇒ 执行面 fail-closed（同一句拒执文案）
package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// TestChatToolSurfaceOnlyZerg — ① 工具清单只留虫族命令。
func TestChatToolSurfaceOnlyZerg(t *testing.T) {
	// 与真跑同构：core 启动时 main.go 会先 RegisterChatExtraTools()（zerg_overview 等 chat 工具由此进 AllTools）
	// ⇒ 这里先调一次，工具清单读数才与运行态一致（✗ 拿「少注册一次」的清单当验收读数）。
	RegisterChatExtraTools()
	forbidden := []string{"bash", "read", "write", "edit", "glob", "grep", "ls", "tool_search", "kb_search",
		"web_search", "web_fetch", "skill_load", "zerg_overview"}
	// ①-c 的 L0 断言只覆盖「本单之前已撤」的那批：二次收窄的四条仍留在注册表 L0 表里
	// （注册表＝L0ToolNames 真源，本单不动），靠 chatForbiddenTools 在渲染面与执行面撤下。
	notL0 := []string{"bash", "read", "write", "edit", "glob", "grep", "ls", "tool_search", "kb_search"}

	// ①-a chat 格式 tools 参数（非流式 / send-tool 路径）
	names := toolNameListOf(BuildToolsParam())
	if len(names) == 0 {
		t.Fatal("BuildToolsParam 工具名为空（抽取口径失效）")
	}
	if !hasName(names, ZergCmdToolName) {
		t.Errorf("tools 参数里没有 zerg 工具：%v", names)
	}
	for _, f := range forbidden {
		if hasName(names, f) {
			t.Errorf("tools 参数里仍有已撤下工具 %q：%v", f, names)
		}
	}

	// ①-b Hermes <tools> 段（真发给模型的那一段）
	prompt := BuildHermesToolPrompt(nil)
	if !strings.Contains(prompt, `"name": "`+ZergCmdToolName+`"`) {
		t.Errorf("<tools> 段没有 zerg 工具")
	}
	for _, f := range forbidden {
		if strings.Contains(prompt, `"name": "`+f+`"`) {
			t.Errorf("<tools> 段仍声明已撤下工具 %q", f)
		}
	}

	// ①-c L0 集合（BuildToolsParam 的过滤真源）
	for _, f := range notL0 {
		if L0ToolNames[f] {
			t.Errorf("L0ToolNames 仍含已撤下工具 %q", f)
		}
	}
	if !L0ToolNames[ZergCmdToolName] {
		t.Errorf("L0ToolNames 缺 zerg（主提示不会出它）")
	}

	// ①-d 撤下面分两档：七个 fs/shell 工具 ＋ 2026-10-17 二次收窄的四条（web_search / web_fetch /
	//      skill_load / zerg_overview）= **硬禁**（执行面 fail-closed）；
	//      tool_search / kb_search = 仅「主提示不出」（拦执行没有安全收益）。
	for _, f := range []string{"bash", "read", "write", "edit", "glob", "grep", "ls",
		"web_search", "web_fetch", "skill_load", "zerg_overview"} {
		if !ChatToolForbidden(f) {
			t.Errorf("ChatToolForbidden(%q)=false（执行面不会拦它）", f)
		}
	}
	for _, f := range forbidden {
		if !chatPromptHidden(f) {
			t.Errorf("chatPromptHidden(%q)=false（主提示还会出它）", f)
		}
	}

	t.Logf("工具清单（改后）%d 条：%v", len(names), names)
}

// TestChatZergToolRunsVersion — ② `zerg version` 真跑：rc=0 ＋ stdout 含版本行。
func TestChatZergToolRunsVersion(t *testing.T) {
	bin, err := zergBinaryPath()
	if err != nil {
		t.Skipf("跳过（本机没有 zerg 可执行件）：%v", err)
	}
	res := ExecuteChatTool(ZergCmdToolName, map[string]any{"command": "zerg version"}, ChatToolsWorkDir)
	if res.Error != "" {
		t.Fatalf("zerg version 执行失败（%s）：%s", bin, res.Error)
	}
	if !strings.HasPrefix(res.Content, "rc=0\n") {
		t.Errorf("期望输出以 rc=0 开头，得到：%.200s", res.Content)
	}
	if !strings.Contains(res.Content, "zerg 2.") && !strings.Contains(res.Content, "zerg ") {
		t.Errorf("stdout 里看不到版本行：%s", res.Content)
	}
	t.Logf("zerg version 读数：%.400q", res.Content)
}

// TestChatZergToolRefusesShellSyntax — ③ 管道/串联/命令替换/多行 ⇒ rc=2 拒执。
func TestChatZergToolRefusesShellSyntax(t *testing.T) {
	cases := []string{
		"zerg version | cat",
		"zerg version; rm -rf /tmp/x",
		"zerg version && zerg help",
		"zerg version > /tmp/out.txt",
		"zerg code show $(cat x)",
		"zerg version `whoami`",
		"zerg version\nzerg help",
	}
	for _, cmd := range cases {
		res := ExecuteChatTool(ZergCmdToolName, map[string]any{"command": cmd}, ChatToolsWorkDir)
		if res.Error == "" {
			t.Errorf("%q 应被拒执（rc=2），实得 content=%.120s", cmd, res.Content)
			continue
		}
		if !strings.Contains(res.Error, "rc=2") || !strings.Contains(res.Error, "对话只能用虫族命令") {
			t.Errorf("%q 的拒执文案不符：%q", cmd, res.Error)
		}
	}
	// 对照：同族但无元字符的写法必须放行（防「一刀切全拒」把工具废掉）
	for _, cmd := range []string{"zerg version", "zerg help", "zerg gap ls --state 仍缺"} {
		if why := zergCmdVerdict(cmd); why != "" {
			t.Errorf("%q 不该被硬闸拒：%s", cmd, why)
		}
	}
}

// TestChatZergToolRefusesNonZerg — ④ 首词非 zerg ⇒ rc=2 拒执。
func TestChatZergToolRefusesNonZerg(t *testing.T) {
	cases := []string{"curl -s http://127.0.0.1:8580/", "ls -la", "zerg.exe version", "./zerg version", "ZERG version"}
	for _, cmd := range cases {
		res := ExecuteChatTool(ZergCmdToolName, map[string]any{"command": cmd}, ChatToolsWorkDir)
		if res.Error == "" {
			t.Errorf("%q（首词非 zerg）应被拒执，实得 content=%.120s", cmd, res.Content)
			continue
		}
		if !strings.Contains(res.Error, "rc=2") || !strings.Contains(res.Error, "对话只能用虫族命令") {
			t.Errorf("%q 的拒执文案不符：%q", cmd, res.Error)
		}
	}
	// 空参数
	if res := ExecuteChatTool(ZergCmdToolName, map[string]any{}, ChatToolsWorkDir); res.Error == "" {
		t.Errorf("空 command 应被拒执")
	}
}

// TestChatForbiddenToolsRefusedOnExecution — ⑤ 撤下工具兑旧名字直接调 ⇒ 执行面 fail-closed。
func TestChatForbiddenToolsRefusedOnExecution(t *testing.T) {
	for _, name := range []string{"bash", "read", "write", "edit", "glob", "grep", "ls"} {
		res := ExecuteChatTool(name, map[string]any{"command": "ls -la", "path": "."}, ChatToolsWorkDir)
		if res.Error == "" {
			t.Errorf("撤下工具 %q 在对话执行面仍能跑（content=%.120s）", name, res.Content)
			continue
		}
		if !strings.Contains(res.Error, "对话只能用虫族命令") {
			t.Errorf("撤下工具 %q 的拒执文案不符：%q", name, res.Error)
		}
	}
}

// TestSplitZergArgv — 带引号的参数要能切对（否则 zerg 命令一带空格就断，模型只好绕回 bash）。
func TestSplitZergArgv(t *testing.T) {
	got, err := splitZergArgv(`zerg gap note abc --text "含 空格" 'q x'`)
	if err != nil {
		t.Fatalf("切分失败：%v", err)
	}
	want := []string{"zerg", "gap", "note", "abc", "--text", "含 空格", "q x"}
	if len(got) != len(want) {
		t.Fatalf("切分条数 %d ≠ %d：%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 段 %q ≠ %q", i, got[i], want[i])
		}
	}
	if _, err := splitZergArgv(`zerg gap note "未闭合`); err == nil {
		t.Errorf("引号未闭合应报错（✗ 猜一个切法硬跑）")
	}
}

// hasName — 工具名列表里有没有它
func hasName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// TestChatToolCallPathEndToEnd — 真链路面（不是一个函数一个函数地测）：
// 对话主路径 = agent.ExecContext.ExecuteTool(name, args, ChatGate) ⇒ 工具名解析（AllTools）
// ⇒ ChatGate 判定 ⇒ 路由（agent 扩展注册表 / chat 执行器）。
//
// 两态：`zerg` 跑得通（rc=0）；`bash`/`read`/`ls` 被门 fail-closed（模型兑旧名字也跑不动）。
func TestChatToolCallPathEndToEnd(t *testing.T) {
	RegisterChatExtraTools() // 与 core 启动同构
	ec := agent.NewExecContext(ChatToolsWorkDir)
	ec.AgentName = "chat"
	gate := &ChatGate{}
	ctx := context.Background()

	// ① 旧名字：必须被拦，且拒执文案逐字含「对话只能用虫族命令」
	// ★ 每个工具都要给**能过参数校验**的实参 —— 否则先在参数面报「xxx 参数为空」，
	//   门那条路根本没走到（那只是「照样跑不动」，证不到本单要的执行面硬闸）。
	writeTarget := filepath.Join(t.TempDir(), "should-not-exist.txt")
	legacy := []struct {
		name string
		args map[string]any
	}{
		{"bash", map[string]any{"command": "ls -la"}},
		{"read", map[string]any{"path": "."}},
		{"ls", map[string]any{"path": "."}},
		{"grep", map[string]any{"pattern": "zerg", "path": "."}},
		{"glob", map[string]any{"pattern": "*"}},
		{"write", map[string]any{"path": writeTarget, "content": "x"}},
		{"edit", map[string]any{"path": writeTarget, "search": "a", "replace": "b"}},
	}
	for _, c := range legacy {
		res := ec.ExecuteTool(ctx, c.name, c.args, gate)
		if res.Error == "" {
			t.Errorf("ExecuteTool(%q) 仍执行（content=%.120s）——对话工具面没收窄", c.name, res.Content)
			continue
		}
		if !strings.Contains(res.Error, "对话只能用虫族命令") {
			t.Errorf("ExecuteTool(%q) 拒执文案不符：%q", c.name, res.Error)
		}
	}
	// fail-closed 的物证：write 目标件不得被创建
	if _, err := os.Stat(writeTarget); err == nil {
		t.Errorf("write 目标件被创建（%s）——执行面没有真正拦下", writeTarget)
	}

	// ② zerg：真跑一条命令（本机无 CLI 则跳过——如实说，✗ 编读数）
	if _, err := zergBinaryPath(); err != nil {
		t.Skipf("跳过 zerg 段（本机没有 zerg 可执行件）：%v", err)
	}
	res := ec.ExecuteTool(ctx, "zerg", map[string]any{"command": "zerg version"}, gate)
	if res.Error != "" {
		t.Fatalf("ExecuteTool(zerg) 失败：%s", res.Error)
	}
	if !strings.HasPrefix(res.Content, "rc=0\n") || !strings.Contains(res.Content, "zerg 2.") {
		t.Errorf("ExecuteTool(zerg) 读数不符：%.300q", res.Content)
	}
	t.Logf("ExecuteTool(zerg) 读数：%.300q", res.Content)
}

// TestChatToolsParamExactlyZerg — 2026-10-17 二次收窄的硬指标：
// BuildToolsParam() 必须**恰好一条**，且那一条是 zerg
// （web_search / web_fetch / skill_load / zerg_overview 一并撤出对话工具面）。
func TestChatToolsParamExactlyZerg(t *testing.T) {
	RegisterChatExtraTools() // 与 core 启动同构（否则读数会漏掉 chat 侧注册的件）
	names := toolNameListOf(BuildToolsParam())
	if len(names) != 1 || names[0] != ZergCmdToolName {
		t.Fatalf("对话工具面应恰 1 条 [%s]，实得 %d 条：%v", ZergCmdToolName, len(names), names)
	}
	prompt := BuildHermesToolPrompt(nil)
	for _, bad := range []string{"web_search", "web_fetch", "skill_load", "zerg_overview"} {
		if strings.Contains(prompt, `"name": "`+bad+`"`) {
			t.Errorf("<tools> 段仍声明已撤下工具 %q", bad)
		}
	}
	// skill_load 的替代必须是虫族命令（撤了不给正门 ⇒ 模型只能瞎猜）
	if !strings.Contains(prompt, "zerg skill ls") || !strings.Contains(prompt, "zerg skill show") {
		t.Errorf("提示面没给技能的替代正门（zerg skill ls / zerg skill show）")
	}
	// 搜网能力缺失要如实写在提示面（✗ 不造替代）
	if !strings.Contains(prompt, "搜网") {
		t.Errorf("提示面没点明搜网能力缺失（口径：如实撤、不造替代）")
	}
	t.Logf("工具清单（二次收窄后）%d 条：%v", len(names), names)
}

// TestChatExecGuardBlocksWithdrawnTools — 执行入口兜底闸（agent 不查门的那几条必须在这里被挡）。
//
// 背景（2026-10-17 实测）：对话路径兑旧名直调 web_search 曾**真联网**返回网页内容 ——
// agent 的 web_search / web_fetch 分支不调 gate.Check ⇒ 只在 ChatGate 里挡是挡不住的。
func TestChatExecGuardBlocksWithdrawnTools(t *testing.T) {
	for _, name := range []string{"web_search", "web_fetch", "skill_load", "zerg_overview", "bash", "read"} {
		if err := ChatExecGuard("sess-guard", name, map[string]any{"query": "x", "url": "http://x", "name": "ca-troubleshooting", "path": "."}); err == nil {
			t.Errorf("ChatExecGuard(%q) 放行了已撤下工具（执行面没兜住）", name)
		} else if !strings.Contains(err.Error(), "对话只能用虫族命令") {
			t.Errorf("ChatExecGuard(%q) 拒执文案不符：%v", name, err)
		}
	}
	// 对照：zerg 必须放行（✗ 一刀切把唯一工具也废掉）
	if err := ChatExecGuard("sess-guard", ZergCmdToolName, map[string]any{"command": "zerg version"}); err != nil {
		t.Errorf("ChatExecGuard(zerg) 不该拒：%v", err)
	}
}
