// family_dev.go —— 自开发面 · **提案件通道**（`zerg dev proposal new|list|show` · §17.2 ② · §17.4 #1）。
//
// 本件落的是七环闭环里**唯一「缺」的那一环**（「② 改」）：§17.2.1 逐字「现状 = 缺 · 零落点 ——
// 现状的\"改\"只有「人/AI 直接编辑文件 + git」」。本件把它变成**一条命令**，且严守三条：
//
//	① **只产可审查物，不落系统**（§17.4 #1 的语义一格）：`new` 写的是**文档面**，
//	   不写进仓（`Zerg-内部文档` / 主仓都不写）、**不触主控**（零 HTTP）⇒ 产出后系统状态逐字不变
//	   （判据出处：§17.3 铁律③ 判据 · 调研-M18 J5）。
//	② **目标必须回指既有编号**（§17.6 `SD1`：动机源 = 一份**人的清单**）—— 回指不上 ⇒ `exit 2`，
//	   **不新立退码**；闭集真源 = `core/internal/contract/dev-targets.json`（go:embed 读入，不另抄）。
//	③ **没退点的件不许提**（同节判据④）：`--rollback` 与 `--evidence` 缺一即拒 ——
//	   提案件的价值在于「能回得去、能证得明」，缺这两格的件提出来只会变成噪声。
//
// 与既有条文的接缝（**不重复立项** ✗ · §17.4 接缝表第 1 行）：
// 提案件的最小字段集与「提 ≠ 批」照 §九 M18 `C3` + `P-M18-1`，本件只把**形状**落成命令；
// `proposal` 的族名在 §3.2 里另有 `propose`（`zerg propose ls`）一条 ⇒ 本件把它落成**同一份清单的
// 第二个入口**（不新写逻辑：两个入口共用一个渲染函数）。
//
// 本版边界（照实说）：提案件落**本地状态目录**（`~/.zerg/proposals/` · 可用 `ZERG_PROPOSAL_DIR` 改写）；
// §17.4 #1 说的「主控 :8580（索引）」**未通**（那需要主控侧加索引面 = 换件档）⇒ 登记待拍，不假装有。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// proposalFields —— 提案件在 `--json` 面上可取的全部字段（K1：机器面先定）。
var proposalFields = []string{"id", "title", "target", "goal", "evidence", "rollback_ref",
	"by", "state", "criterion", "created_at", "path",
	// D3b 第一步（提案接校验 · 判据可机检）：声明改哪些件 + 判据的可跑态两格。
	"files", "criterion_state",
	// §九 M18 `C4` · 批 E · T-60：授权面两对字段（**只增不改** —— `by` 保留为 `subject` 的兼容别名）。
	"subject", "subject_kind", "egg_id", "approver", "approver_kind"}

// proposalStates —— 三态闭集（§17.4 #1 的 `--state 未决|已批准|已否决`，逐字）。
var proposalStates = []string{"未决", "已批准", "已否决"}

// proposalRecord —— 一份提案件（可审查物）。字段名一律用**契约里的词**，不自造近义词。
type proposalRecord struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Target    string   `json:"target"`       // 回指既有编号（SD1）
	Goal      string   `json:"goal"`         // 要达成什么
	Evidence  []string `json:"evidence"`     // 出处（≥1；空 ⇒ 拒）
	Rollback  string   `json:"rollback_ref"` // 退点（没有它不许提）
	Criterion string   `json:"criterion"`    // 判据条（哪条命令/哪个数字能证明成了）
	// Files —— 这件提案**声明要改**的件（仓内相对路径 · D3b 第一步）。
	// 它是 `zerg dev edit` 的唯一作用域真源：**没声明过的件写不进去**（越界写 ⇒ rc 2）。
	Files []string `json:"files"`
	By    string   `json:"by"` // 提出者（兼容别名 = Subject；字段只增不改）
	// ---- §九 M18 `C4`「提 ≠ 批」的两对字段（批 E · T-60）----
	// 提者/批者**分家**：`subject`+`subject_kind` 是提出者，`approver`+`approver_kind` 是批准者。
	// `subject_kind` 四值闭集 `human`/`ai`/`egg`/`ci`（§18.1 接缝第 18 行）；`kind=egg` ⇒ `egg_id` 必填。
	Subject      string `json:"subject"`        // 提出者（谁提的）
	SubjectKind  string `json:"subject_kind"`   // human / ai / egg / ci（四值闭集）
	EggID        string `json:"egg_id"`         // `subject_kind=egg` 时必填
	Approver     string `json:"approver"`       // 批准者（谁批的）
	ApproverKind string `json:"approver_kind"`  // 只许 human（批准只能人给）
	State        string `json:"state"`          // 未决 / 已批准 / 已否决
	CreatedAt    string `json:"created_at"`     // RFC3339
	ContractNo   string `json:"contract"`       // 产出时的契约号（SD9：证据单不许混比）
	Path         string `json:"path,omitempty"` // 落点（读回时补）
}

