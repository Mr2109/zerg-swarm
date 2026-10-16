// cli_gate_matrix_atomic_test.go —— 缺口 `GAP-20260927-303` / `GAP-20260927-304` 的**真二进制**判据。
//
// 两条都是「静默」类缺陷，判据都要**先造旧行为的反例**：
//
//	① `GAP-20260927-303`：`--out` 写面改**原子替换**（同目录临时件 → `os.Rename` 换名）。
//	   负控：落点已存在且**临时件建不出来**（目录不可写）时，旧形态 `os.WriteFile` 先把落点**截断**
//	   再写 ⇒ 旧行为下这一格 FAIL（落点被毁）；新形态退非 0 且落点**逐字节不动**。
//	② `GAP-20260927-304`：`--out`（写面 · 拆件落盘）与 `--json`（判面 · 机读投影）同给 ⇒ 显式
//	   **用法错 2** 且件不落 —— 旧形态两档都跑（既落件又出机读面、rc=0）⇒ 旧行为下这一格 FAIL。
//
// ★ 本批**禁重编 `bin/`** ⇒ 判据从**当前源码现编**一份 CLI（`zergBinaryFromSource` —— 只落到仓外临时
//
//	目录，✗ 不碰 `bin/zerg`）：否则打的是旧制品、新行为面永远量不到。负控走
//	`GOFLAGS=-overlay=<把本件映回改前版的 json>` 让这次现编打**改前源码** ⇒ 上面两格 FAIL。
//
// 两格都用**仓外临时目录**做落点（`t.TempDir()`），✗ 不碰真仓内任何基线件。
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gateMatrixRepoRoot —— 真仓根（`gate matrix` 的矩阵真源 = `<root>/core/cmd/zerg/testdata/cli-matrix.json`）。
// 测试 cwd = 包目录 `core/cmd/zerg` ⇒ 上三级 = 仓根（与 `zergBinary` 的 `../../../bin/zerg` 同源）。
func gateMatrixRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "core", "cmd", "zerg", "testdata", "cli-matrix.json")); err != nil {
		t.Skipf("仓根解析不到矩阵真源（本格跳过，不算行为面证据）：%v", err)
	}
	return root
}

// TestGateMatrix_OutJsonMutex —— GAP-20260927-304：写面不得兼作判面。
//
// ① 负控格（旧行为 FAIL）：`--out` + `--json` 同给 ⇒ 2、件不落、stdout 0 字节（判在任何 stdout 写入之前）。
// ② 正控格（互斥不许误伤既有档）：`--out` 单独给 ⇒ 0 且落件；`--json` 单独给（纯读面）⇒ 0。
func TestGateMatrix_OutJsonMutex(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := gateMatrixRepoRoot(t)

	outFile := filepath.Join(t.TempDir(), "matrix-mutex.tsv")
	rc, stdout, stderr := execCase(t, bin, root, "gate", "matrix", "--out", outFile, "--yes", "--json", "command,case,want_rc")
	if rc != 2 {
		t.Errorf("`--out` 与 `--json` 同给 ⇒ 用法错 2，得到 %d · stderr=%s", rc, stderr)
	}
	if _, err := os.Stat(outFile); !os.IsNotExist(err) {
		t.Errorf("同给却落了件（写面兼作判面）：%v", err)
	}
	// ★ 缺口 `GAP-20260929-41`（本批）：`out_json_conflict` 是**用法错**（`kind=usage`）⇒ **不吐包封**
	//   ⇒ stdout **零字节**（旧断言「包封含 detail=out_json_conflict 应走 stdout」已过期；本断言在
	//   旧行为下会红 ⇒ 有牙）。本条只认「一个字节都不出」（写面兼作判面那一格也不许出结果面）。
	if stdout != "" {
		t.Errorf("`--out` 与 `--json` 同给（用法错）⇒ stdout 必须零字节（GAP-20260929-41），得到 %q", stdout)
	}
	if !strings.Contains(stderr, "不许同给") {
		t.Errorf("判词没点名两档不许同给：%s", stderr)
	}

	okFile := filepath.Join(t.TempDir(), "matrix-ok.tsv")
	if rc, _, errb := execCase(t, bin, root, "gate", "matrix", "--out", okFile, "--yes"); rc != 0 {
		t.Errorf("`--out` 单独给 ⇒ 0（互斥误伤），得到 %d · stderr=%s", rc, errb)
	}
	if _, err := os.Stat(okFile); err != nil {
		t.Errorf("`--out` 单独给应落件：%v", err)
	}
	if rc, _, _ := execCase(t, bin, root, "gate", "matrix", "--json", "command,case,want_rc"); rc != 0 {
		t.Errorf("`--json` 单独给（纯读面）⇒ 0（互斥误伤），得到 %d", rc)
	}
}

// TestGateMatrix_OutAtomicReplace —— GAP-20260927-303：`--out` 原子替换，落点不被半成品/失败破坏。
func TestGateMatrix_OutAtomicReplace(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := gateMatrixRepoRoot(t)

	// ① 正控：落点已存在 ⇒ 被**完整替换**（旧内容一字节不留；换名落件 = 新内容全量在）。
	outFile := filepath.Join(t.TempDir(), "matrix.tsv")
	if err := os.WriteFile(outFile, []byte("SENTINEL-OLD-CONTENT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, _, errb := execCase(t, bin, root, "gate", "matrix", "--out", outFile, "--yes"); rc != 0 {
		t.Fatalf("导出（带 --yes）⇒ 0，得到 %d · stderr=%s", rc, errb)
	}
	got, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("落点读不回来：%v", err)
	}
	if strings.Contains(string(got), "SENTINEL-OLD-CONTENT") {
		t.Errorf("落点没被完整替换（旧内容还在 = 不是换名落件）")
	}
	if !strings.HasPrefix(string(got), "# 命令面 must-fail 矩阵") {
		first := string(got)
		if i := strings.IndexByte(first, '\n'); i >= 0 {
			first = first[:i]
		}
		t.Errorf("落点内容不是导出的矩阵：%q", first)
	}

	// ② 负控格（旧行为 FAIL）：临时件建不出来（目录不可写）⇒ 退非 0 且落点**逐字节不动**。
	//   旧形态 `os.WriteFile` 会先把已存在的落点截断再写（目录写位不是打开既存件的前提）⇒ 落点被毁。
	if os.Geteuid() == 0 {
		t.Skip("root 不受目录写位约束 ⇒ 跳过该负控")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "matrix-sem.tsv")
	if err := os.WriteFile(target, []byte("SENTINEL-KEEP\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dir, 0o755) }() // 还写位，让 t.TempDir() 清理得掉
	rc, _, errb := execCase(t, bin, root, "gate", "matrix", "--out", target, "--yes")
	if rc == 0 {
		t.Errorf("临时件建不出来却退 0（非原子落件）：stderr=%s", errb)
	}
	kb, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("原子替换：中途失败不该动落点，却读不回来：%v", err)
	}
	if string(kb) != "SENTINEL-KEEP\n" {
		t.Errorf("中途失败把落点破坏了（旧内容 %q ⇒ 现 %q）—— 非原子落件", "SENTINEL-KEEP\n", string(kb))
	}
}
