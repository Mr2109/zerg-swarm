// check_affected_reprove.go —— 判据件 `scripts/gates/check-affected-reprove.py`（786 行）的 **Go 真搬**
// （v2.5.14 · M2 一枚 · 入口 `checkAffectedReproveGateMain` · 承接 GAP-20261001-62）。
//
// 本件 = 「受影响面声明表是否漏声明」的**反证体检**（主法丙·成对扰动式）之 Go 真搬：
//
//	C0 成对自证（扰动后 sha ≠ 前 · 复原后 sha == 前）· C1 判据先绿前置 · C2 成对判读
//	C3 无步引用 ⇒ 不给结论 · C4 空转可分辨 · C5 纯只读（备份落仓外 + 受控回滚）· C6 手法按步档可选。
//
// 入口签名（与 py `main()` 对齐）：
//
//	func checkAffectedReproveGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 全给结论且至少一「真依赖」· 1 全落「真无依赖」· 2 不给结论 · 3 复原失败
//
// ★★ 两条标准声明行：
//
//	(a) **默认根定位差异**：py `DEFAULT_ROOT = dirname(dirname(HERE))`（脚本位置上溯 = 仓根）；
//	    本件**无 `__file__` 语境** ⇒ 不给 `--root` 时从 **cwd** 上溯找含 `scripts/gates` 的目录、
//	    找不到回落 cwd（`carvDefaultRoot`）。落在同一真仓、cwd=仓根时两者读数逐字相同（已对拍甲/乙）；
//	    「脚本位置 ≠ cwd 上溯根」时路径推导面不同 ⇒ **不属逐字对齐承诺**。
//	(b) **自指检查**：① 本件**在自己（py 原件）的扫描面内吗？** py `candidates()` 只取步表命令位按名点到的
//	    `scripts/gates/` 件并**显式剔除自身基名** ⇒ 本件自身（`core/cmd/zerg/check_affected_reprove.go`）
//	    不在该扫描面内；② 本件**落在他门扫描面内吗？真**（`check-hardcoded-private-paths` 的 SCAN_DIRS 含
//	    `core/`）⇒ 件头与正文**零绝对私有路径字面量**（盘根/家目录前缀一概不写；`/usr/bin/python3` 是系统
//	    解释器路径、非私有路径）；③ 处理有效性 = 落仓前后同一他门取数逐字相同（读数见交付回执）。
//
// ★ 手法档三形态（insert / semantic / auto）与 py 逐字：默认档（不给 `--mutation`）**一字不改**。
//
// 未覆盖面（✗ 不声称逐字对齐 · 逐条）：
//
//	① **argparse 面**：py 支持长选项**前缀缩写**（`--lis`⇒`--list`）与 `--opt=值`；本件实现**精确长旗标 +**
//	   `=` 取值 + 缺值报话，缩写面不覆盖。`-h`/`--help` 与各类 usage/error 正文**逐字未复刻**（只保退码 2）。
//	② **正则语义差**：py 的 `\s`/`\S`/`\w`/`\d` 是 Unicode 语义，Go RE2 默认 ASCII ⇒ 文本夹 `U+00A0`
//	   一类空白/数字时两态不同（本件已在 `carvIsPySpace` 里按 py 语义补齐 `\x1c-\x1f`，但正则引擎自身的
//	   `\s`/`\w` 仍为 ASCII）；`_CONST_RE_PY` 的 `\d` 在 Go 侧写作 `[0-9]`（py 认 Unicode 数字）⇒ 该档差异
//	   只声明、未对拍。py 非多行 `$` 匹配「串尾换行之前」，Go RE2 `$` 只匹配串尾 —— 本件逐处按**按行匹配**
//	   规避（`strings.Split("\n")` 后无尾换行）。
//	③ **`re` 的负向先行**：`_break_output_py` 的 `(?<![\w.])print\s*\(` RE2 不支持 ⇒ 本件**手写**左起第一个
//	   满足「前一码点非 `\w`/`.` 且后随 `\s*\(`」的 `print`（语义逐字对齐，未逐形态对拍）。
//	④ **异常路径**：py `__main__` 是裸 `sys.exit(main())` ⇒ main 内未捕异常 = 回溯 + rc=1（C 类）；本件对
//	   「读不到 / 子进程起不来」一类落 rc=2 / 127 fail-closed，**正文与 py 的 traceback 不同**。
//	⑤ **`errors='replace'` 粒度**：py 按非法**序列**替换一个 U+FFFD，Go `strings.ToValidUTF8` 按非法**字节**
//	   各替换一个 ⇒ 长度不同（真仓多为合法 UTF-8 ⇒ 通常不触发）。
//	⑥ **子进程被杀**：py `subprocess` 的 `returncode` 为负（信号），Go `ExitCode()` 为 -1 ⇒ 该极端档未对拍。
//	⑦ **自检手段差异**：py `--self-test` 用 `_run_fixture`（**子进程**跑仓外 py 副本）+ `_inproc`（importlib
//	   装载副本模块 + 换 `sha`/`shutil` 全局）跑 14 例；本件**无 `__file__`/模块重载** ⇒ 改为**同进程**调用入口
//	   （chdir 到夹具使 `carvDefaultRoot` == py 的 `__file__` 根）+ 换 `carvSHA`/`carvCopy2` 函数变量。**打印的
//	   14 例正文逐字对齐**（只含 rc/布尔/固定串，无路径），但「驱动件」在 ①b 里由 py 的「py 副本 sha」改为本件的
//	   「夹具步表 sha」（本件无自身副本可放）。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
)

const (
	carvGatesDirRel  = "scripts/gates"
	carvPrecommitRel = zergPrecommitGatesRel
	carvSelfRel      = "scripts/gates/check-affected-reprove.py"
	carvSelfBase     = "check-affected-reprove.py"
	carvFixDriverRel = zergPrecommitGatesRel
)

// carvExit* —— 四档退码（与 py `EXIT_OK/EXIT_UNRESOLVED/EXIT_HARD/EXIT_RESTORE` 逐字）
// ＋ 子进程面两档（超时 / 起不来）。
const (
	carvExitOK         = 0
	carvExitUnresolved = 1
	carvExitHard       = 2
	carvExitRestore    = 3
	carvExitTimeout    = 124
	carvExitSpawnFail  = 127
)

// 扰动手法档。
const (
	carvMutInsert   = "insert"
	carvMutSemantic = "semantic"
	carvMutAuto     = "auto"
)

const carvMutPy = "import sys as _reprove_sys; _reprove_sys.exit(97)  # 反证扰动（体检用 · 必复原）\n"
const carvMutSh = "exit 97  # 反证扰动（体检用 · 必复原）\n"

func carvIsReportOnly(mode string) bool { return mode == "tri-report" }

