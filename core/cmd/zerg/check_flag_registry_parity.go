// check_flag_registry_parity.go —— 门禁件 `scripts/gates/check-flag-registry-parity.py` 的 **Go 真搬**（v2.5.14 · M2-0 · 路B第三批 · 候选区间首字母 f–p）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据）的第 N 枚：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（4.1 M2-0 = 「无输入物料、纯静态检查的判据（读仓内件即可）」）：
//
//	✓ 选：只读仓内 `core/cmd/zerg/` 整包 `.go`（**无外部树依赖**：不碰文档仓 / 公开树 / 生成物 / 在线面）·
//	  输出**定序**（`pkg_sources` 走 `sorted(listdir)` / Go `os.ReadDir` 同为按名升序，其余面全走 `sorted(...)`）·
//	  退码 `0/1/2` 三档齐（0 对齐 / 1 新分家项 / 2 取数面拿不到）· **纯静态无写面**（不写仓、不起进程、不联网）·
//	  492 行（**行数适中**）· **无人正在改**（`git status` 现读不含它）· 不属已列重件（重件 = 801/690 行那两件）。
//	✗ 不选 `check-fields-face-parity.py`（213 行）：真身面要**现跑 `bin/zerg`**（外部制品 + 子进程）⇒ 非
//	  「纯静态」「无外部物料」。
//	✗ 不选 `check-help-snapshot.py`（281 行）：要 `bin/zerg` + **仓外文档仓**快照两样输入 ⇒ 外部树依赖。
//	✗ 不选 `check-lineref-limit.py`（337 行）/ `check-gap-ledger-face.py`（447 行）/ `check-idspace-map.py`
//	  （328 行）：都读**仓外 `Zerg-内部文档/`**（或全树 `os.walk` 裸名解算）⇒ 外部树依赖 / 遍历序非确定。
//	✗ 不选 `check-manifest-freshness*.py`：起 `git` 子进程（外部命令面）⇒ 非纯静态。
//	✗ 不选 `check-parser-discipline.py`（441 行）：扫描面走目录遍历（遍历序对拍会假差）。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-flag-registry-parity.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· 入口 `flagRegistryParityGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态：
//	  先证明「搬得逐字相同」，接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func flagRegistryParityGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 对齐 / 1 有新的分家项 / 2 取数面拿不到（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ★ 标准声明（一）· **默认根定位**（py `__file__` 上溯 ⟷ Go cwd）：
//
//	本门 py 侧**不用** `__file__` 上溯 —— `main()` 里 `ap.add_argument("root", nargs="?", default=".")`，
//	即**缺省根 = 字面 `"."`（从 cwd 取）**；Go 入口同样把缺省根取 `"."`（`filepath.Abs` 由 cwd 解）。
//	⇒ **两面同形，无差异**（依据：对拍 C2「cwd=仓根 · 不给根参数」rc/stdout/stderr 三面逐字节同）。
//	这与前几枚样板（py 从 `__file__` 上溯 ≤6 层）**不同** —— 照实声明，不套模板。
//
// ★ 标准声明（二）· **自指检查**：**是** —— 本门扫 `core/cmd/zerg/*.go` 整包（`pkg_sources` 按
//
//	`os.listdir` 取包内全部 `.go`、只排除 `_test.go`），本搬件 `check_flag_registry_parity.go`
//	**正落在被扫面内**（同时 `consts` 汇总面也吃本件的顶层 `const` 字面）。
//	依据（真判，不是照抄）：本件**落仓前后** py 侧在真仓上的 `取数：` 行逐字相同
//	（`命令树条数=166 · 名字表项数=110 · 解析面项数=45 · 用法串旗标数=158` 与透传命令列表不变 —— 见
//	`git-status-*.txt` 与 `real_before/after.txt`）。
//	按「**是则标记字面禁用反引号原串**」：本件四个**取数标记字面**（树签名 / 名字表签名 / 解析面签名 /
//	名字表终止串）一律写成**解释型字符串**（双引号 + `\"` 转义），**不用**反引号原串；且本件只贡献
//	`frp*` 前缀常量名 —— 与任何 `usage:` 键**无碰撞**，故自指面**不扰判**。
//
// 已对拍面（仓外夹具两态逐字）：全绿（指定根 / 缺省 cwd）· 三类分家项各造 violation（缺一处 / 多一处 /
// 形态）· 空转（命令树零条 ⇒ 2）· 取不到输入（根不存在 / 包目录零 `.go` ⇒ 2）· 未知旗标（argparse ⇒ 2）·
// 多余位置参数（argparse ⇒ 2）· `--json`（真仓机器面一行）· `--self-test` 十格 · `--list-rules` ·
// `-h`（argparse 帮助正文逐字）· 真仓现读。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 的**其余**用法错正文（如 `--root` 缺值 ⇒ `argument --root: expected one argument`、
//	   `--json=x` ⇒ `ignored explicit argument`）—— 本件只逐字对齐 `unrecognized arguments` 这一档。
//	② **异常路径**：`main.go` 有命令树签名但其后找不到 `"\n\t}\n"` ⇒ py `.index()` 抛 `ValueError`
//	   未捕获（进程退 1 traceback）；本件回落 `len(main)` ⇒ 退码面不同。`VF_END` 找不到同上。
//	③ py `splitlines()` 认全部 Unicode 行界（`\v`/`\f`/U+2028…），本件按 `"\n"` 切 ⇒ 含异体行界的件
//	   注释剥离粒度不同。
//	④ 非 UTF-8 件：py `open(...,encoding="utf-8")` 抛 `UnicodeDecodeError`（未捕获 ⇒ 退 1）；本件 `os.ReadFile`
//	   原样取字节 ⇒ 不退，正文另论。
//	⑤ `--root=` 空串：py `a.root_opt or a.root`（空串为假 ⇒ 落回位置参数 / `"."`）；本件同判（已对齐）。
//
//	—— ①②③④ 四面均**不在** M2-0 的样板对拍输入集内；接线前须逐面补齐或显式剔除。
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

// ── 常量（与 py 逐条对齐 · 名字前缀 `frp` 防包内同名）──────────────────────────────
const (
	frpVersion = "1.0.0"
	frpPKGRel  = "core/cmd/zerg"
	frpMainRel = "core/cmd/zerg/main.go"
)

// frpRCS —— py `RCS`（逐字；含 3 个 `=`，供自检档⑨核）。
const frpRCS = "0=对齐 · 1=有新的分家项（逐条点名）· 2=取数面拿不到（不给结论）"

