// family_dev_edit.go —— 受控写面（`zerg dev edit` · D3b 第二步 · §17.3 铁律③「可受控写」）。
//
// 病灶（D3b 逐字）：七环里**唯一「缺」的那一环是「② 改」** —— 判据与取证已全在命令面，
// 而「改源码 / 写文件」今天只有「人/AI 直接编辑 + git」。本件把它变成**一条命令**，且四条硬约束：
//
//	① **作用域 = 提案声明过的件**（§17.3 铁律③①「写只落候选区」的等价物）：
//	   `--proposal <id>` 必给，`--file` 必须在那件提案的 `files[]` 里 ⇒ 没声明过的件**写不进去**（退码 2）。
//	   提案本身就是那次写的「可审查物 + 回滚路径（`rollback_ref`）」。
//	② **默认干跑**（§九 M3 `C4` 三态的第一态）：不带旗标 = `--dry-run`，只出计划件、零副作用；
//	   真写要 D3 档齐 —— `--confirm=<主机名>`（值必须与主机名逐字相同）+ `--yes`。
//	③ **逐条审计**（§九 M3 `C5`：一行一事件 · 追加只写 · **写失败即拒**）：审计行**先落盘**，
//	   落不下就不改件（「写不进日志就不许执行」）。审计记：谁 / 何时 / 改哪件 / 前后 sha256 / 字节数。
//	④ **可回滚**（§17.3 铁律③③）：件在 git 工作树里 ⇒ 回滚路径 = 提案的 `rollback_ref`（件级回滚见 `dev rollback`）。
//
// 与既有条文的接缝（**不重复立项** ✗）：写面**不新立锁**（§九 M5：真源在持锁者）、**不新立退码**
// （照 §4.1 K3 那张表）、**不改契约**；本件只是「改」这一环的**唯一入口**。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// devEditFields —— `--json` 面的全部字段（K1：机器面先定）。
var devEditFields = []string{"proposal", "file", "mode", "result", "before_sha256", "after_sha256",
	"before_bytes", "after_bytes", "audit_path", "out_path"}

// replacePair —— `--replace <件>` 里的一枚替换（JSON 数组的一格）。
// 形态：`[{"old":"…","new":"…"}, …]` —— **精确、唯一命中**才许改（模糊替换会静默改错地方）。
type replacePair struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// editAuditLine —— 审计的一行（一行一事件 · 追加只写）。
type editAuditLine struct {
	At           string `json:"at"`
	Event        string `json:"event"`
	Proposal     string `json:"proposal"`
	File         string `json:"file"`
	Mode         string `json:"mode"`
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
	BeforeBytes  int64  `json:"before_bytes"`
	AfterBytes   int    `json:"after_bytes"`
	By           string `json:"by"`
	Confirm      string `json:"confirm"`
	AuditPath    string `json:"audit_path"`
	Note         string `json:"note,omitempty"`
}

