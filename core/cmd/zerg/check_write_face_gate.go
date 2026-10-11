// check_write_face_gate.go —— 判据件 `scripts/gates/check-write-face-gate.py`（481 行）的
// **Go 真搬**（M2 · 搬运面 · 入口 `checkWriteFaceGateGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 core/cmd/zerg/）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的 argparse
//	面差异、py 解释器版本面、与未覆盖面）。
//
// 选它的理由（本单口径：真未搬清单里**纯静态只读**一批）：
//
//	✓ 选：判据只读 `core/cmd/zerg/*.go`（排除 `_test.go`）＋ `main.go` 的 catalog 块 ＋
//	  `core/internal/contract/authority-map.json` 的「写面登记」表，逐件静态抽取「声明面（usage /
//	  danger）↔ 实现面（真含写调用的函数）」，判「写面是否声明授权旗标 / 是否进白名单」。**不真跑
//	  子进程、不碰网络、零计时、默认档不读时刻不写盘**（`--self-test` 只在 `os.MkdirTemp` 造的临时
//	  目录里造件 —— `defer os.RemoveAll` 回收，**非** A 类泄漏形态）。
//	· 外部树依赖：`--root`（env `ZERG_ROOT`）可显式指到任意合成假仓根（旋钮名照 py 同名旗标 `--root`）。
//	· 退码 0/1/2 三档齐 · 纯只读无写面（默认档）· 落件前 `git status` 现读不含它。
//	· 现读 import 面：argparse / json / os / re / sys ＋（跨件 importlib 加载）`check-write-authz.py`
//	  —— **零子进程 / time / datetime / random / socket / urllib / shutil / glob**（tempfile 只在
//	  self_test 内 import）。
//
// 跨件复用（★ 与 py 同构，✗ 非「第二套解析」）：
//
//	py 本件在 import 期用 `importlib` 从同目录 `check-write-authz.py` 加载模块（`_cwa`），实现面
//	**只调它的 `scan_file`**（写点 → 所属函数）。Go 侧照此**复用同包已落件的 `cwaScanFile`**
//	（`check_write_authz.go`）—— 单一定义、无第二实现、无第二条真源。py 的 `_cwa is None` 兜底支
//	（加载失败 ⇒ 实现面空集 ⇒ 全部写面判不出）在 Go 里**不可达**（编译期链接），列入未覆盖面。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 main.go 里注册命令（✗ 不进命令树 · ✗ 不改 gate_inproc_table.go）；
//	· **不**删 `scripts/gates/check-write-face-gate.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `checkWriteFaceGateGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面 · ✗ 不改任何 family_*.go / 已落旧件。
//
// 入口签名（与 py 的 main(argv) 对齐）：
//
//	func checkWriteFaceGateGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py sys.argv[1:] 同形（不含程序名；inproc 口径不吃 argv[0]）
//	  · 退码  —— 0 全有交代 / 1 有未声明·未白名单·假条目·判不了（逐条点名） / 2 不给结论
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「未覆盖面」）
//
// ═══════════════════════ ★ 遍历序与「键序」（本件有目录遍历 ＋ 正则块 ＋ 有序聚合）═══════════════════════
//
//	① 目录遍历：py `sorted(os.listdir(scopedir))`（两处：实现面、常量面）—— **显式排序**（按码点；
//	  UTF-8 下 == Go 字节序）⇒ Go 侧用 sort.Strings 复现。★ 输出行序的决定者 = catalog 块序（见②）
//	  与这份 sorted(...)（`写面函数` 计数为该排序下逐件收「含写调用的顶层函数」）。
//	② catalog 块：py `re.findall(r"\{(?:[^{}]|\{[^{}]*\})*?\}", text)` = 左最先·非重叠·非贪婪 ⇒
//	  Go RE2 同为左最先·非贪婪（`*?` 受支持）⇒ 逐块同构；块内再 `re.search(...)` 取首个。
//	③ `registered = sorted(valid.items(), key=lambda kv: (kv[1], kv[0]))` = 先 `授权类` 后 `命令`
//	  ⇒ Go sort.Slice(先 cls 后 cmd)；`by_cls` 按 registered 插入序聚合（切片段保序，✗ 零 map 迭代进
//	  输出）；`sorted(by_cls.items())` 只用于那行「%s %d」摘要（按 cls 码点序）⇒ Go 单列排序副本。
//	④ ★ 无 Go map 迭代进输出：valid/wfuncs/waived/undecidedCmds 只做布尔「含否」判；hits / fake /
//	  undecided / registered / by_cls 全走切片保 py 追加序；by_cls 的 `无闸` 键序由③决定。
//
// ═══════════════════════ ★ py 字符串原语（逐处现读）═══════════════════════
//
//	① `open(...).read()` = universal-newlines（`\r\n`/`\r`→`\n`）⇒ 复用 cwaReadText。
//	② `str(v).strip()` 对登记面字段 = **py str.strip（含 `\x1c-\x1f`）** ⇒ 复用 cwaPyStrip。
//	③ `json.dumps(ensure_ascii=False)` 默认分隔符 `", "` / `": "` ⇒ 本件手搓 JSON（✗ 不用 json.Marshal）、
//	  字符串转义走 cwaJSONStr（`"` `\` `\b\f\n\r\t` 与 <0x20 的 `\u00xx`，小写十六进制）。
//	④ `os.path.join(root, "core", "internal", "contract", "authority-map.json")` ⇒ 复用 cwaPyJoin。
//
// ═══════════════════════ ★ 声明 (a) 默认根定位差异（本件逐字同构）═══════════════════════
//
//	py `--root` 缺省 = `os.environ.get("ZERG_ROOT", os.getcwd())` ⇒ **起于 cwd**（不是 __file__ 上溯）。
//	Go 无 __file__ 语境，但本件 py 本就用 cwd ⇒ 走 os.LookupEnv + os.Getwd 逐字同构。
//
// ═══════════════════════ ★ 声明 (b) 自指面（本件落在扫描面内 ⇒ 逐条消解）═══════════════════════
//
//	① 本判据扫 `core/cmd/zerg/*.go`（非测试）⇒ **本件自身落在扫描面内**。消解：
//	  · 写点正则走 cwaScanFile（`\bos\.WriteFile\(` 形）⇒ 本件源码**不出现**该写点原语的连续字面
//	    （写出它一律走「前缀 + 字面」拼串：函数名与左括号之间**必有一对引号**断开）。
//	  · 自检夹具的写点字面**全部收进夹具函数**（名含 `Fixture` / `SelfTest` ⇒ 命中 cwa 的「夹具函数」
//	    判据被剔出；且夹具函数体含 `os.MkdirTemp` ⇒ 名字与体两路都命中）。
//	  · 运行档（cwfgRun）只读、零写点字面 ⇒ 本件对真仓「裸写点」计数贡献 0。
//	② 件头注释无真引号路径原串（描述式）。
//
// ═══════════════════════ ★ 未覆盖面（逐条）═══════════════════════
//
//	· argparse `options:` 面按 **py ≥3.10** 硬编码（≤3.9 印 `optional arguments:`）⇒ 环境依赖；本单
//	  对拍基准钉 /opt/homebrew/bin/python3 3.14.5（见件尾）。
//	· py `json.load` 失败正文（`JSONDecodeError` 的 line/column/char）未逐字复刻；Go 用 encoding/json
//	  报错文本 ⇒ 该格（authority-map.json 非法 JSON）**不逐字对齐**，真仓该路不可达。
//	· py 顶层非 dict（如 JSON 数组）时 `data.get` 抛 AttributeError（回溯 rc=1）；Go 反序列化失败
//	  ⇒ 归「解析失败」支。声明为未复刻。
//	· py 登记面字段 `str(v)` 对**非标量**（dict/list）的 repr 未逐字复刻（真仓登记面全为字符串）。
//	· py `\s`/`\w`/`\b` 是 Unicode 语义；Go RE2 为 ASCII ⇒ 仅当被扫文本含非 ASCII 空白/词字符时可能分叉
//	  （真仓与夹具均为 ASCII 空白）。
//	· py 读档 UTF-8 严格模式：非法字节 ⇒ 回溯 rc=1；Go 侧按 replacement 走、跳过该件（真仓无障碍）。
//	· 不可读件：py `open` 无 try ⇒ OSError 回溯 rc=1；✗ Go 侧不再静默跳过 —— B 类读失败 ⇒ 登记 + BLOCKED rc=2 逐件点名（正解体例 = check_doc_cmds.go 的 dcmdReadFails）。
//	· `_cwa is None`（跨件 importlib 加载失败）兜底支在 Go 不可达（编译期链接）⇒ 声明。
//	· 自检档失败分支的 `%r`/`%q` repr 措辞按 py 单引号 repr 未逐字复刻（**PASS 档不打印该支**）。
//	· argparse 短选项聚类（`-hx` 一类）与 `--` 之后的复杂 positional 语义未逐一复刻（声明）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ── 常量（照 py 同名量逐字）────────────────────────────────────────────────
const (
	cwfgProg         = "check-write-face-gate.py"
	cwfgHelpDesc     = "命令面写面两面判（GAP-20260927-90 · authority-map 写面登记接入）"
	cwfgRunHeader    = "命令面写面「声明↔实现」两面判（GAP-20260927-90 · authority-map 写面登记接入）"
	cwfgAuthMapRel   = "core/internal/contract/authority-map.json"
	cwfgBlockedScope = "BLOCKED —— 缺 main.go / 扫描面 ⇒ rc=2 不给结论"
	cwfgBlockedCat0  = "BLOCKED —— catalog() 解析出 0 条（空域）⇒ rc=2 不给结论"
	cwfgRegMissing   = "authority-map.json 缺「写面登记」表（非 list）"
	// cwfgScopeRel —— 扫描面相对仓根（B 类读失败登记面用；与 cwfgRun 的 scopedir 同源）。
	cwfgScopeRel = "core/cmd/zerg"
)

