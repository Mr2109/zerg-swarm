package main

import "path/filepath"

// mpRealPath —— 与壳 `realpath` 同口径（macOS `/var` → `/private/var`）。
//
// 为什么必须解析：`mirror-cn` 壳印的是 `realpath` 过的路径，Go 半若印未解析的
// ⇒ 两态对拍（`zz_mirror_cn_parity_test.go`）在 P2/P3 逐行不同。解析失败（件不在盘）
// 就原样返回 —— 不因为一个只读展示把整条命令拦死。
func mpRealPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil && r != "" {
		return r
	}
	return p
}
