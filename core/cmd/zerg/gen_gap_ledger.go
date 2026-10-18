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
// ★ 2026-10-17（A 案 · `GAP-20261017-16`）**判据口径改成「生成件族」**：`zerg gap export --out <目录>`
// 落的不是单件直出，而是「拆件索引（1.3~1.6KB）+ 每面每 120 条一页」的件族（真源 `family_gap_export.go`
// 的 `gapExportWriteFiles`）⇒ 旧判据「件字节 == 单件直出字节」与 `--out` 的产出**结构上不可相等**，
// `--check` 恒红（实测「件 1367 字节 vs 直出 26360 字节」；历史对照 2026-09-27 那期 1637B 索引 + 9 页同形）。
// 现判据 = G1 索引在盘 / G2 索引日期 == 直出日期 / G3 面集相等 / G4 逐页件在盘且「索引声明 ⟷ 页件头 ⟷
// 条目数」三方相符 / G5 索引内部闭合（Σ页条目 == 未闭 == 各面声明 == 页脚）/ G6 对真源只判「多于」（手改）
// 而「少于」只报白并归因（派生量不设红灯 · 追平口径归只报告档 门63）。详见本件「生成件族判据」节。
// 写面（`--emit`）同步改成**走唯一拆件写者** `bin/zerg gap export --out <目录> --yes`（不再自己写单件直出）。
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
	// 账本件名不再本件自写：族名口径见 ledger_family.go（ledgerFamilySplit · 单件名只是
	// 族里「后缀=空」那一件）。GAP-20260927-323：旧读法 `^缺口总账-[\p{Nd}]{8}\.md$` 只认单件名
	// ⇒ 一套逻辑产出的族里，页件与「无索引件的族」整族不可见。
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

