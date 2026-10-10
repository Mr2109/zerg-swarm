// check_lineref_census.go —— 门禁族「件行引用语义普查」· **只报告**判据（缺口 GAP-20260927-220 · want①）。
//
// 病（缺口逐字 · 摘要）：注释/文档里的「件:行号」引用会漂 —— 被引件一改行，引用侧照样写着旧行号，
// 而**没有任何一层**会去现读对拍「那一行真的是注释说的那个东西吗」。同一条账另举了「同一份数据里
// 两处注释给出两个互相矛盾的行号」的血证（同仓两处写 72 / 81）。现存门67（check-lineref-limit）只判
// 「行号止 > 被引件物理行数」这一个**上限**，落在界内的漂移照不出来。
//
// 本门判什么：「**带锚点**的件:行号引用」—— 锚点 = 引用紧邻的符号名（注释/文档自己写出的「那个东西」），
// 且**锚点与引用写在同一反引号跨度内**（这一形态的「那个东西」是确定的，✗ 不猜）：
//
//	`<件>:<行>[-<止>] <符号>`     例：`family_code.go:34 codeFindRowCap`
//
// ⇒ 现读被引件的 [行, 止] 区段，判「符号在不在那里」。不在 ⇒ **红**（逐条点名 引用件:行:目标 ＋ 该行原文）。
//
// 口径（逐条）：
//
//	· ★ **只报告**：命中不阻断（接线档 `tri-report`）。rc = 1（有不符 / 悬空）/ 0（全相符）；
//	  扫描面取不到 / 空转 ⇒ rc=2（不给结论 · ✗ 不当绿）。
//	· ★ **只读**：只读仓内件（＋ 可选同级文档仓仅用于**解落点**）；不起子进程 / 不联网 / 不写任何件。
//	· ★ 取不到扫描面真源 ⇒ **不给结论（rc=2）**，✗ 不静默回落（fail-closed）。
//	· ★ 被引件读不到 / 落点解不出 ⇒ **判不了**（逐条点名 · **不计红**）—— 同门67 口径。
//	  ⚠ 例外：落点**看起来是仓内件**（非 `../` / `Zerg-内部文档/` / `项目文档/` 前缀）却解不出 ⇒ **悬空**（红）：
//	  账户里「人照着注释去找、找不到就以为件被删了」正是这一类。
//	· ★★ **覆盖边界（✗ 不假装全量）**：可对拍的前提是注释把「那个东西」**确定地**写出来 ——
//	  故只判「同跨度 锚点 ＋ 件:行号」这一形态。以下**不在本门面**（各自归口，✗ 本门不猜）：
//	    ① 无锚点的裸 `<件>:<行>` 引用 —— 它的「上限」面归门67；
//	    ② 锚点与引用分处两个反引号跨度、周围又有一串反引号 token（如 `a` / `b` / `c`（`件:行`））——
//	       此时「哪个 token 才是那个东西」**不确定**；实测把它也算上会整片误报（同一行上最近的 token
//	       常是列举里的末项而非主体）⇒ 本门**宁可少判、不误判**（判据弱于对象 / 噪声门是更坏的病）。
//	  本门自述面 = 上列唯一形态；分母（带锚点引用数）逐条可读。
//
// ★ 不入 `main.go` 命令树；入 `gate_inproc_table.go` 的键 `check-lineref-census`（进程内路由）。
package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	clrcRCok    = 0
	clrcRCfail  = 1
	clrcRCblock = 2

	clrcMaxBytes = 2_000_000

	// clrcRefExtPat —— 被引件的扩展名面（引用里的 <件> 认作被引件的后缀）。
	clrcRefExtPat = "go|py|sh|md|json|tsv|rs|toml|yaml|yml|txt|jsonl|lock|cfg"
)

// clrcScanExts —— 只扫这些后缀的**引用侧**件（注释/文档面）。
var clrcScanExts = map[string]bool{".go": true, ".py": true, ".sh": true, ".md": true, ".rs": true}

