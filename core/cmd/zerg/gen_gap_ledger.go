// gen_gap_ledger.go —— 生成器件 `scripts/docs/gen-gap-ledger.py`（451 行）的 **Go 真搬**
// （v2.5.14 · M2 · C 波 · 生成器面搬运 · 入口 `genGapLedgerGateMain`）。
//
// 本件形态：生成器 · 产出 = 「结转账」文件（`缺口总账-<日期>.md`）。
// 生成器验收与判据件不同：**同输入下 py 产出 ⟷ 本入口产出 文件 sha256 逐字节相同**
// （见 `gen_gap_ledger_test.go` 与回执的产物对拍表）。
//
// 真源面：本件 **现跑** `<仓根>/bin/zerg gap export` 取其 stdout **逐字节照落**为生成件；
// 本件自己不排版、不增删一个字（同 py）。
//
// 逐字节对齐要点（照 py 逐条）：
//
//	① py argparse 的 `--help`/usage 文本按现读原文**逐字嵌入**（本环境 py3.14 ⇒ `options:`）；
//	   解释器版本会改该面（≤3.9 印 `optional arguments:`）⇒ 已列入未覆盖面（环境依赖）。
//	② py 的 `\d`/`\s` 是 **Unicode 语义**、Go RE2 默认 ASCII ⇒ 已展开为 `\p{Nd}` / 自写空白类。
//	③ py `subprocess.run(..., text=True)` = **universal newlines** 解码 ⇒ 本件对捕获字节做
//	   `\r\n`→`\n`、`\r`→`\n` 折叠后再取日期/落盘（✗ 不直接按原始字节切）。
//	④ 路径拼接一律 **posixpath 同构**（自写 `gglPyJoin`/`gglPyNormpath`/`gglPyRelpath`，
//	   ✗ 不用 `filepath.Join`）—— 见各函数注释。
//	⑤ 版本线排序键 = py `(tuple(int(d) for d in findall(r"\d+", ver)), ver)`：**数字元组**优先、
//	   同位再比字符串（✗ 不用字符串比；`v2.5.9` 会在纯字符串下反序压过 `v2.5.13`）。
//	⑥ py `sorted(os.listdir(base))` / `sorted(glob(...))` / `sorted(seen.items())` 三处**显式排序**
//	   ⇒ 输出行序的决定者是这三处（遍历原序只控制 `os.walk` 的产出序、最终被 `vkey` 排序覆盖）。
//	⑦ `--json` 面：py `json.dumps(..., ensure_ascii=False)` 的**默认分隔符 = `", "` / `": "`**
//	   ＋ **键按 dict 插入序** ⇒ 本件自写有序 JSON 编码（✗ 不用 `encoding/json` 的 map/无空格形态）。
//	⑧ 非确定面：`--mark` 档 py 件没有该子命令；本件机读/落盘产物**不含时刻**（`--emit` 只写 export 字节）
//	   ⇒ 产可逐字节对拍。唯一时刻源在 py 侧只有 `--version`（常量）以外无处，产物为纯确定性。
//	⑨ **stderr 面（解释器版本告警 · 环境依赖 · ✗ 不复刻）**：py 真身 `scripts/docs/gen-gap-ledger.py:29`
//	   的**模块 docstring（非 raw 串）**里写有 `^Q-\d+$` ⇒ **CPython ≥3.12 在编译该源文件时**会向 stderr
//	   打一条 `SyntaxWarning: invalid escape sequence '\d'`（并回显 `:29` 源码行）；**≤3.11 完全静默**，
//	   且 3.12/3.13（短式 `invalid escape sequence '\d'`）与 3.14（长式 `"\d" is an invalid escape sequence…`）
//	   的**告警正文还不一样**。该告警**不是本件契约** —— 程序自身从不产它，是**解释器编译 py 源**的副产
//	   ⇒ 判为**解释器版本面（环境依赖）**、列入**未覆盖面**：本件**绝不合成**该告警（在 Go 侧逐字复刻
//	   ＝把某台解释器版本的告警**正文钉进仓件**，违「工具链面」规矩）。★ 两态取证（同一解释器路径）：
//	   按仓约定钉 `/usr/bin/python3`（3.9.6 · 见 `baseline_ratchet_test.go:21 pyInterp`）跑 py 真身 ⟷ 本件，
//	   `--self-test` 的 **rc / stdout / stderr 三面逐字同**（stderr **皆空**）；该行**只在 ≥3.12 才见**。
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py `REPO_ROOT = dirname(dirname(abspath(__file__)))` ⇒ 起于"脚本所在目录
//	    的祖父"；本件无 `__file__` 语境 ⇒ 从 **cwd** 取根（`gglRepoRoot`）。「脚本位置 == cwd」（在仓根
//	    执行）时两者读数逐字相同；否则路径推导面不同（✗ 不属逐字对齐承诺）。仓外夹具对拍时把 py 副本
//	    放 `<fx>/scripts/docs/`、并以 cwd=`<fx>` 跑本件 ⇒ 两侧 `REPO_ROOT` 同为 `<fx>`（已实测坐实）。
//	(b) **自指检查**：本件扫描面 = 调用方点名的 `<docs-root>`（外仓文档树）与 `<仓根>/bin/zerg` 的
//	    **运行输出**；本件（落 `core/cmd/zerg/`）**不在自身扫描面内**。量化：全件零绝对私有路径字面量
//	    （零 `/Volumes`、零 `/Users`）、退码仅 0/1/2（⊆ 真源退码表已登记集）、零「JSON 键名连续引号原串」。
//
// 本单边界（照实）：**只落码、不接线** —— 不注册命令、不删 py 原件、无调用点（M2 刻意状态）。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const gglSelfName = "gen-gap-ledger.py"
const gglVersion = "gen-gap-ledger.py v1.1.0"

// ── 登记面（与门 `check-gap-ledger-view.py` 同一口径 · 逐字对齐）──────────────
const gglDocsDirname = "Zerg-内部文档"
const gglLedgerDir = "项目文档"
const gglSourceName = "zerg-cli-gaps.jsonl"
const gglExportTimeout = 90 * time.Second

