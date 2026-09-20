// root.go —— 仓根解析（供门禁直通与帮助导出用）。
//
// 红线（scripts/gates/check-hardcoded-private-paths.py）：代码里**不许写死私有绝对路径**
// （`<volume-path>` / `~…`）—— 公开快照会把它们替换成占位符，机群上的二进制
// 就指向不存在的目录。故这里只按三条**可推导**的路走：环境变量 → 可执行文件所在目录上溯 →
// 当前目录上溯；判据是「仓根有 core/internal/version/version.go」。
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
