package main

// 命令表 · trace 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_trace() []*command {
	return []*command{
		// ---- CA 全量追踪（批 B1 · 2026-10-20）：独立入口 `core/cmd/zerg-trace` 归入命令面 ----
		// 旗标**逐字**保留源件那五枚（`--live` / `--json` / `--step` / `--tokens` / `--errors`），
		// `<logdir>` 必填。**双轨期**：`core/cmd/zerg-trace` 二进制不动（本族只加一条入口）。
		// 只读（`os.Open` / `os.Stat` 两件）⇒ 不写 `danger`（走默认「只读」幂等档）。
		{
			path:     []string{"trace"},
			kind:     "Trace",
			summary:  "CA 全量追踪（`events.jsonl` 时间线 · 每轮明细 · token 分析 · 错误清单 · 机器指标联动；只读）",
			usage:    "zerg trace <logdir> [--live] [--json] [--step N] [--tokens] [--errors]",
			arity:    "any",
			args:     []string{"<logdir>（必填：含 `events.jsonl` 的日志目录；`sysmetrics.jsonl` 有则联动）"},
			fields:   traceFields,
			endpoint: "",
			run:      cmdTrace,
		},
	}
}
