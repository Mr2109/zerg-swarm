// builtin_rules.go — T5.2「默认拦什么清单化」（设计稿 docs/01-设计/设计-内建调试版-v1.2.md §〇 F1）。
//
// 本文件把两张清单落成**带 id 的数据**，并由 BuiltinFloor() 装配成地板规则集：
//
//	① BuiltinTable=any_mode_no_auto（id 命名空间 `any.*`）——「任何模式都不自动批」：
//	   显式 ask 规则 / 组织·系统级 ask / 需要用户交互的工具 / critical path 的 rm·rmdir / 工作目录外的读取
//	② BuiltinTable=never_auto_approve（id 命名空间 `never.*`）——「永不自动批」：
//	   包安装 / 变更型 git / rm·sudo / 破坏性云命令 / dotenv·密钥·git config·agent 自身配置
//
// ── 为什么落在 Go 源文件里，而不是仓内可嵌入的数据文件（二选一，这里给理由）──
//
//	· **编译期校验**：工具名 / 效果 / 匹配式形态写错 ⇒ 编译不过。数据文件要走 ParseRules，
//	  等于把「清单本身写坏」变成一个**运行期**失败模式（而本包里解析失败是 fail-closed 的：
//	  整份地板消失、拒绝启用）。
//	· **叶子包纪律**：本包零内部依赖、且不碰文件系统（T5.1）。数据文件要么 go:embed（多一份
//	  嵌入资源与解析路径），要么读盘（破纪律），要么塞进 Config.Floor 文本（谁注谁写，内建清单
//	  就不再是内建）。Go 数据 = 零解析、零 IO、零新增依赖。
//	· **注释同址**：「为何永不自动批」是中文散文，写在 Go 里与数据挨着、能被 gofmt 与评审看见；
//	  数据文件里只能写 `#` 注释并靠人维护格式。
//	· **少一个能静默失效的面**：编译期常量不存在「解析失败」这个失败模式。
//
// ── 效果只有两档（每条为什么是这一档，写在条目 Note 里）──
//
//	ask  = 要人批：任何档位都不得自动放行（含全放档 bypass），但**人批可放行**。
//	deny = 不设人批通道：要放行必须改地板 —— 只给不可逆、破坏面无可控的动作。
//
// 两表都落进**地板**（BuiltinFloor）：地板压过任何档位（T5.1 语义），这是「不自动批」唯一的
// 实现位置 —— 放在普通规则集里会被档位与 allow 规则放过去。
//
// ── 纪律 ──
//
//	· 工具名只用仓内**真实存在**的工具（本批只用 bash / read / write，见 builtinAllowedTools）；
//	  宁可不写，也不凭空造工具名。
//	· 写不成的条款**不硬凑**（不写 `ask bash(aws *)` 这类过度拦截来充数），而是记进
//	  BuiltinPendingEntries()：条款原文 + 为什么写不成 + 现在靠什么兜住。
//	· **形态要与工具匹配**：给 bash 的条目必须是**行首前缀**形态 —— 匹配式里出现 `/` 会被分类成
//	  "路径匹配式"（去读 path 参数），在 bash 调用上必然「判不了 ⇒ ask」，而且会把**该工具的全部调用**
//	  一起污染成 ask（连 `git status` 都变问人）。本批用 TestBuiltinFloor_GlobalAllowList 钉住这一点。
//	· 条目只加不减：删/改一条 = 放松一条保护，必须走评审并在提交信息里点名。
package policy

// ── 类型 ────────────────────────────────────────────────────────────────────

// BuiltinTable — 条目出自哪张清单。id 命名空间与它一一对应（any.* / never.*）。
type BuiltinTable string

const (
	// TableAnyModeNoAuto — 表①：「任何模式都不自动批」（档位不得放行）。
	TableAnyModeNoAuto BuiltinTable = "any_mode_no_auto"
	// TableNeverAutoApprove — 表②：「永不自动批」（自动批准通路的一票否决）。
	TableNeverAutoApprove BuiltinTable = "never_auto_approve"
)

