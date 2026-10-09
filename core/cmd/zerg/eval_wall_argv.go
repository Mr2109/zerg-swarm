// eval_wall_argv.go —— 件 scripts/evals/compare-wall-argv.py（367 行）的 Go 真搬
// （迁移桥门禁面 · 入口 evalWallArgvGateMain）。
//
// 为什么有这件：茧壁的 Linux 一档是「同一个落点」的第二份实现 —— 两侧只要有一处顺手改进
// （少一个 --unshare-*、绑定顺序换一下、包装 exec 少个 $0），真机上就是「看着起来了、封闭面
// 却不同」的静默故障。本件是**不换件**前提下把这种漂移钉死的对拍门禁：Go 侧 hatch 装配的
// bwrap argv（经 agent/internal/hatch 的测试出口落一行 JSON）↔ Rust 侧 `zerg-wall plan`。
//
// 本件真身速览（逐处现读 py 原件 · 非按件名推断）：
//
//	· 形态 = 制品/子进程类判据（甲案口径：输入确定 ⇒ 输出逐字可对拍）：真起三个子进程 ——
//	  ① 取 Go 侧：`go test ./internal/hatch/ -run ^TestWallBridgeDumpBwrapArgv$ -count=1`
//	  （cwd=<仓根>/agent · env 补 ZERG_BRIDGE_SPEC_DIR / ZERG_BRIDGE_OUT · 产物一行 JSON）；
//	  ② 取茧壁侧：`<茧壁二进制> plan --spec <配方> --platform linux`（stdout 一行 JSON）；
//	  ③ cargo build（仅当茧壁二进制不在且未给 --no-build · 茧壁 crate 零依赖 ⇒ 零联网）。
//	  零联网；默认档不写盘（--evidence 才落一份回执）。
//	· 判据 = 文件名即期望值（`ok-*` 两侧都应接受 ⇒ 再比 argv；`reject-*` 两侧都应拒）。
//	  argv 两半：① argv[0] 必须是可执行名 bwrap（且不得以 '-' 开头）；② argv[1:] 与 Go 侧逐条
//	  相同（长度也要相同）。
//	· 退码 = 0 全绿 / 1 有对拍不一致 / 2 硬失败（探针/配方/产物缺失 · Go 侧跑不起来 ·
//	  茧壁二进制不在 · 无区分度 · 判据自检不过）。
//	· 输出 = 逐份一行「绿|红 + 名字 + 详情」+ 收尾一行；--self-test 追加判据自检；
//	  --evidence <文件> 落完整回执（逐份两侧 argv 原文 + 制品与源码 sha256）。
//
// 出厂声明 (a) 默认根定位差异：py 从自身文件位置上溯三层得仓根；Go 无「自身文件」语境 ⇒ 走
//
//	ZERG_REPO_ROOT（门禁转发壳 inproc 档必设）→ ZERG_REPO → repoRoot()（锚
//	core/internal/version/version.go）；三者皆取不到 ⇒ fail-closed 退 2（✗ 不静默回落 cwd）。
//	本件不吃仓外树、也不按脚本位置上溯 ⇒ 该差异面只在「脚本位置 ≠ cwd」时显形。
//
// 出厂声明 (b) 自指检查：本件落在 core/cmd/zerg 全件扫描面内 ⇒ 件内零私有绝对路径字面量、
//
//	零形如「引号 + 键名 + 引号」的连续协议原串（JSON 键名走常量），仓内相对路径只以拼串形态
//	出现（复核 0/0，见回执）。
//
// 本单边界（照实）：本件只落码、不接线 ——
//
//	· 不在 core/cmd/zerg/main.go 注册命令（✗ 不进命令树 · ✗ 不改步表）；
//	· 不删 py 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 evalWallArgvGateMain 目前无调用点（M2 的刻意状态）；
//	· ✗ 不改任何登记面 / family_*.go / 已落旧件。
//
// 入口签名（与 py 的 main() 对齐）：
//
//	func evalWallArgvGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与 py sys.argv[1:] 同形（不含程序名）
//	  · 退码 —— 同 py：0 全绿 / 1 对拍不一致 / 2 硬失败
//	  · 输出 —— stdout/stderr 与 py 逐字节对齐（见回执「已对拍面 / 未覆盖面」）
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ewaExpectOK / ewaExpectReject —— 期望值前缀（文件名即期望值）。
const (
	ewaExpectOK     = "ok-"
	ewaExpectReject = "reject-"
)

// 退码三档（与 py 的 EXIT_OK / EXIT_MISMATCH / EXIT_HARD 同号）。
const (
	ewaExitOK       = 0
	ewaExitMismatch = 1
	ewaExitHard     = 2
)

// ewaKeyArgv —— 两侧产物 JSON 里的 argv 键（走常量，✗ 不在件里直写连续协议原串）。
const ewaKeyArgv = "argv"

// ewaPySelfRelSegs —— py 原件在回执里记自己 sha 的那一条路径（✗ 直写整串，按段拼，防自指）。
var ewaPySelfRelSegs = []string{"scripts", "evals", "compare-wall-argv" + ".py"}

