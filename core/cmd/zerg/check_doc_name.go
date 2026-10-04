// check_doc_name.go —— 判据件 `scripts/gates/check-doc-name.py`（1071 行）的 **Go 真搬**（M2 · 甲案 C 类第 7 枚）。
//
// 入口 `checkDocNameGateMain(stdout, stderr io.Writer, argv []string) int`（M2 刻意无调用点：不接线 / 不注册命令 /
// 不删 py 原件）。验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），stdout / rc **逐字节相同**（差异面见下）。
//
// ★ 两条标准声明行：
//
//	(a) 默认根定位差异：py `HERE = dirname(abspath(__file__))` · `REPO_ROOT_DEFAULT = dirname(dirname(HERE))`
//	    ⇒ 默认根起于**脚本所在目录**（与 cwd 无关）；本件无 `__file__` 语境 ⇒ 默认根 = cwd（`cdnAbs(".")`）。
//	    逐字对比：py `REPO_ROOT_DEFAULT`（脚本位置上溯两层）⟷ Go `cdnDefaultRepoRoot()`（cwd）。
//	    「脚本位置 == cwd」时两者读数逐字相同；否则路径推导面不同 ⇒ 不属逐字对齐承诺。
//	(b) 自指检查（先真判）：本件是 `.go`、落 `core/cmd/zerg/`；本门扫描面 = 《清单》§0 全仓文件/目录**名**，
//	    本件名 `check_doc_name.go` 为小写+下划线（N1/N2/N3/N4 皆不咬），且本门只判**名称**、不读 `.go` 正文
//	    ⇒ 与本门自身扫描面不相交。他门面：本件落在 `check-error-kinds`(R3 扫 core/cmd/zerg/*.go) 的扫描面内
//	    ⇒ 正文零被扫错误构造调用原串、零私有绝对路径字面量（绝对根一律运行时取）；落仓前后他门读数见回执。
//
// 差异面（声明 · 逐条）：
//
//	① `--help`/`-h` 正文：py argparse 自动生成 ⟷ 本件自写 usage ⇒ **正文不逐字对齐**（rc=0 同），列入未覆盖面。
//	② 未知参数 / 非法 `--scope`：py argparse 打 usage 到 stderr 后 rc=2 ⟷ 本件打简短 usage 到 stderr、rc=2
//	   ⇒ **stderr 正文不逐字对齐**（stdout / rc 对齐）。
//	③ `--self-test` 的**元自检（⑫ 注入副本）**：py 把自身源码改写后起子进程验「自检未过 ⇒ 拒绝扫真目标」；
//	   本件无「注入源码副本」语境 ⇒ 未搬该格（其余自检格逐条搬）。默认档（未给 `--self-test`）自检成功**零输出**，
//	   故默认档 stdout 与 py 逐字相同；显式 `--self-test` 清单少 1 行（元自检），列入未覆盖面。
//	④ 目录遍历序：本件走 `os.ReadDir` 读面 + 末尾**全局 `sort` 定序**（py 亦末尾 `out.sort()`）⇒ 输出行序由该
//	   全局排序决定，非 readdir 原序；`judge_same_stem` 的 `members[0]` 亦取全局排序后首件（见 cdnWalk 注）。
//	⑤ py 正则 `\s`（N7 分段 / `re.split`）为 Unicode 语义 ⟷ Go RE2 `\s` 为 ASCII ⇒ 词干夹 Unicode 空白
//	   （U+00A0 一类）时两态可能分叉（真仓此类文件名同时命中 N2 ⇒ 已红，不改变 rc；列入未覆盖面）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ── 常量与真源（逐条抄自 py 件头）──────────────────────────────────────────────

const cdnFullw = "\u3000\uff08\uff09\u3010\u3011\u300c\u300d\uff0c\u3002\u3001\uff1a\uff1b\uff01\uff1f\u201c\u201d"

var cdnDocishExt = []string{".md", ".markdown", ".mdx", ".tsv", ".csv", ".json", ".yaml", ".yml", ".py",
	".sh", ".rs", ".go", ".txt"}

var (
	cdnCapsStem = regexp.MustCompile(`^[A-Z][A-Z0-9_]{2,}$`)
	cdnDate8Re  = regexp.MustCompile(`-\d{8}`)
	cdnDate6Re  = regexp.MustCompile(`-\d{6}`)
	cdnVerMid   = regexp.MustCompile(`-v\d+(?:\.\d+)*-`)
	cdnVerTail  = regexp.MustCompile(`-v\d+(?:\.\d+)*$`)
	cdnNNPrefix = regexp.MustCompile(`^\d{2,3}-`)
	cdnCJK      = regexp.MustCompile("[\u4e00-\u9fff]")
	cdnLAT      = regexp.MustCompile(`[A-Za-z]`)
	cdnSegRe    = regexp.MustCompile(`[-_.\s]`)
)

// 扫描域排除名单：逐字照抄《清单-命名规范化》§0。
var (
	cdnExclDirs = []string{"node_modules", "target", "__pycache__", ".history", ".obsidian", "dist",
		".zerg", "zerg-wt", "venv", ".cargo", ".git"}
	cdnExclPrefix   = []string{".git/", "vendor/", "tools/ocr/venv/"}
	cdnFrozenPrefix = []string{}
)

type cdnRule struct{ rid, sev, what, src string }

var cdnRules = []cdnRule{
	{"N1", "FAIL", "①大小写：文件词干全为大写拉丁字母（≥3）", "《清单》§1 ①（与 mkdocs `index.md` 约定冲突；本卷 APFS 大小写不敏感）"},
	{"N2", "FAIL", "②空格/全角：名称含 U+0020 / U+3000 / 全角标点", "《清单》§1 ②"},
	{"N3", "FAIL", "③日期后缀：词干含 -YYYYMMDD 或 -YYYYMM", "《清单》§5 ③（日期移入 frontmatter updated_at）+ §5 ⑦（6 位日期）"},
	{"N4", "FAIL", "③版本段不在尾位：-vX.Y 其后还有分隔段", "《清单》§5 ③（`名-…-vX.Y[-YYYYMMDD]`，版本段移尾）"},
	{"N5", "WARN", "④前缀序号不一致：同层 NN- 与无前缀并存", "《清单》§5 ④（结构级 · 只登记不改 · 牵动 nav/白名单）"},
	{"N6", "FAIL", "⑥同目录同基名、扩展名不同（去最后一个扩展名后相同）", "《清单》§1 ⑥（K8 那类）+ §4.3 允许对"},
	{"N7", "WARN", "⑤中英紧邻：同一分隔段内汉字与拉丁字母直接相邻", "《清单》§5 ⑤（机械插 `-` 需人审）"},
}

// ── 例外表（A/B/C/D 四档 · 逐条抄自 py）──────────────────────────────────────

type cdnRow struct {
	id, typ, kind, name, reason, src, objFlat string
	rowNum                                    int
	rules                                     []string
	paths                                     []string
	pairs                                     [][2]string
	extKeys                                   []string
	extVals                                   [][]string
	docSets                                   [][]string
	machSets                                  [][]string
}

