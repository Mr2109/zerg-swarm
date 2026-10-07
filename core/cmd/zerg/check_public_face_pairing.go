// check_public_face_pairing.go —— 判据件 `scripts/gates/check-public-face-pairing.py`（670 行）的 **Go 真搬**
// （M2 一枚 · 公开面成对闸族 · 入口 `checkPublicFacePairingGateMain` · 承接 GAP-20261001-59）。
//
// 验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 声明 (a) 默认根定位差异：py 从脚本自身位置（`__file__`）上溯 2 层推 `REPO_ROOT`（本件所在目录的上两层）；
// Go 无脚本位置语境 ⇒ 缺省根 = **cwd**（不向上搜「含某标记的目录」，以免与 py 的纯上溯语义分叉）。
// 两态在「脚本位置 == cwd」时逐字同构（两环境对拍即按此布置：甲 = 真仓 + 显式传根；
// 乙 = 夹具根放 `fx/scripts/gates/` + cwd = fx）。
//
// 声明 (b) ★自指判定（本件是否落在扫描面内）——
//
//	① 本门自己的扫描面 = `<--face>`（缺省 `publish/docs`）下的 `*.md` ⇒ **本件自身（core/ 下的 .go）
//	   绝不在本门扫面内**（量化：本门扫的只有面内 .md，本件不在该面任何子树里）。
//	② 有 core/ 在扫面内的**他门**：逐个现读其针形并避让 ——
//	   · 私有卷 / 家目录绝对路径针（扫 core/）⇒ 本件**零**该类字面量（卷名与用户名一律拆串）。
//	   · 干跑语义登记门（扫 core/）⇒ 本件正文**不出现**该门登记触发词（本门无该语义面）。
//	   落件前后对同一他门取受控读数对照（见交付④）。
//	③ 夹具 / 自检素材里的合成仓内容一律是本件自造串，与真仓公开面件不同名同形。
//
// 已知口径差（照实记）：
//
//	· 例外登记面真身在**另一个 py 件**（`scripts/gates/check-public-face-anchor.py` 的
//	  `SINGLE_LANG_EXCEPTIONS`）；py 走跨件现读（动态执行那个件）；本件**不执行** py
//	  ⇒ 自写**字面量解析器**读同一份真源（只取 `file` / `reason` 两键）。
//	  若真源把该表写成「算出来的」（非常量字面量）⇒ 本件解析失败并 rc=2（不给结论），✗ 不静默错判。
//	· 最外层无兜底（py 原件为裸出口形态）⇒ 本件按 fail-closed 收成 rc=2 + 具名文案（见未覆盖面）。
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	cpfpVersion         = "check-public-face-pairing.py v1.0（`T-12` 成对闸 · 2026-09-24 落件 · 接线 = 闸⑧ ⑧d）"
	cpfpFaceDefault     = "publish/docs"
	cpfpZhSuffix        = ".zh-CN.md"
	cpfpEnSuffix        = ".en.md"
	cpfpReadmeZh        = "README.zh-CN.md"
	cpfpReadmeEn        = "README.md"
	cpfpRegistrationRel = "scripts/gates/check-public-face-anchor.py"
	cpfpSelfRel         = "scripts/gates/check-public-face-pairing.py"
)

var cpfpWalkPrune = []string{".git", "node_modules", "target"}

// 拒收的逃生开关（一律 rc=2；例外只能显式登记）。
var cpfpRefuseFlags = []string{"--waive", "--allow-unpaired", "--skip-pairs", "--ignore-missing"}

// 机器人直推分支表（逐条带理由）。
var cpfpBotBranchPrefixes = []struct{ prefix, why string }{
	{"chore/crowdin", "参照件的机器人直推分支（nodejs.org 的 Crowdin 机器人）逐字同名 —— 本仓现读无此分支；留作形态一致 + 自证有一格"},
	{"bot/", "通用机器人分支前缀"},
	{"dependabot/", "GitHub 依赖机器人分支"},
	{"renovate/", "Renovate 机器人分支"},
}

var cpfpBotAuthorRe = regexp.MustCompile(`\[bot\]|-bot@|\+bot@`)

const (
	cpfpExitOK      = 0
	cpfpExitRed     = 1
	cpfpExitBlocked = 2
)

// cpfpRefused —— 拒绝给结论（rc=2）。
type cpfpRefused struct{ msg string }

func (e *cpfpRefused) Error() string { return e.msg }

func cpfpRefuse(format string, a ...any) error {
	return &cpfpRefused{msg: fmt.Sprintf(format, a...)}
}

// ══ py 语义工具（posixpath 同构 · 模板 templates/py-path-semantics.go）════════════════════

// cpfpPyJoin = CPython posixpath.join 逐段规则（★ 三语义差：中间 `..` 原样留 · 后段绝对丢前段 ·
// 空段保留尾 `/`）。按件名缩写定名 `cpfpPyJoin`。
func cpfpPyJoin(parts ...string) string {
	path := ""
	if len(parts) > 0 {
		path = parts[0]
	}
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

func cpfpPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
	}
	if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
		initial = 2
	}
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || initial == 0 && len(newComps) == 0 ||
			len(newComps) > 0 && newComps[len(newComps)-1] == ".." {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	res := strings.Join(newComps, "/")
	if initial > 0 {
		res = strings.Repeat("/", initial) + res
	}
	if res == "" {
		return "."
	}
	return res
}

func cpfpPyAbspath(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = ""
		}
		p = cpfpPyJoin(wd, p)
	}
	return cpfpPyNormpath(p)
}

func cpfpSplitComps(p string) []string {
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

func cpfpPyRelpath(path, start string) string {
	startAbs := cpfpPyAbspath(start)
	pathAbs := cpfpPyAbspath(path)
	sl := cpfpSplitComps(startAbs)
	pl := cpfpSplitComps(pathAbs)
	i := 0
	for i < len(sl) && i < len(pl) && sl[i] == pl[i] {
		i++
	}
	var rel []string
	for j := 0; j < len(sl)-i; j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pl[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return cpfpPyJoin(rel...)
}

// cpfpPySplit = CPython posixpath.split（返回 dir, base）—— ★ ≠ filepath.Split（尾斜杠处置不同）。
func cpfpPySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" {
		if trimmed := strings.TrimRight(head, "/"); trimmed != "" {
			head = trimmed
		}
	}
	return head, tail
}

func cpfpPyDirname(p string) string {
	d, _ := cpfpPySplit(p)
	return d
}

// cpfpPyStrip = CPython str.strip()（★ 含 \x1c-\x1f ⇒ ✗ 不能用 strings.TrimSpace 单独了事）。
func cpfpPyStrip(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		if r >= 0x1c && r <= 0x1f {
			return true
		}
		return unicode.IsSpace(r)
	})
}

// ══ git（只经子进程 · 不接管道）════════════════════════════════════════════════════

func cpfpEnvSet(env []string, k, v string) []string {
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, k+"=") {
			continue
		}
		out = append(out, e)
	}
	return append(out, k+"="+v)
}