// cwfgReadFails —— B 类读失败登记面（扫描面里读不了的 *.go 件 · 逐件 相对路径 + err 正文）。
//
//	★ 正解体例 = check_doc_cmds.go 的 dcmdReadFails（同族先例）：读失败 ✗ 不静默 ⇒ 登记 + cwfgRun
//	  出口 BLOCKED rc=2 逐件点名 ＋ 不给结论（读不到 ≠ 通过）。★ 自清：包级登记面不许跨调用残留。
var cwfgReadFails []string

// cwfgRegReadFail —— 登记一件读失败（同一件同名同 err 只记一次：实现面与常量面两次扫同一件）。
func cwfgRegReadFail(rel string, err error) {
	msg := fmt.Sprintf("%s（%v）", rel, err)
	for _, ex := range cwfgReadFails {
		if ex == msg {
			return
		}
	}
	cwfgReadFails = append(cwfgReadFails, msg)
}

// cwfgUsageLine —— py argparse 用法行（单行，72 列 < 80 ⇒ 不换行，逐字）。
const cwfgUsageLine = "usage: check-write-face-gate.py [-h] [--root ROOT] [--json] [--self-test]\n"

// cwfgHelpText —— py argparse --help 全文（逐字，py ≥3.10 的 `options:` 面）。
const cwfgHelpText = cwfgUsageLine + "\n" + cwfgHelpDesc + "\n\n" +
	"options:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --json\n" +
	"  --self-test\n"

