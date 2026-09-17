// obs_tool_decision_test.go — T1.2 实证：工具调用的「允许/拒绝 + 拒绝原因」是**可观测的一等事件**。
//
// 要治的盲区（写在这里，改测试前先读）：
//
//	一个调用"被系统拒绝"（权限/白名单/参数不合法/危险命令/被隐藏…）与"模型根本没想调"，
//	在原观测面里**同形**（都只是"没有那条工具事件"）⇒ 无法归因。本文件守住三态可分：
//	  ① 有 tool_decision 且 decision=deny  + deny_reason ⇒ 被拒（且原因可枚举）
//	  ② 有 tool_decision 且 decision=allow               ⇒ 放行
//	  ③ **没有 tool_decision**                            ⇒ 没发起该调用（负控：绝不能是"被拒了但没记"）
//
// 纪律（与 obs_test.go / obs_trace_test.go 同源）：
//   - 观测一律落隔离目录：t.Setenv("ZERG_STATE_DIR", t.TempDir())，绝不写用户真实状态。
//   - 判据分两层：**wire 层**（键真的在 JSONL 原文里）+ **read 层**（能解回、值是真的）——只断 Go 字段=自证。
//   - 拒绝用例同时断言"现有安全语义没被放宽"（越界仍被拒），观测不改判定。
package chat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
	"github.com/Mr2109/zerg-swarm/core/internal/loopcore"
	"github.com/Mr2109/zerg-swarm/core/internal/toolobs"
)

// obsDecisionLine — 判定事件读回结构（键缺席 ⇒ 零值，这正是"没记"的判据）
type obsDecisionLine struct {
	TS         string `json:"ts"`
	Kind       string `json:"kind"`
	Session    string `json:"session"`
	Tool       string `json:"tool"`
	Decision   string `json:"decision"`
	DenyReason string `json:"deny_reason"`
	ArgsDigest string `json:"args_digest"`
	EventName  string `json:"event_name"`
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
	SpanKind   string `json:"span_kind"`
	Result     string `json:"result"`
}

// readToolDecisionLines — 只挑 event_name=tool_decision 的行（其余事件不干扰断言）
func readToolDecisionLines(t *testing.T) []obsDecisionLine {
	t.Helper()
	raw := readObs(t)
	if raw == "" {
		return nil
	}
	var out []obsDecisionLine
	for _, ln := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var r obsDecisionLine
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			t.Fatalf("观测行不是合法 JSON（读侧必须宽容）：%v\n%s", err, ln)
		}
		if r.EventName == "tool_decision" {
			out = append(out, r)
		}
	}
	return out
}

// newT12ExecContext — 隔离工作区 + 会话 id 的 ExecContext（判定事件的骨架字段靠 session）
func newT12ExecContext(t *testing.T, session string) *agent.ExecContext {
	t.Helper()
	ec := agent.NewExecContext(t.TempDir())
	ec.AgentName = "t12-test"
	ec.Session = session
	return ec
}

