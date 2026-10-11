// check_multirepo_summary.go —— 判据件 `scripts/gates/check-multirepo-summary.py`（578 行 · 33101 字节 ·
// sha256 2860959e41ee3ba820232bd74ca50d5056f3e6b060388114ea1dfc5a58449958）的 **Go 真搬**
// （M2 · `A7` 仓外仓只读汇总族 · 入口 `checkMultirepoSummaryGateMain`）。
//
// ═══════════════════ 选件读数（现读 · 落单依据）═══════════════════
//
//	· 真件：`scripts/gates/check-multirepo-summary.py` —— 578 行 / 33101 字节 / `-rw-r--r--`（非可执行位）。
//	· 形态：**真跑子进程**（`git` 只读子命令 ＋ 可选命令面制品 `zerg repo status`）＋ 读 `.git` 元数据面逐件 sha256。
//	  输入确定（夹具各造两仓）⇒ 输出逐字可对拍；唯 `--self-test` 里 commit/index 的 sha 随提交时刻变 ⇒ 属非确定面（见回执未覆盖面）。
//	· 退码三档：0 汇总跑通 / 1 只读自证红 / 2 拒写·用法错·缺件·自检不过。
//	· 旗标：`--root` · `--docs-root` · `--zerg` · `--source {zerg,git}` · `--prove-readonly [{narrow,tree}]`
//	  · `--json` · `--list-rules` · `--self-test` · `--version` · `-h/--help`；写词预扫**先于**参数解析（命中 ⇒ rc=2）。
//	· 前缀：`mrs`（本件私有符号一律 `mrs` 开头；★ 并行路撞名防护）。
//
// ═══════════════════ 出厂声明（a）默认根定位差异 ═══════════════════
//
//	· py `HERE = dirname(abspath(__file__))` · `DEFAULT_ROOT = dirname(dirname(HERE))` —— 从**脚本自身位置**上溯两层。
//	  Go 无该语境 ⇒ `--root` 缺省改为 **从 cwd 上溯**找最近的「含 `scripts/gates/` 子目录」的层（`mrsDefaultRepoRoot`）。
//	  cwd 落在真仓内（仓根 / `scripts/gates`）时两者逐字同构；cwd 在仓外时分家 ⇒ 未覆盖面①。
//	· 旋钮面：py 的根只由脚本位置推得（**无**根旗标）—— 本件 `--root`/`--docs-root`/`--zerg` 与 py **同名同义**；
//	  py 另认 env `ZERG_DOCS_ROOT`/`ZERG_DOCS_ALT`/`ZERG_DOCS`（本件逐字照搬，空串回落）。
//
// ═══════════════════ 出厂声明（b）★自指检查 ═══════════════════
//
//	· 本门**自身扫描面**是否含 `core/`？现读 py 全文：**零源码目录扫描** ——
//	  唯一 `os.walk` 命中 1 处（`repo_fingerprint()` 里，面 = `<gitdir>/refs`）；`os.listdir`/`glob`/`fnmatch` 命中 0。
//	  `tree` 档清单来自 `git ls-files`（仓内跟踪件 ＋ 未忽略未跟踪件），只用于**逐件 sha256 摘要**、不进任何“按内容匹配”判据。
//	  ⇒ 本门**不按件名/内容扫 `core/`**，新型扫描面不含本件 ⇒ 自指① 不适用，无需影子根（依据见回执 selfref 字段）。
//	· 自指②（是否落在**他门**扫描面内）：本件正文**零绝对路径字面量**（`/` 开头的私有根、`~` 展开面一律不写）、
//	  中文仅出现在**输出文案**（判据值），不写被扫形态的真引号串。按“一旦被跟踪”复核见回执。
//
// ═══════════════════ 遍历 / 排序点（逐点交代）═══════════════════
//
//	① 指纹面 `repo_fingerprint()`：`META_NAMES` 固定序取件（键唯一）＋ `os.walk(<gitdir>/refs)`（同目录 `sorted(fn)`），
//	   `tree` 档清单 = `git ls-files -z` ＋ `--others --exclude-standard`，末了 `sorted(set(rels))`。
//	   ★ 该 map **只用于 `len()` 与 `diff_fingerprint()` 的 `sorted(union)` 差集** ⇒ 遍历序**不进输出行序**。
//	② **输出行序决定者** = 三处：ⓐ 表体两行 = `repos` 切片序（`主仓` 先、`文档仓` 后，见 `mrsRun`）；
//	   ⓑ JSON `items` 同序；ⓒ 自证差异明细 = `diff_fingerprint` 的 `sorted(union(before,after))`（码点序 ⇒ `sort.Strings`）。
//	   ⛔ 本件**零 Go map 迭代进输出**（map 仅 O(1) 取值）。
//
// ═══════════════════ `os.path.join` ⟷ 路径词法面（逐处自写）═══════════════════
//
//	· 本件**全程自写** `mrsPyJoin`/`mrsPyNormpath`/`mrsPyAbspath`/`mrsPyRelpath`/`mrsPyDirname`（✗ 不用 `filepath.*`）。
//	· py 调用点：`join`（拼 `.git`、`refs`、`bin/zerg`、夹具件、`<gitdir>/<n>`、`<树>/<rel>`）—— 普通段，
//	  三语义差（吃 `..` · 后段绝对丢前段 · 空段留尾 `/`）在**主逻辑面不触发**；仍全文自写以求一处语义面统一。
//	· `normpath`（`git_dir()` 的 worktree `gitdir:` 分支）· `abspath`（`HERE`/`run()` 的仓根）·
//	  `relpath`（指纹键 `<.git>/` ＋ `relpath(p, gd)`）· `dirname`（`DEFAULT_ROOT`/`discover_docs_root`）⇒ 逐字自写。
//	· 本件**零 `realpath`** ⇒ 不涉“悬空链接回词法路径 ⟷ EvalSymlinks 报错”那一面。
//	· `str.strip()` 是 py 的 Unicode 空白（含 `\x1c-\x1f`）⇒ `mrsPyStrip` 自写；切片 `s[:N]` 按码点 ⇒ `mrsPyRunes`。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ── 常量 ────────────────────────────────────────────────────────────────────
const (
	mrsVersion  = "check-multirepo-summary.py v1.0.0"
	mrsSelfName = "check-multirepo-summary.py"
	mrsProg     = "check-multirepo-summary.py"
	mrsFields   = "head,branch,path,status,untracked"
)

// mrsWriteWords —— 写词闭集（负控唯一真源；py `WRITE_WORDS`，逐字照搬 · 30 词）。
var mrsWriteWords = []string{
	"add", "commit", "push", "stage", "amend", "tag", "fetch", "pull", "clone",
	"merge", "rebase", "reset", "revert", "checkout", "restore", "clean", "rm",
	"mv", "apply", "stash", "update-index", "write-tree", "hash-object", "gc",
	"prune", "repack", "unlock", "commit-tree", "symbolic-ref-set", "config-set",
}

// mrsReadonlySubs —— 只读 git 子命令白名单（唯一入口用；不在名单 = 拒）。
var mrsReadonlySubs = []string{"rev-parse", "status", "ls-files", "for-each-ref", "cat-file"}

// mrsMetaNames —— 窄口径指纹面（`.git` 里本命令唯一可能碰到的元数据）。
var mrsMetaNames = []string{"HEAD", "index", "packed-refs", "ORIG_HEAD", "FETCH_HEAD",
	"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG"}

// ── py 词法面自写函数 ────────────────────────────────────────────────────────