// 写面旗标两档（照 py 逐字）：
//
//	· 授权旗标（构成「闸」）：--yes / --confirm
//	· 落点旗标（只定写到哪、不构成闸）：--out
var (
	cwfgAuthFlagRe  = regexp.MustCompile(`--(yes|confirm)\b`)
	cwfgOutFlagRe   = regexp.MustCompile(`--out\b`)
	cwfgWriteFlagRe = regexp.MustCompile(`--(yes|confirm|out)\b`) // ★ 死常量：py 定义但正文从不读，逐字保留（无输出面）

	cwfgUsageRe      = regexp.MustCompile(`usage:\s*"([^"]*)"`)
	cwfgUsageConstRe = regexp.MustCompile(`usage:\s*([A-Za-z_][A-Za-z0-9_]*)\s*,`)
	cwfgConstRe      = regexp.MustCompile(`(?m)^\s*(?:const\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*:?=\s*"([^"]*)"\s*$`)
	cwfgPathRe       = regexp.MustCompile(`path:\s*\[\]string\{([^}]*)\}`)
	cwfgDangerRe     = regexp.MustCompile(`danger:\s*(nil|&dangerSpec\{)`)
	cwfgRunRe        = regexp.MustCompile(`run:\s*([A-Za-z_][\w]*)`)
	cwfgBlockRe      = regexp.MustCompile(`\{(?:[^{}]|\{[^{}]*\})*?\}`)
	cwfgInnerStrRe   = regexp.MustCompile(`"([^"]*)"`)
)

// cwfgAuthClassFallback —— py AUTH_CLASS_FALLBACK（授权类闭集回落）。
var cwfgAuthClassFallback = []string{"D2", "D3", "无闸"}

// cwfgEntry —— 一条 catalog 条目（path/usage/usage_known/has_danger/run）。
type cwfgEntry struct {
	Path       string
	Usage      string
	UsageKnown bool
	HasDanger  bool
	Run        string
}

// cwfgRegItem —— 已登记项（命令 + 授权类）。
type cwfgRegItem struct {
	cmd string
	cls string
}

// cwfgHit —— 一条「未声明/未白名单」命中（命令 + 原因 + usage）。
type cwfgHit struct {
	cmd   string
	why   string
	usage string
}

// cwfgClsGroup —— by_cls 的一个分组（保 registered 插入序）。
type cwfgClsGroup struct {
	cls  string
	cmds []string
}

// ── py 目录遍历（sorted(os.listdir)）────────────────────────────────────────
func cwfgReadDirSorted(dir string) ([]string, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	list := make([]string, 0, len(des))
	for _, de := range des {
		list = append(list, de.Name())
	}
	sort.Strings(list)
	return list, nil
}

// cwfgImplWriteFuncs —— py impl_write_funcs：回「含写调用的顶层函数名」集合。
// ★ py 走 `_cwa.scan_file`（跨件 importlib）⇒ Go 复用同包 cwaScanFile（单一定义）。
func cwfgImplWriteFuncs(scopedir string) map[string]bool {
	out := map[string]bool{}
	list, derr := cwfgReadDirSorted(scopedir)
	if derr != nil {
		// ★ B 类读失败：✗ 不静默跳过（那会让该件里的写面函数凭空消失 ⇒ 假绿）⇒ 登记。
		cwfgRegReadFail(cwfgScopeRel, derr)
		return out
	}
	for _, name := range list {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		hits, err := cwaScanFile(cwaPyJoin(scopedir, name))
		if err != nil {
			// ★ B 类读失败：逐件登记（含 err 正文），✗ 不静默 continue。
			cwfgRegReadFail(cwfgScopeRel+"/"+name, err)
			continue
		}
		for _, h := range hits {
			out[h.Func] = true
		}
	}
	return out
}

// cwfgConstUsageMap —— py const_usage_map：扫包内 `*.go`（非测试）取 {标识符: "字面量"}（首选胜）。
func cwfgConstUsageMap(scopedir string) map[string]string {
	out := map[string]string{}
	list, derr := cwfgReadDirSorted(scopedir)
	if derr != nil {
		// ★ B 类读失败：✗ 不静默跳过 ⇒ 登记（照 cwfgImplWriteFuncs 同款）。
		cwfgRegReadFail(cwfgScopeRel, derr)
		return out
	}
	for _, name := range list {
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		text, err := cwaReadText(cwaPyJoin(scopedir, name))
		if err != nil {
			// ★ B 类读失败：逐件登记，✗ 不静默 continue。
			cwfgRegReadFail(cwfgScopeRel+"/"+name, err)
			continue
		}
		for _, m := range cwfgConstRe.FindAllStringSubmatch(text, -1) {
			if _, exists := out[m[1]]; !exists {
				out[m[1]] = m[2]
			}
		}
	}
	return out
}

// cwfgCatalogEntries —— py catalog_entries：粗解析 main.go 的 catalog 条目。
func cwfgCatalogEntries(mainGo string, consts map[string]string) ([]cwfgEntry, error) {
	text, err := cwaReadText(mainGo)
	if err != nil {
		return nil, err
	}
	var ents []cwfgEntry
	for _, block := range cwfgBlockRe.FindAllString(text, -1) {
		um := cwfgUsageRe.FindStringSubmatch(block)
		cm := cwfgUsageConstRe.FindStringSubmatch(block)
		if um == nil && cm == nil {
			continue // 该块没有 usage 字段 ⇒ 非命令条目
		}
		var usage string
		known := false
		if um != nil {
			usage, known = um[1], true
		} else if v, ok := consts[cm[1]]; ok {
			usage, known = v, true
		} else {
			usage, known = "", false // 常量引用但回源件查不到字面量 ⇒ 判不了
		}
		var path string
		if pm := cwfgPathRe.FindStringSubmatch(block); pm != nil {
			var parts []string
			for _, s := range cwfgInnerStrRe.FindAllStringSubmatch(pm[1], -1) {
				parts = append(parts, s[1])
			}
			path = strings.Join(parts, " ")
		}
		hasDanger := false
		if dm := cwfgDangerRe.FindStringSubmatch(block); dm != nil {
			hasDanger = dm[1] != "nil"
		}
		run := ""
		if rm := cwfgRunRe.FindStringSubmatch(block); rm != nil {
			run = rm[1]
		}
		ents = append(ents, cwfgEntry{Path: path, Usage: usage, UsageKnown: known,
			HasDanger: hasDanger, Run: run})
	}
	return ents, nil
}

// ── py 值原语（登记面字段 `ent.get(k, default)` → `str(v)` → `.strip()`）────────
func cwfgFieldOr(ent map[string]any, key string) any {
	if v, ok := ent[key]; ok {
		return v
	}
	return "" // py `.get(key, "")` 的缺键默认
}

