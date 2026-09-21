// family_impact.go —— 变更影响面（`zerg impact <目标>`）· `A1` 骨架 + **`A2` 六层取数**。
// （设计-变更影响面-v1.6 §7.1/§7.3/§7.4 · §二 六层与层规 · 任务单-影响面实施-20260922 §二 `A2`）。
//
// 本件的范围（逐字照任务单）：
//
//	· `A1` 已交：**只读**骨架 —— 人面三行 + 六键包封（`items` 恒数组 · `[]` 永不为 `null`）·
//	  零命中退 `1`（不是 `0`、不是失败）· 目标解析不到退 `2` · 契约登记表读不到退 `8`。
//	· `A2` 本件：把 §二 的**六层接成一条链**（① 编译器 → ② 符号 → ③ 契约 → ④ 词法 + 形近 →
//	  ⑤ 语义（按需）→ ⑥ 公开面）—— 实现件在 `family_impact_layers.go`，本件只**接线 + 出数**：
//	  人面三行给**真数**（不再是骨架期的三个 0）· 第 ⑥ 层命中时**多一行**条件行（§3.1/§3.7）·
//	  stderr 恒出一张**层表**（每层带 `head_sha` + `layer` + 该层口径值 + 粒度 + 时刻 + 耗时）。
//	· 仍**不在**本件：卡片与四级排序裁序的**实现**在 `family_impact_card.go`（`A3`）· 挂进
//	  `dev edit` 干跑（`A4`）· **落盘缓存与毫秒档**在 `family_impact_cache.go`（`A5`：本件只把
//	  缓存挂到链上（`impactCacheOn`）并把落点/毫秒档打进 stderr 那一块）。
//
// 档位（§4.4 · `A2` 风险那一条）：**默认档只吃毫秒层 + 编译器层**；贵层（② 符号 `callgraph`
// 现跑 3.9–5.8s）走**按需档**。开关用**既有全局布尔** `--all`（与 `build show --all` 同形：
// 「全都要」）—— §7.1 的 `--depth` 一类**旗标名待 `R32`/B4 拍板**，故本批**不新造名字**，
// 只把两档机制落下来；`--quick`（`P-040` 贵项跳过）与默认档同效，给了也照实明说。
// 跳过的层在层表里写「未跑」⇒ **不悄悄少给几层**（§4.4）。
//
// 三条判据的落点（任务单 §二 `A1` 判据 ①–③ · 一字不改）：
//
//	① `--json` 里**六键恒在**（`schema`/`kind`/`items`/`meta`/`warnings`/`truncated`）——
//	   出口是唯一的既有实现 `emitSelected`；本件**不改 `emitEnvelope` 的注释与语义** ✗。
//	② `items` 空时为 `[]`、**永不为 `null`**；退码成对：**零命中 ⇒ `1`** ·
//	   **目标解析不到 / 出仓 / 缺目标 ⇒ `2`** · **契约登记表读不到 ⇒ `8`**（「读不到」不许当「没有」）。
//	③ 人面**恒三行**、字头与顺序固定：`会牵动：` / `会红：` / `建议：`（§7.3）；
//	   **命中生效面时多带一行** `公开面：`（§3.1「三行是常量，这三条是条件行：没命中就不打」）。
//
// 诚实边界（§十一 · 失败模式 `F6`「把『没报』读成『没影响』」）：层表**逐层点名**状态
// （取值 / 未跑 / 未适用 / 未装 / 未建索引 / 读不到）—— 一个 0 若是**没跑**来的，层表里会写明。
//
// 红线（任务单 §二 `A2` · 逐条）：不引 `gopls` / `rust-analyzer` ✗ · 不加 `go.work` ✗ ·
// 不动 `publish/` 那七件生效面里的任何一行 ✗ · 不拿私有树件数冒充产出树件数 ✗ ·
// 不为凑绿放宽任何既有判据 ✗ · 不写缓存 / 不落审计（属 `A5`/`A4`）· 不改 `emitEnvelope*` ✗
// （`A1` 红线继续守）· 不动 `zerg code find` 的扫码口径与排除表 ✗。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// impactFields —— `zerg impact --json <字段>` 的字段表（= §3.1 的条目四字段；K1：机器面先定）。
// `A2` **不动这张表**（§九 M6 `R7` 字段只增不改 —— 本批不需要增：层规三件落在人面条件行与
// stderr 层表上，`items` 仍是四字段条目；`why` 取值扩到六选一由层名承载）。
var impactFields = []string{"what", "why", "how", "red"}

