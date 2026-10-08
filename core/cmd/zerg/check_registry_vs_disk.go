// check_registry_vs_disk.go —— 门禁件 `scripts/gates/check-registry-vs-disk.py` 的 **Go 真搬**（v2.5.14 · M2-0 第三枚）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据 · 4.1 · 读仓内件即可）的**第三枚**：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（4.1 M2-0 = 「无输入物料、纯静态检查的判据（读仓内件即可）」）：
//
//	✓ 选：只读仓内两件登记面（`publish/whitelist.txt` + `scripts/build/publish-public.sh` 的 `EXCLUDES=( … )`）
//	  与它们点名的仓内真物（**无外部树依赖**）· 输出**定序**（条目按文件序 · 集合处一律 `sorted`）·
//	  退码 `0/1/2` 三档齐 · 498 行（**行数适中**）· **无人正在改**（`git status` 现读不含它）· 不属已列重件。
//	✗ 不选 `check-service-declaration.py`（384 行）：现值面靠 `subprocess` 真跑 `ps` / `launchctl`
//	  （**输入物料 + 进程面**）⇒ 不属「纯静态检查」。
//	✗ 不选 `check-tty-output-contract.py`（338 行）：靠 `script -q` 造真 TTY（**外部进程 + PTY**）⇒ 同上。
//	✗ 不选 `check-wallclock-ratchet.py`（232 行）：带写通道（`--write` 改基线件 + `datetime.now()` 墙钟）⇒
//	  **有写面**且输出含时间戳（**非确定**）。
//	✗ 不选 `check-symbol-delete-ripple.py`（333 行）：默认落点读 `<状态目录>/r28-plan.txt`（`~/.zerg/state`，
//	  **外部树依赖**）且吃 stdin。
//	✗ 不选 `check-shell-unicode-vars.py`（128 行）：默认档**带写面**（就地 `$VAR`→`${VAR}`）⇒ 不属「**纯**静态」。
//	✗ 不选已搬的 5 枚（`check_error_kinds` / `check_gotoolchain` / `check_mutation_residue` /
//	  `check_contract_registry` / `check_wall_layer`）：**共享件**（本单 ✗ 禁改）。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改任何 `add_step`）；
//	· **不**删 `scripts/gates/check-registry-vs-disk.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· 入口 `registryVsDiskGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态：
//	  先证明「搬得逐字相同」，接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func registryVsDiskGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 判到了（报告档） / 1 阻断档命中或自检不过 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ★ 标准声明行（两条 · 逐条照实）
//
//	(a) **默认根定位差异**：py `REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))`
//	    ⇒ 默认根 = **脚本所在目录的仓**（与 cwd 无关）；Go 入口**无 `__file__` 语境**（未来接线后是 CLI 子命令，
//	    更没有脚本路径）⇒ 本件默认从**当前工作目录**上溯 ≤8 层找同时含 `publish/whitelist.txt` 与
//	    `scripts/build/publish-public.sh` 的目录，找不到回落 cwd。落在**同一真仓、cwd=仓根**时两者读数逐字相同
//	    （已对拍 P01/P02/P11/P18）；但「脚本位置 ≠ cwd」时**路径推导面不同** ⇒ 这一面**不属逐字对齐承诺**，
//	    对拍一律显式传 `--repo`。
//	(b) **自指检查**：本件**不在**它自己扫的面内 —— 本件扫面 = `publish/whitelist.txt` 与
//	    `scripts/build/publish-public.sh` 的**正文**（读句子 / 解析 `EXCLUDES`），加「登记条目点名的盘上真物」
//	    的存在性与内容身份。本件落 `core/cmd/zerg/check_registry_vs_disk.go`，**不被任何登记条目点名** ⇒
//	    **不进**①②③ 任何一格。唯一间接路径：若 `core/` 这一条登记**沦为「空指/悬空」**，才会去算它的
//	    内容身份（遍历 `core/` 树、**只取件名**、不读件正文）—— 真仓现读 `core/` 在盘 ⇒ 该支不成立。
//	    ⇒ 本件**不读任何件正文**做判据（只 `os.path.exists` / `getsize` / `sha256` / 目录**件名集**）
//	      ⇒ **无**「夹具/自检标记反引号原串直写正文」之患（参 `check_wall_layer.go` 血证：那一件扫**正文**，
//	      才要回避反引号）。此处照实写明，不虚设。
//
// 已对拍面（仓外夹具两态逐字 · 见 `pairwise.md`）：真仓现读（报告档 / 阻断档）· 干净夹具 · 四类命中
// （空指 / 悬空 / 文案 C1 / 文案 C2）· 空转（白名单 0 条 ⇒ 2）· 缺件（白名单不在 ⇒ 2）· 取不到输入
// （`--repo` 指向不存在 ⇒ 2）· `--json` 合法字段集（含 `detail` 全量）· `--json` 表外字段拒收 ⇒ 2 ·
// `--self-test` 七格。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 帮助 / usage 正文**（`-h/--help` · 未知旗标 · `--repo`/`--json` 缺值 · 未知位置参数 ·
//	   长选项缩写）—— 本件落 rc=0（`-h`）/2（错），但**正文自定**（不逐字仿 argparse 的 usage 排版）。
//	② **默认根定位**：见上 (a) —— 非「脚本位置 == cwd == 仓根」时**不属逐字对齐承诺**。
//	③ **异常路径**：登记件存在但**读不到**（权限）/ 不是 UTF-8（py `open(..., encoding="utf-8")` 抛
//	   `UnicodeDecodeError`）/ 是目录（py 抛 `IsADirectoryError`）—— py 一律**未捕获抛 traceback** ⇒ 进程退码 1；
//	   本件落「读不到」⇒ 该真源按**缺件**口径（problems）走 ⇒ 退 2 同、**正文与 py 的 traceback 不同**。
//	④ py `--self-test` 用 `subprocess`? —— **不用**（进程内直调 `audit()`）⇒ 这一面**可**逐字对齐（P14 已对拍）。
//	⑤ **大目录内容身份封顶**：py 在 `DIR_CAP=5000` 处**按遍历序**截断再排序；本件**先全部收集、排序后再截断**
//	   —— 只有「登记条目点名的目录 > 5000 件」时**两个签名值**可能不同（真仓现读**无此态**：无目录沦为
//	   空指/悬空 ⇒ 从不算目录身份；本件自检同样不触发）。
//
//	—— ①②③⑤ 四面均**不在** M2-0 的样板对拍承诺面内；接线前须逐面补齐或显式剔除。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// crsWhitelistRel / crsExcludesRel —— 两件登记面（仓内相对路径）。★ 切源 2026-10-08
// （`GAP-20260926-223` 账[3]）：排除面由旧 `scripts/build/publish-public.sh` 的 `EXCLUDES=( … )`
// 改指行式新表 `publish/excludes.txt`。
const (
	crsWhitelistRel = "publish/whitelist.txt"
	crsExcludesRel  = "publish/excludes.txt"
)

