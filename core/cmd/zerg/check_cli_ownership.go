// check_cli_ownership.go —— 判据件 `check-cli-ownership`：**CLI 归属门**。
//
// 定位：与 (a)【自研面无新增 py/sh】（键 `check-lang-freeze` · `check_lang_freeze.go`）**成对**的第二条
// 可机检判据 —— (a) 拦「新写了 py/sh」（加法面），本门逼「**存量件接 inproc 标准路由**」（存量面）。
// 设计稿：`~/.zerg/state/设计-CLI归属判据-20261007.md`；代拍与收紧口径：
// `~/.zerg/state/任务卡-AlltoGo-20261007.md §三.3` · `~/.zerg/state/实施单-AlltoGo-b判据-v2-20260808.md`
// （键名 / 起手档 / 判词均由 Mr2109 代拍）。
//
// ★★ 薄面口径（**逐字**，✗ 谢绝语义失真）：
//
//	本门判的「打得着」= **走 inproc 标准路由** —— 即键(件) ∈ `gateCheckGoEntry`（判据臂键表）。
//	现役 forward shell 对任何 `scripts/` 下 py/sh 都能**名录可达**（键不在 inproc ⇒ 走 `exec` 兜底真跑
//	该 py/sh）⇒ 那是一条 **CLI 打点**但**不是标准路由**。故红件的判词一律写
//
//	  「**未接 inproc 标准路由**」，
//
//	✗ **禁写「打不着 / 不可达」** —— 兜底路由下它其实够得着，那样写会让语义失真（设计稿 §五-1）。
//
// 三态判定（真源全复用 · ✗ 不新造闭集表 / ✗ 不新造 cli-ownership.json）：
//
//	A 未接标准路由  候选件 ∧ 键 ∉ inproc(判据臂键表) ∧ 键 ∉ 名册 ∧ 键 ∉ 台账 ⇒ rc=1：点名
//	                「<件> 未接 inproc 标准路由（键 <键> ∉ 判据臂键表 ∧ ∉ 名册 ∧ ∉ 台账）」；
//	B 例外放行      候选件 的键 ∈ `retire_exceptions` ⇒ 放行 ＋ 逐条打印理由（category/reason）；
//	C 销账放行      候选件 的键 ∈ `retire_ledger` ⇒ 放行（唯一销账口 = 退场正门 `zerg script retire`）。
//
// 真源（单点 · ✗ 不新造第二真源）：
//
//	· 判据臂键表 = **同包** `gateCheckGoEntry`（`gate_inproc_table.go` · 现读 179 键）；本件与它同包 ⇒
//	  直接遍历该 map（同 (a) 复用同包 `scriptRetireClosedSetExcludedPrefixes` 的手法）。
//	· 键(件) = 件名去末段扩展名（`filepath.Base` 减 `filepath.Ext` · 与名录/inproc 同口径）。
//	· 闭集口径 = **同包** `scriptRetireInClosedSet`（`family_script_retire.go`）—— 复用它的**排除前缀**
//	  `scriptRetireClosedSetExcludedPrefixes`（决-12 单点真源），✗ 不复制一份前缀表。
//	· 例外名册 / 退场台账 = **同包** `core/internal/contract/adopt-retire.json` 的
//	  `retire_exceptions` / `retire_ledger`（`scriptRetireTruthRel` / `scriptRetireExceptionKey`
//	  / `scriptRetireLedgerKey` · 经 `scriptRetireDoc` 读）。
//
// 覆盖面（**薄面** · 起手口径）：候选 = 闭集内的 py/sh（＝全仓 − `scriptRetireClosedSetExcludedPrefixes`
// 三前缀 · 只认 `.py`/`.sh` 后缀）。★ 逐目录的「判据类/非判据类」细分与 evals/build 的去向由 AlltoGo-② 逐族
// 收编另账；本门起手按闭集全量判、`tri-report` 只报告（先看命中、三分类收敛，再考虑升档）。
//
// ★★ 类 B（2026-10-14 补 · 缺口 `GAP-20261012-16` 的 Go 真身面）—— 「**Go 真身件未接命令树入口**」：
// 原候选闭集只认 py/sh ⇒ 「core 有完整 Go 真身却无命令树入口」这件事**结构上不在它的面内**
// （账内现读边界逐字：「该门治 py/sh 闭集件的 inproc 路由，Go 真身入口面不在其候选闭集内 ⇒ 只覆盖一半」）。
// 类 B 判决 = **从命令树 `run:` 符号沿同包顶层符号调用链可达**（＝该真身在进程内被命令面调得到）：
//
//	· 候选 = `core/cmd/zerg/*_native.go`（**件名词缀约定**：本族 Go 真身件一律 `*_native.go`，
//	  如 `family_mirror_public_native.go` / `family_publish_verify_native.go`）——薄面 · ✗ 不把该目录
//	  下全部 `.go` 收进候选（那会退化成「什么都收」，判据不再有鉴别力）。
//	· `run:` 符号 = 命令树条目的 `run: <符号>`（`main.go` 的 commands 表）；调用链 = 件 → 它**调用形**
//	  引用的同包顶层符号 → 定义该符号的件（BFS 到不动点 · 只走**非测试**件）。
//	· 判词照旧写「**未接 inproc 标准路由**」（✗ 禁「打不着/不可达」）；✗ 不为类 B 开第二个放行口 ——
//	  例外名册/退场台账的真源闭集**只收 py/sh**（`scriptRetireInClosedSet`）⇒ Go 件本就登不进去。
//
// 旗标（家族同型 · 先现读 `check_lang_freeze.go` 对齐）：`--check`（**默认** · 只读）｜`--self-test`｜
// `--root <根>`。三档互斥（`--check` / `--self-test`）。
//
// 退码：0 全绿 ／ 1 门禁红（A 命中）／ 2 不给结论（仓根取不到 · 真源读不到/解不动 · 扫到 0 件 · 用法错）。
//
// 出厂声明 (a) 默认根定位：无 `__file__` 语境 ⇒ 不给 `--root` 时取 `repoRoot()`（`ZERG_REPO` →
// 可执行文件目录上溯 → cwd 上溯，锚 = `core/internal/version/version.go`）。对拍一律显式传 `--root`。
//
// 出厂声明 (b) 自指检查：本件是 `.go` ⇒ 落不进本门自己的扫描面（只收 `.py`/`.sh`）；真源是 `.json` ⇒ 同不在内。
// 本件 read-only（默认档不写任何件）。判据臂键表是**内存 map** ⇒ ✗ 无任何路径/文件字面量落到别的门的扫描面。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	// ccoJudgeKey —— 判据键名（与件名同型 · 家族口径）。
	ccoJudgeKey = "check-cli-ownership"
	// ccoProg —— 件名（报话前缀）。
	ccoProg = "check-cli-ownership"

	// ── 类 B（Go 真身件）取数面的常量 ──
	// ccoGoDirRel —— Go 真身件所在目录（仓内相对 · ✗ 不写死任何件名）。
	ccoGoDirRel = "core/cmd/zerg"
	// ccoSuffixNative —— Go 真身件的**件名词缀**（本族约定：`family_*_native.go`）。
	ccoSuffixNative = "_native.go"
	// ccoSuffixTest —— 测试件后缀（✗ 不进判据面：测试里被调到 ≠ 命令面接上了）。
	ccoSuffixTest = "_test.go"
	// ccoSuffixGo —— Go 件后缀。
	ccoSuffixGo = ".go"
)