// impactLineHead —— 人面三行的**固定字头 · 固定顺序**（判据③；与 §3.1 的三行骨架同一份，不另立）。
var impactLineHead = [3]string{"会牵动：", "会红：", "建议："}

// 目标两态（§7.2 四态里的前两态；`R2` 已拍「先只收 文件 + 契约 id」——符号态与提案态另拍）。
const (
	impactKindFile     = "file"
	impactKindContract = "contract"
)

// impactContractIDRe —— 契约 id 的形状（`registry.json` 的 `entries[].id`：`S-a` 一族）。
// 只用来决定「要不要去读契约登记表」——**形状像 ≠ 在册**，在不在册一律以表为准。
var impactContractIDRe = regexp.MustCompile(`^[A-Za-z]-[a-z0-9]+$`)

// impactRegistryRel —— 契约登记表（§二 第 ③ 层那条真源 · **只读**；本件不另抄一份 id 清单）。
const impactRegistryRel = "core/internal/contract/registry.json"

// impactTarget —— 解析出来的目标（`Raw` 是用户给的原样串，`Rel`/`ID` 按态各有一个）。
type impactTarget struct {
	Kind string // impactKindFile / impactKindContract
	Raw  string
	Rel  string // 件目标：仓内相对路径（slash）
	ID   string // 契约目标：契约 id
}

// cmdImpact —— `zerg impact <目标>`：`A1` 骨架 + `A2` 六层取数 + `A3` 波纹卡片
// （只读 · 零副作用 · 不写缓存 · 不落审计）。
func cmdImpact(inv *invocation, stdout, stderr io.Writer) int {
	raw := ""
	if len(inv.args) > 0 {
		raw = strings.TrimSpace(inv.args[0])
	}
	if raw == "" {
		inv.setErr("usage", "missing_target", "缺目标")
		fmt.Fprintf(stderr, "%s: `impact` 要给一枚目标（件 · 或契约 id —— §7.2 前两态，`R2` 已拍）\n", progName)
		fmt.Fprintf(stderr, "用法：%s\n", impactUsageLine)
		fmt.Fprintf(stderr, "下一步：件目标给仓内相对路径（例 `core/cmd/zerg/family_code.go`）；契约目标给在册 id（例 `S-g`）\n")
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 取不了数（不给结论 · 退码 8）\n", progName)
		fmt.Fprintf(stderr, "在仓内跑，或设 ZERG_REPO=<仓根>\n")
		return exitBlocked
	}
	tgt, why := impactResolve(root, raw)
	if tgt == nil {
		return impactReject(inv, stderr, root, raw, why)
	}
	// K2 先判（`--json` 不给字段 ⇒ 1 + stdout 0 字节）：**取数之前**判 —— 否则白跑六层。
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail
		}
		// 字段名也**先判**（未知字段 ⇒ 2 + 列合法字段）：判据一字不改（仍是 §九 M6 I5 的
		// 「点名字段」面），只把它挪到取数之前 —— 六层现跑读秒级，别为一条打错字的字段白跑一遍。
		for _, f := range inv.fields {
			ok := false
			for _, l := range fieldListOf(inv.path) {
				if l == f {
					ok = true
					break
				}
			}
			if !ok {
				return reportBadField(stderr, inv.path, f)
			}
		}
	}
	// 档位（§4.4）：默认档 = 毫秒层 + 编译器层；贵层（② 符号层）走按需档 `--all`。
	// `A5`：链上挂**落盘缓存**（`impactCacheOn` —— 读 + 写；落点与键从契约件读）。
	cheap := !inv.all || inv.quick
	layers := impactPullLayers(root, tgt, cheap, impactCacheOn)
	rows, totalRows := impactCollectRows(layers)
	// `A3` 波纹卡片：先把骨架三行（+ 条件行）与退法算出来 —— 卡片预算要把**常量部分**也算进去
	// （§4.1 的账：上限是死的，裁的是条目，不是骨架）。
	rev := impactReversibilityOf(root, tgt)
	pubLine := impactPublicLine(layers)
	l1, l2, l3 := impactHumanLines(tgt, layers, rows, rev, cheap)
	card := impactCardOf(rows, l1+"\n"+l2+"\n"+l3+"\n"+pubLine, tgt.Raw)
	// ① 人面：**恒三行**（判据③）+ 命中生效面时的条件行（§3.1/§3.7）。`--json` 时不打人面。
	if !inv.jsonGiven {
		fmt.Fprintln(stdout, l1)
		fmt.Fprintln(stdout, l2)
		fmt.Fprintln(stdout, l3)
		if pubLine != "" {
			fmt.Fprintln(stdout, pubLine)
		}
	}
	// ② stderr：层表（每层带 head_sha + layer + 该层口径值 + 粒度 + 时刻 + 耗时）+ 卡片块 + 时效声明。
	emitImpactLayerTable(stderr, tgt, layers, card.Items, totalRows, card, cheap)
	emitImpactCacheBlock(stderr, root, layers)
	emitImpactCardBlock(stderr, tgt, card, rev, layers, inv.forHuman)
	if inv.forModel && !inv.forHuman {
		fmt.Fprintf(stderr, "%s: `--for-model` 与默认档**同效**（§4.1：模型档就是默认档）—— 给了也照实明说，不另开一条分叉\n", progName)
	}
	if inv.forModel && inv.forHuman {
		fmt.Fprintf(stderr, "%s: `--for-model` 与 `--for-human` 同时给了 ⇒ 以**人面档**为准（模型档是人面档的子集 · §4.1；照实明说，不静默挑一个）\n", progName)
	}
	// ③ 机器面：六键包封（`items` 恒数组；本件只换里面装的东西 —— 换成**被预算裁过的卡片条目**，不改包封）。
	if inv.jsonGiven {
		if rc := emitSelected(stdout, stderr, inv, inv.path, inv.fields, card.Items); rc != exitOK {
			return rc
		}
	}
	// ④ 退码：六层取到东西 ⇒ 0；六层都没给出条目 ⇒ **零命中**（§7.5 的「无影响面」= 1）。
	// 口径（`A3` 写死）：看的是**六层全量条数**（裁前的那个数），不是裁后的卡片条数 ——
	// 卡片被裁空不等于「没影响面」（§3.3：裁了必须显式声明，退码不许被裁序改写）。
	if totalRows == 0 {
		return exitFail
	}
	return exitOK
}

