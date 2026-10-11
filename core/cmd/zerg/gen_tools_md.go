// gen_tools_md.go —— 生成器件 `tools/gen_tools_md.py`（112 行）的 **Go 真搬**
// （v2.5.14 · M2 · 批-4 生成器搬运 · 入口 `genToolsMdGateMain`）。
//
// 本件形态：生成器 · 纯静态只读注册表 · **只写 stdout**（无写盘面 · 无临时目录）。
// 外部依赖仅 Python 标准库（os/re/sys/collections）⇒ 无联网 · 无子进程 · 无真 TTY
// · 无第三方包 ⇒ 适格（现读 112 行全文核：零 `subprocess`/`socket`/`time`/`random`/
// `tempfile`/写面 `open(...,'w')`；唯二 IO 是读注册表与印 stdout/stderr）。
//
// 验收判据 = 同输入（同一棵仓树 / 同一份注册表）下，py 原件 ⟷ 本入口 产出
// **stdout 逐字节相同** ＋ 逐格 rc/stdout/stderr 三面相同。
//
// 逐字节对齐要点（照 py 逐条）：
//
//	① py `re.split(r"(?=\{\s*Name:)")` 是**前瞻零宽切分** ⇒ Go RE2 无 `(?=)` ⇒ 自写
//	   `gtmSplitBlocks`：取 `\{\s*Name:` 全部**非重叠匹配的起址**为切点、切点处开新段
//	   —— 与 py 零宽前瞻的切点集**逐位相同**（前瞻成立 ⇔ 该处 `\{\s*Name:` 能匹配；
//	   切点必须是字面 `{`，故两切点不可能相邻、两次匹配也不可能重叠）。
//	② py 的 `\s` 是 **Unicode 语义**、Go RE2 默认 ASCII ⇒ 真源是 ASCII Go 源码
//	   （`{` 与 `Name:` 之间只有空格/换行/制表）⇒ 实测不触发；仍列入未覆盖面。
//	③ `sorted(cat, key=lambda k: (-len(cat[k]), k))` = 计数降序 + 同数按**键升序** ⇒
//	   Go `sort.Slice`（先比 `-len`，再比键；键比较 = 码点序 = UTF-8 字节序）。
//	④ `sorted(cat[c])` = 键名升序 ⇒ `sort.Strings`。
//	⑤ 归类键 `c or "未分类"`：**空串** ⇒ 未分类（✗ 不把别的 falsy 也并进来 —— py 只判空串）。
//	⑥ `cat[c].append(name)` 的**插入序**只影响 `sorted` 之前的容器、不进输出行序
//	   ⇒ Go 用 map 取 `len`/收集键再排序，输出面逐字同（✗ 不靠 map 迭代序出输出行）。
//	⑦ 文本读取走 py **文本模式 universal newlines**：`\r\n` 与裸 `\r` 都折成 `\n`
//	   ⇒ 本件只把注册表当**正则扫描面**（分段/取字段/找 L0），不按行切、不 strip
//	   ⇒ 折行与否不影响命中集（`\r` 只可能落在 `\s` 里、被同一正则吃掉）。
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py `REPO = dirname(dirname(abspath(__file__)))` ⇒ 起于
//	    **脚本所在目录的父目录**；本件无 `__file__` 语境 ⇒ 从 **cwd** 取根
//	    （`abspath(cwd)`）。「脚本位置 == cwd」（即仓根执行 `python3 tools/gen_tools_md.py`）
//	    时两者读数逐字相同；否则**路径推导面不同** ⇒ 不属逐字对齐承诺。
//	(b) **自指检查**：本件扫描面 = 注册表 `core/internal/chat/chat_tool_registry.go`
//	    （✗ 不扫 `.go` 件自身）⇒ 本件（落 `core/cmd/zerg/`）**不在自身扫描面内**。
//	    量化依据：全件零形如「JSON 键名连续引号原串」的写法（一律描述式）· 退码仅
//	    0/1/2（⊆ 真源退码表已登记集）· 零绝对私有路径字面量。
//
// 本单边界（照实）：**只落码、不接线** ——
//
//	· 不在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树）；
//	· 不删 py 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `genToolsMdGateMain` 目前**无调用点**（M2 刻意状态）。
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// ── 常量（照 py 逐字）────────────────────────────────────────────────

