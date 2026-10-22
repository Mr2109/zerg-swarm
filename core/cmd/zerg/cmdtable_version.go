package main

// 命令表 · version 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_version() []*command {
	return []*command{
		{
			path:    []string{"version"},
			kind:    "Version",
			summary: "单行身份（组件 版本 代码 sha 构建时间）· --json 报三层版本",
			usage:   "zerg version [--json <字段>] [--build-info]",
			fields:  []string{"name", "version", "commit", "build_time", "contract", "object_schema", "core_version", "core_code_sha", "window"},
			run:     cmdVersion,
		},
	}
}
