// check_wall_layer.go —— 门禁件 `scripts/gates/check-wall-layer.py` 的 **Go 真搬样板**（v2.5.14 · M2-0 路3）。
//
// 为什么有这件：v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//
//	M1 = 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面（转发壳）；M2 = 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据）的**一枚样板**：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**。
//
// 选它的理由（M2-0 = 「无输入物料 · 纯静态检查 · 输出确定有序」；候选池 5 件现读后自定）：
//
//	✓ 选：`check-wall-layer.py`（239 行 · 唯一真源只读**仓内**两处：`core/cmd/zerg/*.go` 与 `wall|agent` 树）
//	· **无外部树依赖**（不碰文档仓 / 公开产物树 / `bin/`）· 输出**定序**（`sorted(os.listdir(...))` +
//	  模块/文件序遍历，无「按 inode 序」的读取）· 退码 `0/1/2` 三档齐 · **纯只读、无写面** · 行数适中 ·
//	  `git status` 现读**不含它**（无人正在改）。
//	✗ 不选 `check-registry-vs-disk.py`（498 行）：判据面**直接读 `scripts/build/publish-public.sh`** ——
//	  那件是**登记面**（本批硬约束①禁碰），且它 import 复用 `check-history-secrets.py` ⇒ 外部件依赖。
//	✗ 不选 `check-publish-layer-names.py`（484 行）：真源 = `publish/mirror-public-lib.py`（**登记面**，禁碰），
//	  且 L4 要**真跑 `bin/zerg` 子进程**（本批硬约束：禁碰 `bin/`）。
//	✗ 不选 `check-lineref-limit.py`（337 行）：落点解析**依赖同级外部树 `Zerg-内部文档/`**（四处落点规则里两处指向它），
//	  且裸件名解析走 `os.walk` 取 `cands[0]`（遍历序 ⇒ 落点路径可非确定）⇒ 不满足「无外部树依赖 + 输出确定有序」。
//	✗ 不选 `check-keep-marks.py`（391 行）：判据面要**输入物料**（案卷 JSON `keep-marks-registry.json`，
//	  `--dossier` 可指向仓外件）⇒ 不属「无输入物料」；且 `--self-test` 里用 `subprocess` 真跑自身。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改任何 `add_step`）；
//	· **不**删 `scripts/gates/check-wall-layer.py` 原件（退场规则：**对拍通过后才**退场，且逐件显式点名）；
//	· 入口 `walllayerGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态：
//	  先证明「搬得逐字相同」，接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func walllayerGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 通过 / 1 门禁红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字）：全绿 · 四条规则各自造 violation（R1 改档 / R1 少档 / R2 space 标记 /
// R3 命令面件 · 含 `wall/**` 整文件 JSON、`wall/*.txt` 行分隔 JSON、`agent/**` 三路 plan 面）· 空转 ⇒ 2 ·
// 取不到输入（根不存在）⇒ 2 · 未知旗标 ⇒ 2 · `--json` / `--json=stdout` 无机器面拒收 ⇒ 2 ·
// `--self-test` 三档 · 真仓现读（显式根 / 缺省根 · cwd=仓根）· ★ 目录内多件（readdir 原序 ≠ 字典序）：单层 + 递归子目录两格，stdout 与 py 逐字节相同（GAP-20261001-53）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助正文**：本件**有** argparse（`root` nargs='?' · `--self-test`）。帮助正文按**现读复刻**
//	   （`-h/--help` 与 `optional arguments:` 那一版逐字抄自现读 py 输出，含 prog 名 `check-wall-layer.py` 硬编码
//	   = 原件基名），**不是**实现级同源 ⇒ 换了 Python 版本（段标题 `options:` / 措辞）就不同。此处**明写**它。
//	   `--json` 拒收文案与「未知旗标」两档同样按现读 py 输出逐字复刻（含 `usage:` 两行）。
//	② 「不给根参数」时的**默认根定位**：py 由 `argparse` 缺省 `'.'` 起、`find_root()` 从 `os.path.abspath(start)`
//	   上溯 ≤8 层找 `core/cmd/zerg/wall.go`（**与 `__file__` 无关**）⇒ 本件同样从 **cwd** 上溯：同真仓 + cwd=仓根
//	   时读数逐字相同（已对拍）；「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 对拍一律显式传根。
//	③ **异常路径**：`core/cmd/zerg` 读不到 / 某 `.go` 非 UTF-8（py `open(...,encoding="utf-8")` 抛
//	   `UnicodeDecodeError`）· `wall|agent` 下 JSON 坏件（py `json.load` 抛 ⇒ 本件 `except` 吞掉继续）——
//	   py 在**前半**一律**未捕获抛 traceback**（进程退码 1）；本件落「读不到」并继续 ⇒ **stderr 正文与 py 的
//	   traceback 不同**（退码同为 1 的只有「读不到 ⇒ 静默 continue」那一半：py 在 `wall|agent` 面**本来就**吞异常）。
//	④ py `--self-test` 用 `tempfile.TemporaryDirectory()` 造真夹具后**进程内**调 `check()`；本件同样进程内
//	   造临时树（`os.MkdirTemp`）复入口取 rc，打印文案逐字对齐 —— 差在**实现手段**（临时根名 / 清理时机），
//	   不在读数；`$TMPDIR` 不可写时 py 抛异常而本件落 rc=2 ⇒ 这一档**不属逐字对齐承诺**。
//	⑤ JSON 解析角落：py `json.loads` 收 `NaN`/`Infinity`/裸 `null`（裸 `null` ⇒ 落行分隔分支），
//	   Go `encoding/json` 收前者三样中的 `null` 但**拒** `NaN`/`Infinity` ⇒ 角落形态**不属逐字对齐承诺**；
//	   本件对拍输入集全用规整 JSON。另：Python `\s`（str 模式）含 Unicode 空白，本件按 ASCII 集
//	   `[ \t\n\r\f\v]` 复刻；★ `os.walk` 的扫序 = readdir **原序**（✗ 非字典序）—— 已用 `os.Open` + `(*os.File).ReadDir(-1)` 复刻（`os.ReadDir` 会字典序排序 ⇒ 已弃），产出序按 `os.walk` 口径**先当前层文件 → 再下钻子目录**，并以「readdir 原序 ≠ 字典序」的仓外夹具对拍**单层 + 递归子目录两格**（stdout 与 py 逐字节相同 · GAP-20261001-53）。
//
//	⑥ **自扰面（本件特有 · 必读）**：本件落在判据**自己的扫描面内**（`core/cmd/zerg/*.go` · 非 `_test.go`）
//	   ⇒ 本件正文里的**字面形态**会改变「真仓现读」的读数（实测：反引号原串里写 `layer:` + 空格 + 引号
//	   会让本门在真仓现读时凭空多出 layer 标记 ⇒ 把**接线中的「门⑥ 茧壁」（阻断档）**判红）。
//	   本件已刻意规避：自检③ 的替换串用**转义双引号**（源码文本里是 `layer:` + 空格 + 反斜杠 + 引号 ⇒
//	   判据正则不命中）；`good` 夹具里的行也用转义形。⇒ 后续动本件者：**别**把这两处改回反引号原串。
//
//	—— ①②③④⑤⑥ 六面均**不在** M2-0 的样板对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ── 判据常量（与 py 逐条对齐 · 名字前缀 `wl` 防包内同名）──────────────
var (
	wlClosed = []string{"host", "node", "space"} // py CLOSED

	wlForbiddenArgv = map[string]bool{ // py FORBIDDEN_ARGV
		"zerg": true, "zerg-core": true, "zerg-agentd": true,
		"zerg-wall": true, "zerg-compat": true, "zerg-model": true,
		"precommit-gates.sh": true,
	}

	// py CONST_RE = re.compile(r'layer(Host|Node|Space)\s*=\s*"([^"]+)"')
	wlConstRE = regexp.MustCompile(`layer(Host|Node|Space)[ \t\n\r\f\v]*=[ \t\n\r\f\v]*"([^"]+)"`)
	// py CMD_LAYER_RE = re.compile(r'layer:\s*"([^"]+)"')
	wlCmdLayerRE = regexp.MustCompile(`layer:[ \t\n\r\f\v]*"([^"]+)"`)

	wlClosedJoined = strings.Join(wlClosed, "/") // py '/'.join(CLOSED)
)

