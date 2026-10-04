// gate_run_partial_seal.go —— 部分趟侧「章账引用自比」（设计稿《设计-门禁部分趟-v0.7》§13.1 ·
// docB 条 4 · 表乙 T-PR-16 / 拍板项 D11）。
//
// ⚠ 候件（仓外）：拟落路径 `core/cmd/zerg/gate_run_partial_seal.go`（**新件** · 现读不存在）。
// 本件只落**已拍/已定稿**口径 —— D11 已拍：步骤 5 走**部分趟侧自比**（**零改 `seal verify`**）。
//
// 一句话：部分趟**引用上趟绿**时，把「本趟不重跑的步」`S_keep` 逐枚过一遍**章账核对** ——
// 比的是「章里 `head`/`worktree_fp`」 vs 「基线**全量**趟 `D_base` 的 `head`/`worktree_fp`」
// （**基线相对**），✗ 不是 vs 现读（现读工作树已被本趟的修复改动）。
//
// ★★ 为什么必须「部分趟侧自比」（D11 · §13.1 ★关键）：现读 `zerg gate seal verify` 比的是**现读**
//
//	`head`/`worktree_fp`（`family_gate_seal_verify.go:188-205`）。而部分趟的**前提就是工作树已改**（修红）
//	⇒ 现读 fp ≠ 章里 fp ⇒ `seal verify` 会把基线章**一律判 `stale`** ⇒ 拿它当引用判据 = **永远不可引**。
//	⇒ 故：`seal verify` 只当**章账结构证据**（只读调一次）；**可引与否**由本件把「章 head/fp」与
//	「`D_base/freshness.json` 的 head/fp」两处**自比**得出。**零改 `seal verify`**（✗ 不动其判定）。
//
// 8 步序列（§13.1 逐字 → 本件落点）：
//
//	① 定基线        → gatePartialSealRefs 开头 + readGateFreshness   （读 D_base/freshness.json ⇒ (b_head,b_fp)；取不到 ⇒ rc=2）
//	② 选 S_keep     → gatePartialSealKeepSet             （= 现读步集 − R；本趟不重跑者）
//	③ 逐名初筛      → gatePartialSealRefs 的 baseStatus 段（D_base/results.tsv 该步非 PASS ⇒ 台账自相矛盾 ⇒ rc=2）
//	④ 章账核验(只读) → gatePartialSealVerifyLedger + gatePartialSealSealOfStep（只读 · 不 --write/不续期）
//	⑤ 基线相对比对  → gatePartialSealFlatCompare + gatePartialSealMissingCells（章 head==b_head 且 章 fp==b_fp ⇒ 可引）
//	⑥ 不可引回填    → 调用方把 backfill[] 并入 R（D11 已拍：**回填重跑**，严格性优先于收益）
//	⑦ 抬头逐印      → gatePartialSealPrintHeader
//	⑧ 纪律          → gatePartialSealRefs（全程**只读** · `executed=true` 计数恒 0 · 本趟**不真跑**被引用的步）
//
// 退码：0 自比完成（refer[]/backfill[] 有效）· 2 **判不了**（基线取不到 / 章账读不到 / 台账自相矛盾
// ⇒ **整趟拒执**，✗ 不当空集、✗ 不当全绿）。本件**不新增退码**（`3` 属部分趟主链 T-PR-20/D15，不在本片）。
//
// 单源复用（✗ 本件不另写第二份真源）：`sealLedgerDefaultRel`（章账落点）· `sealVerifyCells`（九格）·
// `readGateFreshness`（随件）· `gateResultsReadTSV`（结果表）· `slgLoadLedger`/`slgChainGet`（章账解析）·
// `short16` · `exitOK`/`exitUsage`。
//
// ✗ 不做（红线 §3.2 / §十）：按内容指纹跳步；未跑步标「沿用/继承绿」（引用**只作引用**，绝不当绿）；
// 自动把部分趟当收口；为部分趟盖章（若有人盖过 ⇒ **须显式拒**，见 gatePartialSealPartialCoverGuard，本片只落最小守门）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// partialSealVerdict —— 一枚 `S_keep` 步的章账自比结论。
type partialSealVerdict struct {
	Step     string
	Refer    bool // true=可引（引用上趟绿）；false=不可引（回填重跑）
	Reason   string
	Chapter  int
	SealHead string
	SealFP   string
}

