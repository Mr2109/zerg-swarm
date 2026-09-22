// family_impact_docindex.go —— `C3` 文档面与反向索引（码 ⇒ 文档）。
//
// 依据（逐字）：`设计-变更影响面-v1.6` §6.3（码⇒文档这一向的接法）/ §6.4（两棵树不是一个域）/
// §6.6 第 4 条（**不写口径的「提到」一律读作面内未见**）/ §十 `R23`（**一次**扫全量 md 建
// 「词 → 篇」倒排 · 产物落 `<状态目录>/impact-index/` · 键含**文档仓 `head_sha`** ·
// **默认口径 B 档**（反引号包住的符号名）· A 档随 `--wide` · C 档随 `--strict`）；
// 契约登记表 `S-k`（真源 = `core/internal/contract/impact-docindex.json`，本件只读它）。
//
// 任务单 §四 `C3` 八字段判据①–⑤ 的落点：
//
//	① **倒排可跑**：现测（本机 · 现跑重测见回执）—— 件 / 行 / 块 / 词 / 对 + **耗时**；
//	   对照**逐符号全扫** 0.147s/符号（3,500 条「件:行」≈ 8.6 分钟）⇒ **不许逐符号** ✗；
//	② **产物键含文档仓 `head_sha`**：文件名 = `docindex-<head 前 8>-<口径 sha 前 8>.tsv`，
//	   清单件里 `head_sha` 逐字写；**`head_sha` 变了 ⇒ 旧件不许命中**（重扫）；
//	③ **三档口径可切**：A 裸词（`--wide`）/ B 反引号包住的符号名（默认）/ C 与「件:行」同行（`--strict`）
//	   —— 三档取法全在契约件里，代码不另写第二份；
//	④ **`N = 0` 必写「面内未见」+ 面内定义**（不许写成「无文档受影响」✗）；
//	⑤ **通用词假阳必须排除**（`fetch` / `Retry` / `writeError` / `WebSearch` —— 名单在契约件里，
//	   排除计数逐词落清单件）。
//
// 落地形态（为什么是这样）：
//
//	· 本件只做**一次扫描**（走一趟目录遍历读 `.md`）—— 不逐符号、不扩 `git grep`、不引 Doxygen ✗；
//	· 索引是**派生件**，落点在**状态目录**下（不在仓内 ⇒ 不进公开面、不进 git 跟踪）；目录名读
//	  `impact-state.json` 的 `dirs.index`（`S-h` 那一处真源）⇒ 源码里不写第二份；
//	· **默认档**：命中已有索引 ⇒ 取值（毫秒级）；索引不在 / 不是当前文档仓 `head_sha` ⇒ 照实写
//	  「未建索引（按需档会现建）」；**按需档** `--all` ⇒ 现建一次并把人读读数打进层表；
//	· **不许接**契约件 `不许接的动作` 里那三样自动化（名单只在契约件里读，本件一个字都不写）；本件没有删除动作。
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// impactDocIndexContractRel —— 文档面反向索引的契约真源（登记表 `S-k` · **只读**）。
const impactDocIndexContractRel = "core/internal/contract/impact-docindex.json"

// impactDocIndexSchema —— 契约件形状号（不认 ⇒ 不给结论，不猜三档取法）。
const impactDocIndexSchema = "zerg/impact-docindex/1"

// impactDocTiers —— 三档的档号（A 裸词 / B 反引号包住的符号名 / C 与「件:行」同行）。
// 默认档由契约件 `口径三档.默认档` 给（本件不写死「默认 = B」）。
var impactDocTiers = []string{"A", "B", "C"}

