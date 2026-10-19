// two_states_gate_mutate.go —— 判据件 `scripts/gates/two-states-gate-mutate.py`（674 行 · 甲案 C 类第 7 枚
// · py 现值 sha16 = ae378c7928843669）的 **Go 真搬**。
//
// 本件做什么（py 逐行现读后落）：判据 7 门禁（`scripts/sandbox-probes/verify-two-states.py --self-test --wall
// <茧壁>`）的**变异验证** —— 三条变异（G1 茧壁路线静默回落 / G2 路线起不来当全被拦 / G3 `claim_egress` 不再
// 判封闭态可达）各改坏一次门禁目标件，断言「门禁必须红 ⇒ 还原后 sha 逐字节一致 ⇒ 复跑必须绿」。
//
// ★★ 写面与「改仓内件档未搬」（甲案口径 · 本件与 py 的**唯一**行为分歧，单列点名）：
//
//	py 的**真跑档**会**真改仓内件**（`scripts/sandbox-probes/verify-two-states.py` 原地改坏 → `finally` 还原），
//	且要求 `wall/target/debug/zerg-wall` 在位。按甲案「缺省档只读」硬线，本 Go 实现**不照搬该档**：
//	  · **缺省档 = 只读**：只对目标件做只读结构断言（锚点各恰命中一次 · 变异体编译得过 · 探测串非空且不在原文），
//	    变异体只写**仓外**临时目录；绝不写任何仓内件 ⇒ 结论按「判不了 ≠ 绿」退 2。
//	  · 变异环只在**显式传 `--overlay <仓外树>`**时驱动（Go 独有旋钮 ⇒ 与 py **不可合同**，当场单列）；
//	    且 overlay 自身含 `.git` 时**拒在仓内改件**（退 2）。
//	⇒ 与 py 真跑档逐字对齐的档**未搬**（见件尾未覆盖面第 ① 条）。
//
// ★ 受治理（缺口 GAP-20261001-64 · Mr2109 2026-10-10 拍板「另立受治理的仓外写档」）：本件的**仓外写面**
//
//	（`--overlay <仓外树>`）已登**中央分类登记** `gateCheckClassEntry`（`gate_inproc_table.go` · 判据名 ⇒
//	write D2）⇒ 壳侧 `gate check` 缺 `--yes` 一律拒执；缺省档仍**只读退 2**、✗ 永不放开**仓内**真写。
//
// 外部交互点（逐行现读 py · 全件只有这些）：
//
//	· `subprocess`（50 行 import）两处：① `run_gate()`（:124-125）跑 `[sys.executable, ROOT/TARGET,
//	  "--self-test", "--wall", WALL_BIN]`，`cwd=ROOT`，stderr 合进 stdout；② `compiles()`（:130-132）跑
//	  `[sys.executable, "-c", py_compile…, path]`。⇒ Go 侧同形，解释器**钉死 `/usr/bin/python3`**（py 用
//	  `sys.executable`；对拍台一律用 `/usr/bin/python3` 起 py ⇒ 两侧同值 · GAP-20261001-61）。
//	· 写面：`mutate()`（:115 `open(path,"w")`）写**目标件**、`shutil.copy`（:186 备份 / :219 还原）、
//	  `shutil.rmtree`（:221）、`os.utime(path,None)`（:220）；自检面另有夹具写面（全在 `$TMPDIR` 仓外）。
//	  ⇒ Go：备份/还原/rmtree 全在**仓外** `os.MkdirTemp`；缺省档一条都不写仓内件。
//	· 读面：`sha()`（:90-95 分块 sha256）· `open(path, encoding="utf-8")`（:110 文本模式 = universal newlines）
//	  · `os.path.exists` → `os.path.isfile`（:156/:159）· `__file__`（:263）· `sys.executable`。
//	· 遍历面：`os.walk` / `os.listdir` / `os.scandir` / `glob` **命中 0** ⇒ 无遍历序问题（现读 `zerg code find`）。
//	· 计时/随机/环境/网络面：`time` / `datetime` / `random` / `socket` / `urllib` / `os.environ` **命中 0**
//	  ⇒ 输出不含时刻/随机 ⇒ **零掩码逐字节直比**；`tempfile.mkdtemp` 只用其**名字**（`prefix=`），路径不进输出。
//	· 输出序决定者：**唯一的**决定者是 `MUTATIONS` 列表序（:62-87）+ `args.only` 的过滤（:181）——
//	  `for m in MUTATIONS` 单层直序，无排序/无集合迭代/无 dict 迭代 ⇒ Go 用同序切片直序。
//
// 出厂声明 (a) **默认根定位差异**：py 的根 = `dirname×3(abspath(__file__))`（脚本在 `scripts/gates/` ⇒ 上溯三级
// = 仓根，:55）；Go 无 `__file__` 语境 ⇒ 从**当前工作目录**上溯 ≤6 层找含 `scripts/gates/` 的目录、找不到回落 cwd。
// 「脚本位置 ≠ cwd」时路径推导面不同 ⇒ 该面**不属逐字对齐承诺**，对拍一律显式传 `--overlay`（或令 cwd=树根）。
//
// 出厂声明 (b) **自指检查 —— 判定为「真」**（本件落 `core/cmd/zerg/`，落在下列各门扫描面内）：
//
//	· 私有绝对路径门（扫 `core/` 全树）：本件正文一处不写私有绝对路径前缀（含注释）⇒ 悬空字面计数 = 0。
//	· error.kind 闭集门（扫 `core/cmd/zerg/*.go` 的 kind 键值形态）：本件不写该键 ⇒ 计数 = 0。
//	· 茧壁层级门：本件不写层级常量/标记键值 ⇒ 计数 = 0。
//	· T3（裸数字退码 ⊆ 真源退码表）：本件退码一律走具名常量（0/1/2），无表外裸数字。
//	· 「处理有效」的硬取证 = 同根串影子仓两态（本件在/不在）跑上述各门 ⇒ 输出 sha16 逐字相同（见回执）。
//	· ★ 本件正文里的字面量（锚点/替换/夹具模板）**全是为「读被扫面」而存的**，且都以「拼串/转义」形态落地，
//	  ✗ 不以真引号原串直写任何会被他门扫到的路径或标记。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func twoStatesMutateGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码 —— 0 真跑全部按预期 / 1 真跑有一步不符合预期 / 2 硬失败（与 py 三码逐字对齐）
//
// 词法面逐处现读：`fnmatch`/`glob`/`normpath`/`abspath`/`relpath`/`realpath` **命中 0** ⇒ 只涉 `os.path.join`
// 一处（:56-57/:155/:124）⇒ 自写 `tsmPyJoin`（CPython posixpath.join 逐段规则）；`str.strip()` / `str.splitlines()`
// 按 py Unicode 语义自写（`tsmPyStrip` / `tsmPySplitLines`）；切片 `l[:200]` / `out[-1500:]` / `pre[:12]`
// 一律按**码点**切（`tsmHeadRunes` / `tsmTailRunes`）· ✗ 不按 byte；`bytes.decode("utf-8","replace")` 按非法
// **序列**替换（`tsmDecodeReplace`）。符号链接面：`isfile`/`exists` 一律 `os.Stat`（跟随链）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	tsmExitOK         = 0
	tsmExitUnexpected = 1
	tsmExitHard       = 2
)

// tsmTargetRel —— py `TARGET`（:56 · `os.path.join("scripts","sandbox-probes","verify-two-states.py")`）。
const tsmTargetRel = "scripts/sandbox-probes/verify-two-states.py"

// tsmWallRel —— py `os.path.relpath(WALL_BIN, ROOT)` 的读数（WALL_BIN = ROOT/wall/target/debug/zerg-wall）。
const tsmWallRel = "wall/target/debug/zerg-wall"

// tsmPython —— 钉死解释器（GAP-20261001-61）：py 用 `sys.executable`，对拍台对 py 一律用 /usr/bin/python3。
const tsmPython = "/usr/bin/python3"

