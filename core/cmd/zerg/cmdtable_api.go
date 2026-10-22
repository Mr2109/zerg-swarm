package main

// 命令表 · api 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_api() []*command {
	return []*command{
		{
			path:     []string{"api", "ls"},
			kind:     "Capability",
			summary:  "HTTP 能力面（投影 /api/capabilities）",
			usage:    "zerg api ls [--json <字段>]",
			fields:   []string{"name", "endpoint", "desc", "example"},
			endpoint: "GET /api/capabilities",
			run:      cmdAPILs,
		},
		{
			path:     []string{"api", "openapi"},
			kind:     "ApiPath",
			summary:  "HTTP 路径表（投影 /api/openapi.json 的 paths）",
			usage:    "zerg api openapi [--json <字段>]",
			fields:   []string{"path", "method", "summary"},
			endpoint: "GET /api/openapi.json",
			run:      cmdAPIOpenAPI,
		},
		{
			path:     []string{"api", "help"},
			summary:  "api 族说明（只读面 · 逃生门 `api call` 暂不开）",
			usage:    "zerg api help",
			endpoint: "",
			run:      cmdAPIHelp,
		},
		// ---- 批 B4（2026-10-20 · `zerg api serve`）：独立入口 `core/cmd/zerg-api` 归入命令面 ----
		// 源件 227 行 · 旗标三枚**逐字**保留（`-port 8083` / `-workdir .` / `-insecure`），只把 Go
		// `flag` 的单横线写成双横线。★ 并入**既有** `api` 族（`ls`/`openapi`/`help` 三条只读投影 =
		// 打主控 `/api/*` 的**客户端**面），✗ 不新开顶层族；动作名分家：`serve` = **在本机起服务端**。
		// **双轨期**：`core/cmd/zerg-api` 二进制一字不动。实现在 `family_api_serve.go`
		// （件头逐条写清与源件的五处差异 —— 用法退码 1→2、`--port` 值域前置判、工作目录启动时定死、
		// 不切进程 cwd、优雅停）。
		// 写面（`POST /task` 真派单 · 改任务单目录）但**不进危险表**，理由与 `sched run` 那一格同：
		// 它是操作者当场在本机起的那个常驻进程（本来由 launchd 拉起），Ctrl-C/SIGTERM 即停；
		// 远端可达面**由鉴权闸守着**（缺 `ZERG_API_TOKEN` 即拒启，无令牌必须显式 `--insecure` 并打横幅）
		// ⇒ 本机可逆、无远端副作用；与 `trace`/`review` 同一种「不进危险表」的判法。
		{
			path:     []string{"api", "serve"},
			kind:     "ApiServe",
			summary:  "在本机起外部接入 REST API（**常驻档**：`POST /task` 提交任务进 git 自循环 · `GET /task/{id}` 查 · `GET /capabilities` 机器可读清单 · `GET /status` 统计 · `GET /panel` 面板；缺 `ZERG_API_TOKEN` 即拒启、`--insecure` 放行并打横幅）",
			usage:    "zerg api serve [--port <端口>] [--workdir <目录>] [--insecure]",
			arity:    "none",
			args:     []string{"（不收位置参数：端口走 `--port`、工作目录走 `--workdir`）"},
			endpoint: "",
			opened:   true,
			run:      cmdAPIServe,
		},
	}
}
