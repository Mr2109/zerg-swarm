// check_fields_face_parity.go —— 判据件 `scripts/gates/check-fields-face-parity.py`（213 行 · C 类可对拍）的
// **Go 真搬**（M2 一枚 · 承接 `GAP-20261001-13` · 入口 `checkFieldsFaceParityGateMain`）。
//
// 为什么有这件：
//
//	M2 口径（甲案）：判据件允许**真跑子进程 / 吃制品**，条件 = **输入确定 ⇒ 输出可逐字对拍**。
//	本件即该档第一枚：它不吃仓内文件内容当判据，而是**现跑制品** `bin/zerg <path…> --json <坏名>`
//	取「真身字段表」，再与命令树声明面逐名对拍。⇒ 本件**唯一外因 = 制品 `bin/zerg`**。
//	对拍把制品**只读复制到仓外**（两态共用同一份副本），并在回执里报其 sha256 前16 当制品指纹。
//
// 本件与「外部树改显式入参旋钮」：
//
//	py 的入参面**本来就有**这些旋钮，本件**照抄同名**，✗ 不另起名、✗ 不新造 env：
//	  位置参数 `root`（仓根）· `--bin`（制品路径）· `--min-covered`（棘轮）· `--timeout`（单命令秒数）
//	  · `--json`（机器读面）· `--self-test`（成对负控自检）。
//	  ① 旋钮缺省值 = py 默认推导**逐字同构**（见 (a) 的逐字对比）。
//	  ② **退 2 边界**：本件**一棵**外部树（仓根；`main.go` 与 `bin/` 都挂在它下面）＋**一个**制品
//	     ⇒ 新口径的「都取不到才退 2」在本件**退化为单棵判定**，而本门 py 原件的退码档**本来就是 2**
//	     （缺件 / 制品不在 / 覆盖不够 ⇒ 2，件头第 26 行明文）⇒ 逐字沿用 py 的 0/1/2 语义。
//	  ③ **✗ 不静默绿**：读不到命令树 / 制品不在 / 覆盖低于棘轮**一律非 0**（rc=2）⇒ 无静默绿面。
//	  ④ **写面**：本件**零写盘**（除自检在系统临时目录里建一枚合成夹具、用完即删 —— 见「自检自回收」节）。
//	     判据路径**不写任何文件**、不落状态目录、不自我改写。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在命令树里注册命令（✗ 不进 `commands` 表 · `add_step` **已挂** `precommit-gates.sh:3470`（门58 · tri 档））；
//	· **不**删 py 原件（退场规则：**对拍通过后**才逐件点名退场）；
//	· **已接线**：入口 `checkFieldsFaceParityGateMain` 由 `gateCheckGoEntry["check-fields-face-parity"]` 分派（`zerg gate check check-fields-face-parity`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `os.path.dirname(os.path.dirname(os.path.abspath(<脚本体路径>)))`
//	    （**脚本位置**上溯两级 = 仓根：`<仓根>/scripts/gates/<件>` ⇒ 上溯两级得 `<仓根>`）；
//	    本件**无「脚本体路径」语境**、只能从 **cwd** 上溯找含 `scripts/gates` 的目录、找不到回落
//	    `abspath(cwd)`（同族体例）。落在同一真仓、cwd=仓根（或 cwd 在仓内任意层）时两者读数逐字相同
//	    （已对拍甲/乙）；「脚本位置 ≠ cwd 上溯根」时路径推导面不同 ⇒ **不属逐字对齐承诺**，
//	    对拍一律 cwd=仓根 / 显式传位置参数 `root`。
//	    **旋钮缺省值 vs py 默认推导逐字对比**：
//	      py   `ROOT_DEFAULT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))`
//	      py   `BIN_DEFAULT  = os.path.join(ROOT, "bin", "zerg")`
//	      本件 `ffpDefaultRoot()` = cwd 上溯找 `scripts/gates`；BIN 缺省 = `ffpPyJoin(root,"bin","zerg")`
//	    ⇒ **事实判断（是否需串拼）**：py 的默认根是一串 `dirname()` 的直接结果（**绝对、已 clean、无 `..` 段**）；
//	      另三处拼路径分别是 `join(<脚本目录>, "..", "..")`（★ **含 `..` 段 ⇒ 触发语义差 ⇒ 必须自写**）、
//	      `join(root, "bin", "zerg")`、`join(root, MAIN_REL)`（后两处 `rel` 都是**仓内相对件名**、不含 `..`）
//	      ⇒ 本件**一律走自写 `ffpPyJoin`/`ffpPyAbspath`/`ffpPyNormpath`**（✗ 不用 `path/filepath.Join`），
//	      并对 `..` 与「双前导斜杠」两态各钉一格单测。
//	(b) **自指检查（先真判 · 四形态）**：
//	  ① 本件**在自己的扫描面内吗？—— 否**。本件的扫描面只有**一处**：读 `<root>/core/cmd/zerg/main.go`
//	     的文本（`open()` 1 处）。它**不遍历目录**、不 `glob`、不列目录；`core/cmd/zerg/*.go` 里
//	     没有一条被它读。⇒ 本件自身**不在**自己的判据面内（量化：`os.walk`/`listdir`/`glob` 各 0 处）。
//	  ② 本件**落在「他门」扫描面内吗？—— 是**（★ 首判即真，必须处置）：
//	     · 私径门扫 `core/`（含 `.go` 与 `_test.go`）⇒ 本件与测试件**零**私有绝对路径字面量
//	       （盘根 / 家目录前缀一概不写；夹具路径一律 `t.TempDir()` 或运行期拼串）；
//	     · 错误种类门的用法面扫 `core/cmd/zerg/*.go`（非 `_test.go`）里的两类原串 ⇒ 本件**不写**
//	       这两类原串（JSON 键名走 `ffpK` 运行时拼串，件内不出现「键名 + 冒号 + 引号」的整串）；
//	     · 茧壁门的命令树标记扫 `core/cmd/zerg/*.go`（非 `_test.go`）⇒ 本件**不写**层级标记原串。
//	  ③ **处理有效性已证**（硬取证法 = 落仓前后同门输出逐字相同）：落仓前后对**同一批门**取数
//	     （私径门 / 错误种类门 / 茧壁门 / 覆盖面门），输出 **sha16 逐字相同** 且 **rc 相同**
//	     （读数见交付回执）。⇒ 本件对真仓他门读数**贡献 0**。
//	  ④ **JSON 键名字面**是本件最大的自指面：键名若以「键名 + 冒号 + 引号」的整串落地，会被错误种类门
//	     的用法面点到名 ⇒ 一律经 `ffpK()` 运行时取用，件内**零**该形态字面。
//
// 未覆盖面（✗ 不声称逐字对齐 · 逐条）：
//
//	① **脚本原语语义差（已逐处现读，本件逐条对齐或声明）**：
//	   ⓐ `\s` 是 **py Unicode 语义**（含 `\x1c-\x1f` / `\x85` / `\xa0` / `\u2000-\u200a` …）
//	      ⟷ Go RE2 默认 ASCII ⇒ 本件把两处 `\s*`（在 `path:` / `fields:` 之后）**展开为显式 Unicode
//	      空白类** `ffpSpaceCls`（29 个码点 · 与 `str.isspace()` 同集）；✗ 不用裸 `\s`。
//	   ⓑ `str.strip()` 剥的是 py Unicode 空白（含 `\x1c-\x1f`）⟷ Go `strings.TrimSpace` 不剥 `\x1c-\x1f`
//	      ⇒ 本件自写 `ffpPyStrip`（用 `ffpPyIsSpace`），✗ 不用 `strings.TrimSpace`。
//	   ⓒ `open()` **文本模式**行分隔折换（`\r\n` 与裸 `\r` 都折 `\n`）⟷ Go `os.ReadFile` 原样
//	      ⇒ 本件自写 `ffpUniversalNewlines` 显式折（`main.go` 若用 CRLF 落盘，两态才同）。
//	   ⓓ `str.split()`（无参）按 **py Unicode 空白**切并丢空段 ⟷ `strings.Fields` 按 `unicode.IsSpace`
//	      ⇒ 仅差 `\x1c-\x1f` 四个码点（本文输入是引号内片段、实测不含）⇒ **声明差异**，不另写。
//	   ⓔ `str.splitlines()` 认 11 种行界（含 `\x1c-\x1e` / `\x85` / `\u2028` / `\u2029`）⟷
//	      `strings.Split(s,"\n")` ⇒ 本件自写 `ffpSplitLines` 与 py 同集。
//	   ⓕ `str.split(":",1)` / `str.split(",")`：字面切分，与 Go `strings.SplitN/Split` 同构（已对齐）。
//	② **argparse 面**：本件实现**精确长旗标 + 唯一前缀缩写 + `--opt=值` 等号形 + 缺值报话 + 未知旗标报话**。
//	   ✗ 未实现：`--` 分隔符之后的「一律当位置参数」那档；`-hx` 一类短旗标粘连；`--json=x` 对
//	   store_true 的「ignored explicit argument」报话（本件按未知旗标处置）。`-h`/`--help` 正文按
//	   Python 3.9 argparse 的 80 列**逐字固化**（已对拍）。
//	③ **异常路径（C 类）**：py 最外层是裸 `sys.exit(main())` ⇒ 未捕异常 = 回溯 + **rc=1**。本件把
//	   `main.go` **非 UTF-8** 一档按 rc=1 落（stdout 不产正文、stderr 走声明文案），✗ 不逐字复刻回溯。
//	④ **「跑不起来」文案**：py `subprocess.run` 抛 `OSError` 时 why 串是 CPython 的
//	   `[Errno N] …: '<路径>'`；本件同 rc（不影响 rc 面）但 why 正文用 Go 原话 ⇒ 该格**只保证 rc**、
//	   正文不同（★ 只在「制品存在但不可执行 / 是目录」那档才走到，正常档不会）。
//	⑤ **timeout 重序列化**：py `"{}".format(timeout)` 对 int 打 `15`、对 float 打 `1.0` ⇒ 本件按
//	   「是否给过 `--timeout`」分档复刻（给了 ⇒ float 形态；没给 ⇒ int 形态）；极端非整数值的 py
//	   最短 repr 与 Go `%g` 的差异（如 `1e+20`）**未覆盖**。
//	⑥ **列表正文**：`%s` 打 Python `list` 用 `['a', 'b']`（元素 repr）⇒ 本件 `ffpPyRepr` 只复刻
//	   单/双引号与常见转义；非可打印字符的 `\xNN` repr 形态**未覆盖**（本件元素是字段名 / 命令路径，
//	   实测不含）。
//	⑦ **`os.Stat` 跟随链**：py `os.path.exists` 跟随符号链接 ⇒ 本件用 `os.Stat`（✗ 非 `Lstat`）；
//	   本件**无** `islink` 面（量化：0 处）。悬挂链 ⇒ 两态同为「不存在」。
//	⑧ **目录遍历面**：**不适用** —— py 零 `os.walk` / 零 `os.listdir` / 零 `glob`（量化 0/0/0），
//	   判据面的全部输入是「一枚文件的文本」＋「制品的运行期输出」⇒ 无遍历序可言。
//	⑨ **JSON 输入反序列化面**：**不适用** —— py 零 `json.load` / 零 `json.loads`（量化 0/0），
//	   只用 `json.dumps` 出机器读面 ⇒ 「区分缺键与显式 null」的**输入端**在本件不存在。
//	   ⇒ 输出侧的三形态本件**逐形态钉住**：`[]`（空集合走显式空切片，✗ 不落 `null`）·
//	   缺键（`error` 档的条目**只有** `error` 一键，✗ 不补默认键）· `null`（本门输出**永不产 `null`**，
//	   由单测 `T3` 与对拍格 `B11` 双钉）。
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"
)