// tsmMutDriverRel —— py `_MUT_DRIVER_REL`（:294 · 自检把「本件」硬拷贝进夹具的落点）。py 的落点＝
// `scripts/gates/two-states-gate-mutate.py`（`basename(SELF_PATH)` 带 `.py`）；Go 的「本件」是可执的 `zerg`
// 二进制（无后缀），而夹具里经 `gate check <键>` **重入**复用 ⇒ 落点必须带 `.py`：判据名录
// （`family_gate_check.go` 的 `gateCheckRoster`）只收 `.py`/`.sh` 与「无后缀＋可识别 shebang」活门，
// 无后缀的 Mach-O 两侧都不认 ⇒ 该键解析不到（不在册 ⇒ rc=2）⇒ 落点与 py 逐字同形。
const tsmMutDriverRel = "scripts/gates/two-states-gate-mutate.py"

// tsmFixVersionRel / tsmFixGateScriptRel —— 夹具**仓根判据件** ＋ **门禁最小入口**。夹具里的「本件」是
// `zerg` 二进制、经 `gate check` 重入 ⇒ CLI 侧要解析仓根（锚 `core/internal/version/version.go`，见 root.go）
// 且 `gate check` 要门禁最小入口（`scripts/gates/precommit-gates.sh` 自举件、缺即 rc=2）。两件只落**仓外
// 夹具树**（`$TMPDIR`），真仓一字不动。
const tsmFixVersionRel = "core/internal/version/version.go"
const tsmFixGateScriptRel = pathPrecommitGates
const tsmFixVersionSrc = "package version\n"
const tsmFixGateScriptSrc = "#!/bin/sh\nexit 0\n"

// tsmFixWallRel —— py `_FIX_WALL_REL`（:267）。
const tsmFixWallRel = "wall/target/debug/zerg-wall"

// tsmFixWallStub —— py `_FIX_WALL_STUB`（:268 · 哑茧壁件：本件只 `isfile` 它、自检不跑茧壁）。
const tsmFixWallStub = "#!/bin/sh\nexit 0\n"

// tsmDropLine —— py `_DROP_LINE`（:269）。
const tsmDropLine = "    pass  # 夹具：把 G1 那段摘掉（锚点命中 0 次 · 正文仍可编译）\n"

// tsmWantCases —— py `WANT_CASES`（:293 · 写死：少跑一格即断言失败）。
const tsmWantCases = 19

// tsmMutation —— py `MUTATIONS` 的元素（:62-87）。`anchor` 在目标件里必须**恰好命中一次**。
type tsmMutation struct {
	id     string
	why    string
	anchor string
	repl   string
}

// tsmMutations —— 逐字取自 py `MUTATIONS`（输出序的唯一决定者）。
var tsmMutations = []tsmMutation{
	{
		id:  "G1",
		why: "茧壁路线静默回落成原生黄金配方（探针照样输出、断言照样全过）",
		anchor: `    return [route, "run", "--spec", write_wall_spec(platform, probe, env_extra)]
`,
		repl: `    # 变异 G1：忽略 route，静默改用原生黄金配方
    return list(prefix_for(platform, profile if profile is not None else
                           ("pF.sb" if platform == "darwin" else "L2"))) \
        + [PY, os.path.join(PROBE_ROOT, probe)]
`,
	},
	{
		id:  "G2",
		why: "路线起不来时当成「全被拦」（起不来的沙箱看起来最安全）",
		anchor: `        raise ProbeError("rc=%d：%s\n%s\n命令：%s" % (p.returncode, probe,
                        (p.stderr or "").strip()[:400],
                        " ".join(display_path(a) for a in argv)))
`,
		repl: `        return {"unix_bind": "FAIL:refused", "loopback": "FAIL:refused",
                "egress": "BLOCKED:refused", "target": "refused"}
`,
	},
	{
		id:     "G3",
		why:    "断言 B 不再判「封闭态出网仍可达」（红的那一版被抹掉）",
		anchor: "    if classify_reachable(i_v):\n",
		repl:   "    if False and classify_reachable(i_v):\n",
	},
}

// tsmFixGateTpl —— py `FIX_GATE_TPL`（:274-342）逐字同构。自检夹具门禁（「有牙齿的镜子」最小模型）：
// `@@TOKENS@@` / `@@A1..3@@` 由 `tsmFixtureGateSrc` 现填。
const tsmFixGateTpl = `#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""夹具门禁（自检专用 · 仓外 · 「有牙齿的镜子」最小模型）。"""
import sys

TOKENS = [t[::-1] for t in @@TOKENS@@]     # **倒序存**：本行字面量不得被自己检出（自指陷阱）


def _detect():
    src = open(__file__, encoding="utf-8").read()
    for t in TOKENS:
        if t in src:
            return t
    return None


PY = sys.executable
PROBE_ROOT = "."


def write_wall_spec(platform, probe, env_extra):
    return "spec"


def prefix_for(platform, profile):
    return []


def classify_reachable(x):
    return True


def display_path(a):
    return str(a)


class ProbeError(Exception):
    pass


def route_cmd(route, platform, probe, env_extra, profile=None):
@@A1@@

def collect(p, probe, argv):
    if p is not None:
@@A2@@
    return {}


def guard(i_v):
@@A3@@
        return True


def main():
    if "--self-test" not in sys.argv:
        print("夹具门禁：只认 --self-test")
        return 0
    hit = _detect()
    if hit:
        print("✗ 夹具门禁判红：检出变异串「%s」⇒ 这条路线没有真的经过茧壁 ⇒ 硬失败" % hit)
        return 1
    print("夹具门禁：绿（未检出变异）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
`

// ── 可注入面（照 py 的 in-process swap：`_run_fixture_inproc` 的 `swap`）──────────────────────────
// 缺省 = 真实现（真起子进程）；测试件/自检可用 `ZERG_ZTS_MUT_MUTE`（夹具驱动进程内）换真源做防空绿。
var (
	tsmRunGateFn  = tsmRunGate
	tsmCompilesFn = tsmCompiles
	tsmSkipRestor = false
	tsmGateCalls  = 0
)

// ── 路径/文本原语（py 语义自写）─────────────────────────────────────────────────────────────

// tsmPyJoin —— CPython posixpath.join 逐段规则：后段绝对 ⇒ 丢前面全部；空段参与拼接（`a` + “ ⇒ `a/`）。
func tsmPyJoin(parts ...string) string {
	out := ""
	for _, p := range parts {
		if strings.HasPrefix(p, "/") {
			out = ""
		}
		if out == "" {
			out = p
			continue
		}
		if strings.HasSuffix(out, "/") {
			out += p
		} else {
			out += "/" + p
		}
	}
	return out
}

// tsmPyIsSpace —— py `str.isspace()`（含 \x1c-\x1f · Go `unicode.IsSpace` 不含这几个）。
func tsmPyIsSpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

// tsmPyStrip —— py `str.strip()`（Unicode 空白语义）。
func tsmPyStrip(s string) string {
	return strings.TrimFunc(s, tsmPyIsSpace)
}

// tsmPySplitLines —— py `str.splitlines()` 的分段集（本件输出只遇 \n / \r / \r\n，其余一并覆盖）。
func tsmPySplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			out = append(out, cur.String())
			cur.Reset()
			i += size
		case '\r':
			out = append(out, cur.String())
			cur.Reset()
			i += size
			if i < len(s) && s[i] == '\n' {
				i++
			}
		default:
			cur.WriteString(s[i : i+size])
			i += size
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// tsmHeadRunes / tsmTailRunes —— py 切片按**码点**（`l[:200]` · `out[-1500:]` · `pre[:12]`）。
func tsmHeadRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func tsmTailRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// tsmDecodeReplace —— py `bytes.decode("utf-8","replace")`：每个非法**序列**替换一个 U+FFFD。
func tsmDecodeReplace(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size <= 1 {
			want := 1
			switch c := b[i]; {
			case c >= 0xc2 && c <= 0xdf:
				want = 2
			case c >= 0xe0 && c <= 0xef:
				want = 3
			case c >= 0xf0 && c <= 0xf4:
				want = 4
			}
			n := 1
			for j := 1; j < want && i+j < len(b); j++ {
				if b[i+j]&0xc0 != 0x80 {
					break
				}
				n++
			}
			sb.WriteRune(utf8.RuneError)
			i += n
			continue
		}
		sb.Write(b[i : i+size])
		i += size
	}
	return sb.String()
}