// cwfgPyStr —— py `str(v)`（标量面逐字；非标量面为近似，见未覆盖面）。
func cwfgPyStr(v any) string {
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
	case json.Number:
		return x.String()
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, cwfgPyRepr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		var parts []string
		for k, e := range x {
			parts = append(parts, cwfgPyRepr(k)+": "+cwfgPyRepr(e))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%v", x)
	}
}

// cwfgPyRepr —— py `repr(v)` 的近似（仅非标量/失败支用，见未覆盖面）。
func cwfgPyRepr(v any) string {
	switch x := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(x, "'", "\\'") + "'"
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	default:
		return cwfgPyStr(v)
	}
}

// cwfgPyTruthy —— py 真值判（含 None/""/0/[]/{} 为假）。
func cwfgPyTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		f, err := strconv.ParseFloat(x.String(), 64)
		if err != nil {
			return true
		}
		return f != 0
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
}

// cwfgSortedKeys —— `sorted(<set[str]>)`。
func cwfgSortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cwfgFallbackClasses —— set(AUTH_CLASS_FALLBACK)。
func cwfgFallbackClasses() map[string]bool {
	m := map[string]bool{}
	for _, c := range cwfgAuthClassFallback {
		m[c] = true
	}
	return m
}

// cwfgLoadWriteRegistry —— py load_write_registry：回 (entries, cls, err)。
// entries=nil 且 err=="" ⇒ 契约件不存在（按「无登记」处理）；err!="" ⇒ fail-closed。
func cwfgLoadWriteRegistry(root string) ([]any, map[string]bool, string) {
	path := cwaPyJoin(root, cwfgAuthMapRel)
	if fi, statErr := os.Stat(path); statErr != nil || fi.IsDir() {
		return nil, cwfgFallbackClasses(), ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, cwfgFallbackClasses(), "authority-map.json 解析失败：" + err.Error()
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var data map[string]any
	if err := dec.Decode(&data); err != nil {
		return nil, cwfgFallbackClasses(), "authority-map.json 解析失败：" + err.Error()
	}
	cls := map[string]bool{}
	if raw, ok := data["授权类闭集"]; ok {
		if arr, ok := raw.([]any); ok {
			for _, c := range arr {
				m, ok := c.(map[string]any)
				if !ok {
					continue
				}
				v, ok := m["值"]
				if !ok || !cwfgPyTruthy(v) {
					continue
				}
				cls[cwfgPyStr(v)] = true
			}
		}
	}
	if len(cls) == 0 {
		cls = cwfgFallbackClasses()
	}
	raw, ok := data["写面登记"]
	if !ok {
		return nil, cls, cwfgRegMissing
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, cls, cwfgRegMissing
	}
	return arr, cls, ""
}

// cwfgValidateRegistry —— py validate_registry：回 (valid, fake, undecided)。
func cwfgValidateRegistry(reg []any, cls map[string]bool, catByPath map[string]*cwfgEntry) (map[string]string, [][2]string, [][2]string) {
	valid := map[string]string{}
	var fake, undecided [][2]string
	seen := map[string]bool{}
	for _, raw := range reg {
		ent, ok := raw.(map[string]any)
		if !ok {
			fake = append(fake, [2]string{"<非对象>", "条目不是 JSON 对象"})
			continue
		}
		cmd := cwaPyStrip(cwfgPyStr(cwfgFieldOr(ent, "命令")))
		clsV := cwaPyStrip(cwfgPyStr(cwfgFieldOr(ent, "授权类")))
		reason := cwaPyStrip(cwfgPyStr(cwfgFieldOr(ent, "理由")))
		source := cwaPyStrip(cwfgPyStr(cwfgFieldOr(ent, "出处")))
		who := cmd
		if who == "" {
			who = "<命令空>"
		}
		var cat *cwfgEntry
		if cmd != "" {
			cat = catByPath[cmd]
		}
		var probs []string
		if cmd == "" {
			probs = append(probs, "命令空")
		} else if seen[cmd] {
			probs = append(probs, "命令重复登记")
		} else if cat == nil {
			probs = append(probs, "命令面不存在（catalog() 无此 path ⇒ 瞎写的命令）")
		}
		if clsV == "" {
			probs = append(probs, "授权类空")
		} else if !cls[clsV] {
			probs = append(probs, fmt.Sprintf("授权类「%s」不在授权类闭集 {%s}", clsV,
				strings.Join(cwfgSortedKeys(cls), "/")))
		}
		if reason == "" {
			probs = append(probs, "理由空")
		}
		if source == "" {
			probs = append(probs, "出处空")
		}
		if clsV == "无闸" && reason != "" && !strings.Contains(reason, "无闸") {
			probs = append(probs, "无闸条目理由未逐字写「无闸」（洗白）")
		}
		// ★ 无闸不真：只认授权旗标（--yes/--confirm）；落点旗标 --out 不构成闸。
		if clsV == "无闸" && cat != nil && cat.UsageKnown && cwfgAuthFlagRe.MatchString(cat.Usage) {
			probs = append(probs, "无闸声明不真：usage 已声明授权旗标（--yes/--confirm）⇒ 该写面有闸，不得洗成无闸")
		}
		// ★ usage 判不了（常量引用查不到字面量）⇒ 不静默放行。
		if clsV == "无闸" && cat != nil && !cat.UsageKnown {
			undecided = append(undecided, [2]string{who, "判不了：usage 为常量引用但回源件查不到字面量 ⇒ 无法确认它无闸（✗ 不静默当无闸）"})
		}
		if (clsV == "D2" || clsV == "D3") && cat != nil && !cat.HasDanger {
			probs = append(probs, fmt.Sprintf("%s 声明不真：catalog() 无 danger 档", clsV))
		}
		if cmd != "" {
			seen[cmd] = true
		}
		if len(probs) > 0 {
			fake = append(fake, [2]string{who, strings.Join(probs, "；")})
		} else {
			valid[cmd] = clsV
		}
	}
	return valid, fake, undecided
}

// cwfgClassSummary —— py `" · ".join("%s %d" % (k, len(v)) for k, v in sorted(by_cls.items())) or "无"`。
func cwfgClassSummary(byCls []cwfgClsGroup) string {
	if len(byCls) == 0 {
		return "无"
	}
	sorted := append([]cwfgClsGroup{}, byCls...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].cls < sorted[j].cls })
	parts := make([]string, 0, len(sorted))
	for _, g := range sorted {
		parts = append(parts, fmt.Sprintf("%s %d", g.cls, len(g.cmds)))
	}
	return strings.Join(parts, " · ")
}

