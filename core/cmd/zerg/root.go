// root.go —— 仓根解析（供门禁直通与帮助导出用）。
//
// 红线（scripts/gates/check-hardcoded-private-paths.py）：代码里**不许写死私有绝对路径**
// （`<volume-path>` / `~…`）—— 公开快照会把它们替换成占位符，机群上的二进制
// 就指向不存在的目录。故这里只按三条**可推导**的路走：环境变量 → 可执行文件所在目录上溯 →
// 当前目录上溯；判据是「仓根有 core/internal/version/version.go」。
//
// ★ 危险面（本单只登记 · 零行为变化 · 缺口 `GAP-20260927-436/437/438`）：第 16 行那枚 `ZERG_REPO`
// 是**无条件采信**的 —— 设了就**换掉仓根**，不校验它是不是仓根（也不回退到下面两条推导路）。
// 指到非仓根时下游表现**不一致**（2026-09-27 按件现读）：`gate run` / `gate ls` 硬错 → rc=2
// （门禁最小入口不在）；`repo status` / `repo commit` 硬错 → rc=8（所指位置下没有 .git）；
// `doctor` 对**非仓根**（无仓根判据件）已报 BLOCKED（缺口 `GAP-20260927-436`）；而**公开树物料化快照**
// 因带同一枚判据件曾被误报「仓库根 **PASS**」⇒ 由 `isRepoRoot` 的**快照识别**收口（缺口 `GAP-20261006-08`）。
// ⇒ **写面 / 门禁动作请显式 `env -u ZERG_REPO`**。脚本侧同族旋钮：`ZERG_REPO_ROOT`（`scripts/gates/`
// 下 6 处 `REPO_ROOT` 覆盖；与命令面这两枚**不是同一个名字、也无单一真源**）。
package main

import (
	"os"
	"path/filepath"
)

// repoRoot 返回仓根；解析不到返回空串（调用方决定报 BLOCKED 而不是猜一个）。
func repoRoot() string {
	if v := os.Getenv("ZERG_REPO"); v != "" {
		return v
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			if r := findRoot(filepath.Dir(resolved)); r != "" {
				return r
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if r := findRoot(wd); r != "" {
			return r
		}
	}
	return ""
}

func findRoot(dir string) string {
	for {
		if isRepoRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// isRepoRoot 判 dir 是不是 **zerg 真仓根**（不是「找得到仓根判据件」就算了）。
//
// 判据**两条都要**成立：
//
//	① 仓根判据件在：`<dir>/core/internal/version/version.go`；
//	② **不是公开树物料化快照** —— 发布链把公开树摊平后 `git init` 出来的产物，它**也**带同一枚
//	   version.go、也**有** `.git`，只按 ① 判会把它当仓根（静默换源；缺口 `GAP-20261006-08`：
//	   `ZERG_REPO` 指快照 ⇒ `repo status` 收为仓根 · `doctor` 报「仓库根 PASS」）。
//
// ② 的认法（真源现读定案 · `scripts/build/publish-public.sh` §2/§6「套用发布专属文件」段）：
// 快照根上那几件**只由发布链种下、真仓根没有** —— 出现**任一** ⇒ 判快照：
//
//	· `install.sh`                ← 来自 `publish/install.sh`
//	· `.gitleaks.toml`            ← 来自 `publish/gitleaks.toml`
//	· `gateway/fleet.example.yaml` ← 由私有的 `gateway/fleet.yaml` **改名**而来
//
// ★ 只认**根**相对路径（真仓里的 `publish/install.sh` 一类不受影响）。
func isMaterializedPublicTree_r42(dir string) bool {
	for _, rel := range []string{
		"install.sh",
		".gitleaks.toml",
		filepath.Join("gateway", "fleet.example.yaml"),
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			return true
		}
	}
	return false
}

func isRepoRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "core", "internal", "version", "version.go")); err != nil {
		return false
	}
	return !isMaterializedPublicTree_r42(dir)
}
