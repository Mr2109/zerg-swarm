package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rootMakeFakeRepo 造一个「够格」的仓根：只有 isRepoRoot 的第 ① 判据（稳定件锚
// `core/internal/version/version.go`）。★ 铁律：锚用稳定件，✗ 别用 `.git`（包目录下常有
// 游离嵌套 `.git` 会把仓根错定到包目录）· ✗ 别用本件自己当锚。
func rootMakeFakeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	verDir := filepath.Join(root, "core", "internal", "version")
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(verDir, "version.go"), []byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// 正控：三路各自命中，`via` 与**实走的路**相符。
func TestRootResolvePositiveVia(t *testing.T) {
	repo := rootMakeFakeRepo(t)
	// 路① env（值校验通过 ⇒ 采信并把 via 报成 env）
	if root, via, why := RootResolveForTest(repo, "", ""); root != repo || via != rootViaEnv || why != "" {
		t.Fatalf("env 路：got (%q,%q,%q) want (%q,%q,\"\")", root, via, why, repo, rootViaEnv)
	}
	// 路② 可执行件上溯：exeDir = <root>/bin
	bin := filepath.Join(repo, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if root, via, _ := RootResolveForTest("", bin, ""); root != repo || via != rootViaExe {
		t.Fatalf("exe 路：got (%q,%q) want (%q,%q)", root, via, repo, rootViaExe)
	}
	// 路③ cwd 上溯：wd = <root>/core/cmd/zerg（深三层仍能上溯命中）
	deep := filepath.Join(repo, "core", "cmd", "zerg")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if root, via, _ := RootResolveForTest("", "", deep); root != repo || via != rootViaCwd {
		t.Fatalf("cwd 路：got (%q,%q) want (%q,%q)", root, via, repo, rootViaCwd)
	}
}

// 负控①：非仓目录 ⇒ fail-closed（root=="" / via==""），✗ 不把哪里当仓根，且**出声**。
func TestRootResolveNonRepoFailClosed(t *testing.T) {
	plain := t.TempDir() // 无 core/internal/version/version.go
	root, via, why := RootResolveForTest("", plain, plain)
	if root != "" || via != "" {
		t.Fatalf("非仓目录 ⇒ 应 fail-closed，实得 root=%q via=%q", root, via)
	}
	if strings.TrimSpace(why) == "" {
		t.Fatalf("fail-closed 必须出声（why 非空）")
	}
}

// 负控②：畸形 `ZERG_REPO`（指到非仓目录）⇒ **不给结论**，✗ 不静默采信、✗ 不回落 cwd。
// ★ 牙：这里刻意让 cwd **是**一个真仓根 —— `repoRoot()` 在本支会无条件采信 env 值；
// 本解析必须拒 env 且**不**回落去把 cwd 当仓根（否则这条负控会「绿」成一条假结论）。
func TestRootResolveMalformedEnvRejected(t *testing.T) {
	bad := t.TempDir()          // 非仓目录（畸形 env 值）
	repo := rootMakeFakeRepo(t) // cwd 是**真**仓根
	root, via, why := RootResolveForTest(bad, "", repo)
	if root != "" || via != "" {
		t.Fatalf("畸形 ZERG_REPO ⇒ 不许给结论，实得 root=%q via=%q（回落 cwd = 静默采信）", root, via)
	}
	if !strings.Contains(why, "ZERG_REPO") {
		t.Fatalf("出声须点名 `ZERG_REPO`，实得 %q", why)
	}
	// 成对正控：把 env 撤掉（空串）⇒ 同一条 cwd 路必须命中（证明这条负控的「红」不是探针恒红）。
	if root2, via2, _ := RootResolveForTest("", "", repo); root2 != repo || via2 != rootViaCwd {
		t.Fatalf("成对正控：env 撤掉后 cwd 路应命中，实得 root=%q via=%q", root2, via2)
	}
}
