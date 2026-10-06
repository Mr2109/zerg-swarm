// check_dev_targets.go —— 判据件 `scripts/gates/check-dev-targets.py`（112 行）的 **Go 真搬**
// （v2.5.14 · M2 · 搬运面 批-3 第 4 粒 · 入口 `checkDevTargetsGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**。
//
// 选它的理由（真未搬清单里**行数最小的一批** · 纯静态无外部树依赖）：
//
//	✓ 选：判据是**纯 JSON 面** —— 只读一件 `core/internal/contract/dev-targets.json`（缺省即 cwd
//	  相对路径），断言 `set(ids) == set(families 展平)` 且 `ids == 展平序`。
//	  **不真跑子进程、不碰网络、零计时、零随机**（`--self-test` 只在自建临时目录里造件，见下）。
//	· 现读 import 面：仅 `json` / `sys`（`tempfile` / `os` 只在 `self_test()` 内）—— **零 `argparse` /
//	  `subprocess` / `time` / `datetime` / `random` / `socket` / `urllib`**。⇒ 无 argparse 帮助面差异。
//	· `tempfile.TemporaryDirectory` 本身**自带回收**（`with` 形）⇒ 非 A 类泄漏形态（逐字保留其自回收语义；
//	  Go 侧 `os.MkdirTemp` + 建点成功支的 `defer os.RemoveAll`）。
//	· 退码 0/1/2 三档齐 · 默认档**纯只读无写面** · `git status` 现读不含它（无人正在改）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 add_step 表）；
//	· **不**删 `scripts/gates/check-dev-targets.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `checkDevTargetsGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件 / `gate_inproc_table.go`。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkDevTargetsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 一致 / 1 不一致（含顶层非对象崩溃档）/ 2 不给结论（缺件 · 解析失败 · 形状不符 · 空转）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「未覆盖面」）
//
// ═══════════════════ ★ argv 解析面（逐点交代 · 本件 py 无 argparse）═══════════════════
//
//	py：`if "--self-test" in args: return self_test()`（**先于过滤**）；再 `args = [a for a in args if not
//	a.startswith("--")]` ⇒ **一切 `--` 前缀项被静默丢弃**（`--json` / `--help` / 任意 `--xxx` 皆被忽略、
//	不是错误）；`path = args[0] if args else DEFAULT` ⇒ 单破折号项（如 `-h`）**不被过滤** ⇒ 会被当成
//	**路径**（`-h` ⇒ 读不到该名件 ⇒ rc=2「缺件」）。Go 逐字复刻（`strings.HasPrefix(a, "--")`）。
//
// ═══════════════════ ★ 默认根定位（声明 (a)）═══════════════════
//
//	py **不用 `__file__`**：缺省路径 = 字面 `"core/internal/contract/dev-targets.json"`（**cwd 相对**）。
//	Go 无 `__file__` 语境，但本件 py 本就起于 cwd ⇒ 两者**逐字同构**（✗ 无「脚本位置 ≠ cwd」的推导差）。
//
// ═══════════════════ ★ 声明 (b) 自指检查 —— 本件**不在**自己的扫描面内（逐形态）═══════════════════
//
//	① 扫描面 = `<cwd>/core/internal/contract/dev-targets.json` **单件** ⇒ 本件（`core/cmd/zerg/*.go`）
//	   不在内 ⇒ 自指 = 假。
//	② 件头/正文**零真引号路径原串**落进该 JSON 的扫描输出（本件只把该路径当**输入**读）。
//	③ self-test 造的合成件落点 = **自建临时目录**，不进仓树 ⇒ 真仓 `--self-test` 不受影响。
//	④ 是否落在**他门**扫描面内：本件在 `core/cmd/zerg/*.go` ⇒ 落 `check-error-kinds` 的 R3 面；R3 只认
//	   「错误构造调用的首参字面」（**带左括号**的调用式），**✗ 不扫 JSON 信封的 kind 键** ⇒ 本件不写该
//	   调用式 ⇒ 命中 0。⚠ 亦落 `check-hardcoded-private-paths`（其 SCAN_DIRS 含 `core/`）⇒ 本件**不写任何
//	   私有绝对路径的首段字面**（此处按**描述式**写，✗ 不把原串当例子写出来）⇒ 命中 0。
//
// ═══════════════════ ★ 未覆盖面（✗ 不声称逐字对齐）═══════════════════
//
//	① **异常正文**：py `except Exception as e: "解析失败：%s（%s）"` 里那个 `%s` 是 **CPython 异常 str** ——
//	   对非法 JSON 是 `json.JSONDecodeError` 的 `"Expecting value: line 1 column 1 (char 0)"` 一类，对本件
//	   Go 侧的 `encoding/json` 永远是另一句话 ⇒ **该档只保 rc=2 与「解析失败」骨架，正文不同**。
//	② **顶层非对象 ⇒ 崩溃档**（真口子 · C 类最外层无兜底）：py 的 `d.get(...)` 落在 `try` **之外** ⇒ 顶层
//	   为标量/串/数组时 `AttributeError` **未捕** ⇒ **回溯 + rc=1**（并非「不给结论 rc=2」）；且崩溃发生在
//	   任何 `print()` 之前 ⇒ **py stdout 为空**。本件**同退码 rc=1、stdout 同为空**，stderr 走**声明文案**
//	   （✗ 非 py 回溯正文 —— 回溯含本机 py 安装路径 / 行号，不可移植）。★ 此口子**如实交回入账**（✗ 不改 py 原件）。
//	③ **元素非字符串 / 混合类型**：py `sorted()` / `set()` 遇不可比较或不可哈希元素 ⇒ **未捕异常**（回溯 ·
//	   rc=1）；本件按 `cdtPyStr` 尽力转串 ⇒ 该档**不属逐字对齐**。
//	④ **`families[fam]` 非数组**（数 / 对象 / null）：py 迭代抛未捕异常或走 dict key 序（不可逐字）；本件
//	   **只覆盖数组**（串值按**码点**迭代）⇒ 其余档未覆盖。
//	⑤ **JSON 数字 NaN / Infinity**：py `json.load` 收（得 float('nan')），Go `encoding/json` **拒**。
//	⑥ `--json` **无机器面**：py 本就把 `--json` 当**未知前缀项静默丢弃**（非机器读面）⇒ 本件同样丢弃 ⇒
//	   该档**无 JSON 键表可核**（空值三形态不适用）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// cdtFamilyOrder —— py `FAMILY_ORDER`（固定族序 · 展平按它）。
var cdtFamilyOrder = []string{"D", "E", "F", "G"}