// gtmSrcRel —— 注册表在仓内的相对落点（py `SRC = os.path.join(REPO, "core", "internal", "chat", "chat_tool_registry.go")`）。
var gtmSrcRel = []string{"core", "internal", "chat", "chat_tool_registry.go"}

// gtmDoc —— py 模块 __doc__（`--help`/`-h` 逐字印它 ＋ 一个尾换行）。由 py 原件现读抽取。
var gtmDoc = "gen_tools_md.py — 从工具注册表生成工具目录（公开文档的中文原版）\n" +
	"\n" +
	"用法（仓库根执行）：\n" +
	"    python3 tools/gen_tools_md.py > docs/TOOLS.zh-CN.md          # 公开仓\n" +
	"    python3 tools/gen_tools_md.py > publish/docs/TOOLS.zh-CN.md   # 私有仓（快照源目录）\n" +
	"\n" +
	"数据源：core/internal/chat/chat_tool_registry.go（单一真相源）\n" +
	"说明：\n" +
	"  - 本脚本只读注册表；改工具清单请改注册表，不要手改生成的文档。\n" +
	"  - 生成结果含中文页头（语言切换 + 源声明），**是中文原版**；\n" +
	"    英文版 docs/TOOLS.en.md 是它的派生翻译，改完必须同 PR 同步（工具名/分类不译，描述文字保持中文）。\n"

var (
	// gtmREBlock —— 前瞻切分点：`(?=\{\s*Name:)` 的「非零宽化」等价形态。
	gtmREBlock = regexp.MustCompile(`\{\s*Name:`)
	// gtmREEntry —— 条目正则：`\{\s*Name:\s*"([^"]+)"\s*,\s*Category:\s*"([^"]*)"`。
	gtmREEntry = regexp.MustCompile(`\{\s*Name:\s*"([^"]+)"\s*,\s*Category:\s*"([^"]*)"`)
	// gtmREL0 —— L0 标志：`L0:\s*true`。
	gtmREL0 = regexp.MustCompile(`L0:\s*true`)
)

// ── posixpath 语义（照 py os.path.*，✗ 不用 filepath.Join）─────────────────

// gtmPyJoin —— posixpath.join：空段忽略 · 后段绝对路径丢弃前段 · `..` 原样留串。
func gtmPyJoin(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "/") {
			b.Reset()
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "/") {
			b.WriteString("/")
		}
		b.WriteString(p)
	}
	return b.String()
}

// ── 解析（py 主逻辑逐段）─────────────────────────────────────────────

type gtmEntry struct {
	name     string
	category string
}

// gtmSplitBlocks —— py `re.split(r"(?=\{\s*Name:)")` 的等价：切点 = `\{\s*Name:` 的非重叠起址。
func gtmSplitBlocks(src string) []string {
	locs := gtmREBlock.FindAllStringIndex(src, -1)
	if len(locs) == 0 {
		return []string{src}
	}
	out := make([]string, 0, len(locs)+1)
	prev := 0
	for _, l := range locs {
		out = append(out, src[prev:l[0]])
		prev = l[0]
	}
	out = append(out, src[prev:])
	return out
}

// gtmParse —— py 的 entries / l0 两收集（按块序 · 保序）。
func gtmParse(src string) ([]gtmEntry, map[string]bool) {
	var entries []gtmEntry
	l0 := map[string]bool{}
	for _, b := range gtmSplitBlocks(src) {
		m := gtmREEntry.FindStringSubmatch(b)
		if m == nil {
			continue
		}
		entries = append(entries, gtmEntry{name: m[1], category: m[2]})
		if gtmREL0.MatchString(b) {
			l0[m[1]] = true
		}
	}
	return entries, l0
}

// gtmCategoryName —— py `c or "未分类"`（只判空串）。
func gtmCategoryName(c string) string {
	if c == "" {
		return "未分类"
	}
	return c
}

