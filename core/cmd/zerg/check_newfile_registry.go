// check_newfile_registry.go —— 门76 判据件 `scripts/gates/check-newfile-registry.py`（729 行）的 **Go 真搬**。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2 新口径（外部树不再出局 ⇒ 改走**显式入参旋钮**）下的一枚：门76 = 新件登记六面总闸。
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**。
//
// 选它 / 不选它的理由：
//
//	✓ 选：只读六张登记面 + 现读盘上件集（**读**登记面合规 · 禁的是**改**）· 输出**定序**
//	  （所有集合处一律 `sorted`）· 退码 `0/1/2` 三档齐 · 729 行（行数适中）· **无人正在改**。
//	✓ 外部树（仓外台账 tsv / 仓外冻结基线）按新口径走**显式入参旋钮**（`--inventory` / `--baseline`，
//	  名照 py 同名旗标），缺省 = py 默认推导逐字同构（`--inventory` 缺省 = 同级 `Zerg-内部文档/项目文档/*/清单-脚本现状-*.tsv`
//	  取 sorted 末件；该串**不含 `..` 段** ⇒ `filepath.Join` 与 `os.path.join` 逐字同，**不做串拼**）。
//
// ★★ 唯一「不逐字可搬」的面（照实写在这里，**不粉饰**）：
//
//	① **面② 的「现读载入判据件」面 = Python `importlib` 动态载入任意 `.py` 并调用其 `scan_hits(root)`**
//	   —— Go **无等价能力**（Go 不能在进程内执行任意 Python 模块）。本件的处置：**以 `python3` 驱动
//	   子进程逐字复刻** py 的 `load_dry_module()` + `collect_paths()` + `dry_registry()` 三步
//	   （驱动脚本内嵌于本件 `gnrDryDriverPy`），**✗ 不另抄一份门⑩ 的扫描正则**（单一真源仍是那个 `.py`）。
//	   代价：本件**保留一枚解释器依赖**（仅面② 一处）；`python3` 缺席 / 驱动不可判 ⇒ 面② 按
//	   **判不了（fail-closed rc=2）** 走，✗ 不静默绿。**这一面交回父代理入账**（见件尾「交回」）。
//	② **py `--dump-baseline` 生成口自身是真口子（`GAP-20261001-21`）**：它把**当前树**的现读件集写成
//	   冻结基线 ⇒ 下一趟分母恒 `0` ⇒ 只能得 `rc=2`。**本件逐字保留该行为**（✗ 不自行修），交回入账。
//
// ★ 标准声明行（两条 · 逐条照实）
//
//	(a) **默认根定位差异**：py `REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))`
//	    ⇒ 默认根 = **脚本所在目录的仓**（与 cwd 无关）；Go 入口**无 `__file__` 语境** ⇒ 本件默认从
//	    **当前工作目录**上溯 ≤8 层找同时含 `scripts/gates/precommit-gates.sh` 与 `scripts` 的目录，
//	    找不到回落 cwd。落在**同一真仓、cwd=仓根**时两者读数逐字相同（已对拍 甲 格）；但「脚本位置 ≠ cwd」
//	    时**路径推导面不同** ⇒ 这一面**不属逐字对齐承诺**，对拍一律显式传 `--repo`。
//	(b) **自指检查（四形态逐条判）**：
//	    ① 本件落在门76 **自己**的扫描面内？——**否**。本件的集合定义 = `scan_ledger_face(root)`，它只遍历
//	       `<root>/scripts/**`；本件落 `core/cmd/zerg/check_newfile_registry.go` ⇒ **不在**该面内
//	       ⇒ 不进 ①②③⑤ 任何一格（已对拍：真仓 甲 格分母 3 件，与 py 逐字同）。
//	    ② 本件正文含**真引号的路径原串**会不会把它自己扫红？——本件**扫描面只判「件名在不在表里」**，
//	       不读任何被扫件正文 ⇒ **无**此患（零「写进被扫面的标记字面」）。
//	    ③ 本件是否落在**他门**的扫描面内？——**是（门⑩）**：`scripts/gates/check-dryrun-semantics.py` 的
//	       `SCAN_DIRS` 含 `core/`、`SCAN_EXTS` 含 `*.go`、模式 `dr[y]... run`（此处**故意拆写**）。
//	       **处理**：本件全文**零处**出现该许可模式（判据件的件名/登记表名在本件里**一律不落字面** ——
//	       面② 的判据件名由**步骤表现读**得出）⇒ 两态读数逐字相同（落仓前后 py 侧 `check-dryrun-semantics`
//	       的 `现跑件` 计数同值，见回执）。
//	    ④ 本件是否落在 `check-hardcoded-private-paths`（`SCAN_DIRS` 含 `core/`）面内？——**否**：本件零绝对
//	       私有路径字面（家目录走 `os.UserHomeDir()`，✗ 不写 `/Users/...`）。
//	    ⇒ 综合：自指 = **否**（唯一相邻面 = 门⑩，已按 ③ 处置并留证）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助 / usage 正文**（`-h/--help` · 未知旗标 · 缺值 · 位置参数）：本件落 rc=0 / 2 与 py 同，
//	   但**正文自定**（不逐字仿 argparse 的 usage 排版）。
//	② **默认根定位**：见上 (a)。
//	③ **非 UTF-8 输入的字面替换粒度**：py `errors="replace"` 与 Go `strings.ToValidUTF8` 对**连续非法字节**
//	   的 U+FFFD 分割粒度可能不同（真仓六张表皆合法 UTF-8 ⇒ 现读零影响）。
//	④ **面② 的判据件载入例外**：py 在 `spec.loader.exec_module` / `scan_hits(root)` 抛异常时 = **未捕异常
//	   ⇒ 回溯 + rc=1**；本件按新口径 **fail-closed rc=2**（正文亦不同）。
//	⑤ **面② 的 `__pycache__` 副产物**：py 走 `importlib` 会把字节码缓存写到判据件旁（真仓现读确见
//	   `scripts/gates/__pycache__`）；本件驱动**关掉字节码写入**（`sys.dont_write_bytecode`）⇒ 盘上副产物不同，
//	   读数面不受影响。
//	⑥ **`--self-test` 夹具的 `mkdtemp` 随机后缀**：py 自检把夹具落在 `<TMPDIR>/gnr-selftest-XXXXXXXX`
//	   并把**该路径印进正文**（`仓根：…`）⇒ 同一实现连跑两发亦不同。本件同样打印该路径 ⇒ 该格属**非确定面**，
//	   掩码该段后逐字节可比。
//	⑦ 本件**不做** py 的 `import glob as globmod` 那种 `**` 递归语义（`glob.glob` 缺省 `recursive=False`
//	   亦不递归）⇒ 两侧同为「单层 glob」，现读无差。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func checkNewfileRegistryGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 判到了（报告档；`--strict` 下 R1/R2/R3/R5 任一 > 0 ⇒ 1） / 2 不给结论
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见上「未覆盖面」）
//
// 交回父代理入账（✗ 本件不自行修）：
//
//	· `GAP-20261001-21`：`--dump-baseline` 生成口打**当前树** ⇒ 分母恒 0（逐字保留，见上 ★★②）。
//	· **面② 的解释器依赖**（见上 ★★①）：本件是 M2 已落件里**唯一**保留 `python3` 子进程的一枚。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// ── 常量（逐字照 py）───────────────────────────────────────────────────────
const (
	gnrSelfRel     = "scripts/gates/check-newfile-registry.py"
	gnrMarksRel    = "scripts/公开标记.tsv"
	gnrStepsRel    = "scripts/gates/precommit-gates.sh"
	gnrCovRel      = "scripts/gates/gate-coverage.config.json"
	gnrCovJudgeRel = "scripts/gates/check-gate-coverage.py"
	gnrPubRel      = "scripts/build/publish-public.sh"
	gnrMirrorRel   = "publish/mirror-public-lib.py"
	gnrLedgerSib   = "Zerg-内部文档"
	gnrStepMark    = "干跑"
	gnrAddStep     = "add_step"
	gnrJudgeRE     = `scripts/[A-Za-z0-9_/.-]+\.py`
	gnrExcludesRE  = `(?s)EXCLUDES=\((.*?)\n\)`
	gnrDropRE      = `(?s)DROP_EXACT\s*=\s*\{(.*?)\n\}`
)

