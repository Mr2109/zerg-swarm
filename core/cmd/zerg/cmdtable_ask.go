package main

// 命令表 · ask 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_ask() []*command {
	return []*command{
		// ---- §十八.3 融合四件（批 E · T-59）----
		// 依赖次序（**不许倒**）：能力路由（T-41 的 id 规范化真源）→ `ask` → 意图 schema → `plan`/`apply`。
		{
			path:     []string{"ask"},
			kind:     "Ask",
			summary:  "问一次推理、**不落任务队列**（`task submit` 的对偶）· 能力筛是硬筛",
			usage:    "zerg ask <提示> [--capability 名]… [--prefer 名]… [--model 名] [--node 名]… [--min-ctx n] [--min-mem-gb n] [--dry-run] [--timeout 时长] [--json <字段>]",
			arity:    "any",
			args:     []string{"提示（一句话）"},
			fields:   askFields,
			endpoint: "GET /api/fleet/models · GET /api/models/registry",
			run:      cmdAsk,
		},
	}
}
