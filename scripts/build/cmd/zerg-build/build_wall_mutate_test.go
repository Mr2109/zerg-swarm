// build_wall_mutate_test.go —— `scripts/build/build-wall-mutate.py`（变异验证）的 **Go 等强对等体**。
//
// 证 `TestNegctlBuildWall`（负例活体控制）**真的会红**：把被测真身 `cmd/zerg-build/main.go` 的三条分支
// **真的改坏**（`return 2` → `return 0`），断言负控**必须红在那一条**对照项上，再断言源码逐字节一致、
// 断言复跑绿。
//
// ★ 与 py 原件的**唯一手段差**（本义不变）：变异**经 `go test -overlay`** 施加在**编译期** —— overlay
// 把仓内 `main.go` 映射到**仓外变异副本** ⇒ **仓内源一字不改**（py 原件是原地改再 `finally` 还原）。
// 这正是本单要求「✗ 不真改仓内源」的落法；而「改坏了必须被负控吃到」这条判据与 py 逐字同。
//
// 三条变异各打在**一条**控制项上（换着红，才说明不是「一切都红」）：
//
//	M1 无 cargo 时以 0 退出（「跳过却像绿」）  ⇒ 必红 N2
//	M2 仓不完整（无 wall/Cargo.toml）时以 0 退出 ⇒ 必红 N1
//	M3 认不得的参数静默忽略（以 0 退出）       ⇒ 必红 N3
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mutateNegctlTestName —— 负控顶层测试名（`-run` 用）；子用例名逐字见各 arm 的 expectFail。
const mutateNegctlTestName = "TestNegctlBuildWall"

// mutateArm —— 一条变异：锚点原文（逐字取自现读真身 main.go）＋ 必须红在的负控子用例名。
var mutateArms = []struct {
	name       string
	anchor     string
	expectFail string
}{
	{
		"M1 无 cargo 时以 0「跳过」（main.go 无 cargo 分支）",
		"\tcargoPath, err := exec.LookPath(\"cargo\")\n" +
			"\tif err != nil {\n" +
			"\t\tfmt.Fprintln(stderr, \"❌ 本机没有 cargo（rust 工具链）⇒ 编不出茧壁；构建段见 wall/README.md\")\n" +
			"\t\treturn 2\n" +
			"\t}\n",
		"N2_无cargo_rc2",
	},
	{
		"M2 仓不完整（无 wall/Cargo.toml）时以 0 退出（main.go 仓不完整分支）",
		"\tif !fileExists(filepath.Join(wallDir, \"Cargo.toml\")) {\n" +
			"\t\tfmt.Fprintf(stderr, \"❌ 茧壁源码不在 %s/Cargo.toml ⇒ 仓不完整（wall/ 是仓内一等公民，不走跳过这条路）\\n\", wallDir)\n" +
			"\t\treturn 2\n" +
			"\t}\n",
		"N1_仓不完整_无wall_rc2_且不产制品",
	},
	{
		"M3 认不得的参数静默忽略（main.go default 分支）",
		"\t\tdefault:\n" +
			"\t\t\tfmt.Fprintf(stderr, \"❌ 未知参数: %s（本件只认 --debug / --dist / --repo-root / --help，不静默忽略）\\n\", args[i])\n" +
			"\t\t\treturn 2\n" +
			"\t\t}\n",
		"N3_陌生参数_rc2",
	},
}

func mutateSHA(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func mutateYesNo(b bool) string {
	if b {
		return "有"
	}
	return "没有"
}

// mutateRunNegctlTest —— 在 module 里跑负控**测试**（可带 overlay），返回 rc 与合并输出。
// 不带 overlay ⇒ 起点/复跑两态；带 overlay ⇒ 变异态（编译期换上仓外变异副本）。
func mutateRunNegctlTest(t *testing.T, goBin, moduleDir, overlayPath string) (int, string) {
	t.Helper()
	args := []string{"test", "-count=1"}
	if overlayPath != "" {
		args = append(args, "-overlay="+overlayPath)
	}
	args = append(args, "-run", "^"+mutateNegctlTestName+"$", "./cmd/zerg-build")
	cmd := exec.Command(goBin, args...)
	cmd.Dir = moduleDir
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	rc := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("跑负控测试起不来：%v", err)
		}
		rc = ee.ExitCode()
	}
	return rc, buf.String()
}