var gnrLedgerGlob = filepath.Join("项目文档", "*", "清单-脚本现状-*.tsv")

// gnrScriptExcludes —— 逐字对齐 `script_inventory_test.go:28-32` 的 13 个排除面。
var gnrScriptExcludes = []string{
	"target", "node_modules", "dist", "bin", "vendor", "data", ".git", ".venv", "venv",
	".build", "zerg-wt", "_history", "__pycache__",
}

var (
	gnrJudgeRECompiled  = regexp.MustCompile(gnrJudgeRE)
	gnrExcludesCompiled = regexp.MustCompile(gnrExcludesRE)
	gnrDropCompiled     = regexp.MustCompile(gnrDropRE)
)

// gnrDryDriverPy —— py `load_dry_module()` + `collect_paths()` + `dry_registry()` 的**逐字复刻驱动**。
// argv：1=判据件绝对路径 · 2=仓根 · 3=步骤表里抽出的**仓根相对**判据件名（仅用于例外文案）。
// 输出：一行 JSON（**唯一** stdout 内容；模块自身的打印被临时导去 stderr）。
const gnrDryDriverPy = `import importlib.util, io, json, os, sys

sys.dont_write_bytecode = True
cand, root, relp_arg = sys.argv[1], sys.argv[2], sys.argv[3]
res = {"rel": None, "hits_ok": False, "hits": [], "reg_ok": False, "reg": [], "problem": None}
_real = sys.stdout
sys.stdout = sys.stderr


def _emit():
    sys.stdout = _real
    sys.stdout.write(json.dumps(res, ensure_ascii=False))
    sys.stdout.flush()
    raise SystemExit(0)


spec = None
try:
    spec = importlib.util.spec_from_file_location("_gnr_dry_judge", cand)
except Exception:
    spec = None
if spec is None or spec.loader is None:
    res["problem"] = "判据件载入不了：%s" % (relp_arg,)
    _emit()

mod = importlib.util.module_from_spec(spec)
try:
    spec.loader.exec_module(mod)
except Exception as exc:
    res["problem"] = "判据件载入不了：%s（%s）" % (relp_arg, exc)
    _emit()

try:
    raw = mod.scan_hits(root)
except Exception as exc:
    res["problem"] = "判据件扫描函数不可判：%s" % (exc,)
    _emit()

if raw is None or (isinstance(raw, tuple) and len(raw) > 0 and raw[0] is None):
    why = raw[1] if isinstance(raw, tuple) and len(raw) > 1 else raw
    res["problem"] = "扫描空转 / 不可判：%s" % (why,)
    _emit()

hits = set()


def _walk(x):
    if isinstance(x, str):
        hits.add(x)
    elif isinstance(x, (set, frozenset, list, tuple)):
        for i in x:
            _walk(i)
    elif isinstance(x, dict):
        for k in x:
            _walk(k)


_walk(raw)
res["hits_ok"] = True
res["hits"] = sorted(hits)

relp = getattr(mod, "REGISTRY_REL", None)
if isinstance(relp, str):
    res["rel"] = relp
if not relp:
    res["problem"] = "判据件未声明登记表路径（REGISTRY_REL）"
    _emit()

path = os.path.join(root, relp)
if not os.path.isfile(path):
    res["problem"] = "登记表不在盘：%s" % (relp,)
    _emit()
try:
    with open(path, encoding="utf-8", errors="replace") as fh:
        data = json.loads(fh.read())
except ValueError as exc:
    res["problem"] = "登记表解析失败：%s" % (exc,)
    _emit()
except OSError:
    res["problem"] = "登记表不在盘：%s" % (relp,)
    _emit()

reg = set()
for e in (data.get("件") or []):
    if isinstance(e, dict) and e.get("file") and isinstance(e["file"], str):
        reg.add(e["file"])
if reg:
    res["reg_ok"] = True
    res["reg"] = sorted(reg)
_emit()
`

// ── 基础工具（逐字对齐 py 语义）─────────────────────────────────────────────

func gnrIsDir(p string) bool { st, err := os.Stat(p); return err == nil && st.IsDir() }

func gnrIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// gnrReadText —— py `open(path, encoding="utf-8", errors="replace").read()`。
func gnrReadText(p string) (string, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return strings.ToValidUTF8(string(b), "\uFFFD"), true
}

// gnrSplitLines —— py `str.splitlines()` 的换行集（✗ 不是 `strings.Split(s, "\n")`）。
func gnrSplitLines(s string) []string {
	out := []string{}
	var cur strings.Builder
	for i := 0; i < len(s); {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if gnrIsLineBreak(r) {
			out = append(out, cur.String())
			cur.Reset()
			i += sz
			if r == '\r' && i < len(s) && s[i] == '\n' {
				i++
			}
			continue
		}
		cur.WriteRune(r)
		i += sz
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func gnrIsLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', 0x0b, 0x0c, 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// gnrRel —— py `os.path.relpath(p, root).replace(os.sep, "/")`。
func gnrRel(root, p string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(r)
}

// gnrTruthy —— py 真值语义（用于 json 取值）。
func gnrTruthy(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case float64:
		return x != 0
	case []interface{}:
		return len(x) > 0
	case map[string]interface{}:
		return len(x) > 0
	}
	return true
}

// gnrPyStr —— py `"%s" % v` 的形态（None ⇒ "None"）。
func gnrPyStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case []interface{}:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, gnrPyStr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return fmt.Sprintf("%v", v)
}

// gnrStripQ —— py `s.strip('"').strip("'")`。
func gnrStripQ(s string) string {
	s = strings.Trim(s, `"`)
	return strings.Trim(s, "'")
}

func gnrContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// ── 分母：两套现读口径 ─────────────────────────────────────────────────────
// ★ 遍历/排序点交代：py `os.walk` 里**明写** `dirnames[:] = sorted(...)` 与 `for name in sorted(filenames)`，
//
//	所以**遍历序**是「目录名 sorted 的 DFS × 文件名单层 sorted」；但本函数**返回集合**，而**输出行序**
//	的决定者是 `audit()` 里的 `sorted(ledger_face)` / `sorted(ledger_face - base)`（res["roster"] /
//	new_files / 对差 三处）⇒ **遍历序不决定任何一行输出**。本件按 Go 侧 `f.ReadDir(-1)`（**原序**）
//	收集进 map，序不进输出。✗ 不用 `os.ReadDir`（它会替我们排序、且与本函数无关）。
func gnrScanLedgerFace(root string) map[string]bool {
	base := filepath.Join(root, "scripts")
	if !gnrIsDir(base) {
		return nil
	}
	out := map[string]bool{}
	var walk func(dir string)
	walk = func(dir string) {
		fh, err := os.Open(dir)
		if err != nil {
			return
		}
		ents, err := fh.ReadDir(-1) // 原序：✗ 不用排序版
		fh.Close()
		if err != nil {
			return
		}
		for _, e := range ents {
			name := e.Name()
			p := filepath.Join(dir, name)
			lst, lerr := os.Lstat(p)
			if lerr != nil {
				continue
			}
			isLink := lst.Mode()&os.ModeSymlink != 0
			st := lst
			if isLink {
				if s2, e2 := os.Stat(p); e2 == nil {
					st = s2
				}
			}
			if st.IsDir() {
				// py `os.walk(followlinks=False)`：符号链接目录进 dirnames 但**不下降**、也不当文件处理。
				if isLink {
					continue
				}
				if gnrContains(gnrScriptExcludes, name) {
					continue
				}
				walk(p)
				continue
			}
			isScript := strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".py")
			if !isScript {
				if st.Mode().Perm()&0o111 == 0 { // py `os.access(p, os.X_OK)`
					continue
				}
				txt, ok := gnrReadText(p)
				if !ok {
					continue
				}
				if !strings.HasPrefix(txt, "#!") {
					continue
				}
			}
			out[gnrRel(root, p)] = true
		}
	}
	walk(base)
	return out
}

type gnrGate1Face struct {
	set     bool
	files   map[string]bool
	rawData map[string]interface{}
}

// gnrScanGate1Face —— 门① 候选面口径（`gate-coverage.config.json` 的 `scan.script_globs` + `nosuffix_dir`）。
func gnrScanGate1Face(root string) gnrGate1Face {
	cfg := filepath.Join(root, gnrCovRel)
	if !gnrIsFile(cfg) {
		return gnrGate1Face{}
	}
	txt, ok := gnrReadText(cfg)
	if !ok {
		return gnrGate1Face{}
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(txt), &data); err != nil {
		return gnrGate1Face{}
	}
	scan, _ := data["scan"].(map[string]interface{})
	if scan == nil {
		scan = map[string]interface{}{}
	}
	globs := []string{}
	if g, ok := scan["script_globs"].([]interface{}); ok {
		for _, v := range g {
			if s, ok := v.(string); ok {
				globs = append(globs, s)
			}
		}
	}
	if len(globs) == 0 {
		return gnrGate1Face{rawData: data}
	}
	comps := map[string]bool{}
	if ex, ok := data["exclude"].(map[string]interface{}); ok {
		if pc, ok := ex["path_components"].([]interface{}); ok {
			for _, v := range pc {
				if s, ok := v.(string); ok {
					comps[s] = true
				}
			}
		}
	}
	out := map[string]bool{}
	for _, g := range globs {
		matches, err := filepath.Glob(filepath.Join(root, g))
		if err != nil {
			continue
		}
		for _, p := range matches {
			if !gnrIsFile(p) {
				continue
			}
			r := gnrRel(root, p)
			if gnrAnyComp(r, comps) {
				continue
			}
			out[r] = true
		}
	}
	if ns, ok := scan["nosuffix_dir"].(string); ok && ns != "" {
		d := filepath.Join(root, ns)
		if gnrIsDir(d) {
			if ents, err := os.ReadDir(d); err == nil {
				names := make([]string, 0, len(ents))
				for _, e := range ents {
					names = append(names, e.Name())
				}
				sort.Strings(names) // py `sorted(os.listdir(d))` 明写
				for _, name := range names {
					p := filepath.Join(d, name)
					if gnrIsFile(p) && !strings.Contains(name, ".") {
						out[gnrRel(root, p)] = true
					}
				}
			}
		}
	}
	return gnrGate1Face{set: true, files: out, rawData: data}
}

func gnrAnyComp(rel string, comps map[string]bool) bool {
	for _, seg := range strings.Split(rel, "/") {
		if comps[seg] {
			return true
		}
	}
	return false
}

