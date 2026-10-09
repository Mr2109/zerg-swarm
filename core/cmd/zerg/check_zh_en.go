// check_zh_en.go —— 判据件 `scripts/gates/check-zh-en.py`（691 行 · 40825 B）的 **Go 真搬**
// （v2.5.14 · M2 · 新口径靶：外部树走显式入参旋钮 · 入口 `checkZhEnGateMain` · 承接 GAP-20261001-13）。
//
// ★ 选件说明（停手报明 + 改搬同族 .py）：本单原靶 = `scripts/gates/check-i18n-drift.py`；实查该
//
//	目录下**只有 `check-i18n-drift.sh`（bash 形态 · 197 行）· 无同名 .py**（`ls scripts/gates/*i18n*`
//	命中 1 件 = .sh）。按「无 .py ⇒ 报明实际文件名与形态并改搬同族 .py」的口径，本件改搬同族双
//	语判据 **`scripts/gates/check-zh-en.py`**（zh/en 双语与术语检查 · 同为 i18n 漂移族）。原 .sh 件
//	一字未动、不入本批。
//
// 验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码 **三面逐字相同**
// （除下列**显式声明的**新口径差异面）。
//
// 本件与新口径（外部树走显式入参旋钮）—— 逐条对应：
//
//	① 旋钮名**照 py 已有同名旗标/env**：py 本件**无任何外部树旗标 / env**（`ROOTS` 是**写死的**
//	   六个 cwd 相对目录）⇒ 按「名照 py 同名旗标/env」无可照之名时取**同族最简同名**：本件落
//	   `--root <目录>`（同 `check-glossary.py` 的 `--root` 语义 + 同族式样）＋ env `ZERG_ZHEN_ROOT`。
//	   ✗ 不造第三个互不认识的名字。
//	② 缺省值 = py 默认推导**逐字同构**：py `ROOTS = ["core", ...]` 相对 **cwd** 遍历 ⇒ 缺省旋钮
//	   为空串 ⇒ 本件以 `chzeJoin(root, r)`（root=="" ⇒ 逐字等于 r）遍历**同样六个相对目录** ⇒
//	   stdout 里 `p = dp + "/" + f` 逐字同 py 的 `os.path.join(dp, f)`（含 `..` 段才串拼；本件 ROOTS
//	   无 `..` 段 ⇒ `chzeJoin` 的 normalizer 无机会吃段，但为逐字对齐仍自实现 posix join）。
//	③ ★ 退 2 边界：py 的**三条**「取不到」分支（cwd 下 core/ 与 agent/ 都不在 · 扫到 0 件 ·
//	   登记表指向的件不在盘）**逐字照搬**退 2；`--root` 指向的树取不到 ⇒ 走同一分支（core/agent
//	   都不在 ⇒ 不给结论）。✗ 不静默绿。
//	④ **读不了的件**：py 该档本就是 `⚠ 读不了 …` + `continue`（**不改退码** · 非 rc=2）⇒ 本件逐字
//	   沿用（✗ 不抬档，交回入账）。
//
// ★ 目录遍历序（本批最重要标准）：py `scan()` 用 `os.walk` ⇒ 内部 `os.scandir` 的 **readdir 原序**
//
//	（未排序）。Go `os.ReadDir` / `filepath.WalkDir` / `filepath.Walk` **会排序** ✗ ⇒ 本件一律用
//	`f.ReadDir(-1)`（保留原序）逐层递归，**逐字复刻 os.walk 产出序**（先当前层 `fn`（按 entry 序）
//	· 再按 `dn` 序下钻子目录 · `dn[:] = [d for d in dn if d not in SKIP_DIRS]` 名过滤）。
//	★ 区分：**遍历序** = readdir 原序（本件照复刻）；py 代码里 `sorted()` 只出现在 main() 的
//	`sorted(set(missing))`（登记表缺件名去重排序）—— 那才是「py 明写 sorted 的序」，本件用 sort 复现，
//	两者**不是**同一个东西：输出行序的决定者 = 遍历序（`hits[...]` 追加序），非 sorted。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-zh-en.py` 原件（退场规则：**对拍通过后才**退场）；
//	· 入口 `checkZhEnGateMain` 目前**无调用点**（M2 的刻意状态）。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `ROOTS` 是**字面相对串**，靠 cwd 解析（与 `__file__` 无关）；本件
//	    无 cwd 之外的语境 ⇒ 缺省旋钮 root="" ⇒ 遍历同样的字面相对串 ⇒ 与 py **逐字同构**。
//	    **旋钮缺省 vs py 默认推导逐字对比**：
//	      py  `os.walk("core")` ⟷ Go `chzeWalk(chzeJoin("", "core"))` = `chzeWalk("core")`
//	      py  `os.path.join(dp, f)` ⟷ Go `p = dp + "/" + f`（dp 以 ROOTS 条目起头）
//	    即「cwd == py 的 cwd」时两读数逐字相同（对拍甲/乙两环境已证）。
//	(b) **自指检查（先真判 · 四形态逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 是**。本门扫面 = `ROOTS`（含 `core`）下 `TERM_EXT`（含 `.go`）件；
//	       本件 = `core/cmd/zerg/check_zh_en.go`（`.go`）⇒ **落在自己扫面内**。
//	       **量化依据**：`chzeTermExt` 含 `.go`（1 处定义）；本件路径首段 `core` ∈ `chzeRoots`。
//	    ② **本件落在任何他门扫描面内吗 ⇒ 是**（`check-hardcoded-private-paths.py` 的 SCAN_DIRS 含
//	       `core/`；`check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`）⇒ 按纪律处置：
//	       · 正文**零私有绝对路径字面量**（盘根绝对路径、家目录前缀一概不写；真仓根/外部树一律
//	         运行时取；需要举例时拆开拼串）；
//	       · **不写**禁用词**裸字面量**（`chzeForbid` 的键用 `\u` 转义构造 ⇒ 源码行里**没有**被扫
//	         形态的连续子串；这是本件的自指处置 —— 否则本门会**自己抓自己**）；
//	       · **不写**反引号包裹的门扫描标记原串（`check-keep-marks` 一类）；
//	       · self-test 夹具里的合成词条（禁用词）只在**运行时串拼**，✗ 不落本件正文的连续被扫形态。
//	    ③ **证明处理有效**：落仓前后跑本门（py 原件）同一命令，命中数与输出 sha16 **逐字不变**
//	       （见交付读数 —— 落此件后 `合计命中` 仍 = 21、`登记过期` 仍 = 0）。
//	    ④ 本件**不接线** ⇒ 不进任何命令树/用法面 ⇒ 无「他门用法面」新增字面量。
//
// py 原件真口子（★ 不自行修 · 逐字保留 · **交回入账**）：
//
//	① `--self-test` 的 `selftest_fp()` 用 `tempfile.mkdtemp` 且**从不回收**（每跑一次自检，系统临时
//	   目录多一个 `zh-en-fp-selftest-*` 残留）—— 本件 Go 侧用 `defer os.RemoveAll` 回收（**输出不可
//	   见差异**：py 只打 `  <名> …` 与末行，均不含夹具路径 ⇒ 两态 stdout 逐字同）。
//	② `selftest_fp()` 里 `os.chdir(tmp)` 后**只在 `finally` 复原**；若中途异常，cwd 可能不复原 ——
//	   本件同样以 `defer` 复原（行为等价、无输出差异）。
//	③ `main()` 最外层**无兜底**：py 只认 `--self-test`/`--selftest` 两个旗标，**其余 argv 一概不解析、
//	   不报未知参数**（与 check-glossary.py 不同）⇒ 本件逐字沿用「忽略未知 argv」。**新增旋钮
//	   `--root` 是本批新口径**（py 无此旗标）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① ★ **正则语义差**（逐条）：
//	   · py `\s` / `\S` 是 **Unicode 语义**（含 `\v`）；Go RE2 默认 **ASCII**（`\s`=`[\t\n\f\r ]`，
//	     **不含 `\v`**）。涉及位 = `RE_HEREDOC_START` 的 `\s*` · `ROLE_PAT` 的 `host\s*:` /
//	     `Machine\s*:`。**手写对齐**：本件以 `chzePyWS` 显式类（`[\t\n\x0b\f\r \x{85}\x{a0}
//	     \x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`）替代 `\s` ⇒ 与 py
//	     的 `\s` 逐码点一致（真仓语料该位零命中）。
//	   · py `\b` 是 **Unicode 词界**；Go RE2 `\b` 是 ASCII 词界。涉及位 = `ROLE_PAT` 的 `local\b`。
//	     **手写对齐**：本件以 `local(?:[^\p{L}\p{N}_]|$)` 替代 `local\b`（Unicode 词字符右界 + 串尾）。
//	   · py `[^\w]`/`\w`（`(?<![...])`... 本件未用）—— ✗ 未涉及。
//	   · py `RE_TRIPLE` 用**反向引用** `\1`（RE2 **不支持**）⇒ **手写对齐**：本件 `chzeStripBlocks`
//	     以「最早定界符（`"""` 优先于 `'''`）→ 下一次同款定界符」的最短配对扫描复刻 `re.sub` 的
//	     非重叠、非贪婪、左到右语义（自证 `[三引号]`/`[遮蔽未闭合]` 两条已钉）。
//	   · py `RE_CMDSUB` / `RE_HEREDOC_START` 无反向引用 ⇒ RE2 可直接编译（逐字同式样）。
//	   · py `$`（无 `(?m)`）= 匹配**串尾换行之前**；Go RE2 `$`（无 `(?m)`）= 只匹配**串尾**。本件
//	     `ROLE_PAT` 仅在**单行** `code`（`strip_noncode` 输出，无 `\n`）上 search ⇒ 两态逐字同。
//	② **`_py_quote_blind_lines` 的对拍源是 `python-tokenize`**（Python 自己的词法器）：本件无 py
//	   词法器 ⇒ 以**自写 Python 字符串行扫描**（`chzePyStrLines`）近似其 `strlines` 真值。自证
//	   `c7`（`_yu1==[3]` / `_yu2==[]`）已逐字命中；对**病态 .py**（配不齐三引号落在注释/普通串里）
//	   的判别粒度可能与 tokenize 有极小差（真仓 1222 件现读 `MASK_PYQ` 空 ⇒ 两态 rc 同）。
//	③ 读文件失败正文：py 打 `  ⚠ 读不了 <p>：<OSError 串>` ⟷ 本件打 Go `err` 串（正文自定、不属对齐）。
//	④ **非 UTF-8 角落**：py `open(…, errors="replace")` 把非法字节换成 U+FFFD；本件 `string(b)` 保留
//	   原字节，RE2 按 rune 解码遇非法字节得 U+FFFD ⇒ 多数形态同，粒度角落可能不同（真仓语料全 UTF-8）。
//	⑤ `--root` 旋钮为本批新口径（py 无）—— 属**新增面**、非「对齐承诺」；缺省（不给 `--root`/
//	   env）时两态逐字同（对拍已证）。
//
// `--evidence GAP-20261001-13`（在册 M2 承接 · 先已回读 · 语义对口：want_family=gate / want_action=check）。
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── py 常量（逐字复刻）────────────────────────────────────────────────────────────────────

