// cli_agent_ls_roster_test.go —— `agent ls` 与**名册**逐台对齐的回归牙（缺口 `GAP-20261021-08`）。
//
// 病根（改前三态）：`/api/fleet/status` 的 `machines` 是**心跳快照表**（`core/internal/store/store.go:138`
// 只在 `ReceiveHeartbeat` 里建行），主控 `StatusHandler`（`core/internal/api/handlers.go:1224`）不与名册并集
// ⇒ 名册里在、但此刻没有心跳的机器**整行消失**（现读真机：名册 3 台、`agent ls` 只出 2 台）。
// 名册是**声明面**（§九 M13 `Z2` 真源 · `Z6` 离线也出表）⇒ 观察面比声明面少台 = 报错面不对。
//
// 判据（每一格都要有牙）：
//
//	① 名册 3 台 + 心跳 1 台 ⇒ 出 **3 行**，逐台点名；缺心跳那两行 `backend_state=无心跳`、`healthy=unknown`，
//	   其余格**整键缺席**（✗ 不印 0 / ✗ 不印 idle —— 那是把「没数据」写成真读数）；
//	② 负控（不许造假台）：心跳 3 台 + 名册 1 台 ⇒ 仍出 3 行、无一行 `无心跳`；
//	③ 名册读不到 ⇒ 只出心跳那几台 ＋ stderr 明说「对齐判不了」（✗ 静默当 0 台 / ✗ 当绿）；
//	④ `agent show <名册有、心跳无的机>` ⇒ 退码 0 且 `backend_state=无心跳`（改前退 2 并说「名册里没有这台机」= 假话）；
//	   名册里**真没有**的机 ⇒ 仍退 2（负控：放宽接受面不许把这一档也放过去）。
//
// 夹具全部合成：`HOME`/`ZERG_STATE_DIR` 重定向到临时目录，主控地址走进程内 `httptest`（`newSyntheticMaster`）。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// agentRosterLsStatusJSON —— 合成 `/api/fleet/status`：只有 alpha 有心跳（beta/gamma 名册里有、没心跳）。
const agentRosterLsStatusJSON = `{"machines":{"alpha":{"machine":"alpha","healthy":true,"code_version":"2.5.15",` +
	`"code_sha":"aaa111","backend_state":"idle","active_requests":0,"models":["m1"]}}}`

// agentRosterLsStatus3JSON —— 负控用：心跳三台，名册只登记一台 ⇒ 不许凭空多行。
const agentRosterLsStatus3JSON = `{"machines":{` +
	`"alpha":{"machine":"alpha","healthy":true,"backend_state":"idle"},` +
	`"beta":{"machine":"beta","healthy":true,"backend_state":"idle"},` +
	`"gamma":{"machine":"gamma","healthy":true,"backend_state":"busy"}}}`

// agentRosterFixture —— 名册夹具（形态与真件 `~/.zerg/contexts/default.yaml` 同：`nodes:` 下逐台一段）。
const agentRosterFixture = `# 名册夹具（测试专用）
context: default
default_node: alpha
nodes:
  alpha:
    host: 127.0.0.1
    port: 8100
  beta:
    host: <worker-ip>
    port: 8100
  gamma:
    host: <worker-ip>
    port: 8100
`

// agentRosterOnlyAlphaFixture —— 负控名册：只登记 alpha（心跳多出来的两台不属于名册，也不许被标 无心跳）。
const agentRosterOnlyAlphaFixture = `context: default
nodes:
  alpha:
    host: 127.0.0.1
    port: 8100
`

// agentRosterHome 建一枚隔离 HOME（`roster == ""` ⇒ 不建名册件，用来看「读不到」那一档）。
func agentRosterHome(t *testing.T, roster string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZERG_STATE_DIR", filepath.Join(home, ".zerg", "state"))
	if roster == "" {
		return
	}
	dir := filepath.Join(home, ".zerg", "contexts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建名册目录失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "default.yaml"), []byte(roster), 0o644); err != nil {
		t.Fatalf("写名册夹具失败：%v", err)
	}
}

// agentLsEnvelope —— `--json` 包封（只看 items 与 meta.count）。
type agentLsEnvelope struct {
	Items []map[string]string `json:"items"`
	Meta  struct {
		Count int `json:"count"`
	} `json:"meta"`
}

// agentLsRun 跑一次 `agent ls --json machine,healthy,backend_state`，返回 (包封, stdout, stderr)。
func agentLsRun(t *testing.T) (agentLsEnvelope, string, string) {
	t.Helper()
	var out, errb strings.Builder
	rc := zerg.RunForTest([]string{"agent", "ls", "--json", "machine,healthy,backend_state"}, &out, &errb)
	if rc != 0 {
		t.Fatalf("`agent ls` 退码 %d（要 0）· stderr=%s", rc, errb.String())
	}
	var env agentLsEnvelope
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("`--json` 不是合法包封：%v · stdout=%s", err, out.String())
	}
	return env, out.String(), errb.String()
}

// agentLsByMachine —— items 摊成 name → 行。
func agentLsByMachine(env agentLsEnvelope) map[string]map[string]string {
	m := map[string]map[string]string{}
	for _, it := range env.Items {
		m[it["machine"]] = it
	}
	return m
}

