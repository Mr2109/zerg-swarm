// cli_fleet_topology_test.go —— `zerg fleet topology`（部署拓扑只读面 · 缺口 `GAP-20260926-92`）的判据件。
//
// 判什么（逐条对应缺口账内的 want / 判据）：
//
//	① **判决对**：声明轴（名册 `components` 含 `core` ⇔ 自持）＋ 证据轴（fleet 回执 converged）
//	   ＋ 矛盾轴（回执之后又被 `agent deploy` 推送）三档合成，闭集取值逐字正确；
//	② **只读**：跑一趟前后，整棵夹具树逐件 sha256 / 件数 / 元数据**逐字节不变**（本面零写盘）；
//	③ **fail-closed**：名册缺 ⇒ 退码 8（不给结论）；投影结构不认 ⇒ 1；`--json` 用法错 ⇒ 2；
//	④ **读不到不当绿**：回执目录 / 推送台账不在盘上 ⇒ 对应列写「未核」且 stderr 明报，✗ 不当「没有」；
//	⑤ **机器面**：`--json` 字段投影 / 未知字段拒收 / 包封 kind 与字段闭集；
//	⑥ **正门名**：用法串含 `fleet topology`（账内判据 `zerg code find 'fleet topology' --path core/cmd/zerg`
//	   靠这一枚字面命中 —— 本件把它钉住，防后人改用法串把判据改回「零命中」）。
//
// 本件**不连网、不 ssh、不碰真 `~/.zerg`**：一切路径走夹具（`HOME` / `ZERG_STATE_DIR` 重定向到临时目录）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─────────────────────────── 夹具 ───────────────────────────

// ftopoTestRoster —— 三台机的名册（真源形态照 `~/.zerg/contexts/default.yaml` 逐字段同形）：
//
//	alpha —— 本机（`ssh: "-"`）· 含 core ⇒ 自持声明
//	beta  —— 仅 agentd ⇒ 非自持（只能推送换件）
//	gamma —— 没写 components ⇒ 判不了（未知）
const ftopoTestRoster = `context: default
core: http://127.0.0.1:8580
token_source: file:~/.zerg/token

nodes:
  alpha:
    host: 127.0.0.1
    port: 8100
    os: macos
    kind: work
    ssh: "-"
    root: /tmp/ftopo/alpha
    prefix: /tmp/ftopo/alpha/bin
    api: "http://127.0.0.1:8580"
    components: "core,agentd"
    role: node

  beta:
    host: 192.0.2.9
    port: 8100
    os: ubuntu
    kind: ai
    ssh: g01@192.0.2.9
    root: /home/<user>/zerg
    prefix: /home/<user>/zerg/bin
    api: ""
    components: agentd
    role: node

  gamma:
    host: 198.51.100.7
    port: 8100
    os: macos
    kind: mini
    ssh: mini@198.51.100.7
    root: /tmp/ftopo/gamma
    prefix: /tmp/ftopo/gamma/bin
    api: ""
    role: node
`

// ftopoTestFleet —— 投影件（与上面名册逐字段相符 · 漂移 0）。
const ftopoTestFleet = `auth:
  token: ""
updates:
  check: true
  nodes:
    - { name: alpha, ssh: "-", root: /tmp/ftopo/alpha, prefix: /tmp/ftopo/alpha/bin, api: "http://127.0.0.1:8580", components: "core,agentd" }
    - { name: beta, ssh: g01@192.0.2.9, root: /home/<user>/zerg, prefix: /home/<user>/zerg/bin, api: "", components: agentd }
    - { name: gamma, ssh: mini@198.51.100.7, root: /tmp/ftopo/gamma, prefix: /tmp/ftopo/gamma/bin, api: "", components: "" }
models:
fleet:
  alpha: { host: 127.0.0.1, port: 8100, os: macos, kind: work }
  beta: { host: 192.0.2.9, port: 8100, os: ubuntu, kind: ai }
  gamma: { host: 198.51.100.7, port: 8100, os: macos, kind: mini }
`

