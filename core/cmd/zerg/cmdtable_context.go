package main

// 命令表 · context 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_context() []*command {
	return []*command{
		{
			path:    []string{"context", "ls"},
			kind:    "Context",
			summary: "档位名册（离线也出表）",
			// ★ 2026-09-24（缺口 `Q-156`）：用法串补上**真旗标** `--resume`。
			//   病灶（修前现读实据）：`--resume` 解析器认（`parseInvocation` 的 `--resume` 那一格）
			//   且该命令**真读**（`readonly.go` 的 `cmdContextLs` → `cmdContextResume`），
			//   用法串却只写 `[--json <字段>]` ⇒ 公开文档面照用法串引 `zerg context ls --resume`
			//   时被判「旗标不在用法串里」（闸⑧ 的 `nonexistent-parameter-documented` 两处逐条在案）。
			//   同一枚旗标的先例写法：`readonly.go` 的续做面登记（§十二 `P-120` 定案 ①）。flag 名逐字不改。
			usage:    "zerg context ls [--resume] [--json <字段>]",
			fields:   []string{"name", "core", "gateway", "default_node", "token_source"},
			endpoint: "",
			run:      cmdContextLs,
		},
		// ---- P1 · 上下文压缩的可回查面（设计稿 v5 §6）----
		// ★ 只读呈现与取回；压缩决策在对话/任务侧的引擎里，不在命令面。
		// ★ `compact` 暂不提供：CLI 侧没有「压缩目标」⇒ 硬造即假命令，挂账等有目标那天。
		{
			path:     []string{"context", "stats"},
			kind:     "ContextStats",
			summary:  "只读：卸载面与审计面的读数（件数 / 字节 / 事件数 · `--json` 闭集四字段）",
			usage:    "zerg context stats [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数）"},
			fields:   []string{"dir", "count", "bytes", "events"},
			endpoint: "",
			run:      cmdContext,
		},
		{
			path:     []string{"context", "show"},
			kind:     "ContextShow",
			summary:  "只读：按 id 取回**卸载的原文**（逐字节 · `zerg context stats` 里的落点目录）",
			usage:    "zerg context show <sha16>",
			arity:    "any",
			args:     []string{"id（单段文件名；不在盘 ⇒ 2 并列现有几个）"},
			endpoint: "",
			run:      cmdContext,
		},

		// ☆ 波3-A 命令面真修（缺口 `GAP-20260925-32`）：机器名册 `nodes:` 的第一条改面。
		//   定档理由（`family_config.go:cmdContextSetPort` 现读）：只改 `~/.zerg/contexts/<档>.yaml`
		//   里**现成的** `port:` 一行（临时件 + rename 原子写 · 写完读回逐字节再校 · 任一步不过 ⇒ 回滚）；
		//   可逆（把端口改回即原状）⇒ **D2**（`--yes` 即可），不是 D3。
		//   三态面：`--dry-run` ⇒ 计划件走 stdout + rc=0 且一个字节都不落；缺 `--confirm`/`--yes` ⇒ 2（fail-closed）。
		{
			path:     []string{"context", "set-port"},
			machine:  machineNodeName,
			kind:     "ContextSetPort",
			summary:  "改机器名册（`~/.zerg/contexts/<档>.yaml` 的 `nodes:`）里某台机**现成的** `port:`（先校验后写 · `--dry-run` 先行 · 真写要 `--confirm=<机器名>` + `--yes` · 写完读回再校 · 任一步不过 ⇒ 回滚）—— host/port/os/kind 的唯一写入口",
			usage:    "zerg context set-port --node <机器名> --port <端口> [--context <档名>] --confirm=<机器名> [--dry-run | --yes] [--json <字段>]",
			arity:    "any",
			args:     []string{"机器名（--node：`nodes:` 段的一级键）", "端口（--port：1..65535）"},
			fields:   []string{"node", "port", "context", "line", "changed"},
			danger:   &dangerSpec{dangerD2, "机器名", "改机器名册 `~/.zerg/contexts/<档>.yaml` 里该节点现成的 `port:`（同目录临时件 + rename 原子写 · 写完读回再校 · 任一步不过 ⇒ 回滚；不碰主控运行态）；可逆：把端口改回即原状", "缺口账 GAP-20260925-32 · 同族写面先例 zerg model add（D2）· §九 M3 C1", true, dangerScopeOutRepo},
			opened:   true,
			endpoint: "",
			run:      cmdContextSetPort,
		},
	}
}
