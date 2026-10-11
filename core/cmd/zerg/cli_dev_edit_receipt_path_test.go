// cli_dev_edit_receipt_path_test.go —— 本单两条缺口的回归牙（外部测试包 · 进程内跑当前源码）。
//
//	GAP-20260928-136：**路径口径归一** —— `--file`（提案面与改件面）与 `--from` / `--replace`（改件面）
//	  走过**同一套**口径：**仓内绝对**路径 ⇄ **仓内相对**路径**自动转换**（两侧可互换、可对拍）；
//	  **仓外绝对**路径仍按越界写 / 越界声明拒（退 2 —— 归一不放宽作用域）。
//
//	GAP-20260928-106：**回执面** ——
//	  ① `warnings[]` 有**真值落点**（真写档此前只在 stderr 出声 ⇒ 包封 `warnings[]` 恒 `[]`）；
//	  ② **件正文摘要**（改前 / 改后正文的**全 64 位 sha256**）登进 `--json` 字段表
//	     （`before_sha256_full` / `after_sha256_full`）⇒ 回执自身足以验改动，不必回仓现读 sha16。
//
// 全部夹具在 `t.TempDir()` 的合成仓根 / 合成状态目录里跑（不碰真仓）；真写那两格用自签批准件（同
// `writeOperatorKey`：与 `approve` 族消费者同一条验签路）。
package main_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// ---- GAP-20260928-136：`--file` / `--from` 的路径口径归一 ----

// TestDevEdit_PathCanonicalAbsoluteTargetMatchesDeclared —— 改件面 `--file` 给**仓内绝对**路径 ⇒
// 自动转**仓内相对**、与提案声明对上（改前：按「绝对路径 —— 作用域只认仓内相对路径」拒 = 症状）；
// 负控：**仓外绝对** `--file` 仍越界拒（退 2 —— 归一不放宽作用域）。
func TestDevEdit_PathCanonicalAbsoluteTargetMatchesDeclared(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9301", target)
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, "package main\n\n// 口径归一\n")

	abs := filepath.Join(repo, target)
	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9301", "--file", abs,
		"--from", from, "--dry-run")
	if rc != 0 {
		t.Fatalf("仓内绝对 --file 应自动转仓内相对、与声明对上 ⇒ 干跑退 0，得到 %d · stdout=%s · stderr=%s",
			rc, out, errb)
	}
	if !strings.Contains(out, "  目标件   : "+target) {
		t.Errorf("计划面目标件要归一成仓内相对 %q：%q", target, out)
	}
	if strings.Contains(out, abs) {
		t.Errorf("计划面不该再出现仓内绝对路径 %q：%q", abs, out)
	}

	// 负控：仓外绝对 ⇒ 越界拒（退 2），逐字点名「绝对路径」。
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9301", "--file", "/etc/passwd",
		"--from", from, "--dry-run")
	if rc != 2 {
		t.Fatalf("仓外绝对 --file 仍须越界拒（退 2），得到 %d · stderr=%s", rc, errb)
	}
	if !strings.Contains(errb, "绝对路径") {
		t.Errorf("越界拒要逐字点名「绝对路径」：%q", errb)
	}
}

// TestDevEdit_PathCanonicalFromAcceptsRepoRelative —— `--from` 与 `--file` 同一套口径：给**仓内相对**
// 路径 ⇒ 按**仓根**解析（改前：相对 cwd ⇒ 同一次改动要在两种形态间手工搬）；负控：仓内不存在的相对路径 ⇒ 退 2。
func TestDevEdit_PathCanonicalFromAcceptsRepoRelative(t *testing.T) {
	repo, target := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	writeEditProposal(t, prop, "DEV-9302", target)
	candRel := filepath.Join("core", "cmd", "zerg", "cand.go")
	mustWrite(t, filepath.Join(repo, candRel), "package main\n\n// 仓内候选\n")

	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9302", "--file", target,
		"--from", candRel, "--dry-run")
	if rc != 0 {
		t.Fatalf("`--from %s`（仓内相对）应按仓根解析 ⇒ 干跑退 0，得到 %d · stdout=%s · stderr=%s",
			candRel, rc, out, errb)
	}
	// 负控：仓内**不存在**的相对路径 ⇒ 读不了 ⇒ 退 2（不猜内容）。
	rc, _, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9302", "--file", target,
		"--from", filepath.Join("core", "cmd", "zerg", "nope.go"), "--dry-run")
	if rc != 2 || !strings.Contains(errb, "读不了内容来源") {
		t.Errorf("仓内不存在的相对路径 ⇒ 退 2 并点名读不了：rc=%d stderr=%q", rc, errb)
	}
}

