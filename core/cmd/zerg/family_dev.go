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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode"

	"github.com/Mr2109/zerg-swarm/core/internal/contract"
)

// proposalFields —— 提案件在 `--json` 面上可取的全部字段（K1：机器面先定）。
var proposalFields = []string{"id", "title", "target", "goal", "evidence", "rollback_ref",
	"by", "state", "criterion", "created_at", "path",
	// D3b 第一步（提案接校验 · 判据可机检）：声明改哪些件 + 判据的可跑态两格。
	"files", "criterion_state",
	// §九 M18 `C4` · 批 E · T-60：授权面两对字段（**只增不改** —— `by` 保留为 `subject` 的兼容别名）。
	"subject", "subject_kind", "egg_id", "approver", "approver_kind",
	// ★ GAP-20260928-161（方案甲 · 本枚）：前瞻判据两格 —— `criterion_new`（值）+ `criterion_new_state`（它的态）。
	//   与 `criterion`/`criterion_state` **同体例**（**只增不改**：强档那两格一字未动）。
	"criterion_new", "criterion_new_state"}

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
	// CriterionNew —— 前瞻判据（GAP-20260928-161 · 方案甲）：本版**还跑不动**的判据
	// （如本提案将要引入的新命令形态）。**分栏**独立于强档 `criterion`（强档一字未动 · 两档不许互相冒充）。
	CriterionNew string `json:"criterion_new"`
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
	// ★ GAP-20260928-95（本枚）：**作废标记**（`--retract` / `--void` 落的）—— 号不复用：件**保留在盘**、
	//   只加标记 ⇒ 下一次 `nextProposalID` 仍数得到它（✗ 不删件 / ✗ 不删号）。四格 `omitempty`：
	//   未作废的件一个字节不变（只增键，既有键不动）。
	Void       bool   `json:"void,omitempty"`        // 作废位
	VoidAt     string `json:"void_at,omitempty"`     // 作废时刻（RFC3339）
	VoidBy     string `json:"void_by,omitempty"`     // 谁作废的
	VoidReason string `json:"void_reason,omitempty"` // 作废理由（留痕）
}

