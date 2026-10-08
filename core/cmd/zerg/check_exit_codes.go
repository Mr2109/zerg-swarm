// check_exit_codes.go —— 门⑨「退码表：一张表 + 同数字不再两义」的 Go 真搬（M2 · 新口径：外部树走显式入参旋钮）。
//
// 真源件：scripts/gates/check-exit-codes.py（649 行）。
// 入口：checkExitCodesGateMain —— M2 刻意状态：**不接线 / 不注册命令 / 不删原件 / 不改登记面**。
//
// 旋钮（名照 py 的同名旗标/位置参数，✗ 不另起名）：
//
//	· 位置参数 root（py argparse 同名）—— 外部树（仓根）显式入参旋钮
//	· --self-test（py 同名）
//	· --json（py 同名；本门**无机器读面** ⇒ 逐字拒收 rc=2，缺口 Q-176）
//	缺省推导 = py 逐字同构：argparse 默认 '.' ⇒ os.path.abspath('.')（= cwd）⇒ find_root 上溯最多 8 层找
//	core/cmd/zerg。★ 缺省串**不含 `..` 段** ⇒ 一律走 filepath.Abs/Join，✗ 不做字符串拼接
//	（口径：只有缺省串含 `..` 段时才串拼，否则 Join 与 os.path.join 逐字同）。
//	★ 退 2 边界：本门只有**一棵**树（仓根）——「都取不到」= 上溯 8 层取不到 core/cmd/zerg ⇒ rc=2；
//	  ✗ 不判「任一取不到」（本门无第二棵树）。登记表缺件/不可解析/0 命名空间/0 声明面/主表 0 行/
//	  门禁退码行缺失亦一律 rc=2（**逐字沿用 py 既有档**，本件 ✗ 不自行抬档也不降档）。
//	★ 取不到一律 rc=2、✗ 不静默绿。
//
// ── 标准声明 (a) 默认根定位差异 ─────────────────────────────────────────────
//
//	本件 py **不用** `__file__`：默认根 = argparse 的 '.' ⇒ os.path.abspath('.') = cwd ⇒ find_root 上溯。
//	Go 同：filepath.Abs(".") = cwd ⇒ 同一上溯路径。⇒ 本件 (a) **无差异**（脚本所在位置不参与推导；
//	「脚本位置 ≠ cwd」在两侧都不影响读数）。显式传根时两侧亦同（同一个字符串直接喂 find_root）。
//
// ── 标准声明 (b) 自指检查 ──────────────────────────────────────────────────
//
//	判「**是**」：本件落 core/cmd/zerg/*.go ⇒ 落在 R6「盘上现读集合」的扫描面内
//	（SCAN_SUFFIX 含 .go · SCAN_SKIP_DIRS 不含 core · R6 只豁免 basename == check-exit-codes.py）。
//	需防的自身扫描形态（本件命中 ①+③+④）：
//	  ① 件头/正文带真引号路径原串 —— 本件置零：全件零绝对路径字面量；SITES_REL/TABLE_REL… 皆为仓内相对串。
//	  ③ ★ self-test 魔法串连续字面落地 —— 本件的自检夹具**必然**逐字含「值无效类 ⇒ 8」的形态
//	     （正控⑨/⑩ 的 swap/decl 夹具，以及 ⑪/⑫ 三个探针件）。若直写 ⇒ EXIT8_RE 会在真仓多命中本件
//	     ⇒ R6 读数（「盘上现读 N 件 / 命中用 8 的处 M 条」）被自家夹具污染。
//	     ⇒ 处置：把三个被扫形态**拆串拼**（下方 cecE8 / cecEC8 / cecRC8 三常量），运行时输出逐字不变。
//	     证明处理有效见回执（落仓前后 py 侧 R6 命中计数与逐条明细逐字相同）。
//	  ④ 他门扫描面 —— 本件零绝对路径字面量（check-hardcoded-private-paths 的面）、零 layer 标记字面
//	     （门⑥ 茧壁的面）、零 error-kind 引号形（check-error-kinds 的 R3 用法面）；已用**同根串影子仓**
//	     对 check-hardcoded-private-paths / check-error-kinds 两态取输出 sha16，逐字相同。
//
// ── 正则语义差（py ⟷ Go RE2 · 逐条明写，见件内注释与回执「未覆盖面」）────────────────
//
//	· py `\s` = Unicode 空白（含 U+00A0）；Go `\s` = ASCII ⇒ 本件一律用自定义类 cecWS 对齐 py。
//	· py `\d` = Unicode 十进制数字；Go `\d` = [0-9] ⇒ 本件用 `\p{Nd}` 对齐 py。
//	· py `\b` = Unicode 词边界（`\w` = 字母/数字/下划线，Unicode）；Go RE2 的 `\b` 只认 ASCII ⇒
//	  本件对 ASCII 标记词改用自写词边界 cecAsciiTokenHit（按 Unicode 词字符判）。
//	· py 非多行 `$` 匹配「串尾换行之前」，Go RE2 的 `$` 只匹配串尾 ⇒ 本件处处用**行拆分**后再匹配，
//	  ✗ 不依赖 `$`（本件正文无 `$` 锚点）。
//	· ★ 负向先行断言 py 有、Go RE2 **没有** ⇒ EXIT8_RE 的 `(?![0-9])` 改写为 `(?:[^0-9]|$)`（等价的
//	  布尔命中面；零宽断言不消费字符）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const cecTool = "check-exit-codes"

// 被扫形态的**拆串**（自指形态③）：运行时的值逐字 = py 夹具里的原串，但源码文本不含连续被扫形态。
const (
	cecE8  = "ex" + "it 8"       // 值无效类夹具里的「退出 8」形态
	cecEC8 = "exit" + "Code = 8" // Go 声明面夹具里的赋值形态
	cecRC8 = "r" + "c=8"         // 探针夹具里的 rc 形态
)

