// family_script_inventory.go —— `A3`（`G-13`/`Q-002`）取「b」案：**仓外台账的唯一写命令**。
//
// 已拍口径（逐字 · 以设计稿为准）：`设计-仓外台账写面-A3-b案-v1.0-20260923.md`（`Zerg-内部文档/项目文档/v2.5.11/`）
//
//	· 「乙改甲……取「**b**」」= 台账**留仓外** ✗（**不**搬进主仓 `testdata/` ✗ · **不**改测试读 fixture ✗）
//	  + 给它一条**唯一写命令**且**写面进审计** ✓。
//	· 命令形状 = `zerg script inventory sync [--docs-root <Zerg-内部文档 根>] [--dry-run | --yes] [--json <字段>]`
//	  （**3 段** · 现读 `zerg help` 第 1 行「深度 ≤ 3 层」⇒ 合法 ✓）；退码三态照 `repo commit` 的同一张表。
//
// 本件与本族只读面的**分家**（设计稿 §三 · ★ 2026-09-24 缺口 `Q-160` 后仍成立，但**分家的东西变了**）：
// 只读面（`family_h.go cmdScriptLs` = `zerg script ls`）**仍不加任何写旗标** —— 只读面加一枚写旗标
// 就变成第二条写路径（这条规矩照旧 ✓）。但**收件口径不再分家**：`cmdScriptLs` 现调本件的
// `scriptInvScan` 取件（`Q-160` 前它自带 `scanRoots`：只收 `.sh`/`.py`、只下钻一层 ⇒ 报 136 而
// 判据件/台账同口径是 139 ⇒ **同数不同集**）。本命令**只**写那一份台账，**不碰**任何别的件。
//
// 判据面（设计稿 §五 9 条 · 全部可机检）**逐条落在本件里**：
//
//	1 三态齐（`--dry-run`=0 / 缺 `--yes`=2 / 带 `--yes`=0）·
//	2 干跑零副作用（目标件 sha256 与审计行数都不变）·
//	3 真写「变」+ 读回（写后 sha256 变 · 审计尾行的 `inventory_after_sha256` == 现算值）·
//	5 差异逐条给行不给数（`rows_added` / `rows_removed` 必配逐件点名）·
//	6 声明行必在且重算（三个数 == 现跑重算值）·
//	7 唯一写路径（静态计数见 §五 判据 7 的 grep）·
//	8 落点白名单 + 读不到不给结论（不许顺手新建一份）·
//	9 原子写 + 回滚可证（写前备份落状态目录 · 任一复查不过 ⇒ 逐字节写回）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Mr2109/zerg-swarm/core/internal/statepath"
)

// scriptInvLedgerRel —— **落点白名单**（常量：命令内只允许这一个件名）。
// `--docs-root` 只换**仓根**，**不许换件名**（设计稿 §二「形状五条不许」第 2 条）。
// 路径**不动** —— 路径一动，消费者测试 `script_inventory_test.go:127` 当场红。
const scriptInvLedgerRel = "项目文档/v2.5.10/清单-脚本现状-20260920.tsv"

// scriptInvDeclPrefix —— 声明行的前缀（消费者 `parseInventory` 逐字认这一个词）。
const scriptInvDeclPrefix = "现跑合计"

// scriptInvExcludeDirs —— 排除面（**与消费者测试 `scriptInvExclude` 同口径** · 13 项）。
// ★ 设计稿 §三 末点名的「三处各写一套 = 下一个 `Q-036`」：本件把口径**照抄一份**并在 §八 登记为欠账
// （提成共享函数要动**消费者测试件**，那是另一批的落点 ⇒ 本批不许顺手改它 ✗）。
// ★ 2026-09-24（`Q-160`）：`family_h.go` 那份（`scanRoots`）**已退位** ⇒ 扫描器**只剩本件这一份**
// （旁证面仍有一份在消费者测试件里，那份的合并照旧留给上面那条欠账 ✗ 本批不动）。
var scriptInvExcludeDirs = map[string]bool{
	"target": true, "node_modules": true, "dist": true, "bin": true, "vendor": true, "data": true,
	".git": true, ".venv": true, "venv": true, ".build": true, "zerg-wt": true, "_history": true,
	"__pycache__": true,
}