// ① 拒绝路径：一次**被拒的工具调用**必须落 decision=deny + 可枚举原因（而不是"无记录"）
// —— 用「越界读」触发真实的白名单拒绝分支（path_outside_workspace）。
func TestToolDecisionDenyEventRecorded(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	ec := newT12ExecContext(t, "sess-T12-deny")

	args := map[string]any{"path": "../outside-do-not-read.txt"}
	res := ec.ExecuteTool(context.Background(), "read", args, nil)
	if res.Error == "" {
		t.Fatal("越界读必须仍被拒绝（T1.2 只加观测，不得放宽任何安全语义）")
	}

	// wire 层：四个关键键必须真的在落盘原文里
	raw := readObs(t)
	for _, key := range []string{`"event_name":"tool_decision"`, `"decision":"deny"`,
		`"deny_reason":"path_outside_workspace"`, `"tool":"read"`, `"args_digest":"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("落盘原文缺少 %s\n原文：%s", key, raw)
		}
	}
	// 隐私：参数原文不得落盘（只落摘要）
	if strings.Contains(raw, "outside-do-not-read.txt") {
		t.Errorf("参数原文被写进观测（只允许落摘要）：%s", raw)
	}

	// read 层：恰好一条判定事件（拒绝就不再补 allow——一次调用一条判定）
	lines := readToolDecisionLines(t)
	if len(lines) != 1 {
		t.Fatalf("被拒的调用应落 1 条判定事件，实际 %d 条：%s", len(lines), raw)
	}
	d := lines[0]
	if d.Decision != "deny" || d.DenyReason != "path_outside_workspace" || d.Tool != "read" {
		t.Errorf("判定事件不符：decision=%q reason=%q tool=%q", d.Decision, d.DenyReason, d.Tool)
	}
	if d.Session != "sess-T12-deny" {
		t.Errorf("会话未落：%q", d.Session)
	}
	if want := toolobs.Digest(args); d.ArgsDigest != want || len(d.ArgsDigest) != 16 {
		t.Errorf("参数摘要应为 sha256 前 16 位 %q（不落原文），实际 %q", want, d.ArgsDigest)
	}
	// trace/span 骨架（T1.1 九字段）必须同时落在**本事件**上——否则判定事件无法挂进父子树
	if len(d.TraceID) != obsIDHexLen || len(d.SpanID) != obsIDHexLen {
		t.Errorf("判定事件缺 trace/span 骨架：trace=%q span=%q", d.TraceID, d.SpanID)
	}
	if d.Kind != "tool" {
		t.Errorf("kind 应为 tool（与 OBS-3 同族），实际 %q", d.Kind)
	}
}

// ② 允许路径：正常调用必须落 decision=allow（否则"放行"永远没有正证据）
func TestToolDecisionAllowEventRecorded(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	ec := newT12ExecContext(t, "sess-T12-allow")

	// 工作区内真实文件（工作区 = ec.WorkDir 的 TempDir）
	f := filepath.Join(ec.WorkDir, "ok.txt")
	if err := os.WriteFile(f, []byte("hello t12\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"path": "ok.txt"}
	res := ec.ExecuteTool(context.Background(), "read", args, nil)
	if res.Error != "" {
		t.Fatalf("工作区内读应放行，实际报错：%s", res.Error)
	}

	raw := readObs(t)
	for _, key := range []string{`"event_name":"tool_decision"`, `"decision":"allow"`, `"tool":"read"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("落盘原文缺少 %s\n原文：%s", key, raw)
		}
	}
	lines := readToolDecisionLines(t)
	if len(lines) != 1 {
		t.Fatalf("放行的调用应落 1 条判定事件，实际 %d 条：%s", len(lines), raw)
	}
	d := lines[0]
	if d.Decision != "allow" {
		t.Errorf("decision 应为 allow，实际 %q", d.Decision)
	}
	if d.DenyReason != "" {
		t.Errorf("allow 不应带拒绝原因，实际 %q", d.DenyReason)
	}
	if want := toolobs.Digest(args); d.ArgsDigest != want {
		t.Errorf("参数摘要应为 %q，实际 %q", want, d.ArgsDigest)
	}
	if len(d.TraceID) != obsIDHexLen || len(d.SpanID) != obsIDHexLen {
		t.Errorf("判定事件缺 trace/span 骨架：trace=%q span=%q", d.TraceID, d.SpanID)
	}
}

// ③ 负控（反例守扠）：**未调用的工具不产生 decision 事件**。
// 防的正是把"没调"误读成"被拒"，或反过来把"被拒"读成"没调"。
func TestToolDecisionNegativeControlUncalledToolHasNoEvent(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())

	// 先证观测面是"活的"（写一条真实存在的 OBS-3 工具行）——否则"没有判定事件"可能只是"整个观测面没工作"
	ObsTool("sess-T12-neg", 1, "bash", "1ms", true, 1, MaxToolRounds)

	if lines := readToolDecisionLines(t); len(lines) != 0 {
		t.Fatalf("没有调用任何工具，却出现 %d 条判定事件（负控失败）：%+v", len(lines), lines)
	}
	raw := readObs(t)
	if !strings.Contains(raw, `"kind":"tool"`) {
		t.Fatalf("观测面本身没工作（连 OBS-3 的工具行都没落）⇒ 负控不成立：%s", raw)
	}
	if strings.Contains(raw, `"event_name":"tool_decision"`) {
		t.Errorf("未调用工具却落了判定事件：%s", raw)
	}
}

