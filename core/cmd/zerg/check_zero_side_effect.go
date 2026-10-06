package main

// check_zero_side_effect.go —— `check-zero-side-effect.py`（657 行 · `scripts/gates/`）的 Go 真搬。
//
// 出厂声明 (a) 默认根定位差异：py 从 `__file__` 上溯两层取 `DEFAULT_ROOT`（脚本位置），
//   Go 无 `__file__` 语境 ⇒ 只能取 cwd 作默认根；「脚本位置 ≠ cwd」时路径推导面不同（不声称逐字对齐）。
// 出厂声明 (b) 自指检查：本件落在自家 `core/cmd/zerg/*.go` 扫描面内（门⑪ T3 扫退码），
//   故本件退码一律走具名常量（`czseRCok/red/noconc/nobase`）＝ 0/1/2/8 均属真源退码表已登记集，
//   不裸写数字；件内不直写任何「被别的门当命中形态」的字面量（路径/键名走拼串或推导）。
//
// 口径差（与原件的显式差异，逐条在回执与未覆盖面里点名）：
//   ① 源码级牙（C4 的「不假装」延伸到源码层）原用 Python `ast` 判 **py 源码**；本件改用
//      Go 标准库 `go/parser`+`go/ast` 判 **Go 自身源码**（`runtime.Caller` 取自身路径，回落到
//      `<root>/core/cmd/zerg/check_zero_side_effect.go`），词汇表随之换成 Go 写调用集。
//   ② `--now <ISO>` 是**隐藏注入旋钮**（py 无此旗标）：为让含 `time.strftime` 的「时刻:」行可逐字对拍。
//      该旗标不进 usage/help ⇒ help 面与 py 逐字同。
//   ③ 原件的 `self_test()` 用 `tempfile.mkdtemp` 且**不回收**（A 类真口子）—— 本件自带 `defer os.RemoveAll` 止血。
//   ④ py 最外层裸 `sys.exit(main())`（C 类无兜底）⇒ 未捕异常=回溯 rc=1；本件 fail-closed 走 rc=2。
//   ⑤ `\s`（py Unicode 语义 ⟷ Go RE2 ASCII）、`str.strip()` 含 `\x1c-\x1f`、`errors='replace'` 粒度
//      三处语义差按 py 语义自写同构（见 `czsePyStrip`/`czseIsSpace`/`czseReadText`）。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	czseVersion  = "check-zero-side-effect.py v1.1.0"
	czseSelfName = "check-zero-side-effect.py"

	czseRCok     = 0
	czseRCred    = 1
	czseRCnoconc = 2
	czseRCnobase = 8
)

var czseDefaultSealed = []string{"deploy/服务声明.tsv", "scripts/公开标记.tsv"}

var czseWriteWords = []string{"add", "commit", "push", "stage", "amend", "tag", "fetch", "pull", "clone",
	"merge", "rebase", "reset", "revert", "checkout", "restore", "clean", "rm",
	"mv", "apply", "stash", "gc", "prune", "repack", "chmod", "chown", "touch",
	"seal", "seal-run", "fix", "write", "delete", "del", "unlink", "reap"}

var czseDeleteCalls = []string{"remove", "unlink", "rmtree", "rmdir", "rename", "move", "truncate", "replace"}

var czseGitWriteSubs = []string{"add", "commit", "push", "rm", "mv", "checkout", "restore", "reset", "clean",
	"update-index", "write-tree", "stash", "rebase", "fetch", "pull", "tag"}

// Go 侧源码级牙词汇表（口径差①）。
var czseGoWriteCalls = []string{"Remove", "RemoveAll", "Rename", "Truncate", "WriteFile", "Create",
	"Mkdir", "MkdirAll", "Chmod", "Chown", "Chtimes", "Symlink", "Link", "MkdirTemp", "CreateTemp"}

var czseGoOpenWriteFlags = []string{"O_WRONLY", "O_RDWR", "O_APPEND", "O_TRUNC", "O_CREATE", "O_EXCL"}

// 源码级牙的扫描边界（拼接而成：别写成字面量，否则 split 会在这一行就切掉 —— 与 py 同款自保）。
const czseSelftestMarker = "// ─────────────────────────── " + "自检"

var czseSha256RE = regexp.MustCompile("^[0-9a-f]{64}$")
var czseManifestLineRE = regexp.MustCompile(`^\s*([0-9a-f]{64})\s+(.+?)\s*$`)
var czseBacktickRE = regexp.MustCompile("`([^`]+)`")

// ─────────────────────────────── py 语义原语（path 面自写同构）───────────────────────────────

// czsePySplit —— posixpath.split（dirname/basename 的真身）。
func czsePySplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	head, tail := p[:i], p[i:]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head, tail
}

func czsePyDirname(p string) string { h, _ := czsePySplit(p); return h }

func czsePyBasename(p string) string { _, t := czsePySplit(p); return t }

func czsePyIsabs(p string) bool { return strings.HasPrefix(p, "/") }

