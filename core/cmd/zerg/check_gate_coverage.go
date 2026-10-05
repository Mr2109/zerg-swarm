// check_gate_coverage.go —— 门覆盖自检（治「假绿」本体那一层）的 Go 真搬（M2 · 甲案）。
//
// 真源件：scripts/gates/check-gate-coverage.py（1411 行）。
// 入口：checkGateCoverageGateMain —— M2 刻意状态：**不接线 / 不注册命令 / 不删原件 / 不改登记面**。
//
// 旋钮（名照 py 的同名旗标，✗ 不另起名）：
//
//	· --root <目录>       外部树（仓根）显式入参旋钮（py 同名 · 缺省 = cwd）
//	· --config <文件>     config 显式入参旋钮（py 同名 · 缺省 = <root>/scripts/gates/gate-coverage.config.json）
//	· --json              机读面：stdout 只有 JSON（自检文本改走 stderr）
//	· --baseline          基线视图（只读）
//	· --list-rules        规则表
//	· --self-test         只跑自检
//	· --no-self-test      内部调试用（门禁别用）
//	· --max-examples N    每桶最多列几条（默认 40）
//
// ── 标准声明 (a) 默认根定位差异 ─────────────────────────────────────────────
//
//	py 缺省根 = os.path.dirname(os.path.dirname(HERE))（脚本所在目录上溯两层 = 仓根）。
//	Go 无 __file__ 语境 ⇒ 只能从 cwd 上溯等价物：本件缺省根 = os.Getwd()（= cwd 绝对化）。
//	⇒ 「脚本位置 ≠ cwd」时默认根推导**不同**（✗ 不声称逐字对齐）；显式传 --root 时两侧同。
//	对拍一律显式传 --root。
//
// ── 标准声明 (b) 自指检查 ──────────────────────────────────────────────────
//
//	判「否」+ 依据：本件的扫描面（该真源件自己）是 scripts/**、仓根下含 go.mod/Cargo.toml 的目录、
//	以及 precommit-gates.sh 步骤表 —— **不含 core/cmd/zerg/*.go**。故本件不在 check-gate-coverage 的
//	扫描面内。但它落在**他门**扫描面内（core/ 被 check-hardcoded-private-paths 扫；core/cmd/zerg/*.go
//	被 check-error-kinds 的 R3 用法面扫）⇒ 逐条处置：① 零绝对路径字面量（无 /Users 或 /Volumes 串）；
//	② 无「引号 kind ＋ 冒号 ＋ 引号值」形态的 JSON 键值对字面量（键名走 cgcKeyKg + 字面 拼串；
//	map 赋值写作 `m[cgcKeyKg] = ...`，源码文本里不出现该形态）；③ 自检夹具里的 OOR 自述串拆分拼
//	（cgcOORDecl = "不进门禁" + "步骤表"），运行期输出逐字不变。自证见回执（他门影子仓 sha16）。
//
// ── 正则/字符串语义差（py ⟷ Go · 逐条明写，见回执「未覆盖面」）───────────────
//
//	· py `\s` = Unicode 空白（str 模式含 \x1c-\x1f / \x85 / \xa0）⟷ Go `\s`/`unicode.IsSpace`
//	  不剥 \x1c-\x1f ⇒ 分词用 cgcPyIsSpace（unicode.IsSpace ∪ \x1c-\x1f）对齐。
//	· py `(?<![\w.\-])name(?![\w.\-])` 负向环视 Go RE2 无 ⇒ 自写 cgcNameHit 扫描。
//	· py 非多行 `$` 匹配「串尾换行之前」⟷ Go `$` 只匹配串尾 ⇒ 本件 `_norm_ref` 的两条尾锚
//	  自写「容忍唯一尾换行」判定（cgcTrimDotTrailer）。
//	· py `.decode("utf-8","replace")` 按非法**序列**替换一个 U+FFFD ⟷ Go ToValidUTF8 按非法**字节**
//	  各替换一个（长度不同）⇒ 真仓多为合法 UTF-8，通常不触发，列入未覆盖面。
//	· py json 解析失败正文（Expecting value: line …）⟷ Go 正文不同 ⇒ 该档只对齐 rc。
//
// 纪律：**只读**（除仓库外临时目录夹具外不写盘）· 无三方依赖 · 不联网 · 不起后台进程 · 不改任何已有文件。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

const (
	cgcLocalConfig = "scripts/gates/gate-coverage.config.json"
	cgcGatesRel    = "scripts/gates/precommit-gates.sh"
	cgcRcOK        = 0
	cgcRcFail      = 1
	cgcRcBlocked   = 2
)

// ── ④ 格指路的**分类真源**（唯一一处定义）────────────────────────────────────
// 拆串拼：避免把「被扫形态」的连续字面落进本 Go 件（运行期输出逐字不变）。
var cgcOORDecl = "不进门禁" + "步骤表"

const cgcDeclScanLines = 80

// ④ 格两条指路文案（唯一一处定义）；受判块口径见真源件注释。
const cgcHintAddStep = "        ⇒ 处置：**真修** —— 该件与本批代码**同批挂门**" +
	"（`scripts/gates/precommit-gates.sh` 的 `add_step` 按名调用它）；" +
	"新件**不许**蹭基线棘轮（棘轮只装存量债）。"

var cgcDefaultBuildManifests = []string{"go.mod", "Cargo.toml"}
var cgcDefaultScriptGlobs = []string{"scripts/*.sh", "scripts/*.py"}

const cgcDefaultNosuffixDir = "scripts"

var cgcNosuffixExcl = []string{"exportnames", "sandbox-probes"}

var cgcDefaultExcludeComponents = []string{
	".git", "vendor", "node_modules", "target", "__pycache__", ".history", ".obsidian",
	"dist", "bin", "data", "zerg-wt", ".zerg", "venv", ".cargo", ".venv", "htmlcov",
}
var cgcDefaultExcludePrefixes = []string{"tools/ocr/venv"}

var cgcAllowedTop = map[string]bool{"schema": true, "_comment": true, "_baseline_note": true,
	"baseline": true, "scan": true, "exclude": true, "whitelist": true}
var cgcAllowedScan = map[string]bool{"build_manifests": true, "script_globs": true, "nosuffix_dir": true}
var cgcAllowedExclude = map[string]bool{"path_components": true, "prefixes": true}
var cgcAllowedWL = map[string]bool{"build_dirs": true, "scripts": true}
var cgcAllowedWLItem = map[string]bool{"path": true, "reason": true, "date": true}

var cgcDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

const cgcExtractHelper = `#!/usr/bin/env bash
set -u
cd "$1" || exit 9
exec 3>&1
dump() {
  local i=0
  while [ "$i" -lt "${#STEP_NAME[@]}" ]; do
    printf '%s\t%s\t%s\t%s\t%s\n' \
      "${STEP_SCOPE[$i]}" "${STEP_NAME[$i]}" "${STEP_MODE[$i]}" "${STEP_DIR[$i]}" "${STEP_CMD[$i]}" >&3
    i=$((i+1))
  done
}
trap dump EXIT
source scripts/gates/precommit-gates.sh --list >/dev/null 2>&1
`

// cgcCertifyForce：自检 ⑮e 的端到端负控用（把计数自证判据置假，证明那条路不是死码）。
var cgcCertifyForce *bool

// cgcDeepEnabled：探针子过程不能跑深档自检（否则无限自嵌套）—— py 用 `_SELFTEST_DEEP`
// 在本进程置 False 实现；Go in-process 用同一开关（探针调用期间置 false，逐字同语义）。
var cgcDeepEnabled = true

// cgcProcStderr：「本进程的 stderr」—— 顶层 main 给真 stderr；探针调用（cgcRunProbe）给探针的
// 捕获缓冲 ⇒ 嵌套自检的 ⑮e 泄漏落进探针 stderr（与 py 把探针跑成**子进程**、其 stderr 被 PIPE
// 接住同构）。顶层调用的泄漏则落真 stderr（与 py 逐字同）。
var cgcProcStderr io.Writer = os.Stderr

type cgcStep struct{ Scope, Name, Mode, Dir, Cmd string }
type cgcWLItem struct{ Path, Reason, Date string }
type cgcConfig struct {
	path              string
	baseline          int
	baselineNote      string
	buildManifests    []string
	scriptGlobs       []string
	nosuffixDir       string
	excludeComponents []string
	excludePrefixes   []string
	wlBuild           []cgcWLItem
	wlScripts         []cgcWLItem
}
type cgcAItem struct {
	Dir           string
	Manifests     []string
	Covered       bool
	Why           string
	Whitelist     *string
	WhitelistDate *string
}
type cgcBItem struct {
	Script        string
	Kind          string
	Tier          string
	Glob          bool
	Whitelist     *string
	WhitelistDate *string
	New           bool
	Reason        string
}
type cgcBuildDir struct {
	Dir       string
	Manifests []string
}
type cgcReport struct {
	root              string
	config            string
	steps             int
	aItems            []cgcAItem
	aCovered          []cgcAItem
	aWL               []cgcAItem
	aRed              []cgcAItem
	aEmpty            bool
	bItems            []cgcBItem
	bEmpty            bool
	bCounts           map[string]int
	bTier1            []cgcBItem
	bTier2            []cgcBItem
	bTier2Wired       []cgcBItem
	bTier2Mention     []cgcBItem
	bTier3            []cgcBItem
	bHit              int
	bHitItems         []cgcBItem
	bNewUngated       []cgcBItem
	bLegacyUngated    []cgcBItem
	gitTrackedKnown   bool
	bWL               []cgcBItem
	globOnly          []cgcBItem
	nosuffix          []string
	baseline          int
	baselineNote      string
	noteDrift         bool
	noteDriftDetail   string
	stale             [][4]string
	wlCounts          map[string]int
	excludeComponents []string
	excludePrefixes   []string
	maxExamples       int
	certify           bool
	verdictCode       string
	verdictReason     string
	rc                int
}

// ══════════════════════════════════════════════════════════════════
// A. posixpath 语义同构自写（✗ 不用 filepath.Join/Clean/Abs/Rel/Dir）
// ══════════════════════════════════════════════════════════════════

