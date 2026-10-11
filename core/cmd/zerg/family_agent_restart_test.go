// family_agent_restart_test.go —— `zerg agent restart`（`cmdAgentRestart` · 缺口 `GAP-20260927-195` ·
// §十五.4 例外清单 **`F-5` 远端子端重启**）的回归牙。
//
// 正控：① 本机回环支仍是 `launchctl kickstart -k`（`F-5` 不动它）；
//
//	② 远端（mini-node-2 · macOS）支**放行到 `F-5`** —— 计划件点 `F-5` + 给将执行的远端命令，**不再退 8**；
//	③ 远端（edge-node · ubuntu）支走**既有 polkit 免密格** ⇒ `systemctl restart edge-node-agent.service`；
//	④ 远端**真跑**走 ssh 分派（接缝替换 ⇒ ✗ 不真 ssh）⇒ rc=0（`F-5` 那一格真被走到）；
//	⑤ `F-5` 已**照同形**登记进 `offlineExceptions()` 真源面。
//
// 负控（真红）：① 远端**真跑**才动动作面（干跑零副作用）；② **其它 ssh 面仍退 8**
//
//	（`agent registry <远端机>` ⇒ rc=8 —— `F-5` 只开「重启」这一格，没放开别的 ssh 面）。
//
// ★ 全程**不碰真仓真名册**（`ZERG_REPO` 指仓外临时根）· ✗ 不真连远端机 · ✗ 不 ssh 出去 · ✗ 不启停任何服务。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// arFleetRoot —— 仓外临时名册根（`repoRoot()` 无条件采信 `ZERG_REPO` ⇒ 换源；✗ 不碰真仓）。
func arFleetRoot(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "gateway"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gateway", "fleet.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_REPO", root)
	return root
}

const arFleet = "fleet:\n" +
	"  Mr2109: { host: 127.0.0.1, port: 8100, os: macos, kind: work }\n" +
	"  mini-node-2: { host: <worker-ip>, port: 8100, os: macos, kind: mini }\n" +
	"  edge-node: { host: <worker-ip>, port: 8100, os: ubuntu, kind: ai }\n"

func arInv(machine string, dryRun bool) *invocation {
	inv := &invocation{path: []string{"agent", "restart"}, args: []string{machine}}
	inv.dryRun = dryRun
	return inv
}