// frpRules —— py `RULES`（三处字面拼接后的逐字正文）。
const frpRules = "面① 用法串 = 命令树 usage:（非字面量按包内常量名现读）；面② 名字表 = valueFlagName 体；" +
	"面③ 解析面 = valueFlagName ∪ parseInvocation 专用 case。" +
	"甲 缺一处 = 用法串取值形态的旗标 ∉ 面③（透传型命令不计）；" +
	"乙 多一处 = 面② 的名字在全部用法串里零提及；" +
	"丙 形态 = 面② 的名字在用法串里只写裸形。" +
	" 存量豁免表逐条带原因（棘轮 · 只许下调）；新增分家项 ⇒ rc=1。" +
	"面④ 读者面（按命令 · 报告档 · 不进退码）= 某命令 usage 串里的旗标 ⇒ 该命令的实现件" +
	"（定义其 run: 处理函数的包内 .go 件 · 先剥注释）必读它（字面 \"--F\" 或类型化字段 " +
	"inv.<field>）—— 缺口账 `GAP-20260927-325`（全局「零读者」是错的判据）。" +
	"面⑤ 反读面（按命令 · 报告档 · 不进退码）= 某命令实现按**取值**读了（`inv.flagVal`）某旗标 ⇒ " +
	"该命令的 usage 串必列它（面④ 的另一向 —— 「实现读 ⇒ 用法必列」，缺口 `GAP-20261005-15`）；" +
	"拒执守卫（同一函数里 `inv.flagVals`/`inv.hasFlag` 存在性检查）与别处已列 不计。"

// ★ 取数标记字面 —— 按「自指检查 = 是 ⇒ 禁用反引号原串」，一律写成**解释型字符串**（双引号 + 转义）。
const (
	frpVFSig     = "func valueFlagName(a string) string {" // py `VF_SIG`
	frpVFEnd     = "return \"\""                           // py `VF_END`
	frpParseSign = "func parseInvocation"                  // py `PARSE_SIG`
	frpTreeSign  = "commands = []*command{"                // py `TREE_SIG`
)

// frpBareSep —— py `BARE_SEP`（裸形分隔符）。
const frpBareSep = "|｜],、"

var (
	// FLAG_RE = (--[A-Za-z0-9-]+)
	frpFlagRE = regexp.MustCompile(`(--[A-Za-z0-9-]+)`)
	// CASE_RE = case a == "(--[A-Za-z0-9-]+)"
	frpCaseRE = regexp.MustCompile(`case a == "(--[A-Za-z0-9-]+)"`)
	// CASE_LIT_RE = case "(--[A-Za-z0-9-]+)"
	frpCaseLitRE = regexp.MustCompile(`case "(--[A-Za-z0-9-]+)"`)
	// PREFIX_RE = strings\.HasPrefix\(a, "(--[A-Za-z0-9-]+)=
	frpPrefixRE = regexp.MustCompile(`strings\.HasPrefix\(a, "(--[A-Za-z0-9-]+)=`)
	// PATH_RE = path:\s*\[\]string\{([^}]*)\}
	frpPathRE = regexp.MustCompile(`path:\s*\[\]string\{([^}]*)\}`)
	// RUN_RE = run:\s*([A-Za-z0-9_]+)（面④ 读者面：命令 → 处理函数）
	frpRunRE = regexp.MustCompile(`run:\s*([A-Za-z0-9_]+)`)
	// FUNC_DEF_RE = ^func (\w+)\(（面④：函数名 → 件名 · 多行锚）
	frpFuncDefRE = regexp.MustCompile(`(?m)^func ([A-Za-z0-9_]+)\(`)
	// USAGE_RE = usage:\s*(.*)
	frpUsageRE = regexp.MustCompile(`usage:\s*(.*)`)
	// USAGE_LIT_RE = "((?:[^"\\]|\\.)*)"  （py 用 .match ⇒ 本件锚 `^`）
	frpUsageLitRE = regexp.MustCompile("^\"((?:[^\"\\\\]|\\\\.)*)\"")
	// CONST_RE = const (\w+) = "((?:[^"\\]|\\.)*)"
	frpConstRE = regexp.MustCompile("const (\\w+) = \"((?:[^\"\\\\]|\\\\.)*)\"")
	// py `re.split(r"\n\t\t\{", tree)`
	frpTreeSplitRE = regexp.MustCompile("\n\t\t\\{")
	// py `re.findall(r'"([^"]+)"', ...)`
	frpQuotedRE = regexp.MustCompile(`"([^"]+)"`)
)

// frpKey —— 存量豁免表键（py 里是 `(面, 旗标)` 元组）。
type frpKey struct{ Face, Flag string }

// frpEXEMPT —— py `EXEMPT`（棘轮表；现读清空 6 → 0）。
var frpEXEMPT = map[frpKey]string{}

// frpViol —— 一条分家项（py 里是四元组）。
type frpViol struct{ Face, Flag, Where, Why string }

// frpRow —— 命令树一条（py dict 三键 + 面④ `run`）。
type frpRow struct {
	Path        string
	Usage       string
	Passthrough bool
	Run         string
}

// frpSrc —— 包内一件（名 + 已剥注释正文）。
type frpSrc struct{ Name, Body string }

// frpCounts —— `取数：` 行与 json 的 `counts`。
type frpCounts struct {
	Commands    int
	FlagNames   int
	ParseItems  int
	UsageFlags  int
	Passthrough []string
}

// ── 帮助 / 用法错正文（argparse 逐字）─────────────────────────────────────────────
const frpProgName = "check-flag-registry-parity.py"

const frpUsageBlock = "usage: " + frpProgName + " [-h] [--root ROOT_OPT] [--json]\n" +
	"                                     [--list-rules] [--self-test]\n" +
	"                                     [root]"

const frpHelpText = frpUsageBlock + "\n\n" +
	"门73 旗标两处登记面对拍（只读）\n\n" +
	"positional arguments:\n" +
	"  root             仓根（默认当前目录）\n\n" +
	"optional arguments:\n" +
	"  -h, --help       show this help message and exit\n" +
	"  --root ROOT_OPT  同位置参数（显式档）\n" +
	"  --json           末尾追加机器面一行\n" +
	"  --list-rules     打判据 / 取数口径 / 退码\n" +
	"  --self-test      只跑自检（合成夹具 · 不碰真目标）\n"

// frpUsageErr —— argparse 用法错（stderr）+ rc=2（`unrecognized arguments` 那一档逐字）。
func frpUsageErr(stderr io.Writer, msg string) {
	fmt.Fprintln(stderr, frpUsageBlock)
	fmt.Fprintf(stderr, "%s: error: %s\n", frpProgName, msg)
}

// ── 取数面（与 py 逐函数对齐）────────────────────────────────────────────────────
func frpStripComments(src string) string {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		if k := strings.Index(l, "//"); k >= 0 {
			lines[i] = l[:k]
		}
	}
	return strings.Join(lines, "\n")
}