// cmdDevProposal —— `zerg dev proposal <动作>`（动作 = new / list / show）。
func cmdDevProposal(inv *invocation, stdout, stderr io.Writer) int {
	// ★ GAP-20260928-95（本枚）：`--retract <提案 id>` / `--void <提案 id>` 作废档 ——
	//   位置：动作分发**之前**（本档不用动作词；`dev proposal` 是 any-arity，旗标自带点名）。
	//   两个别名任一给了 ⇒ 进作废档（两者同给 / 缺值 / 缺理由的冲突在 `cmdDevProposalRetract` 里判）。
	if inv.hasFlag("--retract") || inv.hasFlag("--void") {
		return cmdDevProposalRetract(inv, stdout, stderr)
	}
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
	case "revise":
		return cmdDevProposalRevise(inv, stdout, stderr)
	case "":
		fmt.Fprintf(stderr, "%s: `dev proposal` 要给动作：new | list | show | check | revise\n", progName)
		fmt.Fprintf(stderr, "用法：zerg dev proposal new --title <题> --target <既有编号（只收 D/E/F/G 族 · 逐条闭集真源 core/internal/contract/dev-targets.json）> --goal <目标> "+
			"--evidence <出处（必填 · 至少一条）—— 两类：① **id 形状**（首部 `GAP-…` / `DEV-…`，如 `GAP-YYYYMMDD-NN`）须**在册**（缺口账 / 在册提案件里查得到）—— 查不到 ⇒ 退码 2「出处 … 在缺口账 / 在册件里查不到 ⇒ 不给结论」；② **描述性证据**（无前缀自由文本）直接收，不要求在册> --rollback <退点件（必填 · 仓外整件路径 —— 退建议照它核 sha256 取改前态）> "+
			"--criterion <判据（必填 · 一条首词是 zerg、本版真跑得动的命令）> [--criterion-new <前瞻判据（可选 · 一条首词是 zerg、本版**还跑不动**的命令形态 ⇒ 显式标前瞻；本版已能真跑的命令写 --criterion）>] [--file <要改的件>]… [--by <提出者>] [--batch <件.json（可机读批量档：JSON 数组 · 逐格一对象 · 键 = 旗标名去 --）>]\n")
		fmt.Fprintf(stderr, "       zerg dev proposal list [--state 未决|已批准|已否决]\n")
		fmt.Fprintf(stderr, "       zerg dev proposal show <提案 id>\n")
		fmt.Fprintf(stderr, "       zerg dev proposal check [<提案 id>] [--deep]   # 判据复核（缺判据/判据跑不动 ⇒ 退码 2；--deep 真跑每件判据 · 报 rc 与命中数）\n")
		fmt.Fprintf(stderr, "       zerg dev proposal revise <提案 id> (--criterion <新判据> | --criterion-new <新前瞻判据>) [--dry-run | --yes]   # 改判据：就地改既有提案的判据（闸与 new 同源）\n")
		fmt.Fprintf(stderr, "       zerg dev proposal (--retract <提案 id> | --void <提案 id>) --reason <理由> [--by <谁>] [--dry-run | --yes]   # 作废：打作废标记（号不复用 · 落审计一行）\n")
		inv.setErr("usage", "missing_action", "缺动作")
		return exitUsage
	default:
		fmt.Fprintf(stderr, "%s: 未知 `dev proposal` 动作 %q\n", progName, action)
		fmt.Fprintf(stderr, "可用：new · list · show · check · revise\n")
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
	// ★ 2026-09-28（缺口 GAP-20260928-77 · 本单）：多余位置参数 ⇒ 用法错 2 · 一个字节不落。
	//   病灶（本单修前现读实据，仓外二进制 + 仓外状态目录）：dev proposal new extraarg ... ⇒ 退 0，
	//   且**真写了提案件**（DEV-0002.json 落盘）—— 多给的那一枚被静默吞、写盘照走。
	//   同族口径（§4.1 K14 四件套 · 缺口 Q-154）：命令面不许把「给错了」读成「给对了」。
	//   位置 = 任何盘面动作**之前**判（同 family_gate_results.go 多余位置参数那一条的体例）。
	if len(inv.args) > 1 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: dev proposal new 不收位置参数（new 之后多给了 %q）—— 字段一律走旗标 · 未写任何件\n", progName, inv.args[1])
		return exitUsage
	}
	// ★ GAP-20260927-413（本枚 · 甲案「可机读批量档」）：`--batch <件.json>` —— 把「批量建提案」
	//   落成**一条可控形态**（读一枚 JSON 数组件 · 逐格走**与单片档完全同一条**闸）。
	//   为什么要有它：`dev proposal new` 在「多旗标 + for 循环 / 管道」拼命令的形态下会卡**调用侧**
	//   的人工应答（调用侧审批层判「无法解析的嵌套执行体」⇒ BLOCKED · 零写入）⇒ 批量建提案时子代理必停手。
	//   本档让调用侧只发**一条**命令（不用 for / 不用管道），批量意图写进**可机读**的声明件里。
	//   定位在此：与单片档同一入口分叉；下面那片单片路径**一字未动**（旧档位行为逐字不变）。
	if strings.TrimSpace(inv.flagVal("--batch")) != "" {
		return cmdDevProposalNewBatch(inv, stdout, stderr)
	}
	rec := proposalRecord{
		Title:     strings.TrimSpace(inv.flagVal("--title")),
		Target:    strings.TrimSpace(inv.flagVal("--target")),
		Goal:      strings.TrimSpace(inv.flagVal("--goal")),
		Rollback:  strings.TrimSpace(inv.flagVal("--rollback")),
		Criterion: strings.TrimSpace(inv.flagVal("--criterion")),
		// ★ GAP-20260928-161（方案甲 · 本枚）：前瞻判据档（本版**还跑不动**的命令形态 · 显式标前瞻）。
		CriterionNew: strings.TrimSpace(inv.flagVal("--criterion-new")),
		By:           strings.TrimSpace(inv.flagVal("--by")),
		Evidence:     trimAll(inv.flagVals("--evidence")),
		Files:        trimAll(inv.flagVals("--file")),
		State:        "未决",
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
		// ★ GAP-20260927-336：撞 rc=2 的那一刻**当场列出全闭集** —— 不必再出仓手抠那个 json。
		fmt.Fprintf(stderr, "%s\n", legalTargetsLine())
		return exitUsage
	}
	rec.Target = canon
	// ③ 退点与出处（判据④：没退点的件不许提；出处为空 ⇒ 不是判据）
	if rec.Rollback == "" {
		inv.setErr("usage", "rollback_required", "缺退点（--rollback）")
		fmt.Fprintf(stderr, "%s: 缺退点（`--rollback <方式>`）—— **没退点的件不许提**（§17.3 铁律③③：每个写动作必须能指回一条回滚路径）\n", progName)
		return exitUsage
	}
	// ③a 退点**现读可得**（`GAP-20261009-01` · fail-closed）：非空之外再判「退点现读拿不拿得到」——
	//   路径形的退点（绝对 / 带目录分隔符）必须**现读得到**（退建议照它核 sha256 取改前态；
	//   读不到的裸名如 `/core__cmd__zerg__family_build.go` 到真回退时取不到任何东西 ⇒ 在这里就拒执）。
	//   ★ 非路径形 = schema 已认的「方式」文本（`--rollback <方式>`）⇒ 按其本义收（✗ 别把合法形态一并拒）。
	if why := rollbackUsableWhy(rec.Rollback); why != "" {
		inv.setErr("usage", "rollback_unusable", why)
		fmt.Fprintf(stderr, "%s: %s\n", progName, why)
		fmt.Fprintf(stderr, "口径（`GAP-20261009-01` · §17.3 铁律③③）：退点要**现读可得** —— 一条**读得到的路径**（退建议照它核 sha256 取改前态）才叫退点；读不到的路径形退点 = 真回退时取不到东西 ⇒ 不给提（退码 2）\n")
		return exitUsage
	}
	if len(rec.Evidence) == 0 {
		inv.setErr("usage", "evidence_required", "缺出处（--evidence）")
		fmt.Fprintf(stderr, "%s: 缺出处（`--evidence <出处>` 至少一条）—— 证据为空的件**不给结论**（§17.3 铁律②③：证据为空 ⇒ 不给结论）\n", progName)
		return exitUsage
	}
	// ③b 出处**在册**（`GAP-20260927-254`）：非空之外再判「该出处 id 在缺口账 / 在册件里真能查到」。
	//    病灶（本单逐字）：:183-187 只判 `len(rec.Evidence)==0`，对**非空**的出处 id 不做任何账内存在性判
	//    ⇒ 可以提一件挂着不存在出处的提案而无人拦。
	//    落法：查不到 ⇒ `rc=2` 不给结论（与 `--criterion` 的可机检**同档** —— 挂空出处的提案与没有人话判据的提案一样，都是噪声）。
	//    位置：与「缺出处」同段（都在**任何盘面动作之前**判）—— 出处不过就不该落件。
	if why := evidenceNotInLedgerWhy(rec.Evidence); why != "" {
		inv.setErr("usage", "evidence_not_in_ledger", why)
		fmt.Fprintf(stderr, "%s: %s\n", progName, why)
		fmt.Fprintf(stderr, "口径（§17.3 铁律② · 与 `--criterion` 同档）：**id 形状**的出处（`GAP-…`/`DEV-…`）要**在册** —— `%s gap ls` 的账里、或在册提案件里查得到；"+
			"查不到的 id = 挂空的证据 ⇒ 不给结论（GAP-20261009-09：描述性证据——无前缀的自由文本——直接收，不走本闸）\n", progName)
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
	// ⑤b 前瞻判据档（★ GAP-20260928-161 · 方案甲）：`--criterion-new` 收的是「**本版还跑不动**」的判据
	//    （如本提案将要引入的新命令形态 `zerg <新命令> …`），并**显式标前瞻**（分栏落盘：`criterion_new` + 它的态）。
	//    两档不许互相冒充（唯一判定口 = criterionForward）：前瞻档收到「本版命令树里能解析到」的命令
	//    ⇒ 拒（那是强档 `--criterion` 的地盘）。位置：与「判据可机检」同段（都在任何盘面动作之前）。
	if rec.CriterionNew != "" {
		if _, why := criterionForward(rec.CriterionNew); why != "" {
			inv.setErr("usage", "criterion_new_not_forward", why)
			fmt.Fprintf(stderr, "%s: %s\n", progName, why)
			fmt.Fprintf(stderr, "口径（★ GAP-20260928-161 · 方案甲）：`--criterion-new` 是**前瞻档** —— 收一条**本版还跑不动**的\n")
			fmt.Fprintf(stderr, "  `%s …` 命令形态（将来才有的命令 · 显式标为前瞻）；本版**已能真跑**的命令请写强档 `--criterion`（两档不许互相冒充）\n", progName)
			return exitUsage
		}
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
	// ★ 授权侧（`GAP-20260927-140` ① · 2026-10-08 拍）：`new` 真写提案件 ⇒ 默认档 fail-closed
	//   （缺 `--yes` ⇒ 退 2、零写）—— `list`/`show`/`check`（只读）不走这条。
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
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

	// ★ GAP-20260928-65（写盘后回读校验这一半）：**写成功 ≠ 写对了** —— 落盘是「先 Marshal 再 Write」，
	//   短写 / 被别的写者截断 / 文件系统异常都会让盘上件与内存里的 `rec` 不是同一份，而此前一路退 0
	//   （静默残废：提案件留着、内容却残缺，谁都不报）。⇒ 写完立刻**逐字节回读对拍**：盘上件必须
	//   **逐字等于**刚写出去的那一份（`body` + 行尾 `\n`，与上面 `f.Write` 的同一份字节）；
	//   不等 ⇒ 退 8（不给结论、**不报成功** —— 与 §九 M3 C5「写不进就不许执行」同一口径）。
	//   ✗ 本节**不**做「反引号 / `$` 防呆」那一半：那要判「shell 有没有在参数进 CLI 之前先吃掉片段」，
	//   而那块**不可在进程内观测**（真值已丢在 shell 里）⇒ 另一半须落在帮助面（`main.go` 的 usage ·
	//   本单禁碰 ⇒ 停手报，不假装做了）。
	expected := string(body) + "\n"
	if back, rerr := os.ReadFile(rec.Path); rerr != nil {
		inv.setErr("failed", "proposal_readback_unreadable", rerr.Error())
		fmt.Fprintf(stderr, "%s: 提案件写完却读不回来 %s：%v（写失败即拒 · 不报成功）\n", progName, rec.Path, rerr)
		return exitFail
	} else if string(back) != expected {
		inv.setErr("failed", "proposal_readback_mismatch", "回读与写入的字节不一致")
		fmt.Fprintf(stderr, "%s: 提案件**回读对拍不一致**（盘上 %d 字节 ≠ 写入 %d 字节）—— 件可能被截断 / 并发改写 ⇒ 报失败、不报成功\n",
			progName, len(back), len(expected))
		return exitFail
	}

	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSON(stdout, stderr, inv, inv.path, proposalFields, proposalRow(rec))
	}
	fmt.Fprintln(stdout, rec.ID)
	fmt.Fprintf(stderr, "提案件落点: %s\n", rec.Path)
	fmt.Fprintf(stderr, "状态: %s（「提 ≠ 批」：批准要人给 · §九 M18 C4）\n", rec.State)
	// 本提案声明件 —— 回执**逐件列出**用户声明的 `--file`（复用上面已解析的 `rec.Files`，不重解析一遍）：
	// 声明的件清单是 `zerg dev edit` 的唯一作用域真源；回执不回显它 ⇒ 提的人看不见自己声明的面。
	// 空声明（没给 `--file`）不加行 —— 既有回执逐字不变。
	if len(rec.Files) > 0 {
		fmt.Fprintf(stderr, "本提案声明件: %s\n", strings.Join(rec.Files, " · "))
	} else {
		// ★ `GAP-20260928-154`：**空声明提案** —— 在**提出的这一刻**当场说清「提得成、但用不了」。
		//   病灶（缺口正文逐字）：「提案不带声明旗标时声明清单为空，随后写件一律退 2 越界写被拒：
		//   提案可提但不可用（今夜两次踩中，废两张提案号）」。
		//   口径：声明的改件清单是 `zerg dev edit` 的唯一作用域真源（empty ⇒ 无件可写）——
		//   这一条与 `dev edit` 侧那句「越界写被拒 … 声明的是：（未声明）」同源，不新立第二套说法。
		//   ★ **退码一字不变**（提得成仍退 0）：「当场拒提」= 把 0 改成非 0 ＝ **行为面** ⇒ 登记待拍，
		//   不在本单擅动（见回执「待拍项」）。
		fmt.Fprintf(stderr, "★ 警告：本提案**声明件为空**（没给 `--file`）—— 提案**提得成、但用不了**：\n")
		fmt.Fprintf(stderr, "  `dev edit --proposal %s --file <件>` 对**任何**件都会拒执（**越界写被拒** · 退码 2）——\n", rec.ID)
		fmt.Fprintf(stderr, "  声明的改件清单（`--file <仓内相对路径>`）是 `dev edit` 的**唯一作用域真源**：清单为空 ⇒ 一个件也写不进去。\n")
		fmt.Fprintf(stderr, "  下一步：带上声明重提一枚（`--file` 可多枚）；这一枚 %s **请作废，别拿去写件**（带了声明的那一条路不受影响）。\n", rec.ID)
	}
	fmt.Fprintf(stderr, "零副作用口径：本命令**不触主控**（零 HTTP）· **不写入仓** —— 产出后系统状态逐字不变（§17.3 铁律③ 判据）\n")
	return exitOK
}

// ---- 批量档（★ GAP-20260927-413 · 甲案「可机读批量档」）----

// proposalBatchKeys —— `--batch` 件里每一格**认得的键**（键 = 单片档旗标名去掉前导 `--`）。
// 与单片档那一排旗标**同源**：认不得的键 ⇒ 点名到格（不静默吞 · §4.1 K14 四件套）。
var proposalBatchKeys = []string{
	"title", "target", "goal", "evidence", "rollback", "criterion", "criterion-new",
	"file", "by", "subject", "subject-kind", "egg-id", "approver", "approver-kind",
}

// proposalBatchKnownKey 判一枚键在不在上面那张表里。
func proposalBatchKnownKey(k string) bool {
	for _, x := range proposalBatchKeys {
		if x == k {
			return true
		}
	}
	return false
}

// proposalSingleFlagsPresent 收出这一发里**同时给了**的单片档旗标（`--batch` 与它们互斥）。
func proposalSingleFlagsPresent(inv *invocation) []string {
	hit := []string{}
	for _, k := range proposalBatchKeys {
		if inv.hasFlag("--" + k) {
			hit = append(hit, "--"+k)
		}
	}
	return hit
}

// proposalBatchItemInvocation 把 `--batch` 件里的**一格**摊成一次「单片档」的合成 invocation
// （键 = 旗标名去 `--` · 值 = 字符串或字符串数组）—— 逐格复用单片档那条**唯一**的路，不另写第二套闸。
// why 非空 ⇒ 这一格形状不合法（点名到底）。
func proposalBatchItemInvocation(parent *invocation, idx int, item map[string]json.RawMessage) (*invocation, string) {
	s := &invocation{path: parent.path, args: []string{"new"}, kv: map[string][]string{}}
	for k, raw := range item {
		if !proposalBatchKnownKey(k) {
			return nil, fmt.Sprintf("`--batch` 第 %d 格认不得的键 %q —— 认得的键（= 单片档旗标名去 `--`）：%s",
				idx+1, k, strings.Join(proposalBatchKeys, " · "))
		}
		vals, why := proposalBatchKeyVals(raw)
		if why != "" {
			return nil, fmt.Sprintf("`--batch` 第 %d 格的键 %q %s", idx+1, k, why)
		}
		s.kv["--"+k] = append(s.kv["--"+k], vals...)
	}
	return s, ""
}

