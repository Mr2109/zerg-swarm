// family_agent_deploy_test.go —— `zerg agent deploy`（缺口 `GAP-20261018-20`）的回归牙。
//
// 两态判据（本单验收 ①）：
//
//	正控 A：缺 `--confirm` ⇒ **退 2**，且**一个远端动作都不发**（审计件也不落）—— fail-closed；
//	正控 B：`--dry-run` ⇒ **退 0** 且**出计划件**（四步 ＋ 三条「不做的」逐条点名），零副作用；
//	正控 C：真换（接缝替换）⇒ 退 0，落地路径 = `<prefix>/zerg-agentd`，身份判词 `verified`；
//	正控 D：`--file` 缺 ⇒ 退 2；`--confirm` 值不匹配 ⇒ 退 2（✗ 不「当没给」）。
//
// 负控（真红）：
//
//	负控 ①：**坏件**（普通文本 / 非子端守护件）⇒ 退 1 且**不 ssh、不重启**（旧件仍在跑）；
//	负控 ②：换完身份**没跟上**（子端自报的 code_sha 与件内嵌 commit 不同）⇒ 退 1，✗ 不报绿。
//
// ★ 全程不碰真仓真名册（`ZERG_REPO` 指仓外临时根）· ✗ 不 ssh · ✗ 不重启任何服务 · ✗ 不写真状态目录。
package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// adRepoRoot 造一棵仓外名册根（`fleet:` 定地址 · `updates.nodes` 定落地前缀）。
func adRepoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "gateway"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "fleet:\n" +
		"  Mr2109: { host: 127.0.0.1, port: 8100, os: macos, kind: work }\n" +
		"  edge-node: { host: <worker-ip>, port: 8100, os: ubuntu, kind: ai }\n" +
		"updates:\n" +
		"  check: true\n" +
		"  nodes:\n" +
		"    - { name: Mr2109, ssh: -, root: \"/tmp\", prefix: \"/tmp/bin\", api: \"\", components: \"core,agentd\" }\n" +
		"    - { name: edge-node, ssh: g01@<worker-ip>, root: /home/<user>/zerg, prefix: /home/<user>/zerg/bin, api: \"\", components: agentd }\n"
	if err := os.WriteFile(filepath.Join(root, "gateway", "fleet.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_REPO", root)
	t.Setenv("ZERG_AGENT_DEPLOY_AUDIT", filepath.Join(t.TempDir(), "agent_deploy_audit.jsonl"))
	return root
}

// adArtifact 一份「合格件」的替身（✗ 真件：判据只关心身份两格与字节）。
func adArtifact(path, targetOS string) (agentDeployArtifact, string, error) {
	art := agentDeployArtifact{
		Path:      "/tmp/fake-zerg-agentd",
		Bytes:     []byte("fake-bytes"),
		SHA256:    strings.Repeat("ab", 32),
		Kind:      "elf",
		Commit:    "2698b27fc+dirty",
		BuildTime: "2026-10-19T00:00:00Z",
	}
	return art, "embedded-tags", nil
}

// adInv 造一条 invocation（`--file` 走 kv —— 与解析面同一读法 `inv.flagVal`）。
func adInv(machine, file string) *invocation {
	inv := &invocation{path: []string{"agent", "deploy"}, args: []string{machine}}
	if file != "" {
		inv.kvSet("--file", file)
	}
	return inv
}

// adSeams 装上三枚接缝（读件 / 换件 / 重启 / 子端自报 / 主控面）并记调用次数。
type adSeamState struct {
	transfer int
	restart  int
	probe    int
	gotPath  string
	gotBytes int
}

func adInstallSeams(t *testing.T, art agentDeployArtifact, self agentDeploySelf, fleetSHA string) *adSeamState {
	t.Helper()
	st := &adSeamState{}
	oldRead, oldTr, oldRs, oldPr, oldFl, oldSl :=
		agentDeployReadArtifactFn, agentDeployTransferFn, agentDeployRestartFn,
		agentDeployProbeFn, agentDeployFleetSHAFn, agentDeploySleep
	t.Cleanup(func() {
		agentDeployReadArtifactFn, agentDeployTransferFn, agentDeployRestartFn =
			oldRead, oldTr, oldRs
		agentDeployProbeFn, agentDeployFleetSHAFn, agentDeploySleep = oldPr, oldFl, oldSl
	})
	agentDeployReadArtifactFn = adArtifact
	agentDeployTransferFn = func(machine string, a agentAddr, remotePath string, got agentDeployArtifact, stderr io.Writer) error {
		st.transfer++
		st.gotPath, st.gotBytes = remotePath, len(got.Bytes)
		return nil
	}
	agentDeployRestartFn = func(machine string, a agentAddr) error { st.restart++; return nil }
	agentDeployProbeFn = func(a agentAddr) agentDeploySelf { st.probe++; return self }
	agentDeployFleetSHAFn = func(machine string) string { return fleetSHA }
	agentDeploySleep = func(d time.Duration) {}
	return st
}

