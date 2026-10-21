package main

// check_repo_symlink_farm.go —— 判据件 `scripts/gates/check-repo-symlink-farm.py`（210 行）的 **Go 真搬**
// （v2.5.14 · M2 · 缺口 GAP-20260928-44 第二腿 · 入口 `checkRepoSymlinkFarmGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的口径差异面与未覆盖面）。
//
// 选它 / 不选它的理由（本单口径 · 纯静态只读单件）：
//
//	✓ 选：判据只做 **`$HOME` 浅层符号链接 ⟷ 本仓** 的比对（`os.walk` + `lstat` + `realpath`）——
//	  **不开子进程、不碰网络、不读时刻、不用随机**；主路径**零写面**（`--self-test` 建夹具用等效
//	  `MkdirTemp` + 自回收 ⇒ 唯一写面自回收）。退码 `0/1/2` 三档齐 · 行数适中（210 行）。
//	✗ 不选同批纯静态候选：`check-loose-exit-registry`(221) 用 `ast` 解析 Python ⇒ Go 无对等解析器；
//	  `check-verify-cmd-real`(510) / `check-write-face-gate`(481) 带写面/`atexit`；`check-gap-state-vs-artifact`(569) 更大。
//
// 本单边界（照实）：本件**已接线**（`gateCheckGoEntry["check-repo-symlink-farm"]` 分派 ·
// `precommit-gates.sh:3792` 门104 · tri-report 档）——
//
//	· **不**在 `main.go` 里注册命令（✗ 不进命令树）· **不**删 `scripts/gates/check-repo-symlink-farm.py` 原件
//	  （退场规则：**对拍通过后才**退场 · 逐件点名）；入口 `checkRepoSymlinkFarmGateMain` 已有调用点
//	  （`zerg gate check check-repo-symlink-farm` ⇒ inproc 表分派）。
//	· ✗ 不改登记面 · ✗ 不改任何 `family_*.go` / 已落旧件 / `main.go` / `gate_inproc_table.go` / `precommit-gates.sh` / `scripts/**`。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkRepoSymlinkFarmGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（**不含程序名**）
//	  · 退码  —— 0 浅层无「指向本仓」链接 / 1 命中（逐条打印）/ 2 扫描根读不到 · 自检不过
//	  · 输出  —— stdout 与 py 逐字节对齐（`--json` 走 py `json.dumps(indent=2, ensure_ascii=False)` 同渲染）。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：
//	     py `--repo` 缺省 = `os.environ.get("ZERG_REPO_ROOT") or dirname³(abspath(__file__))`（起于**脚本位置**）。
//	     Go 无 `__file__` 语境 ⇒ `ZERG_REPO_ROOT` 非空则采信，否则回落 **cwd**（✗ 不从可执行体上溯）。
//	     py `--home` 缺省 = `os.environ.get("ZERG_SYM_SCAN_HOME") or expanduser("~")` ⇒ Go 同名同序照搬。
//	     两态对拍均**显式传 `--home`/`--repo`** ⇒ 此差异面不参对拍。
//	(b) **自指检查** —— 本件**不在**任何他门的扫描面内（四形态逐条 · 量化）：
//	     ① 本件 = `core/cmd/zerg/check_repo_symlink_farm.go`；本门自身扫面 = **`$HOME` 浅层**（不是源码树）⇒ 不入。
//	     ② `check-selftest-knobs.py` 的 `core/cmd/zerg/*.go` 面只判「提交链件」（件基本名须逐字出现在
//	        `precommit-gates.sh` 里）⇒ 现读本件名**命中 1**（`:3792` 门104 · 原「命中 0 / 不接线」与现读相反）。
//	     ③ 件头/正文零绝对私有路径字面量（卷名与家目录名一律未出现）⇒ 不触发私有路径门。
//	     ④ 件内不写成错误构造 API 的调用形态连续字面 ⇒ 不触发 error.kind 的 R3 用法面。
//
// 未覆盖面（逐条）：见回执（py `os.path.realpath` 悬空链接的词法回落粒度 · `str.splitlines` 特殊行界 ·
// 非 UTF-8 输入件 · argparse 其余报错档正文 · `--self-test` 内 `TemporaryDirectory` 随机路径）。

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const crsfProg = "check-repo-symlink-farm.py"

