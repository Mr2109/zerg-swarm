// check_record_not_evidence.go —— 判据件 `scripts/gates/check-record-not-evidence.py` 的 Go 真搬
// （M2 · 入口 `checkRecordNotEvidenceGateMain` · 承接缺口 GAP-20261001-13）。
//
// 为什么有这件：元门「记录面不得充作证据」（R1 成员即存在 / R2 表当引用 / R3 记录即已修 ·
// 只报告档 `tri-report`）。判据面在 py 半落成后，按 v2.5.14 的 M1/M2 定案（M1 = 转发壳
// `zerg gate check <判据名>`；M2 = 按价值真搬 Go，落点 `core/cmd/zerg/`）再落一枚 Go 真搬件
// ⇒ 两个入口（`zerg gate check` 走 Go inproc · 步骤表那一步走 py）**同批**。
//
// 验收判据 = 对同一 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码**三面逐字相同**。
//
// ★ 本件已接线（inproc 表分派）（不改 main.go 命令树 · 不删 py 原件 · 不碰 gate_inproc_table.go）。
//
// ═══════════════════ 出厂声明行 (a) 默认根定位差异 ═══════════════════
//
//	py 从 `__file__` 上溯三级（`dirname(dirname(dirname(abspath(__file__))))`）得仓根；
//	Go 无 `__file__` 语境 ⇒ 本件默认根 = cwd（`os.Getwd()`）。⇒「脚本位置 ≠ cwd」时
//	`--root` 缺省推导面不同（不声称逐字对齐）。对拍一律显式传 `--root`，或把 cwd 设在仓根。
//
// ═══════════════════ 出厂声明行 (b) 自指检查 ═══════════════════
//
//	真判：本门扫描面 = `--scan-dir`（缺省 scripts + core/cmd/zerg）下的文本件；本件落在
//	`core/cmd/zerg/` 之内 ⇒ **本件在自身扫描面内**（自指形态①）。
//	消解：本件正文把被扫形态一律写成描述式/间接拼接（不写自门会命中的路径原串）；表名
//	实参全部走 `--scan-dir` / `--tables` 显式入参；夹具串只在自检临时目录里构造。
//
// 退码（与 py 逐字同）：0 未命中 · 1 有命中（只报告） · 2 判不了（不给结论）。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// crnePWS 是 CPython `\s` 在 str 上匹配的空白字符类（含 \x1c-\x1f · \x85 · \xa0 等）。
const crnePWS = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

// crneWord 是 CPython `\w` 的等价类（Unicode 字母/数字/下划线），用于复刻 py 的 `\b` 边界。
const crneWord = `\p{L}\p{N}_`

var crneRepoPrefixes = []string{"scripts/", "core/", "agent/", "ui/", "docs/", "publish/",
	"mcp/", "gateway/", "tools/", "wall/", "shared/", "dist/"}

var crneFileExt = []string{".py", ".sh", ".go", ".md", ".json", ".jsonl", ".tsv", ".csv",
	".yml", ".yaml", ".rs", ".txt", ".toml", ".js", ".ts"}

var crneTextExt = map[string]bool{}

var crneDataExt = map[string]bool{".tsv": true, ".csv": true, ".md": true,
	".jsonl": true, ".txt": true, ".json": true}

var crneNameLexicon = []string{"台账", "登记表", "登记", "名册", "清单", "标记表", "标记", "索引",
	"roster", "registry", "inventory", "ledger", "marks", "manifest",
	"index", "whitelist", "allow"}

var crneSkipDirs = map[string]bool{".git": true, "__pycache__": true, "vendor": true,
	"node_modules": true, "target": true, "dist": true, "build": true, "bin": true,
	".venv": true, "venv": true, ".mypy_cache": true}