// impactCollectRows 收六层的条目（**全量** · 不在这里截页）：条数上限归卡片（§3.1 ≤ 12 条），
// 层内的每层上限仍归各层自己（`impactRowPage`）。
func impactCollectRows(layers []impactLayer) ([]map[string]string, int) {
	rows := []map[string]string{}
	for _, l := range layers {
		rows = append(rows, l.Rows...)
	}
	return rows, len(rows)
}

// impactLayerBySeq 取某一层（浅拷贝遍历；层序 = §二 的表序）。
func impactLayerBySeq(layers []impactLayer, seq string) (impactLayer, bool) {
	for _, l := range layers {
		if l.Seq == seq {
			return l, true
		}
	}
	return impactLayer{}, false
}

// impactReject —— 目标被拒（**两档分开** · §7.5：「读不到」不许混成「没有」）。
func impactReject(inv *invocation, stderr io.Writer, root, raw, why string) int {
	switch why {
	case "registry_unreadable":
		inv.setErr("blocked", "registry_unreadable", "契约登记表读不到")
		fmt.Fprintf(stderr, "%s: 契约登记表读不到（%s）⇒ 判不了「这条契约在不在册」（不给结论 · 退码 8）\n",
			progName, impactRegistryRel)
		fmt.Fprintf(stderr, "「读不到」不当「没有」：先确认件在盘上、可读（%s）\n", filepath.Join(root, impactRegistryRel))
		return exitBlocked
	case "outside_repo":
		inv.setErr("usage", "path_outside_repo", "目标出仓")
		fmt.Fprintf(stderr, "%s: 目标出仓了（%s 不在 %s 下）⇒ 退码 2（**按路径段判**，不按字符串前缀 —— ⑦ CLI 缺口一栗的教训）\n",
			progName, raw, root)
		return exitUsage
	default: // not_found
		inv.setErr("usage", "target_unresolved", "目标解析不到")
		fmt.Fprintf(stderr, "%s: 目标解析不到：%s —— 它既不是仓内件、也不是在册契约 id ⇒ 退码 2（不给结论）\n",
			progName, raw)
		if ids, err := impactRegistryIDs(root); err == nil {
			fmt.Fprintf(stderr, "在册契约 id（%d 条）：%s\n", len(ids), strings.Join(ids, ","))
		}
		fmt.Fprintf(stderr, "件目标要给**仓内相对**路径（例 `core/cmd/zerg/main.go`）；契约目标是 `S-a` 一类 id\n")
		return exitUsage
	}
}

