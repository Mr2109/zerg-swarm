// isolate_ro.go —— 件 scripts/dev/isolate-ro.sh（507 行 · **只读隔离判据** · write D2）的 Go **真搬**。
//
// 为什么有这件（缺口 GAP-20260926-20 P0 · 根因「在别处试」没有不会误删真仓的路）：
//
//	事故形态：一枚卵为「隔离验证」自建临时目录 + 符号链接农场，其中一条
//	`ln -s <真仓>/core/cmd/zerg/testdata <农场>/…/testdata`，随后删农场里的件
//	—— 删掉的其实是真仓的件（符号链接把删除动作透传回去了）。
//	⇒ 规矩只有一条：「在别处试」= 硬拷贝出一棵独立工作树；真仓只读、绝不建链接、绝不删。
//
// 本件把原件的**实际判定行为**（三个子命令 new / verify / --self-test 的逐条断言、
// 输出文案、退码 0/1/2）搬进 Go，**不再 exec 脚本**（✗ 不调 `bash scripts/dev/isolate-ro.sh`）。
// 拷贝引擎亦在 Go 内等价承载（原生递归拷贝 + rsync 过滤器语义的等价匹配），
// 不依赖系统 rsync 的实际传输（仅在「缺 rsync」那一档保留原件的 fail-closed 行为文案）。
//
// 入口签名（与原件 `$@` 对齐 · inproc 口径 argv **不含程序名**）：
//
//	func isolateRoGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与原件位置参数同形（动作 new|verify + 位置参数 + 旗标）
//	  · 退码  —— 0 正常 · 1 危险形态（拒绝的符号链接 / 对拍有差异）· 2 不给结论（用法错/缺件/空转/自检不过）
//	  · 输出  —— stdout/stderr 与原件逐字对齐（除已声明的非确定面）
//
// 出厂声明 (a) 默认根定位差异：原件**不推导仓根**（三个子命令都吃显式路径实参）⇒ 无仓根差异面。
// 仅 `--self-test` 与 `new` 会在 `${TMPDIR:-/tmp}` 下建临时树（两侧同源：`os.TempDir()`）。
//
// 出厂声明 (b) 自指检查：本件落在 `core/cmd/zerg/*.go` 全件扫描面内 ⇒ 逐条消解：
//
//	· 零私有绝对路径字面量（只写 `scripts/dev/isolate-ro.sh` 这类仓根相对串）；
//	· 零连续「引号 + 键名 + 引号」原串 / 零错误登记函数的括弧引号形态（件头与正文一律描述式）；
//	· 零命令树层级字面量（本件不进命令树 · 入口只经 inproc 表 gateCheckGoEntry 分派）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// isoDefaultName —— 原件 DEFAULT_NAME（临时树名前缀）。
const isoDefaultName = "isolate-ro"

// isoScriptRel —— 原件自身的仓根相对路径（只作提示行文案 · ✗ 不 exec 它）。
const isoScriptRel = "scripts/dev/isolate-ro.sh"

// isoDefaultExcludes —— 原件 DEFAULT_EXCLUDES（缺省排除重件 / 构建产物 / 运行期数据）。
// ★ `ui/target` / `wall/target` 是**相对仓根**的路径（rsync `--exclude=/ui/target` 根锚定）。
var isoDefaultExcludes = []string{
	".git", ".zerg", "bin", "data", "dist", "node_modules", "vendor", "zerg-wt",
	"__pycache__", ".venv", "venv", "htmlcov", ".cargo", "target", "ui/target",
	"wall/target", ".build", "_history",
}

// isoUsage —— 原件 usage()（`sed -n '2,42p' "$SELF" | sed 's/^# \{0,1\}//'` 逐字正文）。
// 原件从**自己的件**截取；本件把该正文**内嵌**（脚本退场后仍可自足 · 内容一字未改）。
const isoUsage = "zerg-class: write D2\nisolate-ro.sh —— **只读隔离**：给「在别处试」一条不会误删真仓的路（根因 `GAP-20260926-20` P0）。\n\n事故形态（本件要防的那一件）：一枚卵为了「隔离验证」自建临时目录 + **符号链接**农场，\n  其中一条 `ln -s <真仓>/core/cmd/zerg/testdata <农场>/…/testdata`，随后\n  `rm -f <农场>/…/cli-matrix.json` —— 删掉的其实是**真仓的件**（符号链接把删除动作透传回去了）。\n⇒ 规矩只有一条：**「在别处试」= 硬拷贝出一棵独立的工作树**；真仓只读、绝不建链接、绝不删。\n\n三个子命令\n--------\n  new    <真仓>   [--dst <目录>] [--include <glob>]… [--only <件路径>]… [--exclude <路径>]… [--all]\n                  [--reject-symlinks] [--no-manifest] [--quiet] [--list]\n      ★ `--only <件路径>`：**单件直取** —— 只取该一个件（父目录骨架 + 清单，别的一件不取）；可多次（`GAP-20260926-47`）。\n      ★ `--list`：**预演** —— 先列会取哪些件 + 件数 + 字节，**一个字节不写**（`GAP-20260926-47` · 零写盘）。\n      在 `${TMPDIR:-/tmp}` 下硬拷贝出一棵**可随意读写**的工作树；缺省排除重件\n      （`.git` / `bin` / `data` / `dist` / `node_modules` / `vendor` / `ui/target` / `wall/target` / `zerg-wt` …）。\n      ★ 缺省排除是**根锚定**写法（rsync `--exclude=/<名>`）⇒ 名单里的**相对路径**（如 `ui/target`）\n        按仓根逐条锚定；脚本开头会**逐行印出生效的排除清单**（清面必须可见 · `GAP-20260927-96`）。\n      ★ 旗标接受 `--exclude <值>` 与 `--exclude=<值>` **两种写法**（同 `--include`/`--dst`/`--max-examples`）。\n      ★ 源路径本身是**符号链接** ⇒ 打印一行说明后**按它解析到的真身硬拷贝**（`--reject-symlinks` ⇒ 直接拒绝）。\n      ★ 副本里任何**指向副本之外**（含指回真仓）的符号链接，一律**换成硬拷贝**（`--reject-symlinks` ⇒ 拒绝并点名）。\n      ★ 拷贝完在副本根写 `ISOLATED.md`（「这是副本，删无妨；真仓是 …」）+ `ISOLATED.manifest.tsv`（逐件 sha256）。\n      输出：真仓路径 · 副本路径 · 件数 · 总字节。\n\n  verify <副本> <真仓> [--quiet] [--max-examples N]\n      逐件 sha256 **对拍副本与真仓**（副本侧优先取 `new` 留下的 manifest，缺则现算）\n      ⇒ 用来**证明这一轮验证过程一个字节都没改到真仓**。rc：0 全一致 · 1 有差异（点名）· 2 判不了。\n\n  self-test\n      `--self-test` 三档：正控（隔离→写副本→对拍 rc=0）· 负控（改副本 ⇒ rc=1；真仓里造回真仓的链接 ⇒ 被拒/被换）\n      · 用法错（缺参数 / 真仓不存在 ⇒ rc=2）。合成夹具全在临时目录，**一件真东西都不碰**。\n\n用法\n----\n    bash scripts/dev/isolate-ro.sh new \"<真仓>\" --include 'core/cmd/zerg/testdata/*'\n    bash scripts/dev/isolate-ro.sh verify \"$TREE\" \"<真仓>\"\n    bash scripts/dev/isolate-ro.sh --self-test\n\n退码：0 正常 · 1 危险形态（拒绝的符号链接 / 对拍有差异）· 2 **不给结论**（用法错 / 缺件 / 空转 / 自检不过）\n\n硬规矩（本件自己也守）\n"

