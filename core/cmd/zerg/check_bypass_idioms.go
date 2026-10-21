// check_bypass_idioms.go —— 判据件 `scripts/gates/check-bypass-idioms.py`（343 行）的
// **Go 真搬**（v2.5.14 · M2 · 入口 `checkBypassIdiomsGateMain`）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳；M2 = 按价值真搬 Go（落点 `core/cmd/zerg/`）。
//	本件 = M2 一枚，验收判据 = 同 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码 **三面逐字相同**
//	（除下列**显式声明的**差异面）。★ 本件**已接线（inproc 表分派）**：不注册命令、不删 py 原件、不动登记面。
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

const cbiRegistryCeiling = 554

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
//
// ★ 历史存量**一次性入册**（GAP-20260927-133 ⑤ · 2026-10-13 拍 · 解封一次）：
//
//	入册前木门对存量 577 处恒红（存量永远清不掉 ⇒ 假红）；本批把 577 处按**内容锚**
//	（逐行正文原子串）**一次性登记入册** ⇒ 未登记归 0，此后仍**只许减不许增**：
//	`cbiRegistryCeiling` 只登记本次入册后的条数，✗ 永不上调（抬上限充绿即违规）；
//	治掉一处 ⇒ 该锚变 stale ⇒ 按提示**删此条并把上限减一**（棘轮只许下调）。
var cbiRegistry = []cbiRegistryForm{
	// ── 历史存量（一次性入册 · 553 条 · 覆盖 577 处）────────────────────────
	{"form": "F09-FORCE-FLAG", "path": "publish/install.sh",
		"anchor": "    codesign -s - --force \"$PREFIX/bin/$name\" >/dev/null 2>&1 || say \"   ⚠️ 重签名失败：${name}（macOS 上未签名二进制会被秒杀）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/install.sh",
		"anchor": "  gh release download ${TAG:+\"$TAG\"} --repo \"$REPO_SLUG\" --pattern manifest.json --dir \"$work\" --clobber 2>/dev/null \\",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/install.sh",
		"anchor": "  got=$(shasum -a 256 \"$work/dl/$a\" 2>/dev/null | awk '{print $1}' || sha256sum \"$work/dl/$a\" | awk '{print $1}')",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "publish/install.sh",
		"anchor": "  say \"   → 安装到 $PREFIX/bin/（旧件留 *.prev，可 mv 回滚）；macOS 上逐件 codesign -s - --force\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/install.sh",
		"anchor": "  say \"→ 源 $SRC_TAG / 本机 ${CUR:-未装}——$( [ -n \"$CUR\" ] && echo '如需升级：bash install.sh' || echo '如需安装：bash install.sh' )\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/install.sh",
		"anchor": "[ -x \"$PREFIX/bin/zerg-core\" ] && CUR=\"$(\"$PREFIX/bin/zerg-core\" --version 2>/dev/null | head -1 || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/install.sh",
		"anchor": "say \"   主控自报身份：$(\"$PREFIX/bin/zerg-core\" --version 2>/dev/null | head -1 || echo '（用 --version 查询）')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "publish/mirror-acceptance.sh",
		"anchor": "    n=\"$(git -C \"${BASE}/real-r1\" -c core.quotepath=false ls-tree -r --name-only \"$sha\" | grep -Ec \"$pat\" || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "  SEQ=$(( $(wc -l < \"$RC_FILE\" 2>/dev/null || echo 0) + 1 ))",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "publish/mirror-acceptance.sh",
		"anchor": "  diff \"${BASE}/priv-readdirty0.txt\" \"${BASE}/priv-readdirty1.txt\" | sed 's/^/      /' || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "N1=\"$(git -C \"${BASE}/toy-a1\" rev-list --count refs/heads/main 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "N2=\"$(git -C \"${BASE}/toy-a2\" rev-list --count refs/heads/main 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "RN1=\"$(git -C \"${BASE}/real-r1\" rev-list --count refs/heads/main 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "RN2=\"$(git -C \"${BASE}/real-r2\" rev-list --count refs/heads/main 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "RT1=\"$(git -C \"${BASE}/real-r1\" rev-parse 'refs/heads/main^{tree}' 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "RT2=\"$(git -C \"${BASE}/real-r2\" rev-parse 'refs/heads/main^{tree}' 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "T1=\"$(git -C \"${BASE}/toy-a1\" rev-parse 'refs/heads/main^{tree}' 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "T2=\"$(git -C \"${BASE}/toy-a2\" rev-parse 'refs/heads/main^{tree}' 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-acceptance.sh",
		"anchor": "[ -n \"$RT1\" ] && [ \"$RT1\" = \"$RT2\" ] && ok \"A(真仓 3 笔) tree hash 完全相同：${RT1:0:12}\" || bad \"A(真仓) tree hash 不同\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-acceptance.sh",
		"anchor": "[ -n \"$T1\" ] && [ \"$T1\" = \"$T2\" ] && ok \"A(小仓) tree hash 完全相同：${T1:0:12}\" || bad \"A(小仓) tree hash 不同（${T1} vs ${T2}）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "elif [ \"$(git -C \"${BASE}/leak-out\" rev-list --all --count 2>/dev/null || echo 0)\" = \"0\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-acceptance.sh",
		"anchor": "if mkdir -p \"$ACCEPT_STATE_DIR\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-cn-init.sh",
		"anchor": "      echo \"  ✓ 存在：$TOKEN_FILE  权限=$(stat -f '%Lp' \"$TOKEN_FILE\" 2>/dev/null || stat -c '%a' \"$TOKEN_FILE\")  原始长度=${RAW} 字节 / 剥空白后=${CLN} 字节（**内容不显示** ✗）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-cn-init.sh",
		"anchor": "    T=$(read_token 2>/dev/null) || { echo \"  ✗ 无令牌 ⇒ **跳过 API 校验**：本项**未判**，不给结论（rc=2）—— 未判 ≠ 绿（GAP-20260927-133②）\" >&2; exit 2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-cn-init.sh",
		"anchor": "    if [ -n \"$BASE\" ]; then git -C \"$D\" merge-base --is-ancestor \"$BASE\" \"$NEW\" || { echo \"  ✗ 非快进 ⇒ 拒绝（只增铁律 ✓）\" >&2; exit 3; }; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "publish/mirror-cn-init.sh",
		"anchor": "cmd=\"${1:-}\"; shift || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "publish/mirror-cn.sh",
		"anchor": "REMOTE=\"${ZERG_CN_REMOTE:-}\"     # 例：git@gitee.com:A72109/zerg-swarm.git",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-cn.sh",
		"anchor": "[ -n \"$OUT\" ]    || { echo \"✗ 需要 --out（镜像产出目录；首次给新目录，之后复用同一目录做增量）\" >&2; exit 2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-cn.sh",
		"anchor": "[ -n \"$REMOTE\" ] || { echo \"✗ 需要 --remote 或环境变量 ZERG_CN_REMOTE（国内镜像仓地址）\" >&2; exit 2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-cn.sh",
		"anchor": "if [ -n \"$BASE\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "publish/mirror-public-lib.py",
		"anchor": "                        mr.git([\"update-index\", \"--force-remove\", \"--\", mp], env_extra=idx_env, check=False)",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "publish/mirror-public-lib.py",
		"anchor": "                mr.git([\"update-index\", \"--force-remove\", \"--\", mp], env_extra=idx_env, check=False)",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-public.sh",
		"anchor": "      \"${PUBLISH_AUDIT_FALLBACK}\" \"${reason}\" >> \"${PUBLISH_AUDIT_TRUE_MAIN}\" 2>/dev/null \\",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "publish/mirror-public.sh",
		"anchor": "    \"$ts\" \"$1\" \"$(hostname)\" \"$PUBLISH_AUDIT_BY\" \"$REPO\" \"${EXPECT_OLD:-}\" \"${MIRROR_TIP:-}\" \"${MIRROR_TREE:-}\" \"$2\" \"$4\" \"$3\" \"${OUT:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "publish/mirror-public.sh",
		"anchor": "    git push --force-with-lease=\"refs/heads/main:${EXPECT_OLD}\" origin main",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "publish/mirror-public.sh",
		"anchor": "    if [ -z \"${ZERG_MIRROR_NO_LOCK_REASON:-}\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-public.sh",
		"anchor": "    p=\"$(ps -o ppid= -p \"${p}\" 2>/dev/null | tr -d ' \\n')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-public.sh",
		"anchor": "  FWD_REPO=\"$(cd \"$REPO\" 2>/dev/null && pwd || printf '%s' \"$REPO\")\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-public.sh",
		"anchor": "  [ -n \"$ALLOW\" ] && FWD+=(--allow \"$ALLOW\")",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-public.sh",
		"anchor": "  [ -n \"$ALLOW\" ] && LIST_ARGS+=(--allow \"$ALLOW\")",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-public.sh",
		"anchor": "  [ -n \"$REMOTE\" ] || { echo \"需要 --remote 或 ZERG_PUBLIC_REMOTE\" >&2; exit 2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-public.sh",
		"anchor": "  [ -n \"$REPORT\" ] && FWD+=(--report \"$REPORT\")",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-public.sh",
		"anchor": "  holder=\"$(awk '/^pid:/{print $2; exit}' \"$(publish_instance_lock_path)\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-public.sh",
		"anchor": "  if ! mkdir -p \"$(dirname \"${lk}\")\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "publish/mirror-public.sh",
		"anchor": "  if [ -n \"${ZERG_MIRROR_NO_LOCK:-}\" ]; then                     # 逃生口：关闸必留理由",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "publish/mirror-public.sh",
		"anchor": "  if [ -z \"${MIRROR_TIP:-}\" ]; then return 0; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-public.sh",
		"anchor": "  if printf '%s\\n' \"$line\" >> \"$PUBLISH_AUDIT\" 2>/dev/null; then return 0; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "publish/mirror-public.sh",
		"anchor": "  local reason=\"${ZERG_PUBLISH_AUDIT_REASON:-}\" s ts",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "publish/mirror-public.sh",
		"anchor": "  local want=\"${ZERG_MIRROR_LOCK_HELD:-}\" holder",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-public.sh",
		"anchor": "  printf '%s\\n' \"$c\" >> \"$PUBLISH_AUDIT_FALLBACK\" 2>/dev/null \\",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "publish/mirror-public.sh",
		"anchor": "  publish_audit_append \"$(publish_audit_line \"publish_rejected\" \"$rc\" \"push/readback 未达预期（见上）\" \"${REMOTE_TIP:-}\")\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "publish/mirror-public.sh",
		"anchor": "REMOTE=\"${ZERG_PUBLIC_REMOTE:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-public.sh",
		"anchor": "[ -n \"$ALLOW\" ] && LIB_ARGS+=(--allow \"$ALLOW\")",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "publish/mirror-public.sh",
		"anchor": "[ -n \"$REPORT\" ] && LIB_ARGS+=(--report \"$REPORT\")",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "publish/mirror-public.sh",
		"anchor": "case \"$(cd \"$OUT\" 2>/dev/null && pwd || echo \"$OUT\")\" in",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/publish-preflight.sh",
		"anchor": "  BASE8D=\"$(git -C \"$FACE\" log -1 --format=%H -S \"const Version = \\\"$VER8D\\\"\" -- core/internal/version/version.go 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/publish-preflight.sh",
		"anchor": "VER8D=\"$(sed -n 's/^const Version = \"\\([^\"]*\\)\".*/\\1/p' \"$FACE/core/internal/version/version.go\" 2>/dev/null | head -1)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/publish-preflight.sh",
		"anchor": "VER=\"$(sed -n 's/^const Version = \"\\([^\"]*\\)\".*/\\1/p' \"$D/core/internal/version/version.go\" 2>/dev/null | head -1)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/publish-preflight.sh",
		"anchor": "if [ -n \"$VER8D\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/publish-public.sh",
		"anchor": "  [ -n \"$REMOTE\" ] || { echo \"需要 --remote 或 ZERG_PUBLIC_REMOTE\" >&2; exit 2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/publish-public.sh",
		"anchor": "  git remote add origin \"$REMOTE\" 2>/dev/null || git remote set-url origin \"$REMOTE\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/publish-public.sh",
		"anchor": "PRIV_HEAD=\"$(cd \"$REPO_ROOT\" && git rev-parse HEAD 2>/dev/null || echo unknown)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/publish-public.sh",
		"anchor": "REMOTE=\"${ZERG_PUBLIC_REMOTE:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/publish-public.sh",
		"anchor": "find \"$OUT\" -name '__pycache__' -type d -prune -exec rm -rf {} + 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/publish-public.sh",
		"anchor": "find \"$OUT\" -type f -name '*.[0-9][0-9][0-9][0-9][0-9][0-9][0-9]*' -delete 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/publish-public.sh",
		"anchor": "find \"$OUT\" -type f \\( -name '*.pyc' -o -name '*.pyo' -o -name '*.o' -o -name '*.a' -o -name '*.so' -o -name '*.dylib' -o -name '*.class' \\) -delete 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/publish-public.sh",
		"anchor": "while IFS= read -r _excl || [ -n \"$_excl\" ]; do",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/start-zerg-core.sh",
		"anchor": "    PID=\"$(launchctl print \"${DOMAIN}/${LABEL}\" 2>/dev/null | awk '/^\\tpid = /{print $3}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/start-zerg-core.sh",
		"anchor": "    for _p in $(zerg_core_pids); do kill \"$_p\" 2>/dev/null || true; done",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/start-zerg-core.sh",
		"anchor": "    launchctl bootout \"${DOMAIN}/${LABEL}\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/start-zerg-core.sh",
		"anchor": "  launchctl bootstrap \"${DOMAIN}\" \"$PLIST\" 2>/dev/null || launchctl load -w \"$PLIST\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/build/start-zerg-core.sh",
		"anchor": "  launchctl kickstart -k \"${DOMAIN}/${LABEL}\" >/dev/null 2>&1 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/start-zerg-core.sh",
		"anchor": "LABEL=\"${ZERG_LABEL:-$(\"$REPO_ROOT/bin/zerg\" gate check decl-label bin/zerg-core 2>/dev/null || true)}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/start-zerg-core.sh",
		"anchor": "[ -n \"$LABEL\" ] || { echo \"❌ 取不到主控 label：声明件缺 bin/zerg-core 行（scripts/svc/decl-label.sh）\" >&2; exit 2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/start-zerg-core.sh",
		"anchor": "chmod +x \"$REPO_ROOT/scripts/svc/zerg-core-daemon.sh\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/start-zerg-core.sh",
		"anchor": "if [ ! -f \"$PLIST\" ] || grep -q \"zerg-core-daemon.sh\" \"$PLIST\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/test-publish-parity.sh",
		"anchor": "             $(git -C \"${MIR}\" rev-list refs/heads/main) 2>/dev/null | wc -l | tr -d ' ')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/test-publish-parity.sh",
		"anchor": "REMOTES=\"$(git -C \"${OLD}\" remote 2>/dev/null | wc -l | tr -d ' ')$(git -C \"${MIR}\" remote 2>/dev/null | wc -l | tr -d ' ')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/test-publish-parity.sh",
		"anchor": "elif grep -q 'ZERG_MIRROR_ALLOW_PUSH=1' \"${BASE}/mirror.log\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/test-publish-parity.sh",
		"anchor": "if grep -qE '^已推送|已推送（非 force）' \"${BASE}/old.log\" \"${BASE}/mirror.log\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "          [ -n \"$n_name\" ] || continue",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "        \"export PATH=\\\"\\$HOME/.local/go-sdk/go/bin:\\$HOME/go-sdk/go/bin:/usr/local/go/bin:\\$PATH\\\"; test -d '$root/.git' && echo git-ok; test -f '$root/scripts/build/zerg-upgrade.sh' && echo kernel-ok; test -x '$px/zerg-core' && echo cli-ok; test -d '$relay/objects' && echo relay-ok; (command -v go >/dev/null && echo go-ok) 2>/dev/null\" 2>&1 || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "        [ -n \"$FLEET_REMOTE\" ] && cmd=\"$cmd ZERG_UPDATE_REMOTE='$FLEET_REMOTE'\" && src_remote=\"$FLEET_REMOTE\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "        say \"  ❌ ${n_name}  code_sha=$( [ -n \"$sha\" ] && printf '%s' \"$sha\" | cut -c1-12 || echo '<未上报>' )（≠ 目标）  ·  内核回执：$(kr_of_node \"$n_name\")（最近一次内核运行留痕 · 非当前安装态）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      \"$REPO_ROOT/bin/zerg\" ui start >/dev/null 2>&1 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      CORE_LABEL=\"$(awk -F'\\t' '$1==\"launchd\" && $5==\"bin/zerg-core\" {print $2; exit}' \"${ZERG_SVCDECL_DECL:-$REPO_ROOT/deploy/服务声明.tsv}\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      TARGET_SHA=\"$(git -C \"$GITREPO\" ls-remote \"${FLEET_REMOTE:-origin}\" \"$FLEET_REF\" 2>/dev/null | awk 'NR==1{print $1}')\" || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      [ \"$FORCE\" = \"1\" ] && cmd=\"$cmd --force\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      [ \"$FORCE\" = \"1\" ] && lcmd=\"$lcmd --force\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      [ -n \"$n_name\" ] || continue",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      [ -n \"$tgt\" ] && [ \"$tgt\" != \"-\" ] || { printf '%s\\t%s\\t%s\\t%s\\t%s\\t%s\\t%s\\t%s\\t%s\\t%s\\n' \"$nm\" \"-\" \"pending-manual\" \"名册缺 ssh 目标\" \"-\" \"-\" \"-\" \"-\" \"-\" \"-\" >> \"$NODES_TSV\"; return 0; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      awk -F'\\t' -v n=\"$1\" 'NF>=6 && $1==n {f=1; v=(($6==\"\"||$6==\"-\") ? \"（无）\" : $6); exit} END{print f ? v : \"（无）\"}' \"$NODES_TSV\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      bash \"$STARTUI\" >/dev/null 2>&1 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      curl_tok -s -m 5 \"$API/api/fleet/status\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      if [ -n \"$AGENTD_LABEL\" ] && launchctl print \"$AGENTD_DOMAIN/$AGENTD_LABEL\" >/dev/null 2>&1; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      if [ -n \"$CORE_LABEL\" ] && launchctl print \"gui/$UID/$CORE_LABEL\" >/dev/null 2>&1; then launchctl kickstart -k \"gui/$UID/$CORE_LABEL\" >/dev/null 2>&1 || rfail=\"core\"; else rfail=\"core\"; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      if [ -n \"$lr_sha\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      if [ -n \"$missing\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      if [ -n \"$oprog\" ] && [ \"$oprog\" != \"$PREFIX/zerg-agentd\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      if [ -n \"$src_remote\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      kr=\"$(\"$FLEET_SSH\" -o BatchMode=yes -o ConnectTimeout=\"$FLEET_TIMEOUT\" \"$tgt\" \"cat $rc_dir/latest.json 2>/dev/null\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      kr_now=\"$(printf '%s' \"$MATRIX_BEFORE\" | matrix_sha \"$nm\")\"; [ -n \"$kr_now\" ] || kr_now=\"<未上报>\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      lr=\"$(\"$FLEET_SSH\" -o BatchMode=yes -o ConnectTimeout=\"$FLEET_TIMEOUT\" \"$tgt\" \"cat $rc_dir/latest-update-launch.json 2>/dev/null\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      oprog=\"$(launchctl print \"$AGENTD_DOMAIN/$AGENTD_LABEL\" 2>/dev/null | awk '$1==\"program\" {print $3; exit}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F10-SKIP-ENV", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      say \"⚠️  ZERG_FLEET_SKIP_MATRIX=1：跳过矩阵核对（口径缺失，回执会如实标注）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F10-SKIP-ENV", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "      say \"⚠️  机群执行完毕，但**矩阵核对被跳过**（ZERG_FLEET_SKIP_MATRIX=1）——未经核对，不算验收\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    --force) FORCE=1; shift ;;",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    FLEET_REMOTE=\"${ZERG_UPDATE_REMOTE:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    NODE_COUNT=\"$(printf '%s\\n' \"$ROSTER\" | grep -c . || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    [ \"$TARGET_OS\" = \"darwin\" ] && codesign -s - --force \"$PREFIX/$name\" >/dev/null 2>&1 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    [ -f \"$RECEIPTS/latest-update-launch.json\" ] && llr=\"$(cat \"$RECEIPTS/latest-update-launch.json\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    [ -n \"$AGENTD_LABEL\" ] || die \"节点 agentd 的 launchd label 取不到（① env ZERG_AGENTD_LABEL ② deploy/服务声明.tsv ③ launchd 实况——三档全空）⇒ 拒执，未动任何文件\" 4",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    [ -n \"$ROSTER\" ] || die \"机群名册为空：设 ZERG_FLEET_NODES，或在 gateway/fleet.yaml 写 update.nodes\" 1",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    [ -n \"$TARGET_SHA\" ] || die \"读不到目标提交（离线 / 无远端 ${FLEET_REMOTE:-origin} / 非法 ref ${FLEET_REF}）——机群升级必须知道目标；--tag <sha> 可显式指定\" 1",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    [ -n \"$c\" ] || continue",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    [ -n \"$d\" ] || die \"未提供 git 树构建产物：请先跑 \\`zerg update\\`（它 fetch→构建→交本内核），或加 --from-assets 走应急资产通道\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    [ -n \"$via_decl\" ] && { printf '%s' \"$via_decl\"; return 0; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    bash \"$STARTCORE\" --stop >/dev/null 2>&1 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    bash \"$STARTCORE\" >/dev/null 2>&1 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    case \"$(launchctl print \"gui/$(id -u)/$lbl\" 2>/dev/null | awk '$1==\"program\" {print $3; exit}')\" in",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    codesign -s - --force \"$PREFIX/$name\" >/dev/null 2>&1 || say \"   ⚠️ $name 重签名失败（未签名二进制会被秒杀）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    cp -p \"$FRC\" \"$RECEIPTS/fleet-receipt-$FTS.json\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    cp -p \"$FRC\" \"$RECEIPTS/latest-fleet.json\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    elif [ -n \"$START_AGENTD\" ]; then bash \"$START_AGENTD\" restart >/dev/null 2>&1 || rfail=\"agentd\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    gh release download ${TAG:+\"$TAG\"} --repo \"${ZERG_ASSETS_REPO:-Mr2109/zerg-swarm}\" --pattern manifest.json --dir \"$work\" --clobber 2>/dev/null \\",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    if [ \"$TARGET_OS\" = \"darwin\" ]; then for name in $restored; do codesign -s - --force \"$PREFIX/$name\" >/dev/null 2>&1 || true; codesign --verify \"$PREFIX/$name\" >/dev/null 2>&1 || vfail=\"$vfail $name\"; done; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    if [ -n \"$TAG\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    if [ -n \"$UI_LABEL\" ]; then say \"⏸  停 UI（按 label ${UI_LABEL} bootout · 再逐 PID 收残）\"; else say \"⏸  停 UI（无 label，逐 PID 点名）\"; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    if [ -n \"$now\" ] && [ \"$now\" != \"0\" ] && [ \"$now\" != \"$pid\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    if [ -n \"${ZERG_FLEET_RELAY_PATH:-}\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    if [ -n \"${ZERG_START_UI:-}\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    if manages_core; then bash \"$STARTCORE\" >/dev/null 2>&1 || true; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    launchctl bootout \"gui/$(id -u)/$UI_LABEL\" >/dev/null 2>&1 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    launchctl print \"$AGENTD_DOMAIN/$AGENTD_LABEL\" 2>/dev/null | awk '$1==\"pid\" {print $3; exit}'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    ls -1t \"$RECEIPTS\"/*.json 2>/dev/null | head -5 | while read -r f; do say \"  $(basename \"$f\")\"; done",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    mkdir -p \"$RECEIPTS\"; RTS=\"$(date -u +%Y%m%dT%H%M%SZ)\"; n=0; vfail=\"\"; rfail=\"\"; restored=\"\"; for name in zerg-core zerg-agent zerg-agentd zerg-ui; do [ -f \"$PREFIX/$name.prev\" ] || continue; cp -p \"$PREFIX/$name\" \"$PREFIX/$name.failed\" 2>/dev/null || true; mv -f \"$PREFIX/$name.prev\" \"$PREFIX/$name\"; restored=\"$restored $name\"; n=$((n+1)); say \"↩️  换回 $name\"; done",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    mv -f \"$PREFIX/$name\" \"$PREFIX/$name.failed\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    printf '%s' \"$MATRIX_FINAL\" | python3 - \"$TARGET_SHA\" \"$(printf '%s\\n' \"$ROSTER\" | cut -f1 | tr '\\n' ' ')\" <<'PY' 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    printf 'X-Auth-Token: %s\\n' \"$(cat \"$TOKEN_FILE\" 2>/dev/null || true)\" | curl -m 30 -H @- \"$@\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "    via_decl=\"$(\"$REPO_ROOT/bin/zerg\" gate check decl-label bin/zerg-agentd 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  \"$1\" --version 2>/dev/null | head -1 || echo \"无法自报\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  UI_LABEL=\"$(\"$REPO_ROOT/bin/zerg\" gate check decl-label \"$UI_PATTERN\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  [ -n \"$AGENTD_LABEL\" ] && { printf '%s' \"$AGENTD_LABEL\"; return 0; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  [ -n \"$NEED\" ] && say \"→ 有新版：${SRC_TAG}\" || say \"→ 已是最新：${SRC_TAG}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  [ -n \"$NEED\" ] || say \"  ⚠️ 看起来已是最新（仍可强制换装）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  [ -n \"$out\" ] || die \"组件集为空（--components '$raw'）\" 4",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  [ -n \"$out\" ] || { echo 0; return; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  [ -n \"$raw\" ] || return 0",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  elif git -C \"$GITREPO\" merge-base --is-ancestor \"$CMP_SHA\" \"$SRC_SHA_CMP\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  for _p in $(ui_pids); do kill \"$_p\" 2>/dev/null || true; done",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  for lbl in $(launchctl list 2>/dev/null | awk 'NR>1 {print $3}' | grep -E '^com\\.zerg\\.agent' || true); do",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  if [ \"$(want_sha \"$_a\" 2>/dev/null || echo '-')\" = \"-\" ]; then MISSING=\"$MISSING $_a\"; else WANT=\"$WANT $_a\"; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  if [ -n \"$AGENTD_LABEL\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  if [ -n \"$START_AGENTD\" ]; then bash \"$START_AGENTD\" is-active >/dev/null 2>&1; return $?; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  if [ -n \"$START_AGENTD\" ]; then bash \"$START_AGENTD\" restart || return 1; else",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  if [ -n \"$START_AGENTD\" ]; then bash \"$START_AGENTD\" stop; return $?; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  if [ -n \"$UI_LABEL\" ] && command -v launchctl >/dev/null 2>&1 \\",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  if [ -n \"$UI_LABEL\" ] && command -v launchctl >/dev/null 2>&1; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  if [ -n \"$raw\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  if [ -n \"${UNMANAGED:-}\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  if git -C \"$GITREPO\" merge-base --is-ancestor \"$SRC_SHA_CMP\" \"$CMP_SHA\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  kill -TERM \"$pid\" 2>/dev/null || return 1",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  out=\"$(curl_tok -s -m 5 \"$API/api/tasks\" 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  raw=\"$(curl_tok -s -m 5 \"$API/api/capabilities\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  say \"            子端 $( [ -n \"$CUR_AGENT\" ] && echo \"$CUR_AGENT\" || echo '未安装' )\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  say \"            守护 $( [ -n \"$CUR_AGENTD\" ] && echo \"$CUR_AGENTD\" || echo '未安装' )\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  say \"  在途任务 : ${INFLIGHT} 个$( [ \"$INFLIGHT\" != \"0\" ] && echo '（需 --force 或等待）' )\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  say \"  当前版本 : 主控 $( [ -n \"$CUR_CORE\" ] && echo \"$CUR_CORE\" || echo '未安装' )\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  say \"⛔ 有 ${INFLIGHT} 个在途任务——拒绝升级（等它收尾，或 --force）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "  systemctl show -p MainPID --value \"$AGENTD_UNIT\" 2>/dev/null | tr -d '[:space:]'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "' 2>/dev/null || echo 0",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "' 2>/dev/null || say \"  （矩阵读取失败）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "AGENTD_LABEL=\"${ZERG_AGENTD_LABEL:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "LOCAL_HEAD=\"$(git -C \"$GITREPO\" rev-parse --short HEAD 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "SOURCE=\"${ZERG_UPGRADE_SOURCE:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "START_AGENTD=\"${ZERG_START_AGENTD:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "TARGET_UI_SHA=\"$(want_sha zerg-ui-$PLAT 2>/dev/null || echo '-')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "UI_BIN=\"${ZERG_UI_BIN:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "UI_LABEL=\"${ZERG_UI_LABEL:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "[ -n \"$CUR_CORE\" ] && case \"$CUR_CORE\" in *\"$SRC_SHA\"*) ;; *) NEED=\"yes\" ;; esac",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "cp -p \"$RC\" \"$RECEIPTS/latest.json\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "if [ -n \"$CMP_SHA\" ] && [ \"$CMP_SHA\" != \"unknown\" ] && [ \"$CMP_SHA\" != \"$SRC_SHA_CMP\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "if [ -n \"$MISSING\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "if [ -n \"${ZERG_UPGRADE_PLAT:-}\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "if [ -z \"$CMP_SHA\" ] || ! git -C \"$GITREPO\" cat-file -e \"${SRC_SHA}^{commit}\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "ls -1t \"$RECEIPTS\"/*.json 2>/dev/null | tail -n +21 | xargs -r rm -f    # 只留最近 20 份",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "print(\"%s|%s|%s\" % (t.get(\"pin\",\"\"), t.get(\"actual\",\"\"), t.get(\"policy\",\"\")))' 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "print(\"%s→%s\" % (t.get(\"pin\",\"?\"), t.get(\"actual\",\"?\")))' 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "print((d.get(\"target\") or {}).get(\"commit\",\"\"))' 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "print((d.get(\"verify\") or {}).get(\"live_code_sha\") or \"\")' 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "print(d.get(\"build_time\") or \"\")' 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "print(d.get(\"code_sha\") or \"\")' 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "print(d.get(\"failed_at\") or \"\")' 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "print(d.get(\"result\",\"\"))' 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/build/zerg-upgrade.sh",
		"anchor": "python3 - \"$RC\" \"$TS\" \"$RESULT\" \"$FAILED_AT\" \"$PREFIX\" \"$PLAT\" \"$SOURCE\" \"$SRC_TAG\" \"$SRC_SHA_CMP\" \"$CUR_CORE\" \"$CUR_AGENT\" \"$INFLIGHT\" \"$(ver_of \"$PREFIX/zerg-core\")\" \"$(ver_of \"$PREFIX/zerg-agent\")\" \"$SRC_KIND\" \"${UNMANAGED:-}\" \"$LIVE_FOR_RECEIPT\" \"$ROLE\" \"$COMP_LIST\" \"$work/manifest.json\" \"$(ver_of \"$PREFIX/zerg-agentd\")\" <<'PY'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "              \"$(command -v llama-server 2>/dev/null)\" \\",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "    [ -n \"$cand\" ] && [ -x \"$cand\" ] && { ENG=\"$cand\"; break; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "    c=$(curl -s -o /dev/null -w '%{http_code}' -m 2 \"http://127.0.0.1:$PORT/health\" 2>/dev/null)",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "    echo \"  生成未通过（HTTP=${GEN_CODE:-?} tokens=${n:-0}）⇒ **该轮无效、不计入**；正文前 200 字：$(head -c 200 \"$GEN_JSON\" 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "    kill -0 \"$pid\" 2>/dev/null || break",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "    pkill -f \"llama-server.*--port $PORT\" 2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "  for cand in \"${CALIB_ENGINE:-}\" \\",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "  ioreg -rd1 -c IOAccelerator 2>/dev/null | tr ',' '\\n' \\",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "  pkill -f \"llama-server.*--port $PORT\" 2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "  rss_kb=$(ps -o rss= -p \"$pid\" 2>/dev/null | tr -d ' ')",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "  w0=$(gpu_wired_bytes); [ -n \"${w0:-}\" ] || w0=0   # 本轮基线（在起引擎之前）",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "  w1=$(gpu_wired_bytes); [ -n \"${w1:-}\" ] || w1=0",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "  w2=$(gpu_wired_bytes); [ -n \"${w2:-}\" ] || w2=0",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "MACHINE=${MACHINE:-$(hostname -s 2>/dev/null)}",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "[ -n \"${MACHINE:-}\" ] || { echo \"!! 探测不到机器名（hostname -s 空）⇒ 请设 MACHINE=<名> 显式指定\"; exit 2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-local-model.sh",
		"anchor": "pkill -f \"llama-server.*--port $PORT\" 2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-x3-model.sh",
		"anchor": "c=$(curl -s -o /dev/null -w '%{http_code}' -m 2 \"http://127.0.0.1:$PORT$READY_PATH\" 2>/dev/null)",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-x3-model.sh",
		"anchor": "    kill -0 \"$pid\" 2>/dev/null || break",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-x3-model.sh",
		"anchor": "pkill -f \"$KILL_PAT\" 2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-x3-model.sh",
		"anchor": "  rocm-smi --showmeminfo gtt 2>/dev/null | awk '/GTT Total Used/ {print $NF; exit}'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-x3-model.sh",
		"anchor": "  rss_kb=$(ps -o rss= -p \"$pid\" 2>/dev/null | tr -d ' ')",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/calib/calib-x3-model.sh",
		"anchor": "G0=$(gtt_used); [ -n \"${G0:-}\" ] || { echo \"!! 读不到 GTT 读数，不敢编数字 ⇒ 退出\"; exit 3; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/calib/calib-x3-model.sh",
		"anchor": "if [ -n \"${_g:-}\" ] && [ \"$(( _g - G0 ))\" -gt \"$warm_max_b\" ]; then warm_max_b=$(( _g - G0 )); fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/calib/calib-x3-model.sh",
		"anchor": "if ! ss -ltnp 2>/dev/null | grep -F \"127.0.0.1:$PORT\" | grep -q \"pid=$pid,\"; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/calib/calib-x3-model.sh",
		"anchor": "if [ -n \"$CACHE_GB\" ]; then echo \"cache_gb: $CACHE_GB\"; fi   # 缺口 GAP-20260926-19",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/evals/_archive/v25_benchmark.py",
		"anchor": "        f'NO_PROXY=\"127.0.0.1,localhost\" codex exec --skip-git-repo-check \"{task}\" 2>&1',",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/accept-no-probe-selftest.sh",
		"anchor": "GO_BIN=$(command -v go 2>/dev/null || true)",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/accept-no-probe.sh",
		"anchor": "    _out=$(LC_ALL=C grep -c -E -e \"${_p}\" \"${_f}\" 2>/dev/null)",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/accept-no-probe.sh",
		"anchor": "    _out=$(LC_ALL=C grep -c -F -e \"${_p}\" \"${_f}\" 2>/dev/null)",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/accept-no-probe.sh",
		"anchor": "  LC_ALL=C strings \"${BIN}\" >\"${STRINGS_DUMP}\" 2>/dev/null || {",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/accept-no-probe.sh",
		"anchor": "GO_BIN=$(command -v go 2>/dev/null || true)",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/accept-no-probe.sh",
		"anchor": "SHA=$(LC_ALL=C shasum -a 256 \"${BIN}\" 2>/dev/null | awk '{print $1}')",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/accept-no-probe.sh",
		"anchor": "if ! LC_ALL=C strings -a \"${BIN}\" >\"${STRINGS_DUMP}\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/all.sh",
		"anchor": "          done | sort -u > \"${CHANGED_LIST}\" || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/all.sh",
		"anchor": "        git -C \"${REPO_ROOT}\" -c core.quotepath=false diff --name-only --cached           2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/all.sh",
		"anchor": "        git -C \"${REPO_ROOT}\" -c core.quotepath=false ls-files --others --exclude-standard 2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/all.sh",
		"anchor": "      { git -C \"${REPO_ROOT}\" -c core.quotepath=false diff --name-only                    2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/all.sh",
		"anchor": "    ( cd \"${REPO_ROOT}\" && list_steps 2>/dev/null | sed -n 's/^ *[a-z][a-z-]*[[:space:]]\\{1,\\}[a-z][a-z-]*[[:space:]]\\{1,\\}//p' )",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/all.sh",
		"anchor": "    _p=\"$( cd \"${REPO_ROOT}\" && list_steps_plain 2>/dev/null )\"; _prc=$?",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/all.sh",
		"anchor": "    four_out=\"$(cd \"${REPO_ROOT}\" && ./bin/zerg gate check check-cli-contract 2>&1)\" || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/all.sh",
		"anchor": "    four_out=\"$(cd \"${REPO_ROOT}\" && to_wrap ./bin/zerg gate check check-cli-contract 2>&1)\" || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/all.sh",
		"anchor": "    if [ \"${r}\" -ne 0 ] && grep -q 'unrecognized arguments' \"${logf}\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/all.sh",
		"anchor": "    tail -5 \"${TMPLOG}/real-gates.err\" >&2 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/all.sh",
		"anchor": "  four_line=\"$(printf '%s\\n' \"${four_out}\" | grep -m1 '命令树' || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/all.sh",
		"anchor": "  read -r RCREAL_TOTAL REAL_META_RAN REAL_META_RED REAL_META_FAKE < \"${REALMETA}\" || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/all.sh",
		"anchor": "  rs0=\"$(perl -MTime::HiRes=time -e 'printf \"%.3f\\n\", time' 2>/dev/null || date +%s)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/all.sh",
		"anchor": "  rs1=\"$(perl -MTime::HiRes=time -e 'printf \"%.3f\\n\", time' 2>/dev/null || date +%s)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/all.sh",
		"anchor": "HAVE_PERL=0; [ -n \"$(command -v perl || true)\" ] && HAVE_PERL=1",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/all.sh",
		"anchor": "PY=\"$(command -v python3 || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/all.sh",
		"anchor": "_now() { perl -MTime::HiRes=time -e 'printf \"%.3f\\n\", time' 2>/dev/null || date +%s; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/all.sh",
		"anchor": "if [ \"${UNITTEST}\" = \"1\" ] && [ -n \"${ZERG_ALLSH_GATES:-}\" ]; then GATES=\"${ZERG_ALLSH_GATES}\"; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/check-agentd-unit-default.sh",
		"anchor": "    printf '%s\\n' '           AGENTD_UNIT=\"${ZERG_SETUP_AGENTD_UNIT:-}\"   # 缺省留空，参数面解析后按真源推导'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/check-agentd-unit-default.sh",
		"anchor": "    printf '%s\\n' '           [ -n \"$AGENTD_UNIT\" ] || AGENTD_UNIT=\"${NODE_NAME}-agent\"'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/check-agentd-unit-default.sh",
		"anchor": "    printf '%s\\n' '   修法  = NODE_NAME=\"${ZERG_SETUP_NODE_NAME:-$(hostname -s 2>/dev/null || hostname 2>/dev/null || echo node)}\"'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/check-agentd-unit-default.sh",
		"anchor": "    printf '%s\\n' '   修法  = 加一行： [ -n \"$AGENTD_LABEL\" ] || AGENTD_LABEL=\"com.zerg.agent-${NODE_NAME}\"'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/check-agentd-unit-default.sh",
		"anchor": "    printf '%s\\n' 'AGENTD_LABEL=\"${ZERG_SETUP_AGENTD_LABEL:-}\"'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/check-agentd-unit-default.sh",
		"anchor": "    printf '%s\\n' 'AGENTD_UNIT=\"${ZERG_SETUP_AGENTD_UNIT:-}\"'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/check-agentd-unit-default.sh",
		"anchor": "    printf '%s\\n' 'NODE_NAME=\"${ZERG_SETUP_NODE_NAME:-$(hostname -s 2>/dev/null || echo node)}\"'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/check-agentd-unit-default.sh",
		"anchor": "    printf '%s\\n' '[ -n \"$AGENTD_LABEL\" ] || AGENTD_LABEL=\"com.zerg.agent-${NODE_NAME}\"'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/check-agentd-unit-default.sh",
		"anchor": "    printf '%s\\n' '[ -n \"$AGENTD_UNIT\" ]  || AGENTD_UNIT=\"${NODE_NAME}-agent\"'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/check-gate-bootstrap.sh",
		"anchor": "    hits=\"$(grep -cE '(^|[^A-Za-z0-9_/-])zerg[[:space:]]' \"${hook}\" || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/check-gate-bootstrap.sh",
		"anchor": "  n=\"$(printf '%s\\n' \"${l1}\" | grep -c . || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "    hit=\"$(git_ro ls-tree -r --name-only \"$full\" -- scripts 2>/dev/null | grep -E \"^scripts/.*/${name}\\.(py|sh)$\" | head -n 1 || printf '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/judge-targets.sh",
		"anchor": "    if [ -n \"$hit\" ]; then M_PIECE=\"$hit\"; M_KIND=\"script\"; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  [ -n \"$M_TDIR\" ] && [ -n \"$M_NAME\" ] || return 1",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  [ -n \"$full\" ] || { warn \"提交不存在：$commit\"; return 2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  [ -n \"$rundir\" ] || { warn \"mktemp 失败\"; return 1; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  [ -n \"$scratch\" ] || { warn \"mktemp 失败\"; return 1; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  [ -n \"$tdir\" ] || { warn \"靶未建：先跑 $PROG build $name $commit\"; return 2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  full=\"$(git_ro rev-parse --verify --quiet \"${commit}^{commit}\" 2>/dev/null || printf '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  ids=\"$(grep -F -- \"$name\" \"$led\" 2>/dev/null | grep -o '\"id\": *\"[^\"]*\"' | grep -o 'GAP-[0-9][0-9]*-[0-9][0-9]*' | sort -u | paste -sd, -)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  if ZERG_TARGET_ROOT=\"$scratch\" cmd_build \"$KNOWN_JUDGE\" \"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef\" --quiet 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  if ZERG_TARGET_ROOT=\"$scratch\" cmd_run no-such-judge-xyz 785bf66e -- zerg gate check no-such-judge-xyz --self-test 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  if [ -n \"$(git_ro status --porcelain -- \"$M_PIECE\" 2>/dev/null || printf '')\" ]; then M_DIRTY=1; else M_DIRTY=0; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  if [ -n \"$ids\" ]; then printf '%s' \"$ids\"; else printf '%s' \"-\"; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  if [ -n \"$n\" ]; then write_manifest \"$n\"; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  if git_ro cat-file -e \"${full}:${gofile}\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  local pre post; pre=\"$(command -v zerg 2>/dev/null || printf '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  post=\"$(command -v zerg 2>/dev/null || printf '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  rundir=\"$(mktemp -d \"${TMPDIR:-/var/tmp}/zerg-target-run.XXXXXX\" 2>/dev/null || printf '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/judge-targets.sh",
		"anchor": "  scratch=\"$(mktemp -d \"${TMPDIR:-/var/tmp}/zerg-target-selftest.XXXXXX\" 2>/dev/null || printf '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/mutation-smoke.sh",
		"anchor": "      cp -p \"$SANDBOX/$(basename \"${MUT_FILE[$i]}\").$i\" \"$SANDBOX_WORK/${MUT_FILE[$i]}\" 2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/mutation-smoke.sh",
		"anchor": "  repo_after=\"$(shasum -a 256 \"$repo_f\" 2>/dev/null | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/mutation-smoke.sh",
		"anchor": "  repo_before=\"$(shasum -a 256 \"$repo_f\" 2>/dev/null | awk '{print $1}')\"   # 仓内前像（**只读**）",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/mutation-smoke.sh",
		"anchor": "ORIG_GOCACHE=\"$(go env GOCACHE 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/mutation-smoke.sh",
		"anchor": "ORIG_GOMODCACHE=\"$(go env GOMODCACHE 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/mutation-smoke.sh",
		"anchor": "[ -n \"$ORIG_GOCACHE\" ] && export GOCACHE=\"$ORIG_GOCACHE\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/mutation-smoke.sh",
		"anchor": "[ -n \"$ORIG_GOMODCACHE\" ] && export GOMODCACHE=\"$ORIG_GOMODCACHE\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/overlay-guard.sh",
		"anchor": "  ref_sha=\"$(shasum -a 256 \"$ref\" 2>/dev/null | awk '{print $1}' || sha256sum \"$ref\" | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/overlay-guard.sh",
		"anchor": "[ -n \"$out\" ] || out=\"$(mktemp -t overlay-out-XXXXXX)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/overlay-guard.sh",
		"anchor": "[ -n \"$ov\" ] || die2 \"缺 --check/--overlay <overlay.json>\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/overlay-guard.sh",
		"anchor": "if [ -n \"$ref\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/overlay-guard.sh",
		"anchor": "new_sha=\"$(shasum -a 256 \"$out\" 2>/dev/null | awk '{print $1}' || sha256sum \"$out\" | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "        B8_ROSTER_CMD=\"zerg doctor --json name,verdict,detail 2>/dev/null | python3 -c \\\"import json,sys",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "        add_step gates \"门82 hooksPath 挂载链：配置→解析路径→件→执行位（毫秒件 · 阻断）\" tri \"${REPO_ROOT}\" \"hp=\\$(git config --local core.hooksPath 2>/dev/null); if [ -z \\$hp ]; then echo '✗ hooksPath 挂载链：本仓 core.hooksPath 未设（挂载被清空 / 指歪）⇒ 红'; exit 1; fi; gp=\\$(git rev-parse --git-path hooks/pre-commit 2>/dev/null); ok=0; case \\$gp in .githooks/pre-commit) ok=1 ;; *) case \\$gp in */.githooks/pre-commit) ok=1 ;; esac ;; esac; if [ \\$ok -eq 0 ]; then echo '✗ hooksPath 挂载链：解析路径不是 .githooks/pre-commit（配置指歪 / 挂到别处）⇒ 红'; exit 1; fi; if [ ! -f .githooks/pre-commit ]; then echo '✗ hooksPath 挂载链：.githooks/pre-commit 不在 ⇒ 红'; exit 1; fi; if [ ! -x .githooks/pre-commit ]; then echo '✗ hooksPath 挂载链：提交钩子无执行位 ⇒ 红'; exit 1; fi; m=\\$(git ls-files -s -- .githooks/pre-commit 2>/dev/null); m=\\${m%%[[:space:]]*}; if [ -z \\$m ]; then echo '✗ hooksPath 挂载链：index 里读不到该件 ⇒ 不给结论'; exit 2; fi; if [ \\$m != 100755 ]; then echo '✗ hooksPath 挂载链：index 位不是 100755 ⇒ 红'; exit 1; fi; echo '✓ hooksPath 挂载链通：配置→.githooks/pre-commit（文件 + 执行位 + index 100755）'; exit 0\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "        add_step gates \"门⑫ 文档-命令一致性：docs 阻断 + devdocs 棘轮（M12·D3）\" tri \"${REPO_ROOT}\" \"ZERG_DEVDOCS_ROOT=\\\"${ZERG_DEVDOCS_ROOT:-}\\\" zerg gate check check-doc-cmds\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "        add_step release \"release: B4 标签名与版本真源匹配（阻断）\" tri \"${REPO_ROOT}\" \"zerg gate check check_version || exit 1; V=\\$(grep -m1 '^const Version = ' core/internal/version/version.go | sed 's/.*\\\"\\\\(.*\\\\)\\\".*/\\\\1/'); if [ \\\"\\${GITHUB_REF_TYPE:-}\\\" = \\\"tag\\\" ]; then [ \\\"\\${GITHUB_REF_NAME}\\\" = \\\"v\\${V}\\\" ] || { echo \\\"✗ 标签 \\${GITHUB_REF_NAME} ≠ v\\${V}\\\"; exit 1; }; echo \\\"标签面照判：\\${GITHUB_REF_NAME} == v\\${V}\\\"; else echo \\\"（不在 tag 上 ⇒ 标签面不进判据；CI 里 GITHUB_REF_TYPE=tag 时照判）版本面 v\\${V} 一致\\\"; fi\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "      \"$(cut -f2 \"${t}/to.timeouts.tsv\" 2>/dev/null)\" \"超时（限时 2s）· 判不了\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "      \"${slug}\" \"${REPO_ROOT}\" \"${ZERG_DEVDOCS_ROOT:-<默认>}\" \"${ZERG_GATE_STATE_DIR}\" \"${_ZERG_GATE_STATE_EXPLICIT}\" \"${ZERG_STATE_DIR:-<默认>}\" \"${ZERG_PUBLIC_TREE:-<默认>}\" \"${TMPDIR:-<默认>}\"; } >>\"${outdir}/_rootfp.log\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "    \"$(bash \"${BASH_SOURCE[0]}\" --help 2>/dev/null | grep -c -- '--candidate')\" \"1\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "    \"$(bash \"${BASH_SOURCE[0]}\" --help 2>/dev/null | grep -c -- '--fast')\" \"2\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "    >>\"${dir}/no-lock-audit.jsonl\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "    _lk_holder=\"$(awk '/^pid:/{print $2; exit}' \"$(gate_instance_lock_path)\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "    if [ -n \"${jname}\" ] && { grep -qE \"^[[:space:]]*\\\"${jname}\\\"[[:space:]]*:\" core/cmd/zerg/gate_inproc_table.go 2>/dev/null || [ -f \"core/cmd/zerg/${jname//-/_}.go\" ]; }; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "    p=\"$(ps -o ppid= -p \"${p}\" 2>/dev/null | tr -d ' \\n')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "    unset ZERG_STATE_DIR 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  add_step self \"自检-封口探针\" rc \"${t}\" \"if [ -n \\\"\\${ZERG_SVCDECL_DECL:-}\\\" ]; then echo LEAK; exit 1; fi; echo SEALED\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  d=\"$( cd \"${root}\" && env -u GIT_INDEX_FILE -u GIT_DIR -u GIT_WORK_TREE git -c core.quotepath=false diff --name-only HEAD -- scripts/gates 2>/dev/null )\" || return 2",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  env -u GIT_INDEX_FILE -u GIT_DIR -u GIT_WORK_TREE git -C \"${gf}\" checkout -q -- scripts/gates/a.sh 2>/dev/null   # 先把③改过的件还原，否则④判的是③的残留",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  filter_fast || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  for d in $(ls -dt \"${root}\"/zerg-mirror-* \"${root}\"/zerg-public-* \"${root}\"/zerg-pub-* 2>/dev/null); do",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  for v in \"${GATE_SEALED_ENV[@]}\"; do unset \"${v}\" 2>/dev/null || true; done",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  grep -c '^BLOCKED' \"$1\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  grep -c '^FAIL' \"$1\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  grep -c '^PASS' \"$1\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  grep -c '^REPORT' \"$1\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  if ! mkdir -p \"$(dirname \"${lk}\")\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  if [ -f \"${markf}\" ] && [ \"$(grep -c . \"${markf}\" 2>/dev/null || true)\" -gt 0 ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  if [ -f \"${results%.tsv}.timeouts.tsv\" ] && [ \"$(grep -c . \"${results%.tsv}.timeouts.tsv\" 2>/dev/null || true)\" -gt 0 ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  if [ -n \"${ZERG_GATE_NO_LOCK:-}\" ]; then gate_nolock_audit \"$@\" || return 2; return 0; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  if [ -n \"${ZERG_PUBLIC_TREE:-}\" ]; then printf '%s\\n' \"${ZERG_PUBLIC_TREE}\"; return 0; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  line=\"$(grep -m1 '✗' \"${log}\" 2>/dev/null | tr -d '\\r' | cut -c1-160)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  local n; n=\"$(printf '%s\\n' \"${roots}\" | grep -c . || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  local reason=\"${ZERG_GATE_NO_LOCK_REASON:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  mkdir -p \"${dir}\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  n=\"$( cd \"${root}\" && env -u GIT_INDEX_FILE -u GIT_DIR -u GIT_WORK_TREE git -c core.quotepath=false ls-files --others --exclude-standard -- scripts/gates 2>/dev/null )\" || return 2",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  printf '%s\\n' \"$(_fp(){ if [ -z \"${1:-}\" ]; then printf '%s' '=<默认>'; return; fi; if [ \"${1#/}\" = \"$1\" ]; then set -- \"${REPO_ROOT}/$1\"; fi; cd -- \"$1\" 2>/dev/null && pwd -P || printf '%s' \"$1\"; }; printf '── 根/旋钮指纹 · 仓根=%s · ZERG_DEVDOCS_ROOT=%s · ZERG_STATE_DIR=%s · ZERG_GATE_STATE_DIR=%s（显式=%s） · ZERG_DOCS_ROOT=%s · ZERG_PUBLIC_TREE=%s · TMPDIR=%s · LIST_ONLY=%s · GOFLAGS=%s · GOPROXY=%s ──\\n' \"${REPO_ROOT}\" \"$(_fp \"${ZERG_DEVDOCS_ROOT:-}\")\" \"$(_fp \"${ZERG_STATE_DIR:-}\")\" \"$(_fp \"${ZERG_GATE_STATE_DIR}\")\" \"${_ZERG_GATE_STATE_EXPLICIT}\" \"$(_fp \"${ZERG_DOCS_ROOT:-}\")\" \"$(_fp \"${ZERG_PUBLIC_TREE:-}\")\" \"$(_fp \"${TMPDIR:-}\")\" \"${LIST_ONLY}\" \"${GOFLAGS:-=<默认>}\" \"${GOPROXY:-=<默认>}\")\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  printf '◀ %s · rc=%s · 仓根=%s\\n' \"${slug}\" \"${rc}\" \"${REPO_ROOT}\" >>\"${outdir}/_rootfp.log\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  raw=\"$(ps -Ao pid=,ppid=,command= 2>/dev/null)\" || {",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "  roots=\"$(printf '%s\\n' \"${raw}\" | awk -v self=\"${self_pids} \" -v mytok=\"${ZERG_LANE_TOKEN:-}\" '",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "_ZERG_GATE_STATE_DEFAULT=\"${HOME:-}/.zerg/state\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/precommit-gates.sh",
		"anchor": "if [ -n \"${ZERG_GATE_STATE_DIR:-}\" ]; then _ZERG_GATE_STATE_EXPLICIT=1; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/probe-build-all-wall-wiring.sh",
		"anchor": "RES_REPO=\"$(find \"${REPO_ROOT}\" -type f \\( -name '*.patched' -o -name '*.orig' -o -name '*.rej' \\) -not -path '*/.git/*' 2>/dev/null | wc -l | tr -d ' ')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/probe-build-all-wall-wiring.sh",
		"anchor": "RES_TMP=\"$(find \"${TMP}\" -type f \\( -name '*.patched' -o -name '*.orig' -o -name '*.rej' \\) 2>/dev/null | wc -l | tr -d ' ')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F12-JUDGE-FOLD", "path": "scripts/gates/probe-build-all-wall-wiring.sh",
		"anchor": "[ -n \"${out}\" ] || exit 0",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "      noface) grep -q -e '--self-test' -e '--selftest' \"${GATEDIR}/${n}\" 2>/dev/null && sh_badclass+=(\"${n}（noface 档却有自检字面）\") ;;",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "      rss=\"$(cat \"${RESDIR}/self.${n}.tsv\" 2>/dev/null || echo NA)\"; [ -n \"${rss}\" ] || rss=NA",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "      self)   grep -q -e '--self-test' -e '--selftest' \"${GATEDIR}/${n}\" 2>/dev/null || sh_badclass+=(\"${n}（self 档却无自检字面）\") ;;",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "      wait \"${pids[0]}\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "    elif grep -q -- '--selftest' \"${f}\" 2>/dev/null; then flag=\"--selftest\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "    first=\"$(grep -m1 -v '^[[:space:]]*$' \"${logf}\" 2>/dev/null | LC_ALL=C head -c 400 | tr '\\t' ' ' | tr -d '\\r' || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "    if grep -q -- '--self-test' \"${f}\" 2>/dev/null; then flag=\"--self-test\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/gates/real-gates.sh",
		"anchor": "  for n in \"${SH_LIVE[@]:-}\"; do [ -n \"$n\" ] || continue; add_sh_gate \"$n\"; done",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "  for p in \"${pids[@]:-}\"; do [ -n \"${p}\" ] && wait \"${p}\" 2>/dev/null || true; done",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "  nreal=\"$(cat \"${RESDIR}/real.${n}.tsv\" 2>/dev/null || printf '\\t\\t（没跑出结果）')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "  nself=\"$(cat \"${RESDIR}/self.${n}.tsv\" 2>/dev/null || echo NA)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "  sh_class() { if grep -q -e '--self-test' -e '--selftest' \"${GATEDIR}/$1\" 2>/dev/null; then echo self; else echo noface; fi; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/real-gates.sh",
		"anchor": "HAVE_PERL=0; [ -n \"$(command -v perl || true)\" ] && HAVE_PERL=1",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/gates/real-gates.sh",
		"anchor": "PY=\"$(command -v python3 || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "_now() { perl -MTime::HiRes=time -e 'printf \"%.3f\\n\", time' 2>/dev/null || date +%s; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/real-gates.sh",
		"anchor": "reason_for() { printf '%s\\n' \"${SKIP_TABLE:-}\" | awk -F'\\t' -v n=\"$1\" '$1==n{print $2\": \"$3; exit}'; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/real-gates.sh",
		"anchor": "skip_names=\"$(awk -F'\\t' '$2==\"skip\" && $8 !~ /^sh-/{printf \"%s%s\", (c++?\",\":\"\"), $1}' \"${ALLROWS}\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/redaction-probes.sh",
		"anchor": "  echo \"git rev     : $(git -C \"${ROOT}\" rev-parse --short HEAD 2>/dev/null || echo '?')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "    ( cd \"${mod}\" && go list ./... 2>/dev/null | sort )",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "    ( cd \"${mod}\" && go test -list '.*' \"${_ipkg}\" ) 2>/dev/null | awk '/^Test/{print}' | sort > \"${_itests}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "    >>\"${dir}/no-lock-audit.jsonl\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "    _af_ip=\"$( cd \"${mod}\" && go list \"./${_af_d}\" 2>/dev/null )\" || _af_ip=\"\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "    _gc=\"$( ( cd \"${mod}\" && go env GOCACHE ) 2>/dev/null )\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "    _lk_holder=\"$(awk '/^pid:/{print $2; exit}' \"$(shard_instance_lock_path)\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "    cargo metadata --no-deps --format-version 1 2>/dev/null \\",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "    git -C \"${_af_root}\" -c core.quotepath=false diff --name-only --cached         2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "    git -C \"${_af_root}\" -c core.quotepath=false ls-files --others --exclude-standard 2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "    p=\"$(ps -o ppid= -p \"${p}\" 2>/dev/null | tr -d ' \\n')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  ( cd \"${mod}\" && go test -list '.*' \"${intra_path}\" ) 2>/dev/null | awk '/^Test/{print}' | sort > \"${tests_txt}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  [ -n \"${r}\" ] || r=\"$(git -C \"$(dirname \"${BASH_SOURCE[0]}\")\" rev-parse --show-toplevel 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  _af_iso=\"$(mktemp -d \"${_af_out_norm}/shard-affected-iso-${_af_tok}-XXXXXX\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  _af_mod=\"$(cd \"${mod}\" 2>/dev/null && pwd)\" || { printf '✗ --affected：--module 解析失败：%s ⇒ 不给结论(2)\\n' \"${mod}\" >&2; exit 2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  _n=\"$(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || printf '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  d=\"$(cd \"$(dirname \"${BASH_SOURCE[0]}\")\" 2>/dev/null && pwd)\" || d=\"\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  if ! mkdir -p \"$(dirname \"${lk}\")\" 2>/dev/null; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  if [ -n \"${SHARD_GO_TEST_NO_LOCK:-}\" ]; then shard_nolock_audit \"$@\"; return $?; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  if [ -z \"${GOCACHE:-}\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  local reason=\"${SHARD_GO_TEST_NO_LOCK_REASON:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  mkdir -p \"${_af_iso}/gate-lock\" \"${_af_iso}/tmp\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  mkdir -p \"${dir}\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  n_pkg=\"$(grep -c . \"${_af_pkgs}\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  r=\"$(git rev-parse --show-toplevel 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  rm -rf \"${_af_iso}\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  sort -u \"${_af_pkgs}\" -o \"${_af_pkgs}\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/gates/shard-go-test.sh",
		"anchor": "  { git -C \"${_af_root}\" -c core.quotepath=false diff --name-only                  2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "    [ -f \"\\$PF\" ] && kill \"\\$(cat \"\\$PF\")\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "    [ -f \"\\$PF\" ] && kill -0 \"\\$(cat \"\\$PF\")\" 2>/dev/null && exit 0 || exit 1 ;;",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "    cmd=\"$(ps -o command= -p \"$pid\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "    if printf '%s' \"$cmd\" | grep -Eq -- \"$pat\"; then kill \"$pid\" 2>/dev/null || true; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "    pids=\"$(cat \"$pf\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "    pids=\"$(pgrep -f \"$pat\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  LR=\"$(ls -1t \"$SANDBOX/$m/receipts\"/update-launch-*.json 2>/dev/null | head -1)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  OLD_RUN=\"$(curl -s -m 3 \"http://127.0.0.1:$LOCAL_PORT/api/fleet/status\" | python3 -c 'import json,sys;print((json.load(sys.stdin)[\"machines\"].get(\"mini-node-1\") or {}).get(\"code_sha\",\"\"))' 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  OLD_SHA=\"$(curl -s -m 3 \"http://127.0.0.1:$LOCAL_PORT/api/capabilities\" | python3 -c 'import json,sys;print(json.load(sys.stdin).get(\"code_sha\",\"\"))' 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  [ -f \"\\$PF\" ] && kill \"\\$(cat \"\\$PF\")\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  [ -n \"$OLD_SHA\" ] && break",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  [ -n \"$pat\" ] || return 0",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  [ -n \"${SANDBOX:-}\" ] && rm -rf -- \"${SANDBOX}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  [ -n \"${TOOLS:-}\" ]   && rm -rf -- \"${TOOLS}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  for b in zerg-core zerg-agent zerg-agentd; do codesign -s - --force \"$out/$b\" >/dev/null 2>&1 || true; done",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  if [ -n \"$LR\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  if [ -n \"$pf\" ] && [ -f \"$pf\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  is-active) [ -f \"\\$PF\" ] && kill -0 \"\\$(cat \"\\$PF\")\" 2>/dev/null && { echo active; exit 0; }; exit 3 ;;",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  kill \"$(cat \"$SANDBOX/mini-node-1/agentd.pid\" 2>/dev/null)\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  n_kernel=\"$(ls -1 \"$SANDBOX/$m/receipts\"/kernel-*.log 2>/dev/null | wc -l | tr -d ' ')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  n_rc=\"$(ls -1 \"$SANDBOX/$m/receipts\"/2*.json 2>/dev/null | wc -l | tr -d ' ')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "  stop) [ -f \"\\$PF\" ] && kill \"\\$(cat \"\\$PF\")\" 2>/dev/null || true; rm -f \"\\$PF\"; exit 0 ;;",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "B_RC_X3=\"$(ls -1 \"$SANDBOX/edge-node/receipts\" 2>/dev/null | wc -l | tr -d ' ')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "FRC=\"$(ls -1t \"$SANDBOX/local/receipts\"/fleet-*.json 2>/dev/null | head -1)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "INSTALLED_SHA=\"$(shasum -a 256 \"$SANDBOX/linbin-linux/zerg-agentd\" 2>/dev/null | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "LOCAL_V=\"$(\"$SANDBOX/local/bin/zerg-core\" --version 2>/dev/null | awk '{print $3}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "M1_V=\"$(\"$SANDBOX/mini-node-1/bin/zerg-agentd\" --version 2>/dev/null | awk '{print $3}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "MATRIX0=\"$(curl -s -m 3 \"http://127.0.0.1:$LOCAL_PORT/api/fleet/status\" 2>/dev/null)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "REAL_BIN_AFTER=\"$(cd \"$REPO_ROOT\" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "REAL_BIN_SNAP=\"$(cd \"$REPO_ROOT\" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "REAL_STATE_AFTER=\"$(find \"$HOME/.zerg/state\" -type f ! -name 'update_check.json' ! -name 'tool_uses.json' ! -name 'internal_engine.json' ! -name 'prefix_cache.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "REAL_STATE_SNAP=\"$(find \"$HOME/.zerg/state\" -type f ! -name 'update_check.json' ! -name 'tool_uses.json' ! -name 'internal_engine.json' ! -name 'prefix_cache.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "UNKNOWN_TARGETS=\"$(awk '/^ssh /{print $2}' \"$SSHLOG\" | sort -u | grep -vE '^(edge-node|mini-node-1|bare)$' || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "X3_V=\"$(\"$SANDBOX/edge-node/bin/zerg-agentd\" --version 2>/dev/null | awk '{print $3}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "[ -d \"$SANDBOX\" ] && chmod -R u+w \"$SANDBOX\" 2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "[ -f \"\\$PF\" ] && kill \"\\$(cat \"\\$PF\")\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "[ -n \"$D1\" ] && [ \"$D1\" = \"$D2\" ] && ok \"同 commit+同平台+同构建时间 ⇒ 同 sha256（${D1:0:12}…）\" || bad \"两次构建 sha256 不同（可复现性断言失败）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "[ -n \"$FRC\" ] && ok \"机群回执已落盘：$(basename \"$FRC\")\" || bad \"没有机群回执\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "a=\"$(ls -1 \"$SANDBOX/edge-node/receipts\" 2>/dev/null | wc -l | tr -d ' ')\"; [ \"$a\" = \"$B_RC_X3\" ] && ok \"edge-node 未产生回执（没执行更新）\" || bad \"--plan 产生了回执\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "cd \"\\$NODE\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "n_unknown=\"$(grep -c \"拒绝连接未知名册目标\" \"$SSHLOG\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "note \"假 ssh 调用总数：$(grep -c '^ssh ' \"$SSHLOG\" 2>/dev/null || echo 0)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-fleet-sandbox.sh",
		"anchor": "snap_machine() { find \"$1\" -type f -exec shasum -a 256 {} \\; 2>/dev/null | sort | shasum -a 256 | awk '{print $1}'; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "  got=\"$(gitq -C \"$p\" rev-parse refs/heads/main 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "FETCH_SHA=\"$(gitq -C \"$SANDBOX/edge-node/work\" rev-parse FETCH_HEAD 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "REAL_BIN_AFTER=\"$(cd \"$REPO_ROOT\" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "REAL_BIN_SNAP=\"$(cd \"$REPO_ROOT\" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "REAL_RC_AFTER=\"$(ls -la \"$HOME/.zerg/update_receipts\" 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "REAL_RC_SNAP=\"$(ls -la \"$HOME/.zerg/update_receipts\" 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "REAL_STATE_AFTER=\"$(find \"$HOME/.zerg/state\" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "REAL_STATE_SNAP=\"$(find \"$HOME/.zerg/state\" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "UNKNOWN=\"$(awk '/^ssh /{print $2}' \"$SSHLOG\" | sort -u | grep -vE '^(edge-node|mini-node-1|mini0|ghost)$' || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "[ -d \"$SANDBOX\" ] && chmod -R u+w \"$SANDBOX\" 2>/dev/null",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "[ -n \"$FETCH_ERR\" ] && printf '%s\\n' \"$FETCH_ERR\" | sed 's/^/       （git 输出）/' || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "[ -n \"$M0_CMD\" ] && ok \"mini0 的节点步被执行了（能被看到，才谈得上核对）\" || bad \"mini0 节点步没执行\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "[ -n \"\\${ZERG_FAKE_SSH_NOISE:-}\" ] && { echo \"MOTD（假 ssh 噪声）：欢迎登录 \\$node —— 真机 ssh 常会打横幅/告警，判定不得被它污染\" >&2; echo \"回显（ssh -v 之类会回显整条命令）：\\$cmd\" >&2; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "ck \"yaml 里的 relay= 生效（裸仓落在 yaml 指定的路径）\" \"$(sha12 \"$(gitq -C \"$SANDBOX/edge-node/yaml-relay.git\" rev-parse refs/heads/main 2>/dev/null)\")\" \"$(sha12 \"$C3\")\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "ck \"取到的树里带着镜像特征件 .mirror-state\" \"$(gitq -C \"$SANDBOX/edge-node/work\" ls-tree --name-only HEAD | grep -c '^\\.mirror-state$' || true)\" \"1\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "cleanup_sandbox() { [ -n \"${SANDBOX:-}\" ] && rm -rf -- \"${SANDBOX}\"; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "gitq clone -q --depth 1 \"file://$MIRROR\" \"$SANDBOX/edge-node/work\" 2>/dev/null || gitq clone -q --depth 1 \"$MIRROR\" \"$SANDBOX/edge-node/work\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-relay-sandbox.sh",
		"anchor": "snap_tree() { find \"$1\" -type f -exec shasum -a 256 {} \\; 2>/dev/null | sort | shasum -a 256 | awk '{print $1}'; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "\t\texec.Command(\"codesign\", \"-s\", \"-\", \"--force\", filepath.Join(binDir, b)).Run()",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    BIN=\"\\${ZERG_FAKE_UNIT_BIN:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    PF=\"\\$PF_DIR/\\$unit.pid\"; [ -f \"\\$PF\" ] && kill \"\\$(cat \"\\$PF\")\" 2>/dev/null || true; rm -f \"\\$PF\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    PF=\"\\$PF_DIR/\\$unit.pid\"; [ -f \"\\$PF\" ] && kill \"\\$(cat \"\\$PF\")\" 2>/dev/null || true; rm -f \"\\$PF\"; exit 0 ;;",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    [ -f \"\\$PF\" ] && kill \"\\$(cat \"\\$PF\")\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    cmd=\"$(ps -o command= -p \"$pid\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    if [ -f \"\\$PF\" ] && kill -0 \"\\$(cat \"\\$PF\")\" 2>/dev/null; then echo \"\\$lab = { pid = \\$(cat \"\\$PF\") }\"; exit 0; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    if [ -f \"\\$PF\" ] && kill -0 \"\\$(cat \"\\$PF\")\" 2>/dev/null; then echo active; exit 0; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    if printf '%s' \"$cmd\" | grep -Eq -- \"$pat\"; then kill \"$pid\" 2>/dev/null || true; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    pids=\"$(cat \"$pf\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    pids=\"$(pgrep -f \"$pat\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "    plist=\"\\$(cat \"\\$PF_DIR/\\$lab.plist\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "  [ -n \"$pat\" ] || return 0",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "  [ -n \"${SANDBOX:-}\" ] && rm -rf -- \"${SANDBOX}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "  [ -n \"${TOOLS:-}\" ]   && rm -rf -- \"${TOOLS}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "  if [ -n \"$pf\" ] && [ -f \"$pf\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "\"$SANDBOX/fakebin/launchctl\" bootout \"gui/$(id -u)/com.zerg.core\" >/dev/null 2>&1 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "BI_COMMIT=\"$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))[\"commit\"])' \"$NODE_TREE/bin/build-info.json\" 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "DISK_SHA=\"$(\"$CTL_PREFIX/zerg-core\" --version 2>/dev/null | awk '{print $3}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "LIVE2=\"$(curl -s -m 3 \"http://127.0.0.1:$PORT/api/capabilities\" 2>/dev/null | python3 -c 'import json,sys;print(json.load(sys.stdin).get(\"code_sha\",\"\"))' 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "LIVE_SHA=\"$(curl -s -m 3 \"http://127.0.0.1:$PORT/api/capabilities\" 2>/dev/null | python3 -c 'import json,sys;print(json.load(sys.stdin).get(\"code_sha\",\"\"))' 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "NODE_SHA=\"$(\"$NODE_PREFIX/zerg-agentd\" --version 2>/dev/null | awk '{print $3}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "REAL_BIN_AFTER=\"$(cd \"$REPO_ROOT\" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "REAL_BIN_SNAP=\"$(cd \"$REPO_ROOT\" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "REAL_STATE_AFTER=\"$(find \"$HOME/.zerg/state\" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "REAL_STATE_SNAP=\"$(find \"$HOME/.zerg/state\" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "UNKNOWN_UNITS=\"$(grep -vE \"daemon-reload|${AGENTD_UNIT}\" \"$SYSCTL_LOG\" || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "cmd=\"\\${1:-}\"; shift || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-setup-sandbox.sh",
		"anchor": "has()  { if grep -qF -- \"$2\" \"$1\" 2>/dev/null; then ok \"$3\"; else bad \"$3（未在 $1 找到 $2）\"; fi; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "    cmd=\"$(ps -o command= -p \"$pid\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "    if printf '%s' \"$cmd\" | grep -Eq -- \"$pat\"; then kill \"$pid\" 2>/dev/null || true; fi",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "    pids=\"$(cat \"$pf\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "    pids=\"$(pgrep -f \"$pat\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "  NEW_PID=\"$(cat \"$SANDBOX/core.pid\" 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "  OLD_SHA=\"$(curl -s -m 3 \"http://127.0.0.1:$PORT/api/capabilities\" | python3 -c 'import json,sys;print(json.load(sys.stdin).get(\"code_sha\",\"\"))' 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "  [ -f \"\\$PF\" ] && kill \"\\$(cat \"\\$PF\")\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "  [ -n \"$OLD_SHA\" ] && break",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "  [ -n \"$pat\" ] || return 0",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "  [ -n \"${SANDBOX:-}\" ] && rm -rf -- \"${SANDBOX}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "  [ -n \"${TOOLS:-}\" ]   && rm -rf -- \"${TOOLS}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "  for b in zerg-core zerg-agent; do codesign -s - --force \"$out/$b\" >/dev/null 2>&1 || true; done",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "  if [ -n \"$NEW_PID\" ] && [ \"$NEW_PID\" != \"$UPD_PID_BEFORE\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "  if [ -n \"$pf\" ] && [ -f \"$pf\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "A_STATE=\"$(find \"$STATE\" -type f ! -name 'update_check.json' | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "B_STATE=\"$(find \"$STATE\" -type f ! -name 'update_check.json' | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "LR=\"$(ls -1t \"$RECEIPTS\"/update-launch-*.json 2>/dev/null | head -1)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "NEW_PID=\"$(cat \"$SANDBOX/core.pid\" 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "NEW_SHA=\"$(curl -s -m 3 \"http://127.0.0.1:$PORT/api/capabilities\" | python3 -c 'import json,sys;print(json.load(sys.stdin)[\"code_sha\"])' 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "N_RC_AFTER=\"$(ls -1 \"$RECEIPTS\"/2*.json 2>/dev/null | wc -l | tr -d ' ')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "N_RC_BEFORE=\"$(ls -1 \"$RECEIPTS\"/2*.json 2>/dev/null | wc -l | tr -d ' ')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "REAL_BIN_AFTER=\"$(cd \"$REPO_ROOT\" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "REAL_BIN_SNAP=\"$(cd \"$REPO_ROOT\" && ls -la bin 2>/dev/null | shasum -a 256 | awk '{print $1}'; shasum -a 256 bin/* 2>/dev/null | awk '{print $1}' | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "REAL_STATE_AFTER=\"$(find \"$HOME/.zerg/state\" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "REAL_STATE_SNAP=\"$(find \"$HOME/.zerg/state\" -type f ! -name 'update_check.json' 2>/dev/null | sort | xargs -I{} shasum -a 256 {} 2>/dev/null | shasum -a 256 | awk '{print $1}')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "R_SHA=\"$(curl -s -m 3 \"http://127.0.0.1:$PORT/api/capabilities\" | python3 -c 'import json,sys;print(json.load(sys.stdin)[\"code_sha\"])' 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "ZERG_UPDATE_REMOTE=\"https://nonexistent.invalid/x.git\" \"$UPD\" update --check --no-cache >/dev/null 2>&1 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "[ -f \"\\$PF\" ] && kill \"\\$(cat \"\\$PF\")\" 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "[ -n \"$NEW_PID\" ] && [ \"$NEW_PID\" != \"$UPD_PID_BEFORE\" ] && ok \"主控 pid 已变化（新进程）\" || bad \"pid 未变（可能自己换自己）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "grep -q \"运行进程）自报 code_sha\" \"$RECEIPTS\"/kernel-*.log 2>/dev/null && ok \"内核 verify 走的是「运行进程自报」\" || note \"（内核日志未见 verify 行，见下）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/sandbox/test-update-sandbox.sh",
		"anchor": "if [ -n \"$LR\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "      if [ -n \"$rk\" ] && [ \"$rk\" = \"$lk_sha\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "      rk=\"$(printf '%s\\n' \"$rk_raw\" | grep -E '^[0-9a-f]{40}$' | tail -1 || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "      rk_raw=\"$(\"$SSH_CMD\" ${SSH_OPTS} \"$n_ssh\" \"git -C '${n_root}' hash-object '${kpath}'\" </dev/null 2>&1 || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "    \"$SSH_CMD\" ${SSH_OPTS} \"$n_ssh\" \"mkdir -p '$(dirname \"$kpath\")' && cat > '$kpath' && chmod +x '$kpath'\" < \"$KERNEL\" 2>/dev/null || krc=$?",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "    --force)   FORCE=1; shift ;;",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "    [ -n \"$ONLY_NODE\" ] && [ \"$ONLY_NODE\" != \"$n_name\" ] && continue",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "    [ -n \"$TO_PATH\" ] && tp=\"$TO_PATH\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "    [ -n \"$n_name\" ] || continue",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "    [ -n \"$tp\" ] || tp=\"$(dirname \"$n_root\")/zerg-relay.git\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "    lk_sha=\"$(git hash-object \"$KERNEL\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "    rb=\"$(printf '%s\\n' \"$rb_raw\" | grep -oE '[0-9a-f]{40}' | tail -1 || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "  [ \"$FORCE\" = \"1\" ] && FORCE_FLAG=\"--force\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "  [ -n \"$ONLY_NODE\" ] && [ \"$ONLY_NODE\" != \"$n_name\" ] && continue",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "  [ -n \"$TO_PATH\" ] && tp=\"$TO_PATH\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "  [ -n \"$n_name\" ] || continue",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "  [ -n \"$tp\" ] || tp=\"$(dirname \"$n_root\")/zerg-relay.git\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "  rb=\"$(printf '%s\\n' \"$rb_raw\" | grep -E '^[0-9a-f]{40}$' | tail -1 || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "  verdict=\"$(printf '%s\\n' \"$eout\" | grep -E '^RELAY-(EXISTS|CREATED)$' | tail -1 || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "N_NODES=\"$(printf '%s\\n' \"$NODES\" | grep -c . || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "SRC_SHA=\"$(git -C \"$FROM\" rev-parse \"refs/heads/${REF}\" 2>/dev/null)\" || SRC_SHA=\"\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "SRC_TOP=\"$(git -C \"$FROM\" rev-parse --show-toplevel 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "[ -n \"$FROM\" ] || die \"--from 缺镜像仓目录（例：--from /tmp/zerg-mirror-20260913）\" 64",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "[ -n \"$NODES\" ] || die \"名册里没有节点：设 ZERG_FLEET_NODES，或在 gateway/fleet.yaml 的 updates.nodes 里列出\" 2",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "[ -n \"$SRC_SHA\" ] || die \"源仓没有 refs/heads/${REF}：${FROM}\" 2",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "if [ -n \"$ONLY_NODE\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/fleet-relay.sh",
		"anchor": "if [ -n \"$SRC_TOP\" ] && [ \"$SRC_TOP\" = \"$REPO_ROOT\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "    [ -n \"$c\" ] || continue",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "    [ -n \"$disk\" ] || die '读不到 zerg-agentd 的自报身份' 5",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "    [ -n \"$disk\" ] || die '读不到安装件 zerg-core 的自报身份' 5",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "    [ -n \"$got\" ] && break",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F09-FORCE-FLAG", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "    codesign -s - --force \"$dst\" >/dev/null 2>&1 || warn \"${comp} 重签名失败（未签名二进制会被秒杀）\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "    if [ -n \"$(git -C \"$TREE\" status --porcelain 2>/dev/null)\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "    printf 'X-Auth-Token: %s\\n' \"$(cat \"$TOKEN_FILE\" 2>/dev/null || true)\" | curl -m 30 -H @- \"$@\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "    tgt=\"$(git -C \"$TREE\" rev-parse FETCH_HEAD 2>/dev/null || echo '')\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  \"$1\" --version 2>/dev/null | awk '{print $3}' | sed 's/+.*//'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F02-OR-TRUE", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  \"$SYSTEMCTL\" daemon-reload >/dev/null 2>&1 || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  [ -n \"$disk\" ] || die \"读不到 zerg-agentd 的自报身份（--version；linux 件需在目标机上读）\" 5",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  [ -n \"$disk\" ] || die \"读不到安装件 zerg-core 的自报身份（--version）\" 5",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  [ -n \"$out\" ] || die \"组件集为空（--components ${raw}）\" 4",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  [ -n \"$pin\" ] || die \"读不到 ${gomod} 的 go 指令（钉住的单一真源）\" 7",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  [ -n \"$raw\" ] || return 0",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  go version 2>/dev/null | awk '{print $3}'",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  if [ -n \"$raw\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  if [ -n \"$tree\" ] && ! sha_match \"$disk\" \"$tree\"; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "  raw=\"$(curl_tok -s -m 3 \"$API/api/capabilities\" 2>/dev/null || true)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "AGENTD_LABEL=\"${ZERG_SETUP_AGENTD_LABEL:-}\"      # 同上（darwin 用户域 LaunchAgent 的作业名）",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "AGENTD_UNIT=\"${ZERG_SETUP_AGENTD_UNIT:-}\"        # 空 ⇒ 参数面解析后按真源推导（见下「单元/作业名推导」）",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "COMPONENTS=\"${ZERG_SETUP_COMPONENTS:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "NODE_NAME=\"${ZERG_SETUP_NODE_NAME:-$(hostname -s 2>/dev/null || hostname 2>/dev/null || echo node)}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "PREFIX=\"${ZERG_PREFIX:-}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "[ -n \"$AGENTD_LABEL\" ] || AGENTD_LABEL=\"com.zerg.agent-${NODE_NAME}\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "[ -n \"$AGENTD_UNIT\" ]  || AGENTD_UNIT=\"${NODE_NAME}-agent\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F06-NONEMPTY-GUARD", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "[ -n \"$PREFIX\" ] || PREFIX=\"$TREE/bin\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "[ -n \"${ZERG_SETUP_SYSTEMCTL:-}\" ] && SYSTEMCTL_EXPLICIT=1",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F11-EMPTY-DEFAULT", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "if [ -n \"${ZERG_SETUP_OS:-}\" ]; then",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "print(d.get(\"code_sha\") or \"\")' 2>/dev/null || true",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/setup-zerg.sh",
		"anchor": "tree_head() { git -C \"$TREE\" rev-parse HEAD 2>/dev/null || echo ''; }",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
	{"form": "F01-DEVNULL", "path": "scripts/svc/zerg-core-daemon.sh",
		"anchor": "export ZERG_GIT_COMMIT=\"$(git -C \"$REPO_ROOT\" rev-parse --short HEAD 2>/dev/null || echo unknown)\"",
		"reason": "GAP-20260927-133 ⑤ 历史存量一次性入册（只许减 · 待治）",
		"source": "GAP-20260927-133", "gate": "本件"},
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