// 正控①：本机回环支仍是 `launchctl kickstart -k`（F-5 只管远端那一格，不动本机支）。
func TestAgentRestart_LocalBranchStaysKickstart(t *testing.T) {
	arFleetRoot(t, arFleet)
	var out, errb bytes.Buffer
	rc := cmdAgentRestart(arInv("Mr2109", true), &out, &errb)
	if rc != exitOK {
		t.Fatalf("本机干跑应退 0，实得 %d · stderr=%s", rc, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "local-launchctl") || !strings.Contains(s, "launchctl kickstart -k gui/") {
		t.Fatalf("本机支应仍是 launchctl kickstart -k，实得 stdout=%q", s)
	}
	if strings.Contains(s, "F-5") {
		t.Fatalf("本机支不该出现 F-5（F-5 是远端那一格），实得 stdout=%q", s)
	}
}

// 正控②：远端（mini-node-2 · macOS）支放行到 F-5 —— 计划件点 F-5 + 给将执行的远端命令，不再退 8。
func TestAgentRestart_RemoteMiniIsF5(t *testing.T) {
	arFleetRoot(t, arFleet)
	var out, errb bytes.Buffer
	rc := cmdAgentRestart(arInv("mini-node-2", true), &out, &errb)
	if rc != exitOK {
		t.Fatalf("远端干跑应退 0（不再退 8），实得 %d · stderr=%s", rc, errb.String())
	}
	s := out.String()
	for _, want := range []string{"remote-ssh", "F-5", "远端子端重启", "ssh ", "launchctl kickstart -k", "pkill -TERM"} {
		if !strings.Contains(s, want) {
			t.Fatalf("远端计划件应含 %q，实得 stdout=%q", want, s)
		}
	}
	for _, bad := range []string{"不给结论", "退 8", "退码 8"} {
		if strings.Contains(s, bad) {
			t.Fatalf("远端支已放行到 F-5，不该再出现 %q，实得 stdout=%q", bad, s)
		}
	}
}

// 正控③：远端（edge-node · ubuntu）支走既有 polkit 免密格 ⇒ `systemctl restart edge-node-agent.service`。
func TestAgentRestart_RemoteX3UsesPolkitRestart(t *testing.T) {
	arFleetRoot(t, arFleet)
	var out, errb bytes.Buffer
	rc := cmdAgentRestart(arInv("edge-node", true), &out, &errb)
	if rc != exitOK {
		t.Fatalf("远端干跑应退 0，实得 %d · stderr=%s", rc, errb.String())
	}
	if s := out.String(); !strings.Contains(s, "F-5") || !strings.Contains(s, "systemctl restart edge-node-agent.service") {
		t.Fatalf("edge-node 支应点 F-5 且走 polkit 免密格，实得 stdout=%q", s)
	}
}

// 正控④（真红负控①）：远端**真跑**走 ssh 分派（动作面 / 就绪面两枚接缝替换 ⇒ ✗ 不真 ssh、✗ 不真连）⇒ rc=0。
func TestAgentRestart_RemoteRealPathDispatchesSSH(t *testing.T) {
	arFleetRoot(t, arFleet)
	t.Setenv("ZERG_AGENT_RESTART_AUDIT", filepath.Join(t.TempDir(), "agent_restart_audit.jsonl"))

	var gotMachine string
	var gotAddr agentAddr
	called := 0
	oldAct, oldReady := agentRestartAction, agentRestartReady
	agentRestartAction = func(machine string, a agentAddr) error {
		gotMachine, gotAddr, called = machine, a, called+1
		return nil // ✗ 不真 ssh、✗ 不真重启
	}
	agentRestartReady = func(a agentAddr) (bool, string, int) { return true, "就绪桩（✗ 不真连远端机）", 0 }
	t.Cleanup(func() { agentRestartAction, agentRestartReady = oldAct, oldReady })

	var out, errb bytes.Buffer
	inv := &invocation{path: []string{"agent", "restart"}, args: []string{"mini-node-2"}}
	inv.confirmGiven, inv.confirm, inv.yes = true, "mini-node-2", true
	rc := cmdAgentRestart(inv, &out, &errb)
	if rc != exitOK {
		t.Fatalf("远端真跑应退 0（不再退 8），实得 %d · stderr=%s", rc, errb.String())
	}
	if called != 1 || gotMachine != "mini-node-2" || agentAddrIsLocal(gotAddr) {
		t.Fatalf("远端真跑应恰好调一次动作面且目标是远端机，实得 called=%d machine=%q addr=%+v", called, gotMachine, gotAddr)
	}
	if !strings.Contains(errb.String(), "分支 remote-ssh") || !strings.Contains(errb.String(), "执行 `ssh ") {
		t.Fatalf("远端真跑应经 ssh 分派（stderr 点名分支与出口），实得 stderr=%q", errb.String())
	}
}

// 负控①：远端**干跑**零副作用 —— 一个动作都不发。
func TestAgentRestart_RemoteDryRunTouchesNothing(t *testing.T) {
	arFleetRoot(t, arFleet)
	called := 0
	oldAct := agentRestartAction
	agentRestartAction = func(machine string, a agentAddr) error { called++; return nil }
	t.Cleanup(func() { agentRestartAction = oldAct })

	var out, errb bytes.Buffer
	if rc := cmdAgentRestart(arInv("mini-node-2", true), &out, &errb); rc != exitOK {
		t.Fatalf("干跑应退 0，实得 %d", rc)
	}
	if called != 0 {
		t.Fatalf("干跑**不许**动动作面（零副作用），实得 %d 次", called)
	}
}

// 负控②：**其它 ssh 面仍退 8** —— `F-5` 只开「重启」这一格，`agent registry <远端机>` 不许被顺带放开。
func TestAgentRestart_OtherSSHFaceStillBlocked(t *testing.T) {
	arFleetRoot(t, arFleet)
	var errb bytes.Buffer
	if _, rc := agentRegistryPath("mini-node-2", agentAddr{Host: "<worker-ip>", Port: 8100, OS: "macos"}, &invocation{}, &errb); rc != exitBlocked {
		t.Fatalf("其它 ssh 面（agent registry <远端机>）仍应退 8，实得 %d · stderr=%s", rc, errb.String())
	}
}

// 正控⑤：`F-5` 已**照同形**登记进 `offlineExceptions()` 真源面（`F-1`–`F-5`）。
func TestAgentRestart_F5RegisteredInOfflineExceptions(t *testing.T) {
	var row *offlineExceptionRow
	ids := []string{}
	rows := offlineExceptions()
	for i := range rows {
		r := rows[i]
		ids = append(ids, r.ID)
		if r.ID == "F-5" {
			row = &r
		}
	}
	if row == nil {
		t.Fatalf("offlineExceptions() 里没有 F-5 格，实得 %v", ids)
	}
	if !strings.Contains(row.Command, "zerg agent restart") {
		t.Fatalf("F-5 的 Command 应点名 `zerg agent restart <远端机>`，实得 %q", row.Command)
	}
	for _, want := range []string{"只做重启", "不提权", "polkit", "launchctl"} {
		if !strings.Contains(row.Why, want) {
			t.Fatalf("F-5 的 Why 应含 %q，实得 %q", want, row.Why)
		}
	}
}
