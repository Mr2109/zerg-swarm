// repo_root.go — 产出引用的**文件解析根**（`Deps.CWD` 的生产赋值来源）。
//
// 为什么要有它：`claims_numbers.go` 的第二半硬核（`文件:行`）只在拿到 `root` 时才判；
// `root` 为空 ⇒ 那条腿**自动休眠**（解析不了根就不猜，口径见 claims_numbers.go 文件头）。
// 于是「测试里亮、线上不亮」—— 单测显式喂 root，生产三个 `loopcore.Deps{…}` 构造点却谁都没赋值。
// 本文件只补「根从哪来」这一格，✗ 不改任何判定逻辑（`VerifyNumericClaims` / `DeriveClaims` 一字未动）。
//
// ── 口径（保守：认不出就返回空）──
//
//	① 先认 **cwd**：从进程当前目录起**向上**找带标记的目录；
//	② 再认 **os.Executable()** 所在目录：同样向上找（二进制常从 `bin/` 启动，cwd 未必在仓内）；
//	③ 两路都不成立 ⇒ **返回空**（`文件:行` 腿继续休眠）—— ✗ 不猜、✗ 不拿 cwd 本身顶替。
//
// 标记必须是**验证过的**（✗ 不靠目录名/件名形状猜）：目录下有 `core/go.mod`（普通件），
// 或目录本身是 module root（有 `go.mod`）且带 `core/` 子目录。
//
// 限深 ≤ `repoRootMaxDepth` 层：向上无限找会把「宿主机某个远祖目录恰好像仓」也算成根，
// 那种根一定是错的（引用串是相对仓根的），宁可返回空。
package loopcore

import (
	"os"
	"path/filepath"
)

// repoRootMaxDepth — 向上找标记的最大层数（含起点目录）。
// 取 8：仓内最深常见路径 `core/internal/<包>/<件>` 距仓根 4 层，留一倍余量；
// 再深就不是「仓内 cwd」了。
const repoRootMaxDepth = 8

// RepoRoot — 产出引用的文件解析根：先认 cwd（向上找含 core/go.mod 的目录），
// 再按 os.Executable() 所在目录向上找同样标记；都不成立返回空（文件:行 腿休眠，✗ 不猜）。
func RepoRoot() string {
	if wd, err := os.Getwd(); err == nil {
		if root, ok := findRepoRootUp(wd); ok {
			return root
		}
	}
	if exe, err := os.Executable(); err == nil {
		// 符号链接（`bin/zerg` → 别处）按真身所在目录找：链接所在的目录往往不是仓。
		if real, lerr := filepath.EvalSymlinks(exe); lerr == nil {
			exe = real
		}
		if root, ok := findRepoRootUp(filepath.Dir(exe)); ok {
			return root
		}
	}
	return ""
}

// findRepoRootUp — 从 dir（含自身）起向上最多 repoRootMaxDepth 层，找**标记已验证**的仓根。
// 到卷根 / 限深用尽 ⇒ ("", false)（✗ 不猜）。
func findRepoRootUp(dir string) (string, bool) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for i := 0; i <= repoRootMaxDepth; i++ {
		if isRepoRootMarker(d) {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d { // 到卷根（"/" 的父还是 "/"）
			return "", false
		}
		d = parent
	}
	return "", false
}

// isRepoRootMarker — 该目录是不是仓根：只认两种**可验证**形态（✗ 不靠名字猜）。
func isRepoRootMarker(dir string) bool {
	if info, err := os.Stat(filepath.Join(dir, "core", "go.mod")); err == nil && info.Mode().IsRegular() {
		return true
	}
	// 另一形态：目录本身是 module root（go.mod）且带 core/ 子目录。
	if info, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && info.Mode().IsRegular() {
		if st, err := os.Stat(filepath.Join(dir, "core")); err == nil && st.IsDir() {
			return true
		}
	}
	return false
}
