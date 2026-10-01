// check_ci_package_parity.go —— 门禁件 `scripts/gates/check-ci-package-parity.py` 的 **Go 真搬**（v2.5.14 · M2-0 第四枚）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据 · 设计 v1.0 §4.1）的**第四枚**：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（§4.2）。
//
// 选它的理由（对照 M2-0 四条硬口径）：
//
//	✓ **无外部树依赖**：只读仓内两件 —— `scripts/gates/ci-public-test-surface.tsv`（白/黑名单）、
//	  `publish/ci/ci.yml`（发布源件）。**不碰**文档仓 `Zerg-内部文档/` / 公开树 / 生成物 / 制品 `bin/` / 真机状态目录。
//	  （py 件头只把 `Zerg-内部文档/…任务清单` 当**设计真源引述**，代码一个字节都不读它 —— 已现读核过。）
//	✓ **输出确定有序**：`in-ci` / `名单外` / `点名表` 三处全走 `sorted({…})`，工作目录计数走 `sorted(ci.items())`
//	  ⇒ 无 dict 遍历序 · 无 `os.walk`/`glob` ⇒ 逐字节对拍可证。
//	✓ **退码 0/1/2 三档齐**：0 两侧逐包一致 / 1 有差（逐包点名）/ 2 不给结论（缺件 · 空转 · 名单件坏行 · 自检不过）。
//	✓ **纯静态无写面**：main 路径一个字节都不写（`--self-test` 只在 `tempfile.mkdtemp` / `os.MkdirTemp` 里造夹具）。
//	✓ **无人正在改**：`git status --porcelain` 现读**不含** `scripts/gates/check-ci-package-parity.py`
//	  （也不含它读的两件：tsv / ci.yml）。
//	✓ 体量适中（413 行）· **无 subprocess / 无 git / 无 datetime.now / 无 os.walk / 无 glob**。
//	  （唯一 `shutil` 用途是自检里的 `rmtree`，非判据面。）
//
// 不选的理由（逐件·本单现读后自定 · 候选区间 = `check-*.py` 首字母 a–e）：
//
//	✗ `check-affected-map.py`（523）/ `check-affecte-*`（共享件）—— `check-affected-reprove.py` 本批**共享件禁改**。
//	✗ `check-annotate-*`（无此件）· `check-anchor-uniqueness.py`（340）· `check-dryrun-semantics.py`（349）
//	  —— **读仓外 `Zerg-内部文档/`**（外部树依赖）⇒ 不属 M2-0「读仓内件即可」。
//	✗ `check-curl-timeout.py`（167）—— 扫描根靠 `os.walk` ⇒ **遍历序非确定**（py `scandir` inode 序 ≠ Go `WalkDir` 字典序）⇒ 对拍假差。
//	✗ `check-compat-manifest.py`（449）—— 判据面靠 `os.walk(core/agent/ui)` 逐源扫描 + Rust 面解析；
//	  虽输出前 `sorted` 过，但**跨语言（Go/Rust）源码扫描面**与本批「最少可证逐字」取向不合 ⇒ 让位给更小的件。
//	✗ `check-ai-direct-edits.py`（258）· `check-approve-backfill.py`（683）· `check-approve-*.py`（CI/closedset/freeze/merge/tier/time）
//	  —— 判据面含**审批状态机 / 真机状态目录 / sha 账**，属「有输入物料」档，不落 M2-0。
//	✗ `check-build-tags.py`（863）—— 现跑 `go build`/子进程（制品与外部进程依赖）。
//	✗ `check-artifact-source-parity.py`（476）—— 现跑制品、含 `datetime`/`glob`/子进程。
//	✗ `check-cli-contract.py`（1283）—— 体量过大 + 子进程面。
//	✗ `check-contract-registry.py`（291）—— **已由路1 真搬**（`core/cmd/zerg/check_contract_registry.go`）。
//	✗ `check-error-kinds.py`（214）/ `check-gotoolchain.py` / `check-mutation-residue.py` / `check-wall_layer.go`
//	  —— **已搬**（各带 `_test.go`）。
//	✗ `check-exit-codes.py`（649）—— 判据面靠现跑 `bin/zerg`（制品依赖）⇒ 非纯静态 · 且本批禁碰 `bin/`。
//	✗ 8 件 `doc-*` / `check-dangling-refs.py` / `family_*` / 已列共享件 —— **本批禁改 / 另路在改**（`git status` 现读）。
//
// ★★ 标准两条声明行（M2 样板体例 · 必写）：
//
//	★(a) **默认根定位差异**：py 的 `--root` 默认 = `REPO_ROOT = os.path.dirname(os.path.dirname(HERE))`
//	   （HERE 由 `__file__` 得，即 `scripts/gates/` 上溯两级 = 仓根）；本件**无 `__file__` 语境**
//	   （未来接线后是 CLI 子命令）⇒ 默认 = **当前工作目录**（`os.Getwd()` → `filepath.Abs`）。
//	   落在**同一真仓、cwd=仓根**时两者读数逐字相同（已对拍 C19）；「脚本位置 ≠ cwd」时**路径推导面不同**。
//	★(b) **自指检查**：「本件是否落在它自己扫的面内」——**否（源码面）· 是（数据面一行）**，逐条照实：
//	   · judge 只读**两份登记/配置件**的**文本**（tsv 的 `(module,pkg,tier)` 行 + `ci.yml` 的逐包点名块），
//	     **不扫任何 Go 源** ⇒ 本件源码（与其测试件的夹具字面）**不进**判据面，不存在「把自己判红」的自指。
//	   · 但本件的**包名** `core/cmd/zerg` **是 tsv 里已登记的一行**（现读：`core\tcmd/zerg\tout-ci`），
//	     且 `ci.yml` **不**点名 `./cmd/zerg/` ⇒ 本件落进仓后**两件读数一字不变**（两态 C1 真仓现读 rc=0 已证）。
//	     ⇒ 与 `check_wall_layer.go` 那次「把门⑥ 茧壁标记字面直写进正文 ⇒ 被自己扫成红」**不同**：
//	     本件正文里出现的 `scripts/gates/ci-public-test-surface.tsv` / `publish/ci/ci.yml` 字面**不被本件判据读**，
//	     故无需回避写法 —— 仍逐条声明，供接线前复核。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func ciPackageParityGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 全绿 / 1 有差 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字）：全绿（真仓 & 合成仓）· 判据 A（只在名单件 / 只在 ci.yml）· 判据 B（名单外包进 CI）
// · 模块归属错 · 空转（tsv `in-ci` 0 行 / ci.yml 点名 0 包）· 缺件（tsv 缺 / ci.yml 缺）· 名单件坏行（tier 闭集外）
// · `--json` 形态（合法子集 / 表外字段名 / 空字段）· `--no-self-test` · `--self-test` · 真仓现读（不给根 · cwd=仓根）。
//
// 未覆盖面（✗ 不声称逐字对齐 · 接线前须逐面补齐或显式剔除）：
//
//	① **argparse 帮助 / usage 正文**：py 用 `argparse`（`-h/--help` · 未知旗标 · `--json`/`--root` 缺值 ·
//	   未知位置参数 · 长选项前缀缩写 `--roo`）⇒ 这些格子 py 打的是 argparse 生成的 usage/error 正文（依赖 `prog` 名
//	   与终端宽度 `COLUMNS`）。★ 本件落「`-h` ⇒ rc=2」而 py 的 `-h` **rc=0 且 usage 打 stdout**（argparse 内建行为）
//	   ⇒ 这一面**不属**逐字对齐承诺（已对拍并当场点名）。
//	② **默认根定位差异**：见 ★(a)。「脚本位置 ≠ cwd」时路径推导面不同。
//	③ **异常路径（读件失败的正文）**：本件对 **ENOENT / IsADirectory** 两种常见形态**逐字复刻 Python 的 OSError 正文**
//	   （`[Errno 2] No such file or directory: '<path>'` / `[Errno 21] Is a directory: '<path>'`）⇒ 这两格**可**对齐；
//	   其余 errno（权限 `EACCES` 等）落 **Go 的 error 正文** ⇒ **同样退 2，但正文与 py 不同**。
//	④ **件非 UTF-8**：py `open(..., encoding="utf-8")` 抛 `UnicodeDecodeError`（**未捕获** ⇒ traceback 打到 stderr、退码非 0/1/2）；
//	   本件按字节读 ⇒ 正文/退码都不同（属「坏件」极端面，不在样板对拍集内）。
//	⑤ **self-test 手段**：py 用 `tempfile.mkdtemp` + `shutil.rmtree`、进程内直调 `judge()`（**不起子进程**）；
//	   本件用 `os.MkdirTemp` + `RemoveAll`、进程内直调 `cippJudge()` ⇒ 临时路径**不入输出** ⇒ 这一面**可**逐字对齐（已对拍 C-S/T）。
//
//	—— ①②③④ 四面均**不在** M2-0 样板的「承诺逐字」集内（①③④ 已逐格点名 · ②是刻意差异）；⑤ 已覆盖。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ── 判据常量（与 py 逐条对齐 · 名字前缀 `cipp` 防包内同名）────────────────────────

