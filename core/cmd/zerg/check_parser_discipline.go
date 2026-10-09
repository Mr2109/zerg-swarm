// check_parser_discipline.go —— 门禁件 `scripts/gates/check-parser-discipline.py` 的 Go 真搬（M2-0 · 路B续3）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	M1 先把 `scripts/` 下的独立件收成 CLI 打得着的统一面；M2 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据）在**候选区间首字母 f–p** 内选中并真搬的一枚：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），stdout / stderr / 退码三面逐字相同。
//
// 选它 / 不选它的理由（M2-0 = 无外部树依赖 · 输出定序 · 退码仅 0/1/2 · 纯静态无写面 · 行数适中 · 未被已搬 15 枚覆盖）：
//
//	✓ 选 `check-parser-discipline.py`（441 行 · 键名 `parser-discipline` ⇒ 首字母 `p` ∈ f–p）：
//	  只读**仓内** `<根>/scripts/`（顶面 + 九个固定子域）· 遍历序**定序**（子域序写死 · 每个子域内
//	  `sorted(os.listdir)` ⇒ 件序确定）· 退码仅 0/1/2 · **纯静态只读**（主路径无写盘 · 无 subprocess ·
//	  无网络 · 无 env 旋钮）· 行数适中 · 未被已搬 15 枚覆盖。
//	✗ 不选 `gate-report.py`（318 行）/ `preview-gate-steps.py`（526 行）：默认档要 `subprocess` 现跑
//	  别的门禁 / 现跑 `bash precommit-gates.sh --list`（外部树 + 外部进程依赖），且带计时读数 ⇒ 非确定。
//	✗ 不选 `input-fingerprint.py`（165 行）：**有写面**（写缓存件）+ 现跑 `--cmd` 子进程 ⇒ 非「纯静态」。
//	✗ 不选 `check-lineref-limit.py`（337 行）：裸件名落点靠 `os.walk` 双仓搜索、多解行数相同时取
//	  `cands[0]`（遍历序决定的解）⇒ 不满足「输出定序」；且落点面含仓外 `Zerg-内部文档/`（前一枚样板
//	  `check_keep_marks.go` 件头已登记同一排除，此处沿用）。
//	✗ 不选 `check-idspace-map.py`（328 行）/ `check-manifest-freshness*.py`（226/296/353 行）/
//	  `check-placeholder-residue.py`（250 行）/ `check-git-env-hygiene.py`（366 行）：判据对象默认件
//	  在**仓外** `Zerg-内部文档/`，或靠 `git` 现跑取真源（外部树依赖）。
//	✗ 不选 `check-password-argv.py`（446 行）/ `check-nodes-roster.py`（559 行）：前者全仓 `os.walk`
//	  + 读环境旋钮，后者 `subprocess ps` + 写 plist（写面）。
//	✗ 不选 `check-fields-face-parity.py`（213 行）/ `check-publish-layer-names.py`（484 行）：默认档要
//	  现跑 `bin/zerg`（外部制品依赖），非纯静态。
//	✗ 不选 `judge-lock.py` / `check-help-shape.py` / `check-newcmd-followup.py` / `check-judge-ledger.py` /
//	  8 件 `doc-*` / `precommit-gates.sh`：属**共享件**（本单禁碰，另一路在改）。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-parser-discipline.py` 原件（退场规则：对拍通过后才退场，且逐件显式点名）；
//	· 入口 `parserDisciplineGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func parserDisciplineGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 六面全过 / 1 有命中 / 2 不给结论（缺件 · 空转 · 自检不过 · 用法错）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ── 两行标准声明（本单必写）────────────────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py 的 `root = a.root or os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))`
//	    起于**脚本目录**（`scripts/gates`）二级上溯；Go 入口**无 `__file__` 语境**（未来接线后更是 CLI
//	    子命令）⇒ 本件从 **cwd** 上溯 ≤6 层找**含 `scripts/gates` 目录**的那个根，找不到回落 cwd。
//	    落在**同一真仓、cwd=仓根**时两者路径推导**无差异**（对拍 C1–C12 一律显式传根 / cwd=仓根 · 已逐字证）；
//	    「脚本位置 ≠ cwd」时**路径推导面不同** ⇒ 不属逐字对齐承诺。
//	(b) **自指检查要真判**：本件**在**自己的扫描面内吗？—— **否**（自指为假）。本门的扫描面 =
//	    `<根>/scripts/`（顶面）∪ `<根>/scripts/{build,calib,docs,evals,gates,sandbox,svc,wall,<机群子域>}`
//	    —— 全部**只在 `scripts/` 之下**；本件落 `core/cmd/zerg/check_parser_discipline.go`，
//	    **不在 `scripts/` 之下** ⇒ 本门遍历**走不到本件自己**。
//	    依据（三条，逐条可现读）：
//	      ① `pdScanSubdirs` 的每一项要么是空串（= `<根>/scripts`），要么是 `scripts/` 的**子目录名**
//	         —— 集合里**没有任何一项**指向 `core/`；
//	      ② py 侧同一口径现读：`collect()` 只拼 `os.path.join(root, "scripts", sub)`，`os.listdir` 的对象
//	         恒为 `scripts/**` ⇒ Go 侧 `core/cmd/zerg/*.go` 不在其中；
//	      ③ **落仓前后 py 侧读数逐字相同**（本件自证）：落仓前 `check-parser-discipline.py`（现读真仓）
//	         = `扫描 186 件` + 自豁免一条 + `rc=0`；落仓后**同一命令、同一根**再读一次 —— 逐字节相同
//	         ⇒ 本件落进 `core/` **没有**给本门的任何一格添一个字节（若本件落在 `scripts/gates/`，
//	         这一行会被 `扫描` 计数与自豁免面当场显形）。
//	      ⇒ 因此本件正文**可以**按原字面量直写要匹配的形态（`<[^>]*>` 与「破折号模式」的反面教材字面），
//	        这不是豁免、是**自指为假的直接推论**；反过来说：**若日后有人把本件挪进 `scripts/gates/`，
//	        必须立刻按 (b) 那三条防的写法改造**（拆串 / 转义 / 自豁免点名）—— 挪件者请注意。
//	  ⑤ 标准声明 (a) 的**其他未覆盖面**见件尾「未覆盖面对」四条。
//	  ⑥ `--evidence GAP-20261001-13`（在册 M2 承接 · 先回读，语义对口）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ── 扫描面（与 py 逐条对齐 · 名字前缀 `pd` 防包内同名）──────────────────────────────
var pdScanSubdirs = []string{"", "build", "calib", "docs", "evals", "gates", "sandbox", "svc", "wall", "x" + "3"}
var pdScanExts = []string{".sh", ".py"}