// 正控 A：缺 `--confirm` ⇒ 退 2，零副作用（✗ 不发远端动作、✗ 不写审计）。
func TestAgentDeploy_MissingConfirmIsFailClosed(t *testing.T) {
	adRepoRoot(t)
	art, _, _ := adArtifact("/tmp/fake-zerg-agentd", "ubuntu")
	st := adInstallSeams(t, art, agentDeploySelf{Reachable: true, SHA: art.Commit, Digest: art.Digest()}, art.Commit)

	var out, errb bytes.Buffer
	rc := cmdAgentDeploy(adInv("edge-node", "/tmp/fake"), &out, &errb)
	if rc != exitUsage {
		t.Fatalf("缺 --confirm 应退 2，实得 %d · stderr=%s", rc, errb.String())
	}
	if st.transfer != 0 || st.restart != 0 {
		t.Fatalf("缺确认 ⇒ 一个远端动作都不许发，实得 transfer=%d restart=%d", st.transfer, st.restart)
	}
	if _, err := os.Stat(os.Getenv("ZERG_AGENT_DEPLOY_AUDIT")); err == nil {
		t.Fatal("缺确认 ⇒ 审计件不许落（零副作用）")
	}
	if !strings.Contains(errb.String(), "confirm_required") {
		t.Fatalf("拒执理由应点名 confirm_required，实得 stderr=%q", errb.String())
	}
}

// 正控 B：`--dry-run` ⇒ 退 0 且出计划件；零副作用（✗ 不发远端动作、✗ 不写审计）。
func TestAgentDeploy_DryRunPrintsPlanAndTouchesNothing(t *testing.T) {
	adRepoRoot(t)
	art, _, _ := adArtifact("/tmp/fake-zerg-agentd", "ubuntu")
	st := adInstallSeams(t, art, agentDeploySelf{Reachable: true, SHA: "old-sha"}, art.Commit)

	inv := adInv("edge-node", "/tmp/fake")
	inv.dryRun = true
	var out, errb bytes.Buffer
	if rc := cmdAgentDeploy(inv, &out, &errb); rc != exitOK {
		t.Fatalf("干跑应退 0，实得 %d · stderr=%s", rc, errb.String())
	}
	if st.transfer != 0 || st.restart != 0 {
		t.Fatalf("干跑**不许**动远端（零副作用），实得 transfer=%d restart=%d", st.transfer, st.restart)
	}
	if _, err := os.Stat(os.Getenv("ZERG_AGENT_DEPLOY_AUDIT")); err == nil {
		t.Fatal("干跑 ⇒ 审计件不许落（零副作用）")
	}
	plan := out.String()
	for _, want := range []string{
		"计划件", "/home/<user>/zerg/bin/zerg-agentd", art.SHA256, art.Commit,
		"zerg-agentd.new", "mv -f", "systemctl restart edge-node-agent.service",
		"✗ scp 直覆盖", "✗ systemctl stop", "✗ sudo",
	} {
		if !strings.Contains(plan, want) {
			t.Fatalf("计划件应含 %q，实得 stdout=%q", want, plan)
		}
	}
}

// 正控 C：真换（接缝）⇒ 退 0，落地 = `<prefix>/zerg-agentd`，身份判词 verified。
func TestAgentDeploy_RealRunVerifiesIdentity(t *testing.T) {
	adRepoRoot(t)
	art, _, _ := adArtifact("/tmp/fake-zerg-agentd", "ubuntu")
	st := adInstallSeams(t, art, agentDeploySelf{Reachable: true, SHA: art.Commit, Digest: art.Digest()}, art.Commit)

	inv := adInv("edge-node", "/tmp/fake")
	inv.confirmGiven, inv.confirm, inv.yes = true, "edge-node", true
	var out, errb bytes.Buffer
	if rc := cmdAgentDeploy(inv, &out, &errb); rc != exitOK {
		t.Fatalf("真换应退 0，实得 %d · stderr=%s", rc, errb.String())
	}
	if st.transfer != 1 || st.restart != 1 {
		t.Fatalf("应恰好换件一次、重启一次，实得 transfer=%d restart=%d", st.transfer, st.restart)
	}
	if st.gotPath != "/home/<user>/zerg/bin/zerg-agentd" {
		t.Fatalf("落地路径应取自 updates.nodes 的 prefix，实得 %q", st.gotPath)
	}
	if !strings.Contains(out.String(), "verified") {
		t.Fatalf("人面应报身份判词 verified，实得 stdout=%q", out.String())
	}
	if !strings.Contains(errb.String(), "换前") || !strings.Contains(errb.String(), "身份跟上") {
		t.Fatalf("应打出换前/换后两态，实得 stderr=%q", errb.String())
	}
}

