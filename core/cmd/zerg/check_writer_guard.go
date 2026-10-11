// check_writer_guard.go —— 门禁件 `scripts/gates/check-writer-guard.py` 的 **Go 真搬**（M2 · 守卫族）。
//
// 真源：`scripts/gates/check-writer-guard.py`（521 行 · 纯只读 · rc 0/1/2 · 人面 + `--json` 机器面）。
// 入口：`checkWriterGuardGateMain(stdout, stderr io.Writer, argv []string) int`（argv = py `sys.argv[1:]`）。
//
// ★ 本件只落码，**不接线**：不注册命令、不改 `main.go` / `gate.go` / `family_*.go` / 任何 `add_step`；
// 不删 py 原件（退场对拍通过后才逐件点名）。
//
// 未覆盖面（✗ 不声称逐字对齐 · 详单见伴随测试件与交付报）：
//
//	① argparse 前缀缩写 / 「旗标缺值」usage 正文未实现（`-h` 未定义 ⇒ 未知旗标，已复刻）。
//	② 默认根：py 由 `__file__` 上溯三级（⇒ 仓根）；Go 无 `__file__` ⇒ 起于 cwd / `os.Executable()`
//	   上溯 ≤8 层找含 `scripts/gates/` 的祖先（⇒ 仓根），兜底 abspath(cwd)（见下 cwgDefaultRoot）。
//	③ 异常正文：`OSError`（仅复刻 `FileNotFoundError` 的 `[Errno 2] ...` 形）/ `JSONDecodeError` /
//	   台账非 dict 的 `AttributeError` / 台账 `regex` 非 str 的 `TypeError` 正文不属承诺。
//	④ 遍历序：用 `f.ReadDir(-1)`（非排序）复刻 `os.scandir` 原序；**输出行序处处由 py 自己的
//	   `sorted(...)` 决定**（`check`/`check_ledger` G2/`stale`/`missing`/`mismatch`/`buckets`/`set`）。
//	⑤ 正则：RE2 无后顾断言 ⇒ `REDIRECT_RE` 手写同构；`\s`/`\S` 展开为 Python `str` 空白类。
//	   `\b` 仍按 RE2 ASCII 词边界（py 是 Unicode 词边界）⇒ 该角不属承诺。
//	⑥ `str.splitlines()` / `str.strip()` / `str.isspace()` 的 Python 全行界与 `\x1c-\x1f` 空白已自写复刻。
//	⑦ `os.path.isfile` 跟随符号链接 ⟷ 本件用 `os.Stat`（跟随）而非 `os.Lstat`（标准②）。
//	⑧ `os.access(p, X_OK)` ⟷ `mode&0111 != 0`（同形；root/ACL 角落不属承诺）。
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
	"strconv"
	"strings"
)

const (
	cwgGuardToken   = "atomic-write.sh"
	cwgLedgerSchema = "gate-check-write-tools/1"
	cwgLedgerRegex  = `^#\s*zerg-class:\s*(read|write)\s+(D0|D2|D3)\s*$`
	cwgGeneratedBy  = "zerg gate check check-writer-guard --emit"
)

// cwgFields —— py `FIELDS`（`--json` 字段闭集 · 唯一真源）。
var cwgFields = []string{
	"root", "scanned", "count", "hits", "report_only", "compliant", "rc", "verdict",
	"write_ledger_missing", "write_ledger_stale",
}

// cwgLedgerRel = py `os.path.join("scripts","gates","gate-check-write-tools.json")`。
var cwgLedgerRel = filepath.Join("scripts", "gates", "gate-check-write-tools.json")

// cwgRetireTruthRel —— 退场真源（**同一件** · `scriptRetireTruthRel` · 本件只读 · ✗ 不写、✗ 不新造第二真源）。
//
// ★ 已批口径（2026-10-09 · Mr2109「按推荐」）：空账容忍分支的旁证只能取自它。
var cwgRetireTruthRel = filepath.Join("core", "internal", "contract", "adopt-retire.json")

// cwgRetireLedgerKey —— 退场台账键（逐字照真源 · 与 `scriptRetireLedgerKey` 同串）。
const cwgRetireLedgerKey = "retire_ledger"

var cwgScriptSuffixes = []string{".sh", ".bash", ".zsh", ".py"}
var cwgRosterSuffixes = []string{".py", ".sh"}

var cwgSkipDirs = map[string]bool{
	".git": true, ".obsidian": true, "node_modules": true, "vendor": true, "target": true,
	"__pycache__": true, "dist": true, "bin": true, "data": true, "zerg-wt": true, ".zerg": true,
	"venv": true, ".venv": true, ".cargo": true, "htmlcov": true, ".history": true,
	"_history": true, "scratch": true,
}

// Python `str` 模式 `\s` 逐字展开（RE2 不吃 `\uXXXX` ⇒ 用 `\x{...}`）。
const cwgWSS = `\t\n\v\f\r\x{1c}-\x{1f} \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

var (
	cwgWS   = "[" + cwgWSS + "]"
	cwgNonW = "[^" + cwgWSS + "]"
)

func isPyWS(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0,
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func pyStrip(s string) string { return strings.TrimFunc(s, isPyWS) }

// pySplitLines —— Python `str.splitlines()`（keepends=False）逐字复刻。
func pySplitLines(s string) []string {
	var out []string
	var cur []rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		if c == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
			out = append(out, string(cur))
			cur = cur[:0]
			i++
			continue
		}
		if c == '\n' || c == '\r' || c == '\v' || c == '\f' ||
			c == 0x1c || c == 0x1d || c == 0x1e || c == 0x85 || c == 0x2028 || c == 0x2029 {
			out = append(out, string(cur))
			cur = cur[:0]
			continue
		}
		cur = append(cur, c)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// cwgFileLines —— py 文本模式读文件的「universal newlines + 逐行迭代」语义。
func cwgFileLines(text string) []string {
	t := strings.ReplaceAll(text, "\r\n", "\n")
	t = strings.ReplaceAll(t, "\r", "\n")
	lines := strings.Split(t, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// ── 判据正则（与 py 逐条同源；`\s`/`\S` 展开；仅 `REDIRECT_RE` 因后顾断言手写）──────────────
var (
	cwgClassRE = regexp.MustCompile("^#" + cwgWS + "*zerg-class:" + cwgWS + "*(read|write)" +
		cwgWS + "+(D0|D2|D3)" + cwgWS + "*$")

	cwgRepoSrcRE = regexp.MustCompile(`^(?:scripts|publish|core|agent|docs|\.githooks|tools)/[A-Za-z0-9._/@+\-]+$`)

	cwgCPMVRE = regexp.MustCompile("(?:^|[;&|]|" + cwgWS + ")(?:cp|mv|install)" + cwgWS +
		"+(?:-[A-Za-z-]+" + cwgWS + "+)*" + cwgNonW + "+" + cwgWS + "+([^" + cwgWSS + ";|)&]+)")

	cwgTeeRE = regexp.MustCompile(`\btee` + cwgWS + `+(?:-a` + cwgWS + `+)?([^` + cwgWSS + `;|)&]+)`)

	cwgDdRE = regexp.MustCompile(`\bof=([^` + cwgWSS + `;|)&]+)`)

	cwgPyOpenRE = regexp.MustCompile(`\bopen` + cwgWS + `*\(` + cwgWS + `*([^,)]+)` + cwgWS +
		`*,` + cwgWS + `*['"](?:w|wb|wt|w\+)['"]`)

	cwgPywTextRE = regexp.MustCompile(`\.(?:write_text|write_bytes)` + cwgWS + `*\(`)

	cwgPywPathRE = regexp.MustCompile(`\b(?:Path|PurePath)` + cwgWS + `*\(` + cwgWS + `*([^,)]+)`)

	cwgPywCopyRE = regexp.MustCompile(`\bshutil\.(?:copy|copy2|copyfile)` + cwgWS + `*\(` +
		cwgWS + `*[^,]+,[ \t]*` + cwgWS + `*([^,)]+)`)
)

