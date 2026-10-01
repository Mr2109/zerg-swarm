// check_gap_ledger_face_test.go —— `check_gap_ledger_face.go` 的**成对负控**单测（M2 · 账面面貌类）。
//
// 口径：**成对负控（✗ 禁恒绿）** —— 同一套判据，正控（合成账全在闭集内）⇒ rc=0；
// 负控（注入值域外值 / 抽掉表头 / 抽掉账件 / 空转 / 登记件缺闭集值 / 计数句不符）⇒ rc=1 或 rc=2；
// 再把负控那一处补回 ⇒ 回绿 rc=0。哑化验证（仓外副本）见交付读数。
//
// ★ 夹具**全在 `t.TempDir()` 下**（仓外）：带 `scripts/gates/` 的合成仓根 + 兄弟 `Zerg-内部文档/`，
//
//	账件表头**直接取 `glfFaces` 的逐字句**（不手抄表头字面量 ⇒ 表头随真源走）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func glfTWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func glfTRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkGapLedgerFaceGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// glfTBuild —— 造一棵合成树：`<base>/repo`（含 `scripts/gates/`）+ 兄弟 `<base>/Zerg-内部文档/`。
// 三个可调点：ledgerA/ledgerB 正文、登记件正文；nil 表示用默认（绿）正文。
func glfTBuild(t *testing.T, base, ledgerA, ledgerB, register string) (repo, docs, reg string) {
	t.Helper()
	repo = filepath.Join(base, "repo")
	docs = filepath.Join(base, "Zerg-内部文档")
	_ = os.MkdirAll(filepath.Join(repo, "scripts", "gates"), 0o755)
	la := filepath.Join(docs, glfLedgerA)
	lb := filepath.Join(docs, glfLedgerB)
	reg = filepath.Join(docs, glfRegisterRel)
	if ledgerA == "" {
		ledgerA = glfTLedgerA(true)
	}
	if ledgerB == "" {
		ledgerB = glfTLedgerB()
	}
	if register == "" {
		register = glfTRegister()
	}
	glfTWrite(t, la, ledgerA)
	glfTWrite(t, lb, ledgerB)
	glfTWrite(t, reg, register)
	return repo, docs, reg
}

// glfTLedgerA —— 账件 A：面 A/面 B 两节（节间以非表行隔开，避免相互串读）。
func glfTLedgerA(green bool) string {
	aVal := "✓"
	if !green {
		aVal = "已派" // 面 A 闭集外（面 A 闭集 = ✓/半/无）
	}
	return "# 合成账 A\n\n" +
		glfFaces[0].header + "\n|---|---|---|---|---|---|\n" +
		"| 1 | a | b | " + aVal + " | c | d |\n" +
		"| 2 | a | b | 半 | c | d |\n" +
		"| 3 | a | b | 无 | c | d |\n" +
		"\n## 十三\n\n" +
		glfFaces[1].header + "\n|---|---|---|---|---|---|\n" +
		"| 1 | a | b | ✓已解 | c | d |\n" +
		"| 2 | a | b | ⟳本轮补 | c | d |\n"
}

// glfTLedgerB —— 账件 B：面 C/面 D 两节。
func glfTLedgerB() string {
	return "# 合成账 B\n\n" +
		glfFaces[2].header + "\n|---|---|---|---|---|---|---|\n" +
		"| 1 | a | b | c | d | e | 已解 |\n" +
		"\n## 二/三\n\n" +
		glfFaces[3].header + "\n|---|---|---|---|---|---|---|\n" +
		"| 1 | a | b | c | d | e | 仍缺 |\n"
}

// glfTRegister —— 登记件：四面的闭集值逐字 + 四面现读计数句。
func glfTRegister() string {
	return "闭集 `✓` / `半` / `无` / `✓已解` / `⟳本轮补` / `✗仍缺` / `已解` / `已派` / `已立项` / `仍缺`\n" +
		"面 A · 表头 1 处 · 数据行 3\n" +
		"面 B · 表头 1 处 · 数据行 2\n" +
		"面 C · 表头 1 处 · 数据行 1\n" +
		"面 D · 表头 1 处 · 数据行 1\n"
}

