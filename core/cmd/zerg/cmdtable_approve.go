package main

// 命令表 · approve 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_approve() []*command {
	return []*command{
		// ---- D3b 第四步（2026-09-21）：**人批通道**（`zerg approve` 一族 · §九 M18 `C4`②）----
		// 人签批准件 = `require_approval` 的逃生门；模型写得出件的字节，**写不出那枚签名**（口令只在人手里）。
		{
			path:      []string{"approve", "ls"},
			kind:      "Approval",
			summary:   "列人签批准件（逐件带上**验签判决**：验过 / 无签名 / 签名坏 —— 后两者不算批准）",
			usage:     "zerg approve ls [--json <字段>]",
			fields:    approveFields,
			optFields: approveOptionalFields,
			endpoint:  "",
			run:       cmdApprove,
		},
		{
			path:      []string{"approve", "show"},
			kind:      "ApprovalShow",
			summary:   "看一枚批准件的全貌 + 验签判决（消费者只认「验过」那一档）",
			usage:     "zerg approve show <工具名> [--json <字段>]",
			arity:     "any",
			args:      []string{"工具名"},
			fields:    approveFields,
			optFields: approveOptionalFields,
			endpoint:  "",
			run:       cmdApprove,
		},
		{
			path:    []string{"approve", "new"},
			kind:    "ApprovalSign",
			summary: "**人签**一枚批准件（要人在终端上敲口令；非交互会话一律拒）· 写不进即拒 · 同名不覆盖",
			usage:   "zerg approve new --tool <工具名> --by <人名> --note <理由> [--scope <范围>] [--self-test] [--json <字段>] [--dry-run | --confirm=<工具名> --yes]",
			arity:   "any",
			args:    []string{"工具名（--tool）", "人名（--by）"},
			fields:  approveNewFields,
			danger:  &dangerSpec{dangerD3, "工具名", "签一枚批准件（逃生门）—— 只作 require_approval 的放行凭据；人不在场时等于没签", "§九 M18 C4② · §17.6 SD7 · D3b 第四步", true, dangerScopeOutRepo},
			// `opened`: 真跑已开放（**人在终端上** + 操作员口令 ⇒ 签出批准件；非交互一律拒）。
			opened:   true,
			endpoint: "",
			run:      cmdApproveNew,
		},
		{
			path:     []string{"approve", "keygen"},
			danger:   &dangerSpec{dangerD3, "旧 key_id", "生成操作员密钥对：私钥（口令加密）＋ 公钥落盘；**换钥路**归档并 os.Remove 旧钥（旧件全废 · 不可逆）", "GAP-20261005-36 · 同族先例 approve new（D3）· §九 M18 C4②", false, dangerScopeOutRepo},
			opened:   true,
			kind:     "ApprovalKeygen",
			summary:  "生成**操作员密钥**（人在终端上设口令；私钥口令加密落盘，公钥给消费者验签）",
			usage:    "zerg approve keygen --by <人名> --yes [--dry-run] [--confirm=<旧 key_id>]",
			arity:    "any",
			args:     []string{"人名（--by）"},
			endpoint: "",
			run:      cmdApproveKeygen,
		},
		// ---- GAP-20261001-76：**撤销/覆盖正门**（`zerg approve rotate` · 同族先例 approve new（D3））----
		// 病征：批准件没有撤销/覆盖/轮换正门，`new` 同名不覆盖 ⇒ 换 scope 只能手改家目录件。
		// 治法：一条正门 —— 给 `--scope` = 覆盖/轮换（旧件退位 mv + 人签新件）；不给 = 撤销（只退位）。
		{
			path:    []string{"approve", "rotate"},
			kind:    "ApprovalRotate",
			summary: "**撤销/覆盖**一枚批准件（给 `--scope` = 换范围（旧件**退位**（mv 不删）+ 人签新件）；不给 = 撤销（只退位））· 要人在终端上敲 · D3",
			usage:   "zerg approve rotate <工具名> --by <人名> [--scope <新范围>] [--note <理由>] [--dry-run | --confirm=<工具名> --yes]",
			arity:   "any",
			args:    []string{"工具名（位置参数）", "人名（--by）"},
			danger:  &dangerSpec{dangerD3, "工具名", "撤销/覆盖一枚批准件（逃生门）—— 旧件**退位**（mv 进 retired/ · ✗ 不删）+ 覆盖档人签新件；不可逆，人不在场等于没动", "GAP-20261001-76 · 同族先例 approve new（D3）· §九 M18 C4②", true, dangerScopeOutRepo},
			// `opened`: 真跑已开放（**人在终端上** + 操作员口令 ⇒ 覆盖签新件；非交互一律拒）。
			opened:   true,
			endpoint: "",
			run:      cmdApproveRotate,
		},
	}
}