// cmdDevProposal —— `zerg dev proposal <动作>`（动作 = new / list / show）。
func cmdDevProposal(inv *invocation, stdout, stderr io.Writer) int {
	action := ""
	if len(inv.args) > 0 {
		action = inv.args[0]
	}
	switch action {
	case "new":
		return cmdDevProposalNew(inv, stdout, stderr)
	case "list":
		return cmdDevProposalList(inv, stdout, stderr)
	case "show":
		return cmdDevProposalShow(inv, stdout, stderr)
	case "check":
		return cmdDevProposalCheck(inv, stdout, stderr)
	case "":
		fmt.Fprintf(stderr, "%s: `dev proposal` 要给动作：new | list | show | check\n", progName)
		fmt.Fprintf(stderr, "用法：zerg dev proposal new --title <题> --target <既有编号> --goal <目标> "+
			"--evidence <出处> --rollback <退点> --criterion <可跑的判据> [--file <要改的件>]… [--by <提出者>]\n")
		fmt.Fprintf(stderr, "       zerg dev proposal list [--state 未决|已批准|已否决]\n")
		fmt.Fprintf(stderr, "       zerg dev proposal show <提案 id>\n")
		fmt.Fprintf(stderr, "       zerg dev proposal check [<提案 id>]   # 判据**可机检**复核（缺判据/判据跑不动 ⇒ 退码 2）\n")
		inv.setErr("usage", "missing_action", "缺动作")
		return exitUsage
	default:
		fmt.Fprintf(stderr, "%s: 未知 `dev proposal` 动作 %q\n", progName, action)
		fmt.Fprintf(stderr, "可用：new · list · show · check\n")
		inv.setErr("usage", "unknown_action", "未知动作")
		return exitUsage
	}
}

// cmdProposeLs —— `zerg propose ls`：§3.2 的 `propose` 那一条（**同一份清单的第二个入口**）。
//
// 为什么不另写一份渲染：两个入口看的是同一批件 —— 另写一份就是「同一件事两套说法」（§1.2 G6 的病根）。
func cmdProposeLs(inv *invocation, stdout, stderr io.Writer) int {
	return proposalList(inv, stdout, stderr)
}

// ---- new ----

