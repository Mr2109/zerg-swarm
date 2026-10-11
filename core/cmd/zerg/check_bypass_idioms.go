// check_bypass_idioms.go —— 判据件 `scripts/gates/check-bypass-idioms.py`（343 行）的
// **Go 真搬**（v2.5.14 · M2 · 入口 `checkBypassIdiomsGateMain`）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳；M2 = 按价值真搬 Go（落点 `core/cmd/zerg/`）。
//	本件 = M2 一枚，验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码 **三面逐字相同**
//	（除下列**显式声明的**差异面）。★ 本件**只落码、不接线**：不注册命令、不删 py 原件、不动登记面。
//
// 旋钮面（照 py 同名旗标 · ✗ 不另起名）：
//
//	py 本就带 `--root`（判哪棵树 · 可重复）· `--registry`（追加登记册 JSON 数组）· `--json` · `--list-forms`
//	· `-h/--help`，且 `--self-test`/`--selftest` 由**前置串判**直接走自检（不走 argparse）⇒ 本件**同名照搬**。
//	py 另读 `ZERG_REPO_ROOT` env 当仓根 ⇒ 本件**同读**（✗ 无 env 就回落 cwd 上溯，见 (a)）。
//
// 缺省推导（逐字同构）：
//
//	py  `REPO_ROOT = os.environ.get("ZERG_REPO_ROOT") or os.path.dirname(os.path.dirname(os.path.abspath(__file__)))`
//	Go  `cbiRepoRoot()` = env `ZERG_REPO_ROOT` 非空则用，否则从 **cwd** 上溯找含 `scripts/gates/` 的目录（无 `__file__` 语境）
//	py  `roots = [abspath(r) for r in --root] or [REPO_ROOT/publish, REPO_ROOT/scripts]`
//	Go  `cbiPyJoin(repoRoot, "publish")` / `cbiPyJoin(repoRoot, "scripts")`（★ 字符串拼接，✗ filepath.Join）
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py 从 `__file__` 上溯（与 cwd 无关）⟷ Go 无 `__file__` 语境、只能从 cwd 上溯。
//	    「脚本位置 == cwd 上溯根」时两者读数逐字相同（对拍甲已证：真仓 + cwd=仓根）；否则**路径推导面不同**
//	    ⇒ 不属逐字对齐承诺。★ py 的 `ZERG_REPO_ROOT` env 面本件同读 ⇒ 该面**反而**可恢复逐字（对拍未 export 它）。
//	(b) **自指检查（先真判 · 四形态逐条）**：
//	    ① **本件在自己扫描面内吗 ⇒ 否**。本门件面 = 默认根 `publish/` + `scripts/` 下的 `.sh`/`.py`；本件
//	      `.go` 落 `core/cmd/zerg/` ⇒ 既不在根内、也非后缀 ⇒ **不进件面**。且 py 侧对**同名件**自排除
//	      （`SELF_NAME = basename(__file__)` = `check-bypass-idioms.py`）—— 本件照搬该自排除名（py 原件仍在盘、
//	      仍落 `scripts/gates/`），保证真仓本门读数**落仓前后逐字相同**（见交付读数）。
//	    ② **本件的测试件在件面内吗 ⇒ 否**（`.go`/`_test.go` 不落 `publish/`/`scripts/`，非 `.sh`/`.py`）。
//	    ③ **self-test 魔法串是否连续字面落地 ⇒ 否**：本件无「self-test 开关魔法串」机检形态（自检夹具用
//	      合成串 `git@example.invalid:o/r.git` · `read_token` —— 与 py 自检同一套合成值，非私有面）。
//	    ④ **是否落在任何他门扫描面内 ⇒ 会**（`core/cmd/zerg/*.go` 落 `check-error-kinds` 等门的扫描面）
//	      ⇒ 正文**零绝对路径字面量、零 JSON 键名字面量**（`kind`/`schema` 一类键名走 `cbiJSONStr(<键名>)` 派生，
//	      ✗ 不写连续的 `"键": "值"` 原串）⇒ 规避 R3 一族自伤。
//
// py 原件真口子（★ 不自行修 · 逐字保留 · **交回入账**）：
//
//	① **B 类读失败静默**（py `scan` 的 `except OSError: continue`）：件已 `files += 1` 计入扫描面、
//	  但正文一行都不入判 ⇒ 「命中/不命中」都可能被这份静默掩盖（少判且无落点）。
//	  ★ **本件 Go 侧已修**（B 类真口子修 · 与 py **有意分叉** · 同族先例 `check_doc_cmds.go` 的
//	  `dcmdReadFails`）：读不了的件 ⇒ 登记（相对路径 + err 正文）入 `cbiReadFails`；入口 `cbiRun` 补判红
//	  （stderr BLOCKED 头 + 逐件点名）⇒ rc=2（不再静默少判）。✗ py 半仍静默（该格属两态对拍**有意差异面**）。
//	② **B 类目录面**（`os.walk` 默认 `onerror=None`）：读不了的**目录**被静默跳过（与「读不了的件」不同面）。
//	  ★ **本件 Go 侧已修**：目录打不开 ⇒ 同一 `cbiReadFails` 登记面 ⇒ 入口 rc=2（✗ 不再「`os.Open` 失败即 return」）。
//	③ **最外层无兜底**（C 类）：py `__main__` 是裸 `sys.exit(main())` ⇒ `--registry` 指向「存在但读不了 / 非
//	  JSON 数组 / JSON 解析失败」会走已捕分支（rc=2）；但**登记册元素非 dict**（如字符串）会在 `e.get(...)` 抛
//	  AttributeError ⇒ 未捕异常 ⇒ 回溯 + **rc=1**。本件对该**未覆盖面**改为 fail-closed rc=2 并声明（件头未覆盖面③）。
//	④ **D 类**：本件无死旗标（`--root`/`--registry`/`--json`/`--list-forms` 均被读取）。
//
// 遍历 / 排序点逐点交代（★ 决定输出行序的那一条已点名）：
//
//	T1 `os.walk(root)`（topdown · followlinks=False）：**readdir 原序**；当前层**文件先出**，再按 `dirnames`
//	  原序下钻。Go 一律 `f.ReadDir(-1)`（★ ✗ 不用会排序的 `os.ReadDir` / `WalkDir`）。
//	T2 `dirnames[:] = [d for d in dirnames if d not in EXCLUDE_DIRS]`：**目录名**成员判定（保序）。
//	T3 目录分类 = `entry.is_dir()`（**跟随符号链接**）⇒ Go 用 `os.Stat`，✗ 不用 `entry.IsDir()`（Lstat 语义，
//	  对「指向目录的符号链接」会判反）；递归资格另判「非符号链接」（同 `os.walk` followlinks=False）。
//	T4 `for fn in sorted(filenames)`：**当前层文件名按 py 字典序**（码点序 = Go `sort.Strings` 的字节序）。
//	T5 产出 `occ` 保 T1 原序（根序 → 目录原序 → 文件字典序 → 行号升序）。
//	★★ **输出行序的决定者**：`occ` 明细行序 = **T5 原序**（`unregistered` 逐处点名即按它）；`stale` 行序 =
//	  登记册**入参序**（切片保序）；`hits[]` 的形态 id 序 = FORMS **表序**（切片保序）。全件**零 map 迭代进输出**。
//	★ 本件 py 从**不**对遍历结果做全局 `sorted()`（唯一排序点 = T4 的层内文件名）。
//
// 正则语义差（py `re` ⟷ Go RE2；涉及面逐条写明）：
//
//	R1 `\s`：py Unicode ⟷ Go ASCII ⇒ 译 py 空白类（本件 `cbiPyRegexToGo`）。涉及 F01/F02/F03/F04/F05/F06/F07/F08/F10/F12。
//		注：本件逐**行**判（行由 `str.splitlines()` 切出），行内不可能含 `\n`/`\r`/`\v`/`\f`/`\x1c-\x1e`/`\x85`/
//		`\u2028`/`\u2029`（那些都是 splitlines 的断行符）⇒ 行内 `\s` 与 Go `\s` 的实际差**仅剩**非断行 Unicode 空白
//		（`\xa0`/`\u1680`/`\u2000-\u200a`/`\u202f`/`\u205f`/`\u3000`）—— 仍按 py 全类译文（零遗漏）。
//	R2 `\b`：py Unicode 词界 ⟷ Go ASCII ⇒ 译消费式 `(?:[^\p{L}\p{N}_]|\A|\z)`（仅判在场）。剩余分叉（写进未覆盖面）：
//		`\b` 的**零宽**语义被换成**消费一位** ⇒ 仅当该位还需被后随子表达式复用时才可能不同（本件 12 式里后随者皆为
//		`[^\n]*` / 表尾 / 字面，已逐式核过无此形态）。
//	R3 `$`：py 非多行 `$` = 「串尾或串尾换行之前」⟷ Go `$` 只匹配串尾。本件输入行**无尾换行**（`splitlines()` 已去）
//		⇒ 该面**不触发**；仍写进未覆盖面。涉及 F03/F12 的 `$`。
//	R4 `\uXXXX`：本件 12 式**不含** `\u`/`\U` 转义（译文函数仍备该分支以保通用）。
//	R5 `errors="replace"` 非 UTF-8：py 按非法**序列**替一个 U+FFFD ⟷ Go `strings.ToValidUTF8` 按无效**字节**各替一个
//		⇒ 粒度不同（真仓件皆合法 UTF-8 ⇒ 不触发），写进未覆盖面。
//	R6 `str.splitlines()` 的断行符集（含 `\v`/`\f`/`\x1c-\x1e`/`\x85`/`\u2028`/`\u2029`）本件**逐字复刻**
//		（`cbiSplitLines`），✗ 不用 Go 的 `strings.Split(s, "\n")`（那会漏断行符 ⇒ 行号/整行注释计数假差）。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① argparse **argparse 版本措辞差**：py ≤3.9 印 `optional arguments:` · ≥3.10 印 `options:`（同一格在两条 PATH
//	  上给出**相反**答案）。本件帮助正文**钉 py 3.9.6 措辞**（`optional arguments:`；与同包 50 枚已搬件同钉）。
//	  ★ 门跑环境若换成 ≥3.10 解释器，则 `-h` 一格仅**该行**差 11 字节（**环境依赖 · 已声明**，非实现差）。
//	② argparse 极端内部形态未逐一验证：`store_true` 带 `=值` 的 `ignored explicit argument` 文案已复刻；
//	  但 `--help=值`、缩写歧义的多候选**全匹列**（本件已按定义序列出）之外的长尾未穷举。
//	③ `--registry` 的**解析错误正文**：py 印 `str(e)`，本件**逐字复刻** FileNotFoundError（`[Errno 2] ...`）、
//	  PermissionError（`[Errno 13] ...`）、IsADirectoryError（`[Errno 21] ...`）；但 `json.JSONDecodeError` 的
//	  `line/column/char` 三数与 `ValueError("登记册须是 JSON 数组")` 之外的 JSON 长尾正文**未逐字复刻**
//	  （该档只保 rc=2）。④ 登记册元素**非 dict**（py 会 AttributeError rc=1）本件 fail-closed rc=2（**有意口径差**）。
//	⑤ 见 (a)：默认根「脚本位置 ≠ cwd 上溯根」时不属逐字对齐承诺。⑥ 见 R2/R3/R5。
//	⑦ `--self-test`：py 侧经 `subprocess.run([sys.executable, SELF_PATH] ...)` **真起子进程**，本件改**进程内**跑同一
//	  管线（`cbiRun`）并比对 rc/正文 ⇒ 属「第二类·运行时可表达性差」，**在 py 自检全过时逐字节同**（已实测：py 全过
//	  ⇒ 各档 `tail` 恒空 ⇒ 随机 tmp 路径不入输出）。若某档不过，py 的 `tail` 取「子进程 stdout+stderr 末行」，本件
//	  以进程内同一缓冲复刻（同源 ⇒ 应同；仍属未覆盖面）。
//	⑧ `os.path.abspath`/`os.path.relpath`/`os.path.dirname` 用**词法**自写（`cbiPy*`），✗ 不用 `filepath.*`。
//
// `--evidence GAP-20261001-13`（在册 M2 承接 · 先已回读 · 语义对口 · 状态目录 `~/.zerg/state/zerg-cli-gaps.jsonl`）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── 常量与数据（逐字照 py）──────────────────────────────────────────────────

