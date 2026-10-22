package main

// 命令表 · eval 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_eval() []*command {
	return []*command{
		// ---- T-56 余项 · 评测族 `eval`（`scripts/evals/` 22 件 ⇒ ①5 收编 / ②4 内部 / ④13 待拍 · DEV-0010）----
		{
			path:     []string{"eval", "ls"},
			kind:     "Eval",
			summary:  "评测线 22 件的逐件归属（① 收编 5 · ② 保留内部 4 · ④ 维持待拍 13 —— 现算不手写）",
			usage:    "zerg eval ls [--json <字段>]",
			fields:   evalLsFields,
			endpoint: "",
			run:      cmdEvalLs,
		},
		{
			path:     []string{"eval", "show"},
			kind:     "Eval",
			summary:  "单件评测脚本的现状（归属 · 一句理由 · 执行面）",
			usage:    "zerg eval show <名> [--json <字段>]",
			arity:    "any",
			args:     []string{"件名（与 `zerg eval ls` 逐字相同）"},
			fields:   evalShowFields,
			endpoint: "",
			run:      cmdEvalShow,
		},
		{
			path:    []string{"eval", "run"},
			kind:    "Eval",
			summary: "跑一支评测脚本（D2 三态 · **只对 ① 收编的件开放** · 缺 --yes ⇒ 2 · 真调旧脚本、退码原样转出）",
			usage:   "zerg eval run <名> [位置参数…] [--json <字段>] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"件名（与 `zerg eval ls` 逐字相同）", "位置参数（原样交给脚本）"},
			fields:  evalRunFields,
			danger: &dangerSpec{dangerD2, "件名",
				"真跑一件评测脚本（可能要跑着的生产面 · 占机器与模型槽）—— 命令面只转发退码，脚本本体一个字不改",
				"T-56 余项 · DEV-0010 · 归属-收编与退役-20260920 §④ 评测线 22 件", false, dangerScopeUnknown},
			// `opened`: 真跑已开放（`--yes` 就真跑脚本；退码原样透传）。
			opened: true,
			run:    cmdEvalRun,
		},
		{
			path:     []string{"eval", "run-all"},
			kind:     "EvalRunAll",
			summary:  "评测批跑总入口（三层：层1 `go test ./internal/agent/` · 层2/3 已归档载荷真跑 · **真搬**：批跑逻辑/落档/退码在 Go · ✗ 不再 exec `scripts/build/run_all.sh`）",
			usage:    "zerg eval run-all [<模型> [版本标签]]",
			arity:    "any",
			args:     []string{"模型（缺省 example-35b）", "版本标签（缺省 = 当前时刻 yyyymmdd-HHMM）"},
			endpoint: "",
			run:      cmdEvalRunAllFamily,
		},
	}
}