// BuiltinRule — 一条内置清单条目（**数据**：id / 表 / 效果 / 工具 / 匹配式 / 为何）。
//
// 它只描述「拦什么」，不含任何执行路径：本包不接 toolobs / chat / gateway / agent（接线另批）。
type BuiltinRule struct {
	ID        string       // 稳定 id（命名空间 `any.` / `never.`；审计、用例、豁免审批都按它点名）
	Table     BuiltinTable // 出自哪张清单
	Effect    Effect       // ask | deny（**地板里不许出现 allow**：地板只加约束、不放松）
	Tool      string       // 工具名（仓内真实工具；已归一化小写）
	Specifier string       // 匹配式（"" = 整个工具；形态必须是现有三种可判别形态之一）
	Note      string       // 一句中文：为何永不自动批
}

// BuiltinPending — 清单里**暂未实体化成规则**的条款。
//
// 写不成一条规则的原因有两类：① 仓内没有对应工具（不凭空造名）；② 现有匹配式语法表达不了
// （只有"行首前缀 / 路径 / 域名"三种形态）。这类条款必须显式记账 —— 沉默等于假装覆盖了。
type BuiltinPending struct {
	ID       string       // 稳定 id（pending.*）
	Table    BuiltinTable // 出自哪张清单
	Claim    string       // 清单原条款
	Reason   string       // 为什么现在写不成一条规则
	Coverage string       // 这条条款现在靠什么兜住（引擎语义 / 工具层 / 待语法扩展）
}

// ── 表①：「任何模式都不自动批」（地板清单）────────────────────────────────────

// builtinInteractivePrograms — 「需要用户交互」这一类里可实体化的部分：
// **只用于交互**的整屏程序（源：core/internal/agent/bash_v101.go 的交互命令表；那里对**裸调用**
// 直接拒，本处对**任何用法**都要求人批 —— 两层分工：工具层管「会挂死」，策略层管「要先问人」）。
//
// 故意**不收**解释器类（python / python3 / node / bash / sh / zsh）：它们有正常非交互用法
// （`python3 跑脚本.py`、`sh 脚本.sh`），整体拦会把正常作业一起变成每次都要人批（误伤）；
// 它们的交互用法由工具层按「裸调用 / -i 交互标志」判定。
var builtinInteractivePrograms = []string{
	"vi", "vim", "nano", "less", "more", "top", "htop", "irb", "sqlite3", "mysql", "psql",
}