const cbiProg = "check-bypass-idioms.py"

// cbiSelfName —— py `SELF_NAME = os.path.basename(__file__)`：面内同名件自排除。
const cbiSelfName = "check-bypass-idioms.py"

const cbiRegistryCeiling = 2

// cbiScanSuffixes —— py `SCAN_SUFFIX = (".sh", ".py")`（str.endswith 元组口径）。
var cbiScanSuffixes = []string{".sh", ".py"}

// cbiExcludeDirs —— 通用排除表（目录**名**成员判定；读表不打序）。
var cbiExcludeDirs = map[string]bool{
	"target": true, "node_modules": true, "dist": true, "bin": true, "vendor": true,
	"data": true, ".git": true, ".venv": true, "venv": true, ".build": true, "_history": true,
	"__pycache__": true,
}

type cbiForm struct {
	id    string
	rx    string
	short string
	why   string
}

// cbiForms —— 12 条正则形态（**唯一真源**）· 逐字照 py `FORMS`。
var cbiForms = []cbiForm{
	{"F01-DEVNULL", `2\s*>\s*/dev/null`,
		"吞掉 stderr", "读不到的原因被丢掉 ⇒ 只看得见空值、看不见失败"},
	{"F02-OR-TRUE", `\|\|\s*true\b`,
		"失败折成成功", "`|| true` 把 rc 抹平成 0 ⇒ 判据失败与通过同形"},
	{"F03-OR-COLON", `\|\|\s*:(?:\s|$|;)`,
		"no-op 兜底", "`|| :` 同上（成功码 0，无输出）"},
	{"F04-SKIP-EXIT0", `exit\s+0\b[^\n]*(跳过|略过|skip|不做)|(跳过|略过|skip|不做)[^\n]*exit\s+0\b`,
		"跳过即绿", "明写「跳过」却 `exit 0` ⇒ 未判折绿"},
	{"F05-MISSING-EXIT0", `exit\s+0\b[^\n]*(无令牌|缺令牌|无凭据|未找到|读不到|不可达|不存在|没有)|(无令牌|缺令牌|无凭据|未找到|读不到|不可达|不存在)[^\n]*exit\s+0\b`,
		"前置缺失即绿", "前置（令牌/远端/件）缺失 ⇒ `exit 0` ⇒ 未判折绿"},
	{"F06-NONEMPTY-GUARD", `\[\s*-n\s+\"?\$[A-Za-z_][A-Za-z0-9_]*\"?\s*\]`,
		"仅当非空才检", "变量取自外部（如远端基线）而空值直接跳过整段判据"},
	{"F07-LSREMOTE-SWALLOW", `git\s+ls-remote[^\n]*(?:2\s*>\s*/dev/null|\|\|\s*true)`,
		"远端基线读取吞错", "读远端吞 stderr / 折 rc ⇒ 读不到被当成没坏（GAP-133①）"},
	{"F08-CURL-FOLD", `curl\b[^\n]*\|\|\s*(?:true|:|exit\s+0)`,
		"网络失败折绿", "curl 失败被 `|| true/:` 折成成功"},
	{"F09-FORCE-FLAG", `--force\b|--no-verify\b|--skip-[a-z][a-z-]*`,
		"绕闸旗标", "强推/免检/跳步旗标须成文登记其依据"},
	{"F10-SKIP-ENV", `\b[A-Z][A-Z0-9_]*(?:SKIP|BYPASS|FORCE)[A-Z0-9_]*\s*=\s*[1-9]`,
		"环境旋钮绕闸", "`SKIP_*`/`BYPASS_*`/`FORCE_*` 环境位把门关掉"},
	{"F11-EMPTY-DEFAULT", `\$\{[A-Za-z_][A-Za-z0-9_]*:-\}`,
		"空值兜底", "`${X:-}` 把未设/空值静默折成空串"},
	{"F12-JUDGE-FOLD", `(?:merge-base|is-ancestor|rev-parse|grep\s+-q|grep\s+-c|test\s+-|\{)[^\n]*\|\|\s*(?:true|:|exit\s+0)`,
		"判据失败折绿", "祖先/解析/grep 判据失败被折成成功"},
}

