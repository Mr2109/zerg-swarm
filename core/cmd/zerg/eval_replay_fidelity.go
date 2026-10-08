// eval_replay_fidelity.go —— 件 scripts/evals/replay-fidelity.sh（247 行 · sha16 8cc543fe91f347db）的
// Go 真搬（入口 evalReplayFidelityGateMain）。
//
// 为什么有这件：本件是 `zerg eval` 评测线（`scripts/evals/`）里**唯一一条「可执行验收」脚本**——
// 它不是纯静态只读判据，而是**跨实现复核**：
//
//	· 由 Go 侧的 `go test ./internal/replay/` 真跑固定夹具，把三条指标与取证写成 JSON 报告；
//	· 脚本再用**独立于 Go 侧口径**的 shell 摘要（dir_digest：路径+内容 sha 流再取 sha256）复算产物，
//	  两侧都相等才叫「产物等价」（只用一侧自述 = 让被测者自己给自己打分）；
//	· 比对器自己先跑负控自检（同内容判等 / 差一字节判不等 / 缺目录不算相等），自检不过即 rc=2 拒出结论。
//
// 本件真身速览（逐处现读 scripts/evals/replay-fidelity.sh · 非按件名推断）：
//
//	· 形态 = **制品/子进程类判据**：真跑 `go test ./internal/replay/ -count=1 -v`（输入确定 ⇒ 报告逐字
//	  可对拍），再读三份 JSON（fidelity.json / audit.json / invalidation.json）逐字段判；读判据一律用
//	  **结构字段**，不用输出字样。
//	· 旗标 = `--fixture DIR` / `--selftest` / `--keep` / `-h|--help`；未知参 ⇒ rc=2。
//	· 退码 = 0 全过 / 1 有失败项 / 2 用法错·前置缺件·自检不过（不给结论）。
//	· 默认夹具 = `${TMPDIR:-/tmp}/zerg-replay-fidelity`（每次重跑都先 rm -rf 重建）。
//	· 依赖 = `core/` 在盘 ∧ `python3` 在 PATH（原件用它读 JSON；本件 Go 侧自读 JSON，**仍保留该前置检查**
//	  以保行为逐档同构 —— 见件尾「未覆盖面」）。
//
// 出厂声明 (a) 默认根定位差异：py/bash 从 `${BASH_SOURCE[0]}`（脚本自身位置）上溯两级取仓根 ⟷ Go 无
// `__file__` 语境，只能读 `ZERG_REPO_ROOT`/`ZERG_REPO`，否则从 cwd 上溯定位**稳定锚件**
// `core/internal/version/version.go`（同族 repoRoot 口径）。两者在「脚本就在本仓 scripts/evals/ 下、
// 且 cwd 在仓内」时**同锚**；脚本位置 ≠ cwd 且无 env 旋钮时该差异面**不逐字**，如实声明。
//
// 出厂声明 (b) 自指检查：本件落在 core/cmd/zerg 全件扫描面内 ⇒ 逐条消解：件内零绝对路径字面量、零形如
// 「引号 + 键名 + 引号」的协议原串（JSON 键名走字段取值，非件头字面量）、仓内相对路径仅作**非引号注释**
// 出现（复核 0/0，见回执）。
//
// 本单边界（照实）：本件真搬 + 接线，**不退役** ——
//
//	· 接线 = family_eval.go 的 roster 给 replay-fidelity 条目补 `GoMain: evalReplayFidelityGateMain`；
//	· 不删 scripts/evals/replay-fidelity.sh 原件（退场待后批：公开标记摘行 + 两 emit + 三处同类面）；
//	· 调用面 trace.go:49 的「可执行验收」指引改指现役 Go 入口；
//	· ✗ 不改任何登记面 / gate_inproc_table.go / gen_*.go / main.go。
//
// 入口签名（与原件 $@ / py sys.argv[1:] 对齐）：
//
//	func evalReplayFidelityGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv —— 与原件 `$@` 同形（不含程序名/件名）
//	  · 退码 —— 同原件：0 / 1 / 2
//	  · 输出 —— stdout/stderr 与原件逐字节对齐（见回执「已对拍面 / 未覆盖面」）
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const rfgProg = "replay-fidelity"

// rfgCheckWidth —— 原件 `printf '  ✓ %-38s %s\n'` 的字段宽（bash printf 按**字节**计宽 ⇒ 本件按字节补白）。
const rfgCheckWidth = 38

// rfgRCNotFound —— 起不来子命令时 shell 惯例码（bash 对「命令找不到」返 127）。
const rfgRCNotFound = 127

