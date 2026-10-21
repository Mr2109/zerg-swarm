// check_mutation_residue.go —— 门禁件 `scripts/gates/check-mutation-residue.py`（门72「变异残态」）的
// **Go 真搬样板**（v2.5.14 · M2-0 批 · 已接线（inproc 表分派））。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（**无输入物料**、纯静态检查的判据 · 4.1）的第三枚：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（4.1 M2-0 = 「无输入物料、纯静态检查的判据（读仓内件即可）」）：
//
//	✓ 选 `scripts/gates/check-mutation-residue.py`（391 行 · 纯 stdlib）：
//	  · **无外部树依赖**：真源件 `<根>/scripts/gates/mutation-smoke.sh` 与它点名的靶件**都在本仓内**
//	    （不碰文档仓 / 公开树 / 生成物 / bin/ / 状态目录）；
//	  · **输出确定有序**：逐点按 `--list` 的**点号序**出（`for i, f in enumerate(rows)`），无目录遍历；
//	  · **退码 0/1/2 三档齐**：0 全原形 / 1 有残态 / 2 判不了（argparse 用法错同落 2）；
//	  · **纯静态无写面**：判据面只读（自检件一律落 `$TMPDIR` 下的合成树）；
//	  · **行数适中**（391）· `git status --porcelain` 现读**不含它**（无人正在改）。
//	✗ 不选 `check-gap-axis-reconcile.py`（759 行）：取数面在**状态目录**（`~/.zerg/state/zerg-cli-gaps.jsonl`
//	  + env `ZERG_STATE_DIR`）+ 现跑 `git ls-files` + 现读 `core/cmd/zerg/family_gap.go` 的口径锚点
//	  ⇒ **多外部树/状态依赖**、且 `--out` 带**写面**（非「纯静态无写面」）· 行数亦非适中。
//	✗ 不选 `check-public-face-wording.py`（713 行 / 51KB）：扫面 = `publish/docs/**` **16 件** + 根面 **8 件**
//	  + 现读 `publish/replace-rules.tsv` 白名单口径 ⇒ **多外部物料**、且 51KB 的件头判据表**行数不适中**。
//	✗ 不选 `check-artifact-source-parity.py`（476 行）：取数面 = 现跑**制品** `<件> --version` + `git rev-parse`
//	  + `git merge-base/rev-list` + **输入面 mtime 扫描**（`os.walk`）⇒ 依赖 `bin/` 下的制品（本单**禁碰 `bin/`**）、
//	  遍历序与 mtime 面**非确定**（Go `WalkDir` 字典序 ≠ py `scandir` inode 序）；且它正是**预期红**的门74
//	  ⇒ 现读读数不可作对拍基线。
//
// ⚠ 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3577`（门72 · tri 档））；
//	· **不**删 `scripts/gates/check-mutation-residue.py` 原件（退场规则 4.3：**对拍通过后**才退场，且逐件显式点名）；
//	· **已接线**：入口 `mutationResidueGateMain` 由 `gateCheckGoEntry["check-mutation-residue"]` 分派（`zerg gate check check-mutation-residue`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func mutationResidueGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（**不含程序名**）
//	  · 退码   —— 0 全原形 / 1 有残态 / 2 判不了（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字 · 见本单 `pairwise.md`）：全绿 · 换值型残态 · 删行型残态 · 锚多命中（歧义）·
// 靶件不在 · 两读对不上 · 空转（五数组读不出）· 取不到输入（根不存在 · 真源件不在）· 生产侧右端加一列（反误报）·
// 表头缺本件要的列 · `--list` 无表头 · 未知旗标 · `--json`（argparse 拒收）· `-h/--help` 帮助正文 ·
// `--self-test` 19 条断言 · 真仓现读（指定根 / 不给根参数且 cwd=仓根）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助正文**：本件把 py 现跑的 `-h` 正文与两条用法错正文**逐字内嵌**（`mutHelpBody` /
//	  `mutUsageError`）⇒ 这三面**在**对拍承诺里；但 argparse 的**别样形态**不在承诺内 ——
//	  ②③ 之外还有：(a) **前缀缩写**（`--sel` 会被 py 收成 `--self-test`、本件按未知旗标拒）；
//	  (b) `--root -h` 的**取值歧义**（py 报 `expected one argument`、本件把 `-h` 当根值）；
//	  (c) `-h` 与未知旗标**同现**时的先后（py 见 `-h` 即出帮助退出 0）。
//	② **`__file__` → cwd 根定位差异**：py「不给 `--root`」时默认根 = 由 `__file__`（`scripts/gates`）上溯两层，
//	  与 **cwd 无关**；Go 入口**无 `__file__` 语境**（未来接线后是 CLI 子命令）⇒ 本件默认从 **cwd** 上溯 ≤6 层
//	  找含 `scripts/gates/mutation-smoke.sh` 的目录、找不到回落 cwd。两者在**同一真仓 + cwd=仓根**时读数逐字相同
//	  （已对拍）；「脚本位置 ≠ cwd」或「夹具根 ≠ 真仓」时**路径推导面不同** ⇒ 对拍一律**显式传根**。
//	③ **异常路径**：真源件 / 靶件存在但**读不到**（权限）或**不是 UTF-8**（py `open(...,encoding="utf-8")`
//	  抛 `UnicodeDecodeError`）⇒ py 的 `OSError` 文案是 `[Errno 13] Permission denied: '…'` 那一形、
//	  非 UTF-8 更会**抛 traceback**；本件落「读不出（…）」行，**退码语义同（判不了 ⇒ 2 / 读件失败即 2）**
//	  但**正文不同**。`--list` 的 stdout 非 UTF-8 时 py 走 `decode(...,"replace")` 插 U+FFFD、本件按字节透传。
//	  起不来（`bash` 不在 PATH）一档：py 报 `起不来：<OSError 文案>`、本件报 `起不来：<exec 文案>` ⇒ 正文不算逐字。
//	④ **自检手段**：py `--self-test` 用 `tempfile.mkdtemp()`（随 `$TMPDIR`）造合成树、**进程内**调 `judge()`；
//	  本件同法（`os.MkdirTemp("", …)` 同随 `$TMPDIR`）⇒ 打印文案逐字对齐（已对拍）；差只在**实现手段**
//	  （Go 无 `assert_eq` 的 `global` 计数、改闭包计数），**不在读数**。
//	⑤ `--list` 的**子进程面**：py `subprocess.run(["bash", truth, "--list"], cwd=root)`；本件 `exec.Command`
//	  同形同 cwd。若 PATH 上的 `bash` 与 py 认的不是同一件、或靶件树里 `bash` 行为不同 ⇒ **不在**承诺内
//	  （对拍的两态共用同一台机的同一个 `bash`）。
//
//	—— ②③④⑤ 五面均**不在** M2-0 的样板对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ── 判据常量（与 py 逐条对齐 · 名字前缀 `mut` 防包内同名）─────────────────────
const mutTruthRel = "scripts/gates/mutation-smoke.sh"

