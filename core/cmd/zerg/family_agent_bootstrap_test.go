// family_agent_bootstrap_test.go —— `zerg agent bootstrap` 的「域选择 + 四坑」只读判据回归牙
// （缺口 `GAP-20260925-22` ·「agent bootstrap-user-domain」）。
//
// 正控：① 仓内夹具可读（Mr2109 / mini-node-1 各一件 · 可执行件取 `ProgramArguments` 首个 `<string>` 而非 Label）；② 健康事实可判（四格逐字 · 整单 ok）；③ 端到端 `--dry-run`（真夹具机 ⇒ 退 0、计划件落 stdout）。
// 负控一（非法/缺格 ⇒ 判红并点名）：裸 `:port`（listen_host 空）⇒ red 且点名 IPv4；/tmp 二进制 ⇒ signed red 且点名签名；无 `--machine` ⇒ 认不出（取不到）。
// 负控二（取不到 ⇒ 非 0 不给结论）：没有夹具的机 ⇒ `--dry-run` 退 8、stdout 0 字节（✗ 不许静默当已就绪）；缺机器名 ⇒ 退 2。
package main

import (
	"bytes"
	"strings"
	"testing"
)

// 正控① 仓内夹具可读：`--machine=Mr2109` / `--machine=mini-node-1` 各命中一件，字段逐字（含可执行件）。
func TestAgentBootFindFixture_Readable(t *testing.T) {
	root := repoRoot()
	if root == "" {
		t.Fatalf("仓根解析不到（本测应在仓内的 core/cmd/zerg 下跑）")
	}
	ms, ok := agentBootFindFixture(root, "Mr2109")
	if !ok {
		t.Fatalf("仓内应有 Mr2109 的部署夹具（deploy/com.zerg.agent-Mr2109.plist）")
	}
	if ms.Machine != "Mr2109" || ms.ListenHost != "127.0.0.1" || ms.Controller != "http://127.0.0.1:8580" || ms.Port != "8100" {
		t.Fatalf("Mr2109 夹具字段应逐字：得 %+v", ms)
	}
	// ★ 可执行件 = ProgramArguments 首个 <string>（…/bin/zerg-agentd）—— ✗ 不是 Label（com.zerg.agent-Mr2109）。
	if !strings.HasSuffix(ms.Program, "/bin/zerg-agentd") {
		t.Fatalf("Mr2109 可执行件应取 ProgramArguments 首个 <string>（…/bin/zerg-agentd），得 %q", ms.Program)
	}
	mini, ok := agentBootFindFixture(root, "mini-node-1")
	if !ok {
		t.Fatalf("仓内应有 mini-node-1 的部署夹具（gateway/deploy/com.x3gw.agent.mini.plist）")
	}
	if mini.Machine != "mini-node-1" || mini.ListenHost != "0.0.0.0" {
		t.Fatalf("mini 夹具字段应逐字（--machine mini-node-1 · --host 0.0.0.0）：得 %+v", mini)
	}
	if !strings.Contains(mini.Controller, "127.0.0.1:8580") {
		t.Fatalf("mini 夹具拨号地址应含默认 127.0.0.1:8580：得 %q", mini.Controller)
	}
	// ★ 可执行件 = /bin/sh（`/bin/sh -c "exec …"` 那一形态）—— ✗ 不是 Label（com.x3gw.agent）。
	if mini.Program != "/bin/sh" {
		t.Fatalf("mini 可执行件应取 ProgramArguments 首个 <string>（/bin/sh），得 %q", mini.Program)
	}
}

// 正控② 健康事实可判：四格逐字、整单 ok。
func TestAgentBootJudge_HealthyVerbatim(t *testing.T) {
	cells, verdict := agentBootJudge(agentBootPlistFact{
		Program: "/usr/local/bin/zerg-agentd", Controller: "http://127.0.0.1:8580",
		ListenHost: "0.0.0.0", Port: "8100", Machine: "Mr2109",
	})
	if verdict != agentBootOK {
		t.Fatalf("健康事实整单应 %s，得 %s（%+v）", agentBootOK, verdict, cells)
	}
	if len(cells) != 4 {
		t.Fatalf("应 4 格，得 %d（%+v）", len(cells), cells)
	}
	for _, c := range cells {
		if c.Flag != agentBootOK {
			t.Fatalf("健康事实每格应 %s，格 %s 得 %s（%+v）", agentBootOK, c.Key, c.Flag, c)
		}
	}
	if cells[0].Key != "domain" || cells[0].Value != agentBootDomainUser {
		t.Fatalf("domain 格应 %q/%q，得 %q/%q", "domain", agentBootDomainUser, cells[0].Key, cells[0].Value)
	}
	// signed 格值逐字取自 Program（不是 Label）。
	if cells[3].Key != "signed" || cells[3].Value != "/usr/local/bin/zerg-agentd" {
		t.Fatalf("signed 格值应逐字取 ProgramArguments 首个 <string>：得 %+v", cells[3])
	}
}

