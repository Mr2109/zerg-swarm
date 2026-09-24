// family_doc.go —— `zerg doc meta fill`：**批量回填文件头**（缺口-命令面-20260921 §八 · 开工记录 D3 §三
// 新增第 1 条 · 2026-09-21）。
//
// 病灶（D3 的实测 · 原样）：③ 那 63 篇缺「日期 + 不开源标注」的文件头是**一次性脚本**回填的
// （`/tmp/d3-head-fill.py`，手搓 · 不进仓）—— 命令面**没有「批量改字段」这条路**。本件把它变成一条命令。
//
// 三条硬口径（照 D3③-a 那一笔的做法，**不自造第二套**）：
//
//	① **默认干跑**：不带 `--dry-run` 与 `--yes` ⇒ 出计划件、**一个字节都不写**（rc=2，`--yes` 缺 ⇒ 不执行）；
//	   显式 `--dry-run` ⇒ 计划件（rc=0）。真写要 `--yes`。
//	② **只填机械可判的字段**：① 日期 · ② 「本稿不开源」标注。
//	   日期的来源只有两个既有事实：**文件名里的 `-YYYYMMDD`**；文件名无日期 ⇒ 该件**最后一次提交日**
//	   （`git log -1 --format=%cs`）。两个都取不到 ⇒ 那件**不给结论**（跳过并计数 · 退码 8），**不新造日期** ✗。
//	③ **不碰正文语义** ✗：只在 H1 之后**插入两行**（标注行 + 空行），既有行一个字不改；已有标注的件一律跳过（幂等）。
//
// 写面纪律同 `dev edit`（§九 M3 C5）：**审计先落盘**（一行一事件 · 追加只写），落不下就不改件。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// docMetaFillFields —— `--json` 的全部字段（K1：机器面先定）。
var docMetaFillFields = []string{"file", "date", "date_source", "action", "reason",
	"before_sha256", "after_sha256", "before_bytes", "after_bytes", "audit_path"}

// docAnnotationTail —— 标注行 `> <日期>` 之后的半句（与同目录既有 30 篇 + D3③-a 那 62 篇**逐字相同**）。
const docAnnotationTail = " · **本稿不开源**（开发文档侧：与主仓并列、不进公开面与站点导出）。"

// docHeadScanLines —— 「抬头」的行数口径（与 `zerg code find '不开源'` 的抬头判据同口径：前 12 行）。
const docHeadScanLines = 12

// docNameDateRE —— 文件名里的日期后缀（`-YYYYMMDD`）；抓不到就走「该件最后一次提交日」。
var docNameDateRE = regexp.MustCompile(`-(\d{4})(\d{2})(\d{2})`)

// docMetaAuditLine —— 审计的一行（一行一事件 · 追加只写）。
type docMetaAuditLine struct {
	At           string `json:"at"`
	Event        string `json:"event"`
	Root         string `json:"root"`
	File         string `json:"file"`
	Date         string `json:"date"`
	DateSource   string `json:"date_source"`
	InsertAt     int    `json:"insert_at"`
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
	BeforeBytes  int64  `json:"before_bytes"`
	AfterBytes   int    `json:"after_bytes"`
	By           string `json:"by"`
	AuditPath    string `json:"audit_path"`
}

// docMetaCandidate —— 一件待回填的文件（计划面与写面共用同一份读数）。
type docMetaCandidate struct {
	Rel        string // 相对被扫根的路径
	Abs        string
	Before     []byte
	After      []byte
	Date       string
	DateSource string
	InsertAt   int
	Reason     string // 非空 ⇒ 这一件**不给结论**（跳过 · 不写）
}