// cbiCompiled —— 预编译（写坏 ⇒ 建表即失败，调用方按「不给结论」处理）。
var cbiCompiled = func() []cbiCompiledForm {
	out := make([]cbiCompiledForm, 0, len(cbiForms))
	for _, f := range cbiForms {
		re, err := regexp.Compile(cbiPyRegexToGo(f.rx))
		if err != nil {
			return nil // 与 py 预编译写坏同档：建不出表 ⇒ 判据没牙
		}
		out = append(out, cbiCompiledForm{id: f.id, re: re, short: f.short, why: f.why})
	}
	return out
}()

type cbiCompiledForm struct {
	id    string
	re    *regexp.Regexp
	short string
	why   string
}

// cbiRegistryForm —— 登记册条目（py 侧是 dict；`.get(k)` 语义见 cbiEntryStr）。
type cbiRegistryForm = map[string]any

// cbiRegistry —— 登记册（**唯一真源** · 只许减不许增）。逐字照 py `REGISTRY`。
var cbiRegistry = []cbiRegistryForm{
	{"form": "F07-LSREMOTE-SWALLOW", "path": "publish/mirror-cn.sh",
		"anchor": `git ls-remote "$REMOTE" refs/heads/main 2>/dev/null | cut -f1 || true`,
		"reason": "GAP-133 ① 待修：远端读不到被折成 rc=0、快进判据整段不跑",
		"source": "GAP-20260927-133", "gate": "本件（待接线）"},
	{"form": "F05-MISSING-EXIT0", "path": "publish/mirror-cn-init.sh",
		"anchor": "（无令牌 ⇒ 跳过 API 校验 ✓）",
		"reason": "GAP-133 ② 待修：无令牌 = 未判，却 `exit 0`",
		"source": "GAP-20260927-133", "gate": "本件（待接线）"},
}

// ── py 语义薄壳 ──────────────────────────────────────────────────────────

// cbiPyJoin —— `os.path.join` 逐字（保 `..`/后段绝对/空段三类语义）。
func cbiPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		if strings.HasPrefix(p, "/") || out == "" {
			out = p
			continue
		}
		if strings.HasSuffix(out, "/") {
			out = out + p
		} else {
			out = out + "/" + p
		}
	}
	return out
}

