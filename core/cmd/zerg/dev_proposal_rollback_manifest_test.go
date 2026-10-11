// dev_proposal_rollback_manifest_test.go —— 提案级退点清单件（缺口 `GAP-20261005-70` · 本单）的判据机检。
//
// 病灶（缺口逐字）：`dev edit` 的单一 `rollback_ref` 装不下多件提案 —— 多件提案里其余件退不了
// （2026-10-05 当日 58 件多件提案：42 件只盖 1 件 · 16 件零盖）。
//
// 本单落的是 **bounded 那一半**（`core/cmd/zerg/family_dev.go` 侧）：`--rollback` 除了**单值形态**
// （legacy），还收一枚**提案级退点清单** `{件仓内相对路径: 该件前态件路径}` —— 提案仍**只占一个
// `rollback_ref` 槽**（✗ 不改已定的「一件一退点」口径），并让 `dev proposal show` **逐件列出**。
// ★ 退点档（`dev edit` 侧 `devEditReturnPointOf` 按件挑匹配）属**其余**面（见缺口账目标值）。
//
// 判据（四条）：
//
//	① 改前：清单形态被当「路径形」判死（rc=2）—— 这正是缺口（多件提案装不下每件退点）。
//	② 改后：清单形态**收得下**（rc=0），逐条核「键=仓内相对路径 · 值=现读可得的普通件」。
//	③ 向后兼容：单值形态（路径形 / 方式文本）逐字不变（既有调用不受影响）。
//	④ 负控：残废清单 / 键是绝对路径 / 值读不到 ⇒ 一律 rc=2 判死（✗ 不悄悄退回单值形态）。
package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	zerg "github.com/Mr2109/zerg-swarm/core/cmd/zerg"
	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// gap70JSONStr —— 把一个字符串编成 JSON 字面量（构清单夹具用）。
func gap70JSONStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// gap70ProposalID —— 从 `dev proposal new` 的输出里取提案 id（stdout / stderr 都找）。
func gap70ProposalID(out, errb string) string {
	for _, f := range strings.Fields(out + "\n" + errb) {
		if strings.HasPrefix(f, "DEV-") {
			return f
		}
	}
	return ""
}

func TestDevProposalRollbackManifestPerFile(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())

	// 仓外前态件（夹具）：现读可得的普通件（清单里的「值」）。
	pre := filepath.Join(t.TempDir(), "before.go")
	if err := os.WriteFile(pre, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := []string{"--title", "清单件", "--target", "待办:" + ids[0], "--goal", "g",
		"--evidence", "GAP-20260927-254", "--criterion", testCriterion}

	// ③ 向后兼容：单值形态（路径形）逐字不变 ⇒ 0
	if rc, _, e := runProposalNew(t, append(append([]string{}, base...), "--rollback", pre)...); rc != 0 {
		t.Fatalf("单值（路径形）退点 ⇒ rc=%d（要 0 · legacy 不受影响）· stderr=%s", rc, e)
	}
	// ③ 向后兼容：单值形态（方式文本 · 非路径形）逐字不变 ⇒ 0
	if rc, _, e := runProposalNew(t, append(append([]string{}, base...), "--rollback", "git revert")...); rc != 0 {
		t.Fatalf("单值（方式文本）退点 ⇒ rc=%d（要 0 · legacy 不受影响）· stderr=%s", rc, e)
	}

	// ② 清单形态 ⇒ 收得下（0），逐件核。
	manifest := `{"core/cmd/zerg/main.go":` + gap70JSONStr(pre) +
		`,"core/cmd/zerg/family_dev.go":` + gap70JSONStr(pre) + `}`
	rc, out, errb := runProposalNew(t, append(append([]string{}, base...),
		"--rollback", manifest,
		"--file", "core/cmd/zerg/main.go", "--file", "core/cmd/zerg/family_dev.go")...)
	if rc != 0 {
		t.Fatalf("提案级退点清单 ⇒ rc=%d（要 0 · 每件都有退点）· stderr=%s", rc, errb)
	}
	id := gap70ProposalID(out, errb)
	if id == "" {
		t.Fatalf("成功时应给提案 id，实测空 · stdout=%q stderr=%q", out, errb)
	}

	// 读回面**逐件列出**（判据：每件都可读）
	var so, se strings.Builder
	if rc := zerg.RunForTest([]string{"dev", "proposal", "show", id}, &so, &se); rc != 0 {
		t.Fatalf("show %s ⇒ rc=%d · stderr=%s", id, rc, se.String())
	}
	if !strings.Contains(so.String(), "退点清单") || strings.Count(so.String(), "现读可得") < 2 {
		t.Errorf("show 没**逐件**列出退点清单（要 2 件的「现读可得」）：\n%s", so.String())
	}

	// ④ 负控①：残废清单（`{`-开头却解不出）⇒ 2（✗ 不悄悄退回单值形态）
	if rc, _, e := runProposalNew(t, append(append([]string{}, base...), "--rollback", `{not json`)...); rc != 2 || !strings.Contains(e, "清单") {
		t.Errorf("残废清单 ⇒ rc=%d（要 2）、stderr 要点名「清单」：%q", rc, e)
	}
	// ④ 负控②：键是绝对路径 ⇒ 2
	if rc, _, e := runProposalNew(t, append(append([]string{}, base...),
		"--rollback", `{"/etc/passwd":`+gap70JSONStr(pre)+`}`)...); rc != 2 || !strings.Contains(e, "仓内相对路径") {
		t.Errorf("键绝对路径 ⇒ rc=%d（要 2）、stderr 要点名「仓内相对路径」：%q", rc, e)
	}
	// ④ 负控③：值读不到 ⇒ 2（fail-closed：退点要现读可得）
	if rc, _, e := runProposalNew(t, append(append([]string{}, base...),
		"--rollback", `{"core/cmd/zerg/main.go":"/no/such/before.go"}`)...); rc != 2 || !strings.Contains(e, "读不到") {
		t.Errorf("值读不到 ⇒ rc=%d（要 2）、stderr 要点名「读不到」：%q", rc, e)
	}
}
