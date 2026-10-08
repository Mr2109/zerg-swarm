// negctl_build_wall_test.go —— `scripts/build/negctl-build-wall.sh`（负例活体控制）的 **Go 等强对等体**。
//
// 背景（DEV-2103/2104）：被测真身已是 Go module `scripts/build/cmd/zerg-build`（`run()` 是唯一出口 ·
// `main()` 只 `os.Exit(run(...))`）⇒ 负控**可在进程内直调 `run()`** 取**真实退码**，不必再「拷 module
// → `go build` 出真身二进制 → 跑」（旧 shell 之所以要拷，是因为它只能拿真退码跑进程；退码来源同一）。
//
// 四条牙（本义一字未改 · 判据**只看真实退出码**）：
//
//	C0 控制组（必须先绿）：真仓 `--debug` ⇒ rc=0 且 `wall/target/debug/zerg-wall` 可执行
//	   —— 没有这一条，下面的「红」可能只是「一切都红」（无区分度）。
//	N1 仓不完整（桩树无 `wall/Cargo.toml`）⇒ rc=2，且**不许**产出 `bin/zerg-wall`
//	N2 本机无 cargo（PATH 里没有 cargo）   ⇒ rc=2（**不许**以「跳过」成功退出）
//	N3 认不得的参数                        ⇒ rc=2（静默忽略会让「跑错路线」看不出来）
//
// 真仓/工具链不在位该档 Skip（合成环境 / 公开快照）—— 与同族 `build_wall_test.go` 同规。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// negctlRun —— 进程内直调 `run()`（真身唯一出口），返回 rc / stdout / stderr。
// pathOverride 非空 ⇒ 本次把 PATH 换成它（用来把 cargo 所在目录从 PATH 里摘掉）。
func negctlRun(t *testing.T, pathOverride string, args ...string) (int, string, string) {
	t.Helper()
	if pathOverride != "" {
		t.Setenv("PATH", pathOverride)
	}
	var out, errb bytes.Buffer
	rc := run(args, &out, &errb)
	return rc, out.String(), errb.String()
}

// negctlStubTree —— 造最小桩仓根（可带/不带 `wall/Cargo.toml`），落在仓外临时目录。
func negctlStubTree(t *testing.T, withCargoToml bool) string {
	t.Helper()
	d := t.TempDir()
	if withCargoToml {
		if err := os.MkdirAll(filepath.Join(d, "wall"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "wall", "Cargo.toml"),
			[]byte("# 占位：只为过「Cargo.toml 存在」这一关，构建本身不该走到\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

// TestNegctlBuildWall —— 负例活体控制四条牙（正控绿 · N1~N3 各自真红在 rc=2）。
func TestNegctlBuildWall(t *testing.T) {
	root := discoverRepoRoot()
	if root == "" {
		t.Skip("解析不到仓根（合成环境）⇒ 跳负控：真仓 C0 走不到绿")
	}

	t.Run("C0_真仓_debug_必须先绿", func(t *testing.T) {
		if _, err := exec.LookPath("cargo"); err != nil {
			t.Skipf("本机无 cargo ⇒ C0 走不到绿，负控失去区分度（硬失败，不静默降级）：%v", err)
		}
		rc, _, se := negctlRun(t, "", "--debug")
		if rc != 0 {
			t.Fatalf("C0 rc=%d（要 0）· stderr=%s", rc, se)
		}
		art := filepath.Join(root, "wall", "target", "debug", "zerg-wall")
		if !isExecutable(art) {
			t.Fatalf("C0 产物必须可执行，实际不在/不可执行：%s", art)
		}
	})

	t.Run("N1_仓不完整_无wall_rc2_且不产制品", func(t *testing.T) {
		stub := negctlStubTree(t, false)
		rc, _, se := negctlRun(t, "", "--repo-root", stub)
		if rc != 2 {
			t.Fatalf("N1 rc=%d（要 2）· stderr=%s", rc, se)
		}
		if fileExists(filepath.Join(stub, "bin", "zerg-wall")) {
			t.Fatalf("N1 不许悄悄产出 bin/zerg-wall：%s", filepath.Join(stub, "bin", "zerg-wall"))
		}
	})

	t.Run("N2_无cargo_rc2", func(t *testing.T) {
		stub := negctlStubTree(t, true)
		// 最小 PATH（不含 ~/.cargo/bin；cargo 装在那）⇒ run() 的 exec.LookPath("cargo") 落空。
		rc, _, se := negctlRun(t, "/usr/bin:/bin", "--repo-root", stub)
		if rc != 2 {
			t.Fatalf("N2 rc=%d（要 2 —— 不许以「跳过」成功退出）· stderr=%s", rc, se)
		}
	})

	t.Run("N3_陌生参数_rc2", func(t *testing.T) {
		rc, _, se := negctlRun(t, "", "--wall-wall")
		if rc != 2 {
			t.Fatalf("N3 rc=%d（要 2）· stderr=%s", rc, se)
		}
	})
}