// ── 正门正则（与 py 逐条对读；语义差见件头未覆盖面②）─────────────────────────────
var (
	carvAddStepRe    = regexp.MustCompile(`add_step\s+(\S+)\s+"([^"]*)"\s+(\S+)\s+"([^"]*)"\s+"([^"]*)"`)
	carvGateScriptRe = regexp.MustCompile(`scripts/gates/([A-Za-z0-9_.\-]+)`)
	carvDefRePy      = regexp.MustCompile(`^def\s+[A-Za-z_]\w*\s*\(.*\)\s*(->[^:]*)?:\s*$`)
	carvDefReSh      = regexp.MustCompile(`^[A-Za-z_]\w*\s*\(\s*\)\s*\{`)
	carvConstRePy    = regexp.MustCompile(`^([A-Z][A-Z0-9_]{2,})\s*=\s*(.+?)\s*$`)
	carvIntRe        = regexp.MustCompile(`^-?[0-9]+$`)
)

// ── py 语义小工具（✗ 不用 filepath.* / strings.TrimSpace 直代）──────────────────────

// carvPyJoin —— 照 CPython posixpath.join 直译（`..` 段原样留 · 后段绝对路径丢弃前段 · 空段照串）。
func carvPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
		if strings.HasPrefix(b, "/") {
			path = b
		} else if path == "" || strings.HasSuffix(path, "/") {
			path += b
		} else {
			path += "/" + b
		}
	}
	return path
}

// carvPyDirname —— posixpath.split 语义（剥尾斜杠）。
func carvPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && strings.TrimRight(head, "/") == "" {
		head = head[:1]
	}
	return head
}

// carvPyAbspath —— 纯词法拼接、✗ 不归一（`..`/`//` 原样留）。
func carvPyAbspath(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	cwd, err := os.Getwd()
	if err != nil {
		return p
	}
	return carvPyJoin(cwd, p)
}

// carvIsPySpace —— py `\s`（比 unicode.IsSpace 多 `\x1c-\x1f`）。
func carvIsPySpace(r rune) bool {
	if r >= '\x1c' && r <= '\x1f' {
		return true
	}
	return unicode.IsSpace(r)
}

func carvPyStrip(s string) string {
	r := []rune(s)
	i, j := 0, len(r)
	for i < j && carvIsPySpace(r[i]) {
		i++
	}
	for j > i && carvIsPySpace(r[j-1]) {
		j--
	}
	return string(r[i:j])
}

func carvPyRStrip(s string) string {
	r := []rune(s)
	j := len(r)
	for j > 0 && carvIsPySpace(r[j-1]) {
		j--
	}
	return string(r[:j])
}

func carvStartsWithPySpace(s string) bool {
	for _, r := range s {
		return carvIsPySpace(r)
	}
	return false
}

func carvIsWordChar(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) }

// carvDecodeReplace —— py `errors='replace'` 的近似（粒度差见件头未覆盖面⑤）+ universal newlines。
func carvDecodeReplace(b []byte) string {
	s := strings.ToValidUTF8(string(b), "\uFFFD")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func carvRead(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return carvDecodeReplace(b), nil
}

func carvFileSHA(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

// 可注入旋钮（自检用；照 py `_inproc` 换 `sha`/`shutil.copy2` 的口径）。
var carvSHA = carvFileSHA
var carvCopy2 = carvCopy2Impl

func carvCopy2Impl(src, dst string) error {
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, b, fi.Mode().Perm()); err != nil {
		return err
	}
	_ = os.Chtimes(dst, time.Now(), fi.ModTime())
	return nil
}

func carvBoolPy(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func carvHead12(s string) string {
	r := []rune(s)
	if len(r) > 12 {
		return string(r[:12])
	}
	return s
}

func carvLastRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[len(r)-n:]
	}
	return string(r)
}

func carvFirstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

func carvTailEllipsis(tail string) string {
	return carvFirstRunes(strings.ReplaceAll(tail, "\n", " ⏎ "), 300)
}

// ── 语义词族（rc=1）三形态 + 兜底 ─────────────────────────────────────────────────

func carvGutBodyPy(src string) (string, bool) {
	lines := strings.Split(src, "\n")
	idx := -1
	for i, ln := range lines {
		if carvDefRePy.MatchString(ln) {
			idx = i
		}
	}
	if idx < 0 {
		return "", false
	}
	j := idx + 1
	hasBody := false
	for j < len(lines) {
		ln := lines[j]
		if carvPyStrip(ln) == "" || carvStartsWithPySpace(ln) {
			if carvPyStrip(ln) != "" {
				hasBody = true
			}
			j++
			continue
		}
		break
	}
	if !hasBody {
		return "", false
	}
	out := append([]string{}, lines[:idx+1]...)
	out = append(out, "    raise SystemExit(1)")
	out = append(out, lines[j:]...)
	return strings.Join(out, "\n"), true
}

func carvGutBodySh(src string) (string, bool) {
	lines := strings.Split(src, "\n")
	idx := -1
	for i, ln := range lines {
		if carvDefReSh.MatchString(ln) {
			idx = i
		}
	}
	if idx < 0 {
		return "", false
	}
	out := append([]string{}, lines[:idx+1]...)
	out = append(out, "  return 1")
	out = append(out, lines[idx+1:]...)
	return strings.Join(out, "\n"), true
}

func carvFlipConstantPy(src string) (string, bool) {
	lines := strings.Split(src, "\n")
	for i, ln := range lines {
		m := carvConstRePy.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		rhs := m[2]
		if len(rhs) == 0 {
			continue
		}
		var newLine string
		if len(rhs) >= 2 && (rhs[0] == '"' || rhs[0] == '\'') && rhs[len(rhs)-1] == rhs[0] {
			newLine = m[1] + " = " + string(rhs[0]) + "reprove-x" + string(rhs[len(rhs)-1])
		} else if carvIntRe.MatchString(rhs) {
			n, _ := strconv.Atoi(rhs)
			newLine = fmt.Sprintf("%s = %d", m[1], n+1)
		} else if (rhs[0] == '(' || rhs[0] == '[') && (rhs[len(rhs)-1] == ')' || rhs[len(rhs)-1] == ']') {
			trimmed := carvPyRStrip(rhs[:len(rhs)-1])
			sep := ""
			if strings.HasSuffix(trimmed, "\"") || strings.HasSuffix(trimmed, "'") {
				sep = ", "
			}
			newLine = m[1] + " = " + rhs[:len(rhs)-1] + sep + rhs[len(rhs)-1:]
		} else {
			continue
		}
		lines[i] = newLine
		return strings.Join(lines, "\n"), true
	}
	return "", false
}

// carvBreakOutputPy —— 手写 py `(?<![\w.])print\s*\(` 的左起第一命中（RE2 无负向先行）。
func carvBreakOutputPy(src string) (string, bool) {
	r := []rune(src)
	for i := 0; i+5 <= len(r); i++ {
		if string(r[i:i+5]) != "print" {
			continue
		}
		if i > 0 {
			prev := r[i-1]
			if carvIsWordChar(prev) || prev == '.' {
				continue
			}
		}
		j := i + 5
		for j < len(r) && carvIsPySpace(r[j]) {
			j++
		}
		if j < len(r) && r[j] == '(' {
			return string(r[:i]) + "_reprove_out(" + string(r[j+1:]), true
		}
	}
	return "", false
}