// tsmReadText —— py `open(..., encoding="utf-8")` 文本模式（universal newlines）· 非法 UTF-8 = 判不了。
func tsmReadText(path string) (string, string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err.Error()
	}
	if !utf8.Valid(b) {
		return "", "非法 UTF-8 输入 ⇒ 判不了（py 侧为 UnicodeDecodeError 回溯）"
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s, ""
}

// tsmWriteText —— py `open(..., "w", encoding="utf-8")`（os.linesep = \n ⇒ 不另翻译）。
func tsmWriteText(path, text string) string {
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err.Error()
	}
	return ""
}

// tsmShaStr —— py `sha()`（:90-95 · 分块 sha256 十六进制）。
func tsmShaStr(path string) (string, string) {
	f, err := os.Open(path)
	if err != nil {
		return "", err.Error()
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err.Error()
	}
	return hex.EncodeToString(h.Sum(nil)), ""
}

// tsmApplyMutation —— py `apply_mutation()`（:98-106 · **纯函数**）：锚点恰一次 ⇒ ("", 变异后正文)；
// 否则 (说明, "", false)（**不写盘**）。
func tsmApplyMutation(text, anchor, repl string) (string, string, bool) {
	n := strings.Count(text, anchor)
	if n != 1 {
		return fmt.Sprintf("锚点命中 %d 次（应为 1）—— 未写盘", n), "", false
	}
	return "", strings.Replace(text, anchor, repl, 1), true
}

// tsmMutate —— py `mutate()`（:109-117）：读 ⇒ 应用 ⇒ 写；返回错说明（空串 = 成功）。
func tsmMutate(path, anchor, repl string) string {
	text, rerr := tsmReadText(path)
	if rerr != "" {
		return rerr
	}
	err, newText, ok := tsmApplyMutation(text, anchor, repl)
	if !ok {
		return err
	}
	return tsmWriteText(path, newText)
}

// tsmIsFile / tsmExists —— py `os.path.isfile` / `os.path.exists`（**跟随符号链接** ⇒ 一律 os.Stat）。
func tsmIsFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular()
}

func tsmExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// tsmRunGate —— py `run_gate()`（:120-126）：茧壁不在 ⇒ (2, 说明)；否则跑目标件 `--self-test --wall`。
func tsmRunGate(root string) (int, string) {
	wall := tsmPyJoin(root, tsmWallRel)
	if !tsmIsFile(wall) {
		return tsmExitHard, fmt.Sprintf("茧壁二进制不在 %s ⇒ 先 `cd wall && cargo build`", wall)
	}
	cmd := exec.Command(tsmPython, tsmPyJoin(root, tsmTargetRel), "--self-test", "--wall", wall)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return tsmExecCode(err), tsmDecodeReplace(out)
}

// tsmCompiles —— py `compiles()`（:129-133）：`python3 -c py_compile…` ⇒ (编译过?, 输出)。
func tsmCompiles(path string) (bool, string) {
	cmd := exec.Command(tsmPython, "-c",
		"import py_compile,sys; py_compile.compile(sys.argv[1], doraise=True)", path)
	out, err := cmd.CombinedOutput()
	return tsmExecCode(err) == 0, tsmDecodeReplace(out)
}

// tsmExecCode —— py `subprocess.run(...).returncode`；起不来时 py 侧裸回溯（未捕）⇒ fail-closed 退 2。
func tsmExecCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return tsmExitHard
}

// tsmDeriveRoot —— 出厂声明 (a)：从 cwd 上溯 ≤6 层找含 `scripts/gates/` 的目录，找不到回落 cwd。
func tsmDeriveRoot(cwd string) string {
	d := cwd
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
	return cwd
}

func tsmCwd() string {
	if d, err := os.Getwd(); err == nil {
		return d
	}
	return "."
}

// tsmPyReprStr / tsmPyRepr —— py `repr()`（本件只用于「一串不含撇号的串」⇒ 单引号形态即可）。
func tsmPyReprStr(s string) string {
	q := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case rune(q) == r:
			b.WriteByte('\\')
			b.WriteByte(byte(r))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(q)
	return b.String()
}