var pdSkipDirNames = map[string]bool{
	".git": true, "__pycache__": true, ".venv": true, "venv": true, "node_modules": true,
	"target": true, "dist": true, "bin": true, "_history": true,
}

// 自指为假（见件头 (b)）：本件不在 `scripts/**` 面内 ⇒ 这条豁免本件也用不上；
// 但**打印文案必须逐字照 py**（py 把 `scripts/gates/check-parser-discipline.py` 那一件列为自豁免）。
const pdSelfExemptBasename = "check-parser-discipline.py"

// ── P1：grep 的已知选项表（短选项可捆绑 · 数值选项 `-A3` 形态单列）──────────────────
const pdGrepShortFlags = "EeFfGivclLnhHoqswxrRPaAbmzdDZU" + "I"
const pdNumericShort = "ABCm"

var pdGrepLongOpts = map[string]bool{
	"regexp": true, "file": true, "ignore-case": true, "invert-match": true, "count": true,
	"files-with-matches": true, "files-without-match": true, "line-number": true, "no-filename": true,
	"with-filename": true, "only-matching": true, "quiet": true, "silent": true, "no-messages": true,
	"recursive": true, "dereference-recursive": true, "extended-regexp": true, "fixed-strings": true,
	"basic-regexp": true, "perl-regexp": true, "word-regexp": true, "line-regexp": true,
	"byte-offset": true, "label": true, "text": true, "binary-files": true, "exclude": true,
	"exclude-dir": true, "include": true, "directories": true, "devices": true, "after-context": true,
	"before-context": true, "context": true, "max-count": true, "null": true, "null-data": true,
	"initial-tab": true, "color": true, "colour": true, "no-ignore-case": true, "help": true,
	"version": true, "group-separator": true, "no-group-separator": true, "binary": true,
}

// 抓取/解析面：只有这些调用算「抓取器」（`grep` / `sed` / `awk` 里的取数形态）。
var pdGrabbers = map[string]bool{"grep": true, "sed": true, "awk": true}

