// gen_glossary_exports.go —— 生成器件 `scripts/docs/gen-glossary-exports.py`（299 行）的 **Go 真搬**
// （v2.5.14 · M2 · 批-4 生成器搬运 · 入口 `genGlossaryExportsGateMain`）。
//
// 验收判据 = 同输入（同一棵仓树 + 同一主表 mtime）下，py 原件 ⟷ 本入口 **产出五件逐字节相同**
// （同输入比产出 sha256）＋ 逐格 rc/stdout/stderr 三面相同。
//
// 本件形态：生成器 · 纯静态只读主表 · 写仓内五件导出。外部依赖仅 Python 标准库
// （os/re/sys/datetime）⇒ 无联网 · 无模板引擎 · 无第三方 py 包 ⇒ 适格。
//
// 逐字节对齐要点（照 py 逐条）：
//
//	① 主表读取走 **Python 文本模式 universal newlines**：`\r\n` 与裸 `\r` 折成 `\n`
//	   ⇒ Go 侧显式折（✗ 不直接按行切）。
//	② `str.strip()` 剥的是 **Python 的 Unicode 空白**（含 `\x1c-\x1f` / `\x85` / `\xa0`）
//	   ⇒ 自写 `ggexPyStrip`（✗ 不用 `strings.TrimSpace`）。
//	③ `line[:60]` 按**码点**切 ⇒ 先 `[]rune` 再切（✗ 按字节切）。
//	④ os.path.join/abspath/dirname 语义 ⇒ 自写 `ggexPyJoin` / `ggexDirname`（✗ 不用 `filepath.Join`）。
//	⑤ 导出五件的迭代序 = py dict 插入序 ⇒ Go 用有序切片（✗ 零 map 迭代）。
//	⑥ `updated_at` = **主表 mtime 的本地日期**（不是跑生成器的当天）⇒ Go `os.Stat` + `Local()`。
//
// 两条标准声明行：
//
//	(a) **默认根定位差异**：py `ROOT = abspath(join(dirname(abspath(__file__)), "..", ".."))`
//	    ⇒ 起于**脚本所在目录**；本件无 `__file__` 语境 ⇒ 从 **cwd** 上溯 ≤12 层找含
//	    `core/cmd/zerg/main.go` 的目录、找不到回落 `abspath(cwd)`。「脚本位置 == cwd 上溯根」
//	    时两者读数逐字相同；否则**路径推导面不同** ⇒ 不属逐字对齐承诺。
//	(b) **自指检查**：本门扫描面 = 主表 `docs/术语表.tsv` 与产出五件（均为数据件，✗ 不扫 `.go`）
//	    ⇒ 本件（`.go` · 落 `core/cmd/zerg/`）**不在自身扫描面内**。量化依据：全件零形如
//	    「JSON 键名连续引号原串」的写法（一律描述式）· 退码仅 0/1/2（⊆ 真源退码表已登记集）·
//	    零绝对私有路径字面量。
//
// 本单边界（照实）：**已接线（inproc 表分派）** ——
//
//	· 不在 `core/cmd/zerg/main.go` 注册命令（✗ 不进命令树）；
//	· 不删 py 原件（退场规则：对拍通过后才逐件点名退场）；
//	· **已接线**：入口 `genGlossaryExportsGateMain` 由 `gateCheckGoEntry["gen-glossary-exports"]` 分派（`zerg gate check gen-glossary-exports`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ── 常量（照 py 逐字）────────────────────────────────────────────────

var ggexCols = []string{"zh", "en", "id", "do_not_translate", "forbid_zh", "forbid_en", "owner", "note"}

const ggexColsRepr = "['zh', 'en', 'id', 'do_not_translate', 'forbid_zh', 'forbid_en', 'owner', 'note']"

const ggexDefaultFreq = 10000
const ggexDefaultTag = "nz"

var (
	ggexREOKASCII = regexp.MustCompile("^[A-Za-z0-9_.\\-/]+$")
	ggexREOKCJK   = regexp.MustCompile("^[\\x{3400}-\\x{9fff}\\x{3000}-\\x{303f}]+$")
)

