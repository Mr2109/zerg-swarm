// check_ai_direct_edits.go —— 门禁件 `scripts/gates/check-ai-direct-edits.py` 的 **Go 真搬样板**（v2.5.14 · M2-0 · 路 A · 首字母 a–e 批）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）；
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据）的一枚样板：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它的理由（M2-0 = 「无外部树依赖 · 输出确定有序 · 退码 0/1/2 · 纯静态无写面 · 行数适中」）：
//
//	✓ 选：`check-ai-direct-edits.py`（258 行）——判据面只读**仓内**一件
//	  `core/cmd/zerg/testdata/cli-matrix.json`（门⑪ 的同一份投影）＋ **一个回执目录**
//	  （默认 `$ZERG_RECEIPT_DIR` / `$ZERG_STATE_DIR/receipts` / `~/.zerg/state/receipts`，可用 `--dir` 指）——
//	  **无外部树依赖**（不碰文档仓 / 公开产物树 / `bin/`）。
//	  · 输出**定序**：回执页 `sorted(os.listdir(...))` + 逐页判定序固定（REQ 序 R2→R3→R1→R4），
//	    无 `os.walk` / 无「按 inode 序」读取 ⇒ 逐字节对拍可证。
//	  · 退码 `0/1/2` 三档齐：0 全绿 / 1 门禁红（R1–R4 任一）/ 2 不给结论（空转 · 目录不存在 · 未知旗标 · `--json` 无机器面）。
//	  · **纯静态、无写面**（默认档不写任何件；仅 `--self-test` 在临时目录造夹具 —— 与 py 同）。
//	  · `git status` 现读**不含它**（无人正在改）。
//	✗ 不选 `check-ci-package-parity.py` / `check-contract-registry.py` / `check-error-kinds.py`：**已落仓**（同批前单已搬，`git status` 见 `?? core/cmd/zerg/check_*.go`）。
//	✗ 不选 `check-curl-timeout.py`（167 行）：扫描根靠 `os.walk` ⇒ **遍历序非确定**（Go `WalkDir` 字典序 ≠ py `scandir` inode 序）⇒ 不满足「输出有序」。
//	✗ 不选 `check-doc-section-numbers.py`（189 行）等 8 件 `doc-*`：**另一路在改**（`git status` 现读 M）⇒ 违反 M2 并行路铁律②（共享件）。
//	✗ 不选 `check-anchor-uniqueness.py`（340 行）/ `check-compat-manifest.py`（449 行）/ `check-affected-map.py`（523 行）/ `check-exit-codes.py`（649 行）/ `check-build-tags.py`（863 行）：判据面**依赖同级外部树或输入物料**（案卷/清单 JSON 由调用方指定 · 可指向仓外）⇒ 不属 M2-0「无输入物料」，且部分**行数偏大**。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改任何 `add_step`）；
//	· **不**删 `scripts/gates/check-ai-direct-edits.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· 入口 `aidirecteditsGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态：
//	  先证明「搬得逐字相同」，接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func aidirecteditsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 通过 / 1 门禁红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ── ★ 两条标准声明行（M2 样板必备）──────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py `DEFAULT_ROOT = dirname(dirname(HERE))`，`HERE = dirname(abspath(__file__))`
//	    ⇒ 由**脚本位置**（`scripts/gates` 上溯两层）**固定**指向仓根，与 cwd 无关；Go 入口**无 `__file__` 语境**
//	    ⇒ 本件默认根取 **cwd**（`os.Getwd()`）。同真仓 + cwd=仓根 时两者读数逐字相同（已对拍 C14）；
//	    「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 对拍一律显式传 `--root`。
//	(b) **自指检查**：本件扫描面 = **一个回执目录**（state 树，默认 `~/.zerg/state/receipts`）＋ **仓内 `cli-matrix.json`**，
//	    **不读** `scripts/gates/` · **不读** `core/cmd/zerg/*.go` ⇒ **本件不在自己扫描面内**。
//	    故夹具 / 自检 / 件头里的标记字面（`zerg gate run` `/ §20.3` / `TR-…`）**无需**回避反引号原串直写 ——
//	    即**不存在** `check_wall_layer.go` 血证（曾把门⑥ 茧壁凭空判红）里那种「本件在自己扫描面内」的风险。
//
// 已对拍面（仓外夹具两态逐字 · ≥14 格）：
//
//	全绿 · 各造 violation（R1 AI 直改 / R2 缺格 / R2 trace 形态 / R3 件名 ≠ round_id / R4 幽灵调用）·
//	成对负控（动过仓 ↔ 没动仓）· 多页定序 · 空转（回执目录 0 页）⇒ 2 · 取不到输入（回执目录不存在）⇒ 2 ·
//	未知旗标 ⇒ 2 · 位置参数 ⇒ 2 · `--json` / `--json=stdout` 无机器面拒收 ⇒ 2 · `-h` / `--help` ⇒ 0 ·
//	`--self-test` 三档 · 真仓现读（显式根 / 缺省根 · cwd=仓根）。
//
// 未覆盖面（✗ 不声称逐字对齐 · 逐条已列）：
//
//	① **回执坏 JSON 的异常正文**：py `json.load` 抛 `json.decoder.JSONDecodeError`，错误行 `R2 回执不是合法 JSON：%s（%s）`
//	   的第②个 `%s` = **CPython 的 JSON 扫描器文案**（`Expecting property name enclosed in double quotes: line 1 column 2 (char 1)` 一类）；
//	   本件落 Go `encoding/json` 的文案 ⇒ **退码同为 1，但该行正文不同**。故对拍表**不收**「坏 JSON」格。
//	② **`cli-matrix.json` 读不出来的失败行**：py `open/json` 抛的 `FileNotFoundError str` 进 `命令树投影读不出来（…）：<e>`
//	   的 `<e>` 位；Go 文案不同 ⇒ 同样**退码 2、正文不同**。故对拍表**不收**「根下无矩阵」格（「取不到输入」改用**回执目录不存在**那一格，该格 py 文案与 Go 逐字同）。
//	③ **回执字段非字符串**（`round_id`/`trace_id`/`what`/`next` 为 `null`/数/数组）：py 走 `str(...)` / `%r` 的 **Python repr**；
//	   本件对**标量**（串/布尔/数/null）逐字复刻，**嵌套容器**（数组/对象）的 `str()` repr 未复刻 ⇒ 不属逐字对齐承诺。
//	④ **argparse 前缀缩写**（`--ro`/`--di`）与 `--help=…` 等写法：py argparse **收**缩写；本件只认全称 ⇒ 不属承诺。
//	⑤ **EPIPE / 写端关闭**：py 的 `print` 抛 `BrokenPipeError`（进程退 1）；Go `fmt.Fprintf` 吞错返回 ⇒ 不属承诺。
//	⑥ py `--self-test` 用 `subprocess` **真跑自身进程**取真退出码；本件 `--self-test` **进程内**复用同一入口取 rc
//	   （打印文案逐字对齐；差在实现手段，不在读数）。若 `sys.executable` 缺失/py 件被移走，py 自检会报错而本件不受影响 ⇒ 这一档不属承诺。
//
//	—— ①②③④⑤⑥ 均**不在**本单对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ── 与 py 逐条对齐的常量 / 正则（名字前缀 `ade` 防包内同名）────────────────────────

