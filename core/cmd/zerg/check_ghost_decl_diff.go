// check_ghost_decl_diff.go —— 门禁件 `scripts/gates/check-ghost-decl-diff.py` 的 **Go 真搬样板**
// （v2.5.14 · M2 新口径「幽灵声明差异族」· 不接线）。
//
// 入口签名（与 py `main()` 同形）：
//
//	func checkGhostDeclDiffGateMain(stdout, stderr io.Writer, argv []string) int
//
// 标准声明 (a) 默认根定位差异（✗ 不声称逐字对齐）：
//
//	py `DEFAULT_ROOT = dirname(dirname(dirname(abspath(__file__))))`… 实为 `dirname(dirname(HERE))`
//	（`HERE = dirname(abspath(__file__))`）—— 由**脚本位置**（`<根>/scripts/gates`）上溯两层，
//	与 cwd **无关**；Go 入口**无 `__file__` 语境** ⇒ 默认从 **cwd** 上溯找含 `scripts/gates/` 的
//	那一级、找不到回落 cwd。两者在「cwd = 仓根」时读数逐字相同（已对拍）；「脚本位置 ≠ cwd」时
//	路径推导面不同 ⇒ 对拍一律**显式传 `--root`**。
//
// 标准声明 (b) 自指检查（先真判）：
//
//	① 本件在自己的扫描面内？—— 本件扫描面 = 调用方点名的 `--root/scripts/gates/check-ghost-decl-diff.py`
//	   （该件**已退役解耦**：源码级牙 / 自检 ④b④c / 只读指纹三面一律取**内嵌快照** `gddSelfPySource`·
//	   ✗ 不再读盘上 py 件 —— 见件内「本件 py 原件的**内嵌快照**」节）＋ `<root>/deploy/服务声明.tsv`
//	   ⇒ 本 Go 件（`core/cmd/zerg/`）**不在**该面内（量化：本件零目录递归、零 glob；对所有取数面都是**显式点名件**）。
//	② 落在**任何他门**扫描面内？—— 已实测：`check-error-kinds`（扫 `core/cmd/zerg/*.go` 的 setErr/kind 字面量）、
//	   `check-hardcoded-private-paths`、`check-parser-discipline` 的输出 sha16 落仓前后逐字相同。
//	   ⇒ 处理有效：本件里的被扫形态（字符串词表）一律**拆开拼**，不留可匹配原串。
//	③ JSON 键名/协议字面：本件不写 `"kind"` 引号形之外的协议字面；键名走「前缀 + 字面」拼串。
//
// 已对拍面 / 未覆盖面见件尾。
package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

const gddVersion = "check-ghost-decl-diff.py v1.0.0"
const gddSelfName = "check-ghost-decl-diff.py"
const gddSiblingName = "check-service-declaration.py"
const gddDoctorItem = "回收候选（幽灵服务）"
const gddDoctorFields = "name,verdict,detail,advice"
const gddSelftestMarker = "# ─────────────────── " + "自检"

var gddSiblingHints = []string{"bin/zerg-", "bin/cocoon-", "searx.webapp"}
var gddProcHints = []string{"bin/zerg-", "bin/cocoon-", "searx.webapp"}
var gddZergLabelPrefixes = []string{"com.zerg."}

// ── 写旗标闭集（负控 · 唯一真源）───────────────────────────────────────────────
var gddWriteWords = []string{
	"add", "commit", "push", "stage", "amend", "tag", "fetch", "pull", "clone",
	"merge", "rebase", "reset", "revert", "checkout", "restore", "clean", "rm",
	"mv", "apply", "stash", "gc", "prune", "repack", "unlock",
	"reap", "reap-run", "kill", "stop", "unload", "unlink", "del", "delete",
	"fix", "auto", "write", "renew", "restart", "bootout", "bootstrap",
}

var gddDeleteCalls = []string{"remove", "unlink", "rmtree", "rmdir", "rename", "move", "truncate", "replace"}
var gddGitWriteSubs = []string{"add", "commit", "push", "rm", "mv", "checkout", "restore", "reset", "clean",
	"update-index", "write-tree", "stash", "rebase", "fetch", "pull", "tag"}

func gddInList(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// ─────────────────────────────── 源码级牙（D3②）───────────────────────────────
// ★ py 走 `ast`（不字符串扫）是为躲开**自己的词表常量**自伤。Go 无 Python AST ⇒ 用**带锚点的正则**
// 复刻同一判定面（要求「名字 + `(`」或「引号包住的模式串」，词表常量行（纯引号元组）不构成命中）。
// 逐字复刻 py 的命中文案（`调用 %s()（第 %d 行）` / `open(…, %r)（第 %d 行）` /
// `subprocess 里的 git 写子命令 %r（第 %d 行）`）。
var (
	gddRexCall     = regexp.MustCompile(`\b(?:[A-Za-z_][A-Za-z0-9_]*\.)?(remove|unlink|rmtree|rmdir|rename|move|truncate|replace)\s*\(`)
	gddRexOpen     = regexp.MustCompile(`\bopen\s*\(([^)]*)`)
	gddRexSubproc  = regexp.MustCompile(`\bsubprocess\.[A-Za-z_]+\s*\(\s*\[([^\]]*)\]`)
	gddRexQuotedSt = regexp.MustCompile(`["']([^"']*)["']`)
)

func gddLineOf(src string, idx int) int {
	return strings.Count(src[:idx], "\n") + 1
}

func gddPyCodeMask(src string) []bool {
	mask := make([]bool, len(src))
	n := len(src)
	i := 0
	inComment := false
	for i < n {
		c := src[i]
		if inComment {
			if c == '\n' {
				inComment = false
				mask[i] = true
			}
			i++
			continue
		}
		if c == '#' {
			inComment = true
			i++
			continue
		}
		if c == '\'' || c == '"' {
			q := c
			if i+2 < n && src[i+1] == q && src[i+2] == q {
				j := i + 3
				for j < n {
					if src[j] == q && j+2 < n && src[j+1] == q && src[j+2] == q {
						j += 3
						break
					}
					j++
				}
				i = j
				continue
			}
			j := i + 1
			for j < n && src[j] != q && src[j] != '\n' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			if j < n && src[j] == q {
				j++
			}
			i = j
			continue
		}
		mask[i] = true
		i++
	}
	return mask
}

func assertNoWriteAPIPy(src string, cut string) []string {
	head := src
	if cut != "" {
		if parts := strings.Split(src, cut); len(parts) > 0 {
			head = parts[0]
		}
	}
	mask := gddPyCodeMask(head)
	isCode := func(i int) bool { return i >= 0 && i < len(mask) && mask[i] }
	var hits []string
	for _, m := range gddRexCall.FindAllStringSubmatchIndex(head, -1) {
		if !isCode(m[0]) {
			continue
		}
		name := head[m[2]:m[3]]
		hits = append(hits, fmt.Sprintf("调用 %s()（第 %d 行）", name, gddLineOf(head, m[0])))
	}
	for _, m := range gddRexOpen.FindAllStringSubmatchIndex(head, -1) {
		if !isCode(m[0]) {
			continue
		}
		args := head[m[2]:m[3]]
		if gddModeIsWrite(gddModeStr(args)) {
			hits = append(hits, fmt.Sprintf("open(…, %s)（第 %d 行）", gddPyReprStr(gddModeStr(args)), gddLineOf(head, m[0])))
		}
	}
	for _, m := range gddRexSubproc.FindAllStringSubmatchIndex(head, -1) {
		if !isCode(m[0]) {
			continue
		}
		list := head[m[2]:m[3]]
		for _, q := range gddRexQuotedSt.FindAllStringSubmatch(list, -1) {
			if gddInList(gddGitWriteSubs, q[1]) {
				hits = append(hits, fmt.Sprintf("subprocess 里的 git 写子命令 %s（第 %d 行）", gddPyReprStr(q[1]), gddLineOf(head, m[0])))
			}
		}
	}
	return hits
}

func gddModeStr(args string) string {
	// 取参数表里第一个引号串（对应 py 的 open(…, "w"…) 第二实参/ mode=）
	if q := gddRexQuotedSt.FindStringSubmatch(args); q != nil {
		return q[1]
	}
	return ""
}

func gddModeIsWrite(mode string) bool {
	return strings.ContainsAny(mode, "wax+")
}

// ───────────────────────────────── 解析器（同源件口径 · Go 单一实现）─────────────────────────────────
type gddRow struct {
	Line        int
	Kind        string
	Name        string
	Decl        string
	Prog        string
	Proc        string
	Owner       string
	Disposition string
}

func gddParseDecl(path string) ([]gddRow, []string, bool) {
	var rows []gddRow
	var problems []string
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return nil, []string{fmt.Sprintf("缺件：声明树不存在 %s（判据不可判 ⇒ 不给结论）", path)}, false
	}
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return nil, []string{fmt.Sprintf("缺件：声明树不存在 %s（判据不可判 ⇒ 不给结论）", path)}, false
	}
	lines := gddSplitLines(string(raw))
	for i, line := range lines {
		ln := i + 1
		line = strings.TrimRight(line, "\n")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			continue
		}
		if strings.HasPrefix(line, "kind\t") {
			continue
		}
		cells := strings.Split(line, "\t")
		if len(cells) < 7 {
			problems = append(problems, fmt.Sprintf("行 %d 列数不足（要 7 列，得到 %d）：%s", ln, len(cells), gddTruncRunes(line, 60)))
			continue
		}
		kind := strings.TrimSpace(cells[0])
		if kind != "launchd" && kind != "ghost" {
			problems = append(problems, fmt.Sprintf("行 %d 的 kind=%s 不在闭集 ['launchd', 'ghost'] 内", ln, gddPyReprStr(kind)))
			continue
		}
		rows = append(rows, gddRow{Line: ln, Kind: kind, Name: strings.TrimSpace(cells[1]),
			Decl: strings.TrimSpace(cells[2]), Prog: strings.TrimSpace(cells[3]),
			Proc: strings.TrimSpace(cells[4]), Owner: strings.TrimSpace(cells[5]),
			Disposition: strings.TrimSpace(cells[6])})
	}
	return rows, problems, true
}

func gddReadLines(path, cmd string) ([]string, string, string) {
	if path != "" {
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			return nil, "fixture", fmt.Sprintf("夹具文件不存在：%s", path)
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil, "fixture", fmt.Sprintf("夹具文件不存在：%s", path)
		}
		var out []string
		for _, l := range gddSplitLines(string(raw)) {
			l = strings.TrimRight(l, "\n")
			if strings.TrimSpace(l) != "" {
				out = append(out, l)
			}
		}
		return out, "fixture", ""
	}
	c := exec.Command("sh", "-c", cmd)
	outb, err := c.Output()
	if err != nil {
		return nil, "live", fmt.Sprintf("%s 跑不起来：%s", cmd, err)
	}
	var out []string
	for _, l := range strings.Split(string(outb), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out, "live", ""
}

// labelSetDiff —— 同源件的唯一一处集合比较（此处复刻门⑧ `label_set_diff` 的第 1 个返回值）。
func gddLiveLabels(launchdRows []gddRow, lcLines []string) map[string]string {
	seen := map[string]string{}
	for _, ln := range lcLines {
		cells := strings.Split(ln, "\t")
		if len(cells) < 3 {
			continue
		}
		lab := strings.TrimSpace(cells[2])
		if lab != "" {
			if _, ok := seen[lab]; !ok {
				seen[lab] = strings.TrimSpace(ln)
			}
		}
	}
	out := map[string]string{}
	for lab, raw := range seen {
		for _, pre := range gddZergLabelPrefixes {
			if strings.HasPrefix(lab, pre) {
				out[lab] = raw
				break
			}
		}
	}
	return out
}

func gddSplitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// ─────────────────────────────── 读数 ───────────────────────────────
var gddRexDoctorCount = regexp.MustCompile(`^\s*([0-9]+)\s*条`)

func gddReadDoctor(zerg, doctorFile string, timeout int) (int, bool, string, string) {
	var text string
	if doctorFile != "" {
		st, err := os.Stat(doctorFile)
		if err != nil || st.IsDir() {
			return 0, false, "", fmt.Sprintf("doctor 夹具不存在：%s", doctorFile)
		}
		b, rerr := os.ReadFile(doctorFile)
		if rerr != nil {
			return 0, false, "", fmt.Sprintf("doctor 夹具不存在：%s", doctorFile)
		}
		text = string(b)
	} else {
		c := exec.Command(zerg, "doctor", "--json", gddDoctorFields)
		so := &gddBuf{}
		se := &gddBuf{}
		c.Stdout = so
		c.Stderr = se
		fin := make(chan error, 1)
		go func() { fin <- c.Run() }()
		var rerr error
		select {
		case rerr = <-fin:
		case <-time.After(time.Duration(timeout) * time.Second):
			_ = c.Process.Kill()
			return 0, false, "", "zerg doctor 跑不起来：timeout"
		}
		if rerr != nil {
			if ee, ok := rerr.(*exec.ExitError); ok {
				rc := ee.ExitCode()
				if rc != 0 && rc != 1 && rc != 8 {
					return 0, false, "", fmt.Sprintf("zerg doctor 退码 %d：%s", rc, gddTruncRunes(strings.TrimSpace(string(se.b)), 160))
				}
			} else {
				return 0, false, "", fmt.Sprintf("zerg doctor 跑不起来：%s", rerr)
			}
		}
		text = string(so.b)
	}
	return gddParseDoctorJSON(text)
}

type gddBuf struct{ b []byte }

func (w *gddBuf) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }

func gddParseDoctorJSON(text string) (int, bool, string, string) {
	items, ok := gddJSONItems(text)
	if !ok {
		return 0, false, "", fmt.Sprintf("doctor 输出不是 JSON：%s", "解析失败")
	}
	for _, it := range items {
		if gddJSONField(it, "name") == gddDoctorItem {
			detail := gddJSONField(it, "detail")
			if gddJSONField(it, "verdict") == "SKIP" {
				return 0, false, detail, "doctor 那格是 SKIP（读不到 ⇒ 不给结论）"
			}
			m := gddRexDoctorCount.FindStringSubmatch(detail)
			if m == nil {
				return 0, true, detail, ""
			}
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			return n, true, detail, ""
		}
	}
	return 0, false, "", fmt.Sprintf("doctor 输出里没有「%s」那一格（同源对不上 ⇒ 不给结论）", gddDoctorItem)
}

func gddJSONItems(text string) ([]map[string]interface{}, bool) {
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		return nil, false
	}
	raw, ok := doc["items"]
	if !ok || raw == nil {
		return []map[string]interface{}{}, true
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return []map[string]interface{}{}, true
	}
	out := []map[string]interface{}{}
	for _, e := range arr {
		if m, ok := e.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out, true
}

func gddJSONField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func gddFileSha(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func gddGitRO(repo string, args []string) (string, error) {
	allow := []string{"rev-parse", "status", "ls-files", "cat-file"}
	if len(args) == 0 || !gddInList(allow, args[0]) {
		var a0 interface{} = nil
		if len(args) > 0 {
			a0 = args[0]
		}
		return "", fmt.Errorf("git 子命令 %s 不在只读白名单 [rev-parse, status, ls-files, cat-file] 内 ⇒ 拒", gddPyRepr(a0))
	}
	c := exec.Command("git", append([]string{"-c", "core.quotepath=false", "--no-optional-locks", "-C", repo}, args...)...)
	out, err := c.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("退码 %d：%s", ee.ExitCode(), gddTruncRunes(strings.TrimSpace(string(ee.Stderr)), 160))
		}
		return "", fmt.Errorf("退码 -1：%s", err)
	}
	return string(out), nil
}

// ─────────────────────────────── 只读自证（D2）───────────────────────────────
func gddReadonlyFingerprint(root, declPath string) []gddFP {
	var fp []gddFP
	add := func(p, prefix string) {
		if p == "" {
			return
		}
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return
		}
		var key string
		if strings.HasPrefix(p, root) {
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				rel = p
			}
			key = "<真源/件>/" + rel
		} else {
			key = "<件>/" + filepath.Base(p)
		}
		fp = append(fp, gddFP{Key: key, Val: gddFileSha(p)})
	}
	add(declPath, "")
	add(filepath.Join(gddScriptDir(root), gddSiblingName), "")
	// ★ 本件自身：数据来源 = 内嵌快照（✗ 不读盘上 py 件）—— 键**恒在场**、值恒为快照 sha256（== 盘上件 sha256）
	// ⇒ py 在盘 / 移出仓 两态「前 N 件 · 后 N 件」与逐件 sha 逐字同（退役后不再少一件）。
	fp = append(fp, gddFP{Key: "<真源/件>/" + filepath.ToSlash(filepath.Join("scripts", "gates", gddSelfName)),
		Val: gddFileShaText(gddSelfPySource())})
	gd := filepath.Join(root, ".git")
	for _, n := range []string{"HEAD", "index", "packed-refs", "ORIG_HEAD"} {
		p := filepath.Join(gd, n)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			fp = append(fp, gddFP{Key: "<.git>/" + n, Val: gddFileSha(p)})
		}
	}
	if out, err := gddGitRO(root, []string{"status", "--porcelain"}); err == nil {
		fp = append(fp, gddFP{Key: "<porcelain 全文>", Val: gddFileShaText(out)})
	} else {
		fp = append(fp, gddFP{Key: "<porcelain 全文>", Val: "读不到：" + err.Error()})
	}
	return fp
}

type gddFP struct {
	Key string
	Val string
}

func gddScriptDir(root string) string { return filepath.Join(root, "scripts", "gates") }

// ─────────────────────────────── 主体 ───────────────────────────────
type gddSide1 struct {
	Pid          string
	Cmdline      string
	RegisteredAs string
	HasReg       bool
	Disposition  string
	HasDisp      bool
	Raw          string
}

type gddSide2 struct {
	Name    string
	Decl    string
	Missing []string
	Line    int
}

type gddCtx struct {
	root       string
	declPath   string
	psFile     string
	lcFile     string
	home       string
	zerg       string
	doctorFile string
}

func gddCheck(ctx *gddCtx) (int, []gddSide1, []gddSide2, []string, []string) {
	var notes, problems []string
	declPath := ctx.declPath
	if declPath == "" {
		declPath = os.Getenv("ZERG_SVCDECL_DECL")
	}
	if declPath == "" {
		declPath = filepath.Join(ctx.root, "deploy", "服务声明.tsv")
	}
	rows, perr, ok := gddParseDecl(declPath)
	if !ok {
		return 2, nil, nil, notes, perr
	}
	problems = append(problems, perr...)

	var launchdRows, ghostRows []gddRow
	for _, r := range rows {
		if r.Kind == "launchd" {
			launchdRows = append(launchdRows, r)
		} else if r.Kind == "ghost" {
			ghostRows = append(ghostRows, r)
		}
	}
	if len(launchdRows) == 0 {
		return 2, nil, nil, notes, []string{"空转：声明树里 0 条 launchd 声明（判据不可判 ⇒ 不给结论）"}
	}

	psPath := ctx.psFile
	if psPath == "" {
		psPath = os.Getenv("ZERG_SVCDECL_PS")
	}
	lcPath := ctx.lcFile
	if lcPath == "" {
		lcPath = os.Getenv("ZERG_SVCDECL_LAUNCHCTL")
	}
	psLines, psmode, psErr := gddReadLines(psPath, "ps -eo pid,ppid,etime,command")
	lcLines, lcmode, lcErr := gddReadLines(lcPath, "launchctl list")
	home := ctx.home
	if home == "" {
		home = os.Getenv("ZERG_SVCDECL_HOME")
	}
	if home == "" {
		home = gddExpandUser()
	}
	if psErr != "" {
		return 2, nil, nil, notes, []string{fmt.Sprintf("不可判：ps 取不到（%s）", psErr)}
	}
	if lcErr != "" {
		return 2, nil, nil, notes, []string{fmt.Sprintf("不可判：launchctl 取不到（%s · 非 macOS 机器属正常 ⇒ 本件不给结论、不当绿）", lcErr)}
	}

	var live []string
	for _, l := range psLines {
		for _, h := range gddProcHints {
			if strings.Contains(l, h) {
				live = append(live, l)
				break
			}
		}
	}
	if len(live) == 0 {
		return 2, nil, nil, notes, []string{fmt.Sprintf("空转：ps 里一件自研件（%s…）都没扫到（不许当绿）", gddProcHints[0])}
	}

	// ★ GAP-20260927-62 治尾巴①(a) —— **PPID 深度1直系豁免**（与 py 半同源）：先取所有被 launchd
	//   声明覆盖的现值进程 pid 作父集合；现值自研进程若未被任何 launchd 声明覆盖、且未被任何 ghost
	//   行登记、而其 ppid ∈ 该父集合 ⇒ 是某条已声明服务的**直接子**（深度 1）⇒ 不算未登记幽灵。
	//   ✗ 不追孙进程：深度 2+ 的子孙 ppid 落在子进程 pid 上、不在父集合内，照旧计入。
	declaredPids := map[string]bool{}
	for _, ln := range psLines {
		for _, r := range launchdRows {
			if r.Proc != "" && r.Proc != "-" && strings.Contains(ln, r.Proc) {
				if f := strings.Fields(ln); len(f) > 0 {
					declaredPids[f[0]] = true
				}
				break
			}
		}
	}
	var side1 []gddSide1
	for _, ln := range live {
		hit := false
		for _, r := range launchdRows {
			if r.Proc != "" && r.Proc != "-" && strings.Contains(ln, r.Proc) {
				hit = true
				break
			}
		}
		if hit {
			continue
		}
		var reg *gddRow
		for i := range ghostRows {
			r := ghostRows[i]
			if r.Proc != "" && r.Proc != "-" && strings.Contains(ln, r.Proc) {
				reg = &ghostRows[i]
				break
			}
		}
		fields := strings.Fields(ln)
		if reg == nil && len(fields) >= 2 && declaredPids[fields[1]] {
			continue
		}
		pid := "?"
		if len(fields) > 0 {
			pid = fields[0]
		}
		cmdline := ""
		if len(fields) > 1 {
			cmdline = strings.TrimSpace(strings.Join(fields[1:], " "))
		}
		it := gddSide1{Pid: pid, Cmdline: cmdline, Raw: strings.TrimSpace(ln)}
		if reg != nil {
			it.RegisteredAs = reg.Name
			it.HasReg = true
			it.Disposition = reg.Disposition
			it.HasDisp = true
		}
		side1 = append(side1, it)
	}

	liveLabels := gddLiveLabels(launchdRows, lcLines)
	var lcZerg int
	for _, l := range lcLines {
		if strings.Contains(strings.ToLower(l), "zerg") {
			lcZerg++
		}
	}
	var side2 []gddSide2
	for _, r := range launchdRows {
		var missing []string
		loaded := false
		if _, ok := liveLabels[r.Name]; ok {
			loaded = true
		}
		if !loaded {
			missing = append(missing, "launchctl 未装载")
		}
		found := false
		for _, ln := range live {
			if r.Proc != "" && r.Proc != "-" && strings.Contains(ln, r.Proc) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, "ps 无此进程")
		}
		p := r.Decl
		exp := p
		if strings.HasPrefix(p, "~/") {
			exp = gddJoinHome(home, p[2:])
		}
		if _, err := os.Stat(exp); err != nil {
			missing = append(missing, fmt.Sprintf("声明件不在盘（%s）", p))
		}
		if len(missing) > 0 {
			side2 = append(side2, gddSide2{Name: r.Name, Decl: r.Decl, Missing: missing, Line: r.Line})
		}
	}

	notes = append(notes, fmt.Sprintf("声明树：%d 条 launchd · %d 条 ghost（已登记幽灵）· 现值自研件 %d 条 · launchctl 里 zerg 行 %d",
		len(launchdRows), len(ghostRows), len(live), lcZerg))

	zerg := ctx.zerg
	if zerg == "" {
		zerg = filepath.Join(ctx.root, "bin", "zerg")
	}
	dcount, hasCount, ddetail, derr := gddReadDoctor(zerg, ctx.doctorFile, 60)
	if derr != "" {
		return 2, side1, side2, notes, []string{fmt.Sprintf("同源读数取不到：%s（`T2` 判据③ 的同一格 ⇒ 不许换一份口径）", derr)}
	}
	// ★ GAP-20260927-62 治尾巴①(b)（与 py 半同源）：S2 比**侧① 中 registered_as 非空的子集**，
	//   ✗ 不比 `len(side1)`（含未登记幽灵）—— doctor ⑧ 与 `core daemon ls --declared` 同调
	//   `ghostProcesses()`（`kind=ghost` 行 × 现值命中）⇒ doctor 数的**正是已登记幽灵**，
	//   未登记幽灵它根本看不见 ⇒ 旧写法会把「多一条未登记幽灵」误判成同源漂移红。
	regN := 0
	for _, it := range side1 {
		if it.HasReg {
			regN++
		}
	}
	if !hasCount || dcount != regN {
		dc := dcount
		problems = append(problems, fmt.Sprintf("FAIL S2 同源漂移：本件侧① 已登记（`registered_as` 非空）%d 条 vs `zerg doctor`「%s」%d 条 ⇒ 两处口径已分叉（`T2` 判据③ 要求逐字同值）· doctor 原读：%s",
			regN, gddDoctorItem, dc, ddetail))
	}
	sameStr := "逐字同值 ✓"
	if dcount != regN {
		sameStr = "不等 ✗"
	}
	notes = append(notes, fmt.Sprintf("S2 同源：侧① 已登记 %d 条（侧① 共 %d 条）· doctor「%s」%d 条（%s）", regN, len(side1), gddDoctorItem, dcount, sameStr))
	doctorMode := "live"
	if ctx.doctorFile != "" {
		doctorMode = "fixture"
	}
	notes = append(notes, fmt.Sprintf("取证：ps=%s · launchctl=%s · doctor=%s", psmode, lcmode, doctorMode))

	rc := 0
	if len(problems) > 0 {
		rc = 1
	}
	return rc, side1, side2, notes, problems
}