// ccoKeyOf —— 键 = 件名去末段扩展名（`filepath.Base` 减 `filepath.Ext`）。
//
// 与名录/inproc 同口径：`scripts/gates/check-foo.py` ⇒ `check-foo` · `scripts/build/x.y.sh` ⇒ `x.y`。
func ccoKeyOf(rel string) string {
	base := filepath.Base(filepath.ToSlash(rel))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// ccoSkipDir —— 目录剪枝：`.git` / `__pycache__` 一律跳过；闭集排除前缀（**同一真源**
// `scriptRetireClosedSetExcludedPrefixes`）下的目录整体跳过（省时 · 与逐件闭集判定同口径）。
func ccoSkipDir(rel string) bool {
	base := rel
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		base = rel[i+1:]
	}
	if base == ".git" || base == "__pycache__" {
		return true
	}
	slash := rel + "/"
	for _, p := range scriptRetireClosedSetExcludedPrefixes {
		if strings.HasPrefix(slash, p) {
			return true
		}
	}
	return false
}

// ccoScan —— 现读闭集内 py/sh 件（与 `zerg script retire` 同一 `scriptRetireInClosedSet` 判定）。
//
// 排序按 path 升序（确定性；✗ 无 map 迭代进输出）。
func ccoScan(root string) []string {
	var out []string
	root = filepath.Clean(root)
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil // 读不到的子树跳过（不使整门炸）
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if ccoSkipDir(rel) {
				return fs.SkipDir
			}
			return nil
		}
		if !scriptRetireInClosedSet(rel) {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out
}

