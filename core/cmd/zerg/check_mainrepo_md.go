// check_mainrepo_md.go —— 判据件 `scripts/gates/check-mainrepo-md.py`（192 行）的 Go 真搬
// （v2.5.14 · M2 · 搬运面 批-3 第 7 粒 · 入口 `checkMainrepoMdGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值真搬 Go（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），stdout / stderr / 退码三面逐字相同。
//
// 选它的理由（桶 k-o 内**行数最小**的纯静态件）：
//
//	✓ 选：判据是**纯静态只读** —— 扫面 = 仓根 `*.md` ∪ `docs/**/*.md`，对件级登记表
//	  判 B1 悬空登记 / B2 漏登记 / B3 空件。**不真跑子进程、不碰网络、零计时、零随机**。
//	· 现读 import 面：仅 `argparse` / `json` / `os` / `sys`（`tempfile` 只在 `self_test()` 内）——
//	  零 `subprocess` / `time` / `datetime` / `random` / `socket` / `urllib`。
//	· `tempfile.TemporaryDirectory` 本身**自带回收**（`with` 形）⇒ 非 A 类泄漏形态；Go 侧
//	  `os.MkdirTemp` + **建点成功支** `defer os.RemoveAll`。
//	· 退码 0/1/2 三档齐 · 默认档**纯只读无写面**。
//	· 遍历序**不影响输出**（`face()` 末尾全局 `sorted(out)`；明细也各自 `sorted`）⇒ 用 Go
//	  排序读法与 py 逐字同构（此处不涉「保 readdir 原序」面）。
//	· `git status` 现读不包含它（无人正在改）。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3822`（门107 · tri-report 档））；
//	· **不**删 `scripts/gates/check-mainrepo-md.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· **已接线**：入口 `checkMainrepoMdGateMain` 由 `gateCheckGoEntry["check-mainrepo-md"]` 分派（`zerg gate check check-mainrepo-md`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件 / `gate_inproc_table.go`。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func checkMainrepoMdGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 全绿 / 1 有 B1/B2/B3 / 2 仓根形态不对 · 扫面 0 件 · argparse 报错
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（argparse 帮助/退错正文按参考解释器逐字钉版）
//
// ═══════════════════ ★ argv 解析面（argparse 复刻）═══════════════════
//
//	py 走 `argparse.ArgumentParser(description="主仓 .md 件级只读门（GAP-20260926-157）")`，
//	注册 `--root`（取值 · 缺省 = `ZERG_ROOT` env 优先、回落 `os.getcwd()`）· `--json`（store_true）·
//	`--self-test`（store_true）。Go 侧复刻：`-h/--help` · 长旗标**前缀消歧** · `--X=V` 与 `--X V` 两形 ·
//	缺值报话（`argument --root: expected one argument`）· 未知旗标（`unrecognized arguments: …`）·
//	位置参（无位置参定义 ⇒ 亦并入 `unrecognized arguments`）· `--` 分隔符（其后全算位置参）。
//
// ═══════════════════ ★ 默认根定位（声明 (a)）═══════════════════
//
//	py default = `os.environ.get("ZERG_ROOT", os.getcwd())` ⇒ **不依赖 `__file__`**。Go 无
//	`__file__` 语境，但本件缺省本就起于 env/cwd ⇒ 两者**逐字同构**（✗ 无「脚本位置 ≠ cwd」推导差）。
//
// ═══════════════════ ★ 声明 (b) 自指检查 —— 本件**不在**自己的扫描面内（逐形态）═══════════════════
//
//	① 扫描面 = 仓根 `*.md` ∪ `docs/**/*.md`（**只收 `.md`**）⇒ 本件（`core/cmd/zerg/*.go`）
//	   不在内 ⇒ 自指 = 假。
//	② 件头/正文**零真引号路径原串**落进该扫描输出（本件只把登记表键当**输入**比对）。
//	③ self-test 造的合成件落点 = **自建临时目录**，不进仓树 ⇒ 真仓 `--self-test` 不受影响。
//	④ 是否落在**他门**扫描面内：本件在 `core/cmd/zerg/*.go` ⇒ 落 `check-error-kinds` 的 R3 面；R3 只认
//	   「错误构造调用的首参字面」（**带左括号**的调用式），**✗ 不扫 JSON 信封的键名** ⇒ 本件不写该
//	   调用式 ⇒ 命中 0。⚠ 亦落 `check-hardcoded-private-paths`（其 SCAN_DIRS 含 `core/`）⇒ 本件**不写
//	   任何私有绝对路径首段字面**（此处按**描述式**写，✗ 不把原串当例子写出来）⇒ 命中 0。
//
// ═══════════════════ ★ 未覆盖面（✗ 不声称逐字对齐）═══════════════════
//
//	① **argparse 极端内部形态**：短旗标簇（`-hx`）· 负数作选项值 · `--root=-x` 以外的前缀边界 ——
//	   本件按常用档复刻，未逐一验证的极端形态见此列。
//	② **读不了的目录**：py `os.walk` 默认 `onerror=None` ⇒ 读不了的**目录**被静默跳过（B 类目录面），
//	   被跳过的件既不进「可判」也不进「判不了」；本件同构（`os.ReadDir` 出错即跳过 ⇒ 输出同形）。
//	③ **悬空符号链接的根 `*.md`**：py `os.path.getsize` 对悬空链接抛 OSError（最外层无兜底 ⇒ 回溯 rc=1）；
//	   本件 `cmrmSize` 出错当非 0 ⇒ 该格**不退 1**（歧路，如实列此）。
//	④ **非 UTF-8 件名 / 组合码点排序**：py `sorted` 按码点序，Go `sort.Strings` 按 UTF-8 字节序 ——
//	   对合法 UTF-8 两者等序；非法序列档未覆盖。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// cmrmRegistry —— py `REGISTRY`（件级登记表 · `{仓相对路径: 理由}` · 42 行）。★ 单一真源。
var cmrmRegistry = map[string]string{
	"AGENTS.md":                                  "根面：仓库级约定（agent 施工/落件/提交纪律）",
	"CHANGELOG.md":                               "根面：版本变更流水",
	"README.md":                                  "根面：仓库门面与入口",
	"docs/README.md":                             "文档面·顶层：docs 体系入口",
	"docs/index-nav.md":                          "文档面·顶层：站内导航索引",
	"docs/术语表.md":                                "文档面·顶层：术语表正文",
	"docs/术语表-说明.md":                             "文档面·顶层：术语表说明",
	"docs/site/README.md":                        "文档面·site：发布站说明",
	"docs/site/nav.md":                           "文档面·site：站内导航",
	"docs/skills/code-quality.md":                "文档面·技能：代码质量",
	"docs/skills/tool-upgrade.md":                "文档面·技能：工具升级主线",
	"docs/skills/tool-upgrade-acceptance.md":     "文档面·技能：升级验收",
	"docs/skills/tool-upgrade-checklist.md":      "文档面·技能：升级清单",
	"docs/skills/tool-upgrade-pitfalls.md":       "文档面·技能：升级坑单",
	"docs/skills/tool-upgrade-rules.md":          "文档面·技能：升级规则",
	"docs/skills/tool-upgrade-self-evolution.md": "文档面·技能：自演化",
	"docs/skills/zerg-overview.md":               "文档面·技能：虫族总览",
	"docs/zh/README.md":                          "文档面·zh：中文入口",
	"docs/zh/参考-命令行.md":                          "文档面·zh：参考-命令行（生成区）",
	"docs/zh/参考-版本与锚.md":                         "文档面·zh：参考-版本与锚",
	"docs/zh/架构-总览.md":                           "文档面·zh：架构总览",
	"docs/zh/概念-虫卵与孵化.md":                        "文档面·zh：概念-虫卵与孵化",
	"docs/zh/模块-主控.md":                           "文档面·zh：模块-主控",
	"docs/zh/模块-任务与片.md":                         "文档面·zh：模块-任务与片",
	"docs/zh/模块-子端.md":                           "文档面·zh：模块-子端",
	"docs/zh/模块-网关与路由.md":                        "文档面·zh：模块-网关与路由",
	"docs/zh/用法-命令行入门.md":                        "文档面·zh：用法-命令行入门",
	"docs/zh/用法-安装与启动.md":                        "文档面·zh：用法-安装与启动",
	"docs/zh/用法-对话链路.md":                         "文档面·zh：用法-对话链路",
	"docs/en/README.md":                          "文档面·en：英文入口",
	"docs/en/参考-命令行.md":                          "文档面·en：参考-命令行（生成区）",
	"docs/en/用法-命令行入门.md":                        "文档面·en：用法-命令行入门",
	"docs/en/_drafts/README.md":                  "文档面·en·草稿：草稿区说明",
	"docs/en/_drafts/设计-文档体系-v1.0.md":            "文档面·en·草稿：文档体系设计稿",
	"docs/常青/优化清单.md":                            "文档面·常青：优化清单",
	"docs/常青/开发标准流程.md":                          "文档面·常青：开发标准流程",
	"docs/常青/战略-虫族终极愿景.md":                       "文档面·常青：终极愿景",
	"docs/常青/接入指南.md":                            "文档面·常青：接入指南",
	"docs/常青/接入指南-客户端配置.md":                      "文档面·常青：接入指南-客户端配置",
	"docs/常青/接入指南-行为与排障.md":                      "文档面·常青：接入指南-行为与排障",
	"docs/常青/白皮书-虫族经济模型-v0.1.md":                 "文档面·常青：经济模型白皮书",
	"docs/常青/白皮书-虫族经济模型-开放问题与路线图.md":             "文档面·常青：经济模型开放问题",
}

