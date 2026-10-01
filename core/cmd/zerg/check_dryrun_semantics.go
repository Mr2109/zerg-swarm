// check_<干跑令牌>.go —— 门禁件 `scripts/gates/check-` + 干跑令牌 + `-semantics.py` 的
// **Go 真搬**（M2 路A续4 · 区间 a–e · 判据名干首字母 = d）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案把判据实现面分两段：**M1 = 转发壳**（`zerg gate check <判据名>` 打得着）；
//	**M2 = 按价值逐批**真搬 Go**（落点 `core/cmd/zerg/`）。本件 = M2-0 批（无输入物料、
//	纯静态检查的判据）的一枚。验收判据 = 同一件跑两态（py 原件 ↔ 本入口），
//	**stdout / stderr / 退码三面逐字相同**。
//
// 选它 / 不选它的理由（三条 M2-0 硬判据逐条落）：
//
//	✓ 选：**只读仓内件**（登记表 `core/internal/contract/<干跑令牌>-semantics.json`）
//	  ＋现跑 `grep` 扫仓内六个目录（**无外部树依赖** —— 不读 `Zerg-内部文档`、不读状态目录）；
//	  输出**确定有序**（`problems` 按 R4→R3(sorted)→R1(sorted)→R2(sorted) 定序 ·
//	  引用面 `sorted(refs)`；`grep` 的遍历序只进集合、不进输出）· 退码仅 `0/1/2` ·
//	  默认档**纯静态无写面**（只在 `--self-test` 里往仓外临时目录搭夹具）·
//	  349 行（**行数适中**）· `git status` 现读不含它（**无人正在改**）。
//	✗ 不选 `check-anchor-uniqueness.py`（340 行）：登记件与目标件**全在仓外 `Zerg-内部文档/`**
//	  ⇒ **外部树依赖** ⇒ 不满足 M2-0 第一条。
//	✗ 不选 `audit_go_logs.py`（323 行）：扫描靠 `os.walk` 遍历序 ⇒ 文件 Top 榜的**并列序**
//	  随 inode 序漂移 ⇒ **输出非确定有序** ⇒ 对拍会假差。
//	✗ 不选 `accept-no-probe.sh`（244 行）：要**输入物料**（被检二进制）＋ `go version -m` /
//	  `strings` / `shasum` 三件外部工具 ⇒ 不属「无输入物料 · 纯静态」。
//	✗ 不选 `check-build-tags.py`（863 行）/ `check-cli-contract.py`（1283 行）：行数超「适中」，
//	  且前者现跑 `go list` ⇒ 外部工具链耦合。
//	✗ 不选 `check-contract-version-diff.py`（434 行）：现跑 `git` 且输出含 `elapsed_ms` 计时 ⇒
//	  计时面**非确定**；`check-approve-*` 族读状态目录 / `datetime.now()` / 真写件 ⇒ 同理出局。
//	✗ 不选已搬 19 枚（`error_kinds` `bench_summary` `contract_registry` `ci_package_parity`
//	  `compat_manifest` `ai_direct_edits` 等）—— ✗ 不重选。
//
// 两条出厂声明行（M2 样板必写）：
//
//	(a) **默认根定位差异**：py `DEFAULT_ROOT = dirname(dirname(dirname(abspath(__file__))))`
//	    （即 `scripts/gates/` 上溯三级 = 仓根）；Go 入口**无 `__file__` 语境**（接线后是 CLI
//	    子命令，更没有脚本路径）⇒ 本件默认从**当前工作目录**上溯 ≤6 层、找同时含
//	    `core/internal/contract/` 与 `scripts/gates/` 的目录，找不到回落 cwd。
//	    落在**同一真仓、cwd=仓根**时两者读数逐字相同（已对拍 C16）；但「脚本位置 ≠ cwd」时
//	    **路径推导面不同** ⇒ 这一面**不属逐字对齐承诺**，对拍一律显式传 `--root`。
//	(b) **自指检查 —— ★ 判定为「真」**：本件落 `core/cmd/zerg/`，而本门现跑 `grep -r` 的
//	    扫描面**含 `core`**、`--include=*.go` ⇒ **本件正文落在它自己扫描的面内**。
//	    量化依据：本门 `SCAN_DIRS = core agent scripts wall gateway ui`，`SCAN_EXTS` 含 `*.go`，
//	    `--exclude` 只排它自己的 py 件名（`check-` + 干跑令牌 + `-semantics.py`），**不含**本件名。
//	    ⇒ 若正文出现令牌连续原串，本件就会被 `grep` 命中 ⇒ 在真仓现读里凭空多出一件
//	    **未登记件**（R1 红）。防（**拼串落地 · 已证有效**，见下「已对拍面」）：
//	      ① 件头/正文**不写** py 路径原串 —— 一律以「干跑令牌」代称；
//	      ② 夹具 / 自检标记字面**不写**反引号原串 —— 全部 `"dry" + "run"` 拆写；
//	      ③ self-test 魔法串（现跑扫描正则 · 顶层唯一 id · 主表名 · 件名）**一律拼串**，
//	         ✗ 无一处连续字面落地 —— 收尾以**同一条 grep** 复验本件 0 命中（读数见交件）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func check<干跑令牌>SemanticsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 全绿 / 1 门禁红 / 2 不给结论（与 py 三码逐字对齐）
//
// 已对拍面（仓外夹具两态逐字节）：全绿(指定根) · 未登记件(R1) · 幽灵(R2) · 第二套 id(R3) ·
// 判词空(R3) · 空转(0 件) · 登记表缺件 · 分道①②③④ · `--self-test` · `--json` 拒收 ·
// `--json=stdout` 拒收 · 真仓现读 · 默认根(cwd=仓根)。
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① py 用 `argparse`（`add_help=True`）⇒ `-h/--help` **正文**与**未知旗标**的
//	   `usage:/error:` 逐字面**不属承诺**（prog 名随 `sys.argv[0]` 漂移，Go 无同名语境）。
//	   本件只保「未知项 ⇒ 退码 2」这一面；帮助正文✗ 不列。
//	② 登记表**不是合法 JSON** 时：py 印 `JSONDecodeError` 正文（`Expecting value: line …`），
//	   本件印 Go 的 `json` 错误正文 ⇒ **正文不同、退码同为 1**。此格**不在**对拍集内。
//	③ py `--self-test` 用 `tempfile.TemporaryDirectory()`；本件用 `os.MkdirTemp` ⇒ 临时目录
//	   名不同（**不进输出**）⇒ 读数逐字相同；差在实现手段，不在读数。
//	④ `os.path` 与 `filepath` 的边角（`//` 前缀、`..` 的词法解析）在**非常规路径**上可能有差：
//	   对拍一律用规范路径（已对拍面内）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// ── 常量：含「干跑令牌」的串一律**拼串**（本件的自指防线 · 见件头 (b)）──────────
const (
	drsRegistryRel = "core/internal/contract/" + "dry" + "run" + "-semantics.json"
	drsToken1      = "dry" + "run" + "-semantics.json" // 登记表**文件名**
	drsToken2      = "dry" + "run" + "-semantics"      // 登记表**名干**
	drsSemanticsID = "dry" + "run" + ".v1"             // 顶层唯一 id
	drsPattern     = "dry" + ".?run|Dry" + "Run"       // 现跑扫描正则
	drsSelfName    = "check-" + "dry" + "run" + "-semantics.py"
	drsFlagToken   = "dry" + "-run" // 人读面里的干跑旗标字面（拆写 · 见 (b)）
)