// mrsPyJoin —— 逐字复刻 CPython `posixpath.join`（含三语义差）。
func mrsPyJoin(a string, p ...string) string {
	path := a
	for _, b := range p {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// mrsPyNormpath —— 逐字复刻 CPython `posixpath.normpath`（★ 保留**恰好两个**前导斜杠）。
func mrsPyNormpath(p string) string {
	if p == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(p, "/") {
		initial = 1
		if strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///") {
			initial = 2
		}
	}
	var nc []string
	for _, c := range strings.Split(p, "/") {
		if c == "" || c == "." {
			continue
		}
		if c != ".." || (initial == 0 && len(nc) == 0) || (len(nc) > 0 && nc[len(nc)-1] == "..") {
			nc = append(nc, c)
		} else if len(nc) > 0 {
			nc = nc[:len(nc)-1]
		}
	}
	out := strings.Join(nc, "/")
	if initial > 0 {
		out = strings.Repeat("/", initial) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// mrsPyDirname —— 逐字复刻 CPython `posixpath.dirname`。
func mrsPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// mrsPyAbspath —— 逐字复刻 CPython `posixpath.abspath`（相对则先拼 cwd 再 normpath）。
func mrsPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		p = mrsPyJoin(wd, p)
	}
	return mrsPyNormpath(p)
}

func mrsSplitNonEmpty(p string) []string {
	var out []string
	for _, x := range strings.Split(p, "/") {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

// mrsPyRelpath —— 逐字复刻 CPython `posixpath.relpath`。
func mrsPyRelpath(path, start string) string {
	if path == "" {
		return ""
	}
	sl := mrsSplitNonEmpty(mrsPyAbspath(start))
	pl := mrsSplitNonEmpty(mrsPyAbspath(path))
	i := 0
	for i < len(sl) && i < len(pl) && sl[i] == pl[i] {
		i++
	}
	var rel []string
	for k := i; k < len(sl); k++ {
		rel = append(rel, "..")
	}
	for k := i; k < len(pl); k++ {
		rel = append(rel, pl[k])
	}
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

// mrsPyStrip —— 逐字复刻 py `str.strip()`（Unicode 空白 · ★ 含 `\x1c-\x1f`、`\x85`、`\xa0`）。
func mrsPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	return false
}

func mrsPyStrip(s string) string {
	return strings.TrimRightFunc(strings.TrimLeftFunc(s, mrsPyIsSpace), mrsPyIsSpace)
}

// mrsPyRunes —— py 切片 `s[:n]` 按**码点**。
func mrsPyRunes(s string, n int) string {
	if n < 0 {
		return s
	}
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

// mrsPyRepr —— 逐字复刻 py `repr(str)`（本件只用于 ASCII 命令名/旗标）。
func mrsPyRepr(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\'':
			b.WriteString("\\'")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 || r == 0x7f {
				b.WriteString(fmt.Sprintf("\\x%02x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// mrsPyListRepr —— 逐字复刻 py `list[str]` 的 repr：`['a', 'b']`（空表 = `[]`）。
func mrsPyListRepr(items []string) string {
	parts := make([]string, 0, len(items))
	for _, x := range items {
		parts = append(parts, mrsPyRepr(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// mrsPyStr —— py `%s` 对 any 的形态（bool ⇒ True/False · int ⇒ 十进制 · str ⇒ 原串）。
func mrsPyStr(v any) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}

// mrsPyEq —— py `==`（类型不符 ⇒ False）。
func mrsPyEq(a, b any) bool {
	switch x := a.(type) {
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case int:
		y, ok := b.(int)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	}
	return false
}

// mrsPad —— py `%-Ns` / `%Ns`（★ 按**码点**计宽）。
func mrsPad(s string, width int, left bool) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	pad := strings.Repeat(" ", width-n)
	if left {
		return s + pad
	}
	return pad + s
}

func mrsPadInt(n, width int) string {
	return mrsPad(strconv.Itoa(n), width, false)
}

// ── 写旗标预扫（牙 A2）──────────────────────────────────────────────────────
func mrsParseWriteFlags(argv []string) []string {
	var hits []string
	for _, tok := range argv {
		t := mrsPyStrip(tok)
		if t == "" || t == "--" {
			continue
		}
		name := mrsPyStrip(strings.SplitN(strings.TrimLeft(t, "-"), "=", 2)[0])
		name = strings.ToLower(name)
		for _, w := range mrsWriteWords {
			if name == w {
				hits = append(hits, t)
				break
			}
		}
	}
	return hits
}

// ── 只读 git 入口（牙 A4）──────────────────────────────────────────────────
type mrsRefused struct{ msg string }

func (e mrsRefused) Error() string { return e.msg }

func mrsGitRo(repo string, args []string, timeout time.Duration) (string, error) {
	if len(args) == 0 {
		return "", mrsRefused{"空 git 调用 ⇒ 拒"}
	}
	sub := args[0]
	ok := false
	for _, s := range mrsReadonlySubs {
		if s == sub {
			ok = true
			break
		}
	}
	if !ok {
		return "", mrsRefused{fmt.Sprintf("git 子命令 %s 不在只读白名单 %s 内 ⇒ 拒（本件绝不代跑任何 git 写子命令）",
			mrsPyRepr(sub), mrsPyListRepr(mrsReadonlySubs))}
	}
	cmd := append([]string{"git", "--no-optional-locks", "-C", repo, "-c", "core.quotepath=false"}, args...)
	out, err := mrsRunCmd(cmd, "", nil, timeout)
	if err != nil {
		if ee, isExit := err.(*mrsCmdErr); isExit {
			return "", fmt.Errorf("退码 %d：%s", ee.rc, mrsPyRunes(mrsPyStrip(ee.msg), 200))
		}
		return "", fmt.Errorf("跑不起来：%s（%s）", strings.Join(cmd, " "), err)
	}
	return out, nil
}

// mrsCmdErr —— 子进程非零退码。
type mrsCmdErr struct {
	rc  int
	msg string
}

func (e *mrsCmdErr) Error() string { return e.msg }

// mrsRunCmd —— 跑一条命令，回 stdout；非零退码 ⇒ `*mrsCmdErr`（stdout 兜底 stderr）。
func mrsRunCmd(cmd []string, dir string, env []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	if dir != "" {
		c.Dir = dir
	}
	if env != nil {
		c.Env = env
	}
	var so, se bytes.Buffer
	c.Stdout = &so
	c.Stderr = &se
	err := c.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			msg := se.String()
			if strings.TrimSpace(msg) == "" {
				msg = so.String()
			}
			return so.String(), &mrsCmdErr{rc: ee.ExitCode(), msg: msg}
		}
		return so.String(), err
	}
	return so.String(), nil
}

// mrsFileSha —— 件 sha256 十六进制（1MB 分块）。
func mrsFileSha(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func mrsIsDir(p string) bool {
	st, err := os.Stat(p) // ★ 跟随符号链接（py os.path.isdir 同义）
	return err == nil && st.IsDir()
}

func mrsIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// mrsIsRepo —— py `is_repo()`。
func mrsIsRepo(path string) bool {
	if path == "" || !mrsIsDir(path) {
		return false
	}
	if _, err := mrsGitRo(path, []string{"rev-parse", "--git-dir"}, 20*time.Second); err != nil {
		return false
	}
	return true
}

// mrsGitDir —— py `git_dir()`（worktree/子模 `.git` 是文件 ⇒ 读 `gitdir:` 行）。
func mrsGitDir(repo string) (string, bool) {
	g := mrsPyJoin(repo, ".git")
	if mrsIsDir(g) {
		return g, true
	}
	if mrsIsFile(g) {
		b, err := os.ReadFile(g)
		if err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				ls := mrsPyStrip(line)
				if strings.HasPrefix(ls, "gitdir:") {
					rest := strings.SplitN(line, ":", 2)[1]
					return mrsPyNormpath(mrsPyJoin(repo, mrsPyStrip(rest))), true
				}
			}
		}
	}
	return "", false
}

// ── 读数（两个来源）────────────────────────────────────────────────────────
type mrsRow struct {
	path      string
	untracked bool
	status    string
	sha       string
}

// mrsReadZerg —— 口径来源①：既有命令面 `zerg repo status --root <仓> --json <字段>`。
func mrsReadZerg(zerg, repo string, timeout time.Duration) ([]mrsRow, string) {
	cmd := []string{zerg, "repo", "status", "--root", repo, "--json", mrsFields}
	out, err := mrsRunCmd(cmd, "", nil, timeout)
	if err != nil {
		var ee *mrsCmdErr
		if errors.As(err, &ee) {
			if ee.rc != 0 && ee.rc != 1 { // 1 = 有未提交（不是错）
				msg := ee.msg
				if strings.TrimSpace(msg) == "" {
					msg = out
				}
				return nil, fmt.Sprintf("退码 %d（合法只有 0/1）：%s", ee.rc, mrsPyRunes(mrsPyStrip(msg), 160))
			}
			// rc 0/1：继续解析 out
		} else {
			return nil, fmt.Sprintf("跑不起来：%s", err)
		}
	}
	var d map[string]any
	if e := json.Unmarshal([]byte(out), &d); e != nil {
		return nil, fmt.Sprintf("stdout 不是 JSON：%s", e)
	}
	itemsAny, _ := d["items"].([]any)
	if len(itemsAny) == 0 {
		return nil, "items 空（不给结论 —— 不把「读不到」当「干净」）"
	}
	var rows []mrsRow
	for _, itAny := range itemsAny {
		it, _ := itAny.(map[string]any)
		path := mrsJSONGetStr(it, "path", "")
		status := mrsJSONGetStr(it, "status", "?")
		unt := mrsJSONGetStr(it, "untracked", "") == "\u662f" || status == "??"
		sha := mrsJSONGetStr(it, "head", "?")
		rows = append(rows, mrsRow{path: path, untracked: unt, status: status, sha: sha})
	}
	return rows, ""
}

func mrsJSONGetStr(m map[string]any, k, dflt string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return dflt
	}
	if s, ok := v.(string); ok {
		return s
	}
	return dflt
}

// mrsReadGit —— 口径来源②：`git -C <仓> --no-optional-locks status --porcelain`。
func mrsReadGit(repo string, gitStatusCmd []string) ([]mrsRow, string, error) {
	var out string
	if gitStatusCmd != nil {
		o, err := mrsRunCmd(gitStatusCmd, repo, nil, 60*time.Second)
		if err != nil {
			if _, isExit := err.(*mrsCmdErr); !isExit {
				return nil, "", err
			}
		}
		out = o
	} else {
		o, err := mrsGitRo(repo, []string{"status", "--porcelain"}, 60*time.Second)
		if err != nil {
			return nil, "", err
		}
		out = o
	}
	sha, err := mrsGitRo(repo, []string{"rev-parse", "HEAD"}, 60*time.Second)
	if err != nil {
		return nil, "", err
	}
	sha = mrsPyStrip(sha)
	br := "?"
	if o, err := mrsGitRo(repo, []string{"rev-parse", "--abbrev-ref", "HEAD"}, 60*time.Second); err == nil {
		br = mrsPyStrip(o)
	}
	var rows []mrsRow
	for _, line := range mrsSplitLines(out) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		status := mrsPyStrip(mrsPyRunes(line, 2))
		path := mrsPyStrip(mrsPyDropRunes(line, 3))
		if strings.HasPrefix(path, "\"") && strings.HasSuffix(path, "\"") {
			if u := mrsUnquoteGitPath(path); u != "" {
				path = u
			}
		}
		rows = append(rows, mrsRow{path: path, untracked: status == "??", status: status, sha: sha})
	}
	return rows, br, nil
}

// mrsSplitLines —— py `str.splitlines()`（`\n`·`\r`·`\r\n` 皆断行）。
func mrsSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		if c == '\r' {
			out = append(out, cur.String())
			cur.Reset()
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			continue
		}
		cur.WriteByte(c)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// mrsPyDropRunes —— py 切片 `s[N:]` 按码点。
func mrsPyDropRunes(s string, n int) string {
	if n <= 0 {
		return s
	}
	rs := []rune(s)
	if len(rs) <= n {
		return ""
	}
	return string(rs[n:])
}

// mrsUnquoteGitPath —— 复刻 py `path[1:-1].encode('utf-8').decode('unicode_escape').encode('latin1','ignore').decode('utf-8','ignore')`。
func mrsUnquoteGitPath(quoted string) string {
	rs := []rune(quoted)
	if len(rs) < 2 {
		return ""
	}
	inner := string(rs[1 : len(rs)-1])
	// ① 转义解码：反斜杠转义 + 非转义字节按 latin1 取码点
	var mid []rune
	b := []byte(inner)
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' || i+1 >= len(b) {
			mid = append(mid, rune(b[i]))
			continue
		}
		i++
		switch c := b[i]; c {
		case 'n':
			mid = append(mid, '\n')
		case 't':
			mid = append(mid, '\t')
		case 'r':
			mid = append(mid, '\r')
		case '\\':
			mid = append(mid, '\\')
		case '\'':
			mid = append(mid, '\'')
		case '"':
			mid = append(mid, '"')
		case 'a':
			mid = append(mid, 0x07)
		case 'b':
			mid = append(mid, 0x08)
		case 'f':
			mid = append(mid, 0x0c)
		case 'v':
			mid = append(mid, 0x0b)
		case 'x':
			if i+2 < len(b) {
				if v, ok := mrsHexVal(b[i+1 : i+3]); ok {
					mid = append(mid, rune(v))
					i += 2
					continue
				}
			}
			mid = append(mid, '\\')
		case 'u':
			if i+4 < len(b) {
				if v, ok := mrsHexVal(b[i+1 : i+5]); ok {
					mid = append(mid, rune(v))
					i += 4
					continue
				}
			}
			mid = append(mid, '\\')
		default:
			if c >= '0' && c <= '7' {
				j := i
				for j < len(b) && j < i+3 && b[j] >= '0' && b[j] <= '7' {
					j++
				}
				if v, e := strconv.ParseInt(string(b[i:j]), 8, 32); e == nil {
					mid = append(mid, rune(v))
					i = j - 1
					continue
				}
			}
			mid = append(mid, '\\', rune(c))
		}
	}
	// ② encode('latin1','ignore')：码点 >255 丢弃
	var lb []byte
	for _, r := range mid {
		if r <= 0xff {
			lb = append(lb, byte(r))
		}
	}
	// ③ decode('utf-8','ignore')：非法序列按字节丢弃
	return strings.ToValidUTF8(string(lb), "")
}

func mrsHexVal(b []byte) (int, bool) {
	v := 0
	for _, c := range b {
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			return 0, false
		}
		v = v*16 + d
	}
	return v, true
}

// ── 读数汇总 ───────────────────────────────────────────────────────────────
type mrsRead struct {
	head      string
	branch    string
	dirty     int
	untracked int
	total     int
	source    string
}

// mrsSummarize —— py `summarize()` ⇒ (读数, 错误)。
func mrsSummarize(repo, zerg, source string) (*mrsRead, string) {
	var zergErr string
	if source == "zerg" && zerg != "" && mrsIsFile(zerg) {
		rows, err := mrsReadZerg(zerg, repo, 60*time.Second)
		if err == "" {
			head := ""
			if len(rows) > 0 {
				head = rows[0].sha
			} else if o, e := mrsGitRo(repo, []string{"rev-parse", "HEAD"}, 60*time.Second); e == nil {
				head = mrsPyStrip(o)
			}
			branch := "?"
			if o, e := mrsGitRo(repo, []string{"rev-parse", "--abbrev-ref", "HEAD"}, 60*time.Second); e == nil {
				branch = mrsPyStrip(o)
			}
			dirty, unt := 0, 0
			for _, r := range rows {
				if !r.untracked {
					dirty++
				} else {
					unt++
				}
			}
			return &mrsRead{head: head, branch: branch, dirty: dirty, untracked: unt, total: len(rows),
				source: "zerg repo status --root <仓> --json " + mrsFields}, ""
		}
		zergErr = err
	}
	if source == "zerg" && zergErr != "" {
		return nil, zergErr
	}
	rows, br, err := mrsReadGit(repo, nil)
	if err != nil {
		var rf mrsRefused
		if errors.As(err, &rf) {
			return nil, "拒：" + rf.Error()
		}
		return nil, "读不到：" + err.Error()
	}
	o, e := mrsGitRo(repo, []string{"rev-parse", "HEAD"}, 60*time.Second)
	if e != nil {
		return nil, "读不到：" + e.Error()
	}
	head := mrsPyStrip(o)
	dirty, unt := 0, 0
	for _, r := range rows {
		if !r.untracked {
			dirty++
		} else {
			unt++
		}
	}
	return &mrsRead{head: head, branch: br, dirty: dirty, untracked: unt, total: len(rows),
		source: "git --no-optional-locks status --porcelain"}, ""
}

// ── 指纹（只读自证）────────────────────────────────────────────────────────
// mrsReadFails —— B 类读失败登记面（refs 目录遍历失败 ⇒ 逐处登记「目录 + 原因」）。
// ★ B 类真口子修（与 py **有意分叉** · 同族先例 check_error_kinds.go 的 cekReadFails）：
//
//	原 py 语义 = `os.walk(onerror=None)` 读不了的目录**静默跳过** ⇒ 判据面被抽空也不出声。
//	本件收紧：读失败 ⇒ 登记 + 入口判红（stdout BLOCKED 头 + 逐件点名）⇒ rc=2（✗ 不再静默少判）。
//	自清点：每次 mrsRun 起手清空 ⇒ 不许跨调用残留。目录面自成一句（标「读不了目录」）。
var mrsReadFails []string

// mrsNoteReadFail —— 登记一处读失败（同一目录只登一次 ⇒ ✗ 不重复点名）。
func mrsNoteReadFail(what string) {
	for _, u := range mrsReadFails {
		if u == what {
			return
		}
	}
	mrsReadFails = append(mrsReadFails, what)
}

func mrsRepoFingerprint(repo string, tree bool) map[string]string {
	fp := map[string]string{}
	gd, ok := mrsGitDir(repo)
	if ok {
		for _, n := range mrsMetaNames {
			p := mrsPyJoin(gd, n)
			if mrsIsFile(p) {
				if s, err := mrsFileSha(p); err == nil {
					fp["<.git>/"+n] = s
				}
			}
		}
		refs := mrsPyJoin(gd, "refs")
		// py os.walk(refs)：只在 refs 存在时下钻；同目录 sorted(fn)；键唯一 ⇒ 序不进输出。
		var walk func(dir string)
		walk = func(dir string) {
			ents, err := os.ReadDir(dir)
			if err != nil {
				// ★ B 类真口子修（原读不了的目录静默跳过 · ✗ 不再静默）。
				// ★ 只有「在盘却读不了」才是真读失败（✗ 不把「目录本就缺席」记成读失败 · 与同族先例 cek 一致）。
				if fi, serr := os.Stat(dir); serr == nil && fi.IsDir() {
					mrsNoteReadFail(dir + "（读不了目录：" + err.Error() + "）")
				}
				return
			}
			var files, dirs []string
			for _, e := range ents {
				if e.IsDir() {
					dirs = append(dirs, e.Name())
				} else {
					files = append(files, e.Name())
				}
			}
			sort.Strings(files)
			for _, f := range files {
				p := mrsPyJoin(dir, f)
				if s, err := mrsFileSha(p); err == nil {
					fp["<.git>/"+mrsPyRelpath(p, gd)] = s
				}
			}
			for _, d := range dirs {
				walk(mrsPyJoin(dir, d))
			}
		}
		walk(refs)
	}
	if o, err := mrsGitRo(repo, []string{"status", "--porcelain"}, 60*time.Second); err == nil {
		h := sha256.Sum256([]byte(o))
		fp["<porcelain 全文>"] = hex.EncodeToString(h[:])
	} else {
		fp["<porcelain 全文>"] = fmt.Sprintf("读不到：%s", err)
	}
	if tree {
		var rels []string
		okAll := true
		for _, args := range [][]string{{"ls-files", "-z"}, {"ls-files", "-z", "--others", "--exclude-standard"}} {
			o, err := mrsGitRo(repo, args, 60*time.Second)
			if err != nil {
				fp["<工作树清单>"] = fmt.Sprintf("读不到：%s", err)
				okAll = false
				break
			}
			for _, x := range strings.Split(o, "\x00") {
				if x != "" {
					rels = append(rels, x)
				}
			}
		}
		if okAll {
			sorted := append([]string(nil), rels...)
			sort.Strings(sorted)
			seen := map[string]bool{}
			for _, rel := range sorted {
				if seen[rel] {
					continue
				}
				seen[rel] = true
				p := mrsPyJoin(repo, rel)
				st, err := os.Lstat(p)
				if err != nil || st.Mode()&os.ModeSymlink != 0 || !mrsIsFile(p) {
					continue
				}
				if s, err := mrsFileSha(p); err == nil {
					fp["<树>/"+rel] = s
				} else {
					fp["<树>/"+rel] = "读不到"
				}
			}
		}
	}
	return fp
}

type mrsDiffEntry struct {
	k string
	b *string
	a *string
}

func mrsDiffFingerprint(before, after map[string]string) []mrsDiffEntry {
	keyset := map[string]bool{}
	for k := range before {
		keyset[k] = true
	}
	for k := range after {
		keyset[k] = true
	}
	keys := make([]string, 0, len(keyset))
	for k := range keyset {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var diffs []mrsDiffEntry
	for _, k := range keys {
		bv, bok := before[k]
		av, aok := after[k]
		if !bok && !aok {
			continue
		}
		if bok != aok || (bok && bv != av) {
			var bp, ap *string
			if bok {
				v := bv
				bp = &v
			}
			if aok {
				v := av
				ap = &v
			}
			diffs = append(diffs, mrsDiffEntry{k: k, b: bp, a: ap})
		}
	}
	return diffs
}

// ── 汇总主体 ───────────────────────────────────────────────────────────────
func mrsDiscoverDocsRoot(root, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("ZERG_DOCS_ROOT"); env != "" {
		return env
	}
	cand := os.Getenv("ZERG_DOCS_ALT")
	if cand == "" {
		cand = os.Getenv("ZERG_DOCS")
	}
	if cand == "" {
		cand = mrsPyJoin(mrsPyDirname(mrsPyAbspath(root)), "Zerg-内部文档")
	}
	return cand
}

type mrsRepoLabel struct{ label, root string }

// mrsReadRow —— 一仓读数行（label/root ＋ 读数）。
type mrsReadRow struct {
	label, root string
	rd          *mrsRead
}

// mrsProofPerRepo —— 只读自证每仓计数。
type mrsProofPerRepo struct {
	label              string
	before, after, dif int
}

// mrsProofDiff —— 只读自证逐件差异（值可缺席 ⇒ nil ⇒ JSON null）。
type mrsProofDiff struct {
	repo, file string
	b, a       *string
}

func mrsRun(stdout io.Writer, root, docsRoot, source, prove string, jsonOut bool, zerg string) int {
	mrsReadFails = nil // ★ 自清：包级登记面不许跨调用残留（否则上一 Test 残留一进门就 BLOCKED）
	repos := []mrsRepoLabel{{"主仓", mrsPyAbspath(root)}, {"文档仓", mrsPyAbspath(docsRoot)}}
	var problems []string
	for _, rl := range repos {
		if !mrsIsRepo(rl.root) {
			problems = append(problems, fmt.Sprintf("缺件/不给结论：%s「%s」不是 git 仓（或无 .git）⇒ 不拿「读不到」当「干净」", rl.label, rl.root))
		}
	}
	if len(problems) > 0 {
		fmt.Fprintln(stdout, "⛔ 不给结论（rc=2）：")
		for _, p := range problems {
			fmt.Fprintf(stdout, "   · %s\n", p)
		}
		return 2
	}

	var fpBefore map[string]map[string]string
	if prove != "" {
		fpBefore = map[string]map[string]string{}
		for _, rl := range repos {
			fpBefore[rl.label] = mrsRepoFingerprint(rl.root, prove == "tree")
		}
	}
	// ★ B 类真口子修：refs 目录遍历（只读自证指纹面）跑完 ⇒ 有登记即 BLOCKED rc=2（✗ 不给结论）。
	if len(mrsReadFails) > 0 {
		fmt.Fprintf(stdout, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(mrsReadFails))
		for _, u := range mrsReadFails {
			fmt.Fprintf(stdout, "  [读失败] %s\n", u)
		}
		return 2
	}

	type readRow = mrsReadRow
	var reads []readRow
	var errs []string
	for _, rl := range repos {
		rd, err := mrsSummarize(rl.root, zerg, source)
		if err != "" {
			errs = append(errs, fmt.Sprintf("%s「%s」：%s", rl.label, rl.root, err))
		} else {
			reads = append(reads, readRow{rl.label, rl.root, rd})
		}
	}
	if len(errs) > 0 {
		fmt.Fprintln(stdout, "⛔ 不给结论（rc=2）：")
		for _, e := range errs {
			fmt.Fprintf(stdout, "   · %s\n", e)
		}
		return 2
	}

	totalDirty, totalUnt := 0, 0
	for _, rr := range reads {
		totalDirty += rr.rd.dirty
		totalUnt += rr.rd.untracked
	}

	type perRepo = mrsProofPerRepo
	var proofPer []perRepo
	var proofDiffs []mrsProofDiff
	diffCount := 0
	rc := 0
	hasProof := false
	if prove != "" {
		hasProof = true
		for _, rl := range repos {
			b := fpBefore[rl.label]
			a := mrsRepoFingerprint(rl.root, prove == "tree")
			d := mrsDiffFingerprint(b, a)
			for _, de := range d {
				proofDiffs = append(proofDiffs, mrsProofDiff{rl.label, de.k, de.b, de.a})
			}
			proofPer = append(proofPer, perRepo{rl.label, len(b), len(a), len(d)})
		}
		diffCount = len(proofDiffs)
		if diffCount > 0 {
			rc = 1
		}
	}

	if jsonOut {
		fmt.Fprintln(stdout, mrsJSONDoc(reads, totalDirty, totalUnt, prove, hasProof, proofPer, proofDiffs, diffCount))
		return rc
	}

	fmt.Fprintln(stdout, "── `A7` 仓外仓只读汇总（主仓 + 文档仓 · **一条命令**）──")
	fmt.Fprintf(stdout, "  口径来源: %s\n", reads[0].rd.source)
	fmt.Fprintf(stdout, "  %s %s %s %s %s %s\n",
		mrsPad("仓", 8, true), mrsPad("HEAD", 42, true), mrsPad("分支", 8, true),
		mrsPad("脏件数", 8, false), mrsPad("未跟踪数", 10, false), mrsPad("合计", 8, false))
	for _, rr := range reads {
		fmt.Fprintf(stdout, "  %s %s %s %s %s %s\n",
			mrsPad(rr.label, 8, true), mrsPad(rr.rd.head, 42, true), mrsPad(rr.rd.branch, 8, true),
			mrsPadInt(rr.rd.dirty, 8), mrsPadInt(rr.rd.untracked, 10), mrsPadInt(rr.rd.total, 8))
	}
	fmt.Fprintf(stdout, "  ── 两仓合计：未提交 %d 处（脏 %d · 未跟踪 %d）\n", totalDirty+totalUnt, totalDirty, totalUnt)
	fmt.Fprintln(stdout, "  · 脏 ≠ 错：口径照 `zerg repo status`（退码 1 = 有未提交、**不是错**）；本件只报读数")
	fmt.Fprintf(stdout, "  · 写旗标闭集（负控唯一真源 · %d 词）：%s\n", len(mrsWriteWords), strings.Join(mrsWriteWords, " "))

	if prove != "" {
		modeDesc := "HEAD + .git 元数据面 + porcelain 全文"
		if prove == "tree" {
			modeDesc = "上一行再加**git 眼里的工作树**逐件 sha256（跟踪件 + 未忽略的未跟踪件）"
		}
		fmt.Fprintf(stdout, "── 只读自证（%s 口径：%s）──\n", prove, modeDesc)
		for _, row := range proofPer {
			mark := "✓ 逐件不变"
			if row.dif != 0 {
				mark = "✗"
			}
			fmt.Fprintf(stdout, "  %s：前 %d 件 / 后 %d 件 · 差异 %d 件 %s\n", row.label, row.before, row.after, row.dif, mark)
		}
		for _, d := range proofDiffs {
			bs, as := "", ""
			if d.b != nil {
				bs = *d.b
			}
			if d.a != nil {
				as = *d.a
			}
			fmt.Fprintf(stdout, "      ✗ %s %s：%s ⇒ %s\n", d.repo, d.file, mrsPyRunes(bs, 16), mrsPyRunes(as, 16))
		}
		if diffCount > 0 {
			fmt.Fprintf(stdout, "  ⇒ rc=1：跑本次汇总期间有 %d 处指纹变了 ⇒ **只读这句话不成立**（逐处已打印）\n", diffCount)
		} else {
			fmt.Fprintln(stdout, "  ⇒ 零副作用：两仓 `HEAD` 与件 `sha256` 逐件不变（差异 0 件）")
		}
	}
	if rc == 0 {
		note := "未要求自证"
		if prove != "" {
			note = "只读自证通过"
		}
		fmt.Fprintf(stdout, "⇒ rc=0：两仓读数已出（%s）\n", note)
	}
	return rc
}

// mrsJSONStr —— py `json.dumps(..., ensure_ascii=False)` 对 str 的转义。
func mrsJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func mrsJSONStrOrNull(p *string) string {
	if p == nil {
		return "null"
	}
	return mrsJSONStr(*p)
}

func mrsJSONDoc(rows []mrsReadRow, totalDirty, totalUnt int, prove string, hasProof bool,
	proofPer []mrsProofPerRepo, proofDiffs []mrsProofDiff, diffCount int) string {
	var b strings.Builder
	b.WriteString("{\"schema\": \"zerg/v1\", \"kind\": \"MultiRepoSummary\", \"items\": [")
	for i, rr := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{")
		b.WriteString("\"label\": " + mrsJSONStr(rr.label))
		b.WriteString(", \"root\": " + mrsJSONStr(rr.root))
		b.WriteString(", \"head\": " + mrsJSONStr(rr.rd.head))
		b.WriteString(", \"branch\": " + mrsJSONStr(rr.rd.branch))
		b.WriteString(", \"dirty\": " + strconv.Itoa(rr.rd.dirty))
		b.WriteString(", \"untracked\": " + strconv.Itoa(rr.rd.untracked))
		b.WriteString(", \"total\": " + strconv.Itoa(rr.rd.total))
		b.WriteString(", \"source\": " + mrsJSONStr(rr.rd.source))
		b.WriteString("}")
	}
	b.WriteString("], \"meta\": {\"count\": " + strconv.Itoa(len(rows)))
	b.WriteString(", \"dirty\": " + strconv.Itoa(totalDirty))
	b.WriteString(", \"untracked\": " + strconv.Itoa(totalUnt))
	b.WriteString(", \"source\": " + mrsJSONStr("local（本机）"))
	b.WriteString(", \"tool\": " + mrsJSONStr(mrsVersion))
	b.WriteString("}, \"proof\": ")
	if !hasProof {
		b.WriteString("null")
	} else {
		b.WriteString("{\"per_repo\": [")
		for i, p := range proofPer {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("{\"label\": " + mrsJSONStr(p.label) + ", \"before\": " + strconv.Itoa(p.before) +
				", \"after\": " + strconv.Itoa(p.after) + ", \"diff\": " + strconv.Itoa(p.dif) + "}")
		}
		b.WriteString("], \"mode\": " + mrsJSONStr(prove))
		b.WriteString(", \"diff_count\": " + strconv.Itoa(diffCount))
		b.WriteString(", \"diffs\": [")
		for i, d := range proofDiffs {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("{\"repo\": " + mrsJSONStr(d.repo) + ", \"file\": " + mrsJSONStr(d.file) +
				", \"before\": " + mrsJSONStrOrNull(d.b) + ", \"after\": " + mrsJSONStrOrNull(d.a) + "}")
		}
		b.WriteString("]}")
	}
	b.WriteString(", \"warnings\": [], \"truncated\": false}")
	return b.String()
}

// ── 判据/闭集自述 ──────────────────────────────────────────────────────────
func mrsListRules(stdout io.Writer) int {
	fmt.Fprintf(stdout, "── %s 判据与闭集 ──\n", mrsVersion)
	fmt.Fprintf(stdout, "写词闭集（负控 · %d 词 · 形态无关：`--add` / `-a` / 裸词 `add` / `--push=origin` 全认）：\n", len(mrsWriteWords))
	fmt.Fprintln(stdout, "  "+strings.Join(mrsWriteWords, " "))
	fmt.Fprintf(stdout, "只读 git 子命令白名单（唯一入口 git_ro · 不在名单 = 拒）：%s\n", strings.Join(mrsReadonlySubs, " "))
	fmt.Fprintln(stdout, "每条 git 调用硬带：`--no-optional-locks`（没有它，`git status` 会改写 `.git/index` ⇒ 只读是假的）")
	meta := make([]string, 0, len(mrsMetaNames))
	for _, n := range mrsMetaNames {
		meta = append(meta, "<.git>/"+n)
	}
	fmt.Fprintf(stdout, "窄口径指纹面：%s + `refs/**` + porcelain 全文摘要（**不含** `logs/`/`config`：外部写入者会动，不属本命令副作用面）\n", strings.Join(meta, " "))
	fmt.Fprintln(stdout, "宽口径（`--prove-readonly=tree`）：再加 git 眼里的工作树逐件 sha256（`ls-files` + `--others --exclude-standard`）")
	fmt.Fprintf(stdout, "口径来源：默认 `zerg repo status --root <仓> --json %s`（与既有单仓命令同源）· `--source git` 直读 git\n", mrsFields)
	fmt.Fprintln(stdout, "退码：0 汇总跑通 · 1 只读自证红 · 2 拒写/用法错/缺件/自检不过")
	return 0
}

// ── 参数解析（argparse 同形）───────────────────────────────────────────────
type mrsArgs struct {
	root      string
	docsRoot  string
	zerg      string
	source    string
	prove     string
	json      bool
	listRules bool
	selfTest  bool
	help      bool
	version   bool
}

const mrsUsageLine = "usage: check-multirepo-summary.py [-h] [--root ROOT] [--docs-root DOCS_ROOT]\n" +
	"                                  [--zerg ZERG] [--source {zerg,git}]\n" +
	"                                  [--prove-readonly [{narrow,tree}]] [--json]\n" +
	"                                  [--list-rules] [--self-test] [--version]\n"

const mrsHelpText = mrsUsageLine +
	"\n" +
	"A7 仓外仓只读汇总（主仓 + 文档仓）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT           主仓仓根（默认真源：本脚本所在仓）\n" +
	"  --docs-root DOCS_ROOT\n" +
	"                        文档仓仓根（默认：主仓的兄弟目录 Zerg-内部文档/）\n" +
	"  --zerg ZERG           命令面可跑制品（口径来源①）\n" +
	"  --source {zerg,git}   读数口径来源（默认 zerg = 与单仓命令同源）\n" +
	"  --prove-readonly [{narrow,tree}]\n" +
	"                        跑前后对拍只读自证（缺省参数 = narrow）\n" +
	"  --json                机读输出（zerg/v1 形状）\n" +
	"  --list-rules\n" +
	"  --self-test\n" +
	"  --version             show program's version number and exit\n"

// mrsLongOpts —— 参与「唯一前缀」匹配的长旗标表（argparse allow_abbrev 缺省）。
var mrsLongOpts = []string{"--help", "--root", "--docs-root", "--zerg", "--source",
	"--prove-readonly", "--json", "--list-rules", "--self-test", "--version"}

func mrsMatchLong(name string) string {
	for _, o := range mrsLongOpts {
		if o == name {
			return o
		}
	}
	var cands []string
	for _, o := range mrsLongOpts {
		if strings.HasPrefix(o, name) {
			cands = append(cands, o)
		}
	}
	if len(cands) == 1 {
		return cands[0]
	}
	return ""
}

// mrsParseArgs —— 逐字复刻 argparse 的主要面（✗ 不全：见未覆盖面②）。
func mrsParseArgs(argv []string) (mrsArgs, string) {
	a := mrsArgs{root: mrsDefaultRepoRoot(), source: "zerg"}
	a.zerg = mrsPyJoin(a.root, "bin", "zerg")
	var unknown []string
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if tok == "--" {
			i++
			for ; i < len(argv); i++ {
				unknown = append(unknown, argv[i])
			}
			break
		}
		if strings.HasPrefix(tok, "--") {
			name := tok
			val := ""
			hasEq := false
			if eq := strings.Index(tok, "="); eq >= 0 {
				name = tok[:eq]
				val = tok[eq+1:]
				hasEq = true
			}
			opt := mrsMatchLong(name)
			if opt == "" {
				unknown = append(unknown, tok)
				i++
				continue
			}
			switch opt {
			case "--help":
				a.help = true
				return a, ""
			case "--version":
				a.version = true
				return a, ""
			case "--json", "--list-rules", "--self-test":
				if hasEq {
					return a, fmt.Sprintf("argument %s: ignored explicit argument %s", opt, mrsPyRepr(val))
				}
				switch opt {
				case "--json":
					a.json = true
				case "--list-rules":
					a.listRules = true
				case "--self-test":
					a.selfTest = true
				}
				i++
			case "--root", "--docs-root", "--zerg", "--source":
				if !hasEq {
					if i+1 >= len(argv) {
						return a, fmt.Sprintf("argument %s: expected one argument", opt)
					}
					i++
					val = argv[i]
				}
				switch opt {
				case "--root":
					a.root = val
				case "--docs-root":
					a.docsRoot = val
				case "--zerg":
					a.zerg = val
				case "--source":
					if val != "zerg" && val != "git" {
						return a, fmt.Sprintf("argument --source: invalid choice: %s (choose from 'zerg', 'git')", mrsPyRepr(val))
					}
					a.source = val
				}
				i++
			case "--prove-readonly":
				if hasEq {
					if val != "narrow" && val != "tree" {
						return a, fmt.Sprintf("argument --prove-readonly: invalid choice: %s (choose from 'narrow', 'tree')", mrsPyRepr(val))
					}
					a.prove = val
				} else if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
					i++
					v := argv[i]
					if v != "narrow" && v != "tree" {
						return a, fmt.Sprintf("argument --prove-readonly: invalid choice: %s (choose from 'narrow', 'tree')", mrsPyRepr(v))
					}
					a.prove = v
				} else {
					a.prove = "narrow"
				}
				i++
			}
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			if tok == "-h" {
				a.help = true
				return a, ""
			}
			unknown = append(unknown, tok)
			i++
			continue
		}
		unknown = append(unknown, tok)
		i++
	}
	if len(unknown) > 0 {
		return a, "unrecognized arguments: " + strings.Join(unknown, " ")
	}
	return a, ""
}

// mrsDefaultRepoRoot —— Go 侧缺省根：从 cwd 上溯找最近的「含 scripts/gates/ 子目录」的层。
func mrsDefaultRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := mrsPyAbspath(wd)
	for {
		if mrsIsDir(mrsPyJoin(d, "scripts", "gates")) {
			return d
		}
		parent := mrsPyDirname(d)
		if parent == d || parent == "" {
			return mrsPyAbspath(wd)
		}
		d = parent
	}
}