func cmdDevProposalNew(inv *invocation, stdout, stderr io.Writer) int {
	rec := proposalRecord{
		Title:     strings.TrimSpace(inv.flagVal("--title")),
		Target:    strings.TrimSpace(inv.flagVal("--target")),
		Goal:      strings.TrimSpace(inv.flagVal("--goal")),
		Rollback:  strings.TrimSpace(inv.flagVal("--rollback")),
		Criterion: strings.TrimSpace(inv.flagVal("--criterion")),
		By:        strings.TrimSpace(inv.flagVal("--by")),
		Evidence:  trimAll(inv.flagVals("--evidence")),
		Files:     trimAll(inv.flagVals("--file")),
		State:     "未决",
		// §九 M18 `C4`（T-60）：提者 / 批者两对字段；`--subject` 为主、`--by` 为兼容别名。
		Subject:      strings.TrimSpace(inv.flagVal("--subject")),
		SubjectKind:  strings.TrimSpace(inv.flagVal("--subject-kind")),
		EggID:        strings.TrimSpace(inv.flagVal("--egg-id")),
		Approver:     strings.TrimSpace(inv.flagVal("--approver")),
		ApproverKind: strings.TrimSpace(inv.flagVal("--approver-kind")),
	}
	if rec.Subject == "" {
		rec.Subject = rec.By
	}
	if rec.By == "" {
		rec.By = rec.Subject
	}
	if rec.SubjectKind == "" {
		rec.SubjectKind = subjectKindDefault
	}
	// ① 缺件逐条点名（§九 M7：错误要给下一步，不给一句「参数错误」）
	missing := []string{}
	if rec.Title == "" {
		missing = append(missing, "--title")
	}
	if rec.Target == "" {
		missing = append(missing, "--target")
	}
	if rec.Goal == "" {
		missing = append(missing, "--goal")
	}
	if len(missing) > 0 {
		msg := fmt.Sprintf("缺必需旗标：%s", strings.Join(missing, " · "))
		inv.setErr("usage", "missing_required_flag", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "提案件的**最小字段集**照 §九 M18 C3；本件的判据要求四格齐：--title / --target / --goal / --rollback\n")
		return exitUsage
	}
	// ①b 授权面四条规则（§九 M18 `C4` · 批 E · T-60）：**提 ≠ 批** + 谁不许批。
	//    位置：与「缺件逐条点名」同段（都在**任何盘面动作之前**判）—— 授权面不过就不该落件。
	if rc, done := judgeAuthz(rec, inv, stderr); done {
		return rc
	}
	// ② 目标回指（§17.6 SD1：回指不上 ⇒ 2）
	canon, why := canonTarget(rec.Target)
	if why != "" {
		inv.setErr("usage", "target_not_in_backlog", why)
		fmt.Fprintf(stderr, "%s: %s\n", progName, why)
		fmt.Fprintf(stderr, "口径（§17.6 `SD1` 逐字）：动机源是一份**人的清单** —— 目标只能承接《待办-20260920.md》的 D/E/F/G 族编号 ⇒ AI **不选目标、只承接目标**\n")
		if s := nearestTarget(rec.Target); s != "" {
			fmt.Fprintf(stderr, "最像的既有编号: %s\n", s)
		}
		return exitUsage
	}
	rec.Target = canon
	// ③ 退点与出处（判据④：没退点的件不许提；出处为空 ⇒ 不是判据）
	if rec.Rollback == "" {
		inv.setErr("usage", "rollback_required", "缺退点（--rollback）")
		fmt.Fprintf(stderr, "%s: 缺退点（`--rollback <方式>`）—— **没退点的件不许提**（§17.3 铁律③③：每个写动作必须能指回一条回滚路径）\n", progName)
		return exitUsage
	}
	if len(rec.Evidence) == 0 {
		inv.setErr("usage", "evidence_required", "缺出处（--evidence）")
		fmt.Fprintf(stderr, "%s: 缺出处（`--evidence <出处>` 至少一条）—— 证据为空的件**不给结论**（§17.3 铁律②③：证据为空 ⇒ 不给结论）\n", progName)
		return exitUsage
	}
	if rec.By == "" {
		rec.By = "（未声明）"
	}
	// ⑤ **判据可机检**（D3b 第一步 · 判据缺 / 判据跑不动 ⇒ rc 2 · 不新立码）：
	//    判据的裁决权在地基（§17.3 铁律②「退出码 + kind + 证据字段」三件齐才算判据），
	//    所以一件提案的判据必须是**一条命令面自己跑得动的 `zerg …`** —— 一句人话不算判据。
	if _, why := criterionRunnable(rec.Criterion); why != "" {
		inv.setErr("usage", "criterion_not_machine_checkable", why)
		fmt.Fprintf(stderr, "%s: %s\n", progName, why)
		fmt.Fprintf(stderr, "口径（§17.3 铁律② · 缺口-命令面 §九 I5）：判据要**可机检** —— 给一条本版跑得动的 `zerg …`，"+
			"例：%s gate run --fast\n", progName)
		return exitUsage
	}
	if why := declaredFilesWhy(rec.Files); why != "" {
		inv.setErr("usage", "bad_declared_file", why)
		fmt.Fprintf(stderr, "%s: %s\n", progName, why)
		fmt.Fprintf(stderr, "`--file` 收的是**仓内相对路径**（`zerg dev edit` 的唯一作用域真源 · 不许绝对路径、不许 `..`）\n")
		return exitUsage
	}
	rec.ContractNo = contractVersionText()
	rec.CreatedAt = time.Now().Format(time.RFC3339)

	dir := proposalDir()
	if dir == "" {
		inv.setErr("blocked", "no_state_dir", "解析不到状态目录（HOME 取不到）")
		fmt.Fprintf(stderr, "%s: 解析不到状态目录（HOME 取不到 · 可用 ZERG_PROPOSAL_DIR 指定）⇒ 不给结论\n", progName)
		return exitBlocked
	}
	// `--dry-run`：只出件、**零副作用**（三态里的第一态，照 §九 M3 C4；不写任何文件）
	if inv.dryRun {
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			return selectJSON(stdout, stderr, inv, inv.path, proposalFields, proposalRow(rec))
		}
		fmt.Fprintf(stdout, "提案 id  : %s（--dry-run 未分配落点）\n", "（未写盘）")
		fmt.Fprintf(stdout, "  目标   : %s\n", rec.Target)
		fmt.Fprintf(stdout, "  标题   : %s\n", rec.Title)
		fmt.Fprintf(stdout, "  退点   : %s\n", rec.Rollback)
		fmt.Fprintf(stdout, "  状态   : %s\n", rec.State)
		fmt.Fprintf(stderr, "（--dry-run：只出件 · 零副作用 —— 未写任何文件、未触主控）\n")
		return exitOK
	}
	rec.ID = nextProposalID(dir)
	rec.Path = filepath.Join(dir, rec.ID+".json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		inv.setErr("failed", "state_dir_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: 建不了状态目录 %s：%v\n", progName, dir, err)
		return exitFail
	}
	body, err := json.MarshalIndent(rec, "", " ")
	if err != nil {
		inv.setErr("failed", "marshal_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 件序列化失败：%v\n", progName, err)
		return exitFail
	}
	// 追加只写（§九 M3 C5）：**新建**，绝不覆盖已有件（同名即拒 —— 不吞不盖）。
	f, err := os.OpenFile(rec.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		inv.setErr("failed", "proposal_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写不进提案件 %s：%v（§九 M3 C5：写失败即拒，不吞）\n", progName, rec.Path, err)
		return exitFail
	}
	if _, err := f.Write(append(body, '\n')); err != nil {
		f.Close()
		inv.setErr("failed", "proposal_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写提案件失败：%v\n", progName, err)
		return exitFail
	}
	f.Close()

	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSON(stdout, stderr, inv, inv.path, proposalFields, proposalRow(rec))
	}
	fmt.Fprintln(stdout, rec.ID)
	fmt.Fprintf(stderr, "提案件落点: %s\n", rec.Path)
	fmt.Fprintf(stderr, "状态: %s（「提 ≠ 批」：批准要人给 · §九 M18 C4）\n", rec.State)
	fmt.Fprintf(stderr, "零副作用口径：本命令**不触主控**（零 HTTP）· **不写入仓** —— 产出后系统状态逐字不变（§17.3 铁律③ 判据）\n")
	return exitOK
}