// adeMatrixRel —— py `MATRIX_REL`（命令树投影相对仓根的路径）。
const adeMatrixRel = "core/cmd/zerg/testdata/cli-matrix.json"

// adeProg —— py argparse 的 prog（= 原件基名，帮助/报错正文里逐字出现）。
const adeProg = "check-ai-direct-edits.py"

// adeUsageLine —— argparse 报错首行（含 `usage:`，逐字）。
const adeUsageLine = "usage: check-ai-direct-edits.py [-h] [--root ROOT] [--dir DIR] [--self-test]\n"

// adeHelp —— `-h/--help` 正文（现读复刻 · 现读 py = `/usr/bin/python3` 3.9.6 ⇒ argparse 段标题 `optional arguments:`；
// 换 Python 版本（≥3.10 段标题作 `options:` / 措辞）即不同 ⇒ 见件头未覆盖面④延伸）。
const adeHelp = "usage: check-ai-direct-edits.py [-h] [--root ROOT] [--dir DIR] [--self-test]\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --dir DIR\n" +
	"  --self-test\n"

// adeREQ —— py `REQ`（R2 五格中的「必填四格」）。
var adeREQ = []string{"round_id", "trace_id", "what", "next"}

// adeTraceRE —— py `TRACE_RE = ^TR-[0-9a-f]{12}$`。
var adeTraceRE = regexp.MustCompile(`^TR-[0-9a-f]{12}$`)