func frpRead(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// frpPkgSources —— py `pkg_sources()`（按名升序取整包 `.go`、排除 `_test.go`）。
func frpPkgSources(root string) ([]frpSrc, error) {
	d := filepath.Join(root, frpPKGRel)
	fi, err := os.Stat(d)
	if err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("包目录不在盘：%s", frpPKGRel)
	}
	ents, err := os.ReadDir(d)
	if err != nil {
		return nil, fmt.Errorf("包目录不在盘：%s", frpPKGRel)
	}
	var out []frpSrc
	for _, e := range ents {
		n := e.Name()
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			out = append(out, frpSrc{Name: n, Body: frpStripComments(frpRead(filepath.Join(d, n)))})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("包目录下零个 .go 源件：%s", frpPKGRel)
	}
	return out, nil
}

// frpUniq —— py `list(dict.fromkeys(...))`（保持首见序去重）。
func frpUniq(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// frpParseFaces —— py `parse_faces()`（→ V / D / rows / consts）。
func frpParseFaces(root string) ([]string, map[string]bool, []frpRow, map[string]string, error) {
	srcs, err := frpPkgSources(root)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	mainSrc, ok := "", false
	for _, s := range srcs {
		if s.Name == "main.go" {
			mainSrc, ok = s.Body, true
		}
	}
	if !ok {
		return nil, nil, nil, nil, fmt.Errorf("命令树真源不在盘：%s", frpMainRel)
	}

	// 面② 名字表
	i := strings.Index(mainSrc, frpVFSig)
	if i < 0 {
		return nil, nil, nil, nil, fmt.Errorf("`%s` 找不到（名字表取数面变了 ⇒ 不给结论）", frpVFSig)
	}
	jr := strings.Index(mainSrc[i:], frpVFEnd)
	if jr < 0 {
		return nil, nil, nil, nil, fmt.Errorf("`%s` 找不到（名字表取数面变了 ⇒ 不给结论）", frpVFSig)
	}
	j := i + jr
	V := frpUniq(frpFlagRE.FindAllString(mainSrc[i:j], -1))

	// 面③ 解析面
	if !strings.Contains(mainSrc, frpParseSign) {
		return nil, nil, nil, nil, fmt.Errorf("`%s` 找不到（解析面取数面变了 ⇒ 不给结论）", frpParseSign)
	}
	p := strings.Index(mainSrc, frpParseSign)
	segEnd := len(mainSrc)
	if i > p {
		segEnd = i
	}
	seg := mainSrc[p:segEnd]
	D := map[string]bool{}
	for _, m := range frpCaseRE.FindAllStringSubmatch(seg, -1) {
		D[m[1]] = true
	}
	for _, m := range frpCaseLitRE.FindAllStringSubmatch(seg, -1) {
		D[m[1]] = true
	}
	for _, m := range frpPrefixRE.FindAllStringSubmatch(seg, -1) {
		D[m[1]] = true
	}

	// 面① 用法串（命令树逐条）
	if !strings.Contains(mainSrc, frpTreeSign) {
		return nil, nil, nil, nil, fmt.Errorf("命令树 `%s` 找不到（用法串取数面变了 ⇒ 不给结论）", frpTreeSign)
	}
	t0 := strings.Index(mainSrc, frpTreeSign)
	t1r := strings.Index(mainSrc[t0:], "\n\t}\n")
	if t1r < 0 {
		t1r = len(mainSrc) - t0 // py 此处抛 ValueError（未捕获 ⇒ 退 1）—— 见件头未覆盖面②
	}
	t1 := t0 + t1r
	tree := mainSrc[t0:t1]

	consts := map[string]string{}
	for _, s := range srcs {
		for _, m := range frpConstRE.FindAllStringSubmatch(s.Body, -1) {
			if _, dup := consts[m[1]]; !dup {
				consts[m[1]] = m[2]
			}
		}
	}

	var rows []frpRow
	for _, block := range frpTreeSplitRE.Split(tree, -1) {
		mp := frpPathRE.FindStringSubmatch(block)
		if mp == nil {
			continue
		}
		var parts []string
		for _, q := range frpQuotedRE.FindAllStringSubmatch(mp[1], -1) {
			parts = append(parts, q[1])
		}
		path := strings.Join(parts, "/")
		mu := frpUsageRE.FindStringSubmatch(block)
		if mu == nil {
			continue
		}
		raw := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(mu[1]), ","))
		usage := ""
		if strings.HasPrefix(raw, "\"") {
			if lit := frpUsageLitRE.FindStringSubmatch(raw); lit != nil {
				usage = lit[1]
			}
		} else {
			key := ""
			if f := strings.Fields(raw); len(f) > 0 {
				key = strings.Trim(f[0], ",")
			}
			usage = consts[key]
		}
		run := ""
		if mr := frpRunRE.FindStringSubmatch(block); mr != nil {
			run = mr[1]
		}
		rows = append(rows, frpRow{Path: path, Usage: usage,
			Passthrough: strings.Contains(block, "passthrough: true"), Run: run})
	}
	if len(rows) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("命令树解析出 0 条命令（解析不出 = 判不了，不给结论）")
	}
	if len(V) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("面② 名字表解析出 0 项（空转 = 假绿，不给结论）")
	}
	return V, D, rows, consts, nil
}

// frpValueForm —— py `value_form()`。
func frpValueForm(usage string, pos int) bool {
	if pos >= len(usage) {
		return false
	}
	c := usage[pos]
	if c == '=' || c == ':' {
		return true
	}
	t := strings.TrimLeft(usage[pos:], " \t\n\v\f\r\u00a0")
	if t == "" {
		return false
	}
	r := []rune(t)[0]
	if strings.ContainsRune(frpBareSep, r) {
		return false
	}
	if strings.HasPrefix(t, "--") {
		return false
	}
	return true
}

// ── 面④ 读者面（按命令 · 缺口账 `GAP-20260927-325`）──────────────────────────────────
// 「某命令 usage 串里的旗标 ⇒ 该命令的实现件必读」—— 按命令判（全局「零读者」是错的判据）。
// 「实现件」= 定义该命令 `run:` 处理函数的包内 `.go` 件（先剥注释）。
// 「读了」= 字面 `"--F"` 出现在实现件正文里，或 `--F → inv.<field>` 且实现件出现 `inv.<field>`。
// ★ 首版 = 报告档（逐条列出 · 不进退码）。
type frpNoReader struct{ Path, Flag, Impl string }

// frpGlobalOpts —— 由解析器/派发层代表**所有**命令消费的全局选项（面④ 不计 · 与 py 同名同集）。
var frpGlobalOpts = map[string]bool{"--json": true, "--dry-run": true, "--yes": true, "--confirm": true, "--help": true}

