// family_dev_candidate.go —— 自开发面的**候选区 / 证据单 / 四值回滚**三条（§17.4 第 2/3/5 条 ·
// §17.6 `SD4`/`SD8`/`SD9`/`SD10` · §17.7 次序 · 开工单 T-58）。
//
// 本件落三件事，形状全部取自**一份新真源** `core/internal/contract/dev-candidate.json`
// （生产侧与消费侧读同一份；另写一份就是第二份真源）：
//
//	① `zerg dev verify` —— **合成一份验收证据单**（§17.4 第 5 条）：逐条 `criterion`/`verdict`/
//	   `evidence{}`；**证据为空 ⇒ 退码 2（不给结论）**（判据①）。产物落**候选区**（闭集）。
//	   ★ `SD8`：它**只产证据单、不给「通过」的结论** —— 「通过」只能由门禁出口（rc=0）或人来给。
//	② `zerg dev build` / `zerg dev test` —— 候选区里的构建与测试（§17.4 第 2/3 条）。
//	   **§17.7 次序不许倒**（判据②）：找不到该候选的验收证据单 ⇒ 退码 2
//	   （`detail=build_without_verify` / `test_without_verify`），**不许「先建了再补判据」**。
//	   本版这两条只到 `--dry-run` 出计划件（真跑一律拒执 ⇒ 2），沿用 `cmdGuarded` 同一套防呆形状。
//	③ 候选区**闭集**（判据⑤）：候选件只能待在候选根里 —— 生产目录（`bin/` 与 `dist/` 下、排除候选根）
//	   里**永不**出现名字带候选 id 的件。判定口 `productionCandidateNameViolations` 由
//	   `dev_candidate_test.go` 成对负控（喂一件坏件 ⇒ 必红）。
//
// 与既有条文的接缝（**不重复立项** ✗）：
//
//	· `SD10-b` 撞名（判据⑥）：`build` / `release` 两处，**K 族是规范入口**、`dev` 族只多一个作用域，
//	  底下的脚本是同一条 —— 逐字见真源的 `canonical_entry`。
//	· `SD4`（候选区总量上限 / 门禁趟数上限）：**数值不在此臆造**（照 `RC6`「阈值显式、不许魔数」）
//	  ⇒ 真源里只登记，数值待 §十二 下一版拍板。
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

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// devVerifyFields —— `dev verify --json` 的全部字段（K1：机器面先定）。
var devVerifyFields = []string{"candidate", "criterion", "verdict", "rc", "log_path",
	"code_sha", "node", "layer", "contract"}

// evidenceEntry —— 证据单的一条（三个必备字段名取自真源 `evidence_entry_fields`）。
type evidenceEntry struct {
	Criterion string            `json:"criterion"`
	Verdict   string            `json:"verdict"`
	Evidence  map[string]string `json:"evidence"`
}

// evidenceSheet —— 一份验收证据单（`SD9`：带 `contract` 号 ⇒ 不许混比不同契约号的件）。
type evidenceSheet struct {
	Schema    string            `json:"schema"`
	Candidate string            `json:"candidate"`
	Contract  string            `json:"contract"`
	Results   string            `json:"results"`
	Entries   []evidenceEntry   `json:"entries"`
	Totals    map[string]int    `json:"totals"`
	Note      string            `json:"note"`
	Extra     map[string]string `json:"extra,omitempty"` // 逐条可选证据键的缺省（code_sha/node/layer）
}

// candidateRootDir —— 候选根：`ZERG_CANDIDATE_ROOT` > `<仓根>/dist/candidates`（**不写死绝对路径**）。
func candidateRootDir() string {
	if v := strings.TrimSpace(os.Getenv("ZERG_CANDIDATE_ROOT")); v != "" {
		return v
	}
	root := repoRoot()
	if root == "" {
		return ""
	}
	spec, err := contract.DevCandidate()
	if err != nil {
		return ""
	}
	return filepath.Join(root, filepath.FromSlash(spec.CandidateRoot))
}