// impactDocWordRe —— 词素（ASCII 标识符 · **大小写原样**：`Retry` 与 `retry` 不同源）。
var impactDocWordRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]{1,}`)

// impactDocTickRe —— 反引号包住的片段（B / C 两档的取词面）。
var impactDocTickRe = regexp.MustCompile("`([^`]+)`")

// impactDocRefRe —— 「件:行」引用（C 档的门槛：该行必须有这么一处引用）。
var impactDocRefRe = regexp.MustCompile(`[A-Za-z0-9_./\-]+\.(?:go|py|sh|rs|md|json|ya?ml|toml):\d+`)

// impactDocIndexContract —— 契约件（字段名与契约件键一一对应；多一格都不要）。
type impactDocIndexContract struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Face   struct {
		RootEnv    string   `json:"根_环境变量"`
		RootDflt   string   `json:"根_默认"`
		Definition string   `json:"面内定义"`
		Ext        string   `json:"后缀"`
		Skip       []string `json:"排除目录"`
	} `json:"面"`
	Tiers map[string]struct {
		Name string `json:"名"`
		Flag string `json:"旗标"`
		Take string `json:"取法"`
	} `json:"口径三档"`
	DefaultTier string   `json:"默认档"`
	Stop        []string `json:"通用词假阳排除"`
	BlockText   string   `json:"分块口径"`
	BlockLine   int      `json:"每块行数"`
	Product     struct {
		Dir      string   `json:"落点"`
		File     string   `json:"文件名"`
		Manifest string   `json:"清单件"`
		Keys     []string `json:"键"`
		Cols     []string `json:"TSV 列"`
		Rule     string   `json:"纪律"`
	} `json:"产物"`
	Zero   string   `json:"零命中"`
	NoAct2 []string `json:"不许接的动作"`
}

// impactDocIndexContractOf 现读契约件（只读真源；读不到 / 形状不认 / 三档不成形 ⇒ 明写原因）。
func impactDocIndexContractOf(root string) (impactDocIndexContract, string) {
	var c impactDocIndexContract
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(impactDocIndexContractRel)))
	if err != nil {
		return c, "契约件读不到（" + err.Error() + "）"
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, "契约件解不开（" + err.Error() + "）"
	}
	if c.Schema != impactDocIndexSchema {
		return c, fmt.Sprintf("形状号不认（`schema`=%q ≠ %q ⇒ 不给结论）", c.Schema, impactDocIndexSchema)
	}
	if strings.TrimSpace(c.Face.Ext) == "" || strings.TrimSpace(c.Face.Definition) == "" || len(c.Tiers) < 3 {
		return c, "键不成形（`面.后缀` / `面.面内定义` / `口径三档` 缺一 ⇒ 不给结论）"
	}
	if _, ok := c.Tiers[c.DefaultTier]; !ok {
		return c, fmt.Sprintf("`默认档` = %q 不在 `口径三档` 里（%v）⇒ 不给结论（默认档归契约件，不自选）",
			c.DefaultTier, impactDocTiers)
	}
	return c, ""
}

// impactDocFaceRoot 文档面根（环境变量优先 = 契约件 `面.根_环境变量`；否则仓根的 `面.根_默认`）。
func impactDocFaceRoot(root string, c impactDocIndexContract) string {
	if v := strings.TrimSpace(os.Getenv(c.Face.RootEnv)); v != "" {
		if filepath.IsAbs(v) {
			return v
		}
		return filepath.Join(root, filepath.FromSlash(v))
	}
	dflt := c.Face.RootDflt
	if i := strings.Index(dflt, "（"); i > 0 {
		dflt = strings.TrimSpace(dflt[:i]) // 契约件那一格带了括号说明 ⇒ 取路径那一段
	}
	return filepath.Join(root, filepath.FromSlash(dflt))
}

// impactDocHeadSHA 文档仓的 `head_sha`（**键的三件之一**）；取不到 ⇒ 空串（调用方不给结论）。
func impactDocHeadSHA(docRoot string) string {
	out, _, code, err := impactRunIn(docRoot, "git", "rev-parse", "HEAD")
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}