var cdnExemptA = []cdnRow{
	{id: "A01", rowNum: 2, typ: "④", kind: "prefix", rules: []string{"N5"},
		paths: []string{"docs/常青/", "docs/skills/"},
		reason: "两个正式面候选目录**无 NN- 前缀**，而 01-设计/02-调研/thunderbolt 有 ⇒ 结构级" +
			"（2026-09-19 分家：原列四项里的 `docs/调研/` `docs/03-评审/` 已迁出仓 ⇒ " +
			"从 paths 删除，只留仓内两项）",
		src:     "清单 §5 ④ · TSV 第 2 行（建议 = 不改 · 结构级，仅登记）",
		objFlat: "docs/常青/ , docs/skills/"},
	{id: "A02", rowNum: 3, typ: "①", kind: "path", rules: []string{"N1"},
		paths:  []string{"README.md"},
		reason: "行业约定文件；`publish/whitelist.txt` 公开面入口",
		src:    "清单 §4.2 · TSV 第 3 行（建议 = 不改）", objFlat: "README.md"},
	{id: "A03", rowNum: 5, typ: "⑥", kind: "pair", rules: []string{"N6"},
		pairs:  [][2]string{{"core/internal/compat", "compat"}},
		reason: "Go 包同名约定（compat.go + compat.json）",
		src:    "清单 §4.3 · TSV 第 5 行（建议 = 不改 · 登记为允许对）", objFlat: "core/internal/compat/compat"},
	{id: "A04", rowNum: 6, typ: "⑥", kind: "pair", rules: []string{"N6"},
		pairs:  [][2]string{{"core/internal/compat", "compat"}},
		reason: "同上（JSON 侧）",
		src:    "清单 §4.3 · TSV 第 6 行（同上）", objFlat: "core/internal/compat/compat"},
	{id: "A05", rowNum: 8, typ: "①", kind: "path", rules: []string{"N1"},
		paths:  []string{"AGENTS.md"},
		reason: "行业约定文件（agent 入口）；`publish/private-paths.txt:29` 明列",
		src:    "清单 §4.2 · TSV 第 8 行（建议 = 不改）", objFlat: "AGENTS.md"},
	{id: "A06", rowNum: 15, typ: "⑥", kind: "pair", rules: []string{"N6"},
		pairs:  [][2]string{{"scripts/docs", "edit-assert"}},
		reason: "无扩展可执行件；改名 = 断全部 `python3 scripts/docs/edit-assert` 调用方",
		src:    "清单 §4.3 · TSV 第 15 行（建议 = 不改 · 登记为允许对）", objFlat: "scripts/docs/edit-assert"},
	{id: "A07", rowNum: 16, typ: "⑥", kind: "pair", rules: []string{"N6"},
		pairs:  [][2]string{{"scripts/docs", "edit-assert"}},
		reason: "`.md` = 同一脚本的说明书",
		src:    "清单 §4.3 · TSV 第 16 行（同上）", objFlat: "scripts/docs/edit-assert"},
	{id: "A08", rowNum: 21, typ: "⑥", kind: "pair", rules: []string{"N6"},
		pairs:  [][2]string{{"tools", "doc_index_gen"}},
		reason: "`tools/*.md`（145 个）为工具说明书，是既有约定",
		src:    "清单 §4.3 · TSV 第 21 行（同上）", objFlat: "tools/doc_index_gen"},
	{id: "A09", rowNum: 22, typ: "⑥", kind: "pair", rules: []string{"N6"},
		pairs:  [][2]string{{"tools", "doc_index_gen"}},
		reason: "同上（实现侧）",
		src:    "清单 §4.3 · TSV 第 22 行（同上）", objFlat: "tools/doc_index_gen"},
	{id: "A10", rowNum: 23, typ: "⑥", kind: "pair", rules: []string{"N6"},
		pairs:  [][2]string{{"tools", "kb_docs_sync"}},
		reason: "同上（工具说明书）",
		src:    "清单 §4.3 · TSV 第 23 行（同上）", objFlat: "tools/kb_docs_sync"},
	{id: "A11", rowNum: 24, typ: "⑥", kind: "pair", rules: []string{"N6"},
		pairs:  [][2]string{{"tools", "kb_docs_sync"}},
		reason: "同上（实现侧）",
		src:    "清单 §4.3 · TSV 第 24 行（同上）", objFlat: "tools/kb_docs_sync"},
	{id: "A12", rowNum: 28, typ: "⑥", kind: "pair", rules: []string{"N6"},
		pairs:  [][2]string{{"scripts/gates", "mutate-scan"}},
		reason: "无扩展可执行件（同 edit-assert）",
		src:    "清单 §4.3 · TSV 第 28 行（同上）", objFlat: "scripts/gates/mutate-scan"},
	{id: "A13", rowNum: 29, typ: "⑥", kind: "pair", rules: []string{"N6"},
		pairs:  [][2]string{{"scripts/gates", "mutate-scan"}},
		reason: "`.md` = 同一脚本的说明书",
		src:    "清单 §4.3 · TSV 第 29 行（同上）", objFlat: "scripts/gates/mutate-scan"},
	{id: "A20", rowNum: 143, typ: "③", kind: "path", rules: []string{"N3", "N4"},
		paths:   []string{"docs/常青/白皮书-虫族经济模型-v0.1.md"},
		reason:  "版本号已在尾位，与本仓 `名-…-vX.Y[-YYYYMMDD]` 约定一致",
		src:     "清单 §5 · TSV 第 143 行（建议 = 不改 · 版本号已在尾位）",
		objFlat: "docs/常青/白皮书-虫族经济模型-v0.1.md"},
	{id: "A22", rowNum: 147, typ: "①", kind: "name", rules: []string{"N1"},
		paths:   []string{"LICENSE", "NOTICE", "SECURITY.md", "CONTRIBUTING.md", "THIRD_PARTY_LICENSES.md"},
		reason:  "公开面法定/治理文件，全大写是行业约定",
		src:     "清单 §5 · TSV 第 147 行（建议 = 不改 · 符合行业约定）",
		objFlat: "LICENSE , NOTICE , SECURITY.md , CONTRIBUTING.md , THIRD_PARTY_LICENSES.md"},
}

var cdnExemptB = []cdnRow{
	{id: "B01", kind: "reserved", rules: []string{"N1"},
		paths: []string{"README.md", "AGENTS.md", "SKILL.md", "VERSION", "CHANGELOG.md"},
		reason: "行业/工具约定保留名 —— **只在文档面之外生效**（docs/ 内的同名件照判）。2026-09-21 发布闸 + " +
			"`CHANGELOG.md`（Keep a Changelog 1.1.0 约定名 · P9 拍板「`CHANGELOG.md` 给机检」）",
		objFlat: "README.md , AGENTS.md , SKILL.md , VERSION , CHANGELOG.md"},
	{id: "B02", kind: "pair_ext", rules: []string{"N6"},
		extKeys: []string{"go", "Cargo"},
		extVals: [][]string{{".mod", ".sum"}, {".toml", ".lock"}},
		reason: "构建工具链约定（Go module / Cargo），不属文档命名面。" +
			"**休眠申报**：N6 的口径（至少一方是文档/脚本面扩展）已把它们挡在门外 ⇒ 本仓当前 0 命中",
		objFlat: "go/.mod/.sum , Cargo/.toml/.lock"},
	{id: "B03", kind: "pair_doc_machine", rules: []string{"N6"},
		docSets:  [][]string{{".md", ".markdown", ".mdx"}},
		machSets: [][]string{{".tsv", ".csv", ".json"}},
		reason: "人读件 + 机读件同基名（本仓既有形态：`清单-命名规范化.md/.tsv` = 设计件 + 机读件）。" +
			"**我方判断 · 与本脚本自身的差异已登记**：去掉这一条，该对会被判 ⑥（见 `scripts/gates/check-doc-name.md` §差异）",
		objFlat: ".md/.markdown/.mdx/.tsv/.csv/.json"},
}

var cdnExemptC = []cdnRow{
	{id: "C01", kind: "pair", rules: []string{"N6"}, pairs: [][2]string{{"scripts/gates", "check-doc-meta"}},
		reason:  "本轮 W2 交付：门脚本 + 说明书（同 `scripts/docs/edit-assert` 既有约定）· **待父代理批准**",
		objFlat: "scripts/gates/check-doc-meta"},
	{id: "C02", kind: "pair", rules: []string{"N6"}, pairs: [][2]string{{"scripts/gates", "check-doc-name"}},
		reason:  "同上（本文档脚本 + 说明书）· **待父代理批准**",
		objFlat: "scripts/gates/check-doc-name"},
	{id: "C03", kind: "pair", rules: []string{"N6"}, pairs: [][2]string{{"scripts/gates", "check-doc-freshness"}},
		reason: "文档体系 P1 同批交付的第三条门脚本 + 说明书（`check-doc-freshness.{py,md}`）——" +
			"与 C01/C02、A06/A07、A12/A13 同一形态 · **待父代理批准**",
		objFlat: "scripts/gates/check-doc-freshness"},
	{id: "C04", kind: "pair", rules: []string{"N6"}, pairs: [][2]string{{"scripts/gates", "precommit-gates"}},
		reason: "`scripts/gates/precommit-gates.md` 由**路 D**（门禁挂接 Q14）2026-09-18 20:43 新增，" +
			"形态 = 「门脚本 + 说明书」（同 C01–C03 / A06/A07 / A12/A13）⇒ 登记为**允许对**。" +
			"**已拍 2026-09-18（Mr2109）**：认下 C04（拍板来源 = 文档体系 P1 第三批 拍板单 §⑥）。" +
			"撤法：删本行并把 `EXEMPT_C_EXPECTED` 4→3 ⇒ 该对判红照实回来",
		objFlat: "scripts/gates/precommit-gates"},
	{id: "C05", kind: "pair", rules: []string{"N6"}, pairs: [][2]string{{"scripts/gates", "check-gate-coverage"}},
		reason: "**门①（门覆盖自检）**的「门脚本 + 说明书」对（`check-gate-coverage.{py,md}`）——" +
			"由第二波（2026-09-18）新建并挂进 `scripts/gates/precommit-gates.sh` 的 `gates` scope（模式 `tri`）。" +
			"形态同 C01–C04 / A06/A07 / A12/A13 ⇒ 登记为**允许对**。" +
			"撤法：删本行并把 `EXEMPT_C_EXPECTED` 7→6 ⇒ 该对判红照实回来",
		objFlat: "scripts/gates/check-gate-coverage"},
	{id: "C06", kind: "pair", rules: []string{"N6"}, pairs: [][2]string{{"scripts/gates", "check-version-sources"}},
		reason: "**门②（版本单一真源）**的「门脚本 + 说明书」对（`check-version-sources.{py,md}`）——" +
			"同 C05（第二波新建 · 挂 `gates` scope · 模式 `tri`）。" +
			"撤法：删本行并把 `EXEMPT_C_EXPECTED` 7→6 ⇒ 该对判红照实回来",
		objFlat: "scripts/gates/check-version-sources"},
	{id: "C07", kind: "pair", rules: []string{"N6"}, pairs: [][2]string{{"scripts/gates", "check-wired-scripts"}},
		reason: "**门③（脚本接线自检）**的「门脚本 + 说明书」对（`check-wired-scripts.{py,md}`）——" +
			"同 C05（第二波新建 · 挂 `gates` scope · 模式 `tri-report` 只报告）。" +
			"撤法：删本行并把 `EXEMPT_C_EXPECTED` 7→6 ⇒ 该对判红照实回来",
		objFlat: "scripts/gates/check-wired-scripts"},
	{id: "C08", kind: "pair", rules: []string{"N6"}, pairs: [][2]string{{"scripts/gates", "check-dangling-refs"}},
		reason: "**门⑮（主仓悬空引用 `docs/…` 指向不存在）**的「门脚本 + 说明书」对" +
			"（`check-dangling-refs.{py,md}`）—— 同 C05（2026-09-22 新建 · 挂 `docs` scope · 模式 `tri-report` 只报告 · 同时进快速档）。" +
			"撤法：删本行并把 `EXEMPT_C_EXPECTED` 8→7 ⇒ 该对判红照实回来",
		objFlat: "scripts/gates/check-dangling-refs"},
	{id: "C09", kind: "pair", rules: []string{"N6"}, pairs: [][2]string{{"scripts/gates", "check-git-env-hygiene"}},
		reason: "**hook / 门禁脚本对外仓·合成仓的 git 调用先清环境**（`W-43` / `R-07`）的「门脚本 + 说明书」对" +
			"（`check-git-env-hygiene.{py,md}`）—— 同 C05（2026-09-24 波11 序125 新建 · **单跑档、本笔不挂门**" +
			"（本波件面未含 `precommit-gates.sh` ⇒ 不在 `add_step` 里）· 默认报告档 `tri-report`）。" +
			"撤法：删本行并把 `EXEMPT_C_EXPECTED` 9→8 ⇒ 该对判红照实回来",
		objFlat: "scripts/gates/check-git-env-hygiene"},
}