// candidateIDValid —— 候选 id 必须**逐字**匹配真源里的闭集形态（`DEV-` + 四位十进制）。
// 近名（`dev-0001` / `DEV-1` / `DEV-00001`）一律拒 —— 名字不同就是另一个东西（§3.3 `N2`）。
func candidateIDValid(spec *contract.DevCandidateSpec, id string) bool {
	re, err := regexp.Compile(spec.CandidateIDPattern)
	if err != nil {
		return false
	}
	return re.MatchString(id)
}

// candidateDir —— 一个候选的落点目录（**在闭集根里**，绝不落到生产目录）。
func candidateDir(root, id string) string { return filepath.Join(root, id) }

// evidenceSheetPath —— 该候选的验收证据单落点（`dev verify` 写、`dev build`/`dev test` 读）。
func evidenceSheetPath(root, id string) string {
	return filepath.Join(candidateDir(root, id), "evidence.json")
}

// ---- `zerg dev verify` ----

// cmdDevVerify —— 合成一份验收证据单（§17.4 第 5 条 · 判据①）。
//
// 退码：`0` 全绿 · `1` 有 FAIL · `2` **证据为空 / 候选 id 不在闭集里 / 不给结论** ·
// `8` 有 BLOCKED（**不许当绿** —— 它与「失败」是两回事）。
func cmdDevVerify(inv *invocation, stdout, stderr io.Writer) int {
	spec, err := contract.DevCandidate()
	if err != nil {
		inv.setErr("blocked", "contract_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 候选区真源读不出来：%v ⇒ 不给结论\n", progName, err)
		return exitBlocked
	}
	id := strings.TrimSpace(inv.flagVal("--candidate"))
	if id == "" {
		inv.setErr("usage", "missing_required_flag", "缺 --candidate")
		fmt.Fprintf(stderr, "%s: `dev verify` 要 `--candidate <候选 id>`（形态 %s）\n", progName, spec.CandidateIDPattern)
		return exitUsage
	}
	if !candidateIDValid(spec, id) {
		inv.setErr("usage", "bad_candidate_id", "候选 id 不在闭集里")
		fmt.Fprintf(stderr, "%s: 候选 id %q 不在闭集里（形态 %s）—— 名字不同就是另一个东西，不做近似匹配\n",
			progName, id, spec.CandidateIDPattern)
		return exitUsage
	}
	root := candidateRootDir()
	if root == "" {
		inv.setErr("blocked", "no_candidate_root", "解析不到候选根")
		fmt.Fprintf(stderr, "%s: 解析不到候选根（可用 ZERG_CANDIDATE_ROOT 指定）⇒ 不给结论\n", progName)
		return exitBlocked
	}
	// ── 升阶闸门档（§20.4 · §20.7 `OM11` · 批 E · T-62）：只加旗标、不新增命令名 ─────────
	//    它在读证据单**之前**判：四道闸里 `G3` 读的就是候选区的全量门禁结果表。
	if inv.stageGate {
		return cmdDevVerifyGate(inv, stdout, stderr, id)
	}
	results := strings.TrimSpace(inv.flagVal("--results"))
	if results == "" {
		results = filepath.Join(candidateDir(root, id), "results.tsv")
	}
	entries, why := readEvidenceEntries(spec, results, inv)
	if why != "" {
		// 判据①：**证据为空 ⇒ 2（不给结论）** —— 空证据不是「没事」，是「没结论」。
		inv.setErr("usage", "no_evidence", why)
		fmt.Fprintf(stderr, "%s: 证据为空 ⇒ **不给结论**（退码 2）—— %s\n", progName, why)
		fmt.Fprintf(stderr, "口径：证据单的每一条都要带必备证据键（%s）；一条都读不出来时不写「通过」（§17.3 铁律②）\n",
			strings.Join(spec.EvidenceKeysRequired, " · "))
		return exitUsage
	}
	sheet := evidenceSheet{
		Schema:    "zerg/dev-evidence/v1",
		Candidate: id,
		Contract:  contractVersionText(),
		Results:   results,
		Entries:   entries,
		Totals:    map[string]int{},
		Note:      "本件只收证据、**不给「通过」的结论**（SD8）：通过只能由门禁出口（rc=0）或人给。",
		Extra: map[string]string{
			"code_sha": strings.TrimSpace(inv.flagVal("--code-sha")),
			"node":     lastNode(inv),
			"layer":    strings.TrimSpace(inv.flagVal("--layer")),
		},
	}
	for _, v := range spec.EvidenceVerdicts {
		sheet.Totals[v] = 0
	}
	for _, e := range entries {
		sheet.Totals[e.Verdict]++
	}
	sheetPath := evidenceSheetPath(root, id)
	if !inv.dryRun {
		if err := os.MkdirAll(candidateDir(root, id), 0o755); err != nil {
			inv.setErr("failed", "candidate_dir_unwritable", err.Error())
			fmt.Fprintf(stderr, "%s: 建不了候选目录 %s：%v\n", progName, candidateDir(root, id), err)
			return exitFail
		}
		body, _ := json.MarshalIndent(sheet, "", " ")
		if err := os.WriteFile(sheetPath, append(body, '\n'), 0o644); err != nil {
			inv.setErr("failed", "evidence_write_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 写不进证据单 %s：%v（§九 M3 C5：写失败即拒，不吞）\n", progName, sheetPath, err)
			return exitFail
		}
	}
	rc := sheetExitCode(spec, sheet.Totals)
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		rows := make([]map[string]string, 0, len(entries))
		for _, e := range entries {
			rows = append(rows, map[string]string{
				"candidate": id, "criterion": e.Criterion, "verdict": e.Verdict,
				"rc": e.Evidence["rc"], "log_path": e.Evidence["log_path"],
				"code_sha": e.Evidence["code_sha"], "node": e.Evidence["node"],
				"layer": e.Evidence["layer"], "contract": sheet.Contract,
			})
		}
		if code := selectJSONList(stdout, stderr, inv, inv.path, devVerifyFields, rows); code != exitOK {
			return code
		}
		return rc
	}
	fmt.Fprintf(stdout, "候选      : %s（证据单 %s）\n", id, sheetPath)
	fmt.Fprintf(stdout, "判据来源  : %s\n", results)
	for _, e := range entries {
		fmt.Fprintf(stdout, "  %-8s %s · rc=%s · log=%s\n", e.Verdict, e.Criterion, e.Evidence["rc"], e.Evidence["log_path"])
	}
	fmt.Fprintf(stdout, "合计      : %d 条 · %s\n", len(entries), totalsText(spec, sheet.Totals))
	fmt.Fprintf(stdout, "契约号    : %s\n", sheet.Contract)
	fmt.Fprintf(stderr, "★ 本命令只收证据、**不给「通过」的结论**（SD8）—— 通过只能由门禁出口（rc=0）或人来给\n")
	fmt.Fprintf(stderr, "候选区闭集：本件只写 %s 之下（生产目录一个字节都不动）\n", filepath.FromSlash(spec.CandidateRoot))
	return rc
}

