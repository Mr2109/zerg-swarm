package main

// 命令表 · propose 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_propose() []*command {
	return []*command{
		{
			path:     []string{"propose", "ls"},
			kind:     "Proposal",
			summary:  "提案清单（§3.2 propose 那条 · 与 dev proposal list 同一份清单的第二个入口）",
			usage:    "zerg propose ls [--state 未决|已批准|已否决] [--json <字段>]",
			fields:   proposalFields,
			endpoint: "",
			run:      cmdProposeLs,
		},
	}
}