// gglCand —— 一件族成员（ver = 版本线目录名 · rel = 文档仓相对路径 · key = 族键
// `缺口总账-<日期>` · kind = index/page/member）。
type gglCand struct{ ver, rel, key, kind string }

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
				// 族名口径走共用规则：单件名只是族里「后缀=空」那一件，页件 / `-页` / `-1..-9` 同属一族。
				piece, isFam := ledgerFamilySplit(fn)
				if !isFam {
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
				out = append(out, gglCand{ver: ver, rel: rel, key: piece.Key, kind: piece.Kind})
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

// gglLedgerFamily —— 一族账本件（族键下的全部成员 · 旧读法只看得到最后一件）。
type gglLedgerFamily struct {
	Ver     string
	Key     string
	Members []gglCand // 按 rel 排序
}

// gglCandLess —— 挑最新族/最新件的同一把尺子：(版本线 vkey, 族键日期, 族键, 相对件名)。
func gglCandLess(a, b gglCand) bool {
	av, bv := gglVkey(a.ver), gglVkey(b.ver)
	if gglVerKeyLess(av, bv) {
		return true
	}
	if gglVerKeyLess(bv, av) {
		return false
	}
	ad, bd := ledgerFamilyDateKey(a.key), ledgerFamilyDateKey(b.key)
	if ad != bd {
		return ad < bd
	}
	if a.key != b.key {
		return a.key < b.key
	}
	return a.rel < b.rel
}

// gglPickFamily —— 现读最新的「账本族」并交出**族里每一件**（见 ledger_family.go 的族名口径）。
// 旧读法 `cand.sort(key=vkey)[-1]` 只挑一件（且候选只收单件名）⇒ 族里其余件不可见。
// needIndex=true ⇒ 只挑「族里有索引件（后缀=空）」的族：落点要写索引件，✗ 不拿页件族当落点。
func gglPickFamily(docs string, needIndex bool) (gglLedgerFamily, bool) {
	cands := gglLedgerCandidates(docs)
	best := -1
	for i := range cands {
		if needIndex && cands[i].kind != "index" {
			continue
		}
		if best < 0 || gglCandLess(cands[best], cands[i]) {
			best = i
		}
	}
	if best < 0 {
		return gglLedgerFamily{}, false
	}
	fam := gglLedgerFamily{Ver: cands[best].ver, Key: cands[best].key}
	for _, c := range cands {
		if c.ver == fam.Ver && c.key == fam.Key {
			fam.Members = append(fam.Members, c)
		}
	}
	sort.SliceStable(fam.Members, func(i, j int) bool { return fam.Members[i].rel < fam.Members[j].rel })
	return fam, true
}

// gglPickVersionDir —— 现读含结转账的最新版本线；读不到 ⇒ ("", false)。
// 签名与调用点不变（✗ 不再自己挑单件），实现改由 gglPickFamily 的族挑选给出。
func gglPickVersionDir(docs string) (string, bool) {
	if fam, ok := gglPickFamily(docs, true); ok {
		return fam.Ver, true
	}
	return "", false
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

// ═══════════════════ 生成件族（索引 1 + 页 N）判据（A 案 · GAP-20261017-16）═══════════════════
//
// 口实：`zerg gap export --out <目录>` 落的是「拆件索引 + 每面每 120 条一页」的生成件族
// （真源 `family_gap_export.go` 的 `gapExportWriteFiles`）。索引件是入口（只有全部页件写成才落它），
// 页件名 `缺口总账-<日期>-<面>-pNN.md`。旧判据把「件字节 == 单件直出字节」当唯一形态 ⇒ 与 `--out`
// 的产出**结构上不可相等**（索引 1.3~1.6KB ⟷ 直出 26KB）⇒ `--check` 恒红（`GAP-20261017-16`）。
//
// 现判据（族口径 · 与只报告档 门63 `check-gap-ledger-view` 的 R1 同形态，但本门是**阻断**档）：
//
//	G1 索引件在盘（当期日期 `缺口总账-YYYYMMDD.md`；兼容历史命名 `缺口总账-YYYY-MM-DD.md`）—— 不在 ⇒ 红
//	G2 索引头日期 == 现跑直出日期 —— 不等 ⇒ 红
//	G3 索引 `> 按面` 的面集 == 直出面集（六值闭集逐面）—— 不等 ⇒ 红
//	G4 索引点名的每一页 ⇒ 页件在同一目录在盘 · 页件头（面 / 日期 / 本面共 N 条）与索引逐项相符 ·
//	   页内条目数 == 本页声明 == 索引区间长度 ⇒ 任一不符 ⇒ 红
//	G5 内部闭合：Σ页内条目 == 件头「未闭」== 各面 `### 面（N 条 · P 页）`之和 == `> 按面` 之和
//	   == 页脚「未闭」· 页脚「件数」== 1 + 页数 ⇒ 任一不等 ⇒ 红
//	G6 对真源直出：每面条数 / 未闭 / 真源条数 **多于** 直出 ⇒ 红（多于真源 = 手改或真源回退，
//	   结构性不可能）；**少于** ⇒ 只报白并归因（派生量不设红灯：真源每 `gap add` 一条就 +1，若判红则
//	   本门在每次加缺口后必红，且与门63 R1b「当期是否追平」的只报告口径重复）。
//
// ★ 六值面闭集复用 `gapImpactClosed`（`family_gap_export.go` 的真源），✗ 不另抄一份。

// gglLedgerPrefix —— 族名前缀（取自 ledger_family.go 的唯一字面量）。
const gglLedgerPrefix = ledgerFamilyName

var (
	// 索引件首行：`## 缺口真源导出（**只列未闭**） · <YYYYMMDD|YYYY-MM-DD>`
	gglIdxTitleRE = regexp.MustCompile(`^## 缺口真源导出（\*\*只列未闭\*\*）[ \t]*·[ \t]*(\d{8}|\d{4}-\d{2}-\d{2})[ \t]*$`)
	// 件头计数器（索引件与 stdout 直出同一形状）。
	gglIdxHdrRE = regexp.MustCompile(`真源条数 \*\*(\d+)\*\* · 未闭 \*\*(\d+)\*\* · 已闭 \*\*(\d+)\*\*（其中「不做」\*\*(\d+)\*\*）`)
	// `> 按面：命令面 48 · 门禁面 15 · …`（索引件与 stdout 直出都有这一行）。
	gglIdxFaceRE = regexp.MustCompile(`(?m)^> 按面：(.+)$`)
	gglIdxFaceKV = regexp.MustCompile(`^(\S+) (\d+)$`)
	// 面小节：`### 命令面（48 条 · 1 页）`
	gglIdxSecRE = regexp.MustCompile(`^### ([^（]+)（(\d+) 条 · (\d+) 页）$`)
	// 页声明：`- 第 1 页：`缺口总账-20261017-命令面-p01.md` · 面内 1–48 条`
	gglIdxPageRE = regexp.MustCompile("^- 第 (\\d+) 页：`([^`]+)` · 面内 (\\d+)[–-](\\d+) 条$")
	// 页脚：`（本清册由 CLI 生成 · 未闭 78 条 · 件数 6（索引 1 + 页 5））`
	gglIdxFootRE = regexp.MustCompile(`^（本清册由 CLI 生成 · 未闭 (\d+) 条 · 件数 (\d+)（索引 1 \+ 页 (\d+)））$`)
	// 页件首行：`# 门禁面 · 缺口总账 20261017`
	gglPageHdrRE = regexp.MustCompile(`^# ([^·]+?) · 缺口总账 (\d{8}|\d{4}-\d{2}-\d{2})$`)
	// 页件元行：`> 第 1 页·本页 14 条·面内 1–14 条·本面共 14 条`
	gglPageMetaRE = regexp.MustCompile(`^> 第 (\d+) 页·本页 (\d+) 条·面内 (\d+)[–-](\d+) 条·本面共 (\d+) 条$`)
)

// gglFamilyVerdict —— 族判据结论（Verdict 一行人面 · Red 逐条红因 · Notes 观察值·不判红）。
type gglFamilyVerdict struct {
	OK      bool
	Verdict string
	Red     []string
	Notes   []string
}

// gglPageDecl —— 索引声明的页（G4 的逐页对拍单元）。
type gglPageDecl struct {
	Face   string
	No     int
	Name   string
	Lo, Hi int
}

// gglFamilyRead —— 当期生成件族的现读（索引件解析 + 逐页件读）。Red 非空即结构性不闭合。
type gglFamilyRead struct {
	Path      string
	Bytes     int
	Date      string
	SrcTotal  int
	Open      int
	Closed    int
	NoDecide  int
	FaceCount map[string]int
	SectionN  map[string]int
	SectionP  map[string]int
	Pages     []gglPageDecl
	PageSum   int
	FootOpen  int
	FootFiles int
	FootPages int
	Red       []string
}

// gglDate8 —— `YYYY-MM-DD` 与 `YYYYMMDD` 归一成 date8（两代 `--out` 都用过）。
func gglDate8(s string) string { return strings.ReplaceAll(s, "-", "") }

// gglReadFamily —— 读当期生成件族：索引件先按现行命名找，找不到再试历史命名；都不在 ⇒ nil。
func gglReadFamily(docs, ver, date8, tgt string) *gglFamilyRead {
	dir := gglPyJoin(docs, gglLedgerDir, ver)
	path := tgt
	body, ok := gglReadBytes(path)
	if !ok {
		alt := ""
		if len(date8) == 8 {
			alt = gglPyJoin(dir, gglLedgerPrefix+"-"+date8[:4]+"-"+date8[4:6]+"-"+date8[6:]+".md")
		}
		if alt != "" {
			if b2, ok2 := gglReadBytes(alt); ok2 {
				path, body, ok = alt, b2, true
			}
		}
	}
	if !ok {
		return nil
	}
	return gglParseIndex(path, dir, string(body))
}

// gglParseIndex —— 解析索引件 + 逐页件现读；结构性不符一律进 Red（fail-closed：读不出也是红）。
func gglParseIndex(path, dir, body string) *gglFamilyRead {
	norm := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	f := &gglFamilyRead{Path: path, Bytes: len([]byte(body)),
		FaceCount: map[string]int{}, SectionN: map[string]int{}, SectionP: map[string]int{}}
	lines := strings.Split(norm, "\n")

	// ① 首非空行 = 件头（日期）
	sawLine := false
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		sawLine = true
		if m := gglIdxTitleRE.FindStringSubmatch(strings.TrimRight(ln, "\r")); m != nil {
			f.Date = gglDate8(m[1])
		} else {
			f.Red = append(f.Red, "索引件首行不是 `## 缺口真源导出（**只列未闭**） · <日期>`（不是本门的生成件）")
		}
		break
	}
	if !sawLine {
		f.Red = append(f.Red, "索引件是空件（一个非空行都没有）")
		return f
	}
	// ② 件头计数器
	if m := gglIdxHdrRE.FindStringSubmatch(norm); m != nil {
		f.SrcTotal, _ = strconv.Atoi(m[1])
		f.Open, _ = strconv.Atoi(m[2])
		f.Closed, _ = strconv.Atoi(m[3])
		f.NoDecide, _ = strconv.Atoi(m[4])
	} else {
		f.Red = append(f.Red, "索引件缺件头计数行 `> 真源条数 **N** · 未闭 **N** · 已闭 **N**（其中「不做」**N**）`")
	}
	// ③ `> 按面`
	if m := gglIdxFaceRE.FindStringSubmatch(norm); m != nil {
		for _, seg := range strings.Split(m[1], " · ") {
			kv := gglIdxFaceKV.FindStringSubmatch(strings.TrimSpace(seg))
			if kv == nil {
				f.Red = append(f.Red, fmt.Sprintf("`> 按面` 段写法不合法：%q", strings.TrimSpace(seg)))
				continue
			}
			n, _ := strconv.Atoi(kv[2])
			f.FaceCount[kv[1]] = n
		}
	} else {
		f.Red = append(f.Red, "索引件缺 `> 按面：…` 行（面集无从对拍）")
	}
	// ④ 面小节 + 页声明
	for _, ln := range lines {
		if m := gglIdxSecRE.FindStringSubmatch(strings.TrimRight(ln, "\r")); m != nil {
			n, _ := strconv.Atoi(m[2])
			p, _ := strconv.Atoi(m[3])
			face := strings.TrimSpace(m[1])
			f.SectionN[face] = n
			f.SectionP[face] = p
			continue
		}
		if m := gglIdxPageRE.FindStringSubmatch(strings.TrimRight(ln, "\r")); m != nil {
			no, _ := strconv.Atoi(m[1])
			lo, _ := strconv.Atoi(m[3])
			hi, _ := strconv.Atoi(m[4])
			face := ""
			if i := strings.LastIndex(m[2], "-p"); i > 0 {
				if j := strings.LastIndex(m[2][:i], "-"); j > 0 {
					face = m[2][j+1 : i]
				}
			}
			f.Pages = append(f.Pages, gglPageDecl{Face: face, No: no, Name: m[2], Lo: lo, Hi: hi})
		}
	}
	// ⑤ 页脚（末个非空行；✗ 不用整件 `^…$` —— 无 `(?m)` 时 `^` 只认整件开头）
	foot := ""
	for k := len(lines) - 1; k >= 0; k-- {
		if strings.TrimSpace(lines[k]) != "" {
			foot = strings.TrimRight(lines[k], "\r")
			break
		}
	}
	if m := gglIdxFootRE.FindStringSubmatch(foot); m != nil {
		f.FootOpen, _ = strconv.Atoi(m[1])
		f.FootFiles, _ = strconv.Atoi(m[2])
		f.FootPages, _ = strconv.Atoi(m[3])
	} else {
		f.Red = append(f.Red, "索引件缺页脚 `（本清册由 CLI 生成 · 未闭 N 条 · 件数 N（索引 1 + 页 N））`")
	}
	// ⑥ 逐页件（G4）
	for i := range f.Pages {
		pd := &f.Pages[i]
		pb, ok := gglReadBytes(gglPyJoin(dir, pd.Name))
		if !ok {
			f.Red = append(f.Red, fmt.Sprintf("页件不在盘：`%s`（索引点名了它）", pd.Name))
			continue
		}
		ptxt := strings.ReplaceAll(strings.ReplaceAll(string(pb), "\r\n", "\n"), "\r", "\n")
		hdrFace, hdrDate, faceTotal, pageDeclared, pageNo := "", "", -1, -1, -1
		bullets := 0
		for _, ln := range strings.Split(ptxt, "\n") {
			ln = strings.TrimRight(ln, "\r")
			if hdrFace == "" {
				if m := gglPageHdrRE.FindStringSubmatch(ln); m != nil {
					hdrFace, hdrDate = strings.TrimSpace(m[1]), gglDate8(m[2])
					continue
				}
			}
			if pageNo < 0 {
				if m := gglPageMetaRE.FindStringSubmatch(ln); m != nil {
					pageNo, _ = strconv.Atoi(m[1])
					pageDeclared, _ = strconv.Atoi(m[2])
					faceTotal, _ = strconv.Atoi(m[5])
					continue
				}
			}
			if strings.HasPrefix(ln, "- `GAP-") {
				bullets++
			}
		}
		want := pd.Hi - pd.Lo + 1
		switch {
		case hdrFace == "" || hdrDate != f.Date:
			f.Red = append(f.Red, fmt.Sprintf("页件 `%s` 首行不合（要 `# <面> · 缺口总账 %s`）", pd.Name, f.Date))
		case pageNo < 0:
			f.Red = append(f.Red, fmt.Sprintf("页件 `%s` 缺元行 `> 第 N 页·本页 N 条·面内 a–b 条·本面共 N 条`", pd.Name))
		default:
			if hdrFace != pd.Face {
				f.Red = append(f.Red, fmt.Sprintf("页件 `%s` 面名 %q ≠ 件名面 %q", pd.Name, hdrFace, pd.Face))
			}
			if pageNo != pd.No {
				f.Red = append(f.Red, fmt.Sprintf("页件 `%s` 元行页号 %d ≠ 索引页号 %d", pd.Name, pageNo, pd.No))
			}
			if pageDeclared != want {
				f.Red = append(f.Red, fmt.Sprintf("页件 `%s` 元行「本页 %d 条」≠ 索引区间 %d–%d（%d 条）", pd.Name, pageDeclared, pd.Lo, pd.Hi, want))
			}
			if bullets != want {
				f.Red = append(f.Red, fmt.Sprintf("页件 `%s` 现读条目 %d 条 ≠ 索引区间 %d–%d（%d 条）", pd.Name, bullets, pd.Lo, pd.Hi, want))
			}
			if faceTotal >= 0 {
				if n, ok := f.SectionN[hdrFace]; ok && n != faceTotal {
					f.Red = append(f.Red, fmt.Sprintf("页件 `%s` 「本面共 %d 条」≠ 索引小节 `### %s（%d 条 …）`", pd.Name, faceTotal, hdrFace, n))
				}
			}
		}
		f.PageSum += bullets
	}
	// ⑦ 内部闭合（G5）
	sumSec, sumPages, sumFaces := 0, 0, 0
	for _, n := range f.SectionN {
		sumSec += n
	}
	for _, p := range f.SectionP {
		sumPages += p
	}
	for _, n := range f.FaceCount {
		sumFaces += n
	}
	if len(f.SectionN) > 0 && sumSec != f.Open {
		f.Red = append(f.Red, fmt.Sprintf("各面 `### 面（N 条 …）` 之和 %d ≠ 件头「未闭 %d」", sumSec, f.Open))
	}
	if sumPages != len(f.Pages) {
		f.Red = append(f.Red, fmt.Sprintf("各面声明页数之和 %d ≠ 索引列出的页数 %d", sumPages, len(f.Pages)))
	}
	if len(f.FaceCount) > 0 && sumFaces != f.Open {
		f.Red = append(f.Red, fmt.Sprintf("`> 按面` 六面之和 %d ≠ 件头「未闭 %d」", sumFaces, f.Open))
	}
	if f.PageSum != f.Open {
		f.Red = append(f.Red, fmt.Sprintf("Σ页内条目 %d ≠ 件头「未闭 %d」（漏页 / 重页 / 手改）", f.PageSum, f.Open))
	}
	if f.FootOpen != f.Open {
		f.Red = append(f.Red, fmt.Sprintf("页脚「未闭 %d」≠ 件头「未闭 %d」", f.FootOpen, f.Open))
	}
	if f.FootFiles != len(f.Pages)+1 {
		f.Red = append(f.Red, fmt.Sprintf("页脚「件数 %d」≠ 1 + 页数 %d", f.FootFiles, len(f.Pages)))
	}
	if f.FootPages != len(f.Pages) {
		f.Red = append(f.Red, fmt.Sprintf("页脚「页 %d」≠ 索引列出的页数 %d", f.FootPages, len(f.Pages)))
	}
	return f
}

// gglLiveCounts —— 从现跑直出（stdout 面）取面计数与未闭/真源总数（与索引件同一张件头形状）。
func gglLiveCounts(out string) (map[string]int, int, int) {
	faces := map[string]int{}
	if m := gglIdxFaceRE.FindStringSubmatch(out); m != nil {
		for _, seg := range strings.Split(m[1], " · ") {
			if kv := gglIdxFaceKV.FindStringSubmatch(strings.TrimSpace(seg)); kv != nil {
				n, _ := strconv.Atoi(kv[2])
				faces[kv[1]] = n
			}
		}
	}
	open, src := 0, 0
	if m := gglIdxHdrRE.FindStringSubmatch(out); m != nil {
		src, _ = strconv.Atoi(m[1])
		open, _ = strconv.Atoi(m[2])
	}
	return faces, open, src
}

// gglJudgeFamily —— 族判据主体（G1…G6）。✗ 不比总字节：直出是单件、落盘是族，字节天然不同。
func gglJudgeFamily(f *gglFamilyRead, date8, exportOut string) gglFamilyVerdict {
	v := gglFamilyVerdict{}
	if f == nil {
		v.Red = append(v.Red, "索引件不在盘：`"+gglLedgerPrefix+"-"+date8+".md`（历史命名 `"+gglLedgerPrefix+"-YYYY-MM-DD.md` 亦不在）")
		v.Verdict = "生成件不在盘（索引件与历史命名件都读不到） ⇒ 判红（漂移）"
		return v
	}
	v.Red = append(v.Red, f.Red...)
	if f.Date != date8 {
		v.Red = append(v.Red, fmt.Sprintf("索引头日期 %s ≠ 现跑直出日期 %s（索引不是当期件）", f.Date, date8))
	}
	liveFaces, liveOpen, liveSrc := gglLiveCounts(exportOut)
	for _, face := range gapImpactClosed {
		ic, iok := f.FaceCount[face]
		lc, lok := liveFaces[face]
		if iok != lok {
			v.Red = append(v.Red, fmt.Sprintf("面集不符：`%s` %s索引、%s直出", face, gglPresence(iok), gglPresence(lok)))
			continue
		}
		if ic > lc {
			v.Red = append(v.Red, fmt.Sprintf("面 `%s` 索引 %d 条 > 直出 %d 条（多于真源 = 手改或真源回退）", face, ic, lc))
		} else if ic < lc {
			v.Notes = append(v.Notes, fmt.Sprintf("面 `%s` 索引 %d 条 < 直出 %d 条（真源自上次导出后新增 %d 条 · 追平走 `zerg gap export --out <目录> --yes`）", face, ic, lc, lc-ic))
		}
	}
	for face := range f.FaceCount {
		if _, ok := liveFaces[face]; !ok {
			v.Red = append(v.Red, fmt.Sprintf("面集不符：索引多出 `%s`（直出里没有这一面）", face))
		}
	}
	if f.Open > liveOpen {
		v.Red = append(v.Red, fmt.Sprintf("索引「未闭 %d」> 直出「未闭 %d」（多于真源）", f.Open, liveOpen))
	} else if f.Open < liveOpen {
		v.Notes = append(v.Notes, fmt.Sprintf("索引「未闭 %d」< 直出「未闭 %d」⇒ 索引落后 %d 条（真源新增、非手改；追平归只报告档 门63 R1b）", f.Open, liveOpen, liveOpen-f.Open))
	}
	if f.SrcTotal > liveSrc {
		v.Red = append(v.Red, fmt.Sprintf("索引「真源条数 %d」> 直出「真源条数 %d」（多于真源 = 手改）", f.SrcTotal, liveSrc))
	}
	if len(v.Red) > 0 {
		more := ""
		if len(v.Red) > 1 {
			more = fmt.Sprintf("，共 %d 处", len(v.Red))
		}
		v.Verdict = "生成件族不闭合（" + v.Red[0] + more + "） ⇒ 判红（漂移）"
		return v
	}
	v.OK = true
	v.Verdict = fmt.Sprintf("生成件族 == 导出件族（索引 1 + 页 %d · 未闭 %d 条 · Σ页条目 %d · 面 %d · 索引 %d 字节）",
		len(f.Pages), f.Open, f.PageSum, len(f.FaceCount), f.Bytes)
	return v
}

// gglPresence —— 面集差异的措辞（在 / 不在）。
func gglPresence(ok bool) string {
	if ok {
		return "在"
	}
	return "不在"
}

// gglRunExportOut —— 现跑唯一拆件写者 `<仓根>/bin/zerg gap export --out <目录> --yes`（D2 档：`--yes` 才真写）。
func gglRunExportOut(repoRoot, dir string) (string, map[string]string) {
	bin := gglExportBin(repoRoot)
	full := bin + " gap export --out " + dir + " --yes"
	ctx, cancel := context.WithTimeout(context.Background(), gglExportTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "gap", "export", "--out", dir, "--yes")
	cmd.Dir = repoRoot
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "timeout", map[string]string{"cmd": full, "msg": fmt.Sprintf("超时 %d 秒", int(gglExportTimeout.Seconds()))}
		}
		var ee *exec.Error
		if errors.As(err, &ee) && errors.Is(ee.Err, exec.ErrNotFound) {
			return "no_bin", map[string]string{"cmd": bin, "msg": ee.Err.Error()}
		}
		var xe *exec.ExitError
		if errors.As(err, &xe) {
			if err := ctx.Err(); err != nil {
				return "timeout", map[string]string{"cmd": full, "msg": fmt.Sprintf("超时 %d 秒", int(gglExportTimeout.Seconds()))}
			}
			se := strings.TrimSpace(errb.String())
			if len(se) > 400 {
				se = se[len(se)-400:]
			}
			return "rc", map[string]string{"cmd": full, "rc": strconv.Itoa(xe.ExitCode()), "stderr": se}
		}
		return "no_bin", map[string]string{"cmd": bin, "msg": err.Error()}
	}
	return "ok", map[string]string{"cmd": full}
}

