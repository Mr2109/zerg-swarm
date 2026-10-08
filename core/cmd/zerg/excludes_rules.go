// excludes_rules.go —— 公开面排除表（`publish/excludes.txt`）的**唯一** Go 解析件
// （缺口 `GAP-20260926-223` 账[3] · 迁表步；照 `private_face_rules.go` 先例 `GAP-20260927-414`）。
//
// 病：同一份排除清单**语法**此前在 Go 侧被**逐条镜像了 6+ 遍**（各异构解析器）——
//
//	· `check_history_secrets.go` 的 `chsecExcludesRe` / `chsecLoadExcludes`
//	· `check_marks_coverage_parity.go` 的 `mkcp` 侧甲 `strings.Index` 解析
//	· `check_newfile_registry.go` 的 `gnrExcludesRE` / `gnrLoadExcludes`
//	· `check_newcmd_followup.go` 的 `cnfExclA1RE` / `cnfExclA2RE`
//	· `check_registry_vs_disk.go` 的 `crsLoadExcludes`
//	· `family_publish_set.go` 的 `publishSetQuotedEntries`
//
// ⇒ 真源（原 `scripts/build/publish-public.sh` 的 `EXCLUDES=( … )`）一改，各处置器各漂各的
// （双写风险 —— 与 `publish/private-paths.txt` 曾被镜像两遍同族）。
// 本件把**行口径 + 命中口径**抽成**一处**；读者只留薄壳（名字/签名不动 ⇒ 调用点与既有判据件不受影响）。
//
// 口径（逐字照现读读者）：
//   - 逐行 strip（**strip 函数由调用方给** —— py `str.strip()` 与 Go `strings.TrimSpace`
//     对 Unicode 空白的覆盖面**不同**，故不替调用方选：各消费者把自己原来那枚传进来）；
//   - 空行 / `#` 开头整行跳过；
//   - 命中：`rel == e` 或 `strings.HasPrefix(rel, e 去尾 "/" + "/")`。
//     ★ 目录条**不带尾 `/`**（本真源现读式；与 `private_face_rules.go` 的尾 `/` 目录口径**相反**）。
package main

import (
	"os"
	"strings"
)

// excludesRuleStrip —— 本件自用的 strip（现读读者皆用 Go 侧 `strings.TrimSpace`）。
func excludesRuleStrip(s string) string { return strings.TrimSpace(s) }

// parseExcludesRules —— 排除表的**唯一**行口径（`strip` 由调用方给：见件头）。
// 返回**非 nil 空切片**（与现读读者的可观察行为一致）。
func parseExcludesRules(rawLines []string, strip func(string) string) []string {
	entries := []string{}
	for _, raw := range rawLines {
		line := strip(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	return entries
}

// excludesRulesMatch —— 排除表的**唯一**命中口径（逐字照 `publish/mirror-public-lib.py`
// 的 `map_path` 与 Go 侧现读读者）：`rel == e` 或 `rel` 以「`e` 去尾 `/` 后的前缀 `+/`」开头。
func excludesRulesMatch(rel string, entries []string) bool {
	for _, e := range entries {
		if rel == e || strings.HasPrefix(rel, strings.TrimSuffix(e, "/")+"/") {
			return true
		}
	}
	return false
}

// loadExcludesTable —— 读行式真源 `publish/excludes.txt` ⇒ 条目切片。
// 缺件 / 读不到 ⇒ 返回空切片 + err（**不给结论** · 调用方 fail-closed；✗ 不静默当「零排除」）。
func loadExcludesTable(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return []string{}, err
	}
	return parseExcludesRules(strings.Split(string(b), "\n"), excludesRuleStrip), nil
}
