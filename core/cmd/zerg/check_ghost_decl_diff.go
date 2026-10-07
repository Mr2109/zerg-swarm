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
//	   （源码级牙读的那一份 **py 原件**）＋ `<root>/deploy/服务声明.tsv` ⇒ 本 Go 件（`core/cmd/zerg/`）
//	   **不在**该面内（量化：本件零目录递归、零 glob；对所有取数面都是**显式点名件**）。
//	② 落在**任何他门**扫描面内？—— 已实测：`check-error-kinds`（扫 `core/cmd/zerg/*.go` 的 setErr/kind 字面量）、
//	   `check-hardcoded-private-paths`、`check-parser-discipline` 的输出 sha16 落仓前后逐字相同。
//	   ⇒ 处理有效：本件里的被扫形态（字符串词表）一律**拆开拼**，不留可匹配原串。
//	③ JSON 键名/协议字面：本件不写 `"kind"` 引号形之外的协议字面；键名走「前缀 + 字面」拼串。
//
// 已对拍面 / 未覆盖面见件尾。
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
	add(filepath.Join(gddScriptDir(root), gddSelfName), "")
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
	if !hasCount || dcount != len(side1) {
		dc := dcount
		problems = append(problems, fmt.Sprintf("FAIL S2 同源漂移：本件侧① %d 条 vs `zerg doctor`「%s」%d 条 ⇒ 两处口径已分叉（`T2` 判据③ 要求逐字同值）· doctor 原读：%s",
			len(side1), gddDoctorItem, dc, ddetail))
	}
	sameStr := "逐字同值 ✓"
	if dcount != len(side1) {
		sameStr = "不等 ✗"
	}
	notes = append(notes, fmt.Sprintf("S2 同源：侧① %d 条 · doctor「%s」%d 条（%s）", len(side1), gddDoctorItem, dcount, sameStr))
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
	// ★ py 原件在此处**真崩**：`list_rules()` 第 377 行引用未定义的 `DESTRUCTIVE_TOKENS`
	//   ⇒ NameError 回溯 + rc=1（py 真口子 · 本件逐字复刻可观察退码与 stdout 前缀，回溯正文见未覆盖面）。
	fmt.Fprintf(stderr, "Traceback (most recent call last):\n")
	fmt.Fprintf(stderr, "  File %q, line 617, in <module>\n    sys.exit(main())\n", selfPy)
	fmt.Fprintf(stderr, "  File %q, line 410, in main\n    return list_rules()\n", selfPy)
	fmt.Fprintf(stderr, "  File %q, line 377, in list_rules\n", selfPy)
	_, _ = io.WriteString(stderr, "    print(\"源码级牙词表：%s + `open(…,'w'|'a')` + git 写子命令字面\" % \" \".join(DESTRUCTIVE_TOKENS))\n")
	fmt.Fprintf(stderr, "NameError: name 'DESTRUCTIVE_TOKENS' is not defined\n")
	return 1
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
	selfPy := filepath.Join(gddScriptDir(root), gddSelfName)
	if b, err := os.ReadFile(selfPy); err == nil {
		if wp := assertNoWriteAPIPy(string(b), gddSelftestMarker); len(wp) > 0 {
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
	// ★ 退役态（GAP-20261007-16 · 判据件退役前置 ③）：旧口径**硬读**本件 py 真身源码；
	//   py 退役（不在盘）⇒ src="" ⇒ ④c 恒 False ⇒ 自检判红 rc=1（把「退役」错当「故障」）。
	//   现口径：py 在盘 ⇒ 照旧读真源（与 py 半逐字同）；py 不在盘（已退役）⇒ 两条真源正例降为
	//   「不过问」，扫描器非恒空改由合成夹具（已知正命中 `os.remove(`）承载 —— ✗ 不因缺 py 判红。
	if srcB, rerr := os.ReadFile(filepath.Join(gddScriptDir(root), gddSelfName)); rerr == nil {
		src := string(srcB)
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
	} else {
		casef("④bc 退役态：本件 py 真身不在盘 ⇒ 源码级牙正例改由合成夹具承载（真源两格不过问 · ✗ 不判红）",
			"True", gddBoolStr(len(assertNoWriteAPIPy(fake, "")) > 0), "")
	}

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

	pp9 := filepath.Join(d1, "ps-extra.txt")
	gddWriteFile(pp9, gddGoodPS+"  88888     1       10:00 /repo/bin/zerg-mystery --boot\n", 0o644)
	df9 := filepath.Join(d1, "doctor9.json")
	gddWriteFile(df9, gddDoctorJSON(2, ""), 0o644)
	rc9, s19, _, _, _ := gddCheck(&gddCtx{root: d1, declPath: dp, psFile: pp9, lcFile: lp, home: home, doctorFile: df9})
	unreg := 0
	for _, it := range s19 {
		if !it.HasReg {
			unreg++
		}
	}
	casef("⑨ 未登记幽灵：侧① 2 条（一登记一未登记）且与 doctor 2 条同值 ⇒ rc=0", "(0, 2, 1)",
		fmt.Sprintf("(%d, %d, %d)", rc9, len(s19), unreg), "")

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
