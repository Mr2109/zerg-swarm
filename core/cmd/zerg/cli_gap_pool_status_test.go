// cli_gap_pool_status_test.go —— 候选池 `status` 闭集自查的回归牙（缺口 `GAP-20260928-176` · 批2 收口）。
//
// 落点：`package main_test` + `zerg.RunForTest`（跑**当前源码**的行为，不是盘上旧制品），
// 合成状态目录（`t.TempDir()` 当状态目录）—— **不碰真 `~/.zerg/state`**、不碰任何仓内件、
// 不真跑任何危险动作。复用同族 `cli_gap_test.go` 的 `gapRun`。
//
// 钉住的一格（`readGapPool` 的池内 `status` 闭集校验）：池行的 `status` 只认设计稿 §11.3 的
// 四值闭集（`pending|promoted|expired|merged`）；越界 ⇒ **读不出来**（fail-closed · 退码 8），
// ✗ 不许静默跳过那一行（静默跳过 = `stats` 把它吞出四格外、`ls` 照原样印 ⇒ 池坏了却看着正常）。
//
// 两态：
//
//	正控（合法 status）⇒ rc=0；负控（越界 status）⇒ rc=8 且判词点名闭集。
//	同一坏池下 `ls` / `stats` 两个出口都必须 8（同一读点，不许一个吞一个不吞）。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gapPoolStatusEnv —— 合成状态目录 + 一只**手写**的候选池件（逐行 JSON 原文）。
func gapPoolStatusEnv(t *testing.T, rows ...string) (state, pool string) {
	t.Helper()
	state = t.TempDir()
	pool = filepath.Join(state, "zerg-cli-gap-candidates.jsonl")
	t.Setenv("ZERG_STATE_DIR", state)
	body := ""
	if len(rows) > 0 {
		body = strings.Join(rows, "\n") + "\n"
	}
	if err := os.WriteFile(pool, []byte(body), 0o644); err != nil {
		t.Fatalf("造池件失败：%v", err)
	}
	return
}

// gapPoolStatusRow —— 一条形状合法的池行（`status` 由调用方即时给：好值 / 越界值）。
func gapPoolStatusRow(cid, status string) string {
	return `{"cid":"` + cid + `","source":"guard","raw":"合成症状","created_at":"2026-09-28T21:00:00.000000+08:00","ttl_days":7,"status":"` + status + `","fp":"deadbeef0000"}`
}

// TestGapIdeaPoolStatusClosed —— 池内 `status` 越界 ⇒ 读不出来（退码 8 · 点名闭集）。
func TestGapIdeaPoolStatusClosed(t *testing.T) {
	// 正控：四值闭集内的 status ⇒ 正常读出（0）—— 证明牙不是「一律 8」。
	gapPoolStatusEnv(t, gapPoolStatusRow("CAND-20260928-01", "pending"))
	if rc, out, errb := gapRun(t, "gap", "idea", "ls"); rc != 0 {
		t.Fatalf("正控：合法 status 应 rc=0，得 %d（out=%q stderr=%q）", rc, out, errb)
	}

	// 负控：第二条 `status` 越界 ⇒ 8 且判词点名闭集。
	gapPoolStatusEnv(t,
		gapPoolStatusRow("CAND-20260928-01", "pending"),
		gapPoolStatusRow("CAND-20260928-02", "bogus"))
	rc, _, errb := gapRun(t, "gap", "idea", "ls")
	if rc != 8 {
		t.Fatalf("负控：池行 status 越界应 rc=8（读不出来），得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(errb, "不在闭集") {
		t.Fatalf("负控：判词要点名闭集，stderr=%q", errb)
	}
	// 同一坏池：`stats`（同一读点的另一出口）也必须 8 —— 不许一个吞、一个不吞。
	if rc2, _, _ := gapRun(t, "gap", "idea", "stats"); rc2 != 8 {
		t.Fatalf("负控：坏池下 gap idea stats 也应 rc=8，得 %d", rc2)
	}
}
