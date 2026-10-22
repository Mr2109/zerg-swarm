package main

// 命令表 · root 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_root() []*command {
	return []*command{
		{
			path:    []string{"root"},
			kind:    "RepoRoot",
			summary: "印仓根（人面一行）· `--json` 给 {root, via}（via = env / 可执行件上溯 / cwd 上溯）",
			usage:   "zerg root [--json <字段>]",
			fields:  []string{"root", "via"},
			run:     cmdRoot,
		},
	}
}
