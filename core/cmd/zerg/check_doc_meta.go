// check_doc_meta.go —— 判据件 `scripts/gates/check-doc-meta.py`（974 行）的 **Go 真搬**
// （v2.5.14 · M2 · 新口径靶：文仓/外部树依赖类 · 入口 `checkDocMetaGateMain`）。
//
// 本件 = 文档 frontmatter 元数据门（6 件套必填 + 值域封闭 + 额外字段禁入）的真搬。验收判据 =
// 同 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码 **三面逐字相同**
// （除下列**显式声明**的新口径差异面与未覆盖面）。
//
// 本件与新口径（外部树走显式入参旋钮）—— 逐条对应：
//
//	① 旋钮名**照 py 已有同名旗标/env**：py 本就用 `--repo-root` / `--schema` / `--target` /
//	   `--scope`（四条旗标）+ env `ZERG_DEVDOCS_ROOT` / `ZERG_DOCS_ALT` / `ZERG_DOCS`
//	   （`devdocs_root()` 的三级取根）⇒ 本件**同名同序照搬、不另起名**（✗ 不造互不认识的新名）。
//	② 缺省值 = py 默认推导**逐字同构**：
//	   `ZERG_DEVDOCS_ROOT` > (`ZERG_DOCS_ALT` or `ZERG_DOCS` or `pyJoin(pyDir(abs(repo_root)), "Zerg-内部文档")`)
//	   > 再 `pyJoin(sibling, "项目文档", "v2.5.10")`；`--schema` 缺省 = `pyJoin(repo, "docs/site/frontmatter-schema.json")`。
//	③ ★ 退 2 边界：py 的**四条**「取不到 ⇒ 不给结论」分支（devdocs 根不在 · 目标不存在 ·
//	   formal 候选目录一个都不存在 · 扫描域为空）**逐字照搬**退 2 —— 这是「这一格取不到」，
//	   不是「任一外部树取不到就退 2」。本件**未**把「任一取不到」扩成红。
//	④ ★ py 该档本来就退 1/0 的（M1 无 frontmatter 块 = 默认告警 rc=0 / `--missing=fail` 升红 rc=1；
//	   M14/M15 = BLOCK rc=2）**逐字沿用**，✗ 不抬档。**未覆盖面⑦** 列出 py 最外层无兜底的那一格。
//	⑤ ★★ **`..` 段**：`--scope formal` 的仓外候选串 `../Zerg-内部文档/项目文档/现行/` 与打印串
//	   `pyJoin(root, alt)` **含 `..` 段** ⇒ 本件一律走自实现 `cdmPyJoin`（posixpath.join 语义，
//	   逐字保留 `..`），✗ 不用 `filepath.Join`（它会把 `a/../b` 归一成 `b`）。无 `..` 段的
//	   缺省串同样走 `cdmPyJoin`，两态逐字同。`FROZEN_PREFIX_ALT = "../Zerg-内部文档/项目文档/"` 正是
//	   为这些 `..` 形态的 rel 而存在。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · `add_step` **未挂**（`precommit-gates.sh` 现读零命中））；
//	· **不**删 `scripts/gates/check-doc-meta.py` 原件（退场规则：**对拍通过后才**退场）；
//	· **已接线**：入口 `checkDocMetaGateMain` 由 `gateCheckGoEntry["check-doc-meta"]` 分派（`zerg gate check check-doc-meta`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// ★★ 目录遍历序（本批第一标准）—— 逐点交代（原序 / 显式排序 / 理由）：
//
//	py `walk_docs()` 的遍历序 = `os.walk` 的 **readdir 原序**（`os.scandir` 不排序）；
//	Go `os.ReadDir`/`filepath.WalkDir`/`Walk` **会排序** ⇒ ✗ 差排 = 整批假失配。
//	本件一律用 **`f.ReadDir(-1)`（保原序读法）**，逐点：
//	  ⓐ `cdmWalkDocs` 顶层 `for t in targets`：py 按**入参顺序**逐个 target 走（`--target` append 序
//	     在前、位置参数在后：`given = list(args.targets) + list(args.target)`）⇒ 本件同序照搬（**原序**）。
//	  ⓑ 每层目录：`os.walk` 产出的 `dns`（子目录名）= **readdir 原序**；py 只做**过滤**不做排序
//	     （`dns[:] = [d for d in dns if d not in EXCL_DIRS and not d.startswith(".")]`，filter 保序）
//	     ⇒ 本件用 `f.ReadDir(-1)` 拿原序、过滤后**按原序**递归（**原序**）。理由：这一步 py 没有 sorted()。
//	  ⓒ 每层目录的**文件**：py 明确 `for f in sorted(fns)` ⇒ 这里**必须显式排序**
//	     （`cdmSortedNames`，Go `sort.Strings` 的 UTF-8 字节序 == py 的 code point 序）。
//	     这是「py 代码里显式 sorted() 的序」那一类，**不是**遍历序。
//	  ⓓ ★ `out.sort()`（py :235）：整个结果表最后再**全局排序**一次 `(rel, abs)` 元组。
//	     rel 逐件唯一 ⇒ 等价于按 rel 排序 ⇒ **上述 ⓐ–ⓒ 的序在任何可观察输出上都被这一条抹平**
//	     （输出的行序 = 全局 rel 序）。本件照搬：`cdmWalkDocs` 末尾对结果做一次显式全局排序。
//	     理由：ⓐ–ⓒ 保原序是为了**逐字同构 py 的控制流**（dup 件、`--target` 重复给的件也要按同一
//	     位置进入表），ⓓ 才是决定 stdout 行序的那一条；两者都不许用「WalkDir 的默认排序」顶替。
//	  ⓔ `sorted(set(keep))`（`--scope formal` 的 targets，py :929）⇒ **显式排序**（+ 去重），照搬。
//	  ⓕ `sorted(extra)`（M6 未申报字段，py :448）⇒ **显式排序**，照搬。
//	  ⓖ `sorted(k for k in used ...)`（schema 未知关键字，py :490）⇒ **显式排序**，照搬。
//	  ⓗ `sorted(rep["skipped_cls"].items())` / `sorted(RULE_NOTE.items())` /
//	     `sorted(rep["per_rule"].items())`（py :616/:617/:962）⇒ **显式排序**，照搬（规则号按
//	     **串**序：M1 < M10 < M16 < M2 …，实测 py 打的就是这个序）。
//	  ⓘ `TYPE_DEFAULT_EXACT` / `FORMAL_PREFIX` / `TYPE_DEFAULT_UNSURE` / `RULES` / `DOC_EXT`
//	     在 py 里是 **dict/tuple 的插入序**（`for pre, t in TYPE_DEFAULT_EXACT.items()` 与
//	     `for pre, alt in FORMAL_PREFIX`）⇒ 本件用**切片对**照搬插入序，✗ 不用 Go map
//	     （map 迭代序随机 ⇒ 假失配）。
//	  ⓙ `_inline_list` / `parse_frontmatter` 的顺序 = **源文件行序 / 串内出现序**（**原序**），照搬。
//	  ⓛ `resolve` 的 `hashlib` 遍历（自检 ⑨ `tree_sha`）：py `os.walk` + `dns.sort()` + `sorted(fns)`
//	     ⇒ 这里 py **自己排了序**（`dns.sort()` 是显式排序）⇒ 本件同序处理；且该函数只产出**一个
//	     sha256 用于相等断言**，序不影响判据。
//
// ★★ 正则语义差（py `re` ⟷ Go RE2 · 逐条写明并逐条对齐）：
//
//	① py `\s`/`\S`/`\w`/`\W`/`\d`/`\D` 是 **Unicode 语义**，Go RE2 默认 **ASCII**。
//	   本件**逐处手写对齐**：
//	     · `\s` ⇒ `cdmWSClass`（`[\t\n\v\f\r \x{0085}\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}
//	       \x{202f}\x{205f}\x{3000}\x{001c}-\x{001f}]` —— 含 py `_PyUnicode_IsWhitespace` 认的
//	       `\x1c-\x1f` 与 `\x85`）。**涉及 3 处**：`_scalar` 的行内注释剥离、`parse_frontmatter`
//	       的 `^\s+\t` 制表符缩进判、`parse_frontmatter` 的 `^(\s*)- (.*)$` 列表项判。
//	       **未覆盖**：py `str.strip()`（`.strip()`/`.lstrip()`）用的是 `str.isspace()`，本件用
//	       `strings.TrimSpace`/`unicode.IsSpace` ⇒ 极少数码点（如 U+001C–U+001F 之外的自定义空白）
//	       可能不同 ⇒ 见未覆盖面②。
//	     · `\d` ⇒ `\p{Nd}`（**2 处**：schema 读出的 `pattern` 经 `cdmRE2FromPy` 转译；`default_type_for`
//	       的 `^\d{2}-` 手写为 `^\p{Nd}\p{Nd}-`）。**连带**：`_valid_date` 的 `^(\d{4})-(\d{2})-(\d{2})$`
//	       也改 `\p{Nd}`，并且 `int(...)` 那一层用 `cdmNdValue()` 把 Nd 码点折成数值（py `int()` 认
//	       Nd 数字）—— 见未覆盖面③。
//	     · `\w` ⇒ `[\p{L}\p{N}_]`（**1 处**：`cdmRE2FromPy` 的通用转译；本 schema 未用 `\w`）。
//	     · `\b` ⇒ **无法逐字对齐**（py `\b` 是 Unicode 词边界，RE2 的 `\b` 是 ASCII 词边界）⇒
//	       本 schema 未用 `\b`；通用转译里**原样保留**并在未覆盖面④点名。
//	② ★ py 非多行 `$` = 「串尾**或**串尾换行之前」，Go RE2 的 `$`（无 `(?m)`）只匹配**串尾**。
//	   本件 `cdmRE2FromPy` 把裸 `$` 译成 `\z`（绝对串尾）。**理由 + 边界**：本门喂给 pattern 的
//	   `value` 一律来自 `_scalar()`/`_inline_list()` 的**单行**取值（`rest = m.group(2).strip()`
//	   或列表项），**不含 `\n`** ⇒ 「串尾换行之前」那一支**永远走不到** ⇒ `\z` 与 py `$` 在本门的
//	   数据流上**逐字等价**。⚠ 若日后 schema 的 pattern 被喂进含尾换行的串，两者会分叉 ⇒ 未覆盖面⑤。
//	③ py `(?!)` 负向先行等：本 schema **零用量**；RE2 无 lookaround ⇒ 若 schema 日后引入，
//	   `regexp.Compile` 会失败 ⇒ 本件按 **rc=2 不给结论**处理（fail-closed），✗ 不静默放行。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：
//	    py `HERE = os.path.dirname(os.path.abspath(__file__))` ·
//	       `REPO_ROOT_DEFAULT = os.path.dirname(os.path.dirname(HERE))`
//	       ⇒ 默认仓根起于**脚本所在目录**（与 cwd 无关；脚本在 `<仓根>/scripts/gates/`）。
//	    Go **无 `__file__` 语境** ⇒ `cdmDefaultRepo()` 从 **cwd** 上溯 ≤8 层找含
//	       `core/cmd/zerg/main.go` 的目录，找不到回落 `cdmAbs(cwd)`。
//	    逐字对比（缺省档）：
//	      py  `repo = os.path.abspath(REPO_ROOT_DEFAULT)`（脚本位置上溯两层）
//	      Go  `cdmDefaultRepo()`（cwd 上溯找 `core/cmd/zerg/main.go`；回落 abs(cwd)）
//	      py  `devdocs_root(repo) = abs($ZERG_DEVDOCS_ROOT or pyJoin($ZERG_DOCS_ALT or $ZERG_DOCS
//	           or pyJoin(pyDir(abs(repo)), "Zerg-内部文档"), "项目文档", "v2.5.10"))`
//	      Go  `cdmDevdocsRoot(repo)`：**同名同序逐字**（空串 env 视同未设，同 py 的 `or` 短路）
//	      py  `schema = abs($--schema) if 给了 else pyJoin(repo, "docs/site/frontmatter-schema.json")`
//	      Go  `cdmSchemaPath()`：**逐字同构**
//	    「脚本位置 == cwd 上溯根」时两者读数**逐字相同**（对拍甲环境已证）；否则**路径推导面不同**
//	    ⇒ 不属逐字对齐承诺。**未覆盖面⑥** 列出「脚本位置 ≠ cwd 上溯根」时**具体的**分叉格。
//	(b) ★ **自指检查（先真判 · 四形态逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否**。本门扫描面 = 一棵根下的 **`.md`/`.markdown`/`.mdx`** 件
//	       （`walk_docs` 收件时 `f.lower().endswith(DOC_EXT)` 过滤）。本件是 `.go`、落
//	       `core/cmd/zerg/`。**量化依据**：`cdmWalkDocs` 内 `cdmHasDocExt(f)` 过滤 **1 处**；
//	       全件零 `.md` 路径字面量；本件对自身的扫描面贡献 **0 篇**。
//	    ② **本件落在任何他门扫描面内吗 ⇒ 是**（`check-hardcoded-private-paths.py` 的 SCAN_DIRS 含
//	       `core/`；`check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`；`check-wall-layer` 扫茧壁标记）
//	       ⇒ 按纪律处置：
//	       · 正文**零私有绝对路径字面量**（盘根绝对路径、家目录前缀一概不写：仓根/外部树一律运行时取）；
//	       · **不写** `check-error-kinds` 的被扫错误构造调用原串（本件用 `fmt.Errorf`/自定义错误值，
//	         不写该门点名的那一族调用形态）；
//	       · **不写**反引号包裹的门扫描标记原串（`cdmWSClass` 里的空白类以**转义形态** `\x{…}` 落地，
//	         ✗ 不落真空白原串；文件头里的 `docs/项目文档/` 系 py 正文逐字照搬所必需，已实证不触任何门）；
//	       · self-test 夹具里的魔法串（`cdmBlkMarker` 一族）只在**运行时建在 TMPDIR 下**的夹具件正文里
//	         （走 `cdmJoin` 拼串），✗ 不落本件正文的连续被扫形态。
//	    ③ **证明处理有效**：造 `/var/tmp/cdm-shadow`（全符号链接同构真仓）⇒ 让本件在/不在两态各跑
//	       他门一次，两态输出 **sha16 逐字相同**（读数见交付回执）。
//	    ④ ★ **本件是否落在「任何他门」的扫描面内**已按②逐一判（见「处理有效」读数），✗ 不只判「自己这一门」。

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Mr2109/zerg-swarm/core/internal/chat"
)