// ftopoTestReceipt —— 一份 fleet 回执（`kind=fleet` · per-machine-source-update）：
// alpha 收敛、beta 没收敛（未收敛 ⇒ 不让它算「自持证据」）。
const ftopoTestReceipt = `{
  "schema": 2, "kind": "fleet", "at": "20261001T000000Z", "mode": "per-machine-source-update",
  "nodes": [
    {"name": "alpha", "status": "ok", "detail": "", "converged": true,
     "source_remote": "/tmp/ftopo/alpha-relay.git", "code_sha_after": "aaaa1111"},
    {"name": "beta", "status": "not-converged", "detail": "已在该机完成源码式更新（内核已接手换装）",
     "converged": false, "source_remote": "/home/<user>/zerg-relay.git", "code_sha_after": "bbbb2222"}
  ],
  "matrix_before": {"alpha": {"code_sha": "old", "code_version": "2.5.14"}},
  "matrix_after": {"alpha": {"code_sha": "aaaa1111", "code_version": "2.5.15"}}
}`

// ftopoTestLedger —— 推送台账（`agent_deploy_audit.jsonl` 形态）：beta 一条已核推送（executing 那行不算）。
const ftopoTestLedger = `{"at":"2026-10-02T10:00:00+08:00","event":"executing","by":"(未声明)","machine":"beta","file":"bin/zerg-agentd-linux-amd64","rc":0,"note":"换件前一行"}
{"at":"2026-10-02T10:00:07+08:00","event":"verified","by":"(未声明)","machine":"beta","file":"bin/zerg-agentd-linux-amd64","identity":"verified","rc":0,"note":"code_sha 与 code_digest 两格都对上"}
`

// ftopoFixture —— 夹具树：`$HOME/.zerg/**` 全部重定向到临时目录（✗ 不碰真 `~/.zerg`）。
type ftopoFixture struct {
	home       string
	rosterPath string
	fleetPath  string
	receipts   string
	state      string
}

// newFtopoFixture —— 建树（roster/fleet 可由调用方覆盖 ⇒ 漂移用例改一处即可）。
func newFtopoFixture(t *testing.T, roster, fleet string, receipts map[string]string, ledger string) ftopoFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZERG_STATE_DIR", filepath.Join(home, ".zerg", "state"))
	f := ftopoFixture{
		home:       home,
		rosterPath: filepath.Join(home, ".zerg", "contexts", "default.yaml"),
		fleetPath:  filepath.Join(home, "gateway", "fleet.yaml"),
		receipts:   filepath.Join(home, ".zerg", "update_receipts"),
		state:      filepath.Join(home, ".zerg", "state"),
	}
	for _, d := range []string{filepath.Dir(f.rosterPath), filepath.Dir(f.fleetPath), f.receipts, f.state} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("建夹具目录 %s 失败：%v", d, err)
		}
	}
	if roster != "" {
		if err := os.WriteFile(f.rosterPath, []byte(roster), 0o644); err != nil {
			t.Fatalf("写名册失败：%v", err)
		}
	}
	if fleet != "" {
		if err := os.WriteFile(f.fleetPath, []byte(fleet), 0o644); err != nil {
			t.Fatalf("写投影件失败：%v", err)
		}
	}
	for name, body := range receipts {
		if err := os.WriteFile(filepath.Join(f.receipts, name), []byte(body), 0o644); err != nil {
			t.Fatalf("写回执 %s 失败：%v", name, err)
		}
	}
	if ledger != "" {
		if err := os.WriteFile(filepath.Join(f.state, "agent_deploy_audit.jsonl"), []byte(ledger), 0o644); err != nil {
			t.Fatalf("写推送台账失败：%v", err)
		}
	}
	return f
}

// inv —— 造一枚 invocation（与派发层给的那枚同形：path + kv + tty=false）。
func (f ftopoFixture) inv(extra ...string) *invocation {
	inv := &invocation{
		path: []string{"fleet", "topology"},
		kv: map[string][]string{
			"--roster": {f.rosterPath},
			"--path":   {f.fleetPath},
		},
	}
	for _, a := range extra {
		name, val, _ := strings.Cut(a, "=")
		inv.kv[name] = append(inv.kv[name], val)
	}
	return inv
}

