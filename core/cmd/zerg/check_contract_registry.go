// check_contract_registry.go —— 门禁件 `scripts/gates/check-contract-registry.py` 的 **Go 真搬**（v2.5.14 · M2-0 第三枚）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据 · 设计 v1.0 §4.1）的**第三枚**：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（§4.2）。
//
// 选它的理由（对照 M2-0 四条硬口径）：
//
//	✓ **无外部树依赖**：只读仓内件 —— `core/internal/contract/registry.json`（判据主体）、
//	  `core/cmd/zerg/family_impact_contract.go`（R8 声明侧）、以及表里 `truth`/`gate` 列点名的
//	  仓内路径（`os.path.exists` / `isfile`）。**不碰**文档仓 / 公开树 / 生成物。
//	✓ **输出确定有序**：`reds` 按「顶层 → 逐条 entry（文件序）→ R2/R3/R4/R5/R6 → R7 → R8」固定追加，
//	  无字典遍历、无 `os.walk`/`glob` ⇒ 逐字节对拍可证。
//	✓ **退码 0/1/2 三档齐**：0 表 ↔ 件一致 / 1 有红（逐条点名）/ 2 不给结论（根解析不到 · 表读不到 · 解析失败）。
//	✓ **纯静态无写面**：main 路径一个字节都不写（`--self-test` 只在 `tempfile.mkdtemp` 里造夹具）。
//	✓ **无人正在改**：`git status --porcelain` 现读**不含** `scripts/gates/check-contract-registry.py`。
//	✓ 体量适中（291 行）· **无 subprocess / 无 git / 无 datetime.now / 无 os.walk / 无 glob**。
//
// 不选的理由（逐件·本单现读后自定）：
//
//	✗ `check-anchor-uniqueness.py`（340）/ `check-idspace-map.py`（328）/ `check-dryrun-semantics.py`（349）
//	  —— **读仓外 `Zerg-内部文档/`**（外部树依赖）⇒ 不属 M2-0「读仓内件即可」。
//	✗ `check-token-argv.py`（370）/ `check-password-argv.py`（446）/ `check-curl-timeout.py`（167）
//	  —— 扫描根靠 `os.walk` ⇒ **遍历序非确定**（py `scandir` inode 序 ≠ Go `WalkDir` 字典序）⇒ 对拍假差。
//	✗ `check-manifest-freshness.py`（226）—— 输出含 `datetime.now()` 墙钟（「距今 N.Nh」）＋ 退码 64 档
//	  ＋ 现调 `git rev-parse`（外部进程）⇒ 非「无外部依赖 + 0/1/2 + 输出确定」。
//	✗ `check-placeholder-residue.py`（250）—— 判据面靠 `git grep`（外部进程 / 仓历史）⇒ 非纯静态仓内件。
//	✗ `check-fields-face-parity.py`（213）—— 真身侧**现跑 `bin/zerg`**（制品依赖）⇒ 非纯静态 · 且本批禁碰 `bin/`。
//	✗ `check-version-sources.py`（801）/ `check-marks-coverage-parity.py`（690）—— **已列重件**。
//	✗ 8 件 `doc-*` / `check-dangling-refs.py` / `check-writer-guard.py` 等 —— **另一路在改**（`git status` 现读）。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-contract-registry.py` 原件（退场规则 §4.3：**对拍通过后才**退场，逐件显式点名）；
//	· 入口 `contractRegistryGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态：
//	  先证明「搬得逐字相同」，接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func contractRegistryGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 一致 / 1 有红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字）：全绿（真仓 & 合成仓）· 逐条负控（R1 schema / R1 note / R1 空 entries ·
// R2 缺字段 · R3 形态 · R3 重号 · R4 越域 · R5 truth 缺 · R6 无门 / ghost / 非路径非 go test · R7 / R8）·
// 取不到输入（`--root` 解析不到 / 表不在盘）· `--json` 形态（合法子集 / 表外字段 / 空字段）· `--self-test` · 真仓现读。
//
// 未覆盖面（✗ 不声称逐字对齐 · 接线前须逐面补齐或显式剔除）：
//
//	① **argparse 帮助正文 / usage 正文**：py 用 `argparse`（`-h/--help` · 未知旗标 · `--json` 缺值 ·
//	   未知位置参数 · 长选项前缀缩写）⇒ 这些格子 py 打的是 argparse 生成的 usage/error 正文（依赖
//	   `prog` 名与终端宽度）★ 本件落「未知旗标 ⇒ rc=2」但**正文自定** ⇒ 这一面**不属**逐字对齐承诺。
//	   （本件支持的长选项只有精确名 `--root/--registry/--json/--self-test/--selftest`；`=` 值形态亦支持。）
//	② **默认根定位差异**：py 从 `__file__` 所在目录（`scripts/gates`）上溯 ≤8 层找 `core/internal/version/version.go`；
//	   本件**无 `__file__` 语境**（未来接线后是 CLI 子命令）⇒ 默认从**当前工作目录**上溯 ≤8 层。
//	   落在**同一真仓、cwd=仓根**时两者读数逐字相同（已对拍 C17）；「脚本位置 ≠ cwd」时**路径推导面不同**。
//	③ **异常路径**：表存在但**解析失败**（非 JSON）时 py 抛 `json.JSONDecodeError`、正文含其 exception 字符串
//	   （如 `Expecting value: line 1 column 1 (char 0)`）；本件落「契约登记表解析失败（…）：<Go 的 error 正文>」
//	   ⇒ **同样退 2，但正文与 py 不同**。表**读不到**（权限）同理。
//	④ py `--self-test` 用 `subprocess`？—— **否**（本件 py 的 `self_test()` 进程内直调 `check()`，不起子进程）
//	   ⇒ 这一面**可**逐字对齐（已对拍 C16）。
//
//	—— ①②③ 三面均**不在** M2-0 样板对拍输入集内；接线前须逐面补齐或显式剔除（④ 已覆盖）。
//
// ★ R9（缺口 `GAP-20260927-228`/`-229` · 与 py **同批**落 · M2 双实现件）：描述 ↔ 真源**列举对拍** ——
// `truth_shape` 点名的顶层键集合 ⊇ 真源 JSON（`truth` 路径部分 · `.json`）的顶层键集合（去元数据键
// `schema`/`id`/`note`/`not_in_public_face` 与一切 `_` 起头的键）；**漏列即红**，✗ 不依赖是否自称数字。
// 非 `.json` / 非对象的真源件**不属本条面**（跳过，不判红）。
package main