const ffpProg = "check-fields-face-parity.py"

// ffpMainRel —— py `MAIN_REL = os.path.join("core", "cmd", "zerg", "main.go")`（运行期拼，✗ 不写整串）。
var ffpMainRel = ffpPyJoin("core", "cmd", "zerg", "main.go")

// ffpPkgRel —— py `PKG_REL = os.path.join("core","cmd","zerg")`（运行期拼）。
var ffpPkgRel = ffpPyJoin("core", "cmd", "zerg")

// ffpIdentDepthMax —— py `IDENT_DEPTH_MAX = 4`（递归展开深度上限）。
const ffpIdentDepthMax = 4

const (
	ffpLegalFieldsPrefix = "合法字段:"
	// 腿②「公告 == 可选」：一枚字段被判**选不出**时制品逐字打的标记（`zerg: 未知字段 "<f>"`）。
	ffpUnknownFieldMark = "未知字段"
	ffpBogus            = "zzz-nonexistent-zz"
	ffpPerCmdTimeout    = 15
	// ★ 自检合成夹具的探针预算（GAP-20261019-05）：夹具是 `/bin/sh` 一行脚本，只验判据不验时延；
	// 原先 2s/3s 在满载机上把「探针还没跑起来」读成「够不着」⇒ covered=0 ⇒ run() rc=2 ⇒ 端到端正控假红。
	// 取 2× 生产预算（`ffpPerCmdTimeout`=15s）⇒ 判定面一个字节不改，只把夹具的时延噪声挤出去。
	ffpSelfTestProbeTO     = 30 * time.Second
	ffpSelfTestProbeTORepr = "30"
	ffpMinCoveredDefault   = 29
	// ★ 腿② 棘轮（`--min-selectable`）：今天能逐枚打靶的**格数**下限（只许上调、永不下调）。
	ffpMinSelectableDefault = 92
)

// ffpNotProbedPrefixes —— 腿②「会起整套门 / 长任务 / 阻塞」⇒ **不打靶**白名单（自伤洞：孤儿门跑占闸 ·
// `GAP-20261002-11` / `-20261003-17`）。判据 = **宁少打不可误跑**。★ `gate bench`：探针 argv 恒为
// `<bin> <path…> --json <field>`、永不带 `--fast`/`--scope` ⇒ 打到它时必是**默认全量档** ⇒ 整条排除。
var ffpNotProbedPrefixes = []string{"gate bench", "gate run", "gate wait", "gap reread"}

const ffpNotProbedReason = "会起整套门 / 长任务 / 阻塞 ⇒ 不打靶（宁少打不可误跑 · 自伤洞 GAP-20261002-11）"

// ffpSpaceCls —— py `\s` 的 Unicode 展开（与 `str.isspace()` 同集 · 29 个码点）。
// ★ 必须是「一个字符类」，✗ 不是 0/1 个字符 —— 用法见 ffpReCmd / ffpReFields。
const ffpSpaceCls = `[\t\n\x0b\x0c\r \x1c-\x1f\x85\xa0\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

var (
	ffpReCmd     = regexp.MustCompile(`path:` + ffpSpaceCls + `*\[\]string\{([^}]*)\}`)
	ffpReFields  = regexp.MustCompile(`fields:` + ffpSpaceCls + `*\[\]string\{([^}]*)\}`)
	ffpRePathSeg = regexp.MustCompile(`"([^"]+)"`)
	// 拼接变量形态（缺口 GAP-20260928-159）：`fields: <标识符>` ⇒ 跨件解析该标识符的名字集。
	ffpReFieldsIdent = regexp.MustCompile(`fields:` + ffpSpaceCls + `*([A-Za-z_][A-Za-z0-9_]*)` + ffpSpaceCls + `*,`)
	// `[]string{…}` 里那批字面量 / 裸标识符。
	ffpReStrLit      = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)
	ffpReIdent       = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\b`)
	ffpReIdentLit    = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\b` + ffpSpaceCls + `*=` + ffpSpaceCls + `*\[\]string\{([^{}]*)\}`)
	ffpReIdentAppend = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\b` + ffpSpaceCls + `*=` + ffpSpaceCls + `*append` + ffpSpaceCls + `*\(([^\n]*)`)
	ffpReEllipsis    = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\.\.`)
)

// ── py 路径语义家族（自写 · ✗ 不用 path/filepath.Join|Abs|Dir）─────────────────────────
// 为什么：见件头 (a)。三处拼路径里 `join(<脚本目录>, "..", "..")` **含 `..` 段** ⇒ filepath.Join
// 会把它归一掉、py **原样留串**再靠 abspath 归一 ⇒ 必须自写。

// ffpPyJoin 复刻 posixpath.join（✗ 别手工「跳过空段」，那会把 "a/" 变成 "a"）。
func ffpPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "/"): // 后段绝对 ⇒ 重置前面所有段
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}

// ffpPyNormpath 复刻 posixpath.normpath。★ 保留「开头恰两个斜杠」：`//a` 不归一成 `/a`。
func ffpPyNormpath(path string) string {
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
			// 跳过
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

// ffpPyDirname 复刻 posixpath.dirname（= split 的头段；剥尾斜杠，但纯 `/` 保留）。
func ffpPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// ffpPyAbspath 复刻 posixpath.abspath：**纯词法**（不碰盘、不解析符号链接）。cwd 由调用方传入。
func ffpPyAbspath(cwd, path string) string {
	return ffpPyNormpath(ffpPyJoin(cwd, path))
}

func ffpCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// ── py 字符串原语（自写）──────────────────────────────────────────────────────────

// ffpPyIsSpace —— py `str.isspace()` 的码点集（含 `\x1c-\x1f` · `\x85` · `\xa0` …）。
func ffpPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0:
		return true
	}
	if r >= 0x2000 && r <= 0x200a {
		return true
	}
	switch r {
	case 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return false
}