// proposalBatchKeyVals 收一格的键值（字符串 / 字符串数组两形态 —— 别的形态拒执，不猜）。
func proposalBatchKeyVals(raw json.RawMessage) ([]string, string) {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return []string{one}, ""
	}
	var many []string
	if json.Unmarshal(raw, &many) == nil {
		return many, ""
	}
	return nil, "必须是字符串或字符串数组"
}

// cmdDevProposalNewBatch —— `dev proposal new --batch <件.json>`：可机读的**批量建提案**档。
//
// 缺口（GAP-20260927-413）正文逐字给的病征：`dev proposal new` 在**多旗标 + for 循环 / 管道**形态下
// 会卡**调用侧**的人工应答（调用侧审批层判「无法解析的嵌套执行体」⇒ BLOCKED · 零写入）⇒ 批量建提案时
// 子代理必停手。本档给的是缺口要的**那条出口**：批量意图落进一枚**可机读**的声明件、由命令面自己摊开
// —— 调用侧只发**一条**命令（不用 for / 不用管道 / 不拼多段命令 · 那正是挂住的那一形态）。
//
// 三条口径（与缺口硬线逐条对齐 · 一条都不放宽）：
//
//	① **显式声明**：批量意图只从 `--batch <件>` 来（与单片旗标**互斥** · 同给 ⇒ 退 2 点名）；
//	   件必须是 JSON **数组**、每格一个对象；认不得的键 / 非字符串值 ⇒ 退 2 **点名到格**。
//	② **不放宽审批语义**：每一格走**与单片档完全同一条**闸（同一函数 —— 同一条目标回指 / 退点现读 /
//	   出处 / 判据 / 声明件路），真写仍要 `--yes`（同一 `writeGuardYes` 门）。本档只做**调度**：
//	   不新开闸、不跳闸、不并批签。
//	③ **不许静默成功 / 不许挂住**：**先全验后全写** —— 任何一格不过闸 ⇒ **一枚未写** + 点名该格
//	   （不是「跳过坏的、写好的」）；件里 0 格 ⇒ 退 2（空批 ≠「建成了 0 枚」）；写入途中失败 ⇒
//	   照实报本轮已写几枚（不吞、不报「全成」）。
func cmdDevProposalNewBatch(inv *invocation, stdout, stderr io.Writer) int {
	specPath := strings.TrimSpace(inv.flagVal("--batch"))
	// ① 与单片旗标互斥（意图不唯一 ⇒ 不猜、不静默选一路）
	if dup := proposalSingleFlagsPresent(inv); len(dup) > 0 {
		msg := fmt.Sprintf("`--batch` 与单片旗标互斥（同时给了：%s）—— 批量档的每一格写进 `--batch` 的件里",
			strings.Join(dup, " · "))
		inv.setErr("usage", "batch_conflicts_single", msg)
		fmt.Fprintf(stderr, "%s: %s\n", progName, msg)
		fmt.Fprintf(stderr, "⇒ 退 2 · 一枚未写（要么单片（逐枚旗标）、要么批量（一枚件）· 不许混着给）\n")
		return exitUsage
	}
	// ② 读件（读不到 / 解不动 / 不是数组 / 0 格 ⇒ 一律退 2 · 不给结论）
	raw, err := os.ReadFile(specPath)
	if err != nil {
		inv.setErr("usage", "batch_unreadable", "--batch 件读不到")
		fmt.Fprintf(stderr, "%s: `--batch` 件读不到 %s：%v ⇒ 不给结论（退 2 · 本批一枚未写）\n", progName, specPath, err)
		return exitUsage
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		inv.setErr("usage", "batch_unparsable", "--batch 件解不动")
		fmt.Fprintf(stderr, "%s: `--batch` 件解不动 %s：%v ⇒ 不给结论（退 2 · 本批一枚未写）\n", progName, specPath, err)
		fmt.Fprintf(stderr, "件形态：JSON **数组** · 每格一个对象 · 键 = 旗标名去 `--` · 例：\n")
		fmt.Fprintf(stderr, "  [{\"title\":\"…\",\"target\":\"D3b\",\"goal\":\"…\",\"evidence\":[\"GAP-…\"],\"rollback\":\"/…\",\"criterion\":\"%s gate run --fast\",\"file\":[\"core/…\"]}]\n", progName)
		return exitUsage
	}
	if len(items) == 0 {
		inv.setErr("usage", "batch_empty", "--batch 件里 0 条")
		fmt.Fprintf(stderr, "%s: `--batch` 件里 **0 条** ⇒ 不给结论（退 2 · 本批一枚未写；空批 ≠「建成了 0 枚」的静默成功）\n", progName)
		return exitUsage
	}
	// ③ 逐格摊成合成 invocation（认不得的键 / 坏值 ⇒ 退 2 点名到格）
	synths := make([]*invocation, 0, len(items))
	for i, it := range items {
		s, why := proposalBatchItemInvocation(inv, i, it)
		if why != "" {
			inv.setErr("usage", "batch_bad_item", why)
			fmt.Fprintf(stderr, "%s: %s\n", progName, why)
			fmt.Fprintf(stderr, "⇒ 退 2 · 本批**一枚未写**（逐格先验后写 · 不跳过坏的写好的）\n")
			return exitUsage
		}
		synths = append(synths, s)
	}
	// ④ 第一趟「全验」：每格走单片档的 `--dry-run` 那一态（跑完全部闸 · 零副作用）
	for i, s := range synths {
		s.dryRun, s.yes = true, false
		var o, e strings.Builder
		if rc := cmdDevProposalNew(s, &o, &e); rc != exitOK {
			inv.setErr("usage", "batch_item_refused", "本批有格不过闸")
			fmt.Fprintf(stderr, "%s: `--batch` **第 %d 格不过闸** ⇒ 本批**一枚未写**（逐格先验后写）：\n%s", progName, i+1, e.String())
			return rc
		}
	}
	// ⑤ 干跑档：全验已过 ⇒ 只回执「将建 N 枚」，零写（与单片档 `--dry-run` 同一条口径）
	if inv.dryRun {
		fmt.Fprintf(stdout, "（--dry-run）批量档：将建 **%d** 枚提案（逐格全验已过 · 零写入 · 未触主控 · 未写仓）\n", len(synths))
		rows := make([]map[string]string, 0, len(synths))
		for i, s := range synths {
			rows = append(rows, map[string]string{
				"id":     fmt.Sprintf("（第 %d 格 · --dry-run 未分配落点）", i+1),
				"target": s.flagVal("--target"), "title": s.flagVal("--title")})
		}
		return listCmd(inv, stdout, stderr, []string{"id", "target", "title"}, rows)
	}
	// ⑥ 真写前置：`--yes`（与单片档同一条 fail-closed 门 —— 本档只调度、不新开闸）
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	// ⑦ 第二趟「逐格真写」（同一写入 + 回读对拍；途中失败 ⇒ 照实报本轮已写几枚 · 不吞）
	ids := make([]string, 0, len(synths))
	for i, s := range synths {
		s.dryRun, s.yes = false, true
		var o, e strings.Builder
		if rc := cmdDevProposalNew(s, &o, &e); rc != exitOK {
			fmt.Fprintf(stderr, "%s: `--batch` **第 %d 格写入失败**（本批已写 %d 枚 · 逐字）：\n%s", progName, i+1, len(ids), e.String())
			for _, id := range ids {
				fmt.Fprintln(stdout, id)
			}
			return rc
		}
		// 单片档写在 stderr 上的逐格回执（含「声明件为空」那类警告）**照转** —— 不吞警告。
		if e.Len() > 0 {
			fmt.Fprint(stderr, e.String())
		}
		ids = append(ids, strings.TrimSpace(o.String()))
	}
	// ⑧ 回执：机器面走 `listCmd`（与 `dev proposal list` 同一渲染 · 全字段可取）；人面逐行印 id（与单片档同形）。
	dir := proposalDir()
	recs, lerr := loadProposals(dir)
	if lerr != nil {
		inv.setErr("blocked", "state_dir_unreadable", lerr.Error())
		fmt.Fprintf(stderr, "%s: 批量档已写 %d 枚，但回读不了提案目录 %s：%v ⇒ 不给结论（退码 8）\n", progName, len(ids), dir, lerr)
		return exitBlocked
	}
	rows := []map[string]string{}
	for _, r := range recs {
		if containsStr(ids, r.ID) {
			r.Path = filepath.Join(dir, r.ID+".json")
			rows = append(rows, proposalRow(r))
		}
	}
	sort.Slice(rows, func(a, b int) bool { return rows[a]["id"] < rows[b]["id"] })
	if inv.jsonGiven {
		return listCmd(inv, stdout, stderr, []string{"id", "title", "target", "state", "by", "created_at"}, rows)
	}
	for _, r := range rows {
		fmt.Fprintln(stdout, r["id"])
	}
	fmt.Fprintf(stderr, "批量档：**%d 枚**提案已落 %s（逐格先验后写 · 全验过才写 · 「提 ≠ 批」：批准要人给）\n", len(rows), dir)
	fmt.Fprintf(stderr, "零副作用口径：本命令**不触主控**（零 HTTP）· **不写入仓** —— 产出后系统仓状态逐字不变（§17.3 铁律③ 判据）\n")
	return exitOK
}

// ---- ★ GAP-20260928-95：`dev proposal --retract <提案 id>` / `--void <提案 id>` —— 作废档 ----

// proposalRetractEvent —— 本档审计事件名（**自己的事件名** · 与现网 `edit` 事件不同名）。
const proposalRetractEvent = "proposal_retract"

