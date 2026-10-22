package main

// 命令表 · cocoon 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_cocoon() []*command {
	return []*command{
		// ---- 虫茧沙箱 · 生命周期五条（C1 · 2026-10-20）------------------------------------
		// 真源 = `cocoon/meta.json`（`core/internal/cocoon/meta.go`）；后端本版只接 FakeBackend。
		// ★ 与下面 `cocoon pack|info|verify|xfer`（卵的单文件封装 · A12 归名）**不是一件事**。
		{
			path:     []string{"cocoon", "ls"},
			kind:     "Cocoon",
			summary:  "虫茧清单（只读 meta 真源：id / phase / 端口 / 绑定的卵 · 目录里有而 meta 没有的孤儿不进清单）",
			usage:    "zerg cocoon ls [--json <字段>]",
			fields:   cocoonLsFields,
			endpoint: "本机状态根 <ZERG_COCOON_ROOT|cocoons>/<id>/meta.json（不扫目录当真源）",
			run:      cmdCocoonLs,
		},
		{
			path:     []string{"cocoon", "open"},
			kind:     "CocoonOpen",
			summary:  "按 cocoon.toml 起一个茧（分配 id → 写 meta → Provision → 起服务 → 回写 pid/ports；后端由 ZERG_COCOON_BACKEND 选：fake / nsjail / auto）",
			usage:    "zerg cocoon open <dir> [--dry-run] [--json <字段>]",
			args:     []string{"茧目录（含 cocoon.toml）"},
			arity:    "any",
			fields:   cocoonOpenFields,
			endpoint: "本机：<dir>/cocoon.toml + 状态根 <id>/meta.json（后端 ZERG_COCOON_BACKEND=fake|nsjail|auto，署名见 meta.backend）",
			run:      cmdCocoonOpen,
		},
		{
			path:     []string{"cocoon", "close"},
			kind:     "CocoonClose",
			summary:  "停一个茧（先落 stopping 再停 → 写 gone → 留盘）；`--purge` 额外删树（先留墓碑、失败即零删除 · 要 --confirm=<id>）",
			usage:    "zerg cocoon close <id> [--purge --confirm=<id>] [--dry-run] [--json <字段>]",
			args:     []string{"茧 id（照 `cocoon ls` 的 id 那一格）"},
			arity:    "any",
			fields:   cocoonCloseFields,
			endpoint: "本机：状态根 <id>/meta.json + <id>.meta.json（墓碑）",
			run:      cmdCocoonClose,
		},
		{
			path:     []string{"cocoon", "inspect"},
			kind:     "CocoonInspect",
			summary:  "单个茧的现状（只读：meta + 后端能力面 + AI 注入面形状 · 令牌只出脱敏形）",
			usage:    "zerg cocoon inspect <id> [--json <字段>]",
			args:     []string{"茧 id（照 `cocoon ls` 的 id 那一格）"},
			arity:    "any",
			fields:   cocoonInspectFields,
			endpoint: "本机：状态根 <id>/meta.json ＋ 后端 Capabilities（零写盘、连锁件都不建）",
			run:      cmdCocoonInspect,
		},
		{
			path:     []string{"cocoon", "bind"},
			kind:     "CocoonBind",
			summary:  "把茧绑到一颗虫卵（写 meta 的 egg 字段 · 终态的茧不许再绑）",
			usage:    "zerg cocoon bind <id> <卵名> [--json <字段>]",
			args:     []string{"茧 id", "卵名（模型 × 机 的那一枚）"},
			arity:    "any",
			fields:   cocoonBindFields,
			endpoint: "本机：状态根 <id>/meta.json（写 egg 一格）",
			run:      cmdCocoonBind,
		},
		// ---- 虫茧沙箱 · 宿主 broker 的命令面（T4c · 2026-10-21）------------------------------
		// ★ 为什么要有这一条（T4b 实测的两个「开箱即坏」）：
		//   ① broker 缺省 `127.0.0.1:8082` 与 `zerg-core` 网关**撞车**（`gw.Start(8082)`）⇒
		//      本机 bind 直接 EADDRINUSE ⇒ 缺省起不来。T4c 让到 `127.0.0.1:8092`
		//      （`cocoon.DefaultBrokerHTTPAddr`，一处真源）。
		//   ② 此前**没有起 broker 的命令面** ⇒ 茧内拿到的 `ZERG_AI_BASE` 指向一扇没人开的门。
		// ★ 只留**一个旋钮** `ZERG_BROKER_ADDR`（旧名 `ZERG_BROKER_HTTP_ADDR` 当同义回退）：
		//   它同时决定 ① 监听地址 ② 茧内缺省 base 的 host:port —— 两处都走 `cocoon.ResolveBrokerAddr`。
		{
			path:     []string{"cocoon", "broker"},
			kind:     "CocoonBroker",
			summary:  "宿主 broker 的起/收/看（start 后台脱钩起 · 等 /health 真 200 才回 0 · 端口被占点名占用进程；stop 按 pid 收并事后验端口；status 一行 + --json；地址旋钮 ZERG_BROKER_ADDR 同时定监听与茧内 ZERG_AI_BASE）",
			usage:    "zerg cocoon broker <start|stop|status> [--dry-run] [--json <字段>]",
			args:     []string{"动作（start | stop | status）"},
			arity:    "any",
			fields:   cocoonBrokerFields,
			endpoint: "本机回环 <ZERG_BROKER_ADDR|ZERG_BROKER_HTTP_ADDR|127.0.0.1:8092> 的 /health ＋ POST /v1/chat/completions（真上游 ZERG_AI_UPSTREAM，缺省 http://127.0.0.1:9400）",
			run:      cmdCocoonBroker,
		},
		// ---- A 档 批 1（换件面）· A12：卵的**单文件封装**（`*.egg`）工具 —— 从「虫卵」归名到「虫茧」（决-13）----
		// 依据：`~/.zerg/state/拍板落定-所有稿-20261007.md` 决-13（Mr2109「拍 按你推荐」）。
		// 底层唯一实现 = `core/cmd/zerg/zerg_egg.go`（`scripts/<机型>/zerg-egg.py` 的 Go 真搬体 · DEV-1798）——
		// 命令面**只做入口 ＋ 转发**（连退码也不改写；✗ 不重写 `zerg_egg.go` 里任何一段）。
		// ★ 族名口径：`egg` 已被【虫卵（沙箱/容器）投影面】占用（`egg ls/show/run/stop`）⇒ 本工具归「虫茧」名下（✗ 不叫 `egg pack`）。
		{
			path:        []string{"cocoon", "pack"},
			summary:     "把一棵源目录打成单文件卵包（`*.egg` · 底层 = `zerg_egg.go` 真搬体 · 命令面只转发不翻译）",
			usage:       "zerg cocoon pack <源目录> <输出.egg> [--align <n>] [--chunk-size <n>] [--engine-dir <目录>] [--libs-dir <目录>] [--self-test]",
			arity:       "any",
			args:        []string{"源目录", "输出 .egg 件（旗标逐字来自 scripts/x" + "3/zerg-egg.py 的真搬体 · 原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdCocoonEgg,
		},
		{
			path:        []string{"cocoon", "info"},
			summary:     "读一枚卵包的索引（magic/align/段数/逐段 sha256 · 底层 = `zerg_egg.go` 真搬体）",
			usage:       "zerg cocoon info <卵包.egg>",
			arity:       "any",
			args:        []string{"卵包 .egg 件（旗标逐字来自真搬体 · 原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdCocoonEgg,
		},
		{
			path:        []string{"cocoon", "verify"},
			summary:     "校验一枚卵包（头块/对齐/逐段 sha256 · 可选重建到目录 · 底层 = `zerg_egg.go` 真搬体）",
			usage:       "zerg cocoon verify <卵包.egg> [--rebuild-into <目录>]",
			arity:       "any",
			args:        []string{"卵包 .egg 件（旗标逐字来自真搬体 · 原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdCocoonEgg,
		},
		{
			path:        []string{"cocoon", "xfer"},
			summary:     "按块可续传地把一枚卵包拷到目标（中断可续 · 坏块自愈 · 底层 = `zerg_egg.go` 真搬体）",
			usage:       "zerg cocoon xfer <源.egg> <目标.egg> [--chunk-size <n>]",
			arity:       "any",
			args:        []string{"源 .egg 件", "目标 .egg 件（旗标逐字来自真搬体 · 原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdCocoonEgg,
		},
	}
}
