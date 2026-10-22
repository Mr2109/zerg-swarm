package main

// 命令表 · apply 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_apply() []*command {
	return []*command{
		{
			path:     []string{"apply"},
			kind:     "Apply",
			summary:  "**做**：只吃那一份意图件（L1 schema → L2 引用 → L3 干跑 → L4 人在环）· **校验四层已开放**；写面（真做）本版未开放 · 拒执退码 2",
			usage:    "zerg apply <件> [--confirm=<目标>] [--json <字段>]",
			arity:    "any",
			args:     []string{"意图件路径"},
			fields:   applyFields,
			endpoint: "",
			refuses:  true, // 真跑拒执（`cmdApply` 校验四层已开放、**写面（真做）本版未开放** ⇒ 真做退 2）
			run:      cmdApply,
		},
	}
}
