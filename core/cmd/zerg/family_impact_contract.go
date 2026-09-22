// family_impact_contract.go —— 契约面接线件（`B2` · 任务单-影响面实施-20260922 §三 `B2` 判据 ①–③）。
//
// 本件把**契约面**接到 `core/internal/contract/registry.json` 的 `entries[]` 上，只做三件事
// （任务单判据 ①–③ **逐字**）：
//
//	① **每条目录项带 `entry` 四字段**（`id` / `change_class` / `gate` / `change_note`）
//	   且 `id` 集合与 `registry.json` **逐字一致** —— 四字段**逐条打**（缺哪一格就报哪一格，
//	   **不许静默绿** ✗）；「读不到」与「一条都没有」都不许被读成「都不在册」。
//	② **兼容级别只许报、不许据此红绿任何一步**（任务单逐字：「今天没有基线」；设计 v1.6 未证实项
//	   `Y-b7` = 契约**跨版兼容级别**无历史可比）⇒ 本件只立**一个**取值（`未评`）与一条纪律；
//	   红绿的唯一来源仍是 `impactLayerContract` 里那一条（`change_class == "B"` ⇒ 进「会红」闭集），
//	   退码也**一个字不受它影响**。
//	③ **「版本间对拍」只出设计、不落实现**（`R5` 已拍「要，但**另立项**」· 任务单 §三 B2 目标逐字
//	   「只出设计、不落实现」）—— 本件只把四条口径 + **两枚指纹**（`head_sha` + 基线 rev）落在 stderr，
//	   **不落新快照件** ✗ · **不扩到全仓** ✗（先只对 `registry.json` 一条）。
//
// 为什么这一块落 stderr：六键包封（`schema`/`kind`/`items`/`meta`/`warnings`/`truncated`）**一个键
// 都不加** ✗（`A1`–`A5` 红线「不改 `emitEnvelope*`」本批未解禁）⇒ 契约面这一份取值与 `B1`/`B2`
// 时间面同款落 stderr，缺口照实点名（与层表那条同款）。
//
// 红线（本件逐条）：**不许改 `registry.json` 的既有 7 条**（除 `R43` 那条明文二选一）✗ ·
// **不许落新快照件** ✗ · **不许扩到全仓** ✗ · **不许用兼容级别判红绿** ✗ · **不改 `emitEnvelope*`** ✗ ·
// **不动 `publish/` 七件生效面** ✗ · 只读（不写缓存 / 不落审计 / 不改件 / 不建索引）· 不引新依赖。
//
// `R43`（任务单 §三 B2 前置依赖点名 · 归属「未定」⇒ **本件只点名、不派实现** ✗）：
// `registry.json` 第 3 行自陈的门 `scripts/gates/check-contract-registry.py` —— 「补件」或「改第 3 行」
// **二选一**，「别留着」；本件只**现读盘上在不在**并照实报。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// impactContractFourFields —— 判据① 点名的**四字段**（顺序即打出来的顺序）。
var impactContractFourFields = []string{"id", "change_class", "gate", "change_note"}

// impactContractSixFields —— 判据③「比什么」点名的**每条 6 字段**（逐字取自设计 v1.6 §2.2 / R5）。
var impactContractSixFields = []string{"name", "truth", "truth_shape", "version_field", "gate", "change_class"}

// impactContractCompatLevel —— 兼容级别那一格的**唯一取值**（判据②）。
//
// 为什么今天只有一个取值：任务单逐字「**今天没有基线**」（设计 v1.6 未证实项 `Y-b7`：契约
// **跨版兼容级别**（无历史可比））⇒ 本件**不编**任何级别值（编出来的级别会变成一个没人能复算的数）；
// 即便将来有人给出基线，这一格也只许**报** —— 见 `impactContractCompatNote`。
const impactContractCompatLevel = "未评（今天没有基线 · 设计 v1.6 未证实项 `Y-b7`：跨版兼容级别无历史可比）"

// impactContractCompatNote —— 判据② 的**纪律原话**（只报 · 不判红绿）。
const impactContractCompatNote = "兼容级别**只许报、不许据此红绿任何一步**；红绿的唯一来源 = `change_class == \"B\"`（③ 层命中条的闭集字段）⇒ 本格改值**不改**任何一步的红绿，也**不改**退码"

// impactContractRegistryGateRel —— `registry.json` 第 3 行自陈的那道门（`R43` 的**对象**）。
const impactContractRegistryGateRel = "scripts/gates/check-contract-registry.py"