// proposalRetractAudit —— 作废审计的一行（落**同一件** `edit_audit.jsonl` · **自己的字段名** ·
// 体例照同族的 `gapAuditLine` / `scriptRetireAudit`：追加只写（`O_APPEND`）· 缺必需格 ⇒ 这一行不写）。
// 为什么单列自己的字段名：现网 `edit_audit.jsonl` 那几格讲的是**改件**（`before_sha256` / `after_sha256`
// 记的是被改文件的前后 hash），本事件讲的是**作废一件提案** —— 两件事不是同一件事（`[P13]` 那条陷阱）。
type proposalRetractAudit struct {
	At          string `json:"at"`           // 时刻（RFC3339）
	Event       string `json:"event"`        // proposal_retract
	Proposal    string `json:"proposal"`     // 被作废的提案号（号仍占 · 不复用）
	ProposalDir string `json:"proposal_dir"` // 当趟真用的提案目录（回指授权物 · 同 GAP-20260928-79 口径）
	Reason      string `json:"reason"`       // 作废理由（留痕）
	By          string `json:"by"`           // 谁作废的
	Confirm     string `json:"confirm"`      // 本机名（真写档的确认值）
}

// proposalRetractAppendAudit —— 追加一行（`O_APPEND` · 历史行逐字节不变 · **失败即拒**）。
// 落点与 `dev edit` / `gap` 族同一件：`editAuditPath()`（唯一真源 · 本件不另抄落点式子）。
func proposalRetractAppendAudit(path string, l proposalRetractAudit) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
	}
	// 缺必需格 ⇒ 这一行不许写（体例照 gap 族 / scriptRetire）。
	if strings.TrimSpace(l.At) == "" || strings.TrimSpace(l.Event) == "" ||
		strings.TrimSpace(l.Proposal) == "" || strings.TrimSpace(l.Reason) == "" {
		return fmt.Errorf("审计行缺必需格 ⇒ 这一行不许写")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(l)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(body, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// cmdDevProposalRetract —— `--retract <提案 id>` / `--void <提案 id>` 的真写面。
//
// 硬线（缺口 GAP-20260928-95）：**作废必须留痕且号不复用** —— 只往提案件加 `void` 标记（件保留在盘 ⇒
// 下一次 `nextProposalID` 仍数得到它 ⇒ ✗ 号不复用）；审计落**一行**（`edit_audit.jsonl` · 体例照既有）。
//
// 退码（同族写面口径 · 不新立码）：
//
//	0 落账或幂等「无变化」· 2 缺值 / 缺理由 / 提案不存在 / 两别名同给 · 8 状态目录读不到。
func cmdDevProposalRetract(inv *invocation, stdout, stderr io.Writer) int {
	// ① 两个别名同给 ⇒ 一条命令一个动作（不猜 · 用法错 2）。
	givenR, givenV := inv.hasFlag("--retract"), inv.hasFlag("--void")
	if givenR && givenV {
		inv.setErr("usage", "retract_void_conflict", "--retract 与 --void 同给")
		fmt.Fprintf(stderr, "%s: `--retract` 与 `--void` 是**同一个档的两个名字** ⇒ 不许同给（一条命令一个动作）\n", progName)
		fmt.Fprintf(stderr, "  作废：%s dev proposal --retract <提案 id> --reason <理由> [--by <谁>] [--dry-run | --yes]\n", progName)
		return exitUsage
	}
	id := strings.TrimSpace(inv.flagVal("--retract"))
	if id == "" {
		id = strings.TrimSpace(inv.flagVal("--void"))
	}
	// ② 裸给（旗标给了却不给值）⇒ 不许静默降级（同族先例：`gap note --retract` 的 GAP-20260929-27）。
	if id == "" {
		inv.setErr("usage", "retract_missing_value", "给了作废旗标却点不出提案号")
		fmt.Fprintf(stderr, "%s: 给了 `--retract`/`--void` 却**没给值** —— 要点名一枚提案号（`DEV-NNNN`）；裸给**不降级**成别档\n", progName)
		fmt.Fprintf(stderr, "  作废：%s dev proposal --retract <提案 id> --reason <理由> [--by <谁>] [--dry-run | --yes]\n", progName)
		return exitUsage
	}
	// ③ 动作词与作废档同给 ⇒ 冲突（意图不唯一）。
	if len(inv.args) > 0 {
		inv.setErr("usage", "retract_with_action", "作废档与动作词同给")
		fmt.Fprintf(stderr, "%s: `--retract`/`--void` 与动作词 %q **不许同给**（作废不用动作词）\n", progName, inv.args[0])
		return exitUsage
	}
	// ④ 理由必填（作废要留痕：谁、为什么）—— 缺 ⇒ 按既有「缺必需旗标」档拒 2（照 `new` 同档）。
	reason := strings.TrimSpace(inv.flagVal("--reason"))
	if reason == "" {
		inv.setErr("usage", "retract_reason_required", "作废缺理由（--reason）")
		fmt.Fprintf(stderr, "%s: 作废要**留痕** —— 缺理由（`--reason <一句话>`）⇒ 退 2（照 `new` 的「缺必需旗标」同档）\n", progName)
		fmt.Fprintf(stderr, "  作废：%s dev proposal --retract <提案 id> --reason <理由> [--by <谁>] [--dry-run | --yes]\n", progName)
		return exitUsage
	}
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = "（未声明）"
	}
	dir := proposalDir()
	if dir == "" {
		inv.setErr("blocked", "no_state_dir", "解析不到状态目录（HOME 取不到）")
		fmt.Fprintf(stderr, "%s: 解析不到状态目录（HOME 取不到 · 可用 ZERG_PROPOSAL_DIR 指定）⇒ 不给结论\n", progName)
		return exitBlocked
	}
	recs, err := loadProposals(dir)
	if err != nil {
		inv.setErr("blocked", "state_dir_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了提案目录 %s：%v ⇒ 不给结论（退码 8）\n", progName, dir, err)
		return exitBlocked
	}
	idx := -1
	for i := range recs {
		if recs[i].ID == id {
			idx = i
			break
		}
	}
	// ⑤ 提案不存在 ⇒ 同族写面（`revise` / `dev edit`）同口径：用法错 2（写必须有可审查物）。
	if idx < 0 {
		inv.setErr("usage", "proposal_not_found", "没有这个提案 id")
		fmt.Fprintf(stderr, "%s: 提案目录里没有 %q ⇒ 拒执（作废也要有可审查物 ⇒ 先 `%s dev proposal list`）\n", progName, id, progName)
		return exitUsage
	}
	rec := recs[idx]
	rec.Path = filepath.Join(dir, id+".json")
	// ⑥ 幂等：已在作废位 ⇒ 「无变化」0（不写件、不写审计）。
	if rec.Void {
		inv.changed = boolPtr(false)
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			return selectJSON(stdout, stderr, inv, inv.path, proposalFields, proposalRow(rec))
		}
		fmt.Fprintf(stdout, "无变化 %s · **已在作废位**（void:true · 幂等命中：不写件、不写审计）\n", id)
		return exitOK
	}
	// ⑦ `--dry-run`：只出计划件、零副作用（与同族 `new`/`revise` 同一条三态口径）。
	if inv.dryRun {
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			return selectJSON(stdout, stderr, inv, inv.path, proposalFields, proposalRow(rec))
		}
		fmt.Fprintf(stdout, "提案 id  : %s（--dry-run 未写盘）\n", rec.ID)
		fmt.Fprintf(stdout, "  作废   : void=false → true（**只加标记** · 件保留在盘 ⇒ 号不复用）\n")
		fmt.Fprintf(stdout, "  理由   : %s（by=%s）\n", reason, by)
		fmt.Fprintf(stdout, "  状态   : %s（**不改** —— 作废只加标记）\n", rec.State)
		fmt.Fprintf(stderr, "（--dry-run：只出件 · 零副作用 —— 未改任何文件、未写审计）\n")
		return exitOK
	}
	// ⑧ 真写前置：`--yes`（与 `new`/`revise` 同一条 fail-closed 门）。
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	// ⑨ `--json` 字段面前置判定（先校后写）：坏字段 ⇒ 未改件、未写审计。
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
	}
	rec.Void = true
	rec.VoidAt = time.Now().Format(time.RFC3339)
	rec.VoidBy = by
	rec.VoidReason = reason
	body, merr := json.MarshalIndent(rec, "", " ")
	if merr != nil {
		inv.setErr("failed", "marshal_failed", merr.Error())
		fmt.Fprintf(stderr, "%s: 件序列化失败：%v\n", progName, merr)
		return exitFail
	}
	// ⑩ 真写：**审计先落盘**（写不进审计 ⇒ 不改件 · 体例照 `dev edit`）⇒ 再原子改件 ⇒ 回读对拍。
	auditPath := editAuditPath()
	line := proposalRetractAudit{
		At: time.Now().Format(time.RFC3339), Event: proposalRetractEvent,
		Proposal: rec.ID, ProposalDir: dir, Reason: reason, By: by, Confirm: inv.confirm,
	}
	if aerr := proposalRetractAppendAudit(auditPath, line); aerr != nil {
		inv.setErr("failed", "audit_unwritable", aerr.Error())
		fmt.Fprintf(stderr, "%s: **审计落不下盘 ⇒ 拒执**（写不进日志就不改件）：%v\n", progName, aerr)
		return exitFail
	}
	tmp := rec.Path + ".retract.tmp"
	f, terr := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if terr != nil {
		inv.setErr("failed", "proposal_write_failed", terr.Error())
		fmt.Fprintf(stderr, "%s: 写不进临时件 %s：%v（写失败即拒 · 不吞）\n", progName, tmp, terr)
		return exitFail
	}
	if _, werr := f.Write(append(body, '\n')); werr != nil {
		f.Close()
		_ = os.Remove(tmp)
		inv.setErr("failed", "proposal_write_failed", werr.Error())
		fmt.Fprintf(stderr, "%s: 写作废件失败：%v（审计已留痕：%s）\n", progName, werr, auditPath)
		return exitFail
	}
	f.Close()
	if rerr := os.Rename(tmp, rec.Path); rerr != nil {
		_ = os.Remove(tmp)
		inv.setErr("failed", "proposal_write_failed", rerr.Error())
		fmt.Fprintf(stderr, "%s: 落盘失败（rename %s → %s）：%v（审计已留痕：%s）\n", progName, tmp, rec.Path, rerr, auditPath)
		return exitFail
	}
	expected := string(body) + "\n"
	if back, rerr := os.ReadFile(rec.Path); rerr != nil {
		inv.setErr("failed", "proposal_readback_unreadable", rerr.Error())
		fmt.Fprintf(stderr, "%s: 作废件写完却读不回来 %s：%v（写失败即拒 · 不报成功）\n", progName, rec.Path, rerr)
		return exitFail
	} else if string(back) != expected {
		inv.setErr("failed", "proposal_readback_mismatch", "回读与写入的字节不一致")
		fmt.Fprintf(stderr, "%s: 作废件**回读对拍不一致**（盘上 %d 字节 ≠ 写入 %d 字节）⇒ 报失败、不报成功\n", progName, len(back), len(expected))
		return exitFail
	}
	inv.changed = boolPtr(true)
	if inv.jsonGiven {
		return selectJSON(stdout, stderr, inv, inv.path, proposalFields, proposalRow(rec))
	}
	fmt.Fprintf(stdout, "已作废 %s（void:true · **号不复用**：件保留在盘，nextProposalID 仍数得到它）\n", rec.ID)
	fmt.Fprintf(stdout, "  理由   : %s（by=%s）\n", reason, by)
	fmt.Fprintf(stdout, "  真源   : %s（**不删件** · 只加标记）\n", rec.Path)
	fmt.Fprintf(stdout, "  审计   : %s（event=%s · 一行）\n", auditPath, proposalRetractEvent)
	fmt.Fprintf(stderr, "零副作用口径：本命令**不触主控**（零 HTTP）· **不写入仓** —— 只改本提案一份（状态目录）\n")
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
		// ★ GAP-20260928-156（2026-10-04 本枚）：**空声明**在读回面**顶到最前**（回显而非警示）。
		//   病灶（缺口逐字）：空声明提案「提得成（废号照旧产生）」，而**读回面**（`show`）看完只当寻常件
		//   回显 ⇒ 读者看不出它其实**用不了**（`dev edit` 对任何件都拒）。`new` 的回执已有警告（同批），
		//   此处把同一件事搬到 show 面**最前**：声明件为空 ⇒ 紧接着 id 顶一行，一眼看见。
		if len(r.Files) == 0 {
			fmt.Fprintf(stdout, "★ 警告   : 本提案**声明件为空** —— 提得成、但**用不了**：`dev edit --proposal %s --file <件>` 对**任何**件都拒执（**越界写被拒** · 退码 2）；请重提一枚带 `--file` 的（旧号作废）\n", r.ID)
		}
		fmt.Fprintf(stdout, "  标题   : %s\n", r.Title)
		fmt.Fprintf(stdout, "  目标   : %s（回指既有编号 · §17.6 SD1）\n", r.Target)
		fmt.Fprintf(stdout, "  要点   : %s\n", r.Goal)
		fmt.Fprintf(stdout, "  出处   : %s\n", strings.Join(r.Evidence, " · "))
		fmt.Fprintf(stdout, "  退点   : %s\n", r.Rollback)
		fmt.Fprintf(stdout, "  判据   : %s\n", orDash(r.Criterion))
		fmt.Fprintf(stdout, "  提出者 : %s（kind=%s%s）\n", r.Subject, r.SubjectKind, eggSuffix(r.EggID))
		fmt.Fprintf(stdout, "  批准者 : %s（kind=%s）—— 「提 ≠ 批」两个字段 · 批只人给（§九 M18 C4）\n",
			orDash(r.Approver), orDash(r.ApproverKind))
		// ★ GAP-20260928-95（本枚）：作废标记在读回面顶出（号仍在 · 只标作废）。
		if r.Void {
			fmt.Fprintf(stdout, "  ★作废   : 是（%s · by %s · 理由：%s）—— 号不复用：本件保留在盘，勿拿去写件\n",
				r.VoidAt, r.VoidBy, r.VoidReason)
		}
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

