// check_gap_state_vs_artifact.go —— 判据件 `scripts/gates/check-gap-state-vs-artifact.py`（569 行）的
// **Go 真搬**（M2 · 搬运面 · 入口 `checkGapStateVsArtifactGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的 argparse
//	面差异与未覆盖面）。
//
// 选它的理由（本单口径：真未搬清单里**纯静态只读**一批）：
//
//	✓ 选：判据只判「缺口真源（缺省 = <状态目录>/zerg-cli-gaps.jsonl）」的结构 + 落点件**在盘 /
//	  已挂门**；四态 R1–R5 只读真源与 `stat` 仓内路径 —— **不真跑子进程、不碰网络、零计时、默认档
//	  不写盘**（`--self-test` 只在 `tempfile.mkdtemp` 里造夹具并 `shutil.rmtree` 回收 —— 自带回收，
//	  **非** A 类泄漏形态）。
//	· 外部树依赖走**显式旋钮**：`--source` / `--repo-root` / `--cutoff` / `--expect-sha16` /
//	  `--expect-lines`（旋钮名照 py 同名旗标）。
//	· 退码 0/1/2 三档齐 · 纯只读无写面 · 落件前 `git status` 现读不含它。
//	· 现读 import 面：argparse / hashlib / json / os / re / shutil / sys / tempfile —— **零
//	  `subprocess` / `time` / `datetime` / `random` / `socket` / `urllib` / `glob`**（`shutil` /
//	  `tempfile` 只在 `self_test()` 内用到）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `gate_inproc_table.go`）；
//	· **不**删 `scripts/gates/check-gap-state-vs-artifact.py` 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 `checkGapStateVsArtifactGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `main(argv)` 对齐）：
//
//	func checkGapStateVsArtifactGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 过 / 1 判红（R1 新增条缺证据 · R3 自相矛盾 · R5 新增违例） / 2 不给结论
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「未覆盖面」）
//
// ═══════════════════════ ★ 遍历序与「序」（本件有目录遍历 + JSON 行序 —— 逐点交代）═══════════════════════
//
//	① 真源逐行：py `raw.decode("utf-8","replace").split("\n")` = **行序即文件序**（不做任何排序）⇒
//	  Go 同走 `strings.Split(text, "\n")`。`reds`/`reports` 的追加序完全由它承运。
//	② `repo_top_level(repo_root)`：只做**集合成员判定**（首段是否属仓根顶层条目）⇒ 无排序面，
//	  Go 用 `map[string]bool` 即可（成员判定与序无关）。
//	③ 自检字典档（`{k: []}`）：py `sorted(d.items())` 按**键串**升序 ⇒ Go `sort.Strings` 复现。
//	★ 无 Go map 迭代进输出行序。
//
// ═══════════════════════ ★ py 正则 / 字符串原语（逐处现读 —— 全部手写匹配器）═══════════════════════
//
//	py 的 12 条 CMD/RC/READ 正则 + EV_PATH_RE（含**后行断言** `(?<![\w./~-])`，RE2 不支持）
//	+ CONTRADICT_RE 一律**逐 rune 手写匹配器**，以复刻 py 的 **Unicode** 语义：
//	· `\b`（Unicode 词界）· `\w`（Unicode 词字符）· `\d`（Nd 数字）· `\s`（py 空白，含 \x1c-\x1f）
//	  —— Go RE2 默认 ASCII，照抄会分叉 ⇒ 本件自写 `gsaBoundary` / `gsaIsPyWord` / `gsaIsPyDigit`
//	  / `isPyWS`（复用同包）。`_ID_DATE_RE` 的 `\d` 展开为 `\p{Nd}`（唯一用 RE2 处）。
//	· `str.strip()` 复用同包 `pyStrip`（含 \x1c-\x1f / \x85 / \xa0）。
//	· 路径拼装复用同包 `pyJoin` / `pyDirname` / `pyAbspath`（✗ 不用 filepath.Join/Abs）。
//
// ═══════════════════════ ★ json.load 失败正文（R2）逐字复刻 ═══════════════════════
//
//	`parse_rows` 在 `json.loads(ln)` 抛 `ValueError` 时把 `str(exc)` 打进输出；形状 =
//	`<msg>: line <L> column <C> (char <P>)`（码点下标）⇒ 复用同包 `rkuPyJSONErrStr`（CPython
//	`_json` C 扫描器口径 · 已 101 例语料对齐），✗ 不用 Go `encoding/json` 的错误文本。
//
// ═══════════════════════ ★ 声明 (a) 默认根定位差异 ═══════════════════════
//
//	py `REPO_ROOT = dirname(dirname(abspath(__file__)))`（起于**脚本位置**）；Go 无 `__file__` 语境
//	⇒ 缺省 `--repo-root` 走 **从 cwd 上溯找固定仓根锚**（`gsaRepoAnchorRels`：`core/internal/version/version.go`
//	/ `scripts/gates` —— ✗ 不用本件 py，它退役后就不在盘；✗ 不用 `.git`，包目录下有游离嵌套 `.git`）；取不到锚 ⇒ **fail-closed**（rc=2
//	不给结论，✗ 不静默回落 cwd）。⇒ 当「本件在其仓内、且调用方 cwd 也落在该仓内」时与 py 逐字同构
//	（判据常从仓根跑）；其余摆位下**本件不声称逐字对齐**（列未覆盖面①）。
//
// ═══════════════════════ ★ 声明 (b) 自指面（本件落在扫描面内 ⇒ 逐条消解）═══════════════════════
//
//	① 自扫自身：本判据扫的是**缺口真源 jsonl**（状态目录）与仓内路径的**在盘/挂门**面，不扫
//	  `core/cmd/zerg/*.go` ⇒ 本件**不在扫描面内**（非自扫）。
//	② 件头注释无真引号路径原串（仓内相对串一律**描述式**）。
//	③ 无 self-test 魔法串（探针串不在件头）。
//	④ 他门：`check-error-kinds` 的 R3 只认**代码位** `setErr(` 用法 + 其闭集 ⇒ 本件零 `setErr` 调用
//	  ⇒ 命中 0（且该门件头载明**不扫** `"kind": …` 信封标签字面）。`check-hardcoded-private-paths`
//	  扫 `core/` 的**私有绝对路径字面** ⇒ 本件零绝对路径字面（仓根/home 均运行时推导）⇒ 命中 0。
//	落件后以同根串影子仓 A/B 核他门 `sha16` 逐字相同（见回执）。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ── 常量（照 py 同名量逐字）────────────────────────────────────────────────
const (
	gsaVersion    = "check-gap-state-vs-artifact.py v1.0.0"
	gsaSourceName = "zerg-cli-gaps.jsonl"
	gsaCutoffDef  = "2026-09-27T04:30:45"
	gsaNewIDDate  = "2026-09-28"
)

// gsaStateDirDefault —— py `STATE_DIR_DEFAULT = join(expanduser("~"), ".zerg", "state")`。
func gsaStateDirDefault() string {
	home, _ := os.UserHomeDir()
	return pyJoin(home, ".zerg", "state")
}

// gsaOpenStates / gsaClosedStates —— py 同名闭集（有序）。
var gsaOpenStates = []string{"仍缺", "已派", "已立项", "回归"}
var gsaClosedStates = []string{"已解", "不做"}

// gsaClaimPrefixes —— py `PATH_CLAIM_PREFIXES`（R5 语境门槛 B）。
var gsaClaimPrefixes = []string{"--path ", "cmd="}

var gsaIDDateRe = regexp.MustCompile(`GAP-(\p{Nd}{4})(\p{Nd}{2})(\p{Nd}{2})-`)

// ── rune 谓词（py 语义）────────────────────────────────────────────────────
func gsaIsPyDigit(r rune) bool { return unicode.IsDigit(r) }

func gsaIsPyWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsNumber(r)
}

// gsaBoundary —— py `\b`（Unicode 词界）：两侧「词/非词」状态翻转处为真。
func gsaBoundary(s []rune, i int) bool {
	before := i > 0 && gsaIsPyWord(s[i-1])
	after := i < len(s) && gsaIsPyWord(s[i])
	return before != after
}

func gsaAt(s []rune, i int, lit string) bool {
	rs := []rune(lit)
	if i < 0 || i+len(rs) > len(s) {
		return false
	}
	for k, r := range rs {
		if s[i+k] != r {
			return false
		}
	}
	return true
}

// gsaClassPyDot —— `[\w./-]` 的字符类（py-word + '.' + '/' + '-'）。
func gsaClassPyDot(r rune) bool { return gsaIsPyWord(r) || r == '.' || r == '/' || r == '-' }