var (
	crneMDSep    = regexp.MustCompile(`^[|\-:` + crnePWS + `]+$`)
	crneScanHint = regexp.MustCompile(`(?:^|[^` + crneWord + `])(glob|iglob|rglob|os\.walk|listdir|ls-files|walk|read_text|open[` + crnePWS + `]*\()`)
	crneRefHint  = regexp.MustCompile(`(?:^|[^` + crneWord + `])(in|not[` + crnePWS + `]+in)(?:$|[^` + crneWord + `])|\.get[` + crnePWS + `]*\(|contains[` + crnePWS + `]*\(|append[` + crnePWS + `]*\(`)
	crneDoneRe   = regexp.MustCompile(`已修|已闭|已解|已登记|已审|已销|已清|关闭|fixed|closed|resolved|done`)
	crneReconRe  = regexp.MustCompile(`sha256|sha16|sha1|commit|rc=|rc(?:$|[^` + crneWord + `])|对拍|diff|命令|cmd|--|回读|现跑|verify`)
	crneSrcDecl  = regexp.MustCompile(`唯一真源|单一真源|唯一来源|单一来源|唯一来源件|唯一真源件`)
	crneBidirRe  = regexp.MustCompile(`双向|差集|对拍`)
	crneSha64    = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// crneReadFails —— B 类读失败登记面（读不了的件/目录 ⇒ 逐件登记 相对路径 + err 正文）。
// ★ B 类真口子修（与 py 有意分叉 · 同族先例 check_doc_cmds.go 的 dcmdReadFails）：
//
//	读失败 ⇒ 登记 + 入口判红（stderr BLOCKED 头 + 逐件点名）⇒ rc=2（✗ 不再静默少判）。
//	自清点：每次 crneTextFiles 起手清空 ⇒ 不许跨调用残留。
var crneReadFails []string

func init() {
	for _, e := range crneFileExt {
		crneTextExt[e] = true
	}
}

// crneFinding 一条命中（JSON 键序与 py dict 插入序逐字同：rule · where · why）。
type crneFinding struct {
	Rule  string `json:"rule"`
	Where string `json:"where"`
	Why   string `json:"why"`
}

// crneMeta 是 --json 的 meta 段（键序与 py 逐字同）。
type crneMeta struct {
	Root     string   `json:"root"`
	ScanDirs []string `json:"scan_dirs"`
	Surfaces []string `json:"surfaces"`
	Scanned  int      `json:"scanned"`
	Findings int      `json:"findings"`
	// SurfaceReads —— ①「当场印出抽到 N 条」的读数（逐候选的行数 / 仓内路径数）。
	// ★ `json:"-"`：**不改** `--json` 的 meta 形状（键序与 py 逐字同）—— 只在文本档照出（crneReport）。
	SurfaceReads []string `json:"-"`
}

type crneOut struct {
	Meta     crneMeta      `json:"meta"`
	Findings []crneFinding `json:"findings"`
}

// crneRow 是 read_table 的一行（首字段 · 原行 · 全部单元格）。
type crneRow struct {
	first string
	raw   string
	cells []string
}

// ── py 原语同构 ─────────────────────────────────────────────────────────────

func crneIsPyWS(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0,
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func crnePyStrip(s string) string {
	return strings.TrimFunc(s, crneIsPyWS)
}

// crnePyJoin 复刻 CPython posixpath.join 的逐段规则（含 .. 段 / 后段绝对 / 空段三态）。
func crnePyJoin(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		if strings.HasPrefix(p, "/") {
			b.Reset()
			b.WriteString(p)
			continue
		}
		if b.Len() == 0 || strings.HasSuffix(b.String(), "/") {
			b.WriteString(p)
			continue
		}
		b.WriteString("/")
		b.WriteString(p)
	}
	return b.String()
}

// crnePyBase 复刻 posixpath.basename。
func crnePyBase(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return p
	}
	return p[i+1:]
}

// crneExt 复刻 posixpath.splitext(...)[1]。
func crneExt(p string) string {
	base := crnePyBase(p)
	i := strings.LastIndex(base, ".")
	if i <= 0 {
		return ""
	}
	return base[i:]
}

func crneHasExtSuffix(s string, exts []string) bool {
	for _, e := range exts {
		if strings.HasSuffix(s, e) {
			return true
		}
	}
	return false
}

// crneReplaceErrors 近似 CPython errors='replace'（按非法字节序列替换；粒度差见未覆盖面）。
func crneReplaceErrors(b []byte) string {
	return strings.ToValidUTF8(string(b), "\uFFFD")
}

// crneUniversalNewlines 复刻文本模式 universal newlines（\r\n 与裸 \r 折成 \n）。
func crneUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// crneRequireDomain 复刻 py 的同名空集守卫（同 check-empty-domain-guards.py 口径）。
func crneRequireDomain(n, minCount, rc int, what string, stderr io.Writer) int {
	if n < minCount {
		fmt.Fprintf(stderr, "BLOCKED —— 空域 / 取不到输入：%s（现读 %d < 写死下限 %d）⇒ rc=%d 不给结论\n",
			what, n, minCount, rc)
		return rc
	}
	return 0
}

