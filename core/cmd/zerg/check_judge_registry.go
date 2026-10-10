// check_judge_registry.go —— 门禁件 `scripts/gates/check-judge-registry.py` 的 **Go 真搬**（M2 · 甲案 C 类第 12 枚）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案把判据实现面分两段走 —— **M1** 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面
//	（转发壳）；**M2** 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。
//	本件 = 甲案 C 类（判据注册表真源生成 + 双向差集自检）的一枚：
//	验收判据 = 对同一件跑两态（py 原件 ↔ 本件入口），**stdout / stderr / 退码三面逐字相同**。
//
// 本件做什么（照 py 真身逐条读出 · ★ 已同步 DEV-1142「拆两半」新版）：
//
//	`--emit`  —— 现读生成注册表（**派生半（甲）**）：递归 `<根>/scripts/` 全子树，只收 `.py` / `.sh`
//	              （目录名 `__pycache__` 不下钻），判据名 = 文件名去扩展名；每条落 8 个列面字段
//	              （id/name/path/cwd/interp/sha256/tool_version/cache_key），
//	              另有 schema/generated_by/roster_dir/extensions/min_tool_version/invocation/count/dupes/entries 九个顶层键。
//	              ★ DEV-1142：四字段（needs_env/serial/hazard/dry_run）**已搬进人签半作 `declare`** ⇒ 派生半**不再出**这四列。
//	`--check` —— 双向差集（盘上有件表里没点名 ⇒ 红；表里点名盘上无件 ⇒ 红）＋ 四条列面机检
//	              （路径越出 `scripts/` 前缀 / 判据名 ≠ 去扩展名 / 表内路径与盘上不符 /
//	              sha256 缓存键过期）＋ **第三方向 甲⇄乙**（人签半 `scripts/gates/judge-adoption.json` ·
//	              schema `judge-adoption/1`：三类决定 `adopt`/`exempt`/`declare` · `declare` 四字段闭集 ·
//	              `enforce` 迁移期与 `enforce_sig` 上牙机检 · 未决定脚注）＋ **第四腿「判据覆盖面」**
//	              （人签半 `covered_by` 声明 ⇄ 盘面 / 步骤表面 **双向对拍**：`file:` 盘上件 / `step:` 步名
//	              逐项正向解析 ＋ 反查同一对象不得被多判据同时认领）⇒ 空 = 绿。
//	`--self-test` —— 成对负控：合成夹具上正控绿 ＋ 拆两半成对负控（格A–格M）＋ 覆盖面成对负控（格N/格N2）
//	              ＋ 六条甲半负控各自必红。
//
// 遍历序（★ 本件逐处现读的结论）：py 用 `os.walk` 且**显式** `dns[:]=sorted(...)` / `sorted(fns)`
// ⇒ 遍历本身已排序；且 `scan_roster` 的产物**全部**再走 `sorted()`（`build_registry` 按 name 排序、
// `dupes` 排序、`_problem_list` 三处 `sorted(...)`）⇒ **输出行序与 readdir 原序无关**，故本件用
// `filepath.WalkDir`（字典序）即逐字同构；`__pycache__` 目录走 `fs.SkipDir` 复刻 py 的目录过滤。
//
// ★★ 标准声明 (a)：**默认根定位差异 —— 有差异**。
//
//	py 的 `--root` 缺省 = 脚本所在目录上溯三层的仓根（由 `__file__` 推出）；本件**无 `__file__`
//	语境** ⇒ 缺省换成**当前工作目录**（`os.Getwd()`）—— 与 check-judge-ledger / check-gotoolchain
//	等同批样板同一口径。落在**同一真仓、cwd = 仓根**时两者读数逐字相同（已对拍）；「脚本位置 ≠ cwd」
//	且都不给 `--root` 时路径推导面不同 ⇒ 这一面**不属逐字对齐承诺**，对拍一律**显式传 `--root`**。
//
// ★★ 标准声明 (b)：**自指检查 —— 逐形态真判**。
//
//	① 本件在**自己**的扫描面内吗？—— **否**。本门扫描面 = 调用方给的根下 `scripts/` 子树里的
//	   `.py` / `.sh` 件；本件落在 `core/cmd/zerg/` 下、后缀是 `.go` ⇒ 双条都不进面。
//	② 本件落在**任何他门**的扫描面内吗？—— **否**。本件正文**零私有卷 / 家目录绝对路径字面**、
//	   **零登记面件名清单**；若收口批现读发现某他门扫面含 `core/` 且判据含本件形态，按「拼串/转义」
//	   形态改写（第四形态处置）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func checkJudgeRegistryGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 绿 / 1 红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ⚠ 本单边界（照实）：只落码、不接线 —— 本件不新增命令树项（门79 的 Go 入口**早已**接进转发壳
// `gateCheckGoEntry` 的 `check-judge-registry` 一格）· 不改 `add_step` · 不删 py 原件。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 帮助 / 用法 / 错误正文（`-h` / 未知旗标 / 互斥三选冲突 / 缺值 / 前缀缩写）：
//	   py 打 argparse 生成的 usage 正文（含 prog 名、终端宽度）⇒ 本件只对齐**退码 2**，正文自定。
//	② 异常路径正文：注册表**读不到**（权限 / 不存在）py 打 `[Errno 2] …` 串；**非 JSON** py 打
//	   `json.JSONDecodeError` 串 ⇒ 本件同退码（2）· 正文为 Go 的 error 串。人签半同理。
//	③ `entries` **在场但非数组**（null / 数字 / 布尔）时 py 在迭代处抛异常 ⇒ 回溯 rc=1；
//	   本件 fail-closed rc=2 ＋ 具名文案（✗ 不动 py 原件）。
//	④ 非字符串 `name` / `id` 的排序与作键、浮点数的 Python repr、以及 `%r` 对**嵌套 dict 转义**的
//	   全枚举：本件对常规标量与数组逐字相同，对上述边角未做全枚举对拍。
//	⑤ 子进程面：`.sh` 件的 `tool_version` 走 `bash --version`（与 py 同形、结果缓存）；`.py` 件的
//	   `tool_version` py 用**运行解释器**的版本 ⇒ 本件改跑 `python3 --version` 取 PATH 上 python3 的
//	   版本 ⇒ 两者仅在「py 用非 PATH 的 python3 跑」时不同。
//	⑥ py `load_decisions` 对「件在盘但不可读」抛 `OSError`、对 JSON 解析失败抛 `ValueError`
//	   （JSONDecodeError 子类）⇒ 两条路 py 文案不同；本件按「读失败 / 解析·schema 失败」两档同构。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	jrSchema           = "judge-registry/1"
	jrDecisionsSchema  = "judge-adoption/1"
	jrRosterDir        = "scripts"
	jrRegistryDefault  = "scripts/gates/judge-registry.json"
	jrDecisionsDefault = "scripts/gates/judge-adoption.json"
	jrGeneratedBy      = "zerg gate check check-judge-registry --emit"
)

var jrInterp = map[string]string{".py": "python3", ".sh": "bash"}

// ── 闭集（拍板③ · 与 py 逐字同）──
//
//	jrDecisionValues    = DECISION_VALUES（决定值闭集 · 第三类 `declare` 与 adopt/exempt 并列）
//	jrAdoptionValues    = ADOPTION_VALUES（采纳面 = 只这两个值算「采纳/豁免」· 与 declare 正交）
//	jrDeclareFields     = DECLARE_FIELDS（declare 载荷四字段）
//	jrDeclareBoolFields = DECLARE_BOOL_FIELDS（布尔两值的那两个）
//	jrHazardValues      = HAZARD_VALUES（借 v1.2 P9 危险档闭集）
//	jrDryRunValues      = DRY_RUN_VALUES
//	jrCoverFaces        = COVER_FACES（「判据覆盖面」`covered_by` 覆盖对象引用**形态闭集**）
var (
	jrDecisionValues    = []string{"adopt", "exempt", "declare"}
	jrAdoptionValues    = []string{"adopt", "exempt"}
	jrDeclareFields     = []string{"needs_env", "serial", "hazard", "dry_run"}
	jrDeclareBoolFields = []string{"needs_env", "serial"}
	jrHazardValues      = []string{"none", "swap", "replace", "publish", "kill"}
	jrDryRunValues      = []string{"none", "argv", "noop"}
	// jrCoverFaces —— `covered_by` 覆盖对象引用的「面」闭集：`file:<仓内相对路径>` ⇒ 盘上普通件；
	// `step:<步名>` ⇒ 门禁脚本自己的 `add_step` 步骤表里的步名。
	jrCoverFaces = []string{"file", "step"}
)

var jrBashVerCache *string

// ── 顶层件结构：键序与类型逐条对齐 py 的 build_registry 返回字面 ──
//
// ★ 手工拼 JSON 易错（转义/缩进/键序三面），故走 encoding/json ＋ SetEscapeHTML(false)
// ＋ SetIndent("", "  ")，复刻 `json.dumps(reg, ensure_ascii=False, indent=2) + "\n"`。

