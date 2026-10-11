// family_dev_reclaim.go —— `zerg dev reclaim`：**直写补正门**（缺口 `GAP-20261010-30`）。
//
// 病灶（缺口逐字）：「直写补正门」口径已定（真前态 = 该件在 `edit_audit.jsonl` 的审计末行 `after`
// → `cp` 回盘造前态 → `dev edit --from` 净零重落 · 核 sha 逐字同），但只存在于
// `~/.zerg/state/直写补正门-20261009/` 的**仓外脚本**里 —— **无命令面出口**（本仓目标「一切从 CLI 开始」）。
//
// 本命令把该口径做成**一条命令面出口**，核心五步（逐件 · fail-closed）：
//
//	① 从 `edit_audit.jsonl` 读该件**审计末行 `after`** = 真前态 sha（P）；
//	② 造前态：把 P 对应的内容**回盘**（`--pre <件>=<仓外前态件>` 或 `git show HEAD:<件>` 里与 P
//	   逐字相符的那一份）· 无审计行 ⇒ **空前态**（`mv` 出仓 —— ✗ 不删）；
//	③ 调**既有** `dev edit --from <仓外改后件>` 正门净零重落（✗ 不另造第二套审计写入）；
//	④ 核 sha 逐字同（落定后件 sha == 补正前盘上 sha）并**断言审计新增一行**（该件末行 after == D）；
//	⑤ 任一步失败即停 —— 且**恢复原盘上内容**，✗ 不留半成品坏态。
//
// 边界（照抄上游口径）：✗ 不放宽任何闸 —— 人签面照旧（真写 = 调既有 `dev edit`，它的
// `--confirm=<本机名>` ＋ `--yes` ＋ 人签批准件三条前置一字未动）；✗ 不物理删件（一律 `cp`/`mv`）；
// 取不到 / 判不了 ⇒ 非 0 不给结论（读不到件级账 ⇒ 8；真前态内容取不到 / 与审计末行 after 不符 ⇒ 2 并点名）。
//
// 为什么「调既有 `dev edit` 正门」而不是在本件里直写审计：本动作**只补审计链**（盘上内容逐字不变），
// 审计写入 / 批准件验签 / 语法自检 / 作用域闸**唯一真源**都在 `family_dev_edit.go` ⇒ 本件
// 通过**子进程**调同一份二进制（`os.Executable()`）的 `dev edit`，绝不落地第二套写盘口径。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// reclaimFields —— `--json` 面的全部字段（K1：机器面先定）。
var reclaimFields = []string{"file", "pre_source", "before_sha256", "after_sha256",
	"before_bytes", "after_bytes", "result"}

// reclaimAuditRow —— 只取本命令要的那几格（件级账一行一事件）。
type reclaimAuditRow struct {
	Event   string `json:"event"`
	File    string `json:"file"`
	After   string `json:"after_sha256"`
	Sibling string `json:"sibling_repo"`
}

// reclaimLastAuditAfter 读件级账，取该件**最后一条** `edit`（非同级仓件）行的 `after_sha256`。
// 返回 (after 全 64 位 hex, 有该行吗, 错)。整账读不到 ⇒ 错（调用方判「不给结论」· 退 8）。
func reclaimLastAuditAfter(auditPath, fileRel string) (string, bool, error) {
	b, err := os.ReadFile(auditPath)
	if err != nil {
		return "", false, err
	}
	after, found := "", false
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var r reclaimAuditRow
		if json.Unmarshal([]byte(ln), &r) != nil {
			continue
		}
		if r.Event == "edit" && r.File == fileRel && r.Sibling == "" && r.After != "" {
			after, found = r.After, true
		}
	}
	return after, found, nil
}

// reclaimGitShow 取 `<ref>:<件>` 的已提交内容（与 `dev edit --base HEAD` 同一口径族：走 git show）。
func reclaimGitShow(root, ref, fileRel string) ([]byte, error) {
	cmd := exec.Command("git", "-C", root, "show", ref+":"+fileRel)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git show %s:%s 取不到（%s）", ref, fileRel, msg)
	}
	return out.Bytes(), nil
}

// reclaimParsePre 解析 `--pre <件>=<仓外前态件路径>`（可重复）。
func reclaimParsePre(vals []string) (map[string]string, string) {
	m := map[string]string{}
	for _, v := range vals {
		v = strings.TrimSpace(v)
		i := strings.Index(v, "=")
		if i <= 0 {
			return nil, fmt.Sprintf("`--pre %s` 形态不对 —— 要 `<件>=<仓外前态件路径>`", v)
		}
		f := strings.TrimSpace(v[:i])
		p := strings.TrimSpace(v[i+1:])
		if f == "" || p == "" {
			return nil, fmt.Sprintf("`--pre %s` 里有空值（件名与前态件路径都要给）", v)
		}
		m[f] = p
	}
	return m, ""
}