// sheetExitCode —— 唯一判定口：`0` 全绿 · `1` 有 FAIL · `8` 有 BLOCKED（**不当绿**）。
// 口径：`1` 优先于 `8`（「有失败」比「有没结论的」更要紧，且两者都不进 0）。
func sheetExitCode(spec *contract.DevCandidateSpec, totals map[string]int) int {
	if totals["FAIL"] > 0 {
		return exitFail
	}
	if totals["BLOCKED"] > 0 {
		return exitBlocked
	}
	return exitOK
}

func totalsText(spec *contract.DevCandidateSpec, totals map[string]int) string {
	parts := []string{}
	for _, v := range spec.EvidenceVerdicts {
		parts = append(parts, fmt.Sprintf("%s %d", v, totals[v]))
	}
	return strings.Join(parts, " · ")
}

// readEvidenceEntries —— 读门禁结果表（`status\tname\trc\tsecs\tlog` 五列）→ 证据条目。
//
// 返回 (entries, why)：`why != ""` 表示**证据为空**（缺件 / 0 行 / 任一条必备证据键不全）——
// 三种都升成「不给结论」，**不**降级成空表入册（那就是把「没跑到」写成「通过」）。
func readEvidenceEntries(spec *contract.DevCandidateSpec, results string, inv *invocation) ([]evidenceEntry, string) {
	body, err := os.ReadFile(results)
	if err != nil {
		return nil, fmt.Sprintf("读不到结果表 %s（%v）", results, err)
	}
	entries := []evidenceEntry{}
	for i, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 5 {
			return nil, fmt.Sprintf("结果表第 %d 行不成形（%d 列 < 5）：%q", i+1, len(cols), line)
		}
		verdict, name, rc, logp := strings.TrimSpace(cols[0]), strings.TrimSpace(cols[1]), strings.TrimSpace(cols[2]), strings.TrimSpace(cols[4])
		if !contract.Has(spec.EvidenceVerdicts, verdict) {
			return nil, fmt.Sprintf("结果表第 %d 行的判决词 %q 不在闭集里（%s）",
				i+1, verdict, strings.Join(spec.EvidenceVerdicts, "|"))
		}
		ev := map[string]string{
			"rc":       rc,
			"log_path": logp,
			"code_sha": strings.TrimSpace(inv.flagVal("--code-sha")),
			"node":     lastNode(inv),
			"layer":    strings.TrimSpace(inv.flagVal("--layer")),
		}
		// 必备证据键逐条查空（`evidence{}` 不是可选装饰 —— 空的那个条目不算证据）。
		missing := []string{}
		for _, k := range spec.EvidenceKeysRequired {
			if strings.TrimSpace(ev[k]) == "" {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			return nil, fmt.Sprintf("结果表第 %d 行（%s）的证据缺 %s", i+1, name, strings.Join(missing, " · "))
		}
		entries = append(entries, evidenceEntry{Criterion: name, Verdict: verdict, Evidence: ev})
	}
	if len(entries) == 0 {
		return nil, fmt.Sprintf("结果表 %s 是空的（0 条判决）", results)
	}
	return entries, ""
}

