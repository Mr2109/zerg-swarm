// family_agent_run_test.go —— `zerg agent run`（任务运行器归入命令面 · 批 B5）的判据。
//
// 落点 = `package main`（**内部**）：本件钉的四格（任务单 §三 B5 的「判据件至少」）：
//
//	① **登记面**：命令树里真有 `agent run`（用法串含源件那批旗标 · `arity=none` · `fields:` 与
//	   `agentRunFields` 逐字一致 · 已开放），且**既有 `agent` 动作一条不少**（归入 = 只增一格 ·
//	   ✗ 不新开顶层族）；
//	② **正常路径**：旗标 → 配置（缺省值逐字取源件）· 接单档的任务文本与 `fixing` 写回 ·
//	   终止原因 → 退码（差异 ③ 的重新取号）· `--json` 包封（字段闭集 + 未选字段不出现）·
//	   人面结果面四行；
//	③ **用法面**：十二格一律 `2` + stdout 零字节（`--task`/`--issue` 三态 · 值旗标裸给 ·
//	   整数不合法 · 未知旗标 · 多余位置参数 · `--json` 两档）+ 缺令牌 `4` + 新旗标**不泄漏**给别族；
//	④ **只读面**：用法面与缺令牌两档**一个字节都不写**（工作目录逐件 sha256 不变 · 问题单逐字节
//	   不变 —— 差异 ② 的核心：令牌闸在执行前，源件会先把问题单标成 `fixing` 再死）。
//
// ★ 模型腿（真调网关走完 `agent.Loop`）**不在本件里跑**：那要真网关 + 真模型 + 真工具执行，
// 属双轨二进制 `core/cmd/zerg-agent` 与 `core/internal/agent` 既有判据的面。本件钉的是
// 「归入命令面」这一层：旗标面 / 退码面 / 包封面 / 零副作用面 / 登记面。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Mr2109/zerg-swarm/core/internal/agent"
)