// ffpPyStrip 复刻 `str.strip()`（✗ 不用 strings.TrimSpace：它不剥 `\x1c-\x1f`）。
func ffpPyStrip(s string) string {
	rs := []rune(s)
	i, j := 0, len(rs)
	for i < j && ffpPyIsSpace(rs[i]) {
		i++
	}
	for j > i && ffpPyIsSpace(rs[j-1]) {
		j--
	}
	return string(rs[i:j])
}

func ffpIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// ffpSplitLines 复刻 `str.splitlines()`（11 种行界 · `\r\n` 当一条 · 无尾空行）。
func ffpSplitLines(s string) []string {
	out := []string{}
	var cur strings.Builder
	i := 0
	for i < len(s) {
		r, sz := utf8.DecodeRuneInString(s[i:])
		if ffpIsLineBoundary(r) {
			if r == '\r' && i+sz < len(s) && s[i+sz] == '\n' {
				i += sz + 1
			} else {
				i += sz
			}
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
		i += sz
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// ffpUniversalNewlines 复刻 `open(..., encoding="utf-8")` 文本模式的行分隔折换。
func ffpUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// ffpPySplit —— py `str.split()`（无参：按 py Unicode 空白切、丢空段）。
// 实测输入是引号内片段（不含 `\x1c-\x1f`）⇒ 与 strings.Fields 同构；差异见未覆盖面 ①ⓓ。
func ffpPySplit(s string) []string {
	return strings.FieldsFunc(s, ffpPyIsSpace)
}

// ── JSON 输出（py `json.dumps(..., ensure_ascii=False)` · 默认分隔符 `", "` / `": "`）──

type ffpJV struct {
	kind byte // 'o' 对象 · 'a' 数组 · 's' 串 · 'n' 整数 · 'b' 布尔
	s    string
	num  string
	b    bool
	arr  []ffpJV
	obj  []ffpKP
}

type ffpKP struct {
	k string
	v ffpJV
}

func ffpObj(pairs ...ffpKP) ffpJV { return ffpJV{kind: 'o', obj: pairs} }
func ffpArr(xs ...ffpJV) ffpJV    { return ffpJV{kind: 'a', arr: xs} }
func ffpStr(s string) ffpJV       { return ffpJV{kind: 's', s: s} }
func ffpInt(n int) ffpJV          { return ffpJV{kind: 'n', num: strconv.Itoa(n)} }
func ffpBool(b bool) ffpJV        { return ffpJV{kind: 'b', b: b} }
func ffpStrArr(xs []string) ffpJV {
	out := make([]ffpJV, len(xs))
	for i, x := range xs {
		out[i] = ffpStr(x)
	}
	return ffpJV{kind: 'a', arr: out}
}

// ffpK —— ★ 自指接缝：键名一律经此取用，件内**不出现**「引号键名 + 冒号 + 引号值」的整串。
func ffpK(name string) string { return name }

// ffpEncStr —— py `json.encoder.py_encode_basestring`（ensure_ascii=False）逐字复刻。
func ffpEncStr(s string) string {
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

// ffpDump —— py `json.dumps(obj, ensure_ascii=False)`（紧凑档 · 保插入序）。
func ffpDump(v ffpJV) string {
	switch v.kind {
	case 'o':
		if len(v.obj) == 0 {
			return "{}"
		}
		parts := make([]string, len(v.obj))
		for i, p := range v.obj {
			parts[i] = ffpEncStr(p.k) + ": " + ffpDump(p.v)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case 'a':
		if len(v.arr) == 0 {
			return "[]"
		}
		parts := make([]string, len(v.arr))
		for i, x := range v.arr {
			parts[i] = ffpDump(x)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case 's':
		return ffpEncStr(v.s)
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

// ── Python 容器正文（`%s` 打 list / tuple）──────────────────────────────────────

func ffpPyRepr(s string) string {
	q := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if byte(r) == q {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte(q)
	return b.String()
}

func ffpPyListRepr(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = ffpPyRepr(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func ffpPyTupleRepr(e, m []string) string {
	return "(" + ffpPyListRepr(e) + ", " + ffpPyListRepr(m) + ")"
}

// ── 解析面 ────────────────────────────────────────────────────────────────────

type ffpFace struct {
	path   string
	fields []string
}

// ffpReadTextMode —— py `read(path)`：文本模式（折行界）＋ utf-8 严格解码。
// 返回 kind：0 成 · 1 OSError · 2 UnicodeDecodeError。
func ffpReadTextMode(path string) (string, int) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", 1
	}
	if !utf8.Valid(b) {
		return "", 2
	}
	return ffpUniversalNewlines(string(b)), 0
}

// ffpPkgSources —— py `package_sources(root)`：`core/cmd/zerg` 包内全部非测试 .go（按名排序后以 `\n` 连接）。
func ffpPkgSources(root string) string {
	dir := ffpPyJoin(root, ffpPkgRel)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	names := []string{}
	for _, e := range ents {
		n := e.Name()
		if strings.HasSuffix(n, ".go") && !strings.HasSuffix(n, "_test.go") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	out := []string{}
	for _, n := range names {
		b, err := os.ReadFile(ffpPyJoin(dir, n))
		if err != nil {
			continue
		}
		out = append(out, string(b))
	}
	return strings.Join(out, "\n")
}

// ffpStripComments —— py `_strip_comments`：逐行取首个 `//` 之前那段。
func ffpStripComments(src string) string {
	ls := strings.Split(src, "\n")
	for i, ln := range ls {
		if j := strings.Index(ln, "//"); j >= 0 {
			ls[i] = ln[:j]
		}
	}
	return strings.Join(ls, "\n")
}

// ffpIdentDefs —— py `_ident_defs(pkg_src)` ⇒ (lit, refs)。同名 Go 标识符不另立真源。
func ffpIdentDefs(pkgSrc string) (map[string][][]string, map[string][][]string) {
	code := ffpStripComments(pkgSrc)
	lit := map[string][][]string{}
	refs := map[string][][]string{}
	for _, m := range ffpReIdentLit.FindAllStringSubmatch(code, -1) {
		ident, body := m[1], m[2]
		names := []string{}
		for _, sm := range ffpReStrLit.FindAllStringSubmatch(body, -1) {
			names = append(names, sm[1])
		}
		stripped := ffpReStrLit.ReplaceAllString(body, " ")
		for _, ref := range ffpReIdent.FindAllStringSubmatch(stripped, -1) {
			rx := regexp.MustCompile(`\b` + regexp.QuoteMeta(ref[1]) + `\b` + ffpSpaceCls + `*=` + ffpSpaceCls + `*"((?:[^"\\]|\\.)*)"`)
			if sm := rx.FindStringSubmatch(code); sm != nil {
				names = append(names, sm[1])
			}
		}
		lit[ident] = append(lit[ident], names)
	}
	for _, m := range ffpReIdentAppend.FindAllStringSubmatch(code, -1) {
		ident, expr := m[1], m[2]
		ids := []string{}
		for _, r := range ffpReEllipsis.FindAllStringSubmatch(expr, -1) {
			if r[1] != ident {
				ids = append(ids, r[1])
			}
		}
		if len(ids) > 0 {
			refs[ident] = append(refs[ident], ids)
		}
	}
	return lit, refs
}

// ffpResolveIdentNames —— py `_resolve_ident_names`：拼接变量的字段表 ⇒ 名字集（递归展开 · 环保护 · 深度上限）。
func ffpResolveIdentNames(lit, refs map[string][][]string, ident string,
	memo map[string]map[string]bool, depth int) map[string]bool {
	if v, ok := memo[ident]; ok {
		return v
	}
	if depth > ffpIdentDepthMax {
		return map[string]bool{}
	}
	memo[ident] = map[string]bool{} // 环保护：解析途中再遇到自己 ⇒ 取空（不递归）
	names := map[string]bool{}
	for _, ns := range lit[ident] {
		for _, n := range ns {
			names[n] = true
		}
	}
	for _, ids := range refs[ident] {
		for _, ref := range ids {
			if ref == ident {
				continue
			}
			if _, ok := lit[ref]; ok {
				for n := range ffpResolveIdentNames(lit, refs, ref, memo, depth+1) {
					names[n] = true
				}
			} else if _, ok := refs[ref]; ok {
				for n := range ffpResolveIdentNames(lit, refs, ref, memo, depth+1) {
					names[n] = true
				}
			}
		}
	}
	memo[ident] = names
	return names
}

// ffpSortedUniq —— py `sorted(set(names))`。
func ffpSortedUniq(names []string) []string {
	seen := map[string]bool{}
	uniq := []string{}
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			uniq = append(uniq, n)
		}
	}
	sort.Strings(uniq)
	return uniq
}

// ffpDeclaredFaces —— py `declared_faces(root)`。
// none=true ⇒ py 返回 None（读不到 / 块找不到）；fatal=true ⇒ py 抛未捕异常（解码失败 ⇒ rc=1）。
func ffpDeclaredFaces(root string) (faces []ffpFace, none, fatal bool) {
	src, kind := ffpReadTextMode(ffpPyJoin(root, ffpMainRel))
	if kind == 1 {
		return nil, true, false
	}
	if kind == 2 {
		return nil, false, true
	}
	pkgSrc := ffpPkgSources(root)
	identLit, identRefs := ffpIdentDefs(pkgSrc)
	memo := map[string]map[string]bool{}
	start := strings.Index(src, "commands = []*command{")
	if start < 0 {
		return nil, true, false
	}
	rel := strings.Index(src[start:], "\n\t}\n")
	if rel < 0 {
		return nil, true, false
	}
	seg := src[start : start+rel]
	ms := ffpReCmd.FindAllStringSubmatchIndex(seg, -1)
	out := []ffpFace{}
	for i, m := range ms {
		g1 := seg[m[2]:m[3]]
		parts := []string{}
		for _, pm := range ffpRePathSeg.FindAllStringSubmatch(g1, -1) {
			parts = append(parts, pm[1])
		}
		if len(parts) == 0 {
			continue
		}
		nxt := len(seg)
		if i+1 < len(ms) {
			nxt = ms[i+1][0]
		}
		window := seg[m[1]:nxt]
		has := false
		flds := []string{}
		if fm := ffpReFields.FindStringSubmatchIndex(window); fm != nil {
			has = true
			sub := window[fm[2]:fm[3]]
			for _, pm := range ffpRePathSeg.FindAllStringSubmatch(sub, -1) {
				flds = append(flds, ffpPyStrip(pm[1]))
			}
		} else if im := ffpReFieldsIdent.FindStringSubmatch(window); im != nil {
			// 标识符解析不到名字（没定义 / 展开为空）⇒ 本门不把它当「有字段面」条目。
			got := ffpResolveIdentNames(identLit, identRefs, im[1], memo, 0)
			if len(got) > 0 {
				has = true
				for k := range got {
					flds = append(flds, k)
				}
			}
		}
		if has {
			out = append(out, ffpFace{path: strings.Join(parts, " "), fields: ffpSortedUniq(flds)})
		}
	}
	return out, false, false
}

// ── 真身面（现跑制品）────────────────────────────────────────────────────────────

type ffpNope struct {
	path   string
	reason string
}

type ffpMism struct {
	path          string
	declaredExtra []string
	runtimeExtra  []string
}

// ffpKillGroup —— 杀**整棵**探针子树（自伤洞：孤儿门跑占闸 · `GAP-20261002-11`）。
// `Setpgid` ⇒ 子进程自成进程组、`pgid == 子 pid` ⇒ `Kill(-pgid, sig)` 一次收**整组**
// （✗ 只杀直接子 ⇒ 孙趟门跑 reparent 到 pid 1 继续跑、还继承锁 fd）。
func ffpKillGroup(pgid int, sig syscall.Signal) {
	_ = syscall.Kill(-pgid, sig)
}

// ffpRunProbe —— 起一枚探针子进程 ⇒ (rc, stdout, stderr, timedOut, startErr)。（自伤洞两刀之一 · 纵深）
// ★ `Setpgid`（= py 的 `start_new_session=True`）＋ 超时杀**整组**：先 SIGTERM、宽限 2s 再 SIGKILL
// （✗ 不用 exec.CommandContext —— 它只杀**直接子**，孙进程成孤儿并继承父趟锁 fd）。
func ffpRunProbe(binpath string, args []string, timeout time.Duration) (int, string, string, bool, error) {
	var outb, errb bytes.Buffer
	cmd := exec.Command(binpath, args...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb // Stdin 留 nil ⇒ 子进程读空设备（= py 的 DEVNULL）
	// ★ 独立进程组（= os.setsid）：超时才能收整棵子树；✗ 不用 CommandContext（只杀直接子）。
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if e := cmd.Start(); e != nil {
		return 0, "", "", false, e
	}
	pgid := cmd.Process.Pid
	var to atomic.Bool
	timer := time.AfterFunc(timeout, func() {
		to.Store(true)
		ffpKillGroup(pgid, syscall.SIGTERM)
		time.AfterFunc(2*time.Second, func() { ffpKillGroup(pgid, syscall.SIGKILL) })
		_ = cmd.Process.Kill() // 兜底（直接子；整组那两刀在上一行）
	})
	werr := cmd.Wait()
	timer.Stop()
	if to.Load() {
		return 0, outb.String(), errb.String(), true, nil
	}
	rc := 0
	if werr != nil {
		var ee *exec.ExitError
		if errors.As(werr, &ee) {
			rc = ee.ExitCode()
		} else {
			return 0, "", "", false, werr
		}
	}
	return rc, outb.String(), errb.String(), false, nil
}

// ffpRuntimeFace —— py `runtime_face`。got=false ⇒ py 返回 None（why 非空）。
func ffpRuntimeFace(binpath, path string, timeout time.Duration, timeoutRepr string) (flds []string, got bool, why string) {
	args := append([]string{}, ffpPySplit(path)...)
	args = append(args, "--json", ffpBogus)
	rc, _, errStr, timedOut, startErr := ffpRunProbe(binpath, args, timeout)
	if startErr != nil {
		return nil, false, "跑不起来：" + startErr.Error() // 正文口径差见未覆盖面 ④
	}
	if timedOut {
		return nil, false, "超时（>" + timeoutRepr + "s ⇒ 不算对拍面）"
	}
	for _, line := range ffpSplitLines(errStr) {
		if strings.HasPrefix(ffpPyStrip(line), ffpLegalFieldsPrefix) {
			idx := strings.Index(line, ":")
			if idx < 0 {
				continue
			}
			out := []string{}
			for _, s := range strings.Split(line[idx+1:], ",") {
				if t := ffpPyStrip(s); t != "" {
					out = append(out, t)
				}
			}
			return out, true, ""
		}
	}
	return nil, false, "无字段表（rc=" + strconv.Itoa(rc) + " ⇒ 前置校验先于字段面 / 要活服务）"
}

// ffpLegalFieldsOf —— py `legal_fields_of`：stderr 里 `合法字段: a,b,c` ⇒ 真身闭集（无 ⇒ nil）。
func ffpLegalFieldsOf(err string) []string {
	for _, line := range ffpSplitLines(err) {
		if strings.HasPrefix(ffpPyStrip(line), ffpLegalFieldsPrefix) {
			idx := strings.Index(line, ":")
			if idx < 0 {
				continue
			}
			out := []string{}
			for _, s := range strings.Split(line[idx+1:], ",") {
				if t := ffpPyStrip(s); t != "" {
					out = append(out, t)
				}
			}
			return out
		}
	}
	return nil
}

// ffpClassifyField —— py `classify_field`：腿②「公告 == 可选」的三档（ok / rejected / contradiction）。
// `rc` 只为与 py 同签名（本档判据只看报话，不看 rc —— 「rc 非 0」不等于「字段被拒」）。
func ffpClassifyField(rc int, err, field string) string {
	_ = rc
	legal := ffpLegalFieldsOf(err)
	if !strings.Contains(err, ffpUnknownFieldMark+" \""+field+"\"") {
		return "ok"
	}
	for _, f := range legal {
		if f == field {
			return "contradiction"
		}
	}
	return "rejected"
}

// ffpProbeExcluded —— 腿② 不打靶白名单（自伤洞：孤儿门跑占闸 · `GAP-20261002-11` / `-20261003-17`）。
// true ⇒ 该命令**一格不打靶**（记 `not_probed` + 为何），✗ 不让探针把它真跑起来。
func ffpProbeExcluded(path string) bool {
	for _, p := range ffpNotProbedPrefixes {
		if path == p || strings.HasPrefix(path, p+" ") {
			return true
		}
	}
	return false
}

// ffpSelectableProbe —— py `selectable_probe`：现跑一枚 `--json <field>`（只触字段面 · ✗ 不落写路径）。
// ★ 调用方须先用 `ffpProbeExcluded()` 把「会起整套门 / 长任务 / 阻塞」的命令挡在**打靶面外**。
// got=false ⇒ 超时 / 跑不起来（不算打靶格）。
func ffpSelectableProbe(binpath, path, field string, timeout time.Duration) (verdict string, got bool) {
	args := append([]string{}, ffpPySplit(path)...)
	args = append(args, "--json", field)
	rc, _, errStr, timedOut, startErr := ffpRunProbe(binpath, args, timeout)
	if startErr != nil || timedOut {
		return "", false
	}
	return ffpClassifyField(rc, errStr, field), true
}

// ffpCompare —— py `compare(decl, real)`：集合差（逐名 · 各自 sorted）。
func ffpCompare(decl, real []string) (extra, missing []string) {
	ds := map[string]bool{}
	for _, s := range decl {
		ds[s] = true
	}
	rs := map[string]bool{}
	for _, s := range real {
		rs[s] = true
	}
	extra, missing = []string{}, []string{}
	for s := range ds {
		if !rs[s] {
			extra = append(extra, s)
		}
	}
	for s := range rs {
		if !ds[s] {
			missing = append(missing, s)
		}
	}
	sort.Strings(extra)
	sort.Strings(missing)
	return extra, missing
}

type ffpSelCell struct {
	path  string
	field string
}

type ffpSelUnreach struct {
	path   string
	field  string
	reason string
}

// ffpSelNotProbe —— 腿② 不打靶白名单命中格（记 `not_probed` + 为何 · 自伤洞 `GAP-20261002-11`）。
type ffpSelNotProbe struct {
	path   string
	field  string
	reason string
}

type ffpResult struct {
	errorMsg      string
	fatal         bool
	declared      int
	covered       int
	minCovered    int
	matched       []string
	mismatched    []ffpMism
	notComparable []ffpNope
	// ── 腿②「公告 == 可选」（本枚加）────────────────────────────────────────────
	selCovered    int
	minSelectable int
	selMatched    []string
	selRejected   []ffpSelCell
	selContra     []ffpSelCell
	selUnreach    []ffpSelUnreach
	// ★ 不打靶白名单命中格（自伤洞 · `GAP-20261002-11`）：会起整套门 / 长任务 / 阻塞 ⇒ 一格不打靶。
	selNotProbed []ffpSelNotProbe
}

// ffpRun —— py `run(root, binpath, min_covered, timeout, min_selectable)`。
func ffpRun(root, binpath string, minCovered int, timeout time.Duration, timeoutRepr string,
	minSelectable int) (int, ffpResult) {
	decl, none, fatal := ffpDeclaredFaces(root)
	if fatal {
		return 1, ffpResult{fatal: true, minCovered: minCovered, minSelectable: minSelectable}
	}
	if none {
		return 2, ffpResult{errorMsg: "读不到命令树（" + ffpMainRel + "）⇒ 判不了",
			minCovered: minCovered, minSelectable: minSelectable}
	}
	if _, err := os.Stat(binpath); err != nil { // py os.path.exists：跟随符号链接 ⇒ os.Stat
		return 2, ffpResult{errorMsg: "缺制品 " + binpath + " ⇒ 判不了（不许拿声明自己跟自己比）",
			minCovered: minCovered, minSelectable: minSelectable}
	}
	ok := []string{}
	mism := []ffpMism{}
	nope := []ffpNope{}
	covered := 0
	selOK := []string{}
	selRej := []ffpSelCell{}
	selCon := []ffpSelCell{}
	selUnr := []ffpSelUnreach{}
	selNp := []ffpSelNotProbe{}
	selCovered := 0
	for _, f := range decl {
		real, got, why := ffpRuntimeFace(binpath, f.path, timeout, timeoutRepr)
		if !got {
			// 真身表取不到 ⇒ 字段面这一格**够不着** ⇒ 腿② 这一条整条不打靶（已在 `nope` 逐条点名）。
			nope = append(nope, ffpNope{path: f.path, reason: why})
			continue
		}
		covered++
		extra, missing := ffpCompare(f.fields, real)
		if len(extra) > 0 || len(missing) > 0 {
			mism = append(mism, ffpMism{path: f.path, declaredExtra: extra, runtimeExtra: missing})
		} else {
			ok = append(ok, f.path)
		}
		// ★ 腿② 不打靶白名单（自伤洞两刀之一 · 治本）：会起整套门的命令**一格不打靶** ⇒ 记 `not_probed`。
		if ffpProbeExcluded(f.path) {
			for _, fld := range f.fields {
				selNp = append(selNp, ffpSelNotProbe{path: f.path, field: fld, reason: ffpNotProbedReason})
			}
			continue
		}
		// ── 腿②「公告 == 可选」：对**两集无差**的命令逐枚跑登记的每一格 ──────────────
		good := true
		for _, fld := range f.fields {
			verdict, got2 := ffpSelectableProbe(binpath, f.path, fld, timeout)
			if !got2 {
				selUnr = append(selUnr, ffpSelUnreach{path: f.path, field: fld,
					reason: "超时（>" + timeoutRepr + "s ⇒ 不算打靶格）"})
				good = false
				continue
			}
			selCovered++
			if verdict == "ok" {
				continue
			}
			good = false
			if verdict == "contradiction" {
				selCon = append(selCon, ffpSelCell{path: f.path, field: fld})
			} else {
				selRej = append(selRej, ffpSelCell{path: f.path, field: fld})
			}
		}
		if good && len(f.fields) > 0 {
			selOK = append(selOK, f.path)
		}
	}
	rc := 0
	if len(mism) > 0 || len(selRej) > 0 || len(selCon) > 0 {
		rc = 1
	} else if covered < minCovered || selCovered < minSelectable {
		rc = 2 // 覆盖不够 = 空转（「配置没了所以全绿」那一类假绿）
	}
	return rc, ffpResult{
		declared: len(decl), covered: covered, minCovered: minCovered,
		matched: ok, mismatched: mism, notComparable: nope,
		selCovered: selCovered, minSelectable: minSelectable,
		selMatched: selOK, selRejected: selRej, selContra: selCon, selUnreach: selUnr,
		selNotProbed: selNp,
	}
}

// ── 成对负控自检（py `selftest` · 7 控）────────────────────────────────────────

// ffpSelfTestWhy —— 端到端夹具的失败诊断串（GAP-20261019-05）：rc ＋ 两条覆盖读数 ＋ 首条「够不着」原因。
// 只打 `rc=2` 判不了「探针超时」与「判据真红」（两者码位相同）；满载机上因此说不清是谁的错。
func ffpSelfTestWhy(rc int, res ffpResult) string {
	why := "rc=" + strconv.Itoa(rc) +
		" covered=" + strconv.Itoa(res.covered) + "/" + strconv.Itoa(res.minCovered) +
		" sel_covered=" + strconv.Itoa(res.selCovered) + "/" + strconv.Itoa(res.minSelectable)
	if len(res.notComparable) > 0 {
		why += " nope:" + res.notComparable[0].path + "=" + res.notComparable[0].reason
	}
	if res.errorMsg != "" {
		why += " err:" + res.errorMsg
	}
	return why
}

func ffpSelfTest(stdout io.Writer) int {
	ok := true
	chk := func(name string, cond bool, why string) {
		if cond {
			fmt.Fprint(stdout, "   ✓ "+name+"\n")
		} else {
			msg := "   ✗ " + name
			if why != "" {
				msg += "（" + why + "）"
			}
			fmt.Fprint(stdout, msg+"\n")
		}
		ok = ok && cond
	}
	eqs := func(a, b []string) bool {
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

	e, m := ffpCompare([]string{"a", "b"}, []string{"a", "b"})
	chk("正控：声明 == 真身 ⇒ 无差", len(e) == 0 && len(m) == 0, "")
	e, m = ffpCompare([]string{"a", "b", "c"}, []string{"a", "b"})
	chk("负控①：声明多一名 ⇒ 点名 extra=['c']", eqs(e, []string{"c"}) && len(m) == 0, ffpPyTupleRepr(e, m))
	e, m = ffpCompare([]string{"a"}, []string{"a", "b"})
	chk("负控②：声明少一名 ⇒ 点名 runtime_extra=['b']", len(e) == 0 && eqs(m, []string{"b"}), ffpPyTupleRepr(e, m))
	e, m = ffpCompare([]string{"a", "z"}, []string{"a", "b"})
	chk("负控③：同数不同名 ⇒ 两边都点名", eqs(e, []string{"z"}) && eqs(m, []string{"b"}), ffpPyTupleRepr(e, m))
	e, m = ffpCompare([]string{"b", "a"}, []string{"a", "b"})
	chk("负控④：仅顺序不同 ⇒ 无差（不比序）", len(e) == 0 && len(m) == 0, "")

	// ── 腿②「公告 == 可选」（本枚加）：成对负控（纯函数 · 合成 stderr · 不碰真目标）────────
	chk("腿②正控：rc=0 ⇒ ok", ffpClassifyField(0, "", "model") == "ok", "")
	chk("腿②负控⑦：rc=2 报话非「未知字段 model」（卡在位置参数/活服务）⇒ ok",
		ffpClassifyField(2, "zerg: `agent bench` 要一台机器名（`zerg agent ls` 看名册）\n", "model") == "ok", "")
	chk("腿②负控⑧：「未知字段 model」且同消息清单不含 model ⇒ rejected",
		ffpClassifyField(2, "zerg: "+ffpUnknownFieldMark+" \"model\"\n"+ffpLegalFieldsPrefix+" host,state\n", "model") == "rejected", "")
	chk("腿②负控⑨（公告≠可选 ⇒ 必红）：rc=2「未知字段 model」却把 model 列进同条清单 ⇒ contradiction",
		ffpClassifyField(2, "zerg: "+ffpUnknownFieldMark+" \"model\"\n"+ffpLegalFieldsPrefix+" host,model,state\n", "model") == "contradiction", "")
	chk("腿②负控⑩：「未知字段 other」不牵累本格 model ⇒ ok",
		ffpClassifyField(2, "zerg: "+ffpUnknownFieldMark+" \"other\"\n"+ffpLegalFieldsPrefix+" host,model\n", "model") == "ok", "")

	// 解析面：合成命令树 ⇒ 抓得到 path ＋ 声明名单（✗ 不是 bool）。
	tmp, err := os.MkdirTemp("", "check-fields-face-parity-selftest-")
	if err != nil {
		chk("解析面：字面量 ⇒ 名单逐字（不是 bool）", false, err.Error())
	} else {
		_ = os.MkdirAll(ffpPyJoin(tmp, "core", "cmd", "zerg"), 0o755)
		body := "\tcommands = []*command{\n" +
			"\t\t{path: []string{\"zz\", \"ls\"}, fields: []string{\"name\", \"verdict\"}},\n" +
			"\t\t{path: []string{\"zz\", \"ed\"}, fields: zzFields, run: x},\n" +
			"\t\t{path: []string{\"zz\", \"cf\"}, fields: zzConst, run: x},\n" +
			"\t}\n"
		_ = os.WriteFile(ffpPyJoin(tmp, ffpMainRel), []byte(body), 0o644)
		defs := "package main\n\n" +
			"var zzA = []string{\n\t\"alpha\", \"beta\",\n}\n\n" +
			"var zzB = []string{\"gamma\",\n\t\"delta\"}\n\n" +
			"var zzNoise = []string{\"--not-a-field\"}\n\n" +
			"var zzFields = append(append([]string{}, zzA...), zzB...)\n\n" +
			"var zzConst = []string{\"kappa\", zzConstField}\n\n" +
			"const zzConstField = \"has_profile\"\n"
		_ = os.WriteFile(ffpPyJoin(tmp, "core", "cmd", "zerg", "zz_defs.go"), []byte(defs), 0o644)
		got, none, fatal := ffpDeclaredFaces(tmp)
		has := func(path string, fs ...string) bool {
			for _, g := range got {
				if g.path == path && eqs(g.fields, fs) {
					return true
				}
			}
			return false
		}
		rep := "["
		for i, g := range got {
			if i > 0 {
				rep += ", "
			}
			rep += "(" + ffpPyRepr(g.path) + ", " + ffpPyListRepr(g.fields) + ")"
		}
		rep += "]"
		okbase := !none && !fatal
		chk("解析面：字面量 ⇒ 名单逐字（不是 bool）", okbase && has("zz ls", "name", "verdict"), rep)
		chk("解析面：拼接变量 `fields: zzFields` ⇒ 跨件展开名集（多行字面量 · 缺口 GAP-20260928-159）",
			okbase && has("zz ed", "alpha", "beta", "delta", "gamma"), rep)
		chk("解析面：`[]string{…, <常量标识符>}` 的裸标识符 ⇒ 按字符串标量取值（缺口 GAP-20261005-45）",
			okbase && has("zz cf", "has_profile", "kappa"), rep)
		noNoise := true
		for _, g := range got {
			for _, f := range g.fields {
				if f == "--not-a-field" {
					noNoise = false
				}
			}
		}
		chk("解析面：未被子表引用的 zzNoise 不被误收（不啃无关标识符）", okbase && noNoise, rep)
		_ = os.RemoveAll(tmp) // ★ 自回收（py 的 tempfile.TemporaryDirectory 也回收 ⇒ delta=0）
	}

	// ★★ 腿② 端到端成对负控：合成制品（假 `bin/zerg`）⇒ 真跑 ffpRun，成对判绿/红。
	if tmp2, err2 := os.MkdirTemp("", "check-fields-face-parity-selftest-"); err2 == nil {
		_ = os.MkdirAll(ffpPyJoin(tmp2, "core", "cmd", "zerg"), 0o755)
		body := "	commands = []*command{\n" +
			"		{path: []string{\"zz\", \"ffp\"}, fields: []string{\"model\", \"host\"}},\n	}\n"
		_ = os.WriteFile(ffpPyJoin(tmp2, ffpMainRel), []byte(body), 0o644)
		mk := func(lines string) string {
			p := ffpPyJoin(tmp2, "fakerz")
			_ = os.WriteFile(p, []byte("#!/bin/sh\n"+lines+"\n"), 0o755)
			return p
		}
		// 绿档：坏名报「未知字段 + 清单」；登记的两格都退 0 ⇒ 声明==公告 且 公告==可选。
		green := mk("f=\"\"\nfor a in \"$@\"; do f=\"$a\"; done\n" +
			"if [ \"$f\" = \"" + ffpBogus + "\" ]; then\n" +
			"  printf '%s\\n' \"zerg: " + ffpUnknownFieldMark + " \\\"" + ffpBogus + "\\\"\" \"" +
			ffpLegalFieldsPrefix + " host,model\" >&2\n  exit 2\nfi\nexit 0")
		rcG, resG := ffpRun(tmp2, green, 1, ffpSelfTestProbeTO, ffpSelfTestProbeTORepr, 1)
		chk("腿②端到端正控：假制品全接受 ⇒ run() rc=0", rcG == 0, ffpSelfTestWhy(rcG, resG))
		// ★ 红档（成对）：对登记格打「未知字段 <f>」**却**把 f 列进同条清单 = 「公告 ≠ 可选」⇒ 必红。
		red := mk("f=\"\"\nfor a in \"$@\"; do f=\"$a\"; done\n" +
			"printf '%s\\n' \"zerg: " + ffpUnknownFieldMark + " \\\"$f\\\"\" \"" +
			ffpLegalFieldsPrefix + " host,model\" >&2\nexit 2")
		rcR, resR := ffpRun(tmp2, red, 1, ffpSelfTestProbeTO, ffpSelfTestProbeTORepr, 1)
		hit := false
		for _, c := range resR.selContra {
			if c.path == "zz ffp" && c.field == "model" {
				hit = true
			}
		}
		chk("腿②端到端负控（公告≠可选 ⇒ 必红）：run() rc=1 且逐格点名", rcR == 1 && hit, ffpSelfTestWhy(rcR, resR))
		_ = os.RemoveAll(tmp2) // ★ 自回收
	}

	// ★ 腿② 不打靶白名单（自伤洞两刀之一 · 治本）：会起整套门的目标 ⇒ 记 not_probed 且**零执行**（假制品留痕缺席）。
	if tmp3, err3 := os.MkdirTemp("", "check-fields-face-parity-selftest-"); err3 == nil {
		_ = os.MkdirAll(ffpPyJoin(tmp3, "core", "cmd", "zerg"), 0o755)
		body := "\tcommands = []*command{\n" +
			"\t\t{path: []string{\"gate\", \"bench\"}, fields: []string{\"log\", \"rc\"}},\n" +
			"\t\t{path: []string{\"zz\", \"show\"}, fields: []string{\"alpha\"}},\n\t}\n"
		_ = os.WriteFile(ffpPyJoin(tmp3, ffpMainRel), []byte(body), 0o644)
		heavy := ffpPyJoin(tmp3, "HEAVY_RAN")
		normal := ffpPyJoin(tmp3, "NORMAL_RAN")
		script := "#!/bin/sh\ncase \"$*\" in\n" +
			"  'gate bench --json " + ffpBogus + "') echo \"" + ffpLegalFieldsPrefix + " log,rc\" >&2; exit 2;;\n" +
			"  'zz show --json " + ffpBogus + "') echo \"" + ffpLegalFieldsPrefix + " alpha\" >&2; exit 2;;\n" +
			"  'gate bench --json '*) : > '" + heavy + "'; exit 0;;\n" +
			"  'zz show --json alpha') : > '" + normal + "'; exit 0;;\n" +
			"  *) exit 0;;\nesac\n"
		bin := ffpPyJoin(tmp3, "fakerz")
		_ = os.WriteFile(bin, []byte(script), 0o755)
		rcNP, resNP := ffpRun(tmp3, bin, 2, ffpSelfTestProbeTO, ffpSelfTestProbeTORepr, 1)
		npHit := []string{}
		for _, c := range resNP.selNotProbed {
			if c.path == "gate bench" {
				npHit = append(npHit, c.field)
			}
		}
		sort.Strings(npHit)
		_, heavyErr := os.Stat(heavy)
		_, normalErr := os.Stat(normal)
		chk("腿②不打靶（自伤洞治本）：会起整套门的目标（gate bench）记 not_probed 且**零执行**",
			os.IsNotExist(heavyErr) && eqs(npHit, []string{"log", "rc"}),
			"rc="+strconv.Itoa(rcNP)+" np_hit="+ffpPyListRepr(npHit)+" heavy_ran="+strconv.FormatBool(heavyErr == nil))
		matched := false
		for _, m := range resNP.selMatched {
			if m == "zz show" {
				matched = true
			}
		}
		chk("腿②不打靶对照：正常命令（zz show）照打靶（假制品真被跑到）",
			normalErr == nil && matched,
			"matched="+strconv.FormatBool(matched)+" normal_ran="+strconv.FormatBool(normalErr == nil))
		_ = os.RemoveAll(tmp3) // ★ 自回收
	}

	// 负控⑥：拼接变量未定义 ⇒ 不当「有字段表」（不出假红、也不静默漏判成空名单）。
	if tmp6, err6 := os.MkdirTemp("", "check-fields-face-parity-selftest-"); err6 == nil {
		_ = os.MkdirAll(ffpPyJoin(tmp6, "core", "cmd", "zerg"), 0o755)
		body := "\tcommands = []*command{\n" +
			"\t\t{path: []string{\"zz\", \"gh\"}, fields: zzGhost, run: x},\n\t}\n"
		_ = os.WriteFile(ffpPyJoin(tmp6, ffpMainRel), []byte(body), 0o644)
		g6, none6, fatal6 := ffpDeclaredFaces(tmp6)
		rep6 := "["
		for i, g := range g6 {
			if i > 0 {
				rep6 += ", "
			}
			rep6 += "(" + ffpPyRepr(g.path) + ", " + ffpPyListRepr(g.fields) + ")"
		}
		rep6 += "]"
		chk("负控⑥：标识符未定义 ⇒ 不进声明面", !none6 && !fatal6 && len(g6) == 0, rep6)
		_ = os.RemoveAll(tmp6)
	}

	rc, _ := ffpRun("/nonexistent-zerg-root-zz", "/nonexistent-bin-zz", 1, 2*time.Second, "2", 1)
	chk("负控⑤：读不到命令树 ⇒ rc=2", rc == 2, "rc="+strconv.Itoa(rc))

	if ok {
		fmt.Fprint(stdout, "── 全过 ──\n")
		return 0
	}
	fmt.Fprint(stdout, "── 有红 ──\n")
	return 1
}

// ── 默认根（声明 (a)）─────────────────────────────────────────────────────────

func ffpDefaultRoot() string {
	d := ffpPyAbspath(ffpCwd(), ".")
	cur := d
	for {
		if fi, err := os.Stat(ffpPyJoin(cur, "scripts", "gates")); err == nil && fi.IsDir() {
			return cur
		}
		parent := ffpPyDirname(cur)
		if parent == cur || parent == "" {
			break
		}
		cur = parent
	}
	return d
}

// ── argparse 子集 ─────────────────────────────────────────────────────────────

type ffpArgs struct {
	selfTest, json, help bool
	root, bin            string
	minCovered           int
	minSelectable        int
	timeout              float64
	timeoutSet           bool
}

func ffpUsageHead() string {
	ind := strings.Repeat(" ", len("usage: ")+len(ffpProg)+1)
	return "usage: " + ffpProg + " [-h] [--self-test] [--json] [--bin BIN]\n" +
		ind + "[--min-covered MIN_COVERED]\n" +
		ind + "[--min-selectable MIN_SELECTABLE]\n" +
		ind + "[--timeout TIMEOUT]\n" +
		ind + "[root]\n"
}

func ffpHelpText() string {
	return ffpUsageHead() + "\n" +
		"positional arguments:\n" +
		"  root\n" +
		"\n" +
		"optional arguments:\n" +
		"  -h, --help            show this help message and exit\n" +
		"  --self-test\n" +
		"  --json\n" +
		"  --bin BIN\n" +
		"  --min-covered MIN_COVERED\n" +
		"  --min-selectable MIN_SELECTABLE\n" +
		"  --timeout TIMEOUT\n"
}

func ffpArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, ffpUsageHead())
	fmt.Fprintf(stderr, "%s: error: %s\n", ffpProg, msg)
	return 2
}

// ffpResolveOpt —— argparse 长选项解析：精确优先，否则唯一前缀（本表无歧义前缀）。
func ffpResolveOpt(body string) (string, bool) {
	longs := []string{"--help", "--self-test", "--json", "--bin", "--min-covered", "--min-selectable", "--timeout"}
	full := "--" + body
	for _, o := range longs {
		if o == full {
			return o, true
		}
	}
	hit := ""
	n := 0
	for _, o := range longs {
		if strings.HasPrefix(o, full) {
			hit = o
			n++
		}
	}
	if n == 1 {
		return hit, true
	}
	return "", false
}

func ffpParseArgs(stdout, stderr io.Writer, argv []string) (ffpArgs, int) {
	a := ffpArgs{minCovered: ffpMinCoveredDefault, minSelectable: ffpMinSelectableDefault,
		timeout: float64(ffpPerCmdTimeout)}
	unknown := []string{}
	pos := []string{}
	// valueOf：`--opt=值` 或「下一枚」。
	valueOf := func(i int, eq string) (string, int, bool) {
		if eq != "" {
			if eq == "=" {
				return "", i, false
			}
			return eq[1:], i, true
		}
		if i+1 >= len(argv) {
			return "", i, false
		}
		return argv[i+1], i + 1, true
	}
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if tok == "-h" {
			fmt.Fprint(stdout, ffpHelpText())
			return a, 0
		}
		if !strings.HasPrefix(tok, "--") || len(tok) <= 2 {
			pos = append(pos, tok)
			continue
		}
		body := tok[2:]
		eq := ""
		if j := strings.Index(body, "="); j >= 0 {
			body, eq = body[:j], "="+body[j+1:]
		}
		name, ok := ffpResolveOpt(body)
		if !ok {
			unknown = append(unknown, tok)
			continue
		}
		switch name {
		case "--help":
			fmt.Fprint(stdout, ffpHelpText())
			return a, 0
		case "--self-test":
			a.selfTest = true
		case "--json":
			a.json = true
		case "--bin", "--min-covered", "--min-selectable", "--timeout":
			v, ni, okv := valueOf(i, eq)
			if !okv {
				return a, ffpArgErr(stderr, "argument "+name+": expected one argument")
			}
			i = ni
			switch name {
			case "--bin":
				a.bin = v
			case "--min-covered":
				n, cerr := strconv.Atoi(strings.TrimSpace(v))
				if cerr != nil {
					return a, ffpArgErr(stderr, "argument --min-covered: invalid int value: '"+v+"'")
				}
				a.minCovered = n
			case "--min-selectable":
				n, cerr := strconv.Atoi(strings.TrimSpace(v))
				if cerr != nil {
					return a, ffpArgErr(stderr, "argument --min-selectable: invalid int value: '"+v+"'")
				}
				a.minSelectable = n
			case "--timeout":
				f, ferr := strconv.ParseFloat(strings.TrimSpace(v), 64)
				if ferr != nil {
					return a, ffpArgErr(stderr, "argument --timeout: invalid float value: '"+v+"'")
				}
				a.timeout = f
				a.timeoutSet = true
			}
		}
	}
	if len(pos) > 1 {
		unknown = append(unknown, pos[1:]...)
	}
	if len(pos) > 0 {
		a.root = pos[0]
	}
	if len(unknown) > 0 {
		return a, ffpArgErr(stderr, "unrecognized arguments: "+strings.Join(unknown, " "))
	}
	return a, -1
}

// ffpPyFloatRepr —— py `"{}".format(<float>)` 的常用形态（整值补 `.0`）。
func ffpPyFloatRepr(v float64) string {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if !strings.ContainsAny(s, ".eE") {
		s += ".0"
	}
	return s
}

// ── 入口（= py `main()`）──────────────────────────────────────────────────────

func checkFieldsFaceParityGateMain(stdout, stderr io.Writer, argv []string) int {
	a, rc := ffpParseArgs(stdout, stderr, argv)
	if rc >= 0 {
		return rc
	}
	if a.selfTest {
		fmt.Fprint(stdout, "── check-fields-face-parity 自检（成对负控 · 合成夹具 · 不碰真目标）──\n")
		return ffpSelfTest(stdout)
	}
	root := a.root
	if root == "" {
		root = ffpDefaultRoot()
	} else {
		root = ffpPyAbspath(ffpCwd(), root)
	}
	binpath := a.bin
	if binpath == "" {
		binpath = ffpPyJoin(root, "bin", "zerg")
	}
	timeoutRepr := strconv.Itoa(ffpPerCmdTimeout)
	if a.timeoutSet {
		timeoutRepr = ffpPyFloatRepr(a.timeout)
	}
	timeout := time.Duration(a.timeout * float64(time.Second))
	rc, res := ffpRun(root, binpath, a.minCovered, timeout, timeoutRepr, a.minSelectable)

	if res.fatal {
		// py 此档抛未捕 UnicodeDecodeError ⇒ 回溯 + rc=1；本件 rc 同、正文不同（未覆盖面 ③）。
		fmt.Fprintf(stderr, "%s: error: 读命令树时解码失败（非 UTF-8）\n", ffpProg)
		return rc
	}
	if a.json {
		item := ffpJSONItem(res)
		top := ffpObj(
			ffpKP{ffpK("schema"), ffpStr("zerg/v1")},
			ffpKP{ffpK("kind"), ffpStr("FieldsFaceParity")},
			ffpKP{ffpK("items"), ffpArr(item)},
			ffpKP{ffpK("meta"), ffpObj(
				ffpKP{ffpK("count"), ffpInt(1)},
				ffpKP{ffpK("source"), ffpStr("local（本机）")},
			)},
			ffpKP{ffpK("warnings"), ffpArr()},
			ffpKP{ffpK("truncated"), ffpBool(false)},
		)
		fmt.Fprintln(stdout, ffpDump(top))
		return rc
	}
	if res.errorMsg != "" {
		fmt.Fprintln(stdout, "✗ "+res.errorMsg)
		return rc
	}
	fmt.Fprintln(stdout, "== 命令级 `fields` ⟷ 面级 `--json` 键集 对拍（缺口 `Q-178`）· 公告 == 可选（`GAP-20261005-45`）==")
	fmt.Fprintf(stdout, "   声明侧（%s）：%d 条带字段表的命令\n", ffpMainRel, res.declared)
	fmt.Fprintf(stdout, "   真身侧（现跑 `--json %s` ⇒ `合法字段:` 表）：可对拍 %d 条 · 不可对拍 %d 条\n",
		ffpBogus, res.covered, len(res.notComparable))
	fmt.Fprintf(stdout, "   可选侧（逐枚跑登记字段 `--json <f>` · %d 条命令可断言）：可打靶 %d 格 · 不打靶 %d 格 · 棘轮 %d\n",
		res.covered, res.selCovered, len(res.selNotProbed), res.minSelectable)
	for _, x := range res.notComparable {
		fmt.Fprintf(stdout, "   ○ 不给静默：%s —— %s\n", x.path, x.reason)
	}
	for _, x := range res.mismatched {
		fmt.Fprintf(stdout, "   ✗ %s：声明多 %s · 真身多 %s\n",
			x.path, ffpPyListRepr(x.declaredExtra), ffpPyListRepr(x.runtimeExtra))
	}
	for _, x := range res.selRejected {
		fmt.Fprintf(stdout, "   ✗ 登记却选不出：%s --json %s\n", x.path, x.field)
	}
	for _, x := range res.selContra {
		fmt.Fprintf(stdout, "   ✗ 自相矛盾（同条清单列了却被判未知）：%s --json %s\n", x.path, x.field)
	}
	for _, x := range res.selUnreach {
		fmt.Fprintf(stdout, "   ○ 打靶格够不着：%s --json %s —— %s\n", x.path, x.field, x.reason)
	}
	for _, x := range res.selNotProbed {
		fmt.Fprintf(stdout, "   ○ 不打靶（会起整套门 / 长任务）：%s --json %s —— %s\n", x.path, x.field, x.reason)
	}
	fmt.Fprintln(stdout, "== 结论 ==")
	switch {
	case len(res.mismatched) > 0:
		fmt.Fprintf(stdout, "   ⇒ 红（rc=1）：声明集 ⟷ 真身集有逐名差异（%d 条）\n", len(res.mismatched))
	case len(res.selContra) > 0:
		fmt.Fprintf(stdout, "   ⇒ 红（rc=1）：有「公告 ≠ 可选」的自相矛盾（%d 格）\n", len(res.selContra))
	case len(res.selRejected) > 0:
		fmt.Fprintf(stdout, "   ⇒ 红（rc=1）：有「登记却选不出」的格（%d 格）\n", len(res.selRejected))
	case res.covered < res.minCovered:
		fmt.Fprintf(stdout, "   ⇒ 判不了（rc=2）：可对拍 %d < 棘轮 %d（**只许上调** —— 覆盖掉了就是空转）\n",
			res.covered, res.minCovered)
	case res.selCovered < res.minSelectable:
		fmt.Fprintf(stdout, "   ⇒ 判不了（rc=2）：可打靶 %d 格 < 棘轮 %d（**只许上调** —— 覆盖掉了就是空转）\n",
			res.selCovered, res.minSelectable)
	default:
		fmt.Fprintf(stdout, "   ⇒ 绿（rc=0）：%d 条逐名相等 · 可对拍 %d ≥ 棘轮 %d ｜ 公告 == 可选：可打靶 %d 格 ≥ 棘轮 %d\n",
			len(res.matched), res.covered, res.minCovered, res.selCovered, res.minSelectable)
	}
	return rc
}

// ffpJSONItem —— 机器读面里那一条：error 档**只有** `error` 一键（缺键形态），否则六键齐（空集合走 `[]`）。
func ffpJSONItem(res ffpResult) ffpJV {
	if res.errorMsg != "" {
		return ffpObj(ffpKP{ffpK("error"), ffpStr(res.errorMsg)})
	}
	items := []ffpJV{}
	for _, x := range res.mismatched {
		items = append(items, ffpObj(
			ffpKP{ffpK("path"), ffpStr(x.path)},
			ffpKP{ffpK("declared_extra"), ffpStrArr(x.declaredExtra)},
			ffpKP{ffpK("runtime_extra"), ffpStrArr(x.runtimeExtra)},
		))
	}
	nopes := []ffpJV{}
	for _, x := range res.notComparable {
		nopes = append(nopes, ffpObj(
			ffpKP{ffpK("path"), ffpStr(x.path)},
			ffpKP{ffpK("reason"), ffpStr(x.reason)},
		))
	}
	// 腿②「公告 == 可选」：拒格 / 自相矛盾格（各 `path`+`field`）· 够不着的格（另带 `reason`）。
	selCellArr := func(cells []ffpSelCell) ffpJV {
		out := []ffpJV{}
		for _, x := range cells {
			out = append(out, ffpObj(
				ffpKP{ffpK("path"), ffpStr(x.path)},
				ffpKP{ffpK("field"), ffpStr(x.field)},
			))
		}
		return ffpJV{kind: 'a', arr: out}
	}
	selUnr := []ffpJV{}
	for _, x := range res.selUnreach {
		selUnr = append(selUnr, ffpObj(
			ffpKP{ffpK("path"), ffpStr(x.path)},
			ffpKP{ffpK("field"), ffpStr(x.field)},
			ffpKP{ffpK("reason"), ffpStr(x.reason)},
		))
	}
	selNpArr := []ffpJV{}
	for _, x := range res.selNotProbed {
		selNpArr = append(selNpArr, ffpObj(
			ffpKP{ffpK("path"), ffpStr(x.path)},
			ffpKP{ffpK("field"), ffpStr(x.field)},
			ffpKP{ffpK("reason"), ffpStr(x.reason)},
		))
	}
	return ffpObj(
		ffpKP{ffpK("declared"), ffpInt(res.declared)},
		ffpKP{ffpK("covered"), ffpInt(res.covered)},
		ffpKP{ffpK("min_covered"), ffpInt(res.minCovered)},
		ffpKP{ffpK("matched"), ffpStrArr(res.matched)},
		ffpKP{ffpK("mismatched"), ffpJV{kind: 'a', arr: items}},
		ffpKP{ffpK("not_comparable"), ffpJV{kind: 'a', arr: nopes}},
		ffpKP{ffpK("selectable_covered"), ffpInt(res.selCovered)},
		ffpKP{ffpK("min_selectable"), ffpInt(res.minSelectable)},
		ffpKP{ffpK("selectable_matched"), ffpStrArr(res.selMatched)},
		ffpKP{ffpK("selectable_rejected"), selCellArr(res.selRejected)},
		ffpKP{ffpK("selectable_contradiction"), selCellArr(res.selContra)},
		ffpKP{ffpK("selectable_unreachable"), ffpJV{kind: 'a', arr: selUnr}},
		ffpKP{ffpK("selectable_not_probed"), ffpJV{kind: 'a', arr: selNpArr}},
	)
}
