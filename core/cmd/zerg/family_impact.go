// family_impact.go —— 变更影响面（`zerg impact <目标>`）· **`A1` 骨架**
// （设计-变更影响面-v1.6 §7.1/§7.3/§7.4 · 任务单-影响面实施-20260922 §二 `A1`）。
//
// `A1` 的范围（**逐字照任务单**）：「把 `建议 zerg impact <目标>` 的**只读**骨架立起来 ——
// 人面三行 + 六键包封（`items` 恒数组 · **不新增第七键**）」。⇒ 本件**只出命令与包封**：
// 六层取数（① 编译器 → ② 符号 → ③ 契约 → ④ 词法/形近 → ⑤ 语义 → ⑥ 公开面）属 `A2`、
// 卡片与四级排序属 `A3`、挂进改码干跑属 `A4`、落盘缓存与毫秒档属 `A5` —— **一个都不在本件里做** ✗。
//
// 三条判据的落点（任务单 §二 `A1` 判据 ①–③）：
//
//	① `--json` 里**六键恒在**（`schema`/`kind`/`items`/`meta`/`warnings`/`truncated`）——
//	   出口是唯一的既有实现 `emitSelected`；本件**不改 `emitEnvelope` 的注释与语义** ✗。
//	② `items` 空时为 `[]`、**永不为 `null`**（`emitSelected` 的 `[]` 分支）；
//	   退码成对：**零命中 ⇒ `1`**（§7.5「无影响面」· 不是 `0`、不是失败）·
//	   **目标解析不到 / 出仓 / 缺目标 ⇒ `2`**（用法错 ⇒ **不给结论**）·
//	   **契约登记表读不到 ⇒ `8`**（「读不到」不许当「没有」）。
//	③ 人面**恒三行**、字头与顺序固定：`会牵动：` / `会红：` / `建议：`（§7.3）。
//
// 诚实边界（§十一 · 失败模式 `F6`「把『没报』读成『没影响』」）：本件**没接任何取数层** ⇒
// 三行里的三个 0 是**未取数**，不是「面内未见」—— 这句必须**每次跑都打出来**（stderr），
// 否则一个恒返回空集的命令会被读成「这一改没事」（那正是「零引用被当结论用」的同族病）。
// ★ 已知缺口（照实记）：骨架期机器面**自述不了**「未取数」——`meta.layers_not_run[]` 属 `A2`/`A5`，
// 而本件不许改 `emitEnvelope*` ⇒ 机器侧只有 `rc=1` 一枚码 + `items` 空数组；
// 要在机器面分辨「未取数」与「面内未见」，等 `A2` 把层规接上。
//
// 红线（任务单 §二 `A1` 红线 · 逐条）：不新增第七键 ✗ · 不改 `emitEnvelope` 注释与语义 ✗ ·
// 不动 `zerg code find` 的扫码口径与排除表 ✗ · **不写缓存 / 不落审计** ✗（本任务只出命令与包封）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// impactFields —— `zerg impact --json <字段>` 的字段表（= §3.1 的条目四字段；K1：机器面先定）。
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

// cmdImpact —— `zerg impact <目标>`：`A1` 骨架（只读 · 零副作用 · 不写缓存 · 不落审计）。
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
	// ① 人面：**恒三行**（判据③）。`--json` 时不打人面（机器面只留包封）。
	if !inv.jsonGiven {
		emitImpactSkeleton(stdout, stderr, tgt)
	} else {
		emitImpactDisclaimer(stderr)
	}
	// ② 机器面：六键包封（`items` 恒数组 · 骨架期恒 `[]`）。K2：给了 `--json` 不给字段 ⇒ 1 + stdout 0 字节。
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail
		}
		if rc := emitSelected(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{}); rc != exitOK {
			return rc
		}
	}
	// ③ 退码：骨架期凡目标有效 ⇒ **零命中**（§7.5 的「无影响面」= `1`，不是 `0`、不是失败）。
	return exitFail
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
const impactUsageLine = "zerg impact <文件｜契约 id> [--json <字段>]"

// emitImpactSkeleton —— 人面**恒三行**（判据③：字头固定 · 顺序固定）+ stderr 一条诚实声明。
func emitImpactSkeleton(stdout, stderr io.Writer, tgt *impactTarget) {
	fmt.Fprintf(stdout, "%s受影响包 0 个 · 文件 0 个 · 契约 0 条（`A1` 骨架：六层未取数 —— 这三个 0 是**未取数**，不是「面内未见」）\n",
		impactLineHead[0])
	fmt.Fprintf(stdout, "%s（未取数：门禁步名映射属 `B1` · 反向包与契约指向属 `A2`；本行**不真跑**门禁 · §7.8）\n",
		impactLineHead[1])
	fmt.Fprintf(stdout, "%s%s\n", impactLineHead[2], strings.Join(impactSuggestions(tgt), " · "))
	emitImpactDisclaimer(stderr)
}

// emitImpactDisclaimer —— 诚实声明（§十一 · `F6`）：**每次跑都要说清「未取数」**。
func emitImpactDisclaimer(stderr io.Writer) {
	fmt.Fprintf(stderr, "%s: `A1` 骨架 —— 六层取数**一个都没接**（属 `A2`）⇒ 三行里的 0 是**未取数**，不是「面内未见」；"+
		"「没报」不许读成「没事」（§十一 诚实边界）\n", progName)
	fmt.Fprintf(stderr, "%s: 退码口径（§7.5）：1 = 无影响面（零命中 · **不是错**）· 2 = 用法错/目标解析不到 · 8 = 读不到（不给结论）\n",
		progName)
}

// impactSuggestions —— 「建议」行的候选命令（§7.3：**最多 3 条**；只给**今天真能敲**的那两条）。
//
// 为什么不给「反向包 / 符号边 / 步名」这三条：它们正是 `A2`/`B1` 要接的面 —— 现在写出来就是
// 给了跑不通的命令（`P-0xx` 那条纪律：不发件不留跑不通的命令 · 与 `setup-zerg.sh` 的教训同族）。
func impactSuggestions(tgt *impactTarget) []string {
	pat, dir := "", "core"
	if tgt.Kind == impactKindContract {
		pat = regexp.QuoteMeta(tgt.ID)
	} else {
		pat = regexp.QuoteMeta(filepath.Base(tgt.Rel))
		if d := filepath.ToSlash(filepath.Dir(tgt.Rel)); d != "" {
			dir = d
		}
	}
	return []string{
		fmt.Sprintf("zerg code find '%s' --path %s（词法面 · 属 `A2`）", pat, dir),
		"zerg gate run --fast（门面 · 属 `B1`）",
	}
}