var chzeRoots = []string{"core", "agent", "gateway", "scripts", "deploy", "docs/skills"}

var chzeSkipDirs = []string{"node_modules", "target", "vendor", ".git", "dist", ".venv"}

var chzeFullwExt = []string{".sh", ".yaml", ".yml", ".json", ".toml", ".py", ".plist", ".conf"}

// chzeTermExt = FULLW_EXT + (".go",) —— 只用于成员判定。
var chzeTermExt = append([]string{".go"}, chzeFullwExt...)

// chzeFullw = py FULLW 串（全角标点集）。.go 不在 FULLW_EXT ⇒ 本件正文的全角标点不被自查。
var chzeFullw = "，。：；（）！？“”‘’　"

// chzeForbidKV —— 有序对（复刻 py dict 插入序）。键以 `\u` 转义构造 ⇒ 源码行不含裸字面量（自指处置）。
type chzeForbidKV struct {
	k, v      string
	commentOK bool // 整行注释内是否豁免禁用词：术语「词选择」组 = true；私有称谓「保密」组 = false（任何上下文都点名）
}

var chzeForbid = []chzeForbidKV{
	{"\u866b\u5de2", "术语铁律：只有「虫茧」", true},
	{"\u96c6\u88c5\u7bb1", "术语铁律：只有「虫茧」", true},
	{"\u8001\u53e8", "私有称谓：署名只用 GitHub 用户名 Mr2109", false},
	{"\u8111(Hermes)", "私有称谓：署名只用 GitHub 用户名 Mr2109", false},
	{"\u8111\uff08Hermes\uff09", "私有称谓：署名只用 GitHub 用户名 Mr2109", false},
}

// chzeSelfRel —— 本件相对路径（自指形态①：本件落在自己扫面内 ⇒ 列入白名单，与 py 白名单自身同规）。
const chzeSelfRel = "core/cmd/zerg/check_zh_en.go"

var chzeWhitelist = map[string]bool{
	"scripts/build/publish-preflight.sh":         true,
	"scripts/build/publish-public.sh":            true,
	"scripts/build/test-publish-parity.sh":       true,
	"docs/skills/code-quality.md":                true,
	"scripts/gates/check-zh-en.py":               true,
	"scripts/gates/check-public-face-wording.py": true,
	"scripts/gates/check-private-path-forms.py":  true,
	chzeSelfRel: true,
}

// chzeRoleExemptHint —— 旧角色词豁免后缀。
var chzeRoleExemptHint = []string{"_test.go", "selfupdate", "toolchain"}