// ccoInprocTable —— `gateCheckGoEntry` 的一份**引用**（经 init() 抄入 · 同 `check_doc_freshness.go`
// 的 `cdfInprocFaceTable` 手法）。
//
// ★ 为什么不在变量初始化期直引用 `gateCheckGoEntry`：本件入口 `checkCliOwnershipGateMain` **在**该表里
// （键 `check-cli-ownership`）⇒ 若在**变量初始化期**直引用该表，即构成 initialization cycle
// （表 → checkCliOwnershipGateMain → ccoCheck → ccoInprocKeys → 表）。init 函数体不进「变量初始化依赖图」
// ⇒ 环即断；运行时该表早已建好 ⇒ 读数与直引用逐字同。
var ccoInprocTable map[string]func(stdout, stderr io.Writer, argv []string) int

func init() { ccoInprocTable = gateCheckGoEntry }

// ccoInprocKeys —— 现读判据臂键表（同包 `gateCheckGoEntry` 的键集 · 经 `ccoInprocTable` 引用）。
//
// ★ 这是本门**唯一**读 inproc 的口子；`ccoJudge` 收它当**入参**（✗ 不在判据体里硬编码读全局）——
// 这样负控可**喂空表**（哑化 inproc 读取支）而不改真表。
func ccoInprocKeys() map[string]bool {
	m := make(map[string]bool, len(ccoInprocTable))
	for k := range ccoInprocTable {
		m[k] = true
	}
	return m
}

// ccoLoadRetire —— 读退场真源的两块（例外名册 · 退场台账），**按键**索引。
//
// 返回 (例外名册, 退场台账, ok)：ok=false ⇒ 真源读不到 / 解不动 ⇒ 调用方**不给结论**（rc=2）。
// ★ 与 (a) `clfLoadRetire` 的差别：本门 B/C 放行按**键**判（「键(件) ∈ 名册」）⇒ 这里把 object 转成键。
func ccoLoadRetire(root string) (map[string]scriptRetireExc, map[string]bool, bool) {
	exc := map[string]scriptRetireExc{}
	led := map[string]bool{}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(scriptRetireTruthRel)))
	if err != nil {
		return exc, led, false
	}
	var doc scriptRetireDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return exc, led, false
	}
	for _, e := range doc.RetireExceptions {
		if o := strings.TrimSpace(e.Object); o != "" {
			exc[ccoKeyOf(o)] = e
		}
	}
	for _, e := range doc.RetireLedger {
		if o := strings.TrimSpace(e.Object); o != "" {
			led[ccoKeyOf(o)] = true
		}
	}
	return exc, led, true
}