// TestDevProposal_PathCanonicalRepoInternalAbsoluteAccepted —— 提案面 `--file` 与前一条**同一套**口径：
// 仓内绝对自动转仓内相对 ⇒ 收（改前 = 症状 rc=2）；负控：仓外绝对 ⇒ 仍越界声明拒（退 2）。
func TestDevProposal_PathCanonicalRepoInternalAbsoluteAccepted(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())
	base := []string{"--title", "路径口径", "--target", "待办:" + ids[0], "--goal", "g",
		"--evidence", proposalEvidenceInLedger, "--rollback", "r", "--criterion", testCriterion}

	wd, err := os.Getwd() // `go test` 的 cwd = 本包目录（仓内）
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(wd, "main.go") // 仓内绝对路径
	rc, out, errb := runProposalNew(t, append(append([]string{}, base...), "--file", abs)...)
	if rc != 0 {
		t.Fatalf("仓内绝对 --file 应自动转仓内相对 ⇒ 退 0，得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}

	rc, _, errb = runProposalNew(t, append(append([]string{}, base...), "--file", "/etc/passwd")...)
	if rc != 2 || !strings.Contains(errb, "绝对路径") {
		t.Errorf("仓外绝对 --file ⇒ 退 2 并点名「绝对路径」：rc=%d stderr=%q", rc, errb)
	}
}

// ---- GAP-20260928-106：回执面（件正文摘要 + `warnings[]` 真值落点）----

// TestDevEdit_ReceiptCarriesBodyDigestAndWarnings —— 真写 + `--json` 点名件正文摘要：
//
//	① 干跑档与真写档**都产出** `after_sha256_full` / `before_sha256_full`（全 64 位 ⇒ 回执自身可验改动）；
//	② 真写档 `warnings[]` 不再恒 `[]`（缺摘要这条**真事**有了落点）。
func TestDevEdit_ReceiptCarriesBodyDigestAndWarnings(t *testing.T) {
	repo, _ := synthRepoForEdit(t)
	prop, state := t.TempDir(), t.TempDir()
	// 目标件**改前不存在**（新建件）⇒ 影响面摘要走 `new_file` 那一档 ⇒ 波纹指纹**未写**（`digest == ""`）
	// ⇒ 正是缺口里「缺摘要这一格没有落点」的场景（真写档要把它落进 `warnings[]`）。
	target := filepath.Join("core", "cmd", "zerg", "brand_new.go")
	writeEditProposal(t, prop, "DEV-9303", target)
	sign := writeOperatorKey(t, state)
	mustWrite(t, filepath.Join(state, "approvals", "dev_edit.json"),
		string(sign("dev_edit", target, "张三", "2026-10-11T00:00:00+08:00", "放行本件"))+"\n")
	body := "package main\n\n// 件正文摘要\n"
	from := filepath.Join(t.TempDir(), "new.go")
	mustWrite(t, from, body)
	sum := sha256.Sum256([]byte(body))
	want := hex.EncodeToString(sum[:])

	// ① 干跑档：件正文摘要点得出来（字段面收）。
	rc, out, errb := runDevEdit(t, repo, prop, state, "--proposal", "DEV-9303", "--file", target,
		"--from", from, "--dry-run", "--json", "file,after_sha256_full,before_sha256_full")
	if rc != 0 {
		t.Fatalf("干跑 + 点名件正文摘要 ⇒ 退 0，得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(out, `"after_sha256_full":"`+want+`"`) {
		t.Errorf("干跑回执缺**件正文摘要**（全 64 位 after sha256 = %s）：%q", want, out)
	}

	// ② 真写档：件正文摘要 + `warnings[]` 真值（不再恒 `[]`）。
	rc, out, errb = runDevEdit(t, repo, prop, state, "--proposal", "DEV-9303", "--file", target,
		"--from", from, "--confirm="+hostnameOf(t), "--yes",
		"--json", "file,after_sha256_full,before_sha256_full")
	if rc != 0 {
		t.Fatalf("真写 + 点名件正文摘要 ⇒ 退 0，得到 %d · stdout=%s · stderr=%s", rc, out, errb)
	}
	if !strings.Contains(out, `"after_sha256_full":"`+want+`"`) {
		t.Errorf("真写回执缺**件正文摘要**（全 64 位 after sha256 = %s）：%q", want, out)
	}
	if strings.Contains(out, `"warnings":[]`) {
		t.Errorf("本趟有真事（影响面摘要未取数）⇒ `warnings[]` 不该仍是空数组：%q", out)
	}
	if !strings.Contains(out, "缺摘要") {
		t.Errorf("`warnings[]` 要点名「缺摘要」这条真事（缺口正文：这一格此前没有落点）：%q", out)
	}
}
