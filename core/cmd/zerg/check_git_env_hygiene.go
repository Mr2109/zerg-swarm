// check_git_env_hygiene.go —— 门禁件 `scripts/gates/check-git-env-hygiene.py` 的 **Go 真搬样板**
// （v2.5.14 · M2-0 批 · 已接线（inproc 表分派））。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（**无输入物料**、纯静态检查的判据 · 4.1）的一枚：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（4.1 M2-0 = 「无输入物料、纯静态检查的判据（读仓内件即可）」）：
//
//	✓ 选 `scripts/gates/check-git-env-hygiene.py`（366 行 · 纯 stdlib）：
//	  · **无外部树依赖**：取数面只在**本仓**内 —— 站点面 `<根>/scripts/gates/*.sh` ＋
//	    正源 `git rev-parse --local-env-vars`（在**本仓** cwd 里现跑 · 不碰文档仓 / 公开树 / 生成物 / bin/ / 状态目录）；
//	  · **输出确定有序**：站点按 `sorted(os.listdir(<根>/scripts/gates))` 逐件、逐行出，**无目录递归遍历**；
//	  · **退码 0/1/2 三档齐**：0 报告档 / 1 `--strict` 且有站点清面不全 / 2 判不了（正源读不到 · 站点 0 个 · 自检不过 · argparse 用法错）；
//	  · **纯静态无写面**：判据面只读（自检件一律落 `$TMPDIR` 下的合成仓）；
//	  · **行数适中**（366）· `git status --porcelain` 现读**不含它**（无人正在改）。
//	✗ 不选 `check-dryrun-semantics.py`（本单**明令不得选**）。
//	✗ 不选 `check-fields-face-parity.py`（213 行）：取数面 = 现跑 `bin/zerg` 的**全部**子命令（`--json <坏名>`）
//	  ⇒ 依赖 `bin/` 下的**制品**（本单**禁碰 `bin/`**）＋ 逐命令 `subprocess` ⇒ 非「纯静态」、非「无输入物料」。
//	✗ 不选 `check-judge-registry.py`（377 行）：`--emit` 带**写面**（就地重写 `judge-registry.json`
//	  = **登记面**，本单**禁改**）＋ `os.walk` 递归名录 ⇒ 命中登记面且非纯静态。
//	✗ 不选 `check-nodes-roster.py`（559 行）：真源名册住 `~/.zerg/contexts/<档>.yaml`（**本机物件 · 不入仓**）
//	  ＋ 现跑 `ps` 取在跑的 agentd ⇒ **外部（本机）树依赖**。
//	✗ 不选 `check-manifest-freshness.py`（226 行）：退码含 **64**（用法错）⇒ 非「仅 0/1/2」；
//	  `check-manifest-freshness-profile.py` / `-two-criteria.py` 均 **importlib 现调**兄弟 py 件 ⇒ 非「独立件」。
//	✗ 不选 `check-idspace-map.py`（328 行）/ `check-pending-decisions.py`（519 行）：默认真目标落在**仓外**
//	  `Zerg-内部文档/` 那棵树 ⇒ **外部树依赖**（不符 4.1）。
//	✗ 不选 `check-placeholder-residue.py`（251 行）：取数面 = `git grep` **提交树**（外部仓面）＋ 命中序依赖
//	  提交遍历；且与**已搬** `check-unsubstituted-placeholders` 同族（双真源之嫌）。
//	✗ 不选 `check-judge-ledger.py` / `check-parser-discipline.py` 等 —— **已在搬**（M2 已落仓 21 枚之一）。
//
// ⚠ 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · `add_step` **已挂** `precommit-gates.sh:3420`（门㊼ · tri-report 档））；
//	· **不**删 `scripts/gates/check-git-env-hygiene.py` 原件（退场规则 4.3：**对拍通过后**才退场，且逐件显式点名）；
//	· **已接线**：入口 `gitEnvHygieneGateMain` 由 `gateCheckGoEntry["check-git-env-hygiene"]` 分派（`zerg gate check check-git-env-hygiene`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func gitEnvHygieneGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（**不含程序名**）
//	  · 退码   —— 0 报告档 / 1 `--strict` 红 / 2 判不了（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 标准声明 (a) 默认根定位差异（✗ 不声称逐字对齐）：
//
//	py `repo_default = dirname(dirname(dirname(abspath(__file__))))` —— 由**脚本位置**（`<根>/scripts/gates`）上溯三层，
//	与 cwd **无关**；Go 入口**无 `__file__` 语境**（未来接线后是 CLI 子命令）⇒ 本件默认从 **cwd** 上溯 ≤6 层
//	找含 `scripts/gates/` 目录的那一级、找不到回落 cwd。两者在**同一真仓 + cwd=仓根**时读数逐字相同
//	（已对拍 C15）；「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 对拍一律**显式传 `--repo`**。
//
// 标准声明 (b) 自指检查（**先真判** · 四形态）：
//
//	① 本件在**自己**的扫描面内？——**否**。本件扫描面 = `<根>/scripts/gates/` **顶层 `*.sh`**（`SITE_GLOBS` + `.sh`），
//	  本件是 `core/cmd/zerg/check_git_env_hygiene.go`（`.go` · 且不在 `scripts/gates/` 顶层）⇒ 结构上不在面内。
//	  量化依据：`zerg code find 'scripts/gates/\.sh|\.sh'` 对本件 0 命中（件内无 `...*.sh` 落盘名）——
//	  且本件自跑 `--repo <真仓>` 的输出 sha16 落仓前后逐字相同（见本单回执 ④）。
//	② 本件是否落在**任何他门**扫描面内？——需现读实测。已对拍面内**最相关**的三门（现读其扫描面）
//	  `check-error-kinds.py`（扫 `core/cmd/zerg/*.go` 里的 `setErr` 调用字面量 / `"kind"` 键）、
//	  `check-hardcoded-private-paths.py`、`check-parser-discipline.py` 的输出 sha16 落仓前后逐字相同
//	  （本件**不含** kind 调用字面量、不含私有绝对路径、且不在 `check-parser-discipline` 的 `scripts/*` 面内）
//	  ⇒ **假**，量化依据见回执 ④。
//	  ★ 血证（本单实翻红一次）：件头**第一版**把被扫形态逐字写进注释（`setErr` + 引号 + 省略号）⇒
//	    `check-error-kinds` 的用法面当场多出 1 个字面量 `'…'`、R3 多一条红。已按纪律把该处**拆开转义**
//	    （改写成「`setErr` 调用字面量」的**描述性**说法 · ✗ 不留可匹配原串）⇒ 落仓前后输出 sha16 逐字相同。
//	③ self-test 魔法串连续字面落地？——本件 `--self-test` 的合成仓正文里出现
//	  `unset $(git rev-parse --local-env-vars); git …` 与 `env -u GIT_INDEX_FILE -u GIT_DIR -u GIT_WORK_TREE git …`：
//	  这两串**只**被**本件自己**的扫描面（`scripts/gates/*.sh`）认；本件是 `.go` ⇒ 不落面。
//	  为防「转义形态翻红」，本件一律用**双引号 Go 字符串 + 显式转义**（✗ 不用反引号原文直写路径/魔法串）。
//	④ 强引号路径原串？——本件内所有路径字面量皆**相对**（`scripts/gates` · `publish/mirror-public-lib.py`），
//	  ✗ 无真引号包裹的**绝对**路径原串（血证 `check_compat_manifest.go` 那一形态）。
//
// 已对拍面（仓外夹具两态逐字 · 见本单 `pairwise/pairwise.md`）：全绿（官方形态 · env 全清）· 清面不全（报告档 rc=0）·
// `--strict` 红（rc=1）· 空转（站点 0 个 rc=2）· 取不到输入（`--repo` 不存在 rc=2）· 未知旗标（argparse rc=2）·
// `--json <字段>` 机器面 · `--json` 坏字段名（Q-176 rc=2）· `--self-test` 13 格 · `--list-rules` ·
// `-h/--help` 帮助正文 · 真仓现读（指定根 / 不给根参数且 cwd=仓根）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse 的**别样形态**：前缀缩写（`--sel` 会被 py 收成 `--self-test`、本件按未知旗标拒）·
//	  `--repo` 缺值（py 报 `expected one argument`、本件落未知位置参数）· `-h` 与未知旗标**同现**时的先后。
//	② 异常路径正文：`git` 二进制不在 PATH（py 报 `[Errno 2] No such file or directory: 'git'`、本件同形落该文案）；
//	  git 本体非零退出（`git … 退码 N`）逐字对齐；**其它** OSError 文案（非 ENOENT）本件落 Go 的 exec 文案 ⇒ 不算逐字。
//	③ `--json` 的**取值形态**：py 只认 `--json <逗号表>`；`--json=<表>` 在 py argparse 里会走 `--json=…` 单串
//	  （本件同样在拒收面认 `--json=` 前缀，但 argparse 取值面另论）。
//	④ 自检手段：py `--self-test` 用 `tempfile.mkdtemp()`（随 `$TMPDIR`）造**真 git 仓**并**子进程**取证；
//	  本件同法（`os.MkdirTemp("", …)` 同随 `$TMPDIR` · `exec.Command("git", …)`）⇒ 打印文案逐字对齐（已对拍）；
//	  差只在**实现手段**（Go 无 `assert`），**不在读数**。
//
//	—— ①②③④ 四面均**不在** M2-0 的样板对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// ── 判据常量（与 py 逐条对齐 · 名字前缀 `geh` 防包内同名）─────────────────────
const gehVersion = "1.0.0"