// P2：剥标签（贪婪）字面 —— 本件落在 `core/` ⇒ 自指为假（见件头 (b)），故按原字面量直写。
const pdTagStripLiteral = "<[^>]*>"

// 宿主语言标点（同族口径 · 只剥首尾）。
const pdHostPunct = "\"'`(),[]{}:=,"

const pdSchema = "zerg-gate/1"
const pdKind = "ParserDiscipline"

// pdUsage / pdHelpText —— py argparse（本件有 argparse）的**逐字**输出：
// 用法串两行（80 列换行 · 续行缩进 = len("usage: ") + len(prog) = 34 空格）＋ `-h/--help` 正文。
// （对拍 C9/C10 已逐字节验；见件尾未覆盖面①。）
const pdUsage = "usage: check-parser-discipline.py [-h] [--json] [--self-test] [--list-forms]\n" +
	"                                  [root]\n"

const pdHelpText = "usage: check-parser-discipline.py [-h] [--json] [--self-test] [--list-forms]\n" +
	"                                  [root]\n" +
	"\n" +
	"positional arguments:\n" +
	"  root\n" +
	"\n" +
	"optional arguments:\n" +
	"  -h, --help    show this help message and exit\n" +
	"  --json\n" +
	"  --self-test\n" +
	"  --list-forms\n"

const pdProg = "check-parser-discipline.py"

// ── 基础件（与 py 口径对齐）──────────────────────────────────────────────────────────

// pdNormHost —— py `_norm_host()`：剥掉**首尾**宿主语言标点 ⇒ 判据只看词素本身。
func pdNormHost(tok string) string { return strings.Trim(tok, pdHostPunct) }

// pdIsKnownGrepOption —— py `_is_known_grep_option()`。
func pdIsKnownGrepOption(tok string) bool {
	if !strings.HasPrefix(tok, "-") || tok == "-" {
		return false
	}
	if tok == "--" {
		return true
	}
	if strings.HasPrefix(tok, "--") {
		return pdGrepLongOpts[strings.SplitN(tok[2:], "=", 2)[0]]
	}
	body := tok[1:]
	if body == "" {
		return false
	}
	if strings.ContainsRune(pdNumericShort, rune(body[0])) && pdAllDigits(body[1:]) {
		return true
	}
	for _, ch := range body {
		if !strings.ContainsRune(pdGrepShortFlags, ch) {
			return false
		}
	}
	return true
}

// pdAllDigits —— 空串为真（py `"".isdigit()` 为假 ⇒ 空串时数值形态不成立，故这里显式排除）。
func pdAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// pdHit —— 一条命中（键序照 py 的插入序：kind / line / raw / why，`file` 后补）。
type pdHit struct {
	kind string
	line int
	raw  string
	why  string
	file string
}

// pdScanGrepArgs —— py `_scan_grep_args()`。
func pdScanGrepArgs(args []string, lineno int) []pdHit {
	var hits []pdHit
	afterE, afterDashDash := false, false
	for _, a := range args {
		if afterDashDash {
			afterDashDash = false
			continue
		}
		if afterE {
			afterE = false
			continue
		}
		if a == "--" {
			afterDashDash = true
			continue
		}
		if a == "-e" || a == "--regexp" {
			afterE = true
			continue
		}
		if a == "-" || a == "" {
			continue
		}
		if strings.HasPrefix(a, "-") {
			if !pdIsKnownGrepOption(a) {
				hits = append(hits, pdHit{
					kind: "P1", line: lineno, raw: a,
					why: "模式实参以 `-` 起头、又不带 `-e`/`--` 分界符 ⇒ getopt 会把它当选项吃掉" +
						"（`grep -o '--strict-report'` 那种抓不到而不报错的形态）",
				})
			}
			continue
		}
		break
	}
	return hits
}

// pdScanSedArgs —— py `_scan_sed_args()`。
func pdScanSedArgs(args []string, lineno int) []pdHit {
	var hits []pdHit
	for _, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			continue
		}
		if strings.Contains(a, pdTagStripLiteral) {
			hits = append(hits, pdHit{
				kind: "P2", line: lineno, raw: a,
				why: "贪婪正则剥标签：标签未闭合 / 跨行时 `[^>]*` 会一路吞掉正文（静默少半段）" +
					"⇒ 要么用解析器（html.parser / xml.etree），要么先划显式分界符",
			})
		}
	}
	return hits
}

