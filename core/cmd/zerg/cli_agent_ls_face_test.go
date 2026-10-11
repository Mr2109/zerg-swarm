// cli_agent_ls_face_test.go —— `agent ls` **人面列 == 字段面（机读面）同源** 的回归牙
// （缺口 GAP-20261010-29 / GAP-20261010-06）。
//
// 为什么要有它：改前 `agent ls` 的人面表**另写一份硬编 8 格清单**，而命令树登记 16 格 ⇒ 人面看不到
// `active_requests`（及 models / model / backend_state / gpu_used_gb / backend_rss_gb / gpu_temp_c /
// last_seen）⇒ 「默认 ls 即判某台在不在干活」做不到。正解 = 人面列取 `fieldListOf`（与 `--json` 同源）。
//
// 判据：① 人面表头逐字 == 命令树登记的字段表（同源 · ✗ 不许第三套口径）；② 表头含 `active_requests`；
// ③ 该列的值随载荷如实出现。判据件**不连真网**：走本进程内的合成主控（`newSyntheticMaster`）。
package main_test

import (
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
)

// agentLsFaceStatusJSON —— 合成 `GET /api/fleet/status`：一台机 `active_requests=3`（判「在干活」那格）。
const agentLsFaceStatusJSON = `{"machines":{"mini-node-2":{"machine":"mini-node-2","healthy":true,"code_version":"2.5.14",` +
	`"code_sha":"abc123","cpu_pct":61.6,"gpu_used_gb":0,"vram_known":false,"backend_state":"busy",` +
	`"active_requests":3,"models":["m1"]}}}`

// TestAgentLsHumanFaceMirrorsFieldList —— 人面列必须与字段面同源（一句机检）。
func TestAgentLsHumanFaceMirrorsFieldList(t *testing.T) {
	srv := newSyntheticMaster(t, map[string]string{"/api/fleet/status": agentLsFaceStatusJSON})
	defer srv.Close()

	var out, errb strings.Builder
	if rc := zerg.RunForTest([]string{"agent", "ls"}, &out, &errb); rc != 0 {
		t.Fatalf("agent ls 退码 %d（要 0）· stderr=%s", rc, errb.String())
	}
	stdout := out.String()

	info, ok := zerg.CommandInfoOfForTest("agent ls")
	if !ok {
		t.Fatal("命令树里找不到 `agent ls`")
	}
	want := strings.Join(info.Fields, "\t")
	got := firstLine(stdout)
	if got != want {
		t.Fatalf("`agent ls` 人面列未与字段面同源：\n人面=%q\n登记=%q", got, want)
	}
	if !strings.Contains(got, "active_requests") {
		t.Fatalf("人面表头缺 `active_requests`：%q", got)
	}

	cols := strings.Split(got, "\t")
	row := agentLsFaceRow(stdout, "mini-node-2")
	if row == "" {
		t.Fatalf("人面表里没有 mini-node-2：%q", stdout)
	}
	vals := strings.Split(row, "\t")
	if len(vals) != len(cols) {
		t.Fatalf("数据行格数 %d ≠ 表头格数 %d（行=%q）", len(vals), len(cols), row)
	}
	idx := -1
	for i, c := range cols {
		if c == "active_requests" {
			idx = i
		}
	}
	if idx < 0 || agentLsFaceVal(vals, idx) != "3" {
		t.Fatalf("`active_requests` 列该为 3，实得列号=%d 值=%q（行=%q）", idx, agentLsFaceVal(vals, idx), row)
	}
}

// agentLsFaceVal 取一格的文本（越界给哨兵，便于报清是「缺列」还是「值不对」）。
func agentLsFaceVal(vals []string, i int) string {
	if i < 0 || i >= len(vals) {
		return "<无>"
	}
	return vals[i]
}

// agentLsFaceRow 取人面表里机器名那一行（非 TTY ⇒ 制表符分隔）。
func agentLsFaceRow(out, machine string) string {
	for _, ln := range strings.Split(out, "\n") {
		if f := strings.SplitN(ln, "\t", 2); len(f) == 2 && f[0] == machine {
			return ln
		}
	}
	return ""
}