// gtmBuild —— py main() 的产出段：返回 stdout 正文（含 print 的尾换行由调用方补）＋
// 三枚统计（total / L0 数 / 类别数）。
func gtmBuild(src string) (string, int, int, int) {
	entries, l0 := gtmParse(src)
	cat := map[string][]string{}
	for _, e := range entries {
		k := gtmCategoryName(e.category)
		cat[k] = append(cat[k], e.name)
	}
	total := len(entries)
	lenL0 := len(l0)
	lenCat := len(cat)

	out := []string{
		"# 工具目录（TOOLS）",
		"",
		"[English](TOOLS.en.md) | **中文**",
		"",
		"> *中文原版（唯一真相源）。英文版为派生翻译；两版如有不一致，以**本文**为准。*",
		"> *注：下方工具**描述**即实际发送给模型的文本（中文）——本版有意不翻译描述文字。*",
		"",
		"> 本文件由工具注册表**自动生成**（`core/internal/chat/chat_tool_registry.go`）——",
		"> 改工具清单请改注册表，不要手改本文件。",
		fmt.Sprintf("> 统计：**%d** 个注册工具，其中 **%d** 个为 L0 常驻（直接写进模型提示），其余按需发现。", total, lenL0),
		"",
		"## 两层工具",
		"",
		"| 层 | 数量 | 说明 |",
		"|---|---|---|",
		fmt.Sprintf("| **L0 常驻** | %d | 高频基础能力，定义直接注入模型提示，随时可调 |", lenL0),
		fmt.Sprintf("| **延迟加载** | %d | 通过 `tool_search`（按关键词/类别）发现后再调用，避免提示膨胀 |", total-lenL0),
		"",
		"调用约定：模型先看提示里有没有；没有就用 `tool_search` 搜（支持中文关键词），拿到定义再调。",
		"",
		"## 按类别",
		"",
	}

	keys := make([]string, 0, len(cat))
	for k := range cat {
		keys = append(keys, k)
	}
	// py `sorted(cat, key=lambda k: (-len(cat[k]), k))`：计数降序 + 同数键升序。
	sort.Slice(keys, func(i, j int) bool {
		li, lj := len(cat[keys[i]]), len(cat[keys[j]])
		if li != lj {
			return li > lj
		}
		return keys[i] < keys[j]
	})
	for _, c := range keys {
		out = append(out, fmt.Sprintf("### %s（%d）", c, len(cat[c])))
		out = append(out, "")
		ns := append([]string(nil), cat[c]...)
		sort.Strings(ns) // py sorted(cat[c])
		for _, n := range ns {
			suffix := ""
			if l0[n] {
				suffix = "  ← **L0 常驻**"
			}
			out = append(out, "- `"+n+"`"+suffix)
		}
		out = append(out, "")
	}

	out = append(out,
		"## 工具自身文档（履历）",
		"",
		"每个工具的**版本、能力、参数、注意事项**写在 `tools/<name>.md`（履历），",
		"与注册表、工具简介三者应保持一致——这是本项目的硬约定（见 `docs/design/工具升级规范.md`）。",
		"",
		"## 安全相关",
		"",
		"- shell（`bash`）的删除类命令受**范围闸门**约束：仅允许任务工作区与 `ZERG_EXTRA_ALLOW_DIR` 白名单",
		"- 文件类工具（`read`/`write`/`edit`/`glob`）同样受路径域限制，越界返回**可执行的替代路径**而非空报错",
		"- 参数格式错误会触发**格式反馈协议**：返回「系统断言 + 正确示例」，避免模型重发同样的坏请求",
		"",
		"## 重新生成",
		"",
		"```bash",
		"python3 tools/gen_tools_md.py > docs/TOOLS.zh-CN.md   # 英文版 docs/TOOLS.en.md 需同 PR 同步",
		"```",
		"",
	)

	return strings.Join(out, "\n"), total, lenL0, lenCat
}

// ── IO 面（取不到输入一律不静默）────────────────────────────────────────