func tsmPyRepr(items []string) string {
	parts := make([]string, 0, len(items))
	for _, s := range items {
		parts = append(parts, tsmPyReprStr(s))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// tsmPyListInts —— py `%s` 打一个 int 列表：`[1, 1, 1]`。
func tsmPyListInts(v []int) string {
	parts := make([]string, 0, len(v))
	for _, n := range v {
		parts = append(parts, fmt.Sprintf("%d", n))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// tsmPyBool —— py `%s` 打 bool：True / False。
func tsmPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// ── 自检夹具面（仓外 · 硬拷贝 · 禁符号链接农场 · py :345-395 逐字同构）────────────────────────────

// tsmFixtureTokens —— py `_fixture_tokens()`（:345-347）：每条 `repl` 的**首行去空白**（自维护）。
func tsmFixtureTokens() []string {
	out := make([]string, 0, len(tsmMutations))
	for _, m := range tsmMutations {
		lines := tsmPySplitLines(m.repl)
		first := ""
		if len(lines) > 0 {
			first = tsmPyStrip(lines[0])
		}
		out = append(out, first)
	}
	return out
}

// tsmFixtureGateSrc —— py `_fixture_gate_src()`（:350-363）：三条锚点**逐字**从 `MUTATIONS` 取。
func tsmFixtureGateSrc(tokens []string, drop, dup string) string {
	toks := tokens
	if toks == nil {
		toks = tsmFixtureTokens()
	}
	rev := make([]string, 0, len(toks))
	for _, t := range toks {
		rev = append(rev, tsmReverseStr(t))
	}
	src := strings.Replace(tsmFixGateTpl, "@@TOKENS@@", tsmPyRepr(rev), 1)
	for i, m := range tsmMutations {
		a := m.anchor
		if m.id == drop {
			a = tsmDropLine
		}
		if m.id == dup {
			a = a + a
		}
		src = strings.Replace(src, fmt.Sprintf("@@A%d@@", i+1), a, 1)
	}
	return src
}

// tsmReverseStr —— py `s[::-1]`（按**码点**倒序）。
func tsmReverseStr(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// tsmFixtureTarget —— py `_fixture_target()`（:394-395）。
// ★ 空夹具 ⇒ 返回空串（**fail-closed**）：夹具搭不起来时若回落到相对路径，会落进**真仓**
// （血证：夹具建点失败那一路曾把 `<真仓>/scripts/sandbox-probes/verify-two-states.py` 删掉又清空）。
func tsmFixtureTarget(fix string) string {
	if fix == "" {
		return ""
	}
	return filepath.Join(fix, "scripts", "sandbox-probes", "verify-two-states.py")
}

// tsmWriteFixtureGate —— py `_write_fixture_gate()`（:370-372）。
func tsmWriteFixtureGate(fix string, tokens []string, drop, dup string) string {
	return tsmWriteText(tsmFixtureTarget(fix), tsmFixtureGateSrc(tokens, drop, dup))
}

// tsmCopyFile —— py `shutil.copy`（内容 + 权限位）。
func tsmCopyFile(src, dst string) string {
	b, err := os.ReadFile(src)
	if err != nil {
		return err.Error()
	}
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(src); err == nil {
		mode = fi.Mode().Perm()
	}
	if err := os.WriteFile(dst, b, mode); err != nil {
		return err.Error()
	}
	_ = os.Chmod(dst, mode)
	return ""
}

// tsmMkFixture —— py `_mk_fixture()`（:375-388）：仓外最小仓树 = **硬拷贝本驱动** + 夹具门禁 + 哑茧壁件。
func tsmMkFixture(base, driver, name, drop, dup string) (string, string) {
	fix, err := os.MkdirTemp(base, name+"-")
	if err != nil {
		return "", err.Error()
	}
	if e := os.MkdirAll(filepath.Join(fix, "scripts", "sandbox-probes"), 0o755); e != nil {
		return "", e.Error()
	}
	if e := os.MkdirAll(filepath.Dir(filepath.Join(fix, tsmMutDriverRel)), 0o755); e != nil {
		return "", e.Error()
	}
	if e := tsmCopyFile(driver, filepath.Join(fix, tsmMutDriverRel)); e != "" {
		return "", e
	}
	_ = os.Chmod(filepath.Join(fix, tsmMutDriverRel), 0o755)
	// ★ 夹具里的「本件」是 `zerg` 二进制、经 `gate check <键>` 重入 ⇒ CLI 侧要仓根判据件与门禁最小入口。
	if e := os.MkdirAll(filepath.Dir(filepath.Join(fix, tsmFixVersionRel)), 0o755); e != nil {
		return "", e.Error()
	}
	if e := tsmWriteText(filepath.Join(fix, tsmFixVersionRel), tsmFixVersionSrc); e != "" {
		return "", e
	}
	if e := tsmWriteText(filepath.Join(fix, tsmFixGateScriptRel), tsmFixGateScriptSrc); e != "" {
		return "", e
	}
	if e := os.Chmod(filepath.Join(fix, tsmFixGateScriptRel), 0o755); e != nil {
		return "", e.Error()
	}
	if e := tsmWriteFixtureGate(fix, nil, drop, dup); e != "" {
		return "", e
	}
	wall := filepath.Join(fix, tsmFixWallRel)
	if e := os.MkdirAll(filepath.Dir(wall), 0o755); e != nil {
		return "", e.Error()
	}
	if e := os.WriteFile(wall, []byte(tsmFixWallStub), 0o755); e != nil {
		return "", e.Error()
	}
	_ = os.Chmod(wall, 0o755)
	return fix, ""
}

// ── 入口 ────────────────────────────────────────────────────────────────────────────────

type tsmArgs struct {
	only      string
	selftest  bool
	pairGuard bool
	overlay   string
}

func tsmUsage() string {
	return "usage: two-states-gate-mutate [-h] [--only ONLY] [--self-test] [--selftest] [--pair-guard]\n" +
		"                                   [--overlay OVERLAY]\n" +
		"判据 7 门禁（--wall 路线）变异验证：改坏门禁，自检必须红\n"
}

// tsmParseArgs —— py argparse 面（`--only` / `--self-test`(`--selftest` 别名) / `--pair-guard`）
// ＋ Go 独有旋钮 `--overlay <仓外树>`。返回 (参数, 错误正文, 是否 -h/--help)。
func tsmParseArgs(argv []string) (tsmArgs, string, bool) {
	a := tsmArgs{}
	bad := func(msg string) (tsmArgs, string, bool) {
		return a, "usage: two-states-gate-mutate [-h] [--only ONLY] [--self-test] [--selftest] [--pair-guard]\n" +
			"two-states-gate-mutate: error: " + msg + "\n", false
	}
	need := func(i int, flag string) (string, int, string) {
		if i+1 >= len(argv) {
			return "", i, "argument " + flag + ": expected one argument"
		}
		return argv[i+1], i + 1, ""
	}
	for i := 0; i < len(argv); i++ {
		s := argv[i]
		switch {
		case s == "-h" || s == "--help":
			return a, "", true
		case s == "--self-test" || s == "--selftest":
			a.selftest = true
		case s == "--pair-guard":
			a.pairGuard = true
		case s == "--only":
			v, ni, e := need(i, "--only")
			if e != "" {
				return bad(e)
			}
			a.only, i = v, ni
		case strings.HasPrefix(s, "--only="):
			a.only = strings.TrimPrefix(s, "--only=")
		case s == "--overlay":
			v, ni, e := need(i, "--overlay")
			if e != "" {
				return bad(e)
			}
			a.overlay, i = v, ni
		case strings.HasPrefix(s, "--overlay="):
			a.overlay = strings.TrimPrefix(s, "--overlay=")
		case strings.HasPrefix(s, "-") && s != "-":
			return bad("unrecognized arguments: " + s)
		default:
			return bad("unrecognized arguments: " + s)
		}
	}
	return a, "", false
}

// tsmOnlyInRoster —— py `args.only not in {m["id"] for m in MUTATIONS}` 的成员判定（在册 id 集合）。
func tsmOnlyInRoster(id string) bool {
	for _, m := range tsmMutations {
		if m.id == id {
			return true
		}
	}
	return false
}

// tsmMutationIDsCSV —— py `",".join(m["id"] for m in MUTATIONS)`（在册 id 按序逗号拼接）。
func tsmMutationIDsCSV() string {
	ids := make([]string, 0, len(tsmMutations))
	for _, m := range tsmMutations {
		ids = append(ids, m.id)
	}
	return strings.Join(ids, ",")
}

// twoStatesMutateGateMain —— 入口（与 py `main()` 逐字对齐；唯一分歧 = 缺省档只读 · 见件头）。
func twoStatesMutateGateMain(stdout, stderr io.Writer, argv []string) int {
	if v := os.Getenv("ZERG_ZTS_MUT_MUTE"); v != "" {
		tsmInstallMute(v)
	}
	a, perr, help := tsmParseArgs(argv)
	if help {
		fmt.Fprint(stdout, tsmUsage())
		return tsmExitOK
	}
	if perr != "" {
		fmt.Fprint(stderr, perr)
		return tsmExitHard
	}

	// ★ 缺口 `GAP-20260926-181`：`--only <拼错的 id>`（如 `--only ZZ`）此前**一格不跑**却打进
	//   「变异验证全绿」并 rc=0 ⇒ 拼错 id 与真跑成功**同形**（假绿）。判据：`--only` 非空时必须命中
	//   在册 id，否则**判不了**（退码 2）—— 拼错 id ≠ 全绿。（逐字对 py :168-174）
	if a.only != "" && !tsmOnlyInRoster(a.only) {
		fmt.Fprintf(stderr, "!! [--only 校验] 未知的变异 id %s（在册：%s）⇒ 判不了（退码 2）—— 拼错 id ≠ 全绿\n",
			tsmPyReprStr(a.only), tsmMutationIDsCSV())
		return tsmExitHard
	}

	if a.selftest {
		fmt.Fprint(stdout, "── two-states-gate-mutate.py 自证（成对正负控 + 防空绿档 · 仓外夹具硬拷贝 · 真退出码）\n")
		return tsmSelfTest(stdout, stderr)
	}

	root := tsmDeriveRoot(tsmCwd())
	if a.overlay != "" {
		root = a.overlay
	}
	target := tsmPyJoin(root, tsmTargetRel)
	wall := tsmPyJoin(root, tsmWallRel)

	// py :156-158 —— 目标件不在 ⇒ 硬失败。
	if !tsmExists(target) {
		fmt.Fprintf(stdout, "!! 找不到目标 %s —— 硬失败\n", tsmTargetRel)
		return tsmExitHard
	}
	// py :159-162 —— 茧壁不在 ⇒ 硬失败。
	if !tsmIsFile(wall) {
		fmt.Fprintf(stdout, "!! 茧壁二进制不在 %s ⇒ 先 `cd wall && cargo build`（本脚本不替人编）—— 硬失败\n", tsmWallRel)
		return tsmExitHard
	}
	// py :164-170 —— 通则档：缺半即判不了（rc=2）。
	if a.pairGuard {
		ran := make([]string, 0, len(tsmMutations))
		for _, m := range tsmMutations {
			if a.only == "" || m.id == a.only {
				ran = append(ran, m.id)
			}
		}
		if len(ran) != len(tsmMutations) {
			fmt.Fprintf(stdout, "!! [通则·缺半即判不了] 本次只跑 %d/%d 条（%s）—— 阳性/阴性对照必须成对，"+
				"「只跑一半」= 判不了 ≠ 绿 ⇒ 硬失败（退码 2）\n",
				len(ran), len(tsmMutations), strings.Join(ran, ","))
			return tsmExitHard
		}
	}

	// ★★ 甲案硬线：缺省档**只读**（绝不改仓内件）⇒ 只做只读结构断言并报「改仓内件档未搬」。
	if a.overlay == "" {
		return tsmReadOnlyReport(stdout, target)
	}
	// ★★ 仓内件拒写（overlay 自身含 .git ⇒ 疑似真仓）。
	if tsmExists(tsmPyJoin(root, ".git")) {
		fmt.Fprintf(stdout, "!! --overlay %s 含 .git ⇒ 疑似真仓 ⇒ 拒在仓内改件"+
			"（本件只许在**仓外拷贝/overlay**上变异）—— 硬失败\n", root)
		return tsmExitHard
	}
	return tsmDrive(stdout, root, target, a)
}

// tsmDrive —— py `main()` :172-244 的变异环（写在**显式 overlay**上 · 备份/还原全在仓外临时目录）。
func tsmDrive(stdout io.Writer, root, target string, a tsmArgs) int {
	fmt.Fprintf(stdout, "=== 判据 7 门禁变异验证（目标 %s · 门禁 = --self-test --wall）===\n", tsmTargetRel)
	rc, out := tsmRunGateFn(root)
	fmt.Fprintf(stdout, "基线：门禁 rc=%d（期望 0）\n", rc)
	if rc != 0 {
		fmt.Fprintf(stdout, "!! 基线就不是绿的 ⇒ 硬失败（不许在红基线上做变异）：\n%s\n", tsmTailRunes(out, 1500))
		return tsmExitHard
	}

	bad := []string{}
	tbl := make([][3]string, 0, len(tsmMutations))
	for _, m := range tsmMutations {
		if a.only != "" && m.id != a.only {
			continue
		}
		pre, serr := tsmShaStr(target)
		if serr != "" {
			fmt.Fprintf(stdout, "!! [%s] 读目标件失败：%s\n", m.id, serr)
			return tsmExitHard
		}
		tmpdir, terr := os.MkdirTemp("", "zerg-two-states-mutate-")
		if terr != nil {
			fmt.Fprintf(stdout, "!! [%s] 建仓外临时目录失败：%s\n", m.id, terr.Error())
			return tsmExitHard
		}
		bak := filepath.Join(tmpdir, "verify-two-states.py")
		if e := tsmCopyFile(target, bak); e != "" {
			fmt.Fprintf(stdout, "!! [%s] 备到仓外失败：%s\n", m.id, e)
			return tsmExitHard
		}
		hard := -1
		func() {
			// py :218-221 —— `finally`：先回拷、再顶 mtime、后清临时目录（**还原写在 finally 里**）。
			defer func() {
				if !tsmSkipRestor {
					tsmCopyFile(bak, target)
				}
				now := time.Now()
				_ = os.Chtimes(target, now, now)
				_ = os.RemoveAll(tmpdir)
			}()
			if err := tsmMutate(target, m.anchor, m.repl); err != "" {
				fmt.Fprintf(stdout, "!! [%s] %s\n", m.id, err)
				hard = tsmExitHard
				return
			}
			post, _ := tsmShaStr(target)
			if post == pre {
				fmt.Fprintf(stdout, "!! [%s] 写盘后 sha 未变 —— 变异没生效\n", m.id)
				hard = tsmExitHard
				return
			}
			ok, cerr := tsmCompilesFn(target)
			if !ok {
				fmt.Fprintf(stdout, "!! [%s] 变异体编译不过 ⇒ 这条变异作废（红必须是判据红，不是语法错）：\n%s\n", m.id, cerr)
				hard = tsmExitHard
				return
			}
			rc, out = tsmRunGateFn(root)
			redOK := rc != 0
			crashed := strings.Contains(out, "Traceback (most recent call last)")
			mark := "✗"
			if redOK && !crashed {
				mark = "✓"
			}
			note := ""
			if crashed {
				note = "· 崩溃（traceback）⇒ 不算判据红"
			}
			fmt.Fprintf(stdout, "%s [%s] %s ⇒ 门禁 rc=%d（期望非 0）%s\n", mark, m.id, m.why, rc, note)
			switch {
			case !redOK:
				bad = append(bad, fmt.Sprintf("%s 变异后门禁仍绿 ⇒ 这面镜子抓不住：%s", m.id, m.why))
			case crashed:
				bad = append(bad, fmt.Sprintf("%s 变异后门禁崩了（traceback）⇒ 不是判据红，证据不足", m.id))
			default:
				hits := []string{}
				for _, l := range tsmPySplitLines(out) {
					s := tsmPyStrip(l)
					if strings.HasPrefix(s, "✗") || strings.Contains(s, "硬失败") || strings.Contains(s, "[✗]") {
						hits = append(hits, s)
					}
				}
				for i, l := range hits {
					if i >= 4 {
						break
					}
					fmt.Fprintf(stdout, "     红的原文：%s\n", tsmHeadRunes(l, 200))
				}
				if len(hits) == 0 {
					fmt.Fprintf(stdout, "     （门禁 rc=%d 但没打出 `✗`/硬失败字样 —— 复核输出尾部）\n%s\n",
						rc, tsmTailRunes(out, 800))
				}
			}
		}()
		if hard >= 0 {
			return hard
		}

		post, _ := tsmShaStr(target)
		restored := post == pre
		mark := "✗"
		if restored {
			mark = "✓"
		}
		detail := "逐字节一致"
		if !restored {
			detail = fmt.Sprintf("不一致！%s ≠ %s", post, pre)
		}
		fmt.Fprintf(stdout, "%s [%s] 还原后 sha %s\n", mark, m.id, detail)
		if !restored {
			bad = append(bad, fmt.Sprintf("%s 还原后 sha 不一致：%s ≠ %s（停手人工看）", m.id, post, pre))
			return tsmExitHard
		}
		rc, out = tsmRunGateFn(root)
		mark = "✗"
		if rc == 0 {
			mark = "✓"
		}
		fmt.Fprintf(stdout, "%s [%s] 复跑：rc=%d（期望 0）\n", mark, m.id, rc)
		if rc != 0 {
			bad = append(bad, fmt.Sprintf("%s 还原后复跑 rc=%d（期望 0）：%s", m.id, rc, tsmTailRunes(out, 800)))
		}
		tbl = append(tbl, [3]string{m.id, m.why, pre})
	}

	fmt.Fprint(stdout, "\n")
	if len(bad) > 0 {
		for _, b := range bad {
			fmt.Fprintf(stdout, "!! %s\n", b)
		}
		return tsmExitUnexpected
	}
	for _, r := range tbl {
		fmt.Fprintf(stdout, "  %s %s · 变异前 sha=%s…\n", r[0], r[1], tsmHeadRunes(r[2], 12))
	}
	fmt.Fprint(stdout, "变异验证全绿：每条都「红 ⇒ 还原 sha 一致 ⇒ 复跑绿」\n")
	return tsmExitOK
}

// tsmReadOnlyReport —— **缺省档（缺省只读）**：只读结构断言（py 自检 ⑬/⑭ 的只读面）＋ 明写「改仓内件档未搬」。
// 变异体只写**仓外**临时目录；绝不写任何仓内件 ⇒ 结论按「判不了 ≠ 绿」退 2。
func tsmReadOnlyReport(stdout io.Writer, target string) int {
	d, err := os.MkdirTemp("", "zerg-two-states-mutate-ro-")
	if err != nil {
		fmt.Fprintf(stdout, "!! 建仓外临时目录失败：%s\n", err.Error())
		return tsmExitHard
	}
	defer os.RemoveAll(d)

	rt, rerr := tsmReadText(target)
	detail := make([]string, 0, len(tsmMutations))
	counts := make([]int, 0, len(tsmMutations))
	mutOK := true
	for _, m := range tsmMutations {
		counts = append(counts, strings.Count(rt, m.anchor))
		p := filepath.Join(d, "mutant-"+m.id+".py")
		aerr, newText, ok := tsmApplyMutation(rt, m.anchor, m.repl)
		if !ok {
			mutOK = false
			detail = append(detail, fmt.Sprintf("%s:锚点不对（%s）", m.id, aerr))
			continue
		}
		if e := tsmWriteText(p, newText); e != "" {
			mutOK = false
			detail = append(detail, fmt.Sprintf("%s:写仓外变异体失败（%s）", m.id, e))
			continue
		}
		ok2, _ := tsmCompilesFn(p)
		mutOK = mutOK && ok2
		word := "编译过"
		if !ok2 {
			word = "**编译不过**"
		}
		detail = append(detail, m.id+":"+word)
	}
	toks := tsmFixtureTokens()
	tokOK := true
	for _, t := range toks {
		if t == "" || strings.Contains(rt, t) {
			tokOK = false
		}
	}
	ones := make([]int, 0, len(tsmMutations))
	for range tsmMutations {
		ones = append(ones, 1)
	}
	expect := tsmPyListInts(ones)
	tokWord := "非空且不在原文"
	if !tokOK {
		tokWord = "**有问题**"
	}
	fmt.Fprintf(stdout, "[真目标] 现读真仓 `%s` 的**只读结构**（不跑茧壁 · 不写仓 ⇒ 只做结构断言）\n", tsmTargetRel)
	fmt.Fprintf(stdout, "  变异 %d 条 · 锚点命中 %s（应 %s）· 变异体 %s · 探测串 %s\n",
		len(tsmMutations), tsmPyListInts(counts), expect, strings.Join(detail, " · "), tokWord)
	if rerr != "" {
		fmt.Fprintf(stdout, "  （目标件读面：%s）\n", rerr)
	}
	ok := (len(tsmMutations) == 3 && len(tsmDistinctIDs()) == 3 &&
		tsmEqualInts(counts, ones) && tsmDistinctAnchors() == len(tsmMutations) && mutOK && tokOK)
	fmt.Fprintf(stdout, "  只读结构断言：%s\n", map[bool]string{true: "全过", false: "不过"}[ok])
	fmt.Fprintf(stdout, "!! 本件真跑档（改仓内件 · 改坏 → `finally` 还原）**未搬**：缺省**只读** ⇒ 要驱动变异环请显式传"+
		" --overlay <仓外树>（该树含 .git 时拒写）；判不了 ≠ 绿 ⇒ 硬失败（退码 2）\n")
	return tsmExitHard
}

func tsmDistinctIDs() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, m := range tsmMutations {
		if !seen[m.id] {
			seen[m.id] = true
			out = append(out, m.id)
		}
	}
	return out
}

func tsmDistinctAnchors() int {
	seen := map[string]bool{}
	for _, m := range tsmMutations {
		seen[m.anchor] = true
	}
	return len(seen)
}

func tsmEqualInts(a, b []int) bool {
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

// tsmInstallMute —— 防空绿档的**注入旋钮**（Go 独有 · 只在夹具驱动子进程里由 env 打开；py 侧走 in-process swap）。
func tsmInstallMute(v string) {
	switch v {
	case "run_gate_zero":
		tsmRunGateFn = func(string) (int, string) { return 0, "夹具：镜子恒绿" }
	case "run_gate_crash":
		tsmRunGateFn = func(string) (int, string) {
			tsmGateCalls++
			if tsmGateCalls == 1 {
				return 0, "夹具：基线绿"
			}
			return 1, "Traceback (most recent call last):\n夹具：镜子崩了"
		}
	case "compiles_fail":
		tsmCompilesFn = func(string) (bool, string) { return false, "夹具：故意判编译不过" }
	case "restore_skip":
		tsmSkipRestor = true
	}
}

// tsmDriverPath —— 自检的「本件硬拷贝」源：默认可执行自身；测试/对拍台可用 env 指定探针二进制。
func tsmDriverPath() string {
	if v := os.Getenv("ZERG_ZTS_MUT_DRIVER"); v != "" {
		return v
	}
	if p, err := os.Executable(); err == nil {
		return p
	}
	return ""
}

// tsmRunFixtureDriver —— py `_run_fixture_driver()`（:423-427）：跑夹具里的**硬拷贝驱动**。
// ★ Go 侧重入必须走**真子命令形态**：夹具里的「本件」是 `zerg` 二进制，而 `zerg` 按 `args[0]` 派发
// （`main.go:77` `run(os.Args[1:], …)`、✗ 不认 `argv[0]`）⇒ 直跑 `--overlay …` 落「未知旗标」rc=2
// （18 格全 rc=2 的真因）。故 argv 首段带上 `gate check <键>`（照已落件先例 `check_approve_closedset.go`
// 的 `cacsSpawnSelf` 注入点 ＋ 路由写法）。夹具树须有仓根判据件与门禁最小入口（见 `tsmMkFixture`）。
// ★ 另多传 `--overlay <fix>`（旋钮面与 py 不可合同 ⇒ 单列点名），其余输出逐字对齐。
// ★ 且多传 `--yes`：本判据已登中央分类表（write D2）⇒ 壳侧缺 `--yes` 拒执 ⇒ 内部重入须自带确认盾
//
//	（壳侧 `gateCheckStripShellFlags` 会把它剔掉 · ✗ 不落给判据体本身）。
func tsmRunFixtureDriver(fix string, mute string, extra ...string) (int, string) {
	if fix == "" {
		// 夹具搭不起来 ⇒ **fail-closed 停手**：✗ 不许拿相对路径在真仓里跑（见 tsmFixtureTarget 注释）。
		return tsmExitHard, ""
	}
	argv := append([]string{"gate", "check", "two-states-gate-mutate", "--yes", "--overlay", fix}, extra...)
	cmd := exec.Command(filepath.Join(fix, tsmMutDriverRel), argv...)
	cmd.Dir = fix
	if mute != "" {
		cmd.Env = append(os.Environ(), "ZERG_ZTS_MUT_MUTE="+mute)
	}
	out, err := cmd.CombinedOutput()
	return tsmExecCode(err), tsmDecodeReplace(out)
}

// tsmSelfTest —— py `self_test()`（:477-670）逐字同构：成对正负控 + 退码钳 + 防空绿 + 真目标只读结构。
// ★ 夹具里的「本件」是 `zerg` 二进制，重入走 `gate check two-states-gate-mutate --overlay <fix>`（真子命令
// 形态 · 见 `tsmRunFixtureDriver`）⇒ 夹具树带仓根判据件与门禁最小入口（见 `tsmMkFixture`）。
func tsmSelfTest(stdout, stderr io.Writer) int {
	d, err := os.MkdirTemp("", "zts-mut-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "!! 建仓外自检根失败：%s\n", err.Error())
		return tsmExitHard
	}
	defer os.RemoveAll(d)

	driver := tsmDriverPath()
	root := tsmDeriveRoot(tsmCwd())
	realTarget := tsmPyJoin(root, tsmTargetRel)
	realSha0 := ""
	if tsmIsFile(realTarget) {
		realSha0, _ = tsmShaStr(realTarget)
	}

	bad, total := 0, 0
	caseFn := func(tag, desc string, ok bool, detail string) {
		total++
		if !ok {
			bad++
		}
		mark := "✗"
		if ok {
			mark = "✓"
		}
		fmt.Fprintf(stdout, "  %s %s %s\n       %s\n", mark, tag, desc, detail)
	}
	mk := func(name, drop, dup string) string {
		fix, e := tsmMkFixture(d, driver, name, drop, dup)
		if e != "" {
			fmt.Fprintf(stdout, "!! 搭夹具失败（%s）：%s\n", name, e)
		}
		return fix
	}

	fmt.Fprint(stdout, "── two-states-gate-mutate.py 自检（成对正负控 · 防空绿档 · 仓外夹具硬拷贝 · 不碰真目标）──\n")
	fmt.Fprint(stdout, "[正控] 健康夹具（硬拷贝的本件 + 有牙齿的夹具门禁）⇒ 三条变异都「红 ⇒ 还原 sha 一致 ⇒ 复跑绿」\n")
	fix := mk("healthy", "", "")
	rc, out := tsmRunFixtureDriver(fix, "")
	caseFn("①正控", "健康夹具 ⇒ rc=0 · 逐条印「（期望非 0）」「还原后 sha 逐字节一致」· 末行「变异验证全绿」",
		rc == 0 && strings.Count(out, "（期望非 0）") == len(tsmMutations) &&
			strings.Count(out, "还原后 sha 逐字节一致") == len(tsmMutations) &&
			strings.Count(out, "红的原文") >= len(tsmMutations) && strings.Contains(out, "变异验证全绿"),
		fmt.Sprintf("rc=%d（期望 0）· 变异档 %d/%d · 还原档 %d/%d · 末行 %s",
			rc, strings.Count(out, "（期望非 0）"), len(tsmMutations),
			strings.Count(out, "还原后 sha 逐字节一致"), len(tsmMutations),
			tsmYesNo(strings.Contains(out, "变异验证全绿"))))

	fmt.Fprint(stdout, "[负控] 把**夹具门禁**弄哑（探测串表清空 ⇒ 变异不再被检出）⇒ 驱动必须判红（镜子抓不住）\n")
	tsmWriteFixtureGate(fix, []string{}, "", "")
	rc, out = tsmRunFixtureDriver(fix, "")
	caseFn("②负控", "镜子对变异无感（恒绿）⇒ 驱动 rc=1 且逐条点名「镜子抓不住」· 不许报绿",
		rc == 1 && strings.Count(out, "镜子抓不住") == len(tsmMutations) && !strings.Contains(out, "变异验证全绿"),
		fmt.Sprintf("rc=%d（期望 1）· 点名 %d/%d", rc, strings.Count(out, "镜子抓不住"), len(tsmMutations)))

	fmt.Fprint(stdout, "[正控] 与 ② 成对：同一棵树、只把镜子的探测串放回 ⇒ 驱动回 rc=0（判红不是恒红）\n")
	tsmWriteFixtureGate(fix, nil, "", "")
	rc, out = tsmRunFixtureDriver(fix, "")
	caseFn("③正控", "复原（探测串放回）⇒ rc=0 且末行「变异验证全绿」（与 ② 成对：弄哑 ⇒ 判红 / 修回 ⇒ 回绿）",
		rc == 0 && strings.Contains(out, "变异验证全绿"), fmt.Sprintf("rc=%d（期望 0）", rc))

	fmt.Fprint(stdout, "[负控/退码钳] 锚点判据必须**恰一次**：命中 0 次 / 命中 2 次 ⇒ rc=2 且**不写盘**（不留半改的树）\n")
	f0 := mk("anchor0", "G1", "")
	t0 := tsmFixtureTarget(f0)
	s0, _ := tsmShaStr(t0)
	rc, out = tsmRunFixtureDriver(f0, "")
	s0b, _ := tsmShaStr(t0)
	caseFn("④负控", "锚点命中 0 次 ⇒ rc=2 且目标件 sha **逐字节不变**（逐字说明「未写盘」）",
		rc == 2 && strings.Contains(out, "锚点命中 0 次") && s0b == s0,
		fmt.Sprintf("rc=%d（期望 2）· 写盘 %s", rc, tsmWritten(s0b == s0)))

	fmt.Fprint(stdout, "[正控] 与 ④ 成对：把那段放回 ⇒ 驱动回 rc=0\n")
	tsmWriteFixtureGate(f0, nil, "", "")
	rc, out = tsmRunFixtureDriver(f0, "")
	caseFn("⑤正控", "复原（锚点回到恰一次）⇒ rc=0 且末行「变异验证全绿」（与 ④ 成对）",
		rc == 0 && strings.Contains(out, "变异验证全绿"), fmt.Sprintf("rc=%d（期望 0）", rc))

	f2 := mk("anchor2", "", "G1")
	t2 := tsmFixtureTarget(f2)
	s2, _ := tsmShaStr(t2)
	rc, out = tsmRunFixtureDriver(f2, "")
	s2b, _ := tsmShaStr(t2)
	caseFn("⑥负控", "锚点命中 2 次 ⇒ rc=2 且目标件 sha 逐字节不变（同一判据的另一向 · 不许「随便挑一处改」）",
		rc == 2 && strings.Contains(out, "锚点命中 2 次") && s2b == s2,
		fmt.Sprintf("rc=%d（期望 2）· 写盘 %s", rc, tsmWritten(s2b == s2)))

	fmt.Fprint(stdout, "[退码钳] 茧壁件 / 目标件不在 ⇒ rc=2（判不了 ≠ 绿 · 也不许当「变异没抓住」）\n")
	f7 := mk("nowall", "", "")
	_ = os.Remove(filepath.Join(f7, tsmFixWallRel))
	rc, out = tsmRunFixtureDriver(f7, "")
	caseFn("⑦退码钳", "夹具里茧壁件不在 ⇒ rc=2 且逐字说明「茧壁二进制不在」",
		rc == 2 && strings.Contains(out, "茧壁二进制不在"), fmt.Sprintf("rc=%d（期望 2）", rc))

	f8 := mk("notarget", "", "")
	_ = os.Remove(tsmFixtureTarget(f8))
	rc, out = tsmRunFixtureDriver(f8, "")
	caseFn("⑧退码钳", "夹具里目标件不在 ⇒ rc=2 且逐字说明「找不到目标」",
		rc == 2 && strings.Contains(out, "找不到目标"), fmt.Sprintf("rc=%d（期望 2）", rc))

	fmt.Fprint(stdout, "[防空绿档] 逐处弄哑关键真源 ⇒ 读数必须翻回（弄哑而不变 = 那条真源本来就是死的）\n")
	f9 := mk("mute-gate", "", "")
	rc, out = tsmRunFixtureDriver(f9, "run_gate_zero")
	caseFn("⑨防空绿", "弄哑 `run_gate`（恒返 0 ⇒ 镜子永不变红）⇒ 驱动必 rc=1（不是恒绿：抓不住就得报）",
		rc == 1 && strings.Count(out, "镜子抓不住") == len(tsmMutations),
		fmt.Sprintf("rc=%d（期望 1）· 点名 %d/%d", rc, strings.Count(out, "镜子抓不住"), len(tsmMutations)))

	f10 := mk("mute-crash", "", "")
	rc, out = tsmRunFixtureDriver(f10, "run_gate_crash")
	caseFn("⑩防空绿", "`run_gate` 只给「崩了」（traceback）⇒ 驱动必 rc=1（崩 ≠ 判据红 · 证据不足）",
		rc == 1 && strings.Contains(out, "不算判据红"),
		fmt.Sprintf("rc=%d（期望 1）· 点名「不算判据红」%s", rc, tsmYesNo(strings.Contains(out, "不算判据红"))))

	f11 := mk("mute-compile", "", "")
	rc, out = tsmRunFixtureDriver(f11, "compiles_fail")
	caseFn("⑪防空绿", "弄哑 `compiles`（恒判不过）⇒ 驱动必 rc=2（「红必须是判据红，不是语法错」是活的）",
		rc == 2 && strings.Contains(out, "变异体编译不过"), fmt.Sprintf("rc=%d（期望 2）", rc))

	f12 := mk("mute-restore", "", "")
	t12 := tsmFixtureTarget(f12)
	src12, _ := tsmReadText(t12)
	s12, _ := tsmShaStr(t12)
	rc, out = tsmRunFixtureDriver(f12, "restore_skip")
	s12b, _ := tsmShaStr(t12)
	written := s12b != s12
	_ = tsmWriteText(t12, src12)
	caseFn("⑫防空绿", "弄哑「还原」（`iw.restore` 不落地）⇒ 驱动必 rc=2 且点名「还原后 sha 不一致」",
		rc == 2 && strings.Contains(out, "还原后 sha 不一致") && written,
		fmt.Sprintf("rc=%d（期望 2）· 变异确实留在盘上 %s", rc, tsmBoolWord(written)))

	fmt.Fprintf(stdout, "[真目标] 现读真仓 `%s` 的**只读结构**（不跑茧壁 · 不写仓 ⇒ 只做结构断言）\n", tsmTargetRel)
	rt, _ := tsmReadText(realTarget)
	counts := make([]int, 0, len(tsmMutations))
	for _, m := range tsmMutations {
		counts = append(counts, strings.Count(rt, m.anchor))
	}
	mutOK, detail := true, []string{}
	for _, m := range tsmMutations {
		p := filepath.Join(d, "mutant-"+m.id+".py")
		aerr, newText, ok := tsmApplyMutation(rt, m.anchor, m.repl)
		if !ok {
			mutOK = false
			detail = append(detail, fmt.Sprintf("%s:锚点不对（%s）", m.id, aerr))
			continue
		}
		_ = tsmWriteText(p, newText)
		ok2, _ := tsmCompilesFn(p)
		mutOK = mutOK && ok2
		word := "编译过"
		if !ok2 {
			word = "**编译不过**"
		}
		detail = append(detail, m.id+":"+word)
	}
	toks := tsmFixtureTokens()
	tokOK := true
	for _, t := range toks {
		if t == "" || strings.Contains(rt, t) {
			tokOK = false
		}
	}
	ones := make([]int, 0, len(tsmMutations))
	for range tsmMutations {
		ones = append(ones, 1)
	}
	caseFn("⑬真目标",
		"真仓目标件：**三条**变异（id 互不相同 · 各锚点恰命中 1 次 · 三条互不相同）· 三个变异体都编译过 · 夹具探测串非空且不在原文",
		len(tsmMutations) == 3 && len(tsmDistinctIDs()) == 3 && tsmEqualInts(counts, ones) &&
			tsmDistinctAnchors() == len(tsmMutations) && mutOK && tokOK,
		fmt.Sprintf("变异 %d 条 · 锚点命中 %s（应 %s）· 变异体 %s · 探测串 %s",
			len(tsmMutations), tsmPyListInts(counts), tsmPyListInts(ones),
			strings.Join(detail, " · "), map[bool]string{true: "非空且不在原文", false: "**有问题**"}[tokOK]))

	broken := filepath.Join(d, "broken.py")
	_ = tsmWriteText(broken, "def broken(:\n")
	okBad, _ := tsmCompilesFn(broken)
	caseFn("⑭防空绿", "`compiles()` 的成对向：故意写坏的件必须判**编译不过**（否则「红 = 判据红」这条判据是死的）",
		!okBad && mutOK,
		fmt.Sprintf("坏件判过？%s（应 False）· 三个变异体都判过：%s", tsmPyBool(okBad), tsmBoolWord(mutOK)))

	realShaNow, _ := tsmShaStr(realTarget)
	caseFn("⑮不碰真目标", "真仓目标件 sha 与自检开始时**逐字节一致**（本自检全程只读 · 夹具全在仓外）",
		realSha0 != "" && realShaNow == realSha0, fmt.Sprintf("sha=%s…", tsmHeadRunes(realSha0, 12)))

	fmt.Fprint(stdout, "[通则] 对照必须成对：「只跑一半」不许读成绿（缺半 ⇒ 判不了 ≠ 绿）\n")
	rcHalf, outHalf := tsmRunFixtureDriver(fix, "", "--only", "G1", "--pair-guard")
	caseFn("⑯负控", fmt.Sprintf("`--only G1 --pair-guard`：只跑 1/%d 条 ⇒ rc=2 且逐字点名「缺半即判不了」（不报绿）", len(tsmMutations)),
		rcHalf == 2 && strings.Contains(outHalf, "缺半即判不了") && !strings.Contains(outHalf, "变异验证全绿"),
		fmt.Sprintf("rc=%d（期望 2）· 逐字 %s", rcHalf, tsmYesNo(strings.Contains(outHalf, "缺半即判不了"))))

	rcFull, outFull := tsmRunFixtureDriver(fix, "", "--pair-guard")
	caseFn("⑰正控", fmt.Sprintf("与 ⑯ 成对：`--pair-guard` 跑满 %d/%d 条 ⇒ rc=0 且末行「变异验证全绿」（判不了不是恒判不了）",
		len(tsmMutations), len(tsmMutations)),
		rcFull == 0 && strings.Contains(outFull, "变异验证全绿"), fmt.Sprintf("rc=%d（期望 0）", rcFull))

	rcOld, outOld := tsmRunFixtureDriver(fix, "", "--only", "G1")
	caseFn("⑱只增自证", "不带 `--pair-guard` 时 `--only G1` 仍 rc=0 —— 既有 `--only` 语义一字未改（本档只增）",
		rcOld == 0 && strings.Contains(outOld, "变异验证全绿"), fmt.Sprintf("rc=%d（期望 0 · 既有语义不动）", rcOld))

	rcTypo, outTypo := tsmRunFixtureDriver(fix, "", "--only", "ZZ")
	caseFn("⑲负控", "`--only ZZ`（拼错的 id）⇒ rc=2 且逐字点名「未知的变异 id」· 不报绿（拼错 ≠ 全绿 · 缺口 GAP-20260926-181）",
		rcTypo == 2 && strings.Contains(outTypo, "未知的变异 id") && !strings.Contains(outTypo, "变异验证全绿"),
		fmt.Sprintf("rc=%d（期望 2）· 逐字 %s", rcTypo, tsmYesNo(strings.Contains(outTypo, "未知的变异 id"))))

	fmt.Fprint(stdout, "  ⚠ 照实记（只报 · 不当绿）：本件**真跑**会真改仓（改坏 → `finally` 还原）且要求\n")
	fmt.Fprint(stdout, "     `wall/target/debug/zerg-wall` 在位 ⇒ 只挂在 `real-gates.sh` 的 `build` 档；\n")
	fmt.Fprint(stdout, "     本批只补自检面（判据与真跑退码一字未改）。\n")

	fmt.Fprintf(stdout, "  自证用例：%d 条 · 断言条数 %d（少跑一格即失效）\n", total, tsmWantCases)
	if total != tsmWantCases {
		fmt.Fprintf(stderr, "✗ 判不了：自检用例数对不上（应 %d，实跑 %d）⇒ rc=2（不许当绿）\n", tsmWantCases, total)
		return tsmExitHard
	}
	if bad == 0 {
		fmt.Fprint(stdout, "⇒ 自检 全过（rc=0）\n")
		return tsmExitOK
	}
	fmt.Fprintf(stdout, "⇒ 自检 有 %d 条断言不过（rc=1）\n", bad)
	return tsmExitUnexpected
}

// tsmYesNo / tsmWritten / tsmBoolWord —— 只用于把 py 的三元表达式读数逐字打成「有 / 无 / 是 / 否」。
func tsmYesNo(b bool) string {
	if b {
		return "有"
	}
	return "无"
}

func tsmWritten(b bool) string {
	if b {
		return "无"
	}
	return "**有！**"
}

func tsmBoolWord(b bool) string {
	if b {
		return "是"
	}
	return "否"
}