const cippVersion = "check-ci-package-parity.py v1.0.0"

var (
	cippTSVRel = filepath.Join("scripts", "gates", "ci-public-test-surface.tsv")
	cippCIRel  = filepath.Join("publish", "ci", "ci.yml")
)

var cippTiers = []string{"in-ci", "out-ci", "no-test"}
var cippModules = []string{"core", "agent"}

var (
	// WD_RE = ^\s*working-directory:\s*(\S+)\s*$
	cippWDRE = regexp.MustCompile(`^\s*working-directory:\s*(\S+)\s*$`)
	// GOTEST_RE = ^\s*go test\s*\\\s*$
	cippGoTestRE = regexp.MustCompile(`^\s*go test\s*\\\s*$`)
	// PKG_RE = ^\s*\./([A-Za-z0-9_./\-]+)/\s*\\?\s*$
	cippPkgRE = regexp.MustCompile(`^\s*\./([A-Za-z0-9_./\-]+)/\s*\\?\s*$`)
	// END_RE = ^\s*-count=1\s*$
	cippEndRE = regexp.MustCompile(`^\s*-count=1\s*$`)
)

// cippJSONFields —— py `JSON_FIELDS`（**唯一真源**：印表与拒收同读这一处）。
var cippJSONFields = []string{"surface", "ci", "only_surface", "only_ci", "out_ci_in_ci", "out_ci"}

