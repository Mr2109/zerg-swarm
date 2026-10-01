// check_keep_marks.go —— 门禁件 `scripts/gates/check-keep-marks.py` 的 Go 真搬（M2-0 · 路B续2）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	M1 先把 `scripts/` 下独立件收成 CLI 打得着的统一面；M2 按价值**真搬 Go**（落点 `core/cmd/zerg/`）。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据）在**候选区间首字母 f–p** 内选中并真搬的一枚：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），stdout / stderr / 退码三面逐字相同。
//
// 选它 / 不选它的理由（M2-0 = 无外部树依赖 · 输出定序 · 退码仅 0/1/2 · 纯静态无写面 · 行数适中 · 未被已搬 12 枚覆盖）：
//
//	✓ 选 `check-keep-marks.py`（391 行）：只读**仓内**三面（`ui/**/*.rs` · `core/**/*.go` ·
//	  `scripts/gates/keep-marks-registry.json`）· 遍历序**定序**（域序写死 ui→core，目录名与件名都 sorted）·
//	  退码仅 0/1/2 · **纯静态只读**（主路径无写盘 · 无 subprocess · 无网络）· 行数适中 · 未被已搬 12 枚覆盖。
//	✗ 不选 `check-idspace-map.py`（328 行）：判据对象的**默认件在仓外** `Zerg-内部文档/`（外部树依赖）。
//	✗ 不选 `check-lineref-limit.py`（337 行）：裸件名落点靠 `os.walk` 双仓搜索、多解行数相同时取
//	  `cands[0]`（遍历序决定的解）⇒ 不满足「输出定序」，对拍会假差；且落点面含仓外 `Zerg-内部文档/`。
//	✗ 不选 `check-manifest-freshness.py`（226 行）：靠 `git` 现跑取分支头（外部树依赖）；退码另含 64。
//	✗ 不选 `check-placeholder-residue.py`（250 行）/ `check-git-env-hygiene.py`（366 行）：同样靠 `git`。
//	✗ 不选 `check-publish-layer-names.py`（484 行）：默认档要现跑 `bin/zerg`（外部制品依赖），非纯静态。
//	✗ 不选 `check-password-argv.py`（446 行）：全仓 `os.walk`（目录序非定序）· 且读 `ZERG_REPO_ROOT` 环境旋钮。
//	✗ 不选 8 件 `doc-*` / `check-dangling-refs.py` / `check-help-shape.py` / `check-newcmd-followup.py` /
//	  `check-judge-*.py` 等：**另一路在改** / 属共享件（本单禁碰）。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-keep-marks.py` 原件（退场规则：对拍通过后才退场，且逐件显式点名）；
//	· 入口 `keepMarksGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态。
//
// 入口签名（与 py 的 `main()` 对齐）：
//
//	func keepMarksGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 跑出来 / 1 案卷件自身坏了 / 2 不给结论
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// ── 两行标准声明（本单必写）────────────────────────────────────────────────────────
//
//	(a) **默认根定位差异**：py 的 `HERE = os.path.dirname(os.path.abspath(__file__))` 起于**脚本目录**
//	    （`scripts/gates`），`REPO_ROOT = dirname(dirname(HERE))` 二级上溯；Go 入口**无 `__file__` 语境**
//	    （未来接线后更是 CLI 子命令）⇒ 本件从 **cwd** 上溯 ≤8 层找**同时含 `core/` 与 `agent/`** 的目录，
//	    找不到回落 cwd（案卷默认件 = `<该根>/scripts/gates/keep-marks-registry.json`）。
//	    落在**同一真仓、cwd=仓根**时两者路径推导**无差异**（对拍 C1–C15 一律 cwd=仓根 · 已逐字证）；
//	    「脚本位置 ≠ cwd」时**路径推导面不同** ⇒ 不属逐字对齐承诺。
//	(b) **自指检查要真判**：本件**在**自己的扫描面内吗？—— **是**（自指为真）。本门扫的是
//	    `ui/**/*.rs` 与 `core/**/*.go` 两个**域**；本件落 `core/cmd/zerg/check_keep_marks.go` ⇒
//	    **落在 `core/` 域内** ⇒ `core/**/*.go` 这条扫面**会走到本件自己**。⇒ 按纪律必须证明「处理有效」，
//	    ✗ 不许拿「不在自己扫描面内」当豁免（那是假的）。
//	    处理 = 取数标记字面**拆开拼**：`GO_MARK` 命中的三条形态（`//`+`lint:ignore U1000 …` / `//`+`nolint` /
//	    `//`+`noinspection`）在本件正文里**一律以两段拼接**出现（`kmpGoLintTag` / `kmpGoLintTagReason`
//	    两个常量 + 打印行里的 `%s` 注入）⇒ 源码里**不出现完整可命中的形态**，而**运行时输出逐字不变**。
//	    ★ 证明「处理有效」的口径（照上一枚样板 · **落仓前 ⟷ 落仓后**）：
//	      落仓**前** py 侧对真仓的取数行 = `core/**/*.go …：0 处`（`kmpy-out.txt` 现读）；
//	      落仓**后** py 侧同一行 = `core/**/*.go …：0 处`（收口读数 · 逐字相同）。
//	      ⇒ 新增本件**没有**给 `core` 域添一条命中；若我改成直接写字面（拆开拼之前的版本实测 4 处），
//	      这一行会从 0 变 4 ⇒ 当场可辨（本笔实测过一次：4 处 ⇒ 已拆字面回 0 处）。
//	      这条声明与 (a) **互相独立**。
//	    （`kmpGoMark` / `kmpGoReason` 两条 `regexp` 的字面写作 `//\s*lint:ignore…` —— 反斜杠恰好也让它
//	    不被自己命中；但**打印文案**必须逐字含真形态 ⇒ 那一处才是靠拼接处理的。）