const (
	wlHeaderLine = "── 茧壁边界：layer 闭集 + space 空集 + argv 零命令面（§九 M10 X1/X2/EDGE-NODE）──"
	wlNoRootMsg  = "⛔ 不给结论：找不到 core/cmd/zerg/wall.go（本门只认命令面真源所在的那棵树）"
	wlGreenMsg   = "⇒ 全绿（rc=0）"
	wlRedMsg     = "⇒ 门禁红（rc=1）：茧壁边界被放宽"
	wlInconclMsg = "⇒ 不给结论（rc=2）"
)

// wlJSONFields —— py `JSON_FIELDS`（本门无机器读面 ⇒ 空 · 唯一真源：印表与拒收同读这一处）。
var wlJSONFields = []string{}

// wlPyRepr —— Python `repr()` 对 str 的口径（本件只用到这一档：引号择法与转义）。
//
//	CPython：默认单引号；**含 `'` 且不含 `"`** ⇒ 改用双引号（自检列表里那一格就靠这条）。
func wlPyRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case rune(quote):
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// wlPyListStr —— Python `str(list[str])`（`['a', 'b']` 那一形）。
func wlPyListStr(items []string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, wlPyRepr(it))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// wlPyStr —— Python `str(x)` 对 JSON 值那一档（命令面判定只看 basename ⇒ 只覆盖到能出现的形态）。
func wlPyStr(v interface{}) string {
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
	case json.Number:
		return t.String()
	case float64:
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// wlTitle —— Python `str.title()` 对单段 ASCII 词的效果（`host` ⇒ `Host`）。
func wlTitle(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// wlFindRoot —— py `find_root()`：从 start 的 abspath 上溯 ≤8 层找 `core/cmd/zerg/wall.go`。
func wlFindRoot(start string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for i := 0; i < 8; i++ {
		if st, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg", "wall.go")); err == nil && st.Mode().IsRegular() {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", false
}

// wlJSONParse —— 近似 py `json.loads`（整串一件 · 裸 `null` 按 py 的 `is not None` 语义算「没解出」）。
func wlJSONParse(s string) (interface{}, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, false
	}
	if v == nil { // py: json.loads('null') ⇒ None ⇒ `if whole is not None` 不成立
		return nil, false
	}
	return v, true
}

type wlPlan struct {
	path string
	argv []interface{}
}

// wlWalkPruned —— `os.walk` 的定序替身：DirEntry 序 + 目录剪枝（py 的 `dirs[:] = [...]`）。
func wlWalkPruned(base string, prune map[string]bool, fn func(path string)) {
	// ★ 保 readdir 原序：py `os.walk` 走 `os.scandir`（readdir 原序、**不排序**）⇒ 用
	// `(*os.File).ReadDir(-1)`，**不**用会字典序排序的 `os.ReadDir`（GAP-20261001-53；对拍见件头⑤）。
	f, err := os.Open(base)
	if err != nil {
		return // py: os.walk 对不存在的根**静默**给空序列
	}
	ents, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return
	}
	// ★ 复刻 `os.walk` 产出序：py 先把 scandir 结果分 files/dirs **再** yield ⇒ 口径 =
	// **先当前层文件（entry 序）→ 再按目录 entry 序下钻** ✗ 不能边遍历边下钻（目录夹在中间时会早钻，GAP-20261001-53）。
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		fn(filepath.Join(base, e.Name()))
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if prune[e.Name()] {
			continue
		}
		wlWalkPruned(filepath.Join(base, e.Name()), prune, fn)
	}
}