// TestBuildWallMutate —— 起点绿 → 三条变异各自红在预期控制项 → 仓内源逐字节未动 → 复跑绿。
func TestBuildWallMutate(t *testing.T) {
	root := discoverRepoRoot()
	if root == "" {
		t.Skip("解析不到仓根（合成环境）⇒ 跳变异验证")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("本机无 go 工具链 ⇒ 变异体无法编译校验（硬失败，不静默降级）：%v", err)
	}
	if _, err := exec.LookPath("cargo"); err != nil {
		t.Skipf("本机无 cargo ⇒ 负控起点走不到绿（无区分度）：%v", err)
	}
	moduleDir := filepath.Join(root, "scripts", "build")
	mainGo := filepath.Join(moduleDir, "cmd", "zerg-build", "main.go")
	if !fileExists(mainGo) {
		t.Skipf("真身不在（仅桩树/公开快照会走到这里）：%s", mainGo)
	}
	orig, err := os.ReadFile(mainGo)
	if err != nil {
		t.Fatal(err)
	}
	origSHA := mutateSHA(orig)

	t.Log("=== 起点：负例控制必须先绿（否则「红」没有意义）===")
	if rc, out := mutateRunNegctlTest(t, goBin, moduleDir, ""); rc != 0 {
		t.Fatalf("起点不绿（rc=%d）⇒ 不进入变异，先修控制本身：\n%s", rc, out)
	}
	t.Logf("  PASS 起点绿 · sha256(scripts/build/cmd/zerg-build/main.go)=%s", origSHA[:16])

	tmp := t.TempDir()
	for i, arm := range mutateArms {
		t.Logf("=== 变异：%s（必须红在 %s）===", arm.name, arm.expectFail)
		text := string(orig)
		if hits := strings.Count(text, arm.anchor); hits != 1 {
			t.Fatalf("锚点命中 %d 次（必须恰好 1）⇒ 硬失败，本变异作废", hits)
		}
		repl := strings.Replace(arm.anchor, "return 2", "return 0", 1)
		mutPath := filepath.Join(tmp, fmt.Sprintf("main-mut-%d.go", i))
		if err := os.WriteFile(mutPath, []byte(strings.Replace(text, arm.anchor, repl, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		ovPath := filepath.Join(tmp, fmt.Sprintf("ov-%d.json", i))
		ovBytes, err := json.Marshal(map[string]map[string]string{"Replace": {mainGo: mutPath}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ovPath, ovBytes, 0o644); err != nil {
			t.Fatal(err)
		}

		// 变异体须先**编得过**（避免把「编译坏了」误当「判据红了」）—— `go build` 走同一 overlay。
		bo := exec.Command(goBin, "build", "-overlay="+ovPath,
			"-o", filepath.Join(tmp, fmt.Sprintf("mutbin-%d", i)), "./cmd/zerg-build")
		bo.Dir = moduleDir
		if bout, berr := bo.CombinedOutput(); berr != nil {
			t.Fatalf("变异体编译不过 ⇒ 本变异作废（坏的应是行为，不是编译）：%v\n%s", berr, bout)
		}

		rc, out := mutateRunNegctlTest(t, goBin, moduleDir, ovPath)
		failLine := "--- FAIL: " + mutateNegctlTestName + "/" + arm.expectFail
		hasRed := strings.Contains(out, failLine)
		t.Logf("  负控 rc=%d；期望红的那条在输出里：%s", rc, mutateYesNo(hasRed))
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "--- FAIL: ") {
				t.Logf("   %s", strings.TrimSpace(line))
			}
		}
		if rc == 0 || !hasRed {
			t.Errorf("变异不符期望：rc=%d（期望非 0）· 期望红在 %s（输出里有：%s）",
				rc, arm.expectFail, mutateYesNo(hasRed))
		}
	}

	// 还原复核：overlay 从不动仓内源 ⇒ 现读 main.go 必与起点逐字节同（此即「还原一致」的等价证）。
	now, err := os.ReadFile(mainGo)
	if err != nil {
		t.Fatal(err)
	}
	if got := mutateSHA(now); got != origSHA {
		t.Fatalf("仓内源 sha 与起点不一致（overlay 本不该改仓内源）：%s != %s", got[:16], origSHA[:16])
	}
	t.Logf("  PASS 仓内源逐字节未动（sha256=%s 与起点同）", origSHA[:16])

	rc, out := mutateRunNegctlTest(t, goBin, moduleDir, "")
	t.Logf("  复跑负例控制 rc=%d", rc)
	if rc != 0 {
		t.Fatalf("还原后复跑不绿：\n%s", out)
	}
	t.Log("  PASS 复跑绿")
}