// agentRunInvoke —— 走**外部**入口跑一次（与 `package main_test` 的 `RunForTest` 同一件）。
func agentRunInvoke(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := run(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

// agentRunInv —— 造一枚已解析的 invocation（配置档直取 · 便于单测解析面）。
func agentRunInv(t *testing.T, argv ...string) *invocation {
	t.Helper()
	inv, err := parseInvocation(append([]string{"agent", "run"}, argv...))
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	inv.path = []string{"agent", "run"}
	return inv
}

// agentRunIsolatedHome —— 令牌闸与状态目录都指到合成面（✗ 不碰真 `~/.zerg`）。
func agentRunIsolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZERG_STATE_DIR", filepath.Join(home, "state"))
	t.Setenv("ZERG_AUTH_TOKEN", "tok-b5")
	return home
}

// ① 登记面：path / 旗标逐字 / fields / arity / 既有动作仍在 / 不新开族。
func TestAgentRunCommandRegistered(t *testing.T) {
	info, ok := CommandInfoOfForTest("agent run")
	if !ok {
		t.Fatalf("命令树里没有 `zerg agent run`（批 B5 的核心那一条）")
	}
	// 源件那批旗标**逐字**保留（只把单横线写成双横线）；`-version` 由差异 ⑩ 明确不搬。
	for _, f := range []string{
		"--task", "--issue", "--model", "--workdir", "--max-turns", "--tools", "--mcp",
		"--auto-issue", "--subtask", "--retry", "--state", "--gateway",
		"--x3-status", "--x3-token", "--json",
	} {
		if !strings.Contains(info.Usage, f) {
			t.Errorf("用法串漏了旗标 %s（源件必须逐字保留）：%q", f, info.Usage)
		}
	}
	if !strings.HasPrefix(info.Usage, "zerg agent run") {
		t.Errorf("用法串没以 `zerg agent run` 开头：%q", info.Usage)
	}
	if info.Passthrough {
		t.Errorf("`agent run` 不该是透传型（它自己判旗标语义）")
	}
	if !info.OpenForRun {
		t.Errorf("`agent run` 要标成**已开放**（真跑一个任务 —— 不是占位动作）")
	}
	if got, want := strings.Join(info.Fields, ","), strings.Join(agentRunFields, ","); got != want {
		t.Errorf("`fields:` = %q（要与 agentRunFields 逐字同源：%q）", got, want)
	}

	var arity string
	for _, c := range commands {
		if joinPath(c.path) == "agent run" {
			arity = c.arity
		}
	}
	if arity != arityNone {
		t.Errorf("`agent run` 的 arity = %q（要 %q —— 位置参数一律拒）", arity, arityNone)
	}

	// 既有 `agent` 动作**一条不少**（归入是「只增一格」，✗ 不许挤掉子端面）。
	for _, p := range []string{"agent ls", "agent show", "agent deploy", "agent load", "agent unload"} {
		if _, ok := CommandInfoOfForTest(p); !ok {
			t.Errorf("`zerg %s` 不见了 —— 本批只许新增 `agent run`，不许动子端面", p)
		}
	}
	// ✗ 不新开顶层族：第二段是 `run` 的顶层命令一条都不许有（`run` 只作 `agent` 的动作段）。
	for _, c := range commands {
		if len(c.path) >= 1 && c.path[0] == "run" {
			t.Errorf("出现了顶层 `zerg run …`（%v）—— 本批并入既有 `agent` 族，✗ 不新开族", c.path)
		}
	}
}

// ② 正常路径：旗标 → 配置 / 接单档文本与写回 / 退码映射 / 包封 / 人面结果面。
func TestAgentRunNormalPath(t *testing.T) {
	agentRunIsolatedHome(t)
	wd := t.TempDir()

	// ---- 缺省值逐字取源件 ----
	inv := agentRunInv(t, "--task", "写一个 Go HTTP 服务器", "--workdir", wd)
	cfg, rc := agentRunParse(inv, &bytes.Buffer{})
	if rc != exitOK {
		t.Fatalf("`agent run --task … --workdir <dir>` 解析 rc=%d（要 0）", rc)
	}
	if cfg.model != agentRunDefaultModel {
		t.Errorf("`--model` 缺省 = %q（要源件的 %q）", cfg.model, agentRunDefaultModel)
	}
	if cfg.maxTurns != agent.DefaultMaxTurns {
		t.Errorf("`--max-turns` 缺省 = %d（要唯一真源 agent.DefaultMaxTurns = %d）", cfg.maxTurns, agent.DefaultMaxTurns)
	}
	if cfg.retry != agentRunDefaultRetry {
		t.Errorf("`--retry` 缺省 = %d（要源件的 %d）", cfg.retry, agentRunDefaultRetry)
	}
	if cfg.workDir != wd || !filepath.IsAbs(cfg.workDir) {
		t.Errorf("工作目录 = %q（要绝对路径 %q）", cfg.workDir, wd)
	}
	if cfg.gateway == "" {
		t.Errorf("`--gateway` 缺省为空（要 statepath.GatewayBaseURL() 的现读值）")
	}
	if cfg.autoIssue || cfg.subtask {
		t.Errorf("两枚布尔旗标没给时必须是 false：%+v", cfg)
	}
	if cfg.token != "tok-b5" {
		t.Errorf("令牌没从环境变量取到：%q", cfg.token)
	}

	// ---- 全旗标档：逐格落到配置 ----
	inv = agentRunInv(t, "--task", "t", "--workdir", wd, "--model", "m-1", "--max-turns", "7",
		"--tools", "bash,write", "--mcp", "kb:kb|serve", "--auto-issue", "--subtask",
		"--retry", "3", "--state", filepath.Join(wd, "st.json"), "--gateway", "http://127.0.0.1:9")
	cfg, rc = agentRunParse(inv, &bytes.Buffer{})
	if rc != exitOK {
		t.Fatalf("全旗标档解析 rc=%d（要 0）", rc)
	}
	if cfg.model != "m-1" || cfg.maxTurns != 7 || cfg.retry != 3 || !cfg.autoIssue || !cfg.subtask ||
		cfg.tools != "bash,write" || cfg.mcp != "kb:kb|serve" || cfg.gateway != "http://127.0.0.1:9" ||
		cfg.stateFile != filepath.Join(wd, "st.json") {
		t.Errorf("全旗标档没逐格落下：%+v", cfg)
	}

	// ---- 接单档：任务文本逐字骨架 + `fixing` 写回（源件 `MarkIssueStatus` 单一真源）----
	issue := filepath.Join(wd, "T-B5.md")
	body := "# 任务\n- **任务**: 修一个函数\n- **状态**: open\n- **根因**: \n"
	if err := os.WriteFile(issue, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	inv = agentRunInv(t, "--issue", issue, "--workdir", wd)
	if cfg, rc = agentRunParse(inv, &bytes.Buffer{}); rc != exitOK {
		t.Fatalf("接单档解析 rc=%d（要 0）", rc)
	}
	task, irc := agentRunLoadIssue(inv, cfg, &bytes.Buffer{})
	if irc != exitOK {
		t.Fatalf("接单档读入 rc=%d（要 0）", irc)
	}
	for _, want := range []string{"接单修复问题单 " + issue, "修一个函数", "1.查根因", "4.标记状态为 resolved"} {
		if !strings.Contains(task, want) {
			t.Errorf("接单档任务文本缺 %q：\n%s", want, task)
		}
	}
	after, err := os.ReadFile(issue)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "- **状态**: fixing") {
		t.Errorf("接单档没把问题单标成 `fixing`：\n%s", string(after))
	}
	if !strings.Contains(string(after), "- **任务**: 修一个函数") {
		t.Errorf("接单档动了问题单的其余行（只该改状态那一行）：\n%s", string(after))
	}

	// ---- 终止原因 → 退码（差异 ③：源件的 3/4 重新取号成 1）----
	for _, one := range []struct {
		reason agent.TerminateReason
		want   int
	}{
		{agent.ReasonComplete, exitCodeOf("ok")},
		{agent.ReasonModelError, exitCodeOf("failed")},
		{agent.ReasonMaxTurns, exitCodeOf("failed")},
		{agent.ReasonBlocked, exitCodeOf("failed")},
		{agent.ReasonTokenBudget, exitCodeOf("failed")},
	} {
		if got := agentRunExitCode(one.reason); got != one.want {
			t.Errorf("退码映射 %q = %d（要 %d —— 命令面主表里 3=partial_ok / 4=unauthenticated）",
				one.reason, got, one.want)
		}
	}
	// ★ 与源件进程内码空间的**撞号**是这条差异的理由：显式钉住「本命令绝不再产 3/4 这两个值」。
	for _, reason := range []agent.TerminateReason{agent.ReasonModelError, agent.ReasonTokenBudget, agent.ReasonMaxTurns} {
		if got := agentRunExitCode(reason); got == 3 || got == 4 {
			t.Errorf("%q 映射到 %d —— 与命令面 3(partial_ok)/4(unauthenticated) 撞号（门⑨硬红线）", reason, got)
		}
	}

	// ---- 包封：字段闭集 + 未选字段不出现 + 恒数组 ----
	res := agent.LoopResult{Reason: agent.ReasonComplete, Turns: 3, Tokens: 1234, Content: "做完了"}
	row := agentRunRow(cfg, res, "/tmp/logs/x", exitOK, 1500*1000000)
	if row["ok"] != "true" || row["status"] != "complete" || row["turns"] != "3" ||
		row["tokens"] != "1234" || row["exit_code"] != "0" || row["duration_ms"] != "1500" {
		t.Errorf("结果行不对：%+v", row)
	}
	inv = agentRunInv(t, "--task", "t", "--json", "ok,status,turns")
	var out, errb bytes.Buffer
	if rc := selectJSON(&out, &errb, inv, inv.path, inv.fields, row); rc != exitOK {
		t.Fatalf("`--json` 包封 rc=%d（要 0）· stderr=%s", rc, errb.String())
	}
	var env struct {
		Schema string `json:"schema"`
		Items  []map[string]any
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("包封不是 JSON：%v\n%s", err, out.String())
	}
	if env.Schema != "zerg/v1" {
		t.Errorf("包封第一键 schema = %q（要 zerg/v1）", env.Schema)
	}
	if len(env.Items) != 1 {
		t.Fatalf("`items` 要恒数组且一条：%s", out.String())
	}
	got := env.Items[0]
	for _, k := range []string{"ok", "status", "turns"} {
		if _, ok := got[k]; !ok {
			t.Errorf("选中的字段 %q 不在包封里：%s", k, out.String())
		}
	}
	for _, k := range []string{"tokens", "summary", "workdir", "logs", "exit_code", "duration_ms"} {
		if _, ok := got[k]; ok {
			t.Errorf("没选中的字段 %q 出现在包封里（多余字段 = 契约外的面）：%s", k, out.String())
		}
	}

	// ---- 人面结果面：四行 + 摘要（截 200 字）+ 工作区/日志两行 ----
	var human bytes.Buffer
	long := strings.Repeat("啊", 260)
	agentRunHumanSummary(&human, cfg, agent.LoopResult{Reason: agent.ReasonComplete, Turns: 2, Tokens: 9, Content: long}, "/tmp/logs/y")
	h := human.String()
	for _, want := range []string{
		"🏁 Result:", "status: 完成 (checker 通过)", "turns:  2", "Token:  9",
		"📂 Workspace: " + cfg.workDir, "📋 Log:   /tmp/logs/y", agentRunClip(long, 200),
	} {
		if !strings.Contains(h, want) {
			t.Errorf("人面结果面缺 %q：\n%s", want, h)
		}
	}
	if strings.Contains(h, long) {
		t.Errorf("摘要没截（源件同档要截）：\n%s", h)
	}
	// 差异 ⑫：截断按**字符边界**（源件按字节切 ⇒ 中文半枚非法 UTF-8）。
	clipped := agentRunClip(long, 200)
	if len(clipped) > 200+3 {
		t.Errorf("截断后长度 %d 字节（要 ≤203 = 200 + `...`）", len(clipped))
	}
	if !utf8.ValidString(clipped) {
		t.Errorf("截断切在字符中间（打出非法 UTF-8）：%q", clipped)
	}
	if got := agentRunClip("short", 200); got != "short" {
		t.Errorf("短摘要不该被改动：%q", got)
	}
	// 纯 ASCII 档与源件的字节口径逐字同。
	ascii := strings.Repeat("a", 260)
	if got, want := agentRunClip(ascii, 200), ascii[:200]+"..."; got != want {
		t.Errorf("ASCII 摘要 = %q（要与源件字节口径逐字同：%q）", got, want)
	}
}

// ③ 用法面：十二格一律 2 + stdout 零字节；缺令牌 4；新旗标不泄漏；解析面认得十一枚。
func TestAgentRunUsageFace(t *testing.T) {
	agentRunIsolatedHome(t)
	wd := t.TempDir()
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"两枚都没给", []string{"agent", "run"}, "必填其一"},
		{"--task 裸给", []string{"agent", "run", "--task"}, "`--task` 要一段"},
		{"--issue 裸给", []string{"agent", "run", "--issue"}, "`--issue` 要一个"},
		{"两枚同给", []string{"agent", "run", "--task", "t", "--issue", "i"}, "只能给一枚"},
		{"--model 裸给", []string{"agent", "run", "--task", "t", "--model"}, "--model"},
		{"--workdir 裸给", []string{"agent", "run", "--task", "t", "--workdir"}, "--workdir"},
		{"--max-turns 非整数", []string{"agent", "run", "--task", "t", "--max-turns", "abc"}, "≥1 的整数"},
		{"--max-turns 0", []string{"agent", "run", "--task", "t", "--max-turns", "0"}, "≥1 的整数"},
		{"--retry 非整数", []string{"agent", "run", "--task", "t", "--retry", "abc"}, "≥0 的整数"},
		// `-1` 形态：值看起来像旗标 ⇒ 解析面按「裸形」收，`-1` 自己落进未知旗标兜底（仍 2）。
		{"--retry 负数", []string{"agent", "run", "--task", "t", "--retry", "-1"}, "未知旗标"},
		{"未知旗标", []string{"agent", "run", "--nosuchflag-zz"}, "未知旗标"},
		{"多余位置参数", []string{"agent", "run", "extra"}, "不收位置参数"},
		{"--json 不给字段", []string{"agent", "run", "--task", "t", "--json"}, "字段列表"},
	}
	for _, c := range cases {
		rc, out, errb := agentRunInvoke(t, c.argv...)
		if rc != exitUsage {
			t.Errorf("%s：rc=%d（要 %d · 用法错）", c.name, rc, exitUsage)
		}
		if out != "" {
			t.Errorf("%s：stdout 要零字节（用法类错误不吐包封 · K2 甲档）：%q", c.name, out)
		}
		if !strings.Contains(errb, c.want) {
			t.Errorf("%s：stderr 没说到 %q：%q", c.name, c.want, errb)
		}
	}
	// 字段面：不在册的字段 ⇒ 2（且不跑到执行面 —— 字段校验在执行前）。
	for _, argv := range [][]string{
		{"agent", "run", "--task", "t", "--json", "nope"},
		{"agent", "run", "--task", "t", "--json", "ok,nope"},
	} {
		rc, out, errb := agentRunInvoke(t, argv...)
		if rc != exitUsage || out != "" || !strings.Contains(errb, "nope") {
			t.Errorf("%v：rc=%d out=%q err=%q（要 2 + 零字节 + 点名 nope）", argv, rc, out, errb)
		}
	}

	// 缺令牌 ⇒ **4**（差异 ②：命令面 `unauthenticated`；源件同况是 `log.Fatalf` ⇒ 1）。
	t.Setenv("ZERG_AUTH_TOKEN", "")
	t.Setenv("ZERG_API_TOKEN", "")
	empty := t.TempDir()
	t.Setenv("HOME", empty)
	rc, out, errb := agentRunInvoke(t, "agent", "run", "--task", "t", "--workdir", wd)
	if rc != exitCodeOf("unauthenticated") {
		t.Errorf("缺令牌：rc=%d（要 %d = unauthenticated）", rc, exitCodeOf("unauthenticated"))
	}
	if out != "" {
		t.Errorf("缺令牌：stdout 要零字节：%q", out)
	}
	if !strings.Contains(errb, "缺共享令牌") || !strings.Contains(errb, "未认证") {
		t.Errorf("缺令牌判词不对：%q", errb)
	}

	// 解析面：十一枚新旗标都认（名字表 ∪ 布尔 case），且**写进 `inv.kv`**（`foreignFlag` 看得见）。
	agentRunIsolatedHome(t)
	inv := agentRunInv(t, "--task", "t", "--issue", "", "--tools", "a", "--mcp", "b",
		"--max-turns", "2", "--retry", "2", "--gateway", "http://g", "--x3-status", "http://s",
		"--x3-token", "k", "--auto-issue", "--subtask")
	for _, f := range []string{"--task", "--tools", "--mcp", "--max-turns", "--retry",
		"--gateway", "--x3-status", "--x3-token", "--auto-issue", "--subtask"} {
		if !inv.hasFlag(f) {
			t.Errorf("解析面没登记 %q（不登记 = 一律退 2「未知旗标」= 新旗标等于不可用）", f)
		}
	}

	// **不泄漏**：这十一枚在别族的命令上一律仍 2（数据驱动收口那一排）。
	for _, argv := range [][]string{
		{"trace", "--task", "x"},
		{"agent", "ls", "--subtask"},
		{"agent", "show", "edge-node", "--auto-issue"},
		{"agent", "deploy", "edge-node", "--mcp", "a"},
		{"sched", "show", "--gateway", "http://x"},
		{"api", "ls", "--max-turns", "2"},
	} {
		rc, out, _ := agentRunInvoke(t, argv...)
		if rc != exitUsage || out != "" {
			t.Errorf("%v：rc=%d out=%q（别族给新旗标要仍 2 且 stdout 零字节）", argv, rc, out)
		}
	}
}