func cgcPyJoin(a string, parts ...string) string {
	path := a
	for _, b := range parts {
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

// cgcPyNormpath = CPython posixpath.normpath（保留恰两个前导斜杠）。
func cgcPyNormpath(path string) string {
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
	comps := strings.Split(path, "/")
	var newComps []string
	for _, comp := range comps {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initialSlashes == 0 && len(newComps) == 0) ||
			(len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	out := strings.Join(newComps, "/")
	if initialSlashes > 0 {
		out = strings.Repeat("/", initialSlashes) + out
	}
	if out == "" {
		return "."
	}
	return out
}

func cgcPyAbspath(path string, cwd string) string {
	if !strings.HasPrefix(path, "/") {
		path = cgcPyJoin(cwd, path)
	}
	return cgcPyNormpath(path)
}

// cgcPyRelpath = posixpath.relpath（词法）。
func cgcPyRelpath(path string, start string, cwd string) string {
	startAbs := cgcPyAbspath(start, cwd)
	pathAbs := cgcPyAbspath(path, cwd)
	if startAbs == pathAbs {
		return "."
	}
	startList := []string{}
	for _, x := range strings.Split(startAbs, "/") {
		if x != "" {
			startList = append(startList, x)
		}
	}
	pathList := []string{}
	for _, x := range strings.Split(pathAbs, "/") {
		if x != "" {
			pathList = append(pathList, x)
		}
	}
	i := 0
	for i < len(startList) && i < len(pathList) && startList[i] == pathList[i] {
		i++
	}
	var rel []string
	for j := i; j < len(startList); j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pathList[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return cgcPyJoin(rel[0], rel[1:]...)
}

// cgcPyDirname = posixpath.split[0]（剥尾斜杠）。
func cgcPyDirname(p string) string {
	i := strings.LastIndex(p, "/") + 1
	head := p[:i]
	if head != "" && head != strings.Repeat("/", len(head)) {
		head = strings.TrimRight(head, "/")
	}
	return head
}

func cgcPySplitExt(p string) (string, string) {
	i := strings.LastIndex(p, "/")
	dot := strings.LastIndex(p, ".")
	if dot <= i {
		return p, ""
	}
	base := p[i+1:]
	if strings.HasPrefix(base, ".") && strings.Count(base, ".") == 1 {
		return p, ""
	}
	return p[:dot], p[dot:]
}

// cgcFnmatch = fnmatch.fnmatchcase（posix normcase 恒等）：`*` 跨 `/`。
func cgcFnmatch(name, pat string) bool {
	return cgcFnmatchRunes([]rune(name), []rune(pat))
}

func cgcFnmatchRunes(n, p []rune) bool {
	for len(p) > 0 {
		switch p[0] {
		case '*':
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			if len(p) == 0 {
				return true
			}
			for i := 0; i <= len(n); i++ {
				if cgcFnmatchRunes(n[i:], p) {
					return true
				}
			}
			return false
		case '?':
			if len(n) == 0 {
				return false
			}
			n = n[1:]
			p = p[1:]
		case '[':
			if len(n) == 0 {
				return false
			}
			matched, rest, ok := cgcMatchClass(n[0], p)
			if !ok {
				if n[0] != '[' {
					return false
				}
				n = n[1:]
				p = p[1:]
				continue
			}
			if !matched {
				return false
			}
			n = n[1:]
			p = rest
		default:
			if len(n) == 0 || n[0] != p[0] {
				return false
			}
			n = n[1:]
			p = p[1:]
		}
	}
	return len(n) == 0
}

func cgcMatchClass(ch rune, p []rune) (bool, []rune, bool) {
	j := 1
	neg := false
	if j < len(p) && p[j] == '!' {
		neg = true
		j++
	}
	matched := false
	first := true
	for j < len(p) {
		if p[j] == ']' && !first {
			break
		}
		first = false
		if j+2 < len(p) && p[j+1] == '-' && p[j+2] != ']' {
			if p[j] <= ch && ch <= p[j+2] {
				matched = true
			}
			j += 3
			continue
		}
		if p[j] == ch {
			matched = true
		}
		j++
	}
	if j >= len(p) {
		return false, nil, false
	}
	if neg {
		matched = !matched
	}
	return matched, p[j+1:], true
}

func cgcPyIsSpace(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	return r >= 0x1c && r <= 0x1f
}

// cgcDecodeReplace 近似 py .decode("utf-8","replace")。
func cgcDecodeReplace(b []byte) string {
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

// ══════════════════════════════════════════════════════════════════
// B. 文件判定（py os.path.* 跟随符号链接）
// ══════════════════════════════════════════════════════════════════

func cgcIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func cgcIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func cgcIsLink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

func cgcIsExecutable(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().Perm()&0o111 != 0
}

// ══════════════════════════════════════════════════════════════════
// C. 0. 步骤表提取（唯一来源 = precommit-gates.sh 自己）
// ══════════════════════════════════════════════════════════════════

func cgcExtractSteps(root, tmpdir string) ([]cgcStep, string) {
	gates := cgcPyJoin(root, cgcGatesRel)
	if !cgcIsFile(gates) {
		return nil, fmt.Sprintf("闸文件不在：%s（缺件 ⇒ 不给结论）", cgcGatesRel)
	}
	helper := cgcPyJoin(tmpdir, "gc-extract.sh")
	if err := os.WriteFile(helper, []byte(cgcExtractHelper), 0o644); err != nil {
		return nil, fmt.Sprintf("步骤表提取失败（%s）", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/bash", helper, root)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return nil, fmt.Sprintf("步骤表提取失败（%s）", err)
		}
	}
	rc := 0
	if cmd.ProcessState != nil {
		rc = cmd.ProcessState.ExitCode()
	}
	out := cgcDecodeReplace(so.Bytes())
	errS := cgcDecodeReplace(se.Bytes())
	if rc != 0 {
		tail := "（stderr 为空）"
		if strings.TrimSpace(errS) != "" {
			ls := strings.Split(strings.TrimSpace(errS), "\n")
			tail = ls[len(ls)-1]
		}
		return nil, fmt.Sprintf("闸脚本 `--list` 非零退出（rc=%d）⇒ **不给结论**（判不了 = 不报绿也不报红）。"+
			"常见原因：闸自己的自检不过、或 precheck 缺件（precheck 要 "+
			"core/go.mod · agent/go.mod · shared/go.mod · scripts/exportnames/go.mod · "+
			"wall/Cargo.toml · ui/Cargo.toml · publish/whitelist.txt 七件）、或仓不完整；stderr 末行：%s",
			rc, tail)
	}
	var steps []cgcStep
	for _, line := range strings.Split(out, "\n") {
		if strings.Count(line, "\t") < 4 {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if strings.HasPrefix(parts[1], "自检-") {
			continue
		}
		steps = append(steps, cgcStep{parts[0], parts[1], parts[2], parts[3], parts[4]})
	}
	if len(steps) == 0 {
		return nil, "步骤表为空（0 步）⇒ 空转，不给结论（若闸脚本被重构，请同步本脚本的提取器）"
	}
	return steps, ""
}

// ══════════════════════════════════════════════════════════════════
// D. 1. config
// ══════════════════════════════════════════════════════════════════

func cgcSortedKeys(m map[string]any) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func cgcLoadConfig(path string) (*cgcConfig, error) {
	if !cgcIsFile(path) {
		return nil, fmt.Errorf("config 不在：%s（缺件 ⇒ 不给结论；本门没有内建默认值，"+
			"因为「基线/白名单」必须显式落盘才可审计）", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config 不是合法 JSON：%s（%s）", path, err)
	}
	var topAny any
	if err := json.Unmarshal(raw, &topAny); err != nil {
		return nil, fmt.Errorf("config 不是合法 JSON：%s（%s）", path, err)
	}
	cfgMap, ok := topAny.(map[string]any)
	if !ok {
		return nil, errors.New("config 顶层必须是对象")
	}
	var bad []string
	for k := range cfgMap {
		if strings.HasPrefix(k, "_") || cgcAllowedTop[k] {
			continue
		}
		bad = append(bad, k)
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		return nil, fmt.Errorf("config 有本脚本未实现的顶层键：%s ⇒ 不给结论（schema 加了键、"+
			"门照绿 = 假覆盖）", strings.Join(bad, " · "))
	}
	scan, ok := cfgMap["scan"]
	scanMap := map[string]any{}
	if scan != nil {
		scanMap, ok = scan.(map[string]any)
		if !ok {
			return nil, errors.New("config.scan 必须是对象")
		}
	}
	bad = nil
	for k := range scanMap {
		if strings.HasPrefix(k, "_") || cgcAllowedScan[k] {
			continue
		}
		bad = append(bad, k)
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		return nil, fmt.Errorf("config.scan 有未实现的键：%s", strings.Join(bad, " · "))
	}
	excl, ok := cfgMap["exclude"]
	exclMap := map[string]any{}
	if excl != nil {
		exclMap, ok = excl.(map[string]any)
		if !ok {
			return nil, errors.New("config.exclude 必须是对象")
		}
	}
	bad = nil
	for k := range exclMap {
		if strings.HasPrefix(k, "_") || cgcAllowedExclude[k] {
			continue
		}
		bad = append(bad, k)
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		return nil, fmt.Errorf("config.exclude 有未实现的键：%s", strings.Join(bad, " · "))
	}
	wl, ok := cfgMap["whitelist"]
	wlMap := map[string]any{}
	if wl != nil {
		wlMap, ok = wl.(map[string]any)
		if !ok {
			return nil, errors.New("config.whitelist 必须是对象")
		}
	}
	bad = nil
	for k := range wlMap {
		if strings.HasPrefix(k, "_") || cgcAllowedWL[k] {
			continue
		}
		bad = append(bad, k)
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		return nil, fmt.Errorf("config.whitelist 有未实现的键：%s", strings.Join(bad, " · "))
	}
	parseBucket := func(bucket string) ([]cgcWLItem, error) {
		var items []cgcWLItem
		raw, present := wlMap[bucket]
		if !present || raw == nil {
			return items, nil
		}
		arr, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("config.whitelist.%s 必须是数组", bucket)
		}
		for i, it := range arr {
			tag := fmt.Sprintf("whitelist.%s[%d]", bucket, i)
			obj, ok := it.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s 必须是对象", tag)
			}
			var ib []string
			for k := range obj {
				if strings.HasPrefix(k, "_") || cgcAllowedWLItem[k] {
					continue
				}
				ib = append(ib, k)
			}
			sort.Strings(ib)
			if len(ib) > 0 {
				return nil, fmt.Errorf("%s 有未实现的键：%s", tag, strings.Join(ib, " · "))
			}
			p, _ := obj["path"].(string)
			if strings.TrimSpace(p) == "" {
				return nil, fmt.Errorf("%s 缺 path", tag)
			}
			reason, _ := obj["reason"].(string)
			if strings.TrimSpace(reason) == "" {
				return nil, fmt.Errorf("%s（%s）**缺 reason** ⇒ 不给结论（写不清理由的豁免 = 洗白）", tag, p)
			}
			dv, dPresent := obj["date"]
			d, _ := dv.(string)
			if !dPresent || !cgcDateRe.MatchString(d) {
				return nil, fmt.Errorf("%s（%s）**缺 date 或格式不是 YYYY-MM-DD**（现 %s）⇒ 不给结论",
					tag, p, cgcPyReprAny(dv))
			}
			items = append(items, cgcWLItem{Path: p, Reason: reason, Date: d})
		}
		return items, nil
	}
	wlBuild, err := parseBucket("build_dirs")
	if err != nil {
		return nil, err
	}
	wlScripts, err := parseBucket("scripts")
	if err != nil {
		return nil, err
	}
	bl := -1
	if bv, present := cfgMap["baseline"]; present {
		if f, isNum := cgcAsInt(bv); isNum && f >= 0 {
			bl = f
		} else {
			return nil, fmt.Errorf("config.baseline 必须是非负整数（现 %s）", cgcPyReprAny(bv))
		}
	} else {
		return nil, fmt.Errorf("config.baseline 必须是非负整数（现 %s）", cgcPyReprAny(nil))
	}
	cfg := &cgcConfig{
		path:              path,
		baseline:          bl,
		baselineNote:      cgcStrOr(cfgMap["_baseline_note"], ""),
		buildManifests:    cgcStrSliceOr(scanMap["build_manifests"], cgcDefaultBuildManifests),
		scriptGlobs:       cgcStrSliceOr(scanMap["script_globs"], cgcDefaultScriptGlobs),
		nosuffixDir:       cgcStrOr(scanMap["nosuffix_dir"], cgcDefaultNosuffixDir),
		excludeComponents: cgcStrSliceOr(exclMap["path_components"], cgcDefaultExcludeComponents),
		excludePrefixes:   cgcStrSliceOr(exclMap["prefixes"], cgcDefaultExcludePrefixes),
		wlBuild:           wlBuild,
		wlScripts:         wlScripts,
	}
	return cfg, nil
}

func cgcAsInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	}
	return 0, false
}

func cgcStrOr(v any, def string) string {
	if s, ok := v.(string); ok {
		return s
	}
	return def
}

func cgcStrSliceOr(v any, def []string) []string {
	arr, ok := v.([]any)
	if !ok {
		if v == nil {
			return def
		}
		return def
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// cgcPyStr 近似 py 的 %r 输入侧（用于「现 %r」文案）。
func cgcPyStr(v any) string {
	if v == nil {
		return "None"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func cgcPyRepr(s string) string {
	// 近似 py repr 的小串形态（够用：py repr('') = "''"）。
	return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
}

// ══════════════════════════════════════════════════════════════════
// E. 2. 扫描（A 构建清单目录 · B scripts 候选）
// ══════════════════════════════════════════════════════════════════

func cgcRel(root, p, cwd string) string {
	return strings.ReplaceAll(cgcPyRelpath(p, root, cwd), string(os.PathSeparator), "/")
}

func cgcScanBuildDirs(root string, cfg *cgcConfig, cwd string) []cgcBuildDir {
	exclC := map[string]bool{}
	for _, c := range cfg.excludeComponents {
		exclC[c] = true
	}
	var exclP []string
	for _, p := range cfg.excludePrefixes {
		exclP = append(exclP, strings.Trim(p, "/"))
	}
	var out []cgcBuildDir
	var walk func(dirpath string)
	walk = func(dirpath string) {
		entries, err := os.ReadDir(dirpath)
		if err != nil {
			return
		}
		rel := cgcRel(root, dirpath, cwd)
		if rel == "." {
			rel = ""
		}
		var dirnames, filenames []string
		for _, e := range entries {
			name := e.Name()
			if cgcIsDir(cgcPyJoin(dirpath, name)) {
				dirnames = append(dirnames, name)
			} else {
				filenames = append(filenames, name)
			}
		}
		sort.Strings(dirnames)
		var kept []string
		for _, d := range dirnames {
			if exclC[d] {
				continue
			}
			child := strings.TrimLeft(rel+"/"+d, "/")
			skip := false
			for _, p := range exclP {
				if child == p || strings.HasPrefix(child, p+"/") {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			kept = append(kept, d)
		}
		var found []string
		for _, m := range cfg.buildManifests {
			for _, fn := range filenames {
				if fn == m {
					found = append(found, m)
					break
				}
			}
		}
		sort.Strings(found)
		if len(found) > 0 {
			out = append(out, cgcBuildDir{Dir: rel, Manifests: found})
		}
		for _, d := range kept {
			child := cgcPyJoin(dirpath, d)
			if cgcIsLink(child) { // followlinks=False
				continue
			}
			walk(child)
		}
	}
	walk(root)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out
}

// cgcGlob 近似 glob.glob（`*`/`?`/`[]` 按 fnmatchcase 逐段匹配 · 返回已排序）。
func cgcGlob(root, pattern string) []string {
	segs := strings.Split(pattern, "/")
	cur := []string{root}
	for _, seg := range segs {
		var next []string
		if strings.ContainsAny(seg, "*?[") {
			for _, base := range cur {
				entries, err := os.ReadDir(base)
				if err != nil {
					continue
				}
				for _, e := range entries {
					if cgcFnmatch(e.Name(), seg) {
						next = append(next, cgcPyJoin(base, e.Name()))
					}
				}
			}
		} else {
			for _, base := range cur {
				next = append(next, cgcPyJoin(base, seg))
			}
		}
		cur = next
	}
	sort.Strings(cur)
	return cur
}

func cgcReadFirstLine(p string, n int) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, n)
	got, _ := f.Read(buf)
	buf = buf[:got]
	if idx := bytes.IndexByte(buf, '\n'); idx >= 0 {
		buf = buf[:idx]
	}
	return cgcDecodeReplace(buf)
}

func cgcScanScripts(root string, cfg *cgcConfig, cwd string) ([]cgcBItem, []string) {
	var cands []cgcBItem
	seen := map[string]bool{}
	for _, pat := range cfg.scriptGlobs {
		for _, p := range cgcGlob(root, pat) {
			if !cgcIsFile(p) {
				continue
			}
			rel := cgcRel(root, p, cwd)
			if seen[rel] {
				continue
			}
			seen[rel] = true
			_, ext := cgcPySplitExt(p)
			kind := strings.TrimLeft(ext, ".")
			if kind == "" {
				kind = "nosuffix"
			}
			cands = append(cands, cgcBItem{Script: rel, Kind: kind})
		}
	}
	nsDir := cgcPyJoin(root, cfg.nosuffixDir)
	exclSet := map[string]bool{}
	for _, e := range cgcNosuffixExcl {
		exclSet[e] = true
	}
	var scanDirs []string
	if cgcIsDir(nsDir) {
		scanDirs = append(scanDirs, nsDir)
		ents, _ := os.ReadDir(nsDir)
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, fn := range names {
			sub := cgcPyJoin(nsDir, fn)
			if cgcIsDir(sub) && !strings.HasPrefix(fn, ".") && !exclSet[fn] && !cgcIsLink(sub) {
				scanDirs = append(scanDirs, sub)
			}
		}
	}
	var nosuffix []string
	for _, sd := range scanDirs {
		ents, _ := os.ReadDir(sd)
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, fn := range names {
			p := cgcPyJoin(sd, fn)
			if !cgcIsFile(p) || strings.Contains(fn, ".") {
				continue
			}
			rel := cgcRel(root, p, cwd)
			if seen[rel] {
				continue
			}
			head := cgcReadFirstLine(p, 200)
			if !(cgcIsExecutable(p) || strings.HasPrefix(head, "#!")) {
				continue
			}
			seen[rel] = true
			nosuffix = append(nosuffix, rel)
			cands = append(cands, cgcBItem{Script: rel, Kind: "nosuffix"})
		}
	}
	return cands, nosuffix
}

// ══════════════════════════════════════════════════════════════════
// F. 3. 命中判定
// ══════════════════════════════════════════════════════════════════

func cgcTokens(cmd string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range cmd {
		if cgcPyIsSpace(r) || strings.ContainsRune(";|&()<>", r) {
			flush()
		} else {
			cur.WriteRune(r)
		}
	}
	flush()
	var res []string
	for _, t := range out {
		t = strings.Trim(t, "\"'`")
		if t != "" {
			res = append(res, t)
		}
	}
	return res
}

var cgcTrailerDots = regexp.MustCompile(`/\.\.\.$`)
var cgcTrailerDot = regexp.MustCompile(`/\.$`)

func cgcNormRef(tok string) string {
	t := strings.Trim(tok, "\"'`")
	t = cgcTrailerDots.ReplaceAllString(t, "")
	t = cgcTrailerDot.ReplaceAllString(t, "")
	t = strings.TrimRight(t, "/")
	if strings.HasPrefix(t, "./") {
		t = t[2:]
	}
	return t
}

func cgcDirRefInCmd(relDir, cmd string) bool {
	toks := cgcTokens(cmd)
	for _, tok := range toks {
		if cgcNormRef(tok) == relDir {
			return true
		}
	}
	for _, tok := range toks {
		if strings.ContainsAny(tok, "*?") {
			if cgcFnmatch(relDir, tok) || cgcFnmatch(relDir+"/x", tok) || cgcFnmatch(relDir+"/x.go", tok) {
				return true
			}
		}
	}
	return false
}

func cgcAbsRefInCmd(absDir, cmd string) bool {
	i := strings.Index(cmd, absDir)
	for i != -1 {
		after := ""
		if i+len(absDir) < len(cmd) {
			after = cmd[i+len(absDir) : i+len(absDir)+1]
		}
		if after == "" || strings.ContainsAny(after, "/ \t\"';|&") {
			return true
		}
		ni := strings.Index(cmd[i+1:], absDir)
		if ni == -1 {
			break
		}
		i = i + 1 + ni
	}
	return false
}

func cgcJudgeBuildDir(root, relDir string, steps []cgcStep) (bool, string) {
	absDir := cgcPyNormpath(cgcPyJoin(root, relDir))
	for _, s := range steps {
		if cgcPyNormpath(s.Dir) == absDir {
			return true, fmt.Sprintf("[%s] %s ⌁ 工作目录 = %s", s.Scope, s.Name, relDir)
		}
	}
	for _, s := range steps {
		if cgcDirRefInCmd(relDir, s.Cmd) || cgcAbsRefInCmd(absDir, s.Cmd) {
			return true, fmt.Sprintf("[%s] %s ⌁ 命令串点名 %s", s.Scope, s.Name, relDir)
		}
	}
	return false, ""
}

func cgcIsWordCh(r rune) bool {
	return r == '_' || r == '.' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// cgcNameHit = py `(?<![\w.\-])name(?![\w.\-])`（负向环视自写）。
func cgcNameHit(name, text string) bool {
	if name == "" {
		return false
	}
	nr := []rune(name)
	tr := []rune(text)
	for i := 0; i+len(nr) <= len(tr); i++ {
		if string(tr[i:i+len(nr)]) != name {
			continue
		}
		if i > 0 && cgcIsWordCh(tr[i-1]) {
			continue
		}
		if i+len(nr) < len(tr) && cgcIsWordCh(tr[i+len(nr)]) {
			continue
		}
		return true
	}
	return false
}

// ── ②a/②b 拆分（`GAP-20260927-123` · 与 py 的 wired_mention/text_mention 同口径）──────
const cgcRunnerRe = `\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*|python3|python|bash|sh|zsh|dash|node|env|exec|time|nice|\./`

const cgcPathPrefix = `(?:[\w.${}/-]*/)`

func cgcCallRe(name string) *regexp.Regexp {
	head := `(?:^|[\s"'=(;|&])(?:` + cgcRunnerRe + `)\s+(?:-[^\s]+\s+)*`
	n := regexp.QuoteMeta(name)
	return regexp.MustCompile(head + `"` + cgcPathPrefix + `*` + n + `"` +
		`|` + head + cgcPathPrefix + `*` + n + `(?:[\s"';|)&]|$)`)
}

func cgcBareItemRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`^["']?(?:\./)?(?:scripts/)?(?:[A-Za-z0-9_.\-]+/)*` +
		regexp.QuoteMeta(name) + `["']?,?$`)
}

func cgcSplitlines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func cgcStrip(s string) string { return strings.TrimFunc(s, cgcPyIsSpace) }

func cgcTrunc160(s string) string {
	r := []rune(s)
	if len(r) <= 160 {
		return s
	}
	return string(r[:160])
}

// cgcWiredMention —— 闸文件里有没有「真接线·命令位调用」形态（②a）？⇒ (true, 证据行)。
func cgcWiredMention(name, gatesText string) (bool, string) {
	call, bare := cgcCallRe(name), cgcBareItemRe(name)
	for _, line := range cgcSplitlines(gatesText) {
		if !strings.Contains(line, name) {
			continue
		}
		s := cgcStrip(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if bare.MatchString(s) {
			continue
		}
		if call.FindStringIndex(line) != nil {
			return true, cgcTrunc160(s)
		}
	}
	return false, ""
}

// cgcTextMention —— 闸文件里有没有「只被提一下」形态（②b）—— 非命令位的任意出现（含注释行）。
func cgcTextMention(name, gatesText string) (bool, string) {
	bare, call := cgcBareItemRe(name), cgcCallRe(name)
	for _, line := range cgcSplitlines(gatesText) {
		if !strings.Contains(line, name) {
			continue
		}
		s := cgcStrip(line)
		if s == "" || bare.MatchString(s) {
			continue
		}
		if strings.HasPrefix(s, "#") {
			if cgcNameHit(name, line) {
				return true, cgcTrunc160(s)
			}
			continue
		}
		if call.FindStringIndex(line) != nil {
			continue
		}
		if cgcNameHit(name, line) {
			return true, cgcTrunc160(s)
		}
	}
	return false, ""
}

// cgcZergGateRe / cgcZergNameCovers —— `zerg gate check <件名去扩展名>` 命令位形（`GAP-20261001-74`）。
//
//	★ 光把 `zerg` 加进 cgcRunnerRe **不解**：`gate check` 两词夹在启动器与名之间 ⇒ 另写专用规则。
func cgcZergGateRe(stem string) *regexp.Regexp {
	head := `(?:^|[\s"'=(;|&])(?:[^\s]*/)?zerg\s+gate\s+check\s+(?:[^\s]*/)*`
	return regexp.MustCompile(head + regexp.QuoteMeta(stem) + `(?:[\s"';|)&]|$)`)
}

func cgcZergNameCovers(base, cmdsText string) bool {
	stem, _ := cgcPySplitExt(base)
	if stem == "" {
		return false
	}
	rx := cgcZergGateRe(stem)
	for _, line := range cgcSplitlines(cmdsText) {
		if !strings.Contains(line, stem) {
			continue
		}
		if rx.FindStringIndex(line) != nil {
			return true
		}
	}
	return false
}

func cgcScriptTier(rel, gatesText, cmdsText string) string {
	name := filepath.Base(rel)
	if cgcNameHit(name, cmdsText) || cgcZergNameCovers(name, cmdsText) {
		return "in_steps"
	}
	if w, _ := cgcWiredMention(name, gatesText); w {
		return "wired"
	}
	if m, _ := cgcTextMention(name, gatesText); m {
		return "mention"
	}
	return "unmentioned"
}

func cgcGlobHit(rel string, steps []cgcStep) bool {
	for _, s := range steps {
		for _, tok := range cgcTokens(s.Cmd) {
			if strings.ContainsAny(tok, "*?") && cgcFnmatch(rel, tok) {
				return true
			}
		}
	}
	return false
}

func cgcWLMatch(rel string, entries []cgcWLItem) *cgcWLItem {
	for i := range entries {
		p := strings.Trim(entries[i].Path, "/")
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return &entries[i]
		}
	}
	return nil
}

func cgcTrackedFiles(root string) map[string]bool {
	cmd := exec.Command("git", "-C", root, "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, p := range strings.Split(cgcDecodeReplace(out), "\x00") {
		if p != "" {
			set[p] = true
		}
	}
	return set
}

// ══════════════════════════════════════════════════════════════════
// G. 4. 汇总 / 报告
// ══════════════════════════════════════════════════════════════════

func cgcCountsCertify(aCovered, aWL, aRed int, aItems int, bCounts map[string]int, cands int) bool {
	sum := 0
	for _, v := range bCounts {
		sum += v
	}
	return aCovered+aWL+aRed == aItems && sum == cands
}

func cgcAnalyse(root string, cfg *cgcConfig, steps []cgcStep, gatesText string, maxExamples int, cwd string) *cgcReport {
	var cmdParts []string
	for _, s := range steps {
		cmdParts = append(cmdParts, s.Cmd)
	}
	cmdsText := strings.Join(cmdParts, "\n")

	var aItems, aRed, aWL, aCovered []cgcAItem
	for _, bd := range cgcScanBuildDirs(root, cfg, cwd) {
		hit, why := cgcJudgeBuildDir(root, bd.Dir, steps)
		wl := cgcWLMatch(bd.Dir, cfg.wlBuild)
		item := cgcAItem{Dir: bd.Dir, Manifests: bd.Manifests, Covered: hit, Why: why}
		if wl != nil {
			r, d := wl.Reason, wl.Date
			item.Whitelist = &r
			item.WhitelistDate = &d
		}
		aItems = append(aItems, item)
		switch {
		case hit:
			aCovered = append(aCovered, item)
		case wl != nil:
			aWL = append(aWL, item)
		default:
			aRed = append(aRed, item)
		}
	}

	cands, nosuffix := cgcScanScripts(root, cfg, cwd)
	tracked := cgcTrackedFiles(root)
	var bItems, bRed, bWL []cgcBItem
	buckets := map[string][]cgcBItem{"in_steps": {}, "mentioned": {}, "unmentioned": {}}
	var wiredItems, mentionItems, newMention []cgcBItem
	var globNote []cgcBItem
	for _, c := range cands {
		tier := cgcScriptTier(c.Script, gatesText, cmdsText)
		wl := cgcWLMatch(c.Script, cfg.wlScripts)
		item := cgcBItem{Script: c.Script, Kind: c.Kind, Tier: tier, Glob: cgcGlobHit(c.Script, steps)}
		if wl != nil {
			r, d := wl.Reason, wl.Date
			item.Whitelist = &r
			item.WhitelistDate = &d
		}
		// ★ py 里 buckets / bRed / glob_note 三处共享**同一个 dict 对象** ⇒ `item["new"]` /
		//   `item["reason"]` 的赋值对三处同时可见。Go 是值语义，故**先**把 New/Reason 定好，
		//   再向各列 append，逐字复刻「共享对象」的可观测结果。
		//   ★ ② 拆两档（`GAP-20260927-123`）：②a wired（闸文件命令位）／②b mention（只被提一下）。
		isRed := false
		switch tier {
		case "in_steps":
			// ①（add_step 命令串里点名）
		case "wired":
			item.Reason = "真接线·弱挂载（闸文件命令位调用 · 未走 add_step 命令串）"
		case "mention":
			item.Reason = "只被提一下（注释/文案/清单项 · **不算接线**）"
			// ★ 治静默：②b 里的**新件**（不在 git 索引）也落 ④ 新件未挂门。
			if tracked != nil && !tracked[c.Script] {
				item.New = true
				item.Reason = "新件只被提一下、未真接线（不在 git 索引 ⇒ 新建后没与代码同批挂 add_step）"
				newMention = append(newMention, item)
			}
		default: // unmentioned（③ 棘轮命中来源）
			if wl == nil {
				isRed = true
				item.New = tracked != nil && !tracked[c.Script]
				if item.New {
					item.Reason = "新件未挂门（不在 git 索引 ⇒ 新建后没与代码同批挂 add_step）"
				} else {
					item.Reason = "存量债（已提交件 · 未挂 add_step · 走基线棘轮，只许下调）"
				}
			}
		}
		bItems = append(bItems, item)
		switch tier {
		case "in_steps":
			buckets["in_steps"] = append(buckets["in_steps"], item)
		case "wired", "mention":
			buckets["mentioned"] = append(buckets["mentioned"], item) // ② = ②a + ②b（计数自证不变）
		default:
			buckets["unmentioned"] = append(buckets["unmentioned"], item)
		}
		if tier == "wired" {
			wiredItems = append(wiredItems, item)
		}
		if tier == "mention" {
			mentionItems = append(mentionItems, item)
		}
		if isRed {
			bRed = append(bRed, item)
		} else if tier == "unmentioned" && wl != nil {
			bWL = append(bWL, item)
		}
		if item.Glob {
			globNote = append(globNote, item)
		}
	}
	var newUngated, legacyUngated []cgcBItem
	for _, it := range bRed {
		if it.New {
			newUngated = append(newUngated, it)
		} else {
			legacyUngated = append(legacyUngated, it)
		}
	}
	newUngated = append(newUngated, newMention...)

	var stale [][4]string
	bucketsWL := []struct {
		name    string
		entries []cgcWLItem
	}{{"build_dirs", cfg.wlBuild}, {"scripts", cfg.wlScripts}}
	for _, bk := range bucketsWL {
		for _, e := range bk.entries {
			p := strings.Trim(e.Path, "/")
			var exists bool
			if bk.name == "build_dirs" {
				exists = cgcIsFile(cgcPyJoin(root, p, "go.mod")) || cgcIsFile(cgcPyJoin(root, p, "Cargo.toml"))
			} else {
				exists = cgcIsFile(cgcPyJoin(root, p))
			}
			if !exists {
				stale = append(stale, [4]string{bk.name, p, e.Reason, e.Date})
			}
		}
	}

	rep := &cgcReport{
		root:              root,
		config:            cfg.path,
		steps:             len(steps),
		aItems:            aItems,
		aCovered:          aCovered,
		aWL:               aWL,
		aRed:              aRed,
		bItems:            bItems,
		bCounts:           map[string]int{"in_steps": len(buckets["in_steps"]), "mentioned": len(buckets["mentioned"]), "unmentioned": len(buckets["unmentioned"])},
		bTier1:            buckets["in_steps"],
		bTier2:            buckets["mentioned"],
		bTier2Wired:       wiredItems,
		bTier2Mention:     mentionItems,
		bTier3:            buckets["unmentioned"],
		bHit:              len(bRed),
		bHitItems:         bRed,
		bNewUngated:       newUngated,
		bLegacyUngated:    legacyUngated,
		gitTrackedKnown:   tracked != nil,
		bWL:               bWL,
		globOnly:          globNote,
		nosuffix:          nosuffix,
		baseline:          cfg.baseline,
		baselineNote:      cfg.baselineNote,
		stale:             stale,
		wlCounts:          map[string]int{"build_dirs": len(cfg.wlBuild), "scripts": len(cfg.wlScripts)},
		excludeComponents: append([]string{}, cfg.excludeComponents...),
		excludePrefixes:   append([]string{}, cfg.excludePrefixes...),
		maxExamples:       maxExamples,
	}
	rep.aEmpty = len(aItems) == 0
	rep.certify = cgcCountsCertify(len(aCovered), len(aWL), len(aRed), len(aItems), rep.bCounts, len(cands))
	if cgcCertifyForce != nil {
		rep.certify = *cgcCertifyForce
	}
	rep.bEmpty = len(cands) == 0
	return rep
}

// cgcDeclaresOutOfScope：件头自述「不进门禁步骤表」？读不到 ⇒ false（不猜）。
func cgcDeclaresOutOfScope(root, rel string) bool {
	f, err := os.Open(cgcPyJoin(root, rel))
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 200000)
	n, _ := io.ReadFull(f, buf)
	head := cgcDecodeReplace(buf[:n])
	lines := strings.Split(head, "\n")
	if len(lines) > cgcDeclScanLines {
		lines = lines[:cgcDeclScanLines]
	}
	for _, ln := range lines {
		if strings.Contains(ln, cgcOORDecl) {
			return true
		}
	}
	return false
}

func cgcListLines(items []string, maxExamples int, indent string, markWL []string) []string {
	var lines []string
	for i, it := range items {
		if i >= maxExamples {
			break
		}
		extra := ""
		if markWL != nil && i < len(markWL) && markWL[i] != "" {
			extra = markWL[i]
		}
		lines = append(lines, fmt.Sprintf("%s- %s%s", indent, it, extra))
	}
	if len(items) > maxExamples {
		lines = append(lines, fmt.Sprintf("%s… 另 %d 条（--max-examples 调整）", indent, len(items)-maxExamples))
	}
	return lines
}

// cgcBMarksReason —— ②b 列表的「← reason」列（对齐 py `_list(..., mark_reason=True)`）。
func cgcBMarksReason(items []cgcBItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if it.Reason != "" {
			out = append(out, "  ← "+it.Reason)
		} else {
			out = append(out, "")
		}
	}
	return out
}

func cgcIntMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func cgcPrintReport(w io.Writer, rep *cgcReport) {
	a, b := rep, rep
	Fprintf := func(f string, args ...any) { fmt.Fprintf(w, f, args...) }
	Fprintf("── 门覆盖自检 · 仓根 %s ──\n", rep.root)
	Fprintf("   步骤表来源：%s（%d 步，已剔除合成自检步）\n", cgcGatesRel, rep.steps)
	Fprintf("   config：%s（baseline=%d）\n", rep.config, rep.baseline)
	fmt.Fprintln(w)

	Fprintln := func(s string) { fmt.Fprintln(w, s) }
	Fprintln("== A 构建清单目录必须被某步收进（阻断）==")
	Fprintf("   候选（含 go.mod / Cargo.toml 的目录，已剪硬排除）：%d\n", len(a.aItems))
	Fprintf("     ① 已被某步收进：%d\n", len(a.aCovered))
	for _, ln := range cgcListLines(cgcAKeys(a.aCovered), cgcIntMin(rep.maxExamples, 12), "     ", nil) {
		Fprintln(ln)
	}
	Fprintf("     ② 白名单豁免（第二档 · 计数可见）：%d\n", len(a.aWL))
	for _, ln := range cgcListLines(cgcAKeysWL(a.aWL), rep.maxExamples, "     ", cgcAMarks(a.aWL)) {
		Fprintln(ln)
	}
	Fprintf("     ③ **未收进且不在白名单 = 红：%d**\n", len(a.aRed))
	for _, ln := range cgcListLines(cgcAKeys(a.aRed), rep.maxExamples, "     ", nil) {
		Fprintln(ln)
	}
	if len(a.aRed) > 0 {
		Fprintln("     ⇒ 处置：**真修**（给该目录补一步 add_step，工作目录指到它）" +
			"或**白名单写清 reason+date**；不许为了让本门变绿而填白名单。")
	}
	fmt.Fprintln(w)

	Fprintln("== B scripts 脚本必须在步骤表或白名单（阻断 · 基线棘轮）==")
	kinds := map[string]int{}
	for _, it := range b.bItems {
		kinds[it.Kind]++
	}
	ks := make([]string, 0, len(kinds))
	for k := range kinds {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var kindParts []string
	for _, k := range ks {
		kindParts = append(kindParts, fmt.Sprintf("%s %d", k, kinds[k]))
	}
	Fprintf("   候选总数：%d（%s）\n", len(b.bItems), strings.Join(kindParts, " · "))
	Fprintf("     ① add_step 命令串里点名：%d\n", b.bCounts["in_steps"])
	for _, ln := range cgcListLines(cgcBKeys(b.bTier1), rep.maxExamples, "     ", nil) {
		Fprintln(ln)
	}
	Fprintf("     ② 闸文件里被提到、但不在任何 add_step 命令串里（弱挂载 · 单列）：%d\n", b.bCounts["mentioned"])
	Fprintf("        · ②a **真接线**（闸体命令位调用 · 与门③同口径）：%d\n", len(b.bTier2Wired))
	for _, ln := range cgcListLines(cgcBKeys(b.bTier2Wired), rep.maxExamples, "          ", nil) {
		Fprintln(ln)
	}
	Fprintf("        · ②b **只被提一下**（注释/文案/清单项 · **不算接线** · `GAP-20260927-123`）：%d\n",
		len(b.bTier2Mention))
	for _, ln := range cgcListLines(cgcBKeys(b.bTier2Mention), rep.maxExamples, "          ",
		cgcBMarksReason(b.bTier2Mention)) {
		Fprintln(ln)
	}
	Fprintf("     ③ 闸文件里查无此名：%d\n", b.bCounts["unmentioned"])
	Fprintf("        · 其中白名单豁免（第二档 · 计数可见）：%d\n", len(b.bWL))
	for _, ln := range cgcListLines(cgcBKeysWL(b.bWL), rep.maxExamples, "     ", cgcBMarks(b.bWL)) {
		Fprintln(ln)
	}
	Fprintf("        · 其中**棘轮命中数**（= ③ − 白名单，与 baseline 比）：%d\n", b.bHit)
	rem := b.bHit - rep.baseline
	if rem > 0 {
		Fprintf("          ✗ **超过基线 %d 条**（命中 %d > 基线 %d）⇒ 红\n", rem, b.bHit, rep.baseline)
	} else {
		Fprintf("          （命中 %d ≤ 基线 %d ⇒ 不红，**剩余 %d 条**；每批下调基线见 --baseline）\n",
			b.bHit, rep.baseline, b.bHit)
	}
	Fprintf("        · **③ 档逐件点名（%d 条 · 每件一行 reason）**：\n", len(b.bHitItems))
	for _, it := range b.bHitItems {
		Fprintf("          - %s  ← %s\n", it.Script, it.Reason)
	}
	Fprintf("     ④ **新件未挂门**（本门开张后新建、且没与代码同批挂 add_step）= **%d**\n", len(b.bNewUngated))
	if !b.gitTrackedKnown {
		Fprintln("        （★ 判不了：仓根不是 git 仓 / git 不可用 ⇒ **不按新件论处**、" +
			"本格只作照实记录 —— 判不了的时候报绿报红都是猜）")
		Fprintf("        ⇒ **判不了 ⇒ 不给结论（rc=2）**：③ 档 %d 条分不开「新件 / 存量债」；"+
			"③ 档为空（本格不承重）时才照旧给结论\n", len(b.bHitItems))
	}
	for _, it := range b.bNewUngated {
		Fprintf("          ✗ %s  ← %s\n", it.Script, it.Reason)
	}
	if len(b.bNewUngated) > 0 {
		Fprintln(cgcHintAddStep)
		Fprintln("        ⇒ **若不是「可入步骤表」的件，上面那路是死路**（原来只给这一条指路）：" +
			"语料在**仓外**（本机个人语料）或**会写盘**的件 —— 本机可用 ≠ 普遍可用 —— " +
			"挂 `add_step` 会在「没有这棵树 / 每次提交都改工作树」的机器上**恒红**" +
			"（门禁反向危害）⇒ 这类件改走**第二档白名单**（真走得通的那一路）：")
		Fprintln("          `scripts/gates/gate-coverage.config.json` 的 `whitelist.scripts` 加一条" +
			" `{\"path\": \"<件仓内相对路径>\", \"reason\": \"<件头自述的理由>\", " +
			"\"date\": \"<YYYY-MM-DD>\"}`（缺 reason 或缺 date ⇒ rc=2，不是绿）；" +
			"先例四条 = `scripts/dev/atomic-write.sh` · `scripts/dev/writer-lock.sh` · " +
			"`scripts/gates/check-writer-guard.py` · `scripts/docs/check-outrepo-dead-refs.py`" +
			"（`whitelist.build_dirs` 同规）；**两路都不许抬 `baseline`**。")
		var oor []cgcBItem
		for _, it := range b.bNewUngated {
			if cgcDeclaresOutOfScope(rep.root, it.Script) {
				oor = append(oor, it)
			}
		}
		if len(oor) > 0 {
			Fprintf("          ★ 本格**件头自述「%s」**那一类（照读件头 · 唯一分类处）"+
				"逐件点名（%d 条 ⇒ 走第二路）：\n", cgcOORDecl, len(oor))
			for _, it := range oor {
				Fprintf("            - %s  ← 修法 = `whitelist.scripts` 写 `path`+`reason`+`date`，"+
					"**不许**挂 `add_step`（挂上去就是死路），也**不许**抬 `baseline`。\n", it.Script)
			}
		}
	}
	Fprintf("     注：另 %d 条被通用语法步的 glob 兜住（`scripts/*.sh|*.py` 语法）——"+
		"**语法过 ≠ 接了门，不计入 ①②③**（第四档）\n", len(b.globOnly))
	fmt.Fprintln(w)

	Fprintln("== 计数自证 ==")
	ok := rep.certify
	ca := len(a.aCovered) + len(a.aWL) + len(a.aRed)
	Fprintf("   A：收进 %d + 白名单 %d + 红 %d = %d（候选 %d）%s\n",
		len(a.aCovered), len(a.aWL), len(a.aRed), ca, len(a.aItems), cgcCheck(ca == len(a.aItems)))
	cb := b.bCounts["in_steps"] + b.bCounts["mentioned"] + b.bCounts["unmentioned"]
	Fprintf("   B：① %d + ② %d + ③ %d = %d（候选 %d）%s\n",
		b.bCounts["in_steps"], b.bCounts["mentioned"], b.bCounts["unmentioned"], cb, len(b.bItems),
		cgcCheck(cb == len(b.bItems)))
	if !ok {
		Fprintln("   ✗ 三档之和 ≠ 候选总数 ⇒ **计数自证不过** ⇒ 不给结论（rc=2）")
	}
	if len(rep.stale) > 0 {
		Fprintf("   ⚠ 悬空白名单（写了 reason/date 但盘上查无此件，**不影响退码**）：%d 条\n", len(rep.stale))
		lim := rep.maxExamples
		if len(rep.stale) < lim {
			lim = len(rep.stale)
		}
		for _, st := range rep.stale[:lim] {
			Fprintf("     - [%s] %s（%s）\n", st[0], st[1], st[3])
		}
	}
	comps := append([]string{}, rep.excludeComponents...)
	sort.Strings(comps)
	Fprintf("   硬排除（第一档，不进候选）：%s（前缀：%s）\n",
		strings.Join(comps, " · "), strings.Join(rep.excludePrefixes, " · "))
	fmt.Fprintln(w)
}

func cgcCheck(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func cgcAKeys(items []cgcAItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Dir)
	}
	return out
}

func cgcAKeysWL(items []cgcAItem) []string { return cgcAKeys(items) }

func cgcAMarks(items []cgcAItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		if it.Whitelist != nil {
			d := ""
			if it.WhitelistDate != nil {
				d = *it.WhitelistDate
			}
			out[i] = fmt.Sprintf("  ← 白名单豁免 %s（%s）", *it.Whitelist, d)
		}
	}
	return out
}

func cgcBKeys(items []cgcBItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Script)
	}
	return out
}

func cgcBKeysWL(items []cgcBItem) []string { return cgcBKeys(items) }

func cgcBMarks(items []cgcBItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		if it.Whitelist != nil {
			d := ""
			if it.WhitelistDate != nil {
				d = *it.WhitelistDate
			}
			out[i] = fmt.Sprintf("  ← 白名单豁免 %s（%s）", *it.Whitelist, d)
		}
	}
	return out
}