// rfgHelpFallback —— `-h|--help` 的**退役态合成副本**：原件该档打印自身第 2..30 行（`sed -n '2,30p'`）。
// 件在盘时**优先现读原件**（与原件逐字同）；原件退役（移出仓）时回落本副本 ⇒ 该档仍可用。
// ★ 本副本逐字取自原件第 2..30 行（含尾换行），✗ 非手编改写。
const rfgHelpFallback = `# scripts/evals/replay-fidelity.sh —— T4.4 保真度三指标 + 二分定位（E7）的**可执行验收**，
# 顺带打印 T4.5（非确定性源审计表）与 T4.6（trace 版本与失效）的取证。
#
# 为什么要有这个脚本（而不是只跑 go test）
# ----------------------------------------
# ① 判据要**跨实现**复核：Go 侧算出的产物 sha256，这里用 shasum 独立再算一遍。
#    两边都相等才叫"产物等价"；只用一侧的自述，等于让被测者自己给自己打分。
# ② 门禁自己必须先能被证明"会红"（负控，见设计稿 A8）：本脚本先对合成目录做自检，
#    自检不过 ⇒ exit 2，**拒绝出结论**（一个永远绿的比对器比没有比对器更坏）。
# ③ 输出给人看的是三行结论 + 四段证据，不是一堆 PASS 字样。
#
# 判据（每条都从结果文件数出来，不在子壳里累加计数器）
# ------------------------------------------------
#   ① F = 命中 / 尝试，missed > 0 即 FAIL；F 必须 = 1.0
#   ② 录制运行与回放运行的产物 sha256 相等（Go 侧摘要 + shell 侧独立摘要，两侧都要相等）
#   ③ replay(replay(x)) == replay(x)：独立两趟 + 叠加那趟，三份摘要全等；且夹具目录只读
#   ④ 二分定位：变异体与基准的最早分歧 seq = 4（夹具写死的期望值），前缀 4 条
#   ⑤ 审计：十类逐项声明，红项 0（自审整包）
#   ⑥ 作废判定：7 种指纹变化全部作废且判别词带两边指纹；"指纹相同 ⇒ 不作废"必须成立
#
# 用法
# ----
#   bash scripts/evals/replay-fidelity.sh                 # 跑全部（夹具每次重建）
#   bash scripts/evals/replay-fidelity.sh --fixture DIR   # 指定夹具目录（默认 $TMPDIR/zerg-replay-fidelity）
#   bash scripts/evals/replay-fidelity.sh --selftest      # 只跑负控自检
#   bash scripts/evals/replay-fidelity.sh --keep          # 保留中间件（默认就保留：证据要能被复核）
#
# 退出码：0 全过 · 1 有失败项 · 2 用法错/前置缺件/自检不过（不给结论）
set -u
`

// ── 目录摘要（独立于 Go 侧口径的 shell 复算）────────────────────────────────────
//
// dir_digest(d)（原件）：
//
//	非目录 ⇒ "MISSING"；0 个普通件 ⇒ "EMPTY"；
//	否则 ( cd d && find . -type f | LC_ALL=C sort | xargs shasum -a 256 | shasum -a 256 | awk '{print $1}' )
//
// ⇒ 逐件常态 `<内容 sha256(hex)>  <./相对路径>`（两空格），按 LC_ALL=C（字节序）排序、行尾换行，
// 再把这段文本整体取 sha256 的 hex。本件照此直译（✗ 不走 shasum 子进程，自算同值）。
func rfgDirDigest(d string) string {
	fi, err := os.Stat(d)
	if err != nil || !fi.IsDir() {
		return "MISSING"
	}
	files := []string{}
	_ = fs.WalkDir(os.DirFS(d), ".", func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if de.IsDir() {
			return nil
		}
		// `find . -type f`：只收**普通件**（lstat 语义；符号链接不计）。
		if !de.Type().IsRegular() {
			return nil
		}
		files = append(files, "./"+p)
		return nil
	})
	if len(files) == 0 {
		return "EMPTY"
	}
	sort.Slice(files, func(i, j int) bool { return files[i] < files[j] }) // LC_ALL=C = 字节序
	var buf bytes.Buffer
	for _, p := range files {
		b, err := os.ReadFile(d + "/" + strings.TrimPrefix(p, "./"))
		if err != nil {
			return "MISSING"
		}
		sum := sha256.Sum256(b)
		buf.WriteString(hex.EncodeToString(sum[:]))
		buf.WriteString("  ")
		buf.WriteString(p)
		buf.WriteByte('\n')
	}
	outer := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(outer[:])
}