// ── 上下文与小工具 ───────────────────────────────────────────────────────────

// isoCtx 承载一次调用的 IO 与临时基目录（自检时替换为合成夹具）。
type isoCtx struct {
	stdout  io.Writer
	stderr  io.Writer
	tmpBase string // ${TMPDIR:-/tmp} 去尾斜杠
}

func (c *isoCtx) say(s string)  { fmt.Fprintf(c.stdout, "%s\n", s) }
func (c *isoCtx) warn(s string) { fmt.Fprintf(c.stderr, "%s\n", s) }

// die 照原件 die()：stderr 逐字一行，退 rc。
func (c *isoCtx) die(rc int, s string) int { c.warn(s); return rc }

func isoNewCtx(stdout, stderr io.Writer) *isoCtx {
	base := os.TempDir()
	base = strings.TrimSuffix(base, "/")
	return &isoCtx{stdout: stdout, stderr: stderr, tmpBase: base}
}

// isoAbsPath 照 abs_path()：绝对路径原样；相对路径拼 `$(pwd)/`。
func isoAbsPath(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}
	return wd + "/" + p
}

// isoRealpath 照 python `os.path.realpath(path)`（strict=False）：逐段解析符号链接，
// 不存在的尾巴按**词法**保留（✗ 不因悬空/未挂载而报错 —— 真仓常有注销卷）。
func isoRealpath(path string) string {
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = isoAbsPath(path)
	}
	out := "" // 已解析的前缀（"" = 根）
	pending := strings.Split(path, "/")
	seen := 0
	for len(pending) > 0 && seen < 64 {
		comp := pending[0]
		pending = pending[1:]
		if comp == "" || comp == "." {
			continue
		}
		if comp == ".." {
			if out != "" {
				if i := strings.LastIndexByte(out, '/'); i >= 0 {
					out = out[:i]
				} else {
					out = ""
				}
			}
			continue
		}
		trial := out + "/" + comp
		fi, err := os.Lstat(trial)
		if err == nil && fi.Mode()&os.ModeSymlink != 0 {
			seen++
			tgt, rerr := os.Readlink(trial)
			if rerr != nil {
				break
			}
			var head []string
			if strings.HasPrefix(tgt, "/") {
				out = ""
				head = strings.Split(tgt, "/")
			} else {
				head = strings.Split(tgt, "/")
			}
			rest := append([]string{}, head...)
			rest = append(rest, pending...)
			pending = rest
			continue
		}
		out = trial
	}
	if out == "" {
		return "/"
	}
	return out
}

// isoHashOf 照 hash_of()：文件字节的 sha256 小写十六进制；读不到 ⇒ ""。
func isoHashOf(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// isoIsDir / isoIsFile 照 `[ -d ]` / `[ -f ]`（★ bash test 内置**跟随符号链接** ⇒ 用 Stat）；
// isoIsLink 照 `[ -L ]`（✗ 不跟随 ⇒ 用 Lstat）。
func isoIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
func isoIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}
func isoIsLink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

// isoFindFileRels 照 `find "$1" -type f ! -name 'ISOLATED.md' ! -name 'ISOLATED.manifest.tsv'`：
// **readdir 原序 + 遇目录即下钻**（find 的产出序），只收**普通文件**（✗ 符号链接不算）。
func isoFindFileRels(root string, skipMeta bool) []string {
	var out []string
	var rec func(dirAbs, rel string)
	rec = func(dirAbs, rel string) {
		f, err := os.Open(dirAbs)
		if err != nil {
			return
		}
		ents, rerr := f.Readdir(-1)
		_ = f.Close()
		if rerr != nil {
			return
		}
		for _, fi := range ents {
			name := fi.Name()
			rc := name
			if rel != "" {
				rc = rel + "/" + name
			}
			if fi.IsDir() {
				rec(dirAbs+"/"+name, rc)
				continue
			}
			if !fi.Mode().IsRegular() {
				continue
			}
			if skipMeta && (name == "ISOLATED.md" || name == "ISOLATED.manifest.tsv") {
				continue
			}
			out = append(out, rc)
		}
	}
	rec(root, "")
	return out
}

// isoCountFiles 照 count_files()（`find "$1" -type f | wc -l`）。
func isoCountFiles(root string) int { return len(isoFindFileRels(root, false)) }

// isoTotalBytes 照 total_bytes()（`find "$1" -type f -exec stat -f %z … | awk 求和`）。
func isoTotalBytes(root string) int64 {
	var sum int64
	for _, rel := range isoFindFileRels(root, false) {
		if fi, err := os.Stat(root + "/" + rel); err == nil {
			sum += fi.Size()
		}
	}
	return sum
}