// cwfgRun —— py run(root, as_json)。
func cwfgRun(stdout, stderr io.Writer, root string, asJSON bool) int {
	cwfgReadFails = nil // ★ 自清：包级登记面不许跨调用残留（残留会让下一次一进来就被 BLOCKED）
	scopedir := cwaPyJoin(root, "core", "cmd", "zerg")
	mainGo := cwaPyJoin(scopedir, "main.go")
	dOK := false
	if fi, err := os.Stat(scopedir); err == nil && fi.IsDir() {
		dOK = true
	}
	mOK := false
	if fi, err := os.Stat(mainGo); err == nil && !fi.IsDir() {
		mOK = true
	}
	if !dOK || !mOK {
		fmt.Fprintln(stderr, cwfgBlockedScope)
		return 2
	}
	reg, cls, rerr := cwfgLoadWriteRegistry(root)
	if rerr != "" {
		fmt.Fprintf(stderr, "BLOCKED —— %s ⇒ rc=2 不给结论\n", rerr)
		return 2
	}
	wfuncs := cwfgImplWriteFuncs(scopedir)
	consts := cwfgConstUsageMap(scopedir)
	// ★ B 类读失败（扫描面读不了的 *.go）⇒ 登记 + 判红 + rc=2：读不到 ≠ 通过（✗ 不给「全绿」结论）。
	if len(cwfgReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(cwfgReadFails))
		for _, u := range cwfgReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return 2
	}
	entries, err := cwfgCatalogEntries(mainGo, consts)
	if err != nil {
		// py 原件此路为未捕异常（回溯 rc=1）；Go fail-closed rc=2，声明为未覆盖面。
		return 2
	}
	if len(entries) == 0 {
		fmt.Fprintln(stderr, cwfgBlockedCat0)
		return 2
	}
	catByPath := map[string]*cwfgEntry{}
	for i := range entries {
		if entries[i].Path != "" {
			catByPath[entries[i].Path] = &entries[i]
		}
	}
	valid, fake, undecided := cwfgValidateRegistry(reg, cls, catByPath)
	undecidedCmds := map[string]bool{}
	for _, u := range undecided {
		undecidedCmds[u[0]] = true
	}
	waived := map[string]bool{}
	for c, k := range valid {
		if k == "无闸" {
			waived[c] = true
		}
	}
	registered := make([]cwfgRegItem, 0, len(valid))
	for c, k := range valid {
		registered = append(registered, cwfgRegItem{cmd: c, cls: k})
	}
	sort.Slice(registered, func(i, j int) bool {
		if registered[i].cls != registered[j].cls {
			return registered[i].cls < registered[j].cls
		}
		return registered[i].cmd < registered[j].cmd
	})
	var byCls []cwfgClsGroup
	byClsIdx := map[string]int{}
	for _, r := range registered {
		idx, ok := byClsIdx[r.cls]
		if !ok {
			byClsIdx[r.cls] = len(byCls)
			byCls = append(byCls, cwfgClsGroup{cls: r.cls})
			idx = byClsIdx[r.cls]
		}
		byCls[idx].cmds = append(byCls[idx].cmds, r.cmd)
	}
	var hits []cwfgHit
	for ei := range entries {
		e := &entries[ei]
		cmd := e.Path
		if cmd == "" {
			cmd = e.Run
		}
		writes := wfuncs[e.Run]
		if !e.UsageKnown {
			if writes && !undecidedCmds[cmd] {
				undecided = append(undecided, [2]string{cmd, "判不了：usage 为常量引用且回源件查不到字面量 ⇒ 无法判它有没有闸"})
			}
			continue
		}
		declaresAuth := cwfgAuthFlagRe.MatchString(e.Usage)
		declaresOut := cwfgOutFlagRe.MatchString(e.Usage)
		if waived[cmd] {
			continue
		}
		if writes && !declaresAuth && !e.HasDanger {
			why := "写面但 usage 未声明授权旗标（--yes/--confirm）且无 danger（缺白名单）"
			if declaresOut {
				why = "带落点旗标 --out 但无授权旗标（--yes/--confirm）⇒ 属「无闸」写面，缺白名单登记"
			}
			hits = append(hits, cwfgHit{cmd: cmd, why: why, usage: e.Usage})
		} else if e.HasDanger && !declaresAuth {
			hits = append(hits, cwfgHit{cmd: cmd, why: "已登记 danger 但 usage 未声明授权旗标", usage: e.Usage})
		}
	}
	rc := 0
	if len(hits) > 0 || len(fake) > 0 || len(undecided) > 0 {
		rc = 1
	}
	if asJSON {
		var b strings.Builder
		b.WriteString(`{"rc": `)
		b.WriteString(strconv.Itoa(rc))
		b.WriteString(`, "registered": [`)
		for i, r := range registered {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(`{"cmd": ` + cwaJSONStr(r.cmd) + `, "cls": ` + cwaJSONStr(r.cls) + `}`)
		}
		b.WriteString(`], "registered_by_class": {`)
		for i, g := range byCls {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(cwaJSONStr(g.cls) + `: [`)
			for j, c := range g.cmds {
				if j > 0 {
					b.WriteString(", ")
				}
				b.WriteString(cwaJSONStr(c))
			}
			b.WriteString(`]`)
		}
		b.WriteString(`}, "unregistered": [`)
		for i, h := range hits {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(`{"cmd": ` + cwaJSONStr(h.cmd) + `, "why": ` + cwaJSONStr(h.why) + `}`)
		}
		b.WriteString(`], "fake_entries": [`)
		for i, f := range fake {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(`{"cmd": ` + cwaJSONStr(f[0]) + `, "why": ` + cwaJSONStr(f[1]) + `}`)
		}
		b.WriteString(`], "undecided": [`)
		for i, u := range undecided {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(`{"cmd": ` + cwaJSONStr(u[0]) + `, "why": ` + cwaJSONStr(u[1]) + `}`)
		}
		b.WriteString(`]}`)
		fmt.Fprintln(stdout, b.String())
	} else {
		fmt.Fprintln(stdout, cwfgRunHeader)
		fmt.Fprintf(stdout, "catalog 条目 %d · 写面函数 %d · 已登记写面 = %d（%s）· 未声明/未白名单 = %d 条 · 假条目 = %d 条 · 判不了 = %d 条\n",
			len(entries), len(wfuncs), len(registered), cwfgClassSummary(byCls), len(hits), len(fake), len(undecided))
		for _, r := range registered {
			fmt.Fprintf(stdout, "  · [%s] %s\n", r.cls, r.cmd)
		}
		for _, h := range hits {
			fmt.Fprintf(stdout, "  ✗ %s：%s ⇒ usage=「%s」\n", h.cmd, h.why, h.usage)
		}
		for _, f := range fake {
			fmt.Fprintf(stdout, "  ✗[假条目] %s：%s\n", f[0], f[1])
		}
		for _, u := range undecided {
			fmt.Fprintf(stdout, "  ✗[判不了] %s：%s\n", u[0], u[1])
		}
		fmt.Fprintln(stdout, "  ★ 登记面 = `core/internal/contract/authority-map.json` 的「写面登记」（白名单）；未登记而会写盘者仍判「未声明/未白名单」；表里假条目（命令不存在 / 授权类越集 /")
		fmt.Fprintln(stdout, "    理由·出处空 / 无闸不真）逐条点名。闸由授权旗标（--yes/--confirm）构成，落点旗标（--out）不算闸。")
	}
	return rc
}