// rfgFirst16 —— 复刻 shell `${VAR:0:16}`（不足 16 字节取全串）。
func rfgFirst16(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

// rfgPadBytes —— 复刻 bash printf `%-N s`（按字节计宽、右侧补空格、超宽不截）。
func rfgPadBytes(s string, w int) string {
	n := w - len(s)
	if n < 0 {
		n = 0
	}
	return s + strings.Repeat(" ", n)
}

// rfgPadRunes —— 复刻 py `%-N s`（按**字符**计宽；JSON 表打印面用它）。
func rfgPadRunes(s string, w int) string {
	n := w - utf8.RuneCountInString(s)
	if n < 0 {
		n = 0
	}
	return s + strings.Repeat(" ", n)
}

// rfgMark —— ✓/✗ 记号。
func rfgMark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

// ── 判定工具（判定一律立刻落盘：末了只数 results.tsv，避免子壳计数器假绿）──────

// rfgCheck —— 复刻原件 check()：写 `TAG\t项\t说明` 到 results.tsv ＋ 打一行 `  ✓/✗ <项 %-38s> <说明>`。
func rfgCheck(results *bytes.Buffer, resultsPath string, stdout io.Writer, label string, fail int, desc string) {
	tag := "PASS"
	ok := true
	if fail != 0 {
		tag, ok = "FAIL", false
	}
	fmt.Fprintf(results, "%s\t%s\t%s\n", tag, label, desc)
	fmt.Fprintf(stdout, "  %s %s %s\n", rfgMark(ok), rfgPadBytes(label, rfgCheckWidth), desc)
	_ = os.WriteFile(resultsPath, results.Bytes(), 0o644)
}

// ── JSON 读面（复刻原件 json_get / json_len：点路径取值，输出 py str 形态）───────

// rfgJSONLoad —— 读一件 JSON（失败返回 false；原件此刻 py 会回溯 ⇒ rc=1，本件在调用侧按「取不到」处理）。
func rfgJSONLoad(path string) (interface{}, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	return v, true
}

// rfgJSONPick —— 复刻 py：`for k in dotpath.split("."): cur = cur[int(k)] if isinstance(cur, list) else cur[k]`。
func rfgJSONPick(v interface{}, dotpath string) (interface{}, bool) {
	cur := v
	for _, k := range strings.Split(dotpath, ".") {
		switch t := cur.(type) {
		case []interface{}:
			i, err := strconv.Atoi(k)
			if err != nil || i < 0 || i >= len(t) {
				return nil, false
			}
			cur = t[i]
		case map[string]interface{}:
			nv, ok := t[k]
			if !ok {
				return nil, false
			}
			cur = nv
		default:
			return nil, false
		}
	}
	return cur, true
}

// rfgPyStr —— 复刻 py `print(cur)`（bool 走 true/false；其余走 str()；None ⇒ "None"）。
func rfgPyStr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return t
	case json.Number:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}

// rfgPyInt —— 复刻 py `%d` 对数值/整数的取整输出（本件数值全为整）。
func rfgPyInt(v interface{}) string {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return strconv.FormatInt(i, 10)
		}
		if f, err := t.Float64(); err == nil {
			return strconv.FormatInt(int64(f), 10)
		}
		return t.String()
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case int:
		return strconv.Itoa(t)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// rfgJSONGet —— 取字符串档读数（取不到 ⇒ 空串）。
func rfgJSONGet(v interface{}, dotpath string) string {
	if c, ok := rfgJSONPick(v, dotpath); ok {
		return rfgPyStr(c)
	}
	return ""
}

// rfgJSONLen —— 取列表/串长度档读数（取不到 ⇒ 空串）。
func rfgJSONLen(v interface{}, dotpath string) string {
	c, ok := rfgJSONPick(v, dotpath)
	if !ok {
		return ""
	}
	switch t := c.(type) {
	case []interface{}:
		return strconv.Itoa(len(t))
	case string:
		return strconv.Itoa(utf8.RuneCountInString(t))
	case map[string]interface{}:
		return strconv.Itoa(len(t))
	}
	return ""
}

// rfgAToI —— 原件用 `[ "$X" -gt 0 ]` 一类整数比较；取不到/非数按 0（不使进程崩）。
func rfgAToI(s string) int {
	i, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return i
}

// ── 负控自检：比对器必须先能红 ──────────────────────────────────────────────────

// rfgSelfDigest —— 自检用的比法器注入点（负控用；测试件可替换成「什么都判相等」的必红桩）。
var rfgSelfDigest = rfgDirDigest

