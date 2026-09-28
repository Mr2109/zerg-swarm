// gate_freshness.go —— **门新鲜度闸**（缺口 `GAP-20260928-194` · 设计稿《设计-流程规则程序化-v1.0-20260928》§2.2 的 `A2`/`A5` 条 · §4 第 2 件）。
//
// 病灶（设计稿逐字）：
//
//	A2 片收口至少跑一次含单测的贵步门 —— 程序面**无**（快速档 `--fast` 不含单测步，跑完照样能提交）；
//	A5 门结果与工作树不许两代分离 —— 程序面**无**。
//
// 本件的两条口径（**唯一真源**，别处不许再写一份）：
//
//	① 指纹定义：把「提交可能包含的件」列全（tracked + 未跟踪未忽略），逐个取**工作树内容** blob sha，
//	   整串再 hash 一次 ⇒ **提交不换内容 ⇒ 指纹不变**；改一个字节 ⇒ 指纹立刻变。
//	   用的 git 命令都**只读**（`--no-optional-locks` 明令不刷索引）⇒ 算指纹不写盘。
//	② 全链：那一趟的 `results.tsv` 步名集合 ⊇ **门件自己现读的清单**（`bash <脚本> --list`，
//	   逐字照脚本 `print_steps` 的出口解析）⇒ 步数不写死；且清单里「单测那一步」都在且都 PASS。
//
// 退码语义（本件只**判**，不改任何既有出口）：
//
//	0 找得到「全链 + 单测通过 + 指纹逐字相符」的一趟 ⇒ 放行；
//	2 找不到 ⇒ **拒**（逐条点名差在哪里 + 给出该跑的完整真命令）；
//	8 判不了（指纹算不出 / 清单读不回 / 趟目录读不动）⇒ 不给结论。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// gateFreshnessFile —— 门跑那一趟产物里的**指纹随件**（与 `results.tsv` 同目录）。
const gateFreshnessFile = "freshness.json"

// gateFreshnessRecord —— 随件的一行（只两格真源：当时指纹 / 当时 HEAD）。
type gateFreshnessRecord struct {
	Head        string `json:"head"`
	Fingerprint string `json:"worktree_fp"`
	At          string `json:"at"`
}