// chzePyWS —— py `\s` 的 Unicode 等价类（手写对齐 · 见未覆盖面①）。
const chzePyWS = `[\t\n\x0b\f\r \x{85}\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

// ── 正则（逐字复刻 py；★ `\s`/`\b` 差异见件头未覆盖面①）────────────────────────────────────

var (
	// ROLE_PAT：`host\s*:\s*['\"]?local\b | Machine\s*:\s*['\"]local['\"] | MachineSnapshot\(['\"]local['\"]\)`
	chzeRERole = regexp.MustCompile(`(host` + chzePyWS + `*:` + chzePyWS + `*['"]?local(?:[^\p{L}\p{N}_]|$)|Machine` + chzePyWS + `*:` + chzePyWS + `*['"]local['"]|MachineSnapshot\(['"]local['"]\))`)
	// RE_CMDSUB：`\$\((?:[^()]|\([^()]*\))*\)`
	chzeRECmdSub = regexp.MustCompile(`\$\((?:[^()]|\([^()]*\))*\)`)
	// RE_HEREDOC_START：`<<-?\s*['\"]?([A-Za-z_][A-Za-z0-9_]*)['\"]?`
	chzeREHeredocStart = regexp.MustCompile(`<<-?` + chzePyWS + `*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)
)

// ── KNOWN_FP（逐行豁免口 · 键 = `(件, 内容子串)` · 子串以 `\u` 转义串拼）───────────────────────────

type chzeFPDef struct{ path, sub, why string }

// chzeFPStale —— 登记过期条目（path · 子串 · 理由 · 命中行串）。
type chzeFPStale struct{ path, sub, why, hit string }

var chzeKnownFP = []chzeFPDef{
	{"core/cmd/zerg/check_glossary.go", "\"" + "\u866b\u5de2|" + "\u96c6\u88c5\u7bb1" + "\", \"pod\"", "术语词表字面量（`COLS` 里逐字写着 `" + "\u866b\u5de2|" + "\u96c6\u88c5\u7bb1" + "` 用作**被检词**，与 `docs/术语表.forbid.txt` 同源）"},
	{"core/cmd/zerg/check_glossary.go", "terms(\"docs/zh/a.md\", \"这里叫" + "\u866b\u5de2" + "。\", \"zh\")", "自证夹具：断言「T2 中文页禁写 " + "\u866b\u5de2" + "」的那条**用例正文**（`\"这里叫" + "\u866b\u5de2" + "。\"`）"},
	{"core/cmd/zerg/check_glossary.go", "旧称" + "\u866b\u5de2" + "。<!-- glossary-allow -->", "自证夹具：断言「T2 单条豁免生效」的**用例正文**（`\"旧称" + "\u866b\u5de2" + "。<!-- glossary-allow -->\"`）"},
	{"core/cmd/zerg/check_glossary.go", "\u866b\u5de2\\t\\t\\t\\t\u866b\u5de2\\t\\tMr2109\\t英文待拍", "`Q-176` 第五批合成夹具：喂 `load_glossary` 的**词表行**（`" + "\u866b\u5de2" + "` 当 `forbid_zh` 列 ⇒ 验 T2 抓得到）"},
	{"core/cmd/zerg/check_glossary.go", "WriteFile(dirty, []byte(\"旧称：这里叫" + "\u866b\u5de2" + "。", "`Q-176` 第五批合成夹具：中文页的**用例正文**（`\"旧称：这里叫" + "\u866b\u5de2" + "。\"` ⇒ 验 T2 负控 rc=1）"},
	{"core/cmd/zerg/check_glossary.go", "中文页出现 `forbid_zh` 词（" + "\u866b\u5de2" + "）⇒ T2 非阻塞 rc=1", "`Q-176` 第五批自证**用例标题串**（`chk(\"档② 负控：…（" + "\u866b\u5de2" + "）…\")`；与 413 同种：标题里点名被检词）"},
	{"scripts/docs/gen-glossary-exports.py", "\"" + "\u866b\u5de2" + "\", \"pod\", \"Mr2109\", \"\"]))])[1].split(\"\\t\")[0], \"pod\")", "生成器自证夹具：喂 `gen_forbid(...)` 的合成词表行（验它把 `pod` 翻出来）"},
	{"scripts/docs/gen-glossary-exports.py", "\"只有「虫茧」\" in gen_forbid([dict(zip(COLS, [\"虫茧\", \"cocoon\", \"\", \"\", \"" + "\u866b\u5de2" + "\", \"\", \"Mr2109\", \"\"]))])[0], True)", "生成器自证夹具：喂 `gen_forbid(...)` 的合成词表行（验理由串「只有「虫茧」」在）"},
}

// chzeGoHalf —— 登记条「退役态回落」表：py 真身 ⇒ 其 Go 真搬件。
// 口径 = 判「能力可用」而非「件在盘」（同族先例：check_contract_registry.go R7 退役态不过问 ·
// check_public_face_anchor「py 在盘 ⇒ 旧路；py 缺席 ∧ Go 半在盘 ⇒ 退役态」）：
// py 真身不在盘 ∧ 其 Go 真搬件在盘 ⇒ 该登记条按**退役态**处理（不过问 · ✗ 不算缺失、✗ 不算过期）；
// 两半皆缺 ⇒ 照旧 fail-closed 点「登记表指向的件不在盘」（✗ 不拆闸、✗ 不静默通过）。
var chzeGoHalf = map[string]string{
	"scripts/docs/gen-glossary-exports.py": "core/cmd/zerg/gen_glossary_exports.go",
}

// chzeRetiredGoHalfInPlace —— 退役态判定（判「能力可用」）：该登记条指向的 py 真身缺席时，
// 其 Go 真搬件是否在盘。在盘 ⇒ true（本登记条不过问）。
func chzeRetiredGoHalfInPlace(cwd, pyRel string) bool {
	goRel := chzeGoHalf[pyRel]
	if goRel == "" {
		return false
	}
	st, err := os.Stat(chzeJoin(cwd, goRel))
	return err == nil && !st.IsDir()
}

// ── 遮蔽层（heredoc / 跨行引号 / 三引号）────────────────────────────────────────────────────

// chzeQuoteCols —— py `_hd_quote_cols`：行内字符级状态机，返回「位于引号内」的**rune 列号集合**，并就
// 地推进 st（跨行持态）。规则：`'…'` 原样；`"…"` 认 `\` 转义、认 `$(`；`$(` 压栈挂起外层引号态；
// 行内注释（`#` 在行首或前面是空白）之后整行不参与状态。
func chzeQuoteCols(l string, st *chzeHdState) map[int]bool {
	rs := []rune(l)
	n := len(rs)
	cols := map[int]bool{}
	q := st.q
	stack := st.stack
	i := 0
	for i < n {
		c := rs[i]
		if q == 0 && len(stack) == 0 && c == '#' && (i == 0 || rs[i-1] == ' ' || rs[i-1] == '\t') {
			break
		}
		if q == '"' && c == '\\' {
			i += 2
			continue
		}
		if q == 0 && c == ')' && len(stack) > 0 {
			q = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		} else if (q == 0 || q == '"') && c == '$' && i+1 < n && rs[i+1] == '(' {
			stack = append(stack, q)
			q = 0
			i += 2
			continue
		} else if q == 0 {
			if c == '\'' || c == '"' {
				q = c
			}
		} else if c == q {
			q = 0
		}
		if q != 0 {
			cols[i] = true
		}
		i++
	}
	st.q = q
	st.stack = stack
	return cols
}

type chzeHdState struct {
	q     rune
	stack []rune
}

// chzeMaskHeredocsChk —— py `mask_heredocs_chk`：返回 (遮蔽后正文, 起始行号列表)。
func chzeMaskHeredocsChk(text string) (string, []int) {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	tag := ""
	hasTag := false
	starts := []int{}
	hst := &chzeHdState{q: 0}
	tagStack := []int{}
	for idx, l := range lines {
		lineno := idx + 1
		qcols := chzeQuoteCols(l, hst)
		if !hasTag {
			out = append(out, l)
			if loc := chzeREHeredocStart.FindStringSubmatchIndex(l); loc != nil {
				runestart := utf8.RuneCountInString(l[:loc[0]])
				if !qcols[runestart] {
					tag = l[loc[2]:loc[3]]
					hasTag = true
					starts = append(starts, lineno)
					tagStack = append(tagStack, lineno)
				}
			}
		} else {
			if strings.TrimSpace(l) == tag {
				out = append(out, l)
				hasTag = false
				if len(tagStack) > 0 {
					tagStack = tagStack[:len(tagStack)-1]
				}
				if len(starts) > 0 {
					starts = starts[:len(starts)-1]
				}
			} else {
				out = append(out, "")
			}
		}
	}
	return strings.Join(out, "\n"), starts
}

func chzeMaskHeredocs(text string) string {
	m, _ := chzeMaskHeredocsChk(text)
	return m
}