// wlExtractArgv —— `isinstance(obj, dict) and isinstance(obj.get('argv'), list)`。
func wlExtractArgv(obj interface{}) ([]interface{}, bool) {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return nil, false
	}
	argv, ok := m["argv"].([]interface{})
	if !ok {
		return nil, false
	}
	return argv, true
}

// wlCheck —— py `check(root)`：返回 (rc, lines)。
func wlCheck(root string) (int, []string) {
	var errs []string
	cliDir := filepath.Join(root, "core", "cmd", "zerg")

	var src strings.Builder
	ents, err := os.ReadDir(cliDir)
	if err != nil {
		// py 在 `os.listdir` 上**未捕获抛异常** ⇒ traceback ⇒ 进程 1；本件收作门禁红（见件头③）。
		return 1, []string{"✗ core/cmd/zerg 读不到（" + err.Error() + "）"}
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names) // py sorted(os.listdir(...))

	readFails := []string{} // ★ 登记面：读失败的件累积于此（非空 ⇒ 汇总处判红退 2）
	for _, name := range names {
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			b, err := os.ReadFile(filepath.Join(cliDir, name))
			if err != nil {
				// ★ 读失败不再静默（原 py `except OSError: continue` 口子）：登记后继续
				readFails = append(readFails, filepath.Join(cliDir, name)+"（"+err.Error()+"）")
				continue
			}
			src.Write(b)
		}
	}
	s := src.String()

	// ── R1 闭集 ────────────────────────────────────────────────
	consts := map[string]string{}
	order1 := []string{}
	for _, m := range wlConstRE.FindAllStringSubmatch(s, -1) {
		k := strings.ToLower(m[1])
		if _, dup := consts[k]; !dup {
			order1 = append(order1, k)
		}
		consts[k] = m[2]
	}
	_ = order1
	if len(consts) < 3 {
		return 2, []string{"⛔ 不给结论：层级闭集常量没读全（R1 空转 = 假覆盖）"}
	}
	badKeys := []string{}
	for k, v := range consts {
		if !wlIn(v, wlClosed) {
			badKeys = append(badKeys, k)
		}
	}
	sort.Strings(badKeys) // py sorted(bad_const.items())
	for _, k := range badKeys {
		errs = append(errs, fmt.Sprintf("R1 层级闭集被改：layer%s = %s（闭集只有 %s）",
			wlTitle(k), wlPyRepr(consts[k]), wlClosedJoined))
	}
	if !wlSameStringSet(consts, wlClosed) {
		vals := make([]string, 0, len(consts))
		for _, v := range consts {
			vals = append(vals, v)
		}
		sort.Strings(vals)
		errs = append(errs, fmt.Sprintf("R1 闭集不足三档或多了档：%s", wlPyListStr(vals)))
	}

	// ── R2 space 档恒空 ────────────────────────────────────────
	marks := []string{}
	for _, m := range wlCmdLayerRE.FindAllStringSubmatch(s, -1) {
		marks = append(marks, m[1])
	}
	if len(marks) == 0 && s == "" {
		return 2, []string{"⛔ 不给结论：命令树里一个 layer 标记都没读到（空转 = 假覆盖）"}
	}
	spaceMarks := []string{}
	for _, m := range marks {
		if m == "space" {
			spaceMarks = append(spaceMarks, m)
		}
	}
	for range spaceMarks {
		errs = append(errs, "R2 命令树里出现 `layer: \"space\"` —— 空间内**零命令面**（X2/P-048 取空集）")
	}
	for _, m := range marks {
		if !wlIn(m, wlClosed) {
			errs = append(errs, fmt.Sprintf("R1/R2 命令树里出现闭集外的层级 %s", wlPyRepr(m)))
		}
	}

	// ── R3 plan 产物的 argv ────────────────────────────────────
	plans := []wlPlan{}
	// wall/**（剪 `target`）· 认 `.txt` 与 `.json` · 整文件 JSON 或**一行一条 JSON**
	wlWalkPruned(filepath.Join(root, "wall"), map[string]bool{"target": true}, func(p string) {
		if !strings.HasSuffix(p, ".txt") && !strings.HasSuffix(p, ".json") {
			return
		}
		b, err := os.ReadFile(p)
		if err != nil {
			// ★ 读失败不再静默（原 py `except OSError: continue` 口子）：登记后继续
			readFails = append(readFails, p+"（"+err.Error()+"）")
			return
		}
		txt := string(b)
		var cands []interface{}
		if whole, ok := wlJSONParse(txt); ok {
			cands = append(cands, whole)
		} else {
			for _, line := range strings.Split(txt, "\n") {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "{") {
					continue
				}
				if obj, ok := wlJSONParse(line); ok {
					cands = append(cands, obj)
				}
			}
		}
		for _, obj := range cands {
			if argv, ok := wlExtractArgv(obj); ok {
				plans = append(plans, wlPlan{p, argv})
			}
		}
	})
	// agent/**（剪 `target` / `testdata`）· 只认 `.json` 整件
	wlWalkPruned(filepath.Join(root, "agent"), map[string]bool{"target": true, "testdata": true}, func(p string) {
		if !strings.HasSuffix(p, ".json") {
			return
		}
		b, err := os.ReadFile(p)
		if err != nil {
			// ★ 读失败不再静默（原 py `except OSError: continue` 口子）：登记后继续
			readFails = append(readFails, p+"（"+err.Error()+"）")
			return
		}
		if obj, ok := wlJSONParse(string(b)); ok {
			if argv, ok2 := wlExtractArgv(obj); ok2 {
				plans = append(plans, wlPlan{p, argv})
			}
		}
	})
	if len(plans) == 0 {
		return 2, []string{"⛔ 不给结论：一份带 `argv` 的 plan 产物都没找到（R3 空转 = 假覆盖）"}
	}
	scanned := 0
	for _, pl := range plans {
		scanned++
		rel, err := filepath.Rel(root, pl.path)
		if err != nil {
			rel = pl.path
		}
		for _, tok := range pl.argv {
			baseTok := filepath.Base(wlPyStr(tok))
			if wlForbiddenArgv[baseTok] {
				errs = append(errs, fmt.Sprintf("R3 %s 的 argv 里出现命令面件 %s", rel, wlPyRepr(baseTok)))
			}
		}
	}

	vals := make([]string, 0, len(consts))
	for _, v := range consts {
		vals = append(vals, v)
	}
	sort.Strings(vals)
	lines := []string{
		fmt.Sprintf("  层级闭集 = %s（三档）", strings.Join(vals, "/")),
		fmt.Sprintf("  命令树 layer 标记 %d 处 · 其中 space = %d（须为 0）", len(marks), len(spaceMarks)),
		fmt.Sprintf("  扫到 plan 产物 %d 份（argv 均无命令面件）", scanned),
	}
	if len(readFails) > 0 {
		// ★ 汇总处判红：覆盖面不完整 ⇒ 不给结论（原 py 在 wall|agent 面**静默吞掉**读失败）。
		//   本函无 stderr 形参（入口 `walllayerGateMain` 持 stderr）⇒ 沿用门内口径：BLOCKED 头 + 逐件点名
		//   随 `lines` 回入口打 stdout（与门内其余 rc=2 档同路）。
		sort.Strings(readFails)
		blk := []string{"⛔ BLOCKED：以下件读失败，覆盖面不完整 —— 不给结论（假覆盖）"}
		for _, p := range readFails {
			blk = append(blk, "   · "+p)
		}
		return 2, append(lines, blk...)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			lines = append(lines, "✗ "+e)
		}
		return 1, lines
	}
	return 0, lines
}