// clrcPruneDirs —— 遍历剪枝集（仓内噪声面；✗ 不当真源）。
var clrcPruneDirs = map[string]bool{
	".git": true, "node_modules": true, "target": true, "vendor": true,
	"_history": true, "dist": true, "venv": true, "site-packages": true,
	".venv": true, "build": true, "__pycache__": true, ".mypy_cache": true,
}

// clrcA1 —— 同跨度形态：`<件>:<行>[-<止>] <符号>`（四个捕获：件 / 行 / 止(可空) / 符号）。
var clrcA1 = regexp.MustCompile("`([^`\\s<>「」（）、]+?\\.(?:" + clrcRefExtPat + ")):(\\d+)(?:-(\\d+))?\\s+([A-Za-z_][A-Za-z0-9_]*)(?:\\(\\))?`")

// clrcRef —— 一处「带锚点的件:行号引用」。
type clrcRef struct {
	hostRel string // 引用侧件（仓内相对路径）
	hostLn  int    // 引用所在行
	text    string // 该行原文（截断）
	tgt     string // 被引件路径（引用里写的原样）
	n       int    // 行号
	hi      int    // 止行号（0 = 单行）
	sym     string // 锚点符号（「那个东西」）
}

// clrcTrimLine —— 只取摘要、限宽。
func clrcTrimLine(s string) string {
	r := []rune(strings.TrimRight(s, " \t\r"))
	if len(r) > 200 {
		r = append(r[:200], '…')
	}
	return string(r)
}

// clrcIsCommentLine —— 只认注释/文档行（✗ 不把代码/数据里出现的引用当判据面）。
func clrcIsCommentLine(rel, line string) bool {
	if strings.EqualFold(filepath.Ext(rel), ".md") {
		return true
	}
	s := strings.TrimSpace(line)
	return strings.HasPrefix(s, "//") || strings.HasPrefix(s, "#") ||
		strings.HasPrefix(s, "*") || strings.HasPrefix(s, "/*")
}

// clrcIndex —— 造「被引件基名 ⇒ 候选路径」索引（一个扫描面）。
func clrcIndex(root string) map[string][]string {
	idx := map[string][]string{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if clrcPruneDirs[d.Name()] && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		if info, e := d.Info(); e == nil && info.Size() > clrcMaxBytes {
			return nil
		}
		idx[d.Name()] = append(idx[d.Name()], p)
		return nil
	})
	for k := range idx {
		sort.Strings(idx[k])
	}
	return idx
}

// clrcScan —— 扫引用侧面，收集带锚点的引用。
func clrcScan(root string) (int, []clrcRef) {
	var refs []clrcRef
	files := 0
	seen := map[string]bool{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if clrcPruneDirs[d.Name()] && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !clrcScanExts[strings.ToLower(filepath.Ext(p))] {
			return nil
		}
		if info, e := d.Info(); e == nil && info.Size() > clrcMaxBytes {
			return nil
		}
		raw, e := os.ReadFile(p)
		if e != nil {
			return nil
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			rel = p
		}
		rel = filepath.ToSlash(rel)
		files++
		for i, ln := range strings.Split(string(raw), "\n") {
			if !clrcIsCommentLine(rel, ln) {
				continue
			}
			for _, m := range clrcA1.FindAllStringSubmatch(ln, -1) {
				key := fmt.Sprintf("%s:%d:%s:%d:%s", rel, i+1, m[1], clrcAtoi(m[2]), m[4])
				if seen[key] {
					continue
				}
				seen[key] = true
				refs = append(refs, clrcRef{rel, i + 1, clrcTrimLine(ln), m[1], clrcAtoi(m[2]), clrcAtoi(m[3]), m[4]})
			}
		}
		return nil
	})
	sort.Slice(refs, func(a, b int) bool {
		if refs[a].hostRel != refs[b].hostRel {
			return refs[a].hostRel < refs[b].hostRel
		}
		return refs[a].hostLn < refs[b].hostLn
	})
	return files, refs
}

func clrcAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// clrcExternalLooking —— 落点看起来是**仓外/文档仓**（解不出时判「判不了」而非「悬空」）。
func clrcExternalLooking(tgt string) bool {
	return strings.HasPrefix(tgt, "../") || strings.HasPrefix(tgt, "Zerg-内部文档/") ||
		strings.HasPrefix(tgt, "承接自-") || strings.Contains(tgt, "项目文档/")
}

// 落点解析态。
const (
	clrcFound = iota
	clrcNotFound
	clrcAmbiguous
)

// clrcResolve —— 解被引件落点：根相对 → 同目录 → 根上一级（覆盖 `../Zerg-内部文档` 与 `Zerg-内部文档/` 两形）
// → 同级文档仓 → 唯基名搜索。多解/无解 ⇒ 判不了（调用侧按「悬空」或「判不了」分类）。
func clrcResolve(root, docsRoot, hostRel, tgt string, idx map[string][]string) (string, int) {
	cands := []string{
		filepath.Join(root, filepath.FromSlash(tgt)),
		filepath.Join(root, filepath.Dir(filepath.FromSlash(hostRel)), filepath.FromSlash(tgt)),
		filepath.Join(root, "..", filepath.FromSlash(tgt)),
	}
	if docsRoot != "" {
		cands = append(cands, filepath.Join(docsRoot, filepath.FromSlash(tgt)))
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && st.Mode().IsRegular() {
			return c, clrcFound
		}
	}
	cs := idx[filepath.Base(filepath.FromSlash(tgt))]
	switch len(cs) {
	case 1:
		return cs[0], clrcFound
	case 0:
		return "", clrcNotFound
	default:
		return "", clrcAmbiguous
	}
}

// clrcVerdict —— 一处引用的判定。
type clrcVerdict struct {
	ref    clrcRef
	status string // OK / DRIFT / OVER / DANGLING / UNJUDGED
}

// clrcRun —— 默认档：现读扫描面 ⇒ 逐条对拍并报告。
func clrcRun(stdout, stderr io.Writer, root, docsRoot string) int {
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		fmt.Fprintf(stdout, "⛔ 不给结论：扫描根取不到/不是目录 —— %s\n", root)
		return clrcRCblock
	}
	files, refs := clrcScan(root)
	if files == 0 {
		fmt.Fprintf(stdout, "⛔ 不给结论：扫描面 0 件（%s 下无 .go/.py/.sh/.md/.rs ⇒ 空转）\n", root)
		return clrcRCblock
	}
	idx := clrcIndex(root)
	if docsRoot != "" {
		for k, v := range clrcIndex(docsRoot) {
			idx[k] = append(idx[k], v...)
		}
	}
	var vs []clrcVerdict
	counts := map[string]int{}
	for _, r := range refs {
		v := clrcVerdict{ref: r}
		path, rs := clrcResolve(root, docsRoot, r.hostRel, r.tgt, idx)
		switch rs {
		case clrcAmbiguous:
			v.status = "UNJUDGED"
		case clrcNotFound:
			if clrcExternalLooking(r.tgt) {
				v.status = "UNJUDGED"
			} else {
				v.status = "DANGLING"
			}
		default:
			raw, e := os.ReadFile(path)
			if e != nil {
				v.status = "UNJUDGED"
				break
			}
			lines := strings.Split(string(raw), "\n")
			hi := r.hi
			if hi == 0 {
				hi = r.n
			}
			if r.n < 1 || hi > len(lines) {
				v.status = "OVER"
				break
			}
			if strings.Contains(strings.Join(lines[r.n-1:hi], "\n"), r.sym) {
				v.status = "OK"
			} else {
				v.status = "DRIFT"
			}
		}
		counts[v.status]++
		vs = append(vs, v)
	}
	red := counts["DRIFT"] + counts["OVER"] + counts["DANGLING"]
	fmt.Fprintf(stdout, "件行引用语义普查（锚点符号 ⟷ 被引行）：扫描 %d 件 · 带锚点引用 %d 处 · "+
		"相符 %d · 漂移 %d · 越界 %d · 悬空 %d · 判不了 %d\n",
		files, len(refs), counts["OK"], counts["DRIFT"], counts["OVER"], counts["DANGLING"], counts["UNJUDGED"])
	what := map[string]string{
		"DRIFT":    "漂移（该行不是那个东西）",
		"OVER":     "越界（行号止 > 被引件行数）",
		"DANGLING": "悬空（仓内落点解不出）",
	}
	for _, v := range vs {
		switch v.status {
		case "DRIFT", "OVER", "DANGLING":
			hi := ""
			if v.ref.hi != 0 {
				hi = fmt.Sprintf("-%d", v.ref.hi)
			}
			fmt.Fprintf(stdout, "✗ %s %s:%d  目标=%s:%d%s  应见符号=%s\n",
				what[v.status], v.ref.hostRel, v.ref.hostLn, v.ref.tgt, v.ref.n, hi, v.ref.sym)
			fmt.Fprintf(stdout, "      行文: %s\n", v.ref.text)
		case "UNJUDGED":
			fmt.Fprintf(stdout, "· 判不了 %s:%d  目标=%s:%d  符号=%s\n",
				v.ref.hostRel, v.ref.hostLn, v.ref.tgt, v.ref.n, v.ref.sym)
		}
	}
	if red == 0 {
		fmt.Fprintf(stdout, "⇒ 绿：%d 处带锚点引用「符号都在被引行上」\n", counts["OK"])
		return clrcRCok
	}
	fmt.Fprintf(stdout, "⇒ 不符 %d 处（★ 只报告档 · tri-report：不阻断提交 · ✗ 本门不改任何被点名件）\n", red)
	return clrcRCfail
}