// chzeMaskXMLComments —— 单行清洗层原先不认 XML 注释（`<!-- … -->` 可跨行）⇒ 注释里的全角/术语
// 被当「代码区」抓成误报。本层把注释内每个非换行码点置空格、换行原样保留（行号不变），
// 使注释内容不再参与术语/全角/旧角色词判定（✗ 不加白名单了事 · 是让扫描器真不认识注释里的东西）。
func chzeMaskXMLComments(text string) string {
	var b strings.Builder
	i, n := 0, len(text)
	in := false
	for i < n {
		if !in && i+4 <= n && text[i:i+4] == "<!--" {
			in = true
			b.WriteString("    ")
			i += 4
			continue
		}
		if in {
			if i+3 <= n && text[i:i+3] == "-->" {
				in = false
				b.WriteString("   ")
				i += 3
				continue
			}
			if text[i] == '\n' {
				b.WriteByte('\n')
			} else {
				b.WriteByte(' ')
			}
			i++
			continue
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

// chzeXMLMaskIf —— 仅对可能含 XML 注释的 .plist/.conf 走 XML 注释剥离；其余 ext 原样返回。
func chzeXMLMaskIf(raw, ext string) string {
	if ext == ".plist" || ext == ".conf" {
		return chzeMaskXMLComments(raw)
	}
	return raw
}

// chzeOpenQuote —— py `_open_quote`：返回该行留下的未闭合引号字符（0 = 行内成对）。
func chzeOpenQuote(line string) rune {
	rs := []rune(line)
	var q rune
	i := 0
	for i < len(rs) {
		c := rs[i]
		if c == '\\' {
			i += 2
			continue
		}
		if q != 0 {
			if c == q {
				q = 0
			}
		} else if c == '"' || c == '\'' {
			q = c
		}
		i++
	}
	return q
}

// chzeMaskMultilineQuotesChk —— py `mask_multiline_quotes_chk`：返回 (遮蔽后正文, 未收尾起始行号列表)。
func chzeMaskMultilineQuotesChk(text string) (string, []int) {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	var quote rune
	starts := []int{}
	for idx, l := range lines {
		lineno := idx + 1
		if quote != 0 {
			rs := []rune(l)
			fi := -1
			for k, c := range rs {
				if c == quote {
					fi = k
					break
				}
			}
			if fi < 0 {
				out = append(out, "")
				continue
			}
			out = append(out, strings.Repeat(" ", fi+1)+string(rs[fi+1:]))
			quote = 0
			if len(starts) > 0 {
				starts = starts[:len(starts)-1]
			}
			continue
		}
		out = append(out, l)
		if strings.HasPrefix(strings.TrimLeftFunc(l, unicode.IsSpace), "#") {
			quote = 0
		} else {
			quote = chzeOpenQuote(l)
		}
		if quote != 0 {
			starts = append(starts, lineno)
		}
	}
	return strings.Join(out, "\n"), starts
}

func chzeMaskMultilineQuotes(text string) string {
	m, _ := chzeMaskMultilineQuotesChk(text)
	return m
}

// chzeStripBlocks —— py `strip_blocks`（RE_TRIPLE 用反向引用 ⇒ 手写最短配对扫描）。
func chzeStripBlocks(text string) string {
	var b strings.Builder
	i, n := 0, len(text)
	for i < n {
		delim := ""
		if i+3 <= n {
			if text[i:i+3] == "\"\"\"" {
				delim = "\"\"\""
			} else if text[i:i+3] == "'''" {
				delim = "'''"
			}
		}
		if delim == "" {
			b.WriteByte(text[i])
			i++
			continue
		}
		j := strings.Index(text[i+3:], delim)
		if j < 0 {
			b.WriteByte(text[i])
			i++
			continue
		}
		match := text[i : i+3+j+3]
		b.WriteString(strings.Repeat("\n", strings.Count(match, "\n")))
		i = i + 3 + j + 3
	}
	return b.String()
}

// chzePyStrLines —— 近似 py `tokenize` 的 STRING 记号行区间（对拍源 · 见未覆盖面②）。
func chzePyStrLines(text string) map[int]bool {
	out := map[int]bool{}
	rs := []rune(text)
	n := len(rs)
	line := 1
	i := 0
	const (
		stNorm = iota
		stComment
		stSingle
		stTriple
	)
	state := stNorm
	var sq rune
	var tq string
	for i < n {
		c := rs[i]
		switch state {
		case stComment:
			if c == '\n' {
				state = stNorm
				line++
			}
			i++
		case stTriple:
			out[line] = true
			if i+3 <= n && string(rs[i:i+3]) == tq {
				i += 3
				state = stNorm
				continue
			}
			if c == '\n' {
				line++
			}
			i++
		case stSingle:
			if c == '\\' {
				i += 2
				continue
			}
			if c == sq {
				state = stNorm
				i++
				continue
			}
			if c == '\n' {
				state = stNorm
				line++
				i++
				continue
			}
			out[line] = true
			i++
		default: // stNorm
			if c == '#' {
				state = stComment
				i++
				continue
			}
			if i+3 <= n && (string(rs[i:i+3]) == "\"\"\"" || string(rs[i:i+3]) == "'''") {
				tq = string(rs[i : i+3])
				out[line] = true
				state = stTriple
				i += 3
				continue
			}
			if c == '"' || c == '\'' {
				sq = c
				out[line] = true
				state = stSingle
				i++
				continue
			}
			if c == '\n' {
				line++
			}
			i++
		}
	}
	return out
}

// chzeQuoteBlindLines —— py `_py_quote_blind_lines`：列出「被正则掩成空行、而字符串行真值说不是串」
// 且**本就是命中**（代码区全角 / 禁用词）的行号。
func chzeQuoteBlindLines(text, masked string) []int {
	rawL := strings.Split(text, "\n")
	mskL := strings.Split(masked, "\n")
	blanked := []int{}
	for i := 0; i < len(rawL) && i < len(mskL); i++ {
		if strings.TrimSpace(rawL[i]) != "" && strings.TrimSpace(mskL[i]) == "" {
			blanked = append(blanked, i+1)
		}
	}
	if len(blanked) == 0 {
		return nil
	}
	strlines := chzePyStrLines(text)
	out := []int{}
	for _, l := range blanked {
		if strlines[l] {
			continue
		}
		src := rawL[l-1]
		code := chzeStripNoncode(src, ".py")
		if chzeHasFullw(code) || chzeHasForbid(src) {
			out = append(out, l)
		}
	}
	return out
}

func chzeStripBlocksChk(text string) (string, []int) {
	masked := chzeStripBlocks(text)
	return masked, chzeQuoteBlindLines(text, masked)
}

func chzeHasFullw(s string) bool {
	for _, ch := range chzeFullw {
		if strings.ContainsRune(s, ch) {
			return true
		}
	}
	return false
}

func chzeHasForbid(s string) bool {
	for _, kv := range chzeForbid {
		if strings.Contains(s, kv.k) {
			return true
		}
	}
	return false
}

// chzeCommentLine —— 整行注释判定：trim 掉行首空白后，以该 ext 的注释标记起头。
// 注释标记口径与 chzeStripNoncode 一致（`//` 通用；`#` 只对 sh/py/yaml/yml/toml/conf/plist）。
// 用于禁用词扫面：整行注释内的术语「词选择」引用属非代码区 ⇒ 不报（✗ 不落库、✗ 不加白名单）。
func chzeCommentLine(l, ext string) bool {
	t := strings.TrimLeft(l, " \t")
	if strings.HasPrefix(t, "//") {
		return true
	}
	switch ext {
	case ".sh", ".py", ".yaml", ".yml", ".toml", ".conf", ".plist":
		return strings.HasPrefix(t, "#")
	}
	return false
}

// ── 单行清洗 ─────────────────────────────────────────────────────────────────────────────

// chzeStripNoncode —— py `strip_noncode`：剥掉单行注释与单行字符串，返回「代码区」子串。
func chzeStripNoncode(line, ext string) string {
	if strings.Contains(line, "$(") {
		line = chzeRECmdSub.ReplaceAllStringFunc(line, func(m string) string {
			return strings.Repeat(" ", utf8.RuneCountInString(m))
		})
	}
	hashComment := false
	switch ext {
	case ".sh", ".py", ".yaml", ".yml", ".toml", ".conf", ".plist":
		hashComment = true
	}
	rs := []rune(line)
	n := len(rs)
	var out []rune
	var quote rune
	i := 0
	for i < n {
		c := rs[i]
		if quote != 0 {
			if c == '\\' {
				i += 2
				continue
			}
			if c == quote {
				quote = 0
			}
			i++
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			i++
			continue
		}
		if (i+1 < n && rs[i] == '/' && rs[i+1] == '/') || (hashComment && c == '#') {
			break
		}
		out = append(out, c)
		i++
	}
	return string(out)
}

func chzeBodyOf(path, ext string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		// ★ B 类真口子修（原 py `except OSError: continue` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
		chzeReadFails = append(chzeReadFails, fmt.Sprintf("%s（%v）", path, err))
		return ""
	}
	raw := string(b)
	if ext == ".py" {
		return chzeStripBlocks(raw)
	}
	if ext == ".sh" || ext == ".plist" || ext == ".conf" {
		return chzeMaskMultilineQuotes(chzeMaskHeredocs(chzeXMLMaskIf(raw, ext)))
	}
	return raw
}

// ── 豁免口 ───────────────────────────────────────────────────────────────────────────────

func chzeFpHit(p, line string) (string, bool) {
	for _, e := range chzeKnownFP {
		if e.path == p && e.sub != "" && strings.Contains(line, e.sub) {
			return e.why, true
		}
	}
	return "", false
}

// chzeRegistryStale —— py `registry_stale`：每条 `(件, 子串)` 在**它那件**里必须恰好命中 1 行。
func chzeRegistryStale() (stale []chzeFPStale, missing []string) {
	for _, e := range chzeKnownFP {
		if e.sub == "" {
			stale = append(stale, chzeFPStale{e.path, "", "登记子串为空", ""})
			continue
		}
		cwd, _ := os.Getwd()
		full := chzeJoin(cwd, e.path)
		st, err := os.Stat(full)
		if err != nil || st.IsDir() {
			// ★ 退役态（判「能力可用」· 同族先例 check_contract_registry.go R7）：py 真身不在盘 ∧
			//   其 Go 真搬件在盘 ⇒ 本登记条不过问（✗ 不算缺失 · ✗ 不算过期）；两半皆缺 ⇒ fail-closed。
			if chzeRetiredGoHalfInPlace(cwd, e.path) {
				continue
			}
			missing = append(missing, e.path)
			continue
		}
		body := chzeBodyOf(full, strings.ToLower(chzeExt(e.path)))
		hit := []int{}
		for i, ln := range strings.Split(body, "\n") {
			if strings.Contains(ln, e.sub) {
				hit = append(hit, i+1)
			}
		}
		if len(hit) == 0 {
			stale = append(stale, chzeFPStale{e.path, e.sub, "命中 0 行（该内容已不在件里）", intsToStr(hit)})
		} else if len(hit) > 1 {
			stale = append(stale, chzeFPStale{e.path, e.sub, fmt.Sprintf("命中 %d 行（子串不唯一 ⇒ 指不到一处）", len(hit)), intsToStr(hit)})
		}
	}
	return stale, missing
}

func intsToStr(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprintf("%d", x)
	}
	return strings.Join(parts, ", ")
}

func chzeExt(p string) string {
	i := strings.LastIndex(p, ".")
	if i < 0 {
		return ""
	}
	return p[i:]
}

// ── 扫描 ─────────────────────────────────────────────────────────────────────────────────

var (
	chzeMaskUnclosed    = [][2]interface{}{} // [(件, []int)]
	chzeMaskMlqUnclosed = [][2]interface{}{}
	chzeMaskPyqUnclosed = [][2]interface{}{}
	// chzeReadFails —— B 类读失败登记面（读不了的登记件 · 逐件登记 件路径 + err 正文）。
	chzeReadFails []string
)

type chzeHits struct {
	order []string
	m     map[string][]string
}

func newChzeHits() *chzeHits {
	h := &chzeHits{order: []string{"禁用词", "全角标点(代码区)", "旧角色词", "已登记假红(豁免)"}, m: map[string][]string{}}
	for _, k := range h.order {
		h.m[k] = []string{}
	}
	return h
}

func chzeFpRow(h *chzeHits, p string, i int, line, why string) {
	ent := fmt.Sprintf("%s:%d  [%s]  %s", p, i, why, chzeRuneSlice(strings.TrimSpace(line), 60))
	found := false
	for _, x := range h.m["已登记假红(豁免)"] {
		if x == ent {
			found = true
			break
		}
	}
	if !found {
		h.m["已登记假红(豁免)"] = append(h.m["已登记假红(豁免)"], ent)
	}
}

func chzeRuneSlice(s string, n int) string {
	rs := []rune(s)
	if len(rs) > n {
		rs = rs[:n]
	}
	return string(rs)
}

// chzeWalk —— ★ 保留原序读法（`f.ReadDir(-1)`），复刻 os.walk 产出序。
func chzeWalk(dir string, visit func(p, f string)) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	ents, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return
	}
	files := []string{}
	dirs := []string{}
	for _, e := range ents {
		p := dir + "/" + e.Name()
		if e.IsDir() {
			if !chzeContains(chzeSkipDirs, e.Name()) {
				dirs = append(dirs, p)
			}
		} else {
			files = append(files, p)
		}
	}
	for _, p := range files {
		visit(p, chzeBase(p))
	}
	for _, d := range dirs {
		chzeWalk(d, visit)
	}
}