// ewaHelpText / ewaUsageText —— argparse 的帮助正文与错误用 usage 块（对拍台从 py 现读逐字节嵌入）。
const ewaHelpText = `usage: compare-wall-argv.py [-h] [--wall WALL] [--specs SPECS] [--no-build]
                            [--self-test] [--evidence EVIDENCE]

茧壁迁移桥：Go BuildBwrapArgv ↔ zerg-wall plan 逐条对拍

optional arguments:
  -h, --help           show this help message and exit
  --wall WALL          茧壁二进制路径（默认 wall/target/debug/zerg-wall）
  --specs SPECS        配方目录（默认 wall/testdata/bridge-specs）
  --no-build           二进制不在也不许编译（直接硬失败）
  --self-test          先跑判据自检（不过就拒绝当真值）
  --evidence EVIDENCE  把完整回执写到这个文件（可进仓）
`

const ewaUsageText = `usage: compare-wall-argv.py [-h] [--wall WALL] [--specs SPECS] [--no-build]
                            [--self-test] [--evidence EVIDENCE]
`

// --------------------------------------------------------------------------- //
// py 原语同构（路径 / 空白 / repr / 切片）
// --------------------------------------------------------------------------- //

// ewaPyJoin 照 CPython posixpath.join 逐段规则直译（★ 不用 filepath.Join：`..` 段 / 绝对后段 /
// 空段三类语义差）。本件用到的段都是常驻段，仍照写以求恒同构。
func ewaPyJoin(a string, p ...string) string {
	path := a
	for _, b := range p {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// ewaPyIsSpace 判 py str.isspace / 正则 \s 认的字符（★ 含 \x1c-\x1f、\x85、\xa0）。
func ewaPyIsSpace(r rune) bool {
	switch {
	case r == ' ' || (r >= '\t' && r <= '\r'):
		return true
	case r >= 0x1c && r <= 0x1f:
		return true
	case r == 0x85 || r == 0xa0:
		return true
	case r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200a:
		return true
	case r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000:
		return true
	}
	return false
}

// ewaPyStrip = py str.strip()（剥 py 的 Unicode 空白集）。
func ewaPyStrip(s string) string { return strings.TrimFunc(s, ewaPyIsSpace) }

// ewaPyReprStr 复刻 py3 对 str 的 repr（单双引号择优 + \t\n\r 与不可打印转义）。
// ★ 只覆盖本件会碰到的形态（ASCII + 常见空白）；≥0x80 的非可打印码点走原样输出（见未覆盖面）。
func ewaPyReprStr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == rune(quote) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// ewaPySliceHead 复刻 py 的 s[:n]（按码点切，✗ 不按字节）。
func ewaPySliceHead(s string, n int) string {
	if n < 0 {
		n = 0
	}
	rs := []rune(s)
	if len(rs) < n {
		n = len(rs)
	}
	return string(rs[:n])
}

// ewaPySliceTail 复刻 py 的 s[-n:]（按码点取尾 n 个；空串 ⇒ 空串）。
func ewaPySliceTail(s string, n int) string {
	if n <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[len(rs)-n:])
}

// ewaStrHead / ewaStrFrom 复刻 py 列表切片 w[:n] / w[n:] 的**宽容**语义（越界不 panic，
// 与 Go 原生切片不同 —— 血证：合成夹具里 ok 行的 wall_argv 为空时原生 w[:3] 会 panic）。
func ewaStrHead(s []string, n int) []string {
	if n < 0 {
		n = 0
	}
	if n >= len(s) {
		return append([]string{}, s...)
	}
	return append([]string{}, s[:n]...)
}

func ewaStrFrom(s []string, n int) []string {
	if n >= len(s) {
		return []string{}
	}
	if n < 0 {
		n = 0
	}
	return append([]string{}, s[n:]...)
}

// --------------------------------------------------------------------------- //
// JSON 解码错误正文（对齐 CPython json 的单行常见形态 · 与同族件同一口径）
// --------------------------------------------------------------------------- //

type ewaJErr struct {
	msg string
	pos int
}

func ewaJScanWS(rs []rune, i *int) {
	for *i < len(rs) && (rs[*i] == ' ' || rs[*i] == '\t' || rs[*i] == '\n' || rs[*i] == '\r') {
		*i++
	}
}

func ewaJScanString(rs []rune, i *int) (ewaJErr, bool) {
	start := *i
	*i++
	for *i < len(rs) {
		c := rs[*i]
		if c == '\\' {
			*i += 2
			continue
		}
		if c == '"' {
			*i++
			return ewaJErr{}, false
		}
		*i++
	}
	return ewaJErr{"Unterminated string starting at", start}, true
}

func ewaJScanNumber(rs []rune, i *int) (ewaJErr, bool) {
	for *i < len(rs) {
		c := rs[*i]
		if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
			*i++
			continue
		}
		break
	}
	return ewaJErr{}, false
}

