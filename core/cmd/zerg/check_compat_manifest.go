// check_compat_manifest.go —— 门禁件 `scripts/gates/check-compat-manifest.py` 的 **Go 真搬**（v2.5.14 · M2 路A（a–e 批）第六枚）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值真搬 Go（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据 · 设计 v1.0 §4.1）：验收判据 = 对同一件跑两态
//	（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（§4.2）。
//
// 选它的理由（对照 M2-0 六条硬口径）：
//
//	✓ **无外部树依赖**：只读仓内件 —— `core/internal/compat/compat.json`（判据主体）、
//	  `core/internal/compat/migrations.go`（R2 入口面）、表里 `anchor_file` 点名的仓内件（R3）、
//	  `core/**` 与 `agent/**` 下的 `.go` 件（R4/R6 发现面）、`ui/**` 下的 `.rs` 件（R4 发现面）。
//	  **不碰**文档仓 / 公开树 / 状态目录 / 生成物。
//	✓ **输出确定有序**：发现面 `source_files()` 结尾 `sorted(out)`；R4 逐名 `sorted(...)`；
//	  R1/R2/R3/R5 按 JSON 数组序；R6 按清单 `dirs` 的**文档序**（本件另读一遍 token 序保住，
//	  ✗ 不用 Go map 的随机序）⇒ 逐字节对拍可证。
//	✓ **退码 0/1/2 三档齐**：0 清单 ↔ 代码双向一致 / 1 有红（或根定位不到）/ 2 用法错（`--json` 无机器读面）。
//	✓ **纯静态无写面**：main 路径一个字节都不写（`--self-test` 只在 `os.MkdirTemp` 里造夹具）。
//	✓ **行数适中**（py 449 行）· **无 subprocess / 无 git / 无 datetime.now / 无网络**。
//	✓ **无人正在改**：`git status --porcelain` 现读**不含** `scripts/gates/check-compat-manifest.py`
//	  （也**不含**它读的 `core/internal/compat/compat.json` / `migrations.go`）。
//
// 不选的理由（逐件 · 本单现读后自定）：
//
//	✗ `check-anchor-uniqueness.py`（340）—— 登记件在**仓外** `Zerg-内部文档/`（外部树依赖）⇒ 不属 M2-0。
//	✗ `check-curl-timeout.py`（167）—— 扫描根靠 `os.walk`，**目录进场序非确定**（py `scandir` inode 序
//	  ≠ Go `WalkDir` 字典序）⇒ 对拍假差（同批已由前几单逐条否掉）。
//	✗ `check-dryrun-semantics.py`（349）—— 判据面靠 `subprocess` 现跑 `grep`（外部进程）；且它读的
//	  `core/internal/contract/dryrun-semantics.json` 是**登记面且正在被另一路改**（`git status` 现读）。
//	✗ `check-affected-map.py`（523）—— 带 `--emit` **写面**（就地生成 `scripts/gates/affected-map.json`）
//	  ⇒ 非「纯静态」；且要读 `precommit-gates.sh` 的 `add_step` 行（共享件）。
//	✗ `check-approve-tier.py`（460）/ `check-artifact-source-parity.py`（476）—— 现跑 `bin/zerg`
//	  制品（制品依赖 · 本批禁碰 `bin/`；门74 预期红）。
//	✗ `check-approve-time.py`（398）/ `check-contract-version-diff.py`（434）—— 读 `~/.zerg/state/**`
//	  与 `git log`（状态目录 / 外部进程 · 非纯静态）。
//	✗ `check-approve-ci.py`（414）/ `check-approve-closedset.py`（407）/ `check-approve-freeze.py`（335）
//	  / `check-archive-mount.py`（442）/ `check-dangling-refs.py`（734）—— **另一路在改**（`git status` 现读）。
//	✗ `check-build-tags.py`（863）/ `check-cli-contract.py`（1283）/ `check-approve-merge.py`（838）
//	  / `check-approve-backfill.py`（683）/ `check-exit-codes.py`（649）—— **已列重件 / 越限**。
//	✗ 8 件 `doc-*` / `check-docs-orphans-links.py` / `check-affected-reprove.py` —— 共享件 / **另一路在改**。
//
// ⚠ 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3418`（门㊺ · tri 档））；
//	· **不**删 `scripts/gates/check-compat-manifest.py` 原件（退场规则 §4.3：**对拍通过后才**退场）；
//	· **已接线**：入口 `compatManifestGateMain` 由 `gateCheckGoEntry["check-compat-manifest"]` 分派（`zerg gate check check-compat-manifest`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func compatManifestGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 一致 / 1 有红 / 2 用法错（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ★★ 两条标准声明行（M2 每件必写 · 本单逐条给出依据）：
//
//	(a) **默认根定位差异**：py 侧 `main()` 的末档是 `os.path.dirname(os.path.abspath(__file__))`
//	  —— 即**脚本所在目录**（`scripts/gates/`）起，`find_root` 上溯 ≤8 层；本件**无 `__file__` 语境**
//	  （未来接线后是 CLI 子命令）⇒ 末档换成**当前工作目录**（`os.Getwd()`）起上溯 ≤8 层。
//	  落在**同一真仓、cwd=仓根**（或 cwd=仓内任一层，如 `core/cmd/zerg`）时两者读数逐字相同
//	  （已对拍 C1/C2/C13）；「脚本位置 ≠ cwd」时**路径推导面不同**（如 cwd 在仓外且不给位置参数）。
//
//	(b) **自指检查要真判 —— 本件**在**自己的扫描面内（判「是」· 依据如下）**：
//	  R4 的发现面 = `source_files(root, ['.go'], ['core','agent'])` ＝ **`core/**/*.go` 全体**（含
//	  `_test.go`，含 `core/cmd/zerg/`）。本件落点 `core/cmd/zerg/check_compat_manifest.go` **就在这条面里**
//	  ⇒ 本件源码里若出现**能被 GO_PATTERNS 命中的字面量**，R4 就会把本件自己判成「引用了未登记状态文件」
//	  ⇒ 真仓现读由绿转红（这就是「自指」的真身）。
//	  **反制（血证形态）**：`--self-test` 的两条 R4 夹具串（`statepath.File(...)`）一律写成
//	  **转义双引号形态**（`statepath.File(\"…\")`，见 `cmProbeGoFlat` / `cmProbeGoNested`）—— 磁盘正文里
//	  `(` 之后是反斜杠而非引号，GO_PATTERNS 的 `\(\s*"` 段**命中不了**；✗ **禁用反引号原串**写这类夹具
//	  （原串会把 `statepath.File(` 后紧跟**真引号**的形态逐字落进本件正文 ⇒ 当场自伤 —— 本单第一发
//	  正是被**件头这句话自己**扫红的：真仓现读报「状态文件 '…'（被 core/cmd/zerg/check_compat_manifest.go
//	  引用）未登记」⇒ 当场改成「不落该字面」的写法，再看真仓现读回到 rc=0）。
//	  同理，本件登记 GO_PATTERNS 时用的是正则**转义形态**（`statepath\.File\(…`）—— `statepath` 之后是
//	  `\` 而不是 `.` ⇒ 也命不中自己。**证据**：落仓后真仓现读两态（py / 本件）依旧 rc=0（对拍 C13/C14），
//	  即「本件没有把自己扫红」——这一格是**真判**，不是口头声明。
//
// 已对拍面（仓外夹具两态逐字 · 见 `pairwise.md`）：全绿（合成仓 + 真仓指定根 + 真仓默认根）·
// R1/R2/R3/R4/R5/R6 各造 violation 逐条 · 空清单（`schema=0` · 空转 ≠ 绿）· 取不到输入（根定位不到）·
// 未知旗标 · `--json` / `--json=stdout` 拒收 · `--self-test` · `-h` · 位置参数（相对/绝对）。
//
// 未覆盖面（✗ 不声称逐字对齐 · 接线前须逐面补齐或显式剔除）：
//
//	① **argparse 前缀缩写**：py 用 `argparse`，`--self-t` / `--self` 这类**唯一前缀**会被接受；
//	   本件只认精确名 `--self-test` / `--selftest` / `-h` / `--help`（✗ 不做前缀匹配）。
//	② **`-h/--help` 与 usage 正文**：本件按**现读 py 的逐字输出**硬写（含 `prog = check-compat-manifest.py`
//	   与 `optional arguments:` 段名）。⚠ 该正文依赖 **Python 版本**（3.10+ 段名改叫 `options:`）与终端宽度
//	   （本件按默认 80 列无折行硬写）⇒ 换 Python / 换宽度即不同，**不算**逐字对齐承诺。
//	③ **异常路径**：清单存在但**不是 JSON** / `schema` 不是整数 / 某条 entry 不是对象时，py 抛
//	   `json.JSONDecodeError` / `TypeError` / `AttributeError`（**退码同为 1**，但 py 打 traceback 正文，
//	   本件落自定正文明线）⇒ 这一格**退码同、正文不同**，不进逐字面。
//	④ py `--self-test` 用 `subprocess`？—— **否**（py 的 `cmd_selftest()` 进程内直调 `run_rules`）⇒ 这一面
//	   **可**逐字对齐（已对拍 C8）；py 的 `--self-test` 三档别名里本件也只认 `--self-test`/`--selftest`（同①）。
//	⑤ **`os.walk` 剪枝面**：py 剪 `.git/target/vendor/.zerg`（R4/R6 面）；本件同剪。但 py 的 `os.walk`
//	   会把**符号链接目录**当普通目录继续下钻、Go `WalkDir` 默认**不跟随**——本仓 discovery 面无这类链接，
//	   故两态今日同形；若有，本件少走到那些件（只影响发现面的**多报/少报**，不影响既有格子）。
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