// gtmRepoRoot —— 声明行 (a)：无 `__file__` 语境 ⇒ 取 `abspath(cwd)`。
func gtmRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	abs, err := filepath.Abs(wd)
	if err != nil {
		return wd
	}
	return abs
}

// gtmPyOSErrMsg —— 复刻 py 的 OSError 文案 `[Errno N] <strerror>: '<path>'`。
func gtmPyOSErrMsg(path string, err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", path)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Sprintf("[Errno 13] Permission denied: '%s'", path)
	}
	if st, e := os.Stat(path); e == nil && st.IsDir() {
		return fmt.Sprintf("[Errno 21] Is a directory: '%s'", path)
	}
	return fmt.Sprintf("[Errno 5] Input/output error: '%s'", path)
}

// gtmReadRegistry —— py `open(SRC, encoding="utf-8").read()` 的两条失败面：
// ① 打不开（OSError）⇒ 具名 errno；② 字节非合法 UTF-8（UnicodeDecodeError·非 OSError）
// ⇒ py 无最外层兜底＝回溯 rc=1 ⇒ 本件按新口径 fail-closed rc=2（见未覆盖面）。
func gtmReadRegistry(path string) (string, bool, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false, gtmPyOSErrMsg(path, err)
	}
	if !utf8.Valid(b) {
		return "", false, "注册表不是合法 UTF-8 —— fail-closed（py 侧为回溯）"
	}
	return string(b), true, ""
}

// ── 入口编排 ────────────────────────────────────────────────────────

func gtmHas(argv []string, s string) bool {
	for _, a := range argv {
		if a == s {
			return true
		}
	}
	return false
}

// gtmMainRoot —— py main()，根显式传入（便于对拍/单测，✗ 不依赖 cwd）。
func gtmMainRoot(stdout, stderr io.Writer, root string) int {
	path := gtmPyJoin(append([]string{root}, gtmSrcRel...)...)
	src, ok, msg := gtmReadRegistry(path)
	if !ok {
		fmt.Fprintf(stderr, "读取注册表失败: %s\n", msg)
		return 1
	}
	text, total, lenL0, lenCat := gtmBuild(src)
	fmt.Fprint(stdout, text+"\n")
	fmt.Fprintf(stderr, "# 生成完成：%d 工具 / %d L0 / %d 类别\n", total, lenL0, lenCat)
	return 0
}

// gtmRun —— 旗标面（照 py `__main__` 逐字）：
//
//	未知参数（以 `-` 起且不在 --help/-h 内）⇒ 逐字 `✗ 未知参数: <原样>` + rc=2；
//	`--help` / `-h` ⇒ 只印 __doc__（＋一个尾换行）⇒ rc=0；
//	否则 ⇐ main()（rc 0 或 1）。
func gtmRun(stdout, stderr io.Writer, argv []string) int {
	// ★ `--timeout` / `--cancel-on-interrupt` 是**全局旗标**：由 `main.go` 全局面消费
	//   （墙钟上界 / Ctrl-C 取消）；到不了本运行体也**不须本件另立语义** ⇒ 在此登记「吞下」
	//   （同族先例：`gate run` 闭白名单 `gateRunValueFlags`/`gateRunBoolFlags` 已登记同两枚）。
	known := map[string]bool{"--help": true, "-h": true,
		"--timeout": true, "--cancel-on-interrupt": true, "--node": true}
	var bad []string
	for _, a := range argv {
		if strings.HasPrefix(a, "-") && !known[a] {
			bad = append(bad, a)
		}
	}
	if len(bad) > 0 {
		fmt.Fprintf(stderr, "✗ 未知参数: %s\n", strings.Join(bad, " "))
		return 2
	}
	if gtmHas(argv, "--help") || gtmHas(argv, "-h") {
		fmt.Fprint(stdout, gtmDoc)
		fmt.Fprint(stdout, "\n")
		return 0
	}
	return gtmMainRoot(stdout, stderr, gtmRepoRoot())
}

// genToolsMdGateMain —— 本件入口（inproc 口径：argv **不含程序名**）。
func genToolsMdGateMain(stdout, stderr io.Writer, argv []string) int {
	return gtmRun(stdout, stderr, argv)
}