import (
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

// ── 判据常量（与 py 逐条对齐 · 名字前缀 `ccr` 防包内同名）────────────────────────
var (
	ccrRegistryRel = filepath.Join("core", "internal", "contract", "registry.json")
	ccrGoDeclRel   = filepath.Join("core", "cmd", "zerg", "family_impact_contract.go")
	ccrRootAnchor  = filepath.Join("core", "internal", "version", "version.go")
)

const ccrGoDeclConst = "impactContractRegistryGateRel"
const ccrSchemaID = "zerg-contract-registry/1"
const ccrSelfRel = pathContractRegistryPy

var ccrRequiredEntryFields = []string{"id", "name", "truth", "truth_shape",
	"version_field", "gate", "change_class", "change_note"}

var ccrChangeClasses = []string{"A", "B"}

// ccrJSONFields —— py `JSON_FIELDS`（**唯一真源**：印表与拒收同读这一处）。
var ccrJSONFields = []string{"entries", "reds", "gate", "wired"}

// ccrMetaTopKeys —— R9 元数据键（与 py `META_TOP_KEYS` 逐字对齐）：跨件通用的**自述 / 出处 / 落点标记**，
// 不是「契约的键」，不许被要求点名；另加「一切 `_` 起头的键」（判据里另判）。
var ccrMetaTopKeys = map[string]bool{"schema": true, "id": true, "note": true, "not_in_public_face": true}

// ccrBacktickRE —— py `BACKTICK_RE`：R9 点名集抽取（`truth_shape` 里反引号包住的 token）。
var ccrBacktickRE = regexp.MustCompile("`([^`]+)`")

var (
	// ID_RE = ^S-[a-z]$
	ccrIDRE = regexp.MustCompile(`^S-[a-z]$`)
	// TOKEN_RE = [A-Za-z0-9_./\u4e00-\u9fff-]*[A-Za-z0-9_/-]+\.(?:py|sh|go|json|md|ts|rs|toml)
	ccrTokenRE = regexp.MustCompile(`[A-Za-z0-9_./\x{4e00}-\x{9fff}-]*[A-Za-z0-9_/-]+\.(?:py|sh|go|json|md|ts|rs|toml)`)
)

// ccrInfo —— py `info` dict（四键同形）。
type ccrInfo struct {
	Entries int
	Reds    int
	Gate    string
	Wired   bool
}

// contractRegistryGateMain —— 入口（= py `main()`）。
func contractRegistryGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① 解析实参（argparse 等价：长选项精确名 + `=` 值形态）。未知旗标 ⇒ rc=2（正文见件头未覆盖面①）。
	a, bad := ccrParseArgs(argv)
	if bad != "" {
		fmt.Fprintf(stderr, "✗ 用法错（rc=2）：%s\n", bad)
		return 2
	}

	// ② 根定位（py：`a.root or ZERG_REPO or __file__ 目录` ⇒ 本件末档换 cwd · 见未覆盖面②）。
	start := a.root
	if start == "" {
		start = os.Getenv("ZERG_REPO")
	}
	if start == "" {
		if wd, err := os.Getwd(); err == nil {
			start = wd
		} else {
			start = "."
		}
	}
	root, rootOK := ccrFindRoot(start)

	// ③ Q-176 同类：`--json` 字段名闭集校验（**先于** `--self-test` · 与 py 同序）。
	if a.json != nil {
		want := []string{}
		for _, f := range strings.Split(*a.json, ",") {
			if f != "" {
				want = append(want, f)
			}
		}
		var unknown []string
		for _, f := range want {
			if !ccrIn(f, ccrJSONFields) {
				unknown = append(unknown, f)
			}
		}
		if len(want) == 0 || len(unknown) > 0 {
			badStr := strings.Join(unknown, ",")
			if badStr == "" {
				badStr = "（空）"
			}
			fmt.Fprintf(stderr, "✗ `--json` 字段表外：%s（可选字段：%s）\n", badStr, strings.Join(ccrJSONFields, ","))
			return 2
		}
	}

	// ④ 表路径（py：`a.registry or (join(root, REGISTRY_REL) if root else REGISTRY_REL)`）
	reg := a.registry
	if reg == "" {
		if rootOK {
			reg = filepath.Join(root, ccrRegistryRel)
		} else {
			reg = ccrRegistryRel
		}
	}

	// ⑤ `--self-test`（py 在 self_test 前先印一行档头）
	if a.selfTest {
		fmt.Fprintln(stdout, "门 · 契约登记表「表 ↔ 件」一致性（W-14）· 自检档（正控真表只读 + 合成负控 · 不碰真目标）")
		return ccrSelfTest(stdout, root, rootOK)
	}

	rc, lines, info := ccrCheck(root, rootOK, reg)
	if a.json != nil {
		fmt.Fprintln(stdout, ccrJSONDump(*a.json, info))
		return rc
	}
	rootDisp := "None"
	if rootOK {
		rootDisp = root
	}
	fmt.Fprintf(stdout, "门 · 契约登记表「表 ↔ 件」一致性（W-14）· 仓根 %s\n", rootDisp)
	fmt.Fprintf(stdout, "  表：%s\n", reg)
	for _, l := range lines {
		fmt.Fprintf(stdout, "  %s\n", l)
	}
	if rc == 0 {
		fmt.Fprintln(stdout, "⇒ 表 ↔ 件一致（rc=0）")
	} else if rc == 1 {
		fmt.Fprintln(stdout, "⇒ 表 ↔ 件不一致（rc=1）：逐条见上")
	}
	return rc
}