// 负控 ②：换完身份**没跟上** ⇒ 退 1（✗ 报绿）。
func TestAgentDeploy_IdentityMismatchIsRed(t *testing.T) {
	adRepoRoot(t)
	art, _, _ := adArtifact("/tmp/fake-zerg-agentd", "ubuntu")
	adInstallSeams(t, art, agentDeploySelf{Reachable: true, SHA: "0940abe0c", Digest: art.Digest()}, art.Commit)

	inv := adInv("edge-node", "/tmp/fake")
	inv.confirmGiven, inv.confirm, inv.yes = true, "edge-node", true
	var out, errb bytes.Buffer
	if rc := cmdAgentDeploy(inv, &out, &errb); rc != exitFail {
		t.Fatalf("身份没跟上应退 1，实得 %d · stderr=%s", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "身份没跟上") {
		t.Fatalf("应点名「身份没跟上」，实得 stderr=%q", errb.String())
	}
}

// 负控 ②b：`code_sha` 同、但**字节**不同（同提交的两份不同件）⇒ 也要退 1（靠 code_digest 抓住）。
func TestAgentDeploy_SameCommitDifferentBytesIsRed(t *testing.T) {
	adRepoRoot(t)
	art, _, _ := adArtifact("/tmp/fake-zerg-agentd", "ubuntu")
	adInstallSeams(t, art, agentDeploySelf{Reachable: true, SHA: art.Commit, Digest: "ffffffffffffffff"}, art.Commit)

	inv := adInv("edge-node", "/tmp/fake")
	inv.confirmGiven, inv.confirm, inv.yes = true, "edge-node", true
	var out, errb bytes.Buffer
	if rc := cmdAgentDeploy(inv, &out, &errb); rc != exitFail {
		t.Fatalf("同提交不同字节应退 1，实得 %d · stderr=%s", rc, errb.String())
	}
	if !strings.Contains(errb.String(), "code_digest") {
		t.Fatalf("拒执理由应点名 code_digest，实得 stderr=%q", errb.String())
	}
}

// 负控 ①：坏件（普通文本）⇒ 退 1，且不 ssh、不重启（走**真**读件面）。
func TestAgentDeploy_BadArtifactRejectedBeforeAnySSH(t *testing.T) {
	adRepoRoot(t)
	art, _, _ := adArtifact("/tmp/fake-zerg-agentd", "ubuntu")
	st := adInstallSeams(t, art, agentDeploySelf{Reachable: true, SHA: art.Commit, Digest: art.Digest()}, art.Commit)
	// 恢复真读件面（其余接缝仍替换 ⇒ 没走到它们就说明在本地就拒了）
	agentDeployReadArtifactFn = agentDeployReadArtifact

	bad := filepath.Join(t.TempDir(), "zerg-agentd")
	if err := os.WriteFile(bad, bytes.Repeat([]byte("not-a-binary\n"), 200000), 0o755); err != nil {
		t.Fatal(err)
	}
	inv := adInv("edge-node", bad)
	inv.confirmGiven, inv.confirm, inv.yes = true, "edge-node", true
	var out, errb bytes.Buffer
	if rc := cmdAgentDeploy(inv, &out, &errb); rc != exitFail {
		t.Fatalf("坏件应退 1，实得 %d · stderr=%s", rc, errb.String())
	}
	if st.transfer != 0 || st.restart != 0 {
		t.Fatalf("坏件 ⇒ 一个远端动作都不许发，实得 transfer=%d restart=%d", st.transfer, st.restart)
	}
	if !strings.Contains(errb.String(), "拒换") {
		t.Fatalf("应点名「拒换」，实得 stderr=%q", errb.String())
	}
}

// 负控 ①b：真 Go 二进制但**不是子端守护件** ⇒ 退 1（拿测试二进制本身当件）。
func TestAgentDeploy_NonDaemonBinaryRejected(t *testing.T) {
	adRepoRoot(t)
	art, _, _ := adArtifact("/tmp/fake-zerg-agentd", "ubuntu")
	st := adInstallSeams(t, art, agentDeploySelf{Reachable: true, SHA: art.Commit, Digest: art.Digest()}, art.Commit)
	// 真读件面：这条负控判的正是「件本身不是子端守护件」
	agentDeployReadArtifactFn = agentDeployReadArtifact

	exe, err := os.Executable()
	if err != nil {
		t.Skipf("取不到本测试二进制：%v", err)
	}
	inv := adInv("edge-node", exe)
	inv.confirmGiven, inv.confirm, inv.yes = true, "edge-node", true
	var out, errb bytes.Buffer
	if rc := cmdAgentDeploy(inv, &out, &errb); rc != exitFail {
		t.Fatalf("非守护件应退 1，实得 %d · stderr=%s", rc, errb.String())
	}
	if st.transfer != 0 {
		t.Fatalf("非守护件 ⇒ 不许 ssh，实得 transfer=%d", st.transfer)
	}
}

// 正控 D：`--file` 缺 ⇒ 退 2；`--confirm` 值不匹配 ⇒ 退 2。
func TestAgentDeploy_UsageGates(t *testing.T) {
	adRepoRoot(t)
	art, _, _ := adArtifact("/tmp/fake-zerg-agentd", "ubuntu")
	st := adInstallSeams(t, art, agentDeploySelf{Reachable: true, SHA: art.Commit, Digest: art.Digest()}, art.Commit)

	var out, errb bytes.Buffer
	if rc := cmdAgentDeploy(adInv("edge-node", ""), &out, &errb); rc != exitUsage {
		t.Fatalf("缺 --file 应退 2，实得 %d", rc)
	}
	inv := adInv("edge-node", "/tmp/fake")
	inv.confirmGiven, inv.confirm, inv.yes = true, "mini-node-2", true
	out.Reset()
	errb.Reset()
	if rc := cmdAgentDeploy(inv, &out, &errb); rc != exitUsage {
		t.Fatalf("--confirm 值不匹配应退 2，实得 %d", rc)
	}
	if st.transfer != 0 {
		t.Fatalf("用法错 ⇒ 不许动远端，实得 transfer=%d", st.transfer)
	}
}

// 件形态/目标 os 对拍：linux 目标给 Mach-O 件 ⇒ 拒（纯本地判据，无需夹具件）。
func TestAgentDeployWantKind(t *testing.T) {
	cases := []struct{ osName, want string }{
		{"ubuntu", "elf"}, {"linux", "elf"}, {"macos", "macho"}, {"darwin", "macho"}, {"plan9", ""},
	}
	for _, c := range cases {
		if got := agentDeployWantKind(c.osName); got != c.want {
			t.Fatalf("agentDeployWantKind(%q)=%q，want %q", c.osName, got, c.want)
		}
	}
	elf := []byte{0x7f, 'E', 'L', 'F', 2, 1, 1}
	if got := agentDeployMagicKind(elf); got != "elf" {
		t.Fatalf("ELF 魔数应认成 elf，实得 %q", got)
	}
	macho := []byte{0xcf, 0xfa, 0xed, 0xfe, 7, 0, 0, 1}
	if got := agentDeployMagicKind(macho); got != "macho" {
		t.Fatalf("Mach-O 魔数应认成 macho，实得 %q", got)
	}
	if got := agentDeployMagicKind([]byte("plain")); got != "" {
		t.Fatalf("普通文本不该认成二进制件，实得 %q", got)
	}
}

// 判词三态不塌成一格：verified / sha_only / mismatch / unjudgeable 各有其时。
func TestAgentDeployVerdict(t *testing.T) {
	art, _, _ := adArtifact("/tmp/fake-zerg-agentd", "ubuntu")
	cases := []struct {
		name string
		self agentDeploySelf
		want string
	}{
		{"两格对上", agentDeploySelf{Reachable: true, SHA: art.Commit, Digest: art.Digest()}, "verified"},
		{"只有提交", agentDeploySelf{Reachable: true, SHA: art.Commit}, "sha_only"},
		{"提交不同", agentDeploySelf{Reachable: true, SHA: "old", Digest: art.Digest()}, "mismatch"},
		{"字节不同", agentDeploySelf{Reachable: true, SHA: art.Commit, Digest: "ffffffffffffffff"}, "mismatch"},
		{"打不到", agentDeploySelf{Why: "打不到"}, "unjudgeable"},
	}
	for _, c := range cases {
		got, why := agentDeployVerdict(c.self, art)
		if got != c.want {
			t.Fatalf("%s：判词应为 %q，实得 %q（why=%s）", c.name, c.want, got, why)
		}
		if why == "" {
			t.Fatalf("%s：判词必须带一句原因（✗ 留空）", c.name)
		}
	}
}