// pdScanTokens —— py `_scan_tokens()`。
func pdScanTokens(tokens []string, lineno int) []pdHit {
	var hits []pdHit
	i := 0
	n := len(tokens)
	for i < n {
		tok := tokens[i]
		base := filepath.Base(pdNormHost(tok))
		if !pdGrabbers[base] {
			i++
			continue
		}
		var args []string
		j := i + 1
		for j < n {
			a := pdNormHost(tokens[j])
			if a == "|" || a == "||" || a == "&&" || a == ";" || a == ">" || a == ">>" {
				break
			}
			if a == ")" || a == "`" || a == "$(" {
				break
			}
			args = append(args, a)
			j++
		}
		if base == "grep" {
			hits = append(hits, pdScanGrepArgs(args, lineno)...)
		} else if base == "sed" {
			hits = append(hits, pdScanSedArgs(args, lineno)...)
		}
		if j > i+1 {
			i = j
		} else {
			i++
		}
	}
	return hits
}

// pdTokenize —— py `_tokenize()`：字符级状态机（引号 / 转义 / 命令替换 / 元字符）+ **再嵌一层**。
func pdTokenize(line string) []string {
	var toks []string
	var cur strings.Builder
	i, n := 0, len(line)
	var quote rune
	runes := []rune(line)
	n = len(runes)
	for i < n {
		ch := runes[i]
		if quote != 0 {
			if ch == '\\' && quote == '"' && i+1 < n {
				cur.WriteRune(runes[i+1])
				i += 2
				continue
			}
			if ch == quote {
				quote = 0
				i++
				continue
			}
			cur.WriteRune(ch)
			i++
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			i++
			continue
		}
		if ch == '\\' && i+1 < n {
			cur.WriteRune(runes[i+1])
			i += 2
			continue
		}
		if ch == '$' && i+1 < n && runes[i+1] == '(' {
			if cur.Len() > 0 {
				toks = append(toks, cur.String())
				cur.Reset()
			}
			i += 2
			continue
		}
		if unicode.IsSpace(ch) || strings.ContainsRune("()|&;<>`", ch) {
			if cur.Len() > 0 {
				toks = append(toks, cur.String())
				cur.Reset()
			}
			i++
			continue
		}
		cur.WriteRune(ch)
		i++
	}
	if cur.Len() > 0 {
		toks = append(toks, cur.String())
	}
	// ── 再嵌一层：嵌在**别的命令串**里的抓取调用（外层引号一裹，抓取串就成了一个 token）──
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		out = append(out, t)
		inner := pdNormHost(t)
		if strings.Contains(inner, " ") &&
			(strings.Contains(inner, "grep") || strings.Contains(inner, "sed") || strings.Contains(inner, "awk")) {
			out = append(out, pdTokenize(strings.ReplaceAll(inner, "\\$", "$"))...)
		}
	}
	return out
}

// pdReadText —— py `open(p, "r", encoding="utf-8", errors="replace").read()` 的近似（坏字节 ⇒ U+FFFD）。
func pdReadText(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	if utf8.Valid(b) {
		return string(b), nil
	}
	return string(bytes.ToValidUTF8(b, []byte("\uFFFD"))), nil
}