// impactDocHit 一条「词 → 篇」命中。
type impactDocHit struct {
	Word string
	Doc  string // 相对**文档面根**
	Line int    // 首现行（1 起）
}

// impactDocIndex 一份倒排（内存里的视图 + 落盘读数）。
type impactDocIndex struct {
	OK        bool
	Why       string
	Root      string
	Head      string
	Caliber   string
	TSVPath   string
	Manifest  string // 清单件路径
	Files     int
	Lines     int
	Blocks    int
	BlockLine int // 每块行数（契约件 `每块行数` · 与 `S-h` 的 `index.chunk_policy` 同源）
	Words     int
	Pairs     int
	Bytes     int64
	BuiltMS   int64
	BuiltAt   string
	TierRows  map[string]int
	TierWords map[string]int
	StopCount map[string]int
	// 倒排本体：档 → 词 → 篇 → 首现行
	Tier map[string]map[string]map[string]int
}

// impactDocIndexDir 索引落点目录（目录名读 `impact-state.json` 的 `dirs.index` —— `S-h` 那处真源）。
func impactDocIndexDir(root string) (string, string) {
	if lay, err := impactStateLayoutOf(root); err == nil {
		return impactCacheIndexDir(lay), ""
	} else {
		return "", "落点契约件读不到（" + impactStateContractRel + "：" + err.Error() + "）"
	}
}

// impactDocIndexNames 索引两个文件名（TSV + 清单件）—— 键 = **文档仓 `head_sha` 前 8** +
// **口径串的 `sha256` 前 8**（`shortHex` 只给「人读短串」，进文件名的那一格必须是真摘要 ⇒ 不拿它拼路径）。
func impactDocIndexNames(dir, head, caliber string) (tsv, manifest string) {
	h := strings.TrimSpace(head)
	if len(h) > 8 {
		h = h[:8]
	}
	sum := sha256.Sum256([]byte(caliber))
	base := filepath.Join(dir, "docindex-"+h+"-"+hex.EncodeToString(sum[:])[:8])
	return base + ".tsv", base + ".json"
}

// impactDocIndexCaliber 口径串（进文件名那一段 sha）：三档取法 + 分块 + 档号集合 —— 任何一格变了就换键。
func impactDocIndexCaliber(c impactDocIndexContract) string {
	parts := []string{c.Schema}
	for _, t := range impactDocTiers {
		if v, ok := c.Tiers[t]; ok {
			parts = append(parts, t+"="+v.Take)
		}
	}
	parts = append(parts, fmt.Sprintf("block=%d", c.BlockLine))
	return strings.Join(parts, " | ")
}