var (
	// py `^缺口总账-(\d{8})\.md$`（`\d` Unicode ⇒ `\p{Nd}`）。
	gglLedgerNameRE = regexp.MustCompile(`^缺口总账-[\p{Nd}]{8}\.md$`)
	// py `·\s*(\d{4})-(\d{2})-(\d{2})\s*$` with re.M（`\s` 近似为 ASCII 空白类，见未覆盖面）。
	gglExportDateRE = regexp.MustCompile(`(?m)·[ \t\n\r\f\v]*([\p{Nd}]{4})-([\p{Nd}]{2})-([\p{Nd}]{2})[ \t\n\r\f\v]*$`)
	// py `^Q-\d+$`（`\d` Unicode ⇒ `\p{Nd}`）。
	gglQIDRE = regexp.MustCompile(`^Q-[\p{Nd}]+$`)
	// py `\d+`（Unicode 数字）。
	gglDigitsRE = regexp.MustCompile(`[\p{Nd}]+`)
)

// 豁免（与门同一条）：路径段命中 ⇒ 承接副本、不算版本线候选。
var gglExemptPathSegs = []string{"承接自"}

// ── py argparse 现读文本（本环境 py3.14）──────────────────────────────────
const gglUsage = "usage: gen-gap-ledger.py [-h] [--emit] [--check] [--dry-run] [--qid-check]\n" +
	"                         [--docs-root DOCS_ROOT] [--version-dir VERSION_DIR]\n" +
	"                         [--source SOURCE] [--json] [--self-test] [--version]\n"

const gglHelpText = gglUsage +
	"\n" +
	"结转账生成件落盘面：`zerg gap export` 直出 ⇒ `缺口总账-<日期>.md`\n" +
	"\n" +
	"options:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --emit                落盘（唯一写面 · 只写生成件那一件）\n" +
	"  --check               只判：生成件 == 现跑直出？（不改任何件）\n" +
	"  --dry-run             只打落点与差异（零副作用）\n" +
	"  --qid-check           只跑 qid 读面校验（不碰生成件）\n" +
	"  --docs-root DOCS_ROOT\n" +
	"                        文档仓根（默认 = 主仓的兄弟目录 Zerg-内部文档/）\n" +
	"  --version-dir VERSION_DIR\n" +
	"                        版本线目录（默认真源：现读含结转账的最新版本线）\n" +
	"  --source SOURCE       真源 jsonl（缺省 = <状态目录>/zerg-cli-gaps.jsonl）\n" +
	"  --json                机读输出（zerg/v1 形状）\n" +
	"  --self-test           合成夹具自检（不碰真源/真账/真文档仓）\n" +
	"  --version             show program's version number and exit\n"

// ── posixpath 同构（自写 · ✗ 不用 filepath）───────────────────────────────

// gglPyJoin —— posixpath.join：空段忽略（除首段空⇒保留）· 后段绝对路径丢弃前段 · `..` 原样留串。
func gglPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path = path + b
		} else {
			path = path + "/" + b
		}
	}
	return path
}

// gglPyNormpath —— CPython posixpath.normpath（保留恰两个前导斜杠 · 见 doc_index_gen 同型说明）。
func gglPyNormpath(path string) string {
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
	var nc []string
	for _, c := range strings.Split(path, "/") {
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

// gglPyAbspath —— os.path.abspath：非绝对则拼 cwd，再 normpath。
func gglPyAbspath(path string) string {
	if !strings.HasPrefix(path, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		path = gglPyJoin(wd, path)
	}
	return gglPyNormpath(path)
}

// gglPyRelpath —— os.path.relpath（词法 · abspath 后算公共前缀 + `..`）。
func gglPyRelpath(path, start string) string {
	p := gglPyAbspath(path)
	s := gglPyAbspath(start)
	if p == s {
		return "."
	}
	pp := strings.Split(strings.TrimPrefix(p, "/"), "/")
	ss := strings.Split(strings.TrimPrefix(s, "/"), "/")
	i := 0
	for i < len(pp) && i < len(ss) && pp[i] == ss[i] {
		i++
	}
	var out []string
	for j := i; j < len(ss); j++ {
		out = append(out, "..")
	}
	out = append(out, pp[i:]...)
	if len(out) == 0 {
		return "."
	}
	return gglPyNormpath(strings.Join(out, "/"))
}

func gglRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return gglPyAbspath(wd)
}

func gglExportBin(repoRoot string) string { return gglPyJoin(repoRoot, "bin", "zerg") }

func gglExportCmd(repoRoot string) string {
	return gglExportBin(repoRoot) + " gap export"
}

// ── export 现跑 ─────────────────────────────────────────────────────────

// gglRunExport —— 现跑 `<仓根>/bin/zerg gap export`（cwd = 仓根 · 环境原样透传）。
// 返回 (status, stdout, eread)：status ∈ {"ok","no_bin","timeout","rc","decode"}。
func gglRunExport(repoRoot string) (string, string, map[string]string) {
	bin := gglExportBin(repoRoot)
	ctx, cancel := context.WithTimeout(context.Background(), gglExportTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "gap", "export")
	cmd.Dir = repoRoot
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "timeout", "", map[string]string{"cmd": bin, "msg": fmt.Sprintf("超时 %d 秒", int(gglExportTimeout.Seconds()))}
		}
		var ee *exec.Error
		if errors.As(err, &ee) && errors.Is(ee.Err, exec.ErrNotFound) {
			return "no_bin", "", map[string]string{"cmd": bin, "msg": ee.Err.Error()}
		}
		if errors.Is(err, fs.ErrNotExist) {
			return "no_bin", "", map[string]string{"cmd": bin, "msg": fmt.Sprintf("[Errno 2] No such file or directory: '%s'", bin)}
		}
		var xe *exec.ExitError
		if errors.As(err, &xe) {
			se := strings.TrimSpace(errb.String())
			if len(se) > 400 {
				se = se[len(se)-400:]
			}
			return "rc", "", map[string]string{"cmd": gglExportCmd(repoRoot), "rc": strconv.Itoa(xe.ExitCode()), "stderr": se}
		}
		return "no_bin", "", map[string]string{"cmd": bin, "msg": err.Error()}
	}
	raw := out.Bytes()
	if !utf8.Valid(raw) {
		// py text=True 解码非法字节 ⇒ UnicodeDecodeError（未捕 ⇒ 回溯 rc=1）；本件按新口径 fail-closed。
		return "decode", "", map[string]string{"cmd": gglExportCmd(repoRoot), "msg": "导出物非合法 UTF-8（fail-closed）"}
	}
	return "ok", gglUniversalNL(string(raw)), map[string]string{"cmd": gglExportCmd(repoRoot), "rc": "0"}
}