// gateGitReadOnly —— 只读 git（**只此一处**：指纹口径的两个命令都走它）。
func gateGitReadOnly(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var out, errb strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s：%v（%s）", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// worktreeFingerprint —— 口径①（唯一一处实现）。返回（指纹 64-hex · HEAD 短 sha）。
func worktreeFingerprint(root string) (string, string, error) {
	h, err := gateGitReadOnly(root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", "", err
	}
	head := strings.TrimSpace(h)
	// ★ 口径①（2026-09-29 修）：指纹取**内容**，不取「HEAD + diff」。
	//   为什么改：`sha256(HEAD + diff)` 在**提交那一刻**就会变（HEAD 前进、diff 清空），
	//   而树的**内容**其实一个字节没动 ⇒ 变成「每提交一次就得重跑一趟全链」，把闸变成了负担。
	//   新口径 = 先把「提交可能包含的件」列全（tracked + 未跟踪但未被忽略），逐个取**工作树内容**的
	//   blob sha，再整串 hash 一次 ⇒ **提交不换内容 ⇒ 指纹不变**；改一个字节（哪怕没跑门）⇒ 指纹立刻变。
	ls, err := gateGitReadOnly(root, "--no-optional-locks", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return "", "", err
	}
	names := []string{}
	for _, n := range strings.Split(strings.TrimRight(ls, "\x00"), "\x00") {
		if n != "" {
			names = append(names, n)
		}
	}
	sum := sha256.New()
	const batch = 200
	for a := 0; a < len(names); a += batch {
		b := a + batch
		if b > len(names) {
			b = len(names)
		}
		args := append([]string{"--no-optional-locks", "hash-object", "--"}, names[a:b]...)
		out, err := gateGitReadOnly(root, args...)
		blobs := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if err != nil || len(blobs) != b-a {
			// ★ 回退（2026-09-29 血证）：本批里混着 git 取不了内容的件
			//   （管线 / socket / 断链 —— 现读实测 `vendor/rtk`）⇒ 一条怪件会把整批、进而把
			//   整个指纹算废，把提交正门堵死（rc=8）。改为：逐件取，取不到的记常量 `-`。
			//   它们本来也进不了提交 ⇒ 记常量不影响「提交可能包含的内容」这一口径。
			blobs = make([]string, b-a)
			for k, n := range names[a:b] {
				one, e1 := gateGitReadOnly(root, "--no-optional-locks", "hash-object", "--", n)
				if e1 != nil {
					blobs[k] = "-"
				} else {
					blobs[k] = strings.TrimSpace(one)
				}
			}
		}
		for k, n := range names[a:b] {
			fmt.Fprintf(sum, "%s\x00%s\x00", n, strings.TrimSpace(blobs[k]))
		}
	}
	return hex.EncodeToString(sum.Sum(nil)), head, nil
}

// writeGateFreshness —— 把**门跑当时**的指纹与 HEAD 写进那一趟的产物目录。
// ★ 只写这一枚随件；写不下去**不改**本命令的任何退码（`gate.go` 那条「只转发、不翻译」不下岗）。
func writeGateFreshness(dir, root, fp, head string) error {
	if dir == "" || fp == "" {
		return fmt.Errorf("没有可写的趟目录或指纹为空")
	}
	if _, err := os.Stat(filepath.Join(dir, gateResultsTSV)); err != nil {
		return fmt.Errorf("目录里没有 %s ⇒ 不是一趟", gateResultsTSV)
	}
	rec := gateFreshnessRecord{Head: head, Fingerprint: fp, At: time.Now().Format(time.RFC3339)}
	body, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, gateFreshnessFile), append(body, '\n'), 0o644)
}

// readGateFreshness —— 只读随件（读不到 ⇒ 空记录 + err；调用方照实印「未记」）。
func readGateFreshness(dir string) (gateFreshnessRecord, error) {
	var rec gateFreshnessRecord
	b, err := os.ReadFile(filepath.Join(dir, gateFreshnessFile))
	if err != nil {
		return rec, err
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		return rec, err
	}
	return rec, nil
}

// gateRunOutdirArg —— 用户显式给的 `--outdir`（没有 ⇒ 空串：由脚本自己按时间戳建）。
func gateRunOutdirArg(tail []string) string {
	for i, a := range tail {
		if a == "--outdir" && i+1 < len(tail) {
			return tail[i+1]
		}
		if strings.HasPrefix(a, "--outdir=") {
			return strings.TrimPrefix(a, "--outdir=")
		}
	}
	return ""
}

// gateRunNewestOutdirSince —— 跑完之后认「这一趟」的目录：扫 `<TMPDIR>/zerg-gates-*`，
// 取带 `results.tsv` 且其 mtime 落在 `[since-3s, now]` 的**最新**一份（防把旧趟当这一趟）。
func gateRunNewestOutdirSince(since time.Time) string {
	base := os.TempDir()
	ents, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	best := ""
	var bestT time.Time
	for _, e := range ents {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), gateRunPrefix) {
			continue
		}
		p := filepath.Join(base, e.Name())
		st, err := os.Stat(filepath.Join(p, gateResultsTSV))
		if err != nil {
			continue
		}
		if st.ModTime().Before(since.Add(-3 * time.Second)) {
			continue
		}
		if best == "" || st.ModTime().After(bestT) {
			best, bestT = p, st.ModTime()
		}
	}
	return best
}

