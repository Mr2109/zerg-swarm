// cli_agent_metrics_test.go —— `agent ls` / `agent show` GPU 两格「不可得示 n/a」的回归牙
// （缺口 GAP-20260925-34 剩面①②）。
//
// 为什么要有它：`agent ls` 与 `agent show` 投影的是**同一份** `/api/fleet/status` 载荷 ⇒ 两处
// 必须同一口径。改前 `agent show` 已把 macOS 的 `gpu_pct`（主控投影层缺键）与 `gpu_used_gb`
// （`vram_known=false` 的「未知」哨兵 0）改写成 `n/a`，而 `agent ls` 没做这一步 ⇒ 同一台机在
// ls 面留空串 / 印假 0、在 show 面示 `n/a`：**两套口径**，且 ls 面仍是「不反映真实值」的假数。
//
// 判据：① macOS 两格都 `n/a`；② ls 与 show 逐字相等；③ Linux（edge-node，`vram_known=true`）真值不动。
// 判据件**不连真网**：主控地址指本进程内的 `httptest` 合成主控（`newSyntheticMaster`）。
package main_test

import (
	"encoding/json"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// agentMetricsStatusJSON —— 合成 `GET /api/fleet/status`：一台 macOS（`gpu_pct` 键**缺**、
// `gpu_used_gb=0` 哨兵、`vram_known=false`）＋ 一台 Linux（`gpu_pct` 真值、`vram_known=true`）。
// 逐字照主控 `projectStatusMachines` 的投影形态（macOS 机 gpu_pct 整键不出现）。
const agentMetricsStatusJSON = `{"machines":{` +
	`"mini-node-2":{"machine":"mini-node-2","healthy":true,"cpu_pct":61.6,"gpu_used_gb":0,"vram_known":false,"backend_state":"idle","active_requests":0,"models":["m1"]},` +
	`"edge-node":{"machine":"edge-node","healthy":true,"cpu_pct":0.3,"gpu_pct":0,"gpu_used_gb":0.2,"vram_known":true,"backend_state":"idle","active_requests":1,"models":["m1"]}}}`

// agentMetricsItem 从 `--json` 包封里取出点名那台机的一行（逐字 = 机器名相等）。
func agentMetricsItem(t *testing.T, stdout, machine string) map[string]string {
	t.Helper()
	var env struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("`--json` 不是合法包封：%v · stdout=%s", err, stdout)
	}
	for _, it := range env.Items {
		if it["machine"] == machine {
			return it
		}
	}
	t.Fatalf("包封里没有 %s：stdout=%s", machine, stdout)
	return nil
}

// TestAgentLiveMetrics_LsAndShowAgreeOnUnknownGPU —— 三判据一次机检（含两态可造的合成夹具）。
func TestAgentLiveMetrics_LsAndShowAgreeOnUnknownGPU(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/status": agentMetricsStatusJSON})
	defer srv.Close()

	run := func(argv ...string) string {
		t.Helper()
		var out, errb strings.Builder
		if rc := zerg.RunForTest(argv, &out, &errb); rc != 0 {
			t.Fatalf("%v 退码 %d（要 0）· stderr=%s", argv, rc, errb.String())
		}
		return out.String()
	}
	lsOut := run("agent", "ls", "--json", "machine,gpu_pct,gpu_used_gb")
	showOut := run("agent", "show", "mini-node-2", "--json", "machine,gpu_pct,gpu_used_gb")

	lsMac := agentMetricsItem(t, lsOut, "mini-node-2")
	showMac := agentMetricsItem(t, showOut, "mini-node-2")

	// ① macOS：两格必须示 `n/a`（✗ 假 0 · ✗ 空串/缺键）
	if lsMac["gpu_pct"] != "n/a" || lsMac["gpu_used_gb"] != "n/a" {
		t.Fatalf("agent ls 对 macOS 应示 gpu_pct/gpu_used_gb 均 n/a，实得 %+v", lsMac)
	}
	// ② ls 与 show 同口径（同一份载荷 ⇒ 两处逐字相等；改前 ls 面此处不同 ⇒ 必红）
	if lsMac["gpu_pct"] != showMac["gpu_pct"] || lsMac["gpu_used_gb"] != showMac["gpu_used_gb"] {
		t.Fatalf("ls 与 show 两套口径：ls=%+v show=%+v", lsMac, showMac)
	}
	// ③ Linux（edge-node）真值不许被「一律改 n/a」误伤
	lsX3 := agentMetricsItem(t, lsOut, "edge-node")
	if lsX3["gpu_pct"] != "0" || lsX3["gpu_used_gb"] != "0.2" {
		t.Fatalf("edge-node 真值不许动，实得 %+v", lsX3)
	}
}