// gglUniversalNL —— py 文本模式 universal newlines。
func gglUniversalNL(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// ── 版本线 / 日期 / 落点 ──────────────────────────────────────────────────

type gglCand struct{ ver, rel string }

// gglWalk —— os.walk 产出序：先文件（readdir 原序）后按子目录原序下钻（followlinks=false 见未覆盖面）。
func gglWalk(root string, fn func(dirpath string, files []string)) {
	ents, err := os.ReadDir(root)
	if err != nil {
		return
	}
	var files, dirs []string
	for _, e := range ents {
		p := gglPyJoin(root, e.Name())
		st, err := os.Stat(p) // py entry.is_dir() 跟随符号链接
		if err == nil && st.IsDir() {
			dirs = append(dirs, p)
		} else {
			files = append(files, e.Name())
		}
	}
	fn(root, files)
	for _, d := range dirs {
		gglWalk(d, fn)
	}
}

func gglLedgerCandidates(docs string) []gglCand {
	base := gglPyJoin(docs, gglLedgerDir)
	st, err := os.Stat(base)
	if err != nil || !st.IsDir() {
		return nil
	}
	ents, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var vers []string
	for _, e := range ents {
		vers = append(vers, e.Name())
	}
	sort.Strings(vers) // py sorted(os.listdir(base))
	var out []gglCand
	for _, ver := range vers {
		vdir := gglPyJoin(base, ver)
		st2, err2 := os.Stat(vdir)
		if err2 != nil || !st2.IsDir() {
			continue
		}
		gglWalk(vdir, func(dirpath string, files []string) {
			for _, fn := range files {
				if !gglLedgerNameRE.MatchString(fn) {
					continue
				}
				rel := gglPyRelpath(gglPyJoin(dirpath, fn), docs)
				exempt := false
				for _, seg := range gglExemptPathSegs {
					if strings.Contains(rel, seg) {
						exempt = true
						break
					}
				}
				if exempt {
					continue
				}
				out = append(out, gglCand{ver: ver, rel: rel})
			}
		})
	}
	return out
}

type gglVerKey struct {
	nums []int
	s    string
}

func gglVkey(ver string) gglVerKey {
	ms := gglDigitsRE.FindAllString(ver, -1)
	nums := make([]int, 0, len(ms))
	for _, m := range ms {
		n, _ := strconv.Atoi(m) // py int() 认 Unicode 数字 ⇒ 本件只覆盖 ASCII（见未覆盖面）
		nums = append(nums, n)
	}
	return gglVerKey{nums: nums, s: ver}
}

// gglVerKeyLess —— py 元组比较 (nums, ver)：逐位比 nums，前缀短者小；nums 全同再比 ver 字符串。
func gglVerKeyLess(a, b gglVerKey) bool {
	n := len(a.nums)
	if len(b.nums) < n {
		n = len(b.nums)
	}
	for i := 0; i < n; i++ {
		if a.nums[i] != b.nums[i] {
			return a.nums[i] < b.nums[i]
		}
	}
	if len(a.nums) != len(b.nums) {
		return len(a.nums) < len(b.nums)
	}
	return a.s < b.s
}

// gglPickVersionDir —— 现读含结转账的最新版本线（sorted(..., key=vkey)[-1]）；读不到 ⇒ ("", false)。
func gglPickVersionDir(docs string) (string, bool) {
	cand := gglLedgerCandidates(docs)
	if len(cand) == 0 {
		return "", false
	}
	best := 0
	for i := 1; i < len(cand); i++ {
		if gglVerKeyLess(gglVkey(cand[best].ver), gglVkey(cand[i].ver)) {
			best = i
		}
	}
	return cand[best].ver, true
}

// gglExportDate —— 从 export 人面取 `YYYYMMDD`（取不到 ⇒ ("", false)）。
func gglExportDate(out string) (string, bool) {
	m := gglExportDateRE.FindStringSubmatch(out)
	if m == nil {
		return "", false
	}
	return m[1] + m[2] + m[3], true
}

func gglTargetPath(docs, versionDir, date8 string) string {
	return gglPyJoin(docs, gglLedgerDir, versionDir, "缺口总账-"+date8+".md")
}

// gglReadBytes —— py `os.path.isfile` 口径：非常规件（含目录）⇒ (nil,false)。
func gglReadBytes(path string) ([]byte, bool) {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return nil, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return b, true
}

// gglWriteAtomic —— 固定后缀 tmp + fsync + 原子 rename；写后读回逐字节比。
func gglWriteAtomic(path string, data []byte) bool {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return false
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return false
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return false
	}
	if err := f.Close(); err != nil {
		return false
	}
	if err := os.Rename(tmp, path); err != nil {
		return false
	}
	got, ok := gglReadBytes(path)
	return ok && bytes.Equal(got, data)
}

// ── qid 读面 ───────────────────────────────────────────────────────────

func gglStateDirDefault() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "~"
	}
	return gglPyJoin(home, ".zerg", "state")
}

func gglDefaultSource() string {
	sd := os.Getenv("ZERG_STATE_DIR")
	if sd == "" {
		sd = gglStateDirDefault()
	}
	return gglPyJoin(sd, gglSourceName)
}

func gglReadSourceRows(path string) ([]map[string]interface{}, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "真源读不到：" + gglPyOSErrMsg(path, err)
	}
	if !utf8.Valid(b) {
		return nil, "真源不是合法 UTF-8（fail-closed）"
	}
	rows := []map[string]interface{}{} // ★ 非 nil 空表：空 ≠ 取不到（nil 只表「取不到」）
	for i, ln := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			return nil, fmt.Sprintf("真源第 %d 行不是 JSON（账面解读不了 ⇒ 不给结论）：%s", i+1, err.Error())
		}
		rows = append(rows, m)
	}
	return rows, ""
}