// ---- ★ GAP-20260927-267：`dev proposal revise <提案 id>` —— 提案的「改判据」动作 ----
//
// 病灶（缺口逐字）：「提案无改判据动作（只有 new|list|show|check）⇒ 判据一旦落件即固定，
// 偏离只能新提案覆盖」。⇒ 本动作给既有提案**就地改判据**的出口：改 `criterion` / `criterion-new`
// 两格（其余字段一字不动），闸与 `new` **同源**（复用 criterionRunnable / criterionForward 两个
// 唯一判定口 · ✗ 不另造第二套校验）。退码与同族写面 `dev edit` 同口径：提案不存在 ⇒ 用法错 2。
func cmdDevProposalRevise(inv *invocation, stdout, stderr io.Writer) int {
	if len(inv.args) < 2 {
		inv.setErr("usage", "missing_target", "缺提案 id")
		fmt.Fprintf(stderr, "%s: `dev proposal revise` 要一个提案 id（先 `zerg dev proposal list`）\n", progName)
		return exitUsage
	}
	if len(inv.args) > 2 {
		inv.setErr("usage", "extra_args", "多余位置参数")
		fmt.Fprintf(stderr, "%s: dev proposal revise 只收一个提案 id（多给了 %q）\n", progName, inv.args[2])
		return exitUsage
	}
	id := inv.args[1]
	newCrit := strings.TrimSpace(inv.flagVal("--criterion"))
	newCritNew := strings.TrimSpace(inv.flagVal("--criterion-new"))
	// ① 至少给一格（都不给 ⇒ 没改任何东西 ⇒ 用法错 2 · 零写）
	if newCrit == "" && newCritNew == "" {
		inv.setErr("usage", "nothing_to_revise", "没给要改的判据")
		fmt.Fprintf(stderr, "%s: `dev proposal revise` 至少要给 `--criterion` 或 `--criterion-new` 之一（都没给 = 没改任何东西 ⇒ 退 2 · 零写）\n", progName)
		return exitUsage
	}
	// ② 判据校验 —— 与 `new` 同源的两个唯一判定口（强档 criterionRunnable / 前瞻档 criterionForward）。
	if newCrit != "" {
		if _, why := criterionRunnable(newCrit); why != "" {
			inv.setErr("usage", "criterion_not_machine_checkable", why)
			fmt.Fprintf(stderr, "%s: %s\n", progName, why)
			fmt.Fprintf(stderr, "口径（§17.3 铁律② · 与 `new` 同源）：改后的判据要**可机检** —— 一条本版跑得动的 `%s …`，例：%s gate run --fast\n", progName, progName)
			return exitUsage
		}
	}
	if newCritNew != "" {
		if _, why := criterionForward(newCritNew); why != "" {
			inv.setErr("usage", "criterion_new_not_forward", why)
			fmt.Fprintf(stderr, "%s: %s\n", progName, why)
			fmt.Fprintf(stderr, "口径（★ GAP-20260928-161 · 方案甲 · 与 `new` 同源）：`--criterion-new` 是**前瞻档** —— 收一条本版**还跑不动**的 `%s …` 命令形态；本版已能真跑的命令请写强档 `--criterion`（两档不许互相冒充）\n", progName)
			return exitUsage
		}
	}
	dir := proposalDir()
	if dir == "" {
		inv.setErr("blocked", "no_state_dir", "解析不到状态目录（HOME 取不到）")
		fmt.Fprintf(stderr, "%s: 解析不到状态目录（HOME 取不到 · 可用 ZERG_PROPOSAL_DIR 指定）⇒ 不给结论\n", progName)
		return exitBlocked
	}
	recs, err := loadProposals(dir)
	if err != nil {
		inv.setErr("blocked", "state_dir_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了提案目录 %s：%v ⇒ 不给结论（退码 8）\n", progName, dir, err)
		return exitBlocked
	}
	idx := -1
	for i := range recs {
		if recs[i].ID == id {
			idx = i
			break
		}
	}
	// ③ 提案不存在 ⇒ 同族写面（`dev edit`）同口径：用法错 2（写必须有可审查物）。
	if idx < 0 {
		inv.setErr("usage", "proposal_not_found", "没有这个提案 id")
		fmt.Fprintf(stderr, "%s: 提案目录里没有 %q ⇒ 拒执（写必须有可审查物 ⇒ 先 `%s dev proposal list`）\n", progName, id, progName)
		return exitUsage
	}
	rec := recs[idx]
	rec.Path = filepath.Join(dir, rec.ID+".json")
	oldCrit, oldCritNew := rec.Criterion, rec.CriterionNew
	if newCrit != "" {
		rec.Criterion = newCrit
	}
	if newCritNew != "" {
		rec.CriterionNew = newCritNew
	}
	// ④ `--dry-run`：只出计划件、零副作用（与单片 `new` 同一条三态口径）
	if inv.dryRun {
		if inv.jsonGiven {
			if rc := requireFields(inv, stderr); rc != exitOK {
				return rc
			}
			return selectJSON(stdout, stderr, inv, inv.path, proposalFields, proposalRow(rec))
		}
		fmt.Fprintf(stdout, "提案 id  : %s（--dry-run 未写盘）\n", rec.ID)
		fmt.Fprintf(stdout, "  判据   : %s → %s\n", orDash(oldCrit), orDash(rec.Criterion))
		fmt.Fprintf(stdout, "  前瞻   : %s → %s\n", orDash(oldCritNew), orDash(rec.CriterionNew))
		fmt.Fprintf(stderr, "（--dry-run：只出件 · 零副作用 —— 未改任何文件）\n")
		return exitOK
	}
	// ⑤ 真写前置：`--yes`（与 `new` 同一条 fail-closed 门）
	if rc := writeGuardYes(inv, stderr); rc != exitOK {
		return rc
	}
	body, err := json.MarshalIndent(rec, "", " ")
	if err != nil {
		inv.setErr("failed", "marshal_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 件序列化失败：%v\n", progName, err)
		return exitFail
	}
	// ⑥ 原子改写：同目录临时件 + `os.Rename`（就地覆盖 —— `new` 是「追加只写·不覆盖已有」，改判据就是改这一件）
	tmp := rec.Path + ".revise.tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		inv.setErr("failed", "proposal_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写不进临时件 %s：%v（写失败即拒 · 不吞）\n", progName, tmp, err)
		return exitFail
	}
	if _, err := f.Write(append(body, '\n')); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		inv.setErr("failed", "proposal_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写临时件失败：%v\n", progName, err)
		return exitFail
	}
	f.Close()
	if err := os.Rename(tmp, rec.Path); err != nil {
		_ = os.Remove(tmp)
		inv.setErr("failed", "proposal_write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 落盘失败（rename %s → %s）：%v\n", progName, tmp, rec.Path, err)
		return exitFail
	}
	// ⑦ 回读对拍（★ GAP-20260928-65 同口径）：写成功 ≠ 写对了 —— 盘上件必须逐字等于刚写出去的字节。
	expected := string(body) + "\n"
	if back, rerr := os.ReadFile(rec.Path); rerr != nil {
		inv.setErr("failed", "proposal_readback_unreadable", rerr.Error())
		fmt.Fprintf(stderr, "%s: 提案件写完却读不回来 %s：%v（写失败即拒 · 不报成功）\n", progName, rec.Path, rerr)
		return exitFail
	} else if string(back) != expected {
		inv.setErr("failed", "proposal_readback_mismatch", "回读与写入的字节不一致")
		fmt.Fprintf(stderr, "%s: 提案件**回读对拍不一致**（盘上 %d 字节 ≠ 写入 %d 字节）⇒ 报失败、不报成功\n", progName, len(back), len(expected))
		return exitFail
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSON(stdout, stderr, inv, inv.path, proposalFields, proposalRow(rec))
	}
	fmt.Fprintln(stdout, rec.ID)
	fmt.Fprintf(stderr, "提案件（已改判据）: %s\n", rec.Path)
	fmt.Fprintf(stderr, "  判据   : %s → %s\n", orDash(oldCrit), orDash(rec.Criterion))
	fmt.Fprintf(stderr, "  前瞻   : %s → %s\n", orDash(oldCritNew), orDash(rec.CriterionNew))
	fmt.Fprintf(stderr, "零副作用口径：本命令**不触主控**（零 HTTP）· **不写入仓** —— 只改本提案一份（状态目录）\n")
	return exitOK
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
		"id":                  r.ID,
		"title":               r.Title,
		"target":              r.Target,
		"goal":                r.Goal,
		"evidence":            strings.Join(r.Evidence, ","),
		"rollback_ref":        r.Rollback,
		"by":                  r.By,
		"subject":             r.Subject,
		"subject_kind":        r.SubjectKind,
		"egg_id":              r.EggID,
		"approver":            r.Approver,
		"approver_kind":       r.ApproverKind,
		"state":               r.State,
		"criterion":           r.Criterion,
		"criterion_new":       r.CriterionNew,
		"files":               strings.Join(r.Files, ","),
		"criterion_state":     criterionState(r.Criterion),
		"criterion_new_state": criterionNewState(r.CriterionNew),
		"created_at":          r.CreatedAt,
		"path":                r.Path,
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
//
// ★ 第四种收（本枚 · GAP-20260927-266）：**行断言形态** `<件>:<行> 含 <串>`（含 `包含` / 否定 `不含`）
//
//	—— 见下一条分支与 `criterionLineAssert`。它不走命令树（不是 `zerg …`），但同样是「可机检」：
//	把判据钉到具体一行（改前那一行不含该串）⇒ 判据才有牙 —— 目录级搜索**改前也命中** = 判据无牙。
func criterionRunnable(spec string) (string, string) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return "", "缺判据（--criterion）—— 判据为空 ⇒ 提案**不给结论**（§17.3 铁律②：判据/证据为空不算判据）"
	}
	// ★ GAP-20260927-266：**行断言形态**先认（它不是 `zerg …` 命令，别掉进下面「首词不是 zerg」那条）。
	if file, ln, _, _, ok := criterionLineAssert(s); ok {
		switch {
		case filepath.IsAbs(file) || strings.HasPrefix(file, "/"):
			return "", fmt.Sprintf("行断言里的件 %q 是绝对路径 —— 要**仓内相对路径**（形态：`<件>:<行> 含 <串>`）", file)
		case strings.Contains(file, ".."):
			return "", fmt.Sprintf("行断言里的件 %q 有 `..` —— 不许越出仓根（形态：`<件>:<行> 含 <串>`）", file)
		}
		return fmt.Sprintf("行断言 %s:%d", file, ln), ""
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
			// ★ GAP-20261001-40：拒执时**给出替代只读判据名**（不逼人退到无意义值）。
			//   `gate check <判据名>` 的只读等价档 = 它的 `--dry-run` 那一态（永远跑得动、零副作用）；
			//   其余危险档命令一律给「单步只读门跑 / 族级帮助」两条现成只读面。
			alt := "或换一条**只读**判据（例：`" + progName + " gate run --step '<步名>' --only-step` · `" + progName + " help <族>`）"
			if strings.Join(c.path, " ") == "gate check" {
				alt = "或换它的**只读等价档**：`" + progName + " gate check <判据名> --dry-run`（那一态永远跑得动 · 零副作用）"
			}
			return "", fmt.Sprintf("判据里的 `%s` 在本版是**危险档、真跑未开放**（跑不到一个绿）——"+
				" 要拿它当判据，写成它的 `--dry-run` 那一态；%s", strings.Join(c.path, " "), alt)
		}
		return strings.Join(c.path, " "), ""
	}
	// ★ GAP-20260927-265（本枚）：强档 `--criterion` 只收**本版命令树里跑得动**的命令 —— 但若这条命令
	//   正是**本提案将要引入**的新命令（本版还跑不动），拒执时**不能只点名了事**（那人会被逼回「先写旧命令
	//   当判据 / 等重编 bin 再改提案」的手搓路 · 缺口正文逐字）⇒ 当场指向前瞻档 `--criterion-new`
	//   （`GAP-20260928-161` 的出口 · 两档不许互相冒充）；顺带给「最像的合法输入」，便于改词。
	hint := ""
	if s := nearest(words[0]); s != "" {
		hint = fmt.Sprintf("；本版最像的命令：`%s %s`", progName, s)
	}
	return "", fmt.Sprintf("判据里的命令 %q 不在命令树里（与门⑫ 同一条口径：命令示例必须能在命令清单里解析到）——"+
		" 若它正是**本提案将要引入**的新命令（本版还跑不动），写前瞻档 `--criterion-new` 那一格；"+
		" 强档 `--criterion` 只收本版真跑得动的命令（两档不许互相冒充 · `GAP-20260928-161`）%s",
		strings.Join(words, " "), hint)
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

