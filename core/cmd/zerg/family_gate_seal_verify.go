// family_gate_seal_verify.go —— `zerg gate seal verify <章账>`：**核章档（第四档）章账的只读核验面**。
//
// 一句话：**章 = 步级 / 件级的签名记录**。本命令把 append-only 章账
// `scripts/gates/seal/ledger.jsonl` 逐枚**核**一遍 —— 只读，不真跑任何一步：
//
//	① **九格齐备**：`step/status/rc/secs/log/worktree_fp/head/item_set_digest/judge_ver/gate_ver`
//	   缺任一格 ⇒ 章无效 ⇒ **判不了**（退码 2，逐枚点名叫出缺的是哪几格）；
//	② **断言 `executed=true == 0`**：核章**不真跑** ⇒ 逐枚章状态一律 `executed=false`，
//	   输出行现印「executed=true 计数 0（**必须 0**）」；
//	③ **失效规则条条有牙**（现读比对，不手工续期）：
//	   · 章的 `head` 与现读 HEAD 不符 ⇒ `stale`；
//	   · 章的 `worktree_fp` 与现读工作树指纹不符 ⇒ `stale`；
//	   · 该步已不在**现读步集**（增删改名）⇒ `gap`。
//
// 输出纪律（闭集 · 不可放宽）：**只许三态** `未真跑（核章）` / `stale` / `gap`，
// **任何位置不出现** `PASS`（核章不真跑 ⇒ 绝不渲染成绿）。
//
// 退码：0 全有效（九格齐 + 无 stale/gap）· 1 有 `stale`/`gap`（逐枚点名）·
// 2 **判不了**（章账读不到 / 缺格 / 现读步集或指纹取不回 —— 一律不给结论）。
//
// 只读：本命令只读章账、门件现读步集与工作树指纹，**不写盘、不跑步骤、不续期**。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// sealLedgerDefaultRel —— 章账缺省落点（append-only · 与 `scripts/gates/seal-ledger.py` 同源）。
// ★ 2026-09-30 修 · N6：与判据件 `scripts/gates/seal-ledger.py` **同目录同基名、扩展名不同** ⇒
//
//	撞命名规则 ⑥（`check-doc-name.py` 的 N6）⇒ 换落到**子目录** `scripts/gates/seal/ledger.jsonl`
//	（名字语义保留：同族、同类、只挪一层）。★ 本常量是**单一真源**：指纹口径
//	（`gate_freshness.go` 的 `worktreeFingerprint`）照它精确排除章账自身。
const sealLedgerDefaultRel = "scripts/gates/seal/ledger.jsonl"

// gateSealVerifyFields —— `gate seal verify --json` 的闭集字段。
var gateSealVerifyFields = []string{"chapter_no", "step", "state", "reasons",
	"executed_true", "seals", "covers", "steps_now", "stale", "gap",
	"cur_head", "cur_worktree_fp"}

// sealVerifyStates —— 核章输出**只许**这三种状态（闭集 · 禁渲染成绿）。
var sealVerifyStates = []string{"未真跑（核章）", "stale", "gap"}

// sealVerifyCells —— 九格（逐格判齐备；缺任一格 ⇒ 判不了）。
var sealVerifyCells = []string{"step", "status", "rc", "secs", "log",
	"worktree_fp", "head", "item_set_digest", "judge_ver", "gate_ver"}

// sealRecord —— 章账一行（JSONL · append-only）。
type sealRecord map[string]interface{}