var cdnExemptD = []cdnRow{
	{id: "D01", kind: "prefix_name", rules: []string{"N1"},
		paths: []string{"docs/"}, name: "README.md",
		reason: "`docs/` 内的 `README.md` = **语言层入口件 / 重定向存根**（`docs/README.md` 是并入 " +
			"`docs/index-nav.md` 后按零删除保留的旧路径；`docs/{zh,en,en/_drafts,site}/README.md` " +
			"是目录律入口）。README.md 属**行业约定保留名**（与 B01 同源）⇒ 只在文档面之内开这一个" +
			"口子（AGENTS.md / SKILL.md / VERSION 在 docs/ 内照判，K8 要的那条没被放松）",
		src:     "父代理 2026-09-18 拍板 · 本说明书 §差异 6/§例外表 D 档（(b) 路：加规则级豁免登记，不改文件名）",
		objFlat: "docs/ + 基名 README.md"},
	{id: "D02", kind: "path", rules: []string{"N3"},
		paths: []string{"bin/_history/zerg-core.prev-20260912.bak"},
		reason: "备件残片：`bin/` 被 `.gitignore` 忽略、**未纳入版本控制** ⇒ `git mv` 不可用" +
			"（本路硬规矩：禁 `mv`/`rm`）；日期是它**作为备件的身份**（哪个二进制的前身）",
		src:     "父代理 2026-09-18 拍板 · 本说明书 §差异 7（处置形态同《清单》A19/A21「移出仓」= 父代理动作）",
		objFlat: "bin/_history/zerg-core.prev-20260912.bak"},
}

var cdnExemptAll = func() []cdnRow {
	out := []cdnRow{}
	out = append(out, cdnExemptA...)
	out = append(out, cdnExemptB...)
	out = append(out, cdnExemptC...)
	out = append(out, cdnExemptD...)
	return out
}()

var cdnExemptExpected = map[string]int{"A": 15, "B": 3, "C": 9, "D": 2}

// ── posix 路径原语（照 CPython posixpath 直译，✗ 不用 filepath.*）────────────────

func cdnPyJoin(parts ...string) string {
	result := ""
	for _, b := range parts {
		switch {
		case strings.HasPrefix(b, "/"):
			result = b
		case result == "" || strings.HasSuffix(result, "/"):
			result += b
		default:
			result += "/" + b
		}
	}
	return result
}

func cdnNormpath(path string) string {
	if path == "" {
		return "."
	}
	initialSlashes := strings.HasPrefix(path, "/")
	twoSlashes := strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///")
	out := []string{}
	for _, c := range strings.Split(path, "/") {
		switch c {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 && out[len(out)-1] != ".." {
				out = out[:len(out)-1]
			} else if !initialSlashes {
				out = append(out, "..")
			}
		default:
			out = append(out, c)
		}
	}
	res := strings.Join(out, "/")
	if initialSlashes {
		if twoSlashes {
			res = "//" + res
		} else {
			res = "/" + res
		}
	}
	if res == "" {
		return "."
	}
	return res
}

func cdnAbs(p string) string {
	if strings.HasPrefix(p, "/") {
		return cdnNormpath(p)
	}
	cwd, _ := os.Getwd()
	return cdnNormpath(cdnPyJoin(cwd, p))
}

func cdnSplit(p string) (string, string) {
	i := strings.LastIndex(p, "/") + 1
	return p[:i], p[i:]
}

func cdnDirname(p string) string {
	head, _ := cdnSplit(p)
	if head == "" {
		return ""
	}
	head = strings.TrimRight(head, "/")
	if head == "" {
		return "/"
	}
	return head
}

func cdnBase(p string) string {
	_, tail := cdnSplit(p)
	return tail
}

func cdnSplitext(base string) (string, string) {
	i := strings.LastIndex(base, ".")
	if i <= 0 {
		return base, ""
	}
	return base[:i], base[i:]
}