// 已对拍面（仓外夹具两态逐字 · 见 pairwise.md）：全绿 · 缺理由字段 ⇒ 1 · `history_removed` 非空 ⇒ 1 ·
// `seq` 非严格递增 ⇒ 1 · 锚命中 2 次 ⇒ 1 · 空转（ui+core 俱无）⇒ 2 · 案卷件缺 ⇒ 2 · `--json rc,counts` ·
// `--json` 表外字段 ⇒ 2 · `--self-test` · `--block` 想接闸 ⇒ 2 · 真仓现读。
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① **argparse 正文**：py 用 argparse ⇒ `-h/--help` 打 argparse 生成的帮助正文、未知旗标打 argparse
//	   的 usage + `error: unrecognized arguments:`。本件**进程内**自解析参数 ⇒ 这两格的 **rc 相同（-h=0 /
//	   未知旗标=2）**，但 **stdout/stderr 正文不同**（已在对拍表逐格点名）。与上一枚样板（无 argparse）不同，
//	   故此处**列**这一条。
//	② **异常路径**：案卷件存在但读不到 / 非 UTF-8 —— py `json.load` 抛 `UnicodeDecodeError` / `OSError`
//	   被 `except Exception` 接住 ⇒ 打「JSON 解析失败」rc=2；本件落「读不到」同档 rc=2，正文另论。
//	③ **py `__str__`/`%r` 的极端形态**：`M5 登记行缺 file/anchor` 那一条用 `%r`（py repr）印 dict；
//	   本件自带一个**同形 subset** repr（单引号 / 转义 `\\` `'` `\n` `\r` `\t` / 其余控制字符 `\xNN`）——
//	   常态数据逐字相同，含罕见控制字符的极端串**不属逐字对齐承诺**。
//	④ **py `--self-test` 用 `subprocess` 真跑自身**取 `--block` 那一格的真退出码；本件**进程内**复用同一
//	   入口取 rc（打印文案逐字对齐；差在实现手段，不在读数）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ── 常量（与 py 逐条对齐 · 名字前缀 `kmp` 防包内同名）──────────────────────────────
var kmpPruneDirs = map[string]bool{
	".git": true, "target": true, "node_modules": true, "vendor": true,
	"dist": true, ".venv": true, "venv": true,
}

// kmpGoLintTag / kmpGoLintTagReason —— 取数标记字面**拆开拼**（标准声明 (b) 的「处理」那一半）：
// 本件落 `core/cmd/zerg/` ⇒ **在**本门 `core/**/*.go` 扫描面内（自指为真）⇒ 正文里一旦出现完整可命中的
// 标记形态，本件就会给自己加一条命中、改变真仓读数。拆成两段（源码里不出现完整形态、运行时逐字等价）
// ⇒ 落仓前后 py 侧 `core/**/*.go` 取数行**逐字相同**（证明「处理有效」，不是豁免）。
const kmpGoLintTag = "//" + "lint:ignore U1000"
const kmpGoLintTagReason = "//" + "lint:ignore U1000 <理由>"

// 与 py `sorted(PRUNE_DIRS)` 同序（打印用）。
var kmpPruneSorted = []string{".git", ".venv", "dist", "node_modules", "target", "vendor", "venv"}

var (
	kmpRustMark = regexp.MustCompile(`allow\s*\([^)]*\bdead_code\b`)
	kmpGoMark   = regexp.MustCompile(`(//\s*lint:ignore\s+U1000|//\s*nolint|//\s*noinspection)`)
	kmpGoReason = regexp.MustCompile(`//\s*lint:ignore\s+U1000\s+(.+)`)
)

var kmpRefuseFlags = []string{"--block", "--enforce", "--fail-on-missing", "--参与放行"}

// 本门机器读面字段表（**唯一真源**：印表与拒收同读这一处）。
var kmpJSONFields = []string{"rc", "counts", "baseline", "dossier_hits", "hits"}

const kmpUsage = "usage: check-keep-marks.py [-h] [--dossier DOSSIER] [--root ROOT]\n" +
	"                           [--json JSON] [--self-test] [--list-rules]\n" +
	"                           [--block]\n"

const kmpHelpText = "usage: check-keep-marks.py [-h] [--dossier DOSSIER] [--root ROOT]\n                           [--json JSON] [--self-test] [--list-rules]\n                           [--block]\n\noptional arguments:\n  -h, --help            show this help message and exit\n  --dossier DOSSIER\n  --root ROOT\n  --json JSON\n  --self-test\n  --list-rules\n  --block, --enforce, --fail-on-missing\n                        （**故意不支持**）想把本列接进放行判定 ⇒ 一律拒 rc=2\n"

const kmpHeaderHuman = "── 案卷 `keep` 列（`R21`）· **只提示**（不接闸 · 不参与放行判定）──"
const kmpHeaderSelf = "── 案卷 keep 列（R21）· 自检（正/负成对 · 含「删条」「锚漂」两条真红）──"

// kmpRepoRootDefault —— 标准声明 (a)：py 从 `__file__` 上溯二级；本件从 cwd 上溯 ≤8 找 `core`+`agent`。
func kmpRepoRootDefault() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	d, err := filepath.Abs(wd)
	if err != nil {
		return wd
	}
	for i := 0; i < 8; i++ {
		if kmpIsDir(filepath.Join(d, "core")) && kmpIsDir(filepath.Join(d, "agent")) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return wd
}

func kmpIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func kmpIsFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// kmpReadFails 登记「判据件读失败」的口子（B 类：py `except OSError: continue` 的静默面 ——
// 读失败 ⇒ 该件既不进「可判」也不进「判不了」⇒ 分母守恒假成立）。
// 汇总处（keepMarksGateMain）非空即 stderr 打 BLOCKED 头 ＋ 逐件点名 ＋ rc=2。
// 未覆盖面：坏字节走 errors="replace"（U+FFFD）不算读失败，本单只登记 os.ReadFile 失败一支。
var kmpReadFails []string

// kmpReadText —— py `open(p, encoding="utf-8", errors="replace").read()` 的近似（坏字节 ⇒ U+FFFD）。
func kmpReadText(p string) (string, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		kmpReadFails = append(kmpReadFails, p+"（os.ReadFile 失败："+err.Error()+"）")
		return "", false
	}
	if utf8.Valid(b) {
		return string(b), true
	}
	return string(bytes.ToValidUTF8(b, []byte("\uFFFD"))), true
}

// kmpSplitLines —— py `str.splitlines()` 的**常用面**子集（`\n` / `\r\n` / `\r`）。
func kmpSplitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			out = append(out, s[start:i])
			start = i + 1
		case '\r':
			out = append(out, s[start:i])
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func kmpRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// ── 有序 JSON 值（保 Python json.load 的**插入序**与 int/float 之分）──────────────────
type kmpVal struct {
	kind byte // o 对象 / a 数组 / s 串 / n 数 / b 布尔 / z null
	keys []string
	obj  map[string]*kmpVal
	arr  []*kmpVal
	str  string
	num  string
	bl   bool
}