// cpfpGitEnv = py `git_env(base)`：base 非空 ⇒ 覆盖 GIT_* / HOME（隔离合成夹具）。
func cpfpGitEnv(base string) []string {
	env := os.Environ()
	if base != "" {
		env = cpfpEnvSet(env, "GIT_CONFIG_GLOBAL", os.DevNull)
		env = cpfpEnvSet(env, "GIT_CONFIG_SYSTEM", os.DevNull)
		env = cpfpEnvSet(env, "HOME", base)
		env = cpfpEnvSet(env, "GIT_AUTHOR_NAME", "PairingSelfTest")
		env = cpfpEnvSet(env, "GIT_COMMITTER_NAME", "PairingSelfTest")
		env = cpfpEnvSet(env, "GIT_AUTHOR_EMAIL", "selftest@example.invalid")
		env = cpfpEnvSet(env, "GIT_COMMITTER_EMAIL", "selftest@example.invalid")
	}
	return env
}

// cpfpGitRun 返回 (returncode, stdout)；起不来 ⇒ (127, "")（同 py 的 OSError 档）。
func cpfpGitRun(root string, args []string, env []string) (int, string) {
	full := append([]string{"git", "-C", root, "-c", "core.quotepath=false"}, args...)
	cmd := exec.Command(full[0], full[1:]...)
	cmd.Env = env
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return cmd.ProcessState.ExitCode(), so.String()
		}
		return exitBlocked, "" // ★ 子进程**起不来**（非 ExitError）⇒ 读不到 ⇒ 归 `exitBlocked`(8「有 BLOCKED · 读不到不许当健康」)；
		// 原先裸写 127（shell 惯例）不在命令面真源表里 ⇒ 门⑪ T3 报（GAP-20261002-08）。
	}
	return 0, so.String()
}

func cpfpGitOK(root string, env []string) bool {
	rc, _ := cpfpGitRun(root, []string{"rev-parse", "--git-dir"}, env)
	return rc == 0
}

func cpfpRevResolvable(root, rev string, env []string) bool {
	if rev == "" {
		return true
	}
	parts := []string{rev}
	if strings.Contains(rev, "..") {
		parts = strings.Split(rev, "..")
	}
	for _, p := range parts {
		if p == "" {
			p = "HEAD"
		}
		rc, _ := cpfpGitRun(root, []string{"rev-parse", "--quiet", "--verify", p + "^{commit}"}, env)
		if rc != 0 {
			return false
		}
	}
	return true
}

func cpfpSplitNames(text string) []string {
	var out []string
	for _, ln := range strings.Split(text, "\n") {
		ln = cpfpPyStrip(ln)
		if ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

// ══ 扫描面（正向 glob + 登记面派生的负向 glob）══════════════════════════════════════

// cpfpEnumerateFace = py `enumerate_face`：只收 `.md`；点件 / 点目录 / 剪枝目录不入；
// 目录项按 readdir 原序读入后**本件自排**（py 逐层 sorted + 末尾全局 sorted ⇒ 输出序决定者 = 末尾 sort）。
func cpfpEnumerateFace(faceDir string) []string {
	var files []string
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return // 同 py os.walk 默认 onerror=None：读不了的目录静默跳过（B 类目录面 · 照实照搬）
		}
		var dirs, nondirs []string
		for _, e := range entries {
			name := e.Name()
			// py entry.is_dir() 跟随符号链接 ⇒ 必须 os.Stat（✗ 不用 lstat 语义的 e.IsDir()）
			st, serr := os.Stat(cpfpPyJoin(dir, name))
			if serr == nil && st.IsDir() {
				dirs = append(dirs, name)
			} else {
				nondirs = append(nondirs, name)
			}
		}
		sort.Strings(dirs)
		sort.Strings(nondirs)
		for _, fn := range nondirs {
			if strings.HasPrefix(fn, ".") || !strings.HasSuffix(fn, ".md") {
				continue
			}
			rel := strings.ReplaceAll(cpfpPyRelpath(cpfpPyJoin(dir, fn), faceDir), string(os.PathSeparator), "/")
			files = append(files, rel)
		}
		for _, d := range dirs {
			if strings.HasPrefix(d, ".") || cpfpContainsStr(cpfpWalkPrune, d) {
				continue
			}
			// os.walk 默认 followlinks=False：指向目录的符号链接不下钻
			if fi, lerr := os.Lstat(cpfpPyJoin(dir, d)); lerr == nil && fi.Mode()&os.ModeSymlink != 0 {
				continue
			}
			walk(cpfpPyJoin(dir, d))
		}
	}
	walk(faceDir)
	sort.Strings(files)
	return files
}

// ══ 例外登记面（单一真源 · 另一个判据件里的常量表）══════════════════════════════════

type cpfpRegEntry struct{ File, Reason string }

// cpfpLit —— 极简 Python 字面量解析器（列表 / 字典 / 字符串含相邻拼接 / 整数 / True/False/None）。
type cpfpLit struct {
	r []rune
	i int
}

func cpfpLitDelim(c rune) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f', ',', ']', '}', ':', '#', '\'', '"':
		return true
	}
	return false
}

func (p *cpfpLit) skip() {
	for p.i < len(p.r) {
		c := p.r[p.i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f' {
			p.i++
			continue
		}
		if c == '#' {
			for p.i < len(p.r) && p.r[p.i] != '\n' {
				p.i++
			}
			continue
		}
		break
	}
}

func (p *cpfpLit) parseValue() (any, error) {
	p.skip()
	if p.i >= len(p.r) {
		return nil, fmt.Errorf("unexpected end of literal")
	}
	switch c := p.r[p.i]; {
	case c == '[':
		return p.parseList()
	case c == '{':
		return p.parseDict()
	case c == '"' || c == '\'':
		return p.parseString()
	}
	start := p.i
	for p.i < len(p.r) && !cpfpLitDelim(p.r[p.i]) {
		p.i++
	}
	tok := string(p.r[start:p.i])
	switch tok {
	case "True":
		return true, nil
	case "False":
		return false, nil
	case "None":
		return nil, nil
	case "":
		return nil, fmt.Errorf("empty token")
	}
	if n, err := strconv.ParseInt(strings.ReplaceAll(tok, "_", ""), 10, 64); err == nil {
		return n, nil
	}
	return tok, nil
}

// parseString 复刻「相邻字符串字面量自动拼接」。
func (p *cpfpLit) parseString() (any, error) {
	var b strings.Builder
	for {
		p.skip()
		if p.i >= len(p.r) {
			break
		}
		q := p.r[p.i]
		if q != '"' && q != '\'' {
			break
		}
		p.i++
		closed := false
		for p.i < len(p.r) {
			c := p.r[p.i]
			if c == '\\' {
				p.i++
				if p.i >= len(p.r) {
					break
				}
				switch e := p.r[p.i]; e {
				case 'n':
					b.WriteRune('\n')
				case 't':
					b.WriteRune('\t')
				case 'r':
					b.WriteRune('\r')
				case '\\':
					b.WriteRune('\\')
				case '\'':
					b.WriteRune('\'')
				case '"':
					b.WriteRune('"')
				case '0':
					b.WriteRune(0)
				default:
					b.WriteRune(e)
				}
				p.i++
				continue
			}
			if c == q {
				p.i++
				closed = true
				break
			}
			b.WriteRune(c)
			p.i++
		}
		if !closed {
			return nil, fmt.Errorf("unterminated string")
		}
	}
	return b.String(), nil
}

