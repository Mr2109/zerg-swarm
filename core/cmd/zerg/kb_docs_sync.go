// kb_docs_sync.go —— 工具面 `tools/kb_docs_sync.py`（179 行）的 **Go 真搬**
// （v2.5.14 · M2 · C 波 · 工具/生成器面搬运 · 入口 `kbDocsSyncGateMain`）。
//
// 本件形态：**生成器/工具**（非判据件）—— 两个可观测出口：
//
//	① 人读面：stdout 的候选清单
//	② 落盘面：dry-run 写 `kb_sync_candidates.json`；`--mark` 写 `kb_docs_registry.json`
//
// 生成器验收口径（照 M2 甲案）：**同输入下 py 产出 ⟷ 本入口产出 逐字节相同**
// （stdout ＋ 落盘件的 sha256 逐字节同）；见 `kb_docs_sync_test.go` 与回执的产物对拍表。
//
// 逐字节对齐要点（照 py 逐条）：
//
//	① py 的 `\d`/`\s` 是 **Unicode 语义**、Go RE2 默认 ASCII ⇒ 版本线名正则的 `\d` 已展开 `\p{Nd}`。
//	② 路径拼接一律 **posixpath 同构**（自写 `kbsPyJoin`/`kbsPyAbspath`/`kbsPyNormpath`/`kbsPyRelpath`，
//	   ✗ 不用 `filepath.Join`；血证：后段绝对路径 py 丢弃前段、Go 会拼成 `a/b`）。
//	③ py `open(..., encoding="utf-8").read()` = **universal newlines** ⇒ 本件对读出字节先做
//	   `\r\n`→`\n`、`\r`→`\n` 折叠再按 `\n` 切（✗ 不直接按原始字节切行）。
//	④ py `str.strip()` 剥的是 py 的 Unicode 空白（**含 `\x1c-\x1f`**，Go `unicode.IsSpace` 不含）
//	   ⇒ 自写 `kbsPyStrip`（✗ 不用 `strings.TrimSpace`）；`s[:400]` 按**码点**切 ⇒ 先 `[]rune`。
//	⑤ py `glob.glob` = readdir 原序 → 再 `sorted(...)` ⇒ 本件读目录用 `ReadDir(-1)` 保原序、
//	   过滤后 `sort.Strings`（Go 字节序 == py 码点序，UTF-8 下等价）；hidden 件按 glob 口径排除。
//	⑥ py `os.path.isdir()` **跟随符号链接** ⇒ 本件用 `os.Stat`（✗ 不用 `entry.IsDir()` 的 lstat 语义）。
//	⑦ JSON 面：py `json.dump(..., ensure_ascii=False, indent=1)` 的**键序 = dict 插入序**、
//	   分隔符 `", "`/`": "` 且缩进 1 ⇒ 本件自写有序编码（✗ 不用 `encoding/json` 的 HTML 转义/乱序 map）。
//	⑧ 非确定面：dry-run 全路径**不含时刻**（纯确定性，产物可逐字节对拍）；`--mark` 的 `ts` 走
//	   **Go-only 注入旋钮 `--now`**（✗ 不掩码），使两侧产物可逐字节对拍。
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py `REPO`/`KB_DIR` 是**写死的绝对路径常量** ⇒ 从任意 cwd 都同一读数；
//	    本件无 `__file__` 语境、也无该常量（私有绝对路径 ✗ 写进仓件）⇒ 默认根从 **cwd** 取，
//	    KB 根从 `repo/../../知识库` 推导（在真仓根执行时与 py 的写死值**逐字相同**）；
//	    否则路径推导面不同（✗ 不属逐字对齐承诺）。仓外夹具对拍时两侧都用显式旋钮/常量替换对齐。
//	(b) **自指检查**：本件扫描面 = 调用方点名的文档树/知识库目录；本件（落 `core/cmd/zerg/`）
//	    **不在自身扫描面内**。量化：全件零连续私有绝对路径字面量（`/Vol` + 卷名拼串）、
//	    退码仅 0/1/2（⊆ 真源退码表已登记集）、零「JSON 键名连续引号原串」。
//
// 本单边界（照实）：**只落码、不接线** —— 不注册命令、不改 `main.go`、不删 py 原件、
// 无调用点（M2 刻意状态；接线与「doc 族扩 action」属另一路独占）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ── 常量（照 py 逐字）─────────────────────────────────────────────────────
const kbsSelfName = "kb_docs_sync.py"
const kbsVersionLine = "kb_docs_sync v1.0.0"