func carvRc1Exit(rel, src string) string {
	lines := strings.Split(src, "\n")
	ins := "import sys as _reprove_sys; _reprove_sys.exit(1)  # 反证扰动（rc=1 语义词族 · 必复原）"
	if strings.HasSuffix(rel, ".sh") {
		ins = "exit 1  # 反证扰动（rc=1 语义词族 · 体检用 · 必复原）"
	}
	at := 1
	if strings.HasSuffix(rel, ".py") && len(lines) > 1 && strings.Contains(lines[1], "coding") {
		at = 2
	}
	lines = append(lines, "")
	copy(lines[at+1:], lines[at:])
	lines[at] = ins
	return strings.Join(lines, "\n")
}

func carvSafeCall(fn func(string) (string, bool), src string) (out string, ok bool) {
	defer func() {
		if recover() != nil {
			out, ok = "", false
		}
	}()
	return fn(src)
}

func carvSemanticMutate(rel, src string) (string, string) {
	type carvForm struct {
		name string
		fn   func(string) (string, bool)
	}
	var forms []carvForm
	if strings.HasSuffix(rel, ".sh") {
		forms = []carvForm{{"gut-body", carvGutBodySh}, {"rc1-exit", nil}}
	} else {
		forms = []carvForm{
			{"gut-body", carvGutBodyPy},
			{"flip-constant", carvFlipConstantPy},
			{"break-output", carvBreakOutputPy},
			{"rc1-exit", nil},
		}
	}
	for _, f := range forms {
		if f.fn == nil {
			return carvRc1Exit(rel, src), f.name
		}
		out, ok := carvSafeCall(f.fn, src)
		if ok && out != src {
			return out, f.name
		}
	}
	return carvRc1Exit(rel, src), "rc1-exit"
}

func carvChooseMutation(refs []carvRef, mutation string) string {
	if mutation == carvMutSemantic {
		return carvMutSemantic
	}
	if mutation == carvMutAuto {
		for _, r := range refs {
			if carvIsReportOnly(r.mode) {
				return carvMutSemantic
			}
		}
		return carvMutInsert
	}
	return carvMutInsert
}

// ── 步表解析（唯一真源 = precommit-gates.sh 的 add_step 行）────────────────────────

type carvStep struct{ scope, name, mode, dir, cmd string }
type carvRef struct{ name, mode string }
type carvSample struct {
	rel  string
	refs []carvRef
}

func carvParseSteps(root string) []carvStep {
	fp := carvPyJoin(root, carvPrecommitRel)
	fi, err := os.Stat(fp)
	if err != nil || fi.IsDir() {
		return nil
	}
	text, rerr := carvRead(fp)
	if rerr != nil {
		return nil
	}
	// 件在盘 ⇒ 即使 0 条 add_step 也返**空表非 nil**（对齐 py `parse_steps`：存在的件返 []，
	// `if steps is None` 只咬缺件）—— nil 只留给上面两处「取不到件」。
	out := []carvStep{}
	for _, m := range carvAddStepRe.FindAllStringSubmatch(text, -1) {
		out = append(out, carvStep{scope: m[1], name: m[2], mode: m[3], dir: m[4], cmd: m[5]})
	}
	return out
}

func carvStepsByFile(steps []carvStep) map[string][]carvRef {
	d := map[string][]carvRef{}
	for _, s := range steps {
		for _, mt := range carvGateScriptRe.FindAllStringSubmatch(s.cmd, -1) {
			base := mt[1]
			if _, ok := d[base]; !ok {
				d[base] = nil
			}
			found := false
			for _, r := range d[base] {
				if r.name == s.name && r.mode == s.mode {
					found = true
					break
				}
			}
			if !found {
				d[base] = append(d[base], carvRef{name: s.name, mode: s.mode})
			}
		}
	}
	return d
}

func carvCandidates(root string, steps []carvStep) []carvSample {
	by := carvStepsByFile(steps)
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []carvSample
	for _, base := range keys {
		if base == carvSelfBase {
			continue
		}
		rel := carvPyJoin(carvGatesDirRel, base)
		if fi, err := os.Stat(carvPyJoin(root, rel)); err == nil && !fi.IsDir() {
			out = append(out, carvSample{rel: rel, refs: by[base]})
		}
	}
	return out
}

func carvPick(samples []carvSample, n int) []carvSample {
	if len(samples) == 0 {
		return nil
	}
	if n >= len(samples) {
		return append([]carvSample{}, samples...)
	}
	if n <= 1 {
		return []carvSample{samples[0]}
	}
	var idx []int
	for i := 0; i < n; i++ {
		v := math.RoundToEven(float64(i) * float64(len(samples)-1) / float64(n-1))
		idx = append(idx, int(v))
	}
	seen := map[int]bool{}
	var out []carvSample
	for _, i := range idx {
		if !seen[i] {
			seen[i] = true
			out = append(out, samples[i])
		}
	}
	return out
}

// ── 跑一步（真起子进程 · 环境清面照派单铁律）──────────────────────────────────────

func carvCleanEnv() []string {
	drop := map[string]bool{
		"ZERG_STATE_DIR": true, "ZERG_REPO": true,
		"ZERG_REPO_ROOT": true, "ZERG_PROPOSAL_DIR": true,
	}
	var out []string
	for _, kv := range os.Environ() {
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			continue
		}
		if drop[kv[:i]] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func carvRunStep(root, name string, timeoutSec int) (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", carvPrecommitRel, "--step", name, "--only-step")
	cmd.Dir = root
	cmd.Env = carvCleanEnv()
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return carvExitTimeout, fmt.Sprintf("[超时 %ds]", timeoutSec)
	}
	rc := carvExitOK
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = carvExitSpawnFail
		}
	}
	return rc, carvLastRunes(carvDecodeReplace(out), 400)
}

// ── 受控回滚（备份落仓外 + sha 逐字节核验）────────────────────────────────────────

type carvGuard struct {
	root      string
	rel       string
	abs       string
	backupDir string
	bak       string
	pre       string
	active    bool
}

func (g *carvGuard) arm() string {
	g.abs = carvPyJoin(g.root, g.rel)
	if g.backupDir != "" {
		g.bak = carvPyJoin(g.backupDir, strings.ReplaceAll(g.rel, "/", "__"))
	}
	g.pre = carvSHA(g.abs)
	if g.backupDir != "" {
		_ = os.MkdirAll(g.backupDir, 0o755)
		_ = carvCopy2(g.abs, g.bak)
	}
	g.active = true
	return g.pre
}