func chzeBase(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return p
	}
	return p[i+1:]
}

func chzeContains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// chzeScan —— py `scan()` 的真值复刻（root = 旋钮基目录；root=="" ⇒ 逐字等于 py 的字面相对串）。
func chzeScan(root string, errw io.Writer) (int, *chzeHits) {
	files := 0
	h := newChzeHits()
	for _, r := range chzeRoots {
		base := chzeJoin(root, r)
		if !chzeIsDir(base) {
			continue
		}
		chzeWalk(base, func(realp, fname string) {
			p := r + strings.TrimPrefix(realp, base)
			ext := strings.ToLower(chzeExt(fname))
			if !chzeContains(chzeTermExt, ext) && !strings.HasSuffix(fname, ".env") {
				return
			}
			files++
			b, err := os.ReadFile(realp)
			if err != nil {
				fmt.Fprintf(errw, "  ⚠ 读不了 %s：%s\n", p, err)
				return
			}
			raw := string(b)
			var body string
			switch ext {
			case ".py":
				var blind []int
				body, blind = chzeStripBlocksChk(raw)
				if len(blind) > 0 {
					chzeMaskPyqUnclosed = append(chzeMaskPyqUnclosed, [2]interface{}{p, blind})
				}
			case ".sh", ".plist", ".conf":
				masked, unclosed := chzeMaskHeredocsChk(chzeXMLMaskIf(raw, ext))
				if len(unclosed) > 0 {
					chzeMaskUnclosed = append(chzeMaskUnclosed, [2]interface{}{p, unclosed})
				}
				var mlq []int
				body, mlq = chzeMaskMultilineQuotesChk(masked)
				if len(mlq) > 0 {
					chzeMaskMlqUnclosed = append(chzeMaskMlqUnclosed, [2]interface{}{p, mlq})
				}
			default:
				body = raw
			}
			for i, l := range strings.Split(body, "\n") {
				ln := i + 1
				why, fp := chzeFpHit(p, l)
				if !chzeWhitelist[p] {
					isComment := chzeCommentLine(l, ext)
					for _, kv := range chzeForbid {
						if strings.Contains(l, kv.k) {
							if fp {
								chzeFpRow(h, p, ln, l, why)
							} else if isComment && kv.commentOK {
								// 整行注释内的术语「词选择」引用（注释里说明术语表/负控）：非代码区 ⇒ 不报。
								// ★ 私有称谓组 commentOK=false：保密面，注释里出现仍点名。
							} else {
								h.m["禁用词"] = append(h.m["禁用词"], fmt.Sprintf("%s:%d  [%s]  %s", p, ln, kv.v, chzeRuneSlice(strings.TrimSpace(l), 70)))
							}
						}
					}
				}
				code := chzeStripNoncode(l, ext)
				if chzeContains(chzeFullwExt, ext) {
					if fp {
						chzeFpRow(h, p, ln, l, why)
					} else {
						for _, ch := range chzeFullw {
							if strings.ContainsRune(code, ch) {
								h.m["全角标点(代码区)"] = append(h.m["全角标点(代码区)"], fmt.Sprintf("%s:%d  全角「%s」  %s", p, ln, string(ch), chzeRuneSlice(strings.TrimSpace(l), 70)))
								break
							}
						}
					}
				}
				if m := chzeRERole.FindStringSubmatch(code); m != nil {
					if !chzeAnyIn(p, chzeRoleExemptHint) {
						h.m["旧角色词"] = append(h.m["旧角色词"], fmt.Sprintf("%s:%d  %s  %s", p, ln, m[1], chzeRuneSlice(strings.TrimSpace(l), 70)))
					}
				}
			}
		})
	}
	return files, h
}