// frpFlagField —— 旗标 → 结构体字段（读面经 `inv.<field>` 而不写字面时的替代口径 · 与 py 同表）。
var frpFlagField = map[string]string{
	"--resume": "resume", "--declared": "declared", "--verified": "verified",
	"--last": "last", "--list": "list", "--all": "all", "--fast": "fast", "--full": "full",
	"--to": "to", "--reset": "reset", "--gate": "stageGate",
	"--model": "modelWant", "--slice-id": "sliceID", "--depends-on": "dependsOn",
	"--acceptance": "acceptanceDeclared", "--desc": "desc", "--priority": "priority",
	"--accept": "acceptWant", "--node": "nodes", "--context": "contextWant",
	"--schema": "schemaWant", "--wait": "wait", "--no-wait": "noWait", "--follow": "follow",
	"--quick": "quick", "--self-test": "selfTest", "--direct": "direct", "--ttl": "ttl",
	"--reload": "reload", "--force": "force", "--idempotency-key": "idemKey",
	"--manifest-only": "manifestOnly", "--plain": "plain",
	"--deep": "deep", "--for-human": "forHuman", "--for-model": "forModel",
	"--strict": "strict", "--wide": "wide",
}

func frpReaderFace(root string) ([]frpNoReader, error) {
	srcs, err := frpPkgSources(root)
	if err != nil {
		return nil, err
	}
	funcFile := map[string]string{}
	for _, s := range srcs {
		for _, m := range frpFuncDefRE.FindAllStringSubmatch(s.Body, -1) {
			if _, dup := funcFile[m[1]]; !dup {
				funcFile[m[1]] = s.Name
			}
		}
	}
	_, _, rows, _, err := frpParseFaces(root)
	if err != nil {
		return nil, err
	}
	out := []frpNoReader{}
	for _, r := range rows {
		if r.Passthrough {
			continue
		}
		impl := funcFile[r.Run]
		body := ""
		if impl != "" {
			for _, s := range srcs {
				if s.Name == impl {
					body = s.Body
				}
			}
		}
		flags := frpUniq(frpFlagRE.FindAllString(r.Usage, -1))
		sort.Strings(flags)
		for _, f := range flags {
			if frpGlobalOpts[f] {
				continue
			}
			if strings.Contains(body, "\""+f+"\"") {
				continue
			}
			if fld, ok := frpFlagField[f]; ok && regexp.MustCompile(`inv\.`+regexp.QuoteMeta(fld)+`\b`).MatchString(body) {
				continue
			}
			tag := impl
			if tag == "" {
				tag = "（取不到实现件）"
			}
			out = append(out, frpNoReader{Path: r.Path, Flag: f, Impl: tag})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Flag != out[j].Flag {
			return out[i].Flag < out[j].Flag
		}
		return out[i].Impl < out[j].Impl
	})
	return out, nil
}

// ── 面⑤ 反读面（按命令 · 缺口账 `GAP-20261005-15`）────────────────────────────────
// 与面④ 同一条判据的另一向：「某命令实现按**取值**读了（`inv.flagVal`）某旗标 ⇒ 该命令的 usage
// 串必列它」（面④ = 「usage 有 ⇒ 实现必读」）。三处收窄（与 py `no_usage_face` 逐行同源）：
// 可达函数闭包 / 取值读 / 拒执守卫（同函数里 `flagVals`/`hasFlag` 存在性检查）与别处已列 不计。
// ★ 首版 = 报告档（不进退码）。缺口 `GAP-20261005-15`：`gap add` 读 `--state` 而用法串不列它。
type frpNoUsage struct{ Path, Flag, Fn, Impl string }

var (
	// py `GREV_RDV_RE`
	frpRevValRE = regexp.MustCompile(`inv\.flagVal\(\s*"(--[A-Za-z0-9-]+)"`)
	// py `GREV_RDP_RE`
	frpRevPresRE = regexp.MustCompile(`inv\.(?:flagVals|hasFlag)\(\s*"(--[A-Za-z0-9-]+)"`)
	// py `GREV_CALL_RE`
	frpRevCallRE = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

// frpRevFuncBodies —— py `rev_func_bodies()`（函数名 → (件名, 函数体)；同名取首见 · 按件名升序）。
func frpRevFuncBodies(srcs []frpSrc) map[string][2]string {
	out := map[string][2]string{}
	for _, s := range srcs {
		ms := frpFuncDefRE.FindAllStringSubmatchIndex(s.Body, -1)
		for i, m := range ms {
			a := m[0]
			b := len(s.Body)
			if i+1 < len(ms) {
				b = ms[i+1][0]
			}
			name := s.Body[m[2]:m[3]]
			if _, dup := out[name]; !dup {
				out[name] = [2]string{s.Name, s.Body[a:b]}
			}
		}
	}
	return out
}

// frpRevReach —— py `rev_reach()`（从 run 出发、在本包内的可达函数闭包 · 含 run 自己）。
func frpRevReach(bodies map[string][2]string, run string) map[string]bool {
	seen := map[string]bool{}
	stack := []string{run}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[f] {
			continue
		}
		ent, ok := bodies[f]
		if !ok {
			continue
		}
		seen[f] = true
		for _, c := range frpRevCallRE.FindAllStringSubmatch(ent[1], -1) {
			if _, ok2 := bodies[c[1]]; ok2 && !seen[c[1]] {
				stack = append(stack, c[1])
			}
		}
	}
	return seen
}

