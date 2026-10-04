// family_impact_matrix.go —— 影响面 → 命令面契约矩阵派生格的**反查腿**（缺口 `GAP-20260927-366` 残留）。
//
// 依据（逐字）：`设计候件-GAP-20260927-366-影响面点名矩阵格.md` §四 拟设计 1–3。
//
// 366 的**申报面已落**：`scripts/gates/affected-map.json:240-249` 已把
// `core/cmd/zerg/testdata/cli-matrix.json` 列为命令面真源（4 步）·`check-affected-map.py:55-58`
// 的 C 面声明也已点名三件（`main.go` / `cli-matrix.json` / `cli-contract-baseline.json`）。
// 缺的只有这条**联动腿**：改动命中某命令的 argv / 六键包封字节面 ⇒ **自动点名矩阵那一格**。
//
// 本件只落**只读取数口**（两张反查表 + 卡片体例点名行 + 机读行）——
// **不接进** `cmdImpact` 主干（接法 = 影响面卡片 `family_impact_card.go` 还是新开一格 · 待拍 ·
// 见设计候件 §六；本件只备「料」，接线由父代理按拍板结论单批落）。
//
// 红线（逐条 · 与 `family_impact_hook.go:13-14` 同一份）：
//
//	✗ 不改真写前置语义 · ✗ 不让 `impact_digest` 变成放行条件 · ✗ 不新增对外面（不加命令 / 旗标）·
//	✗ 只读（不写缓存 / 不落审计 / 不改件 / 不建索引）。
//
// 真源只有一处：`core/cmd/zerg/testdata/cli-matrix.json`（本件**只读**它，并**复用同包**
// `family_gate_matrix.go` 的 `cliMatrixFile` / `cliMatrixCase` 读法 —— 不另抄第二份解析器）。
// 三档不许混（照 `impactContractCatalogOf` 的同一条纪律）：
//
//	· 件读不到 ⇒ `读不到`（**不许当「没有格」**）；
//	· 解析不了 ⇒ `读不到`；
//	· `cases[]` 一条都没有 ⇒ `读不到`（**空表不许当「没有格」**）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// impactMatrixRel —— 命令面契约矩阵真源（与 `affected-map.json` / `check-affected-map.py` C 面声明同名件）。
const impactMatrixRel = "core/cmd/zerg/testdata/cli-matrix.json"

// impactMatrixCatalog —— 矩阵现读快照 + **两张反查表**（`command` → 格 id · argv 逐 token → 格 id）。
//
// 两张表都在建表时**去重 + 定序**（`impactMatrixSortedIndex`）⇒ 同一目标两跑逐字相同（无时钟无耗时 ·
// 同 `impactContractSignal` 的 `M8` 纪律）。`Why` 一并带出（供卡片 `why` 取「契约」那一档时留证据）。
type impactMatrixCatalog struct {
	Rel       string
	Lines     int
	Cells     []cliMatrixCase
	ByCommand map[string][]string // 命令路径 → 格 id（定序 · 去重）
	ByToken   map[string][]string // argv 逐 token → 格 id（定序 · 去重）
	Status    string              // 取值 / 读不到
	Reason    string
}

