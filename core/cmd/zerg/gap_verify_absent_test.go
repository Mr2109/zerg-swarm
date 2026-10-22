package main

// gap_verify_absent_test.go —— 缺口 `GAP-20260925-03`（判据自指）的三条牙。
//
// 病：`gap add --verify-cmd` 历来**必须解析到命令树** ⇒ 「某条命令缺失」这类缺口根本记不进账
// （判据指向一个不存在的命令，等于自己把自己挡在门外）。
//
// 治：`--verify-absent` 把校验方向反过来 —— 现读**必须解析不到**才合法入账；若它已经在树里，
// 说明缺口已修或 path 写错 ⇒ 拒收 2 并点名，✗ 不静默收下。
//
// 三条牙（本件）：
//
//	〔甲〕不存在的 path ＋ `--verify-absent` ⇒ rc=0（能入账）
//	〔乙〕已存在的 path ＋ `--verify-absent` ⇒ rc=2 且拒词点名「已在命令树里」
//	〔丙〕不存在的 path ＋ **不带**该旗标 ⇒ rc=2（旧行为一字不动）

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gapVerifyAbsentRun(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := run(argv, &out, &errb)
	return rc, out.String(), errb.String()
}

func TestGapAddVerifyAbsent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_STATE_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "zerg-cli-gaps.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("造空账失败：%v", err)
	}

	base := []string{
		"gap", "add",
		"--symptom", "反向判据：这条命令应当不存在（合成测试，不入真账）",
		"--handmade", "手搓对照：现读树里搜不到该 path",
		"--impact", "命令面",
		"--want-family", "gap",
		"--want-action", "add",
		"--repro-cmd", "zerg gap add --verify-absent",
		"--dry-run",
	}

	// 〔甲〕不存在的 path ＋ --verify-absent ⇒ rc=0
	argvA := append(append([]string{}, base...), "--verify-cmd", "zerg 不存在族 不存在动作", "--verify-absent")
	rcA, outA, errA := gapVerifyAbsentRun(t, argvA...)
	if rcA != 0 {
		t.Fatalf("〔甲〕反向判据必须能入账（rc=%d · stdout=%q · stderr=%q）", rcA, outA, errA)
	}
	t.Logf("〔甲〕rc=0 · 计划面=%q", gapVATestFirstLine(outA))

	// 〔乙〕已存在的 path ＋ --verify-absent ⇒ rc=2 且点名
	argvB := append(append([]string{}, base...), "--verify-cmd", "zerg gap ls", "--verify-absent")
	rcB, _, errB := gapVerifyAbsentRun(t, argvB...)
	if rcB != 2 {
		t.Fatalf("〔乙〕命令已在树里 ⇒ 必须拒收 2（实测 rc=%d · stderr=%q）", rcB, errB)
	}
	if !strings.Contains(errB, "已在命令树里") {
		t.Errorf("〔乙〕拒词要点名「已在命令树里」（stderr=%q）", errB)
	}
	if !strings.Contains(errB, "退码 2") && !strings.Contains(errB, "② path 写错") {
		t.Errorf("〔乙〕要给出下一步（stderr=%q）", errB)
	}
	t.Logf("〔乙〕rc=2 逐字：%q", gapVATestFirstLine(errB))

	// 〔丙〕旧行为：不带旗标 + 不存在的 path ⇒ rc=2（解析面照旧）
	argvC := append(append([]string{}, base...), "--verify-cmd", "zerg 不存在族 不存在动作")
	rcC, _, errC := gapVerifyAbsentRun(t, argvC...)
	if rcC != 2 {
		t.Fatalf("〔丙〕不带 --verify-absent 时解析面必须照旧拒收 2（实测 rc=%d · stderr=%q）", rcC, errC)
	}
	if !strings.Contains(errC, "解析不到命令树") {
		t.Errorf("〔丙〕拒词要点名「解析不到命令树」（stderr=%q）", errC)
	}
	t.Logf("〔丙〕rc=2 逐字：%q", gapVATestFirstLine(errC))
}

func gapVATestFirstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