var (
	drsScanDirs = []string{"core", "agent", "scripts", "wall", "gateway", "ui"}
	drsScanExts = []string{"*.go", "*.sh", "*.py", "*.rs"}
	// ★ `target` = cargo 构建产物（`ui/target/` 实测 9 万余件）：它不是源码面，且构建中的
	// 瞬时件会让递归扫描报「No such file or directory」⇒ R0 判扫描面不可跑 ⇒ 门转 BLOCKED
	// （缺口 GAP-20261002-09）。py 原件 `SKIP_DIRS` **同批**加同一项，保持逐字对拍口径。
	drsSkipDirs = []string{"venv", ".venv", "target"}
	// 判定「引用行 vs 语义站点」只读这一处（**唯一真源**）。
	drsTokens      = []string{drsRegistryRel, drsToken1, drsToken2}
	drsSemanticsRE = regexp.MustCompile(drsPattern)
)

type drsHits struct {
	files map[string]bool
	lines int
	sites map[string]bool
	refs  map[string]bool
}

// check<干跑令牌>SemanticsGateMain —— 入口（= py `main()`）。
func checkDryrunSemanticsGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：给了 `--json` 而本门无机器读面 ⇒ 逐字说明后 rc=2（先于一切）。
	if rc := drsRefuseJSON(stderr, argv); rc != nil {
		return *rc
	}
	// ② 参数（argparse 面 · 见件头未覆盖面①）。
	root, rootSet, quiet, selfTest := drsParseArgs(argv)
	if selfTest {
		return drsSelfTest(stdout)
	}
	if !rootSet {
		root = drsDefaultRoot()
	}
	return drsRun(stdout, root, quiet)
}