// cwgRedirectTok —— py `REDIRECT_RE = (?<![0-9>&])>(?![>&=])\s*([^\s;|)&]+)` 的手写同构扫描器。
func cwgRedirectTok(line string) []string {
	rs := []rune(line)
	var out []string
	i := 0
	for i < len(rs) {
		if rs[i] != '>' {
			i++
			continue
		}
		if i > 0 {
			p := rs[i-1]
			if (p >= '0' && p <= '9') || p == '&' || p == '>' {
				i++
				continue
			}
		}
		if i+1 < len(rs) {
			n := rs[i+1]
			if n == '>' || n == '&' || n == '=' {
				i++
				continue
			}
		}
		j := i + 1
		for j < len(rs) && isPyWS(rs[j]) {
			j++
		}
		k := j
		for k < len(rs) {
			c := rs[k]
			if isPyWS(c) || c == ';' || c == '|' || c == ')' || c == '&' {
				break
			}
			k++
		}
		if k > j {
			out = append(out, string(rs[j:k]))
			i = k
			continue
		}
		i++
	}
	return out
}

// ── Python repr / str 口径（str · 二元组 · None/数值）────────────────────────────
func cwgPyRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case byte(r) == quote:
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func cwgRepr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return cwgPyRepr(t)
	case [2]string:
		return "(" + cwgPyRepr(t[0]) + ", " + cwgPyRepr(t[1]) + ")"
	case json.Number:
		return t.String()
	case int:
		return strconv.Itoa(t)
	case bool:
		if t {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprintf("%v", t)
	}
}

func cwgReprLoose(v interface{}) string {
	if s, ok := v.(string); ok {
		return cwgPyRepr(s)
	}
	return cwgRepr(v)
}

// ── 判据主体 ─────────────────────────────────────────────────────────────
type cwgForm struct {
	name string
	why  string
	rx   *regexp.Regexp
	fn   func(string) []string
}

func (f cwgForm) find(line string) []string {
	if f.fn != nil {
		return f.fn(line)
	}
	var out []string
	for _, m := range f.rx.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1])
	}
	return out
}

// py FORMS 顺序（REDIRECT, CP/MV, TEE, DD, PY-OPEN-W）—— 决定同一行内命中顺序。
var cwgForms = []cwgForm{
	{"REDIRECT", "重定向 `>` 整件覆盖落点", nil, cwgRedirectTok},
	{"CP/MV", "`cp`/`mv`/`install` 整件覆盖落点", cwgCPMVRE, nil},
	{"TEE", "`tee` 落点（同名件被覆盖）", cwgTeeRE, nil},
	{"DD", "`dd of=` 整件覆盖落点", cwgDdRE, nil},
	{"PY-OPEN-W", "Python `open(..., 'w')` 整件覆盖落点", cwgPyOpenRE, nil},
}

type cwgHit struct {
	file string
	ln   int
	txt  string
	form string
	why  string
	dst  string
}

func cwgEndsWithAny(s string, suf []string) bool {
	for _, x := range suf {
		if strings.HasSuffix(s, x) {
			return true
		}
	}
	return false
}

// cwgIsRepoLiteral —— py `is_repo_literal`。
func cwgIsRepoLiteral(tok, root string) bool {
	t := strings.Trim(pyStrip(tok), "'\"")
	if t == "" || strings.Contains(t, "$") || strings.HasPrefix(t, "/") || strings.HasPrefix(t, "~") {
		return false
	}
	if !cwgRepoSrcRE.MatchString(t) {
		return false
	}
	if root != "" {
		st, err := os.Stat(filepath.Join(root, t))
		if err != nil || !st.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// cwgScanFile —— py `scan_file`：ok=false 表示 py 返回 None（读不到）。
func cwgScanFile(path, rel, root string) ([]cwgHit, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	text := strings.ToValidUTF8(string(b), "\uFFFD")
	if strings.Contains(text, cwgGuardToken) {
		return []cwgHit{}, true
	}
	var hits []cwgHit
	inDoc := false
	for i, line := range pySplitLines(text) {
		n := i + 1
		toggle := strings.Count(line, `"""`) + strings.Count(line, "'''")
		if inDoc {
			if toggle%2 == 1 {
				inDoc = false
			}
			continue
		}
		if toggle%2 == 1 {
			inDoc = true
		}
		s := pyStrip(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		txt := strings.TrimRight(line, "\n")
		for _, fm := range cwgForms {
			for _, tok := range fm.find(line) {
				if cwgIsRepoLiteral(tok, root) {
					hits = append(hits, cwgHit{rel, n, txt, fm.name, fm.why + "（" + fm.name + "）",
						strings.Trim(pyStrip(tok), "'\"")})
				}
			}
		}
		if cwgPywTextRE.MatchString(line) {
			for _, m := range cwgPywPathRE.FindAllStringSubmatch(line, -1) {
				tok := m[1]
				if cwgIsRepoLiteral(tok, root) {
					hits = append(hits, cwgHit{rel, n, txt, "PY-PATHLIB",
						"Python `write_text`/`write_bytes` 的 `Path(…)` 落点整件覆盖（PY-PATHLIB）",
						strings.Trim(pyStrip(tok), "'\"")})
				}
			}
		}
		for _, m := range cwgPywCopyRE.FindAllStringSubmatch(line, -1) {
			tok := m[1]
			if cwgIsRepoLiteral(tok, root) {
				hits = append(hits, cwgHit{rel, n, txt, "PY-SHUTIL-COPY",
					"Python `shutil.copy*` 的目标实参整件覆盖（PY-SHUTIL-COPY）",
					strings.Trim(pyStrip(tok), "'\"")})
			}
		}
	}
	return hits, true
}

// cwgWalk —— `os.walk`（topdown · followlinks=False）语义的本仓实现。
// 关闭序：ReadDir(-1) 取**原序**（非排序）；子目录按原序递归。
func cwgWalk(walkRoot, relBase string, fn func(p, rel string)) {
	var walk func(dir string)
	walk = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			return
		}
		ents, err := f.ReadDir(-1)
		_ = f.Close()
		if err != nil {
			return
		}
		var subs []string
		for _, e := range ents {
			name := e.Name()
			p := filepath.Join(dir, name)
			isdir := e.IsDir()
			if !isdir {
				// 符号链接指向目录：py `entry.is_dir()` 跟随 ⇒ 落 dirnames，但 followlinks=False 不递归。
				if st, err := os.Stat(p); err == nil && st.IsDir() {
					continue
				}
			}
			if isdir {
				if !cwgSkipDirs[name] {
					subs = append(subs, p)
				}
				continue
			}
			if strings.HasPrefix(name, ".") && !strings.HasPrefix(name, ".githooks") {
				continue
			}
			rel, _ := filepath.Rel(relBase, p)
			fn(p, rel)
		}
		for _, d := range subs {
			walk(d)
		}
	}
	walk(walkRoot)
}

func cwgCollectScripts(root string) [][2]string {
	var out [][2]string
	cwgWalk(root, root, func(p, rel string) {
		name := filepath.Base(p)
		if cwgEndsWithAny(name, cwgScriptSuffixes) {
			out = append(out, [2]string{p, rel})
			return
		}
		if !strings.Contains(name, ".") {
			if st, err := os.Stat(p); err == nil && st.Mode()&0111 != 0 {
				out = append(out, [2]string{p, rel})
				return
			}
			if b, err := os.ReadFile(p); err == nil && len(b) >= 2 && b[0] == '#' && b[1] == '!' {
				out = append(out, [2]string{p, rel})
			}
		}
	})
	return out
}

// cwgParseDecl —— py `parse_decl`（件头前 80 行找 `#zerg-class:`）。
func cwgParseDecl(path string) (string, string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", false
	}
	text := strings.ToValidUTF8(string(b), "\uFFFD")
	for i, line := range cwgFileLines(text) {
		if i >= 80 {
			break
		}
		m := cwgClassRE.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
		if m != nil {
			return m[1], m[2], true
		}
	}
	return "", "", false
}

