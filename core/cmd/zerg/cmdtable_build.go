package main

// 命令表 · build 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_build() []*command {
	return []*command{
		// ---- 批 D · T-48 构建族（K 族 · §3.5 · §7.1 P11 · §17.2 ③ 建环）----
		{
			path:    []string{"build", "ls"},
			kind:    "BuildArtifact",
			summary: "制品现状（现读 bin/ 逐件 sha256 + 身份件）—— 清单真源仍是构建脚本",
			usage:   "zerg build ls [--json <字段>]",
			fields:  []string{"name", "sha256", "bytes", "mtime", "version", "code", "built"},
			// ★ 缺口 GAP-20260927-311（批⑤ · 2026-10-13）：`identity`（逐件**身份串**）走**可选字段层**——
			//   登在出口白名单、`--json` 点得出来，但不进 `fields` 冻结表 ⇒ 不重生命令页。
			optFields: []string{"identity"},
			endpoint:  "",
			run:       cmdBuildLs,
		},
		{
			path:     []string{"build", "show"},
			kind:     "BuildArtifactShow",
			summary:  "一件的**身份**（sha256/mtime/inode/type/arch/签名态 —— 换件后验「在跑的件 == 盘上件」要它）",
			usage:    "zerg build show <件> | --all [--json <字段>]",
			arity:    "any",
			args:     []string{"件名（bin/ 下的名字，或一个路径）"},
			fields:   []string{"name", "sha256", "bytes", "mtime", "inode", "type", "arch", "signed"},
			endpoint: "",
			run:      cmdBuildShow,
		},
		// ---- 缺口 `GAP-20261003-12`：构建签名**身份体检**（只读面 · 三格）----
		// 病征：构建脚本的身份解析（`build-all.sh:173-205` resolve_sign_identity）没有 CLI 出口。
		// 治法：只读复现该解析（真源 = scripts/build/internal/sign/sign.go 的 Resolve），出三格
		//（解析身份 · 两态标记 · 最终 SIGN_MODE）⇒ 判据可从 CLI 打得着 · 零写、不重编、不碰 bin/。
		{
			path:     []string{"build", "doctor"},
			kind:     "BuildDoctor",
			summary:  "构建签名**身份体检**（只读 · 复现构建脚本 `resolve_sign_identity` ⇒ 三格：解析身份 · 两态标记 · 最终 SIGN_MODE）",
			usage:    "zerg build doctor [--json <字段>]",
			arity:    "none",
			args:     []string{"（无位置参数）"},
			fields:   []string{"identity", "marker", "sign_mode"},
			endpoint: "",
			run:      cmdBuildDoctor,
		},
		{
			path:    []string{"build", "all"},
			machine: machineHostName,
			danger:  &dangerSpec{dangerD3, "主机名", "重编制品（自举档 `--only cli` 只写 bin/zerg 一件；**换件档会覆盖 bin/ 里的在跑制品** · 底层 Go 真身 scripts/build/cmd/zerg-build-all/main.go）", "GAP-20261005-39 · family_build.go:279 自印「危险档：D3」· §17.2 ③ 建环", true, dangerScopeSystem},
			opened:  true,
			summary: "重编制品（**自举档已开放**：`--only cli` 只写 bin/zerg 一件；换件档/发布档仍未开放）",
			usage:   "zerg build all [--node <机>] [--only cli] [--dry-run | --confirm=<主机名> --yes]",
			run:     cmdBuildPassthrough,
		},
		{
			path:    []string{"build", "release"},
			machine: machineHostName,
			danger:  &dangerSpec{dangerD3, "主机名", "打包发布件（清单档 `--manifest-only` 只写 dist/<版本>/release/；**换件档真跑未开放** · 底层 Go 真身 scripts/build/cmd/zerg-build-all/main.go）", "GAP-20261005-39 · family_build.go:279 自印「危险档：D3」· §17.2 ③ 建环", true, dangerScopeRepo},
			opened:  true,
			summary: "打包发布件（**计划面已开放**：`--dry-run` 出计划件；**清单档 `--manifest-only` 已开放** —— 只重写 `dist/<版本>/release/` 的清单与逐件 sha256，不编译、不重打包、不碰在跑件；**换件档真跑已开放（2026-10-10 · Mr2109 拍）** —— 内含全量构建、会覆盖 `bin/` 里的在跑制品）",
			usage:   "zerg build release [--node <机>] [--manifest-only] [--dry-run | --confirm=<主机名> --yes]",
			run:     cmdBuildPassthrough,
		},
		{
			path:        []string{"build", "wall"},
			kind:        "BuildWall",
			summary:     "茧壁构建（真身 = `scripts/build/cmd/zerg-build/main.go`（330 行）· 只转发、不翻译 · 退码原样转出 0/2 · 递不到真身 ⇒ 8）",
			usage:       "zerg build wall [--debug|--help]",
			arity:       "any",
			args:        []string{"旗标与退码逐字来自 scripts/build/cmd/zerg-build/main.go（原样透传 · 含 --debug / --help）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdBuildWallFamily,
		},
	}
}
