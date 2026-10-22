package main

// 命令表 · ui 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_ui() []*command {
	return []*command{
		// ── 门禁面 · `zerg ui i18n`（i18n 门进命令面 · 2026-09-25 · 缺口 C-10）──────────
		//   病灶：i18n 门在命令面**没有入口** ⇒ 人要手搓 `python3 ui/scripts/check-i18n.py`，
		//   退码与 stdout/stderr 全靠人盯。本条目 = 该脚本的命令面等价物：
		//   **脚本退多少，命令面退多少**（薄壳，不翻译门禁失败）。
		{
			path:     []string{"ui", "i18n"},
			kind:     "I18n",
			summary:  "i18n 门禁（四道门 G1..G4 · 脚本退多少命令面退多少）",
			usage:    "zerg ui i18n [--json <字段>]",
			fields:   []string{"rc", "verdict"},
			endpoint: "",
			run:      cmdI18n,
		},
		// ── S7 · 5 键归口（ui 族扩容）：`check-i18n`/`i18n-audit` 两件独立脚本收成命令面 ────────────
		// 归口件：`~/.zerg/state/②类-5键-CLI工具子命令归口-20261007.md`（② 类接线 · 5 键 · CLI 工具子命令）。
		// 底层唯一实现 = `i18n_audit.go`（`ui/scripts/i18n_audit.py` 的 Go 真搬体）；命令面**只做入口 ＋ 转发**
		// （连退码也不改写）—— ✗ 不重写真搬体里任何一段。纯分析工具 · 无自动调用者 ⇒ 排末。
		{
			path:        []string{"ui", "i18n-audit"},
			summary:     "i18n 中文字面量审计（纯分析工具 · 落点走 `ZERG_I18N_AUDIT_OUT` · 底层 = `i18n_audit.go` 真搬体 · 命令面只转发不翻译）",
			usage:       "zerg ui i18n-audit [classify|keys|all]",
			arity:       "any",
			args:        []string{"模式（classify/keys/all · 缺省 all · 旗标逐字来自真搬体 · 原样透传）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdI18nAudit,
		},
		// ---- A 档 批 3（运行面）· A6：UI「脱离会话」启动口的 **CLI 正门**（`zerg ui start` · 族 `ui` 扩 action · 真搬）----
		// 依据：`~/.zerg/state/A档18件-收CLI设计稿-20261007.md` 批 3 · A6（原底层件 = `scripts/build/start-zerg-ui.sh`）。
		// ★ 真搬（2026-10-08）：底层行为收进 `ui_start.go`（`setsid`/逐 PID 点名/日志重定向 ⇒ 逐档等价复现 ·
		//   ✗ 不再 exec 那个脚本；只 `ps`/`tail` 这类 OS 工具仍直调）。
		{
			path:        []string{"ui", "start"},
			summary:     "UI 脱离会话启动（真搬 · 行为落 Go —— 不再 exec scripts/build/start-zerg-ui.sh）",
			usage:       "zerg ui start",
			arity:       "any",
			args:        []string{"旗标与退码逐档照 scripts/build/start-zerg-ui.sh（真搬 · 原件退役后逐字同）"},
			endpoint:    "",
			passthrough: true,
			run:         cmdUIStart,
		},
	}
}