// gglTruthy —— py 真值（JSON 值）。
func gglTruthy(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case []interface{}:
		return len(x) > 0
	case map[string]interface{}:
		return len(x) > 0
	}
	return true
}

type gglQid struct {
	enabled   bool
	total     int
	withQid   int
	badFormat [][2]string
	dup       [][2]interface{} // [qid, []string ids]
	rc        int              // -1 = None
	probe     string
	problem   string
}

func gglQidValidate(rows []map[string]interface{}) gglQid {
	var pairs [][2]interface{}
	for _, r := range rows {
		if v, ok := r["qid"]; ok && gglTruthy(v) {
			id := ""
			if iv, ok2 := r["id"]; ok2 {
				id = gglPyStr(iv)
			}
			pairs = append(pairs, [2]interface{}{id, v})
		}
	}
	res := gglQid{enabled: len(pairs) > 0, total: len(rows), withQid: len(pairs), rc: -1}
	if len(pairs) == 0 {
		return res
	}
	seen := map[string][]string{}
	var order []string
	for _, p := range pairs {
		rid := p[0].(string)
		q := gglPyStr(p[1])
		if !gglQIDRE.MatchString(q) {
			res.badFormat = append(res.badFormat, [2]string{rid, q})
		}
		if _, ok := seen[q]; !ok {
			order = append(order, q)
		}
		seen[q] = append(seen[q], rid)
	}
	sort.Strings(order)
	for _, q := range order {
		if len(seen[q]) > 1 {
			res.dup = append(res.dup, [2]interface{}{q, seen[q]})
		}
	}
	if len(res.badFormat) > 0 || len(res.dup) > 0 {
		res.rc = 1
	} else {
		res.rc = 0
	}
	return res
}

// gglPyStr —— py `str(x)`（JSON 值）：字符串原样；None ⇒ "None"；bool ⇒ True/False；数字 ⇒ 十进制。
func gglPyStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	case string:
		return x
	}
	return fmt.Sprintf("%v", v)
}

func gglQidVerdict(q gglQid) string {
	if !q.enabled {
		return fmt.Sprintf("qid 字段未启用 · 0/%d 条带 qid（未启用态不判红）", q.total)
	}
	var bad []string
	for _, b := range q.badFormat {
		bad = append(bad, b[0]+"→"+b[1])
	}
	var dups []string
	for _, d := range q.dup {
		ids := d[1].([]string)
		dups = append(dups, d[0].(string)+"×"+strconv.Itoa(len(ids)))
	}
	if q.rc == 0 {
		return fmt.Sprintf("qid 面过 · %d/%d 条带 qid（形态合法 · 号唯一）", q.withQid, q.total)
	}
	bs := "-"
	if len(bad) > 0 {
		n := len(bad)
		if n > 8 {
			n = 8
		}
		bs = strings.Join(bad[:n], " ")
	}
	ds := "-"
	if len(dups) > 0 {
		n := len(dups)
		if n > 8 {
			n = 8
		}
		ds = strings.Join(dups[:n], " ")
	}
	return fmt.Sprintf("qid 面判红 · 形态非法 %d 处（%s）· 号重复 %d 处（%s）", len(bad), bs, len(dups), ds)
}

func gglQidProbe(source string) gglQid {
	path := gglPyAbspath(source)
	if source == "" {
		path = gglPyAbspath(gglDefaultSource())
	}
	rows, err := gglReadSourceRows(path)
	if err != "" {
		return gglQid{enabled: false, total: 0, withQid: 0, rc: -1, probe: path, problem: err}
	}
	q := gglQidValidate(rows)
	q.probe = path
	return q
}

// ── 有序 JSON（py json.dumps ensure_ascii=False · 默认分隔符）────────────────

type gglKV struct{ k, v string }

func gglJStr(s string) string {
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

func gglJObj(kv []gglKV) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, p := range kv {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(gglJStr(p.k))
		b.WriteString(": ")
		b.WriteString(p.v)
	}
	b.WriteByte('}')
	return b.String()
}

func gglJArr(items []string) string {
	return "[" + strings.Join(items, ", ") + "]"
}

func gglJInt(n int) string { return strconv.Itoa(n) }
func gglJBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
func gglJNull() string { return "null" }

// gglQidJSON —— qid 的 meta 形状（enabled/with_qid/total/rc/probe）。
func gglQidJSON(q gglQid) string {
	rc := "null"
	if q.rc >= 0 {
		rc = strconv.Itoa(q.rc)
	}
	return gglJObj([]gglKV{
		{"enabled", gglJBool(q.enabled)},
		{"with_qid", gglJInt(q.withQid)},
		{"total", gglJInt(q.total)},
		{"rc", rc},
		{"probe", gglJStr(q.probe)},
	})
}

// gglPyOSErrMsg —— 复刻 py OSError 文案 `[Errno N] <strerror>: '<path>'`。
func gglPyOSErrMsg(path string, err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", path)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Sprintf("[Errno 13] Permission denied: '%s'", path)
	}
	if st, e := os.Stat(path); e == nil && st.IsDir() {
		return fmt.Sprintf("[Errno 21] Is a directory: '%s'", path)
	}
	return fmt.Sprintf("[Errno 5] Input/output error: '%s'", path)
}

// ── 自检（合成夹具 · 不碰真源/真账/真文档仓）──────────────────────────────

// gglNone —— py `None` 的 repr 载体（本件只用于自检输出）。
type gglNone struct{}

// gglTup —— py tuple 的 repr 载体（本件只用于自检输出）。
type gglTup struct{ items []interface{} }

func gglQidRcVal(rc int) interface{} {
	if rc < 0 {
		return gglNone{}
	}
	return rc
}

func gglPairList(pairs [][2]string) []interface{} {
	out := make([]interface{}, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, gglTup{[]interface{}{p[0], p[1]}})
	}
	return out
}

func gglDupList(dup [][2]interface{}) []interface{} {
	out := make([]interface{}, 0, len(dup))
	for _, d := range dup {
		ids := d[1].([]string)
		idl := make([]interface{}, 0, len(ids))
		for _, s := range ids {
			idl = append(idl, s)
		}
		out = append(out, gglTup{[]interface{}{d[0].(string), idl}})
	}
	return out
}