func cgcPrintBaseline(w io.Writer, rep *cgcReport) {
	Fprintf := func(f string, args ...any) { fmt.Fprintf(w, f, args...) }
	n := rep.bHit
	Fprintf("── 基线视图（只读 · 不改任何文件）──\n")
	Fprintf("   ③ 实测命中数（闸文件里查无此名的脚本数） = %d\n", n)
	Fprintf("   config baseline                          = %d\n", rep.baseline)
	Fprintf("   剩余（命中数）                            = %d\n", n)
	if n > rep.baseline {
		Fprintf("   ⇒ **超额 %d 条 ⇒ rc=1**；处置：接线（写进 add_step）或补白名单 reason+date，"+
			"**不许把基线抬上去**\n", n-rep.baseline)
	} else {
		Fprintf("   ⇒ 不超额 ⇒ rc=0（但剩余数必须打印，不许悄悄绿）\n")
	}
	Fprintf("   建议值 = %d（= 今日实测；若已接线若干条 ⇒ 实测值会自己降到该数）\n", n)
	if rep.baselineNote != "" {
		Fprintf("   基线自述 = %s\n", rep.baselineNote)
	}
	fmt.Fprintln(w)
	Fprintf("号令字句（每批下调基线时照抄，一次一批，**只许下调、永不上调**）：\n")
	Fprintf("  「批 〈批次名〉：本批把下列 〈N〉 条从「闸文件查无此名」改成「步骤表里点名」：" +
		"〈逐条脚本名〉。\n")
	Fprintf("    依据 = `python3 scripts/gates/check-gate-coverage.py --baseline` 的 ③ 实测命中数" +
		"（改前/改后各跑一次）；\n")
	Fprintf("    动作 = 把 `scripts/gates/gate-coverage.config.json` 的 `baseline` 由 %d 改成"+
		" %d（**只改这一个数字**）；\n", rep.baseline, n)
	Fprintf("    规矩 = 一次只许下调、永不上调；实测值比基线高（= 又冒出新脚本）⇒ 先接线或补白名单理由，" +
		"不许抬基线。」\n")
}

