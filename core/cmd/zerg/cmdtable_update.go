package main

// 命令表 · update 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_update() []*command {
	return []*command{
		{
			path:    []string{"update"},
			machine: machineHostName,
			// ★ Q-129：`--check` 是**只读**档（不下载、不构建、不换件 ⇒ 不入危险档），与换件那条
			//   D3 路径**同一条命令两档**（先例：`dev proposal` 整条登 D2、`new` 才 fail-closed）。
			summary: "源码式自更新（危险 D3 · 本版未开放）；`--check` 只读：查有无新版（不下载不换件）",
			usage:   "zerg update [--check | --node <机> --confirm=<主机名> --yes [--dry-run]] [--json <字段>]",
			arity:   "any",
			args:    []string{"主机名"},
			fields:  updateCheckFields,
			danger:  &dangerSpec{dangerD3, "主机名", "按真源走一次自更新（校验 + 换件 + 回执；走 F-3 例外清单）", "§7.1 P12 · §九 M20 F-3 · 开工单 T-52", true, dangerScopeSystem},
			run:     cmdUpdateOrGuarded,
		},
	}
}
