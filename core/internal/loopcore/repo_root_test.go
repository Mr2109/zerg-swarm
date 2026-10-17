// repo_root_test.go — `Deps.CWD`（产出引用的文件解析根）的两态取证。
//
// 取证纪律（与 claims_numbers_test.go 同一套）：判据用**真实的** `RepoRoot()` / `VerifyNumericClaims` /
// `VerifyClaims` 算，不在本件里复述口径；✗ 不改进程 cwd 后不恢复（用 `t.Chdir`，它自带清理）。
//
// 覆盖：
//
//	(a) 仓内 cwd ⇒ 非空，且该根下 `core/go.mod` 真存在
//	(b) 无标记目录 ⇒ 空（认不出就不猜 ⇒ `文件:行` 腿保持休眠）
//	(c) 限深：标记在 9 层以上找不到、8 层以内找得到（✗ 无限上溯）
//	(d) 集成断言：`CWD` 非空 ⇒ 真件 ＋ `:999999` 判 retract；同一正文 `CWD` 空 ⇒ 不判（休眠态）
package loopcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── (a) 仓内 cwd ⇒ 非空 + 标记可验证 ─────────────────────────────────────────

func TestRepoRoot_FromRepoCwd(t *testing.T) {
	root := RepoRoot()
	if strings.TrimSpace(root) == "" {
		t.Fatalf("★ 在仓内（本包目录）调 RepoRoot() 必须非空 —— 空 ⇒ 生产上 `文件:行` 腿永远休眠")
	}
	if !filepath.IsAbs(root) {
		t.Fatalf("解析根应是绝对路径，实际 %q", root)
	}
	marker := filepath.Join(root, "core", "go.mod")
	info, err := os.Stat(marker)
	if err != nil {
		t.Fatalf("★ 解析根下必须真有 core/go.mod（%s）：%v", marker, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("★ core/go.mod 应是普通件，实际 mode=%s", info.Mode())
	}
	// 反证「不是拿 cwd 顶替」：包目录本身不是仓根。
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("取 cwd 失败：%v", err)
	}
	if root == wd {
		t.Fatalf("★ 解析根不能等于 cwd（本包目录 %s 不是仓根，向上找才有标记）", wd)
	}
	t.Logf("(a) RepoRoot()=%s（core/go.mod 在位）", root)
}

// ── (b) 无标记目录 ⇒ 空 ─────────────────────────────────────────────────────

func TestRepoRoot_NoMarkerGoesEmpty(t *testing.T) {
	t.Chdir(t.TempDir()) // t.Chdir 自带复原；✗ 不裸 os.Chdir
	if got := RepoRoot(); got != "" {
		t.Fatalf("★ 无标记目录（临时目录，向上无 core/go.mod）⇒ 必须返回空（✗ 不猜），实际 %q", got)
	}
	t.Log("(b) 无标记 ⇒ 空（腿休眠）")
}

// ── (c) 限深：≤8 层找得到、9 层找不到 ────────────────────────────────────────

func TestRepoRoot_DepthLimited(t *testing.T) {
	root := fakeRepoRoot(t)
	near := descend(t, root, 8)
	if got, ok := findRepoRootUp(near); !ok || got != root {
		t.Fatalf("★ 距标记 8 层应找得到根 %q，实际 (%q, %v)", root, got, ok)
	}
	far := descend(t, root, 9)
	if got, ok := findRepoRootUp(far); ok {
		t.Fatalf("★ 距标记 9 层必须找不到（限深 ≤%d），实际返回 %q —— 无限上溯会把远祖目录误当仓根", repoRootMaxDepth, got)
	}
	t.Logf("(c) 限深：8 层命中 / 9 层不命中（上限 %d）", repoRootMaxDepth)
}

// ── (d) 集成断言：CWD 非空 ⇒ 文件:行 腿判红；CWD 空 ⇒ 不判 ──────────────────

func TestRepoRoot_LightsFileLineLeg(t *testing.T) {
	root := RepoRoot()
	if strings.TrimSpace(root) == "" {
		t.Fatalf("★ 仓内 cwd 下 RepoRoot() 必须非空（否则线上 `文件:行` 腿永远休眠）")
	}
	// 真存在的件（本包的 kernel.go）＋ 一个必超界的行号 ⇒ 只可能是「行号超出总行数」这条判据。
	const ref = "core/internal/loopcore/kernel.go:999999"
	if _, err := os.Stat(filepath.Join(root, "core", "internal", "loopcore", "kernel.go")); err != nil {
		t.Fatalf("解析根下应真有该件（%s）：%v", ref, err)
	}
	content := "交付：见 " + ref + " 的 Deps.CWD 字段（函数体完整）。"

	lit := VerifyNumericClaims(content, nil, root)
	if len(lit.HardFailed) != 1 {
		t.Fatalf("★ CWD 非空 ⇒ `文件:行` 腿必须判红 1 条，实际 %d 条：%+v", len(lit.HardFailed), lit.HardFailed)
	}
	if v := lit.HardFailed[0].Verdict; v != VerdictRetract {
		t.Fatalf("★ 行号超出总行数 ⇒ 必须判 retract（假回执），实际 %s", v)
	}
	if got := lit.HardFailed[0].Claim.Quote; got != ref {
		t.Fatalf("片段应是引用串本身 %q，实际 %q", ref, got)
	}
	if !strings.HasPrefix(lit.HardFailed[0].Claim.ReceiptID, ClaimNonexistentPrefix) {
		t.Fatalf("合成回执号应以 %q 开头，实际 %q", ClaimNonexistentPrefix, lit.HardFailed[0].Claim.ReceiptID)
	}

	// 对照组（接线前/根解析不到的那一态）：同一正文、root 空 ⇒ 不判。
	dormant := VerifyNumericClaims(content, nil, "")
	if len(dormant.HardFailed) != 0 {
		t.Fatalf("★ CWD 空 ⇒ `文件:行` 腿应休眠（不判），实际判了 %d 条：%+v", len(dormant.HardFailed), dormant.HardFailed)
	}
	t.Logf("(d) 两态：CWD=%s ⇒ %s（1 条）· CWD 空 ⇒ 0 条", root, lit.HardFailed[0].Verdict)
}

// fakeRepoRoot — 造一个带标记的假仓根（仓外；core/go.mod 为普通件）。
func fakeRepoRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "fakeroot")
	if err := os.MkdirAll(filepath.Join(root, "core"), 0o755); err != nil {
		t.Fatalf("建假仓根失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "go.mod"), []byte("module fake\n"), 0o644); err != nil {
		t.Fatalf("写 core/go.mod 失败：%v", err)
	}
	return root
}

// descend — 从 base 往下建 depth 层子目录，返回最深那层。
func descend(t *testing.T, base string, depth int) string {
	t.Helper()
	d := base
	for i := 0; i < depth; i++ {
		d = filepath.Join(d, "d")
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("建子目录失败（第 %d 层）：%v", i+1, err)
		}
	}
	return d
}