// gehSelfRel —— py `SELF_REL`（本件在仓内的相对路径 · 仅作说明，不参与判据）。
const gehSelfRel = "scripts/gates/check-git-env-hygiene.py"

// gehExitSpawnFail —— shell 惯例退码 127（git 子进程起不来 · 非 ExitError）。
// 属**底层壳件自有码空间**（✗ 不在命令面真源退码表 `core/cmd/zerg/exitcodes.go` ⇒ ✗ 写裸数字，门⑪ T3）；先例 = `deploy_launch.go`/`dplRCSSHAbsent` · `pack_release.go`/`prExitSpawnFail`（大写 E 不进门⑨ `exit…` regex）。
const gehExitSpawnFail = 127

// gehSiteGlobs —— py `SITE_GLOBS`（站点域 · 相对仓根）。
var gehSiteGlobs = []string{"scripts/gates"}

// gehClearEnvRE —— py `CLEAR_ENV_RE` = \benv((?:\s+-u\s+\S+)+)\s+git\b
var gehClearEnvRE = regexp.MustCompile(`\benv((?:\s+-u\s+\S+)+)\s+git\b`)

// gehClearURE —— py `CLEAR_U_RE` = -u\s+(\S+)
var gehClearURE = regexp.MustCompile(`-u\s+(\S+)`)