// ---- list / show ----

func cmdDevProposalList(inv *invocation, stdout, stderr io.Writer) int {
	return proposalList(inv, stdout, stderr)
}

func proposalList(inv *invocation, stdout, stderr io.Writer) int {
	want := strings.TrimSpace(inv.flagVal("--state"))
	if want != "" && !containsStr(proposalStates, want) {
		inv.setErr("usage", "bad_state", "状态不在闭集里")
		fmt.Fprintf(stderr, "%s: `--state` 只认三值：%s（给了 %q）\n", progName, strings.Join(proposalStates, "|"), want)
		return exitUsage
	}
	dir := proposalDir()
	recs, nerr := loadProposals(dir)
	if nerr != nil {
		inv.setErr("blocked", "state_dir_unreadable", nerr.Error())
		fmt.Fprintf(stderr, "%s: 读不了提案目录 %s：%v ⇒ 不给结论（「读不到」不当「没有」· §九 M9 RC9）\n", progName, dir, nerr)
		return exitBlocked
	}
	rows := []map[string]string{}
	for _, r := range recs {
		if want != "" && r.State != want {
			continue
		}
		r.Path = filepath.Join(dir, r.ID+".json")
		rows = append(rows, proposalRow(r))
	}
	return listCmd(inv, stdout, stderr, []string{"id", "title", "target", "state", "by", "created_at"}, rows)
}