// adeZergRE —— py `ZERG_RE = (?:^|[\s（(])zerg\s+([A-Za-z][A-Za-z0-9_-]*)`。
var adeZergRE = regexp.MustCompile(`(?:^|[\s（(])zerg\s+([A-Za-z][A-Za-z0-9_-]*)`)

// adeJSONFields —— py `JSON_FIELDS`（本门无机器读面 ⇒ 空 · **唯一真源**）。
var adeJSONFields = []string{}

// ── py `default_dir()` ──────────────────────────────────────────────────────

func adeDefaultDir() string {
	if d := strings.TrimSpace(os.Getenv("ZERG_RECEIPT_DIR")); d != "" {
		return d
	}
	if s := strings.TrimSpace(os.Getenv("ZERG_STATE_DIR")); s != "" {
		return filepath.Join(s, "receipts")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".zerg", "state", "receipts")
}

// ── py `default root`（差异见件头声明行 (a)）────────────────────────────────
//
// py：DEFAULT_ROOT = dirname(dirname(HERE))（HERE = 脚本所在 `scripts/gates`）⇒ 与 cwd 无关。
// Go：无 `__file__` ⇒ 取 cwd（同真仓 + cwd=仓根 时逐字相同）。
func adeDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ── py `load_command_tree(root)` ────────────────────────────────────────────

func adeLoadCommandTree(root string) (map[string]bool, string) {
	p := filepath.Join(root, adeMatrixRel)
	b, err := os.ReadFile(p)
	if err != nil {
		// py 的异常正文（FileNotFoundError str）与 Go 不同 ⇒ 见件头未覆盖面②。
		return nil, fmt.Sprintf("命令树投影读不出来（%s）：%v", adeMatrixRel, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Sprintf("命令树投影读不出来（%s）：%v", adeMatrixRel, err)
	}
	names := map[string]bool{}
	if cases, ok := m["cases"].([]any); ok {
		for _, c := range cases {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if s, ok := cm["command"].(string); ok && s != "" {
				f := strings.Fields(s)
				if len(f) > 0 {
					names[f[0]] = true
				}
			}
		}
	}
	return names, ""
}

// ── py `check_dir(d, tree)` → (rc, lines) ───────────────────────────────────

func adeCheckDir(d string, tree map[string]bool) (int, []string) {
	fi, err := os.Stat(d)
	if err != nil || !fi.IsDir() {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：回执目录不存在（%s）—— 没有回执 ≠ 绿（§20.3 H3）", d)}
	}
	ents := []string{}
	des, err := os.ReadDir(d)
	if err == nil {
		for _, e := range des {
			if strings.HasSuffix(e.Name(), ".json") {
				ents = append(ents, e.Name())
			}
		}
	}
	sort.Strings(ents)
	if len(ents) == 0 {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：回执目录里 0 页（%s）—— 没有回执 ≠ 绿（§20.3 H3）", d)}
	}
	var errs []string
	for _, name := range ents {
		p := filepath.Join(d, name)
		raw, rerr := os.ReadFile(p)
		var r map[string]any
		if rerr != nil {
			errs = append(errs, fmt.Sprintf("R2 回执不是合法 JSON：%s（%v）", name, rerr))
			continue
		}
		if jerr := json.Unmarshal(raw, &r); jerr != nil {
			// py 的 JSONDecodeError 文案与 Go 不同 ⇒ 见件头未覆盖面①（退码同、正文异）。
			errs = append(errs, fmt.Sprintf("R2 回执不是合法 JSON：%s（%v）", name, jerr))
			continue
		}
		if r == nil {
			errs = append(errs, fmt.Sprintf("R2 回执不是合法 JSON：%s（裸 null）", name))
			continue
		}
		rid := adeGetStr(r, "round_id")
		// R2 五格 + trace_id 形态
		for _, k := range adeREQ {
			if !adeTruthy(r, k) {
				errs = append(errs, fmt.Sprintf("R2 回执缺格 %s：%s", k, name))
			}
		}
		if rid != "" {
			if !adeTraceRE.MatchString(adeGetStr(r, "trace_id")) {
				errs = append(errs, fmt.Sprintf("R2 trace_id 形态不对（要 TR- + 12 位十六进制）：%s ⇒ %s",
					name, adePyReprValue(r, "trace_id")))
			}
		}
		// R3 件名 ↔ round_id
		if rid != "" && name != rid+".json" {
			errs = append(errs, fmt.Sprintf("R3 件名与 round_id 对不上：文件 %s · round_id %s", name, adePyRepr(rid)))
		}
		evtext := adeEvidenceText(r)
		commits := adeListLen(r, "commits")
		// R1 H1：动过仓 ⇒ 必须有一条命令面调用
		calls := adeZergRE.FindAllStringSubmatch(evtext, -1)
		var callNames []string
		for _, m := range calls {
			callNames = append(callNames, m[1])
		}
		if commits > 0 && len(callNames) == 0 {
			errs = append(errs, fmt.Sprintf("R1 **AI 直改**：%s 说动过仓（commits=%d 条）而证据里**没有任何 `zerg …` 调用** "+
				"⇒ 门红（§20.3 H1：执行只能走命令面）", name, commits))
		}
		// R4 点名的命令面调用必须真在命令树里
		if tree != nil {
			for _, c := range callNames {
				if !tree[c] {
					errs = append(errs, fmt.Sprintf("R4 证据里点名的命令面调用 `zerg %s` 在命令树里**解析不到**（幽灵调用）：%s", c, name))
				}
			}
		}
	}
	if len(errs) > 0 {
		return 1, errs
	}
	return 0, []string{fmt.Sprintf("回执 %d 页逐页过：H1 零直改 ✓ · H3 五格 ✓ · 件名 ✓ · 命令面调用可解析 ✓", len(ents))}
}

// adeGetStr —— py `str(r.get(k, ""))`（缺格 ⇒ ""；标量逐字复刻；容器见未覆盖面③）。
func adeGetStr(r map[string]any, k string) string {
	v, ok := r[k]
	if !ok {
		return ""
	}
	return adePyStr(v)
}

// adeTruthy —— py `str(r.get(k, "")).strip()` 的真假（缺格 ⇒ "" ⇒ 假）。
func adeTruthy(r map[string]any, k string) bool {
	v, ok := r[k]
	if !ok {
		return false
	}
	return strings.TrimSpace(adePyStr(v)) != ""
}

// adePyStr —— py `str(x)`（串/布尔/数/null 逐字；容器见未覆盖面③）。
func adePyStr(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		return adePyNum(t)
	case json.Number:
		return t.String()
	}
	return ""
}