// gatePartialSealLedgerPath —— 章账落点：空 ⇒ 缺省真源 sealLedgerDefaultRel；相对路径 ⇒ 挂仓根。
func gatePartialSealLedgerPath(root, ledger string) string {
	if ledger == "" {
		return filepath.Join(root, sealLedgerDefaultRel)
	}
	if filepath.IsAbs(ledger) {
		return ledger
	}
	return filepath.Join(root, ledger)
}

// gatePartialSealKeepSet —— §13.1 步②：`S_keep = 现读步集 full − 本趟重跑集 R`（**按 full 序**）。
func gatePartialSealKeepSet(full, rerun []string) []string {
	inR := map[string]bool{}
	for _, r := range rerun {
		inR[r] = true
	}
	out := []string{}
	for _, n := range full {
		if !inR[n] {
			out = append(out, n)
		}
	}
	return out
}

// gatePartialSealSealOfStep —— §13.1 步④：从章账记录里取**该步最近一枚章**。
// 章账 append-only ⇒ 同一步多枚时取**末次**（最近盖的那一枚）；跳过 cover 行与坏行。
func gatePartialSealSealOfStep(recs []map[string]interface{}, step string) (map[string]interface{}, bool) {
	var found map[string]interface{}
	ok := false
	for _, r := range recs {
		if r == nil {
			continue
		}
		if _, bad := r["_unparseable"]; bad {
			continue
		}
		if slgChainGet(r, "kind") == "cover" {
			continue
		}
		if slgChainGet(r, "step") == step {
			found = r
			ok = true
		}
	}
	return found, ok
}

// gatePartialSealMissingCells —— §13.1 步④：九格缺哪几格（缺任一 ⇒ 章无效 ⇒ 不可引）。
func gatePartialSealMissingCells(rec map[string]interface{}) []string {
	miss := []string{}
	for _, k := range sealVerifyCells {
		if slgChainGet(rec, k) == "" {
			miss = append(miss, k)
		}
	}
	return miss
}

// gatePartialSealFlatCompare —— §13.1 步⑤ **基线相对比对**：章 `head`/`worktree_fp` vs 基线趟的
// `head`/`worktree_fp`（**逐字相等**）。任一不符 ⇒ 该章对应的结论与基线趟不是同一棵树 ⇒ 不可引。
func gatePartialSealFlatCompare(bHead, bFP, sealHead, sealFP string) bool {
	return bHead != "" && bFP != "" && sealHead == bHead && sealFP == bFP
}

// gatePartialSealPartialCoverGuard —— 最小守门：若章账里出现「为部分趟盖的章」痕迹 ⇒ 拒（§13.1 ★「须显式拒」）。
//
// 现读口径：部分趟**不写** `results.tsv` ⇒ `seal-ledger.py --seal <趟>` 根本跑不起来 ⇒ 正常不会为部分趟盖章。
// 本守门只拦**覆盖表**里 `run_dir` 明显是部分趟目录的那种（含 `results-partial` 记号或目录名含 `partial-`）。
// ★ 完整「显式拒」规则仍属**待写**（§13.1 ★末句）—— 本件只落这条最小守门，不擅扩。
func gatePartialSealPartialCoverGuard(recs []map[string]interface{}) []string {
	hits := []string{}
	for _, r := range recs {
		if r == nil || slgChainGet(r, "kind") != "cover" {
			continue
		}
		runDir := slgChainGet(r, "run_dir")
		low := strings.ToLower(runDir)
		if strings.Contains(low, "results-partial") || strings.Contains(filepath.Base(runDir), "partial-") {
			hits = append(hits, runDir)
		}
	}
	return hits
}

// gatePartialSealVerifyLedger —— §13.1 步④：**只读**调一次 `zerg gate seal verify <章账> --json state`
// （零改其判定）。当**章账结构证据**；bin/zerg 不在场 ⇒ 跳过（此时可引判定仍由本件直读章账自比得出）。
// 返回（rc, 一句话注）。rc<0 ⇒ 未跑/起不动。
func gatePartialSealVerifyLedger(root, ledger string) (int, string) {
	zerg := filepath.Join(root, "bin", "zerg")
	if _, err := os.Stat(zerg); err != nil {
		return -1, "无 bin/zerg ⇒ 跳过只读核（可引判定仍由本件直读章账自比得出 · 零改 seal verify）"
	}
	cmd := exec.Command(zerg, "gate", "seal", "verify", ledger, "--json", "state")
	cmd.Dir = root
	var so, se strings.Builder
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), fmt.Sprintf("seal verify 只读核 rc=%d（结构证据；其 stale 系「现读≠章」的**预期**态 ⇒ 不作本件否决依据 · ★关键）", ee.ExitCode())
		}
		return -2, "seal verify 起不动：" + err.Error()
	}
	return 0, "seal verify 只读核 rc=0（章账结构证据 · 只读 · 不 --write/不续期）"
}

