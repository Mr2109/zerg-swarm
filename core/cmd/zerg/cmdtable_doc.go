package main

// 命令表 · doc 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_doc() []*command {
	return []*command{
		// ── 缺口面 P0 之外 · `zerg doc meta fill`（批量回填文件头 · 2026-09-21）──────────────
		//   病灶原样（开工记录 D3 §三）：③ 那 63 篇的抬头是**一次性 `/tmp` 脚本**回填的 ——
		//   命令面**没有「批量改字段」这条路**。本条目就是那次脚本的命令面等价物。
		{
			path:    []string{"doc", "meta"},
			kind:    "DocMeta",
			summary: "文档元数据面（动作 `fill` = 批量回填文件头：只填机械可判的日期 + 不开源标注 · 默认干跑 · 写审计）",
			usage:   "zerg doc meta fill [--scope devdocs | <目录>] [--docs-ver <X.Y.Z>] [--dry-run] [--by <谁>] [--json <字段>] [--yes]",
			arity:   "any",
			args:    []string{"动作（本版只有 fill）"},
			fields:  docMetaFillFields,
			danger: &dangerSpec{dangerD2, "（被扫根）",
				"回填文件头（日期 + 不开源标注）—— 只加机械可判的抬头行、不碰正文语义；可回滚 = git",
				"缺口-命令面-20260921 §八 H2 · 开工记录 D3 §三 新增 1 条 · D3③-a 的等价命令", false, dangerScopeRepo},
			// `opened`: 真跑已开放（`--yes` 就回填文件头；审计先落盘 + 回读对拍 sha256）。
			opened: true,
			run:    cmdDocMeta,
		},
		// ── 缺口面 `GAP-20260927-129` ④「把『定格』做成可现算读数」：只读命令 `zerg doc consumers` ──
		//   逐消费者现读其**解析出的版本目录**（zerg_overview / help export / doc meta fill / doc_search）·
		//   缺件不静默（版本档案取源根判不了 ⇒ 退 8 并点名候选）· 判定口径与判据 `check-doc-consumers`
		//   共用同一份现读（✗ 不两处各写一套）。
		{
			path:    []string{"doc", "consumers"},
			kind:    "DocConsumers",
			summary: "四个消费判据各自解析出的「定格版本目录」现读读数（**只读**：不写盘、不起子进程、不联网 · 逐消费者可比出定格是否分叉 · 判据 `check-doc-consumers` 共用同一读数）",
			usage:   "zerg doc consumers [--json <字段>]",
			arity:   "none",
			args:    []string{"（无位置参数）"},
			fields:  docConsumersFields,
			run:     cmdDocConsumers,
		},
		// ── S7 · 5 键归口（doc 族扩容）：3 件独立工具收成命令面（`kb-docs-sync`=A18 · `gen-tools-md` · `doc-index-gen`）──
		// 归口件：`~/.zerg/state/②类-5键-CLI工具子命令归口-20261007.md`。三件底层唯一实现 = 各自 Go 真搬体
		// （`kb_docs_sync.go` / `gen_tools_md.go` / `doc_index_gen.go`）；命令面**只做入口 ＋ 转发**（连退码也不改写）
		// —— ✗ 不重写真搬体里任何一段。`gen-tools-md`/`index-gen` 被门 `check-doc-freshness` 真跑。
		{
			path:        []string{"doc", "kb-sync"},
			summary:     "KB 文档同步（核验入库后把候选标记进 kb registry · 底层 = `kb_docs_sync.go` 真搬体 · 命令面只转发不翻译 · = A18）",
			usage:       "zerg doc kb-sync [--mark <file|全部>] [--help]",
			arity:       "any",
			args:        []string{"旗标逐字来自 tools/kb_docs_sync.py 的真搬体 · 原样透传"},
			endpoint:    "",
			passthrough: true,
			run:         cmdDocKbSync,
		},
		{
			path:        []string{"doc", "gen-tools-md"},
			summary:     "生成 TOOLS 文档（publish/docs/TOOLS.*.md · 被门 `check-doc-freshness` 真跑 · 底层 = `gen_tools_md.go` 真搬体）",
			usage:       "zerg doc gen-tools-md [--help]",
			arity:       "any",
			args:        []string{"旗标逐字来自 tools/gen_tools_md.py 的真搬体 · 原样透传"},
			endpoint:    "",
			passthrough: true,
			run:         cmdDocGenToolsMd,
		},
		{
			path:        []string{"doc", "index-gen"},
			summary:     "生成某版本目录的 `INDEX.md`（被门 `check-doc-freshness` 真跑 · 底层 = `doc_index_gen.go` 真搬体）",
			usage:       "zerg doc index-gen <版本目录> [--out <INDEX.md>] [--help]",
			arity:       "any",
			args:        []string{"版本目录", "旗标逐字来自 tools/doc_index_gen.py 的真搬体 · 原样透传"},
			endpoint:    "",
			passthrough: true,
			run:         cmdDocIndexGen,
		},
		// ---- A 档 批 2（文档面）· A14：文档站「导出 + 构建」的 **CLI 正门**（`zerg doc site build` · 族 `doc` 扩 action · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 2 · A14。
		// ★ 真搬（2026-10-08）：`docs/site/export-and-build.sh` 的四段编排落进 `doc_site_build.go`
		//   （✗ 不再 exec 那个脚本）；只 `mkdocs`/`venv`/`pip` 那四处外部工具仍以子进程直调。
		{
			path:        []string{"doc", "site", "build"},
			summary:     "文档站「导出 + 构建」（真搬 · 行为落 Go —— 不再 exec docs/site/export-and-build.sh）",
			usage:       "zerg doc site build [--strict | --serve | --help]",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 docs/site/export-and-build.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdDocSiteBuild,
		},
	}
}