// builtinCriticalDeleteRules — 「critical path 的 rm / rmdir」可实体化的部分。
//
// **为什么只有家目录的三种拼写、且是逐字相等（不带通配）**：
//
//	· 现有语法只按「命令行首前缀」比（形态由**语法**决定，不看工具），而**匹配式里出现 `/` 会被判成
//	  "路径匹配式"**（match.go 的 classifySpec）⇒ 该条会去读 path 参数，用在 bash 命令上必然
//	  「参数取不到 ⇒ 判不了 ⇒ ask」。更糟的是它会**污染整个工具的判定**：只要该条工具名命中，
//	  同一工具的任何调用都会被升级成 ask（连 `git status` 都变 ask）。
//	  ⇒ 所以 `rm -rf /`、`rm -rf /Users`、`rmdir /` 这类带 `/` 的整条写法**写不得**（写 = 把 bash
//	  全部调用降级成问人），清单记账见 BuiltinPendingEntries() 的 pending.any.rm_critical_full。
//	· 带通配 `rm -rf /*` 会把 `/tmp` 与工作区内的正常清理一起 deny（过度拦截，与工具层 v1.0.2 的
//	  "允许删除域 = 工作区 + /tmp + 白名单"直接冲突）；拼写变体的**全集**已由工具层按**目标集合**
//	  判定（与拼写无关，见 docs/01-设计/设计-bash-工具-v1.0.2-删除范围门控.md）。
//
// 故策略层只钉"家目录被整条删掉"这几种最典型的写法（拼写全集由工具层兜），作为纵深第二道。
var builtinCriticalDeleteRules = []BuiltinRule{
	{
		ID: "any.critical.rm_home", Table: TableAnyModeNoAuto, Effect: EffectDeny,
		Tool: "bash", Specifier: "rm -rf ~",
		Note: "删家目录：用户全部数据（含未入库草稿与密钥）一并消失 —— 2026-09-06 家目录清空事故就是这一类。",
	},
	{
		ID: "any.critical.rm_home_env", Table: TableAnyModeNoAuto, Effect: EffectDeny,
		Tool: "bash", Specifier: "rm -rf $HOME",
		Note: "$HOME 是同一个目标的另一种拼写 —— 两种拼写都要在清单里，否则换个写法就绕过（拼写全集由工具层兜底）。",
	},
	{
		ID: "any.critical.rm_home_braces", Table: TableAnyModeNoAuto, Effect: EffectDeny,
		Tool: "bash", Specifier: "rm -rf ${HOME}",
		Note: "第三种拼写（${HOME} 带花括号）：只写两种，等于给第三种留门 —— 清单要么写全，要么在记账里写明没写全。",
	},
}

// builtinAnyModeNoAuto — 表①全量（顺序 = BuiltinFloor 顺序）。
var builtinAnyModeNoAuto = builtinAnyModeRules()

func builtinAnyModeRules() []BuiltinRule {
	out := make([]BuiltinRule, 0, len(builtinInteractivePrograms)+len(builtinCriticalDeleteRules))
	for _, p := range builtinInteractivePrograms {
		out = append(out, BuiltinRule{
			ID: "any.interactive." + p, Table: TableAnyModeNoAuto, Effect: EffectAsk,
			Tool: "bash", Specifier: p + " *",
			Note: p + " 是整屏 / 等 stdin 的交互程序：非交互执行会挂死等人 —— 任何档位都不得自动放行（含全放档），须人批。",
		})
	}
	out = append(out, builtinCriticalDeleteRules...)
	return out
}

// ── 表②：「永不自动批」（never-auto-approve）──────────────────────────────────