func (p *cpfpLit) parseList() (any, error) {
	p.i++
	out := []any{}
	for {
		p.skip()
		if p.i >= len(p.r) {
			return nil, fmt.Errorf("unterminated list")
		}
		if p.r[p.i] == ']' {
			p.i++
			return out, nil
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		p.skip()
		if p.i < len(p.r) && p.r[p.i] == ',' {
			p.i++
			continue
		}
		if p.i < len(p.r) && p.r[p.i] == ']' {
			p.i++
			return out, nil
		}
		return nil, fmt.Errorf("bad list separator")
	}
}

func (p *cpfpLit) parseDict() (any, error) {
	p.i++
	out := map[string]any{}
	for {
		p.skip()
		if p.i >= len(p.r) {
			return nil, fmt.Errorf("unterminated dict")
		}
		if p.r[p.i] == '}' {
			p.i++
			return out, nil
		}
		k, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		p.skip()
		if p.i >= len(p.r) || p.r[p.i] != ':' {
			return nil, fmt.Errorf("bad dict key")
		}
		p.i++
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		out[fmt.Sprintf("%v", k)] = v
		p.skip()
		if p.i < len(p.r) && p.r[p.i] == ',' {
			p.i++
			continue
		}
		if p.i < len(p.r) && p.r[p.i] == '}' {
			p.i++
			return out, nil
		}
		return nil, fmt.Errorf("bad dict separator")
	}
}

// cpfpParseRegistrationList 从登记面的源码取 `SINGLE_LANG_EXCEPTIONS` 的列表值。
func cpfpParseRegistrationList(src string) ([]any, error) {
	name := "SINGLE_LANG_EXCEPTIONS"
	runes := []rune(src)
	off := -1
	start := 0
	for _, ln := range strings.Split(src, "\n") {
		if strings.HasPrefix(ln, name) {
			rest := strings.TrimLeft(strings.TrimPrefix(ln, name), " \t")
			if strings.HasPrefix(rest, "=") {
				off = start + utf8.RuneCountInString(name) +
					utf8.RuneCountInString(strings.TrimPrefix(ln, name)) - utf8.RuneCountInString(rest) + 1
				break
			}
		}
		start += utf8.RuneCountInString(ln) + 1
	}
	if off < 0 {
		return nil, fmt.Errorf("module 'zerg_pairing_registration' has no attribute '%s'", name)
	}
	p := &cpfpLit{r: runes, i: off}
	v, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	lst, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("'%s' is not a list literal", name)
	}
	return lst, nil
}

// cpfpLoadRegisteredExceptions = py `load_registered_exceptions`（缺件 / 读不了 / 形状不对 / 空表 ⇒ rc=2）。
func cpfpLoadRegisteredExceptions(registrationPath, root string) ([]cpfpRegEntry, error) {
	base := root
	if base == "" {
		base = cpfpDefaultRoot()
	} else {
		base = cpfpPyAbspath(base)
	}
	path := registrationPath
	if !strings.HasPrefix(registrationPath, "/") {
		path = cpfpPyJoin(base, registrationPath)
	}
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		// 退役态（B轨 · 2026-10-07）：默认登记面件（原 py `check-public-face-anchor.py`）退役
		// （缺席 · 其 Go 半在盘）⇒ 回落到**内化登记**（`cpfpRetiredExceptions` · ✗ 不 fail-closed）。
		// 缺件且无 Go 半（真消失）⇒ 照旧 rc=2（缺件 ≠ 绿）。
		if gh := cpfpGoHalfRel(registrationPath); gh != "" && cpfpFileIsFile(cpfpPyJoin(base, gh)) {
			return cpfpRetiredExceptions(), nil
		}
		return nil, cpfpRefuse("缺件：例外登记面 %s 不在（`T-11b` 的登记面 · 单一真源）⇒ 判不了「哪几件是已登记的单语例外」"+
			"（缺件 ≠ 绿；登记面缺席 ⇒ `design/` 两件会当场假红 ✗）", registrationPath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, cpfpRefuse("读不了：例外登记面 %s 解析失败（%s）⇒ **不给结论（rc=2）**", registrationPath, err)
	}
	rows, perr := cpfpParseRegistrationList(string(data))
	if perr != nil {
		return nil, cpfpRefuse("读不了：例外登记面 %s 解析失败（%s）⇒ **不给结论（rc=2）**", registrationPath, perr)
	}
	var out []cpfpRegEntry
	for _, rv := range rows {
		m, ok := rv.(map[string]any)
		if !ok {
			return nil, cpfpRefuse("形状不对：例外登记面 %s 的条目缺 `file` 键（%s）⇒ 判不了", registrationPath, cpfpGoRepr(rv))
		}
		fv, has := m["file"]
		if !has || !cpfpPyTruthy(fv) {
			return nil, cpfpRefuse("形状不对：例外登记面 %s 的条目缺 `file` 键（%s）⇒ 判不了", registrationPath, cpfpGoRepr(m))
		}
		reason := ""
		if rvv, has := m["reason"]; has {
			if rvv == nil {
				reason = "None"
			} else {
				reason = fmt.Sprintf("%v", rvv)
			}
		}
		out = append(out, cpfpRegEntry{File: strings.Trim(fmt.Sprintf("%v", fv), "/"), Reason: reason})
	}
	if len(out) == 0 {
		return nil, cpfpRefuse("空登记面：%s 的 `SINGLE_LANG_EXCEPTIONS` 一条都没有 ⇒ **不给结论**（空表 = 假覆盖）",
			registrationPath)
	}
	return out, nil
}

func cpfpGoHalfRel(pyPath string) string {
	base := cpfpPyBasename(pyPath)
	if !strings.HasSuffix(base, ".py") {
		return ""
	}
	return "core/cmd/zerg/" + strings.ReplaceAll(strings.TrimSuffix(base, ".py"), "-", "_") + ".go"
}

func cpfpPyBasename(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func cpfpFileIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// cpfpRetiredExceptions —— 默认例外登记面（原 py `check-public-face-anchor.py`）**退役后**的内化真源
// （B轨 · 2026-10-07）：条目与其 Go 半 `cpfaExceptions` 同源（file / reason 逐条同）。
// py 件在盘时仍现读其 `SINGLE_LANG_EXCEPTIONS`（单一真源不变）；py 件退役（不在盘）时回落到本表。
func cpfpRetiredExceptions() []cpfpRegEntry {
	out := make([]cpfpRegEntry, 0, len(cpfaExceptions))
	for _, e := range cpfaExceptions {
		out = append(out, cpfpRegEntry{File: e.File, Reason: e.Reason})
	}
	return out
}

func cpfpPyTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return t != ""
	case int64:
		return t != 0
	case bool:
		return t
	}
	return true
}