type jrMinToolVersion struct {
	Bash   string `json:"bash"`
	Python string `json:"python"`
}

type jrInvocation struct {
	Argv       string `json:"argv"`
	ExpectedRC string `json:"expected_rc"`
	Note       string `json:"note"`
}

// jrEntry —— ★ DEV-1142：四字段（needs_env/serial/hazard/dry_run）**已搬进人签半** ⇒ 派生半只留 8 列。
type jrEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Cwd         string `json:"cwd"`
	Interp      string `json:"interp"`
	SHA256      string `json:"sha256"`
	ToolVersion string `json:"tool_version"`
	CacheKey    string `json:"cache_key"`
}

type jrRegistry struct {
	Schema         string           `json:"schema"`
	GeneratedBy    string           `json:"generated_by"`
	RosterDir      string           `json:"roster_dir"`
	Extensions     []string         `json:"extensions"`
	MinToolVersion jrMinToolVersion `json:"min_tool_version"`
	Invocation     jrInvocation     `json:"invocation"`
	Count          int              `json:"count"`
	Dupes          []string         `json:"dupes"`
	Entries        []jrEntry        `json:"entries"`
}

// jrNote 与 py 的 note 字面逐字相同。
const jrNote = "入口参数与期望退码由转发壳逐字透传（`zerg gate check <判据名> [其余逐字给子件]`）；" +
	"0/1/2 三码透传，件自身崩/超时 ⇒ 壳 8 · judge_broken/timeout。"

// ── py 字符串原语（逐处自写同构）──

// jrPySplitExt 照 CPython posixpath.splitext 直译（`filepath.Ext` 对被点开头的件名判法不同）。
func jrPySplitExt(p string) (string, string) {
	sepIndex := strings.LastIndex(p, "/")
	dotIndex := strings.LastIndex(p, ".")
	if dotIndex > sepIndex {
		filenameIndex := sepIndex + 1
		for filenameIndex < dotIndex {
			if p[filenameIndex] != '.' {
				return p[:dotIndex], p[dotIndex:]
			}
			filenameIndex++
		}
	}
	return p, ""
}

// jrPyBaseName 照 CPython posixpath.basename（= split(p)[1] = p[rfind('/')+1:]）。
func jrPyBaseName(p string) string {
	i := strings.LastIndex(p, "/")
	return p[i+1:]
}