// scriptInvDenyWriteFlags —— 「**第二条写路径**」旗标（设计稿 §二「形状五条不许」第 1 条）：
// 这三枚都是**解析器认识**的旗标（`--set` 收进 `setPairs` · `--out`/`--file` 收进 `kv`）
// ⇒ 必须**在本命令里**显式拒执；`--del`/`--append` 一类不在名字表里 ⇒ dispatch 已按「未知旗标 2」判掉。
func scriptInvDenyWriteFlags(inv *invocation) []string {
	out := []string{}
	if len(inv.setPairs) > 0 {
		out = append(out, "--set")
	}
	for _, f := range []string{"--out", "--file"} {
		if _, ok := inv.kv[f]; ok {
			out = append(out, f)
		}
	}
	return out
}

// scriptInvRow —— 台账的一行（5 格 · 与消费者 `parseInventory` 的列面逐字同）。
type scriptInvRow struct {
	Path  string
	Kind  string
	Help  string
	JSON  string
	Lines int
}

// scriptInvKindOK —— 第 2 列的类型闭集（消费者逐字同：`sh` / `py` / `无后缀`）。
func scriptInvKindOK(k string) bool { return k == "sh" || k == "py" || k == "无后缀" }

func scriptInvYN(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// scriptInvCountLines —— 行数（末行无换行也算一行；与消费者「行数」列同一算法）。
func scriptInvCountLines(s string) int {
	n := strings.Count(s, "\n")
	if len(s) > 0 && !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// scriptInvScan —— 现跑扫 `scripts/` 面：`.sh` / `.py` / 无后缀且首行 `#!` 的可执行件。
// 三列（类型 / 有 --help / 有 --json）**从件自己的正文重算**（不抄台账里的旧值 —— 那是回潮的口子）。
func scriptInvScan(root string) ([]scriptInvRow, error) {
	base := filepath.Join(root, "scripts")
	rows := []scriptInvRow{}
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if scriptInvExcludeDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		kind := ""
		switch {
		case strings.HasSuffix(name, ".sh"):
			kind = "sh"
		case strings.HasSuffix(name, ".py"):
			kind = "py"
		}
		body, rerr := os.ReadFile(p)
		if kind == "" {
			// 无后缀：只有「可执行 + 首行 #!」才算脚本（与门禁的语法步同口径）
			st, serr := d.Info()
			if serr != nil || st.Mode()&0o111 == 0 {
				return nil
			}
			if rerr != nil || !strings.HasPrefix(string(body), "#!") {
				return nil
			}
			kind = "无后缀"
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		text := string(body)
		rows = append(rows, scriptInvRow{
			Path:  rel,
			Kind:  kind,
			Help:  scriptInvYN(strings.Contains(text, "--help")),
			JSON:  scriptInvYN(strings.Contains(text, "--json")),
			Lines: scriptInvCountLines(text),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	return rows, nil
}

// scriptInvParseLedger —— 解台账：**逐字照消费者 `parseInventory` 的跳行规则**
// （`★` / 两个空格 / `口径` / `「` 开头的行不算数据；单格且含「现跑合计」的行是声明行；
// 5 格且第 2 列在闭集里的行才是数据行 —— 表头那一行按名字跳过）。
func scriptInvParseLedger(text string) (rows map[string]scriptInvRow, declIdx int, declLine string) {
	rows = map[string]scriptInvRow{}
	declIdx = -1
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "★") || strings.HasPrefix(line, "  ") ||
			strings.HasPrefix(line, "口径") || strings.HasPrefix(line, "「") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) == 1 && strings.Contains(line, scriptInvDeclPrefix) {
			declIdx = i
			declLine = line
			continue
		}
		if len(f) != 5 || !scriptInvKindOK(f[1]) || f[0] == "脚本" {
			continue
		}
		n, _ := strconv.Atoi(strings.TrimSpace(f[4]))
		rows[f[0]] = scriptInvRow{Path: f[0], Kind: f[1], Help: f[2], JSON: f[3], Lines: n}
	}
	return rows, declIdx, declLine
}

// scriptInvDeclLineOf —— 声明行（**三个数**从现跑行面重算 · 设计稿 §五 判据 6）。
func scriptInvDeclLineOf(live []scriptInvRow) string {
	h, j := 0, 0
	for _, r := range live {
		if r.Help == "yes" {
			h++
		}
		if r.JSON == "yes" {
			j++
		}
	}
	return fmt.Sprintf("%s：件数 %d · 有 --help %d · 有 --json %d", scriptInvDeclPrefix, len(live), h, j)
}

func scriptInvRowLine(r scriptInvRow) string {
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%d", r.Path, r.Kind, r.Help, r.JSON, r.Lines)
}

// scriptInvPlan —— 一次真跑/干跑的**全部中间物**（干跑与真写读的是同一份 ⇒ 两档不许各算一遍）。
type scriptInvPlan struct {
	Target     string
	Live       []scriptInvRow
	Added      []string
	Removed    []string
	Unchanged  int
	OldDecl    string
	NewDecl    string
	BeforeSHA  string
	AfterSHA   string
	OldBody    []byte
	NewBody    []byte
	NewLines   []string
	AuditPath  string
	BackupPath string
}

// scriptInvBuildPlan —— 解析落点 → 现跑 → 差异 → 新件字节。任一步读不到 ⇒ 返回 error（调用方判 rc=2）。
func scriptInvBuildPlan(root, target string) (*scriptInvPlan, error) {
	old, err := os.ReadFile(target)
	if err != nil {
		return nil, fmt.Errorf("落点读不到（%s）：%v", target, err)
	}
	live, err := scriptInvScan(root)
	if err != nil {
		return nil, fmt.Errorf("扫 `scripts/` 面失败：%v", err)
	}
	known, declIdx, declLine := scriptInvParseLedger(string(old))
	if declIdx < 0 {
		return nil, fmt.Errorf("台账里没有「%s：件数 N · 有 --help N · 有 --json N」那一行 ⇒ "+
			"本命令只**重算那一行的三个数**，不新造口径（缺它 ⇒ 消费者当场报「没有声明的数可比」）", scriptInvDeclPrefix)
	}
	liveIdx := map[string]scriptInvRow{}
	for _, r := range live {
		liveIdx[r.Path] = r
	}
	p := &scriptInvPlan{Target: target, Live: live, OldDecl: declLine, NewDecl: scriptInvDeclLineOf(live)}
	for _, r := range live {
		if _, ok := known[r.Path]; ok {
			p.Unchanged++
		} else {
			p.Added = append(p.Added, r.Path)
		}
	}
	for path := range known {
		if _, ok := liveIdx[path]; !ok {
			p.Removed = append(p.Removed, path)
		}
	}
	sort.Strings(p.Added)
	sort.Strings(p.Removed)
	// 重建件面：**非数据行逐字节原样保留**（头部 4 行口径行 + 别的说明行）·
	// 存活的数据行**原地**换成现跑值（不重排）· 消失的行**去掉** · 声明行**只换三个数** · 新件按名追加在尾。
	out := []string{}
	orig := strings.Split(strings.TrimRight(string(old), "\n"), "\n")
	for i, line := range orig {
		if i == declIdx {
			out = append(out, p.NewDecl)
			continue
		}
		l := strings.TrimRight(line, "\r")
		f := strings.Split(l, "\t")
		if len(f) == 5 && scriptInvKindOK(f[1]) && f[0] != "脚本" {
			r, ok := liveIdx[f[0]]
			if !ok {
				continue // 消失的行：去掉（差异已逐条登记）
			}
			out = append(out, scriptInvRowLine(r))
			continue
		}
		out = append(out, line)
	}
	for _, path := range p.Added {
		out = append(out, scriptInvRowLine(liveIdx[path]))
	}
	p.NewLines = out
	p.NewBody = []byte(strings.Join(out, "\n") + "\n")
	p.OldBody = old
	p.BeforeSHA = scriptInvSHA(old)
	p.AfterSHA = scriptInvSHA(p.NewBody)
	p.AuditPath = editAuditPath()
	p.BackupPath = filepath.Join(statepath.Dir(), "script-inventory-backup.tsv")
	return p, nil
}

func scriptInvSHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func scriptInvSHAOfFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return scriptInvSHA(b)
}

// scriptInvAuditLine —— 审计行（**落同一件** `<状态目录>/edit_audit.jsonl` + **自己的事件名 + 自己的字段名**）。
// 字段名选型照 `approve_signed` 判据② 的同一纪律：**不复用** legacy 那六格（`before_sha256` 一类）。
type scriptInvAuditLine struct {
	At       string `json:"at"`
	Event    string `json:"event"`
	Face     string `json:"face"`
	Target   string `json:"target"`
	Before   string `json:"inventory_before_sha256"`
	After    string `json:"inventory_after_sha256"`
	Live     int    `json:"rows_live"`
	Added    int    `json:"rows_added"`
	Removed  int    `json:"rows_removed"`
	DeclLine string `json:"declared_line"`
	By       string `json:"by"`
	Confirm  string `json:"confirm"`
}

func scriptInvAuditFilled(l scriptInvAuditLine) error {
	miss := []string{}
	if strings.TrimSpace(l.At) == "" {
		miss = append(miss, "at")
	}
	if strings.TrimSpace(l.Event) == "" {
		miss = append(miss, "event")
	}
	if strings.TrimSpace(l.Face) == "" {
		miss = append(miss, "face")
	}
	if strings.TrimSpace(l.Target) == "" {
		miss = append(miss, "target")
	}
	if strings.TrimSpace(l.Before) == "" {
		miss = append(miss, "inventory_before_sha256")
	}
	if strings.TrimSpace(l.After) == "" {
		miss = append(miss, "inventory_after_sha256")
	}
	if len(miss) > 0 {
		return fmt.Errorf("审计行缺必需格：%s ⇒ 这一行不许写（设计稿 §四 纪律②）", strings.Join(miss, " / "))
	}
	return nil
}

// scriptInvAppendAudit —— 追加只写（`O_APPEND` · 历史行逐字节不变 · **失败即拒**）。
func scriptInvAppendAudit(path string, l scriptInvAuditLine) error {
	if err := scriptInvAuditFilled(l); err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("审计落点解析不出来（HOME / ZERG_STATE_DIR 都取不到）")
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

// scriptInvWriteAtomic —— 写 = 临时件 + `fsync` + 原子 `rename`（照 `dev edit` 的「写前自检 / 写后读回」纪律）。
func scriptInvWriteAtomic(path string, body []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".script-inventory-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	st, err := os.Stat(path)
	if err == nil {
		if err := os.Chmod(tmpName, st.Mode().Perm()); err != nil {
			os.Remove(tmpName)
			return err
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// scriptInvDocsRoot —— 文档仓根：`--docs-root` > `ZERG_DOCS` > `<主仓上一级>/Zerg-内部文档`
// （与消费者测试 `decision_records_test.go:183 zergDocsRoot` 的同一套三级优先级）。
func scriptInvDocsRoot(inv *invocation, root string) (string, string) {
	if v := strings.TrimSpace(inv.flagVal("--docs-root")); v != "" {
		return v, "--docs-root"
	}
	if v := strings.TrimSpace(os.Getenv("ZERG_DOCS")); v != "" {
		return v, "ZERG_DOCS"
	}
	if root != "" {
		return filepath.Join(filepath.Dir(root), "Zerg-内部文档"), "主仓上一级/Zerg-内部文档"
	}
	return "", "取不到"
}

// scriptInvPrintPlan —— 计划件（干跑与「缺 `--yes`」两档**打印同一份**）。
// 差异**逐条点名**（设计稿 §五 判据 5：只给数字 ⇒ 不许出这一行）。
func scriptInvPrintPlan(stdout, stderr io.Writer, p *scriptInvPlan, docsRoot, docsSrc string, realWrite bool) {
	fmt.Fprintln(stdout, "计划件（"+map[bool]string{true: "--yes 前的一次预览", false: "--dry-run"}[realWrite]+
		" · 零副作用 —— 未写件、未写审计）")
	fmt.Fprintf(stdout, "  落点     : %s\n", p.Target)
	fmt.Fprintf(stdout, "  文档仓根 : %s（%s）\n", docsRoot, docsSrc)
	fmt.Fprintf(stdout, "  现跑     : 件数 %d\n", len(p.Live))
	fmt.Fprintf(stdout, "  差异     : 新增 %d 件 / 消失 %d 件 / 未变 %d 件\n", len(p.Added), len(p.Removed), p.Unchanged)
	for _, a := range p.Added {
		fmt.Fprintf(stdout, "    + %s\n", a)
	}
	for _, r := range p.Removed {
		fmt.Fprintf(stdout, "    - %s\n", r)
	}
	fmt.Fprintf(stdout, "  声明行   : %s\n", p.NewDecl)
	if p.OldDecl != p.NewDecl {
		fmt.Fprintf(stdout, "  声明行旧 : %s\n", p.OldDecl)
	}
	fmt.Fprintf(stdout, "  写前 sha : %s\n", p.BeforeSHA)
	fmt.Fprintf(stdout, "  写后 sha : %s（按**将要写的内容**现算）\n", p.AfterSHA)
	fmt.Fprintf(stdout, "  备份落点 : %s\n", p.BackupPath)
	fmt.Fprintf(stdout, "  审计落点 : %s\n", p.AuditPath)
	fmt.Fprintln(stderr, "（"+map[bool]string{true: "--yes 前的一次预览", false: "--dry-run"}[realWrite]+
		"：只出计划件 · 零副作用）")
}

// scriptInvEmitJSON —— 机器面（六键包封由 `emitEnvelopeWith` 出；`items` = 一行十格）。
func scriptInvEmitJSON(inv *invocation, stdout, stderr io.Writer, p *scriptInvPlan, result string) int {
	if !requireFields(inv, stderr) {
		// K2：给了 --json 不给字段 ⇒ 1 + stdout 0 字节。
		// ★ 照实登记一条**框架级不一致**（本批现读撞到 · 不擅自抹平）：危险档命令的退码由框架兜底成
		//   机器可读包封（走 stdout），而包封的 `error.exit_code` 是从 `kind` **映射**出来的 ——
		//   K2 这一格的真退码是 **1**，若把 kind 报成 `usage` 则包封会写 `exit_code: 2`（与 rc 打架）。
		//   ⇒ 本命令**不**在这一格报 kind，让框架打它自己的兜底包封（`exit_code: 1` 与 rc 一致 ·
		//   包封里 `kind` 一句「命令未报出 kind，按退码兜底」照实可见）。缺口登记见回执「未做/未核」。
		return exitFail
	}
	row := map[string]string{
		"result":                  result,
		"target":                  p.Target,
		"rows_live":               strconv.Itoa(len(p.Live)),
		"rows_added":              strconv.Itoa(len(p.Added)),
		"rows_removed":            strconv.Itoa(len(p.Removed)),
		"rows_unchanged":          strconv.Itoa(p.Unchanged),
		"declared_line":           p.NewDecl,
		"inventory_before_sha256": p.BeforeSHA,
		"inventory_after_sha256":  p.AfterSHA,
		"audit_path":              p.AuditPath,
		"rows_added_named":        strings.Join(p.Added, ","),
		"rows_removed_named":      strings.Join(p.Removed, ","),
	}
	return selectJSONList(stdout, stderr, inv, inv.path, inv.fields, []map[string]string{row})
}

// cmdScriptInventorySync —— `zerg script inventory sync`（`A3` 的「b」案唯一写面）。
func cmdScriptInventorySync(inv *invocation, stdout, stderr io.Writer) int {
	if denied := scriptInvDenyWriteFlags(inv); len(denied) > 0 {
		inv.setErr("usage", "second_write_path", "本命令**不接受**逐条改旗标（一条命令写 = 没有第二条写路径）")
		fmt.Fprintf(stderr, "%s: `script inventory sync` **不接受** %s —— 它是「逐条改」那类旗标"+
			"（设计稿 §二「形状五条不许」第 1 条：出现即拒执）\n", progName, strings.Join(denied, " / "))
		fmt.Fprintf(stderr, "要改行面只能改 `scripts/` 下的真件再跑本命令（命令只**重算**行面与两个 yes/no 列）\n")
		return exitUsage
	}
	if len(inv.args) > 0 {
		inv.setErr("usage", "no_positional", "本命令不吃位置参数")
		fmt.Fprintf(stderr, "%s: `script inventory sync` 不吃位置参数（收到 %q）\n", progName, inv.args[0])
		return exitUsage
	}
	root := repoRoot()
	if root == "" {
		inv.setErr("blocked", "repo_root_absent", "解析不到仓根")
		fmt.Fprintf(stderr, "%s: 解析不到仓根 ⇒ 扫不了 `scripts/` 面（不给结论 · 退码 8）\n", progName)
		return exitBlocked
	}
	docsRoot, docsSrc := scriptInvDocsRoot(inv, root)
	if docsRoot == "" {
		inv.setErr("blocked", "docs_root_absent", "文档仓根取不到")
		fmt.Fprintf(stderr, "%s: 文档仓根取不到（`--docs-root` / `ZERG_DOCS` / 主仓上一级 `Zerg-内部文档` 三条都空）⇒ 不给结论\n", progName)
		return exitBlocked
	}
	target := filepath.Join(docsRoot, scriptInvLedgerRel)
	p, err := scriptInvBuildPlan(root, target)
	if err != nil {
		// 判据 8：落点不在白名单 / 解析不到 ⇒ **不给结论**（不许「顺手新建一份」）
		inv.setErr("usage", "ledger_unreadable", err.Error())
		fmt.Fprintf(stderr, "%s: %v\n", progName, err)
		fmt.Fprintf(stderr, "落点白名单只有一件：`<文档仓根>/%s`；**不许**顺手新建第二份台账（新建 = 两套口径）\n", scriptInvLedgerRel)
		return exitUsage
	}
	if len(p.Live) == 0 {
		// 判据/负控 ⑧：空转 = 假覆盖（照门③/门⑤ 的同一口径）
		inv.setErr("blocked", "scan_empty", "`scripts/` 面 0 件 ⇒ 不给结论")
		fmt.Fprintf(stderr, "%s: 扫 `scripts/` 面 0 件 ⇒ 空转不给结论（退码 2）—— 「扫不到」不是「台账是空的」\n", progName)
		return exitUsage
	}

	if inv.dryRun {
		// ★ 机器面分家：给了 `--json` ⇒ 计划件走 **stderr**、stdout 只留六键包封
		//   （stdout 是结果面 —— 混进人面文字会让消费侧解析不到包封）。
		planOut := stdout
		if inv.jsonGiven {
			planOut = stderr
		}
		scriptInvPrintPlan(planOut, stderr, p, docsRoot, docsSrc, false)
		if inv.jsonGiven {
			return scriptInvEmitJSON(inv, stdout, stderr, p, "dry_run")
		}
		return exitOK
	}
	if !inv.yes {
		planOut := stdout
		if inv.jsonGiven {
			planOut = stderr
		}
		scriptInvPrintPlan(planOut, stderr, p, docsRoot, docsSrc, true)
		if inv.jsonGiven {
			if rc := scriptInvEmitJSON(inv, stdout, stderr, p, "need_yes"); rc != exitOK {
				return rc
			}
		}
		inv.setErr("usage", "yes_required", "D2 档缺 --yes ⇒ 不执行")
		fmt.Fprintf(stderr, "%s: `script inventory sync` 是 D2 档 —— **缺 --yes ⇒ 不执行**（照 `repo commit` 的 fail-closed · 从不提问）\n", progName)
		fmt.Fprintf(stderr, "先看计划件：%s script inventory sync --dry-run\n", progName)
		return exitUsage
	}

	// ---- 真写：审计先落盘 → 备份 → 原子写 → 写后读回对拍 → 复查不过逐字节写回 ----
	if p.BeforeSHA == p.AfterSHA {
		fmt.Fprintf(stdout, "幂等命中：现跑行面与台账逐字节相同（sha %s）⇒ 不写件、不落审计\n", p.BeforeSHA)
		if inv.jsonGiven {
			return scriptInvEmitJSON(inv, stdout, stderr, p, "idempotent")
		}
		return exitOK
	}
	audit := scriptInvAuditLine{
		At: time.Now().Format(time.RFC3339), Event: "inventory_written", Face: "script inventory sync",
		Target: p.Target, Before: p.BeforeSHA, After: p.AfterSHA,
		Live: len(p.Live), Added: len(p.Added), Removed: len(p.Removed),
		DeclLine: p.NewDecl, By: orDash(inv.flagVal("--by")), Confirm: "--yes",
	}
	if err := scriptInvAppendAudit(p.AuditPath, audit); err != nil {
		// 审计先落盘那一侧：写不进审计 ⇒ **不写件**
		inv.setErr("blocked", "audit_unwritable", "审计写不进 ⇒ 不写件")
		fmt.Fprintf(stderr, "%s: 审计写不进（%s）：%v ⇒ **不写件**（`dev edit` 那一侧：没有审计的写 = 之后判不了「谁改的」）\n",
			progName, p.AuditPath, err)
		return exitBlocked
	}
	if err := scriptInvWriteBackup(p.BackupPath, p.OldBody); err != nil {
		inv.setErr("blocked", "backup_unwritable", "写前备份落不下来 ⇒ 不写件")
		fmt.Fprintf(stderr, "%s: 写前备份落不下来（%s）：%v ⇒ 不写件（回滚要有可证的落点）\n", progName, p.BackupPath, err)
		return exitBlocked
	}
	if err := scriptInvWriteAtomic(p.Target, p.NewBody); err != nil {
		inv.setErr("failed", "write_failed", "原子写失败")
		fmt.Fprintf(stderr, "%s: 原子写失败：%v ⇒ 已按写前备份逐字节写回\n", progName, err)
		_ = scriptInvRestore(p)
		return exitFail
	}
	// 写后读回对拍
	back := scriptInvSHAOfFile(p.Target)
	if back != p.AfterSHA {
		fmt.Fprintf(stderr, "%s: 写后读回对拍不等（现算 %s ≠ 期望 %s）⇒ 逐字节写回\n", progName, back, p.AfterSHA)
		_ = scriptInvRestore(p)
		inv.setErr("failed", "readback_mismatch", "写后读回对拍不等 ⇒ 已回滚")
		return exitFail
	}
	fmt.Fprintf(stdout, "已写台账 %s · 件数 %d（新增 %d / 消失 %d / 未变 %d）· sha %s → %s\n",
		p.Target, len(p.Live), len(p.Added), len(p.Removed), p.Unchanged, scriptInvShort(p.BeforeSHA), scriptInvShort(p.AfterSHA))
	fmt.Fprintf(stdout, "审计落点 %s（事件 inventory_written · 追加只写）· 备份 %s\n", p.AuditPath, p.BackupPath)
	for _, a := range p.Added {
		fmt.Fprintf(stdout, "  + %s\n", a)
	}
	for _, r := range p.Removed {
		fmt.Fprintf(stdout, "  - %s\n", r)
	}
	if inv.jsonGiven {
		return scriptInvEmitJSON(inv, stdout, stderr, p, "written")
	}
	return exitOK
}

func scriptInvShort(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func scriptInvWriteBackup(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o600)
}

// scriptInvRestore —— 逐字节写回（回滚可证：写回后 sha256 == 写前值）。
func scriptInvRestore(p *scriptInvPlan) error {
	if err := scriptInvWriteAtomic(p.Target, p.OldBody); err != nil {
		return err
	}
	if got := scriptInvSHAOfFile(p.Target); got != p.BeforeSHA {
		return fmt.Errorf("写回后 sha256 %s ≠ 写前 %s", got, p.BeforeSHA)
	}
	return nil
}