// impactMatrixSortedIndex 把 `map[键]map[id]bool` 摊成 `map[键][]id`（每格 id **去重 + 定序**）。
// 空键 / 空 id 一律丢（防「矩阵里混进空命令行」把空键当万能命中）。
func impactMatrixSortedIndex(idx map[string]map[string]bool) map[string][]string {
	out := map[string][]string{}
	for k, set := range idx {
		ids := make([]string, 0, len(set))
		for id := range set {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out[k] = ids
	}
	return out
}

// impactMatrixCellsOf 现读矩阵真源并建两张反查表（**只读** · 与 `cmdGateMatrix` 读的是同一个件）。
//
// 复用同包 `cliMatrixFile`（`family_gate_matrix.go`）—— 它有意不读公开面两条可选列
// （`want_rc_public` / `want_stdout_bytes_public`）：反查只认**本机档**那张表的 `command` / `argv`
// 键，公开面档不参与点名（同 `gate matrix` 的口径）。
func impactMatrixCellsOf(root string) impactMatrixCatalog {
	cat := impactMatrixCatalog{
		Rel:       impactMatrixRel,
		Cells:     []cliMatrixCase{},
		ByCommand: map[string][]string{},
		ByToken:   map[string][]string{},
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(impactMatrixRel)))
	if err != nil {
		cat.Status, cat.Reason = "读不到", fmt.Sprintf("矩阵真源读不到（%s）", impactMatrixRel)
		return cat
	}
	var mf cliMatrixFile
	if err := json.Unmarshal(b, &mf); err != nil {
		cat.Status, cat.Reason = "读不到", fmt.Sprintf("矩阵真源解不开（%v）", err)
		return cat
	}
	cat.Lines = len(strings.Split(strings.TrimRight(string(b), "\n"), "\n"))
	if len(mf.Cases) == 0 {
		cat.Status, cat.Reason = "读不到", "`cases[]` 一条都没有（**空表不许当「没有格」**）"
		return cat
	}
	cat.Cells = mf.Cases

	byCmd := map[string]map[string]bool{}
	byTok := map[string]map[string]bool{}
	add := func(idx map[string]map[string]bool, key, id string) {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(id) == "" {
			return
		}
		if idx[key] == nil {
			idx[key] = map[string]bool{}
		}
		idx[key][id] = true
	}
	for _, c := range mf.Cases {
		add(byCmd, strings.TrimSpace(c.Command), c.ID)
		for _, t := range c.Argv {
			add(byTok, strings.TrimSpace(t), c.ID)
		}
	}
	cat.ByCommand = impactMatrixSortedIndex(byCmd)
	cat.ByToken = impactMatrixSortedIndex(byTok)
	cat.Status = "取值"
	return cat
}

// impactMatrixCellsForCommand 反查「某命令路径」命中的格 id（无命中 ⇒ 空表，**不是 nil**）。
// 「读不到」的目录一律返回空表 —— 调用方须先看 `cat.Status`，别把空表读成「没有格」。
func impactMatrixCellsForCommand(cat impactMatrixCatalog, command string) []string {
	if cat.Status != "取值" {
		return []string{}
	}
	if ids, ok := cat.ByCommand[strings.TrimSpace(command)]; ok {
		return append([]string{}, ids...)
	}
	return []string{}
}

// impactMatrixCellsForArgvToken 反查「某个 argv token」（如 `--dry-run`）命中的格 id。
// 为什么按**逐 token**建表：argv 面改一枚旗标就会牵动多格（同命令的多个 must-fail 态），
// 逐 token 反查能把「改了哪个旗标」精确定位到格集合（设计候件 §六 待拍 1 的建议取法）。
func impactMatrixCellsForArgvToken(cat impactMatrixCatalog, token string) []string {
	if cat.Status != "取值" {
		return []string{}
	}
	if ids, ok := cat.ByToken[strings.TrimSpace(token)]; ok {
		return append([]string{}, ids...)
	}
	return []string{}
}

// impactMatrixCellOf 按格 id 取回那一格（**另取一遍**的读法口 · 找不到 ⇒ (零值, false)）。
func impactMatrixCellOf(cat impactMatrixCatalog, id string) (cliMatrixCase, bool) {
	for _, c := range cat.Cells {
		if c.ID == id {
			return c, true
		}
	}
	return cliMatrixCase{}, false
}

// impactMatrixCellCard —— 一个契约格的**卡片四字段**（`what`/`why`/`how`/`red`，与
// `impactCardEntry.Row` 体例同形；`why` 取六选一里的「契约」那一档 —— 见 `impactWhySix`）。
//
// 为什么要给四字段而不是一句自由文：影响面摘要里点名一条要能进卡片的同一台台
// （`why` 闭集六选一 + `red` 非空即置顶）⇒ 这里就把四字段备好，接线时直接 `append` 进卡片候选。
func impactMatrixCellCard(c cliMatrixCase) map[string]string {
	return map[string]string{
		"what": "契约格:" + c.ID,
		"why":  "契约",
		"how": fmt.Sprintf("command=%s · argv=%s · want_rc=%d · want_stdout_bytes=%d",
			c.Command, strings.Join(c.Argv, " "), c.WantRC, c.WantStdoutBytes),
		"red": "门⑪ 命令面契约（改命中本命令的 argv / 六键包封字节面 ⇒ 本格 want_rc / want_stdout_bytes 漂即红）",
	}
}

