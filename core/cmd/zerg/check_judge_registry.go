// check_judge_registry.go —— 门禁件 `scripts/gates/check-judge-registry.py` 的 **Go 真搬**（M2 · 甲案 C 类第 12 枚）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案把判据实现面分两段走 —— **M1** 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面
//	（转发壳）；**M2** 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。
//	本件 = 甲案 C 类（判据注册表真源生成 + 双向差集自检）的一枚：
//	验收判据 = 对同一件跑两态（py 原件 ↔ 本件入口），**stdout / stderr / 退码三面逐字相同**。
//
// 本件做什么（照 py 真身逐条读出）：
//
//	`--emit`  —— 现读生成注册表：递归 `<根>/scripts/` 全子树，只收 `.py` / `.sh`
//	              （目录名 `__pycache__` 不下钻），判据名 = 文件名去扩展名；每条落 12 个列面字段
//	              （id/name/path/cwd/interp/needs_env/serial/hazard/dry_run/sha256/tool_version/cache_key），
//	              另有 schema/generated_by/roster_dir/extensions/min_tool_version/invocation/count/dupes/entries 九个顶层键。
//	`--check` —— 双向差集（盘上有件表里没点名 ⇒ 红；表里点名盘上无件 ⇒ 红）＋ 四条列面机检
//	              （路径越出 `scripts/` 前缀 / 判据名 ≠ 去扩展名 / 表内路径与盘上不符 /
//	              sha256 缓存键过期）⇒ 空 = 绿。
//	`--self-test` —— 成对负控：合成夹具上正控绿 ＋ 五条负控各自必红（① 盘上有件表里无 ·
//	              ② 表里点名盘上无 · ③ 缓存键过期 · ④ 同名多件 · ⑤ 路径越出前缀）。
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
//	   取证 = 「落件前后 py 原件 `--emit --out -` 的 stdout sha16 逐字相同」（回执给读数）。
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
// ⚠ 本单边界（照实）：只落码、不接线 —— 不进命令树 · 不改 `add_step` · 不删 py 原件 ·
// 入口暂无调用点（先证明「搬得逐字相同」，接线与命令注册是后一单的事）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 帮助 / 用法 / 错误正文（`-h` / 未知旗标 / 互斥二选冲突 / 缺值 / 前缀缩写）：
//	   py 打 argparse 生成的 usage 正文（含 prog 名、终端宽度）⇒ 本件只对齐**退码 2**，正文自定。
//	② 异常路径正文：注册表**读不到**（权限 / 不存在）py 打 `[Errno 2] …` 串；**非 JSON** py 打
//	   `json.JSONDecodeError` 串 ⇒ 本件同退码（2）· 正文为 Go 的 error 串。
//	③ `entries` **在场但非数组**（null / 数字 / 布尔）时 py 在迭代处抛异常 ⇒ 回溯 rc=1；
//	   本件 fail-closed rc=2 ＋ 具名文案（✗ 不动 py 原件）。
//	④ 非字符串 `name` 的排序、浮点数的 Python repr、以及 `%r` 对**嵌套 dict 转义**的全枚举：
//	   本件对常规标量与数组逐字相同，对上述边角未做全枚举对拍。
//	⑤ 子进程面：`.sh` 件的 `tool_version` 走 `bash --version`（与 py 同形、结果缓存）；`.py` 件的
//	   `tool_version` py 用**运行解释器**的版本 ⇒ 本件改跑 `python3 --version` 取 PATH 上 python3 的
//	   版本 ⇒ 两者仅在「py 用非 PATH 的 python3 跑」时不同。
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
	jrSchema          = "judge-registry/1"
	jrRosterDir       = "scripts"
	jrRegistryDefault = "scripts/gates/judge-registry.json"
	jrGeneratedBy     = "scripts/gates/check-judge-registry.py --emit"
)

var jrInterp = map[string]string{".py": "python3", ".sh": "bash"}

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

type jrEntry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Cwd         string `json:"cwd"`
	Interp      string `json:"interp"`
	NeedsEnv    bool   `json:"needs_env"`
	Serial      bool   `json:"serial"`
	Hazard      string `json:"hazard"`
	DryRun      string `json:"dry_run"`
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

// jrNote 与 py 第 143-144 行的 note 字面逐字相同。
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

// jrPyList 复刻 Python `str(list-of-str)`（供自检负控失败档的 `%s`）。
func jrPyList(xs []string) string {
	parts := make([]string, 0, len(xs))
	for _, x := range xs {
		parts = append(parts, jrPyStrRepr(x))
	}
	return "[" + strings.Join(parts, ", ") + "]"
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
			NeedsEnv: false, Serial: false, Hazard: "none", DryRun: "none",
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

// jrProblemList 照 py _problem_list：双向差集 ＋ 列面机检 ⇒ 问题清单（空 = 绿）。
// 第二返回值 = 是否给得出结论（false ⇒ 调用方 rc=2 fail-closed · 见件头未覆盖③）。
func jrProblemList(root string, reg *jv) ([]string, bool) {
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
	return problems, true
}

// jrLoadRegistry 照 py load_registry：读文件 → 有序 JSON → schema 校验。
// 第三返回 = 校验失败文案（空 = 通过）。
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

func jrSchemaOf(reg *jv) string {
	if sv := jget(reg, "schema"); sv != nil && sv.kind == 's' {
		return sv.s
	}
	return ""
}

// jrCmdEmit 照 py cmd_emit。
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
		fmt.Fprintf(stdout, "判据注册表已生成：%s（条目 %d · 同名歧义 %d）\n", out, reg.Count, len(reg.Dupes))
	}
	if len(reg.Dupes) > 0 {
		fmt.Fprintf(stderr, "⚠ 盘上有同名多件（%d）：%s\n", len(reg.Dupes), strings.Join(reg.Dupes, " · "))
	}
	return 0
}