func clrcWriteFile(root, rel, content string) {
	p := filepath.Join(root, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(content), 0o644)
}

// clrcSelfTest —— 正/负控（**合成树 · 不碰真目标**）。
func clrcSelfTest(stdout, stderr io.Writer) int {
	ok := true
	chk := func(cond bool, okMsg, badMsg string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", okMsg)
		} else {
			fmt.Fprintf(stdout, "   ✗ %s\n", badMsg)
			ok = false
		}
	}
	td, err := os.MkdirTemp("", "clrc-selftest-")
	if err != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td) }()
	// 夹具：被引件 b.go（第 2 行 = func hello）· 引用侧 a.md（带锚点形态）。
	clrcWriteFile(td, "b.go", "package main\nfunc hello() {}\nfunc bye() {}\n")
	clrcWriteFile(td, "a.md", "见 `b.go:2 hello` 处。\n")
	var out1 strings.Builder
	rc1 := clrcRun(&out1, stderr, td, "")
	chk(rc1 == clrcRCok, "正控：锚点符号在被引行上 ⇒ rc=0",
		fmt.Sprintf("正控失败：期望 rc=0，实得 %d（out=%q）", rc1, out1.String()))
	chk(strings.Contains(out1.String(), "相符 1"), "正控：分母可读（1 处带锚点引用）",
		"正控失败：读数里没有「相符 1」")
	// 负控①：行号指向**不是那个东西** ⇒ rc=1，并点名 件:行:目标 ＋ 应见符号。
	clrcWriteFile(td, "a.md", "见 `b.go:3 hello` 处。\n")
	var out2 strings.Builder
	rc2 := clrcRun(&out2, stderr, td, "")
	chk(rc2 == clrcRCfail, "负控①：行号指向不是那个东西 ⇒ rc=1（真红）",
		fmt.Sprintf("负控①失败：期望 rc=1，实得 %d（out=%q）", rc2, out2.String()))
	chk(strings.Contains(out2.String(), "a.md:1") && strings.Contains(out2.String(), "目标=b.go:3") &&
		strings.Contains(out2.String(), "应见符号=hello"),
		"负控①：逐条点名 引用件:行:目标（含应见符号）",
		fmt.Sprintf("负控①失败：点名不齐（out=%q）", out2.String()))
	// 负控②：扫描根取不到 ⇒ rc=2 且明说「不给结论」（✗ 不静默回落）。
	var out3 strings.Builder
	rc3 := clrcRun(&out3, stderr, filepath.Join(td, "does-not-exist"), "")
	chk(rc3 == clrcRCblock && strings.Contains(out3.String(), "不给结论"),
		"负控②：扫描根取不到 ⇒ rc=2 且明说「不给结论」（✗ 不静默回落）",
		fmt.Sprintf("负控②失败：rc=%d out=%q", rc3, out3.String()))
	// 负控③：扫描面 0 件 ⇒ rc=2（空转不给结论）。
	td3, e3 := os.MkdirTemp("", "clrc-selftest-empty-")
	if e3 != nil {
		fmt.Fprint(stdout, "自检跑不了：造不出临时目录\n")
		return 1
	}
	defer func() { _ = os.RemoveAll(td3) }()
	var out4 strings.Builder
	rc4 := clrcRun(&out4, stderr, td3, "")
	chk(rc4 == clrcRCblock, "负控③：扫描面 0 件 ⇒ rc=2（空转不给结论）",
		fmt.Sprintf("负控③失败：期望 rc=2，实得 %d", rc4))
	// 负控④：仓内落点解不出（非外部形）⇒ 悬空（红且点名）。
	clrcWriteFile(td, "a.md", "见 `no-such-file.go:1 hello` 处。\n")
	var out5 strings.Builder
	rc5 := clrcRun(&out5, stderr, td, "")
	chk(rc5 == clrcRCfail && strings.Contains(out5.String(), "悬空"),
		"负控④：仓内落点解不出 ⇒ 悬空（红 · 点名）",
		fmt.Sprintf("负控④失败：rc=%d out=%q", rc5, out5.String()))

	res := "FAIL"
	if ok {
		res = "PASS"
	}
	fmt.Fprintf(stdout, "自检 %s（7 格）\n", res)
	if ok {
		return 0
	}
	return 1
}