func cmdDevProposalShow(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) < 2 {
		inv.setErr("usage", "missing_target", "缺提案 id")
		fmt.Fprintf(stderr, "%s: `dev proposal show` 要一个提案 id（先 `zerg dev proposal list`）\n", progName)
		return exitUsage
	}
	id := inv.args[1]
	dir := proposalDir()
	recs, err := loadProposals(dir)
	if err != nil {
		inv.setErr("blocked", "state_dir_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了提案目录 %s：%v ⇒ 不给结论\n", progName, dir, err)
		return exitBlocked
	}
	for _, r := range recs {
		if r.ID != id {
			continue
		}
		r.Path = filepath.Join(dir, r.ID+".json")
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			return selectJSON(stdout, stderr, inv, inv.path, proposalFields, proposalRow(r))
		}
		fmt.Fprintf(stdout, "提案 id  : %s\n", r.ID)
		fmt.Fprintf(stdout, "  标题   : %s\n", r.Title)
		fmt.Fprintf(stdout, "  目标   : %s（回指既有编号 · §17.6 SD1）\n", r.Target)
		fmt.Fprintf(stdout, "  要点   : %s\n", r.Goal)
		fmt.Fprintf(stdout, "  出处   : %s\n", strings.Join(r.Evidence, " · "))
		fmt.Fprintf(stdout, "  退点   : %s\n", r.Rollback)
		fmt.Fprintf(stdout, "  判据   : %s\n", orDash(r.Criterion))
		fmt.Fprintf(stdout, "  提出者 : %s（kind=%s%s）\n", r.Subject, r.SubjectKind, eggSuffix(r.EggID))
		fmt.Fprintf(stdout, "  批准者 : %s（kind=%s）—— 「提 ≠ 批」两个字段 · 批只人给（§九 M18 C4）\n",
			orDash(r.Approver), orDash(r.ApproverKind))
		fmt.Fprintf(stdout, "  状态   : %s\n", r.State)
		fmt.Fprintf(stdout, "  契约号 : %s\n", r.ContractNo)
		fmt.Fprintf(stdout, "  落点   : %s\n", r.Path)
		return exitOK
	}
	// 闭集外 kind 不许用（§九 M7 · 门⑤ R3）：`proposal_not_found` 按 watch.go 同款判 `failed`。
	inv.setErr("failed", "proposal_not_found", "没有这个提案 id")
	fmt.Fprintf(stderr, "%s: 提案目录里没有 %q（先 `zerg dev proposal list` 看现有件）\n", progName, id)
	return exitFail
}

// ---- 落点与读盘 ----