// crsClaimHead —— 说明块起始句（现读逐字；找不到 ⇒ rc=2）。
const crsClaimHead = "这些在脚本 EXCLUDES 中以模式表达"

// crsDirCap —— 目录内容身份的名字集封顶件数（逐字照 py `DIR_CAP`）。
const crsDirCap = 5000

// crsJSONFields —— 本门机器读面字段表（**唯一真源**：装包封与拒收同读这一处）。
var crsJSONFields = []string{
	"schema", "dangling", "uncovered", "claims", "not_judged",
	"whitelist_entries", "excludes_entries", "detail",
}

var (
	crsCommentedEntryRE = regexp.MustCompile(`^\s*#\s*([A-Za-z0-9_./-]+/?)\s*[←<]`)
	crsTokenRE          = regexp.MustCompile(`^[ -~]+$`)
	crsGlobShapeRE      = regexp.MustCompile(`^[A-Za-z0-9_.*/-]*[*?][A-Za-z0-9_.*/-]*$`)
	crsDirShapeRE       = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*/$`)
	crsExtShapeRE       = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*\.[A-Za-z0-9]{1,6}$`)
	crsParenRE          = regexp.MustCompile(`（[^）]*）`)
	crsTokenSplitRE     = regexp.MustCompile(`[\s、，]+`)
	crsHasAlnumRE       = regexp.MustCompile(`[A-Za-z0-9]`)
)

// ── 小数据结构（按 py dict 的键序给 JSON 用）────────────────────────────────

type crsIdentity struct {
	shape      string // "dir" | "file" | "absent" | "glob"
	items      int
	namesSha12 string
	partial    bool
	rule       string
	bytes      int
	sha12      string
	globNote   string
}

type crsCommented struct {
	line  int
	entry string
	text  string
}

type crsBlockLine struct {
	line int
	text string
}

type crsToken struct {
	line  int
	token string
}

type crsDangling struct {
	registry string
	entry    string
	why      string
	ident    *crsIdentity
}

type crsUncovered struct {
	entry string
	why   string
	ident *crsIdentity
}

type crsClaim struct {
	rule       string
	line       int
	token      string
	why        string
	nearest    []string
	hasNearest bool
	ident      *crsIdentity
}

type crsCounts struct {
	dangling         int
	uncovered        int
	claims           int
	notJudged        int
	whitelistEntries int
	excludesEntries  int
	claimTokens      int
}

type crsResult struct {
	problems  []string
	dangling  []crsDangling
	uncovered []crsUncovered
	claims    []crsClaim
	notJudged []crsToken
	counts    *crsCounts
}