// reclaimSafeName 把仓内相对路径拍成仓外临时件名的安全片段。
func reclaimSafeName(rel string) string {
	return strings.ReplaceAll(filepath.ToSlash(rel), "/", "_")
}

// reclaimDevEditArgs 拼出**既有 `dev edit` 正门**的一次调用参数（本件只补审计链 ⇒ 走正门）。
func reclaimDevEditArgs(proposalID, fileRel, cand, by, confirm string, confirmGiven bool) []string {
	args := []string{"dev", "edit", "--proposal", proposalID, "--file", fileRel, "--from", cand, "--by", by}
	if confirmGiven {
		args = append(args, "--confirm="+confirm)
	}
	return append(args, "--yes")
}

// cmdDevReclaim —— `zerg dev reclaim`：直写补正门（净零重落 · 只补审计链）。
// 三态（与 `dev edit` 同一张表）：`--dry-run` ⇒ 只出计划件（退 0）；缺 `--yes` ⇒ 计划件走 stderr + 退 2；
// `--confirm=<本机名>` + `--yes` 齐 ⇒ 真跑（调既有 `dev edit` 正门 · 人签面照旧）。
func cmdDevReclaim(inv *invocation, stdout, stderr io.Writer) int {
	if rc := dryRunYesConflict(inv, stderr); rc != exitOK {
		return rc
	}
	proposalID := strings.TrimSpace(inv.flagVal("--proposal"))
	if proposalID == "" {
		inv.setErr("usage", "missing_proposal", "缺提案 id")
		fmt.Fprintf(stderr, "%s: `dev reclaim` 必须给 `--proposal <提案 id>` —— 补正仍走**受控写面**（作用域 = 提案声明过的件）\n", progName)
		return exitUsage
	}
	files := trimAll(inv.args)
	if len(files) == 0 {
		inv.setErr("usage", "missing_file", "缺件名")
		fmt.Fprintf(stderr, "%s: `dev reclaim` 要至少一枚**仓内相对路径**件（形态：%s dev reclaim <件…> --proposal <提案 id>）\n", progName, progName)
		return exitUsage
	}
	if why := declaredFilesWhy(files); why != "" {
		inv.setErr("usage", "bad_file_path", why)
		fmt.Fprintf(stderr, "%s: %s\n", progName, why)
		return exitUsage
	}
	preMap, perr := reclaimParsePre(inv.flagVals("--pre"))
	if perr != "" {
		inv.setErr("usage", "bad_pre_flag", perr)
		fmt.Fprintf(stderr, "%s: %s\n", progName, perr)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	// 作用域：提案必须存在且声明过每一件（与 `dev edit` 同一闸 · 不另立）。
	dir := proposalDir()
	recs, err := loadProposals(dir)
	if err != nil {
		inv.setErr("blocked", "state_dir_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: 读不了提案目录 %s：%v ⇒ 不给结论（退码 8）\n", progName, dir, err)
		return exitBlocked
	}
	var prop *proposalRecord
	for i := range recs {
		if recs[i].ID == proposalID {
			prop = &recs[i]
			break
		}
	}
	if prop == nil {
		inv.setErr("usage", "proposal_not_found", "没有这个提案 id")
		fmt.Fprintf(stderr, "%s: 提案目录里没有 %q ⇒ 拒执（先 `%s dev proposal new`）\n", progName, proposalID, progName)
		return exitUsage
	}
	if prop.State != "未决" && prop.State != "已批准" {
		inv.setErr("usage", "proposal_not_open", "提案已否决")
		fmt.Fprintf(stderr, "%s: 提案 %s 的状态是 %q ⇒ 拒执\n", progName, proposalID, prop.State)
		return exitUsage
	}
	for _, f := range files {
		if !containsStr(prop.Files, f) {
			inv.setErr("usage", "file_not_declared", "件不在提案声明的清单里")
			fmt.Fprintf(stderr, "%s: **越界写被拒** —— 件 %s 不在提案 %s 声明的清单里（声明的是：%s）\n", progName, f, proposalID, orDashList(prop.Files))
			return exitUsage
		}
	}
	// 确认档：不足 ⇒ 出计划件（fail-closed · 从不提问）。
	dryRun := inv.dryRun || !(inv.confirmGiven && inv.yes)
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = strings.TrimSpace(prop.Subject)
	}
	if by == "" {
		by = "（未声明）"
	}
	auditPath := editAuditPath()
	if auditPath == "" {
		inv.setErr("blocked", "audit_path_absent", "审计落点解析不出来")
		fmt.Fprintf(stderr, "%s: 审计落点解析不出来 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	if !dryRun && !devEditHostAccept(inv.confirm) {
		inv.setErr("usage", "confirm_mismatch", "确认值不匹配本机名")
		fmt.Fprintf(stderr, "%s: 确认值不匹配目标（--confirm 给的是 %q，本机名是 %q）⇒ 不执行\n", progName, inv.confirm, planHost())
		return exitUsage
	}
	tmp, terr := os.MkdirTemp("", "dev-reclaim-")
	if terr != nil {
		inv.setErr("failed", "tmp_failed", terr.Error())
		fmt.Fprintf(stderr, "%s: 建不了仓外工作目录：%v ⇒ 停手（件未动）\n", progName, terr)
		return exitFail
	}
	defer os.RemoveAll(tmp) // 自回收（TMPDIR 前后 delta=0）

	rows := []map[string]string{}
	for _, fileRel := range files {
		outPath := filepath.Join(root, filepath.FromSlash(fileRel))
		diskBytes, rerr := os.ReadFile(outPath)
		if rerr != nil {
			inv.setErr("usage", "target_unreadable", rerr.Error())
			fmt.Fprintf(stderr, "%s: 读不了目标件 %s：%v（要补正的件必须在盘上）⇒ 拒执（退码 2）\n", progName, outPath, rerr)
			return exitUsage
		}
		D := sha256Of(diskBytes)
		P, hasLine, aerr := reclaimLastAuditAfter(auditPath, fileRel)
		if aerr != nil {
			inv.setErr("blocked", "audit_unreadable", aerr.Error())
			fmt.Fprintf(stderr, "%s: 读不了件级账 %s：%v ⇒ 不给结论（退码 8 · 缺审计行 ⇒ 拒并点名）\n", progName, auditPath, aerr)
			return exitBlocked
		}
		if hasLine && P == D {
			rows = append(rows, map[string]string{
				"file": fileRel, "pre_source": "（无需）", "before_sha256": shortSHA(P), "after_sha256": shortSHA(D),
				"before_bytes": fmt.Sprintf("%d", len(diskBytes)), "after_bytes": fmt.Sprintf("%d", len(diskBytes)),
				"result": "无需补正（审计末行 after 逐字＝现盘）"})
			fmt.Fprintf(stderr, "%s: ★ %s 审计链已完整（末行 after 逐字＝现盘 sha）⇒ 无需补正\n", progName, fileRel)
			continue
		}
		// ② 造前态：解析 P 对应的内容（`--pre` 优先 · 否则 git HEAD）；无审计行 ⇒ 空前态（mv 出仓）。
		var preBytes []byte
		preAbsent := false
		preSource := ""
		preBytesCell := ""
		switch {
		case !hasLine:
			preAbsent = true
			preSource = "（无审计行 ⇒ 空前态）"
			preBytesCell = "0"
		default:
			if p, ok := preMap[fileRel]; ok {
				b, rerr2 := os.ReadFile(p)
				if rerr2 != nil {
					inv.setErr("usage", "pre_unreadable", rerr2.Error())
					fmt.Fprintf(stderr, "%s: 读不了 `--pre` 前态件 %s：%v ⇒ 拒执（退码 2 · 判不了不给结论）\n", progName, p, rerr2)
					return exitUsage
				}
				if sha256Of(b) != P {
					inv.setErr("usage", "pre_sha_mismatch", "前态件与审计末行 after 不符")
					fmt.Fprintf(stderr, "%s: **拒执** —— `--pre` 件 %s 的 sha256 与审计末行 after 不符（要 %s，得 %s）⇒ 判不了，不给结论（退码 2）\n", progName, p, shortSHA(P), shortSHA(sha256Of(b)))
					return exitUsage
				}
				preBytes, preSource, preBytesCell = b, "仓外前态件 "+p, fmt.Sprintf("%d", len(b))
			} else {
				b, gerr := reclaimGitShow(root, "HEAD", fileRel)
				if gerr != nil {
					inv.setErr("usage", "pre_unresolvable", "取不到真前态内容")
					fmt.Fprintf(stderr, "%s: **拒执** —— %s：取不到真前态内容（既无 `--pre` · git HEAD 也取不到）⇒ 判不了，不给结论（退码 2）\n", progName, gerr)
					return exitUsage
				}
				if sha256Of(b) != P {
					inv.setErr("usage", "pre_sha_mismatch", "HEAD 版与审计末行 after 不符")
					fmt.Fprintf(stderr, "%s: **拒执** —— git HEAD 版的 sha256 与审计末行 after 不符（要 %s，得 %s）⇒ 判不了（前态未提交？用 `--pre <件>=<仓外前态件>` 交底），不给结论（退码 2）\n", progName, shortSHA(P), shortSHA(sha256Of(b)))
					return exitUsage
				}
				preBytes, preSource, preBytesCell = b, "git HEAD:"+fileRel, fmt.Sprintf("%d", len(b))
			}
		}
		if dryRun {
			rows = append(rows, map[string]string{
				"file": fileRel, "pre_source": preSource, "before_sha256": shortSHA(P), "after_sha256": shortSHA(D),
				"before_bytes": preBytesCell, "after_bytes": fmt.Sprintf("%d", len(diskBytes)), "result": "planned"})
			fmt.Fprintf(stderr, "%s: [计划] %s —— 真前态=%s · 后态=%s ⇒ 将造前态后调 `dev edit --from` 净零重落（未执行）\n",
				progName, fileRel, preSource, shortSHA(D))
			continue
		}
		// ③ 落仓外改后件（= 现盘内容 D），再回盘造前态。
		cand := filepath.Join(tmp, "after-"+reclaimSafeName(fileRel))
		if werr := os.WriteFile(cand, diskBytes, 0o644); werr != nil {
			inv.setErr("failed", "cand_write_failed", werr.Error())
			fmt.Fprintf(stderr, "%s: 写不了仓外改后件 %s：%v ⇒ 停手（件未动）\n", progName, cand, werr)
			return exitFail
		}
		held := filepath.Join(tmp, "held-"+reclaimSafeName(fileRel))
		if preAbsent {
			if rerr := os.Rename(outPath, held); rerr != nil {
				inv.setErr("failed", "mv_failed", rerr.Error())
				fmt.Fprintf(stderr, "%s: mv 出仓失败（%s）：%v ⇒ 停手（件未动）\n", progName, outPath, rerr)
				return exitFail
			}
		} else if werr := os.WriteFile(outPath, preBytes, 0o644); werr != nil {
			inv.setErr("failed", "restore_failed", werr.Error())
			fmt.Fprintf(stderr, "%s: 回盘前态失败（%s）：%v ⇒ 停手\n", progName, outPath, werr)
			return exitFail
		}
		restore := func() {
			if preAbsent {
				_ = os.Rename(held, outPath)
			} else {
				_ = os.WriteFile(outPath, diskBytes, 0o644)
			}
		}
		// ③-b 调**既有** `dev edit` 正门（净零重落 · 人签面照旧）—— 同二进制、同一条 `run` 派发路。
		devArgs := reclaimDevEditArgs(proposalID, fileRel, cand, by, inv.confirm, inv.confirmGiven)
		rc := run(devArgs, stdout, stderr)
		if rc != exitOK {
			restore()
			inv.setErr("failed", "dev_edit_failed", fmt.Sprintf("dev edit rc=%d", rc))
			fmt.Fprintf(stderr, "%s: `dev edit` 正门退 %d ⇒ **失败即停** —— 已恢复 %s 原内容（✗ 不留半成品坏态）\n", progName, rc, fileRel)
			return rc
		}
		// ④ 核 sha 逐字同 ＋ 断言审计新增一行。
		nowBytes, nerr := os.ReadFile(outPath)
		if nerr != nil || sha256Of(nowBytes) != D {
			restore()
			inv.setErr("failed", "sha_mismatch", "落定 sha 与补正前不符")
			fmt.Fprintf(stderr, "%s: **核 sha 不符** —— 落定后 %s 与补正前盘上 sha 不逐字同 ⇒ **失败即停**（已恢复原内容）\n", progName, fileRel)
			return exitFail
		}
		P2, has2, _ := reclaimLastAuditAfter(auditPath, fileRel)
		if !has2 || P2 != D {
			restore()
			inv.setErr("failed", "audit_line_absent", "审计未新增一行")
			fmt.Fprintf(stderr, "%s: **审计未新增一行**（该件末行 after 仍非现盘 sha）⇒ **失败即停**（已恢复原内容）\n", progName)
			return exitFail
		}
		rows = append(rows, map[string]string{
			"file": fileRel, "pre_source": preSource, "before_sha256": shortSHA(P), "after_sha256": shortSHA(D),
			"before_bytes": preBytesCell, "after_bytes": fmt.Sprintf("%d", len(diskBytes)), "result": "reclaimed"})
		fmt.Fprintf(stderr, "%s: ★ 补正完成 %s —— 真前态=%s · 后态=%s（逐字同 · 审计新增一行）\n",
			progName, fileRel, shortSHA(P), shortSHA(D))
	}
	if inv.jsonGiven {
		if rc := requireFields(inv, stderr); rc != exitOK {
			return rc
		}
		return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, rows)
	}
	for _, r := range rows {
		fmt.Fprintf(stdout, "  %s\t前后 sha %s → %s\t%s\t%s\n", r["file"], r["before_sha256"], r["after_sha256"], r["pre_source"], r["result"])
	}
	return exitOK
}