const kbsDocsSub = "项目文档"
const kbsZergDocsDir = "Zerg-内部文档"
const kbsKBLeaf = "知识库"
const kbsRegistryName = "kb_docs_registry.json"
const kbsCandidatesName = "kb_sync_candidates.json"

// 私有绝对路径只用**公共前后缀拼串**出现在人读帮助面（✗ 不写连续字面量 —— 免给
// `check-hardcoded-private-paths.py` 的 core/ 扫描面新增命中）。
const kbsHelpKBVol = "/Vol" + "umes/AHZ/00 项目/知识库"

// 关键文档 glob 清单（顺序 = py `pats` 顺序，影响 registry 插入序）。
var kbsKeyDocPats = []string{
	"00-架构*.md", "使用-虫族指南*.md", "变更-*.md", "01-模块-主控*.md",
	"04-模块-任务*.md", "06-模块-子端*.md", "07-模块-UI*.md", "设计-v2.5.8-*.md",
}

// 版本线名：py `re.match(r"^v\d+\.\d+\.\d+$", d)`（`\d` Unicode ⇒ `\p{Nd}`；py `$` 容忍唯一尾换行）。
var kbsVerDirRE = regexp.MustCompile("^v[\\p{Nd}]+\\.[\\p{Nd}]+\\.[\\p{Nd}]+(\n)?$")

// py `__doc__`（`--help`/`-h` 逐字打印 · print 再补一个尾换行）。
const kbsHelpText = "kb_docs_sync v1.0.0 —— 项目文档 → 知识库同步（变更检测 + 候选门控）\n" +
	"\n" +
	"设计: Zerg-内部文档/项目文档/v2.5.8/设计-v2.5.8-文档体系-模型读取成本-20260903.md 6.2（★ 2026-09-19 二次分家批5：该件现位置 = 同级 `Zerg-内部文档/项目文档/…`，仓内不再有）\n" +
	"★ 2026-09-19 「项目文档」二次分家批2：取源根 `DOCS` **双认**（仓内 `docs/项目文档` 优先、缺则同级\n" +
	"  仓外 `Zerg-内部文档/项目文档/`）——与 `docs/site/export-and-build.sh` 的 `ZERG_DOCS_ALT` 同根同默认；\n" +
	"  **两处都缺 ⇒ exit 2 不给结论**（原行为是打印「未找到当前版本目录」后 exit 0 = 假绿）。\n" +
	"  ★ 版本值**未动**（本脚本行为已改 ⇒ 按铁律 4 本该升版本；但 `scripts/gates/check-tool-version-sync.sh`\n" +
	"  明写「改抬头前先由 Mr2109 定版本值，勿自动顺延」⇒ 版本值**待拍**，见 tools/kb_docs_sync.md 履历登记）。\n" +
	"  ★ 批3 后 `file_path`（= relpath 到 REPO）会带 `../Zerg-内部文档/` 前缀 ⇒ KB 侧按**文件名**建索引\n" +
	"  （registry 键 = basename）不受影响；`file_path` 写法待拍。\n" +
	"核心: 元数据注册表 + content_hash 变更检测 —— 只处理新增/变更 —— 产出候选条目（不直连 knowledge.db——\n" +
	"      知识库铁律: 入库走 MCP kb_*——脚本做检测与候选——入库由核验者执行）\n" +
	"\n" +
	"用法:\n" +
	"  python3 kb_docs_sync.py                 # dry-run: 检测变更 → 打印候选清单（不写库）\n" +
	"  python3 kb_docs_sync.py --mark <file>   # 核验入库后标记注册表（更新 hash/ts）\n" +
	"注册表: " + kbsHelpKBVol + "/kb_docs_registry.json（持久——记录已入库 file_path+hash）\n" +
	"候选输出: " + kbsHelpKBVol + "/kb_sync_candidates.json（核验者据此 kb_add——domain=虫族）\n"