// criterionNewState —— 前瞻判据的可跑态（★ GAP-20260928-161 · 方案甲 · **与 criterionState 同体例**）：
// 缺（没给）/ 前瞻（给了且**本版跑不动** —— 显式标前瞻，正是本档要的）/ 可跑（本版真能跑 ⇒ 不该落在这档）。
// 取值喂 `--json` 面的 `criterion_new_state` 那一格（分栏落盘）。
func criterionNewState(spec string) string {
	if strings.TrimSpace(spec) == "" {
		return "缺"
	}
	if _, why := criterionRunnable(spec); why != "" {
		return "前瞻"
	}
	return "可跑"
}

// criterionForward —— 前瞻判据档 `--criterion-new` 的**接受面**（★ GAP-20260928-161 · 方案甲 · 唯一判定口）。
//
// 前瞻形态 = 一条**首词是 `zerg`**、但**本版命令树里解析不到**的命令（将来才有的命令形态，
// 例：本提案将要引入的新命令 `zerg gap backfill-future --dry-run`）。它与强档 `--criterion` 恰好互补、
// **不许互相冒充**（`criterionRunnable` 一字未动 ⇒ 强档那边天然只收「本版能解析到」的）：
//
//	· 本版**已能真跑**的（命令树里解析得到 / 行断言 / 危险档带 --dry-run）⇒ 进 `--criterion`，本档**拒**；
//	· 与命令形态无关的（人话 / 别的工具 / 没有命令词）⇒ 哪个档都不收，本档点名「前瞻判据也要是命令形态」。
//
// 返回（态, why）：接受 ⇒ ("前瞻", "")；拒 ⇒ ("", why)。空串由调用方先挡（可选旗标）。
func criterionForward(spec string) (string, string) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return "", "缺前瞻判据（--criterion-new）"
	}
	fields := strings.Fields(s)
	prog := fields[0]
	if prog != progName && prog != "./bin/"+progName && !strings.HasSuffix(prog, "/"+progName) {
		return "", fmt.Sprintf("前瞻判据 `--criterion-new` 的首词是 %q —— 前瞻档收的是**本版还跑不动的命令形态**"+
			"（如将来才有的 `%s <对象> <动作>`），人话 / 别的工具都不算判据", prog, progName)
	}
	words := []string{}
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "-") {
			break
		}
		words = append(words, f)
	}
	if len(words) == 0 {
		return "", "前瞻判据里没有命令词（形态：`zerg <对象> <动作> [参数]`）"
	}
	// 任一前缀能在命令树里解析到 ⇒ 本版已能真跑 ⇒ 该进强档，本档拒（两档不许互相冒充）。
	for n := len(words); n >= 1; n-- {
		if find(words[:n]) != nil {
			return "", fmt.Sprintf("前瞻判据里的命令 `%s` **本版命令树里能解析到**（本版已能真跑）——"+
				" 那要进强档 `--criterion`（两档不许互相冒充）", strings.Join(words[:n], " "))
		}
	}
	return "前瞻", ""
}