// ④ 只读面：用法面与缺令牌两档一个字节都不写（差异 ② 的核心断言）。
func TestAgentRunReadOnly(t *testing.T) {
	agentRunIsolatedHome(t)
	wd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wd, "docs", "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	issue := filepath.Join(wd, "docs", "issues", "T-RO.md")
	body := "# 任务\n- **任务**: 只读面\n- **状态**: open\n"
	if err := os.WriteFile(issue, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	before := schedRunTreeHash(t, wd)

	// 用法错三档：都不许建 `<workdir>/.zerg/logs`、不许动问题单。
	for _, argv := range [][]string{
		{"agent", "run", "--workdir", wd},
		{"agent", "run", "--task", "t", "--issue", issue, "--workdir", wd},
		{"agent", "run", "--task", "t", "--workdir", wd, "--max-turns", "x"},
		{"agent", "run", "--task", "t", "--workdir", wd, "--json", "nope"},
	} {
		if rc, _, _ := agentRunInvoke(t, argv...); rc != exitUsage {
			t.Errorf("%v：rc=%d（要 2）", argv, rc)
		}
	}
	if after := schedRunTreeHash(t, wd); after != before {
		t.Errorf("用法错动了工作目录（连日志目录都不该建）：\n改前：\n%s\n改后：\n%s", before, after)
	}

	// 缺令牌：问题单**逐字节不变**（源件会先标 `fixing` 再 `log.Fatalf` ⇒ 差异 ② 的断言）。
	t.Setenv("ZERG_AUTH_TOKEN", "")
	t.Setenv("ZERG_API_TOKEN", "")
	t.Setenv("HOME", t.TempDir())
	if rc, _, _ := agentRunInvoke(t, "agent", "run", "--issue", issue, "--workdir", wd); rc != exitCodeOf("unauthenticated") {
		t.Errorf("缺令牌 + 接单档：rc=%d（要 4）", rc)
	}
	nowRaw, err := os.ReadFile(issue)
	if err != nil {
		t.Fatal(err)
	}
	if string(nowRaw) != body {
		t.Errorf("缺令牌时问题单被改了（令牌闸必须执行前就判）：\n改前：%q\n改后：%q", body, string(nowRaw))
	}
	if after := schedRunTreeHash(t, wd); after != before {
		t.Errorf("缺令牌档动了工作目录：\n改前：\n%s\n改后：\n%s", before, after)
	}

	// 读不到的问题单 ⇒ 1（跑到了、没成功），且不建日志目录（读入在建 Agent 之前）。
	agentRunIsolatedHome(t)
	missing := filepath.Join(wd, "docs", "issues", "没有这一单.md")
	rc, out, errb := agentRunInvoke(t, "agent", "run", "--issue", missing, "--workdir", wd)
	if rc != exitFail {
		t.Errorf("问题单读不到：rc=%d（要 %d）", rc, exitFail)
	}
	if out != "" || !strings.Contains(errb, "读 issue 失败") {
		t.Errorf("问题单读不到：out=%q err=%q（要零字节 stdout + 那句判词）", out, errb)
	}
	if _, serr := os.Stat(filepath.Join(wd, ".zerg")); !os.IsNotExist(serr) {
		t.Errorf("问题单读不到还建了 `<workdir>/.zerg`（读入必须在建日志之前）")
	}
}