// ccoResult —— 一次判定的读数（纯函数出口 · 负控可断言）。
type ccoResult struct {
	Rc       int      // 0 绿 / 1 红 / 2 无候选
	Total    int      // 候选件数
	Green    int      // 键 ∈ inproc（标准路由）
	Cleared  int      // 键 ∈ 台账（销账放行）
	Excused  []string // 例外放行（逐条理由）
	Red      []string // 未接标准路由（逐条点名 · 类 A ＋ 类 B 合并）
	GoScanOK bool     // 类 B 取数面到位（`core/cmd/zerg/` 目录读得到）
	GoTotal  int      // 类 B 候选件数（`*_native.go`）
	GoWired  int      // 类 B 已接命令树 `run:` 链的件数
}

// ccoJudge —— 三态判定（**纯函数** · 只看传入的真源 ⇒ 负控可喂空表 / 合成件）。
//
// inproc = 判据臂键表（可由 `ccoInprocKeys()` 现读，或夹具合成）；led = 退场台账键集；
// exc = 例外名册（键 → 条目）。rc=1 ⟺ A 命中（任一候选件未接标准路由）。
func ccoJudge(root string, inproc map[string]bool, led map[string]bool, exc map[string]scriptRetireExc) ccoResult {
	cands := ccoScan(root)
	res := ccoResult{Total: len(cands)}
	for _, rel := range cands {
		k := ccoKeyOf(rel)
		switch {
		case inproc[k]:
			res.Green++
		case led[k]:
			res.Cleared++
		default:
			if e, ok := exc[k]; ok {
				res.Excused = append(res.Excused,
					fmt.Sprintf("例外放行 %s（键 %s ∈ 名册 · %s · %s）", rel, k, e.Category, e.Reason))
			} else {
				res.Red = append(res.Red,
					fmt.Sprintf("%s 未接 inproc 标准路由（键 %s ∉ 判据臂键表 ∧ ∉ 名册 ∧ ∉ 台账）", rel, k))
			}
		}
	}
	// ── 类 B（Go 真身件 · `core/cmd/zerg/*_native.go`）：判决 = 从命令树 run: 符号沿同包顶层符号调用链可达 ──
	if files, natives, ok := ccoScanGo(root); ok {
		res.GoScanOK = true
		res.GoTotal = len(natives)
		wired, goRed := ccoJudgeGo(files, natives)
		res.GoWired = wired
		res.Red = append(res.Red, goRed...)
	}
	if len(res.Red) > 0 {
		res.Rc = 1
	}
	return res
}