// cbiJoin —— 斜杠拼串（rel 只作打印/前缀匹配，✗ 不落盘）。
func cbiJoin(a, b string) string {
	if a == "" {
		return b
	}
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// cbiPyDir —— `os.path.dirname`（词法；见 posixpath.split 的剥尾斜杠语义）。
func cbiPyDir(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	if i == 0 {
		return "/"
	}
	return p[:i]
}

// cbiNormPath —— `os.path.normpath` 纯词法版（★ 保留恰两个前导斜杠）。
func cbiNormPath(p string) string {
	if p == "" {
		return "."
	}
	abs := strings.HasPrefix(p, "/")
	double := strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///")
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
	if double {
		res = "/" + res
	}
	return res
}

// cbiAbs —— `os.path.abspath`（相对 ⇒ 拼 cwd；再 normpath）。
func cbiAbs(p string) string {
	if !strings.HasPrefix(p, "/") {
		wd, err := os.Getwd()
		if err != nil {
			wd = "."
		}
		p = wd + "/" + p
	}
	return cbiNormPath(p)
}

// cbiRelPath —— `os.path.relpath(path, start)` 纯词法版。
func cbiRelPath(path, start string) string {
	p := cbiAbs(path)
	s := cbiAbs(start)
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

// cbiRepoRoot —— 见 (a)：env `ZERG_REPO_ROOT` 优先；否则 cwd 上溯找含 `scripts/gates/` 的目录。
func cbiRepoRoot() string {
	if env := os.Getenv("ZERG_REPO_ROOT"); env != "" {
		return cbiAbs(env)
	}
	return cbiDefaultRepo()
}

func cbiDefaultRepo() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d := cbiAbs(wd)
	for i := 0; i < 24; i++ {
		if st, e := os.Stat(cbiJoin(cbiJoin(d, "scripts"), "gates")); e == nil && st.IsDir() {
			return d
		}
		parent := cbiPyDir(d)
		if parent == "" || parent == d {
			break
		}
		d = parent
	}
	return cbiAbs(wd)
}

// cbiIsSymlink —— `os.path.islink`（Lstat）。
func cbiIsSymlink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

// cbiUniversalNewlines —— py 文本模式默认 `newline=None` 的换行归一（`\r\n`/`\r` ⇒ `\n`）。
func cbiUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// cbiReadText —— 读件 ⇒ 逐字近似 py `open(..., encoding="utf-8", errors="replace")` + 通用换行。
func cbiReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return cbiUniversalNewlines(strings.ToValidUTF8(string(b), "\uFFFD")), nil
}

// cbiReadFails —— B 类读失败登记面（读不了的件 / 目录 ⇒ 逐件登记 相对路径 + err 正文）。
// ★ B 类真口子修（与 py **有意分叉** · 同族先例 `check_doc_cmds.go` 的 `dcmdReadFails`）：
//
//	读失败 ⇒ 登记 + 入口判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（不再静默少判）。
//	自清点：每次 `cbiScan` 起手清空 ⇒ 不许跨调用残留（残留会让下一次一进门就被 BLOCKED）。
var cbiReadFails []string

// cbiIsLinebreak —— py `str.splitlines()` 认的断行符（`_PyUnicode_IsLinebreak`）。
func cbiIsLinebreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// cbiSplitLines —— py `str.splitlines()` 的等价（断行符集逐字复刻；`\r\n` 计一次）。
func cbiSplitLines(s string) []string {
	if s == "" {
		return nil
	}
	rs := []rune(s)
	var out []string
	start := 0
	i := 0
	for i < len(rs) {
		if cbiIsLinebreak(rs[i]) {
			out = append(out, string(rs[start:i]))
			cr := rs[i] == '\r'
			i++
			if cr && i < len(rs) && rs[i] == '\n' {
				i++
			}
			start = i
			continue
		}
		i++
	}
	if start < len(rs) {
		out = append(out, string(rs[start:]))
	}
	return out
}

// cbiIsPySpace —— py `str.isspace()` 认的空白（含 `\x1c-\x1f`；✗ 不是 Go 的 `unicode.IsSpace` 全等）。
func cbiIsPySpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	if r > 0x7f {
		return unicode.IsSpace(r)
	}
	return false
}

// cbiPyStrip —— py `str.strip()`（Unicode 空白）。
func cbiPyStrip(s string) string {
	return strings.TrimFunc(s, cbiIsPySpace)
}

// cbiPyRStrip —— py `str.rstrip()`。
func cbiPyRStrip(s string) string {
	return strings.TrimRightFunc(s, cbiIsPySpace)
}

// cbiPyLStrip —— py `str.lstrip()`。
func cbiPyLStrip(s string) string {
	return strings.TrimLeftFunc(s, cbiIsPySpace)
}