func TestGapLedgerFaceGatePort(t *testing.T) {
	// ① 正控：四面全在闭集内 ⇒ rc=0
	repo, docs, reg := glfTBuild(t, t.TempDir(), "", "", "")
	if rc, out, errb := glfTRun(t, "--root", repo, "--docs-root", docs, "--register", reg); rc != 0 {
		t.Fatalf("① 正控失败：期望 rc=0，实得 rc=%d out=%q err=%q", rc, out, errb)
	} else if !strings.Contains(out, "值域内") {
		t.Fatalf("① 正控缺结论行：%q", out)
	}

	// ② 负控：面 A 注入闭集外的 `已派` ⇒ rc=1 且逐条点名（件:行 + 原样值）
	repo2, docs2, reg2 := glfTBuild(t, t.TempDir(), glfTLedgerA(false), "", "")
	rc, out, _ := glfTRun(t, "--root", repo2, "--docs-root", docs2, "--register", reg2)
	if rc != 1 {
		t.Fatalf("② 负控失败：期望 rc=1，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "值域外") || !strings.Contains(out, "已派") ||
		!strings.Contains(out, glfLedgerA+":5") {
		t.Fatalf("② 负控未点名到「件:行 + 原样值」：%q", out)
	}

	// ③ 成对回绿：同一夹具去掉那个闭集外的值 ⇒ rc=0
	repo3, docs3, reg3 := glfTBuild(t, t.TempDir(), glfTLedgerA(true), "", "")
	if rc, out, _ := glfTRun(t, "--root", repo3, "--docs-root", docs3, "--register", reg3); rc != 0 {
		t.Fatalf("③ 成对回绿失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	}

	// ④ 空转（面 A 表头在 · 0 行）⇒ rc=2（不给结论 · ✗ 不当绿）
	emptyA := "# 合成账 A\n\n" + glfFaces[0].header + "\n|---|---|---|---|---|---|\n\n## 十三\n\n" +
		glfFaces[1].header + "\n|---|---|---|---|---|---|\n| 1 | a | b | ✓已解 | c | d |\n"
	repo4, docs4, reg4 := glfTBuild(t, t.TempDir(), emptyA, "", "")
	if rc, out, _ := glfTRun(t, "--root", repo4, "--docs-root", docs4, "--register", reg4); rc != 2 {
		t.Fatalf("④ 空转失败：期望 rc=2，实得 rc=%d out=%q", rc, out)
	} else if !strings.Contains(out, "不给结论") {
		t.Fatalf("④ 空转缺「不给结论」：%q", out)
	}

	// ⑤ 表头逐字句找不到（面 A 换了表头）⇒ rc=2
	badHdr := "# 合成账 A\n\n| 号 | 别的 | 表头 |\n|---|---|---|\n| 1 | a | ✓ |\n"
	repo5, docs5, reg5 := glfTBuild(t, t.TempDir(), badHdr, "", "")
	if rc, out, _ := glfTRun(t, "--root", repo5, "--docs-root", docs5, "--register", reg5); rc != 2 {
		t.Fatalf("⑤ 表头找不到失败：期望 rc=2，实得 rc=%d out=%q", rc, out)
	}

	// ⑥ 账件不在盘 ⇒ rc=2（不拿「读不到」当「没问题」）
	repo6, docs6, reg6 := glfTBuild(t, t.TempDir(), "", "", "")
	if err := os.Remove(filepath.Join(docs6, glfLedgerB)); err != nil {
		t.Fatal(err)
	}
	if rc, out, _ := glfTRun(t, "--root", repo6, "--docs-root", docs6, "--register", reg6); rc != 2 {
		t.Fatalf("⑥ 缺账件失败：期望 rc=2，实得 rc=%d out=%q", rc, out)
	}

	// ⑦ 登记件缺一个闭集值（去掉 `无`）⇒ rc=2
	badReg := "闭集 `✓` / `半` / `✓已解` / `⟳本轮补` / `✗仍缺` / `已解` / `已派` / `已立项` / `仍缺`\n" +
		"面 A · 表头 1 处 · 数据行 3\n面 B · 表头 1 处 · 数据行 2\n面 C · 表头 1 处 · 数据行 1\n面 D · 表头 1 处 · 数据行 1\n"
	repo7, docs7, reg7 := glfTBuild(t, t.TempDir(), "", "", badReg)
	if rc, out, _ := glfTRun(t, "--root", repo7, "--docs-root", docs7, "--register", reg7); rc != 2 {
		t.Fatalf("⑦ 登记缺闭集值失败：期望 rc=2，实得 rc=%d out=%q", rc, out)
	} else if !strings.Contains(out, "闭集值") {
		t.Fatalf("⑦ 缺「闭集值」判词：%q", out)
	}

	// ⑦b 登记件计数句与现读不符 ⇒ rc=2
	badCnt := "闭集 `✓` / `半` / `无` / `✓已解` / `⟳本轮补` / `✗仍缺` / `已解` / `已派` / `已立项` / `仍缺`\n" +
		"面 A · 表头 1 处 · 数据行 9\n面 B · 表头 1 处 · 数据行 2\n面 C · 表头 1 处 · 数据行 1\n面 D · 表头 1 处 · 数据行 1\n"
	repo7b, docs7b, reg7b := glfTBuild(t, t.TempDir(), "", "", badCnt)
	if rc, out, _ := glfTRun(t, "--root", repo7b, "--docs-root", docs7b, "--register", reg7b); rc != 2 {
		t.Fatalf("⑦b 计数句不符失败：期望 rc=2，实得 rc=%d out=%q", rc, out)
	} else if !strings.Contains(out, "缺计数句") {
		t.Fatalf("⑦b 缺「缺计数句」判词：%q", out)
	}

	// ⑧ `--no-register`：只判账 ⇒ rc=0（登记件不在盘也不影响）
	repo8, docs8, _ := glfTBuild(t, t.TempDir(), "", "", "")
	if rc, out, _ := glfTRun(t, "--root", repo8, "--docs-root", docs8, "--no-register"); rc != 0 {
		t.Fatalf("⑧ --no-register 失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	}

	// ⑨ `--json` 绿档 ⇒ rc=0 且机读头在
	repo9, docs9, reg9 := glfTBuild(t, t.TempDir(), "", "", "")
	if rc, out, _ := glfTRun(t, "--root", repo9, "--docs-root", docs9, "--register", reg9, "--json"); rc != 0 {
		t.Fatalf("⑨ --json 失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	} else if !strings.Contains(out, `"schema": "zerg/v1"`) || !strings.Contains(out, `"kind": "GapLedgerFace"`) {
		t.Fatalf("⑨ --json 头不符：%q", out)
	}
}

func TestGapLedgerFaceGateSelfTest(t *testing.T) {
	if rc, out, errb := glfTRun(t, "--self-test"); rc != 0 {
		t.Fatalf("--self-test 失败：期望 rc=0，实得 rc=%d out=%q err=%q", rc, out, errb)
	} else if !strings.Contains(out, "自检结论: 全过") {
		t.Fatalf("--self-test 缺「全过」：%q", out)
	}
}

func TestGapLedgerFaceGateListRules(t *testing.T) {
	if rc, out, _ := glfTRun(t, "--list-rules"); rc != 0 {
		t.Fatalf("--list-rules 失败：期望 rc=0，实得 rc=%d", rc)
	} else if !strings.Contains(out, "面 A · ") || !strings.Contains(out, "交叉引用白名单（唯一条）：") {
		t.Fatalf("--list-rules 缺判据面/白名单：%q", out)
	}
}

func TestGapLedgerFaceGateVersion(t *testing.T) {
	if rc, out, _ := glfTRun(t, "--version"); rc != 0 || strings.TrimSpace(out) != glfVersion {
		t.Fatalf("--version 失败：rc=%d out=%q", rc, out)
	}
}
