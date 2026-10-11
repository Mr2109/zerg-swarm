// family_impact_face_parity_test.go —— 缺口 `GAP-20260927-55` 的「两面判定口」成对自检。
//
// 判据口 = `impactFaceParityReading`（`family_impact_layers.go`）。它把 `zerg impact` ④a 的
// **索引面**（`git grep` · `codeFaceIndex` = 只搜 tracked）与 `zerg code find` 的**工作树面**
// （`codeWorktreeFaceHits` · `codeFaceWorktree` = tracked + 未跟踪）摆成同一条读数，并保证
// 「两面不一致 ⇒ 点名两面读数」/「任一面取不到 ⇒ 不给结论」。
//
//	· 正控：两面一致 ⇒ ok=true，读数**逐字**等于「两面读数：索引面「…」N 条 · 工作树面「…」N 条」
//	  （即与既有口径 `codeFaceMismatchNote` 的「一致 ⇒ 空串」逐字相符：**不**追加点名）；
//	· 负控①（造一处两面不一致）：索引 1 / 工作树 4 ⇒ 读数**逐字**接上 `codeFaceMismatchNote` 的点名，
//	  且点名里**同时**报出两面的读数（缺一即 t.Error = 真红）；不一致本身**不是**「不给结论」（仍 ok=true）；
//	· 负控②（取不到任一面）：索引面 / 工作树面 / 两面各自缺失 ⇒ ok=false（**不给结论**）并点名缺哪一面。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImpactFaceParity_ConsistentMatchesExistingCaliber(t *testing.T) {
	word := "family_code.go"
	got, ok := impactFaceParityReading(word, 4, true, 4, true)
	if !ok {
		t.Fatalf("正控破：两面都取到且一致 ⇒ 该 ok=true，实得 %v（读数 %q）", ok, got)
	}
	want := fmt.Sprintf("两面读数：索引面「%s」4 条 · 工作树面「%s」4 条", codeFaceIndex, codeFaceWorktree)
	if got != want {
		t.Errorf("正控破：一致时读数应与既有口径逐字相符：\n得到 %q\n要得 %q", got, want)
	}
	// 既有口径：一致 ⇒ `codeFaceMismatchNote` 回空串 ⇒ 读数里**不许**出现「不一致」点名。
	if note := codeFaceMismatchNote(word, 4, 4); note != "" {
		t.Fatalf("既有口径已变：一致时 `codeFaceMismatchNote` 竟非空：%q", note)
	}
	if strings.Contains(got, "两面不一致") {
		t.Errorf("正控破：两面一致却点名了「不一致」：%q", got)
	}
}

func TestImpactFaceParity_MismatchNamesBothReadings(t *testing.T) {
	word := "errors.go"
	got, ok := impactFaceParityReading(word, 1, true, 4, true)
	if !ok {
		t.Fatalf("负控①：两面都取到、只是不一致 ⇒ 该 ok=true（不一致不是「不给结论」），实得 %v", ok)
	}
	note := codeFaceMismatchNote(word, 4, 1)
	if note == "" {
		t.Fatal("负控①准备态破：既有口径对不一致竟回空串")
	}
	want := fmt.Sprintf("两面读数：索引面「%s」1 条 · 工作树面「%s」4 条 · %s", codeFaceIndex, codeFaceWorktree, note)
	if got != want {
		t.Errorf("负控①破：不一致时读数应逐字接上点名：\n得到 %q\n要得 %q", got, want)
	}
	if !strings.Contains(got, "不一致") || !strings.Contains(got, "1") || !strings.Contains(got, "4") {
		t.Errorf("负控①破：点名没同时报出两面的读数（1 / 4）：%q", got)
	}
}

func TestImpactFaceParity_MissingFaceRefusesConclusion(t *testing.T) {
	for _, tc := range []struct {
		name        string
		idxOK, wtOK bool
		wantMissing string
	}{
		{"索引面取不到", false, true, codeFaceIndex},
		{"工作树面取不到", true, false, codeFaceWorktree},
		{"两面都取不到", false, false, codeFaceIndex},
	} {
		got, ok := impactFaceParityReading("w", 1, tc.idxOK, 2, tc.wtOK)
		if ok {
			t.Errorf("负控②（%s）破：取不到一面竟给了结论（该 ok=false）：%q", tc.name, got)
		}
		if !strings.Contains(got, "不给结论") {
			t.Errorf("负控②（%s）破：没写「不给结论」：%q", tc.name, got)
		}
		if !strings.Contains(got, tc.wantMissing) {
			t.Errorf("负控②（%s）破：没点名缺的那一面 %q：%q", tc.name, tc.wantMissing, got)
		}
	}
}