const crsfMaxDepth = 2

var crsfSkipDirnames = map[string]bool{
	"Library": true, ".Trash": true, "Applications": true, ".git": true, "node_modules": true,
}

// ── argparse 面（逐字复刻 py 3.9 的用法/帮助/错误正文）──────────────────────────
var crsfUsageText = "usage: " + crsfProg + " [-h] [--home HOME] [--repo REPO] [--json]\n" +
	strings.Repeat(" ", len("usage: ")+len(crsfProg)+1) + "[--self-test]\n"

var crsfHelpText = crsfUsageText + "\n" +
	"家目录「指向本仓的符号链接」只读判据（GAP-20260928-44）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --home HOME\n" +
	"  --repo REPO\n" +
	"  --json       机器面（只印 JSON）\n" +
	"  --self-test\n"

var crsfLongOpts = []string{"--help", "--home", "--repo", "--json", "--self-test"}

// ── py 路径原语（照 CPython 逐段规则自写 · ✗ 不用 filepath.Join/Clean/Abs）──────────
func crsfPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
			continue
		}
		if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// crsfPyNormpath —— CPython posixpath.normpath（★ 保留恰两个前导斜杠；`//a/b` ⇒ `//a/b`、`///a/b` ⇒ `/a/b`）。
func crsfPyNormpath(path string) string {
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
	comps := strings.Split(path, "/")
	var newComps []string
	for _, c := range comps {
		if c == "" || c == "." {
			continue
		}
		if c != ".." || (initial == 0 && len(newComps) == 0) || (len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, c)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	out := strings.Join(newComps, "/")
	if initial > 0 {
		out = strings.Repeat("/", initial) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// crsfPyAbspath —— CPython posixpath.abspath（纯词法：cwd 拼 + normpath）。
func crsfPyAbspath(path string) string {
	if !strings.HasPrefix(path, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "/"
		}
		path = crsfPyJoin(wd, path)
	}
	return crsfPyNormpath(path)
}

// crsfExpanduser —— py os.path.expanduser 的 `~` 分支。
func crsfExpanduser(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				return home
			}
			return crsfPyJoin(home, path[2:])
		}
	}
	return path
}

// crsfRealpath —— py os.path.realpath(strict=False) 的等效：解析得动就解析，解不动回落词法 abspath。
func crsfRealpath(path string) string {
	if r, err := filepath.EvalSymlinks(path); err == nil {
		return r
	}
	return crsfPyAbspath(path)
}

func crsfNorm(path string) string {
	return crsfRealpath(path)
}

// crsfIsWithin —— child 是否在 parent 之内（含相等）：realpath 后按成分前缀比（避免 /a/bc 命中 /a/b）。
func crsfIsWithin(child, parent string) bool {
	child = crsfNorm(child)
	parent = crsfNorm(parent)
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, strings.TrimRight(parent, "/")+"/")
}

func crsfCountSep(path string) int {
	return strings.Count(strings.TrimRight(path, "/"), "/")
}

// crsfJSONStr —— py json.dumps(..., ensure_ascii=False) 的字符串渲染。
func crsfJSONStr(s string) string {
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

type crsfHit struct {
	link   string
	target string
}

// crsfReadDirNames —— 按 readdir 原序取条目名（✗ 不用会排序的 os.ReadDir）。
func crsfReadDirNames(dir string) ([]os.DirEntry, bool) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	ents, err := f.ReadDir(-1)
	if err != nil && len(ents) == 0 {
		return nil, false
	}
	return ents, true
}

