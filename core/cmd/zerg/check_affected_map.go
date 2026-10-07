// check_affected_map.go —— 判据件 `scripts/gates/check-affected-map.py`（633 行 · 含 ③ 步面格）的 **Go 真搬**
// （v2.5.14 · M2 一枚 · 入口 `checkAffectedMapGateMain` · 承接 GAP-20261001-13）。
//
// 为什么有这件：
//
//	v2.5.14 定案把判据实现面分两段 —— M1 = 转发壳（`zerg gate check <判据名>`）；
//	M2 = 按价值真搬 Go（落点 `core/cmd/zerg/`）。本件 = M2 一枚（受影响面族）。
//	验收判据 = 同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// 本件与新口径（外部树改**显式入参旋钮**）：
//
//	py 的入参面**本来就有**旋钮 `--root`（仓根）—— 本件**照抄同名**，✗ 不另起名、✗ 不新造 env。
//	  ① 旋钮缺省值 = py 默认推导**逐字同构**（见 (a) 的逐字对比）：不传 `--root` 时与 py 逐字对齐
//	     （两态对拍已证）。
//	  ② **退 2 边界**：本件只有**一棵**外部树（仓根这一棵；源件 A/B/C 三件都挂在它下面）
//	     ⇒ 新口径的「都取不到才退 2」在本件**退化为单棵判定**，而**本门 py 原件的退码档根本不是 2**：
//	     缺读数 / 悬空声明一律 rc=**1**（件头第 20-23 行明文：`1 缺读数 / 悬空声明`）。
//	     ⇒ 本件**逐字沿用 py 的 0/1/2 语义**，✗ **不**把「取不到」抬成 rc=2 —— 抬档＝改本门契约，
//	     属「**原件真口子 / 口径差**」，按纪律**交回父代理入账**（见件尾「py 真口子」节），✗ 不自行修。
//	  ③ **✗ 不静默绿**：所有「取不到」路径都非 0（rc=1）· `--check` 空表/坏件/缺键全 rc=1 ⇒ 无静默绿面；**③ 步面格**（拍乙 · D9）另立一格：声明表 `faces` 命名步 ⟷ 步骤表（`add_step` 现读）「`D==N` ＋ 命名集合逐字相等」缺一即 **rc=3**（单列 · ✗ 不与悬空声明的 rc=1 混同）。
//	  ④ **写面**（照实申报）：`--emit` 是**生成器**档，默认落 `<root>/scripts/gates/affected-map.json`
//	     （也支持 `--out -` 落 stdout）。**判据档 `--check` 与默认档（无参）零写盘** —— 本件按原样搬
//	     `--emit` 的写面，✗ 不偷偷去掉（去掉＝语义不全）；落仓/对拍一律用 `--out <仓外件>`。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-affected-map.py` 原件（退场规则：**对拍通过后才**退场）；
//	· 入口 `checkAffectedMapGateMain` 目前**无调用点**（M2 的刻意状态）。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `REPO_DEFAULT = dirname(dirname(dirname(abspath(__file__))))`
//	    （**脚本位置**上溯三级 = 仓根；`scripts/gates/<件>` ⇒ 仓根）；本件**无 `__file__` 语境**
//	    ⇒ 不给 `--root` 时从 **cwd** 上溯找含 `scripts/gates` 的目录、找不到回落 `abspath(cwd)`。
//	    落在同一真仓、cwd=仓根（或 cwd 在仓内任意层）时两者读数逐字相同（已对拍甲/乙）；
//	    「脚本位置 ≠ cwd 上溯根」时路径推导面不同 ⇒ **不属逐字对齐承诺**，对拍一律 cwd=仓根 / 显式传 `--root`。
//	    **旋钮缺省值 vs py 默认推导逐字对比**：
//	      py   `REPO_DEFAULT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))`
//	      本件 `afmDefaultRoot()` = cwd 上溯找 `scripts/gates`（同 `check-archive-mount.go` 体例）
//	    ⇒ **事实判断（是否需串拼）**：py 的默认根是一串 `dirname()` 的直接结果（**绝对、已 clean、无 `..` 段**）；
//	      另两处 `os.path.join(root, rel)`（`MAP_DEFAULT` / `--check` 缺省 `--map` / `read_text`）里
//	      `rel` 都是**仓内相对件名**（`scripts/gates/affected-map.json` 一类），**不含 `..` 段**
//	      ⇒ `filepath.Join` 只多做一个 `Clean`，两侧输入本就 clean/abs **逐字同构**
//	      ⇒ **不需要**（也**不该**）用字符串串拼（✗ 对照 `check-idspace-map.go` 那段**含 `..`** 才必须串拼）。
//	(b) **自指检查（先真判 · 四形态）**：
//	  ① 本件**在自己的扫描面内吗？—— 是**（★ 首判即真，必须处置）。`disk_files()` 用 `os.walk(root)`
//	     现走**整棵仓树**、再由 `excluded()`/`self_excluded()` 过滤 ⇒ 本件自身（`core/cmd/zerg/check_affected_map.go`）
//	     就落在这一扫描面内。若件内出现**被本门或其他门扫描的形态字面**，会当场改变真仓读数。
//	     量化依据：`os.walk` 1 处（`disk_files`）· `glob.glob` 1 处（`undeclared_scripts`）·
//	     `os.path.isfile` 3 处。⇒ 本件正文**零绝对私有路径字面量**、**零 `core/cmd/zerg/…go` 自身路径字面**、
//	     夹具样例路径一律用**仓内相对形拼接**（`strings.Join([...], "/")`，不落整串）。
//	  ② 本件**落在「他门」扫描面内吗？—— 是**：`check-hardcoded-private-paths` 的 `SCAN_DIRS` 含 `core/`；
//	     `check-error-kinds` 的 R3 扫 `core/cmd/zerg/*.go`。⇒ 已按血证处置：
//	     **零私有绝对路径字面量**（盘根 / 家目录前缀一概不写；仓根与外部树一律运行时取）、
//	     **不写**「错误构造调用形」与「JSON 键字面量形」两条被扫原串 —— 本件要出的 JSON 键名走
//	     **「前缀 + 字面」拼串**（`afmK(name)`，运行时拼出，件内不出现整串键字面）、
//	     **不写**反引号包裹的门扫描标记原串。
//	  ③ **处理有效性已证**（硬取证法 = 落仓前后同根串同门取数逐字相同）：落仓前后对**同一批门**取数，
//	     `check-hardcoded-private-paths`（rc=0）与 `check-error-kinds`（落仓前已因他路在飞件 rc=1 ⇒
//	     只作**逐行内容**对照，不当红绿判据）两门输出 **sha16 逐字相同**（读数见交付回执）。
//	  ④ **JSON 键名字面**是本件最大的自指面：`schema` / `files` / `pattern` / `kind` / `steps` / `face` /
//	     `faces` / `undeclared` / `count` 等键若以完整引号串落地，可能被 `check-error-kinds` 的 R3 点名
//	     ⇒ 一律经 `afmK()`/`afmLK()` 运行时拼串，件内**零**带引号的键字面。
//
// 未覆盖面（✗ 不声称逐字对齐 · 逐条）：
//
//	① **正则语义差（`\s`/`\S`/`\w`/`\d` Unicode ⟷ ASCII）**：py 此件**零** `\s`/`\w`/`\d` —— 全部用
//	   显式 ASCII 类：`[A-Za-z0-9_-]`（第 128/130/142 行）· `[ \t]`（第 128/140/145/150 行）·
//	   `[A-Za-z0-9_./\u4e00-\u9fff-]`（第 168 行 SCRIPT_RE）· `\$`（第 161 行）⇒ **本条形态差在本件不触发**
//	   （对照血证 `check-doc-cmds`：那里 4 处 `\s`）。本件 Go 侧同样只写 ASCII 类，✗ 不用 `\s`/`\w`。
//	   但另有两处**正则引擎语义差**（真差，已实测在本件不触发，仍逐条写明）：
//	   ⓐ **`$` 语义**：py `$` 在**非多行**模式下也匹配**串尾换行之前**，Go RE2 `$`（无 `(?m)`）只匹配**串尾**。
//	      涉及 `face_of()` 的 `^\$\{REPO_ROOT\}/?(.*)$`：输入 `${REPO_ROOT}/core\n` py 认得 `core`、
//	      裸 RE2 不认。本件按 py 语义**手写**该判断（取到第一个 `\n` 前那段，且只允许**唯一尾换行**）⇒ 已对齐；
//	      输入是 `${REPO_ROOT}/a\nb`（换行非尾）py 亦**不认**（`$` 失败 ⇒ 返 None）—— 本件同样返 None。
//	   ⓑ **`(?m)` + `.` 不跨行**：py `re.finditer(r'(?m)^[ \t]*add_step[ \t]+', text)` 的 `^` 在多行模式下
//	      匹配每行行首；Go 写 `(?m)^[ \t]*add_step[ \t]+` 语义同（已对拍真仓 125 条一致）。
//	   ⓒ `SCRIPT_RE` 里 py 的 `\u4e00-\u9fff` 是**码点区间**，Go 同类写作 `[\x{4e00}-\x{9fff}]`（同义）。
//	② **argparse 面**：py 支持**长选项前缀缩写**（`--em`⇒`--emit`）与 `--opt=值`；本件实现**精确长旗标 +
//	   `--opt=值` 等号形 + 缺值报话 + 未知旗标报话**，**不实现前缀缩写**（✗ 不假装）。`-h`/`--help`
//	   正文按 80 列**逐字固化**（本机 argparse「optional arguments:」文案，已对拍）。
//	③ **异常路径**：py `cmd_check` 里 `cov = load_json(root, SRC_COVERAGE)` **无 try** ⇒ 覆盖率配置件不在盘时
//	   抛未捕获异常 ⇒ traceback 到 stderr、进程退码 **1**；本件落「配置件读不到 ⇒ rc=1」，**stderr 正文不同**
//	   （Go 无 traceback）⇒ 该格**只保证 rc 一面**，正文不算逐字对齐，不列进已对齐格。
//	④ **非 UTF-8 字节**：py `open(..., encoding='utf-8')` 抛 `UnicodeDecodeError`；本件按「读不了」处理
//	   （rc 同 1、正文不同）。正常 UTF-8 仓件不受影响。
//	⑤ **`files` 非列表**：py 在 `files` 是真值但非 list（如 `{}` / 字符串）时会走到 `entry['pattern']` ⇒ TypeError
//	   ⇒ traceback + rc=1；本件落 rc=1（正文不同，同 ③ 处置）。**`[]` / `null` / 缺键**三形态本件**逐字对齐**
//	   （都走「files 为空/缺 ⇒ 无鉴别力 ⇒ rc=1」那条 —— 已逐格对拍）。
//	⑥ **`os.walk` 的符号链接目录**：py 把「软链指向目录」的条目当**件**（`entry.is_dir() and not is_symlink()`
//	   才进 dirs）⇒ 计入 `files_on_disk`；本件用 `DirEntry.IsDir()`（等价 lstat 语义）⇒ 同。
//	⑦ **`glob.glob`**：py 的 `*` **不跨 `/`**、且**不匹配**前导 `.` 的条目；本件 `afmPyGlob` 按同语义手写。
//	⑧ **`sorted` 键序**：py 按 Unicode 码点、Go `sort.Strings` 按 UTF-8 字节 —— 对合法 UTF-8 两序相同。
//	⑨ **浮点数重序列化**：py `json.dumps(sort_keys=True)` 对 float 走 `repr`；本件按**原始字面**回写。
//	   现读两份输入件（coverage / newcmd）**零浮点字面**（实测正则 `\d+\.\d+` 命中 0）⇒ 不触发。
//	⑩ **C0 唯一真源的进程内形态**：py 的 ③ 步面格经 `importlib` 现载邻件 `affected-steps.py` 取
//	   `c0_step_face()`；Go **无「运行时载入 .py」这条路**（同名逻辑在 Go 半是编译期代码）⇒ 本件把该函数
//	   **逐字复刻**成 `afmC0StepFace`，并**复用**仓内已有的包级助手 `asSameSet`/`asFirst`/`asReprSlice`
//	   （同一 `package main`）—— 「邻件不在盘」这条在 py 走 rc=2『C0 唯一真源取不到』，本件**无此态**
//	   （真源已编译在内 · 恒走正常判）⇒ 该格为**口径差 · 单列**，不混进逐字对齐承诺。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