// cippRow —— 名单件一行：(module, pkg, tier)。
type cippRow struct{ module, pkg, tier string }

// ── 入口（= py `main()`）─────────────────────────────────────────────────────

func ciPackageParityGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① 解析实参（argparse 等价：长选项精确名 + `=` 值形态）。未知旗标 ⇒ rc=2（正文见未覆盖面①）。
	a, bad := cippParseArgs(argv)
	if bad != "" {
		fmt.Fprintf(stderr, "✗ 用法错（rc=2）：%s\n", bad)
		return 2
	}

	// ② Q-176 同类：`--json` 字段名闭集校验（**先于** VERSION 与 self-test · 与 py 同序）。
	//    ★ py 的 `if args.json:` 对空串为假 ⇒ `--json=""` 等价「没给」—— 本件同口径。
	if a.json != nil && *a.json != "" {
		var unknown []string
		for _, w := range strings.Split(*a.json, ",") {
			w = strings.TrimSpace(w)
			if w != "" && !cippIn(w, cippJSONFields) {
				unknown = append(unknown, w)
			}
		}
		if len(unknown) > 0 {
			fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", strings.Join(cippTick(unknown), " · "))
			fmt.Fprintf(stderr, "   可选字段表：%s\n", strings.Join(cippTick(cippJSONFields), " · "))
			fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开")
			return 2
		}
	}

	fmt.Fprintln(stdout, cippVersion)

	if a.selfTest {
		return cippSelfTest(stdout)
	}
	if !a.noSelfTest {
		if rcST := cippSelfTest(stdout); rcST != 0 {
			fmt.Fprintln(stdout, "⛔ 自检不过 ⇒ 拒绝判真目标（门禁自己先能被证明「会红」）")
			return 2
		}
	}

	root, err := filepath.Abs(a.root)
	if err != nil {
		root = a.root
	}
	rc, lines, data := cippJudge(root)
	for _, ln := range lines {
		fmt.Fprintln(stdout, ln)
	}
	if a.json != nil && *a.json != "" {
		var want []string
		for _, w := range strings.Split(*a.json, ",") {
			if w = strings.TrimSpace(w); w != "" {
				want = append(want, w)
			}
		}
		fmt.Fprintln(stdout, cippJSONDump(want, data))
	}
	return rc
}

// cippArgs —— 解析结果（`json` 为 nil = 未给；非 nil 含空串 = py 的「给了但空」，等价未给）。
type cippArgs struct {
	root       string
	json       *string
	selfTest   bool
	noSelfTest bool
}

// cippParseArgs —— argparse 等价解析（精确长选项 + `=`；未知 ⇒ 返非空说明，rc=2）。
func cippParseArgs(argv []string) (cippArgs, string) {
	var a cippArgs
	for i := 0; i < len(argv); i++ {
		x := argv[i]
		switch {
		case x == "--self-test":
			a.selfTest = true
		case x == "--no-self-test":
			a.noSelfTest = true
		case x == "--root":
			if i+1 >= len(argv) {
				return a, "--root 缺值"
			}
			i++
			a.root = argv[i]
		case strings.HasPrefix(x, "--root="):
			a.root = x[len("--root="):]
		case x == "--json":
			if i+1 >= len(argv) {
				return a, "--json 缺值"
			}
			i++
			v := argv[i]
			a.json = &v
		case strings.HasPrefix(x, "--json="):
			v := x[len("--json="):]
			a.json = &v
		case x == "-h" || x == "--help":
			return a, "帮助正文（见件头未覆盖面①：py 的 -h 打 usage 且 rc=0，本件 rc=2）"
		case strings.HasPrefix(x, "-") && x != "-":
			return a, "未知旗标 " + x
		default:
			return a, "未知位置参数 " + x
		}
	}
	if a.root == "" {
		// py: `--root` default=REPO_ROOT（由 __file__ 上溯两级）⇒ 本件默认 cwd（见 ★(a)）。
		if wd, err := os.Getwd(); err == nil {
			a.root = wd
		} else {
			a.root = "."
		}
	}
	return a, ""
}

// ── 解析（单一入口：自检夹具与真目标**共用同一套**）──────────────────────────────