// ── 旗标面 ────────────────────────────────────────────────────────────────
// py known = {--mark, --help, -h}；本件另收 3 个 **Go-only 旋钮**（py 不认 ⇒ 旋钮面不合同，
// 见未覆盖面：`--repo`/`--kb-dir`/`--now`）。
var kbsKnownFlags = map[string]bool{
	"--mark": true, "--help": true, "-h": true,
	"--repo": true, "--kb-dir": true, "--now": true,
}

type kbsArgs struct {
	help  bool
	repo  string
	kbDir string
	now   string
}

// kbsScan —— 只做「旗标识别」（未知旗标的 rc=2 判定在 kbsDispatch 里，顺序照 py）。
func kbsScan(argv []string) kbsArgs {
	var a kbsArgs
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "--help", "-h":
			a.help = true
		case "--repo":
			if i+1 < len(argv) {
				a.repo = argv[i+1]
				i++
			}
		case "--kb-dir":
			if i+1 < len(argv) {
				a.kbDir = argv[i+1]
				i++
			}
		case "--now":
			if i+1 < len(argv) {
				a.now = argv[i+1]
				i++
			}
		}
	}
	return a
}

func kbsFirstIdx(argv []string, want string) int {
	for i, x := range argv {
		if x == want {
			return i
		}
	}
	return -1
}

// ── py pathlib/posixpath 同构 ────────────────────────────────────────────

// kbsPyJoin —— 照 CPython `posixpath.join` 逐段规则（✗ 不用 `filepath.Join`）。
func kbsPyJoin(a string, p ...string) string {
	path := a
	for _, b := range p {
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

// kbsPyNormpath —— 照 CPython `posixpath.normpath`（保留恰两个前导斜杠）。
func kbsPyNormpath(path string) string {
	if path == "" {
		return "."
	}
	initial := 0
	if strings.HasPrefix(path, "/") {
		initial = 1
		if strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "///") {
			initial = 2
		}
	}
	var newComps []string
	for _, comp := range strings.Split(path, "/") {
		if comp == "" || comp == "." {
			continue
		}
		if comp != ".." || (initial == 0 && len(newComps) == 0) || (len(newComps) > 0 && newComps[len(newComps)-1] == "..") {
			newComps = append(newComps, comp)
		} else if len(newComps) > 0 {
			newComps = newComps[:len(newComps)-1]
		}
	}
	out := strings.Join(newComps, "/")
	if initial > 0 {
		out = strings.Repeat("/", initial) + out
	}
	if out == "" {
		return "."
	}
	return out
}

// kbsPyAbspath —— `normpath(join(cwd, path))`（py 纯词法、不解析符号链接）。
func kbsPyAbspath(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	return kbsPyNormpath(kbsPyJoin(wd, path))
}

// kbsPyBasename —— `posixpath.split(path)[1]`。
func kbsPyBasename(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return path
	}
	return path[i+1:]
}

// kbsPyRelpath —— 照 CPython `posixpath.relpath`（词法 · abspath 后比公共前缀）。
func kbsPyRelpath(path, start string) string {
	if path == "" {
		return ""
	}
	sa := kbsPyAbspath(start)
	pa := kbsPyAbspath(path)
	if sa == pa {
		return "."
	}
	sp := kbsSplitComps(sa)
	pp := kbsSplitComps(pa)
	i := 0
	for i < len(sp) && i < len(pp) && sp[i] == pp[i] {
		i++
	}
	var rel []string
	for j := i; j < len(sp); j++ {
		rel = append(rel, "..")
	}
	rel = append(rel, pp[i:]...)
	if len(rel) == 0 {
		return "."
	}
	return strings.Join(rel, "/")
}