// ── 类 B：Go 真身件（`core/cmd/zerg/*_native.go`）──────────────────────────────────
//
// 判决口径（**逐字** · ✗ 谢绝语义失真）：真身件「接上了」= **从命令树 `run:` 符号沿同包顶层符号
// 调用链可达** —— 即命令面真调得到它（本族的调用面全是进程内调用，没有 py/sh 那种「键入 inproc
// 分发表」的第二形态）。判词仍写「未接 inproc 标准路由」（✗ 禁「打不着/不可达」）。
//
// ★ 为什么按**件名后缀**收候选（而不是「该目录下所有 `.go`」）：本族 Go 真身件的成文约定就是
// `*_native.go`（6 件 · 见件头），逐件都由命令树某条 `run:` 链承接；把闭集放宽到「所有 .go」
// 会让 700+ 件全成候选、判据当场失去鉴别力（＝把判据放宽成「什么都收」）。
var (
	// ccoGoFuncRE —— 顶层 func 声明（含方法：`func (r T) Name(`）⇒ 取符号名。
	ccoGoFuncRE = regexp.MustCompile(`(?m)^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	// ccoGoRunRE —— 命令树条目的 `run: <符号>`。
	ccoGoRunRE = regexp.MustCompile(`\brun:\s*([A-Za-z_][A-Za-z0-9_]*)`)
	// ccoGoCallRE —— 调用形引用（`Name(`）⇒ 件级调用面。
	ccoGoCallRE = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

// ccoScanGo —— 现读类 B 取数面：`<根>/core/cmd/zerg` 下的**非测试** `.go` 件（件名 → 正文），
// 以及其中 `*_native.go` 的候选件清单（仓内相对路径 · 升序）。目录读不到 ⇒ ok=false。
func ccoScanGo(root string) (map[string]string, []string, bool) {
	dir := filepath.Join(root, filepath.FromSlash(ccoGoDirRel))
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, false
	}
	files := map[string]string{}
	var natives []string
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ccoSuffixGo) || strings.HasSuffix(name, ccoSuffixTest) {
			continue
		}
		b, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			continue
		}
		files[name] = string(b)
		if strings.HasSuffix(name, ccoSuffixNative) {
			natives = append(natives, ccoGoDirRel+"/"+name)
		}
	}
	sort.Strings(natives)
	return files, natives, true
}

// ccoGoWired —— 从命令树 `run:` 符号出发、沿「件 → 它调用形引用的同包顶层符号 → 定义该符号的件」
// 做件级 BFS（到不动点）⇒ 可达件集。纯函数（只看传入的件表 ⇒ 负控可喂合成件）。
func ccoGoWired(files map[string]string) map[string]bool {
	symOf := map[string]string{}
	for name, src := range files {
		for _, m := range ccoGoFuncRE.FindAllStringSubmatch(src, -1) {
			if _, dup := symOf[m[1]]; !dup {
				symOf[m[1]] = name
			}
		}
	}
	callOf := map[string]map[string]bool{}
	for name, src := range files {
		set := map[string]bool{}
		for _, m := range ccoGoCallRE.FindAllStringSubmatch(src, -1) {
			if _, ok := symOf[m[1]]; ok {
				set[m[1]] = true
			}
		}
		callOf[name] = set
	}
	reach := map[string]bool{}
	var q []string
	for _, src := range files {
		for _, m := range ccoGoRunRE.FindAllStringSubmatch(src, -1) {
			if f, ok := symOf[m[1]]; ok && !reach[f] {
				reach[f] = true
				q = append(q, f)
			}
		}
	}
	for len(q) > 0 {
		f := q[0]
		q = q[1:]
		for sym := range callOf[f] {
			if g := symOf[sym]; g != "" && !reach[g] {
				reach[g] = true
				q = append(q, g)
			}
		}
	}
	return reach
}

// ccoJudgeGo —— 类 B 判决（**纯函数**）：返回 (已接件数, 未接逐条点名)。候选清单与件表都由调用方给
// ⇒ 负控可哑化「命令树 `run:` 行」造两态。
func ccoJudgeGo(files map[string]string, natives []string) (int, []string) {
	reach := ccoGoWired(files)
	wired := 0
	var red []string
	for _, rel := range natives {
		if reach[filepath.Base(rel)] {
			wired++
			continue
		}
		red = append(red, fmt.Sprintf(
			"%s 未接 inproc 标准路由（Go 真身件 · 从命令树 `run:` 符号沿同包顶层符号调用链**未引用**它）", rel))
	}
	return wired, red
}

// ccoHas —— 逐字命中一行（自检断言用）。
func ccoHas(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

// ccoCheck —— 默认 / `--check`：只读判定（现读判据臂键表 ＋ 真源 ⇒ 三态）。
func ccoCheck(root string, stdout, stderr io.Writer) int {
	exc, led, ok := ccoLoadRetire(root)
	if !ok {
		fmt.Fprintf(stderr, "%s: 读不到退场真源 %s（解不动/缺件）⇒ 不给结论（rc=2）\n",
			ccoProg, scriptRetireTruthRel)
		return 2
	}
	cands := ccoScan(root)
	if len(cands) == 0 {
		fmt.Fprintf(stderr, "%s: 扫到 0 件候选（闭集 py/sh · 根 %s）⇒ **空 ≠ 缺**，不给结论（rc=2）\n",
			ccoProg, root)
		return 2
	}
	inproc := ccoInprocKeys()
	res := ccoJudge(root, inproc, led, exc)
	if res.GoScanOK && res.GoTotal == 0 {
		fmt.Fprintf(stderr, "%s: 类 B 扫到 0 件候选（%s/*_native.go）⇒ **空 ≠ 缺**，不给结论（rc=2）\n",
			ccoProg, ccoGoDirRel)
		return 2
	}
	fmt.Fprintf(stdout, "── CLI 归属门（存量件是否走 inproc 标准路由）── %s\n", ccoJudgeKey)
	fmt.Fprintf(stdout, "判据臂键表 %d 键｜候选 %d 件（闭集 py/sh）｜绿 %d · 销账放行 %d · 例外放行 %d\n",
		len(inproc), res.Total, res.Green, res.Cleared, len(res.Excused))
	if res.GoScanOK {
		fmt.Fprintf(stdout, "类 B Go 真身件 %d 件（%s/*_native.go）｜接命令树 run 链 %d · 未接 %d\n",
			res.GoTotal, ccoGoDirRel, res.GoWired, res.GoTotal-res.GoWired)
	} else {
		fmt.Fprintf(stdout, "类 B Go 真身件：%s/ 目录读不到 ⇒ 本类跳过（✗ 不当绿）\n", ccoGoDirRel)
	}
	fmt.Fprintf(stdout, "未接合计 %d 件（明细见上 · 逐条判词只点名真未接的件）\n", len(res.Red))
	for _, l := range res.Red {
		fmt.Fprintf(stdout, "  %s\n", l)
	}
	for _, l := range res.Excused {
		fmt.Fprintf(stdout, "  %s\n", l)
	}
	if res.Rc == 1 {
		fmt.Fprintln(stdout, "⇒ 门禁红（rc=1）：有件未接 inproc 标准路由（键须入判据臂键表，或入例外名册/走退场）")
		return 1
	}
	fmt.Fprintln(stdout, "⇒ 全绿（rc=0）：候选件皆走 inproc 标准路由或已放行")
	return 0
}

// ccoSelfTest —— 合成自检（仓外临时树 · 三态 + 负控 + 缺真源）：全过 ⇒ rc=0。
//
// ★ 负控（**喂空表 = 哑化 inproc 读取支**）：同一候选「补键入表 ⇒ 绿」对「喂空表 ⇒ 红」**成对**，
// 若判据体不再真读传入的 inproc 表 ⇒ 该格必失败（`report` 打 `件:行`）。✗ 不改真表来造红。
func ccoSelfTest(stdout, stderr io.Writer) int {
	tmp, err := os.MkdirTemp("", "cco-selftest-")
	if err != nil {
		tmp = os.TempDir()
	} else {
		defer os.RemoveAll(tmp)
	}
	writeFile := func(root, rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
	}
	ok, bad := 0, 0
	report := func(name string, cond bool, detail string) {
		if cond {
			fmt.Fprintf(stdout, "   ✓ %s\n", name)
			ok++
			return
		}
		fmt.Fprintf(stdout, "   ✗ %s：%s\n", name, detail)
		bad++
	}

	// ── 夹具（仓外临时树）：一件候选（thing.py）＋ 一件排除面（vendor/skip.py）＋ 真源 ──
	writeFile(tmp, "thing.py", "print(1)\n")
	writeFile(tmp, "vendor/skip.py", "print(9)\n")
	writeFile(tmp, scriptRetireTruthRel, `{"retire_ledger":[],"retire_exceptions":[]}`)
	wantRed := "thing.py 未接 inproc 标准路由（键 thing ∉ 判据臂键表 ∧ ∉ 名册 ∧ ∉ 台账）"

	// ① 负控 · 喂空表（哑化 inproc 读取支）⇒ 必红 ＋ 逐字判词 ＋ 候选计数（排除前缀不计）
	resEmpty := ccoJudge(tmp, map[string]bool{}, map[string]bool{}, map[string]scriptRetireExc{})
	report("甲 键∉表/名册/台账 ⇒ 必红（喂空表）", resEmpty.Rc == 1 && ccoHas(resEmpty.Red, wantRed),
		fmt.Sprintf("期望 rc=1 且点名 %q，得 rc=%d red=%v", wantRed, resEmpty.Rc, resEmpty.Red))
	report("排除前缀（vendor/）不计入候选", resEmpty.Total == 1,
		fmt.Sprintf("候选应 1 件，得 %d", resEmpty.Total))

	// ② 补键入表 ⇒ 必绿
	resIn := ccoJudge(tmp, map[string]bool{"thing": true}, map[string]bool{}, map[string]scriptRetireExc{})
	report("乙 补键入表 ⇒ 必绿", resIn.Rc == 0 && resIn.Green == 1,
		fmt.Sprintf("期望 rc=0 且绿 1，得 rc=%d green=%d", resIn.Rc, resIn.Green))

	// ③ 成对（✗ 禁恒绿）：喂空表红(1) ≠ 补表绿(0)
	report("成对断言：空表红(1) ≠ 补表绿(0)", resEmpty.Rc != resIn.Rc,
		fmt.Sprintf("两格同码 rc=%d/%d ⇒ 无鉴别力", resEmpty.Rc, resIn.Rc))

	// ④ 名册放行（B）+ 逐条理由
	resExc := ccoJudge(tmp, map[string]bool{}, map[string]bool{},
		map[string]scriptRetireExc{"thing": {Object: "thing.py", Category: "合成类", Reason: "合成理由"}})
	report("丙 名册放行 ⇒ 绿 + 理由",
		resExc.Rc == 0 && ccoHas(resExc.Excused, "例外放行 thing.py（键 thing ∈ 名册 · 合成类 · 合成理由）"),
		fmt.Sprintf("期望放行 + 理由，得 rc=%d excused=%v", resExc.Rc, resExc.Excused))

	// ⑤ 台账销账放行（C）
	resLed := ccoJudge(tmp, map[string]bool{}, map[string]bool{"thing": true}, map[string]scriptRetireExc{})
	report("丁 台账销账 ⇒ 绿", resLed.Rc == 0 && resLed.Cleared == 1,
		fmt.Sprintf("期望 rc=0 销账 1，得 rc=%d cleared=%d", resLed.Rc, resLed.Cleared))

	// ⑥ 缺真源 ⇒ rc=2（不给结论）
	miss := tmp + "-miss"
	writeFile(miss, "thing.py", "print(1)\n")
	var o6, e6 strings.Builder
	rc6 := ccoCheck(miss, &o6, &e6)
	report("戊 缺退场真源 ⇒ 不给结论（rc=2）",
		rc6 == 2 && strings.Contains(e6.String(), "读不到退场真源"),
		fmt.Sprintf("期望 rc=2 且报「读不到退场真源」，得 rc=%d err=%q", rc6, e6.String()))
	_ = os.RemoveAll(miss)

	// ── ⑧⑨⑩ 类 B（Go 真身件）：仓外临时树里合成一份「命令树 + 两个真身件」，再**哑化命令树**造第二态 ──
	tmpGo, gerr := os.MkdirTemp("", "cco-selftest-go-")
	if gerr != nil {
		tmpGo = os.TempDir()
	} else {
		defer os.RemoveAll(tmpGo)
	}
	writeFile(tmpGo, "core/cmd/zerg/main.go",
		"package main\n\nvar commands = []*command{\n\t{path: []string{\"foo\"}, run: cmdFoo},\n}\n")
	writeFile(tmpGo, "core/cmd/zerg/family_a_native.go",
		"package main\n\nfunc cmdFoo(stdout, stderr io.Writer, argv []string) int { return aNative(stdout, stderr, argv) }\n"+
			"\nfunc aNative(stdout, stderr io.Writer, argv []string) int { return 0 }\n")
	writeFile(tmpGo, "core/cmd/zerg/family_b_native.go",
		"package main\n\nfunc bNative(stdout, stderr io.Writer, argv []string) int { return 0 }\n")
	filesGo, nativesGo, okGo := ccoScanGo(tmpGo)
	wiredGo, redGo := ccoJudgeGo(filesGo, nativesGo)
	report("庚' 类 B 候选面 = `*_native.go` 两件（✗ 不吃 main.go）", okGo && len(nativesGo) == 2,
		fmt.Sprintf("期望候选 2 件，得 ok=%v 候选=%d", okGo, len(nativesGo)))
	report("辛' 类 B：被命令树 `run:` 链调到的真身件 ⇒ 绿（不计红）", wiredGo == 1 && len(redGo) == 1,
		fmt.Sprintf("期望 已接 1 / 未接 1，得 已接=%d 未接=%v", wiredGo, redGo))
	wantGoRed := "core/cmd/zerg/family_b_native.go 未接 inproc 标准路由（Go 真身件 · 从命令树 `run:` 符号沿同包顶层符号调用链**未引用**它）"
	report("壬' 类 B：无人调用的真身件 ⇒ 必红 ＋ 逐字点名", ccoHas(redGo, wantGoRed),
		fmt.Sprintf("期望点名 %q，得 %v", wantGoRed, redGo))
	writeFile(tmpGo, "core/cmd/zerg/main.go", "package main\n\nvar commands = []*command{}\n")
	filesGo2, nativesGo2, _ := ccoScanGo(tmpGo)
	wiredGo2, redGo2 := ccoJudgeGo(filesGo2, nativesGo2)
	report("癸' 类 B 哑化（拿掉命令树那行 `run:`）⇒ 两件都红（证判据真读命令树）",
		wiredGo2 == 0 && len(redGo2) == 2,
		fmt.Sprintf("期望 已接 0 / 未接 2，得 已接=%d 未接=%d", wiredGo2, len(redGo2)))
	report("子' 类 B 成对：命令树在(绿1红1) ≠ 命令树哑化(绿0红2)",
		wiredGo != wiredGo2 && len(redGo) != len(redGo2),
		fmt.Sprintf("两态同读数 已接=%d/%d 未接=%d/%d ⇒ 无鉴别力", wiredGo, wiredGo2, len(redGo), len(redGo2)))

	// ⑦ 现读真表非空（防呆 · 证 ccoInprocKeys 真取到同包表）
	report("己 现读判据臂键表非空", len(ccoInprocKeys()) > 0,
		"ccoInprocKeys() 返空 ⇒ 同包表读不到")

	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时树 · 未碰真仓）\n", ok, bad)
	if bad == 0 {
		return 0
	}
	return 2
}

// checkCliOwnershipGateMain —— 入口（argv 与家族同形 · 不含程序名）。
func checkCliOwnershipGateMain(stdout, stderr io.Writer, argv []string) int {
	rootArg := ""
	rootSet := false
	doCheck, selfTest := false, false

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
				fmt.Fprintf(stderr, "%s: `--root` 缺值 ⇒ 不给结论（rc=2）\n", ccoProg)
				return 2
			}
			rootArg, rootSet = v, true
		case a == "--check":
			doCheck = true
		case a == "--self-test":
			selfTest = true
		case a == "-h" || a == "--help":
			fmt.Fprintf(stdout, "用法：%s [--check | --self-test] [--root <根>]\n", ccoProg)
			return 0
		default:
			fmt.Fprintf(stderr, "%s: 未知旗标 %s ⇒ 不给结论（rc=2）\n", ccoProg, a)
			return 2
		}
	}
	if doCheck && selfTest {
		fmt.Fprintf(stderr, "%s: `--check` / `--self-test` 互斥 ⇒ 不给结论（rc=2）\n", ccoProg)
		return 2
	}
	if selfTest {
		return ccoSelfTest(stdout, stderr)
	}
	root := rootArg
	if !rootSet {
		root = repoRoot()
	}
	if strings.TrimSpace(root) == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根（`--root` 未给且 `repoRoot()` 取不到）⇒ 不给结论（rc=2）\n", ccoProg)
		return 2
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return ccoCheck(root, stdout, stderr)
}