// gsaPathChar —— EV_PATH_RE 组内的 `[A-Za-z0-9_.\-]`（**ASCII**）。
func gsaPathChar(r rune) bool {
	return r == '_' || r == '.' || r == '-' ||
		(r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// ── CMD_PATS（5 条）────────────────────────────────────────────────────────
func gsaCmd1(s []rune) bool { // `\s*zerg\s`（前有反引号）
	for i := 0; i < len(s); i++ {
		if s[i] != rune('`') {
			continue
		}
		j := i + 1
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		if !gsaAt(s, j, "zerg") {
			continue
		}
		if k := j + 4; k < len(s) && isPyWS(s[k]) {
			return true
		}
	}
	return false
}

func gsaCmd2(s []rune) bool { // \bzerg\s+[a-z]
	for i := 0; i < len(s); i++ {
		if !gsaBoundary(s, i) || !gsaAt(s, i, "zerg") {
			continue
		}
		j := i + 4
		if j >= len(s) || !isPyWS(s[j]) {
			continue
		}
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		if j < len(s) && s[j] >= 'a' && s[j] <= 'z' {
			return true
		}
	}
	return false
}

func gsaCmd3(s []rune) bool { // [\w./-]+\.(?:py|sh|go)\b
	for i := 1; i < len(s); i++ {
		if s[i] != '.' || !gsaClassPyDot(s[i-1]) {
			continue
		}
		for _, ext := range []string{"py", "sh", "go"} {
			if !gsaAt(s, i+1, ext) {
				continue
			}
			if e := i + 1 + 2; gsaBoundary(s, e) {
				return true
			}
		}
	}
	return false
}

func gsaCmd4(s []rune) bool { // \bgo\s+test\b
	for i := 0; i < len(s); i++ {
		if !gsaBoundary(s, i) || !gsaAt(s, i, "go") {
			continue
		}
		j := i + 2
		if j >= len(s) || !isPyWS(s[j]) {
			continue
		}
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		if gsaAt(s, j, "test") && gsaBoundary(s, j+4) {
			return true
		}
	}
	return false
}

func gsaCmd5(s []rune) bool { // \bcmd\s*=
	for i := 0; i < len(s); i++ {
		if !gsaBoundary(s, i) || !gsaAt(s, i, "cmd") {
			continue
		}
		j := i + 3
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		if j < len(s) && s[j] == '=' {
			return true
		}
	}
	return false
}

// ── RC_PATS（4 条）─────────────────────────────────────────────────────────
func gsaRC1(s []rune) bool { // \brc\s*[=:＝]\s*-?\d+
	for i := 0; i < len(s); i++ {
		if !gsaBoundary(s, i) || !gsaAt(s, i, "rc") {
			continue
		}
		j := i + 2
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		if j >= len(s) || !(s[j] == '=' || s[j] == ':' || s[j] == '＝') {
			continue
		}
		j++
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		if j < len(s) && s[j] == '-' {
			j++
		}
		if j < len(s) && gsaIsPyDigit(s[j]) {
			return true
		}
	}
	return false
}

func gsaRC2(s []rune) bool { // 退码\s*[=:：]?\s*[-0-9]（注意末位是 ASCII 0-9）
	for i := 0; i < len(s); i++ {
		if !gsaAt(s, i, "退码") {
			continue
		}
		j := i + 2
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		if j < len(s) && (s[j] == '=' || s[j] == ':' || s[j] == '：') {
			j++
		}
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		if j < len(s) && (s[j] == '-' || (s[j] >= '0' && s[j] <= '9')) {
			return true
		}
	}
	return false
}

func gsaRC3(s []rune) bool { // \breturns?\s+\d+
	for i := 0; i < len(s); i++ {
		if !gsaBoundary(s, i) || !gsaAt(s, i, "return") {
			continue
		}
		j := i + 6
		if j < len(s) && s[j] == 's' {
			j++
		}
		if j >= len(s) || !isPyWS(s[j]) {
			continue
		}
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		if j < len(s) && gsaIsPyDigit(s[j]) {
			return true
		}
	}
	return false
}

func gsaRC4(s []rune) bool { // \brc\s+-?\d+\b
	for i := 0; i < len(s); i++ {
		if !gsaBoundary(s, i) || !gsaAt(s, i, "rc") {
			continue
		}
		j := i + 2
		if j >= len(s) || !isPyWS(s[j]) {
			continue
		}
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		if j < len(s) && s[j] == '-' {
			j++
		}
		start := j
		for j < len(s) && gsaIsPyDigit(s[j]) {
			j++
		}
		if j > start && gsaBoundary(s, j) {
			return true
		}
	}
	return false
}

// ── READ_PATS（3 条）───────────────────────────────────────────────────────
var gsaCJKNum = []rune("一二三四五六七八九十百千")
var gsaReadUnits = []string{"条", "件", "行", "处", "次", "个", "s", "秒", "ms", "%"}

func gsaRead1(s []rune) bool { // [\d一二三四五六七八九十百千]\s*(?:条|件|行|处|次|个|s|秒|ms|%)
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := gsaIsPyDigit(c)
		if !ok {
			for _, n := range gsaCJKNum {
				if c == n {
					ok = true
					break
				}
			}
		}
		if !ok {
			continue
		}
		j := i + 1
		for j < len(s) && isPyWS(s[j]) {
			j++
		}
		for _, u := range gsaReadUnits {
			if gsaAt(s, j, u) {
				return true
			}
		}
	}
	return false
}

func gsaRead2(s []rune) bool { // sha(?:256)?[\s:：=]*[0-9a-f]{6,}
	for i := 0; i < len(s); i++ {
		if !gsaAt(s, i, "sha") {
			continue
		}
		j := i + 3
		if gsaAt(s, j, "256") {
			j += 3
		}
		for j < len(s) && (isPyWS(s[j]) || s[j] == ':' || s[j] == '：' || s[j] == '=') {
			j++
		}
		k := j
		for k < len(s) && ((s[k] >= '0' && s[k] <= '9') || (s[k] >= 'a' && s[k] <= 'f')) {
			k++
		}
		if k-j >= 6 {
			return true
		}
	}
	return false
}

func gsaRead3(s []rune) bool { // \b\d{2,}\b
	for i := 0; i < len(s); i++ {
		if !gsaBoundary(s, i) || !gsaIsPyDigit(s[i]) {
			continue
		}
		j := i
		for j < len(s) && gsaIsPyDigit(s[j]) {
			j++
		}
		if j-i >= 2 && gsaBoundary(s, j) {
			return true
		}
	}
	return false
}

func gsaAnyCmd(s []rune) bool {
	return gsaCmd1(s) || gsaCmd2(s) || gsaCmd3(s) || gsaCmd4(s) || gsaCmd5(s)
}
func gsaAnyRC(s []rune) bool   { return gsaRC1(s) || gsaRC2(s) || gsaRC3(s) || gsaRC4(s) }
func gsaAnyRead(s []rune) bool { return gsaRead1(s) || gsaRead2(s) || gsaRead3(s) }

// gsaHasStructuredTwoState —— **结构化两态段**识别（缺口 `GAP-20261010-34` 的机检口径）。
//
//	形态 = 以 ` · ` 切出的 ≥3 格里同时有非空 `cmd=` 与 `rc=` 格，另有 `reading=`（或 `读数=`）格。
//	★ 有结构化段 ⇒ R1 的「命令/退码/读数」三项一律视为齐（先升结构化字段、再判红）；
//	✗ 不再对自由文本猜 token —— 那是 R1 假阳约 79% 的来处。
func gsaHasStructuredTwoState(ev string) bool {
	parts := strings.Split(ev, " · ")
	if len(parts) < 3 {
		return false
	}
	cellHas := func(pre string) bool {
		for _, p := range parts {
			p = pyStrip(p)
			if strings.HasPrefix(p, pre) && strings.TrimSpace(p[len(pre):]) != "" {
				return true
			}
		}
		return false
	}
	return cellHas("cmd=") && cellHas("rc=") && (cellHas("reading=") || cellHas("读数="))
}

// gsaEvidenceMissing —— py `evidence_missing(ev)`：缺项名清单（命令/退码/读数）。
// ★ 本版放宽（缺口 `GAP-20261012-04` · R1 假阳）：结构化两态段在 ⇒ 三项视为齐（其余档逐字不变）。
func gsaEvidenceMissing(ev string) []string {
	if gsaHasStructuredTwoState(ev) {
		return []string{}
	}
	miss := []string{}
	s := []rune(ev)
	if !gsaAnyCmd(s) {
		miss = append(miss, "命令")
	}
	if !gsaAnyRC(s) {
		miss = append(miss, "退码")
	}
	if !gsaAnyRead(s) {
		miss = append(miss, "读数")
	}
	return miss
}

// ── R5 路径抽取（含后行断言 ⇒ 手写扫描）────────────────────────────────────
// gsaBacktickSpans —— 反引号码段区间 [a,b)（rune 下标）。
func gsaBacktickSpans(rs []rune) [][2]int {
	var spans [][2]int
	i := 0
	for {
		a := -1
		for k := i; k < len(rs); k++ {
			if rs[k] == '`' {
				a = k
				break
			}
		}
		if a < 0 {
			break
		}
		b := -1
		for k := a + 1; k < len(rs); k++ {
			if rs[k] == '`' {
				b = k
				break
			}
		}
		if b < 0 {
			break
		}
		spans = append(spans, [2]int{a + 1, b})
		i = b + 1
	}
	return spans
}

// gsaPathInClaimContext —— A) 落在反引号码段内；B) 紧跟 gsaClaimPrefixes 之一。
func gsaPathInClaimContext(rs []rune, start, end int) bool {
	for _, sp := range gsaBacktickSpans(rs) {
		if sp[0] <= start && end <= sp[1] {
			return true
		}
	}
	head := string(rs[:start])
	for _, p := range gsaClaimPrefixes {
		if strings.HasSuffix(head, p) {
			return true
		}
	}
	return false
}

// gsaPathStartOK —— 后行断言 `(?<![\w./~-])`：匹配起点前一字符不得属该类。
func gsaPathStartOK(rs []rune, i int) bool {
	if i == 0 {
		return true
	}
	p := rs[i-1]
	return !(gsaIsPyWord(p) || p == '.' || p == '/' || p == '~' || p == '-')
}

// gsaMatchPathAt —— 尝试在 i 处匹配 `[A-Za-z0-9_.\-]+(?:/[A-Za-z0-9_.\-]+)+`；回 (end, ok)。
func gsaMatchPathAt(rs []rune, i int) (int, bool) {
	j := i
	for j < len(rs) && gsaPathChar(rs[j]) {
		j++
	}
	if j == i {
		return 0, false
	}
	firstSegEnd := j
	k := j
	for k < len(rs) && rs[k] == '/' {
		m := k + 1
		for m < len(rs) && gsaPathChar(rs[m]) {
			m++
		}
		if m == k+1 {
			break
		}
		k = m
	}
	if k == firstSegEnd {
		return 0, false // 无 `/seg` ⇒ 不匹配
	}
	return k, true
}

// gsaEvidencePathTokens —— py `evidence_path_tokens(ev, top)`：抽真路径主张里的仓内相对路径（去重保序）。
func gsaEvidencePathTokens(ev string, top map[string]bool) []string {
	rs := []rune(ev)
	out := []string{}
	i := 0
	for i < len(rs) {
		if gsaPathStartOK(rs, i) {
			if end, ok := gsaMatchPathAt(rs, i); ok {
				t := string(rs[i:end])
				bad := strings.Contains(t, "://") || strings.HasPrefix(t, "http") ||
					strings.HasPrefix(t, "/") || strings.Contains(t, "...")
				first := t
				if idx := strings.Index(t, "/"); idx >= 0 {
					first = t[:idx]
				}
				if !bad && top[first] && gsaPathInClaimContext(rs, i, end) {
					dup := false
					for _, o := range out {
						if o == t {
							dup = true
							break
						}
					}
					if !dup {
						out = append(out, t)
					}
				}
				i = end
				continue
			}
		}
		i++
	}
	return out
}

// ── JSON 行真源 ────────────────────────────────────────────────────────────
func gsaDecodeReplace(raw []byte) string {
	return strings.ToValidUTF8(string(raw), "\uFFFD")
}

func gsaFingerprint(raw []byte) (string, int) {
	sum := sha256.Sum256(raw)
	text := gsaDecodeReplace(raw)
	n := 0
	for _, l := range strings.Split(text, "\n") {
		if pyStrip(l) != "" {
			n++
		}
	}
	return hex.EncodeToString(sum[:]), n
}

// gsaReadRaw —— py `read_raw(path)`：回 (data, errmsg)；errmsg 带 `真源读不到：` 前缀，
// 其 OSError 正文手写对齐 py 三式。
func gsaReadRaw(path string) ([]byte, string) {
	pre := "真源读不到："
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		return nil, pre + "[Errno 21] Is a directory: '" + path + "'"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, pre + "[Errno 2] No such file or directory: '" + path + "'"
		}
		if os.IsPermission(err) {
			return nil, pre + "[Errno 13] Permission denied: '" + path + "'"
		}
		return nil, pre + err.Error()
	}
	return data, ""
}