// cippLoadSurface —— py `load_surface(path)` ⇒ (rows, problems, errMsg)；
// errMsg 非空 = py 的 `SurfaceError`（调用方给 rc=2）。
func cippLoadSurface(path string) ([]cippRow, []string, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Sprintf("名单件读不到（%s）：%s", path, cippPyOSErr(path, err))
	}
	var rows []cippRow
	var problems []string
	for i, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(cippLStrip(line), "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 3 {
			problems = append(problems, fmt.Sprintf("名单件第 %d 行：列数 %d < 3（要 `module<TAB>pkg<TAB>tier…`）", i+1, len(f)))
			continue
		}
		module, pkg, tier := strings.TrimSpace(f[0]), strings.TrimSpace(f[1]), strings.TrimSpace(f[2])
		if !cippIn(tier, cippTiers) {
			problems = append(problems, fmt.Sprintf("名单件第 %d 行：`tier` = %s 不在闭集 %s 内（判不了 ⇒ 不给结论）",
				i+1, cippPyRepr(tier), strings.Join(cippTiers, "/")))
			continue
		}
		if !cippIn(module, cippModules) {
			problems = append(problems, fmt.Sprintf("名单件第 %d 行：`module` = %s 不在 %s 内",
				i+1, cippPyRepr(module), strings.Join(cippModules, "/")))
			continue
		}
		rows = append(rows, cippRow{module, pkg, tier})
	}
	return rows, problems, ""
}

// cippParseCIPackages —— py `parse_ci_packages(text)` ⇒ ({wd: [pkg…]}, errMsg)；errMsg 非空 = `SurfaceError`。
func cippParseCIPackages(text string) (map[string][]string, string) {
	out := map[string][]string{}
	curWD := ""
	haveWD := false
	collecting := false
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if !collecting {
			if m := cippWDRE.FindStringSubmatch(line); m != nil {
				curWD = m[1]
				haveWD = true
				continue
			}
			if cippGoTestRE.MatchString(line) {
				if !haveWD {
					return nil, "`go test` 逐包块的上面没有 `working-directory:` ⇒ 判不了归属模块"
				}
				collecting = true
				if _, ok := out[curWD]; !ok {
					out[curWD] = []string{}
				}
			}
			continue
		}
		if cippEndRE.MatchString(line) {
			collecting = false
			continue
		}
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if m := cippPkgRE.FindStringSubmatch(line); m != nil {
			out[curWD] = append(out[curWD], m[1])
			continue
		}
		return nil, fmt.Sprintf("`go test` 续行里出现既不是包也不是注释的行：%s", cippPyRepr(line))
	}
	return out, ""
}