// ── 常量表（逐字照搬 py 的插入序；✗ 不用 Go map 保序）───────────────────────

const cdmProg = "check-doc-meta.py"

// cdmSchemaDefault —— py `SCHEMA_DEFAULT`（相对仓根）。
const cdmSchemaDefault = "docs/site/frontmatter-schema.json"

// cdmDocExt —— py `DOC_EXT`（元组序照搬）。
var cdmDocExt = []string{".md", ".markdown", ".mdx"}

// cdmExclDirs —— py `EXCL_DIRS`（逐字 · 元组序照搬）。
var cdmExclDirs = []string{"node_modules", "target", "__pycache__", ".history", ".obsidian", "dist",
	".zerg", "zerg-wt", "venv", ".cargo", ".git"}

// cdmExclPrefix —— py `EXCL_PREFIX`。
var cdmExclPrefix = []string{".git/", "vendor/", "tools/ocr/venv/"}

// cdmFrozenPrefix —— 仓内冻结区前缀。★ 真源在 `doc_zone_source.go`（GAP-20260926-35）：
// 本门不再自带一份，否则会与 check-doc-name 各判一头（同一件两门结论相反）。
var cdmFrozenPrefix = docFrozenRepoPrefix

// cdmFrozenPrefixAlt —— 仓外（Zerg-内部文档）侧冻结区前缀 · 含 `..` 段。★ 同源。
var cdmFrozenPrefixAlt = docFrozenAltPrefix

// cdmFormalItem —— py `FORMAL_PREFIX` 的一项（in = 仓内前缀 · alt = 仓外候选，空串 ⇒ None）。
type cdmFormalItem struct {
	in  string
	alt string
}

// cdmStableLanding —— **去版本号稳定落点**名（= `chat.StableLandingName`：`现行` · 一处真源，
// ✗ 不在本包另写字面量；与判据 `check-doc-consumers` / 只读命令 `zerg doc consumers` 同法）。
var cdmStableLanding = chat.StableLandingName

// cdmFormalPrefix —— py `FORMAL_PREFIX`（元组序照搬；✗ 不用 map）。
// 落点面（缺口 GAP-20260927-121 · 2026-10-15）：首项原钉版本目录字面量（v2.5.10 那一版）⇒ 改指活版落点：
// 稳定面「唯一份」= `<项目文档根>/现行/`（版本只当元数据 · 落点路径不随版本变）；仓内前缀 / 仓外候选
// 各由 `<项目文档根>` ＋ `chat.StableLandingName` 拼出（基础字面量与 `cdmFrozenPrefix` /
// `cdmFrozenPrefixAlt` 同）⇒ ✗ 不硬编版本字面量、✗ 不随当前版号改字
// （判法照判据 `check-doc-consumers` 的「稳定落点」，见其 `dcConforms` / `dcStableLandingName`）。
var cdmFormalPrefix = []cdmFormalItem{
	{"docs/项目文档/" + cdmStableLanding + "/", "../Zerg-内部文档/项目文档/" + cdmStableLanding + "/"},
	{"docs/常青/", ""},
	{"docs/skills/", ""},
	{"docs/zh/", ""},
	{"docs/en/", ""},
}

// cdmSkillContractKeys —— py `SKILL_CONTRACT_KEYS`（另一套 frontmatter 契约 ⇒ 只剔除不计不合规）。
var cdmSkillContractKeys = []string{"name", "description"}

// cdmTypeDefaultPair —— py `TYPE_DEFAULT_EXACT` 的一项（**插入序**即判据序）。
type cdmTypeDefaultPair struct{ pre, typ string }

// cdmTypeDefaultExact —— py `TYPE_DEFAULT_EXACT`（dict 插入序照搬）。
var cdmTypeDefaultExact = []cdmTypeDefaultPair{
	{"docs/skills/", "how-to"},
	{"docs/01-设计/", "explanation"},
}

// cdmTypeDefaultUnsure —— py `TYPE_DEFAULT_UNSURE`（元组序照搬 · 本脚本不判）。
var cdmTypeDefaultUnsure = []string{"docs/调研/", "docs/02-调研/", "docs/03-评审/", "docs/thunderbolt/",
	"docs/常青/"}

// cdmRule —— py `RULES` 的一项。
type cdmRule struct{ id, sev, what, src string }

// cdmRules —— py `RULES`（表序 = list_rules 的打印序；报告面按**串排序**另排一次）。
var cdmRules = []cdmRule{
	{"M1", "WARN", "文件无 frontmatter 块（未回填）",
		"§A1.3③「缺失只告警」；--missing=fail 时升为 FAIL（正式面第一波验收口径）"},
	{"M2", "FAIL", "6 件套缺项（title/type/status/source_of_truth/owner/updated_at）",
		"§3.4② 必填集"},
	{"M3", "FAIL", "type 不在封闭六值内", "§3.4① 判据①"},
	{"M4", "FAIL", "status 不在封闭四值内",
		"附录B ⑲（authoritative|draft|historical|speculation）"},
	{"M5", "FAIL", "ai_assisted 不在 human|ai|mixed", "附录B ㉔"},
	{"M6", "FAIL", "未申报字段（additionalProperties: false）", "§3.4② 硬规矩①"},
	{"M7", "FAIL", "字段类型 / 形状不对（非 string、source_of_truth 空串或空表、aliases 重复等）",
		"§3.4② 表 + schema 的 type/oneOf/minLength/uniqueItems"},
	{"M8", "FAIL", "title 空 或 > 60 字（中文按字符计）", "§3.4④（本仓标题 p99 = 59）"},
	{"M9", "FAIL", "updated_at 不是 YYYY-MM-DD 或不是合法日期", "§3.4② 表（date）"},
	{"M10", "FAIL", "updated_at 晚于今天",
		"§3.4② 表「合法日期，不得晚于今天」——JSON Schema 表达不了 ⇒ 运行期判据"},
	{"M11", "FAIL", "status: authoritative 而缺 reviewed_at",
		"§B1.3 / §3.4② 表（可选字段：authoritative 时必填）"},
	{"M12", "FAIL", "type: adr 而缺 status / 日期", "§3.4① 判据③"},
	{"M13", "FAIL", "frontmatter 内重复键（YAML 非法）", "仓内教训：重复键让解析器崩/静默取一条"},
	{"M14", "BLOCK", "frontmatter 用了本脚本未建模的写法（嵌套映射 / 块标量 / 锚点 / 制表符缩进）",
		"零三方依赖下的「不给结论」口径"},
	{"M15", "BLOCK", "schema 用了本脚本未实现的关键字", "判据同源：schema 加规则而门照绿 = 假覆盖"},
	{"M16", "WARN", "type 与目录默认值不一致", "§3.4① 判据②「告警不失败」（允许显式覆盖）"},
}

// cdmMetaKeys / cdmDataKeywords —— py `META_KEYS` / `DATA_KEYWORDS`（schema 关键字白名单）。
var cdmMetaKeys = []string{"$schema", "$id", "title", "description", "$comment", "examples", "default",
	"deprecated", "readOnly", "writeOnly"}

var cdmDataKeywords = []string{"type", "enum", "const", "required", "properties", "additionalProperties",
	"items", "minItems", "maxItems", "uniqueItems", "minLength", "maxLength",
	"pattern", "format", "oneOf", "anyOf", "allOf", "not", "if", "then", "else"}

// cdmEnumRule —— py `ENUM_RULE`（字段名 ⇒ 规则号）。
var cdmEnumRule = map[string]string{"type": "M3", "status": "M4", "ai_assisted": "M5"}

// cdmFailRules —— 判 violation（rc=1）的规则号集合（py `judge_file` 的 any(...) 那一串）。
var cdmFailRules = []string{"M2", "M3", "M4", "M5", "M6", "M7", "M8", "M9", "M10", "M11", "M12", "M13"}

// ── py 语义薄壳 ─────────────────────────────────────────────────────────

// cdmPyJoin —— `os.path.join` 的**逐字**语义（保 `..` 段；✗ 不是 filepath.Join）。
// py: 若后段以 `/` 开头 ⇒ 丢弃前缀；否则前缀（若非空）末尾加一个 `/` 再拼。
func cdmPyJoin(parts ...string) string {
	out := ""
	for i, p := range parts {
		if strings.HasPrefix(p, "/") || out == "" {
			out = p
			continue
		}
		_ = i
		if strings.HasSuffix(out, "/") {
			out = out + p
		} else {
			out = out + "/" + p
		}
	}
	return out
}

// cdmPyDir —— `os.path.dirname`（py 的 dirname **不归一**，但只在 abspath 之后用 ⇒ 这里同理）。
func cdmPyDir(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	if i == 0 {
		return "/"
	}
	return p[:i]
}

// cdmNormPath —— `os.path.normpath` 的纯词法版（折叠 `//`、`.`、`..`；✗ 不解析符号链接）。
func cdmNormPath(p string) string {
	if p == "" {
		return "."
	}
	abs := strings.HasPrefix(p, "/")
	out := []string{}
	for _, c := range strings.Split(p, "/") {
		switch c {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
				continue
			}
			if abs {
				continue
			}
			out = append(out, "..")
		default:
			out = append(out, c)
		}
	}
	res := strings.Join(out, "/")
	if abs {
		res = "/" + res
	}
	if res == "" {
		res = "."
	}
	return res
}

// cdmAbs —— `os.path.abspath`（相对 ⇒ 拼 cwd；再 normpath）。
func cdmAbs(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		p = wd + "/" + p
	}
	return cdmNormPath(p)
}

// cdmRelPath —— `os.path.relpath(path, start)` 的纯词法版。
func cdmRelPath(path, start string) string {
	p := cdmAbs(path)
	s := cdmAbs(start)
	if p == s {
		return "."
	}
	pp := []string{}
	if p != "/" {
		pp = strings.Split(strings.TrimPrefix(p, "/"), "/")
	}
	ss := []string{}
	if s != "/" {
		ss = strings.Split(strings.TrimPrefix(s, "/"), "/")
	}
	i := 0
	for i < len(pp) && i < len(ss) && pp[i] == ss[i] {
		i++
	}
	out := []string{}
	for j := i; j < len(ss); j++ {
		out = append(out, "..")
	}
	out = append(out, pp[i:]...)
	if len(out) == 0 {
		return "."
	}
	return strings.Join(out, "/")
}

// cdmJoin —— 本次遍历内部一律用「斜杠拼串」（rel 只作打印/前缀匹配，✗ 不落盘）。
func cdmJoin(a, b string) string {
	if a == "" {
		return b
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// cdmDefaultRepo —— Go 侧的仓根缺省（见件头 (a)）：cwd 上溯找 `core/cmd/zerg/main.go`。
func cdmDefaultRepo() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := cdmAbs(wd)
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(d, "core", "cmd", "zerg", "main.go")); err == nil {
			return d
		}
		parent := cdmPyDir(d)
		if parent == "" || parent == d {
			break
		}
		d = parent
	}
	return cdmAbs(wd)
}

// cdmCurrentVersion —— 当前发布版（带 `v` 前缀，如 `v2.5.14`）—— 读**唯一真源**
// `core/internal/version/version.go` 的 `const Version = "X.Y.Z"`（与门① `cdnCurrentVersion` 同读法）。
// 读不到真源 ⇒ ok=false（调用方退 rc=2 fail-closed，✗ 不静默回落旧版号/空）。
func cdmCurrentVersion(repoRoot string) (string, bool) {
	p := cdmPyJoin(repoRoot, "core", "internal", "version", "version.go")
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	re := regexp.MustCompile(`const\s+Version\s*=\s*"([^"]+)"`)
	m := re.FindStringSubmatch(string(b))
	if m == nil {
		return "", false
	}
	return "v" + m[1], true
}