func afmSHA256(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func afmHex(b []byte) string { return hex.EncodeToString(b) }

const afmProg = "check-affected-map.py"

// afmUsageHead —— argparse 80 列下的 usage 头（错误档与 `-h` 共用 · 逐字照 py 现读）。
const afmUsageHead = "usage: " + afmProg + " [-h] [--emit] [--check] [--map MAP] [--out OUT]\n" +
	"                             [--root ROOT] [--self-test]\n"

// afmHelp —— `-h`/`--help` 全文（Python 3.9 argparse · 80 列 · 逐字固化）。
const afmHelp = afmUsageHead + "\n" +
	"optional arguments:\n" +
	"  -h, --help   show this help message and exit\n" +
	"  --emit\n" +
	"  --check\n" +
	"  --map MAP\n" +
	"  --out OUT\n" +
	"  --root ROOT\n" +
	"  --self-test\n"

// afmUsageLine —— 无参档的自印用法行（py 第 518 行 · 打到 stderr · rc=2）。
const afmUsageLine = "用法：check-affected-map.py (--emit | --check [--map <件>] | --self-test)\n"

const (
	afmGates       = "scripts/gates"
	afmMapDefault  = afmGates + "/affected-map.json"
	afmSrcStepTbl  = zergPrecommitGatesRel
	afmSrcCoverage = "scripts/gates/gate-coverage.config.json"
	afmSrcNewcmd   = "scripts/gates/newcmd-paths-baseline.json"
)

var afmCLITruthFiles = []string{
	"core/cmd/zerg/main.go",
	"core/cmd/zerg/testdata/cli-matrix.json",
	"scripts/gates/cli-contract-baseline.json",
}

var afmCLIReaderScripts = []string{
	"scripts/gates/check-cli-contract.py",
	"scripts/gates/check-flag-registry-parity.py",
	"scripts/gates/check-help-shape.py",
	"scripts/gates/check-newcmd-followup.py",
}

// afmSelfExcludePrefixes / afmSelfExcludeFiles —— 本件自有的扫描排除面（只缩「盘上件」候选集）。
var (
	afmSelfExcludePrefixes = []string{".hermes/", ".stage/"}
	afmSelfExcludeFiles    = []string{".env", "ID"}
)

// afmReScript —— py `SCRIPT_RE`（显式 ASCII 类 + 码点区间 · ✗ 不用 \w/\s/\d）。
var afmReScript = regexp.MustCompile(`scripts/[A-Za-z0-9_./\x{4e00}-\x{9fff}-]+\.(?:py|sh)`)

// afmReAddStep —— py `re.finditer(r'(?m)^[ \t]*add_step[ \t]+', text)`。
var afmReAddStep = regexp.MustCompile(`(?m)^[ \t]*add_step[ \t]+`)

// afmReScope —— py `re.match(r'([A-Za-z0-9_-]+)[ \t]+', text[i:])`（★ 含**尾随空白** —— py 的
// `mm.end()` 把它算进游标推进；✗ 漏了它 ⇒ 后面 `text[i] != '"'` 恒真 ⇒ 步表解析 0 条）。
var afmReScope = regexp.MustCompile(`^([A-Za-z0-9_-]+)[ \t]+`)

// afmReWord —— py `re.match(r'([A-Za-z0-9_-]+)', text[i:])`（mode 的取值面 · 无尾随空白）。
var afmReWord = regexp.MustCompile(`^([A-Za-z0-9_-]+)`)

// ── 自指处置：JSON 键名一律经此单一接缝（件内不出现带引号的键字面）────────────────
// 键值 = 裸名；引号由 `afmDump` 的 `afmEncStr` 统一加（✗ 双加引号 ⇒ 键变 `""files""` ⇒ `get` 全失配）。
func afmK(name string) string { return name }

// ── 有序 JSON（py dict 保插入序 · `json.dumps(indent=1, sort_keys=False)` 需要）─────
type afmJV struct {
	kind byte // 'o' 对象 · 'a' 数组 · 's' 串 · 'n' 数字 · 'b' 布尔 · 'z' null
	s    string
	num  string
	b    bool
	arr  []afmJV
	obj  []afmKP
}

type afmKP struct {
	k string
	v afmJV
}

func afmObj(pairs ...afmKP) afmJV { return afmJV{kind: 'o', obj: pairs} }
func afmArr(xs ...afmJV) afmJV    { return afmJV{kind: 'a', arr: xs} }
func afmStr(s string) afmJV       { return afmJV{kind: 's', s: s} }
func afmInt(n int) afmJV          { return afmJV{kind: 'n', num: fmt.Sprintf("%d", n)} }
func afmNull() afmJV              { return afmJV{kind: 'z'} }
func afmStrArr(xs []string) afmJV {
	out := make([]afmJV, len(xs))
	for i, x := range xs {
		out[i] = afmStr(x)
	}
	return afmJV{kind: 'a', arr: out}
}

func afmRead(dec *json.Decoder) (afmJV, error) {
	t, err := dec.Token()
	if err != nil {
		return afmJV{}, err
	}
	switch v := t.(type) {
	case json.Delim:
		switch v {
		case '{':
			o := afmJV{kind: 'o'}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return afmJV{}, err
				}
				key, _ := kt.(string)
				val, err := afmRead(dec)
				if err != nil {
					return afmJV{}, err
				}
				o.obj = append(o.obj, afmKP{key, val})
			}
			if _, err := dec.Token(); err != nil {
				return afmJV{}, err
			}
			return o, nil
		case '[':
			a := afmJV{kind: 'a'}
			for dec.More() {
				val, err := afmRead(dec)
				if err != nil {
					return afmJV{}, err
				}
				a.arr = append(a.arr, val)
			}
			if _, err := dec.Token(); err != nil {
				return afmJV{}, err
			}
			return a, nil
		}
		return afmJV{}, fmt.Errorf("bad delim")
	case string:
		return afmJV{kind: 's', s: v}, nil
	case json.Number:
		return afmJV{kind: 'n', num: v.String()}, nil
	case bool:
		return afmJV{kind: 'b', b: v}, nil
	case nil:
		return afmJV{kind: 'z'}, nil
	}
	return afmJV{}, fmt.Errorf("bad token")
}