var builtinNeverAutoApprove = []BuiltinRule{
	// ── 包安装：装完就改了运行环境与依赖树，"没装过"这个状态回不去 ──
	{
		ID: "never.pkg.npm_install", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "npm install*",
		Note: "npm 装包会改依赖树（node_modules / package-lock）并执行 postinstall 脚本 ⇒ 供应链面，须人批。",
	},
	{
		ID: "never.pkg.npm_i", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "npm i*",
		Note: "`npm i` 是 install 的简写 —— 简写不进清单，等于用一个字母的长度绕过整条保护。",
	},
	{
		ID: "never.pkg.npm_ci", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "npm ci*",
		Note: "`npm ci` 会**清空并重建** node_modules：比 install 更具破坏性（本地改动直接抹掉）。",
	},
	{
		ID: "never.pkg.yarn_add", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "yarn add*",
		Note: "yarn 装包同样改依赖树并落 lock 文件 ⇒ 与 npm 同类，须人批。",
	},
	{
		ID: "never.pkg.pnpm_add", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "pnpm add*",
		Note: "pnpm 装包会改全局 content-addressable store 与仓内 lock ⇒ 影响面出仓，须人批。",
	},
	{
		ID: "never.pkg.pip_install", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "pip install*",
		Note: "pip 装的是**系统/用户级** Python 环境（-U 可覆盖既有版本）⇒ 影响范围超出工作区。",
	},
	{
		ID: "never.pkg.pip3_install", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "pip3 install*",
		Note: "pip3 同 pip —— 两种拼写都进清单，避免换写法绕过。",
	},
	{
		ID: "never.pkg.brew_install", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "brew install*",
		Note: "brew 装的是**整机**软件并会连带升级依赖 ⇒ 改了本机环境，不在工作区管辖范围内。",
	},
	{
		ID: "never.pkg.go_get", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "go get*",
		Note: "go get 改 go.mod / go.sum 并把外部代码拉进构建 ⇒ 改的是**构建输入**。",
	},
	{
		ID: "never.pkg.go_install", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "go install*",
		Note: "go install 拉外部源码编成可执行文件落到 GOBIN ⇒ 装的是「以后会被跑」的东西。",
	},
	{
		ID: "never.pkg.cargo_install", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "cargo install*",
		Note: "cargo install 拉 crates.io 源码在本机编译并落盘可执行文件 ⇒ 同 go install。",
	},

	// ── 变更型 git：丢弃本地状态 / 外发 ──
	{
		ID: "never.git.reset_hard", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "git reset --hard*",
		Note: "`--hard` 直接丢弃工作区未提交改动（未 add 的内容 git 也救不回）⇒ 不可逆。",
	},
	{
		ID: "never.git.checkout_paths", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "git checkout --*",
		Note: "`git checkout -- <路径>` 用索引版本覆盖工作区 ⇒ 同样是**丢弃**本地改动。",
	},
	{
		ID: "never.git.clean_force", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "git clean -fd*",
		Note: "`clean -fd` 删掉所有未跟踪文件/目录（构建产物、草稿、报告全在内）且不进回收站。",
	},
	{
		ID: "never.git.push", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "git push*",
		Note: "push 把本地状态**外发**且远端不可撤（--force 更是重写历史），还会触发 CI / 部署。",
	},

	// ── 删除与提权 ──
	{
		ID: "never.rm", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "rm *",
		Note: "rm 不进回收站 ⇒ 不可逆（「能不能删」由工具层按允许域判，这里只管「必须先问人」）。",
	},
	{
		ID: "never.rmdir", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "rmdir *",
		Note: "rmdir 是同一删除面的另一种命令 ⇒ 与 rm 一样须人批。",
	},
	{
		ID: "never.sudo", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "sudo*",
		Note: "sudo 把命令提到 root：越过文件权限与系统边界，影响面从「工作区」变成「整机」。",
	},

	// ── 破坏性云命令（可实体化的部分；aws/gcloud/az 的 delete 类见 BuiltinPendingEntries）──
	{
		ID: "never.cloud.terraform_destroy", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "bash", Specifier: "terraform destroy*",
		Note: "terraform destroy 按 state 拆掉整份云端资源（含数据库/存储）⇒ 一行命令产生账单级后果。",
	},

	// ── dotenv / 密钥：读 = 拿凭据，写 = 改认证面 ──
	{
		ID: "never.secret.dotenv_read", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "read", Specifier: "./.env",
		Note: ".env 是共享令牌的第三种来源（见 core/internal/config/config.go 的 TokenFilePath 注释）⇒ 读到即等于拿到凭据。",
	},
	{
		ID: "never.secret.dotenv_write", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "write", Specifier: "./.env",
		Note: "改写 .env 能替换令牌 / 注入环境变量 ⇒ 改的是**认证面**，不是普通文件。",
	},
	{
		ID: "never.secret.token_read", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "read", Specifier: "~/.zerg/token",
		Note: "共享令牌文件本体（config.TokenFilePath() 的默认落点）—— 读它等于偷凭据。",
	},
	{
		ID: "never.secret.ssh_read", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "read", Specifier: "~/.ssh/**",
		Note: "私钥目录：私钥泄露 = 身份被冒用，而且**没法撤销**（只能换密钥）。",
	},

	// ── 配置文件：写 = 改 agent 自己的行为面 ──
	{
		ID: "never.cfg.git_config_read", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "read", Specifier: "./.git/config",
		Note: "git config 里可能有带令牌的 remote URL —— 读它通常是「找凭据」的第一步。",
	},
	{
		ID: "never.cfg.git_config_write", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "write", Specifier: "./.git/config",
		Note: "改 git config 能换 remote / 改提交身份 / 改 hook 路径 ⇒ 直接改**代码外发目标**。",
	},
	{
		ID: "never.cfg.fleet_write", Table: TableNeverAutoApprove, Effect: EffectAsk,
		Tool: "write", Specifier: "./gateway/fleet.yaml",
		Note: "fleet.yaml 是模型路由与网关自身配置 ⇒ 改它等于改 agent 自己的行为面（自我提权的入口）。",
	},
}