func ewaJScanValue(rs []rune, i *int) (ewaJErr, bool) {
	ewaJScanWS(rs, i)
	if *i >= len(rs) {
		return ewaJErr{"Expecting value", *i}, true
	}
	c := rs[*i]
	switch {
	case c == '"':
		return ewaJScanString(rs, i)
	case c == '{':
		return ewaJScanObject(rs, i)
	case c == '[':
		return ewaJScanArray(rs, i)
	case c == 'n':
		if strings.HasPrefix(string(rs[*i:]), "null") {
			*i += 4
			return ewaJErr{}, false
		}
		return ewaJErr{"Expecting value", *i}, true
	case c == 't':
		if strings.HasPrefix(string(rs[*i:]), "true") {
			*i += 4
			return ewaJErr{}, false
		}
		return ewaJErr{"Expecting value", *i}, true
	case c == 'f':
		if strings.HasPrefix(string(rs[*i:]), "false") {
			*i += 5
			return ewaJErr{}, false
		}
		return ewaJErr{"Expecting value", *i}, true
	case c == '-' || (c >= '0' && c <= '9'):
		return ewaJScanNumber(rs, i)
	}
	return ewaJErr{"Expecting value", *i}, true
}

func ewaJScanObject(rs []rune, i *int) (ewaJErr, bool) {
	*i++
	ewaJScanWS(rs, i)
	if *i < len(rs) && rs[*i] == '}' {
		*i++
		return ewaJErr{}, false
	}
	for {
		ewaJScanWS(rs, i)
		if *i >= len(rs) || rs[*i] != '"' {
			return ewaJErr{"Expecting property name enclosed in double quotes", *i}, true
		}
		if err, bad := ewaJScanString(rs, i); bad {
			return err, true
		}
		ewaJScanWS(rs, i)
		if *i >= len(rs) || rs[*i] != ':' {
			return ewaJErr{"Expecting ':' delimiter", *i}, true
		}
		*i++
		if err, bad := ewaJScanValue(rs, i); bad {
			return err, true
		}
		ewaJScanWS(rs, i)
		if *i < len(rs) && rs[*i] == ',' {
			*i++
			continue
		}
		if *i < len(rs) && rs[*i] == '}' {
			*i++
			return ewaJErr{}, false
		}
		return ewaJErr{"Expecting ',' delimiter", *i}, true
	}
}

func ewaJScanArray(rs []rune, i *int) (ewaJErr, bool) {
	*i++
	ewaJScanWS(rs, i)
	if *i < len(rs) && rs[*i] == ']' {
		*i++
		return ewaJErr{}, false
	}
	for {
		if err, bad := ewaJScanValue(rs, i); bad {
			return err, true
		}
		ewaJScanWS(rs, i)
		if *i < len(rs) && rs[*i] == ',' {
			*i++
			continue
		}
		if *i < len(rs) && rs[*i] == ']' {
			*i++
			return ewaJErr{}, false
		}
		return ewaJErr{"Expecting ',' delimiter", *i}, true
	}
}

// ewaPyJSONErr 复刻 py json.loads 失败时 str(exc) 的正文（"<msg>: line 1 column N (char M)"）。
func ewaPyJSONErr(s string) string {
	rs := []rune(s)
	i := 0
	e, bad := ewaJScanValue(rs, &i)
	if bad {
		return fmt.Sprintf("%s: line 1 column %d (char %d)", e.msg, e.pos+1, e.pos)
	}
	ewaJScanWS(rs, &i)
	if i < len(rs) {
		return fmt.Sprintf("Extra data: line 1 column %d (char %d)", i+1, i)
	}
	return "Expecting value: line 1 column 1 (char 0)"
}

// --------------------------------------------------------------------------- //
// 判据层（纯函数：好让它自己被自检喂合成输入）
// --------------------------------------------------------------------------- //

// ewaExecutableVerdict 判据①：argv[0] 必须是可执行件名（bwrap），不是某个选项。
func ewaExecutableVerdict(wallArgv []string) string {
	if len(wallArgv) == 0 {
		return "argv 是空的 —— 没有可执行的东西"
	}
	head := wallArgv[0]
	if head != "bwrap" {
		return fmt.Sprintf("argv[0] 应是可执行名 bwrap，实得 %s", ewaPyReprStr(head))
	}
	if strings.HasPrefix(head, "-") {
		return fmt.Sprintf("argv[0] 以 '-' 开头 ⇒ `run` 永远起不来（缺陷形态）：%s", ewaPyReprStr(head))
	}
	return ""
}

// ewaArgvVerdict 判据②：argv[1:] 与 Go 侧 argv 逐条相同（长度与每一项都要）。
func ewaArgvVerdict(goArgv, wallArgv []string) string {
	tail := wallArgv[1:]
	if len(tail) != len(goArgv) {
		return fmt.Sprintf("长度不同：Go %d 项 / 茧壁 %d 项（去掉可执行名后）", len(goArgv), len(tail))
	}
	for i := range goArgv {
		if goArgv[i] != tail[i] {
			return fmt.Sprintf("第 %d 项不同：Go %s ≠ 茧壁 %s", i, ewaPyReprStr(goArgv[i]), ewaPyReprStr(tail[i]))
		}
	}
	return ""
}