// ---- `zerg dev build` / `zerg dev test` ----

// cmdDevBuild —— 候选区构建（§17.4 第 2 条）。判据②：**verify 先于 build**。
func cmdDevBuild(inv *invocation, stdout, stderr io.Writer) int {
	return devOrderedAction(inv, stdout, stderr, "build")
}

// cmdDevTest —— 两种问法**一个入口**（缺口面 P0-3 · 2026-09-21）：
//
//	· `--pkg <包> [--run <正则>]` ⇒ **作用域档**：只跑跟这次改动有关的那几个测（见 family_dev_test.go）；
//	· 其余（`--candidate <id>`）⇒ 候选区档：同一把尺子 —— 判据（证据单）在前（§17.4 第 3 条）。
//
// 为什么是同一个入口而不是两条命令：§十一 P0-3 逐字「**不开新命令** —— 给已有的 `dev test`
// 加两枚旗标」；且两种问法**互斥**（作用域档还带候选 = 说不清在测谁 ⇒ 退 2，不替人挑一个）。
func cmdDevTest(inv *invocation, stdout, stderr io.Writer) int {
	scoped := strings.TrimSpace(inv.flagVal("--pkg")) != "" || strings.TrimSpace(inv.flagVal("--run")) != ""
	if scoped {
		if strings.TrimSpace(inv.flagVal("--candidate")) != "" {
			inv.setErr("usage", "candidate_and_pkg", "--candidate 与 --pkg/--run 互斥")
			fmt.Fprintf(stderr, "%s: `--candidate` 与 `--pkg`/`--run` 互斥（作用域档 vs 候选区档 · 退码 2）\n", progName)
			return exitUsage
		}
		return cmdDevTestScoped(inv, stdout, stderr)
	}
	return devOrderedAction(inv, stdout, stderr, "test")
}

