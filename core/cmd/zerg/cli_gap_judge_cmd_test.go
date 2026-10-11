// cli_gap_judge_cmd_test.go —— `gap set-state` 的 `--judge-cmd`（**可跑判据**）成对机检
// （main.go 开窗第 4 条 · 「前提不成立」销案的**更强一路**）。
//
// 病（本单靶心）：DEV-2443 的「可现读锚」闸只要求证据**点名一个可现读的物**（`family_gap_state.go`
// 的 `gapPremiseAnchorWhy`），仍**不能证明「前提真的不成立」**。本单加一条更强的路：给了
// `--judge-cmd` ⇒ 正门**真跑**它、用 rc/读数当证据（`rc=0` = 判据成立 · 否则拒收）；没给 ⇒ **维持**
// DEV-2443 现判（向后兼容 · ✗ 不把已有的合法「前提不成立」销案判红）。
//
// 本件钉住的格（缺一即失败）：
//
//	① 判据通过（rc=0）⇒ 销案成立（dry-run rc=0；真写落位、`solved_evidence` 含真跑读数）
//	② 判据失败（rc≠0）⇒ 销案**拒收**（rc=2 · 逐字报 rc 与读数 · 真源 sha 不变 · 审计不增）
//	③ 判据跑不起来（rc=127）⇒ **拒收**（rc=2 · ✗ 不静默当通过）
//	④ **不给** `--judge-cmd` ⇒ 维持 DEV-2443：纯散文拒（rc=2 · 点名「可现读的物」）、`件:行` 收（rc=0）
//	⑤ 只对「前提不成立」有效：`--judge-cmd` + `--solved-kind 销案` ⇒ rc=2；裸给 ⇒ rc=2
//	⑥ 普通「实现」销案**不受影响**（两态齐 + `--yes` ⇒ rc=0 且落位）
//
// 落点与同族一致：`package main_test` + `zerg.RunForTest`（跑**当前源码**行为）· 全部合成盘面
// （`gapEnv` / `gapRun` / `gapAddArgv` / `gapIDOf` / `gapRecOf` / `gapSHAOf` / `gapAuditLines` 复用同族尺）
// ⇒ 不碰真账。假 `zerg` 走临时 PATH（退码由 `FAKE_RC` 定）⇒ 不碰真制品、不真跑真命令。
package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGapPremiseJudgeCmdGate(t *testing.T) {
	_, ledger, audit := gapEnv(t)

	rc, out, errb := gapRun(t, append(gapAddArgv("手搓：judge-cmd 靶子", "判据靶子"), "--yes")...)
	if rc != 0 {
		t.Fatalf("造靶子失败：rc=%d · stdout=%q stderr=%q", rc, out, errb)
	}
	id := gapIDOf(t, out)

	// 合成一枚**假 `zerg`**：`--judge-cmd` 切词后首词裸 `zerg` ⇒ 走 PATH 解析到这枚假件
	// （真跑子进程 · 不碰真制品）；退码由环境变量 `FAKE_RC` 定（脚本内 `${FAKE_RC:-0}`）。
	binDir := t.TempDir()
	fake := filepath.Join(binDir, "zerg")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"判据读数行\"\nexit \"${FAKE_RC:-0}\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))

	sha0 := gapSHAOf(t, ledger)
	na0 := len(gapAuditLines(t, audit))

	// ① 判据通过（rc=0）⇒ dry-run rc=0（销案成立）。
	t.Setenv("FAKE_RC", "0")
	if rc, _, errb := gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "前提不成立（挂判据）", "--judge-cmd", "zerg gap ls", "--dry-run"); rc != 0 {
		t.Fatalf("① 判据通过 ⇒ dry-run 要 0，得到 %d · stderr=%q", rc, errb)
	}

	// ② 判据失败（rc≠0）⇒ 拒收 rc=2 · 逐字报 rc=3 · 真源不动。
	t.Setenv("FAKE_RC", "3")
	rc, out, errb = gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "前提不成立（判据该失败）", "--judge-cmd", "zerg gap ls", "--yes")
	if rc != 2 {
		t.Errorf("② 判据 rc=3 ⇒ 要 2（拒收），得到 %d · stderr=%q", rc, errb)
	}
	if !strings.Contains(errb, "未通过") || !strings.Contains(errb, "rc=3") {
		t.Errorf("② 拒收判词要逐字点名「未通过」与 rc=3，得到 %q", errb)
	}
	if out != "" {
		t.Errorf("② 拒收那一态 stdout 要 0 字节，得到 %q", out)
	}
	if gapSHAOf(t, ledger) != sha0 || len(gapAuditLines(t, audit)) != na0 {
		t.Errorf("② 拒收动了真源或写了审计")
	}

	// ③ 判据跑不起来（rc=127）⇒ 拒收 rc=2（✗ 不静默当通过）。
	t.Setenv("FAKE_RC", "127")
	if rc, _, errb := gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "前提不成立（判据跑不起来）", "--judge-cmd", "zerg gap ls", "--yes"); rc != 2 {
		t.Errorf("③ 判据 rc=127 ⇒ 要 2（拒收 · 判不了不当通过），得到 %d · stderr=%q", rc, errb)
	}
	if gapSHAOf(t, ledger) != sha0 || len(gapAuditLines(t, audit)) != na0 {
		t.Errorf("③ 拒收动了真源或写了审计")
	}

	// ④ 不给 --judge-cmd ⇒ 维持 DEV-2443 现判（向后兼容 · ✗ 不把合法销案判红）。
	//   ④a 纯散文 ⇒ 拒（rc=2 · 点名「可现读的物」）。
	if rc, _, errb := gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "纯自然语言（不给判据）", "--dry-run"); rc != 2 {
		t.Errorf("④a 不给判据 + 纯散文 ⇒ 要 2（维持 DEV-2443），得到 %d · stderr=%q", rc, errb)
	} else if !strings.Contains(errb, "可现读的物") {
		t.Errorf("④a 拒收判词要逐字点名「可现读的物」，得到 %q", errb)
	}
	//   ④b `件:行` 现读可得 ⇒ 收（rc=0）。
	if rc, _, errb := gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "前提不成立：见 core/cmd/zerg/family_code.go:42", "--dry-run"); rc != 0 {
		t.Errorf("④b 不给判据 + 件:行（现读可得）⇒ 要 0（维持 DEV-2443），得到 %d · stderr=%q", rc, errb)
	}

	// ⑤ 只对「前提不成立」有效：kind=销案 + 判据 ⇒ rc=2；裸给 ⇒ rc=2。
	if rc, _, _ := gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "销案",
		"--evidence", "e", "--judge-cmd", "zerg gap ls", "--dry-run"); rc != 2 {
		t.Errorf("⑤a `--judge-cmd` + kind=销案 ⇒ 要 2（只对前提不成立），得到 %d", rc)
	}
	if rc, _, _ := gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "e", "--judge-cmd", "--dry-run"); rc != 2 {
		t.Errorf("⑤b 裸给 `--judge-cmd` ⇒ 要 2，得到 %d", rc)
	}

	// ①' 真写：判据通过（rc=0）+ 两态齐 + --yes ⇒ rc=0、落位、`solved_evidence` 含真跑读数。
	t.Setenv("FAKE_RC", "0")
	if rc, _, errb := gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "前提不成立",
		"--evidence", "前提不成立（真写 判据通过）",
		"--before-read", "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前列于仍缺",
		"--after-read", "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后列于已解",
		"--judge-cmd", "zerg gap ls", "--yes"); rc != 0 {
		t.Fatalf("①' 判据通过 + 两态 + --yes ⇒ 要 0，得到 %d · stderr=%q", rc, errb)
	}
	r := gapRecOf(t, ledger, id)
	if r["state"] != "已解" || r["solved_kind"] != "前提不成立" {
		t.Errorf("①' 真写落位不对：state=%v solved_kind=%v", r["state"], r["solved_kind"])
	}
	ev := fmt.Sprint(r["solved_evidence"])
	if !strings.Contains(ev, "判据{cmd=zerg gap ls") || !strings.Contains(ev, "rc=0") {
		t.Errorf("①' `solved_evidence` 没落真跑判据读数：%q", ev)
	}

	// ⑥ 普通「实现」销案**不受影响**（两态齐 + --yes ⇒ rc=0 且落位）。
	if rc, _, errb := gapRun(t, "gap", "set-state", id, "--state", "已解", "--solved-kind", "实现",
		"--evidence", "真修已落 待提交面",
		"--before-read", "cmd=zerg gap ls --state 仍缺 · rc=1 · reading=改前列于仍缺",
		"--after-read", "cmd=zerg gap ls --state 已解 · rc=0 · reading=改后列于已解", "--yes"); rc != 0 {
		t.Fatalf("⑥ `实现`+两态+--yes 要 0（不受本闸影响），得到 %d · stderr=%q", rc, errb)
	}
	if r := gapRecOf(t, ledger, id); r["solved_kind"] != "实现" {
		t.Errorf("⑥ 真写落位不对：solved_kind=%v", r["solved_kind"])
	}
}