func cwgCollectDeclared(root string) map[string][2]string {
	out := map[string][2]string{}
	base := filepath.Join(root, "scripts")
	cwgWalk(base, root, func(p, rel string) {
		name := filepath.Base(p)
		if !cwgEndsWithAny(name, cwgRosterSuffixes) {
			return
		}
		if c, l, ok := cwgParseDecl(p); ok {
			out[rel] = [2]string{c, l}
		}
	})
	return out
}

func cwgLoadLedger(root string) (map[string]interface{}, error) {
	path := filepath.Join(root, cwgLedgerRel)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("[Errno 2] No such file or directory: '%s'", path)
		}
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("ledger is not a JSON object")
	}
	return m, nil
}

// ── ★ 退场台账旁证（已批口径 · 2026-10-09 · Mr2109「按推荐」）─────────────────────────
//
// 口径逐字：**给「已登记退场条目」留一条容忍口径** —— 台账 items **空集**时，若退场台账能证
// 「**最后一只写档类工具已被登记退场**」（该退场条目在册 ＋ 对应的 Go 半/新路径存在或在盘）⇒ **判绿**
// 并留痕一句；★ **✗ 非终态空转下不得判绿**（账为空但退场台账里查不到对应登记 ⇒ **仍 rc=2 不给结论**）。
//
// ★ 判「写档类工具」的真源 = 中央分类表 `gateCheckClassEntry`（`gate_inproc_table.go` · GAP-20261008-11）：
// **件头 `#zerg-class:` 随件退场即消失**（退役后读空）⇒ 不能拿它当旁证真源；中央表按**判据名**登记、
// 与件在不在盘无关。⇒ 退场条目 `object` 的基名（去目录去后缀）⇒ 判据名 ⇒ 查中央表 `class=="write"`。

// cwgRetireEntry —— 退场台账一条（object/state/evidence · 逐字照真源条目形状）。
type cwgRetireEntry struct {
	Object   string `json:"object"`
	State    string `json:"state"`
	Evidence string `json:"evidence"`
}

// cwgBaseNoExt —— `a/b/c.sh` ⇒ `c`（去目录去后缀 · 用于与中央分类表的判据名对齐）。
func cwgBaseNoExt(p string) string {
	b := filepath.Base(p)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

// cwgGoPathRE —— evidence 里点名的 Go 半/新路径（`…/x.go` · 覆盖 `core/**` 与 `scripts/**` 两类落点）。
var cwgGoPathRE = regexp.MustCompile(`[A-Za-z0-9_][A-Za-z0-9_./-]*\.go`)

// cwgRetireGoHalfOnDisk —— 退场条目「对应的 Go 半/新路径是否在盘」（普通件）。
//
//	① 判据名 ⇒ `core/cmd/zerg/<名 · - → _>.go`（同族 Go 真搬落点 · 先试）；
//	② 否则在 `evidence` 里找仓内相对 `….go` 路径（覆盖 `scripts/**` 一类新落点）。
//
// 返命中的仓内相对路径（slash 形）；都不在盘 ⇒ ""（不旁证 ⇒ 调用方照旧 rc=2）。
func cwgRetireGoHalfOnDisk(root, key, evidence string) string {
	if key != "" {
		rel := filepath.ToSlash(filepath.Join("core", "cmd", "zerg", strings.ReplaceAll(key, "-", "_")+".go"))
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil && st.Mode().IsRegular() {
			return rel
		}
	}
	for _, tok := range cwgGoPathRE.FindAllString(evidence, -1) {
		if !strings.Contains(tok, "/") {
			continue
		}
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(tok))); err == nil && st.Mode().IsRegular() {
			return tok
		}
	}
	return ""
}