// cmdDocMeta —— `zerg doc meta <动作>` 的分发口（与 `approve` / `dev proposal` 一族同一形状：
// 族 = `doc meta`（命令树里两条词）· 动作 = `args[0]`；未知名 ⇒ 退 2 + 可用动作，
// **不偷偷当成别的动作**（K14：错误文案要给下一步）。
func cmdDocMeta(inv *invocation, stdout, stderr io.Writer) int {
	action := ""
	if len(inv.args) > 0 {
		action = inv.args[0]
	}
	switch action {
	case "fill":
		inv.args = inv.args[1:]
		return cmdDocMetaFill(inv, stdout, stderr)
	}
	inv.setErr("usage", "unknown_action", "未知动作")
	if action == "" {
		fmt.Fprintf(stderr, "%s: `doc meta` 要给一个动作（本版：fill）\n", progName)
	} else {
		fmt.Fprintf(stderr, "%s: 未知 `doc meta` 动作 %q（本版只有：fill）\n", progName, action)
	}
	fmt.Fprintf(stderr, "形态：%s doc meta fill [--scope devdocs | <目录>] [--dry-run] [--json <字段>] [--yes]\n", progName)
	return exitUsage
}

// cmdDocMetaFill —— `zerg doc meta fill [--scope devdocs|<根>] [--dry-run] [--json <字段>] [--yes]`。
func cmdDocMetaFill(inv *invocation, stdout, stderr io.Writer) int {
	scope := strings.TrimSpace(inv.flagVal("--scope"))
	if scope == "" && len(inv.args) > 0 {
		// 位置参数也认（`zerg doc meta fill <目录>`）—— 动作那一条已被 cmdDocMeta 剥掉
		scope = strings.TrimSpace(inv.args[0])
	}
	by := strings.TrimSpace(inv.flagVal("--by"))
	if by == "" {
		by = "（未声明）"
	}

	// ① 落点（用法面先判：**在任何盘面动作之前**）
	root, why := docFillRoot(scope, strings.TrimSpace(inv.flagVal("--docs-ver")))
	if why != "" {
		inv.setErr("usage", "bad_root", why)
		fmt.Fprintf(stderr, "%s: %s ⇒ 退码 2\n", progName, why)
		fmt.Fprintf(stderr, "可用：`--scope devdocs`（ZERG_DEVDOCS_ROOT > 版本档案取源根下「版本号最大且 ≥3 篇」的版本目录，可用 `--docs-ver <X.Y.Z>` 钉版）或给一个目录\n")
		return exitUsage
	}
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		inv.setErr("usage", "root_not_dir", "落点不是目录")
		fmt.Fprintf(stderr, "%s: 落点不是目录：%s（给 `--scope devdocs` 或一个**已存在**的目录）⇒ 退码 2\n", progName, root)
		return exitUsage
	}

	// ② 扫件：只认 `*.md`（用例 = 那 63 篇）；点目录跳过。
	files, serr := docMetaMDs(root)
	if serr != nil {
		inv.setErr("blocked", "walk_failed", serr.Error())
		fmt.Fprintf(stderr, "%s: 扫不动 %s（%v）⇒ 不给结论（退码 8）\n", progName, root, serr)
		return exitBlocked
	}

	// ③ 逐件读数（**只读**）：缺标注的才进候选；日期的两个来源都取不到 ⇒ 该件不给结论。
	cands := []docMetaCandidate{}
	for _, rel := range files {
		c, need := docMetaPlan(root, rel)
		if !need {
			continue
		}
		cands = append(cands, c)
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].Rel < cands[j].Rel })

	auditPath := docMetaAuditPath()
	rows := []map[string]string{}
	wrote, undecidable := 0, 0
	write := inv.yes
	for i := range cands {
		c := &cands[i]
		row := map[string]string{
			"file": c.Rel, "date": c.Date, "date_source": c.DateSource,
			"before_sha256": shortSHA(sha256Of(c.Before)), "before_bytes": fmt.Sprintf("%d", len(c.Before)),
			"after_sha256": "（未算：这一件不给结论）", "after_bytes": "0",
			"audit_path": auditPath, "action": "skipped", "reason": c.Reason,
		}
		if c.Reason != "" {
			undecidable++
			rows = append(rows, row)
			continue
		}
		row["after_sha256"] = shortSHA(sha256Of(c.After))
		row["after_bytes"] = fmt.Sprintf("%d", len(c.After))
		if !write {
			row["action"], row["reason"] = "planned", "（干跑：零副作用 · 未写任何文件）"
			rows = append(rows, row)
			continue
		}
		// ④ 真写：**审计先落盘**（写不进日志就不许执行 · §九 M3 C5）⇒ 再落件 ⇒ 再回读对拍 sha256
		line := docMetaAuditLine{
			At: time.Now().Format(time.RFC3339), Event: "doc_meta_fill", Root: root, File: c.Rel,
			Date: c.Date, DateSource: c.DateSource, InsertAt: c.InsertAt,
			BeforeSHA256: sha256Of(c.Before), AfterSHA256: sha256Of(c.After),
			BeforeBytes: int64(len(c.Before)), AfterBytes: len(c.After), By: by, AuditPath: auditPath,
		}
		if err := appendDocMetaAudit(auditPath, line); err != nil {
			inv.setErr("failed", "audit_unwritable", err.Error())
			fmt.Fprintf(stderr, "%s: **审计落不下盘 ⇒ 拒执**（§九 M3 C5）：%v\n", progName, err)
			return exitFail
		}
		// 序138 的「先复原再报」：写前登记本件的现盘样子（同 `dev edit` 那一处，逐条给理由）。
		_ = wallclockRecordPreImage(c.Abs)
		if err := os.WriteFile(c.Abs, c.After, 0o644); err != nil {
			inv.setErr("failed", "write_failed", err.Error())
			fmt.Fprintf(stderr, "%s: 写不进 %s：%v（审计已留痕：%s）\n", progName, c.Abs, err, auditPath)
			return exitFail
		}
		back, rerr := os.ReadFile(c.Abs)
		if rerr != nil || sha256Of(back) != line.AfterSHA256 {
			inv.setErr("failed", "writeback_mismatch", "回读与写前算出的 sha256 不一致")
			fmt.Fprintf(stderr, "%s: **回读对拍不一致**（%s）⇒ 报失败、不报成功\n", progName, c.Rel)
			return exitFail
		}
		row["action"], row["reason"] = "written", ""
		wrote++
		rows = append(rows, row)
	}

	// ⑤ 报面
	fmt.Fprintf(stderr, "%s: `doc meta fill` 根 %s · 扫到 %d 件 md · 待回填 %d 件 · %s\n",
		progName, root, len(files), len(cands), ifStr(write, fmt.Sprintf("已写 %d 件", wrote), "干跑（零副作用）"))
	fmt.Fprintf(stderr, "  只填**机械可判**的两样：日期（文件名 `-YYYYMMDD` > 该件最后一次提交日）+「本稿不开源」标注；\n")
	fmt.Fprintf(stderr, "  **不碰正文语义**（只在 H1 之后插入两行 · 既有行一个字不改 · 已有标注的跳过 ⇒ 幂等）\n")
	fmt.Fprintf(stderr, "  审计：%s（一行一事件 · 追加只写 · **写不进审计就不改件**）\n", auditPath)
	if undecidable > 0 {
		fmt.Fprintf(stderr, "%s: **%d 件不给结论**（日期两个来源都取不到 · 或读不到 H1）⇒ 跳过不写、退码 8（**不许当绿**）\n",
			progName, undecidable)
	}
	if !inv.yes && !inv.dryRun {
		inv.setErr("usage", "missing_yes", "缺 --yes")
		fmt.Fprintf(stderr, "%s: 这一步是 D2 档（改文件）—— **缺 `--yes` ⇒ 不执行**（退码 2）；先看 `--dry-run` 的计划件\n", progName)
	}
	if rc := listCmd(inv, stdout, stderr, docMetaFillFields, rows); rc != exitOK {
		return rc
	}
	if undecidable > 0 {
		return exitBlocked
	}
	if !inv.yes && !inv.dryRun {
		return exitUsage
	}
	return exitOK
}