// jrCmdCheck 照 py cmd_check。
func jrCmdCheck(root, regPath string, stdout, stderr io.Writer) int {
	reg, errMsg := jrLoadRegistry(regPath)
	if errMsg != "" {
		fmt.Fprintln(stderr, errMsg)
		return 2
	}
	problems, ok := jrProblemList(root, reg)
	if !ok {
		fmt.Fprintln(stderr, "⛔ 注册表 `entries` 非数组 ⇒ 不给结论（rc=2）")
		return 2
	}
	if len(problems) > 0 {
		fmt.Fprintf(stderr, "✗ 判据注册表对拍**红**（双向差集/列面 %d 条）：\n", len(problems))
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
	fmt.Fprintf(stdout, "✓ 判据注册表对拍绿（rc=0）：条目 %d · 双向差集空 · 缓存键新鲜\n", count)
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

// jrCmdSelfTest 照 py cmd_self_test（成对负控）。夹具走 MkdirTemp ＋ defer RemoveAll ⇒ 前后计数 0。
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

	reg := jrBuildRegistry(root)
	if b, err := json.Marshal(reg); err == nil {
		_ = os.WriteFile(pyJoin(root, "reg.json"), b, 0o644)
	}

	// 正控
	if probs, _ := jrProblemList(root, jrJVOf(reg)); len(probs) > 0 {
		fmt.Fprintf(stdout, "✗ 正控：干净夹具被判红：%s\n", jrPyList(probs))
		ok = false
	} else {
		fmt.Fprintf(stdout, "✓ 正控：干净夹具判绿（条目 %d）\n", reg.Count)
	}

	// 负控①：盘上新增件、表里没有 ⇒ 红
	_ = jrWriteFile(root, "scripts/d.py", "#!/usr/bin/env python3\nprint('d')\n")
	if p, _ := jrProblemList(root, jrJVOf(reg)); !jrAnyContains(p, "没点名") {
		fmt.Fprintf(stdout, "✗ 负控①（盘上有表里无）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控①（盘上有件表里无）：现红")
	}
	_ = os.Remove(pyJoin(root, "scripts/d.py"))

	// 负控②：表里点名、盘上无件 ⇒ 红
	ghost := jrJVOf(reg)
	if ev := jget(ghost, "entries"); ev != nil && ev.kind == 'a' {
		ev.arr = append(ev.arr, &jv{kind: 'o', obj: []jpair{
			{"id", &jv{kind: 's', s: "ghost"}},
			{"name", &jv{kind: 's', s: "ghost"}},
			{"path", &jv{kind: 's', s: "scripts/ghost.py"}},
			{"sha256", &jv{kind: 's', s: strings.Repeat("0", 64)}},
		}})
	}
	if p, _ := jrProblemList(root, ghost); !jrAnyContains(p, "悬空登记") {
		fmt.Fprintf(stdout, "✗ 负控②（表里点名盘上无）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控②（表里点名盘上无）：现红")
	}

	// 负控③：件被改、缓存键过期 ⇒ 红
	_ = jrWriteFile(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a-changed')\n")
	if p, _ := jrProblemList(root, jrJVOf(reg)); !jrAnyContains(p, "缓存键过期") {
		fmt.Fprintf(stdout, "✗ 负控③（缓存键过期）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控③（缓存键过期）：现红")
	}
	_ = jrWriteFile(root, "scripts/a.py", "#!/usr/bin/env python3\nprint('a')\n")

	// 负控④：同名多件 ⇒ 红
	_ = jrWriteFile(root, "scripts/sub2/a.py", "#!/usr/bin/env python3\nprint('a2')\n")
	if p, _ := jrProblemList(root, jrJVOf(reg)); !jrAnyContains(p, "同名多件") {
		fmt.Fprintf(stdout, "✗ 负控④（同名多件）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控④（同名多件）：现红")
	}

	// 负控⑤：路径越出前缀 ⇒ 红
	bad := jrJVOf(reg)
	if ev := jget(bad, "entries"); ev != nil && ev.kind == 'a' {
		for _, e := range ev.arr {
			if nm := jget(e, "name"); nm != nil && nm.kind == 's' && nm.s == "a" {
				if pv := jget(e, "path"); pv != nil {
					pv.s = "core/a.py"
				}
			}
		}
	}
	if p, _ := jrProblemList(root, bad); !jrAnyContains(p, "前缀") && !jrAnyContains(p, "悬空登记") {
		fmt.Fprintf(stdout, "✗ 负控⑤（越出前缀）：该红没红 ⇒ %s\n", jrPyList(p))
		ok = false
	} else {
		fmt.Fprintln(stdout, "✓ 负控⑤（越出前缀）：现红")
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
		case a == "--emit":
			emit = true
		case a == "--check":
			doCheck = true
		case a == "--self-test":
			selfTest = true
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, "用法：--emit [--out <件>] | --check [--registry <件>] | --self-test [--root <根>]\n")
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
		return jrCmdCheck(root, regPath, stdout, stderr)
	}
	fmt.Fprintf(stderr, "用法错：`--emit` / `--check` / `--self-test` 三选一必给一个（都不给 ⇒ rc=2）\n")
	return 2
}