// cdmDevdocsRoot —— py `devdocs_root()` 的逐字同构（旋钮名同名同序）。版本目录**现读唯一真源**
// （GAP-20260927-47：曾钉死 `v2.5.10` ⇒ 换版静默过期）；读不到真源 ⇒ ok=false（调用方退 rc=2 不给结论）。
func cdmDevdocsRoot(repoRoot string) (string, bool) {
	if env := os.Getenv("ZERG_DEVDOCS_ROOT"); env != "" {
		return cdmAbs(env), true
	}
	sibling := os.Getenv("ZERG_DOCS_ALT")
	if sibling == "" {
		sibling = os.Getenv("ZERG_DOCS")
	}
	if sibling == "" {
		sibling = cdmPyJoin(cdmPyDir(cdmAbs(repoRoot)), "Zerg-内部文档")
	}
	ver, ok := cdmCurrentVersion(repoRoot)
	if !ok {
		return "", false
	}
	return cdmAbs(cdmPyJoin(sibling, "项目文档", ver)), true
}

// cdmEnvKnobs —— py `_env_knobs()`：关键取根旋钮的**实解析值**（未设 = ∅）。
// 供步日志自证「扫的哪棵树」（缺口 GAP-20260927-287）；三值同名同序照搬 py。
func cdmEnvKnobs() [3]string {
	var out [3]string
	for i, k := range []string{"ZERG_DEVDOCS_ROOT", "ZERG_DOCS_ALT", "ZERG_DOCS"} {
		if v := os.Getenv(k); v != "" {
			out[i] = v
		} else {
			out[i] = "∅"
		}
	}
	return out
}

// cdmIsDir / cdmIsFile / cdmExists —— `os.path.isdir/isfile/exists`（跟随符号链接）。
func cdmIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func cdmIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func cdmExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// ── 扫描域 ─────────────────────────────────────────────────────────────

// cdmInExcluded —— py `in_excluded()`。
func cdmInExcluded(rel string) bool {
	for _, p := range cdmExclPrefix {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return false
}

// cdmHasDocExt —— py `f.lower().endswith(DOC_EXT)`（三扩展名任一）。
func cdmHasDocExt(name string) bool {
	l := strings.ToLower(name)
	for _, e := range cdmDocExt {
		if strings.HasSuffix(l, e) {
			return true
		}
	}
	return false
}

// cdmFileEntry —— (rel_posix, abs)。
type cdmFileEntry struct {
	rel string
	abs string
}

// cdmSortedNames —— ⓒ 的「显式 sorted() 复现」（Python str 序 == UTF-8 字节序）。
func cdmSortedNames(names []string) []string {
	out := append([]string{}, names...)
	sort.Strings(out)
	return out
}

// cdmWalkDocs —— py `walk_docs()` 的逐字同构（遍历序见件头 ⓐ–ⓓ）。
func cdmWalkDocs(root string, targets []string) []cdmFileEntry {
	out := []cdmFileEntry{}
	for _, t := range targets { // ⓐ 原序：入参顺序
		if cdmIsFile(t) {
			rel := strings.ReplaceAll(cdmRelPath(t, root), string(os.PathSeparator), "/")
			if !cdmInExcluded(rel) {
				out = append(out, cdmFileEntry{rel, cdmAbs(t)})
			}
			continue
		}
		cdmWalkInto(&out, root, cdmAbs(t))
	}
	sort.SliceStable(out, func(i, j int) bool { // ⓓ 全局 `out.sort()`（(rel,abs) 元组序）
		if out[i].rel != out[j].rel {
			return out[i].rel < out[j].rel
		}
		return out[i].abs < out[j].abs
	})
	return out
}

// cdmWalkInto —— 一层 + 递归（ⓑ 目录原序 · ⓒ 文件名显式 sorted）。
func cdmWalkInto(out *[]cdmFileEntry, root, dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	ents, err := f.ReadDir(-1) // ★ 保原序读法（✗ ReadDir 排序版 / ✗ WalkDir）
	_ = f.Close()
	if err != nil {
		return
	}
	rel := cdmRelPath(dir, root)
	rel = strings.ReplaceAll(rel, string(os.PathSeparator), "/")
	relp := ""
	if rel != "." {
		relp = rel + "/"
	}
	if cdmInExcluded(relp) {
		return
	}
	dirs := []string{}  // 原序
	names := []string{} // 待显式排序
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() { // Lstat 语义（✗ 不跟随符号链接，同 os.walk 的 followlinks=False）
			if cdmInExclDirName(name) || strings.HasPrefix(name, ".") {
				continue
			}
			dirs = append(dirs, name)
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !cdmHasDocExt(name) {
			continue
		}
		names = append(names, name)
	}
	for _, name := range cdmSortedNames(names) { // ⓒ
		*out = append(*out, cdmFileEntry{relp + name, cdmJoin(cdmAbs(dir), name)})
	}
	for _, d := range dirs { // ⓑ 原序递归
		cdmWalkInto(out, root, cdmJoin(dir, d))
	}
}

// cdmInExclDirName —— py `d not in EXCL_DIRS`（元组序照搬）。
func cdmInExclDirName(name string) bool {
	for _, d := range cdmExclDirs {
		if name == d {
			return true
		}
	}
	return false
}

// ── py 值模型与 repr/str ────────────────────────────────────────────────

// cdmPyStr —— `str(v)`（本门值只有 str/bool/None/list 四种）。
func cdmPyStr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return t
	case []interface{}:
		parts := make([]string, 0, len(t))
		for _, x := range t {
			parts = append(parts, cdmPyRepr(x))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return fmt.Sprintf("%v", v)
}

// cdmPyRepr —— `repr(v)`（本门值域内的子集实现）。
func cdmPyRepr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return cdmPyQuote(t)
	case []interface{}:
		parts := make([]string, 0, len(t))
		for _, x := range t {
			parts = append(parts, cdmPyRepr(x))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return cdmPyStr(v)
}

// cdmPyQuote —— py `repr` 的引号选择 + 转义（可打印非 ASCII 原样保留，同 py3）。
func cdmPyQuote(s string) string {
	q := byte('\'')
	if strings.ContainsRune(s, '\'') {
		if strings.ContainsRune(s, '"') {
			q = '\''
		} else {
			q = '"'
		}
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch {
		case r == rune(q) && q == '\'':
			b.WriteString("\\'")
		case r == '\\':
			b.WriteString("\\\\")
		case r == '\n':
			b.WriteString("\\n")
		case r == '\r':
			b.WriteString("\\r")
		case r == '\t':
			b.WriteString("\\t")
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, "\\x%02x", r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(q)
	return b.String()
}

// cdmPadRight —— `%-Ns`（按**字符**宽度）。
func cdmPadRight(s string, n int) string {
	if utf8.RuneCountInString(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-utf8.RuneCountInString(s))
}

// ── frontmatter：严格子集解析 ──────────────────────────────────────────

// cdmUndecidable / cdmSchemaUnsupported —— py 的两个哨兵异常。
type cdmUndecidable struct{ msg string }

func (e cdmUndecidable) Error() string { return e.msg }

type cdmSchemaUnsupported struct{ msg string }

func (e cdmSchemaUnsupported) Error() string { return e.msg }

// cdmWSClass —— ★ 手写对齐 py `\s`（Unicode 语义）的字符类内容（见件头「正则语义差①」）。
const cdmWSClass = `\t\n\v\f\r \x{0085}\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{001c}-\x{001f}`

var (
	// py `re.match(r"^(.*?)\s+#", tok)`（`\s` ⇒ cdmWSClass）
	cdmReSpaceHash = regexp.MustCompile(`^([\s\S]*?)[` + cdmWSClass + `]+#`)
	// py `re.match(r"^\s+\t", line)`（`\s` ⇒ cdmWSClass）
	cdmReLeadWSThenTab = regexp.MustCompile(`^[` + cdmWSClass + `]+\t`)
	// py `re.match(r"^(\s*)- (.*)$", line)`（`\s` ⇒ cdmWSClass）
	cdmReList = regexp.MustCompile(`^([` + cdmWSClass + `]*)- ([\s\S]*)$`)
	// py `re.match(r"^([A-Za-z0-9_.\-]+):(.*)$", line)`（ASCII 显式 ⇒ 无差）
	cdmReKeyLine = regexp.MustCompile(`^([A-Za-z0-9_.\-]+):([\s\S]*)$`)
	// py `re.match(r"^\d{2}-", base)`（`\d` ⇒ `\p{Nd}`）
	cdmReTwoDigitDash = regexp.MustCompile(`^\p{Nd}\p{Nd}-`)
	// py `re.match(r"^(\d{4})-(\d{2})-(\d{2})$", s)`（`\d` ⇒ `\p{Nd}`；`$` ⇒ `\z`，见件头②）
	cdmReValidDate = regexp.MustCompile(`^(\p{Nd}{4})-(\p{Nd}{2})-(\p{Nd}{2})\z`)
)

// cdmPyLStrip —— py `str.lstrip()`（空白，近似 unicode.IsSpace，见未覆盖面②）。
func cdmPyLStrip(s string) string { return strings.TrimLeftFunc(s, unicode.IsSpace) }

// cdmSplitFrontmatter —— py `split_frontmatter()`。
func cdmSplitFrontmatter(text string) (*string, int, error) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, 1, nil
	}
	hi := len(lines)
	if hi > 400 {
		hi = 400
	}
	for i := 1; i < hi; i++ {
		s := strings.TrimSpace(lines[i])
		if s == "---" || s == "..." {
			raw := strings.Join(lines[1:i], "\n")
			return &raw, i + 2, nil
		}
	}
	return nil, 0, cdmUndecidable{"frontmatter 起始 '---' 未闭合（前 400 行内无结束行）"}
}

// cdmScalar —— py `_scalar()`（值域：string / bool / nil）。
func cdmScalar(tok string, lineno int) (interface{}, error) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return "", nil
	}
	switch tok[0] {
	case '&', '*':
		return nil, cdmUndecidable{fmt.Sprintf("第 %d 行用了 YAML 锚点/别名（& / *）", lineno)}
	case '{':
		return nil, cdmUndecidable{fmt.Sprintf("第 %d 行用了流式映射 {…}", lineno)}
	case '|', '>':
		if utf8.RuneCountInString(tok) <= 2 {
			return nil, cdmUndecidable{fmt.Sprintf("第 %d 行用了块标量（| / >）", lineno)}
		}
	case '"', '\'':
		if utf8.RuneCountInString(tok) < 2 || tok[len(tok)-1] != tok[0] {
			return nil, cdmUndecidable{fmt.Sprintf("第 %d 行的引号未闭合", lineno)}
		}
		return tok[1 : len(tok)-1], nil
	}
	if m := cdmReSpaceHash.FindStringSubmatch(tok); m != nil {
		tok = strings.TrimSpace(m[1])
	}
	switch tok {
	case "true", "True":
		return true, nil
	case "false", "False":
		return false, nil
	case "~", "null", "Null":
		return nil, nil
	}
	return tok, nil
}

// cdmInlineList —— py `_inline_list()`。
func cdmInlineList(tok string, lineno int) ([]interface{}, error) {
	s := strings.TrimSpace(tok)
	if len(s) < 2 {
		return []interface{}{}, nil
	}
	inner := s[1 : len(s)-1]
	items := []interface{}{}
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := cdmScalar(part, lineno)
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, nil
}

// cdmParseFrontmatter —— py `parse_frontmatter()`：返回 (data, dups)。
func cdmParseFrontmatter(raw string) (map[string]interface{}, []string, error) {
	data := map[string]interface{}{}
	lines := strings.Split(raw, "\n")
	pending := ""
	hasPending := false
	for idx, line := range lines {
		lineno := idx + 1
		if strings.TrimSpace(line) == "" || strings.HasPrefix(cdmPyLStrip(line), "#") {
			continue
		}
		if strings.HasPrefix(line, "\t") || cdmReLeadWSThenTab.MatchString(line) {
			return nil, nil, cdmUndecidable{fmt.Sprintf("第 %d 行用制表符缩进（YAML 不允许）", lineno)}
		}
		if m := cdmReList.FindStringSubmatch(line); m != nil {
			if !hasPending {
				return nil, nil, cdmUndecidable{fmt.Sprintf("第 %d 行是列表项，但前面没有键头", lineno)}
			}
			val, err := cdmScalar(m[2], lineno)
			if err != nil {
				return nil, nil, err
			}
			cur, _ := data[pending].([]interface{})
			data[pending] = append(cur, val)
			continue
		}
		m := cdmReKeyLine.FindStringSubmatch(line)
		if m == nil {
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				return nil, nil, cdmUndecidable{fmt.Sprintf("第 %d 行是嵌套结构（本脚本只建模顶层键）", lineno)}
			}
			head := line
			if len([]rune(head)) > 40 {
				head = string([]rune(head)[:40])
			}
			return nil, nil, cdmUndecidable{fmt.Sprintf("第 %d 行不是 'key: value' 形态：%s", lineno, cdmPyRepr(head))}
		}
		if strings.HasPrefix(line, " ") {
			return nil, nil, cdmUndecidable{fmt.Sprintf("第 %d 行有缩进（嵌套结构）", lineno)}
		}
		key, rest := m[1], strings.TrimSpace(m[2])
		if _, dup := data[key]; dup {
			data[key+"\x00dup"] = true
			continue
		}
		if rest == "" {
			data[key] = []interface{}{}
			pending, hasPending = key, true
			continue
		}
		pending, hasPending = "", false
		if rest[0] == '[' {
			if !strings.HasSuffix(rest, "]") {
				return nil, nil, cdmUndecidable{fmt.Sprintf("第 %d 行的行内列表未闭合", lineno)}
			}
			items, err := cdmInlineList(rest, lineno)
			if err != nil {
				return nil, nil, err
			}
			data[key] = items
		} else {
			val, err := cdmScalar(rest, lineno)
			if err != nil {
				return nil, nil, err
			}
			data[key] = val
		}
	}
	dups := []string{}
	for k := range data {
		if strings.HasSuffix(k, "\x00dup") {
			dups = append(dups, strings.TrimSuffix(k, "\x00dup"))
		}
	}
	sort.Strings(dups) // py `sorted(...)`
	for k := range data {
		if strings.HasSuffix(k, "\x00dup") {
			delete(data, k)
		}
	}
	return data, dups, nil
}