// cippJudge —— py `judge(root)` ⇒ (rc, out_lines, data)。
func cippJudge(root string) (int, []string, map[string]interface{}) {
	var out []string
	out = append(out, "── CI 逐包点名一致性：名单件 ↔ `publish/ci/ci.yml`（波A `A2` 判据② 的机检）──")
	out = append(out, fmt.Sprintf("   树：%s", root))
	tsvPath := filepath.Join(root, cippTSVRel)
	ciPath := filepath.Join(root, cippCIRel)

	rows, problems, serr := cippLoadSurface(tsvPath)
	if serr != "" {
		out = append(out, fmt.Sprintf("   ⛔ %s ⇒ 不给结论（rc=2）", serr))
		return 2, out, map[string]interface{}{"surface": nil, "ci": nil}
	}
	if len(problems) > 0 {
		for _, p := range problems {
			out = append(out, "   ⛔ "+p)
		}
		out = append(out, fmt.Sprintf("   ⛔ 名单件形态坏 %d 处 ⇒ 不给结论（rc=2）：坏行**不许**当合法行静默跳过", len(problems)))
		return 2, out, map[string]interface{}{"surface": nil, "ci": nil}
	}

	ciRaw, rerr := os.ReadFile(ciPath)
	if rerr != nil {
		out = append(out, fmt.Sprintf("   ⛔ CI 件读不到（%s）：%s ⇒ 不给结论（rc=2）", ciPath, cippPyOSErr(ciPath, rerr)))
		return 2, out, map[string]interface{}{"surface": nil, "ci": nil}
	}
	ci, perr := cippParseCIPackages(string(ciRaw))
	if perr != "" {
		out = append(out, fmt.Sprintf("   ⛔ %s ⇒ 不给结论（rc=2）", perr))
		return 2, out, map[string]interface{}{"surface": nil, "ci": nil}
	}

	inCI := cippUniqSorted(rows, true)
	outCI := cippUniqSorted(rows, false)
	named := cippNamedSorted(ci)
	out = append(out, fmt.Sprintf("   名单件（%s）：`in-ci` **%d** 个包 · 名单外（`out-ci`+`no-test`）**%d** 个包",
		cippTSVRel, len(inCI), len(outCI)))
	out = append(out, fmt.Sprintf("   `ci.yml` 逐包点名：**%d** 个包（工作目录 %s）", len(named), cippWDCounts(ci)))

	// C 守卫：任一侧 0 条 ⇒ 判据不可判（「配置没了所以全绿」是最难发现的假绿）
	if len(inCI) == 0 {
		out = append(out, "   ⛔ 守卫 C：名单件里 `in-ci` 行 **0 条** ⇒ 判据不可判 ⇒ 不给结论（rc=2）")
		return 2, out, map[string]interface{}{"surface": cippPairs(inCI), "ci": cippPairs(named)}
	}
	if len(named) == 0 {
		out = append(out, "   ⛔ 守卫 C：`ci.yml` 点名出 **0 个包** ⇒ 判据不可判 ⇒ 不给结论（rc=2）")
		return 2, out, map[string]interface{}{"surface": cippPairs(inCI), "ci": cippPairs(named)}
	}

	namedSet := map[[2]string]bool{}
	for _, x := range named {
		namedSet[x] = true
	}
	inCISet := map[[2]string]bool{}
	for _, x := range inCI {
		inCISet[x] = true
	}
	var onlySurface, onlyCI, badOut [][2]string
	for _, x := range inCI {
		if !namedSet[x] {
			onlySurface = append(onlySurface, x)
		}
	}
	for _, x := range named {
		if !inCISet[x] {
			onlyCI = append(onlyCI, x)
		}
	}
	for _, x := range outCI {
		if namedSet[x] {
			badOut = append(badOut, x)
		}
	}

	rc := 0
	if len(onlySurface) > 0 {
		out = append(out, fmt.Sprintf("   ✗ 判据 A：名单件说 `in-ci`、`ci.yml` **没点名**的包 %d 个（名单里的包 CI 根本不跑）：",
			len(onlySurface)))
		for _, x := range onlySurface {
			out = append(out, fmt.Sprintf("       ── %s  ./%s/", x[0], x[1]))
		}
		rc = 1
	}
	if len(onlyCI) > 0 {
		out = append(out, fmt.Sprintf("   ✗ 判据 A：`ci.yml` 点名、名单件**没有** `in-ci` 的包 %d 个（CI 跑了名单外的包）：",
			len(onlyCI)))
		for _, x := range onlyCI {
			out = append(out, fmt.Sprintf("       ── %s  ./%s/", x[0], x[1]))
		}
		rc = 1
	}
	if len(badOut) > 0 {
		out = append(out, fmt.Sprintf("   ✗ 判据 B：名单**外**（`out-ci`/`no-test`）的包出现在 `ci.yml` 点名表里 %d 个"+
			"（`A2` 判据② 逐字：名单外包不在 CI）：", len(badOut)))
		for _, x := range badOut {
			out = append(out, fmt.Sprintf("       ── %s  ./%s/", x[0], x[1]))
		}
		rc = 1
	}

	if rc == 0 {
		out = append(out, fmt.Sprintf("   ✅ 判据 A：两向差集皆空（`in-ci` %d ↔ 点名 %d，逐包一致）", len(inCI), len(named)))
		out = append(out, fmt.Sprintf("   ✅ 判据 B：名单外 %d 个包一个都没进 CI 点名表", len(outCI)))
		out = append(out, "⇒ 全绿（rc=0）：名单件与 `ci.yml` 的逐包点名**逐字一致**")
	} else {
		out = append(out, "⇒ 有差（rc=1）：修名单件或修 `ci.yml`（**同批改** —— 处置只有这两条，"+
			"不许放宽本判据来凑绿）")
	}
	data := map[string]interface{}{
		"surface":      cippPairs(inCI),
		"ci":           cippPairs(named),
		"only_surface": cippPairs(onlySurface),
		"only_ci":      cippPairs(onlyCI),
		"out_ci_in_ci": cippPairs(badOut),
		"out_ci":       cippPairs(outCI),
	}
	return rc, out, data
}

// ── 集合工具（与 py 的排序口径逐条对齐）──────────────────────────────────────

// cippUniqSorted —— py `sorted({(m,p) for m,p,t in rows if (t=="in-ci")==wantInCI})`。
func cippUniqSorted(rows []cippRow, wantInCI bool) [][2]string {
	seen := map[[2]string]bool{}
	var xs [][2]string
	for _, r := range rows {
		if (r.tier == "in-ci") != wantInCI {
			continue
		}
		k := [2]string{r.module, r.pkg}
		if seen[k] {
			continue
		}
		seen[k] = true
		xs = append(xs, k)
	}
	cippSortPairs(xs)
	return xs
}

// cippNamedSorted —— py `sorted({(wd,p) for wd,ps in ci.items() for p in ps})`。
func cippNamedSorted(ci map[string][]string) [][2]string {
	seen := map[[2]string]bool{}
	var xs [][2]string
	for wd, ps := range ci {
		for _, p := range ps {
			k := [2]string{wd, p}
			if seen[k] {
				continue
			}
			seen[k] = true
			xs = append(xs, k)
		}
	}
	cippSortPairs(xs)
	return xs
}

// cippSortPairs —— py 元组排序（元素序 · 逐字节）。
func cippSortPairs(xs [][2]string) {
	sort.SliceStable(xs, func(i, j int) bool {
		if xs[i][0] != xs[j][0] {
			return xs[i][0] < xs[j][0]
		}
		return xs[i][1] < xs[j][1]
	})
}