// criterionLineAssert —— 判据的**行断言形态**解析（GAP-20260927-266 · 本枚）：
// `<件>:<行> 含 <串>`（分隔词收 `含` / `包含` · 否定收 `不含`）。
//
// 为什么需要它（缺口逐字）：`--criterion` 此前只收命令面，验一件改动只能写成**目录级搜索**
// （`zerg code find <正则>`）—— 而同一标识符往往在别处已有 ⇒ **改前也命中** ⇒ 判据恒绿、无牙。
// 钉到 `<件>:<行>` 上（改前那一行不含该串）⇒ 判据才有牙。
//
// 只做**形状**解析：件名的绝对路径 / `..` 由调用方（criterionRunnable）另判；行号 ≥1；串 = 分隔词
// **之后整段**（可含空格、非空）。ok=false ⇒ 不是行断言形态，调用方照旧走命令面那条路。
func criterionLineAssert(spec string) (file string, line int, sub string, negate bool, ok bool) {
	s := strings.TrimSpace(spec)
	i := strings.IndexFunc(s, unicode.IsSpace)
	if i < 0 {
		return "", 0, "", false, false
	}
	loc, rest := s[:i], strings.TrimSpace(s[i:])
	j := strings.LastIndex(loc, ":")
	if j <= 0 || j == len(loc)-1 {
		return "", 0, "", false, false
	}
	n, err := strconv.Atoi(loc[j+1:])
	if err != nil || n < 1 {
		return "", 0, "", false, false
	}
	k := strings.IndexFunc(rest, unicode.IsSpace)
	if k < 0 {
		return "", 0, "", false, false
	}
	kw, tail := rest[:k], strings.TrimSpace(rest[k:])
	if tail == "" {
		return "", 0, "", false, false
	}
	switch kw {
	case "含", "包含":
		negate = false
	case "不含":
		negate = true
	default:
		return "", 0, "", false, false
	}
	return loc[:j], n, tail, negate, true
}

// criterionAssertVerdict —— 行断言的**当前读数**（三态 · 只读）：成立 / 不成立 / 判不了（读不到）。
// `判不了` 覆盖：不是行断言 / 仓根解析不到 / 件读不到 / 行号越界 —— 一律**不当成立**
// （本仓铁律：读不到 ≠ 通过 · `GAP-20260927-266` 的机器面）。
func criterionAssertVerdict(spec, root string) string {
	file, line, sub, negate, ok := criterionLineAssert(spec)
	if !ok {
		return "判不了（不是行断言形态）"
	}
	if root == "" {
		return "判不了（仓根解析不到）"
	}
	b, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		return "判不了（件读不到：" + file + "）"
	}
	lines := strings.Split(string(b), "\n")
	if line > len(lines) {
		return fmt.Sprintf("判不了（%s 只有 %d 行，断言指 %d 行）", file, len(lines), line)
	}
	got := strings.Contains(lines[line-1], sub)
	if negate {
		got = !got
	}
	if got {
		return "成立"
	}
	return "不成立"
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

// rollbackUsableWhy —— 退点「现读可得」判定（**fail-closed** · `GAP-20261009-01`）。
//
// 病灶：`--rollback` 此前只判**非空** ⇒ 一个**读不到的裸名**（如 `/core__cmd__zerg__family_build.go`：
// 既非仓内相对路径、也非可执行命令）也满足「没退点的件不许提」并落地，而 `dev edit` 的退建议照它核
// sha256 取改前态时**取不到任何东西**（`rollback_cmd=""` · 档 2）⇒ 退点在**提出这一步**就该判死。
//
// 判法（✗ 不新创形态）：只看「现读拿不拿得到」——
//
//	① **路径形**退点（`filepath.IsAbs` 或含目录分隔符 `/`）：按 `devEditReturnPointOf` 同一口径解析
//	   （绝对按原样、相对按仓根拼）⇒ 必须是**现读得到的普通件**（不是读不到、也不是目录）；
//	② 其余 = schema 已认的**「方式」**文本（`--rollback <方式>`，如 `git revert …`）⇒ 按其本义收
//	   （✗ 不把合法形态一并拒）。
//
// 返回空串 = 现读可得；非空 = **逐字**的「退点不可得」理由（调用方据此拒执 rc=2）。
func rollbackUsableWhy(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "" // 非空那一档由上游 `rollback_required` 单独判
	}
	if !filepath.IsAbs(v) && !strings.Contains(v, "/") {
		// 非路径形 = 「方式」文本（schema 认的形态）⇒ 按其本义收。
		return ""
	}
	p := v
	if !filepath.IsAbs(p) {
		p = filepath.Join(repoRoot(), filepath.FromSlash(p))
	}
	st, err := os.Stat(p)
	if err != nil {
		return fmt.Sprintf("退点不可得：%s 是路径形却读不到（%s：%v）—— 退建议照它核 sha256 取改前态，读不到就取不回任何东西", v, p, err)
	}
	if st.IsDir() {
		return fmt.Sprintf("退点不可得：%s 是**目录**（%s）—— 退点要一件**可回写的件**，不是目录", v, p)
	}
	return ""
}

// ---- ★ GAP-20260927-277：`dev proposal check --deep` —— 判据**真跑**（核真值）----
//
// 病灶（缺口逐字）：「判据面只判『可机检』不核真值：173 件全报可跑，真跑 16 件不命中」——
// `criterionRunnable` 只判「命令解析得到 / 不是危险档」，不判「真敲一次拿不拿到绿」⇒
// 假可跑的判据（如非法步名 `--step gofmt-core` · 同族缺口 `GAP-20261001-25`）能蒙过 check。
//
// 本档（`--deep` · 贵面现跑，形态照 `zerg impact --deep`）把每一条命令面判据**真跑一次**、把退码收回来：
// 命中 = 退码 0；不命中 = 非 0 / 跑不动 / 超时（读不到 ≠ 通过）。默认档（不带 `--deep`）一字未动。

// proposalDeepTimeout —— 单条判据真跑的上限（跑不动 ⇒ 判不了，不无限等）。
const proposalDeepTimeout = 15 * time.Minute

// proposalDeepExe —— 真跑判据时裸 `zerg` 解析到的二进制（缺省 = 本进程自己；测试可改写成现成制品）。
var proposalDeepExe = func() (string, error) { return os.Executable() }

// proposalDeepRun —— 真跑一条判据（命令形态 `zerg …`），返回 (退码, 跑得动吗)。
//
// 判据串可能含 shell 引号（如 `--step '门⑩ …'`）⇒ 走 `sh -c <判据串>` 保留原引号语义
// （不用 `strings.Fields` —— 那会把引号里的空格切碎 = 假红）。首词 `zerg` 由 PATH 解析到
// **本进程的二进制**（其目录前置到 PATH）⇒ `zerg` / `./bin/zerg` / `<abs>/zerg` 三形态都落到现役件。
// cwd = 仓根（判据里的仓内相对路径才有意义）。★ 独立进程组 + 收整组（照 `ffpRunProbe` 的自伤洞口径）：
// 超时杀**整棵**子树，✗ 不用 `exec.CommandContext`（只杀直接子、孙进程成孤儿还继承锁 fd）。
func proposalDeepRun(criterion, root string) (int, bool) {
	exe, err := proposalDeepExe()
	if err != nil {
		return 0, false
	}
	cmd := exec.Command("sh", "-c", criterion)
	cmd.Dir = root
	cmd.Env = proposalDeepEnv(os.Environ(), filepath.Dir(exe))
	cmd.Stdout = io.Discard // 本档只读退码，判据自己的报告不进本命令的输出
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if e := cmd.Start(); e != nil {
		return 0, false // 起不来 ⇒ 判不了（fail-closed：不当命中）
	}
	pgid := cmd.Process.Pid
	var to atomic.Bool
	timer := time.AfterFunc(proposalDeepTimeout, func() {
		to.Store(true)
		ffpKillGroup(pgid, syscall.SIGTERM)
		time.AfterFunc(2*time.Second, func() { ffpKillGroup(pgid, syscall.SIGKILL) })
		_ = cmd.Process.Kill()
	})
	werr := cmd.Wait()
	timer.Stop()
	if to.Load() {
		return 0, false // 超时 ⇒ 判不了
	}
	if werr == nil {
		return 0, true
	}
	var ee *exec.ExitError
	if errors.As(werr, &ee) {
		return ee.ExitCode(), true // 真退出码（正/负都算跑得动）
	}
	return 0, false
}