// impactMatrixCellName —— 一条**点名行**（人面可读 · 与卡片 `what` 同一身份串）。
func impactMatrixCellName(c cliMatrixCase) string {
	return fmt.Sprintf("契约格 %s（command=%s · want_rc=%d · want_stdout_bytes=%d）",
		c.ID, c.Command, c.WantRC, c.WantStdoutBytes)
}

// impactMatrixCellsSignal —— 反查腿的**机读行**（**定序 · 无时钟无耗时** ⇒ 同一目标两跑逐字相同 · `M8`）。
//
// 形状（**不是**六键包封的一部分 —— 包封一个键都不加 ✗；`kind` 是命令面保留键 ⇒ 本行用 `signal`）：
//
//	{"signal":"impact.matrix","rel":…,"cells":N,"commands":[…],"tokens":[…],
//	 "by_command":{…},"by_token":{…}}
func impactMatrixCellsSignal(cat impactMatrixCatalog) string {
	cmds := make([]string, 0, len(cat.ByCommand))
	for k := range cat.ByCommand {
		cmds = append(cmds, k)
	}
	sort.Strings(cmds)
	toks := make([]string, 0, len(cat.ByToken))
	for k := range cat.ByToken {
		toks = append(toks, k)
	}
	sort.Strings(toks)
	doc := struct {
		Signal    string              `json:"signal"`
		Rel       string              `json:"rel"`
		Status    string              `json:"status"`
		Cells     int                 `json:"cells"`
		Commands  []string            `json:"commands"`
		Tokens    []string            `json:"tokens"`
		ByCommand map[string][]string `json:"by_command"`
		ByToken   map[string][]string `json:"by_token"`
	}{
		Signal:    "impact.matrix",
		Rel:       cat.Rel,
		Status:    cat.Status,
		Cells:     len(cat.Cells),
		Commands:  cmds,
		Tokens:    toks,
		ByCommand: cat.ByCommand,
		ByToken:   cat.ByToken,
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return fmt.Sprintf("（机读行拼不出来：%v）", err)
	}
	return string(b)
}

// emitImpactMatrixCellsBlock —— stderr 的**反查腿块**（接线落点草案 · 与 `emitImpactContractBlock`
// 同款落 stderr · 不写进六键包封）。本件**只备料**：谁调它、打在哪一段由拍板结论定（设计候件 §六 待拍 2）。
//
// 「读不到」⇒ 只打一行**不给结论**并把理由说清（不许静默成空块、不许空表当没有格）。
func emitImpactMatrixCellsBlock(w io.Writer, root string) {
	cat := impactMatrixCellsOf(root)
	fmt.Fprintf(w, "%s: 命令面契约矩阵反查腿（`GAP-20260927-366` 残留 · 只读 `%s`）\n", progName, impactMatrixRel)
	if cat.Status != "取值" {
		fmt.Fprintf(w, "  本跑：**读不到** —— %s ⇒ 反查腿**不给结论**（「读不到」不当「没有格」）\n", cat.Reason)
		return
	}
	fmt.Fprintf(w, "  矩阵 %d 行 · 格 %d 条 · 命令面键 %d 个 · argv token 键 %d 个\n",
		cat.Lines, len(cat.Cells), len(cat.ByCommand), len(cat.ByToken))
	fmt.Fprintf(w, "  零副作用：只读（矩阵真源一条）⇒ 不写缓存 / 不落审计 / 不改件 / 不建索引\n")
	fmt.Fprintf(w, "%s: 反查腿机读行：%s\n", progName, impactMatrixCellsSignal(cat))
}
