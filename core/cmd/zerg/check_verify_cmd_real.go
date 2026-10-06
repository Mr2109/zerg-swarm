// check_verify_cmd_real.go —— 判据件 `scripts/gates/check-verify-cmd-real.py`（510 行）的 **Go 真搬**
// （v2.5.14 · M2 · 第三条战线「搬运面」· 入口 `checkVerifyCmdRealGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值真搬 Go（逐批判据搬 Go · 落点 core/cmd/zerg/）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），stdout / stderr / 退码三面逐字相同。
//
// 形态判定（现读 py 原件全 510 行 · import 面 = argparse/atexit/json/os/shutil/sys/tempfile）：
//
//	✓ **纯静态只读** —— 只读两件真源：命令树机器面 `core/cmd/zerg/testdata/cli-matrix.json`
//	  与缺口账 `<状态目录>/zerg-cli-gaps.jsonl`；判定全是纯函数（闭集比较 / 前缀比较）。
//	  零 `subprocess` / 零 `socket` / 零 `urllib` / 零 `time` / 零 `datetime` / 零 `random`。
//	· **默认档不写盘**：`--self-test` 在**系统临时目录**里造合成夹具（`mkdtemp`），跑完回收
//	  （py 走 `atexit` 回收 · Go 走 `defer`）⇒ Go 侧把 `defer os.RemoveAll` 钉在**建点成功支**。
//	· 退码 0/1/2 三档齐 · 无外部树（`--root/--ledger/--matrix` 三旋钮显式入参）。
//	· `git status` 现读不含它（无人正在改）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 core/cmd/zerg/main.go 注册命令（✗ 不进命令树 · ✗ 不改 add_step 步骤表）；
//	· **不**删 scripts/gates/check-verify-cmd-real.py 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `checkVerifyCmdRealGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 family_*.go / 已落旧件 / gate_inproc_table.go。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkVerifyCmdRealGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 报告档完成 / 1（仅 --strict 超额）/ 2 不给结论
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（argparse 帮助/退错正文按参考解释器逐字钉版）
//
// ═══════════════════ ★ argv 解析面（argparse 复刻）═══════════════════
//
//	py 走 `argparse.ArgumentParser(add_help=True, description=…)`，注册 `--root`（取值 · 缺省 =
//	REPO_ROOT_DEFAULT，即脚本上溯两级）· `--ledger`（取值 · 缺省 ""）· `--matrix`（取值 · 缺省 ""）·
//	`--json`/`--self-test`/`--no-self-test`/`--baseline`/`--list-rules`/`--strict`（store_true）·
//	`--max-examples`（type=int · 缺省 12）。Go 侧复刻：`-h/--help` · 长旗标**前缀消歧** ·
//	`--X=V` 与 `--X V` 两形 · 缺值报话 · 未知旗标（`unrecognized arguments: …`）· 歧义报话
//	（`ambiguous option: --s could match --self-test, --strict`）· `--` 分隔符（其后全算位置参）。
//
// ═══════════════════ ★ 默认根定位（声明 (a)）═══════════════════
//
//	py `REPO_ROOT_DEFAULT = dirname(dirname(HERE))`（`__file__` 上溯两级）；Go 无 `__file__` 语境
//	（未来接线后是 CLI 子命令）⇒ 本件默认根回落 **当前工作目录**。落在同一真仓且 cwd=仓根时两者读数
//	逐字相同；「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 该面**不属逐字对齐承诺**，对拍一律显式传根。
//
// ═══════════════════ ★ 声明 (b) 自指检查（真判 + 前瞻）═══════════════════
//
//	本件落在三道他门扫描面内（一旦被 git 跟踪即进面）：check-error-kinds（扫 core/cmd/zerg/*.go 的
//	**代码位** R3：错误构造调用的首参字面 —— 本件**不写**该调用式 ⇒ 命中 0）· check-hardcoded-private-paths
//	（扫 core/ 的私有绝对路径首段字面 —— 本件**零**私有绝对路径，一律描述式 ⇒ 命中 0）· check-wall-layer
//	（扫 `layerXxx = \"…\"` 常量与 `layer: \"…\"` 命令标记 —— 本件**不写**该形态 ⇒ 命中 0）。
//	本件自身不扫 core/ ⇒ 无自扫描面。self-test 合成件落点 = 系统临时目录，不进仓树。
//
// ═══════════════════ ★ Join / 词法面逐处交代（✗ 不用 filepath.Join/Abs）═══════════════════
//
//	逐处现读 py 原件的路径拼接：`MATRIX_REL`（posixpath.join 五段）· `default_ledger`（join 三段）·
//	`load_tree` 的 `os.path.join(root, MATRIX_REL)` · `_fixture` 的三段 join · `main` 的
//	`os.path.abspath(args.root)`。⇒ 本件带自写 cvrPyJoin / cvrPyNormpath / cvrPyAbspath（CPython
//	posixpath 语义，含 **normpath 保留恰两个前导斜杠** 的语义与 **join 的 `..` 原样留 / 绝对后段丢前段
//	/ 空段** 三条差，本件已钉单测）。
//
// ═══════════════════ ★ py 字符串原语 / JSON 面逐处交代 ═══════════════════
//
//	· `str.strip()` 是 py **Unicode 空白**（含 \x1c-\x1f · \x85 · \xa0）⇒ 自写 cvrPyStrip（✗ 非
//	  strings.TrimSpace；Go `unicode.IsSpace` **不剥** \x1c-\x1f）。
//	· `str.split()`（无参）按同一空白集切、丢空段 ⇒ 自写 cvrPySplit。
//	· `os.path.basename` ⇒ 自写 cvrPyBasename（末段口径）。
//	· `json.dumps(..., ensure_ascii=False, sort_keys=True)` 的**空格分隔符**（`", "` / `": "`）与
//	  py 转义规则（非 ASCII 原样 · 不转 `<>&`）⇒ 自写 cvrPyJSONMarshal（✗ 非 encoding/json 的紧凑形）。
//	· 文件按行读 = **universal newlines**（\r\n 与裸 \r 都折成 \n）⇒ 自写 cvrSplitLinesPy。
//
// ═══════════════════ ★ 未覆盖面（✗ 不声称逐字对齐）═══════════════════
//
//	① **argparse 帮助/用法正文随解释器版本漂**：≤3.9 印 `optional arguments:`、≥3.10 印 `options:`。
//	   本件按**门跑同侧**的解释器（`/usr/bin/python3` · 3.9）逐字钉版；换 ≥3.10 解释器则**只差该行**。
//	② **argparse 极端内部形态**：短旗标簇（`-hx`）· 负数作选项值（`--max-examples -1`）· `--X=` 空值边界
//	   —— 本件按常用档复刻，未逐一验证的极端形态见此列。
//	③ **JSONDecodeError 正文**：py 把 `json.load`/`json.loads` 的异常正文（`Expecting value: line … column …`）
//	   原样嵌进「不是合法 JSON（…）」；本件用 Go 解码器的错文（口径不同）⇒ 该档只对齐 rc，正文列此。
//	④ **非 UTF-8 输入件 / 组合码点排序**：py 读文本遇非法字节抛 UnicodeDecodeError（C 类 ⇒ 回溯 rc=1）；
//	   本件按字节读、非法序列档未覆盖。`sorted` 码点序 ⟷ Go 字节序对合法 UTF-8 等序。
//	⑤ **非对象账行 / 非字符串字段**：py 会在 `r.get(...)`/`.strip()` 处抛未捕异常（⇒ 回溯 rc=1）；
//	   本件按缺省/空串处置（fail-closed 前的温和档），真实账行恒为对象且字段为串 ⇒ 不触发。
//
// ★ 真口子（交回入账，✗ 不改 py）：A 类 —— `self_test` 若自身抛异常，`atexit` 仍会回收（本例非泄漏形态）；
//
//	本件一律 defer 回收（行为差仅在异常路径，输出不可见）。C 类 —— `__main__` 裸 `sys.exit(main(...))`、
//	main 内未捕异常 ⇒ 回溯 rc=1 而非「不给结论 rc=2」（本件对读失败一律 fail-closed 到 rc=2 并写未覆盖③⑤）。
//	D 类 —— 未见死旗标：`--root/--ledger/--matrix/--json/--self-test/--no-self-test/--baseline/--list-rules/
//	--strict/--max-examples` 均被读取且有效。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	cvrRCOK      = 0
	cvrRCFail    = 1
	cvrRCBlocked = 2
)