// gatePartialSealRefs —— §13.1 步①–⑧ 的总入口（D11：部分趟侧自比 · 零改 seal verify）。
//
// 入参：
//
//	root    仓根（用于定位缺省章账、bin/zerg）
//	baseDir 基线**全量**趟目录 D_base（由 `--since` 选定 · §3.5 · 含 freshness.json）
//	keep    S_keep（§13.1 步② · = 现读步集 − R）
//	full    现读步集 N（全表序）
//	ledger  章账落点（空 ⇒ 缺省 scripts/gates/seal/ledger.jsonl）
//	out     抬头输出
//
// 返回（refer 可引 · backfill 不可引⇒回填重跑 · rc 0=完成 / 2=判不了⇒整趟拒执）。
func gatePartialSealRefs(root, baseDir string, keep, full []string, ledger string, out io.Writer) (refer, backfill []string, rc int) {
	ledger = gatePartialSealLedgerPath(root, ledger)

	// ── ① 定基线：读 D_base/freshness.json ⇒ (b_head, b_fp)；取不到 ⇒ rc=2（§3.6）。
	base, err := readGateFreshness(baseDir)
	if err != nil || base.Head == "" || base.Fingerprint == "" {
		fmt.Fprintf(out, "章账引用自比：基线趟 %s **取不到 freshness.json 的 head/worktree_fp** ⇒ 判不了（rc=2）—— ✗ 不退回 HEAD 凑一个\n", baseDir)
		return nil, nil, exitUsage
	}
	bHead, bFP := base.Head, base.Fingerprint

	// ── ③ 逐名初筛：读 D_base/results.tsv，S_keep 里出现基线**非 PASS** 的步 ⇒ 台账自相矛盾 ⇒ rc=2。
	rows, _, rerr := gateResultsReadTSV(filepath.Join(baseDir, gateResultsTSV))
	if rerr != nil {
		fmt.Fprintf(out, "章账引用自比：基线趟 results.tsv 读不到（%v）⇒ 判不了（rc=2）\n", rerr)
		return nil, nil, exitUsage
	}
	baseStatus := map[string]string{}
	for _, r := range rows {
		baseStatus[r.Name] = r.Status
	}
	contra := []string{}
	for _, s := range keep {
		if st, ok := baseStatus[s]; ok && st != "PASS" {
			contra = append(contra, fmt.Sprintf("%s（基线状态 %s）", s, st))
		}
	}
	if len(contra) > 0 {
		fmt.Fprintln(out, "章账引用自比：**台账自相矛盾** —— S_keep 里出现基线非 PASS 的步（本应进 R）⇒ 判不了（rc=2，fail-closed · §13.1 步③）")
		for _, c := range contra {
			fmt.Fprintf(out, "  ✗ %s\n", c)
		}
		return nil, nil, exitUsage
	}

	// ── ④ 章账核验（只读）：直读章账取每枚章的 head/worktree_fp（**不 --write、不续期**）。
	recs := slgLoadLedger(ledger)
	nSeals, badLines := 0, 0
	for _, r := range recs {
		if r == nil {
			continue
		}
		if _, bad := r["_unparseable"]; bad {
			badLines++
			continue
		}
		if slgChainGet(r, "kind") != "cover" {
			nSeals++
		}
	}
	if nSeals == 0 {
		fmt.Fprintf(out, "章账引用自比：章账 %s 读不到任何一枚章（可解析行 %d · 坏行 %d）⇒ 判不了（rc=2）\n", ledger, len(recs), badLines)
		return nil, nil, exitUsage
	}
	svRC, svNote := gatePartialSealVerifyLedger(root, ledger)
	if bad := gatePartialSealPartialCoverGuard(recs); len(bad) > 0 {
		fmt.Fprintln(out, "章账引用自比：**章账里出现「为部分趟盖的章」痕迹** ⇒ 判不了（rc=2，§13.1 ★ 须显式拒）")
		for _, b := range bad {
			fmt.Fprintf(out, "  ✗ cover.run_dir=%s\n", b)
		}
		return nil, nil, exitUsage
	}

	// ── ⑤ 显式参与（拒绝条件见下）────────────────────────────────────────────
	stepInFull := map[string]bool{}
	for _, n := range full {
		stepInFull[n] = true
	}
	verdicts := make([]partialSealVerdict, 0, len(keep))
	for _, s := range keep {
		v := partialSealVerdict{Step: s}
		seal, ok := gatePartialSealSealOfStep(recs, s)
		switch {
		case !ok:
			v.Reason = "章账里**无该步的章** ⇒ 不可引"
		default:
			v.Chapter = int(slgToIntOr(seal, "chapter_no", 0))
			v.SealHead = slgChainGet(seal, "head")
			v.SealFP = slgChainGet(seal, "worktree_fp")
			if miss := gatePartialSealMissingCells(seal); len(miss) > 0 {
				v.Reason = "章**缺格**：" + strings.Join(miss, " · ") + " ⇒ 章无效（硬前提①）"
			} else if !stepInFull[s] {
				v.Reason = "该步已**不在现读步集**（gap）⇒ 不可引"
			} else if !gatePartialSealFlatCompare(bHead, bFP, v.SealHead, v.SealFP) {
				v.Reason = fmt.Sprintf("**基线相对不符**（章 head=%s fp=%s ≠ 基线 head=%s fp=%s）",
					short16(v.SealHead), short16(v.SealFP), short16(bHead), short16(bFP))
			} else {
				v.Refer = true
			}
		}
		verdicts = append(verdicts, v)
	}

	for _, v := range verdicts {
		if v.Refer {
			refer = append(refer, v.Step)
		} else {
			backfill = append(backfill, v.Step)
		}
	}

	// ── ⑦ 抬头逐印（§13.1 步⑦）：只印读数，✗ 绝不渲染成 PASS/绿。
	gatePartialSealPrintHeader(out, ledger, baseDir, bHead, bFP, len(full), verdicts, svRC, svNote, nSeals)

	// ── ⑧ 纪律：全程只读；executed=true 计数恒 0；本趟不真跑被引用的步。
	fmt.Fprintf(out, "纪律    全程**只读**章账 · executed=true 计数 0（**必须 0**）· 本趟**不真跑**被引用的步（仅引用，✗ 非「沿用/继承绿」）\n")
	return refer, backfill, exitOK
}