// cmdDevEdit —— `zerg dev edit`：受控写入的唯一入口（默认干跑）。
func cmdDevEdit(inv *invocation, stdout, stderr io.Writer) int {
	proposalID := strings.TrimSpace(inv.flagVal("--proposal"))
	fileRel := strings.TrimSpace(inv.flagVal("--file"))
	fromPath := strings.TrimSpace(inv.flagVal("--from"))
	replacePath := strings.TrimSpace(inv.flagVal("--replace"))

	// ①② 用法面先判（在任何盘面动作之前）
	if proposalID == "" {
		inv.setErr("usage", "missing_proposal", "缺提案 id")
		fmt.Fprintf(stderr, "%s: `dev edit` 必须给 `--proposal <提案 id>` —— 写**只允许改提案声明过的件**（§17.3 铁律③①）\n", progName)
		fmt.Fprintf(stderr, "先看现有件：%s dev proposal list\n", progName)
		return exitUsage
	}
	if fileRel == "" {
		inv.setErr("usage", "missing_file", "缺件名")
		fmt.Fprintf(stderr, "%s: `dev edit` 要一枚 `--file <仓内相对路径>`（可重复时逐次调用 —— 一次一个件，便于逐件对账）\n", progName)
		return exitUsage
	}
	if (fromPath == "") == (replacePath == "") {
		inv.setErr("usage", "bad_content_source", "内容来源要给且只给一种")
		fmt.Fprintf(stderr, "%s: 内容来源**恰给一种**：`--from <件>`（整件替换）或 `--replace <件>`（逐格精确替换）\n", progName)
		return exitUsage
	}
	if why := declaredFilesWhy([]string{fileRel}); why != "" {
		inv.setErr("usage", "bad_file_path", why)
		fmt.Fprintf(stderr, "%s: %s\n", progName, why)
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 不给结论（退码 8）\n", progName)
		return exitBlocked
	}
	outPath := filepath.Join(root, filepath.FromSlash(fileRel))
	if !strings.HasPrefix(outPath, root+string(os.PathSeparator)) {
		inv.setErr("usage", "file_outside_repo", "件不在仓根下")
		fmt.Fprintf(stderr, "%s: `--file %s` 解析后不在仓根下 ⇒ 拒执（越界写 · 退码 2）\n", progName, fileRel)
		return exitUsage
	}

	// ③ 作用域：提案必须存在，且**声明过**这一件
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
		fmt.Fprintf(stderr, "%s: 提案目录里没有 %q ⇒ 拒执（写必须有可审查物 ⇒ 先 `%s dev proposal new`）\n",
			progName, proposalID, progName)
		return exitUsage
	}
	if !containsStr(prop.Files, fileRel) {
		inv.setErr("usage", "file_not_declared", "这一件不在提案声明的改件清单里")
		fmt.Fprintf(stderr, "%s: **越界写被拒** —— 件 %s 不在提案 %s 声明的清单里（声明的是：%s）\n",
			progName, fileRel, proposalID, orDashList(prop.Files))
		fmt.Fprintf(stderr, "口径（§17.3 铁律③①）：写只允许改**提案声明过**的件 —— 要改这一件，先提一件声明它的提案\n")
		return exitUsage
	}
	if prop.State != "未决" && prop.State != "已批准" {
		inv.setErr("usage", "proposal_not_open", "提案已否决")
		fmt.Fprintf(stderr, "%s: 提案 %s 的状态是 %q ⇒ 拒执（已否决的件不许照它改码）\n", progName, proposalID, prop.State)
		return exitUsage
	}

	// ④ 算出「改完以后长什么样」+ 前后 sha256/字节数（不落盘的那一半）
	before, err := os.ReadFile(outPath) // 新件（还不存在）⇒ 空内容 + 记明「新建」
	beforeMissing := false
	if err != nil {
		if !os.IsNotExist(err) {
			inv.setErr("failed", "read_target_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 读不了目标件 %s：%v（写失败即拒 · 不吞）\n", progName, outPath, err)
			return exitFail
		}
		beforeMissing = true
		before = []byte{}
	}
	mode := "from"
	after := []byte{}
	if fromPath != "" {
		after, err = os.ReadFile(fromPath)
		if err != nil {
			inv.setErr("usage", "content_unreadable", err.Error())
			fmt.Fprintf(stderr, "%s: 读不了内容来源 %s：%v（退码 2 · 不猜内容）\n", progName, fromPath, err)
			return exitUsage
		}
	} else {
		mode = "replace"
		after, err = applyReplacements(before, replacePath)
		if err != nil {
			inv.setErr("usage", "replace_failed", err.Error())
			fmt.Fprintf(stderr, "%s: %v\n", progName, err)
			return exitUsage
		}
	}
	beforeSHA := sha256Of(before)
	afterSHA := sha256Of(after)
	auditPath := editAuditPath()
	row := map[string]string{
		"proposal": proposalID, "file": fileRel, "mode": mode,
		"before_sha256": shortSHA(beforeSHA), "after_sha256": shortSHA(afterSHA),
		"before_bytes": fmt.Sprintf("%d", len(before)), "after_bytes": fmt.Sprintf("%d", len(after)),
		"audit_path": auditPath, "out_path": outPath,
	}

	// ⑤ 干跑（默认那一态）：计划件 + 零副作用
	if inv.dryRun || !(inv.confirmGiven && inv.yes) {
		if !inv.dryRun {
			// 三态：**没带 --dry-run 但确认档不齐** ⇒ 出计划件（fail-closed：从不提问、也从不偷偷写）
			row["result"] = "planned"
			emitDevEditPlan(stdout, stderr, row, prop, beforeMissing, inv, true)
			return exitUsage
		}
		row["result"] = "planned"
		emitDevEditPlan(stdout, stderr, row, prop, beforeMissing, inv, false)
		return exitOK
	}
	if inv.confirm != planHost() {
		inv.setErr("usage", "confirm_mismatch", "确认值不匹配主机名")
		fmt.Fprintf(stderr, "%s: 确认值不匹配目标（--confirm 给的是 %q，本机主机名是 %q）⇒ 不执行\n",
			progName, inv.confirm, planHost())
		return exitUsage
	}

	// ⑥ 真写：**审计先落盘**（写不进日志就不许执行 · §九 M3 C5）⇒ 再写件 ⇒ 再自检回读 sha256
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = prop.Subject
	}
	if by == "" {
		by = "（未声明）"
	}
	line := editAuditLine{
		At: time.Now().Format(time.RFC3339), Event: "edit", Proposal: proposalID, File: fileRel,
		Mode: mode, BeforeSHA256: beforeSHA, AfterSHA256: afterSHA,
		BeforeBytes: int64(len(before)), AfterBytes: len(after), By: by,
		Confirm: inv.confirm, AuditPath: auditPath,
	}
	if beforeMissing {
		line.Note = "新建件（写前不存在）"
	}
	if err := appendEditAudit(auditPath, line); err != nil {
		inv.setErr("failed", "audit_unwritable", err.Error())
		fmt.Fprintf(stderr, "%s: **审计落不下盘 ⇒ 拒执**（§九 M3 C5「写不进日志就不许执行」）：%v\n", progName, err)
		return exitFail
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		inv.setErr("failed", "mkdir_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 建不了目录 %s：%v（审计已留痕：%s）\n", progName, filepath.Dir(outPath), err, auditPath)
		return exitFail
	}
	if err := os.WriteFile(outPath, after, 0o644); err != nil {
		inv.setErr("failed", "write_failed", err.Error())
		fmt.Fprintf(stderr, "%s: 写不进 %s：%v（审计已留痕：%s）\n", progName, outPath, err, auditPath)
		return exitFail
	}
	back, err := os.ReadFile(outPath)
	if err != nil || sha256Of(back) != afterSHA {
		inv.setErr("failed", "writeback_mismatch", "回读与写前算出的 sha256 不一致")
		fmt.Fprintf(stderr, "%s: **回读对拍不一致**（写的是 %s，盘上读到的不一样）⇒ 报失败、不报成功\n", progName, shortSHA(afterSHA))
		return exitFail
	}
	row["result"] = "written"
	if inv.jsonGiven {
		if !requireFields(inv, stderr) {
			return exitFail
		}
		return selectJSON(stdout, stderr, inv, inv.path, devEditFields, row)
	}
	fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", fileRel, shortSHA(beforeSHA), shortSHA(afterSHA), fmt.Sprintf("%d 字节", len(after)))
	fmt.Fprintf(stderr, "%s: 已改 %s（提案 %s · 审计 %s）\n", progName, outPath, proposalID, auditPath)
	fmt.Fprintf(stderr, "回滚路径：提案 %s 的退点 = %s（件级回滚见 `%s dev rollback`）\n", proposalID, orDash(prop.Rollback), progName)
	return exitOK
}