func (g *carvGuard) perturb(technique string) (bool, string, string) {
	src, _ := carvRead(g.abs)
	form := "前置插入"
	if technique == carvMutSemantic {
		newSrc, f := carvSemanticMutate(g.rel, src)
		form = f
		carvWritePreserve(g.abs, newSrc)
	} else {
		lines := strings.Split(src, "\n")
		ins := carvMutPy
		if strings.HasSuffix(g.rel, ".sh") {
			ins = carvMutSh
		}
		at := 1
		if strings.HasSuffix(g.rel, ".py") && len(lines) > 1 && strings.Contains(lines[1], "coding") {
			at = 2
		}
		lines = append(lines, "")
		copy(lines[at+1:], lines[at:])
		lines[at] = strings.TrimSuffix(ins, "\n")
		carvWritePreserve(g.abs, strings.Join(lines, "\n"))
	}
	now := carvSHA(g.abs)
	if now == g.pre {
		return false, "扰动后 sha 未变（扰动没生效）⇒ 不给结论", form
	}
	return true, carvHead12(now), form
}

func carvWritePreserve(p, content string) {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(p); err == nil {
		mode = fi.Mode().Perm()
	}
	_ = os.WriteFile(p, []byte(content), mode)
}

func (g *carvGuard) restore() (bool, string) {
	if !g.active {
		return true, "未扰动"
	}
	if g.backupDir != "" {
		if _, err := os.Stat(g.bak); err == nil {
			_ = carvCopy2(g.bak, g.abs)
		}
	}
	now := carvSHA(g.abs)
	g.active = false
	if now != g.pre {
		return false, fmt.Sprintf("复原后 sha 不一致（%s ≠ %s）⇒ 仓内可能残留扰动体", carvHead12(now), carvHead12(g.pre))
	}
	return true, fmt.Sprintf("复原逐字节一致 sha=%s", carvHead12(now))
}

var carvGuards []*carvGuard

func carvGuardsRemove(g *carvGuard) {
	for i, x := range carvGuards {
		if x == g {
			carvGuards = append(carvGuards[:i:i], carvGuards[i+1:]...)
			return
		}
	}
}

func carvEmergencyRestore() {
	for _, g := range carvGuards {
		if g == nil {
			continue
		}
		func() {
			defer func() { _ = recover() }()
			g.restore()
		}()
	}
}

var carvSigCh = make(chan os.Signal, 1)
var carvSigInstalled bool

func carvInstallTraps() {
	signal.Stop(carvSigCh)
	signal.Notify(carvSigCh, syscall.SIGINT, syscall.SIGTERM)
	if !carvSigInstalled {
		carvSigInstalled = true
		go func() {
			for {
				<-carvSigCh
				carvEmergencyRestore()
				os.Exit(carvExitRestore)
			}
		}()
	}
}

// ── 主判读 ────────────────────────────────────────────────────────────────────

type carvTally struct{ samples, steps, dep, nodep, unresolved int }

func carvAnalyse(root string, samples []carvSample, backupDir string, verbose bool, mutation string) (int, []string, carvTally) {
	var L []string
	add := func(s string) { L = append(L, s) }
	rcGlobal := carvExitOK
	tally := carvTally{}
	for _, sm := range samples {
		rel, refs := sm.rel, sm.refs
		tally.samples++
		tech := carvChooseMutation(refs, mutation)
		ni := make([]string, 0, len(refs))
		for _, r := range refs {
			ni = append(ni, fmt.Sprintf("%s[%s]", r.name, r.mode))
		}
		add(fmt.Sprintf("── 样本 %s（被 %d 步按名点到：%s）", rel, len(refs), strings.Join(ni, " · ")))
		g := &carvGuard{root: root, rel: rel, backupDir: backupDir}
		pre := g.arm()
		carvGuards = append(carvGuards, g)

		var greens []carvRef
		for _, r := range refs {
			tally.steps++
			rc, tail := carvRunStep(root, r.name, 600)
			if rc == 0 {
				greens = append(greens, r)
				add(fmt.Sprintf("   判据先绿 ✓ %s rc=0", r.name))
			} else {
				tally.unresolved++
				rcGlobal = carvExitHard
				add(fmt.Sprintf("   **不给结论**：判据先绿不成立 —— `%s` 扰动前就 rc=%d（✗ 不许把「本来就红」读成「扰动引起的红」）", r.name, rc))
				if verbose {
					add(fmt.Sprintf("       尾部：%s", carvTailEllipsis(tail)))
				}
			}
		}
		if len(greens) == 0 {
			carvGuardsRemove(g)
			g.restore()
			continue
		}

		ok, note, form := g.perturb(tech)
		if !ok {
			add(fmt.Sprintf("   **不给结论**：%s", note))
			carvGuardsRemove(g)
			g.restore()
			rcGlobal = carvExitHard
			continue
		}
		if tech == carvMutSemantic {
			add(fmt.Sprintf("   扰动生效 ✓ %s：sha %s → %s（rc=1 语义词族 · %s · 必复原）", rel, carvHead12(pre), note, form))
		} else {
			add(fmt.Sprintf("   扰动生效 ✓ %s：sha %s → %s（前置插入 · 必复原）", rel, carvHead12(pre), note))
		}

		for _, r := range greens {
			rc2, tail := carvRunStep(root, r.name, 600)
			if rc2 != 0 {
				tally.dep++
				add(fmt.Sprintf("   **真依赖** ✓ %s 扰动后 rc=%d（该步确实读了 %s）", r.name, rc2, rel))
			} else {
				tally.nodep++
				extra := ""
				if tech == carvMutSemantic && carvIsReportOnly(r.mode) {
					extra = "（只报档：rc=1 落 REPORT ⇒ **不影响门退出码** · 「只报档＝真无依赖」这一态正是 exit(97) 结构上取不到的那格）"
				}
				add(fmt.Sprintf("   **真无依赖** ✓ %s 扰动后仍 rc=0（该步**并不真读** %s —— 可正当留在未声明）%s", r.name, rel, extra))
			}
			if verbose {
				add(fmt.Sprintf("       尾部：%s", carvTailEllipsis(tail)))
			}
		}

		rok, rnote := g.restore()
		carvGuardsRemove(g)
		if !rok {
			add(fmt.Sprintf("   **硬失败（rc=3）**：%s", rnote))
			return carvExitRestore, L, tally
		}
		add("   " + rnote)
	}

	add(fmt.Sprintf("== 读数：样本 %d 件 · 步 %d 条 · 真依赖 %d · 真无依赖 %d · 不给结论 %d",
		tally.samples, tally.steps, tally.dep, tally.nodep, tally.unresolved))
	if tally.samples == 0 {
		add("⇒ **不给结论（rc=2）**：样本集为空（空转 ≠ 全绿）")
		return carvExitHard, L, tally
	}
	if tally.dep == 0 && tally.nodep == 0 {
		add("⇒ **不给结论（rc=2）**：一条成对判读都没跑出来（空转 ≠ 全绿）")
		return carvExitHard, L, tally
	}
	if rcGlobal == carvExitOK && tally.dep == 0 {
		add("⇒ rc=1：样本**全部**落「真无依赖」（一次红都没见）⇒ 判据可能没牙，✗ 不许读成「无漏声明」；交回人复核样本选取")
		return carvExitUnresolved, L, tally
	}
	if rcGlobal == carvExitOK {
		add("⇒ rc=0：逐件成对判读完成 · 复原逐字节一致 · 「0 处」与「没跑」已由上行读数分辨")
	} else {
		add("⇒ rc=2：有样本落「不给结论」（上列逐条点名 · fail-closed）")
	}
	return rcGlobal, L, tally
}