// proposalDeepEnv 把 exeDir 前置到 PATH（判据里的裸 `zerg` 解析到本进程的二进制）。
func proposalDeepEnv(env []string, exeDir string) []string {
	for i, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			env[i] = "PATH=" + exeDir + string(os.PathListSeparator) + kv[len("PATH="):]
			return env
		}
	}
	return append(env, "PATH="+exeDir)
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
	// ★ GAP-20260927-336：本只读面顺带把**合法 target 闭集**列出来（成功/空转两态都印；退码一字不动）
	//   —— check 是唯一一个不新动作就能到的「提案族只读面」。
	fmt.Fprintf(stderr, "%s\n", legalTargetsLine())
	rows := []map[string]string{}
	bad := 0
	assertBad := 0
	seen := 0
	anyAssert := false
	// ★ GAP-20260927-277：`--deep` 的两格计数器（真跑件数 / 不命中数）—— 不深档恒 0、一字不印。
	deepRan, deepMiss := 0, 0
	root := repoRoot()
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
		// ★ GAP-20260927-266（本枚）：行断言形态 —— 判据的**当前读数**也判（成立 / 不成立 / 判不了）。
		//   不成立或判不了 ⇒ 一样不给结论（退码 2）—— 这一格就是「判据有没有牙」的机器面。
		assertVerdict := ""
		isAssert := false
		if _, _, _, _, isA := criterionLineAssert(r.Criterion); isA {
			anyAssert = true
			assertVerdict = criterionAssertVerdict(r.Criterion, root)
			if st == "可跑" && assertVerdict != "成立" {
				assertBad++
			}
		}
		// ★ GAP-20260927-277：`--deep` —— 判据**真跑**（核真值），把退码收回来。
		//   命中 = 退码 0；不命中 = 非 0 / 跑不动 / 超时（读不到 ≠ 通过）。
		deepRC, deepVerdict := "", ""
		if inv.deep {
			switch {
			case st != "可跑":
				deepVerdict = "未跑（判据不可跑）"
			case isAssert:
				// 行断言的「真值」= 现有 verdict（读件核行，不起子进程）。
				deepRan++
				if assertVerdict == "成立" {
					deepVerdict = "命中"
				} else {
					deepVerdict = "不命中（" + assertVerdict + "）"
					deepMiss++
				}
			default:
				drc, ok := proposalDeepRun(r.Criterion, root)
				deepRan++
				if !ok {
					deepRC = "?"
					deepVerdict = "判不了（跑不动 / 超时）"
					deepMiss++
				} else {
					deepRC = strconv.Itoa(drc)
					if drc == 0 {
						deepVerdict = "命中"
					} else {
						deepVerdict = "不命中"
						deepMiss++
					}
				}
			}
		}
		rows = append(rows, map[string]string{
			"id": r.ID, "target": r.Target, "state": r.State,
			"criterion": r.Criterion, "criterion_state": st, "why": why,
			"files": strings.Join(r.Files, ","), "assert": assertVerdict,
			"deep_rc": deepRC, "deep": deepVerdict,
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
	disp := []string{"id", "target", "state", "criterion_state", "criterion", "files", "why"}
	if anyAssert {
		// 有行断言才多印一列（纯命令面判据的行**逐字节不变** —— 列数不因本枚而变）。
		disp = append(disp, "assert")
	}
	if inv.deep {
		// ★ GAP-20260927-277：`--deep` 才多印两列（不深档行**逐字节不变** —— 列数不因本枚而变）。
		disp = append(disp, "deep_rc", "deep")
	}
	rc := listCmd(inv, stdout, stderr, disp, rows)
	if rc != exitOK {
		return rc
	}
	if inv.deep {
		// ★ GAP-20260927-277「报 rc 与命中数」：深档每次（成功/不命中两态）都印这行；rc 取本命令的退码。
		fmt.Fprintf(stderr, "%s: 真跑 %d 件判据 · %d 命中 · %d 不命中\n", progName, deepRan, deepRan-deepMiss, deepMiss)
	}
	if bad > 0 {
		inv.setErr("usage", "criterion_not_machine_checkable",
			fmt.Sprintf("%d 件提案的判据缺 / 跑不动", bad))
		fmt.Fprintf(stderr, "%s: %d/%d 件提案的判据**不可机检** ⇒ 不给结论（退码 2）：判据必须是一条本版跑得动的 `%s …`\n",
			progName, bad, seen, progName)
		return exitUsage
	}
	if assertBad > 0 {
		// ★ GAP-20260927-266：行断言**改前就成立 / 判不了** ⇒ 判据无牙 / 未达 ⇒ 一样不给结论。
		inv.setErr("usage", "criterion_assertion_unmet",
			fmt.Sprintf("%d 件提案的行断言当前不成立 / 判不了", assertBad))
		fmt.Fprintf(stderr, "%s: %d/%d 件提案的**行断言**当前不成立 → 判据未达（不给结论 · 退码 2）："+
			"行断言要**改后成立**（改前就成立 = 判据无牙）；判不了（件读不到 / 行越界）**一样红**（读不到 ≠ 通过）\n",
			progName, assertBad, seen)
		return exitUsage
	}
	if inv.deep {
		if deepMiss > 0 {
			// ★ GAP-20260927-277：判据「可机检」≠ 真值成立 ⇒ 真跑不命中一律不给结论
			//   （退码 2 · 逐件在 `deep` 列点名）。
			inv.setErr("usage", "criterion_deep_unmet",
				fmt.Sprintf("%d 件判据真跑不命中", deepMiss))
			fmt.Fprintf(stderr, "%s: %d/%d 件判据**真跑不命中** ⇒ 不给结论（退码 2）："+
				"判据「可机检」不等于真值成立（见 `deep` 列逐件点名）\n", progName, deepMiss, deepRan)
			return exitUsage
		}
		return exitOK
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

// legalTargetsLine —— ★ GAP-20260927-336（2026-10-04 本枚）：**合法 target 闭集**的一行只读读数。
//
// 病灶（缺口逐字）：`dev proposal new --target` 此前**没有只读面**列出合法 target 闭集 —— 闭集只活在
// `core/internal/contract/dev-targets.json`，传一个不在闭集里的编号只撞 rc=2，而报错只给**一枚**最像的
// （`nearestTarget`）⇒ 调用方要看全闭集只能**去仓外手抠**那个 json。`dev proposal check` 列的是**已有提案**
// 的 target（拿空目录时一条都没有）= 循环，帮不上。
//
// 落法（**不新旗标、不新动作** —— 边界：新旗标/新动作要动 `main.go` 命令树/解析面，本片禁区）：
// 把闭集**接出来**成一枚共享只读读数，喂两处现成的只读面 —— ① `dev proposal new` 的 target 拒收路径
// （撞 rc=2 的那一刻当场把全闭集列出来，不必再出仓）· ② `dev proposal check` 的 stderr（成功面也能读）。
// 取值**只此一处**（`contract.DevTargets()` —— 与 `canonTarget`/`nearestTarget` 同源，禁抄第二份）。
func legalTargetsLine() string {
	ids, err := contract.DevTargets()
	if err != nil || len(ids) == 0 {
		// 读不到 ⇒ **不给结论**（不当成「没有 target 可选」—— 本仓铁律：读不到 ≠ 没有）。
		return "合法 target 闭集：**读不到**（core/internal/contract/dev-targets.json）⇒ 判不了，不冒充「没有」"
	}
	return fmt.Sprintf("合法 target 闭集（D/E/F/G 族 · 逐条真源 core/internal/contract/dev-targets.json · %d 条）：%s",
		len(ids), strings.Join(ids, " · "))
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

// ---- 出处**在册**（`GAP-20260927-254`）----

// evidenceIsIDShaped —— 出处是不是**账内 id 形状**：带 `GAP-`（缺口账 id）或 `DEV-`（在册提案号）前缀。
//
// ★ `GAP-20261009-09`：`--evidence` 认两类证据 ——
//
//	① **id 形状**（`GAP-…` / `DEV-…`）⇒ 仍须**在册**（下面逐条查缺口账 / 在册件）；
//	② **描述性证据**（无前缀的自由文本）⇒ 按其本义收，**不**要求在册。
//
// 这一格只判「长得像不像账内 id」——**不放宽**对真 id 的校验（id 形状仍必在册），也不让不存在的 id 蒙混。
func evidenceIsIDShaped(e string) bool {
	k := strings.ToUpper(strings.TrimSpace(e))
	return strings.HasPrefix(k, "GAP-") || strings.HasPrefix(k, "DEV-")
}

// evidenceNotInLedgerWhy 判每条**出处 id** 是否**在册**：缺口账（`zerg gap ls` 的真源）或在册提案件里查得到。
// 返回**第一条查不到的**（空串 = 全部在册）。
//
// ★ `GAP-20261009-09`：只对 **id 形状** 的出处判在册（`evidenceIsIDShaped`）；描述性证据
// （无 `GAP-`/`DEV-` 前缀的自由文本）**直接收** —— 证据面由此有了「描述性文本」的投放处。
// ★ 真源只走读面：缺口账走 `readGapLedger`（与 `gap ls` **同一份式子**，不另抄一份账），
//
//	在册件走 `loadProposals`（与 `dev proposal list` 同源）—— 不新立第二套「在册」判据。
//
// ★ 「账读不到」**不**退化成「放行」（与 §九 M9「读不到不当没有」同口径）：读不动 ⇒ 不给结论（rc=2）。
// ★ 大小写折叠只为**容错书写**（`gap show` 面 id 全大写）；判的是「查得到/查不到」，不改 id 本身。
func evidenceNotInLedgerWhy(ev []string) string {
	// ★ GAP-20261009-09：只对 **id 形状** 的出处判在册；描述性证据直接收。
	//   先筛：全无 id 形状 ⇒ 无须读账（描述性证据不依赖缺口账 ⇒ 账读不到也不该拦它）。
	var ids []string
	for _, e := range ev {
		if evidenceIsIDShaped(e) {
			ids = append(ids, e)
		}
	}
	if len(ids) == 0 {
		return ""
	}
	led, err := readGapLedger()
	if err != nil {
		return fmt.Sprintf("缺口账读不出来（%v）⇒ 出处**无法校验** ⇒ 不给结论（退码 2）", err)
	}
	inLedger := map[string]bool{}
	for _, r := range led.Recs {
		inLedger[strings.ToUpper(strings.TrimSpace(r.ID))] = true
	}
	inProposal := map[string]bool{}
	if recs, err := loadProposals(proposalDir()); err == nil {
		for _, r := range recs {
			inProposal[strings.ToUpper(strings.TrimSpace(r.ID))] = true
		}
	}
	for _, e := range ids {
		key := strings.ToUpper(strings.TrimSpace(e))
		if inLedger[key] || inProposal[key] {
			continue
		}
		return fmt.Sprintf("出处 %q 在缺口账 / 在册件里**查不到** ⇒ 不给结论（退码 2）—— 挂不存在出处的提案不许提", e)
	}
	return ""
}