// ── 自检（成对负控 · 合成夹具 · 仓外临时目录）────────────────────────────
func mrsMkGitEnv(home string) []string {
	return []string{
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"HOME=" + home, "PATH=" + os.Getenv("PATH"),
	}
}

func mrsMkRepo(d, name string, files []string, dirty bool) string {
	repo := mrsPyJoin(d, name)
	_ = os.MkdirAll(repo, 0o755)
	env := mrsMkGitEnv(d)
	g := func(args ...string) {
		cmd := append([]string{"git", "-C", repo}, args...)
		_, _ = mrsRunCmd(cmd, "", env, 60*time.Second)
	}
	g("init", "-q", "-b", "main")
	for _, f := range files {
		_ = os.WriteFile(mrsPyJoin(repo, f), []byte("one\n"), 0o644)
	}
	g(append([]string{"add", "--"}, files...)...)
	g("commit", "-qm", "init")
	if dirty {
		fa := mrsPyJoin(repo, "a.txt")
		if fh, err := os.OpenFile(fa, os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			_, _ = fh.WriteString("two\n")
			fh.Close()
		}
		_ = os.WriteFile(mrsPyJoin(repo, "新件.txt"), []byte("new\n"), 0o644)
	}
	return repo
}

// checkMultirepoSummaryGateMain —— py `main()` 同形入口（argv 与 py `sys.argv[1:]` 对齐）。
func checkMultirepoSummaryGateMain(stdout, stderr io.Writer, argv []string) int {
	return mrsMain(stdout, stderr, argv)
}