// impactResolve 把用户给的目标解析成**前两态**之一（§7.2：优先级即顺序）。
//
// 返回 `(nil, why)` 时 why ∈ {registry_unreadable, outside_repo, not_found} —— **三档不许混**
// （「不给结论」与「用法错」是两种码）：
//
//	· 形状像契约 id ⇒ 去读登记表；**表读不到 ⇒ registry_unreadable（退 8）** —— 不许把读不到读成没有；
//	· 件目标：出仓判据按**路径段**（`filepath.Rel` + `..` 前缀），**不按字符串前缀** ——
//	  `zerg code find --path` 那一栗（`Zerg-内部文档` 以 `Zerg` 开头就穿出去了）不在本命令重演；
//	· 两态都不成立 ⇒ not_found（退 2）。
func impactResolve(root, raw string) (*impactTarget, string) {
	if impactContractIDRe.MatchString(raw) {
		ids, err := impactRegistryIDs(root)
		if err != nil {
			return nil, "registry_unreadable"
		}
		for _, id := range ids {
			if id == raw {
				return &impactTarget{Kind: impactKindContract, Raw: raw, ID: raw}, ""
			}
		}
		return nil, "not_found"
	}
	if filepath.IsAbs(raw) {
		return nil, "outside_repo"
	}
	rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(raw)))
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, "outside_repo"
	}
	if rel == "." || rel == "" {
		return nil, "not_found"
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		return nil, "not_found"
	}
	return &impactTarget{Kind: impactKindFile, Raw: raw, Rel: rel}, ""
}

