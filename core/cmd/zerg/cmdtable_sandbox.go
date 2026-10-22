package main

// 命令表 · sandbox 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_sandbox() []*command {
	return []*command{
		// ---- L1 骨架（2026-10-08）：新族 `sandbox` / `wall` / `fleet` / `mcp` —— 只立族、不填业务 ----
		// 依据：`~/.zerg/state/AlltoGo接线总图-20261008.md` §②「立族清单 = 新立 4 族」·
		// `待拍清单-AlltoGo-20261008.md` P2（方案 A 立新族）/ P3（`zerg wall` 撞名 ⇒ 帮助主题改名腾出 `wall`）
		// / P4（`setup-zerg.sh` 归 `zerg svc setup` —— 另一族 · 不在本单四族内）。
		// 口径（本单只立骨架）：每族 **≥1 条只报告/占位动作**，占位动作**一律 fail-closed** ——
		// 未实现 ⇒ 退码 `2` + 逐字报「未接」（✗ 绝不静默返回 0）；实现体 = `ui_start.go` 的
		// `cmdFamilyNotWired`（复用一件**已登记**件作宿主 ⇒ 门78 面④ 齐 · 见该函数头）。X1 层级标记
		// = `host`（命令面恒在茧壁外 · 门⑥ `check-wall-layer` 闭集）。各路接线时把 `run:` 指向各自
		// `family_<族>.go` 真实现、并把 `refuses` 随实情撤/留 —— 本条即该族的第一条动作。
		// ---- L1 批1（2026-10-08）：`zerg sandbox` 真动作分派（壳转账 · 只转发、不翻译）----
		// 依据：`~/.zerg/state/沙登茧壁组-接线预扫-20261008.tsv` 批1（沙箱探针 ＋ 两态门禁）。
		// 底层唯一实现 = `scripts/sandbox-probes/*.py|*.sh`（真跑子进程 / 沙箱语义 ⇒ 非纯 Go 可化）；
		// 命令面只做入口 ＋ 转发（✗ 不重写脚本任何一段）。未接的夹具动作（`sandbox test`）仍 fail-closed
		// 退 2（批3）。实现体宿主 = `ui_start.go`（门78 面④ 要求实现件已在 `scripts/公开标记.tsv` ⇒ 复用
		// 已登记件作宿主 · 本单铁律禁碰该表 —— 同 `cmdFamilyNotWired` 那条理由）。
		{
			path:        []string{"sandbox", "probe"},
			layer:       "host",
			summary:     "虫卵探针（`conn_jit` / `jitprobe` / `lx2-linux` / `miniprobe` · 底层只调 scripts/sandbox-probes/<名>.py —— 命令面不重写逻辑）",
			usage:       "zerg sandbox probe <conn_jit|jitprobe|lx2-linux|miniprobe> [<透传旗标…>]",
			arity:       "any",
			args:        []string{"探针名（逐字 · 见用法串的闭集）", "旗标与退码逐字来自 scripts/sandbox-probes/<名>.py（原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSandboxProbe,
		},
		{
			path:        []string{"sandbox", "verify"},
			layer:       "host",
			summary:     "两态可验门禁（判据7 · 断言A空间内通信可用 ＋ 断言B出网被拦 · 底层只调 scripts/sandbox-probes/verify-two-states.py —— 命令面不重写逻辑）",
			usage:       "zerg sandbox verify [--wall <茧壁二进制> | --self-test]",
			arity:       "any",
			args:        []string{"旗标与退码逐字来自 scripts/sandbox-probes/verify-two-states.py（原样透传 · 含 --wall / --self-test）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSandboxVerify,
		},
		{
			path:        []string{"sandbox", "negctl"},
			layer:       "host",
			summary:     "两态门禁的负例活体控制（四条必须 rc=2 · 底层只调 scripts/sandbox-probes/negctl-two-states.sh —— 命令面不重写逻辑）",
			usage:       "zerg sandbox negctl",
			arity:       "any",
			args:        []string{"旗标与退码逐字来自 scripts/sandbox-probes/negctl-two-states.sh（原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdSandboxNegctl,
		},
		{
			path:        []string{"sandbox", "test"},
			layer:       "host",
			summary:     "虫卵验收夹具（子动作分派：`relay`/`fleet`/`setup`/`update` 四子动作**均已接线** —— 各自底层只调 scripts/sandbox/test-<名>-sandbox.sh，命令面只做入口＋转发、不重写逻辑）",
			usage:       "zerg sandbox test <relay|fleet|setup|update>",
			arity:       "any",
			passthrough: true,
			run:         cmdSandboxTest,
		},
	}
}
