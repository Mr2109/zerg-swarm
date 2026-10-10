// cli_dev_edit_literal_text_test.go —— `zerg dev edit --literal-text`（缺口 GAP-20260927-234「守卫 literal-text」
// · Mr2109「甲案」仓内半）：三条语义硬线的成对自检 ——
//
//	① 正控：带 `--literal-text` 干跑 ⇒ rc=0 且**回执逐字出现**（声明口「看得见」）＋ 干跑零副作用；
//	② 负控甲：**不带**该旗标（其余同）⇒ 同一发里**回执必须不出现**（✗ 禁恒出声）；
//	③ 负控乙：该旗标**不得改变任何闸 / 审批的结果** —— 同一条命令带与不带，闸的判定**逐字相同**
//	   （用法闸 = 缺 `--proposal`；审批闸 = 无批准件真写）⇒ 证它只出声、不放宽任何闸。
//
// 落点：`package main_test` + `zerg.RunForTest`（跑**当前源码**的行为，不是盘上旧制品）；
// 夹具（`synthRepoForEdit` / `writeEditProposal` / `hostnameOf` / `mustWrite` / `runDevEdit`）复用同包
// `cli_dev_edit_approval_test.go`（同一套合成仓根 + 合成状态目录，不碰真仓的件）。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// literalTextReceiptSub —— 回执里**必须逐字出现**的那一段（声明语义的锚）。
const literalTextReceiptSub = "已声明（--literal-text）：本次命令行含"

func TestDevEdit_LiteralTextDeclaration(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9201", target)
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// 改后（--literal-text 自检夹具）\n")

	// ① 正控：带 --literal-text + 干跑 ⇒ rc=0 且回执逐字出现；干跑零副作用。
	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9201", "--file", target,
		"--from", from, "--dry-run", "--literal-text")
	if rc != 0 {
		t.Fatalf("正控：带 --literal-text 干跑 ⇒ rc=0，得 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(errb, literalTextReceiptSub) || !strings.Contains(errb, "--literal-text") {
		t.Fatalf("正控：回执必须逐字出现在 stderr：%q", errb)
	}
	if got, _ := os.ReadFile(filepath.Join(repo, target)); !strings.Contains(string(got), "// 原件") {
		t.Errorf("正控：干跑零副作用 —— 件一个字节不许动：%q", got)
	}

	// ② 负控甲：**不带**旗标（其余同）⇒ 回执**必须不出现**（✗ 禁恒出声）。
	rc2, _, errb2 := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9201", "--file", target,
		"--from", from, "--dry-run")
	if rc2 != 0 {
		t.Fatalf("负控甲：不带旗标干跑应 rc=0，得 %d · stderr=%s", rc2, errb2)
	}
	if strings.Contains(errb2, literalTextReceiptSub) {
		t.Errorf("负控甲：不带旗标 ⇒ 回执**必须不出现**：%q", errb2)
	}

	// ③ 负控乙：该旗标不得改变任何闸/审批的判决 ⇒ 同命令带与不带，闸判定逐字相同。
	//   闸甲（用法闸）：缺 `--proposal` ⇒ 两发都 rc=2 且都出同一条用法判词（回执多一行不碍判决）。
	rcU, _, errU := runDevEdit(t, repo, prop, state, "--file", target, "--from", from, "--dry-run")
	rcU2, _, errU2 := runDevEdit(t, repo, prop, state, "--file", target, "--from", from, "--dry-run", "--literal-text")
	if rcU != rcU2 || rcU != 2 {
		t.Fatalf("负控乙（用法闸）：缺提案 ⇒ 两发都 rc=2（得 %d vs %d）", rcU, rcU2)
	}
	const usageGate = "必须给 `--proposal"
	if !strings.Contains(errU, usageGate) || !strings.Contains(errU2, usageGate) {
		t.Fatalf("负控乙（用法闸）：闸判词两发都须在：errU=%q errU2=%q", errU, errU2)
	}

	//   闸乙（审批闸）：真写（--confirm=<本机名> --yes）但**无批准件** ⇒ 两发都 rc=2 且都出同一条审批判词。
	rcA, _, errA := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9201", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes")
	rcA2, _, errA2 := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9201", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes", "--literal-text")
	if rcA != rcA2 || rcA != 2 {
		t.Fatalf("负控乙（审批闸）：无批准件真写 ⇒ 两发都 rc=2（得 %d vs %d）", rcA, rcA2)
	}
	const apprGate = "真写要一枚人签批准件"
	if !strings.Contains(errA, apprGate) || !strings.Contains(errA2, apprGate) {
		t.Fatalf("负控乙（审批闸）：闸判词两发都须在：errA=%q errA2=%q", errA, errA2)
	}
}