// isoHumanBytes 照 human_bytes()（awk：split "B KB MB GB TB"; while b>=1024 …; printf "%.1f %s"）。
func isoHumanBytes(b int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(b)
	i := 0
	for f >= 1024 && i < 4 {
		f /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

// ── rsync 过滤器：构造 + 匹配 ────────────────────────────────────────────────

// isoBuildFilterLines 照 build_filters()：打印 rsync 过滤器行（顺序即优先级）。
func (o *isoOpts) isoBuildFilterLines() []string {
	var out []string
	if len(o.includes) == 0 {
		for _, e := range isoDefaultExcludes {
			out = append(out, "--exclude=/"+e)
		}
		for _, e := range o.userExcludes {
			out = append(out, "--exclude="+e)
		}
		return out
	}
	for _, g := range o.includes {
		p := g
		if i := strings.IndexAny(p, "*?"); i >= 0 {
			p = p[:i]
		}
		p = strings.TrimSuffix(p, "/")
		acc := ""
		for _, d := range strings.Split(p, "/") {
			if d == "" {
				continue
			}
			if acc == "" {
				acc = d
			} else {
				acc = acc + "/" + d
			}
			out = append(out, "--include=/"+acc+"/")
		}
		out = append(out, "--include=/"+g)
		out = append(out, "--include=/"+g+"/***")
	}
	if !o.all {
		for _, e := range isoDefaultExcludes {
			out = append(out, "--exclude=/"+e)
		}
	}
	for _, e := range o.userExcludes {
		out = append(out, "--exclude="+e)
	}
	out = append(out, "--exclude=*")
	return out
}

// isoFilter —— 一条过滤器规则（解析自 `--include=` / `--exclude=` 行）。
type isoFilter struct {
	include bool
	pat     string
}

// isoParseFilters 把过滤器行解析成规则表（顺序即优先级）。
func isoParseFilters(lines []string) []isoFilter {
	var out []isoFilter
	for _, ln := range lines {
		if strings.HasPrefix(ln, "--include=") {
			out = append(out, isoFilter{include: true, pat: ln[len("--include="):]})
		} else if strings.HasPrefix(ln, "--exclude=") {
			out = append(out, isoFilter{include: false, pat: ln[len("--exclude="):]})
		}
	}
	return out
}

// isoDecide 照 rsync「首条匹配即生效」语义：命中 include ⇒ 收；命中 exclude ⇒ 弃；
// 无规则命中 ⇒ 收（★ 有 include 时结尾有 `--exclude=*` 兜底 ⇒ 无命中即弃）。
func isoDecide(fs []isoFilter, rel string, isDir bool) bool {
	for _, f := range fs {
		if f.matches(rel, isDir) {
			return f.include
		}
	}
	return true
}

func (f isoFilter) matches(rel string, isDir bool) bool {
	pat := f.pat
	if strings.HasSuffix(pat, "/") {
		if !isDir {
			return false // 尾 `/` ⇒ 只匹配目录
		}
		pat = pat[:len(pat)-1]
	}
	if strings.HasPrefix(pat, "/") {
		return isoMatch(pat, "/"+rel) // 前导 `/` ⇒ 根锚定
	}
	if strings.Contains(pat, "/") {
		return isoMatch(pat, "/"+rel) // 含 `/` ⇒ 对整条路径匹配
	}
	base := rel
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		base = rel[i+1:]
	}
	return isoMatch(pat, base) // 无 `/` ⇒ 只对末段匹配
}

// isoMatch —— rsync 通配匹配：`*` 不跨 `/`；`**`（含 `***`）跨 `/`；`?` = 单个非 `/`；`[...]` 字符类。
func isoMatch(p, s string) bool {
	for len(p) > 0 {
		switch p[0] {
		case '*':
			star := 0
			for len(p) > 0 && p[0] == '*' {
				star++
				p = p[1:]
			}
			cross := star >= 2
			if len(p) == 0 {
				if cross {
					return true
				}
				return !strings.ContainsRune(s, '/')
			}
			if cross {
				for i := 0; i <= len(s); i++ {
					if isoMatch(p, s[i:]) {
						return true
					}
				}
				return false
			}
			for i := 0; ; i++ {
				if isoMatch(p, s[i:]) {
					return true
				}
				if i >= len(s) || s[i] == '/' {
					return false
				}
			}
		case '?':
			if len(s) == 0 || s[0] == '/' {
				return false
			}
			p, s = p[1:], s[1:]
		case '[':
			adv, ok := isoMatchClass(p, s)
			if !ok {
				return false
			}
			p, s = p[adv:], s[1:]
		case '\\':
			if len(p) >= 2 {
				if len(s) == 0 || s[0] != p[1] {
					return false
				}
				p, s = p[2:], s[1:]
			} else {
				if len(s) == 0 || s[0] != '\\' {
					return false
				}
				p, s = p[1:], s[1:]
			}
		default:
			if len(s) == 0 || p[0] != s[0] {
				return false
			}
			p, s = p[1:], s[1:]
		}
	}
	return len(s) == 0
}

// isoMatchClass 处理 `[...]` 字符类：返回 (消耗的 p 长度, 是否命中 s[0])。
func isoMatchClass(p, s string) (int, bool) {
	if len(s) == 0 || s[0] == '/' {
		return 0, false
	}
	i := 1
	neg := false
	if i < len(p) && (p[i] == '!' || p[i] == '^') {
		neg = true
		i++
	}
	matched := false
	first := true
	for i < len(p) && (p[i] != ']' || first) {
		first = false
		if i+2 < len(p) && p[i+1] == '-' && p[i+2] != ']' {
			lo, hi := p[i], p[i+2]
			if lo > hi {
				lo, hi = hi, lo
			}
			if s[0] >= lo && s[0] <= hi {
				matched = true
			}
			i += 3
			continue
		}
		if p[i] == s[0] {
			matched = true
		}
		i++
	}
	if i >= len(p) {
		return 0, false // 未闭合 ⇒ 不当字符类
	}
	return i + 1, matched != neg
}

// ── 遍历（rsync 产出序：本目录文件按名排序 → 再逐个下钻子目录按名排序）─────────

// isoEntry —— 待传输的一格（rsync flist 的一行）。
type isoEntry struct {
	rel    string
	abs    string
	isDir  bool
	isLink bool
	mode   os.FileMode
	size   int64
}

// isoWalk 按 rsync 产出序收集：本目录**文件**按名排序先出，再**子目录**按名排序逐个「目录项 ＋ 下钻」。
func isoWalk(dirAbs, rel string, fs []isoFilter, dirs, files *[]isoEntry) error {
	f, err := os.Open(dirAbs)
	if err != nil {
		return err
	}
	ents, rerr := f.Readdir(-1)
	_ = f.Close()
	if rerr != nil {
		return rerr
	}
	var chFiles, chDirs []isoEntry
	for _, fi := range ents {
		name := fi.Name()
		rc := name
		if rel != "" {
			rc = rel + "/" + name
		}
		isDir := fi.IsDir()
		if !isoDecide(fs, rc, isDir) {
			continue
		}
		e := isoEntry{rel: rc, abs: dirAbs + "/" + name, isDir: isDir,
			isLink: fi.Mode()&os.ModeSymlink != 0, mode: fi.Mode(), size: fi.Size()}
		if isDir {
			chDirs = append(chDirs, e)
		} else {
			chFiles = append(chFiles, e)
		}
	}
	sort.Slice(chFiles, func(i, j int) bool { return chFiles[i].rel < chFiles[j].rel })
	sort.Slice(chDirs, func(i, j int) bool { return chDirs[i].rel < chDirs[j].rel })
	*files = append(*files, chFiles...)
	for _, d := range chDirs {
		*dirs = append(*dirs, d)
		if err := isoWalk(d.abs, d.rel, fs, dirs, files); err != nil {
			return err
		}
	}
	return nil
}

// isoMkdtempRaw 照 `mktemp -d "${TMPBASE}/<pattern>XXXXXX"` 的字符串拼接语义（✗ 不走 Clean）。
func (c *isoCtx) isoMkdtempRaw(pattern string) (string, error) {
	const cs = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	seed := time.Now().UnixNano()
	for i := 0; i < 1000; i++ {
		var b strings.Builder
		for k := 0; k < 6; k++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			b.WriteByte(cs[uint64(seed>>33)%uint64(len(cs))])
		}
		name := c.tmpBase + "/" + pattern + b.String()
		if err := os.Mkdir(name, 0o700); err == nil {
			return name, nil
		} else if !os.IsExist(err) {
			return "", err
		}
	}
	return "", os.ErrExist
}