// ── 判据常量（与 py 逐条对齐 · 名字前缀 `cm` 防包内同名）──────────────────────────

var (
	cmManifestRel   = filepath.Join("core", "internal", "compat", "compat.json")
	cmMigrationsRel = filepath.Join("core", "internal", "compat", "migrations.go")
)

// cmGoPatterns —— py `GO_PATTERNS`（**转义形态**：`statepath` 之后是 `\` ⇒ ✗ 命不中本件正文 · 见件头 (b)）。
var cmGoPatterns = []*regexp.Regexp{
	regexp.MustCompile(`statepath\.File\(\s*"([^"]+)"\s*\)`),
	regexp.MustCompile(`filepath\.Join\(\s*stateDir\s*,\s*"([^"]+)"\s*\)`),
}

// cmRustPatterns —— py `RUST_PATTERNS`（只扫 `ui/**/*.rs`）。
var cmRustPatterns = []*regexp.Regexp{
	regexp.MustCompile(`ui_dir\(\)\s*\.\s*join\(\s*"([^"]+)"\s*\)`),
	regexp.MustCompile(`format!\(\s*"\{\}/([A-Za-z0-9_.\-]+\.json)"`),
}

// cmJSONFields —— py `JSON_FIELDS = []`（本门**无机器读面** · 唯一真源）。
var cmJSONFields = []string{}