// run —— 跑一趟人面，回 (rc, stdout, stderr)。
func (f ftopoFixture) run(t *testing.T, inv *invocation) (int, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := cmdFleetTopology(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// rowOf —— 人面 TSV 里某机那一行（列序：machine/components/自持/通道/最近自更新/最近推送/判决）。
func ftopoRowOf(t *testing.T, stdout, machine string) []string {
	t.Helper()
	for i, line := range strings.Split(stdout, "\n") {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if cols[0] == machine {
			return cols
		}
	}
	t.Fatalf("人面表里没有 %q：%q", machine, stdout)
	return nil
}

// treeDigest —— 整棵树的「件集 + 逐件 sha256 + 权限位」指纹（只读证明用）。
func ftopoTreeDigest(t *testing.T, root string) string {
	t.Helper()
	var parts []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			parts = append(parts, "D "+rel)
			return nil
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		sum := sha256.Sum256(raw)
		fi, _ := d.Info()
		parts = append(parts, "F "+rel+" "+hex.EncodeToString(sum[:])+" "+fi.Mode().String())
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s 失败：%v", root, err)
	}
	return strings.Join(parts, "\n")
}

// ─────────────────────────── ① 判决闭集 ───────────────────────────

func TestFleetTopologyVerdictTable(t *testing.T) {
	converged := ftopoReceipt{At: "20261001T000000Z", Converged: true}
	notConverged := ftopoReceipt{At: "20261001T000000Z", Converged: false}
	pushBefore := ftopoPush{At: "20260901T000000Z", File: "bin/zerg-agentd"}
	pushAfter := ftopoPush{At: "20261005T000000Z", File: "bin/zerg-agentd"}
	none := ftopoReceipt{}

	cases := []struct {
		name    string
		comps   string
		rec     ftopoReceipt
		haveRec bool
		push    ftopoPush
		have    bool
		want    string
	}{
		{"含core+回执收敛+回执后无推送 ⇒ 自持", "core,agentd", converged, true, pushBefore, true, fleetTopoSelfHosted},
		{"含core+回执收敛+无任何推送 ⇒ 自持", "core,agentd", converged, true, ftopoPush{}, false, fleetTopoSelfHosted},
		{"含core+无回执 ⇒ 自持未核", "core,agentd", none, false, ftopoPush{}, false, fleetTopoSelfHostedUnverified},
		{"含core+回执没收敛 ⇒ 自持未核", "core,agentd", notConverged, true, ftopoPush{}, false, fleetTopoSelfHostedUnverified},
		{"含core+回执收敛+之后又被推送 ⇒ 矛盾", "core,agentd", converged, true, pushAfter, true, fleetTopoPushAfterSelfUpdate},
		{"仅agentd ⇒ 非自持（推送）", "agentd", converged, true, ftopoPush{}, false, fleetTopoPushOnly},
		{"仅agentd+有推送 ⇒ 非自持（推送）", "agentd", none, false, pushAfter, true, fleetTopoPushOnly},
		{"没写 components ⇒ 未知（✗ 不猜）", "", converged, true, pushAfter, true, fleetTopoUnknown},
		{"components 只有空白 ⇒ 未知", "   ", converged, true, ftopoPush{}, false, fleetTopoUnknown},
	}
	for _, c := range cases {
		if got := ftopoVerdict(c.comps, c.rec, c.haveRec, c.push, c.have); got != c.want {
			t.Errorf("%s：判决 %q（要 %q）", c.name, got, c.want)
		}
	}
}

// TestFleetTopologyCoreMatch —— `components` 解析：逗号分隔 / 空白 / 大小写；✗ 子串误判（`corex` 不算 core）。
func TestFleetTopologyCoreMatch(t *testing.T) {
	for _, s := range []string{"core", "core,agentd", " agentd , CORE ", "Core"} {
		if !ftopoHasCore(s) {
			t.Errorf("%q 应判含 core", s)
		}
	}
	for _, s := range []string{"agentd", "corex", "cores", ""} {
		if ftopoHasCore(s) {
			t.Errorf("%q 不应判含 core", s)
		}
	}
}

// ─────────────────────────── ② 人面判决（端到端） ───────────────────────────

func TestFleetTopologyCmdHumanVerdicts(t *testing.T) {
	f := newFtopoFixture(t, ftopoTestRoster, ftopoTestFleet,
		map[string]string{"fleet-20261001T000000Z.json": ftopoTestReceipt}, ftopoTestLedger)

	rc, out, errb := f.run(t, f.inv())
	if rc != exitOK {
		t.Fatalf("退码 %d（要 0）· stderr=%s", rc, errb)
	}

	// 三台各一行，逐机判决
	alpha := ftopoRowOf(t, out, "alpha")
	if alpha[2] != "自持" || alpha[3] != "源码自更新" || alpha[6] != "自持" {
		t.Errorf("alpha 行不对（自持/通道/判决 = %q/%q/%q）：%v", alpha[2], alpha[3], alpha[6], alpha)
	}
	if alpha[4] != "20261001T000000Z" {
		t.Errorf("alpha 的最近自更新应取回执 at（要 20261001T000000Z · 得 %q）", alpha[4])
	}
	beta := ftopoRowOf(t, out, "beta")
	if beta[2] != "非自持" || beta[3] != "推送换件" || beta[6] != "非自持（推送）" {
		t.Errorf("beta 行不对（自持/通道/判决 = %q/%q/%q）：%v", beta[2], beta[3], beta[6], beta)
	}
	if !strings.HasPrefix(beta[5], "2026-10-02T10:00:07") {
		t.Errorf("beta 的最近推送应取台账 verified 那条（得 %q）", beta[5])
	}
	gamma := ftopoRowOf(t, out, "gamma")
	if gamma[6] != "未知" || gamma[3] != "未声明" {
		t.Errorf("gamma 行不对（通道/判决 = %q/%q）：%v", gamma[3], gamma[6], gamma)
	}

	// 汇总行（人面可照抄）
	for _, want := range []string{"合计 3 台", "自持 1", "非自持（推送）1", "未知 1", "名册⟷投影漂移 0 项"} {
		if !strings.Contains(errb, want) {
			t.Errorf("stderr 缺汇总片段 %q：\n%s", want, errb)
		}
	}
	// 真源路径点名（可复跑定位）
	if !strings.Contains(errb, f.rosterPath) || !strings.Contains(errb, f.fleetPath) {
		t.Errorf("stderr 未点名真源件：\n%s", errb)
	}
}

// TestFleetTopologyPassedAfterSelfUpdate —— 回执收敛之后又被推送 ⇒ 矛盾档（口径「不再推预编译制品」对不上）。
func TestFleetTopologyPassedAfterSelfUpdate(t *testing.T) {
	ledger := ftopoTestLedger +
		`{"at":"2026-10-20T09:00:00+08:00","event":"verified","by":"(未声明)","machine":"alpha","file":"bin/zerg-agentd","identity":"verified","rc":0}` + "\n"
	f := newFtopoFixture(t, ftopoTestRoster, ftopoTestFleet,
		map[string]string{"fleet-20261001T000000Z.json": ftopoTestReceipt}, ledger)

	rc, out, errb := f.run(t, f.inv())
	if rc != exitOK {
		t.Fatalf("退码 %d（要 0 —— 判决是读数，不是失败）· stderr=%s", rc, errb)
	}
	alpha := ftopoRowOf(t, out, "alpha")
	if alpha[6] != "自持但另有推送" {
		t.Errorf("alpha 判决要「自持但另有推送」（回执 20261001 之后 2026-10-20 有推送）· 得 %q", alpha[6])
	}
	if !strings.Contains(errb, "自持但另有推送 1") {
		t.Errorf("汇总未计入矛盾档：\n%s", errb)
	}
}

// TestFleetTopologyLocalAlias —— 回执把本机记作 `local`（`zerg-upgrade.sh --fleet` 的节点名）
// ⇒ 必须归并到名册里 `ssh: "-"` 的那台（否则本机自更新证据整条丢）。
func TestFleetTopologyLocalAlias(t *testing.T) {
	roster := strings.Replace(ftopoTestRoster, "components: \"core,agentd\"", "components: \"core,agentd\"", 1)
	f := newFtopoFixture(t, roster, ftopoTestFleet, map[string]string{
		"fleet-20261001T000000Z.json": `{"schema":2,"kind":"fleet","at":"20261001T000000Z","mode":"per-machine-source-update",
			"nodes":[{"name":"local","status":"ok","converged":true,"source_remote":"/tmp/relay.git","code_sha_after":"cccc"}]}`,
	}, "")

	rc, out, errb := f.run(t, f.inv())
	if rc != exitOK {
		t.Fatalf("退码 %d（要 0）· stderr=%s", rc, errb)
	}
	alpha := ftopoRowOf(t, out, "alpha")
	if alpha[4] != "20261001T000000Z" || alpha[6] != "自持" {
		t.Errorf("本机别名 `local` 未归并到 alpha（最近自更新=%q 判决=%q）", alpha[4], alpha[6])
	}
}

// ─────────────────────────── ③ 只读证明 ───────────────────────────

func TestFleetTopologyLeavesTreeUntouched(t *testing.T) {
	f := newFtopoFixture(t, ftopoTestRoster, ftopoTestFleet,
		map[string]string{"fleet-20261001T000000Z.json": ftopoTestReceipt}, ftopoTestLedger)

	before := ftopoTreeDigest(t, f.home)
	for i := 0; i < 2; i++ { // 跑两趟（幂等面同一读数 · ✗ 第二趟改了什么）
		if rc, _, errb := f.run(t, f.inv()); rc != exitOK {
			t.Fatalf("第 %d 趟退码 %d · stderr=%s", i+1, rc, errb)
		}
	}
	after := ftopoTreeDigest(t, f.home)
	if before != after {
		t.Fatalf("本面**动了盘**（只读面零写铁律）：\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
}

// ─────────────────────────── ④ fail-closed / 读不到不当绿 ───────────────────────────

func TestFleetTopologyRosterAbsent(t *testing.T) {
	f := newFtopoFixture(t, "", ftopoTestFleet, nil, "")
	rc, out, errb := f.run(t, f.inv())
	if rc != exitBlocked {
		t.Fatalf("名册不在 ⇒ 退码 %d（要 8 · 读不到不许当绿）· stderr=%s", rc, errb)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("不给结论时 stdout 应 0 字节（得 %q）", out)
	}
	if !strings.Contains(errb, "退码 8") {
		t.Errorf("stderr 未明报退码 8：\n%s", errb)
	}
}

func TestFleetTopologyProjectionAbsent(t *testing.T) {
	f := newFtopoFixture(t, ftopoTestRoster, "", nil, "")
	rc, _, errb := f.run(t, f.inv())
	if rc != exitBlocked {
		t.Fatalf("投影件不在 ⇒ 退码 %d（要 8）· stderr=%s", rc, errb)
	}
}

func TestFleetTopologyProjectionShapeUnknown(t *testing.T) {
	f := newFtopoFixture(t, ftopoTestRoster, "auth:\n  token: \"\"\n", nil, "")
	rc, _, errb := f.run(t, f.inv())
	if rc != exitFail {
		t.Fatalf("投影件缺两段 ⇒ 退码 %d（要 1）· stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "结构不认") {
		t.Errorf("stderr 未点名结构问题：\n%s", errb)
	}
}

// TestFleetTopologyEvidenceAbsentIsNotGreen —— 回执目录/台账都不在盘上：
// 退码仍 0（名册投影读到了，判决能出），但两列必须写「未核」且 stderr 明报 —— ✗ 不许静默当「从没有过」。
func TestFleetTopologyEvidenceAbsentIsNotGreen(t *testing.T) {
	f := newFtopoFixture(t, ftopoTestRoster, ftopoTestFleet, nil, "")
	if err := os.RemoveAll(f.receipts); err != nil {
		t.Fatalf("拆回执目录失败：%v", err)
	}
	rc, out, errb := f.run(t, f.inv())
	if rc != exitOK {
		t.Fatalf("退码 %d（要 0）· stderr=%s", rc, errb)
	}
	alpha := ftopoRowOf(t, out, "alpha")
	if !strings.Contains(alpha[4], "未核") || !strings.Contains(alpha[5], "未核") {
		t.Errorf("证据不在盘上时两列须写「未核」（得 %q / %q）", alpha[4], alpha[5])
	}
	if alpha[6] != "自持（未核）" {
		t.Errorf("无回执 ⇒ 判决须是「自持（未核）」（得 %q）", alpha[6])
	}
	for _, want := range []string{"回执目录不在", "推送台账不在", "✗ **不**当「从没推过」"} {
		if !strings.Contains(errb, want) {
			t.Errorf("stderr 缺「读不到不当绿」的明报片段 %q：\n%s", want, errb)
		}
	}
}

// ─────────────────────────── ⑤ 漂移对拍（`--expect-no-changes`） ───────────────────────────

func TestFleetTopologyDriftAssertion(t *testing.T) {
	drifted := strings.Replace(ftopoTestFleet, "prefix: /home/<user>/zerg/bin", "prefix: /home/<user>/WRONG/bin", 1)
	f := newFtopoFixture(t, ftopoTestRoster, drifted,
		map[string]string{"fleet-20261001T000000Z.json": ftopoTestReceipt}, "")

	// 不断言 ⇒ 只读面恒 0（漂移是读数，不是失败）
	rc, out, errb := f.run(t, f.inv())
	if rc != exitOK {
		t.Fatalf("不带断言时退码 %d（要 0）· stderr=%s", rc, errb)
	}
	beta := ftopoRowOf(t, out, "beta")
	if beta[6] != "非自持（推送）" { // 漂移不改判决轴
		t.Errorf("漂移不应影响判决列（要「非自持（推送）」· 得 %q）", beta[6])
	}
	betaDrift := ftopoRowOf(t, out, "beta")
	if betaDrift[7] != "1" {
		t.Errorf("beta 的漂移列应 1（投影 prefix 被改坏 · 得 %q）", betaDrift[7])
	}
	if !strings.Contains(errb, "名册 ⟷ 投影有 1 项漂移") {
		t.Errorf("stderr 未报漂移条数：\n%s", errb)
	}

	// 带断言 ⇒ 1，且逐项点名（现读/名册两侧值）
	rc2, _, errb2 := f.run(t, f.inv("--expect-no-changes"))
	if rc2 != exitFail {
		t.Fatalf("`--expect-no-changes` 遇漂移 ⇒ 退码 %d（要 1）· stderr=%s", rc2, errb2)
	}
	if !strings.Contains(errb2, "/home/<user>/WRONG/bin") || !strings.Contains(errb2, "/home/<user>/zerg/bin") {
		t.Errorf("漂移明细应两侧具名（现读 ⟷ 名册）：\n%s", errb2)
	}
}

// ─────────────────────────── ⑥ 机器面（--json） ───────────────────────────

func TestFleetTopologyJSONProjection(t *testing.T) {
	f := newFtopoFixture(t, ftopoTestRoster, ftopoTestFleet,
		map[string]string{"fleet-20261001T000000Z.json": ftopoTestReceipt}, ftopoTestLedger)

	inv := f.inv()
	inv.jsonGiven = true
	inv.fields = []string{"machine", "self_host", "verdict", "drift"}
	var out, errb strings.Builder
	if rc := cmdFleetTopology(inv, &out, &errb); rc != exitOK {
		t.Fatalf("退码 %d（要 0）· stderr=%s", rc, errb.String())
	}

	var env struct {
		Schema string                 `json:"schema"`
		Kind   string                 `json:"kind"`
		Items  []map[string]string    `json:"items"`
		Meta   map[string]interface{} `json:"meta"`
		Warns  []string               `json:"warnings"`
		Trunc  bool                   `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("`--json` 输出不是合法 JSON：%v\n%s", err, out.String())
	}
	if env.Schema != "zerg/v1" || env.Kind != "FleetTopology" {
		t.Errorf("包封头不对：schema=%q kind=%q", env.Schema, env.Kind)
	}
	if len(env.Items) != 3 {
		t.Fatalf("items 应 3 条（得 %d）：%s", len(env.Items), out.String())
	}
	want := map[string]string{"alpha": "self-hosted", "beta": "push-only", "gamma": "unknown"}
	for _, it := range env.Items {
		if len(it) != 4 {
			t.Errorf("%s 行的键数应 4（字段投影 · 得 %d）：%v", it["machine"], len(it), it)
		}
		if got := it["verdict"]; got != want[it["machine"]] {
			t.Errorf("%s 判决 %q（要 %q）", it["machine"], got, want[it["machine"]])
		}
		if _, ok := it["last_push"]; ok {
			t.Errorf("没点名的字段不许出现：%v", it)
		}
	}
	if c, _ := env.Meta["count"].(float64); int(c) != 3 {
		t.Errorf("meta.count 应 3（得 %v）", env.Meta["count"])
	}
	if ch, _ := env.Meta["changed"].(bool); ch {
		t.Error("只读面 meta.changed 必须 false")
	}
	if env.Warns == nil || len(env.Warns) != 0 {
		t.Errorf("warnings 应是空数组 []（得 %v）", env.Warns)
	}
	if env.Trunc {
		t.Error("truncated 必须 false（本面不裁条）")
	}
}

func TestFleetTopologyJSONUsageErrors(t *testing.T) {
	f := newFtopoFixture(t, ftopoTestRoster, ftopoTestFleet, nil, "")

	// ① 给 `--json` 不给字段 ⇒ 2
	inv := f.inv()
	inv.jsonGiven = true
	var out, errb strings.Builder
	if rc := cmdFleetTopology(inv, &out, &errb); rc != exitUsage {
		t.Fatalf("`--json` 不给字段 ⇒ 退码 %d（要 2）· stderr=%s", rc, errb.String())
	}
	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("用法错时 stdout 应 0 字节（得 %q）", out.String())
	}

	// ② 未知字段 ⇒ 2，且把合法闭集列出来（可照抄）
	inv2 := f.inv()
	inv2.jsonGiven = true
	inv2.fields = []string{"machine", "cpu_pct"}
	var out2, errb2 strings.Builder
	if rc := cmdFleetTopology(inv2, &out2, &errb2); rc != exitUsage {
		t.Fatalf("未知字段 ⇒ 退码 %d（要 2）· stderr=%s", rc, errb2.String())
	}
	if !strings.Contains(errb2.String(), "cpu_pct") || !strings.Contains(errb2.String(), "self_host") {
		t.Errorf("未知字段面应点名坏字段并列出闭集：\n%s", errb2.String())
	}
}

// ─────────────────────────── ⑦ 正门名（账内判据的钉子） ───────────────────────────

// TestFleetTopologyRegisteredRowMatches —— **登记后的对拍牙**（本笔不注册 ⇒ 现在自动空跑，登记那一路一插入即生效）：
// 命令树里那一行的 `usage` / `fields` 必须与本件的 `fleetTopoUsage` / `fleetTopoJSONFields` **逐字相同**
// （两处各写一份字面量是契约侧 `FIELDS_RE`/`USAGE_RE` 的硬要求 ⇒ 靠这条测试防「登记时抄错一格」）。
func TestFleetTopologyRegisteredRowMatches(t *testing.T) {
	cmd := find([]string{"fleet", "topology"})
	if cmd == nil {
		t.Log("`fleet topology` 尚未登记进命令树（本笔只落件不注册）—— 登记后本测试自动转为对拍牙")
		return
	}
	if cmd.usage != fleetTopoUsage {
		t.Errorf("登记行的 usage 与本件不一致：\n登记=%q\n本件=%q", cmd.usage, fleetTopoUsage)
	}
	if strings.Join(cmd.fields, ",") != strings.Join(fleetTopoJSONFields, ",") {
		t.Errorf("登记行的 fields 与本件不一致：\n登记=%v\n本件=%v", cmd.fields, fleetTopoJSONFields)
	}
	if cmd.kind != "FleetTopology" {
		t.Errorf("kind 应 FleetTopology（得 %q）", cmd.kind)
	}
	if cmd.run == nil {
		t.Error("登记行缺 run（命令不可执行）")
	}
}

// TestFleetTopologyUsageCarriesFaceName —— 账内现判据 = `zerg code find 'fleet topology' --path core/cmd/zerg`。
// 用法串（登记时搬进命令树那一行）必须含这枚字面，否则判据又回「零命中」。
func TestFleetTopologyUsageCarriesFaceName(t *testing.T) {
	if !strings.Contains(fleetTopoUsage, "fleet topology") {
		t.Fatalf("用法串必须含 `fleet topology`（账内判据靠它命中）：%q", fleetTopoUsage)
	}
	if !strings.HasPrefix(fleetTopoUsage, "zerg ") {
		t.Fatalf("用法串须以 `zerg ` 起头（门形）：%q", fleetTopoUsage)
	}
	// 字段闭集必须是**闭集**：`--json` 的每个字段都在闭集里（防「登记了取不出」）
	for _, f := range []string{"machine", "components", "self_host", "channel", "last_self_update", "last_push", "verdict", "drift"} {
		if !ftopoFieldOK(f) {
			t.Errorf("字段闭集缺 %q", f)
		}
	}
	if ftopoFieldOK("cpu_pct") {
		t.Error("字段闭集不该收 cpu_pct（本面无该读数）")
	}
}