// sealStr —— 取一格的字面串（非串格转成十进制串；缺失/空 ⇒ 空串）。
func sealStr(r sealRecord, k string) string {
	v, ok := r[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	s := fmt.Sprintf("%v", v)
	if s == "<nil>" {
		return ""
	}
	return s
}

// sealNum —— 取一格当整数（取不到 ⇒ 0）。
func sealNum(r sealRecord, k string) int {
	switch t := r[k].(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n := 0
		for _, c := range t {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		return n
	}
	return 0
}

// sealVerdict —— 逐枚章的核验结论。
type sealVerdict struct {
	ChapterNo int
	Step      string
	State     string
	Reasons   []string
}

// cmdGateSealVerify —— `zerg gate seal verify [<章账>] [--json <字段>]`。
func cmdGateSealVerify(inv *invocation, stdout, stderr io.Writer) int {
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_unresolved", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 2）\n", progName)
		return exitUsage
	}
	ledger := ""
	if len(inv.args) > 0 {
		ledger = strings.TrimSpace(inv.args[0])
	}
	if ledger == "" {
		ledger = filepath.Join(root, sealLedgerDefaultRel)
	} else if !filepath.IsAbs(ledger) {
		ledger = filepath.Join(root, ledger)
	}
	b, err := os.ReadFile(ledger)
	if err != nil {
		inv.setErr("blocked", "seal_ledger_unreadable", "章账读不到")
		fmt.Fprintf(stderr, "%s: 章账读不到（%s：%v）⇒ **判不了**（退码 2）—— 不许当绿\n", progName, ledger, err)
		fmt.Fprintf(stderr, "  形态：%s gate seal verify [<章账>]（缺省 %s）\n", progName, sealLedgerDefaultRel)
		return exitUsage
	}

	recs := []sealRecord{}
	badLines := 0
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var r sealRecord
		if err := json.Unmarshal([]byte(ln), &r); err != nil {
			badLines++
			continue
		}
		recs = append(recs, r)
	}
	seals, covers := []sealRecord{}, []sealRecord{}
	for _, r := range recs {
		if sealStr(r, "kind") == "cover" {
			covers = append(covers, r)
			continue
		}
		seals = append(seals, r)
	}
	if len(seals) == 0 {
		inv.setErr("blocked", "seal_ledger_empty", "章账里一枚章都没有")
		fmt.Fprintf(stderr, "%s: 章账 %s 读不到任何一枚章（可解析行 %d · 坏行 %d）⇒ **判不了**（退码 2）—— 不许当绿\n",
			progName, ledger, len(recs), badLines)
		return exitUsage
	}

	// 现读两格：步集（门件自报）与工作树指纹 + HEAD —— 取不回 ⇒ 判不了。
	names, nerr := gateFullStepNames(root)
	if nerr != nil {
		inv.setErr("blocked", "gate_step_list_unreadable", nerr.Error())
		fmt.Fprintf(stderr, "%s: 读不回门件现读步集（%v）⇒ 不给结论（退码 2）\n", progName, nerr)
		return exitUsage
	}
	curSet := map[string]bool{}
	for _, n := range names {
		curSet[n] = true
	}
	curFP, curHead, ferr := worktreeFingerprint(root)
	if ferr != nil {
		inv.setErr("blocked", "worktree_fingerprint_failed", ferr.Error())
		fmt.Fprintf(stderr, "%s: 算不出当前工作树指纹（%v）⇒ 不给结论（退码 2）\n", progName, ferr)
		return exitUsage
	}

	verdicts := []sealVerdict{}
	nStale, nGap, sev := 0, 0, 0
	for _, r := range seals {
		missing := []string{}
		for _, k := range sealVerifyCells {
			if sealStr(r, k) == "" {
				missing = append(missing, k)
			}
		}
		reasons := []string{}
		state := sealVerifyStates[0]
		if len(missing) > 0 {
			reasons = append(reasons, "缺格："+strings.Join(missing, " · ")+" ⇒ 章无效（硬前提①）")
			state = "gap"
			sev = 2
		}
		if h := sealStr(r, "head"); h != "" && curHead != "" && h != curHead {
			reasons = append(reasons, fmt.Sprintf("stale：head 不符（章 %s ≠ 现读 %s）", h, curHead))
			if state == sealVerifyStates[0] {
				state = "stale"
			}
			if sev < 1 {
				sev = 1
			}
		}
		if fp := sealStr(r, "worktree_fp"); fp != "" && curFP != "" && fp != curFP {
			reasons = append(reasons, fmt.Sprintf("stale：worktree_fp 不符（章 %s ≠ 现读 %s）", short16(fp), short16(curFP)))
			if state == sealVerifyStates[0] {
				state = "stale"
			}
			if sev < 1 {
				sev = 1
			}
		}
		step := sealStr(r, "step")
		if step != "" && !curSet[step] {
			reasons = append(reasons, "gap：该步已不在现读步集（增删改名）")
			state = "gap"
			if sev < 1 {
				sev = 1
			}
		}
		switch state {
		case "gap":
			nGap++
		case "stale":
			nStale++
		}
		verdicts = append(verdicts, sealVerdict{sealNum(r, "chapter_no"), step, state, reasons})
	}

	// 输出纪律：executed=true 计数**恒 0**（核章不真跑）；三态逐枚落闭集。
	nExec := 0
	fmt.Fprintf(stdout, "── 核章档（第四档）· 未真跑 · 结论由章支持 ──\n")
	fmt.Fprintf(stdout, "── 章账 %s · 章 %d 枚 · 覆盖表 %d 张 · executed=true 计数 %d（**必须 0**）· 现读步集 %d 步 ──\n",
		ledger, len(seals), len(covers), nExec, len(names))
	fmt.Fprintf(stdout, "── 当前 HEAD %s · 工作树指纹 %s ──\n", short16(curHead), short16(curFP))
	for _, v := range verdicts {
		fmt.Fprintf(stdout, "   [%s] #%d %s\n", v.State, v.ChapterNo, v.Step)
		for _, rs := range v.Reasons {
			fmt.Fprintf(stdout, "        ✗ %s\n", rs)
		}
	}
	if inv.jsonGiven {
		rows := make([]map[string]string, 0, len(verdicts))
		for _, v := range verdicts {
			rows = append(rows, map[string]string{
				"chapter_no": fmt.Sprintf("%d", v.ChapterNo), "step": v.Step,
				"state": v.State, "reasons": strings.Join(v.Reasons, " ； "),
				"executed_true": "0", "seals": fmt.Sprintf("%d", len(seals)),
				"covers": fmt.Sprintf("%d", len(covers)), "steps_now": fmt.Sprintf("%d", len(names)),
				"stale": fmt.Sprintf("%d", nStale), "gap": fmt.Sprintf("%d", nGap),
				"cur_head": curHead, "cur_worktree_fp": curFP,
			})
		}
		return listCmd(inv, stdout, stderr, gateSealVerifyFields, rows)
	}
	switch sev {
	case 0:
		fmt.Fprintf(stdout, "⇒ 核章：章链有效（rc=0）· **未真跑任何一步**（executed=true == 0）\n")
		return exitOK
	case 1:
		fmt.Fprintf(stdout, "⇒ 核章：**有不符项**（rc=1）—— 过期**不等于**重跑全量：只补跑该章覆盖的那几步\n")
		return exitFail
	}
	fmt.Fprintf(stdout, "⇒ 核章：**判不了**（rc=2：缺格/读不回）—— 不给结论\n")
	return exitUsage
}