// impactDocIndexBuild **一次扫全量 md** 建倒排（件 / 行 / 块 三个体量读数与词 / 对一起落清单件）。
//
// 为什么一次扫就够：三档要的词都从**同一遍读**里分档取出（A 取行内全部词素 · B 取反引号里的 ·
// C = B ∩ 该行有「件:行」引用）—— 逐符号全扫是另一条路（0.147s/符号 ⇒ 3,500 条 ≈ 8.6 分钟），
// 本件**不走**它 ✗。
func impactDocIndexBuild(docRoot, dir string, c impactDocIndexContract) (impactDocIndex, error) {
	start := time.Now()
	idx := impactDocIndex{
		Root: docRoot, Head: impactDocHeadSHA(docRoot), Caliber: impactDocIndexCaliber(c),
		TierRows: map[string]int{}, TierWords: map[string]int{}, StopCount: map[string]int{},
		Tier: map[string]map[string]map[string]int{},
	}
	idx.OK = true
	for _, t := range impactDocTiers {
		idx.Tier[t] = map[string]map[string]int{}
		idx.StopCount[t] = 0
	}
	stop := map[string]bool{}
	for _, w := range c.Stop {
		stop[w] = true
	}
	if idx.Head == "" {
		return idx, fmt.Errorf("文档仓 `head_sha` 取不到（%s）⇒ 不给结论（键的三件之一缺）", docRoot)
	}

	// ① 收件清单（排好序 ⇒ 两跑逐字相同）。
	paths := []string{}
	walkErr := filepath.Walk(docRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if p == docRoot {
				return nil
			}
			for _, sk := range c.Face.Skip {
				if info.Name() == sk {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), strings.ToLower(c.Face.Ext)) {
			paths = append(paths, p)
		}
		return nil
	})
	if walkErr != nil {
		return idx, walkErr
	}
	sort.Strings(paths)

	// ② 一遍读 + 分档取词。
	rows := [][4]string{} // 档 · 词 · 篇 · 行
	for _, p := range paths {
		rel, rerr := filepath.Rel(docRoot, p)
		if rerr != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			continue
		}
		idx.Files++
		text := strings.TrimRight(string(b), "\n")
		lines := []string{}
		if text != "" {
			lines = strings.Split(text, "\n")
		}
		idx.Lines += len(lines)
		if len(lines) > 0 {
			blk := c.BlockLine
			if blk <= 0 {
				blk = 1
			}
			idx.Blocks += (len(lines) + blk - 1) / blk
		}
		for i, ln := range lines {
			no := i + 1
			refLine := impactDocRefRe.MatchString(ln)
			bWords := []string{}
			for _, m := range impactDocTickRe.FindAllStringSubmatch(ln, -1) {
				bWords = append(bWords, impactDocWordRe.FindAllString(m[1], -1)...)
			}
			aWords := impactDocWordRe.FindAllString(ln, -1)
			add := func(tier, w string) {
				if stop[w] {
					idx.StopCount[tier]++
					return
				}
				if idx.Tier[tier][w] == nil {
					idx.Tier[tier][w] = map[string]int{}
				}
				if _, ok := idx.Tier[tier][w][rel]; !ok {
					idx.Tier[tier][w][rel] = no
					rows = append(rows, [4]string{tier, w, rel, fmt.Sprintf("%d", no)})
				}
			}
			for _, w := range aWords {
				add("A", w)
			}
			for _, w := range bWords {
				add("B", w)
				if refLine {
					add("C", w)
				}
			}
		}
	}
	// ③ 落盘（先临时件再改名 ⇒ 半个索引永远不会被读到；**没有删除动作**）。
	sort.Slice(rows, func(i, j int) bool {
		for k := 0; k < 3; k++ {
			if rows[i][k] != rows[j][k] {
				return rows[i][k] < rows[j][k]
			}
		}
		return rows[i][3] < rows[j][3]
	})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return idx, err
	}
	tsv, manifest := impactDocIndexNames(dir, idx.Head, idx.Caliber)
	idx.TSVPath, idx.Manifest = tsv, manifest
	var sb strings.Builder
	words := map[string]bool{}
	for _, r := range rows {
		sb.WriteString(strings.Join([]string{r[0], r[1], r[2], r[3]}, "\t"))
		sb.WriteString("\n")
		idx.TierRows[r[0]]++
		words[r[1]] = true
	}
	idx.Pairs = len(rows)
	idx.Words = len(words)
	for _, t := range impactDocTiers {
		idx.TierWords[t] = len(idx.Tier[t])
	}
	payload := []byte(sb.String())
	tmp := tsv + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return idx, err
	}
	if err := os.Rename(tmp, tsv); err != nil {
		return idx, err
	}
	idx.Bytes = int64(len(payload))
	idx.BlockLine = c.BlockLine
	idx.BuiltMS = time.Since(start).Milliseconds()
	idx.BuiltAt = impactEffectiveAt()
	man := map[string]any{
		"schema": impactDocIndexSchema, "head_sha": idx.Head, "root": idx.Root,
		"口径": idx.Caliber, "面内定义": c.Face.Definition, "每块行数": c.BlockLine,
		"件": idx.Files, "行": idx.Lines, "块": idx.Blocks, "词": idx.Words, "对": idx.Pairs,
		"字节": idx.Bytes, "耗时毫秒": idx.BuiltMS, "时刻": idx.BuiltAt,
		"三档": idx.TierRows, "三档词数": idx.TierWords, "排除计数": idx.StopCount,
		"不许接的动作": c.NoAct2, "tsv": filepath.Base(tsv),
	}
	mb, _ := json.MarshalIndent(man, "", "  ")
	mtmp := manifest + ".tmp"
	if err := os.WriteFile(mtmp, append(mb, '\n'), 0o644); err != nil {
		return idx, err
	}
	if err := os.Rename(mtmp, manifest); err != nil {
		return idx, err
	}
	return idx, nil
}