// ── 主逻辑 ─────────────────────────────────────────────────────────────
func crsfScan(home, repo string) ([]crsfHit, []string, bool) {
	if st, err := os.Stat(home); err != nil || !st.IsDir() {
		return nil, []string{"扫描根读不到：" + home + " ⇒ 不给结论"}, false
	}
	repoReal := crsfNorm(repo)
	baseDepth := crsfCountSep(home)
	var hits []crsfHit

	var recurse func(dirpath string)
	recurse = func(dirpath string) {
		ents, ok := crsfReadDirNames(dirpath)
		if !ok {
			return
		}
		var dirs, nondirs []string
		for _, e := range ents {
			isDir := false
			if st, err := os.Stat(crsfPyJoin(dirpath, e.Name())); err == nil && st.IsDir() {
				isDir = true
			}
			if isDir {
				dirs = append(dirs, e.Name())
			} else {
				nondirs = append(nondirs, e.Name())
			}
		}
		depth := crsfCountSep(dirpath) - baseDepth
		if depth >= crsfMaxDepth {
			dirs = nil
		} else {
			var kept []string
			for _, d := range dirs {
				if crsfSkipDirnames[d] || strings.HasPrefix(d, ".") {
					continue
				}
				kept = append(kept, d)
			}
			dirs = kept
		}
		merged := append(append([]string{}, nondirs...), dirs...)
		sort.Strings(merged)
		for _, name := range merged {
			p := crsfPyJoin(dirpath, name)
			fi, err := os.Lstat(p)
			if err != nil || fi.Mode()&os.ModeSymlink == 0 {
				continue
			}
			resolved := crsfNorm(p)
			if crsfIsWithin(resolved, repoReal) {
				hits = append(hits, crsfHit{link: p, target: resolved})
			}
		}
		for _, d := range dirs {
			newPath := crsfPyJoin(dirpath, d)
			if fi, err := os.Lstat(newPath); err == nil && fi.Mode()&os.ModeSymlink != 0 {
				continue // followlinks=False：符号链接目录不下钻
			}
			recurse(newPath)
		}
	}
	recurse(home)

	note := fmt.Sprintf("扫描根=%s（浅层 depth≤%d）· 本仓根=%s", home, crsfMaxDepth, repoReal)
	return hits, []string{note}, true
}

func crsfRunCheck(home, repo string) (int, string, []crsfHit, []string, []string) {
	var problems []string
	hits, notes, ok := crsfScan(home, repo)
	if !ok {
		problems = append(problems, "扫描根读不到 ⇒ 不给结论")
		return 2, home, nil, notes, problems
	}
	if len(hits) > 0 {
		problems = append(problems, fmt.Sprintf("家目录浅层存在**指向本仓**的符号链接 %d 条 ⇒ 判红", len(hits)))
		return 1, home, hits, notes, problems
	}
	return 0, home, hits, notes, problems
}

// crsfJSON —— py `json.dumps(rep, ensure_ascii=False, indent=2)` 的逐字节渲染。
func crsfJSON(home, repo string, hits []crsfHit, notes, problems []string) string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString("  \"home\": " + crsfJSONStr(home) + ",\n")
	b.WriteString("  \"repo\": " + crsfJSONStr(repo) + ",\n")
	if len(hits) == 0 {
		b.WriteString("  \"hits\": [],\n")
	} else {
		b.WriteString("  \"hits\": [\n")
		for i, h := range hits {
			b.WriteString("    {\n")
			b.WriteString("      \"link\": " + crsfJSONStr(h.link) + ",\n")
			b.WriteString("      \"target\": " + crsfJSONStr(h.target) + "\n")
			b.WriteString("    }")
			if i < len(hits)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("  ],\n")
	}
	crsfJSONStrArr(&b, "notes", notes)
	b.WriteString(",\n")
	crsfJSONStrArr(&b, "problems", problems)
	b.WriteString("\n}\n")
	return b.String()
}

