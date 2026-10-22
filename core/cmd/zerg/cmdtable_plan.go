package main

// 命令表 · plan 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_plan() []*command {
	return []*command{
		{
			path:    []string{"plan"},
			kind:    "Plan",
			summary: "**算**：产出一份意图件（M6 包封 · F1–F7 + 四附加件）· 写意图件要 `--yes`（默认档 fail-closed）",
			usage:   "zerg plan <族> <动作> <对象…> [--node 名]… [--expect 旧值] [--target-ref <编号>] [--out <件>] [--dry-run | --yes] [--json <字段>]",
			arity:   "any",
			args:    []string{"族（= target.kind）", "动作", "对象名"},
			fields:  planFields,
			// 授权侧（`GAP-20260927-140` ①）：真写意图件（`--out <件>` 或 `$ZERG_PLAN_DIR`）而修前无
			// `danger` ⇒ 默认档零授权写。fail-closed 落在写盘点（见 `cmdPlan`）。
			danger:   &dangerSpec{dangerD2, "（意图件落点）", "写一份意图件（`--out <件>` 或 `$ZERG_PLAN_DIR` 缺省 ~/.zerg/plans/<plan_id>.json）", "GAP-20260927-140 ① · GAP-20260927-90 · 同族先例 doc meta fill（D2）", false, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdPlan,
		},
	}
}