// gateFullStepNames —— 口径②的左半：**门件自己现读的完整步集**（`bash <脚本> --list`）。
// 解析逐字照脚本 `print_steps` 的出口：`printf '   %-5s %-6s %s\n'` ⇒ 头两列之外**原样**是步名。
func gateFullStepNames(root string) ([]string, error) {
	script := filepath.Join(root, gateScriptRel)
	if _, err := os.Stat(script); err != nil {
		return nil, fmt.Errorf("门禁最小入口不在（%s）", gateScriptRel)
	}
	out, err := exec.Command("bash", script, "--list").Output()
	if err != nil {
		return nil, fmt.Errorf("`%s --list` 跑不动：%v", gateScriptRel, err)
	}
	names := []string{}
	for _, ln := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(ln, "   ") {
			continue // 表头（以 ─ 起）与计数（以 共 起）都不带这个前导
		}
		rest := strings.TrimLeft(ln, " ")
		i := strings.IndexByte(rest, ' ')
		if i < 0 {
			continue
		}
		rest = strings.TrimLeft(rest[i:], " ")
		j := strings.IndexByte(rest, ' ')
		if j < 0 {
			continue
		}
		// 头两列都是 `printf` 的**定宽**域（`%-5s`/`%-6s`）⇒ 它俩后面可能不止一个空格；
		// 步名从「第二列之后的第一个非空字符」起**原样**取到行尾（脚本 `print_steps` 用 `%s` 原样印）。
		if name := strings.TrimLeft(rest[j:], " "); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("`%s --list` 一条步骤都没读出来", gateScriptRel)
	}
	return names, nil
}

// gateUnitTestSteps —— 口径②的右半：「单测那一步」**从现读清单里认**（不写死步名、不写死数字）。
// 判据 = 步名里含 `go test ./...`（整模块单测那几个门步；`go test -race ./...` 不在此列）。
func gateUnitTestSteps(full []string) []string {
	out := []string{}
	for _, n := range full {
		if strings.Contains(n, "go test ./...") {
			out = append(out, n)
		}
	}
	return out
}

// gateFreshnessCandidate —— 一趟候选（有结果表 + 有随件的那一趟）。
type gateFreshnessCandidate struct {
	Dir   string
	Rows  []gateResultsRow
	Rec   gateFreshnessRecord
	HasFP bool
}

// gateFreshnessCandidates —— 扫 `<TMPDIR>/zerg-gates-*` 里所有**带结果表**的趟（只读）。
func gateFreshnessCandidates() ([]gateFreshnessCandidate, error) {
	base := os.TempDir()
	ents, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("读不到临时目录 %s：%w", base, err)
	}
	out := []gateFreshnessCandidate{}
	for _, e := range ents {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), gateRunPrefix) {
			continue
		}
		p := filepath.Join(base, e.Name())
		if _, err := os.Stat(filepath.Join(p, gateResultsTSV)); err != nil {
			continue
		}
		rows, _, err := gateResultsReadTSV(filepath.Join(p, gateResultsTSV))
		if err != nil {
			continue
		}
		c := gateFreshnessCandidate{Dir: p, Rows: rows}
		if rec, err := readGateFreshness(p); err == nil {
			c.Rec, c.HasFP = rec, true
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir > out[j].Dir })
	return out, nil
}

// gateFreshnessMissing —— 这一趟相对现读清单**缺的步**（逐条点名用）。
func gateFreshnessMissing(c gateFreshnessCandidate, full []string) []string {
	have := map[string]string{}
	for _, r := range c.Rows {
		have[r.Name] = r.Status
	}
	missing := []string{}
	for _, n := range full {
		if _, ok := have[n]; !ok {
			missing = append(missing, n)
		}
	}
	return missing
}

// gateFreshnessStatusOf —— 这一趟里某一步的状态（不在 ⇒ 空串）。
func gateFreshnessStatusOf(c gateFreshnessCandidate, name string) string {
	for _, r := range c.Rows {
		if r.Name == name {
			return r.Status
		}
	}
	return ""
}