func adePyNum(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "nan"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// adePyReprValue —— py `%r` 于 `r.get(k)`（缺格 ⇒ 空串的 repr，即两个单引号）。
func adePyReprValue(r map[string]any, k string) string {
	v, ok := r[k]
	if !ok {
		return "''"
	}
	if v == nil {
		return "None"
	}
	if s, ok := v.(string); ok {
		return adePyRepr(s)
	}
	return adePyStr(v)
}

// adePyRepr —— py `repr(str)`（常用档：无引号冲突走单引号；含 `'` 不含 `"` 走双引号）。
func adePyRepr(s string) string {
	hasS := strings.ContainsRune(s, '\'')
	hasD := strings.ContainsRune(s, '"')
	if hasS && !hasD {
		return "\"" + s + "\""
	}
	if hasS && hasD {
		return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
	}
	return "'" + s + "'"
}

// adeEvidenceText —— py：`ev = r.get("evidence") or []`；串 ⇒ 包一层；`" · ".join(str(x) …)`。
func adeEvidenceText(r map[string]any) string {
	v, ok := r["evidence"]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if arr, ok := v.([]any); ok {
		parts := make([]string, len(arr))
		for i, x := range arr {
			parts[i] = adePyStr(x)
		}
		return strings.Join(parts, " · ")
	}
	return "" // 容器/怪异形态见未覆盖面③
}

// adeListLen —— py `len(commits)`，其中 `commits = r.get("commits") or []`，串 ⇒ 包一层。
func adeListLen(r map[string]any, k string) int {
	v, ok := r[k]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case string:
		if t == "" {
			return 0
		}
		return 1
	case []any:
		return len(t)
	}
	return 0
}

// ── py `refuse_json_without_machine_face(argv)` ─────────────────────────────

func adeRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	var hit []string
	for _, a := range argv {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			hit = append(hit, a)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	parts := make([]string, len(hit))
	for i, h := range hit {
		parts[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(parts, " · "))
	if len(adeJSONFields) == 0 {
		fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
	} else {
		fs := make([]string, len(adeJSONFields))
		for i, f := range adeJSONFields {
			fs[i] = "`" + f + "`"
		}
		fmt.Fprintf(stderr, "   字段表：%s\n", strings.Join(fs, " · "))
	}
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	return 2, true
}

// ── py `self_test(root)`：三档之外的全档（1 正控 + 5 负控 + 2 空转）────────────

func adeSelfTest(stdout io.Writer) int {
	var fails []string
	tmp, err := os.MkdirTemp("", "zerg-gate13-")
	if err != nil {
		tmp = os.TempDir()
	}
	defer os.RemoveAll(tmp)

	// py `_mk`：把回执对象写成 `<dir>/<name>`（ensure_ascii=False ⇒ 中文原样）。
	mk := func(d, name string, obj map[string]any) {
		b, _ := json.Marshal(obj)
		_ = os.WriteFile(filepath.Join(d, name), b, 0o644)
	}
	// py `_good`：合规回执模板。
	good := func(rid string, kw map[string]any) map[string]any {
		r := map[string]any{
			"round_id":   rid,
			"trace_id":   "TR-0123456789ab",
			"what":       "做了事",
			"next":       "下一步",
			"blockers":   []any{},
			"evidence":   []any{"zerg gate run --fast"},
			"commits":    []any{"deadbee"},
			"created_at": "2026-09-20T00:00:00+08:00",
		}
		for k, v := range kw {
			r[k] = v
		}
		return r
	}
	mkdir := func(sub string) string {
		p := filepath.Join(tmp, sub)
		_ = os.MkdirAll(p, 0o755)
		return p
	}

	// ① 空转：目录不存在 ⇒ 2
	if rc, _ := adeCheckDir(filepath.Join(tmp, "nope"), nil); rc != 2 {
		fails = append(fails, fmt.Sprintf("自检① 目录不存在应 rc=2，实测 %d", rc))
	}
	// ② 空转：目录在但 0 页 ⇒ 2
	if rc, _ := adeCheckDir(mkdir("empty"), nil); rc != 2 {
		fails = append(fails, fmt.Sprintf("自检② 0 页应 rc=2，实测 %d", rc))
	}
	// ③ 正控：合规 ⇒ 0
	okDir := mkdir("ok")
	mk(okDir, "R-20260920T000001.json", good("R-20260920T000001", nil))
	if rc, _ := adeCheckDir(okDir, map[string]bool{"gate": true}); rc != 0 {
		fails = append(fails, fmt.Sprintf("自检③ 合规回执应 rc=0，实测 %d", rc))
	}
	// ④ 负控 R1：动过仓但证据里没有命令面调用 ⇒ 1
	n1 := mkdir("n1")
	mk(n1, "R-20260920T000002.json", good("R-20260920T000002", map[string]any{"evidence": []any{"vim core/x.go"}}))
	if rc, _ := adeCheckDir(n1, map[string]bool{"gate": true}); rc != 1 {
		fails = append(fails, fmt.Sprintf("自检④ R1 应 rc=1（AI 直改），实测 %d", rc))
	}
	// ⑤ 负控 R1 不误判：没动仓（commits 空）⇒ 0
	n2 := mkdir("n2")
	mk(n2, "R-20260920T000003.json", good("R-20260920T000003", map[string]any{"commits": []any{}, "evidence": []any{"只读 grep"}}))
	if rc, _ := adeCheckDir(n2, map[string]bool{"gate": true}); rc != 0 {
		fails = append(fails, fmt.Sprintf("自检⑤ 没动仓应 rc=0（不误判），实测 %d", rc))
	}
	// ⑥ 负控 R2：trace_id 形态坏 ⇒ 1
	n3 := mkdir("n3")
	mk(n3, "R-20260920T000004.json", good("R-20260920T000004", map[string]any{"trace_id": "trace-bad"}))
	if rc, _ := adeCheckDir(n3, map[string]bool{"gate": true}); rc != 1 {
		fails = append(fails, fmt.Sprintf("自检⑥ R2 应 rc=1，实测 %d", rc))
	}
	// ⑦ 负控 R3：件名 ≠ round_id ⇒ 1
	n4 := mkdir("n4")
	mk(n4, "R-错的.json", good("R-20260920T000005", nil))
	if rc, _ := adeCheckDir(n4, map[string]bool{"gate": true}); rc != 1 {
		fails = append(fails, fmt.Sprintf("自检⑦ R3 应 rc=1，实测 %d", rc))
	}
	// ⑧ 负控 R4：点名的命令面调用在命令树里解析不到 ⇒ 1
	n5 := mkdir("n5")
	mk(n5, "R-20260920T000006.json", good("R-20260920T000006", map[string]any{"evidence": []any{"zerg 绝无此命令"}}))
	if rc, _ := adeCheckDir(n5, map[string]bool{"gate": true}); rc != 1 {
		fails = append(fails, fmt.Sprintf("自检⑧ R4 应 rc=1（幽灵调用），实测 %d", rc))
	}

	if len(fails) > 0 {
		fmt.Fprintf(stdout, "门⑬ 自检失败 %d 条：\n", len(fails))
		for _, f := range fails {
			fmt.Fprintln(stdout, "  ✗ "+f)
		}
		return 1
	}
	fmt.Fprintln(stdout, "门⑬ 自检全过：1 正控 + 5 负控 + 2 空转（成对负控）")
	return 0
}

// ── 入口（= py `main()`）────────────────────────────────────────────────────

func aidirecteditsGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后 rc=2（先于一切，含 argparse/--self-test）。
	if rc, ok := adeRefuseJSON(stderr, argv); ok {
		return rc
	}
	// ② argparse：`-h/--help` 在任何位置出现即出帮助并 rc=0（含「未知旗标在前」也照样先出帮助 —— 现读复刻）。
	for _, a := range argv {
		if a == "-h" || a == "--help" {
			fmt.Fprint(stdout, adeHelp)
			return 0
		}
	}
	// ③ argparse：选项解析（`--root` / `--dir` / `--self-test`），未识别项收集后报错（rc=2）。
	root := adeDefaultRoot()
	dirArg := ""
	selfTest := false
	var unrecognized []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--root":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, adeUsageLine)
				fmt.Fprintf(stderr, "%s: error: argument --root: expected one argument\n", adeProg)
				return 2
			}
			i++
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		case a == "--dir":
			if i+1 >= len(argv) {
				fmt.Fprint(stderr, adeUsageLine)
				fmt.Fprintf(stderr, "%s: error: argument --dir: expected one argument\n", adeProg)
				return 2
			}
			i++
			dirArg = argv[i]
		case strings.HasPrefix(a, "--dir="):
			dirArg = a[len("--dir="):]
		case a == "--self-test":
			selfTest = true
		default:
			unrecognized = append(unrecognized, a)
		}
	}
	if len(unrecognized) > 0 {
		fmt.Fprint(stderr, adeUsageLine)
		fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", adeProg, strings.Join(unrecognized, " "))
		return 2
	}
	// ④ `--self-test` 存在即走自检（py：`if a.self_test: return self_test(a.root)`）。
	if selfTest {
		return adeSelfTest(stdout)
	}
	d := dirArg
	if d == "" {
		d = adeDefaultDir()
	}
	tree, terr := adeLoadCommandTree(root)
	if terr != "" {
		fmt.Fprintf(stdout, "⛔ 不给结论：%s\n", terr)
		return 2
	}
	rc, lines := adeCheckDir(d, tree)
	fmt.Fprintf(stdout, "门⑬ AI 直改纪律（§20.3 H1 · 回执 %s）· 仓根 %s\n", d, root)
	for _, ln := range lines {
		fmt.Fprintf(stdout, "  %s\n", ln)
	}
	switch rc {
	case 0:
		fmt.Fprint(stdout, "⇒ 零直改纪律全过（rc=0）\n")
	case 1:
		fmt.Fprint(stdout, "⇒ 有红项（rc=1）：上面逐条点名\n")
	default:
		fmt.Fprint(stdout, "⇒ 不给结论（rc=2）\n")
	}
	return rc
}

// adeRunForTest —— 进程内跑入口取 (rc, stdout+stderr)（`--self-test` 复刻 py 的 subprocess 汇总口径）。
// 仅供同批测试件复用；py 侧合流 = `(p.stdout or "") + (p.stderr or "")`。
func adeRunForTest(argv []string) (int, string) {
	var o, e bytes.Buffer
	rc := aidirecteditsGateMain(&o, &e, argv)
	return rc, o.String() + e.String()
}