// cdtDefaultPath —— py main 里的缺省路径字面（cwd 相对 · ✗ 非 `__file__` 推导）。
const cdtDefaultPath = "core/internal/contract/dev-targets.json"

// checkDevTargetsGateMain —— 入口（= py `main()`）。
func checkDevTargetsGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① `--self-test` 存在即走自检（py：先于过滤判 `"--self-test" in args`）。
	for _, a := range argv {
		if a == "--self-test" {
			return cdtSelfTest(stdout)
		}
	}
	// ② 过滤掉一切 `--` 前缀项（py：`[a for a in args if not a.startswith("--")]`）。
	var filtered []string
	for _, a := range argv {
		if !strings.HasPrefix(a, "--") {
			filtered = append(filtered, a)
		}
	}
	path := cdtDefaultPath
	if len(filtered) > 0 {
		path = filtered[0]
	}
	rc, problems, crashed := cdtCheck(path)
	if crashed {
		// py：`d.get` 未捕 → 回溯 rc=1，且崩溃于任何 print 之前 ⇒ stdout 为空。本件同退码、stdout 同空。
		fmt.Fprintf(stderr, "✗ %s：顶层不是 JSON 对象 ⇒ py 侧 d.get 未捕异常（回溯 · rc=1）；本件同退码 rc=1（stderr 正文另论 · 见件头未覆盖面②）\n", path)
		return 1
	}
	fmt.Fprintln(stdout, "── 门 · dev-targets 静态断言（GAP-20260927-197）：set(ids)==set(展平) 且 ids==展平序 ──")
	for _, p := range problems {
		fmt.Fprintf(stdout, "  · %s\n", p)
	}
	switch rc {
	case 0:
		fmt.Fprintln(stdout, "⇒ 全绿（rc=0）：ids 与 families 展平集合相等且同序")
	case 1:
		fmt.Fprintln(stdout, "⇒ 门禁红（rc=1）：ids 与 families 展平不一致")
	default:
		fmt.Fprintln(stdout, "⇒ 不给结论（rc=2）")
	}
	return rc
}