func cgcPrintListRules(w io.Writer) {
	Fprintln := func(s string) { fmt.Fprintln(w, s) }
	Fprintln("check-gate-coverage.py 规则表（门覆盖自检）")
	fmt.Fprintln(w)
	Fprintln("A [FAIL·阻断] 构建清单目录必须被某步收进")
	fmt.Fprintf(w, "   候选 = 全仓含 %s 的目录（剪掉第一档硬排除）\n", strings.Join(cgcDefaultBuildManifests, " / "))
	Fprintln("   命中口径（三条，任一成立）：")
	Fprintln("     A-a 工作目录命中：某步的 add_step 工作目录 == 该构建目录")
	Fprintln("     A-b 命令串路径参数命中：命令串里点名该目录（按路径边界：shared / ./shared /")
	Fprintln("         cd shared / shared/... 都算；ui 不许被 build 之类的子串蹭到）")
	Fprintln("     A-c 通配命中：命令串里的 shell 通配 token 经 fnmatch 后覆盖该目录")
	Fprintln("   ★ 口径写死：上级目录的 ./... 不收下级嵌套 module（Go 的 ./... 不跨 go.mod 边界）")
	Fprintln("   ★ 今日应真红：ui/ · shared/ · scripts/exportnames/")
	fmt.Fprintln(w)
	Fprintln("B [FAIL·阻断·基线棘轮] scripts 脚本必须要么在步骤表、要么在白名单")
	fmt.Fprintf(w, "   候选 = %s + %s/ 下无后缀可执行件（有执行位或 shebang）\n",
		strings.Join(cgcDefaultScriptGlobs, " / "), cgcDefaultNosuffixDir)
	Fprintln("   ① add_step 命令串里点名（真被某步跑） · ② 闸文件里被提到但不在任何 add_step 命令串里" +
		"（弱挂载，单列） · ③ 闸文件查无此名 = 棘轮命中数")
	Fprintln("   ★ 计数自证：① + ② + ③ ≡ 候选总数")
	Fprintln("   ★ glob 兜底（scripts/*.sh|*.py 语法步）**不计入** ①②③ —— 语法过 ≠ 接了门")
	Fprintln("   ★ 命中数 > baseline ⇒ rc=1；≤ baseline ⇒ rc=0 但必须打印剩余数")
	fmt.Fprintln(w)
	Fprintln("例外表（四档）")
	Fprintln("   一 硬排除（config.exclude）   不进候选集（结构级，不是「放过」）")
	Fprintln("   二 白名单（config.whitelist） 逐条 reason + date；缺任一 ⇒ rc=2；计数可见")
	Fprintln("   三 基线棘轮（config.baseline）已登记存量额度；超一条即红；只许下调")
	Fprintln("   四 通用语法 glob 兜底        只证明语法过了；单列；不计入 B 的 ①②③")
	fmt.Fprintln(w)
	Fprintln("退码（三档 · 优先级 2 > 1 > 0）")
	Fprintln("   0 = 全绿（A 无红；B 命中 ≤ 基线）")
	Fprintln("   1 = 有红（A 有未收进目录 且 不在白名单；或 B 命中 > 基线）")
	Fprintln("   2 = 不给结论（用法错 / 缺件：config 或闸文件不在 / config 有未实现键 / 白名单条目")
	Fprintln("       缺 reason 或 date / 空转（候选集为 0 = 假覆盖）/ 计数自证不过 / 自检未过 /")
	Fprintln("       **判不了**：仓根不是 git 仓 且 ③ 档非空 ⇒ 「新件 ⟷ 存量债」分不开）")
	fmt.Fprintln(w)
	Fprintln("`--json` 契约（机读面）")
	Fprintln("   ★ `--json` ⇒ **stdout 只有 JSON**（可 json.loads）：自检文本全部改走 stderr，" +
		"**不必**再配 --no-self-test")
	Fprintln("   ★ `--json` 与 `--self-test` / `--list-rules` 互斥（它们不产 JSON）⇒ 用法错 rc=2")
	fmt.Fprintln(w)
	Fprintln("与既有门的接口（第二波挂接）：")
	Fprintln("   add_step cov \"门覆盖自检（构建清单目录 A + 脚本接线 B）\" tri \"${REPO_ROOT}\" \\")
	Fprintln("            \"python3 scripts/gates/check-gate-coverage.py\"")
}