// ── py 路径语义家族（复刻 CPython posixpath · 见 templates/py-path-semantics.go）──
// ✗ 不用 filepath.Join（`..` 段 / 绝对后段 / 空段三类语义与 py 不同）。

// cmrmPyJoin —— 复刻 posixpath.join（按 CPython 逐段规则直译）。
func cmrmPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "/"):
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// cmrmPyNormpath —— 复刻 posixpath.normpath（★ 保留恰两个前导斜杠）。
func cmrmPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initialSlashes := 0
	if strings.HasPrefix(path, "/") {
		initialSlashes = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initialSlashes = 2
		}
	}
	var comps []string
	for _, comp := range strings.Split(path, "/") {
		switch {
		case comp == "" || comp == ".":
		case comp != ".." || (initialSlashes == 0 && len(comps) == 0) || (len(comps) > 0 && comps[len(comps)-1] == ".."):
			comps = append(comps, comp)
		default:
			if len(comps) > 0 {
				comps = comps[:len(comps)-1]
			}
		}
	}
	out := strings.Join(comps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// cmrmPyAbspath —— 复刻 posixpath.abspath（纯词法 · cwd 由调用方传入）。
func cmrmPyAbspath(cwd, path string) string {
	return cmrmPyNormpath(cmrmPyJoin(cwd, path))
}

// cmrmPyRelpath —— 复刻 posixpath.relpath（start 传 cwd）。
func cmrmPyRelpath(cwd, path, start string) string {
	if path == "" {
		return ""
	}
	split := func(p string) []string {
		var out []string
		for _, c := range strings.Split(p, "/") {
			if c != "" {
				out = append(out, c)
			}
		}
		return out
	}
	s, p := split(cmrmPyAbspath(cwd, start)), split(cmrmPyAbspath(cwd, path))
	i := 0
	for i < len(s) && i < len(p) && s[i] == p[i] {
		i++
	}
	var rel []string
	for k := 0; k < len(s)-i; k++ {
		rel = append(rel, "..")
	}
	rel = append(rel, p[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

// ── 件系统小工具（py 语义：isfile/isdir 跟随符号链接 ⇒ 用 os.Stat）──
func cmrmIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func cmrmIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func cmrmIsSymlink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&os.ModeSymlink != 0
}

func cmrmSize(p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		return -1
	}
	return fi.Size()
}

func cmrmCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

// cmrmFace —— py `face(root)`：根面 `*.md` ∪ `docs/**/*.md` ⇒ 仓相对路径，末尾全局 sorted。
func cmrmFace(root string) []string {
	out := []string{}
	if ents, err := os.ReadDir(root); err == nil {
		for _, e := range ents {
			name := e.Name()
			if strings.HasSuffix(name, ".md") && cmrmIsFile(cmrmPyJoin(root, name)) {
				out = append(out, name)
			}
		}
	}
	d := cmrmPyJoin(root, "docs")
	if cmrmIsDir(d) {
		cmrmWalk(d, root, &out)
	}
	sort.Strings(out)
	return out
}

// cmrmWalk —— 复刻 `os.walk`（剪 `.git`/`__pycache__` · followlinks=False · 只收 `.md`）。
// 遍历序不影响最终输出（外层 `sorted`），故用 `os.ReadDir`（已排序）等价。
func cmrmWalk(d, root string, out *[]string) {
	ents, err := os.ReadDir(d)
	if err != nil {
		return // py os.walk onerror=None：读不了的目录静默跳过（未覆盖面②）
	}
	for _, e := range ents {
		name := e.Name()
		p := cmrmPyJoin(d, name)
		isDir := false
		if fi, err := os.Stat(p); err == nil {
			isDir = fi.IsDir()
		}
		if isDir {
			if name == ".git" || name == "__pycache__" {
				continue
			}
			if !cmrmIsSymlink(p) { // followlinks=False
				cmrmWalk(p, root, out)
			}
			continue
		}
		if strings.HasSuffix(name, ".md") {
			*out = append(*out, cmrmPyRelpath(cmrmCwd(), cmrmPyJoin(d, name), root))
		}
	}
}

// cmrmJSONStr —— py `json.dumps(s, ensure_ascii=False)`：非 ASCII 直出 · ✗ 不转 `<`/`>`/`&`。
func cmrmJSONStr(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimRight(b.String(), "\n")
}

// cmrmJSONList —— py `json.dumps(list, ensure_ascii=False)`（分隔 `", "` · 空表 `[]`）。
func cmrmJSONList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = cmrmJSONStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cmrmRun —— py `run(root, as_json)`。
func cmrmRun(root string, asJSON bool, stdout, stderr io.Writer) int {
	if !cmrmIsFile(cmrmPyJoin(root, "AGENTS.md")) && !cmrmIsDir(cmrmPyJoin(root, "docs")) {
		if asJSON {
			fmt.Fprintln(stdout, "{\"rc\": 2, \"error\": \"仓根形态不对（无 AGENTS.md 也无 docs/）\"}")
		} else {
			fmt.Fprintln(stderr, "BLOCKED —— 仓根形态不对（无 AGENTS.md / docs/）⇒ rc=2，不给结论")
		}
		return 2
	}
	f := cmrmFace(root)
	if len(f) == 0 {
		if asJSON {
			fmt.Fprintln(stdout, "{\"rc\": 2, \"error\": \"扫面 0 件（空转）\"}")
		} else {
			fmt.Fprintln(stderr, "BLOCKED —— 扫面 0 件 ⇒ rc=2（空转 = 假覆盖，拒绝给结论）")
		}
		return 2
	}
	on := make(map[string]bool, len(f))
	for _, p := range f {
		on[p] = true
	}
	var dangling, missing, empty []string
	for k := range cmrmRegistry {
		if !on[k] {
			dangling = append(dangling, k)
		}
	}
	for _, p := range f {
		if _, ok := cmrmRegistry[p]; !ok {
			missing = append(missing, p)
		}
	}
	for _, p := range f {
		if _, ok := cmrmRegistry[p]; ok && cmrmSize(cmrmPyJoin(root, p)) == 0 {
			empty = append(empty, p)
		}
	}
	sort.Strings(dangling)
	sort.Strings(missing)
	sort.Strings(empty)
	bad := len(dangling) + len(missing) + len(empty)
	rc := 0
	if bad > 0 {
		rc = 1
	}
	if asJSON {
		fmt.Fprintf(stdout, "{\"rc\": %d, \"face\": %d, \"registered\": %d, \"dangling\": %s, \"missing\": %s, \"empty\": %s}\n",
			rc, len(f), len(cmrmRegistry), cmrmJSONList(dangling), cmrmJSONList(missing), cmrmJSONList(empty))
	} else {
		fmt.Fprintln(stdout, "主仓 `.md` 件级只读门（GAP-20260926-157）")
		fmt.Fprintf(stdout, "扫面 = 根面 *.md ∪ docs/**/*.md：%d 件 · 登记表 %d 行\n", len(f), len(cmrmRegistry))
		fmt.Fprintf(stdout, "── B1 悬空登记（表里点名、盘上无此件）：%d 处\n", len(dangling))
		for _, x := range dangling {
			fmt.Fprintf(stdout, "   ✗ %s\n", x)
		}
		fmt.Fprintf(stdout, "── B2 漏登记（盘上有、表里没有）：%d 处\n", len(missing))
		for _, x := range missing {
			fmt.Fprintf(stdout, "   ✗ %s\n", x)
		}
		fmt.Fprintf(stdout, "── B3 空件（登记过、0 字节）：%d 处\n", len(empty))
		for _, x := range empty {
			fmt.Fprintf(stdout, "   ✗ %s\n", x)
		}
		if bad > 0 {
			fmt.Fprintf(stdout, "⇒ 违规 %d 处（rc=1）：主仓 `.md` 必须逐件登记（B1 改表 · B2 补表 · B3 补正文）\n", bad)
		} else {
			fmt.Fprintf(stdout, "⇒ 全绿（rc=0）：%d 件主仓 `.md` 逐件登记 · 无空件\n", len(f))
		}
	}
	return rc
}

// cmrmSelfTest —— py `self_test()`：四合成档（含临时目录 · 建点成功支 defer 回收）。
func cmrmSelfTest(stdout, stderr io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "mainrepo-md-selftest-")
	if err != nil {
		td = os.TempDir()
	} else {
		defer os.RemoveAll(td)
	}
	_ = os.MkdirAll(cmrmPyJoin(td, "docs", "zh"), 0o755)
	_ = os.WriteFile(cmrmPyJoin(td, "AGENTS.md"), []byte("x\n"), 0o644)
	_ = os.WriteFile(cmrmPyJoin(td, "docs", "zh", "a.md"), []byte("y\n"), 0o644)
	saved := cmrmRegistry
	defer func() { cmrmRegistry = saved }()

	cmrmRegistry = map[string]string{"AGENTS.md": "根面", "docs/zh/a.md": "文档面"}
	if cmrmRun(td, false, stdout, stderr) != 0 {
		fmt.Fprintln(stdout, "  ✗ ① 正控失败：应回 0")
		ok = false
	}
	cmrmRegistry = map[string]string{"AGENTS.md": "根面"}
	if cmrmRun(td, true, stdout, stderr) != 1 {
		fmt.Fprintln(stdout, "  ✗ ② B2 失败：应回 1")
		ok = false
	}
	cmrmRegistry = map[string]string{"AGENTS.md": "根面", "docs/zh/a.md": "文档面", "docs/gone.md": "幽灵"}
	if cmrmRun(td, false, stdout, stderr) != 1 {
		fmt.Fprintln(stdout, "  ✗ ③ B1 失败：应回 1")
		ok = false
	}
	_ = os.WriteFile(cmrmPyJoin(td, "docs", "zh", "a.md"), nil, 0o644)
	if cmrmRun(td, false, stdout, stderr) != 1 {
		fmt.Fprintln(stdout, "  ✗ ④ B3 失败：空件应回 1")
		ok = false
	}
	verdict := "PASS"
	if !ok {
		verdict = "FAIL"
	}
	fmt.Fprintf(stdout, "自检 %s（4 格）\n", verdict)
	if ok {
		return 0
	}
	return 1
}

// ── argparse 复刻 ──────────────────────────────────────────────────────
const (
	cmrmProg  = "check-mainrepo-md.py"
	cmrmUsage = "usage: check-mainrepo-md.py [-h] [--root ROOT] [--json] [--self-test]"
	cmrmHelp  = "usage: check-mainrepo-md.py [-h] [--root ROOT] [--json] [--self-test]\n" +
		"\n" +
		"主仓 .md 件级只读门（GAP-20260926-157）\n" +
		"\n" +
		"optional arguments:\n" +
		"  -h, --help   show this help message and exit\n" +
		"  --root ROOT\n" +
		"  --json\n" +
		"  --self-test\n"
)

var cmrmLongOpts = []string{"help", "root", "json", "self-test"}

type cmrmNS struct {
	root     string
	rootSet  bool
	asJSON   bool
	selfTest bool
	help     bool
}

// cmrmResolveLong —— py argparse 长旗标前缀消歧（精确优先 · 唯一前缀可用 · 多义报话）。
func cmrmResolveLong(body string) (string, string) {
	if body == "" {
		return "", ""
	}
	for _, o := range cmrmLongOpts {
		if o == body {
			return o, ""
		}
	}
	var hits []string
	for _, o := range cmrmLongOpts {
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

// cmrmParseArgs —— 复刻 argparse 常用档，返回 (ns, errMsg)。errMsg 非空 ⇒ 走 usage+error rc=2。
func cmrmParseArgs(argv []string) (*cmrmNS, string) {
	ns := &cmrmNS{}
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
			canon, ambig := cmrmResolveLong(body)
			if ambig != "" {
				return nil, "ambiguous option: --" + body + " could match " + ambig
			}
			if canon == "" {
				unknown = append(unknown, a)
				continue
			}
			switch canon {
			case "help":
				ns.help = true
			case "json":
				ns.asJSON = true
			case "self-test":
				ns.selfTest = true
			case "root":
				if !hasVal {
					if i+1 >= len(argv) || (strings.HasPrefix(argv[i+1], "-") && argv[i+1] != "-") {
						return nil, "argument --root: expected one argument"
					}
					i++
					val = argv[i]
				}
				ns.root, ns.rootSet = val, true
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

func cmrmDefaultRoot() string {
	if v, ok := os.LookupEnv("ZERG_ROOT"); ok {
		return v
	}
	return cmrmCwd()
}

// checkMainrepoMdGateMain —— 入口（= py `main()`）。
func checkMainrepoMdGateMain(stdout, stderr io.Writer, argv []string) int {
	ns, errMsg := cmrmParseArgs(argv)
	if errMsg != "" {
		fmt.Fprintf(stderr, "%s\n%s: error: %s\n", cmrmUsage, cmrmProg, errMsg)
		return 2
	}
	if ns.help {
		fmt.Fprint(stdout, cmrmHelp)
		return 0
	}
	if ns.selfTest {
		return cmrmSelfTest(stdout, stderr)
	}
	root := ns.root
	if !ns.rootSet {
		root = cmrmDefaultRoot()
	}
	return cmrmRun(root, ns.asJSON, stdout, stderr)
}