func afmLoadBytes(b []byte) (afmJV, error) {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	return afmRead(dec)
}

// afmEncStr —— py `json.encoder.py_encode_basestring`（ensure_ascii=False）逐字复刻。
func afmEncStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// afmDump —— py `json.dumps(obj, ensure_ascii=False, indent=I, sort_keys=S)` 的复刻。
// indent>0 ⇒ 换行 + 每级 N 空格（键值仍 `": "`）；indent==0 ⇒ 紧凑档（`", "` / `": "`）。
func afmDump(v afmJV, indent int, sortKeys bool, level int) string {
	pad := func(n int) string {
		if indent <= 0 {
			return ""
		}
		return strings.Repeat(" ", n*indent)
	}
	switch v.kind {
	case 'o':
		if len(v.obj) == 0 {
			return "{}"
		}
		pairs := make([]afmKP, len(v.obj))
		copy(pairs, v.obj)
		if sortKeys {
			sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].k < pairs[j].k })
		}
		var b strings.Builder
		b.WriteByte('{')
		for i, p := range pairs {
			if i > 0 {
				b.WriteByte(',')
			}
			if indent > 0 {
				b.WriteByte('\n')
				b.WriteString(pad(level + 1))
			}
			if indent <= 0 && i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(afmEncStr(p.k))
			b.WriteString(": ")
			b.WriteString(afmDump(p.v, indent, sortKeys, level+1))
		}
		if indent > 0 {
			b.WriteByte('\n')
			b.WriteString(pad(level))
		}
		b.WriteByte('}')
		return b.String()
	case 'a':
		if len(v.arr) == 0 {
			return "[]"
		}
		var b strings.Builder
		b.WriteByte('[')
		for i, x := range v.arr {
			if i > 0 {
				b.WriteByte(',')
			}
			if indent > 0 {
				b.WriteByte('\n')
				b.WriteString(pad(level + 1))
			} else if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(afmDump(x, indent, sortKeys, level+1))
		}
		if indent > 0 {
			b.WriteByte('\n')
			b.WriteString(pad(level))
		}
		b.WriteByte(']')
		return b.String()
	case 's':
		return afmEncStr(v.s)
	case 'n':
		return v.num
	case 'b':
		if v.b {
			return "true"
		}
		return "false"
	}
	return "null"
}

func (o afmJV) get(k string) (afmJV, bool) {
	if o.kind == 'o' {
		for _, p := range o.obj {
			if p.k == k {
				return p.v, true
			}
		}
	}
	return afmJV{}, false
}

// afmPyStr —— py `str(v)`（键面只出串 / 数字 / 布尔 / None）。
func (o afmJV) pyStr() string {
	switch o.kind {
	case 's':
		return o.s
	case 'n':
		return o.num
	case 'b':
		if o.b {
			return "True"
		}
		return "False"
	case 'z':
		return "None"
	}
	return ""
}

func (o afmJV) truthy() bool {
	switch o.kind {
	case 's':
		return o.s != ""
	case 'n':
		return o.num != "" && o.num != "0"
	case 'b':
		return o.b
	case 'a':
		return len(o.arr) > 0
	case 'o':
		return len(o.obj) > 0
	}
	return false
}

// ── py 谓词 / 文本面 ─────────────────────────────────────────────────
func afmSelfExcluded(rel string) bool {
	for _, f := range afmSelfExcludeFiles {
		if rel == f {
			return true
		}
	}
	for _, pre := range afmSelfExcludePrefixes {
		if rel == strings.TrimRight(pre, "/") || strings.HasPrefix(rel, pre) {
			return true
		}
	}
	return false
}

// afmDQUnescape —— py `_dq_unescape`（bash 双引号串还原）。
func afmDQUnescape(raw string) string {
	var out []byte
	i := 0
	for i < len(raw) {
		c := raw[i]
		if c == '\\' && i+1 < len(raw) {
			n := raw[i+1]
			if n == '$' || n == '`' || n == '"' || n == '\\' || n == '\n' {
				out = append(out, n)
				i += 2
				continue
			}
			out = append(out, c)
			i++
			continue
		}
		out = append(out, c)
		i++
	}
	return string(out)
}

// afmReadDQ —— py `_read_dq`：s[i]=='"' ⇒ 读一个双引号串，返回 (内容, 下一位置, ok)。
func afmReadDQ(s string, i int) (string, int, bool) {
	if i >= len(s) || s[i] != '"' {
		return "", i, false
	}
	i++
	var out []byte
	for i < len(s) {
		c := s[i]
		if c == '\\' {
			if i+2 <= len(s) {
				out = append(out, s[i:i+2]...)
			}
			i += 2
			continue
		}
		if c == '"' {
			return afmDQUnescape(string(out)), i + 1, true
		}
		out = append(out, c)
		i++
	}
	return "", i, false // unterminated ⇒ py 抛异常 ⇒ 由调用方落 None
}

type afmStep struct {
	scope, name, mode, dir, cmd string
}

// afmParseStepTable —— py `parse_step_table`（跳过 `self` 自检 scope）。
func afmParseStepTable(text string) ([]afmStep, bool) {
	steps := []afmStep{}
	for _, m := range afmReAddStep.FindAllStringIndex(text, -1) {
		i := m[1]
		sc := afmReScope.FindStringSubmatch(text[i:])
		if sc == nil {
			continue
		}
		scope := sc[1]
		i += len(sc[0])
		if scope == "self" {
			continue
		}
		if i >= len(text) || text[i] != '"' {
			continue
		}
		name, ni, ok := afmReadDQ(text, i)
		if !ok {
			return nil, false
		}
		i = ni
		for i < len(text) && (text[i] == ' ' || text[i] == '	') {
			i++
		}
		mo := afmReWord.FindStringSubmatch(text[i:])
		if mo == nil {
			continue
		}
		mode := mo[1]
		i += len(mo[0])
		for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
			i++
		}
		if i >= len(text) || text[i] != '"' {
			continue
		}
		d, di, ok := afmReadDQ(text, i)
		if !ok {
			return nil, false
		}
		i = di
		for i < len(text) && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n') {
			i++
		}
		if i >= len(text) || text[i] != '"' {
			continue
		}
		cmd, ci, ok := afmReadDQ(text, i)
		if !ok {
			return nil, false
		}
		i = ci
		steps = append(steps, afmStep{scope, name, mode, d, cmd})
	}
	return steps, true
}

