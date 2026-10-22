package main

// 命令表 · metrics 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_metrics() []*command {
	return []*command{
		// ---- 度量与排序面（组1 序12 · `承接自-v2.5.11/承接-度量与排序面-20260921.md:40-42` · 2026-09-24）----
		// 与 `impact`（变更影响面）**配对用、不合并成一条**：前者回答「改这一处会牵动谁」（别改坏），
		// 本命令回答「**该改哪**」（把四类读数归一成一个可比排序）。只读 ⇒ 不写 `danger`
		// （走默认「只读」幂等档）、不改 `emitEnvelope`、不落审计、不写缓存。
		{
			path:     []string{"metrics"},
			kind:     "Metrics",
			summary:  "「该改哪」那把尺：把四类读数（重复 / 未用符号 / 覆盖率 / 门禁）**归一成一个可比排序**（只读 · 空输入 ⇒ 不给结论）",
			usage:    "zerg metrics [--input <读数档>] [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数：读数档走 `--input`）"},
			fields:   metricsFields,
			endpoint: "",
			run:      cmdMetrics,
		},
	}
}