// ── 读件（py 文本模式：通用换行翻译）────────────────────────────────────────

// crsReadText —— py `open(path, encoding="utf-8").read()`（默认 newline=None ⇒ 通用换行）。
func crsReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s, nil
}

// crsLoadWhitelist —— py 复用的 `load_whitelist`：非空、非 `#` 的整行（strip 后）。
func crsLoadWhitelist(path string) []string {
	entries := []string{}
	text, err := crsReadText(path)
	if err != nil {
		return entries
	}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	return entries
}

// crsLoadExcludes —— 排除表真源（切源 2026-10-08 · `publish/excludes.txt` 行式）。
// 行口径 = 唯一 Go 解析件 `excludes_rules.go::parseExcludesRules`（strip 传本件原 `strings.TrimSpace`）。
// 缺件 / 读不到 ⇒ 空切片（调用方据 len==0 fail-closed）。
func crsLoadExcludes(path string) []string {
	if _, err := os.Stat(path); err != nil {
		return []string{}
	}
	text, err := crsReadText(path)
	if err != nil {
		return []string{}
	}
	return parseExcludesRules(strings.Split(text, "\n"), strings.TrimSpace)
}

// crsPublishedPath —— py 复用的 `published_path`：目录条目按前缀、文件条目按等值。
func crsPublishedPath(rel string, entries []string) bool {
	for _, e := range entries {
		if strings.HasSuffix(e, "/") {
			if strings.HasPrefix(rel, e) {
				return true
			}
		} else if rel == e {
			return true
		}
	}
	return false
}

// ── 内容身份 ────────────────────────────────────────────────────────────────

func crsSha256File(path string) (string, error) {
	fh, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	h := sha256.New()
	if _, err := io.Copy(h, fh); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// crsContentIdentity —— py `content_identity`：文件 ⇒ sha256；目录 ⇒ 件数 + 名字集 sha256。
func crsContentIdentity(root, rel string) *crsIdentity {
	p := filepath.Join(root, rel)
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		var names []string
		over := false
		_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			rp, rerr := filepath.Rel(p, path)
			if rerr != nil {
				return nil
			}
			names = append(names, filepath.ToSlash(rp))
			return nil
		})
		sort.Strings(names)
		if len(names) > crsDirCap {
			over = true
			names = names[:crsDirCap]
		}
		h := sha256.Sum256([]byte(strings.Join(names, "\n")))
		return &crsIdentity{shape: "dir", items: len(names),
			namesSha12: hex.EncodeToString(h[:])[:12], partial: over,
			rule: fmt.Sprintf("名字集 sha256（前 12 位 · 封顶 %d 件）", crsDirCap)}
	}
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		s, _ := crsSha256File(p)
		return &crsIdentity{shape: "file", bytes: int(st.Size()), sha12: s[:12]}
	}
	return &crsIdentity{shape: "absent"}
}

// crsIdentityStr —— py `identity_str`。
func crsIdentityStr(ident *crsIdentity) string {
	if ident == nil {
		return "—"
	}
	switch ident.shape {
	case "glob":
		return "glob 形状（无内容身份：它不是盘上真物）"
	case "file":
		return fmt.Sprintf("file · %d B · sha256:%s…", ident.bytes, ident.sha12)
	case "dir":
		tail := ""
		if ident.partial {
			tail = "（封顶，partial）"
		}
		return fmt.Sprintf("dir · %d 件 · names-sha256:%s…%s", ident.items, ident.namesSha12, tail)
	}
	return "不在盘"
}

// ── 说明块 / token ──────────────────────────────────────────────────────────

func crsReadWhitelistComments(root string) ([]crsCommented, []crsBlockLine) {
	path := filepath.Join(root, crsWhitelistRel)
	commented := []crsCommented{}
	blockLines := []crsBlockLine{}
	text, err := crsReadText(path)
	if err != nil {
		return commented, blockLines
	}
	inBlock := false
	for i, line := range strings.Split(text, "\n") {
		ln := i + 1
		if m := crsCommentedEntryRE.FindStringSubmatch(line); m != nil {
			commented = append(commented, crsCommented{line: ln, entry: m[1], text: strings.TrimSpace(line)})
		}
		if strings.Contains(line, crsClaimHead) {
			inBlock = true
			continue
		}
		if inBlock {
			if strings.HasPrefix(strings.TrimSpace(line), "#") && strings.HasPrefix(line, "#   ") {
				blockLines = append(blockLines, crsBlockLine{line: ln, text: strings.TrimSpace(line)})
			} else {
				inBlock = false
			}
		}
	}
	return commented, blockLines
}