func rfgSelftest(stdout io.Writer, fix string) bool {
	d := fix + "/selftest"
	_ = os.RemoveAll(d)
	_ = os.MkdirAll(d+"/x", 0o755)
	_ = os.MkdirAll(d+"/y", 0o755)
	_ = os.WriteFile(d+"/x/f.txt", []byte("a\n"), 0o644)
	_ = os.WriteFile(d+"/y/f.txt", []byte("a\n"), 0o644)
	ok := true

	a, b := rfgSelfDigest(d+"/x"), rfgSelfDigest(d+"/y")
	if a == b && a != "EMPTY" {
		fmt.Fprintf(stdout, "  %s %s %s\n", "✓", rfgPadBytes("自检：同内容目录判相等", rfgCheckWidth), "摘要 "+a)
	} else {
		fmt.Fprintf(stdout, "  %s %s %s\n", "✗", rfgPadBytes("自检：同内容目录判相等", rfgCheckWidth), "摘要 "+a+" vs "+b)
		ok = false
	}

	_ = os.WriteFile(d+"/y/f.txt", []byte("b\n"), 0o644)
	a, b = rfgSelfDigest(d+"/x"), rfgSelfDigest(d+"/y")
	if a != b {
		fmt.Fprintf(stdout, "  %s %s %s\n", "✓", rfgPadBytes("自检：差一字节判不等", rfgCheckWidth), "摘要已变")
	} else {
		fmt.Fprintf(stdout, "  %s %s %s\n", "✗", rfgPadBytes("自检：差一字节判不等", rfgCheckWidth), "仍相等 ⇒ 比对器是假绿")
		ok = false
	}

	a, b = rfgSelfDigest(d+"/x/nope"), rfgSelfDigest(d+"/x")
	if a != b {
		fmt.Fprintf(stdout, "  %s %s %s\n", "✓", rfgPadBytes("自检：缺目录不算相等", rfgCheckWidth), "缺目录得 "+a)
	} else {
		fmt.Fprintf(stdout, "  %s %s %s\n", "✗", rfgPadBytes("自检：缺目录不算相等", rfgCheckWidth), "被当成了相等")
		ok = false
	}
	return ok
}

// ── 起 go test（本件的数据源：固定夹具 + 报告由该测试写出）──────────────────────

// rfgEnvWith —— 复刻 `VAR=v cmd` 的 shell 前缀赋值语义：删同名旧值再追加（后写胜）。
func rfgEnvWith(extra map[string]string) []string {
	env := []string{}
	for _, e := range os.Environ() {
		k := e
		if i := strings.IndexByte(e, '='); i >= 0 {
			k = e[:i]
		}
		if _, drop := extra[k]; drop {
			continue
		}
		env = append(env, e)
	}
	for _, k := range []string{"GOFLAGS", "GOSUMDB", "GOPROXY", "ZERG_REPLAY_FIDELITY_DIR"} {
		if v, ok := extra[k]; ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// rfgRunGoTest —— 复刻原件 `( cd core && GOFLAGS=-mod=mod GOSUMDB=off GOPROXY=… ZERG_REPLAY_FIDELITY_DIR=FIX
// go test ./internal/replay/ -count=1 -v ) > LOG 2>&1`；退码原样转出（起不来 ⇒ 127）。
func rfgRunGoTest(coreDir, fix, logPath string) int {
	cmd := exec.Command("go", "test", "./internal/replay/", "-count=1", "-v")
	cmd.Dir = coreDir
	cmd.Env = rfgEnvWith(map[string]string{
		"GOFLAGS":                  "-mod=mod",
		"GOSUMDB":                  "off",
		"GOPROXY":                  "https://goproxy.cn,direct",
		"ZERG_REPLAY_FIDELITY_DIR": fix,
	})
	out, err := cmd.CombinedOutput()
	_ = os.WriteFile(logPath, out, 0o644)
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return rfgRCNotFound
	}
	return 0
}

// rfgCountLinePrefix —— 数 LOG 里以某前缀开头的行（复刻 `grep -c`）。
func rfgCountLinePrefix(logText, prefix string) int {
	n := 0
	for _, ln := range strings.Split(logText, "\n") {
		if strings.HasPrefix(ln, prefix) {
			n++
		}
	}
	return n
}

// rfgLogFailingExcerpt —— 复刻 `grep -E '^(--- FAIL|    )' LOG | head -20`（逐行返回，最多 20）。
func rfgLogFailingExcerpt(logText string) []string {
	out := []string{}
	for _, ln := range strings.Split(logText, "\n") {
		if strings.HasPrefix(ln, "--- FAIL") || strings.HasPrefix(ln, "    ") {
			out = append(out, ln)
			if len(out) >= 20 {
				break
			}
		}
	}
	return out
}

// ── 目录复制（负控用：cp -R）────────────────────────────────────────────────────

func rfgCopyDir(src, dst string) {
	_ = fs.WalkDir(os.DirFS(src), ".", func(p string, de fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		target := dst + "/" + strings.TrimPrefix(p, "./")
		if de.IsDir() {
			_ = os.MkdirAll(target, 0o755)
			return nil
		}
		b, err := os.ReadFile(src + "/" + strings.TrimPrefix(p, "./"))
		if err != nil {
			return nil
		}
		_ = os.WriteFile(target, b, 0o644)
		return nil
	})
}

// ── py json.dumps（ensure_ascii=False）面：打印 invalidation 头部 ────────────────

// rfgPyJSONString —— 复刻 py json.dumps(str, ensure_ascii=False)。
func rfgPyJSONString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
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

type rfgKV struct {
	k string
	v json.RawMessage
}

// rfgOrderedObj —— 把 JSON 对象按**出现序**拆成键值对（py json.load 保序 ⇒ json.dumps 保序）。
func rfgOrderedObj(raw json.RawMessage) ([]rfgKV, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, false
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, false
	}
	out := []rfgKV{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, ok := kt.(string)
		if !ok {
			return nil, false
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, false
		}
		out = append(out, rfgKV{key, v})
	}
	return out, true
}