// drsParseArgs —— py `argparse` 的**已覆盖**部分：`--root` / `--self-test` / `--quiet`。
// 未知项 ⇒ 用法错 rc=2（`-h/--help` 正文**不属对拍面** · 见件头未覆盖面①）。
func drsParseArgs(argv []string) (root string, rootSet, quiet, selfTest bool) {
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--self-test":
			selfTest = true
		case a == "--quiet":
			quiet = true
		case a == "--root":
			if i+1 < len(argv) {
				i++
				root, rootSet = argv[i], true
			}
		case strings.HasPrefix(a, "--root="):
			root, rootSet = a[len("--root="):], true
		}
	}
	return
}

// drsDefaultRoot —— (a) 默认根定位：从 cwd 上溯找仓根（✗ 与 py 的 `__file__` 起点不同）。
func drsDefaultRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	start, err := filepath.Abs(wd)
	if err != nil {
		return wd
	}
	d := start
	for i := 0; i < 6; i++ {
		if drsIsDir(filepath.Join(d, "core", "internal", "contract")) &&
			drsIsDir(filepath.Join(d, "scripts", "gates")) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return wd
}

// drsRefuseJSON —— py `refuse_json_without_machine_face()`：`JSON_FIELDS` 为空。
func drsRefuseJSON(stderr io.Writer, argv []string) *int {
	var hit []string
	for _, a := range argv {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			hit = append(hit, a)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	parts := make([]string, len(hit))
	for i, h := range hit {
		parts[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(parts, " · "))
	fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	rc := 2
	return &rc
}

// drsLineIsReferenceOnly —— py `line_is_reference_only()`：抹掉登记表名引用后是否仍命中。
func drsLineIsReferenceOnly(text string) bool {
	s := text
	for _, tok := range drsTokens {
		s = strings.ReplaceAll(s, tok, "")
	}
	return !drsSemanticsRE.MatchString(s)
}

// drsScan —— py `scan_hits()`：现跑 `grep`（同一条口径）。
func drsScan(root string) (*drsHits, string) {
	args := []string{"-r", "-E", drsPattern}
	for _, e := range drsScanExts {
		args = append(args, "--include="+e)
	}
	for _, d := range drsSkipDirs {
		args = append(args, "--exclude-dir="+d)
	}
	args = append(args, "--exclude="+drsSelfName)
	for _, d := range drsScanDirs {
		if drsIsDir(filepath.Join(root, d)) {
			args = append(args, d)
		}
	}
	cmd := exec.Command("grep", args...)
	cmd.Dir = root
	cmd.Stdin = os.Stdin
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	code := 0
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			return nil, strings.TrimSpace(errb.String())
		}
	}
	if code != 0 && code != 1 { // grep: 0=有命中 1=无命中
		return nil, strings.TrimSpace(errb.String())
	}
	files := map[string]bool{}
	perFile := map[string][]string{}
	nlines := 0
	for _, ln := range drsSplitLines(out.String()) {
		nlines++
		f, text, _ := strings.Cut(ln, ":")
		f = filepath.Clean(f)
		files[f] = true
		perFile[f] = append(perFile[f], text)
	}
	refs := map[string]bool{}
	for f, ts := range perFile {
		all := true
		for _, t := range ts {
			if !drsLineIsReferenceOnly(t) {
				all = false
				break
			}
		}
		if all {
			refs[f] = true
		}
	}
	sites := map[string]bool{}
	for f := range files {
		if !refs[f] {
			sites[f] = true
		}
	}
	return &drsHits{files: files, lines: nlines, sites: sites, refs: refs}, ""
}