// cbiRuneCut —— py 串切片 `s[:n]`（按**码点**，✗ 不是字节）。
func cbiRuneCut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func cbiHasSuffix(name string, suffixes []string) bool {
	for _, s := range suffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

// ── 正则：py `re` ⇒ Go RE2（R1–R4 的对齐；不可对齐者已在件头声明）────────────

// cbiPyRegexToGo —— 把 py 正则源串译成 Go RE2 等价串（`\d`→`\p{Nd}` · `\b`→消费式词界 · `\uXXXX`→`\x{..}`）。
func cbiPyRegexToGo(pat string) string {
	var b strings.Builder
	rs := []rune(pat)
	i := 0
	for i < len(rs) {
		c := rs[i]
		if c != '\\' || i+1 >= len(rs) {
			b.WriteRune(c)
			i++
			continue
		}
		n := rs[i+1]
		switch n {
		case 'd':
			b.WriteString(`\p{Nd}`)
			i += 2
		case 'D':
			b.WriteString(`\P{Nd}`)
			i += 2
		case 's':
			b.WriteString(cbiPyWSClass)
			i += 2
		case 'S':
			b.WriteString(`[^` + cbiPyWSBody + `]`)
			i += 2
		case 'w':
			b.WriteString(`[\p{L}\p{N}_]`)
			i += 2
		case 'W':
			b.WriteString(`[^\p{L}\p{N}_]`)
			i += 2
		case 'b':
			b.WriteString(`(?:[^\p{L}\p{N}_]|\A|\z)`)
			i += 2
		case 'Z':
			b.WriteString(`\z`)
			i += 2
		case 'u':
			if i+5 < len(rs) && cbiAllHex(rs[i+2:i+6], 4) {
				b.WriteString(`\x{` + string(rs[i+2:i+6]) + `}`)
				i += 6
			} else {
				b.WriteRune(c)
				i++
			}
		case 'U':
			if i+9 < len(rs) && cbiAllHex(rs[i+2:i+10], 8) {
				b.WriteString(`\x{` + string(rs[i+2:i+10]) + `}`)
				i += 10
			} else {
				b.WriteRune(c)
				i++
			}
		default:
			b.WriteRune(c)
			b.WriteRune(n)
			i += 2
		}
	}
	return b.String()
}

const cbiPyWSBody = `\t\n\v\f\r \x{001c}-\x{001f}\x{0085}\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`
const cbiPyWSClass = `[` + cbiPyWSBody + `]`

func cbiAllHex(rs []rune, n int) bool {
	if len(rs) != n {
		return false
	}
	for _, r := range rs {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// ── 扫描 ────────────────────────────────────────────────────────────────

type cbiOcc struct {
	rel  string
	line int
	text string
	hits []string
}

// cbiScan —— py `scan`（T1–T5）⇒ (occurrences, comments, files_scanned)。
func cbiScan(roots []string, repoRoot string) ([]cbiOcc, int, int) {
	cbiReadFails = nil // ★ 自清：包级登记面不许跨调用残留
	var occ []cbiOcc
	comments := 0
	files := 0
	for _, root := range roots {
		st, err := os.Stat(root) // os.path.isdir（跟随符号链接）
		if err != nil || !st.IsDir() {
			continue
		}
		cbiWalkInto(&occ, &comments, &files, repoRoot, root, root)
	}
	return occ, comments, files
}

// cbiWalkInto —— 一层 + 递归（T1 遍历序 / T2 排除目录名 / T3 Stat 分类 / T4 层内文件排序）。
func cbiWalkInto(occ *[]cbiOcc, comments, files *int, repoRoot, root, dir string) {
	f, err := os.Open(dir)
	if err != nil {
		// ★ B 类真口子修（原 py `os.walk(onerror=None)` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
		cbiReadFails = append(cbiReadFails, fmt.Sprintf("%s（%v）", cbiRelOf(dir, repoRoot, root), err))
		return
	}
	ents, err := f.ReadDir(-1) // ★ T1 readdir 原序（✗ 排序版 os.ReadDir / ✗ WalkDir）
	_ = f.Close()
	if err != nil {
		return
	}
	var dirs []string
	var filenames []string
	for _, e := range ents {
		name := e.Name()
		full := cbiJoin(dir, name)
		st, serr := os.Stat(full) // ★ T3 跟随符号链接 = py entry.is_dir()
		if serr == nil && st.IsDir() {
			if !cbiExcludeDirs[name] { // T2 目录名成员判定（保序）
				dirs = append(dirs, name)
			}
			continue
		}
		filenames = append(filenames, name)
	}
	sort.Strings(filenames) // T4 层内文件名按 py 字典序
	for _, fn := range filenames {
		if !cbiHasSuffix(fn, cbiScanSuffixes) {
			continue
		}
		if fn == cbiSelfName { // 自排除（本件 py 原件名）
			continue
		}
		full := cbiJoin(dir, fn)
		rel := cbiRelOf(full, repoRoot, root)
		*files++
		text, rerr := cbiReadText(full)
		if rerr != nil {
			// ★ B 类真口子修（原 py `except OSError: continue` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
			cbiReadFails = append(cbiReadFails, fmt.Sprintf("%s（%v）", rel, rerr))
			continue
		}
		for i, line := range cbiSplitLines(text) {
			if strings.HasPrefix(cbiPyLStrip(line), "#") {
				*comments++
				continue
			}
			hits := cbiHits(line)
			if len(hits) > 0 {
				*occ = append(*occ, cbiOcc{rel: rel, line: i + 1, text: cbiPyRStrip(line), hits: hits})
			}
		}
	}
	for _, d := range dirs { // T1 子目录原序递归
		full := cbiJoin(dir, d)
		if cbiIsSymlink(full) { // followlinks=False ⇒ 符号链接目录不下钻
			continue
		}
		cbiWalkInto(occ, comments, files, repoRoot, root, full)
	}
}

// cbiRelOf —— py `rel = relpath(full, REPO_ROOT) if full.startswith(REPO_ROOT + sep) else relpath(full, root)`。
func cbiRelOf(full, repoRoot, root string) string {
	if strings.HasPrefix(full, repoRoot+"/") {
		return cbiRelPath(full, repoRoot)
	}
	return cbiRelPath(full, root)
}

// cbiHits —— 逐行命中的形态 id 表（按 FORMS 表序，切片保序）。
func cbiHits(line string) []string {
	var hits []string
	for _, cf := range cbiCompiled {
		if cf.re.MatchString(line) {
			hits = append(hits, cf.id)
		}
	}
	return hits
}

// ── 判决 ────────────────────────────────────────────────────────────────

type cbiStale struct {
	idx int
	e   cbiRegistryForm
}

// cbiJudge —— py `judge` ⇒ (unregistered, stale)。
func cbiJudge(occ []cbiOcc, registry []cbiRegistryForm) ([]cbiOcc, []cbiStale) {
	used := map[int]bool{}
	var unregistered []cbiOcc
	for _, o := range occ {
		matched := false
		for idx, e := range registry {
			path, _ := cbiEntryStr(e, "path")
			if path != o.rel {
				continue
			}
			if anchor, ok := cbiEntryStr(e, "anchor"); ok && anchor != "" {
				if strings.Contains(o.text, anchor) {
					if form, ok2 := cbiEntryStr(e, "form"); ok2 && cbiContains(o.hits, form) {
						matched = true
						used[idx] = true
					}
				}
			}
		}
		if !matched {
			unregistered = append(unregistered, o)
		}
	}
	var stale []cbiStale
	for i, e := range registry {
		if !used[i] {
			stale = append(stale, cbiStale{idx: i, e: e})
		}
	}
	return unregistered, stale
}

// cbiEntryStr —— py `e.get(k)` 的字符串档（缺键 / 非字符串 ⇒ ok=false，对应 None/非串）。
func cbiEntryStr(m cbiRegistryForm, k string) (string, bool) {
	v, ok := m[k]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// cbiEntryShow —— py 打印 `e.get(k)`：缺键 ⇒ None。
func cbiEntryShow(m cbiRegistryForm, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return "None"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func cbiContains(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}

// ── 机器面（byte 级复刻 py `json.dumps(obj, ensure_ascii=False)`）─────────────

// cbiJSONStr —— py `json.dumps(str, ensure_ascii=False)` 的字符串格。
func cbiJSONStr(s string) string {
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
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
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

// cbiJSONStrList —— py `json.dumps([str, ...])` 的数组格（空 ⇒ `[]`；★ 不 omitempty）。
func cbiJSONStrList(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = cbiJSONStr(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cbiJSONValue —— 通用 JSON 值（None ⇒ null；字符串 ⇒ 引号格）。
func cbiJSONValue(v any) string {
	if v == nil {
		return "null"
	}
	if s, ok := v.(string); ok {
		return cbiJSONStr(s)
	}
	if b, ok := v.(bool); ok {
		if b {
			return "true"
		}
		return "false"
	}
	if f, ok := v.(float64); ok {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return cbiJSONStr(fmt.Sprintf("%v", v))
}

// cbiJSONPayload —— py `--json` 的载荷（键**固定序**；`registry` 用 **global** 条数）。
func cbiJSONPayload(roots []string, files, occN, comments, registryN int, unreg []cbiOcc, stale []cbiStale) string {
	var ui []string
	for _, o := range unreg {
		ui = append(ui, "{"+cbiJSONStr("file")+": "+cbiJSONStr(o.rel)+
			", "+cbiJSONStr("line")+": "+strconv.Itoa(o.line)+
			", "+cbiJSONStr("forms")+": "+cbiJSONStrList(o.hits)+"}")
	}
	var si []string
	for _, s := range stale {
		var v any
		v = s.e["form"]
		f := cbiJSONValue(v)
		v = s.e["path"]
		p := cbiJSONValue(v)
		v = s.e["anchor"]
		an := cbiJSONValue(v)
		si = append(si, "{"+cbiJSONStr("form")+": "+f+
			", "+cbiJSONStr("path")+": "+p+
			", "+cbiJSONStr("anchor")+": "+an+"}")
	}
	parts := []string{
		cbiJSONStr("schema") + ": " + cbiJSONStr("zerg/v1"),
		cbiJSONStr("kind") + ": " + cbiJSONStr("BypassIdiomScan"),
		cbiJSONStr("roots") + ": " + cbiJSONStrList(roots),
		cbiJSONStr("files_scanned") + ": " + strconv.Itoa(files),
		cbiJSONStr("occurrences") + ": " + strconv.Itoa(occN),
		cbiJSONStr("comments") + ": " + strconv.Itoa(comments),
		cbiJSONStr("forms") + ": " + strconv.Itoa(len(cbiForms)),
		cbiJSONStr("registry") + ": " + strconv.Itoa(registryN),
		cbiJSONStr("registry_ceiling") + ": " + strconv.Itoa(cbiRegistryCeiling),
		cbiJSONStr("unregistered") + ": " + strconv.Itoa(len(unreg)),
		cbiJSONStr("stale") + ": " + strconv.Itoa(len(stale)),
		cbiJSONStr("unregistered_items") + ": [" + strings.Join(ui, ", ") + "]",
		cbiJSONStr("stale_items") + ": [" + strings.Join(si, ", ") + "]",
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// ── list-forms 正文 ──────────────────────────────────────────────────────

func cbiListForms() string {
	var b strings.Builder
	p := func(s string) { b.WriteString(s); b.WriteString("\n") }
	p("⑯ 元门 · 绕闸形态登记册（缺口 GAP-20260927-133 ⑤）")
	p("判据：面内每一处绕闸形态都必须对上登记册一条（内容锚 · 理由 · 出处 · 看着它的门）；对不上 ⇒ rc=1")
	p("退码：0 全绿 · 1 有未登记绕闸形态 · 2 不给结论（扫面 0 件 / FORMS 空 / 登记超上限 / 自检不过）")
	p("12 条形态：")
	for _, f := range cbiForms {
		p(fmt.Sprintf("  · %-20s %s —— %s", f.id, f.short, f.why))
	}
	p(fmt.Sprintf("登记册（%d 条 · 上限 %d）：", len(cbiRegistry), cbiRegistryCeiling))
	for _, e := range cbiRegistry {
		p(fmt.Sprintf("  · [%s] %s  «%s»  ← %s · %s",
			cbiEntryShow(e, "form"), cbiEntryShow(e, "path"), cbiEntryShow(e, "anchor"),
			cbiEntryShow(e, "source"), cbiEntryShow(e, "gate")))
	}
	return b.String()
}

// ── 用法 / argparse 复刻 ──────────────────────────────────────────────────

const cbiUsage = "usage: check-bypass-idioms.py [-h] [--root ROOT] [--registry REGISTRY]\n" +
	"                              [--json] [--list-forms]\n"

const cbiHelp = cbiUsage + "\n" +
	"optional arguments:\n" +
	"  -h, --help           show this help message and exit\n" +
	"  --root ROOT          判哪棵树（可重复；默认 publish/ + scripts/）\n" +
	"  --registry REGISTRY  追加登记册（JSON 数组 · 自检正控用）\n" +
	"  --json\n" +
	"  --list-forms\n"

type cbiArgs struct {
	roots     []string
	registry  string
	json      bool
	listForms bool
}

var cbiLongOpts = []string{"--root", "--registry", "--json", "--list-forms", "--help"}

var cbiNeedsValue = map[string]bool{"--root": true, "--registry": true}

// cbiStoreTrue —— argparse `store_true`（不接受 `=值`）。
var cbiStoreTrue = map[string]bool{"--json": true, "--list-forms": true}

// cbiResolveLong —— 长旗标**前缀匹配**（唯一命中 ⇒ 规范名；多命中 ⇒ 歧义）。
func cbiResolveLong(tok string) (string, string, bool, error) {
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
		for _, o := range cbiLongOpts {
			if o == name {
				matches = []string{o}
				break
			}
			if strings.HasPrefix(o, name) {
				matches = append(matches, o)
			}
		}
		if len(matches) == 0 {
			return "", "", false, nil
		}
		if len(matches) > 1 {
			return "", "", false, fmt.Errorf("ambiguous option: %s could match %s", name, strings.Join(matches, ", "))
		}
		name = matches[0]
	} else {
		return "", "", false, nil
	}
	if hasInline {
		return name, inline, true, nil
	}
	return name, "\x00none", false, nil
}

// cbiParseArgs —— argparse 等价解析。
func cbiParseArgs(argv []string) (*cbiArgs, string, bool, error) {
	args := &cbiArgs{}
	var unrecognized []string
	endOpts := false
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if endOpts {
			unrecognized = append(unrecognized, tok)
			continue
		}
		if tok == "--" {
			endOpts = true
			continue
		}
		if tok == "-h" || tok == "--help" {
			return nil, cbiHelp, true, nil
		}
		if !strings.HasPrefix(tok, "-") || tok == "-" {
			unrecognized = append(unrecognized, tok)
			continue
		}
		name, inline, hasInline, err := cbiResolveLong(tok)
		if err != nil {
			return nil, "", false, err
		}
		if name == "" {
			unrecognized = append(unrecognized, tok)
			continue
		}
		val := inline
		if cbiStoreTrue[name] && hasInline {
			return nil, "", false, fmt.Errorf("argument %s: ignored explicit argument '%s'", name, inline)
		}
		if inline == "\x00none" {
			if cbiNeedsValue[name] {
				if i+1 >= len(argv) || (strings.HasPrefix(argv[i+1], "-") && argv[i+1] != "-" && len(argv[i+1]) > 1) {
					return nil, "", false, fmt.Errorf("argument %s: expected one argument", name)
				}
				i++
				val = argv[i]
			}
		}
		switch name {
		case "--root":
			args.roots = append(args.roots, val)
		case "--registry":
			args.registry = val
		case "--json":
			args.json = true
		case "--list-forms":
			args.listForms = true
		}
	}
	if len(unrecognized) > 0 {
		return nil, "", false, fmt.Errorf("unrecognized arguments: %s", strings.Join(unrecognized, " "))
	}
	return args, "", false, nil
}

// ── 登记册加载 ────────────────────────────────────────────────────────────

// cbiLoadRegistry —— py `load_registry`（错误正文交调用方打印；✗ 空串 = ok）。
func cbiLoadRegistry(path string) ([]cbiRegistryForm, string) {
	f, err := os.Open(path)
	if err != nil {
		return nil, cbiOSErrText(path, err)
	}
	defer f.Close()
	data, rerr := io.ReadAll(f)
	if rerr != nil {
		return nil, cbiOSErrText(path, rerr)
	}
	var raw any
	if derr := cbiJSONDecodeBytes(data, &raw); derr != "" {
		return nil, derr
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, "登记册须是 JSON 数组"
	}
	out := make([]cbiRegistryForm, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			// py：`e.get(...)` 对非 dict 抛 AttributeError ⇒ 未捕 rc=1。本件 fail-closed（件头未覆盖面④）。
			return nil, "登记册元素非 JSON 对象（py 侧会 AttributeError ⇒ rc=1；本件按不给结论处理）"
		}
		out = append(out, cbiRegistryForm(m))
	}
	return out, ""
}

// cbiOSErrText —— py `str(OSError)` 的常见三档逐字复刻。
func cbiOSErrText(path string, err error) string {
	if pe, ok := err.(*os.PathError); ok {
		switch {
		case os.IsNotExist(pe):
			return "[Errno 2] No such file or directory: '" + path + "'"
		case os.IsPermission(pe):
			return "[Errno 13] Permission denied: '" + path + "'"
		case strings.Contains(pe.Err.Error(), "is a directory"):
			return "[Errno 21] Is a directory: '" + path + "'"
		}
	}
	return err.Error()
}

// cbiJSONDecodeBytes —— JSON 解析（错误正文未逐字复刻 ⇒ 件头未覆盖面③；返回非空 = 错）。
func cbiJSONDecodeBytes(data []byte, out *any) string {
	if err := json.Unmarshal(data, out); err != nil {
		return err.Error()
	}
	return ""
}

// ── 自检（成对负控 · 合成夹具 · 不碰真目标）────────────────────────────────

// cbiStAssert —— py `_st`：正控比对 rc；✗ 时附末行 tail（≤90 码点）。
func cbiStAssert(stdout io.Writer, bad *int, label string, want, rc int, out string) bool {
	good := rc == want
	if !good {
		*bad++
	}
	tail := ""
	if !good {
		lines := cbiSplitLines(cbiPyStrip(out))
		last := ""
		if len(lines) > 0 {
			last = lines[len(lines)-1]
		}
		tail = " ｜ " + cbiRuneCut(last, 90)
	}
	mark := "✓"
	if !good {
		mark = "✗"
	}
	fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
	return good
}

// cbiSelfTest —— py `self_test()`（print 落 stdout；返回 rc）。
func cbiSelfTest(stdout io.Writer) int {
	bad := 0
	fmt.Fprintln(stdout, "── check-bypass-idioms.py --self-test（正控 0 · 负控 1 · 空转/用法 2）")
	if len(cbiForms) == 12 && len(cbiCompiled) == 12 {
		fmt.Fprintf(stdout, "  ✓ 形态表：12 条正则形态（%s … %s）\n", cbiForms[0].id, cbiForms[len(cbiForms)-1].id)
	} else {
		bad++
		fmt.Fprintf(stdout, "  ✗ 形态表：期望 12 条，实得 %d\n", len(cbiForms))
	}
	if len(cbiRegistry) <= cbiRegistryCeiling {
		fmt.Fprintf(stdout, "  ✓ 棘轮：登记册 %d 条 ≤ 上限 %d（只许减不许增）\n", len(cbiRegistry), cbiRegistryCeiling)
	} else {
		bad++
		fmt.Fprintf(stdout, "  ✗ 棘轮：登记册 %d 条 > 上限 %d —— 被破\n", len(cbiRegistry), cbiRegistryCeiling)
	}

	tmp, terr := os.MkdirTemp("", "bypass-idioms-selftest-")
	if terr != nil {
		// 建点失败 ⇒ 不删（✗ 无条件 RemoveAll 会误删系统临时目录）
		fmt.Fprintln(stdout, "  ✗ 自检夹具建点失败")
		fmt.Fprintln(stdout, "── 自检：**1 档不过** ✗")
		return 2
	}
	defer os.RemoveAll(tmp) // ★ 安全删点：仅在 MkdirTemp 成功支

	badD := cbiJoin(tmp, "bad")
	okD := cbiJoin(tmp, "ok")
	emptyD := cbiJoin(tmp, "empty")
	for _, d := range []string{badD, okD, emptyD} {
		_ = os.MkdirAll(d, 0o755)
	}
	badLines := []string{
		`BASE=$(git ls-remote "git@example.invalid:o/r.git" refs/heads/main 2>/dev/null | cut -f1 || true)`,
		`T=$(read_token 2>/dev/null) || { echo "（无令牌 ⇒ 跳过校验 ✓）"; exit 0; }`,
	}
	_ = os.WriteFile(cbiJoin(badD, "probe.sh"), []byte(strings.Join(badLines, "\n")+"\n"), 0o644)
	okLines := []string{
		"# 注释里出现 exit 0 与 2>/dev/null 都不入判",
		`BASE=$(git ls-remote "$R" refs/heads/main); LSRC=$?`,
		`if [ "$LSRC" -ne 0 ]; then echo "读不到 ⇒ rc=2" >&2; exit 2; fi`,
	}
	_ = os.WriteFile(cbiJoin(okD, "probe.sh"), []byte(strings.Join(okLines, "\n")+"\n"), 0o644)
	reg := "[" +
		`{"form": "F07-LSREMOTE-SWALLOW", "path": "probe.sh", "anchor": "2>/dev/null | cut -f1 || true", "reason": "自检", "source": "自检", "gate": "本件"}, ` +
		`{"form": "F05-MISSING-EXIT0", "path": "probe.sh", "anchor": "（无令牌 ⇒ 跳过校验 ✓）", "reason": "自检", "source": "自检", "gate": "本件"}` +
		"]"
	regf := cbiJoin(tmp, "reg.json")
	_ = os.WriteFile(regf, []byte(reg), 0o644)

	run := func(a ...string) (int, string) {
		var o, e strings.Builder
		rc := cbiRun(a, cbiRepoRoot(), &o, &e)
		return rc, o.String() + e.String()
	}

	rc1, out1 := run("--root", badD)
	rc2, out2 := run("--root", okD)
	rc3, out3 := run("--root", emptyD)
	rc4, out4 := run("--root", badD, "--registry", regf)
	cbiStAssert(stdout, &bad, "档① 负控：未登记绕闸形态（同形 GAP-133 ①②）⇒ rc=1", 1, rc1, out1)
	cbiStAssert(stdout, &bad, "档② 正控：等价但已收 rc 的写法（无绕闸形态）⇒ rc=0", 0, rc2, out2)
	cbiStAssert(stdout, &bad, "档③ 空转：面内 0 个 .sh/.py ⇒ rc=2（不许当绿）", 2, rc3, out3)
	cbiStAssert(stdout, &bad, "档④ 登记转绿：同一批绕闸处按内容锚登记 ⇒ rc=0（棘轮可减）", 0, rc4, out4)

	concl := "全过 ✓"
	if bad > 0 {
		concl = fmt.Sprintf("**%d 档不过** ✗", bad)
	}
	fmt.Fprintf(stdout, "── 自检：%s\n", concl)
	if bad == 0 {
		return 0
	}
	return 2
}

// ── 主流程 ────────────────────────────────────────────────────────────────

// checkBypassIdiomsGateMain —— 入口（与 py `main()` 逐字同构）。
func checkBypassIdiomsGateMain(stdout, stderr io.Writer, argv []string) int {
	return cbiRun(argv, cbiRepoRoot(), stdout, stderr)
}

// cbiRun —— 进程内主线（自检与入口共用；repoRoot 显式传入以对齐 py 的 REPO_ROOT）。
func cbiRun(argv []string, repoRoot string, stdout, stderr io.Writer) int {
	// py：`if "--self-test" in sys.argv[1:] or "--selftest" in sys.argv[1:]`（argparse 之前）
	for _, a := range argv {
		if a == "--self-test" || a == "--selftest" {
			return cbiSelfTest(stdout)
		}
	}
	args, helpText, helpWanted, err := cbiParseArgs(argv)
	if err != nil {
		fmt.Fprint(stderr, cbiUsage)
		fmt.Fprintf(stderr, "%s: error: %s\n", cbiProg, err.Error())
		return 2
	}
	if helpWanted {
		fmt.Fprint(stdout, helpText)
		return 0
	}

	if args.listForms {
		fmt.Fprint(stdout, cbiListForms())
		return 0
	}

	if len(cbiForms) == 0 {
		fmt.Fprint(stderr, "⛔ `FORMS` 为空（判据没牙）⇒ 不给结论（rc=2）\n")
		return 2
	}
	if len(cbiRegistry) > cbiRegistryCeiling {
		fmt.Fprintf(stderr, "⛔ 登记册 %d 条 > 上限 %d ⇒ 棘轮被破（只许减不许增）⇒ rc=2\n",
			len(cbiRegistry), cbiRegistryCeiling)
		return 2
	}

	registry := append([]cbiRegistryForm{}, cbiRegistry...)
	if args.registry != "" {
		extra, lerr := cbiLoadRegistry(args.registry)
		if lerr != "" {
			fmt.Fprintf(stderr, "⛔ 登记册读不到/不可解析：%s ⇒ 不给结论（rc=2）\n", lerr)
			return 2
		}
		registry = append(registry, extra...)
	}

	var roots []string
	if len(args.roots) > 0 {
		for _, r := range args.roots {
			roots = append(roots, cbiAbs(r))
		}
	} else {
		roots = []string{cbiPyJoin(repoRoot, "publish"), cbiPyJoin(repoRoot, "scripts")}
	}
	occ, comments, files := cbiScan(roots, repoRoot)

	// ★ B 类真口子修：读失败 ⇒ 响亮 BLOCKED rc=2 + 逐件点名（✗ 不再静默少判）。
	//   同族先例 check_doc_cmds.go 的 dcmdReadFails —— 读失败 ⇒ BLOCKED 头 + 逐件点名 + 不给结论。
	if len(cbiReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(cbiReadFails))
		for _, u := range cbiReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return 2
	}

	if files == 0 {
		fmt.Fprintf(stderr, "⛔ 空转：扫描面 **0 个** `.sh`/`.py`（根写错 or 树不对）：%s\n", strings.Join(roots, " · "))
		fmt.Fprint(stderr, "⇒ 不给结论（rc=2）：**空转 ≠ 绿**\n")
		return 2
	}

	unreg, stale := cbiJudge(occ, registry)

	if args.json {
		fmt.Fprintln(stdout, cbiJSONPayload(roots, files, len(occ), comments, len(cbiRegistry), unreg, stale))
		if len(unreg) > 0 {
			return 1
		}
		return 0
	}

	fmt.Fprint(stdout, "── 元门 · 绕闸形态登记册（GAP-20260927-133 ⑤）──\n")
	scanLine := make([]string, len(roots))
	for i, r := range roots {
		if strings.HasPrefix(r, repoRoot) {
			scanLine[i] = cbiRelPath(r, repoRoot)
		} else {
			scanLine[i] = r
		}
	}
	fmt.Fprintf(stdout, "   扫面：%s\n", strings.Join(scanLine, " · "))
	fmt.Fprintf(stdout, "   面内 .sh/.py **%d 个** · 绕闸形态命中 **%d 处** · 整行注释 %d 行（不入判）\n",
		files, len(occ), comments)
	fmt.Fprintf(stdout, "   形态表 %d 条 · 登记册 %d 条（上限 %d）\n",
		len(cbiForms), len(cbiRegistry), cbiRegistryCeiling)
	for _, s := range stale {
		fmt.Fprintf(stdout, "   ⚠ 登记了却没生效（本锚一处都没对上 ⇒ 应删此条并把上限减一）：[%s] %s «%s»\n",
			cbiEntryShow(s.e, "form"), cbiEntryShow(s.e, "path"), cbiEntryShow(s.e, "anchor"))
	}
	if len(unreg) == 0 {
		fmt.Fprint(stdout, "⇒ 全绿（rc=0）：面内绕闸形态**逐处都有登记**\n")
		return 0
	}
	fmt.Fprintf(stdout, "   ✗ 未登记的绕闸形态 **%d 处**（逐处点名）：\n", len(unreg))
	for _, o := range unreg {
		fmt.Fprintf(stdout, "      %s:%d  [%s]  %s\n", o.rel, o.line, strings.Join(o.hits, ","), cbiRuneCut(cbiPyStrip(o.text), 140))
	}
	fmt.Fprint(stdout, "⇒ 有未登记绕闸形态（rc=1）：**读不到/判不出不许折成绿** —— 要么收 rc 走 rc=2，要么按五件套登记（内容锚 · 理由 · 出处 · 看着它的门）\n")
	return 1
}