// TestImpactLayerLexical_UnreadableIndexFaceRefusesConclusion —— 端到端：④a 现读接线面。
// 目标根 = 仓外一个**不是 git 仓**的空目录 ⇒ 索引面（`git grep`）起不来 ⇒ 该层必须写「读不到」
// （不给结论 · fail-loud），✗ 不许拿工作树面的数当答案（那正是缺口要治的「静默取其一」）。
func TestImpactLayerLexical_UnreadableIndexFaceRefusesConclusion(t *testing.T) {
	lay := impactLayerLexical(t.TempDir(), &impactTarget{Kind: impactKindFile, Raw: "x.go", Rel: "x.go"})
	if lay.Status != "读不到" {
		t.Errorf("索引面取不到 ⇒ 该层该写「读不到」，实得 %q：%s", lay.Status, lay.Detail)
	}
	if !strings.Contains(lay.Detail, "不给结论") {
		t.Errorf("取不到面时该明写「不给结论」：%s", lay.Detail)
	}
}

// layRowsFloor —— ② ④a 清单行数的**冻结下限**（棘轮 · 只许增不许减；本值 = 当次现读）。
// 本用例夹具里工作树面命中恰 1 件（未跟踪 `untracked_new.go`）⇒ 下限 = 1。
const layRowsFloor = 1

// TestImpactLayerLexical_WorktreeFaceIncludesUntracked —— 缺口 `GAP-20261010-18` 的回归牙：
// ④a 现读走**工作树面**（tracked + 未跟踪 · 与 `zerg code find` 同源）⇒ 一件**未跟踪**的新写件，
// 只要它的工作树内容里出现 ④a 的查询词，就**必须进 ④a 清单**（旧读法走 `git grep` 索引面 ⇒ 不进）。
//
// 夹具：仓外 `t.TempDir()` 里造一棵**真 git 仓**（✗ 不在真仓造未跟踪件污染工作树）——
// ① 提交一件 tracked 靶 `target.go`（内容**不含**查询词）⇒ 索引面零命中；② 落一件**未跟踪**的
// `untracked_new.go`，内容含查询词 ⇒ 只有工作树面看得见。判据：④a 清单里必须出现 `untracked_new.go`。
// ★ 负控真红：拿改前件（`git grep` 索引面版）overlay 本件同一条用例 ⇒ 必须 FAIL（牙长在门上）。
func TestImpactLayerLexical_WorktreeFaceIncludesUntracked(t *testing.T) {
	root := t.TempDir()
	runFixture := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("夹具命令 %v 失败：%v · %s", args, err, out)
		}
	}
	runFixture("git", "init", "-q")
	runFixture("git", "config", "user.email", "t@t")
	runFixture("git", "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// tracked 靶：件名 `target.go`（= ④a 查询词），但**内容不含**它 ⇒ 索引面零命中。
	if err := os.WriteFile(filepath.Join(root, "target.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runFixture("git", "add", "go.mod", "target.go")
	runFixture("git", "commit", "-q", "-m", "init")
	// 未跟踪件（不在索引里）：工作树内容里出现查询词 `target.go`。
	if err := os.WriteFile(filepath.Join(root, "untracked_new.go"), []byte("// this line mentions target.go on purpose\npackage main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lay := impactLayerLexical(root, &impactTarget{Kind: impactKindFile, Raw: "target.go", Rel: "target.go"})
	if lay.Status != "取值" {
		t.Fatalf("④a 该取值，实得 %q：%s", lay.Status, lay.Detail)
	}
	found := false
	for _, r := range lay.Rows {
		if strings.Contains(r["what"], "untracked_new.go") {
			found = true
		}
	}
	// ② 冻结下限（棘轮 · 只许增）：④a 清单行数 < 当次现读数下限 1 ⇒ 判红（结构静默变小）。
	if n := len(lay.Rows); n < layRowsFloor {
		t.Fatalf("④a 清单行数 %d < 冻结下限 %d（棘轮只许增 · 结构静默变小）", n, layRowsFloor)
	}
	if !found {
		t.Fatalf("④a 清单里没有未跟踪件 `untracked_new.go` —— 工作树面没接上（正是缺口 GAP-20261010-18）。行数=%d · 读数=%s", len(lay.Rows), lay.Detail)
	}
}