// ── 最小 JSON Schema 判定子集 ─────────────────────────────────────────

// cdmJNode —— 保序的 JSON 节点（py `json.loads` 出的 dict 保插入序 ⇒ ✗ 不能用 Go map）。
type cdmJNode struct {
	kind byte // 'o' 对象 · 'a' 数组 · 's' 串 · 'n' 数 · 'b' 布尔 · 'z' null
	str  string
	num  string
	b    bool
	arr  []*cdmJNode
	obj  []cdmJPair
}

type cdmJPair struct {
	k string
	v *cdmJNode
}

func (n *cdmJNode) member(k string) *cdmJNode {
	for i := range n.obj {
		if n.obj[i].k == k {
			return n.obj[i].v
		}
	}
	return nil
}

// cdmParseJSONOrdered —— 用 Decoder.Token 保序解析（重复键后值覆盖前值，同 py）。
func cdmParseJSONOrdered(text string) (*cdmJNode, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	n, err := cdmJSONValue(dec)
	if err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("Extra data")
	}
	return n, nil
}

func cdmJSONValue(dec *json.Decoder) (*cdmJNode, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch v := t.(type) {
	case json.Delim:
		switch v {
		case '{':
			nd := &cdmJNode{kind: 'o'}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k, _ := kt.(string)
				sub, err := cdmJSONValue(dec)
				if err != nil {
					return nil, err
				}
				replaced := false
				for i := range nd.obj {
					if nd.obj[i].k == k {
						nd.obj[i].v = sub
						replaced = true
						break
					}
				}
				if !replaced {
					nd.obj = append(nd.obj, cdmJPair{k, sub})
				}
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return nd, nil
		case '[':
			nd := &cdmJNode{kind: 'a'}
			for dec.More() {
				sub, err := cdmJSONValue(dec)
				if err != nil {
					return nil, err
				}
				nd.arr = append(nd.arr, sub)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return nd, nil
		}
		return nil, fmt.Errorf("unexpected delimiter %v", v)
	case string:
		return &cdmJNode{kind: 's', str: v}, nil
	case bool:
		return &cdmJNode{kind: 'b', b: v}, nil
	case json.Number:
		return &cdmJNode{kind: 'n', num: v.String()}, nil
	case nil:
		return &cdmJNode{kind: 'z'}, nil
	}
	return nil, fmt.Errorf("unexpected token")
}

// cdmNodePyStr —— py `str(x)` for JSON 值（enum 打印用）。
func cdmNodePyStr(n *cdmJNode) string {
	switch n.kind {
	case 's':
		return n.str
	case 'n':
		return n.num
	case 'b':
		if n.b {
			return "True"
		}
		return "False"
	case 'z':
		return "None"
	case 'a':
		parts := make([]string, 0, len(n.arr))
		for _, x := range n.arr {
			parts = append(parts, cdmNodePyRepr(x))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case 'o':
		parts := make([]string, 0, len(n.obj))
		for _, p := range n.obj {
			parts = append(parts, cdmPyQuote(p.k)+": "+cdmNodePyRepr(p.v))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return ""
}

func cdmNodePyRepr(n *cdmJNode) string {
	if n.kind == 's' {
		return cdmPyQuote(n.str)
	}
	return cdmNodePyStr(n)
}

// cdmCollectKeywords —— py `collect_keywords()`。
func cdmCollectKeywords(n *cdmJNode, found map[string]bool) {
	if n == nil {
		return
	}
	switch n.kind {
	case 'o':
		for _, p := range n.obj {
			found[p.k] = true
			switch p.k {
			case "properties":
				if p.v.kind == 'o' {
					for _, sub := range p.v.obj {
						cdmCollectKeywords(sub.v, found)
					}
				}
				continue
			case "items", "additionalProperties", "not", "if", "then", "else":
				if p.v.kind == 'o' || p.v.kind == 'a' {
					cdmCollectKeywords(p.v, found)
				}
				continue
			case "oneOf", "anyOf", "allOf":
				if p.v.kind == 'a' {
					for _, sub := range p.v.arr {
						cdmCollectKeywords(sub, found)
					}
				}
				continue
			}
		}
	case 'a':
		for _, sub := range n.arr {
			cdmCollectKeywords(sub, found)
		}
	}
}

// cdmTypeOK —— py `_type_ok()`（本门值域：string/bool/None/list）。
func cdmTypeOK(v interface{}, want string) (bool, error) {
	switch want {
	case "object":
		_, ok := v.(map[string]interface{})
		return ok, nil
	case "array":
		_, ok := v.([]interface{})
		return ok, nil
	case "string":
		_, ok := v.(string)
		return ok, nil
	case "boolean":
		_, ok := v.(bool)
		return ok, nil
	case "integer":
		return false, nil
	case "number":
		return false, nil
	case "null":
		return v == nil, nil
	}
	return false, cdmSchemaUnsupported{"未知 type: " + want}
}

// cdmTypeName —— py `type(value).__name__`（本门值域）。
func cdmTypeName(v interface{}) string {
	switch v.(type) {
	case string:
		return "str"
	case bool:
		return "bool"
	case nil:
		return "NoneType"
	case []interface{}:
		return "list"
	}
	return "object"
}

// cdmNdValue —— py `int()` 认 Nd 数字（见件头「正则语义差①」的连带面）。
var cdmNdStarts = []rune{
	0x0030, 0x0660, 0x06F0, 0x07C0, 0x0966, 0x09E6, 0x0A66, 0x0AE6, 0x0B66, 0x0BE6, 0x0C66, 0x0CE6,
	0x0D66, 0x0DE6, 0x0E50, 0x0ED0, 0x0F20, 0x1040, 0x1090, 0x17E0, 0x1810, 0x1946, 0x19D0, 0x1A80,
	0x1A90, 0x1B50, 0x1BB0, 0x1C40, 0x1C50, 0xA620, 0xA8D0, 0xA900, 0xA9D0, 0xA9F0, 0xAA50, 0xABF0,
	0xFF10, 0x104A0, 0x10D30, 0x11066, 0x110F0, 0x11136, 0x111D0, 0x112F0, 0x11450, 0x114D0, 0x11650,
	0x116C0, 0x11730, 0x118E0, 0x11950, 0x11C50, 0x11D50, 0x16A60, 0x16B50, 0x1D7CE, 0x1D7D8, 0x1D7E2,
	0x1D7EC, 0x1D7F6, 0x1E140, 0x1E2F0, 0x1E950, 0x1FBF0,
}

func cdmNdValue(r rune) (int, bool) {
	lo, hi := 0, len(cdmNdStarts)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		s := cdmNdStarts[mid]
		if r >= s && r < s+10 {
			return int(r - s), true
		}
		if r < s {
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}
	return 0, false
}

// cdmAtoiNd —— py `int(串)` 对 Nd 数字串的语义。
func cdmAtoiNd(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, r := range s {
		d, ok := cdmNdValue(r)
		if !ok {
			return 0, false
		}
		n = n*10 + d
	}
	return n, true
}

// cdmValidDate —— py `_valid_date()`。
func cdmValidDate(s string) bool {
	m := cdmReValidDate.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	y, ok1 := cdmAtoiNd(m[1])
	mo, ok2 := cdmAtoiNd(m[2])
	d, ok3 := cdmAtoiNd(m[3])
	if !ok1 || !ok2 || !ok3 {
		return false
	}
	if mo < 1 || mo > 12 || d < 1 || d > 31 {
		return false
	}
	t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	return t.Year() == y && int(t.Month()) == mo && t.Day() == d
}

// cdmItem —— 一条命中（py 的三元组 (rule, loc, msg)）。
type cdmItem struct{ rule, loc, msg string }

// cdmJSONCompact —— 元素去重用（py `json.dumps(x, ensure_ascii=False)`）。
func cdmJSONCompact(v interface{}) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Sprintf("%v", v)
	}
	return strings.TrimRight(b.String(), "\n")
}

// cdmRE2FromPy —— ★ 把 py 正则转成 RE2 等价形（见件头「正则语义差」①—③）。
func cdmRE2FromPy(p string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c != '\\' || i+1 >= len(p) {
			if c == '$' {
				b.WriteString(`\z`) // py 非多行 `$` → 绝对串尾（本门数据流上等价，见件头②）
				continue
			}
			b.WriteByte(c)
			continue
		}
		i++
		switch p[i] {
		case 'd':
			b.WriteString(`\p{Nd}`)
		case 'D':
			b.WriteString(`[^\p{Nd}]`)
		case 'w':
			b.WriteString(`[\p{L}\p{N}_]`)
		case 'W':
			b.WriteString(`[^\p{L}\p{N}_]`)
		case 's':
			b.WriteString(`[` + cdmWSClass + `]`)
		case 'S':
			b.WriteString(`[^` + cdmWSClass + `]`)
		default:
			b.WriteByte('\\')
			b.WriteByte(p[i])
		}
	}
	re, err := regexp.Compile(b.String())
	if err != nil {
		return "", err
	}
	_ = re
	return b.String(), nil
}

// cdmLoadSchema —— py `load_schema()`。
func cdmLoadSchema(schemaPath string) (*cdmJNode, error) {
	if !cdmIsFile(schemaPath) {
		return nil, cdmUndecidable{"schema 缺件：" + schemaPath}
	}
	txt, err := os.ReadFile(schemaPath)
	if err != nil {
		// py 该档是**未捕 OSError**（回溯 rc=1）⇒ 新口径 fail-closed 退 2，见未覆盖面⑦。
		return nil, cdmUndecidable{"schema 读不了：" + err.Error()}
	}
	if !utf8.Valid(txt) {
		return nil, cdmUndecidable{"schema 不是合法 UTF-8（py 该档为未捕 UnicodeDecodeError）"}
	}
	node, err := cdmParseJSONOrdered(string(txt))
	if err != nil {
		return nil, cdmUndecidable{"schema 不是合法 JSON：" + err.Error()}
	}
	found := map[string]bool{}
	cdmCollectKeywords(node, found)
	unknown := []string{}
	for k := range found {
		if cdmInList(k, cdmMetaKeys) || cdmInList(k, cdmDataKeywords) || strings.HasPrefix(k, "x-") {
			continue
		}
		unknown = append(unknown, k)
	}
	sort.Strings(unknown) // py `sorted(...)`
	if len(unknown) > 0 {
		return nil, cdmSchemaUnsupported{"schema 用了本脚本未实现的关键字：" + strings.Join(unknown, ", ")}
	}
	return node, nil
}

func cdmInList(s string, l []string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// cdmRE2Cache —— 已编译的 schema pattern 缓存（键 = 原串）。
var cdmRE2Cache = map[string]*regexp.Regexp{}

func cdmCompilePattern(pat string) (*regexp.Regexp, error) {
	if re, ok := cdmRE2Cache[pat]; ok {
		return re, nil
	}
	tr, err := cdmRE2FromPy(pat)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(tr)
	if err != nil {
		return nil, err
	}
	cdmRE2Cache[pat] = re
	return re, nil
}

// cdmJSONEqNode —— py `value == json 值`（本门值域内的子集）。
func cdmJSONEqNode(v interface{}, n *cdmJNode) bool {
	switch n.kind {
	case 's':
		s, ok := v.(string)
		return ok && s == n.str
	case 'b':
		bv, ok := v.(bool)
		return ok && bv == n.b
	case 'z':
		return v == nil
	case 'n':
		if bv, ok := v.(bool); ok { // py `True == 1` / `False == 0`
			if n.num == "1" {
				return bv
			}
			if n.num == "0" {
				return !bv
			}
			return false
		}
		return false
	case 'a':
		lv, ok := v.([]interface{})
		if !ok || len(lv) != len(n.arr) {
			return false
		}
		for i := range lv {
			if !cdmJSONEqNode(lv[i], n.arr[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// cdmValidate —— py `validate()`。
func cdmValidate(value interface{}, schema *cdmJNode, prop string) ([]cdmItem, error) {
	out := []cdmItem{}
	if schema == nil || schema.kind != 'o' {
		return out, nil
	}
	loc := prop
	if loc == "" {
		loc = "$"
	}
	if en := schema.member("enum"); en != nil && en.kind == 'a' {
		hit := false
		for _, e := range en.arr {
			if cdmJSONEqNode(value, e) {
				hit = true
				break
			}
		}
		if !hit {
			rule := "M7"
			if r, ok := cdmEnumRule[prop]; ok {
				rule = r
			}
			vals := make([]string, 0, len(en.arr))
			for _, e := range en.arr {
				vals = append(vals, cdmNodePyStr(e))
			}
			out = append(out, cdmItem{rule, loc,
				fmt.Sprintf("值 %s 不在封闭值域 %s", cdmPyRepr(value), strings.Join(vals, "/"))})
		}
	}
	if cn := schema.member("const"); cn != nil && !cdmJSONEqNode(value, cn) {
		out = append(out, cdmItem{"M7", loc,
			fmt.Sprintf("值不等于 const %s", cdmNodePyRepr(cn))})
	}
	if tn := schema.member("type"); tn != nil {
		wants := []string{}
		if tn.kind == 'a' {
			for _, e := range tn.arr {
				wants = append(wants, cdmNodePyStr(e))
			}
		} else {
			wants = []string{cdmNodePyStr(tn)}
		}
		okAny := false
		for _, w := range wants {
			ok, err := cdmTypeOK(value, w)
			if err != nil {
				return out, err
			}
			if ok {
				okAny = true
			}
		}
		if !okAny {
			out = append(out, cdmItem{"M7", loc,
				fmt.Sprintf("类型不符（要 %s，实际 %s）", strings.Join(wants, "/"), cdmTypeName(value))})
			return out, nil
		}
	}
	if sv, isStr := value.(string); isStr {
		runeLen := utf8.RuneCountInString(sv)
		if ml := schema.member("minLength"); ml != nil {
			want := 0
			fmt.Sscanf(cdmNodePyStr(ml), "%d", &want)
			if runeLen < want {
				rule := "M7"
				if prop == "title" {
					rule = "M8"
				}
				out = append(out, cdmItem{rule, prop, fmt.Sprintf("长度 %d < minLength %d", runeLen, want)})
			}
		}
		if ml := schema.member("maxLength"); ml != nil {
			want := 0
			fmt.Sscanf(cdmNodePyStr(ml), "%d", &want)
			if runeLen > want {
				rule := "M7"
				if prop == "title" {
					rule = "M8"
				}
				out = append(out, cdmItem{rule, prop, fmt.Sprintf("长度 %d > maxLength %d", runeLen, want)})
			}
		}
		if pn := schema.member("pattern"); pn != nil {
			re, err := cdmCompilePattern(cdmNodePyStr(pn))
			if err != nil {
				return out, cdmSchemaUnsupported{"schema pattern 转 RE2 失败：" + err.Error()}
			}
			if !re.MatchString(sv) {
				rule := "M7"
				if prop == "updated_at" || prop == "reviewed_at" {
					rule = "M9"
				}
				out = append(out, cdmItem{rule, prop,
					fmt.Sprintf("%s 不匹配 %s", cdmPyRepr(sv), cdmNodePyStr(pn))})
			}
		}
		if fn := schema.member("format"); fn != nil && cdmNodePyStr(fn) == "date" && !cdmValidDate(sv) {
			rule := "M7"
			if prop == "updated_at" || prop == "reviewed_at" {
				rule = "M9"
			}
			out = append(out, cdmItem{rule, prop, fmt.Sprintf("%s 不是合法日期", cdmPyRepr(sv))})
		}
	}
	if lv, isList := value.([]interface{}); isList {
		if mn := schema.member("minItems"); mn != nil {
			want := 0
			fmt.Sscanf(cdmNodePyStr(mn), "%d", &want)
			if len(lv) < want {
				out = append(out, cdmItem{"M7", loc, fmt.Sprintf("列表长度 %d < minItems %d", len(lv), want)})
			}
		}
		if mx := schema.member("maxItems"); mx != nil {
			want := 0
			fmt.Sscanf(cdmNodePyStr(mx), "%d", &want)
			if len(lv) > want {
				out = append(out, cdmItem{"M7", loc, fmt.Sprintf("列表长度 %d > maxItems %d", len(lv), want)})
			}
		}
		if ui := schema.member("uniqueItems"); ui != nil && ui.kind == 'b' && ui.b {
			seen := map[string]bool{}
			for _, x := range lv {
				seen[cdmJSONCompact(x)] = true
			}
			if len(seen) != len(lv) {
				out = append(out, cdmItem{"M7", loc, "列表有重复项"})
			}
		}
		if it := schema.member("items"); it != nil {
			for _, item := range lv {
				sub, err := cdmValidate(item, it, prop)
				if err != nil {
					return out, err
				}
				out = append(out, sub...)
			}
		}
	}
	if mv, isMap := value.(map[string]interface{}); isMap {
		if pn := schema.member("properties"); pn != nil && pn.kind == 'o' {
			for _, p := range pn.obj { // ★ 保插入序（✗ 不用 map 迭代）
				if sub, ok := mv[p.k]; ok {
					items, err := cdmValidate(sub, p.v, p.k)
					if err != nil {
						return out, err
					}
					out = append(out, items...)
				}
			}
		}
		if ap := schema.member("additionalProperties"); ap != nil && ap.kind == 'b' && !ap.b {
			if pn := schema.member("properties"); pn != nil && pn.kind == 'o' {
				extra := []string{}
				for k := range mv {
					if pn.member(k) == nil {
						extra = append(extra, k)
					}
				}
				sort.Strings(extra) // py `sorted(extra)`
				for _, k := range extra {
					out = append(out, cdmItem{"M6", k,
						fmt.Sprintf("未申报字段（%s 不在 schema 的 properties 里）", k)})
				}
			}
		}
		if rq := schema.member("required"); rq != nil && rq.kind == 'a' {
			for _, e := range rq.arr { // 数组序 = JSON 声明序
				k := cdmNodePyStr(e)
				if _, ok := mv[k]; !ok {
					out = append(out, cdmItem{"M2", k, "必填项缺失"})
				}
			}
		}
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		br := schema.member(key)
		if br == nil || br.kind != 'a' {
			continue
		}
		ok := false
		for _, b := range br.arr {
			sub, err := cdmValidate(value, b, prop)
			if err != nil {
				return out, err
			}
			if len(sub) == 0 {
				ok = true
				break
			}
		}
		if !ok {
			out = append(out, cdmItem{"M7", loc,
				fmt.Sprintf("%s 的 %d 个分支都不满足", key, len(br.arr))})
		}
	}
	if al := schema.member("allOf"); al != nil && al.kind == 'a' {
		for _, b := range al.arr {
			sub, err := cdmValidate(value, b, prop)
			if err != nil {
				return out, err
			}
			out = append(out, sub...)
		}
	}
	if ifn := schema.member("if"); ifn != nil {
		cond, err := cdmValidate(value, ifn, prop)
		if err != nil {
			return out, err
		}
		var branch *cdmJNode
		if len(cond) == 0 {
			branch = schema.member("then")
		} else {
			branch = schema.member("else")
		}
		if branch != nil {
			sub, err := cdmValidate(value, branch, prop)
			if err != nil {
				return out, err
			}
			for _, v := range sub {
				switch {
				case v.rule == "M2" && v.loc == "reviewed_at":
					v = cdmItem{"M11", v.loc, "status 为 authoritative ⇒ reviewed_at 必填"}
				case v.rule == "M2" && (v.loc == "status" || v.loc == "updated_at"):
					v = cdmItem{"M12", v.loc, fmt.Sprintf("type 为 adr ⇒ %s 必填", v.loc)}
				}
				out = append(out, v)
			}
		}
	}
	return out, nil
}

// ── 单文件判定 ────────────────────────────────────────────────────────

// cdmJudgeResult —— py `judge_file()` 的返回面。
type cdmJudgeResult struct {
	status string // ok | violation | warn | blocked | skipped
	cls    string
	items  []cdmItem
}

// cdmOSErrStr —— 尽力贴近 py `%s % e` 的 OSError 形态（见未覆盖面⑧）。
func cdmOSErrStr(err error, path string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no such file or directory"):
		return fmt.Sprintf("[Errno 2] No such file or directory: '%s'", path)
	case strings.Contains(msg, "permission denied"):
		return fmt.Sprintf("[Errno 13] Permission denied: '%s'", path)
	case strings.Contains(msg, "is a directory"):
		return fmt.Sprintf("[Errno 21] Is a directory: '%s'", path)
	}
	return msg
}

// cdmJudgeFile —— py `judge_file()`。
func cdmJudgeFile(rel, abspath string, schema *cdmJNode, missingMode string, today time.Time) cdmJudgeResult {
	raw0, err := os.ReadFile(abspath)
	if err != nil {
		return cdmJudgeResult{"blocked", "IO",
			[]cdmItem{{"M15", rel, "读不了：" + cdmOSErrStr(err, abspath)}}}
	}
	text := strings.ToValidUTF8(string(raw0), "\uFFFD") // py `errors="replace"`
	raw, _, serr := cdmSplitFrontmatter(text)
	if serr != nil {
		return cdmJudgeResult{"blocked", "写法未建模",
			[]cdmItem{{"M14", rel, serr.Error()}}}
	}
	if raw == nil {
		cls := "未回填"
		if cdmBase(rel) == "SKILL.md" {
			cls = "技能契约件"
			return cdmJudgeResult{"skipped", cls, nil}
		}
		status := "warn"
		if missingMode == "fail" {
			status = "violation"
		}
		return cdmJudgeResult{status, cls, []cdmItem{{"M1", rel, "无 frontmatter 块"}}}
	}
	data, dups, perr := cdmParseFrontmatter(*raw)
	if perr != nil {
		return cdmJudgeResult{"blocked", "写法未建模", []cdmItem{{"M14", rel, perr.Error()}}}
	}
	if len(data) > 0 && cdmKeysSubsetOf(data, cdmSkillContractKeys) {
		return cdmJudgeResult{"skipped", "技能契约件", nil}
	}
	items := []cdmItem{}
	for _, k := range dups {
		items = append(items, cdmItem{"M13", k, "重复键"})
	}
	vitems, verr := cdmValidate(data, schema, "")
	if verr != nil {
		if _, ok := verr.(cdmSchemaUnsupported); ok {
			return cdmJudgeResult{"blocked", "schema 不支持", []cdmItem{{"M15", rel, verr.Error()}}}
		}
		return cdmJudgeResult{"blocked", "schema 不支持", []cdmItem{{"M15", rel, verr.Error()}}}
	}
	items = append(items, vitems...)
	if ua, ok := data["updated_at"].(string); ok && cdmValidDate(ua) {
		parts := strings.Split(ua, "-")
		y, _ := cdmAtoiNd(parts[0])
		mo, _ := cdmAtoiNd(parts[1])
		d, _ := cdmAtoiNd(parts[2])
		ut := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
		if ut.After(today) {
			items = append(items, cdmItem{"M10", "updated_at",
				fmt.Sprintf("%s 晚于今天 %s", ua, today.Format("2006-01-02"))})
		}
	}
	if dt := cdmDefaultTypeFor(rel); dt != "" {
		if tv, ok := data["type"].(string); ok && tv != dt {
			items = append(items, cdmItem{"M16", "type", fmt.Sprintf("type=%s 而该目录默认值 = %s", tv, dt)})
		}
	}
	filtered := items[:0]
	for _, i := range items {
		if i.rule != "" {
			filtered = append(filtered, i)
		}
	}
	items = filtered
	for _, i := range items {
		if cdmInList(i.rule, cdmFailRules) {
			return cdmJudgeResult{"violation", "", items}
		}
	}
	if len(items) > 0 {
		return cdmJudgeResult{"warn", "", items}
	}
	return cdmJudgeResult{"ok", "", nil}
}

func cdmBase(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[i+1:]
	}
	return rel
}

func cdmKeysSubsetOf(data map[string]interface{}, allowed []string) bool {
	for k := range data {
		if !cdmInList(k, allowed) {
			return false
		}
	}
	return true
}

// cdmDefaultTypeFor —— py `default_type_for()`（判据序 = 表插入序）。
func cdmDefaultTypeFor(rel string) string {
	for _, p := range cdmTypeDefaultExact {
		if strings.HasPrefix(rel, p.pre) {
			return p.typ
		}
	}
	for _, pref := range []string{"docs/项目文档/", "../Zerg-内部文档/项目文档/"} {
		if !strings.HasPrefix(rel, pref) {
			continue
		}
		base := cdmBase(rel)
		if cdmReTwoDigitDash.MatchString(base) &&
			(strings.Contains(base, "模块") || strings.Contains(base, "架构") ||
				strings.Contains(base, "设计") || strings.Contains(base, "体系")) {
			return "reference"
		}
		for _, p2 := range []cdmTypeDefaultPair{
			{"使用-", "how-to"}, {"变更-", "record"}, {"承接项-", "record"},
			{"文档-代码对照台账", "record"}, {"任务表-", "record"},
			{"进度记录-", "record"}, {"靶子表", "record"},
		} {
			if strings.HasPrefix(base, p2.pre) {
				return p2.typ
			}
		}
		return ""
	}
	if strings.HasPrefix(rel, "docs/常青/") {
		base := cdmBase(rel)
		if strings.HasPrefix(base, "战略-") {
			return "explanation"
		}
		if strings.Contains(base, "接入") {
			return "reference"
		}
		return ""
	}
	for _, p := range cdmTypeDefaultUnsure {
		if strings.HasPrefix(rel, p) {
			return ""
		}
	}
	return ""
}

// ── 报告 ─────────────────────────────────────────────────────────────

// cdmTally —— 五档计数（打印序照搬 py 的 dict 字面量序）。
type cdmTally struct {
	ok, violation, warn, blocked, skipped int
}

func (t *cdmTally) bump(status string) {
	switch status {
	case "ok":
		t.ok++
	case "violation":
		t.violation++
	case "warn":
		t.warn++
	case "blocked":
		t.blocked++
	case "skipped":
		t.skipped++
	}
}

// cdmReport —— py `run_scan()` 的返回面。
type cdmReport struct {
	files           int
	tally           cdmTally
	frozen          cdmTally
	frozenPrefixOrd []string       // 实际命中前缀（按 cdmFrozenPrefix ++ cdmFrozenPrefixAlt 真源顺序）
	frozenPrefixCnt map[string]int // 前缀 → 命中篇数（缺口 GAP-20260926-37）
	repoHits        int            // 仓内前缀命中篇数（faces.repo）
	altHits         int            // 仓外取源根前缀命中篇数（faces.outrepo）
	perRule         map[[2]string]int
	perRuleOrd      [][2]string
	examples        map[[2]string][]string
	skippedOrd      []string
	skippedCnt      map[string]int
	violations      []string // 违规（非冻结）件名清单（缺口 GAP-20260927-283）
}

// cdmRunScan —— py `run_scan()`。
func cdmRunScan(root string, targets []string, schema *cdmJNode, missingMode string,
	countFrozen bool, today time.Time) (*cdmReport, error) {
	files := cdmWalkDocs(root, targets)
	if len(files) == 0 {
		return nil, cdmUndecidable{"扫描域为空（没有任何 " + strings.Join(cdmDocExt, "/") + " 文档）"}
	}
	rep := &cdmReport{
		files:           len(files),
		frozenPrefixCnt: map[string]int{},
		perRule:         map[[2]string]int{},
		examples:        map[[2]string][]string{},
		skippedCnt:      map[string]int{},
	}
	frozenAll := append(append([]string{}, cdmFrozenPrefix...), cdmFrozenPrefixAlt...)
	for _, fe := range files {
		isFrozen := false
		matched := ""
		for _, p := range frozenAll {
			if strings.HasPrefix(fe.rel, p) {
				isFrozen = true
				matched = p
				break
			}
		}
		if matched != "" {
			rep.frozenPrefixCnt[matched]++
		}
		res := cdmJudgeFile(fe.rel, fe.abs, schema, missingMode, today)
		bucket := &rep.tally
		if isFrozen && !countFrozen {
			bucket = &rep.frozen
		}
		bucket.bump(res.status)
		if res.status == "violation" && !(isFrozen && !countFrozen) {
			rep.violations = append(rep.violations, fe.rel)
		}
		if res.status == "skipped" {
			if _, ok := rep.skippedCnt[res.cls]; !ok {
				rep.skippedOrd = append(rep.skippedOrd, res.cls)
			}
			rep.skippedCnt[res.cls]++
		}
		kind := "live"
		if isFrozen && !countFrozen {
			kind = "frozen"
		}
		for _, it := range res.items {
			key := [2]string{it.rule, kind}
			if _, ok := rep.perRule[key]; !ok {
				rep.perRuleOrd = append(rep.perRuleOrd, key)
			}
			rep.perRule[key]++
			rep.examples[key] = append(rep.examples[key],
				fmt.Sprintf("%s → %s: %s", fe.rel, it.loc, it.msg))
		}
	}
	// 实际命中前缀（真源顺序）＋ 两张脸的命中篇数（缺口 GAP-20260926-37）。
	for _, p := range frozenAll {
		rep.frozenPrefixOrd = append(rep.frozenPrefixOrd, p)
	}
	for _, p := range cdmFrozenPrefix {
		rep.repoHits += rep.frozenPrefixCnt[p]
	}
	for _, p := range cdmFrozenPrefixAlt {
		rep.altHits += rep.frozenPrefixCnt[p]
	}
	return rep, nil
}

// cdmReportBody —— py `print_report()` 的正文（返回行列表，调用方按 LF 拼）。
func cdmReportBody(rep *cdmReport, missing string, maxExamples int) []string {
	lines := []string{}
	t := rep.tally
	lines = append(lines, fmt.Sprintf("扫描域：%d 篇（%s）", rep.files, strings.Join(cdmDocExt, "/")))
	lines = append(lines, fmt.Sprintf("  合规 %d · 不合规 %d · 告警 %d · 不可判定 %d · 剔除 %d",
		t.ok, t.violation, t.warn, t.blocked, t.skipped))
	f := rep.frozen
	if f.ok+f.violation+f.warn+f.blocked+f.skipped != 0 {
		lines = append(lines, "  冻结区（`docs/项目文档/` 仓内 + 批3 后同一批文件的现位置 `../Zerg-内部文档/项目文档/` · 只登记不改"+
			" ⇒ 不计入退码；原第二项 `docs/issues/` 已于"+
			" 2026-09-19 分家至 `Zerg-内部文档/`、条目已删。★ 二次分家批5：两处前缀**都认**（见 FROZEN_PREFIX 注）；"+
			"仓内那一条今天零命中，删它属批6 收尾）："+
			fmt.Sprintf(" 合规 %d · 不合规 %d · 告警 %d · 不可判定 %d · 剔除 %d",
				f.ok, f.violation, f.warn, f.blocked, f.skipped))
	}
	if len(rep.skippedCnt) > 0 {
		parts := append([]string{}, rep.skippedOrd...)
		sort.Strings(parts) // py `sorted(...items())`
		segs := make([]string, 0, len(parts))
		for _, k := range parts {
			segs = append(segs, fmt.Sprintf("%s %d", k, rep.skippedCnt[k]))
		}
		lines = append(lines, "  剔出类别（不算不合规）："+strings.Join(segs, " · "))
	}
	ruleIDs := make([]string, 0, len(cdmRules))
	note := map[string]cdmRule{}
	for _, r := range cdmRules {
		ruleIDs = append(ruleIDs, r.id)
		note[r.id] = r
	}
	sort.Strings(ruleIDs) // py `sorted(RULE_NOTE.items())` ⇒ 规则号按**串**序
	for _, rid := range ruleIDs {
		nLive := rep.perRule[[2]string{rid, "live"}]
		nFrozen := rep.perRule[[2]string{rid, "frozen"}]
		if nLive == 0 && nFrozen == 0 {
			continue
		}
		sev := note[rid].sev
		if rid == "M1" && missing == "fail" {
			sev = "FAIL*"
		}
		lines = append(lines, fmt.Sprintf("  %s [%s] %s —— 命中 %d（冻结区 %d）",
			rid, sev, note[rid].what, nLive, nFrozen))
		ex := rep.examples[[2]string{rid, "live"}]
		for i, line := range ex {
			if i >= maxExamples {
				break
			}
			lines = append(lines, "      · "+line)
		}
		if nLive > maxExamples {
			lines = append(lines, fmt.Sprintf("      · …另 %d 条（--max-examples 调整）", nLive-maxExamples))
		}
	}
	if missing == "fail" {
		lines = append(lines, "  ★ FAIL* = 本档由 --missing=fail 提升（默认 warn；§A1.3③ 回填期口径）")
	}
	return lines
}

// cdmDecide —— py `decide()`。
func cdmDecide(rep *cdmReport) (int, string) {
	t := rep.tally
	if t.blocked > 0 {
		return 2, fmt.Sprintf("BLOCKED_UNDECIDABLE：%d 篇判不了（不给结论）", t.blocked)
	}
	if t.violation > 0 {
		return 1, fmt.Sprintf("FAIL：%d 篇不合规", t.violation)
	}
	return 0, fmt.Sprintf("OK：合规 %d 篇（告警 %d 条不阻断）", t.ok, t.warn)
}

// ── py `json.dumps(..., ensure_ascii=False, indent=2)` 的逐字复刻 ───────

// cdmJObj —— 保序对象（✗ Go map 会被 json 包按键排序）。
type cdmJObj struct {
	keys []string
	vals []interface{}
}

func cdmJObjNew() *cdmJObj { return &cdmJObj{} }

func (o *cdmJObj) set(k string, v interface{}) *cdmJObj {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
	return o
}

// cdmJSONString —— py json 的串转义（ensure_ascii=False）。
func cdmJSONString(s string) string {
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
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// cdmDumpJSON —— py dumps 的 indent=2 格式。
func cdmDumpJSON(v interface{}, depth int, b *strings.Builder) {
	pad := strings.Repeat(" ", depth*2)
	switch t := v.(type) {
	case *cdmJObj:
		if len(t.keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for i, k := range t.keys {
			b.WriteString(strings.Repeat(" ", (depth+1)*2))
			b.WriteString(cdmJSONString(k))
			b.WriteString(": ")
			cdmDumpJSON(t.vals[i], depth+1, b)
			if i != len(t.keys)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(pad + "}")
	case []interface{}:
		if len(t) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i, x := range t {
			b.WriteString(strings.Repeat(" ", (depth+1)*2))
			cdmDumpJSON(x, depth+1, b)
			if i != len(t)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(pad + "]")
	case string:
		b.WriteString(cdmJSONString(t))
	case int:
		fmt.Fprintf(b, "%d", t)
	default:
		fmt.Fprintf(b, "%v", t)
	}
}

// ── 自检（成对负控）────────────────────────────────────────────────────

// cdmGoodDoc —— py `GOOD_DOC`。
const cdmGoodDoc = "---\n" +
	"title: 好件\n" +
	"type: reference\n" +
	"status: draft\n" +
	"source_of_truth: tools/versions.json\n" +
	"owner: Mr2109\n" +
	"updated_at: 2026-09-18\n" +
	"ai_assisted: mixed\n" +
	"---\n\n" +
	"# 好件\n"

// cdmBadDocs —— py `BAD_DOCS`（名字/定位/规则号/正文）。
var cdmBadDocs = [][4]string{
	{"bad-missing-owner.md", "owner", "M2",
		"---\ntitle: 缺 owner\ntype: reference\nstatus: draft\nsource_of_truth: x\nupdated_at: 2026-09-18\n---\n"},
	{"bad-type.md", "type", "M3",
		"---\ntitle: 坏 type\ntype: guide\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"},
	{"bad-status.md", "status", "M4",
		"---\ntitle: 坏 status\ntype: reference\nstatus: reviewed\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"},
	{"bad-ai-assisted.md", "ai_assisted", "M5",
		"---\ntitle: 坏披露\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\nai_assisted: robot\n---\n"},
	{"bad-extra.md", "custom", "M6",
		"---\ntitle: 未申报字段\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\ncustom: 1\n---\n"},
	{"bad-shape.md", "source_of_truth", "M7",
		"---\ntitle: 形状不对\ntype: reference\nstatus: draft\nsource_of_truth: \"\"\nowner: o\nupdated_at: 2026-09-18\n---\n"},
	{"bad-title-long.md", "title", "M8", ""}, // 正文在运行时拼（61 个「长」）
	{"bad-date-fmt.md", "updated_at", "M9",
		"---\ntitle: 日期形态错\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026/09/18\n---\n"},
	{"bad-date-future.md", "updated_at", "M10",
		"---\ntitle: 未来日期\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2099-01-01\n---\n"},
	{"bad-authoritative.md", "reviewed_at", "M11",
		"---\ntitle: 权威但没复审\ntype: reference\nstatus: authoritative\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"},
	{"bad-dup-key.md", "title", "M13",
		"---\ntitle: 第一次\ntitle: 第二次\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"},
}

// cdmWarnDirDefaultDoc —— py `WARN_DIR_DEFAULT_DOC`。
const cdmWarnDirDefaultDoc = "---\ntitle: 目录默认值不符\ntype: record\nstatus: draft\n" +
	"source_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"

// cdmBlockedDocs —— py `BLOCKED_DOCS`。
var cdmBlockedDocs = [][3]string{
	{"blk-nested.md", "M14",
		"---\ntitle: 嵌套\nmeta:\n  a: 1\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"},
	{"blk-blockscalar.md", "M14",
		"---\ntitle: |\n  多行\n  文本\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"},
}

// cdmBadTitleLongBody —— py `"长" * 61` 的运行期拼串（✗ 不落 61 个「长」的连续字面）。
func cdmBadTitleLongBody() string {
	return "---\ntitle: " + strings.Repeat("长", 61) +
		"\ntype: reference\nstatus: draft\nsource_of_truth: x\nowner: o\nupdated_at: 2026-09-18\n---\n"
}

func cdmWrite(path, text string) error {
	if err := os.MkdirAll(cdmPyDir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// cdmTmpBase —— py `_tmp_base()`（尊重 TMPDIR；读不出 ⇒ 交回库默认落点）。
func cdmTmpBase() string {
	d := os.Getenv("TMPDIR")
	if d != "" && cdmIsDir(d) {
		return d
	}
	return ""
}

// cdmSelfTest —— py `self_test()`：夹具基目录尊重 TMPDIR、跑完即清。
func cdmSelfTest() (bool, []string) {
	tmp, err := os.MkdirTemp(cdmTmpBase(), "check-doc-meta-")
	if err != nil {
		return false, []string{"✗ 建不了自检临时目录：" + err.Error()}
	}
	defer os.RemoveAll(tmp)
	return cdmSelfTestBody(tmp)
}

// cdmRunCapture —— 子进程的等价物：in-process 跑一次入口（stdout/stderr 合流，同 `stderr=STDOUT`）。
func cdmRunCapture(argv []string) (int, string) {
	var sb strings.Builder
	rc := checkDocMetaGateMain(&sb, &sb, argv)
	return rc, sb.String()
}

// cdmSelfTestBody —— py `_self_test_body()`。
func cdmSelfTestBody(tmp string) (bool, []string) {
	lines := []string{}
	ok := true
	schema_src := cdmPyJoin(cdmDefaultRepo(), cdmSchemaDefault)
	if !cdmIsFile(schema_src) {
		return false, []string{"✗ 缺件：真 schema 不存在 " + schema_src + " ⇒ 自检不给结论"}
	}
	schemaDir := cdmPyJoin(tmp, "schema")
	schemaDst := cdmPyJoin(schemaDir, cdmBase(cdmSchemaDefault))
	schemaText, err := os.ReadFile(schema_src)
	if err != nil {
		return false, []string{"✗ 读不了真 schema：真 schema 不存在 " + schema_src + " ⇒ 自检不给结论"}
	}
	_ = cdmWrite(schemaDst, string(schemaText))
	good := cdmPyJoin(tmp, "good")
	_ = cdmWrite(cdmPyJoin(good, "a-good.md"), cdmGoodDoc)
	bad := cdmPyJoin(tmp, "bad")
	for _, bd := range cdmBadDocs {
		body := bd[3]
		if bd[0] == "bad-title-long.md" {
			body = cdmBadTitleLongBody()
		}
		_ = cdmWrite(cdmPyJoin(bad, bd[0]), body)
	}
	nofm := cdmPyJoin(tmp, "nofm")
	_ = cdmWrite(cdmPyJoin(nofm, "legacy-no-fm.md"), "# 没有 frontmatter\n")
	_ = cdmWrite(cdmPyJoin(nofm, "legacy-second.md"), "# 第二篇也没有\n")
	blk := cdmPyJoin(tmp, "blocked")
	for _, bd := range cdmBlockedDocs {
		_ = cdmWrite(cdmPyJoin(blk, bd[0]), bd[2])
	}
	skill := cdmPyJoin(tmp, "skill")
	_ = cdmWrite(cdmPyJoin(skill, "SKILL.md"),
		"---\nname: some-skill\ndescription: 技能契约件\n---\n\n# x\n")
	dirdef := cdmPyJoin(tmp, "dirdef")
	_ = cdmWrite(cdmPyJoin(dirdef, "docs", "skills", "warn-dir-default.md"), cdmWarnDirDefaultDoc)
	empty := cdmPyJoin(tmp, "empty")
	_ = os.MkdirAll(empty, 0o755)

	base := []string{"--schema", schemaDst, "--repo-root", tmp, "--no-self-test"}

	caseFn := func(label string, expectRC int, args []string, mustContain, mustNotContain []string,
		env map[string]string) bool {
		oldEnv := map[string]*string{}
		for k, v := range env {
			if cur, ok := os.LookupEnv(k); ok {
				c := cur
				oldEnv[k] = &c
			} else {
				oldEnv[k] = nil
			}
			_ = os.Setenv(k, v)
		}
		argv := append(append([]string{}, base...), args...)
		rc, out := cdmRunCapture(argv)
		for k, v := range oldEnv {
			if v == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *v)
			}
		}
		goodCase := rc == expectRC
		for _, s := range mustContain {
			if !strings.Contains(out, s) {
				goodCase = false
			}
		}
		for _, s := range mustNotContain {
			if strings.Contains(out, s) {
				goodCase = false
			}
		}
		mark := "✗"
		if goodCase {
			mark = "✓"
		}
		lines = append(lines, fmt.Sprintf("%s %s（期望 rc=%d，实得 rc=%d）", mark, label, expectRC, rc))
		if !goodCase {
			all := strings.Split(strings.TrimSpace(out), "\n")
			tailLines := []string{}
			if len(all) > 4 {
				tailLines = all[len(all)-4:]
			} else {
				tailLines = all
			}
			parts := make([]string, 0, len(tailLines))
			for _, ln := range tailLines {
				parts = append(parts, cdmPyQuote(ln))
			}
			lines = append(lines, "    输出尾部：["+strings.Join(parts, ", ")+"]")
		}
		return goodCase
	}

	// ① 好件必绿
	ok = caseFn("好件必绿", 0, []string{"--target", good, "--missing=fail"},
		[]string{"合规 1", "OK"}, nil, nil) && ok
	// ② 坏件必红（逐条命中它自己的规则号）
	for _, bd := range cdmBadDocs {
		ok = caseFn("坏件必红 "+bd[0], 1,
			[]string{"--target", cdmPyJoin(bad, bd[0]), "--missing=fail"},
			[]string{bd[2], bd[1]}, nil, nil) && ok
	}
	// ③ 缺件必 rc=2
	ok = caseFn("缺件 · 目标路径不存在", 2, []string{"--target", cdmPyJoin(tmp, "no-such-dir")},
		[]string{"不给结论"}, nil, nil) && ok
	ok = caseFn("缺件 · schema 不存在", 2,
		[]string{"--schema", cdmPyJoin(tmp, "no-such-schema.json"), "--target", good},
		[]string{"不给结论"}, nil, nil) && ok
	ok = caseFn("缺件 · 扫描域为空", 2, []string{"--target", empty},
		[]string{"不给结论"}, nil, nil) && ok
	// ④ 不可判定 ⇒ rc=2
	for _, bd := range cdmBlockedDocs {
		ok = caseFn("不可判定 "+bd[0], 2,
			[]string{"--target", cdmPyJoin(blk, bd[0]), "--missing=fail"},
			[]string{bd[1]}, nil, nil) && ok
	}
	// ⑤ 未回填两种口径
	ok = caseFn("未回填默认为告警（rc=0）", 0, []string{"--target", nofm},
		[]string{"告警 2", "M1"}, nil, nil) && ok
	ok = caseFn("未回填 --missing=fail 为红（rc=1）", 1, []string{"--target", nofm, "--missing=fail"},
		[]string{"不合规 2"}, nil, nil) && ok
	// ⑥ 技能契约件
	ok = caseFn("技能契约件被剔除", 0, []string{"--target", skill, "--missing=fail"},
		[]string{"技能契约件"}, []string{"不合规 1"}, nil) && ok
	// ⑦ 目录默认值不一致 = 告警
	ok = caseFn("目录默认值不一致只告警", 0,
		[]string{"--target", cdmPyJoin(dirdef, "docs", "skills", "warn-dir-default.md"), "--repo-root", dirdef},
		[]string{"M16", "告警 1"}, nil, nil) && ok
	// ⑧ schema 未实现关键字 ⇒ rc=2
	drift := cdmPyJoin(tmp, "schema-drift.json")
	_ = cdmWrite(drift, "{\"type\":\"object\",\"required\":[\"title\"],\"dependencies\":{\"a\":[\"b\"]}}")
	ok = caseFn("schema 未实现关键字 ⇒ 不给结论", 2,
		[]string{"--schema", drift, "--target", good, "--missing=fail"},
		[]string{"未实现的关键字"}, nil, nil) && ok
	// ⑨ 只报告不改
	before := cdmTreeSHA(tmp)
	_, _ = cdmRunCapture(append(append([]string{}, base...), "--target", bad, "--missing=fail"))
	after := cdmTreeSHA(tmp)
	same := before == after
	mark := "✗"
	if same {
		mark = "✓"
	}
	note := "被改动"
	if same {
		note = "一致"
	}
	lines = append(lines, fmt.Sprintf("%s 只报告不改（夹具树逐字节不变：%s）", mark, note))
	ok = same && ok
	// ⑩ D3① 换根负控
	ok = caseFn("D3① devdocs 根不在 ⇒ BLOCKED（仓外不可达不许当绿）", 2,
		[]string{"--repo-root", tmp, "--scope", "devdocs"},
		[]string{"不给结论", "开发文档面根不在"},
		nil, map[string]string{"ZERG_DEVDOCS_ROOT": cdmPyJoin(tmp, "no-such-Zerg-内部文档")}) && ok

	expected := 1 + len(cdmBadDocs) + 3 + len(cdmBlockedDocs) + 2 + 1 + 1 + 1 + 1 + 1
	got := 0
	for _, ln := range lines {
		if strings.HasPrefix(ln, "✓") || strings.HasPrefix(ln, "✗") {
			got++
		}
	}
	lines = append(lines, fmt.Sprintf("自检用例：期望 %d 条 · 实跑 %d 条", expected, got))
	if got != expected {
		lines = append(lines, "✗ 用例条数不符（有静默截断）")
		ok = false
	}
	lines = append(lines, "自检临时目录："+tmp+"（跑完已删）")
	return ok, lines
}

// cdmTreeSHA —— py 自检 ⑨ 的 `tree_sha()`（显式排序：`dns.sort()` + `sorted(fns)`）。
func cdmTreeSHA(d string) string {
	h := sha256.New()
	cdmTreeSHAWalk(h, d, d)
	return hex.EncodeToString(h.Sum(nil))
}

// cdmTreeSHAWalk —— 递归（py `os.walk` + `dns.sort()` + `sorted(fns)`）。
func cdmTreeSHAWalk(h hash.Hash, root, dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	ents, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		return
	}
	dirs := []string{}
	files := []string{}
	for _, e := range ents {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		} else {
			files = append(files, e.Name())
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	for _, name := range files {
		p := cdmJoin(dir, name)
		rel := cdmRelPath(p, root)
		h.Write([]byte(rel))
		if b, err := os.ReadFile(p); err == nil {
			h.Write(b)
		}
	}
	for _, name := range dirs {
		cdmTreeSHAWalk(h, root, cdmJoin(dir, name))
	}
}

// ── 入口 ─────────────────────────────────────────────────────────────

// cdmUsage —— py argparse 的 usage 串（逐字）。
const cdmUsage = "usage: check-doc-meta.py [-h] [--target TARGET] [--repo-root REPO_ROOT]\n" +
	"                         [--schema SCHEMA]\n" +
	"                         [--scope {repo,docs,formal,devdocs}]\n" +
	"                         [--missing {warn,fail}] [--count-frozen]\n" +
	"                         [--max-examples MAX_EXAMPLES] [--json] [--list-rules]\n" +
	"                         [--self-test] [--no-self-test]\n" +
	"                         [targets ...]\n"

// cdmHelp —— py `-h` 的正文（逐字）。
const cdmHelp = cdmUsage + "\n" +
	"文档 frontmatter 元数据门（只报告不改）\n" +
	"\n" +
	"positional arguments:\n" +
	"  targets               扫描目标（文件或目录；默认 = 仓根）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --target TARGET       同上（可多次）\n" +
	"  --repo-root REPO_ROOT\n" +
	"                        仓根（默认 = 本脚本的上一级）\n" +
	"  --schema SCHEMA       schema 路径（默认 <仓根>/docs/site/frontmatter-schema.json）\n" +
	"  --scope {repo,docs,formal,devdocs}\n" +
	"                        repo=全仓文档面 · docs=仅 docs/ · formal=正式面候选第一波（§8.4③）·\n" +
	"                        devdocs=**开发文档面**（D3① 换根：Zerg-内部文档，根见\n" +
	"                        devdocs_root()；schema 仍取主仓）\n" +
	"  --missing {warn,fail}\n" +
	"                        无 frontmatter 块的档位（默认 warn；正式面第一波用 fail）\n" +
	"  --count-frozen        把冻结区（项目文档/）计入退码；`docs/issues/` 已于 2026-09-19 分家至\n" +
	"                        `Zerg-内部文档/` ⇒ 不再有该区\n" +
	"  --max-examples MAX_EXAMPLES\n" +
	"  --json\n" +
	"  --list-rules\n" +
	"  --self-test\n" +
	"  --no-self-test        内部子进程用（防递归）\n"

// cdmArgs —— argparse 解析结果。
type cdmArgs struct {
	targets     []string
	repoRoot    string
	schema      string
	scope       string
	missing     string
	countFrozen bool
	maxExamples int
	json        bool
	listRules   bool
	selfTest    bool
	noSelfTest  bool
}

// cdmArgvErr —— argparse 式错误（usage + error，退 2）。
type cdmArgvErr struct {
	msg string
	// usageOnly: 只印 usage（-h 用）；空 err 且 helpWanted ⇒ 印全帮助
}

// cdmParseArgs —— argparse 的等价解析（长选项前缀匹配 + `--opt=值`）。
func cdmParseArgs(argv []string) (*cdmArgs, string, bool, error) {
	longOpts := []string{"--target", "--repo-root", "--schema", "--scope", "--missing",
		"--count-frozen", "--max-examples", "--json", "--list-rules", "--self-test", "--no-self-test", "--help"}
	args := &cdmArgs{scope: "docs", missing: "warn", maxExamples: 5}
	positional := []string{}
	unrecognized := []string{}
	endOpts := false

	resolve := func(tok string) (string, string, error) {
		// 返回 (全名, 内联值, err)
		name := tok
		inline := ""
		hasInline := false
		if i := strings.Index(tok, "="); i >= 0 {
			name = tok[:i]
			inline = tok[i+1:]
			hasInline = true
		}
		if name == "-h" {
			name = "--help"
		} else if strings.HasPrefix(name, "--") {
			matches := []string{}
			for _, o := range longOpts {
				if o == name {
					matches = []string{o}
					break
				}
				if strings.HasPrefix(o, name) {
					matches = append(matches, o)
				}
			}
			if len(matches) == 0 {
				return "", "", fmt.Errorf("unrecognized arguments: %s", tok)
			}
			if len(matches) > 1 {
				return "", "", fmt.Errorf("ambiguous option: %s could match %s", name, strings.Join(matches, ", "))
			}
			name = matches[0]
		} else {
			return "", "", fmt.Errorf("unrecognized arguments: %s", tok)
		}
		if hasInline {
			return name, inline, nil
		}
		return name, "\x00none", nil
	}

	needsValue := map[string]bool{"--target": true, "--repo-root": true, "--schema": true,
		"--scope": true, "--missing": true, "--max-examples": true}

	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if endOpts {
			positional = append(positional, tok)
			continue
		}
		if tok == "--" {
			endOpts = true
			continue
		}
		if tok == "-h" || tok == "--help" {
			return nil, cdmHelp, true, nil
		}
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			positional = append(positional, tok)
			continue
		}
		name, inline, err := resolve(tok)
		if err != nil {
			return nil, "", false, err
		}
		val := inline
		if inline == "\x00none" {
			if needsValue[name] {
				if i+1 >= len(argv) || (strings.HasPrefix(argv[i+1], "-") && argv[i+1] != "-" && len(argv[i+1]) > 1) {
					return nil, "", false, fmt.Errorf("argument %s: expected one argument", name)
				}
				i++
				val = argv[i]
			} else {
				val = ""
			}
		}
		switch name {
		case "--target":
			args.targets = append(args.targets, val)
		case "--repo-root":
			args.repoRoot = val
		case "--schema":
			args.schema = val
		case "--scope":
			if !cdmInList(val, []string{"repo", "docs", "formal", "devdocs"}) {
				return nil, "", false, fmt.Errorf("argument --scope: invalid choice: '%s' (choose from 'repo', 'docs', 'formal', 'devdocs')", val)
			}
			args.scope = val
		case "--missing":
			if !cdmInList(val, []string{"warn", "fail"}) {
				return nil, "", false, fmt.Errorf("argument --missing: invalid choice: '%s' (choose from 'warn', 'fail')", val)
			}
			args.missing = val
		case "--count-frozen":
			args.countFrozen = true
		case "--max-examples":
			n := 0
			if _, err := fmt.Sscanf(val, "%d", &n); err != nil || fmt.Sprintf("%d", n) != val {
				return nil, "", false, fmt.Errorf("argument --max-examples: invalid int value: '%s'", val)
			}
			args.maxExamples = n
		case "--json":
			args.json = true
		case "--list-rules":
			args.listRules = true
		case "--self-test":
			args.selfTest = true
		case "--no-self-test":
			args.noSelfTest = true
		}
	}
	if len(unrecognized) > 0 {
		return nil, "", false, fmt.Errorf("unrecognized arguments: %s", strings.Join(unrecognized, " "))
	}
	args.targets = append(positional, args.targets...)
	return args, "", false, nil
}

// cdmListRules —— py `list_rules()`。
func cdmListRules() []string {
	lines := []string{"check-doc-meta.py 规则表（值域与字段一律读 docs/site/frontmatter-schema.json）"}
	for _, r := range cdmRules {
		lines = append(lines, fmt.Sprintf("  %s [%s] %s", cdmPadRight(r.id, 4), cdmPadRight(r.sev, 5), r.what))
		lines = append(lines, "        出处："+r.src)
	}
	lines = append(lines, "  退码：0 全绿 · 1 有不合规项 · 2 不给结论（优先级 2>1>0）；冻结区不计入退码。")
	return lines
}

// cdmJoinLines —— 按 py `print()` 的语义（每行一个 \n）落盘。
func cdmJoinLines(lines []string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	return b.String()
}

// checkDocMetaGateMain —— 入口（与 py `main()` 逐字同构）。
func checkDocMetaGateMain(stdout, stderr io.Writer, argv []string) int {
	args, helpText, helpWanted, err := cdmParseArgs(argv)
	if err != nil {
		fmt.Fprint(stderr, cdmUsage)
		fmt.Fprintf(stderr, "%s: error: %s\n", cdmProg, err.Error())
		return 2
	}
	if helpWanted {
		fmt.Fprint(stdout, helpText)
		return 0
	}
	if args.listRules {
		fmt.Fprint(stdout, cdmJoinLines(cdmListRules()))
		return 0
	}
	if args.selfTest {
		ok, lines := cdmSelfTest()
		out := []string{"check-doc-meta 自检（成对负控：好件必绿 / 坏件必红 / 缺件必 rc=2）"}
		for _, ln := range lines {
			out = append(out, "  "+ln)
		}
		fmt.Fprint(stdout, cdmJoinLines(out))
		if ok {
			return 0
		}
		return 2
	}
	if !args.noSelfTest {
		ok, lines := cdmSelfTest()
		if !ok {
			out := []string{"自检未过 ⇒ 拒绝扫真目标（rc=2 不给结论）"}
			for _, ln := range lines {
				out = append(out, "  "+ln)
			}
			fmt.Fprint(stdout, cdmJoinLines(out))
			return 2
		}
	}

	repoRoot := args.repoRoot
	if repoRoot == "" {
		repoRoot = cdmDefaultRepo()
	}
	repo := cdmAbs(repoRoot)
	given := append(append([]string{}, args.targets...), []string{}...)

	var root string
	var targets []string
	if args.scope == "devdocs" && len(given) == 0 {
		var verOK bool
		root, verOK = cdmDevdocsRoot(repo)
		if !verOK {
			fmt.Fprint(stdout, "BLOCKED：版本真源读不到（core/internal/version/version.go 里没有 `const Version = \"…\"`）⇒ 现算不出开发文档面根 ⇒ 不给结论\n")
			fmt.Fprint(stdout, "  口径：读不到真源不当绿（设计稿 §五）。换根请设 ZERG_DEVDOCS_ROOT。\n")
			return 2
		}
		if !cdmIsDir(root) {
			fmt.Fprintf(stdout, "BLOCKED：开发文档面根不在 ⇒ 不给结论：%s\n", root)
			fmt.Fprint(stdout, "  口径：仓外根不可达（设计稿 §五：不许静默当绿 ✗）。换根请设 ZERG_DEVDOCS_ROOT。\n")
			return 2
		}
		targets = []string{root}
	} else {
		root = repo
		targets = []string{}
		for _, x := range given {
			targets = append(targets, cdmAbs(x))
		}
		if len(targets) == 0 {
			if args.scope == "repo" {
				targets = []string{root}
			} else {
				targets = []string{cdmPyJoin(root, "docs")}
			}
		}
	}
	schemaPath := cdmPyJoin(repo, cdmSchemaDefault)
	if args.schema != "" {
		schemaPath = cdmAbs(args.schema)
	}
	for _, t := range targets {
		if !cdmExists(t) {
			fmt.Fprintf(stdout, "BLOCKED：目标不存在 ⇒ 不给结论：%s\n", t)
			return 2
		}
	}
	if args.scope == "formal" {
		keep := []string{}
		for _, fp := range cdmFormalPrefix {
			for _, cand := range []string{fp.in, fp.alt} {
				if cand == "" {
					continue
				}
				p := cdmPyJoin(root, cand) // ★ 保 `..`（✗ filepath.Join）
				if cdmIsDir(p) {
					keep = append(keep, p)
					break
				}
			}
		}
		sort.Strings(keep)
		uniq := []string{}
		for i, p := range keep {
			if i > 0 && keep[i-1] == p {
				continue
			}
			uniq = append(uniq, p)
		}
		targets = uniq
		if len(targets) == 0 {
			fmt.Fprint(stdout, "BLOCKED：正式面候选目录一个都不存在（仓内 + 仓外取源根两处都没有）⇒ 不给结论\n")
			for _, fp := range cdmFormalPrefix {
				extra := ""
				if fp.alt != "" {
					extra = fmt.Sprintf("｜仓外候选：%s（解析 = %s）", fp.alt, cdmPyJoin(root, fp.alt))
				}
				fmt.Fprintf(stdout, "  缺件路径：%s（仓根下解析 = %s）%s\n", fp.in, cdmPyJoin(root, fp.in), extra)
			}
			fmt.Fprint(stdout, "  口径：公开树侧（无 docs/）本 scope 为 BLOCKED、不适用；私有树正常判（判据一字未改）。\n")
			return 2
		}
	}
	if !cdmIsDir(root) {
		fmt.Fprintf(stdout, "BLOCKED：仓根不是目录 ⇒ 不给结论：%s\n", root)
		return 2
	}
	schema, err := cdmLoadSchema(schemaPath)
	if err != nil {
		fmt.Fprintf(stdout, "BLOCKED：%s ⇒ 不给结论\n", err.Error())
		return 2
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	rep, err := cdmRunScan(root, targets, schema, args.missing, args.countFrozen, today)
	if err != nil {
		fmt.Fprintf(stdout, "BLOCKED：%s ⇒ 不给结论\n", err.Error())
		return 2
	}
	rc, verdict := cdmDecide(rep)
	if args.json {
		obj := cdmJObjNew()
		obj.set("rc", rc)
		obj.set("verdict", verdict)
		obj.set("scope", args.scope)
		obj.set("missing_mode", args.missing)
		obj.set("files", rep.files)
		tally := cdmJObjNew()
		tally.set("ok", rep.tally.ok).set("violation", rep.tally.violation).set("warn", rep.tally.warn).
			set("blocked", rep.tally.blocked).set("skipped", rep.tally.skipped)
		obj.set("tally", tally)
		frozen := cdmJObjNew()
		frozen.set("ok", rep.frozen.ok).set("violation", rep.frozen.violation).set("warn", rep.frozen.warn).
			set("blocked", rep.frozen.blocked).set("skipped", rep.frozen.skipped)
		obj.set("frozen", frozen)
		fpv := []interface{}{}
		for _, p := range rep.frozenPrefixOrd {
			if rep.frozenPrefixCnt[p] > 0 {
				fpv = append(fpv, p)
			}
		}
		obj.set("frozen_prefix", fpv)
		faces := cdmJObjNew()
		faces.set("repo", rep.repoHits).set("outrepo", rep.altHits)
		obj.set("faces", faces)
		skipped := cdmJObjNew()
		for _, k := range rep.skippedOrd {
			skipped.set(k, rep.skippedCnt[k])
		}
		obj.set("skipped", skipped)
		perRule := cdmJObjNew()
		keys := append([][2]string{}, rep.perRuleOrd...)
		sort.Slice(keys, func(i, j int) bool {
			if keys[i][0] != keys[j][0] {
				return keys[i][0] < keys[j][0]
			}
			return keys[i][1] < keys[j][1]
		})
		for _, k := range keys {
			perRule.set(k[0]+"/"+k[1], rep.perRule[k])
		}
		obj.set("per_rule", perRule)
		viol := []interface{}{}
		for _, v := range rep.violations {
			viol = append(viol, v)
		}
		obj.set("violations", viol)
		var b strings.Builder
		cdmDumpJSON(obj, 0, &b)
		b.WriteString("\n")
		fmt.Fprint(stdout, b.String())
		return rc
	}
	schemaShown := schemaPath
	if strings.HasPrefix(schemaPath, root) {
		schemaShown = cdmRelPath(schemaPath, root)
	}
	knobs := cdmEnvKnobs()
	fmt.Fprintf(stdout, "  根与旋钮（实解析）：repo_root=%s · scan_root=%s · "+
		"ZERG_DEVDOCS_ROOT=%s · ZERG_DOCS_ALT=%s · ZERG_DOCS=%s\n",
		repo, root, knobs[0], knobs[1], knobs[2])
	fmt.Fprintf(stdout, "check-doc-meta —— schema=%s（scope=%s · missing=%s）\n",
		schemaShown, args.scope, args.missing)
	fmt.Fprint(stdout, cdmJoinLines(cdmReportBody(rep, args.missing, args.maxExamples)))
	fmt.Fprintf(stdout, "%s\n", verdict)
	return rc
}