// docFillRoot —— 被扫根：`--scope devdocs`（或缺省）⇒ 开发文档面**当前版**根；否则把它当**路径**。
//
// ★ 版本无关（缺口 `G-19`）：开发文档面根**不再钉在某一版**（旧代码逐字写着 `v2.5.10` ⇒ 本版新件全在扫描面外，
// 「第四次成文重出」）。现在取源**只有一处** `devDocsCurrentVersionDir()`（与 `help export` 同源）：
// `ZERG_DEVDOCS_ROOT` > `<版本档案取源根>/v<--docs-ver 钉的那版>` > `<版本档案取源根>` 下「版本号最大且 ≥3 篇」的那个。
func docFillRoot(scope, pinVersion string) (root, why string) {
	if scope == "" || scope == "devdocs" {
		if v := strings.TrimSpace(os.Getenv("ZERG_DEVDOCS_ROOT")); v != "" {
			return v, ""
		}
		return devDocsCurrentVersionDir(pinVersion)
	}
	if strings.HasPrefix(scope, "-") {
		return "", fmt.Sprintf("`--scope` 要一个面（`devdocs`）或一个目录，拿到的是旗标 %q", scope)
	}
	if filepath.IsAbs(scope) {
		return scope, ""
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", "取不到工作目录：" + err.Error()
	}
	return filepath.Join(wd, scope), ""
}