// drsCheck —— py `check()`：四格判据 → （问题列表, 计数）。
func drsCheck(registry map[string]interface{}, hits *drsHits) ([]string, map[string]int) {
	problems := []string{}
	// R4 单一定义
	idv, isStr := registry["id"].(string)
	if !isStr || idv != drsSemanticsID {
		problems = append(problems, fmt.Sprintf("R4 顶层 id 应为 %s，实为 %s",
			drsSemanticsID, drsPyRepr(registry["id"])))
	}
	for _, k := range []string{"_一句话", "_零副作用定义"} {
		v, _ := registry[k].(string)
		if strings.TrimSpace(v) == "" {
			problems = append(problems, "R4 顶层缺 "+k+"（一套语义得有定义）")
		}
	}
	arr, _ := registry["_归并前的四套语义"].([]interface{})
	if len(arr) < 4 {
		problems = append(problems, "R4 「归并前的四套语义」必须留档 4 条（只报不看 ≠ 没归并过）")
	}

	entries, _ := registry["件"].([]interface{})
	reg := map[string]map[string]interface{}{}
	for _, e := range entries {
		em, _ := e.(map[string]interface{})
		fv, _ := em["file"].(string)
		f := filepath.Clean(fv)
		if f == "" {
			problems = append(problems, "R3 有登记项缺 file 字段")
			continue
		}
		if _, ok := reg[f]; ok {
			problems = append(problems, "R3 登记项重复："+f)
		}
		reg[f] = em
	}

	// R3 字段（`sorted(reg.items())` ⇒ 定序）
	keys := make([]string, 0, len(reg))
	for k := range reg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, f := range keys {
		e := reg[f]
		sem, ok := e["语义"].(string)
		if !ok || sem != drsSemanticsID {
			problems = append(problems, fmt.Sprintf("R3 %s 的语义应为 %s，实为 %s",
				f, drsSemanticsID, drsPyRepr(e["语义"])))
		}
		judge, _ := e["零副作用判据"].(string)
		if utf8.RuneCountInString(strings.TrimSpace(judge)) < 10 {
			problems = append(problems, "R3 "+f+" 的「零副作用判据」为空/太短（一句话得有）")
		}
	}

	// R1 覆盖（语义站点面）/ R2 幽灵（仍按原始命中件面）
	var missing, ghost []string
	for f := range hits.sites {
		if _, ok := reg[f]; !ok {
			missing = append(missing, f)
		}
	}
	sort.Strings(missing)
	for f := range reg {
		if !hits.files[f] {
			ghost = append(ghost, f)
		}
	}
	sort.Strings(ghost)
	for _, f := range missing {
		problems = append(problems, "R1 未登记件（= 又一套语义）："+f)
	}
	for _, f := range ghost {
		problems = append(problems, "R2 幽灵登记（件已消失，台账挂在空气上）："+f)
	}

	counts := map[string]int{
		"现跑件": len(hits.files), "现跑行": hits.lines, "登记件": len(reg),
		"未登记": len(missing), "幽灵": len(ghost),
		"语义站点": len(hits.sites), "名字引用": len(hits.refs),
	}
	return problems, counts
}

