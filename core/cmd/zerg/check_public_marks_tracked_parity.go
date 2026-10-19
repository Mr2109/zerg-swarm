// check_public_marks_tracked_parity.go —— 判据件 `scripts/gates/check-public-marks-tracked-parity.py`（157 行）的 **Go 真搬**
// （M2 一枚 · 公开标记族「跟踪状态对拍腿」· 入口 `checkPublicMarksTrackedParityGateMain` · 承接 GAP-20260927-408）。
//
// 本件只做一件事：把 `scripts/公开标记.tsv` 里「理由」列声称「未跟踪」的行，用
// `git ls-files --error-unmatch` 现读对拍；表文说「未跟踪」而现读已跟踪 ⇒ rc=1 逐行点名。
// 退码：0 全对拍 · 1 有过期行 · 2 不给结论（表不在盘 / 读不了 / 空转 / err）。
//
// 声明 (a) 默认根定位差异：py 本件 **零 `__file__`** 语境 —— `--root` 缺省 `"."` ⇒ `os.path.abspath(".")`
//
//	== cwd；本件同样取 cwd（`os.Getwd()` + 同构 normpath）⇒ 两态同 cwd 即 **逐字同构**，本条差异**不适用**。
//
// 声明 (b) 自指：本判据扫描面 = `scripts/公开标记.tsv` 单件（只读），本件落 `core/cmd/zerg/`
//
//	⇒ 不落在自身扫描面内；件内无私有路径字面量、无解析后会悬空的文档树字面（改串拼/描述式）。
//
// 子进程面（★ 制品/子进程类判据 · 现行口径）：唯一子进程 = `git -C <root> ls-files --error-unmatch <path>`
//
//	—— **确定读面**（只读 · 输入可锁 · 输出不随被判对象行为变）⇒ 適格成立，✗ 不是「真跑被判代码」；
//	自检夹具同样是**确定夹具基建**（`git init/add/commit` 建仓外临时仓、跑完 `RemoveAll`）。
//
// 遍历/排序点：judge 逐行 `enumerate(lines,1)` 表序（py `splitlines()` 原序）⇒ 零排序、零 map 迭代进输出。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

const (
	cpmtTableRel = pathMarksTable
	cpmtProg     = "check-public-marks-tracked-parity.py"
)

// cpmtClaim —— 「理由」列里声称未跟踪的标记（py 常量 CLAIM）。★ 用 `var` 以便测试件哑化留牙。
var cpmtClaim = "未跟踪"

const cpmtUsage = "usage: check-public-marks-tracked-parity.py [-h] [--root ROOT] [--table TABLE]\n" +
	"                                            [--self-test]\n"

const cpmtHelpBody = "\noptional arguments:\n" +
	"  -h, --help     show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --table TABLE\n" +
	"  --self-test\n"

type cpmtStale struct {
	Line int
	Path string
}

// ── py 路径语义（自写同构 · ✗ 不用 filepath.Join/Clean/Split）──────────────────
func cpmtPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
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

// cpmtPyNormpath —— 照 CPython posixpath.normpath（★ 保留恰两个前导斜杠 `//a`）。
func cpmtPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initial = 2
		}
	}
	out := []string{}
	for _, c := range strings.Split(path, "/") {
		if c == "" || c == "." {
			continue
		}
		if c != ".." || (initial == 0 && len(out) == 0) || (len(out) > 0 && out[len(out)-1] == "..") {
			out = append(out, c)
		} else if len(out) > 0 {
			out = out[:len(out)-1]
		}
	}
	p := strings.Join(out, "/")
	if initial > 0 {
		p = strings.Repeat("/", initial) + p
	}
	if p == "" {
		return "."
	}
	return p
}

// cpmtPyAbspath —— os.path.abspath = normpath(join(cwd,p))（纯词法·不解析符号链接）。
func cpmtPyAbspath(p, cwd string) string {
	if !strings.HasPrefix(p, "/") {
		p = cpmtPyJoin(cwd, p)
	}
	return cpmtPyNormpath(p)
}