// cvrMatrixRel = py MATRIX_REL（posixpath.join("core","cmd","zerg","testdata","cli-matrix.json")）。
var cvrMatrixRel = cvrPyJoin("core", "cmd", "zerg", "testdata", "cli-matrix.json")

const cvrLedgerFile = "zerg-cli-gaps.jsonl"

// 通用命令**闭集**（唯一一处定义 · 逐字写死 · 无子串 / 无模糊匹配）。
var cvrGenericHeads = []string{"help", "doctor"}
var cvrGenericVerbTail = []string{"ls", "status"}

// 棘轮基线（首日现读数 · 只许下调、永不上调）。
var cvrBaseline = map[string]int{"verify": 675, "repro": 526}

const cvrBaselineNote = "2026-10-05 首测（本门新建当天）：账 1180 行 · 命令树 188 条 ⇒ " +
	"verify 面不可判 = 675（其中 通用命令 675 · 不解析命令树 0）· repro 面样板 = 526。" +
	"★ 只认 `BASELINE` 那两格；本段历史值仅供追溯。"

// ─────────────────────────── py 词法/字符串原语 ───────────────────────────

// cvrPyIsSpace —— py str.isspace() 的口径（含 \x1c-\x1f；Go unicode.IsSpace 不剥这四个）。
func cvrPyIsSpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// cvrPyStrip —— py str.strip()（无参 · 剥 py Unicode 空白）。
func cvrPyStrip(s string) string {
	return strings.TrimFunc(s, cvrPyIsSpace)
}