// ── 规则标签（与 py 常量逐字）────────────────────────────────────────────────

const (
	cmR1 = "R1 清单结构 / 版本关系自洽"
	cmR2 = "R2 迁移入口存在（清单→代码）"
	cmR3 = "R3 读写锚点存在（清单→代码）"
	cmR4 = "R4 状态文件已登记（代码→清单）"
	cmR5 = "R5 excluded 必须给理由"
	cmR6 = "R6 状态目录声明可对上代码"
)

// ── 自检夹具串（**转义双引号形态** ⇒ ✗ 不被本件自己的 GO_PATTERNS 命中 · 见件头 (b)）──

const cmProbeGoFlat = "package zzprobe\n\nfunc p() string { return statepath.File(\"brand_new_state.json\") }\n"
const cmProbeGoNested = "package zzprobe\n\nfunc p() string { return statepath.File(\"loopcore/checkpoints\") }\n"

// ── py 输出正文硬写（未覆盖面①②）──────────────────────────────────────────────

const cmProg = "check-compat-manifest.py"

func cmUsageLine() string {
	return "usage: " + cmProg + " [-h] [--self-test] [root]"
}

// cmHelpText —— py `-h/--help` 逐字输出（默认 80 列 · Python 3.9 段名 `optional arguments:`）。
func cmHelpText() string {
	return cmUsageLine() + "\n" +
		"\n" +
		"跨版本状态文件兼容清单门禁（B7 / G10）\n" +
		"\n" +
		"positional arguments:\n" +
		"  root                  仓库根（默认自动定位）\n" +
		"\n" +
		"optional arguments:\n" +
		"  -h, --help            show this help message and exit\n" +
		"  --self-test, --selftest\n" +
		"                        跑门禁自测：反例必须全部被抓到（`--selftest` = 旧拼法，同档）\n"
}

// ── 入口（= py `main()`）────────────────────────────────────────────────────

// compatManifestGateMain —— py `main()` 的 Go 真搬（已接线：`gateCheckGoEntry["check-compat-manifest"]` · `precommit-gates.sh:3418` 门㊺）。
func compatManifestGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① `Q-176` 同族：**先于** argparse —— 给了 `--json` 而本门无机器读面 ⇒ rc=2（与 py 同序）。
	if rc := cmRefuseJSON(argv, stderr); rc != nil {
		return *rc
	}

	// ② argparse 等价：位置参数 `root`（可省）· `--self-test`/`--selftest` · `-h/--help`。
	rootArg, selfTest, help, bad := cmParseArgs(argv)
	if bad != "" {
		fmt.Fprint(stderr, cmUsageLine()+"\n")
		fmt.Fprintf(stderr, "%s: error: %s\n", cmProg, bad)
		return 2
	}
	if help {
		fmt.Fprint(stdout, cmHelpText())
		return 0
	}

	// ③ 根定位（py 末档 = `__file__` 目录 ⇒ 本件末档 = cwd · 见件头 (a)）。
	start := rootArg
	if start == "" {
		if wd, err := os.Getwd(); err == nil {
			start = wd
		} else {
			start = "."
		}
	}
	root, ok := cmFindRoot(start)
	if !ok {
		fmt.Fprintf(stderr, "找不到仓库根（缺 %s）\n", cmManifestRel)
		return 1
	}

	// ④ `--self-test`（py：根定位**之后**才分流 ⇒ 根不到时 `--self-test` 也是 rc=1）。
	if selfTest {
		return cmSelfTest(stdout, root)
	}
	return cmCheck(stdout, stderr, root)
}

// cmParseArgs —— argparse 等价解析（精确长选项 · 位置参数 `root` 至多一个）。
// 返 (root, selfTest, help, bad)；bad 非空 ⇒ usage + error 正文 + rc=2。
func cmParseArgs(argv []string) (string, bool, bool, string) {
	root := ""
	haveRoot := false
	selfTest := false
	for _, x := range argv {
		switch {
		case x == "--self-test" || x == "--selftest":
			selfTest = true
		case x == "-h" || x == "--help":
			return "", false, true, ""
		case strings.HasPrefix(x, "-") && x != "-":
			return "", false, false, "unrecognized arguments: " + x
		default:
			if haveRoot {
				return "", false, false, "unrecognized arguments: " + x
			}
			root = x
			haveRoot = true
		}
	}
	return root, selfTest, false, ""
}

// cmRefuseJSON —— py `refuse_json_without_machine_face()`：命中 `--json` / `--json=…` ⇒ 打印并返 2；否则 nil。
func cmRefuseJSON(argv []string, stderr io.Writer) *int {
	var hit []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, "`"+x+"`")
		}
	}
	if len(hit) == 0 {
		return nil
	}
	rc := 2
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(hit, " · "))
	fields := "无（本门只出人读面）"
	if len(cmJSONFields) > 0 {
		parts := make([]string, len(cmJSONFields))
		for i, f := range cmJSONFields {
			parts[i] = "`" + f + "`"
		}
		fields = strings.Join(parts, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fields)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return &rc
}

// ── 根定位 / 清单读取 ───────────────────────────────────────────────────────