// ggexDoc —— py 模块 __doc__（`--help`/`-h` 逐字印它 ＋ 一个尾换行）。由 py 原件现读抽取。
var ggexDoc = "gen-glossary-exports.py —— 从术语表主表生成四处导出（唯一真源 = docs/术语表.tsv）\n\n设计真源：Zerg-内部文档/01-设计/设计-文档体系-v1.0.md §5.6③（一份主表 + 四处导出 + 导出规则六条）\n依据：Zerg-内部文档/调研/调研-文档分块落地-检索与双语.md §2.2\n\n四处导出（本脚本一次生成，全部**派生物、不得手改**）：\n  ① docs/术语表.md                              人读对照（生成、只读）\n  ② docs/术语表.jieba.txt                       jieba 用户词典（每行 `词语 词频 词性`）\n  ③ 同上文件 + Material `plugins.search.jieba_dict_user`  ← 站内搜索词典复用同一份（不另出文件）\n  ④ styles/config/vocabularies/Zerg/accept.txt    Vale vocab（accept，每行一条正则）\n     styles/config/vocabularies/Zerg/reject.txt    Vale vocab（reject，每行一条正则）\n     docs/术语表.forbid.txt                      门禁词表（`词<TAB>理由`，喂 check-zh-en.py 的 FORBID）\n\n导出规则六条（逐条落在此脚本里）：\n  ① 主表**保序**（导出顺序 = 表内顺序，便于 diff）\n  ② 去重 + **禁空行**\n  ③ 导出物**不得手改**（jieba / Vale 都不吃注释 ⇒ 用独立 README + 门禁代替抬头）\n  ④ 生成后跑 `--check`（等价 `git diff --exit-code`）：与盘上不一致 ⇒ rc=1\n  ⑤ **条数断言**（生成的行数与主表行数逐条对齐，缺一条即 rc≠0）\n  ⑥ `en` 列为空 ⇒ 该词条**不得进译稿**（本脚本把它单列出来并计数；拦截在 check-glossary.py）\n\n用法：\n  python3 scripts/docs/gen-glossary-exports.py            # 生成/更新四处导出\n  python3 scripts/docs/gen-glossary-exports.py --check    # 只比对，不写盘（门禁用；有漂移 rc=1）\n  python3 scripts/docs/gen-glossary-exports.py --selftest # 自证解析与生成规则（不读仓）\n\n退出码：0 = 一致/已生成；1 = 漂移（--check）；2 = 环境/输入异常（不静默）\n"

// ── posixpath 语义（照 py os.path.*，✗ 不用 filepath.Join）─────────────────

// ggexPyJoin —— posixpath.join：空段忽略 · 后段绝对路径丢弃前段 · `..` 原样留串。
func ggexPyJoin(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "/") {
			b.Reset()
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "/") {
			b.WriteString("/")
		}
		b.WriteString(p)
	}
	return b.String()
}

// ggexDirname —— posixpath.split 头部语义（剥尾斜杠）。
func ggexDirname(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	head := p[:i+1]
	if strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head
}

// ── Python 字符串原语（逐处自写，✗ 不用标准库近似）─────────────────────

// ggexPyText —— Python 文本模式读取：universal newlines（`\r\n` / `\r` ⇒ `\n`）。
func ggexPyText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// ggexRstripCR —— py str.rstrip("\r")。
func ggexRstripCR(s string) string { return strings.TrimRight(s, "\r") }

// ggexPyStrip —— py str.strip()：剥 Python 的 Unicode 空白。
func ggexPyStrip(s string) string { return strings.TrimFunc(s, ggexPyIsSpace) }

func ggexPyIsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0,
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// ggexRuneHead —— py 切片 `s[:n]`（按码点，✗ 不按字节）。
func ggexRuneHead(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

func ggexStrEq(a, b []string) bool {
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

func ggexContainsStr(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func ggexHas(argv []string, s string) bool {
	for _, a := range argv {
		if a == s {
			return true
		}
	}
	return false
}

// ggexPyStrRepr —— py repr(str) 的常规子集（用于异常文案里回印报头字段）。
func ggexPyStrRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			if r == rune(quote) {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

func ggexPyListRepr(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = ggexPyStrRepr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func ggexIsFile(p string) bool {
	st, err := os.Stat(p) // py os.path.isfile 跟随符号链接 ⇒ os.Stat（✗ 非 Lstat/IsDir）
	return err == nil && !st.IsDir()
}

func ggexIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func ggexJoinLinesLF(lines []string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return b.String()
}

// ── 主表读取（py read_main）───────────────────────────────────────────

func ggexReadMain(path string) ([]map[string]string, error) {
	if !ggexIsFile(path) {
		return nil, fmt.Errorf("主表不存在：%s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("主表不存在：%s", path)
	}
	raw := strings.Split(ggexPyText(string(b)), "\n")
	var rows []map[string]string
	for i, line := range raw {
		idx := i + 1
		if idx == 1 {
			header := strings.Split(ggexRstripCR(line), "\t")
			if !ggexStrEq(header, ggexCols) {
				return nil, fmt.Errorf("表头不符：期望 %s，实际 %s", ggexColsRepr, ggexPyListRepr(header))
			}
			continue
		}
		if ggexPyStrip(line) == "" {
			continue // 规则②：禁空行（末尾换行不算行）
		}
		f := strings.Split(ggexRstripCR(line), "\t")
		if len(f) != len(ggexCols) {
			return nil, fmt.Errorf("第 %d 行字段数 = %d（应为 %d）：%s", idx, len(f), len(ggexCols), ggexRuneHead(line, 60))
		}
		if ggexPyStrip(f[0]) == "" {
			return nil, fmt.Errorf("第 %d 行 zh 为空", idx)
		}
		row := map[string]string{}
		for j, c := range ggexCols {
			row[c] = f[j]
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("主表零数据行——拒绝静默空转（假绿）")
	}
	seen := map[string]bool{}
	var uniq []map[string]string
	for _, r := range rows {
		if seen[r["zh"]] {
			continue
		}
		seen[r["zh"]] = true
		uniq = append(uniq, r)
	}
	return uniq, nil
}

// ── 生成（py split_multi / gen_jieba / gen_forbid / gen_vale / gen_md）───

func ggexSplitMulti(v string) []string {
	var out []string
	for _, x := range strings.Split(v, "|") {
		x = ggexPyStrip(x)
		if x != "" && !ggexContainsStr(out, x) {
			out = append(out, x)
		}
	}
	return out
}

func ggexGenJieba(rows []map[string]string, freq int) []string {
	var lines []string
	for _, r := range rows {
		w := ggexPyStrip(r["zh"])
		if w == "" || strings.Contains(w, " ") || strings.Contains(w, "\t") {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s %d %s", w, freq, ggexDefaultTag))
	}
	return lines
}

func ggexGenForbid(rows []map[string]string) []string {
	var lines []string
	for _, r := range rows {
		for _, w := range ggexSplitMulti(r["forbid_zh"]) {
			lines = append(lines, fmt.Sprintf("%s\t术语统一：只有「%s」（docs/术语表.tsv）", w, r["zh"]))
		}
		for _, w := range ggexSplitMulti(r["forbid_en"]) {
			lines = append(lines, fmt.Sprintf("%s\t术语统一：只有「%s」（docs/术语表.tsv）", w, r["zh"]))
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, l := range lines {
		key := strings.Split(l, "\t")[0]
		if !seen[key] {
			seen[key] = true
			out = append(out, l)
		}
	}
	return out
}

func ggexGenVale(rows []map[string]string) ([]string, []string) {
	var accept, reject []string
	push := func(bag *[]string, v string) {
		v = ggexPyStrip(v)
		if v == "" || ggexContainsStr(*bag, v) {
			return
		}
		if ggexREOKASCII.MatchString(v) || ggexREOKCJK.MatchString(v) {
			*bag = append(*bag, v)
		}
	}
	for _, r := range rows {
		push(&accept, r["zh"])
		push(&accept, r["en"])
		push(&accept, r["do_not_translate"])
		push(&accept, r["id"])
		for _, w := range ggexSplitMulti(r["forbid_zh"]) {
			push(&reject, w)
		}
		for _, w := range ggexSplitMulti(r["forbid_en"]) {
			push(&reject, w)
		}
	}
	return accept, reject
}

func ggexGenMD(rows, emptyEn []map[string]string, nJieba, nForbid, nAccept, nReject int, today string) string {
	var L []string
	L = append(L, "---")
	L = append(L, "title: 术语表（生成物 · 只读）")
	L = append(L, "type: reference")
	L = append(L, "status: draft")
	L = append(L, "source_of_truth: docs/术语表.tsv")
	L = append(L, "owner: Mr2109")
	L = append(L, "updated_at: "+today)
	L = append(L, "ai_assisted: ai")
	L = append(L, "summary: 由 docs/术语表.tsv 生成的人读对照表；改字请改主表后重跑生成器。")
	L = append(L, "---")
	L = append(L, "")
	L = append(L, "<!-- 本文件由 core/cmd/zerg/gen_glossary_exports.go 生成 —— 请勿手改 ✗")
	L = append(L, "     唯一真源 = docs/术语表.tsv（改主表后重跑生成器）；门禁：生成后 --check 必须 rc=0 -->")
	L = append(L, "")
	L = append(L, "# 术语表（生成物 · 只读）")
	L = append(L, "")
	L = append(L, "> 唯一真源：`docs/术语表.tsv`（人读主表 + 四处导出，见 `docs/术语表-说明.md`）。")
	L = append(L, "> 本页是**派生物**：改字请改主表，然后重跑 `zerg gate check gen-glossary-exports`。")
	L = append(L, "")
	L = append(L, fmt.Sprintf("条数：主表 %d 条 · jieba 词表 %d 行 · 门禁词表 %d 条 · Vale accept %d 条 / reject %d 条",
		len(rows), nJieba, nForbid, nAccept, nReject))
	L = append(L, "")
	L = append(L, "| zh | en | id | do_not_translate | forbid_zh | forbid_en | owner | note |")
	L = append(L, "|---|---|---|---|---|---|---|---|")
	for _, r := range rows {
		cells := make([]string, len(ggexCols))
		for i, c := range ggexCols {
			cells[i] = r[c]
		}
		L = append(L, "| "+strings.Join(cells, " | ")+" |")
	}
	L = append(L, "")
	L = append(L, fmt.Sprintf("## `en` 列为空的词条（**不得进译稿**，共 %d 条）", len(emptyEn)))
	L = append(L, "")
	for _, r := range emptyEn {
		id := r["id"]
		if id == "" {
			id = "—"
		}
		L = append(L, fmt.Sprintf("- **%s**（id: %s）—— %s", r["zh"], id, r["note"]))
	}
	L = append(L, "")
	return strings.Join(L, "\n") + "\n"
}

// ggexToday —— py today()：主表 mtime 的**本地日期**。
func ggexToday(main string) string {
	st, err := os.Stat(main)
	if err != nil {
		return "1970-01-01"
	}
	return time.Unix(st.ModTime().Unix(), 0).Local().Format("2006-01-02")
}

// ── build / run ──────────────────────────────────────────────────────

type ggexExport struct {
	path string
	text string
}

func ggexBuild(root string) ([]ggexExport, map[string]int, error) {
	main := ggexPyJoin(root, "docs", "术语表.tsv")
	rows, err := ggexReadMain(main)
	if err != nil {
		return nil, nil, err
	}
	var emptyEn []map[string]string
	for _, r := range rows {
		if ggexPyStrip(r["en"]) == "" {
			emptyEn = append(emptyEn, r)
		}
	}
	jieba := ggexGenJieba(rows, ggexDefaultFreq)
	forbid := ggexGenForbid(rows)
	accept, reject := ggexGenVale(rows)
	md := ggexGenMD(rows, emptyEn, len(jieba), len(forbid), len(accept), len(reject), ggexToday(main))
	outs := []ggexExport{
		{ggexPyJoin(root, "docs", "术语表.md"), md},
		{ggexPyJoin(root, "docs", "术语表.jieba.txt"), ggexJoinLinesLF(jieba)},
		{ggexPyJoin(root, "docs", "术语表.forbid.txt"), ggexJoinLinesLF(forbid)},
		{ggexPyJoin(root, "styles", "config", "vocabularies", "Zerg", "accept.txt"), ggexJoinLinesLF(accept)},
		{ggexPyJoin(root, "styles", "config", "vocabularies", "Zerg", "reject.txt"), ggexJoinLinesLF(reject)},
	}
	// 规则⑤：条数断言
	if len(jieba) > len(rows) {
		return nil, nil, fmt.Errorf("jieba 行数 > 主表条数")
	}
	// 规则②：禁空行（jieba/forbid/accept/reject 四份机器词表 —— 人读 .md 允许空行）
	names := []string{"jieba", "forbid", "accept", "reject"}
	for k, idx := range []int{1, 2, 3, 4} {
		parts := strings.Split(outs[idx].text, "\n")
		if len(parts) > 0 {
			parts = parts[:len(parts)-1]
		}
		for i, l := range parts {
			if ggexPyStrip(l) == "" {
				return nil, nil, fmt.Errorf("导出物 %s 第 %d 行为空（规则②）", names[k], i+1)
			}
		}
	}
	stats := map[string]int{
		"rows": len(rows), "empty_en": len(emptyEn), "jieba": len(jieba),
		"forbid": len(forbid), "accept": len(accept), "reject": len(reject),
	}
	if !(stats["rows"] > 0 && stats["jieba"] > 0) {
		return nil, nil, fmt.Errorf("条数断言失败：主表或词表为空")
	}
	return outs, stats, nil
}

// ggexRepoRoot —— 声明行 (a)：无 `__file__` 语境 ⇒ cwd 上溯找 `core/cmd/zerg/main.go`。
func ggexRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	cur := wd
	for i := 0; i < 12; i++ {
		if ggexIsFile(filepath.Join(cur, "core", "cmd", "zerg", "main.go")) {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	abs, err := filepath.Abs(wd)
	if err != nil {
		return wd
	}
	return abs
}

// ggexReadFileNL —— py `open(p).read()` 语义（universal newlines）。
func ggexReadFileNL(p string) (string, bool) {
	if !ggexIsFile(p) {
		return "", false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return ggexPyText(string(b)), true
}

func ggexRun(stdout, stderr io.Writer, argv []string) int {
	known := map[string]bool{"--check": true, "--selftest": true, "--help": true, "-h": true}
	var bad []string
	for _, a := range argv {
		if strings.HasPrefix(a, "-") && !known[a] {
			bad = append(bad, a)
		}
	}
	if len(bad) > 0 {
		fmt.Fprintf(stderr, "✗ 未知参数: %s\n", strings.Join(bad, " "))
		return 2
	}
	if ggexHas(argv, "--help") || ggexHas(argv, "-h") {
		fmt.Fprint(stdout, ggexDoc)
		fmt.Fprint(stdout, "\n")
		return 0
	}
	if ggexHas(argv, "--selftest") {
		return ggexSelftest(stdout)
	}
	root := ggexRepoRoot()
	outs, st, err := ggexBuild(root)
	if err != nil {
		fmt.Fprintf(stderr, "✗ 术语表导出失败：%s\n", err.Error())
		return 2
	}
	check := ggexHas(argv, "--check")
	var drift []string
	for _, o := range outs {
		old, ok := ggexReadFileNL(o.path)
		if !ok {
			drift = append(drift, fmt.Sprintf("%s（缺文件）", o.path))
		} else if old != o.text {
			drift = append(drift, fmt.Sprintf("%s（内容不一致）", o.path))
		}
	}
	fmt.Fprintf(stdout, "主表 %d 条（en 空 %d 条 = 不得进译稿）· jieba %d 行 · 门禁词表 %d 条 · Vale accept %d / reject %d\n",
		st["rows"], st["empty_en"], st["jieba"], st["forbid"], st["accept"], st["reject"])
	if check {
		if len(drift) > 0 {
			fmt.Fprint(stderr, "✗ 导出物与主表不一致（改主表后须重跑生成器）：\n")
			for _, d := range drift {
				fmt.Fprintf(stderr, "   %s\n", d)
			}
			return 1
		}
		fmt.Fprint(stdout, "✓ 四处导出与主表一致（等价 git diff --exit-code）\n")
		return 0
	}
	if len(drift) > 0 {
		for _, d := range drift {
			fmt.Fprintf(stdout, "   写 %s\n", strings.SplitN(d, "（", 2)[0])
		}
	}
	for _, o := range outs {
		d := ggexDirname(o.path)
		if d != "" && !ggexIsDir(d) {
			os.MkdirAll(d, 0o777)
		}
		os.WriteFile(o.path, []byte(o.text), 0o644)
	}
	fmt.Fprint(stdout, "✓ 已写 5 个文件（术语表.md / .jieba.txt / .forbid.txt / Zerg/accept.txt / Zerg/reject.txt）\n")
	fmt.Fprint(stdout, "  提示：生成物不得手改；手改即被本脚本 --check 抓出（规则③④）\n")
	return 0
}

// ggexSelftest —— py selftest()：10 条纯函数自证（不读仓）。
func ggexSelftest(stdout io.Writer) int {
	fmt.Fprint(stdout, "── gen-glossary-exports.py 自证\n")
	ok := true
	mk := func(zh, en, id, dnt, fz, fe, ow, no string) map[string]string {
		return map[string]string{
			"zh": zh, "en": en, "id": id, "do_not_translate": dnt,
			"forbid_zh": fz, "forbid_en": fe, "owner": ow, "note": no,
		}
	}
	type ggexCase struct {
		name string
		good bool
	}
	cases := []ggexCase{
		{"split_multi 保序去重", ggexStrEq(ggexSplitMulti("a|b|a"), []string{"a", "b"})},
		{"split_multi 去空白", ggexStrEq(ggexSplitMulti(" a | b "), []string{"a", "b"})},
		{"split_multi 空串", ggexStrEq(ggexSplitMulti(""), []string{})},
		{"Vale 收 ASCII 词", ggexREOKASCII.MatchString("jieba_dict_user")},
		{"Vale 收 CJK 词", ggexREOKCJK.MatchString("虫茧")},
		{"Vale 拒正则元字符", !ggexREOKASCII.MatchString("{: #slug }")},
		{"jieba 行 = 词语 词频 词性",
			strings.Count(ggexGenJieba([]map[string]string{mk("虫茧", "cocoon", "cocoon", "cocoon", "", "", "Mr2109", "")}, ggexDefaultFreq)[0], " ") == 2},
		{"jieba 跳空 zh",
			len(ggexGenJieba([]map[string]string{mk("", "x", "", "", "", "", "", "")}, ggexDefaultFreq)) == 0},
		{"门禁词表用 TAB 分列",
			strings.Split(ggexGenForbid([]map[string]string{mk("虫茧", "cocoon", "", "", "虫巢", "pod", "Mr2109", "")})[1], "\t")[0] == "pod"},
		{"门禁词表带正名理由",
			strings.Contains(ggexGenForbid([]map[string]string{mk("虫茧", "cocoon", "", "", "虫巢", "", "Mr2109", "")})[0], "只有「虫茧」")},
	}
	for _, cs := range cases {
		ok = ok && cs.good
		mark := "✓"
		if !cs.good {
			mark = "✗"
		}
		fmt.Fprintf(stdout, "  %s %s\n", mark, cs.name)
	}
	if ok {
		fmt.Fprint(stdout, "  自证：通过 ✓（10 条）\n")
		return 0
	}
	fmt.Fprint(stdout, "  自证：**失败** ✗（10 条）\n")
	return 1
}

// genGlossaryExportsGateMain —— 本件入口（inproc 口径：argv **不含程序名**）。
func genGlossaryExportsGateMain(stdout, stderr io.Writer, argv []string) int {
	return ggexRun(stdout, stderr, argv)
}