// ── 六张表：逐张现读 ───────────────────────────────────────────────────────
// gnrLoadMarks —— ① `公开标记.tsv` ⇒ {rel: public}；读不到 / 0 行 ⇒ nil。
// ★ py dict 的**插入序**本件用 `ginrMarkKeys` 切片保序；本件对 marks **只做按键查值**（✗ 零 map 迭代）。
func gnrLoadMarks(root string) (map[string]string, []string) {
	p := filepath.Join(root, gnrMarksRel)
	if !gnrIsFile(p) {
		return nil, nil
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil, nil
	}
	rows := map[string]string{}
	keys := []string{}
	for i, line := range gnrSplitLines(txt) {
		if strings.TrimSpace(line) == "" || i == 0 {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 2 {
			continue
		}
		k := strings.TrimSpace(cols[0])
		if _, seen := rows[k]; !seen {
			keys = append(keys, k)
		}
		rows[k] = strings.TrimSpace(cols[1])
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows, keys
}

// gnrLoadSteps —— ③a 步骤表：(add_step 行列表, 全文, 是否拿到)。
func gnrLoadSteps(root string) ([]string, string, bool) {
	p := filepath.Join(root, gnrStepsRel)
	if !gnrIsFile(p) {
		return nil, "", false
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil, "", false
	}
	lines := []string{}
	for _, ln := range gnrSplitLines(txt) {
		if strings.Contains(ln, gnrAddStep) {
			lines = append(lines, ln)
		}
	}
	return lines, txt, true
}

// gnrWlItem —— 门① 白名单条目（只取三格 · 其余不关心）。
type gnrWlItem struct {
	path   interface{}
	reason interface{}
	date   interface{}
}

// gnrLoadWhitelistScripts —— ③b：返回 (条目列表, 问题列表)；缺 ⇒ (nil, [问题])。
func gnrLoadWhitelistScripts(root string) ([]gnrWlItem, []string) {
	p := filepath.Join(root, gnrCovRel)
	if !gnrIsFile(p) {
		return nil, []string{"面③ 判不了：" + gnrCovRel + " 不在盘"}
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil, []string{"面③ 判不了：" + gnrCovRel + " 不在盘"}
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(txt), &data); err != nil {
		return nil, []string{"面③ 判不了：" + gnrCovRel + " 解析失败（" + err.Error() + "）"}
	}
	items := []gnrWlItem{}
	if wl, ok := data["whitelist"].(map[string]interface{}); ok {
		if arr, ok := wl["scripts"].([]interface{}); ok {
			for _, v := range arr {
				m, ok := v.(map[string]interface{})
				if !ok {
					items = append(items, gnrWlItem{})
					continue
				}
				items = append(items, gnrWlItem{path: m["path"], reason: m["reason"], date: m["date"]})
			}
		}
	}
	problems := []string{}
	for _, it := range items {
		if !gnrTruthy(it.reason) || !gnrTruthy(it.date) {
			problems = append(problems, "面③ 白名单条目缺 reason/date："+gnrPyStr(it.path))
		}
	}
	return items, problems
}

// gnrLoadExcludes —— ⑤a `EXCLUDES=( … )`（活跃条目）。
func gnrLoadExcludes(root string) map[string]bool {
	p := filepath.Join(root, gnrPubRel)
	if !gnrIsFile(p) {
		return nil
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil
	}
	m := gnrExcludesCompiled.FindStringSubmatch(txt)
	if m == nil {
		return nil
	}
	out := map[string]bool{}
	for _, ln := range gnrSplitLines(m[1]) {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		s = gnrStripQ(s)
		if s != "" {
			out[s] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// gnrLoadDropExact —— ⑤b `DROP_EXACT = { … }`。
func gnrLoadDropExact(root string) map[string]bool {
	p := filepath.Join(root, gnrMirrorRel)
	if !gnrIsFile(p) {
		return nil
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil
	}
	m := gnrDropCompiled.FindStringSubmatch(txt)
	if m == nil {
		return nil
	}
	out := map[string]bool{}
	for _, ln := range gnrSplitLines(m[1]) {
		s := strings.TrimSpace(strings.SplitN(ln, "#", 2)[0])
		s = strings.TrimRight(s, ",")
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out[gnrStripQ(s)] = true
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// gnrLoadLedgerRows —— ④ 仓外台账 tsv 行集（**显式入参旋钮 `--inventory`**）。
func gnrLoadLedgerRows(root, inv string, invSet bool) (map[string]bool, string, bool) {
	path := inv
	if invSet && inv != "" && !filepath.IsAbs(inv) {
		path = filepath.Join(root, inv)
	}
	if !invSet {
		sib := filepath.Join(filepath.Dir(gnrAbs(root)), gnrLedgerSib)
		pat := filepath.Join(sib, gnrLedgerGlob)
		cand, _ := filepath.Glob(pat)
		sort.Strings(cand) // py `sorted(globmod.glob(...))`
		if len(cand) > 0 {
			path = cand[len(cand)-1]
		} else {
			path = ""
		}
	}
	if path == "" || !gnrIsFile(path) {
		return nil, path, false
	}
	txt, ok := gnrReadText(path)
	if !ok {
		return nil, path, false
	}
	rows := map[string]bool{}
	for i, line := range gnrSplitLines(txt) {
		if strings.TrimSpace(line) == "" || i == 0 {
			continue
		}
		rows[strings.TrimSpace(strings.SplitN(line, "\t", 2)[0])] = true
	}
	if len(rows) == 0 {
		return nil, path, true
	}
	return rows, path, true
}

func gnrAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

// gnrLoadWording —— ⑥ 门① 判据件的「处置」文案是否已指到白名单档。
func gnrLoadWording(root string) (*bool, []string, bool) {
	p := filepath.Join(root, gnrCovJudgeRel)
	if !gnrIsFile(p) {
		return nil, nil, false
	}
	txt, ok := gnrReadText(p)
	if !ok {
		return nil, nil, false
	}
	lines := gnrSplitLines(txt)
	type blk struct {
		b []string
		s string
	}
	blocks := []blk{}
	idx := []int{}
	for j, ln := range lines {
		if strings.Contains(ln, "处置") {
			idx = append(idx, j)
		}
	}
	for _, i := range idx {
		lo := i - 1
		if lo < 0 {
			lo = 0
		}
		hi := i + 4
		if hi > len(lines) {
			hi = len(lines)
		}
		seg := make([]string, 0, hi-lo)
		for _, ln := range lines[lo:hi] {
			seg = append(seg, strings.TrimSpace(ln))
		}
		blob := strings.Join(seg, " ")
		if strings.Contains(blob, "处置") {
			blocks = append(blocks, blk{seg, blob})
		}
	}
	rel := []blk{}
	for _, b := range blocks {
		if strings.Contains(b.s, gnrAddStep) || strings.Contains(b.s, "挂门") {
			rel = append(rel, b)
		}
	}
	if len(rel) == 0 {
		return nil, nil, false
	}
	fixed := true
	for _, b := range rel {
		if !(strings.Contains(b.s, "白名单") && strings.Contains(b.s, "reason")) {
			fixed = false
		}
	}
	out := []string{}
	for _, b := range rel {
		pick := ""
		for _, x := range b.b {
			if strings.Contains(x, "处置") {
				pick = x
				break
			}
		}
		if pick == "" && len(b.b) > 0 {
			pick = b.b[0]
		}
		out = append(out, pick)
	}
	if len(out) > 4 {
		out = out[:4]
	}
	return &fixed, out, true
}

// ── 面②：现读载入判据件（★ 唯一解释器依赖面 · 见件头 ★★①）────────────────────
type gnrDry struct {
	hitsOK  bool
	hits    map[string]bool
	regOK   bool
	reg     map[string]bool
	rel     string
	relSet  bool
	problem string
}

type gnrDryOut struct {
	Rel     *string  `json:"rel"`
	HitsOK  bool     `json:"hits_ok"`
	Hits    []string `json:"hits"`
	RegOK   bool     `json:"reg_ok"`
	Reg     []string `json:"reg"`
	Problem *string  `json:"problem"`
}

// gnrLoadDryJudge —— py `load_dry_module()` + `collect_paths()` + `dry_registry()` 的合体。
func gnrLoadDryJudge(root string) gnrDry {
	_, text, ok := gnrLoadSteps(root)
	if !ok {
		return gnrDry{problem: "步骤表读不到"}
	}
	for _, ln := range gnrSplitLines(text) {
		if !strings.Contains(ln, gnrAddStep) || !strings.Contains(ln, gnrStepMark) {
			continue
		}
		m := gnrJudgeRECompiled.FindString(ln)
		if m == "" {
			continue
		}
		cand := filepath.Join(root, m)
		if !gnrIsFile(cand) {
			continue
		}
		out, errText := gnrRunDryDriver(cand, root, m)
		if out == nil {
			msg := "判据件载入不了：" + m
			if errText != "" {
				msg = msg + "（" + errText + "）"
			}
			return gnrDry{problem: msg}
		}
		d := gnrDry{}
		if out.Rel != nil {
			d.rel, d.relSet = *out.Rel, true
		}
		if out.HitsOK {
			d.hitsOK = true
			d.hits = map[string]bool{}
			for _, h := range out.Hits {
				d.hits[h] = true
			}
		}
		if out.RegOK {
			d.regOK = true
			d.reg = map[string]bool{}
			for _, r := range out.Reg {
				d.reg[r] = true
			}
		}
		if out.Problem != nil {
			d.problem = *out.Problem
		}
		return d
	}
	return gnrDry{problem: "步骤表里找不到「" + gnrStepMark + "」那条步的判据件（.py）"}
}

func gnrRunDryDriver(cand, root, relArg string) (*gnrDryOut, string) {
	cmd := exec.Command("python3", "-c", gnrDryDriverPy, cand, root, relArg)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, strings.TrimSpace(errb.String())
	}
	var o gnrDryOut
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &o); err != nil {
		return nil, strings.TrimSpace(errb.String())
	}
	return &o, ""
}

// ── 主判据 ────────────────────────────────────────────────────────────────
type gnrNJ struct{ face, item, why string }

type gnrCounts struct {
	R1, R2, R3, R5 int
	R4             *int
	R6             *int
}

type gnrRes struct {
	counts    *gnrCounts
	ledgerCnt *int
	gate1Cnt  *int
	diff      []string
	diffSet   bool
	problems  []string
	notJudged []gnrNJ
	roster    []string
	newFiles  []string
	items     map[string][]string
	weak      []string
	fourth    []string
	dryNA     []string
	dryHits   map[string]bool
	dryHitsOK bool
	ledPath   string
	ledRows   *int
	wording   *bool
	wLines    []string
	marks     map[string]string
}

func gnrAudit(root string, newFiles []string, newSet bool, inv string, invSet bool, baseline string) *gnrRes {
	res := &gnrRes{items: map[string][]string{}, problems: []string{}, notJudged: []gnrNJ{}}
	ledgerFace := gnrScanLedgerFace(root)
	if ledgerFace == nil {
		res.problems = append(res.problems, "扫面 0 件：`scripts` 不在盘 ⇒ 空转 = 假绿")
		return res
	}
	roster := []string{}
	for k := range ledgerFace {
		roster = append(roster, k)
	}
	sort.Strings(roster)
	res.roster = roster

	g1 := gnrScanGate1Face(root)
	n := len(ledgerFace)
	res.ledgerCnt = &n
	if g1.set {
		m := len(g1.files)
		res.gate1Cnt = &m
	} else if g1.rawData == nil {
		res.gate1Cnt = nil
	} else {
		res.gate1Cnt = nil
	}
	if g1.set {
		diff := []string{}
		for k := range ledgerFace {
			if !g1.files[k] {
				diff = append(diff, k)
			}
		}
		for k := range g1.files {
			if !ledgerFace[k] {
				diff = append(diff, k)
			}
		}
		sort.Strings(diff)
		res.diff = diff
		res.diffSet = true
	}

	// 分母
	if !newSet {
		if baseline == "" || !gnrIsFile(baseline) {
			shown := baseline
			if shown == "" {
				shown = gnrDefaultBaseline()
			}
			res.problems = append(res.problems, "分母判不了：冻结基线不在盘（"+shown+"）⇒ 不给结论；先跑 `--dump-baseline <仓外路径>`")
			return res
		}
		base := map[string]bool{}
		txt, _ := gnrReadText(baseline)
		for _, ln := range gnrSplitLines(txt) {
			s := strings.TrimSpace(ln)
			if s != "" && !strings.HasPrefix(s, "#") {
				base[s] = true
			}
		}
		nf := []string{}
		for k := range ledgerFace {
			if !base[k] {
				nf = append(nf, k)
			}
		}
		sort.Strings(nf)
		res.newFiles = nf
	} else {
		seen := map[string]bool{}
		nf := []string{}
		for _, f := range newFiles {
			if !seen[f] {
				seen[f] = true
				nf = append(nf, f)
			}
		}
		sort.Strings(nf)
		res.newFiles = nf
	}

	marks, _ := gnrLoadMarks(root)
	dry := gnrLoadDryJudge(root)
	dryErr := dry.problem
	var dryHits, dryReg map[string]bool
	dryRel := ""
	dryRelSet := false
	dryProblem := dryErr
	if dry.hitsOK && dry.regOK {
		dryHits, dryReg = dry.hits, dry.reg
		dryRel, dryRelSet = dry.rel, dry.relSet
		dryProblem = ""
	} else if dry.hitsOK {
		dryHits = dry.hits
		dryRel, dryRelSet = dry.rel, dry.relSet
		if dry.problem == "" {
			dryProblem = ""
		} else {
			dryProblem = dry.problem
		}
	}
	stepLines, stepText, stepOK := gnrLoadSteps(root)
	wlItems, wlProblems := gnrLoadWhitelistScripts(root)
	excludes := gnrLoadExcludes(root)
	drops := gnrLoadDropExact(root)
	ledRows, ledPath, _ := gnrLoadLedgerRows(root, inv, invSet)
	fixed, wLines, wordingOK := gnrLoadWording(root)

	type faceCheck struct {
		name string
		ok   bool
		why  string
	}
	why2 := dryProblem
	if dryRelSet && dryRel != "" {
		why2 = dryRel
	} else if dryProblem == "" {
		why2 = "门⑩ 登记表"
	}
	for _, fc := range []faceCheck{
		{"①公开标记表", marks != nil, gnrMarksRel},
		{"②干跑登记", dry.hitsOK && dry.regOK, why2},
		{"③门①两张真源", stepOK && wlItems != nil, gnrStepsRel},
		{"⑤两器排除表", excludes != nil && drops != nil, gnrPubRel},
	} {
		if !fc.ok {
			res.problems = append(res.problems, "面"+fc.name+" 判不了："+fc.why+" 读不到 / 解析出 0 条（真源读不全 ⇒ 绿就是假绿）")
		}
	}
	res.problems = append(res.problems, wlProblems...)
	if ledRows == nil {
		item := ledPath
		if item == "" {
			item = "（同级 " + gnrLedgerSib + " 下无 清单-脚本现状-*.tsv）"
		}
		res.notJudged = append(res.notJudged, gnrNJ{"④台账面", item,
			"仓外真源 · 本门**不写**（唯一写入口 `zerg script inventory sync` = 危险档 D2 写面 · 需人签）"})
	}
	if !wordingOK {
		res.notJudged = append(res.notJudged, gnrNJ{"⑥文案面", gnrCovJudgeRel, "判据件不在盘 / 找不着处置文案"})
	}

	r1, r2, r3, r5 := []string{}, []string{}, []string{}, []string{}
	weak, fourth, dryNA := []string{}, []string{}, []string{}
	wlPaths := map[string]bool{}
	for _, it := range wlItems {
		if s, ok := it.path.(string); ok {
			wlPaths[s] = true
		}
	}
	for _, f := range res.newFiles {
		// ①
		if marks != nil {
			if _, ok := marks[f]; !ok {
				r1 = append(r1, f)
			}
		}
		// ②
		if dryHits != nil && dryReg != nil {
			if dryHits[f] && !dryReg[f] {
				r2 = append(r2, f)
			} else if !dryHits[f] {
				dryNA = append(dryNA, f)
			}
		}
		// ③
		if stepLines != nil && wlItems != nil {
			strong, weakm := false, false
			for _, ln := range stepLines {
				if strings.Contains(ln, f) {
					strong = true
				}
				if strings.Contains(ln, filepath.Base(f)) {
					weakm = true
				}
			}
			if wlPaths[f] {
				// pass
			} else if strong {
				// pass
			} else if weakm {
				weak = append(weak, f)
			} else if stepText != "" && strings.Contains(stepText, filepath.Base(f)) {
				fourth = append(fourth, f)
			} else {
				r3 = append(r3, f)
			}
		}
		// ⑤
		if marks != nil && excludes != nil && drops != nil {
			if marks[f] == "no" && !(excludes[f] && drops[f]) {
				r5 = append(r5, f)
			}
		}
	}
	var r4 []string
	var r4n *int
	if ledRows != nil {
		r4 = []string{}
		for _, f := range res.newFiles {
			if !ledRows[f] {
				r4 = append(r4, f)
			}
		}
		m := len(r4)
		r4n = &m
	}
	var r6n *int
	if wordingOK {
		v := 0
		if !*fixed {
			v = 1
		}
		r6n = &v
	}
	res.counts = &gnrCounts{R1: len(r1), R2: len(r2), R3: len(r3), R4: r4n, R5: len(r5), R6: r6n}
	res.items["R1"], res.items["R2"], res.items["R3"] = r1, r2, r3
	if r4 == nil {
		r4 = []string{}
	}
	res.items["R4"], res.items["R5"] = r4, r5
	res.weak, res.fourth, res.dryNA = weak, fourth, dryNA
	res.dryHits, res.dryHitsOK = dryHits, dryHits != nil
	if ledRows != nil {
		m := len(ledRows)
		res.ledRows = &m
	}
	res.ledPath = ledPath
	res.wording, res.wLines = fixed, wLines
	res.marks = marks
	return res
}

func gnrDefaultBaseline() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "~"
	}
	return filepath.Join(home, ".zerg", "newfile-registry-baseline.tsv")
}

// gnrRun —— py `run()`：输出 + 退码。strict=阻断档。
func gnrRun(stdout io.Writer, root string, newFiles []string, newSet bool, inv string, invSet bool, strict bool, baseline string, baselineSet bool) int {
	res := gnrAudit(root, newFiles, newSet, inv, invSet, baseline)
	fmt.Fprintln(stdout, "门76 新件登记完整性（件级 × 六张登记表 · **只报告**）")
	fmt.Fprintf(stdout, "  仓根：%s\n", root)
	if res.ledgerCnt != nil {
		fmt.Fprintf(stdout, "  口径差栏：台账口径 %s 件 · 门①候选面 %s 件 · 对差 %s 件（参照读数 · 不并入判红）\n",
			gnrPyStr(float64(*res.ledgerCnt)), gnrCountStr(res.gate1Cnt), gnrDiffStr(res))
	} else {
		fmt.Fprintf(stdout, "  口径差栏：台账口径 %s 件 · 门①候选面 %s 件 · 对差 %s 件（参照读数 · 不并入判红）\n",
			"None", "None", "判不了")
	}
	for i, f := range res.diff {
		if i >= 40 {
			break
		}
		fmt.Fprintf(stdout, "     · 口径差 %s\n", f)
	}
	if res.counts == nil {
		for _, p := range res.problems {
			fmt.Fprintf(stdout, "  ✗ %s\n", p)
		}
		fmt.Fprintln(stdout, "  判据结论：**判不了 ⇒ 不给结论（rc=2）**")
		return 2
	}
	c := res.counts
	fmt.Fprintf(stdout, "  分母（现读新件 ∖ 冻结基线）：%d 件\n", len(res.newFiles))
	for i, f := range res.newFiles {
		if i >= 40 {
			break
		}
		fmt.Fprintf(stdout, "     - %s\n", f)
	}
	fmt.Fprintf(stdout, "  面① 公开标记表缺登记：%d\n", c.R1)
	for _, f := range res.items["R1"] {
		fmt.Fprintf(stdout, "     ✗ %s  ← 表里无此行（补 `scripts/公开标记.tsv` 一件一行 · public=no 也要写理由）\n", f)
	}
	fmt.Fprintf(stdout, "  面② 干跑登记缺件：%d（口径 = 门⑩ 判据件自己的扫描函数（现读载入）· 不适用 %d 件）\n", c.R2, len(res.dryNA))
	for _, f := range res.items["R2"] {
		fmt.Fprintf(stdout, "     ✗ %s  ← 正文命中干跑口径却未登记（补该登记表的 件[].file）\n", f)
	}
	fmt.Fprintf(stdout, "  面③ 门① 两张真源缺件：%d（弱挂载 %d 件 · 通用语法步兜住 %d 件 —— 均不判红）\n", c.R3, len(res.weak), len(res.fourth))
	for _, f := range res.items["R3"] {
		fmt.Fprintf(stdout, "     ✗ %s  ← 既不在 add_step 点名、也不在 whitelist.scripts（补 reason+date 白名单档）\n", f)
	}
	for _, f := range res.weak {
		fmt.Fprintf(stdout, "     · 弱 %s  ← 仅 basename 出现在 add_step 行（疑点 · 不判红）\n", f)
	}
	if c.R4 == nil {
		fmt.Fprintf(stdout, "  面④ 脚本台账：**判不了**（真源 %s）\n", gnrPathOr(res.ledPath))
	} else {
		fmt.Fprintf(stdout, "  面④ 脚本台账缺登记：%d（台账 %s 行 · **单列旁栏 · 不进退码** · 写入口 = 仓外 `zerg script inventory sync` D2 需人签）\n",
			*c.R4, gnrPyStr(float64(*res.ledRows)))
		for _, f := range res.items["R4"] {
			fmt.Fprintf(stdout, "     ✗ %s  ← 台账 tsv 无此行（治法 = 人签跑 sync · 本门只读不写）\n", f)
		}
	}
	for _, nj := range res.notJudged {
		fmt.Fprintf(stdout, "     ✗判不了 %s · %s —— %s\n", nj.face, nj.item, nj.why)
	}
	fmt.Fprintf(stdout, "  面⑤ 公开链两器排除表：%d（仅判 public=no 的件 · 须两器同批）\n", c.R5)
	for _, f := range res.items["R5"] {
		fmt.Fprintf(stdout, "     ✗ %s  ← 表判 no，两器排除表没收它（`EXCLUDES` 与 `DROP_EXACT` 两处都要）\n", f)
	}
	if c.R6 == nil {
		fmt.Fprintln(stdout, "  面⑥ 失败文案面：**判不了**（判据件不在盘）")
	} else {
		fmt.Fprintf(stdout, "  面⑥ 失败文案面（**单列旁栏 · 不进退码**）：%d\n", *c.R6)
		for _, ln := range res.wLines {
			mark := "✗"
			if res.wording != nil && *res.wording {
				mark = "✓"
			}
			fmt.Fprintf(stdout, "     %s %s\n", mark, ln)
		}
		if res.wording != nil && !*res.wording {
			fmt.Fprintln(stdout, "     ⇒ 文案仍只指 add_step；对「语料在仓外」与「写盘型」两类件是禁路 ⇒ 应改指白名单档（reason+date）")
		}
	}
	selfIn := gnrContains(res.newFiles, gnrSelfRel)
	fmt.Fprintf(stdout, "  ★ 自指：本门自身 %s —— 在分母内：%s · 面①：%s · 面②：%s · 面③：%s · 面④：见上「面④」那几行（台账 tsv 里暂无它 · 唯一写入口是仓外 D2 写面 · 需人签）· 面⑤：%s\n",
		gnrSelfRel, gnrBoolZh(selfIn, "是", "否（本趟不在新件集）"),
		gnrSelfMark(res.marks), gnrSelfDry(res.dryHits), gnrSelfInList(res.items["R3"]), gnrSelfInList(res.items["R5"]))
	for _, p := range res.problems {
		fmt.Fprintf(stdout, "  ✗ %s\n", p)
	}
	bad := c.R1 + c.R2 + c.R3 + c.R5
	unjudged := len(res.notJudged) + len(res.problems)
	if c.R4 == nil {
		unjudged++
	}
	if unjudged > 0 || len(res.newFiles) == 0 {
		fmt.Fprintln(stdout, "  判据结论：**判不了 / 分母 0 ⇒ 不给结论（rc=2）**（真的没有 ≠ 判不了）")
		return 2
	}
	if bad > 0 {
		rc := 0
		if strict {
			rc = 1
		}
		fmt.Fprintf(stdout, "  判据结论：**%d 条缺登记**（逐条点名如上）；报告档 rc=%d（只报告 · 不接闸）\n", bad, rc)
		return rc
	}
	fmt.Fprintln(stdout, "  判据结论：判到了 —— 五张登记表逐面 **0 条缺登记**（面④/⑥ 见上）")
	return 0
}

func gnrCountStr(p *int) string {
	if p == nil {
		return "None"
	}
	return gnrPyStr(float64(*p))
}

func gnrDiffStr(res *gnrRes) string {
	if !res.diffSet {
		return "判不了"
	}
	return gnrPyStr(float64(len(res.diff)))
}

func gnrPathOr(p string) string {
	if p == "" {
		return "不在盘"
	}
	return p
}

func gnrBoolZh(b bool, t, f string) string {
	if b {
		return t
	}
	return f
}

func gnrSelfMark(marks map[string]string) string {
	if marks != nil {
		if v, ok := marks[gnrSelfRel]; ok {
			return "已登记（public=" + v + "）"
		}
	}
	return "未登记"
}

func gnrSelfDry(hits map[string]bool) string {
	if hits != nil && hits[gnrSelfRel] {
		return "需登记"
	}
	return "不适用（正文不在该口径的扫描面内）"
}

func gnrSelfInList(list []string) string {
	if gnrContains(list, gnrSelfRel) {
		return "缺登记"
	}
	return "不在缺登记名单"
}

// ── 自检（合成夹具 · 成对 · 不扫真仓）──────────────────────────────────────
const (
	gnrStubRegRel   = "core/internal/contract/semantics-stub.json"
	gnrStubJudgeRel = "scripts/gates/check-semantics-stub.py"
	gnrProbeRel     = "scripts/sandbox/new-probe.sh"
)

var gnrFixtureStub = "REGISTRY_REL = \"" + gnrStubRegRel + "\"\n\n\ndef scan_hits(root):\n    return {\"" + gnrProbeRel + "\"}, None\n"

const gnrFixtureMarks = "文件\tpublic\t理由\nscripts/old.sh\tno\t夹具存量件\n"

const gnrFixtureCov = `{
  "scan": {"script_globs": ["scripts/*.sh", "scripts/*.py", "scripts/sandbox/*.sh"],
           "nosuffix_dir": "scripts"},
  "exclude": {"path_components": ["__pycache__", "_history"]},
  "whitelist": {"scripts": []}
}
`

const gnrFixtureDry = `{
  "_扫描命令": "grep -r -E \"（口径 = 判据件的扫描函数）\" --include=*.sh --include=*.py scripts core",
  "件": [{"file": "scripts/old.sh"}]
}
`

const gnrFixtureSteps = `#!/usr/bin/env bash
add_step "gates" tri-report "门76 新件登记完整性（只报告）" python3 scripts/gates/check-newfile-registry.py
add_step "gates" tri "门⑩ 干跑：一套语义（阻断）" scripts/gates/check-semantics-stub.py
add_step "gates" tri "public: 公开面标记表" python3 scripts/gates/check-public-marks.py
`

const gnrFixturePub = `#!/usr/bin/env bash
EXCLUDES=(
  "tools/ocr/venv"
)
`

const gnrFixtureMirror = `DROP_EXACT = {
    "core/internal/o1.json",
}
`

const gnrFixtureCovJudge = `#!/usr/bin/env python3
def report():
    print("        ⇒ 处置：**真修** —— 该件与本批代码**同批挂门**"
          "（` + "`scripts/gates/precommit-gates.sh`" + ` 的 ` + "`add_step`" + ` 按名调用它）；"
          "新件**不许**蹭基线棘轮。")
`

// gnrFailer —— 让夹具既能给 `*testing.T` 用、也能给自检（t=nil）用。
type gnrFailer interface{ Fatal(...interface{}) }

// gnrBuildFixture —— py `build_fixture()` 的逐字同构（六张表最小真形状 + 一件存量件 + 可选一件新件）。
func gnrBuildFixture(t gnrFailer, tmp, probe string, register map[string]bool) {
	mk := func(rel, body string) {
		p := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			if t != nil {
				t.Fatal(err)
			}
			return
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			if t != nil {
				t.Fatal(err)
			}
		}
	}
	for _, d := range []string{"scripts", "scripts/sandbox", "scripts/build", "scripts/gates", "publish",
		"core/internal/contract"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			if t != nil {
				t.Fatal(err)
			}
			return
		}
	}
	mk(filepath.Join("scripts", "old.sh"), "#!/usr/bin/env bash\necho old\n")
	marks, cov, dry, pub, mir := gnrFixtureMarks, gnrFixtureCov, gnrFixtureDry, gnrFixturePub, gnrFixtureMirror
	if probe != "" {
		mk(probe, "#!/usr/bin/env bash\necho probe\n")
		if register["①"] {
			marks += probe + "\tno\t夹具新件\n"
		}
		if register["②"] {
			dry = strings.Replace(dry, `{"file": "scripts/old.sh"}`,
				`{"file": "scripts/old.sh"}, {"file": "`+probe+`"}`, 1)
		}
		if register["③"] {
			cov = strings.Replace(cov, `"whitelist": {"scripts": []}`,
				`"whitelist": {"scripts": [{"path": "`+probe+`", "reason": "夹具", "date": "2026-09-28"}]}`, 1)
		}
		if register["⑤"] {
			pub = strings.Replace(pub, "\n)", "\n  \""+probe+"\"\n)", 1)
			mir = strings.Replace(mir, "\n}", "\n    \""+probe+"\",\n}", 1)
		}
	}
	mk(gnrMarksRel, marks)
	mk(gnrCovRel, cov)
	mk(gnrStubRegRel, dry)
	mk(gnrStubJudgeRel, gnrFixtureStub)
	mk(gnrStepsRel, gnrFixtureSteps)
	mk(gnrPubRel, pub)
	mk(gnrMirrorRel, mir)
	mk(gnrCovJudgeRel, gnrFixtureCovJudge)
}

type gnrTestCase struct {
	name    string
	want    int
	got     int
	ok      bool
	newF    []string
	newSet  bool
	inv     string
	invSet  bool
	strict  bool
	base    string
	baseSet bool
}

// gnrSelfTest —— py `run_self_test()` 的逐字同构（8 格 · 输出与 py 逐字对齐，仅夹具随机后缀不同）。
func gnrSelfTest(stdout io.Writer) int {
	checks := []gnrTestCase{}
	probe := gnrProbeRel
	run1 := func(name string, want int, newF []string, newSet bool, inv string, invSet bool, strict bool, base string, baseSet bool, setup func(tmp string)) {
		tmp, err := os.MkdirTemp("", "gnr-selftest-")
		if err != nil {
			return
		}
		defer os.RemoveAll(tmp)
		setup(tmp)
		rc := gnrRun(stdout, tmp, newF, newSet, inv, invSet, strict, base, baseSet)
		checks = append(checks, gnrTestCase{name: name, want: want, got: rc, ok: rc == want})
	}
	allFive := func(tmp string) {
		gnrBuildFixture(nil, tmp, probe, map[string]bool{"①": true, "②": true, "③": true, "⑤": true})
		os.WriteFile(filepath.Join(tmp, "inv.tsv"), []byte("件\n"+probe+"\n"), 0o644)
	}
	unregistered := func(tmp string) {
		gnrBuildFixture(nil, tmp, probe, nil)
		os.WriteFile(filepath.Join(tmp, "inv.tsv"), []byte("件\n"+probe+"\n"), 0o644)
	}
	noLedger := func(tmp string) {
		gnrBuildFixture(nil, tmp, probe, map[string]bool{"①": true, "②": true, "③": true, "⑤": true})
		os.WriteFile(filepath.Join(tmp, "inv-absent.tsv"), []byte("件\n"), 0o644)
	}
	noProbe := func(tmp string) {
		gnrBuildFixture(nil, tmp, "", nil)
		os.WriteFile(filepath.Join(tmp, "inv.tsv"), []byte("件\n"), 0o644)
	}
	weakOnly := func(tmp string) {
		gnrBuildFixture(nil, tmp, probe, map[string]bool{"①": true, "②": true, "⑤": true})
		f, _ := os.OpenFile(filepath.Join(tmp, gnrStepsRel), os.O_APPEND|os.O_WRONLY, 0o644)
		if f != nil {
			f.WriteString("add_step \"build\" tri \"构建\" bash scripts/build/new-probe.sh\n")
			f.Close()
		}
		os.WriteFile(filepath.Join(tmp, "inv.tsv"), []byte("件\n"+probe+"\n"), 0o644)
	}
	run1("正控① · 五表登记齐（报告档）⇒ 0", 0, []string{probe}, true, "inv.tsv", true, false, "", false, allFive)
	run1("正控② · 五表登记齐（阻断档）⇒ 0", 0, []string{probe}, true, "inv.tsv", true, true, "", false, allFive)
	run1("负控① · 新件**缺登记**（报告档不阻断）⇒ 0 但逐条点名", 0, []string{probe}, true, "inv.tsv", true, false, "", false, unregistered)
	run1("负控② · 新件缺登记（阻断档）⇒ 1 红", 1, []string{probe}, true, "inv.tsv", true, true, "", false, unregistered)
	run1("判不了① · **分母 0 件**（没有新件）⇒ 2 不许绿", 2, []string{}, true, "", false, false, "", false, noProbe)
	run1("判不了② · 面④ 仓外台账真源不在 ⇒ 2（含 ⑤ 已登记 ⇒ 不许报绿）", 2, []string{probe}, true, "路径不存在.tsv", true, false, "", false, noLedger)
	run1("边界① · 基线不在盘 ⇒ 2（不替基线猜分母）", 2, nil, false, "", false, false, "", false, noProbe)
	run1("边界② · 仅 basename 弱挂载 ⇒ 不判红（不计入六表缺登记）", 0, []string{probe}, true, "inv.tsv", true, true, "", false, weakOnly)

	fmt.Fprintln(stdout, "── 自检（合成夹具 · 成对 · 不扫真仓）──")
	bad := 0
	for _, c := range checks {
		mark := "✗"
		if c.ok {
			mark = "✓"
		}
		fmt.Fprintf(stdout, "  %s %s —— 要 %d · 得 %d\n", mark, c.name, c.want, c.got)
		if !c.ok {
			bad++
		}
	}
	concl := "全过"
	if bad != 0 {
		concl = "**不过**"
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d）\n", concl, len(checks), bad)
	if bad == 0 {
		return 0
	}
	return 1
}

// ── 默认根（见件头 (a)）────────────────────────────────────────────────────
func gnrDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	start, err := filepath.Abs(wd)
	if err != nil {
		return wd
	}
	d := start
	for i := 0; i < 8; i++ {
		if gnrIsDir(filepath.Join(d, "scripts")) &&
			gnrIsFile(filepath.Join(d, gnrStepsRel)) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return wd
}

const gnrUsage = "usage: check-newfile-registry.py [-h] [--repo REPO] [--baseline BASELINE]\n" +
	"                                 [--dump-baseline DUMP_BASELINE]\n" +
	"                                 [--inventory INVENTORY] [--new-file NEW_FILE]\n" +
	"                                 [--strict] [--self-test]\n"

// gnrHelp —— py `argparse --help` 的 stdout 逐字节内嵌（含末尾换行）。
const gnrHelp = gnrUsage + `
门76 新件登记完整性：件级 × 六张登记表（只报告 · 起手不接闸）

optional arguments:
  -h, --help            show this help message and exit
  --repo REPO           被查仓根（默认本仓）
  --baseline BASELINE   冻结基线（仓外生成件 · --dump-baseline 生成）
  --dump-baseline DUMP_BASELINE
                        把现读件集写成基线（仓外路径）后退出
  --inventory INVENTORY
                        面④ 仓外台账 tsv（不给 ⇒ 面④ 判不了 · 需人签）
  --new-file NEW_FILE   直接指分母（可重复 · 两态取证用）
  --strict              阻断档：R1–R5 任一 > 0 ⇒ rc=1（默认报告档 rc=0）
  --self-test           只跑合成夹具自检（不扫真仓）
`

func gnrUsageErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, gnrUsage)
	fmt.Fprintf(stderr, "check-newfile-registry.py: error: %s\n", msg)
	return 2
}