// jrPyStrRepr 复刻 Python `repr(str)`（常规标量面 · 嵌套面见件头未覆盖④）。
func jrPyStrRepr(s string) string {
	quote := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString("\\\\")
		case r < 128 && byte(r) == quote:
			b.WriteByte('\\')
			b.WriteByte(quote)
		case r == '\t':
			b.WriteString("\\t")
		case r == '\n':
			b.WriteString("\\n")
		case r == '\r':
			b.WriteString("\\r")
		case r < 0x20 || r == 0x7f:
			b.WriteString(fmt.Sprintf("\\x%02x", r))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// jrPyNumRepr 复刻 Python 数字 repr 的常规面（整数原样 · 浮点去尾零时补 `.0`）。
func jrPyNumRepr(n string) string {
	if isIntLiteral(n) {
		return n
	}
	f, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return n
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// jrPyRepr 复刻 Python `repr(v)`（有序 JSON 值 · 供 `%r` 档与 str(list) 档复用）。
func jrPyRepr(v *jv) string {
	if v == nil {
		return "None"
	}
	switch v.kind {
	case 'n':
		return "None"
	case 'b':
		if v.b {
			return "True"
		}
		return "False"
	case 'i':
		return jrPyNumRepr(v.num)
	case 's':
		return jrPyStrRepr(v.s)
	case 'a':
		parts := make([]string, 0, len(v.arr))
		for _, e := range v.arr {
			parts = append(parts, jrPyRepr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case 'o':
		parts := make([]string, 0, len(v.obj))
		for _, kv := range v.obj {
			parts = append(parts, jrPyStrRepr(kv.k)+": "+jrPyRepr(kv.v))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "None"
}

// jrPyStr 复刻 Python `"%s" % v`（字符串取原串 · 其余取 repr）。
func jrPyStr(v *jv) string {
	if v != nil && v.kind == 's' {
		return v.s
	}
	return jrPyRepr(v)
}

// jrPyList 复刻 Python `str(list-of-str)`（供自检负控与闭集 `list(...)` 档的 `%s`）。
func jrPyList(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, jrPyStrRepr(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// jrPyTruthy 复刻 Python 真值判定（供 `if d.get("reason") else ""` / 三件套齐判定复用）。
func jrPyTruthy(v *jv) bool {
	if v == nil {
		return false
	}
	switch v.kind {
	case 'n':
		return false
	case 'b':
		return v.b
	case 'i':
		return v.num != "0" && v.num != "0.0"
	case 's':
		return v.s != ""
	case 'a':
		return len(v.arr) > 0
	case 'o':
		return len(v.obj) > 0
	}
	return false
}

// ── 有序 JSON 值的构造 / 克隆 / 改键（供人签半夹具复刻 py 的 dict 字面量与 json 往返）──

func jrNewStr(s string) *jv { return &jv{kind: 's', s: s} }
func jrNewBool(b bool) *jv  { return &jv{kind: 'b', b: b} }
func jrNewArr(items ...*jv) *jv {
	return &jv{kind: 'a', arr: items}
}
func jrNewObj(pairs ...jpair) *jv { return &jv{kind: 'o', obj: pairs} }

// jrClone 复刻 `json.loads(json.dumps(x))`（深拷贝 · 保 key 序）。
func jrClone(v *jv) *jv {
	if v == nil {
		return nil
	}
	c := &jv{kind: v.kind, b: v.b, num: v.num, s: v.s}
	if v.arr != nil {
		c.arr = make([]*jv, len(v.arr))
		for i, e := range v.arr {
			c.arr[i] = jrClone(e)
		}
	}
	if v.obj != nil {
		c.obj = make([]jpair, len(v.obj))
		for i, kv := range v.obj {
			c.obj[i] = jpair{kv.k, jrClone(kv.v)}
		}
	}
	return c
}

// jrSet 复刻 py `d[k] = v`（键在 ⇒ 原位替换 · 键不在 ⇒ 追加）。
func jrSet(o *jv, k string, v *jv) {
	if o == nil || o.kind != 'o' {
		return
	}
	for i := range o.obj {
		if o.obj[i].k == k {
			o.obj[i].v = v
			return
		}
	}
	o.obj = append(o.obj, jpair{k, v})
}

// jrDecisionsArr 取 `decisions` 数组（缺 / 非数组 ⇒ 空表）。
func jrDecisionsArr(o *jv) []*jv {
	if dv := jget(o, "decisions"); dv != nil && dv.kind == 'a' {
		return dv.arr
	}
	return nil
}

func jrAppendDecision(o *jv, d *jv) {
	if dv := jget(o, "decisions"); dv != nil && dv.kind == 'a' {
		dv.arr = append(dv.arr, d)
	}
}

func jrFilterDecisions(o *jv, keep func(*jv) bool) {
	dv := jget(o, "decisions")
	if dv == nil || dv.kind != 'a' {
		return
	}
	out := make([]*jv, 0, len(dv.arr))
	for _, d := range dv.arr {
		if keep(d) {
			out = append(out, d)
		}
	}
	dv.arr = out
}

func jrForEachDecision(o *jv, f func(*jv)) {
	for _, d := range jrDecisionsArr(o) {
		f(d)
	}
}

// jrGetOr 复刻 py `d.get(k, def)`（缺键 ⇒ def · 显式 null ⇒ "None"）。
func jrGetOr(o *jv, k, def string) string {
	if v := jget(o, k); v != nil {
		return jrPyStr(v)
	}
	return def
}

// jrInList 复刻 py `x in (a, b, …)`（字符串面）。
func jrInList(s string, xs []string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// jrSplitCoverRef 切覆盖对象引用 `"<面>:<对象>"`（首个 `:` 为界；无 `:` ⇒ 面 = 整串、对象 = ""）。
// 供闭集机检（面 ∉ 闭集 或 对象为空 ⇒ 调用方报红）。
func jrSplitCoverRef(s string) (string, string) {
	if i := strings.Index(s, ":"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// jrStepNames 现读步骤表（真源 = 门禁脚本自己的 `add_step` 声明 · 与 `gate` 族同一处解析
// `parseAddStepsStrict`）⇒ 步名集合；表读不到 ⇒ 空集（调用方据「是否声明了 `step:` 引用」决定要不要报错）。
func jrStepNames(root string) map[string]bool {
	out := map[string]bool{}
	b, err := os.ReadFile(pyJoin(root, gateScriptRel))
	if err != nil {
		return out
	}
	decls, _ := parseAddStepsStrict(string(b))
	for _, d := range decls {
		out[d.Name] = true
	}
	return out
}

// ── 现读读数 ──

// jrSHA256Hex 照 py sha256_file（分块读 · 逐字节同）。
func jrSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// jrPythonVersion 现读 PATH 上 python3 的版本（对应 py 的 platform.python_version()）。
func jrPythonVersion() string {
	out, err := exec.Command("python3", "--version").CombinedOutput()
	if err != nil {
		return "unknown"
	}
	for _, part := range strings.Fields(string(out)) {
		if len(part) > 0 && part[0] >= '0' && part[0] <= '9' {
			return part
		}
	}
	return "unknown"
}

// jrBashVersion 照 py 的 `bash --version` 解析（取首行中首个数字起头的 token · 截到 `(` 前）。
func jrBashVersion() string {
	out, err := exec.Command("bash", "--version").CombinedOutput()
	if err != nil {
		return "unknown"
	}
	first := ""
	if lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n"); len(lines) > 0 {
		first = lines[0]
	}
	for _, part := range strings.Fields(strings.ReplaceAll(first, "，", " ")) {
		if len(part) > 0 && part[0] >= '0' && part[0] <= '9' {
			return strings.Split(part, "(")[0]
		}
	}
	return "unknown"
}

// jrToolVersion 照 py 的 tool_version（python3 ⇒ 运行解释器版本 · bash ⇒ 现读并缓存）。
func jrToolVersion(interp string) string {
	switch interp {
	case "python3":
		return "python3/" + jrPythonVersion()
	case "bash":
		if jrBashVerCache == nil {
			v := jrBashVersion()
			jrBashVerCache = &v
		}
		return "bash/" + *jrBashVerCache
	}
	return interp + "/unknown"
}

// jrScanRoster 照 py scan_roster：递归 `<root>/scripts/` · 只收 `.py`/`.sh` · 不下钻 `__pycache__`
// ⇒ {名: [仓相对路径…]}（每名下的路径再 sort · 与 py 逐字同）。
func jrScanRoster(root string) map[string][]string {
	base := pyJoin(root, jrRosterDir)
	paths := map[string][]string{}
	_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // py os.walk onerror=None：读不了的目录静默跳过
		}
		if d.IsDir() {
			if d.Name() == "__pycache__" {
				return fs.SkipDir
			}
			return nil
		}
		name, ext := jrPySplitExt(d.Name())
		if _, ok := jrInterp[ext]; !ok {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		paths[name] = append(paths[name], filepath.ToSlash(rel))
		return nil
	})
	for _, v := range paths {
		sort.Strings(v)
	}
	return paths
}

// jrBuildRegistry 照 py build_registry（确定性：条目按 name 排序 · 键序固定）。
// ★ 与人签半**无入参耦合**（构造上不可能覆盖人的决定 · 自检格H）。
func jrBuildRegistry(root string) *jrRegistry {
	paths := jrScanRoster(root)
	dupes := []string{}
	for n, v := range paths {
		if len(v) > 1 {
			dupes = append(dupes, n)
		}
	}
	sort.Strings(dupes)
	names := make([]string, 0, len(paths))
	for n := range paths {
		names = append(names, n)
	}
	sort.Strings(names)
	entries := []jrEntry{}
	for _, name := range names {
		if len(paths[name]) != 1 {
			continue
		}
		rel := paths[name][0]
		_, ext := jrPySplitExt(rel)
		interp := jrInterp[ext]
		tv := jrToolVersion(interp)
		sha, err := jrSHA256Hex(pyJoin(root, rel))
		if err != nil {
			sha = ""
		}
		entries = append(entries, jrEntry{
			ID: name, Name: name, Path: rel, Cwd: ".", Interp: interp,
			SHA256: sha, ToolVersion: tv, CacheKey: sha + "|" + tv,
		})
	}
	return &jrRegistry{
		Schema:      jrSchema,
		GeneratedBy: jrGeneratedBy,
		RosterDir:   jrRosterDir,
		Extensions:  []string{".py", ".sh"},
		MinToolVersion: jrMinToolVersion{
			Bash:   "3.2",
			Python: "3.9",
		},
		Invocation: jrInvocation{
			Argv:       "passthrough",
			ExpectedRC: "passthrough",
			Note:       jrNote,
		},
		Count:   len(entries),
		Dupes:   dupes,
		Entries: entries,
	}
}

// jrMarshalRegistry 复刻 `json.dumps(reg, ensure_ascii=False, indent=2) + "\n"`。
func jrMarshalRegistry(reg *jrRegistry) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // ★ py ensure_ascii=False 不转义 < > &
	enc.SetIndent("", "  ")
	_ = enc.Encode(reg) // Encode 自带尾换行 ⇒ 与 py 的 +"\n" 等值
	return buf.String()
}

// ── 人签半（乙）读取与机检 ──

// jrLoadDecisions 照 py load_decisions：件不在盘 ⇒ (nil, "")（**不是错** · 迁移期）；
// 读失败 / 解析·schema 失败 ⇒ (nil, 具名文案)。
func jrLoadDecisions(path string) (*jv, string) {
	st, serr := os.Stat(path)
	if serr != nil || st.IsDir() { // py `not os.path.isfile(path)` ⇒ None
		return nil, ""
	}
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		return nil, fmt.Sprintf("⛔ 人签半读不到：%s ⇒ 不给结论（rc=2）", rerr)
	}
	dec, perr := jparse(data)
	if perr != nil {
		return nil, fmt.Sprintf("⛔ 人签半加载期硬错：%s ⇒ 不给结论（rc=2）", perr)
	}
	if dec == nil || dec.kind != 'o' || jrSchemaOf(dec) != jrDecisionsSchema {
		present := "None"
		if dec != nil && dec.kind == 'o' {
			if sv := jget(dec, "schema"); sv != nil {
				present = jrPyRepr(sv)
			}
		} else {
			present = jrPyRepr(dec)
		}
		inner := fmt.Sprintf("人签半 schema 不符（须 `%s` · 现读 %s）", jrDecisionsSchema, present)
		return nil, fmt.Sprintf("⛔ 人签半加载期硬错：%s ⇒ 不给结论（rc=2）", inner)
	}
	return dec, ""
}

// jrDeclareFieldProblem 照 py _declare_field_problem ⇒ 问题串 or ""。
func jrDeclareFieldProblem(did, key string, val *jv) string {
	if jrInList(key, jrDeclareBoolFields) {
		if val == nil || val.kind != 'b' {
			return fmt.Sprintf("declare 载荷 `%s` 非布尔（闭集 true/false · 现读 %s）：%s", key, jrPyRepr(val), did)
		}
		return ""
	}
	closed := jrDryRunValues
	if key == "hazard" {
		closed = jrHazardValues
	}
	if val == nil || val.kind != 's' || !jrInList(val.s, closed) {
		return fmt.Sprintf("declare 载荷 `%s` 值不在闭集 %s：%s（现读 %s）", key, jrPyList(closed), did, jrPyRepr(val))
	}
	return ""
}

// jrAdoptionUndecided 照 py _adoption_undecided：在册中**没有采纳/豁免决定**的判据名 · sorted。
func jrAdoptionUndecided(reg *jv, decisions *jv) []string {
	names := []string{}
	if reg != nil {
		if ev := jget(reg, "entries"); ev != nil && ev.kind == 'a' {
			for _, e := range ev.arr {
				if e != nil && e.kind == 'o' && jpresent(e, "name") {
					names = append(names, jrPyStr(jget(e, "name")))
				}
			}
		}
	}
	sort.Strings(names)
	if decisions == nil {
		return names
	}
	adopted := map[string]bool{}
	for _, d := range jrDecisionsArr(decisions) {
		if d == nil || d.kind != 'o' {
			continue
		}
		if !jrInList(jrPyStr(jget(d, "decision")), jrAdoptionValues) {
			continue
		}
		adopted[jrPyStr(jget(d, "id"))] = true
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if !adopted[n] {
			out = append(out, n)
		}
	}
	return out
}

// jrEnforceValue 照 py `decisions.get("enforce", ENFORCE_DEFAULT)`（缺键 ⇒ False）。
func jrEnforceValue(decisions *jv) *jv {
	if v := jget(decisions, "enforce"); v != nil {
		return v
	}
	return jrNewBool(false)
}

// jrDecisionStr 照 py `str(x.get("decision"))`（排序键用）。
func jrDecisionStr(d *jv) string { return jrPyStr(jget(d, "decision")) }

// jrDecisionNotes 照 py _decision_notes：乙半**可见面**（✗ 不判红）。
//
// ★ 输出行序的决定者：`enforce` 状态行在最前；逐条备注按 `(id, str(decision))` 排序；
//
//	未决定行在最末，名字按 sorted。⇒ 零不确定序。
func jrDecisionNotes(decisions *jv, reg *jv) []string {
	if decisions == nil {
		n := 0
		if reg != nil {
			n = len(jrAdoptionUndecided(reg, nil))
		}
		return []string{fmt.Sprintf("人签半（`%s`）未建 ⇒ **迁移期**：乙半面只报告不判红（未决定 %d 条）",
			jrDecisionsDefault, n)}
	}
	enforce := jrEnforceValue(decisions)
	enforceTrue := enforce.kind == 'b' && enforce.b
	line2 := "迁移期：在册无采纳/豁免决定**不判红**，只报计数"
	if enforceTrue {
		line2 = "**上牙**：在册无采纳/豁免决定 ⇒ 判红"
	}
	notes := []string{fmt.Sprintf("人签半 `enforce`=%s ⇒ %s", jrPyStr(enforce), line2)}

	type jrNoteRow struct {
		d      *jv
		id, dv string
	}
	rows := []jrNoteRow{}
	for _, d := range jrDecisionsArr(decisions) {
		if d == nil || d.kind != 'o' || !jpresent(d, "id") {
			continue
		}
		rows = append(rows, jrNoteRow{d, jrPyStr(jget(d, "id")), jrDecisionStr(d)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].id != rows[j].id {
			return rows[i].id < rows[j].id
		}
		return rows[i].dv < rows[j].dv
	})
	for _, r := range rows {
		d := r.d
		v := jrDecisionStr(d)
		var tag, tail string
		switch {
		case v == "declare":
			f := jget(d, "fields")
			if f == nil || f.kind != 'o' {
				f = jrNewObj()
			}
			tag = "声明（四字段）"
			parts := make([]string, 0, len(jrDeclareFields))
			for _, k := range jrDeclareFields {
				parts = append(parts, fmt.Sprintf("%s=%s", k, jrPyRepr(jget(f, k))))
			}
			tail = strings.Join(parts, " · ")
		case v == "exempt":
			tag = "豁免"
			if jrPyTruthy(jget(d, "reason")) {
				tail = jrPyStr(jget(d, "reason"))
			}
		case v == "adopt":
			tag = "采纳"
		default:
			tag = fmt.Sprintf("决定值异常(%s)", jrPyRepr(jget(d, "decision")))
		}
		notes = append(notes, fmt.Sprintf("%s：%s（by %s · %s）%s",
			tag, r.id, jrGetOr(d, "by", "?"), jrGetOr(d, "at", "?"), tail))
	}
	if reg != nil {
		und := jrAdoptionUndecided(reg, decisions)
		if len(und) > 0 {
			suffix := "迁移期只报"
			if enforceTrue {
				suffix = "上牙 ⇒ 判红"
			}
			notes = append(notes, fmt.Sprintf("未决定 %d 条（在册无采纳/豁免决定 · %s）：%s",
				len(und), suffix, strings.Join(und, " · ")))
		}
	}
	return notes
}

// jrPairState 照 py _pair_state（绿行用 · 与 _decision_notes 同源）。
func jrPairState(decisions *jv) string {
	if decisions == nil {
		return "迁移期（人签半未建）"
	}
	if v := jrEnforceValue(decisions); v.kind == 'b' && v.b {
		return "对拍过（enforce=true 上牙）"
	}
	return "对拍过（enforce=false 迁移期）"
}

// jrProblemList 照 py _problem_list：双向差集 ＋ 列面机检 ＋ **甲⇄乙 第三方向** ⇒ 问题清单（空 = 绿）。
// 第二返回值 = 是否给得出结论（false ⇒ 调用方 rc=2 fail-closed · 见件头未覆盖③）。
func jrProblemList(root string, reg *jv, decisions *jv) ([]string, bool) {
	problems := []string{}
	live := jrScanRoster(root)
	liveDupes := []string{}
	for n, v := range live {
		if len(v) > 1 {
			liveDupes = append(liveDupes, n)
		}
	}
	sort.Strings(liveDupes)
	for _, n := range liveDupes {
		problems = append(problems, fmt.Sprintf("盘上同名多件（名字 ⇒ 件 不唯一）：%s（%s）", n, strings.Join(live[n], " · ")))
	}

	regEntries := map[string]*jv{}
	entriesVal := jget(reg, "entries")
	var entriesArr []*jv
	if entriesVal == nil {
		entriesArr = nil // 缺键 ⇒ py `.get("entries", [])` 取默认空表
	} else if entriesVal.kind == 'a' {
		entriesArr = entriesVal.arr
	} else {
		return nil, false // py 在此迭代非数组 ⇒ 未捕异常（件头未覆盖③）
	}
	for _, e := range entriesArr {
		if e == nil || e.kind != 'o' || !jpresent(e, "name") {
			problems = append(problems, fmt.Sprintf("注册表条目缺 `name`：%s", jrPyRepr(e)))
			continue
		}
		name := jrPyStr(jget(e, "name")) // py `e["name"]`：字符串取原串
		if _, dup := regEntries[name]; dup {
			problems = append(problems, fmt.Sprintf("注册表内同名重复条目：%s", name))
		}
		regEntries[name] = e
	}

	liveNames := make([]string, 0, len(live))
	for n := range live {
		liveNames = append(liveNames, n)
	}
	sort.Strings(liveNames)
	for _, n := range liveNames {
		if _, ok := regEntries[n]; !ok {
			problems = append(problems, fmt.Sprintf("盘上有件、注册表**没点名**（新件未登记）：%s（%s）", n, strings.Join(live[n], " · ")))
		}
	}
	regNames := make([]string, 0, len(regEntries))
	for n := range regEntries {
		regNames = append(regNames, n)
	}
	sort.Strings(regNames)
	for _, n := range regNames {
		if _, ok := live[n]; !ok {
			pv := jget(regEntries[n], "path")
			pathStr := "?"
			if pv != nil {
				pathStr = jrPyStr(pv)
			}
			problems = append(problems, fmt.Sprintf("注册表**点名**、盘上无件（悬空登记）：%s（%s）", n, pathStr))
		}
	}

	both := []string{}
	for _, n := range liveNames {
		if _, ok := regEntries[n]; ok {
			both = append(both, n)
		}
	}
	for _, n := range both {
		e := regEntries[n]
		relVal := jget(e, "path")
		if relVal == nil {
			relVal = &jv{kind: 's', s: ""} // py `.get("path", "")`
		}
		if relVal.kind != 's' || !strings.HasPrefix(relVal.s, jrRosterDir+"/") {
			problems = append(problems, fmt.Sprintf("件名 ⇒ 路径越出 `<仓根>/%s/` 前缀：%s（%s）", jrRosterDir, n, jrPyStr(relVal)))
			continue
		}
		rel := relVal.s
		base, _ := jrPySplitExt(jrPyBaseName(rel))
		if base != n {
			problems = append(problems, fmt.Sprintf("判据名 ≠ 件名去扩展名逐字相同：表内 %s ⇄ 件 %s", n, rel))
		}
		if len(live[n]) == 1 && rel != live[n][0] {
			problems = append(problems, fmt.Sprintf("表内路径与盘上现读不符：%s ⇄ %s", rel, live[n][0]))
		}
		full := pyJoin(root, rel)
		st, serr := os.Stat(full)
		if serr != nil || st.IsDir() { // py os.path.isfile（跟随符号链接）
			problems = append(problems, fmt.Sprintf("表内点名的件在盘上不存在：%s", rel))
			continue
		}
		sha, herr := jrSHA256Hex(full)
		shaWanted := ""
		if sv := jget(e, "sha256"); sv != nil && sv.kind == 's' {
			shaWanted = sv.s
		}
		if herr != nil || sha != shaWanted {
			problems = append(problems, fmt.Sprintf("缓存键过期（sha256 不符 · 请 `--emit` 重生成）：%s", rel))
		}
	}

	// ── 第三方向：派生半（在册）⇄ 人签半（采纳 / 豁免 / 声明）· fail-closed ──────────────────
	if decisions != nil {
		enforce := jrEnforceValue(decisions)
		if enforce.kind != 'b' {
			problems = append(problems, fmt.Sprintf("人签半 `enforce` 非布尔（闭集 true/false · 现读 %s）⇒ 按 `false`（迁移期）读", jrPyRepr(enforce)))
			enforce = jrNewBool(false)
		}
		// ★ 上牙（enforce=true）本身必须人签（拍板③）
		if enforce.kind == 'b' && enforce.b {
			esig := jget(decisions, "enforce_sig")
			okSig := esig != nil && esig.kind == 'o' &&
				jrPyTruthy(jget(esig, "by")) && jrPyTruthy(jget(esig, "at")) && jrPyTruthy(jget(esig, "sig"))
			if !okSig {
				problems = append(problems, "人签半 `enforce=true`（**上牙**）**未人签**（缺顶层 `enforce_sig` 的 `by`/`at`/`sig` 之一）")
			}
		}
		adopted := map[string]bool{}
		declared := map[string]bool{}
		for _, d := range jrDecisionsArr(decisions) {
			if d == nil || d.kind != 'o' || !jpresent(d, "id") {
				problems = append(problems, fmt.Sprintf("人签半条目缺 `id`：%s", jrPyRepr(d)))
				continue
			}
			did := jrPyStr(jget(d, "id"))
			v := jrPyStr(jget(d, "decision"))
			if !jrInList(v, jrDecisionValues) {
				problems = append(problems, fmt.Sprintf("人签半决定值不在闭集 %s：%s（现读 %s）",
					jrPyList(jrDecisionValues), did, jrPyRepr(jget(d, "decision"))))
			}
			if !(jrPyTruthy(jget(d, "by")) && jrPyTruthy(jget(d, "at")) && jrPyTruthy(jget(d, "sig"))) {
				problems = append(problems, fmt.Sprintf("人签半决定**未人签**（缺 `by`/`at`/`sig` 之一）：%s", did))
			}
			if jrInList(v, jrAdoptionValues) {
				if adopted[did] {
					problems = append(problems, fmt.Sprintf("人签半内同一条目**采纳/豁免重复或矛盾**：%s", did))
				}
				adopted[did] = true
			} else if v == "declare" {
				if declared[did] {
					problems = append(problems, fmt.Sprintf("人签半内同一条目**declare 重复**：%s", did))
				}
				declared[did] = true
				fields := jget(d, "fields")
				if fields == nil || fields.kind != 'o' {
					problems = append(problems, fmt.Sprintf("declare 缺载荷 `fields`（四字段 %s）：%s", jrPyList(jrDeclareFields), did))
				} else {
					for _, k := range jrDeclareFields {
						if !jpresent(fields, k) {
							problems = append(problems, fmt.Sprintf("declare 载荷缺字段 `%s`：%s", k, did))
							continue
						}
						if pb := jrDeclareFieldProblem(did, k, jget(fields, k)); pb != "" {
							problems = append(problems, pb)
						}
					}
				}
			}
		}
		// 采纳面：在册无采纳/豁免决定 ⇒ **受 enforce 控**（迁移期只脚注 · 上牙判红）
		undecided := jrAdoptionUndecided(reg, decisions)
		if enforce.kind == 'b' && enforce.b {
			for _, n := range undecided {
				problems = append(problems, fmt.Sprintf("在册判据**无人签采纳/豁免决定**（`enforce=true` 上牙）：%s", n))
			}
		}
		// 悬空：决定点名不在册、盘上也无件（两轴合计 · 与 enforce 无关 —— 这是写错，不是迁移期）
		seen := map[string]bool{}
		for k := range adopted {
			seen[k] = true
		}
		for k := range declared {
			seen[k] = true
		}
		dangling := make([]string, 0, len(seen))
		for k := range seen {
			dangling = append(dangling, k)
		}
		sort.Strings(dangling)
		for _, n := range dangling {
			if _, ok := regEntries[n]; !ok {
				if _, ok2 := live[n]; !ok2 {
					problems = append(problems, fmt.Sprintf("人签半**悬空**（点名不在册、盘上也无件）：%s", n))
				}
			}
		}

		// ── 第四腿：「判据覆盖面」`covered_by` 双向对拍（人签半声明 ⇄ 盘面 / 步骤表面）──────────────
		// 正向（声明 ⇒ 面）：每个覆盖对象须在指名那一面解析得到 —— `file:<仓内相对路径>` ⇒ 盘上普通件；
		//   `step:<步名>` ⇒ 步骤表（门禁脚本自己的 `add_step`）里的步名。解析不到 ⇒ 红（该面点名）。
		// 反向（面 ⇒ 声明 · 反查）：同一解析目标不得被两个判据同时认领（对象 ⇒ 认领判据 反查索引去重）。
		// ★ `covered_by` 属**人签半**声明：✗ 不进派生半（`--emit` 产物与人签半字段逐字分离 · 见格A）。
		var coverSteps map[string]bool // 步名集合（懒读一次 · 局部量 ⇒ 无跨调用残留）
		coverIndex := map[string][]string{}
		for _, d := range jrDecisionsArr(decisions) {
			if d == nil || d.kind != 'o' || !jpresent(d, "id") {
				continue // 缺 `id` 已在上文报过
			}
			cv := jget(d, "covered_by")
			if cv == nil {
				continue // 未声明覆盖面 —— 合法（迁移期无声明）
			}
			did := jrPyStr(jget(d, "id"))
			if cv.kind != 'a' {
				problems = append(problems, fmt.Sprintf("人签半 `covered_by` 非数组（须覆盖对象引用表 · 现读 %s）：%s", jrPyRepr(cv), did))
				continue
			}
			for _, ref := range cv.arr {
				if ref == nil || ref.kind != 's' {
					problems = append(problems, fmt.Sprintf("覆盖对象引用非字符串（现读 %s）：%s", jrPyRepr(ref), did))
					continue
				}
				face, tail := jrSplitCoverRef(ref.s)
				if !jrInList(face, jrCoverFaces) || tail == "" {
					problems = append(problems, fmt.Sprintf("覆盖对象引用形态越出闭集 %s（须 `<面>:<对象>`）：%s（现读 %s）", jrPyList(jrCoverFaces), did, jrPyRepr(ref)))
					continue
				}
				resolved := false
				switch face {
				case "file":
					if st, err := os.Stat(pyJoin(root, tail)); err == nil && st.Mode().IsRegular() {
						resolved = true
					} else {
						problems = append(problems, fmt.Sprintf("覆盖对象（`file:%s`）在盘上解析不到（须仓内普通件）：%s", tail, did))
					}
				case "step":
					if coverSteps == nil {
						coverSteps = jrStepNames(root)
					}
					if coverSteps[tail] {
						resolved = true
					} else {
						problems = append(problems, fmt.Sprintf("覆盖对象（`step:%s`）在步骤表里解析不到（须 `%s` 里 `add_step` 的步名）：%s", tail, gateScriptRel, did))
					}
				}
				if resolved {
					coverIndex[ref.s] = append(coverIndex[ref.s], did)
				}
			}
		}
		coverKeys := make([]string, 0, len(coverIndex))
		for k := range coverIndex {
			coverKeys = append(coverKeys, k)
		}
		sort.Strings(coverKeys)
		for _, k := range coverKeys {
			uniq := map[string]bool{}
			for _, id := range coverIndex[k] {
				uniq[id] = true
			}
			if len(uniq) > 1 {
				ids := make([]string, 0, len(uniq))
				for id := range uniq {
					ids = append(ids, id)
				}
				sort.Strings(ids)
				problems = append(problems, fmt.Sprintf("覆盖对象**反查冲突**（同一对象被多判据同时认领）：%s（%s）", k, strings.Join(ids, " · ")))
			}
		}
	}
	return problems, true
}

// jrLoadRegistry 照 py load_registry：读文件 → 有序 JSON → schema 校验。
// 第二返回 = 校验失败文案（空 = 通过）。
func jrLoadRegistry(path string) (*jv, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Sprintf("⛔ 注册表读不到：%s ⇒ 不给结论（rc=2）", err)
	}
	reg, perr := jparse(data)
	if perr != nil {
		return nil, fmt.Sprintf("⛔ 注册表加载期硬错：%s ⇒ 不给结论（rc=2）", perr)
	}
	if reg == nil || reg.kind != 'o' || jrSchemaOf(reg) != jrSchema {
		present := "None"
		if reg != nil && reg.kind == 'o' {
			if sv := jget(reg, "schema"); sv != nil {
				present = jrPyRepr(sv)
			}
		} else {
			present = jrPyRepr(reg)
		}
		return nil, fmt.Sprintf("⛔ 注册表加载期硬错：schema 不符（须 `%s` · 现读 %s） ⇒ 不给结论（rc=2）", jrSchema, present)
	}
	return reg, ""
}

// jrSchemaOf 取对象 `schema` 字符串（缺 / 非串 ⇒ ""）。
func jrSchemaOf(o *jv) string {
	if sv := jget(o, "schema"); sv != nil && sv.kind == 's' {
		return sv.s
	}
	return ""
}

// jrCmdEmit 照 py cmd_emit（**只写派生半** · ✗ 不读 / 不写人签半）。
func jrCmdEmit(root, out string, stdout, stderr io.Writer) int {
	reg := jrBuildRegistry(root)
	text := jrMarshalRegistry(reg)
	if out == "-" {
		io.WriteString(stdout, text)
	} else {
		if d := pyDirname(out); d != "" {
			if st, err := os.Stat(d); err != nil || !st.IsDir() {
				_ = os.MkdirAll(d, 0o755)
			}
		}
		if err := os.WriteFile(out, []byte(text), 0o644); err != nil {
			fmt.Fprintf(stderr, "⛔ 注册表写不出：%s ⇒ 不给结论（rc=2）\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "判据注册表（派生半）已生成：%s（条目 %d · 同名歧义 %d）\n", out, reg.Count, len(reg.Dupes))
	}
	if len(reg.Dupes) > 0 {
		fmt.Fprintf(stderr, "⚠ 盘上有同名多件（%d）：%s\n", len(reg.Dupes), strings.Join(reg.Dupes, " · "))
	}
	return 0
}

// jrCmdCheck 照 py cmd_check（甲半双向差集 ＋ 甲⇄乙 第三方向 · fail-closed）。
func jrCmdCheck(root, regPath, decPath string, stdout, stderr io.Writer) int {
	reg, errMsg := jrLoadRegistry(regPath)
	if errMsg != "" {
		fmt.Fprintln(stderr, errMsg)
		return 2
	}
	decisions, derr := jrLoadDecisions(decPath)
	if derr != "" {
		fmt.Fprintln(stderr, derr)
		return 2
	}
	// 乙半可见面（✗ 不改退码）：enforce 状态 + 逐条采纳/豁免/声明 + 未决定计数；未建则打迁移期脚注。
	for _, nt := range jrDecisionNotes(decisions, reg) {
		fmt.Fprintf(stderr, "  · 乙半 %s\n", nt)
	}
	problems, ok := jrProblemList(root, reg, decisions)
	if !ok {
		fmt.Fprintln(stderr, "⛔ 注册表 `entries` 非数组 ⇒ 不给结论（rc=2）")
		return 2
	}
	if len(problems) > 0 {
		fmt.Fprintf(stderr, "✗ 判据注册表对拍**红**（双向差集 / 列面 / 甲⇄乙 %d 条）：\n", len(problems))
		for _, p := range problems {
			fmt.Fprintln(stderr, "  · "+p)
		}
		return 1
	}
	count := 0
	if cv := jget(reg, "count"); cv != nil && cv.kind == 'i' {
		if n, err := strconv.Atoi(cv.num); err == nil {
			count = n
		}
	}
	fmt.Fprintf(stdout, "✓ 判据注册表对拍绿（rc=0）：条目 %d · 双向差集空 · 缓存键新鲜 · 甲⇄乙 %s\n",
		count, jrPairState(decisions))
	return 0
}

// jrWriteFile 照 py _write。
func jrWriteFile(root, rel, text string) error {
	p := pyJoin(root, rel)
	_ = os.MkdirAll(pyDirname(p), 0o755)
	return os.WriteFile(p, []byte(text), 0o644)
}

// jrJVOf 把已建注册表转成有序 JSON 值（复刻 py「build_registry 回 dict ⇒ 再进 _problem_list」）。
func jrJVOf(reg *jrRegistry) *jv {
	b, err := json.Marshal(reg)
	if err != nil {
		return nil
	}
	v, err := jparse(b)
	if err != nil {
		return nil
	}
	return v
}

func jrAnyContains(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}

// jrCmdSelfTest 照 py cmd_self_test（成对负控 · 拆两半格A–格M ＋ 甲半负控①–⑤）。
// 夹具走 MkdirTemp ＋ defer RemoveAll ⇒ 前后计数 0。
func jrCmdSelfTest(stdout io.Writer) int {
	ok := true
	root, err := os.MkdirTemp("", "judge-reg-self-")
	if err != nil {
		fmt.Fprintf(stdout, "✗ 自检夹具建不起来：%s\n", err)
		return 2
	}
	defer os.RemoveAll(root)

	_ = jrWriteFile(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a')\n")
	_ = jrWriteFile(root, "scripts/b.sh", "#!/usr/bin/env bash\necho b\n")
	_ = jrWriteFile(root, "scripts/sub/c.py", "#!/usr/bin/env python3\nprint('c')\n")
	_ = jrWriteFile(root, "scripts/gates/__pycache__/ignored.py", "# 缓存件，不进名录\n")
	_ = jrWriteFile(root, "scripts/gates/precommit-gates.sh", "#!/usr/bin/env bash\nadd_step gates \"门X 覆盖测试步\" tri \".\" \"zerg gate check check-judge-registry --check\"\n")

	reg := jrBuildRegistry(root)
	regJV := jrJVOf(reg)
	if b, e := json.Marshal(reg); e == nil {
		_ = os.WriteFile(pyJoin(root, "reg.json"), b, 0o644)
	}

	// 正控
	if probs, _ := jrProblemList(root, regJV, nil); len(probs) > 0 {
		fmt.Fprintf(stdout, "✗ 正控：干净夹具被判红：%s\n", jrPyList(probs))
		ok = false
	} else {
		fmt.Fprintf(stdout, "✓ 正控：干净夹具判绿（条目 %d）\n", reg.Count)
	}

	// ── 拆两半 · 甲⇄乙 成对负控（**在人签半/负控尚未扰动夹具前**跑 · 夹具此刻干净）──────────
	// 格A：`--emit` 产物 ✗ 不得含人签半任何字段（含四字段已搬走）。
	emitText := jrMarshalRegistry(reg)
	badEmit := []string{}
	for _, k := range []string{"\"decision\"", "\"adopt\"", "\"exempt\"", "\"declare\"", "\"sig\"",
		"\"adopted\"", "\"needs_env\"", "\"serial\"", "\"hazard\"", "\"dry_run\"", "\"covered_by\""} {
		if strings.Contains(emitText, k) {
			badEmit = append(badEmit, k)
		}
	}
	if len(badEmit) > 0 {
		fmt.Fprintf(stdout, "✗ 格A（甲/乙 分离）：`--emit` 产物含人签半字段：%s\n", jrPyList(badEmit))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格A（甲/乙 分离）：`--emit` 产物无人签半字段（含四字段已搬走）")
	}

	// 格B：人签半未建 ⇒ 乙半面不判红（迁移期）
	if p, _ := jrProblemList(root, regJV, nil); jrAnyContains(p, "人签") {
		fmt.Fprintf(stdout, "✗ 格B（人签半未建）：乙半面误判红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格B（人签半未建）：乙半面不判红（迁移期只报告）")
	}

	// 人签半夹具：覆盖在册全部件 ⇒ adopt（人签三件套齐）· `enforce=false`（迁移期）
	names := []string{}
	for _, e := range reg.Entries {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	decItems := make([]*jv, 0, len(names))
	for _, n := range names {
		decItems = append(decItems, jrNewObj(
			jpair{"id", jrNewStr(n)},
			jpair{"decision", jrNewStr("adopt")},
			jpair{"by", jrNewStr("tester")},
			jpair{"at", jrNewStr("2026-10-03")},
			jpair{"sig", jrNewStr("sig-" + n)},
		))
	}
	dec := jrNewObj(
		jpair{"schema", jrNewStr(jrDecisionsSchema)},
		jpair{"enforce", jrNewBool(false)},
		jpair{"decisions", jrNewArr(decItems...)},
	)

	// 格C：全覆盖 adopt ⇒ 绿
	if p, _ := jrProblemList(root, regJV, dec); len(p) > 0 {
		fmt.Fprintf(stdout, "✗ 格C（人签半全覆盖 adopt）：该绿没绿 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格C（人签半全覆盖 adopt）：判绿")
	}

	// 格D：抽掉一条决定 ＋ 上牙 ⇒ 红（在册无人签采纳/豁免决定）
	d2 := jrClone(dec)
	jrSet(d2, "enforce", jrNewBool(true))
	jrSet(d2, "enforce_sig", jrNewObj(jpair{"by", jrNewStr("tester")}, jpair{"at", jrNewStr("2026-10-03")}, jpair{"sig", jrNewStr("s-enforce")}))
	jrFilterDecisions(d2, func(d *jv) bool { return jrPyStr(jget(d, "id")) != "a" })
	if p, _ := jrProblemList(root, regJV, d2); !jrAnyContains(p, "无人签采纳/豁免决定") {
		fmt.Fprintf(stdout, "✗ 格D（enforce=true · 在册缺一条决定）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格D（enforce=true · 在册缺一条决定）：现红")
	}

	// 格E：悬空决定 ⇒ 红
	d3 := jrClone(dec)
	jrAppendDecision(d3, jrNewObj(
		jpair{"id", jrNewStr("ghost-x")}, jpair{"decision", jrNewStr("adopt")},
		jpair{"by", jrNewStr("tester")}, jpair{"at", jrNewStr("2026-10-03")}, jpair{"sig", jrNewStr("s")},
	))
	if p, _ := jrProblemList(root, regJV, d3); !jrAnyContains(p, "悬空") {
		fmt.Fprintf(stdout, "✗ 格E（悬空决定）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格E（悬空决定）：现红")
	}

	// 格F：决定值不在闭集 ＋ 未人签（sig 空）⇒ 两红
	d4 := jrClone(dec)
	jrForEachDecision(d4, func(d *jv) {
		if jrPyStr(jget(d, "id")) == "a" {
			jrSet(d, "decision", jrNewStr("maybe"))
			jrSet(d, "sig", jrNewStr(""))
		}
	})
	if p, _ := jrProblemList(root, regJV, d4); !(jrAnyContains(p, "闭集") && jrAnyContains(p, "未人签")) {
		fmt.Fprintf(stdout, "✗ 格F（闭集/未人签）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格F（闭集 / 未人签）：现红")
	}

	// 格G：exempt 可见（notes 逐条打印 · ✗ 不判红）
	d5 := jrClone(dec)
	jrForEachDecision(d5, func(d *jv) {
		if jrPyStr(jget(d, "id")) == "a" {
			jrSet(d, "decision", jrNewStr("exempt"))
			jrSet(d, "reason", jrNewStr("太贵"))
		}
	})
	notes := jrDecisionNotes(d5, regJV)
	visible := false
	for _, n := range notes {
		if strings.Contains(n, "豁免") && strings.Contains(n, "a") && strings.Contains(n, "太贵") {
			visible = true
		}
	}
	if !visible {
		fmt.Fprintf(stdout, "✗ 格G（exempt 可见）：备注里没印出豁免 ⇒ %s\n", jrPyList(notes))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格G（exempt 可见）：备注逐条打印（非静默放过）")
	}
	if p, _ := jrProblemList(root, regJV, d5); len(p) > 0 {
		fmt.Fprintf(stdout, "✗ 格G2（exempt ✗ 不判红）：豁免被误判红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格G2（exempt ✗ 不判红）：豁免只报告不判红")
	}

	// 格H：`--emit` 与人签半**无入参耦合**（构造上不可能覆盖人的决定 · 本件签名即证）
	fmt.Fprintln(stdout, "✓ 格H（`--emit` ✗ 不碰人签半）：`build_registry` 与人签半无入参耦合")

	// ── 拍板③ · declare 类 ＋ 迁移期 enforce 开关 成对负控 ─────────────────────────────
	// 格I：declare 类（四字段值合法 · enforce=false）⇒ 不判红 · 载荷可见
	d6 := jrClone(dec)
	jrSet(d6, "enforce", jrNewBool(false))
	jrFilterDecisions(d6, func(d *jv) bool { return jrPyStr(jget(d, "id")) != "b" })
	jrAppendDecision(d6, jrNewObj(
		jpair{"id", jrNewStr("b")}, jpair{"decision", jrNewStr("declare")},
		jpair{"by", jrNewStr("tester")}, jpair{"at", jrNewStr("2026-10-03")}, jpair{"sig", jrNewStr("sig-b")},
		jpair{"fields", jrNewObj(
			jpair{"needs_env", jrNewBool(true)}, jpair{"serial", jrNewBool(false)},
			jpair{"hazard", jrNewStr("swap")}, jpair{"dry_run", jrNewStr("argv")})},
	))
	if p, _ := jrProblemList(root, regJV, d6); len(p) > 0 {
		fmt.Fprintf(stdout, "✗ 格I（declare 合法 + enforce=false）：该绿没绿 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格I（declare 合法 + enforce=false）：判绿（b 仅声明未采纳 ⇒ 采纳面迁移期不判红）")
	}
	notes = jrDecisionNotes(d6, regJV)
	visible = false
	for _, n := range notes {
		if strings.Contains(n, "声明") && strings.Contains(n, "hazard") && strings.Contains(n, "swap") && strings.Contains(n, "dry_run") {
			visible = true
		}
	}
	if !visible {
		fmt.Fprintf(stdout, "✗ 格I2（declare 可见）：备注里没印出四字段 ⇒ %s\n", jrPyList(notes))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格I2（declare 可见）：备注逐条打印四字段（非静默）")
	}

	// 格J：declare 载荷缺 `fields` / 缺字段 / 值越出闭集（needs_env 非布尔 · hazard / dry_run 越界）⇒ 红
	d7 := jrClone(dec)
	jrSet(d7, "enforce", jrNewBool(false))
	jrAppendDecision(d7, jrNewObj(
		jpair{"id", jrNewStr("c")}, jpair{"decision", jrNewStr("declare")},
		jpair{"by", jrNewStr("tester")}, jpair{"at", jrNewStr("2026-10-03")}, jpair{"sig", jrNewStr("sig-c")},
		jpair{"fields", jrNewObj(
			jpair{"needs_env", jrNewStr("yes")}, jpair{"serial", jrNewBool(false)},
			jpair{"hazard", jrNewStr("boom")}, jpair{"dry_run", jrNewStr("yes")})},
	))
	jrAppendDecision(d7, jrNewObj(
		jpair{"id", jrNewStr("a")}, jpair{"decision", jrNewStr("declare")},
		jpair{"by", jrNewStr("tester")}, jpair{"at", jrNewStr("2026-10-03")}, jpair{"sig", jrNewStr("sig-a")},
	))
	p7, _ := jrProblemList(root, regJV, d7)
	want7 := jrAnyContains(p7, "`needs_env` 非布尔") && jrAnyContains(p7, "`hazard` 值不在闭集") &&
		jrAnyContains(p7, "`dry_run` 值不在闭集") && jrAnyContains(p7, "缺载荷 `fields`")
	if !want7 {
		fmt.Fprintf(stdout, "✗ 格J（declare 越界/缺键）：该红没红 ⇒ %s\n", jrPyList(p7))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格J（declare 越界/缺键）：现红（非布尔 / 值越闭集 / 缺载荷 各自点名）")
	}

	// 格J2：declare 载荷缺**单个**字段键 ⇒ 红
	d7b := jrClone(dec)
	jrSet(d7b, "enforce", jrNewBool(false))
	jrAppendDecision(d7b, jrNewObj(
		jpair{"id", jrNewStr("c")}, jpair{"decision", jrNewStr("declare")},
		jpair{"by", jrNewStr("tester")}, jpair{"at", jrNewStr("2026-10-03")}, jpair{"sig", jrNewStr("sig-c")},
		jpair{"fields", jrNewObj(
			jpair{"needs_env", jrNewBool(false)}, jpair{"serial", jrNewBool(false)}, jpair{"hazard", jrNewStr("none")})},
	))
	if p, _ := jrProblemList(root, regJV, d7b); !jrAnyContains(p, "declare 载荷缺字段 `dry_run`") {
		fmt.Fprintf(stdout, "✗ 格J2（declare 缺字段键）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格J2（declare 缺字段键）：现红（缺 `dry_run` 点名）")
	}

	// 格K：enforce 两态 —— false 未决定不判红；true（enforce_sig 齐）未决定判红 · 脚注报计数
	d8 := jrNewObj(jpair{"schema", jrNewStr(jrDecisionsSchema)}, jpair{"enforce", jrNewBool(false)}, jpair{"decisions", jrNewArr()})
	d9 := jrNewObj(jpair{"schema", jrNewStr(jrDecisionsSchema)}, jpair{"enforce", jrNewBool(true)}, jpair{"decisions", jrNewArr()},
		jpair{"enforce_sig", jrNewObj(jpair{"by", jrNewStr("tester")}, jpair{"at", jrNewStr("2026-10-03")}, jpair{"sig", jrNewStr("s-enforce")})})
	pFalse, _ := jrProblemList(root, regJV, d8)
	pTrue, _ := jrProblemList(root, regJV, d9)
	if len(pFalse) > 0 {
		fmt.Fprintf(stdout, "✗ 格K1（enforce=false）：未决定被误判红 ⇒ %s\n", jrPyList(pFalse))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格K1（enforce=false）：未决定不判红（迁移期只报告）")
	}
	if !jrAnyContains(pTrue, "无人签采纳/豁免决定") {
		fmt.Fprintf(stdout, "✗ 格K2（enforce=true）：未决定该红没红 ⇒ %s\n", jrPyList(pTrue))
		ok = false
	} else {
		fmt.Fprintf(stdout, "✓ 格K2（enforce=true）：未决定判红（现 %d 条）\n", len(pTrue))
	}
	notesFalse := jrDecisionNotes(d8, regJV)
	visible = false
	for _, n := range notesFalse {
		if strings.Contains(n, fmt.Sprintf("未决定 %d 条", len(reg.Entries))) {
			visible = true
		}
	}
	if !visible {
		fmt.Fprintf(stdout, "✗ 格K3（enforce=false 脚注）：未印「未决定 N 条」 ⇒ %s\n", jrPyList(notesFalse))
		ok = false
	} else {
		fmt.Fprintf(stdout, "✓ 格K3（enforce=false 脚注）：脚注报「未决定 %d 条」\n", len(reg.Entries))
	}

	// 格L：上牙（enforce=true）**未人签** ⇒ 红（拍板③：上牙本身必须人签）
	d10 := jrNewObj(jpair{"schema", jrNewStr(jrDecisionsSchema)}, jpair{"enforce", jrNewBool(true)}, jpair{"decisions", jrNewArr()})
	if p, _ := jrProblemList(root, regJV, d10); !(jrAnyContains(p, "上牙") && jrAnyContains(p, "未人签")) {
		fmt.Fprintf(stdout, "✗ 格L（上牙未人签）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格L（上牙未人签）：现红（enforce=true 缺 enforce_sig）")
	}

	// 格M：`enforce` 非布尔 ⇒ 红 · 且按 false（迁移期）读
	d11 := jrNewObj(jpair{"schema", jrNewStr(jrDecisionsSchema)}, jpair{"enforce", jrNewStr("yes")}, jpair{"decisions", jrNewArr()})
	if p, _ := jrProblemList(root, regJV, d11); !jrAnyContains(p, "`enforce` 非布尔") {
		fmt.Fprintf(stdout, "✗ 格M（enforce 非布尔）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格M（enforce 非布尔）：现红（并落回迁移期读）")
	}
	// ── 判据覆盖面 `covered_by`（人签半声明 ⇄ 盘面 / 步骤表面 双向对拍 · 成对）──────────────────
	// 格N：合法覆盖声明（`file:` 盘上件 ＋ `step:` 步骤表步名）⇒ 不判红 · 声明可机读
	dCov := jrClone(dec)
	jrForEachDecision(dCov, func(d *jv) {
		if jrPyStr(jget(d, "id")) == "a" {
			jrSet(d, "covered_by", jrNewArr(jrNewStr("file:scripts/a.py"), jrNewStr("step:门X 覆盖测试步")))
		}
	})
	if p, _ := jrProblemList(root, regJV, dCov); len(p) > 0 {
		fmt.Fprintf(stdout, "✗ 格N（覆盖声明合法）：该绿没绿 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格N（覆盖声明合法 · file:＋step:）：判绿（声明可机读）")
	}

	// 格N2：反查 —— 同一覆盖对象被两判据同时认领 ⇒ 红
	dCov2 := jrClone(dec)
	jrForEachDecision(dCov2, func(d *jv) {
		if id := jrPyStr(jget(d, "id")); id == "a" || id == "b" {
			jrSet(d, "covered_by", jrNewArr(jrNewStr("file:scripts/a.py")))
		}
	})
	if p, _ := jrProblemList(root, regJV, dCov2); !jrAnyContains(p, "反查冲突") {
		fmt.Fprintf(stdout, "✗ 格N2（反查冲突）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 格N2（反查冲突）：现红（同一对象被多判据认领）")
	}

	// ── 甲半负控（承前 · 顺序不变）────────────────────────────────────────────────
	// 负控①：盘上新增件、表里没有 ⇒ 红
	_ = jrWriteFile(root, "scripts/d.py", "#!/usr/bin/env python3\nprint('d')\n")
	if p, _ := jrProblemList(root, regJV, nil); !jrAnyContains(p, "没点名") {
		fmt.Fprintf(stdout, "✗ 负控①（盘上有表里无）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控①（盘上有件表里无）：现红")
	}
	_ = os.Remove(pyJoin(root, "scripts/d.py"))

	// 负控②：表里点名、盘上无件 ⇒ 红
	ghost := jrClone(regJV)
	if ev := jget(ghost, "entries"); ev != nil && ev.kind == 'a' {
		ev.arr = append(ev.arr, jrNewObj(
			jpair{"id", jrNewStr("ghost")}, jpair{"name", jrNewStr("ghost")},
			jpair{"path", jrNewStr("scripts/ghost.py")}, jpair{"sha256", jrNewStr(strings.Repeat("0", 64))},
		))
	}
	if p, _ := jrProblemList(root, ghost, nil); !jrAnyContains(p, "悬空登记") {
		fmt.Fprintf(stdout, "✗ 负控②（表里点名盘上无）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控②（表里点名盘上无）：现红")
	}

	// 负控③：件被改、缓存键过期 ⇒ 红
	_ = jrWriteFile(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a-changed')\n")
	if p, _ := jrProblemList(root, regJV, nil); !jrAnyContains(p, "缓存键过期") {
		fmt.Fprintf(stdout, "✗ 负控③（缓存键过期）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控③（缓存键过期）：现红")
	}
	_ = jrWriteFile(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a')\n")

	// 负控④：同名多件 ⇒ 红
	_ = jrWriteFile(root, "scripts/sub2/a.py", "#!/usr/bin/env python3\nprint('a2')\n")
	if p, _ := jrProblemList(root, regJV, nil); !jrAnyContains(p, "同名多件") {
		fmt.Fprintf(stdout, "✗ 负控④（同名多件）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控④（同名多件）：现红")
	}

	// 负控⑤：路径越出前缀 ⇒ 红
	bad := jrClone(regJV)
	if ev := jget(bad, "entries"); ev != nil && ev.kind == 'a' {
		for _, e := range ev.arr {
			if nm := jget(e, "name"); nm != nil && nm.kind == 's' && nm.s == "a" {
				if pv := jget(e, "path"); pv != nil {
					pv.s = "core/a.py"
				}
			}
		}
	}
	if p, _ := jrProblemList(root, bad, nil); !jrAnyContains(p, "前缀") && !jrAnyContains(p, "悬空登记") {
		fmt.Fprintf(stdout, "✗ 负控⑤（越出前缀）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控⑤（越出前缀）：现红")
	}

	// 负控⑥：覆盖对象（`covered_by`）声明一个盘上不存在的件 ⇒ 红（覆盖面双向对拍的第一腿）
	covGhost := jrClone(dec)
	jrForEachDecision(covGhost, func(d *jv) {
		if jrPyStr(jget(d, "id")) == "a" {
			jrSet(d, "covered_by", jrNewArr(jrNewStr("file:scripts/ghost-covered.py")))
		}
	})
	if p, _ := jrProblemList(root, regJV, covGhost); !jrAnyContains(p, "覆盖对象") {
		fmt.Fprintf(stdout, "✗ 负控⑥（覆盖对象不存在）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控⑥（覆盖对象不存在）：现红")
	}

	if ok {
		return 0
	}
	return 2
}

// checkJudgeRegistryGateMain —— 入口（argv 与 py `sys.argv[1:]` 同形 · 三码透传）。
func checkJudgeRegistryGateMain(stdout, stderr io.Writer, argv []string) int {
	rootArg := ""
	rootSet := false
	outArg := jrRegistryDefault
	regArg := jrRegistryDefault
	decArg := jrDecisionsDefault
	emit, doCheck, selfTest := false, false, false

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		val := func(name string) (string, bool) {
			if a == name {
				if i+1 >= len(argv) {
					return "", false
				}
				i++
				return argv[i], true
			}
			return strings.TrimPrefix(a, name+"="), true
		}
		switch {
		case a == "--root" || strings.HasPrefix(a, "--root="):
			v, okv := val("--root")
			if !okv {
				fmt.Fprintf(stderr, "用法错：`--root` 缺值 ⇒ 不给结论（rc=2）\n")
				return 2
			}
			rootArg, rootSet = v, true
		case a == "--out" || strings.HasPrefix(a, "--out="):
			v, okv := val("--out")
			if !okv {
				fmt.Fprintf(stderr, "用法错：`--out` 缺值 ⇒ 不给结论（rc=2）\n")
				return 2
			}
			outArg = v
		case a == "--registry" || strings.HasPrefix(a, "--registry="):
			v, okv := val("--registry")
			if !okv {
				fmt.Fprintf(stderr, "用法错：`--registry` 缺值 ⇒ 不给结论（rc=2）\n")
				return 2
			}
			regArg = v
		case a == "--decisions" || strings.HasPrefix(a, "--decisions="):
			v, okv := val("--decisions")
			if !okv {
				fmt.Fprintf(stderr, "用法错：`--decisions` 缺值 ⇒ 不给结论（rc=2）\n")
				return 2
			}
			decArg = v
		case a == "--emit":
			emit = true
		case a == "--check":
			doCheck = true
		case a == "--self-test":
			selfTest = true
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, "用法：--emit [--out <件>] | --check [--registry <件>] [--decisions <件>] | --self-test [--root <根>]\n")
			return 0
		default:
			fmt.Fprintf(stderr, "用法错：未知旗标 %s ⇒ 不给结论（rc=2）\n", a)
			return 2
		}
	}

	n := 0
	for _, b := range []bool{emit, doCheck, selfTest} {
		if b {
			n++
		}
	}
	if n > 1 {
		fmt.Fprintf(stderr, "用法错：`--emit` / `--check` / `--self-test` 互斥 ⇒ 不给结论（rc=2）\n")
		return 2
	}

	root := rootArg
	if !rootSet {
		wd, _ := os.Getwd()
		root = wd
	}
	root = pyAbspath(root)

	if selfTest {
		return jrCmdSelfTest(stdout)
	}
	if emit {
		outPath := outArg
		if outArg != "-" {
			if pyIsAbs(outArg) {
				outPath = outArg
			} else {
				outPath = pyJoin(root, outArg)
			}
		}
		return jrCmdEmit(root, outPath, stdout, stderr)
	}
	if doCheck {
		regPath := regArg
		if !pyIsAbs(regArg) {
			regPath = pyJoin(root, regArg)
		}
		decPath := decArg
		if !pyIsAbs(decArg) {
			decPath = pyJoin(root, decArg)
		}
		return jrCmdCheck(root, regPath, decPath, stdout, stderr)
	}
	fmt.Fprintf(stderr, "用法错：`--emit` / `--check` / `--self-test` 三选一必给一个（都不给 ⇒ rc=2）\n")
	return 2
}