// frpNoUsageFace —— py `no_usage_face()`（面⑤ 反读面 · 报告档 · 不进退码）。
func frpNoUsageFace(root string) ([]frpNoUsage, error) {
	srcs, err := frpPkgSources(root)
	if err != nil {
		return nil, err
	}
	bodies := frpRevFuncBodies(srcs)
	_, _, rows, _, err := frpParseFaces(root)
	if err != nil {
		return nil, err
	}
	out := []frpNoUsage{}
	for _, r := range rows {
		if r.Passthrough || r.Run == "" {
			continue
		}
		seen := frpRevReach(bodies, r.Run)
		documented := map[string]bool{}
		for _, f := range frpFlagRE.FindAllString(r.Usage, -1) {
			documented[f] = true
		}
		for _, o := range rows {
			if seen[o.Run] && o.Usage != "" {
				for _, f := range frpFlagRE.FindAllString(o.Usage, -1) {
					documented[f] = true
				}
			}
		}
		names := make([]string, 0, len(seen))
		for f := range seen {
			names = append(names, f)
		}
		sort.Strings(names)
		for _, f := range names {
			body := bodies[f][1]
			present := map[string]bool{}
			for _, m := range frpRevPresRE.FindAllStringSubmatch(body, -1) {
				present[m[1]] = true
			}
			gs := map[string]bool{}
			for _, m := range frpRevValRE.FindAllStringSubmatch(body, -1) {
				gs[m[1]] = true
			}
			glist := make([]string, 0, len(gs))
			for g := range gs {
				glist = append(glist, g)
			}
			sort.Strings(glist)
			for _, g := range glist {
				if frpGlobalOpts[g] || present[g] || documented[g] {
					continue
				}
				out = append(out, frpNoUsage{r.Path, g, f, bodies[f][0]})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Flag != out[j].Flag {
			return out[i].Flag < out[j].Flag
		}
		if out[i].Fn != out[j].Fn {
			return out[i].Fn < out[j].Fn
		}
		return out[i].Impl < out[j].Impl
	})
	return out, nil
}

// frpCompute —— py `compute()`。
func frpCompute(root string) ([]frpViol, []frpKey, []frpKey, []frpKey, frpCounts, []frpNoReader, []frpNoUsage, error) {
	V, D, rows, _, err := frpParseFaces(root)
	if err != nil {
		return nil, nil, nil, nil, frpCounts{}, nil, nil, err
	}
	known := map[string]bool{}
	for _, f := range V {
		known[f] = true
	}
	for f := range D {
		known[f] = true
	}

	anyUse := map[string]map[string]bool{}
	valUse := map[string]map[string]bool{}
	for _, r := range rows {
		for _, loc := range frpFlagRE.FindAllStringSubmatchIndex(r.Usage, -1) {
			f := r.Usage[loc[2]:loc[3]]
			if anyUse[f] == nil {
				anyUse[f] = map[string]bool{}
			}
			anyUse[f][r.Path] = true
			if frpValueForm(r.Usage, loc[3]) {
				if valUse[f] == nil {
					valUse[f] = map[string]bool{}
				}
				valUse[f][r.Path] = true
			}
		}
	}

	var viol []frpViol
	var exemptUsed []frpKey

	valKeys := make([]string, 0, len(valUse))
	for f := range valUse {
		valKeys = append(valKeys, f)
	}
	sort.Strings(valKeys)
	for _, f := range valKeys {
		if known[f] {
			continue
		}
		var live []string
		for p := range valUse[f] {
			pass := false
			for _, r := range rows {
				if r.Path == p && r.Passthrough {
					pass = true
					break
				}
			}
			if !pass {
				live = append(live, p)
			}
		}
		if len(live) == 0 {
			continue // 只在透传型命令的用法串里 ⇒ 设计如此
		}
		sort.Strings(live)
		key := frpKey{"缺一处", f}
		if _, ok := frpEXEMPT[key]; ok {
			exemptUsed = append(exemptUsed, key)
			continue
		}
		viol = append(viol, frpViol{"缺一处", f, "zerg " + strings.ReplaceAll(live[0], "/", " "),
			"用法串写着取值形态，解析面（名字表 ∪ 专用 case）不认 ⇒ 真给就退 2「未知旗标」"})
	}

	vKeys := append([]string(nil), V...)
	sort.Strings(vKeys)
	for _, f := range vKeys {
		if len(anyUse[f]) == 0 {
			key := frpKey{"幽灵", f}
			if _, ok := frpEXEMPT[key]; ok {
				exemptUsed = append(exemptUsed, key)
				continue
			}
			viol = append(viol, frpViol{"多一处", f, "（全表）",
				"名字表有它、全部用法串零提及 ⇒ 上了户口却无人可见"})
			continue
		}
		var bare []string
		for p := range anyUse[f] {
			if !valUse[f][p] {
				bare = append(bare, p)
			}
		}
		if len(bare) > 0 {
			sort.Strings(bare)
			key := frpKey{"形态", f}
			if _, ok := frpEXEMPT[key]; ok {
				exemptUsed = append(exemptUsed, key)
				continue
			}
			viol = append(viol, frpViol{"形态", f, "zerg " + strings.ReplaceAll(bare[0], "/", " "),
				fmt.Sprintf("名字表按**取值**收（%d 条用法串里 %d 处写成裸形）⇒ 值/无值两态不一致",
					len(anyUse[f]), len(bare))})
		}
	}

	used := map[frpKey]bool{}
	for _, k := range exemptUsed {
		used[k] = true
	}
	var stale, noReason []frpKey
	for k := range frpEXEMPT {
		if !used[k] {
			stale = append(stale, k)
		}
	}
	for k, v := range frpEXEMPT {
		if strings.TrimSpace(v) == "" {
			noReason = append(noReason, k)
		}
	}
	frpSortKeys(stale)
	frpSortKeys(noReason)

	var pass []string
	for _, r := range rows {
		if r.Passthrough {
			pass = append(pass, r.Path)
		}
	}
	sort.Strings(pass)
	counts := frpCounts{Commands: len(rows), FlagNames: len(V), ParseItems: len(D),
		UsageFlags: len(anyUse), Passthrough: pass}
	noReader, err := frpReaderFace(root)
	if err != nil {
		return nil, nil, nil, nil, frpCounts{}, nil, nil, err
	}
	noUsage, err := frpNoUsageFace(root)
	if err != nil {
		return nil, nil, nil, nil, frpCounts{}, nil, nil, err
	}
	return viol, exemptUsed, stale, noReason, counts, noReader, noUsage, nil
}

func frpSortKeys(ks []frpKey) {
	sort.Slice(ks, func(a, b int) bool {
		if ks[a].Face != ks[b].Face {
			return ks[a].Face < ks[b].Face
		}
		return ks[a].Flag < ks[b].Flag
	})
}

// frpPyListRepr —— py 列表 repr（`['a', 'b']`；空 ⇒ `[]`）。
func frpPyListRepr(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = "'" + s + "'"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// frpJSONStr —— Python `json.dumps(..., ensure_ascii=False)` 的字符串转义子集。
func frpJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// frpReport —— py `report()`（末尾按需追加 json 一行）。
func frpReport(viol []frpViol, exemptUsed []frpKey, stale []frpKey, noReason []frpKey,
	counts frpCounts, root string, asJSON bool, noReader []frpNoReader, noUsage []frpNoUsage) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("门73 旗标两处登记面对拍（用法串 ⟷ valueFlagName 名字表 + 实现件读者面）· v%s", frpVersion))
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	lines = append(lines, "仓根："+abs)
	lines = append(lines, "取数："+fmt.Sprintf("命令树条数=%d · 名字表项数=%d · 解析面项数=%d · 用法串旗标数=%d · 透传命令=%s",
		counts.Commands, counts.FlagNames, counts.ParseItems, counts.UsageFlags, frpPyListRepr(counts.Passthrough)))
	lines = append(lines, "判据："+frpRules)
	if len(viol) > 0 {
		lines = append(lines, fmt.Sprintf("判红 %d 项（逐条点名 · 面/旗标/哪条命令）：", len(viol)))
		for _, v := range viol {
			lines = append(lines, fmt.Sprintf("  \u2717 [%s] %s · %s · %s", v.Face, v.Flag, v.Where, v.Why))
		}
	} else {
		lines = append(lines, "新分家项 0 项：两面（除存量豁免）对齐。")
	}
	// ── 面④ 读者面（按命令 · 报告档 · 缺口账 `GAP-20260927-325`）──
	lines = append(lines, fmt.Sprintf("读者面（按命令 · 报告档 · 不进退码 · 缺口账 `GAP-20260927-325`）：「某命令 usage 串里的旗标 ⇒ 该命令的实现件必读」——本命令实现件不读的 %d 项：", len(noReader)))
	for _, nr := range noReader {
		lines = append(lines, fmt.Sprintf("  · zerg %s · %s · 实现件 %s", strings.ReplaceAll(nr.Path, "/", " "), nr.Flag, nr.Impl))
	}
	// ── 面⑤ 反读面（按命令 · 报告档 · 缺口账 `GAP-20261005-15`）──
	lines = append(lines, fmt.Sprintf("反读面（按命令 · 报告档 · 不进退码 · 缺口账 `GAP-20261005-15`）：「某命令实现按取值读（`inv.flagVal`）了某旗标 ⇒ 该命令的 usage 串必列它」（面④ 的另一向；拒执守卫 / 别处已列 不计）——本命令实现读了而 usage 没列的 %d 项：", len(noUsage)))
	for _, nu := range noUsage {
		lines = append(lines, fmt.Sprintf("  · zerg %s · %s · 读者函数 %s（%s）", strings.ReplaceAll(nu.Path, "/", " "), nu.Flag, nu.Fn, nu.Impl))
	}
	lines = append(lines, fmt.Sprintf("存量豁免 %d 项（棘轮 · 逐条打印 · 不进退码）：", len(exemptUsed)))
	frpSortKeys(exemptUsed)
	for _, k := range exemptUsed {
		lines = append(lines, fmt.Sprintf("  ~ [%s] %s · %s", k.Face, k.Flag, frpEXEMPT[k]))
	}
	if len(stale) > 0 {
		lines = append(lines, "★ 以下豁免行**已不再分家**（修好了）⇒ 请删掉本件 EXEMPT 里这一行（棘轮只许下调）：")
		for _, k := range stale {
			lines = append(lines, fmt.Sprintf("  ~ [%s] %s", k.Face, k.Flag))
		}
	}
	if len(noReason) > 0 {
		lines = append(lines, "判红：以下豁免行**没写原因**（不许无理由豁免）：")
		for _, k := range noReason {
			lines = append(lines, fmt.Sprintf("  ~ [%s] %s", k.Face, k.Flag))
		}
	}
	out := strings.Join(lines, "\n")
	if asJSON {
		out += "\n" + frpJSONDoc(viol, exemptUsed, stale, noReason, counts, noReader, noUsage)
	}
	return out
}