// cpmtPyDirname —— posixpath.dirname（= split()[0]）。
func cpmtPyDirname(p string) string {
	i := strings.LastIndexByte(p, '/') + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

func cpmtPyIsLineSep(r rune) bool {
	switch r {
	case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\u0085', '\u2028', '\u2029':
		return true
	}
	return false
}

// cpmtPySplitlines —— str.splitlines()（输入已经过 universal-newline 折叠）。
func cpmtPySplitlines(s string) []string {
	out := []string{}
	cur := []rune{}
	for _, r := range s {
		if cpmtPyIsLineSep(r) {
			out = append(out, string(cur))
			cur = cur[:0]
		} else {
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// cpmtPySpace —— str.isspace() 口径（含 \x1c-\x1f；Go unicode.IsSpace 不剥这几个）。
func cpmtPySpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// cpmtPyStrip —— str.strip() 口径（自写，✗ 不用 strings.TrimSpace）。
func cpmtPyStrip(s string) string {
	return strings.TrimFunc(s, cpmtPySpace)
}

// cpmtPyReadLines —— open(path, encoding="utf-8").read().splitlines()：
// 先按 universal-newline 折叠 \r\n / \r → \n，再 splitlines。
func cpmtPyReadLines(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return cpmtPySplitlines(s), nil
}

// cpmtGitTracked —— `git -C root ls-files --error-unmatch path` rc==0（确定读面）。
func cpmtGitTracked(root, path string) bool {
	cmd := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", path)
	return cmd.Run() == nil
}

// cpmtJudge —— 对拍判定（自检可直接调）。⇒ (rc, stale, checked, reason)。
func cpmtJudge(root, tableRel string) (int, []cpmtStale, int, string) {
	tpath := cpmtPyJoin(root, tableRel)
	st, err := os.Stat(tpath)
	if err != nil || st.IsDir() {
		return 2, nil, 0, "table-absent"
	}
	lines, rerr := cpmtPyReadLines(tpath)
	if rerr != nil {
		return 2, nil, 0, "table-unreadable"
	}
	stale := []cpmtStale{}
	checked := 0
	for i, l := range lines {
		cols := strings.Split(l, "\t")
		if len(cols) < 3 {
			continue
		}
		f := cpmtPyStrip(cols[0])
		reason := cols[2]
		if !strings.Contains(reason, cpmtClaim) {
			continue
		}
		checked++
		if cpmtGitTracked(root, f) {
			stale = append(stale, cpmtStale{Line: i + 1, Path: f})
		}
	}
	if checked == 0 {
		return 2, nil, 0, "empty-domain"
	}
	if len(stale) > 0 {
		return 1, stale, checked, "stale"
	}
	return 0, nil, checked, "ok"
}

// ── 自检（合成 git 夹具 · 一律落仓外临时树 · 成对正负控）────────────────────────
func cpmtRepr(v interface{}) string {
	switch x := v.(type) {
	case int:
		return strconv.Itoa(x)
	case string:
		return "'" + x + "'"
	case []cpmtStale:
		parts := []string{}
		for _, s := range x {
			parts = append(parts, fmt.Sprintf("(%d, '%s')", s.Line, s.Path))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return fmt.Sprintf("%v", v)
}

func cpmtRunGit(dir string, args ...string) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	_ = cmd.Run()
}

func cpmtMkGitRepo(tmp string) string {
	r := cpmtPyJoin(tmp, "repo")
	_ = os.MkdirAll(r, 0o755)
	_ = os.WriteFile(cpmtPyJoin(r, "tracked.go"), []byte("package x\n"), 0o644)
	cpmtRunGit(r, "init", "-q")
	cpmtRunGit(r, "-c", "user.email=s@x", "-c", "user.name=s", "add", "tracked.go")
	cpmtRunGit(r, "-c", "user.email=s@x", "-c", "user.name=s", "commit", "-qm", "x")
	_ = os.WriteFile(cpmtPyJoin(r, "untracked.go"), []byte("package x\n"), 0o644)
	return r
}

func cpmtWriteTable(repo, rel string, rows [][2]string) {
	p := cpmtPyJoin(repo, rel)
	d := cpmtPyDirname(p)
	if d != "" && d != "." {
		_ = os.MkdirAll(d, 0o755)
	}
	var b strings.Builder
	for _, row := range rows {
		b.WriteString(row[0] + "\tscripts\t" + row[1] + "\n")
	}
	_ = os.WriteFile(p, []byte(b.String()), 0o644)
}

func cpmtSelftest(stdout io.Writer) int {
	total, bad := 0, 0
	assertEq := func(name string, got, want interface{}) {
		total++
		if !reflect.DeepEqual(got, want) {
			bad++
			fmt.Fprintf(stdout, "  ✗ 断言失败：%s —— 得到 %s，期望 %s\n", name, cpmtRepr(got), cpmtRepr(want))
		}
	}
	fmt.Fprint(stdout, "公开标记跟踪对拍 · 自检（合成 git 夹具 · 临时树 · 成对正负控）\n")
	tmp, err := os.MkdirTemp("", "pmtp-selftest-")
	if err != nil {
		fmt.Fprint(stdout, "── 自检结论：断言 0 条 · 失败 0 条 ──\n")
		return 1
	}
	defer os.RemoveAll(tmp) // 安全删点：仅在 MkdirTemp 成功此支挂 defer
	repo := cpmtMkGitRepo(tmp)
	rel := cpmtTableRel
	// ① 正控：声明「未跟踪」↔ 现读真未跟踪 ⇒ rc=0
	cpmtWriteTable(repo, rel, [][2]string{{"untracked.go", "本件现读未跟踪"}})
	rc, _, checked, _ := cpmtJudge(repo, rel)
	assertEq("① 正控：未跟踪声明与现读一致 ⇒ rc=0", rc, 0)
	assertEq("① 正控：恰对拍 1 行", checked, 1)
	// ② 负控：声明「未跟踪」而现读已跟踪 ⇒ rc=1 且点名 件:行
	cpmtWriteTable(repo, rel, [][2]string{{"tracked.go", "本件现读未跟踪"}})
	rc, stale, _, _ := cpmtJudge(repo, rel)
	assertEq("② 负控：声明未跟踪↔现读已跟踪 ⇒ rc=1", rc, 1)
	assertEq("② 负控：点名恰 1 行且两侧读数对", stale, []cpmtStale{{Line: 1, Path: "tracked.go"}})
	// ③ 负控（换回）：把②改回真未跟踪 ⇒ 又绿
	cpmtWriteTable(repo, rel, [][2]string{{"untracked.go", "本件现读未跟踪"}})
	assertEq("③ 负控（换回）：⇒ rc=0", cpmtJudgeRc(repo, rel), 0)
	// ④ 空转负控：表内 0 行「未跟踪」⇒ rc=2（假覆盖）
	cpmtWriteTable(repo, rel, [][2]string{{"tracked.go", "本件现读已跟踪（不进对拍面）"}})
	assertEq("④ 空转负控：0 行未跟踪声明 ⇒ rc=2", cpmtJudgeRc(repo, rel), 2)
	// ⑤ 缺件负控：表不在盘 ⇒ rc=2
	assertEq("⑤ 缺件负控：表不在盘 ⇒ rc=2", cpmtJudgeRc(repo, "scripts/没有这一份.tsv"), 2)
	fmt.Fprintf(stdout, "── 自检结论：断言 %d 条 · 失败 %d 条 ──\n", total, bad)
	if bad == 0 {
		return 0
	}
	return 1
}

func cpmtJudgeRc(root, tableRel string) int {
	rc, _, _, _ := cpmtJudge(root, tableRel)
	return rc
}

// checkPublicMarksTrackedParityGateMain = 本判据的 Go 入口（argv 不含程序名；stdout/stderr 由调用方给）。
func checkPublicMarksTrackedParityGateMain(stdout, stderr io.Writer, argv []string) int {
	root := "."
	table := cpmtTableRel
	selftest := false
	i := 0
	errf := func(msg string) int {
		fmt.Fprint(stderr, cpmtUsage)
		fmt.Fprintf(stderr, "%s: error: %s\n", cpmtProg, msg)
		return 2
	}
	for i < len(argv) {
		a := argv[i]
		val := func() (string, bool) {
			if eq := strings.IndexByte(a, '='); eq >= 0 {
				return a[eq+1:], true
			}
			if i+1 < len(argv) {
				i++
				return argv[i], true
			}
			return "", false
		}
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, cpmtUsage)
			fmt.Fprint(stdout, cpmtHelpBody)
			return 0
		case a == "--root" || strings.HasPrefix(a, "--root="):
			v, ok := val()
			if !ok {
				return errf("argument --root: expected one argument")
			}
			root = v
		case a == "--table" || strings.HasPrefix(a, "--table="):
			v, ok := val()
			if !ok {
				return errf("argument --table: expected one argument")
			}
			table = v
		case a == "--self-test":
			selftest = true
		default:
			return errf("unrecognized arguments: " + a)
		}
		i++
	}
	if selftest {
		return cpmtSelftest(stdout)
	}
	cwd, _ := os.Getwd()
	rootAbs := cpmtPyAbspath(root, cwd)
	_, stale, checked, reason := cpmtJudge(rootAbs, table)
	switch reason {
	case "table-absent", "table-unreadable":
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：表不在盘 / 读不了 %s\n", table)
		return 2
	case "empty-domain":
		fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：表内 0 行声称「%s」（空转 = 假覆盖）\n", cpmtClaim)
		return 2
	case "stale":
		fmt.Fprintf(stderr, "✗ 过期登记 %d 行（表文说「%s」· `git ls-files` 现读为**已跟踪**）：\n", len(stale), cpmtClaim)
		for _, s := range stale {
			fmt.Fprintf(stderr, "  %s:%d  %s\n", table, s.Line, s.Path)
		}
		fmt.Fprint(stdout, "判据：时点读数必须与现读对拍 —— 表文写「未跟踪」而现读已跟踪 ⇒ 判不了 ⇒ 不给结论（退 1）\n")
		return 1
	}
	fmt.Fprintf(stdout, "✓ 对拍通过：%d 行「%s」声明与 `git ls-files` 现读一致\n", checked, cpmtClaim)
	return 0
}