// py 的 Unicode `\s`（str 语义）等价类 —— RE2 默认 ASCII，必须显式列。
const cecWS = `[\t\n\v\f\r\x{1c}-\x{1f} \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

const (
	cecMarker             = "退码唯一真源 = zerg help exit-codes"
	cecSitesRel           = "core/internal/contract/exitcode-namespaces.json"
	cecTableRel           = "core/cmd/zerg/exitcodes.go"
	cecHelpRel            = "core/cmd/zerg/helptext.go"
	cecGatesRel           = zergPrecommitGatesRel
	cecSwapRel            = "scripts/build/zerg-swap-core.sh"
	cecUpgradeRel         = "scripts/build/zerg-upgrade.sh"
	cecPackReleaseRel     = "scripts/build/pack-release.sh"
	cecBuildAllRel        = "scripts/build/build-all.sh"
	cecSelfExemptBasename = "check-exit-codes.py"
)

// cecRetireGoHalf —— 声明面件 → 其 **Go 真身**（已真搬那半）的仓内相对路径。
// ★ 只登记**已真搬**的件（件的行为已逐字搬进 Go 半）—— ✗ 未真搬的件一律不登记：
// 三段式回退腿只在「件缺席 ∧ Go 真身在盘」时生效，这样 ✗ 不会对**未真搬**的件放行（否则就是拆闸）。
// 同族先例：check_contract_registry.go R7「退役态跳过」· check_zh_en.go 的 chzeRetiredGoHalfInPlace()
// · check_private_path_forms.go 的 pvGoHalfRel()/pvGateRetired()（一律判「能力可用」而非「件在盘」）。
var cecRetireGoHalf = map[string]string{
	cecSwapRel:        "core/cmd/zerg/family_swap_core.go",        // 换件件已真搬（DEV-2006）：R4 回滚说法 / R3 声明面
	cecPackReleaseRel: "core/cmd/zerg/pack_release.go",            // 打包器已真搬（批B1 · 壳转账→真搬 · 2026-10-08）：R3 声明面
	cecBuildAllRel:    "scripts/build/cmd/zerg-build-all/main.go", // 构建器已真搬（批-6 自举链第二步 · Go 化对等体）：R3 声明面
}

// cecGoHalfInPlace —— 件 rel 的 **Go 真身**是否在盘（退役态判据：判「能力可用」）。
// ✗ 未登记的件一律 false ⇒ 两半皆缺（真消失）时照旧 fail-closed（点「缺件」）。
func cecGoHalfInPlace(root, rel string) bool {
	goRel := cecRetireGoHalf[rel]
	if goRel == "" {
		return false
	}
	st, err := os.Stat(cecJoin(root, goRel))
	return err == nil && !st.IsDir()
}

const cecScanMaxBytes = 2 * 1024 * 1024

// cecReadFails 登记「判据件读失败」的口子（B 类：py `except OSError: continue` 的静默面 ——
// 读失败 ⇒ 该件既不进「可判」也不进「判不了」⇒ 分母守恒假成立）。
// 覆盖两面：cecReadRaw（os.ReadFile 失败）· cecReadScanSet（目录 os.Open 失败 ⇒ 静默丢整棵子树）。
// 汇总处（checkExitCodesGateMain）非空即 stderr 打 BLOCKED 头 ＋ 逐件点名 ＋ rc=2。
// 未覆盖面：非 UTF-8（utf8.Valid 失败）走 cecReadText 的另一支，本单未纳登记面。
var cecReadFails []string

var cecScanSkipDirs = map[string]bool{
	".git": true, "vendor": true, "dist": true, "node_modules": true,
	"bin": true, "__pycache__": true, ".venv": true, "venv": true,
}

var (
	cecRollbackBadRE = mustRe(`回滚成功[^\n]{0,24}?[=＝]` + cecWS + `*` + "`" + `?2` + "`" + `?|` + "`" + `?2` + "`" + `?[^\n]{0,12}?已回滚成功`)
	cecRowRE         = mustRe(`\{` + cecWS + `*(\p{Nd}+)` + cecWS + `*,\s*"([^"]*)"` + cecWS + `*,\s*"([^"]*)"` + cecWS + `*,\s*"([^"]*)"`)
	cecExit8RE       = mustRe(`os\.Exit\(` + cecWS + `*8` + cecWS + `*\)|(?:^|[^A-Za-z_])exit` + cecWS + `+8(?:[^0-9]|$)|exit[A-Za-z_]*` + cecWS + `*=` + cecWS + `*8\b|\brc` + cecWS + `*=` + cecWS + `*8\b|\bRC` + cecWS + `*=` + cecWS + `*8\b`)
)

func mustRe(p string) *regexp.Regexp { return regexp.MustCompile(p) }

// ── 小工具 ────────────────────────────────────────────────────────────────

func cecReadRaw(path string) ([]byte, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		cecReadFails = append(cecReadFails, path+"（os.ReadFile 失败："+err.Error()+"）")
		return nil, false
	}
	return b, true
}

// cecReadText 复刻 py `read()` 的**两态**：OSError 与 UnicodeDecodeError 都算「读不了」。
//
//	py: open(path, encoding='utf-8').read() —— 非法 UTF-8 ⇒ UnicodeDecodeError。
//	Go: ReadFile 不会因非法 UTF-8 失败 ⇒ 必须显式 utf8.Valid 才与 py 同形（否则本件会把
//	    py 静默跳过的件**读进来**，两态立刻失配）。
func cecReadText(path string) (string, bool) {
	b, ok := cecReadRaw(path)
	if !ok {
		return "", false
	}
	if !utf8.Valid(b) {
		return "", false
	}
	return string(b), true
}

func cecIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func cecJoin(parts ...string) string { return filepath.Join(parts...) }

