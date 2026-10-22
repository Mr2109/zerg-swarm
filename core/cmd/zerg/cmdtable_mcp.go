package main

// 命令表 · mcp 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_mcp() []*command {
	return []*command{
		// ---- L1 批3（2026-10-08）：`zerg mcp` 真动作分派（★ 真 Go 化 · 非壳搬运 · 占位 `mcp ls` 下架）----
		// 依据：`mcp_zerg_server-真Go化-先期设计-20261008.md` §五 步3（命令树接线）· §三（真 Go 化路径）。
		// 真实现 = `core/cmd/zerg/mcp_serve.go`（8 工具真 Go 化 · 默认 stdio · 复用仓内
		// `github.com/mark3labs/mcp-go v0.58.0` 的 `server` 子包 —— ✗ 不引新第三方模块 · ✗ 手写 JSON-RPC）；
		// 承载宿主 = `ui_start.go`（门78 面④ 要求新增 path 的实现件**已在** `scripts/公开标记.tsv` ⇒
		// 复用**已登记**件作宿主 · 本单铁律禁碰该表 —— 同 `cmdSandboxProbe` / `cmdWallEvidence`）。
		// ★ 口径（照设计 §3.6）：stdio 下 stdout 是 **MCP 协议通道** ⇒ 本命令**没有** `--json` 信封面
		// （前置拒收 rc=2）；退码只表「server 本身能不能起」：0 干净退出 · 2 用法错 · 8 名册读不到 · 1 收尾失败。
		// ★ host 取值照 **GAP-73 拍定 B**：`fleet_topology` 的机器地址与 `machine` 合法集**从名册取**
		// （`gateway/fleet.yaml` 的 `fleet:` 段 —— 自陈是 `~/.zerg/contexts/default.yaml` 的 `nodes:` 之
		// **同形投影**）· ✗ **不碰** `core/internal/store`（主控心跳契约）。
		{
			path:    []string{"mcp", "serve"},
			kind:    "McpServe",
			layer:   "host",
			summary: "虫族控制面 MCP server（默认 stdio · 8 工具：zerg_help/fleet_status/list_models/get_model_info/fleet_topology/load_model/unload_model/switch_model）—— 真 Go 化（复用仓内 mark3labs/mcp-go · stdout 是 MCP 协议通道 ⇒ 无 --json）",
			usage:   "zerg mcp serve",
			run:     cmdMCPServeFamily,
		},
	}
}