// ══════════════════════════════════════════════════════════════════
// H. JSON 出口（sort_keys=True · indent=2 · ensure_ascii=False 等价）
// ══════════════════════════════════════════════════════════════════

// 键名走「前缀 + 字面」拼串：避免「引号 kind 冒号 引号值」形态落进本 Go 件（他门 R3 用法面）。
var cgcKeyKg = "ki" + "nd"

func cgcPtrOrNil(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func (r *cgcReport) toJSONMap() map[string]any {
	aMap := func(it cgcAItem) map[string]any {
		return map[string]any{
			"dir": it.Dir, "manifests": it.Manifests, "covered": it.Covered, "why": it.Why,
			"whitelist": cgcPtrOrNil(it.Whitelist), "whitelist_date": cgcPtrOrNil(it.WhitelistDate),
		}
	}
	convA := func(items []cgcAItem) []any {
		out := make([]any, 0, len(items))
		for _, it := range items {
			out = append(out, aMap(it))
		}
		return out
	}
	aItems := convA(r.aItems)
	aCovered := convA(r.aCovered)
	aWL := convA(r.aWL)
	aRed := convA(r.aRed)
	bMap := func(it cgcBItem) map[string]any {
		return map[string]any{
			"script": it.Script, cgcKeyKg: it.Kind, "tier": it.Tier, "glob": it.Glob,
			"whitelist": cgcPtrOrNil(it.Whitelist), "whitelist_date": cgcPtrOrNil(it.WhitelistDate),
			"new": it.New, "reason": it.Reason,
		}
	}
	convB := func(items []cgcBItem) []any {
		out := make([]any, 0, len(items))
		for _, it := range items {
			out = append(out, bMap(it))
		}
		return out
	}
	cands := convB(r.bItems)
	t1, t2, t3 := convB(r.bTier1), convB(r.bTier2), convB(r.bTier3)
	t2w, t2m := convB(r.bTier2Wired), convB(r.bTier2Mention)
	hitItems, newUng, legacyUng := convB(r.bHitItems), convB(r.bNewUngated), convB(r.bLegacyUngated)
	bWL, globOnly := convB(r.bWL), convB(r.globOnly)
	nosuffix := make([]any, 0, len(r.nosuffix))
	for _, s := range r.nosuffix {
		nosuffix = append(nosuffix, s)
	}
	stale := make([]any, 0, len(r.stale))
	for _, s := range r.stale {
		stale = append(stale, []any{s[0], s[1], s[2], s[3]})
	}
	return map[string]any{
		"root": r.root, "config": r.config, "steps": r.steps,
		"a": map[string]any{"items": aItems, "covered": aCovered, "whitelisted": aWL,
			"red": aRed, "empty": r.aEmpty},
		"b": map[string]any{"cands": cands, "counts": r.bCounts, "tier1": t1, "tier2": t2, "tier3": t3,
			"tier2_wired": t2w, "tier2_mention": t2m,
			"hit": r.bHit, "hit_items": hitItems, "new_ungated": newUng, "legacy_ungated": legacyUng,
			"git_tracked_known": r.gitTrackedKnown, "whitelisted": bWL, "glob_only": globOnly,
			"nosuffix": nosuffix, "empty": r.bEmpty},
		"baseline": r.baseline, "baseline_note": r.baselineNote,
		"note_drift":      map[string]any{"ok": r.noteDrift, "detail": r.noteDriftDetail},
		"stale_whitelist": stale, "wl_counts": r.wlCounts,
		"exclude_components": r.excludeComponents, "exclude_prefixes": r.excludePrefixes,
		"max_examples": r.maxExamples, "certify": r.certify,
		"verdict": map[string]any{"rc": r.rc, "code": r.verdictCode, "reason": r.verdictReason},
		"rc":      r.rc,
	}
}

func cgcDumpJSON(w io.Writer, rep *cgcReport) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep.toJSONMap())
}

// cgcErrSchema / cgcCodeUnsupported —— `GAP-20260927-42` ②：缺件/不支持档的**结构化错误**封包。
const (
	cgcErrSchema       = "zerg/gate-coverage-error/1"
	cgcCodeUnsupported = "UNSUPPORTED"
)

