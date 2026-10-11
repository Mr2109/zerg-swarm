// check_error_kinds_test.go —— Go 真搬样板的**成对负控**定向单测（M2-0 · 4.2 附加四条之「负控成对」）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckErrorKindsGatePort（定向：go test -run TestCheckErrorKindsGatePort）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cekFixture —— 在 t.TempDir() 下造一棵 `core/cmd/zerg/` 夹具树（仓外）。
func cekFixture(t *testing.T, errorsGo string, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "errors.go"), []byte(errorsGo), 0o644); err != nil {
		t.Fatal(err)
	}
	ec := "var exitcodeTable = []exitcodeRow{\n" +
		"\t{0, \"ok\", \"成功\", \"—\"},\n\t{1, \"failed\", \"一般失败\", \"看 kind\"},\n" +
		"\t{2, \"usage\", \"用法错\", \"改用法\"},\n\t{14, \"conflict\", \"冲突\", \"排队\"},\n}\n"
	if err := os.WriteFile(filepath.Join(p, "exitcodes.go"), []byte(ec), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range extra {
		if err := os.WriteFile(filepath.Join(p, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func cekRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := errorKindsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckErrorKindsGatePort(t *testing.T) {
	good := "var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\n"

	// ── 正控（必然判绿）：闭集合规 + 用法面全在闭集内 ─────────────────
	rcGreen, outGreen, _ := cekRun(t, cekFixture(t, good, map[string]string{
		"usage_ok.go": "package main\nfunc f(){ setErr(\"conflict\") }\n",
	}))
	if rcGreen != 0 || !strings.Contains(outGreen, "⇒ 全绿（rc=0）") {
		t.Fatalf("正控失败：好件应 rc=0，得 rc=%d out=%q", rcGreen, outGreen)
	}

	// ── 负控 R1（必然判红）：大写 kind ───────────────────────────────
	rcR1, outR1, _ := cekRun(t, cekFixture(t,
		"var errorKinds = []errorKindRow{\n\t{\"CONFIRM_REQUIRED\", 2, false, \"r\", \"m\", \"s\"},\n}\n", nil))
	if rcR1 != 1 || !strings.Contains(outR1, "R1 kind 命名不合规") {
		t.Fatalf("负控 R1 失败：得 rc=%d out=%q", rcR1, outR1)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcGreen == rcR1 {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcGreen, rcR1)
	}

	// ── 负控 R2 · 重复 kind ─────────────────────────────────────────
	rcDup, outDup, _ := cekRun(t, cekFixture(t,
		"var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\n", nil))
	if rcDup != 1 || !strings.Contains(outDup, "R2 kind 重复") {
		t.Fatalf("负控 R2-重复 失败：得 rc=%d out=%q", rcDup, outDup)
	}

	// ── 负控 R2 · 未登记退码（99 不在退码表） ───────────────────────
	rcGhost, outGhost, _ := cekRun(t, cekFixture(t,
		"var errorKinds = []errorKindRow{\n\t{\"ghost_kind\", 99, false, \"r\", \"m\", \"s\"},\n}\n", nil))
	if rcGhost != 1 || !strings.Contains(outGhost, "不在退码表里") {
		t.Fatalf("负控 R2-未登记 失败：得 rc=%d out=%q", rcGhost, outGhost)
	}

	// ── 负控 R2 · 负退码 ────────────────────────────────────────────
	rcNeg, outNeg, _ := cekRun(t, cekFixture(t,
		"var errorKinds = []errorKindRow{\n\t{\"conflict\", -3, false, \"r\", \"m\", \"s\"},\n}\n", nil))
	if rcNeg != 1 || !strings.Contains(outNeg, "R2 退码为负") {
		t.Fatalf("负控 R2-负码 失败：得 rc=%d out=%q", rcNeg, outNeg)
	}

	// ── 负控 R3 · 用法面命中闭集外 kind ─────────────────────────────
	rcR3, outR3, _ := cekRun(t, cekFixture(t, good, map[string]string{
		"usage_bad.go": "package main\nfunc g(){ setErr(\"outside_kind\") }\n",
	}))
	if rcR3 != 1 || !strings.Contains(outR3, "R3 用到闭集外的 kind") {
		t.Fatalf("负控 R3 失败：得 rc=%d out=%q", rcR3, outR3)
	}

	// ── 空转（≠ 绿）：闭集表空 ⇒ rc=2 不给结论 ──────────────────────
	rcEmpty, outEmpty, _ := cekRun(t, cekFixture(t, "var errorKinds = []errorKindRow{\n}\n", nil))
	if rcEmpty != 2 || !strings.Contains(outEmpty, "不给结论") {
		t.Fatalf("空转失败：空闭集应 rc=2，得 rc=%d out=%q", rcEmpty, outEmpty)
	}

	// ── 取不到输入：根下无 core/cmd/zerg/errors.go ⇒ rc=2 ──────────
	rcNoRoot, outNoRoot, _ := cekRun(t, t.TempDir())
	if rcNoRoot != 2 || !strings.Contains(outNoRoot, "找不到 core/cmd/zerg/errors.go") {
		t.Fatalf("取不到输入失败：得 rc=%d out=%q", rcNoRoot, outNoRoot)
	}

	// ── Q-176：本门无机器读面 ⇒ --json 拒收 rc=2（stderr 明说） ────
	rcJSON, _, errJSON := cekRun(t, "--json")
	if rcJSON != 2 || !strings.Contains(errJSON, "本门无机器读面") {
		t.Fatalf("--json 拒收失败：得 rc=%d stderr=%q", rcJSON, errJSON)
	}

	// ── --self-test 成对负控自身必须全过（防恒绿/恒红） ─────────────
	rcST, outST, _ := cekRun(t, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自检结论: 全过") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}
}

// TestCheckErrorKindsReadFailTeeth —— B 类读失败不再静默的回归牙：R3 扫描面里一件 .go 锁成
// chmod 000 ⇒ 登记面（cekReadFails）必响：rc=2 + stderr BLOCKED 头 + 逐件点名。
// 删回静默（原 os.ReadFile 失败即 continue）本用例必红。
func TestCheckErrorKindsReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 无视权限位（chmod 000 仍可读）⇒ 跳过")
	}
	good := "var errorKinds = []errorKindRow{\n\t{\"conflict\", 14, false, \"r\", \"m\", \"s\"},\n}\n"
	root := cekFixture(t, good, map[string]string{
		"usage_ok.go": "package main\nfunc f(){ setErr(\"conflict\") }\n",
		"locked.go":   "package main\nfunc z(){ setErr(\"conflict\") }\n",
	})
	locked := filepath.Join(root, "core", "cmd", "zerg", "locked.go")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod 000 失败：%v", err)
	}
	defer func() { _ = os.Chmod(locked, 0o644) }()
	rc, out, errb := cekRun(t, root)
	if rc != 2 || !strings.Contains(errb, "BLOCKED") || !strings.Contains(errb, "core/cmd/zerg/locked.go") {
		t.Fatalf("读不到件应 rc=2 + BLOCKED 点名 locked.go，得 rc=%d out=%q err=%q", rc, out, errb)
	}
}