// devOrderedAction —— 两条动作共用的**次序闸**：没有该候选的验收证据单 ⇒ 拒（§17.7）。
func devOrderedAction(inv *invocation, stdout, stderr io.Writer, action string) int {
	spec, err := contract.DevCandidate()
	if err != nil {
		inv.setErr("blocked", "contract_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 候选区真源读不出来：%v ⇒ 不给结论\n", progName, err)
		return exitBlocked
	}
	id := strings.TrimSpace(inv.flagVal("--candidate"))
	if id == "" {
		inv.setErr("usage", "missing_required_flag", "缺 --candidate")
		fmt.Fprintf(stderr, "%s: `dev %s` 要 `--candidate <候选 id>`（形态 %s）\n", progName, action, spec.CandidateIDPattern)
		return exitUsage
	}
	if !candidateIDValid(spec, id) {
		inv.setErr("usage", "bad_candidate_id", "候选 id 不在闭集里")
		fmt.Fprintf(stderr, "%s: 候选 id %q 不在闭集里（形态 %s）\n", progName, id, spec.CandidateIDPattern)
		return exitUsage
	}
	root := candidateRootDir()
	sheet := evidenceSheetPath(root, id)
	if _, err := os.Stat(sheet); err != nil {
		inv.setErr("usage", action+"_without_verify", "缺验收证据单")
		fmt.Fprintf(stderr, "%s: 找不到该候选的验收证据单（%s）⇒ **拒执**（退码 2）\n", progName, sheet)
		fmt.Fprintf(stderr, "口径（§17.7 逐字）：**先有判据、再有自动化** —— 次序里 `verify` 必须先于 `%s`（判据②）\n", action)
		fmt.Fprintf(stderr, "先跑：zerg dev verify --candidate %s\n", id)
		return exitUsage
	}
	// 次序见过了 ⇒ 交回**同一套防呆形状**（`--dry-run` 出计划件 / 真跑拒执 ⇒ 2）。
	// 目标就是这枚候选：没给位置参数时用 `--candidate` 的值补上 —— 计划件里必须能看见它是谁
	// （「目标：未给」的计划件等于一张没有对象的单子）。
	if len(inv.args) == 0 {
		inv.args = []string{id}
	}
	return cmdGuarded(inv, stdout, stderr)
}

// ---- 候选区闭集：生产目录里不许出现带候选 id 的件（判据⑤）----

// candidateNameRE —— **宽松**识别（只要是 `DEV-` + 数字就算）—— 专门用来抓「名字不同就想蒙混过去」。
var candidateNameRE = regexp.MustCompile(`(?i)DEV-[0-9]+`)

// productionCandidateNameViolations —— 生产目录（`bin/` 与 `dist/` 下、排除候选根）里
// 名字带候选 id 的件 ⇒ 逐条列出（相对仓根的路径，排序）。判据⑤的**唯一判定口**。
//
// 为什么用宽松正则：判据要红的是「候选件跑到生产目录去了」这件事，
// 用严的闭集形态会**漏掉** `dev-0001.bak` 这类手工变体 —— 放它过去就是把红线变成纸线。
func productionCandidateNameViolations(repoRoot string, spec *contract.DevCandidateSpec) ([]string, error) {
	excludes := map[string]bool{}
	for _, e := range spec.ProductionExcludes {
		excludes[filepath.Join(repoRoot, filepath.FromSlash(e))] = true
	}
	out := []string{}
	for _, pr := range spec.ProductionRoots {
		base := filepath.Join(repoRoot, filepath.FromSlash(pr))
		if _, err := os.Stat(base); err != nil {
			continue // 那个生产根不存在 = 没东西可查（不是错）
		}
		err := filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if excludes[p] {
					return filepath.SkipDir
				}
				return nil
			}
			if candidateNameRE.MatchString(info.Name()) {
				rel, rerr := filepath.Rel(repoRoot, p)
				if rerr != nil {
					rel = p
				}
				out = append(out, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// ---- 小工具 ----

// lastNode 取 `--node` 的最后一个值（它走 M13 的具名旗标 `inv.nodes`，不走 kv 表）。
func lastNode(inv *invocation) string {
	if len(inv.nodes) == 0 {
		return ""
	}
	return strings.TrimSpace(inv.nodes[len(inv.nodes)-1])
}