// ccrArgs —— 解析结果。
type ccrArgs struct {
	root, registry string
	json           *string
	selfTest       bool
}

// ccrParseArgs —— argparse 等价解析（精确长选项 + `=`；未知 ⇒ 返非空说明，rc=2）。
func ccrParseArgs(argv []string) (ccrArgs, string) {
	var a ccrArgs
	for i := 0; i < len(argv); i++ {
		x := argv[i]
		switch {
		case x == "--self-test" || x == "--selftest":
			a.selfTest = true
		case x == "--root":
			if i+1 >= len(argv) {
				return a, "--root 缺值"
			}
			i++
			a.root = argv[i]
		case strings.HasPrefix(x, "--root="):
			a.root = x[len("--root="):]
		case x == "--registry":
			if i+1 >= len(argv) {
				return a, "--registry 缺值"
			}
			i++
			a.registry = argv[i]
		case strings.HasPrefix(x, "--registry="):
			a.registry = x[len("--registry="):]
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
			return a, "帮助正文（见件头未覆盖面①）"
		case strings.HasPrefix(x, "-") && x != "-":
			return a, "未知旗标 " + x
		default:
			return a, "未知位置参数 " + x
		}
	}
	return a, ""
}

// ccrIn —— py `in`（列表成员）。
func ccrIn(s string, xs []string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// ccrFindRoot —— py `find_root(start)`：上溯 ≤8 层找含 ROOT_ANCHOR 的目录；找不到返 (…, false)。
func ccrFindRoot(start string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		d = start
	}
	for i := 0; i < 8; i++ {
		if ccrIsFile(filepath.Join(d, ccrRootAnchor)) {
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

// ccrCheck —— py `check(root, registry_path)` ⇒ (rc, lines, info)。
func ccrCheck(root string, rootOK bool, registryPath string) (int, []string, ccrInfo) {
	info := ccrInfo{}
	if !rootOK {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：解析不到仓根（找 %s 失败）—— 设 ZERG_REPO=<仓根>", ccrRootAnchor)}, info
	}
	if !ccrIsFile(registryPath) {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：契约登记表读不到（%s）—— 表不在盘上 ⇒ 「表 ↔ 件」无法判", registryPath)}, info
	}
	raw, err := os.ReadFile(registryPath)
	if err != nil {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：契约登记表解析失败（%s）：%s", registryPath, err)}, info
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return 2, []string{fmt.Sprintf("⛔ 不给结论：契约登记表解析失败（%s）：%s", registryPath, err)}, info
	}

	var reds []string

	// ── R1 顶层结构 ─────────────────────────────────────────────────────────
	if ccrPyStr(doc["schema"]) != ccrSchemaID {
		reds = append(reds, fmt.Sprintf("R1 顶层 `schema` 不是 %s（实得 %s）",
			ccrPyRepr(ccrSchemaID), ccrPyRepr(doc["schema"])))
	}
	for _, k := range []string{"note", "authority"} {
		if !ccrTruthy(doc[k]) {
			reds = append(reds, fmt.Sprintf("R1 顶层缺 `%s`（登记表要写清「真源 / 导出物 / 规则」三键）", k))
		}
	}
	entries, isList := doc["entries"].([]interface{})
	if !isList || len(entries) == 0 {
		reds = append(reds, "R1 `entries` 不是非空数组（空表 = 空转 = 假覆盖）")
		return 1, reds, info
	}
	info.Entries = len(entries)

	// ── R2/R3/R4/R5/R6 逐条 ─────────────────────────────────────────────────
	seen := map[string]int{}
	for i, ei := range entries {
		e, _ := ei.(map[string]interface{})
		if e == nil {
			e = map[string]interface{}{}
		}
		eid := ccrGet(e, "id", "<无 id>")
		eidStr := ccrPyStr(eid)
		for _, f := range ccrRequiredEntryFields {
			if strings.TrimSpace(ccrPyStr(ccrGet(e, f, ""))) == "" {
				reds = append(reds, fmt.Sprintf("R2 %s 缺字段 `%s`", eidStr, f))
			}
		}
		// R3
		if !ccrIDRE.MatchString(ccrPyStr(ccrGet(e, "id", ""))) {
			reds = append(reds, fmt.Sprintf("R3 %s 的 id 形状不合 `^S-[a-z]$`", eidStr))
		}
		if prev, dup := seen[eidStr]; dup {
			reds = append(reds, fmt.Sprintf("R3 id 重复：%s（第 %d 条与第 %d 条）", eidStr, prev+1, i+1))
		}
		seen[eidStr] = i
		// R4
		cc := ccrGet(e, "change_class", nil)
		if !ccrIn(ccrPyStr(cc), ccrChangeClasses) {
			reds = append(reds, fmt.Sprintf("R4 %s 的 change_class=%s 不在闭集 %s",
				eidStr, ccrPyRepr(cc), ccrPyReprList(ccrChangeClasses)))
		}
		// R5
		tp := ccrPathPart(ccrGet(e, "truth", ""))
		if tp != "" && !ccrExists(filepath.Join(root, tp)) {
			reds = append(reds, fmt.Sprintf("R5 %s 的 truth 件不在盘：%s", eidStr, tp))
		}
		// R6
		gate := strings.TrimSpace(ccrPyStr(ccrGet(e, "gate", "")))
		toks := ccrGateTokens(gate)
		if gate == "" {
			reds = append(reds, fmt.Sprintf("R6 %s 的 `gate` 是空的 ⇒ 无门的 entry（表指向一个没人判的契约）", eidStr))
		} else if len(toks) > 0 {
			for _, t := range toks {
				if !ccrIsFile(filepath.Join(root, t)) {
					reds = append(reds, fmt.Sprintf("R6 %s 的 gate 件不在盘：%s", eidStr, t))
				}
			}
		} else if !strings.Contains(gate, "go test") {
			reds = append(reds, fmt.Sprintf("R6 %s 的 `gate` 既不是路径型、也不含 `go test`：%s ⇒ 判不出「谁在读它」",
				eidStr, ccrPyRepr(gate)))
		}
		// R9 —— ★ 描述 ↔ 真源列举对拍（GAP-20260927-228/-229）：truth_shape 点名的顶层键集合
		//      ⊇ 真源 JSON 的顶层键集合（去元数据键）；漏列即红，✗ 不依赖是否自称数字。
		r9path := ""
		if tp != "" {
			r9path = filepath.Join(root, tp)
		}
		miss, r9err := ccrR9Missing(r9path, ccrPyStr(ccrGet(e, "truth_shape", "")))
		if r9err {
			reds = append(reds, fmt.Sprintf("R9 %s 的真源 `%s` 解析失败 ⇒ truth_shape 点名集无法对拍（不给绿）", eidStr, tp))
		} else if len(miss) > 0 {
			parts := make([]string, len(miss))
			for i, k := range miss {
				parts[i] = "`" + k + "`"
			}
			reds = append(reds, fmt.Sprintf("R9 %s 的 `truth_shape` 漏点名真源顶层键 %d 个：%s",
				eidStr, len(miss), strings.Join(parts, " / ")))
		}
	}

	// ── R7 表自称的门在盘且就是本脚本（★ 退役态：件已退役 ⇒ 跳过 / 降为不过问）─────
	// G10（2026-10-07 · 退役批）：本件 `scripts/gates/check-contract-registry.py` 正被退役（M2 判据件 Go 化）
	//   ⇒ 登记表（`registry.json` 的 `note`）不再自称有该 py 门（点名字样已摘）；本判据改由 Go 半承载。
	//   R7 按其**退役态**处理：「表未自称本 py 门」或「该门件已不在盘」两种成因一律**跳过**（✗ 不判红）——
	//   件退役不是「表在说谎」，而是「该门已换人（Go 半）」⇒ ✗ 不得因缺 py 而 fail-closed。
	//   仅当表自称的门**在盘却与本身份不符**时仍判红（「表 ↔ 件不是同一份」这一条不变）。
	note := ccrPyStr(ccrGet(doc, "note", ""))
	var named []string
	for _, t := range ccrGateTokens(note) {
		if strings.HasSuffix(t, "check-contract-registry.py") {
			named = append(named, t)
		}
	}
	if len(named) == 0 {
		// 退役态①：表不再点名本 py 门 ⇒ R7 不过问（info.Wired 保持 false · info.Gate 保持空）
	} else {
		info.Gate = named[0]
		if named[0] != ccrSelfRel {
			reds = append(reds, fmt.Sprintf("R7 表自称的门是 %s，本件是 %s（表 ↔ 件不是同一份）", named[0], ccrSelfRel))
		}
		if !ccrIsFile(filepath.Join(root, ccrSelfRel)) {
			// 退役态②：表仍自称该 py 门、但该门件已退役（不在盘）⇒ 跳过（✗ 不判红）
		} else {
			info.Wired = true
		}
	}

	// ── R8 Go 侧声明同一路径 ────────────────────────────────────────────────
	gp := filepath.Join(root, ccrGoDeclRel)
	if !ccrIsFile(gp) {
		reds = append(reds, fmt.Sprintf("R8 Go 侧声明件不在盘：%s", ccrGoDeclRel))
	} else {
		src := ""
		if b, err := os.ReadFile(gp); err == nil {
			src = string(b)
		}
		m := regexp.MustCompile(regexp.QuoteMeta(ccrGoDeclConst) + `\s*=\s*"([^"]+)"`).FindStringSubmatch(src)
		if m == nil {
			reds = append(reds, fmt.Sprintf("R8 Go 侧 %s 的 %s 声明读不到", ccrGoDeclRel, ccrGoDeclConst))
		} else if m[1] != ccrSelfRel {
			reds = append(reds, fmt.Sprintf("R8 Go 侧声明 %s，表自称 %s（两处漂了）", m[1], ccrSelfRel))
		}
	}

	info.Reds = len(reds)
	if len(reds) > 0 {
		return 1, reds, info
	}
	lines := []string{
		fmt.Sprintf("契约登记表 OK：%d 条条目 · 八字段齐 · id 唯一 · truth 件全在盘 · gate 面全可判（无门 0 条）· "+
			"truth_shape 点名齐（真源顶层键无漏列）", info.Entries),
	}
	if info.Wired {
		lines = append(lines, fmt.Sprintf("表自称的门在盘：%s（= 本件）· Go 侧声明同一路径", info.Gate))
	} else {
		// 退役态（G10）：本 py 门件已退役 ⇒ R7 不过问（✗ 不判红）
		lines = append(lines, fmt.Sprintf("表自称的门件已退役（%s）· 本判据改由 Go 半承载（R7 降为不过问）", ccrSelfRel))
	}
	return 0, lines, info
}

// ── 小工具（与 py 语义逐条对齐）──────────────────────────────────────────────

// ccrGet —— py `d.get(k, def)`：键在 ⇒ 其值（含 None）；键缺 ⇒ def。
func ccrGet(d map[string]interface{}, k string, def interface{}) interface{} {
	if v, ok := d[k]; ok {
		return v
	}
	return def
}

// ccrTruthy —— py 真值（None/""/0/False ⇒ 假）。
func ccrTruthy(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	case bool:
		return x
	case float64:
		return x != 0
	default:
		return true
	}
}

// ccrPyStr —— py `str(v)`（None ⇒ "None"）。
func ccrPyStr(v interface{}) string {
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

// ccrPyRepr —— py `repr(v)`（本件面内取值恒为简单串/None ⇒ 单引号 repr 逐字）。
func ccrPyRepr(v interface{}) string {
	if s, ok := v.(string); ok {
		return "'" + s + "'"
	}
	return ccrPyStr(v)
}

// ccrPyReprList —— py 列表 repr：`['A', 'B']`。
func ccrPyReprList(xs []string) string {
	parts := make([]string, len(xs))
	for i, s := range xs {
		parts[i] = "'" + s + "'"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ccrPathPart —— py `path_part(truth)`：`(truth or "").strip()` 后按 `（` / `(` 取首段。
func ccrPathPart(truth interface{}) string {
	t := ""
	if s, ok := truth.(string); ok {
		t = s
	}
	t = strings.TrimSpace(t)
	if i := strings.Index(t, "（"); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	if i := strings.Index(t, "("); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	return t
}

// ccrGateTokens —— py `gate_tokens(gate)`：TOKEN_RE.findall(gate or "")。
func ccrGateTokens(gate string) []string {
	return ccrTokenRE.FindAllString(gate, -1)
}

// ccrR9Missing —— py `r9_missing(truth_abs, truth_shape)`：R9 **漏点名**的真源顶层键（排序后）⇒ (miss, err)。
//
//	err=true ⇒ 真源件在盘但**解析失败**（判红：点名集无法对拍，不许给绿）；
//	非 .json / 不在盘 / 非 JSON 对象（数组 / 标量）⇒ 不属本条面 ⇒ (nil, false)。
func ccrR9Missing(truthAbs, truthShape string) ([]string, bool) {
	if !strings.HasSuffix(truthAbs, ".json") || !ccrIsFile(truthAbs) {
		return nil, false
	}
	raw, err := os.ReadFile(truthAbs)
	if err != nil {
		return nil, true
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, true
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil, false
	}
	named := map[string]bool{}
	for _, m := range ccrBacktickRE.FindAllStringSubmatch(truthShape, -1) {
		tok := strings.TrimSpace(m[1])
		if tok == "" {
			continue
		}
		named[tok] = true
		named[strings.TrimSpace(strings.SplitN(tok, ".", 2)[0])] = true
	}
	var miss []string
	for k := range obj {
		if ccrMetaTopKeys[k] || strings.HasPrefix(k, "_") {
			continue
		}
		if !named[k] {
			miss = append(miss, k)
		}
	}
	sort.Strings(miss)
	return miss, false
}

// ccrIsFile —— py `os.path.isfile`。
func ccrIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// ccrExists —— py `os.path.exists`（存在即可 · 含目录）。
func ccrExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ccrJSONDump —— py `json.dumps({f: info[f] …}, ensure_ascii=False, sort_keys=True)`：
// 键按字母序 · 分隔符 `, ` / `: `（与 py 默认一致 —— **不是** Go 的紧凑形态）。
func ccrJSONDump(request string, info ccrInfo) string {
	req := map[string]bool{}
	for _, f := range strings.Split(request, ",") {
		if ccrIn(f, ccrJSONFields) {
			req[f] = true
		}
	}
	keys := []string{}
	for k := range req {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		var val string
		switch k {
		case "entries":
			val = strconv.Itoa(info.Entries)
		case "reds":
			val = strconv.Itoa(info.Reds)
		case "gate":
			val = ccrJSONString(info.Gate)
		case "wired":
			if info.Wired {
				val = "true"
			} else {
				val = "false"
			}
		}
		parts = append(parts, "\""+k+"\": "+val)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// ccrJSONString —— JSON 串（ensure_ascii=false 语义：非 ASCII 原样）。
func ccrJSONString(s string) string {
	const ccrBS = 0x5c // 反斜杠（用码位写，避开源里的转义形态）
	esc := func(ch rune) string { return string([]rune{ccrBS, ch}) }
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case 0x22: // 双引号
			b.WriteString(esc('"'))
		case ccrBS: // 反斜杠
			b.WriteString(string([]rune{ccrBS, ccrBS}))
		case 0x0a: // 换行
			b.WriteString(esc('n'))
		case 0x0d: // 回车
			b.WriteString(esc('r'))
		case 0x09: // 制表
			b.WriteString(esc('t'))
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("%s%04x", esc('u'), r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ── 自检（成对负控 · 1 正控 + 9 负控 = 10 组 · 与 py `self_test()` 逐字对齐）────────
//
//	与 py 同手段：进程内直调 `ccrCheck`（不起子进程）· 夹具只在 `os.MkdirTemp` 里造。
func ccrSelfTest(stdout io.Writer, root string, rootOK bool) int {
	var bad []string
	n := 0

	caseFn := func(name string, wantRC, gotRC int, wantRed string, lines []string) {
		n++
		if gotRC != wantRC {
			bad = append(bad, fmt.Sprintf("✗ %s：want rc=%d got rc=%d", name, wantRC, gotRC))
			return
		}
		if wantRed != "" {
			found := false
			for _, l := range lines {
				if strings.Contains(l, wantRed) {
					found = true
					break
				}
			}
			if !found {
				bad = append(bad, fmt.Sprintf("✗ %s：rc 对了但判词里没有 %s", name, ccrPyRepr(wantRed)))
			}
		}
	}

	real := ccrRegistryRel
	if rootOK {
		real = filepath.Join(root, ccrRegistryRel)
	}
	rc, lines, _ := ccrCheck(root, rootOK, real)
	caseFn("① 正控：真表 ⇒ rc=0", 0, rc, "", lines)
	if rc == 0 {
		found := false
		for _, l := range lines {
			if strings.Contains(l, "无门 0 条") {
				found = true
				break
			}
		}
		if !found {
			bad = append(bad, "✗ ① 正控：该印「无门 0 条」")
		}
	}

	tmp, err := os.MkdirTemp("", "zerg-cr-selftest-")
	if err != nil {
		tmp = os.TempDir()
	}
	defer os.RemoveAll(tmp)

	goodRaw, rerr := os.ReadFile(real)
	var good map[string]interface{}
	if rerr == nil {
		_ = json.Unmarshal(goodRaw, &good)
	}

	fixture := func(name string, mutate func(doc map[string]interface{})) (int, []string) {
		doc := ccrDeepCopy(good)
		if doc == nil {
			doc = map[string]interface{}{}
		}
		mutate(doc)
		p := filepath.Join(tmp, name)
		if b, e := json.MarshalIndent(doc, "", " "); e == nil {
			_ = os.WriteFile(p, b, 0o644)
		}
		rc, lines, _ := ccrCheck(root, rootOK, p)
		return rc, lines
	}

	// ② 负控：加一条**无门**的 entry
	rc, lines = fixture("no-gate.json", func(d map[string]interface{}) {
		es, _ := d["entries"].([]interface{})
		d["entries"] = append(es, map[string]interface{}{
			"id": "S-z", "name": "无门的条目", "truth": "core/internal/contract/reap.json",
			"truth_shape": "x", "version_field": "schema", "gate": "",
			"change_class": "B", "change_note": "x"})
	})
	caseFn("② 负控：加一条**无门**的 entry ⇒ rc=1", 1, rc, "S-z", lines)

	// ③ 负控：gate 指向不存在的件
	rc, lines = fixture("ghost-gate.json", func(d map[string]interface{}) {
		ccrEntryAt(d, 0)["gate"] = "scripts/gates/check-nonexistent-xyz.py"
	})
	caseFn("③ 负控：gate 指向不存在的件 ⇒ rc=1", 1, rc, "不在盘", lines)

	// ④ 负控：缺一个字段
	rc, lines = fixture("missing-field.json", func(d map[string]interface{}) {
		delete(ccrEntryAt(d, 1), "change_note")
	})
	caseFn("④ 负控：缺一个字段 ⇒ rc=1", 1, rc, "缺字段", lines)

	// ⑤ 负控：id 重复
	rc, lines = fixture("dup-id.json", func(d map[string]interface{}) {
		es, _ := d["entries"].([]interface{})
		if len(es) > 0 {
			d["entries"] = append(es, ccrDeepCopyAny(es[0]))
		}
	})
	caseFn("⑤ 负控：id 重复 ⇒ rc=1", 1, rc, "id 重复", lines)

	// ⑥ 负控：truth 件不在盘
	rc, lines = fixture("ghost-truth.json", func(d map[string]interface{}) {
		ccrEntryAt(d, 0)["truth"] = "core/internal/contract/这件的真源不在盘.json"
	})
	caseFn("⑥ 负控：truth 件不在盘 ⇒ rc=1", 1, rc, "truth 件不在盘", lines)

	// ⑦ 负控：gate 既非路径也不含 `go test`
	rc, lines = fixture("gate-blank-word.json", func(d map[string]interface{}) {
		ccrEntryAt(d, 1)["gate"] = "由某个没人知道的东西覆盖"
	})
	caseFn("⑦ 负控：gate 既非路径也不含 `go test` ⇒ rc=1", 1, rc, "判不出", lines)

	// ⑧ 负控：schema 号不对
	rc, lines = fixture("bad-schema.json", func(d map[string]interface{}) {
		d["schema"] = "别的号"
	})
	caseFn("⑧ 负控：schema 号不对 ⇒ rc=1", 1, rc, "schema", lines)

	// ⑪ 负控 · R9：自建**迷你根**（锚点 / 表 / 真源 / 门件 / Go 声明）——
	//     真源顶层键有 alpha/beta，而只点名 `alpha` ⇒ beta 漏列 ⇒ rc=1。
	//     ✗ 不复用调用方的 root（py 跑真仓、Go 单测跑合成仓 ⇒ 不 hermetic）。
	gr := pathContractRegistryPy
	if r9root, e := os.MkdirTemp("", "zerg-cr-r9-"); e == nil {
		w := func(rel, body string) {
			p := filepath.Join(r9root, rel)
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, []byte(body), 0o644)
		}
		w(filepath.Join("core", "internal", "version", "version.go"), "package version\n")
		w(filepath.Join("core", "internal", "contract", "r9.json"), "{\"alpha\": 1, \"beta\": 2}\n")
		w(gr, "# 夹具门件\n")
		w(filepath.Join("core", "cmd", "zerg", "family_impact_contract.go"),
			"package main\n\nconst impactContractRegistryGateRel = "+strconv.Quote(gr)+"\n")
		regDoc := map[string]interface{}{
			"schema": "zerg-contract-registry/1", "note": "真源见 `" + gr + "`", "authority": "夹具",
			"entries": []interface{}{map[string]interface{}{
				"id": "S-a", "name": "甲", "truth": "core/internal/contract/r9.json",
				"truth_shape": "`alpha`", "version_field": "schema", "gate": gr,
				"change_class": "A", "change_note": "x"}},
		}
		if b, me := json.Marshal(regDoc); me == nil {
			w(filepath.Join("core", "internal", "contract", "registry.json"), string(b))
		}
		rc, lines, _ = ccrCheck(r9root, true, filepath.Join(r9root, "core", "internal", "contract", "registry.json"))
		_ = os.RemoveAll(r9root)
		caseFn("⑪ 负控：truth_shape 漏点名真源顶层键 ⇒ rc=1", 1, rc, "漏点名", lines)
	} else {
		bad = append(bad, "✗ ⑪ 负控：建迷你根失败："+e.Error())
	}

	// ⑨ 负控：表读不到 ⇒ rc=2
	rc, lines, _ = ccrCheck(root, rootOK, filepath.Join(tmp, "根本没这份表.json"))
	caseFn("⑨ 负控：表读不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论", lines)

	// ⑩ 负控：根解析不到 ⇒ rc=2
	rc, lines, _ = ccrCheck("", false, real)
	caseFn("⑩ 负控：根解析不到 ⇒ rc=2（不给结论）", 2, rc, "不给结论", lines)

	if len(bad) > 0 {
		fmt.Fprintf(stdout, "✗ 自检失败（%d 组断言未过 / 共 %d 组）：\n", len(bad), n)
		for _, b := range bad {
			fmt.Fprintf(stdout, "   %s\n", b)
		}
		return 1
	}
	fmt.Fprintf(stdout, "✓ 自检全过（%d 组：正控 1 + 负控 10 · 成对）\n", n)
	return 0
}

// ccrEntryAt —— 取 entries[i]（map）；越界/类型不符 ⇒ 造一个空 map 挂回去（不改真实文件）。
func ccrEntryAt(d map[string]interface{}, i int) map[string]interface{} {
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

// ccrDeepCopy —— 深拷贝（走 JSON 往返 · 与 py `json.loads(json.dumps(good))` 同效）。
func ccrDeepCopy(d map[string]interface{}) map[string]interface{} {
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

// ccrDeepCopyAny —— 任意 JSON 值深拷贝。
func ccrDeepCopyAny(v interface{}) interface{} {
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]interface{}{}
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]interface{}{}
	}
	return out
}