// gatePartialSealPrintHeader —— §13.1 步⑦「抬头逐印」。
func gatePartialSealPrintHeader(out io.Writer, ledger, baseDir, bHead, bFP string, n int,
	vs []partialSealVerdict, svRC int, svNote string, nSeals int) {
	refer, backfill := 0, 0
	for _, v := range vs {
		if v.Refer {
			refer++
		} else {
			backfill++
		}
	}
	fmt.Fprintln(out, "── 章账引用自比（部分趟侧 · 零改 seal verify · §13.1 docB 条 4 / D11）─────────")
	fmt.Fprintf(out, "基线    %s · head=%s · worktree_fp=%s\n", baseDir, short16(bHead), short16(bFP))
	fmt.Fprintf(out, "章账    %s · 章 %d 枚\n", ledger, nSeals)
	fmt.Fprintf(out, "只读核  %s\n", svNote)
	fmt.Fprintf(out, "S_keep  %d 步（本趟不重跑 · 拟引用上趟绿）· N = %d 步\n", len(vs), n)
	fmt.Fprintf(out, "引用上趟绿  %d 步（章账核验过：章 head/worktree_fp 与基线**逐字相符**）\n", refer)
	fmt.Fprintf(out, "不可引回填  %d 步（⇒ 回填进 R · 本趟真跑 · D11 取「回填重跑」）\n", backfill)
	fmt.Fprintf(out, "未跑        %d 步（= 引用上趟绿集 · 本趟不真跑 · 不入本趟台账）\n", refer)
	for _, v := range vs {
		if v.Refer {
			fmt.Fprintf(out, "  [引用·章账核验过] #%d %s\n", v.Chapter, v.Step)
		}
	}
	for _, v := range vs {
		if !v.Refer {
			fmt.Fprintf(out, "  [回填重跑] %s\n", v.Step)
			fmt.Fprintf(out, "        ✗ %s\n", v.Reason)
		}
	}
}