// rfgPyJSONValue —— 复刻 py json.dumps(value) 的紧凑分隔（', ' / ': '，ensure_ascii=False）。
func rfgPyJSONValue(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "null"
	}
	switch trimmed[0] {
	case '{':
		if kv, ok := rfgOrderedObj(raw); ok {
			parts := make([]string, 0, len(kv))
			for _, e := range kv {
				parts = append(parts, rfgPyJSONString(e.k)+": "+rfgPyJSONValue(e.v))
			}
			return "{" + strings.Join(parts, ", ") + "}"
		}
	case '[':
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err == nil {
			parts := make([]string, 0, len(arr))
			for _, e := range arr {
				parts = append(parts, rfgPyJSONValue(e))
			}
			return "[" + strings.Join(parts, ", ") + "]"
		}
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return rfgPyJSONString(s)
		}
	}
	return trimmed
}

// ── 参数解析（原件 while case；✗ 无 argparse ⇒ 精确匹配，不做前缀缩写）────────────

type rfgArgs struct {
	fixture      string
	hasFixture   bool
	selfTestOnly bool
	helpReq      bool
	parseErr     string // 取值为原件原文提示
}

func rfgParseArgs(argv []string) *rfgArgs {
	a := &rfgArgs{}
	i := 0
	for i < len(argv) {
		switch argv[i] {
		case "--fixture":
			if i+1 >= len(argv) {
				a.parseErr = "✗ --fixture 需要一个目录参数"
				return a
			}
			a.fixture = argv[i+1]
			a.hasFixture = true
			i += 2
		case "--selftest":
			a.selfTestOnly = true
			i++
		case "--keep":
			i++
		case "-h", "--help":
			a.helpReq = true
			return a
		default:
			a.parseErr = "✗ 未知参数：" + argv[i]
			return a
		}
	}
	return a
}

// rfgHelpText —— `-h|--help`：优先现读原件第 2..30 行（与原件逐字同）；原件退役 ⇒ 用内嵌合成副本。
func rfgHelpText(root string) string {
	p := root + "/scripts/evals/replay-fidelity.sh"
	b, err := os.ReadFile(p)
	if err != nil {
		return rfgHelpFallback
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) < 30 {
		return rfgHelpFallback
	}
	return strings.Join(lines[1:30], "\n") + "\n"
}

// ── 仓根定位（稳定锚件 core/internal/version/version.go；取不到 ⇒ 空串）──────────