// ══════════════════════════ 自检（成对正负控 · 仓外夹具）══════════════════════════

// 夹具步表（自检专用 · 仓外）：add_step 行是驱动解析的唯一真源；runner 按步档把子件 rc 映射成门出口码。
// ★ 子件解释器钉成 /usr/bin/python3（GAP-20261001-61：测件里 python3 一律钉死并在回执现读版本）。
const carvFixStub = `#!/usr/bin/env bash
# 夹具步表（自检专用 · 仓外）：add_step 行是驱动解析的**唯一真源**；
# 下面的 runner 是这棵树自己的「闸」，按步档（tri / tri-report）把子件 rc 映射成门的出口码
#   —— 照真门 _judge() 的四档口径（tri-report 只差「rc=1 ⇒ 不影响退出码」那一格）。
FIX_ROOT="__FIXROOT__"
# add_step gates "夹具步A" tri "${REPO_ROOT}" "/usr/bin/python3 scripts/gates/gate-a.py"
# add_step gates "夹具步B" tri "${REPO_ROOT}" "/usr/bin/python3 scripts/gates/gate-b.py"
# add_step gates "夹具步C(吞rc)" tri "${REPO_ROOT}" "/usr/bin/python3 scripts/gates/gate-c.py"
# add_step gates "夹具步D(只报)" tri-report "${REPO_ROOT}" "/usr/bin/python3 scripts/gates/gate-d.py"
run() { ( cd "${FIX_ROOT}" && bash -c "$1" ); }
_j() {  # _j <模式> <子件 rc> ⇒ 门出口码（0 全绿 · 1 有失败项 · 2 不给结论）
  case "$1" in
    tri)        case "$2" in 0) return 0;; 1) return 1;; 2) return 2;; *) return 1;; esac ;;
    tri-report) case "$2" in 0) return 0;; 1) return 0;; 2) return 2;; *) return 1;; esac ;;
    rc)         case "$2" in 0) return 0;; *) return 1;; esac ;;
    *)          return 1 ;;
  esac
}
step=""; for a in "$@"; do case "$a" in --step) shift; step="$1"; break;; esac; done
case "$step" in
  "夹具步A") run "/usr/bin/python3 scripts/gates/gate-a.py"; r=$?; _j tri "$r"; exit $?;;
  "夹具步B") run "/usr/bin/python3 scripts/gates/gate-b.py"; r=$?; _j tri "$r"; exit $?;;
  "夹具步C(吞rc)") run "/usr/bin/python3 scripts/gates/gate-c.py"; echo "夹具：步C 吞掉 rc ⇒ 恒 0"; _j tri 0; exit $?;;
  "夹具步D(只报)") run "/usr/bin/python3 scripts/gates/gate-d.py"; r=$?; _j tri-report "$r"; exit $?;;
  *) echo "夹具：没有这一步「$step」" >&2; exit 2;;
esac
`

const carvFixGateA = `#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import os, sys


def main():
    p = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data.txt")
    return 0 if open(p).read().strip() == "OK" else 1


if __name__ == "__main__":
    sys.exit(main())
`

const carvFixGateD = `#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import os, sys


def main():
    p = os.path.join(os.path.dirname(os.path.abspath(__file__)), "data-d.txt")
    return 0 if open(p).read().strip() == "OK" else 1


if __name__ == "__main__":
    sys.exit(main())
`

const carvFixGateB = "#!/usr/bin/env python3\nimport sys\nsys.exit(0)\n"
const carvFixGateC = "#!/usr/bin/env python3\nimport sys\nsys.exit(0)\n"

const carvWantCases = 14

func carvMkFixture(base, name string) string {
	fix, err := os.MkdirTemp(base, name+"-")
	if err != nil {
		return ""
	}
	g := carvPyJoin(fix, "scripts", "gates")
	_ = os.MkdirAll(g, 0o755)
	_ = os.WriteFile(carvPyJoin(g, "precommit-gates.sh"),
		[]byte(strings.ReplaceAll(carvFixStub, "__FIXROOT__", fix)), 0o644)
	for _, nv := range []struct {
		n, body string
	}{
		{"gate-a.py", carvFixGateA}, {"gate-b.py", carvFixGateB},
		{"gate-c.py", carvFixGateC}, {"gate-d.py", carvFixGateD},
	} {
		_ = os.WriteFile(carvPyJoin(g, nv.n), []byte(nv.body), 0o644)
	}
	for _, d := range []string{"data.txt", "data-d.txt"} {
		_ = os.WriteFile(carvPyJoin(g, d), []byte("OK\n"), 0o644)
	}
	return fix
}

// carvRunFixture —— 对应 py `_run_fixture`（合并 stdout+stderr）—— 本件走**同进程**调用。
func carvRunFixture(fix string, argv []string) (int, string) {
	old, err := os.Getwd()
	if err != nil {
		old = ""
	}
	if err := os.Chdir(fix); err != nil {
		return carvExitHard, ""
	}
	var buf bytes.Buffer
	rc := checkAffectedReproveGateMain(&buf, &buf, argv)
	if old != "" {
		_ = os.Chdir(old)
	}
	return rc, buf.String()
}

// carvInproc —— 对应 py `_inproc`（只捕 stdout）。
func carvInproc(fix string, argv []string) (int, string) {
	old, err := os.Getwd()
	if err != nil {
		old = ""
	}
	if err := os.Chdir(fix); err != nil {
		return carvExitHard, ""
	}
	var out, errb bytes.Buffer
	rc := checkAffectedReproveGateMain(&out, &errb, argv)
	if old != "" {
		_ = os.Chdir(old)
	}
	return rc, out.String()
}