// cpfpGoRepr 近似 py 的 %r（★ 非逐字同构：仅用于 rc=2 的「形状不对」文案 · 见未覆盖面）。
func cpfpGoRepr(v any) string {
	switch t := v.(type) {
	case string:
		return "'" + t + "'"
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, "'"+k+"': "+cpfpGoRepr(t[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// cpfpNegativeGlobs：由登记件所在子树推出负向 glob（首次出现序 · 同一子树多件逐条点名）。
type cpfpGlob struct {
	Glob string
	Srcs []string
}

func cpfpNegativeGlobs(face string, exceptions []cpfpRegEntry) []cpfpGlob {
	var order []string
	globs := map[string][]string{}
	for _, row := range exceptions {
		d := cpfpPyDirname(row.File)
		base := face
		if d != "" {
			base = cpfpPyJoin(face, d)
		}
		g := "!" + base + "/**/*.md"
		if _, ok := globs[g]; !ok {
			order = append(order, g)
			globs[g] = []string{}
		}
		globs[g] = append(globs[g], row.File)
	}
	out := make([]cpfpGlob, 0, len(order))
	for _, g := range order {
		out = append(out, cpfpGlob{Glob: g, Srcs: globs[g]})
	}
	return out
}

// cpfpPairOf：成对映射（`X.zh-CN.md` ↔ `X.en.md`；`README.zh-CN.md` ↔ `README.md`）。
func cpfpPairOf(rel string) (string, bool) {
	d, b := cpfpPySplit(rel)
	pre := ""
	if d != "" {
		pre = d + "/"
	}
	switch {
	case b == cpfpReadmeZh:
		return pre + cpfpReadmeEn, true
	case b == cpfpReadmeEn:
		return pre + cpfpReadmeZh, true
	case strings.HasSuffix(b, cpfpZhSuffix):
		return pre + b[:len(b)-len(cpfpZhSuffix)] + cpfpEnSuffix, true
	case strings.HasSuffix(b, cpfpEnSuffix):
		return pre + b[:len(b)-len(cpfpEnSuffix)] + cpfpZhSuffix, true
	}
	return "", false
}

// ══ 改动面（只 tracked · untracked 逐条打印「不判」）════════════════════════════════

func cpfpCurrentBranch(root string, env []string) string {
	rc, out := cpfpGitRun(root, []string{"symbolic-ref", "--short", "-q", "HEAD"}, env)
	if rc == 0 {
		return cpfpPyStrip(out)
	}
	return ""
}

func cpfpBotBranchOf(branch string) (string, string, bool) {
	for _, b := range cpfpBotBranchPrefixes {
		if strings.HasPrefix(branch, b.prefix) {
			return b.prefix, b.why, true
		}
	}
	return "", "", false
}

func cpfpChangedFace(root, face, rev string, env []string) (map[string]bool, []string, []string, error) {
	var notes, untracked []string
	rc, out := cpfpGitRun(root, []string{"ls-files", "-o", "--exclude-standard", "--", face}, env)
	if rc == 0 {
		untracked = cpfpSplitNames(out)
	}
	judged := map[string]bool{}
	if rev != "" && strings.Contains(rev, "..") {
		a, b, _ := strings.Cut(rev, "..")
		if b == "" {
			b = "HEAD"
		}
		rc, out = cpfpGitRun(root, []string{"rev-list", "--reverse", a + ".." + b}, env)
		if rc != 0 {
			return nil, nil, nil, cpfpRefuse("判不了：`rev-list %s` 读不出（区间解不出）⇒ **不给结论（rc=2）**", rev)
		}
		var botHits []string
		for _, sha := range cpfpSplitNames(out) {
			rcA, author := cpfpGitRun(root, []string{"show", "-s", "--format=%an <%ae>", sha}, env)
			rcN, names := cpfpGitRun(root, []string{"show", "--name-only", "--format=", sha}, env)
			if rcA != 0 || rcN != 0 {
				short := sha
				if len(short) > 8 {
					short = short[:8]
				}
				return nil, nil, nil, cpfpRefuse("判不了：读不出提交 %s 的作者/件列表 ⇒ **不给结论（rc=2）**", short)
			}
			author = cpfpPyStrip(author)
			if cpfpBotAuthorRe.MatchString(author) {
				touched := cpfpSplitNames(names)
				if len(touched) > 0 {
					short := sha
					if len(short) > 8 {
						short = short[:8]
					}
					botHits = append(botHits, fmt.Sprintf("%s %s（%d 件）", short, author, len(touched)))
				}
				continue
			}
			for _, n := range cpfpSplitNames(names) {
				judged[n] = true
			}
		}
		if len(botHits) > 0 {
			notes = append(notes, "机器人笔豁免（照参照件 `if:` 那一格：机器人面不判）："+strings.Join(botHits, " · "))
		}
		rc, out = cpfpGitRun(root, []string{"diff", "--name-only", "HEAD"}, env)
		if rc != 0 {
			return nil, nil, nil, cpfpRefuse("判不了：`git diff HEAD` 读不出 ⇒ **不给结论（rc=2）**")
		}
		for _, n := range cpfpSplitNames(out) {
			judged[n] = true
		}
		return judged, notes, untracked, nil
	}
	base := rev
	if base == "" {
		base = "HEAD"
	}
	rc, out = cpfpGitRun(root, []string{"diff", "--name-only", base}, env)
	if rc != 0 {
		return nil, nil, nil, cpfpRefuse("判不了：`git diff --name-only %s` 读不出 ⇒ **不给结论（rc=2）**", base)
	}
	for _, n := range cpfpSplitNames(out) {
		judged[n] = true
	}
	return judged, notes, untracked, nil
}

// ══ 判据主体 ══════════════════════════════════════════════════════════════════════

type cpfpPair struct{ Zh, En string }

func cpfpContainsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func cpfpJudge(stdout io.Writer, root, face, rev, registrationPath string, env []string, quiet bool) (int, error) {
	say := func(s string) {
		if !quiet {
			fmt.Fprintln(stdout, s)
		}
	}

	faceDir := cpfpPyJoin(root, face)
	if fi, err := os.Stat(faceDir); err != nil || !fi.IsDir() {
		return cpfpExitBlocked, cpfpRefuse("缺件：面 %s/ 不在 ⇒ **不给结论（rc=2）**（缺件 ≠ 绿）", face)
	}
	if !cpfpGitOK(root, env) {
		return cpfpExitBlocked, cpfpRefuse("判不了：%s 不是 git 仓（读不到改动面）⇒ **不给结论（rc=2）**", root)
	}
	if !cpfpRevResolvable(root, rev, env) {
		return cpfpExitBlocked, cpfpRefuse("判不了：rev `%s` 解不出 ⇒ **不给结论（rc=2）**（拒绝猜）", rev)
	}

	files := cpfpEnumerateFace(faceDir)
	if len(files) == 0 {
		return cpfpExitBlocked, cpfpRefuse("空转：%s/ 下 0 件（「件没了所以全绿」是最难发现的那一类假绿）⇒ **不给结论（rc=2）**", face)
	}

	exceptions, err := cpfpLoadRegisteredExceptions(registrationPath, root)
	if err != nil {
		return cpfpExitBlocked, err
	}
	exFiles := map[string]bool{}
	for _, row := range exceptions {
		if !cpfpContainsStr(files, row.File) {
			say(fmt.Sprintf("   ⚠ 悬空登记：例外登记面的 `%s` 不在扫面里（照实打印 · 不改退码 · 与 `T-13` 的 `J0` 同处置）", row.File))
		}
		exFiles[row.File] = true
	}

	// ── P0 扫描面 ──
	say("   P0 扫描面（`paths:` 形态）：")
	say(fmt.Sprintf("      - '%s/**/*.md'", face))
	for _, g := range cpfpNegativeGlobs(face, exceptions) {
		say(fmt.Sprintf("      %s   ← 已登记的单语例外（%s）", g.Glob, strings.Join(g.Srcs, " · ")))
	}

	var judgedFace []string
	judgedSet := map[string]bool{}
	for _, rel := range files {
		if !exFiles[rel] {
			judgedFace = append(judgedFace, rel)
			judgedSet[rel] = true
		}
	}
	var pairs []cpfpPair
	var unpaired []string
	seenZh := map[string]bool{}
	for _, rel := range judgedFace {
		q, ok := cpfpPairOf(rel)
		if !ok || !judgedSet[q] {
			unpaired = append(unpaired, rel)
			continue
		}
		zh, en := q, rel
		if strings.HasSuffix(rel, cpfpZhSuffix) || rel == cpfpReadmeZh {
			zh, en = rel, q
		}
		// py 侧是 dict（键 = zh 侧）⇒ 每对只计一次（同键覆盖 · 首次遇到序）
		if !seenZh[zh] {
			seenZh[zh] = true
			pairs = append(pairs, cpfpPair{Zh: zh, En: en})
		}
	}
	say(fmt.Sprintf("   扫面 %d 件（成对件 %d 件 = **%d 对** + 已登记单语例外 **%d** 件 + 无对应语言件 **%d** 件）",
		len(files), len(judgedFace)-len(unpaired), len(pairs), len(exFiles), len(unpaired)))
	if len(pairs) == 0 {
		return cpfpExitBlocked, cpfpRefuse("空转：成对件 **0 对**（判不出任何成对性）⇒ **不给结论（rc=2）**")
	}

	// ── P1 机器人面豁免（分支面）──
	branch := cpfpCurrentBranch(root, env)
	if branch != "" {
		if prefix, why, hit := cpfpBotBranchOf(branch); hit {
			say(fmt.Sprintf("   P1 机器人面豁免：当前分支 `%s` 命中 `%s`（%s）⇒ 整跑豁免（照参照件 `if:` 那一格）", branch, prefix, why))
			say("⇒ rc=0（机器人面豁免 · **豁免已打印**、不静默）")
			return cpfpExitOK, nil
		}
	}

	changed, notes, untracked, err := cpfpChangedFace(root, face, rev, env)
	if err != nil {
		return cpfpExitBlocked, err
	}
	for _, n := range notes {
		say("   P1 " + n)
	}
	changedKeys := make([]string, 0, len(changed))
	for k := range changed {
		changedKeys = append(changedKeys, k)
	}
	sort.Strings(changedKeys)
	var chFace, chOut []string
	for _, c := range changedKeys {
		rel := ""
		if strings.HasPrefix(c, face+"/") {
			rel = c[len(face)+1:]
		}
		if rel != "" {
			chFace = append(chFace, rel)
		} else {
			chOut = append(chOut, c)
		}
	}
	chSet := map[string]bool{}
	for _, r := range chFace {
		chSet[r] = true
	}
	for _, u := range untracked {
		say("   P4 untracked ⇒ 不在改动面 ⇒ 不判：" + u)
	}

	revLabel := rev
	if revLabel == "" {
		revLabel = "HEAD"
	}
	say(fmt.Sprintf("   改动面：共 %d 件（公开面内 **%d** 件 · 面外 %d 件不判）· rev=`%s`",
		len(changed), len(chFace), len(chOut), revLabel))

	var reds, exemptHits []string
	for _, rel := range chFace {
		if !judgedSet[rel] {
			if exFiles[rel] {
				exemptHits = append(exemptHits, rel)
				say("   已登记的单语例外在改动面里 ⇒ **不判成对**（逐条打印）：" + rel)
			}
			continue
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].Zh < pairs[j].Zh })
	for _, p := range pairs {
		inZh, inEn := chSet[p.Zh], chSet[p.En]
		if inZh && inEn {
			say(fmt.Sprintf("   同批 ✓ %s + %s", p.Zh, p.En))
		} else if inZh {
			reds = append(reds, fmt.Sprintf("只改 zh 不改 en：%s 在改动面里，而 %s **一字未动**", p.Zh, p.En))
		} else if inEn {
			reds = append(reds, fmt.Sprintf("只改 en 不改 zh：%s 在改动面里，而 %s **一字未动**", p.En, p.Zh))
		}
	}
	for _, rel := range unpaired {
		if chSet[rel] {
			reds = append(reds, fmt.Sprintf("单边出现：%s 在改动面里，却既无对应语言件、又未登记例外", rel))
		} else {
			say(fmt.Sprintf("   静态成对性（只打印 · 不判红）：%s 不成对 —— 该面由 `T-13` 的 `J0` / 闸⑧b ③ 判，本件不重复判红", rel))
		}
	}

	// ── P2 负控 / P3 正控 ──
	if len(reds) > 0 {
		for _, r := range reds {
			say("   ✗ P2 负控命中：" + r)
		}
		say(fmt.Sprintf("⇒ **有红（rc=1）**：%d 处单边改动（处置：补上另一侧，或撤掉这一次单边改动 —— **不许**放宽判据凑绿 ✗）",
			len(reds)))
		return cpfpExitRed, nil
	}
	if len(chFace) == 0 {
		say("   P3 正控：改动面里 **0 件**公开面件（不碰任何公开面件）⇒ rc=0")
	} else {
		say(fmt.Sprintf("   P3 正控：公开面内改动 %d 件 · 全部成对同批（或全数落在已登记例外里：%d 件）⇒ rc=0",
			len(chFace), len(exemptHits)))
	}
	say("⇒ 全绿（rc=0）")
	return cpfpExitOK, nil
}

// ══ `--json` 计数面 ═══════════════════════════════════════════════════════════════

type cpfpCounts struct {
	Face, Rev                                     string
	Files, Exceptions, PairFiles, Pairs, Unpaired int
	ChangedAll, ChangedFace, Untracked            int
}

func cpfpCountsOf(root, face, rev, registrationPath string, env []string) (cpfpCounts, error) {
	out := cpfpCounts{Face: face, Rev: "HEAD"}
	if rev != "" {
		out.Rev = rev
	}
	faceDir := cpfpPyJoin(root, face)
	var files []string
	if fi, err := os.Stat(faceDir); err == nil && fi.IsDir() {
		files = cpfpEnumerateFace(faceDir)
	}
	ex, err := cpfpLoadRegisteredExceptions(registrationPath, root)
	if err != nil {
		return out, err
	}
	exFiles := map[string]bool{}
	for _, r := range ex {
		exFiles[r.File] = true
	}
	var judged []string
	judgedSet := map[string]bool{}
	for _, f := range files {
		if !exFiles[f] {
			judged = append(judged, f)
			judgedSet[f] = true
		}
	}
	nPairs := 0
	for _, f := range judged {
		if q, ok := cpfpPairOf(f); ok && judgedSet[q] {
			nPairs++
		}
	}
	out.Files = len(files)
	out.Exceptions = len(exFiles)
	out.PairFiles = nPairs
	out.Pairs = nPairs / 2
	out.Unpaired = len(judged) - nPairs

	changed, _, untracked, cerr := cpfpChangedFace(root, face, rev, env)
	if cerr != nil {
		changed, untracked = map[string]bool{}, nil
	}
	out.ChangedAll = len(changed)
	cf := 0
	for c := range changed {
		if strings.HasPrefix(c, face+"/") {
			cf++
		}
	}
	out.ChangedFace = cf
	out.Untracked = len(untracked)
	return out, nil
}

func cpfpJSONStr(s string) string {
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
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cpfpJSONCounts 复刻 `json.dumps(c, ensure_ascii=False, sort_keys=True)`（键序 = 字典序）。
func cpfpJSONCounts(c cpfpCounts) string {
	var parts []string
	add := func(k, v string) { parts = append(parts, cpfpJSONStr(k)+": "+v) }
	add("changed_all", strconv.Itoa(c.ChangedAll))
	add("changed_face", strconv.Itoa(c.ChangedFace))
	add("exceptions", strconv.Itoa(c.Exceptions))
	add("face", cpfpJSONStr(c.Face))
	add("files", strconv.Itoa(c.Files))
	add("pair_files", strconv.Itoa(c.PairFiles))
	add("pairs", strconv.Itoa(c.Pairs))
	add("rc", strconv.Itoa(cpfpExitOK))
	add("rev", cpfpJSONStr(c.Rev))
	add("unpaired", strconv.Itoa(c.Unpaired))
	add("untracked", strconv.Itoa(c.Untracked))
	return "{" + strings.Join(parts, ", ") + "}"
}

// ══ 自检（合成 git 夹具 · 用后删 · 不碰真目标）════════════════════════════════════

func cpfpMkdirAll(p string) { _ = os.MkdirAll(p, 0o755) }

func cpfpFixWrite(root, rel, text string) {
	p := cpfpPyJoin(root, rel)
	cpfpMkdirAll(cpfpPyDirname(p))
	_ = os.WriteFile(p, []byte(text), 0o644)
}

func cpfpMkFixture(base string, stderr io.Writer) (string, string, bool) {
	root := cpfpPyJoin(base, "repo")
	files := map[string]string{
		"A.zh-CN.md":           "# A 真源\n",
		"A.en.md":              "# A derived\n",
		"README.zh-CN.md":      "# 真源\n",
		"README.md":            "# Derived\n",
		"design/格式反馈协议-FFP.md": "# 理念件（zh 单语）\n",
	}
	names := make([]string, 0, len(files))
	for k := range files {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, rel := range names {
		cpfpFixWrite(root, cpfpPyJoin("publish/docs", rel), files[rel])
	}
	cpfpFixWrite(root, "publish/docs/design/未登记理念.md", "# 未登记（无 en · 只在扫面里 · 不在改动面）\n")
	reg := cpfpPyJoin(root, "scripts/gates/reg_fixture.py")
	cpfpFixWrite(root, cpfpPyJoin("scripts", "gates", "reg_fixture.py"), "# 自检夹具 · 登记面（形状照真件三键：file/mapping/lines）\n"+
		"SINGLE_LANG_EXCEPTIONS = [\n"+
		"    {\"file\": \"design/格式反馈协议-FFP.md\", \"mapping\": \"A13\", \"lines\": 263, \"reason\": \"自检夹具\"},\n"+
		"]\n")
	cpfpFixWrite(root, "docs/OTHER.md", "# 面外（不在扫描面里）\n")
	env := cpfpGitEnv(base)
	for _, a := range [][]string{{"init", "-q", "-b", "main"}, {"add", "-A"}, {"commit", "-q", "-m", "fixture"}} {
		rc, out := cpfpGitRun(root, a, env)
		if rc != 0 {
			fmt.Fprintf(stderr, "自检夹具建不起来（git %s ⇒ rc=%d：%s）\n", strings.Join(a, " "), rc, cpfpPyStrip(out))
			return root, "", false
		}
	}
	return root, cpfpPyRelpath(reg, root), true
}

func cpfpRunQuiet(root, regRel string, rev string) int {
	env := cpfpGitEnv(root)
	rc, err := cpfpJudge(io.Discard, root, cpfpFaceDefault, rev, regRel, env, true)
	if err != nil {
		return cpfpExitBlocked
	}
	return rc
}

func cpfpSelfTest(stdout, stderr io.Writer) int {
	ok := 0
	var fails []string
	check := func(label string, want, got int) {
		pad := 46 - utf8.RuneCountInString(label)
		if pad < 0 {
			pad = 0
		}
		if want == got {
			ok++
			fmt.Fprintf(stdout, "   自检 ✓ %s%s rc=%d\n", label, strings.Repeat(" ", pad), got)
		} else {
			fails = append(fails, fmt.Sprintf("%s（要 rc=%d · 得 rc=%d）", label, want, got))
			fmt.Fprintf(stdout, "   自检 ✗ %s%s rc=%d（要 %d）\n", label, strings.Repeat(" ", pad), got, want)
		}
	}

	base, err := os.MkdirTemp("", "zerg-pairing-selftest-")
	if err != nil {
		fmt.Fprintf(stderr, "自检夹具建不起来（%s）\n", err)
		return cpfpExitBlocked
	}
	defer func() { _ = os.RemoveAll(base) }()

	root, reg, built := cpfpMkFixture(base, stderr)
	if !built {
		fmt.Fprint(stderr, "⛔ 自检不过 ⇒ 拒绝扫真目标（同族先例 check-publish-face-sync.py ④）\n")
		return cpfpExitRed
	}
	env := cpfpGitEnv(base)

	// ① 正控：不碰任何公开面件（改面外件）
	cpfpFixWrite(root, "docs/OTHER.md", "# 面外（不在扫描面里）改了\n")
	check("正控：不碰公开面件 ⇒ 0", cpfpExitOK, cpfpRunQuiet(root, reg, ""))
	cpfpGitRun(root, []string{"checkout", "--", "docs/OTHER.md"}, env)

	// ② 负控：只改 zh 不改 en
	cpfpFixWrite(root, "publish/docs/A.zh-CN.md", "# A 真源（改）\n")
	check("负控：只改 zh 不改 en ⇒ 1", cpfpExitRed, cpfpRunQuiet(root, reg, ""))
	cpfpGitRun(root, []string{"checkout", "--", "publish/docs/A.zh-CN.md"}, env)

	// ③ 负控：只改 en 不改 zh
	cpfpFixWrite(root, "publish/docs/A.en.md", "# A derived（改）\n")
	check("负控：只改 en 不改 zh ⇒ 1", cpfpExitRed, cpfpRunQuiet(root, reg, ""))
	cpfpGitRun(root, []string{"checkout", "--", "publish/docs/A.en.md"}, env)

	// ④ 正控：成对同批
	cpfpFixWrite(root, "publish/docs/A.zh-CN.md", "# A 真源（改）\n")
	cpfpFixWrite(root, "publish/docs/A.en.md", "# A derived（改）\n")
	check("正控：成对同批 ⇒ 0", cpfpExitOK, cpfpRunQuiet(root, reg, ""))
	cpfpGitRun(root, []string{"checkout", "--", "publish/docs/A.zh-CN.md", "publish/docs/A.en.md"}, env)

	// ⑤ 负向 glob 格：改已登记的单语例外 ⇒ 不判成对
	cpfpFixWrite(root, "publish/docs/design/格式反馈协议-FFP.md", "# 理念件（改了）\n")
	check("负向 glob：改已登记单语例外 ⇒ 0", cpfpExitOK, cpfpRunQuiet(root, reg, ""))
	cpfpGitRun(root, []string{"checkout", "--", "publish/docs/design/格式反馈协议-FFP.md"}, env)

	// ⑥ 负控：改动面里出现未登记的单语件（笔面 · 提交一笔）
	cpfpFixWrite(root, "publish/docs/design/未登记理念.md", "# 未登记（无 en · 未登记 · 这笔真的动了它）\n")
	cpfpGitRun(root, []string{"commit", "-q", "-a", "-m", "改未登记的单语件"}, env)
	check("负控：改动面里未登记单语件 ⇒ 1", cpfpExitRed, cpfpRunQuiet(root, reg, "HEAD~1..HEAD"))
	cpfpGitRun(root, []string{"reset", "-q", "--hard", "HEAD~1"}, env)

	// ⑦ 未登记的单语件「不在改动面」⇒ 只打印不判红
	check("不成对件不在改动面 · 只打印不判红 ⇒ 0", cpfpExitOK, cpfpRunQuiet(root, reg, ""))

	// ⑧ 机器人笔豁免
	cpfpFixWrite(root, "publish/docs/A.zh-CN.md", "# A 真源（机器人改）\n")
	rc, _ := cpfpGitRun(root, []string{"commit", "-q", "-a",
		"--author=dependabot[bot] <49699333+dependabot[bot]@users.noreply.github.com>",
		"-m", "bot: 只改 zh"}, env)
	if rc != 0 {
		check("自检夹具：机器人笔建不起来", 0, rc)
	} else {
		check("机器人笔豁免（只改 zh 的机器人笔）⇒ 0", cpfpExitOK, cpfpRunQuiet(root, reg, "HEAD~1..HEAD"))
	}

	// ⑨ 同一改动面改成「人笔」⇒ 变红（成对负控）
	cpfpFixWrite(root, "publish/docs/A.zh-CN.md", "# A 真源（人改）\n")
	cpfpGitRun(root, []string{"commit", "-q", "-a", "-m", "人: 只改 zh"}, env)
	check("同一改动面（人笔）⇒ 1（成对负控）", cpfpExitRed, cpfpRunQuiet(root, reg, "HEAD~1..HEAD"))

	// ⑩ 机器人分支豁免（分支面）
	cpfpGitRun(root, []string{"checkout", "-q", "-b", "chore/crowdin"}, env)
	cpfpFixWrite(root, "publish/docs/A.zh-CN.md", "# A 真源（机器人分支改）\n")
	check("机器人分支豁免（chore/crowdin）⇒ 0", cpfpExitOK, cpfpRunQuiet(root, reg, ""))
	cpfpGitRun(root, []string{"checkout", "--", "publish/docs/A.zh-CN.md"}, env)
	cpfpGitRun(root, []string{"checkout", "-q", "main"}, env)

	// ⑪ untracked 件 ⇒ 不在改动面 ⇒ 不判
	cpfpFixWrite(root, "publish/docs/B.zh-CN.md", "# B（未跟踪 · 无 en）\n")
	check("untracked 件不在改动面 ⇒ 0", cpfpExitOK, cpfpRunQuiet(root, reg, ""))
	_ = os.Remove(cpfpPyJoin(root, "publish/docs/B.zh-CN.md"))

	// ⑫ 空扫面 ⇒ rc=2
	empty := cpfpPyJoin(base, "empty")
	cpfpMkdirAll(cpfpPyJoin(empty, "publish/docs"))
	cpfpGitRun(empty, []string{"init", "-q"}, cpfpGitEnv(base))
	check("空扫面（0 件）⇒ 2", cpfpExitBlocked, cpfpRunQuiet(empty, reg, ""))

	// ⑬ 面缺件 ⇒ rc=2
	check("面不在 ⇒ 2", cpfpExitBlocked, cpfpRunQuiet(base, reg, ""))

	// ⑭ rev 解不出 ⇒ rc=2
	check("rev 解不出 ⇒ 2", cpfpExitBlocked, cpfpRunQuiet(root, reg, "no-such-rev"))

	// ⑮ 例外登记面读不到 ⇒ rc=2
	check("例外登记面缺件 ⇒ 2", cpfpExitBlocked, cpfpRunQuiet(root, "scripts/gates/no-such-reg.py", ""))

	total := 15
	if len(fails) > 0 || ok != total {
		fmt.Fprintf(stdout, "   自检**不过**：断言 %d 格 · 通过 %d · 失败 %d\n", total, ok, len(fails))
		for _, f := range fails {
			fmt.Fprintf(stdout, "     ✗ %s\n", f)
		}
		if ok != total && len(fails) == 0 {
			fmt.Fprintf(stdout, "     ✗ 断言条数对不上（通过 %d ≠ 声明 %d）—— 用例被静默截断\n", ok, total)
		}
		return cpfpExitBlocked
	}
	fmt.Fprintf(stdout, "   自检 ✓ 断言 %d 格全过（失败 0 格）\n", total)
	return cpfpExitOK
}

func cpfpListRules(stdout io.Writer) {
	fmt.Fprintf(stdout, "── %s ──\n", cpfpVersion)
	fmt.Fprintf(stdout, "   面（默认）: %s · 成对映射: %s ↔ %s · %s ↔ %s\n", cpfpFaceDefault, cpfpZhSuffix, cpfpEnSuffix, cpfpReadmeZh, cpfpReadmeEn)
	fmt.Fprintf(stdout, "   单一真源（例外登记面）: %s\n", cpfpRegistrationRel)
	fmt.Fprint(stdout, "   负向 glob 形态（参照件 nodejs translations-pr-lint.yml 逐字）: '!<目录>/**' 剔掉已登记的单语例外子树\n")
	fmt.Fprint(stdout, "   机器人分支表（逐条带理由）:\n")
	for _, b := range cpfpBotBranchPrefixes {
		pad := 16 - utf8.RuneCountInString(b.prefix)
		if pad < 0 {
			pad = 0
		}
		fmt.Fprintf(stdout, "     · %s%s %s\n", b.prefix, strings.Repeat(" ", pad), b.why)
	}
	fmt.Fprintf(stdout, "   拒收的逃生开关（一律 rc=2）: %s\n", strings.Join(cpfpRefuseFlags, " · "))
	rows := [][2]string{
		{"P0", "扫描面（`paths:` 形态）：正向 `<面>/**/*.md` + 负向 glob 逐条现读自 `T-11b` 登记面"},
		{"P1", "机器人面豁免：分支面（机器人直推分支前缀表）+ 笔面（`[bot]` / `-bot@` 作者 · `A..B` 形态）"},
		{"P2", "负控：成对件只动一侧 ⇒ rc=1（逐对点名 · 两向分列）· 改动面里未登记的单语件 ⇒ 同判 rc=1"},
		{"P3", "正控：不碰任何公开面件 ⇒ rc=0（逐字打印计数 · 不静默）"},
		{"P4", "只管 tracked 的改动面；untracked 件逐条打印「不在改动面 ⇒ 不判」"},
		{"P5", "静态成对性只打印不判红（`T-13` 的 `J0` / 闸⑧b ③ 的面 · 不重复判）"},
		{"P6", "空转/缺件/登记面读不到/git 读不到/rev 解不出/自检不过 ⇒ rc=2（不给结论）"},
	}
	for _, r := range rows {
		fmt.Fprintf(stdout, "   %s %s\n", r[0], r[1])
	}
}

// ══ 参数（= argparse 子集）════════════════════════════════════════════════════════

type cpfpArgs struct {
	root, face, rev, registration string
	selfTest, noSelfTest          bool
	listRules, json               bool
}

const cpfpUsage = "usage: check-public-face-pairing.py [-h] [--root ROOT] [--face FACE]\n" +
	"                                    [--rev REV] [--registration REGISTRATION]\n" +
	"                                    [--self-test] [--no-self-test]\n" +
	"                                    [--list-rules] [--json]"

const cpfpHelpTextRaw = "usage: check-public-face-pairing.py [-h] [--root ROOT] [--face FACE]\n" +
	"                                    [--rev REV] [--registration REGISTRATION]\n" +
	"                                    [--self-test] [--no-self-test]\n" +
	"                                    [--list-rules] [--json]\n" +
	"\n" +
	"公开面成对闸（`T-12` · 漏点 `E-1`）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT           判据树（默认 = 本仓根）\n" +
	"  --face FACE           扫描面（仓根相对 · 默认 publish/docs）\n" +
	"  --rev REV             改动面：单 rev（<rev> → 工作树）或区间 A..B（逐笔按作者归属）\n" +
	"  --registration REGISTRATION\n" +
	"                        例外登记面（单一真源 · 默认 scripts/gates/check-public-face-\n" +
	"                        anchor.py）\n" +
	"  --self-test           只跑自检（合成 git 夹具 · 不碰真目标）\n" +
	"  --no-self-test        跳过自检（闸里按名调用用这个）\n" +
	"  --list-rules          列出判据 / 退码 / 单一真源\n" +
	"  --json                机器面（rc + 计数）\n"

func cpfpDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func cpfpArgError(stderr io.Writer, msg string) int {
	fmt.Fprintln(stderr, cpfpUsage)
	fmt.Fprintf(stderr, "check-public-face-pairing.py: error: %s\n", msg)
	return cpfpExitBlocked
}

func cpfpParseArgs(stdout, stderr io.Writer, argv []string) (cpfpArgs, int, bool) {
	a := cpfpArgs{root: cpfpDefaultRoot(), face: cpfpFaceDefault, registration: cpfpRegistrationRel}
	var extras []string
	flagsTrue := []string{"--self-test", "--no-self-test", "--list-rules", "--json"}
	for i := 0; i < len(argv); i++ {
		x := argv[i]
		switch {
		case x == "-h" || x == "--help":
			fmt.Fprint(stdout, cpfpHelpTextRaw)
			return a, cpfpExitOK, true
		case x == "--self-test":
			a.selfTest = true
		case x == "--no-self-test":
			a.noSelfTest = true
		case x == "--list-rules":
			a.listRules = true
		case x == "--json":
			a.json = true
		case x == "--root" || x == "--face" || x == "--rev" || x == "--registration":
			if i+1 >= len(argv) {
				return a, cpfpArgError(stderr, "argument "+x+": expected one argument"), true
			}
			i++
			switch x {
			case "--root":
				a.root = argv[i]
			case "--face":
				a.face = argv[i]
			case "--rev":
				a.rev = argv[i]
			case "--registration":
				a.registration = argv[i]
			}
		default:
			if strings.HasPrefix(x, "--root=") {
				a.root = strings.TrimPrefix(x, "--root=")
			} else if strings.HasPrefix(x, "--face=") {
				a.face = strings.TrimPrefix(x, "--face=")
			} else if strings.HasPrefix(x, "--rev=") {
				a.rev = strings.TrimPrefix(x, "--rev=")
			} else if strings.HasPrefix(x, "--registration=") {
				a.registration = strings.TrimPrefix(x, "--registration=")
			} else if eq := cpfpBoolFlagEq(x, flagsTrue); eq != "" {
				return a, cpfpArgError(stderr, "argument "+eq+": ignored explicit argument '"+strings.TrimPrefix(x, eq+"=")+"'"), true
			} else {
				extras = append(extras, x)
			}
		}
	}
	if len(extras) > 0 {
		return a, cpfpArgError(stderr, "unrecognized arguments: "+strings.Join(extras, " ")), true
	}
	return a, cpfpExitOK, false
}

func cpfpBoolFlagEq(x string, flags []string) string {
	for _, f := range flags {
		if strings.HasPrefix(x, f+"=") {
			return f
		}
	}
	return ""
}

// ══ 入口（= py `main()`）═════════════════════════════════════════════════════════

func checkPublicFacePairingGateMain(stdout, stderr io.Writer, argv []string) (rc int) {
	// 最外层兜底（py 原件无此档）⇒ fail-closed rc=2 + 具名文案（口径差见件头 / 未覆盖面）。
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stdout, "⛔ 判据内部异常（不给结论）：%v\n", r)
			rc = cpfpExitBlocked
		}
	}()

	for _, f := range cpfpRefuseFlags {
		if cpfpContainsStr(argv, f) {
			fmt.Fprintf(stdout, "⛔ 拒收：`%s` —— 例外只能**显式登记**（登记面在 %s 里）⇒ **不给结论（rc=2）**\n", f, cpfpRegistrationRel)
			return cpfpExitBlocked
		}
	}
	a, rc2, done := cpfpParseArgs(stdout, stderr, argv)
	if done {
		return rc2
	}

	if a.listRules {
		cpfpListRules(stdout)
		return cpfpExitOK
	}
	if a.selfTest {
		return cpfpSelfTest(stdout, stderr)
	}
	if !a.noSelfTest {
		if rcST := cpfpSelfTest(stdout, stderr); rcST != cpfpExitOK {
			fmt.Fprint(stdout, "⛔ 自检不过 ⇒ 拒绝扫真目标（同族先例 check-publish-face-sync.py ④）\n")
			return cpfpExitBlocked
		}
	}

	root := cpfpPyAbspath(a.root)
	if a.json {
		c, err := cpfpCountsOf(root, a.face, a.rev, a.registration, nil)
		if err != nil {
			fmt.Fprintf(stdout, "⛔ %s\n", err)
			return cpfpExitBlocked
		}
		fmt.Fprintln(stdout, cpfpJSONCounts(c))
		return cpfpExitOK
	}
	fmt.Fprint(stdout, "── 公开面成对闸（`T-12` · 漏点 `E-1`）──\n")
	rcJ, err := cpfpJudge(stdout, root, a.face, a.rev, a.registration, nil, false)
	if err != nil {
		fmt.Fprintf(stdout, "⛔ %s\n", err)
		return cpfpExitBlocked
	}
	return rcJ
}
