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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

// ---- ★ 缺口 `GAP-20260928-126`（②半 · 只读出口）--------------------------------------------
//
// 病灶（缺口逐字）：`dev proposal new` 的 `--rollback` 收的是**仓外整件路径**，而该路径无人看管
// （scratch 清理后改前态不可核）。①半（写面 `keepRollbackSnapshot`）已把前态**复本**落进状态目录
// `rollback-keep/`，但**读不回来** ⇒ 「退点依据可核」立不出判据。本件判 ②半（只读出口
// `rollbackKeepReadout` · 由 `dev proposal show` 印出）：
//
//	① 路径形 ＋ 看管档在盘 ⇒ show 印出**档路径 · 字节 · sha256**，且该 sha 与**独立现算**的盘上件逐字相同；
//	② 只读：跑 show 前后，看管目录 / 提案目录的件名清单逐字不变（不建目录、不写件）；
//	③ 反例（成对）：看管档被删 ⇒ **明说「无」并点名不可核**（✗ 不是恒绿的那一行）；「方式」文本退点 ⇒ 明说不落看管副本。
func TestDevProposalRollbackKeepReadout(t *testing.T) {
	proposalEvidenceDir(t)
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		t.Fatalf("回指清单读不出来：%v", err)
	}
	stateDir := os.Getenv("ZERG_STATE_DIR")
	t.Setenv("ZERG_PROPOSAL_DIR", t.TempDir())

	// 仓外前态件（夹具 · 写面会把它的复本落进 <状态目录>/rollback-keep/）
	pre := filepath.Join(t.TempDir(), "before.go")
	body := []byte("package x\n// 前态\n")
	if err := os.WriteFile(pre, body, 0o644); err != nil {
		t.Fatal(err)
	}
	base := []string{"--title", "看管出口", "--target", "待办:" + ids[0], "--goal", "g",
		"--evidence", "GAP-20260927-254", "--criterion", testCriterion}
	rc, out, errb := runProposalNew(t, append(append([]string{}, base...), "--rollback", pre)...)
	if rc != 0 {
		t.Fatalf("带路径形退点的提案 ⇒ rc=%d（要 0）· stderr=%s", rc, errb)
	}
	id := gap70ProposalID(out, errb)
	if id == "" {
		t.Fatalf("成功时应给提案 id · stdout=%q stderr=%q", out, errb)
	}

	// ① 读面印出**写面写下的真态**：档路径 · 字节 · sha256（与独立现算的盘上件逐字相同）
	sum := sha256.Sum256(body)
	wantSHA := hex.EncodeToString(sum[:])
	keep := filepath.Join(stateDir, "rollback-keep", id+"-"+filepath.Base(pre))
	if st, err := os.Stat(keep); err != nil || !st.Mode().IsRegular() {
		t.Fatalf("①半（写面）该落下的看管档不在盘：%s（%v）", keep, err)
	}
	var so, se strings.Builder
	if rc := zerg.RunForTest([]string{"dev", "proposal", "show", id}, &so, &se); rc != 0 {
		t.Fatalf("show %s ⇒ rc=%d · stderr=%s", id, rc, se.String())
	}
	if got := so.String(); !strings.Contains(got, "退点看管") || !strings.Contains(got, keep) ||
		!strings.Contains(got, fmt.Sprintf("%d 字节", len(body))) || !strings.Contains(got, "sha256 "+wantSHA) {
		t.Errorf("读面没把看管档真态读出来（要档路径＋字节＋sha256 %s）：\n%s", wantSHA, got)
	}

	// ② 只读：连跑两遍 show，件名清单逐字不变（不建目录、不写件）
	before := strings.Join(dirNames(t, stateDir), " · ")
	var so2, se2 strings.Builder
	if rc := zerg.RunForTest([]string{"dev", "proposal", "show", id}, &so2, &se2); rc != 0 {
		t.Fatalf("show（第二遍）⇒ rc=%d", rc)
	}
	if after := strings.Join(dirNames(t, stateDir), " · "); after != before {
		t.Errorf("读面动了盘（跑 show 前后件名清单不等）：\n前:%s\n后:%s", before, after)
	}

	// ③ 反例（成对）：看管档被删 ⇒ **明说「无」＋不可核**（✗ 不许恒绿）
	if err := os.Remove(keep); err != nil {
		t.Fatal(err)
	}
	var so3, se3 strings.Builder
	if rc := zerg.RunForTest([]string{"dev", "proposal", "show", id}, &so3, &se3); rc != 0 {
		t.Fatalf("show（无档态）⇒ rc=%d", rc)
	}
	if got := so3.String(); !strings.Contains(got, "**无**") || !strings.Contains(got, "不可核") {
		t.Errorf("看管档不在盘时应**明说无档 ＋ 不可核**（造不出反例的判据 = 永真）：\n%s", got)
	}

	// ③′ 反例：「方式」文本退点 ⇒ 明说**不落**看管副本（按其本义收）
	rc, out, errb = runProposalNew(t, append(append([]string{}, base...), "--rollback", "git revert")...)
	if rc != 0 {
		t.Fatalf("「方式」文本退点 ⇒ rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if id2 := gap70ProposalID(out, errb); id2 != "" {
		var so4, se4 strings.Builder
		if rc := zerg.RunForTest([]string{"dev", "proposal", "show", id2}, &so4, &se4); rc != 0 {
			t.Fatalf("show %s ⇒ rc=%d", id2, rc)
		}
		if got := so4.String(); !strings.Contains(got, "非路径形") || !strings.Contains(got, "不落") {
			t.Errorf("「方式」文本退点应明说**不落**看管副本：\n%s", got)
		}
	}
}

// dirNames —— 看管目录 ＋ 提案目录的件名清单（只读快照 · 用来判「读面有没有动盘」）。
func dirNames(t *testing.T, stateDir string) []string {
	t.Helper()
	var out []string
	for _, d := range []string{filepath.Join(stateDir, "rollback-keep"), os.Getenv("ZERG_PROPOSAL_DIR")} {
		ents, err := os.ReadDir(d)
		if err != nil {
			if os.IsNotExist(err) {
				out = append(out, d+"=(无此目录)")
				continue
			}
			t.Fatalf("读不了 %s：%v", d, err)
		}
		for _, e := range ents {
			out = append(out, filepath.Join(d, e.Name()))
		}
	}
	return out
}