// gateFreshnessVerdict —— 一趟是否满足放行三条件；返回（满不满足 · 逐条差在哪）。
func gateFreshnessVerdict(c gateFreshnessCandidate, full, unit []string, curFP string) (bool, []string) {
	diffs := []string{}
	missing := gateFreshnessMissing(c, full)
	if len(missing) > 0 {
		diffs = append(diffs, fmt.Sprintf("缺步 %d/%d（这一趟只有 %d 步）：%s",
			len(missing), len(full), len(c.Rows), strings.Join(clipList(missing, 4), " · ")))
	}
	badUnit := []string{}
	for _, u := range unit {
		switch st := gateFreshnessStatusOf(c, u); st {
		case "":
			badUnit = append(badUnit, u+"（不在这一趟里）")
		case "PASS":
		default:
			badUnit = append(badUnit, fmt.Sprintf("%s（状态 %s）", u, st))
		}
	}
	if len(badUnit) > 0 {
		diffs = append(diffs, "单测步未全通过："+strings.Join(badUnit, " · "))
	}
	if !c.HasFP {
		diffs = append(diffs, "这一趟没有指纹随件（`"+gateFreshnessFile+"`）⇒ 指纹不可比")
	} else if c.Rec.Fingerprint != curFP {
		diffs = append(diffs, fmt.Sprintf("指纹不符：这一趟 %s ≠ 当前 %s", short16(c.Rec.Fingerprint), short16(curFP)))
	}
	return len(diffs) == 0, diffs
}

func clipList(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	out := append([]string{}, xs[:n]...)
	return append(out, fmt.Sprintf("…另 %d 条", len(xs)-n))
}