// gglRepr —— 复刻 py `%r`（本件只需覆盖 self_test 用到的值形）。
func gglRepr(v interface{}) string {
	switch x := v.(type) {
	case string:
		s := strings.ReplaceAll(x, "\\", "\\\\")
		s = strings.ReplaceAll(s, "'", "\\'")
		return "'" + s + "'"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	case gglNone:
		return "None"
	case gglTup:
		parts := make([]string, 0, len(x.items))
		for _, it := range x.items {
			parts = append(parts, gglRepr(it))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case []interface{}:
		parts := make([]string, 0, len(x))
		for _, it := range x {
			parts = append(parts, gglRepr(it))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return fmt.Sprintf("%v", v)
}

func gglSelfTest(stdout io.Writer) int {
	ok := true
	caseFn := func(name string, got, want interface{}) {
		g, w := gglRepr(got), gglRepr(want)
		good := g == w
		ok = ok && good
		mark := "✓"
		if !good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（got=%s · want=%s）\n", mark, name, g, w)
	}
	tmp, err := os.MkdirTemp("", "gap-ledger-selftest-")
	if err != nil {
		fmt.Fprintln(stdout, "自检结论: **有断言失败**")
		return 1
	}
	defer os.RemoveAll(tmp)
	docs := gglPyJoin(tmp, gglDocsDirname)
	for _, rel := range []string{"v2.5.12/缺口总账-20260923.md", "v2.5.9/缺口总账-20260901.md",
		"v2.5.13/缺口总账-20260925.md", "v2.5.12/承接自-v2.5.11/缺口总账-20260922.md"} {
		p := gglPyJoin(docs, gglLedgerDir, rel)
		_ = os.MkdirAll(gglParent(p), 0o755)
		_ = os.WriteFile(p, []byte("# x\n"), 0o644)
	}
	if v, okp := gglPickVersionDir(docs); okp {
		caseFn("版本线挑选（v2.5.13 > v2.5.9 · 豁免件不算）", v, "v2.5.13")
	} else {
		caseFn("版本线挑选（v2.5.13 > v2.5.9 · 豁免件不算）", gglNone{}, "v2.5.13")
	}
	rel := gglPyRelpath(gglTargetPath(docs, "v2.5.13", "20260926"), docs)
	caseFn("落点形状", rel, gglPyJoin(gglLedgerDir, "v2.5.13", "缺口总账-20260926.md"))
	if d, okd := gglExportDate("## 缺口真源导出（**只列未闭**） · 2026-09-26\n"); okd {
		caseFn("日期取读（首行称日期）", d, "20260926")
	} else {
		caseFn("日期取读（首行称日期）", gglNone{}, "20260926")
	}
	if d, okd := gglExportDate("## 无日期行\n"); okd {
		caseFn("日期取读（取不到 ⇒ None · 不给结论）", d, gglNone{})
	} else {
		caseFn("日期取读（取不到 ⇒ None · 不给结论）", gglNone{}, gglNone{})
	}
	if v, okp := gglPickVersionDir(gglPyJoin(tmp, "没有此目录")); okp {
		caseFn("空文档仓 ⇒ 版本线判不出", v, gglNone{})
	} else {
		caseFn("空文档仓 ⇒ 版本线判不出", gglNone{}, gglNone{})
	}
	q := gglQidValidate([]map[string]interface{}{{"id": "GAP-20260101-1"}, {"id": "GAP-20260101-2"}})
	caseFn("qid 未启用 ⇒ enabled=False · rc=None（不判红）",
		gglTup{[]interface{}{q.enabled, gglQidRcVal(q.rc)}}, gglTup{[]interface{}{false, gglNone{}}})
	q = gglQidValidate([]map[string]interface{}{{"id": "a", "qid": "Q-001"}, {"id": "b", "qid": "Q-002"}, {"id": "c"}})
	caseFn("qid 合法 ⇒ rc=0 · 计 2/3",
		gglTup{[]interface{}{q.enabled, q.rc, q.withQid}}, gglTup{[]interface{}{true, 0, 2}})
	q = gglQidValidate([]map[string]interface{}{{"id": "a", "qid": "Q1"}, {"id": "b", "qid": "q-002"}})
	caseFn("qid 形态非法 ⇒ rc=1 · 两处点名",
		gglTup{[]interface{}{q.rc, gglPairList(q.badFormat)}},
		gglTup{[]interface{}{1, gglPairList([][2]string{{"a", "Q1"}, {"b", "q-002"}})}})
	q = gglQidValidate([]map[string]interface{}{{"id": "a", "qid": "Q-007"}, {"id": "b", "qid": "Q-007"}})
	caseFn("qid 重号 ⇒ rc=1 · 点名 Q-007×2",
		gglTup{[]interface{}{q.rc, gglDupList(q.dup)}},
		gglTup{[]interface{}{1, []interface{}{gglTup{[]interface{}{"Q-007", []interface{}{"a", "b"}}}}}})
	bad := gglPyJoin(tmp, "坏真源.jsonl")
	_ = os.WriteFile(bad, []byte("{\"id\":\"x\"}\n不是 json\n"), 0o644)
	rows, e := gglReadSourceRows(bad)
	caseFn("非 JSON 行 ⇒ 不给结论（rows=None · 有说明）",
		gglTup{[]interface{}{rows == nil, strings.Contains(e, "不给结论")}}, gglTup{[]interface{}{true, true}})
	rows2, e2 := gglReadSourceRows(gglPyJoin(tmp, "没有此件.jsonl"))
	caseFn("真源不存在 ⇒ 不给结论",
		gglTup{[]interface{}{rows2 == nil, e2 != ""}}, gglTup{[]interface{}{true, true}})
	tail := "自检结论: 全过"
	if !ok {
		tail = "自检结论: **有断言失败**"
	}
	fmt.Fprintln(stdout, tail)
	if ok {
		return 0
	}
	return 1
}

// ── argparse 近似 ───────────────────────────────────────────────────────

type gglArgs struct {
	emit, check, dryRun, qidCheck, jsonOut, selfTest bool
	version, help                                    bool
	docsRoot, versionDir, source                     string
}

var gglLongOpts = []string{"--emit", "--check", "--dry-run", "--qid-check", "--docs-root",
	"--version-dir", "--source", "--json", "--self-test", "--version", "--help"}

var gglValueOpts = map[string]bool{"--docs-root": true, "--version-dir": true, "--source": true}

// gglResolve —— argparse 精确/唯一前缀解析：返回 (规范名, 是否歧义, 歧义候选)。
func gglResolve(name string) (string, bool, []string) {
	for _, o := range gglLongOpts {
		if o == name {
			return o, false, nil
		}
	}
	var hits []string
	for _, o := range gglLongOpts {
		if strings.HasPrefix(o, name) {
			hits = append(hits, o)
		}
	}
	if len(hits) == 1 {
		return hits[0], false, nil
	}
	if len(hits) > 1 {
		return "", true, hits
	}
	return "", false, nil
}

// gglParseArgs —— 返回 (args, errMsg)。errMsg 非空 ⇒ 走 argparse 错误档（usage + error，rc=2）。
func gglParseArgs(argv []string) (gglArgs, string) {
	var a gglArgs
	var unrec []string
	noMore := false
	fail := func(msg string) (gglArgs, string) { return a, msg }
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if noMore {
			unrec = append(unrec, tok)
			continue
		}
		if tok == "--" {
			noMore = true
			continue
		}
		if tok == "-h" {
			a.help = true
			continue
		}
		if strings.HasPrefix(tok, "--") {
			name, val := tok, ""
			hasEq := false
			if eq := strings.Index(tok, "="); eq >= 0 {
				name, val, hasEq = tok[:eq], tok[eq+1:], true
			}
			resolved, ambiguous, hits := gglResolve(name)
			if ambiguous {
				return fail(fmt.Sprintf("ambiguous option: %s could match %s", name, strings.Join(hits, ", ")))
			}
			if resolved == "" {
				unrec = append(unrec, tok)
				continue
			}
			if resolved == "--help" {
				a.help = true
				continue
			}
			if gglValueOpts[resolved] {
				if hasEq {
					switch resolved {
					case "--docs-root":
						a.docsRoot = val
					case "--version-dir":
						a.versionDir = val
					case "--source":
						a.source = val
					}
					continue
				}
				if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
					i++
					switch resolved {
					case "--docs-root":
						a.docsRoot = argv[i]
					case "--version-dir":
						a.versionDir = argv[i]
					case "--source":
						a.source = argv[i]
					}
					continue
				}
				return fail(fmt.Sprintf("argument %s: expected one argument", resolved))
			}
			// flag 型
			if hasEq {
				return fail(fmt.Sprintf("argument %s: ignored explicit argument '%s'", resolved, val))
			}
			switch resolved {
			case "--emit":
				a.emit = true
			case "--check":
				a.check = true
			case "--dry-run":
				a.dryRun = true
			case "--qid-check":
				a.qidCheck = true
			case "--json":
				a.jsonOut = true
			case "--self-test":
				a.selfTest = true
			case "--version":
				a.version = true
			}
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			unrec = append(unrec, tok)
			continue
		}
		unrec = append(unrec, tok)
	}
	if len(unrec) > 0 {
		return fail("unrecognized arguments: " + strings.Join(unrec, " "))
	}
	return a, ""
}

// ── 入口编排 ───────────────────────────────────────────────────────────

func gglErrUsage(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, gglUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", gglSelfName, msg)
	return 2
}

// gglBail —— py bail()：具名「不给结论（rc=2）」＋ 逐枚读数行；`--json` 档出 bail JSON。
func gglBail(stdout io.Writer, asJSON bool, mode string, docs, ver, exportBin string, what string, read []gglKV) int {
	if asJSON {
		verJ := "null"
		if ver != "" {
			verJ = gglJStr(ver)
		}
		readJ := gglJObj(read)
		obj := gglJObj([]gglKV{
			{"schema", gglJStr("zerg/v1")},
			{"kind", gglJStr("GapLedgerGen")},
			{"tool", gglJStr(gglVersion)},
			{"items", gglJArr([]string{gglJObj([]gglKV{{"rule", gglJStr("R0")}, {"state", gglJStr("problem")}})})},
			{"meta", gglJObj([]gglKV{
				{"mode", gglJStr(mode)},
				{"docs_root", gglJStr(docs)},
				{"version_dir", verJ},
				{"export_bin", gglJStr(exportBin)},
			})},
			{"reds", gglJArr(nil)},
			{"problems", gglJArr([]string{gglJObj([]gglKV{{"rule", gglJStr("R0")}, {"what", gglJStr(what)}, {"read", readJ}})})},
			{"reports", gglJArr(nil)},
			{"warnings", gglJArr(nil)},
			{"truncated", gglJBool(false)},
		})
		fmt.Fprintln(stdout, obj)
		return 2
	}
	fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", what)
	for _, p := range read {
		switch p.k {
		case "cmd", "msg", "stderr", "path":
			val := strings.ReplaceAll(p.v, "\n", " ")
			if len([]rune(val)) > 200 {
				val = string([]rune(val)[:200])
			}
			fmt.Fprintf(stdout, "   %s: %s\n", p.k, val)
		}
	}
	return 2
}

func gglQidCheckMode(a gglArgs) int {
	src := a.source
	if src == "" {
		src = gglDefaultSource()
	}
	path := gglPyAbspath(src)
	rows, err := gglReadSourceRows(path)
	if err != "" {
		if a.jsonOut {
			obj := gglJObj([]gglKV{
				{"schema", gglJStr("zerg/v1")},
				{"kind", gglJStr("GapQidCheck")},
				{"tool", gglJStr(gglVersion)},
				{"items", gglJArr([]string{gglJObj([]gglKV{{"rule", gglJStr("R2")}, {"state", gglJStr("problem")}})})},
				{"meta", gglJObj([]gglKV{{"source", gglJStr(path)}})},
				{"reds", gglJArr(nil)},
				{"problems", gglJArr([]string{gglJObj([]gglKV{{"rule", gglJStr("R2")}, {"what", gglJStr(err)}, {"read", gglJObj(nil)}})})},
				{"reports", gglJArr(nil)},
				{"warnings", gglJArr(nil)},
				{"truncated", gglJBool(false)},
			})
			fmt.Fprintln(os.Stdout, obj)
		} else {
			fmt.Fprintf(os.Stdout, "⛔ 不给结论（rc=2）：%s\n", err)
		}
		return 2
	}
	q := gglQidValidate(rows)
	rc := 0
	if q.enabled {
		rc = q.rc
	}
	if a.jsonOut {
		itemState := "ok"
		if rc == 1 {
			itemState = "red"
		}
		reds := gglJArr(nil)
		if rc == 1 {
			reds = gglJArr([]string{gglJObj([]gglKV{
				{"rule", gglJStr("R2")},
				{"what", gglJStr(gglQidVerdict(q))},
				{"read", gglJObj([]gglKV{{"bad_format", gglJStr(gglPyStrListPairs(q.badFormat))}, {"dup", gglJStr(gglPyStrDup(q.dup))}})},
			})})
		}
		obj := gglJObj([]gglKV{
			{"schema", gglJStr("zerg/v1")},
			{"kind", gglJStr("GapQidCheck")},
			{"tool", gglJStr(gglVersion)},
			{"items", gglJArr([]string{gglJObj([]gglKV{{"rule", gglJStr("R2")}, {"state", gglJStr(itemState)}})})},
			{"meta", gglJObj([]gglKV{
				{"source", gglJStr(path)},
				{"total", gglJInt(q.total)},
				{"with_qid", gglJInt(q.withQid)},
				{"enabled", gglJBool(q.enabled)},
			})},
			{"reds", reds},
			{"problems", gglJArr(nil)},
			{"reports", gglJArr([]string{gglJObj([]gglKV{{"rule", gglJStr("R2")}, {"what", gglJStr(gglQidVerdict(q))}, {"read", gglJObj(nil)}})})},
			{"warnings", gglJArr(nil)},
			{"truncated", gglJBool(false)},
		})
		fmt.Fprintln(os.Stdout, obj)
		return rc
	}
	fmt.Fprintln(os.Stdout, "── qid 读面（GAP-20260927-122 / -130）──")
	fmt.Fprintf(os.Stdout, "  真源   : %s\n", path)
	mark := "✓"
	if rc != 0 {
		mark = "✗"
	}
	fmt.Fprintf(os.Stdout, "%s %s\n", mark, gglQidVerdict(q))
	return rc
}

func gglPyStrListPairs(pairs [][2]string) string {
	items := make([]string, 0, len(pairs))
	for _, p := range pairs {
		items = append(items, "("+gglJStr(p[0])+", "+gglJStr(p[1])+")")
	}
	return "[" + strings.Join(items, ", ") + "]"
}

func gglPyStrDup(dup [][2]interface{}) string {
	items := make([]string, 0, len(dup))
	for _, d := range dup {
		ids := d[1].([]string)
		idItems := make([]string, 0, len(ids))
		for _, s := range ids {
			idItems = append(idItems, gglJStr(s))
		}
		items = append(items, "("+gglJStr(d[0].(string))+", ["+strings.Join(idItems, ", ")+"])")
	}
	return "[" + strings.Join(items, ", ") + "]"
}

// gglMain —— py main()（根显式传入：仓根 · 便于对拍/单测，✗ 不依赖 cwd）。
func gglMain(stdout, stderr io.Writer, a gglArgs, repoRoot string) int {
	mode := "check"
	if a.emit {
		mode = "emit"
	} else if a.check {
		mode = "check"
	} else if a.dryRun {
		mode = "dry"
	}
	exportBin := gglExportBin(repoRoot)
	docs := ""
	if a.docsRoot != "" {
		docs = gglPyAbspath(a.docsRoot)
	} else {
		docs = gglPyAbspath(gglPyJoin(gglParent(repoRoot), gglDocsDirname))
	}
	ver := a.versionDir
	if ver == "" {
		if st, err := os.Stat(docs); err == nil && st.IsDir() {
			if v, ok := gglPickVersionDir(docs); ok {
				ver = v
			}
		}
	}
	st, err := os.Stat(docs)
	if err != nil || !st.IsDir() {
		return gglBail(stdout, a.jsonOut, mode, docs, ver, exportBin,
			fmt.Sprintf("文档仓根不在盘：`%s`（不拿「读不到」当「没问题」）", docs),
			[]gglKV{{"path", docs}})
	}
	if ver == "" {
		return gglBail(stdout, a.jsonOut, mode, docs, ver, exportBin,
			"文档仓里一件 `缺口总账-*.md` 都没有 ⇒ 版本线目录判不出（不给结论）",
			[]gglKV{{"glob", gglPyJoin(docs, gglLedgerDir, "<版>", "缺口总账-<日期>.md")}})
	}
	status, out, eread := gglRunExport(repoRoot)
	if status != "ok" {
		read := []gglKV{}
		for _, k := range []string{"cmd", "msg", "stderr", "rc"} {
			if v, ok := eread[k]; ok {
				read = append(read, gglKV{k, v})
			}
		}
		return gglBail(stdout, a.jsonOut, mode, docs, ver, exportBin,
			fmt.Sprintf("`zerg gap export` 跑不动（%s）⇒ 直出取不到（不许当绿）", status), read)
	}
	date8, ok := gglExportDate(out)
	if !ok {
		return gglBail(stdout, a.jsonOut, mode, docs, ver, exportBin,
			"export 产出里取不到日期行（首行形状变了？）⇒ 文件名落点判不出",
			[]gglKV{{"bytes", strconv.Itoa(len([]byte(out)))}})
	}
	tgt := gglTargetPath(docs, ver, date8)
	want := []byte(out)
	has, hasOK := gglReadBytes(tgt)
	drift := !hasOK || !bytes.Equal(has, want)

	q := gglQidProbe(a.source)

	verdict := ""
	rc := 0
	if mode == "emit" {
		switch {
		case !hasOK:
			verdict = "落盘（新件）"
		case bytes.Equal(has, want):
			verdict = "落盘（逐字节相同 · 重生成无变化）"
		default:
			verdict = fmt.Sprintf("落盘（覆盖漂移件 · 旧件 %d 字节 ⇒ 新件 %d 字节）", len(has), len(want))
		}
		if !gglWriteAtomic(tgt, want) {
			return gglBail(stdout, a.jsonOut, mode, docs, ver, exportBin,
				fmt.Sprintf("写后读回不一致：`%s`", tgt), []gglKV{{"path", tgt}})
		}
		rc = 0
	} else if drift {
		var detail string
		if !hasOK {
			detail = "不在盘"
		} else {
			detail = fmt.Sprintf("与直出不符（件 %d 字节 vs 直出 %d 字节）", len(has), len(want))
		}
		verdict = fmt.Sprintf("生成件%s ⇒ 判红（漂移）", detail)
		rc = 1
	} else {
		verdict = fmt.Sprintf("生成件 == 直出（逐字节相同 · %d 字节）", len(want))
		rc = 0
	}

	qidRC := -1
	if q.enabled && q.rc == 1 {
		qidRC = 1
		if mode == "check" {
			rc = 1
		}
	}
	rel := gglPyRelpath(tgt, gglParent(docs))

	echo := func(s string) { fmt.Fprintln(stdout, s) }
	if a.jsonOut {
		var reds []string
		if drift && mode != "emit" {
			reds = append(reds, gglJObj([]gglKV{
				{"rule", gglJStr("R1")}, {"what", gglJStr(verdict)},
				{"read", gglJObj([]gglKV{{"target", gglJStr(tgt)}})},
			}))
		}
		qidItemState := "report"
		if qidRC == 1 {
			qidItemState = "red"
		} else if q.enabled && q.rc == 0 {
			qidItemState = "ok"
		}
		if qidRC == 1 && mode == "check" {
			reds = append(reds, gglJObj([]gglKV{{"rule", gglJStr("R2")}, {"what", gglJStr(gglQidVerdict(q))}, {"read", gglJObj([]gglKV{{"qid", gglJObj([]gglKV{{"enabled", gglJBool(q.enabled)}, {"with_qid", gglJInt(q.withQid)}, {"total", gglJInt(q.total)}, {"rc", gglQidRC(q.rc)}, {"bad_format", gglPyStrListPairs(q.badFormat)}, {"dup", gglPyStrDup(q.dup)}})}})}}))
		}
		var problems []string
		if q.problem != "" {
			problems = append(problems, gglJObj([]gglKV{{"rule", gglJStr("R2")}, {"what", gglJStr(q.problem)}, {"read", gglJObj(nil)}}))
		}
		obj := gglJObj([]gglKV{
			{"schema", gglJStr("zerg/v1")},
			{"kind", gglJStr("GapLedgerGen")},
			{"tool", gglJStr(gglVersion)},
			{"items", gglJArr([]string{
				gglJObj([]gglKV{{"rule", gglJStr("R0")}, {"state", gglJStr(gglItemState(rc))}}),
				gglJObj([]gglKV{{"rule", gglJStr("R2")}, {"state", gglJStr(qidItemState)}}),
			})},
			{"meta", gglJObj([]gglKV{
				{"mode", gglJStr(mode)},
				{"docs_root", gglJStr(docs)},
				{"version_dir", gglJStr(ver)},
				{"target", gglJStr(tgt)},
				{"rel", gglJStr(rel)},
				{"bytes", gglJInt(len(want))},
				{"export_bin", gglJStr(exportBin)},
				{"export_cmd", gglJStr(eread["cmd"])},
				{"qid", gglQidJSON(q)},
			})},
			{"reds", gglJArr(reds)},
			{"problems", gglJArr(problems)},
			{"reports", gglJArr([]string{
				gglJObj([]gglKV{{"rule", gglJStr("-")}, {"what", gglJStr(verdict)}, {"read", gglJObj(nil)}}),
				gglJObj([]gglKV{{"rule", gglJStr("R2")}, {"what", gglJStr(gglQidVerdict(q))}, {"read", gglJObj(nil)}}),
			})},
			{"warnings", gglJArr(nil)},
			{"truncated", gglJBool(false)},
		})
		echo(obj)
		return rc
	}
	echo("── 结转账生成件（" + gglVersion + " · 设计稿 v1.3 §0 第 ③ 序）──")
	echo("  导出器 : " + exportBin + " gap export（**现跑**）")
	echo("  文档仓 : " + docs)
	echo(fmt.Sprintf("  落点   : %s（%d 字节 · 版本线 %s）", rel, len(want), ver))
	sub := "只判/只打 · 不改任何件"
	if mode == "emit" {
		sub = "写盘"
	}
	echo(fmt.Sprintf("  模式   : %s（%s）", mode, sub))
	mark := "✓"
	if rc != 0 {
		mark = "✗"
	}
	echo(mark + " " + verdict)
	echo(fmt.Sprintf("  qid    : %s（真源 `%s`）", gglQidVerdict(q), q.probe))
	return rc
}

func gglQidRC(rc int) string {
	if rc < 0 {
		return "null"
	}
	return strconv.Itoa(rc)
}

func gglItemState(rc int) string {
	if rc == 1 {
		return "red"
	}
	return "ok"
}

// gglParent —— posixpath.dirname（剥尾斜杠）。
func gglParent(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	if head == "" {
		return "."
	}
	return head
}

// ── 自检（合成夹具 · 不碰真源/真账/真文档仓）──────────────────────────────

func gglRun(stdout, stderr io.Writer, argv []string) int {
	repoRoot := gglRepoRoot()
	return gglDispatch(stdout, stderr, argv, repoRoot)
}

func gglDispatch(stdout, stderr io.Writer, argv []string, repoRoot string) int {
	a, errMsg := gglParseArgs(argv)
	if errMsg != "" {
		return gglErrUsage(stderr, errMsg)
	}
	if a.help {
		fmt.Fprint(stdout, gglHelpText)
		return 0
	}
	if a.version {
		fmt.Fprintln(stdout, gglVersion)
		return 0
	}
	if a.selfTest {
		return gglSelfTest(stdout)
	}
	if a.qidCheck {
		return gglQidCheckMode(a)
	}
	return gglMain(stdout, stderr, a, repoRoot)
}

// genGapLedgerGateMain —— 本件入口（inproc 口径：argv **不含程序名**）。
func genGapLedgerGateMain(stdout, stderr io.Writer, argv []string) int {
	return gglRun(stdout, stderr, argv)
}