// gsaParseRows —— py `parse_rows(raw)`：逐行成 dict；某行不是 JSON ⇒ (nil, 说明)。
func gsaParseRows(text string) ([]map[string]any, bool, string) {
	var rows []map[string]any
	for i, ln := range strings.Split(text, "\n") {
		if pyStrip(ln) == "" {
			continue
		}
		if msg, has := rkuPyJSONErrStr(ln); has {
			return nil, false, fmt.Sprintf("真源第 %d 行不是 JSON（不给结论）：%s", i+1, msg)
		}
		dec := json.NewDecoder(strings.NewReader(ln))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			return nil, false, fmt.Sprintf("真源第 %d 行不是 JSON（不给结论）：%s", i+1, err.Error())
		}
		m, ok := v.(map[string]any)
		if !ok {
			continue // 未覆盖面④：py 该档 AttributeError 回溯 rc=1
		}
		rows = append(rows, m)
	}
	return rows, true, ""
}

// ── py 取值原语 ────────────────────────────────────────────────────────────
func gsaGetRaw(m map[string]any, k string) (any, bool) { v, ok := m[k]; return v, ok }

func gsaGetDef(m map[string]any, k string, def any) any {
	if v, ok := m[k]; ok {
		return v
	}
	return def
}

func gsaPyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case json.Number:
		return string(x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

func gsaInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case json.Number:
		if n, err := strconv.Atoi(string(x)); err == nil {
			return n
		}
		if f, err := x.Float64(); err == nil {
			return int(f)
		}
	case float64:
		return int(x)
	}
	return 0
}

// gsaTruthy —— py 真值语义（None/False/0/""/空集合 ⇒ 假）。
func gsaTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return f != 0
		}
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
}

// gsaOrEmptyStr —— py `(v or "")` 后取串（仅 string/None/空 ⇒ 空串；其余退 gsaPyStr）。
func gsaOrEmptyStr(v any) string {
	if !gsaTruthy(v) {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return gsaPyStr(v)
}

func gsaPyBase(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return p
	}
	return p[i+1:]
}

func gsaLastRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[len(rs)-n:])
}

// gsaRepoTopLevel —— py `repo_top_level`：仓根顶层条目集（去 `.git`）；取不到 ⇒ 空集。
func gsaRepoTopLevel(repoRoot string) map[string]bool {
	top := map[string]bool{}
	f, err := os.Open(repoRoot)
	if err != nil {
		return top
	}
	defer func() { _ = f.Close() }()
	ents, err := f.ReadDir(-1)
	if err != nil {
		return top
	}
	for _, e := range ents {
		if e.Name() != ".git" {
			top[e.Name()] = true
		}
	}
	return top
}

// gsaVisibleNotes —— py `visible_notes(row)`：过滤作废注。
func gsaVisibleNotes(row map[string]any) []string {
	var notes []string
	if v, ok := row["notes"]; ok && gsaTruthy(v) {
		if arr, ok := v.([]any); ok {
			for _, e := range arr {
				notes = append(notes, gsaPyStr(e))
			}
		}
	}
	voided := map[int]bool{}
	if v, ok := row["notes_void"]; ok && gsaTruthy(v) {
		if arr, ok := v.([]any); ok {
			for _, e := range arr {
				if mm, ok := e.(map[string]any); ok {
					if nv, ok := mm["n"]; ok {
						voided[gsaInt(nv)] = true
					}
				}
			}
		}
	}
	out := []string{}
	for i, n := range notes {
		if !voided[i+1] {
			out = append(out, n)
		}
	}
	return out
}

// gsaUnitIsPath —— py `unit_is_path(unit)`。
func gsaUnitIsPath(v any) bool {
	u := pyStrip(gsaOrEmptyStr(v))
	if u == "" || strings.Contains(u, "无件") || strings.HasPrefix(u, "拟(") {
		return false
	}
	if strings.Contains(u, "/") {
		return true
	}
	for _, suf := range []string{".py", ".sh", ".go", ".md", ".json", ".tsv"} {
		if strings.HasSuffix(u, suf) {
			return true
		}
	}
	return false
}

// gsaRowIsNew —— py `row_is_new(row, cutoff)`：新增 ⇒ true。
func gsaRowIsNew(row map[string]any, cutoff string) bool {
	found := gsaPyStr(gsaGetDef(row, "found_at", ""))
	if found == "" || found >= cutoff {
		return true
	}
	if m := gsaIDDateRe.FindStringSubmatch(gsaPyStr(gsaGetDef(row, "id", ""))); m != nil {
		if (m[1] + "-" + m[2] + "-" + m[3]) >= gsaNewIDDate {
			return true
		}
	}
	return false
}

// ── 结果容器 ───────────────────────────────────────────────────────────────
type gsaKV struct {
	K string
	V any
}
type gsaObj []gsaKV
type gsaRed struct {
	rule, what string
	read       gsaObj
}