func carvSelfTest(stdout, stderr io.Writer) int {
	var fails []string
	total := 0
	caseFn := func(tag, desc string, ok bool, detail string) {
		total++
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s %s\n       %s\n", mark, tag, desc, detail)
		if !ok {
			fails = append(fails, tag+" "+desc)
		}
	}

	fmt.Fprintln(stdout, "── check-affected-reprove.py 自检（成对正负控 · 仓外夹具硬拷贝 · 不碰真目标）──")
	base, err := os.MkdirTemp("", "affected-reprove-selftest-")
	if err != nil {
		fmt.Fprintln(stderr, "✗ 判不了：建不了自检根 ⇒ rc=2")
		return carvExitHard
	}
	defer os.RemoveAll(base)

	// ① 正控
	fix := carvMkFixture(base, "healthy")
	gsha := carvSHA(carvPyJoin(fix, carvFixDriverRel))
	tsha := carvSHA(carvPyJoin(fix, carvGateScriptReRel("gate-a.py")))
	rc, out := carvRunFixture(fix, []string{"--files", "scripts/gates/gate-a.py"})
	hasDep := strings.Contains(out, "真依赖")
	hasRest := strings.Contains(out, "复原逐字节一致")
	caseFn("①正控", "扰动 gate-a.py ⇒ 夹具步A 变红 ⇒ 判「真依赖」· rc=0 · 复原逐字节",
		rc == 0 && hasDep && hasRest && strings.Contains(out, "· 真依赖 1 ·"),
		fmt.Sprintf("rc=%d · 真依赖=%s · 复原=%s", rc, carvBoolPy(hasDep), carvBoolPy(hasRest)))
	driverSame := carvSHA(carvPyJoin(fix, carvFixDriverRel)) == gsha
	caseFn("①b真目标只读", "自检全程只动仓外夹具：夹具驱动件与靶件 sha 与开跑前同值",
		driverSame && carvSHA(carvPyJoin(fix, carvGateScriptReRel("gate-a.py"))) == tsha,
		fmt.Sprintf("驱动件/靶件 sha %s", map[bool]string{true: "同值", false: "**变了**"}[driverSame]))

	// ② 成对负控（吞 rc 的步）
	rc, out = carvRunFixture(fix, []string{"--files", "scripts/gates/gate-c.py"})
	caseFn("②成对负控", "吞 rc 的步 ⇒ 扰动后仍 rc=0 ⇒ 判「真无依赖」· rc=1（一次红都没见 ⇒ 不许读成绿）",
		rc == 1 && strings.Contains(out, "真无依赖") && !strings.Contains(out, "**真依赖** ✓"),
		fmt.Sprintf("rc=%d（期望 1）· 真无依赖=%s", rc, carvBoolPy(strings.Contains(out, "真无依赖"))))

	// ③ 负控：判据先绿不成立
	fixBad := carvMkFixture(base, "redbaseline")
	_ = os.WriteFile(carvPyJoin(fixBad, "scripts", "gates", "data.txt"), []byte("BROKEN\n"), 0o644)
	rc, out = carvRunFixture(fixBad, []string{"--files", "scripts/gates/gate-a.py"})
	caseFn("③负控", "基线本就红 ⇒ 不给结论（rc=2）且逐字点名「判据先绿不成立」· 不报绿",
		rc == 2 && strings.Contains(out, "判据先绿不成立") && !strings.Contains(out, "**真依赖** ✓"),
		fmt.Sprintf("rc=%d（期望 2）· 点名=%s", rc, carvBoolPy(strings.Contains(out, "判据先绿不成立"))))

	// ④ 负控：复原失败 ⇒ rc=3（弄哑「还原」那一次 copy2）
	fixNr := carvMkFixture(base, "no-restore")
	realCopy2 := carvCopy2
	carvCopy2 = func(src, dst string) error {
		if strings.Contains(carvPyAbspath(dst), "/scripts/gates/") {
			return nil
		}
		return realCopy2(src, dst)
	}
	rc, out = carvInproc(fixNr, []string{"--files", "scripts/gates/gate-a.py"})
	carvCopy2 = realCopy2
	still, _ := carvRead(carvPyJoin(fixNr, "scripts", "gates", "gate-a.py"))
	stillMutated := strings.Contains(still, "exit(97)")
	caseFn("④负控", "复原不落地 ⇒ 驱动 rc=3 且逐字点名「复原后 sha 不一致」（复原核验是活的，不是摆设）",
		rc == 3 && strings.Contains(out, "复原后 sha 不一致"),
		fmt.Sprintf("rc=%d（期望 3）· 扰动仍在盘上=%s（仓外夹具 · 随夹具清掉）", rc, carvBoolPy(stillMutated)))

	// ⑤ 负控：件不在任何一步命令位
	fixOrphan := carvMkFixture(base, "orphan")
	_ = os.WriteFile(carvPyJoin(fixOrphan, "scripts", "gates", "gate-z.py"),
		[]byte("#!/usr/bin/env python3\nimport sys\nsys.exit(0)\n"), 0o644)
	rc, out = carvRunFixture(fixOrphan, []string{"--files", "scripts/gates/gate-z.py"})
	caseFn("⑤负控", "件不在任何一步命令位 ⇒ rc=2「无步引用」（✗ 不许读成「无依赖」）",
		rc == 2 && strings.Contains(out, "无步引用"),
		fmt.Sprintf("rc=%d · 点名=%s", rc, carvBoolPy(strings.Contains(out, "无步引用"))))

	// ⑥ 防空绿：弄哑 sha
	fixMute := carvMkFixture(base, "mute-sha")
	realSHA := carvSHA
	carvSHA = func(string) string { return "MUTE" }
	rc, out = carvInproc(fixMute, []string{"--files", "scripts/gates/gate-a.py"})
	carvSHA = realSHA
	caseFn("⑥防空绿", "弄哑 sha（扰动生效断言瞎）⇒ 驱动 rc=2 且点名「扰动没生效」（弄哑而不变=真源本来是死的）",
		rc == 2 && strings.Contains(out, "扰动没生效"),
		fmt.Sprintf("rc=%d · 点名=%s", rc, carvBoolPy(strings.Contains(out, "扰动没生效"))))

	// ⑦ 正控成对
	rc, _ = carvRunFixture(fix, []string{"--files", "scripts/gates/gate-a.py"})
	caseFn("⑦正控成对", "与 ⑥ 成对：sha 真源在位 ⇒ 同一棵树回 rc=0（不是恒判不了）",
		rc == 0, fmt.Sprintf("rc=%d", rc))

	// ⑧ rc=1 语义词族 · 正控（阻断档）
	rc, out = carvRunFixture(fix, []string{"--mutation", "semantic", "--files", "scripts/gates/gate-a.py"})
	caseFn("⑧语义词族正控", "阻断档 + semantic（删函数体）⇒ 子件 rc=1 落 FAIL ⇒ 判「真依赖」· rc=0 · 复原",
		rc == 0 && strings.Contains(out, "真依赖") && strings.Contains(out, "gut-body") && strings.Contains(out, "复原逐字节一致"),
		fmt.Sprintf("rc=%d · 真依赖=%s · 手法=%s", rc, carvBoolPy(strings.Contains(out, "真依赖")), carvBoolPy(strings.Contains(out, "gut-body"))))

	// ⑨ rc=1 语义词族 · 负控（只报档）
	rc, out = carvRunFixture(fix, []string{"--mutation", "semantic", "--files", "scripts/gates/gate-d.py"})
	caseFn("⑨语义词族负控", "只报档 + semantic ⇒ rc=1 落 REPORT（门退码不变）⇒ 判「真无依赖」· rc=1",
		rc == 1 && strings.Contains(out, "**真无依赖** ✓") && !strings.Contains(out, "**真依赖** ✓") && strings.Contains(out, "不影响门退出码"),
		fmt.Sprintf("rc=%d（期望 1）· 真无依赖=%s · 不报真依赖=%s", rc,
			carvBoolPy(strings.Contains(out, "**真无依赖** ✓")), carvBoolPy(!strings.Contains(out, "**真依赖** ✓"))))

	// ⑩ rc=1 语义词族 · 负控（本来就红）
	rc, out = carvRunFixture(fixBad, []string{"--mutation", "semantic", "--files", "scripts/gates/gate-a.py"})
	caseFn("⑩语义词族红基线", "基线本就红 + semantic ⇒ 仍「不给结论」（rc=2 · 逐字点名）· 不报绿不报红",
		rc == 2 && strings.Contains(out, "判据先绿不成立") &&
			!strings.Contains(out, "**真依赖** ✓") && !strings.Contains(out, "**真无依赖** ✓"),
		fmt.Sprintf("rc=%d（期望 2）· 点名=%s", rc, carvBoolPy(strings.Contains(out, "判据先绿不成立"))))

	// ⑪ auto：只报档样本 ⇒ semantic
	rc, out = carvRunFixture(fix, []string{"--mutation", "auto", "--files", "scripts/gates/gate-d.py"})
	caseFn("⑪auto按档选", "auto：只报档样本 ⇒ 选 rc=1 语义词族（真无依赖 · rc=1；不是前置插入）",
		rc == 1 && strings.Contains(out, "**真无依赖** ✓") && strings.Contains(out, "rc=1 语义词族") && !strings.Contains(out, "前置插入"),
		fmt.Sprintf("rc=%d（期望 1）· 语义词族=%s", rc, carvBoolPy(strings.Contains(out, "rc=1 语义词族"))))

	// ⑫ auto：阻断档样本 ⇒ insert
	rc, out = carvRunFixture(fix, []string{"--mutation", "auto", "--files", "scripts/gates/gate-a.py"})
	caseFn("⑫auto阻断档", "auto：阻断档样本 ⇒ 仍选旧手法「前置插入」⇒ 判「真依赖」· rc=0",
		rc == 0 && strings.Contains(out, "真依赖") && strings.Contains(out, "前置插入"),
		fmt.Sprintf("rc=%d · 前置插入=%s", rc, carvBoolPy(strings.Contains(out, "前置插入"))))

	// ⑬ 默认档向后兼容
	gsha2 := carvSHA(carvPyJoin(fix, "scripts", "gates", "gate-a.py"))
	rc, out = carvRunFixture(fix, []string{"--files", "scripts/gates/gate-a.py"})
	same := carvSHA(carvPyJoin(fix, "scripts", "gates", "gate-a.py")) == gsha2
	caseFn("⑬默认档不变", "不给 --mutation ⇒ 旧路径（前置插入）· 结论与 ⑫ 逐字同 · 靶件 sha 与开跑前同",
		rc == 0 && strings.Contains(out, "真依赖") && strings.Contains(out, "前置插入") && same,
		fmt.Sprintf("rc=%d · 还原=%s", rc, carvBoolPy(same)))

	fmt.Fprintf(stdout, "  自证用例：%d 条 · 断言条数 %d（少跑一格即失效）\n", total, carvWantCases)
	if total != carvWantCases {
		fmt.Fprintf(stderr, "✗ 判不了：自检用例数对不上（应 %d，实跑 %d）⇒ rc=2\n", carvWantCases, total)
		return carvExitHard
	}
	if len(fails) > 0 {
		fmt.Fprintln(stdout, "✗ 自检不过：")
		for _, x := range fails {
			fmt.Fprintln(stdout, "   "+x)
		}
		return carvExitUnresolved
	}
	fmt.Fprintln(stdout, "⇒ 自检全过（rc=0）：正控 / 成对负控 / 判据先绿 / 无步引用 / 防空绿 / rc=1 语义词族（按档） 逐条对拍 ✓")
	return carvExitOK
}