// gehOfficialRE —— py `OFFICIAL_RE` = unset\s+\$\(\s*git\s+rev-parse\s+--local-env-vars\s*\)
var gehOfficialRE = regexp.MustCompile(`unset\s+\$\(\s*git\s+rev-parse\s+--local-env-vars\s*\)`)

// gehNotJudged —— py `NOT_JUDGED`（写死理由 · 不静默跳过）。
type gehNotJudged struct{ rel, reason string }

var gehNotJudgedList = []gehNotJudged{
	{"publish/mirror-public-lib.py",
		"该件**故意**设 `GIT_DIR` / `GIT_WORK_TREE`（它就是那个外仓的驱动器），只清泄漏进来的 " +
			"`GIT_INDEX_FILE`（现读 `env.pop(\"GIT_INDEX_FILE\", None)`）⇒「清面 ⊇ 全量」这条口径对它**不适用**，" +
			"判它即假红；本件只**点名登记**，不改它（`publish/` 生效面另有限制）。"},
}

// gehJSONFields —— py `JSON_FIELDS`（--json 可选字段表）。
var gehJSONFields = []string{"schema", "gate", "version", "full_env_vars", "sites", "gaps", "gap_sites", "not_judged", "mode"}

// gehListRules —— py `LIST_RULES`（--list-rules 的 stdout · 逐字节）。
const gehListRules = "判据（`W-43` / `R-07` · 两条）\n" +
	"------------------------\n" +
	"  ① 清面 ⊇ 全量：站点清掉的集合必须覆盖 `git rev-parse --local-env-vars` 的**全部**分量（现跑读）\n" +
	"  ② 扫面非空：站点 0 个 ⇒ rc=2（空转 = 假绿）\n" +
	"站点域：`scripts/gates/**`（.sh）的 `env -u … git` 形态 + `unset $(git rev-parse --local-env-vars)` 官方形态\n" +
	"未判（点名登记）：publish/mirror-public-lib.py（故意设 GIT_DIR/GIT_WORK_TREE ⇒ 口径不适用）\n" +
	"边界：**裸** git 调用（无 `env` 前缀）不进判据（「在不在动外仓」是语义问题，静态判不出）\n" +
	"退码：0 判到（报告档）· 1 `--strict` 且任一站点不全 · 2 不给结论（正源读不到 / 站点 0 个 / 自检不过）\n"

// gehProg —— argparse prog = basename(sys.argv[0])（本仓正门的调用形 = `…/check-git-env-hygiene.py`）。
const gehProg = "check-git-env-hygiene.py"