// TestAgentLsRosterUnion_ThreeRosterOneHeartbeat —— 判据①：名册 3 台就出 3 行，缺心跳的两行如实标「无心跳」。
func TestAgentLsRosterUnion_ThreeRosterOneHeartbeat(t *testing.T) {
	agentRosterHome(t, agentRosterFixture)
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/status": agentRosterLsStatusJSON})
	defer srv.Close()

	env, stdout, stderr := agentLsRun(t)
	if env.Meta.Count != 3 || len(env.Items) != 3 {
		t.Fatalf("名册 3 台 ⇒ 应出 3 行，实得 count=%d items=%d · stdout=%s", env.Meta.Count, len(env.Items), stdout)
	}
	by := agentLsByMachine(env)
	for _, n := range []string{"alpha", "beta", "gamma"} {
		if by[n] == nil {
			t.Fatalf("名册里的 %s 没出在 `agent ls` 里（观察面比名册少台）：stdout=%s", n, stdout)
		}
	}
	// 有心跳的那台：真读数原样不动（✗ 不许被「补行」逻辑污染）
	if by["alpha"]["backend_state"] != "idle" || by["alpha"]["healthy"] != "true" {
		t.Fatalf("alpha 有心跳，真读数不许改：%+v", by["alpha"])
	}
	// 没心跳的两台：只有能证实的格，且必须点名「无心跳」
	for _, n := range []string{"beta", "gamma"} {
		row := by[n]
		if row["backend_state"] != "无心跳" {
			t.Fatalf("%s 无心跳 ⇒ backend_state 应为「无心跳」，实得 %+v", n, row)
		}
		if row["healthy"] != "unknown" {
			t.Fatalf("%s 无心跳 ⇒ healthy 应为 unknown（缺 ≠ 健康），实得 %+v", n, row)
		}
		if len(row) != 3 {
			t.Fatalf("%s 只许出 machine/healthy/backend_state 三格（其余缺就缺，✗ 不造数），实得 %+v", n, row)
		}
	}
	// 对齐结论要说出口（✗ 静默补行）
	for _, want := range []string{"名册 3 台", "无心跳 2 台", "beta", "gamma"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr 少了 %q（对齐结论没说出口）· stderr=%s", want, stderr)
		}
	}
}

// TestAgentLsRosterUnion_NoFabricatedRows —— 判据②负控：心跳 3 台、名册 1 台 ⇒ 3 行，且没有一行被标「无心跳」。
func TestAgentLsRosterUnion_NoFabricatedRows(t *testing.T) {
	agentRosterHome(t, agentRosterOnlyAlphaFixture)
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/status": agentRosterLsStatus3JSON})
	defer srv.Close()

	env, stdout, _ := agentLsRun(t)
	if env.Meta.Count != 3 || len(env.Items) != 3 {
		t.Fatalf("心跳 3 台 ⇒ 应出 3 行（不许凭空多/少），实得 count=%d items=%d · stdout=%s", env.Meta.Count, len(env.Items), stdout)
	}
	for _, it := range env.Items {
		if it["backend_state"] == "无心跳" {
			t.Fatalf("有心跳的机器不许被标「无心跳」：%+v", it)
		}
	}
}

// TestAgentLsRosterUnreadable_SaysAlignUndecidable —— 判据③：名册读不到 ⇒ 只出心跳台数 ＋ 明说「对齐判不了」。
func TestAgentLsRosterUnreadable_SaysAlignUndecidable(t *testing.T) {
	agentRosterHome(t, "") // 不建名册件
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/status": agentRosterLsStatusJSON})
	defer srv.Close()

	env, stdout, stderr := agentLsRun(t)
	if env.Meta.Count != 1 || len(env.Items) != 1 {
		t.Fatalf("名册读不到 ⇒ 只出心跳 1 台，实得 count=%d · stdout=%s", env.Meta.Count, stdout)
	}
	for _, want := range []string{"名册读不到", "对齐判不了"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr 少了 %q（读不到不许当绿/当 0 台）· stderr=%s", want, stderr)
		}
	}
}

// TestAgentShow_RosterMachineWithoutHeartbeat —— 判据④：名册有、心跳无 ⇒ 退 0 且标「无心跳」；名册真没有 ⇒ 仍退 2。
func TestAgentShow_RosterMachineWithoutHeartbeat(t *testing.T) {
	agentRosterHome(t, agentRosterFixture)
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/status": agentRosterLsStatusJSON})
	defer srv.Close()

	var out, errb strings.Builder
	if rc := zerg.RunForTest([]string{"agent", "show", "beta", "--json", "machine,healthy,backend_state"}, &out, &errb); rc != 0 {
		t.Fatalf("`agent show beta` 退码 %d（要 0 —— beta 在名册里）· stderr=%s", rc, errb.String())
	}
	var env agentLsEnvelope
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("`--json` 不是合法包封：%v · stdout=%s", err, out.String())
	}
	if len(env.Items) != 1 || env.Items[0]["backend_state"] != "无心跳" || env.Items[0]["healthy"] != "unknown" {
		t.Fatalf("名册有、心跳无的机器应出一行「无心跳/unknown」，实得 %+v", env.Items)
	}

	// 负控：名册里真没有的机 ⇒ 仍退 2（✗ 别把接受面放宽成「什么都收」）
	var out2, errb2 strings.Builder
	if rc := zerg.RunForTest([]string{"agent", "show", "nosuch", "--json", "machine"}, &out2, &errb2); rc != 2 {
		t.Fatalf("名册里没有的机 ⇒ 退码应 2，实得 %d · stderr=%s", rc, errb2.String())
	}
	if !strings.Contains(errb2.String(), "名册里没有这台机") {
		t.Fatalf("名册里没有的机 ⇒ 报话应点名「名册里没有这台机」· stderr=%s", errb2.String())
	}
}