// cippWDCounts —— py `" · ".join("%s×%d" % (k, len(v)) for k,v in sorted(ci.items()))`。
func cippWDCounts(ci map[string][]string) string {
	keys := make([]string, 0, len(ci))
	for k := range ci {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s×%d", k, len(ci[k])))
	}
	return strings.Join(parts, " · ")
}

// cippPairs —— [][2]string → [][]string（供 --json 渲染）。
func cippPairs(xs [][2]string) [][]string {
	if len(xs) == 0 {
		return [][]string{}
	}
	out := make([][]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, []string{x[0], x[1]})
	}
	return out
}

// ── 自检（成对负控 · 合成夹具 · 与 py `self_test()` 逐字对齐）──────────────────────
//
//	与 py 同手段：进程内直调 `cippJudge`（不起子进程）· 夹具只在 `os.MkdirTemp` 里造。
//	临时路径**不进输出** ⇒ 12 条断言正文可逐字对拍。

// cippCITemplate —— py `CI_TEMPLATE`（两个 `%s` = core / agent 的逐包续行）。
const cippCITemplate = `jobs:
  test:
    steps:
      - name: core 构建 + 测试
        working-directory: core
        run: |
          go build ./...
          go test \
%s            -count=1
      - name: agent 构建 + 测试
        working-directory: agent
        run: |
          go build ./...
          go test \
%s            -count=1
  compat:
    steps:
      - name: 兼容层单测
        working-directory: core
        run: go test ./internal/compat/ ./internal/api/ -run 'Compat' -count=1
`

// cippGoodSurface —— py `GOOD_SURFACE`。
const cippGoodSurface = "# 合成夹具（自检用 · 不碰真目标）\n" +
	"core\tinternal/alpha\tin-ci\t纯源码判据\t—\n" +
	"core\tinternal/beta\tin-ci\t纯源码判据\t—\n" +
	"core\tcmd/gamma\tout-ci\t要私有面件\t账：第三批\n" +
	"agent\tinternal/delta\tin-ci\t纯源码判据\t—\n"

// cippPkgLines —— py `_pkg_lines(pkgs)`。
func cippPkgLines(pkgs []string) string {
	var b strings.Builder
	for _, p := range pkgs {
		b.WriteString("            ./" + p + "/ \\\n")
	}
	return b.String()
}

// cippMakeFixture —— py `make_fixture(base, surface_text, core_pkgs, agent_pkgs, drop_ci)`。
func cippMakeFixture(base, surfaceText string, corePkgs, agentPkgs []string, dropCI bool) string {
	_ = os.MkdirAll(filepath.Join(base, "scripts", "gates"), 0o755)
	_ = os.MkdirAll(filepath.Join(base, "publish", "ci"), 0o755)
	_ = os.WriteFile(filepath.Join(base, cippTSVRel), []byte(surfaceText), 0o644)
	if !dropCI {
		body := fmt.Sprintf(cippCITemplate, cippPkgLines(corePkgs), cippPkgLines(agentPkgs))
		_ = os.WriteFile(filepath.Join(base, cippCIRel), []byte(body), 0o644)
	}
	return base
}