// cmFindRoot —— py `find_root(start)`：上溯 ≤8 层找含 `compat.json` 的目录。
func cmFindRoot(start string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		d = start
	}
	for i := 0; i < 8; i++ {
		if cmIsFile(filepath.Join(d, cmManifestRel)) {
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

func cmLoadManifest(root string) (map[string]interface{}, []byte, bool) {
	raw, err := os.ReadFile(filepath.Join(root, cmManifestRel))
	if err != nil {
		return nil, nil, false
	}
	var man map[string]interface{}
	if err := json.Unmarshal(raw, &man); err != nil {
		return nil, raw, false
	}
	return man, raw, true
}

func cmReadText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

// cmSourceFiles —— py `source_files(root, exts, subdirs)`：逐子目录下钻（剪 `.git/target/vendor/.zerg`），
// 收指定后缀的件，最后 **sorted** 定序（⇒ 与 Go `WalkDir` 的字典序无关 · 见件头「输出确定有序」）。
func cmSourceFiles(root string, exts []string, subdirs []string) []string {
	var out []string
	skip := map[string]bool{".git": true, "target": true, "vendor": true, ".zerg": true}
	for _, sub := range subdirs {
		base := filepath.Join(root, sub)
		fi, err := os.Stat(base)
		if err != nil || !fi.IsDir() {
			continue
		}
		_ = filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if skip[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			for _, e := range exts {
				if strings.HasSuffix(d.Name(), e) {
					out = append(out, p)
					break
				}
			}
			return nil
		})
	}
	sort.Strings(out)
	return out
}

// ── 规则（与 py 逐条对齐）──────────────────────────────────────────────────

func cmR1Structure(man map[string]interface{}) []string {
	var v []string
	sch, ok := cmInt(man["schema"])
	if !ok {
		// py 侧此处 `int(...)` 抛异常 ⇒ `run_rules` 兜成「规则执行异常」（退码同 1 · 正文自定 · 未覆盖面③）。
		return []string{fmt.Sprintf("规则执行异常：TypeError（schema 不是整数：%s）", cmRepr(man["schema"]))}
	}
	if sch < 1 {
		v = append(v, fmt.Sprintf("清单自身的 schema 必须 >= 1（实际 %s）", cmRepr(man["schema"])))
	}
	names := map[string]bool{}
	files := map[string]bool{}
	for i, ei := range cmEntries(man) {
		e, _ := ei.(map[string]interface{})
		if e == nil {
			e = map[string]interface{}{}
		}
		w := fmt.Sprintf("entries[%d](%s)", i, cmPyStr(e["name"]))
		name, _ := e["name"].(string)
		f, _ := e["file"].(string)
		if name == "" || f == "" {
			v = append(v, w+"：name/file 不得为空")
			continue
		}
		if names[name] {
			v = append(v, w+"：name 重复")
		}
		names[name] = true
		dir := ""
		if s, ok := e["dir"].(string); ok {
			dir = s
		}
		key := dir + "/" + f
		if files[key] {
			v = append(v, w+"：file 重复（"+key+"）")
		}
		files[key] = true
		if s := cmPyStr(e["owner"]); s != "go" && s != "ui" {
			v = append(v, fmt.Sprintf("%s：owner 必须是 go|ui（实际 %s）", w, cmRepr(e["owner"])))
		}
		if s := cmPyStr(e["kind"]); s != "file" && s != "glob" {
			v = append(v, fmt.Sprintf("%s：kind 必须是 file|glob（实际 %s）", w, cmRepr(e["kind"])))
		}
		if s := cmPyStr(e["envelope"]); s != "inband" && s != "sidecar" {
			v = append(v, fmt.Sprintf("%s：envelope 必须是 inband|sidecar（实际 %s）", w, cmRepr(e["envelope"])))
		}
		if s := cmPyStr(e["stamps_schema"]); s != "layer" && s != "native" && s != "sidecar" && s != "none" {
			v = append(v, fmt.Sprintf("%s：stamps_schema 非法（%s）", w, cmRepr(e["stamps_schema"])))
		}
		cur, curOK := cmInt(e["current_schema"])
		mn, mnOK := cmInt(e["min_readable"])
		if !curOK || !mnOK {
			v = append(v, w+"：current_schema/min_readable 必须是整数")
			continue
		}
		if cur < 1 {
			v = append(v, fmt.Sprintf("%s：current_schema=%d 必须 >= 1（schema 化的意义就是有版本号）", w, cur))
		}
		if mn < 0 {
			v = append(v, fmt.Sprintf("%s：min_readable=%d 不得为负", w, mn))
		}
		if cur < mn {
			v = append(v, fmt.Sprintf("%s：current_schema=%d < min_readable=%d —— 声明自相矛盾", w, cur, mn))
		}
		if cmPyStr(e["envelope"]) == "inband" && strings.TrimSpace(cmPyStr(e["schema_field"])) == "" {
			v = append(v, w+"：inband 信封必须给 schema_field")
		}
		if strings.TrimSpace(cmPyStr(e["anchor_file"])) == "" || strings.TrimSpace(cmPyStr(e["anchor"])) == "" {
			v = append(v, w+"：anchor_file/anchor 不得为空")
		}
	}
	return v
}

func cmR2MigrationEntry(man map[string]interface{}, root string) []string {
	var v []string
	src := cmReadText(filepath.Join(root, cmMigrationsRel))
	if src == "" {
		return []string{"迁移实现文件缺失：" + cmMigrationsRel}
	}
	for _, ei := range cmEntries(man) {
		e, _ := ei.(map[string]interface{})
		if e == nil {
			e = map[string]interface{}{}
		}
		name := cmPyStr(e["name"])
		fn := strings.TrimSpace(cmPyStr(e["migrate_func"]))
		if fn == "" {
			v = append(v, fmt.Sprintf("%s：migrate_func 不得为空", name))
			continue
		}
		if !regexp.MustCompile(`func\s+migrate` + regexp.QuoteMeta(fn) + `\s*\(`).MatchString(src) {
			v = append(v, fmt.Sprintf("%s：migrate_func=%s 在 %s 里找不到 func migrate%s（迁移入口不存在）",
				name, fn, cmMigrationsRel, fn))
		}
	}
	return v
}

func cmR3ReadWriteAnchor(man map[string]interface{}, root string) []string {
	var v []string
	for _, ei := range cmEntries(man) {
		e, _ := ei.(map[string]interface{})
		if e == nil {
			e = map[string]interface{}{}
		}
		name := cmPyStr(e["name"])
		af := strings.TrimSpace(cmPyStr(e["anchor_file"]))
		anchor := cmPyStr(e["anchor"])
		if af == "" || strings.TrimSpace(anchor) == "" {
			continue // R1 已报
		}
		src := cmReadText(filepath.Join(root, af))
		if src == "" {
			v = append(v, fmt.Sprintf("%s：anchor_file 不存在（%s）", name, af))
			continue
		}
		if !strings.Contains(src, anchor) {
			v = append(v, fmt.Sprintf("%s：anchor %s 不在 %s 里（该文件可能没有真正的读写实现）",
				name, cmRepr(anchor), af))
		}
	}
	return v
}

// cmDeclaredNames —— py `declared_names(man)`：`file` 原文 + `basename(file)`（entries 与 excluded 两处都收）。
func cmDeclaredNames(man map[string]interface{}) map[string]bool {
	out := map[string]bool{}
	add := func(f string) {
		out[f] = true
		out[filepath.Base(f)] = true
	}
	for _, ei := range cmEntries(man) {
		if e, ok := ei.(map[string]interface{}); ok {
			add(cmPyStr(e["file"]))
		}
	}
	if xs, ok := man["excluded"].([]interface{}); ok {
		for _, xi := range xs {
			if x, ok := xi.(map[string]interface{}); ok {
				add(cmPyStr(x["file"]))
			}
		}
	}
	return out
}

// cmDiscoveredStateFiles —— py `discovered_state_files(root)`：按 **sorted 件序** 首次命中为准（setdefault）。
func cmDiscoveredStateFiles(root string) map[string]string {
	found := map[string]string{}
	for _, path := range cmSourceFiles(root, []string{".go"}, []string{"core", "agent"}) {
		src := cmReadText(path)
		for _, pat := range cmGoPatterns {
			for _, m := range pat.FindAllStringSubmatch(src, -1) {
				if _, seen := found[m[1]]; !seen {
					found[m[1]] = path
				}
			}
		}
	}
	for _, path := range cmSourceFiles(root, []string{".rs"}, []string{"ui"}) {
		src := cmReadText(path)
		for _, pat := range cmRustPatterns {
			for _, m := range pat.FindAllStringSubmatch(src, -1) {
				if _, seen := found[m[1]]; !seen {
					found[m[1]] = path
				}
			}
		}
	}
	return found
}

func cmR4CodeToManifest(man map[string]interface{}, root string) []string {
	var v []string
	declared := cmDeclaredNames(man)
	found := cmDiscoveredStateFiles(root)
	names := make([]string, 0, len(found))
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		if declared[name] {
			continue
		}
		rel, err := filepath.Rel(root, found[name])
		if err != nil {
			rel = found[name]
		}
		v = append(v, fmt.Sprintf("状态文件 %s（被 %s 引用）未登记 —— 请写进 compat.json 的 entries（带 schema 版本）或 excluded（带理由）",
			cmRepr(name), rel))
	}
	return v
}