// cwgRetireCorroboration —— 台账空集时的**唯一**容忍口：返旁证描述 ＋ Go 半/新路径（都在 ⇒ 可容绿）。
//
// 逐条扫 `retire_ledger`：`object` 基名 ∈ 中央分类表且 `class=="write"`（= 写档类工具的**登记退场**条目），
// 且其「对应 Go 半/新路径」在盘 ⇒ 命中第一条即返。无旁证/真源读不到/读不动 ⇒ 返 ("","")（**fail-closed**）。
func cwgRetireCorroboration(root string) (string, string) {
	b, err := os.ReadFile(filepath.Join(root, cwgRetireTruthRel))
	if err != nil {
		return "", ""
	}
	var doc struct {
		RetireLedger []cwgRetireEntry `json:"retire_ledger"`
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	if err := dec.Decode(&doc); err != nil {
		return "", ""
	}
	var lastObj, lastState, lastGo string
	for _, e := range doc.RetireLedger {
		key := cwgBaseNoExt(e.Object)
		spec, ok := gateCheckClassEntry[key]
		if !ok || spec.Class != "write" {
			continue
		}
		if gp := cwgRetireGoHalfOnDisk(root, key, e.Evidence); gp != "" {
			// 取**最后一条**命中的写档类工具（退场台账按追加序 ⇒ 最后一只已登记退场的写档类工具）。
			lastObj, lastState, lastGo = e.Object, e.State, gp
		}
	}
	if lastGo == "" {
		return "", ""
	}
	return fmt.Sprintf("object=%s（state=%s）", cwgPyRepr(lastObj), cwgPyRepr(lastState)), lastGo
}

// cwgCheckLedger —— py `check_ledger`：返回 (rc, info, notes)。
func cwgCheckLedger(root string) (int, map[string]int, []string) {
	info := map[string]int{"write_ledger_missing": 0, "write_ledger_stale": 0}
	var notes []string
	led, err := cwgLoadLedger(root)
	if err != nil {
		return 2, info, []string{fmt.Sprintf("⛔ G0 不给结论：台账读不到/解析失败（%s）：%s", cwgLedgerRel, err.Error())}
	}
	if led["schema"] != cwgLedgerSchema {
		return 2, info, []string{fmt.Sprintf("⛔ G0 不给结论：台账 schema 不对（%s ≠ %s）",
			cwgReprLoose(led["schema"]), cwgPyRepr(cwgLedgerSchema))}
	}
	rxstr, _ := led["regex"].(string)
	if _, cerr := regexp.Compile(rxstr); cerr != nil {
		return 2, info, []string{fmt.Sprintf("⛔ G0 不给结论：台账 regex 编译不过：%s", cerr.Error())}
	}
	items, ok := led["items"].([]interface{})
	if !ok || len(items) == 0 {
		// ★ 已批口径（2026-10-09 · Mr2109「按推荐」）：给「已登记退场条目」留一条容忍口径。
		//   台账 items **显式空集**时，仅当下列两条**同时**成立才判绿：① 件头**零声明**
		//   （cwgCollectDeclared 空 ⇒ 写档件确已退尽 · 防「非终态空转」）；② 退场台账能证
		//   「最后一只写档类工具已被登记退场」（cwgRetireCorroboration）。否则**仍 rc=2 不给结论**。
		//   ★ ✗ 空即绿（放宽）；✗ 删本条（原 rc=2 分支一字不动地保留在下方）。
		if ok && len(items) == 0 && len(cwgCollectDeclared(root)) == 0 {
			if ev, gp := cwgRetireCorroboration(root); ev != "" {
				return 0, info, []string{fmt.Sprintf(
					"✅ G0 空账容态：台账 items 空集 ＋ 件头零声明 ＋ 退场台账旁证「最后一只写档类工具已登记退场」—— 旁证来自退场台账 %s · Go 半/新路径在盘 %s（已批口径 2026-10-09）",
					ev, gp)}
			}
		}
		return 2, info, []string{"⛔ G0 不给结论：台账 items 空集（空转不当绿）"}
	}
	countStr := "None"
	switch cv := led["count"].(type) {
	case json.Number:
		countStr = cv.String()
	case nil:
		countStr = "None"
	default:
		countStr = fmt.Sprintf("%v", cv)
	}
	if countStr != strconv.Itoa(len(items)) {
		return 2, info, []string{fmt.Sprintf("⛔ G0 不给结论：台账 count(%s) ≠ items 数(%d)", countStr, len(items))}
	}
	for _, it := range items {
		m, _ := it.(map[string]interface{})
		fv, _ := m["file"].(string)
		if !strings.HasPrefix(fv, "scripts"+string(os.PathSeparator)) && !strings.HasPrefix(fv, "scripts/") {
			return 2, info, []string{fmt.Sprintf("⛔ G0 不给结论：台账点名件不在 `scripts/` 域（%s）", cwgPyRepr(fv))}
		}
		if st, err := os.Stat(filepath.Join(root, fv)); err != nil || !st.Mode().IsRegular() {
			return 2, info, []string{fmt.Sprintf("⛔ G0 不给结论：台账点名件不在盘（%s）", cwgPyRepr(fv))}
		}
	}
	declared := cwgCollectDeclared(root)
	lset := map[string][2]string{}
	for _, it := range items {
		m, _ := it.(map[string]interface{})
		fv, _ := m["file"].(string)
		cls, _ := m["class"].(string)
		lvl, _ := m["level"].(string)
		lset[fv] = [2]string{cls, lvl}
	}
	var stale, missing, mismatch []string
	for f := range declared {
		if _, ok := lset[f]; !ok {
			stale = append(stale, f)
		}
	}
	for f := range lset {
		if _, ok := declared[f]; !ok {
			missing = append(missing, f)
		}
	}
	for f, v := range lset {
		if dv, ok := declared[f]; ok && dv != v {
			mismatch = append(mismatch, fmt.Sprintf("%s(台账 %s ⟷ 件头 %s)", f, cwgRepr(v), cwgRepr(dv)))
		}
	}
	sort.Strings(stale)
	sort.Strings(missing)
	sort.Strings(mismatch)
	info["write_ledger_missing"] = len(missing) + len(mismatch)
	info["write_ledger_stale"] = len(stale)
	rc := 0
	if len(stale) > 0 {
		rc = 1
		for _, f := range stale {
			notes = append(notes, fmt.Sprintf("G1 新落写工具漏登记（件头声明了 · 台账 items 无）：%s ⇒ 同批登记 `%s`", f, cwgLedgerRel))
		}
	}
	if len(missing) > 0 {
		rc = 1
		for _, f := range missing {
			notes = append(notes, fmt.Sprintf("G1 台账陈旧（台账点名 · 件头无声明）：%s", f))
		}
	}
	if len(mismatch) > 0 {
		rc = 1
		for _, d := range mismatch {
			notes = append(notes, fmt.Sprintf("G1 class/level 与件头不符：%s", d))
		}
	}
	base := filepath.Join(root, "scripts")
	if st, err := os.Stat(base); err == nil && st.IsDir() {
		sc := cwgCollectScripts(base)
		sort.Slice(sc, func(i, j int) bool { return sc[i][1] < sc[j][1] })
		for _, pr := range sc {
			hs, ok := cwgScanFile(pr[0], pr[1], root)
			if !ok || len(hs) == 0 {
				continue
			}
			c, _, has := cwgParseDecl(pr[0])
			if !has || c != "write" {
				rc = 1
				info["write_ledger_missing"]++
				notes = append(notes, fmt.Sprintf("G2 漏标写形态：scripts/%s 命中整件覆盖写（%d 处）却未显式声明 `#zerg-class: write D2|D3`",
					pr[1], len(hs)))
			}
		}
	}
	return rc, info, notes
}

// cwgCheckInfo —— py `check` 的 info（hits 为**有序切片** · 保 py 追加序）。
type cwgCheckInfo struct {
	root       string
	scanned    int
	count      int
	hits       []cwgHit
	reportOnly bool
	compliant  int
}

func cwgCheck(root string, reportOnly bool) (int, *cwgCheckInfo, []string) {
	info := &cwgCheckInfo{root: root, reportOnly: reportOnly}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return 2, info, []string{fmt.Sprintf("⛔ 不给结论：扫描根读不到（%s）—— 空转不当绿", root)}
	}
	scripts := cwgCollectScripts(root)
	if len(scripts) == 0 {
		return 2, info, []string{fmt.Sprintf("⛔ 不给结论：扫描面 0 个脚本件（%s）—— 空转不当绿", root)}
	}
	info.scanned = len(scripts)
	sort.Slice(scripts, func(i, j int) bool { return scripts[i][1] < scripts[j][1] })
	for _, pr := range scripts {
		hs, ok := cwgScanFile(pr[0], pr[1], root)
		if !ok {
			continue
		}
		if len(hs) == 0 {
			info.compliant++
			continue
		}
		info.hits = append(info.hits, hs...)
	}
	info.count = len(info.hits)
	if reportOnly {
		return 0, info, nil
	}
	if info.count > 0 {
		var notes []string
		for _, h := range info.hits {
			notes = append(notes, fmt.Sprintf("%s:%d  [%s] %s ⇒ 整件覆盖写仓内件 %s 而未走 %s（判为绕过）",
				h.file, h.ln, h.form, h.txt, h.dst, cwgGuardToken))
		}
		return 1, info, notes
	}
	return 0, info, nil
}

