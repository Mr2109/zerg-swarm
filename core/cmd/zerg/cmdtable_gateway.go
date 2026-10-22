package main

// 命令表 · gateway 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_gateway() []*command {
	return []*command{
		{
			path:     []string{"gateway", "models"},
			kind:     "Model",
			summary:  "网关侧模型面（与 `model ls` **同源** —— 网关没有第二份模型表）",
			usage:    "zerg gateway models [--json <字段>]",
			fields:   []string{"id", "host", "backend", "modality", "mem_gb", "file"},
			endpoint: "GET /api/fleet/models",
			run:      cmdGatewayModels,
		},
		{
			path:    []string{"gateway", "breakers"},
			danger:  &dangerSpec{dangerD2, "（网关开关）", "网关断路开关（`--reset` 重置熔断 ⇒ 重新放流量 · `POST /api/gateway/breakers/reset`）", "GAP-20261005-38 · 同族先例 core reload（D2）· §三 C 族", false, dangerScopeRemote},
			opened:  true,
			kind:    "GatewayBreakers",
			summary: "网关断路开关（D2 写面 · `--reset` 要 `--yes`）",
			usage:   "zerg gateway breakers [--reset] [--dry-run | --yes]",
			run:     cmdHazardWrite,
		},
	}
}