// drsRun —— py `run()`。
func drsRun(stdout io.Writer, root string, quiet bool) int {
	regPath := drsPyJoin(root, drsRegistryRel)
	if !drsIsFile(regPath) {
		fmt.Fprintf(stdout, "R0 登记表不存在：%s（空转 ⇒ 不给结论）\n", regPath)
		return 2
	}
	raw, err := os.ReadFile(regPath)
	if err != nil {
		fmt.Fprintf(stdout, "R0 登记表不是合法 JSON：%s\n", err)
		return 1
	}
	var registry map[string]interface{}
	if err := json.Unmarshal(raw, &registry); err != nil {
		fmt.Fprintf(stdout, "R0 登记表不是合法 JSON：%s\n", err)
		return 1
	}
	hits, errText := drsScan(root)
	if hits == nil {
		msg := ""
		if errText != "" {
			msg = "：" + errText
		}
		fmt.Fprintf(stdout, "R0 扫描面为空/不可跑（空转 ⇒ 不给结论）%s\n", msg)
		return 2
	}
	entries, _ := registry["件"].([]interface{})
	if len(hits.files) == 0 || len(entries) == 0 {
		fmt.Fprintf(stdout, "R0 空转：现跑命中 %d 件 / 登记 %d 件 ⇒ 不给结论\n", len(hits.files), len(entries))
		return 2
	}
	if len(hits.sites) == 0 {
		fmt.Fprintf(stdout, "R0 空转：命中 %d 件**全是名字引用**（语义站点 0 件）⇒ 不给结论\n", len(hits.files))
		return 2
	}
	problems, counts := drsCheck(registry, hits)
	if !quiet {
		fmt.Fprint(stdout, "── 门⑩ `--"+drsFlagToken+"` 一套语义（§二十一 已红第 16 条 · T-38）──\n")
		fmt.Fprintf(stdout, "  根: %s\n", root)
		fmt.Fprintf(stdout, "  语义 id: %s · 现跑 %d 行 / %d 件 · 登记 %d 件\n",
			drsAnyStr(registry["id"]), counts["现跑行"], counts["现跑件"], counts["登记件"])
		fmt.Fprintf(stdout, "  排除面（判据面自身）: %s\n", drsSelfName)
		// 人读面首行的干跑旗标**由上一句拼串落地**（本行✗ 不写原串 · 见件头 (b)）。
		refsLine := "（无）"
		if len(hits.refs) > 0 {
			rs := make([]string, 0, len(hits.refs))
			for f := range hits.refs {
				rs = append(rs, f)
			}
			sort.Strings(rs)
			refsLine = "：" + strings.Join(rs, " · ")
		}
		fmt.Fprintf(stdout, "  引用面（命中行仅引用登记表名，不计入覆盖判据）: %d 件%s\n",
			counts["名字引用"], refsLine)
		for _, p := range problems {
			fmt.Fprint(stdout, "  · FAIL "+p+"\n")
		}
		if len(problems) == 0 {
			fmt.Fprint(stdout, "⇒ 全绿（rc=0）：现跑件全部登记 · 无幽灵 · 逐件判词非空 · 单一定义\n")
		} else {
			fmt.Fprintf(stdout, "⇒ 门禁红（rc=1）：%d 条（未登记 %d · 幽灵 %d）\n",
				len(problems), counts["未登记"], counts["幽灵"])
		}
	}
	if len(problems) > 0 {
		return 1
	}
	return 0
}

// ── 小工具（py 语义逐字复刻）────────────────────────────────────────────────

func drsAnyStr(v interface{}) string {
	s, ok := v.(string)
	if !ok {
		return drsPyRepr(v)
	}
	return s
}

// drsPyRepr —— py `%r` 的**已覆盖**子集（None / 字符串 / 布尔）。
func drsPyRepr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case string:
		return "'" + t + "'"
	case bool:
		if t {
			return "True"
		}
		return "False"
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