func cwgPrintHuman(w, ew io.Writer, info *cwgCheckInfo, notes []string) {
	for _, n := range notes {
		fmt.Fprint(ew, n+"\n")
	}
	if info.count == 0 {
		fmt.Fprintf(w, "写者守卫旁路：零命中 ✓（扫 %d 个脚本件 · 零命中件 %d · 根 %s）\n",
			info.scanned, info.compliant, info.root)
		return
	}
	fmt.Fprintf(w, "写者守卫旁路：命中 %d 条（扫 %d 个脚本件）· 聚合计数如下，逐条面用 --json hits\n",
		info.count, info.scanned)
	buckets := map[string]int{}
	var keys []string
	for _, h := range info.hits {
		if _, ok := buckets[h.form]; !ok {
			keys = append(keys, h.form)
		}
		buckets[h.form]++
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "   %-10s %d 条\n", k, buckets[k])
	}
	lim := 3
	if len(info.hits) < lim {
		lim = len(info.hits)
	}
	for _, h := range info.hits[:lim] {
		fmt.Fprintf(w, "   %s:%d  [%s]  %s\n", h.file, h.ln, h.form, h.txt)
	}
}

// ── JSON 输出（复刻 `json.dumps(..., ensure_ascii=False, indent=2)`）──────────────
type cwgKV struct {
	k string
	v interface{}
}
type cwgOrdered []cwgKV