// emitDevEditPlan —— 计划件（干跑与「确认档不齐」两条路共用；零副作用）。
// blocked=true 时，最后一行点明为什么没执行（三态里 fail-closed 的那一格）。
func emitDevEditPlan(stdout, stderr io.Writer, row map[string]string, prop *proposalRecord, beforeMissing bool, inv *invocation, blocked bool) {
	fmt.Fprintln(stdout, "计划件（--dry-run · 零副作用 —— 未写任何文件、未改任何状态）")
	fmt.Fprintf(stdout, "  动作     : %s dev edit（受控写 · 只改提案声明过的件）\n", progName)
	fmt.Fprintf(stdout, "  提案     : %s（状态 %s · 声明改件 %s）\n", prop.ID, prop.State, orDashList(prop.Files))
	fmt.Fprintf(stdout, "  目标件   : %s%s\n", row["file"], ifStr(beforeMissing, "（**新建**：写前不存在）", ""))
	fmt.Fprintf(stdout, "  内容来源 : %s（mode=%s）\n", ifStr(row["mode"] == "from", "--from <件>", "--replace <件>"), row["mode"])
	fmt.Fprintf(stdout, "  前后 sha : %s → %s\n", row["before_sha256"], row["after_sha256"])
	fmt.Fprintf(stdout, "  前后字节 : %s → %s\n", row["before_bytes"], row["after_bytes"])
	fmt.Fprintf(stdout, "  审计落点 : %s（一行一事件 · 追加只写 · **写不进审计就不改件**）\n", row["audit_path"])
	fmt.Fprintf(stdout, "  回滚路径 : %s（提案的退点 + git）\n", orDash(prop.Rollback))
	if blocked {
		fmt.Fprintf(stdout, "  未执行   : D3 档确认不齐 —— 要 `--confirm=%s --yes` 同时到（fail-closed：从不提问）\n", planHost())
	}
	if inv.jsonGiven {
		_ = selectJSON(stdout, stderr, inv, inv.path, devEditFields, row)
	}
	if blocked {
		fmt.Fprintf(stderr, "%s: `dev edit` 是 D3 档（改仓内件）——**缺 `--confirm=<主机名>` 或 `--yes` ⇒ 不执行**（退码 2）\n", progName)
		return
	}
	fmt.Fprintln(stderr, "（--dry-run：只出计划件 · 零副作用 —— 未写任何文件）")
}