func frpJSONDoc(viol []frpViol, exemptUsed []frpKey, stale []frpKey, noReason []frpKey, counts frpCounts, noReader []frpNoReader, noUsage []frpNoUsage) string {
	var b strings.Builder
	b.WriteString("{\"gate\": \"flag-registry-parity\", \"version\": " + frpJSONStr(frpVersion) + ", \"violations\": [")
	for i, v := range viol {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{\"face\": " + frpJSONStr(v.Face) + ", \"flag\": " + frpJSONStr(v.Flag) +
			", \"where\": " + frpJSONStr(v.Where) + ", \"why\": " + frpJSONStr(v.Why) + "}")
	}
	b.WriteString("], \"exempt\": [")
	for i, k := range exemptUsed {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{\"face\": " + frpJSONStr(k.Face) + ", \"flag\": " + frpJSONStr(k.Flag) +
			", \"reason\": " + frpJSONStr(frpEXEMPT[k]) + "}")
	}
	b.WriteString("], \"stale_exempt\": [")
	for i, k := range stale {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{\"face\": " + frpJSONStr(k.Face) + ", \"flag\": " + frpJSONStr(k.Flag) + "}")
	}
	b.WriteString("], \"exempt_without_reason\": [")
	for i, k := range noReason {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{\"face\": " + frpJSONStr(k.Face) + ", \"flag\": " + frpJSONStr(k.Flag) + "}")
	}
	b.WriteString("], \"counts\": {\"命令树条数\": " + fmt.Sprintf("%d", counts.Commands) +
		", \"名字表项数\": " + fmt.Sprintf("%d", counts.FlagNames) +
		", \"解析面项数\": " + fmt.Sprintf("%d", counts.ParseItems) +
		", \"用法串旗标数\": " + fmt.Sprintf("%d", counts.UsageFlags) +
		", \"透传命令\": [")
	for i, p := range counts.Passthrough {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(frpJSONStr(p))
	}
	b.WriteString("]}")
	b.WriteString(", \"no_reader\": [")
	for i, nr := range noReader {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{\"command\": " + frpJSONStr(nr.Path) + ", \"flag\": " + frpJSONStr(nr.Flag) +
			", \"impl\": " + frpJSONStr(nr.Impl) + "}")
	}
	b.WriteString("], \"no_usage\": [")
	for i, nu := range noUsage {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("{\"command\": " + frpJSONStr(nu.Path) + ", \"flag\": " + frpJSONStr(nu.Flag) +
			", \"func\": " + frpJSONStr(nu.Fn) + ", \"impl\": " + frpJSONStr(nu.Impl) + "}")
	}
	b.WriteString("]}")
	return b.String()
}

// frpRun —— py `run()`（Bail ⇒ 两行 + rc=2；否则 report + 退码行）。
func frpRun(stdout io.Writer, root string, asJSON bool) int {
	viol, exemptUsed, stale, noReason, counts, noReader, noUsage, err := frpCompute(root)
	if err != nil {
		fmt.Fprintf(stdout, "门73 旗标两处登记面对拍：**判不了**（%s）⇒ rc=2 不给结论\n", err.Error())
		fmt.Fprintf(stdout, "退码：%s\n", frpRCS)
		return 2
	}
	fmt.Fprintln(stdout, frpReport(viol, exemptUsed, stale, noReason, counts, root, asJSON, noReader, noUsage))
	fmt.Fprintf(stdout, "退码：%s\n", frpRCS)
	if len(viol) > 0 || len(noReason) > 0 {
		return 1
	}
	return 0
}