// cippSelfTest —— py `self_test()` ⇒ 0 / 2。
func cippSelfTest(stdout io.Writer) int {
	type cippCheck struct {
		ok   bool
		desc string
	}
	var checks []cippCheck
	check := func(cond bool, desc string) {
		checks = append(checks, cippCheck{cond, desc})
		fmt.Fprintf(stdout, "   %s %s\n", map[bool]string{true: "✓", false: "✗"}[cond], desc)
	}

	t, err := os.MkdirTemp("", "ci-parity-selftest-")
	if err != nil {
		t = os.TempDir()
	}
	defer os.RemoveAll(t)

	// ① 正控：两侧逐字一致 ⇒ 绿（含一个名单外包 · 一份单行 go test 不参与判）
	b := cippMakeFixture(filepath.Join(t, "ok"), cippGoodSurface,
		[]string{"internal/alpha", "internal/beta"}, []string{"internal/delta"}, false)
	rc, _, _ := cippJudge(b)
	check(rc == 0, "① 正控 PC：名单件 `in-ci` ↔ `ci.yml` 点名**逐字一致** ⇒ rc=0（含 `out-ci` 行与单行 `go test`）")

	// ② 负控 A：名单件多一个包（CI 没点名）⇒ 必红
	b = cippMakeFixture(filepath.Join(t, "only-surface"),
		cippGoodSurface+"core\tinternal/epsilon\tin-ci\t纯源码判据\t—\n",
		[]string{"internal/alpha", "internal/beta"}, []string{"internal/delta"}, false)
	rc, _, d := cippJudge(b)
	check(rc == 1 && cippPairsEq(d["only_surface"], [][]string{{"core", "internal/epsilon"}}),
		"② 负控 A：只在一侧（名单件）存在的包 ⇒ rc=1 且**逐条点名**（判据 A 有牙）")

	// ③ 负控 B：CI 多点名一个包（名单件没有）⇒ 必红
	b = cippMakeFixture(filepath.Join(t, "only-ci"), cippGoodSurface,
		[]string{"internal/alpha", "internal/beta", "internal/zeta"}, []string{"internal/delta"}, false)
	rc, _, d = cippJudge(b)
	check(rc == 1 && cippPairsEq(d["only_ci"], [][]string{{"core", "internal/zeta"}}),
		"③ 负控 B：只在一侧（`ci.yml`）存在的包 ⇒ rc=1 且逐条点名")

	// ④ 负控 C：名单外包出现在 CI 点名表里（`A2` 判据②）⇒ 必红
	b = cippMakeFixture(filepath.Join(t, "out-ci-named"), cippGoodSurface,
		[]string{"internal/alpha", "internal/beta", "cmd/gamma"}, []string{"internal/delta"}, false)
	rc, _, d = cippJudge(b)
	check(rc == 1 && cippPairsEq(d["out_ci_in_ci"], [][]string{{"core", "cmd/gamma"}}),
		"④ 负控 C：`out-ci` 的包被 CI 点了名 ⇒ rc=1（判据 B · 名单外包不在 CI）")

	// ⑤ 负控 D：模块归属错（TSV 说 agent、`ci.yml` 挂在 core）⇒ 也算差
	b = cippMakeFixture(filepath.Join(t, "module-mismatch"),
		strings.Replace(cippGoodSurface, "agent\tinternal/delta\tin-ci", "core\tinternal/delta\tin-ci", 1),
		[]string{"internal/alpha", "internal/beta"}, []string{"internal/delta"}, false)
	rc, _, _ = cippJudge(b)
	check(rc == 1, "⑤ 负控 D：同一包名但**模块归属**不同 ⇒ rc=1（比的是 (module, pkg) 二元组，不是裸包名）")

	// ⑥ 空转 VAC：名单件一条 `in-ci` 都没有 ⇒ 不给结论（不许当绿）
	b = cippMakeFixture(filepath.Join(t, "vacuous"),
		"core\tcmd/gamma\tout-ci\t要私有面件\t账：第三批\n",
		[]string{"internal/alpha"}, nil, false)
	rc, _, _ = cippJudge(b)
	check(rc == 2, "⑥ 空转 VAC1：名单件 `in-ci` 行 0 条 ⇒ rc=2（不给结论，不许当绿）")

	// ⑦ 空转 VAC2：`ci.yml` 点名 0 个包 ⇒ 不给结论
	b = cippMakeFixture(filepath.Join(t, "vacuous-ci"), cippGoodSurface, nil, nil, false)
	rc, _, _ = cippJudge(b)
	check(rc == 2, "⑦ 空转 VAC2：`ci.yml` 点名出 0 个包 ⇒ rc=2（不给结论，不许当绿）")

	// ⑧ 缺件：`ci.yml` 不在 ⇒ 不给结论
	b = cippMakeFixture(filepath.Join(t, "no-ci"), cippGoodSurface,
		[]string{"internal/alpha"}, []string{"internal/delta"}, true)
	rc, _, _ = cippJudge(b)
	check(rc == 2, "⑧ 缺件：`ci.yml` 不在 ⇒ rc=2（缺件 ≠ 绿）")

	// ⑨ 缺件：名单件不在 ⇒ 不给结论
	_ = os.MkdirAll(filepath.Join(t, "no-tsv", "publish", "ci"), 0o755)
	_ = os.WriteFile(filepath.Join(t, "no-tsv", cippCIRel),
		[]byte(fmt.Sprintf(cippCITemplate, cippPkgLines([]string{"internal/alpha"}), cippPkgLines([]string{"internal/delta"}))), 0o644)
	rc, _, _ = cippJudge(filepath.Join(t, "no-tsv"))
	check(rc == 2, "⑨ 缺件：名单件不在 ⇒ rc=2（缺件 ≠ 绿）")

	// ⑩ 闭集：`tier` 打错 ⇒ 不给结论（不是静默跳过）
	b = cippMakeFixture(filepath.Join(t, "bad-tier"),
		cippGoodSurface+"core\tinternal/eta\tin_ci\t打错的下划线\t—\n",
		[]string{"internal/alpha", "internal/beta"}, []string{"internal/delta"}, false)
	rc, _, _ = cippJudge(b)
	check(rc == 2, "⑩ 闭集：`tier` 不在 {in-ci,out-ci,no-test} ⇒ rc=2（坏行**不许**静默跳过）")

	// ⑪ 区分度：同一份夹具，把 CI 侧一个包挪到 agent 侧之外 ⇒ 判据真的在比（不是恒绿/恒红）
	b1 := cippMakeFixture(filepath.Join(t, "disc-a"), cippGoodSurface,
		[]string{"internal/alpha", "internal/beta"}, []string{"internal/delta"}, false)
	b2 := cippMakeFixture(filepath.Join(t, "disc-b"), cippGoodSurface,
		[]string{"internal/alpha", "internal/beta"}, []string{"internal/delta", "internal/theta"}, false)
	r1, _, _ := cippJudge(b1)
	r2, _, _ := cippJudge(b2)
	check(r1 == 0 && r2 == 1, "⑪ 区分度：同夹具下「一致 ⇒ 0」「多一个包 ⇒ 1」（判据有牙）")

	// ⑫ 解析面：单行 `go test … -run`（compat job 那种）**不**进判据点名表
	if raw, e := os.ReadFile(filepath.Join(b1, cippCIRel)); e == nil {
		ci, perr := cippParseCIPackages(string(raw))
		check(perr == "" && cippStrEq(ci["core"], []string{"internal/alpha", "internal/beta"}),
			"⑫ 解析面：只认 `go test \\\\` 逐包续行块 —— 单行 `go test … -run '…'` 不混进来")
	} else {
		check(false, "⑫ 解析面：只认 `go test \\\\` 逐包续行块 —— 单行 `go test … -run '…'` 不混进来")
	}

	var bad []string
	for _, c := range checks {
		if !c.ok {
			bad = append(bad, c.desc)
		}
	}
	concl := "全过"
	if len(bad) > 0 {
		concl = "不过：" + strings.Join(bad, " · ")
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n", concl, len(checks), len(bad))
	if len(bad) > 0 {
		return 2
	}
	return 0
}

// cippPairsEq —— py 的 `[[a,b]] == [[a,b]]`（nil / 空 视为相等）。
func cippPairsEq(v interface{}, want [][]string) bool {
	got, ok := v.([][]string)
	if !ok {
		if _, isNil := v.([]interface{}); isNil {
			return len(want) == 0
		}
		return false
	}
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if len(got[i]) != len(want[i]) {
			return false
		}
		for j := range got[i] {
			if got[i][j] != want[i][j] {
				return false
			}
		}
	}
	return true
}