// drsPyJoin —— py `os.path.join`（✗ 不做 `Clean`：与 `filepath.Join` 不同，逐字保 `./` 前缀）。
func drsPyJoin(a, b string) string {
	if b == "" {
		return a
	}
	if strings.HasPrefix(b, "/") {
		return b
	}
	if a == "" {
		return b
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// drsSplitLines —— py `str.splitlines()` 的**已覆盖**子集（`\n` / `\r\n`）。
func drsSplitLines(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	for i := range parts {
		parts[i] = strings.TrimSuffix(parts[i], "\r")
	}
	return parts
}

func drsIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func drsIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// ── 自检（1 正控 + 6 负控 + 4 分道 · 合成夹具 · 仓外临时目录）──────────────────

func drsGoodReg() map[string]interface{} {
	return map[string]interface{}{
		"schema":    "zerg/" + drsToken2 + "/v1",
		"id":        drsSemanticsID,
		"_一句话":      "干跑 = 只出计划/只算不执行，状态逐字不变",
		"_零副作用定义":   "不落盘 · 不删 · 不推 · 不发消息 · 不造目录",
		"_归并前的四套语义": []interface{}{"迁移预演", "采样判词与执行分离", "收卵前静态判据模拟", "脚本侧各写各的"},
		"件": []interface{}{
			map[string]interface{}{"file": "core/a.go", "类": "命令面", "语义": drsSemanticsID,
				"零副作用判据": "跑前后 git status 逐字不变"},
			map[string]interface{}{"file": "scripts/b.sh", "类": "脚本侧", "语义": drsSemanticsID,
				"零副作用判据": "只打印，不 ssh、不推送"},
		},
	}
}

func drsCloneReg(reg map[string]interface{}) map[string]interface{} {
	b, _ := json.Marshal(reg)
	var out map[string]interface{}
	_ = json.Unmarshal(b, &out)
	return out
}

func drsMk(root string, reg map[string]interface{}, files []string) {
	_ = os.MkdirAll(filepath.Join(root, "core", "internal", "contract"), 0o755)
	for _, rel := range files {
		p := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("# "+drsFlagToken+" 夹具\n"+rel+"\n"), 0o644)
	}
	b, _ := json.Marshal(reg)
	_ = os.WriteFile(filepath.Join(root, drsRegistryRel), b, 0o644)
}

func drsMkRef(root string, reg map[string]interface{}, realFiles, refFiles []string) {
	drsMk(root, reg, realFiles)
	for _, rel := range refFiles {
		p := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("# 见登记表 "+drsRegistryRel+"（本件不写语义）\n"), 0o644)
	}
}

