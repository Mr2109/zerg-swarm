// check_zh_en_test.go —— `check_zh_en.go` 的**成对负控**定向单测（M2 · 承接 GAP-20261001-13）。
//
// ✗ 禁恒绿：本件同时断言「必然判绿」「必然判红」「不给结论（≠绿）」三态 —— 任一态被写死都必失败。
// 用例名：TestCheckZhEnGatePort（定向：go test -run TestCheckZhEnGatePort）。
//
// 夹具纪律：根由 `t.TempDir()` 建后经 `filepath.EvalSymlinks` 解真路径 ⇒ **落在非符号链接路径**上
// （macOS `/var` ⟷ `/private/var`）—— 与「夹具一律落非符号链接路径」那条同规。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chzeFxForbid —— 自证用被检词（运行时串拼 · ✗ 不落裸字面量 ⇒ 本件不被自己抓）。
var chzeFxForbid = "\u866b\u5de2"

// chzeFxRoot —— 造一棵仓外夹具（core/ 空目录）并返回其**真路径**（非符号链接）。
func chzeFxRoot(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	if r, err := filepath.EvalSymlinks(base); err == nil {
		base = r
	}
	if err := os.MkdirAll(filepath.Join(base, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}

// chzeRunIn —— 切 cwd 到 root 后跑入口（缺省旋钮 ⇒ 复刻 py 的 cwd 相对 ROOTS）。
func chzeRunIn(t *testing.T, root string, argv ...string) (int, string, string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	rc := checkZhEnGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// chzeRunAt —— 不切 cwd（考 `--root` 显式旋钮）。
func chzeRunAt(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkZhEnGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// chzeSaveFP —— 用例可临时改登记表（白盒 · 同 py selftest_fp 的做法），跑完复原。
func chzeSaveFP(t *testing.T) {
	t.Helper()
	saved := chzeKnownFP
	t.Cleanup(func() { chzeKnownFP = saved })
}

func TestCheckZhEnGatePort(t *testing.T) {
	fx := chzeFxRoot(t)
	chzeSaveFP(t)
	def := chzeKnownFP // 默认登记表（供 ⑦ 复原口径）

	// ── ① 正控（好件 ⇒ 0 · 登记表清空 ⇒ 不触登记缺失档）─────────────────────────────
	chzeKnownFP = nil
	if err := os.WriteFile(filepath.Join(fx, "core", "good.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcOK, outOK, _ := chzeRunIn(t, fx)
	if rcOK != 0 || !strings.Contains(outOK, "合计命中 = 0") {
		t.Fatalf("正控失败：好件应 rc=0 且合计命中 0，得 rc=%d out=%q", rcOK, outOK)
	}

	// ── ② 负控（禁用词命中 ⇒ 1）：核心件写入被检词（运行时串拼）──────────────────────
	if err := os.WriteFile(filepath.Join(fx, "core", "bad.go"), []byte("var x = \""+chzeFxForbid+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcBad, outBad, _ := chzeRunIn(t, fx)
	if rcBad != 1 || !strings.Contains(outBad, "禁用词：1 处") {
		t.Fatalf("禁用词负控失败：应 rc=1 且禁用词 1 处，得 rc=%d out=%q", rcBad, outBad)
	}
	// 成对（✗ 禁恒绿）：绿 ↔ 红 必须不同码
	if rcOK == rcBad {
		t.Fatalf("成对断言失败：正控 rc=%d 与负控 rc=%d 同码 ⇒ 无鉴别力", rcOK, rcBad)
	}
	os.Remove(filepath.Join(fx, "core", "bad.go"))

	// ── ③ 负控（全角标点(代码区) ⇒ 1）：.sh 代码区写全角（.go 不受全额面管辖）──────────
	if err := os.WriteFile(filepath.Join(fx, "core", "f.sh"), []byte("if [ $a = 1，]; then\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rcF, outF, _ := chzeRunIn(t, fx)
	if rcF != 1 || !strings.Contains(outF, "全角标点(代码区)：1 处") {
		t.Fatalf("全角负控失败：应 rc=1 且全角 1 处，得 rc=%d out=%q", rcF, outF)
	}
	if rcF == rcOK {
		t.Fatalf("成对断言失败：全角负控 rc=%d 与正控 rc=%d 同码", rcF, rcOK)
	}
	os.Remove(filepath.Join(fx, "core", "f.sh"))

	// ── ④ 不给结论（≠ 绿）：[ ] 空扫（扫到 0 件）⇒ 2 ────────────────────────────────
	empty := chzeFxRoot(t)
	rcE, _, errE := chzeRunIn(t, empty)
	if rcE != 2 || !strings.Contains(errE, "扫到 0 个文件") {
		t.Fatalf("空扫失败：应 rc=2 且点名空转，得 rc=%d stderr=%q", rcE, errE)
	}

	// ── ⑤ 不给结论（≠ 绿）：null 形态（core/ 与 agent/ 都取不到）⇒ 2 ─────────────────
	null := t.TempDir()
	if r, err := filepath.EvalSymlinks(null); err == nil {
		null = r
	}
	rcN, _, errN := chzeRunIn(t, null)
	if rcN != 2 || !strings.Contains(errN, "不在仓库根目录") {
		t.Fatalf("null 失败：应 rc=2 且点名不在仓根，得 rc=%d stderr=%q", rcN, errN)
	}

	// ── ⑥ 读不了的件 ⇒ 逐字沿用 py（⚠ 告警 + continue · **不抬 rc** · 非 rc=2）────────
	chzeKnownFP = nil
	noRead := filepath.Join(fx, "core", "noread.go")
	if err := os.WriteFile(noRead, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(noRead, 0); err != nil {
		t.Fatal(err)
	}
	rcNR, _, errNR := chzeRunIn(t, fx)
	if rcNR != rcOK || !strings.Contains(errNR, "读不了") {
		t.Fatalf("读不了件失败：应 rc=%d（不抬档）且 stderr 点名，得 rc=%d stderr=%q", rcOK, rcNR, errNR)
	}
	os.Chmod(noRead, 0o644)
	os.Remove(noRead)

	// ── ⑦ 不给结论（≠ 绿）：登记表指向的件不在盘（缺键形态）⇒ 2 ──────────────────────
	chzeKnownFP = def
	rcK, _, errK := chzeRunIn(t, fx)
	if rcK != 2 || !strings.Contains(errK, "登记表指向的件不在盘") {
		t.Fatalf("登记缺失失败：应 rc=2 且点名，得 rc=%d stderr=%q", rcK, errK)
	}
	chzeKnownFP = nil

	// ── ⑧ 未知 argv 一律不解析（py 无参数校验 ⇒ 忽略）· 正控 rc=0 已成对 ──────────────
	rcBog, outBog, _ := chzeRunIn(t, fx, "--bogus")
	if rcBog != 0 || outBog != outOK {
		t.Fatalf("未知 argv 失败：应忽略且与正控逐字同，得 rc=%d（正控 %d）", rcBog, rcOK)
	}

	// ── ⑨ 显式旋钮 `--root <目录>`（新口径 · cwd 不切）⇒ 指向夹具仍 rc=0 ───────────────
	rcR, outR, _ := chzeRunAt(t, "--root", fx)
	if rcR != 0 || !strings.Contains(outR, "扫描文件数 = 1") {
		t.Fatalf("--root 旋钮失败：应 rc=0 且扫 1 件，得 rc=%d out=%q", rcR, outR)
	}

	// ── ⑩ --self-test 自身必须全过（防恒绿/恒红）⇒ rc=0 + 末行自印 ────────────────────
	rcST, outST, _ := chzeRunIn(t, fx, "--self-test")
	if rcST != 0 || !strings.Contains(outST, "自证：通过 ✓") || !strings.Contains(outST, "14 条单行") {
		t.Fatalf("--self-test 失败：得 rc=%d out=%q", rcST, outST)
	}

	// ── ⑪ 旗标别名 `--selftest` 同档（py 两拼法都认）⇒ rc=0 ──────────────────────────
	rcSA, outSA, _ := chzeRunIn(t, fx, "--selftest")
	if rcSA != 0 || !strings.Contains(outSA, "自证：通过 ✓") {
		t.Fatalf("--selftest 别名失败：得 rc=%d", rcSA)
	}
}

// ── B 类（读失败静默）回归牙：登记表指向的一件锁成 chmod 000 ⇒
// 主件 chzeBodyOf 的 os.ReadFile 失败登记面（chzeReadFails）必响：
// rc=2 + stderr BLOCKED 头 + 逐件点名那份锁死件。登记/判红若删回静默，本用例必红。
func TestCheckZhEnReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 绕过权限位 ⇒ chmod 000 不挡读，本用例无鉴别力")
	}
	fx := chzeFxRoot(t)
	chzeSaveFP(t)
	if err := os.WriteFile(filepath.Join(fx, "core", "good.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(fx, "core", "locked.sh")
	if err := os.WriteFile(locked, []byte("if [ $a = 1 ]; then\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o644)
	chzeKnownFP = []chzeFPDef{{"core/locked.sh", "if [ $a = 1 ]", "回归牙：锁死登记件"}}
	rc, _, errb := chzeRunIn(t, fx)
	if rc != 2 {
		t.Fatalf("读失败回归牙：登记件锁死 ⇒ 应 rc=2（登记 + 判红），得 rc=%d stderr=%q", rc, errb)
	}
	if !strings.Contains(errb, "BLOCKED") {
		t.Fatalf("读失败回归牙：stderr 缺 BLOCKED 头，得 stderr=%q", errb)
	}
	if !strings.Contains(errb, "locked.sh") {
		t.Fatalf("读失败回归牙：未逐件点名锁死件 locked.sh，得 stderr=%q", errb)
	}
}