// ── 未实体化的条款（宁可不写，也不硬凑）──────────────────────────────────────

var builtinPending = []BuiltinPending{
	{
		ID: "pending.any.explicit_ask", Table: TableAnyModeNoAuto, Claim: "显式 ask 规则不被任何档位放松",
		Reason:   "这不是「一条规则」，而是**引擎不变量**（T5.1 已实现）：普通 ask 命中即 ask，全放档也不改写它。",
		Coverage: "引擎语义本身 + 本批用例 TestBuiltinFloor_Invariants（全放档下显式 ask 仍为 ask）。",
	},
	{
		ID: "pending.any.org_system_ask", Table: TableAnyModeNoAuto, Claim: "组织 / 系统级下发的 ask 不得被会话档位覆盖",
		Reason:   "载体就是**本文件这两张表 + BuiltinFloor()**（组织地板的落点已就位）；「谁下发、如何签署、如何分发」属于配置分发（T5.3+），本包不做。",
		Coverage: "BuiltinFloor() 进的是 Engine.floor ⇒ 地板优先于档位（T5.1 语义）。",
	},
	{
		ID: "pending.any.interactive_tools", Table: TableAnyModeNoAuto, Claim: "需要用户交互的工具",
		Reason:   "仓内**没有**问人型工具（respond / reject 两轨仍是设计稿 F9 / F11）⇒ 不凭空造工具名；本批只把「经 bash 调用的整屏交互程序」实体化（any.interactive.*）。",
		Coverage: "any.interactive.*（11 条）+ 工具层对裸解释器/交互标志的直接拒（bash_v101.go）。问人工具落地后另批补 `ask <真名>`。",
	},
	{
		ID: "pending.any.read_outside_workdir", Table: TableAnyModeNoAuto, Claim: "工作目录外的读取",
		Reason:   "现有三种匹配式形态（无 / 行首前缀 / 路径 / 域名）里**没有「是否在工作目录内」这个谓词** ⇒ 写不出来；硬写 `read(/Users/**)` 之类既漏又滥。",
		Coverage: "工具层 validatePath 已**硬拒**工作区外路径（观测码 path_outside，T1.2）：语义已覆盖，策略层待语法扩展（T5.4+）后补条目。",
	},
	{
		ID: "pending.any.rm_critical_full", Table: TableAnyModeNoAuto, Claim: "critical path 上的 rm / rmdir 的删除（全集）",
		Reason:   "两层原因：① 全集要按**删除目标集合**判定（工作区 + /tmp + 白名单之外一律），行首前缀形态表达不了「目标在哪」，写通配 `rm -rf /*` 又会把 /tmp 与工作区内清理一起 deny（过度拦截）；② **带 `/` 的匹配式在本语法里被判成路径匹配式**（读 path 参数）⇒ 用在 bash 上必然判不了，还会把该工具的所有调用一起污染成 ask ⇒ `rm -rf /`、`rm -rf /Users`、`rmdir /` 这类整条写法写不得。",
		Coverage: "工具层 v1.0.2 删除范围门控（与拼写无关）兜全集；策略层只钉家目录三种拼写的整条写法（any.critical.rm_home / rm_home_env / rm_home_braces，纵深第二道）；绝对路径类待形态层支持（如显式 cmd: 前缀声明）后补。",
	},
	{
		ID: "pending.never.cloud_delete_class", Table: TableNeverAutoApprove, Claim: "aws / gcloud / az / terraform 的 delete、destroy 类破坏性云命令",
		Reason:   "只有 `terraform destroy` 的命令关键字落在**行首**（可表达）；aws / gcloud / az 的 delete 关键字在**命令中段**（如 `aws s3 rb`、`gcloud compute instances delete`）⇒ 现有语法只认行首前缀，表达不了。写 `ask bash(aws *)` 会把 `aws s3 ls` 这类只读命令一起拦（过度拦截）⇒ 不凑数。",
		Coverage: "只实体化了 never.cloud.terraform_destroy；其余待「命令中段关键字」形态（T5.4+ 语法扩展）后补。",
	},
}