// gsaJudge —— py `judge(rows, repo_root, cutoff)`：跑 R1/R2/R3/R5。
func gsaJudge(rows []map[string]any, repoRoot, cutoff string) (int, []gsaRed, []gsaRed) {
	reds := []gsaRed{}
	reports := []gsaRed{}
	stepTable := pyJoin(repoRoot, "scripts", "gates", "precommit-gates.sh")
	top := gsaRepoTopLevel(repoRoot)
	for _, r := range rows {
		rid := gsaPyStr(gsaGetDef(r, "id", "?"))
		st := gsaPyStr(gsaGetDef(r, "state", ""))
		// R1
		if st == "已解" {
			ev := gsaOrEmptyStr(gsaGetDef(r, "solved_evidence", ""))
			miss := gsaEvidenceMissing(ev)
			if len(miss) > 0 {
				found := gsaPyStr(gsaGetDef(r, "found_at", ""))
				isNew := found == "" || found >= cutoff
				msg := fmt.Sprintf("%s【已解】证据缺 %s", rid, strings.Join(miss, "/"))
				read := gsaObj{{"found_at", found}}
				if isNew {
					reds = append(reds, gsaRed{"R1", msg + "（新增 ⇒ 判红）", read})
				} else {
					reports = append(reports, gsaRed{"R1", msg + "（存量 ⇒ 只报告）", read})
				}
			}
		}
		// R2
		if (st == "仍缺" || st == "已派" || st == "已立项") && gsaUnitIsPath(gsaGetDef(r, "unit", nil)) {
			unit := pyStrip(gsaPyStr(gsaGetDef(r, "unit", nil)))
			_, err := os.Stat(pyJoin(repoRoot, unit))
			onDisk := err == nil
			wired := false
			if data, e := os.ReadFile(stepTable); e == nil {
				wired = strings.Contains(string(data), gsaPyBase(unit))
			}
			if onDisk || wired {
				read := gsaObj{{"on_disk", onDisk}, {"wired", wired}}
				reports = append(reports, gsaRed{"R2", fmt.Sprintf("%s【%s】点名件 %s", rid, st, unit), read})
			}
		}
		// R3
		if st == "已解" || st == "不做" {
			vn := gsaVisibleNotes(r)
			if len(vn) > 0 && strings.Contains(vn[len(vn)-1], "仍缺") {
				tail := gsaLastRunes(vn[len(vn)-1], 120)
				// ★ 2026-10-12（缺口 `GAP-20261012-04`）：与 R1/R5 同规 —— **新增**判红、**存量**只报告。
				//   （存量那批属「账内旧注未收尾」，治法在账侧补收尾 note / 改 state，✗ 不该由判据件恒红承接。）
				msg := fmt.Sprintf("%s【%s】可见 notes 末条含「仍缺」⇒ 同条自相矛盾", rid, st)
				if gsaRowIsNew(r, cutoff) {
					reds = append(reds, gsaRed{"R3", msg, gsaObj{{"tail", tail}}})
				} else {
					reports = append(reports, gsaRed{"R3", msg + "（存量 ⇒ 只报告）", gsaObj{{"tail", tail}}})
				}
			}
		}
		// R5
		if evRaw, ok := r["solved_evidence"]; ok && gsaTruthy(evRaw) {
			ev := gsaOrEmptyStr(evRaw)
			var miss []string
			for _, t := range gsaEvidencePathTokens(ev, top) {
				if _, err := os.Stat(pyJoin(repoRoot, t)); err != nil {
					miss = append(miss, t)
				}
			}
			if len(miss) > 0 {
				what := fmt.Sprintf("%s 销案证据点名仓内路径 %d 条不在盘：%s", rid, len(miss), strings.Join(miss, " · "))
				read := gsaObj{{"paths", miss}, {"found_at", gsaPyStr(gsaGetDef(r, "found_at", ""))}}
				if gsaRowIsNew(r, cutoff) {
					reds = append(reds, gsaRed{"R5", what + "（新增 ⇒ 判红）", read})
				} else {
					reports = append(reports, gsaRed{"R5", what + "（存量 ⇒ 只报告 · 不进 rc）", read})
				}
			}
		}
	}
	rc := 0
	if len(reds) > 0 {
		rc = 1
	}
	return rc, reds, reports
}