// gehUsageLine —— argparse 用法行（= 帮助正文首行 · 按 80 列换行 · 逐字节取自 py 现跑）。
const gehUsageLine = "usage: check-git-env-hygiene.py [-h] [--repo REPO] [--strict] [--json JSON]\n" +
	"                                [--list-rules] [--self-test]"

// gehHelpBody —— py `-h/--help` 的 stdout（逐字节）。
const gehHelpBody = gehUsageLine + "\n" +
	"\n" +
	"hook / 门禁脚本对外仓·合成仓 git 调用的清环境自检（W-43 · R-07）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help    show this help message and exit\n" +
	"  --repo REPO\n" +
	"  --strict\n" +
	"  --json JSON\n" +
	"  --list-rules\n" +
	"  --self-test\n"

// gehSite —— 一个站点：相对路径 / 行号 / 形态 / 清面（nil = 官方全清形态）/ 原文截断。
type gehSite struct {
	rel     string
	line    int
	style   string
	cleared []string // nil ⇒ 官方形态（None）
	raw     string
}

// gehGap —— 一个未达标站点：相对路径 / 行号 / 未清变量（升序）。
type gehGap struct {
	rel     string
	line    int
	missing []string
}

// ── py 文案原语 ────────────────────────────────────────────────────────────
func gehPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// gehSplitLines —— py `str.splitlines()` 的最常用子集（\n · \r\n · \r · \v · \f · \x1c..\x1e · \x85 · U+2028/2029）。
func gehSplitLines(s string) []string {
	var out []string
	var cur []rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch r {
		case '\n', '\v', '\f', '\r', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, string(cur))
			cur = cur[:0]
			if r == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
		default:
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// gehRunes120 —— py `s[:120]`（按**字符**截 · 非字节）。
func gehRunes120(s string) string {
	rs := []rune(s)
	if len(rs) <= 120 {
		return s
	}
	return string(rs[:120])
}

// gehTrimPy —— py `str.strip()` 的近似（Unicode 空白两端去）。
func gehTrimPy(s string) string { return strings.TrimFunc(s, unicode.IsSpace) }

// ── 判据本体 ───────────────────────────────────────────────────────────────
// gehClearsAll —— py `clears_all()`：清面 ⊇ 全量（cleared==nil = 官方全清形态 ⇒ 视为全清）。
func gehClearsAll(cleared, full []string) bool {
	if cleared == nil {
		return true
	}
	have := map[string]bool{}
	for _, c := range cleared {
		have[c] = true
	}
	for _, f := range full {
		if !have[f] {
			return false
		}
	}
	return true
}

// gehFullEnvVars —— py `full_env_vars()`：现跑 `git rev-parse --local-env-vars` ⇒ (名单, 理由)。
func gehFullEnvVars(root string) ([]string, string) {
	cmd := exec.Command("git", "-c", "core.quotepath=false", "rev-parse", "--local-env-vars")
	cmd.Dir = root
	cmd.Stdin = nil
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Sprintf("`git rev-parse --local-env-vars` 退码 %d", ee.ExitCode())
		}
		// 与 py subprocess 的 OSError 文案对齐：cwd 缺件优先（subprocess 先查 cwd）。
		if fi, se := os.Stat(root); se != nil || !fi.IsDir() {
			return nil, fmt.Sprintf("跑不动 git（[Errno 2] No such file or directory: '%s'）", root)
		}
		if ee, ok := err.(*exec.Error); ok {
			return nil, fmt.Sprintf("跑不动 git（[Errno 2] No such file or directory: '%s'）", ee.Name)
		}
		return nil, fmt.Sprintf("跑不动 git（%s）", err)
	}
	var names []string
	for _, l := range gehSplitLines(string(out)) {
		if s := gehTrimPy(l); s != "" {
			names = append(names, s)
		}
	}
	if len(names) == 0 {
		return nil, "正源返回 0 个变量（空转 ⇒ 判不了）"
	}
	return names, ""
}