// cdtCheck —— py `check(path)`：⇒ (rc, problems, crashed)。crashed=true ⇔ py 侧未捕异常（顶层非对象）。
func cdtCheck(path string) (int, []string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 2, []string{fmt.Sprintf("缺件：%s 不存在（判据不可判 ⇒ 不给结论）", path)}, false
		}
		// py：open 成功但读失败（目录 / 权限 / 编码）⇒ 走 `except Exception` ⇒ rc=2（正文另论 · 见未覆盖面①）。
		return 2, []string{fmt.Sprintf("解析失败：%s（%s）⇒ 不给结论", path, err.Error())}, false
	}
	var top any
	if err := json.Unmarshal(b, &top); err != nil {
		// py：json.load 抛 JSONDecodeError ⇒ `except Exception` ⇒ rc=2「解析失败」（正文另论 · 见未覆盖面①）。
		return 2, []string{fmt.Sprintf("解析失败：%s（%s）⇒ 不给结论", path, err.Error())}, false
	}
	m, ok := top.(map[string]any)
	if !ok {
		// py：`d.get(...)` 在 try 之外 ⇒ AttributeError 未捕 ⇒ 回溯 rc=1（见未覆盖面②）。
		return 1, nil, true
	}
	idsV := m["ids"]
	famV := m["families"]
	idsArr, idsIsList := idsV.([]any)
	famMap, famIsDict := famV.(map[string]any)
	if !idsIsList || !famIsDict {
		return 2, []string{"形状不符：缺 ids(list) 或 families(dict) ⇒ 不给结论"}, false
	}
	if len(idsArr) == 0 {
		return 2, []string{"空转：ids 为空 ⇒ 不给结论"}, false
	}
	ids := make([]string, 0, len(idsArr))
	for _, e := range idsArr {
		ids = append(ids, cdtPyStr(e))
	}
	flat := cdtFlatten(famMap)

	var problems []string
	if !cdtSortedEqual(ids, flat) {
		onlyIDs := cdtSetDiff(ids, flat)
		onlyFlat := cdtSetDiff(flat, ids)
		problems = append(problems, fmt.Sprintf("FAIL 集合不等：仅 ids 有 %s · 仅 families 展平有 %s",
			cdtPyListRepr(onlyIDs), cdtPyListRepr(onlyFlat)))
	}
	if !cdtSliceEqual(ids, flat) {
		n := len(ids)
		if len(flat) < n {
			n = len(flat)
		}
		idx := n
		for i := 0; i < n; i++ {
			if ids[i] != flat[i] {
				idx = i
				break
			}
		}
		aStr, bStr := "None", "None"
		if idx < len(ids) {
			aStr = cdtPyReprStr(ids[idx])
		}
		if idx < len(flat) {
			bStr = cdtPyReprStr(flat[idx])
		}
		problems = append(problems, fmt.Sprintf("FAIL 顺序不等：第 %d 位 ids=%s 展平=%s（len ids=%d 展平=%d）",
			idx+1, aStr, bStr, len(ids), len(flat)))
	}
	if len(problems) == 0 {
		return 0, nil, false
	}
	return 1, problems, false
}

// cdtFlatten —— py `flatten(families)`：按 cdtFamilyOrder 逐族展平（缺键 ⇒ 空）。
func cdtFlatten(families map[string]any) []string {
	out := []string{}
	for _, fam := range cdtFamilyOrder {
		v, ok := families[fam]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				out = append(out, cdtPyStr(e))
			}
		case string:
			// py：families.get(fam, []) 给的是原值，随后被迭代 ⇒ 串按**码点**迭代（见未覆盖面④）。
			for _, r := range x {
				out = append(out, string(r))
			}
		default:
			// 数 / 对象 / null：py 迭代抛未捕异常或走 map key 序（不可逐字）⇒ 略过（见未覆盖面④）。
		}
	}
	return out
}

// cdtSortedEqual —— py `sorted(ids) == sorted(flat)`（码点序 · 同构 == 逐元素等）。
func cdtSortedEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ca := append([]string(nil), a...)
	cb := append([]string(nil), b...)
	sort.Strings(ca)
	sort.Strings(cb)
	return cdtSliceEqual(ca, cb)
}