func cdnRel(path, start string) string {
	p := cdnAbs(path)
	s := cdnAbs(start)
	comp := func(x string) []string {
		f := []string{}
		for _, c := range strings.Split(x, "/") {
			if c != "" {
				f = append(f, c)
			}
		}
		return f
	}
	pp, sp := comp(p), comp(s)
	i := 0
	for i < len(sp) && i < len(pp) && sp[i] == pp[i] {
		i++
	}
	rel := []string{}
	for j := i; j < len(sp); j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pp[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

func cdnIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func cdnEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// cdnDevdocsRoot —— 开发文档面根（与 py devdocs_root 逐字同构）。
func cdnDevdocsRoot(repoRoot string) string {
	if env := os.Getenv("ZERG_DEVDOCS_ROOT"); env != "" {
		return cdnAbs(env)
	}
	sibling := cdnEnvOr("ZERG_DOCS_ALT", cdnEnvOr("ZERG_DOCS",
		cdnPyJoin(cdnDirname(cdnAbs(repoRoot)), "Zerg-内部文档")))
	return cdnAbs(cdnPyJoin(sibling, "项目文档", "v2.5.10"))
}

func cdnDefaultRepoRoot() string {
	cwd, _ := os.Getwd()
	return cdnNormpath(cwd)
}

// ── py 字符串原语 ─────────────────────────────────────────────────────────────

// cdnPyRepr —— 复刻 py 的 %r（字符串），够用即可。
func cdnPyRepr(s string) string {
	hasSingle := strings.Contains(s, "'")
	hasDouble := strings.Contains(s, "\"")
	q := byte('\'')
	if hasSingle && !hasDouble {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r == rune(q) {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else if r < 0x20 {
				b.WriteString(fmt.Sprintf(`\x%02x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(q)
	return b.String()
}

// ── 例外匹配 ──────────────────────────────────────────────────────────────────

func cdnContains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func cdnHasPrefixAny(s string, ps []string) bool {
	for _, p := range ps {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func cdnPairIn(xs [][2]string, k [2]string) bool {
	for _, x := range xs {
		if x == k {
			return true
		}
	}
	return false
}

// cdnExemptRowFor —— 返回命中的 A/B/C/D 档豁免行（nil = 不豁免）。kind = file|dir|pair。
func cdnExemptRowFor(rel, rule, kind string, pairKey *[2]string, stem, base string) *cdnRow {
	relN := rel
	if kind == "dir" && !strings.HasSuffix(rel, "/") {
		relN = rel + "/"
	}
	for i := range cdnExemptAll {
		row := &cdnExemptAll[i]
		if !cdnContains(row.rules, rule) {
			continue
		}
		switch row.kind {
		case "path":
			if (kind == "file" && cdnContains(row.paths, rel)) || (kind == "dir" && cdnContains(row.paths, relN)) {
				return row
			}
		case "prefix":
			if (kind == "file" || kind == "dir") && cdnHasPrefixAny(relN, row.paths) {
				return row
			}
		case "name":
			if kind == "file" && cdnContains(row.paths, base) {
				return row
			}
		case "prefix_name":
			if kind == "file" && base == row.name && cdnHasPrefixAny(rel, row.paths) {
				return row
			}
		case "reserved":
			if kind == "file" && cdnContains(row.paths, base) && !strings.HasPrefix(rel, "docs/") {
				return row
			}
		case "pair":
			if kind == "pair" && pairKey != nil && cdnPairIn(row.pairs, *pairKey) {
				return row
			}
		}
	}
	return nil
}

func cdnSubset(s map[string]bool, want []string) bool {
	w := map[string]bool{}
	for _, x := range want {
		w[x] = true
	}
	for k := range s {
		if !w[k] {
			return false
		}
	}
	return true
}

func cdnIntersects(s map[string]bool, want []string) bool {
	for _, x := range want {
		if s[x] {
			return true
		}
	}
	return false
}

func cdnPairExtExempt(stem string, exts []string) *cdnRow {
	s := map[string]bool{}
	for _, e := range exts {
		s[e] = true
	}
	for i := range cdnExemptB {
		row := &cdnExemptB[i]
		switch row.kind {
		case "pair_ext":
			for j, key := range row.extKeys {
				if key == stem && cdnSubset(s, row.extVals[j]) {
					return row
				}
			}
		case "pair_doc_machine":
			for j := range row.docSets {
				if cdnIntersects(s, row.docSets[j]) && cdnIntersects(s, row.machSets[j]) && len(s) == 2 {
					return row
				}
			}
		}
	}
	return nil
}

// ── 扫描 ──────────────────────────────────────────────────────────────────────

type cdnEntry struct{ rel, kind, ap string }

func cdnInExclDirs(name string) bool {
	return cdnContains(cdnExclDirs, name)
}

func cdnWalk(root string, targets []string) []cdnEntry {
	out := []cdnEntry{}
	seen := map[string]bool{}
	add := func(rel, kind, ap string) {
		if rel == "" || rel == "." || seen[rel] {
			return
		}
		seen[rel] = true
		out = append(out, cdnEntry{rel, kind, ap})
	}
	var walkDir func(t string)
	walkDir = func(t string) {
		f, err := os.Open(t)
		if err != nil {
			return
		}
		ents, _ := f.ReadDir(-1)
		f.Close()
		rel := cdnRel(t, root)
		relp := ""
		if rel != "." {
			relp = rel + "/"
		}
		if relp != "" && cdnHasPrefixAny(relp, cdnExclPrefix) {
			return
		}
		dirs, files := []string{}, []string{}
		for _, e := range ents {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			full := cdnPyJoin(t, name)
			if cdnIsDir(full) {
				if cdnInExclDirs(name) {
					continue
				}
				dirs = append(dirs, name)
			} else {
				files = append(files, name)
			}
		}
		sort.Strings(dirs)
		sort.Strings(files)
		for _, d := range dirs {
			add(relp+d, "dir", cdnPyJoin(t, d))
		}
		for _, fn := range files {
			add(relp+fn, "file", cdnPyJoin(t, fn))
		}
		for _, d := range dirs {
			full := cdnPyJoin(t, d)
			if li, err := os.Lstat(full); err == nil && li.Mode()&os.ModeSymlink != 0 {
				continue
			}
			walkDir(full)
		}
	}
	for _, t := range targets {
		if st, err := os.Stat(t); err == nil && !st.IsDir() {
			add(cdnRel(t, root), "file", cdnAbs(t))
			continue
		}
		if st, err := os.Stat(t); err == nil && st.IsDir() {
			add(cdnRel(t, root), "dir", cdnAbs(t))
			walkDir(t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out
}

// ── 判据 ──────────────────────────────────────────────────────────────────────

type cdnHit struct {
	rule, sev, msg string
	row            *cdnRow
}

func cdnSearchDate(stem string) string {
	for _, re := range []*regexp.Regexp{cdnDate8Re, cdnDate6Re} {
		for _, loc := range re.FindAllStringIndex(stem, -1) {
			end := loc[1]
			if end == len(stem) || stem[end] == '-' || stem[end] == '.' {
				return stem[loc[0]:loc[1]]
			}
		}
	}
	return ""
}

func cdnJudgeName(rel, kind string) []cdnHit {
	base := cdnBase(rel)
	stem, _ := cdnSplitext(base)
	hits := []cdnHit{}
	add := func(rule, msg string) {
		// 档位（缺口 GAP-20260927-63 / GAP-20260926-165）：**件面**取 RULES 声明档
		// （N5/N7 声明 WARN ⇒ 件面命中不再落 FAIL 进 rc）；**目录面一律 WARN**（结构级 ·
		// 只登记不改 —— 守住本件 `cdnListRules` 印的档位口径与《清单》§5 ④）。与 py 孪生件
		// `scripts/gates/check-doc-name.py` 的 `RULE_SEV` 选取逐条同义（✗ 不许只改一侧：
		// 双实现件语义分叉 ⇒ 对拍件当场红）。
		sev := "WARN"
		if kind == "file" {
			sev = "FAIL"
			for _, r := range cdnRules {
				if r.rid == rule {
					sev = r.sev
					break
				}
			}
		}
		row := cdnExemptRowFor(rel, rule, kind, nil, stem, base)
		hits = append(hits, cdnHit{rule, sev, msg, row})
	}
	if cdnCapsStem.MatchString(stem) {
		add("N1", fmt.Sprintf("词干 %s 全为大写拉丁字母", cdnPyRepr(stem)))
	}
	badSet := map[rune]bool{}
	for _, c := range base {
		if strings.ContainsRune(cdnFullw, c) || c == ' ' {
			badSet[c] = true
		}
	}
	if len(badSet) > 0 {
		rs := []rune{}
		for c := range badSet {
			rs = append(rs, c)
		}
		sort.Slice(rs, func(i, j int) bool { return rs[i] < rs[j] })
		us := []string{}
		for _, c := range rs {
			us = append(us, fmt.Sprintf("U+%04X", c))
		}
		add("N2", "名称含 "+strings.Join(us, " / "))
	}
	if md := cdnSearchDate(stem); md != "" {
		add("N3", fmt.Sprintf("词干含日期后缀 %s ⇒ 日期应移入 frontmatter `updated_at`", cdnPyRepr(md)))
	}
	if cdnVerMid.MatchString(stem) && !cdnVerTail.MatchString(stem) {
		add("N4", fmt.Sprintf("版本段 %s 不在尾位 ⇒ 应移到尾部（`名-…-vX.Y[-YYYYMMDD]`）", cdnPyRepr(cdnVerMid.FindString(stem))))
	}
	segs := []string{}
	for _, s := range cdnSegRe.Split(stem, -1) {
		if s != "" {
			segs = append(segs, s)
		}
	}
	for _, seg := range segs {
		joined := false
		rs := []rune(seg)
		for i := 0; i+1 < len(rs); i++ {
			a, b := rs[i], rs[i+1]
			if (cdnCJK.MatchString(string(a)) && cdnLAT.MatchString(string(b))) ||
				(cdnLAT.MatchString(string(a)) && cdnCJK.MatchString(string(b))) {
				add("N7", fmt.Sprintf("段 %s 内汉字与拉丁字母直接相邻（%s%s）", cdnPyRepr(seg), string(a), string(b)))
				joined = true
				break
			}
		}
		if joined {
			break
		}
	}
	return hits
}

type cdnPairGroup struct {
	keyDir, keyStem string
	members         []string
	exts            []string
	row             *cdnRow
}

func cdnJudgeSameStem(entries []cdnEntry) []cdnPairGroup {
	type mem struct{ rel, ext string }
	idx := map[string]int{}
	order := []string{}
	groups := []([]mem){}
	for _, e := range entries {
		if e.kind != "file" {
			continue
		}
		dn := cdnDirname(e.rel)
		base := cdnBase(e.rel)
		stem, ext := cdnSplitext(base)
		k := dn + "\x00" + stem
		if _, ok := idx[k]; !ok {
			idx[k] = len(order)
			order = append(order, k)
			groups = append(groups, []mem{})
		}
		groups[idx[k]] = append(groups[idx[k]], mem{e.rel, ext})
	}
	type outKey struct{ dn, stem string }
	keys := []outKey{}
	byKey := map[string]int{}
	for i, k := range order {
		parts := strings.SplitN(k, "\x00", 2)
		byKey[k] = i
		keys = append(keys, outKey{parts[0], parts[1]})
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].dn != keys[j].dn {
			return keys[i].dn < keys[j].dn
		}
		return keys[i].stem < keys[j].stem
	})
	res := []cdnPairGroup{}
	for _, kk := range keys {
		k := kk.dn + "\x00" + kk.stem
		ms := groups[byKey[k]]
		extSet := map[string]bool{}
		for _, m := range ms {
			extSet[m.ext] = true
		}
		exts := []string{}
		for x := range extSet {
			exts = append(exts, x)
		}
		sort.Strings(exts)
		if len(ms) < 2 || len(exts) < 2 {
			continue
		}
		docish := false
		for _, x := range exts {
			if cdnContains(cdnDocishExt, x) {
				docish = true
				break
			}
		}
		if !docish {
			continue
		}
		members := []string{}
		for _, m := range ms {
			members = append(members, m.rel)
		}
		pk := [2]string{kk.dn, kk.stem}
		row := cdnExemptRowFor(members[0], "N6", "pair", &pk, "", "")
		if row == nil && len(exts) > 0 {
			row = cdnPairExtExempt(kk.stem, exts)
		}
		res = append(res, cdnPairGroup{kk.dn, kk.stem, members, exts, row})
	}
	return res
}

type cdnMixHit struct {
	dn        string
	nPre, nNo int
	row       *cdnRow
}

func cdnJudgePrefixMix(entries []cdnEntry) []cdnMixHit {
	byDir := map[string]*[2][]string{}
	children := map[string][]string{}
	order := []string{}
	for _, e := range entries {
		dn := cdnDirname(e.rel)
		base := cdnBase(e.rel)
		if _, ok := byDir[dn]; !ok {
			byDir[dn] = &[2][]string{}
			order = append(order, dn)
		}
		if e.kind == "dir" {
			children[dn] = append(children[dn], e.rel+"/")
		}
		i := 1
		if cdnNNPrefix.MatchString(base) {
			i = 0
		}
		byDir[dn][i] = append(byDir[dn][i], base)
	}
	sort.Strings(order)
	res := []cdnMixHit{}
	for _, dn := range order {
		pre, no := byDir[dn][0], byDir[dn][1]
		if len(pre) >= 2 && len(no) >= 2 {
			var row *cdnRow
			if dn != "" {
				row = cdnExemptRowFor(dn+"/", "N5", "dir", nil, "", "")
			}
			if row == nil {
				for _, ch := range children[dn] {
					row = cdnExemptRowFor(ch, "N5", "dir", nil, "", "")
					if row != nil {
						break
					}
				}
			}
			disp := dn
			if disp == "" {
				disp = "<仓根>"
			}
			res = append(res, cdnMixHit{disp, len(pre), len(no), row})
		}
	}
	return res
}

// ── 报告 ──────────────────────────────────────────────────────────────────────

type cdnKV struct {
	k string
	v int
}

type cdnReport struct {
	entries       int
	perRule       map[string]int
	examples      map[string][]string
	exemptHits    []cdnKV
	live          map[string]int
	frozen        map[string]int
	perRuleLive   map[string]int
	perRuleFrozen map[string]int
}

func cdnRun(root string, targets []string, countFrozen bool) (*cdnReport, string) {
	entries := cdnWalk(root, targets)
	if len(entries) == 0 {
		return nil, "扫描域为空（一个文件/目录都没有）"
	}
	hasFile := false
	for _, e := range entries {
		if e.kind == "file" {
			hasFile = true
			break
		}
	}
	if !hasFile {
		return nil, "扫描域里没有任何文件（只有目录名 ⇒ 判不了）"
	}
	rep := &cdnReport{
		entries:       len(entries),
		perRule:       map[string]int{},
		examples:      map[string][]string{},
		live:          map[string]int{"FAIL": 0, "WARN": 0},
		frozen:        map[string]int{"FAIL": 0, "WARN": 0},
		perRuleLive:   map[string]int{},
		perRuleFrozen: map[string]int{},
	}
	incExempt := func(id string) {
		for i := range rep.exemptHits {
			if rep.exemptHits[i].k == id {
				rep.exemptHits[i].v++
				return
			}
		}
		rep.exemptHits = append(rep.exemptHits, cdnKV{id, 1})
	}
	isFrozen := func(rel string) bool { return cdnHasPrefixAny(rel, cdnFrozenPrefix) }

	for _, e := range entries {
		fr := isFrozen(e.rel)
		for _, h := range cdnJudgeName(e.rel, e.kind) {
			rep.perRule[h.rule]++
			if h.row != nil {
				incExempt(h.row.id)
				continue
			}
			bucket := rep.live
			if fr {
				bucket = rep.frozen
			}
			bucket[h.sev]++
			tgt := rep.perRuleLive
			if fr {
				tgt = rep.perRuleFrozen
			}
			tgt[h.rule]++
			ex := ""
			if fr {
				ex = "[冻结] "
			}
			rep.examples[h.rule] = append(rep.examples[h.rule], fmt.Sprintf("%s%s → %s", ex, e.rel, h.msg))
		}
	}
	for _, g := range cdnJudgeSameStem(entries) {
		fr := false
		for _, m := range g.members {
			if isFrozen(m) {
				fr = true
				break
			}
		}
		rep.perRule["N6"]++
		if g.row != nil {
			incExempt(g.row.id)
			continue
		}
		bucket := rep.live
		if fr {
			bucket = rep.frozen
		}
		bucket["FAIL"]++
		tgt := rep.perRuleLive
		if fr {
			tgt = rep.perRuleFrozen
		}
		tgt["N6"]++
		ex := ""
		if fr {
			ex = "[冻结] "
		}
		dn := g.keyDir
		if dn == "" {
			dn = "<仓根>"
		}
		rep.examples["N6"] = append(rep.examples["N6"],
			fmt.Sprintf("%s%s/%s → 同基名不同扩展 %s", ex, dn, g.keyStem, strings.Join(g.exts, "/")))
	}
	for _, p := range cdnJudgePrefixMix(entries) {
		rep.perRule["N5"]++
		if p.row != nil {
			incExempt(p.row.id)
			continue
		}
		rep.live["WARN"]++
		rep.perRuleLive["N5"]++
		rep.examples["N5"] = append(rep.examples["N5"],
			fmt.Sprintf("%s → 同层混用 NN- 前缀（前缀 %d · 无前缀 %d）", p.dn, p.nPre, p.nNo))
	}
	if countFrozen {
		rep.live["FAIL"] += rep.frozen["FAIL"]
		rep.live["WARN"] += rep.frozen["WARN"]
		for k, v := range rep.perRuleFrozen {
			rep.perRuleLive[k] += v
		}
	}
	return rep, ""
}

func cdnPrintReport(w io.Writer, rep *cdnReport, maxExamples int) {
	fmt.Fprintf(w, "扫描域：%d 个文件/目录（排除名单同《清单》§0）\n", rep.entries)
	fmt.Fprintf(w, "  不合规 %d（冻结区 %d）· 告警 %d（冻结区 %d）\n",
		rep.live["FAIL"], rep.frozen["FAIL"], rep.live["WARN"], rep.frozen["WARN"])
	fmt.Fprintln(w, "  ★ 仓内冻结区：无 —— `docs/项目文档/` 已于 2026-09-26「批3」分家至仓外 "+
		"`Zerg-内部文档/项目文档/`（仓内零命中 ⇒ 条目已删，处置同 2026-09-19 的 `docs/issues/`）"+
		"⇒ 冻结计数恒 0/0（`--count-frozen` 机制保留）")
	for _, r := range cdnRules {
		n := rep.perRule[r.rid]
		if n == 0 {
			continue
		}
		fmt.Fprintf(w, "  %s [%s] %s —— 命中 %d（可改面 %d · 冻结区 %d）\n",
			r.rid, r.sev, r.what, n, rep.perRuleLive[r.rid], rep.perRuleFrozen[r.rid])
		ex := rep.examples[r.rid]
		for i, line := range ex {
			if i >= maxExamples {
				break
			}
			fmt.Fprintf(w, "      · %s\n", line)
		}
		if n > maxExamples {
			fmt.Fprintf(w, "      · …另 %d 条（--max-examples 调整）\n", n-maxExamples)
		}
	}
	if len(rep.exemptHits) == 0 {
		fmt.Fprintln(w, "  豁免命中（逐条可查 · --list-exempt）：无")
	} else {
		kvs := append([]cdnKV{}, rep.exemptHits...)
		sort.Slice(kvs, func(i, j int) bool { return kvs[i].k < kvs[j].k })
		parts := []string{}
		for _, kv := range kvs {
			parts = append(parts, fmt.Sprintf("%s×%d", kv.k, kv.v))
		}
		fmt.Fprintln(w, "  豁免命中（逐条可查 · --list-exempt）："+strings.Join(parts, " · "))
	}
}

func cdnDecide(rep *cdnReport) (int, string) {
	if rep.live["FAIL"] > 0 {
		return 1, fmt.Sprintf("FAIL：%d 个不合规命中（只报告不改）", rep.live["FAIL"])
	}
	sum := 0
	for _, kv := range rep.exemptHits {
		sum += kv.v
	}
	return 0, fmt.Sprintf("OK：无不合规命中（告警 %d · 已豁免 %d）", rep.live["WARN"], sum)
}

func cdnListRules(w io.Writer) {
	fmt.Fprintln(w, "check-doc-name.py 规则表（来源 = 《清单-命名规范化》§1 七类 + §2 风险档 + §5 处置）")
	for _, r := range cdnRules {
		fmt.Fprintf(w, "  %-3s [%-4s] %s\n", r.rid, r.sev, r.what)
		fmt.Fprintf(w, "       出处：%s\n", r.src)
	}
	fmt.Fprintln(w, "  档位口径：**文件名**命中才计入退码；**目录名**命中一律 WARN（结构级 · 只登记不改）；")
	fmt.Fprintln(w, "            N5/N7 为 WARN；仓内冻结区**已无**（`docs/项目文档/` 2026-09-26 批3 分家至仓外"+
		" `Zerg-内部文档/项目文档/` ⇒ FROZEN_PREFIX 已删；原第二项 `docs/issues/` 2026-09-19 同法已删）。")
	fmt.Fprintln(w, "  退码：0 全绿 · 1 有不合规项 · 2 不给结论（优先级 2>1>0）。")
}

func cdnListExempt(w io.Writer) int {
	fmt.Fprintln(w, "例外表（四档 · 逐条给理由与出处）")
	fmt.Fprintf(w, "A 档 · 《清单-命名规范化》已登记的「不改」条目：%d 条（期望 %d）\n", len(cdnExemptA), cdnExemptExpected["A"])
	for _, row := range cdnExemptA {
		fmt.Fprintf(w, "  %s [%s] 类型 %s · TSV 第 %d 行 · 形态 %s\n", row.id, strings.Join(row.rules, "+"), row.typ, row.rowNum, row.kind)
		fmt.Fprintf(w, "      对象：%s\n", row.objFlat)
		fmt.Fprintf(w, "      理由：%s\n", row.reason)
		fmt.Fprintf(w, "      出处：%s\n", row.src)
	}
	fmt.Fprintf(w, "B 档 · 规则级规范豁免：%d 条（期望 %d）\n", len(cdnExemptB), cdnExemptExpected["B"])
	for _, row := range cdnExemptB {
		fmt.Fprintf(w, "  %s [%s] 形态 %s\n      对象：%s\n      理由：%s\n", row.id, strings.Join(row.rules, "+"), row.kind, row.objFlat, row.reason)
	}
	fmt.Fprintf(w, "C 档 · 新增登记（C01–C03 待批准 · C04 已拍 2026-09-18 · C05–C07 第二波任务书登记 2026-09-18 · "+
		"C08 2026-09-22（门⑮ 主仓悬空引用）登记 · C09 2026-09-24（波11 序125 `W-43` hook git 环境）登记）："+
		"%d 条（期望 %d）\n", len(cdnExemptC), cdnExemptExpected["C"])
	for _, row := range cdnExemptC {
		fmt.Fprintf(w, "  %s [%s] 形态 %s\n      对象：%s\n      理由：%s\n", row.id, strings.Join(row.rules, "+"), row.kind, row.objFlat, row.reason)
	}
	fmt.Fprintf(w, "D 档 · 本轮 A 路拍板登记（登记不改）：%d 条（期望 %d）\n", len(cdnExemptD), cdnExemptExpected["D"])
	for _, row := range cdnExemptD {
		fmt.Fprintf(w, "  %s [%s] 形态 %s\n      对象：%s\n      理由：%s\n      出处：%s\n",
			row.id, strings.Join(row.rules, "+"), row.kind, row.objFlat, row.reason, row.src)
	}
	return 0
}

// ── JSON 输出（逐字复刻 py json.dumps(ensure_ascii=False, indent=2)）─────────────

func cdnJSONStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func cdnJSONIntObjOrdered(order []string, m map[string]int) string {
	if len(order) == 0 {
		return "{}"
	}
	parts := []string{}
	for _, k := range order {
		parts = append(parts, "    "+cdnJSONStr(k)+": "+strconv.Itoa(m[k]))
	}
	return "{\n" + strings.Join(parts, ",\n") + "\n  }"
}

func cdnJSONIntObjMap(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return cdnJSONIntObjOrdered(keys, m)
}

func cdnPrintJSON(w io.Writer, rc int, verdict string, rep *cdnReport) {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(fmt.Sprintf("  \"rc\": %d,\n", rc))
	b.WriteString("  \"verdict\": " + cdnJSONStr(verdict) + ",\n")
	b.WriteString(fmt.Sprintf("  \"entries\": %d,\n", rep.entries))
	b.WriteString("  \"live\": " + cdnJSONIntObjOrdered([]string{"FAIL", "WARN"}, rep.live) + ",\n")
	b.WriteString("  \"frozen\": " + cdnJSONIntObjOrdered([]string{"FAIL", "WARN"}, rep.frozen) + ",\n")
	order := []string{}
	m := map[string]int{}
	for _, kv := range rep.exemptHits {
		order = append(order, kv.k)
		m[kv.k] = kv.v
	}
	b.WriteString("  \"exempt_hits\": " + cdnJSONIntObjOrdered(order, m) + ",\n")
	b.WriteString("  \"per_rule\": " + cdnJSONIntObjMap(rep.perRule) + ",\n")
	b.WriteString("  \"per_rule_live\": " + cdnJSONIntObjMap(rep.perRuleLive) + ",\n")
	b.WriteString("  \"per_rule_frozen\": " + cdnJSONIntObjMap(rep.perRuleFrozen) + "\n")
	b.WriteString("}\n")
	io.WriteString(w, b.String())
}

// ── 自检（成对负控三层：好件必绿 / 坏件必红 / 缺件必 rc=2）─────────────────────────
//
// ★ 差异面：py 的元自检（⑫）注入自身源码副本再起子进程 —— 本件无「注入源码副本」语境 ⇒ 未搬该格（见件头 ③）。

var (
	cdnGoodFiles = [][2]string{
		{"docs/01-设计/设计-文档体系-v1.0.md", "# x\n"},
		{"docs/调研/调研-Hermes-与虫族对话机制对比.md", "# x\n"},
		{"docs/zh/模块-主控.md", "# x\n"},
		{"README.md", "# x\n"},
		{"scripts/docs/edit-assert", "#!/usr/bin/env python3\n"},
		{"scripts/docs/edit-assert.md", "# 说明书\n"},
		{"docs/项目文档/v2.5.10/01-模块-主控.md", "# x\n"},
	}
	cdnBadFiles = [][2]string{
		{"docs/INDEX.md", "N1"},
		{"docs/研究 - 自我纠错.md", "N2"},
		{"docs/01-设计/设计-文档体系-20260918.md", "N3"},
		{"docs/02-调研/调研-虫族同类项目与定位-202608.md", "N3"},
		{"docs/01-设计/设计-v2.5.10-卵功能与对话机制.md", "N4"},
		{"docs/01-设计/设计-文档体系.md", ""},
	}
	cdnBadDirs = [][2]string{
		{"docs/调研/协作骨架vX-核验/", "N7"},
		{"docs/新目录-20260918/", "N3"},
	}
	cdnBadPairs     = [][3]string{{"docs/01-设计/设计-文档体系.md", "docs/01-设计/设计-文档体系.py", "N6"}}
	cdnBadPrefixMix = "docs/混用目录"
)

func cdnWriteFile(path, text string) {
	_ = os.MkdirAll(cdnDirname(path), 0o755)
	f, err := os.Create(path)
	if err != nil {
		return
	}
	_, _ = f.WriteString(text)
	_ = f.Close()
}

func cdnTreeSha(dir string) string {
	h := sha256.New()
	var rec func(string)
	rec = func(d string) {
		f, err := os.Open(d)
		if err != nil {
			return
		}
		ents, _ := f.ReadDir(-1)
		f.Close()
		names := []string{}
		for _, e := range ents {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, n := range names {
			p := cdnPyJoin(d, n)
			if cdnIsDir(p) && !cdnIsSymlink(p) {
				rec(p)
				continue
			}
			rel := cdnRel(p, dir)
			h.Write([]byte(rel))
			if data, err := os.ReadFile(p); err == nil {
				h.Write(data)
			}
		}
	}
	rec(dir)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func cdnIsSymlink(p string) bool {
	li, err := os.Lstat(p)
	return err == nil && li.Mode()&os.ModeSymlink != 0
}

func cdnSelfTest(meta bool) (bool, []string) {
	base := os.Getenv("TMPDIR")
	if base == "" || !cdnIsDir(base) {
		base = os.TempDir()
	}
	tmp, err := os.MkdirTemp(base, "check-doc-name-")
	if err != nil {
		return false, []string{"✗ 夹具基目录创建失败"}
	}
	defer os.RemoveAll(tmp)

	lines := []string{}
	ok := true

	good := cdnPyJoin(tmp, "good")
	for _, gf := range cdnGoodFiles {
		cdnWriteFile(cdnPyJoin(good, gf[0]), gf[1])
	}
	bad := cdnPyJoin(tmp, "bad")
	for _, bf := range cdnBadFiles {
		cdnWriteFile(cdnPyJoin(bad, bf[0]), "# x\n")
	}
	for _, bd := range cdnBadDirs {
		cdnWriteFile(cdnPyJoin(bad, bd[0], "a.md"), "# x\n")
	}
	for _, bp := range cdnBadPairs {
		cdnWriteFile(cdnPyJoin(bad, bp[0]), "# x\n")
		cdnWriteFile(cdnPyJoin(bad, bp[1]), "x\n")
	}
	mix := cdnPyJoin(bad, cdnBadPrefixMix)
	for _, rel := range []string{"01-a.md", "02-b.md", "c.md", "d.md"} {
		cdnWriteFile(cdnPyJoin(mix, rel), "# x\n")
	}
	pairs := cdnPyJoin(tmp, "pairs")
	cdnWriteFile(cdnPyJoin(pairs, "docs", "01-设计", "机读对.md"), "# x\n")
	cdnWriteFile(cdnPyJoin(pairs, "docs", "01-设计", "机读对.tsv"), "x\n")
	goodpair := cdnPyJoin(tmp, "goodpair")
	for _, gf := range cdnGoodFiles {
		cdnWriteFile(cdnPyJoin(goodpair, gf[0]), gf[1])
	}
	cdnWriteFile(cdnPyJoin(goodpair, "docs", "01-设计", "设计-文档体系-v1.0.py"), "x\n")
	empty := cdnPyJoin(tmp, "empty")
	_ = os.MkdirAll(empty, 0o755)

	baseArgs := []string{"--repo-root", tmp, "--no-self-test"}

	caseFn := func(label string, expectRC int, args, mustContain, mustNotContain []string, envKV *[2]string) bool {
		if envKV != nil {
			old, had := os.LookupEnv(envKV[0])
			_ = os.Setenv(envKV[0], envKV[1])
			defer func() {
				if had {
					_ = os.Setenv(envKV[0], old)
				} else {
					_ = os.Unsetenv(envKV[0])
				}
			}()
		}
		full := append(append([]string{}, baseArgs...), args...)
		var buf bytes.Buffer
		rc := checkDocNameGateMain(&buf, io.Discard, full)
		out := buf.String()
		gc := rc == expectRC
		for _, s := range mustContain {
			if !strings.Contains(out, s) {
				gc = false
			}
		}
		for _, s := range mustNotContain {
			if strings.Contains(out, s) {
				gc = false
			}
		}
		mark := "✓"
		if !gc {
			mark = "✗"
		}
		lines = append(lines, fmt.Sprintf("%s %s（期望 rc=%d，实得 rc=%d）", mark, label, expectRC, rc))
		if !gc {
			ol := strings.Split(strings.TrimSpace(out), "\n")
			if len(ol) > 4 {
				ol = ol[len(ol)-4:]
			}
			lines = append(lines, fmt.Sprintf("    输出尾部：%v", ol))
		}
		return gc
	}

	ok = ok && caseFn("好件必绿（含允许对 edit-assert）", 0,
		[]string{"--target", good, "--repo-root", good}, []string{"OK", "A06"}, nil, nil)
	for _, bf := range cdnBadFiles {
		if bf[1] == "" {
			continue
		}
		ok = ok && caseFn("坏件必红 "+bf[0], 1, []string{"--target", cdnPyJoin(bad, bf[0])}, []string{bf[1]}, nil, nil)
	}
	ok = ok && caseFn("整棵坏树必红且含 N6（同基名）", 1, []string{"--target", bad}, []string{"N6"}, nil, nil)
	for _, bd := range cdnBadDirs {
		ok = ok && caseFn("目录名命中只告警 "+bd[0], 0,
			[]string{"--target", cdnPyJoin(bad, strings.TrimRight(bd[0], "/"))}, []string{bd[1], "告警"}, nil, nil)
	}
	for _, bp := range cdnBadPairs {
		ok = ok && caseFn("同基名未登记必红 "+bp[0], 1,
			[]string{"--target", cdnPyJoin(bad, cdnDirname(bp[0]))}, []string{bp[2]}, nil, nil)
	}
	ok = ok && caseFn("同基名 .md/.tsv 对按 B03 豁免", 0, []string{"--target", pairs}, []string{"B03", "OK"}, nil, nil)
	ok = ok && caseFn("前缀序号混用只告警", 0, []string{"--target", mix}, []string{"N5", "告警"}, nil, nil)
	ok = ok && caseFn("缺件 · 目标路径不存在", 2, []string{"--target", cdnPyJoin(tmp, "no-such")}, []string{"不给结论"}, nil, nil)
	ok = ok && caseFn("缺件 · 扫描域为空", 2, []string{"--target", empty}, []string{"不给结论"}, nil, nil)
	nodocs := cdnPyJoin(tmp, "nodocs")
	cdnWriteFile(cdnPyJoin(nodocs, "scripts", "x.py"), "x\n")
	ok = ok && caseFn("缺件 · 默认文档面 docs/ 不存在 ⇒ BLOCKED（不是假绿）", 2,
		[]string{"--repo-root", nodocs, "--scope", "repo"}, []string{"不给结论", cdnPyJoin(nodocs, "docs")}, nil, nil)
	emptydocs := cdnPyJoin(tmp, "emptydocs")
	cdnWriteFile(cdnPyJoin(emptydocs, "docs", "readme.txt"), "x\n")
	ok = ok && caseFn("缺件 · docs/ 下 0 篇 md（无可扫内容）⇒ BLOCKED（不是假绿）", 2,
		[]string{"--repo-root", emptydocs, "--scope", "repo"}, []string{"不给结论", "0 篇 md"}, nil, nil)

	// ⑦ 例外表条数自证
	var leBuf bytes.Buffer
	rcLE := checkDocNameGateMain(&leBuf, io.Discard, []string{"--list-exempt"})
	le := leBuf.String()
	cntOK := rcLE == 0 &&
		strings.Contains(le, "A 档 · 《清单-命名规范化》已登记的「不改」条目：15 条（期望 15）") &&
		strings.Contains(le, "B 档 · 规则级规范豁免：3 条（期望 3）") &&
		strings.Contains(le, "C 档 · 新增登记（C01–C03 待批准 · C04 已拍 2026-09-18 · "+
			"C05–C07 第二波任务书登记 2026-09-18 · C08 2026-09-22（门⑮ 主仓悬空引用）登记 · "+
			"C09 2026-09-24（波11 序125 `W-43` hook git 环境）登记）：9 条（期望 9）") &&
		strings.Contains(le, "D 档 · 本轮 A 路拍板登记（登记不改）：2 条（期望 2）")
	m := "✓"
	if !cntOK {
		m = "✗"
	}
	lines = append(lines, m+" 例外表条数自证（A15 / B3 / C9 / D2）")
	ok = ok && cntOK

	// ⑧ 只报告不改
	before := cdnTreeSha(tmp)
	var junk bytes.Buffer
	checkDocNameGateMain(&junk, io.Discard, append(append([]string{}, baseArgs...), "--target", bad))
	same := before == cdnTreeSha(tmp)
	m = "✓"
	if !same {
		m = "✗"
	}
	txt := "一致"
	if !same {
		txt = "被改动"
	}
	lines = append(lines, fmt.Sprintf("%s 只报告不改（夹具树逐字节不变：%s）", m, txt))
	ok = ok && same

	// ⑨ N6 判据有牙
	ok = ok && caseFn("好件树 + 未登记同基名对必红（N6 判据有牙）", 1,
		[]string{"--target", goodpair, "--repo-root", goodpair}, []string{"N6"}, nil, nil)
	// ⑩ --scope 可用
	ok = ok && caseFn("--scope repo 可用（= 仓根口径）", 1, []string{"--scope", "repo", "--target", bad}, []string{"N6"}, nil, nil)
	ok = ok && caseFn("--scope docs 解析到 <仓根>/docs", 2, []string{"--scope", "docs"}, []string{"不给结论", cdnPyJoin(tmp, "docs")}, nil, nil)
	// ⑪ 用法错
	ok = ok && caseFn("用法错（未知参数）必 rc=2 不给结论", 2, []string{"--scope", "nonsense"}, nil, []string{"OK："}, nil)
	// ⑬ D3① 换根负控
	envKV := [2]string{"ZERG_DEVDOCS_ROOT", cdnPyJoin(tmp, "no-such-Zerg-内部文档")}
	ok = ok && caseFn("D3① devdocs 根不在 ⇒ BLOCKED（仓外不可达不许当绿）", 2,
		[]string{"--repo-root", tmp, "--scope", "devdocs"}, []string{"不给结论", "开发文档面根不在"}, nil, &envKV)

	expected := 1 + 5 + 1 + len(cdnBadDirs) + len(cdnBadPairs) + 1 + 1 + 2 + 2 + 1 + 1 + 4 + 1
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

// ── 参数与入口 ────────────────────────────────────────────────────────────────

type cdnArgs struct {
	targets     []string
	target      []string
	repoRoot    string
	maxExamples int
	countFrozen bool
	scope       string
	jsonOut     bool
	listRules   bool
	listExempt  bool
	selfTest    bool
	noSelfTest  bool
	help        bool
}

func cdnParseArgs(argv []string) (*cdnArgs, string) {
	a := &cdnArgs{repoRoot: cdnDefaultRepoRoot(), maxExamples: 5}
	takeVal := func(name, val string, hasVal bool, i *int) (string, string) {
		if hasVal {
			return val, ""
		}
		*i++
		if *i >= len(argv) {
			return "", "argument " + name + ": expected one argument"
		}
		return argv[*i], ""
	}
	for i := 0; i < len(argv); i++ {
		s := argv[i]
		if s == "--" {
			a.targets = append(a.targets, argv[i+1:]...)
			break
		}
		if strings.HasPrefix(s, "--") {
			name, val := s, ""
			hasVal := false
			if k := strings.Index(s, "="); k >= 0 {
				name, val, hasVal = s[:k], s[k+1:], true
			}
			switch name {
			case "--target":
				v, e := takeVal(name, val, hasVal, &i)
				if e != "" {
					return nil, e
				}
				a.target = append(a.target, v)
			case "--repo-root":
				v, e := takeVal(name, val, hasVal, &i)
				if e != "" {
					return nil, e
				}
				a.repoRoot = v
			case "--max-examples":
				v, e := takeVal(name, val, hasVal, &i)
				if e != "" {
					return nil, e
				}
				n, err := strconv.Atoi(v)
				if err != nil {
					return nil, "argument --max-examples: invalid int value: '" + v + "'"
				}
				a.maxExamples = n
			case "--scope":
				v, e := takeVal(name, val, hasVal, &i)
				if e != "" {
					return nil, e
				}
				if v != "repo" && v != "docs" && v != "devdocs" {
					return nil, "argument --scope: invalid choice: '" + v + "'"
				}
				a.scope = v
			case "--count-frozen":
				a.countFrozen = true
			case "--json":
				a.jsonOut = true
			case "--list-rules":
				a.listRules = true
			case "--list-exempt":
				a.listExempt = true
			case "--self-test":
				a.selfTest = true
			case "--no-self-test":
				a.noSelfTest = true
			case "--help", "-h":
				a.help = true
			default:
				return nil, "unrecognized arguments: " + s
			}
			continue
		}
		if s == "-h" {
			a.help = true
			continue
		}
		a.targets = append(a.targets, s)
	}
	return a, ""
}

func cdnPrintHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: check-doc-name [-h] [--target TARGET] [--repo-root REPO_ROOT] [--max-examples N]")
	fmt.Fprintln(w, "                       [--count-frozen] [--scope {repo,docs,devdocs}] [--json]")
	fmt.Fprintln(w, "                       [--list-rules] [--list-exempt] [--self-test] [--no-self-test]")
	fmt.Fprintln(w, "                       [targets ...]")
	fmt.Fprintln(w, "命名规范化门（只报告不改）—— Go 真搬；正文见 py 原件件头。")
}

// checkDocNameGateMain —— 真搬入口（M2 刻意无调用点）。
func checkDocNameGateMain(stdout, stderr io.Writer, argv []string) int {
	a, perr := cdnParseArgs(argv)
	if a != nil && a.help {
		cdnPrintHelp(stdout)
		return 0
	}
	if perr != "" {
		cdnPrintHelp(stderr)
		fmt.Fprintln(stderr, "check-doc-name: error: "+perr)
		return 2
	}
	for _, k := range []string{"A", "B", "C", "D"} {
		var got int
		switch k {
		case "A":
			got = len(cdnExemptA)
		case "B":
			got = len(cdnExemptB)
		case "C":
			got = len(cdnExemptC)
		case "D":
			got = len(cdnExemptD)
		}
		if got != cdnExemptExpected[k] {
			fmt.Fprintf(stdout, "BLOCKED：例外表 %s 档条数不符（期望 %d，实得 %d）⇒ 不给结论\n", k, cdnExemptExpected[k], got)
			return 2
		}
	}
	if a.listRules {
		cdnListRules(stdout)
		return 0
	}
	if a.listExempt {
		return cdnListExempt(stdout)
	}
	meta := os.Getenv("CHECK_DOC_NAME_META_OFF") != "1"
	if a.selfTest {
		ok, lines := cdnSelfTest(meta)
		fmt.Fprintln(stdout, "check-doc-name 自检（成对负控三层：好件必绿 / 坏件必红 / 缺件必 rc=2"+
			" + 元自检·注不进真仓）")
		for _, ln := range lines {
			fmt.Fprintln(stdout, "  "+ln)
		}
		if ok {
			return 0
		}
		return 2
	}
	if !a.noSelfTest {
		ok, lines := cdnSelfTest(meta)
		if !ok {
			fmt.Fprintln(stdout, "自检未过 ⇒ 拒绝扫真目标（rc=2 不给结论）")
			for _, ln := range lines {
				fmt.Fprintln(stdout, "  "+ln)
			}
			return 2
		}
	}

	root := cdnAbs(a.repoRoot)
	if a.scope == "devdocs" && len(a.targets)+len(a.target) == 0 {
		root = cdnDevdocsRoot(root)
		if !cdnIsDir(root) {
			fmt.Fprintf(stdout, "BLOCKED：开发文档面根不在 ⇒ 不给结论：%s\n", root)
			fmt.Fprintln(stdout, "  口径：仓外根不可达（设计稿 §五：不许静默当绿 ✗）。换根请设 ZERG_DEVDOCS_ROOT。")
			return 2
		}
	}
	if !cdnIsDir(root) {
		fmt.Fprintf(stdout, "BLOCKED：仓根不是目录 ⇒ 不给结论：%s\n", root)
		return 2
	}
	targets := []string{}
	for _, x := range append(append([]string{}, a.targets...), a.target...) {
		targets = append(targets, cdnAbs(x))
	}
	explicit := len(targets) > 0
	if len(targets) == 0 && (a.scope == "docs" || a.scope == "devdocs") {
		if a.scope == "docs" {
			targets = []string{cdnPyJoin(root, "docs")}
		} else {
			targets = []string{root}
		}
	}
	if len(targets) == 0 {
		targets = []string{root}
	}
	if !explicit {
		docRoot := cdnPyJoin(root, "docs")
		if a.scope == "devdocs" {
			docRoot = root
		}
		if !cdnIsDir(docRoot) {
			fmt.Fprintf(stdout, "BLOCKED：文档面根不存在（缺件不许静默降级）⇒ 不给结论：%s\n", docRoot)
			fmt.Fprintln(stdout, "  口径：公开树侧（无 docs/）本 scope 为 BLOCKED、不适用；私有树正常判（判据一字未改）。")
			return 2
		}
		if cdnCountMd(docRoot) == 0 {
			fmt.Fprintf(stdout, "BLOCKED：docs/ 下扫到 0 篇 md（无可扫内容 ⇒ 空转）⇒ 不给结论：%s\n", docRoot)
			fmt.Fprintln(stdout, "  口径：公开树侧（无 docs/）本 scope 为 BLOCKED、不适用；私有树正常判（判据一字未改）。")
			return 2
		}
	}
	for _, t := range targets {
		if _, err := os.Stat(t); err != nil {
			fmt.Fprintf(stdout, "BLOCKED：目标不存在 ⇒ 不给结论：%s\n", t)
			return 2
		}
	}
	rep, rerr := cdnRun(root, targets, a.countFrozen)
	if rerr != "" {
		fmt.Fprintf(stdout, "BLOCKED：%s ⇒ 不给结论\n", rerr)
		return 2
	}
	rc, verdict := cdnDecide(rep)
	if a.jsonOut {
		cdnPrintJSON(stdout, rc, verdict, rep)
		return rc
	}
	fmt.Fprintln(stdout, "check-doc-name —— 《清单-命名规范化》七类判据的可执行版（只报告不改）")
	cdnPrintReport(stdout, rep, a.maxExamples)
	fmt.Fprintln(stdout, verdict)
	return rc
}

// cdnCountMd —— 前置缺件闸用：默认文档面根下 md 篇数（同 py 口径：排除 EXCL_DIRS + 隐藏项）。
func cdnCountMd(docRoot string) int {
	n := 0
	var rec func(d string)
	rec = func(d string) {
		f, err := os.Open(d)
		if err != nil {
			return
		}
		ents, _ := f.ReadDir(-1)
		f.Close()
		for _, e := range ents {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			p := cdnPyJoin(d, name)
			if cdnIsDir(p) && !cdnIsSymlink(p) {
				if cdnInExclDirs(name) {
					continue
				}
				rec(p)
				continue
			}
			if strings.HasSuffix(name, ".md") {
				n++
			}
		}
	}
	rec(docRoot)
	return n
}