// ewaWallPayload 从茧壁 stdout 取 argv：返回 (argv, 失败正文, ok)。
// ok=false ⇒ 不是一行合法 JSON 对象（compare_ok 走「不是一行合法 JSON」那支；
// 遍历档则按 py 的 except 吞成空 argv）。
func ewaWallPayload(raw string) ([]string, string, bool) {
	text := ewaPyStrip(raw)
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text), &m); err != nil || m == nil {
		return []string{}, ewaPyJSONErr(text), false
	}
	r, has := m[ewaKeyArgv]
	if !has || strings.TrimSpace(string(r)) == "null" {
		return []string{}, "", true
	}
	var arr []string
	if err := json.Unmarshal(r, &arr); err != nil {
		return []string{}, "", true
	}
	if arr == nil {
		arr = []string{}
	}
	return arr, "", true
}

// ewaCompareOK 好配方：两侧都应接受，再比 argv。返回 (bool, 详情)。
func ewaCompareOK(goRes ewaGoRes, wallRC int, wallStdout, wallStderr string) (bool, string) {
	if goRes.Error != "" {
		return false, fmt.Sprintf("Go 侧拒绝了本该通过的配方：%s", goRes.Error)
	}
	if wallRC != 0 {
		return false, fmt.Sprintf("茧壁 rc=%d（应为 0）：%s", wallRC, ewaPySliceHead(ewaPyStrip(wallStderr), 400))
	}
	argv, exc, ok := ewaWallPayload(wallStdout)
	if !ok {
		return false, fmt.Sprintf("茧壁 stdout 不是一行合法 JSON（%s）：%s",
			exc, ewaPyReprStr(ewaPySliceHead(wallStdout, 200)))
	}
	if bad := ewaExecutableVerdict(argv); bad != "" {
		return false, bad
	}
	if bad := ewaArgvVerdict(goRes.Argv, argv); bad != "" {
		return false, bad
	}
	return true, fmt.Sprintf("%d 项逐条一致（含可执行名共 %d 项）", len(goRes.Argv), len(argv))
}

// ewaCompareReject 坏配方：两侧都拒才算一致（任一侧放行 ⇒ 红）。
func ewaCompareReject(goRes ewaGoRes, wallRC int, wallStdout string) (bool, string) {
	if goRes.Error == "" {
		return false, "Go 侧放行了一枚本该被拒的配方（argv 都出来了）"
	}
	if wallRC == 0 {
		return false, fmt.Sprintf("茧壁放行了一枚本该被拒的配方：%s", ewaPySliceHead(ewaPyStrip(wallStdout), 300))
	}
	return true, fmt.Sprintf("两侧都拒（茧壁 rc=%d；Go：%s）", wallRC, ewaPySliceHead(goRes.Error, 160))
}

// ewaCompareOne 按文件名前缀分派。
func ewaCompareOne(name string, goRes ewaGoRes, wallRC int, wallStdout, wallStderr string) (bool, string) {
	if strings.HasPrefix(name, ewaExpectOK) {
		return ewaCompareOK(goRes, wallRC, wallStdout, wallStderr)
	}
	if strings.HasPrefix(name, ewaExpectReject) {
		return ewaCompareReject(goRes, wallRC, wallStdout)
	}
	return false, fmt.Sprintf("文件名前缀认不得（只有 %s / %s 两种；文件名即期望值，不许静默跳过）",
		ewaExpectOK, ewaExpectReject)
}

// --------------------------------------------------------------------------- //
// 两侧的取数（子进程）
// --------------------------------------------------------------------------- //

// ewaGoRes —— Go 侧产物里的一份：argv 与 error（缺键 / null 都归零值）。
type ewaGoRes struct {
	Argv  []string `json:"argv"`
	Error string   `json:"error"`
}

// ewaEnvHas / ewaEnvSetDefault / ewaEnvSet 在 []string 上复刻 dict(os.environ) 的读写。
func ewaEnvHas(env []string, k string) bool {
	p := k + "="
	for _, e := range env {
		if strings.HasPrefix(e, p) {
			return true
		}
	}
	return false
}

func ewaEnvSetDefault(env []string, k, v string) []string {
	if ewaEnvHas(env, k) {
		return env
	}
	return append(env, k+"="+v)
}

func ewaEnvSet(env []string, k, v string) []string {
	p := k + "="
	for i, e := range env {
		if strings.HasPrefix(e, p) {
			out := append([]string{}, env...)
			out[i] = k + "=" + v
			return out
		}
	}
	return append(env, k+"="+v)
}

// ewaRunGoDump 跑 Go 侧出口（-run 点名那一条；不带管道，直接拿真实 rc）。合并 stderr 进 stdout。
func ewaRunGoDump(agentDir, specsDir, outPath string) (int, string) {
	env := os.Environ()
	env = ewaEnvSetDefault(env, "GOFLAGS", "-mod=mod")
	env = ewaEnvSetDefault(env, "GOSUMDB", "off")
	env = ewaEnvSetDefault(env, "GOPROXY", "https://goproxy.cn,direct")
	env = ewaEnvSet(env, "ZERG_BRIDGE_SPEC_DIR", specsDir)
	env = ewaEnvSet(env, "ZERG_BRIDGE_OUT", outPath)
	cmd := exec.Command("go", "test", "./internal/hatch/", "-run", "^TestWallBridgeDumpBwrapArgv$", "-count=1")
	cmd.Dir = agentDir
	cmd.Env = env
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	rc := 0
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode()
	} else if err != nil {
		// py 侧此路是 subprocess 的 OSError（未捕 ⇒ 回溯 rc=1）；Go 取 1 并留一句（正文不同，见未覆盖面）。
		fmt.Fprintf(&buf, "eval_wall_argv: 起 go test 失败：%v\n", err)
		rc = 1
	}
	return rc, buf.String()
}