// impactContractEntry —— 一条**目录项**（判据① 的四字段；四字段之外的东西本件不搬 —— 卡片四字段
// 是 `what`/`why`/`how`/`red`，两个「四字段」不是同一个东西，别混）。
type impactContractEntry struct {
	ID          string `json:"id"`
	ChangeClass string `json:"change_class"`
	Gate        string `json:"gate"`
	ChangeNote  string `json:"change_note"`
}

// impactContractCatalog —— 契约目录的现读快照（**单一真源** = `registry.json` 一份）。
//
// `Rows` 恒为数组（空为 `[]`）；`Missing` 逐条记「哪一条缺哪一格」（缺一格就点名一格）。
type impactContractCatalog struct {
	Rel       string
	Lines     int
	Rows      []impactContractEntry
	Missing   []string // 形如 `S-x 缺 change_note`
	Duplicate []string
	Status    string // 取值 / 读不到
	Reason    string
}

// impactContractCatalogOf 现读契约登记表（**只读真源** · 与 ③ 层读的是同一个件，不另抄一份）。
//
// 三档不许混（照 `impactLayerContract` 的同一条纪律）：
//
//	· 件读不到 ⇒ `读不到`（**不许当「都不在册」**）；
//	· 解析不了 ⇒ `读不到`；
//	· `entries[]` 一条都没有 ⇒ `读不到`（**空表不许当「都不在册」**）。
func impactContractCatalogOf(root string) impactContractCatalog {
	cat := impactContractCatalog{Rel: impactRegistryRel, Rows: []impactContractEntry{}, Missing: []string{}, Duplicate: []string{}}
	abs := filepath.Join(root, filepath.FromSlash(impactRegistryRel))
	b, err := os.ReadFile(abs)
	if err != nil {
		cat.Status, cat.Reason = "读不到", fmt.Sprintf("契约登记表读不到（%s）", impactRegistryRel)
		return cat
	}
	var doc struct {
		Entries []struct {
			ID          string `json:"id"`
			ChangeClass string `json:"change_class"`
			Gate        string `json:"gate"`
			ChangeNote  string `json:"change_note"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		cat.Status, cat.Reason = "读不到", fmt.Sprintf("契约登记表解不开（%v）", err)
		return cat
	}
	cat.Lines = len(strings.Split(strings.TrimRight(string(b), "\n"), "\n"))
	if len(doc.Entries) == 0 {
		cat.Status, cat.Reason = "读不到", "`entries[]` 一条都没有（**空表不许当「都不在册」**）"
		return cat
	}
	seen := map[string]int{}
	for _, e := range doc.Entries {
		cat.Rows = append(cat.Rows, impactContractEntry{
			ID: e.ID, ChangeClass: e.ChangeClass, Gate: e.Gate, ChangeNote: e.ChangeNote,
		})
		// 判据① 的**有牙的那一格**：四字段逐格判空 —— 缺哪一格就点名哪一格（不静默滑过）。
		for _, f := range impactContractFourFields {
			v := map[string]string{"id": e.ID, "change_class": e.ChangeClass, "gate": e.Gate, "change_note": e.ChangeNote}[f]
			if strings.TrimSpace(v) == "" {
				id := e.ID
				if id == "" {
					id = "（id 也空）"
				}
				cat.Missing = append(cat.Missing, fmt.Sprintf("%s 缺 %s", id, f))
			}
		}
		if strings.TrimSpace(e.ID) != "" {
			seen[e.ID]++
		}
	}
	for id, n := range seen {
		if n > 1 {
			cat.Duplicate = append(cat.Duplicate, fmt.Sprintf("%s 出现 %d 次", id, n))
		}
	}
	cat.Status = "取值"
	return cat
}

// impactContractIDSet 目录项的 id 集合（**排序** ⇒ 逐字可比；空集不是错误，但那一条由
// `impactContractAuditOf` 报出来 —— 与 `registry.json` 的 id 列比）。
func impactContractIDSet(cat impactContractCatalog) []string {
	ids := []string{}
	seen := map[string]bool{}
	for _, r := range cat.Rows {
		if strings.TrimSpace(r.ID) == "" || seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		ids = append(ids, r.ID)
	}
	return impactStepSortedUniq(ids)
}

// impactContractAudit —— 判据① 的两半判定：**四字段齐**（缺 N 条）+ **id 集合与 `registry.json`
// 逐字一致**（`kv k` + 两个差集）。
//
// 为什么两半都要：**四字段齐**只说明条目自身没缺格，**id 集合一致**才说明「接线件报出来的目录」
// 与真源是**同一份**（第一半绿、第二半非空差集 = 接线件丢条/多编条 —— 两半都不是摆设）。
type impactContractAudit struct {
	FourOK    bool
	Missing   []string // 缺字段（判据①前半的负控面）
	IDEqual   bool     // 判据①后半
	OnlyInReg []string // 只在 registry.json 里（接线件漏了）
	OnlyInCat []string // 只在目录里（接线件多编了）
	SelfN     int      // registry.json 的 id 列条数（**另取一遍** ⇒ 不相等的发现面）
	CatN      int
}

// impactContractAuditOf 判据①的判定口（`regIDs` = 从 `registry.json` **另取一遍**的 id 列；
// 现成口 = `impactRegistryIDs`（`A1` 起就在 · 只解析 `id` 一列）—— 两遍读同一个件、走**两条代码
// 路径**，对不上就说明接线件与真源脱了钩）。
func impactContractAuditOf(cat impactContractCatalog, regIDs []string) impactContractAudit {
	a := impactContractAudit{
		Missing:   append([]string{}, cat.Missing...),
		OnlyInReg: []string{}, OnlyInCat: []string{},
		SelfN: len(regIDs),
		CatN:  len(impactContractIDSet(cat)),
	}
	a.FourOK = len(cat.Missing) == 0 && len(cat.Rows) > 0
	inCat := map[string]bool{}
	for _, id := range impactContractIDSet(cat) {
		inCat[id] = true
	}
	inReg := map[string]bool{}
	for _, id := range regIDs {
		inReg[id] = true
	}
	for _, id := range regIDs {
		if !inCat[id] {
			a.OnlyInReg = append(a.OnlyInReg, id)
		}
	}
	for _, id := range impactContractIDSet(cat) {
		if !inReg[id] {
			a.OnlyInCat = append(a.OnlyInCat, id)
		}
	}
	a.IDEqual = len(a.OnlyInReg) == 0 && len(a.OnlyInCat) == 0
	return a
}

// impactContractFingerprints —— 判据③ 的**两枚指纹**（`head_sha` + 基线 rev）。
//
// 基线取法**逐字**（任务单/设计 v1.6 §2.2）：
//
//	`git log -1 --format=%H -- core/internal/contract/registry.json`（**上一次改动该件的提交**）
//	**及其父** —— 基线可以是 **git 引用**（同形一手：`buf breaking --against '.git#branch=main'`）
//	⇒ **不落新快照件** ✗。
//
// 取不到 ⇒ 返回原因（**不当 0 看** —— 「取不到」与「没有」是两件事）。
func impactContractFingerprints(root string) (head, rev, parent, reason string) {
	head = impactHeadSHA(root)
	out, _, code, err := impactRunIn(root, "git", "log", "-1", "--format=%H", "--", impactRegistryRel)
	if err != nil || code != 0 {
		return head, "", "", "基线取不到（`git log -1 --format=%H --` 起不来或退码非 0）"
	}
	rev = strings.TrimSpace(out)
	if rev == "" {
		return head, "", "", "基线取不到（这条路径没有历史 —— 空输出**不当「没有变更」**看）"
	}
	out, _, code, err = impactRunIn(root, "git", "rev-parse", rev+"^")
	if err != nil || code != 0 || strings.TrimSpace(out) == "" {
		return head, rev, "", "基线的**父**取不到（`git rev-parse <基线>^`）—— 第一版提交那一档要靠人指基线"
	}
	return head, rev, strings.TrimSpace(out), ""
}

// impactContractRegistryGateExists —— `R43` 那一格：`registry.json` 自陈的门**盘上在不在**
// （照实现读 · 不猜）。本件只登记，**不许替它补件** ✗。
func impactContractRegistryGateExists(root string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(impactContractRegistryGateRel)))
	return err == nil
}

// emitImpactContractBlock —— stderr 的**契约面块**（判据①–③ 的可读落点 · 不写进六键包封）。
//
// 判据② 的**只报**在这里是**打出来的**（兼容级别那一行恒打），判据① 的两半与判据③ 的两枚指纹
// 也各占一行 —— 同一份取值另有 `impactContractJSON` 的机读行（定序 · 无时钟无耗时 ⇒ 两跑逐字同）。
func emitImpactContractBlock(w io.Writer, root string, cat impactContractCatalog, audit impactContractAudit, head, rev, parent, fpReason string) {
	fmt.Fprintf(w, "%s: `B2` 契约面（含「版本间对拍」**只出设计、不落实现** —— `R5` 另立项；接在 `%s` 的 `entries[]` 上）\n",
		progName, impactRegistryRel)
	if cat.Status != "取值" {
		fmt.Fprintf(w, "  本跑：**读不到** —— %s ⇒ 判据① 的两半与判据③ 的指纹都**不给结论**（「读不到」不当「没有」）\n", cat.Reason)
		return
	}
	fmt.Fprintf(w, "  判据①（目录项 · 四字段 `%s`）：%d 条逐条打（`registry.json` %d 行）\n",
		strings.Join(impactContractFourFields, "` / `"), len(cat.Rows), cat.Lines)
	for _, r := range cat.Rows {
		fmt.Fprintf(w, "    id=%s · change_class=%s · gate=%s · change_note=%s\n", r.ID, r.ChangeClass, r.Gate, r.ChangeNote)
	}
	if audit.FourOK {
		fmt.Fprintf(w, "  判据①（前半 · 四字段齐）：✓ %d/%d 条四字段齐（缺字段 0 条 · id 重复 0 条）\n", len(cat.Rows), len(cat.Rows))
	} else {
		fmt.Fprintf(w, "  判据①（前半 · 四字段齐）：✗ —— 缺 %d 处：%s\n", len(audit.Missing), strings.Join(audit.Missing, " · "))
		if len(cat.Duplicate) > 0 {
			fmt.Fprintf(w, "    （另有 id 重复：%s）\n", strings.Join(cat.Duplicate, " · "))
		}
	}
	verdict := "✗（接线件与真源**脱了钩** —— 差集非空，别把前半的绿读成这一半也绿）"
	if audit.IDEqual {
		verdict = "✓ 差集 ∅"
	}
	fmt.Fprintf(w, "  判据①（后半 · id 集合与 `registry.json` 逐字一致）：%dv%d %s；只在真源=%v · 只在目录=%v（真源 id 列**另取一遍** = `impactRegistryIDs`，两条代码路径对拍）\n",
		audit.CatN, audit.SelfN, verdict, audit.OnlyInReg, audit.OnlyInCat)
	// 判据②（**只报**）：这一行恒打 —— 兼容级别今天只有一个取值（`未评`），且它**不进任何红绿判定**。
	fmt.Fprintf(w, "  判据②（兼容级别**只报不拦**）：兼容级别 = %s\n", impactContractCompatLevel)
	fmt.Fprintf(w, "    %s\n", impactContractCompatNote)
	if !impactContractRegistryGateExists(root) {
		fmt.Fprintf(w, "  `R43`（前置点名 · **本件不派实现**）：`registry.json` 自陈的门 `%s` —— **盘上不存在** ⇒ 「补件」或「改第 3 行」二选一（别留着）；本件只现读、不替它补件\n",
			impactContractRegistryGateRel)
	} else {
		fmt.Fprintf(w, "  `R43`（前置点名）：`registry.json` 自陈的门 `%s` 盘上**在** ⇒ 本件只现读；「表 ↔ 件」跑没跑由那道门自己说\n",
			impactContractRegistryGateRel)
	}
	// 判据③（**只出设计**）：四条口径逐字 + 本跑的两枚指纹 + 明确不落实现。
	fmt.Fprintf(w, "  判据③（对拍口径 · **只出设计**，`R5` 另立项 ⇒ 本件**不落实现** ✗ · **不落新快照件** ✗ · **不扩到全仓** ✗）：\n")
	fmt.Fprintf(w, "    ① 基线 = `git log -1 --format=%%H -- %s`（上一次改动该件的提交）**及其父**（基线可以是 git 引用 —— 同形一手 `buf breaking --against '.git#branch=main'`）；\n", impactRegistryRel)
	fmt.Fprintf(w, "    ② 比什么 = `entries[].id` 集合 + 每条 6 字段（`%s`）+ `change_flow` / `authority` 整体；\n", strings.Join(impactContractSixFields, "` / `"))
	fmt.Fprintf(w, "    ③ 零差异**明写「无变更」**（有差异则逐处点名）；④ **只存两枚指纹**（`head_sha` + 基线 rev）\n")
	if fpReason != "" {
		fmt.Fprintf(w, "    本跑指纹：**取不到** —— %s\n", fpReason)
	} else {
		fmt.Fprintf(w, "    本跑指纹：`head_sha`=%s · 基线 rev=%s（其父=%s）\n", head, rev, parent)
	}
	fmt.Fprintf(w, "    面的边界（照实写）：门只到「**表 ↔ 件**」，**不到「旧版 ↔ 新版」** ⇒ **不许把「表 ↔ 件」的绿读成「承诺没破」**；本块也**不落实现**那个对拍引擎\n")
	fmt.Fprintf(w, "  零副作用：只读（`registry.json` 一条 + 两条只读 git 命令）⇒ 不写缓存 / 不落审计 / 不改件 / **不落新快照件** · 不改门禁脚本 · 不动 `registry.json` 的既有条目\n")
	// 机读行（同一份取值 ⇒ 只出文本、**不落盘**）。
	fmt.Fprintf(w, "%s: 契约面机读行：%s\n", progName, impactContractSignal(cat, audit, head, rev, parent))
}

// impactContractFace 现读契约面三件（一次取齐，人面块与机读行共用同一份取值，不另算一遍）：
// 目录项快照 · 判据① 的两半判定 · 判据③ 的两枚指纹（含取不到的原因）。
func impactContractFace(root string) (impactContractCatalog, impactContractAudit, string, string, string, string) {
	cat := impactContractCatalogOf(root)
	regIDs, err := impactRegistryIDs(root)
	if err != nil {
		regIDs = []string{}
	}
	audit := impactContractAuditOf(cat, regIDs)
	head, rev, parent, why := impactContractFingerprints(root)
	return cat, audit, head, rev, parent, why
}

// impactContractSignal —— 契约面的机读行（**定序 · 无时钟无耗时** ⇒ 同一目标两跑逐字相同 · `M8`）。
//
// 形状（**不是**六键包封的一部分 —— 包封那一格一个键都不加 ✗；`kind` 是命令面保留键 ⇒ 本行用
// `signal`，不借保留键）：
//
//	{"signal":"impact.contract","registry":…,"entries":[…四字段…],"id_set":[…],"id_set_equal":…,
//	 "four_fields_missing":[…],"compat_level":…,"compat_level_affects_red":false,…}
func impactContractSignal(cat impactContractCatalog, audit impactContractAudit, head, rev, parent string) string {
	type item struct {
		ID          string `json:"id"`
		ChangeClass string `json:"change_class"`
		Gate        string `json:"gate"`
		ChangeNote  string `json:"change_note"`
	}
	doc := struct {
		Signal        string   `json:"signal"`
		Registry      string   `json:"registry"`
		RegistryGate  string   `json:"registry_gate"`
		Entries       []item   `json:"entries"`
		IDSet         []string `json:"id_set"`
		IDSetEqual    bool     `json:"id_set_equal"`
		OnlyInReg     []string `json:"id_only_in_registry"`
		OnlyInCat     []string `json:"id_only_in_catalog"`
		Missing       []string `json:"four_fields_missing"`
		CompatLevel   string   `json:"compat_level"`
		CompatReport  bool     `json:"compat_level_report_only"`
		CompatRed     bool     `json:"compat_level_affects_red"`
		CompareFields []string `json:"compare_six_fields"`
		BaseLineRev   string   `json:"baseline_rev"`
		BaselinePar   string   `json:"baseline_parent"`
		HeadSHA       string   `json:"head_sha"`
		Scope         string   `json:"scope"`
		SnapshotFile  int      `json:"snapshot_files"`
		EngineInThis  string   `json:"engine_in_this"`
	}{
		Signal:        "impact.contract",
		Registry:      impactRegistryRel,
		RegistryGate:  impactContractRegistryGateRel,
		Entries:       []item{},
		IDSet:         impactContractIDSet(cat),
		IDSetEqual:    audit.IDEqual,
		OnlyInReg:     audit.OnlyInReg,
		OnlyInCat:     audit.OnlyInCat,
		Missing:       audit.Missing,
		CompatLevel:   impactContractCompatLevel,
		CompatReport:  true,
		CompatRed:     false,
		CompareFields: impactContractSixFields,
		BaseLineRev:   rev,
		BaselinePar:   parent,
		HeadSHA:       head,
		Scope:         "只对 `" + impactRegistryRel + "` 一条（不扩到全仓）",
		SnapshotFile:  0,
		EngineInThis:  "本件不落实现（只出设计 · `R5` 另立项）",
	}
	for _, r := range cat.Rows {
		doc.Entries = append(doc.Entries, item{ID: r.ID, ChangeClass: r.ChangeClass, Gate: r.Gate, ChangeNote: r.ChangeNote})
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return fmt.Sprintf("（机读行拼不出来：%v）", err)
	}
	return string(b)
}