func cmR5ExcludedReasons(man map[string]interface{}) []string {
	var v []string
	entryFiles := map[string]bool{}
	for _, ei := range cmEntries(man) {
		if e, ok := ei.(map[string]interface{}); ok {
			entryFiles[filepath.Base(cmPyStr(e["file"]))] = true
		}
	}
	xs, _ := man["excluded"].([]interface{})
	for i, xi := range xs {
		x, _ := xi.(map[string]interface{})
		if x == nil {
			x = map[string]interface{}{}
		}
		f := strings.TrimSpace(cmPyStr(x["file"]))
		if f == "" {
			v = append(v, fmt.Sprintf("excluded[%d]：file 不得为空", i))
			continue
		}
		if strings.TrimSpace(cmPyStr(x["reason"])) == "" {
			v = append(v, fmt.Sprintf("excluded[%d](%s)：必须给出不登记的理由（不许无理由排除）", i, f))
		}
		if entryFiles[filepath.Base(f)] {
			v = append(v, fmt.Sprintf("excluded[%d](%s)：同时出现在 entries —— 二者互斥", i, f))
		}
	}
	return v
}

func cmR6DirsDeclaration(man map[string]interface{}, root string, order []string) []string {
	var v []string
	dirs, _ := man["dirs"].(map[string]interface{})
	if len(order) == 0 {
		order = cmManifestDirsOrder(man) // 兜底（文档序取不到时按键名定序 · 不随机）
	}
	if len(dirs) == 0 {
		v = append(v, "清单缺 dirs 段（状态目录的解析方式必须声明）")
	}
	var blobs []string
	for _, path := range cmSourceFiles(root, []string{".go"}, []string{"core"}) {
		blobs = append(blobs, cmReadText(path))
	}
	joined := strings.Join(blobs, "\n")
	for _, key := range order {
		di, ok := dirs[key]
		if !ok {
			continue
		}
		d, _ := di.(map[string]interface{})
		if d == nil {
			d = map[string]interface{}{}
		}
		env := strings.TrimSpace(cmPyStr(d["env"]))
		resolver := strings.TrimSpace(cmPyStr(d["resolver"]))
		if env != "" && !strings.Contains(joined, env) {
			v = append(v, fmt.Sprintf("dirs.%s：环境变量 %s 未在 Go 源码里出现（声明与实现脱节）", key, env))
		}
		parts := strings.Split(resolver, ".")
		tail := parts[len(parts)-1]
		if tail != "" && !regexp.MustCompile(`func\s+`+regexp.QuoteMeta(tail)+`\s*\(`).MatchString(joined) {
			v = append(v, fmt.Sprintf("dirs.%s：解析器 %s 未在 Go 源码里定义", key, resolver))
		}
	}
	return v
}