func crsClaimTokens(blockLines []crsBlockLine) ([]crsToken, []crsToken) {
	judged := []crsToken{}
	notJudged := []crsToken{}
	for _, bl := range blockLines {
		text := strings.TrimSpace(strings.TrimLeft(bl.text, "#"))
		text = strings.Split(text, "本节到此为止")[0]
		text = crsParenRE.ReplaceAllString(text, " ")
		for _, tok := range crsTokenSplitRE.Split(text, -1) {
			tok = strings.TrimSpace(tok)
			tok = strings.Trim(tok, "`'\"")
			if tok == "" || !crsTokenRE.MatchString(tok) {
				continue
			}
			if !crsHasAlnumRE.MatchString(tok) {
				continue
			}
			if crsGlobShapeRE.MatchString(tok) || crsDirShapeRE.MatchString(tok) || crsExtShapeRE.MatchString(tok) {
				judged = append(judged, crsToken{line: bl.line, token: tok})
			} else {
				notJudged = append(notJudged, crsToken{line: bl.line, token: tok})
			}
		}
	}
	return judged, notJudged
}

// crsCoveredByExcludes —— 命中口径 = 唯一 Go 解析件 `excludes_rules.go::excludesRulesMatch`
// （`token == e` 或 `token` 以 `e` 去尾 `/` + `/` 开头）。返回覆盖它的条目（✗ 无覆盖 ⇒ ""）。
func crsCoveredByExcludes(token string, excludes []string) (string, bool) {
	for _, e := range excludes {
		if token == e || strings.HasPrefix(token, strings.TrimSuffix(e, "/")+"/") {
			return e, true
		}
	}
	return "", false
}

// ── 判决 ────────────────────────────────────────────────────────────────────

func crsAudit(root string) *crsResult {
	res := &crsResult{}
	wlPath := filepath.Join(root, crsWhitelistRel)
	shPath := filepath.Join(root, crsExcludesRel)

	if _, err := os.Stat(wlPath); err != nil {
		res.problems = append(res.problems, "白名单不在盘："+wlPath)
	}
	if _, err := os.Stat(shPath); err != nil {
		res.problems = append(res.problems, "`EXCLUDES` 单一真源不在盘："+shPath)
	}
	if len(res.problems) > 0 {
		return res
	}

	whitelist := crsLoadWhitelist(wlPath)
	excludes := crsLoadExcludes(shPath)
	if len(whitelist) == 0 {
		res.problems = append(res.problems, "白名单解析出 0 条活跃条目（空转 = 假绿）")
	}
	if len(excludes) == 0 {
		res.problems = append(res.problems, "`EXCLUDES` 解析出 0 条活跃条目（空转 = 假绿）")
	}
	commented, blockLines := crsReadWhitelistComments(root)
	if len(blockLines) == 0 {
		res.problems = append(res.problems,
			fmt.Sprintf("白名单里找不到说明块起始句「%s」（真源读不全 ⇒ 不给结论）", crsClaimHead))
	}
	if len(res.problems) > 0 {
		return res
	}

	// ① 空指
	for _, e := range whitelist {
		if _, err := os.Stat(filepath.Join(root, e)); err != nil {
			res.dangling = append(res.dangling, crsDangling{registry: "whitelist", entry: e,
				why: "导出面白名单条目在盘上无物（本条永远选不中任何件）", ident: crsContentIdentity(root, e)})
		}
	}
	for _, e := range excludes {
		if _, err := os.Stat(filepath.Join(root, e)); err != nil {
			res.dangling = append(res.dangling, crsDangling{registry: "EXCLUDES", entry: e,
				why: "排除清单条目在盘上无物（一条也排除不掉）", ident: crsContentIdentity(root, e)})
		}
	}

	// ② 悬空
	for _, e := range excludes {
		if _, err := os.Stat(filepath.Join(root, e)); err != nil {
			continue
		}
		if !crsPublishedPath(e, whitelist) {
			res.uncovered = append(res.uncovered, crsUncovered{entry: e,
				why:   "盘上有物，但没有任何白名单条目覆盖它 ⇒ 这条排除排不掉任何东西",
				ident: crsContentIdentity(root, e)})
		}
	}

	// ③ 文案与实际不符
	for _, c := range commented {
		if _, err := os.Stat(filepath.Join(root, c.entry)); err == nil {
			res.claims = append(res.claims, crsClaim{rule: "C1", line: c.line, token: c.entry,
				why:   "注释说它盘上无此物，而现读盘上**有** ⇒ 该注释断言的现状已过时",
				ident: crsContentIdentity(root, c.entry)})
		}
	}
	judged, notJudged := crsClaimTokens(blockLines)
	res.notJudged = notJudged
	for _, j := range judged {
		if _, ok := crsCoveredByExcludes(j.token, excludes); !ok {
			nearest := []string{}
			head := j.token
			if idx := strings.Index(j.token, "/"); idx >= 0 {
				head = j.token[:idx]
			}
			for _, e := range excludes {
				if strings.Contains(e, head) {
					nearest = append(nearest, e)
					if len(nearest) >= 3 {
						break
					}
				}
			}
			var ident *crsIdentity
			if strings.Contains(j.token, "*") || strings.Contains(j.token, "?") {
				ident = &crsIdentity{shape: "glob", globNote: "glob 形状（不是盘上真物，无内容身份）"}
			} else {
				ident = crsContentIdentity(root, j.token)
			}
			res.claims = append(res.claims, crsClaim{rule: "C2", line: j.line, token: j.token,
				why:     "说明块说「以模式表达」，而 `EXCLUDES` 里找不到能覆盖它的字面条目（同 token 或它的父目录）",
				nearest: nearest, hasNearest: true, ident: ident})
		}
	}

	res.counts = &crsCounts{
		dangling:         len(res.dangling),
		uncovered:        len(res.uncovered),
		claims:           len(res.claims),
		notJudged:        len(res.notJudged),
		whitelistEntries: len(whitelist),
		excludesEntries:  len(excludes),
		claimTokens:      len(judged),
	}
	return res
}