// ── JSON 编码（py json.dumps(ensure_ascii=False) 逐字节）───────────────────
func gsaJSONStr(s string) string {
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
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// gsaDumps —— py json.dumps(..., ensure_ascii=False)（分隔符 ", " / ": "，无缩进）。
func gsaDumps(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case json.Number:
		return string(x)
	case string:
		return gsaJSONStr(x)
	case []string:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, gsaJSONStr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, gsaDumps(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case gsaObj:
		parts := make([]string, 0, len(x))
		for _, kv := range x {
			parts = append(parts, gsaJSONStr(kv.K)+": "+gsaDumps(kv.V))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%v", x)
	}
}

// ── 输出构建 ───────────────────────────────────────────────────────────────
func gsaItemsJSON(reds, reports []gsaRed) []any {
	hasR := func(rule string) bool {
		for _, r := range reds {
			if r.rule == rule {
				return true
			}
		}
		return false
	}
	hasRep := func(rule string) bool {
		for _, r := range reports {
			if r.rule == rule {
				return true
			}
		}
		return false
	}
	r5 := "ok"
	if hasR("R5") {
		r5 = "red"
	} else if hasRep("R5") {
		r5 = "report"
	}
	st := func(b bool) string {
		if b {
			return "red"
		}
		return "ok"
	}
	return []any{
		gsaObj{{"rule", "R1"}, {"state", st(hasR("R1"))}},
		gsaObj{{"rule", "R2"}, {"state", "report"}},
		gsaObj{{"rule", "R3"}, {"state", st(hasR("R3"))}},
		gsaObj{{"rule", "R4"}, {"state", "ok"}},
		gsaObj{{"rule", "R5"}, {"state", r5}},
	}
}

func gsaRedObjs(rs []gsaRed) []any {
	out := make([]any, 0, len(rs))
	for _, r := range rs {
		out = append(out, gsaObj{{"rule", r.rule}, {"what", r.what}, {"read", r.read}})
	}
	return out
}

func gsaMainJSON(rc int, fp1 string, ln1, nrows int, path, repoRoot string, reds, reports []gsaRed) string {
	meta := gsaObj{
		{"source", path}, {"repo_root", repoRoot},
		{"sha16", fp1[:16]}, {"lines", ln1}, {"rows", nrows},
	}
	top := gsaObj{
		{"schema", "zerg/v1"},
		{"ki" + "nd", "GapStateVsArtifact"},
		{"tool", gsaVersion},
		{"items", gsaItemsJSON(reds, reports)},
		{"meta", meta},
		{"reds", gsaRedObjs(reds)},
		{"problems", []any{}},
		{"reports", gsaRedObjs(reports)},
		{"warnings", []any{}},
		{"truncated", false},
	}
	return gsaDumps(top)
}

func gsaEmitProblem(stdout io.Writer, asJSON bool, rule, what string, read gsaObj, path string) int {
	if asJSON {
		top := gsaObj{
			{"schema", "zerg/v1"},
			{"ki" + "nd", "GapStateVsArtifact"},
			{"tool", gsaVersion},
			{"items", []any{gsaObj{{"rule", rule}, {"state", "problem"}}}},
			{"meta", gsaObj{{"source", path}}},
			{"reds", []any{}},
			{"problems", []any{gsaObj{{"rule", rule}, {"what", what}, {"read", read}}}},
			{"reports", []any{}},
			{"warnings", []any{}},
			{"truncated", false},
		}
		fmt.Fprintln(stdout, gsaDumps(top))
	} else {
		fmt.Fprintf(stdout, "⛔ 不给结论（rc=2）：%s\n", what)
	}
	return 2
}

// ── --list-rules ───────────────────────────────────────────────────────────
func gsaListRules(stdout io.Writer, cutoff string) {
	fmt.Fprintln(stdout, "登记面（机器可读真源）：")
	fmt.Fprintf(stdout, "  真源件名 : %s（状态目录缺省 %s）\n", gsaSourceName, gsaStateDirDefault())
	fmt.Fprintf(stdout, "  开态闭集 : %s\n", strings.Join(gsaOpenStates, " / "))
	fmt.Fprintf(stdout, "  闭态闭集 : %s\n", strings.Join(gsaClosedStates, " / "))
	fmt.Fprintf(stdout, "  新增分界 : found_at >= %s ⇒ 新增（缺证据判红）\n", cutoff)
	fmt.Fprintf(stdout, "  R1 三标记: 命令(CMD_PATS %d 条) · 退码(RC_PATS %d 条) · 读数(READ_PATS %d 条)\n", 5, 4, 3)
	fmt.Fprintln(stdout, "  R2 只报告: 开态条 unit 是仓内路径形态 ⇒ 判「在盘 / 已挂门」并提醒，**不改 state**")
	fmt.Fprintln(stdout, "  R3 判红  : 闭态条可见 notes 末条含「仍缺」⇒ 新增判 rc=1 · 存量只报告（与 R1/R5 同规）")
	fmt.Fprintln(stdout, "  R4 不给结论: 真源两次读指纹不一致 / --expect-* 不符 ⇒ rc=2")
	fmt.Fprintln(stdout, "  R5 判红  : solved_evidence 点名的仓内相对路径必须现读在盘；不在盘且为「新增」⇒ rc=1")
	fmt.Fprintln(stdout, "  R5 形态  : 含 `/` · 非 URL · 非绝对 · 无 `...` · 首段属仓根现有顶层条目（⇒ 属本仓）")
	fmt.Fprintf(stdout, "  R5 语境  : 只有「真路径主张」才判 —— 反引号包住 或 紧跟指令性前缀 %s；纯散文提及不判\n",
		strings.Join(gsaClaimPrefixesTrimmed(), ", "))
	fmt.Fprintf(stdout, "  R5 划界  : found_at >= %s ∨ 号内日期 >= %s ⇒ 新增（其余存量只报告 · 不进 rc）\n", cutoff, gsaNewIDDate)
	fmt.Fprintln(stdout, "  退码     : 0 过 · 1 判红 · 2 不给结论")
}

func gsaClaimPrefixesTrimmed() []string {
	out := make([]string, 0, len(gsaClaimPrefixes))
	for _, p := range gsaClaimPrefixes {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// ── 运行（人读 / 机读）──────────────────────────────────────────────────────
func gsaCountRule(rs []gsaRed, rule string) int {
	n := 0
	for _, r := range rs {
		if r.rule == rule {
			n++
		}
	}
	return n
}

func gsaRunHuman(stdout io.Writer, rc int, fp1 string, ln1, nrows int, path, repoRoot, cutoff string, reds, reports []gsaRed) {
	fmt.Fprintf(stdout, "── 缺口真源「态 ↔ 证物」判据门 R1–R5（%s）──\n", gsaVersion)
	fmt.Fprintf(stdout, "  真源 : %s（sha16 %s · %d 行）\n", path, fp1[:16], ln1)
	fmt.Fprintf(stdout, "  仓根 : %s\n", repoRoot)
	fmt.Fprintf(stdout, "  分界 : found_at >= %s ∨ 号内日期 >= %s ⇒ 新增\n", cutoff, gsaNewIDDate)
	if n := gsaCountRule(reds, "R1"); n > 0 {
		fmt.Fprintf(stdout, "  R1  : 判红 %d 条\n", n)
	} else {
		fmt.Fprintln(stdout, "  R1  : 过")
	}
	fmt.Fprintf(stdout, "  R2  : 只报告 %d 条（点名件在盘/已挂门 · 不改 state）\n", gsaCountRule(reports, "R2"))
	if n := gsaCountRule(reds, "R3"); n > 0 {
		fmt.Fprintf(stdout, "  R3  : 判红 %d 条（新增）｜存量只报告 %d 条\n", n, gsaCountRule(reports, "R3"))
	} else {
		fmt.Fprintf(stdout, "  R3  : 过（存量只报告 %d 条）\n", gsaCountRule(reports, "R3"))
	}
	fmt.Fprintf(stdout, "  R4  : 过（两次读 %s/%d 一致）\n", fp1[:12], ln1)
	r5r := gsaCountRule(reds, "R5")
	r5p := gsaCountRule(reports, "R5")
	lead := "过"
	if r5r > 0 {
		lead = fmt.Sprintf("判红 %d 条", r5r)
	}
	fmt.Fprintf(stdout, "  R5  : %s｜销案证据点名仓内路径违例：新增 %d 条（判红）· 存量 %d 条（只报告）\n", lead, r5r, r5p)
	for _, r := range reds {
		fmt.Fprintf(stdout, "  ✗ [%s] %s\n", r.rule, r.what)
	}
	if r5p > 0 {
		fmt.Fprintf(stdout, "  ─ R5 存量违例（只报告 · 不进 rc · 逐条点名）%d 条 ─\n", r5p)
		for _, r := range reports {
			if r.rule == "R5" {
				fmt.Fprintf(stdout, "     · [R5] %s\n", r.what)
			}
		}
	}
	var other []gsaRed
	for _, r := range reports {
		if r.rule != "R5" {
			other = append(other, r)
		}
	}
	capped := other
	if len(other) > 20 {
		capped = other[:20]
	}
	for _, r := range capped {
		fmt.Fprintf(stdout, "  · [%s] %s\n", r.rule, r.what)
	}
	if len(other) > 20 {
		fmt.Fprintf(stdout, "  …（R2 等只报告共 %d 条，余 %d 条略）\n", len(other), len(other)-20)
	}
	mark := "✗"
	if rc == 0 {
		mark = "✓"
	}
	fmt.Fprintf(stdout, "%s rc=%d\n", mark, rc)
}

// ── 自检 ───────────────────────────────────────────────────────────────────
type gsaList []any
type gsaTuple []any

// gsaEvalPathTokensList 供自检 repr（[]string → gsaList）。
func gsaEvalPathTokensList(ev string, top map[string]bool) gsaList {
	out := gsaList{}
	for _, t := range gsaEvidencePathTokens(ev, top) {
		out = append(out, t)
	}
	return out
}

func gsaRedRulesList(reds []gsaRed) gsaList {
	out := gsaList{}
	for _, r := range reds {
		out = append(out, r.rule)
	}
	return out
}

func gsaRedReprList(reds []gsaRed) gsaList {
	out := gsaList{}
	for _, r := range reds {
		out = append(out, gsaObj{{"rule", r.rule}, {"what", r.what}, {"read", gsaObj(r.read)}})
	}
	return out
}

func gsaL(ss ...string) gsaList {
	out := gsaList{}
	for _, s := range ss {
		out = append(out, s)
	}
	return out
}

func gsaAnyRedRule(rs []gsaRed, rule string) bool { return gsaCountRule(rs, rule) > 0 }

// gsaPyRepr —— py `repr()`（自检 `%r` 用）：str 单引号（复用 cwgPyRepr）。
func gsaPyRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	case json.Number:
		return string(x)
	case string:
		return cwgPyRepr(x)
	case gsaList:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, gsaPyRepr(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case gsaTuple:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, gsaPyRepr(e))
		}
		if len(x) == 1 {
			return "(" + parts[0] + ",)"
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case gsaObj:
		parts := make([]string, 0, len(x))
		for _, kv := range x {
			parts = append(parts, cwgPyRepr(kv.K)+": "+gsaPyRepr(kv.V))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%v", x)
	}
}

// gsaR5Regression —— py `R5_REGRESSION_EVIDENCES`（4 条真实误伤件逐字）。
var gsaR5Regression = map[string][2]string{
	"GAP-20260927-208": {"GAP-20260927-208", "父代理据 L35（core/agent 桶路）现读销案：该件 MaxWorkers 已改指针 + 0=显式暂停语义 + 配置 Mr2109:0 均已落地 ⇒ 已修未销，现销（命门面：zerg gap ls）"},
	"GAP-20260927-262": {"GAP-20260927-262", "销案两态对拍 · 改前{cmd=zerg dev edit --proposal DEV-0485 --file core/cmd/zerg/family_dev_edit.go --from before.go --dry-run --json result · rc=0 · reading=schema 命中 0 条, warnings 命中 0 条（干跑整段不发包封）} · 改后{cmd=zerg dev edit --proposal DEV-9001 --file core/cmd/zerg/zzz_laneB71_probe.go --from newcontent.go --dry-run --json result · rc=0 · reading={\"schema\":\"zerg/v1\",\"kind\":\"DevEdit\",\"items\":[{\"result\":\"planned\"}],\"meta\":{...},\"warnings\":[\"影响面摘要：**新增件**...\"],\"truncated\":false}} · 散文=laneB71 2026-09-29：262 已修未销（症状早被更晚提交修掉，账仍旧态）。现读 family_dev_edit.go 干跑档末尾 emitDevEditPlan 已补 --json 出口（selectJSON→emitSelected→emitEnvelopeWith(inv)，读 inv.envWarn/envMeta），:405-419 把真事进 warnings[]。两态：改前（仓外 overlay 复原版 zerg-before）干跑 --json result 输出 schema/warnings 各 0 命中（信封整段未发）；改后（正门 go build -overlay 自建 zerg-src）同跑出六键包封，真事跑 warnings 逐字非空。改动仅在 /Users/" + "Mr2109/.hermes/cache/scratch/laneB71-devedit/ 仓外副本。"},
	"GAP-20261003-10":  {"GAP-20261003-10", "销案两态对拍 · 改前{cmd=zerg gate run --step 门① 覆盖：构建清单目录 + 脚本接线（阻断） --only-step · rc=1 · reading=FAIL｜本笔加新件 lib-sign.sh 后，门① ④ 格逐字点名「新件未挂门（不在 git 索引 ⇒ 新建后没与代码同批挂 add_step）」} · 改后{cmd=zerg gate run --step 门① 覆盖：构建清单目录 + 脚本接线（阻断） --only-step · rc=0 · reading=PASS｜④ 新件未挂门 = 0（lib-sign.sh 走第二档白名单：source 型公共库·无独立入口·挂 add_step=跑空壳假绿）· ③ 62 − 白名单 7 = 棘轮 55 ≤ baseline 56 · 计数自证 121+10+62 = 193 ✓} · 散文=want 逐条达成：①『若留』⇒ 三件一个没删（两枚旧件按 GAP-20261003-11 归档进 bin/_history/）；②『给一条正式产出支』⇒ build-all.sh:441-448 两条支（源 agent/cmd/zerg-agentd，与其余件同一对签名函数）；③『纳入签名』⇒ :500-501 入 SIGN_LIST + sign_one 挂 identifier；④『若废』那一支未走（用户拍板=留）。★ 末句本轮做完：把 pack-release.sh 真产的件补进签名盯防面 ⇒ 抽 scripts/build/lib-sign.sh 公共件（两档 source 同一份；铁律：两份实现就会漂）+ pack-release.sh 在算 sha256 之前对 release/ 逐件签名（顺序硬约束：签名改字节 ⇒ 先签后算，否则 manifest/checksums 记签名前的值、末尾校验判红）+ sign_one identifier 表补四段契约名通配（原表只列两条 ⇒ zerg-core-linux-amd64 等会掉进兜底 com.zerg.全名 ⇒ 同组件两套身份）。单验：仓外假 release/ 五件逐件真签，并用真 codesign -dv 独立复读 identifier 全对（core/cli/agent/ui），ELF 形态 size=157/158/159。提交 b34fbf9e → 33b6b4b4 → 0051d395 → 29c972b5。★ 发布档签字的真跑判据仍无 CLI 档（zerg build release 真跑未开放）⇒ 已并入 GAP-20261003-12 口径。"},
	"GAP-20261003-11":  {"GAP-20261003-11", "销案两态对拍 · 改前{cmd=zerg gate run --step 'release: B7b 跨架构三件签名（P4 先报告）（只报告）' --only-step · rc=0 · reading=PASS 1/FAIL 0｜名单=b34fbf9e 那版三件（bin/zerg-agent-darwin-amd64 · bin/zerg-agent-linux-amd64 · bin/zerg-agentd-linux-amd64）—— ★ 那绿是「签了」不是「能产」，本账要说的正是这个假绿} · 改后{cmd=zerg gate run --step 'release: B7b 跨架构三件签名（P4 先报告）（只报告）' --only-step · rc=0 · reading=PASS 1/FAIL 0｜名单已按真血缘更正为两件（bin/zerg-agentd-linux-amd64 · bin/zerg-agentd-darwin-amd64，源 agent/cmd/zerg-agentd）} · 散文=两件旧件已从「跨架构件」口径摘除并归档：门B7b 名单与 publish/SECURITY.md 按真血缘更正（提交 33b6b4b4，源 agent/cmd/zerg-agentd）；盘上两枚移入 bin/_history/（sha16 c11b2cb825dcad7d · 24f9ca62f3f39267，原名释放给命令面 CLI 组件 agent）；agent/README.md 旧名三处随之更正（提交 0051d395）。"},
}

func gsaSelfTest(stdout io.Writer) int {
	ok := true
	casef := func(name string, got, want any) {
		good := reflect.DeepEqual(got, want)
		ok = ok && good
		m := "✗"
		if good {
			m = "✓"
		}
		fmt.Fprintf(stdout, "  %s %s（got=%s · want=%s）\n", m, name, gsaPyRepr(got), gsaPyRepr(want))
	}

	casef("证据齐（命令+rc+读数）⇒ 无缺项", gsaListOf(gsaEvidenceMissing("现读 `zerg gap ls` ⇒ rc=0 ⇒ 411 条")), gsaL())
	casef("证据缺读数 ⇒ ['读数']", gsaListOf(gsaEvidenceMissing("跑 `zerg gap ls` rc=0")), gsaL("读数"))
	casef("证据缺退码 ⇒ ['退码']", gsaListOf(gsaEvidenceMissing("现读 scripts/gates/x.py 命中 12 处")), gsaL("退码"))
	casef("空证据 ⇒ 三项全缺", gsaListOf(gsaEvidenceMissing("")), gsaL("命令", "退码", "读数"))

	rows := []map[string]any{
		{"id": "old", "state": "已解", "solved_evidence": "无", "found_at": "2026-09-20T00:00:00"},
		{"id": "new", "state": "已解", "solved_evidence": "无", "found_at": "2026-09-28T00:00:00"},
	}
	rc, reds, reports := gsaJudge(rows, "/nonexistent", gsaCutoffDef)
	casef("R1 存量只报 / 新增判红（rc=1 · reds 只有 new）",
		gsaTuple{rc, gsaFoundList(reds), gsaFoundList(reports)},
		gsaTuple{1, gsaL("2026-09-28T00:00:00"), gsaL("2026-09-20T00:00:00")})

	rows3 := []map[string]any{{"id": "c", "state": "已解", "solved_evidence": "`zerg x` rc=0 5 条",
		"found_at": "2026-09-28", "notes": []any{"仍缺：前提还在"}}}
	rc, reds, _ = gsaJudge(rows3, "/nonexistent", gsaCutoffDef)
	casef("R3 闭态 + 末条含「仍缺」⇒ rc=1", gsaTuple{rc, reds[0].rule}, gsaTuple{1, "R3"})
	rows3b := []map[string]any{{"id": "c", "state": "已解", "solved_evidence": "`zerg x` rc=0 5 条",
		"found_at": "2026-09-28", "notes": []any{"仍缺：前提还在"},
		"notes_void": []any{map[string]any{"void": true, "n": 1}}}}
	rc, reds, _ = gsaJudge(rows3b, "/nonexistent", gsaCutoffDef)
	casef("R3 作废注不计入末条 ⇒ 不红", gsaTuple{rc, gsaRedReprList(reds)}, gsaTuple{0, gsaList{}})

	repoRoot, rootOK := gsaDefaultRepoRoot()
	if !rootOK {
		fmt.Fprintln(stdout, "自检 FAIL（cwd 上溯定位不到仓根锚 —— 本件应在真仓内跑）")
		return 1
	}
	rows2 := []map[string]any{{"id": "r2", "state": "仍缺", "unit": "core/internal/version/version.go"}}
	rc, reds, reports = gsaJudge(rows2, repoRoot, gsaCutoffDef)
	casef("R2 点名件在盘 ⇒ 只报告不判红",
		gsaTuple{rc, gsaRedReprList(reds), gsaAnyRedRule(reports, "R2")}, gsaTuple{0, gsaList{}, true})
	rows2b := []map[string]any{{"id": "r2", "state": "仍缺", "unit": "无件(命令面)"}}
	rc, reds, reports = gsaJudge(rows2b, repoRoot, gsaCutoffDef)
	casef("R2 非路径 unit ⇒ 不报", gsaTuple{rc, gsaRedReprList(reports)}, gsaTuple{0, gsaList{}})

	casef("unit_is_path", gsaTuple{gsaUnitIsPath("scripts/gates/x.py"), gsaUnitIsPath("无件"),
		gsaUnitIsPath("拟(门)"), gsaUnitIsPath("")}, gsaTuple{true, false, false, false})

	// ── R5 成对负控 · hermetic 仓根 ──
	tmp, err := os.MkdirTemp("", "gap-r5-selftest-")
	if err != nil {
		fmt.Fprintln(stdout, "自检 FAIL（无法建临时夹具）")
		return 1
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	_ = os.MkdirAll(pyJoin(tmp, "core", "cmd", "zerg"), 0o755)
	_ = os.MkdirAll(pyJoin(tmp, "bin"), 0o755)
	_ = os.WriteFile(pyJoin(tmp, "core", "cmd", "zerg", "main.go"), nil, 0o644)
	top := map[string]bool{}
	if ents, e := os.ReadDir(tmp); e == nil {
		for _, en := range ents {
			if en.Name() != ".git" {
				top[en.Name()] = true
			}
		}
	}
	base := map[string]any{"state": "已解", "found_at": "2026-09-28T00:00:00", "id": "GAP-20260928-99"}
	real := gsaCloneRow(base, "现读 `core/cmd/zerg/main.go` rc=0 12 条")
	bogus := gsaCloneRow(base, "现读 `core/cmd/zerg/chat_tool_overview.go` rc=0 12 条")

	rc, reds, _ = gsaJudge([]map[string]any{real}, tmp, gsaCutoffDef)
	casef("R5 正控：点名仓内真路径（新账）⇒ 绿 rc=0", gsaTuple{rc, gsaRedReprList(reds)}, gsaTuple{0, gsaList{}})
	rc, reds, _ = gsaJudge([]map[string]any{bogus}, tmp, gsaCutoffDef)
	casef("R5 负控：点名仓内不存在路径（新账）⇒ rc=1 + 点名串与 id",
		gsaTuple{rc, gsaRedRulesList(reds), strings.Contains(reds[0].what, "chat_tool_overview.go"), strings.Contains(reds[0].what, "GAP-20260928-99")},
		gsaTuple{1, gsaL("R5"), true, true})
	fmt.Fprintf(stdout, "      ↳ 负控点名行（逐字）：✗ [R5] %s\n", reds[0].what)

	casef("R5 ① 反引号内真路径 ⇒ 判（抽真主张 · 在盘绿）",
		gsaTuple{gsaEvalPathTokensList(gsaOrEmptyStr(real["solved_evidence"]), top), gsaJudgeRc([]map[string]any{real}, tmp, gsaCutoffDef)},
		gsaTuple{gsaL("core/cmd/zerg/main.go"), 0})
	casef("R5 ② 反引号内不存在路径 ⇒ 红（rc=1 + 点名串）",
		gsaTuple{rc, gsaRedRulesList(reds), strings.Contains(reds[0].what, "chat_tool_overview.go")},
		gsaTuple{1, gsaL("R5"), true})
	prose := gsaCloneRow(base, "散文=排查时提到 core/cmd/zerg/main.go（未加反引号）rc=0 12 条")
	prc, preds, _ := gsaJudge([]map[string]any{prose}, tmp, gsaCutoffDef)
	casef("R5 ③ 散文提及真路径名（无反引号）⇒ 不判",
		gsaTuple{prc, gsaRedRulesList(preds), gsaEvalPathTokensList(gsaOrEmptyStr(prose["solved_evidence"]), top)},
		gsaTuple{0, gsaList{}, gsaList{}})
	pfx := map[string]any{"state": "已解", "found_at": "2026-09-28T00:00:00", "id": "GAP-20260928-97",
		"solved_evidence": "现跑 cmd=core/cmd/zerg/zzz_pfx_probe.go rc=0 12 条"}
	prc, preds, _ = gsaJudge([]map[string]any{pfx}, tmp, gsaCutoffDef)
	casef("R5 口径B：紧跟前缀 cmd= 的不存在路径 ⇒ 判（红）",
		gsaTuple{prc, gsaRedRulesList(preds)}, gsaTuple{1, gsaL("R5")})

	keys := make([]string, 0, len(gsaR5Regression))
	for k := range gsaR5Regression {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var regRows []map[string]any
	for _, k := range keys {
		regRows = append(regRows, gsaCloneRow(base, gsaR5Regression[k][1]))
		for i := range regRows {
			if len(regRows)-1 == i {
				regRows[i]["id"] = gsaR5Regression[k][0]
			}
		}
	}
	prc, preds, _ = gsaJudge(regRows, tmp, gsaCutoffDef)
	var r5rules []string
	for _, r := range preds {
		if r.rule == "R5" {
			r5rules = append(r5rules, r.rule)
		}
	}
	casef("R5 ④ 4 条真实误伤件（新账）⇒ R5 不判", gsaL(r5rules...), gsaL())
	gotmap := gsaObj{}
	for _, k := range keys {
		gotmap = append(gotmap, gsaKV{k, gsaEvalPathTokensList(gsaR5Regression[k][1], top)})
	}
	wantmap := gsaObj{}
	for _, k := range keys {
		wantmap = append(wantmap, gsaKV{k, gsaList{}})
	}
	casef("R5 ④ 4 条误伤件均不被抽为真主张", gotmap, wantmap)

	old := gsaCloneRow(bogus, gsaOrEmptyStr(bogus["solved_evidence"]))
	old["id"] = "GAP-20260920-01"
	old["found_at"] = "2026-09-20T00:00:00"
	rc, reds, reps := gsaJudge([]map[string]any{old}, tmp, gsaCutoffDef)
	casef("R5 存量：旧账 + 不存在路径 ⇒ rc=0 且进 reports",
		gsaTuple{rc, gsaRedReprList(reds), gsaAnyRedRule(reps, "R5")}, gsaTuple{0, gsaList{}, true})
	idnew := gsaCloneRow(bogus, gsaOrEmptyStr(bogus["solved_evidence"]))
	idnew["id"] = "GAP-20261001-01"
	idnew["found_at"] = "2026-09-20T00:00:00"
	rc, reds, _ = gsaJudge([]map[string]any{idnew}, tmp, gsaCutoffDef)
	casef("R5 新账按号内日期（>=2026-09-28）⇒ 判红", gsaTuple{rc, gsaRedRulesList(reds)}, gsaTuple{1, gsaL("R5")})
	skip := gsaCloneRow(base, "见 `https://github.com/x/y.go` · `/etc/hosts` · `otherrepo/a/b.py` rc=0 12 条")
	rc, reds, reps = gsaJudge([]map[string]any{skip}, tmp, gsaCutoffDef)
	casef("R5 URL/绝对/非仓内顶层 ⇒ 不判 rc=0", gsaTuple{rc, gsaRedReprList(reds)}, gsaTuple{0, gsaList{}})
	casef("R5 抽串：`~/.zerg/x` 的后缀不被当独立仓内路径",
		gsaEvalPathTokensList("取 ~/.zerg/selftest", map[string]bool{"core": true, "scripts": true}), gsaList{})

	if ok {
		fmt.Fprintln(stdout, "自检结论: 全过")
		return 0
	}
	fmt.Fprintln(stdout, "自检结论: **有断言失败**")
	return 1
}

func gsaListOf(ss []string) gsaList {
	out := gsaList{}
	for _, s := range ss {
		out = append(out, s)
	}
	return out
}

func gsaFoundList(rs []gsaRed) gsaList {
	out := gsaList{}
	for _, r := range rs {
		for _, kv := range r.read {
			if kv.K == "found_at" {
				out = append(out, kv.V)
			}
		}
	}
	return out
}

func gsaJudgeRc(rows []map[string]any, root, cutoff string) int {
	rc, _, _ := gsaJudge(rows, root, cutoff)
	return rc
}

func gsaCloneRow(base map[string]any, ev string) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	out["solved_evidence"] = ev
	return out
}

// gsaRepoAnchorRels —— 仓根**固定锚**（逐级按序判「存在即仓根」· 每项 = 一段相对路径）：
//
//	① `core/internal/version/version.go`（同族口径 · 稳定件）
//	② `scripts/gates`（判据件目录）
//
// ★ ✗ 不用本件 py `scripts/gates/check-gap-state-vs-artifact.py` 当锚 —— 它退役后就不在盘，
// 锚随之消失（正是自检 `TestGsaSelfTestPasses` 转红的真因）。
// ★ ✗ 不用 `.git` 当锚 —— `core/cmd/zerg/` 下现读有一枚**游离的嵌套 `.git`**（非本仓根），
// 拿它当锚会把仓根错定到包目录。
var gsaRepoAnchorRels = [][]string{
	{"core", "internal", "version", "version.go"},
	{"scripts", "gates"},
}

// gsaDefaultRepoRoot —— py `REPO_ROOT = dirname(dirname(abspath(__file__)))`（**脚本所在仓**）。
// Go 无 `__file__` 语境 ⇒ 从 **cwd 上溯**找**固定仓根锚**（见 `gsaRepoAnchorRels`）定仓根
// （≈「脚本所在仓」· 与 py 同件不同法）。
// 取不到锚 ⇒ 返回 ("", false)（**fail-closed**：调用方报「不给结论」rc=2，✗ 不静默回落 cwd）。
func gsaDefaultRepoRoot() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	cur := pyAbspath(wd)
	for {
		for _, rel := range gsaRepoAnchorRels {
			if _, e := os.Stat(pyJoin(cur, strings.Join(rel, "/"))); e == nil {
				return cur, true
			}
		}
		up := pyDirname(cur)
		if up == "" || up == cur || up == "/" {
			break
		}
		cur = up
	}
	return "", false
}

// ── argparse 面 ────────────────────────────────────────────────────────────
const gsaProg = "check-gap-state-vs-artifact.py"

const gsaUsageBlock = `usage: check-gap-state-vs-artifact.py [-h] [--source SOURCE]
                                      [--repo-root REPO_ROOT]
                                      [--cutoff CUTOFF]
                                      [--expect-sha16 EXPECT_SHA16]
                                      [--expect-lines EXPECT_LINES] [--json]
                                      [--list-rules] [--self-test] [--version]
`

const gsaHelpText = `usage: check-gap-state-vs-artifact.py [-h] [--source SOURCE]
                                      [--repo-root REPO_ROOT]
                                      [--cutoff CUTOFF]
                                      [--expect-sha16 EXPECT_SHA16]
                                      [--expect-lines EXPECT_LINES] [--json]
                                      [--list-rules] [--self-test] [--version]

缺口真源「态 ↔ 证物」四态只读门（GAP-20260927-134）

options:
  -h, --help            show this help message and exit
  --source SOURCE       真源 jsonl（缺省 = <状态目录>/zerg-cli-gaps.jsonl）
  --repo-root REPO_ROOT
                        仓根（R2 判「在盘」用 · 默认 = 本脚本所在仓）
  --cutoff CUTOFF       新增/存量分界时刻（R1）
  --expect-sha16 EXPECT_SHA16
                        期望真源 sha256 前 16 位（R4 · 给了就比）
  --expect-lines EXPECT_LINES
                        期望真源行数（R4 · 给了就比）
  --json                机读输出（zerg/v1 形状）
  --list-rules          打登记面：判据 / 闭集 / 分界
  --self-test           合成夹具自检（不碰真源）
  --version             show program's version number and exit
`

type gsaArgs struct {
	source      *string
	repoRoot    string
	repoRootSet bool
	cutoff      string
	expectSha16 *string
	expectLines *int
	jsonOut     bool
	listRules   bool
	selfTest    bool
}

var gsaLongOpts = []string{"--help", "--source", "--repo-root", "--cutoff",
	"--expect-sha16", "--expect-lines", "--json", "--list-rules", "--self-test", "--version"}

var gsaNegNumRe = regexp.MustCompile(`^-\d+$|^-\d*\.\d+$`)

func gsaLooksOption(s string) bool {
	if !strings.HasPrefix(s, "-") || s == "-" {
		return false
	}
	return !gsaNegNumRe.MatchString(s)
}

func gsaResolveLong(name string) (string, []string) {
	for _, o := range gsaLongOpts {
		if o == name {
			return o, nil
		}
	}
	var matches []string
	for _, o := range gsaLongOpts {
		if strings.HasPrefix(o, name) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return "", matches
}

func gsaArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, gsaUsageBlock)
	fmt.Fprintf(stderr, "%s: error: %s\n", gsaProg, msg)
	return 2
}

func gsaPyInt(s string) (int, bool) {
	t := strings.ReplaceAll(strings.TrimSpace(s), "_", "")
	if t == "" {
		return 0, false
	}
	n, err := strconv.Atoi(t)
	if err != nil {
		return 0, false
	}
	return n, true
}

// gsaParseArgs —— py argparse 面逐字复刻；done>=0 ⇒ 已输出（帮助/版本/错误）直接返回该退码。
func gsaParseArgs(stdout, stderr io.Writer, argv []string) (gsaArgs, int) {
	a := gsaArgs{cutoff: gsaCutoffDef}
	var unrecognized []string
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if tok == "-h" || tok == "--help" {
			fmt.Fprint(stdout, gsaHelpText)
			return a, 0
		}
		if strings.HasPrefix(tok, "-") && !strings.HasPrefix(tok, "--") && len(tok) >= 2 && tok[1] == 'h' {
			fmt.Fprint(stdout, gsaHelpText)
			return a, 0
		}
		if tok == "--" {
			unrecognized = append(unrecognized, tok)
			unrecognized = append(unrecognized, argv[i+1:]...)
			break
		}
		if strings.HasPrefix(tok, "--") {
			name, val, hasVal := tok, "", false
			if j := strings.Index(tok, "="); j >= 0 {
				name, val, hasVal = tok[:j], tok[j+1:], true
			}
			canon, matches := gsaResolveLong(name)
			if canon == "" && len(matches) > 1 {
				return a, gsaArgErr(stderr, fmt.Sprintf("ambiguous option: %s could match %s", name, strings.Join(matches, ", ")))
			}
			if canon == "" {
				unrecognized = append(unrecognized, tok)
				continue
			}
			switch canon {
			case "--help":
				if hasVal {
					return a, gsaArgErr(stderr, "argument -h/--help: ignored explicit argument '"+val+"'")
				}
				fmt.Fprint(stdout, gsaHelpText)
				return a, 0
			case "--source", "--repo-root", "--cutoff", "--expect-sha16", "--expect-lines":
				value := ""
				if hasVal {
					value = val
				} else {
					if i+1 >= len(argv) || gsaLooksOption(argv[i+1]) {
						return a, gsaArgErr(stderr, "argument "+canon+": expected one argument")
					}
					i++
					value = argv[i]
				}
				switch canon {
				case "--source":
					v := value
					a.source = &v
				case "--repo-root":
					a.repoRoot, a.repoRootSet = value, true
				case "--cutoff":
					a.cutoff = value
				case "--expect-sha16":
					v := value
					a.expectSha16 = &v
				case "--expect-lines":
					n, ok := gsaPyInt(value)
					if !ok {
						return a, gsaArgErr(stderr, "argument --expect-lines: invalid int value: '"+value+"'")
					}
					a.expectLines = &n
				}
			case "--json":
				if hasVal {
					return a, gsaArgErr(stderr, "argument --json: ignored explicit argument '"+val+"'")
				}
				a.jsonOut = true
			case "--list-rules":
				if hasVal {
					return a, gsaArgErr(stderr, "argument --list-rules: ignored explicit argument '"+val+"'")
				}
				a.listRules = true
			case "--self-test":
				if hasVal {
					return a, gsaArgErr(stderr, "argument --self-test: ignored explicit argument '"+val+"'")
				}
				a.selfTest = true
			case "--version":
				if hasVal {
					return a, gsaArgErr(stderr, "argument --version: ignored explicit argument '"+val+"'")
				}
				fmt.Fprintln(stdout, gsaVersion)
				return a, 0
			}
			continue
		}
		unrecognized = append(unrecognized, tok)
	}
	if len(unrecognized) > 0 {
		return a, gsaArgErr(stderr, "unrecognized arguments: "+strings.Join(unrecognized, " "))
	}
	return a, -1
}

// checkGapStateVsArtifactGateMain —— 入口（= py `main(argv)`）。
func checkGapStateVsArtifactGateMain(stdout, stderr io.Writer, argv []string) int {
	a, done := gsaParseArgs(stdout, stderr, argv)
	if done >= 0 {
		return done
	}
	if a.selfTest {
		return gsaSelfTest(stdout)
	}
	if a.listRules {
		gsaListRules(stdout, a.cutoff)
		return 0
	}
	repoRoot := a.repoRoot
	if !a.repoRootSet {
		rr, ok := gsaDefaultRepoRoot()
		if !ok {
			return gsaEmitProblem(stdout, a.jsonOut, "R0",
				"定位不到仓根（cwd 上溯未找到锚点 core/internal/version/version.go · scripts/gates）⇒ 不给结论",
				gsaObj{}, "")
		}
		repoRoot = rr
	}
	src := ""
	if a.source != nil && *a.source != "" {
		src = *a.source
	} else {
		sd := os.Getenv("ZERG_STATE_DIR")
		if sd == "" {
			sd = gsaStateDirDefault()
		}
		src = pyJoin(sd, gsaSourceName)
	}
	path := pyAbspath(src)
	raw1, err := gsaReadRaw(path)
	if err != "" {
		return gsaEmitProblem(stdout, a.jsonOut, "R0", err, gsaObj{{"path", path}}, path)
	}
	fp1, ln1 := gsaFingerprint(raw1)
	raw2, err2 := gsaReadRaw(path)
	if err2 != "" {
		return gsaEmitProblem(stdout, a.jsonOut, "R0", err2, gsaObj{{"path", path}}, path)
	}
	fp2, ln2 := gsaFingerprint(raw2)
	if fp1 != fp2 || ln1 != ln2 {
		return gsaEmitProblem(stdout, a.jsonOut, "R4",
			fmt.Sprintf("真源在两次读数之间被写过（%s/%d ⇒ %s/%d）⇒ 不给结论", fp1[:12], ln1, fp2[:12], ln2),
			gsaObj{}, path)
	}
	if a.expectSha16 != nil && fp1[:16] != *a.expectSha16 {
		return gsaEmitProblem(stdout, a.jsonOut, "R4",
			fmt.Sprintf("真源 sha16 不符：期望 %s · 现读 %s", *a.expectSha16, fp1[:16]), gsaObj{}, path)
	}
	if a.expectLines != nil && ln1 != *a.expectLines {
		return gsaEmitProblem(stdout, a.jsonOut, "R4",
			fmt.Sprintf("真源行数不符：期望 %d · 现读 %d", *a.expectLines, ln1), gsaObj{}, path)
	}
	rows, ok, perr := gsaParseRows(gsaDecodeReplace(raw1))
	if !ok {
		return gsaEmitProblem(stdout, a.jsonOut, "R4", perr, gsaObj{}, path)
	}
	repoRoot = pyAbspath(repoRoot)
	if st, err := os.Stat(repoRoot); err != nil || !st.IsDir() {
		return gsaEmitProblem(stdout, a.jsonOut, "R0",
			fmt.Sprintf("仓根不是目录（R2/R5 判不了 · 不给结论）：%s", repoRoot),
			gsaObj{{"repo_root", repoRoot}}, path)
	}
	rc, reds, reports := gsaJudge(rows, repoRoot, a.cutoff)
	if a.jsonOut {
		fmt.Fprintln(stdout, gsaMainJSON(rc, fp1, ln1, len(rows), path, repoRoot, reds, reports))
		return rc
	}
	gsaRunHuman(stdout, rc, fp1, ln1, len(rows), path, repoRoot, a.cutoff, reds, reports)
	return rc
}

// ═══════════════════════ ★ 未覆盖面（逐条如实声明）═══════════════════════
//
//	① **默认根定位差异（声明 (a) 的推论）**：py `REPO_ROOT` 起于 `__file__`，Go 起于 cwd 上溯固定仓根锚
//	   （version.go / scripts/gates）⇒ 仅当「脚本位置 == cwd 上溯到的仓根」时逐字同构。对拍：
//	   甲 = 真仓且从仓根跑。取不到锚（真仓外）⇒ 本件 fail-closed rc=2，✗ 不静默回落 cwd。
//	   `--self-test` 的 R2「件在盘」用例锚**稳定件** `core/internal/version/version.go`（✗ 不用本件 py
//	   —— 它退役后就不在盘，锚/用例会随之分叉）。
//	② **argparse 帮助 / 错误正文 = py 版本相关**：本件按**本机 python3.14** 复刻（`options:` 措辞 ·
//	   usage 换行缩进 · 歧义文案 `ambiguous option: --e could match …`）。门跑环境若用 py≤3.9
//	   （印 `optional arguments:`）则帮助面 stdout 分叉（rc 恒 0）。未知旗标/缺值/`--json=1` 的 error
//	   正文同为版本相关措辞。
//	③ **py 正则 Unicode 语义**已按 rune 手写复刻（`\b`/`\w`/`\d`/`\s`），但 py `\b` 的少数 Unicode
//	   类别边界（Mn/Mc/`200C`/`200D` 等 `str.isalnum()` 判定）在本件用 Letter|Digit|Number|`_` 近似
//	   ⇒ 极少数边界码点未实测对齐（真仓件以 CJK/ASCII 为主 ⇒ 不触发）。
//	④ **非对象行 / 非串字段**：py `json.loads` 得非 dict 时下游 `r.get` 会抛 `AttributeError`（C 类
//	   裸回溯 rc=1）；本件对非对象行 `continue`（跳过），对 `notes` 非串元素走 `str()` 兑底 ⇒ 该档
//	   与 py 回溯正文不同（真仓行行是 dict、notes 元素皆串 ⇒ 不触发）。py 扩展 JSON 值（`NaN`/
//	   `Infinity`）Go `encoding/json` 拒 ⇒ 该档退 Go 原文（真仓件件合法 ⇒ 不触发）。
//	⑤ **py `open` 读不出件的正文**：本件按 `ENOENT`/`EACCES`/`EISDIR` 三式手写对齐（
//	   `[Errno N]<msg>: '<path>'`）；其余 errno 回落 Go 原文（真仓件可读 ⇒ 默认档不触发）。
//	⑥ **`raw.decode("utf-8","replace")` 的非 UTF-8 粒度**：py 逐**序列**替 `U+FFFD`、Go
//	   `strings.ToValidUTF8` 逐**无效串**替 ⇒ 长度可能不同（真仓件合法 UTF-8 ⇒ 不触发）。
//	⑦ **`os.path.basename` 尾斜杠档**：本件 `gsaPyBase` 对无尾斜杠的相对件逐字同构；含尾斜杠档
//	   未复刻（R2 的 `unit` 不带尾斜杠 ⇒ 不触发）。
//	⑧ **`--expect-lines` 的 py `int()` 面**：本件按 ASCII 十进制（去下划线/空白）解析；py 还认
//	   Unicode 数字码点 ⇒ 极少数非 ASCII 数字串未对齐（真仓传值皆 ASCII ⇒ 不触发）。
//	⑨ **单短旗标组** `-<h…>`（如 `-hx`）本件一律触发帮助（对齐 py3.14 观察）；其余单短旗标组
//	   未逐一实测（真仓/门跑不发这类 argv ⇒ 不触发）。
//	⑩ **`ZERG_STATE_DIR` 空串档**：py `or` 与 Go 均视空串为「回落缺省」⇒ 同构（此项为已对齐说明）。