// ── 拷贝引擎（原生 · 等价 rsync 过滤器语义）────────────────────────────────────

// isoCopyOne 把一格拷进 dst（目录建目录 · 链接建链接 · 普通件拷内容 + 权限）。
func isoCopyOne(e isoEntry, dst string) error {
	if e.isLink {
		tgt, err := os.Readlink(e.abs)
		if err != nil {
			return err
		}
		return os.Symlink(tgt, dst)
	}
	if e.isDir {
		return os.MkdirAll(dst, e.mode.Perm())
	}
	in, err := os.Open(e.abs)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, e.mode.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// isoCopyTree 照 copy_tree()：把 src 的内容按过滤器拷进 dst。返回 0/1/2（2 = 判不了）。
func (c *isoCtx) isoCopyTree(src, dst string, o *isoOpts) int {
	_, lookErr := exec.LookPath("rsync")
	if lookErr != nil {
		// 原件在「缺 rsync」这一档：带 include 判不了（rc=2）；否则退 bsdtar 兜底（逐字文案见原件）。
		if len(o.includes) != 0 {
			c.warn("  ✗ 缺 rsync ⇒ 带 --include 的隔离判不了（rc=2）：不猜、不放宽")
			return 2
		}
	}
	fs := isoParseFilters(o.isoBuildFilterLines())
	var dirs, files []isoEntry
	if err := isoWalk(src, "", fs, &dirs, &files); err != nil {
		return 1
	}
	for _, d := range dirs {
		if err := os.MkdirAll(dst+"/"+d.rel, 0o755); err != nil {
			return 1
		}
	}
	for _, e := range files {
		if err := isoCopyOne(e, dst+"/"+e.rel); err != nil {
			return 1
		}
	}
	return 0
}

// isoFixLinks 照 fix_links()：副本里指向副本之外的链接换成硬拷贝（reject ⇒ 拒绝并计数）。
// 返回 (已换条数, 危险条数)。
func (c *isoCtx) isoFixLinks(dst string, reject bool) (int, int) {
	n, bad := 0, 0
	for _, link := range isoFindSymlinkRels(dst) {
		full := dst + "/" + link
		tgt, err := os.Readlink(full)
		if err != nil {
			bad++
			continue
		}
		var abs string
		if strings.HasPrefix(tgt, "/") {
			abs = tgt
		} else {
			d := full
			if i := strings.LastIndexByte(d, '/'); i >= 0 {
				d = d[:i]
			}
			abs = d + "/" + tgt
		}
		res := isoRealpath(abs)
		if res == "" {
			res = abs
		}
		if strings.HasPrefix(res, dst+"/") {
			continue // 指在副本内部：安全
		}
		if reject {
			c.warn("  ✗ 拒绝：副本里有指向副本之外的符号链接 —— " + link + " → " + res)
			bad++
			continue
		}
		if isoIsDir(res) {
			_ = os.Remove(full)
			if err := os.MkdirAll(full, 0o755); err == nil {
				err = isoCopyTreeContents(res, full)
			}
			if err != nil {
				c.warn("  ✗ 硬拷贝链接目标失败：" + full + "（rc=1）")
				bad++
				continue
			}
		} else if isoIsFile(res) {
			_ = os.Remove(full)
			if err := isoCopyFilePreserve(res, full); err != nil {
				c.warn("  ✗ 硬拷贝链接目标失败：" + full + "（rc=1）")
				bad++
				continue
			}
		} else {
			c.warn("  ! 悬空符号链接（目标读不到）⇒ 留原样但记一笔：" + link + " → " + res)
			bad++
			continue
		}
		n++
		c.say("  · 链接→硬拷贝：" + link + " ⇐ " + res)
	}
	return n, bad
}

// isoFindSymlinkRels 照 `find "$1" -type l -print` 的产出序（readdir 原序 + 遇目录即下钻）。
func isoFindSymlinkRels(root string) []string {
	var out []string
	var rec func(dirAbs, rel string)
	rec = func(dirAbs, rel string) {
		f, err := os.Open(dirAbs)
		if err != nil {
			return
		}
		ents, rerr := f.Readdir(-1)
		_ = f.Close()
		if rerr != nil {
			return
		}
		for _, fi := range ents {
			name := fi.Name()
			rc := name
			if rel != "" {
				rc = rel + "/" + name
			}
			if fi.IsDir() {
				rec(dirAbs+"/"+name, rc)
				continue
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				out = append(out, rc)
			}
		}
	}
	rec(root, "")
	return out
}

// isoCopyFilePreserve 照 `cp -p <src> <dst>`（内容逐字节 + 保源件权限）。
func isoCopyFilePreserve(src, dst string) error {
	sfi, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, sfi.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// isoCopyTreeContents 照 `cp -R "<src>/." "<dst>/"`（递归拷 src 的内容进 dst · 保链接原样）。
func isoCopyTreeContents(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	ents, rerr := f.Readdir(-1)
	_ = f.Close()
	if rerr != nil {
		return rerr
	}
	for _, fi := range ents {
		name := fi.Name()
		s := src + "/" + name
		d := dst + "/" + name
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			tgt, lerr := os.Readlink(s)
			if lerr != nil {
				return lerr
			}
			if err := os.Symlink(tgt, d); err != nil {
				return err
			}
		case fi.IsDir():
			if err := os.MkdirAll(d, fi.Mode().Perm()); err != nil {
				return err
			}
			if err := isoCopyTreeContents(s, d); err != nil {
				return err
			}
		case fi.Mode().IsRegular():
			if err := isoCopyFilePreserve(s, d); err != nil {
				return err
			}
		}
	}
	return nil
}

// ── 产物件（ISOLATED.md / ISOLATED.manifest.tsv）──────────────────────────────

// isoWriteManifest 照 write_manifest()：逐件 sha256 + 字节数 + 是否在真仓。
func isoWriteManifest(dst, real string) {
	var b strings.Builder
	b.WriteString("relpath\tsha256\tbytes\tin_real\n")
	for _, rel := range isoFindFileRels(dst, true) {
		h := isoHashOf(dst + "/" + rel)
		sz := "0"
		if fi, err := os.Stat(dst + "/" + rel); err == nil {
			sz = strconv.FormatInt(fi.Size(), 10)
		}
		ir := "no"
		if fi, err := os.Stat(real + "/" + rel); err == nil && fi.Mode().IsRegular() {
			ir = "yes"
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", rel, h, sz, ir)
	}
	_ = os.WriteFile(dst+"/ISOLATED.manifest.tsv", []byte(b.String()), 0o644)
}

// isoWriteIsolatedMD 照 write_isolated_md()。
func isoWriteIsolatedMD(dst, real, note string, linksReplaced int) {
	now := time.Now().Format("2006-01-02 15:04:05 -0700")
	var b strings.Builder
	b.WriteString("# 这是副本 —— 删无妨\n\n")
	fmt.Fprintf(&b, "真仓路径：`%s`\n\n", real)
	fmt.Fprintf(&b, "副本路径：`%s`\n\n", dst)
	fmt.Fprintf(&b, "生成时间：%s\n\n", now)
	b.WriteString("生成者：`" + isoScriptRel + " new`\n\n")
	fmt.Fprintf(&b, "拷贝口径：rsync -a（排除重件：%s）\n\n", strings.Join(isoDefaultExcludes, " "))
	fmt.Fprintf(&b, "符号链接：本树里**没有**指向真仓的链接（已逐条换成硬拷贝 %d 条）。%s\n\n", linksReplaced, note)
	b.WriteString("## 规矩（照 GAP-20260926-20 的根因）\n\n")
	b.WriteString("1. 只在本树里读写、删除；**不要** `ln -s` 指回真仓（指回去的链接会让删除动作透传到真仓）。\n")
	b.WriteString("2. 要更多真仓内容 ⇒ 重跑 `new`（可以 `--include`），**不要**在本树里挂真仓的链接。\n")
	fmt.Fprintf(&b, "3. 收尾验回：`bash %s verify %s %s` ⇒ 必须 rc=0（证明真仓一个字节没动）。\n\n", isoScriptRel, dst, real)
	b.WriteString("## 清单\n\n`ISOLATED.manifest.tsv` = 本树逐件 sha256 + 字节数（对拍真仓用）。\n")
	_ = os.WriteFile(dst+"/ISOLATED.md", []byte(b.String()), 0o644)
}

// ── 参数面 ──────────────────────────────────────────────────────────────────

// isoOpts 一次调用的全部旗标/位置参数（照原件的全局变量组）。
type isoOpts struct {
	cmd          string // new | verify
	srcRaw       string
	dst          string
	copyDir      string
	realDir      string
	includes     []string
	userExcludes []string
	only         []string
	reject       bool
	noManifest   bool
	quiet        bool
	all          bool
	list         bool
	maxExamples  int
}

func isoNewOpts() *isoOpts { return &isoOpts{maxExamples: 20} }

// ── new ─────────────────────────────────────────────────────────────────────

func (c *isoCtx) isoCmdNew(o *isoOpts) int {
	if o.srcRaw == "" {
		return c.die(2, "用法错：new 缺 <真仓>（见 --help）")
	}
	srcArg := isoAbsPath(o.srcRaw)
	note := ""
	if isoIsLink(srcArg) {
		real := isoRealpath(srcArg)
		if o.reject {
			return c.die(1, "拒绝：源路径本身是符号链接（"+srcArg+" → "+real+"）；--reject-symlinks 档不许硬拷贝其目标")
		}
		c.say("· 源路径是**符号链接** ⇒ 按真身硬拷贝：" + srcArg + " → " + real)
		note = "（源参数是符号链接，已按真身拷。）"
		srcArg = real
	}
	if !isoIsDir(srcArg) {
		return c.die(2, "判不了：真仓目录不存在 —— "+srcArg)
	}
	realRepo := isoRealpath(srcArg)
	if realRepo == "" {
		realRepo = srcArg
	}

	if len(o.only) != 0 {
		for _, on := range o.only {
			if !isoIsFile(realRepo + "/" + on) {
				return c.die(2, "判不了：--only 指的不是真仓里的件 —— "+on)
			}
			o.includes = append(o.includes, on)
		}
	}

	if o.list {
		return c.isoListPreview(o, realRepo)
	}

	dst := ""
	if o.dst == "" {
		tmp, err := c.isoMkdtempRaw(isoDefaultName + ".")
		if err != nil {
			return c.die(2, "判不了：mktemp -d 失败")
		}
		dst = tmp
	} else {
		dst = isoAbsPath(o.dst)
		rrAbs := isoRealpath(realRepo)
		if rrAbs == "" {
			rrAbs = realRepo
		}
		if strings.HasPrefix(isoRealpath(dst)+"/", rrAbs+"/") {
			return c.die(2, "拒绝：--dst 落在真仓内（"+dst+" ⊂ "+realRepo+"）—— 副本必须建在真仓之外")
		}
		if !isoExists(dst) {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return c.die(2, "判不了：建不出 --dst —— "+dst)
			}
		}
	}

	c.say("== 只读隔离（硬拷贝）==")
	c.say("· 真仓：" + realRepo)
	c.say("· 副本：" + dst)
	if len(o.includes) != 0 {
		c.say("· 白名单 --include：" + strings.Join(o.includes, " "))
	}
	c.say("· 生效排除（rsync 过滤器 · 现读实际生效）：")
	for _, flt := range o.isoBuildFilterLines() {
		if strings.HasPrefix(flt, "--exclude=") {
			c.say("    " + flt)
		}
	}
	rc := c.isoCopyTree(realRepo, dst, o)
	if rc == 2 {
		return 2
	}
	if rc != 0 {
		return c.die(1, fmt.Sprintf("拷贝失败（rsync/tar rc=%d）—— 副本可能不完整，别拿它当基线", rc))
	}

	replaced, bad := c.isoFixLinks(dst, o.reject)
	if bad != 0 {
		return c.die(1, fmt.Sprintf("副本里有 %d 条**危险符号链接**（指向副本之外/悬空）—— 见上；不许在这种树上做删除动作", bad))
	}

	n := isoCountFiles(dst)
	b := isoTotalBytes(dst)

	if !o.noManifest {
		isoWriteManifest(dst, realRepo)
	}
	isoWriteIsolatedMD(dst, realRepo, note, replaced)
	if n <= 0 {
		return c.die(2, "空转：副本里 0 件 ⇒ 判不了（别把空树当隔离成功）")
	}
	c.say("== 读数 ==")
	c.say("· 真仓路径：" + realRepo)
	c.say("· 副本路径：" + dst)
	c.say(fmt.Sprintf("· 件数：%d · 总字节：%d（%s）", n, b, isoHumanBytes(b)))
	c.say(fmt.Sprintf("· 链接→硬拷贝：%d 条 · ISOLATED.md：%s/ISOLATED.md", replaced, dst))
	c.say("· 验回：bash " + isoScriptRel + " verify \"" + dst + "\" \"" + realRepo + "\"")
	c.say("· 收尾（本件**不会**替你删）：rm -rf \"" + dst + "\"")
	return 0
}

// isoExists 照 `[ -e ]`（跟随符号链接 ⇒ Stat；悬空链接 ⇒ false）。
func isoExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// isoListPreview 照 list_preview()：预演会取哪些件 + 件数 + 字节（零写盘）。
func (c *isoCtx) isoListPreview(o *isoOpts, real string) int {
	if _, err := exec.LookPath("rsync"); err != nil {
		c.warn("  ✗ 缺 rsync ⇒ --list 判不了（rc=2）：不猜、不放宽")
		return 2
	}
	lines := o.isoBuildFilterLines()
	c.say("== 预演（--list · 零写盘）==")
	c.say("· 真仓：" + real)
	if len(o.includes) != 0 {
		c.say("· 白名单：" + strings.Join(o.includes, " "))
	}
	c.say("· 生效过滤器（现读 · include 在前 exclude 收尾）：")
	for _, ln := range lines {
		c.say("    " + ln)
	}
	fs := isoParseFilters(lines)
	var dirs, files []isoEntry
	if err := isoWalk(real, "", fs, &dirs, &files); err != nil {
		c.warn("  ✗ 预演扫描读不到：" + err.Error() + "（rc=2）")
		return 2
	}
	var n int
	var b int64
	for _, e := range files {
		n++
		b += e.size
	}
	c.say(fmt.Sprintf("· 会取件数：%d · 字节：%d（%s）", n, b, isoHumanBytes(b)))
	for _, e := range files {
		c.say("    " + e.rel)
	}
	c.say("（预演结束 —— 未写任何件；去掉 --list 才真拷贝）")
	return 0
}

// ── verify ──────────────────────────────────────────────────────────────────

func (c *isoCtx) isoCmdVerify(o *isoOpts) int {
	if o.copyDir == "" || o.realDir == "" {
		return c.die(2, "用法错：verify 要 <副本> <真仓> 两个实参（见 --help）")
	}
	copyDir := isoAbsPath(o.copyDir)
	real := isoAbsPath(o.realDir)
	if !isoIsDir(copyDir) {
		return c.die(2, "判不了：副本目录不存在 —— "+copyDir)
	}
	if !isoIsDir(real) {
		return c.die(2, "判不了：真仓目录不存在 —— "+real)
	}

	manifest := copyDir + "/ISOLATED.manifest.tsv"
	srcNote := "现算副本侧 sha256"
	if isoIsFile(manifest) {
		srcNote = "副本侧取 ISOLATED.manifest.tsv（隔离当时的读数）"
	}

	c.say("== 逐件 sha256 对拍（副本 ⟷ 真仓）==")
	c.say("· 副本：" + copyDir)
	c.say("· 真仓：" + real)
	c.say("· 副本侧口径：" + srcNote)

	var total, same, diff, absent int
	var diffs []string
	for _, rel := range isoFindFileRels(copyDir, true) {
		total++
		ch := ""
		if isoIsFile(manifest) {
			ch = isoManifestLookup(manifest, rel)
		}
		if ch == "" {
			ch = isoHashOf(copyDir + "/" + rel)
		}
		if !isoIsFile(real + "/" + rel) {
			absent++
			diffs = append(diffs, "真仓侧缺件: "+rel)
			continue
		}
		rh := isoHashOf(real + "/" + rel)
		if ch == rh {
			same++
		} else {
			diff++
			diffs = append(diffs, "sha256 不一致: "+rel)
		}
	}

	c.say(fmt.Sprintf("· 对拍件数：%d · 一致：%d · 不一致：%d · 真仓侧缺件：%d", total, same, diff, absent))
	if total == 0 {
		return c.die(2, "空转：副本里 0 件 ⇒ 判不了（空树不算「证明没改到真仓」）")
	}
	if diff > 0 || absent > 0 {
		shown := 0
		for _, d := range diffs {
			shown++
			if shown > o.maxExamples {
				break
			}
			c.say("  ✗ " + d)
		}
		return c.die(1, "真仓与副本对不上 ⇒ 要么真仓被改过、要么副本被改过 —— 先查清再谈结论")
	}
	c.say("⇒ rc=0：副本逐件与真仓一致 ⇒ **这一轮验证一个字节都没改到真仓**")
	return 0
}

// isoManifestLookup 照 `awk -F'\t' -v r=rel '$1==r{print $2; exit}'`：取该件在清单里的 sha256。
func isoManifestLookup(manifest, rel string) string {
	b, err := os.ReadFile(manifest)
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if ln == "" {
			continue
		}
		parts := strings.Split(ln, "\t")
		if len(parts) >= 2 && parts[0] == rel {
			return parts[1]
		}
	}
	return ""
}