// applyReplacements —— 逐格精确替换（每一格的 `old` 必须**恰好命中一次**；否则拒执 —— 不猜、不模糊）。
func applyReplacements(before []byte, replacePath string) ([]byte, error) {
	body, err := os.ReadFile(replacePath)
	if err != nil {
		return nil, fmt.Errorf("读不了替换件 %s：%v", replacePath, err)
	}
	var pairs []replacePair
	if err := json.Unmarshal(body, &pairs); err != nil {
		return nil, fmt.Errorf("替换件 %s 不是 `[{\"old\":…,\"new\":…}]` 形态：%v", replacePath, err)
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("替换件 %s 是空数组 —— 空替换 = 什么也不改 ⇒ 拒执（不给结论）", replacePath)
	}
	cur := string(before)
	for i, p := range pairs {
		if p.Old == "" {
			return nil, fmt.Errorf("第 %d 格的 `old` 是空的 ⇒ 拒执（空模式会命中任意位置 —— 那是模糊替换）", i+1)
		}
		n := strings.Count(cur, p.Old)
		if n != 1 {
			return nil, fmt.Errorf("第 %d 格的 `old` 在件里命中 %d 次（要**恰好 1 次**）⇒ 拒执："+
				"改成更长的上下文，别用会命中多处的模式", i+1, n)
		}
		cur = strings.Replace(cur, p.Old, p.New, 1)
	}
	return []byte(cur), nil
}

// editAuditPath —— 审计落点：`ZERG_EDIT_AUDIT` > `<ZERG_STATE_DIR>/edit_audit.jsonl` > `~/.zerg/state/edit_audit.jsonl`。
func editAuditPath() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_EDIT_AUDIT")); d != "" {
		return d
	}
	if d := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); d != "" {
		return filepath.Join(d, "edit_audit.jsonl")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".zerg", "state", "edit_audit.jsonl")
}

// appendEditAudit —— 追加一行（O_APPEND · 一行一事件）。**失败即拒**（调用方据此不改件）。
func appendEditAudit(path string, line editAuditLine) error {
	if path == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(line)
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

// sha256Of / shortSHA —— 现算 sha256（审计与对拍都读它，不缓存、不估计）。
func sha256Of(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func shortSHA(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:16]
}

// orDashList / ifStr —— 两枚只在报面上用的小工具（空清单 ⇒ `（未声明）`）。
func orDashList(v []string) string {
	if len(v) == 0 {
		return "（未声明）"
	}
	return strings.Join(v, " · ")
}

func ifStr(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}