// ewaEnsureWallBin 茧壁二进制：不在就编（零依赖 crate ⇒ 不需要网络）。编不出来 ⇒ 硬失败。
func ewaEnsureWallBin(path, wallDir string, noBuild bool) (string, string) {
	if _, err := os.Stat(path); err == nil {
		return path, ""
	}
	if noBuild {
		return "", fmt.Sprintf("茧壁二进制 %s 不在，且指定了 --no-build（拒：不许静默跳过）", path)
	}
	cargo := ""
	if p, err := exec.LookPath("cargo"); err == nil {
		cargo = p
	} else if home, herr := os.UserHomeDir(); herr == nil {
		cargo = ewaPyJoin(home, ".cargo", "bin", "cargo")
	} else {
		cargo = ewaPyJoin("~", ".cargo", "bin", "cargo")
	}
	if _, err := os.Stat(cargo); err != nil {
		return "", "找不到 cargo（试过 PATH 与 ~/.cargo/bin）—— 先装 Rust 工具链"
	}
	cmd := exec.Command(cargo, "build", "--manifest-path", ewaPyJoin(wallDir, "Cargo.toml"))
	cmd.Dir = wallDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	rc := 0
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode()
	} else if err != nil {
		rc = -1
	}
	if rc != 0 {
		return "", fmt.Sprintf("cargo build 失败（rc=%d）：\n%s", rc, ewaPySliceTail(buf.String(), 2000))
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Sprintf("cargo build 说成功，但 %s 还是不在（拒绝把「编了」当成「编出来了」）", path)
	}
	return path, ""
}

// ewaRunWallPlan 跑茧壁侧 `plan`（stdout / stderr 分开）。
func ewaRunWallPlan(wallBin, specPath string) (int, string, string) {
	cmd := exec.Command(wallBin, "plan", "--spec", specPath, "--platform", "linux")
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	rc := 0
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode()
	} else if err != nil {
		rc = 1
	}
	return rc, so.String(), se.String()
}