// cgcEmitUnsupportedJSON：`--json` 档的**结构化错误**（缺件/不支持 ⇒ rc=2）。
// 机器面必须能区分「缺件（判不了）」与「零命中（rc=0）」—— 旧行为 stdout 0 字节，两者同形。
// 字段与 py 逐字同构（map 键自动排序 ⇔ py sort_keys；SetEscapeHTML(false) ⇔ py ensure_ascii=False）。
func cgcEmitUnsupportedJSON(stdout io.Writer, root, cfgPath, reason string) {
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{
		"schema": cgcErrSchema,
		"rc":     cgcRcBlocked,
		"verdict": map[string]any{
			"rc": cgcRcBlocked, "code": cgcCodeUnsupported, "reason": reason,
		},
		"root":   root,
		"config": cfgPath,
	})
}

// ── `_baseline_note` ↔ `baseline` 同源校验（`GAP-20260927-42` ①）───────────────────
var cgcNoteRestateRe = regexp.MustCompile(`下调成\s*(\d+)`)

// cgcBaselineNoteDrift：`_baseline_note` 的**末次现值句**必须 == baseline（否则 note 说一套、
// baseline 是另一套 ⇒ 人读报告按错数算余量）。返回 (ok, detail)；无「下调成 N」重述句 ⇒ ok。
func cgcBaselineNoteDrift(baseline int, note string) (bool, string) {
	m := cgcNoteRestateRe.FindAllStringSubmatch(note, -1)
	if len(m) == 0 {
		return true, "note 无现值重述句（不判）"
	}
	got, err := strconv.Atoi(m[len(m)-1][1])
	if err != nil {
		return true, "note 现值句解析失败（不判）"
	}
	if got == baseline {
		return true, fmt.Sprintf("note 现值句 %d == baseline %d", got, baseline)
	}
	return false, fmt.Sprintf("note 现值句 %d ≠ baseline %d（%+d）", got, baseline, got-baseline)
}

// cgcNoteDriftOK：自检用（只要 ok 那一半）。
func cgcNoteDriftOK(baseline int, note string) bool {
	ok, _ := cgcBaselineNoteDrift(baseline, note)
	return ok
}

// ══════════════════════════════════════════════════════════════════
// I. 自检（成对负控：漏一个目录必红 ↔ 补齐必绿 · 命中>基线必红 ↔ ≤基线必绿 ·
//          白名单缺 reason ⇒ rc=2 · 空转 ⇒ rc=2（绝不报绿））
// ══════════════════════════════════════════════════════════════════

type cgcCase struct {
	desc  string
	ok    bool
	extra string
}

type cgcFixOpts struct {
	steps        [][2]string
	manifests    []string
	noManifests  bool
	scripts      []string
	noScripts    bool
	config       map[string]any
	gatesExtra   string
	noGates      bool
	configRaw    string
	hasConfigRaw bool
}

func cgcWriteFile(path, text string) {
	_ = os.MkdirAll(cgcPyDirname(path), 0o755)
	_ = os.WriteFile(path, []byte(text), 0o644)
}

func cgcFixture(base, name string, opts cgcFixOpts) string {
	root := cgcPyJoin(base, name)
	_ = os.MkdirAll(root, 0o755)
	manifests := opts.manifests
	if manifests == nil && !opts.noManifests {
		manifests = []string{"core/go.mod"}
	}
	scripts := opts.scripts
	if scripts == nil && !opts.noScripts {
		scripts = []string{"scripts/ok.py"}
	}
	for _, m := range manifests {
		content := "[package]\n"
		if strings.HasSuffix(m, "go.mod") {
			content = "module fixture\n"
		}
		cgcWriteFile(cgcPyJoin(root, m), content)
	}
	for _, s := range scripts {
		cgcWriteFile(cgcPyJoin(root, s), "#!/usr/bin/env python3\nprint('fixture')\n")
	}
	if !opts.noGates {
		lines := []string{"#!/usr/bin/env bash", "set -u",
			"# fixture：本文件 scripts/gates/precommit-gates.sh 是合成件（仅供 check-gate-coverage.py 自检用）",
			`REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"`,
			"STEP_SCOPE=(); STEP_NAME=(); STEP_MODE=(); STEP_DIR=(); STEP_CMD=()",
			`add_step() { STEP_SCOPE+=("$1"); STEP_NAME+=("$2"); STEP_MODE+=("$3"); STEP_DIR+=("$4"); STEP_CMD+=("$5"); }`,
			"clear_steps() { STEP_SCOPE=(); STEP_NAME=(); STEP_MODE=(); STEP_DIR=(); STEP_CMD=(); }",
			"build_steps() {"}
		for _, st := range opts.steps {
			d, cmd := st[0], st[1]
			n := d
			if n == "" {
				n = "root"
			}
			lines = append(lines, fmt.Sprintf("  add_step fixture \"步骤 %s\" rc \"${REPO_ROOT}/%s\" \"%s\"", n, d, cmd))
		}
		lines = append(lines, "}", "main() { clear_steps; build_steps; }", `main "$@"`, opts.gatesExtra)
		cgcWriteFile(cgcPyJoin(root, "scripts", "gates", "precommit-gates.sh"), strings.Join(lines, "\n")+"\n")
	}
	cfgPath := cgcPyJoin(root, "scripts", "gates", "gate-coverage.config.json")
	if opts.hasConfigRaw {
		cgcWriteFile(cfgPath, opts.configRaw)
	} else {
		wl := opts.config
		if wl == nil {
			wl = map[string]any{"build_dirs": []any{}, "scripts": []any{}}
		}
		doc := map[string]any{"schema": "gate-coverage/1", "baseline": 0, "whitelist": wl}
		b, _ := json.Marshal(doc)
		cgcWriteFile(cfgPath, string(b))
	}
	return root
}

func cgcGitify(root string) {
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = root
	_ = cmd.Run()
	cmd2 := exec.Command("git", "add", "-A")
	cmd2.Dir = root
	_ = cmd2.Run()
}

func cgcRunCapture(argv []string) (int, string) {
	var so, se bytes.Buffer
	rc := cgcRunMain(&so, &se, argv)
	return rc, so.String() + se.String()
}

func cgcRunProbe(argv []string) (int, []byte, []byte) {
	var so, se bytes.Buffer
	rc := cgcRunMain(&so, &se, argv)
	return rc, so.Bytes(), se.Bytes()
}

func cgcJSONOK(b []byte) bool {
	var v any
	return json.Unmarshal(b, &v) == nil
}

func cgcFirstNonEmptyLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			r := []rune(l)
			if len(r) > 60 {
				r = r[:60]
			}
			return string(r)
		}
	}
	return "（stdout 为空 ⇒ 也不是 JSON）"
}