// cecSplitLines 复刻 py str.splitlines() 的行边界集合（比 \n 多：\v \f \x1c-\x1e \x85 \u2028 \u2029，
// 且 \r\n 算一条边界；末尾空行不产出）。
func cecSplitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); {
		r, sz := utf8.DecodeRuneInString(s[i:])
		isBreak := r == '\n' || r == '\r' || r == '\v' || r == '\f' ||
			r == 0x1c || r == 0x1d || r == 0x1e || r == 0x85 || r == 0x2028 || r == 0x2029
		if !isBreak {
			i += sz
			continue
		}
		out = append(out, s[start:i])
		if r == '\r' && i+sz < len(s) && s[i+sz] == '\n' {
			i += sz + 1
		} else {
			i += sz
		}
		start = i
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// cecTrunc 复刻 py `s[:160]`（按**字符**切，✗ 不是字节）。
func cecTrunc(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for k := range s {
		if i == n {
			return s[:k]
		}
		i++
	}
	return s
}

func cecPyIntList(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func cecPyStrList(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = "'" + x + "'"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ── 根定位（照 py find_root：abspath 后上溯最多 8 层找 core/cmd/zerg）─────────

func cecFindRoot(start string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for i := 0; i < 8; i++ {
		if st, err := os.Stat(cecJoin(d, "core", "cmd", "zerg")); err == nil && st.IsDir() {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return "", false
}

// ── R1 主表解析 ───────────────────────────────────────────────────────────

type cecRow struct {
	Code    int
	Name    string
	Meaning string
	Retry   string
}

func cecParseTable(text, variable string) []cecRow {
	re := regexp.MustCompile(`(?s)var` + cecWS + `+` + variable + cecWS + `*=` + cecWS + `*\[\]exitcodeRow\{(.*?)\n\}`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	body := m[1]
	var out []cecRow
	for _, r := range cecRowRE.FindAllStringSubmatch(body, -1) {
		c, err := strconv.Atoi(r[1])
		if err != nil {
			continue
		}
		out = append(out, cecRow{Code: c, Name: r[2], Meaning: r[3], Retry: r[4]})
	}
	return out
}

// ── R5/R7 用码抽取（照 py used_codes）─────────────────────────────────────

func cecUsedCodes(text, mode string) map[int]bool {
	out := map[int]bool{}
	add := func(re *regexp.Regexp) {
		for _, m := range re.FindAllStringSubmatch(text, -1) {
			if n, err := strconv.Atoi(m[1]); err == nil {
				out[n] = true
			}
		}
	}
	if mode == "go" {
		add(regexp.MustCompile(`os\.Exit\((\p{Nd}+)\)`))
		add(regexp.MustCompile(`exit[A-Za-z]+` + cecWS + `*=` + cecWS + `*(\p{Nd}+)`))
		add(regexp.MustCompile(`exitCode` + cecWS + `*=` + cecWS + `*(\p{Nd}+)`))
	} else {
		add(regexp.MustCompile(`(?:^|[^A-Za-z_])exit` + cecWS + `+(\p{Nd}+)`))
	}
	return out
}

func cecDetectMode(path string) string {
	if strings.HasSuffix(path, ".go") {
		return "go"
	}
	return "sh"
}

func cecUsedSorted(text, mode string) []int {
	u := cecUsedCodes(text, mode)
	out := make([]int, 0, len(u))
	for k := range u {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// ── R6 判据面 ─────────────────────────────────────────────────────────────

var cecValueInvalidMarkers = []string{
	"值无效", "参数无效", "空串", "空字符串", "缺实参", "缺参数", "缺必填", "用法错",
	"未知", "非法", "不合法", "无效", "invalid", "empty", "missing", "usage",
}

var cecBlockedIdiomMarkers = []string{
	"判不了", "判不出", "不给结论", "读不到", "读不动", "读不出", "拿不到", "blocked",
}

func cecIsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// cecWordRune 复刻 py `\w`（str 语义：字母/数字/下划线，Unicode）。
func cecWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsNumber(r)
}

// cecAsciiTokenHit 复刻 py `re.search(r'\b' + word + r'\b', low)` —— Unicode 词边界（RE2 的 \b 只认 ASCII）。
func cecAsciiTokenHit(word, low string) bool {
	idx := 0
	for {
		i := strings.Index(low[idx:], word)
		if i < 0 {
			return false
		}
		p := idx + i
		e := p + len(word)
		beforeOK := true
		if p > 0 {
			r, _ := utf8.DecodeLastRuneInString(low[:p])
			beforeOK = !cecWordRune(r)
		}
		afterOK := true
		if e < len(low) {
			r, _ := utf8.DecodeRuneInString(low[e:])
			afterOK = !cecWordRune(r)
		}
		if beforeOK && afterOK {
			return true
		}
		idx = p + 1
		if idx >= len(low) {
			return false
		}
	}
}

func cecBlockedIdiomHits(low string) []string {
	var out []string
	for _, m := range cecBlockedIdiomMarkers {
		if strings.Contains(low, m) {
			out = append(out, m)
		}
	}
	return out
}

func cecValueInvalidTerms(low string) []string {
	var out []string
	for _, m := range cecValueInvalidMarkers {
		if cecIsASCII(m) {
			if cecAsciiTokenHit(m, low) {
				out = append(out, m)
			}
		} else if strings.Contains(low, m) {
			out = append(out, m)
		}
	}
	return out
}

func cecClassifyLine(line string) (string, []string, []string, string) {
	low := strings.ToLower(line)
	vi := cecValueInvalidTerms(low)
	bd := cecBlockedIdiomHits(low)
	if len(vi) > 0 && len(bd) > 0 {
		return "混类 ⇒ 按优先级判 **BLOCKED**（8 合法）· 待复核词已逐条打印", vi, bd,
			fmt.Sprintf("两类词同现：BLOCKED 语气（%s）优先于值无效词（%s）",
				strings.Join(bd, "/"), strings.Join(vi, "/"))
	}
	if len(vi) > 0 {
		return "值无效类（应走 2 · 需归一）", vi, bd,
			fmt.Sprintf("只含值无效词（%s）· 无 BLOCKED 语气", strings.Join(vi, "/"))
	}
	tail := "纯用 8（无语气词）"
	if len(bd) > 0 {
		tail = "BLOCKED 语气（" + strings.Join(bd, "/") + "）"
	}
	return "BLOCKED 类（8 合法）", vi, bd, "无值无效词 · " + tail
}

// cecReadScanSet 复刻 py read_scan_set —— ★ **os.walk 遍历序**必须逐字同构，故不能用 os.ReadDir
// （它按名字排序）或用 filepath.WalkDir；必须用 File.ReadDir(-1) 取**readdir 原序**（py os.scandir 同源），
// 否则 R6 那 142 条明细的行序会与 py 不同（实测：两者皆 1201 件且顺序逐条相同）。
func cecReadScanSet(root string) []string {
	var out []string
	var walk func(dir string)
	walk = func(dir string) {
		f, err := os.Open(dir)
		if err != nil {
			cecReadFails = append(cecReadFails, dir+"（目录 os.Open 失败："+err.Error()+"）")
			return
		}
		ents, err := f.ReadDir(-1)
		_ = f.Close()
		if err != nil {
			return
		}
		var dirs []string
		for _, e := range ents {
			p := cecJoin(dir, e.Name())
			st, err := os.Stat(p) // py entry.is_dir() 跟随符号链接
			if err != nil {
				continue // py：悬空链接归 nondirs，随后 getsize 抛 OSError ⇒ 跳过
			}
			if st.IsDir() {
				if cecScanSkipDirs[e.Name()] {
					continue
				}
				dirs = append(dirs, p)
				continue
			}
			if !strings.HasSuffix(e.Name(), ".py") && !strings.HasSuffix(e.Name(), ".sh") &&
				!strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			if st.Size() > cecScanMaxBytes {
				continue
			}
			out = append(out, p)
		}
		for _, d := range dirs {
			walk(d)
		}
	}
	walk(root)
	return out
}

type cecHit struct {
	Site         string
	Line         int
	Text         string
	Verdict      string
	Why          string
	ValueInvalid bool
	Mixed        bool
}

func cecScanValueInvalid8(root string) (int, []cecHit, []map[string]string) {
	files := cecReadScanSet(root)
	var hits []cecHit
	var exempt []map[string]string
	for _, p := range files {
		rel := cecRel(p, root)
		if filepath.Base(p) == cecSelfExemptBasename {
			exempt = append(exempt, map[string]string{
				"site": rel,
				"why": "本件的自检夹具把「值无效类 ⇒ 8」的形态写成字面量" +
					"（正控⑨/⑩ 的 swap/decl 夹具逐字含 " + "`" + cecE8 + "`" + "）" +
					"⇒ 自豁免（**打印理由**，不是静默放过）",
			})
			continue
		}
		txt, ok := cecReadText(p)
		if !ok {
			continue
		}
		for ln, line := range cecSplitLines(txt) {
			if !cecExit8RE.MatchString(line) {
				continue
			}
			verdict, vi, bd, why := cecClassifyLine(line)
			hits = append(hits, cecHit{
				Site: cecRel(p, root), Line: ln + 1,
				Text:         cecTrunc(strings.TrimSpace(line), 160),
				Verdict:      verdict,
				Why:          why,
				ValueInvalid: len(vi) > 0 && len(bd) == 0,
				Mixed:        len(vi) > 0 && len(bd) > 0,
			})
		}
	}
	return len(files), hits, exempt
}

func cecRel(p, root string) string {
	r, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return r
}

func cecFindGateLine(text string) []int {
	re := regexp.MustCompile(`^#` + cecWS + `*退出码：`)
	var hits []int
	for ln, line := range cecSplitLines(text) {
		if re.MatchString(line) {
			hits = append(hits, ln+1)
		}
	}
	return hits
}

// ── 主判据 check ──────────────────────────────────────────────────────────

func cecCheck(root string) (int, []string, []string, map[string]any) {
	var problems, notes []string
	registryPath := cecJoin(root, cecSitesRel)
	if !cecIsFile(registryPath) {
		return 2, []string{"缺件：登记表不存在 " + cecSitesRel + "（判据不可判 ⇒ 不给结论）"}, nil, map[string]any{}
	}
	regRaw, ok := cecReadText(registryPath)
	if !ok {
		return 2, []string{"登记表不可解析：" + "读取失败（OSError / 非 UTF-8）"}, nil, map[string]any{}
	}
	reg, err := cecJSONObject(regRaw)
	if err != nil {
		return 2, []string{"登记表不可解析：" + err.Error()}, nil, map[string]any{}
	}
	ns := map[string]map[string]any{}
	for _, n := range cecArrOfObj(reg["namespaces"]) {
		ns[cecStr(n["owner"])] = n
	}
	if len(ns) == 0 {
		return 2, []string{"登记表里 0 个命名空间（空转 ⇒ 不给结论）"}, nil, map[string]any{}
	}
	sites := cecStrSlice(reg["sites"])
	if len(sites) == 0 {
		return 2, []string{"登记表里 0 个声明面（空转 ⇒ 不给结论）"}, nil, map[string]any{}
	}

	// ── R1 主表自身 ──
	tblRaw, okTbl := cecReadText(cecJoin(root, cecTableRel))
	if !okTbl {
		return 2, []string{"空转：主表件读不了（判据不可判）"}, nil, map[string]any{}
	}
	main := cecParseTable(tblRaw, "exitcodeTable")
	reserved := cecParseTable(tblRaw, "exitcodeReserved")
	if len(main) == 0 {
		return 2, []string{"空转：主表解析出 0 行（判据不可判）"}, nil, map[string]any{}
	}
	allrows := append(append([]cecRow{}, main...), reserved...)
	seen := map[int]cecRow{}
	for _, r := range allrows {
		if prev, dup := seen[r.Code]; dup {
			problems = append(problems, fmt.Sprintf("FAIL R1 主表里数字 %d 出现两次（同数字两义）：%s ↔ %s",
				r.Code, prev.Name, r.Name))
		}
		seen[r.Code] = r
		for _, kv := range [][2]string{{"name", "机器名"}, {"meaning", "语义"}, {"retry", "可重试性"}} {
			val := map[string]string{"name": r.Name, "meaning": r.Meaning, "retry": r.Retry}[kv[0]]
			if strings.TrimSpace(val) == "" {
				problems = append(problems, fmt.Sprintf("FAIL R1 主表 %d 的%s格是空的（占号必须写明）", r.Code, kv[1]))
			}
		}
	}
	tableCodes := map[int]bool{}
	for _, r := range allrows {
		tableCodes[r.Code] = true
	}
	tableSorted := make([]int, 0, len(tableCodes))
	for c := range tableCodes {
		tableSorted = append(tableSorted, c)
	}
	sort.Ints(tableSorted)
	notes = append(notes, fmt.Sprintf("主表 %d 档（0/1/2 与门禁同码同义）+ 已挂号 %d 档 ⇒ 数字集合 %s",
		len(main), len(reserved), cecPyIntList(tableSorted)))

	// ── R2 门禁直通档逐字对齐 ──
	hRaw, okH := cecReadText(cecJoin(root, cecHelpRel))
	if !okH {
		return 2, []string{"空转：helptext.go 读不了（判据不可判）"}, nil, map[string]any{}
	}
	m := regexp.MustCompile(`const` + cecWS + `+gateExitLine` + cecWS + `*=` + cecWS + `*"((?:[^"\\]|\\.)*)"`).FindStringSubmatch(hRaw)
	if m == nil {
		return 2, []string{"空转：helptext.go 里找不到 gateExitLine（判据不可判）"}, nil, map[string]any{}
	}
	gateConst := m[1]
	gRaw, okG := cecReadText(cecJoin(root, cecGatesRel))
	if !okG {
		return 2, []string{"空转：门禁件读不了（判据不可判）"}, nil, map[string]any{}
	}
	hits := cecFindGateLine(gRaw)
	if len(hits) == 0 {
		return 2, []string{"空转：门禁头注释里找不到 `# 退出码：…` 那一行（判据不可判）"}, nil, map[string]any{}
	}
	if len(hits) > 1 {
		problems = append(problems, fmt.Sprintf("FAIL R2 门禁头注释里 `# 退出码：…` 命中 %d 行（%s）—— 同一件东西两处口径",
			len(hits), cecPyIntList(hits)))
	}
	lineNo := hits[0]
	gateLines := cecSplitLines(gRaw)
	lineTxt := strings.TrimSpace(regexp.MustCompile(`^#`+cecWS+`?`).ReplaceAllString(gateLines[lineNo-1], ""))
	if lineTxt != strings.TrimSpace(gateConst) {
		problems = append(problems, fmt.Sprintf("FAIL R2 门禁头注释的退码行（现第 %d 行）与命令面 gateExitLine **不逐字一致**：\n      门禁: %s\n      命令面: %s",
			lineNo, lineTxt, strings.TrimSpace(gateConst)))
	} else {
		notes = append(notes, fmt.Sprintf("门禁直通档与命令面 gateExitLine 逐字一致（按内容定位：现第 %d 行）", lineNo))
	}

	// ── R3 声明面全部指向同一张表 ──
	sigSites := map[string]bool{}
	signed := cecArrOfObj(reg["signed_differences"])
	for _, d := range signed {
		sigSites[cecStr(d["site"])] = true
	}
	for _, rel := range sites {
		p := cecJoin(root, rel)
		if !cecIsFile(p) {
			// ★ 三段式回退腿（同族先例 check_contract_registry.go R7 / check_zh_en.go 的
			//   chzeRetiredGoHalfInPlace()）：声明面件缺席而其 **Go 真身**在盘 ⇒ 该声明面已退役
			//   （能力换 Go 半承载）⇒ 本规则**不过问**（✗ 不判缺件、✗ 不静默通过 —— 两半皆缺仍红）。
			if cecGoHalfInPlace(root, rel) {
				continue
			}
			problems = append(problems, "FAIL R3 声明面缺件："+rel)
			continue
		}
		txt, ok := cecReadText(p)
		if !ok {
			txt = ""
		}
		if !strings.Contains(txt, cecMarker) {
			if sigSites[rel] {
				notes = append(notes, fmt.Sprintf("标记行由签字替代（只报告）：%s 未带「%s」，但已在 signed_differences 里逐条签字", rel, cecMarker))
			} else {
				problems = append(problems, fmt.Sprintf("FAIL R3 %s 里没有标记行「%s」—— 它又变成一处独立说法了", rel, cecMarker))
			}
		}
	}
	notes = append(notes, fmt.Sprintf("声明面 %d 处逐条点名", len(sites)))

	// ── R4 「回滚成功」唯一说法 ──
	for _, rel := range []string{cecSwapRel, cecUpgradeRel} {
		p := cecJoin(root, rel)
		if !cecIsFile(p) {
			// ★ 三段式回退腿（件在盘优先 · 缺席回退 Go 真身 · 皆无才 fail-closed 点名）：
			//   件缺席而其 **Go 真身**在盘 ⇒ 回退去读 Go 真身里的同一说法（✗ 不拆断言：
			//   `ROLLED_BACK` 与「回滚成功写成 2」两条仍逐条判）；两半皆缺（真消失）⇒ 照旧点「缺件」。
			//   ★ 只有**已真搬**（登记在 cecRetireGoHalf）的件才有回退腿 ⇒ 未真搬的件不放行。
			if gh, ok := cecRetireGoHalf[rel]; ok && cecIsFile(cecJoin(root, gh)) {
				p = cecJoin(root, gh)
			} else {
				problems = append(problems, "FAIL R4 缺件："+rel)
				continue
			}
		}
		txt, ok := cecReadText(p)
		if !ok {
			txt = ""
		}
		if !strings.Contains(txt, "ROLLED_BACK") {
			problems = append(problems, fmt.Sprintf("FAIL R4 %s 里没有 `ROLLED_BACK`（回滚成功 = 1 + kind=ROLLED_BACK 的说法没落）", rel))
		}
		for ln, line := range cecSplitLines(txt) {
			if cecRollbackBadRE.MatchString(line) {
				problems = append(problems, fmt.Sprintf("FAIL R4 %s:%d 又把「回滚成功」写成 2（否决的说法）：%s",
					rel, ln+1, cecTrunc(strings.TrimSpace(line), 100)))
			}
		}
	}

	// ── R5 用码有落点 ──
	coll := map[int]bool{}
	for _, c := range cecArrOfObj(reg["collisions"]) {
		if v, ok := cecToInt(c["number"]); ok {
			coll[v] = true
		}
	}
	oob := map[int]bool{}
	for _, o := range cecArrOfObj(reg["out_of_range"]) {
		if v, ok := cecToInt(o["code"]); ok {
			oob[v] = true
		}
	}
	for _, rel := range sites {
		p := cecJoin(root, rel)
		if !cecIsFile(p) {
			continue
		}
		entry, has := ns[rel]
		if !has {
			problems = append(problems, fmt.Sprintf("FAIL R5 %s 没有登记命名空间（新声明面必须先在登记表里立一行）", rel))
			continue
		}
		own := cecKeyInts(entry["declared"])
		internal := cecKeyInts(entry["internal"])
		allowed := map[int]bool{}
		for c := range tableCodes {
			allowed[c] = true
		}
		for c := range own {
			allowed[c] = true
		}
		for c := range coll {
			allowed[c] = true
		}
		for c := range oob {
			allowed[c] = true
		}
		for c := range internal {
			allowed[c] = true
		}
		txt, ok := cecReadText(p)
		if !ok {
			txt = ""
		}
		used := cecUsedCodes(txt, cecDetectMode(p))
		var extra []int
		for c := range used {
			if !allowed[c] {
				extra = append(extra, c)
			}
		}
		sort.Ints(extra)
		usedSorted := make([]int, 0, len(used))
		for c := range used {
			usedSorted = append(usedSorted, c)
		}
		sort.Ints(usedSorted)
		if len(extra) > 0 {
			problems = append(problems, fmt.Sprintf("FAIL R5 %s 用了未登记的退码 %s（登记覆盖：主表 ∪ 自有号 ∪ 已登记撞号 ∪ 已登记越界）",
				rel, cecPyIntList(extra)))
		} else {
			notes = append(notes, fmt.Sprintf("%s 用码 %s 逐条有落点", filepath.Base(rel), cecPyIntList(usedSorted)))
		}
	}

	// ── R7 跨表差异逐条落判 ──
	universal := map[int]bool{0: true, 1: true, 2: true}
	sigTbl := map[string]map[string]any{}
	for _, d := range signed {
		code := cecRawNum(d["code"])
		sigTbl[cecStr(d["site"])+"\x00"+code] = d
	}
	for _, rel := range sites {
		p := cecJoin(root, rel)
		if !cecIsFile(p) {
			continue
		}
		if e, has := ns[rel]; has && cecStr(e["id"]) == "cli" {
			continue
		}
		txt, ok := cecReadText(p)
		if !ok {
			txt = ""
		}
		for _, c := range cecUsedSorted(txt, cecDetectMode(p)) {
			if !tableCodes[c] || universal[c] || coll[c] {
				continue
			}
			d := sigTbl[rel+"\x00"+strconv.Itoa(c)]
			if d != nil {
				notes = append(notes, fmt.Sprintf("已签差异（只报告 · **不阻断**）：%s 的 `%d` 与命令面 `%s` 同号不同义 —— %s",
					rel, c, cecCLICode(d), cecStr(d["reason"])))
			} else {
				problems = append(problems, fmt.Sprintf("FAIL R7 %s 用了与命令面主表同号的 `%d`，却没在登记表 signed_differences 里签字"+
					"（**未声明差异**：同数字两义的第一现场 —— 要么签字说明它是「不是同一张表」的"+
					"自有码、要么去掉这个号）", rel, c))
			}
		}
	}
	for _, d := range signed {
		s := cecStr(d["site"])
		if !cecHasStr(sites, s) {
			problems = append(problems, fmt.Sprintf("FAIL R7 signed_differences 点名的面 %s **不在 sites 表**（签字没有落点）", s))
			continue
		}
		p := cecJoin(root, s)
		if cecIsFile(p) {
			txt, ok := cecReadText(p)
			if !ok {
				txt = ""
			}
			code, has := cecToInt(d["code"])
			if !has {
				code = -1
			}
			if !cecUsedCodes(txt, cecDetectMode(p))[code] {
				notes = append(notes, fmt.Sprintf("R7 陈旧签字（只报告）：%s 没在用 `%s`（signed_differences 那一条可撤）",
					s, cecRawNum(d["code"])))
			}
		}
	}

	// ── R6 「判不了」档口径的机检（只报告 · 不阻断）──
	r6Denom, r6Hits, r6Exempt := cecScanValueInvalid8(root)
	var r6vi, r6mix, r6bd []cecHit
	for _, h := range r6Hits {
		switch {
		case h.ValueInvalid:
			r6vi = append(r6vi, h)
		case h.Mixed:
			r6mix = append(r6mix, h)
		default:
			r6bd = append(r6bd, h)
		}
	}
	notes = append(notes, fmt.Sprintf("R6 「判不了」档口径机检（只报告 · 不阻断）：盘上现读 %d 件（.py/.sh/.go · 跳 VCS/"+
		"依赖/产物 与 >2MB）· 命中用 8 的处 %d 条 ⇒ 「值无效类」（按甲案应走 2 · 待归一）%d 条 ·"+
		" 「BLOCKED 类」（8 合法）%d 条 · 「混类」（两类词同现 ⇒ 按优先级判 BLOCKED ·"+
		" 待复核词逐条打印）%d 条 · 自豁免（打印理由）%d 件",
		r6Denom, len(r6Hits), len(r6vi), len(r6bd), len(r6mix), len(r6Exempt)))
	for _, e := range r6Exempt {
		notes = append(notes, fmt.Sprintf("R6 ⊘ 自豁免 %s —— %s", e["site"], e["why"]))
	}
	for _, h := range r6Hits {
		notes = append(notes, fmt.Sprintf("R6 %s:%d [%s] 理由: %s ｜逐字: %s", h.Site, h.Line, h.Verdict, h.Why, h.Text))
	}

	rc := 0
	if len(problems) > 0 {
		rc = 1
	}
	return rc, problems, notes, map[string]any{
		"table": tableSorted, "sites": len(sites),
		"collisions": len(coll), "out_of_range": len(oob),
		"signed_differences": len(sigTbl),
		"r6_scan_set":        r6Denom, "r6_hits_8": len(r6Hits),
		"r6_value_invalid_8": len(r6vi),
		"r6_mixed_8":         len(r6mix),
		"r6_blocked_8":       len(r6bd),
		"r6_self_exempt":     len(r6Exempt),
	}
}

func cecHasStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func cecCLICode(d map[string]any) string {
	if v, has := d["cli_code"]; has && v != nil {
		return cecRawNum(v)
	}
	return "?"
}

func cecPrintReport(rc int, problems, notes []string) int {
	fmt.Fprint(os.Stdout, "── 门⑨ 退码表：一张表 + 同数字不再两义（§二十一 已红第 4 条 · T-26 · §九 M3 C6 · §十二 P-129）──\n")
	fmt.Fprint(os.Stdout, "  真源: core/cmd/zerg/exitcodes.go · 登记表: "+cecSitesRel+"\n")
	for _, n := range notes {
		fmt.Fprint(os.Stdout, "  · "+n+"\n")
	}
	for _, p := range problems {
		fmt.Fprint(os.Stdout, "  · "+p+"\n")
	}
	switch rc {
	case 0:
		fmt.Fprint(os.Stdout, "⇒ 全绿（rc=0）：七处声明面指向同一张表 · 主表内数字唯一 · 回滚成功只有一个说法 · 用码逐条有落点 · 跨表差异逐条签字\n")
	case 1:
		fmt.Fprint(os.Stdout, "⇒ 门禁红（rc=1）：退码说法/编号面出现第二个口径\n")
	default:
		fmt.Fprint(os.Stdout, "⇒ 不给结论（rc=2）\n")
	}
	return rc
}

func cecPrintReportTo(w io.Writer, rc int, problems, notes []string) int {
	fmt.Fprint(w, "── 门⑨ 退码表：一张表 + 同数字不再两义（§二十一 已红第 4 条 · T-26 · §九 M3 C6 · §十二 P-129）──\n")
	fmt.Fprint(w, "  真源: core/cmd/zerg/exitcodes.go · 登记表: "+cecSitesRel+"\n")
	for _, n := range notes {
		fmt.Fprint(w, "  · "+n+"\n")
	}
	for _, p := range problems {
		fmt.Fprint(w, "  · "+p+"\n")
	}
	switch rc {
	case 0:
		fmt.Fprint(w, "⇒ 全绿（rc=0）：七处声明面指向同一张表 · 主表内数字唯一 · 回滚成功只有一个说法 · 用码逐条有落点 · 跨表差异逐条签字\n")
	case 1:
		fmt.Fprint(w, "⇒ 门禁红（rc=1）：退码说法/编号面出现第二个口径\n")
	default:
		fmt.Fprint(w, "⇒ 不给结论（rc=2）\n")
	}
	return rc
}

// ── Q-176：本门无机器读面 ⇒ 给了 --json 不许沉默（退码 = 用法错 2）────────────

var cecJSONFields []string

func cecRefuseJSON(argv []string, stderr io.Writer) *int {
	var hit []string
	for _, x := range argv {
		if x == "--json" || strings.HasPrefix(x, "--json=") {
			hit = append(hit, x)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	var quoted []string
	for _, h := range hit {
		quoted = append(quoted, "`"+h+"`")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	fields := "无（本门只出人读面）"
	if len(cecJSONFields) > 0 {
		var q []string
		for _, f := range cecJSONFields {
			q = append(q, "`"+f+"`")
		}
		fields = strings.Join(q, " · ")
	}
	fmt.Fprintf(stderr, "   字段表：%s\n", fields)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	rc := 2
	return &rc
}

// ── 真搬入口 ─────────────────────────────────────────────────────────────

func checkExitCodesGateMain(stdout, stderr io.Writer, argv []string) int {
	cecReadFails = nil // ★ 自清：包级登记面不许跨调用残留（残留会让下一次一进门就被 BLOCKED）
	if rc := cecRefuseJSON(argv, stderr); rc != nil {
		return *rc
	}
	root := "."
	selfTest := false
	var positionals []string
	for _, a := range argv {
		switch {
		case a == "--self-test":
			selfTest = true
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, cecUsage())
			return 0
		case strings.HasPrefix(a, "-") && a != "-":
			// argparse：未知旗标 ⇒ 用法错 rc=2（正文不复刻，见件头未覆盖面）
			fmt.Fprintf(stderr, "usage: check-exit-codes.py [-h] [--self-test] [root]\n")
			fmt.Fprintf(stderr, "check-exit-codes.py: error: unrecognized arguments: %s\n", a)
			return 2
		default:
			positionals = append(positionals, a)
		}
	}
	if len(positionals) > 1 {
		fmt.Fprintf(stderr, "usage: check-exit-codes.py [-h] [--self-test] [root]\n")
		fmt.Fprintf(stderr, "check-exit-codes.py: error: unrecognized arguments: %s\n",
			strings.Join(positionals[1:], " "))
		return 2
	}
	if len(positionals) == 1 {
		root = positionals[0]
	}
	if selfTest {
		return cecSelfTestTo(stdout)
	}
	r, ok := cecFindRoot(root)
	if !ok {
		fmt.Fprint(stdout, "⛔ 不给结论：找不到 core/cmd/zerg（本门只认命令面真源所在的那棵树）\n")
		return 2
	}
	rc, problems, notes, _ := cecCheck(r)
	if len(cecReadFails) > 0 {
		fmt.Fprintf(stderr, "⛔ BLOCKED：判据件读失败 %d 件（读失败静默 ⇒ 不给结论 · rc=2）\n", len(cecReadFails))
		for _, f := range cecReadFails {
			fmt.Fprintf(stderr, "    · %s\n", f)
		}
		return 2
	}
	return cecPrintReportTo(stdout, rc, problems, notes)
}

func cecUsage() string {
	return "usage: check-exit-codes.py [-h] [--self-test] [root]\n\n" +
		"positional arguments:\n  root\n\noptions:\n  -h, --help  show this help message and exit\n" +
		"  --self-test\n"
}

// ── JSON 取值助手（复刻 py dict.get / json.loads 的语义）───────────────────

func cecJSONDecode(raw string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func cecJSONObject(raw string) (map[string]any, error) {
	v, err := cecJSONDecode(raw)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("顶层不是对象")
	}
	return m, nil
}

func cecArrOfObj(v any) []map[string]any {
	xs, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []map[string]any
	for _, e := range xs {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func cecStr(v any) string {
	s, _ := v.(string)
	return s
}

func cecStrSlice(v any) []string {
	xs, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, e := range xs {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func cecToInt(v any) (int, bool) {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i), true
		}
		if f, err := t.Float64(); err == nil {
			return int(f), true
		}
	case float64:
		return int(t), true
	case string:
		if i, err := strconv.Atoi(t); err == nil {
			return i, true
		}
	}
	return 0, false
}

// cecRawNum 复刻 py `%s` 打一个 JSON 标量（int ⇒ "8" · 串 ⇒ 原样 · 缺席 ⇒ "None"）。
func cecRawNum(v any) string {
	switch t := v.(type) {
	case nil:
		return "None"
	case json.Number:
		return t.String()
	case string:
		return t
	case bool:
		if t {
			return "True"
		}
		return "False"
	}
	return fmt.Sprint(v)
}

func cecKeyInts(v any) map[int]bool {
	out := map[int]bool{}
	m, ok := v.(map[string]any)
	if !ok {
		return out
	}
	for k := range m {
		if i, err := strconv.Atoi(k); err == nil {
			out[i] = true
		}
	}
	return out
}

// cecJSONQuote 复刻 py `json.dumps(s, ensure_ascii=False)`（只用于自检夹具的 gateExitLine 字面量）。
func cecJSONQuote(s string) string {
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
		case '	':
			b.WriteString(string([]byte{92, 116}))
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

// ─────────────────────────── 自检（成对负控）───────────────────────────

const (
	cecGoodTable = "package main\nvar exitcodeTable = []exitcodeRow{\n	{0, \"ok\", \"成功\", \"—\"},\n" +
		"	{2, \"usage\", \"用法错 / **不给结论**\", \"改用法后可重试\"},\n}\n" +
		"var exitcodeReserved = []exitcodeRow{\n	{130, \"interrupted\", \"人打断\", \"—\"},\n}\n"

	cecGoodGateLineDecl = "退出码：0 全绿 · 1 有失败项 · 2 **不给结论**"

	cecGoodDecl = "# 退码唯一真源 = zerg help exit-codes（契约 §三）—— 本件是组件自有码空间\n" +
		"const (\n	exitOK = 0\n	exitUsage = 2\n)\n"

	cecGoodSwap = "# 退码唯一真源 = zerg help exit-codes（契约 §三）——本件是脚本自有码空间\n" +
		"# 「回滚成功」= 1 + error.kind=ROLLED_BACK（不是 0，也不是 2）\n" +
		"exit 0\nexit 2\nsay \"error.kind=ROLLED_BACK\"\n"

	cecGoodUp = "# 退码唯一真源 = zerg help exit-codes（契约 §三）——本件是脚本自有码空间\n" +
		"# 「回滚成功」= 1 + error.kind=ROLLED_BACK\nexit 0\nexit 1\n"

	cecGoodTable8 = "package main\nvar exitcodeTable = []exitcodeRow{\n	{0, \"ok\", \"成功\", \"—\"},\n" +
		"	{2, \"usage\", \"用法错 / **不给结论**\", \"改用法后可重试\"},\n" +
		"	{8, \"blocked\", \"有 BLOCKED（「读不到」不许当健康）\", \"补齐前置后可重试\"},\n}\n" +
		"var exitcodeReserved = []exitcodeRow{\n	{130, \"interrupted\", \"人打断\", \"—\"},\n}\n"

	cecGoodRosterJSON = `{"schema":"zerg/exitcodes-namespaces/v1","namespaces":[` +
		`{"id":"cli","owner":"core/cmd/zerg/exitcodes.go","scan":"go-table"},` +
		`{"id":"zerg-compat","owner":"core/cmd/zerg-compat/main.go","scan":"go","declared":{"0":"ok","2":"有待迁移文件"}},` +
		`{"id":"swap","owner":"scripts/build/zerg-swap-core.sh","scan":"sh","declared":{"0":"换件成功","2":"用法错"}},` +
		`{"id":"upgrade","owner":"scripts/build/zerg-upgrade.sh","scan":"sh","declared":{"0":"成功","1":"失败"}},` +
		`{"id":"gates","owner":"scripts/gates/precommit-gates.sh","scan":"sh","declared":{"0":"全绿","1":"有失败项","2":"不给结论"}}],` +
		`"collisions":[{"number":2,"verdict":"已登记","trigger":"待拍"}],"out_of_range":[],` +
		`"sites":["core/cmd/zerg-compat/main.go","scripts/build/zerg-swap-core.sh",` +
		`"scripts/build/zerg-upgrade.sh","scripts/gates/precommit-gates.sh"]}`
)

func cecGoodRoster() map[string]any {
	v, _ := cecJSONDecode(cecGoodRosterJSON)
	m, _ := v.(map[string]any)
	return m
}

type cecOver struct {
	table        *string
	gateLine     *string
	gateFileLine *string
	decl         *string
	swap         *string
	up           *string
	roster       map[string]any
	rosterAbsent bool
}

func cecW(dir, rel, text string) {
	p := cecJoin(dir, rel)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(text), 0o644)
}

func cecFixture(d string, over cecOver) {
	tbl := cecGoodTable
	if over.table != nil {
		tbl = *over.table
	}
	cecW(d, cecTableRel, tbl)
	gateLine := cecGoodGateLineDecl
	if over.gateLine != nil {
		gateLine = *over.gateLine
	}
	cecW(d, cecHelpRel, "package main\n\nconst gateExitLine = "+cecJSONQuote(gateLine)+"\n")
	gfl := gateLine
	if over.gateFileLine != nil {
		gfl = *over.gateFileLine
	}
	cecW(d, cecGatesRel, "#!/usr/bin/env bash\n# "+cecMarker+"（契约 §三）\n# "+gfl+"\n")
	decl := cecGoodDecl
	if over.decl != nil {
		decl = *over.decl
	}
	cecW(d, "core/cmd/zerg-compat/main.go", decl)
	sw := cecGoodSwap
	if over.swap != nil {
		sw = *over.swap
	}
	cecW(d, cecSwapRel, sw)
	up := cecGoodUp
	if over.up != nil {
		up = *over.up
	}
	cecW(d, cecUpgradeRel, up)
	reg := over.roster
	if reg == nil {
		reg = cecGoodRoster()
	}
	b, _ := json.Marshal(reg)
	cecW(d, cecSitesRel, string(b))
}

func cecFlag(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

func cecSelfTestTo(w io.Writer) int {
	cecReadFails = nil // ★ 自清：包级登记面不许跨调用残留（残留会让下一次一进门就被 BLOCKED）
	type cecCase struct {
		name string
		want int
		over cecOver
	}
	ss := func(s string) *string { return &s }
	dupTable := strings.Replace(cecGoodTable,
		"{2, \"usage\", \"用法错 / **不给结论**\", \"改用法后可重试\"},",
		"{0, \"dup\", \"重复的 0\", \"—\"},", 1)
	noMarkerDecl := strings.Replace(cecGoodDecl,
		"# "+cecMarker+"（契约 §三）—— 本件是组件自有码空间\n", "", 1)
	badSwap := strings.Replace(cecGoodSwap,
		"# 「回滚成功」= 1 + error.kind=ROLLED_BACK（不是 0，也不是 2）\n",
		"# 回滚成功 = 2（老说法）\n", 1)
	rosterSignedSwap := cecGoodRoster()
	rosterSignedSwap["signed_differences"] = []any{map[string]any{
		"site": "scripts/build/zerg-swap-core.sh", "code": 8, "cli_code": 8,
		"reason": "R6 自证夹具：脚本声明面把值无效类写成 8"}}
	rosterSignedDecl := cecGoodRoster()
	rosterSignedDecl["signed_differences"] = []any{map[string]any{
		"site": "core/cmd/zerg-compat/main.go", "code": 8, "cli_code": 8,
		"reason": "R6 自证夹具：Go 声明面把空串类写成 8"}}
	rosterNoNS := cecGoodRoster()
	rosterNoNS["namespaces"] = []any{}

	cases := []cecCase{
		{"正控① 好件：主表唯一 · 门禁行逐字对齐 · 四处都有标记 · 回滚说法为 1+ROLLED_BACK ⇒ rc=0", 0, cecOver{}},
		{"负控② 主表里数字重复（0 出现两次）⇒ rc=1", 1, cecOver{table: ss(dupTable)}},
		{"负控③ 门禁头注释退码行被改一个词 ⇒ rc=1", 1, cecOver{gateFileLine: ss("退出码：0 全绿 · 1 有失败项 · 2 不给结论")}},
		{"负控④ 某处声明面摘掉标记行 ⇒ rc=1", 1, cecOver{decl: ss(noMarkerDecl)}},
		{"负控⑤ 把「回滚成功」又写成 2 ⇒ rc=1", 1, cecOver{swap: ss(badSwap)}},
		{"负控⑥ 声明面用了未登记的退码（swap 用了 7）⇒ rc=1", 1, cecOver{swap: ss(cecGoodSwap + "exit 7\n")}},
		{"负控⑦ 登记表缺件 ⇒ rc=2（判据不可判）", 2, cecOver{rosterAbsent: true}},
		{"负控⑧ 登记表里 0 个命名空间 ⇒ rc=2（空转不许当绿）", 2, cecOver{roster: rosterNoNS}},
		{"正控⑨ R6 只报告：主表有 8 时，脚本声明面把「值无效」写成 " + cecE8 + " 也**不阻断**（rc 仍 0）", 0,
			cecOver{table: ss(cecGoodTable8), roster: rosterSignedSwap,
				swap: ss(cecGoodSwap + "# 值无效：参数空串 ⇒ " + cecE8 + "\n" + cecE8 + "\n")}},
		{"正控⑩ R6 只报告：Go 声明面把「空串」写成 " + cecEC8 + " 也**不阻断**（rc 仍 0）", 0,
			cecOver{table: ss(cecGoodTable8), roster: rosterSignedDecl,
				decl: ss(cecGoodDecl + "// 值无效：空串参数 ⇒ " + cecEC8 + "\nvar exit" + "Bad = 8\n")}},
	}
	ok := true
	for _, c := range cases {
		d, err := os.MkdirTemp("", "cec-")
		if err != nil {
			fmt.Fprintf(w, "  ✗ %s（fixture 建不起来）\n", c.name)
			ok = false
			continue
		}
		if c.over.rosterAbsent {
			over := c.over
			over.roster = cecGoodRoster()
			cecFixture(d, over)
			_ = os.Remove(cecJoin(d, cecSitesRel))
		} else {
			cecFixture(d, c.over)
		}
		rc, problems, _, _ := cecCheck(d)
		fmt.Fprintf(w, "  %s %s（rc=%d）\n", cecFlag(rc == c.want), c.name, rc)
		if rc != c.want {
			ok = false
			for _, p := range problems {
				fmt.Fprintf(w, "      · %s\n", p)
			}
		}
		_ = os.RemoveAll(d)
	}

	// ⑪ 成对（自豁免 ≠ 放宽真判据）
	d, _ := os.MkdirTemp("", "cec-")
	probe := "# 值无效：参数空串 ⇒ " + cecE8 + "\n" + cecE8 + "\n"
	cecW(d, "scripts/build/probe-real.sh", probe)
	cecW(d, "scripts/gates/"+cecSelfExemptBasename, probe)
	_, h1, ex1 := cecScanValueInvalid8(d)
	realN, fakeN := 0, 0
	for _, x := range h1 {
		if strings.HasSuffix(x.Site, "probe-real.sh") {
			realN++
		}
		if strings.HasSuffix(x.Site, cecSelfExemptBasename) {
			fakeN++
		}
	}
	okPair := realN == 2 && fakeN == 0 && len(ex1) == 1
	fmt.Fprintf(w, "  %s ⑪ 自豁免不放宽真判据：别件真缺陷仍计入（%d 行）· 本件名夹具只进自豁免"+
		"（命中 %d 行 · 豁免 %d 件）\n", cecFlag(okPair), realN, fakeN, len(ex1))
	ok = ok && okPair
	_ = os.RemoveAll(d)

	// ⑫ 判据面两态
	d3, _ := os.MkdirTemp("", "cec-")
	cecW(d3, "scripts/gates/allsh-414-probe.sh",
		"echo \"all.sh: real-gates.sh --json 读不动（JSON 不合法 / 空）⇒ --real **判不了**（"+cecRC8+
			"）\" >&2\n"+cecE8+"\n")
	cecW(d3, "scripts/build/value-invalid-probe.sh",
		"echo \"参数值无效：空串 ⇒ "+cecE8+"\" >&2\n"+
			"echo \"缺实参 ⇒ "+cecE8+"\" >&2\n"+cecE8+"\n")
	cecW(d3, "scripts/build/precondition-missing-probe.sh",
		"# 真源在但读不出 ⇒ "+cecRC8+" · meta.reason=precondition_missing\n"+cecE8+"\n")
	_, h3, _ := cecScanValueInvalid8(d3)
	cnt := func(suffix string) int {
		n := 0
		for _, x := range h3 {
			if strings.HasSuffix(x.Site, suffix) && x.ValueInvalid {
				n++
			}
		}
		return n
	}
	n414, nok2, npre := cnt("allsh-414-probe.sh"), cnt("value-invalid-probe.sh"), cnt("precondition-missing-probe.sh")
	fixture414OK := n414 == 0 && nok2 == 2 && npre == 0
	fmt.Fprintf(w, "  %s ⑫ 判据面两态：真 BLOCKED（all.sh:414 形态 · 读不动/判不了）**不计入**值无效类（%d 行）"+
		" · 真「值无效写成 8」**仍逐条计入**（%d 行）· ASCII 子词（precondition_missing）不再误命中"+
		"（%d 行）\n", cecFlag(fixture414OK), n414, nok2, npre)
	var why414 *cecHit
	for i := range h3 {
		if strings.HasSuffix(h3[i].Site, "allsh-414-probe.sh") {
			why414 = &h3[i]
			break
		}
	}
	verdict, why := "（没命中 · 异常）", ""
	if why414 != nil {
		verdict, why = why414.Verdict, why414.Why
	}
	fmt.Fprintf(w, "  %s ⑫ 逐条理由（all.sh:414 形态那条判哪类）：[%s] %s\n",
		cecFlag(why414 != nil && !why414.ValueInvalid), verdict, why)
	ok = ok && fixture414OK && why414 != nil && !why414.ValueInvalid
	_ = os.RemoveAll(d3)

	if ok {
		fmt.Fprint(w, "自检结论: 全过\n")
		return 0
	}
	fmt.Fprint(w, "自检结论: **有断言失败**\n")
	return 1
}
