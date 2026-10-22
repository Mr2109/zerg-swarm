package main

// 命令表 · net 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_net() []*command {
	return []*command{
		{
			path: []string{"net", "probe"},
			kind: "NetProbe",
			// ★ 字段表必须写成 `[]string{…}` **字面量**（不许抽成 `xxxFields` 变量）：
			//   `scripts/gates/check-cli-contract.py` 的 `FIELDS_RE` 只认这一种形状
			//   （`fields:\s*\[\]string\{…\}`）—— 抽成变量会让它读成「这条没有字段表」，
			//   于是重冻时派生的候选格变成「说明面不认 --json」，那句话与本命令**当场相反**。
			//   先例：`port ls` 也是字面量（同一条纪律，只是此前没人写下理由）。
			summary:  "环境面**只读**投影（代理在哪 / 走不走得通 / 直连还是代理 · 取不到 ⇒ 8）",
			usage:    "zerg net probe [--json <字段>]",
			fields:   []string{"proxy", "reachable", "mode"},
			endpoint: "",
			run:      cmdNetProbe,
		},
	}
}
