package main

// 命令表 · fs 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_fs() []*command {
	return []*command{
		// ---- F37（2026-10-15）：`fs` 族（乙档 · 用户已拍 · 族名照账面 want 逐字）----------------
		// ★ 本笔**只接 `fs unprotected` 一条**（`hidden-faces` 半归 F44 ⇒ ✗ 不碰 `family_fs.go`）。
		// 族名与子动作名照账面三条 want 原文（`GAP-20260927-223`/`-224`/`-225` 的 ③ 长线逐字写
		// `zerg fs unprotected` / `zerg fs hidden-faces`）⇒ 零口径改名。
		// ★ 分工（用户 2026-10-15 拍定）：`zerg repo status` 管「本仓面现状」（七栏不动）·
		//   `zerg fs` 管「**盘 ⟷ 版本面的差**」（含被忽略面 ＋ 体量排行）。
		// ★ 判据一字不落本表项：判据核心 = `family_fs.go:fsUnprotectedCore`（F38 已落 · 本笔只接线 ·
		//   ✗ 不改判据语义 · ✗ 不落第二套判脏口径 —— 真源仍全在 git 自己那一面：
		//   `git ls-files --others`（未跟踪 ∩ 盘上在）＋ `git check-ignore`（未被忽略））。
		// ★ 体量口径 = 降序前 50（`fsUnprotectedDefaultTop`）· 覆盖树 = 全仓（显式排除首段
		//   `bin`/`dist`/`.git`）。退码原样转出核心三态：0 无件 / 1 有件（逐件点名）/ 2 不给结论。
		// ★ ✗ 不新增旗标（usage 只列既有的 `--json` ⇒ 门73 旗标名表零改动 · usageDeclaresFlag 无新增落点）。
		{
			path:    []string{"fs", "unprotected"},
			kind:    "FsUnprotected",
			summary: "列**未保护件**（盘上有 / 版本面无 / 未被忽略 —— 真源全走 git 自己那一面）· 只读 · 体量降序取前 50（覆盖全仓 · 排除 `bin`/`dist`/`.git`）· 退码 0 无件 / 1 有件（逐件点名）/ 2 不给结论",
			usage:   "zerg fs unprotected [--json <字段>]",
			arity:   "none",
			args:    []string{"（不收位置参数 —— 仓根取现读工作树：「盘 ⟷ 版本面的差」是**本仓**的读数 · ✗ 不开 `--root`）"},
			fields:  []string{"path", "bytes", "kind"},
			run:     cmdFSUnprotected,
		},
		// ---- GAP-20261017-10（P0 · 2026-10-17）：`fs` 族三条**正门** `read`/`write`/`run` -----------
		// 动因（缺口正文逐字）：对话面要撤 `bash`/`read`/`write`/`edit`，而虫族 CLI **没有对应正门** ——
		// `zerg run|exec|shell|sh` 四条全是未知命令（rc=2），`fs` 族只有 `unprotected` 一个动作
		// （`fs read` / `fs write` 报「是族名/中间节点」）⇒ 先造三条正门，才撤得动对话面。
		// ★ 判据/实现**全部**落在 `core/cmd/zerg/family_fs_tools.go`（✗ 一个字都不落 `family_fs.go` ——
		//   那件是 `fs unprotected` 的判据核心，本笔不改它的语义）。本处只做命令树登记（八格 + 危险档）。
		// ★ 三档：`fs read` **危险档 0**（只读）· `fs write` **D2**（照 `script inventory sync` 那一套）·
		//   `fs run` **D3**（最高 · 首词硬白名单 fail-closed · argv 直投不经 shell）。
		{
			path:    []string{"fs", "read"},
			kind:    "FsRead",
			summary: "只读一件（仓根相对 · 人面出**带行号文本**）· `--json` 闭集五格（path/content/total_lines/offset/limit）· 零写盘 · 件不存在 ⇒ 2（不给结论 · ✗ 不冒充空件）· `--allow-sibling-repo <名>` 显式开闸读兄弟仓（闭集只认登记过的兄弟仓名 · 缺省仍 fail-closed）",
			usage:   "zerg fs read <件> [--offset N] [--limit N] [--allow-sibling-repo <名>] [--json <字段>]",
			arity:   "any",
			args:    []string{"件（仓根相对路径；绝对路径只收**仓内**的 —— 出仓 ⇒ 2；给 `--allow-sibling-repo <名>` 时改按**兄弟仓根**解析 · 闭集只认登记过的兄弟仓名）", "窗口：`--offset N` 起始行（1 起 · 缺省 1）· `--limit N` 取几行（缺省 0 = 到件尾）"},
			fields:  fsReadFields,
			run:     cmdFSRead,
		},
		{
			path:    []string{"fs", "write"},
			kind:    "FsWrite",
			summary: "写/追加本仓一件（**D2**：`--dry-run` 零副作用 · 缺 `--yes` ⇒ 2 · 写面先落审计 `edit_audit.jsonl`）· 禁绝对路径出仓 · 禁符号链接 · 父目录不存在 ⇒ 2（✗ 不自建）· `--replace <补丁件>` 定点替换（每格 `old` 恰好命中 1 次才改 · **不必先读全文**）· `--allow-sibling-repo <名>` 显式开闸写兄弟仓（闭集只认登记过的兄弟仓名 · 审计行带 `sibling_repo`/`sibling_root` · 缺省仍 fail-closed）",
			usage:   "zerg fs write <件> [--content <文本> | --content-stdin | --replace <补丁件>] [--append] [--by <谁>] [--allow-sibling-repo <名>] [--json <字段>] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"件（仓根相对路径 · 父目录必须**已存在**；给 `--allow-sibling-repo <名>` 时改按**兄弟仓根**解析 · 闭集只认登记过的兄弟仓名）", "内容来源：`--content <文本>` / `--content-stdin` / `--replace <补丁件>`（**三选一**；补丁件 = `[{\"old\":…,\"new\":…}]` · 每格 `old` 恰好命中 1 次 · 与 `dev edit --replace` 同形 · 走它**不必先读全文**）"},
			fields:  fsWriteFields,
			danger:  &dangerSpec{dangerD2, "件路径（仓根相对）", "写/追加**本仓工作树内**的一件（同目录临时件 + rename 原子写 · 写后读回对拍 · 不等即按前态写回）；审计先落 `<状态目录>/edit_audit.jsonl`（事件 `fs_file_written`）—— 可逆：按该审计行的 `fs_before_sha256` 恢复（件原本不在 ⇒ 删件）", "缺口 `GAP-20261017-10`（P0）· 同族 D2 写面先例 `script inventory sync`（--dry-run 零副作用 · 缺 --yes fail-closed · 写面进审计）", false, dangerScopeRepo},
			opened:  true,
			run:     cmdFSWrite,
		},
		{
			path:    []string{"fs", "run"},
			kind:    "FsRun",
			layer:   "host",
			summary: "跑一条命令（**D3**：首词硬白名单 `zerg|go|git|make|npm|cargo` · 其余 **rc=2 拒执** · **argv 直投 ✗ 不经 shell** · `--cwd` 必须在仓内 · 超时缺省 120s · 输出超 64KB 截断并标截断）",
			usage:   "zerg fs run <命令…> [--cwd <目录>] [--timeout <秒>] [--json <字段>] [--dry-run | --yes]",
			arity:   "any",
			args:    []string{"命令与实参（首词只许 zerg/go/git/make/npm/cargo；子命令另有闭集：go `build|test|vet|fmt` · git `status|log|diff|show|cat-file|ls-files`）", "子命令自己的旗标写在 `--` 之后（例：`zerg fs run git log -- --oneline -3 --yes`）—— CLI 解析器把 `-` 开头的词当本命令旗标，`--` 之后一律原样转给子命令"},
			fields:  fsRunFields,
			danger:  &dangerSpec{dangerD3, "整条 argv（命令串）", "以本仓为工作目录**真起子进程**（argv 直投 · 不起 shell · 输出合流回显）；子命令闭集当场拒掉 `go run` / `git push` / `git reset --hard` 一类 —— 本命令**自己不写仓件、不落审计**（写不写由被跑的那条命令自己的危险档负责）", "缺口 `GAP-20261017-10`（P0）· 危险档最高档（D3）· fail-closed 硬白名单", false, dangerScopeUnknown},
			opened:  true,
			run:     cmdFSRun,
		},
	}
}