// proposalDir —— 提案件落点：`ZERG_PROPOSAL_DIR` > `~/.zerg/proposals`（**不写死绝对路径**）。
func proposalDir() string {
	if v := strings.TrimSpace(os.Getenv("ZERG_PROPOSAL_DIR")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zerg", "proposals")
}

// loadProposals 读目录里的全部提案件（目录不存在 ⇒ 空清单，不是错 —— 「没有」与「读不到」要分开）。#
func loadProposals(dir string) ([]proposalRecord, error) {
	if dir == "" {
		return nil, fmt.Errorf("状态目录解析不出来")
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		// 目录不存在 = 「还没有提案」（真没有）⇒ 空清单；**其它**读盘错误（权限等）=「读不到」
		// ⇒ 原样上抛，由调用方判 BLOCKED（§九 M9 RC9：读不到不当没有）。
		if os.IsNotExist(err) {
			return []proposalRecord{}, nil
		}
		return nil, err
	}
	out := []proposalRecord{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r proposalRecord
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, fmt.Errorf("%s 解不动：%w", e.Name(), err)
		}
		if r.ID == "" {
			r.ID = strings.TrimSuffix(e.Name(), ".json")
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// nextProposalID —— 下一个提案号（`DEV-NNNN` · **不抢** `P-*` / `U-*` / `SD*` / `MF*` 号空间 ✗）。
func nextProposalID(dir string) string {
	recs, err := loadProposals(dir)
	max := 0
	if err == nil {
		for _, r := range recs {
			var n int
			if _, err := fmt.Sscanf(r.ID, "DEV-%d", &n); err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("DEV-%04d", max+1)
}

// proposalRow —— 一件提案件 → 机器面的一行（字段值一律字符串，**转义走 jstr** 那条路）。
func proposalRow(r proposalRecord) map[string]string {
	return map[string]string{
		"id":              r.ID,
		"title":           r.Title,
		"target":          r.Target,
		"goal":            r.Goal,
		"evidence":        strings.Join(r.Evidence, ","),
		"rollback_ref":    r.Rollback,
		"by":              r.By,
		"subject":         r.Subject,
		"subject_kind":    r.SubjectKind,
		"egg_id":          r.EggID,
		"approver":        r.Approver,
		"approver_kind":   r.ApproverKind,
		"state":           r.State,
		"criterion":       r.Criterion,
		"files":           strings.Join(r.Files, ","),
		"criterion_state": criterionState(r.Criterion),
		"created_at":      r.CreatedAt,
		"path":            r.Path,
	}
}

// ---- 判据可机检（D3b 第一步 · 「提案接校验」）----
//
// 病征（D3b 逐字）：提案面已有（T-58），但「判据」这一格**没人判** —— `--criterion` 此前是**可选**的，
// 填一句人话也照样落件。而 §17.3 铁律② 的三件里，「判据」本身就是判据：**判不出真假的判据 = 没有判据**。
// 本件的落法：把「可机检」写成**一条命令面自己跑得动**的命令行（唯一判定口 = criterionRunnable）。
//
// 三条拒收（一律 rc=2 · 不新立码）：
//
//	① 空 —— 判据为空 ⇒ 不给结论（与「证据为空」同一条口径）；
//	② 首词不是 `zerg` —— 判据的裁决权在地基，别的工具/人话都不算；
//	③ 命令词解析不到命令树里，或落在**本版未开放的危险档**上（跑不到一个绿 ⇒ 不算可机检；
//	   要拿危险档当判据就写它的 `--dry-run` 那一态 —— 那一态永远跑得动、且零副作用）。
func criterionRunnable(spec string) (string, string) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return "", "缺判据（--criterion）—— 判据为空 ⇒ 提案**不给结论**（§17.3 铁律②：判据/证据为空不算判据）"
	}
	fields := strings.Fields(s)
	prog := fields[0]
	if prog != progName && prog != "./bin/"+progName && !strings.HasSuffix(prog, "/"+progName) {
		return "", fmt.Sprintf("判据的首词是 %q —— 判据要**一条命令面跑得动的** `%s …`（人话 / 别的工具都不算判据）", prog, progName)
	}
	words := []string{}
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "-") {
			break
		}
		words = append(words, f)
	}
	if len(words) == 0 {
		return "", "判据里没有命令词（形态：`zerg <对象> <动作> [参数]`）"
	}
	for n := len(words); n >= 1; n-- {
		c := find(words[:n])
		if c == nil {
			continue
		}
		if c.danger != nil && !hasDryRunFlag(fields) {
			return "", fmt.Sprintf("判据里的 `%s` 在本版是**危险档、真跑未开放**（跑不到一个绿）——"+
				" 要拿它当判据，写成它的 `--dry-run` 那一态", strings.Join(c.path, " "))
		}
		return strings.Join(c.path, " "), ""
	}
	return "", fmt.Sprintf("判据里的命令 %q 不在命令树里（与门⑫ 同一条口径：命令示例必须能在命令清单里解析到）",
		strings.Join(words, " "))
}

// hasDryRunFlag 判一条命令串里有没有 `--dry-run`（那一态永远跑得动 · 零副作用）。
func hasDryRunFlag(fields []string) bool {
	for _, f := range fields {
		if f == "--dry-run" || f == "--dry-run=true" {
			return true
		}
	}
	return false
}

// criterionState —— 判据的可跑态（三值 · 给 `--json` 与 check 表用）：可跑 / 缺 / 跑不动。
func criterionState(spec string) string {
	if strings.TrimSpace(spec) == "" {
		return "缺"
	}
	if _, why := criterionRunnable(spec); why != "" {
		return "跑不动"
	}
	return "可跑"
}

// declaredFilesWhy —— 提案声明的件（`--file`）的形状校验：**仓内相对路径**，不许绝对路径 / `..`。
// 它是 `zerg dev edit` 的作用域真源（越界写 ⇒ 2），所以形状必须在**提出时**就判死。
func declaredFilesWhy(files []string) string {
	for _, f := range files {
		switch {
		case f == "":
			return "`--file` 里有空值（声明的件名不许为空）"
		case strings.HasPrefix(f, "/"), filepath.IsAbs(f):
			return fmt.Sprintf("`--file %s` 是绝对路径 —— 作用域只认**仓内相对路径**", f)
		case strings.Contains(f, ".."):
			return fmt.Sprintf("`--file %s` 里有 `..` —— 不许越出仓根（越界写 ⇒ 退码 2）", f)
		}
	}
	return ""
}

// cmdDevProposalCheck —— `zerg dev proposal check [<提案 id>]`：判据**可机检**的复核面。
//
// 退码（fail-closed，与 §九 M9「读不到不当没有」同口径）：
//
//	0 全部件的判据都「可跑」（且至少核到 1 件）
//	2 有件缺判据 / 判据跑不动，或**一件都没核到**（空转 ⇒ 不给结论 · 不静默放绿）
//	8 提案目录读不到（读不到 ⇒ 不给结论）
func cmdDevProposalCheck(inv *invocation, stdout, stderr io.Writer) int {
	only := ""
	if len(inv.args) > 1 {
		only = inv.args[1]
	}
	dir := proposalDir()
	recs, err := loadProposals(dir)
	if err != nil {
		inv.setErr("blocked", "state_dir_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了提案目录 %s：%v ⇒ 不给结论（退码 8）\n", progName, dir, err)
		return exitBlocked
	}
	rows := []map[string]string{}
	bad := 0
	seen := 0
	for _, r := range recs {
		if only != "" && r.ID != only {
			continue
		}
		seen++
		st := criterionState(r.Criterion)
		_, why := criterionRunnable(r.Criterion)
		if st != "可跑" {
			bad++
		}
		rows = append(rows, map[string]string{
			"id": r.ID, "target": r.Target, "state": r.State,
			"criterion": r.Criterion, "criterion_state": st, "why": why,
			"files": strings.Join(r.Files, ","),
		})
	}
	if seen == 0 {
		msg := "一件都没核到（提案目录里没有件"
		if only != "" {
			msg += fmt.Sprintf(" · 也没有 id 为 %q 的件", only)
		}
		msg += "）⇒ **空转不给结论**（退码 2）"
		inv.setErr("usage", "nothing_checked", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "先提一件：%s dev proposal new --title … --target <既有编号> --goal … --evidence <出处> "+
			"--rollback <退点> --criterion '%s gate run --fast'\n", progName, progName)
		return exitUsage
	}
	rc := listCmd(inv, stdout, stderr,
		[]string{"id", "target", "state", "criterion_state", "criterion", "files", "why"}, rows)
	if rc != exitOK {
		return rc
	}
	if bad > 0 {
		inv.setErr("usage", "criterion_not_machine_checkable",
			fmt.Sprintf("%d 件提案的判据缺 / 跑不动", bad))
		fmt.Fprintf(stderr, "%s: %d/%d 件提案的判据**不可机检** ⇒ 不给结论（退码 2）：判据必须是一条本版跑得动的 `%s …`\n",
			progName, bad, seen, progName)
		return exitUsage
	}
	fmt.Fprintf(stderr, "%s: %d 件提案的判据全部**可机检**（每条都能敲一次拿到退码）\n", progName, seen)
	return exitOK
}

// ---- 目标回指（§17.6 SD1）----

// canonTarget 判一个目标是不是「既有编号」：认 `<id>` 与 `待办:<id>` 两种写法（§六 跨表互指口径）。
// 命中 ⇒ 返回规范形（`<id>`）与空 reason；否则 reason 写清为什么不认。
func canonTarget(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "目标是空的"
	}
	// 跨表引用形态：`<台账名>:<本地号>` —— 本件只认「待办」这一本台账（动机源 = 人的清单）
	ledger := ""
	if i := strings.Index(s, ":"); i >= 0 {
		ledger, s = s[:i], s[i+1:]
		if ledger != "待办" {
			return "", fmt.Sprintf("跨表引用只认 `待办:<编号>`（给的是台账 %q）—— 动机源必须是人给的清单（§17.6 SD1）", ledger)
		}
	}
	ids, err := contract.DevTargets()
	if err != nil {
		return "", fmt.Sprintf("回指清单读不出来：%v（**不**退化成「放行」）", err)
	}
	up := strings.ToUpper(s)
	for _, id := range ids {
		if strings.ToUpper(id) == up {
			return id, ""
		}
	}
	return "", fmt.Sprintf("目标 %q 回指不上既有编号（待办 D/E/F/G 族 %d 条）", s, len(ids))
}