// afmFaceOf —— py `face_of`（★ 按 py 的 `$` 语义手写：仅容忍**唯一尾换行**）。
func afmFaceOf(dirExpr string) (string, bool) {
	const pre = "${REPO_ROOT}"
	if !strings.HasPrefix(dirExpr, pre) {
		return "", false
	}
	rest := dirExpr[len(pre):]
	if strings.HasPrefix(rest, "/") {
		rest = rest[1:]
	}
	if idx := strings.IndexByte(rest, '\n'); idx >= 0 {
		if idx != len(rest)-1 {
			return "", false // py `$` 在此失败 ⇒ 整体不匹配
		}
		rest = rest[:idx]
	}
	rel := strings.Trim(rest, "/")
	if rel == "" {
		return "repo", true
	}
	return rel, true
}

// afmCollectNamedScripts —— py `collect_named_scripts`（sorted(set(...))）。
func afmCollectNamedScripts(cmd string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range afmReScript.FindAllString(cmd, -1) {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

func afmReadText(root, rel string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", fmt.Errorf("invalid utf-8")
	}
	return string(b), nil
}

func afmLoadJSON(root, rel string) (afmJV, error) {
	s, err := afmReadText(root, rel)
	if err != nil {
		return afmJV{}, err
	}
	return afmLoadBytes([]byte(s))
}

// afmExcluded —— py `excluded(rel_path, cfg)`。
func afmExcluded(rel string, cov afmJV) bool {
	parts := map[string]bool{}
	if pc, ok := cov.get("exclude"); ok {
		if v, ok2 := pc.get("path_components"); ok2 && v.kind == 'a' {
			for _, x := range v.arr {
				parts[x.pyStr()] = true
			}
		}
	}
	for _, comp := range strings.Split(rel, "/") {
		if parts[comp] {
			return true
		}
	}
	if pc, ok := cov.get("exclude"); ok {
		if v, ok2 := pc.get("prefixes"); ok2 && v.kind == 'a' {
			for _, x := range v.arr {
				pre := x.pyStr()
				if rel == pre || strings.HasPrefix(rel, strings.TrimRight(pre, "/")+"/") {
					return true
				}
			}
		}
	}
	return false
}

func afmIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// ── 现读 A：步表 ─────────────────────────────────────────────────────
func afmGitHead(root string) (string, bool) {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", false
	}
	s := strings.TrimSpace(string(out))
	if len(s) > 16 {
		s = s[:16]
	}
	return s, true
}

func afmFingerprint(root string, steps []afmStep, cov, newcmd afmJV) string {
	var buf []byte
	for _, st := range steps {
		buf = append(buf, []byte(fmt.Sprintf("%s|%s|%s|%s|%s\n", st.scope, st.name, st.mode, st.dir, st.cmd))...)
		for _, sc := range afmCollectNamedScripts(st.cmd) {
			p := filepath.Join(root, sc)
			if b, err := os.ReadFile(p); err == nil {
				buf = append(buf, afmSHA256(b)...)
			}
		}
	}
	buf = append(buf, []byte(afmDump(cov, 0, true, 0))...)
	buf = append(buf, []byte(afmDump(newcmd, 0, true, 0))...)
	// ★ GAP-20261005-51 甲案：入键**不含 git HEAD** —— 否则「emit 时压 HEAD、emit 后一提交 HEAD 即变」
	//   ⇒ 冻结点自指、任一提交必过期、无不动点（与 py check-affected-map.py:340-353 同步）。
	return afmHex(afmSHA256(buf))
}