// cmRunRules —— py `run_rules(man, root)`：只收有问题的规则（标签 → 问题行）。
func cmRunRules(man map[string]interface{}, root string, dirOrder []string) map[string][]string {
	out := map[string][]string{}
	type rule struct {
		label string
		fn    func() []string
	}
	rules := []rule{
		{cmR1, func() []string { return cmR1Structure(man) }},
		{cmR2, func() []string { return cmR2MigrationEntry(man, root) }},
		{cmR3, func() []string { return cmR3ReadWriteAnchor(man, root) }},
		{cmR4, func() []string { return cmR4CodeToManifest(man, root) }},
		{cmR5, func() []string { return cmR5ExcludedReasons(man) }},
		{cmR6, func() []string { return cmR6DirsDeclaration(man, root, dirOrder) }},
	}
	for _, r := range rules {
		probs := cmSafeRule(r.fn)
		if len(probs) > 0 {
			out[r.label] = probs
		}
	}
	return out
}

// cmSafeRule —— py `try/except` 兜底：规则自身崩了也算红（不漏报 · 不静默通过）。
func cmSafeRule(fn func() []string) (probs []string) {
	defer func() {
		if r := recover(); r != nil {
			probs = []string{fmt.Sprintf("规则执行异常：%v", r)}
		}
	}()
	return fn()
}

// cmCheck —— py `cmd_check(root)`。
func cmCheck(stdout, stderr io.Writer, root string) int {
	man, raw, ok := cmLoadManifest(root)
	if !ok {
		// py 侧此处抛 `json.JSONDecodeError` / `FileNotFoundError`（traceback · 退码 1）—— 未覆盖面③。
		fmt.Fprintf(stderr, "⛔ 兼容清单读不到或解析失败（%s）\n", filepath.Join(root, cmManifestRel))
		return 1
	}
	dirOrder := cmManifestDirsOrderFromRaw(raw)
	fmt.Fprintf(stdout, "兼容清单: %s\n", filepath.Join(root, cmManifestRel))
	fmt.Fprintf(stdout, "条目 %d · 明确不登记 %d · 清单 schema %s\n",
		len(cmEntries(man)), len(cmExcluded(man)), cmPyStr(man["schema"]))
	fmt.Fprintln(stdout, "")

	bad := cmRunRules(man, root, dirOrder)
	if len(bad) > 0 {
		labels := make([]string, 0, len(bad))
		for l := range bad {
			labels = append(labels, l)
		}
		sort.Strings(labels)
		for _, label := range labels {
			fmt.Fprintf(stdout, "❌ %s\n", label)
			for _, p := range bad[label] {
				fmt.Fprintf(stdout, "     · %s\n", p)
			}
		}
		fmt.Fprintln(stdout, "")
		fmt.Fprintf(stdout, "门禁未通过：清单与代码不一致（%d 类问题）\n", len(bad))
		return 1
	}
	fmt.Fprintln(stdout, "✅ 六条规则全过：清单与代码双向一致，且 current_schema >= min_readable >= 0")
	for _, ei := range cmEntries(man) {
		e, _ := ei.(map[string]interface{})
		if e == nil {
			e = map[string]interface{}{}
		}
		cur, _ := cmInt(e["current_schema"])
		mn, _ := cmInt(e["min_readable"])
		fmt.Fprintf(stdout, "   %-20s %-28s schema=%d min=%d %s/%s\n",
			cmPyStr(e["name"]), cmPyStr(e["file"]), cur, mn,
			cmPyStr(e["envelope"]), cmPyStr(e["stamps_schema"]))
	}
	return 0
}

// ── 自测（反例必须全被抓到 · 与 py `cmd_selftest` 逐字对齐）─────────────────────

