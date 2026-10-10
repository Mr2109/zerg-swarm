// cli_gap_two_state_gate_test.go —— 缺口 `GAP-20260928-195` 的机检：
// 「新账销案必须带两态读数」的**共用闸**（`gapTwoStateGate`）须覆盖**所有能改到 `已解` 的面** ——
// 单条 `set-state`（原有 · 见 `cli_gap_state_test.go`）· 批量 `bulk set-state`（本枚）·
// 判据批量面 `verify`（本枚 · 见 `cli_gap_test.go` 的负控）· 回执 `receipt apply`（本枚）。
//
// 本件钉批量面三格（它是缺口逐字点名的**第一条绕道**）：
//
//	① 正控：`bulk set-state --state 已解` 带成对两态读数 ⇒ 0 且真改态
//	② 负控：`bulk set-state --state 已解` **不带**两态读数 ⇒ 2 且**真源 sha 逐字不变 · 审计零行**（绕道被拒）
//	③ 旁证：`bulk set-state --state 仍缺`（非销案）不带读数也放行（闸判「不适用」· 行为一字不动）
//
// 全部合成盘面（`t.TempDir()` 当状态目录 · 复用 `cli_gap_test.go` / `cli_gap_state_test.go` 的
// 尺子）⇒ **不碰真 `~/.zerg/state`**。
package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGapTwoStateGateBulkSetState(t *testing.T) {
	_, ledger, audit := gapEnv(t)

	// 造两条靶子（走同族写面 `add` —— 真源只由命令写；id 由 add 生成 ⇒ **新账**）。
	if rc, _, errb := gapRun(t, append(gapAddArgv("手搓：批量销案两态闸靶子甲", "两态闸批量靶子甲"), "--yes")...); rc != 0 {
		t.Fatalf("造靶子甲失败：rc=%d stderr=%q", rc, errb)
	}
	if rc, _, errb := gapRun(t, append(gapAddArgv("手搓：批量销案两态闸靶子乙", "两态闸批量靶子乙"), "--yes")...); rc != 0 {
		t.Fatalf("造靶子乙失败：rc=%d stderr=%q", rc, errb)
	}
	ids := gapAllIDs(t, ledger)
	if len(ids) != 2 {
		t.Fatalf("靶子条数不对（要 2）：%v", ids)
	}

	brA := "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前列于仍缺"
	brB := "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后列于已解"

	// 清单件（仓外夹具 —— 合成盘面）。
	list := filepath.Join(t.TempDir(), "list.jsonl")
	if err := os.WriteFile(list, []byte("{\"id\":\""+ids[0]+"\",\"evidence\":\"批量销案一句\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// ② 负控：不带两态读数 ⇒ 2（共用闸）· 真源 sha 与审计行数**都不动**。
	sha0 := gapSHAOf(t, ledger)
	na0 := len(gapAuditLines(t, audit))
	if rc, _, errb := gapRun(t, "gap", "bulk", "set-state", list, "--state", "已解", "--yes"); rc != 2 {
		t.Errorf("负控（绕道）：`bulk set-state --state 已解` 不带两态读数要 2，得到 %d · %s", rc, errb)
	}
	if gapSHAOf(t, ledger) != sha0 || len(gapAuditLines(t, audit)) != na0 {
		t.Errorf("负控（绕道）：拒写却动了盘（真源 sha 或审计行数变了）")
	}

	// ① 正控：带成对两态读数 ⇒ 0 且真改态。
	if rc, _, errb := gapRun(t, "gap", "bulk", "set-state", list, "--state", "已解", "--yes",
		"--before-read", brA, "--after-read", brB); rc != 0 {
		t.Errorf("正控：`bulk set-state --state 已解` 带两态读数要 0，得到 %d · %s", rc, errb)
	}
	if r := gapRecOf(t, ledger, ids[0]); r["state"] != "已解" {
		t.Errorf("正控：真跑后 state 要 已解，得到 %v", r["state"])
	}

	// ③ 旁证：非销案态（仍缺）不带读数也放行（闸判「不适用」· 行为一字不动）。
	list2 := filepath.Join(t.TempDir(), "list2.jsonl")
	if err := os.WriteFile(list2, []byte("{\"id\":\""+ids[1]+"\",\"evidence\":\"不适用旁证\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, _, errb := gapRun(t, "gap", "bulk", "set-state", list2, "--state", "仍缺", "--yes"); rc != 0 {
		t.Errorf("旁证：非销案态（仍缺）不带两态读数要 0（闸判不适用），得到 %d · %s", rc, errb)
	}
	if r := gapRecOf(t, ledger, ids[1]); r["state"] != "仍缺" {
		t.Errorf("旁证：state 要 仍缺，得到 %v", r["state"])
	}
}

// gapAllIDs —— 账内全部 id（按行序）。
func gapAllIDs(t *testing.T, ledger string) []string {
	t.Helper()
	ids := []string{}
	for _, ln := range gapReadLines(t, ledger) {
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("真源某行不是 JSON：%v", err)
		}
		if s, _ := m["id"].(string); s != "" {
			ids = append(ids, s)
		}
	}
	return ids
}

// TestGapTwoStateAutoRun —— 缺口 `GAP-20260928-196`（甲）的机检：`gap set-state` 销案两态读数的
// **自动跑**两枚旗标（`--before-cmd` / `--after-cmd`）。三格（全合成盘面 · 子进程用**假 `zerg`**
// 走临时 PATH ⇒ 不碰真制品、不碰真 `~/.zerg/state`）：
//
//	① 正控：给 `--before-cmd`/`--after-cmd`（退码 0）⇒ CLI 自动跑、三格填好、被**唯一判处**判为
//	   「齐」⇒ 真改态 0，且 `solved_evidence` 里落的是**自动填**的三格（cmd= 原样串 · rc= 真退码）。
//	② 负控①：自动跑的命令**真退非 0**（rc=3）⇒ 读数**如实带 rc=3**（✗ 伪造 0 则本格真红）。
//	③ 负控②：自动跑的命令**跑不起来**（rc=127）⇒ **判不了**（退码 8），真源 sha 逐字不变、
//	   审计零行、该条仍 `仍缺`（✗ 不许当 0 用）。
func TestGapTwoStateAutoRun(t *testing.T) {
	_, ledger, audit := gapEnv(t)

	// 三条靶子（走同族写面 `add` 造 · id 由 add 生成 ⇒ **新账** · 销案必须带两态）。
	for _, sym := range []string{"两态自动跑靶子甲", "两态自动跑靶子乙", "两态自动跑靶子丙"} {
		if rc, _, errb := gapRun(t, append(gapAddArgv("手搓："+sym, sym), "--yes")...); rc != 0 {
			t.Fatalf("造靶子（%s）失败：rc=%d stderr=%q", sym, rc, errb)
		}
	}
	ids := gapAllIDs(t, ledger)
	if len(ids) != 3 {
		t.Fatalf("靶子条数不对（要 3）：%v", ids)
	}
	id1, id2, id3 := ids[0], ids[1], ids[2]

	// 合成一枚**假 `zerg`**：`--before-cmd`/`--after-cmd` 切词后首词裸 `zerg` ⇒ 走 PATH 解析到这枚
	// 假件（真跑子进程 · 不碰真制品）；退码由环境变量 `FAKE_RC` 定（脚本内 `${FAKE_RC:-0}`）。
	binDir := t.TempDir()
	fake := filepath.Join(binDir, "zerg")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"两态自动跑 · 读数行\"\nexit \"${FAKE_RC:-0}\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	// ① 正控：退码 0 ⇒ 自动填三格 ⇒ 判「齐」⇒ 真改态。
	t.Setenv("FAKE_RC", "0")
	if rc, out, errb := gapRun(t, "gap", "set-state", id1, "--state", "已解", "--evidence", "自动跑正控",
		"--before-cmd", "zerg gap ls", "--after-cmd", "zerg gap ls", "--yes"); rc != 0 {
		t.Fatalf("正控（给 --before-cmd/--after-cmd）⇒ 0，得到 %d · out=%q err=%q", rc, out, errb)
	}
	if r := gapRecOf(t, ledger, id1); r["state"] != "已解" {
		t.Errorf("正控：真跑后 state 要 已解，得到 %v", r["state"])
	} else if ev := fmt.Sprint(r["solved_evidence"]); !strings.Contains(ev, "cmd=zerg gap ls") ||
		!strings.Contains(ev, "rc=0") || !strings.Contains(ev, "reading=") {
		t.Errorf("正控：自动填的三格没落进 solved_evidence：%q", ev)
	}

	// ② 负控①：真退非 0（rc=3）⇒ 读数如实带非 0（✗ 不许伪造 0）。
	t.Setenv("FAKE_RC", "3")
	sha2 := gapSHAOf(t, ledger)
	na2 := len(gapAuditLines(t, audit))
	if rc, out, errb := gapRun(t, "gap", "set-state", id2, "--state", "已解", "--evidence", "自动跑负控①",
		"--before-cmd", "zerg gap ls", "--after-cmd", "zerg gap ls", "--yes"); rc != 0 {
		t.Fatalf("负控①（命令真退 3）⇒ 0（读数照收 · 真非 0 也是合法读数），得到 %d · out=%q err=%q", rc, out, errb)
	}
	if gapSHAOf(t, ledger) == sha2 || len(gapAuditLines(t, audit)) != na2+1 {
		t.Errorf("负控①：真退 3 应当写账（真源 sha 应变 · 审计 +1）")
	}
	ev2 := fmt.Sprint(gapRecOf(t, ledger, id2)["solved_evidence"])
	if !strings.Contains(ev2, "rc=3") {
		t.Errorf("负控①：读数没如实带真退码 rc=3（伪造 0？）：%q", ev2)
	}
	if strings.Contains(ev2, "rc=0") {
		t.Errorf("负控①：读数被伪造成 rc=0（✗ 不许伪造 rc）：%q", ev2)
	}

	// ③ 负控②：跑不起来（rc=127）⇒ 判不了（退码 8）· 真源 sha 逐字不变 · 审计零行 · 仍 仍缺。
	t.Setenv("FAKE_RC", "127")
	sha3 := gapSHAOf(t, ledger)
	na3 := len(gapAuditLines(t, audit))
	if rc, _, errb := gapRun(t, "gap", "set-state", id3, "--state", "已解", "--evidence", "自动跑负控②",
		"--before-cmd", "zerg gap ls", "--after-cmd", "zerg gap ls", "--yes"); rc != 8 {
		t.Errorf("负控②（命令跑不起来 rc=127）⇒ 判不了 8（✗ 不当 0 用），得到 %d · err=%q", rc, errb)
	}
	if gapSHAOf(t, ledger) != sha3 || len(gapAuditLines(t, audit)) != na3 {
		t.Errorf("负控②：判不了那一发动了盘（真源 sha 或审计行数变了）")
	}
	if r := gapRecOf(t, ledger, id3); r["state"] != "仍缺" {
		t.Errorf("负控②：判不了 ⇒ state 要仍 仍缺，得到 %v", r["state"])
	}
}