const clrcUsage = "usage: check-lineref-census [--root ROOT] [--docs-root DOCS_ROOT] [--self-test]\n"

// checkLinerefCensusGateMain —— 进程内入口（`gate_inproc_table.go` 键 `check-lineref-census`）。
func checkLinerefCensusGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ""
	docsRoot := ""
	hasRoot := false
	selfTest := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, clrcUsage)
			return clrcRCok
		case a == "--self-test":
			selfTest = true
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, clrcUsage)
				return clrcRCblock
			}
			i++
			root = argv[i]
			hasRoot = true
		case strings.HasPrefix(a, "--root="):
			root = strings.TrimPrefix(a, "--root=")
			hasRoot = true
		case a == "--docs-root":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, clrcUsage)
				return clrcRCblock
			}
			i++
			docsRoot = argv[i]
		case strings.HasPrefix(a, "--docs-root="):
			docsRoot = strings.TrimPrefix(a, "--docs-root=")
		default:
			fmt.Fprint(stderr, clrcUsage)
			fmt.Fprintf(stderr, "check-lineref-census: error: unrecognized arguments: %s\n", a)
			return clrcRCblock
		}
	}
	if selfTest {
		return clrcSelfTest(stdout, stderr)
	}
	if !hasRoot {
		if v, ok := os.LookupEnv("ZERG_ROOT"); ok {
			root = v
		} else {
			wd, err := os.Getwd()
			if err != nil {
				wd = "."
			}
			root = wd
		}
	}
	if docsRoot == "" {
		cand := filepath.Join(root, "..", "Zerg-内部文档")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			docsRoot = cand
		}
	}
	return clrcRun(stdout, stderr, root, docsRoot)
}