// afmBuildMap —— py `build_map`。返回 (map, ok)；ok=false ⇒ 走 py 的未捕获异常档（rc=1，正文不同）。
func afmBuildMap(root string) (afmJV, bool) {
	stepText, err := afmReadText(root, afmSrcStepTbl)
	if err != nil {
		return afmJV{}, false
	}
	cov, err := afmLoadJSON(root, afmSrcCoverage)
	if err != nil {
		return afmJV{}, false
	}
	newcmd, err := afmLoadJSON(root, afmSrcNewcmd)
	if err != nil {
		return afmJV{}, false
	}
	steps, ok := afmParseStepTable(stepText)
	if !ok {
		return afmJV{}, false
	}

	// faces: 面 → 该面的步名（按步表工作目录现读分组）。
	faces := map[string][]string{}
	for _, st := range steps {
		f, okf := afmFaceOf(st.dir)
		if !okf {
			return afmJV{}, false // py `sorted(faces)` 里 None 会 TypeError（未捕获 ⇒ rc=1）
		}
		faces[f] = append(faces[f], st.name)
	}
	faceKeys := make([]string, 0, len(faces))
	for k := range faces {
		faceKeys = append(faceKeys, k)
	}
	sort.Strings(faceKeys)

	type afmEntry struct {
		pattern, kind, face string
		steps               []string
		evidence            string
	}
	merged := map[string]*afmEntry{}
	order := []string{}
	put := func(e afmEntry) {
		if cur, hit := merged[e.pattern]; hit {
			cur.steps = afmUnionSorted(cur.steps, e.steps)
			cur.evidence = cur.evidence + " ／ " + e.evidence
			return
		}
		cp := e
		merged[e.pattern] = &cp
		order = append(order, e.pattern)
	}

	// 面型（非 repo）：目录前缀 ⇒ 该面全部步。
	for _, f := range faceKeys {
		if f == "repo" {
			continue
		}
		put(afmEntry{
			pattern: f + "/**", kind: "prefix", face: f,
			steps:    afmSortedCopy(faces[f]),
			evidence: fmt.Sprintf("步表工作目录 = ${REPO_ROOT}/%s（%d 步）", f, len(faces[f])),
		})
	}

	repoSteps := []afmStep{}
	for _, st := range steps {
		if f, okf := afmFaceOf(st.dir); okf && f == "repo" {
			repoSteps = append(repoSteps, st)
		}
	}
	byScript := map[string][]string{}
	for _, st := range repoSteps {
		for _, sc := range afmCollectNamedScripts(st.cmd) {
			byScript[sc] = append(byScript[sc], st.name)
		}
	}
	scKeys := make([]string, 0, len(byScript))
	for k := range byScript {
		scKeys = append(scKeys, k)
	}
	sort.Strings(scKeys)
	for _, sc := range scKeys {
		u := afmUnionSorted(nil, byScript[sc])
		put(afmEntry{
			pattern: sc, kind: "literal", face: "repo",
			steps:    u,
			evidence: fmt.Sprintf("步命令按名点到（%d 步）", len(u)),
		})
	}

	// 件型（repo 面）：覆盖面登记面 —— whitelist.scripts 的件 ⇒ 门①。
	gate1 := []string{}
	for _, n := range faces["repo"] {
		if strings.Contains(n, "check-gate-coverage.py") {
			gate1 = append(gate1, n)
		}
	}
	sort.Strings(gate1)
	if wl, okw := cov.get("whitelist"); okw {
		if ws, ok2 := wl.get("scripts"); ok2 && ws.kind == 'a' {
			for _, w := range ws.arr {
				p := ""
				if pv, ok3 := w.get("path"); ok3 {
					p = pv.pyStr()
				}
				put(afmEntry{
					pattern: p, kind: "literal", face: "repo",
					steps:    afmSortedCopy(gate1),
					evidence: "gate-coverage.config.json whitelist.scripts 登记面（门① 覆盖面判据）",
				})
			}
		}
	}
	put(afmEntry{
		pattern: afmSrcCoverage, kind: "literal", face: "repo",
		steps:    afmSortedCopy(gate1),
		evidence: "门① 覆盖面配置真源",
	})

	// 件型（repo 面）：命令面真源 ⇒ 读它的门步。
	cliSteps := []string{}
	readerSet := map[string]bool{}
	for _, s := range afmCLIReaderScripts {
		readerSet[s] = true
	}
	for _, st := range repoSteps {
		for _, sc := range afmCollectNamedScripts(st.cmd) {
			if readerSet[sc] {
				cliSteps = append(cliSteps, st.name)
			}
		}
	}
	cliSteps = afmUnionSorted(nil, cliSteps)
	ncCount := -1
	if v, okv := newcmd.get("count"); okv && v.kind == 'n' {
		fmt.Sscanf(v.num, "%d", &ncCount)
	}
	for _, tf := range afmCLITruthFiles {
		put(afmEntry{
			pattern: tf, kind: "literal", face: "repo",
			steps: afmSortedCopy(cliSteps),
			evidence: fmt.Sprintf("命令树 path 全集真源（newcmd-paths-baseline.json count=%d）⇒ %d 门步读它",
				ncCount, len(cliSteps)),
		})
	}
	ncSteps := []string{}
	for _, n := range faces["repo"] {
		if strings.Contains(n, "check-newcmd-followup.py") {
			ncSteps = append(ncSteps, n)
		}
	}
	put(afmEntry{
		pattern: afmSrcNewcmd, kind: "literal", face: "repo",
		steps:    afmSortedCopy(afmUnionSorted(nil, ncSteps)),
		evidence: "命令面随动冻结基线件（门78 读它）",
	})

	// 悬空过滤：literal 指不到的件不当声明。
	dropped := []string{}
	kept := []string{}
	for _, k := range order {
		e := merged[k]
		if e.kind == "literal" && !afmIsFile(filepath.Join(root, e.pattern)) {
			dropped = append(dropped, e.pattern)
			continue
		}
		kept = append(kept, k)
	}
	sort.Strings(kept)
	sort.Strings(dropped)
	entries := []afmJV{}
	declared := map[string]bool{}
	for _, k := range kept {
		e := merged[k]
		entries = append(entries, afmObj(
			afmKP{afmK("pattern"), afmStr(e.pattern)},
			afmKP{afmK("kind"), afmStr(e.kind)},
			afmKP{afmK("face"), afmStr(e.face)},
			afmKP{afmK("steps"), afmStrArr(e.steps)},
			afmKP{afmK("evidence"), afmStr(e.evidence)},
		))
		if e.kind == "literal" {
			declared[e.pattern] = true
		}
	}

	// 未声明面：脚本候选集里零命名者单列。
	scanGlobs := []string{}
	if sc, oks := cov.get("scan"); oks {
		if g, ok2 := sc.get("script_globs"); ok2 && g.kind == 'a' {
			for _, x := range g.arr {
				scanGlobs = append(scanGlobs, x.pyStr())
			}
		}
	}
	undeclared := []string{}
	for _, g := range scanGlobs {
		for _, p := range afmPyGlob(root, g) {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				continue
			}
			if afmSelfExcluded(rel) {
				continue
			}
			if afmIsFile(filepath.Join(root, rel)) && !declared[rel] {
				undeclared = append(undeclared, rel)
			}
		}
	}
	sample := undeclared
	if len(sample) > 20 {
		sample = sample[:20]
	}

	faceOut := []afmKP{}
	for _, f := range faceKeys {
		faceOut = append(faceOut, afmKP{f, afmStrArr(afmSortedCopy(faces[f]))})
	}
	head := afmNull()
	if h, okh := afmGitHead(root); okh {
		head = afmStr(h)
	}
	return afmObj(
		afmKP{afmK("schema"), afmStr("affected-map/1")},
		afmKP{afmK("_口径"), afmStr("声明表（件→步/面 映射真源）· 由 `python3 scripts/gates/check-affected-map.py --emit` " +
			"**现读生成**（A 步表 ＋ B gate-coverage.config.json ＋ C newcmd-paths-baseline.json）· " +
			"✗ 手敲。未声明件按已拍「默认每轮都跑」处置（本表 files 未覆盖者即未声明）。")},
		afmKP{afmK("source_head"), head},
		afmKP{afmK("input_fingerprint"), afmStr(afmFingerprint(root, steps, cov, newcmd))},
		afmKP{afmK("_fingerprint_note"), afmStr("入键 = 步表文本 ＋ 每个按名点到脚本件的 sha256 ＋ gate-coverage.config.json " +
			"＋ newcmd-paths-baseline.json（内容相对 · ✗ 不含 git HEAD ⇒ 无冻结点自指 · 只有被管辖输入变才过期）。" +
			"取不到 ⇒ 下游 affected-steps.py 报「不给结论」rc=2。")},
		afmKP{afmK("faces"), afmObj(faceOut...)},
		afmKP{afmK("files"), afmArr(entries...)},
		afmKP{afmK("undeclared"), afmObj(
			afmKP{afmK("note"), afmStr("脚本候选集（gate-coverage.config.json scan.script_globs 现读展开）里**零命名**者 ⇒ 未声明面；" +
				"按已拍第 3 条默认每轮都跑。✗ 不得读作已声明。")},
			afmKP{afmK("count"), afmInt(len(undeclared))},
			afmKP{afmK("sample_first20"), afmStrArr(sample)},
		)},
		afmKP{afmK("dropped_ghosts"), afmStrArr(dropped)},
		afmKP{afmK("_dropped_ghosts_note"), afmStr("现读来源点到、但盘上查无的件（✗ 不当声明 ⇒ 不冒「表里有件盘上无」）。")},
	), true
}

func afmSortedCopy(xs []string) []string {
	out := append([]string{}, xs...)
	sort.Strings(out)
	return out
}