// ── 装配 ────────────────────────────────────────────────────────────────────

// BuiltinFloor — 内置地板规则集：**唯一装配点**（两表合并，表①在前、表②在后，表内保持声明顺序）。
//
// 用法（接线批）：`NewEngine(userRules, append(BuiltinFloor(), orgFloor...))`。
// 同效果重复命中时取**先出现**的那条做 MatchedRule（T5.1 语义）⇒ 内置条目放在前面，
// 审计回显到的是内置 id（ID 非空 = 内置条目，空 = 文本规则）。
//
// 每次调用返回**新切片**：调用方改返回值不会污染下一次（返回值可变，内置数据不可变）。
func BuiltinFloor() []Rule {
	src := BuiltinFloorTable()
	out := make([]Rule, 0, len(src))
	for i, b := range src {
		out = append(out, Rule{
			ID:        b.ID,
			Effect:    b.Effect,
			Tool:      b.Tool,
			Specifier: b.Specifier,
			Kind:      classifySpec(b.Specifier),
			Floor:     true,
			Index:     i + 1,
			Raw:       builtinRaw(b),
		})
	}
	return out
}

// BuiltinFloorTable — 两表合并回显（带 id / 表 / 为何），供审计、UI、用例逐条点名。
func BuiltinFloorTable() []BuiltinRule {
	out := make([]BuiltinRule, 0, len(builtinAnyModeNoAuto)+len(builtinNeverAutoApprove))
	out = append(out, builtinAnyModeNoAuto...)
	out = append(out, builtinNeverAutoApprove...)
	return out
}

// BuiltinTableRules — 单表回显（表值非法 ⇒ nil，不猜）。
func BuiltinTableRules(t BuiltinTable) []BuiltinRule {
	var src []BuiltinRule
	switch t {
	case TableAnyModeNoAuto:
		src = builtinAnyModeNoAuto
	case TableNeverAutoApprove:
		src = builtinNeverAutoApprove
	default:
		return nil
	}
	out := make([]BuiltinRule, len(src))
	copy(out, src)
	return out
}

// BuiltinRuleByID — 按 id 取条目（审计从 Verdict.MatchedRule.ID 反查"为何"用）。
func BuiltinRuleByID(id string) (BuiltinRule, bool) {
	for _, b := range BuiltinFloorTable() {
		if b.ID == id {
			return b, true
		}
	}
	return BuiltinRule{}, false
}

// BuiltinPendingEntries — 未实体化条款回显（宁可不写，也不假装覆盖；见类型注释）。
func BuiltinPendingEntries() []BuiltinPending {
	out := make([]BuiltinPending, len(builtinPending))
	copy(out, builtinPending)
	return out
}

// builtinRaw — 条目的规则文本形态（进 Verdict 取证：审计要能像文本规则一样读到"是哪一条"）。
func builtinRaw(b BuiltinRule) string {
	if b.Specifier == "" {
		return string(b.Effect) + " " + b.Tool
	}
	return string(b.Effect) + " " + b.Tool + "(" + b.Specifier + ")"
}