// cvrPySplit —— py str.split()（无参 · 按 py 空白切 · 丢空段）。
func cvrPySplit(s string) []string {
	var out []string
	var b strings.Builder
	for _, r := range s {
		if cvrPyIsSpace(r) {
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// cvrPyBasename —— py posixpath.basename（末段：末个 '/' 之后整段，尾斜杠 ⇒ 空串）。
func cvrPyBasename(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// cvrPyJoin —— CPython posixpath.join（三条语义差照 CPython 直译：`..` 原样留 · 绝对后段丢前段 · 空段）。
func cvrPyJoin(a string, parts ...string) string {
	path := a
	for _, b := range parts {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// cvrPyIsAbs —— py posixpath.isabs。
func cvrPyIsAbs(p string) bool { return strings.HasPrefix(p, "/") }

// cvrPyNormpath —— CPython posixpath.normpath（保留**恰两个**前导斜杠）。
func cvrPyNormpath(path string) string {
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
	var out []string
	abs := initial > 0
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (!abs && len(out) == 0) || (len(out) > 0 && out[len(out)-1] == "..") {
			out = append(out, comp)
		} else if len(out) > 0 {
			out = out[:len(out)-1]
		}
	}
	res := strings.Join(out, "/")
	if initial > 0 {
		res = strings.Repeat("/", initial) + res
	}
	if res == "" {
		return "."
	}
	return res
}

// cvrPyAbspath —— CPython os.path.abspath = normpath(join(getcwd(), path))（词法 · 不解析符号链接）。
func cvrPyAbspath(path string) string {
	if !cvrPyIsAbs(path) {
		cwd, _ := os.Getwd()
		path = cvrPyJoin(cwd, path)
	}
	return cvrPyNormpath(path)
}

// cvrSplitLinesPy —— py 文本文件按行迭代（universal newlines：\r\n 与裸 \r 折成 \n；末尾换行不产空行）。
func cvrSplitLinesPy(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// ─────────────────────────── 1. 通用命令闭集 + 命令树解析 ───────────────────────────

func cvrNormCmd(s string) string {
	return strings.Join(cvrPySplit(cvrPyStrip(strings.Trim(s, "\"'"))), " ")
}

// cvrTokensOf —— 去 `zerg` 前缀后的 token 列表（首词 basename == zerg 才剥）。
func cvrTokensOf(cmd string) []string {
	toks := cvrPySplit(cvrNormCmd(cmd))
	if len(toks) > 0 && cvrPyBasename(toks[0]) == "zerg" {
		toks = toks[1:]
	}
	return toks
}

func cvrIn(s string, set []string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

// cvrIsGeneric —— 命中通用命令闭集？（逐 token 比对 · 无子串匹配）
func cvrIsGeneric(toks []string) bool {
	switch len(toks) {
	case 1:
		return cvrIn(toks[0], cvrGenericHeads)
	case 2:
		return cvrIn(toks[0], cvrGenericHeads) || cvrIn(toks[1], cvrGenericVerbTail)
	}
	return false
}

// cvrResolves —— 去 `zerg` 前缀后的 token 序列，是不是命令树里某条命令 token 序列的**前缀**？
func cvrResolves(toks []string, tree [][]string) bool {
	if len(toks) == 0 {
		return false
	}
	for _, cmd := range tree {
		if len(cmd) > len(toks) {
			continue
		}
		same := true
		for i := range cmd {
			if cmd[i] != toks[i] {
				same = false
				break
			}
		}
		if same {
			return true
		}
	}
	return false
}

// ─────────────────────────── 2. 真源装载 ───────────────────────────

type cvrUnsupportedErr struct{ msg string }

func (e *cvrUnsupportedErr) Error() string { return e.msg }

func cvrUnsupported(format string, a ...any) error {
	return &cvrUnsupportedErr{msg: fmt.Sprintf(format, a...)}
}

// cvrDefaultLedger —— py default_ledger()。
func cvrDefaultLedger() string {
	sd := os.Getenv("ZERG_STATE_DIR")
	if sd == "" {
		sd = cvrPyJoin(os.Getenv("HOME"), ".zerg", "state")
	}
	return cvrPyJoin(sd, cvrLedgerFile)
}

func cvrIsRegular(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// cvrLoadTree —— py load_tree()：返回 (tree, path, err)。
func cvrLoadTree(root, matrix string) ([][]string, string, error) {
	path := matrix
	if path == "" {
		path = cvrPyJoin(root, cvrMatrixRel)
	}
	if !cvrIsRegular(path) {
		return nil, "", cvrUnsupported("命令树机器面缺件：%s（缺件 ⇒ 不给结论）", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", cvrUnsupported("命令树机器面不是合法 JSON：%s（%s）", path, cvrJSONErrText(err))
	}
	data, derr := cvrJSONDecode(raw)
	if derr != nil {
		return nil, "", cvrUnsupported("命令树机器面不是合法 JSON：%s（%s）", path, cvrJSONErrText(derr))
	}
	obj, ok := data.(map[string]any)
	if !ok {
		return nil, "", cvrUnsupported("命令树机器面 %s 的 `cases` 为空 ⇒ 空转，不给结论", path)
	}
	cases, ok := obj["cases"].([]any)
	if !ok || len(cases) == 0 {
		return nil, "", cvrUnsupported("命令树机器面 %s 的 `cases` 为空 ⇒ 空转，不给结论", path)
	}
	tree := [][]string{}
	for _, c := range cases {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		cmdv, _ := cm["command"].(string)
		if cvrPyStrip(cmdv) != "" {
			tree = append(tree, cvrPySplit(cmdv))
		}
	}
	if len(tree) == 0 {
		return nil, "", cvrUnsupported("命令树机器面里 0 条 `command` ⇒ 空转，不给结论")
	}
	return tree, path, nil
}

// cvrLoadLedger —— py load_ledger()：返回 (rows, path, err)。
func cvrLoadLedger(ledger string) ([]map[string]any, string, error) {
	path := ledger
	if path == "" {
		path = cvrDefaultLedger()
	}
	if !cvrIsRegular(path) {
		return nil, "", cvrUnsupported("缺口账真源缺件：%s（缺件 ⇒ 不给结论）", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", cvrUnsupported("缺口账真源缺件：%s（缺件 ⇒ 不给结论）", path)
	}
	var rows []map[string]any
	for i, line := range cvrSplitLinesPy(string(raw)) {
		line = cvrPyStrip(line)
		if line == "" {
			continue
		}
		obj, derr := cvrJSONDecode([]byte(line))
		if derr != nil {
			return nil, "", cvrUnsupported("缺口账第 %d 行不是合法 JSON：%s", i+1, cvrJSONErrText(derr))
		}
		m, ok := obj.(map[string]any)
		if !ok {
			return nil, "", cvrUnsupported("缺口账第 %d 行不是合法 JSON：%s", i+1, cvrJSONErrText(fmt.Errorf("非对象行")))
		}
		rows = append(rows, m)
	}
	if len(rows) == 0 {
		return nil, "", cvrUnsupported("缺口账为空（0 行）⇒ 空转，不给结论")
	}
	return rows, path, nil
}

// ─────────────────────────── 3. 判定 / 汇总 ───────────────────────────

type cvrItem struct{ ID, Cmd, Why string }

type cvrReport struct {
	total       int
	vGeneric    []cvrItem
	vUnresolved []cvrItem
	rGeneric    []cvrItem
}

func (r *cvrReport) vBad() int { return len(r.vGeneric) + len(r.vUnresolved) }

func cvrStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func cvrIDOf(r map[string]any) string {
	v := cvrStr(r["id"])
	if v == "" {
		return "?"
	}
	return v
}

func cvrAnalyse(rows []map[string]any, tree [][]string) *cvrReport {
	rep := &cvrReport{total: len(rows)}
	for _, r := range rows {
		rid := cvrIDOf(r)
		vc := cvrStr(r["verify_cmd"])
		tv := cvrTokensOf(vc)
		if cvrIsGeneric(tv) {
			rep.vGeneric = append(rep.vGeneric, cvrItem{ID: rid, Cmd: cvrNormCmd(vc), Why: "通用命令（闭集）"})
		} else if !cvrResolves(tv, tree) {
			rep.vUnresolved = append(rep.vUnresolved, cvrItem{ID: rid, Cmd: cvrNormCmd(vc), Why: "不解析命令树"})
		}
		rc := cvrStr(r["repro_cmd"])
		tr := cvrTokensOf(rc)
		if cvrIsGeneric(tr) {
			rep.rGeneric = append(rep.rGeneric, cvrItem{ID: rid, Cmd: cvrNormCmd(rc), Why: "通用命令（闭集）"})
		}
	}
	return rep
}

func cvrMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// cvrVerdict —— py verdict_of()：⇒ (rc, code, reason)。baseline=nil ⇒ 用默认 BASELINE。
func cvrVerdict(rep *cvrReport, strict bool, baseline map[string]int) (int, string, string) {
	if baseline == nil {
		baseline = cvrBaseline
	}
	extraV := rep.vBad() - baseline["verify"]
	extraR := len(rep.rGeneric) - baseline["repro"]
	over := extraV > 0 || extraR > 0
	if strict && over {
		return cvrRCFail, "RED", fmt.Sprintf(
			"超额：verify %d > 基线 %d（超 %d）· repro %d > 基线 %d（超 %d）",
			rep.vBad(), baseline["verify"], cvrMax(extraV, 0),
			len(rep.rGeneric), baseline["repro"], cvrMax(extraR, 0))
	}
	tag := "不超额"
	if over {
		tag = "超额（报告档不阻断）"
	}
	return cvrRCOK, "REPORT", fmt.Sprintf(
		"报告档 · %s：verify 样板 %d / %d（通用 %d · 不解析 %d）· repro 样板 %d / %d",
		tag, rep.vBad(), rep.total, len(rep.vGeneric), len(rep.vUnresolved), len(rep.rGeneric), rep.total)
}

// ─────────────────────────── 4. 报告 ───────────────────────────

func cvrPrintReport(w io.Writer, rep *cvrReport, ledgerPath, matrixPath string, maxEx int) {
	fmt.Fprintln(w, "── 判据须能真判门（GAP-20260926-55 · 报告档首版）──")
	fmt.Fprintf(w, "   缺口账真源：%s（%d 行）\n", ledgerPath, rep.total)
	fmt.Fprintf(w, "   命令树真源：%s\n", matrixPath)
	fmt.Fprintf(w, "   基线（棘轮 · 只许下调）：verify=%d · repro=%d\n", cvrBaseline["verify"], cvrBaseline["repro"])
	fmt.Fprintln(w)
	fmt.Fprintln(w, "== verify_cmd 面（①② 两关都判）==")
	fmt.Fprintf(w, "   不可判 = **%d / %d**（其中 通用命令 %d · 不解析命令树 %d）\n",
		rep.vBad(), rep.total, len(rep.vGeneric), len(rep.vUnresolved))
	for i, it := range rep.vGeneric {
		if i >= maxEx {
			break
		}
		fmt.Fprintf(w, "     - [通用] %s  ← %s\n", it.ID, it.Cmd)
	}
	if len(rep.vGeneric) > maxEx {
		fmt.Fprintf(w, "     … 另 %d 条通用命令（--max-examples 调整）\n", len(rep.vGeneric)-maxEx)
	}
	for i, it := range rep.vUnresolved {
		if i >= maxEx {
			break
		}
		fmt.Fprintf(w, "     - [不解析] %s  ← %s\n", it.ID, it.Cmd)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "== repro_cmd 面（只判 ②「非通用命令」· 解析面不判）==")
	fmt.Fprintf(w, "   样板 = **%d / %d**（通用命令）\n", len(rep.rGeneric), rep.total)
	for i, it := range rep.rGeneric {
		if i >= maxEx {
			break
		}
		fmt.Fprintf(w, "     - [通用] %s  ← %s\n", it.ID, it.Cmd)
	}
	if len(rep.rGeneric) > maxEx {
		fmt.Fprintf(w, "     … 另 %d 条通用命令（--max-examples 调整）\n", len(rep.rGeneric)-maxEx)
	}
	fmt.Fprintln(w, "   ★ 未覆盖形态：`--repro-cmd` 设计上收**自由文本** ⇒ 「解析到命令树」这一关**不适用**（属 `GAP-20260927-246` 的写侧面）。")
	fmt.Fprintln(w)
	_, _, reason := cvrVerdict(rep, false, nil)
	fmt.Fprintln(w, "== 结论 ==")
	fmt.Fprintf(w, "   %s\n", reason)
	fmt.Fprintln(w, "   ⇒ 报告档（rc=0）；`--strict` 时超额 ⇒ rc=1")
}

func cvrPrintBaseline(w io.Writer, rep *cvrReport) {
	fmt.Fprintln(w, "── 棘轮视图（只读 · 不改任何文件）──")
	fmt.Fprintf(w, "   verify 面不可判 现测 = %d · 基线 = %d · 剩余（可下调到）= %d\n",
		rep.vBad(), cvrBaseline["verify"], rep.vBad())
	fmt.Fprintf(w, "   repro  面样板   现测 = %d · 基线 = %d · 剩余 = %d\n",
		len(rep.rGeneric), cvrBaseline["repro"], len(rep.rGeneric))
	fmt.Fprintf(w, "   自述 = %s\n", cvrBaselineNote)
	fmt.Fprintln(w, "   号令：一次只许**下调**基线（改成现测值），**永不上调**；超额 ⇒ 先补真判据或挪出闭集，✗ 不许抬基线。")
}

func cvrPrintListRules(w io.Writer) {
	fmt.Fprintln(w, "check-verify-cmd-real.py 规则表（判据须能真判门 · GAP-20260926-55）")
	fmt.Fprintln(w, "  判据：一条账某列「能真判」 ⟺ ① 解析到命令树  且  ② 不是通用命令")
	fmt.Fprintln(w, "  通用命令**闭集**（去 zerg 前缀后 · 逐 token 比对 · 无子串匹配）：")
	fmt.Fprintf(w, "     · 1 词：%s\n", strings.Join(cvrGenericHeads, " / "))
	fmt.Fprintf(w, "     · 2 词：首词 ∈ %s  或  尾词 ∈ %s\n",
		strings.Join(cvrGenericHeads, " / "), strings.Join(cvrGenericVerbTail, " / "))
	fmt.Fprintln(w, "  面分：verify 面 ①② 都判 · repro 面 只判 ②（解析面不适用）")
	fmt.Fprintf(w, "  棘轮：BASELINE verify=%d · repro=%d（只许下调）\n", cvrBaseline["verify"], cvrBaseline["repro"])
	fmt.Fprintln(w, "  退码：0 报告完成 · 1（仅 --strict 超额）· 2 不给结论")
}

// ─────────────────────────── 5. 自检（成对负控 · 走真退码）───────────────────────────

type cvrCase struct {
	desc  string
	ok    bool
	extra string
}

func cvrFixture(base, name string, matrixCmds []string, ledgerRows []map[string]any) (string, string) {
	root := cvrPyJoin(base, name)
	_ = os.MkdirAll(cvrPyJoin(root, "core", "cmd", "zerg", "testdata"), 0o755)
	cases := make([]any, 0, len(matrixCmds))
	for _, c := range matrixCmds {
		cases = append(cases, map[string]any{"command": c})
	}
	_ = os.WriteFile(cvrPyJoin(root, cvrMatrixRel),
		[]byte(cvrPyJSONMarshal(map[string]any{"cases": cases})), 0o644)
	ledger := cvrPyJoin(root, "ledger.jsonl")
	var sb strings.Builder
	for _, row := range ledgerRows {
		sb.WriteString(cvrPyJSONMarshal(row))
		sb.WriteString("\n")
	}
	_ = os.WriteFile(ledger, []byte(sb.String()), 0o644)
	return root, ledger
}

// cvrSelfTestLoad —— 测试辅助（= py `_load`）：可覆盖账行。
func cvrSelfTestLoad(root, ledger string, rows []map[string]any) *cvrReport {
	tree, _, _ := cvrLoadTree(root, "")
	var r []map[string]any
	if rows != nil {
		r = rows
	} else {
		r, _, _ = cvrLoadLedger(ledger)
	}
	return cvrAnalyse(r, tree)
}

func cvrSelfTest(sink, stderr io.Writer) bool {
	base, err := os.MkdirTemp("", "vcr-selftest-")
	if err != nil {
		base = os.TempDir()
	} else {
		// ★ 安全删点：只在「建点成功」支挂 defer（回落系统临时目录时 **不** 删）。
		defer os.RemoveAll(base)
	}

	var cases []cvrCase
	check := func(desc string, ok bool, extra string) {
		cases = append(cases, cvrCase{desc: desc, ok: ok, extra: extra})
	}

	// ── 闭集（纯函数）逐格 ──
	check("闭集：`zerg task ls` ⇒ 通用", cvrIsGeneric(cvrTokensOf("zerg task ls")), "")
	check("闭集：`zerg gate ls` ⇒ 通用", cvrIsGeneric(cvrTokensOf("zerg gate ls")), "")
	check("闭集：`zerg gap ls` ⇒ 通用", cvrIsGeneric(cvrTokensOf("zerg gap ls")), "")
	check("闭集：`zerg help dangerous` ⇒ 通用", cvrIsGeneric(cvrTokensOf("zerg help dangerous")), "")
	check("闭集：`zerg help` ⇒ 通用", cvrIsGeneric(cvrTokensOf("zerg help")), "")
	check("闭集：`zerg doctor` ⇒ 通用", cvrIsGeneric(cvrTokensOf("zerg doctor")), "")
	check("闭集：`zerg doctor --json` ⇒ 通用", cvrIsGeneric(cvrTokensOf("zerg doctor --json")), "")
	check("闭集：`zerg repo status` ⇒ 通用", cvrIsGeneric(cvrTokensOf("zerg repo status")), "")
	check("闭集：`zerg gate find check-x` ⇒ **非**通用（闭集外不咬）",
		!cvrIsGeneric(cvrTokensOf("zerg gate find check-x")), "")
	check("闭集：`zerg gate run --fast` ⇒ **非**通用", !cvrIsGeneric(cvrTokensOf("zerg gate run --fast")), "")
	check("闭集：`zerg model show <名>` ⇒ **非**通用", !cvrIsGeneric(cvrTokensOf("zerg model show m1")), "")
	check("闭集：`./bin/zerg gate ls`（带路径前缀）⇒ 通用（basename 剥法）",
		cvrIsGeneric(cvrTokensOf("./bin/zerg gate ls")), "")

	// ── 命令树解析（纯函数）逐格 ──
	tree := [][]string{}
	for _, c := range []string{"gate ls", "gate find", "task ls", "help", "doctor", "gap ls", "repo status"} {
		tree = append(tree, cvrPySplit(c))
	}
	check("解析：`zerg gate find kw` ⇒ 前缀命中 `gate find`", cvrResolves(cvrTokensOf("zerg gate find kw"), tree), "")
	check("解析：`zerg gate ls` ⇒ 命中 `gate ls`", cvrResolves(cvrTokensOf("zerg gate ls"), tree), "")
	check("解析：`zerg 不存在命令abc` ⇒ **不解析**（负控 ⊥ 上一格）",
		!cvrResolves(cvrTokensOf("zerg 不存在命令abc"), tree), "")
	check("解析：只有 `zerg` 前缀 ⇒ 不解析", !cvrResolves(cvrTokensOf("zerg"), tree), "")

	// ── analyse + 真退码（成对负控）──
	root, ledger := cvrFixture(base, "mix",
		[]string{"gate ls", "gate find", "task ls", "help"},
		[]map[string]any{
			{"id": "G-T1", "verify_cmd": "zerg gate ls", "repro_cmd": "zerg task ls"},
			{"id": "G-T2", "verify_cmd": "zerg gate find check-x", "repro_cmd": "go test ./... -run X"},
		})
	rep := cvrSelfTestLoad(root, ledger, nil)
	check("混合夹具：verify 不可判 = 1（G-T1 通用 · G-T2 真判）", rep.vBad() == 1, fmt.Sprintf("bad=%d", rep.vBad()))
	check("混合夹具：repro 样板 = 1（G-T1 通用 · G-T2 自由文本不判）", len(rep.rGeneric) == 1,
		fmt.Sprintf("bad=%d", len(rep.rGeneric)))
	zero := map[string]int{"verify": 0, "repro": 0}
	big := map[string]int{"verify": 99, "repro": 99}
	rc, _, _ := cvrVerdict(rep, false, zero)
	check("报告档：超额（1 > 0）仍 rc=0（**不阻断**）", rc == cvrRCOK, fmt.Sprintf("rc=%d", rc))
	rcS, _, _ := cvrVerdict(rep, true, zero)
	check("--strict：超额 ⇒ rc=1（与上一格成对）", rcS == cvrRCFail, fmt.Sprintf("rc=%d", rcS))
	rcB, _, _ := cvrVerdict(rep, true, big)
	check("--strict：不超额（1 ≤ 99）⇒ rc=0（闭集判据非恒红）", rcB == cvrRCOK, fmt.Sprintf("rc=%d", rcB))

	rep0 := cvrSelfTestLoad(root, ledger, []map[string]any{
		{"id": "G-T2", "verify_cmd": "zerg gate find check-x", "repro_cmd": "go test ./... -run X"}})
	check("负控：无样板夹具 ⇒ verify 0 · repro 0", rep0.vBad() == 0 && len(rep0.rGeneric) == 0, "")

	// ── 缺件 / 空转 ⇒ rc=2（真退码）──
	rc = cvrMain(stdoutDiscard{}, stderr, []string{"--no-self-test", "--root", root,
		"--ledger", cvrPyJoin(root, "nope.jsonl")})
	check("缺件：账读不到 ⇒ rc=2", rc == cvrRCBlocked, fmt.Sprintf("rc=%d", rc))

	emptyRoot, emptyLedger := cvrFixture(base, "empty", []string{"gate ls"}, []map[string]any{})
	rc = cvrMain(stdoutDiscard{}, stderr, []string{"--no-self-test", "--root", emptyRoot, "--ledger", emptyLedger})
	check("空转：账 0 行 ⇒ rc=2（绝不报绿）", rc == cvrRCBlocked, fmt.Sprintf("rc=%d", rc))

	nomxRoot, nomxLedger := cvrFixture(base, "nomx", []string{"gate ls"},
		[]map[string]any{{"id": "G-1", "verify_cmd": "zerg gate ls"}})
	_ = os.Remove(cvrPyJoin(nomxRoot, cvrMatrixRel))
	rc = cvrMain(stdoutDiscard{}, stderr, []string{"--no-self-test", "--root", nomxRoot, "--ledger", nomxLedger})
	check("缺件：命令树缺件 ⇒ rc=2", rc == cvrRCBlocked, fmt.Sprintf("rc=%d", rc))

	fmt.Fprintln(sink, "── check-verify-cmd-real 自检（成对负控 · 真退码）──")
	bad := 0
	for _, c := range cases {
		tail := ""
		if c.extra != "" && !c.ok {
			tail = "（" + c.extra + "）"
		}
		mark := "✓"
		if !c.ok {
			mark = "✗"
			bad++
		}
		fmt.Fprintf(sink, "   %s %s%s\n", mark, c.desc, tail)
	}
	verdict := "全过"
	if bad != 0 {
		verdict = "**不过**"
	}
	fmt.Fprintf(sink, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n", verdict, len(cases), bad)
	return bad == 0
}

// stdoutDiscard —— 自检内的嵌套 main 调用只需 stderr 面（py 那几档只往 sys.stderr 打）。
type stdoutDiscard struct{}

func (stdoutDiscard) Write(p []byte) (int, error) { return len(p), nil }

// ─────────────────────────── 6. argparse 复刻 ───────────────────────────

const (
	cvrProg  = "check-verify-cmd-real.py"
	cvrUsage = "usage: check-verify-cmd-real.py [-h] [--root ROOT] [--ledger LEDGER]\n" +
		"                                [--matrix MATRIX] [--json] [--self-test]\n" +
		"                                [--no-self-test] [--baseline] [--list-rules]\n" +
		"                                [--strict] [--max-examples MAX_EXAMPLES]"
	cvrHelp = cvrUsage + "\n" +
		"\n" +
		"判据须能真判门（GAP-20260926-55）\n" +
		"\n" +
		"optional arguments:\n" +
		"  -h, --help            show this help message and exit\n" +
		"  --root ROOT\n" +
		"  --ledger LEDGER\n" +
		"  --matrix MATRIX\n" +
		"  --json\n" +
		"  --self-test\n" +
		"  --no-self-test\n" +
		"  --baseline\n" +
		"  --list-rules\n" +
		"  --strict\n" +
		"  --max-examples MAX_EXAMPLES\n"
)

var cvrLongOpts = []string{"help", "root", "ledger", "matrix", "json", "self-test",
	"no-self-test", "baseline", "list-rules", "strict", "max-examples"}

type cvrNS struct {
	root       string
	rootSet    bool
	ledger     string
	matrix     string
	asJSON     bool
	selfTest   bool
	noSelfTest bool
	baseline   bool
	listRules  bool
	strict     bool
	maxEx      int
	help       bool
}

func cvrResolveLong(body string) (string, string) {
	if body == "" {
		return "", ""
	}
	for _, o := range cvrLongOpts {
		if o == body {
			return o, ""
		}
	}
	var hits []string
	for _, o := range cvrLongOpts {
		if strings.HasPrefix(o, body) {
			hits = append(hits, "--"+o)
		}
	}
	if len(hits) == 1 {
		return strings.TrimPrefix(hits[0], "--"), ""
	}
	if len(hits) > 1 {
		return "", strings.Join(hits, ", ")
	}
	return "", ""
}

// cvrIsFlagOpt —— 该长旗标是否 store_true（不取值）。
func cvrIsFlagOpt(o string) bool {
	switch o {
	case "json", "self-test", "no-self-test", "baseline", "list-rules", "strict", "help":
		return true
	}
	return false
}

// cvrPyInt —— py int(str) 的常用档（前后 py 空白 · 可选符号 · 数字间单下划线）。
func cvrPyInt(s string) (int, bool) {
	t := cvrPyStrip(s)
	if t == "" {
		return 0, false
	}
	sign := ""
	if t[0] == '+' || t[0] == '-' {
		sign, t = t[:1], t[1:]
	}
	if t == "" {
		return 0, false
	}
	var b strings.Builder
	for i := 0; i < len(t); i++ {
		c := t[i]
		if c >= '0' && c <= '9' {
			b.WriteByte(c)
			continue
		}
		if c == '_' && i > 0 && i+1 < len(t) && t[i-1] >= '0' && t[i-1] <= '9' && t[i+1] >= '0' && t[i+1] <= '9' {
			continue
		}
		return 0, false
	}
	n, err := strconv.Atoi(sign + b.String())
	if err != nil {
		return 0, false
	}
	return n, true
}

// cvrParseArgs —— 复刻 argparse 常用档，返回 (ns, errMsg)。errMsg 非空 ⇒ usage+error rc=2。
func cvrParseArgs(argv []string) (*cvrNS, string) {
	ns := &cvrNS{maxEx: 12}
	var unknown []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--":
			unknown = append(unknown, argv[i+1:]...)
			i = len(argv)
		case strings.HasPrefix(a, "--"):
			body := a[2:]
			val := ""
			hasVal := false
			if eq := strings.Index(body, "="); eq >= 0 {
				val, hasVal = body[eq+1:], true
				body = body[:eq]
			}
			canon, ambig := cvrResolveLong(body)
			if ambig != "" {
				return nil, "ambiguous option: --" + body + " could match " + ambig
			}
			if canon == "" {
				unknown = append(unknown, a)
				continue
			}
			if cvrIsFlagOpt(canon) {
				if hasVal {
					return nil, "argument --" + canon + ": ignored explicit argument '" + val + "'"
				}
				switch canon {
				case "help":
					ns.help = true
				case "json":
					ns.asJSON = true
				case "self-test":
					ns.selfTest = true
				case "no-self-test":
					ns.noSelfTest = true
				case "baseline":
					ns.baseline = true
				case "list-rules":
					ns.listRules = true
				case "strict":
					ns.strict = true
				}
				continue
			}
			if !hasVal {
				if i+1 >= len(argv) || (strings.HasPrefix(argv[i+1], "-") && argv[i+1] != "-") {
					return nil, "argument --" + canon + ": expected one argument"
				}
				i++
				val = argv[i]
			}
			switch canon {
			case "root":
				ns.root, ns.rootSet = val, true
			case "ledger":
				ns.ledger = val
			case "matrix":
				ns.matrix = val
			case "max-examples":
				n, ok := cvrPyInt(val)
				if !ok {
					return nil, "argument --max-examples: invalid int value: '" + val + "'"
				}
				ns.maxEx = n
			}
		case strings.HasPrefix(a, "-") && a != "-":
			if a == "-h" {
				ns.help = true
			} else {
				unknown = append(unknown, a)
			}
		default:
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		return nil, "unrecognized arguments: " + strings.Join(unknown, " ")
	}
	return ns, ""
}

// ─────────────────────────── 7. 入口 ───────────────────────────

func cvrMain(stdout, stderr io.Writer, argv []string) int {
	ns, errMsg := cvrParseArgs(argv)
	if errMsg != "" {
		fmt.Fprintf(stderr, "%s\n%s: error: %s\n", cvrUsage, cvrProg, errMsg)
		return cvrRCBlocked
	}
	if ns.help {
		fmt.Fprint(stdout, cvrHelp)
		return cvrRCOK
	}
	if ns.asJSON && (ns.selfTest || ns.listRules) {
		which := "--list-rules"
		if ns.selfTest {
			which = "--self-test"
		}
		fmt.Fprintf(stderr, "✗ 用法错（rc=2）：`--json` 与 `%s` 互斥（后者只产人读文本、不产 JSON）\n", which)
		return cvrRCBlocked
	}
	if ns.listRules {
		cvrPrintListRules(stdout)
		return cvrRCOK
	}
	if ns.selfTest {
		fmt.Fprintln(stdout, "── check-verify-cmd-real 自检（成对负控 · 真退码）──")
		if cvrSelfTest(stdout, stderr) {
			return cvrRCOK
		}
		return cvrRCBlocked
	}
	if !ns.noSelfTest {
		st := stdout
		if ns.asJSON {
			st = stderr
		}
		fmt.Fprintln(st, "── 自检（成对负控）──")
		if !cvrSelfTest(st, stderr) {
			fmt.Fprintln(st, "自检未过 ⇒ 拒绝扫真账（rc=2 不给结论）")
			return cvrRCBlocked
		}
		fmt.Fprintln(st)
	}

	root := cvrPyAbspath(ns.root)
	tree, matrixPath, terr := cvrLoadTree(root, ns.matrix)
	if terr != nil {
		return cvrEmitError(stdout, stderr, ns, terr)
	}
	rows, ledgerPath, lerr := cvrLoadLedger(ns.ledger)
	if lerr != nil {
		return cvrEmitError(stdout, stderr, ns, lerr)
	}
	rep := cvrAnalyse(rows, tree)

	rc, code, reason := cvrVerdict(rep, ns.strict, nil)
	if ns.asJSON {
		obj := map[string]any{
			"schema": "zerg/v1",
			"kind":   "VerifyCmdReal",
			"total":  rep.total,
			"baseline": map[string]any{
				"verify": cvrBaseline["verify"], "repro": cvrBaseline["repro"]},
			"verify": map[string]any{
				"bad": rep.vBad(), "generic": len(rep.vGeneric), "unresolved": len(rep.vUnresolved),
				"items": cvrItemsJSON(append(append([]cvrItem{}, rep.vGeneric...), rep.vUnresolved...))},
			"repro": map[string]any{
				"bad": len(rep.rGeneric), "generic": len(rep.rGeneric),
				"items": cvrItemsJSON(rep.rGeneric)},
			"strict": ns.strict, "rc": rc, "code": code, "reason": reason,
		}
		fmt.Fprintln(stdout, cvrPyJSONMarshal(obj))
		fmt.Fprintf(stderr, "rc=%d · %s\n", rc, reason)
	} else {
		cvrPrintReport(stdout, rep, ledgerPath, matrixPath, ns.maxEx)
		if ns.baseline {
			cvrPrintBaseline(stdout, rep)
		}
	}
	return rc
}

func cvrItemsJSON(items []cvrItem) []any {
	out := make([]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{"id": it.ID, "cmd": it.Cmd, "why": it.Why})
	}
	return out
}

func cvrEmitError(stdout, stderr io.Writer, ns *cvrNS, err error) int {
	fmt.Fprintf(stderr, "✗ 不给结论（rc=2）：%s\n", err.Error())
	if ns.asJSON {
		fmt.Fprintln(stdout, cvrPyJSONMarshal(map[string]any{
			"schema": "zerg/v1", "kind": "VerifyCmdRealError", "rc": cvrRCBlocked, "reason": err.Error(),
		}))
	}
	return cvrRCBlocked
}

// checkVerifyCmdRealGateMain —— 入口（= py `main()`）。
func checkVerifyCmdRealGateMain(stdout, stderr io.Writer, argv []string) int {
	return cvrMain(stdout, stderr, argv)
}

// sortedKeys 供 JSON 编码器使用（保 py sort_keys 语义）。
func cvrSortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ─────────────────────────── 8. JSON 面（py json 语义）───────────────────────────

// cvrJSONDecode —— py json.loads 的常用档（Go 解码器 · 数字落 float64）。
func cvrJSONDecode(raw []byte) (any, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// cvrJSONErrText —— 异常正文（py json.JSONDecodeError 的 `msg: line L column C (char N)` 口径不同，
// 见件头未覆盖③；此处取 Go 解码器正文）。
func cvrJSONErrText(err error) string { return err.Error() }

// cvrPyJSONStr —— py json.dumps 的字符串转义（ensure_ascii=False：非 ASCII 原样 · 不转 `<>&`）。
// ★ 反斜杠一律用 WriteByte(0x5c) 拼，避免源码里出现连续转义串。
func cvrPyJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte(0x22) // "
	for _, r := range s {
		switch r {
		case 0x22: // "
			b.WriteByte(0x5c)
			b.WriteByte(0x22)
		case 0x5c: // backslash
			b.WriteByte(0x5c)
			b.WriteByte(0x5c)
		case 0x08:
			b.WriteByte(0x5c)
			b.WriteByte('b')
		case 0x0c:
			b.WriteByte(0x5c)
			b.WriteByte('f')
		case 0x0a:
			b.WriteByte(0x5c)
			b.WriteByte('n')
		case 0x0d:
			b.WriteByte(0x5c)
			b.WriteByte('r')
		case 0x09:
			b.WriteByte(0x5c)
			b.WriteByte('t')
		default:
			if r < 0x20 {
				b.WriteByte(0x5c)
				b.WriteByte('u')
				b.WriteString(fmt.Sprintf("%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(0x22)
	return b.String()
}

// cvrPyJSONMarshal —— py json.dumps(..., ensure_ascii=False, sort_keys=True) 的默认分隔符形（", " / ": "）。
func cvrPyJSONMarshal(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return cvrPyJSONStr(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatFloat(t, 'f', 1, 64)
		}
		return strconv.FormatFloat(t, 'g', -1, 64)
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = cvrPyJSONMarshal(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []string:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = cvrPyJSONStr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		keys := cvrSortedKeys(t)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = cvrPyJSONStr(k) + ": " + cvrPyJSONMarshal(t[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return cvrPyJSONMarshal(fmt.Sprintf("%v", t))
	}
}