// ── 人读面 ──────────────────────────────────────────────────────────────────

func crsPrintReport(stdout io.Writer, root string, res *crsResult, strict bool) int {
	c := res.counts
	fmt.Fprintln(stdout, "登记表 ↔ 盘上真物对拍 —— 域：`publish/whitelist.txt` × `publish/excludes.txt`（排除表行式真源）")
	fmt.Fprintf(stdout, "仓：%s\n", root)
	if c == nil {
		for _, p := range res.problems {
			fmt.Fprintf(stdout, "  ✗ %s\n", p)
		}
		fmt.Fprintln(stdout, "⇒ **不给结论**（rc=2）：真源读不全 ⇒ 绿就是假绿")
		return 2
	}
	fmt.Fprintf(stdout, "两册规模：白名单活跃 %d 条 · `EXCLUDES` 活跃 %d 条\n", c.whitelistEntries, c.excludesEntries)
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "── ① 空指（条目在盘上无物）：%d 处\n", c.dangling)
	for _, d := range res.dangling {
		fmt.Fprintf(stdout, "   · [%s] %s —— %s · 内容身份：%s\n", d.registry, d.entry, d.why, crsIdentityStr(d.ident))
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "── ② 悬空（`EXCLUDES` 条目有物、却不在导出面内）：%d 处\n", c.uncovered)
	for _, u := range res.uncovered {
		fmt.Fprintf(stdout, "   · %s —— %s · 内容身份：%s\n", u.entry, u.why, crsIdentityStr(u.ident))
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "── ③ 文案与实际不符：%d 处\n", c.claims)
	for _, cl := range res.claims {
		near := ""
		if cl.hasNearest && len(cl.nearest) > 0 {
			near = "· `EXCLUDES` 里最像的：" + strings.Join(cl.nearest, " / ")
		}
		fmt.Fprintf(stdout, "   · [%s] :%d `%s` —— %s %s\n", cl.rule, cl.line, cl.token, cl.why, near)
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "── 未判 token（口径外 · **逐条列出**不静默）：%d 个（受判 token 形状 = 带 glob / 尾斜杠 / 路径.后缀）\n", c.notJudged)
	for _, nj := range res.notJudged {
		fmt.Fprintf(stdout, "   · :%d `%s`\n", nj.line, nj.token)
	}
	fmt.Fprintln(stdout, "")
	hits := c.dangling + c.uncovered + c.claims
	if hits == 0 {
		fmt.Fprintln(stdout, "⇒ 三格全 0（rc=0）：登记表 ↔ 盘上真物一致")
		return 0
	}
	if strict {
		fmt.Fprintf(stdout, "⇒ **阻断档（--strict）**：三格合计 %d 处 ⇒ rc=1（逐条点名如上；报告档不阻断）\n", hits)
		return 1
	}
	fmt.Fprintf(stdout, "⇒ 报告档（rc=0）：三格合计 %d 处**只报不拦** —— 零化要改登记件（`publish/whitelist.txt` 的注释 /"+
		" `EXCLUDES` 的条目），属**另一笔**的活（本件只点名，不代拍）\n", hits)
	return 0
}

// ── 机器面（Python `json.dumps(..., ensure_ascii=False)` 逐字）────────────────

type crsOM struct {
	keys []string
	vals []any
}

func (o *crsOM) set(k string, v any) {
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
}

func crsPyQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
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

