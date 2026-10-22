package main

// 命令表 · doctor 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_doctor() []*command {
	return []*command{
		// ---- 批 A · S2 只读面（§6.2 最小可验证集）：全程零写操作 ----
		{
			path:    []string{"doctor"},
			kind:    "DoctorCheck",
			summary: "环境自检（本机项 + 主控可达）· 逐项判定词",
			usage:   "zerg doctor [--json <字段>]",
			fields:  []string{"name", "verdict", "detail", "advice", "artifact_commit", "source_head", "bin_perms", "signature", "digest_dialect"},
			// ★ 缺口 GAP-20260927-200（批⑤ · 2026-10-13）：`credential_consistency`（凭据一致性判词）走**可选字段层**。
			optFields: []string{"credential_consistency"},
			endpoint:  "",
			run:       cmdDoctor,
		},
		{
			// ── 缺口 `GAP-20260926-194`：目标机只读体检入口（**不新开族** —— 落 `doctor` 名下的动作面）───
			// 真身 = `family_doctor_node.go:cmdDoctorNode`。只读：一条 ssh（本机那一台直接 `sh -s`）+ 只读命令；
			// 判定词沿用 doctor 五值闭集（PASS/FAIL/BLOCKED/REPORT），「未接」= REPORT（结构性缺席，只报告、不进退码）。
			// ★ 机名是**运行期数据**（`--machine <机>` · 缺省按本机名册名解析）：命令名 / kind / 件名一律中性，
			//   ✗ 不把任何具体机型名写进命令面（公开面不许有机型名字面量）。
			path:     []string{"doctor", "node"},
			kind:     "DoctorNode",
			summary:  "目标机只读体检（版本/sha · 单元 · 端口 · 模型/卵 · 磁盘 —— 一条只读探针 · ✗ 不写盘/不改服务/不提权；读不到的格照实标「未接」· 机名缺省取本机）",
			usage:    "zerg doctor node [--machine <机>]",
			arity:    "none",
			endpoint: "",
			opened:   true,
			run:      cmdDoctorNode,
		},
	}
}