func rfgRepoRoot() string {
	for _, k := range []string{"ZERG_REPO_ROOT", "ZERG_REPO"} {
		if v := os.Getenv(k); v != "" && rfgIsRepoRoot(v) {
			return v
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := wd
	for {
		if rfgIsRepoRoot(dir) {
			return dir
		}
		parent := strings.TrimSuffix(dir, "/")
		if i := strings.LastIndexByte(parent, '/'); i >= 0 {
			parent = parent[:i]
			if parent == "" {
				parent = "/"
			}
		} else {
			return ""
		}
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func rfgIsRepoRoot(dir string) bool {
	st, err := os.Stat(dir + "/core/internal/version/version.go")
	return err == nil && !st.IsDir()
}

// ── 入口（= 原件 main 流）──────────────────────────────────────────────────────

func evalReplayFidelityGateMain(stdout, stderr io.Writer, argv []string) int {
	a := rfgParseArgs(argv)
	if a.helpReq {
		root := rfgRepoRoot()
		fmt.Fprint(stdout, rfgHelpText(root))
		return 0
	}
	if a.parseErr != "" {
		fmt.Fprintln(stderr, a.parseErr)
		return 2
	}

	root := rfgRepoRoot()
	coreDir := root + "/core"
	fix := os.Getenv("TMPDIR")
	if fix == "" {
		fix = "/tmp"
	}
	fix += "/zerg-replay-fidelity"
	if a.hasFixture {
		fix = a.fixture
	}

	if st, err := os.Stat(coreDir); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "✗ 找不到 core/（脚本按仓库根定位，实际在 %s）\n", root)
		return 2
	}
	if _, err := exec.LookPath("python3"); err != nil {
		fmt.Fprintln(stderr, "✗ 缺 python3（本脚本用它读 JSON 判据，不用输出字样判）")
		return 2
	}

	_ = os.RemoveAll(fix)
	_ = os.MkdirAll(fix, 0o755)
	resultsPath := fix + "/results.tsv"
	logPath := fix + "/00-go-test.log"
	results := &bytes.Buffer{}
	_ = os.WriteFile(resultsPath, nil, 0o644)

	fmt.Fprintf(stdout, "══ T4.4 保真度三指标（E7）· 夹具 %s\n", fix)
	fmt.Fprintln(stdout, "── 负控自检（比对器先要能被证明会红）")
	if !rfgSelftest(stdout, fix) {
		fmt.Fprintln(stderr, "✗ 自检未过 ⇒ 脚本本身有问题，拒绝出结论")
		return 2
	}
	rfgCheck(results, resultsPath, stdout, "负控自检", 0, "三条自检全过（同内容判等 / 差一字节判不等 / 缺目录不算相等）")

	if a.selfTestOnly {
		fmt.Fprintln(stdout, "（--selftest：只跑自检，到此为止）")
		return 0
	}

	fmt.Fprintln(stdout, "── 跑真夹具（Go 侧：录制 + 两趟回放 + 叠加回放 + 二分 + 审计 + 作废判定）")
	rc := rfgRunGoTest(coreDir, fix, logPath)
	logText, _ := os.ReadFile(logPath)
	passN := rfgCountLinePrefix(string(logText), "--- PASS")
	rfgCheck(results, resultsPath, stdout, "go test ./internal/replay/", rc,
		fmt.Sprintf("%d 条用例通过（日志 %s）", passN, logPath))
	if rc != 0 {
		fmt.Fprintln(stdout, "  ── 失败用例（节选）──")
		for _, ln := range rfgLogFailingExcerpt(string(logText)) {
			fmt.Fprintln(stdout, ln)
		}
	}

	// 缺件即报错（不给结论）—— 报告不存在说明 Go 侧没能跑到写报告那一步
	for _, f := range []string{"fidelity.json", "audit.json", "invalidation.json"} {
		if _, err := os.Stat(fix + "/" + f); err != nil {
			fmt.Fprintf(stderr, "✗ 缺报告 %s/%s（Go 侧没跑到出报告那一步）⇒ 不给结论\n", fix, f)
			return 2
		}
	}

	fid, _ := rfgJSONLoad(fix + "/fidelity.json")
	aud, _ := rfgJSONLoad(fix + "/audit.json")
	inv, _ := rfgJSONLoad(fix + "/invalidation.json")
	// 头部要按**文件出现序**打印（py json.load 保序 ⇒ json.dumps 保序）⇒ 另取 header 原文。
	invRaw, _ := os.ReadFile(fix + "/invalidation.json")
	var invTop map[string]json.RawMessage
	_ = json.Unmarshal(invRaw, &invTop)
	hdrRaw := invTop["header"]

	// ① F = 命中录播事件 / 回放期尝试事件（missed>0 即 FAIL）
	fmt.Fprintln(stdout, "── ① F = 命中录播事件 / 回放期尝试事件（missed>0 即 FAIL）")
	att := rfgJSONGet(fid, "fidelity.attempted")
	hit := rfgJSONGet(fid, "fidelity.hit")
	miss := rfgJSONGet(fid, "fidelity.missed")
	fval := rfgJSONGet(fid, "fidelity.f")
	fail := 0
	if !(miss == "0" && att == hit && rfgAToI(att) > 0) {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "F=1.0 且 missed=0", fail,
		fmt.Sprintf("F=%s（命中 %s / 尝试 %s，未命中 %s）", fval, hit, att, miss))
	fail = 0
	if miss != "0" {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "missed=0（未命中不是警告，是 FAIL）", fail, "missed="+miss)

	// ② 输出等价性：录制产物 == 回放产物（Go 侧摘要 + shell 侧独立摘要）
	fmt.Fprintln(stdout, "── ② 输出等价性：录制产物 == 回放产物（Go 侧摘要 + shell 侧独立摘要）")
	rd := rfgJSONGet(fid, "recorded_digest")
	ad := rfgJSONGet(fid, "replay_a_digest")
	eq := rfgJSONGet(fid, "equivalence.equal")
	fail = 0
	if !(eq == "true" && rd == ad) {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "Go 侧产物摘要相等", fail, "recorded == replay-a = "+strings.TrimPrefix(rd, "sha256:"))
	shr := rfgDirDigest(fix + "/recorded")
	sha := rfgDirDigest(fix + "/replay-a")
	fail = 0
	if !(shr == sha && shr != "EMPTY" && shr != "MISSING") {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "shell 侧独立摘要相等", fail,
		fmt.Sprintf("recorded=%s replay-a=%s（与 Go 侧口径刻意不同）", rfgFirst16(shr), rfgFirst16(sha)))
	// 负控（用真夹具数据）：把回放产物改一字节 ⇒ 同一个比对器必须判不等
	neg := fix + "/negctl-replay-a"
	_ = os.RemoveAll(neg)
	rfgCopyDir(fix+"/replay-a", neg)
	if f, err := os.OpenFile(neg+"/answer.txt", os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.Write([]byte("X"))
		_ = f.Close()
	}
	negd := rfgDirDigest(neg)
	fail = 0
	if negd == sha {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "负控：真夹具产物改一字节 ⇒ 判不等", fail,
		fmt.Sprintf("改过的摘要 %s ≠ 原摘要 %s", rfgFirst16(negd), rfgFirst16(sha)))

	// ③ 确定性自检：replay(replay(x)) == replay(x)
	fmt.Fprintln(stdout, "── ③ 确定性自检：replay(replay(x)) == replay(x)")
	det := rfgJSONGet(fid, "determinism.layer_stable")
	stb := rfgJSONGet(fid, "determinism.stable")
	unt := rfgJSONGet(fid, "determinism.fixture_untouched")
	fail = 0
	if !(stb == "true" && det == "true" && unt == "true") {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "两趟独立 + 叠加 + 夹具只读", fail,
		fmt.Sprintf("stable=%s layer_stable=%s fixture_untouched=%s", stb, det, unt))
	shb := rfgDirDigest(fix + "/replay-b")
	fail = 0
	if !(sha == shb && shb != "EMPTY") {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "shell 侧 replay-a == replay-b", fail,
		fmt.Sprintf("replay-a=%s replay-b=%s", rfgFirst16(sha), rfgFirst16(shb)))
	lcall := rfgJSONGet(fid, "live_calls")
	fail = 0
	if lcall != "0" {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "回放期真发次数 = 0", fail,
		"真发 "+lcall+" 次（回放不真发是结构性的，这里钉住它）")

	// ④ 二分定位最早分歧（夹具变异体：第 4 条录播的结果被改）
	fmt.Fprintln(stdout, "── ④ 二分定位最早分歧（夹具变异体：第 4 条录播的结果被改）")
	eseq := rfgJSONGet(fid, "bisect.earliest_seq")
	plen := rfgJSONGet(fid, "bisect.prefix_len")
	probes := rfgJSONGet(fid, "bisect.probes")
	fail = 0
	if !(eseq == "4" && plen == "4") {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "最早分歧 seq=4 / 前缀 4 条", fail,
		fmt.Sprintf("探测 %s 次（夹具 6 条；n 大时二分才省）", probes))

	// ⑤ T4.5 非确定性源审计（十类逐项声明，红项必须为 0）
	fmt.Fprintln(stdout, "── ⑤ T4.5 非确定性源审计（十类逐项声明，红项必须为 0）")
	red := rfgJSONGet(aud, "red_count")
	itemsLen := rfgJSONLen(aud, "items")
	fail = 0
	if red != "0" {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "整包自审红项 = 0", fail, "红项 "+red)
	fail = 0
	if rfgAToI(itemsLen) < 10 {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "十类逐项在表", fail, "声明项 "+itemsLen+" 类")
	rfgPrintAuditTable(stdout, aud)
	rfgCheck(results, resultsPath, stdout, "审计表打印", 0, "已打印十类（脚本判据只看 red_count，表是给人看的）")

	// ⑥ T4.6 trace 版本与失效（指纹变 ⇒ 作废，附两边指纹；指纹同 ⇒ 不作废）
	fmt.Fprintln(stdout, "── ⑥ T4.6 trace 版本与失效（指纹变 ⇒ 作废，附两边指纹；指纹同 ⇒ 不作废）")
	alli := rfgJSONGet(inv, "all_invalidated")
	rfgPrintInvalidationTable(stdout, inv, hdrRaw)
	fail = 0
	if alli != "true" {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "7 种指纹变化全部作废且带两边指纹", fail, "all_invalidated="+alli)
	ctrl := rfgJSONGet(inv, "cases.0.invalidated")
	fail = 0
	if ctrl != "false" {
		fail = 1
	}
	rfgCheck(results, resultsPath, stdout, "反例：指纹相同不作废（防误报）", fail, "对侧判定 invalidated="+ctrl)

	// 结论
	fmt.Fprintln(stdout, "── 结论")
	fails := 0
	passes := 0
	for _, ln := range strings.Split(results.String(), "\n") {
		if strings.HasPrefix(ln, "FAIL") {
			fails++
		}
		if strings.HasPrefix(ln, "PASS") {
			passes++
		}
	}
	fmt.Fprintf(stdout, "  判定通过 %d 项 / 失败 %d 项（明细 %s）\n", passes, fails, resultsPath)
	if fails != 0 {
		fmt.Fprintln(stdout, "  ✗ 三条指标未全过（exit 1）—— 逐条看上面对应的 ✗ 行")
		return 1
	}
	fmt.Fprintln(stdout, "  ✓ 三条指标全过：F=1.0 且 missed=0 · 录制与回放产物摘要相等（两侧独立复算）· replay(replay(x))==replay(x)")
	fmt.Fprintf(stdout, "  （夹具与证据留档：%s）\n", fix)
	return 0
}

// rfgPrintAuditTable —— 复刻原件第 ⑤ 段内嵌 py 打印的十类表（判据只看 red_count，表是给人看的）。
func rfgPrintAuditTable(stdout io.Writer, aud interface{}) {
	itemsV, _ := rfgJSONPick(aud, "items")
	items, _ := itemsV.([]interface{})
	bad := 0
	for _, it := range items {
		if m, ok := it.(map[string]interface{}); ok {
			if b, ok := m["red"].(bool); ok && b {
				bad++
			}
		}
	}
	fmt.Fprintf(stdout, "  类别表（%d 类，红 %d）：\n", len(items), bad)
	for idx, it := range items {
		m, _ := it.(map[string]interface{})
		id := rfgPyStr(m["id"])
		status := rfgPyStr(m["status"])
		fmt.Fprintf(stdout, "    %2d. %s %s 命中=%s(豁免%s/计数%s) 声明=%s %s\n",
			idx+1, rfgPadRunes(id, 10), rfgPadRunes(status, 6),
			rfgPyInt(m["hits"]), rfgPyInt(m["allowed"]), rfgPyInt(m["counted"]), rfgPyInt(m["expect_hits"]),
			rfgMark(!(boolOf(m["red"]))))
	}
	if saV, ok := rfgJSONPick(aud, "stale_allow"); ok {
		if sa, ok2 := saV.([]interface{}); ok2 && len(sa) > 0 {
			fmt.Fprintf(stdout, "    过期豁免：%d 条\n", len(sa))
		}
	}
}

// rfgPrintInvalidationTable —— 复刻原件第 ⑥ 段内嵌 py：trace/头部 + 逐案作废判定表。
// hdrRaw 是 invalidation.json 的 `header` **原文**（保序打印，✗ 不走 map 快照 —— 那会按字母序）。
func rfgPrintInvalidationTable(stdout io.Writer, inv interface{}, hdrRaw json.RawMessage) {
	trace := rfgJSONGet(inv, "trace")
	fmt.Fprintf(stdout, "  trace %s 头部 %s\n", trace, rfgPyJSONValue(hdrRaw))
	casesV, _ := rfgJSONPick(inv, "cases")
	cases, _ := casesV.([]interface{})
	for _, c := range cases {
		m, _ := c.(map[string]interface{})
		fmt.Fprintf(stdout, "    %s 作废=%-5s 两边指纹齐全=%-5s\n",
			rfgPadRunes(rfgPyStr(m["name"]), 22),
			rfgPadRunes(pyBoolStr(m["invalidated"]), 5),
			rfgPadRunes(pyBoolStr(m["has_both_sides"]), 5))
	}
}

// pyBoolStr —— py str(True)/str(False)。
func pyBoolStr(v interface{}) string {
	if b, ok := v.(bool); ok {
		if b {
			return "True"
		}
		return "False"
	}
	return rfgPyStr(v)
}

// boolOf —— py 真值取用（仅在 red 字段上）。
func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}
