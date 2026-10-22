package main

// 命令表 · code 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_code() []*command {
	return []*command{
		{
			path:    []string{"code", "find"},
			kind:    "CodeFind",
			summary: "在码里找一处东西在哪（只读取证 · 手搓 grep/git grep 的替身 · **扫工作树**：含未跟踪件与被忽略目录，比 `git grep` 的索引面多一片（两个面各扫多少件，命令每跑一次自己报一行「扫了 N 件」—— **不在这里写死**，写了就会烂；差值由运行时的数说话）；跳过 >2MB 的件）",
			usage:   "zerg code find <正则> [--path <目录或单件·仓外根用绝对路径显式指（只读）>] [--glob <模式>] [--limit <N>] [-i|--ignore-case] [--after <N>|-A<N>] [--before <N>|-B<N>|-C<N>] [--kind <代码|注释>] [--timeout <秒>] [--count | --files-only] [--full] [--hidden] [--include-runtime] [--tracked] [--json <字段>]",
			arity:   "any",
			args:    []string{"正则（**Go/RE2 语法** —— 不是 POSIX BRE；**交替**用**裸** `|`（例：a|b）；写 `\\|` 会被**拒**：本命令退 2 并给替身，因 RE2 里 `\\|` 是**字面竖线**不是交替）"},
			// ★ `GAP-20260928-118`（2026-09-28 · 本枚）：字段表补上 `truncated` —— 截断位此前
			// 只能从顶层包封读，字段面没有直读口（给 `truncated` 即报未知字段）。三档面（默认 /
			// `--count` / `--files-only`）逐行都产出它（**跑级真值**：本跑真裁了 = true，没裁 = false）
			// ⇒ 表与产出集同源；不给这一格时输出逐字节不变（`marshalObject` 只投影点名的格）。
			// ★ `GAP-20260929-29`（2026-10-04）：字段表再补上 `face`（工作树面 / 索引面）—— 与
			// `meta.face` 同源（`codeFaceWorktree`）⇒ 条目面与包封面各有一个直读口；同样只在
			// `--json` 那一态落键、不给这一格时输出逐字节不变。
			fields: []string{"path", "line", "text", "kind", "match", "truncated", "face"},
			// ★ `GAP-20260928-142`（2026-10-10 · 本枚）：`count` 走**可选字段层**（`optFields`）
			// —— 登在出口白名单里、`--json count` 点得出来（默认面 = 本跑命中条数 · `--count` 面
			// = 按件命中数既有语义），但**不进**上面那份文档/帮助导出面的**冻结表** ⇒ 不造
			// 「表列着、面不产出」的反例，也无需重生成命令页（与 `approveOptionalFields` 同一体例）。
			optFields: []string{"count", "path_kind"},
			endpoint:  "",
			run:       cmdCodeFind,
		},
		{
			path:     []string{"code", "show"},
			kind:     "CodeShow",
			summary:  "看源码里**某一行**长什么样（带 `件:行` · 只读取证 · 手搓 `sed -n` / `awk` 的替身）· 纯件名 = **整件读**（缺口 `GAP-20261001-59` · 手搓 `read_file` 的正门）",
			usage:    "zerg code show <件:行 | 件:起-止 | 件> [--ctx <N>] [--batch <件:行,…>] [--full] [--json <字段>]",
			arity:    "any",
			args:     []string{"件:行（件 = 仓相对路径 · 行 = 正整数）· 纯件名 = 整件读"},
			fields:   []string{"path", "line", "text", "target"},
			endpoint: "",
			run:      cmdCodeShow,
		},
		{
			path:     []string{"code", "diff"},
			kind:     "CodeDiff",
			summary:  "按件与 HEAD 的差（只读 · 手搓 `git diff` 的替身；`--stat` 另附整段差异统计）",
			usage:    "zerg code diff <件> [--stat] [--json <字段>]",
			arity:    "any",
			args:     []string{"件（仓相对路径）"},
			fields:   codeDiffFields,
			endpoint: "",
			run:      cmdCodeDiff,
		},
		// ---- 码面 R5 四枚（缺口 `GAP-20260927-235/236/237` · `GAP-20260928-30` · 2026-10-04）----
		// 实现件已落（`family_code.go` / `family_h.go`）⇒ 本处只做**命令树登记**（同一批 code 族）。
		// ★ 新命令 ⇒ 同批随动：cli-matrix must-fail 格 ＋ cli-contract-baseline ＋ 门69 两半（详见收口单）。
		{
			path:     []string{"code", "callers"},
			kind:     "CodeCallers",
			summary:  "反查一个函数的**全部调用点**（只读 · 手搓全仓搜的替身 · 缺口 GAP-20260927-237）· role=def 标定义行",
			usage:    "zerg code callers <函数名> [--func <函数名>] [--path <目录或单件>] [--glob <模式>] [--limit <N>] [--full] [--json <字段>]",
			arity:    "any",
			args:     []string{"函数名（与 --func 两收 · 后者优先）"},
			fields:   []string{"path", "line", "role", "text"},
			endpoint: "",
			run:      cmdCodeCallers,
		},
		{
			path:     []string{"code", "denom-recount"},
			kind:     "CodeDenomRecount",
			summary:  "两份件清单的**对称差**（只读 · 「计数差 1」时定点名那 1 件 · 缺口 GAP-20260927-236）",
			usage:    "zerg code denom-recount <左件> <右件> [--json <字段>]",
			arity:    "any",
			args:     []string{"左清单件（一行一件）", "右清单件"},
			fields:   []string{"path", "side"},
			endpoint: "",
			run:      cmdCodeDenomRecount,
		},
		{
			path:     []string{"code", "git-two-way"},
			kind:     "CodeGitTwoWay",
			summary:  "同一 git 命令**带/不带**某全局选项两版对比（只读诊断 · 缺口 GAP-20260927-235）",
			usage:    "zerg code git-two-way <k=v 或 -开关> -- <git 参数…> [--json <字段>]",
			arity:    "any",
			args:     []string{"全局选项", "git 参数（写在 -- 之后，防被当 zerg 旗标）"},
			fields:   []string{"side", "text"},
			endpoint: "",
			run:      cmdCodeGitTwoWay,
		},
		{
			path:     []string{"code", "audit-readings"},
			kind:     "CodeAuditReadings",
			summary:  "列出注释里的**绝对读数** + 真源取法（只读 · **只列候选不下判断** · 缺口 GAP-20260928-30）",
			usage:    "zerg code audit-readings --path <件或目录> [--glob <模式>] [--limit <N>] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：件/目录由 --path 给）"},
			fields:   []string{"path", "line", "reading", "hint", "text"},
			endpoint: "",
			run:      cmdCodeAuditReadings,
		},

		// ---- 同值异名常量的只读面（缺口 `GAP-20261001-69` 的前置工具 · 本枚）----
		// ★ 只读（L0）：零写盘；**不改**任何重复常量、**不改**任何调用点 —— 本单只造工具。
		{
			path:     []string{"code", "const-dup"},
			kind:     "CodeConstDup",
			summary:  "扫**同值异名常量**（只读 · 同一个字符串值被多个常量各抄一份 ⇒ 逐条列 值 / 保留名 / 被废弃名·件:行 / 归属类）· 缺口 `GAP-20261001-69` 的前置工具",
			usage:    "zerg code const-dup --path <件或目录> [--glob <模式>] [--root <仓根>] [--json <字段>] [--fail-on-hit]",
			arity:    "none",
			args:     []string{"（不收位置参数：件/目录由 --path 给）"},
			fields:   ccdFields,
			endpoint: "",
			run:      cmdCodeConstDup,
		},
	}
}