// docMetaMDs —— 被扫根下的全部 `*.md`（相对路径 · 排序 · 点目录与点件跳过）。
func docMetaMDs(root string) ([]string, error) {
	out := []string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(strings.ToLower(name), ".md") {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out, err
}

// docMetaPlan —— 一件的计划（**只读**）：返回 (候选, 要不要处理)。
// 已有标注（抬头 12 行内有「不开源」）⇒ 不处理（幂等）。
func docMetaPlan(root, rel string) (docMetaCandidate, bool) {
	abs := filepath.Join(root, rel)
	before, err := os.ReadFile(abs)
	if err != nil {
		return docMetaCandidate{Rel: rel, Abs: abs, Reason: "读不到件：" + err.Error()}, true
	}
	c := docMetaCandidate{Rel: rel, Abs: abs, Before: before}
	lines := strings.Split(string(before), "\n")
	head := lines
	if len(head) > docHeadScanLines {
		head = head[:docHeadScanLines]
	}
	for _, ln := range head {
		if strings.Contains(ln, "不开源") {
			return c, false // 已有标注 ⇒ 跳过（幂等）
		}
	}
	// H1 在哪（`# ` 开头的第一行）—— 找不到 ⇒ 这一件不给结论（插入点不可判 · 不猜）
	h1 := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "# ") {
			h1 = i
			break
		}
	}
	if h1 < 0 {
		c.Reason = "读不到 H1（插入点机械上判不了）⇒ 不给结论"
		return c, true
	}
	// 插入点 = H1 之后第一段空行的后面（与同目录既有件的形态逐字相同）
	at := h1 + 1
	for at < len(lines) && strings.TrimSpace(lines[at]) == "" {
		at++
	}
	date, src, derr := docMetaDate(root, rel, lines)
	if derr != nil {
		c.Reason = derr.Error()
		return c, true
	}
	c.Date, c.DateSource, c.InsertAt = date, src, at
	ins := []string{"> " + date + docAnnotationTail, ""}
	after := append([]string{}, lines[:at]...)
	after = append(after, ins...)
	after = append(after, lines[at:]...)
	c.After = []byte(strings.Join(after, "\n"))
	return c, true
}

// docMetaDate —— 日期（**只有两个既有来源**，都不新造）：文件名 `-YYYYMMDD` > 该件最后一次提交日。
func docMetaDate(root, rel string, lines []string) (string, string, error) {
	if m := docNameDateRE.FindStringSubmatch(filepath.Base(rel)); m != nil {
		return m[1] + "-" + m[2] + "-" + m[3], "文件名 `-YYYYMMDD`", nil
	}
	// 该件最后一次提交日（`git log -1 --format=%cs`）—— 只读，且**不许猜**：取不到就不给结论。
	cmd := exec.Command("git", "-C", root, "log", "-1", "--format=%cs", "--", rel)
	out, err := cmd.Output()
	d := strings.TrimSpace(string(out))
	if err != nil || d == "" {
		return "", "", fmt.Errorf("文件名无日期、且取不到该件的最后一次提交日（git log -1 --format=%%cs）⇒ 不给结论")
	}
	return d, "该件最后一次提交日（git log -1 --format=%cs）", nil
}

// docMetaAuditPath —— 审计落点（本机状态目录 · 不入仓）。
func docMetaAuditPath() string {
	return filepath.Join(stateDirOf(), "doc_meta_audit.jsonl")
}

// appendDocMetaAudit —— 追加一行审计（一行一事件；**失败即拒**：调用方据此不改件）。
func appendDocMetaAudit(path string, line docMetaAuditLine) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(line)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