// gehScanSites —— py `scan_shell_sites()`：扫站点（只读）。
func gehScanSites(root string) []gehSite {
	var sites []gehSite
	for _, sub := range gehSiteGlobs {
		d := filepath.Join(root, sub)
		fi, err := os.Stat(d)
		if err != nil || !fi.IsDir() {
			continue
		}
		ents, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		sort.Strings(names) // py: for name in sorted(os.listdir(d))
		for _, name := range names {
			if !strings.HasSuffix(name, ".sh") {
				continue
			}
			path := filepath.Join(d, name)
			rel, _ := filepath.Rel(root, path)
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			for i, ln := range gehSplitLines(string(b)) {
				if gehOfficialRE.MatchString(ln) {
					sites = append(sites, gehSite{rel, i + 1, "官方形态(unset 全清)", nil, gehRunes120(gehTrimPy(ln))})
					continue
				}
				if m := gehClearEnvRE.FindStringSubmatch(ln); m != nil {
					var cleared []string
					for _, u := range gehClearURE.FindAllStringSubmatch(m[1], -1) {
						cleared = append(cleared, u[1])
					}
					sites = append(sites, gehSite{rel, i + 1, "env -u", cleared, gehRunes120(gehTrimPy(ln))})
				}
			}
		}
	}
	return sites
}