func chzeAnyIn(p string, hints []string) bool {
	for _, x := range hints {
		if strings.Contains(p, x) {
			return true
		}
	}
	return false
}

// ── posix 路径小工具（逐字复刻 py 的 os.path.* on POSIX）────────────────────────────────────

func chzeJoin(a string, rest ...string) string {
	out := a
	for _, b := range rest {
		if strings.HasPrefix(b, "/") {
			out = b
			continue
		}
		if out == "" {
			out = b
			continue
		}
		if strings.HasSuffix(out, "/") {
			out = out + b
			continue
		}
		out = out + "/" + b
	}
	return out
}

func chzeIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func chzeIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ── 默认根（旋钮）────────────────────────────────────────────────────────────────────────

// chzeRootKnob —— 外部树显式入参旋钮：`--root <目录>` 优先，其次 `ZERG_ZHEN_ROOT`，缺省 ""（= py 推导）。
func chzeRootKnob(argv []string) string {
	for i := 0; i < len(argv); i++ {
		if argv[i] == "--root" {
			if i+1 < len(argv) {
				return argv[i+1]
			}
			return ""
		}
		if strings.HasPrefix(argv[i], "--root=") {
			return argv[i][len("--root="):]
		}
	}
	if v := os.Getenv("ZERG_ZHEN_ROOT"); v != "" {
		return v
	}
	return ""
}

// ── 入口 ─────────────────────────────────────────────────────────────────────────────────

// checkZhEnGateMain —— 真搬入口（M2 刻意无调用点）。
func checkZhEnGateMain(stdout, stderr io.Writer, argv []string) int {
	if has(chzeHasArg(argv, "--self-test"), chzeHasArg(argv, "--selftest")) {
		if chzeSelfTest(stdout) {
			return 0
		}
		return 1
	}
	root := chzeRootKnob(argv)
	if !chzeIsDir(chzeJoin(root, "core")) && !chzeIsDir(chzeJoin(root, "agent")) {
		fmt.Fprint(stderr, "✗ 不在仓库根目录（core/ 与 agent/ 都不存在）——拒绝静默空转\n")
		return 2
	}
	chzeMaskUnclosed = nil
	chzeMaskMlqUnclosed = nil
	chzeMaskPyqUnclosed = nil
	chzeReadFails = nil
	files, hits := chzeScan(root, stderr)
	if len(chzeMaskUnclosed) > 0 {
		fmt.Fprint(stderr, "✗ 遮蔽面有未闭合的 heredoc 段 —— 门在这些行上整段失明，本轮**不给结论**\n")
		for k, pf := range chzeMaskUnclosed {
			if k >= 10 {
				break
			}
			fmt.Fprintf(stderr, "   · %s：起始行 %s（其后直到文件末皆被遮蔽）\n", pf[0], intsToStr(chzeHeadInts(pf[1].([]int), 5)))
		}
		fmt.Fprint(stderr, "   ★ 处置：收尾标记必须**独占一行**（现读 `l.strip()==TAG`）；起始标记出现在引号内时不算真 heredoc（根因档待办）。\n")
		return 2
	}
	if len(chzeMaskMlqUnclosed) > 0 {
		fmt.Fprint(stderr, "⛔ 不给结论（rc=2）：跨行引号层有未收尾的段 —— 门在这些行上整段失明，本轮**判不了**\n")
		for k, pf := range chzeMaskMlqUnclosed {
			if k >= 10 {
				break
			}
			fmt.Fprintf(stderr, "   · %s：起始行 %s（该行起的引号在本件内无收尾 ⇒ 其后直到文件末皆被当字符串掩掉）\n", pf[0], intsToStr(chzeHeadInts(pf[1].([]int), 5)))
		}
		fmt.Fprint(stderr, "   ★ 处置：跨行引号必须有配对收尾（现读 `l.find(quote)`）；引号起于命令头之后而本件内无收尾时，本层判不了（不把「掩掉后零命中」当绿）。\n")
		return 2
	}
	if len(chzeMaskPyqUnclosed) > 0 {
		fmt.Fprint(stderr, "⛔ 不给结论（rc=2）：三引号层有配不齐的段 —— 门在这些行上失明，本轮**判不了**\n")
		for k, pf := range chzeMaskPyqUnclosed {
			if k >= 10 {
				break
			}
			fmt.Fprintf(stderr, "   · %s：被掩行 %s（对拍源 = %s；该段里正则掩掉的是真代码）\n", pf[0], intsToStr(chzeHeadInts(pf[1].([]int), 5)), chzeMaskPyqJudge)
		}
		fmt.Fprint(stderr, "   ★ 处置：件里 \"\"\" / ''' 必须成对，且不得出现在注释或普通字符串里（现读 RE_TRIPLE 就近配对）；配不齐时本层判不了（不把「掩掉后零命中」当绿）。\n")
		return 2
	}
	if files == 0 {
		fmt.Fprint(stderr, "✗ 扫到 0 个文件——拒绝静默空转（假绿）\n")
		return 2
	}
	total := len(hits.m["禁用词"]) + len(hits.m["全角标点(代码区)"]) + len(hits.m["旧角色词"])
	fmt.Fprintf(stdout, "扫描文件数 = %d（全角标点只扫无编译器兜底的文件；.go 仅扫术语/旧角色词）\n", files)
	for _, k := range hits.order {
		v := hits.m[k]
		fmt.Fprintf(stdout, "\n── %s：%d 处\n", k, len(v))
		for i, x := range v {
			if i >= 15 {
				break
			}
			fmt.Fprintf(stdout, "    %s\n", x)
		}
		if len(v) > 15 {
			fmt.Fprintf(stdout, "    …另 %d 处\n", len(v)-15)
		}
	}
	stale, missing := chzeRegistryStale()
	if len(chzeReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(chzeReadFails))
		for _, u := range chzeReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return 2
	}
	if len(missing) > 0 {
		seen := map[string]bool{}
		uniq := []string{}
		for _, m := range missing {
			if !seen[m] {
				seen[m] = true
				uniq = append(uniq, m)
			}
		}
		sort.Strings(uniq)
		for _, m := range uniq {
			fmt.Fprintf(stderr, "✗ 登记表指向的件不在盘：%s —— 判不了「登记过期」，拒绝静默放过\n", m)
		}
		return 2
	}
	fmt.Fprintf(stdout, "\n── 登记过期（键失效 ⇒ 必报）：%d 条\n", len(stale))
	for _, s := range stale {
		extra := ""
		if s.hit != "" {
			extra = "（命中行 " + s.hit + "）"
		}
		fmt.Fprintf(stdout, "   · %s  «%s» ⇒ %s%s\n", s.path, chzeRuneSlice(s.sub, 70), s.why, extra)
	}
	if len(stale) > 0 {
		fmt.Fprint(stdout, "   ★ 处置：登记过期 ⇒ **修登记**（换一段逐字片段）或**改件**——不得当作豁免成功\n")
	}
	fmt.Fprintf(stdout, "\n合计命中 = %d\n", total)
	if total == 0 && len(stale) == 0 {
		return 0
	}
	return 1
}