// ── 判据面 ──────────────────────────────────────────────────────────────────

func crneIsRepoPath(tok string) bool {
	t := crnePyStrip(tok)
	t = strings.Trim(t, "`\"'")
	t = strings.TrimRight(t, ",")
	if t == "" || strings.Contains(t, " ") || len([]rune(t)) < 3 {
		return false
	}
	for _, p := range crneRepoPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return strings.Contains(t, "/") && crneHasExtSuffix(t, crneFileExt)
}

func crneNameHit(path string) bool {
	low := strings.ToLower(path)
	for _, k := range crneNameLexicon {
		if strings.Contains(low, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func crneReadDirUnsorted(dir string) []os.DirEntry {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer f.Close()
	ents, err := f.ReadDir(-1)
	if err != nil && len(ents) == 0 {
		return nil
	}
	return ents
}

// crneWalk 复刻 os.walk(followlinks=False)：先 yield 本层全部文件（readdir 原序），
// 再按 dirs 原序下钻；符号链接目录列进 dirs 但不下钻。
func crneWalk(relDir, absDir string, order *[]string, lookup map[string]string) {
	ents := crneReadDirUnsorted(absDir)
	if ents == nil {
		// ★ B 类真口子修（原 py `os.walk(onerror=None)` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
		crneReadFails = append(crneReadFails, fmt.Sprintf("%s（读不了目录）", relDir))
		return
	}
	var dirs, files []string
	for _, e := range ents {
		name := e.Name()
		if crneIsDirFollow(crnePyJoin(absDir, name)) {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	for _, fn := range files {
		if !crneTextExt[crneExt(fn)] {
			continue
		}
		ap := crnePyJoin(absDir, fn)
		st, err := os.Stat(ap)
		if err != nil {
			continue
		}
		if st.Size() > 2*1024*1024 {
			continue
		}
		b, err := os.ReadFile(ap)
		if err != nil {
			// ★ B 类真口子修（原 py `except OSError: continue` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
			crneReadFails = append(crneReadFails, fmt.Sprintf("%s/%s（%v）", relDir, fn, err))
			continue
		}
		rel := relDir + "/" + fn
		if _, ok := lookup[rel]; !ok {
			*order = append(*order, rel)
		}
		lookup[rel] = crneUniversalNewlines(crneReplaceErrors(b))
	}
	for _, dn := range dirs {
		if crneSkipDirs[dn] {
			continue
		}
		np := crnePyJoin(absDir, dn)
		if crneIsSymlink(np) {
			continue
		}
		crneWalk(relDir+"/"+dn, np, order, lookup)
	}
}

// crneTextFiles 复刻 py text_files：返回插入序（= os.walk 序）+ 查值表。
func crneTextFiles(root string, dirs []string) ([]string, map[string]string) {
	crneReadFails = nil // ★ 自清：包级登记面不许跨调用残留
	order := []string{}
	lookup := map[string]string{}
	for _, d := range dirs {
		base := crnePyJoin(root, d)
		if !crneIsDir(base) {
			continue
		}
		crneWalk(d, base, &order, lookup)
	}
	return order, lookup
}

func crneIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func crneIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func crneIsDirFollow(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func crneIsSymlink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

func crneExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// crneReadTable 复刻 py read_table。
func crneReadTable(text string) (string, []crneRow) {
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if crnePyStrip(l) != "" {
			lines = append(lines, l)
		}
	}
	var comments []string
	for _, l := range lines {
		s := crnePyStrip(l)
		if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") {
			comments = append(comments, l)
		}
	}
	header := ""
	hasHeader := false
	rows := []crneRow{}
	for _, l := range lines {
		s := crnePyStrip(l)
		if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//") {
			continue
		}
		if crneMDSep.MatchString(s) {
			continue
		}
		var cells []string
		if strings.Count(s, "|") >= 2 {
			mid := strings.Trim(s, "|")
			for _, c := range strings.Split(mid, "|") {
				cells = append(cells, crnePyStrip(c))
			}
		} else if strings.Contains(l, "\t") {
			for _, c := range strings.Split(l, "\t") {
				cells = append(cells, crnePyStrip(c))
			}
		} else if strings.Contains(l, ",") && strings.Count(l, ",") >= 2 {
			for _, c := range strings.Split(l, ",") {
				cells = append(cells, crnePyStrip(c))
			}
		} else {
			for _, c := range strings.Split(s, " ") {
				if c != "" {
					cells = append(cells, c)
				}
			}
			if len(cells) == 0 {
				cells = strings.Fields(s)
			}
		}
		if len(cells) == 0 {
			continue
		}
		first := strings.Trim(cells[0], "`\"'")
		first = crnePyStrip(first)
		if !hasHeader && !crneIsRepoPath(first) {
			header = l
			hasHeader = true
			continue
		}
		rows = append(rows, crneRow{first: first, raw: l, cells: cells})
	}
	headParts := append([]string{}, comments...)
	if hasHeader {
		headParts = append(headParts, header)
	}
	return strings.Join(headParts, "\n"), rows
}

func crneSortedSet(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// crneAnalyze 复刻 py analyze。返回 (findings, meta, 插入序, 查值表)。
func crneAnalyze(root string, dirs, tables []string) ([]crneFinding, crneMeta, []string, map[string]string) {
	order, lookup := crneTextFiles(root, dirs)
	findings := []crneFinding{}
	var surfaces []string
	// ① 逐候选当场收集「抽到 N 条」读数（文本档照出 ⇒ 读者可自查读数与结构是否相符）。
	var surfaceReads []string
	if len(tables) > 0 {
		surfaces = append(surfaces, tables...)
	} else {
		for _, rel := range order {
			if !crneDataExt[crneExt(rel)] {
				continue
			}
			if !crneNameHit(rel) {
				continue
			}
			text := lookup[rel]
			_, rows := crneReadTable(text)
			paths := 0
			for _, r := range rows {
				if crneIsRepoPath(r.first) {
					paths++
				}
			}
			if len(rows) >= 5 && paths*2 >= len(rows) {
				// ① 当场印出抽到 N 条（分母可读 ⇒ 读者自查读数与结构是否相符 · ✗ 不只印结论）。
				surfaceReads = append(surfaceReads,
					fmt.Sprintf("%s：记录面候选 · 抽到 %d 行（其中 %d 条仓内路径）", rel, len(rows), paths))
				surfaces = append(surfaces, rel)
			}
		}
	}
	surfaces = crneSortedSet(surfaces)

	for _, rel := range surfaces {
		ap := crnePyJoin(root, rel)
		if !crneIsFile(ap) {
			findings = append(findings, crneFinding{"R?", rel + ":0",
				"声明的记录面不在盘上（判不了）"})
			continue
		}
		text, ok := lookup[rel]
		if !ok {
			b, err := os.ReadFile(ap)
			if err != nil {
				// ★ B 类真口子修（原 py `except OSError: continue` 静默 · ✗ 不再静默）：登记 + 判红 + rc=2。
				crneReadFails = append(crneReadFails, fmt.Sprintf("%s（%v）", rel, err))
				continue
			}
			text = crneUniversalNewlines(crneReplaceErrors(b))
		}
		header, rows := crneReadTable(text)
		var members []crneRow
		for _, r := range rows {
			if crneIsRepoPath(r.first) {
				members = append(members, r)
			}
		}
		// ① 当场印出抽到 N 条（不足下界 ⇒ 判非记录面 · 读数照出 ✗ 不静默丢弃）。
		if len(rows) < 5 || len(members)*2 < len(rows) {
			surfaceReads = append(surfaceReads, fmt.Sprintf("%s：抽到 %d 行（不足下界或成员不足半数 ⇒ 判非记录面）", rel, len(rows)))
			continue
		}
		headRows := []string{}
		for i, r := range rows {
			if i >= 3 {
				break
			}
			headRows = append(headRows, r.raw)
		}
		headText := header + "\n" + strings.Join(headRows, "\n")
		srcExempt := crneSrcDecl.MatchString(headText) && crneBidirRe.MatchString(headText)

		pairExempt := len(members) > 0
		for _, m := range members {
			ap2 := crnePyJoin(root, m.first)
			if !crneExists(ap2) {
				pairExempt = false
				break
			}
			want := ""
			for _, c := range m.cells {
				if crneSha64.MatchString(crnePyStrip(c)) {
					want = crnePyStrip(c)
					break
				}
			}
			if want != "" && crneIsFile(ap2) {
				b, err := os.ReadFile(ap2)
				var got string
				if err == nil {
					sum := sha256.Sum256(b)
					got = hex.EncodeToString(sum[:])
				}
				if got != want {
					pairExempt = false
					break
				}
			}
		}

		if !pairExempt {
			for _, m := range members {
				indep := 0
				for _, other := range order {
					if other == rel {
						continue
					}
					if strings.Contains(lookup[other], m.first) {
						indep++
					}
				}
				if indep == 0 {
					findings = append(findings, crneFinding{"R1", rel + ":1",
						fmt.Sprintf("成员即存在却无独立现读：`%s` 在记录面外 0 处", m.first)})
				}
			}
		}

		if !srcExempt {
			for _, other := range order {
				ext := crneExt(other)
				if other == rel || (ext != ".py" && ext != ".sh" && ext != ".go") {
					continue
				}
				base := crnePyBase(rel)
				otext := lookup[other]
				if !strings.Contains(otext, base) && !strings.Contains(otext, rel) {
					continue
				}
				if crneScanHint.MatchString(otext) && crneRefHint.MatchString(otext) {
					findings = append(findings, crneFinding{"R2", other + ":1",
						fmt.Sprintf("引用 / 扫描面含记录面 `%s`（命中即算引用）", rel)})
				}
			}
		}

		if !pairExempt {
			for _, r := range rows {
				if crneDoneRe.MatchString(r.raw) && !crneReconRe.MatchString(r.raw) {
					findings = append(findings, crneFinding{"R3", rel + ":1",
						fmt.Sprintf("记录即已修（无对盘字段）：`%s`", crneTruncRunes(crnePyStrip(r.raw), 60))})
				}
			}
		}
	}

	meta := crneMeta{Root: root, ScanDirs: append([]string{}, dirs...),
		Surfaces: surfaces, SurfaceReads: surfaceReads, Scanned: len(order), Findings: len(findings)}
	return findings, meta, order, lookup
}

func crneTruncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// crneReport 复刻 py report（文本档 / --json 档）。
func crneReport(stdout io.Writer, findings []crneFinding, meta crneMeta, asJSON bool) int {
	if asJSON {
		out := crneOut{Meta: meta, Findings: findings}
		if out.Findings == nil {
			out.Findings = []crneFinding{}
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", " ")
		if err := enc.Encode(out); err != nil {
			return 2
		}
		fmt.Fprint(stdout, buf.String())
		if len(findings) > 0 {
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, "── check-record-not-evidence：记录面不得充作证据（R1/R2/R3 · 只报告）──")
	fmt.Fprintf(stdout, "  扫描面：%s（%d 个记录面候选）\n",
		strings.Join(meta.ScanDirs, " · "), len(meta.Surfaces))
	for _, s := range meta.Surfaces {
		fmt.Fprintf(stdout, "   · 记录面：%s\n", s)
	}
	// ① 读数照出（抽到 N 条 · 分母可读）：逐候选的「抽到几行 / 几条仓内路径」。
	for _, rd := range meta.SurfaceReads {
		fmt.Fprintf(stdout, "   ◦ 读数：%s\n", rd)
	}
	for _, r := range []string{"R1", "R2", "R3"} {
		hits := []crneFinding{}
		for _, f := range findings {
			if f.Rule == r {
				hits = append(hits, f)
			}
		}
		fmt.Fprintf(stdout, "  【%s】命中 %d 条\n", r, len(hits))
		for i, h := range hits {
			if i >= 20 {
				break
			}
			fmt.Fprintf(stdout, "    · %s  %s\n", h.Where, h.Why)
		}
		if len(hits) > 20 {
			fmt.Fprintf(stdout, "    … 另 %d 条（--json 取全量）\n", len(hits)-20)
		}
	}
	verdict := "未命中"
	rc := 0
	if len(findings) > 0 {
		verdict = "有命中 · 只报告（不阻断）"
		rc = 1
	}
	fmt.Fprintf(stdout, "⇒ %s（rc=%d）\n", verdict, rc)
	return rc
}

func crneRepoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

// ── 自检（合成夹具成对负控 · 不碰真目标）─────────────────────────────────────

func crneMk(path, content string) error {
	if err := os.MkdirAll(crneDirOf(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func crneDirOf(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return "."
	}
	return p[:i]
}

func crneReprBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func crneSelfTest(stdout io.Writer) int {
	base, err := os.MkdirTemp("", "record-not-evidence-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "✗ 自检：建临时目录失败：%v\n", err)
		return 1
	}
	defer os.RemoveAll(base)

	type crneCheck struct {
		label string
		ok    bool
		got   string
		want  string
	}
	var checks []crneCheck
	nf := 0
	report := func(label, got, want string) {
		ok := got == want
		checks = append(checks, crneCheck{label, ok, got, want})
		if !ok {
			nf++
		}
		mark := "✓"
		if !ok {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s ⇒ 得 %s（要 %s）\n", mark, label, got, want)
	}

	// ① must-fail：喂表必报（成员不在盘上 + 有代码面把表当引用源）
	d := base + "/feed"
	_ = crneMk(crnePyJoin(d, "scripts", "名册.tsv"),
		"件\t说明\n"+crneRepeatGhost("scripts/ghost%d.py\tghost zero-ref\n", 6))
	_ = crneMk(crnePyJoin(d, "scripts", "scan.py"),
		"import glob, os\nfor p in glob.glob('scripts/*'):\n"+
			"    if os.path.basename(p) in open('scripts/名册.tsv').read():\n"+
			"        print('counted as referenced')\n")
	fs, _, _, _ := crneAnalyze(d, []string{"scripts"}, nil)
	report("① must-fail · 喂表必报（R1 或 R2 命中 ≥1）", crneReprBool(len(fs) >= 1), "True")

	// ② must-not-report：双向对拍（每条都在盘上 + sha 对上 + 表头声明唯一真源 + 双向）
	d2 := base + "/reconcile"
	var rowList []string
	for i := 0; i < 5; i++ {
		p := fmt.Sprintf("scripts/real%d.py", i)
		_ = crneMk(crnePyJoin(d2, p), fmt.Sprintf("# real %d\n", i))
		b, _ := os.ReadFile(crnePyJoin(d2, p))
		sum := sha256.Sum256(b)
		rowList = append(rowList, p+"\t"+hex.EncodeToString(sum[:]))
	}
	_ = crneMk(crnePyJoin(d2, "scripts", "清单.tsv"),
		"# 唯一真源：本表即 scripts/ 的单一来源；漂移由 check-x.py 双向差集管\n"+
			"件\tsha256\n"+strings.Join(rowList, "\n")+"\n")
	_ = crneMk(crnePyJoin(d2, "scripts", "check-x.py"),
		"import glob\nnames = open('scripts/清单.tsv').read()\n"+
			"for p in glob.glob('scripts/*'):\n    assert p in names\n")
	fs2, _, _, _ := crneAnalyze(d2, []string{"scripts"}, nil)
	report("② must-not-report · 双向对拍 + 真源 + 对拍豁免 ⇒ 0 命中", strconv.Itoa(len(fs2)), "0")

	// ③ must-fail：R3 记录即已修（断言完成 · 无对盘字段）
	d3 := base + "/done"
	_ = crneMk(crnePyJoin(d3, "scripts", "台账.tsv"),
		"件\t状态\n"+crneRepeatGhost("scripts/ghost%d.py\t已修\n", 6))
	fs3, _, _, _ := crneAnalyze(d3, []string{"scripts"}, nil)
	r3 := false
	for _, f := range fs3 {
		if f.Rule == "R3" {
			r3 = true
		}
	}
	report("③ must-fail · 记录即已修（R3 命中 ≥1）", crneReprBool(r3), "True")

	// ④ 真源豁免：声明唯一真源 + 双向 ⇒ 不报 R2（即使有扫描面读它）
	d4 := base + "/src"
	_ = crneMk(crnePyJoin(d4, "scripts", "registry.tsv"),
		"# 唯一真源：本表即 scripts/ 的单一来源；漂移由 check-y.py 双向差集管\n"+
			"件\t说明\n"+crneRepeatGhost("scripts/ghost%d.py\tg\n", 6))
	_ = crneMk(crnePyJoin(d4, "scripts", "check-y.py"),
		"import glob\nS = set(open('scripts/registry.tsv').read().split())\n"+
			"for p in glob.glob('scripts/*'):\n    print(p in S)\n")
	fs4, _, _, _ := crneAnalyze(d4, []string{"scripts"}, nil)
	r2 := false
	for _, f := range fs4 {
		if f.Rule == "R2" {
			r2 = true
		}
	}
	report("④ 真源豁免 ⇒ 不报 R2", crneReprBool(r2), "False")

	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n",
		map[bool]string{true: "全过", false: "不过"}[nf == 0], len(checks), nf)
	if nf == 0 {
		return 0
	}
	return 1
}

func crneRepeatGhost(tmpl string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, tmpl, i)
	}
	return b.String()
}

// ── 入口（argparse 同构的旗标面）────────────────────────────────────────────

type crneArgs struct {
	root     string
	scanDir  []string
	tables   []string
	json     bool
	selfTest bool
	help     bool
}

func crneIsOption(tok string) bool {
	return len(tok) >= 2 && strings.HasPrefix(tok, "--")
}

func crneParseArgs(argv []string) (crneArgs, string) {
	a := crneArgs{}
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		switch {
		case tok == "-h" || tok == "--help":
			a.help = true
		case tok == "--json":
			a.json = true
		case tok == "--self-test":
			a.selfTest = true
		case tok == "--root":
			if i+1 >= len(argv) {
				return a, "argument --root: expected one argument"
			}
			i++
			a.root = argv[i]
		case strings.HasPrefix(tok, "--root="):
			a.root = strings.TrimPrefix(tok, "--root=")
		case tok == "--scan-dir":
			if i+1 >= len(argv) {
				return a, "argument --scan-dir: expected one argument"
			}
			i++
			a.scanDir = append(a.scanDir, argv[i])
		case strings.HasPrefix(tok, "--scan-dir="):
			a.scanDir = append(a.scanDir, strings.TrimPrefix(tok, "--scan-dir="))
		case tok == "--tables":
			for i+1 < len(argv) && !crneIsOption(argv[i+1]) {
				i++
				a.tables = append(a.tables, argv[i])
			}
		case strings.HasPrefix(tok, "--tables="):
			a.tables = append(a.tables, strings.TrimPrefix(tok, "--tables="))
		default:
			return a, "unrecognized arguments: " + tok
		}
	}
	return a, ""
}

const crneUsage = "usage: check-record-not-evidence.py [-h] [--root ROOT] [--scan-dir SCAN_DIR]\n" +
	"                                    [--tables [TABLES ...]] [--json]\n" +
	"                                    [--self-test]\n"

const crneHelpText = crneUsage +
	"\n" +
	"记录面不得充作证据（R1/R2/R3 元门 · 只报告）\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help            show this help message and exit\n" +
	"  --root ROOT           仓根（默认从本件位置上溯三层）\n" +
	"  --scan-dir SCAN_DIR   扫描目录（可重复；默认 scripts + core/cmd/zerg）\n" +
	"  --tables [TABLES ...]\n" +
	"                        显式指定记录面件（相对仓根；给了就不自动发现）\n" +
	"  --json                机器可读输出\n" +
	"  --self-test           合成夹具成对负控（不碰真目标）\n"

// checkRecordNotEvidenceGateMain 是 `zerg gate check check-record-not-evidence` 的进程内入口。
func checkRecordNotEvidenceGateMain(stdout, stderr io.Writer, argv []string) int {
	a, errMsg := crneParseArgs(argv)
	if errMsg != "" {
		fmt.Fprintf(stderr, "%scheck-record-not-evidence.py: error: %s\n", crneUsage, errMsg)
		return 2
	}
	if a.help {
		fmt.Fprint(stdout, crneHelpText)
		return 0
	}
	if a.selfTest {
		return crneSelfTest(stdout)
	}
	root := a.root
	if root == "" {
		root = crneRepoRoot()
	}
	if !crneIsDir(root) {
		fmt.Fprintf(stdout, "⛔ 仓根不在：%s ⇒ 不给结论（rc=2）\n", root)
		return 2
	}
	dirs := a.scanDir
	if len(dirs) == 0 {
		dirs = []string{"scripts", "core/cmd/zerg"}
	}
	findings, meta, _, _ := crneAnalyze(root, dirs, a.tables)
	// ★ B 类真口子修：读失败 ⇒ 响亮 BLOCKED rc=2 + 逐件点名（✗ 不再静默少判）。
	if len(crneReadFails) > 0 {
		fmt.Fprintf(stderr, "BLOCKED —— 有 %d 件读不到（B 类读失败 ⇒ 登记 + 判红 · rc=2 · 不给结论）：\n", len(crneReadFails))
		for _, u := range crneReadFails {
			fmt.Fprintf(stderr, "  [读失败] %s\n", u)
		}
		return 2
	}
	if crneRequireDomain(meta.Scanned, 1, 2,
		"扫描面 "+strings.Join(dirs, " · ")+" 下的文本件", stderr) != 0 {
		return 2
	}
	return crneReport(stdout, findings, meta, a.json)
}
