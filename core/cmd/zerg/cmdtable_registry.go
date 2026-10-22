package main

// 命令表 · registry 族（自 main.go 拆出 · 缺口 GAP-20261011-03「按族拆件」）
//
// 为什么拆：main.go 曾用单个 297 条的字面量承载全部命令（4272 行），
// 每次加一条命令都要开窗改同一个巨型文件 ⇒ 多路并发写同一件（现读 14 处未提交 hunk 并存）。
// 拆后：一条命令只动**本族一件**；装配在 main.go 的 init 里（顺序 = 首次出现序，✗ 依赖字面量顺序的代码不会变）。
//
// 纪律：本文件只放**命令表条目**（path/kind/summary/usage/fields/run…），不放实现逻辑。
func cmdtable_registry() []*command {
	return []*command{
		// ---- 登记面自动补面（`GAP-20261020-03` 下半 · Mr2109 拍「方案 B：一处登记，其余自动校验」）----
		// ★ 只补**能由件本身推出来**的那一面：`scripts/公开标记.tsv` 缺行 ⇒ 尾部追加一行
		//   （第 2 列缺省 `no`、第 3 列逐字「自动补登 · 待人工确认用途」）。
		// ★★ 硬约束：干跑语义 / 门步骤 / 契约矩阵**一律不补**（语义是判断不是事实 ——
		//   猜一条 = 凭空造语义；缺的 `.sh` 只在输出里列「需人写」）。实现件见 `family_registry_sync.go` 件头。
		{
			path:     []string{"registry", "sync"},
			kind:     "RegistrySync",
			summary:  "把**能由件本身推出来**的登记面补齐（只补 `scripts/公开标记.tsv` 缺行 · 第 2 列缺省 `no` · 第 3 列「自动补登 · 待人工确认用途」· 尾部追加 · 原子写 · `--dry-run` 零副作用且列出将改的行）；**语义面（干跑语义 / 门步骤 / 契约矩阵）一律✗不补**，`.sh` 缺干跑语义条目只在输出里点名「需人写」· 幂等（第二遍补 0 行）",
			usage:    "zerg registry sync [--dry-run] [--json <字段>]",
			arity:    "none",
			args:     []string{"（不收位置参数：仓根由 env `ZERG_REPO_ROOT` 或 cwd 上溯定）"},
			fields:   rsFields,
			endpoint: "",
			run:      cmdRegistrySync,
		},
	}
}