func short16(s string) string {
	if s == "" {
		return "（空）"
	}
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

// gateStaleWaiver —— `--allow-stale-gates <一句话理由>` 的**留痕跳道**（不是白名单）。
// 真留一笔痕（写进现成的审计落点 `commit_audit.jsonl`）；**落不下痕 ⇒ 不许跳**（退 2）。
func gateStaleWaiver(root, mode, reason string, diffs []string, stderr io.Writer) int {
	if strings.TrimSpace(reason) == "" {
		fmt.Fprintf(stderr, "%s: `--allow-stale-gates` **不带理由**（裸跳）⇒ 拒执（退码 2）—— 跳道必须留一句人话理由\n", progName)
		return exitUsage
	}
	head, _, _ := worktreeFingerprint(root)
	ap := commitAuditPath()
	line := commitAuditLine{
		At: time.Now().Format(time.RFC3339), Event: "gate_stale_waived", Mode: mode,
		Head: head, Reason: reason, AuditPath: ap,
		Note: "门新鲜度闸被显式跳过（--allow-stale-gates）：" + strings.Join(diffs, " ； "),
	}
	if err := appendCommitAudit(ap, line); err != nil {
		fmt.Fprintf(stderr, "%s: 跳道的**留痕写不下去**（%s：%v）⇒ 拒执（退码 2）—— 无痕不许跳\n", progName, ap, err)
		return exitUsage
	}
	fmt.Fprintf(stderr, "%s: ⚠ **门新鲜度闸被显式跳过**（`--allow-stale-gates`）—— 理由已留痕到 `%s`：%s\n", progName, ap, reason)
	fmt.Fprintf(stderr, "%s:   跳过时差的正是这几条：%s\n", progName, strings.Join(diffs, " ； "))
	return exitOK
}

// gateFreshnessGate —— 提交正门的新闸（`repo commit` / `repo commit --only` 共用这一处）。
// 三条件全中 ⇒ 0（放行）；差任何一条 ⇒ 2（逐条点名 + 给出该跑的真命令）；判不了 ⇒ 8。
// 留痕跳道：`--allow-stale-gates <理由>` 给了理由 ⇒ 拒的那一态改成「留一笔痕后放行」（落不下痕 ⇒ 仍拒）。
func gateFreshnessGate(root, mode string, inv *invocation, stderr io.Writer) int {
	// ⓪ 裸跳一律先拒（不传理由的跳道不存在）—— 判在**任何**门跑产物之前：理由这一格与「有没有那一趟」无关。
	waiveGiven := inv.hasFlag("--allow-stale-gates")
	reason := strings.TrimSpace(inv.flagVal("--allow-stale-gates"))
	if waiveGiven && reason == "" {
		inv.setErr("usage", "stale_waiver_requires_reason", "`--allow-stale-gates` 必须同时给一句话理由")
		fmt.Fprintf(stderr, "%s: `--allow-stale-gates` 不带理由（裸跳）⇒ 拒执（退码 2）—— 跳道必须留一句人话理由\n", progName)
		fmt.Fprintf(stderr, "  形态：--allow-stale-gates <为什么这一趟可以跳过门新鲜度闸>\n")
		return exitUsage
	}
	curFP, curHead, err := worktreeFingerprint(root)
	if err != nil {
		inv.setErr("blocked", "worktree_fingerprint_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 算不出当前工作树指纹 ⇒ 不给结论（退码 8）：%v\n", progName, err)
		return exitBlocked
	}
	full, err := gateFullStepNames(root)
	if err != nil {
		inv.setErr("blocked", "gate_step_list_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不回门件自己的步骤清单 ⇒ 不给结论（退码 8）：%v\n", progName, err)
		return exitBlocked
	}
	unit := gateUnitTestSteps(full)
	cands, err := gateFreshnessCandidates()
	if err != nil {
		inv.setErr("blocked", "gate_runs_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不动门跑产物目录 ⇒ 不给结论（退码 8）：%v\n", progName, err)
		return exitBlocked
	}
	// 拒的那一态：先收集「差在哪里」，再统一出口（跳道也挂在这一处 —— 它跳的是**闸**，与有没有那一趟无关）。
	diffs := []string{}
	best := ""
	if len(cands) == 0 {
		diffs = append(diffs, fmt.Sprintf("没有可用的一趟：`%s` 下一份带 `%s` 的门跑产物都没有", os.TempDir(), gateResultsTSV))
	} else {
		nDiff := -1
		for i := range cands {
			ok, ds := gateFreshnessVerdict(cands[i], full, unit, curFP)
			if ok {
				fmt.Fprintf(stderr, "%s: 门新鲜度闸 ✓ 全链 %d 步 · 单测步 %d 个全 PASS · 指纹 %s 逐字相符（那一趟 %s）\n",
					progName, len(full), len(unit), short16(curFP), cands[i].Dir)
				return exitOK
			}
			if nDiff < 0 || len(ds) < nDiff {
				best, diffs, nDiff = cands[i].Dir, ds, len(ds)
			}
		}
	}
	inv.setErr("usage", "stale_gate_run", "没有一趟「全链 + 单测通过 + 指纹相符」的门跑结果")
	fmt.Fprintf(stderr, "%s: **门新鲜度闸不过 ⇒ 拒提交**（退码 2）—— 当前工作树指纹 %s · HEAD %s\n",
		progName, short16(curFP), curHead)
	fmt.Fprintf(stderr, "  现读全链步数 %d · 单测步 %d 个 · 扫过 %d 趟门跑结果%s\n",
		len(full), len(unit), len(cands), bestNote(best))
	for _, d := range diffs {
		fmt.Fprintf(stderr, "    ✗ %s\n", d)
	}
	fmt.Fprintf(stderr, "  该跑的完整命令：%s gate run\n", progName)
	fmt.Fprintf(stderr, "  （口径：只认「整套全链 + 含单测步且通过 + 指纹逐字相符」；要显式跳过必须带理由 `--allow-stale-gates <一句话理由>`）\n")
	if waiveGiven {
		return gateStaleWaiver(root, mode, reason, diffs, stderr)
	}
	return exitUsage
}

// bestNote —— 「最接近的一趟」那一格的尾巴（没有候选时不留）。
func bestNote(dir string) string {
	if dir == "" {
		return ""
	}
	return "；最接近的一趟 " + dir
}