// impactDocIndexLoad 读一份倒排（**只读**）：清单件校验形状号 + `head_sha` 逐字同；TSV 读回内存。
func impactDocIndexLoad(tsv, manifest, head string) (impactDocIndex, error) {
	idx := impactDocIndex{OK: false, TSVPath: tsv, Manifest: manifest,
		Tier: map[string]map[string]map[string]int{}, TierRows: map[string]int{}, TierWords: map[string]int{}, StopCount: map[string]int{}}
	for _, t := range impactDocTiers {
		idx.Tier[t] = map[string]map[string]int{}
	}
	mb, err := os.ReadFile(manifest)
	if err != nil {
		idx.Why = "清单件读不到（" + err.Error() + "）"
		return idx, err
	}
	var man struct {
		Schema    string         `json:"schema"`
		Head      string         `json:"head_sha"`
		Root      string         `json:"root"`
		Caliber   string         `json:"口径"`
		Files     int            `json:"件"`
		Lines     int            `json:"行"`
		Blocks    int            `json:"块"`
		Words     int            `json:"词"`
		Pairs     int            `json:"对"`
		Bytes     int64          `json:"字节"`
		BuiltMS   int64          `json:"耗时毫秒"`
		BuiltAt   string         `json:"时刻"`
		BlockLine int            `json:"每块行数"`
		TierRows  map[string]int `json:"三档"`
		StopCount map[string]int `json:"排除计数"`
	}
	if err := json.Unmarshal(mb, &man); err != nil {
		idx.Why = "清单件解不开（" + err.Error() + "）"
		return idx, err
	}
	if man.Schema != impactDocIndexSchema {
		idx.Why = fmt.Sprintf("清单件形状号不认（%q ≠ %q）", man.Schema, impactDocIndexSchema)
		return idx, fmt.Errorf("schema mismatch")
	}
	// **键的第二件**：文档仓 `head_sha` 必须逐字同（变了 ⇒ 旧件不许命中 —— 与 `A5` 同一条命门）。
	if strings.TrimSpace(head) == "" || man.Head != head {
		idx.Why = fmt.Sprintf("文档仓 `head_sha` 不匹配（现读 %q ≠ 落盘 %q）⇒ 旧件不许命中（重扫）",
			dashIfEmpty(head), dashIfEmpty(man.Head))
		return idx, fmt.Errorf("head mismatch")
	}
	f, err := os.Open(tsv)
	if err != nil {
		idx.Why = "倒排件读不到（" + err.Error() + "）"
		return idx, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	n := 0
	for sc.Scan() {
		fs := strings.Split(sc.Text(), "\t")
		if len(fs) != 4 {
			continue
		}
		tier, w, doc := fs[0], fs[1], fs[2]
		if _, ok := idx.Tier[tier]; !ok {
			idx.Tier[tier] = map[string]map[string]int{}
		}
		if idx.Tier[tier][w] == nil {
			idx.Tier[tier][w] = map[string]int{}
		}
		ln := 0
		fmt.Sscanf(fs[3], "%d", &ln)
		idx.Tier[tier][w][doc] = ln
		n++
	}
	idx.OK = true
	idx.Head, idx.Root, idx.Caliber = man.Head, man.Root, man.Caliber
	idx.Files, idx.Lines, idx.Blocks = man.Files, man.Lines, man.Blocks
	idx.Words, idx.Pairs, idx.Bytes = man.Words, man.Pairs, man.Bytes
	idx.BuiltMS, idx.BuiltAt, idx.BlockLine = man.BuiltMS, man.BuiltAt, man.BlockLine
	idx.TierRows, idx.StopCount = man.TierRows, man.StopCount
	for _, t := range impactDocTiers {
		idx.TierWords[t] = len(idx.Tier[t])
	}
	return idx, nil
}

// impactDocFaceView 文档倒排面（`C3`）这一跑的读数与条目。
type impactDocFaceView struct {
	Status string // 取值 / 未建索引 / 读不到
	Detail string
	Rows   []map[string]string
	Docs   int    // **N**：命中的**篇数**（不是行数）
	Tier   string // 本跑用的口径档号
	Built  string // 本跑现建了的话，人读读数（否则空串）
}

// impactDocFaceLookup 查一次文档倒排面（`C3` 的全部取数都在这里）：
//
//	① 契约件不认 / 落点目录名取不到 / 文档面根读不到 ⇒ **不给结论**（逐条写原因）；
//	② 索引不在 或 与文档仓 `head_sha` 不匹配 ⇒ 默认档写「未建索引（按需档会现建）」；
//	   按需档（`build=true`）⇒ **现建一次**（一次扫全量 md）并把体量读数带出来；
//	③ 命中按**档**取（A `--wide` / B 默认 / C `--strict`）；**N = 0 ⇒ 写「面内未见」+ 面内定义**。
func impactDocFaceLookup(root string, tgt *impactTarget, layout impactStateLayout, layoutOK bool,
	build bool, tier string) impactDocFaceView {
	v := impactDocFaceView{Status: "未建索引", Tier: tier}
	c, why := impactDocIndexContractOf(root)
	if why != "" {
		v.Status = "读不到"
		v.Detail = "文档面契约件不给结论（" + why + "）"
		return v
	}
	if !layoutOK {
		v.Detail = "索引落点目录名取不到（落点契约件 `" + impactStateContractRel +
			"` 读不到 ⇒ 不许猜目录名）⇒ **未建索引 ⇒ 不取数**"
		return v
	}
	dir := impactCacheIndexDir(layout)
	docRoot := impactDocFaceRoot(root, c)
	if st, err := os.Stat(docRoot); err != nil || !st.IsDir() {
		v.Status = "读不到"
		v.Detail = fmt.Sprintf("文档面根读不到（%s）⇒ 不给结论 · 面内定义：%s", docRoot, c.Face.Definition)
		return v
	}
	head := impactDocHeadSHA(docRoot)
	cal := impactDocIndexCaliber(c)
	tsv, man := impactDocIndexNames(dir, head, cal)
	idx, err := impactDocIndexLoad(tsv, man, head)
	if err != nil {
		if !build {
			v.Detail = fmt.Sprintf("索引不在 / 与文档仓 `head_sha` 不匹配（%s）⇒ **未建索引 ⇒ 不取数**"+
				"（按需档 `--all` 会**现建一次**：一次扫全量 `%s`）· 面内定义：%s",
				err.Error(), c.Face.Ext, c.Face.Definition)
			return v
		}
		idx, err = impactDocIndexBuild(docRoot, dir, c)
		if err != nil {
			v.Status = "读不到"
			v.Detail = "现建索引失败（" + err.Error() + "）⇒ 不给结论"
			return v
		}
		v.Built = fmt.Sprintf("本跑**现建**：件 %d · 行 %d · 块 %d · 词 %d · 对 %d · **耗时 %d ms** · %s",
			idx.Files, idx.Lines, idx.Blocks, idx.Words, idx.Pairs, idx.BuiltMS, idx.BuiltAt)
	}
	words := impactDocQueryWords(root, tgt)
	hits, n := impactDocIndexQuery(idx, words, tier)
	rows := []map[string]string{}
	for _, h := range hits {
		rows = append(rows, impactRow("件级:"+h.Doc+":"+strconv.Itoa(h.Line), "词法",
			fmt.Sprintf("文档面倒排命中（口径 %s 档 · 词 %s）—— 「提及」≠「在用」（`R24`：只报不拦）", tier, h.Word), ""))
	}
	v.Rows, v.Docs, v.Status = rows, n, "取值"
	v.Detail = fmt.Sprintf("文档倒排面（`C3`）：口径 **%s 档**（%s）· 面 = 全量 `%s`"+
		"（面内定义：%s）· 文档仓 head_sha=%s · 索引 body = 件 %d / 行 %d / 块 %d / 词 %d / 对 %d（每块 %d 行 · 落点 %s）%s"+
		" · 查询词 %d 枚 · 命中 **%d 篇**",
		tier, c.Tiers[tier].Name, c.Face.Ext, c.Face.Definition, shortHex(head),
		idx.Files, idx.Lines, idx.Blocks, idx.Words, idx.Pairs, idx.BlockLine, dir,
		func() string {
			if v.Built != "" {
				return " · " + v.Built
			}
			return fmt.Sprintf(" · 落盘件 built_at=%s（耗时 %d ms）", idx.BuiltAt, idx.BuiltMS)
		}(), len(words), n)
	if n == 0 {
		v.Detail += " ⇒ **面内未见**（`N = 0` 说的是**这一片里没人提**，**不是**「无文档受影响」✗）：" +
			c.Face.Definition
	}
	v.Detail += " · 通用词假阳已按契约件排除：" + strings.Join(c.Stop, "/") + "（四个词在 A/B/C 三档上的命中计数 = " +
		fmt.Sprintf("%d/%d/%d", idx.StopCount["A"], idx.StopCount["B"], idx.StopCount["C"]) + "）"
	return v
}

// impactDocQueryWords 目标件（或契约 id）在文档面要找的词（口径写死 · 一件一组）：
// 件名 · 仓内相对路径 · 顶层符号名（上限 40 · 字典序 ⇒ 两跑逐字相同）。
func impactDocQueryWords(root string, tgt *impactTarget) []string {
	set := map[string]bool{}
	if tgt == nil {
		return nil
	}
	if tgt.Kind == impactKindContract {
		set[tgt.ID] = true
		return []string{tgt.ID}
	}
	rel := tgt.Rel
	set[rel] = true
	set[filepath.Base(filepath.FromSlash(rel))] = true
	syms := impactGoFileSymbols(filepath.Join(root, filepath.FromSlash(rel)))
	names := []string{}
	for n := range syms {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) > 40 {
		names = names[:40]
	}
	for _, n := range names {
		set[n] = true
	}
	out := []string{}
	for w := range set {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// impactDocIndexQuery 查一次（**只读内存里的倒排**）：返回命中的 (词, 篇, 行) 与「面内未见」口径串。
func impactDocIndexQuery(idx impactDocIndex, words []string, tier string) ([]impactDocHit, int) {
	if !idx.OK {
		return nil, 0
	}
	hits := []impactDocHit{}
	docs := map[string]bool{}
	t := idx.Tier[tier]
	if t == nil {
		return hits, 0
	}
	for _, w := range words {
		for doc, ln := range t[w] {
			hits = append(hits, impactDocHit{Word: w, Doc: doc, Line: ln})
			docs[doc] = true
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Doc != hits[j].Doc {
			return hits[i].Doc < hits[j].Doc
		}
		if hits[i].Line != hits[j].Line {
			return hits[i].Line < hits[j].Line
		}
		return hits[i].Word < hits[j].Word
	})
	return hits, len(docs)
}