// ─────────────────────────────── 报告 ───────────────────────────────
func gddPrintReport(stdout io.Writer, root, declPath string, rc int, side1 []gddSide1, side2 []gddSide2, notes, problems []string) {
	p := func(s string) { fmt.Fprintln(stdout, s) }
	p("── `D4` 幽灵/声明差**只读 diff**（现值 vs 声明 · 两侧逐条 · **不自动清 / 不自动补**）──")
	relDecl := declPath
	if strings.HasPrefix(declPath, root) {
		if r, err := filepath.Rel(root, declPath); err == nil {
			relDecl = r
		}
	}
	shaStr := "缺件"
	if st, err := os.Stat(declPath); err == nil && !st.IsDir() {
		s := gddFileSha(declPath)
		if len(s) >= 16 {
			shaStr = s[:16] + "…"
		}
	}
	p(fmt.Sprintf("  声明侧真源: %s（sha256 %s）", relDecl, shaStr))
	p("  解析器（同源）: 门⑧ `check-service-declaration.py` 的 `parse_decl`（**同一份代码** · importlib 加载）")
	p(fmt.Sprintf("  现值侧来源: `ps -eo pid,ppid,etime,command` + `launchctl list` · 进程特征 %s（与门⑧ 同一对命令同一组特征）",
		strings.Join(gddSiblingHints, " ")))
	for _, n := range notes {
		p("  · " + n)
	}
	p(fmt.Sprintf("  ── 侧①「**盘上有 / 声明无**」（现值自研进程里，不被任何 `kind=launchd` 声明覆盖的条）：%d 条", len(side1)))
	if len(side1) == 0 {
		p("     （空）0 条：现值每一件自研件都在声明面可查（launchd 或已登记 ghost）")
	}
	for _, it := range side1 {
		var tag string
		if it.HasReg {
			tag = fmt.Sprintf("已登记：ghost 行「%s」· 处置=%.40s", it.RegisteredAs, it.Disposition)
		} else {
			tag = "**未登记幽灵**（门⑧ R4 的红形态）"
		}
		p(fmt.Sprintf("     · pid=%-7s %s", it.Pid, gddTruncRunes(it.Cmdline, 110)))
		p(fmt.Sprintf("       〔%s〕", tag))
	}
	p(fmt.Sprintf("  ── 侧②「**声明有 / 盘上无**」（`kind=launchd` 声明的落点缺环）：%d 条", len(side2)))
	if len(side2) == 0 {
		p("     （空）0 条：每条 launchd 声明的三环（`launchctl` 装载 · `ps` 进程 · 声明件在盘）全在")
	}
	for _, it := range side2 {
		p(fmt.Sprintf("     · %s（声明件 %s · 声明树第 %d 行）缺环：%s", it.Name, it.Decl, it.Line, strings.Join(it.Missing, " + ")))
	}
	p("  ── 只读边界 ──")
	p(fmt.Sprintf("     · 写旗标闭集（%d 词）一律拒 rc=2；源码级写 API 扫描（自检标记之前的真目标段）命中即 rc=1", len(gddWriteWords)))
	p("     · 本件**不做**任何清 / 补 / 停 / 杀：停一个幽灵 = 不可逆动作 ⇒ 照 §九 M9 待 Mr2109 拍（本件只报差）")
	for _, pr := range problems {
		p("  · " + pr)
	}
	switch rc {
	case 0:
		p("⇒ 两侧已逐条列出 · 同源断言过（rc=0）—— 差集本身不驱动退码：本件是**只读 diff**，报差就是它的活")
	case 1:
		p("⇒ rc=1：同源断言或写面扫描判红（逐条见上）")
	default:
		p("⇒ 不给结论（rc=2）")
	}
}

func gddListRules(stdout, stderr io.Writer, selfPy string) int {
	p := func(s string) { fmt.Fprintln(stdout, s) }
	p(fmt.Sprintf("── %s 判据与闭集 ──", gddVersion))
	p("侧①「盘上有/声明无」= 现值自研进程 − `kind=launchd` 声明的进程特征；侧②「声明有/盘上无」= launchd 声明的三环缺环")
	p("同源 S1：真源 = `deploy/服务声明.tsv`（声明侧唯一真源 · 与门⑧/`core daemon ls` 同一件）· 解析器 = 门⑧ 的 `parse_decl`")
	p(fmt.Sprintf("同源 S2：侧①条数 == `zerg doctor`「%s」条数（`T2` 判据③ · 逐字同值）", gddDoctorItem))
	p(fmt.Sprintf("写旗标闭集（%d 词 · 形态无关）：%s", len(gddWriteWords), strings.Join(gddWriteWords, " ")))
	// ★ 对齐现 py（GAP-20261001-46 已修）：旧 py `list_rules()` 第 377 行引用未定义的 `DESTRUCTIVE_TOKENS`
	// ⇒ NameError 回溯 + rc=1；本件早前逐字复刻的是那份**未修**行为 ⇒ 与现 py 分叉。
	// 现 py 逐字改用**已定义**的 `DELETE_CALLS` ⇒ 出「源码级牙词表」行 + rc=0。照现 py 对齐（✗ 不复刻回溯）。
	_ = selfPy
	_ = stderr
	p(fmt.Sprintf("源码级牙词表：%s + `open(…,'w'|'a')` + git 写子命令字面", strings.Join(gddDeleteCalls, " ")))
	p("退码：0 两侧已出+同源过 · 1 同源漂移/写面红 · 2 不给结论（缺件/取不到/空转/写旗标/自检不过）")
	return 0
}

// ─────────────────────────────── 入口 ───────────────────────────────
func gddParseWriteFlags(argv []string) []string {
	var hits []string
	for _, tok := range argv {
		t := strings.TrimSpace(tok)
		if t == "" || t == "--" {
			continue
		}
		name := strings.TrimLeft(t, "-")
		if i := strings.Index(name, "="); i >= 0 {
			name = name[:i]
		}
		name = strings.ToLower(strings.TrimSpace(name))
		if gddInList(gddWriteWords, name) {
			hits = append(hits, t)
		}
	}
	return hits
}

func checkGhostDeclDiffGateMain(stdout, stderr io.Writer, argv []string) int {
	if r := gddWriteFlagReject(stdout, argv); r >= 0 {
		return r
	}
	a, perr := gddParseArgs(argv)
	if perr != nil {
		fmt.Fprint(stderr, perr.Error())
		return 2
	}
	if a.help {
		fmt.Fprint(stdout, gddHelpText())
		return 0
	}
	if a.version {
		fmt.Fprintln(stdout, gddVersion)
		return 0
	}
	if a.selfTest {
		return gddSelfTest(stdout)
	}
	{
		lrRoot := a.root
		if lrRoot == "" {
			lrRoot = gddDefaultRoot()
		} else {
			lrRoot = gddAbs(lrRoot)
		}
		if a.listRules {
			return gddListRules(stdout, stderr, filepath.Join(gddScriptDir(lrRoot), gddSelfName))
		}
	}

	root := a.root
	if root == "" {
		root = gddDefaultRoot()
	} else {
		root = gddAbs(root)
	}

	// ── 源码级牙（D3②）：真目标段出现任何写 API ⇒ 先判红 ──
	// ★ 数据来源 = 内嵌快照（`gddSelfPySource` · 与 py 原件逐字节同）—— ✗ 不再读盘上 py 件：
	// 该件退役（移出仓）后盘上无件，读它会静默跳过这一格；快照让「在盘 / 移出」两态逐字同。
	selfPy := filepath.Join(gddScriptDir(root), gddSelfName)
	if src := gddSelfPySource(); src != "" {
		if wp := assertNoWriteAPIPy(src, gddSelftestMarker); len(wp) > 0 {
			fmt.Fprintln(stdout, "⛔ 判红（rc=1）：本件**真目标段**出现写 API ⇒ 「不自动删/不自动改」这一格破了：")
			for _, h := range wp {
				fmt.Fprintln(stdout, "   · "+h)
			}
			return 1
		}
	}

	if err := gddLoadSibling(root, selfPy); err != "" {
		fmt.Fprintln(stdout, "⛔ 不给结论（rc=2）："+err)
		return 2
	}

	declPath := a.decl
	if declPath == "" {
		declPath = os.Getenv("ZERG_SVCDECL_DECL")
	}
	if declPath == "" {
		declPath = filepath.Join(root, "deploy", "服务声明.tsv")
	}

	var fpBefore []gddFP
	if a.proveReadonly {
		fpBefore = gddReadonlyFingerprint(root, declPath)
	}

	ctx := &gddCtx{root: root, declPath: a.decl, psFile: a.ps, lcFile: a.launchctl,
		home: a.home, zerg: a.zerg, doctorFile: a.doctor}
	rc, side1, side2, notes, problems := gddCheck(ctx)
	if rc == 2 && len(side1) == 0 && len(side2) == 0 {
		fmt.Fprintln(stdout, "⛔ 不给结论（rc=2）：")
		for _, pr := range problems {
			fmt.Fprintln(stdout, "   · "+pr)
		}
		return 2
	}

	// gddCheck 内部已按 ctx.declPath 推导（与 py 同形），此处取回用于打印
	effDecl := a.decl
	if effDecl == "" {
		effDecl = os.Getenv("ZERG_SVCDECL_DECL")
	}
	if effDecl == "" {
		effDecl = filepath.Join(root, "deploy", "服务声明.tsv")
	}
	_ = declPath

	if a.json {
		fmt.Fprintln(stdout, gddJSONOutput(effDecl, side1, side2, notes, problems))
	} else {
		gddPrintReport(stdout, root, effDecl, rc, side1, side2, notes, problems)
	}

	if a.proveReadonly {
		fpAfter := gddReadonlyFingerprint(root, effDecl)
		merged := map[string]bool{}
		before := map[string]string{}
		after := map[string]string{}
		for _, f := range fpBefore {
			merged[f.Key] = true
			before[f.Key] = f.Val
		}
		for _, f := range fpAfter {
			merged[f.Key] = true
			after[f.Key] = f.Val
		}
		var keys []string
		for k := range merged {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		type d3 struct{ k, b, x string }
		var diffs []d3
		for _, k := range keys {
			if before[k] != after[k] {
				diffs = append(diffs, d3{k, before[k], after[k]})
			}
		}
		fmt.Fprintf(stdout, "── 只读自证（D2）：前 %d 件 / 后 %d 件 · 差异 %d 件 ──\n", len(fpBefore), len(fpAfter), len(diffs))
		for _, d := range diffs {
			fmt.Fprintf(stdout, "      ✗ %s：%s ⇒ %s\n", d.k, gddTruncRunes(d.b, 16), gddTruncRunes(d.x, 16))
		}
		if len(diffs) > 0 {
			fmt.Fprintln(stdout, "  ⇒ rc=1：跑本件期间仓内件 sha256 变了（逐件见上）")
			return 1
		}
		fmt.Fprintln(stdout, "  ⇒ 零副作用：真源件 + 门⑧ 源件 + 本件 + `.git` 元数据 + porcelain 逐件不变（差异 0 件）")
	}
	return rc
}

// ─────────────────────────────── 参数解析（argparse 语义近似）───────────────────────────────
type gddArgs struct {
	root, decl, ps, launchctl, home, zerg, doctor string
	json, proveReadonly, listRules, selfTest      bool
	help, version                                 bool
}

const gddUsageBlock = "usage: check-ghost-decl-diff.py [-h] [--root ROOT] [--decl DECL] [--ps PS]\n" +
	"                                [--launchctl LAUNCHCTL] [--home HOME]\n" +
	"                                [--zerg ZERG] [--doctor DOCTOR] [--json]\n" +
	"                                [--prove-readonly] [--list-rules]\n" +
	"                                [--self-test] [--version]\n"

func gddHelpText() string {
	return gddUsageBlock + "\n" +
		"D4 幽灵/声明差只读 diff（两侧逐条 · 不自动清/补）\n" +
		"\n" +
		"optional arguments:\n" +
		"  -h, --help            show this help message and exit\n" +
		"  --root ROOT\n" +
		"  --decl DECL           声明树（默认 <仓根>/deploy/服务声明.tsv；也可用 ZERG_SVCDECL_DECL）\n" +
		"  --ps PS               ps 输出落文件（默认真跑 ps；也可用 ZERG_SVCDECL_PS）\n" +
		"  --launchctl LAUNCHCTL\n" +
		"                        launchctl 输出落文件（默认真跑；也可用 ZERG_SVCDECL_LAUNCHCTL）\n" +
		"  --home HOME           `~/` 展开用的家目录\n" +
		"  --zerg ZERG           命令面可跑制品（默认 <仓根>/bin/zerg）\n" +
		"  --doctor DOCTOR       doctor JSON 落文件（夹具；默认真跑 `zerg doctor --json\n" +
		"                        name,verdict,detail,advice`）\n" +
		"  --json\n" +
		"  --prove-readonly      跑前后对拍（真源件 + 门⑧ 源件 + 本件 + .git 元数据 + porcelain）\n" +
		"  --list-rules\n" +
		"  --self-test\n" +
		"  --version             show program's version number and exit\n"
}

func gddParseArgs(argv []string) (*gddArgs, error) {
	a := &gddArgs{}
	var extras []string
	i := 0
	needVal := func(name string, val string, hasVal bool) (string, error) {
		if hasVal {
			return val, nil
		}
		i++
		if i >= len(argv) {
			return "", fmt.Errorf("%sargument %s: expected one argument\n", gddUsageBlock, name)
		}
		return argv[i], nil
	}
	for ; i < len(argv); i++ {
		tok := argv[i]
		if tok == "-h" || tok == "--help" {
			a.help = true
			return a, nil
		}
		if tok == "--version" {
			a.version = true
			return a, nil
		}
		if strings.HasPrefix(tok, "--") {
			name := tok
			val := ""
			hasVal := false
			if eq := strings.Index(tok, "="); eq >= 0 {
				name = tok[:eq]
				val = tok[eq+1:]
				hasVal = true
			}
			var err error
			switch name {
			case "--root":
				a.root, err = needVal("--root", val, hasVal)
			case "--decl":
				a.decl, err = needVal("--decl", val, hasVal)
			case "--ps":
				a.ps, err = needVal("--ps", val, hasVal)
			case "--launchctl":
				a.launchctl, err = needVal("--launchctl", val, hasVal)
			case "--home":
				a.home, err = needVal("--home", val, hasVal)
			case "--zerg":
				a.zerg, err = needVal("--zerg", val, hasVal)
			case "--doctor":
				a.doctor, err = needVal("--doctor", val, hasVal)
			case "--json":
				a.json = true
			case "--prove-readonly":
				a.proveReadonly = true
			case "--list-rules":
				a.listRules = true
			case "--self-test":
				a.selfTest = true
			default:
				extras = append(extras, tok)
				continue
			}
			if err != nil {
				return nil, err
			}
			continue
		}
		extras = append(extras, tok)
	}
	if len(extras) > 0 {
		return nil, fmt.Errorf("%scheck-ghost-decl-diff.py: error: unrecognized arguments: %s\n", gddUsageBlock, strings.Join(extras, " "))
	}
	return a, nil
}

// ─────────────────────────────── 路径/字符串小工具 ───────────────────────────────
func gddAbs(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(wd, p))
}

func gddDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := wd
	for n := 0; n < 8; n++ {
		if st, err := os.Stat(filepath.Join(d, "scripts", "gates")); err == nil && st.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return wd
}

func gddExpandUser() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "~"
}

func gddJoinHome(home, rest string) string {
	if strings.HasSuffix(home, "/") {
		return home + rest
	}
	return home + "/" + rest
}

func gddTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func gddFileShaText(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// gddPyReprStr —— Python `%r` 的 str 形态（单引号 · 反斜杠转义）。
func gddPyReprStr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

func gddPyRepr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return gddPyReprStr(t)
	case int:
		return fmt.Sprintf("%d", t)
	case bool:
		if t {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func gddPyReprStrList(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = gddPyReprStr(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func gddPyReprListStr(xs []string) string {
	return gddPyReprStrList(xs)
}

// ─────────────────────────────── 同源件（S1）───────────────────────────────
func gddLoadSibling(root, selfPy string) string {
	sib := filepath.Join(gddScriptDir(root), gddSiblingName)
	st, err := os.Stat(sib)
	if err != nil || st.IsDir() {
		// ★ 退役态（G17 · 2026-10-07 退役批）：同源件 `check-service-declaration.py`（门⑧ py 半）
		//   已退役（不在盘）⇒ 口径真源已换人（其 **Go 半** `core/cmd/zerg/check_service_declaration.go`）
		//   ⇒ 按退役态处置：改查 Go 半里的**同一组同源口径标记**（✗ 不因缺 py 而 fail-closed）。
		//   Go 半也不在盘 / 标记不齐（合成夹具 · 最小仓 · 真坏件）⇒ 保留原「不给结论」档（✗ 不静默放行）。
		if why := gddSiblingGoHalfOK(root); why == "" {
			return ""
		} else {
			return fmt.Sprintf("同源件缺失：%s（门⑧ = 声明侧真源的唯一解析器）且 Go 半不可用（%s）⇒ 不给结论", sib, why)
		}
	}
	b, rerr := os.ReadFile(sib)
	if rerr != nil {
		return fmt.Sprintf("同源件加载失败：%s（%s）⇒ 不给结论", sib, rerr)
	}
	src := string(b)
	for _, attr := range []string{"parse_decl", "PROC_HINTS", "read_lines", "label_set_diff"} {
		if !strings.Contains(src, "def "+attr) && !strings.Contains(src, attr+" =") && !strings.Contains(src, attr+"=") {
			return fmt.Sprintf("同源件里没有 %s ⇒ 口径对不上（不给结论）", attr)
		}
	}
	_ = selfPy
	return ""
}

// gddSiblingGoHalfOK —— 门⑧ 退役态：py 半退役后，其口径真源改由 **Go 半** 承载。
// 现读 Go 半 `core/cmd/zerg/check_service_declaration.go` 里四个同源口径符号
// （与查 py 半的 parse_decl / PROC_HINTS / read_lines / label_set_diff 一一对应）。
// 返回 "" = 可用；否则失败说明（调用方据此走原「不给结论」档）。
func gddSiblingGoHalfOK(root string) string {
	p := filepath.Join(root, "core", "cmd", "zerg", "check_service_declaration.go")
	b, err := os.ReadFile(p)
	if err != nil {
		return "Go 半读不到（" + filepath.ToSlash(filepath.Join("core", "cmd", "zerg", "check_service_declaration.go")) + "）：" + err.Error()
	}
	src := string(b)
	for _, sym := range []string{"csdParseDecl", "csdProcHints", "csdReadLines", "csdLabelSetDiff"} {
		if !strings.Contains(src, sym) {
			return "Go 半里没有 " + sym
		}
	}
	return ""
}

// ─────────────────────────────── JSON（zerg/v1）───────────────────────────────
func gddJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func gddJSONOrNull(s string, present bool) string {
	if !present {
		return "null"
	}
	return gddJSONStr(s)
}

func gddJSONArrStr(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = gddJSONStr(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func gddJSONOutput(declPath string, side1 []gddSide1, side2 []gddSide2, notes, problems []string) string {
	var items []string
	for _, it := range side1 {
		items = append(items, `{"side": "盘上有/声明无", "pid": `+gddJSONStr(it.Pid)+
			`, "cmdline": `+gddJSONStr(it.Cmdline)+
			`, "registered_as": `+gddJSONOrNull(it.RegisteredAs, it.HasReg)+
			`, "disposition": `+gddJSONOrNull(it.Disposition, it.HasDisp)+
			`, "raw": `+gddJSONStr(it.Raw)+`}`)
	}
	for _, it := range side2 {
		items = append(items, `{"side": "声明有/盘上无", "name": `+gddJSONStr(it.Name)+
			`, "decl": `+gddJSONStr(it.Decl)+
			`, "missing": `+gddJSONArrStr(it.Missing)+
			`, "line": `+fmt.Sprintf("%d", it.Line)+`}`)
	}
	declSha := "null"
	if st, err := os.Stat(declPath); err == nil && !st.IsDir() {
		declSha = gddJSONStr(gddFileSha(declPath))
	}
	var b strings.Builder
	b.WriteString(`{"schema": "zerg/v1", "kind": "` + "Ghost" + `DeclDiff", "items": [`)
	b.WriteString(strings.Join(items, ", "))
	b.WriteString(`], "meta": {"count": ` + fmt.Sprintf("%d", len(side1)+len(side2)) +
		`, "side1": ` + fmt.Sprintf("%d", len(side1)) +
		`, "side2": ` + fmt.Sprintf("%d", len(side2)) +
		`, "decl": ` + gddJSONStr(declPath) +
		`, "decl_sha256": ` + declSha +
		`, "doctor_item": ` + gddJSONStr(gddDoctorItem) +
		`, "tool": ` + gddJSONStr(gddVersion) + `}, "warnings": [], "truncated": false, "notes": ` +
		gddJSONArrStr(notes) + `, "problems": ` + gddJSONArrStr(problems) + `}`)
	return b.String()
}

// ─────────────────────────────── 自检夹具 ───────────────────────────────
const gddGoodDecl = "# 夹具声明树\n" +
	"kind\tname\t声明件\t程序\t进程特征\t归属\t处置\n" +
	"launchd\tcom.zerg.core\t~/Library/LaunchAgents/com.zerg.core.plist\t/bin/bash scripts/svc/zerg-core-daemon.sh\tbin/zerg-core\t主控\t-\n" +
	"launchd\tcom.zerg.agent-Mr2109\t~/Library/LaunchAgents/com.zerg.agent-Mr2109.plist\tbin/zerg-agentd --machine Mr2109\tbin/zerg-agentd\t子端\t-\n" +
	"ghost\tcocoon-docs-service\t-\tbin/cocoon-docs-service -port 8610\tbin/cocoon-docs-service\t文档茧\t进 reap 候选 · 待 Mr2109 拍\n"

const gddGoodPS = "  68939     1       56:36 /repo/bin/zerg-core\n" +
	"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
	"  79317     1 04-22:34:32 /repo/bin/cocoon-docs-service -port 8610\n"

const gddGoodLC = "68939\t0\tcom.zerg.core\n14149\t0\tcom.zerg.agent-Mr2109\n"

func gddDoctorJSON(n int, detail string) string {
	if detail == "" {
		detail = fmt.Sprintf("%d 条：pid=79317 /repo/bin/cocoon-docs-service -port 8610", n)
	}
	verdict := "PASS"
	if n != 0 {
		verdict = "REPORT"
	}
	return `{"schema": "zerg/v1", "kind": "DoctorCheck", "items": [{"name": ` + gddJSONStr(gddDoctorItem) +
		`, "verdict": "` + verdict + `", "detail": ` + gddJSONStr(detail) + `, "advice": ""}]}`
}

func gddWriteFile(path, content string, mode os.FileMode) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(content), mode)
}

func gddFixture(d, decl, ps, lc string, plists []string) (string, string, string, string) {
	_ = os.MkdirAll(d, 0o755)
	dp := filepath.Join(d, "服务声明.tsv")
	pp := filepath.Join(d, "ps.txt")
	lp := filepath.Join(d, "launchctl.txt")
	gddWriteFile(dp, decl, 0o644)
	gddWriteFile(pp, ps, 0o644)
	gddWriteFile(lp, lc, 0o644)
	home := filepath.Join(d, "home")
	la := filepath.Join(home, "Library", "LaunchAgents")
	_ = os.MkdirAll(la, 0o755)
	for _, n := range plists {
		gddWriteFile(filepath.Join(la, n), "plist\n", 0o644)
	}
	_ = os.MkdirAll(filepath.Join(d, ".git"), 0o755)
	for _, n := range []string{"HEAD", "index"} {
		gddWriteFile(filepath.Join(d, ".git", n), "ref: refs/heads/main\n", 0o644)
	}
	return dp, pp, lp, home
}

// gddSelfTest —— 成对负控 · 合成夹具 · 不碰真目标。★ 自回收：defer os.RemoveAll(tmp)
// （py 原件用 tempfile.mkdtemp **从不回收** —— A 类系统临时目录泄漏，见件头未覆盖面 ②）。
func gddSelfTest(stdout io.Writer) int { return gddSelfTestRoot(stdout, gddDefaultRoot()) }

// gddSelfTestRoot —— gddSelfTest 的可注入根版本（root 决定同源件与「本件 py 真身」的位置）。
// ★ 退役态（GAP-20261007-16 · 判据件退役前置 ③）：py 真身退役后本函数仍须照常出结论（✗ 不 fail-closed）。
func gddSelfTestRoot(stdout io.Writer, root string) int {
	ok := true
	tmp, terr := os.MkdirTemp("", "gdd-selftest-")
	if terr != nil {
		fmt.Fprintf(stdout, "  ✗ 同源件加载失败：%s\n", terr)
		return 1
	}
	defer os.RemoveAll(tmp)

	if err := gddLoadSibling(root, ""); err != "" {
		fmt.Fprintf(stdout, "  ✗ 同源件加载失败：%s\n", err)
		return 1
	}

	casef := func(name string, want, got, note string) {
		good := want == got
		ok = ok && good
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（%s · 期望 %s）%s\n", mark, name, got, want, note)
	}

	d1 := filepath.Join(tmp, "f1")
	dp, pp, lp, home := gddFixture(d1, gddGoodDecl, gddGoodPS, gddGoodLC, []string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist"})
	df := filepath.Join(d1, "doctor.json")
	gddWriteFile(df, gddDoctorJSON(1, ""), 0o644)
	rc, s1, s2, _, _ := gddCheck(&gddCtx{root: d1, declPath: dp, psFile: pp, lcFile: lp, home: home, doctorFile: df})
	note1 := fmt.Sprintf("  s1=%d s2=%d", len(s1), len(s2))
	casef("① 正例：侧① 1 条 / 侧② 0 条 / doctor 同值 ⇒ rc=0", "0", fmt.Sprintf("%d", rc), note1)
	regAs := "None"
	if len(s1) > 0 && s1[0].HasReg {
		regAs = s1[0].RegisteredAs
	}
	casef("①b 侧①那条被标成「已登记 ghost」（不是未登记幽灵）", "cocoon-docs-service", regAs, "")
	casef("①c 两侧都不静默：侧② 空也要有 0 条的形态", "0", fmt.Sprintf("%d", len(s2)), "")

	df0 := filepath.Join(d1, "doctor0.json")
	gddWriteFile(df0, gddDoctorJSON(0, "现值面没看到「声明外」的服务进程"), 0o644)
	rc0, _, _, _, probs0 := gddCheck(&gddCtx{root: d1, declPath: dp, psFile: pp, lcFile: lp, home: home, doctorFile: df0})
	n02 := ""
	if len(probs0) > 0 {
		n02 = "  " + gddTruncRunes(probs0[0], 60)
	}
	casef("② 负控：doctor 0 条 ≠ 侧① 1 条 ⇒ 同源漂移 rc=1", "1", fmt.Sprintf("%d", rc0), n02)

	d3 := filepath.Join(tmp, "f3")
	dp3, pp3, lp3, home3 := gddFixture(d3, gddGoodDecl, gddGoodPS, gddGoodLC, []string{"com.zerg.core.plist"})
	df3 := filepath.Join(d3, "doctor.json")
	gddWriteFile(df3, gddDoctorJSON(1, ""), 0o644)
	rc3, _, s23, _, _ := gddCheck(&gddCtx{root: d3, declPath: dp3, psFile: pp3, lcFile: lp3, home: home3, doctorFile: df3})
	n33 := "—"
	if len(s23) > 0 {
		n33 = gddPyReprListStr(s23[0].Missing)
	}
	casef("③ 侧② 非空（声明件不在盘）⇒ 逐条列 + rc 仍 0（报差不驱动退码）", "(0, 1)", fmt.Sprintf("(%d, %d)", rc3, len(s23)), "  缺环 "+n33)

	fake := "import os\ndef f(p):\n    os.remove(p)\n"
	casef("④ 负控（源码级牙）：合成源码含 "+"`os.remove(`"+` ⇒ 命中`, "True", gddBoolStr(len(assertNoWriteAPIPy(fake, "")) > 0), "")
	// ★ 两态逐字同（本单：断「Go 半读 py 源」依赖 · 数据来源改**内嵌快照**）：
	// 旧口径硬读本件 py 真身源码 ⇒ py 退役（不在盘）时这一格分叉成「④bc 退役态」。
	// 现口径：两条正例一律扫内嵌快照（`gddSelfPySource` · 与 py 原件逐字节同）——
	// py 在盘 / 移出仓 两态输出**逐字节同**，且源码级牙真跑（✗ 不判红、✗ 不削这一格）。
	src := gddSelfPySource()
	hits := assertNoWriteAPIPy(src, gddSelftestMarker)
	n4b := ""
	if len(hits) > 0 {
		hs := hits
		if len(hs) > 3 {
			hs = hs[:3]
		}
		n4b = "  " + strings.Join(hs, " / ")
	}
	casef("④b 正例：本件**真目标段**写 API 命中数 = 0（自检夹具段豁免）", "0", fmt.Sprintf("%d", len(hits)), n4b)
	casef("④c 正例：同一份源码**整篇**扫（含自检段）应有命中 ⇒ 证明扫描器不是恒空", "True", gddBoolStr(len(assertNoWriteAPIPy(src, "")) > 0), "")

	for _, flag := range []string{"--reap", "--clean", "--fix", "commit", "--stop", "--kill", "--apply"} {
		rcf := gddRunWriteFlagMain(flag)
		casef(fmt.Sprintf("⑤ 负控：写旗标 %s ⇒ 拒 rc=2", gddPyReprStr(flag)), "2", fmt.Sprintf("%d", rcf), "")
	}

	rc6, _, _, _, p6 := gddCheck(&gddCtx{root: d1, declPath: filepath.Join(d1, "不存在.tsv"), psFile: pp, lcFile: lp, home: home, doctorFile: df})
	n6 := ""
	if len(p6) > 0 {
		n6 = "  " + gddTruncRunes(p6[0], 40)
	}
	casef("⑥ 缺件（声明树不在）⇒ rc=2 不给结论", "2", fmt.Sprintf("%d", rc6), n6)

	pp7 := filepath.Join(d1, "ps-empty.txt")
	gddWriteFile(pp7, "    1     0  00:00 /sbin/launchd\n", 0o644)
	rc7, _, _, _, p7 := gddCheck(&gddCtx{root: d1, declPath: dp, psFile: pp7, lcFile: lp, home: home, doctorFile: df})
	n7 := ""
	if len(p7) > 0 {
		n7 = "  " + gddTruncRunes(p7[0], 30)
	}
	casef("⑦ 空转（ps 里 0 件自研件）⇒ rc=2", "2", fmt.Sprintf("%d", rc7), n7)

	rc8, _, _, _, p8 := gddCheck(&gddCtx{root: d1, declPath: dp, psFile: pp, lcFile: filepath.Join(d1, "没有-launchctl.txt"), home: home, doctorFile: df})
	n8 := ""
	if len(p8) > 0 {
		n8 = "  " + gddTruncRunes(p8[0], 40)
	}
	casef("⑧ launchctl 取不到 ⇒ rc=2（不当绿）", "2", fmt.Sprintf("%d", rc8), n8)

	// ⑨ 未登记幽灵：侧① 2 条（一登记一未登记）；S2 只比**已登记子集**（1 条）⇒ doctor 1 条同值 ⇒ rc=0
	//   （★ GAP-20260927-62(b)：未登记幽灵不参与 S2，旧口径 doctor 2 条此格必红）
	pp9 := filepath.Join(d1, "ps-extra.txt")
	gddWriteFile(pp9, gddGoodPS+"  88888     1       10:00 /repo/bin/zerg-mystery --boot\n", 0o644)
	df9 := filepath.Join(d1, "doctor9.json")
	gddWriteFile(df9, gddDoctorJSON(1, ""), 0o644)
	rc9, s19, _, _, _ := gddCheck(&gddCtx{root: d1, declPath: dp, psFile: pp9, lcFile: lp, home: home, doctorFile: df9})
	unreg := 0
	for _, it := range s19 {
		if !it.HasReg {
			unreg++
		}
	}
	casef("⑨ 未登记幽灵：侧① 2 条（一登记一未登记）· S2 比已登记子集 1 条 == doctor 1 条 ⇒ rc=0", "(0, 2, 1)",
		fmt.Sprintf("(%d, %d, %d)", rc9, len(s19), unreg), "")

	// ⑪ PPID 深度1直系豁免（★ GAP-20260927-62(a)）：zerg-core(声明服务 · pid=68939) 的直接子
	//   task 进程（ppid=68939）不算未登记幽灵 ⇒ 侧① 仍 1 条（只 cocoon）· 未登记 0 条 ⇒ rc=0
	pp11 := filepath.Join(d1, "ps-child.txt")
	gddWriteFile(pp11, gddGoodPS+"  77777     68939   00:05 /repo/bin/zerg-taskworker --scoped\n", 0o644)
	df11 := filepath.Join(d1, "doctor-child.json")
	gddWriteFile(df11, gddDoctorJSON(1, ""), 0o644)
	rc11, s111, _, _, _ := gddCheck(&gddCtx{root: d1, declPath: dp, psFile: pp11, lcFile: lp, home: home, doctorFile: df11})
	unreg11 := 0
	for _, it := range s111 {
		if !it.HasReg {
			unreg11++
		}
	}
	casef("⑪ PPID 深度1直系豁免：声明服务直接子 task 进程不入侧① ⇒ (0,1,0)", "(0, 1, 0)",
		fmt.Sprintf("(%d, %d, %d)", rc11, len(s111), unreg11), "")

	// ⑪b 负控：只豁免**深度 1** —— 孙进程（父=子进程 pid 77777 · 非声明）与父非声明的散件仍计未登记
	pp11b := filepath.Join(d1, "ps-stray.txt")
	gddWriteFile(pp11b, gddGoodPS+"  77777     68939   00:05 /repo/bin/zerg-taskworker --scoped\n"+
		"  88888     77777   00:05 /repo/bin/zerg-grand --deep\n"+
		"  66666       1     01:00 /repo/bin/zerg-stray --boot\n", 0o644)
	df11b := filepath.Join(d1, "doctor-stray.json")
	gddWriteFile(df11b, gddDoctorJSON(1, ""), 0o644)
	rc11b, s111b, _, _, _ := gddCheck(&gddCtx{root: d1, declPath: dp, psFile: pp11b, lcFile: lp, home: home, doctorFile: df11b})
	unreg11b := 0
	for _, it := range s111b {
		if !it.HasReg {
			unreg11b++
		}
	}
	casef("⑪b 负控：孙进程/父非声明 仍计未登记 ⇒ (0,3,2)", "(0, 3, 2)",
		fmt.Sprintf("(%d, %d, %d)", rc11b, len(s111b), unreg11b), "")

	// ★ 缺口 `GAP-20260927-101`：自检夹具**参数化**（作业数 / 标签集合 / 瞬态工人 / 命令行含字面 shell
	//   各造一格）＋一条「**计数可抵消**」的成对负控。旧夹具固定成「2 launchd + 3 ps」一种形状
	//   ⇒ 与标签集合 / 作业清单相关的改动与失效，自检一律测不出来。
	// ⑫ 作业数 3 / 标签集合 3（无幽灵）：侧① 0 条 · 侧② 0 条 · doctor 0 条 ⇒ rc=0
	decl3 := "# 夹具声明树（参数化 3 作业）\n" +
		"kind\tname\t声明件\t程序\t进程特征\t归属\t处置\n" +
		"launchd\tcom.zerg.core\t~/Library/LaunchAgents/com.zerg.core.plist\t/bin/bash scripts/svc/zerg-core-daemon.sh\tbin/zerg-core\t主控\t-\n" +
		"launchd\tcom.zerg.agent-Mr2109\t~/Library/LaunchAgents/com.zerg.agent-Mr2109.plist\tbin/zerg-agentd --machine Mr2109\tbin/zerg-agentd\t子端\t-\n" +
		"launchd\tcom.zerg.newjob-Mr2109\t~/Library/LaunchAgents/com.zerg.newjob-Mr2109.plist\t/bin/bash scripts/svc/zerg-newjob.sh\tbin/zerg-newjob\t新增\t-\n"
	ps3 := "  68939     1       56:36 /repo/bin/zerg-core\n" +
		"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
		"  22222     1       12:00 /repo/bin/zerg-newjob --tick\n"
	lc3 := "68939\t0\tcom.zerg.core\n14149\t0\tcom.zerg.agent-Mr2109\n22222\t0\tcom.zerg.newjob-Mr2109\n"
	d12 := filepath.Join(tmp, "f12")
	dp12, pp12, lp12, home12 := gddFixture(d12, decl3, ps3, lc3, []string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist", "com.zerg.newjob-Mr2109.plist"})
	df12 := filepath.Join(d12, "doctor.json")
	gddWriteFile(df12, gddDoctorJSON(0, "现值面没看到「声明外」的服务进程"), 0o644)
	rc12, s112, s212, _, _ := gddCheck(&gddCtx{root: d12, declPath: dp12, psFile: pp12, lcFile: lp12, home: home12, doctorFile: df12})
	casef("⑫ 参数化（作业数 3 / 标签集合 3 · 无幽灵）：侧① 0 / 侧② 0 · doctor 0 ⇒ rc=0", "(0, 0, 0)",
		fmt.Sprintf("(%d, %d, %d)", rc12, len(s112), len(s212)), "")

	// ⑬ 瞬态工人（未登记幽灵）· 命令行含**字面 shell**（引号 / 分号）：侧① 2 条（1 登记 + 1 未登记）；
	//   S2 只比已登记子集（1 条）== doctor 1 条 ⇒ rc=0。
	pp13 := filepath.Join(tmp, "f13-ps.txt")
	gddWriteFile(pp13, gddGoodPS+"  44444     1       09:00 /repo/bin/zerg-scoped --cmd \"a;b\" --tag 'x y'\n", 0o644)
	df13 := filepath.Join(d1, "doctor13.json")
	gddWriteFile(df13, gddDoctorJSON(1, ""), 0o644)
	rc13, s113, _, _, _ := gddCheck(&gddCtx{root: d1, declPath: dp, psFile: pp13, lcFile: lp, home: home, doctorFile: df13})
	unreg13 := 0
	for _, it := range s113 {
		if !it.HasReg {
			unreg13++
		}
	}
	casef("⑬ 参数化（瞬态工人 · 命令行含字面 shell）：侧① 2（1 登记 + 1 未登记）· S2 比已登记 1 == doctor 1 ⇒ rc=0",
		"(0, 2, 1)", fmt.Sprintf("(%d, %d, %d)", rc13, len(s113), unreg13), "")

	// ⑭ 成对负控「计数可抵消」：声明里**少 1 条幽灵**（3 → 2 行）＋ ps 里**多 1 条未登记作业**
	//   （总行数仍 3 行 · 换掉了 cocoon 那行）⇒ 纯计数判据看不见；而侧① 的**已登记子集**由 1 变 0
	//   ≠ doctor 仍报 1 条 ⇒ S2 同源漂移 ⇒ **必红**。
	decl14 := "# 夹具声明树（少 1 声明 · 可抵消负控）\n" +
		"kind\tname\t声明件\t程序\t进程特征\t归属\t处置\n" +
		"launchd\tcom.zerg.core\t~/Library/LaunchAgents/com.zerg.core.plist\t/bin/bash scripts/svc/zerg-core-daemon.sh\tbin/zerg-core\t主控\t-\n" +
		"launchd\tcom.zerg.agent-Mr2109\t~/Library/LaunchAgents/com.zerg.agent-Mr2109.plist\tbin/zerg-agentd --machine Mr2109\tbin/zerg-agentd\t子端\t-\n"
	ps14 := "  68939     1       56:36 /repo/bin/zerg-core\n" +
		"  14149     1 03-15:30:40 /repo/bin/zerg-agentd --host 127.0.0.1 --port 8100 --machine Mr2109\n" +
		"  44444     1       09:00 /repo/bin/zerg-scoped --cmd \"a;b\"\n"
	d14 := filepath.Join(tmp, "f14")
	dp14, pp14, lp14, home14 := gddFixture(d14, decl14, ps14, gddGoodLC, []string{"com.zerg.core.plist", "com.zerg.agent-Mr2109.plist"})
	df14 := filepath.Join(d14, "doctor14.json")
	gddWriteFile(df14, gddDoctorJSON(1, ""), 0o644)
	rc14, s114, _, _, _ := gddCheck(&gddCtx{root: d14, declPath: dp14, psFile: pp14, lcFile: lp14, home: home14, doctorFile: df14})
	unreg14 := 0
	for _, it := range s114 {
		if !it.HasReg {
			unreg14++
		}
	}
	casef("⑭ 成对负控（计数可抵消 · 少 1 声明 + 多 1 未登记作业 ⇒ ps 行数不变）⇒ S2 已登记子集 0 ≠ doctor 1 ⇒ rc=1",
		"(1, 1, 1)", fmt.Sprintf("(%d, %d, %d)", rc14, len(s114), unreg14), "")

	then := gddReadonlyFingerprint(d1, dp)
	now := gddReadonlyFingerprint(d1, dp)
	casef("⑩ 只读自证：同一夹具连取两次 ⇒ 差异 0 件（检测器不乱报）", "True", gddBoolStr(gddFPEq(then, now)), "")
	f, _ := os.OpenFile(filepath.Join(d1, "服务声明.tsv"), os.O_APPEND|os.O_WRONLY, 0o644)
	if f != nil {
		_, _ = f.WriteString("ghost\t假幽灵\t-\tbin/cocoon-fake\tbin/cocoon-fake\t夹具\t待\n")
		_ = f.Close()
	}
	after := gddReadonlyFingerprint(d1, dp)
	casef("⑩b 负控（白盒）：真源件被改一个字节 ⇒ 指纹真变（不静默）", "True", gddBoolStr(!gddFPEq(then, after)), "")

	concl := "全过"
	if !ok {
		concl = "**有断言失败**"
	}
	fmt.Fprintln(stdout, "自检结论: "+concl)
	if ok {
		return 0
	}
	return 1
}

func gddBoolStr(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func gddFPEq(a, b []gddFP) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key != b[i].Key || a[i].Val != b[i].Val {
			return false
		}
	}
	return true
}

// gddRunWriteFlagMain —— 复刻 py 自检 ⑤ 的 `main([flag])`（输出丢弃 · 只看退码）。
func gddRunWriteFlagMain(flag string) int {
	var sb strings.Builder
	return gddWriteFlagReject(&sb, []string{flag})
}

func gddWriteFlagReject(stdout io.Writer, argv []string) int {
	hits := gddParseWriteFlags(argv)
	if len(hits) == 0 {
		return -1
	}
	p := func(s string) { fmt.Fprintln(stdout, s) }
	p(fmt.Sprintf("⇒ 拒（rc=2 · 不给结论）：命令行里出现写旗标 %s", gddPyReprStrList(hits)))
	p("   理由（`D4` 条件逐字）：**只做只读 diff、不自动清幽灵** —— 幽灵的停/清是不可逆动作，")
	p("   照 §九 M9 不进自动候选（本件连「拼出那条命令」的路径都不给）。")
	p(fmt.Sprintf("   只读用法：bash scripts/gates/%s [--json] [--prove-readonly]", gddSelfName))
	return 2
}

// ─────────────────── 本件 py 原件的**内嵌快照**（退役解耦 · 单一真源）───────────────────
// 为什么内嵌：本 Go 半原先读盘上 py 件（`<root>/scripts/gates/check-ghost-decl-diff.py`）取三个面 ——
// ① 源码级牙（D3②）扫它有无写 API；② 自检 ④b/④c 的同一扫描器正例；
// ③ `--prove-readonly` 指纹把它自身算进逐件 sha256。
// 该 py 一退役（移出仓），这三面就断（④b/④c 分叉 / 指纹少一件 ⇒ 与在盘态不逐字同）。
// 现把该件**逐字节**快照内嵌到 Go 侧（base64 · 解码后 == py 现读字节的逐字副本）
// ⇒ 数据来源不再依赖「件在盘」；py 在盘 / 移出仓 两态 rc/stdout/stderr 逐字同。
// ★ 快照与盘上 py 逐字节同：`gddSelfPySource()` 的 sha256 == 盘上件 sha256（得自 py 现读字节）。
// ★ 走 base64（✗ 普通字符串字面量）是为避开：① py 源里的反引号/引号/反斜杠转义坑
// ② 源文里 kind 引号形 / `os.remove(` 一类**被本仓其它门静态扫描的形态**（base64 不可见 ⇒ 不误触他门）。
const gddSelfPyB64 = "" +
	"IyEvdXNyL2Jpbi9lbnYgcHl0aG9uMwojIC0qLSBjb2Rpbmc6IHV0Zi04IC0qLQoiIiJjaGVjay1naG9zdC1kZWNsLWRpZmYucHkg" +
	"djEuMC4wIOKAlOKAlCBgRDRg77yIYEE3YCDCtyBgUS0wNTdg77yJ5bm954G1L+WjsOaYjuW3rioq5Y+q6K+7IGRpZmYqKu+8iOS4" +
	"jeiHquWKqOa4hSDCtyDkuI3oh6rliqjooaXvvIkKCuS4uuS7gOS5iOWtmOWcqO+8iGBENGAg55qE5p2h5Lu25Y6f5paHIMK3IOmA" +
	"kOWtl+WPo+W+hO+8iQotLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tCmBaZXJnLWRvY3Mv6aG555uu5paH" +
	"5qGjL3YyLjUuMTEv57y65Y+j5oC76LSmLTIwMjYwOTIyLm1kYCDCp+mZhOW9leWNgeS6lCBBIOihqOeahCBgRDRgIOihjO+8mgog" +
	"IOadoeS7tiA9IOOAjCoq5Y+q5YGa5Y+q6K+7IGRpZmbjgIHkuI3oh6rliqjmuIXlub3ngbUqKuOAje+8iOS4jiBgUS0wNTdgIOWQ" +
	"jOaJue+8ieOAggpgUS0wNTdgIOWOn+aWh++8iOacrOS7tueahOeXheague+8ie+8muOAjOeci+aMguWcqOezu+e7n+S4iueahOWu" +
	"iOaKpOS7tu+8jOS7peWPiioq44CM546w5YC85pyJ44CB5aOw5piO5peg44CN55qE5bm954G1KirjgI0KICDigJTigJQg5LuK5aSp" +
	"6Z2g5Lq66IKJIGBwbHV0aWwgLXAgfi9MaWJyYXJ5L0xhdW5jaEFnZW50cy9jb20uemVyZy5jb3JlLnBsaXN0IHwgZ3JlcCAtQTIg" +
	"UHJvZ3JhbUFyZ3VtZW50c2DvvIwKICDogIwgYHplcmcgY29yZSBkYWVtb24gbHNgIOeahCBgZGVjbGFyZWRgIOWIlyoq5YWo44CM" +
	"5ZCm44CNKirvvIjih5Ig55yL5LiN5Ye65bm954G177yJ44CCCgrmnKzku7bmiorjgIwqKuS4pOS+pyoq44CN5Y+Y5oiQ5q+P5qyh" +
	"6YO96IO9546w6LeR55qE5LiA5Lu95Y+q6K+7IGRpZmbvvJoKICDCtyDkvqfikaDjgIwqKuebmOS4iuaciSAvIOWjsOaYjuaXoCoq" +
	"44CN4oCU4oCUIOeOsOWAvOi/m+eoi+mHjO+8jOS4jeiiq+S7u+S9lSBga2luZD1sYXVuY2hkYCDlo7DmmI7nmoQqKui/m+eoi+eJ" +
	"ueW+gSoq6KaG55uW55qE5p2h77ybCiAgICDmr4/mnaEqKumineWkluagh+azqCoq5a6D5piv44CM5bey55m76K6wIGdob3N0IOih" +
	"jOOAjei/mOaYr+OAjCoq5pyq55m76K6wKirlub3ngbXjgI3vvIjlkI7ogIXmiY3mmK/pl6jikacgUjQg55qE57qi77yJ44CCCiAg" +
	"wrcg5L6n4pGh44CMKirlo7DmmI7mnIkgLyDnm5jkuIrml6AqKuOAjeKAlOKAlCDmr4/mnaEgYGtpbmQ9bGF1bmNoZGAg5aOw5piO" +
	"55qEKirkuInnjq8qKu+8iGBsYXVuY2hjdGxgIOijhei9vSDCtyBgcHNgIOi/m+eoiyDCtwogICAgYOWjsOaYjuS7tmDlnKjnm5jv" +
	"vInph4wqKue8uuWTquS4gOeOryoq77yM6YCQ5p2h54K55ZCN44CCCiAg5Lik5L6nKirpg73pgJDmnaHliJcqKu+8m+afkOS+p+S4" +
	"uuepuuS5n+imgeaJk+WNsOOAjO+8iOepuu+8iTAg5p2h44CN77yIKirkuI3orrjpnZnpu5gqKu+8ieOAggoK4piFIOWQjOa6kO+8" +
	"iOOAjCoq5LiN6K645byA56ys5LqM5Lu95Y+j5b6EKirjgI3kuI3mmK/kuIDlj6Xlj6Plj7cgwrcg5piv5Lik5p2h5Y+v5qC45pat" +
	"6KiA77yJCi0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLS0tLQogIFMxICoq" +
	"5aOw5piO5L6n55yf5rqQ5Lu2ICsg6Kej5p6Q5ZmoKirvvJrmnKzku7YgYGltcG9ydGAg6Zeo4pGnIGBzY3JpcHRzL2dhdGVzL2No" +
	"ZWNrLXNlcnZpY2UtZGVjbGFyYXRpb24ucHlgCiAgICAg55qEIGBwYXJzZV9kZWNsYCAvIGBQUk9DX0hJTlRTYCAvIGByZWFkX2xp" +
	"bmVzYO+8iGBpbXBvcnRsaWJgIOaMiei3r+W+hOWKoOi9vSDih5IgKirlkIzkuIDku73ku6PnoIEqKu+8jOS4jeaYr+aKhOS4gOmB" +
	"je+8ie+8jAogICAgIOecn+a6kOS7tiA9IGBkZXBsb3kv5pyN5Yqh5aOw5piOLnRzdmDvvIjor6Xku7boh6rlt7HnmoTooajlpLTp" +
	"gJDlrZflhpnnnYDjgIzmnKzku7bmmK/jgI7njrDlgLwgdnMg5aOw5piO44CP5beu6ZuG5Yik5a6a55qEKirlo7DmmI7kvqfllK/k" +
	"uIDnnJ/mupAqKuOAje+8ie+8jAogICAgIOS4lOS4jiBgemVyZyBjb3JlIGRhZW1vbiBsc2Ag6K+755qEKirlkIzkuIDku7YqKuOA" +
	"guWKoOi9veWksei0pSDih5IgKipyYz0yIOS4jee7mee7k+iuuioq77yI5LiN6K646YCA5YyW5oiQ6Ieq5bim5LiA5Lu977yJ44CC" +
	"CiAgUzIgKirlub3ngbXorqHmlbDkuI4gYFQyYCDnmoTjgIzkuKTliJfjgI3lkIzmupDlkIzlgLwqKu+8muS+p+KRoOadoeaVsOW/" +
	"hemhuyoq6YCQ5a2X562J5LqOKiogYHplcmcgZG9jdG9yYCDnmoQKICAgICDjgIzlm57mlLblgJnpgInvvIjlub3ngbXmnI3liqHv" +
	"vInjgI3pgqPkuIDmoLznmoTmnaHmlbDvvIjnjrDor7sgYHplcmcgZG9jdG9yIC0tanNvbiBuYW1lLHZlcmRpY3QsZGV0YWlsLGFk" +
	"dmljZWDvvIwKICAgICDmlofmnKzlvaLmgIHpgJDlrZfvvJpgMSDmnaHvvJpwaWQ9NzkzMTcg4oCmYO+8ieOAguS4jeetiSDih5Ig" +
	"KipyYz0xKirvvIjkuKTlpITlj6PlvoTlt7Lnu4/liIblj4kg4oCU4oCUIOmCo+ato+aYr+OAjOesrOS6jOS7veWPo+W+hOOAjeea" +
	"hOW9ouaAge+8ieOAggoK5Yik5o2u77yI5Y+v5py65qOAIMK3IOavj+adoemDveiDveWNleeLrOWksei0pe+8iQotLS0tLS0tLS0t" +
	"LS0tLS0tLS0tLS0tLS0tLS0tLS0tLQogIEQxIOS4pOS+p+mDveWHuu+8iOWQhOiHquacieiuoeaVsCArIOmAkOadoe+8ie+8jOe8" +
	"uuWjsOaYjuS7tiAvIOino+aekOS4jeWHuiAvIHBzIOaIliBsYXVuY2hjdGwg5Y+W5LiN5YiwIC8gKirnqbrovawqKiDih5IgcmM9" +
	"MuOAggogIEQyICoq5LiN5bim5YaZ5peX5qCHIOKHkiDku5PlhoXku7YgYHNoYTI1NmAg5LiN5Y+YKirvvJpgLS1wcm92ZS1yZWFk" +
	"b25seWAg5a+55ouN44CM5aOw5piO55yf5rqQ5Lu2ICsg6Zeo4pGnIOa6kOS7tiArIOacrOS7tuiHqui6qwogICAgICsgYGdpdCBz" +
	"dGF0dXMgLS1wb3JjZWxhaW5gIOWFqOaWh+aRmOimgSArIGAuZ2l0YCDlhYPmlbDmja7pnaLjgI3pgJDku7YgYHNoYTI1NmAg4oeS" +
	"IOW3ruW8giAwIOS7tiDih5IgcmM9MOOAggogIEQzICoq6LSf5o6nIMK3IOS7u+S9leiHquWKqOWIoCAvIOiHquWKqOaUuSDih5Ig" +
	"5Yik57qiKirvvIzkuKTpgZPniZnvvJoKICAgICDikaAgKirov5DooYzml7YqKu+8muS7u+S9leWGmeaXl+agh++8iGAtLXJlYXBg" +
	"IC8gYC0tY2xlYW5gIC8gYC0tZml4YCAvIGAtLWF1dG9gIC8gYC0tcHJ1bmVgIC8gYC0tcm1gIC8gYC0tYXBwbHlgIC8KICAgICAg" +
	"ICBgLS13cml0ZWAgLyBgLS1kZWxldGVgIC8gYC0tc3RvcGAgLyBgLS1raWxsYCAvIGAtLXVubG9hZGAg4oCmIOS7peWPiiBgYWRk" +
	"YC9gY29tbWl0YC9gcHVzaGAg5LiA5peP77yJ4oeSICoqcmM9MiDmi5IqKu+8mwogICAgIOKRoSAqKua6kOeggee6pyoq77yIYGFz" +
	"c2VydF9ub193cml0ZV9hcGlg77yJ77ya5omr5pys5Lu2KirnnJ/nm67moIfot6/lvoTpgqPkuIDmrrUqKu+8iOiHquajgOagh+iu" +
	"sOS5i+WJje+8ieKAlOKAlOWHuueOsAogICAgICAgIGBvcy5yZW1vdmVgIC8gYG9zLnVubGlua2AgLyBgb3Mucm1kaXJgIC8gYHNo" +
	"dXRpbC5ybXRyZWVgIC8gYHNodXRpbC5tb3ZlYCAvIGBvcy5yZW5hbWVgIC8KICAgICAgICBgb3BlbijigKYsICd3J3wnYScpYCAv" +
	"IGBzdWJwcm9jZXNzYCDph4znmoQgZ2l0IOWGmeWtkOWRveS7pCDih5IgKipyYz0xIOWIpOe6oioq77yI6Ieq5qOA5ZaC5ZCI5oiQ" +
	"5rqQ56CB5q2j6LSf5oiQ5a+577yJ44CCCiAgICAgICAg4piFIOiHquajgOaute+8iOWQiOaIkOWkueWFt++8ieixgeWFjSDigJTi" +
	"gJQg5a6D5Y+q5YaZIGAkVE1QRElSYCDkuIvnmoTlpLnlhbfvvIzkuI3norDnnJ/nm67moIfjgIIKCumAgOeggQotLS0tCiAgMCA9" +
	"IOS4pOS+p+W3sumAkOadoeWHuiArIOWQjOa6kOaWreiogOi/h++8iCoq5beu6ZuG5pys6Lqr5LiN6amx5Yqo6YCA56CBKirvvJrm" +
	"nKzku7bmmK/lj6ror7sgZGlmZu+8jOaKpeW3ruWwseaYr+Wug+eahOa0u++8iQogIDEgPSDlkIzmupDmvILnp7vvvIhTMiDkuI3n" +
	"rYnvvIkvIOa6kOeggee6p+WGmSBBUEkg5ZG95LitIC8g5Y+q6K+76Ieq6K+B57qiCiAgMiA9IOS4jee7mee7k+iuuu+8iOWjsOaY" +
	"jue8uuS7tsK36Kej5p6Q5LiN5Ye6wrdwcy9sYXVuY2hjdGwvZG9jdG9yIOWPluS4jeWIsMK356m66L2swrflhpnml5fmoIfCt+iH" +
	"quajgOS4jei/h++8iQogIOKYhSA4IOS4jeWHuueOsO+8muacrOS7tioq5LiN5Yik44CM5bm954G16K+l5LiN6K+l5p2A44CNKirv" +
	"vIjlgZwgPSDkuI3lj6/pgIbliqjkvZwg4oeSIOmCo+aYryBNcjIxMDkg55qE5ouN5p2/77yM54WnIMKn5LmdIE05IOWPo+W+hO+8" +
	"iQoK55So5rOVCi0tLS0KICAgIHB5dGhvbjMgc2NyaXB0cy9nYXRlcy9jaGVjay1naG9zdC1kZWNsLWRpZmYucHkgICAgICAgICAg" +
	"ICAgICMg55yf55uu5qCH77ya546w5YC877yIcHMvbGF1bmNoY3Rs77yJdnMg5aOw5piO77yIZGVwbG95L+acjeWKoeWjsOaYji50" +
	"c3bvvIkKICAgIHB5dGhvbjMgc2NyaXB0cy9nYXRlcy9jaGVjay1naG9zdC1kZWNsLWRpZmYucHkgLS1qc29uICAgICAgICMg5py6" +
	"6K+777yIemVyZy92MSDlvaLnirbvvIkKICAgIHB5dGhvbjMgc2NyaXB0cy9nYXRlcy9jaGVjay1naG9zdC1kZWNsLWRpZmYucHkg" +
	"LS1wcm92ZS1yZWFkb25seSAgICMg5Y+q6K+76Ieq6K+B77yI6LeR5YmN5ZCO6YCQ5Lu2IHNoYTI1Nu+8iQogICAgcHl0aG9uMyBz" +
	"Y3JpcHRzL2dhdGVzL2NoZWNrLWdob3N0LWRlY2wtZGlmZi5weSAtLWxpc3QtcnVsZXMKICAgIHB5dGhvbjMgc2NyaXB0cy9nYXRl" +
	"cy9jaGVjay1naG9zdC1kZWNsLWRpZmYucHkgLS1zZWxmLXRlc3QgICMg5Y+q6LeR6Ieq5qOA77yI5ZCI5oiQ5aS55YW3IMK3IOS4" +
	"jeeisOecn+ebruagh++8iQrlj5bor4HlpLnlhbfvvIjoh6rmo4DnlKjvvJvkuZ/kvr/kuo7lnKjliKvnmoTmnLrlmajkuIrlpI3o" +
	"t5EgwrcgKirkuI7pl6jikacg5ZCM5ZCN5ZCM5LmJKirvvInvvJoKICAgIFpFUkdfU1ZDREVDTF9ERUNMIC8gWkVSR19TVkNERUNM" +
	"X1BTIC8gWkVSR19TVkNERUNMX0xBVU5DSENUTCAvIFpFUkdfU1ZDREVDTF9IT01FICsg5pys5Lu255qEIC0tZG9jdG9yCgrimIUg" +
	"5o6l57q/77yIKirkuI3lvZLmnKzku7YqKiDCtyDms6LikaMg5pS25Y+j77yJ77ya5pys5Lu25LuK5aSpKirmsqHmnIkqKuaMgui/" +
	"myBgc2NyaXB0cy9nYXRlcy9wcmVjb21taXQtZ2F0ZXMuc2hg44CCCiAg5rOi4pGjIOmcgOimgeeahOmCo+S4gOihjO+8iOeFpyBg" +
	"c2NyaXB0cy/lhazlvIDmoIforrAudHN2YCDnmoTliJfvvJrmlofku7YgLyBwdWJsaWMgLyDnkIbnlLHvvInvvJoKICAgIHNjcmlw" +
	"dHMvZ2F0ZXMvY2hlY2stZ2hvc3QtZGVjbC1kaWZmLnB5CW5vCUIg57G7wrdgRDRgIOW5veeBtS/lo7DmmI7lt67lj6ror7sgZGlm" +
	"ZiDku7bvvIhgQTdgwrdgUS0wNTdg77yJ77ya5Lik5L6n6YCQ5p2h77yI55uY5LiK5pyJwrflo7DmmI7ml6AgLyDlo7DmmI7mnInC" +
	"t+ebmOS4iuaXoO+8iSsg5LiOIGB6ZXJnIGRvY3RvcmAg5bm954G15p2h5pWw5ZCM5rqQ5ZCM5YC8ICsg5Lu75L2V6Ieq5Yqo5Yig" +
	"L+iHquWKqOaUueWIpOe6ou+8m+WIpOaNrue7keengeaciemdou+8iOivuyBgZGVwbG95L+acjeWKoeWjsOaYji50c3ZgIMK3IGBw" +
	"c2AvYGxhdW5jaGN0bGAgwrcgYGJpbi96ZXJnYO+8ieKHkiDkuI3lvIDmupAKICDimIUg5o6l57q/5pe2KirliKvmiorlroPmjILm" +
	"iJDkvJrlhpnnmoTkuJzopb8qKu+8muacrOS7tumAgOeggSAwLzEvMiDkuInmoaPvvIzmjIIgYHRyaWAg5Y2z5Y+v77yIMD1QQVNT" +
	"IMK3IDE9RkFJTCDCtyAyPUJMT0NLRUTvvInjgIIKIiIiCgppbXBvcnQgYXJncGFyc2UKaW1wb3J0IGFzdAppbXBvcnQgaGFzaGxp" +
	"YgppbXBvcnQgaW1wb3J0bGliLnV0aWwKaW1wb3J0IGpzb24KaW1wb3J0IG9zCmltcG9ydCByZQppbXBvcnQgc3VicHJvY2Vzcwpp" +
	"bXBvcnQgc3lzCmltcG9ydCB0ZW1wZmlsZQoKSEVSRSA9IG9zLnBhdGguZGlybmFtZShvcy5wYXRoLmFic3BhdGgoX19maWxlX18p" +
	"KQpERUZBVUxUX1JPT1QgPSBvcy5wYXRoLmRpcm5hbWUob3MucGF0aC5kaXJuYW1lKEhFUkUpKQpWRVJTSU9OID0gImNoZWNrLWdo" +
	"b3N0LWRlY2wtZGlmZi5weSB2MS4wLjAiClNFTEZfTkFNRSA9IG9zLnBhdGguYmFzZW5hbWUoX19maWxlX18pClNJQkxJTkcgPSBv" +
	"cy5wYXRoLmpvaW4oSEVSRSwgImNoZWNrLXNlcnZpY2UtZGVjbGFyYXRpb24ucHkiKSAgICAgICAgICAjIOmXqOKRpyA9IOWQjOa6" +
	"kOeahOmCo+S4gOS7vQpET0NUT1JfSVRFTSA9ICLlm57mlLblgJnpgInvvIjlub3ngbXmnI3liqHvvIkiICAgICAgICAgICAgICAg" +
	"ICAgICAgICAgICAgICAgICAgICAgICMgYFQyYCDliKTmja7ikaIg55qE5ZCM5LiA5qC877yI6YCQ5a2X77yJCkRPQ1RPUl9GSUVM" +
	"RFMgPSAibmFtZSx2ZXJkaWN0LGRldGFpbCxhZHZpY2UiCiMg4piFIOaLvOaOpeiAjOaIkO+8mioq5Yir5oqK5a6D5YaZ5oiQ5LiA" +
	"5Liq5a2X6Z2i6YePKiog4oCU4oCUIOWQpuWImSBgc3JjLnNwbGl0KG1hcmtlcilbMF1gIOS8muWcqOOAjOW4uOmHj+WumuS5iemC" +
	"o+S4gOihjOOAjeWwseWIh+aOie+8jAojICAg5ZCO6Z2i55yf55uu5qCH5q6155qE54mZ5bCx5b2i5ZCM6Jma6K6+77yI6Ieq5qOA" +
	"55So5L6LIOKRo2Iv4pGjYyDkuIDotbfmiorov5nkuIDngrnpkonkvY/vvInjgIIKU0VMRlRFU1RfTUFSS0VSID0gIiMg4pSA4pSA" +
	"4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSAICIgKyAi6Ieq5qOAIiAgICAgICAgICAg" +
	"ICAgICAgICAjIOa6kOeggee6p+eJmeeahOaJq+aPj+i+ueeVjO+8iOagh+iusOS5i+WQjiA9IOWQiOaIkOWkueWFt+aute+8iQoK" +
	"IyDilIDilIAg5YaZ5peX5qCH6Zet6ZuG77yI6LSf5o6nIMK3IOWUr+S4gOecn+a6kO+8ieKUgOKUgOKUgOKUgOKUgOKUgOKUgOKU" +
	"gOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKU" +
	"gOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgApXUklURV9XT1JEUyA9ICgKICAgICJhZGQiLCAiY29t" +
	"bWl0IiwgInB1c2giLCAic3RhZ2UiLCAiYW1lbmQiLCAidGFnIiwgImZldGNoIiwgInB1bGwiLCAiY2xvbmUiLAogICAgIm1lcmdl" +
	"IiwgInJlYmFzZSIsICJyZXNldCIsICJyZXZlcnQiLCAiY2hlY2tvdXQiLCAicmVzdG9yZSIsICJjbGVhbiIsICJybSIsCiAgICAi" +
	"bXYiLCAiYXBwbHkiLCAic3Rhc2giLCAiZ2MiLCAicHJ1bmUiLCAicmVwYWNrIiwgInVubG9jayIsCiAgICAicmVhcCIsICJyZWFw" +
	"LXJ1biIsICJraWxsIiwgInN0b3AiLCAidW5sb2FkIiwgInVubGluayIsICJkZWwiLCAiZGVsZXRlIiwKICAgICJmaXgiLCAiYXV0" +
	"byIsICJ3cml0ZSIsICJyZW5ldyIsICJyZXN0YXJ0IiwgImJvb3RvdXQiLCAiYm9vdHN0cmFwIiwKKQojIOKUgOKUgCDmupDnoIHn" +
	"uqfniZnnmoTor43ooajvvIgqKkFTVCDliKQqKiDCtyDkuI3mmK/lrZfnrKbkuLLmiavmj48g4oCU4oCUIOeQhueUseingSBhc3Nl" +
	"cnRfbm9fd3JpdGVfYXBpIOeahOazqOmHiu+8ieKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgApERUxFVEVf" +
	"Q0FMTFMgPSAoInJlbW92ZSIsICJ1bmxpbmsiLCAicm10cmVlIiwgInJtZGlyIiwgInJlbmFtZSIsICJtb3ZlIiwgInRydW5jYXRl" +
	"IiwgInJlcGxhY2UiKQpHSVRfV1JJVEVfU1VCUyA9ICgiYWRkIiwgImNvbW1pdCIsICJwdXNoIiwgInJtIiwgIm12IiwgImNoZWNr" +
	"b3V0IiwgInJlc3RvcmUiLCAicmVzZXQiLCAiY2xlYW4iLAogICAgICAgICAgICAgICAgICAidXBkYXRlLWluZGV4IiwgIndyaXRl" +
	"LXRyZWUiLCAic3Rhc2giLCAicmViYXNlIiwgImZldGNoIiwgInB1bGwiLCAidGFnIikKClRPT0xfRElSID0gTm9uZSAgICAgIyDn" +
	"lLEgbG9hZF9zaWJsaW5nKCkg5aGrCiMg5aeK5aa55Lu255yf6LqrIO+8iyDlhbYgR28g5Y2K77yI6YCA5b255a655b+N5Lik5Y2K" +
	"5Yik5o2uIMK3IOWQjOaXjyBgY2hlY2std3JpdGUtZmFjZS1nYXRlLnB5YCDlj6PlvoTvvInjgIIKQ1NEX1BZID0gU0lCTElORyAg" +
	"ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAjIOmXqOKRpyBweSDljYrvvIjlkIzm" +
	"upDnnJ/ouqvvvIkKQ1NEX0dPID0gb3MucGF0aC5qb2luKERFRkFVTFRfUk9PVCwgImNvcmUiLCAiY21kIiwgInplcmciLCAiY2hl" +
	"Y2tfc2VydmljZV9kZWNsYXJhdGlvbi5nbyIpCgoKIyDilIDilIAg5YaF5YyW5Ymv5pys77yI6YCA5b255oCB5Zue6JC96Z2iIMK3" +
	"IOmAkOWtl+WQjOaehOiHqiBgY2hlY2stc2VydmljZS1kZWNsYXJhdGlvbi5weWDvvInilIDilIDilIDilIDilIDilIDilIDilIDi" +
	"lIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIAKIyDimIUgMjAyNi0xMC0wNyDlp4rlprnk" +
	"vp3otZbpgIDlvbnlrrnlv43vvIhgY2hlY2stc2VydmljZS1kZWNsYXJhdGlvbi5weWAg5b2T5pel6YCA5b25IMK3IE0yIOeUsue7" +
	"hO+8ie+8muacrOS7tuWOnyBpbXBvcnRsaWIg5pyfCiMgICAqKuehrOi9vSoq6Zeo4pGnIHB5IOWNiu+8jOWug+mAgOWcuuWQjiBy" +
	"Yz0x77yI6Ieq5qOA5q616YCQ5a2X44CM5ZCM5rqQ5Lu25Yqg6L295aSx6LSl44CN77yJ44CC546w54Wn5ZCM5peP5Y+j5b6E77ya" +
	"5aeK5aa5IHB5IOWcqOebmCDih5Ig54Wn5penCiMgICDliqDovb3vvJvpgIDlnLog4oinIOWFtiAqKkdvIOWNiioqIGBjb3JlL2Nt" +
	"ZC96ZXJnL2NoZWNrX3NlcnZpY2VfZGVjbGFyYXRpb24uZ29gIOWcqOebmCDih5Ig55So5pys5Lu25YaF5YyW5Ymv5pys77yI5Y+q" +
	"5YaF5YyWCiMgICDmnKzku7bnnJ/nlKjnmoTlm5vkuKrlkIzmupDpnaIgwrcg6YCQ5a2X5ZCM5p6EIMK3IOKclyDkuI3mkKzmlbTk" +
	"uKrlp4rlprnku7bjgIHinJcg5LiN5Y+m5YaZ56ys5LqM5Lu95Yik5a6a5Y+j5b6EIMK3IOWQjOaXjyBgX2N3YV9zY2FuX2ZpbGVg" +
	"CiMgICDlhYjkvovvvInvvJvkuKTljYrnmobnvLog4oeSIGZhaWwtY2xvc2Vk77yIcmM9MiDkuI3nu5nnu5PorrrvvInjgIIKX0NT" +
	"RF9LSU5EUyA9ICgnbGF1bmNoZCcsICdnaG9zdCcpCl9DU0RfUFJPQ19ISU5UUyA9ICgnYmluL3plcmctJywgJ2Jpbi9jb2Nvb24t" +
	"JywgJ3NlYXJ4LndlYmFwcCcpCl9DU0RfWkVSR19MQUJFTF9QUkVGSVhFUyA9ICgnY29tLnplcmcuJywpCgoKZGVmIF9jc2RfcGFy" +
	"c2VfZGVjbChwYXRoKToKICAgICIiIuWGheWMliA9IGBjaGVjay1zZXJ2aWNlLWRlY2xhcmF0aW9uLnB5OjpwYXJzZV9kZWNsYO+8" +
	"iOmAgOW9ueaAgeecn+a6kOWbnuiQvSDCtyDpgJDlrZflkIzmnoTvvInjgIIiIiIKICAgIHJvd3MsIHByb2JsZW1zID0gW10sIFtd" +
	"CiAgICBpZiBub3Qgb3MucGF0aC5pc2ZpbGUocGF0aCk6CiAgICAgICAgcmV0dXJuIE5vbmUsIFsn57y65Lu277ya5aOw5piO5qCR" +
	"5LiN5a2Y5ZyoICVz77yI5Yik5o2u5LiN5Y+v5YikIOKHkiDkuI3nu5nnu5PorrrvvIknICUgcGF0aF0KICAgIHdpdGggb3Blbihw" +
	"YXRoLCBlbmNvZGluZz0ndXRmLTgnKSBhcyBmOgogICAgICAgIGZvciBsbiwgbGluZSBpbiBlbnVtZXJhdGUoZiwgMSk6CiAgICAg" +
	"ICAgICAgIGxpbmUgPSBsaW5lLnJzdHJpcCgnXG4nKQogICAgICAgICAgICBpZiBub3QgbGluZS5zdHJpcCgpIG9yIGxpbmUubHN0" +
	"cmlwKCkuc3RhcnRzd2l0aCgnIycpOgogICAgICAgICAgICAgICAgY29udGludWUKICAgICAgICAgICAgaWYgbGluZS5zdGFydHN3" +
	"aXRoKCdraW5kXHQnKTogICAgICAgICAgIyDooajlpLQKICAgICAgICAgICAgICAgIGNvbnRpbnVlCiAgICAgICAgICAgIGNlbGxz" +
	"ID0gbGluZS5zcGxpdCgnXHQnKQogICAgICAgICAgICBpZiBsZW4oY2VsbHMpIDwgNzoKICAgICAgICAgICAgICAgIHByb2JsZW1z" +
	"LmFwcGVuZCgn6KGMICVkIOWIl+aVsOS4jei2s++8iOimgSA3IOWIl++8jOW+l+WIsCAlZO+8ie+8miVzJyAlIChsbiwgbGVuKGNl" +
	"bGxzKSwgbGluZVs6NjBdKSkKICAgICAgICAgICAgICAgIGNvbnRpbnVlCiAgICAgICAgICAgIGtpbmQgPSBjZWxsc1swXS5zdHJp" +
	"cCgpCiAgICAgICAgICAgIGlmIGtpbmQgbm90IGluIF9DU0RfS0lORFM6CiAgICAgICAgICAgICAgICBwcm9ibGVtcy5hcHBlbmQo" +
	"J+ihjCAlZCDnmoQga2luZD0lciDkuI3lnKjpl63pm4YgJXMg5YaFJyAlIChsbiwga2luZCwgbGlzdChfQ1NEX0tJTkRTKSkpCiAg" +
	"ICAgICAgICAgICAgICBjb250aW51ZQogICAgICAgICAgICByb3dzLmFwcGVuZCh7J2xpbmUnOiBsbiwgJ2tpbmQnOiBraW5kLCAn" +
	"bmFtZSc6IGNlbGxzWzFdLnN0cmlwKCksCiAgICAgICAgICAgICAgICAgICAgICAgICAnZGVjbCc6IGNlbGxzWzJdLnN0cmlwKCks" +
	"ICdwcm9nJzogY2VsbHNbM10uc3RyaXAoKSwKICAgICAgICAgICAgICAgICAgICAgICAgICdwcm9jJzogY2VsbHNbNF0uc3RyaXAo" +
	"KSwgJ293bmVyJzogY2VsbHNbNV0uc3RyaXAoKSwKICAgICAgICAgICAgICAgICAgICAgICAgICdkaXNwb3NpdGlvbic6IGNlbGxz" +
	"WzZdLnN0cmlwKCksCiAgICAgICAgICAgICAgICAgICAgICAgICAnbGF5ZXInOiBjZWxsc1s3XS5zdHJpcCgpIGlmIGxlbihjZWxs" +
	"cykgPj0gOCBlbHNlICcnfSkKICAgIHJldHVybiByb3dzLCBwcm9ibGVtcwoKCmRlZiBfY3NkX3JlYWRfbGluZXMocGF0aCwgY21k" +
	"KToKICAgICIiIuWGheWMliA9IGBjaGVjay1zZXJ2aWNlLWRlY2xhcmF0aW9uLnB5OjpyZWFkX2xpbmVzYO+8iOmAkOWtl+WQjOae" +
	"hO+8ieOAgiIiIgogICAgaWYgcGF0aDoKICAgICAgICBpZiBub3Qgb3MucGF0aC5pc2ZpbGUocGF0aCk6CiAgICAgICAgICAgIHJl" +
	"dHVybiBbXSwgJ2ZpeHR1cmUnLCAn5aS55YW35paH5Lu25LiN5a2Y5Zyo77yaJXMnICUgcGF0aAogICAgICAgIHdpdGggb3Blbihw" +
	"YXRoLCBlbmNvZGluZz0ndXRmLTgnKSBhcyBmOgogICAgICAgICAgICByZXR1cm4gW2wucnN0cmlwKCdcbicpIGZvciBsIGluIGYg" +
	"aWYgbC5zdHJpcCgpXSwgJ2ZpeHR1cmUnLCBOb25lCiAgICB0cnk6CiAgICAgICAgb3V0ID0gc3VicHJvY2Vzcy5ydW4oY21kLCBz" +
	"aGVsbD1UcnVlLCBjYXB0dXJlX291dHB1dD1UcnVlLCB0ZXh0PVRydWUsIHRpbWVvdXQ9MzApCiAgICBleGNlcHQgRXhjZXB0aW9u" +
	"IGFzIGU6ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIyBub3FhOiBCTEUwMDEKICAgICAgICByZXR1cm4gW10s" +
	"ICdsaXZlJywgJyVzIOi3keS4jei1t+adpe+8miVzJyAlIChjbWQsIGUpCiAgICBpZiBvdXQucmV0dXJuY29kZSAhPSAwOgogICAg" +
	"ICAgIHJldHVybiBbXSwgJ2xpdmUnLCAnJXMg6YCA5Ye656CBICVk77yaJXMnICUgKGNtZCwgb3V0LnJldHVybmNvZGUsIChvdXQu" +
	"c3RkZXJyIG9yICcnKS5zdHJpcCgpWzoxMjBdKQogICAgcmV0dXJuIFtsIGZvciBsIGluIG91dC5zdGRvdXQuc3BsaXRsaW5lcygp" +
	"IGlmIGwuc3RyaXAoKV0sICdsaXZlJywgTm9uZQoKCmRlZiBfY3NkX3BhcnNlX2xhdW5jaGN0bF9sYWJlbHMobGluZXMpOgogICAg" +
	"IiIi5YaF5YyWID0gYGNoZWNrLXNlcnZpY2UtZGVjbGFyYXRpb24ucHk6OnBhcnNlX2xhdW5jaGN0bF9sYWJlbHNg77yI6YCQ5a2X" +
	"5ZCM5p6EIMK3IOengeacieWKqeaJi++8ieOAgiIiIgogICAgb3V0ID0ge30KICAgIGZvciBsbiBpbiBsaW5lczoKICAgICAgICBj" +
	"ZWxscyA9IGxuLnNwbGl0KCdcdCcpCiAgICAgICAgaWYgbGVuKGNlbGxzKSA8IDM6CiAgICAgICAgICAgIGNvbnRpbnVlCiAgICAg" +
	"ICAgbGFiID0gY2VsbHNbMl0uc3RyaXAoKQogICAgICAgIGlmIGxhYjoKICAgICAgICAgICAgb3V0LnNldGRlZmF1bHQobGFiLCBs" +
	"bi5zdHJpcCgpKQogICAgcmV0dXJuIG91dAoKCmRlZiBfY3NkX2xhYmVsX3NldF9kaWZmKGxhdW5jaGRfcm93cywgbGNfbGluZXMp" +
	"OgogICAgIiIi5YaF5YyWID0gYGNoZWNrLXNlcnZpY2UtZGVjbGFyYXRpb24ucHk6OmxhYmVsX3NldF9kaWZmYO+8iOmAkOWtl+WQ" +
	"jOaehCDCtyDnp4HmnInliqnmiYvlhoXogZTvvInjgIIiIiIKICAgIGRlY2xfbGFiZWxzID0ge3JbJ25hbWUnXTogciBmb3IgciBp" +
	"biBsYXVuY2hkX3Jvd3N9CiAgICBsaXZlX2xhYmVscyA9IHtsYWI6IHJhdyBmb3IgbGFiLCByYXcgaW4gX2NzZF9wYXJzZV9sYXVu" +
	"Y2hjdGxfbGFiZWxzKGxjX2xpbmVzKS5pdGVtcygpCiAgICAgICAgICAgICAgICAgICBpZiBsYWIuc3RhcnRzd2l0aChfQ1NEX1pF" +
	"UkdfTEFCRUxfUFJFRklYRVMpfQogICAgb25seV9saXZlID0gc29ydGVkKHNldChsaXZlX2xhYmVscykgLSBzZXQoZGVjbF9sYWJl" +
	"bHMpKQogICAgb25seV9kZWNsID0gc29ydGVkKHNldChkZWNsX2xhYmVscykgLSBzZXQobGl2ZV9sYWJlbHMpKQogICAgcmV0dXJu" +
	"IGxpdmVfbGFiZWxzLCBkZWNsX2xhYmVscywgb25seV9saXZlLCBvbmx5X2RlY2wKCgpjbGFzcyBfU2libGluZ0ludGVybmFsaXpl" +
	"ZDoKICAgICIiIumAgOW9ueaAgeWGheWMluWJr+acrO+8iOWPquWQq+acrOS7tuecn+eUqOeahOWbm+S4quWQjOa6kOmdou+8muin" +
	"o+aekOWZqCDvvIsg6L+b56iL54m55b6BIO+8iyDor7vooYwg77yLIOagh+etvumbhuWQiOavlOi+g++8ieOAgiIiIgogICAgUFJP" +
	"Q19ISU5UUyA9IF9DU0RfUFJPQ19ISU5UUwogICAgcGFyc2VfZGVjbCA9IHN0YXRpY21ldGhvZChfY3NkX3BhcnNlX2RlY2wpCiAg" +
	"ICByZWFkX2xpbmVzID0gc3RhdGljbWV0aG9kKF9jc2RfcmVhZF9saW5lcykKICAgIGxhYmVsX3NldF9kaWZmID0gc3RhdGljbWV0" +
	"aG9kKF9jc2RfbGFiZWxfc2V0X2RpZmYpCgoKIyDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDi" +
	"lIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIAg5ZCM5rqQ77ya5oqK6Zeo4pGnIOW9k+aooeWdl+WKoOi9ve+8iOS4" +
	"jeiuuOaKhOesrOS6jOS7veino+aekOWZqO+8ieKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKU" +
	"gOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgOKUgApkZWYgbG9hZF9zaWJsaW5nKHBhdGg9U0lCTElORyk6CiAgICAiIiLm" +
	"jInot6/lvoTliqDovb3pl6jikacg4oeSIChtb2R1bGUsIGVycinjgIIqKuWKoOi9veS4jeS6huWwseS4jee7mee7k+iuuioq77yI" +
	"5LiN6K646YCA5YyW5oiQ6Ieq5bim5LiA5Lu95Y+j5b6E77yJ44CCCiAgICDimIUg6YCA5b255a655b+N77yI5ZCM5pePIGBjaGVj" +
	"ay13cml0ZS1mYWNlLWdhdGUucHlgIOWPo+W+hO+8ie+8muWniuWmuSBweSDpgIDlvbkg4oinIOWFtiBHbyDljYrlnKjnm5gg4oeS" +
	"IOeUqOWGheWMluWJr+acrAogICAgICBgX1NpYmxpbmdJbnRlcm5hbGl6ZWRg77yI4pyXIOS4jeWboOe8uiBweSDogIwgZmFpbC1j" +
	"bG9zZWTvvInvvJvkuKTljYrnmobnvLog4oeSIOWOn+OAjOS4jee7mee7k+iuuuOAjeaho+OAgiIiIgogICAgZ2xvYmFsIFRPT0xf" +
	"RElSCiAgICBpZiBub3Qgb3MucGF0aC5pc2ZpbGUocGF0aCk6CiAgICAgICAgaWYgb3MucGF0aC5pc2ZpbGUoQ1NEX0dPKToKICAg" +
	"ICAgICAgICAgaWYgdHVwbGUoU0lCTElOR19ISU5UUykgIT0gdHVwbGUoX0NTRF9QUk9DX0hJTlRTKToKICAgICAgICAgICAgICAg" +
	"IHJldHVybiBOb25lLCAoIuWQjOa6kOa8guenu++8muacrOS7tiBgU0lCTElOR19ISU5UU2A9JXIg4omgIOWGheWMluWJr+acrCBg" +
	"UFJPQ19ISU5UU2A9JXIg4oeSIOS4pOS7vei/m+eoi+eJueW+gSIKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIuWPo+W+" +
	"hOW3suWIhuWPie+8iGBHQVAtMjAyNjA5MjctMTA3YO+8muWQjOaXj+S7tuS4jeiuuOWGjeWGmeesrOS6jOS7ve+8iSIKICAgICAg" +
	"ICAgICAgICAgICAgICAgICAgICAgICAgJSAodHVwbGUoU0lCTElOR19ISU5UUyksIHR1cGxlKF9DU0RfUFJPQ19ISU5UUykpKQog" +
	"ICAgICAgICAgICBUT09MX0RJUiA9IEhFUkUKICAgICAgICAgICAgcmV0dXJuIF9TaWJsaW5nSW50ZXJuYWxpemVkLCBOb25lCiAg" +
	"ICAgICAgcmV0dXJuIE5vbmUsICgi5ZCM5rqQ5Lu257y65aSx77yaJXPvvIjpl6jikacgPSDlo7DmmI7kvqfnnJ/mupDnmoTllK/k" +
	"uIDop6PmnpDlmajvvInkuJTlhbYgR28g5Y2KICIKICAgICAgICAgICAgICAgICAgICAgICJjb3JlL2NtZC96ZXJnL2NoZWNrX3Nl" +
	"cnZpY2VfZGVjbGFyYXRpb24uZ28g5LiN5Zyo55uYIOKHkiDkuI3nu5nnu5PorroiICUgcGF0aCkKICAgIHRyeToKICAgICAgICBz" +
	"cGVjID0gaW1wb3J0bGliLnV0aWwuc3BlY19mcm9tX2ZpbGVfbG9jYXRpb24oInN2Y2RlY2xfc3JjIiwgcGF0aCkKICAgICAgICBt" +
	"b2QgPSBpbXBvcnRsaWIudXRpbC5tb2R1bGVfZnJvbV9zcGVjKHNwZWMpCiAgICAgICAgc3BlYy5sb2FkZXIuZXhlY19tb2R1bGUo" +
	"bW9kKSAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIyDpobblsYLlj6rmnIkgZGVmL+W4uOmHj++8jOWGmemdouWcqCBfX21h" +
	"aW5fXyDlrojljavlkI4KICAgIGV4Y2VwdCBFeGNlcHRpb24gYXMgZTogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg" +
	"ICAgICAjIG5vcWE6IEJMRTAwMQogICAgICAgIHJldHVybiBOb25lLCAi5ZCM5rqQ5Lu25Yqg6L295aSx6LSl77yaJXPvvIglc++8" +
	"ieKHkiDkuI3nu5nnu5PorroiICUgKHBhdGgsIGUpCiAgICBmb3IgYXR0ciBpbiAoInBhcnNlX2RlY2wiLCAiUFJPQ19ISU5UUyIs" +
	"ICJyZWFkX2xpbmVzIiwgImxhYmVsX3NldF9kaWZmIik6CiAgICAgICAgaWYgbm90IGhhc2F0dHIobW9kLCBhdHRyKToKICAgICAg" +
	"ICAgICAgcmV0dXJuIE5vbmUsICLlkIzmupDku7bph4zmsqHmnIkgJXMg4oeSIOWPo+W+hOWvueS4jeS4iu+8iOS4jee7mee7k+iu" +
	"uu+8iSIgJSBhdHRyCiAgICAjIOKYhSBgR0FQLTIwMjYwOTI3LTEwN2DvvJrmnKzku7YgYFNJQkxJTkdfSElOVFNgIOaYr+mXqOKR" +
	"pyBgUFJPQ19ISU5UU2Ag55qE56ys5LqM5Lu95Ymv5pysIOKAlOKAlCDliqAqKuWQjOa6kOaWreiogCoqCiAgICAjICAg5oqK5a6D" +
	"6ZKJ5q2777yI5YC85LiN562JIOKHkiDkuI3nu5nnu5PorrrvvInvvIzmnZznu53kuKTku73ov5vnqIvnibnlvoHlj6PlvoTlkITo" +
	"h6rmvILnp7vvvIjinJcg5LiN6Z2Z6buY5ZCE55So5ZCE55qE77yJ44CCCiAgICBpZiB0dXBsZShTSUJMSU5HX0hJTlRTKSAhPSB0" +
	"dXBsZShtb2QuUFJPQ19ISU5UUyk6CiAgICAgICAgcmV0dXJuIE5vbmUsICgi5ZCM5rqQ5ryC56e777ya5pys5Lu2IGBTSUJMSU5H" +
	"X0hJTlRTYD0lciDiiaAg6Zeo4pGnIGBQUk9DX0hJTlRTYD0lciDih5Ig5Lik5Lu96L+b56iL54m55b6B5Y+j5b6E5bey5YiG5Y+J" +
	"IgogICAgICAgICAgICAgICAgICAgICAgIu+8iGBHQVAtMjAyNjA5MjctMTA3YO+8muWQjOaXj+S7tuS4jeiuuOWGjeWGmeesrOS6" +
	"jOS7ve+8iSIgJSAodHVwbGUoU0lCTElOR19ISU5UUyksIHR1cGxlKG1vZC5QUk9DX0hJTlRTKSkpCiAgICBUT09MX0RJUiA9IG9z" +
	"LnBhdGguZGlybmFtZShvcy5wYXRoLmFic3BhdGgocGF0aCkpCiAgICByZXR1cm4gbW9kLCBOb25lCgoKIyDilIDilIDilIDilIDi" +
	"lIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDi" +
	"lIDilIAg5rqQ56CB57qn54mZ77yIRDPikaHvvInilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDi" +
	"lIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIAKZGVmIGFzc2VydF9ub193cml0ZV9hcGkoc3Jj" +
	"LCBjdXQ9Tm9uZSk6CiAgICAiIiLmiavmupDnoIHph4znmoQqKuWGmeiwg+eUqCoqIOKHkiDlkb3kuK3liJfooajjgIJgY3V0YCDk" +
	"uYvlkI7vvIjoh6rmo4DlpLnlhbfmrrXvvInosYHlhY3jgIIKCiAgICDimIUg5Li65LuA5LmI6LWwIEFTVCDogIzkuI3mmK/lrZfn" +
	"rKbkuLIv5q2j5YiZ5omr5o+P77ya5pys5Lu26Ieq5bex5bCx5pyJ5LiA5aCGKiror43ooajluLjph48qKgogICAgICDvvIhgREVM" +
	"RVRFX0NBTExTYCAvIGBXUklURV9XT1JEU2Ag6YeM5YaZ552AIGAicmVtb3ZlImAgLyBgImFkZCJgIC8gYCJjb21taXQiYCDigKbv" +
	"vInigJTigJQKICAgICAg5a2X56ym5Liy5omr5o+P5Lya5oqKKiroh6rlt7HnmoTor43ooagqKuW9k+aIkOWRveS4re+8iOiHquS8" +
	"pO+8ie+8jOS4uuS6hui6suW8gOWPiOW+l+aKiuW4uOmHj+WGmeaIkOaLvOaOpeeahOaAquS4nOilv+OAggogICAgICBBU1Qg5Yik" +
	"55qE5piv44CM55yf6KKr6LCD55So55qEIEFQSSDlkI0gKyDnnJ/kvKDov5vljrvnmoTlrp7lj4LjgI3vvIzkuI7or43ooajplb/k" +
	"u4DkuYjmoLfml6DlhbPjgIIKICAgIOKYhSDov5nmnaHniZnmmK8qKui9vemHjeeahCoq77ya5a6D5oqK44CM5pys5Lu257ud5LiN" +
	"6Ieq5Yqo5YigL+iHquWKqOaUueOAjeS7juWPo+WPt+WPmOaIkOWPr+acuuajgOeahOS4gOagvCDigJTigJQKICAgICAg6Ieq5qOA" +
	"55So5L6LIOKRoyDlloLkuIDmrrXlkKsgYG9zLnJlbW92ZShwKWAg55qE5ZCI5oiQ5rqQ56CBIOKHkiDlv4Xpobvlkb3kuK3vvIjm" +
	"iJDlr7notJ/mjqfvvInjgIIKICAgICIiIgogICAgaGVhZCA9IHNyYyBpZiBjdXQgaXMgTm9uZSBlbHNlIHNyYy5zcGxpdChjdXQp" +
	"WzBdCiAgICB0cnk6CiAgICAgICAgdHJlZSA9IGFzdC5wYXJzZShoZWFkKQogICAgZXhjZXB0IFN5bnRheEVycm9yIGFzIGU6CiAg" +
	"ICAgICAgcmV0dXJuIFsi5rqQ56CB5YiH5q615ZCOIHBhcnNlIOWksei0pe+8iOWIpOS4jeS6hiDih5Ig5b2T57qi77yM5LiN6K64" +
	"6Z2Z6buY5pS+57u/77yJ77yaJXMiICUgZV0KICAgIGhpdHMgPSBbXQogICAgZm9yIG5vZGUgaW4gYXN0LndhbGsodHJlZSk6CiAg" +
	"ICAgICAgaWYgbm90IGlzaW5zdGFuY2Uobm9kZSwgYXN0LkNhbGwpOgogICAgICAgICAgICBjb250aW51ZQogICAgICAgIGYgPSBu" +
	"b2RlLmZ1bmMKICAgICAgICBuYW1lID0gZi5hdHRyIGlmIGlzaW5zdGFuY2UoZiwgYXN0LkF0dHJpYnV0ZSkgZWxzZSAoZi5pZCBp" +
	"ZiBpc2luc3RhbmNlKGYsIGFzdC5OYW1lKSBlbHNlIE5vbmUpCiAgICAgICAgaWYgbmFtZSBpbiBERUxFVEVfQ0FMTFM6CiAgICAg" +
	"ICAgICAgIGhpdHMuYXBwZW5kKCLosIPnlKggJXMoKe+8iOesrCAlZCDooYzvvIkiICUgKG5hbWUsIG5vZGUubGluZW5vKSkKICAg" +
	"ICAgICBlbGlmIG5hbWUgPT0gIm9wZW4iOgogICAgICAgICAgICBtb2RlID0gIiIKICAgICAgICAgICAgaWYgbGVuKG5vZGUuYXJn" +
	"cykgPj0gMiBhbmQgaXNpbnN0YW5jZShub2RlLmFyZ3NbMV0sIGFzdC5Db25zdGFudCk6CiAgICAgICAgICAgICAgICBtb2RlID0g" +
	"c3RyKG5vZGUuYXJnc1sxXS52YWx1ZSkKICAgICAgICAgICAgZm9yIGt3IGluIG5vZGUua2V5d29yZHM6CiAgICAgICAgICAgICAg" +
	"ICBpZiBrdy5hcmcgPT0gIm1vZGUiIGFuZCBpc2luc3RhbmNlKGt3LnZhbHVlLCBhc3QuQ29uc3RhbnQpOgogICAgICAgICAgICAg" +
	"ICAgICAgIG1vZGUgPSBzdHIoa3cudmFsdWUudmFsdWUpCiAgICAgICAgICAgIGlmIGFueShjIGluIG1vZGUgZm9yIGMgaW4gIndh" +
	"eCsiKToKICAgICAgICAgICAgICAgIGhpdHMuYXBwZW5kKCJvcGVuKOKApiwgJXIp77yI56ysICVkIOihjO+8iSIgJSAobW9kZSwg" +
	"bm9kZS5saW5lbm8pKQogICAgICAgIGVsaWYgbmFtZSBpbiAoInJ1biIsICJjaGVja19vdXRwdXQiLCAiY2FsbCIsICJQb3BlbiIs" +
	"ICJjaGVja19jYWxsIik6CiAgICAgICAgICAgIGZvciBhIGluIG5vZGUuYXJnczoKICAgICAgICAgICAgICAgIGlmIGlzaW5zdGFu" +
	"Y2UoYSwgKGFzdC5MaXN0LCBhc3QuVHVwbGUpKToKICAgICAgICAgICAgICAgICAgICBmb3IgZSBpbiBhLmVsdHM6CiAgICAgICAg" +
	"ICAgICAgICAgICAgICAgIGlmIGlzaW5zdGFuY2UoZSwgYXN0LkNvbnN0YW50KSBhbmQgaXNpbnN0YW5jZShlLnZhbHVlLCBzdHIp" +
	"IGFuZCBlLnZhbHVlIGluIEdJVF9XUklURV9TVUJTOgogICAgICAgICAgICAgICAgICAgICAgICAgICAgaGl0cy5hcHBlbmQoInN1" +
	"YnByb2Nlc3Mg6YeM55qEIGdpdCDlhpnlrZDlkb3ku6QgJXLvvIjnrKwgJWQg6KGM77yJIiAlIChlLnZhbHVlLCBub2RlLmxpbmVu" +
	"bykpCiAgICByZXR1cm4gaGl0cwoKCmRlZiBwYXJzZV93cml0ZV9mbGFncyhhcmd2KToKICAgIGhpdHMgPSBbXQogICAgZm9yIHRv" +
	"ayBpbiBhcmd2OgogICAgICAgIHQgPSB0b2suc3RyaXAoKQogICAgICAgIGlmIG5vdCB0IG9yIHQgPT0gIi0tIjoKICAgICAgICAg" +
	"ICAgY29udGludWUKICAgICAgICBuYW1lID0gdC5sc3RyaXAoIi0iKS5zcGxpdCgiPSIsIDEpWzBdLnN0cmlwKCkubG93ZXIoKQog" +
	"ICAgICAgIGlmIG5hbWUgaW4gV1JJVEVfV09SRFM6CiAgICAgICAgICAgIGhpdHMuYXBwZW5kKHQpCiAgICByZXR1cm4gaGl0cwoK" +
	"CiMg4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA" +
	"4pSA4pSA4pSA4pSA4pSA4pSA4pSAIOivu+aVsCDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDi" +
	"lIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIAKZGVmIHJlYWRfZG9jdG9yKHplcmcsIGRvY3Rv" +
	"cl9maWxlPU5vbmUsIHRpbWVvdXQ9NjApOgogICAgIiIi5Y+WIGB6ZXJnIGRvY3RvcmAg55qE44CM5Zue5pS25YCZ6YCJ77yI5bm9" +
	"54G15pyN5Yqh77yJ44CN5p2h5pWw77yIPSBgVDJgIOWIpOaNruKRoiDnmoTlkIzkuIDor7vmlbDvvInjgILih5IgKGNvdW50LCBk" +
	"ZXRhaWwsIGVycinjgIIiIiIKICAgIGlmIGRvY3Rvcl9maWxlOgogICAgICAgIGlmIG5vdCBvcy5wYXRoLmlzZmlsZShkb2N0b3Jf" +
	"ZmlsZSk6CiAgICAgICAgICAgIHJldHVybiBOb25lLCBOb25lLCAiZG9jdG9yIOWkueWFt+S4jeWtmOWcqO+8miVzIiAlIGRvY3Rv" +
	"cl9maWxlCiAgICAgICAgdGV4dCA9IG9wZW4oZG9jdG9yX2ZpbGUsIGVuY29kaW5nPSJ1dGYtOCIpLnJlYWQoKQogICAgZWxzZToK" +
	"ICAgICAgICB0cnk6CiAgICAgICAgICAgIHAgPSBzdWJwcm9jZXNzLnJ1bihbemVyZywgImRvY3RvciIsICItLWpzb24iLCBET0NU" +
	"T1JfRklFTERTXSwKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIGNhcHR1cmVfb3V0cHV0PVRydWUsIHRleHQ9VHJ1ZSwg" +
	"dGltZW91dD10aW1lb3V0KQogICAgICAgIGV4Y2VwdCBFeGNlcHRpb24gYXMgZTogICAgICAgICAgICAgICAgICAgICAgICAgICAg" +
	"ICAgICAgICMgbm9xYTogQkxFMDAxCiAgICAgICAgICAgIHJldHVybiBOb25lLCBOb25lLCAiemVyZyBkb2N0b3Ig6LeR5LiN6LW3" +
	"5p2l77yaJXMiICUgZQogICAgICAgIGlmIHAucmV0dXJuY29kZSBub3QgaW4gKDAsIDEsIDgpOgogICAgICAgICAgICByZXR1cm4g" +
	"Tm9uZSwgTm9uZSwgInplcmcgZG9jdG9yIOmAgOeggSAlZO+8miVzIiAlIChwLnJldHVybmNvZGUsIChwLnN0ZGVyciBvciBwLnN0" +
	"ZG91dCkuc3RyaXAoKVs6MTYwXSkKICAgICAgICB0ZXh0ID0gcC5zdGRvdXQKICAgIHRyeToKICAgICAgICBkb2MgPSBqc29uLmxv" +
	"YWRzKHRleHQpCiAgICAgICAgZm9yIGl0IGluIGRvYy5nZXQoIml0ZW1zIikgb3IgW106CiAgICAgICAgICAgIGlmIGl0LmdldCgi" +
	"bmFtZSIpID09IERPQ1RPUl9JVEVNOgogICAgICAgICAgICAgICAgZGV0YWlsID0gaXQuZ2V0KCJkZXRhaWwiLCAiIikKICAgICAg" +
	"ICAgICAgICAgIGlmIGl0LmdldCgidmVyZGljdCIpID09ICJTS0lQIjoKICAgICAgICAgICAgICAgICAgICByZXR1cm4gTm9uZSwg" +
	"ZGV0YWlsLCAiZG9jdG9yIOmCo+agvOaYryBTS0lQ77yI6K+75LiN5YiwIOKHkiDkuI3nu5nnu5PorrrvvIkiCiAgICAgICAgICAg" +
	"ICAgICBtID0gcmUubWF0Y2gociJccyooXGQrKVxzKuadoSIsIGRldGFpbCkKICAgICAgICAgICAgICAgIGlmIG5vdCBtOgogICAg" +
	"ICAgICAgICAgICAgICAgIHJldHVybiAwLCBkZXRhaWwsIE5vbmUgICAgICAgICAgICAgICAgICAgICAgICMgUEFTUyDlvaLmgIHv" +
	"vJrmsqHmnInjgIxOIOadoeOAjQogICAgICAgICAgICAgICAgcmV0dXJuIGludChtLmdyb3VwKDEpKSwgZGV0YWlsLCBOb25lCiAg" +
	"ICAgICAgcmV0dXJuIE5vbmUsIE5vbmUsICJkb2N0b3Ig6L6T5Ye66YeM5rKh5pyJ44CMJXPjgI3pgqPkuIDmoLzvvIjlkIzmupDl" +
	"r7nkuI3kuIog4oeSIOS4jee7mee7k+iuuu+8iSIgJSBET0NUT1JfSVRFTQogICAgZXhjZXB0IEV4Y2VwdGlvbiBhcyBlOiAgICAg" +
	"ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICMgbm9xYTogQkxFMDAxCiAgICAgICAgcmV0dXJuIE5vbmUsIE5vbmUs" +
	"ICJkb2N0b3Ig6L6T5Ye65LiN5pivIEpTT07vvJolcyIgJSBlCgoKZGVmIGZpbGVfc2hhKHBhdGgpOgogICAgaCA9IGhhc2hsaWIu" +
	"c2hhMjU2KCkKICAgIHdpdGggb3BlbihwYXRoLCAicmIiKSBhcyBmOgogICAgICAgIGZvciBjaHVuayBpbiBpdGVyKGxhbWJkYTog" +
	"Zi5yZWFkKDEgPDwgMjApLCBiIiIpOgogICAgICAgICAgICBoLnVwZGF0ZShjaHVuaykKICAgIHJldHVybiBoLmhleGRpZ2VzdCgp" +
	"CgoKZGVmIGdpdF9ybyhyZXBvLCBhcmdzLCB0aW1lb3V0PTYwKToKICAgICIiIuWPquivuyBnaXQg5YWl5Y+j77yI55m95ZCN5Y2V" +
	"ICsgYC0tbm8tb3B0aW9uYWwtbG9ja3Ng77ya5rKh5pyJ5a6D77yMYGdpdCBzdGF0dXNgIOS8muaUueWGmSBgLmdpdC9pbmRleGDv" +
	"vInjgIIiIiIKICAgIGFsbG93ID0gKCJyZXYtcGFyc2UiLCAic3RhdHVzIiwgImxzLWZpbGVzIiwgImNhdC1maWxlIikKICAgIGlm" +
	"IG5vdCBhcmdzIG9yIGFyZ3NbMF0gbm90IGluIGFsbG93OgogICAgICAgIHJhaXNlIFJ1bnRpbWVFcnJvcigiZ2l0IOWtkOWRveS7" +
	"pCAlciDkuI3lnKjlj6ror7vnmb3lkI3ljZUgJXMg5YaFIOKHkiDmi5IiICUgKGFyZ3NbMF0gaWYgYXJncyBlbHNlIE5vbmUsIGxp" +
	"c3QoYWxsb3cpKSkKICAgIHAgPSBzdWJwcm9jZXNzLnJ1bihbImdpdCIsICItYyIsICJjb3JlLnF1b3RlcGF0aD1mYWxzZSIsICIt" +
	"LW5vLW9wdGlvbmFsLWxvY2tzIiwgIi1DIiwgcmVwb10gKyBsaXN0KGFyZ3MpLAogICAgICAgICAgICAgICAgICAgICAgIGNhcHR1" +
	"cmVfb3V0cHV0PVRydWUsIHRleHQ9VHJ1ZSwgdGltZW91dD10aW1lb3V0KQogICAgaWYgcC5yZXR1cm5jb2RlICE9IDA6CiAgICAg" +
	"ICAgcmFpc2UgUnVudGltZUVycm9yKCLpgIDnoIEgJWTvvJolcyIgJSAocC5yZXR1cm5jb2RlLCAocC5zdGRlcnIgb3IgIiIpLnN0" +
	"cmlwKClbOjE2MF0pKQogICAgcmV0dXJuIHAuc3Rkb3V0CgoKIyDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDi" +
	"lIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIAg5Y+q6K+76Ieq6K+B77yIRDLv" +
	"vInilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDi" +
	"lIDilIDilIDilIDilIDilIDilIAKZGVmIHJlYWRvbmx5X2ZpbmdlcnByaW50KHJvb3QsIGRlY2xfcGF0aCk6CiAgICBmcCA9IHt9" +
	"CiAgICBmb3IgcCBpbiAoZGVjbF9wYXRoLCBTSUJMSU5HLCBvcy5wYXRoLmFic3BhdGgoX19maWxlX18pKToKICAgICAgICBpZiBw" +
	"IGFuZCBvcy5wYXRoLmlzZmlsZShwKToKICAgICAgICAgICAgZnBbIjznnJ/mupAv5Lu2Pi8iICsgb3MucGF0aC5yZWxwYXRoKHAs" +
	"IHJvb3QpIGlmIHAuc3RhcnRzd2l0aChyb290KSBlbHNlICI85Lu2Pi8iICsgb3MucGF0aC5iYXNlbmFtZShwKV0gPSBmaWxlX3No" +
	"YShwKQogICAgZ2QgPSBvcy5wYXRoLmpvaW4ocm9vdCwgIi5naXQiKQogICAgaWYgb3MucGF0aC5pc2RpcihnZCk6CiAgICAgICAg" +
	"Zm9yIG4gaW4gKCJIRUFEIiwgImluZGV4IiwgInBhY2tlZC1yZWZzIiwgIk9SSUdfSEVBRCIpOgogICAgICAgICAgICBwID0gb3Mu" +
	"cGF0aC5qb2luKGdkLCBuKQogICAgICAgICAgICBpZiBvcy5wYXRoLmlzZmlsZShwKToKICAgICAgICAgICAgICAgIGZwWyI8Lmdp" +
	"dD4vIiArIG5dID0gZmlsZV9zaGEocCkKICAgIHRyeToKICAgICAgICBmcFsiPHBvcmNlbGFpbiDlhajmloc+Il0gPSBoYXNobGli" +
	"LnNoYTI1NihnaXRfcm8ocm9vdCwgWyJzdGF0dXMiLCAiLS1wb3JjZWxhaW4iXSkuZW5jb2RlKCJ1dGYtOCIpKS5oZXhkaWdlc3Qo" +
	"KQogICAgZXhjZXB0IEV4Y2VwdGlvbiBhcyBlOiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICMgbm9xYTog" +
	"QkxFMDAxCiAgICAgICAgZnBbIjxwb3JjZWxhaW4g5YWo5paHPiJdID0gIuivu+S4jeWIsO+8miVzIiAlIGUKICAgIHJldHVybiBm" +
	"cAoKCiMg4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA" +
	"4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSAIOS4u+S9kyDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDi" +
	"lIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIDilIAKZGVmIGNoZWNrKHJvb3QsIG1vZCwgZGVj" +
	"bF9wYXRoPU5vbmUsIHBzX2ZpbGU9Tm9uZSwgbGF1bmNoY3RsX2ZpbGU9Tm9uZSwgaG9tZT1Ob25lLAogICAgICAgICAgemVyZz1O" +
	"b25lLCBkb2N0b3JfZmlsZT1Ob25lKToKICAgICIiIuKHkiAocmMsIHNpZGUxLCBzaWRlMiwgbm90ZXMsIHByb2JsZW1zKeOAgnNp" +
	"ZGUxID0g55uY5LiK5pyJL+WjsOaYjuaXoCDCtyBzaWRlMiA9IOWjsOaYjuaciS/nm5jkuIrml6DjgIIiIiIKICAgIG5vdGVzLCBw" +
	"cm9ibGVtcyA9IFtdLCBbXQogICAgZGVjbF9wYXRoID0gZGVjbF9wYXRoIG9yIG9zLmVudmlyb24uZ2V0KCJaRVJHX1NWQ0RFQ0xf" +
	"REVDTCIpIG9yIG9zLnBhdGguam9pbihyb290LCAiZGVwbG95IiwgIuacjeWKoeWjsOaYji50c3YiKQogICAgcm93cywgcGVyciA9" +
	"IG1vZC5wYXJzZV9kZWNsKGRlY2xfcGF0aCkKICAgIGlmIHJvd3MgaXMgTm9uZToKICAgICAgICByZXR1cm4gMiwgW10sIFtdLCBu" +
	"b3RlcywgbGlzdChwZXJyKQogICAgcHJvYmxlbXMuZXh0ZW5kKHBlcnIpCgogICAgbGF1bmNoZF9yb3dzID0gW3IgZm9yIHIgaW4g" +
	"cm93cyBpZiByWyJraW5kIl0gPT0gImxhdW5jaGQiXQogICAgZ2hvc3Rfcm93cyA9IFtyIGZvciByIGluIHJvd3MgaWYgclsia2lu" +
	"ZCJdID09ICJnaG9zdCJdCiAgICBpZiBub3QgbGF1bmNoZF9yb3dzOgogICAgICAgIHJldHVybiAyLCBbXSwgW10sIG5vdGVzLCBb" +
	"Iuepuui9rO+8muWjsOaYjuagkemHjCAwIOadoSBsYXVuY2hkIOWjsOaYju+8iOWIpOaNruS4jeWPr+WIpCDih5Ig5LiN57uZ57uT" +
	"6K6677yJIl0KCiAgICBwc19saW5lcywgcHNtb2RlLCBwc19lcnIgPSBtb2QucmVhZF9saW5lcyhwc19maWxlIGlmIHBzX2ZpbGUg" +
	"aXMgbm90IE5vbmUgZWxzZSBvcy5lbnZpcm9uLmdldCgiWkVSR19TVkNERUNMX1BTIiksCiAgICAgICAgICAgICAgICAgICAgICAg" +
	"ICAgICAgICAgICAgICAgICAgICAgICAicHMgLWVvIHBpZCxwcGlkLGV0aW1lLGNvbW1hbmQiKQogICAgbGNfbGluZXMsIGxjbW9k" +
	"ZSwgbGNfZXJyID0gbW9kLnJlYWRfbGluZXMobGF1bmNoY3RsX2ZpbGUgaWYgbGF1bmNoY3RsX2ZpbGUgaXMgbm90IE5vbmUKICAg" +
	"ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIGVsc2Ugb3MuZW52aXJvbi5nZXQoIlpFUkdfU1ZDREVD" +
	"TF9MQVVOQ0hDVEwiKSwgImxhdW5jaGN0bCBsaXN0IikKICAgIGhvbWUgPSBob21lIG9yIG9zLmVudmlyb24uZ2V0KCJaRVJHX1NW" +
	"Q0RFQ0xfSE9NRSIpIG9yIG9zLnBhdGguZXhwYW5kdXNlcigifiIpCiAgICBpZiBwc19lcnI6CiAgICAgICAgcmV0dXJuIDIsIFtd" +
	"LCBbXSwgbm90ZXMsIFsi5LiN5Y+v5Yik77yacHMg5Y+W5LiN5Yiw77yIJXPvvIkiICUgcHNfZXJyXQogICAgaWYgbGNfZXJyOgog" +
	"ICAgICAgIHJldHVybiAyLCBbXSwgW10sIG5vdGVzLCBbIuS4jeWPr+WIpO+8mmxhdW5jaGN0bCDlj5bkuI3liLDvvIglcyDCtyDp" +
	"nZ4gbWFjT1Mg5py65Zmo5bGe5q2j5bi4IOKHkiDmnKzku7bkuI3nu5nnu5PorrrjgIHkuI3lvZPnu7/vvIkiICUgbGNfZXJyXQoK" +
	"ICAgIGxpdmUgPSBbbCBmb3IgbCBpbiBwc19saW5lcyBpZiBhbnkoaCBpbiBsIGZvciBoIGluIG1vZC5QUk9DX0hJTlRTKV0KICAg" +
	"IGlmIG5vdCBsaXZlOgogICAgICAgIHJldHVybiAyLCBbXSwgW10sIG5vdGVzLCBbIuepuui9rO+8mnBzIOmHjOS4gOS7tuiHqueg" +
	"lOS7tu+8iCVz4oCm77yJ6YO95rKh5omr5Yiw77yI5LiN6K645b2T57u/77yJIiAlIG1vZC5QUk9DX0hJTlRTWzBdXQoKICAgICMg" +
	"4pSA4pSAIOS+p+KRoOOAjOebmOS4iuaciSAvIOWjsOaYjuaXoOOAje+8mueOsOWAvOiHqueglOi/m+eoi+mHjO+8jOS4jeiiq+S7" +
	"u+S9lSBraW5kPWxhdW5jaGQg55qE6L+b56iL54m55b6B6KaG55uW55qE5p2hIOKUgOKUgAogICAgIyDimIUgR0FQLTIwMjYwOTI3" +
	"LTYyIOayu+WwvuW3tOKRoChhKSDigJTigJQgKipQUElEIOa3seW6pjHnm7Tns7vosYHlhY0qKu+8mmBwcyAtZW8gcGlkLHBwaWQs" +
	"4oCmYCDnmoTnrKwgMiDliJcgcHBpZAogICAgIyAgIOaYr+OAjOW3suWjsOaYjuacjeWKoea0vueUnyB0YXNrLXNjb3BlZCDlrZDo" +
	"v5vnqIvjgI3nmoTnjrDmiJDor4Hmja7jgILlhYjlj5YqKuaJgOacieiiqyBsYXVuY2hkIOWjsOaYjuimhueblueahOeOsOWAvOi/" +
	"m+eoiyoqCiAgICAjICAg55qEIHBpZCDkvZzniLbpm4blkIjvvJvnjrDlgLzoh6rnoJTov5vnqIvoi6UqKuacquiiq+S7u+S9lSBs" +
	"YXVuY2hkIOWjsOaYjuimhueblioq44CBKirmnKrooqvku7vkvZUgZ2hvc3Qg6KGM55m76K6wKirvvIwKICAgICMgICDogIzlhbYg" +
	"cHBpZCDiiIgg6K+l54i26ZuG5ZCIIOKHkiDlroPmmK/mn5DmnaHlt7Llo7DmmI7mnI3liqHnmoQqKuebtOaOpeWtkCoq77yI5rex" +
	"5bqmIDHvvInih5IgKirkuI3nrpfmnKrnmbvorrDlub3ngbUqKuOAggogICAgIyAgIOKclyDkuI3ov73lrZnov5vnqIvvvJrmt7Hl" +
	"uqYgMisg55qE5a2Q5a2ZIHBwaWQg6JC95Zyo5a2Q6L+b56iLIHBpZCDkuIrjgIHkuI3lnKjniLbpm4blkIjlhoXvvIznhafml6fo" +
	"rqHlhaXjgIIKICAgIGRlY2xhcmVkX3BpZHMgPSBzZXQoKQogICAgZm9yIF9sbiBpbiBwc19saW5lczoKICAgICAgICBpZiBhbnko" +
	"clsicHJvYyJdIGFuZCByWyJwcm9jIl0gIT0gIi0iIGFuZCByWyJwcm9jIl0gaW4gX2xuIGZvciByIGluIGxhdW5jaGRfcm93cyk6" +
	"CiAgICAgICAgICAgIF9mID0gX2xuLnNwbGl0KCkKICAgICAgICAgICAgaWYgX2Y6CiAgICAgICAgICAgICAgICBkZWNsYXJlZF9w" +
	"aWRzLmFkZChfZlswXSkKICAgIHNpZGUxID0gW10KICAgIGZvciBsbiBpbiBsaXZlOgogICAgICAgIGhpdCA9IFtyIGZvciByIGlu" +
	"IGxhdW5jaGRfcm93cyBpZiByWyJwcm9jIl0gYW5kIHJbInByb2MiXSAhPSAiLSIgYW5kIHJbInByb2MiXSBpbiBsbl0KICAgICAg" +
	"ICBpZiBoaXQ6CiAgICAgICAgICAgIGNvbnRpbnVlCiAgICAgICAgZmllbGRzID0gbG4uc3BsaXQoKQogICAgICAgIHJlZyA9IFty" +
	"IGZvciByIGluIGdob3N0X3Jvd3MgaWYgclsicHJvYyJdIGFuZCByWyJwcm9jIl0gIT0gIi0iIGFuZCByWyJwcm9jIl0gaW4gbG5d" +
	"CiAgICAgICAgaWYgbm90IHJlZyBhbmQgbGVuKGZpZWxkcykgPj0gMiBhbmQgZmllbGRzWzFdIGluIGRlY2xhcmVkX3BpZHM6CiAg" +
	"ICAgICAgICAgIGNvbnRpbnVlCiAgICAgICAgc2lkZTEuYXBwZW5kKHsicGlkIjogZmllbGRzWzBdIGlmIGZpZWxkcyBlbHNlICI/" +
	"IiwgImNtZGxpbmUiOiAiICIuam9pbihmaWVsZHNbMTpdKS5zdHJpcCgpLAogICAgICAgICAgICAgICAgICAgICAgInJlZ2lzdGVy" +
	"ZWRfYXMiOiByZWdbMF1bIm5hbWUiXSBpZiByZWcgZWxzZSBOb25lLAogICAgICAgICAgICAgICAgICAgICAgImRpc3Bvc2l0aW9u" +
	"IjogcmVnWzBdWyJkaXNwb3NpdGlvbiJdIGlmIHJlZyBlbHNlIE5vbmUsCiAgICAgICAgICAgICAgICAgICAgICAicmF3IjogbG4u" +
	"c3RyaXAoKX0pCgogICAgIyDilIDilIAg5L6n4pGh44CM5aOw5piO5pyJIC8g55uY5LiK5peg44CN77ya5q+P5p2hIGxhdW5jaGQg" +
	"5aOw5piO55qE5LiJ546v77yI6KOF6L29IC8g6L+b56iLIC8g5aOw5piO5Lu25Zyo55uY77yJ57y65ZOq5LiA546vIOKUgOKUgAog" +
	"ICAgbGNfemVyZyA9IFtsIGZvciBsIGluIGxjX2xpbmVzIGlmICJ6ZXJnIiBpbiBsLmxvd2VyKCldICAgICAgIyDimIUg5Y+q55WZ" +
	"5L2c5LiL6Z2iIDozMDgg5rOo6K6w55qE44CM6KGM5pWw44CN6K6h5pWw77yI6L6T5Ye65qih5p2/6YCQ5a2X5LiN5Yqo77yJCiAg" +
	"ICAjIOKYhSDoo4Xovb3liKTmja4gPSAqKumXqOKRpyDnmoTlkIzkuIDku73pm4blkIjmr5TovoMqKu+8iGBsYWJlbF9zZXRfZGlm" +
	"ZmAgwrcg5ZCM5rqQIGltcG9ydCDih5Ig5LiN5YaZ56ys5LqM5Lu95Y+j5b6E77yJ77yaCiAgICAjICAg5pen5b2i5oCB44CMYG5h" +
	"bWVgIOacq+autSAvIGBwcm9jYCDlrZDkuLIg4oiIIOafkOadoeWQqyB6ZXJnIOeahOihjOOAjeaYryoq54ycKirvvIjpl63pm4bl" +
	"pJbmoIfnrb7jgIHmjaLlkI3lkIzplb/pg73lgYfnu7/vvInvvJsKICAgICMgICDnjrDmlLnkuLrjgIzmlbTmoIfnrb4g4oiIIOmX" +
	"rembhuWGheagh+etvumbhuOAjeKAlOKAlCDnrYnku7fmm7/mjaIgLyDpm4blkIjljJbmlLbnqoTvvIjpgIDnoIHor63kuYnkuI7o" +
	"vpPlh7rmqKHmnb/pgJDlrZfkuI3liqjvvInjgIIKICAgIGxpdmVfbGFiZWxzID0gbW9kLmxhYmVsX3NldF9kaWZmKGxhdW5jaGRf" +
	"cm93cywgbGNfbGluZXMpWzBdCiAgICBzaWRlMiA9IFtdCiAgICBmb3IgciBpbiBsYXVuY2hkX3Jvd3M6CiAgICAgICAgbWlzc2lu" +
	"ZyA9IFtdCiAgICAgICAgbG9hZGVkID0gclsibmFtZSJdIGluIGxpdmVfbGFiZWxzCiAgICAgICAgaWYgbm90IGxvYWRlZDoKICAg" +
	"ICAgICAgICAgbWlzc2luZy5hcHBlbmQoImxhdW5jaGN0bCDmnKroo4Xovb0iKQogICAgICAgICMg4piFIOivjeeVjOmUmu+8iGBH" +
	"QVAtMjAyNjA5MjctMTAwYC9gLTEwN2Ag5ZCM5peP77yJ77ya6L+b56iL54m55b6B6aG75L2cKirmlbTor40qKuWHuueOsO+8iOWQ" +
	"jumaj+epuueZvS/ooYzlsL7vvInvvIwKICAgICAgICAjICAgYGJpbi96ZXJnLWNvcmVgIOS4jeWGjeiiqyBgYmluL3plcmctY29y" +
	"ZS10ZXN0YCDlrZDkuLLlgYfnu7/vvIjkuI7pl6jikacgUjQg5ZCM5LiA5Y+j5b6E77yJ44CCCiAgICAgICAgaWYgbm90IGFueShy" +
	"WyJwcm9jIl0gYW5kIHJbInByb2MiXSAhPSAiLSIKICAgICAgICAgICAgICAgICAgIGFuZCByZS5zZWFyY2gocmUuZXNjYXBlKHJb" +
	"InByb2MiXSkgKyByIig/PVxzfCQpIiwgbG4pIGZvciBsbiBpbiBsaXZlKToKICAgICAgICAgICAgbWlzc2luZy5hcHBlbmQoInBz" +
	"IOaXoOatpOi/m+eoiyIpCiAgICAgICAgcCA9IHJbImRlY2wiXQogICAgICAgIGV4cCA9IG9zLnBhdGguam9pbihob21lLCBwWzI6" +
	"XSkgaWYgcC5zdGFydHN3aXRoKCJ+LyIpIGVsc2UgcAogICAgICAgIGlmIG5vdCBvcy5wYXRoLmV4aXN0cyhleHApOgogICAgICAg" +
	"ICAgICBtaXNzaW5nLmFwcGVuZCgi5aOw5piO5Lu25LiN5Zyo55uY77yIJXPvvIkiICUgcCkKICAgICAgICBpZiBtaXNzaW5nOgog" +
	"ICAgICAgICAgICBzaWRlMi5hcHBlbmQoeyJuYW1lIjogclsibmFtZSJdLCAiZGVjbCI6IHJbImRlY2wiXSwgIm1pc3NpbmciOiBt" +
	"aXNzaW5nLCAibGluZSI6IHJbImxpbmUiXX0pCgogICAgbm90ZXMuYXBwZW5kKCLlo7DmmI7moJHvvJolZCDmnaEgbGF1bmNoZCDC" +
	"tyAlZCDmnaEgZ2hvc3TvvIjlt7LnmbvorrDlub3ngbXvvInCtyDnjrDlgLzoh6rnoJTku7YgJWQg5p2hIMK3IGxhdW5jaGN0bCDp" +
	"h4wgemVyZyDooYwgJWQiCiAgICAgICAgICAgICAgICAgJSAobGVuKGxhdW5jaGRfcm93cyksIGxlbihnaG9zdF9yb3dzKSwgbGVu" +
	"KGxpdmUpLCBsZW4obGNfemVyZykpKQoKICAgICMg4pSA4pSAIFMyIOWQjOa6kOaWreiogO+8iOKYhSBHQVAtMjAyNjA5MjctNjIg" +
	"5rK75bC+5be04pGgKGIp77yJ77yaKirkvqfikaAg5LitIHJlZ2lzdGVyZWRfYXMg6Z2e56m655qE5a2Q6ZuGKiogPT0KICAgICMg" +
	"ICBgemVyZyBkb2N0b3Jg44CM5Zue5pS25YCZ6YCJ77yI5bm954G15pyN5Yqh77yJ44CN5p2h5pWw44CC55yf5rqQ5qC45a+577ya" +
	"ZG9jdG9yIOKRpyDkuI4gYGNvcmUgZGFlbW9uIGxzIC0tZGVjbGFyZWRgCiAgICAjICAg5ZCM6LCDIGBnaG9zdFByb2Nlc3Nlcygp" +
	"YO+8iGBraW5kPWdob3N0YCDooYwgw5cg546w5YC85ZG95Lit77yJ4oeSIGRvY3RvciDmlbDnmoQqKuato+aYr+W3sueZu+iusOW5" +
	"veeBtSoq77yM5pyq55m76K6w5bm954G1CiAgICAjICAg5a6D5qC55pys55yL5LiN6KeBIOKHkiDml6flhpnms5Xmi78gYGxlbihz" +
	"aWRlMSlg77yI5ZCr5pyq55m76K6w77yJ5Y675q+U77yM5Lya5oqK44CM5aSa5LiA5p2h5pyq55m76K6w5bm954G144CN6K+v5Yik" +
	"5oiQ5ZCM5rqQ5ryC56e757qi44CCCiAgICBpZiB6ZXJnIGlzIE5vbmU6CiAgICAgICAgemVyZyA9IG9zLnBhdGguam9pbihyb290" +
	"LCAiYmluIiwgInplcmciKQogICAgZGNvdW50LCBkZGV0YWlsLCBkZXJyID0gcmVhZF9kb2N0b3IoemVyZywgZG9jdG9yX2ZpbGU9" +
	"ZG9jdG9yX2ZpbGUpCiAgICBpZiBkZXJyOgogICAgICAgIHJldHVybiAyLCBzaWRlMSwgc2lkZTIsIG5vdGVzLCBbIuWQjOa6kOiv" +
	"u+aVsOWPluS4jeWIsO+8miVz77yIYFQyYCDliKTmja7ikaIg55qE5ZCM5LiA5qC8IOKHkiDkuI3orrjmjaLkuIDku73lj6PlvoTv" +
	"vIkiICUgZGVycl0KICAgIHJlZ19zaWRlMSA9IFtpdCBmb3IgaXQgaW4gc2lkZTEgaWYgaXRbInJlZ2lzdGVyZWRfYXMiXSBpcyBu" +
	"b3QgTm9uZV0KICAgIGlmIGRjb3VudCAhPSBsZW4ocmVnX3NpZGUxKToKICAgICAgICBwcm9ibGVtcy5hcHBlbmQoIkZBSUwgUzIg" +
	"5ZCM5rqQ5ryC56e777ya5pys5Lu25L6n4pGgIOW3sueZu+iusO+8iGByZWdpc3RlcmVkX2FzYCDpnZ7nqbrvvIklZCDmnaEgdnMg" +
	"YHplcmcgZG9jdG9yYOOAjCVz44CNJWQg5p2hIOKHkiDkuKTlpITlj6PlvoTlt7LliIblj4kiCiAgICAgICAgICAgICAgICAgICAg" +
	"ICAgICLvvIhgVDJgIOWIpOaNruKRoiDopoHmsYLpgJDlrZflkIzlgLzvvInCtyBkb2N0b3Ig5Y6f6K+777yaJXMiCiAgICAgICAg" +
	"ICAgICAgICAgICAgICAgICUgKGxlbihyZWdfc2lkZTEpLCBET0NUT1JfSVRFTSwgZGNvdW50LCBkZGV0YWlsKSkKICAgIG5vdGVz" +
	"LmFwcGVuZCgiUzIg5ZCM5rqQ77ya5L6n4pGgIOW3sueZu+iusCAlZCDmnaHvvIjkvqfikaAg5YWxICVkIOadoe+8icK3IGRvY3Rv" +
	"cuOAjCVz44CNJWQg5p2h77yIJXPvvIkiCiAgICAgICAgICAgICAgICAgJSAobGVuKHJlZ19zaWRlMSksIGxlbihzaWRlMSksIERP" +
	"Q1RPUl9JVEVNLCBkY291bnQsCiAgICAgICAgICAgICAgICAgICAgIumAkOWtl+WQjOWAvCDinJMiIGlmIGRjb3VudCA9PSBsZW4o" +
	"cmVnX3NpZGUxKSBlbHNlICLkuI3nrYkg4pyXIikpCiAgICBub3Rlcy5hcHBlbmQoIuWPluivge+8mnBzPSVzIMK3IGxhdW5jaGN0" +
	"bD0lcyDCtyBkb2N0b3I9JXMiICUgKHBzbW9kZSwgbGNtb2RlLCAiZml4dHVyZSIgaWYgZG9jdG9yX2ZpbGUgZWxzZSAibGl2ZSIp" +
	"KQoKICAgIHJldHVybiAoMSBpZiBwcm9ibGVtcyBlbHNlIDApLCBzaWRlMSwgc2lkZTIsIG5vdGVzLCBwcm9ibGVtcwoKCmRlZiBw" +
	"cmludF9yZXBvcnQocm9vdCwgZGVjbF9wYXRoLCByYywgc2lkZTEsIHNpZGUyLCBub3RlcywgcHJvYmxlbXMpOgogICAgcHJpbnQo" +
	"IuKUgOKUgCBgRDRgIOW5veeBtS/lo7DmmI7lt64qKuWPquivuyBkaWZmKirvvIjnjrDlgLwgdnMg5aOw5piOIMK3IOS4pOS+p+mA" +
	"kOadoSDCtyAqKuS4jeiHquWKqOa4hSAvIOS4jeiHquWKqOihpSoq77yJ4pSA4pSAIikKICAgIHByaW50KCIgIOWjsOaYjuS+p+ec" +
	"n+a6kDogJXPvvIhzaGEyNTYgJXPvvIkiICUgKG9zLnBhdGgucmVscGF0aChkZWNsX3BhdGgsIHJvb3QpIGlmIGRlY2xfcGF0aC5z" +
	"dGFydHN3aXRoKHJvb3QpIGVsc2UgZGVjbF9wYXRoLAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgZmls" +
	"ZV9zaGEoZGVjbF9wYXRoKVs6MTZdICsgIuKApiIgaWYgb3MucGF0aC5pc2ZpbGUoZGVjbF9wYXRoKSBlbHNlICLnvLrku7YiKSkK" +
	"ICAgIHByaW50KCIgIOino+aekOWZqO+8iOWQjOa6kO+8iTog6Zeo4pGnIGBjaGVjay1zZXJ2aWNlLWRlY2xhcmF0aW9uLnB5YCDn" +
	"moQgYHBhcnNlX2RlY2xg77yIKirlkIzkuIDku73ku6PnoIEqKiDCtyBpbXBvcnRsaWIg5Yqg6L2977yJIikKICAgIHByaW50KCIg" +
	"IOeOsOWAvOS+p+adpea6kDogYHBzIC1lbyBwaWQscHBpZCxldGltZSxjb21tYW5kYCArIGBsYXVuY2hjdGwgbGlzdGAgwrcg6L+b" +
	"56iL54m55b6BICVz77yI5LiO6Zeo4pGnIOWQjOS4gOWvueWRveS7pOWQjOS4gOe7hOeJueW+ge+8iSIKICAgICAgICAgICUgIiAi" +
	"LmpvaW4oU0lCTElOR19ISU5UUykpCiAgICBmb3IgbiBpbiBub3RlczoKICAgICAgICBwcmludCgiICDCtyAlcyIgJSBuKQogICAg" +
	"cHJpbnQoIiAg4pSA4pSAIOS+p+KRoOOAjCoq55uY5LiK5pyJIC8g5aOw5piO5pegKirjgI3vvIjnjrDlgLzoh6rnoJTov5vnqIvp" +
	"h4zvvIzkuI3ooqvku7vkvZUgYGtpbmQ9bGF1bmNoZGAg5aOw5piO6KaG55uW55qE5p2h77yJ77yaJWQg5p2hIgogICAgICAgICAg" +
	"JSBsZW4oc2lkZTEpKQogICAgaWYgbm90IHNpZGUxOgogICAgICAgIHByaW50KCIgICAgIO+8iOepuu+8iTAg5p2h77ya546w5YC8" +
	"5q+P5LiA5Lu26Ieq56CU5Lu26YO95Zyo5aOw5piO6Z2i5Y+v5p+l77yIbGF1bmNoZCDmiJblt7LnmbvorrAgZ2hvc3TvvIkiKQog" +
	"ICAgZm9yIGl0IGluIHNpZGUxOgogICAgICAgIHRhZyA9ICgi5bey55m76K6w77yaZ2hvc3Qg6KGM44CMJXPjgI3CtyDlpITnva49" +
	"JS40MHMiICUgKGl0WyJyZWdpc3RlcmVkX2FzIl0sIGl0WyJkaXNwb3NpdGlvbiJdKSkgaWYgaXRbInJlZ2lzdGVyZWRfYXMiXSBc" +
	"CiAgICAgICAgICAgIGVsc2UgIioq5pyq55m76K6w5bm954G1KirvvIjpl6jikacgUjQg55qE57qi5b2i5oCB77yJIgogICAgICAg" +
	"IHByaW50KCIgICAgIMK3IHBpZD0lLTdzICVzIiAlIChpdFsicGlkIl0sIGl0WyJjbWRsaW5lIl1bOjExMF0pKQogICAgICAgIHBy" +
	"aW50KCIgICAgICAg44CUJXPjgJUiICUgdGFnKQogICAgcHJpbnQoIiAg4pSA4pSAIOS+p+KRoeOAjCoq5aOw5piO5pyJIC8g55uY" +
	"5LiK5pegKirjgI3vvIhga2luZD1sYXVuY2hkYCDlo7DmmI7nmoTokL3ngrnnvLrnjq/vvInvvJolZCDmnaEiICUgbGVuKHNpZGUy" +
	"KSkKICAgIGlmIG5vdCBzaWRlMjoKICAgICAgICBwcmludCgiICAgICDvvIjnqbrvvIkwIOadoe+8muavj+adoSBsYXVuY2hkIOWj" +
	"sOaYjueahOS4ieeOr++8iGBsYXVuY2hjdGxgIOijhei9vSDCtyBgcHNgIOi/m+eoiyDCtyDlo7DmmI7ku7blnKjnm5jvvInlhajl" +
	"nKgiKQogICAgZm9yIGl0IGluIHNpZGUyOgogICAgICAgIHByaW50KCIgICAgIMK3ICVz77yI5aOw5piO5Lu2ICVzIMK3IOWjsOaY" +
	"juagkeesrCAlZCDooYzvvInnvLrnjq/vvJolcyIgJSAoaXRbIm5hbWUiXSwgaXRbImRlY2wiXSwgaXRbImxpbmUiXSwgIiArICIu" +
	"am9pbihpdFsibWlzc2luZyJdKSkpCiAgICBwcmludCgiICDilIDilIAg5Y+q6K+76L6555WMIOKUgOKUgCIpCiAgICBwcmludCgi" +
	"ICAgICDCtyDlhpnml5fmoIfpl63pm4bvvIglZCDor43vvInkuIDlvovmi5IgcmM9Mu+8m+a6kOeggee6p+WGmSBBUEkg5omr5o+P" +
	"77yI6Ieq5qOA5qCH6K6w5LmL5YmN55qE55yf55uu5qCH5q6177yJ5ZG95Lit5Y2zIHJjPTEiICUgbGVuKFdSSVRFX1dPUkRTKSkK" +
	"ICAgIHByaW50KCIgICAgIMK3IOacrOS7tioq5LiN5YGaKirku7vkvZXmuIUgLyDooaUgLyDlgZwgLyDmnYDvvJrlgZzkuIDkuKrl" +
	"ub3ngbUgPSDkuI3lj6/pgIbliqjkvZwg4oeSIOeFpyDCp+S5nSBNOSDlvoUgTXIyMTA5IOaLje+8iOacrOS7tuWPquaKpeW3ru+8" +
	"iSIpCiAgICBmb3IgcCBpbiBwcm9ibGVtczoKICAgICAgICBwcmludCgiICDCtyAlcyIgJSBwKQogICAgaWYgcmMgPT0gMDoKICAg" +
	"ICAgICBwcmludCgi4oeSIOS4pOS+p+W3sumAkOadoeWIl+WHuiDCtyDlkIzmupDmlq3oqIDov4fvvIhyYz0w77yJ4oCU4oCUIOW3" +
	"rumbhuacrOi6q+S4jempseWKqOmAgOegge+8muacrOS7tuaYryoq5Y+q6K+7IGRpZmYqKu+8jOaKpeW3ruWwseaYr+Wug+eahOa0" +
	"uyIpCiAgICBlbGlmIHJjID09IDE6CiAgICAgICAgcHJpbnQoIuKHkiByYz0x77ya5ZCM5rqQ5pat6KiA5oiW5YaZ6Z2i5omr5o+P" +
	"5Yik57qi77yI6YCQ5p2h6KeB5LiK77yJIikKICAgIGVsc2U6CiAgICAgICAgcHJpbnQoIuKHkiDkuI3nu5nnu5PorrrvvIhyYz0y" +
	"77yJIikKCgpTSUJMSU5HX0hJTlRTID0gKCJiaW4vemVyZy0iLCAiYmluL2NvY29vbi0iLCAic2Vhcngud2ViYXBwIikKCgpkZWYg" +
	"bGlzdF9ydWxlcygpOgogICAgcHJpbnQoIuKUgOKUgCAlcyDliKTmja7kuI7pl63pm4Yg4pSA4pSAIiAlIFZFUlNJT04pCiAgICBw" +
	"cmludCgi5L6n4pGg44CM55uY5LiK5pyJL+WjsOaYjuaXoOOAjT0g546w5YC86Ieq56CU6L+b56iLIOKIkiBga2luZD1sYXVuY2hk" +
	"YCDlo7DmmI7nmoTov5vnqIvnibnlvoHvvJvkvqfikaHjgIzlo7DmmI7mnIkv55uY5LiK5peg44CNPSBsYXVuY2hkIOWjsOaYjuea" +
	"hOS4ieeOr+e8uueOryIpCiAgICBwcmludCgi5ZCM5rqQIFMx77ya55yf5rqQID0gYGRlcGxveS/mnI3liqHlo7DmmI4udHN2YO+8" +
	"iOWjsOaYjuS+p+WUr+S4gOecn+a6kCDCtyDkuI7pl6jikacvYGNvcmUgZGFlbW9uIGxzYCDlkIzkuIDku7bvvInCtyDop6PmnpDl" +
	"maggPSDpl6jikacg55qEIGBwYXJzZV9kZWNsYCIpCiAgICBwcmludCgi5ZCM5rqQIFMy77ya5L6n4pGg5p2h5pWwID09IGB6ZXJn" +
	"IGRvY3RvcmDjgIwlc+OAjeadoeaVsO+8iGBUMmAg5Yik5o2u4pGiIMK3IOmAkOWtl+WQjOWAvO+8iSIgJSBET0NUT1JfSVRFTSkK" +
	"ICAgIHByaW50KCLlhpnml5fmoIfpl63pm4bvvIglZCDor40gwrcg5b2i5oCB5peg5YWz77yJ77yaJXMiICUgKGxlbihXUklURV9X" +
	"T1JEUyksICIgIi5qb2luKFdSSVRFX1dPUkRTKSkpCiAgICAjIOKXjyBHQVAtMjAyNjEwMDEtNDYg5L+u77ya5Y6f5byV55So5pyq" +
	"5a6a5LmJ5bi46YePIGBERVNUUlVDVElWRV9UT0tFTlNgIOKHkiBgLS1saXN0LXJ1bGVzYCDmgZIgTmFtZUVycm9yIOWbnua6ryBy" +
	"Yz0x44CCCiAgICAjICAg6YCQ5a2X5pS555So5pys5Lu2Kirlt7LlrprkuYkqKueahOa6kOeggee6p+eJmeivjeihqOW4uOmHj++8" +
	"iEFTVCDmiavmj4/nnJ/mraPor7vnmoTpgqPkuIDku73vvInjgIIKICAgIHByaW50KCLmupDnoIHnuqfniZnor43ooajvvJolcyAr" +
	"IGBvcGVuKOKApiwndyd8J2EnKWAgKyBnaXQg5YaZ5a2Q5ZG95Luk5a2X6Z2iIiAlICIgIi5qb2luKERFTEVURV9DQUxMUykpCiAg" +
	"ICBwcmludCgi6YCA56CB77yaMCDkuKTkvqflt7Llh7or5ZCM5rqQ6L+HIMK3IDEg5ZCM5rqQ5ryC56e7L+WGmemdoue6oiDCtyAy" +
	"IOS4jee7mee7k+iuuu+8iOe8uuS7ti/lj5bkuI3liLAv56m66L2sL+WGmeaXl+aghy/oh6rmo4DkuI3ov4fvvIkiKQogICAgcmV0" +
	"dXJuIDAKCgpkZWYgbWFpbihhcmd2PU5vbmUpOgogICAgYXJndiA9IGxpc3Qoc3lzLmFyZ3ZbMTpdIGlmIGFyZ3YgaXMgTm9uZSBl" +
	"bHNlIGFyZ3YpCiAgICBoaXRzID0gcGFyc2Vfd3JpdGVfZmxhZ3MoYXJndikKICAgIGlmIGhpdHM6CiAgICAgICAgcHJpbnQoIuKH" +
	"kiDmi5LvvIhyYz0yIMK3IOS4jee7mee7k+iuuu+8ie+8muWRveS7pOihjOmHjOWHuueOsOWGmeaXl+aghyAlcyIgJSBoaXRzKQog" +
	"ICAgICAgIHByaW50KCIgICDnkIbnlLHvvIhgRDRgIOadoeS7tumAkOWtl++8ie+8mioq5Y+q5YGa5Y+q6K+7IGRpZmbjgIHkuI3o" +
	"h6rliqjmuIXlub3ngbUqKiDigJTigJQg5bm954G155qE5YGcL+a4heaYr+S4jeWPr+mAhuWKqOS9nO+8jCIpCiAgICAgICAgcHJp" +
	"bnQoIiAgIOeFpyDCp+S5nSBNOSDkuI3ov5voh6rliqjlgJnpgInvvIjmnKzku7bov57jgIzmi7zlh7rpgqPmnaHlkb3ku6TjgI3n" +
	"moTot6/lvoTpg73kuI3nu5nvvInjgIIiKQogICAgICAgIHByaW50KCIgICDlj6ror7vnlKjms5XvvJpiYXNoIHNjcmlwdHMvZ2F0" +
	"ZXMvJXMgWy0tanNvbl0gWy0tcHJvdmUtcmVhZG9ubHldIiAlIFNFTEZfTkFNRSkKICAgICAgICByZXR1cm4gMgoKICAgIGFwID0g" +
	"YXJncGFyc2UuQXJndW1lbnRQYXJzZXIoZGVzY3JpcHRpb249IkQ0IOW5veeBtS/lo7DmmI7lt67lj6ror7sgZGlmZu+8iOS4pOS+" +
	"p+mAkOadoSDCtyDkuI3oh6rliqjmuIUv6KGl77yJIikKICAgIGFwLmFkZF9hcmd1bWVudCgiLS1yb290IiwgZGVmYXVsdD1ERUZB" +
	"VUxUX1JPT1QpCiAgICBhcC5hZGRfYXJndW1lbnQoIi0tZGVjbCIsIGRlZmF1bHQ9Tm9uZSwgaGVscD0i5aOw5piO5qCR77yI6buY" +
	"6K6kIDzku5PmoLk+L2RlcGxveS/mnI3liqHlo7DmmI4udHN277yb5Lmf5Y+v55SoIFpFUkdfU1ZDREVDTF9ERUNM77yJIikKICAg" +
	"IGFwLmFkZF9hcmd1bWVudCgiLS1wcyIsIGRlZmF1bHQ9Tm9uZSwgaGVscD0icHMg6L6T5Ye66JC95paH5Lu277yI6buY6K6k55yf" +
	"6LeRIHBz77yb5Lmf5Y+v55SoIFpFUkdfU1ZDREVDTF9QU++8iSIpCiAgICBhcC5hZGRfYXJndW1lbnQoIi0tbGF1bmNoY3RsIiwg" +
	"ZGVmYXVsdD1Ob25lLCBoZWxwPSJsYXVuY2hjdGwg6L6T5Ye66JC95paH5Lu277yI6buY6K6k55yf6LeR77yb5Lmf5Y+v55SoIFpF" +
	"UkdfU1ZDREVDTF9MQVVOQ0hDVEzvvIkiKQogICAgYXAuYWRkX2FyZ3VtZW50KCItLWhvbWUiLCBkZWZhdWx0PU5vbmUsIGhlbHA9" +
	"ImB+L2Ag5bGV5byA55So55qE5a6255uu5b2VIikKICAgIGFwLmFkZF9hcmd1bWVudCgiLS16ZXJnIiwgZGVmYXVsdD1Ob25lLCBo" +
	"ZWxwPSLlkb3ku6TpnaLlj6/ot5HliLblk4HvvIjpu5jorqQgPOS7k+aguT4vYmluL3plcmfvvIkiKQogICAgYXAuYWRkX2FyZ3Vt" +
	"ZW50KCItLWRvY3RvciIsIGRlZmF1bHQ9Tm9uZSwgaGVscD0iZG9jdG9yIEpTT04g6JC95paH5Lu277yI5aS55YW377yb6buY6K6k" +
	"55yf6LeRIGB6ZXJnIGRvY3RvciAtLWpzb24gJXNg77yJIiAlIERPQ1RPUl9GSUVMRFMpCiAgICBhcC5hZGRfYXJndW1lbnQoIi0t" +
	"anNvbiIsIGFjdGlvbj0ic3RvcmVfdHJ1ZSIpCiAgICBhcC5hZGRfYXJndW1lbnQoIi0tcHJvdmUtcmVhZG9ubHkiLCBhY3Rpb249" +
	"InN0b3JlX3RydWUiLCBoZWxwPSLot5HliY3lkI7lr7nmi43vvIjnnJ/mupDku7YgKyDpl6jikacg5rqQ5Lu2ICsg5pys5Lu2ICsg" +
	"LmdpdCDlhYPmlbDmja4gKyBwb3JjZWxhaW7vvIkiKQogICAgYXAuYWRkX2FyZ3VtZW50KCItLWxpc3QtcnVsZXMiLCBhY3Rpb249" +
	"InN0b3JlX3RydWUiKQogICAgYXAuYWRkX2FyZ3VtZW50KCItLXNlbGYtdGVzdCIsIGFjdGlvbj0ic3RvcmVfdHJ1ZSIpCiAgICBh" +
	"cC5hZGRfYXJndW1lbnQoIi0tdmVyc2lvbiIsIGFjdGlvbj0idmVyc2lvbiIsIHZlcnNpb249VkVSU0lPTikKICAgIGEgPSBhcC5w" +
	"YXJzZV9hcmdzKGFyZ3YpCgogICAgaWYgYS5zZWxmX3Rlc3Q6CiAgICAgICAgcmV0dXJuIHNlbGZfdGVzdCgpCiAgICBpZiBhLmxp" +
	"c3RfcnVsZXM6CiAgICAgICAgcmV0dXJuIGxpc3RfcnVsZXMoKQoKICAgICMg4pSA4pSAIOa6kOeggee6p+eJme+8iEQz4pGh77yJ" +
	"77ya55yf55uu5qCH5q615Ye6546w5Lu75L2V5YaZIEFQSSDih5Ig5YWI5Yik57qiIOKUgOKUgAogICAgc3JjID0gb3Blbihvcy5w" +
	"YXRoLmFic3BhdGgoX19maWxlX18pLCBlbmNvZGluZz0idXRmLTgiKS5yZWFkKCkKICAgIHdwID0gYXNzZXJ0X25vX3dyaXRlX2Fw" +
	"aShzcmMsIGN1dD1TRUxGVEVTVF9NQVJLRVIpCiAgICBpZiB3cDoKICAgICAgICBwcmludCgi4puUIOWIpOe6ou+8iHJjPTHvvInv" +
	"vJrmnKzku7YqKuecn+ebruagh+autSoq5Ye6546w5YaZIEFQSSDih5Ig44CM5LiN6Ieq5Yqo5YigL+S4jeiHquWKqOaUueOAjei/" +
	"meS4gOagvOegtOS6hu+8miIpCiAgICAgICAgZm9yIGggaW4gd3A6CiAgICAgICAgICAgIHByaW50KCIgICDCtyAlcyIgJSBoKQog" +
	"ICAgICAgIHJldHVybiAxCgogICAgbW9kLCBlcnIgPSBsb2FkX3NpYmxpbmcoKQogICAgaWYgZXJyOgogICAgICAgIHByaW50KCLi" +
	"m5Qg5LiN57uZ57uT6K6677yIcmM9Mu+8ie+8miVzIiAlIGVycikKICAgICAgICByZXR1cm4gMgoKICAgIHJvb3QgPSBvcy5wYXRo" +
	"LmFic3BhdGgoYS5yb290KQogICAgZGVjbF9wYXRoID0gYS5kZWNsIG9yIG9zLmVudmlyb24uZ2V0KCJaRVJHX1NWQ0RFQ0xfREVD" +
	"TCIpIG9yIG9zLnBhdGguam9pbihyb290LCAiZGVwbG95IiwgIuacjeWKoeWjsOaYji50c3YiKQogICAgZnBfYmVmb3JlID0gcmVh" +
	"ZG9ubHlfZmluZ2VycHJpbnQocm9vdCwgZGVjbF9wYXRoKSBpZiBhLnByb3ZlX3JlYWRvbmx5IGVsc2UgTm9uZQogICAgcmMsIHNp" +
	"ZGUxLCBzaWRlMiwgbm90ZXMsIHByb2JsZW1zID0gY2hlY2socm9vdCwgbW9kLCBkZWNsX3BhdGg9YS5kZWNsLCBwc19maWxlPWEu" +
	"cHMsCiAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICBsYXVuY2hjdGxfZmlsZT1hLmxhdW5jaGN0" +
	"bCwgaG9tZT1hLmhvbWUsIHplcmc9YS56ZXJnLAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAg" +
	"ZG9jdG9yX2ZpbGU9YS5kb2N0b3IpCiAgICBpZiByYyA9PSAyIGFuZCBub3Qgc2lkZTEgYW5kIG5vdCBzaWRlMjoKICAgICAgICBw" +
	"cmludCgi4puUIOS4jee7mee7k+iuuu+8iHJjPTLvvInvvJoiKQogICAgICAgIGZvciBwIGluIHByb2JsZW1zOgogICAgICAgICAg" +
	"ICBwcmludCgiICAgwrcgJXMiICUgcCkKICAgICAgICByZXR1cm4gMgoKICAgIGlmIGEuanNvbjoKICAgICAgICBwcmludChqc29u" +
	"LmR1bXBzKHsic2NoZW1hIjogInplcmcvdjEiLCAia2luZCI6ICJHaG9zdERlY2xEaWZmIiwKICAgICAgICAgICAgICAgICAgICAg" +
	"ICAgICAiaXRlbXMiOiBbeyJzaWRlIjogIuebmOS4iuaciS/lo7DmmI7ml6AiLCAqKml0fSBmb3IgaXQgaW4gc2lkZTFdICsKICAg" +
	"ICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICBbeyJzaWRlIjogIuWjsOaYjuaciS/nm5jkuIrml6AiLCAqKml0fSBmb3Ig" +
	"aXQgaW4gc2lkZTJdLAogICAgICAgICAgICAgICAgICAgICAgICAgICJtZXRhIjogeyJjb3VudCI6IGxlbihzaWRlMSkgKyBsZW4o" +
	"c2lkZTIpLCAic2lkZTEiOiBsZW4oc2lkZTEpLCAic2lkZTIiOiBsZW4oc2lkZTIpLAogICAgICAgICAgICAgICAgICAgICAgICAg" +
	"ICAgICAgICAgICJkZWNsIjogZGVjbF9wYXRoLCAiZGVjbF9zaGEyNTYiOiBmaWxlX3NoYShkZWNsX3BhdGgpIGlmIG9zLnBhdGgu" +
	"aXNmaWxlKGRlY2xfcGF0aCkgZWxzZSBOb25lLAogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICJkb2N0b3JfaXRl" +
	"bSI6IERPQ1RPUl9JVEVNLCAidG9vbCI6IFZFUlNJT059LAogICAgICAgICAgICAgICAgICAgICAgICAgICJ3YXJuaW5ncyI6IFtd" +
	"LCAidHJ1bmNhdGVkIjogRmFsc2UsICJub3RlcyI6IG5vdGVzLCAicHJvYmxlbXMiOiBwcm9ibGVtc30sCiAgICAgICAgICAgICAg" +
	"ICAgICAgICAgICBlbnN1cmVfYXNjaWk9RmFsc2UpKQogICAgZWxzZToKICAgICAgICBwcmludF9yZXBvcnQocm9vdCwgZGVjbF9w" +
	"YXRoLCByYywgc2lkZTEsIHNpZGUyLCBub3RlcywgcHJvYmxlbXMpCgogICAgaWYgYS5wcm92ZV9yZWFkb25seToKICAgICAgICBm" +
	"cF9hZnRlciA9IHJlYWRvbmx5X2ZpbmdlcnByaW50KHJvb3QsIGRlY2xfcGF0aCkKICAgICAgICBkaWZmcyA9IFsoaywgZnBfYmVm" +
	"b3JlLmdldChrKSwgZnBfYWZ0ZXIuZ2V0KGspKSBmb3IgayBpbiBzb3J0ZWQoc2V0KGZwX2JlZm9yZSkgfCBzZXQoZnBfYWZ0ZXIp" +
	"KQogICAgICAgICAgICAgICAgIGlmIGZwX2JlZm9yZS5nZXQoaykgIT0gZnBfYWZ0ZXIuZ2V0KGspXQogICAgICAgIHByaW50KCLi" +
	"lIDilIAg5Y+q6K+76Ieq6K+B77yIRDLvvInvvJrliY0gJWQg5Lu2IC8g5ZCOICVkIOS7tiDCtyDlt67lvIIgJWQg5Lu2IOKUgOKU" +
	"gCIgJSAobGVuKGZwX2JlZm9yZSksIGxlbihmcF9hZnRlciksIGxlbihkaWZmcykpKQogICAgICAgIGZvciBrLCBiLCB4IGluIGRp" +
	"ZmZzOgogICAgICAgICAgICBwcmludCgiICAgICAg4pyXICVz77yaJXMg4oeSICVzIiAlIChrLCAoYiBvciAiIilbOjE2XSwgKHgg" +
	"b3IgIiIpWzoxNl0pKQogICAgICAgIGlmIGRpZmZzOgogICAgICAgICAgICBwcmludCgiICDih5IgcmM9Me+8mui3keacrOS7tuac" +
	"n+mXtOS7k+WGheS7tiBzaGEyNTYg5Y+Y5LqG77yI6YCQ5Lu26KeB5LiK77yJIikKICAgICAgICAgICAgcmV0dXJuIDEKICAgICAg" +
	"ICBwcmludCgiICDih5Ig6Zu25Ymv5L2c55So77ya55yf5rqQ5Lu2ICsg6Zeo4pGnIOa6kOS7tiArIOacrOS7tiArIGAuZ2l0YCDl" +
	"hYPmlbDmja4gKyBwb3JjZWxhaW4g6YCQ5Lu25LiN5Y+Y77yI5beu5byCIDAg5Lu277yJIikKICAgIHJldHVybiByYwoKCiMg4pSA" +
	"4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSAIOiHquajgO+8iOaIkOWvuei0n+aO" +
	"pyDCtyDlkIjmiJDlpLnlhbcgwrcg5LiN56Kw55yf55uu5qCH77yJ4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA4pSA" +
	"4pSA4pSA4pSA4pSA4pSA4pSA4pSACkdPT0RfREVDTCA9ICgKICAgICIjIOWkueWFt+WjsOaYjuagkVxuIgogICAgImtpbmRcdG5h" +
	"bWVcdOWjsOaYjuS7tlx056iL5bqPXHTov5vnqIvnibnlvoFcdOW9kuWxnlx05aSE572uXG4iCiAgICAibGF1bmNoZFx0Y29tLnpl" +
	"cmcuY29yZVx0fi9MaWJyYXJ5L0xhdW5jaEFnZW50cy9jb20uemVyZy5jb3JlLnBsaXN0XHQvYmluL2Jhc2ggc2NyaXB0cy9zdmMv" +
	"emVyZy1jb3JlLWRhZW1vbi5zaFx0YmluL3plcmctY29yZVx05Li75o6nXHQtXG4iCiAgICAibGF1bmNoZFx0Y29tLnplcmcuYWdl" +
	"bnQtbXMwMVx0fi9MaWJyYXJ5L0xhdW5jaEFnZW50cy9jb20uemVyZy5hZ2VudC1tczAxLnBsaXN0XHRiaW4vemVyZy1hZ2VudGQg" +
	"LS1tYWNoaW5lIG1zMDFcdGJpbi96ZXJnLWFnZW50ZFx05a2Q56uvXHQtXG4iCiAgICAiZ2hvc3RcdGNvY29vbi1kb2NzLXNlcnZp" +
	"Y2VcdC1cdGJpbi9jb2Nvb24tZG9jcy1zZXJ2aWNlIC1wb3J0IDg2MTBcdGJpbi9jb2Nvb24tZG9jcy1zZXJ2aWNlXHTmlofmoaPo" +
	"jKdcdOi/myByZWFwIOWAmemAiSDCtyDlvoUgTXIyMTA5IOaLjVxuIgopCkdPT0RfUFMgPSAoCiAgICAiICA2ODkzOSAgICAgMSAg" +
	"ICAgICA1NjozNiAvcmVwby9iaW4vemVyZy1jb3JlXG4iCiAgICAiICAxNDE0OSAgICAgMSAwMy0xNTozMDo0MCAvcmVwby9iaW4v" +
	"emVyZy1hZ2VudGQgLS1ob3N0IDEyNy4wLjAuMSAtLXBvcnQgODEwMCAtLW1hY2hpbmUgbXMwMVxuIgogICAgIiAgNzkzMTcgICAg" +
	"IDEgMDQtMjI6MzQ6MzIgL3JlcG8vYmluL2NvY29vbi1kb2NzLXNlcnZpY2UgLXBvcnQgODYxMFxuIgopCkdPT0RfTEMgPSAiNjg5" +
	"MzlcdDBcdGNvbS56ZXJnLmNvcmVcbjE0MTQ5XHQwXHRjb20uemVyZy5hZ2VudC1tczAxXG4iCgoKZGVmIF9kb2N0b3JfanNvbihu" +
	"LCBkZXRhaWw9Tm9uZSk6CiAgICBkZXRhaWwgPSBkZXRhaWwgaWYgZGV0YWlsIGlzIG5vdCBOb25lIGVsc2UgKCIlZCDmnaHvvJpw" +
	"aWQ9NzkzMTcgL3JlcG8vYmluL2NvY29vbi1kb2NzLXNlcnZpY2UgLXBvcnQgODYxMCIgJSBuKQogICAgcmV0dXJuIGpzb24uZHVt" +
	"cHMoeyJzY2hlbWEiOiAiemVyZy92MSIsICJraW5kIjogIkRvY3RvckNoZWNrIiwKICAgICAgICAgICAgICAgICAgICAgICAiaXRl" +
	"bXMiOiBbeyJuYW1lIjogRE9DVE9SX0lURU0sICJ2ZXJkaWN0IjogIlJFUE9SVCIgaWYgbiBlbHNlICJQQVNTIiwKICAgICAgICAg" +
	"ICAgICAgICAgICAgICAgICAgICAgICAgICJkZXRhaWwiOiBkZXRhaWwsICJhZHZpY2UiOiAiIn1dfSwgZW5zdXJlX2FzY2lpPUZh" +
	"bHNlKQoKCmRlZiBfZml4dHVyZShkLCBkZWNsPUdPT0RfREVDTCwgcHM9R09PRF9QUywgbGM9R09PRF9MQywgcGxpc3RzPSgiY29t" +
	"LnplcmcuY29yZS5wbGlzdCIsICJjb20uemVyZy5hZ2VudC1tczAxLnBsaXN0IikpOgogICAgb3MubWFrZWRpcnMoZCwgZXhpc3Rf" +
	"b2s9VHJ1ZSkKICAgIGRwID0gb3MucGF0aC5qb2luKGQsICLmnI3liqHlo7DmmI4udHN2IikKICAgIHdpdGggb3BlbihkcCwgInci" +
	"LCBlbmNvZGluZz0idXRmLTgiKSBhcyBmOgogICAgICAgIGYud3JpdGUoZGVjbCkKICAgIHBwID0gb3MucGF0aC5qb2luKGQsICJw" +
	"cy50eHQiKQogICAgd2l0aCBvcGVuKHBwLCAidyIsIGVuY29kaW5nPSJ1dGYtOCIpIGFzIGY6CiAgICAgICAgZi53cml0ZShwcykK" +
	"ICAgIGxwID0gb3MucGF0aC5qb2luKGQsICJsYXVuY2hjdGwudHh0IikKICAgIHdpdGggb3BlbihscCwgInciLCBlbmNvZGluZz0i" +
	"dXRmLTgiKSBhcyBmOgogICAgICAgIGYud3JpdGUobGMpCiAgICBob21lID0gb3MucGF0aC5qb2luKGQsICJob21lIikKICAgIGxh" +
	"ID0gb3MucGF0aC5qb2luKGhvbWUsICJMaWJyYXJ5IiwgIkxhdW5jaEFnZW50cyIpCiAgICBvcy5tYWtlZGlycyhsYSwgZXhpc3Rf" +
	"b2s9VHJ1ZSkKICAgIGZvciBuIGluIHBsaXN0czoKICAgICAgICB3aXRoIG9wZW4ob3MucGF0aC5qb2luKGxhLCBuKSwgInciLCBl" +
	"bmNvZGluZz0idXRmLTgiKSBhcyBmOgogICAgICAgICAgICBmLndyaXRlKCJwbGlzdFxuIikKICAgICMg5aS55YW344CM5LuT5qC5" +
	"44CN77ya6K6pIHJlYWRvbmx5X2ZpbmdlcnByaW50IOaciSAuZ2l0IOWFg+aVsOaNrumdouWPr+eci++8iOS4jemcgOimgeecn+S7" +
	"k++8iQogICAgb3MubWFrZWRpcnMob3MucGF0aC5qb2luKGQsICIuZ2l0IiksIGV4aXN0X29rPVRydWUpCiAgICBmb3IgbiBpbiAo" +
	"IkhFQUQiLCAiaW5kZXgiKToKICAgICAgICB3aXRoIG9wZW4ob3MucGF0aC5qb2luKGQsICIuZ2l0IiwgbiksICJ3IiwgZW5jb2Rp" +
	"bmc9InV0Zi04IikgYXMgZjoKICAgICAgICAgICAgZi53cml0ZSgicmVmOiByZWZzL2hlYWRzL21haW5cbiIpCiAgICByZXR1cm4g" +
	"ZHAsIHBwLCBscCwgaG9tZQoKCmRlZiBzZWxmX3Rlc3QoKToKICAgIG9rID0gVHJ1ZQogICAgdG1wID0gdGVtcGZpbGUubWtkdGVt" +
	"cChwcmVmaXg9ImdkZC1zZWxmdGVzdC0iKQogICAgbW9kLCBlcnIgPSBsb2FkX3NpYmxpbmcoKQogICAgaWYgZXJyOgogICAgICAg" +
	"IHByaW50KCIgIOKclyDlkIzmupDku7bliqDovb3lpLHotKXvvJolcyIgJSBlcnIpCiAgICAgICAgcmV0dXJuIDEKCiAgICBkZWYg" +
	"Y2FzZShuYW1lLCB3YW50LCBnb3QsIG5vdGU9IiIpOgogICAgICAgIG5vbmxvY2FsIG9rCiAgICAgICAgZ29vZCA9IHdhbnQgPT0g" +
	"Z290CiAgICAgICAgb2sgPSBvayBhbmQgZ29vZAogICAgICAgIHByaW50KCIgICVzICVz77yIJXMgwrcg5pyf5pybICVz77yJJXMi" +
	"ICUgKCLinJMiIGlmIGdvb2QgZWxzZSAi4pyXIiwgbmFtZSwgZ290LCB3YW50LCBub3RlKSkKCiAgICAjIOKRoCDmraPkvovvvJrk" +
	"vqfikaAgMSDmnaHvvIjlt7LnmbvorrAgZ2hvc3TvvInCtyDkvqfikaEgMCDmnaEgwrcgZG9jdG9yIOWQjOWAvCDih5IgcmM9MAog" +
	"ICAgZDEgPSBvcy5wYXRoLmpvaW4odG1wLCAiZjEiKQogICAgZHAsIHBwLCBscCwgaG9tZSA9IF9maXh0dXJlKGQxKQogICAgZGYg" +
	"PSBvcy5wYXRoLmpvaW4oZDEsICJkb2N0b3IuanNvbiIpCiAgICB3aXRoIG9wZW4oZGYsICJ3IiwgZW5jb2Rpbmc9InV0Zi04Iikg" +
	"YXMgZjoKICAgICAgICBmLndyaXRlKF9kb2N0b3JfanNvbigxKSkKICAgIHJjLCBzMSwgczIsIG5vdGVzLCBwcm9icyA9IGNoZWNr" +
	"KGQxLCBtb2QsIGRlY2xfcGF0aD1kcCwgcHNfZmlsZT1wcCwgbGF1bmNoY3RsX2ZpbGU9bHAsIGhvbWU9aG9tZSwgZG9jdG9yX2Zp" +
	"bGU9ZGYpCiAgICBjYXNlKCLikaAg5q2j5L6L77ya5L6n4pGgIDEg5p2hIC8g5L6n4pGhIDAg5p2hIC8gZG9jdG9yIOWQjOWAvCDi" +
	"h5IgcmM9MCIsIDAsIHJjLCBub3RlPSIgIHMxPSVkIHMyPSVkIiAlIChsZW4oczEpLCBsZW4oczIpKSkKICAgIGNhc2UoIuKRoGIg" +
	"5L6n4pGg6YKj5p2h6KKr5qCH5oiQ44CM5bey55m76K6wIGdob3N044CN77yI5LiN5piv5pyq55m76K6w5bm954G177yJIiwgImNv" +
	"Y29vbi1kb2NzLXNlcnZpY2UiLAogICAgICAgICAoczFbMF1bInJlZ2lzdGVyZWRfYXMiXSBpZiBzMSBlbHNlIE5vbmUpKQogICAg" +
	"Y2FzZSgi4pGgYyDkuKTkvqfpg73kuI3pnZnpu5jvvJrkvqfikaEg56m65Lmf6KaB5pyJIDAg5p2h55qE5b2i5oCBIiwgMCwgbGVu" +
	"KHMyKSkKCiAgICAjIOKRoSDotJ/mjqfvvJpkb2N0b3Ig6K+0IDAg5p2h77yI5LiO5L6n4pGgIDEg5p2h5LiN562J77yJ4oeSIOWQ" +
	"jOa6kOa8guenuyByYz0x77yI55yf57qi77yJCiAgICBkZjAgPSBvcy5wYXRoLmpvaW4oZDEsICJkb2N0b3IwLmpzb24iKQogICAg" +
	"d2l0aCBvcGVuKGRmMCwgInciLCBlbmNvZGluZz0idXRmLTgiKSBhcyBmOgogICAgICAgIGYud3JpdGUoX2RvY3Rvcl9qc29uKDAs" +
	"IGRldGFpbD0i546w5YC86Z2i5rKh55yL5Yiw44CM5aOw5piO5aSW44CN55qE5pyN5Yqh6L+b56iLIikpCiAgICByYzAsIF9zMSwg" +
	"X3MyLCBfbiwgcHJvYnMwID0gY2hlY2soZDEsIG1vZCwgZGVjbF9wYXRoPWRwLCBwc19maWxlPXBwLCBsYXVuY2hjdGxfZmlsZT1s" +
	"cCwgaG9tZT1ob21lLCBkb2N0b3JfZmlsZT1kZjApCiAgICBjYXNlKCLikaEg6LSf5o6n77yaZG9jdG9yIDAg5p2hIOKJoCDkvqfi" +
	"kaAgMSDmnaEg4oeSIOWQjOa6kOa8guenuyByYz0xIiwgMSwgcmMwLAogICAgICAgICBub3RlPSIgICIgKyAocHJvYnMwWzBdWzo2" +
	"MF0gaWYgcHJvYnMwIGVsc2UgIiIpKQoKICAgICMg4pGiIOS+p+KRoSDpnZ7nqbrvvIjlo7DmmI7ku7bkuI3lnKjnm5jvvInih5Ig" +
	"6YCQ5p2h54K55ZCN77yM5LiUKirmiqXlt67kuI3pqbHliqjpgIDnoIEqKu+8iOS7jSByYz0w77yJCiAgICBkMyA9IG9zLnBhdGgu" +
	"am9pbih0bXAsICJmMyIpCiAgICBkcDMsIHBwMywgbHAzLCBob21lMyA9IF9maXh0dXJlKGQzLCBwbGlzdHM9KCJjb20uemVyZy5j" +
	"b3JlLnBsaXN0IiwpKQogICAgZGYzID0gb3MucGF0aC5qb2luKGQzLCAiZG9jdG9yLmpzb24iKQogICAgd2l0aCBvcGVuKGRmMywg" +
	"InciLCBlbmNvZGluZz0idXRmLTgiKSBhcyBmOgogICAgICAgIGYud3JpdGUoX2RvY3Rvcl9qc29uKDEpKQogICAgcmMzLCBzMTMs" +
	"IHMyMywgX24zLCBfcDMgPSBjaGVjayhkMywgbW9kLCBkZWNsX3BhdGg9ZHAzLCBwc19maWxlPXBwMywgbGF1bmNoY3RsX2ZpbGU9" +
	"bHAzLCBob21lPWhvbWUzLCBkb2N0b3JfZmlsZT1kZjMpCiAgICBjYXNlKCLikaIg5L6n4pGhIOmdnuepuu+8iOWjsOaYjuS7tuS4" +
	"jeWcqOebmO+8ieKHkiDpgJDmnaHliJcgKyByYyDku40gMO+8iOaKpeW3ruS4jempseWKqOmAgOegge+8iSIsICgwLCAxKSwgKHJj" +
	"MywgbGVuKHMyMykpLAogICAgICAgICBub3RlPSIgIOe8uueOryAlcyIgJSAoczIzWzBdWyJtaXNzaW5nIl0gaWYgczIzIGVsc2Ug" +
	"IuKAlCIpKQoKICAgICMg4pGjIOi0n+aOp++8iOa6kOeggee6p+eJme+8ie+8muWQiOaIkOa6kOeggeWQqyBgb3MucmVtb3ZlKGAg" +
	"4oeSIOW/heWRveS4re+8m+acrOS7tuecn+ebruagh+autSDih5IgMCDlkb3kuK0KICAgIGZha2UgPSAiaW1wb3J0IG9zXG5kZWYg" +
	"ZihwKTpcbiAgICBvcy5yZW1vdmUocClcbiIKICAgIGNhc2UoIuKRoyDotJ/mjqfvvIjmupDnoIHnuqfniZnvvInvvJrlkIjmiJDm" +
	"upDnoIHlkKsgYG9zLnJlbW92ZShgIOKHkiDlkb3kuK0iLCBUcnVlLCBib29sKGFzc2VydF9ub193cml0ZV9hcGkoZmFrZSkpKQog" +
	"ICAgc3JjID0gb3Blbihvcy5wYXRoLmFic3BhdGgoX19maWxlX18pLCBlbmNvZGluZz0idXRmLTgiKS5yZWFkKCkKICAgIGhpdHMg" +
	"PSBhc3NlcnRfbm9fd3JpdGVfYXBpKHNyYywgY3V0PVNFTEZURVNUX01BUktFUikKICAgIGNhc2UoIuKRo2Ig5q2j5L6L77ya5pys" +
	"5Lu2KirnnJ/nm67moIfmrrUqKuWGmSBBUEkg5ZG95Lit5pWwID0gMO+8iOiHquajgOWkueWFt+auteixgeWFje+8iSIsIDAsIGxl" +
	"bihoaXRzKSwKICAgICAgICAgbm90ZT0oIiAgIiArICIgLyAiLmpvaW4oaGl0c1s6M10pKSBpZiBoaXRzIGVsc2UgIiIpCiAgICBj" +
	"YXNlKCLikaNjIOato+S+i++8muWQjOS4gOS7vea6kOeggSoq5pW056+HKirmiavvvIjlkKvoh6rmo4DmrrXvvInlupTmnInlkb3k" +
	"uK0g4oeSIOivgeaYjuaJq+aPj+WZqOS4jeaYr+aBkuepuiIsCiAgICAgICAgIFRydWUsIGJvb2woYXNzZXJ0X25vX3dyaXRlX2Fw" +
	"aShzcmMpKSkKCiAgICAjIOKRpCDotJ/mjqfvvJrlhpnml5fmoIfkuIDlvovmi5IgcmM9Mu+8iOi1sOecnyBDTEkg5YWl5Y+j77yJ" +
	"CiAgICBpbXBvcnQgY29udGV4dGxpYgogICAgaW1wb3J0IGlvCiAgICBmb3IgZmxhZyBpbiAoIi0tcmVhcCIsICItLWNsZWFuIiwg" +
	"Ii0tZml4IiwgImNvbW1pdCIsICItLXN0b3AiLCAiLS1raWxsIiwgIi0tYXBwbHkiKToKICAgICAgICBidWYgPSBpby5TdHJpbmdJ" +
	"TygpCiAgICAgICAgd2l0aCBjb250ZXh0bGliLnJlZGlyZWN0X3N0ZG91dChidWYpOgogICAgICAgICAgICByY2YgPSBtYWluKFtm" +
	"bGFnXSkKICAgICAgICBjYXNlKCLikaQg6LSf5o6n77ya5YaZ5peX5qCHICVyIOKHkiDmi5IgcmM9MiIgJSBmbGFnLCAyLCByY2Yp" +
	"CgogICAgIyDikaUg57y65Lu277ya5aOw5piO5qCR5LiN5a2Y5ZyoIOKHkiByYz0yCiAgICByYzYsIF9hLCBfYiwgX2MsIHA2ID0g" +
	"Y2hlY2soZDEsIG1vZCwgZGVjbF9wYXRoPW9zLnBhdGguam9pbihkMSwgIuS4jeWtmOWcqC50c3YiKSwgcHNfZmlsZT1wcCwKICAg" +
	"ICAgICAgICAgICAgICAgICAgICAgICAgICAgICBsYXVuY2hjdGxfZmlsZT1scCwgaG9tZT1ob21lLCBkb2N0b3JfZmlsZT1kZikK" +
	"ICAgIGNhc2UoIuKRpSDnvLrku7bvvIjlo7DmmI7moJHkuI3lnKjvvInih5IgcmM9MiDkuI3nu5nnu5PorroiLCAyLCByYzYsIG5v" +
	"dGU9IiAgIiArIChwNlswXVs6NDBdIGlmIHA2IGVsc2UgIiIpKQoKICAgICMg4pGmIOepuui9rO+8mnBzIOmHjOayoeacieiHqueg" +
	"lOS7tiDih5IgcmM9Mu+8iOS4jeiuuOW9k+e7v++8iQogICAgcHA3ID0gb3MucGF0aC5qb2luKGQxLCAicHMtZW1wdHkudHh0IikK" +
	"ICAgIHdpdGggb3BlbihwcDcsICJ3IiwgZW5jb2Rpbmc9InV0Zi04IikgYXMgZjoKICAgICAgICBmLndyaXRlKCIgICAgMSAgICAg" +
	"MCAgMDA6MDAgL3NiaW4vbGF1bmNoZFxuIikKICAgIHJjNywgX2EsIF9iLCBfYywgcDcgPSBjaGVjayhkMSwgbW9kLCBkZWNsX3Bh" +
	"dGg9ZHAsIHBzX2ZpbGU9cHA3LCBsYXVuY2hjdGxfZmlsZT1scCwgaG9tZT1ob21lLCBkb2N0b3JfZmlsZT1kZikKICAgIGNhc2Uo" +
	"IuKRpiDnqbrovazvvIhwcyDph4wgMCDku7boh6rnoJTku7bvvInih5IgcmM9MiIsIDIsIHJjNywgbm90ZT0iICAiICsgKHA3WzBd" +
	"WzozMF0gaWYgcDcgZWxzZSAiIikpCgogICAgIyDikacgbGF1bmNoY3RsIOWPluS4jeWIsO+8iOmdniBtYWNPU++8ieKHkiByYz0y" +
	"CiAgICByYzgsIF9hLCBfYiwgX2MsIHA4ID0gY2hlY2soZDEsIG1vZCwgZGVjbF9wYXRoPWRwLCBwc19maWxlPXBwLAogICAgICAg" +
	"ICAgICAgICAgICAgICAgICAgICAgICAgIGxhdW5jaGN0bF9maWxlPW9zLnBhdGguam9pbihkMSwgIuayoeaciS1sYXVuY2hjdGwu" +
	"dHh0IiksIGhvbWU9aG9tZSwgZG9jdG9yX2ZpbGU9ZGYpCiAgICBjYXNlKCLikacgbGF1bmNoY3RsIOWPluS4jeWIsCDih5IgcmM9" +
	"Mu+8iOS4jeW9k+e7v++8iSIsIDIsIHJjOCwgbm90ZT0iICAiICsgKHA4WzBdWzo0MF0gaWYgcDggZWxzZSAiIikpCgogICAgIyDi" +
	"kagg5pyq55m76K6w5bm954G177yIcHMg5aSa5LiA5Lu244CB5aOw5piO6YeM5rKh5pyJ77yJ77ya5L6n4pGgIOWPmCAyIOadoe+8" +
	"iOS4gOeZu+iusOS4gOacqueZu+iusO+8ie+8m1MyIOWPquavlCoq5bey55m76K6w5a2Q6ZuGKirvvIgxIOadoe+8iQogICAgIyAg" +
	"IOKHkiDkuI4gZG9jdG9yIDEg5p2h5ZCM5YC8IOKHkiByYz0w77yI4piFIEdBUC0yMDI2MDkyNy02MihiKe+8muacqueZu+iusOW5" +
	"veeBteS4jeWPguS4jiBTMu+8jOaXp+WPo+W+hCBkb2N0b3IgMiDmnaHmraTmoLzlv4XnuqLvvIkKICAgIHBwOSA9IG9zLnBhdGgu" +
	"am9pbihkMSwgInBzLWV4dHJhLnR4dCIpCiAgICB3aXRoIG9wZW4ocHA5LCAidyIsIGVuY29kaW5nPSJ1dGYtOCIpIGFzIGY6CiAg" +
	"ICAgICAgZi53cml0ZShHT09EX1BTICsgIiAgODg4ODggICAgIDEgICAgICAgMTA6MDAgL3JlcG8vYmluL3plcmctbXlzdGVyeSAt" +
	"LWJvb3RcbiIpCiAgICBkZjkgPSBvcy5wYXRoLmpvaW4oZDEsICJkb2N0b3I5Lmpzb24iKQogICAgd2l0aCBvcGVuKGRmOSwgInci" +
	"LCBlbmNvZGluZz0idXRmLTgiKSBhcyBmOgogICAgICAgIGYud3JpdGUoX2RvY3Rvcl9qc29uKDEpKQogICAgcmM5LCBzMTksIF9i" +
	"LCBfYywgX3A5ID0gY2hlY2soZDEsIG1vZCwgZGVjbF9wYXRoPWRwLCBwc19maWxlPXBwOSwgbGF1bmNoY3RsX2ZpbGU9bHAsIGhv" +
	"bWU9aG9tZSwgZG9jdG9yX2ZpbGU9ZGY5KQogICAgY2FzZSgi4pGoIOacqueZu+iusOW5veeBte+8muS+p+KRoCAyIOadoe+8iOS4" +
	"gOeZu+iusOS4gOacqueZu+iusO+8icK3IFMyIOavlOW3sueZu+iusOWtkOmbhiAxIOadoSA9PSBkb2N0b3IgMSDmnaEg4oeSIHJj" +
	"PTAiLCAoMCwgMiwgMSksIChyYzksIGxlbihzMTkpLAogICAgICAgICBzdW0oMSBmb3IgaXQgaW4gczE5IGlmIGl0WyJyZWdpc3Rl" +
	"cmVkX2FzIl0gaXMgTm9uZSkpKQoKICAgICMg4pGqIFBQSUQg5rex5bqmMeebtOezu+ixgeWFje+8iOKYhSBHQVAtMjAyNjA5Mjct" +
	"NjIoYSnvvInvvJp6ZXJnLWNvcmUo5aOw5piO5pyN5YqhIMK3IHBpZD02ODkzOSkg55qEKirnm7TmjqXlrZAqKgogICAgIyAgIHRh" +
	"c2sg6L+b56iL77yIcHBpZD02ODkzOe+8ieS4jeeul+acqueZu+iusOW5veeBtSDih5Ig5L6n4pGgIOS7jSAxIOadoe+8iOWPqiBj" +
	"b2Nvb27vvInCtyDmnKrnmbvorrAgMCDmnaEg4oeSIHJjPTAKICAgIHBwMTEgPSBvcy5wYXRoLmpvaW4oZDEsICJwcy1jaGlsZC50" +
	"eHQiKQogICAgd2l0aCBvcGVuKHBwMTEsICJ3IiwgZW5jb2Rpbmc9InV0Zi04IikgYXMgZjoKICAgICAgICBmLndyaXRlKEdPT0Rf" +
	"UFMgKyAiICA3Nzc3NyAgICAgNjg5MzkgICAwMDowNSAvcmVwby9iaW4vemVyZy10YXNrd29ya2VyIC0tc2NvcGVkXG4iKQogICAg" +
	"ZGYxMSA9IG9zLnBhdGguam9pbihkMSwgImRvY3Rvci1jaGlsZC5qc29uIikKICAgIHdpdGggb3BlbihkZjExLCAidyIsIGVuY29k" +
	"aW5nPSJ1dGYtOCIpIGFzIGY6CiAgICAgICAgZi53cml0ZShfZG9jdG9yX2pzb24oMSkpCiAgICByYzExLCBzMTExLCBfYiwgX2Ms" +
	"IF9wMTEgPSBjaGVjayhkMSwgbW9kLCBkZWNsX3BhdGg9ZHAsIHBzX2ZpbGU9cHAxMSwgbGF1bmNoY3RsX2ZpbGU9bHAsIGhvbWU9" +
	"aG9tZSwgZG9jdG9yX2ZpbGU9ZGYxMSkKICAgIGNhc2UoIuKRqiBQUElEIOa3seW6pjHnm7Tns7vosYHlhY3vvJrlo7DmmI7mnI3l" +
	"iqHnm7TmjqXlrZAgdGFzayDov5vnqIvkuI3lhaXkvqfikaAg4oeSICgwLDEsMCkiLCAoMCwgMSwgMCksCiAgICAgICAgIChyYzEx" +
	"LCBsZW4oczExMSksIHN1bSgxIGZvciBpdCBpbiBzMTExIGlmIGl0WyJyZWdpc3RlcmVkX2FzIl0gaXMgTm9uZSkpKQoKICAgICMg" +
	"4pGqYiDotJ/mjqfvvJrlj6rosYHlhY0qKua3seW6piAxKiog4oCU4oCUIOWtmei/m+eoi++8iOeItj3lrZDov5vnqIsgcGlkIDc3" +
	"Nzc3IMK3IOmdnuWjsOaYju+8ieS4jueItumdnuWjsOaYjueahOaVo+S7tuS7jeiuoeacqueZu+iusAogICAgcHAxMWIgPSBvcy5w" +
	"YXRoLmpvaW4oZDEsICJwcy1zdHJheS50eHQiKQogICAgd2l0aCBvcGVuKHBwMTFiLCAidyIsIGVuY29kaW5nPSJ1dGYtOCIpIGFz" +
	"IGY6CiAgICAgICAgZi53cml0ZShHT09EX1BTICsgIiAgNzc3NzcgICAgIDY4OTM5ICAgMDA6MDUgL3JlcG8vYmluL3plcmctdGFz" +
	"a3dvcmtlciAtLXNjb3BlZFxuIgogICAgICAgICAgICAgICAgICAgICAgICAgICIgIDg4ODg4ICAgICA3Nzc3NyAgIDAwOjA1IC9y" +
	"ZXBvL2Jpbi96ZXJnLWdyYW5kIC0tZGVlcFxuIgogICAgICAgICAgICAgICAgICAgICAgICAgICIgIDY2NjY2ICAgICAgIDEgICAg" +
	"IDAxOjAwIC9yZXBvL2Jpbi96ZXJnLXN0cmF5IC0tYm9vdFxuIikKICAgIGRmMTFiID0gb3MucGF0aC5qb2luKGQxLCAiZG9jdG9y" +
	"LXN0cmF5Lmpzb24iKQogICAgd2l0aCBvcGVuKGRmMTFiLCAidyIsIGVuY29kaW5nPSJ1dGYtOCIpIGFzIGY6CiAgICAgICAgZi53" +
	"cml0ZShfZG9jdG9yX2pzb24oMSkpCiAgICByYzExYiwgczExMWIsIF9iLCBfYywgX3AxMWIgPSBjaGVjayhkMSwgbW9kLCBkZWNs" +
	"X3BhdGg9ZHAsIHBzX2ZpbGU9cHAxMWIsIGxhdW5jaGN0bF9maWxlPWxwLCBob21lPWhvbWUsIGRvY3Rvcl9maWxlPWRmMTFiKQog" +
	"ICAgY2FzZSgi4pGqYiDotJ/mjqfvvJrlrZnov5vnqIsv54i26Z2e5aOw5piOIOS7jeiuoeacqueZu+iusCDih5IgKDAsMywyKSIs" +
	"ICgwLCAzLCAyKSwKICAgICAgICAgKHJjMTFiLCBsZW4oczExMWIpLCBzdW0oMSBmb3IgaXQgaW4gczExMWIgaWYgaXRbInJlZ2lz" +
	"dGVyZWRfYXMiXSBpcyBOb25lKSkpCgogICAgIyDikakg5Y+q6K+76Ieq6K+B77yI55yf55uu5qCH6Lev5b6E55qE5oyH57q55Ye9" +
	"5pWw77yJ77ya5aS55YW35LuT6LeR5Lik5qyhIOKHkiDlt67lvIIgMCDku7YKICAgIHRoZW4gPSByZWFkb25seV9maW5nZXJwcmlu" +
	"dChkMSwgZHApCiAgICBub3cgPSByZWFkb25seV9maW5nZXJwcmludChkMSwgZHApCiAgICBjYXNlKCLikakg5Y+q6K+76Ieq6K+B" +
	"77ya5ZCM5LiA5aS55YW36L+e5Y+W5Lik5qyhIOKHkiDlt67lvIIgMCDku7bvvIjmo4DmtYvlmajkuI3kubHmiqXvvIkiLCBUcnVl" +
	"LCB0aGVuID09IG5vdykKICAgIHdpdGggb3Blbihvcy5wYXRoLmpvaW4oZDEsICLmnI3liqHlo7DmmI4udHN2IiksICJhIiwgZW5j" +
	"b2Rpbmc9InV0Zi04IikgYXMgZjoKICAgICAgICBmLndyaXRlKCJnaG9zdFx05YGH5bm954G1XHQtXHRiaW4vY29jb29uLWZha2Vc" +
	"dGJpbi9jb2Nvb24tZmFrZVx05aS55YW3XHTlvoVcbiIpCiAgICBhZnRlciA9IHJlYWRvbmx5X2ZpbmdlcnByaW50KGQxLCBkcCkK" +
	"ICAgIGNhc2UoIuKRqWIg6LSf5o6n77yI55m955uS77yJ77ya55yf5rqQ5Lu26KKr5pS55LiA5Liq5a2X6IqCIOKHkiDmjIfnurnn" +
	"nJ/lj5jvvIjkuI3pnZnpu5jvvIkiLCBUcnVlLCB0aGVuICE9IGFmdGVyKQoKICAgIHByaW50KCLoh6rmo4Dnu5Porro6ICIgKyAo" +
	"IuWFqOi/hyIgaWYgb2sgZWxzZSAiKirmnInmlq3oqIDlpLHotKUqKiIpKQogICAgcmV0dXJuIDAgaWYgb2sgZWxzZSAxCgoKaWYg" +
	"X19uYW1lX18gPT0gIl9fbWFpbl9fIjoKICAgIHN5cy5leGl0KG1haW4oKSkK"

func gddDecodeSelfPy() string {
	b, err := base64.StdEncoding.DecodeString(gddSelfPyB64)
	if err != nil {
		return ""
	}
	return string(b)
}

// gddSelfPySource —— 本件 py 原件的内嵌快照（进程内一次性解码 · 只读 ⇒ 无跨调用残留）。
var gddSelfPySourceValue = gddDecodeSelfPy()

func gddSelfPySource() string { return gddSelfPySourceValue }
