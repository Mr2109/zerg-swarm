package main

// 命令表 · find 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_find() []*command {
	return []*command{
		// ---- 缺口面 P0（缺口-命令面-20260921 §十一 · 2026-09-21）：今天手搓最多的一类先补上 ----
		// ── 缺口面 · `zerg find`（按名找件 · 2026-09-25 · 手搓 find 的替身）────────────
		{
			path:     []string{"find"},
			kind:     "Find",
			summary:  "按名字片段在仓内找件（受控遍历 · 输出相对路径/大小/修改时间）",
			usage:    "zerg find <名字片段> [--type file|dir] [--root <根>] [--limit N] [--json <字段>]",
			arity:    "any",
			args:     []string{"名字片段"},
			fields:   []string{"relpath", "size", "mtime"},
			endpoint: "",
			run:      cmdFind,
			// ★ 缺口 `GAP-20260927-14`（2026-10-14 · 真修 ①）：`total` 走**可选字段层**（`optFields`，
			//  与 `code find` 的 `count` 同一体例：登在出口白名单、`--json total` 点得出来，但**不进**
			//  上面那份 `fields:` 冻结表 ⇒ 帮助面 / 命令页一行不动、`合法字段:` 那条错误面也一字不变）。
			//  逐行落一格 `total` = 本跑**真命中件数**（跑级真值 · 与包封 `meta.total` 同源同值）。
			optFields: []string{"total"},
		},
	}
}