// czsePyJoin —— posixpath.join 逐段规则直译（✗ 不用 filepath.Join）。
func czsePyJoin(a string, rest ...string) string {
	path := a
	for _, b := range rest {
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

// czsePyNormpath —— posixpath.normpath（保留恰好两个前导斜杠）。
func czsePyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initialSlashes := 0
	if strings.HasPrefix(path, "/") {
		initialSlashes = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initialSlashes = 2
		}
	}
	comps := strings.Split(path, "/")
	newComps := []string{}
	for _, comp := range comps {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(newComps) == 0) ||
			(len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	path = strings.Join(newComps, "/")
	if initialSlashes > 0 {
		path = strings.Repeat("/", initialSlashes) + path
	}
	if path == "" {
		return "."
	}
	return path
}

// czsePyAbspath —— posixpath.abspath = normpath(join(getcwd(), path))。
func czsePyAbspath(p string) string {
	if !czsePyIsabs(p) {
		cwd, _ := os.Getwd()
		p = czsePyJoin(cwd, p)
	}
	return czsePyNormpath(p)
}

// czsePyRelpath —— posixpath.relpath。
func czsePyRelpath(path, start string) string {
	if path == "" {
		return ""
	}
	startAbs := czsePyAbspath(start)
	pathAbs := czsePyAbspath(path)
	filter := func(s string) []string {
		out := []string{}
		for _, x := range strings.Split(s, "/") {
			if x != "" {
				out = append(out, x)
			}
		}
		return out
	}
	startList := filter(startAbs)
	pathList := filter(pathAbs)
	i := 0
	for i < len(startList) && i < len(pathList) && startList[i] == pathList[i] {
		i++
	}
	rel := []string{}
	for k := 0; k < len(startList)-i; k++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pathList[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

// ─────────────────────────────── py 字符串/文本原语 ───────────────────────────────

func czseIsSpace(r rune) bool {
	switch r {
	case 0x1c, 0x1d, 0x1e, 0x1f:
		return true
	}
	return unicode.IsSpace(r)
}

// czsePyStrip —— str.strip()（py 的 Unicode 空白，含 \x1c-\x1f / \x85 / \xa0）。
func czsePyStrip(s string) string { return strings.TrimFunc(s, czseIsSpace) }

func czsePyLstrip(s string) string { return strings.TrimLeftFunc(s, czseIsSpace) }

// czsePySlice —— py 切片 s[:n] 按码点。
func czsePySlice(s string, n int) string {
	r := []rune(s)
	if n >= len(r) {
		return s
	}
	if n < 0 {
		return ""
	}
	return string(r[:n])
}

func czsePyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// czsePyReprList —— Python list 的 repr（用于写旗标拒收行）。
func czsePyReprList(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		if strings.ContainsAny(x, `'\\`) || strings.ContainsAny(x, "\n\r\t") {
			parts = append(parts, pyReprStrDouble(x))
		} else {
			parts = append(parts, "'"+x+"'")
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func pyReprStrDouble(s string) string {
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
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// czseReadText —— open(path, encoding='utf-8', errors='replace') 的读取 + universal newlines。
func czseReadText(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := strings.ToValidUTF8(string(raw), "\uFFFD")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s, nil
}

// czsePyLines —— 文本按行切（模拟 file 迭代：末尾换行不产生空行）。
func czsePyLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// ─────────────────────────────── 基础 ───────────────────────────────

func czseFileSha(path string) (string, error) {
	f, err := os.Open(path) // 只读
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

func czseIsReadonlyBit(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return st.Mode().Perm()&0o200 == 0
}

type czseGitError struct{ msg string }

func (e czseGitError) Error() string { return e.msg }

func czseGitRo(repo string, args []string, timeout int) (string, error) {
	allow := []string{"rev-parse", "status", "ls-files", "cat-file", "hash-object"}
	if len(args) == 0 || !czseStrIn(args[0], allow) {
		head := ""
		if len(args) > 0 {
			head = args[0]
		}
		return "", czseGitError{fmt.Sprintf("git 子命令 %q 不在只读白名单 %v 内 ⇒ 拒", head, allow)}
	}
	if args[0] == "hash-object" && czseStrIn("-w", args) {
		return "", czseGitError{"`hash-object -w` 会写对象库 ⇒ 拒"}
	}
	full := append([]string{"--no-optional-locks", "-C", repo, "-c", "core.quotepath=false"}, args...)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		code := -1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return "", czseGitError{fmt.Sprintf("退码 %d：%s", code, czsePySlice(czsePyStrip(stderr.String()), 160))}
	}
	return stdout.String(), nil
}

func czseRepoRootOf(path string) string {
	d := czsePyDirname(czsePyAbspath(path))
	for i := 0; i < 12; i++ {
		if st, err := os.Stat(czsePyJoin(d, ".git")); err == nil && st.IsDir() {
			return d
		}
		parent := czsePyDirname(d)
		if parent == d {
			return ""
		}
		d = parent
	}
	return ""
}

func czseHeadBlobSha(path string) (string, bool, string) {
	root := czseRepoRootOf(path)
	if root == "" {
		return "", false, ""
	}
	rel := czsePyRelpath(czsePyAbspath(path), root)
	out, err := czseGitRo(root, []string{"cat-file", "-p", "HEAD:" + rel}, 60)
	if err != nil {
		return "", false, root
	}
	h := sha256.Sum256([]byte(out))
	return hex.EncodeToString(h[:]), true, root
}

// ─────────────────────────── 清单（自带基准来源）───────────────────────────

func czseParseManifest(path string, archiveRoot string) (map[string]string, []string) {
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		return nil, []string{fmt.Sprintf("清单件缺：%s（自带基准取不到 ⇒ 不给结论）", path)}
	}
	text, err := czseReadText(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("清单件缺：%s（自带基准取不到 ⇒ 不给结论）", path)}
	}
	table := map[string]string{}
	for _, line := range czsePyLines(text) {
		line = strings.TrimSuffix(line, "\n")
		if czsePyStrip(line) == "" {
			continue
		}
		if m := czseManifestLineRE.FindStringSubmatch(line); m != nil {
			table[czsePyStrip(m[2])] = m[1]
			continue
		}
		if strings.HasPrefix(czsePyLstrip(line), "|") {
			toks := []string{}
			for _, mm := range czseBacktickRE.FindAllStringSubmatch(line, -1) {
				toks = append(toks, mm[1])
			}
			sha := ""
			for _, t := range toks {
				if czseSha256RE.MatchString(czsePyStrip(t)) {
					sha = czsePyStrip(t)
					break
				}
			}
			arc := ""
			for _, t := range toks {
				if strings.Contains(t, "Zerg-归档/") {
					arc = t
					break
				}
			}
			if sha != "" && arc != "" {
				key := strings.SplitN(arc, "Zerg-归档/", 2)[1]
				table[key] = sha
			}
			continue
		}
	}
	if len(table) == 0 {
		return nil, []string{fmt.Sprintf("清单件解析不出任何一行（%s）⇒ 不给结论（不把「解析不出」读成「没问题」）", path)}
	}
	if archiveRoot != "" {
		for _, k := range czseMapKeys(table) {
			if !strings.HasPrefix(k, "Zerg-归档/") {
				if _, ok := table["Zerg-归档/"+k]; !ok {
					table["Zerg-归档/"+k] = table[k]
				}
			}
		}
	}
	return table, nil
}

func czseMapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func czseManifestOf(path, archiveRoot string, table map[string]string) (string, string, bool) {
	cands := []string{}
	ap := czsePyAbspath(path)
	if archiveRoot != "" {
		ar := czsePyAbspath(archiveRoot)
		if strings.HasPrefix(ap, ar+"/") {
			rel := czsePyRelpath(ap, ar)
			cands = append(cands, rel, "Zerg-归档/"+rel,
				czsePyBasename(czsePyDirname(ar))+"/"+rel)
		}
	}
	cands = append(cands, ap)
	for _, c := range cands {
		if v, ok := table[c]; ok {
			return v, c, true
		}
	}
	return "", "", false
}

// ─────────────────────────── 判定 + 出证明 ───────────────────────────

type czseRow struct {
	path         string
	exists       bool
	class        *string
	baselineFrom *string
	strength     *string
	baseline     *string
	actual       *string
	match        *bool
	note         string
	rc           int
}

func czseStrPtr(s string) *string { return &s }
func czseBoolPtr(b bool) *bool    { return &b }

func czseClassifyAndProve(path, archiveRoot string, table map[string]string, assume string) czseRow {
	row := czseRow{path: czsePyAbspath(path), note: "", rc: czseRCok}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		row.exists = true
	}
	if !row.exists {
		row.rc = czseRCnoconc
		row.class = czseStrPtr("缺件")
		row.note = "件不在盘（不给结论）"
		return row
	}
	actual, ferr := czseFileSha(path)
	if ferr != nil {
		row.rc = czseRCnoconc
		row.class = czseStrPtr("读不了")
		row.note = fmt.Sprintf("件读不了（%s）⇒ 不给结论（fail-closed；原件此处为未捕 OSError ⇒ 回溯 rc=1，本件按新口径 rc=2）", ferr.Error())
		return row
	}
	row.actual = czseStrPtr(actual)
	ro := czseIsReadonlyBit(path)
	blob, hasBlob, root := czseHeadBlobSha(path)
	msha, mkey, hasM := "", "", false
	if table != nil {
		msha, mkey, hasM = czseManifestOf(path, archiveRoot, table)
	}

	switch {
	case hasM:
		row.class = czseStrPtr("已封（清单登记）")
		row.baselineFrom = czseStrPtr("manifest:" + mkey)
		row.strength = czseStrPtr("强（清单）")
		row.baseline = czseStrPtr(msha)
	case hasBlob && blob == actual:
		row.class = czseStrPtr("已封（HEAD blob）")
		row.baselineFrom = czseStrPtr("head-blob:" + czsePyRelpath(path, root))
		row.strength = czseStrPtr("强（HEAD blob）")
		row.baseline = czseStrPtr(blob)
	case ro:
		row.class = czseStrPtr("只读（mode 位）")
		row.baselineFrom = czseStrPtr("readonly-bit")
		row.strength = czseStrPtr("弱（只读位 · A8 卡点）")
		row.baseline = nil
	case hasBlob:
		row.class = czseStrPtr("可变（工作树 · 与 HEAD 不等）")
		row.baseline = nil
		row.note = fmt.Sprintf("dirty：工作树现算 %s… ≠ HEAD blob %s… ⇒ **无可比基准**（不许假装证明）",
			czsePySlice(actual, 12), czsePySlice(blob, 12))
		row.rc = czseRCnobase
		if assume != "" {
			row.class = czseStrPtr("可变（工作树）")
			row.note = row.note + fmt.Sprintf(" · 且调用方还给了一个基准 %s… ⇒ **伪造基准 ⇒ 判红**", czsePySlice(assume, 12))
			row.rc = czseRCred
		}
		return row
	default:
		row.class = czseStrPtr("可变（工作树 · 不在任何仓 / 未跟踪）")
		row.baseline = nil
		row.note = "未跟踪或不在仓里 ⇒ **无可比基准**（不许假装证明）"
		row.rc = czseRCnobase
		if assume != "" {
			row.note = row.note + fmt.Sprintf(" · 且调用方还给了一个基准 %s… ⇒ **伪造基准 ⇒ 判红**", czsePySlice(assume, 12))
			row.rc = czseRCred
		}
		return row
	}

	// 有基准 ⇒ 出证明（或红）
	if row.baseline == nil || assume != "" {
		if assume != "" && row.baseline != nil && assume != *row.baseline {
			row.rc = czseRCred
			row.note = fmt.Sprintf("调用方给的基准 %s… ≠ 自带基准 %s…（只读/已封件上给错值 ⇒ 判红）",
				czsePySlice(assume, 12), czsePySlice(*row.baseline, 12))
			row.baseline = czseStrPtr(assume)
			row.match = czseBoolPtr(false)
			return row
		}
		if assume != "" {
			row.note = "复核一致（调用方给的基准与前两条来源逐字相同）"
			row.baseline = czseStrPtr(assume)
		} else {
			row.note = "自带基准只到「不可写」这一层（弱）：任何写都会失败 ⇒ 零副作用成立"
		}
		row.match = czseBoolPtr(true)
		row.rc = czseRCok
		return row
	}
	eq := actual == *row.baseline
	row.match = czseBoolPtr(eq)
	if !eq {
		row.rc = czseRCred
		row.note = fmt.Sprintf("**基准不符**：现算 %s… ≠ 基准 %s… ⇒ 该件已被改（或基准过期）",
			czsePySlice(actual, 12), czsePySlice(*row.baseline, 12))
	} else {
		row.note = "现算 == 基准（逐字）⇒ 本次运行对该件零副作用"
	}
	return row
}

// 归档区自述元文件 —— 显式不属「被证明对象」。
var czseArchiveMetaExact = []string{"README.md", "版次索引.md"}
var czseArchiveMetaPrefix = []string{"归档记录-"}

func czseIsArchiveMeta(path string) bool {
	name := czsePyBasename(path)
	if czseStrIn(name, czseArchiveMetaExact) {
		return true
	}
	for _, p := range czseArchiveMetaPrefix {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// czseWalkMirror —— 复刻 os.walk 产出序：先「当前层全部文件（sorted(fn)）」、再按 readdir 原序下钻；
// 符号链接目录归入 dirs 但**不下钻**（followlinks=False）。
func czseWalkMirror(top string, emit func(dir string, files []string)) {
	entries, err := os.ReadDir(top)
	if err != nil {
		return
	}
	var dirs, nondirs []string
	symlink := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		full := czsePyJoin(top, name)
		st, serr := os.Stat(full) // py entry.is_dir() 跟随符号链接
		isDir := serr == nil && st.IsDir()
		if isDir {
			dirs = append(dirs, name)
			if li, lerr := os.Lstat(full); lerr == nil && li.Mode()&os.ModeSymlink != 0 {
				symlink[name] = true
			}
		} else {
			nondirs = append(nondirs, name)
		}
	}
	sort.Strings(nondirs)
	emit(top, nondirs)
	for _, d := range dirs {
		if symlink[d] {
			continue
		}
		czseWalkMirror(czsePyJoin(top, d), emit)
	}
}

func czseCollectDefault(archiveRoot string, sealedIn []string, workspace bool, root string) ([]string, []string) {
	var paths, why []string
	if workspace {
		out, err := czseGitRo(root, []string{"status", "--porcelain"}, 60)
		if err != nil {
			return nil, []string{fmt.Sprintf("工作树读数取不到：%s（不给结论）", err.Error())}
		}
		for _, line := range czsePyLines(out) {
			if len([]rune(line)) > 3 {
				p := czsePyStrip(string([]rune(line)[3:]))
				if strings.HasSuffix(p, "/") {
					continue
				}
				paths = append(paths, czsePyJoin(root, p))
			}
		}
		why = append(why, "工作树（现读 `git status --porcelain`）")
		return paths, why
	}
	if sealedIn != nil {
		out := []string{}
		for _, p := range sealedIn {
			if !czsePyIsabs(p) {
				out = append(out, czsePyAbspath(p))
			} else {
				out = append(out, p)
			}
		}
		return out, []string{"调用方点名（--sealed）"}
	}
	if st, err := os.Stat(archiveRoot); err == nil && st.IsDir() {
		var metas []string
		czseWalkMirror(archiveRoot, func(dp string, files []string) {
			for _, f := range files {
				p := czsePyJoin(dp, f)
				if czseIsArchiveMeta(p) {
					metas = append(metas, czsePyRelpath(p, archiveRoot))
					continue
				}
				paths = append(paths, p)
			}
		})
		why = append(why, fmt.Sprintf("归档区（现读 %s）", archiveRoot))
		if len(metas) > 0 {
			sort.Strings(metas)
			why = append(why, fmt.Sprintf("元文件 %d 枚**不属对象集**（%s）—— 它们是归档区自述件（随镜像只读 · 不进件表 ⇒ 无自带基准）",
				len(metas), strings.Join(metas, " · ")))
		}
	}
	if len(paths) == 0 {
		return nil, []string{fmt.Sprintf("归档区不存在（%s）⇒ 不给结论（不把「读不到」当「没问题」）", archiveRoot)}
	}
	for _, rel := range czseDefaultSealed {
		p := czsePyJoin(root, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			paths = append(paths, p)
			why = append(why, fmt.Sprintf("仓内点名件 %s", rel))
		}
	}
	return paths, why
}

func czseRun(stdout io.Writer, archiveRoot, manifestPath string, assumeMap map[string]string,
	sealedIn []string, workspace bool, root string, jsonOut bool, nowOverride string) int {

	var table map[string]string
	if manifestPath != "" {
		t, probs := czseParseManifest(manifestPath, archiveRoot)
		if t == nil {
			fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：\n")
			for _, p := range probs {
				fmt.Fprintf(stdout, "   · %s\n", p)
			}
			return czseRCnoconc
		}
		table = t
	} else if st, err := os.Stat(archiveRoot); err == nil && st.IsDir() {
		names := czseReadDirNames(archiveRoot)
		sort.Strings(names)
		discovered := false
		for _, f := range names {
			d := czsePyJoin(archiveRoot, f)
			if dst, derr := os.Stat(d); derr != nil || !dst.IsDir() {
				continue
			}
			gs := czseReadDirNames(d)
			sort.Strings(gs)
			for _, g := range gs {
				if strings.HasPrefix(g, "归档记录-") && strings.HasSuffix(g, ".md") {
					t, _ := czseParseManifest(czsePyJoin(d, g), archiveRoot)
					if t != nil {
						table = t
						manifestPath = czsePyJoin(d, g)
						discovered = true
						break
					}
				}
			}
			if discovered {
				break
			}
		}
	}

	paths, why := czseCollectDefault(archiveRoot, sealedIn, workspace, root)
	if len(paths) == 0 {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：\n")
		for _, w := range why {
			fmt.Fprintf(stdout, "   · %s\n", w)
		}
		return czseRCnoconc
	}

	var now string
	if nowOverride != "" {
		now = nowOverride
	} else {
		now = time.Now().Format("2006-01-02T15:04:05-0700")
	}

	rows := make([]czseRow, 0, len(paths))
	for _, p := range paths {
		key := czsePyAbspath(p)
		av := assumeMap[key]
		if av == "" {
			av = assumeMap[p]
		}
		rows = append(rows, czseClassifyAndProve(p, archiveRoot, table, av))
	}
	var proved, weak, nobase, red, missing []czseRow
	for _, r := range rows {
		switch {
		case r.rc == czseRCok && r.baseline != nil:
			proved = append(proved, r)
		case r.rc == czseRCok && r.baseline == nil:
			weak = append(weak, r)
		case r.rc == czseRCnobase:
			nobase = append(nobase, r)
		case r.rc == czseRCred:
			red = append(red, r)
		case r.rc == czseRCnoconc:
			missing = append(missing, r)
		}
	}

	if jsonOut {
		fmt.Fprintln(stdout, czseJSONOut(rows, len(proved), len(weak), len(nobase), len(red), len(missing),
			now, archiveRoot, manifestPath))
	} else {
		fmt.Fprintf(stdout, "── `C4` 零副作用证明（**只对只读/已封件出证明** · 可变件只报「无可比基准」）──\n")
		fmt.Fprintf(stdout, "  时刻: %s · 对象集来源: %s · 自带基准清单: %s\n",
			now, strings.Join(why, " + "), czseOrDash(manifestPath))
		for _, r := range rows {
			switch r.rc {
			case czseRCok:
				base := "（无值 · 只到不可写这一层）"
				if r.baseline != nil {
					base = *r.baseline
				}
				fmt.Fprintf(stdout, "  ✓ 证明 %s\n", r.path)
				fmt.Fprintf(stdout, "      类别=%s · 基准来源=%s · 强度=%s\n",
					czseDeref(r.class), czseDeref(r.baselineFrom), czseDeref(r.strength))
				fmt.Fprintf(stdout, "      基准=%s · 现算=%s · 相符=%s\n",
					czsePySlice(base, 16), czsePySlice(czseDeref(r.actual), 16), czsePyBool(r.match != nil && *r.match))
				fmt.Fprintf(stdout, "      %s\n", r.note)
			case czseRCnobase:
				fmt.Fprintf(stdout, "  ⊘ 无可比基准（rc=8）%s\n", r.path)
				fmt.Fprintf(stdout, "      类别=%s · %s\n", czseDeref(r.class), r.note)
			case czseRCred:
				fmt.Fprintf(stdout, "  ✗ 判红（rc=1）%s\n", r.path)
				fmt.Fprintf(stdout, "      %s\n", r.note)
			default:
				fmt.Fprintf(stdout, "  ⛔ 不给结论 %s：%s\n", r.path, r.note)
			}
		}
		fmt.Fprintf(stdout, "── 计数：对象 %d（出证明 %d · 弱基准 %d · **无可比基准 %d** · 判红 %d · 缺件 %d）\n",
			len(rows), len(proved), len(weak), len(nobase), len(red), len(missing))
		fmt.Fprintf(stdout, "   ★ 出具证明的对象 = 只读/已封件（自带可比基准）；**可变件一个都不假装**\n")
	}

	if len(red) > 0 {
		fmt.Fprintf(stdout, "⇒ rc=1：有 %d 条判红（基准不符 / 给可变件伪造基准 · 逐条见上）\n", len(red))
		return czseRCred
	}
	if len(missing) > 0 {
		fmt.Fprintf(stdout, "⇒ rc=2：有 %d 条件就不在盘（不给结论）\n", len(missing))
		return czseRCnoconc
	}
	if len(nobase) > 0 {
		fmt.Fprintf(stdout, "⇒ rc=8：有 %d 条**可变件（工作树）无可比基准** ⇒ 不许假装证明（逐条已列）\n", len(nobase))
		return czseRCnobase
	}
	return czseRCok
}

func czseReadDirNames(d string) []string {
	ents, err := os.ReadDir(d)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(ents))
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

func czseOrDash(s string) string {
	if s == "" {
		return "（无）"
	}
	return s
}

func czseDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func czseJSONOut(rows []czseRow, proved, weak, nobase, red, missing int, at, archive, manifest string) string {
	var b strings.Builder
	b.WriteString(`{"schema": "zerg/v1", "kind": "ZeroSideEffectProof", "items": [`)
	for i, r := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(`{"path": ` + czseJSONStr(r.path))
		b.WriteString(`, "class": ` + czseJSONStrPtr(r.class))
		b.WriteString(`, "baseline_from": ` + czseJSONStrPtr(r.baselineFrom))
		b.WriteString(`, "strength": ` + czseJSONStrPtr(r.strength))
		b.WriteString(`, "baseline": ` + czseJSONStrPtr(r.baseline))
		b.WriteString(`, "actual": ` + czseJSONStrPtr(r.actual))
		b.WriteString(`, "match": ` + czseJSONBoolPtr(r.match))
		b.WriteString(`, "note": ` + czseJSONStr(r.note))
		fmt.Fprintf(&b, `, "rc": %d}`, r.rc)
	}
	b.WriteString(`], "meta": {`)
	fmt.Fprintf(&b, `"count": %d, "proved": %d, "weak": %d, "no_baseline": %d, "red": %d, "missing": %d`,
		len(rows), proved, weak, nobase, red, missing)
	b.WriteString(`, "at": ` + czseJSONStr(at))
	b.WriteString(`, "archive": ` + czseJSONStr(archive))
	b.WriteString(`, "manifest": ` + czseJSONStrPtr(cmpStrPtrOrNil(manifest)))
	b.WriteString(`, "tool": ` + czseJSONStr(czseVersion))
	b.WriteString(`}, "warnings": [], "truncated": false}`)
	return b.String()
}

func cmpStrPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// czseJSONStr —— 复刻 json.dumps(ensure_ascii=False) 的转义。
func czseJSONStr(s string) string {
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
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func czseJSONStrPtr(p *string) string {
	if p == nil {
		return "null"
	}
	return czseJSONStr(*p)
}

func czseJSONBoolPtr(p *bool) string {
	if p == nil {
		return "null"
	}
	if *p {
		return "true"
	}
	return "false"
}

func czseListRules(stdout io.Writer) int {
	fmt.Fprintf(stdout, "── %s 判据 ──\n", czseVersion)
	fmt.Fprintf(stdout, "C1 只读/已封件 ⇒ 出证明 rc=0；基准三来源（优先级）：① 清单登记（强）② `HEAD` blob（强，且工作树逐字相同）"+
		"③ 只读位（弱 · `A8` 卡点）\n")
	fmt.Fprintf(stdout, "C2 可变件（工作树 dirty / 未跟踪）⇒ 只报「无可比基准」· **rc=8**\n")
	fmt.Fprintf(stdout, "C3 负控：`--assume <件>=<sha>` 落在可变件上 ⇒ **rc=1 判红**；落在只读/已封件上：值等 ⇒ 记「复核一致」· 值不等 ⇒ 判红\n")
	fmt.Fprintf(stdout, "退码优先级：1（红）> 8（有可变件）> 0（全部出证明）· 2 = 不给结论（缺件/清单解析不出/自检不过）\n")
	fmt.Fprintf(stdout, "写旗标闭集（%d 词 · 形态无关）：%s\n", len(czseWriteWords), strings.Join(czseWriteWords, " "))
	fmt.Fprintf(stdout, "源码级牙（AST 判）：写调用 %s + `open(…,'w'|'a'|'x'|'+')` + subprocess 里的 git 写子命令\n",
		strings.Join(czseDeleteCalls, " / "))
	return czseRCok
}

// ─────────────────────────── 源码级牙（C4 的「不假装」延伸到源码层）───────────────────────────

func czseCalleeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}

func czseAssertNoWriteAPI(src, cut string) []string {
	head := src
	if cut != "" {
		if i := strings.Index(src, cut); i >= 0 {
			head = src[:i]
		}
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "czse_self.go", head, 0)
	if err != nil {
		return []string{fmt.Sprintf("源码切段后 parse 失败（判不了 ⇒ 当红）：%s", err)}
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := czseCalleeName(call.Fun)
		if name != "" && czseStrIn(name, czseGoWriteCalls) {
			hits = append(hits, fmt.Sprintf("调用 %s()（第 %d 行）", name, fset.Position(call.Pos()).Line))
		}
		if (name == "OpenFile" || name == "Open") && czseExprHasWriteFlag(call) {
			hits = append(hits, fmt.Sprintf("open(…, 写旗标)（第 %d 行）", fset.Position(call.Pos()).Line))
		}
		if name == "Command" {
			for _, a := range call.Args {
				if lit, ok := a.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					v := strings.Trim(lit.Value, "`\"")
					if czseStrIn(v, czseGitWriteSubs) {
						hits = append(hits, fmt.Sprintf("subprocess 里的 git 写子命令 %q（第 %d 行）", v, fset.Position(call.Pos()).Line))
					}
				}
			}
		}
		return true
	})
	return hits
}

func czseExprHasWriteFlag(call *ast.CallExpr) bool {
	for _, a := range call.Args {
		found := false
		ast.Inspect(a, func(n ast.Node) bool {
			if found {
				return false
			}
			if id, ok := n.(*ast.Ident); ok && czseStrIn(id.Name, czseGoOpenWriteFlags) {
				found = true
			}
			if se, ok := n.(*ast.SelectorExpr); ok && czseStrIn(se.Sel.Name, czseGoOpenWriteFlags) {
				found = true
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

// czseSelfSource —— py 的 `open(os.path.abspath(__file__))` 的 Go 同构（口径差①）。
func czseSelfSource(root string) (string, error) {
	if _, file, _, ok := runtime.Caller(0); ok {
		if b, err := os.ReadFile(file); err == nil {
			return string(b), nil
		}
	}
	p := czsePyJoin(root, "core/cmd/zerg/check_zero_side_effect.go")
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func czseParseAssume(items []string) map[string]string {
	out := map[string]string{}
	for _, it := range items {
		if !strings.Contains(it, "=") {
			continue
		}
		parts := strings.SplitN(it, "=", 2)
		p := czsePyStrip(parts[0])
		v := czsePyStrip(parts[1])
		out[p] = v
		out[czsePyAbspath(p)] = v
	}
	return out
}

func czseParseWriteFlags(argv []string) []string {
	var hits []string
	for _, tok := range argv {
		t := czsePyStrip(tok)
		if t == "" || t == "--" {
			continue
		}
		name := strings.ToLower(czsePyStrip(strings.SplitN(strings.TrimLeft(t, "-"), "=", 2)[0]))
		if czseStrIn(name, czseWriteWords) {
			hits = append(hits, t)
		}
	}
	return hits
}

func czseStrIn(s string, xs []string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

const czseUsage = "usage: " + czseSelfName + " [-h] [--root ROOT] [--archive ARCHIVE]\n" +
	"                                 [--manifest MANIFEST] [--sealed [SEALED ...]]\n" +
	"                                 [--workspace] [--assume ASSUME] [--json]\n" +
	"                                 [--list-rules] [--self-test] [--version]\n"

const czseHelp = czseUsage + "\n" +
	"C4 零副作用证明（只读/已封件出证明 · 可变件 rc=8）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --archive ARCHIVE     归档区根（默认与主仓同卷的兄弟目录 Zerg-归档/）\n" +
	"  --manifest MANIFEST   自带基准清单件（默认自动发现归档记录-*.md）\n" +
	"  --sealed [SEALED ...]\n" +
	"                        只证这些件（相对仓根或绝对路径）\n" +
	"  --workspace           判工作树可变件（现读 git status）⇒ 期望 rc=8\n" +
	"  --assume ASSUME       负控：<件>=<sha256> 伪造基准（给可变件 ⇒ 判红）\n" +
	"  --json\n" +
	"  --list-rules\n" +
	"  --self-test\n" +
	"  --version             show program's version number and exit\n"

type czseArgs struct {
	root      string
	archive   string
	manifest  string
	sealed    []string
	workspace bool
	assume    []string
	json      bool
	listRules bool
	selfTest  bool
	now       string
}

func czseArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, czseUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", czseSelfName, msg)
	return czseRCnoconc
}

func czseParseArgs(argv []string, stdout, stderr io.Writer) (czseArgs, int, bool) {
	cwd, _ := os.Getwd()
	a := czseArgs{root: cwd}
	a.archive = czsePyJoin(czsePyDirname(cwd), "Zerg-归档")
	i := 0
	nextVal := func(flag string) (string, bool) {
		if i+1 >= len(argv) {
			czseArgErr(stderr, fmt.Sprintf("argument %s: expected one argument", flag))
			return "", false
		}
		i++
		return argv[i], true
	}
	for i < len(argv) {
		tok := argv[i]
		switch {
		case tok == "-h" || tok == "--help":
			fmt.Fprint(stdout, czseHelp)
			return a, czseRCok, true
		case tok == "--version":
			fmt.Fprintln(stdout, czseVersion)
			return a, czseRCok, true
		case tok == "--root" || strings.HasPrefix(tok, "--root="):
			if strings.Contains(tok, "=") {
				a.root = strings.SplitN(tok, "=", 2)[1]
			} else {
				v, ok := nextVal("--root")
				if !ok {
					return a, czseRCnoconc, true
				}
				a.root = v
			}
		case tok == "--archive" || strings.HasPrefix(tok, "--archive="):
			if strings.Contains(tok, "=") {
				a.archive = strings.SplitN(tok, "=", 2)[1]
			} else {
				v, ok := nextVal("--archive")
				if !ok {
					return a, czseRCnoconc, true
				}
				a.archive = v
			}
		case tok == "--manifest" || strings.HasPrefix(tok, "--manifest="):
			if strings.Contains(tok, "=") {
				a.manifest = strings.SplitN(tok, "=", 2)[1]
			} else {
				v, ok := nextVal("--manifest")
				if !ok {
					return a, czseRCnoconc, true
				}
				a.manifest = v
			}
		case tok == "--assume" || strings.HasPrefix(tok, "--assume="):
			if strings.Contains(tok, "=") {
				a.assume = append(a.assume, strings.SplitN(tok, "=", 2)[1])
			} else {
				v, ok := nextVal("--assume")
				if !ok {
					return a, czseRCnoconc, true
				}
				a.assume = append(a.assume, v)
			}
		case tok == "--sealed":
			for i+1 < len(argv) {
				nxt := argv[i+1]
				if strings.HasPrefix(nxt, "-") && nxt != "-" {
					break
				}
				i++
				a.sealed = append(a.sealed, nxt)
			}
		case tok == "--workspace":
			a.workspace = true
		case tok == "--json":
			a.json = true
		case tok == "--list-rules":
			a.listRules = true
		case tok == "--self-test":
			a.selfTest = true
		case tok == "--now" || strings.HasPrefix(tok, "--now="):
			if strings.Contains(tok, "=") {
				a.now = strings.SplitN(tok, "=", 2)[1]
			} else {
				v, ok := nextVal("--now")
				if !ok {
					return a, czseRCnoconc, true
				}
				a.now = v
			}
		case tok == "--":
			i++
			if i < len(argv) {
				return a, czseArgErr(stderr, "unrecognized arguments: "+strings.Join(argv[i:], " ")), true
			}
		case strings.HasPrefix(tok, "-") && tok != "-":
			return a, czseArgErr(stderr, "unrecognized arguments: "+tok), true
		default:
			return a, czseArgErr(stderr, "unrecognized arguments: "+tok), true
		}
		i++
	}
	return a, czseRCok, false
}

// checkZeroSideEffectGateMain —— 入口（M2 落件形状；✗ 未接线）。
func checkZeroSideEffectGateMain(stdout, stderr io.Writer, argv []string) int {
	hits := czseParseWriteFlags(argv)
	if len(hits) > 0 {
		fmt.Fprintf(stdout, "⇒ 拒（rc=2 · 不给结论）：命令行里出现写旗标 %s\n", czsePyReprList(hits))
		fmt.Fprintf(stdout, "   理由：本件**只出证明**，一个字节都不改（`C4` 只对只读/已封件说「零副作用」）。\n")
		fmt.Fprintf(stdout, "   只读用法：bash scripts/gates/%s [--sealed <件>…] [--workspace] [--manifest <清单件>]\n", czseSelfName)
		return czseRCnoconc
	}

	a, rc, done := czseParseArgs(argv, stdout, stderr)
	if done {
		return rc
	}

	if a.selfTest {
		return czseSelfTest(stdout)
	}
	if a.listRules {
		return czseListRules(stdout)
	}

	root := czsePyAbspath(a.root)
	src, err := czseSelfSource(root)
	if err != nil {
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：读不到本件源码（%s）⇒ 源码级牙判不了\n", err.Error())
		return czseRCnoconc
	}
	wp := czseAssertNoWriteAPI(src, czseSelftestMarker)
	if len(wp) > 0 {
		fmt.Fprintf(stdout, "⛔ 判红（rc=1）：本件**真目标段**出现写调用 ⇒ 「不假装」这一格破了：\n")
		for _, h := range wp {
			fmt.Fprintf(stdout, "   · %s\n", h)
		}
		return czseRCred
	}

	var sealed []string
	if len(a.sealed) > 0 {
		for _, p := range a.sealed {
			if czsePyIsabs(p) {
				sealed = append(sealed, p)
			} else {
				sealed = append(sealed, czsePyJoin(czsePyAbspath(a.root), p))
			}
		}
	}
	return czseRun(stdout, a.archive, a.manifest, czseParseAssume(a.assume), sealed, a.workspace, root, a.json, a.now)
}

// ─────────────────────────── 自检（成对负控 · 合成夹具 · 不碰真目标）───────────────────────────

func czseGitEnv(home string) []string {
	return []string{
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"HOME=" + home, "PATH=" + os.Getenv("PATH"),
	}
}

func czseGitRun(repo string, env []string, args ...string) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = env
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	_ = cmd.Run()
}

func czseMkGitRepo(d string) (string, error) {
	repo := czsePyJoin(d, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		return "", err
	}
	env := czseGitEnv(d)
	czseGitRun(repo, env, "init", "-q", "-b", "main")
	if err := os.WriteFile(czsePyJoin(repo, "clean.txt"), []byte("已封件\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(czsePyJoin(repo, "dirty.txt"), []byte("会被改的件\n"), 0o644); err != nil {
		return "", err
	}
	czseGitRun(repo, env, "add", "--", "clean.txt", "dirty.txt")
	czseGitRun(repo, env, "commit", "-qm", "init")
	f, err := os.OpenFile(czsePyJoin(repo, "dirty.txt"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString("改过了\n"); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	return repo, nil
}

func czseMkArchive(d string) (string, string, string, error) {
	arch := czsePyJoin(d, "Zerg-归档")
	sub := czsePyJoin(arch, "批一")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		return "", "", "", err
	}
	x := czsePyJoin(sub, "x.md")
	if err := os.WriteFile(x, []byte("只读件\n"), 0o644); err != nil {
		return "", "", "", err
	}
	os.Chmod(x, 0o444)
	man := czsePyJoin(sub, "归档记录-夹具.md")
	shaOfX, err := czseFileSha(x)
	if err != nil {
		return "", "", "", err
	}
	content := "# 归档记录 · 夹具\n\n| # | 原路径 | 篇数 | 字节 | sha256 | 归档路径 | 搬出日期 | 理由 |\n" +
		"|---|---|---|---|---|---|---|---|\n" +
		"| 1 | `x.md` | 1 | 10 | `" + shaOfX + "` | `Zerg-归档/批一/x.md` | 2026-09-21 | 夹具 |\n"
	if err := os.WriteFile(man, []byte(content), 0o644); err != nil {
		return "", "", "", err
	}
	os.Chmod(man, 0o444)
	return arch, x, man, nil
}

func czseSelfTest(stdout io.Writer) int {
	ok := true
	tmp, err := os.MkdirTemp("", "zse-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "自检结论: **有断言失败**\n")
		return czseRCred
	}
	defer os.RemoveAll(tmp) // 口径差③：原件 A 类泄漏，本件止血

	repo, err := czseMkGitRepo(tmp)
	if err != nil {
		fmt.Fprintf(stdout, "自检结论: **有断言失败**\n")
		return czseRCred
	}
	clean := czsePyJoin(repo, "clean.txt")
	dirty := czsePyJoin(repo, "dirty.txt")
	arch, x, man, err := czseMkArchive(tmp)
	if err != nil {
		fmt.Fprintf(stdout, "自检结论: **有断言失败**\n")
		return czseRCred
	}

	caseFn := func(name, got, want, note string) {
		good := got == want
		ok = ok && good
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（%s · 期望 %s）%s\n", mark, name, got, want, note)
	}

	shaX, _ := czseFileSha(x)

	// ①
	r1 := czseClassifyAndProve(x, arch, map[string]string{"批一/x.md": shaX}, "")
	caseFn("① 正例：只读件 + 清单登记一致 ⇒ rc=0 且基准来源=清单",
		fmt.Sprintf("(%d, %s)", r1.rc, czsePyBool(strings.HasPrefix(czseDeref(r1.baselineFrom), "manifest"))), "(0, True)", "")

	// ②
	bad := strings.Repeat("0", 64)
	r2 := czseClassifyAndProve(x, arch, map[string]string{"批一/x.md": bad}, "")
	caseFn("② 负控：清单登记值 ≠ 现算 ⇒ rc=1（基准不符）", fmt.Sprintf("%d", r2.rc), "1", "  "+czsePySlice(r2.note, 40))

	// ③
	r3 := czseClassifyAndProve(clean, arch, map[string]string{}, "")
	caseFn("③ 正例：仓内已封件 ⇒ 基准来源=HEAD blob · rc=0",
		fmt.Sprintf("(%d, %s)", r3.rc, czsePyBool(strings.HasPrefix(czseDeref(r3.baselineFrom), "head-blob"))), "(0, True)", "")

	// ④
	r4 := czseClassifyAndProve(dirty, arch, map[string]string{}, "")
	caseFn("④ 负控：工作树可变件 ⇒ 无可比基准 · rc=8", fmt.Sprintf("%d", r4.rc), "8", "  "+czsePySlice(r4.note, 44))

	// ⑤
	r5 := czseClassifyAndProve(dirty, arch, map[string]string{}, strings.Repeat("a", 64))
	caseFn("⑤ 负控：给可变件伪造基准 ⇒ rc=1 判红", fmt.Sprintf("%d", r5.rc), "1", "  "+czsePySlice(r5.note, 44))

	// ⑥
	r6 := czseClassifyAndProve(x, arch, map[string]string{"批一/x.md": shaX}, shaX)
	caseFn("⑥ 正例：只读件 + 基准值与现算相同 ⇒ rc=0（复核一致）", fmt.Sprintf("%d", r6.rc), "0", "  "+czsePySlice(r6.note, 30))

	// ⑦
	r7 := czseClassifyAndProve(clean, arch, map[string]string{}, strings.Repeat("b", 64))
	caseFn("⑦ 负控：已封件上给一个错基准 ⇒ rc=1", fmt.Sprintf("%d", r7.rc), "1", "  "+czsePySlice(r7.note, 40))

	// ⑧
	rc8, out8 := czseRunCap(arch, man, map[string]string{}, []string{czsePyJoin(tmp, "没有这个件")}, false, repo, false, "")
	lines8 := czsePyLines(czsePyStrip(out8))
	last8 := ""
	if len(lines8) > 0 {
		last8 = lines8[len(lines8)-1]
	}
	caseFn("⑧ 缺件（--sealed 不在盘）⇒ rc=2 不给结论", fmt.Sprintf("%d", rc8), "2", "  "+czsePySlice(last8, 40))

	// ⑨
	rc9, _ := czseRunCap(arch, czsePyJoin(tmp, "没有这个清单.md"), map[string]string{}, nil, false, repo, false, "")
	caseFn("⑨ 清单件缺 ⇒ rc=2 不给结论", fmt.Sprintf("%d", rc9), "2", "")

	// ⑩
	rc10, out10 := czseRunCap(arch, man, map[string]string{}, []string{x, dirty}, false, repo, false, "")
	proofWord := "无"
	if strings.Contains(out10, "✓ 证明") {
		proofWord = "有"
	}
	caseFn("⑩ 混合：只读件 + 可变件 ⇒ rc=8（不许报 0）", fmt.Sprintf("%d", rc10), "8", "  证明 "+proofWord)

	// ⑪
	rc11, out11 := czseRunCap(arch, "", map[string]string{}, nil, false, repo, false, "")
	detail := "缺"
	if strings.Contains(out11, "✓ 证明") {
		detail = "含 ✓ 证明"
	}
	caseFn("⑪ 自动发现归档记录（表格形态）⇒ rc=0 且表里登记值生效", fmt.Sprintf("%d", rc11), "0", "  明细 "+detail)

	// ⑫（对拍：case 名与 py 逐字同 ⇒ 与 py 两态 stdout 逐字对齐；合成输入仍是 Go 等价物）
	synth := "package main\n\nimport \"os\"\n\nfunc czseSynth(p string) error {\n	return os.Remove(p)\n}\n"
	caseFn("⑫ 负控（源码级牙）：合成源码含 `shutil.rmtree(` ⇒ 命中",
		czsePyBool(len(czseAssertNoWriteAPI(synth, "")) > 0), "True", "")

	src, serr := czseSelfSource("")
	if serr != nil {
		src = ""
	}
	hits := czseAssertNoWriteAPI(src, czseSelftestMarker)
	hnote := ""
	if len(hits) > 0 {
		n := len(hits)
		if n > 2 {
			n = 2
		}
		hnote = "  " + strings.Join(hits[:n], " / ")
	}
	caseFn("⑫b 正例：本件真目标段写调用命中 = 0", fmt.Sprintf("%d", len(hits)), "0", hnote)
	caseFn("⑫c 正例：整篇（含自检段）扫应有命中 ⇒ 扫描器不恒空",
		czsePyBool(len(czseAssertNoWriteAPI(src, "")) > 0), "True", "")

	// ⑬
	for _, flag := range []string{"--chmod", "--seal", "commit", "--fix", "add"} {
		var buf bytes.Buffer
		rc13 := checkZeroSideEffectGateMain(&buf, io.Discard, []string{flag})
		caseFn(fmt.Sprintf("⑬ 负控：写旗标 %s ⇒ 拒 rc=2", pyReprSingle(flag)), fmt.Sprintf("%d", rc13), "2", "")
	}

	if ok {
		fmt.Fprintf(stdout, "自检结论: 全过\n")
		return czseRCok
	}
	fmt.Fprintf(stdout, "自检结论: **有断言失败**\n")
	return czseRCred
}

func pyReprSingle(s string) string {
	if strings.ContainsAny(s, `'\\`) {
		return pyReprStrDouble(s)
	}
	return "'" + s + "'"
}

func czseRunCap(archiveRoot, manifestPath string, assumeMap map[string]string, sealed []string,
	workspace bool, root string, jsonOut bool, now string) (int, string) {
	var buf bytes.Buffer
	rc := czseRun(&buf, archiveRoot, manifestPath, assumeMap, sealed, workspace, root, jsonOut, now)
	return rc, buf.String()
}
