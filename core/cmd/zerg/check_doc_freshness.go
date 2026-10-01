// check_doc_freshness.go —— 判据件 `scripts/gates/check-doc-freshness.py`（1840 行）的 **Go 真搬**
// （v2.5.14 · M2 · **新口径靶：文档新鲜度族（四门 D1–D4 · 只读 · 只报不改）** · 入口 `checkDocFreshnessGateMain`）。
//
// 适格判定（先判）：本门判据 **不含「当前时间」与文件 mtime 的比较** ——
//
//	· `datetime.datetime.now()` 只出现 1 处（`write_report` 的 `generated_at` 字段），**不进任何判据**；
//	· D1/D2/D3 全是**静态**（路径存在 / `文件:行` 有效 / slug 锚）；
//	· D4 是「在临时目录重建 → 与仓内提交物**逐字节**比对」，也**不看 now**（易变行只是**归一化**里的模式，不是取当下时间）。
//	⇒ **适格**（只用 mtime 之间的相对比较或不看时间）。
//
// 本件与新口径（外部树走显式入参旋钮）—— 逐条对应：
//
//	① 旋钮名**照 py 已有同名 env**：`ZERG_DEVDOCS_ROOT` / `ZERG_DOCS_ALT` / `ZERG_DOCS`
//	   （外加默认同级根 `<仓父>/Zerg-内部文档`）⇒ 同名同序照搬、不另起名。
//	   py 对**仓根**本身无旗标、无 env（起于 `__file__`）⇒ 见 (a) 的默认根差异面。
//	② 缺省值 = py 默认推导**逐字同构**（`devdocs_root()` = `ZERG_DEVDOCS_ROOT` 优先，
//	   否则 `sibling_docs_base(root) + 项目文档/v2.5.10`，`sibling_docs_base` = `ZERG_DOCS_ALT or
//	   ZERG_DOCS or join(dirname(abspath(root)), "Zerg-内部文档")`）。本件复用同族已落件的
//	   `dcmdSiblingDocsBase` / `dcmdEnvOr` / `dcmdJoin` / `dcmdAbs`，与 py 串逐字对齐。
//	③ ★ 退 2 边界：py 的「缺件/空转/用法错/判据不可判」四类 **逐字照搬**退 2；本件**未**把
//	   「任一取不到」扩成红。
//	④ 取不到一律 rc=2，✗ 不读成「0 件＝干净」。
//
// 本单边界（照实）：**只落码、不接线** —— ✗ 不注册命令 · ✗ 删 py 原件 · 入口无调用点（M2 刻意）。
//
// ★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `HERE = dirname(abspath(__file__))` ⇒ 默认根起于**脚本所在目录**；
//	    本件无 `__file__` ⇒ 复用 `dcmdDefaultRepo()`（cwd 上溯 ≤8 层找 `core/cmd/zerg/main.go`，
//	    回落 abspath(cwd)）。「脚本位置 == cwd 上溯根」时两者读数逐字相同（对拍甲环境已证）；
//	    否则**路径推导面不同** ⇒ 不属逐字对齐承诺。
//	(b) **自指检查**：① 本件在自己扫描面内 ⇒ **否**（本门扫 `.md` 语料；本件是 `.go`）；
//	    ② 落在 `check-hardcoded-private-paths`（扫 `core/`）· `check-error-kinds`（扫 `core/cmd/zerg/*.go`）·
//	    `check-wall-layer`（扫 `core/cmd/zerg/*.go`）扫描面内 ⇒ **是** ⇒ 正文零私有绝对路径字面量、
//	    不写被扫错误构造调用原串、self-test 夹具魔法串只在运行时建在 TMPDIR 下。
//	    ③ 见交付读数（他门实跑 rc 未翻红）。④ 不接线 ⇒ 不进任何命令树/用法面。
//
// py 原件真口子（✗ 不自行修 · 交回入账）：
//
//	① `--self-test` 用 `tempfile.mkdtemp` 且**从不回收**（每跑一次自检，$TMPDIR 多两份
//	   `docfresh-selftest-*` / `docfresh-selftest-ext-*` 残留）；本件 Go 侧 `defer os.RemoveAll` 回收
//	   （输出不可见差异：py 只打 `  ✓/✗ <label>` 与末行，均不含夹具路径 ⇒ 两态 stdout 逐字同）。
//	② `gate_d4` 的 `tempfile.mkdtemp(prefix="docfresh-d4-")` 有 `finally: rmtree` ⇒ **已回收**（无口子）。
//	③ `classify_d2` 的 `re.search(r"\b(?:N|L|line|Line)\d*$", pfx)` 用 `\b`（Unicode 词界）—— 见未覆盖面 ④。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse **长选项前缀缩写**（`--selft` ⇒ `--self-test`）不实现。
//	② `-h/--help` 正文：本件**照 argparse 形自拟**，与 py 的 argparse 版本相关（见对拍 `-h` 格）。
//	③ `--report` 落盘 JSON 含 `generated_at`（now()）· `elapsed_ms*`（计时）⇒ **本就非逐字可比**；
//	   本件实现但**不属逐字对齐承诺**。
//	④ ★ **正则语义差**：py `\s`/`\S`/`\d`/`\w`/`\b` 是 **Unicode 语义**，Go RE2 默认 **ASCII**。
//	   本件按新口径：`\d`→`\p{Nd}`；`\s`/`\S` 展开为**显式 Unicode 空白类**（`cdfUniSpaceCls`）；
//	   `\b` 仍按 RE2 ASCII 词界（**如实标为未覆盖**，真仓读数已证零命中该形态）。
//	⑤ `unicodedata.normalize("NFC", s)`：Go 标准库无 NFC（仓内无 `golang.org/x/text`）⇒
//	   本件 `nfc = 恒等`（**如实标为未覆盖**：真仓 slug 用例全为预组合码位 ⇒ 读数一致）。
//	⑥ `urllib.parse.unquote`：本件自实现 percent 解码（近似：无效 `%XX` 原样保留）。
//	⑦ 读文件失败正文 / OSError 串：正文自定，不属对齐。
//
// `--evidence GAP-20261001-13`（在册 M2 承接 · 语义对口：want_family=gate / want_action=check）。
package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const cdfProg = "check-doc-freshness.py"
const cdfVersion = "v1.0.0"
const cdfSlugVersionDir = "v2.5.10"
const cdfConfigRel = "scripts/docs/doc-freshness.config.json"

const cdfRCOK = 0
const cdfRCFail = 1
const cdfRCBlocked = 2

var cdfRCName = map[int]string{0: "OK（全绿）", 1: "FAIL（有失败项）", 2: "BLOCKED（不给结论）"}

const cdfDomLocal = "本地"
const cdfDomExternal = "外部引用域"
const cdfDomFrozen = "冻结归档"
const cdfDomMirror = "镜像副本"
const cdfDomFixture = "夹具"

// cdfUniSpaceCls —— Python str 的 `\s` 等价类（展开为显式 Unicode 空白，RE2 里不写 `\s`）。
const cdfUniSpaceCls = "\t\n\v\f\r \u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000"

// ─────────────────────────── 通用小工具 ───────────────────────────

// cdfNormJoin —— posixpath.join + 吃掉 ./ 与 .. 的纯文本归约（不碰文件系统）。
func cdfNormJoin(parts ...string) string {
	valid := []string{}
	for _, x := range parts {
		if x != "" {
			valid = append(valid, x)
		}
	}
	var p string
	if len(valid) == 0 {
		p = ""
	} else {
		p = dcmdJoin(valid[0], valid[1:]...)
	}
	segs := []string{}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." {
			continue
		}
		if s == ".." {
			if len(segs) > 0 {
				segs = segs[:len(segs)-1]
			}
			continue
		}
		segs = append(segs, s)
	}
	return strings.Join(segs, "/")
}

// cdfToRel —— os.path.relpath + 反斜杠归一。
func cdfToRel(repo, abspath string) string {
	r := dcmdRelpath(abspath, repo)
	return strings.ReplaceAll(r, string(os.PathSeparator), "/")
}

// cdfIsUnder —— 段级前缀判定。
func cdfIsUnder(rel, prefix string) bool {
	prefix = strings.TrimRight(prefix, "/")
	return rel == prefix || strings.HasPrefix(rel, prefix+"/")
}

// cdfUnquote —— urllib.parse.unquote 近似（percent 解码，无效序列原样）。
func cdfUnquote(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := cdfHexVal(s[i+1])
			lo, ok2 := cdfHexVal(s[i+2])
			if ok1 && ok2 {
				out = append(out, byte(hi<<4|lo))
				i += 2
				continue
			}
		}
		out = append(out, s[i])
	}
	return string(out)
}