// cippStrEq —— 字符串切片逐元素比较。
func cippStrEq(a, b []string) bool {
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

// ── 小工具（与 py 语义逐条对齐）──────────────────────────────────────────────

// cippIn —— py `in`（列表成员）。
func cippIn(s string, xs []string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// cippLStrip —— py `str.lstrip()`（去首部空白）。
func cippLStrip(s string) string {
	return strings.TrimLeft(s, " \t\n\r\v\f")
}

// cippTick —— 每个元素用一对反引号包裹（py 的 `%s` 格式化形态）。
func cippTick(xs []string) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, "`"+x+"`")
	}
	return out
}

// cippPyRepr —— py `repr(str)`（单引号 · 非 ASCII 原样 · 转义 `\` 与 `'`）。
func cippPyRepr(s string) string {
	const bs = 0x5c
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\'':
			b.WriteString(string([]rune{bs, '\''}))
		case bs:
			b.WriteString(string([]rune{bs, bs}))
		case '\n':
			b.WriteString(string([]rune{bs, 'n'}))
		case '\r':
			b.WriteString(string([]rune{bs, 'r'}))
		case '\t':
			b.WriteString(string([]rune{bs, 't'}))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// cippPyOSErr —— py `"%s" % OSError` 的**常见两形态**逐字复刻（ENOENT / IsADirectory）。
// 其余 errno（权限等）落 Go 正文 ⇒ 见件头未覆盖面③。
func cippPyOSErr(path string, err error) string {
	if os.IsNotExist(err) {
		return "[Errno 2] No such file or directory: " + cippPyRepr(path)
	}
	if fi, e := os.Stat(path); e == nil && fi.IsDir() {
		return "[Errno 21] Is a directory: " + cippPyRepr(path)
	}
	return err.Error() // 其余形态：正文不同（未覆盖面③）
}

// cippJSONDump —— py `json.dumps({k: data.get(k) for k in want}, ensure_ascii=False, sort_keys=True)`：
// 键按字母序 · 分隔符 `, ` / `: `（与 py 默认一致 —— **不是** Go 的紧凑形态）· 缺键 ⇒ null。
func cippJSONDump(want []string, data map[string]interface{}) string {
	seen := map[string]bool{}
	var keys []string
	for _, w := range want {
		if !seen[w] {
			seen[w] = true
			keys = append(keys, w)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, cippJSONString(k)+": "+cippJSONValue(data[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// cippJSONValue —— JSON 值渲染（null / [][]string）。
func cippJSONValue(v interface{}) string {
	switch x := v.(type) {
	case [][]string:
		parts := make([]string, 0, len(x))
		for _, pair := range x {
			inner := make([]string, 0, len(pair))
			for _, s := range pair {
				inner = append(inner, cippJSONString(s))
			}
			parts = append(parts, "["+strings.Join(inner, ", ")+"]")
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return "null"
}

// cippJSONString —— JSON 串（ensure_ascii=false 语义：非 ASCII 原样）。
func cippJSONString(s string) string {
	const bs = 0x5c
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(string([]rune{bs, '"'}))
		case bs:
			b.WriteString(string([]rune{bs, bs}))
		case '\n':
			b.WriteString(string([]rune{bs, 'n'}))
		case '\r':
			b.WriteString(string([]rune{bs, 'r'}))
		case '\t':
			b.WriteString(string([]rune{bs, 't'}))
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("%s%04x", string([]rune{bs, 'u'}), r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