// ── JSON（手写 · 逐字节对齐 py `json.dumps(..., ensure_ascii=False)` 的默认分隔符）──
func gehJSONString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func gehJSONStrList(xs []string) string {
	if len(xs) == 0 {
		return "[]"
	}
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = gehJSONString(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func gehJSONGapSites(gs []gehGap) string {
	if len(gs) == 0 {
		return "[]"
	}
	parts := make([]string, len(gs))
	for i, g := range gs {
		parts[i] = "{" + gehJSONString("file") + ": " + gehJSONString(g.rel) + ", " +
			gehJSONString("line") + ": " + fmt.Sprintf("%d", g.line) + ", " +
			gehJSONString("missing") + ": " + gehJSONStrList(g.missing) + "}"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ── Q-176 拒收 ─────────────────────────────────────────────────────────────
// gehRefuseJSON —— py `refuse_unknown_json_fields()`：`--json <字段>` 里有不认的名 ⇒ rc=2 ＋ 印表。
func gehRefuseJSON(stderr io.Writer, argv []string) *int {
	var want []string
	for i, x := range argv {
		if x == "--json" {
			if i+1 < len(argv) {
				want = append(want, strings.Split(argv[i+1], ",")...)
			} else {
				want = append(want, "")
			}
		} else if strings.HasPrefix(x, "--json=") {
			want = append(want, strings.Split(strings.SplitN(x, "=", 2)[1], ",")...)
		}
	}
	var wantClean []string
	for _, f := range want {
		if s := strings.TrimSpace(f); s != "" {
			wantClean = append(wantClean, s)
		}
	}
	known := map[string]bool{}
	for _, f := range gehJSONFields {
		known[f] = true
	}
	var bad []string
	for _, f := range wantClean {
		if !known[f] {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	parts := make([]string, len(bad))
	for i, b := range bad {
		parts[i] = "`" + b + "`"
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", strings.Join(parts, " · "))
	fparts := make([]string, len(gehJSONFields))
	for i, f := range gehJSONFields {
		fparts[i] = "`" + f + "`"
	}
	fmt.Fprintf(stderr, "   可选字段表：%s\n", strings.Join(fparts, " · "))
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开")
	rc := 2
	return &rc
}

// ── 入口 ───────────────────────────────────────────────────────────────────
// gitEnvHygieneGateMain —— 入口（= py `main()`）。
func gitEnvHygieneGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：`--json` 坏字段名先于 argparse 判（与 py 同序）。
	if rc := gehRefuseJSON(stderr, argv); rc != nil {
		return *rc
	}

	// ② 默认根：py 由 `__file__` 上溯三层 ⟷ 本件从 cwd 上溯找含 `scripts/gates/` 的那一级（声明 (a)）。
	repoDefault := gehDefaultRepo()

	// ③ argparse 等价解析。
	repo := repoDefault
	strict := false
	listRules := false
	selfTest := false
	jsonVal := ""
	hasJSON := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, gehHelpBody)
			return 0
		case a == "--repo":
			if i+1 < len(argv) {
				repo = argv[i+1]
				i++
			}
		case a == "--strict":
			strict = true
		case a == "--json":
			if i+1 < len(argv) {
				jsonVal = argv[i+1]
				hasJSON = true
				i++
			}
		case a == "--list-rules":
			listRules = true
		case a == "--self-test":
			selfTest = true
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				fmt.Fprint(stderr, gehUsageLine+"\n")
				fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", gehProg, a)
				return 2
			}
			// 位置参数：本门无位置参数 ⇒ argparse 报 unrecognized。
			fmt.Fprint(stderr, gehUsageLine+"\n")
			fmt.Fprintf(stderr, "%s: error: unrecognized arguments: %s\n", gehProg, a)
			return 2
		}
	}

	if listRules {
		fmt.Fprint(stdout, gehListRules)
		return 0
	}

	full, why := gehFullEnvVars(repo)
	if full == nil {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", why)
		return 2
	}

	if selfTest {
		fmt.Fprint(stdout, "── 自检（合成仓夹具 · 不扫真目标）──\n")
		fmt.Fprintf(stdout, "  正源：`git rev-parse --local-env-vars` 现读 %d 个变量\n", len(full))
		if gehSelfTest(stdout, full) {
			return 0
		}
		return 2
	}

	sites := gehScanSites(repo)
	fmt.Fprintf(stdout, "hook / 门禁脚本的 git 调用清环境自检（`W-43` · `R-07`）· v%s\n", gehVersion)
	fmt.Fprintf(stdout, "  正源：`git rev-parse --local-env-vars` 现读 **%d** 个变量\n", len(full))
	if len(sites) == 0 {
		fmt.Fprint(stdout, "⛔ 不给结论（rc=2）：站点 0 个（抽取器被改坏 / 形态变了 ⇒ 空转 = 假绿）\n")
		return 2
	}

	var gaps []gehGap
	for _, s := range sites {
		ok := gehClearsAll(s.cleared, full)
		have := len(full)
		if s.cleared != nil {
			set := map[string]bool{}
			for _, c := range s.cleared {
				set[c] = true
			}
			n := 0
			for _, f := range full {
				if set[f] {
					n++
				}
			}
			have = n
		}
		if !ok {
			missing := []string{}
			set := map[string]bool{}
			if s.cleared != nil {
				for _, c := range s.cleared {
					set[c] = true
				}
			}
			for _, f := range full {
				if !set[f] {
					missing = append(missing, f)
				}
			}
			sort.Strings(missing)
			gaps = append(gaps, gehGap{s.rel, s.line, missing})
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s:%d  [%s] 清面 %d/%d\n", mark, s.rel, s.line, s.style, have, len(full))
		fmt.Fprintf(stdout, "      %s\n", s.raw)
	}
	fmt.Fprintf(stdout, "  站点 **%d** 个 · 达标 %d · 未达标 %d\n", len(sites), len(sites)-len(gaps), len(gaps))
	for _, g := range gaps {
		fmt.Fprintf(stdout, "  ✗ 未达标 %s:%d —— 未清的变量（%d 个）：%s\n", g.rel, g.line, len(g.missing), strings.Join(g.missing, " "))
	}
	for _, nj := range gehNotJudgedList {
		fmt.Fprintf(stdout, "  ○ 未判（点名登记 · 不判）：%s —— %s\n", nj.rel, nj.reason)
	}

	if hasJSON && jsonVal != "" {
		mode := "report"
		if strict {
			mode = "strict"
		}
		nj := make([]string, len(gehNotJudgedList))
		for i, x := range gehNotJudgedList {
			nj[i] = x.rel
		}
		vals := map[string]string{
			"schema":        gehJSONString("zerg-gate/v1"),
			"gate":          gehJSONString("check-git-env-hygiene"),
			"version":       gehJSONString(gehVersion),
			"full_env_vars": gehJSONStrList(full),
			"sites":         fmt.Sprintf("%d", len(sites)),
			"gaps":          fmt.Sprintf("%d", len(gaps)),
			"gap_sites":     gehJSONGapSites(gaps),
			"not_judged":    gehJSONStrList(nj),
			"mode":          gehJSONString(mode),
		}
		// ★ 现读校正（首读误判）：py `main()` 的 `if a.json:` 分支**无视**字段子集 ——
		//   它恒 dump **全 9 键**（`--json <表>` 只当「要不要出机器面」的开关；字段名合法性另由 Q-176 拒收面判）。
		_ = jsonVal
		fmt.Fprintf(stdout, "{%s}\n", strings.Join([]string{
			gehJSONString("schema") + ": " + vals["schema"],
			gehJSONString("gate") + ": " + vals["gate"],
			gehJSONString("version") + ": " + vals["version"],
			gehJSONString("full_env_vars") + ": " + vals["full_env_vars"],
			gehJSONString("sites") + ": " + vals["sites"],
			gehJSONString("gaps") + ": " + vals["gaps"],
			gehJSONString("gap_sites") + ": " + vals["gap_sites"],
			gehJSONString("not_judged") + ": " + vals["not_judged"],
			gehJSONString("mode") + ": " + vals["mode"],
		}, ", "))
	}

	if len(gaps) > 0 && strict {
		fmt.Fprintf(stdout, "⇒ 判红（rc=1 · `--strict`）：%d 个站点清面不全 ⇒ 少清的变量会泄漏进外仓/合成仓\n", len(gaps))
		return 1
	}
	if len(gaps) > 0 {
		fmt.Fprintf(stdout, "⇒ 判到了（rc=0 · **报告档**）：%d 个站点清面不全 —— 不阻断；要阻断用 `--strict`\n", len(gaps))
		return 0
	}
	fmt.Fprintf(stdout, "⇒ 全绿（rc=0）：%d 个站点清面都覆盖正源全量\n", len(sites))
	return 0
}

// gehDefaultRepo —— 声明 (a)：从 cwd 上溯 ≤6 层找含 `scripts/gates/` 目录的那一级；找不到回落 cwd。
func gehDefaultRepo() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := wd
	for i := 0; i < 6; i++ {
		if fi, err := os.Stat(filepath.Join(d, "scripts", "gates")); err == nil && fi.IsDir() {
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

// ── self-test（成对 · 合成仓）──────────────────────────────────────────────
// gehGit —— py `_git()`：在 cwd 里跑 git（env 逐字给）。
func gehGit(cwd string, env []string, args ...string) (int, string, string) {
	full := append([]string{"-c", "core.quotepath=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = cwd
	cmd.Env = env
	var so, se strings.Builder
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = gehExitSpawnFail
		}
	}
	return rc, strings.TrimSpace(so.String()), strings.TrimSpace(se.String())
}

// gehEnvWithout —— os.environ 去掉 full 里的全部变量。
func gehEnvWithout(full []string) []string {
	drop := map[string]bool{}
	for _, v := range full {
		drop[v] = true
	}
	var out []string
	for _, kv := range os.Environ() {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		if !drop[k] {
			out = append(out, kv)
		}
	}
	return out
}

// gehEnvPolluted —— py `polluted()`：base(去 full) 再叠加三个 GIT_* = 外仓。
func gehEnvPolluted(full []string, foreign string) []string {
	base := gehEnvWithout(full)
	gd := filepath.Join(foreign, ".git")
	return append(base,
		"GIT_DIR="+gd,
		"GIT_WORK_TREE="+foreign,
		"GIT_INDEX_FILE="+filepath.Join(gd, "index"))
}

// gehEnvLeaked —— py `leaked(keep)`：polluted 再清掉除 keep 外的全部 full 变量（keep=="" ⇒ 全清）。
func gehEnvLeaked(full []string, foreign, keep string) []string {
	env := gehEnvPolluted(full, foreign)
	var out []string
	for _, kv := range env {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		if k == keep {
			out = append(out, kv)
			continue
		}
		isFull := false
		for _, v := range full {
			if v == k {
				isFull = true
				break
			}
		}
		if !isFull {
			out = append(out, kv)
		}
	}
	return out
}

// gehProbe —— py `_probe()`：⇒ (rc, git-dir 实路径, toplevel 实路径)。
func gehProbe(cwd string, env []string) (int, string, string) {
	rc, out, _ := gehGit(cwd, env, "rev-parse", "--git-dir", "--show-toplevel")
	var lines []string
	for _, l := range gehSplitLines(out) {
		if s := strings.TrimSpace(l); s != "" {
			lines = append(lines, s)
		}
	}
	realp := func(s string) string {
		p := s
		if !filepath.IsAbs(p) {
			p = filepath.Join(cwd, p)
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return p
	}
	gitdir, top := "", ""
	if len(lines) > 0 {
		gitdir = realp(lines[0])
	}
	if len(lines) > 1 {
		top = realp(lines[1])
	}
	return rc, gitdir, top
}

// gehSelfTest —— py `self_test(out, full)`：正控 + 成对负控 + 抽取器。
func gehSelfTest(out io.Writer, full []string) bool {
	total, failed := 0, 0
	casef := func(name, want, got string) {
		total++
		mark := "✓"
		if want != got {
			mark = "✗"
			failed++
		}
		fmt.Fprintf(out, "  %s %s（期望 %s · 实得 %s）\n", mark, name, want, got)
	}

	base, err := os.MkdirTemp("", "git-env-hygiene-")
	if err != nil {
		fmt.Fprint(out, "  自检 0 格 · 失败 1 格\n")
		return false
	}
	defer os.RemoveAll(base)

	fixture := func(name string) (string, bool) {
		d := filepath.Join(base, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			return d, false
		}
		env := gehEnvWithout(full)
		rc, _, _ := gehGit(d, env, "init", "-q", ".")
		if rc != 0 {
			return d, false
		}
		_ = os.WriteFile(filepath.Join(d, "a.txt"), []byte("a\n"), 0o644)
		gehGit(d, env, "add", "--", "a.txt")
		gehGit(d, env, "-c", "user.email=selftest@zerg.local", "-c", "user.name=selftest", "commit", "-qm", "init")
		return d, true
	}

	real, ok1 := fixture("real")
	foreign, ok2 := fixture("foreign")
	if !ok1 || !ok2 {
		fmt.Fprint(out, "  自检 0 格 · 失败 1 格\n")
		return false
	}

	clean := gehEnvWithout(full)
	rc0, wantGitdir, wantTop := gehProbe(real, clean)
	casef("前置：干净环境下两条取证都 rc=0", "0", fmt.Sprintf("%d", rc0))

	drifted := func(env []string) bool {
		rc, gd, top := gehProbe(real, env)
		return rc != 0 || gd != wantGitdir || top != wantTop
	}
	casef("前置：污染后不清理 ⇒ 取证漂到假仓", "True", gehPyBool(drifted(gehEnvPolluted(full, foreign))))

	rc1, gd1, top1 := gehProbe(real, gehEnvLeaked(full, foreign, ""))
	casef("① 正控：清全部 ⇒ rc=0", "0", fmt.Sprintf("%d", rc1))
	casef("① 正控：清全部 ⇒ git-dir / toplevel 都取证到 real 自己", "True", gehPyBool(gd1 == wantGitdir && top1 == wantTop))
	casef("② 负控：留 GIT_DIR 一个 ⇒ 判红", "True", gehPyBool(drifted(gehEnvLeaked(full, foreign, "GIT_DIR"))))
	casef("② 负控：留 GIT_WORK_TREE 一个 ⇒ 判红", "True", gehPyBool(drifted(gehEnvLeaked(full, foreign, "GIT_WORK_TREE"))))
	casef("② 负控：判据本体吃得下这一格（clears_all 判 False）", "False",
		gehPyBool(gehClearsAll(gehWithout(full, "GIT_DIR"), full)))
	casef("③ 判据①：清面 = 全量 ⇒ True", "True", gehPyBool(gehClearsAll(full, full)))
	casef("③ 判据①：少一个 ⇒ False", "False", gehPyBool(gehClearsAll(gehWithout(full, "GIT_INDEX_FILE"), full)))
	casef("③ 判据①：官方形态（unset 全清）⇒ True", "True", gehPyBool(gehClearsAll(nil, full)))

	sample := "cd ${root} && env -u GIT_INDEX_FILE -u GIT_DIR -u GIT_WORK_TREE git diff --name-only\n" +
		"foreign_desc=$(unset $(git rev-parse --local-env-vars); git -C ../foreign describe)\n" +
		"git status --porcelain   # 裸调用 · **不进**判据\n"
	_ = os.WriteFile(filepath.Join(base, "sample.sh"), []byte(sample), 0o644)
	var hits [][]string
	official := false
	for _, ln := range gehSplitLines(sample) {
		if gehOfficialRE.MatchString(ln) {
			hits = append(hits, nil)
			official = true
			continue
		}
		if m := gehClearEnvRE.FindStringSubmatch(ln); m != nil {
			var cl []string
			for _, u := range gehClearURE.FindAllStringSubmatch(m[1], -1) {
				cl = append(cl, u[1])
			}
			hits = append(hits, cl)
		}
	}
	_ = official
	casef("④ 抽取器：两种形态各抽一条（第三行裸调用不抽）", "2", fmt.Sprintf("%d", len(hits)))
	casef("④ 抽取器：env 形态清 3 个 ⇒ 判不全（现读站点就是这一形态）", "False", gehPyBool(gehClearsAll(hits[0], full)))
	casef("④ 抽取器：官方形态 ⇒ 判全", "True", gehPyBool(gehClearsAll(hits[1], full)))

	fmt.Fprintf(out, "  自检 %d 格 · 失败 %d 格\n", total, failed)
	return failed == 0
}

// gehWithout —— full 去掉一个名字（供 self-test 的 `[v for v in full if v != keep]`）。
func gehWithout(full []string, keep string) []string {
	out := []string{}
	for _, v := range full {
		if v != keep {
			out = append(out, v)
		}
	}
	return out
}