func cwgJSONStr(s string) string {
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
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
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

func cwgDumps(v interface{}, ind int, b *strings.Builder) {
	pad := func(n int) { b.WriteString(strings.Repeat(" ", n)) }
	switch t := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if t {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.Itoa(t))
	case int64:
		b.WriteString(strconv.FormatInt(t, 10))
	case json.Number:
		b.WriteString(t.String())
	case string:
		b.WriteString(cwgJSONStr(t))
	case cwgOrdered:
		if len(t) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for i, kv := range t {
			pad(ind + 2)
			b.WriteString(cwgJSONStr(kv.k))
			b.WriteString(": ")
			cwgDumps(kv.v, ind+2, b)
			if i < len(t)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		pad(ind)
		b.WriteString("}")
	case []cwgOrdered:
		if len(t) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i, it := range t {
			pad(ind + 2)
			cwgDumps(it, ind+2, b)
			if i < len(t)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		pad(ind)
		b.WriteString("]")
	case []interface{}:
		if len(t) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i, it := range t {
			pad(ind + 2)
			cwgDumps(it, ind+2, b)
			if i < len(t)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		pad(ind)
		b.WriteString("]")
	default:
		b.WriteString(cwgJSONStr(fmt.Sprintf("%v", t)))
	}
}

func cwgVerdict(rc int) string {
	switch rc {
	case 0:
		return "green"
	case 1:
		return "red"
	default:
		return "blocked"
	}
}

func cwgEmitJSON(w io.Writer, info *cwgCheckInfo, lm map[string]int, rc int, fields []string) {
	hitObjs := make([]cwgOrdered, 0, len(info.hits))
	for _, h := range info.hits {
		hitObjs = append(hitObjs, cwgOrdered{
			{"file", h.file}, {"line", h.ln}, {"text", h.txt},
			{"form", h.form}, {"why", h.why}, {"target", h.dst},
		})
	}
	data := map[string]interface{}{
		"root": info.root, "scanned": info.scanned, "count": info.count,
		"hits": hitObjs, "report_only": info.reportOnly, "compliant": info.compliant,
		"rc": rc, "verdict": cwgVerdict(rc),
		"write_ledger_missing": lm["write_ledger_missing"],
		"write_ledger_stale":   lm["write_ledger_stale"],
	}
	var obj cwgOrdered
	for _, k := range fields {
		obj = append(obj, cwgKV{k, data[k]})
	}
	var b strings.Builder
	cwgDumps(obj, 0, &b)
	fmt.Fprint(w, b.String()+"\n")
}

func cwgUsageText() string {
	return "用法\n" +
		"  python3 scripts/gates/check-writer-guard.py [--root <目录>] [--report-only]\n" +
		"  python3 scripts/gates/check-writer-guard.py --json <字段,…>\n" +
		"  --emit [--out <件>] [--dry-run]\n" +
		"  python3 scripts/gates/check-writer-guard.py --self-test\n" +
		"  python3 scripts/gates/check-writer-guard.py --help\n" +
		"说明\n" +
		"  只读旁路探测器：扫根下脚本，找「整件覆盖写仓内源码件、却没走 scripts/dev/atomic-write.sh」的行。\n" +
		"  --root        扫描根（默认 = 本件上溯三级的仓根）\n" +
		"  --report-only 干跑档：命中只登记、退码恒 0\n" +
		"  --json        人面 JSON（字段闭集：" + strings.Join(cwgFields, ",") + "）\n" +
		"  --self-test   合成夹具：3 负控（必命中）+ 2 正控（不得命中）\n" +
		"  --emit        现读生成写工具台账（只收 class==write）· `--out -` ⇒ stdout\n" +
		"  --out         台账落点（缺省 = <根>/scripts/gates/gate-check-write-tools.json）\n" +
		"  --dry-run     只与 --emit 同用：只印不写（计划件走 stdout · 零副作用）\n" +
		"退码\n" +
		"  0 全绿 / 干跑档 · 1 红（命中绕过）· 2 不给结论（用法错 / 未知旗标 / 根读不到 / 扫到 0 个脚本 / 自检不过 / 台账写不出）\n"
}

func cwgWriteFile(path, content string) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(content), 0o644)
}

// cwgSelftest —— py `selftest`（3 负控必命中 · 2 正控不得命中 · 空转必 2 · 干跑档同计数 · 台账 4 格）。
func cwgSelftest(stdout, stderr io.Writer) int {
	tmp, err := os.MkdirTemp(os.Getenv("TMPDIR"), "writer-guard-selftest.")
	if err != nil {
		fmt.Fprint(stderr, "⛔ 自检不过 1 格 ⇒ 退 2（不给结论）\n")
		return 2
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	type chkRec struct {
		name string
		ok   bool
		det  string
	}
	var checks []chkRec
	chk := func(name string, cond bool, detail string) {
		checks = append(checks, chkRec{name, cond, detail})
	}

	fake := filepath.Join(tmp, "repo")
	_ = os.MkdirAll(filepath.Join(fake, "scripts", "gates"), 0o755)
	_ = os.MkdirAll(filepath.Join(fake, "scripts", "dev"), 0o755)
	cwgWriteFile(filepath.Join(fake, "scripts", "gates", "demo.sh"), "OLD\n")

	cwgWriteFile(filepath.Join(fake, "scripts", "bad-cp.sh"),
		"#!/usr/bin/env bash\n# 负控①\ncp /tmp/new "+"scripts/gates/demo.sh\n")
	cwgWriteFile(filepath.Join(fake, "scripts", "bad-redirect.sh"),
		"#!/usr/bin/env bash\n# 负控②\nprintf 'NEW\\n' > "+"scripts/gates/demo.sh\n")
	cwgWriteFile(filepath.Join(fake, "scripts", "bad-py.py"),
		"#!/usr/bin/env python3\n# 负控③\nfrom pathlib import Path\n"+
			"Path("+"'scripts/gates/demo.sh').write_text('NEW')\n")
	cwgWriteFile(filepath.Join(fake, "scripts", "ok-atomic.sh"),
		"#!/usr/bin/env bash\n# 正控①\n"+
			"bash scripts/dev/atomic-write.sh --target "+"scripts/gates/demo.sh --from /tmp/new\n")
	cwgWriteFile(filepath.Join(fake, "scripts", "ok-tmp.sh"),
		"#!/usr/bin/env bash\n# 正控②\n"+
			"tmp=\"$(mktemp -d)\"\nprintf NEW > \"${tmp}/"+"demo.sh\"\ntee \"$OUT\" >/dev/null\n")

	rc, info, _ := cwgCheck(fake, false)
	names := make([]string, 0, len(info.hits))
	cnt := map[string]int{}
	for _, h := range info.hits {
		b := filepath.Base(h.file)
		names = append(names, b)
		cnt[b]++
	}
	sort.Strings(names)
	uniq := []string{}
	for _, n := range names {
		if len(uniq) == 0 || uniq[len(uniq)-1] != n {
			uniq = append(uniq, n)
		}
	}
	chk("① 负控 bad-cp.sh 必须命中", cnt["bad-cp.sh"] >= 1, "命中 "+strconv.Itoa(cnt["bad-cp.sh"]))
	chk("② 负控 bad-redirect.sh 必须命中", cnt["bad-redirect.sh"] >= 1, "命中 "+strconv.Itoa(cnt["bad-redirect.sh"]))
	chk("③ 负控 bad-py.py 必须命中", cnt["bad-py.py"] >= 1, "命中 "+strconv.Itoa(cnt["bad-py.py"]))
	chk("④ 正控 ok-atomic.sh 不得命中", cnt["ok-atomic.sh"] == 0, "命中 "+strconv.Itoa(cnt["ok-atomic.sh"]))
	chk("⑤ 正控 ok-tmp.sh 不得命中", cnt["ok-tmp.sh"] == 0, "命中 "+strconv.Itoa(cnt["ok-tmp.sh"]))
	chk("⑥ 负控件整体判红（rc=1）", rc == 1, "rc="+strconv.Itoa(rc))
	chk("⑦ 区别力：命中件集 == 3 个负控件",
		strings.Join(uniq, ",") == "bad-cp.sh,bad-py.py,bad-redirect.sh",
		"命中件 = "+strings.Join(uniq, ","))

	empty := filepath.Join(tmp, "empty")
	_ = os.MkdirAll(empty, 0o755)
	rc2, _, _ := cwgCheck(empty, false)
	chk("⑧ 空根 ⇒ rc=2 不给结论", rc2 == 2, "rc="+strconv.Itoa(rc2))

	rc3, info3, _ := cwgCheck(fake, true)
	chk("⑨ 干跑档 rc=0 且计数不丢", rc3 == 0 && info3.count == info.count,
		"rc="+strconv.Itoa(rc3)+" count="+strconv.Itoa(info3.count))

	lfake := filepath.Join(tmp, "lrepo")
	_ = os.MkdirAll(filepath.Join(lfake, "scripts", "dev"), 0o755)
	_ = os.MkdirAll(filepath.Join(lfake, "scripts", "gates"), 0o755)
	cwgWriteFile(filepath.Join(lfake, "scripts", "gates", "demo.sh"), "OLD\n")
	cwgWriteFile(filepath.Join(lfake, "scripts", "dev", "w.sh"),
		"#!/usr/bin/env bash\n#zerg-class: write D2\nprintf NEW > "+"scripts/gates/demo.sh\n")
	lf := filepath.Join(lfake, "scripts", "gates", "gate-check-write-tools.json")
	wrLedger := func(items []interface{}) {
		led := map[string]interface{}{"schema": cwgLedgerSchema, "head": "x",
			"count": len(items), "regex": cwgClassRE.String(), "items": items}
		b, _ := json.Marshal(led)
		cwgWriteFile(lf, string(b))
	}
	d2 := func(file string) map[string]interface{} {
		return map[string]interface{}{"file": file, "class": "write", "level": "D2"}
	}

	wrLedger([]interface{}{d2("scripts/dev/w.sh")})
	lrc, _, _ := cwgCheckLedger(lfake)
	chk("⑩ 台账腿正控（声明齐 + 台账齐）⇒ rc=0", lrc == 0, "rc="+strconv.Itoa(lrc))

	cwgWriteFile(filepath.Join(lfake, "scripts", "dev", "x.sh"), "#!/usr/bin/env bash\necho hi\n")
	wrLedger([]interface{}{d2("scripts/dev/w.sh"), d2("scripts/dev/x.sh")})
	l2, _, ln2 := cwgCheckLedger(lfake)
	chk("⑪ 台账多一条 ⇒ rc=1 且点名", l2 == 1 && cwgAnyContains(ln2, "x.sh"), "rc="+strconv.Itoa(l2))

	_ = os.Remove(filepath.Join(lfake, "scripts", "dev", "x.sh"))
	cwgWriteFile(filepath.Join(lfake, "scripts", "dev", "y.sh"),
		"#!/usr/bin/env bash\nprintf NEW > "+"scripts/gates/demo.sh\n")
	wrLedger([]interface{}{d2("scripts/dev/w.sh")})
	l3, _, ln3 := cwgCheckLedger(lfake)
	chk("⑫ 未标写形态件 ⇒ rc=1 且点名", l3 == 1 && cwgAnyContains(ln3, "y.sh"), "rc="+strconv.Itoa(l3))
	_ = os.Remove(filepath.Join(lfake, "scripts", "dev", "y.sh"))

	// ⑬ 台账空集**两态**（★ 2026-10-09 已批口径）：空账 ＋ 无旁证 ⇒ rc=2；空账 ＋ 退场台账旁证
	//   （写档类工具已登记退场 ＋ 其 Go 半在盘）⇒ rc=0 且**留痕**；★ 反证：拿掉旁证 ⇒ **必须回 rc=2**。
	//   （用 fresh efake：件头零声明 —— 终态；lfake 的 w.sh 有 write 声明 ⇒ 属「非终态」不当容绿。）
	efake := filepath.Join(tmp, "erepo")
	_ = os.MkdirAll(filepath.Join(efake, "scripts", "gates"), 0o755)
	cwgWriteFile(filepath.Join(efake, "scripts", "gates", "demo.sh"), "OLD\n") // 非写档件（无件头声明）
	elb, _ := json.Marshal(map[string]interface{}{"schema": cwgLedgerSchema, "head": "x",
		"count": 0, "regex": cwgClassRE.String(), "items": []interface{}{}})
	cwgWriteFile(filepath.Join(efake, "scripts", "gates", "gate-check-write-tools.json"), string(elb))
	e1, _, _ := cwgCheckLedger(efake) // 态①：账空 ＋ 无旁证 ⇒ rc=2
	cwgWriteFile(filepath.Join(efake, cwgRetireTruthRel),
		`{"retire_ledger":[{"object":"scripts/dev/atomic-write.sh","state":"已登记退场（夹具）",`+
			`"evidence":"Go 真身 core/cmd/zerg/atomic_write.go（夹具）"}]}`)
	cwgWriteFile(filepath.Join(efake, "core", "cmd", "zerg", "atomic_write.go"), "package main\n")
	e2, _, en2 := cwgCheckLedger(efake) // 态②：账空 ＋ 有旁证 ⇒ rc=0 且留痕
	_ = os.Remove(filepath.Join(efake, cwgRetireTruthRel))
	e3, _, _ := cwgCheckLedger(efake) // 反证：拿掉旁证 ⇒ 回 rc=2
	chk("⑬ 台账空集两态（无旁证 rc=2 · 有旁证 rc=0 留痕 · 拿掉旁证回 rc=2）",
		e1 == 2 && e2 == 0 && cwgAnyContains(en2, "退场台账") && cwgAnyContains(en2, "atomic-write") && e3 == 2,
		"无旁证="+strconv.Itoa(e1)+" 有旁证="+strconv.Itoa(e2)+" 反证="+strconv.Itoa(e3))

	bad := 0
	for _, c := range checks {
		mark := "✓"
		if !c.ok {
			mark = "✗"
			bad++
		}
		extra := ""
		if c.det != "" {
			extra = "（" + c.det + "）"
		}
		fmt.Fprintf(stdout, "   %s %s%s\n", mark, c.name, extra)
	}
	if bad > 0 {
		fmt.Fprintf(stderr, "⛔ 自检不过 %d 格 ⇒ 退 2（不给结论）\n", bad)
		return 2
	}
	fmt.Fprintf(stdout, "   ── 自检 %d 格全过（3 负控必命中 · 2 正控不得命中 · 空转必 2 · 干跑档同计数）──\n", len(checks))
	return 0
}

func cwgAnyContains(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}

func cwgInFields(f string) bool {
	for _, x := range cwgFields {
		if x == f {
			return true
		}
	}
	return false
}

// cwgDefaultRoot —— py `DEFAULT_ROOT = dirname(dirname(HERE))`（HERE=脚本所在目录 ⇒ 仓根）。
// Go 入口无 `__file__` ⇒ 起于 `os.Executable()` / cwd 上溯找含 `scripts/gates/` 的祖先（⇒ 仓根），
// 兜底 abspath(cwd)（与同包 clrlDefaultRoot 同体例；见件头未覆盖面②）。
func cwgDefaultRoot() string {
	if exe, err := os.Executable(); err == nil {
		if r := cwgAncestorWithGates(filepath.Dir(exe)); r != "" {
			return r
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if abs, err := filepath.Abs(wd); err == nil {
		wd = abs
	}
	if r := cwgAncestorWithGates(wd); r != "" {
		return r
	}
	return wd
}

// cwgAncestorWithGates —— 自 start 上溯 ≤8 层，返回首个含 `scripts/gates/` 的祖先（无 ⇒ 空串）。
func cwgAncestorWithGates(start string) string {
	cur := start
	for i := 0; i < 8; i++ {
		if fi, err := os.Stat(filepath.Join(cur, "scripts", "gates")); err == nil && fi.IsDir() {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur || parent == "" {
			break
		}
		cur = parent
	}
	return ""
}

const cwgLedgerKouJing = "名录收写工具 · 强制危险档的分类台账（派生真值表 · 供机检与人读）。" +
	"真值来自每件自己头部的一行声明（#zerg-class: write D2|D3）—— 与 core/cmd/zerg/main.go 的 dangerSpec " +
	"同形状（分类是条目自带的字段，不是外挂名单）。本件由 scripts/gates/check-writer-guard.py 的 G0/G1 双向差集机检；" +
	".json 不进 gate check 名录（只收 .py/.sh）也不进脚本清单台账。head = 生成时刻 HEAD 短 sha（只作留痕，机检不比它）。"

// cwgLedgerItem —— 台账一条（键序 file/class/level · 手写档同形）。
type cwgLedgerItem struct {
	File  string `json:"file"`
	Class string `json:"class"`
	Level string `json:"level"`
}

// cwgLedgerDoc —— 台账顶层（键序逐字对齐手写档 ＋ generated_by）。
type cwgLedgerDoc struct {
	Schema      string          `json:"schema"`
	GeneratedBy string          `json:"generated_by"`
	Head        string          `json:"head"`
	Count       int             `json:"count"`
	Regex       string          `json:"regex"`
	KouJing     string          `json:"_口径"`
	Items       []cwgLedgerItem `json:"items"`
}

// cwgHeadShort —— 台账 head（生成时刻 HEAD 短 sha · 8 位 · 只作留痕）。
// 非 git 根 / 取不到 ⇒ ""（不猜；空 head 机检不比它）。
func cwgHeadShort(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--short=8", "HEAD").Output()
	if err != nil {
		return ""
	}
	return pyStrip(string(out))
}

// cwgEmit —— `--emit`：现读 ⇒ 生成写工具台账（唯一写面）。
func cwgEmit(root, out string, dryRun bool, stdout, stderr io.Writer) int {
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "⛔ 不给结论（退 2）：扫描根读不到（%s）—— 空转不当绿\n", root)
		return 2
	}
	declared := cwgCollectDeclared(root)
	files := make([]string, 0, len(declared))
	for rel, cv := range declared {
		if cv[0] == "write" {
			files = append(files, rel)
		}
	}
	sort.Strings(files)
	items := make([]cwgLedgerItem, 0, len(files))
	for _, rel := range files {
		items = append(items, cwgLedgerItem{File: rel, Class: "write", Level: declared[rel][1]})
	}
	led := cwgLedgerDoc{
		Schema:      cwgLedgerSchema,
		GeneratedBy: cwgGeneratedBy,
		Head:        cwgHeadShort(root),
		Count:       len(items),
		Regex:       cwgLedgerRegex,
		KouJing:     cwgLedgerKouJing,
		Items:       items,
	}
	b, err := json.MarshalIndent(led, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "⛔ 台账序列化失败：%s ⇒ 不给结论（rc=2）\n", err)
		return 2
	}
	b = append(b, '\n')
	if dryRun {
		fmt.Fprintf(stdout, "计划（--dry-run · 零副作用 · 未写任何文件）：将生成写工具台账 %s · items %d 条\n",
			out, len(items))
		_, _ = stdout.Write(b)
		return 0
	}
	if out == "-" {
		_, _ = stdout.Write(b)
		return 0
	}
	if dd := filepath.Dir(out); dd != "" {
		if merr := os.MkdirAll(dd, 0o755); merr != nil {
			fmt.Fprintf(stderr, "⛔ 台账落点建不了（%s）：%v ⇒ 不给结论（rc=2）\n", dd, merr)
			return 2
		}
	}
	if werr := os.WriteFile(out, b, 0o644); werr != nil {
		fmt.Fprintf(stderr, "⛔ 台账写不了（%s）：%v ⇒ 不给结论（rc=2）\n", out, werr)
		return 2
	}
	fmt.Fprintf(stdout, "已生成写工具台账 %s · items %d 条 · schema %s\n", out, len(items), cwgLedgerSchema)
	return 0
}

// checkWriterGuardGateMain —— 入口。
func checkWriterGuardGateMain(stdout, stderr io.Writer, argv []string) int {
	var rootArg, jsonArg, outArg *string
	reportOnly, selfTest, help := false, false, false
	emit, dryRun := false, false
	var unknown []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--help":
			help = true
		case a == "--report-only":
			reportOnly = true
		case a == "--self-test":
			selfTest = true
		case a == "--emit":
			emit = true
		case a == "--dry-run":
			dryRun = true
		case a == "--out":
			if i+1 < len(argv) {
				v := argv[i+1]
				outArg = &v
				i++
			} else {
				unknown = append(unknown, a)
			}
		case strings.HasPrefix(a, "--out="):
			v := a[len("--out="):]
			outArg = &v
		case a == "--root":
			if i+1 < len(argv) {
				v := argv[i+1]
				rootArg = &v
				i++
			} else {
				unknown = append(unknown, a)
			}
		case strings.HasPrefix(a, "--root="):
			v := a[len("--root="):]
			rootArg = &v
		case a == "--json":
			if i+1 < len(argv) {
				v := argv[i+1]
				jsonArg = &v
				i++
			} else {
				unknown = append(unknown, a)
			}
		case strings.HasPrefix(a, "--json="):
			v := a[len("--json="):]
			jsonArg = &v
		default:
			unknown = append(unknown, a)
		}
	}
	if help {
		fmt.Fprint(stdout, cwgUsageText())
		return 0
	}
	if len(unknown) > 0 {
		fmt.Fprint(stderr, "⛔ 不给结论（退 2）：未知旗标 "+strings.Join(unknown, " ")+
			" ⇒ 本件只认 --root / --json / --report-only / --self-test / --emit / --out / --dry-run / --help\n")
		fmt.Fprint(stderr, cwgUsageText())
		return 2
	}
	if dryRun && !emit {
		fmt.Fprint(stderr, "⛔ 不给结论（退 2）：`--dry-run` 只与 `--emit` 同用\n")
		return 2
	}
	if emit && (selfTest || jsonArg != nil || reportOnly) {
		fmt.Fprint(stderr, "⛔ 不给结论（退 2）：`--emit` 与 `--json` / `--report-only` / `--self-test` 互斥\n")
		return 2
	}
	var fields []string
	hasFields := false
	if jsonArg != nil {
		hasFields = true
		for _, f := range strings.Split(*jsonArg, ",") {
			f = pyStrip(f)
			if f != "" {
				fields = append(fields, f)
			}
		}
		var bad []string
		for _, f := range fields {
			if !cwgInFields(f) {
				bad = append(bad, f)
			}
		}
		if len(fields) == 0 || len(bad) > 0 {
			msg := "⛔ 不给结论（退 2）：--json 字段不在闭集 " + strings.Join(cwgFields, ",")
			if len(bad) > 0 {
				msg += "；点名非法字段 " + strings.Join(bad, ",")
			} else {
				msg += "；字段面为空"
			}
			fmt.Fprint(stderr, msg+"\n")
			return 2
		}
	}
	if selfTest {
		fmt.Fprint(stdout, "  —— check-writer-guard.py --self-test（合成夹具）——\n")
		return cwgSelftest(stdout, stderr)
	}
	root := cwgDefaultRoot()
	if rootArg != nil {
		root = *rootArg
	}
	if emit {
		out := filepath.Join(root, cwgLedgerRel)
		if outArg != nil {
			out = *outArg
			if out != "-" && !filepath.IsAbs(out) {
				out = filepath.Join(root, out)
			}
		}
		return cwgEmit(root, out, dryRun, stdout, stderr)
	}
	rc, info, notes := cwgCheck(root, reportOnly)
	lrc, linfo, lnotes := cwgCheckLedger(root)
	notes = append(notes, lnotes...)
	if !reportOnly && lrc > rc {
		rc = lrc
	}
	if hasFields {
		cwgEmitJSON(stdout, info, linfo, rc, fields)
		return rc
	}
	cwgPrintHuman(stdout, stderr, info, notes)
	return rc
}