// nearestTarget 给一个「最像的既有编号」（K14 四件套：错误文案要给下一步）。
func nearestTarget(s string) string {
	ids, err := contract.DevTargets()
	if err != nil {
		return ""
	}
	up := strings.ToUpper(strings.TrimSpace(s))
	for _, id := range ids {
		u := strings.ToUpper(id)
		if strings.HasPrefix(u, up) || strings.HasPrefix(up, u) {
			return id
		}
	}
	return ""
}

// ---- 小工具 ----

func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ---- 授权面（§九 M18 `C4` · 批 E · T-60）----

// subjectKindDefault —— 不给 `--subject-kind` 时的默认档（提出者是**人**= 最保守的默认：
// 只有显式声明 `ai`/`egg`/`ci` 才落到别的档，不许「不写就当人」以外的猜测）。
const subjectKindDefault = "human"

// judgeAuthz 判授权面四条（`提 ≠ 批` 与「谁不许批」）—— 通过 ⇒ done=false。
//
//	R1 `subject_kind` 必须在四值闭集里（`human`/`ai`/`egg`/`ci`）：精确相等，不做大小写折叠。
//	R2 `subject_kind == egg` ⇒ `--egg-id` **必填**（§18.2 `MF1` 逐字）。
//	R3 给了 `--approver` 且它与 `--subject` **逐字相同** ⇒ 红（**自审自批** —— §九 M16 `V0` 作者 ≠ 审批）。
//	R4 给了 `--approver` 且 `approver_kind != human` ⇒ 红（**批只能人给** —— §九 M18 `C4` ② 段 · §17.6 `SD8`）。
func judgeAuthz(rec proposalRecord, inv *invocation, stderr io.Writer) (int, bool) {
	spec, err := contract.AIBoundary()
	if err != nil {
		inv.setErr("blocked", "contract_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: AI 边界真源读不出来：%v ⇒ 不给结论\n", progName, err)
		return exitBlocked, true
	}
	if !contract.Has(spec.SubjectKinds, rec.SubjectKind) {
		inv.setErr("usage", "bad_subject_kind", "提出者 kind 不在四值闭集里")
		fmt.Fprintf(stderr, "%s: `--subject-kind` %q 不在闭集里（只认 %s）—— 精确相等，不认近义词、不做大小写折叠\n",
			progName, rec.SubjectKind, strings.Join(spec.SubjectKinds, "/"))
		return exitUsage, true
	}
	if rec.SubjectKind == "egg" && rec.EggID == "" {
		inv.setErr("usage", "missing_egg_id", "kind=egg 缺卵 id")
		fmt.Fprintf(stderr, "%s: `--subject-kind egg` 必须带 `--egg-id <卵 id>`（§18.2 `MF1` 逐字：kind=egg 时 egg_id 必填）\n", progName)
		return exitUsage, true
	}
	if rec.Approver != "" && rec.Approver == rec.Subject {
		inv.setErr("usage", "same_subject_approver", "提出者与批准者相同")
		fmt.Fprintf(stderr, "%s: 提出者与批准者**逐字相同**（%q）⇒ 拒（退码 2）—— 「提 ≠ 批」：自审自批的记录退化成自我确认（§九 M16 `V0` 作者 ≠ 审批）\n", progName, rec.Subject)
		return exitUsage, true
	}
	if rec.Approver != "" {
		kind := rec.ApproverKind
		if kind == "" {
			kind = subjectKindDefault
		}
		if kind != "human" {
			inv.setErr("usage", "approver_not_human", "批准者不是人")
			fmt.Fprintf(stderr, "%s: 批准者 kind = %q ⇒ 拒（退码 2）—— **批只能人给**（§九 M18 `C4` ② 段 · §17.6 `SD8`：AI 自评 ≠ 人的验收）\n", progName, kind)
			return exitUsage, true
		}
	}
	return exitOK, false
}

// eggSuffix 只在有卵 id 时补一段（人面上不写空括号）。
func eggSuffix(egg string) string {
	if strings.TrimSpace(egg) == "" {
		return ""
	}
	return " · egg_id=" + egg
}