// cmSelfTest —— py `cmd_selftest(root)`：1 正控 + 8 类反例（其中 R4 两条为成对断言）。
func cmSelfTest(stdout io.Writer, root string) int {
	man, raw, loaded := cmLoadManifest(root)
	if !loaded {
		fmt.Fprintf(stdout, "门禁自测失败：清单读不到（%s）\n", filepath.Join(root, cmManifestRel))
		return 1
	}
	dirOrder := cmManifestDirsOrderFromRaw(raw)

	type tc struct {
		label string
		pred  func() bool
	}
	var cases []tc

	// ① 干净清单必须全过
	cases = append(cases, tc{"干净清单全过（无假阳性）", func() bool {
		return len(cmRunRules(man, root, dirOrder)) == 0
	}})

	// ② R1：current < min_readable（声明自相矛盾）
	cases = append(cases, tc{"R1 current<min_readable", func() bool {
		b := cmDeepCopy(man)
		e := cmEntryAt(b, 0)
		cur, _ := cmInt(e["current_schema"])
		e["min_readable"] = float64(cur + 1)
		return cmHasRule(cmRunRules(b, root, dirOrder), cmR1)
	}})

	// ③ R1：current_schema = 0（等于没有版本号）
	cases = append(cases, tc{"R1 current_schema=0", func() bool {
		b := cmDeepCopy(man)
		cmEntryAt(b, 0)["current_schema"] = float64(0)
		return cmHasRule(cmRunRules(b, root, dirOrder), cmR1)
	}})

	// ④ R2：migrate_func 指向不存在的函数
	cases = append(cases, tc{"R2 迁移入口缺失", func() bool {
		b := cmDeepCopy(man)
		cmEntryAt(b, 0)["migrate_func"] = "NoSuchMigrationXyz"
		return cmHasRule(cmRunRules(b, root, dirOrder), cmR2)
	}})

	// ⑤ R3：anchor 指向不存在的符号
	cases = append(cases, tc{"R3 锚点缺失", func() bool {
		b := cmDeepCopy(man)
		cmEntryAt(b, 0)["anchor"] = "func no_such_symbol_xyz()"
		return cmHasRule(cmRunRules(b, root, dirOrder), cmR3)
	}})

	// ⑥ R4：源码里植入一个未登记的状态文件（临时树，绝不碰真仓）· 配对断言「空树不许报 R4」
	b5 := cmDeepCopy(man)
	b5["entries"] = []interface{}{}
	b5["excluded"] = []interface{}{}
	cases = append(cases, tc{"R4 未登记状态文件（含空树不误报的反证）", func() bool {
		empty, err1 := os.MkdirTemp("", "zerg-cm-empty-")
		if err1 != nil {
			return false
		}
		defer os.RemoveAll(empty)
		if cmHasRule(cmRunRules(b5, empty, nil), cmR4) {
			return false // 空树都报 R4 ⇒ 检测器是「永远报红」，不算数
		}
		tmp, err2 := os.MkdirTemp("", "zerg-cm-planted-")
		if err2 != nil {
			return false
		}
		defer os.RemoveAll(tmp)
		planted := filepath.Join(tmp, "core", "internal", "zzprobe")
		if err := os.MkdirAll(planted, 0o755); err != nil {
			return false
		}
		if err := os.WriteFile(filepath.Join(planted, "probe.go"), []byte(cmProbeGoFlat), 0o644); err != nil {
			return false
		}
		return cmHasRule(cmRunRules(b5, tmp, nil), cmR4)
	}})

	// ⑥b R4 嵌套件：basename 兜不住 ⇒ 必须报；登记全路径 ⇒ 不报（修完后的正控）
	cases = append(cases, tc{"R4 嵌套落点必须登记全路径（basename 兜不住 · 全路径不报）", func() bool {
		plantedRoot, err := os.MkdirTemp("", "zerg-cm-nested-")
		if err != nil {
			return false
		}
		defer os.RemoveAll(plantedRoot)
		planted := filepath.Join(plantedRoot, "core", "internal", "zzprobe")
		if err := os.MkdirAll(planted, 0o755); err != nil {
			return false
		}
		if err := os.WriteFile(filepath.Join(planted, "probe.go"), []byte(cmProbeGoNested), 0o644); err != nil {
			return false
		}
		bare := cmDeepCopy(b5)
		bare["excluded"] = []interface{}{map[string]interface{}{"file": "checkpoints", "reason": "夹具：只写 basename"}}
		nested := cmDeepCopy(b5)
		nested["excluded"] = []interface{}{map[string]interface{}{"file": "loopcore/checkpoints", "reason": "夹具：写全路径"}}
		return cmHasRule(cmRunRules(bare, plantedRoot, nil), cmR4) && !cmHasRule(cmRunRules(nested, plantedRoot, nil), cmR4)
	}})

	// ⑦ R5：excluded 没有理由
	cases = append(cases, tc{"R5 excluded 缺理由", func() bool {
		b6 := cmDeepCopy(man)
		xs, _ := b6["excluded"].([]interface{})
		if len(xs) > 0 {
			if x, ok := xs[0].(map[string]interface{}); ok {
				x["reason"] = "   "
			}
		} else {
			b6["excluded"] = []interface{}{map[string]interface{}{"file": "x.jsonl", "reason": ""}}
		}
		return cmHasRule(cmRunRules(b6, root, dirOrder), cmR5)
	}})

	// ⑧ R6：环境变量名与代码脱节
	//    ⚠ 自指反制（本单第二发血证）：本件**在 R6 的扫描面内**（`core/**/*.go`）⇒ 这一格用的魔法串
	//    **不许**以**连续字面**落在本件里 —— 否则 `env not in joined` 会被本件自己满足，真仓
	//    `--self-test` 的⑧号反例当场抓不住（py 侧同因：它扫的是同一棵树）。故此处拼串落地。
	cases = append(cases, tc{"R6 目录声明脱节", func() bool {
		b7 := cmDeepCopy(man)
		if dirs, ok := b7["dirs"].(map[string]interface{}); ok {
			for k := range dirs {
				if d, ok := dirs[k].(map[string]interface{}); ok {
					d["env"] = "ZERG_NO_SUCH_ENV_" + "XYZ"
				}
			}
		}
		return cmHasRule(cmRunRules(b7, root, dirOrder), cmR6)
	}})

	failed := 0
	for _, c := range cases {
		ok := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(stdout, "     （反例 %s 执行异常：%v）\n", c.label, r)
					ok = false
				}
			}()
			ok = c.pred()
		}()
		if !ok {
			failed++
		}
		mark := "✅"
		if !ok {
			mark = "❌"
		}
		fmt.Fprintf(stdout, "%s 反例 %s\n", mark, c.label)
	}
	fmt.Fprintln(stdout, "")
	if failed > 0 {
		fmt.Fprintf(stdout, "门禁自测失败：%d 个反例没被抓住 —— 这个门禁不可信\n", failed)
		return 1
	}
	fmt.Fprintf(stdout, "门禁自测通过：干净清单全过（无假阳性）+ %d 类反例全部被对应规则抓到\n", len(cases)-1)
	return 0
}