func crsfJSONStrArr(b *strings.Builder, key string, arr []string) {
	if len(arr) == 0 {
		b.WriteString("  \"" + key + "\": []")
		return
	}
	b.WriteString("  \"" + key + "\": [\n")
	for i, s := range arr {
		b.WriteString("    " + crsfJSONStr(s))
		if i < len(arr)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("  ]")
}

// crsfSelfTest —— py `cmd_self_test()`（5 格 · 夹具自回收）。
func crsfSelfTest(out io.Writer) int {
	ok := true
	tmp, err := os.MkdirTemp("", "crsf-selftest-")
	if err != nil {
		fmt.Fprint(out, "自检 FAIL\n")
		return 2
	}
	defer os.RemoveAll(tmp)
	home := crsfPyJoin(tmp, "home")
	repo := crsfPyJoin(tmp, "repo")
	_ = os.MkdirAll(crsfPyJoin(repo, "core"), 0o755)
	_ = os.MkdirAll(home, 0o755)

	// 正控①：无链接 ⇒ rc=0
	if rc, _, _, _, _ := crsfRunCheck(home, repo); rc != 0 {
		fmt.Fprintf(out, "✗ 正控①：干净家目录却判红 rc=%d\n", rc)
		ok = false
	} else {
		fmt.Fprint(out, "✓ 正控①：干净家目录 ⇒ rc=0\n")
	}

	// 负控①：浅层直挂「指向本仓」的链接 ⇒ rc=1
	link1 := crsfPyJoin(home, "zerg-repo-symlink")
	_ = os.Symlink(repo, link1)
	rc, _, hits, _, _ := crsfRunCheck(home, repo)
	if rc != 1 || len(hits) != 1 {
		fmt.Fprintf(out, "✗ 负控①：指向本仓的链接未被判红 rc=%d hits=%v\n", rc, hits)
		ok = false
	} else {
		fmt.Fprintf(out, "✓ 负控①：指向本仓的链接 ⇒ rc=1（%s）\n", hits[0].link)
	}
	_ = os.Remove(link1)

	// 负控②：指向**别处**的链接 ⇒ 不判红
	outside := crsfPyJoin(tmp, "elsewhere")
	_ = os.MkdirAll(outside, 0o755)
	link2 := crsfPyJoin(home, "elsewhere-link")
	_ = os.Symlink(outside, link2)
	if rc, _, _, _, _ := crsfRunCheck(home, repo); rc != 0 {
		fmt.Fprintf(out, "✗ 负控②：指向别处的链接被误判 rc=%d\n", rc)
		ok = false
	} else {
		fmt.Fprint(out, "✓ 负控②：指向别处的链接 ⇒ 不判红（rc=0）\n")
	}
	_ = os.Remove(link2)

	// 负控③：二层子目录里的「指向本仓」链接 ⇒ 判红
	sub := crsfPyJoin(home, "work")
	_ = os.MkdirAll(sub, 0o755)
	link3 := crsfPyJoin(sub, "repo0")
	_ = os.Symlink(repo, link3)
	if rc, _, _, _, _ := crsfRunCheck(home, repo); rc != 1 {
		fmt.Fprintf(out, "✗ 负控③：二层子目录里的指向本仓链接未被判红 rc=%d\n", rc)
		ok = false
	} else {
		fmt.Fprint(out, "✓ 负控③：二层子目录里的指向本仓链接 ⇒ rc=1\n")
	}
	_ = os.Remove(link3)

	// 负控④：扫描根不存在 ⇒ rc=2
	if rc, _, _, _, _ := crsfRunCheck(crsfPyJoin(tmp, "nope"), repo); rc != 2 {
		fmt.Fprintf(out, "✗ 负控④：扫描根不存在却给了结论 rc=%d\n", rc)
		ok = false
	} else {
		fmt.Fprint(out, "✓ 负控④：扫描根不存在 ⇒ rc=2（不给结论）\n")
	}

	// 负控⑤：/a/bc 成分前缀不得被 /a/b 命中
	if crsfIsWithin(crsfPyJoin(tmp, "abc"), crsfPyJoin(tmp, "ab")) {
		fmt.Fprint(out, "✗ 负控⑤：路径前缀误命中（/abc 命中 /ab）\n")
		ok = false
	} else {
		fmt.Fprint(out, "✓ 负控⑤：成分前缀比正确（/abc ⊄ /ab）\n")
	}
	if ok {
		return 0
	}
	return 2
}

func crsfDefaultRepo() string {
	if v := os.Getenv("ZERG_REPO_ROOT"); v != "" {
		return v
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// crsfMainImpl —— py `main(argv)` + argparse 前缀/错误面。
func crsfMainImpl(argv []string, out, stderr io.Writer) int {
	homeFlag := os.Getenv("ZERG_SYM_SCAN_HOME")
	if homeFlag == "" {
		if h, err := os.UserHomeDir(); err == nil {
			homeFlag = h
		}
	}
	repoFlag := crsfDefaultRepo()
	jsonFlag, selfTest := false, false
	var extra []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			extra = append(extra, argv[i+1:]...)
			break
		}
		if a == "-h" {
			io.WriteString(out, crsfHelpText)
			return 0
		}
		if strings.HasPrefix(a, "--") {
			name := a
			val := ""
			hasEq := false
			if k := strings.IndexByte(a, '='); k >= 0 {
				name = a[:k]
				val = a[k+1:]
				hasEq = true
			}
			var cands []string
			for _, o := range crsfLongOpts {
				if strings.HasPrefix(o, name) {
					cands = append(cands, o)
				}
			}
			if len(cands) == 0 {
				extra = append(extra, a)
				continue
			}
			if len(cands) > 1 {
				fmt.Fprint(stderr, crsfUsageText)
				fmt.Fprintf(stderr, "%s: error: ambiguous option: %s could match %s\n", crsfProg, name, strings.Join(cands, ", "))
				return 2
			}
			takeVal := func(dst *string, opt string) bool {
				if hasEq {
					*dst = val
					return true
				}
				if i+1 >= len(argv) {
					fmt.Fprint(stderr, crsfUsageText)
					fmt.Fprintf(stderr, "%s: error: argument %s: expected one argument\n", crsfProg, opt)
					return false
				}
				i++
				*dst = argv[i]
				return true
			}
			switch cands[0] {
			case "--help":
				io.WriteString(out, crsfHelpText)
				return 0
			case "--home":
				if !takeVal(&homeFlag, "--home") {
					return 2
				}
			case "--repo":
				if !takeVal(&repoFlag, "--repo") {
					return 2
				}
			case "--json":
				jsonFlag = true
			case "--self-test":
				selfTest = true
			}
			continue
		}
		extra = append(extra, a)
	}
	if len(extra) > 0 {
		fmt.Fprint(stderr, crsfUsageText)
		fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", crsfProg, strings.Join(extra, " "))
		return 2
	}
	if selfTest {
		return crsfSelfTest(out)
	}
	home := crsfPyAbspath(crsfExpanduser(homeFlag))
	repo := crsfPyAbspath(repoFlag)
	rc, _, hits, notes, problems := crsfRunCheck(home, repo)
	if jsonFlag {
		io.WriteString(out, crsfJSON(home, repo, hits, notes, problems))
		return rc
	}
	fmt.Fprint(out, "check-repo-symlink-farm —— 家目录「指向本仓的符号链接」只读判据\n")
	for _, n := range notes {
		fmt.Fprint(out, "  · "+n+"\n")
	}
	for _, h := range hits {
		fmt.Fprintf(out, "  ✗ 链接 → 目标：%s → %s\n", h.link, h.target)
		fmt.Fprintf(out, "     建议：删该链接（rm '%s'），或若确需保留则登记为已知用法\n", h.link)
	}
	if len(problems) > 0 {
		fmt.Fprintf(out, "✗ 判红 %d 条：\n", len(problems))
		for _, p := range problems {
			fmt.Fprint(out, "  · "+p+"\n")
		}
		return rc
	}
	fmt.Fprint(out, "✓ 全绿（rc=0）：家目录浅层无「指向本仓」的符号链接\n")
	return rc
}

// checkRepoSymlinkFarmGateMain —— 入口（argv 不含程序名）。stdout 先入 buf、末了 flush（复刻 py 块缓冲）。
func checkRepoSymlinkFarmGateMain(stdout, stderr io.Writer, argv []string) int {
	buf := &bytes.Buffer{}
	rc := crsfMainImpl(argv, buf, stderr)
	stdout.Write(buf.Bytes())
	return rc
}