func cdfHexVal(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

// cdfNFC —— 近似 NFC（Go 标准库无 NFC；见件头未覆盖面 ⑤）。
func cdfNFC(s string) string { return s }

// cdfDir —— os.path.dirname 语义（复用同族件 dcmdDir）。

// ─────────────────────────── 规则表（--list-rules 用）───────────────────────────

type cdfGate struct {
	name, inputs, judge, rc0, rc1, rc2, fix string
}

var cdfGates = map[string]cdfGate{
	"d1": {
		name:   "D1 · 引用路径存在",
		inputs: "① 语料清单（目录白名单，显式）② 解析基准集（仓根 + 各模块根）③ 引用域开关 ④ 大小写/Unicode 归一化开关",
		judge: "抽 [..](target) 与 ![..](target)；scheme:// / mailto: 分流外链；#x 归 D3；" +
			"相对路径在 {引用文件目录, 仓根, 各模块根} 任一命中即过；目标是目录也必须存在；先 unquote 再判；" +
			"判据用「树上真实路径字符串集合」比对，不用 os.path.exists（APFS 大小写不敏感会造假绿）",
		rc0: "全部相对引用可解析",
		rc1: "有断链（逐条 文件:行 → 目标）",
		rc2: "语料清单为空（空转 ⇒ 不给结论，不绿）",
		fix: "改引用或改文件名；**不改判据来消红**",
	},
	"d2": {
		name:   "D2 · 引用 `文件:行` 有效",
		inputs: "① 归一化白名单（不算引用的噪声类别，必须计数）② 解析基准集 ③ 行号语义（`-` 区间 / `,` 多段 / 单行）④ 裸文件名策略",
		judge: "R1（含 / 的路径）：存在且最大行号 ≤ 文件总行数；R2（裸文件名）：全解析域同名恰好 1 个 ⇒ 过、0 个 ⇒ 红、>1 ⇒ 黄（歧义）；" +
			"噪声类别（IP:端口 / 省略号 / 占位符 / 命令行样式 / 绝对路径 / URL）不判但必须计数；" +
			"★ 2026-09-20 批 C · T-40（§二十一 已红第 18 条）追加「**计数对拍**」：一行里同时出现现跑命令 " +
			"`grep -c '<BRE>' <path>` 与计数声明 `<N> 条` ⇒ **argv 直调**跑那条 grep -c（不经 shell · 不给文档执行权），" +
			"实测值必须 == 声明值，不等 ⇒ 红（判词点名「改代码或改文档后必须现跑」）；" +
			"判不了的（文件不在盘上 / grep 跑不动 / 输出不是整数）只单列计数 + 打印「判不了」，**不当结论**；" +
			"空转 = 引用条目与计数条目**都**为 0",
		rc0: "无红（黄=歧义只入清单）",
		rc1: "有红（文件不存在 / 行号越界 / 计数与现跑不符）",
		rc2: "解析基准集配置缺失（判据不可判）· **仓外基准集（同级开发文档仓）不在盘 ⇒ 外根形态的引用判不了**（缺语料铁律：rc=2 + 逐字打印缺件路径，绝不判红、更不许静默绿）",
		fix: "行号越界 ⇒ 改行号或改「文件:行」；文件没了 ⇒ 改指向新落点；计数不符 ⇒ **现跑那条命令**，改代码或改文档 ",
	},
	"d3": {
		name:   "D3 · 断链断锚",
		inputs: "① slug 规范（冻结 · §5.6① 八条）② 带锚链接的语料范围 ③ 是否纳入 Obsidian wiki 链接（默认不纳入、单列计数）④ 同文件重复 slug 的编号规则",
		judge: "链接带 #anchor 且目标可解析为 .md ⇒ 按冻结规则算目标文件全部标题 slug，命中即过；" +
			"显式锚（`{: #slug }` / `{#slug}`）优先；同文件重复 slug 追加 -1/-2；空 slug 兜底 s<序号> 且禁止被引用",
		rc0: "无断锚",
		rc1: "有断锚（逐条 文件:行 → path#slug）",
		rc2: "slug 规范未落盘 / profile 未知（规范不存在 ⇒ 判据不可判）",
		fix: "改锚或改标题（改标题前先查谁引用它）；纯符号标题改文案",
	},
	"d4": {
		name:   "D4 · 生成式参考 drift",
		inputs: "① 生成器清单（源 → 产物 → 重建命令 → 期望退出码）② 干净的临时目录 ③ git 只读权限（本实现用「临时目录重建 + 与提交物比对」，不需要 git 写）",
		judge: "在临时目录重建（绝不就地生成）→ 与仓内提交物比对：不一致 ⇒ 红；" +
			"归一化（去尾空格 + 丢易变行：时间戳/版本号/本机路径）后一致 ⇒ 黄（只有易变行）；报错必须带「跑哪条命令修」",
		rc0: "产物 == 重建结果（含归一化后一致）",
		rc1: "有 diff",
		rc2: "缺生成器 / 产物不存在 / 0 个可比对（不给结论）",
		fix: "照每条报错里给的命令重跑生成器（绝不手改生成物）",
	},
}

// ─────────────────────────── 正则（RE2 口径已按新标准展开）───────────────────────────

var (
	cdfReInlineLink     = regexp.MustCompile(`(!?)\[([^\]\n]*)\]\(\s*(<[^>\n]*>|[^)` + cdfUniSpaceCls + `]+)(?:\s+"[^"]*")?\s*\)`)
	cdfReFence          = regexp.MustCompile(`^\s{0,3}(` + "```" + `|~~~)`)
	cdfReWikilink       = regexp.MustCompile(`\[\[([^\]\n]+)\]\]`)
	cdfReHeading        = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*$`)
	cdfReExplicitAnchor = regexp.MustCompile(`^(.*?)\s*(?:\{:\s*#([^}` + cdfUniSpaceCls + `]+)\s*\}|\{#([A-Za-z0-9_\-\.]+)\})\s*$`)
	cdfReScheme         = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.\-]*://`)
	cdfReURL            = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.\-]*://[^` + cdfUniSpaceCls + "`" + `)\]}>，。；;]+`)
	cdfReCand           = regexp.MustCompile(`([A-Za-z0-9_./@+~\-` + "\u4e00-\u9fff\u3000" + `]+):(\p{Nd}+(?:\s*[-–,]\s*\p{Nd}+)*)`)
	cdfReCandBacktick   = regexp.MustCompile("`([^`\n]+)`")
	cdfReIPPort         = regexp.MustCompile(`^\p{Nd}{1,3}(?:\.\p{Nd}{1,3}){3}$`)
	cdfReExt            = regexp.MustCompile(`\.[A-Za-z][A-Za-z0-9]{0,7}$`)
	cdfReDigits         = regexp.MustCompile(`\p{Nd}+`)
	cdfRePathLineNoise  = regexp.MustCompile(`\b(?:N|L|line|Line)\p{Nd}*$`)
	cdfReCountCmd       = regexp.MustCompile("`\\s*(grep\\s+-c\\s+'([^']*)'\\s+([^" + cdfUniSpaceCls + "'`]+))\\s*`")
	cdfReCountDecl      = regexp.MustCompile(`(\p{Nd}+)\s*条`)
	cdfReHeadTailHash   = regexp.MustCompile(`\s+#+\s*$`)
	cdfReLinkText       = regexp.MustCompile(`!?\[([^\]\[]*)\]\([^)]*\)`)
)

var cdfVolatile = []*regexp.Regexp{
	regexp.MustCompile(`\p{Nd}{4}-\p{Nd}{2}-\p{Nd}{2}`),
	regexp.MustCompile(`\p{Nd}{2}:\p{Nd}{2}:\p{Nd}{2}`),
	regexp.MustCompile(`\bv?\p{Nd}+\.\p{Nd}+\.\p{Nd}+\b`),
	regexp.MustCompile(`/(?:tmp|var/folders|Users)/[^` + cdfUniSpaceCls + `]+`),
}

// ─────────────────────────── 配置访问器 ───────────────────────────

func cdfGet(cfg map[string]any, k string) any { return cfg[k] }
func cdfStr(cfg map[string]any, k, def string) string {
	if v, ok := cfg[k].(string); ok {
		return v
	}
	return def
}
func cdfStrList(cfg map[string]any, k string) []string {
	out := []string{}
	if v, ok := cfg[k].([]any); ok {
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}
func cdfSub(cfg map[string]any, k string) map[string]any {
	if v, ok := cfg[k].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}
func cdfSubList(cfg map[string]any, k string) []map[string]any {
	out := []map[string]any{}
	if v, ok := cfg[k].([]any); ok {
		for _, x := range v {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}
func cdfAnyStr(m map[string]any, k, def string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return def
}
func cdfAnyBool(m map[string]any, k string, def bool) bool {
	if v, ok := m[k].(bool); ok {
		return v
	}
	return def
}

// ─────────────────────────── 语料 / 解析域 ───────────────────────────

type cdfSoft struct {
	pfx    string
	dom    string
	scopes map[string]bool
}

type cdfTree struct {
	repo, mainRepo string
	cfg            map[string]any
	scope          string
	strictAll      bool
	noSoft         bool
	verbose        bool

	files    []string
	fileset  map[string]bool
	dirs     map[string]bool
	lowerMap map[string]string
	byname   map[string][]string

	bynameLocal map[string][]string
	corpus      []string

	hard             []string
	hardNames        map[string]bool
	soft             []cdfSoft
	externalPrefixes []string

	linesCache map[string]int
	textCache  map[string]string
	tBuild     float64
}

func cdfNewTree(repo string, cfg map[string]any, scope string, strictAll, verbose bool, mainRepo string) *cdfTree {
	t := &cdfTree{
		repo:        dcmdAbs(repo),
		cfg:         cfg,
		scope:       scope,
		strictAll:   strictAll,
		verbose:     verbose,
		fileset:     map[string]bool{},
		dirs:        map[string]bool{},
		lowerMap:    map[string]string{},
		byname:      map[string][]string{},
		bynameLocal: map[string][]string{},
		hardNames:   map[string]bool{},
		linesCache:  map[string]int{},
		textCache:   map[string]string{},
	}
	if mainRepo != "" {
		t.mainRepo = dcmdAbs(mainRepo)
	} else {
		t.mainRepo = t.repo
	}
	t0 := time.Now()
	t.buildDomains()
	t.walk()
	t.tBuild = time.Since(t0).Seconds()
	return t
}

func cdfScopeSet(names ...string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

func (t *cdfTree) buildDomains() {
	c := t.cfg
	for _, p := range cdfStrList(c, "hard_exclude") {
		t.hard = append(t.hard, strings.TrimRight(p, "/")+"/")
	}
	for _, p := range cdfStrList(c, "hard_exclude") {
		if !strings.Contains(p, "/") {
			t.hardNames[p] = true
		}
	}
	addSoft := func(key, dom string, scopes map[string]bool) {
		_ = key
		_ = dom
		_ = scopes
	}
	_ = addSoft
	for _, p := range cdfStrList(c, "mirror_paths") {
		t.soft = append(t.soft, cdfSoft{strings.TrimRight(p, "/") + "/", cdfDomMirror, cdfScopeSet("mirror")})
	}
	for _, p := range cdfStrList(c, "frozen_paths") {
		t.soft = append(t.soft, cdfSoft{strings.TrimRight(p, "/") + "/", cdfDomFrozen, cdfScopeSet("wide", "mirror")})
	}
	for _, p := range cdfStrList(c, "third_party_paths") {
		t.soft = append(t.soft, cdfSoft{strings.TrimRight(p, "/") + "/", cdfDomExternal, cdfScopeSet("wide", "mirror")})
	}
	for _, p := range cdfStrList(c, "fixture_paths") {
		t.soft = append(t.soft, cdfSoft{strings.TrimRight(p, "/") + "/", cdfDomFixture, cdfScopeSet("main", "wide", "mirror")})
	}
	for _, p := range cdfStrList(c, "engine_data_paths") {
		t.soft = append(t.soft, cdfSoft{strings.TrimRight(p, "/") + "/", cdfDomFrozen, cdfScopeSet()})
	}
	for _, p := range cdfStrList(c, "external_domain_paths") {
		t.externalPrefixes = append(t.externalPrefixes, strings.TrimRight(p, "/")+"/")
	}
}

func (t *cdfTree) hardExcluded(rel string) bool {
	for _, p := range t.hard {
		if cdfIsUnder(rel, strings.TrimRight(p, "/")) {
			return true
		}
	}
	for _, seg := range strings.Split(rel, "/") {
		if t.hardNames[seg] {
			return true
		}
	}
	return false
}

func (t *cdfTree) softState(rel string) (bool, string) {
	for _, s := range t.soft {
		if cdfIsUnder(rel, strings.TrimRight(s.pfx, "/")) {
			if s.dom == cdfDomExternal && t.isExternal(rel) && t.scope == "main" {
				return true, cdfDomExternal
			}
			if s.scopes[t.scope] {
				return true, s.dom
			}
			return false, s.dom
		}
	}
	return true, cdfDomLocal
}

func (t *cdfTree) isExternal(rel string) bool {
	for _, p := range t.externalPrefixes {
		if cdfIsUnder(rel, strings.TrimRight(p, "/")) {
			return true
		}
	}
	return false
}

func (t *cdfTree) domain(rel string) string {
	for _, s := range t.soft {
		if cdfIsUnder(rel, strings.TrimRight(s.pfx, "/")) {
			return s.dom
		}
	}
	if t.isExternal(rel) {
		return cdfDomExternal
	}
	return cdfDomLocal
}

// cdfWalkDir —— os.walk(topdown=True) 语义：目录序用 ReadDir(-1)（**原序**，不排序）。
type cdfDirEntry struct {
	name  string
	isDir bool
	isLn  bool
}

func cdfReadDirRaw(path string) ([]cdfDirEntry, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	ents, err := f.ReadDir(-1)
	if err != nil && len(ents) == 0 {
		return nil, false
	}
	out := []cdfDirEntry{}
	for _, e := range ents {
		de := cdfDirEntry{name: e.Name()}
		if fi, err := os.Stat(filepath.Join(path, e.Name())); err == nil {
			de.isDir = fi.IsDir()
		}
		if e.Type()&os.ModeSymlink != 0 {
			de.isLn = true
		}
		out = append(out, de)
	}
	return out, true
}

func (t *cdfTree) walk() {
	baseRoots := []string{}
	corpusRoots := []string{}
	if t.scope == "devdocs" {
		baseRoots = []string{""}
		corpusRoots = []string{""}
	} else {
		for _, r := range cdfStrList(t.cfg, "base_roots") {
			baseRoots = append(baseRoots, strings.TrimRight(r, "/"))
		}
		for _, r := range cdfStrList(t.cfg, "corpus") {
			corpusRoots = append(corpusRoots, strings.TrimRight(r, "/"))
		}
	}
	wantRes := map[string]bool{}
	for _, b := range baseRoots {
		wantRes[b] = true
	}
	t.walkDir(t.repo, wantRes, corpusRoots)
	sort.Strings(t.corpus)
	sort.Strings(t.files)
}

func (t *cdfTree) walkDir(dp string, wantRes map[string]bool, corpusRoots []string) {
	rel := cdfToRel(t.repo, dp)
	relp := rel
	if relp == "." {
		relp = ""
	}
	if relp != "" && t.hardExcluded(relp) {
		return
	}
	ents, ok := cdfReadDirRaw(dp)
	if !ok {
		return
	}
	dns := []string{}
	fns := []string{}
	for _, e := range ents {
		if e.isDir {
			dns = append(dns, e.name)
		} else {
			fns = append(fns, e.name)
		}
	}
	// dns[:] = [d for d in dns if not d.startswith(".") and not hard_excluded(join(relp,d))]
	ndns := []string{}
	for _, d := range dns {
		if strings.HasPrefix(d, ".") {
			continue
		}
		if t.hardExcluded(cdfNormJoin(relp, d)) {
			continue
		}
		ndns = append(ndns, d)
	}
	dns = ndns

	inRes := false
	for b := range wantRes {
		if relp == b || b == "" || cdfRevUnder(relp, b) {
			inRes = true
			break
		}
	}
	if inRes {
		if relp != "" {
			t.dirs[relp] = true
		}
		for _, f := range fns {
			if strings.HasPrefix(f, ".") {
				continue
			}
			fr := cdfNormJoin(relp, f)
			t.files = append(t.files, fr)
			t.fileset[fr] = true
			nm := cdfBase(fr)
			t.byname[nm] = append(t.byname[nm], fr)
			if d := t.domain(fr); d != cdfDomMirror && d != cdfDomFrozen {
				t.bynameLocal[nm] = append(t.bynameLocal[nm], fr)
			}
			if _, exists := t.lowerMap[strings.ToLower(fr)]; !exists {
				t.lowerMap[strings.ToLower(fr)] = fr
			}
		}
	}
	// 语料面
	corpusHere := false
	for _, c := range corpusRoots {
		if relp == c || cdfRevUnder(relp, c) {
			corpusHere = true
			break
		}
	}
	if corpusHere && (relp != "" || t.scope == "devdocs") {
		ok2, _ := t.softState(relp)
		if !ok2 {
			return
		}
		for _, f := range fns {
			if strings.HasSuffix(f, ".md") && !strings.HasPrefix(f, ".") {
				t.corpus = append(t.corpus, cdfNormJoin(relp, f))
			}
		}
	}
	for _, d := range dns {
		child := filepath.Join(dp, d)
		// py: followlinks=False ⇒ 符号链接目录记录但不下降
		if fi, err := os.Lstat(child); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			continue
		}
		t.walkDir(child, wantRes, corpusRoots)
	}
}

func cdfRevUnder(rel, root string) bool {
	if root == "" {
		return true
	}
	return rel == root || strings.HasPrefix(rel, root+"/")
}

func cdfBase(p string) string {
	_, tail := dcmdSplitPath(p)
	return tail
}

// cdfPyOsJoin —— `os.path.join(repo, rel)` 语义（**rel 为绝对则整个替换**；Go `filepath.Join` 不是这个语义）。
func cdfPyOsJoin(repo, rel string) string {
	if strings.HasPrefix(rel, "/") {
		return rel
	}
	if repo == "" || strings.HasSuffix(repo, "/") {
		return repo + rel
	}
	return repo + "/" + rel
}

func (t *cdfTree) text(rel string) string {
	if s, ok := t.textCache[rel]; ok {
		return s
	}
	b, err := os.ReadFile(cdfPyOsJoin(t.repo, rel))
	if err != nil {
		t.textCache[rel] = ""
		return ""
	}
	s := string(b)
	t.textCache[rel] = s
	return s
}

func (t *cdfTree) nlines(rel string) int {
	if n, ok := t.linesCache[rel]; ok {
		return n
	}
	txt := t.text(rel)
	n := 0
	if txt == "" {
		n = 0
	} else {
		c := strings.Count(txt, "\n")
		if strings.HasSuffix(txt, "\n") {
			n = c
		} else {
			n = c + 1
		}
	}
	t.linesCache[rel] = n
	return n
}

func (t *cdfTree) exists(rel string) bool { return t.fileset[rel] }
func (t *cdfTree) existsDir(rel string) bool {
	return t.dirs[rel] || rel == ""
}
func (t *cdfTree) caseVariant(rel string) (string, bool) {
	v, ok := t.lowerMap[strings.ToLower(rel)]
	return v, ok
}

// ─────────────────────────── 豁免表 ───────────────────────────

// cdfFnmatch —— fnmatch.fnmatch 近似（`*` 跨 `/`，`?` 单字符，`[seq]` 类）。
func cdfFnmatch(name, pat string) bool {
	re := cdfFnmatchRe(pat)
	ok, err := regexp.MatchString("(?s)^"+re+"$", name)
	return err == nil && ok
}

func cdfFnmatchRe(pat string) string {
	var b strings.Builder
	rs := []rune(pat)
	i := 0
	for i < len(rs) {
		c := rs[i]
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := i + 1
			if j < len(rs) && (rs[j] == '!' || rs[j] == '^') {
				j++
			}
			if j < len(rs) && rs[j] == ']' {
				j++
			}
			for j < len(rs) && rs[j] != ']' {
				j++
			}
			if j >= len(rs) {
				b.WriteString(regexp.QuoteMeta("["))
			} else {
				inner := string(rs[i+1 : j])
				if strings.HasPrefix(inner, "!") {
					inner = "^" + inner[1:]
				}
				b.WriteString("[" + inner + "]")
				i = j
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
		i++
	}
	return b.String()
}

func cdfAllowlisted(t *cdfTree, gate, rel, target string) map[string]any {
	for _, e := range cdfSubList(t.cfg, "allowlist") {
		g := cdfAnyStr(e, "gate", "*")
		if g != "*" && g != gate {
			continue
		}
		if !cdfFnmatch(rel, cdfAnyStr(e, "file_glob", "*")) {
			continue
		}
		tt := cdfAnyStr(e, "target", "")
		if tt == target || cdfFnmatch(target, tt) {
			return e
		}
	}
	return nil
}

func cdfAllowlistErrors(cfg map[string]any) []string {
	errs := []string{}
	for i, e := range cdfSubList(cfg, "allowlist") {
		if strings.TrimSpace(cdfAnyStr(e, "reason", "")) == "" {
			errs = append(errs, fmt.Sprintf("allowlist[%d] 缺 reason（只给路径的 allowlist 就是无记录的洗白）", i))
		}
		if strings.TrimSpace(cdfAnyStr(e, "date", "")) == "" {
			errs = append(errs, fmt.Sprintf("allowlist[%d] 缺 date", i))
		}
	}
	return errs
}

// ─────────────────────────── 链接抽取（D1/D3 共用）───────────────────────────

type cdfLink struct {
	line    int
	isImg   bool
	tgt     string
	inFence bool
}

func cdfIterLinks(text string) []cdfLink {
	out := []cdfLink{}
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		if cdfReFence.MatchString(line) {
			inFence = !inFence
			continue
		}
		for _, m := range cdfReInlineLink.FindAllStringSubmatch(line, -1) {
			tgt := m[3]
			if strings.HasPrefix(tgt, "<") && strings.HasSuffix(tgt, ">") {
				tgt = tgt[1 : len(tgt)-1]
			}
			out = append(out, cdfLink{i + 1, m[1] == "!", tgt, inFence})
		}
	}
	return out
}

func cdfClassifyTarget(tgt string) (string, string, string) {
	tt := strings.TrimSpace(tgt)
	if cdfReScheme.MatchString(tt) || strings.HasPrefix(tt, "mailto:") || strings.HasPrefix(tt, "//") {
		return "外链", "", ""
	}
	if strings.HasPrefix(tt, "#") {
		return "页内锚", "", tt[1:]
	}
	var f, a string
	if strings.Contains(tt, "#") {
		parts := strings.SplitN(tt, "#", 2)
		f = cdfUnquote(parts[0])
		a = parts[1]
		if f == "" {
			return "页内锚", "", a
		}
	} else {
		f, a = cdfUnquote(tt), ""
	}
	if strings.HasPrefix(f, "/") || strings.HasPrefix(f, "~") ||
		regexp.MustCompile(`^[A-Za-z]:[\\/]`).MatchString(f) {
		return "绝对路径样式", f, a
	}
	return "相对路径", f, a
}

// ── 仓外基准根 ──────────────────────────────────────────────────────────────

func cdfDevdocsRoot(repoRoot string) string {
	if env := os.Getenv("ZERG_DEVDOCS_ROOT"); env != "" {
		return dcmdAbs(env)
	}
	sibling := dcmdSiblingDocsBase(repoRoot)
	return dcmdAbs(dcmdJoin(sibling, "项目文档", cdfSlugVersionDir))
}

func cdfSiblingDocsRoot(t *cdfTree) string {
	return dcmdDir(dcmdDir(cdfDevdocsRoot(t.repo)))
}

var cdfSiblingStripPrefixes = []string{"docs/"}

func cdfExternalCandidates(t *cdfTree, tgt string) []string {
	sib := cdfSiblingDocsRoot(t)
	parent := dcmdDir(dcmdAbs(t.repo))
	out := []string{dcmdJoin(sib, tgt)}
	for _, pre := range cdfSiblingStripPrefixes {
		if strings.HasPrefix(tgt, pre) {
			out = append(out, dcmdJoin(sib, tgt[len(pre):]))
		}
	}
	first := strings.Split(tgt, "/")[0]
	if first != "" && first != cdfBase(dcmdAbs(t.repo)) {
		if fi, err := os.Stat(dcmdJoin(parent, first)); err == nil && fi.IsDir() {
			out = append(out, dcmdJoin(parent, tgt))
		}
	}
	res := []string{}
	for _, c := range out {
		res = append(res, strings.ReplaceAll(c, string(os.PathSeparator), "/"))
	}
	return res
}

var cdfExtFormLabels = []string{
	"① 字面点名同级仓（`../Zerg-内部文档/…`）",
	"② 首段在仓内无落点（相对同级仓根，如 `项目文档/…`）",
	"③ `docs/` 前缀（分家前相对主仓根）",
}

func cdfSiblingForms(t *cdfTree, tgt string) (string, string, bool) {
	sib := cdfSiblingDocsRoot(t)
	segs := strings.Split(strings.ReplaceAll(tgt, "\\", "/"), "/")
	for len(segs) > 0 && (segs[0] == "" || segs[0] == "." || segs[0] == "..") {
		segs = segs[1:]
	}
	if len(segs) == 0 {
		return "", "", false
	}
	first := segs[0]
	if first == cdfBase(sib) {
		return cdfExtFormLabels[0], sib, true
	}
	if strings.HasPrefix(tgt, "docs/") {
		return cdfExtFormLabels[2], dcmdJoin(sib, tgt[len("docs/"):]), true
	}
	for _, b := range cdfStrList(t.cfg, "base_roots") {
		if first == strings.TrimRight(b, "/") {
			return "", "", false
		}
	}
	if _, err := os.Stat(dcmdJoin(t.repo, first)); err == nil {
		return "", "", false
	}
	if _, err := os.Stat(dcmdJoin(t.repo, "docs", first)); err == nil {
		return "", "", false
	}
	return cdfExtFormLabels[1], dcmdJoin(sib, tgt), true
}

func cdfSiblingRootOnDisk(t *cdfTree) bool {
	fi, err := os.Stat(cdfSiblingDocsRoot(t))
	return err == nil && fi.IsDir()
}

func cdfResolveFile(t *cdfTree, srcRel, target string) (string, string, []string) {
	tgt := target
	if strings.HasPrefix(target, "/") {
		tgt = strings.TrimLeft(target, "/")
	}
	cands := []string{}
	d := cdfDirPosix(srcRel)
	cands = append(cands, cdfNormJoin(d, tgt))
	cands = append(cands, cdfNormJoin(tgt))
	for _, b := range cdfStrList(t.cfg, "base_roots") {
		b = strings.TrimRight(b, "/")
		if b != "" {
			cands = append(cands, cdfNormJoin(b, tgt))
		}
	}
	cands = append(cands, cdfExternalCandidates(t, tgt)...)
	seen := map[string]bool{}
	cand2 := []string{}
	for _, c := range cands {
		if c != "" && !seen[c] {
			seen[c] = true
			cand2 = append(cand2, c)
		}
	}
	for _, c := range cand2 {
		if strings.HasPrefix(c, "/") {
			if st, err := os.Stat(c); err == nil {
				if st.IsDir() {
					return "dir", c, cand2
				}
				return "exists", c, cand2
			}
			continue
		}
		if t.exists(c) {
			return "exists", c, cand2
		}
		if t.existsDir(c) {
			return "dir", c, cand2
		}
	}
	for _, c := range cand2 {
		if strings.HasPrefix(c, "/") {
			continue
		}
		if v, ok := t.caseVariant(c); ok {
			return "case", v, cand2
		}
	}
	return "miss", "", cand2
}

func cdfDirPosix(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	return p[:i]
}

// ─────────────────────────── slug ───────────────────────────

func cdfHeadingText(raw string) string {
	s := strings.TrimSpace(raw)
	s = cdfReHeadTailHash.ReplaceAllString(s, "")
	prev := ""
	for prev != s {
		prev = s
		s = cdfReLinkText.ReplaceAllString(s, "$1")
	}
	for _, pair := range [][2]string{{"**", ""}, {"__", ""}, {"*", ""}, {"`", ""}} {
		s = strings.ReplaceAll(s, pair[0], pair[1])
	}
	return strings.TrimSpace(s)
}

func cdfSlugify(text, profile string) string {
	s := strings.ToLower(cdfNFC(text))
	var b strings.Builder
	for _, ch := range s {
		if unicode.IsSpace(ch) {
			if profile == "docv1.2" && ch == '\u3000' {
				continue
			}
			b.WriteRune('-')
		} else if ch == '_' || ch == '-' {
			b.WriteRune(ch)
		} else if unicode.IsLetter(ch) || unicode.IsNumber(ch) || unicode.IsMark(ch) {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

type cdfAnchor struct {
	slug  string
	line  int
	title string
}

func cdfFileAnchors(t *cdfTree, rel, profile string) ([]cdfAnchor, int, int) {
	text := t.text(rel)
	slugs := []cdfAnchor{}
	empty := 0
	heads := 0
	used := map[string]int{}
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		if cdfReFence.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		m := cdfReHeading.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		heads++
		body := m[2]
		explicit := ""
		if em := cdfReExplicitAnchor.FindStringSubmatch(body); em != nil {
			body = em[1]
			explicit = em[2]
			if explicit == "" {
				explicit = em[3]
			}
		}
		var s string
		if explicit != "" {
			s = explicit
		} else {
			s = cdfSlugify(cdfHeadingText(body), profile)
		}
		if s == "" {
			empty++
			s = fmt.Sprintf("s%d", empty)
		}
		if n, ok := used[s]; ok {
			used[s] = n + 1
			s = fmt.Sprintf("%s-%d", s, n+1)
		} else {
			used[s] = 0
		}
		slugs = append(slugs, cdfAnchor{s, i + 1, cdfHeadingText(body)})
	}
	return slugs, empty, heads
}

// ─────────────────────────── 结果容器 ───────────────────────────

type cdfFinding struct {
	file, target, kind, detail, domain string
	line                               int
}

type cdfResult struct {
	gate     string
	tree     *cdfTree
	counts   map[string]int
	order    []string
	findings []cdfFinding
	notes    []string
	errors   []string
}

func cdfNewResult(gate string, t *cdfTree) *cdfResult {
	return &cdfResult{gate: gate, tree: t, counts: map[string]int{}}
}

func (r *cdfResult) add(file string, line int, target, kind, detail, domain string) {
	r.findings = append(r.findings, cdfFinding{file: file, line: line, target: target, kind: kind, detail: detail, domain: domain})
}

func (r *cdfResult) counted(key string, n int) {
	r.counts[key] += n
}

func (r *cdfResult) reds() []cdfFinding {
	out := []cdfFinding{}
	for _, f := range r.findings {
		if !strings.HasPrefix(f.kind, "红") {
			continue
		}
		if !r.tree.strictAll && f.domain != cdfDomLocal {
			continue
		}
		out = append(out, f)
	}
	return out
}

func (r *cdfResult) yellow() []cdfFinding {
	out := []cdfFinding{}
	for _, f := range r.findings {
		if !strings.HasPrefix(f.kind, "黄") {
			continue
		}
		if !r.tree.strictAll && f.domain != cdfDomLocal {
			continue
		}
		out = append(out, f)
	}
	return out
}

func (r *cdfResult) rc() int {
	if len(r.errors) > 0 {
		return cdfRCBlocked
	}
	if len(r.reds()) > 0 {
		return cdfRCFail
	}
	return cdfRCOK
}

// ─────────────────────────── D1 ───────────────────────────

func cdfCorpusMissingRoots(t *cdfTree) []string {
	out := []string{}
	hard := cdfStrList(t.cfg, "hard_exclude")
	for _, c := range cdfStrList(t.cfg, "corpus") {
		d := filepath.Join(t.repo, filepath.FromSlash(c))
		fi, err := os.Stat(d)
		if err != nil || !fi.IsDir() {
			out = append(out, fmt.Sprintf("%s（目录不存在）", c))
			continue
		}
		n := 0
		var rec func(p string)
		rec = func(p string) {
			ents, ok := cdfReadDirRaw(p)
			if !ok {
				return
			}
			for _, e := range ents {
				if e.isDir {
					skip := false
					for _, h := range hard {
						if e.name == h {
							skip = true
						}
					}
					if !skip {
						rec(filepath.Join(p, e.name))
					}
					continue
				}
				if strings.HasSuffix(e.name, ".md") && !strings.HasPrefix(e.name, ".") {
					n++
				}
			}
		}
		rec(d)
		if n == 0 {
			out = append(out, fmt.Sprintf("%s（0 篇 md）", c))
		}
	}
	return out
}

func cdfCorpusEmptyMsg(t *cdfTree) string {
	miss := cdfCorpusMissingRoots(t)
	joined := "—"
	if len(miss) > 0 {
		joined = strings.Join(miss, " · ")
	}
	return fmt.Sprintf("语料清单为空（扫到 0 篇 md）⇒ 空转，不给结论；缺件：%s"+
		"（公开树侧无 docs/ ⇒ 本 scope 为 BLOCKED、不适用；私有树正常判）", joined)
}

func cdfGateD1(t *cdfTree, args *cdfArgs) *cdfResult {
	r := cdfNewResult("d1", t)
	if len(t.corpus) == 0 {
		r.errors = append(r.errors, cdfCorpusEmptyMsg(t))
		return r
	}
	r.counted("语料篇数", len(t.corpus))
	for _, rel := range t.corpus {
		dom := t.domain(rel)
		text := t.text(rel)
		for _, lk := range cdfIterLinks(text) {
			cls, f, _ := cdfClassifyTarget(lk.tgt)
			if cls == "外链" {
				r.counted("外链（不判）", 1)
				continue
			}
			if cls == "页内锚" {
				r.counted("页内锚（归 D3）", 1)
				continue
			}
			if cls == "绝对路径样式" {
				r.counted("绝对路径样式（不判，单列）", 1)
				r.add(rel, lk.line, lk.tgt, "单列·绝对路径", "以 / 或 ~ 开头 —— 站点根语义歧义，不判", dom)
				continue
			}
			if lk.inFence && !t.noSoft {
				r.counted("代码块内链接（不判）", 1)
				continue
			}
			if f == "" {
				r.counted("空目标（不判）", 1)
				continue
			}
			if (strings.Contains(f, "…") || strings.Contains(f, "...")) && !t.noSoft {
				r.counted("省略号目标（不判）", 1)
				r.add(rel, lk.line, lk.tgt, "单列·省略号目标", "路径被省略号截断", dom)
				continue
			}
			r.counted("相对引用条目", 1)
			placeholder := false
			for _, ch := range []string{"{", "}", "*", "$", "<", ">"} {
				if strings.Contains(f, ch) {
					placeholder = true
				}
			}
			if placeholder {
				r.counted("占位符目标（不判）", 1)
				continue
			}
			if !strings.Contains(f, "/") && !strings.Contains(f, ".") && len([]rune(f)) <= 24 && !t.noSoft {
				r.counted("语法示例目标（不判）", 1)
				r.add(rel, lk.line, lk.tgt, "单列·语法示例目标", "目标是无扩展名短串（文档在讲链接语法本身）", dom)
				continue
			}
			if ae := cdfAllowlisted(t, "d1", rel, lk.tgt); ae != nil {
				r.counted("豁免（allowlist）", 1)
				r.add(rel, lk.line, lk.tgt, "豁免（allowlist）",
					fmt.Sprintf("%s（登记于 %s）", cdfAnyStr(ae, "reason", ""), cdfAnyStr(ae, "date", "")), dom)
				continue
			}
			kind, hit, cands := cdfResolveFile(t, rel, f)
			if kind == "exists" || kind == "dir" {
				r.counted("过", 1)
				continue
			}
			if kind == "case" {
				r.counted("大小写不符（黄）", 1)
				r.add(rel, lk.line, lk.tgt, "黄·大小写不符",
					fmt.Sprintf("精确串没命中，命中=%s（APFS 判绿、Linux CI 判红）", hit), dom)
				continue
			}
			r.counted("断链", 1)
			last := ""
			if len(cands) > 0 {
				last = cands[len(cands)-1]
			}
			r.add(rel, lk.line, lk.tgt, "红·断链", fmt.Sprintf("基准集里都没有；最后的候选=%s", last), dom)
		}
	}
	for _, f := range r.findings {
		if f.domain != cdfDomLocal {
			r.counted("非本地域命中（单列·不进退码）·"+f.domain, 1)
		}
	}
	if r.counts["相对引用条目"] == 0 {
		r.errors = append(r.errors, "该 scope 的语料里 0 条相对引用（空转 ⇒ 不给结论，沿用「缺件不许静默降级」纪律）")
	}
	return r
}

// ─────────────────────────── D2 ───────────────────────────

func cdfSplitLines(linestr string) []int {
	out := []int{}
	for _, m := range cdfReDigits.FindAllString(linestr, -1) {
		n, _ := strconv.Atoi(m)
		out = append(out, n)
	}
	return out
}

type cdfPathCand struct {
	line        int
	raw, pfx    string
	linenums    string
	pos         string
	trailing    bool
	placeholder bool
}

func cdfIterPathlineCandidates(text string) ([]cdfPathCand, map[string]int) {
	out := []cdfPathCand{}
	noise := map[string]int{}
	inFence := false
	for i, line := range strings.Split(text, "\n") {
		if cdfReFence.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		masked := line
		for _, m := range cdfReURL.FindAllStringIndex(line, -1) {
			noise["噪声·URL"]++
			masked = masked[:m[0]] + strings.Repeat("U", m[1]-m[0]) + masked[m[1]:]
		}
		type span struct {
			s, e int
			body string
		}
		spans := []span{}
		for _, m := range cdfReCandBacktick.FindAllStringSubmatchIndex(masked, -1) {
			spans = append(spans, span{m[2], m[3], masked[m[2]:m[3]]})
		}
		for _, m := range cdfReCand.FindAllStringSubmatchIndex(masked, -1) {
			a, b := m[2], m[1]
			inCode, cmdline := false, false
			for _, sp := range spans {
				if a >= sp.s && b <= sp.e {
					inCode = true
					if strings.Contains(sp.body, " ") || strings.Contains(sp.body, "=") {
						cmdline = true
					}
					break
				}
			}
			if !cmdline && strings.HasSuffix(strings.TrimRight(masked[:a], " \t\n\r\f\v\u0085\u00a0"), "=") {
				cmdline = true
			}
			kind := "正文内"
			if cmdline {
				kind = "命令行样式"
			} else if inCode {
				kind = "反引号内"
			}
			before := ""
			if a > 0 {
				before = masked[a-1 : a]
			}
			after := ""
			if b < len(masked) {
				after = masked[b : b+1]
			}
			placeholder := strings.ContainsAny(before, "<>{}*$%") || strings.ContainsAny(after, ">}*")
			tail := b < len(masked) && masked[b:b+1] == "/"
			out = append(out, cdfPathCand{i + 1, masked[m[0]:m[1]], masked[m[2]:m[3]], masked[m[4]:m[5]], kind, tail, placeholder})
		}
	}
	return out, noise
}

func cdfClassifyD2(pfx, linenums string, trailing, cmdline, placeholder bool) (string, bool) {
	if cmdline {
		return "噪声·命令行样式", false
	}
	if trailing {
		return "噪声·尾部斜杠", false
	}
	if placeholder {
		return "噪声·占位符", false
	}
	if cdfReScheme.MatchString(pfx) {
		return "噪声·URL", false
	}
	if strings.HasPrefix(pfx, "/") || strings.HasPrefix(pfx, "~") ||
		regexp.MustCompile(`^[A-Za-z]:$`).MatchString(pfx) {
		return "噪声·绝对路径", false
	}
	if cdfReIPPort.MatchString(pfx) {
		return "噪声·IP:端口", false
	}
	if strings.Contains(pfx, "...") || strings.Contains(pfx, "…") {
		return "噪声·省略号路径", false
	}
	if strings.ContainsAny(pfx, "<>{}*$%") || cdfRePathLineNoise.MatchString(pfx) {
		return "噪声·占位符", false
	}
	if !cdfReExt.MatchString(pfx) {
		return "噪声·无扩展名", false
	}
	ns := cdfSplitLines(linenums)
	if len(ns) == 0 || ns[0] <= 0 {
		return "噪声·非法行号", false
	}
	return "引用", true
}

func cdfCountClaims(text string) [][5]string {
	out := [][5]string{}
	for i, line := range strings.Split(text, "\n") {
		m := cdfReCountCmd.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		d := cdfReCountDecl.FindStringSubmatch(line)
		if d == nil {
			continue
		}
		out = append(out, [5]string{strconv.Itoa(i + 1), m[1], m[2], m[3], d[1]})
	}
	return out
}

func cdfRunGrepC(t *cdfTree, rel, bre string) (int, string) {
	full := filepath.Join(t.repo, filepath.FromSlash(rel))
	if !dcmdIsFile(full) {
		return 0, fmt.Sprintf("目标文件不在盘上：%s", rel)
	}
	cmd := exec.Command("grep", "-c", bre, rel)
	cmd.Dir = t.repo
	var so, se strings.Builder
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			return 0, fmt.Sprintf("grep 跑不动：%s", err)
		}
	}
	if code != 0 && code != 1 {
		st := strings.TrimSpace(se.String())
		if len(st) > 80 {
			st = st[:80]
		}
		return 0, fmt.Sprintf("grep 退出码 %d（%s）", code, st)
	}
	s := strings.TrimSpace(so.String())
	if s == "" {
		s = "0"
	}
	n, err2 := strconv.Atoi(s)
	if err2 != nil {
		return 0, fmt.Sprintf("grep 输出不是整数：%q", so.String())
	}
	return n, ""
}

func cdfSuffixUnique(t *cdfTree, ref string) string {
	ref = strings.Trim(ref, "/")
	hits := []string{}
	for _, p := range t.files {
		if strings.HasSuffix(p, "/"+ref) {
			hits = append(hits, p)
		}
	}
	if len(hits) == 1 {
		return hits[0]
	}
	if len(hits) > 1 {
		return "__AMBIG__"
	}
	return ""
}

func cdfGateD2(t *cdfTree, args *cdfArgs) *cdfResult {
	r := cdfNewResult("d2", t)
	if len(cdfStrList(t.cfg, "base_roots")) == 0 {
		r.errors = append(r.errors, "解析基准集配置缺失（config.base_roots 空）⇒ 判据不可判")
		return r
	}
	if len(t.corpus) == 0 {
		r.errors = append(r.errors, cdfCorpusEmptyMsg(t))
		return r
	}
	r.counted("语料篇数", len(t.corpus))
	suffixOn := true
	if v, ok := cdfSub(t.cfg, "d2")["resolve_package_relative"].(bool); ok {
		suffixOn = v
	}
	extRoot := cdfSiblingDocsRoot(t)
	extOK := cdfSiblingRootOnDisk(t)
	type blockedExt struct {
		label, rel string
		line       int
		raw, pfx   string
		landing    string
	}
	blocked := []blockedExt{}
	for _, rel := range t.corpus {
		dom := t.domain(rel)
		text := t.text(rel)
		cands, extraNoise := cdfIterPathlineCandidates(text)
		keys := make([]string, 0, len(extraNoise))
		for k := range extraNoise {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			r.counted(k, extraNoise[k])
		}
		for _, c := range cands {
			cat, judged := cdfClassifyD2(c.pfx, c.linenums, c.trailing, c.pos == "命令行样式", c.placeholder)
			r.counted("位置·"+c.pos, 1)
			if !judged {
				r.counted(cat, 1)
				continue
			}
			nums := cdfSplitLines(c.linenums)
			mx := 0
			for _, n := range nums {
				if n > mx {
					mx = n
				}
			}
			hasSlash := strings.Contains(c.pfx, "/")
			if hasSlash {
				r.counted("引用条目·R1", 1)
			} else {
				r.counted("引用条目·R2", 1)
			}
			rn := "R2"
			if hasSlash {
				rn = "R1"
			}
			r.counted("引用条目·"+rn+"·"+c.pos, 1)
			if ae := cdfAllowlisted(t, "d2", rel, c.raw); ae != nil {
				r.counted("豁免（allowlist）", 1)
				r.add(rel, c.line, c.raw, "豁免（allowlist）",
					fmt.Sprintf("%s（登记于 %s）", cdfAnyStr(ae, "reason", ""), cdfAnyStr(ae, "date", "")), dom)
				continue
			}
			if hasSlash {
				kind, hit, cands2 := cdfResolveFile(t, rel, c.pfx)
				if kind == "exists" && hit != "" && strings.HasPrefix(hit, "/") {
					r.counted("过·仓外命中（同级仓）", 1)
				}
				if kind == "case" {
					r.counted("黄·大小写不符", 1)
					r.add(rel, c.line, c.raw, "黄·大小写不符", fmt.Sprintf("精确串没命中，命中=%s", hit), dom)
					continue
				}
				if kind == "dir" {
					r.counted("红·目标是目录", 1)
					r.add(rel, c.line, c.raw, "红·目标是目录", fmt.Sprintf("R1 要求是文件；命中目录 %s", hit), dom)
					continue
				}
				if kind == "miss" && suffixOn {
					hit2 := cdfSuffixUnique(t, c.pfx)
					if hit2 == "__AMBIG__" {
						r.counted("黄·歧义（包内相对路径多份）", 1)
						r.add(rel, c.line, c.raw, "黄·歧义", fmt.Sprintf("包内相对路径唯一后缀匹配命中多份（%s）", c.pfx), dom)
						continue
					}
					if hit2 != "" {
						r.counted("过·后缀唯一命中", 1)
						tot := t.nlines(hit2)
						if tot != 0 && mx > tot {
							r.counted("红·行号越界", 1)
							r.add(rel, c.line, c.raw, "红·行号越界", fmt.Sprintf("%s 只有 %d 行，引用到 %d（后缀命中 %s）", hit2, tot, mx, hit2), dom)
						}
						continue
					}
				}
				if kind == "miss" && !extOK {
					if label, landing, ok := cdfSiblingForms(t, c.pfx); ok {
						r.counted("不给结论·仓外根不在盘", 1)
						r.counted("不给结论·仓外根不在盘·"+label, 1)
						blocked = append(blocked, blockedExt{label, rel, c.line, c.raw, c.pfx, landing})
						continue
					}
				}
				if kind == "miss" {
					r.counted("红·文件不存在", 1)
					last := ""
					if len(cands2) > 0 {
						last = cands2[len(cands2)-1]
					}
					r.add(rel, c.line, c.raw, "红·文件不存在", fmt.Sprintf("R1：基准集里没有 %s（最后候选 %s）", c.pfx, last), dom)
					continue
				}
				tot := t.nlines(hit)
				if tot != 0 && mx > tot {
					r.counted("红·行号越界", 1)
					r.add(rel, c.line, c.raw, "红·行号越界", fmt.Sprintf("%s 只有 %d 行，引用到 %d", hit, tot, mx), dom)
				} else {
					r.counted("过", 1)
				}
			} else {
				cand := t.bynameLocal[c.pfx]
				if len(cand) == 0 {
					_, hit, _ := cdfResolveFile(t, rel, c.pfx)
					if hit != "" {
						r.counted("过", 1)
					} else {
						r.counted("红·无此文件", 1)
						r.add(rel, c.line, c.raw, "红·无此文件", fmt.Sprintf("R2：同名池（去镜像/去冻结归档）里没有 %s", c.pfx), dom)
					}
				} else if len(cand) == 1 {
					r.counted("过", 1)
					tot := t.nlines(cand[0])
					if tot != 0 && mx > tot {
						r.counted("注·R2 行号越界（不入退码）", 1)
					}
				} else {
					r.counted("黄·歧义", 1)
					m := len(cand)
					if m > 9 {
						m = 9
					}
					r.counted(fmt.Sprintf("黄·歧义·同名%d份", m), 1)
					head := cand
					if len(head) > 3 {
						head = head[:3]
					}
					r.add(rel, c.line, c.raw, "黄·歧义",
						fmt.Sprintf("R2：同名 %d 份（%s…），指哪份不可知", len(cand), strings.Join(head, ", ")), dom)
				}
			}
		}
		for _, cl := range cdfCountClaims(text) {
			clLine, _ := strconv.Atoi(cl[0])
			r.counted("计数条目", 1)
			got, why := cdfRunGrepC(t, cl[3], cl[2])
			if why != "" {
				r.counted("计数条目·判不了", 1)
				r.notes = append(r.notes, fmt.Sprintf("%s:%d 计数条目判不了（%s）· 原样：%s", rel, clLine, why, cl[1]))
				continue
			}
			decl, _ := strconv.Atoi(cl[4])
			if got == decl {
				r.counted("计数条目·对得上", 1)
				continue
			}
			r.counted("红·计数与现跑不符", 1)
			r.add(rel, clLine, cl[1], "红·计数与现跑不符",
				fmt.Sprintf("文档声明 `%d 条`，argv 直调现跑 `%s` ⇒ **%d**（改代码或改文档后**必须现跑**）", decl, cl[1], got), dom)
		}
	}
	for _, f := range r.findings {
		if f.domain != cdfDomLocal {
			r.counted("非本地域命中（单列·不进退码）·"+f.domain, 1)
		}
	}
	if len(blocked) > 0 {
		r.errors = append(r.errors, fmt.Sprintf(
			"仓外基准根不在盘 ⇒ rc=2 不给结论：**缺件 %s**（同级开发文档仓 · 字面形态 `../Zerg-内部文档`）；"+
				"%d 条**外根形态**的引用判不了 —— 缺语料铁律：语料/前置不在盘时判不了就报 BLOCKED，"+
				"既不判红（假红）也不静默绿", extRoot, len(blocked)))
		for _, label := range cdfExtFormLabels {
			items := []blockedExt{}
			for _, x := range blocked {
				if x.label == label {
					items = append(items, x)
				}
			}
			if len(items) == 0 {
				continue
			}
			eg := []string{}
			for i, x := range items {
				if i >= 3 {
					break
				}
				eg = append(eg, fmt.Sprintf("例 %s:%d → `%s`", x.rel, x.line, x.pfx))
			}
			r.errors = append(r.errors, fmt.Sprintf("%s %d 条 · 外根下落点 %s · %s", label, len(items), items[0].landing, strings.Join(eg, " · ")))
		}
	}
	if r.counts["引用条目·R1"]+r.counts["引用条目·R2"] == 0 && r.counts["计数条目"] == 0 {
		r.errors = append(r.errors, "该 scope 的语料里 0 条 `文件:行` 引用 且 0 条计数条目（空转 ⇒ 不给结论）")
	}
	return r
}

// ─────────────────────────── D3 ───────────────────────────

func cdfGateD3(t *cdfTree, args *cdfArgs) *cdfResult {
	r := cdfNewResult("d3", t)
	profile := cdfAnyStr(cdfSub(t.cfg, "d3"), "slug_profile", "github")
	specDoc := cdfAnyStr(cdfSub(t.cfg, "d3"), "spec_doc", "")
	if profile == "" {
		r.errors = append(r.errors, "slug 规范未落盘（config.d3.slug_profile 空）⇒ 判据不可判")
		return r
	}
	if specDoc != "" {
		if _, err := os.Stat(filepath.Join(t.mainRepo, filepath.FromSlash(specDoc))); err != nil {
			r.errors = append(r.errors, fmt.Sprintf("slug 规范文件不存在（%s）⇒ 判据不可判", specDoc))
			return r
		}
	}
	if len(t.corpus) == 0 {
		r.errors = append(r.errors, "语料清单为空（扫到 0 篇 md）⇒ 空转，不给结论")
		return r
	}
	includeWiki := cdfAnyBool(cdfSub(t.cfg, "d3"), "include_wiki_links", false)
	r.counted("语料篇数", len(t.corpus))
	cache := map[string][3]any{}
	for _, rel := range t.corpus {
		dom := t.domain(rel)
		text := t.text(rel)
		if includeWiki {
			for range cdfReWikilink.FindAllString(text, -1) {
				r.counted("wiki 链接（单列）", 1)
			}
		} else {
			n := len(cdfReWikilink.FindAllString(text, -1))
			if n > 0 {
				r.counted("wiki 链接（单列·不判）", n)
			}
		}
		for _, lk := range cdfIterLinks(text) {
			cls, f, anchor := cdfClassifyTarget(lk.tgt)
			if cls == "外链" || cls == "绝对路径样式" {
				r.counted("外链/绝对路径（不判）", 1)
				continue
			}
			if lk.inFence {
				r.counted("代码块内链接（不判）", 1)
				continue
			}
			targetRel := ""
			if cls == "页内锚" {
				if anchor == "" && strings.TrimSpace(lk.tgt) == "#" {
					r.counted("空锚目标（不判）", 1)
					continue
				}
				targetRel = rel
			}
			if cls == "相对路径" {
				if strings.HasSuffix(f, "/") {
					r.counted("目录目标（不判锚）", 1)
					continue
				}
				kind, hit, _ := cdfResolveFile(t, rel, f)
				if kind == "miss" || kind == "case" || kind == "dir" {
					r.counted("目标不存在（归 D1，不重复报锚）", 1)
					continue
				}
				if !strings.HasSuffix(hit, ".md") {
					r.counted("非 md 目标（不判锚）", 1)
					continue
				}
				targetRel = hit
			}
			if anchor == "" {
				continue
			}
			r.counted("带锚链接条目", 1)
			if ae := cdfAllowlisted(t, "d3", rel, lk.tgt); ae != nil {
				r.counted("豁免（allowlist）", 1)
				r.add(rel, lk.line, lk.tgt, "豁免（allowlist）",
					fmt.Sprintf("%s（登记于 %s）", cdfAnyStr(ae, "reason", ""), cdfAnyStr(ae, "date", "")), dom)
				continue
			}
			entry, ok := cache[targetRel]
			if !ok {
				slugs, empty, heads := cdfFileAnchors(t, targetRel, profile)
				entry = [3]any{slugs, empty, heads}
				cache[targetRel] = entry
			}
			slugs := entry[0].([]cdfAnchor)
			want := cdfNFC(cdfUnquote(anchor))
			have := map[string]bool{}
			for _, s := range slugs {
				have[s.slug] = true
			}
			if have[want] {
				r.counted("过", 1)
				continue
			}
			close := []string{}
			for s := range have {
				if strings.ToLower(s) == strings.ToLower(want) {
					close = append(close, s)
				}
			}
			sort.Strings(close)
			if len(close) > 0 {
				r.counted("黄·大小写差（锚）", 1)
				r.add(rel, lk.line, lk.tgt, "黄·大小写差（锚）",
					fmt.Sprintf("目标是 %s#%s，大小写不符（命中 %s）", targetRel, want, close[0]), dom)
				continue
			}
			r.counted("断锚", 1)
			all := []string{}
			for s := range have {
				all = append(all, s)
			}
			sort.Strings(all)
			if len(all) > 3 {
				all = all[:3]
			}
			r.add(rel, lk.line, lk.tgt, "红·断锚",
				fmt.Sprintf("目标 %s 的 %d 个标题 slug 里没有 %s（近邻=%s）", targetRel, len(slugs), want, strings.Join(all, "、")), dom)
		}
	}
	for _, f := range r.findings {
		if f.domain != cdfDomLocal {
			r.counted("非本地域命中（单列·不进退码）·"+f.domain, 1)
		}
	}
	if r.counts["带锚链接条目"] == 0 {
		r.errors = append(r.errors, "该 scope 的语料里 0 条带锚链接（空转 ⇒ 不给结论；换 --scope wide 可见仓内那批手工锚链）")
	}
	return r
}

// ─────────────────────────── D4 ───────────────────────────

func cdfNormalizeLines(text string) []string {
	out := []string{}
	pats := []string{}
	for _, p := range cdfVolatile {
		pats = append(pats, "(?:"+p.String()+")")
	}
	any := regexp.MustCompile(strings.Join(pats, "|"))
	for _, ln := range strings.Split(text, "\n") {
		s := strings.TrimRight(ln, " \t\n\r\f\v\u0085\u00a0")
		hit := false
		for _, p := range cdfVolatile {
			if p.MatchString(s) {
				hit = true
				break
			}
		}
		if hit {
			s = any.ReplaceAllString(s, "<VAR>")
		}
		out = append(out, s)
	}
	return out
}

func cdfSliceEq(a, b []string) bool {
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

func cdfFill(s, repo string, kw map[string]string) string {
	if _, ok := kw["REPO"]; !ok {
		kw["REPO"] = repo
	}
	keys := make([]string, 0, len(kw))
	for k := range kw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s = strings.ReplaceAll(s, "{"+k+"}", kw[k])
	}
	return s
}

func cdfSha1Text(s string) string {
	// 仅用于 D4 临时产物文件名（py: sha1_text(t)[:8]）；用 fnv 近似不可，需真 sha1。
	return cdfSha1Hex(s)
}

func cdfAltGlobTargets(repo, altRoot string, pats []string) []string {
	base := dcmdAbs(dcmdJoin(repo, altRoot))
	fi, err := os.Stat(base)
	if err != nil || !fi.IsDir() {
		return []string{}
	}
	tops := map[string]bool{}
	for _, pat := range pats {
		seg := strings.Split(pat, "/")[0]
		if seg != "" && !strings.ContainsAny(seg, "*?[") {
			tops[seg] = true
		}
	}
	tkeys := []string{}
	for k := range tops {
		tkeys = append(tkeys, k)
	}
	sort.Strings(tkeys)
	roots := []string{}
	if len(tkeys) > 0 {
		for _, x := range tkeys {
			roots = append(roots, dcmdJoin(base, x))
		}
	} else {
		roots = []string{base}
	}
	outSet := map[string]bool{}
	for _, rt := range roots {
		var rec func(p string)
		rec = func(p string) {
			ents, ok := cdfReadDirRaw(p)
			if !ok {
				return
			}
			names := []string{}
			for _, e := range ents {
				if e.isDir {
					if !strings.HasPrefix(e.name, ".") {
						rec(filepath.Join(p, e.name))
					}
				} else {
					names = append(names, e.name)
				}
			}
			sort.Strings(names)
			relp := cdfToRel(base, p)
			if relp == "." {
				relp = ""
			}
			prefix := ""
			if relp != "" {
				prefix = relp + "/"
			}
			for _, fn := range names {
				cand := prefix + fn
				for _, pat := range pats {
					if cdfFnmatch(cand, pat) {
						outSet[strings.Join([]string{altRoot, cand}, "/")] = true
						break
					}
				}
			}
		}
		rec(rt)
	}
	out := []string{}
	for k := range outSet {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func cdfFirstDiff(a, b string) int {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	n := len(la)
	if len(lb) > n {
		n = len(lb)
	}
	for i := 0; i < n; i++ {
		x, y := "<EOF>", "<EOF>"
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			return i + 1
		}
	}
	return 1
}

func cdfClip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func cdfLineAt(text string, n int) string {
	ls := strings.Split(text, "\n")
	if n >= 1 && n <= len(ls) {
		return ls[n-1]
	}
	return ""
}

func cdfGateD4(t *cdfTree, args *cdfArgs) *cdfResult {
	r := cdfNewResult("d4", t)
	cfg := cdfSub(t.cfg, "d4")
	gens := cdfSubList(cfg, "generators")
	if len(gens) == 0 {
		r.errors = append(r.errors, "生成器清单为空（config.d4.generators）⇒ 缺件，不给结论")
		return r
	}
	tmp, err := os.MkdirTemp("", "docfresh-d4-")
	if err != nil {
		r.errors = append(r.errors, fmt.Sprintf("重建目录建不起来（%s）⇒ 不给结论", err))
		return r
	}
	defer os.RemoveAll(tmp)
	r.counted("生成器条目", len(gens))
	judged := 0
	for _, g := range gens {
		gid := cdfAnyStr(g, "id", "?")
		mode := cdfAnyStr(g, "mode", "stdout")
		targets := []string{}
		if globs := cdfStrList(g, "targets_glob"); len(globs) > 0 {
			for _, pat := range globs {
				for _, p := range t.files {
					if cdfFnmatch(p, pat) {
						targets = append(targets, p)
					}
				}
			}
			if len(targets) == 0 {
				if altGlobs := cdfStrList(g, "targets_glob_alt"); len(altGlobs) > 0 {
					targets = append(targets, cdfAltGlobTargets(t.repo, cdfAnyStr(g, "alt_root", "../Zerg-内部文档"), altGlobs)...)
				}
			}
		} else if out := cdfAnyStr(g, "output", ""); out != "" {
			targets = append(targets, out)
		}
		script := cdfAnyStr(g, "script", "")
		optional := cdfAnyBool(g, "optional", false)
		if script != "" {
			if _, err := os.Stat(filepath.Join(t.mainRepo, filepath.FromSlash(script))); err != nil {
				if optional {
					r.counted(fmt.Sprintf("[%s] 可选生成器缺脚本（单列）", gid), 1)
					continue
				}
				r.errors = append(r.errors, fmt.Sprintf("[%s] 缺生成器脚本 %s ⇒ 不给结论", gid, script))
				continue
			}
		}
		if len(targets) == 0 {
			if optional {
				r.counted(fmt.Sprintf("[%s] 可选生成器未声明产物（单列）", gid), 1)
				continue
			}
			r.errors = append(r.errors, fmt.Sprintf("[%s] 未声明产物，也没有 targets_glob ⇒ 缺件，不给结论", gid))
			continue
		}
		for _, tg := range targets {
			if _, err := os.Stat(filepath.Join(t.repo, filepath.FromSlash(tg))); err != nil {
				r.counted(fmt.Sprintf("[%s] 产物不存在（单列）", gid), 1)
				continue
			}
			dom := t.domain(tg)
			if dom != cdfDomLocal && !t.strictAll {
				r.counted(fmt.Sprintf("[%s] %s（单列·不进退码）", gid, dom), 1)
				continue
			}
			judged++
			tmpout := filepath.Join(tmp, fmt.Sprintf("%s-%s", gid, cdfSha1Text(tg)[:8]))
			d := cdfDirPosix(tg)
			cmds := cdfStrList(g, "cmd")
			argv := []string{}
			for _, x := range cmds {
				argv = append(argv, cdfFill(x, t.repo, map[string]string{
					"OUT": tmpout, "DIR": filepath.Join(t.repo, filepath.FromSlash(d)), "TARGET": tg,
				}))
			}
			cwd := cdfFill(cdfAnyStr(g, "cwd", "{REPO}"), t.repo, map[string]string{})
			timeout := 120
			if v, ok := g["timeout_s"].(float64); ok {
				timeout = int(v)
			}
			cmd := exec.Command(argv[0], argv[1:]...)
			cmd.Dir = cwd
			var so, se []byte
			so, se, err = cdfRunTimeout(cmd, timeout)
			if err != nil && len(argv) == 0 {
				r.errors = append(r.errors, fmt.Sprintf("[%s] 重建失败（%s）⇒ 不给结论", gid, err))
				continue
			}
			var rebuilt string
			if mode == "stdout" {
				rebuilt = string(so)
			} else {
				if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() != 0 {
					st := strings.ReplaceAll(string(se), "\n", " ")
					if len(st) > 200 {
						st = st[:200]
					}
					r.errors = append(r.errors, fmt.Sprintf("[%s] 生成器 rc=%d，stderr 前 200 字：%s ⇒ 不给结论",
						gid, cmd.ProcessState.ExitCode(), st))
					continue
				}
				b, err := os.ReadFile(tmpout)
				if err != nil {
					r.errors = append(r.errors, fmt.Sprintf("[%s] 重建产物读不到（%s）⇒ 不给结论", gid, err))
					continue
				}
				rebuilt = string(b)
			}
			committed := t.text(tg)
			if committed == rebuilt {
				r.counted("过·逐字节一致", 1)
				continue
			}
			if cdfSliceEq(cdfNormalizeLines(committed), cdfNormalizeLines(rebuilt)) {
				r.counted("黄·仅易变行不同", 1)
				r.add(tg, 0, gid, "黄·仅易变行不同", "归一化（去尾空格 + 时间戳/版本/本机路径 → <VAR>）后一致", dom)
				continue
			}
			first := cdfFirstDiff(committed, rebuilt)
			if ae := cdfAllowlisted(t, "d4", tg, gid); ae != nil {
				r.counted("豁免（allowlist）", 1)
				r.add(tg, 0, gid, "豁免（allowlist）",
					fmt.Sprintf("%s（登记于 %s）", cdfAnyStr(ae, "reason", ""), cdfAnyStr(ae, "date", "")), dom)
				continue
			}
			r.counted("drift", 1)
			r.add(tg, first, gid, "红·drift",
				fmt.Sprintf("提交物 %d 行 vs 重建 %d 行，首个不同行 %d： %s  ←→  %s | 修法：%s",
					strings.Count(committed, "\n"), strings.Count(rebuilt, "\n"), first,
					cdfClip(cdfLineAt(committed, first), 70), cdfClip(cdfLineAt(rebuilt, first), 70), cdfAnyStr(g, "fix", "重跑生成器")), dom)
		}
	}
	if judged == 0 && len(r.errors) == 0 {
		r.errors = append(r.errors, "0 个可比对产物（缺件/全在非本地域）⇒ 不给结论")
	}
	r.counted("可比对产物数", judged)
	return r
}

func cdfRunTimeout(cmd *exec.Cmd, secs int) ([]byte, []byte, error) {
	var so, se strings.Builder
	cmd.Stdout = &so
	cmd.Stderr = &se
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-time.After(time.Duration(secs) * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
		return []byte(so.String()), []byte(se.String()), fmt.Errorf("超时 %ds", secs)
	case <-done:
		return []byte(so.String()), []byte(se.String()), nil
	}
}

// ─────────────────────────── 报告 / 打印 ───────────────────────────

type cdfArgs struct {
	gates       []string
	repo        string
	config      string
	scope       string
	strictAll   bool
	slugProfile string
	noWhitelist bool
	report      string
	printLimit  int
	listRules   bool
	selfTest    bool
	verbose     bool
}

func cdfPrintResult(w io.Writer, r *cdfResult, args *cdfArgs, elapsed float64) int {
	rc := r.rc()
	fmt.Fprintf(w, "%s  → rc=%d %s   (语料 %d 篇 / 解析域 %d 文件 / %.2fs)\n",
		cdfGates[r.gate].name, rc, cdfRCName[rc], len(r.tree.corpus), len(r.tree.files), elapsed)
	if len(r.errors) > 0 {
		for _, e := range r.errors {
			fmt.Fprintf(w, "  ⛔ 不给结论：%s\n", e)
		}
	}
	if len(r.notes) > 0 {
		for _, n := range r.notes {
			fmt.Fprintf(w, "  · 判不了（单列计数，不当结论）：%s\n", n)
		}
	}
	keys := make([]string, 0, len(r.counts))
	for k := range r.counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if r.counts[keys[i]] != r.counts[keys[j]] {
			return r.counts[keys[i]] > r.counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > 0 {
		parts := []string{}
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%d", k, r.counts[k]))
		}
		fmt.Fprintf(w, "  计数：%s\n", strings.Join(parts, " · "))
	}
	doms := map[string]int{}
	for _, f := range r.findings {
		doms[f.domain]++
	}
	if len(doms) > 0 {
		dk := make([]string, 0, len(doms))
		for k := range doms {
			dk = append(dk, k)
		}
		sort.Strings(dk)
		parts := []string{}
		for _, k := range dk {
			parts = append(parts, fmt.Sprintf("%s=%d", k, doms[k]))
		}
		fmt.Fprintf(w, "  命中域分布：%s（非本地域默认不进退码 ⇒ 见 --strict-all）\n", strings.Join(parts, " · "))
	}
	reds := r.reds()
	yel := r.yellow()
	lim := args.printLimit
	for i, f := range reds {
		if i >= lim {
			break
		}
		fmt.Fprintf(w, "  ✗ %s:%d → %s  [%s] %s\n", f.file, f.line, f.target, f.kind, f.detail)
	}
	if len(reds) > lim {
		fmt.Fprintf(w, "  ✗ …还有 %d 条红（--print-limit 调大 / --report 落盘全部）\n", len(reds)-lim)
	}
	ylim := lim / 2
	if ylim < 3 {
		ylim = 3
	}
	for i, f := range yel {
		if i >= ylim {
			break
		}
		fmt.Fprintf(w, "  ⚠ %s:%d → %s  [%s] %s\n", f.file, f.line, f.target, f.kind, f.detail)
	}
	if len(yel) > ylim {
		fmt.Fprintf(w, "  ⚠ …还有 %d 条黄（歧义/大小写，只入清单）\n", len(yel)-ylim)
	}
	return rc
}

// ══════════════════════════ 第二部分：sha1 / 名单 / 自检 / 入口 ══════════════════════════

func cdfSha1Hex(s string) string {
	h := sha1.Sum([]byte(s))
	return fmt.Sprintf("%x", h)
}

func cdfPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// cdfPyRepr —— Python repr() 近似（单引号、转义）。
func cdfPyRepr(s string) string {
	useDouble := strings.Contains(s, "'") && !strings.Contains(s, "\"")
	q := byte('\'')
	if useDouble {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, ch := range s {
		switch ch {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case rune(q):
			b.WriteString(`\` + string(q))
		default:
			b.WriteRune(ch)
		}
	}
	b.WriteByte(q)
	return b.String()
}

func cdfJoinComma(xs []string) string { return strings.Join(xs, ", ") }

// ─────────────────────────── --list-rules ───────────────────────────

func cdfCmdListRules(w io.Writer, cfg map[string]any, t *cdfTree, args *cdfArgs) int {
	fmt.Fprintf(w, "check-doc-freshness.py v1.0.0 —— 规则表 / 名单 / 退码（--list-rules）\n")
	repo := dcmdDefaultRepo()
	if t != nil {
		repo = t.repo
	}
	fmt.Fprintf(w, "仓根：%s\n", repo)
	fmt.Fprintf(w, "规格：Zerg-内部文档/01-设计/设计-文档体系-v1.0.md §6.3 §9.1 §5.6① §16 · Zerg-内部文档/调研/调研-文档分块落地-门禁与迁移.md §1 §2.3\n")
	fmt.Fprintf(w, "\n【三档退码】0 全绿 · 1 有失败项 · 2 不给结论（缺件/空转/用法错/判据不可判）\n")
	fmt.Fprintf(w, "依据：scripts/gates/precommit-gates.sh:29（仓内既有语义；本脚本不改它，只对齐语义）\n")
	for _, gid := range []string{"d1", "d2", "d3", "d4"} {
		g := cdfGates[gid]
		fmt.Fprintf(w, "\n── %s\n   入参：%s\n   判定：%s\n   退码：0 %s / 1 %s / 2 %s\n   修法：%s\n",
			strings.ToUpper(gid), g.inputs, g.judge, g.rc0, g.rc1, g.rc2, g.fix)
	}
	fmt.Fprintf(w, "\n【归一化白名单（不算引用的类别 · 必须计数）】\n")
	if nw, ok := cfg["normalize_whitelist"].(map[string]any); ok {
		ks := []string{}
		for k := range nw {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			v := ""
			if s, ok := nw[k].(string); ok {
				v = s
			}
			fmt.Fprintf(w, "  · %-14s %s\n", k, v)
		}
	}
	fmt.Fprintf(w, "\n【排除名单（不计入、也不判红）】\n")
	fmt.Fprintf(w, "  · 硬排除（永不扫）   ：%s\n", cdfJoinComma(cdfStrList(cfg, "hard_exclude")))
	mirror := cdfStrList(cfg, "mirror_paths")
	if len(mirror) == 0 {
		fmt.Fprintf(w, "  · 镜像副本           ：%s\n", "（无 —— `docs/虫族文档` 已于 2026-09-19 退役，条目已从 config 删除；镜像重建请加回本列）")
	} else {
		fmt.Fprintf(w, "  · 镜像副本           ：%s\n", cdfJoinComma(mirror))
	}
	frozen := cdfStrList(cfg, "frozen_paths")
	fmt.Fprintf(w, "  · 冻结归档（只登记） ：%d 套：%s\n", len(frozen), cdfJoinComma(frozen))
	fmt.Fprintf(w, "  · 第三方源码摘录     ：%s\n", cdfJoinComma(cdfStrList(cfg, "third_party_paths")))
	fmt.Fprintf(w, "  · 引擎数据           ：%s\n", cdfJoinComma(cdfStrList(cfg, "engine_data_paths")))
	fixtures := cdfStrList(cfg, "fixture_paths")
	if len(fixtures) == 0 {
		fmt.Fprintf(w, "  · 夹具（仓内为零）   ：%s\n", "（自检在 $TMPDIR 造件，仓内不落夹具）")
	} else {
		fmt.Fprintf(w, "  · 夹具（仓内为零）   ：%s\n", cdfJoinComma(fixtures))
	}
	fmt.Fprintf(w, "\n【引用域（--strict-all 才判红；默认单列计数）】\n")
	fmt.Fprintf(w, "  · 外部引用域         ：%s\n", cdfJoinComma(cdfStrList(cfg, "external_domain_paths")))
	fmt.Fprintf(w, "\n【解析基准集（base_roots，缺一条就出假红）】\n")
	nz := []string{}
	for _, r := range cdfStrList(cfg, "base_roots") {
		if r != "" {
			nz = append(nz, r)
		}
	}
	fmt.Fprintf(w, "  · 仓根 + %s\n", cdfJoinComma(nz))
	var sib string
	if t != nil {
		sib = cdfSiblingDocsRoot(t)
	} else {
		sib = dcmdJoin(dcmdDir(dcmdAbs(dcmdDefaultRepo())), "Zerg-内部文档")
	}
	onDisk := "**不在盘** ⇒ 外根形态的引用判不了 = rc=2 BLOCKED、逐字打印缺件路径"
	if fi, err := os.Stat(sib); err == nil && fi.IsDir() {
		onDisk = "在盘"
	}
	fmt.Fprintf(w, "  · ★ 仓外根（同级开发文档仓 · 分家后那三类外根形态的落点，见 sibling_docs_root()）：%s（%s）\n", sib, onDisk)
	rp := true
	if v, ok := cdfSub(cfg, "d2")["resolve_package_relative"].(bool); ok {
		rp = v
	}
	fmt.Fprintf(w, "  · D2 包内相对路径：唯一后缀匹配=%s（api/handlers.go → core/internal/api/handlers.go）\n", cdfPyBool(rp))
	fmt.Fprintf(w, "\n【slug 规范】profile=%s（§5.6① 八条：NFC · lower · 保 \\p{L}\\p{M}\\p{N}_- · 空白→'-' · 空兜底 s<n> · 重复加 -1/-2 · 引用一律 path#slug · 钉死版本+自测）\n",
		cdfAnyStr(cdfSub(cfg, "d3"), "slug_profile", ""))
	fmt.Fprintf(w, "  规范落盘处：%s（不存在 ⇒ rc=2）\n", cdfAnyStr(cdfSub(cfg, "d3"), "spec_doc", ""))
	fmt.Fprintf(w, "\n【生成器清单（D4）】\n")
	for _, g := range cdfSubList(cdfSub(cfg, "d4"), "generators") {
		fmt.Fprintf(w, "  · %-12s %s\n", cdfAnyStr(g, "id", ""), cdfAnyStr(g, "name", ""))
		fmt.Fprintf(w, "      命令：%s\n", strings.Join(cdfStrList(g, "cmd"), " "))
		outs := cdfAnyStr(g, "output", "")
		if outs == "" {
			outs = strings.Join(cdfStrList(g, "targets_glob"), ", ")
		}
		fmt.Fprintf(w, "      产物：%s\n", outs)
		fmt.Fprintf(w, "      修法：%s\n", cdfAnyStr(g, "fix", ""))
	}
	fmt.Fprintf(w, "\n【豁免表（allowlist · 必须带理由 + 日期）】\n")
	al := cdfSubList(cfg, "allowlist")
	if len(al) == 0 {
		fmt.Fprintf(w, "  （空）\n")
	}
	for _, e := range al {
		fmt.Fprintf(w, "  · [%s] %s  ↦ 目标 %s\n      理由：%s  · 登记日：%s\n",
			cdfAnyStr(e, "gate", "*"), cdfAnyStr(e, "file_glob", "*"), cdfPyRepr(cdfAnyStr(e, "target", "")),
			cdfAnyStr(e, "reason", ""), cdfAnyStr(e, "date", ""))
	}
	fmt.Fprintf(w, "\n【拒绝域（明确判不了 / 不判的）】\n")
	for _, line := range cdfStrList(cfg, "refuse_domain") {
		fmt.Fprintf(w, "  · %s\n", line)
	}
	return cdfRCOK
}

// ─────────────────────────── --self-test ───────────────────────────

const cdfSelftestGenerator = `#!/usr/bin/env python3
import sys, os
d = sys.argv[1]
out = sys.argv[sys.argv.index('--out')+1] if '--out' in sys.argv else os.path.join(d,'INDEX.md')
names = sorted(n for n in os.listdir(d) if n.endswith('.md') and n != 'INDEX.md')
with open(out,'w',encoding='utf-8') as f:
    f.write('# %s 版本索引\n\n' % os.path.basename(os.path.normpath(d)))
    for n in names:
        f.write('| [%s](%s) |\n' % (n, n))
`

func cdfMkroot(files map[string]string, root string) string {
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(content), 0o644)
	}
	return root
}

func cdfBaseCfg(root string) map[string]any {
	_ = root
	return map[string]any{
		"version":               "selftest",
		"corpus":                []any{"docs"},
		"base_roots":            []any{"", "core", "docs", "tools"},
		"hard_exclude":          []any{".git", "node_modules", "target"},
		"mirror_paths":          []any{"docs/虫族文档"},
		"frozen_paths":          []any{},
		"third_party_paths":     []any{},
		"engine_data_paths":     []any{},
		"fixture_paths":         []any{},
		"external_domain_paths": []any{},
		"d2":                    map[string]any{"resolve_package_relative": true},
		"d3":                    map[string]any{"slug_profile": "github", "spec_doc": "docs/设计.md", "include_wiki_links": false},
		"d4":                    map[string]any{"generators": []any{}},
	}
}

func cdfCmdSelfTest(w io.Writer, args *cdfArgs) int {
	fails := []string{}
	nOK := 0
	check := func(label string, cond bool, extra string) {
		if cond {
			nOK++
			fmt.Fprintf(w, "  ✓ %s\n", label)
		} else {
			fails = append(fails, label)
			fmt.Fprintf(w, "  ✗ %s  %s\n", label, extra)
		}
	}

	tmp, err := os.MkdirTemp("", "docfresh-selftest-")
	if err != nil {
		fmt.Fprintf(w, "\nself-test：0 条通过 / 1 条失败\n  ✗ 夹具建不起来\n")
		return cdfRCFail
	}
	defer os.RemoveAll(tmp)

	// ---------- D1 ----------
	fmt.Fprintf(w, "[D1] 三格\n")
	r1 := filepath.Join(tmp, "d1good")
	cdfMkroot(map[string]string{"docs/a.md": "[好](b.md)\n", "docs/b.md": "# b\n"}, r1)
	t := cdfNewTree(r1, cdfBaseCfg(r1), "main", false, false, "")
	res := cdfGateD1(t, args)
	check("好件 rc=0", res.rc() == 0, strconv.Itoa(res.rc()))
	r2 := filepath.Join(tmp, "d1bad")
	cdfMkroot(map[string]string{"docs/a.md": "[坏](missing.md)\n"}, r2)
	t = cdfNewTree(r2, cdfBaseCfg(r2), "main", false, false, "")
	res = cdfGateD1(t, args)
	check("坏件 rc=1", res.rc() == 1, strconv.Itoa(res.rc()))
	check("坏件命中 1 条断链", len(res.reds()) == 1, "")
	r3 := filepath.Join(tmp, "d1empty")
	_ = os.MkdirAll(filepath.Join(r3, "docs"), 0o755)
	t = cdfNewTree(r3, cdfBaseCfg(r3), "main", false, false, "")
	res = cdfGateD1(t, args)
	check("缺件（空语料）rc=2", res.rc() == 2, strconv.Itoa(res.rc()))
	r3b := filepath.Join(tmp, "d1nodocs")
	_ = os.MkdirAll(r3b, 0o755)
	t = cdfNewTree(r3b, cdfBaseCfg(r3b), "main", false, false, "")
	res = cdfGateD1(t, args)
	hasMissDocs := false
	for _, e := range res.errors {
		if strings.Contains(e, "缺件") && strings.Contains(e, "docs") {
			hasMissDocs = true
		}
	}
	check("缺件（语料根 docs/ 不存在 ⇒ 公开树侧）rc=2 且点名缺件路径", res.rc() == 2 && hasMissDocs, strings.Join(res.errors, " · "))

	// ---------- D2 ----------
	fmt.Fprintf(w, "[D2] 三格\n")
	r1 = filepath.Join(tmp, "d2good")
	cdfMkroot(map[string]string{"docs/a.md": "见 `core/x.py:3`。\n", "core/x.py": "l1\nl2\nl3\nl4\n"}, r1)
	t = cdfNewTree(r1, cdfBaseCfg(r1), "main", false, false, "")
	res = cdfGateD2(t, args)
	check("好件 rc=0", res.rc() == 0, fmt.Sprintf("(%d, %v)", res.rc(), res.counts))
	r2 = filepath.Join(tmp, "d2bad")
	cdfMkroot(map[string]string{"docs/a.md": "见 `core/x.py:99` 与 `core/nope.py:1`。\n", "core/x.py": "l1\nl2\nl3\n"}, r2)
	t = cdfNewTree(r2, cdfBaseCfg(r2), "main", false, false, "")
	res = cdfGateD2(t, args)
	check("坏件 rc=1", res.rc() == 1, fmt.Sprintf("(%d, %v)", res.rc(), res.counts))
	check("坏件 2 条红（越界 + 不存在）", len(res.reds()) == 2, strconv.Itoa(len(res.reds())))
	r2b := filepath.Join(tmp, "d2noise")
	cdfMkroot(map[string]string{"docs/a.md": "连通性 `<controller-ip>:8580`、省略号 `a/...x.py:1`、占位符 `<file>.py:1`。\n"}, r2b)
	t = cdfNewTree(r2b, cdfBaseCfg(r2b), "main", false, false, "")
	res = cdfGateD2(t, args)
	check("噪声只计数不判红（三类各 1）", res.counts["噪声·IP:端口"] == 1 &&
		res.counts["噪声·省略号路径"] == 1 && res.counts["噪声·占位符"] == 1, fmt.Sprintf("%v", res.counts))
	check("全是噪声 ⇒ 空转 rc=2（噪声洗白不了）", res.rc() == 2, fmt.Sprintf("(%d, %v)", res.rc(), res.errors))
	r2c := filepath.Join(tmp, "d2countgood")
	cdfMkroot(map[string]string{
		"docs/a.md": "见 `core/x.py:2`。**条数**: 2 条 `/api/x/*`（现跑 `grep -c 'def ' core/x.py`）\n",
		"core/x.py": "def a\nb\ndef c\n"}, r2c)
	t = cdfNewTree(r2c, cdfBaseCfg(r2c), "main", false, false, "")
	res = cdfGateD2(t, args)
	check("计数对拍·好件（声明 2 == 现跑 2）rc=0", res.rc() == 0, fmt.Sprintf("(%d, %v, %v)", res.rc(), res.counts, cdfKinds(res.reds())))
	check("计数对拍·好件：计数条目 1 条且判「对得上」",
		res.counts["计数条目"] == 1 && res.counts["计数条目·对得上"] == 1, fmt.Sprintf("%v", res.counts))
	r2d := filepath.Join(tmp, "d2countbad")
	cdfMkroot(map[string]string{
		"docs/a.md": "见 `core/x.py:2`。**条数**: 3 条 `/api/x/*`（现跑 `grep -c 'def ' core/x.py`）\n",
		"core/x.py": "def a\nb\ndef c\n"}, r2d)
	t = cdfNewTree(r2d, cdfBaseCfg(r2d), "main", false, false, "")
	res = cdfGateD2(t, args)
	check("计数对拍·坏件（声明 3 ≠ 现跑 2）rc=1", res.rc() == 1, fmt.Sprintf("(%d, %v)", res.rc(), res.counts))
	hasCountRed := false
	for _, f := range res.reds() {
		if f.kind == "红·计数与现跑不符" {
			hasCountRed = true
		}
	}
	check("计数对拍·坏件：红条目点名「计数与现跑不符」", hasCountRed, fmt.Sprintf("%v", cdfKinds(res.reds())))
	r2e := filepath.Join(tmp, "d2countmiss")
	cdfMkroot(map[string]string{
		"docs/a.md": "见 `core/x.py:1`。**条数**: 1 条（现跑 `grep -c 'def ' core/nope.py`）\n",
		"core/x.py": "def a\n"}, r2e)
	t = cdfNewTree(r2e, cdfBaseCfg(r2e), "main", false, false, "")
	res = cdfGateD2(t, args)
	anyCountRed := false
	for _, f := range res.reds() {
		if strings.Contains(f.kind, "计数") {
			anyCountRed = true
		}
	}
	check("计数对拍·判不了（目标不在盘上）⇒ 单列计数、rc=0、**不红**",
		res.rc() == 0 && res.counts["计数条目·判不了"] == 1 && !anyCountRed,
		fmt.Sprintf("(%d, %v, %v)", res.rc(), res.counts, res.notes))
	r3 = filepath.Join(tmp, "d2miss")
	c := cdfBaseCfg(r3)
	c["base_roots"] = []any{}
	_ = os.MkdirAll(filepath.Join(r3, "docs"), 0o755)
	cdfMkroot(map[string]string{"docs/a.md": "`core/x.py:1`\n"}, r3)
	t = cdfNewTree(r3, c, "main", false, false, "")
	res = cdfGateD2(t, args)
	check("缺件（基准集缺失）rc=2", res.rc() == 2, fmt.Sprintf("(%d, %v)", res.rc(), res.errors))
	r4 := filepath.Join(tmp, "d2nodocs")
	_ = os.MkdirAll(r4, 0o755)
	t = cdfNewTree(r4, cdfBaseCfg(r4), "main", false, false, "")
	res = cdfGateD2(t, args)
	hasMissDocs = false
	for _, e := range res.errors {
		if strings.Contains(e, "缺件") && strings.Contains(e, "docs") {
			hasMissDocs = true
		}
	}
	check("缺件（语料根 docs/ 不存在 ⇒ 公开树侧）rc=2 且点名缺件路径", res.rc() == 2 && hasMissDocs, strings.Join(res.errors, " · "))

	// 缺语料铁律档
	tmp2, _ := os.MkdirTemp("", "docfresh-selftest-ext-")
	defer os.RemoveAll(tmp2)
	sx := filepath.Join(tmp2, "repo")
	cdfMkroot(map[string]string{"docs/a.md": "见 `项目文档/" + cdfSlugVersionDir + "/x.md:3` 与 `Zerg-内部文档/项目文档/" + cdfSlugVersionDir + "/x.md:3`" +
		" 与 `core/nope.py:1`。\n"}, sx)
	t = cdfNewTree(sx, cdfBaseCfg(sx), "main", false, false, "")
	res = cdfGateD2(t, args)
	check("缺语料·外根形态（② + ①）+ 仓外根不在盘 ⇒ rc=2、**那两条 0 红**",
		res.rc() == 2 && len(res.reds()) == 1 && res.counts["不给结论·仓外根不在盘"] == 2,
		fmt.Sprintf("(%d, %v, %v)", res.rc(), res.counts, cdfKinds(res.reds())))
	sibRoot := cdfSiblingDocsRoot(t)
	anySib := false
	for _, e := range res.errors {
		if strings.Contains(e, sibRoot) {
			anySib = true
		}
	}
	check(fmt.Sprintf("缺语料·报错逐字打印缺件路径（同级仓根 %s）", sibRoot), anySib, fmt.Sprintf("%v", res.errors))
	innerOK := true
	rs := res.reds()
	if len(rs) != 1 || rs[0].kind != "红·文件不存在" || !strings.HasPrefix(rs[0].target, "core/nope.py") {
		innerOK = false
	}
	check("缺语料·**仓内**引用不跟着进 BLOCKED（`core/nope.py:1` 仍判红·文件不存在）", innerOK,
		fmt.Sprintf("%v", cdfKindTargets(rs)))
	cdfMkroot(map[string]string{"项目文档/" + cdfSlugVersionDir + "/x.md": "l1\nl2\nl3\nl4\n"}, filepath.Join(tmp2, "Zerg-内部文档"))
	res2 := cdfGateD2(cdfNewTree(sx, cdfBaseCfg(sx), "main", false, false, ""), args)
	rrs := res2.reds()
	pairOK := res2.counts["不给结论·仓外根不在盘"] == 0 && len(rrs) == 1 && strings.HasPrefix(rrs[0].target, "core/nope.py")
	check("成对负控·外根在盘 ⇒ 同一个件（①/② 形态）照旧判过（本档整体不生效）", pairOK,
		fmt.Sprintf("(%d, %v, %v)", res2.rc(), res2.counts, cdfKindTargets(rrs)))
	sx3 := filepath.Join(tmp2, "repo3")
	cdfMkroot(map[string]string{"docs/a.md": "见 `项目文档/" + cdfSlugVersionDir + "/x.md:99` 与 `项目文档/" + cdfSlugVersionDir + "/真不存在-x.md:1`。\n"}, sx3)
	res3 := cdfGateD2(cdfNewTree(sx3, cdfBaseCfg(sx3), "main", false, false, ""), args)
	kinds3 := cdfKinds(res3.reds())
	sort.Strings(kinds3)
	want3 := []string{"红·行号越界", "红·文件不存在"}
	sort.Strings(want3)
	check("成对负控·外根在盘 ⇒ 真存在件改坏行号仍「红·行号越界」、真缺件仍「红·文件不存在」",
		cdfSliceEq(kinds3, want3) && res3.rc() == 1,
		fmt.Sprintf("(%d, %v)", res3.rc(), cdfKindTargets(res3.reds())))

	// ---------- D3 ----------
	fmt.Fprintf(w, "[D3] 三格\n")
	r1 = filepath.Join(tmp, "d3good")
	cdfMkroot(map[string]string{"docs/设计.md": "# 设计\n", "docs/a.md": "[好](设计.md#设计)\n"}, r1)
	t = cdfNewTree(r1, cdfBaseCfg(r1), "main", false, false, "")
	res = cdfGateD3(t, args)
	check("好件 rc=0", res.rc() == 0, fmt.Sprintf("(%d, %v, %v)", res.rc(), res.counts, cdfDetails(res.reds())))
	r2 = filepath.Join(tmp, "d3bad")
	cdfMkroot(map[string]string{"docs/设计.md": "# 设计\n", "docs/a.md": "[坏](设计.md#不存在)\n"}, r2)
	t = cdfNewTree(r2, cdfBaseCfg(r2), "main", false, false, "")
	res = cdfGateD3(t, args)
	check("坏件 rc=1", res.rc() == 1, fmt.Sprintf("(%d, %v)", res.rc(), res.counts))
	r3 = filepath.Join(tmp, "d3miss")
	c = cdfBaseCfg(r3)
	c["d3"].(map[string]any)["spec_doc"] = "docs/规范不存在.md"
	_ = os.MkdirAll(filepath.Join(r3, "docs"), 0o755)
	cdfMkroot(map[string]string{"docs/a.md": "# a\n"}, r3)
	t = cdfNewTree(r3, c, "main", false, false, "")
	res = cdfGateD3(t, args)
	check("缺件（规范未落盘）rc=2", res.rc() == 2, fmt.Sprintf("(%d, %v)", res.rc(), res.errors))
	cases := [][2]string{
		{"一、为什么升级(实测痛点)", "一为什么升级实测痛点"},
		{"§4.2 三条机制", "42-三条机制"},
		{"零、逐字取证（六项的原文，一字不改）", "零逐字取证六项的原文一字不改"},
		{"甲批（T1–T5）—— ✅ 已落地", "甲批t1t5--已落地"},
		{"6. 观测面", "6-观测面"},
		{"a  b", "a--b"},
	}
	bad := false
	for _, cs := range cases {
		if cdfSlugify(cs[0], "github") != cs[1] {
			bad = true
		}
	}
	check(fmt.Sprintf("slug 冻结用例 %d 条", len(cases)), !bad, "")
	r4 = filepath.Join(tmp, "d3dup")
	cdfMkroot(map[string]string{"docs/设计.md": "# 设计\n\n## 重复\n\n## 重复\n\n## ###\n"}, r4)
	t = cdfNewTree(r4, cdfBaseCfg(r4), "main", false, false, "")
	slugs, _, _ := cdfFileAnchors(t, "docs/设计.md", "github")
	got := []string{}
	for _, s := range slugs {
		got = append(got, s.slug)
	}
	hasDup1 := false
	for _, s := range got {
		if s == "重复-1" {
			hasDup1 = true
		}
	}
	check("同文件重复 slug 加 -1", hasDup1, fmt.Sprintf("%v", got))
	allNonEmpty := true
	for _, s := range got {
		if s == "" {
			allNonEmpty = false
		}
	}
	check("空 slug 兜底 s<n>", allNonEmpty, fmt.Sprintf("%v", got))

	// ---------- D4 ----------
	fmt.Fprintf(w, "[D4] 三格\n")
	gen := "tools/gen.py"
	r1 = filepath.Join(tmp, "d4good")
	cdfMkroot(map[string]string{"tools/gen.py": cdfSelftestGenerator,
		"docs/v1/INDEX.md": "# v1 版本索引\n\n| [a.md](a.md) |\n", "docs/v1/a.md": "x\n"}, r1)
	c = cdfBaseCfg(r1)
	c["d4"] = map[string]any{"generators": []any{map[string]any{"id": "g", "name": "自检生成器",
		"cmd":  []any{"python3", "{REPO}/tools/gen.py", "{DIR}", "--out", "{OUT}"},
		"mode": "out-flag", "output": "docs/v1/INDEX.md", "script": gen, "fix": "重跑 gen.py"}}}
	t = cdfNewTree(r1, c, "main", false, false, "")
	res = cdfGateD4(t, args)
	check("好件 rc=0", res.rc() == 0, fmt.Sprintf("(%d, %v, %v)", res.rc(), res.counts, res.errors))
	r2 = filepath.Join(tmp, "d4bad")
	cdfMkroot(map[string]string{"tools/gen.py": cdfSelftestGenerator,
		"docs/v1/INDEX.md": "# v1 版本索引（手改过）\n\n| [a.md](a.md) |\n", "docs/v1/a.md": "x\n"}, r2)
	c = cdfBaseCfg(r2)
	c["d4"] = map[string]any{"generators": []any{map[string]any{"id": "g", "name": "自检生成器",
		"cmd":  []any{"python3", "{REPO}/tools/gen.py", "{DIR}", "--out", "{OUT}"},
		"mode": "out-flag", "output": "docs/v1/INDEX.md", "script": gen, "fix": "重跑 gen.py"}}}
	t = cdfNewTree(r2, c, "main", false, false, "")
	res = cdfGateD4(t, args)
	check("坏件 rc=1", res.rc() == 1, fmt.Sprintf("(%d, %v)", res.rc(), res.counts))
	hasFix := false
	for _, f := range res.reds() {
		if strings.Contains(f.detail, "重跑 gen.py") {
			hasFix = true
		}
	}
	check("报错带修法命令", hasFix, fmt.Sprintf("%v", res.reds()))
	r3 = filepath.Join(tmp, "d4miss")
	cdfMkroot(map[string]string{"docs/v1/INDEX.md": "# v1\n"}, r3)
	c = cdfBaseCfg(r3)
	c["d4"] = map[string]any{"generators": []any{map[string]any{"id": "g", "name": "缺生成器",
		"cmd":  []any{"python3", "{REPO}/tools/gen.py", "{DIR}"},
		"mode": "out-flag", "output": "docs/v1/INDEX.md", "script": gen, "fix": "x"}}}
	t = cdfNewTree(r3, c, "main", false, false, "")
	res = cdfGateD4(t, args)
	check("缺件（缺生成器）rc=2", res.rc() == 2, fmt.Sprintf("(%d, %v)", res.rc(), res.errors))
	check("三档 0/1/2 各就各位（D1 三格）", true, "")

	rd := cdfMkroot(map[string]string{"总览.md": "# 总览\n\n见 [a](01-设计/a.md)\n",
		"01-设计/a.md": "# a\n\n见 [b](01-设计/b.md)\n",
		"01-设计/b.md": "# b\n", "notes/x.txt": "x\n"}, filepath.Join(tmp, "devroot"))
	cd := cdfBaseCfg(rd)
	td := cdfNewTree(rd, cd, "devdocs", false, false, "")
	check("D3① devdocs 档：**根级 md 也进语料**（整棵 3 篇 · 首跑实测漏的就是这一格）",
		len(td.corpus) == 3, fmt.Sprintf("%v", td.corpus))
	check("D3① devdocs 档：解析域覆盖整棵根（4 件）", len(td.files) == 4, fmt.Sprintf("%v", td.files))
	check("D3① 成对：同一棵树在 main 档下语料 0 篇（换根不是空转）",
		len(cdfNewTree(rd, cd, "main", false, false, "").corpus) == 0, fmt.Sprintf("%v", cdfNewTree(rd, cd, "main", false, false, "").corpus))
	rd1 := cdfGateD1(td, args)
	check("D3① devdocs 档：D1 在同表内引用可解析（无红）", len(rd1.reds()) == 0, fmt.Sprintf("%v", rd1.reds()))

	fmt.Fprintf(w, "[D4] 生成器脚本的解析基准（主仓根 · 换根档）\n")
	main := filepath.Join(tmp, "d4main")
	cdfMkroot(map[string]string{"tools/mygen.py": "print('x')\n"}, main)
	doc := filepath.Join(tmp, "d4docs")
	cdfMkroot(map[string]string{"docs/a.md": "# a\n"}, doc)
	cfg4 := cdfBaseCfg(doc)
	cfg4["d4"] = map[string]any{"generators": []any{map[string]any{"id": "mygen", "script": "tools/mygen.py",
		"output": "docs/INDEX.md", "optional": true}}}
	res = cdfGateD4(cdfNewTree(doc, cfg4, "devdocs", false, false, main), args)
	noMissScript := true
	for k := range res.counts {
		if strings.Contains(k, "缺脚本") {
			noMissScript = false
		}
	}
	check("主仓在盘的脚本不判「缺脚本」（换根档）", noMissScript, fmt.Sprintf("%v", res.counts))
	hasMissOut := false
	for k := range res.counts {
		if strings.Contains(k, "产物不存在") {
			hasMissOut = true
		}
	}
	check("但产物真缺 ⇒ 仍报「产物不存在」", hasMissOut, fmt.Sprintf("%v", res.counts))
	cfg4b := cdfBaseCfg(doc)
	cfg4b["d4"] = map[string]any{"generators": []any{map[string]any{"id": "mygen2", "script": "tools/nope.py",
		"output": "docs/INDEX.md", "optional": true}}}
	res2b := cdfGateD4(cdfNewTree(doc, cfg4b, "devdocs", false, false, main), args)
	hasMissScript := false
	for k := range res2b.counts {
		if strings.Contains(k, "缺脚本") {
			hasMissScript = true
		}
	}
	check("负控：两个根都找不到的脚本 ⇒ 仍报「缺脚本」", hasMissScript, fmt.Sprintf("%v", res2b.counts))

	fmt.Fprintf(w, "\nself-test：%d 条通过 / %d 条失败\n", nOK, len(fails))
	if len(fails) > 0 {
		for _, f := range fails {
			fmt.Fprintf(w, "  ✗ %s\n", f)
		}
		return cdfRCFail
	}
	return cdfRCOK
}

func cdfKinds(fs []cdfFinding) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.kind)
	}
	return out
}
func cdfDetails(fs []cdfFinding) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, f.detail)
	}
	return out
}
func cdfKindTargets(fs []cdfFinding) [][2]string {
	out := [][2]string{}
	for _, f := range fs {
		out = append(out, [2]string{f.kind, f.target})
	}
	return out
}

// ─────────────────────────── main ───────────────────────────

var cdfJSONFields = []string{}

func cdfRefuseJSON(argv []string, stderr io.Writer) (int, bool) {
	hits := []string{}
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hits = append(hits, "`"+x+"`")
		}
	}
	if len(hits) == 0 {
		return 0, false
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(hits, " · "))
	if len(cdfJSONFields) == 0 {
		fmt.Fprintf(stderr, "   字段表：%s\n", "无（本门只出人读面）")
	} else {
		q := []string{}
		for _, f := range cdfJSONFields {
			q = append(q, "`"+f+"`")
		}
		fmt.Fprintf(stderr, "   字段表：%s\n", strings.Join(q, " · "))
	}
	fmt.Fprintf(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return cdfRCBlocked, true
}

const cdfUsageLine = `usage: check-doc-freshness.py [-h] [--repo REPO] [--config CONFIG]
                              [--scope {main,wide,mirror,devdocs}]
                              [--strict-all] [--slug-profile {github,docv1.2}]
                              [--no-whitelist] [--report REPORT]
                              [--print-limit PRINT_LIMIT] [--list-rules]
                              [--self-test] [--verbose]
                              [gates ...]`

func cdfArgErr(stderr io.Writer, msg string) {
	fmt.Fprintf(stderr, "%s\n", cdfUsageLine)
	fmt.Fprintf(stderr, "%s: error: %s\n", cdfProg, msg)
}

func cdfLoadCfg(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func cdfParseArgs(argv []string, stdout, stderr io.Writer) (*cdfArgs, int, bool) {
	a := &cdfArgs{repo: dcmdDefaultRepo(), config: "", scope: "main", printLimit: 40}
	cwd, _ := os.Getwd()
	a.config = filepath.Join(cwd, filepath.FromSlash(cdfConfigRel))
	if r, _ := dcmdDefaultRepoRel(cdfConfigRel); r != "" {
		a.config = r
	}
	scopes := map[string]bool{"main": true, "wide": true, "mirror": true, "devdocs": true}
	profiles := map[string]bool{"github": true, "docv1.2": true}
	i := 0
	needVal := func(name string) (string, bool) {
		if i+1 < len(argv) {
			i++
			return argv[i], true
		}
		cdfArgErr(stderr, fmt.Sprintf("argument %s: expected one argument", name))
		return "", false
	}
	for ; i < len(argv); i++ {
		x := argv[i]
		if x == "-h" || x == "--help" {
			fmt.Fprint(stdout, cdfHelpText)
			return nil, cdfRCOK, true
		}
		if !strings.HasPrefix(x, "-") {
			a.gates = append(a.gates, x)
			continue
		}
		name := x
		val := ""
		hasVal := false
		if k := strings.Index(x, "="); k >= 0 {
			name = x[:k]
			val = x[k+1:]
			hasVal = true
		}
		get := func() (string, bool) {
			if hasVal {
				return val, true
			}
			return needVal(name)
		}
		switch name {
		case "--repo":
			v, ok := get()
			if !ok {
				return nil, cdfRCBlocked, true
			}
			a.repo = v
		case "--config":
			v, ok := get()
			if !ok {
				return nil, cdfRCBlocked, true
			}
			a.config = v
		case "--scope":
			v, ok := get()
			if !ok {
				return nil, cdfRCBlocked, true
			}
			if !scopes[v] {
				cdfArgErr(stderr, fmt.Sprintf("argument --scope: invalid choice: '%s' (choose from 'main', 'wide', 'mirror', 'devdocs')", v))
				return nil, cdfRCBlocked, true
			}
			a.scope = v
		case "--slug-profile":
			v, ok := get()
			if !ok {
				return nil, cdfRCBlocked, true
			}
			if !profiles[v] {
				cdfArgErr(stderr, fmt.Sprintf("argument --slug-profile: invalid choice: '%s' (choose from 'github', 'docv1.2')", v))
				return nil, cdfRCBlocked, true
			}
			a.slugProfile = v
		case "--report":
			v, ok := get()
			if !ok {
				return nil, cdfRCBlocked, true
			}
			a.report = v
		case "--print-limit":
			v, ok := get()
			if !ok {
				return nil, cdfRCBlocked, true
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				cdfArgErr(stderr, fmt.Sprintf("argument --print-limit: invalid int value: '%s'", v))
				return nil, cdfRCBlocked, true
			}
			a.printLimit = n
		case "--strict-all":
			a.strictAll = true
		case "--no-whitelist":
			a.noWhitelist = true
		case "--list-rules":
			a.listRules = true
		case "--self-test":
			a.selfTest = true
		case "--verbose":
			a.verbose = true
		default:
			cdfArgErr(stderr, fmt.Sprintf("unrecognized arguments: %s", x))
			return nil, cdfRCBlocked, true
		}
	}
	return a, 0, false
}

// dcmdDefaultRepoRel —— 仓根 + 仓内相对路径。
func dcmdDefaultRepoRel(rel string) (string, bool) {
	root := dcmdDefaultRepo()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(p); err == nil {
		return p, true
	}
	return "", false
}

const cdfHelpText = `usage: check-doc-freshness.py [-h] [--repo REPO] [--config CONFIG]
                              [--scope {main,wide,mirror,devdocs}]
                              [--strict-all] [--slug-profile {github,docv1.2}]
                              [--no-whitelist] [--report REPORT]
                              [--print-limit PRINT_LIMIT] [--list-rules]
                              [--self-test] [--verbose]
                              [gates ...]

W3 四条新鲜度门禁 D1–D4（只读 · 只报不改）

positional arguments:
  gates                 d1 d2 d3 d4 all（可多个）

optional arguments:
  -h, --help            show this help message and exit
  --repo REPO
  --config CONFIG
  --scope {main,wide,mirror,devdocs}
                        main=主树（默认，去镜像/冻结归档/第三方源码/引擎数据）；wide=+冻结归档+第三方源码；mirro
                        r=+镜像副本；devdocs=**换到开发文档面**（D3①：Zerg-内部文档 整棵，根见
                        devdocs_root()）
  --strict-all          非本地域（外部引用域/冻结归档/镜像）也判红
  --slug-profile {github,docv1.2}
                        覆盖 config.d3.slug_profile（用于实测 §5.6③④ 的 U+3000 分歧）
  --no-whitelist        D1 关掉软白名单（语法示例/省略号/代码块内）—— 用于实测「不做白名单会有多少假红」
  --report REPORT       结果 JSON 落盘路径（建议 /tmp/…，勿写仓内）
  --print-limit PRINT_LIMIT
  --list-rules
  --self-test
  --verbose
`

// checkDocFreshnessGateMain —— 真搬入口（M2 刻意无调用点）。
func checkDocFreshnessGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc, done := cdfRefuseJSON(argv, stderr); done {
		return rc
	}
	args, rc, done := cdfParseArgs(argv, stdout, stderr)
	if done {
		return rc
	}
	if args.selfTest {
		return cdfCmdSelfTest(stdout, args)
	}
	if args.scope == "devdocs" {
		d := cdfDevdocsRoot(dcmdDefaultRepo())
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			fmt.Fprintf(stderr, "开发文档面根不在 ⇒ rc=2 不给结论：%s（换根请设 ZERG_DEVDOCS_ROOT；口径见设计稿 §五）\n", d)
			return cdfRCBlocked
		}
		args.repo = d
	}

	var cfg map[string]any
	if _, err := os.Stat(args.config); err == nil {
		m, err := cdfLoadCfg(args.config)
		if err != nil {
			fmt.Fprintf(stderr, "配置读不了（%s）⇒ rc=2 不给结论：%s\n", args.config, err)
			return cdfRCBlocked
		}
		cfg = m
	} else {
		if !args.listRules {
			fmt.Fprintf(stderr, "缺配置文件 %s（判据不可判）⇒ rc=2 不给结论\n", args.config)
			return cdfRCBlocked
		}
		cfg = map[string]any{"base_roots": []any{"", "docs"}, "corpus": []any{"docs"}, "hard_exclude": []any{},
			"mirror_paths": []any{}, "frozen_paths": []any{}, "third_party_paths": []any{}, "engine_data_paths": []any{},
			"fixture_paths": []any{}, "external_domain_paths": []any{}, "d2": map[string]any{}, "d3": map[string]any{},
			"d4": map[string]any{"generators": []any{}}, "normalize_whitelist": map[string]any{}, "refuse_domain": []any{}}
	}

	if args.listRules {
		var t *cdfTree
		func() {
			defer func() { _ = recover() }()
			t = cdfBuildTree(args, cfg)
		}()
		return cdfCmdListRules(stdout, cfg, t, args)
	}

	if args.slugProfile != "" {
		if _, ok := cfg["d3"].(map[string]any); !ok {
			cfg["d3"] = map[string]any{}
		}
		cfg["d3"].(map[string]any)["slug_profile"] = args.slugProfile
	}

	if errs := cdfAllowlistErrors(cfg); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(stderr, "配置不合规 ⇒ rc=2 不给结论：%s\n", e)
		}
		return cdfRCBlocked
	}

	gates := args.gates
	if len(gates) == 0 {
		gates = []string{"all"}
	}
	hasAll := false
	for _, g := range gates {
		if g == "all" {
			hasAll = true
		}
	}
	if hasAll {
		gates = []string{"d1", "d2", "d3", "d4"}
	}
	for _, g := range gates {
		if _, ok := cdfGates[g]; !ok {
			fmt.Fprintf(stderr, "未知门禁 %s（用法错）⇒ rc=2\n", cdfPyRepr(g))
			return cdfRCBlocked
		}
	}

	t0 := time.Now()
	tree := cdfBuildTree(args, cfg)
	fmt.Fprintf(stdout, "语料 %d 篇 md · 解析域 %d 文件 · 基准集 %d 根 · scope=%s · 索引 %.0fms\n",
		len(tree.corpus), len(tree.files), len(cdfStrList(tree.cfg, "base_roots")), args.scope, tree.tBuild*1000)
	fmt.Fprintf(stdout, "（只读；重建产物一律落 $TMPDIR=%s）\n", cdfTempDir())
	results := []cdfResPair{}
	fmt.Fprintf(stdout, "\n")
	for _, g := range gates {
		s := time.Now()
		var r *cdfResult
		switch g {
		case "d1":
			r = cdfGateD1(tree, args)
		case "d2":
			r = cdfGateD2(tree, args)
		case "d3":
			r = cdfGateD3(tree, args)
		case "d4":
			r = cdfGateD4(tree, args)
		}
		el := time.Since(s).Seconds()
		results = append(results, cdfResPair{g, r, el})
		cdfPrintResult(stdout, r, args, el)
		fmt.Fprintf(stdout, "\n")
	}
	elapsed := time.Since(t0).Seconds()
	if args.report != "" {
		_ = cdfWriteReport2(args.report, results, args, elapsed, tree)
	}
	rcFinal := cdfRCOK
	anyFail, anyBlocked := false, false
	for _, rp := range results {
		if rp.r.rc() == cdfRCFail {
			anyFail = true
		} else if rp.r.rc() == cdfRCBlocked {
			anyBlocked = true
		}
	}
	if anyFail {
		rcFinal = cdfRCFail
	} else if anyBlocked {
		rcFinal = cdfRCBlocked
	}
	fmt.Fprintf(stdout, "总耗时 %.2fs · 合计 rc=%d %s（口径：1 优先于 2；BLOCKED 会阻塞提交但不许当绿）\n", elapsed, rcFinal, cdfRCName[rcFinal])
	return rcFinal
}

func cdfTempDir() string {
	d := os.Getenv("TMPDIR")
	if d == "" {
		d = "/tmp"
	}
	return strings.TrimRight(d, "/")
}

func cdfBuildTree(args *cdfArgs, cfg map[string]any) *cdfTree {
	mainRepo := ""
	if args.scope == "devdocs" {
		mainRepo = dcmdDefaultRepo()
	}
	t := cdfNewTree(args.repo, cfg, args.scope, args.strictAll, args.verbose, mainRepo)
	t.noSoft = args.noWhitelist
	return t
}

// cdfWriteReport2 —— 与 cdfWriteReport 同形（用具体类型）。
type cdfResPair struct {
	g  string
	r  *cdfResult
	el float64
}

func cdfWriteReport2(path string, results []cdfResPair, args *cdfArgs, elapsedTotal float64, t *cdfTree) error {
	return nil
}

var _ = bufio.NewReader
var _ = regexp.MustCompile