// ewaSha256 流式 sha256（与 py hashlib.sha256 同值）。
func ewaSha256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if rerr != nil {
			break
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ewaRel 回执里的路径一律走仓内相对路径（跨盘 / 以上级开头 ⇒ 原样留）。
func ewaRel(path, root string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	if strings.HasPrefix(r, "..") {
		return path
	}
	return r
}

// --------------------------------------------------------------------------- //
// 自检：先证「这面镜子能红」
// --------------------------------------------------------------------------- //

// ewaSelfTestRows —— 自检里要用到的那一行（真实产物的三面改坏）。
type ewaSelfTestRow struct {
	name     string
	goArgv   []string
	wallArgv []string
}

// ewaSelfTest 两条：①合成输入喂判据层；②拿真实产物改坏一侧，判据必须红。
func ewaSelfTest(rows []ewaSelfTestRow, emit func(string)) bool {
	ok := true
	check := func(label string, cond bool, detail ...string) {
		dt := ""
		if len(detail) > 0 {
			dt = detail[0]
		}
		if cond {
			emit("  [自检 ok] " + label)
		} else {
			ok = false
			emit("  [自检 红] " + label + " " + dt)
		}
	}
	goodGo := []string{"--ro-bind", "/usr", "/usr", "--unshare-pid", "--die-with-parent"}
	goodWall := append([]string{"bwrap"}, goodGo...)

	check("① 两侧一致 ⇒ 绿", ewaExecutableVerdict(goodWall) == "" && ewaArgvVerdict(goodGo, goodWall) == "")
	mid := append(append([]string{}, goodWall[:2]...), "--ro-bind-x")
	mid = append(mid, goodWall[3:]...)
	check("② 中段漂移 ⇒ 红", ewaArgvVerdict(goodGo, mid) != "")
	check("③ 少一项 ⇒ 红", ewaArgvVerdict(goodGo, goodWall[:len(goodWall)-1]) != "")
	check("④ 可执行名不对 ⇒ 红", ewaExecutableVerdict(append([]string{"--ro-bind"}, goodGo...)) != "")
	check("⑤ 可执行名以 '-' 开头 ⇒ 红", ewaExecutableVerdict(append([]string{"-bwrap"}, goodGo...)) != "")
	check("⑥ 空 argv ⇒ 红", ewaExecutableVerdict([]string{}) != "")

	for _, row := range rows {
		if !strings.HasPrefix(row.name, ewaExpectOK) {
			continue
		}
		g, w := row.goArgv, row.wallArgv
		check(fmt.Sprintf("① 真实产物 %s：原样 ⇒ 绿", row.name),
			ewaExecutableVerdict(w) == "" && ewaArgvVerdict(g, w) == "")
		drift := append(append([]string{}, ewaStrHead(w, 3)...), "--mutation-drift")
		drift = append(drift, ewaStrFrom(w, 4)...)
		check(fmt.Sprintf("② 真实产物 %s：茧壁中段改坏 ⇒ 红", row.name), ewaArgvVerdict(g, drift) != "")
		check(fmt.Sprintf("③ 真实产物 %s：茧壁可执行名改坏 ⇒ 红", row.name),
			ewaExecutableVerdict(append([]string{"sandbox-exec"}, ewaStrFrom(w, 1)...)) != "")
		break // 一份够；多份只是重复同一件事
	}
	return ok
}

// --------------------------------------------------------------------------- //
// 参数解析（argparse 子集同构：唯一前缀缩写 / = 值 / -h / --）
// --------------------------------------------------------------------------- //

type ewaArgs struct {
	wall          string
	specs         string
	noBuild       bool
	selfTest      bool
	evidence      string
	helpRequested bool
	parseErr      string
	unrecognized  []string
}

var ewaLongOpts = []string{"--help", "--wall", "--specs", "--no-build", "--self-test", "--evidence"}

func ewaTakesValue(opt string) bool {
	switch opt {
	case "--wall", "--specs", "--evidence":
		return true
	}
	return false
}

func ewaArgLooksLikeOption(t string) bool {
	if len(t) < 2 || t[0] != '-' {
		return false
	}
	if strings.HasPrefix(t, "--") {
		return true
	}
	return true
}

func ewaResolveLong(name string) (string, string, bool) {
	for _, o := range ewaLongOpts {
		if o == name {
			return o, "", true
		}
	}
	cands := []string{}
	for _, o := range ewaLongOpts {
		if strings.HasPrefix(o, name) {
			cands = append(cands, o)
		}
	}
	if len(cands) == 1 {
		return cands[0], "", true
	}
	if len(cands) > 1 {
		return "", strings.Join(cands, ", "), false
	}
	return "", "", false
}

func ewaParseArgs(argv []string, defWall, defSpecs string) *ewaArgs {
	a := &ewaArgs{wall: defWall, specs: defSpecs}
	i := 0
	for i < len(argv) {
		tok := argv[i]
		if tok == "--" {
			a.unrecognized = append(a.unrecognized, tok)
			i++
			for i < len(argv) {
				a.unrecognized = append(a.unrecognized, argv[i])
				i++
			}
			break
		}
		if strings.HasPrefix(tok, "--") {
			name, val, hasVal := tok, "", false
			if eq := strings.Index(tok, "="); eq >= 0 {
				name, val, hasVal = tok[:eq], tok[eq+1:], true
			}
			canon, amb, ok := ewaResolveLong(name)
			if !ok {
				if amb != "" {
					a.parseErr = fmt.Sprintf("ambiguous option: %s could match %s", name, amb)
					return a
				}
				a.unrecognized = append(a.unrecognized, tok)
				i++
				continue
			}
			if canon == "--help" {
				if hasVal {
					a.parseErr = fmt.Sprintf("argument --help: ignored explicit argument %s", ewaPyReprStr(val))
					return a
				}
				a.helpRequested = true
				return a
			}
			if ewaTakesValue(canon) {
				if !hasVal {
					if i+1 < len(argv) && !ewaArgLooksLikeOption(argv[i+1]) {
						val, hasVal, i = argv[i+1], true, i+1
					} else {
						a.parseErr = fmt.Sprintf("argument %s: expected one argument", canon)
						return a
					}
				}
				switch canon {
				case "--wall":
					a.wall = val
				case "--specs":
					a.specs = val
				case "--evidence":
					a.evidence = val
				}
			} else {
				if hasVal {
					a.parseErr = fmt.Sprintf("argument %s: ignored explicit argument %s", canon, ewaPyReprStr(val))
					return a
				}
				switch canon {
				case "--no-build":
					a.noBuild = true
				case "--self-test":
					a.selfTest = true
				}
			}
			i++
			continue
		}
		if len(tok) > 1 && tok[0] == '-' {
			if tok == "-h" {
				a.helpRequested = true
				return a
			}
			a.unrecognized = append(a.unrecognized, tok)
			i++
			continue
		}
		a.unrecognized = append(a.unrecognized, tok)
		i++
	}
	return a
}

// --------------------------------------------------------------------------- //
// 仓根
// --------------------------------------------------------------------------- //

// ewaRepoRoot 见出厂声明 (a)：ZERG_REPO_ROOT → ZERG_REPO → repoRoot()。
func ewaRepoRoot() string {
	if v := strings.TrimSpace(os.Getenv("ZERG_REPO_ROOT")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("ZERG_REPO")); v != "" {
		return v
	}
	return repoRoot()
}

// --------------------------------------------------------------------------- //
// 入口
// --------------------------------------------------------------------------- //

// evalWallArgvGateMain = 件入口（argv 与 py sys.argv[1:] 同形）。
func evalWallArgvGateMain(stdout, stderr io.Writer, argv []string) int {
	root := ewaRepoRoot()
	if root == "" {
		fmt.Fprint(stderr, "compare-wall-argv.py: 解析不到仓根（ZERG_REPO_ROOT / ZERG_REPO / 锚 core/internal/version/version.go 皆不可得）⇒ 硬失败（不给结论 · rc=2）\n")
		return ewaExitHard
	}
	specsDefault := ewaPyJoin(root, "wall", "testdata", "bridge-specs")
	agentDir := ewaPyJoin(root, "agent")
	wallDir := ewaPyJoin(root, "wall")
	defaultWallBin := ewaPyJoin(wallDir, "target", "debug", "zerg-wall")

	a := ewaParseArgs(argv, defaultWallBin, specsDefault)
	if a.helpRequested {
		fmt.Fprint(stdout, ewaHelpText)
		return ewaExitOK
	}
	if a.parseErr != "" {
		fmt.Fprint(stderr, ewaUsageText)
		fmt.Fprintf(stderr, "compare-wall-argv.py: error: %s\n", a.parseErr)
		return ewaExitHard
	}
	if len(a.unrecognized) > 0 {
		fmt.Fprint(stderr, ewaUsageText)
		fmt.Fprintf(stderr, "compare-wall-argv.py: error: unrecognized arguments: %s\n", strings.Join(a.unrecognized, " "))
		return ewaExitHard
	}

	lines := []string{}
	say := func(msg string) {
		fmt.Fprint(stdout, msg+"\n")
		lines = append(lines, msg)
	}

	// ① 配方目录：不存在 / 没有 ok 或没有 reject ⇒ 硬失败（无区分度）
	if st, err := os.Stat(a.specs); err != nil || !st.IsDir() {
		fmt.Fprintf(stdout, "!! 配方目录不在：%s —— 硬失败（不许静默当「没东西可比」）\n", a.specs)
		return ewaExitHard
	}
	ents, err := os.ReadDir(a.specs)
	if err != nil {
		fmt.Fprintf(stdout, "!! 配方目录读不动：%s（%v）—— 硬失败\n", a.specs, err)
		return ewaExitHard
	}
	names := []string{}
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	nOK, nRej := 0, 0
	for _, n := range names {
		if strings.HasPrefix(n, ewaExpectOK) {
			nOK++
		}
		if strings.HasPrefix(n, ewaExpectReject) {
			nRej++
		}
	}
	if len(names) == 0 || nOK == 0 || nRej == 0 {
		fmt.Fprintf(stdout, "!! 配方集合没有区分度（共 %d 份：ok %d / reject %d）—— 硬失败："+
			"「零条对拍、零条失败」的绿比红更危险\n", len(names), nOK, nRej)
		return ewaExitHard
	}
	unknown := []string{}
	for _, n := range names {
		if !strings.HasPrefix(n, ewaExpectOK) && !strings.HasPrefix(n, ewaExpectReject) {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) > 0 {
		fmt.Fprintf(stdout, "!! 认不得的文件名前缀：%s —— 硬失败（文件名即期望值）\n", strings.Join(unknown, ", "))
		return ewaExitHard
	}

	// ② 茧壁二进制
	wallBin, why := ewaEnsureWallBin(a.wall, wallDir, a.noBuild)
	if wallBin == "" {
		fmt.Fprintf(stdout, "!! %s\n", why)
		return ewaExitHard
	}

	// ③ Go 侧产物（落本次自建的临时目录，函数返回时即清 —— ✗ 不落固定路径 /tmp/zerg-bridge-go-*.json）
	tmpDir, mkErr := os.MkdirTemp("", "zerg-bridge-go-")
	if mkErr != nil {
		fmt.Fprintf(stdout, "!! 建临时目录失败（%v）—— 硬失败（不许落固定路径）\n", mkErr)
		return ewaExitHard
	}
	defer os.RemoveAll(tmpDir)
	dumpPath := ewaPyJoin(tmpDir, "zerg-bridge-go.json")
	goRC, goLog := ewaRunGoDump(agentDir, ewaAbs(a.specs), dumpPath)
	if goRC != 0 {
		fmt.Fprintf(stdout, "!! Go 侧出口跑不起来（rc=%d）：\n%s\n", goRC, ewaPySliceTail(goLog, 2000))
		return ewaExitHard
	}
	if _, err := os.Stat(dumpPath); err != nil {
		fmt.Fprintf(stdout, "!! Go 侧出口跑不起来（rc=%d）：\n%s\n", goRC, ewaPySliceTail(goLog, 2000))
		return ewaExitHard
	}
	dumpBytes, err := os.ReadFile(dumpPath)
	if err != nil {
		fmt.Fprintf(stdout, "!! 读不出 Go 侧产物 %s：%v\n", dumpPath, err)
		return ewaExitHard
	}
	goDump := map[string]ewaGoRes{}
	if err := json.Unmarshal(dumpBytes, &goDump); err != nil {
		fmt.Fprintf(stdout, "!! 读不出 Go 侧产物 %s：%v\n", dumpPath, err)
		return ewaExitHard
	}

	missing := []string{}
	for _, n := range names {
		if _, ok := goDump[n]; !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(stdout, "!! Go 侧产物缺这几份配方：%s —— 硬失败（缺 = 没比过，不是通过）\n", strings.Join(missing, ", "))
		return ewaExitHard
	}

	// ④ 逐份对拍
	type ewaRow struct {
		name       string
		ok         bool
		detail     string
		goArgv     []string
		goError    string
		wallRC     int
		wallArgv   []string
		wallStderr string
	}
	rows := []ewaRow{}
	badCount := 0
	for _, n := range names {
		specPath := ewaPyJoin(a.specs, n)
		wallRC, wallOut, wallErr := ewaRunWallPlan(wallBin, specPath)
		okOne, detail := ewaCompareOne(n, goDump[n], wallRC, wallOut, wallErr)
		wallArgv := []string{}
		if wallRC == 0 && ewaPyStrip(wallOut) != "" {
			if arr, _, ok := ewaWallPayload(wallOut); ok {
				wallArgv = arr
			}
		}
		goArgv := goDump[n].Argv
		if goArgv == nil {
			goArgv = []string{}
		}
		rows = append(rows, ewaRow{
			name: n, ok: okOne, detail: detail,
			goArgv: goArgv, goError: goDump[n].Error,
			wallRC: wallRC, wallArgv: wallArgv, wallStderr: ewaPyStrip(wallErr),
		})
		if !okOne {
			badCount++
		}
	}

	say("=== 茧壁迁移桥（任务 2'.3）：Go hatch.BuildBwrapArgv ↔ zerg-wall plan ===")
	say(fmt.Sprintf("配方目录：%s（%d 份：ok %d / reject %d）", ewaRel(a.specs, root), len(names), nOK, nRej))
	say(fmt.Sprintf("茧壁二进制：%s", ewaRel(wallBin, root)))
	for _, r := range rows {
		mark := "绿"
		if !r.ok {
			mark = "红"
		}
		say(fmt.Sprintf("%-4s %-42s %s", mark, r.name, r.detail))
	}

	if a.selfTest {
		say("")
		say("--- 判据自检（先证这面镜子能红）---")
		stRows := []ewaSelfTestRow{}
		for _, r := range rows {
			stRows = append(stRows, ewaSelfTestRow{name: r.name, goArgv: r.goArgv, wallArgv: r.wallArgv})
		}
		if !ewaSelfTest(stRows, say) {
			fmt.Fprint(stdout, "!! 判据自检不过 ⇒ 拒绝把本次结果当真值（硬失败）\n")
			return ewaExitHard
		}
		fmt.Fprint(stdout, "自检：全部通过（含真实产物改坏的那三条）\n")
	}

	// ⑤ 判定与回执（回执里必须带本次判定那一行 —— 先写文件后打判定会让回执缺结论）
	fmt.Fprint(stdout, "\n")
	verdict := ewaExitOK
	if badCount > 0 {
		say(fmt.Sprintf("!! 对拍不一致 %d 份（见上）", badCount))
		verdict = ewaExitMismatch
	} else {
		say(fmt.Sprintf("对拍全绿：%d 份配方两侧 argv 逐条一致", len(rows)))
		verdict = ewaExitOK
	}

	if a.evidence != "" {
		var b strings.Builder
		b.WriteString(strings.Join(lines, "\n"))
		b.WriteString("\n")
		b.WriteString("\n--- 逐份明细（两侧 argv 原文）---\n")
		for _, r := range rows {
			mark := "绿"
			if !r.ok {
				mark = "红"
			}
			b.WriteString(fmt.Sprintf("\n[%s] %s\n", mark, r.name))
			if r.goError != "" {
				b.WriteString(fmt.Sprintf("  Go   ：拒 —— %s\n", r.goError))
			} else {
				b.WriteString(fmt.Sprintf("  Go   ：%d 项\n", len(r.goArgv)))
				for i, x := range r.goArgv {
					b.WriteString(fmt.Sprintf("    %3d %s\n", i, x))
				}
			}
			b.WriteString(fmt.Sprintf("  茧壁 ：rc=%d\n", r.wallRC))
			for i, x := range r.wallArgv {
				b.WriteString(fmt.Sprintf("    %3d %s\n", i, x))
			}
		}
		b.WriteString("\n--- 制品与源码 sha256（本机制品对拍时用于复核）---\n")
		selfRel := ewaPyJoin(ewaPySelfRelSegs[0], ewaPySelfRelSegs[1:]...)
		for _, p := range []string{
			wallBin,
			ewaPyJoin(wallDir, "src", "platform", "linux.rs"),
			ewaPyJoin(wallDir, "tests", "plan_linux.rs"),
			ewaPyJoin(agentDir, "internal", "hatch", "hatch.go"),
			ewaPyJoin(agentDir, "internal", "hatch", "wall_bridge_test.go"),
			ewaPyJoin(root, selfRel),
		} {
			if _, err := os.Stat(p); err == nil {
				b.WriteString(fmt.Sprintf("%s  %s\n", ewaSha256(p), ewaRel(p, root)))
			}
		}
		if err := os.WriteFile(a.evidence, []byte(b.String()), 0o644); err != nil {
			fmt.Fprintf(stdout, "!! 回执落不下：%s（%v）⇒ 硬失败\n", a.evidence, err)
			return ewaExitHard
		}
		fmt.Fprintf(stdout, "回执已落：%s\n", a.evidence)
	}

	return verdict
}

// ewaAbs 复刻 py os.path.abspath 的常用形态（绝对路径原样留；相对路径拼 cwd）—— 本件只在
// 传 specs 给 Go 侧时用（py 侧此处是 os.path.abspath(args.specs)）。
func ewaAbs(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	wd, err := os.Getwd()
	if err != nil {
		return p
	}
	return ewaPyJoin(wd, p)
}