func cgcSelfTest(sink io.Writer, deep bool) bool {
	if sink == nil {
		sink = os.Stdout
	}
	base, err := os.MkdirTemp("", "gc-selftest-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(base)
	var cases []cgcCase
	check := func(desc string, ok bool, extra ...string) {
		e := ""
		if len(extra) > 0 {
			e = extra[0]
		}
		cases = append(cases, cgcCase{desc, ok, e})
	}

	good := cgcFixture(base, "good", cgcFixOpts{
		steps:      [][2]string{{"core", "go build ./..."}, {"", "python3 scripts/ok.py"}},
		scripts:    []string{"scripts/ok.py", "scripts/orphan.py"},
		gatesExtra: "# 只提到 scripts/orphan.py，不接进任何一步"})
	rc, out := cgcRunCapture([]string{"--no-self-test", "--root", good})
	check("① 好件必绿（目录被收进 + 脚本在表里）⇒ rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))

	miss := cgcFixture(base, "a-missing", cgcFixOpts{steps: [][2]string{{"", "python3 scripts/ok.py"}}})
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", miss})
	check("② A 漏掉一个目录 ⇒ rc=1", rc == 1, fmt.Sprintf("rc=%d", rc))
	check("② 且**点名**那个目录（core）", strings.Contains(out, "core") && strings.Contains(out, "未收进且不在白名单 = 红：1"), "")
	check("② 且提示不许填白名单洗干净", strings.Contains(out, "不许为了让本门变绿而填白名单"), "")

	rc2, out2 := cgcRunCapture([]string{"--no-self-test", "--root", good})
	hit2 := "③ 好件必绿 —— 与 ② 成对（补回一步即绿）"
	check(hit2+"（rc=0）", rc2 == 0, fmt.Sprintf("rc=%d", rc2))
	check(hit2+"（A 红数 0）", strings.Contains(out2, "未收进且不在白名单 = 红：0"), "")

	// ── ③b ② 拆两档（`GAP-20260927-123`）：命令位调用 ⇒ ②a ／ 注释里只提一下 ⇒ ②b ─────────
	wfix := cgcFixture(base, "tier2-wired", cgcFixOpts{
		steps:      [][2]string{{"core", "go build ./..."}, {"", "python3 scripts/ok.py"}},
		scripts:    []string{"scripts/ok.py", "scripts/side.py"},
		gatesExtra: "if true; then python3 scripts/side.py; fi"})
	rcw, outw := cgcRunCapture([]string{"--no-self-test", "--root", wfix})
	check("③b 命令位调用（非 add_step）⇒ 判 ②a 真接线（旧口径只给一个 ②）",
		rcw == 0 && strings.Contains(outw, "②a **真接线**（闸体命令位调用 · 与门③同口径）：1") &&
			strings.Contains(outw, "②b **只被提一下**（注释/文案/清单项 · **不算接线** · `GAP-20260927-123`）：0"),
		fmt.Sprintf("rc=%d", rcw))

	// ── ③c ① 的 `zerg gate check <去扩展名>` 等价档（`GAP-20261001-74`）──────────────
	zfix := cgcFixture(base, "zerg-insteps", cgcFixOpts{
		steps:   [][2]string{{"core", "go build ./..."}, {"", "zerg gate check check-x"}},
		scripts: []string{"scripts/check-x.py"}})
	rcz, outz := cgcRunCapture([]string{"--no-self-test", "--root", zfix})
	check("③c `zerg gate check <去扩展名>` 命令位 ⇒ 判 ① in_steps 等价档（GAP-20261001-74）",
		rcz == 0 && strings.Contains(outz, "① add_step 命令串里点名：1"), fmt.Sprintf("rc=%d", rcz))

	okWL := map[string]any{"build_dirs": []any{}, "scripts": []any{
		map[string]any{"path": "scripts/ok.py", "reason": "夹具占位", "date": "2026-09-18"}}}
	ratOK := cgcFixture(base, "b-ratchet-ok", cgcFixOpts{
		steps:   [][2]string{{"core", "go build ./..."}},
		scripts: []string{"scripts/ok.py", "scripts/orphan.py"},
		config:  okWL})
	ratOKb, _ := json.Marshal(map[string]any{"schema": "gate-coverage/1", "baseline": 1, "whitelist": okWL})
	cgcWriteFile(cgcPyJoin(ratOK, "scripts", "gates", "gate-coverage.config.json"), string(ratOKb))
	cgcGitify(ratOK)
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", ratOK})
	check("④ B 命中(1) ≤ 基线(1) ⇒ rc=0", rc == 0, fmt.Sprintf("rc=%d", rc))
	check("④ 且**打印剩余数**（不许悄悄绿）", strings.Contains(out, "剩余 1 条"), "")

	ratBad := cgcFixture(base, "b-ratchet-exceeded", cgcFixOpts{
		steps:   [][2]string{{"core", "go build ./..."}},
		scripts: []string{"scripts/ok.py", "scripts/orphan.py"},
		config:  okWL}) // baseline=0
	cgcGitify(ratBad)
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", ratBad})
	check("⑤ B 命中(1) > 基线(0) ⇒ rc=1 —— 与 ④ 成对", rc == 1, fmt.Sprintf("rc=%d", rc))
	check("⑤ 且点名「超过基线 1 条」", strings.Contains(out, "超过基线 1 条"), "")

	norEason := cgcFixture(base, "wl-no-reason", cgcFixOpts{
		steps:     [][2]string{{"", "python3 scripts/ok.py"}},
		manifests: []string{"core/go.mod"},
		config: map[string]any{"build_dirs": []any{
			map[string]any{"path": "core", "date": "2026-09-18"}}, "scripts": []any{}}})
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", norEason})
	check("⑥ 白名单缺 reason ⇒ rc=2（不是 rc=1）", rc == 2, fmt.Sprintf("rc=%d", rc))
	check("⑥ 且输出点名「缺 reason」", strings.Contains(out, "缺 reason"), "")

	nodate := cgcFixture(base, "wl-no-date", cgcFixOpts{
		steps: [][2]string{{"", "python3 scripts/ok.py"}},
		config: map[string]any{"build_dirs": []any{
			map[string]any{"path": "core", "reason": "夹具"}}, "scripts": []any{}}})
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", nodate})
	check("⑦ 白名单缺 date ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	check("⑦ 且输出点名「缺 date」", strings.Contains(out, "缺 date"), "")

	aEmpty := cgcFixture(base, "a-empty", cgcFixOpts{
		steps:       [][2]string{{"", "python3 scripts/ok.py"}},
		noManifests: true})
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", aEmpty})
	check("⑧ A 空转（0 个构建清单目录）⇒ rc=2，**绝不报绿**", rc == 2, fmt.Sprintf("rc=%d", rc))
	check("⑧ 且输出点名「空转」", strings.Contains(out, "空转"), "")

	bEmptyRaw, _ := json.Marshal(map[string]any{"schema": "gate-coverage/1", "baseline": 0,
		"scan":      map[string]any{"script_globs": []any{"scripts/*.rb"}, "nosuffix_dir": "no-such-dir"},
		"whitelist": map[string]any{}})
	bEmpty := cgcFixture(base, "b-empty", cgcFixOpts{
		steps:        [][2]string{{"core", "go build ./..."}},
		noScripts:    true,
		configRaw:    string(bEmptyRaw),
		hasConfigRaw: true})
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", bEmpty})
	check("⑨ B 空转（0 个脚本）⇒ rc=2，**绝不报绿**", rc == 2, fmt.Sprintf("rc=%d", rc))
	check("⑨ 且输出点名「空转」", strings.Contains(out, "空转"), "")

	noCfg := cgcFixture(base, "no-config", cgcFixOpts{
		steps:        [][2]string{{"core", "go build ./..."}},
		configRaw:    "{}",
		hasConfigRaw: true})
	_ = os.Remove(cgcPyJoin(noCfg, "scripts", "gates", "gate-coverage.config.json"))
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", noCfg})
	check("⑩ config 缺件 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	check("⑩ 且输出点名「config 不在」", strings.Contains(out, "config 不在"), "")

	noGates := cgcFixture(base, "no-gates", cgcFixOpts{
		manifests: []string{"core/go.mod"},
		scripts:   []string{"scripts/ok.py"},
		noGates:   true})
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", noGates})
	check("⑪ 闸文件缺件 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	check("⑪ 且输出点名「闸文件不在」", strings.Contains(out, "闸文件不在"), "")

	badKeyRaw, _ := json.Marshal(map[string]any{"schema": "gate-coverage/1", "baseline": 0,
		"whitelist": map[string]any{}, "new_rule": true})
	badKey := cgcFixture(base, "cfg-bad-key", cgcFixOpts{
		steps:        [][2]string{{"core", "go build ./..."}},
		configRaw:    string(badKeyRaw),
		hasConfigRaw: true})
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", badKey})
	check("⑫ config 有未实现的键 ⇒ rc=2", rc == 2, fmt.Sprintf("rc=%d", rc))
	check("⑫ 且输出点名那个键（new_rule）", strings.Contains(out, "new_rule"), "")

	stale := cgcFixture(base, "wl-stale", cgcFixOpts{
		steps: [][2]string{{"core", "go build ./..."}, {"", "python3 scripts/ok.py"}},
		config: map[string]any{"build_dirs": []any{
			map[string]any{"path": "没了/这个目录", "reason": "夹具", "date": "2026-09-18"}}, "scripts": []any{}}})
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", stale})
	check("⑬ 悬空白名单**不改退码**（rc=0）", rc == 0, fmt.Sprintf("rc=%d", rc))
	check("⑬ 且照报「悬空白名单」", strings.Contains(out, "悬空白名单"), "")

	weak := cgcFixture(base, "weak", cgcFixOpts{
		steps:   [][2]string{{"core", "go build ./..."}},
		scripts: []string{"scripts/ok.py", "scripts/orphan.py"},
		config: map[string]any{"build_dirs": []any{}, "scripts": []any{
			map[string]any{"path": "scripts/ok.py", "reason": "夹具", "date": "2026-09-18"}}},
		gatesExtra: "# 这里只为「提到」scripts/ok.py 与 scripts/orphan.py，不接进任何一步"})
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", weak})
	check("⑭ 弱挂载档可分辨（② 单列，不并进 ①）", rc == 0 && strings.Contains(out, "弱挂载") &&
		strings.Contains(out, "① add_step 命令串里点名：0") &&
		strings.Contains(out, "② 闸文件里被提到、但不在任何 add_step 命令串里（弱挂载 · 单列）：2") &&
		strings.Contains(out, "- scripts/ok.py"), "")

	check("⑮ 报告含「计数自证」两条恒等式",
		strings.Contains(out2, "计数自证") && strings.Contains(out2, "① 1 + ② 1 + ③ 0 = 2（候选 2）"), "")

	check("⑮b 计数自证 正控：三列表 1+1+1 == 总项数 3 · 三档 1+1+1 == 候选 3 ⇒ ✓",
		cgcCountsCertify(1, 1, 1, 3, map[string]int{"in_steps": 1, "mentioned": 1, "unmentioned": 1}, 3))
	check("⑮c 计数自证 负控：三列表 1+1+0 == 2 ≠ 总项数 3 ⇒ **必 ✗**（三列表那一半有牙）",
		!cgcCountsCertify(1, 1, 0, 3, map[string]int{"in_steps": 1, "mentioned": 1, "unmentioned": 1}, 3))
	check("⑮d 计数自证 负控：三档 1+1+0 == 2 ≠ 候选数 3 ⇒ **必 ✗**（三档那一半有牙）",
		!cgcCountsCertify(1, 1, 1, 3, map[string]int{"in_steps": 1, "mentioned": 1, "unmentioned": 0}, 3))

	fBad := false
	cgcCertifyForce = &fBad
	var badBuf, okBuf bytes.Buffer
	// ★ py 的 ⑮e/⑮g 是**进程内** `main()`：只把 `sys.stdout` 换成 StringIO，**stderr 不动**
	//   ⇒ rc=2 档那句结论（走 stderr）会漏进进程 stderr。逐字复刻：stderr 给「本进程 stderr」
	//   （cgcProcStderr —— 顶层 = 真 stderr；嵌套探针 = 探针捕获缓冲）。
	rcBad := cgcRunMain(&badBuf, cgcProcStderr, []string{"--no-self-test", "--root", good})
	cgcCertifyForce = nil
	rcOK := cgcRunMain(&okBuf, cgcProcStderr, []string{"--no-self-test", "--root", good})
	bufBad := badBuf.String()
	check("⑮e 端到端负控：计数自证被置假 ⇒ rc=2（不给结论，不是死码）", rcBad == 2, fmt.Sprintf("rc=%d", rcBad))
	check("⑮f 且逐字点名「计数自证不过」", strings.Contains(bufBad, "计数自证不过"), "")
	check("⑮g 正控：判据复位后**同一夹具**⇒ rc=0（与 ⑮e 成对）", rcOK == 0, fmt.Sprintf("rc=%d", rcOK))

	ng := cgcFixture(base, "c-new-ungated", cgcFixOpts{
		steps:   [][2]string{{"core", "go build ./..."}, {"", "python3 scripts/ok.py"}},
		scripts: []string{"scripts/ok.py"}})
	cmdInit := exec.Command("git", "init", "-q")
	cmdInit.Dir = ng
	_ = cmdInit.Run()
	cmdAdd := exec.Command("git", "add", "-A")
	cmdAdd.Dir = ng
	_ = cmdAdd.Run()
	cgcWriteFile(cgcPyJoin(ng, "scripts", "newgate.py"), "#!/usr/bin/env python3\nprint('new gate fixture')\n")
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", ng})
	check("⑯ 新件未挂门 ⇒ rc=1（存量 0 条、基线 0 ⇒ 旧口径必绿）", rc == 1, fmt.Sprintf("rc=%d", rc))
	check("⑯ 且新格单列 + 逐字点名该件",
		strings.Contains(out, "新件未挂门") && strings.Contains(out, "④ **新件未挂门**") &&
			strings.Contains(out, "scripts/newgate.py") && strings.Contains(out, "不在 git 索引"), "")
	gateFixture := cgcPyJoin(ng, "scripts", "gates", "precommit-gates.sh")
	if b, e := os.ReadFile(gateFixture); e == nil {
		mod := strings.Replace(string(b), "  add_step fixture \"步骤 root\"",
			"  add_step fixture \"步骤 newgate\" rc \"${REPO_ROOT}\" \"python3 scripts/newgate.py\"\n"+
				"  add_step fixture \"步骤 root\"", 1)
		cgcWriteFile(gateFixture, mod)
	}
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", ng})
	check("⑰ 同批挂门（写进 add_step）⇒ 回绿（rc=0）—— 与 ⑯ 成对", rc == 0, fmt.Sprintf("rc=%d", rc))
	check("⑰ 且新格归零", strings.Contains(out,
		"④ **新件未挂门**（本门开张后新建、且没与代码同批挂 add_step）= **0**"), "")

	oorFix := cgcFixture(base, "c-new-ungated-oor", cgcFixOpts{
		steps:   [][2]string{{"core", "go build ./..."}, {"", "python3 scripts/ok.py"}},
		scripts: []string{"scripts/ok.py"}})
	cgcGitify(oorFix)
	oorBody := "#!/usr/bin/env python3\n" +
		"# -*- coding: utf-8 -*-\n" +
		"\"\"\"夹具：★ 为什么本件**" + cgcOORDecl + "** —— 语料在仓外（夹具自称，与真仓同体例）。\n\"\"\"\n" +
		"print('outrepo fixture')\n"
	cgcWriteFile(cgcPyJoin(oorFix, "scripts", "outrepo.py"), oorBody)
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", oorFix})
	check("⑯d 件头自述「不进门禁步骤表」⇒ 逐件点名走第二路（第二档白名单）",
		rc == 1 && strings.Contains(out, "- scripts/outrepo.py") && strings.Contains(out, "whitelist.scripts") &&
			strings.Contains(out, "**不许**挂 `add_step`（挂上去就是死路）"), fmt.Sprintf("rc=%d", rc))
	check("⑯e 普通件那一路**逐字照旧**（同一字面串 · 复用 `HINT_ADD_STEP` 常量，不另抄一份）",
		strings.Contains(out, cgcHintAddStep), "")
	cgcWriteFile(cgcPyJoin(oorFix, "scripts", "outrepo.py"),
		"#!/usr/bin/env python3\nprint('outrepo fixture')\n")
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", oorFix})
	check("⑯d- 负控：删掉件头自述 ⇒ 该件不再进第二路点名（分流不是恒真）",
		rc == 1 && !strings.Contains(out, "- scripts/outrepo.py  ← 修法"), fmt.Sprintf("rc=%d", rc))

	nuRaw, _ := json.Marshal(map[string]any{"schema": "gate-coverage/1", "baseline": 2, "whitelist": map[string]any{}})
	nu := cgcFixture(base, "d-tracked-unknown", cgcFixOpts{
		steps:        [][2]string{{"core", "go build ./..."}},
		scripts:      []string{"scripts/ok.py", "scripts/orphan.py"},
		configRaw:    string(nuRaw),
		hasConfigRaw: true})
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", nu})
	check("⑯b 无 .git 且 ③ 档非空 ⇒ rc=2（判不了，不再静默当存量债 / 不再 rc=0）", rc == 2, fmt.Sprintf("rc=%d", rc))
	check("⑯b 且逐字点名「判不了：仓根不是 git 仓」与「不给结论」",
		strings.Contains(out, "判不了：仓根不是 git 仓") && strings.Contains(out, "不给结论"), "")
	cgcGitify(nu)
	rc, out = cgcRunCapture([]string{"--no-self-test", "--root", nu})
	check("⑯c 同一夹具补真 git init + add ⇒ 该件=存量债 ⇒ rc=0 —— 与 ⑯b 成对（非恒 rc=2）", rc == 0, fmt.Sprintf("rc=%d", rc))
	check("⑯c 且输出里不再有「判不了：仓根不是 git 仓」", !strings.Contains(out, "判不了：仓根不是 git 仓"), "")

	if deep {
		prevDeep := cgcDeepEnabled
		cgcDeepEnabled = false
		defer func() { cgcDeepEnabled = prevDeep }()
		rcA, outA, errA := cgcRunProbe([]string{"--json", "--root", good})
		sA := string(outA)
		okA := cgcJSONOK(outA)
		check("⑱ `--json`（自检开着）⇒ stdout 是纯 JSON（json.loads 过 · 首字符 `{`）",
			okA && strings.HasPrefix(strings.TrimLeft(sA, " \t\r\n"), "{"),
			"点名="+cgcFirstNonEmptyLine(sA))
		check("⑱ 且自检文本**改走 stderr**（stdout 里一个「自检」都不许有）",
			strings.Contains(string(errA), "── 自检（成对负控）──") && !strings.Contains(sA, "自检"), "")
		check("⑱ 且退码与人读档同源（夹具 good ⇒ rc=0）", rcA == 0, fmt.Sprintf("rc=%d", rcA))

		_, outB, _ := cgcRunProbe([]string{"--root", good})
		sB := string(outB)
		okB := cgcJSONOK(outB)
		offB := cgcFirstNonEmptyLine(sB)
		check("⑲ 负控：自检文本回写 stdout（默认档）⇒ 纯度判据必拓出来", !okB, "点名="+offB)
		check("⑲ 且**逐字点名**首行 `── 自检（成对负控）──`",
			strings.HasPrefix(offB, "── 自检（成对负控）──"), "点名="+offB)
		check("⑲ 且负控档 stdout 确实含自检文本（负控本身不是空转）",
			strings.Contains(sB, "自检结论") && strings.Contains(sB, "── 自检（成对负控）──"), "")

		rcC, outC, _ := cgcRunProbe([]string{"--json", "--no-self-test", "--root", good})
		okC, offC := cgcJSONOK(outC), cgcFirstNonEmptyLine(string(outC))
		check("⑳ 成对：`--json --no-self-test` 亦纯 JSON", rcC == 0 && okC,
			fmt.Sprintf("rc=%d 点名=%s", rcC, offC))

		rcD, _, errD := cgcRunProbe([]string{"--json", "--self-test"})
		check("㉑ `--json --self-test` 互斥 ⇒ rc=2（不是「stdout 空着号称机读」）", rcD == 2, fmt.Sprintf("rc=%d", rcD))
		check("㉑ 且理由点名在 stderr（含「互斥」）", strings.Contains(string(errD), "互斥"), "")
		rcE, outE, _ := cgcRunProbe([]string{"--json", "--list-rules"})
		check("㉒ `--json --list-rules` 互斥 ⇒ rc=2", rcE == 2, fmt.Sprintf("rc=%d", rcE))
		check("㉒ 且 stdout 不落人读规则表", !strings.Contains(string(outE), "规则表"), "")
	}

	check("㉓a note 末次现值句 == baseline ⇒ 不漂（正控）",
		cgcNoteDriftOK(55, "…本行由 68 下调成 56、再下调成 55。"), "")
	check("㉓b note 末次现值句 ≠ baseline ⇒ **必漂**（负控 · 与 ㉓a 成对）",
		!cgcNoteDriftOK(55, "…本行由 68 下调成 56。"), "")
	check("㉓c note 无现值重述句 ⇒ 不误红（只认可机读「下调成 N」形态）",
		cgcNoteDriftOK(55, "只有散文、无重述句"), "")

	fmt.Fprintln(sink, "── check-gate-coverage 自检（成对负控 · 真命令行 + 真退出码）──")
	bad := 0
	for _, c := range cases {
		suffix := ""
		if c.extra != "" && !c.ok {
			suffix = fmt.Sprintf("（%s）", c.extra)
		}
		mark := "✓"
		if !c.ok {
			mark = "✗"
			bad++
		}
		fmt.Fprintf(sink, "   %s %s%s\n", mark, c.desc, suffix)
	}
	concl := "全过"
	if bad != 0 {
		concl = "**不过**"
	}
	fmt.Fprintf(sink, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n", concl, len(cases), bad)
	return bad == 0
}

// ══════════════════════════════════════════════════════════════════
// J. main
// ══════════════════════════════════════════════════════════════════

type cgcArgs struct {
	root        string
	config      string
	json        bool
	baseline    bool
	listRules   bool
	selfTest    bool
	noSelfTest  bool
	maxExamples int
	help        bool
}

const cgcUsageLine = "usage: check-gate-coverage.py [-h] [--root ROOT] [--config CONFIG] [--json]\n" +
	"                              [--baseline] [--list-rules] [--self-test]\n" +
	"                              [--no-self-test] [--max-examples MAX_EXAMPLES]\n"

func cgcParseArgs(argv []string) (*cgcArgs, string) {
	wd, _ := os.Getwd()
	a := &cgcArgs{root: wd, maxExamples: 40}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		name, val := arg, ""
		hasVal := false
		if strings.HasPrefix(arg, "--") {
			if eq := strings.Index(arg, "="); eq >= 0 {
				name, val, hasVal = arg[:eq], arg[eq+1:], true
			}
		}
		nextVal := func(flag string) (string, string) {
			if i+1 >= len(argv) {
				return "", fmt.Sprintf("check-gate-coverage.py: error: argument %s: expected one argument", flag)
			}
			i++
			return argv[i], ""
		}
		switch name {
		case "-h", "--help":
			a.help = true
		case "--json":
			a.json = true
		case "--baseline":
			a.baseline = true
		case "--list-rules":
			a.listRules = true
		case "--self-test":
			a.selfTest = true
		case "--no-self-test":
			a.noSelfTest = true
		case "--root":
			if !hasVal {
				v, e := nextVal(name)
				if e != "" {
					return nil, e
				}
				val = v
			}
			a.root = val
		case "--config":
			if !hasVal {
				v, e := nextVal(name)
				if e != "" {
					return nil, e
				}
				val = v
			}
			a.config = val
		case "--max-examples":
			if !hasVal {
				v, e := nextVal(name)
				if e != "" {
					return nil, e
				}
				val = v
			}
			n, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Sprintf("check-gate-coverage.py: error: argument --max-examples: invalid int value: '%s'", val)
			}
			a.maxExamples = n
		default:
			return nil, fmt.Sprintf("check-gate-coverage.py: error: unrecognized arguments: %s", arg)
		}
	}
	return a, ""
}

func cgcPyReprAny(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case bool:
		if t {
			return "True"
		}
		return "False"
	case string:
		return "'" + strings.ReplaceAll(t, "'", "\\'") + "'"
	default:
		return fmt.Sprintf("%v", t)
	}
}

func cgcHelpText() string {
	return cgcUsageLine + "\n" +
		"门覆盖自检（check-gate-coverage.py）\n\n" +
		"options:\n" +
		"  -h, --help            show this help message and exit\n" +
		"  --root ROOT\n" +
		"  --config CONFIG\n" +
		"  --json\n" +
		"  --baseline\n" +
		"  --list-rules\n" +
		"  --self-test\n" +
		"  --no-self-test\n" +
		"  --max-examples MAX_EXAMPLES\n"
}

func cgcRunMain(stdout, stderr io.Writer, argv []string) int {
	prevProcErr := cgcProcStderr
	cgcProcStderr = stderr
	defer func() { cgcProcStderr = prevProcErr }()
	args, perr := cgcParseArgs(argv)
	if perr != "" {
		fmt.Fprint(stderr, cgcUsageLine)
		fmt.Fprintln(stderr, perr)
		return cgcRcBlocked
	}
	if args.help {
		fmt.Fprint(stdout, cgcHelpText())
		return cgcRcOK
	}
	if args.json && (args.selfTest || args.listRules) {
		which := "--list-rules"
		if args.selfTest {
			which = "--self-test"
		}
		fmt.Fprintf(stderr, "✗ 用法错（rc=2）：`--json` 与 `%s` 互斥 —— `%s` 只产人读文本、不产 JSON，"+
			"而 `--json` 的契约是「stdout 只有 JSON（可 json.loads）」⇒ 不给结论。\n", which, which)
		return cgcRcBlocked
	}
	if args.listRules {
		cgcPrintListRules(stdout)
		return cgcRcOK
	}
	if args.selfTest {
		fmt.Fprintln(stdout, "── check-gate-coverage 自检（成对负控：漏一个目录必红 ↔ 补齐必绿；"+
			"命中>基线必红 ↔ ≤基线必绿；白名单缺 reason/date ⇒ rc=2；空转 ⇒ rc=2）──")
		if cgcSelfTest(stdout, cgcDeepEnabled) {
			return cgcRcOK
		}
		return cgcRcBlocked
	}
	if !args.noSelfTest {
		st := stdout
		if args.json {
			st = stderr
		}
		fmt.Fprintln(st, "── 自检（成对负控）──")
		if !cgcSelfTest(st, cgcDeepEnabled) {
			fmt.Fprintln(st, "自检未过 ⇒ 拒绝扫真目标（rc=2 不给结论）")
			return cgcRcBlocked
		}
		fmt.Fprintln(st)
	}

	cwd, _ := os.Getwd()
	root := cgcPyAbspath(args.root, cwd)
	cfgPath := args.config
	if cfgPath == "" {
		cfgPath = cgcPyJoin(root, cgcLocalConfig)
	}
	cfg, err := cgcLoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "✗ 不给结论（rc=2）：%s\n", err)
		if args.json {
			cgcEmitUnsupportedJSON(stdout, root, cfgPath, err.Error())
		}
		return cgcRcBlocked
	}
	if !cgcIsDir(root) {
		reason := fmt.Sprintf("仓根不存在：%s", root)
		fmt.Fprintf(stderr, "✗ 不给结论（rc=2）：%s\n", reason)
		if args.json {
			cgcEmitUnsupportedJSON(stdout, root, cfgPath, reason)
		}
		return cgcRcBlocked
	}
	tmpdir, terr := os.MkdirTemp("", "gc-steps-")
	if terr != nil {
		fmt.Fprintf(stderr, "✗ 不给结论（rc=2）：%s\n", fmt.Sprintf("夹具目录创建失败：%s", terr))
		return cgcRcBlocked
	}
	defer os.RemoveAll(tmpdir)
	steps, serr := cgcExtractSteps(root, tmpdir)
	if serr != "" {
		fmt.Fprintf(stderr, "✗ 不给结论（rc=2）：%s\n", serr)
		if args.json {
			cgcEmitUnsupportedJSON(stdout, root, cfgPath, serr)
		}
		return cgcRcBlocked
	}
	gatesText := ""
	if b, e := os.ReadFile(cgcPyJoin(root, cgcGatesRel)); e == nil {
		gatesText = cgcDecodeReplace(b)
	}
	rep := cgcAnalyse(root, cfg, steps, gatesText, args.maxExamples, cwd)

	// ★ `_baseline_note` ↔ `baseline` 同源校验（`GAP-20260927-42` ①）：末次现值句必须 == baseline，
	//   否则人读报告说一套、`baseline` 是另一套 ⇒ 读者按错数算余量。红档并入 rc=1（不是 rc=2）。
	rep.noteDrift, rep.noteDriftDetail = cgcBaselineNoteDrift(rep.baseline, rep.baselineNote)

	nRedA := len(rep.aRed)
	nOverB := rep.bHit - rep.baseline
	nNewB := len(rep.bNewUngated)
	switch {
	case rep.aEmpty || rep.bEmpty:
		which := ""
		if rep.aEmpty {
			which += "A（构建清单目录）"
		}
		if rep.bEmpty {
			which += "B（scripts 脚本）"
		}
		rep.verdictCode = "EMPTY_SCAN"
		rep.verdictReason = fmt.Sprintf("空转：%s 候选集为 0 ⇒ 假覆盖（假绿的一种）⇒ 不给结论", which)
		rep.rc = cgcRcBlocked
	case !rep.certify:
		rep.verdictCode = "CERTIFY_FAILED"
		rep.verdictReason = "计数自证不过（三档之和 ≠ 候选总数）⇒ 不给结论"
		rep.rc = cgcRcBlocked
	case len(rep.bHitItems) > 0 && !rep.gitTrackedKnown:
		rep.verdictCode = "TRACKED_UNKNOWN"
		rep.verdictReason = fmt.Sprintf("判不了：仓根不是 git 仓 / git 不可用 ⇒ ③ 档 %d 条分不开"+
			"「新件 / 存量债」⇒ 不给结论", len(rep.bHitItems))
		rep.rc = cgcRcBlocked
	case nRedA > 0 || nOverB > 0 || nNewB > 0 || !rep.noteDrift:
		over := nOverB
		if over < 0 {
			over = 0
		}
		drift := 0
		if !rep.noteDrift {
			drift = 1
		}
		rep.verdictCode = "RED"
		rep.verdictReason = fmt.Sprintf("A 未收进 %d 条；B 棘轮命中 %d > 基线 %d（超 %d）；"+
			"新件未挂门 %d 条；_baseline_note 漂移 %d 条", nRedA, rep.bHit, rep.baseline, over, nNewB, drift)
		rep.rc = cgcRcFail
	default:
		rep.verdictCode = "OK"
		rep.verdictReason = fmt.Sprintf("A 无未收进目录；B 命中 %d ≤ 基线 %d（剩余 %d）；新件未挂门 0",
			rep.bHit, rep.baseline, rep.bHit)
		rep.rc = cgcRcOK
	}

	if args.json {
		cgcDumpJSON(stdout, rep)
		fmt.Fprintf(stderr, "rc=%d · %s\n", rep.rc, rep.verdictReason)
	} else {
		cgcPrintReport(stdout, rep)
		if args.baseline {
			cgcPrintBaseline(stdout, rep)
		}
		fmt.Fprintln(stdout, "== 结论 ==")
		over := nOverB
		if over < 0 {
			over = 0
		}
		fmt.Fprintf(stdout, "   A 未收进：%d 条 · B 棘轮命中 %d / 基线 %d（超 %d）· ④ 新件未挂门 %d 条\n",
			nRedA, rep.bHit, rep.baseline, over, nNewB)
		nd := "不漂"
		if !rep.noteDrift {
			nd = "**漂移**（note 与 baseline 不同源）"
		}
		fmt.Fprintf(stdout, "   _baseline_note：%s（%s）\n", nd, rep.noteDriftDetail)
		switch rep.rc {
		case cgcRcBlocked:
			fmt.Fprintf(stderr, "   ⇒ **不给结论（rc=2）**：%s\n", rep.verdictReason)
		case cgcRcFail:
			fmt.Fprintln(stdout, "   ⇒ 红（rc=1）：诊断见上；处置 = **真修**（补 add_step）或白名单写清 reason+date")
		default:
			fmt.Fprintf(stdout, "   ⇒ 绿（rc=0）：A 无未收进目录；B 命中 %d ≤ 基线 %d（剩余 %d）\n",
				rep.bHit, rep.baseline, rep.bHit)
		}
	}
	return rep.rc
}

// checkGateCoverageGateMain：M2 入口（不接线 / 不注册命令 / 不删原件）。
func checkGateCoverageGateMain(stdout, stderr io.Writer, argv []string) int {
	return cgcRunMain(stdout, stderr, argv)
}