func mrsMain(stdout, stderr io.Writer, argv []string) int {
	hits := mrsParseWriteFlags(argv)
	if len(hits) > 0 {
		fmt.Fprintf(stdout, "⇒ 拒（rc=2 · 不给结论）：命令行里出现写旗标 %s\n", mrsPyListRepr(hits))
		fmt.Fprintln(stdout, "   理由（`A7` 条件② 逐字）：这条命令长在**仓外仓**上（`Zerg-内部文档/` 是并排仓，")
		fmt.Fprintln(stdout, "   不在主仓仓根下），误写的代价与 `G-06` 同族 ⇒ 任何写旗标（add/commit/push…）一律拒，")
		fmt.Fprintln(stdout, "   且**绝不代跑任何 git 写子命令**（第二道牙 = 唯一入口 `git_ro()` 的子命令白名单）。")
		fmt.Fprintf(stdout, "   只读用法：bash scripts/gates/%s [--root <主仓>] [--docs-root <文档仓>] [--json] [--prove-readonly]\n", mrsSelfName)
		return 2
	}

	a, perr := mrsParseArgs(argv)
	if perr != "" {
		fmt.Fprint(stderr, mrsUsageLine)
		fmt.Fprintf(stderr, "%s: error: %s\n", mrsProg, perr)
		return 2
	}
	if a.help {
		fmt.Fprint(stdout, mrsHelpText)
		return 0
	}
	if a.version {
		fmt.Fprintln(stdout, mrsVersion)
		return 0
	}
	if a.selfTest {
		return mrsSelfTest(stdout, stderr)
	}
	if a.listRules {
		return mrsListRules(stdout)
	}
	docsRoot := mrsDiscoverDocsRoot(a.root, a.docsRoot)
	return mrsRun(stdout, a.root, docsRoot, a.source, a.prove, a.json, a.zerg)
}