// ── self-test（正控 / 负控 / 用法错 · 合成夹具）───────────────────────────────

// isoSelfTest 复刻原件 run_self_test() 的**逐条成对控**（合成夹具 · 一件真东西都不碰）。
// ★ 原件每条 chk 里跑的是 `bash "$SELF" new|verify`（子进程）；本件改**进程内**调用同一入口。
func (c *isoCtx) isoSelfTest() int {
	tmp, err := c.isoMkdtempRaw(isoDefaultName + "-selftest.")
	if err != nil {
		return 2
	}
	defer os.RemoveAll(tmp)
	fix := tmp + "/fix"
	real := fix + "/real"
	for _, d := range []string{
		real + "/core/cmd/zerg/testdata", real + "/scripts/dev", real + "/bin",
	} {
		_ = os.MkdirAll(d, 0o755)
	}
	_ = os.WriteFile(real+"/core/cmd/zerg/testdata/cli-matrix.json", []byte("matrix\n"), 0o644)
	_ = os.WriteFile(real+"/scripts/dev/keep.sh", []byte("keep\n"), 0o644)
	_ = os.WriteFile(real+"/bin/big.bin", []byte("heavy\n"), 0o644)
	realShaBefore := isoHashOf(real + "/core/cmd/zerg/testdata/cli-matrix.json")

	bad := 0
	chk := func(desc string, cond bool, reading string) {
		if cond {
			c.say("   ✓ " + desc)
			return
		}
		tail := ""
		if reading != "" {
			tail = "（" + reading + "）"
		}
		c.say("   ✗ " + desc + tail)
		bad++
	}
	run := func(argv ...string) (int, string) {
		var ob, eb strings.Builder
		sub := &isoCtx{stdout: &ob, stderr: &eb, tmpBase: c.tmpBase}
		rc := sub.isoDispatch(argv)
		return rc, ob.String() + eb.String()
	}

	c.say("── isolate-ro.sh 自检（成对控 · 合成夹具：" + fix + "）──")

	// ── 正控：隔离一棵树 ⇒ rc=0 · ISOLATED.md 在 · 副本可写 · 真仓未变 ──
	rc, out := run("new", real, "--dst", tmp+"/tree")
	chk("正控：new ⇒ rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))
	chk("正控：副本里有 ISOLATED.md", isoIsFile(tmp+"/tree/ISOLATED.md"), "")
	md, _ := os.ReadFile(tmp + "/tree/ISOLATED.md")
	chk("正控：ISOLATED.md 写着真仓路径", strings.Contains(string(md), real), "")
	chk("正控：件数/总字节都打印了（3 件 − 被排除的 bin/ 1 件 = 2",
		strings.Contains(out, "件数：2") && strings.Contains(out, "总字节："), "")
	chk("正控：缺省排除生效（bin/ 没进副本）", !isoExists(tmp+"/tree/bin"), "")
	chk("正控：硬拷贝是真件（同一字节）",
		isoHashOf(tmp+"/tree/core/cmd/zerg/testdata/cli-matrix.json") == realShaBefore, "")
	_ = os.WriteFile(tmp+"/tree/scripts/dev/keep.sh", []byte("edited-in-copy\n"), 0o644)
	_ = os.Remove(tmp + "/tree/core/cmd/zerg/testdata/cli-matrix.json")
	rc, _ = run("verify", tmp+"/tree", real)
	chk("正控：verify ⇒ rc=0（真仓未被改）", rc == 0, fmt.Sprintf("rc=%d", rc))
	chk("正控：真仓那件 sha256 未变（删的是副本）",
		isoHashOf(real+"/core/cmd/zerg/testdata/cli-matrix.json") == realShaBefore, "")

	// ── --no-manifest 档：无清单 ⇒ verify 现算副本侧 sha，不得假红 ──
	rc, _ = run("new", real, "--dst", tmp+"/tree-nomani", "--no-manifest")
	chk("无清单档：new --no-manifest ⇒ rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))
	chk("无清单档：副本里没有 ISOLATED.manifest.tsv",
		!isoIsFile(tmp+"/tree-nomani/ISOLATED.manifest.tsv"), "")
	rc, out = run("verify", tmp+"/tree-nomani", real)
	chk("无清单档：verify ⇒ rc=0（现算副本侧 sha · 不假红）", rc == 0, fmt.Sprintf("rc=%d", rc))
	chk("无清单档：对拍读数「不一致 == 0」", strings.Contains(out, "不一致：0"), "")

	// ── 负控 A：真仓被改一个字节 ⇒ verify 必须 rc=1 且点名 ──
	keepBackup, _ := os.ReadFile(real + "/scripts/dev/keep.sh")
	_ = os.WriteFile(real+"/scripts/dev/keep.sh", []byte("SOMEONE-TOUCHED-THE-REAL-REPO\n"), 0o644)
	rc, out = run("verify", tmp+"/tree", real)
	chk("负控 A：真仓被改 ⇒ verify rc=1", rc == 1, fmt.Sprintf("rc=%d", rc))
	chk("负控 A：点名那一件", strings.Contains(out, "scripts/dev/keep.sh"), "")
	_ = os.WriteFile(real+"/scripts/dev/keep.sh", keepBackup, 0o644)

	// ── 负控 B：真仓里有一条指回真仓的符号链接（事故形态）⇒ 缺省档换成硬拷贝 / 严格档拒绝 ──
	_ = os.MkdirAll(fix+"/farmsrc", 0o755)
	_ = os.Symlink(real+"/core/cmd/zerg/testdata", fix+"/farmsrc/testdata-link")
	rc, _ = run("new", fix+"/farmsrc", "--dst", tmp+"/farm")
	chk("负控 B：缺省档 rc=0 且把链接换成硬拷贝（副本里 0 条链接）",
		rc == 0 && len(isoFindSymlinkRels(tmp+"/farm")) == 0, fmt.Sprintf("rc=%d", rc))
	fc, _ := os.ReadFile(tmp + "/farm/testdata-link/cli-matrix.json")
	chk("负控 B：链接目标内容是真件（可读、对得上）", strings.Contains(string(fc), "matrix"), "")
	rc, out = run("new", fix+"/farmsrc", "--dst", tmp+"/farm2", "--reject-symlinks")
	chk("负控 B'：--reject-symlinks ⇒ rc=1 且点出链接",
		rc == 1 && strings.Contains(out, "testdata-link"), fmt.Sprintf("rc=%d", rc))

	// ── 用法错 ──
	rc, _ = run("new")
	chk("用法错：new 缺参数 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	rc, _ = run("new", tmp+"/no-such-repo")
	chk("用法错：真仓不存在 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	rc, _ = run("verify", tmp+"/tree")
	chk("用法错：verify 缺一个实参 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	rc, _ = run("bogus")
	chk("用法错：未知子命令 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))

	// ── 增补（GAP-20260927-96 / 93）：等号写法 · --dst 包含性 · 生效排除可见 · 缺省排除含 ui/target ──
	_ = os.MkdirAll(real+"/ui/target", 0o755)
	_ = os.MkdirAll(real+"/wall/target", 0o755)
	_ = os.WriteFile(real+"/ui/target/artifact.bin", []byte("big\n"), 0o644)
	_ = os.WriteFile(real+"/wall/target/artifact.bin", []byte("big\n"), 0o644)
	rc, out = run("new", real, "--dst", tmp+"/tree-eq", "--exclude=bin")
	chk("等号写法：--exclude=bin 被接受（旧版报「未知旗标」rc=2）", rc == 0, fmt.Sprintf("rc=%d", rc))
	chk("生效排除清单可见（脚本开头逐行印 --exclude=…）", strings.Contains(out, "--exclude="), "")
	chk("缺省排除含 ui/target（29G 不再被整棵拷进来）", !isoExists(tmp+"/tree-eq/ui/target"), "")
	chk("缺省排除含 wall/target", !isoExists(tmp+"/tree-eq/wall/target"), "")
	rc, out = run("new", real, "--dst", real+"/inside")
	chk("包含性：--dst 落在真仓内 ⇒ rc=2（副本不许写进真仓 · GAP-20260927-93）", rc == 2, fmt.Sprintf("rc=%d", rc))
	chk("包含性：点名「真仓内」", strings.Contains(out, "真仓内"), "")
	chk("包含性：拒写后真仓里没被建出 inside/（未产生副作用）", !isoExists(real+"/inside"), "")

	// ── 增补（GAP-20260926-47）：--list 零写盘 · --only 单件直取 ──
	realNBefore := isoCountFiles(real)
	rc, out = run("new", real, "--only", "scripts/dev/keep.sh", "--dst", tmp+"/tree-only")
	chk("--only 单件：rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))
	gotOnly := strings.Join(isoFindFileRels(tmp+"/tree-only", true), " ")
	if gotOnly != "" {
		gotOnly += " "
	}
	chk("--only 单件：副本里只有那件 + 清单", gotOnly == "scripts/dev/keep.sh ", "")
	chk("--only 单件：件数 1", strings.Contains(out, "件数：1"), "")
	rc, _ = run("new", real, "--only", "no/such/file.txt", "--dst", tmp+"/tree-only2")
	chk("--only 指非件 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	rc, out = run("new", real, "--list")
	chk("--list：rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))
	chk("--list：印「预演」且件数=2（现算：cli-matrix.json + keep.sh）",
		strings.Contains(out, "预演") && strings.Contains(out, "会取件数：2"), "")
	chk("--list：零写盘（真仓件数跑前跑后不变）", isoCountFiles(real) == realNBefore, "")

	if bad == 0 {
		c.say("── 自检结论：全过 ──")
		return 0
	}
	c.say(fmt.Sprintf("── 自检结论：**不过**（%d 条）──", bad))
	return 2
}

// ── 入口（解析 + 分派）────────────────────────────────────────────────────────

// isoDispatch 照原件的 while/case 解析 + 末尾 case 分派（argv 与原件位置参数同形）。
func (c *isoCtx) isoDispatch(argv []string) int {
	o := isoNewOpts()
	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "new" || a == "verify":
			if o.cmd != "" {
				return c.die(2, "用法错：只给一个子命令（见 --help）")
			}
			o.cmd = a
			i++
		case a == "--self-test":
			return c.isoSelfTest()
		case a == "-h" || a == "--help":
			io.WriteString(c.stdout, isoUsage)
			return 0
		case a == "--dst":
			if i+1 >= len(argv) {
				return c.die(2, "用法错：--dst 缺实参")
			}
			o.dst = argv[i+1]
			i += 2
		case strings.HasPrefix(a, "--dst="):
			o.dst = a[len("--dst="):]
			i++
		case a == "--include":
			if i+1 >= len(argv) {
				return c.die(2, "用法错：--include 缺实参")
			}
			o.includes = append(o.includes, argv[i+1])
			i += 2
		case strings.HasPrefix(a, "--include="):
			o.includes = append(o.includes, a[len("--include="):])
			i++
		case a == "--exclude":
			if i+1 >= len(argv) {
				return c.die(2, "用法错：--exclude 缺实参")
			}
			o.userExcludes = append(o.userExcludes, argv[i+1])
			i += 2
		case strings.HasPrefix(a, "--exclude="):
			o.userExcludes = append(o.userExcludes, a[len("--exclude="):])
			i++
		case a == "--max-examples":
			if i+1 >= len(argv) {
				return c.die(2, "用法错：--max-examples 缺实参")
			}
			if v, err := strconv.Atoi(argv[i+1]); err == nil {
				o.maxExamples = v
			}
			i += 2
		case strings.HasPrefix(a, "--max-examples="):
			if v, err := strconv.Atoi(a[len("--max-examples="):]); err == nil {
				o.maxExamples = v
			}
			i++
		case a == "--reject-symlinks":
			o.reject = true
			i++
		case a == "--no-manifest":
			o.noManifest = true
			i++
		case a == "--all":
			o.all = true
			i++
		case a == "--quiet":
			o.quiet = true
			i++
		case a == "--only":
			if i+1 >= len(argv) {
				return c.die(2, "用法错：--only 缺实参")
			}
			o.only = append(o.only, argv[i+1])
			i += 2
		case strings.HasPrefix(a, "--only="):
			o.only = append(o.only, a[len("--only="):])
			i++
		case a == "--list":
			o.list = true
			i++
		case strings.HasPrefix(a, "-"):
			return c.die(2, "用法错：未知旗标 "+a+"（见 --help）")
		default:
			if o.cmd == "new" && o.srcRaw == "" {
				o.srcRaw = a
			} else if o.cmd == "verify" && o.copyDir == "" {
				o.copyDir = a
			} else if o.cmd == "verify" && o.realDir == "" {
				o.realDir = a
			} else {
				return c.die(2, "用法错：多余的位置参数 "+a+"（见 --help）")
			}
			i++
		}
	}

	switch o.cmd {
	case "new":
		return c.isoCmdNew(o)
	case "verify":
		return c.isoCmdVerify(o)
	case "":
		io.WriteString(c.stdout, isoUsage)
		return 2
	}
	return c.die(2, "用法错：未知子命令 "+o.cmd+"（见 --help）")
}

// isolateRoGateMain = 件入口（inproc 口径：argv 不含程序名 · 与原件 `$@` 同形）。
func isolateRoGateMain(stdout, stderr io.Writer, argv []string) int {
	return isoNewCtx(stdout, stderr).isoDispatch(argv)
}