// checkNewfileRegistryGateMain —— 入口（= py `main()`）。
func checkNewfileRegistryGateMain(stdout, stderr io.Writer, argv []string) int {
	repo, repoSet := "", false
	baseline, baselineSet := "", false
	dump, dumpSet := "", false
	inv, invSet := "", false
	var newFiles []string
	strict, selfTest, help := false, false, false
	unrec := []string{}
	need := func(i int) (string, int, bool) {
		if i+1 < len(argv) {
			return argv[i+1], i + 1, true
		}
		return "", i, false
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			help = true
		case a == "--self-test":
			selfTest = true
		case a == "--strict":
			strict = true
		case a == "--repo":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --repo: expected one argument")
			}
			repo, repoSet, i = v, true, ni
		case strings.HasPrefix(a, "--repo="):
			repo, repoSet = a[len("--repo="):], true
		case a == "--baseline":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --baseline: expected one argument")
			}
			baseline, baselineSet, i = v, true, ni
		case strings.HasPrefix(a, "--baseline="):
			baseline, baselineSet = a[len("--baseline="):], true
		case a == "--dump-baseline":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --dump-baseline: expected one argument")
			}
			dump, dumpSet, i = v, true, ni
		case strings.HasPrefix(a, "--dump-baseline="):
			dump, dumpSet = a[len("--dump-baseline="):], true
		case a == "--inventory":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --inventory: expected one argument")
			}
			inv, invSet, i = v, true, ni
		case strings.HasPrefix(a, "--inventory="):
			inv, invSet = a[len("--inventory="):], true
		case a == "--new-file":
			v, ni, ok := need(i)
			if !ok {
				return gnrUsageErr(stderr, "argument --new-file: expected one argument")
			}
			newFiles = append(newFiles, v)
			i = ni
		case strings.HasPrefix(a, "--new-file="):
			newFiles = append(newFiles, a[len("--new-file="):])
		case strings.HasPrefix(a, "-") && a != "-":
			unrec = append(unrec, a)
		default:
			unrec = append(unrec, a)
		}
	}
	if help {
		fmt.Fprint(stdout, gnrHelp)
		return 0
	}
	if len(unrec) > 0 {
		return gnrUsageErr(stderr, "unrecognized arguments: "+strings.Join(unrec, " "))
	}
	if selfTest {
		return gnrSelfTest(stdout)
	}
	if !repoSet {
		repo = gnrDefaultRoot()
	}
	root := gnrAbs(repo)
	if dumpSet {
		face := gnrScanLedgerFace(root)
		if face == nil {
			fmt.Fprintf(stderr, "✗ 扫面不在盘：%s ⇒ 不写基线\n", filepath.Join(root, "scripts"))
			return 2
		}
		if err := os.MkdirAll(filepath.Dir(gnrAbs(dump)), 0o755); err != nil {
			fmt.Fprintf(stderr, "✗ 基线写不了：%s\n", err)
			return 2
		}
		names := []string{}
		for k := range face {
			names = append(names, k)
		}
		sort.Strings(names)
		var sb strings.Builder
		sb.WriteString("# 门76 冻结基线（现读件集 · 生成件 · 禁手维）\n")
		for _, f := range names {
			sb.WriteString(f + "\n")
		}
		if err := os.WriteFile(dump, []byte(sb.String()), 0o644); err != nil {
			fmt.Fprintf(stderr, "✗ 基线写不了：%s\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "基线已写：%s（%d 件）\n", dump, len(face))
		return 0
	}
	if !baselineSet {
		baseline = gnrDefaultBaseline()
	}
	nfSet := len(newFiles) > 0
	return gnrRun(stdout, root, newFiles, nfSet, inv, invSet, strict, baseline, baselineSet)
}
