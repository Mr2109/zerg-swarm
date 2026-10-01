// check_mutation_residue_test.go —— Go 真搬样板的**成对负控**定向单测（M2 路2 · 4.2「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckMutationResidueGatePort（定向：go test ./cmd/zerg -run TestCheckMutationResidueGatePort）。
//
// 负控有牙的证法（见本单 SUMMARY ⑤）：把判据阈值/分支在**仓外副本**上哑化（例：把 `hits == 1` 写成
// `hits >= 0`）⇒ 正控那条断言当场 FAIL —— 落仓件一字未动。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cmrFixture —— 在 t.TempDir()（仓外）造一棵含真源件 + a.go + b.go 的夹具树；script 空 ⇒ 用 mutSynth。
func cmrFixture(t *testing.T, aBody, bBody, script string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	sc := script
	if sc == "" {
		sc = mutSynth
	}
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(mutTruthRel, sc)
	write("a.go", aBody)
	write("b.go", bBody)
	return root
}

func cmrRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := mutationResidueGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckMutationResidueGatePort(t *testing.T) {
	// ── 正控（必然判绿）：两靶件原形 ─────────────────────────────────────
	rcGreen, outGreen, _ := cmrRun(t, "--root", cmrFixture(t, mutGoodA, mutGoodB, ""))
	if rcGreen != 0 || !strings.Contains(outGreen, "\t原形\t") ||
		!strings.Contains(outGreen, "全 2 点**原形**（rc=0）") {
		t.Fatalf("正控失败：好树应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控①（必然判红）：换值型残态 0.8 → 0.9 ⇒ rc=1 且逐条点名 b.go ────
	rcRotten, outRotten, _ := cmrRun(t, "--root", cmrFixture(t,
		mutGoodA, strings.Replace(mutGoodB, "0.8", "0.9", -1), ""))
	if rcRotten != 1 || !strings.Contains(outRotten, "残态点名") ||
		!strings.Contains(outRotten, "· b.go（第 1 点") {
		t.Fatalf("负控-换值 失败：得 rc=%d out=%q", rcRotten, outRotten)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcRotten {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcRotten)
	}

	// ── 负控②：删行型残态（原文锚整行被删 · NEW 为空）⇒ rc=1 且点名 a.go ──
	rcDel, outDel, _ := cmrRun(t, "--root", cmrFixture(t, "func main() {\n}\n", mutGoodB, ""))
	if rcDel != 1 || !strings.Contains(outDel, "· a.go（第 0 点") ||
		!strings.Contains(outDel, "改后值也不在") {
		t.Fatalf("负控-删行 失败：得 rc=%d out=%q", rcDel, outDel)
	}

	// ── 负控③：锚多命中（歧义）⇒ rc=2（判不了 ≠ 绿）────────────────────
	rcAmb, outAmb, _ := cmrRun(t, "--root", cmrFixture(t,
		mutGoodA+"adp.Start(); err != nil {\n", mutGoodB, ""))
	if rcAmb != 2 || !strings.Contains(outAmb, "歧义") {
		t.Fatalf("负控-歧义 失败：得 rc=%d out=%q", rcAmb, outAmb)
	}

	// ── 负控④：靶件不在 ⇒ rc=2 ────────────────────────────────────────
	rootNo := cmrFixture(t, mutGoodA, mutGoodB, "")
	if err := os.Remove(filepath.Join(rootNo, "a.go")); err != nil {
		t.Fatal(err)
	}
	rcNo, outNo, _ := cmrRun(t, "--root", rootNo)
	if rcNo != 2 || !strings.Contains(outNo, "靶件不在") {
		t.Fatalf("负控-靶件不在 失败：得 rc=%d out=%q", rcNo, outNo)
	}

	// ── 取不到输入：根路径不存在（真源件读不出）⇒ rc=2 ────────────────
	rcMiss, outMiss, _ := cmrRun(t, "--root", filepath.Join(t.TempDir(), "nope"))
	if rcMiss != 2 || !strings.Contains(outMiss, "真源件不在") {
		t.Fatalf("取不到输入 失败：得 rc=%d out=%q", rcMiss, outMiss)
	}

	// ── 空转：真源件在但 0 点 ⇒ rc=2（空转 ≠ 绿）──────────────────────
	empty := "#!/usr/bin/env bash\nMUT_FILE=(\n)\nMUT_OLD=(\n)\nMUT_NEW=(\n)\nMUT_TEST=(\n)\nMUT_NAME=(\n)\n"
	rcEmpty, outEmpty, _ := cmrRun(t, "--root", cmrFixture(t, mutGoodA, mutGoodB, empty))
	if rcEmpty != 2 || !strings.Contains(outEmpty, "五个索引数组读不出") {
		t.Fatalf("空转 失败：得 rc=%d out=%q", rcEmpty, outEmpty)
	}

	// ── 两读对不上（判据漂了）⇒ rc=2 ──────────────────────────────────
	rcDrift, outDrift, _ := cmrRun(t, "--root", cmrFixture(t, mutGoodA, mutGoodB, mutScriptDrift))
	if rcDrift != 2 || !strings.Contains(outDrift, "两读第 0 点对不上") {
		t.Fatalf("两读对不上 失败：得 rc=%d out=%q", rcDrift, outDrift)
	}

	// ── 反误报（本件最要紧的一格）：生产侧右端加一列 ⇒ **必须 rc=0** ────
	rcAdd, outAdd, _ := cmrRun(t, "--root", cmrFixture(t, mutGoodA, mutGoodB, mutSynthAddcol))
	if rcAdd != 0 || !strings.Contains(outAdd, "全 2 点**原形**（rc=0）") {
		t.Fatalf("反误报-加列 失败：加列不该破本件，得 rc=%d out=%q", rcAdd, outAdd)
	}
	if rcAdd == rcRotten {
		t.Fatalf("成对断言失败：加列 rc=%d 与真残态 rc=%d 同码 ⇒ 无鉴别力", rcAdd, rcRotten)
	}

	// ── 缺本件要的那一列 ⇒ rc=2 且报「缺列」──────────────────────────
	rcDrop, outDrop, _ := cmrRun(t, "--root", cmrFixture(t, mutGoodA, mutGoodB, mutSynthDropcol))
	if rcDrop != 2 || !strings.Contains(outDrop, "缺本件要的列：点名判据") {
		t.Fatalf("缺列 失败：得 rc=%d out=%q", rcDrop, outDrop)
	}

	// ── 老式无表头 ⇒ rc=2（列契约读不出）────────────────────────────
	rcHead, _, _ := cmrRun(t, "--root", cmrFixture(t, mutGoodA, mutGoodB, mutScriptNoheader))
	if rcHead != 2 {
		t.Fatalf("无表头 失败：得 rc=%d（应 2）", rcHead)
	}

	// ── 未知旗标（argparse 拒收）⇒ rc=2 · stderr 逐字面 ───────────────
	rcFlag, outFlag, errFlag := cmrRun(t, "--root", cmrFixture(t, mutGoodA, mutGoodB, ""), "--no-such-flag")
	if rcFlag != 2 || outFlag != "" ||
		!strings.Contains(errFlag, "error: unrecognized arguments: --no-such-flag") {
		t.Fatalf("未知旗标 失败：得 rc=%d out=%q stderr=%q", rcFlag, outFlag, errFlag)
	}

	// ── `--json` 形态：本门**无机器读面** ⇒ argparse 拒收 rc=2 ─────────
	rcJSON, _, errJSON := cmrRun(t, "--json")
	if rcJSON != 2 || !strings.Contains(errJSON, "error: unrecognized arguments: --json") {
		t.Fatalf("--json 失败：得 rc=%d stderr=%q", rcJSON, errJSON)
	}

	// ── `-h`：帮助正文（逐字取自 py 现跑）⇒ rc=0 ─────────────────────
	rcHelp, outHelp, _ := cmrRun(t, "-h")
	if rcHelp != 0 || !strings.HasPrefix(outHelp, mutUsageLine+"\n") ||
		!strings.Contains(outHelp, "--self-test") {
		t.Fatalf("-h 失败：得 rc=%d out=%q", rcHelp, outHelp)
	}

	// ── `--self-test`：11 例 · 19 条断言自身必须全过（防恒绿/恒红）──────
	rcST, outST, _ := cmrRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "断言 19 条 · 失败 0 条") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}