func crsJSONEnc(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return crsPyQuote(x)
	case int:
		return strconv.Itoa(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case []string:
		parts := make([]string, 0, len(x))
		for _, s := range x {
			parts = append(parts, crsPyQuote(s))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []any:
		parts := make([]string, 0, len(x))
		for _, s := range x {
			parts = append(parts, crsJSONEnc(s))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *crsOM:
		parts := make([]string, 0, len(x.keys))
		for i, k := range x.keys {
			parts = append(parts, crsPyQuote(k)+": "+crsJSONEnc(x.vals[i]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "null"
}

func crsIdentOM(ident *crsIdentity) *crsOM {
	if ident == nil {
		return &crsOM{}
	}
	o := &crsOM{}
	switch ident.shape {
	case "glob":
		o.set("shape", "glob")
		o.set("note", "glob 形状（不是盘上真物，无内容身份）")
	case "file":
		o.set("shape", "file")
		o.set("bytes", ident.bytes)
		o.set("sha256_12", ident.sha12)
	case "dir":
		o.set("shape", "dir")
		o.set("items", ident.items)
		o.set("names_sha256_12", ident.namesSha12)
		o.set("partial", ident.partial)
		o.set("rule", ident.rule)
	default:
		o.set("shape", "absent")
	}
	return o
}

func crsEmitJSON(res *crsResult, fields string) string {
	payload := &crsOM{}
	var dangling, uncovered, claims, notJudged any
	if res.counts != nil {
		dl := []any{}
		for _, d := range res.dangling {
			o := &crsOM{}
			o.set("registry", d.registry)
			o.set("entry", d.entry)
			o.set("why", d.why)
			o.set("identity", crsIdentOM(d.ident))
			dl = append(dl, o)
		}
		dangling = dl
		ul := []any{}
		for _, u := range res.uncovered {
			o := &crsOM{}
			o.set("entry", u.entry)
			o.set("why", u.why)
			o.set("identity", crsIdentOM(u.ident))
			ul = append(ul, o)
		}
		uncovered = ul
		cl := []any{}
		for _, c := range res.claims {
			o := &crsOM{}
			o.set("rule", c.rule)
			o.set("line", c.line)
			o.set("token", c.token)
			o.set("why", c.why)
			if c.hasNearest {
				n := []string{}
				n = append(n, c.nearest...)
				o.set("nearest", n)
			}
			o.set("identity", crsIdentOM(c.ident))
			cl = append(cl, o)
		}
		claims = cl
		nj := []any{}
		for _, t := range res.notJudged {
			o := &crsOM{}
			o.set("line", t.line)
			o.set("token", t.token)
			nj = append(nj, o)
		}
		notJudged = nj
	} else {
		empty := []any{}
		dangling, uncovered, claims, notJudged = empty, empty, empty, empty
	}

	var vDangling, vUncovered, vClaims, vNotJudged, vWL, vEX any
	if res.counts != nil {
		vDangling = res.counts.dangling
		vUncovered = res.counts.uncovered
		vClaims = res.counts.claims
		vNotJudged = res.counts.notJudged
		vWL = res.counts.whitelistEntries
		vEX = res.counts.excludesEntries
	}
	payload.set("schema", "registry-vs-disk/1")
	payload.set("dangling", vDangling)
	payload.set("uncovered", vUncovered)
	payload.set("claims", vClaims)
	payload.set("not_judged", vNotJudged)
	payload.set("whitelist_entries", vWL)
	payload.set("excludes_entries", vEX)

	detail := &crsOM{}
	detail.set("dangling", dangling)
	detail.set("uncovered", uncovered)
	detail.set("claims", claims)
	detail.set("not_judged", notJudged)
	probs := []string{}
	probs = append(probs, res.problems...)
	detail.set("problems", probs)
	payload.set("detail", detail)

	if fields != "" {
		sel := &crsOM{}
		for _, f := range strings.Split(fields, ",") {
			k := strings.TrimSpace(f)
			if k == "" {
				continue
			}
			idx := -1
			for i, pk := range payload.keys {
				if pk == k {
					idx = i
					break
				}
			}
			if idx >= 0 {
				sel.set(k, payload.vals[idx])
			} else {
				sel.set(k, nil)
			}
		}
		payload = sel
	}
	return crsJSONEnc(payload)
}

// ── 缺口 `Q-176` 口径：`--json` 表外字段名 ⇒ 用法错 rc=2 ＋ 印字段表 ──────────

func crsRefuseUnknownJSONFields(argv []string, stderr io.Writer) *int {
	want := []string{}
	for i, x := range argv {
		if x == "--json" {
			v := ""
			if i+1 < len(argv) {
				v = argv[i+1]
			}
			want = append(want, strings.Split(v, ",")...)
		} else if strings.HasPrefix(x, "--json=") {
			want = append(want, strings.Split(strings.SplitN(x, "=", 2)[1], ",")...)
		}
	}
	fields := []string{}
	for _, f := range want {
		if t := strings.TrimSpace(f); t != "" {
			fields = append(fields, t)
		}
	}
	bad := []string{}
	isKnown := func(f string) bool {
		for _, k := range crsJSONFields {
			if k == f {
				return true
			}
		}
		return false
	}
	for _, f := range fields {
		if !isKnown(f) {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(bad))
	for _, b := range bad {
		quoted = append(quoted, "`"+b+"`")
	}
	tbl := make([]string, 0, len(crsJSONFields))
	for _, f := range crsJSONFields {
		tbl = append(tbl, "`"+f+"`")
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", strings.Join(quoted, " · "))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", strings.Join(tbl, " · "))
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开")
	rc := 2
	return &rc
}

// ── 默认根定位（见件头声明行 (a)：与 py 的 `__file__` 起点不同）───────────────

func crsResolveRoot(arg string) string {
	if arg != "" {
		if abs, err := filepath.Abs(arg); err == nil {
			return abs
		}
		return arg
	}
	start, err := os.Getwd()
	if err != nil {
		return "."
	}
	d, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	for i := 0; i < 8; i++ {
		if crsIsFile(filepath.Join(d, crsWhitelistRel)) && crsIsFile(filepath.Join(d, crsExcludesRel)) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func crsIsFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ── 自检（合成夹具 · 成对 · 与 py `run_self_test` 逐字对齐）────────────────────

const crsFixtureWhitelist = `# 合成白名单（自检夹具 · 不是真仓件）
# 说明：这些在脚本 EXCLUDES 中以模式表达 —— ★ **本节只解释 EXCLUDES 的模式**（不是「本节之后的行都排除」）
#   gateway/fleet.yaml  部署痕迹
#   sub/drop.txt  夹具排除件
sub/
gateway/
keep.txt
# gone/ ← ★ 空指：盘上无此目录（夹具）
`

const crsFixturePublishSh = `# 合成排除表（自检夹具 · publish/excludes.txt 行式真源 · 裸条目）
gateway/fleet.yaml
sub/drop.txt
`

func crsBuildFixture(tmp string) {
	_ = os.MkdirAll(filepath.Join(tmp, "publish"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmp, "scripts", "build"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmp, "sub"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmp, "gateway"), 0o755)
	_ = os.WriteFile(filepath.Join(tmp, "keep.txt"), []byte("keep\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tmp, "sub", "drop.txt"), []byte("drop\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tmp, "gateway", "fleet.yaml"), []byte("fleet\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tmp, "publish", "whitelist.txt"), []byte(crsFixtureWhitelist), 0o644)
	_ = os.WriteFile(filepath.Join(tmp, "publish", "excludes.txt"), []byte(crsFixturePublishSh), 0o644)
}

func crsSelfTest(stdout io.Writer) int {
	type check struct {
		name  string
		want  int
		got   int
		extra string
	}
	checks := []check{}

	caseFn := func(name string, wantRC int, strict bool, mutate, drop string) {
		tmp, err := os.MkdirTemp("", "crs-selftest-")
		if err != nil {
			tmp = filepath.Join(os.TempDir(), "crs-selftest-fallback")
			_ = os.MkdirAll(tmp, 0o755)
		}
		defer os.RemoveAll(tmp)
		crsBuildFixture(tmp)
		if mutate == "dangling" {
			p := filepath.Join(tmp, "publish", "excludes.txt")
			if t, e := crsReadText(p); e == nil {
				_ = os.WriteFile(p, []byte(t+"never-existed/\n"), 0o644)
			}
		}
		if mutate == "uncovered" {
			_ = os.WriteFile(filepath.Join(tmp, "loose.txt"), []byte("loose\n"), 0o644)
			p := filepath.Join(tmp, "publish", "excludes.txt")
			if t, e := crsReadText(p); e == nil {
				_ = os.WriteFile(p, []byte(t+"loose.txt\n"), 0o644)
			}
		}
		if mutate == "claim" {
			p := filepath.Join(tmp, "publish", "whitelist.txt")
			if t, e := crsReadText(p); e == nil {
				lines := strings.Split(t, "\n")
				out := []string{}
				done := false
				for _, ln := range lines {
					out = append(out, ln)
					if !done && strings.Contains(ln, crsClaimHead) {
						out = append(out, "#   *.rej  夹具点名的模式")
						done = true
					}
				}
				_ = os.WriteFile(p, []byte(strings.Join(out, "\n")), 0o644)
			}
		}
		if mutate == "stale_comment" {
			_ = os.MkdirAll(filepath.Join(tmp, "gone"), 0o755)
			_ = os.WriteFile(filepath.Join(tmp, "gone", "x.txt"), []byte("x\n"), 0o644)
		}
		if drop != "" {
			_ = os.Remove(filepath.Join(tmp, drop))
		}
		res := crsAudit(tmp)
		rc := 2
		extra := ""
		if res.counts != nil {
			total := res.counts.dangling + res.counts.uncovered + res.counts.claims
			if total == 0 {
				rc = 0
			} else if strict {
				rc = 1
			} else {
				rc = 0
			}
			extra = fmt.Sprintf("（空指 %d · 悬空 %d · 文案 %d）",
				res.counts.dangling, res.counts.uncovered, res.counts.claims)
		}
		checks = append(checks, check{name: name, want: wantRC, got: rc, extra: extra})
	}

	caseFn("正控 · 干净夹具（报告档）⇒ 0", 0, false, "", "")
	caseFn("正控 · 干净夹具（阻断档）⇒ 0", 0, true, "", "")
	caseFn("负控② · 造一**空指行** ⇒ 阻断档判红", 1, true, "dangling", "")
	caseFn("负控③ · 造一**悬空行** ⇒ 阻断档判红", 1, true, "uncovered", "")
	caseFn("负控④ · 说明块点一个 `EXCLUDES` 里没有的 token ⇒ 阻断档判红", 1, true, "claim", "")
	caseFn("负控⑤ · 注释说「盘上无」而盘上有 ⇒ 阻断档判红", 1, true, "stale_comment", "")
	caseFn("判不了 · 白名单不在 ⇒ 2（不给结论）", 2, false, "", filepath.Join("publish", "whitelist.txt"))

	bad := 0
	for _, c := range checks {
		ok := c.got == c.want
		mark := "✓"
		if !ok {
			mark = "✗"
			bad++
		}
		fmt.Fprintf(stdout, "  %s %s —— 要 %d · 得 %d %s\n", mark, c.name, c.want, c.got, c.extra)
	}
	if bad == 0 {
		fmt.Fprintf(stdout, "自检结论: 全过（断言 %d 条 · 失败 %d）\n", len(checks), bad)
		return 0
	}
	fmt.Fprintf(stdout, "自检结论: **不过**（断言 %d 条 · 失败 %d）\n", len(checks), bad)
	return 1
}

// ── 入口（= py `main()`）────────────────────────────────────────────────────

const crsHelpText = `usage: check-registry-vs-disk.py [-h] [--repo REPO] [--strict]
                                 [--json JSON_FIELDS] [--self-test]

登记表 ↔ 盘上真物对拍（空指 / 悬空 / 文案与实际不符）

optional arguments:
  -h, --help          show this help message and exit
  --repo REPO         仓库路径（默认本仓；自检 / 演练可指向别处）
  --strict            阻断档：任一格 > 0 ⇒ rc=1（默认报告档 rc=0）
  --json JSON_FIELDS  机器面：逗号分隔字段（dangling,uncovered,claims,not_judged,detail,s
                      chema…）
  --self-test         只跑合成夹具自检
`

func crsArgError(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, "usage: check-registry-vs-disk.py [-h] [--repo REPO] [--strict]\n                                 [--json JSON_FIELDS] [--self-test]\n")
	fmt.Fprintf(stderr, "check-registry-vs-disk.py: error: %s\n", msg)
	return 2
}

// registryVsDiskGateMain —— 入口（= py `main()`）。
func registryVsDiskGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := crsRefuseUnknownJSONFields(argv, stderr); rc != nil {
		return *rc
	}

	repoArg := ""
	strict := false
	selfTest := false
	jsonFields := ""
	haveJSON := false

	i := 0
	for i < len(argv) {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, crsHelpText)
			return 0
		case a == "--strict":
			strict = true
		case a == "--self-test":
			selfTest = true
		case a == "--repo":
			if i+1 >= len(argv) {
				return crsArgError(stderr, "argument --repo: expected one argument")
			}
			repoArg = argv[i+1]
			i++
		case strings.HasPrefix(a, "--repo="):
			repoArg = a[len("--repo="):]
		case a == "--json":
			if i+1 >= len(argv) {
				return crsArgError(stderr, "argument --json: expected one argument")
			}
			jsonFields = argv[i+1]
			haveJSON = true
			i++
		case strings.HasPrefix(a, "--json="):
			jsonFields = a[len("--json="):]
			haveJSON = true
		default:
			return crsArgError(stderr, "unrecognized arguments: "+a)
		}
		i++
	}

	if selfTest {
		fmt.Fprintln(stdout, "── 自检（合成夹具 · 成对）──")
		return crsSelfTest(stdout)
	}

	root := crsResolveRoot(repoArg)
	res := crsAudit(root)
	rc := crsPrintReport(stdout, root, res, strict)
	if haveJSON {
		fmt.Fprintln(stdout, crsEmitJSON(res, jsonFields))
	}
	return rc
}