// ── 自检夹具（★ 名含 Fixture ⇒ 被 cwa 的「夹具函数」判据剔出，写点字面不算本件裸写点）────

func cwfgFixtureMk(td, mainSrc string, writers map[string]string) {
	d := cwaPyJoin(td, "core", "cmd", "zerg")
	_ = os.MkdirAll(d, 0o755)
	_ = os.WriteFile(cwaPyJoin(d, "main.go"), []byte(mainSrc), 0o644)
	names := make([]string, 0, len(writers))
	for n := range writers {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		_ = os.WriteFile(cwaPyJoin(d, n), []byte(writers[n]), 0o644)
	}
}

func cwfgFixtureWriteReg(td string, entries []any) {
	d := cwaPyJoin(td, "core", "internal", "contract")
	_ = os.MkdirAll(d, 0o755)
	if entries == nil {
		entries = []any{}
	}
	data := map[string]any{
		"授权类闭集": []any{map[string]any{"值": "D2"}, map[string]any{"值": "D3"}, map[string]any{"值": "无闸"}},
		"写面登记":  entries,
	}
	b, _ := json.Marshal(data)
	_ = os.WriteFile(cwaPyJoin(d, "authority-map.json"), b, 0o644)
}

func cwfgFixtureWriteRegRaw(td string, data map[string]any) {
	d := cwaPyJoin(td, "core", "internal", "contract")
	_ = os.MkdirAll(d, 0o755)
	b, _ := json.Marshal(data)
	_ = os.WriteFile(cwaPyJoin(d, "authority-map.json"), b, 0o644)
}

// cwfgFixtureCapture —— py `_run_capture`：stdout+stderr 同灌一块（仅做子串判）。
func cwfgFixtureCapture(td string) (int, string) {
	var buf bytes.Buffer
	rc := cwfgRun(&buf, &buf, td, false)
	return rc, buf.String()
}