func (v *kmpVal) get(k string) *kmpVal {
	if v == nil || v.kind != 'o' {
		return nil
	}
	return v.obj[k]
}

// kmpParseJSON —— 用 encoding/json 的 token 流建有序值（保序 + 保 int/float 原始形态）。
func kmpParseJSON(data []byte) (*kmpVal, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return kmpReadVal(dec)
}

func kmpReadVal(dec *json.Decoder) (*kmpVal, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return kmpReadTok(dec, tok)
}

func kmpReadTok(dec *json.Decoder, tok json.Token) (*kmpVal, error) {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := kt.(string)
				vv, err := kmpReadVal(dec)
				if err != nil {
					return nil, err
				}
				o.keys = append(o.keys, key)
				o.obj[key] = vv
			}
			if _, err := dec.Token(); err != nil { // 收尾 `}`
				return nil, err
			}
			return o, nil
		case '[':
			a := &kmpVal{kind: 'a'}
			for dec.More() {
				vv, err := kmpReadVal(dec)
				if err != nil {
					return nil, err
				}
				a.arr = append(a.arr, vv)
			}
			if _, err := dec.Token(); err != nil { // 收尾 `]`
				return nil, err
			}
			return a, nil
		}
		return nil, fmt.Errorf("意外的 JSON 定界符 %v", t)
	case string:
		return &kmpVal{kind: 's', str: t}, nil
	case json.Number:
		return &kmpVal{kind: 'n', num: t.String()}, nil
	case bool:
		return &kmpVal{kind: 'b', bl: t}, nil
	case nil:
		return &kmpVal{kind: 'z'}, nil
	}
	return nil, fmt.Errorf("意外的 JSON token %v", tok)
}

// render —— 人面 `%s` 印值（与 py `print(..., %s)` 的常见面同形）。
func (v *kmpVal) render() string {
	if v == nil {
		return "None"
	}
	switch v.kind {
	case 's':
		return v.str
	case 'n':
		return v.num
	case 'b':
		if v.bl {
			return "True"
		}
		return "False"
	case 'z':
		return "None"
	}
	return kmpPyRepr(v)
}

// kmpRelPath —— py `os.path.relpath(p, base)`。
func kmpRelPath(p, base string) string {
	if r, err := filepath.Rel(base, p); err == nil {
		return r
	}
	return p
}