// gglWriteAtomic —— 固定后缀 tmp + fsync + 原子 rename；写后读回逐字节比。
//
// ★ 本版**无调用点**（照实）：A 案（`GAP-20261017-16`）起 `--emit` 的写面**委托唯一拆件写者**
// `bin/zerg gap export --out <目录> --yes`（先页件、后索引件）—— 本件不再自己写单件直出，也不再
// 自己落索引件。原语保留：将来若本件真接管写面，原子写 + 写后读回是既有口径（✗ 别改成裸 WriteFile）。
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
			if fam, ok := gglPickFamily(docs, true); ok {
				ver = fam.Ver
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
	liveBytes := len([]byte(out))
	q := gglQidProbe(a.source)

	verdict := ""
	rc := 0
	drift := false
	fam := gglReadFamily(docs, ver, date8, tgt)
	var fv gglFamilyVerdict
	if mode == "emit" {
		// 写面 = **唯一拆件写者**（`gap export --out <目录> --yes`）；本件不再自己把单件直出写进索引位
		// （旧口径那样写会毁掉拆件族：索引被 26KB 单件覆盖、页件成孤儿、门63 当场红）。
		dir := gglPyJoin(docs, gglLedgerDir, ver)
		if st2, eread2 := gglRunExportOut(repoRoot, dir); st2 != "ok" {
			read := []gglKV{}
			for _, k := range []string{"cmd", "msg", "stderr", "rc"} {
				if v, ok := eread2[k]; ok {
					read = append(read, gglKV{k, v})
				}
			}
			return gglBail(stdout, a.jsonOut, mode, docs, ver, exportBin,
				fmt.Sprintf("拆件写者跑不动（%s）⇒ 一个字节都没落（不许当落盘成功）", st2), read)
		}
		fam = gglReadFamily(docs, ver, date8, tgt)
		fv = gglJudgeFamily(fam, date8, out)
		verdict = "落盘（`gap export --out` 重生成完成 · " + fv.Verdict + "）"
		if !fv.OK {
			rc = 1
		}
	} else {
		fv = gglJudgeFamily(fam, date8, out)
		verdict = fv.Verdict
		if !fv.OK {
			drift = true
			rc = 1
		}
	}
	famBytes, famPages, famOpen, famSum, famFaces := 0, 0, 0, 0, 0
	if fam != nil {
		famBytes, famPages, famOpen = fam.Bytes, len(fam.Pages), fam.Open
		famSum, famFaces = fam.PageSum, len(fam.FaceCount)
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
				{"read", gglJObj([]gglKV{
					{"target", gglJStr(tgt)},
					{"index_bytes", gglJInt(famBytes)},
					{"export_bytes", gglJInt(liveBytes)},
				})},
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
		for _, r := range fv.Red {
			problems = append(problems, gglJObj([]gglKV{{"rule", gglJStr("R1")}, {"what", gglJStr(r)}, {"read", gglJObj(nil)}}))
		}
		if q.problem != "" {
			problems = append(problems, gglJObj([]gglKV{{"rule", gglJStr("R2")}, {"what", gglJStr(q.problem)}, {"read", gglJObj(nil)}}))
		}
		var warns []string
		for _, n := range fv.Notes {
			warns = append(warns, gglJStr(n))
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
				{"bytes", gglJInt(famBytes)},
				{"export_bytes", gglJInt(liveBytes)},
				{"family", gglJObj([]gglKV{
					{"index_files", gglJInt(1)},
					{"pages", gglJInt(famPages)},
					{"open", gglJInt(famOpen)},
					{"page_entries", gglJInt(famSum)},
					{"faces", gglJInt(famFaces)},
				})},
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
			{"warnings", gglJArr(warns)},
			{"truncated", gglJBool(false)},
		})
		echo(obj)
		return rc
	}
	echo("── 结转账生成件（" + gglVersion + " · 设计稿 v1.3 §0 第 ③ 序 · A 案：族口径）──")
	echo("  导出器 : " + exportBin + " gap export（**现跑**）")
	echo("  文档仓 : " + docs)
	echo(fmt.Sprintf("  落点   : %s（索引 %d 字节 · 直出 %d 字节 · 版本线 %s）", rel, famBytes, liveBytes, ver))
	sub := "只判/只打 · 不改任何件"
	if mode == "emit" {
		sub = "写盘（走唯一拆件写者 `gap export --out`）"
	}
	echo(fmt.Sprintf("  模式   : %s（%s）", mode, sub))
	mark := "✓"
	if rc != 0 {
		mark = "✗"
	}
	echo(mark + " " + verdict)
	for _, r := range fv.Red {
		echo("  ✗ " + r)
	}
	for _, n := range fv.Notes {
		echo("  ⚠ " + n)
	}
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