// ── 小工具（与 py 语义逐条对齐）──────────────────────────────────────────────

func cmEntries(man map[string]interface{}) []interface{} {
	es, _ := man["entries"].([]interface{})
	return es
}

func cmExcluded(man map[string]interface{}) []interface{} {
	xs, _ := man["excluded"].([]interface{})
	return xs
}

func cmHasRule(bad map[string][]string, label string) bool {
	_, ok := bad[label]
	return ok
}

// cmManifestDirsOrder —— 保住清单 `dirs` 的**文档序**（py 是 `dict.items()` 保插入序；
// Go map 随机序 ⇒ 另读一遍 token 流取键序 · 见件头「输出确定有序」）。
func cmManifestDirsOrder(man map[string]interface{}) []string {
	dirs, ok := man["dirs"].(map[string]interface{})
	if !ok {
		return nil
	}
	// 兜底：文档序取不到时按键名字典序（确定 · 不随机）。
	out := make([]string, 0, len(dirs))
	for k := range dirs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// cmManifestDirsOrderFromRaw —— 真源：从原始 JSON 字节里按文档序抽 `dirs` 的键。
func cmManifestDirsOrderFromRaw(raw []byte) []string {
	top := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil
	}
	d, ok := top["dirs"]
	if !ok {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(d))
	t, err := dec.Token()
	if err != nil {
		return nil
	}
	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return nil
	}
	var keys []string
	lvl := make([]int, 0, 4) // 1 = 等键, 2 = 等值, -1 = 数组（不取键）
	lvl = append(lvl, 1)
	for {
		t, err := dec.Token()
		if err != nil {
			break
		}
		switch v := t.(type) {
		case json.Delim:
			switch v {
			case '{':
				lvl = append(lvl, 1)
			case '[':
				lvl = append(lvl, -1)
			case '}', ']':
				if len(lvl) > 0 {
					lvl = lvl[:len(lvl)-1]
				}
				if len(lvl) > 0 && lvl[len(lvl)-1] != -1 {
					lvl[len(lvl)-1] = 1 // 父层的一个值读完了 ⇒ 回到等键
				}
			}
		default:
			if len(lvl) == 0 {
				continue
			}
			cur := lvl[len(lvl)-1]
			if cur == 1 {
				if s, ok := v.(string); ok {
					keys = append(keys, s)
				}
				lvl[len(lvl)-1] = 2
			} else if cur == 2 {
				lvl[len(lvl)-1] = 1
			}
		}
		if len(lvl) == 0 {
			break
		}
	}
	return keys
}

// cmInt —— py `int(v)` 的可用子集（整数 / 整值浮点 / 十进制串）；不可用 ⇒ (0, false)。
func cmInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case float64:
		if x == math.Trunc(x) && !math.IsInf(x, 0) && !math.IsNaN(x) {
			return int(x), true
		}
		return 0, false
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, false
		}
		i, err := strconv.Atoi(s)
		if err != nil {
			return 0, false
		}
		return i, true
	}
	return 0, false
}

// cmPyStr —— py `str(v)`（None ⇒ "None" · True/False ⇒ 同形）。
func cmPyStr(v interface{}) string {
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
		if x == math.Trunc(x) && !math.IsInf(x, 0) && !math.IsNaN(x) && math.Abs(x) < 1e16 {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

// cmRepr —— py `repr(v)`（本件面内取值恒为简单串 / None ⇒ 单引号 repr 逐字）。
func cmRepr(v interface{}) string {
	if s, ok := v.(string); ok {
		return "'" + s + "'"
	}
	return cmPyStr(v)
}

func cmIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func cmDeepCopy(d map[string]interface{}) map[string]interface{} {
	b, err := json.Marshal(d)
	if err != nil {
		return map[string]interface{}{}
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]interface{}{}
	}
	return out
}

// cmEntryAt —— 取 entries[i]（map）；越界/类型不符 ⇒ 造一个空 map 挂回去（只动内存副本）。
func cmEntryAt(d map[string]interface{}, i int) map[string]interface{} {
	es, _ := d["entries"].([]interface{})
	for len(es) <= i {
		es = append(es, map[string]interface{}{})
	}
	m, ok := es[i].(map[string]interface{})
	if !ok {
		m = map[string]interface{}{}
		es[i] = m
	}
	d["entries"] = es
	return m
}