// mutArrayNames —— py `ARRAYS`（序即 py 的遍历序）。
var mutArrayNames = []string{"MUT_FILE", "MUT_OLD", "MUT_NEW", "MUT_TEST", "MUT_NAME"}

// mutHeaderPrefix —— py `HEADER_PREFIX`（`--list` 首行的机读表头前缀）。
const mutHeaderPrefix = "#"

// mutNeedCols —— py `NEED_COLS`（本件**按列名**取数 · 多出来的列一律忽略）。
var mutNeedCols = []string{"点号", "名", "件", "点名判据"}

// mutStrRE —— py `_STR_RE` = `^"((?:[^"\\]|\\.)*)"$`（bash 数组里的双引号字面量）。
var mutStrRE = regexp.MustCompile(`^"((?:[^"\\]|\\.)*)"$`)

// ── argparse 面（逐字取自 py 现跑 · 唯一真源 = 原件 py 自身的 argparse 输出）──────
//
//	mutProg      = os.path.basename(sys.argv[0])（本仓正门的调用形 = `… /check-mutation-residue.py`
//	               ⇒ 恒为该件名，与 py 逐字同）
//	mutHelpBody  = `python3 <原件> --help` 的 stdout（argparse 生成 · 逐字节内嵌）
const mutProg = "check-mutation-residue.py"

// mutUsageLine —— argparse 用法行（= mutHelpBody 的首行）。
const mutUsageLine = "usage: check-mutation-residue.py [-h] [--root ROOT] [--self-test]"

// mutHelpBody —— py `-h/--help` 的 stdout（逐字节）。
const mutHelpBody = `usage: check-mutation-residue.py [-h] [--root ROOT] [--self-test]

optional arguments:
  -h, --help   show this help message and exit
  --root ROOT
  --self-test
`

// mutUsageError —— py argparse 的用法错（stderr 两行：用法行 + `<件名>: error: <msg>`），rc=2。
func mutUsageError(stderr io.Writer, msg string) {
	fmt.Fprintf(stderr, "%s\n%s: error: %s\n", mutUsageLine, mutProg, msg)
}

// ── 入口（= py `main()`）────────────────────────────────────────────────────
func mutationResidueGateMain(stdout, stderr io.Writer, argv []string) int {
	root, selfTest, stop := mutParseArgs(stdout, stderr, argv)
	if stop != nil {
		return *stop
	}
	if selfTest {
		return mutSelfTest(stdout)
	}
	if root == "" {
		root = mutDefaultRoot()
	}
	root = mutAbs(root)
	rc, lines := mutJudge(root, filepath.Join(root, mutTruthRel))
	for _, ln := range lines {
		fmt.Fprintln(stdout, ln)
	}
	return rc
}