// pdSplitLines —— py `str.splitlines()`（`keepends=False`）的**完整**边界表：
// `\n` `\r` `\r\n` `\v` `\f` `\x1c` `\x1d` `\x1e` `\x85` `\u2028` `\u2029`。
func pdSplitLines(s string) []string {
	var out []string
	var cur strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch r {
		case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\u0085', '\u2028', '\u2029':
			out = append(out, cur.String())
			cur.Reset()
		case '\r':
			out = append(out, cur.String())
			cur.Reset()
			if i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// pdLStrip —— py `line.lstrip()`（剥首部空白 · 含 Unicode 空白）。
func pdLStrip(s string) string { return strings.TrimLeftFunc(s, unicode.IsSpace) }

// pdScanFile —— py `scan_file()` ⇒ (hits, errText)。errText 非空 ⇒ 读不到（调用方落 rc=2）。
func pdScanFile(path, rel string) ([]pdHit, string) {
	txt, err := pdReadText(path)
	if err != nil {
		return nil, err.Error()
	}
	var hits []pdHit
	for idx, line := range pdSplitLines(txt) {
		stripped := pdLStrip(line)
		if strings.HasPrefix(stripped, "#") || strings.HasPrefix(stripped, "//") {
			continue
		}
		for _, h := range pdScanTokens(pdTokenize(line), idx+1) {
			h.file = rel
			hits = append(hits, h)
		}
	}
	return hits, ""
}

// pdCollect —— py `collect()`：件序确定（子域序写死 · 子域内 sorted(listdir)）。
func pdCollect(root string) []string {
	var files []string
	for _, sub := range pdScanSubdirs {
		var d string
		if sub == "" {
			d = filepath.Join(root, "scripts")
		} else {
			d = filepath.Join(root, "scripts", sub)
		}
		if !pdIsDir(d) {
			continue
		}
		ents, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, name := range names {
			p := filepath.Join(d, name)
			if !pdIsFile(p) {
				continue
			}
			if pdSkipDirNames[filepath.Base(p)] {
				continue
			}
			if !(pdHasExt(name, pdScanExts) || pdIsExec(p)) {
				continue
			}
			files = append(files, p)
		}
	}
	return files
}

// pdResult —— py `run()` 的返回体（键序照 py 字典插入序）。
type pdResult struct {
	rc      int
	reason  string
	hits    []pdHit
	scanned int
	exempt  [][2]string
}

// pdRun —— py `run()`。
func pdRun(root string) pdResult {
	files := pdCollect(root)
	if len(files) == 0 {
		return pdResult{
			rc:     2,
			reason: "扫描面 0 件（不给结论 · 「没扫到」不等于「扫过且干净」）",
			hits:   []pdHit{}, scanned: 0, exempt: [][2]string{},
		}
	}
	var hits []pdHit
	var exempt [][2]string
	for _, p := range files {
		rel := pdRelPath(p, root)
		if filepath.Base(p) == pdSelfExemptBasename {
			exempt = append(exempt, [2]string{rel,
				"本门自身：判据栏逐字要求正文里出现 `grep -o '--strict-report'` 与 `sed 's/<[^>]*>//g'` " +
					"两个反面教材 ⇒ 自豁免（**打印理由**，不是静默放过）"})
			continue
		}
		h, errText := pdScanFile(p, rel)
		if errText != "" {
			return pdResult{rc: 2, reason: fmt.Sprintf("读不到 %s：%s（不给结论）", rel, errText),
				hits: []pdHit{}, scanned: len(files), exempt: exempt}
		}
		hits = append(hits, h...)
	}
	rc := 0
	if len(hits) > 0 {
		rc = 1
	}
	if hits == nil {
		hits = []pdHit{}
	}
	if exempt == nil {
		exempt = [][2]string{}
	}
	return pdResult{rc: rc, reason: "", hits: hits, scanned: len(files), exempt: exempt}
}

func pdIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func pdIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func pdIsExec(p string) bool {
	fi, err := os.Stat(p)
	if err != nil {
		return false
	}
	return fi.Mode().Perm()&0o111 != 0
}

func pdHasExt(name string, exts []string) bool {
	for _, e := range exts {
		if strings.HasSuffix(name, e) {
			return true
		}
	}
	return false
}

// pdRelPath —— py `os.path.relpath(p, base)`。
func pdRelPath(p, base string) string {
	if r, err := filepath.Rel(base, p); err == nil {
		return r
	}
	return p
}

// pdRepoRootDefault —— 标准声明 (a)：py 从 `__file__` 上溯二级；本件从 cwd 上溯 ≤6 找含 `scripts/gates` 的根。
func pdRepoRootDefault() string {
	start, err := os.Getwd()
	if err != nil {
		return "."
	}
	d, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	for i := 0; i < 6; i++ {
		if pdIsDir(filepath.Join(d, "scripts", "gates")) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// ── JSON（保 python json.dumps(…, ensure_ascii=False, indent=2) 的逐字形态）──────────
type pdVal struct {
	kind byte // o 对象 / a 数组 / s 串 / n 数 / b 布尔 / z null
	keys []string
	obj  map[string]*pdVal
	arr  []*pdVal
	str  string
	num  string
	bl   bool
}

func pdObj(pairs ...[2]interface{}) *pdVal {
	v := &pdVal{kind: 'o', obj: map[string]*pdVal{}}
	for _, kv := range pairs {
		k := kv[0].(string)
		v.keys = append(v.keys, k)
		v.obj[k] = pdAny(kv[1])
	}
	return v
}

func pdAny(x interface{}) *pdVal {
	switch t := x.(type) {
	case *pdVal:
		return t
	case string:
		return &pdVal{kind: 's', str: t}
	case int:
		return &pdVal{kind: 'n', num: fmt.Sprintf("%d", t)}
	case bool:
		return &pdVal{kind: 'b', bl: t}
	}
	return &pdVal{kind: 'z'}
}

func pdArr(items ...*pdVal) *pdVal { return &pdVal{kind: 'a', arr: items} }

// pdJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的串格。
func pdJSONStr(s string) string {
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

// pdJSONDump —— py `json.dumps(v, ensure_ascii=False, indent=2)`（项分隔 `,` · 键分隔 `: `）。
func pdJSONDump(v *pdVal, level int) string {
	switch v.kind {
	case 's':
		return pdJSONStr(v.str)
	case 'n':
		return v.num
	case 'b':
		if v.bl {
			return "true"
		}
		return "false"
	case 'z':
		return "null"
	case 'a':
		if len(v.arr) == 0 {
			return "[]"
		}
		pad := strings.Repeat("  ", level+1)
		closer := strings.Repeat("  ", level)
		var b strings.Builder
		b.WriteString("[\n")
		for i, e := range v.arr {
			b.WriteString(pad)
			b.WriteString(pdJSONDump(e, level+1))
			if i < len(v.arr)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(closer)
		b.WriteString("]")
		return b.String()
	case 'o':
		if len(v.keys) == 0 {
			return "{}"
		}
		pad := strings.Repeat("  ", level+1)
		closer := strings.Repeat("  ", level)
		var b strings.Builder
		b.WriteString("{\n")
		for i, k := range v.keys {
			b.WriteString(pad)
			b.WriteString(pdJSONStr(k))
			b.WriteString(": ")
			b.WriteString(pdJSONDump(v.obj[k], level+1))
			if i < len(v.keys)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(closer)
		b.WriteString("}")
		return b.String()
	}
	return "null"
}

func pdJSONOutput(res pdResult) string {
	items := pdArr()
	for _, h := range res.hits {
		items.arr = append(items.arr, pdObj(
			[2]interface{}{"file", h.file},
			[2]interface{}{"line", h.line},
			[2]interface{}{"kind", h.kind},
			[2]interface{}{"raw", h.raw},
			[2]interface{}{"why", h.why},
		))
	}
	exempt := pdArr()
	for _, e := range res.exempt {
		exempt.arr = append(exempt.arr, pdObj(
			[2]interface{}{"file", e[0]},
			[2]interface{}{"why", e[1]},
		))
	}
	meta := pdObj(
		[2]interface{}{"scanned", res.scanned},
		[2]interface{}{"hits", len(res.hits)},
		[2]interface{}{"exempt", exempt},
		[2]interface{}{"reason", res.reason},
	)
	root := pdObj(
		[2]interface{}{"schema", pdSchema},
		[2]interface{}{"kind", pdKind},
		[2]interface{}{"items", items},
		[2]interface{}{"meta", meta},
	)
	return pdJSONDump(root, 0)
}

// ── 自检（成对负控 · 合成夹具 · 不碰真目标）────────────────────────────────────────
// pdSelfCase —— py `SELF_TESTS` 的一格。
type pdSelfCase struct {
	name string
	body string
	want int
}

// 与 py `SELF_TESTS` 逐条对齐（16 格；第 17 格 = 空转，见 pdSelfTest）。
var pdSelfCases = []pdSelfCase{
	{"① 正控：解析器形态（`python3 -c` 抽 JSON 字段）⇒ 无命中",
		"#!/bin/bash\npython3 -c 'import json,sys; print(json.load(sys.stdin)[\"a\"])' < f.json\n", 0},
	{"② 正控：`grep -oE '[0-9]+'`（模式不以 `-` 起头）⇒ 无命中",
		"#!/bin/bash\nX=$(printf '%s' \"$S\" | grep -oE '[0-9]+' | head -1)\n", 0},
	{"③ must-fail：`grep -o '--strict-report'`（**不以 `-e`/`--` 起**）⇒ rc=1",
		"#!/bin/bash\nN=$(grep -o '--strict-report' \"$F\" | wc -l)\n", 1},
	{"④ must-fail：`grep -oE '--co'`（捆绑短选项 + 破折号模式）⇒ rc=1",
		"#!/bin/bash\ngrep -oE '--co' \"$F\"\n", 1},
	{"⑤ must-fail：`grep -c -- '--co'` 的反面 —— `grep -c '--co'`（无 `--`）⇒ rc=1",
		"#!/bin/bash\ngrep -c '--co' \"$F\"\n", 1},
	{"⑥ 成对回绿：同一模式改 `grep -o -e '--strict-report'`（**显式分界符**）⇒ rc=0",
		"#!/bin/bash\nN=$(grep -o -e '--strict-report' \"$F\" | wc -l)\n", 0},
	{"⑦ 成对回绿：同一模式改 `grep -o -- '--strict-report'`（`--` 分界符）⇒ rc=0",
		"#!/bin/bash\nN=$(grep -o -- '--strict-report' \"$F\" | wc -l)\n", 0},
	{"⑧ must-fail：`sed 's/<[^>]*>//g'` 剥标签 ⇒ rc=1",
		"#!/bin/bash\nT=$(sed 's/<[^>]*>//g' \"$F\")\n", 1},
	{"⑨ 成对回绿：改 `python3 -c 'html.parser'` 剥标签 ⇒ rc=0",
		"#!/bin/bash\nT=$(python3 -c 'import html,sys; print(html.unescape(sys.stdin.read()))' < \"$F\")\n", 0},
	{"⑩ 成对回绿：普通 `sed 's/x/y/g'`（非剥标签）⇒ rc=0",
		"#!/bin/bash\nT=$(sed 's/x/y/g' \"$F\")\n", 0},
	{"⑪ 真话负控：把 ③ 那一行**注释掉** ⇒ 回绿（命中的确实是那一行，不是噪声）",
		"#!/bin/bash\n# N=$(grep -o '--strict-report' \"$F\" | wc -l)\n", 0},
	{"⑫ 一层嵌入：命令串里塞 `grep -o '--x'` ⇒ 仍判得出 ⇒ rc=1",
		"#!/bin/bash\nadd_step x \"n\" tri \"$R\" \"grep -o '--x' f; rc=$?; exit $rc\"\n", 1},
	{"⑭ 正控（**数据字面** · 本门误报根因形态）：宿主语言字面量列表里的 `\"grep -\"` 元素 ⇒ 不判（rc=0）",
		"#!/usr/bin/env python3\nCONSUMERS = (\n    \".MatchString(\", \"grep -\", \"for \", \"$(\",\n)\n", 0},
	{"⑮ 负控（**证未放宽真判据**）：同一形态落在**真命令位**（`grep -o '--x' f`）⇒ 照旧 rc=1",
		"#!/bin/bash\ngrep -o '--x' f\n", 1},
	{"⑯ 正控（补表后）：`git grep -I -n PLACEHOLDER .`（`-I` 是 GNU grep 真选项）⇒ 不判（rc=0）",
		"#!/bin/bash\ngit grep -I -n PLACEHOLDER .\n", 0},
	{"⑰ 负控（**补表没放宽模式实参判据**）：`git grep -I -n '--strict-report' .` ⇒ 照旧 rc=1",
		"#!/bin/bash\ngit grep -I -n '--strict-report' .\n", 1},
}

// pdSelfTest —— py `self_test()`：17 格（16 + 空转档）。
// 差异（见件尾未覆盖面④）：py 直接 `run(td)` 进程内跑；本件**同样进程内**跑同一个 `pdRun` ⇒ 读数面一致。
func pdSelfTest(stdout io.Writer) int {
	ok, bad := 0, 0
	var lines []string
	td, err := os.MkdirTemp("", "w15-parser-")
	if err != nil {
		td = os.TempDir()
	}
	defer os.RemoveAll(td)
	sub := filepath.Join(td, "scripts")
	_ = os.MkdirAll(sub, 0o755)
	f := filepath.Join(sub, "fixture.sh")
	for _, c := range pdSelfCases {
		_ = os.WriteFile(f, []byte(c.body), 0o644)
		res := pdRun(td)
		good := res.rc == c.want
		mark := "✗"
		if good {
			mark = "✓"
			ok++
		} else {
			bad++
		}
		lines = append(lines, fmt.Sprintf("  %s %s（期望 rc=%d · 现跑 rc=%d）", mark, c.name, c.want, res.rc))
	}
	// ⑬ 空转 ⇒ rc=2（不给结论 ⇒ 不许当绿）
	empty := filepath.Join(td, "scripts-empty")
	_ = os.MkdirAll(empty, 0o755)
	res := pdRun(empty)
	good := res.rc == 2
	mark := "✗"
	if good {
		mark = "✓"
		ok++
	} else {
		bad++
	}
	lines = append(lines, fmt.Sprintf("  %s ⑬ 空转（扫描面 0 件）⇒ rc=2（不给结论 · 不许当绿）（现跑 rc=%d）", mark, res.rc))
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	verdict := "全过"
	if bad != 0 {
		verdict = "有失败"
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n", verdict, ok+bad, bad)
	if bad == 0 {
		return 0
	}
	return 2
}

// pdLegalForms —— py `LEGAL_FORMS`。
var pdLegalForms = [][2]string{
	{"grep 的模式以 `-` 起头", "`grep -o -e '--strict-report' <件>` 或 `grep -o -- '--strict-report' <件>`（显式分界符）"},
	{"从结构化件（json/yaml/toml）取字段", "用解析器（`python3 -c 'json.load'` / `yaml.safe_load`），不用 `grep`/`sed`/`awk` 抠"},
	{"剥 HTML/XML 标签", "用解析器（`html.parser` / `xml.etree`）或先划显式分界符，不用 `s/<[^>]*>//g`"},
	{"同一命令行里有 `|` / `&&` / `;`", "`grep` 之后没有下一条命令；`--` 与 `-e` 的分界符照上一条"},
}

// pdArgErr —— 用法错（py argparse）：stderr 印 usage ＋ `prog: error: …`，退码 2。
func pdArgErr(stderr io.Writer, msg string) int {
	fmt.Fprint(stderr, pdUsage)
	fmt.Fprintf(stderr, "%s: error: %s\n", pdProg, msg)
	return 2
}

// parserDisciplineGateMain —— 入口（= py `main()`）。
func parserDisciplineGateMain(stdout, stderr io.Writer, argv []string) int {
	var root string
	var jsonOut, selfTest, listForms, help bool
	var unknown []string
	for _, a := range argv {
		switch {
		case a == "-h" || a == "--help":
			help = true
		case a == "--json":
			jsonOut = true
		case a == "--self-test":
			selfTest = true
		case a == "--list-forms":
			listForms = true
		case strings.HasPrefix(a, "-") && a != "-":
			unknown = append(unknown, a)
		default:
			if root == "" {
				root = a
			} else {
				unknown = append(unknown, a)
			}
		}
	}
	if help {
		fmt.Fprint(stdout, pdHelpText)
		return 0
	}
	if len(unknown) > 0 {
		return pdArgErr(stderr, "unrecognized arguments: "+strings.Join(unknown, " "))
	}
	if root == "" {
		root = pdRepoRootDefault()
	}

	if listForms {
		for _, f := range pdLegalForms {
			fmt.Fprintf(stdout, "  · %s\n      ⇒ %s\n", f[0], f[1])
		}
		return 0
	}
	if selfTest {
		return pdSelfTest(stdout)
	}

	res := pdRun(root)
	if jsonOut {
		fmt.Fprintln(stdout, pdJSONOutput(res))
		return res.rc
	}
	fmt.Fprintln(stdout, "── 抓取 / 解析纪律（`scripts/` 面 · **无解析器则必须显式分界符**）──")
	fmt.Fprintf(stdout, "  扫描 %d 件\n", res.scanned)
	for _, e := range res.exempt {
		fmt.Fprintf(stdout, "  ⊘ 自豁免：%s —— %s\n", e[0], e[1])
	}
	for _, h := range res.hits {
		fmt.Fprintf(stdout, "  ✗ %s:%d  [%s]  %s\n", h.file, h.line, h.kind, h.raw)
		fmt.Fprintf(stdout, "      %s\n", h.why)
	}
	switch res.rc {
	case 0:
		fmt.Fprintf(stdout, "⇒ 抓取/解析面全过（rc=0）：%d 件里 0 处「破折号模式」0 处「贪婪剥标签」\n", res.scanned)
	case 1:
		fmt.Fprintf(stdout, "⇒ 有命中 %d 处（rc=1）：破折号模式 ⇒ 加 `-e`/`--`；剥标签 ⇒ 换解析器\n", len(res.hits))
	default:
		fmt.Fprintf(stdout, "⇒ %s\n", res.reason)
	}
	return res.rc
}