const chzeMaskPyqJudge = "python-tokenize（Python 自己的词法器）"

func chzeHasArg(argv []string, x string) bool {
	for _, a := range argv {
		if a == x {
			return true
		}
	}
	return false
}

func has(a, b bool) bool { return a || b }

func chzeHeadInts(xs []int, n int) []int {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

// ══ 自证（py selftest / selftest_fp · 输出逐字复刻）════════════════════════════════════════════

func chzePyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func chzeMark(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func chzePyList(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprintf("%d", x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func chzeSelfTest(stdout io.Writer) bool {
	chzeReadFails = nil // ★ 自清：包级登记面不许跨调用残留（残留会让下一次一进门就被 BLOCKED）
	ok := true
	type sc struct {
		ext, line string
		want      bool
	}
	cases := []sc{
		{".sh", "x=1  # 注释里的全角逗号，不该抓", false},
		{".sh", "echo '字符串里的全角逗号，不该抓'", false},
		{".sh", "if [ $a = 1，]; then", true},
		{".sh", "say \"x: $( [ \"$A\" = \"1\" ] && echo '不触碰（a）')\"", false},
		{".yaml", "cmd: run  --flag，x", true},
		{".yaml", "# 注释，不抓", false},
		{".py", "s = '，'  # 字符串，不抓", false},
		{".py", "x = 1，2", true},
		{".go", "if s == \"，\" {", false},
		{".go", "// 注释里的（全角）不抓", false},
		{".json", "{\"a\": \"，\"}", false},
		{".json", "{\"a\": 1，}", true},
		{".toml", "k = '，'", false},
		{".sh", "echo x='，'", false},
	}
	if len(cases) != 14 {
		fmt.Fprintf(stdout, "  ✗ 自证用例数必须为 14，实际 %d（少跑即失效）\n", len(cases))
		return false
	}
	for _, c := range cases {
		got := chzeHasFullw(chzeStripNoncode(c.line, c.ext))
		good := got == c.want
		ok = ok && good
		fmt.Fprintf(stdout, "  %s [%s] 期望=%s 实际=%s  « %s\n", chzeMark(good), c.ext, chzePyBool(c.want), chzePyBool(got), chzeRuneSlice(c.line, 56))
	}
	q3 := "\"\"\""
	py := q3 + "\n模块说明（含全角括号）\n" + q3 + "\nx = 1，2\ny = \"，\"\n"
	pl := strings.Split(chzeStripBlocks(py), "\n")
	d := false
	for _, l := range pl[:2] {
		if chzeHasFullw(chzeStripNoncode(l, ".py")) {
			d = true
		}
	}
	c := false
	for _, l := range pl[3:4] {
		if chzeHasFullw(chzeStripNoncode(l, ".py")) {
			c = true
		}
	}
	ok = ok && (!d) && c
	fmt.Fprintf(stdout, "  %s [三引号] 段落不抓=%s（期望 False）· 段外真代码抓=%s（期望 True）\n", chzeMark(!d && c), chzePyBool(d), chzePyBool(c))

	hd := "cat <<'EOF' > x\n不触碰（body）\nEOF\necho ，真代码\n"
	hl := strings.Split(chzeMaskHeredocs(hd), "\n")
	hb := false
	for _, l := range hl[1:2] {
		if chzeHasFullw(chzeStripNoncode(l, ".sh")) {
			hb = true
		}
	}
	hc := false
	for _, l := range hl[3:] {
		if chzeHasFullw(chzeStripNoncode(l, ".sh")) {
			hc = true
		}
	}
	ok = ok && (!hb) && hc
	fmt.Fprintf(stdout, "  %s [heredoc] 正文不抓=%s（期望 False）· 段外真代码抓=%s（期望 True）\n", chzeMark(!hb && hc), chzePyBool(hb), chzePyBool(hc))

	type hc2 struct {
		name, src string
		nWant     int
		mWant     int
	}
	hcases := []hc2{
		{"C1 单引号内", "printf '%s\\n' '…<<PYEOF…'\n", 0, 0},
		{"C2 双引号内(跨行串)", "printf \"…<<'PYEOF'\\n%s\\nPYEOF\" \"$1\" \"$X\"\n", 0, 0},
		{"C3 add_step 串内", "add_step pub \"python3 - <<'PYEOF'\"\n", 0, 0},
		{"C4 引号内起始不得吞其后真起始（被掩 1；修前掩到末）", "printf '%s' '<<PYEOF'\nx=$(cat <<'EOF'\nbody\nEOF\n", 0, 1},
		{"C5 双引号内 $( 里是真起始（防误杀锁·被掩 1）", "FAST_NAMES=\"$(cat <<'EOF'\nbody\nEOF\n)\"\n", 0, 1},
		{"C6 真起始体内含括号（被掩 1）", "$(python3 - \"$PLIST\" <<'PY'\nprint(1)\nPY\n", 0, 1},
	}
	for _, hc := range hcases {
		ht, hst := chzeMaskHeredocsChk(hc.src)
		ha := strings.Split(hc.src, "\n")
		hbl := strings.Split(ht, "\n")
		hm := 0
		for k := 0; k < len(ha) && k < len(hbl); k++ {
			if hbl[k] == "" && strings.TrimSpace(ha[k]) != "" {
				hm++
			}
		}
		hgood := len(hst) == hc.nWant && hm == hc.mWant
		ok = ok && hgood
		fmt.Fprintf(stdout, "  %s [%s] 起始=%d（期望 %d）· 被掩=%d（期望 %d）\n", chzeMark(hgood), hc.name, len(hst), hc.nWant, hm, hc.mWant)
	}

	mq := strings.Join([]string{"git commit -q -m \"第一行（全角）", "第二行，仍属字符串", "第三行\"", "echo ，真代码", ""}, "\n")
	ml := strings.Split(chzeMaskMultilineQuotes(mq), "\n")
	mb := false
	for _, l := range ml[1:3] {
		if chzeHasFullw(chzeStripNoncode(l, ".sh")) {
			mb = true
		}
	}
	mc := false
	for _, l := range ml[3:] {
		if chzeHasFullw(chzeStripNoncode(l, ".sh")) {
			mc = true
		}
	}
	ok = ok && (!mb) && mc
	fmt.Fprintf(stdout, "  %s [跨行引号] 串内不抓=%s（期望 False）· 串外真代码抓=%s（期望 True）\n", chzeMark(!mb && mc), chzePyBool(mb), chzePyBool(mc))

	cm := "# 批 2'.6 注释里带撇号（全角括号）" + "\n" + "echo ，真代码" + "\n"
	cl := strings.Split(chzeMaskMultilineQuotes(cm), "\n")
	cb := false
	for _, l := range cl[:1] {
		if chzeHasFullw(chzeStripNoncode(l, ".sh")) {
			cb = true
		}
	}
	cc := false
	for _, l := range cl[1:] {
		if chzeHasFullw(chzeStripNoncode(l, ".sh")) {
			cc = true
		}
	}
	ok = ok && (!cb) && cc
	fmt.Fprintf(stdout, "  %s [注释撇号] 注释内不抓=%s（期望 False）· 下一行真代码抓=%s（期望 True）\n", chzeMark(!cb && cc), chzePyBool(cb), chzePyBool(cc))

	ok = ok && chzeSelfTestFP(stdout)
	fmt.Fprintf(stdout, "  自证：%s（14 条单行 + 4 条跨行结构 + 6 条遮蔽起始族 + 3 条豁免口）\n", chzeVerdict(ok))
	return ok
}

func chzeVerdict(b bool) string {
	if b {
		return "通过 ✓"
	}
	return "**失败** ✗"
}

// chzeSelfTestFP —— py `selftest_fp`：豁免口正/负控 + 登记过期负控 + 遮蔽未闭合成对。
func chzeSelfTestFP(stdout io.Writer) bool {
	savedRoots := chzeRoots
	savedWL := chzeWhitelist
	savedFP := chzeKnownFP
	savedCwd, _ := os.Getwd()
	ok := true
	defer func() {
		chzeRoots = savedRoots
		chzeWhitelist = savedWL
		chzeKnownFP = savedFP
		os.Chdir(savedCwd)
	}()
	tmp, err := os.MkdirTemp("", "zh-en-fp-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "  ✗ [豁免口] 夹具目录建不了：%s\n", err)
		return false
	}
	defer os.RemoveAll(tmp) // py 不回收；本件回收（输出不可见差异）
	if err := os.MkdirAll(chzeJoin(tmp, "core"), 0o755); err != nil {
		return false
	}
	probe := chzeJoin(tmp, "core", "probe_fp.py")
	content := "a = \"" + "\u866b\u5de2" + "\"\n" +
		"b = \"" + "\u866b\u5de2" + "\"\n" +
		"c = 1，2  # " + "\u866b\u5de2" + " 同行两种命中\n"
	if err := os.WriteFile(probe, []byte(content), 0o644); err != nil {
		return false
	}
	if err := os.Chdir(tmp); err != nil {
		return false
	}
	chzeRoots = []string{"core"}
	chzeWhitelist = map[string]bool{}
	chzeKnownFP = []chzeFPDef{
		{"core/probe_fp.py", "a = \"" + "\u866b\u5de2" + "\"", "自证登记行①"},
		{"core/probe_fp.py", "c = 1，2  # " + "\u866b\u5de2" + " 同行两种命中", "自证登记行③（同行两种命中 ⇒ 只许列一次）"},
	}

	_, h := chzeScan("", stdout)
	gotFP, gotFB, gotFW := len(h.m["已登记假红(豁免)"]), len(h.m["禁用词"]), len(h.m["全角标点(代码区)"])
	c1 := gotFP == 2 && gotFB == 1 && gotFW == 0
	fmt.Fprintf(stdout, "  %s [豁免口·正控] 登记 2 行（第 3 行同行两种命中）⇒ 豁免=%d（期望 2·去重生效）· 禁用词=%d（期望 1，只剩未登记的第 2 行）· 全角=%d（期望 0）\n", chzeMark(c1), gotFP, gotFB, gotFW)
	ok = ok && c1

	chzeKnownFP = []chzeFPDef{}
	_, h2 := chzeScan("", stdout)
	dFP, dFB, dFW := len(h2.m["已登记假红(豁免)"]), len(h2.m["禁用词"]), len(h2.m["全角标点(代码区)"])
	c2 := dFP == 0 && dFB == 3 && dFW == 1
	chzeKnownFP = []chzeFPDef{{"core/probe_fp.py", "a = \"压根不在件里的串\"", "自证登记行①"}}
	st0, ms0 := chzeRegistryStale()
	c3 := len(st0) == 1 && len(ms0) == 0 && st0[0].path == "core/probe_fp.py" &&
		st0[0].sub == "a = \"压根不在件里的串\"" && strings.Contains(st0[0].why, "命中 0 行")
	chzeKnownFP = []chzeFPDef{{"core/probe_fp.py", "\u866b\u5de2", "自证登记行①"}}
	st1, _ := chzeRegistryStale()
	c4 := len(st1) == 1 && strings.Contains(st1[0].why, "命中 3 行")
	fmt.Fprintf(stdout, "  %s [豁免口·负控] 清空登记 ⇒ 豁免=%d（期望 0）· 禁用词=%d（期望 3）· 全角=%d（期望 1）—— 上面那两处是**登记表**挡的，不是判据失灵；登记过期：子串失配 %d 条（期望 1 · 点名）· 多处命中 %d 条（期望 1 · 点名）\n", chzeMark(c2 && c3 && c4), dFP, dFB, dFW, len(st0), len(st1))
	ok = ok && c2 && c3 && c4

	q3 := "\"\"\""
	m1, u1 := chzeMaskHeredocsChk("echo hi <<PYEOF\nnot closed\n")
	m2, u2 := chzeMaskHeredocsChk("cat <<PYEOF\nbody\nPYEOF\ntail\n")
	c5 := len(u1) == 1 && u1[0] == 1 && len(u2) == 0 && !strings.Contains(m1, "not closed") && strings.Contains(m2, "tail")
	x1, xu1 := chzeMaskMultilineQuotesChk("PY_CMD=\"python3 - '''\nbody without closer\n")
	x2, xu2 := chzeMaskMultilineQuotesChk("git commit -q -m \"第一行\n第二行\"\ntail\n")
	c6 := len(xu1) == 1 && xu1[0] == 1 && len(xu2) == 0 && !strings.Contains(x1, "body without closer") && strings.Contains(x2, "tail")
	y1, yu1 := chzeStripBlocksChk("x = 1\n# 说明：用 " + q3 + " 包块\ny = 2，3\n" + q3 + "\n中文，文档\n" + q3 + "\n")
	_ = y1
	y2, yu2 := chzeStripBlocksChk("x = 1\n" + q3 + "\n中文，文档\n" + q3 + "\ny = 2，3\n")
	_ = y2
	c7 := len(yu1) == 1 && yu1[0] == 3 && len(yu2) == 0
	fmt.Fprintf(stdout, "  %s [遮蔽未闭合] 未闭合起始行=%s（期望 [1]）· 闭合件=%s（期望 []）· 闭合件尾行保留=%s ｜ 跨行引号层：未收尾起始行=%s（期望 [1]）· 收尾件=%s（期望 []）· 尾行保留=%s ｜ 三引号层：配不齐被掩行=%s（期望 [3]）· 成对件=%s（期望 []）\n",
		chzeMark(c5 && c6 && c7), chzePyList(u1), chzePyList(u2), chzePyBool(strings.Contains(m2, "tail")),
		chzePyList(xu1), chzePyList(xu2), chzePyBool(strings.Contains(x2, "tail")),
		chzePyList(yu1), chzePyList(yu2))
	ok = ok && c5 && c6 && c7
	return ok
}