// mutParseArgs —— argparse 口径（本件认的旗标：`-h/--help` · `--root[=| ]值` · `--self-test`）。
// 未知项**收集到底**、按发现序一条报（与 argparse `unrecognized arguments: …` 逐字同形）。
func mutParseArgs(stdout, stderr io.Writer, argv []string) (string, bool, *int) {
	root := ""
	selfTest := false
	var unknown []string
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			// py argparse：见 `-h` 即印帮助到 stdout 并以 0 退出。
			io.WriteString(stdout, mutHelpBody)
			rc := 0
			return "", false, &rc
		case a == "--self-test":
			selfTest = true
		case a == "--root":
			if i+1 >= len(argv) {
				mutUsageError(stderr, "argument --root: expected one argument")
				rc := 2
				return "", false, &rc
			}
			i++
			root = argv[i]
		case strings.HasPrefix(a, "--root="):
			root = a[len("--root="):]
		default:
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		mutUsageError(stderr, "unrecognized arguments: "+strings.Join(unknown, " "))
		rc := 2
		return "", false, &rc
	}
	return root, selfTest, nil
}

// mutDefaultRoot —— 「不给 `--root`」时的默认根。
//
//	py：`abspath(dirname(abspath(__file__)) + "/../..")`（= 脚本所在 `scripts/gates` 上溯两层，**与 cwd 无关**）。
//	本件：**无 `__file__` 语境** ⇒ 从 cwd 上溯 ≤6 层找含真源件的目录，找不到回落 cwd（见件头未覆盖面②）。
func mutDefaultRoot() string {
	start, err := os.Getwd()
	if err != nil {
		return "."
	}
	d, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	for i := 0; i < 6; i++ {
		if mutIsFile(filepath.Join(d, mutTruthRel)) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// mutAbs —— py `os.path.abspath`（规范化，**不**解符号链接）。
func mutAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}

// mutUnescape —— py `_unescape`：先 `\"` → `"`，再 `\\` → `\`（**序不可换**）。
func mutUnescape(s string) string {
	s = strings.ReplaceAll(s, `\"`, `"`)
	return strings.ReplaceAll(s, `\\`, `\`)
}

// mutParseArrays —— py `parse_arrays`：从真源件源码读五个索引数组；任何一处读不出 ⇒ false（判不了 · 不猜）。
func mutParseArrays(src string) (map[string][]string, bool) {
	out := map[string][]string{}
	for _, name := range mutArrayNames {
		re := regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(name) + `=\(\s*$(.*?)^\)\s*$`)
		m := re.FindStringSubmatch(src)
		if m == nil {
			return nil, false
		}
		var items []string
		for _, line := range mutSplitLines(m[1]) {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			mm := mutStrRE.FindStringSubmatch(line)
			if mm == nil {
				return nil, false
			}
			items = append(items, mutUnescape(mm[1]))
		}
		out[name] = items
	}
	n := len(out["MUT_FILE"])
	if n == 0 {
		return nil, false
	}
	for _, name := range mutArrayNames {
		if len(out[name]) != n {
			return nil, false
		}
	}
	return out, true
}

// mutListTable —— py `read_list` 的返回值（行 · 表头列名）。
type mutListTable struct {
	rows [][]string
	cols []string
}

// mutReadList —— py `read_list`：`bash <真源> --list` 现读；按**列名**定位（加列不破本件）。
// 第二个返回值 = 错误说明（空串 = 成功）。
func mutReadList(root, truth string) (*mutListTable, string) {
	cmd := exec.Command("bash", truth, "--list")
	cmd.Dir = root
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Sprintf("--list rc=%d", ee.ExitCode())
		}
		return nil, fmt.Sprintf("起不来：%v", err)
	}
	var lines []string
	for _, raw := range mutSplitLines(so.String()) {
		if strings.TrimSpace(raw) != "" {
			lines = append(lines, raw)
		}
	}
	if len(lines) == 0 {
		return nil, "--list 空表（0 点）"
	}
	head := lines[0]
	if !strings.HasPrefix(head, mutHeaderPrefix) {
		return nil, fmt.Sprintf("--list 首行不是表头（%s 打头的列名行不在）⇒ 列契约读不出", mutHeaderPrefix)
	}
	cols := strings.Split(head[len(mutHeaderPrefix):], "\t")
	bad := false
	for _, c := range cols {
		if c == "" {
			bad = true
		}
	}
	if mutUniqueLen(cols) != len(cols) {
		bad = true
	}
	if bad {
		return nil, fmt.Sprintf("--list 表头的列名有空格 / 重复（%s）", head)
	}
	var missing []string
	for _, c := range mutNeedCols {
		if mutIndex(cols, c) < 0 {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Sprintf("--list 表头缺本件要的列：%s（表头现读 %s）", strings.Join(missing, "，"), head)
	}
	idx := map[string]int{}
	for _, c := range mutNeedCols {
		idx[c] = mutIndex(cols, c)
	}
	var rows [][]string
	for _, raw := range lines[1:] {
		f := strings.Split(raw, "\t")
		if len(f) != len(cols) {
			return nil, fmt.Sprintf("--list 某行列数 %d ≠ 表头列数 %d", len(f), len(cols))
		}
		row := make([]string, 0, len(mutNeedCols))
		for _, c := range mutNeedCols {
			row = append(row, f[idx[c]])
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, "--list 空表（0 点 · 只有表头）"
	}
	return &mutListTable{rows: rows, cols: cols}, ""
}

// mutBadPoint —— 点名行（py 里 `bad` 收 `(i, rel)`、`bad2` 只收 `rel`）。
type mutBadPoint struct {
	i   int
	rel string
}

// mutJudge —— py `judge`：返回 (rc, 输出行)。rc：0 全原形 · 1 有残态 · 2 判不了。
func mutJudge(root, truth string) (int, []string) {
	var out []string
	out = append(out, "== 门72 变异残态（只读）· 仓根 = "+root)
	if !mutIsFile(truth) {
		out = append(out, fmt.Sprintf("判不了：真源件不在（%s）⇒ 突变点表读不出 ⇒ 不给结论（rc=2）", mutTruthRel))
		return 2, out
	}
	srcB, err := os.ReadFile(truth)
	if err != nil {
		out = append(out, fmt.Sprintf("判不了：真源件读不出（%s）⇒ 不给结论（rc=2）", err))
		return 2, out
	}
	arrays, ok := mutParseArrays(string(srcB))
	if !ok {
		out = append(out, "判不了：真源件的五个索引数组读不出 / 条数不齐 / 0 点 ⇒ 不给结论（rc=2）")
		return 2, out
	}
	tab, errDesc := mutReadList(root, truth)
	if tab == nil {
		out = append(out, fmt.Sprintf("判不了：`--list` 不可解析（%s）⇒ 不给结论（rc=2）", errDesc))
		return 2, out
	}
	n := len(arrays["MUT_FILE"])
	out = append(out, fmt.Sprintf(
		"== 突变点表 = %s（`--list` 现读 %d 点 · 表头 %d 列：%s · 本件按名取 %d 列 · 原文锚同件源码现读 %d 点）",
		mutTruthRel, len(tab.rows), len(tab.cols), strings.Join(tab.cols, " / "), len(mutNeedCols), n))
	if len(tab.rows) != n {
		out = append(out, fmt.Sprintf("判不了：两读条数不等（--list %d ≠ 源码数组 %d）⇒ 不给结论（rc=2）",
			len(tab.rows), n))
		return 2, out
	}
	for i, f := range tab.rows {
		if f[0] != strconv.Itoa(i) || f[1] != arrays["MUT_NAME"][i] || f[2] != arrays["MUT_FILE"][i] ||
			f[3] != arrays["MUT_TEST"][i] {
			out = append(out, fmt.Sprintf(
				"判不了：两读第 %s 点对不上（--list 名/件/判据 ≠ 源码数组）⇒ 不给结论（rc=2）", strconv.Itoa(i)))
			return 2, out
		}
	}

	var bad []mutBadPoint
	var bad2 []string
	for i := range tab.rows {
		rel := arrays["MUT_FILE"][i]
		old := arrays["MUT_OLD"][i]
		newv := arrays["MUT_NEW"][i]
		path := filepath.Join(root, rel)
		if old == "" {
			out = append(out, fmt.Sprintf("%d\t判不了\t%s\t原文锚为空 ⇒ 判不了", i, rel))
			bad2 = append(bad2, rel)
			continue
		}
		if !mutIsFile(path) {
			out = append(out, fmt.Sprintf("%d\t判不了\t%s\t靶件不在（%s）⇒ 判不了", i, rel, "缺件"))
			bad2 = append(bad2, rel)
			continue
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			out = append(out, fmt.Sprintf("%d\t判不了\t%s\t读不出（%s）⇒ 判不了", i, rel, rerr))
			bad2 = append(bad2, rel)
			continue
		}
		hits := strings.Count(string(body), old)
		if hits == 1 {
			out = append(out, fmt.Sprintf("%d\t原形\t%s\t原文锚命中 1 处 ✓（%s）", i, rel, arrays["MUT_NAME"][i]))
		} else if hits == 0 {
			if newv != "" && strings.Contains(string(body), newv) {
				out = append(out, fmt.Sprintf("%d\t畸形\t%s\t原文锚命中 0 处 · 检出改后值「%s」⇒ 残态（%s）",
					i, rel, newv, arrays["MUT_NAME"][i]))
			} else {
				out = append(out, fmt.Sprintf("%d\t畸形\t%s\t原文锚命中 0 处 · 改后值也不在（含锚整行被删）⇒ 残态（%s）",
					i, rel, arrays["MUT_NAME"][i]))
			}
			bad = append(bad, mutBadPoint{i: i, rel: rel})
		} else {
			out = append(out, fmt.Sprintf("%d\t判不了\t%s\t原文锚命中 %d 处（应为 1）⇒ 歧义 ⇒ 判不了", i, rel, hits))
			bad2 = append(bad2, rel)
		}
	}

	if len(bad) > 0 {
		out = append(out, "✗ 残态点名（工作树里躺着未复原的突变）：")
		for _, b := range bad {
			out = append(out, fmt.Sprintf("    · %s（第 %d 点 · %s）", b.rel, b.i, arrays["MUT_NAME"][b.i]))
		}
		out = append(out, fmt.Sprintf("⇒ 有 %d 点残态（rc=1）：本读数有牙 —— 残态即判红，不许当绿", len(bad)))
		return 1, out
	}
	if len(bad2) > 0 {
		out = append(out, fmt.Sprintf("⇒ 有 %d 点判不了（rc=2 · 不给结论 ≠ 绿）：%s", len(bad2), strings.Join(bad2, " ")))
		return 2, out
	}
	out = append(out, fmt.Sprintf("⇒ 全 %d 点**原形**（rc=0）：工作树里没有未复原的突变", n))
	return 0, out
}

// ── 自检（合成树 · `$TMPDIR` 下 · 不碰真目标）────────────────────────────────────
//
//	与 py `run_self_test()` 逐格对齐：11 例 · 19 条断言 · 行为三档（0/1/2）全覆盖。
//	⚠ 夹具件（SYNTH / SYNTH_ADDCOL / SYNTH_DROPCOL / 四个派生脚本 / GOOD_A / GOOD_B）**逐字节**取自
//	pi 原文的运行期取值（非重写）⇒ 判据面与 py 同形。
func mutSelfTest(stdout io.Writer) int {
	total, bad := 0, 0
	assertEq := func(desc, actual, expected string) {
		total++
		if actual == expected {
			fmt.Fprintf(stdout, "   ✓ %s\n", desc)
			return
		}
		bad++
		fmt.Fprintf(stdout, "   ✗ %s（实际=%s 期望=%s）\n", desc, actual, expected)
	}
	it := strconv.Itoa

	fmt.Fprintln(stdout, "── 门72 自检（合成件 · $TMPDIR 下 · 不碰真目标）──")

	tmp, err := os.MkdirTemp("", "mutresidue-selftest-")
	if err != nil {
		tmp = os.TempDir()
	}
	defer os.RemoveAll(tmp)

	// _mktree：合成一棵含真源件 + a.go + b.go 的树；script 空 ⇒ 用 mutSynth（= py 的 script=None）。
	mk := func(name, aBody, bBody, script string) string {
		root := filepath.Join(tmp, name)
		_ = os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755)
		sc := script
		if sc == "" {
			sc = mutSynth
		}
		_ = os.WriteFile(filepath.Join(root, mutTruthRel), []byte(sc), 0o644)
		_ = os.WriteFile(filepath.Join(root, "a.go"), []byte(aBody), 0o644)
		_ = os.WriteFile(filepath.Join(root, "b.go"), []byte(bBody), 0o644)
		return root
	}
	// count：py 的 `sum(1 for ln in lines if <子串> in ln)`。
	count := func(lines []string, sub string) int {
		c := 0
		for _, ln := range lines {
			if strings.Contains(ln, sub) {
				c++
			}
		}
		return c
	}

	// ① 全原形 ⇒ rc=0
	r1 := mk("ok", mutGoodA, mutGoodB, "")
	rc1, lines1 := mutJudge(r1, filepath.Join(r1, mutTruthRel))
	assertEq("① 合成树全原形 ⇒ rc=0", it(rc1), "0")
	assertEq("① 输出逐点报「原形」= 2 行", it(count(lines1, "\t原形\t")), "2")

	// ② 改后值在（0.8 → 0.9）⇒ rc=1 且**点名那一件**
	r2 := mk("mut-new", mutGoodA, strings.Replace(mutGoodB, "0.8", "0.9", -1), "")
	rc2, lines2 := mutJudge(r2, filepath.Join(r2, mutTruthRel))
	assertEq("② 换值型残态 ⇒ rc=1", it(rc2), "1")
	assertEq("② 畸形行数 = 1", it(count(lines2, "\t畸形\t")), "1")
	assertEq("② 逐条点名的是 b.go", it(count(lines2, "· b.go（第 1 点")), "1")

	// ③ 删行型残态（NEW 为空）⇒ rc=1 且点名 a.go
	r3 := mk("mut-del", "func main() {\n}\n", mutGoodB, "")
	rc3, lines3 := mutJudge(r3, filepath.Join(r3, mutTruthRel))
	assertEq("③ 删行型残态 ⇒ rc=1", it(rc3), "1")
	assertEq("③ 逐条点名的是 a.go", it(count(lines3, "· a.go（第 0 点")), "1")

	// ④ 锚多命中（歧义）⇒ rc=2（判不了 ≠ 绿）
	r4 := mk("ambig", mutGoodA+"adp.Start(); err != nil {\n", mutGoodB, "")
	rc4, _ := mutJudge(r4, filepath.Join(r4, mutTruthRel))
	assertEq("④ 原文锚多命中 ⇒ 判不了 rc=2（不给结论）", it(rc4), "2")

	// ⑤ 靶件不在 ⇒ rc=2
	r5 := mk("missing", mutGoodA, mutGoodB, "")
	_ = os.Remove(filepath.Join(r5, "a.go"))
	rc5, _ := mutJudge(r5, filepath.Join(r5, mutTruthRel))
	assertEq("⑤ 靶件不在 ⇒ 判不了 rc=2", it(rc5), "2")

	// ⑥ 两读对不上（源码数组与 --list 的输出不一致）⇒ rc=2
	r6 := mk("drift", mutGoodA, mutGoodB, mutScriptDrift)
	rc6, _ := mutJudge(r6, filepath.Join(r6, mutTruthRel))
	assertEq("⑥ 表漂了（源码数组 ≠ --list）⇒ 判不了 rc=2", it(rc6), "2")

	// ⑦ 真源件缺 ⇒ rc=2
	r7 := mk("notruth", mutGoodA, mutGoodB, "")
	_ = os.Remove(filepath.Join(r7, mutTruthRel))
	rc7, _ := mutJudge(r7, filepath.Join(r7, mutTruthRel))
	assertEq("⑦ 真源件不在 ⇒ 判不了 rc=2", it(rc7), "2")

	// ⑧ --list 行与表头不对齐（行列数不足）⇒ rc=2
	r8 := mk("badlist", mutGoodA, mutGoodB, mutScriptBadlist)
	rc8, _ := mutJudge(r8, filepath.Join(r8, mutTruthRel))
	assertEq("⑧ `--list` 行与表头不对齐 ⇒ 判不了 rc=2", it(rc8), "2")

	// ⑨ ★ 加一列（右端 · 既有四格逐字不变）⇒ **必须 rc=0**（缺口 GAP-20260928-116 的核心判据：
	//   按列名取数之后，生产侧长列不再把下游打成 rc=2）
	addOK := mutSynthAddcol != mutSynth &&
		strings.Count(mutSynthAddcol, `\t%s`) == strings.Count(mutSynth, `\t%s`)+1
	assertEq("⑨ 夹具 SYNTH_ADDCOL 真的加上了第五列（替换生效）", mutPyBool(addOK), "True")
	r9 := mk("addcol", mutGoodA, mutGoodB, mutSynthAddcol)
	rc9, lines9 := mutJudge(r9, filepath.Join(r9, mutTruthRel))
	assertEq("⑨ 生产侧右端加一列 ⇒ rc=0（加列不破本件）", it(rc9), "0")
	assertEq("⑨ 加列后仍逐点报「原形」= 2 行",
		it(count2(lines9, "原形", "原文锚命中 1 处")), "2")

	// ⑩ 缺本件要的那一列（表头「点名判据」没了）⇒ rc=2（不给结论 ≠ 绿）
	assertEq("⑩ 夹具 SYNTH_DROPCOL 真的去掉了要的那一列（替换生效）",
		mutPyBool(mutSynthDropcol != mutSynth), "True")
	r10 := mk("dropcol", mutGoodA, mutGoodB, mutSynthDropcol)
	rc10, lines10 := mutJudge(r10, filepath.Join(r10, mutTruthRel))
	assertEq("⑩ 表头缺「点名判据」列 ⇒ 判不了 rc=2", it(rc10), "2")
	assertEq("⑩ 报的是「缺列」不是别的",
		it(count(lines10, "缺本件要的列：点名判据")), "1")

	// ⑪ 表头整行不在（老式无表头的 `--list`）⇒ rc=2（列契约读不出，不猜）
	r11 := mk("noheader", mutGoodA, mutGoodB, mutScriptNoheader)
	rc11, _ := mutJudge(r11, filepath.Join(r11, mutTruthRel))
	assertEq("⑪ `--list` 无表头 ⇒ 判不了 rc=2", it(rc11), "2")

	fmt.Fprintf(stdout, "── 自检结论：断言 %d 条 · 失败 %d 条 ──\n", total, bad)
	if bad == 0 {
		return 0
	}
	return 1
}

// count2 —— py 的 `sum(1 for ln in lines if A in ln and B in ln)`。
func count2(lines []string, a, b string) int {
	c := 0
	for _, ln := range lines {
		if strings.Contains(ln, a) && strings.Contains(ln, b) {
			c++
		}
	}
	return c
}

// mutPyBool —— py `str(True/False)` 口径。
func mutPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// mutSplitLines —— py `str.splitlines()` 口径（\n · \r\n · \r · \v · \f · \x85 · \u2028 · \u2029）。
func mutSplitLines(s string) []string {
	var out []string
	var cur []rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '\r' {
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			out = append(out, string(cur))
			cur = nil
			continue
		}
		if r == '\n' || r == '\v' || r == '\f' || r == 0x85 || r == 0x2028 || r == 0x2029 {
			out = append(out, string(cur))
			cur = nil
			continue
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// mutIsFile —— py `os.path.isfile`（跟随符号链接）。
func mutIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// mutUniqueLen —— 去重后长度（py `len(set(cols))`）。
func mutUniqueLen(in []string) int {
	seen := map[string]bool{}
	for _, s := range in {
		seen[s] = true
	}
	return len(seen)
}

// mutIndex —— py `cols.index(c)`（缺席 ⇒ -1，调用处已先查 missing）。
func mutIndex(in []string, want string) int {
	for i, s := range in {
		if s == want {
			return i
		}
	}
	return -1
}

// ── 自检夹具（**逐字节**取自 py 运行期取值 · 与原件同形）──────────────────────────
const mutSynth = `#!/usr/bin/env bash
MUT_FILE=(
  "a.go"
  "b.go"
)
MUT_OLD=(
  "adp.Start(); err != nil {"
  "Temperature:     0.8,"
)
MUT_NEW=(
  ""
  "Temperature:     0.9,"
)
MUT_TEST=(
  "go test ./x/ -run T1"
  "go test ./x/ -run T2"
)
MUT_NAME=(
  "删 a.go 的 adp.Start() 那一行"
  "b.go 0.8 → 0.9"
)
MUT_COLS=(
  "点号"
  "名"
  "件"
  "点名判据"
)
if [ "$1" = "--list" ]; then
  _SAVE="$IFS"; IFS=$'\t'
  printf '#%s\n' "${MUT_COLS[*]}"
  IFS="$_SAVE"
  i=0
  while [ "$i" -lt 2 ]; do
    printf '%d\t%s\t%s\t%s\n' "$i" "${MUT_NAME[$i]}" "${MUT_FILE[$i]}" "${MUT_TEST[$i]}"
    i=$((i + 1))
  done
  exit 0
fi
exit 0
`

// mutSynthAddcol —— py `SYNTH_ADDCOL`（生产侧右端加第五列 · 既有四格逐字不变）。
const mutSynthAddcol = `#!/usr/bin/env bash
MUT_FILE=(
  "a.go"
  "b.go"
)
MUT_OLD=(
  "adp.Start(); err != nil {"
  "Temperature:     0.8,"
)
MUT_NEW=(
  ""
  "Temperature:     0.9,"
)
MUT_TEST=(
  "go test ./x/ -run T1"
  "go test ./x/ -run T2"
)
MUT_NAME=(
  "删 a.go 的 adp.Start() 那一行"
  "b.go 0.8 → 0.9"
)
MUT_COLS=(
  "点号"
  "名"
  "件"
  "点名判据"
  "备注"
)
if [ "$1" = "--list" ]; then
  _SAVE="$IFS"; IFS=$'\t'
  printf '#%s\n' "${MUT_COLS[*]}"
  IFS="$_SAVE"
  i=0
  while [ "$i" -lt 2 ]; do
    printf '%d\t%s\t%s\t%s\t%s\n' "$i" "${MUT_NAME[$i]}" "${MUT_FILE[$i]}" "${MUT_TEST[$i]}" "第 $i 点备注"
    i=$((i + 1))
  done
  exit 0
fi
exit 0
`

// mutSynthDropcol —— py `SYNTH_DROPCOL`（表头去掉本件要的「点名判据」列）。
const mutSynthDropcol = `#!/usr/bin/env bash
MUT_FILE=(
  "a.go"
  "b.go"
)
MUT_OLD=(
  "adp.Start(); err != nil {"
  "Temperature:     0.8,"
)
MUT_NEW=(
  ""
  "Temperature:     0.9,"
)
MUT_TEST=(
  "go test ./x/ -run T1"
  "go test ./x/ -run T2"
)
MUT_NAME=(
  "删 a.go 的 adp.Start() 那一行"
  "b.go 0.8 → 0.9"
)
MUT_COLS=(
  "点号"
  "名"
  "件"
  "别的东西"
)
if [ "$1" = "--list" ]; then
  _SAVE="$IFS"; IFS=$'\t'
  printf '#%s\n' "${MUT_COLS[*]}"
  IFS="$_SAVE"
  i=0
  while [ "$i" -lt 2 ]; do
    printf '%d\t%s\t%s\t%s\n' "$i" "${MUT_NAME[$i]}" "${MUT_FILE[$i]}" "${MUT_TEST[$i]}"
    i=$((i + 1))
  done
  exit 0
fi
exit 0
`

// mutScriptDrift —— py 自检⑥ 的夹具（`--list` 的名字列被改成「改过的名字」⇒ 两读对不上）。
const mutScriptDrift = `#!/usr/bin/env bash
MUT_FILE=(
  "a.go"
  "b.go"
)
MUT_OLD=(
  "adp.Start(); err != nil {"
  "Temperature:     0.8,"
)
MUT_NEW=(
  ""
  "Temperature:     0.9,"
)
MUT_TEST=(
  "go test ./x/ -run T1"
  "go test ./x/ -run T2"
)
MUT_NAME=(
  "删 a.go 的 adp.Start() 那一行"
  "b.go 0.8 → 0.9"
)
MUT_COLS=(
  "点号"
  "名"
  "件"
  "点名判据"
)
if [ "$1" = "--list" ]; then
  _SAVE="$IFS"; IFS=$'\t'
  printf '#%s\n' "${MUT_COLS[*]}"
  IFS="$_SAVE"
  i=0
  while [ "$i" -lt 2 ]; do
    printf '%d\t%s\t%s\t%s\n' "$i" "改过的名字" "${MUT_FILE[$i]}" "${MUT_TEST[$i]}"
    i=$((i + 1))
  done
  exit 0
fi
exit 0
`

// mutScriptBadlist —— py 自检⑧ 的夹具（`--list` 行列数不足 ⇒ 与表头不对齐）。
const mutScriptBadlist = `#!/usr/bin/env bash
MUT_FILE=(
  "a.go"
  "b.go"
)
MUT_OLD=(
  "adp.Start(); err != nil {"
  "Temperature:     0.8,"
)
MUT_NEW=(
  ""
  "Temperature:     0.9,"
)
MUT_TEST=(
  "go test ./x/ -run T1"
  "go test ./x/ -run T2"
)
MUT_NAME=(
  "删 a.go 的 adp.Start() 那一行"
  "b.go 0.8 → 0.9"
)
MUT_COLS=(
  "点号"
  "名"
  "件"
  "点名判据"
)
if [ "$1" = "--list" ]; then
  _SAVE="$IFS"; IFS=$'\t'
  printf '#%s\n' "${MUT_COLS[*]}"
  IFS="$_SAVE"
  i=0
  while [ "$i" -lt 2 ]; do
    printf '%d\t%s\n' "$i" "${MUT_NAME[$i]}" "${MUT_FILE[$i]}" "${MUT_TEST[$i]}"
    i=$((i + 1))
  done
  exit 0
fi
exit 0
`

// mutScriptNoheader —— py 自检⑪ 的夹具（表头整行被抹掉）。
const mutScriptNoheader = `#!/usr/bin/env bash
MUT_FILE=(
  "a.go"
  "b.go"
)
MUT_OLD=(
  "adp.Start(); err != nil {"
  "Temperature:     0.8,"
)
MUT_NEW=(
  ""
  "Temperature:     0.9,"
)
MUT_TEST=(
  "go test ./x/ -run T1"
  "go test ./x/ -run T2"
)
MUT_NAME=(
  "删 a.go 的 adp.Start() 那一行"
  "b.go 0.8 → 0.9"
)
MUT_COLS=(
  "点号"
  "名"
  "件"
  "点名判据"
)
if [ "$1" = "--list" ]; then
  _SAVE="$IFS"; IFS=$'\t'
    IFS="$_SAVE"
  i=0
  while [ "$i" -lt 2 ]; do
    printf '%d\t%s\t%s\t%s\n' "$i" "${MUT_NAME[$i]}" "${MUT_FILE[$i]}" "${MUT_TEST[$i]}"
    i=$((i + 1))
  done
  exit 0
fi
exit 0
`

// mutGoodA / mutGoodB —— py `GOOD_A` / `GOOD_B`。
const mutGoodA = `func main() {
	if err := adp.Start(); err != nil {
		panic(err)
	}
}
`

const mutGoodB = `cfg := Config{
	Temperature:     0.8,
}
`