// ── 自检夹具（与 py `FIXTURE_MAIN` / `FIXTURE_EXTRA` 逐字；本件里用解释型字符串）────────
const frpFixtureMain = "package zerg\n" +
	"\n" +
	"type command struct {\n" +
	"\tpath        []string\n" +
	"\tusage       string\n" +
	"\tpassthrough bool\n" +
	"}\n" +
	"\n" +
	"var commands []*command\n" +
	"\n" +
	"func init() {\n" +
	"\tcommands = []*command{\n" +
	"\t\t{\n" +
	"\t\t\tpath:  []string{\"task\", \"submit\"},\n" +
	"\t\t\tusage: \"zerg task submit --desc <描述> [--slice-id <片>] [--all]\",\n" +
	"\t\t},\n" +
	"\t\t{\n" +
	"\t\t\tpath:  []string{\"gate\", \"run\"},\n" +
	"\t\t\tusage: \"zerg gate run [--scope <s> | --fast] [--outdir <目录>]\",\n" +
	"\t\t\tpassthrough: true,\n" +
	"\t\t},\n" +
	"\t}\n" +
	"}\n" +
	"\n" +
	"func parseInvocation(args []string) {\n" +
	"\tfor _, a := range args {\n" +
	"\t\tswitch {\n" +
	"\t\tcase a == \"--all\":\n" +
	"\t\tcase a == \"--json\":\n" +
	"\t\t\t_ = a\n" +
	"\t\tcase valueFlagName(a) != \"\":\n" +
	"\t\t\t_ = a\n" +
	"\t\t}\n" +
	"\t}\n" +
	"}\n" +
	"\n" +
	"// valueFlagName —— 唯一真源。注释里写 `--zzz-注释` 不许被读成登记。\n" +
	"func valueFlagName(a string) string {\n" +
	"\tswitch a {\n" +
	"\tcase \"--desc\", \"--slice-id\":\n" +
	"\t\treturn a\n" +
	"\t}\n" +
	"\treturn \"\"\n" +
	"}\n"

const frpFixtureExtra = "package zerg\n" +
	"\n" +
	"const extraUsage = \"zerg probe [--root <仓根>]\"\n"

const frpEmptyMain = "package zerg\n\nfunc valueFlagName(a string) string {\n\treturn \"\"\n}\n\n" +
	"func parseInvocation(a []string) {}\n\nvar commands []*command\n"

func frpMkFixture(base string) string {
	d := filepath.Join(base, frpPKGRel)
	_ = os.MkdirAll(d, 0o755)
	_ = os.WriteFile(filepath.Join(d, "main.go"), []byte(frpFixtureMain), 0o644)
	_ = os.WriteFile(filepath.Join(d, "family_probe.go"), []byte(frpFixtureExtra), 0o644)
	return base
}

// frpSelfTest —— py `self_test()`（十格 · 打印逐字）。
func frpSelfTest(stdout io.Writer) int {
	type check struct {
		name string
		ok   bool
		det  string
	}
	var checks []check
	add := func(name string, ok bool, det string) {
		checks = append(checks, check{name, ok, det})
	}

	base, err := os.MkdirTemp("", "flagparity-selftest-")
	if err != nil {
		base = os.TempDir()
	}
	defer os.RemoveAll(base)

	runOn := func(root string) (string, []frpViol, string) {
		v, _, _, _, _, _, _, err := frpCompute(root)
		if err != nil {
			return "bail", nil, err.Error()
		}
		return "ok", v, ""
	}
	hasViol := func(v []frpViol, face, flag string) bool {
		for _, x := range v {
			if x.Face == face && x.Flag == flag {
				return true
			}
		}
		return false
	}

	d := frpMkFixture(filepath.Join(base, "aligned"))
	st, v, det := runOn(d)
	add("①对齐 ⇒ 无分家项", st == "ok" && len(v) == 0, st+" | "+det)
	add("①' 透传命令的旗标不进判红（--scope 不在解析面）",
		st == "ok" && !hasViol(v, "缺一处", "--scope"), st+" | "+det)

	d = frpMkFixture(filepath.Join(base, "missing"))
	pm := filepath.Join(d, frpPKGRel, "main.go")
	s := frpRead(pm)
	_ = os.WriteFile(pm, []byte(strings.Replace(s, "case \"--desc\", \"--slice-id\":", "case \"--slice-id\":", 1)), 0o644)
	st, v, det = runOn(d)
	add("②缺一处（表里删 --desc）⇒ 判红且**点名 --desc**",
		st == "ok" && hasViol(v, "缺一处", "--desc"), st+" | "+det)

	d = frpMkFixture(filepath.Join(base, "extra"))
	pm = filepath.Join(d, frpPKGRel, "main.go")
	s = frpRead(pm)
	_ = os.WriteFile(pm, []byte(strings.Replace(s,
		"case \"--desc\", \"--slice-id\":", "case \"--desc\", \"--slice-id\", \"--zz-new\":", 1)), 0o644)
	st, v, det = runOn(d)
	add("③多一处（表里加一枚没人写的名）⇒ 判红且**点名 --zz-new**",
		st == "ok" && hasViol(v, "多一处", "--zz-new"), st+" | "+det)

	d = frpMkFixture(filepath.Join(base, "form"))
	pm = filepath.Join(d, frpPKGRel, "main.go")
	s = frpRead(pm)
	_ = os.WriteFile(pm, []byte(strings.Replace(s, "--slice-id <片>", "--slice-id", 1)), 0o644)
	st, v, det = runOn(d)
	add("④形态（用法串取值改裸形）⇒ 判红且**点名 --slice-id**",
		st == "ok" && hasViol(v, "形态", "--slice-id"), st+" | "+det)

	d = frpMkFixture(filepath.Join(base, "nofile"))
	_ = os.Remove(filepath.Join(d, frpPKGRel, "main.go"))
	st, v, det = runOn(d)
	add("⑤命令树真源不在盘 ⇒ 判不了（Bail ⇒ rc=2）", st == "bail", st+" | "+det)

	d = filepath.Join(base, "empty")
	_ = os.MkdirAll(filepath.Join(d, frpPKGRel), 0o755)
	_ = os.WriteFile(filepath.Join(d, frpPKGRel, "main.go"), []byte(frpEmptyMain), 0o644)
	st, v, det = runOn(d)
	add("⑥命令树零条 ⇒ 判不了（不给结论）", st == "bail", st+" | "+det)

	d = frpMkFixture(filepath.Join(base, "nocomment"))
	st, v, det = runOn(d)
	add("⑦注释里的旗标名不被读成登记（面② 不含 --zzz-注释）",
		st == "ok" && !hasViol(v, "多一处", "--zzz-注释"), st+" | "+det)

	exOk := true
	for _, r := range frpEXEMPT {
		if strings.TrimSpace(r) == "" {
			exOk = false
		}
	}
	add("⑧豁免行必须带原因（EXEMPT 逐行有值）", exOk, "")

	add("⑨判据只有一处定义（本件内 RULES 与三档退码常量在位）",
		strings.Contains(frpRules, "缺一处") && strings.Contains(frpRules, "多一处") &&
			strings.Contains(frpRules, "形态") && strings.Count(frpRCS, "=") == 3, frpRCS)

	// 面④ 读者面（按命令）· 成对负控：impl 不读 ⇒ 列缺读者；补上读者 ⇒ 不再列。
	d = frpMkFixture(filepath.Join(base, "reader_missing"))
	nr, _ := frpReaderFace(d)
	hasAll := false
	for _, x := range nr {
		if x.Flag == "--all" {
			hasAll = true
		}
	}
	add("④读者面（负控）：run 指不到实现件 ⇒ 非全局 usage 旗标列缺读者", hasAll, "")
	d = frpMkFixture(filepath.Join(base, "reader_ok"))
	pm = filepath.Join(d, frpPKGRel, "main.go")
	tt := frpRead(pm)
	tt = strings.Replace(tt,
		"\t\t\tusage: \"zerg task submit --desc <描述> [--slice-id <片>] [--all]\",\n\t\t},",
		"\t\t\tusage: \"zerg task submit --desc <描述> [--slice-id <片>] [--all]\",\n\t\t\trun:   cmdTaskSubmit,\n\t\t},", 1)
	tt += "\nfunc cmdTaskSubmit() { _ = \"--desc\"; _ = \"--slice-id\"; _ = \"--all\" }\n"
	_ = os.WriteFile(pm, []byte(tt), 0o644)
	nr2, _ := frpReaderFace(d)
	stillThere := false
	for _, x := range nr2 {
		if x.Path == "task/submit" {
			stillThere = true
		}
	}
	add("④读者面（正控）：实现件读全 ⇒ 该命令不再列缺读者", !stillThere, "")

	// 面⑤ 反读面（按命令）· 成对负控：impl 按取值读但 usage 没列 ⇒ 列；usage 列了 ⇒ 不再列。
	d = frpMkFixture(filepath.Join(base, "nousage_missing"))
	pm = filepath.Join(d, frpPKGRel, "main.go")
	t5 := strings.Replace(frpRead(pm),
		"\t\t\tpassthrough: true,\n\t\t},\n\t}\n}",
		"\t\t\tpassthrough: true,\n\t\t},\n"+
			"\t\t{\n\t\t\tpath:  []string{\"probe\", \"x\"},\n"+
			"\t\t\tusage: \"zerg probe x\",\n\t\t\trun:   cmdProbeNoUsage,\n\t\t},\n\t}\n}", 1)
	t5 += "\nfunc cmdProbeNoUsage() { _ = inv.flagVal(\"--zz-read\") }\n"
	_ = os.WriteFile(pm, []byte(t5), 0o644)
	nu, _ := frpNoUsageFace(d)
	hitNU := false
	for _, x := range nu {
		if x.Path == "probe/x" && x.Flag == "--zz-read" {
			hitNU = true
		}
	}
	add("⑤反读面（负控）：实现按取值读但 usage 没列 ⇒ 列出该命令", hitNU, "")

	d = frpMkFixture(filepath.Join(base, "nousage_ok"))
	pm = filepath.Join(d, frpPKGRel, "main.go")
	t6 := strings.Replace(frpRead(pm),
		"\t\t\tpassthrough: true,\n\t\t},\n\t}\n}",
		"\t\t\tpassthrough: true,\n\t\t},\n"+
			"\t\t{\n\t\t\tpath:  []string{\"probe\", \"x\"},\n"+
			"\t\t\tusage: \"zerg probe x [--zz-read <值>]\",\n\t\t\trun:   cmdProbeNoUsage,\n\t\t},\n\t}\n}", 1)
	t6 += "\nfunc cmdProbeNoUsage() { _ = inv.flagVal(\"--zz-read\") }\n"
	_ = os.WriteFile(pm, []byte(t6), 0o644)
	nu2, _ := frpNoUsageFace(d)
	stillNU := false
	for _, x := range nu2 {
		if x.Path == "probe/x" {
			stillNU = true
		}
	}
	add("⑤反读面（正控）：usage 列了该旗标 ⇒ 该命令不再列", !stillNU, "")

	bad := 0
	for _, c := range checks {
		if !c.ok {
			bad++
		}
	}
	fmt.Fprintf(stdout, "门73 自检（成对负控 · 合成夹具）：断言 %d 条 · 失败 %d 条\n", len(checks), bad)
	for _, c := range checks {
		tag := "PASS"
		tail := ""
		if !c.ok {
			tag = "FAIL"
			det := c.det
			if len([]rune(det)) > 400 {
				det = string([]rune(det)[:400])
			}
			tail = "  ⇐ " + det
		}
		fmt.Fprintf(stdout, "  %s %s%s\n", tag, c.name, tail)
	}
	if bad > 0 {
		return 1
	}
	return 0
}