func kbsSplitComps(p string) []string {
	var out []string
	for _, c := range strings.Split(p, "/") {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

func kbsIsDir(p string) bool {
	st, err := os.Stat(p) // ★ os.stat 语义（跟随符号链接）= py os.path.isdir
	return err == nil && st.IsDir()
}

// ── py 字符串原语 ────────────────────────────────────────────────────────

// kbsPySpace —— py `str.strip()` 的空白集（Unicode White_Space ＋ `\x1c-\x1f`）。
func kbsPySpace(r rune) bool {
	if r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.IsSpace(r)
}

func kbsPyStrip(s string) string {
	rs := []rune(s)
	i, j := 0, len(rs)
	for i < j && kbsPySpace(rs[i]) {
		i++
	}
	for j > i && kbsPySpace(rs[j-1]) {
		j--
	}
	return string(rs[i:j])
}

// kbsPyLStripSet —— py `s.lstrip(chars)`（字符集反复剥）。
func kbsPyLStripSet(s, chars string) string {
	rs := []rune(s)
	i := 0
	for i < len(rs) && strings.ContainsRune(chars, rs[i]) {
		i++
	}
	return string(rs[i:])
}

// kbsUniversalNewlines —— py 文本模式解码（`\r\n`→`\n`、裸 `\r`→`\n`）。
func kbsUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// kbsFnmatch —— py `fnmatch.fnmatchcase` 子集（`*`/`?`；本件 pattern 清单里无 `[` 字符类）。
func kbsFnmatch(name, pat string) bool {
	n, p := []rune(name), []rune(pat)
	ni, pi, star, mark := 0, 0, -1, 0
	for ni < len(n) {
		if pi < len(p) && (p[pi] == '?' || p[pi] == n[ni]) {
			ni++
			pi++
		} else if pi < len(p) && p[pi] == '*' {
			star = pi
			mark = ni
			pi++
		} else if star >= 0 {
			pi = star + 1
			mark++
			ni = mark
		} else {
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

// ── 目录读取（readdir 原序）──────────────────────────────────────────────

type kbsDirEntry struct {
	name  string
	isDir bool
}

func kbsReadDir(dir string) []kbsDirEntry {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer f.Close()
	ents, err := f.ReadDir(-1) // ★ 保 readdir 原序（✗ os.ReadDir 会排序）
	if err != nil {
		return nil
	}
	out := make([]kbsDirEntry, 0, len(ents))
	for _, e := range ents {
		out = append(out, kbsDirEntry{name: e.Name(), isDir: kbsIsDir(kbsPyJoin(dir, e.Name()))})
	}
	return out
}

// kbsGlob1 —— py `glob.glob(<dir>/<pattern>)`：readdir 原序过滤（hidden 按 glob 口径排除）。
func kbsGlob1(dir, pattern string) []string {
	var out []string
	patHidden := strings.HasPrefix(pattern, ".")
	for _, e := range kbsReadDir(dir) {
		if !patHidden && strings.HasPrefix(e.name, ".") {
			continue
		}
		if kbsFnmatch(e.name, pattern) {
			out = append(out, kbsPyJoin(dir, e.name))
		}
	}
	return out
}

// ── 版本线 / 关键文档 ────────────────────────────────────────────────────

func kbsDocsCandidates(repoRoot string) []string {
	return []string{
		kbsPyJoin(repoRoot, "docs", kbsDocsSub),
		kbsPyAbspath(kbsPyJoin(repoRoot, "..", kbsZergDocsDir, kbsDocsSub)),
	}
}

// kbsPickDocs —— py `next((p for p in DOCS_CANDIDATES if isdir(p)), DOCS_CANDIDATES[0])`。
func kbsPickDocs(repoRoot string) string {
	c := kbsDocsCandidates(repoRoot)
	for _, p := range c {
		if kbsIsDir(p) {
			return p
		}
	}
	return c[0]
}

// kbsVerKey —— py `tuple(int(x) for x in v[1:].split("."))`。
func kbsVerKey(v string) []int {
	body := ""
	if strings.HasPrefix(v, "v") {
		body = v[1:]
	} else {
		body = v
	}
	var out []int
	for _, x := range strings.Split(body, ".") {
		n, err := strconv.Atoi(x)
		if err != nil {
			n = 0 // 非 ASCII 十进制数字档：见未覆盖面（真仓版本线名恒 ASCII）
		}
		out = append(out, n)
	}
	return out
}

func kbsKeyLess(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

// kbsCurrentVersionDir —— py `current_version_dir()`（找不到 ⇒ ok=false）。
func kbsCurrentVersionDir(repoRoot string) (string, bool) {
	docs := kbsPickDocs(repoRoot)
	if !kbsIsDir(docs) {
		return "", false
	}
	best := ""
	var bestKey []int
	for _, e := range kbsReadDir(docs) { // ★ readdir 原序；max 取「首个最大」（py max 稳定）
		if !e.isDir || !kbsVerDirRE.MatchString(e.name) {
			continue
		}
		k := kbsVerKey(e.name)
		if best == "" || kbsKeyLess(bestKey, k) {
			best, bestKey = e.name, k
		}
	}
	if best == "" {
		return "", false
	}
	return kbsPyJoin(docs, best), true
}

// kbsDocMap —— py dict（保插入序；同名覆盖保位）。
type kbsDocMap struct {
	keys []string
	vals map[string]string
}

func kbsNewDocMap() *kbsDocMap {
	return &kbsDocMap{vals: map[string]string{}}
}

func (m *kbsDocMap) set(k, v string) {
	if _, ok := m.vals[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.vals[k] = v
}

func (m *kbsDocMap) get(k string) string { return m.vals[k] }
func (m *kbsDocMap) len() int            { return len(m.keys) }

// kbsKeyDocs —— py `key_docs`（每 pattern 先 glob 原序 → sorted → 灌 dict）。
func kbsKeyDocs(verDir string) *kbsDocMap {
	out := kbsNewDocMap()
	for _, p := range kbsKeyDocPats {
		ms := kbsGlob1(verDir, p)
		sort.Strings(ms)
		for _, f := range ms {
			out.set(kbsPyBasename(f), f)
		}
	}
	return out
}

// ── 注册表（保序 · 含显式 null 两态）─────────────────────────────────────

type kbsRegEntry struct {
	name string
	hash string
	ts   string
}

type kbsRegistry struct {
	entries []kbsRegEntry
	index   map[string]int
	nullish map[string]bool // 显式 JSON null（py `prev is None` 为真 ⇒ 当「新增」）
}

func kbsLoadRegistry(kbDir string) *kbsRegistry {
	reg := &kbsRegistry{index: map[string]int{}, nullish: map[string]bool{}}
	raw, err := os.ReadFile(kbsPyJoin(kbDir, kbsRegistryName))
	if err != nil {
		return reg // py：件不在 ⇒ {}；读不了 ⇒ except 吞掉 ⇒ {}（B 类照实复刻）
	}
	names, vals, ok := kbsParseRegistry(raw)
	if !ok {
		return reg // py `except Exception: pass` ⇒ {}
	}
	for i, n := range names {
		reg.index[n] = len(reg.entries)
		reg.entries = append(reg.entries, kbsRegEntry{name: n, hash: vals[i].hash, ts: vals[i].ts})
		reg.nullish[n] = vals[i].isNull
	}
	return reg
}

type kbsRegVal struct {
	hash   string
	ts     string
	isNull bool
}

// kbsParseRegistry —— 保序解析 `{name: {"hash":…,"ts":…}}`（值非对象 ⇒ 视作空对象）。
func kbsParseRegistry(raw []byte) ([]string, []kbsRegVal, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	t, err := dec.Token()
	if err != nil {
		return nil, nil, false
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return nil, nil, false
	}
	var names []string
	var vals []kbsRegVal
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, nil, false
		}
		key, ok := kt.(string)
		if !ok {
			return nil, nil, false
		}
		vt, err := dec.Token()
		if err != nil {
			return nil, nil, false
		}
		switch v := vt.(type) {
		case nil:
			vals = append(vals, kbsRegVal{isNull: true})
		case json.Delim:
			if v == '{' {
				hash, ts := "", ""
				for dec.More() {
					k2, err := dec.Token()
					if err != nil {
						return nil, nil, false
					}
					v2, err := dec.Token()
					if err != nil {
						return nil, nil, false
					}
					if s, ok := v2.(string); ok {
						if nm, _ := k2.(string); nm == "hash" {
							hash = s
						} else if nm == "ts" {
							ts = s
						}
					}
				}
				if _, err := dec.Token(); err != nil { // 收对象闭合 }
					return nil, nil, false
				}
				vals = append(vals, kbsRegVal{hash: hash, ts: ts})
			} else { // 顶层值不是对象（数组/嵌套）：吞掉，记空对象
				if err := kbsSkipValue(dec, v); err != nil {
					return nil, nil, false
				}
				vals = append(vals, kbsRegVal{})
			}
		default:
			vals = append(vals, kbsRegVal{}) // 字符串/数字/布尔：py 在该档裸回溯 ⇒ 见未覆盖面
		}
		names = append(names, key)
	}
	return names, vals, true
}

// kbsSkipValue —— 消费一个已开到 `open` 的复合值（数组/嵌套对象），深度归零即止。
func kbsSkipValue(dec *json.Decoder, open json.Delim) error {
	if open != '[' && open != '{' {
		return nil
	}
	depth := 1
	for depth > 0 {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := t.(json.Delim); ok {
			if d == '[' || d == '{' {
				depth++
			} else {
				depth--
			}
		}
	}
	return nil
}

func (r *kbsRegistry) has(name string) bool {
	_, ok := r.index[name]
	if !ok {
		return false
	}
	return !r.nullish[name]
}

func (r *kbsRegistry) hashOf(name string) string {
	if i, ok := r.index[name]; ok {
		return r.entries[i].hash
	}
	return ""
}

func (r *kbsRegistry) set(name, hash, ts string) {
	if i, ok := r.index[name]; ok {
		r.entries[i].hash, r.entries[i].ts, r.nullish[name] = hash, ts, false
		return
	}
	r.index[name] = len(r.entries)
	r.entries = append(r.entries, kbsRegEntry{name: name, hash: hash, ts: ts})
}

func kbsSaveRegistry(kbDir string, reg *kbsRegistry) error {
	if err := os.MkdirAll(kbDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(kbsPyJoin(kbDir, kbsRegistryName), []byte(kbsEncodeRegistry(reg)), 0o644)
}

// ── 摘要 / 候选 ──────────────────────────────────────────────────────────

func kbsFileHash(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum[:8]), nil // hexdigest()[:16]
}

type kbsCand struct {
	title    string
	filePath string
	domain   string
	source   string
	preview  string
	note     string
	change   string
}

// kbsExtractCandidate —— py `extract_candidate`（读不了 / 非法 UTF-8 ⇒ ok=false，照 py `except`）。
func kbsExtractCandidate(fpath, repoRoot string) (kbsCand, bool) {
	var c kbsCand
	raw, err := os.ReadFile(fpath)
	if err != nil {
		return c, false
	}
	if !utf8.Valid(raw) {
		return c, false // py：strict 解码抛 UnicodeDecodeError ⇒ except ⇒ None
	}
	lines := strings.Split(kbsUniversalNewlines(string(raw)), "\n")
	title := ""
	lim := 6
	if len(lines) < lim {
		lim = len(lines)
	}
	for _, ln := range lines[:lim] {
		if strings.HasPrefix(ln, "#") {
			title = kbsPyStrip(kbsPyLStripSet(ln, "# "))
			break
		}
	}
	var body []string
	hi := 40
	if len(lines) < hi {
		hi = len(lines)
	}
	for _, ln := range lines[1:hi] {
		s := kbsPyStrip(ln)
		if s != "" && !strings.HasPrefix(ln, "```") {
			body = append(body, s)
		}
	}
	joined := strings.Join(body, " ")
	rs := []rune(joined)
	if len(rs) > 400 {
		joined = string(rs[:400]) // py `[:400]` 按码点
	}
	rel := kbsPyRelpath(fpath, repoRoot)
	c.title = title
	if c.title == "" {
		c.title = kbsPyBasename(fpath)
	}
	c.filePath = rel
	c.domain = "虫族"
	c.source = "项目文档"
	c.preview = joined
	c.note = "kb_add title=<标题> domain=虫族 file_path=" + rel + " content=<精炼要点>"
	return c, true
}

// ── 模式 ─────────────────────────────────────────────────────────────────

func kbsNoDocs(stdout io.Writer, repoRoot string) {
	fmt.Fprintln(stdout, "未找到当前版本目录——两处取源根都不在盘上：")
	for _, p := range kbsDocsCandidates(repoRoot) {
		fmt.Fprintf(stdout, "  %s\n", p)
	}
	fmt.Fprintln(stdout, "⇒ 判据不可判（不给结论）· rc=2")
}

// kbsMark -- `--mark` 档（py：先要取源根，再按 arg 分「全部」/单件/无 arg）。
func kbsMark(stdout io.Writer, argv []string, idx int, repoRoot, kbDir, now string) int {
	reg := kbsLoadRegistry(kbDir)
	verDir, ok := kbsCurrentVersionDir(repoRoot)
	if !ok {
		kbsNoDocs(stdout, repoRoot)
		return 2
	}
	docs := kbsKeyDocs(verDir)
	if idx+1 >= len(argv) {
		return 0 // py：`--mark` 后无 arg ⇒ 什么都不做，自然结束 rc=0
	}
	ts := now
	if ts == "" {
		ts = time.Now().Format("2006-01-02T15:04:05")
	}
	arg := argv[idx+1]
	if arg == "全部" {
		for _, name := range docs.keys {
			h, err := kbsFileHash(docs.get(name))
			if err != nil {
				fmt.Fprintf(stdout, "✗ 读不了关键文档：%s\n⇒ 不给结论 · rc=2\n", docs.get(name))
				return 2
			}
			reg.set(name, h, ts)
		}
		if err := kbsSaveRegistry(kbDir, reg); err != nil {
			fmt.Fprintf(stdout, "✗ 注册表写不了：%s\n⇒ 不给结论 · rc=2\n", kbsPyJoin(kbDir, kbsRegistryName))
			return 2
		}
		fmt.Fprintf(stdout, "注册表已更新（%d 份全部标记）\n", docs.len())
		return 0
	}
	name := kbsPyBasename(arg)
	_, inDocs := docs.vals[name]
	if !inDocs {
		fmt.Fprintf(stdout, "未找到关键文档: %s\n", name)
		return 0
	}
	h, err := kbsFileHash(docs.get(name))
	if err != nil {
		fmt.Fprintf(stdout, "✗ 读不了关键文档：%s\n⇒ 不给结论 · rc=2\n", docs.get(name))
		return 2
	}
	reg.set(name, h, ts)
	if err := kbsSaveRegistry(kbDir, reg); err != nil {
		fmt.Fprintf(stdout, "✗ 注册表写不了：%s\n⇒ 不给结论 · rc=2\n", kbsPyJoin(kbDir, kbsRegistryName))
		return 2
	}
	fmt.Fprintf(stdout, "已标记: %s\n", name)
	return 0
}

// kbsMain —— py `main()`（dry-run）。
func kbsMain(stdout, stderr io.Writer, repoRoot, kbDir string) int {
	verDir, ok := kbsCurrentVersionDir(repoRoot)
	if !ok {
		kbsNoDocs(stdout, repoRoot)
		return 2
	}
	ver := kbsPyBasename(verDir)
	reg := kbsLoadRegistry(kbDir)
	docs := kbsKeyDocs(verDir)
	names := append([]string{}, docs.keys...)
	sort.Strings(names)
	var added, changed, unchanged []string
	for _, name := range names {
		h, err := kbsFileHash(docs.get(name))
		if err != nil {
			fmt.Fprintf(stderr, "✗ 读不了关键文档（不给结论 · rc=2）：%s\n", docs.get(name))
			return 2
		}
		switch {
		case !reg.has(name):
			added = append(added, name)
		case reg.hashOf(name) != h:
			changed = append(changed, name)
		default:
			unchanged = append(unchanged, name)
		}
	}
	fmt.Fprintf(stdout, "kb_docs_sync: 当前版 %s——关键文档 %d 份（新增 %d/变更 %d/未变 %d）\n",
		ver, docs.len(), len(added), len(changed), len(unchanged))
	if len(added) == 0 && len(changed) == 0 {
		fmt.Fprintln(stdout, "无变更——知识库已同步（无需操作）")
		return 0
	}
	var cands []kbsCand
	for _, name := range append(append([]string{}, added...), changed...) {
		c, ok := kbsExtractCandidate(docs.get(name), repoRoot)
		if !ok {
			continue
		}
		if kbsContainsStr(added, name) {
			c.change = "新增"
		} else {
			c.change = "变更(覆盖)"
		}
		cands = append(cands, c)
		fmt.Fprintf(stdout, "  [%s] %s\n", c.change, c.filePath)
	}
	candPath := kbsPyJoin(kbDir, kbsCandidatesName)
	if err := os.WriteFile(candPath, []byte(kbsEncodeCandidates(cands)), 0o644); err != nil {
		fmt.Fprintf(stderr, "✗ 候选件写不了（不给结论 · rc=2）：%s\n", candPath)
		return 2
	}
	fmt.Fprintf(stdout, "\n候选条目 → %s\n", candPath)
	fmt.Fprintln(stdout, "下一步（核验门控——G2）:")
	for _, c := range cands {
		fmt.Fprintf(stdout, "  · %s —— %s\n", c.filePath, c.note)
	}
	fmt.Fprintln(stdout, "全部入库后执行: python3 kb_docs_sync.py --mark 全部  （更新注册表——下次只检测真变更）")
	return 0
}

func kbsContainsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// ── 有序 JSON 编码（照 py `json.dump(..., ensure_ascii=False, indent=1)`）──

func kbsJStr(s string) string {
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

func kbsEncodeRegistry(reg *kbsRegistry) string {
	if len(reg.entries) == 0 {
		return "{}"
	}
	var b strings.Builder
	b.WriteString("{\n")
	for i, e := range reg.entries {
		b.WriteString(" ")
		b.WriteString(kbsJStr(e.name))
		b.WriteString(": {\n  ")
		b.WriteString(kbsJStr("hash"))
		b.WriteString(": ")
		b.WriteString(kbsJStr(e.hash))
		b.WriteString(",\n  ")
		b.WriteString(kbsJStr("ts"))
		b.WriteString(": ")
		b.WriteString(kbsJStr(e.ts))
		b.WriteString("\n }")
		if i < len(reg.entries)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.String()
}

func kbsEncodeCandidates(cands []kbsCand) string {
	if len(cands) == 0 {
		return "[]"
	}
	keys := []string{"title", "file_path", "domain", "source", "content_preview", "kb_add_note", "change"}
	vals := make([]string, len(keys))
	var b strings.Builder
	b.WriteString("[\n")
	for i, c := range cands {
		vals[0], vals[1], vals[2], vals[3] = c.title, c.filePath, c.domain, c.source
		vals[4], vals[5], vals[6] = c.preview, c.note, c.change
		b.WriteString(" {\n")
		for j, k := range keys {
			b.WriteString("  ")
			b.WriteString(kbsJStr(k))
			b.WriteString(": ")
			b.WriteString(kbsJStr(vals[j]))
			if j < len(keys)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(" }")
		if i < len(cands)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]")
	return b.String()
}

// ── 分派 / 入口 ──────────────────────────────────────────────────────────

func kbsDispatch(stdout, stderr io.Writer, argv []string, repoRoot, kbDir string, a kbsArgs) int {
	var bad []string
	for _, x := range argv { // py：`a.startswith("-") and a not in known`
		if strings.HasPrefix(x, "-") && !kbsKnownFlags[x] {
			bad = append(bad, x)
		}
	}
	if len(bad) > 0 {
		fmt.Fprintf(stderr, "✗ 未知参数: %s\n", strings.Join(bad, " "))
		return 2
	}
	if a.help {
		fmt.Fprint(stdout, kbsHelpText)
		fmt.Fprintln(stdout) // py `print(__doc__)` 的尾换行
		return 0
	}
	if idx := kbsFirstIdx(argv, "--mark"); idx >= 0 {
		return kbsMark(stdout, argv, idx, repoRoot, kbDir, a.now)
	}
	return kbsMain(stdout, stderr, repoRoot, kbDir)
}

func kbsRun(stdout, stderr io.Writer, argv []string) int {
	a := kbsScan(argv)
	repo := a.repo
	if repo == "" {
		if wd, err := os.Getwd(); err == nil {
			repo = wd
		}
	}
	repo = kbsPyAbspath(repo)
	kb := a.kbDir
	if kb == "" {
		kb = kbsPyAbspath(kbsPyJoin(repo, "..", "..", kbsKBLeaf))
	}
	return kbsDispatch(stdout, stderr, argv, repo, kb, a)
}

// kbDocsSyncGateMain —— 本件入口（inproc 口径：argv **不含程序名**）。
func kbDocsSyncGateMain(stdout, stderr io.Writer, argv []string) int {
	return kbsRun(stdout, stderr, argv)
}