// cwfgFkRepr —— 失败支的近似 repr（PASS 档不打印，见未覆盖面）。
func cwfgFkRepr(items [][2]string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, "("+cwfgPyStr(it[0])+", "+cwfgPyStr(it[1])+")")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cwfgSelfTest —— py self_test()（10 格）。★ 名含 SelfTest 且体含 os.MkdirTemp ⇒ 夹具函数。
func cwfgSelfTest(stdout, stderr io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "")
	if err != nil {
		fmt.Fprintf(stdout, "自检 FAIL（10 格）\n")
		return 1
	}
	defer os.RemoveAll(td)

	mainSrc := "package main\n" +
		"var cat = []*command{\n" +
		" {path: []string{\"help\", \"export\"}, usage: \"zerg help export [--out <目录>] [--dry-run]\", run: cmdHelpExport},\n" +
		" {path: []string{\"impact\"}, usage: impactUsageLine, run: cmdImpact},\n" +
		" {path: []string{\"guarded\"}, usage: \"zerg guarded [--yes]\", danger: &dangerSpec{...}, run: cmdGuarded},\n" +
		"}\n"
	writers := map[string]string{
		"family_help.go": "func cmdHelpExport(inv *invocation, o, e io.Writer) int {\n\t" +
			"os.WriteFile" + "(\"x\", b, 0o644)\n\treturn 0\n}\n",
		"family_impact.go": "const impactUsageLine = \"zerg impact <目标> [--json <字段>]\"\n\n" +
			"func cmdImpact(inv *invocation, o, e io.Writer) int {\n\t" +
			"os.WriteFile" + "(\"y\", b, 0o644)\n\treturn 0\n}\n",
		"family_guarded.go": "func cmdGuarded(inv *invocation, o, e io.Writer) int {\n\t" +
			"os.WriteFile" + "(\"z\", b, 0o644)\n\treturn 0\n}\n",
	}
	cwfgFixtureMk(td, mainSrc, writers)
	regOK := map[string]any{"命令": "help export", "授权类": "无闸",
		"理由": "无闸 —— usage 只有落点旗标 --out、无授权旗标", "出处": "自检"}
	regImpact := map[string]any{"命令": "impact", "授权类": "无闸",
		"理由": "无闸 —— usage 无授权旗标", "出处": "自检"}

	// 格1 正控：空登记 ⇒ help export + impact 会写盘却无闸又无白名单 ⇒ rc=1
	cwfgFixtureWriteReg(td, []any{})
	if cwfgRun(stdout, stderr, td, false) != 1 {
		fmt.Fprintln(stdout, "  ✗ 格1失败：空登记应回 rc=1")
		ok = false
	}

	// 格2 ★缺陷①②负控：两条都登记后应 rc=0 且无假条目
	cwfgFixtureWriteReg(td, []any{regOK, regImpact})
	rc2, out2 := cwfgFixtureCapture(td)
	if rc2 != 0 {
		fmt.Fprintf(stdout, "  ✗ 格2失败：两条都登记（--out 落点旗标 + 常量 usage）后应回 rc=0（实得 rc=%d）\n", rc2)
		ok = false
	}
	if strings.Contains(out2, "✗[假条目]") {
		fmt.Fprintf(stdout, "  ✗ 格2失败：出现假条目假判（out=%q）\n", out2)
		ok = false
	}

	// 格3 ★缺陷①负控（细）：常量 usage 的 impact 不得被假判「不存在」；只登记 impact ⇒ rc=1 只余 help export
	cwfgFixtureWriteReg(td, []any{regImpact})
	_, fk3, _ := cwfgValidateRegistry(
		[]any{map[string]any{"命令": "impact", "授权类": "无闸", "理由": "无闸", "出处": "x"}},
		map[string]bool{"无闸": true},
		map[string]*cwfgEntry{"impact": {Usage: "zerg impact <目标>", UsageKnown: true, HasDanger: false}})
	if len(fk3) != 0 {
		fmt.Fprintf(stdout, "  ✗ 格3失败：常量 usage 的 impact 被假判假条目（fk=%s）\n", cwfgFkRepr(fk3))
		ok = false
	}
	if cwfgRun(stdout, stderr, td, false) != 1 {
		fmt.Fprintln(stdout, "  ✗ 格3失败：只登记 impact 应回 rc=1（help export 仍缺白名单）")
		ok = false
	}

	// 格4 负控：瞎写命令的假条目 ⇒ 仍被点名 ⇒ rc=1
	bogus := map[string]any{"命令": "bogus no-such cmd", "授权类": "无闸", "理由": "无闸 —— 瞎写", "出处": "自检"}
	cwfgFixtureWriteReg(td, []any{regOK, regImpact, bogus})
	if cwfgRun(stdout, stderr, td, false) != 1 {
		fmt.Fprintln(stdout, "  ✗ 格4失败：假条目（瞎写命令）应回 rc=1")
		ok = false
	}
	_, fk4, _ := cwfgValidateRegistry(
		[]any{map[string]any{"命令": "bogus no-such cmd", "授权类": "无闸", "理由": "无闸", "出处": "x"}},
		map[string]bool{"无闸": true}, map[string]*cwfgEntry{})
	if len(fk4) != 1 || !strings.Contains(fk4[0][1], "不存在") {
		fmt.Fprintf(stdout, "  ✗ 格4失败：假条目未被点名（fk=%s）\n", cwfgFkRepr(fk4))
		ok = false
	}

	// 格5 负控：把 usage 已声明授权旗标 --yes 的 guarded 洗成「无闸」⇒ 无闸不真 ⇒ rc=1 且点名
	cwfgFixtureWriteReg(td, []any{map[string]any{"命令": "guarded", "授权类": "无闸",
		"理由": "无闸 —— 洗白试试", "出处": "自检"}})
	if cwfgRun(stdout, stderr, td, false) != 1 {
		fmt.Fprintln(stdout, "  ✗ 格5失败：无闸不真应回 rc=1")
		ok = false
	}
	_, fk5, _ := cwfgValidateRegistry(
		[]any{map[string]any{"命令": "guarded", "授权类": "无闸", "理由": "无闸", "出处": "x"}},
		map[string]bool{"无闸": true},
		map[string]*cwfgEntry{"guarded": {Usage: "zerg guarded [--yes]", UsageKnown: true, HasDanger: true}})
	if len(fk5) != 1 || !strings.Contains(fk5[0][1], "不真") {
		fmt.Fprintf(stdout, "  ✗ 格5失败：无闸不真未被点名（fk=%s）\n", cwfgFkRepr(fk5))
		ok = false
	}

	// 格6 ★缺陷②负控（单点）：只有落点旗标 --out 的写面登记「无闸」⇒ 不判假
	_, fk6, _ := cwfgValidateRegistry(
		[]any{map[string]any{"命令": "help export", "授权类": "无闸", "理由": "无闸", "出处": "x"}},
		map[string]bool{"无闸": true},
		map[string]*cwfgEntry{"help export": {Usage: "zerg help export [--out <目录>] [--dry-run]",
			UsageKnown: true, HasDanger: false}})
	if len(fk6) > 0 {
		fmt.Fprintf(stdout, "  ✗ 格6失败：只有落点旗标 --out 的写面被判假（fk=%s）\n", cwfgFkRepr(fk6))
		ok = false
	}

	// 格7 空域：缺 main.go ⇒ rc=2
	if cwfgRun(stdout, stderr, cwaPyJoin(td, "nope"), false) != 2 {
		fmt.Fprintln(stdout, "  ✗ 格7失败：缺 main.go 未回 rc=2")
		ok = false
	}

	// 格8 ★缺陷①负控：usage 常量引用查不到字面量 ⇒ 报「判不了」，不得假判「命令面不存在」
	td2, err2 := os.MkdirTemp("", "")
	if err2 != nil {
		fmt.Fprintf(stdout, "自检 FAIL（10 格）\n")
		return 1
	}
	defer os.RemoveAll(td2)
	ghostMain := "package main\n" +
		"var cat = []*command{\n" +
		" {path: []string{\"ghost\"}, usage: ghostUsageLine, run: cmdGhost},\n" +
		"}\n"
	ghostWriters := map[string]string{
		"family_ghost.go": "func cmdGhost(inv *invocation, o, e io.Writer) int {\n\t" +
			"os.WriteFile" + "(\"g\", b, 0o644)\n\treturn 0\n}\n",
	}
	cwfgFixtureMk(td2, ghostMain, ghostWriters)
	cwfgFixtureWriteReg(td2, []any{})
	rc8, out8 := cwfgFixtureCapture(td2)
	if rc8 != 1 {
		fmt.Fprintln(stdout, "  ✗ 格8失败：常量查不到的写面应回 rc=1（判不了）")
		ok = false
	}
	if !strings.Contains(out8, "判不了") || strings.Contains(out8, "命令面不存在") {
		fmt.Fprintf(stdout, "  ✗ 格8失败：应报「判不了」且不得假判「不存在」（out=%q）\n", out8)
		ok = false
	}

	// 格9 空域：登记表读不到（缺「写面登记」表）⇒ rc=2
	td3, err3 := os.MkdirTemp("", "")
	if err3 != nil {
		fmt.Fprintf(stdout, "自检 FAIL（10 格）\n")
		return 1
	}
	defer os.RemoveAll(td3)
	cwfgFixtureMk(td3, mainSrc, writers)
	cwfgFixtureWriteRegRaw(td3, map[string]any{"授权类闭集": []any{map[string]any{"值": "无闸"}}})
	if cwfgRun(stdout, stderr, td3, false) != 2 {
		fmt.Fprintln(stdout, "  ✗ 格9失败：登记表读不到未回 rc=2")
		ok = false
	}

	// 格10 空域：catalog() 解析出 0 条 ⇒ rc=2
	td4, err4 := os.MkdirTemp("", "")
	if err4 != nil {
		fmt.Fprintf(stdout, "自检 FAIL（10 格）\n")
		return 1
	}
	defer os.RemoveAll(td4)
	cwfgFixtureMk(td4, "package main\n\nvar cat = []*command{}\n", map[string]string{})
	cwfgFixtureWriteReg(td4, []any{})
	if cwfgRun(stdout, stderr, td4, false) != 2 {
		fmt.Fprintln(stdout, "  ✗ 格10失败：catalog 0 条未回 rc=2")
		ok = false
	}

	res := "FAIL"
	if ok {
		res = "PASS"
	}
	fmt.Fprintf(stdout, "自检 %s（10 格）\n", res)
	if ok {
		return 0
	}
	return 1
}