// frpSortedExemptJoined —— `--list-rules` 尾行（`面/旗标` · sorted）。
func frpSortedExemptJoined() string {
	keys := make([]frpKey, 0, len(frpEXEMPT))
	for k := range frpEXEMPT {
		keys = append(keys, k)
	}
	frpSortKeys(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k.Face + "/" + k.Flag
	}
	return strings.Join(parts, " · ")
}

// flagRegistryParityGateMain —— 入口（= py `main()`）。
func flagRegistryParityGateMain(stdout, stderr io.Writer, argv []string) int {
	rootOpt := ""
	jsonFlag, selfTest, listRules, help := false, false, false, false
	var positionals []string
	for k := 0; k < len(argv); k++ {
		a := argv[k]
		switch {
		case a == "-h" || a == "--help":
			help = true
		case a == "--self-test":
			selfTest = true
		case a == "--list-rules":
			listRules = true
		case a == "--json":
			jsonFlag = true
		case a == "--root":
			if k+1 >= len(argv) {
				frpUsageErr(stderr, "argument --root: expected one argument")
				return 2
			}
			k++
			rootOpt = argv[k]
		case strings.HasPrefix(a, "--root="):
			rootOpt = a[len("--root="):]
		case strings.HasPrefix(a, "-") && a != "-":
			frpUsageErr(stderr, "unrecognized arguments: "+a)
			return 2
		default:
			positionals = append(positionals, a)
		}
	}
	if help {
		io.WriteString(stdout, frpHelpText)
		return 0
	}
	if len(positionals) > 1 {
		frpUsageErr(stderr, "unrecognized arguments: "+strings.Join(positionals[1:], " "))
		return 2
	}
	if selfTest {
		return frpSelfTest(stdout)
	}
	if listRules {
		fmt.Fprintln(stdout, frpRules)
		fmt.Fprintf(stdout, "退码：%s\n", frpRCS)
		fmt.Fprintf(stdout, "存量豁免 %d 项：%s\n", len(frpEXEMPT), frpSortedExemptJoined())
		fmt.Fprintln(stdout, "面④ 读者面（按命令 · 报告档 · 不进退码）：RULES 末句 —— "+
			"「某命令 usage 串里的旗标 ⇒ 该命令的实现件必读」（缺口账 `GAP-20260927-325`）")
		fmt.Fprintln(stdout, "面⑤ 反读面（按命令 · 报告档 · 不进退码 · 缺口账 `GAP-20261005-15`）：RULES 末句 —— "+
			"「某命令实现按取值读了某旗标 ⇒ 该命令的 usage 串必列它」（面④ 的另一向）")
		return 0
	}
	root := "."
	if rootOpt != "" {
		root = rootOpt
	} else if len(positionals) == 1 {
		root = positionals[0]
	}
	return frpRun(stdout, root, jsonFlag)
}