func carvGateScriptReRel(name string) string { return carvPyJoin(carvGatesDirRel, name) }

// ── 入口 ──────────────────────────────────────────────────────────────────────

type carvOpts struct {
	root          string
	list          bool
	sample        int
	files         string
	json          bool
	verbose       bool
	mutation      string
	mutationGiven bool
	selfTest      bool
	help          bool
}

// carvUsageLine / carvHelpText —— 逐字复刻 /usr/bin/python3(3.9.6) argparse 的 usage/help 正文
// （非 tty ⇒ width=78；⚠ COLUMNS 或解释器版本变则正文变，见件头未覆盖面①）。
const carvUsageLine = "usage: check-affected-reprove.py [-h] [--root ROOT] [--list] [--sample SAMPLE]\n" +
	"                                 [--files FILES] [--json] [--verbose]\n" +
	"                                 [--mutation {insert,semantic,auto}]\n" +
	"                                 [--self-test]\n"

const carvHelpTail = "\n" +
	"反证体检（主法丙·成对扰动式）：扰动一件，看哪一步真红\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT\n" +
	"  --list                只列候选样本（不跑、不写盘）\n" +
	"  --sample SAMPLE       等距抽 N 件真跑\n" +
	"  --files FILES         点名样本（逗号分隔 · 仓内相对路径或 basename）\n" +
	"  --json\n" +
	"  --verbose\n" +
	"  --mutation {insert,semantic,auto}\n" +
	"                        扰动手法：insert（默认 · 旧手法 exit(97)）· semantic（rc=1 语义词族）·\n" +
	"                        auto（按步档选）\n" +
	"  --self-test\n"

func carvHelpText() string { return carvUsageLine + carvHelpTail }

func carvArgError(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, carvUsageLine)
	fmt.Fprintf(stderr, "check-affected-reprove.py: error: %s\n", msg)
	return carvExitHard
}

func carvParseArgs(argv []string) (carvOpts, string) {
	opts := carvOpts{root: carvDefaultRoot(), mutation: carvMutInsert}
	i := 0
	needVal := func(flag string) (string, string) {
		if i+1 >= len(argv) {
			return "", "argument " + flag + ": expected one argument"
		}
		i++
		return argv[i], ""
	}
	for ; i < len(argv); i++ {
		a := argv[i]
		name, val, hasEq := a, "", false
		if strings.HasPrefix(a, "--") {
			if eq := strings.IndexByte(a, '='); eq >= 0 {
				name, val, hasEq = a[:eq], a[eq+1:], true
			}
		}
		switch name {
		case "-h", "--help":
			if hasEq {
				return opts, "argument " + name + ": ignored explicit argument '" + val + "'"
			}
			opts.help = true
		case "--list":
			if hasEq {
				return opts, "argument --list: ignored explicit argument '" + val + "'"
			}
			opts.list = true
		case "--json":
			if hasEq {
				return opts, "argument --json: ignored explicit argument '" + val + "'"
			}
			opts.json = true
		case "--verbose":
			if hasEq {
				return opts, "argument --verbose: ignored explicit argument '" + val + "'"
			}
			opts.verbose = true
		case "--self-test":
			if hasEq {
				return opts, "argument --self-test: ignored explicit argument '" + val + "'"
			}
			opts.selfTest = true
		case "--root":
			v := val
			if !hasEq {
				var errMsg string
				v, errMsg = needVal("--root")
				if errMsg != "" {
					return opts, errMsg
				}
			}
			opts.root = v
		case "--files":
			v := val
			if !hasEq {
				var errMsg string
				v, errMsg = needVal("--files")
				if errMsg != "" {
					return opts, errMsg
				}
			}
			opts.files = v
		case "--sample":
			v := val
			if !hasEq {
				var errMsg string
				v, errMsg = needVal("--sample")
				if errMsg != "" {
					return opts, errMsg
				}
			}
			n, cerr := strconv.Atoi(v)
			if cerr != nil {
				return opts, "argument --sample: invalid int value: '" + v + "'"
			}
			opts.sample = n
		case "--mutation":
			v := val
			if !hasEq {
				var errMsg string
				v, errMsg = needVal("--mutation")
				if errMsg != "" {
					return opts, errMsg
				}
			}
			if v != carvMutInsert && v != carvMutSemantic && v != carvMutAuto {
				return opts, "argument --mutation: invalid choice: '" + v + "' (choose from 'insert', 'semantic', 'auto')"
			}
			opts.mutation = v
			opts.mutationGiven = true
		default:
			return opts, "unrecognized arguments: " + a
		}
	}
	return opts, ""
}