// cdtSliceEqual —— py 列表 `==`（逐元素等 · 长度等）。
func cdtSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// cdtSetDiff —— py `sorted(set(a) - set(b))`（去重 + 码点序）。
func cdtSetDiff(a, b []string) []string {
	bs := make(map[string]bool, len(b))
	for _, x := range b {
		bs[x] = true
	}
	seen := map[string]bool{}
	out := []string{}
	for _, x := range a {
		if !bs[x] && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

// cdtPyStr —— py `str(v)` 的常用档（域内元素恒为串 ⇒ 恒同构；非串档见未覆盖面③）。
func cdtPyStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case float64:
		return cdtPyFloatStr(x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// cdtPyFloatStr —— py `str(float)` 的近似（整数 float 作 int 形；其余按最短 round-trip）。
func cdtPyFloatStr(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

// cdtPyReprStr —— py `repr(str)`（引号选择 + 转义 · 复刻 CPython 常用档）。
func cdtPyReprStr(s string) string {
	hasS := strings.Contains(s, "'")
	hasD := strings.Contains(s, "\"")
	quote := byte('\'')
	if hasS && !hasD {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString("\\\\")
		case byte(r) == quote && r < 0x80:
			b.WriteByte('\\')
			b.WriteByte(quote)
		case r == '\n':
			b.WriteString("\\n")
		case r == '\r':
			b.WriteString("\\r")
		case r == '\t':
			b.WriteString("\\t")
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, "\\x%02x", r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// cdtPyListRepr —— py `str(list)`（`['a', 'b']` · 空表 `[]`）。
func cdtPyListRepr(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = cdtPyReprStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cdtSelfTest —— py `self_test()`：四合成档 + 缺件档（全过 ⇒ rc=0 · 不过 ⇒ rc=1）。
// 合成件落在**自建临时目录**（建点成功支 `defer os.RemoveAll` 自回收 ⇒ 跑后计数 0）。
func cdtSelfTest(stdout io.Writer) int {
	ok := true
	type cdtCase struct {
		name string
		want int
		body string
	}
	cases := []cdtCase{
		{"正控 好件（集合与序均一致）⇒ rc=0", 0,
			`{"ids":["D1","D2","E1"],"families":{"D":["D1","D2"],"E":["E1"],"F":[],"G":[]}}`},
		{"负控 集合不等（families 漏 D2）⇒ rc=1", 1,
			`{"ids":["D1","D2","E1"],"families":{"D":["D1"],"E":["E1"]}}`},
		{"负控 顺序不等（ids 换序）⇒ rc=1", 1,
			`{"ids":["D2","D1","E1"],"families":{"D":["D1","D2"],"E":["E1"]}}`},
		{"负控 空 ids ⇒ rc=2", 2,
			`{"ids":[],"families":{"D":[]}}`},
	}
	tmp, err := os.MkdirTemp("", "devt-selftest-")
	if err != nil {
		tmp = os.TempDir()
	} else {
		defer os.RemoveAll(tmp)
	}
	for _, c := range cases {
		// py：`json.dump(body, f, ensure_ascii=False)` ⇒ 直接落原串等价（本文档为 ASCII）。
		p := tmp + string(os.PathSeparator) + "dev-targets.json"
		_ = os.WriteFile(p, []byte(c.body), 0o644)
		rc, probs, _ := cdtCheck(p)
		flag := "✓"
		if rc != c.want {
			flag = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s（rc=%d）\n", flag, c.name, rc)
		if rc != c.want {
			ok = false
			for _, x := range probs {
				fmt.Fprintf(stdout, "      · %s\n", x)
			}
		}
	}
	rc, _, _ := cdtCheck("/nonexistent/nope.json")
	flag := "✓"
	if rc != 2 {
		flag = "✗"
	}
	fmt.Fprintf(stdout, "  %s 负控 缺件 ⇒ rc=2（rc=%d）\n", flag, rc)
	if rc != 2 {
		ok = false
	}
	verdict := "全过"
	if !ok {
		verdict = "**有断言失败**"
	}
	fmt.Fprintln(stdout, "自检结论: "+verdict)
	if ok {
		return 0
	}
	return 1
}
