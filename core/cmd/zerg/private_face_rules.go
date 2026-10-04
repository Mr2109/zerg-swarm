// private_face_rules.go —— 私有面清单（`publish/private-paths.txt`）的**唯一** Go 解析件
// （缺口 `GAP-20260927-414` · 命令面）。
//
// 病：同一份清单**语法**此前在 Go 侧被**逐条镜像了两遍** ——
//
//	· `family_publish_set.go` 的 `publishSetPrivateFaceRules` / `publishSetPrivateAllowed`
//	· `check_public_tree_private.go` 的 `cptpLoadRules` / `cptpIsAllowed`
//
// ⇒ 真源（`scripts/gates/check-public-tree-private.py:47 load_rules` + `:64 is_allowed` ·
// 发布链 `publish/mirror-public-lib.py:817/:824/:862-871`）一改，两处各漂各的（双写风险）。
// 本件把**行口径 + 豁免口径**抽成**一处**：两个消费者只留薄壳（名字/签名一字不动 ⇒ 调用点与既有
// 判据件不受影响）。
//
// 口径（逐字照 py `load_rules` / `is_allowed`）：逐行 strip（**strip 函数由调用方给** ——
// py `str.strip()` 与 Go `strings.TrimSpace` 对 Unicode 空白的覆盖面**不同**，故不替调用方选，
// 各消费者把**自己原来那枚**传进来 ⇒ 本件不改任何一方的既有语义）；空行 / `#` 开头跳过；
// `!` 前缀 = 豁免（去 `!` 后**再 strip**）；行尾 `/` = 目录；其余 = 单件（精确匹配）。
package main

import "strings"

// parsePrivateFaceRules —— 私有面清单的**唯一**行口径（`strip` 由调用方给：见件头）。
// 返回三档：目录（尾 `/`）/ 单件 / `!` 豁免。三档都是**非 nil 空切片**（与既有两处的可观察行为一致）。
func parsePrivateFaceRules(rawLines []string, strip func(string) string) (dirs, files, allowed []string) {
	dirs, files, allowed = []string{}, []string{}, []string{}
	for _, raw := range rawLines {
		line := strip(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "!"):
			allowed = append(allowed, strip(line[1:]))
		case strings.HasSuffix(line, "/"):
			dirs = append(dirs, line)
		default:
			files = append(files, line)
		}
	}
	return dirs, files, allowed
}

// privateFaceAllowed —— 私有面清单的**唯一**豁免口径（py `is_allowed` 逐字）：
// `rel == a` 或（`a` 以 `/` 结尾且 `rel` 以 `a` 开头）。
func privateFaceAllowed(rel string, allowed []string) bool {
	for _, a := range allowed {
		if rel == a || (strings.HasSuffix(a, "/") && strings.HasPrefix(rel, a)) {
			return true
		}
	}
	return false
}
