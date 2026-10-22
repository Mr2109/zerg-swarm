package main

// 命令表 · calib 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_calib() []*command {
	return []*command{
		// ---- T-56 余项 · 标定族 `calib`（`scripts/calib/` 3 件 ⇒ ① 收编 · DEV-0010 · 2026-09-21）----
		{
			path:     []string{"calib", "ls"},
			kind:     "Calib",
			summary:  "标定线 3 件的声明面（名字 · 件 · 角色 · 归属 —— 逐件现读，不另抄一份）",
			usage:    "zerg calib ls [--json <字段>]",
			fields:   calibLsFields,
			endpoint: "",
			run:      cmdCalibLs,
		},
		{
			path:     []string{"calib", "show"},
			kind:     "Calib",
			summary:  "单件标定脚本的现状（归属 · 角色 · 执行面 · 退码口径）",
			usage:    "zerg calib show <名> [--json <字段>]",
			arity:    "any",
			args:     []string{"件名（与 `zerg calib ls` 逐字相同）"},
			fields:   calibShowFields,
			endpoint: "",
			run:      cmdCalibShow,
		},
		{
			path:    []string{"calib", "run"},
			kind:    "Calib",
			summary: "跑一支标定脚本（D2 三态 · --dry-run 零副作用 · 缺 --yes ⇒ 2 · 真调旧脚本、退码原样转出）",
			usage:   "zerg calib run <名> [位置参数…] [--json <字段>] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"件名（与 `zerg calib ls` 逐字相同）", "位置参数（原样交给脚本）"},
			fields:  calibRunFields,
			danger: &dangerSpec{dangerD2, "件名",
				"真跑一件标定脚本（会占机器/模型槽 · 出的是实测档案）—— 命令面只转发退码，脚本本体一个字不改",
				"T-56 余项 · DEV-0010 · 归属-收编与退役-20260920 §④ 标定线 3 件", false, dangerScopeUnknown},
			// `opened`: 真跑已开放（`--yes` 就真跑脚本；退码原样透传）。
			opened: true,
			run:    cmdCalibRun,
		},
	}
}