func wlIn(s string, set []string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

// wlSameStringSet —— py `set(consts.values()) != set(CLOSED)`。
func wlSameStringSet(m map[string]string, closed []string) bool {
	got := map[string]bool{}
	for _, v := range m {
		got[v] = true
	}
	want := map[string]bool{}
	for _, v := range closed {
		want[v] = true
	}
	if len(got) != len(want) {
		return false
	}
	for k := range got {
		if !want[k] {
			return false
		}
	}
	return true
}

// ── 自检（成对负控 5 格 · 与 py `self_test()` 逐格对齐）──────────────────
func wlSelfTest(stdout io.Writer) int {
	ok := true
	fixture := func(src string, plan string) string {
		d, err := os.MkdirTemp("", "zerg-wall-selftest-")
		if err != nil {
			return ""
		}
		cli := filepath.Join(d, "core", "cmd", "zerg")
		if err := os.MkdirAll(cli, 0o755); err != nil {
			return ""
		}
		if err := os.WriteFile(filepath.Join(cli, "wall.go"), []byte(src), 0o644); err != nil {
			return ""
		}
		if plan != "" {
			wdir := filepath.Join(d, "wall")
			if err := os.MkdirAll(wdir, 0o755); err != nil {
				return ""
			}
			if err := os.WriteFile(filepath.Join(wdir, "plan.json"), []byte(plan), 0o644); err != nil {
				return ""
			}
		}
		return d
	}

	good := "const (\n\tlayerHost  = \"host\"\n\tlayerNode  = \"node\"\n" +
		"\tlayerSpace = \"space\"\n)\n" +
		"var c = command{layer: \"host\"}\n"

	// 自检① 好件
	t1 := fixture(good, `{"argv": ["--ro-bind", "/usr", "/usr", "zerg-egg"]}`)
	rc1, _ := wlCheck(t1)
	os.RemoveAll(t1)
	fmt.Fprintf(stdout, "  自检① 好件（闭集齐 · space 空 · argv 只有 zerg-egg 豁免）⇒ rc=%d（要 0）\n", rc1)
	ok = ok && rc1 == 0

	// 自检② 自造第四档
	// ★ 同上：用转义双引号串（源码文本里 `:`/`=` 后是反斜杠）⇒ 判据的常量正则亦不命中。
	bad2 := strings.Replace(good, "layerNode  = \"node\"", "layerNode  = \"target\"", 1)
	t2 := fixture(bad2, `{"argv": ["--ro-bind", "/usr", "/usr"]}`)
	rc2, lines2 := wlCheck(t2)
	os.RemoveAll(t2)
	fmt.Fprintf(stdout, "  自检② 自造第四档 ⇒ rc=%d（要 1）· %s\n", rc2, wlPyListStr(wlFilterR1(lines2)))
	ok = ok && rc2 == 1

	// 自检③ 命令树里出现 space 档
	// ★ 这一行**刻意**不用反引号原串：本件落在判据自己的扫描面内（`core/cmd/zerg/*.go` 非 `_test.go`），
	// 原串会把「命令树标记」那一形逐字写进本件正文 ⇒ 让「门⑥ 茧壁」在真仓现读时多出 layer 标记（自扰）。
	// 转义双引号串在源码文本里是 `layer:` + 空格 + 反斜杠 + 引号 ⇒ 判据正则不命中；字面值逐字不变。
	bad3 := strings.Replace(good, "layer: \"host\"", "layer: \"space\"", 1)
	t3 := fixture(bad3, `{"argv": ["--ro-bind", "/usr", "/usr"]}`)
	rc3, lines3 := wlCheck(t3)
	os.RemoveAll(t3)
	fmt.Fprintf(stdout, "  自检③ 命令树里出现 space 档 ⇒ rc=%d（要 1）· %s\n", rc3, wlPyListStr(wlFilterR2(lines3)))
	ok = ok && rc3 == 1

	// 自检④ argv 里混进命令面件
	t4 := fixture(good, `{"argv": ["/bin/sh", "-c", "exec \"$@\"", "zerg-core"]}`)
	rc4, lines4 := wlCheck(t4)
	os.RemoveAll(t4)
	fmt.Fprintf(stdout, "  自检④ argv 里混进命令面件 ⇒ rc=%d（要 1）· %s\n", rc4, wlPyListStr(wlFilterR3(lines4)))
	ok = ok && rc4 == 1

	// 自检⑤ 没有 plan 产物 ⇒ 空转
	t5 := fixture(good, "")
	rc5, _ := wlCheck(t5)
	os.RemoveAll(t5)
	fmt.Fprintf(stdout, "  自检⑤ 没有 plan 产物 ⇒ rc=%d（要 2 —— 空转不给结论）\n", rc5)
	ok = ok && rc5 == 2

	if ok {
		fmt.Fprint(stdout, "自检结论: 全过\n")
		return 0
	}
	fmt.Fprint(stdout, "自检结论: **有断言失败**\n")
	return 1
}

func wlFilterR1(lines []string) []string { return wlFilterHas(lines, "R1") }
func wlFilterR2(lines []string) []string { return wlFilterHas(lines, "R2") }
func wlFilterR3(lines []string) []string { return wlFilterHas(lines, "R3") }

func wlFilterHas(lines []string, sub string) []string {
	out := []string{}
	for _, l := range lines {
		if strings.Contains(l, sub) {
			out = append(out, l)
		}
	}
	return out
}

// ── Q-176：`--json` 而本门**无机器读面** ⇒ 逐字说明后 rc=2（先于一切，含 `--self-test`）──────
func wlRefuseJSON(stderr io.Writer, argv []string) (int, bool) {
	hit := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return 0, false
	}
	quoted := make([]string, 0, len(hit))
	for _, h := range hit {
		quoted = append(quoted, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	if len(wlJSONFields) > 0 {
		fs := make([]string, 0, len(wlJSONFields))
		for _, f := range wlJSONFields {
			fs = append(fs, "`"+f+"`")
		}
		fmt.Fprintf(stderr, "   字段表：%s\n", strings.Join(fs, " · "))
	} else {
		fmt.Fprint(stderr, "   字段表：无（本门只出人读面）\n")
	}
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2, true
}

// ── argparse 面（按**现读** py 输出逐字复刻 · 见件头未覆盖面①）────────────────────
const wlProg = "check-wall-layer.py"

var wlUsageLine = "usage: " + wlProg + " [-h] [--self-test] [root]\n"

const wlHelpBody = `
positional arguments:
  root

optional arguments:
  -h, --help   show this help message and exit
  --self-test
`

// walllayerGateMain —— 入口（= py `main()`）。
func walllayerGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：`--json` 拒收（先于一切）
	if rc, hit := wlRefuseJSON(stderr, argv); hit {
		return rc
	}
	// ② argparse 面：`-h/--help` · `--self-test` · 未知旗标 / 多余位置参数
	selfTest := false
	var positionals []string
	var unrecognized []string
	for _, a := range argv {
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, wlUsageLine+wlHelpBody)
			return 0
		case a == "--self-test":
			selfTest = true
		case a == "--":
			// argparse 的 `--` 之后一律算位置参数
			continue
		case strings.HasPrefix(a, "-") && a != "-":
			unrecognized = append(unrecognized, a)
		default:
			positionals = append(positionals, a)
		}
	}
	if len(positionals) > 1 {
		unrecognized = append(unrecognized, positionals[1:]...)
	}
	if len(unrecognized) > 0 {
		fmt.Fprint(stderr, wlUsageLine)
		fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", wlProg, strings.Join(unrecognized, " "))
		return 2
	}
	if selfTest {
		return wlSelfTest(stdout)
	}
	root := "."
	if len(positionals) == 1 {
		root = positionals[0]
	}
	r, ok := wlFindRoot(root)
	if !ok {
		fmt.Fprintln(stdout, wlNoRootMsg)
		return 2
	}
	rc, lines := wlCheck(r)
	fmt.Fprintln(stdout, wlHeaderLine)
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
	switch rc {
	case 0:
		fmt.Fprintln(stdout, wlGreenMsg)
	case 1:
		fmt.Fprintln(stdout, wlRedMsg)
	default:
		fmt.Fprintln(stdout, wlInconclMsg)
	}
	return rc
}