// ── argparse 仿（--root / --json / --self-test / -h / --help）──────────────
type cwfgArgs struct {
	root     string
	asJSON   bool
	selfTest bool
}

// cwfgArgErr —— 打用法行 + `prog: error: msg` 到 stderr，回 rc=2（照 argparse 逐字）。
func cwfgArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, cwfgUsageLine)
	fmt.Fprintf(stderr, "%s: error: %s\n", cwfgProg, msg)
	return 2
}

// cwfgResolveOption —— argparse 唯一前缀缩写（候选按定义序）。
func cwfgResolveOption(p string) []string {
	cands := []string{"--help", "--root", "--json", "--self-test"}
	var hits []string
	for _, c := range cands {
		if strings.HasPrefix(c, p) {
			hits = append(hits, c)
		}
	}
	return hits
}

func cwfgParseArgs(stdout, stderr io.Writer, argv []string) (cwfgArgs, int, bool) {
	var a cwfgArgs
	if v, ok := os.LookupEnv("ZERG_ROOT"); ok {
		a.root = v
	} else if wd, err := os.Getwd(); err == nil {
		a.root = wd
	}
	var extras []string
	i := 0
	for i < len(argv) {
		arg := argv[i]
		if arg == "--" { // argparse: `--` 及其后全部进 extras（无 positional 定义）
			extras = append(extras, argv[i:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			name, val := arg, ""
			hasVal := false
			if eq := strings.Index(arg, "="); eq >= 0 {
				name, val, hasVal = arg[:eq], arg[eq+1:], true
			}
			hits := cwfgResolveOption(name)
			if len(hits) == 0 {
				extras = append(extras, arg) // 未识别长选项 ⇒ 收进 extras（继续解析）
				i++
				continue
			}
			if len(hits) > 1 {
				return a, cwfgArgErr(stderr, "ambiguous option: "+name+" could match "+strings.Join(hits, ", ")), false
			}
			switch hits[0] {
			case "--root":
				if !hasVal {
					// argparse：选项样的下一词不算值（`-` 单独除外）⇒ expected one argument
					if i+1 >= len(argv) || (strings.HasPrefix(argv[i+1], "-") && argv[i+1] != "-") {
						return a, cwfgArgErr(stderr, "argument --root: expected one argument"), false
					}
					val = argv[i+1]
					i++
				}
				a.root = val
			case "--json":
				if hasVal {
					return a, cwfgArgErr(stderr, "argument --json: ignored explicit argument '"+val+"'"), false
				}
				a.asJSON = true
			case "--self-test":
				if hasVal {
					return a, cwfgArgErr(stderr, "argument --self-test: ignored explicit argument '"+val+"'"), false
				}
				a.selfTest = true
			case "--help":
				fmt.Fprint(stdout, cwfgHelpText)
				return a, 0, false
			}
		} else if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			// 短选项：`-h` 开头（含聚类 `-hx`）⇒ 立即帮助；其余短选项 ⇒ 收进 extras
			if arg[1] == 'h' {
				fmt.Fprint(stdout, cwfgHelpText)
				return a, 0, false
			}
			extras = append(extras, arg)
		} else {
			extras = append(extras, arg)
		}
		i++
	}
	if len(extras) > 0 {
		return a, cwfgArgErr(stderr, "unrecognized arguments: "+strings.Join(extras, " ")), false
	}
	return a, 0, true
}

// checkWriteFaceGateGateMain —— 入口（与 py main(argv) 对齐）。
func checkWriteFaceGateGateMain(stdout, stderr io.Writer, argv []string) int {
	args, rc, cont := cwfgParseArgs(stdout, stderr, argv)
	if !cont {
		return rc
	}
	if args.selfTest {
		return cwfgSelfTest(stdout, stderr)
	}
	return cwfgRun(stdout, stderr, args.root, args.asJSON)
}