// 负控一（非法/缺格 ⇒ 判红并点名）：裸 `:port` ⇒ listen_host red（点名 IPv4）；/tmp 二进制 ⇒ signed red（点名签名）。
func TestAgentBootJudge_IllegalGoesRed(t *testing.T) {
	cells, verdict := agentBootJudge(agentBootPlistFact{
		Program: "/tmp/zerg-agentd", Controller: "http://127.0.0.1:8580",
		ListenHost: "", Port: "8100", Machine: "Mr2109",
	})
	if verdict != agentBootRed {
		t.Fatalf("裸 :port + /tmp 二进制应整单 %s，得 %s", agentBootRed, verdict)
	}
	byKey := map[string]agentBootCell{}
	for _, c := range cells {
		byKey[c.Key] = c
	}
	if byKey["listen_host"].Flag != agentBootRed || !strings.Contains(byKey["listen_host"].Note, "IPv4") {
		t.Fatalf("裸 :port 应判红且点名 IPv4：得 %+v", byKey["listen_host"])
	}
	if byKey["signed"].Flag != agentBootRed || !strings.Contains(byKey["signed"].Note, "签名") {
		t.Fatalf("/tmp 二进制应判红且点名签名：得 %+v", byKey["signed"])
	}
}

// 负控一b（缺格 ⇒ 认不出 ⇒ 取不到）：无 `<string>` / 无 `--machine` ⇒ ok=false。
func TestAgentBootPlistFactFrom_MissingMachineIsAbsent(t *testing.T) {
	if _, ok := agentBootPlistFactFrom("x.plist", "<plist><dict></dict></plist>"); ok {
		t.Fatalf("没有 <string>/--machine 应认不出（ok=false），不许编一台机名")
	}
	if _, ok := agentBootPlistFactFrom("x.plist", "<string>/bin/sh</string><string>-c</string>"); ok {
		t.Fatalf("有 <string> 但没有 --machine 也应认不出（ok=false）")
	}
}

// 负控一c（可执行件取不到 ⇒ 空串，不拿 Label 顶）：把 Label 放前也不许把 Label 当可执行件。
func TestAgentBootProgramFrom_NotLabel(t *testing.T) {
	body := "<key>Label</key><string>com.x3gw.agent</string><key>ProgramArguments</key><array><string>/bin/sh</string></array>"
	if got := agentBootProgramFrom(body); got != "/bin/sh" {
		t.Fatalf("可执行件应取 ProgramArguments 首个 <string>（/bin/sh），得 %q（✗ 别取到 Label）", got)
	}
	if got := agentBootProgramFrom("<key>Label</key><string>com.x</string>"); got != "" {
		t.Fatalf("没有 ProgramArguments ⇒ 可执行件应空串（取不到），得 %q", got)
	}
}

// 负控二（取不到 ⇒ 非 0 不给结论）：没有夹具的机 ⇒ `--dry-run` 退 8、stdout 0 字节（✗ 不许静默当已就绪）。
func TestCmdAgentBootstrap_NoFixtureIsBlocked(t *testing.T) {
	var out, errb bytes.Buffer
	rc := cmdAgentBootstrap(&invocation{path: []string{"agent", "bootstrap"}, args: []string{"nosuchmachine-zz"}, dryRun: true}, &out, &errb)
	if rc != exitBlocked {
		t.Fatalf("取不到夹具应退 8（blocked），得 %d（out=%q）", rc, out.String())
	}
	if out.Len() != 0 {
		t.Fatalf("取不到时不该给结论（stdout 应 0 字节），得 %q", out.String())
	}
}

// 负控二b（缺机器名 ⇒ 既有档拒）：`--dry-run` 不给机器名 ⇒ 退 2。
func TestCmdAgentBootstrap_MissingTargetIsUsage(t *testing.T) {
	var out, errb bytes.Buffer
	rc := cmdAgentBootstrap(&invocation{path: []string{"agent", "bootstrap"}, dryRun: true}, &out, &errb)
	if rc != exitUsage {
		t.Fatalf("缺机器名应退 2（usage），得 %d", rc)
	}
}

// 正控③（端到端只读）：真夹具机 ⇒ `--dry-run` 退 0、计划件落 stdout 且含「域选择」与四格。
func TestCmdAgentBootstrap_DryRunPlan(t *testing.T) {
	var out, errb bytes.Buffer
	rc := cmdAgentBootstrap(&invocation{path: []string{"agent", "bootstrap"}, args: []string{"mini-node-1"}, dryRun: true}, &out, &errb)
	if rc != exitOK {
		t.Fatalf("有夹具的机 --dry-run 应退 0，得 %d（err=%q）", rc, errb.String())
	}
	s := out.String()
	for _, want := range []string{"域选择", "只读判定", "listen_host", "signed", "0.0.0.0", agentBootDomainUser} {
		if !strings.Contains(s, want) {
			t.Fatalf("计划件应含 %q：\n%s", want, s)
		}
	}
	// ★ 回归：signed 格不许把 Label 当可执行件（曾踩：两件夹具 Label 前于 ProgramArguments）。
	if strings.Contains(s, "signed       : com.x3gw.agent") {
		t.Fatalf("signed 格不得取到 Label（com.x3gw.agent）：\n%s", s)
	}
}
