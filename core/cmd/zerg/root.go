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
// `doctor` 把所指位置报成「仓库根 **PASS**」—— 毒化面在**报绿**这一侧。
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

func isRepoRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "core", "internal", "version", "version.go"))
	return err == nil
}