// ── 自检主体 ───────────────────────────────────────────────────────────────
const mrsWantCases = 20 // 自证用例条数（写死：少跑一格即断言失败）

func mrsSelfTest(stdout, stderr io.Writer) int {
	d, err := os.MkdirTemp("", "mrs-selftest-")
	if err != nil {
		fmt.Fprintln(stderr, "自检：建临时目录失败："+err.Error())
		return 2
	}
	defer os.RemoveAll(d) // ★ 自回收（A 类：delta=0；py 原件不回收 ⇒ 声明差）

	ok := true
	total := 0
	caseFn := func(name string, want, got any, extraOK bool, note string) {
		good := mrsPyEq(want, got) && extraOK
		ok = ok && good
		total++
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（rc=%s · 期望 %s）%s\n", mark, name, mrsPyStr(got), mrsPyStr(want), note)
	}
	cap := func(fn func(io.Writer) int) (int, string) {
		var buf bytes.Buffer
		rc := fn(&buf)
		return rc, buf.String()
	}

	mainRepo := mrsMkRepo(d, "主仓", []string{"a.txt"}, true)
	docs := mrsMkRepo(d, "Zerg-内部文档", []string{"x.md"}, false)

	// ① 正例
	rc, out := cap(func(w io.Writer) int { return mrsRun(w, mainRepo, docs, "git", "", false, "") })
	caseFn("① 正例：两仓 @HEAD + 脏件数（脏 1 · 未跟踪 1）", 0, rc,
		strings.Contains(out, "脏件数") && strings.Contains(out, "主仓") && strings.Contains(out, "文档仓"), "")

	// ①b JSON 形状
	rc, out = cap(func(w io.Writer) int { return mrsRun(w, mainRepo, docs, "git", "", true, "") })
	schema, count, dirty := "", 0, -1
	var jd map[string]any
	if json.Unmarshal([]byte(out), &jd) == nil {
		schema, _ = jd["schema"].(string)
		if m, ok := jd["meta"].(map[string]any); ok {
			if v, ok := m["count"].(float64); ok {
				count = int(v)
			}
			if v, ok := m["dirty"].(float64); ok {
				dirty = int(v)
			}
		}
	}
	caseFn("①b `--json` 形状（zerg/v1 · items 2 条 · 计数现算）", 0, rc,
		schema == "zerg/v1" && count == 2 && dirty == 1, "")

	// ② 负控：写旗标一律拒 rc=2（走真 CLI 入口）
	headBefore, _ := mrsGitRo(mainRepo, []string{"rev-parse", "HEAD"}, 60*time.Second)
	headBefore = mrsPyStrip(headBefore)
	gd, _ := mrsGitDir(mainRepo)
	idxBefore, _ := mrsFileSha(mrsPyJoin(gd, "index"))
	for _, flag := range []string{"--commit", "--add", "--push", "commit", "push", "add", "--push=origin"} {
		rc, out = cap(func(w io.Writer) int { return mrsMain(w, io.Discard, []string{"--root", mainRepo, flag}) })
		caseFn(fmt.Sprintf("② 负控：写旗标 %s ⇒ 拒 rc=2", mrsPyRepr(flag)), 2, rc, strings.Contains(out, "拒"), "")
	}
	headAfter, _ := mrsGitRo(mainRepo, []string{"rev-parse", "HEAD"}, 60*time.Second)
	headAfter = mrsPyStrip(headAfter)
	idxAfter, _ := mrsFileSha(mrsPyJoin(gd, "index"))
	caseFn("②b 拒写后夹具 HEAD/index 逐字节不变（真拒）", headBefore, headAfter, idxBefore == idxAfter,
		fmt.Sprintf("  head %s index %s", mrsPyRunes(headBefore, 8), mrsPyRunes(idxBefore, 8)))

	// ④ 内部牙
	refused := false
	if _, e := mrsGitRo(mainRepo, []string{"commit", "-m", "x"}, 60*time.Second); e != nil {
		var rf mrsRefused
		refused = errors.As(e, &rf)
	}
	caseFn("④ 内部牙：git_ro(['commit', …]) 抛 Refused（连拼命令都到不了）", true, refused, true, "")

	// ③ 只读自证（窄口径）
	rc, out = cap(func(w io.Writer) int { return mrsRun(w, mainRepo, docs, "git", "narrow", false, "") })
	caseFn("③ 只读自证（窄口径）：差异 0 件 ⇒ rc=0", 0, rc, strings.Contains(out, "零副作用"), "")
	rc, out = cap(func(w io.Writer) int { return mrsRun(w, mainRepo, docs, "git", "tree", false, "") })
	caseFn("③b 只读自证（宽口径 · 全工作树逐件 sha256）：差异 0 件 ⇒ rc=0", 0, rc, strings.Contains(out, "零副作用"), "")

	// ⑦ 负控（白盒 · 检测器级）
	_ = mrsRepoFingerprint(mainRepo, true)
	appendX(docs)
	after := mrsRepoFingerprint(docs, true)
	d2 := mrsDiffFingerprint(mrsRepoFingerprint(docs, true), after)
	dInwinBefore := mrsRepoFingerprint(docs, true)
	appendX(docs)
	dInwin := mrsDiffFingerprint(dInwinBefore, mrsRepoFingerprint(docs, true))
	first := "—"
	if len(dInwin) > 0 {
		first = dInwin[0].k
	}
	caseFn("⑦ 负控（白盒）：窗口内真被改 ⇒ 检测器逐件抓到（差异非 0）", false, len(dInwin) == 0, true,
		fmt.Sprintf("  差异 %d 件 · 首件 %s", len(dInwin), first))
	got7b := 0
	if len(dInwin) > 0 {
		got7b = 1
	}
	caseFn("⑦b 负控（白盒）：同一差异落判红规则 ⇒ rc=1（不是 0）", 1, got7b, true, "")
	caseFn("⑦c 正例（白盒）：同件前后各取一次 ⇒ 差异 0 件（检测器不会乱报）", true, len(d2) == 0, true, "")

	// ⑦d `--no-optional-locks` 这一格不是摆设
	p3 := mrsMkRepo(d, "probe-clean", []string{"a.txt"}, false)
	gd3, _ := mrsGitDir(p3)
	idx := mrsPyJoin(gd3, "index")
	pre, _ := mrsFileSha(idx)
	_ = os.Chtimes(mrsPyJoin(p3, "a.txt"), time.Date(2001, 1, 1, 0, 0, 0, 0, time.Local), time.Date(2001, 1, 1, 0, 0, 0, 0, time.Local))
	_, _, _ = mrsReadGit(p3, []string{"git", "-C", p3, "status", "--porcelain"})
	post, _ := mrsFileSha(idx)
	caseFn("⑦d 负控：不带 `--no-optional-locks` 的 `git status` ⇒ `.git/index` 真变（这一格有牙）",
		false, pre == post, true, fmt.Sprintf("  %s ⇒ %s", mrsPyRunes(pre, 8), mrsPyRunes(post, 8)))
	pre2, _ := mrsFileSha(idx)
	_ = os.Chtimes(mrsPyJoin(p3, "a.txt"), time.Date(2001, 1, 1, 0, 0, 0, 0, time.Local), time.Date(2001, 1, 1, 0, 0, 0, 0, time.Local))
	_, _ = mrsGitRo(p3, []string{"status", "--porcelain"}, 60*time.Second)
	post2, _ := mrsFileSha(idx)
	caseFn("⑦e 正例：本件真路径（带 `--no-optional-locks`）⇒ `.git/index` 不变", true, pre2 == post2, true, "")

	// ⑤ 缺件
	rc, out = cap(func(w io.Writer) int { return mrsRun(w, mrsPyJoin(d, "没有这个仓"), docs, "git", "", false, "") })
	caseFn("⑤ 缺件（--root 不是仓）⇒ rc=2 不给结论", 2, rc, strings.Contains(out, "不是 git 仓"), "")

	// ⑥ 写词闭集非空 + 无重复
	dup := false
	seen := map[string]bool{}
	for _, w := range mrsWriteWords {
		if seen[w] {
			dup = true
		}
		seen[w] = true
	}
	caseFn("⑥ 写词闭集非空且无重复", true, len(mrsWriteWords) > 0 && !dup, true, "")

	if total != mrsWantCases {
		fmt.Fprintf(stderr, "自检：用例条数 %d ≠ 期望 %d\n", total, mrsWantCases)
		return 2
	}
	verdict := "自检结论: 全过"
	if !ok {
		verdict = "自检结论: **有断言失败**"
	}
	fmt.Fprintln(stdout, verdict)
	if ok {
		return 0
	}
	return 1
}

func appendX(docs string) {
	fh, err := os.OpenFile(mrsPyJoin(docs, "x.md"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = fh.WriteString("窗口内被改一个字节\n")
	fh.Close()
}