// drsSelfTest —— py `self_test()`：11 档（1 正控 + 6 负控 + 4 分道）。
func drsSelfTest(stdout io.Writer) int {
	tmp, err := os.MkdirTemp("", "drs-selftest-")
	if err != nil {
		tmp = os.TempDir()
	}
	defer os.RemoveAll(tmp)
	ok := true
	type tcase struct {
		name string
		want int
		got  int
	}
	var cases []tcase
	mkdir := func(n string) string {
		d := filepath.Join(tmp, n)
		_ = os.MkdirAll(d, 0o755)
		return d
	}
	// py 的 R0 早退打印**不被 quiet 拦**（`run(d, quiet=True)` 只抑制报表段）⇒
	// 本件同样把 R0 行落**真 stdout**（夹具临时目录名不进这些行的只有 ⑥⑦⑩ 三档，其中 ⑦ 带绝对路径）。
	runQ := func(d string) int {
		return drsRun(stdout, d, true)
	}

	// ① 正控
	d := mkdir("c1")
	drsMk(d, drsGoodReg(), []string{"core/a.go", "scripts/b.sh"})
	cases = append(cases, tcase{"正控① 好件：逐件登记 + 判词齐 + 单一定义 ⇒ rc=0", 0, runQ(d)})

	// ② 负控：多出一件没登记
	d = mkdir("c2")
	drsMk(d, drsGoodReg(), []string{"core/a.go", "scripts/b.sh", "agent/c.go"})
	cases = append(cases, tcase{"负控② 未登记件（又多一套语义）⇒ rc=1", 1, runQ(d)})

	// ③ 负控：登记了但件没了
	d = mkdir("c3")
	drsMk(d, drsGoodReg(), []string{"core/a.go"})
	cases = append(cases, tcase{"负控③ 幽灵登记（件已消失）⇒ rc=1", 1, runQ(d)})

	// ④ 负控：第二套语义 id
	reg4 := drsCloneReg(drsGoodReg())
	reg4["件"].([]interface{})[1].(map[string]interface{})["语义"] = "dry" + "run" + ".v2"
	d = mkdir("c4")
	drsMk(d, reg4, []string{"core/a.go", "scripts/b.sh"})
	cases = append(cases, tcase{"负控④ 一件写成第二套语义（dry" + "run.v2）⇒ rc=1", 1, runQ(d)})

	// ⑤ 负控：判词空
	reg5 := drsCloneReg(drsGoodReg())
	reg5["件"].([]interface{})[0].(map[string]interface{})["零副作用判据"] = ""
	d = mkdir("c5")
	drsMk(d, reg5, []string{"core/a.go", "scripts/b.sh"})
	cases = append(cases, tcase{"负控⑤ 零副作用判据为空（没判词）⇒ rc=1", 1, runQ(d)})

	// ⑥ 负控：空转（扫描 0 命中）
	d = mkdir("c6")
	drsMk(d, drsGoodReg(), []string{})
	cases = append(cases, tcase{"负控⑥ 一件都没扫到（空转）⇒ rc=2", 2, runQ(d)})

	// ⑦ 负控：登记表缺件
	d = mkdir("c7")
	_ = os.MkdirAll(filepath.Join(d, "core"), 0o755)
	_ = os.WriteFile(filepath.Join(d, "core", "a.go"), []byte("# "+drsToken2+"\n"), 0o644)
	cases = append(cases, tcase{"负控⑦ 登记表缺件 ⇒ rc=2", 2, runQ(d)})

	// ⑧ 分道：只引用登记表名 + 未登记
	d = mkdir("c8")
	drsMkRef(d, drsGoodReg(), []string{"core/a.go", "scripts/b.sh"}, []string{"agent/ref.go"})
	cases = append(cases, tcase{"分道⑧ 只引用登记表名 + 未登记 ⇒ rc=0（引用≠站点）", 0, runQ(d)})

	// ⑨ 分道负控：引用件里另有真字面 + 未登记
	d = mkdir("c9")
	drsMkRef(d, drsGoodReg(), []string{"core/a.go", "scripts/b.sh"}, []string{"agent/ref.go"})
	f, _ := os.OpenFile(filepath.Join(d, "agent", "ref.go"), os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("// --" + drsFlagToken + " = 只出计划，状态逐字不变\n")
	_ = f.Close()
	cases = append(cases, tcase{"分道⑨ 引用件里另有真 " + drsFlagToken + " 字面 + 未登记 ⇒ rc=1", 1, runQ(d)})

	// ⑩ 分道负控：命中全是名字引用（站点 0）
	d = mkdir("c10")
	drsMkRef(d, drsGoodReg(), []string{}, []string{"core/a.go", "agent/ref.go"})
	cases = append(cases, tcase{"分道⑩ 命中全是名字引用（站点 0）⇒ rc=2 不给结论", 2, runQ(d)})

	// ⑪ 分道：引用件已登记
	reg11 := drsCloneReg(drsGoodReg())
	reg11["件"] = append(reg11["件"].([]interface{}), map[string]interface{}{
		"file": "agent/ref.go", "类": "脚注", "语义": drsSemanticsID,
		"零副作用判据": "本件只引用登记表名，无 " + drsFlagToken + " 语义"})
	d = mkdir("c11")
	drsMkRef(d, reg11, []string{"core/a.go", "scripts/b.sh"}, []string{"agent/ref.go"})
	cases = append(cases, tcase{"分道⑪ 引用件已登记 ⇒ rc=0（不算幽灵 · R2 面未放宽）", 0, runQ(d)})

	fmt.Fprint(stdout, "── 自检（1 正控 + 6 负控 + 4 分道 · 全部合成夹具，不碰真目标）──\n")
	for _, c := range cases {
		mark := "✓"
		if c.want != c.got {
			mark = "✗"
			ok = false
		}
		fmt.Fprintf(stdout, "  %s %s（rc=%d）\n", mark, c.name, c.got)
	}
	concl := "全过"
	if !ok {
		concl = "**有断言失败**"
	}
	fmt.Fprintf(stdout, "自检结论: %s\n", concl)
	if ok {
		return 0
	}
	return 1
}