// carvDefaultRoot —— 无 `__file__` 语境 ⇒ cwd 上溯找含 scripts/gates 的目录、找不到回落 cwd。
func carvDefaultRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := cwd
	for {
		if fi, err := os.Stat(carvPyJoin(dir, "scripts", "gates")); err == nil && fi.IsDir() {
			return dir
		}
		parent := carvPyDirname(dir)
		if parent == "" || parent == dir || parent == "/" {
			if parent == "/" {
				if fi, err := os.Stat(carvPyJoin("/", "scripts", "gates")); err == nil && fi.IsDir() {
					return "/"
				}
			}
			break
		}
		dir = parent
	}
	return cwd
}

func carvJSONStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		case '\b':
			b.WriteString("\\b")
		case '\f':
			b.WriteString("\\f")
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func carvJSONPayload(rc int, mutation string, mutationGiven bool, tally carvTally, lines []string) string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(" \"schema\": " + carvJSONStr("zerg/v1") + ",\n")
	b.WriteString(" \"kind\": " + carvJSONStr("AffectedReprove") + ",\n")
	fmt.Fprintf(&b, " \"rc\": %d,\n", rc)
	if mutationGiven {
		b.WriteString(" \"mutation\": " + carvJSONStr(mutation) + ",\n")
	}
	b.WriteString(" \"tally\": {\n")
	fmt.Fprintf(&b, "  \"samples\": %d,\n", tally.samples)
	fmt.Fprintf(&b, "  \"steps\": %d,\n", tally.steps)
	fmt.Fprintf(&b, "  \"dep\": %d,\n", tally.dep)
	fmt.Fprintf(&b, "  \"nodep\": %d,\n", tally.nodep)
	fmt.Fprintf(&b, "  \"unresolved\": %d\n", tally.unresolved)
	b.WriteString(" },\n")
	if len(lines) == 0 {
		b.WriteString(" \"lines\": []")
	} else {
		b.WriteString(" \"lines\": [\n")
		for i, x := range lines {
			b.WriteString("  " + carvJSONStr(x))
			if i < len(lines)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(" ]")
	}
	b.WriteString("\n}")
	return b.String()
}

func checkAffectedReproveGateMain(stdout, stderr io.Writer, argv []string) int {
	carvGuards = nil
	opts, perr := carvParseArgs(argv)
	if perr != "" {
		return carvArgError(stderr, perr)
	}
	if opts.help {
		fmt.Fprint(stdout, carvHelpText())
		return carvExitOK
	}
	if opts.selfTest {
		return carvSelfTest(stdout, stderr)
	}

	root := carvPyAbspath(opts.root)
	steps := carvParseSteps(root)
	if steps == nil {
		fmt.Fprintf(stderr, "[不给结论] 取不到步表 %s ⇒ rc=2（判不了 ≠ 绿）\n", carvPrecommitRel)
		return carvExitHard
	}
	cands := carvCandidates(root, steps)
	by := map[string][]carvRef{}
	for _, c := range cands {
		by[c.rel] = c.refs
	}

	if opts.list {
		fmt.Fprintf(stdout, "候选样本 %d 件（步表命令位按名点到 · 排序）：\n", len(cands))
		for _, c := range cands {
			ni := make([]string, 0, len(c.refs))
			for _, r := range c.refs {
				ni = append(ni, fmt.Sprintf("%s[%s]", r.name, r.mode))
			}
			fmt.Fprintf(stdout, "  %-46s ← %d 步：%s\n", c.rel, len(c.refs), strings.Join(ni, " · "))
		}
		return carvExitOK
	}

	var samples []carvSample
	if opts.files != "" {
		var want []string
		for _, x := range strings.Split(opts.files, ",") {
			if s := strings.TrimSpace(x); s != "" {
				want = append(want, s)
			}
		}
		for _, w := range want {
			rel := w
			if !strings.Contains(w, "/") {
				rel = "scripts/gates/" + w
			}
			refs, ok := by[rel]
			if !ok {
				fmt.Fprintf(stderr, "[不给结论] 样本 %s 不在步表命令位里（无步引用）⇒ rc=2\n", rel)
				return carvExitHard
			}
			samples = append(samples, carvSample{rel: rel, refs: refs})
		}
	} else if opts.sample > 0 {
		samples = carvPick(cands, opts.sample)
	} else {
		fmt.Fprintln(stdout, "用法：--list | --sample N | --files a,b | --self-test（至少给一个 ⇒ 否则 rc=2）")
		return carvExitHard
	}

	backupDir := carvPyJoin(carvEnvOrDefault("TMPDIR", "/tmp"), fmt.Sprintf("affected-reprove.%d", os.Getpid()))
	bd := carvPyAbspath(backupDir)
	if bd == root || strings.HasPrefix(bd, root+"/") {
		fmt.Fprintf(stderr, "[不给结论] 备份根落在真仓内（%s）⇒ 绝不在仓里留备份 ⇒ rc=2\n", bd)
		return carvExitHard
	}
	carvInstallTraps()
	if opts.mutationGiven {
		fmt.Fprintf(stdout, "== 反证体检（主法丙·成对扰动式）· 仓根 %s · 手法档 %s\n", root, opts.mutation)
	} else {
		fmt.Fprintf(stdout, "== 反证体检（主法丙·成对扰动式）· 仓根 %s\n", root)
	}
	fmt.Fprintf(stdout, "== 备份落仓外 %s（受控回滚 · atexit+signal 双保险 · 跑前跑后 sha256 逐字节）\n", bd)

	rc, lines, tally := carvAnalyse(root, samples, backupDir, opts.verbose, opts.mutation)
	carvEmergencyRestore()
	_ = os.RemoveAll(backupDir)

	if opts.json {
		fmt.Fprintln(stdout, carvJSONPayload(rc, opts.mutation, opts.mutationGiven, tally, lines))
	} else {
		for _, x := range lines {
			fmt.Fprintln(stdout, x)
		}
	}
	return rc
}

func carvEnvOrDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