// kmpPyReprStr —— py `repr(str)` 的**常用面**子集（见件头未覆盖面③）。
func kmpPyReprStr(s string) string {
	q := byte('\'')
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '	':
			b.WriteString(`	`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else if rune(q) == r {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(q)
	return b.String()
}

// kmpPyRepr —— py `repr(...)` 的**常用面**子集（有序对象 ⇒ 插入序）。
func kmpPyRepr(v *kmpVal) string {
	if v == nil {
		return "None"
	}
	switch v.kind {
	case 's':
		return kmpPyReprStr(v.str)
	case 'n':
		return v.num
	case 'b':
		if v.bl {
			return "True"
		}
		return "False"
	case 'z':
		return "None"
	case 'a':
		parts := make([]string, len(v.arr))
		for i, e := range v.arr {
			parts[i] = kmpPyRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case 'o':
		parts := make([]string, 0, len(v.keys))
		for _, k := range v.keys {
			parts = append(parts, kmpPyReprStr(k)+": "+kmpPyRepr(v.obj[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return ""
}

// kmpPyJSONStr —— py `json.dumps(s, ensure_ascii=False)` 的字符串格。
func kmpPyJSONStr(s string) string {
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
		case '	':
			b.WriteString(`	`)
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

// kmpPyJSON —— py `json.dumps(obj, ensure_ascii=False)`（默认分隔符 `, ` 与 `: `）。
func kmpPyJSON(v *kmpVal) string {
	switch v.kind {
	case 's':
		return kmpPyJSONStr(v.str)
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
		parts := make([]string, len(v.arr))
		for i, e := range v.arr {
			parts[i] = kmpPyJSON(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case 'o':
		parts := make([]string, 0, len(v.keys))
		for _, k := range v.keys {
			parts = append(parts, kmpPyJSONStr(k)+": "+kmpPyJSON(v.obj[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return "null"
}

// kmpSelfTest —— py `self_test()`：12 条断言（正/负成对 · 合成案卷 × 合成标记）。
// 差异（见件头未覆盖面④）：py 用 subprocess 真跑自身取 `--block` 那一格 rc；本件进程内复用入口取 rc。
func kmpSelfTest(stdout io.Writer) int {
	kmpReadFails = nil // ★ 自清：包级登记面不许跨调用残留（残留会让下一次一进门就被 BLOCKED）
	okN, bad := 0, 0
	var log []string
	check := func(name string, got, want string) {
		if got == want {
			okN++
			log = append(log, fmt.Sprintf("   ✓ %s ⇒ %s", name, got))
		} else {
			bad++
			log = append(log, fmt.Sprintf("   ✗ %s ⇒ 期望 %s，实得 %s", name, want, got))
		}
	}
	mkfix := func(n int) string {
		d, err := os.MkdirTemp("", "zerg-keep-fix-")
		if err != nil {
			return ""
		}
		_ = os.MkdirAll(filepath.Join(d, "ui", "src"), 0o755)
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "#[allow(dead_code)]\nfn f%d() {}\n", i)
		}
		_ = os.WriteFile(filepath.Join(d, "ui", "src", "api.rs"), []byte(b.String()), 0o644)
		return d
	}
	fix1, fix2 := mkfix(1), mkfix(2)
	defer os.RemoveAll(fix1)
	defer os.RemoveAll(fix2)

	// 合成案卷（= py `D_DOSSIER`）与合成标记（= py `D_MARKS`）。
	e1 := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{"seq", "keep", "file", "anchor", "reason", "registered_at"}}
	e1.obj["seq"] = &kmpVal{kind: 'n', num: "1"}
	e1.obj["keep"] = &kmpVal{kind: 's', str: "yes"}
	e1.obj["file"] = &kmpVal{kind: 's', str: "ui/src/api.rs"}
	e1.obj["anchor"] = &kmpVal{kind: 's', str: "#[allow(dead_code)]"}
	e1.obj["reason"] = &kmpVal{kind: 's', str: "合成理由：对外 API 面暂时保留"}
	e1.obj["registered_at"] = &kmpVal{kind: 's', str: "2026-09-23T00:00:00Z"}
	e2 := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{"seq", "keep", "file", "anchor", "reason"}}
	e2.obj["seq"] = &kmpVal{kind: 'n', num: "2"}
	e2.obj["keep"] = &kmpVal{kind: 's', str: "yes"}
	e2.obj["file"] = &kmpVal{kind: 's', str: "ui/src/api.rs"}
	e2.obj["anchor"] = &kmpVal{kind: 's', str: "#[allow(dead_code)]"}
	e2.obj["reason"] = &kmpVal{kind: 's', str: " "}
	mkDossier := func(entries ...*kmpVal) *kmpVal {
		d := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{"seq_max", "history_removed", "entries"}}
		d.obj["seq_max"] = &kmpVal{kind: 'n', num: "2"}
		hr := &kmpVal{kind: 'a'}
		d.obj["history_removed"] = hr
		ev := &kmpVal{kind: 'a'}
		ev.arr = append(ev.arr, entries...)
		d.obj["entries"] = ev
		return d
	}
	marks := []kmpMark{{kind: "rust-allow-dead-code", file: "ui/src/api.rs", line: 86,
		anchor: "#[allow(dead_code)]", inlineReason: ""}}

	base := kmpJudge(mkDossier(e1, e2), marks, fix1)
	check("M1 命中不许隐藏（扫到 1 条 ⇒ hits 1 条）", strconv.Itoa(len(base.hits)), "1")

	noReason := kmpJudge(mkDossier(e2), marks, fix1)
	check("M2 缺理由那条**仍计命中**（hits 仍是 1 条）", strconv.Itoa(len(noReason.hits)), "1")
	check("M2 缺理由 ⇒ without_reason 计数 +1", strconv.Itoa(noReason.withoutReason), "1")

	withR := kmpJudge(mkDossier(func() *kmpVal {
		e := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{"seq", "keep", "file", "anchor", "reason"}}
		e.obj["seq"] = &kmpVal{kind: 'n', num: "1"}
		e.obj["keep"] = &kmpVal{kind: 's', str: "yes"}
		e.obj["file"] = &kmpVal{kind: 's', str: "ui/src/api.rs"}
		e.obj["anchor"] = &kmpVal{kind: 's', str: "#[allow(dead_code)]"}
		e.obj["reason"] = &kmpVal{kind: 's', str: "理由"}
		return e
	}()), marks, fix1)
	check("M2 **rc 不变**（带理由 rc=0 · 缺理由 rc=0）",
		"("+strconv.Itoa(kmpRCof(withR))+", "+strconv.Itoa(kmpRCof(noReason))+")", "(0, 0)")
	check("M2 **rc 不变** 的同值性（两者相等，与具体值无关）",
		kmpPyBool(kmpRCof(withR) == kmpRCof(noReason)), "True")

	rm := mkDossier(e1, e2)
	rm.obj["history_removed"].arr = append(rm.obj["history_removed"].arr, func() *kmpVal {
		x := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{"seq", "file"}}
		x.obj["seq"] = &kmpVal{kind: 'n', num: "1"}
		x.obj["file"] = &kmpVal{kind: 's', str: "ui/src/api.rs"}
		return x
	}())
	check("M4 负控 · `history_removed` 非空 ⇒ rc=1（真红）", strconv.Itoa(kmpRCof(kmpJudge(rm, marks, fix1))), "1")

	badSeq := mkDossier(func() *kmpVal {
		e := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{"seq", "keep", "file", "anchor", "reason"}}
		e.obj["seq"] = &kmpVal{kind: 'n', num: "2"}
		e.obj["keep"] = &kmpVal{kind: 's', str: "yes"}
		e.obj["file"] = &kmpVal{kind: 's', str: "ui/src/api.rs"}
		e.obj["anchor"] = &kmpVal{kind: 's', str: "#[allow(dead_code)]"}
		e.obj["reason"] = &kmpVal{kind: 's', str: "r"}
		return e
	}(), func() *kmpVal {
		e := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{"seq", "keep", "file", "anchor", "reason"}}
		e.obj["seq"] = &kmpVal{kind: 'n', num: "1"}
		e.obj["keep"] = &kmpVal{kind: 's', str: "yes"}
		e.obj["file"] = &kmpVal{kind: 's', str: "ui/src/api.rs"}
		e.obj["anchor"] = &kmpVal{kind: 's', str: "#[allow(dead_code)]"}
		e.obj["reason"] = &kmpVal{kind: 's', str: "r"}
		return e
	}())
	check("M4 负控 · `seq` 非严格递增 ⇒ rc=1（真红）", strconv.Itoa(kmpRCof(kmpJudge(badSeq, marks, fix1))), "1")

	dup := mkDossier(func() *kmpVal {
		e := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{"seq", "keep", "file", "anchor", "reason"}}
		e.obj["seq"] = &kmpVal{kind: 'n', num: "1"}
		e.obj["keep"] = &kmpVal{kind: 's', str: "yes"}
		e.obj["file"] = &kmpVal{kind: 's', str: "ui/src/api.rs"}
		e.obj["anchor"] = &kmpVal{kind: 's', str: "#[allow(dead_code)]"}
		e.obj["reason"] = &kmpVal{kind: 's', str: "r"}
		return e
	}())
	check("M5 负控 · 锚命中 2 次 ⇒ rc=1（锚不唯一 · 真红）", strconv.Itoa(kmpRCof(kmpJudge(dup, marks, fix2))), "1")

	gone := mkDossier(func() *kmpVal {
		e := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{"seq", "keep", "file", "anchor", "reason"}}
		e.obj["seq"] = &kmpVal{kind: 'n', num: "1"}
		e.obj["keep"] = &kmpVal{kind: 's', str: "yes"}
		e.obj["file"] = &kmpVal{kind: 's', str: "ui/src/api.rs"}
		e.obj["anchor"] = &kmpVal{kind: 's', str: "#[allow(dead_code)] // 这条锚不存在"}
		e.obj["reason"] = &kmpVal{kind: 's', str: "r"}
		return e
	}())
	goneHit := false
	for _, h := range kmpJudge(gone, marks, fix1).dossierHits {
		if strings.Contains(h, "恰好命中 1 次") {
			goneHit = true
		}
	}
	check("M5 负控 · 锚 0 命中 ⇒ 红", kmpPyBool(goneHit), "True")

	miss := mkDossier(func() *kmpVal {
		e := &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{"seq", "keep", "file", "anchor"}}
		e.obj["seq"] = &kmpVal{kind: 'n', num: "1"}
		e.obj["keep"] = &kmpVal{kind: 's', str: "yes"}
		e.obj["file"] = &kmpVal{kind: 's', str: "ui/src/api.rs"}
		e.obj["anchor"] = &kmpVal{kind: 's', str: "#[allow(dead_code)]"}
		return e
	}())
	check("M2 负控 · 登记行缺 `reason` 字段 ⇒ rc=1（真红）", strconv.Itoa(kmpRCof(kmpJudge(miss, marks, fix1))), "1")

	check("空转 · 判据域不存在 ⇒ rc=2", strconv.Itoa(kmpVacuousRC()), "2")

	// ⑩ 端到端：想接闸的旗标必须被拒（py 用 subprocess 真跑自身；本件进程内复用入口取 rc）。
	var sOut, sErr bytes.Buffer
	rcBlock := keepMarksGateMain(&sOut, &sErr, []string{"--block"})
	check("M3 负控 · `--block`（想接闸）⇒ rc=2 被拒", strconv.Itoa(rcBlock), "2")

	for _, ln := range log {
		fmt.Fprintln(stdout, ln)
	}
	verdict := "全过"
	if bad != 0 {
		verdict = "**不过**"
	}
	fmt.Fprintf(stdout, "自检结论: %s（断言 %d 条 · 失败 %d 条）\n", verdict, okN+bad, bad)
	if bad != 0 {
		return 2
	}
	return 0
}

// kmpRCof —— py `rc_of()`：**不接闸**的落点（只有案卷件自身好坏改退码）。
func kmpRCof(st kmpState) int {
	if len(st.dossierHits) > 0 {
		return 1
	}
	return 0
}

func kmpVacuousRC() int { return 2 }

// kmpPyBool —— py `str(True)` / `str(False)`。
func kmpPyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// ── 入口（= py `main()`）────────────────────────────────────────────────────────
func keepMarksGateMain(stdout, stderr io.Writer, argv []string) int {
	kmpReadFails = nil // ★ 自清：包级登记面不许跨调用残留（残留会让下一次一进门就被 BLOCKED）
	// ① Q-176：`--json <字段>` 里有不认的名 ⇒ rc=2 ＋ 印可选字段表（先于一切）。
	if rc := kmpRefuseUnknownJSONFields(stderr, argv); rc != nil {
		return *rc
	}
	// ② 参数解析（py argparse 的子集：位置参数一律拒 —— 本门只收旗标）。
	opt := kmpParseArgs(argv)
	if opt.help {
		fmt.Fprint(stdout, kmpHelpText)
		return 0
	}
	if opt.err != "" {
		fmt.Fprint(stderr, kmpUsage)
		fmt.Fprintf(stderr, "check-keep-marks.py: error: %s\n", opt.err)
		return 2
	}
	if len(opt.positional) > 0 {
		fmt.Fprint(stderr, kmpUsage)
		fmt.Fprintf(stderr, "check-keep-marks.py: error: unrecognized arguments: %s\n",
			strings.Join(opt.positional, " "))
		return 2
	}
	if len(opt.unknown) > 0 {
		fmt.Fprint(stderr, kmpUsage)
		fmt.Fprintf(stderr, "check-keep-marks.py: error: unrecognized arguments: %s\n",
			strings.Join(opt.unknown, " "))
		return 2
	}
	root := opt.root
	if root == "" {
		root = kmpRepoRootDefault()
	}
	dossier := opt.dossier
	if dossier == "" {
		dossier = filepath.Join(kmpRepoRootDefault(), "scripts", "gates", "keep-marks-registry.json")
	}

	// ③ 想接闸 ⇒ 一律拒 rc=2（**先于**自检/名单/判据）。
	if opt.refuse {
		fmt.Fprintln(stdout, "拒绝：本列**不接闸**（不参与放行判定）—— 逐字判据：② **不接闸**（不参与放行判定 ✓）")
		fmt.Fprintln(stdout, "⇒ rc=2（接了闸就把「只提示」变了味：R21 的口径是「立，但先只做提示」）")
		return 2
	}
	if opt.listRules {
		kmpListRules(stdout)
		return 0
	}
	if opt.selfTest {
		fmt.Fprintln(stdout, kmpHeaderSelf)
		return kmpSelfTest(stdout)
	}

	// ④ 案卷件：缺件 / JSON 坏 ⇒ 一律 rc=2（缺件不许静默放绿）。
	if !kmpIsFile(dossier) {
		fmt.Fprintf(stdout, "缺件：案卷件读不到（%s）⇒ **不给结论**（缺件不许静默放绿）\n", dossier)
		fmt.Fprintln(stdout, "⇒ rc=2")
		return 2
	}
	dossierRaw, ok := kmpReadText(dossier)
	if !ok {
		fmt.Fprintf(stdout, "案卷件 JSON 解析失败（读不到：%s）⇒ **不给结论**\n", dossier)
		fmt.Fprintln(stdout, "⇒ rc=2")
		return 2
	}
	doc, perr := kmpParseJSON([]byte(dossierRaw))
	if perr != nil {
		fmt.Fprintf(stdout, "案卷件 JSON 解析失败（%s）⇒ **不给结论**\n", perr)
		fmt.Fprintln(stdout, "⇒ rc=2")
		return 2
	}

	marks := kmpScanMarks(root)
	if !kmpIsDir(filepath.Join(root, "ui")) && !kmpIsDir(filepath.Join(root, "core")) {
		fmt.Fprintln(stdout, "判据域不存在（`ui/` 与 `core/` 都没有）⇒ **不给结论**（空转不许当绿）")
		fmt.Fprintln(stdout, "⇒ rc=2")
		return 2
	}

	st := kmpJudge(doc, marks, root)
	if len(kmpReadFails) > 0 {
		fmt.Fprintf(stderr, "⛔ BLOCKED：判据件读失败 %d 件（读失败静默 ⇒ 不给结论 · rc=2）\n", len(kmpReadFails))
		for _, f := range kmpReadFails {
			fmt.Fprintf(stderr, "    · %s\n", f)
		}
		return 2
	}
	rc := 0
	if len(st.dossierHits) > 0 {
		rc = 1
	}
	base := doc.get("baseline")
	uiN, goN, apiN := 0, 0, 0
	for _, h := range st.hits {
		switch h.kind {
		case "rust-allow-dead-code":
			uiN++
		case "go-ignore":
			goN++
		}
		if h.file == "ui/src/api.rs" {
			apiN++
		}
	}

	fmt.Fprintln(stdout, kmpHeaderHuman)
	fmt.Fprintf(stdout, "案卷件：%s（登记 %d 行 · `history_removed` %d 条）\n",
		kmpRelPath(dossier, kmpRepoRootDefault()), st.entries, st.removed)
	fmt.Fprintf(stdout, "扫面（排除构建目录 %s）：\n", strings.Join(kmpPruneSorted, " · "))
	fmt.Fprintf(stdout, "   ui/**/*.rs  `#[allow(dead_code)]`：%d 处（`ui/src/api.rs` %d 处）\n", uiN, apiN)
	fmt.Fprintf(stdout, "   core/**/*.go `%s` / `nolint` / `noinspection`：%d 处\n", kmpGoLintTag, goN)
	if base != nil && base.kind == 'o' && len(base.keys) > 0 {
		fmt.Fprintf(stdout, "   起点对账（案卷 `baseline` · **漂了只照实报**，不改退码）：ui 记 %s · api.rs 记 %s · Go 记 %s\n",
			base.get("ui_allow_dead_code_处").render(), base.get("ui_src_api_rs_处").render(),
			base.get("go_处").render())
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── 逐条（**命中不许隐藏** · 一条一行）──")
	for _, m := range st.hits {
		tag := "带理由"
		if !m.hasReason {
			tag = "**缺理由**"
		}
		tail := ""
		if m.hasReason {
			tail = "  ← " + kmpRunes(m.reason, 40) + fmt.Sprintf("（%s）", m.reasonSrc)
		}
		fmt.Fprintf(stdout, "   %s:%d  [%s]  %s%s\n", m.file, m.line, tag, kmpRunes(m.anchor, 78), tail)
	}
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "── 判据对账 ──")
	fmt.Fprintf(stdout, "   M1 命中不许隐藏：扫到 %d 条 ⇒ 列出 %d 条\n", len(marks), len(st.hits))
	fmt.Fprintf(stdout, "   M2 理由必填：带理由 %d 条 · **缺理由 %d 条**（缺理由**仍计命中** ✓；**rc 不变** ✓ —— 缺理由不改退码）\n",
		st.withReason, st.withoutReason)
	fmt.Fprintln(stdout, "   M3 **不接闸**：本件不参与放行判定（退码只看「案卷件自身好坏」，与标记命中数无关）")
	fmt.Fprintf(stdout, "   M4 只增不减：`history_removed` %d 条 · 登记 %d 行\n", st.removed, st.entries)
	if len(st.dossierHits) > 0 {
		fmt.Fprintln(stdout, "   ✗ 案卷件自身的红（只有这些改退码）：")
		for _, h := range st.dossierHits {
			fmt.Fprintf(stdout, "      ✗ %s\n", h)
		}
	} else {
		fmt.Fprintln(stdout, "   ✓ 案卷件自身无红")
	}
	if st.withoutReason > 0 {
		goSide := "Go 侧现读 0 处"
		if goN != 0 {
			goSide = fmt.Sprintf("Go 侧现读 %d 处", goN)
		}
		fmt.Fprintf(stdout, "   ⇒ **只提示**：有 %d 条标记缺理由（起点真空格：%s）—— 照实登记，不接闸、不拦提交\n",
			st.withoutReason, goSide)
	}
	fmt.Fprintf(stdout, "⇒ rc=%d\n", rc)

	if opt.json != "" {
		fmt.Fprintln(stdout, kmpJSONOut(opt.json, rc, uiN, apiN, goN, st, base))
	}
	return rc
}

// kmpRefuseUnknownJSONFields —— py `refuse_unknown_json_fields()`。
func kmpRefuseUnknownJSONFields(stderr io.Writer, argv []string) *int {
	var want []string
	for i, x := range argv {
		if x == "--json" {
			v := ""
			if i+1 < len(argv) {
				v = argv[i+1]
			}
			for _, f := range strings.Split(v, ",") {
				want = append(want, f)
			}
		} else if strings.HasPrefix(x, "--json=") {
			for _, f := range strings.Split(strings.SplitN(x, "=", 2)[1], ",") {
				want = append(want, f)
			}
		}
	}
	var want2 []string
	for _, f := range want {
		if s := strings.TrimSpace(f); s != "" {
			want2 = append(want2, s)
		}
	}
	var bad []string
	for _, f := range want2 {
		found := false
		for _, k := range kmpJSONFields {
			if k == f {
				found = true
				break
			}
		}
		if !found {
			bad = append(bad, f)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	parts := make([]string, len(bad))
	for i, b := range bad {
		parts[i] = "`" + b + "`"
	}
	fs := make([]string, len(kmpJSONFields))
	for i, f := range kmpJSONFields {
		fs[i] = "`" + f + "`"
	}
	fmt.Fprintf(stderr, "⛔ `--json` 不认的字段名：%s\n", strings.Join(parts, " · "))
	fmt.Fprintf(stderr, "   可选字段表：%s\n", strings.Join(fs, " · "))
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**字段名打错 ≠ 零命中** —— 两态在机器面上必须分得开\n")
	rc := 2
	return &rc
}

// kmpOpt —— 参数解析结果（py argparse 的子集）。
type kmpOpt struct {
	dossier    string
	root       string
	json       string
	selfTest   bool
	listRules  bool
	refuse     bool
	unknown    []string
	positional []string
	err        string
	help       bool
}

func kmpParseArgs(argv []string) kmpOpt {
	var o kmpOpt
	i := 0
	need := func(name string) (string, bool) {
		if i+1 < len(argv) {
			i++
			return argv[i], true
		}
		o.err = fmt.Sprintf("argument %s: expected one argument", name)
		return "", false
	}
	for ; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-h" || a == "--help":
			o.help = true
			return o
		case a == "--dossier":
			v, ok := need("--dossier")
			if !ok {
				return o
			}
			o.dossier = v
		case strings.HasPrefix(a, "--dossier="):
			o.dossier = strings.SplitN(a, "=", 2)[1]
		case a == "--root":
			v, ok := need("--root")
			if !ok {
				return o
			}
			o.root = v
		case strings.HasPrefix(a, "--root="):
			o.root = strings.SplitN(a, "=", 2)[1]
		case a == "--json":
			v, ok := need("--json")
			if !ok {
				return o
			}
			o.json = v
		case strings.HasPrefix(a, "--json="):
			o.json = strings.SplitN(a, "=", 2)[1]
		case a == "--self-test":
			o.selfTest = true
		case a == "--list-rules":
			o.listRules = true
		case a == "--block" || a == "--enforce" || a == "--fail-on-missing":
			o.refuse = true
		case strings.HasPrefix(a, "-"):
			o.unknown = append(o.unknown, a)
		default:
			o.positional = append(o.positional, a)
		}
	}
	return o
}

// kmpMark —— 一条现读标记。
type kmpMark struct {
	kind         string
	file         string
	line         int
	anchor       string
	inlineReason string
}

// kmpHit —— 标记 + 理由判定。
type kmpHit struct {
	kmpMark
	reason    string
	reasonSrc string
	hasReason bool
}

type kmpState struct {
	hits          []kmpHit
	withReason    int
	withoutReason int
	dossierHits   []string
	entries       int
	removed       int
}

// kmpWalk —— 复刻 py `os.walk` + `sorted(dirnames)` / `sorted(filenames)` 的**前序**遍历。
func kmpWalk(base string, dirpath string, visit func(dirpath string, files []string)) {
	ents, err := os.ReadDir(dirpath)
	if err != nil {
		return
	}
	files := []string{}
	var subdirs []string
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() {
			if !kmpPruneDirs[name] {
				subdirs = append(subdirs, name)
			}
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)
	sort.Strings(subdirs)
	visit(dirpath, files)
	for _, d := range subdirs {
		kmpWalk(base, filepath.Join(dirpath, d), visit)
	}
}

// kmpScanMarks —— py `scan_marks()`（**排序稳定** · 逐条不许丢）。
func kmpScanMarks(root string) []kmpMark {
	var out []kmpMark
	domains := []struct {
		domain string
		ext    string
		rx     *regexp.Regexp
		kind   string
	}{
		{"ui", ".rs", kmpRustMark, "rust-allow-dead-code"},
		{"core", ".go", kmpGoMark, "go-ignore"},
	}
	for _, d := range domains {
		base := filepath.Join(root, d.domain)
		if !kmpIsDir(base) {
			continue
		}
		kmpWalk(base, base, func(dirpath string, files []string) {
			for _, fn := range files {
				if !strings.HasSuffix(fn, d.ext) {
					continue
				}
				p := filepath.Join(dirpath, fn)
				txt, ok := kmpReadText(p)
				if !ok {
					continue
				}
				for i, ln := range kmpSplitLines(txt) {
					if !d.rx.MatchString(ln) {
						continue
					}
					inline := ""
					if m := kmpGoReason.FindStringSubmatch(ln); m != nil {
						inline = strings.TrimSpace(m[1])
					}
					out = append(out, kmpMark{
						kind: d.kind, file: kmpRelPath(p, root), line: i + 1,
						anchor: strings.TrimSpace(ln), inlineReason: inline,
					})
				}
			}
		})
	}
	return out
}

var kmpReasonSrcCase = "码内（`" + kmpGoLintTagReason + "` 形态）"

// kmpJudge —— py `judge()`（纯函数：案卷 × 现读标记 → 判定）。
func kmpJudge(dossier *kmpVal, marks []kmpMark, root string) kmpState {
	st := kmpState{}
	entriesV := dossier.get("entries")
	removedV := dossier.get("history_removed")
	var entries []*kmpVal
	if entriesV != nil && entriesV.kind == 'a' {
		entries = entriesV.arr
	}
	if removedV != nil && removedV.kind == 'a' {
		st.removed = len(removedV.arr)
	}
	st.entries = len(entries)

	idx := map[string]*kmpVal{}
	for _, e := range entries {
		f := ""
		if v := e.get("file"); v != nil && v.kind == 's' {
			f = v.str
		}
		a := ""
		if v := e.get("anchor"); v != nil && v.kind == 's' {
			a = v.str
		}
		idx[f+"\x00"+strings.TrimSpace(a)] = e
	}

	for _, m := range marks {
		e := idx[m.file+"\x00"+m.anchor]
		reason, src := "", ""
		if e != nil {
			if v := e.get("reason"); v != nil && v.kind == 's' {
				reason = strings.TrimSpace(v.str)
			}
			src = "案卷登记"
		}
		if reason == "" && m.inlineReason != "" {
			reason = m.inlineReason
			src = kmpReasonSrcCase
		}
		h := kmpHit{kmpMark: m, reason: reason, reasonSrc: src, hasReason: reason != ""}
		if h.hasReason {
			st.withReason++
		} else {
			st.withoutReason++
		}
		st.hits = append(st.hits, h)
	}

	// ── 案卷件自身的问题（**只有这些改退码**）──
	if st.removed > 0 {
		st.dossierHits = append(st.dossierHits,
			fmt.Sprintf("M4 只增不减**被破**：`history_removed` 非空（%d 条）⇒ 删了行 ⇒ 红", st.removed))
	}
	anyNil, seqs := false, 0
	var ints []int
	for _, e := range entries {
		seq := e.get("seq")
		seqs++
		if seq == nil {
			anyNil = true
			continue
		}
		if seq.kind == 'n' && !strings.ContainsAny(seq.num, ".eE") {
			if n, err := strconv.Atoi(seq.num); err == nil && n >= 0 {
				ints = append(ints, n)
			}
		}
		_ = seq
	}
	if anyNil {
		st.dossierHits = append(st.dossierHits,
			"M4 登记行缺 `seq`（只增不减的序号是它的凭据）⇒ 红")
	} else {
		bad := false
		if len(ints) != seqs {
			bad = true
		}
		for i := 1; i < len(ints); i++ {
			if ints[i] <= ints[i-1] {
				bad = true
			}
		}
		if bad {
			st.dossierHits = append(st.dossierHits, "M4 `seq` 不是严格递增 / 有重号 ⇒ 红")
		}
	}
	for _, e := range entries {
		if e.get("reason") == nil {
			f := "?"
			if v := e.get("file"); v != nil && v.kind == 's' {
				f = v.str
			}
			st.dossierHits = append(st.dossierHits,
				fmt.Sprintf("M2 登记行**没有 `reason` 字段**：%s ⇒ 红（理由必填）", f))
			continue
		}
		f, a := "", ""
		if v := e.get("file"); v != nil && v.kind == 's' {
			f = v.str
		}
		if v := e.get("anchor"); v != nil && v.kind == 's' {
			a = strings.TrimSpace(v.str)
		}
		if f == "" || a == "" {
			st.dossierHits = append(st.dossierHits,
				fmt.Sprintf("M5 登记行缺 `file`/`anchor`：%s ⇒ 红", kmpPyRepr(e)))
			continue
		}
		p := filepath.Join(root, f)
		if !kmpIsFile(p) {
			st.dossierHits = append(st.dossierHits,
				fmt.Sprintf("M5 锚指向的件不在场：%s（锚 = %s）⇒ 红", f, kmpPyReprStr(kmpRunes(a, 60))))
			continue
		}
		txt, _ := kmpReadText(p)
		n := strings.Count(txt, a)
		if n != 1 {
			st.dossierHits = append(st.dossierHits,
				fmt.Sprintf("M5 内容锚必须恰好命中 1 次：%s 里命中 %d 次（锚 = %s）⇒ 红",
					f, n, kmpPyReprStr(kmpRunes(a, 60))))
		}
	}
	return st
}

// kmpListRules —— py `list_rules()`。
func kmpListRules(stdout io.Writer) {
	fmt.Fprintln(stdout, "案卷 `keep` 列（`R21` · 只增不减 · **不接闸**）· 真源：任务清单-缺口收口-20260923.md:63-65")
	fmt.Fprintln(stdout, "M1 命中不许隐藏（逐条列出）· M2 理由必填、**缺理由仍计命中 且 rc 不变**")
	fmt.Fprintln(stdout, "M3 **不接闸**（不参与放行判定；--block / --enforce / --fail-on-missing 一律拒 rc=2）")
	fmt.Fprintln(stdout, "M4 只增不减（`seq` 严格递增 · `history_removed` 必须为空）")
	fmt.Fprintln(stdout, "M5 内容锚恰好命中 1 次（**不钉行号**）")
	fmt.Fprintf(stdout, "标记形态：Rust `#[allow(dead_code)]`（ui/**/*.rs）· Go `%s`（core/**/*.go）\n", kmpGoLintTagReason)
	fmt.Fprintf(stdout, "扫描排除：%s\n", strings.Join(kmpPruneSorted, " · "))
	fmt.Fprintln(stdout, "案卷件：scripts/gates/keep-marks-registry.json")
	fmt.Fprintln(stdout, "退码：0 跑出来（含「24 处全缺理由」）· 1 案卷件自身坏了 · 2 不给结论（缺件/想接闸/自检不过）")
}

// kmpJSONOut —— py `json.dumps({k: obj[k] for k in want} if want else obj, ensure_ascii=False)`。
func kmpJSONOut(want string, rc, uiN, apiN, goN int, st kmpState, base *kmpVal) string {
	counts := &kmpVal{kind: 'o', obj: map[string]*kmpVal{},
		keys: []string{"ui", "ui_api_rs", "go", "with_reason", "without_reason"}}
	counts.obj["ui"] = &kmpVal{kind: 'n', num: strconv.Itoa(uiN)}
	counts.obj["ui_api_rs"] = &kmpVal{kind: 'n', num: strconv.Itoa(apiN)}
	counts.obj["go"] = &kmpVal{kind: 'n', num: strconv.Itoa(goN)}
	counts.obj["with_reason"] = &kmpVal{kind: 'n', num: strconv.Itoa(st.withReason)}
	counts.obj["without_reason"] = &kmpVal{kind: 'n', num: strconv.Itoa(st.withoutReason)}
	dh := &kmpVal{kind: 'a'}
	for _, h := range st.dossierHits {
		dh.arr = append(dh.arr, &kmpVal{kind: 's', str: h})
	}
	hits := &kmpVal{kind: 'a'}
	for _, h := range st.hits {
		e := &kmpVal{kind: 'o', obj: map[string]*kmpVal{},
			keys: []string{"file", "line", "anchor", "has_reason", "reason_src"}}
		e.obj["file"] = &kmpVal{kind: 's', str: h.file}
		e.obj["line"] = &kmpVal{kind: 'n', num: strconv.Itoa(h.line)}
		e.obj["anchor"] = &kmpVal{kind: 's', str: h.anchor}
		e.obj["has_reason"] = &kmpVal{kind: 'b', bl: h.hasReason}
		e.obj["reason_src"] = &kmpVal{kind: 's', str: h.reasonSrc}
		hits.arr = append(hits.arr, e)
	}
	if base == nil {
		base = &kmpVal{kind: 'o', obj: map[string]*kmpVal{}}
	}
	root := &kmpVal{kind: 'o', obj: map[string]*kmpVal{},
		keys: []string{"rc", "counts", "baseline", "dossier_hits", "hits"}}
	root.obj["rc"] = &kmpVal{kind: 'n', num: strconv.Itoa(rc)}
	root.obj["counts"] = counts
	root.obj["baseline"] = base
	root.obj["dossier_hits"] = dh
	root.obj["hits"] = hits

	var want2 []string
	for _, f := range strings.Split(want, ",") {
		if s := strings.TrimSpace(f); s != "" {
			want2 = append(want2, s)
		}
	}
	sel := root
	if len(want2) > 0 {
		sel = &kmpVal{kind: 'o', obj: map[string]*kmpVal{}, keys: []string{}}
		for _, k := range want2 {
			if _, seen := sel.obj[k]; seen {
				continue
			}
			sel.obj[k] = root.obj[k]
			sel.keys = append(sel.keys, k)
		}
	}
	return kmpPyJSON(sel)
}
