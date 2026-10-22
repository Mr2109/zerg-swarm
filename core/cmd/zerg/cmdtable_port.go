package main

// 命令表 · port 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_port() []*command {
	return []*command{
		{
			path:     []string{"port", "ls"},
			kind:     "PortLs",
			summary:  "看某个端口被谁占着（含 pid/ppid/inode/在跑件路径 · 手敲 lsof 的替身）",
			usage:    "zerg port ls [<端口>] [--json <字段>]",
			arity:    "any",
			args:     []string{"端口（可省：缺省列声明面三个端口）"},
			fields:   []string{"port", "pid", "ppid", "process", "sock", "path"},
			endpoint: "",
			run:      cmdPortLs,
		},
	}
}