// impactRegistryIDs 读契约登记表的 id 列（**只读真源**；`entries[].id` 一条都没有 ⇒ 报错，
// 因为「空表」与「读不到」都不许被读成「一条都不在册」）。
func impactRegistryIDs(root string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(impactRegistryRel)))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Entries []struct {
			ID string `json:"id"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	ids := []string{}
	for _, e := range doc.Entries {
		if strings.TrimSpace(e.ID) != "" {
			ids = append(ids, e.ID)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s 的 entries[].id 一条都没有（空表不许当「都不在册」）", impactRegistryRel)
	}
	sort.Strings(ids)
	return ids, nil
}

// impactUsageLine —— 形态串（与命令树的 `usage` 逐字同源，门⑫ 的口径；本件不旁写第二份）。
// `A3` 起把 §4.1 的**两档**写进形态串（`--for-model` / `--for-human` · `R9` 已拍「分两档」·
// `R38` 拍定它们与 `--json` **不是同一条**、可叠加、都不改六键包封）。
// ★ `--all`（按需档 · §4.4）仍是**既有全局布尔**，故不写进形态串（形态串只写本命令独有的东西）。
const impactUsageLine = "zerg impact <文件｜契约 id> [--for-model｜--for-human] [--json <字段>]"

// impactHumanLines —— 人面**恒三行**（判据③：字头固定 · 顺序固定）+ `A3` 把 §3.8 的可逆性
// **内联在第③行同一行**（§4.1：可逆性行内联在第③行内 · **不新增行数**，所以人面仍恒三行）。
//
// 三个数的口径**写在这里**（免得被当成同一个分母）：受影响包 = ① 反向包数 · 文件 = 六层条目里
// 带件路径的条目**去重后的件数**（② 符号级条目不含件路径 ⇒ 不计入，层表里明写）· 契约 = ③ 命中条数。
//
// ★ `A3` 改了一处口径：**文件数取「六层全量」**（裁前），不是卡片里那 12 条 —— 卡片被裁是
// **预算**的结果，拿它当「受影响文件数」会把一个大面报成小面（数字纪律：口径不同不同源）。
func impactHumanLines(tgt *impactTarget, layers []impactLayer, rows []map[string]string,
	rev impactReversibility, cheap bool) (string, string, string) {
	pkgs := 0
	if l, ok := impactLayerBySeq(layers, "①"); ok {
		pkgs = len(l.Rows)
	}
	files := impactDistinctFiles(rows)
	contracts := 0
	if l, ok := impactLayerBySeq(layers, "③"); ok {
		contracts = len(l.Rows)
	}
	pkgStr := fmt.Sprintf("%d 个", pkgs)
	if tgt.Kind != impactKindFile {
		pkgStr = "未适用（契约目标无编译面）"
	}
	l1 := fmt.Sprintf("%s受影响包 %s · 文件 %d 个 · 契约 %d 条（文件数=六层全量去重 · 卡片裁序不改这个数）",
		impactLineHead[0], pkgStr, files, contracts)
	l2 := impactLineHead[1] + impactRedLine(layers, cheap)
	// §3.8：**建议**行内、给出下一步命令的**同一行**必带可逆性（能退吗 / 退法是哪条命令）。
	l3 := impactLineHead[2] + strings.Join(impactSuggestions(tgt, layers, cheap), " · ") + " · " + rev.Line
	return l1, l2, l3
}

// impactDistinctFiles 条目里的**件数**（去重）；② 符号级条目不含件路径 ⇒ 不计入（层表里写明）。
func impactDistinctFiles(items []map[string]string) int {
	set := map[string]bool{}
	for _, r := range items {
		w := r["what"]
		if i := strings.Index(w, ":"); i > 0 {
			prefix, rest := w[:i], w[i+1:]
			if prefix == "文件级" || prefix == "件级" || prefix == "名字级" {
				p := rest
				if j := strings.LastIndex(rest, ":"); j > 0 {
					if _, err := strconv.Atoi(rest[j+1:]); err == nil {
						p = rest[:j]
					}
				}
				set[p] = true
			}
		}
	}
	return len(set)
}

// impactRedLine 「会红」那一行（闭集：契约 id + 门步名；门步名映射属 `B1` ⇒ **本件不真跑门禁** §7.8）。
//
// ★ §3.5 铁律（`A3` 落成可判的形态）：**语义级条目只进「建议」行、永不进「会红」行** ——
// 义近（概率）条目即使带着 `red` 也不许进这一行；`why` 归一到六选一后按 `义近` 逐条挡掉。
func impactRedLine(layers []impactLayer, cheap bool) string {
	reds := []string{}
	seen := map[string]bool{}
	for _, l := range layers {
		for _, r := range l.Rows {
			if strings.TrimSpace(r["red"]) == "" || impactWhyNormalize(r["why"]) == "义近" {
				continue
			}
			if !seen[r["red"]] {
				seen[r["red"]] = true
				reds = append(reds, r["red"])
			}
		}
	}
	parts := []string{}
	if len(reds) > 0 {
		sort.Strings(reds)
		parts = append(parts, "契约 "+strings.Join(reds, ",")+"（`change_class=B` —— 改它会破承诺）")
	} else {
		parts = append(parts, "没命中契约条目（③ 层现读 `registry.json`）")
	}
	parts = append(parts, "门步名映射属 `B1`（未接 ⇒ 本行**不真跑门禁** · §7.8 只预测不真跑）")
	if l, ok := impactLayerBySeq(layers, "①"); ok && strings.Contains(l.Detail, "编译面已红") {
		parts = append(parts, "★ 编译面现跑已红（看层表 ① 的读数）")
	}
	if cheap {
		parts = append(parts, "② 符号层在默认档**未跑**（要看调用者：`--all`）")
	}
	return strings.Join(parts, " · ")
}

// impactSuggestions —— 「建议」行的候选命令（§7.3：**最多 3 条**；只给**今天真能敲**的）。
func impactSuggestions(tgt *impactTarget, layers []impactLayer, cheap bool) []string {
	pat, dir := "", "core"
	if tgt.Kind == impactKindContract {
		pat = regexp.QuoteMeta(tgt.ID)
	} else {
		pat = regexp.QuoteMeta(filepath.Base(tgt.Rel))
		if d := filepath.ToSlash(filepath.Dir(tgt.Rel)); d != "" {
			dir = d
		}
	}
	out := []string{
		fmt.Sprintf("zerg code find '%s' --path %s（词法面复算 · 全盘口径）", pat, dir),
		"zerg gate run --fast（门面 · 真跑属 `B1`）",
	}
	if cheap {
		out = append(out, fmt.Sprintf("zerg impact %s --all（按需档 · 补 ② 符号层）", tgt.Raw))
	} else {
		out = append(out, "zerg gate run --fast（同上 · 复核）")
	}
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

// emitImpactLayerTable —— stderr 的**层表**（`A2` 判据①的落点）：
// 每层一行，**必带 `layer=` + `口径=` + `head_sha=`** 三件（② 层的 `口径` 里带 `algo=`），
// 另附粒度四档之一（`R40`：四档不可相加）、状态（取值 / 未跑 / 未适用 / 未装 / 未建索引 / 读不到）、
// 该层现跑读数与耗时；末尾恒带时效声明（§3.1「结果随仓变而变」）。
func emitImpactLayerTable(stderr io.Writer, tgt *impactTarget, layers []impactLayer,
	items []map[string]string, totalRows int, card impactCard, cheap bool) {
	fmt.Fprintf(stderr, "%s: `A2` 六层取数（层规三件 = head_sha + layer + 该层口径值；② 层必带 algo）\n", progName)
	head, at := "", ""
	for _, l := range layers {
		if head == "" {
			head, at = l.HeadSHA, l.At
		}
		cost := "—"
		if l.Cost > 0 {
			cost = l.Cost.Round(time.Millisecond).String()
		}
		fmt.Fprintf(stderr, "  层%s layer=%s%s 粒度=%s 口径=%s 状态=%s 耗时=%s head_sha=%s 时刻=%s\n",
			l.Seq, l.Seq, l.Name, l.Grane, l.Caliber, l.Status, cost, dashIfEmpty(l.HeadSHA), l.At)
		fmt.Fprintf(stderr, "    读数：%s\n", l.Detail)
		// `A5`：这一层这一跑是**命中落盘件**还是**现算**（独立一行 ⇒ `A2` 判据① 钉住的层表六行
		// 格式一个字不动；两条行的字头不同，判据正则只认 `  层…layer=` 那一种）。
		fmt.Fprintf(stderr, "    缓存：%s\n", dashIfEmpty(l.CacheNote))
	}
	// 数字纪律：`head_sha` 取不到 ⇒ 该结果**只许当参考**（§7.4 判据：时效三件缺任一 ⇒ 只许当参考）。
	if head == "" {
		fmt.Fprintf(stderr, "%s: ★ 层规缺 `head_sha`（取不到）⇒ 本结果**只许当「参考」，不许当判据**（§7.4）\n", progName)
	} else {
		fmt.Fprintf(stderr, "%s: 结果随仓变而变：本结果算的是 head_sha=%s 那一刻的仓（取数时刻 %s）\n", progName, head, at)
	}
	// 条数口径（`A3` 改写）：上限归**卡片**（≤ 12 条 · §3.1），层内每层上限仍是 `impactRowPage`；
	// 裁的结果在卡片块里显式声明（三件），**不静默截**。
	fmt.Fprintf(stderr, "%s: 条目：卡片 `items` %d 条（六层全量 %d 条 · 卡片上限 %d 条 · 层内上限 %d/层 · token 上限 %d —— §3.1 两个上限都是死的；本跑 truncated=%t）\n",
		progName, len(items), totalRows, impactItemMax, impactRowPage, impactTokenMax, card.Truncated)
	fmt.Fprintf(stderr, "%s: ★ CLI 缺口（照实标）：§3.3 的三件（`truncated=true` / `warnings[]` / `meta.how_to_restore`）在**六键包封里没有落点** —— `emitEnvelopeWith` 把 `warnings` 恒写 `[]`、`truncated` 恒写 `false`、`meta` 只写 `count/source/changed`；`A1`/`A2` 红线「不改 `emitEnvelope*`」本批未解禁 ⇒ 三件落在**卡片块**（同一份取值，逐字同形），包封那一格照实记缺口（不偷偷改包封）\n", progName)
	if cheap {
		fmt.Fprintf(stderr, "%s: 档位 = **默认档**（§4.4「只吃毫秒层 + 编译器层」）—— 没跑的层已在上面逐条点名（宁少报不猜报）；要看 ② 符号层给 `--all`\n", progName)
	}
	fmt.Fprintf(stderr, "%s: 退码口径（§7.5）：0 = 有影响面 · 1 = 无影响面（零命中 · **不是错**）· 2 = 用法错/目标解析不到 · 8 = 读不到（不给结论）\n", progName)
}

// timeMillis 只为人面好看（耗时的显示精度）。
const timeMillis = 1e6

// emitImpactCacheBlock —— `A5` 的**落点与毫秒档**那一块（stderr · 人读；`A2` 的层表一个字不动）。
//
// 打三件（判据的可读面）：
//
//	① **落点**：缓存与索引的绝对路径（目录名从契约件读）+ 契约件路径 + 不在仓内/不进公开面那条纪律；
//	② **本跑账**：三层里命中几层、现算几层、不适用几层；本跑最长一次「读落盘产物」= 多少 ms
//	   （§4.4 的毫秒档判据 = ≤ 4 ms，值现跑到小数点后两位，不四舍五入成整数）；
//	③ **纪律**（照实打，不靠自觉）：只增不改不删 · 不接自动收窄/自动删/自动回滚 · 缓存不许改答案
//	   （命中与未命中逐字同输出）。
func emitImpactCacheBlock(w io.Writer, root string, layers []impactLayer) {
	fmt.Fprintf(w, "%s: `A5` 落盘缓存与毫秒档（§4.4：键 = head_sha + 层 + 该层口径值；毫秒档 = 命中一次 ≤ %.2f ms）\n",
		progName, impactMillisecondTierMS)
	if lay, err := impactStateLayoutOf(root); err != nil {
		fmt.Fprintf(w, "  落点：契约件 `%s` **读不到**（%v）⇒ 本跑不读不写缓存（不给结论、不猜目录名）\n",
			impactStateContractRel, err)
	} else {
		fmt.Fprintf(w, "  落点（契约件 `%s` · schema=%s）：缓存 %s · 索引 %s\n",
			impactStateContractRel, lay.Schema, filepath.Join(stateDirOf(), lay.CacheDir), impactCacheIndexDir(lay))
		fmt.Fprintf(w, "  ★ 两个落点都在**状态目录**下（`ZERG_STATE_DIR` > `~/.zerg/state`）⇒ **不在仓内** ⇒ 不进公开面、不进 git 跟踪\n")
	}
	hit, built, na := 0, 0, 0
	worst, worstLayer := 0.0, ""
	for _, l := range layers {
		switch {
		case l.CacheHit:
			hit++
			ms := float64(l.CacheCost.Microseconds()) / 1000.0
			if ms > worst {
				worst, worstLayer = ms, l.Seq
			}
		case l.CacheNote == "不适用（本层不进缓存 —— 见契约件 `cache.not_cacheable`）":
			na++
		default:
			built++
		}
	}
	if hit > 0 {
		fmt.Fprintf(w, "  本跑账：命中 %d 层 · 现算 %d 层 · 不适用 %d 层；**最长一次「读落盘产物」= %.2f ms**（层%s · 判据上限 %.2f ms ⇒ %s）\n",
			hit, built, na, worst, worstLayer, impactMillisecondTierMS, yesno(worst <= impactMillisecondTierMS))
	} else {
		fmt.Fprintf(w, "  本跑账：命中 0 层 · 现算 %d 层 · 不适用 %d 层（本跑没有可比的「读落盘产物」读数 ⇒ 毫秒档**本跑未测**，不当 0 看）\n",
			built, na)
	}
	fmt.Fprintf(w, "  纪律（照实打）：缓存**只增不改不删**（本件里没有删除动作）· **不接**自动收窄 / 自动删 / 自动回滚 · "+
		"**不许在内存里缓存跨命令复用** · 命中与未命中**逐字同输出**（`M8`）· 缓存**不抬任何上限**（条数 ≤ %d / token ≤ %d 一个字没动）\n",
		impactItemMax, impactTokenMax)
}

func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "（取不到）"
	}
	return s
}