// ④ 安全门（gate）拒绝：走 obsGater 集中覆盖 ⇒ 原因码 gate_block
func TestToolDecisionGateBlockReasonRecorded(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	ec := newT12ExecContext(t, "sess-T12-gate")

	f := filepath.Join(ec.WorkDir, "g.txt")
	if err := os.WriteFile(f, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := ec.ExecuteTool(context.Background(), "read", map[string]any{"path": "g.txt"}, blockingGate{})
	if res.Error == "" {
		t.Fatal("gate 拦下的调用必须仍被拒绝（观测不得放宽判定）")
	}
	lines := readToolDecisionLines(t)
	if len(lines) != 1 {
		t.Fatalf("应落 1 条判定事件，实际 %d 条：%s", len(lines), readObs(t))
	}
	if lines[0].Decision != "deny" || lines[0].DenyReason != toolobs.ReasonGateBlock {
		t.Errorf("gate 拒绝应为 deny/%s，实际 %s/%s", toolobs.ReasonGateBlock, lines[0].Decision, lines[0].DenyReason)
	}

	// 换一个工具（bash）再走一遍：证明 gate 拒绝是**一处集中覆盖**（obsGater 包装），不是只覆盖 read
	res = ec.ExecuteTool(context.Background(), "bash", map[string]any{"command": "ls -la"}, blockingGate{})
	if res.Error == "" {
		t.Fatal("gate 拦下的 bash 调用必须仍被拒绝")
	}
	lines = readToolDecisionLines(t)
	if len(lines) != 2 {
		t.Fatalf("应累计 2 条判定事件（read+bash），实际 %d 条：%s", len(lines), readObs(t))
	}
	if last := lines[len(lines)-1]; last.Tool != "bash" || last.DenyReason != toolobs.ReasonGateBlock {
		t.Errorf("bash 的 gate 拒绝应为 deny/%s/bash，实际 %s/%s/%s",
			toolobs.ReasonGateBlock, last.Decision, last.DenyReason, last.Tool)
	}
}

// blockingGate — 测试用安全门（永远 block；不改动任何生产 gate 语义）
type blockingGate struct{}

func (blockingGate) Check(toolName, args, agentName string) (agent.Decision, error) {
	return agent.Decision{Action: "block", Message: "测试门：拦"}, nil
}

// ⑤ 其余拒绝分支各给一个原因码（覆盖"参数不合法 / 未知工具 / 危险命令"三类，均取自代码里真实的拒绝分支）
func TestToolDecisionReasonCodesPerBranch(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	ec := newT12ExecContext(t, "sess-T12-branches")

	// read 图像 ⇒ 本工具不执行该目标（真实拒绝分支：detectReadKind=img）
	if err := os.WriteFile(filepath.Join(ec.WorkDir, "pic.png"),
		[]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		tool   string
		args   map[string]any
		reason string
	}{
		{"必填参数缺失", "read", map[string]any{}, toolobs.ReasonParamMissing},
		{"参数不合 schema 约定（enum）", "ls", map[string]any{"path": ".", "sort_by": "乱值"}, toolobs.ReasonParamContract},
		{"未知工具", "not_a_real_tool_t12", nil, toolobs.ReasonUnknownTool},
		{"危险命令黑名单", "bash", map[string]any{"command": "mkfs.ext4 /dev/disk9"}, toolobs.ReasonDangerousCmd},
		{"本工具不执行该目标（图像）", "read", map[string]any{"path": "pic.png"}, toolobs.ReasonUnsupported},
	}
	for _, c := range cases {
		res := ec.ExecuteTool(context.Background(), c.tool, c.args, nil)
		if res.Error == "" {
			t.Errorf("%s：该调用必须仍被拒绝（观测不得放宽判定）", c.name)
			continue
		}
		// 每个 case 独立读一次（观测按行追加；取最后一条）
		lines := readToolDecisionLines(t)
		if len(lines) == 0 {
			t.Errorf("%s：没有判定事件（正被治的盲区：拒绝=无记录）", c.name)
			continue
		}
		got := lines[len(lines)-1]
		if got.Decision != "deny" || got.DenyReason != c.reason {
			t.Errorf("%s：应为 deny/%s，实际 %s/%s", c.name, c.reason, got.Decision, got.DenyReason)
		}
	}
}

// ⑥ 内核侧拒绝（工具被隐藏 ⇒ 不执行）同样要落判定事件 —— 否则这个分支仍是"无记录"
func TestToolDecisionKernelHiddenToolDenyRecorded(t *testing.T) {
	t.Setenv("ZERG_STATE_DIR", t.TempDir())
	const sess = "sess-T12-hidden"

	inferCalls := 0
	infer := func(ctx context.Context, model, sysPrompt string, msgs []map[string]any,
		onDelta func(deltaType, text string), tools []map[string]any) (*loopcore.Response, error) {
		inferCalls++
		if inferCalls > 1 {
			return &loopcore.Response{Content: "整理完毕"}, nil // 第二轮无工具调用 ⇒ 自然收尾
		}
		return &loopcore.Response{ToolCalls: []loopcore.ToolCall{
			{ID: "1", Name: "web_search", Args: map[string]any{"query": "t12"}},
		}}, nil
	}
	exec := func(ctx context.Context, name string, args map[string]any) (string, string, error) {
		t.Errorf("被隐藏的工具不得被执行，却执行了 %s", name)
		return "", "", nil
	}
	loopcore.Run(context.Background(), loopcore.Config{MaxRounds: 3}, "m", "sys",
		[]map[string]any{{"role": "user", "content": "hi"}},
		loopcore.Deps{Infer: infer, Exec: exec, Session: sess,
			Hooks: loopcore.Hooks{IsHidden: func(string) bool { return true }}})

	lines := readToolDecisionLines(t)
	if len(lines) != 1 {
		t.Fatalf("内核侧拒绝应落 1 条判定事件，实际 %d 条：%s", len(lines), readObs(t))
	}
	if lines[0].Decision != "deny" || lines[0].DenyReason != toolobs.ReasonToolHidden || lines[0].Tool != "web_search" {
		t.Errorf("内核拒绝应为 deny/%s/web_search，实际 %s/%s/%s",
			toolobs.ReasonToolHidden, lines[0].Decision, lines[0].DenyReason, lines[0].Tool)
	}
	if lines[0].Session != sess {
		t.Errorf("内核侧判定事件应带会话（否则拼不进 trace）：%q", lines[0].Session)
	}
}