func afmUnionSorted(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	for _, x := range b {
		set[x] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ── 双向差集机检 ─────────────────────────────────────────────────────
// afmDiskFiles —— 盘上件 = 文件系统现走（✗ 不经 git ls-files）。
func afmDiskFiles(root string, cov afmJV) ([]string, error) {
	comps := map[string]bool{}
	if pc, ok := cov.get("exclude"); ok {
		if v, ok2 := pc.get("path_components"); ok2 && v.kind == 'a' {
			for _, x := range v.arr {
				comps[x.pyStr()] = true
			}
		}
	}
	out := []string{}
	var walk func(abs, rel string) error
	walk = func(abs, rel string) error {
		ents, err := os.ReadDir(abs)
		if err != nil {
			return err
		}
		for _, e := range ents {
			childRel := e.Name()
			if rel != "" {
				childRel = rel + "/" + e.Name()
			}
			if e.IsDir() {
				if comps[e.Name()] {
					continue
				}
				if err := walk(filepath.Join(abs, e.Name()), childRel); err != nil {
					return err
				}
				continue
			}
			if afmExcluded(childRel, cov) {
				continue
			}
			if afmSelfExcluded(childRel) {
				continue
			}
			out = append(out, childRel)
		}
		return nil
	}
	if err := walk(root, ""); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// afmFnmatch —— py `fnmatch.fnmatch` 的 `*` “ `?` `[seq]` `[!seq]`（`*` 跨 `/`）。
func afmFnmatch(name, pat string) bool {
	var b strings.Builder
	b.WriteString("(?s)^")
	i := 0
	for i < len(pat) {
		c := pat[i]
		switch c {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '[':
			j := i + 1
			if j < len(pat) && (pat[j] == '!' || pat[j] == '^') {
				j++
			}
			if j < len(pat) && pat[j] == ']' {
				j++
			}
			for j < len(pat) && pat[j] != ']' {
				j++
			}
			if j >= len(pat) {
				b.WriteString(regexp.QuoteMeta("["))
			} else {
				inner := pat[i+1 : j]
				if strings.HasPrefix(inner, "!") {
					inner = "^" + inner[1:]
				}
				b.WriteString("[" + strings.ReplaceAll(inner, "\\", "\\\\") + "]")
				i = j
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
		i++
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

// afmPyGlob —— py `glob.glob(os.path.join(root, pattern))`：
// `*` 不跨 `/` · `*`/`?`/`[` 打头的段**不匹配**前导 `.` 的条目 · 结果 sorted。
func afmPyGlob(root, pattern string) []string {
	segs := strings.Split(pattern, "/")
	if len(segs) < 1 {
		return nil
	}
	base := segs[len(segs)-1]
	dirParts := segs[:len(segs)-1]
	hasWild := strings.ContainsAny(base, "*?[")
	if !hasWild {
		p := filepath.Join(append([]string{root}, segs...)...)
		if _, err := os.Lstat(p); err == nil {
			return []string{p}
		}
		return nil
	}
	dir := filepath.Join(append([]string{root}, dirParts...)...)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue
		}
		if afmFnmatch(name, base) {
			out = append(out, filepath.Join(dir, name))
		}
	}
	sort.Strings(out)
	return out
}

func afmEntryMatches(e afmJV, rel string) bool {
	pat := ""
	if p, ok := e.get("pattern"); ok {
		pat = p.pyStr()
	}
	kind := ""
	if k, ok := e.get("kind"); ok {
		kind = k.pyStr()
	}
	if kind == "prefix" {
		base := pat
		if strings.HasSuffix(pat, "/**") {
			base = pat[:len(pat)-3]
		}
		return rel == base || strings.HasPrefix(rel, strings.TrimRight(base, "/")+"/")
	}
	if kind == "glob" {
		return afmFnmatch(rel, pat)
	}
	return rel == pat
}

type afmRep struct {
	reason   string
	class1   []string
	class2   []string
	declared int
	both     int
}

// afmCheckMap —— py `check_map` 纯函数（返回 rc, rep）。
func afmCheckMap(mp afmJV, filesOnDisk []string) (int, afmRep) {
	empty := afmRep{class1: []string{}, class2: []string{}}
	if mp.kind != 'o' {
		return 1, afmRep{reason: "缺读数：声明表缺 `files`（空/坏件）⇒ fail-closed", class1: []string{}, class2: []string{}}
	}
	fv, hasFiles := mp.get("files")
	if !hasFiles {
		return 1, afmRep{reason: "缺读数：声明表缺 `files`（空/坏件）⇒ fail-closed", class1: []string{}, class2: []string{}}
	}
	if !fv.truthy() {
		return 1, afmRep{reason: "缺读数：声明表 `files` 为空 ⇒ 无鉴别力 ⇒ fail-closed", class1: []string{}, class2: []string{}}
	}
	if fv.kind != 'a' {
		// py 会走 `entry['pattern']` ⇒ TypeError（未捕获 ⇒ rc=1，正文不同）。
		return 1, afmRep{reason: "缺读数：声明表 `files` 形态非列表 ⇒ fail-closed", class1: []string{}, class2: []string{}}
	}
	entries := fv.arr
	matched := make([]int, len(entries))
	covered := map[string]bool{}
	for _, rel := range filesOnDisk {
		for i, e := range entries {
			if afmEntryMatches(e, rel) {
				matched[i]++
				covered[rel] = true
			}
		}
	}
	_ = empty
	class1 := []string{}
	for _, r := range filesOnDisk {
		if !covered[r] {
			class1 = append(class1, r)
		}
	}
	class2 := []string{}
	both := 0
	for i := range entries {
		pat := ""
		if p, ok := entries[i].get("pattern"); ok {
			pat = p.pyStr()
		}
		if matched[i] == 0 {
			class2 = append(class2, pat)
		} else {
			both++
		}
	}
	rep := afmRep{reason: "", class1: class1, class2: class2, declared: len(entries), both: both}
	rc := 0
	if len(class2) > 0 {
		rc = 1
	}
	return rc, rep
}

// ── 步面格（拍乙 · D9 步面接线）：**复甲① C0 唯一实现** ──────────────────────
// ★ py 经 importlib 现载邻件 `affected-steps.py` 的 `c0_step_face()`（C0 唯一真源）；
//   Go 无「运行时载入 .py」这条路 ⇒ 本件逐字复刻为 `afmC0StepFace`，并复用包级助手
//   `asSameSet`/`asFirst`/`asReprSlice`（同 `package main` · 见件头未覆盖面⑩）。

// afmDeclaredFaceNames —— py `declared_face_names(mp)`：`faces` 展平**去重**后的步名表。
func afmDeclaredFaceNames(mp afmJV) []string {
	named := []string{}
	seen := map[string]bool{}
	faces, ok := mp.get("faces")
	if !ok || faces.kind != 'o' {
		return named
	}
	faceKeys := make([]string, 0, len(faces.obj))
	for _, p := range faces.obj {
		faceKeys = append(faceKeys, p.k)
	}
	sort.Strings(faceKeys)
	for _, fk := range faceKeys {
		fv, _ := faces.get(fk)
		if fv.kind != 'a' {
			continue
		}
		for _, n := range fv.arr {
			s := n.pyStr()
			if !seen[s] {
				seen[s] = true
				named = append(named, s)
			}
		}
	}
	return named
}

// afmC0StepFace —— py `c0_step_face()`（**唯一真源**）：`D==N` ＋ 命名集合逐字相等。
func afmC0StepFace(tableNames, faceNames []string) (bool, string) {
	N, D := len(tableNames), len(faceNames)
	tset := map[string]bool{}
	for _, t := range tableNames {
		tset[t] = true
	}
	dset := map[string]bool{}
	for _, t := range faceNames {
		dset[t] = true
	}
	if D != N {
		return false, fmt.Sprintf("C0 未过：声明命名 %d 步 ≠ 步骤表 %d 步 ⇒ **有步未声明 = 静默漏选**", D, N)
	}
	if !asSameSet(dset, tset) {
		miss := []string{}
		for t := range tset {
			if !dset[t] {
				miss = append(miss, t)
			}
		}
		ghost := []string{}
		for t := range dset {
			if !tset[t] {
				ghost = append(ghost, t)
			}
		}
		sort.Strings(miss)
		sort.Strings(ghost)
		return false, fmt.Sprintf("C0 未过：命名集合 ≠ 表集合（漏=%s · 幻影=%s）",
			asReprSlice(asFirst(miss, 5)), asReprSlice(asFirst(ghost, 5)))
	}
	return true, fmt.Sprintf("C0 双向计数过：D==N==%d 且命名集合逐字相等（**没少声明**）", N)
}

// afmCheckStepFace —— py `check_step_face()`：0 步面齐 · 3 步面漂移。
// （py 的 `mod is None ⇒ rc=2` 一支在 Go 无对应态 —— 真源已编译在内 · 见件头未覆盖面⑩。）
func afmCheckStepFace(tableNames []string, mp afmJV) (int, string) {
	ok, detail := afmC0StepFace(tableNames, afmDeclaredFaceNames(mp))
	if ok {
		return 0, detail
	}
	return 3, detail
}

// afmCmdCheck —— py `cmd_check`。
func afmCmdCheck(stdout io.Writer, root, mapPath string) int {
	if !afmIsFile(mapPath) {
		fmt.Fprintf(stdout, "[缺读数] 声明表不在：%s ⇒ rc=1（fail-closed）\n", mapPath)
		return 1
	}
	raw, err := os.ReadFile(mapPath)
	if err != nil {
		fmt.Fprintf(stdout, "[缺读数] 声明表读不出：%s ⇒ rc=1\n", err.Error())
		return 1
	}
	mp, jerr := afmLoadBytes(raw)
	if jerr != nil {
		fmt.Fprintf(stdout, "[缺读数] 声明表读不出：%s ⇒ rc=1\n", jerr.Error())
		return 1
	}
	cov, cerr := afmLoadJSON(root, afmSrcCoverage)
	if cerr != nil {
		// py 此处**无 try** ⇒ 未捕获异常（traceback · rc=1）；本件 rc 同、正文不同（见未覆盖面③）。
		return 1
	}
	files, derr := afmDiskFiles(root, cov)
	if derr != nil {
		fmt.Fprintf(stdout, "[缺读数] git ls-files 取不到：%s ⇒ rc=1（fail-closed）\n", derr.Error())
		return 1
	}
	rc, rep := afmCheckMap(mp, files)
	// ★ 步面格（拍乙 · D9）：与件面格（① ②）**分列各算**（✗ 不混同一 rc）。
	sfRC := 2
	sfDetail := ""
	if rawStep, terr := afmReadText(root, afmSrcStepTbl); terr != nil {
		sfRC, sfDetail = 2, fmt.Sprintf("步骤表取不到：%s ⇒ 判不了", terr.Error())
	} else if steps, ok := afmParseStepTable(rawStep); !ok {
		sfRC, sfDetail = 2, "步骤表取不到：unterminated quoted string ⇒ 判不了"
	} else {
		tableNames := make([]string, 0, len(steps))
		for _, s := range steps {
			tableNames = append(tableNames, s.name)
		}
		if len(tableNames) == 0 {
			sfRC, sfDetail = 2, "步骤表解析 0 步（空转 ⇒ 判不了）"
		} else {
			sfRC, sfDetail = afmCheckStepFace(tableNames, mp)
		}
	}
	fmt.Fprintf(stdout, "声明表：%s\n", mapPath)
	// py `mp.get(k)` 缺键 ⇒ None ⇒ `%s` 打 "None"（✗ 不是空串）。
	sch, shh, fph := afmNull(), afmNull(), afmNull()
	if v, ok := mp.get("schema"); ok {
		sch = v
	}
	if v, ok := mp.get("source_head"); ok {
		shh = v
	}
	if v, ok := mp.get("input_fingerprint"); ok {
		fph = v
	}
	fp := fph.pyStr()
	if len(fp) > 16 {
		fp = fp[:16]
	}
	fmt.Fprintf(stdout, "  schema=%s · source_head=%s · 指纹=%s\n", sch.pyStr(), shh.pyStr(), fp)
	fmt.Fprintf(stdout, "  声明条目 %d（both=%d）· 盘上件（已按 exclude 过滤）%d\n", rep.declared, rep.both, len(files))
	if rep.reason != "" {
		fmt.Fprintf(stdout, "  [缺读数] %s ⇒ rc=1\n", rep.reason)
		return rc
	}
	fmt.Fprintf(stdout, "  ① 盘上有件但表里没声（未声明 ⇒ 默认每轮都跑）：%d 条\n", len(rep.class1))
	for _, x := range afmHead(rep.class1, 10) {
		fmt.Fprintf(stdout, "      - %s\n", x)
	}
	fmt.Fprintf(stdout, "  ② 表里有件盘上无（悬空声明 ⇒ 红）：%d 条\n", len(rep.class2))
	for _, x := range afmHead(rep.class2, 10) {
		fmt.Fprintf(stdout, "      - %s\n", x)
	}
	sfWord := "判不了"
	if sfRC == 0 {
		sfWord = "绿"
	} else if sfRC == 3 {
		sfWord = "红"
	}
	fmt.Fprintf(stdout, "  ③ 步面（件面＋步面 · D==N ＋ 逐字相等 · 复甲① C0 唯一实现）：%s · %s\n", sfWord, sfDetail)
	// ★ 两格**分列各算**（✗ 不混同一 rc）：步面漂移 rc=3 · 悬空声明 rc=1 · 步面判不了 rc=2。
	outRC := rc
	if sfRC == 2 {
		outRC = 2
	} else if sfRC == 3 {
		outRC = 3
	}
	fmt.Fprintf(stdout, "  rc=%d（0 读回成功 · 1 悬空声明/缺读数 · 3 步面漂移 · 2 判不了）\n", outRC)
	return outRC
}

func afmHead(xs []string, n int) []string {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

// ── 成对负控（py `self_test` · 10 控）──────────────────────────────────
func afmSelfTest(stdout io.Writer) int {
	fails := []string{}
	tmp, err := os.MkdirTemp("", "affected-map-selftest-")
	if err != nil {
		fmt.Fprintln(stdout, "自检不过：")
		fmt.Fprintf(stdout, "  ✗ %s\n", err.Error())
		return 2
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	disk := []string{"core/a.go", "core/b.go", "scripts/gates/x.py"}
	litX := "scripts/gates/" + "x.py"
	m1 := afmObj(
		afmKP{afmK("files"), afmArr(
			afmObj(afmKP{afmK("pattern"), afmStr("core/**")}, afmKP{afmK("kind"), afmStr("prefix")}),
			afmObj(afmKP{afmK("pattern"), afmStr(litX)}, afmKP{afmK("kind"), afmStr("literal")}),
		)},
	)
	// ① 正控：声明齐 ⇒ rc=0 · both=2。
	if rc, rep := afmCheckMap(m1, disk); rc != 0 {
		fails = append(fails, fmt.Sprintf("① 正控期望 rc=0，实得 %d", rc))
	} else if len(rep.class1) > 0 || len(rep.class2) > 0 || rep.both != 2 {
		fails = append(fails, fmt.Sprintf("① 正控差集/计数不对：%d/%d/%d", len(rep.class1), len(rep.class2), rep.both))
	}
	// ② 负控：盘上多一件没声明 ⇒ class1 非空（≠红，rc 仍 0）。
	if rc, rep := afmCheckMap(m1, append(append([]string{}, disk...), "docs/extra.md")); rc != 0 {
		fails = append(fails, fmt.Sprintf("② 未声明件期望 rc=0（未声明是常态），实得 %d", rc))
	} else if len(rep.class1) != 1 || rep.class1[0] != "docs/extra.md" {
		fails = append(fails, fmt.Sprintf("② 未点名「盘上有件但表里没声」：%v", rep.class1))
	}
	// ③ 负控：悬空声明 ⇒ rc=1 点名叫出来。
	ghost := "scripts/gates/" + "ghost.py"
	m3 := afmObj(
		afmKP{afmK("files"), afmArr(
			afmObj(afmKP{afmK("pattern"), afmStr("core/**")}, afmKP{afmK("kind"), afmStr("prefix")}),
			afmObj(afmKP{afmK("pattern"), afmStr(ghost)}, afmKP{afmK("kind"), afmStr("literal")}),
		)},
	)
	if rc, rep := afmCheckMap(m3, disk); rc != 1 {
		fails = append(fails, fmt.Sprintf("③ 悬空声明期望 rc=1，实得 %d", rc))
	} else if len(rep.class2) != 1 || rep.class2[0] != ghost {
		fails = append(fails, fmt.Sprintf("③ 未点名悬空声明：%v", rep.class2))
	}
	// ④ 负控：空 files ⇒ rc=1（无鉴别力，✗ 不许空转当绿）。
	if rc, _ := afmCheckMap(afmObj(afmKP{afmK("files"), afmArr()}), disk); rc != 1 {
		fails = append(fails, fmt.Sprintf("④ 空表期望 rc=1，实得 %d", rc))
	}
	// ⑤ 负控：缺 files（坏件）⇒ rc=1。
	if rc, _ := afmCheckMap(afmObj(afmKP{afmK("schema"), afmStr("affected-map/1")}), disk); rc != 1 {
		fails = append(fails, fmt.Sprintf("⑤ 坏件期望 rc=1，实得 %d", rc))
	}
	// ⑥ 负控：声明表件不在 ⇒ 命令档 rc=1（真起一次进程，不走纯函数）。
	if rc := afmCmdCheck(stdout, filepath.Dir(tmp), filepath.Join(tmp, "no-such-map.json")); rc != 1 {
		fails = append(fails, fmt.Sprintf("⑥ 缺件期望 rc=1，实得 %d", rc))
	}
	// ⑦–⑩ 端到端（真起 afmCmdCheck · 走 ③ 步面格 · 复甲① C0 唯一实现）。
	root := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(filepath.Join(root, "core"), 0o755); err != nil {
		fails = append(fails, "⑦ 夹具 core 目录建不出："+err.Error())
	}
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		fails = append(fails, "⑦ 夹具 scripts/gates 目录建不出："+err.Error())
	}
	_ = os.WriteFile(filepath.Join(root, "core", "a.go"), []byte("package main\n"), 0o644)
	stepTbl := "DEFAULT_SCOPES=(\"go\")\n" +
		"add_step go \"a-step\" rc \"${REPO_ROOT}/core\" \"go build ./...\"\n" +
		"add_step gates \"b-step\" tri \"${REPO_ROOT}\" \"bash scripts/gates/x.sh\"\n" +
		"add_step gates \"c-step\" tri \"${REPO_ROOT}\" \"bash scripts/gates/y.sh\"\n"
	_ = os.WriteFile(filepath.Join(root, "scripts", "gates", "precommit-gates.sh"), []byte(stepTbl), 0o644)
	covTxt := "{\"scan\":{\"script_globs\":[\"scripts/gates/*.py\"]}," +
		"\"exclude\":{\"path_components\":[\".git\"],\"prefixes\":[]},\"whitelist\":{\"scripts\":[]}}"
	_ = os.WriteFile(filepath.Join(root, "scripts", "gates", "gate-coverage.config.json"), []byte(covTxt), 0o644)

	diskOK := afmArr(afmObj(afmKP{afmK("pattern"), afmStr("core/**")}, afmKP{afmK("kind"), afmStr("prefix")}))
	facesOK := afmObj(
		afmKP{afmK("core"), afmStrArr([]string{"a-step"})},
		afmKP{afmK("repo"), afmStrArr([]string{"b-step", "c-step"})})
	wmap := func(name string, files, faces afmJV) string {
		mp := afmObj(
			afmKP{afmK("schema"), afmStr("affected-map/1")},
			afmKP{afmK("input_fingerprint"), afmStr(strings.Repeat("f", 64))},
			afmKP{afmK("source_head"), afmStr("deadbeef")},
			afmKP{afmK("files"), files},
			afmKP{afmK("faces"), faces},
		)
		p := filepath.Join(tmp, name)
		_ = os.WriteFile(p, []byte(afmDump(mp, 0, false, 0)), 0o644)
		return p
	}
	q := func(mapPath string) int { return afmCmdCheck(io.Discard, root, mapPath) }
	// ⑦ 正控：步面齐（D==N 且逐字相等）⇒ rc=0（③ 绿）。
	if rc := q(wmap("m7.json", diskOK, facesOK)); rc != 0 {
		fails = append(fails, fmt.Sprintf("⑦ 步面齐正控期望 rc=0，实得 %d", rc))
	}
	// ⑧ 负控：删 faces 一步（D≠N）⇒ rc=3（**步面格 · ≠ 悬空声明的 1**）。
	if rc := q(wmap("m8.json", diskOK, afmObj(
		afmKP{afmK("core"), afmStrArr([]string{"a-step"})},
		afmKP{afmK("repo"), afmStrArr([]string{"b-step"})}))); rc != 3 {
		fails = append(fails, fmt.Sprintf("⑧ 步面漂移(D≠N)期望 rc=3，实得 %d", rc))
	}
	// ⑨ 负控：D==N 但集合不等（换名）⇒ rc=3。
	if rc := q(wmap("m9.json", diskOK, afmObj(
		afmKP{afmK("core"), afmStrArr([]string{"a-step"})},
		afmKP{afmK("repo"), afmStrArr([]string{"b-step", "zzz"})}))); rc != 3 {
		fails = append(fails, fmt.Sprintf("⑨ 步面集合不等期望 rc=3，实得 %d", rc))
	}
	// ⑩ 负控：悬空声明 ⇒ rc=1（**≠ 步面漂移的 3** · 两格 rc 分列不混同 · 有牙）。
	ghostLit := "scripts/gates/" + "ghost.py"
	diskGhost := afmArr(
		afmObj(afmKP{afmK("pattern"), afmStr("core/**")}, afmKP{afmK("kind"), afmStr("prefix")}),
		afmObj(afmKP{afmK("pattern"), afmStr(ghostLit)}, afmKP{afmK("kind"), afmStr("literal")}))
	if rc := q(wmap("m10.json", diskGhost, facesOK)); rc != 1 {
		fails = append(fails, fmt.Sprintf("⑩ 悬空声明期望 rc=1（✗ 不与步面 3 混同），实得 %d", rc))
	}
	if len(fails) > 0 {
		fmt.Fprintln(stdout, "自检不过：")
		for _, x := range fails {
			fmt.Fprintln(stdout, "  ✗ "+x)
		}
		return 2
	}
	fmt.Fprintln(stdout, "自检通过：10 控（正控 2 ＋ 负控 8）逐条对拍 ✓")
	return 0
}

// ── 默认根（声明 (a)）────────────────────────────────────────────────
func afmDefaultRoot() string {
	start, err := os.Getwd()
	if err != nil {
		return "."
	}
	d, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	cur := d
	for {
		fi, e := os.Stat(filepath.Join(cur, "scripts", "gates"))
		if e == nil && fi.IsDir() {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return d
}

// ── argparse 子集（精确长旗标 + `=` 取值 + 缺值/未知报话；✗ 不实现前缀缩写）──
type afmArgs struct {
	emit, check, selfTest, help bool
	mp, out, root               string
}

func afmArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, afmUsageHead)
	fmt.Fprintf(stderr, "%s: error: %s\n", afmProg, msg)
	return 2
}

func afmParseArgs(stdout, stderr io.Writer, argv []string) (afmArgs, int) {
	a := afmArgs{}
	var unknown []string
	take := func(i int, body string, val string, hasVal bool) (string, int, bool) {
		if hasVal {
			return val, i, true
		}
		if i+1 >= len(argv) {
			return "", i, false
		}
		return argv[i+1], i + 1, true
	}
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		switch {
		case tok == "-h" || tok == "--help":
			fmt.Fprint(stdout, afmHelp)
			return a, 0
		case tok == "--emit":
			a.emit = true
		case tok == "--check":
			a.check = true
		case tok == "--self-test":
			a.selfTest = true
		case strings.HasPrefix(tok, "--map"):
			body, val, hasVal := tok[2:], "", false
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				body, val, hasVal = tok[2:eq], tok[eq+1:], true
			}
			if body != "map" {
				unknown = append(unknown, tok)
				continue
			}
			v, ni, ok := take(i, body, val, hasVal)
			if !ok {
				return a, afmArgErr(stderr, "argument --map: expected one argument")
			}
			i = ni
			a.mp = v
		case strings.HasPrefix(tok, "--out"):
			body, val, hasVal := tok[2:], "", false
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				body, val, hasVal = tok[2:eq], tok[eq+1:], true
			}
			if body != "out" {
				unknown = append(unknown, tok)
				continue
			}
			v, ni, ok := take(i, body, val, hasVal)
			if !ok {
				return a, afmArgErr(stderr, "argument --out: expected one argument")
			}
			i = ni
			a.out = v
		case strings.HasPrefix(tok, "--root"):
			body, val, hasVal := tok[2:], "", false
			if eq := strings.IndexByte(tok, '='); eq >= 0 {
				body, val, hasVal = tok[2:eq], tok[eq+1:], true
			}
			if body != "root" {
				unknown = append(unknown, tok)
				continue
			}
			v, ni, ok := take(i, body, val, hasVal)
			if !ok {
				return a, afmArgErr(stderr, "argument --root: expected one argument")
			}
			i = ni
			a.root = v
		default:
			unknown = append(unknown, tok)
		}
	}
	if len(unknown) > 0 {
		return a, afmArgErr(stderr, "unrecognized arguments: "+strings.Join(unknown, " "))
	}
	return a, -1
}

// ── 入口（= py `main()`）─────────────────────────────────────────────
func checkAffectedMapGateMain(stdout, stderr io.Writer, argv []string) int {
	a, rc := afmParseArgs(stdout, stderr, argv)
	if rc >= 0 {
		return rc
	}
	if a.selfTest {
		return afmSelfTest(stdout)
	}
	root := a.root
	if root == "" {
		root = afmDefaultRoot()
	}
	if a.emit {
		mp, ok := afmBuildMap(root)
		if !ok {
			// py 在 --emit 档抛未捕获异常（rc=1）⇒ 本件同 rc、正文不同（未覆盖面③）。
			return 1
		}
		out := a.out
		if out == "" {
			out = filepath.Join(root, afmMapDefault)
		}
		text := afmDump(mp, 1, false, 0) + "\n"
		if a.out == "-" {
			fmt.Fprint(stdout, text)
			return 0
		}
		if err := os.WriteFile(out, []byte(text), 0o644); err != nil {
			fmt.Fprintf(stderr, "%s: error: %s\n", afmProg, err.Error())
			return 1
		}
		fv, _ := mp.get("files")
		uv, _ := mp.get("undeclared")
		cnt := 0
		if uv.kind == 'o' {
			if c, ok := uv.get("count"); ok {
				fmt.Sscanf(c.num, "%d", &cnt)
			}
		}
		dg, _ := mp.get("dropped_ghosts")
		fmt.Fprintf(stdout, "已生成声明表：%s（声明条目 %d · 未声明脚本件 %d · 悬空过滤 %d）\n",
			out, len(fv.arr), cnt, len(dg.arr))
		return 0
	}
	if a.check {
		mp := a.mp
		if mp == "" {
			mp = filepath.Join(root, afmMapDefault)
		}
		return afmCmdCheck(stdout, root, mp)
	}
	fmt.Fprint(stderr, afmUsageLine)
	return 2
}