// slgToIntOr —— 取整数格（取不到 ⇒ def）。本件只读，不改任何既有判据。
func slgToIntOr(rec map[string]interface{}, k string, def int) int {
	if rec == nil {
		return def
	}
	return slgToInt(rec[k], def)
}

// ────────────────────────────────────────────────────────────────────────────
// 自检（成对正负控 · 合成夹具 · **全在临时目录 · 不碰真目标 · 不碰 bin/zerg**）
// ────────────────────────────────────────────────────────────────────────────
func gatePartialSealSelftest() int {
	bad := 0
	chk := func(what string, ok bool, extra string) {
		mark := "   ✓ "
		if !ok {
			mark = "   ✗ "
			bad++
		}
		line := mark + what
		if !ok && extra != "" {
			line += "（" + extra + "）"
		}
		fmt.Println(line)
	}
	tmp, err := os.MkdirTemp("", "gate-partial-seal-selftest.")
	if err != nil {
		fmt.Println("自检：建临时目录失败：" + err.Error())
		return 8
	}
	defer os.RemoveAll(tmp)

	const BH = "basehead1"
	const BF = "bfp0000000000000000000000000000000000000000000000000000000000000000"
	const OTHERFP = "ofp1111111111111111111111111111111111111111111111111111111111111111"
	const OTHERHEAD = "otherhead"

	sealLine := func(no int, step, head, fp, drop string) string {
		m := map[string]interface{}{
			"chapter_no": no, "step": step, "status": "PASS", "rc": "0", "secs": "1s",
			"log": "/x/" + step + ".log", "worktree_fp": fp, "head": head,
			"item_set_digest": "isd", "judge_ver": "jv", "gate_ver": "gv",
			"executed": true, "chain": fmt.Sprintf("c%d", no),
		}
		if drop != "" {
			delete(m, drop)
		}
		b, _ := json.Marshal(m)
		return string(b)
	}
	mkBase := func(dir, head, fp string, status map[string]string, order []string) {
		_ = os.MkdirAll(dir, 0o755)
		fb, _ := json.Marshal(map[string]string{"head": head, "worktree_fp": fp, "at": "2026-10-04T00:00:00Z"})
		_ = os.WriteFile(filepath.Join(dir, "freshness.json"), fb, 0o644)
		var sb strings.Builder
		for _, s := range order {
			fmt.Fprintf(&sb, "%s\t%s\t0\t1s\t/x/%s.log\n", status[s], s, s)
		}
		_ = os.WriteFile(filepath.Join(dir, "results.tsv"), []byte(sb.String()), 0o644)
	}
	mkLedger := func(path string, lines []string) {
		_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}
	run := func(baseDir, ledger string, keep, full []string) (string, []string, []string, int) {
		var sb strings.Builder
		refer, backfill, rc := gatePartialSealRefs(tmp, baseDir, keep, full, ledger, &sb)
		return sb.String(), refer, backfill, rc
	}

	// 夹具：基线全量趟（A/B/C PASS · D FAIL）
	base := filepath.Join(tmp, "D_base")
	full := []string{"A-step", "B-step", "C-step", "D-step"}
	st := map[string]string{"A-step": "PASS", "B-step": "PASS", "C-step": "PASS", "D-step": "FAIL"}
	mkBase(base, BH, BF, st, full)

	// ① 正控：A 与基线同头同fp ⇒ 可引；B 同头异fp、C 异头同fp、D 不在S_keep ⇒ B/C 回填。
	l1 := filepath.Join(tmp, "l1.jsonl")
	mkLedger(l1, []string{
		sealLine(1, "A-step", BH, BF, ""),
		sealLine(2, "B-step", BH, OTHERFP, ""),
		sealLine(3, "C-step", OTHERHEAD, BF, ""),
		`{"chapter_no": 4, "kind": "cover", "run_dir": "/tmp/D_base", "steps": ["A-step","B-step","C-step","D-step"], "n": 3}`,
	})
	keep := []string{"A-step", "B-step", "C-step"} // S_keep = full − {D-step}
	out1, ref1, bf1, rc1 := run(base, l1, keep, full)
	chk("① 正控：rc=0 且 A 可引、B/C 回填", rc1 == 0 && len(ref1) == 1 && ref1[0] == "A-step" && len(bf1) == 2,
		fmt.Sprintf("rc=%d refer=%v backfill=%v", rc1, ref1, bf1))
	chk("① 抬头逐印：「引用上趟绿 1 步」「不可引回填 2 步」",
		strings.Contains(out1, "引用上趟绿  1 步") && strings.Contains(out1, "不可引回填  2 步"), out1)

	// ② 负控：基线缺 freshness.json ⇒ rc=2
	empty := filepath.Join(tmp, "no-fresh")
	_ = os.MkdirAll(empty, 0o755)
	_, _, _, rc2 := run(empty, l1, keep, full)
	chk("② 负控：基线取不到 freshness.json ⇒ rc=2（不给结论）", rc2 == 2, fmt.Sprintf("rc=%d", rc2))

	// ③ 负控：S_keep 里混进基线非 PASS 的步（D）⇒ 台账自相矛盾 ⇒ rc=2
	_, _, _, rc3 := run(base, l1, []string{"A-step", "D-step"}, full)
	chk("③ 负控：S_keep 含基线非 PASS ⇒ 台账自相矛盾 rc=2", rc3 == 2, fmt.Sprintf("rc=%d", rc3))

	// ④ 负控：A 章缺一格（item_set_digest）⇒ 不可引（回填）
	l4 := filepath.Join(tmp, "l4.jsonl")
	mkLedger(l4, []string{sealLine(1, "A-step", BH, BF, "item_set_digest")})
	_, ref4, bf4, rc4 := run(base, l4, keep, full)
	chk("④ 负控：章缺格 ⇒ A 不可引（回填）", rc4 == 0 && len(ref4) == 0 && len(bf4) == 3,
		fmt.Sprintf("rc=%d refer=%v backfill=%v", rc4, ref4, bf4))

	// ⑤ 负控：章账空 ⇒ rc=2（不给结论）
	l5 := filepath.Join(tmp, "empty.jsonl")
	_ = os.WriteFile(l5, []byte(""), 0o644)
	_, _, _, rc5 := run(base, l5, keep, full)
	chk("⑤ 负控：章账空 ⇒ rc=2（不给结论）", rc5 == 2, fmt.Sprintf("rc=%d", rc5))

	// ⑥ 负控：章账里出现「为部分趟盖的章」→ 最小守门 ⇒ rc=2
	l6 := filepath.Join(tmp, "l6.jsonl")
	mkLedger(l6, []string{
		sealLine(1, "A-step", BH, BF, ""),
		`{"chapter_no": 2, "kind": "cover", "run_dir": "/tmp/zerg-gates-partial-20261004", "steps": ["A-step"], "n": 1}`,
	})
	_, _, _, rc6 := run(base, l6, keep, full)
	chk("⑥ 负控：为部分趟盖的章痕迹 ⇒ 最小守门 rc=2（须显式拒）", rc6 == 2, fmt.Sprintf("rc=%d", rc6))

	// ⑦ 正控半边：D_base 目录里 results.tsv 全 PASS 的 S_keep ⇒ 全可引（当基线=run 本身）
	mkBase(base, BH, BF, map[string]string{"A-step": "PASS", "B-step": "PASS", "C-step": "PASS", "D-step": "PASS"}, full)
	l7 := filepath.Join(tmp, "l7.jsonl")
	mkLedger(l7, []string{
		sealLine(1, "A-step", BH, BF, ""),
		sealLine(2, "B-step", BH, BF, ""),
		sealLine(3, "C-step", BH, BF, ""),
		sealLine(4, "D-step", BH, BF, ""),
	})
	_, ref7, bf7, rc7 := run(base, l7, []string{"A-step", "B-step", "C-step"}, full)
	chk("⑦ 正控：三步章头/fp 均与基线逐字相符 ⇒ 三步全可引", rc7 == 0 && len(ref7) == 3 && len(bf7) == 0,
		fmt.Sprintf("rc=%d refer=%v backfill=%v", rc7, ref7, bf7))

	fmt.Println("── gate_run_partial_seal.go 自检（成对正负控 · 真两态 · 不碰 bin/zerg）──")
	fmt.Println("自检结论: " + map[bool]string{true: "全过", false: fmt.Sprintf("**不过**（%d 条）", bad)}[bad == 0])
	if bad == 0 {
		return 0
	}
	return 8
}
