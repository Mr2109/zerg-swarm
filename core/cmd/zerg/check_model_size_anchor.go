// check_model_size_anchor.go —— 判据件 `check-model-size-anchor.py`（577 行）的 **Go 真搬**
// （M2 · 入口 `checkModelSizeAnchorGateMain`）。
//
// 为什么有这件（M2 = 按价值真搬 Go · 落点 core/cmd/zerg/）：
//
//	验收判据 = 对同一 argv 跑两态（py 原件 ⟷ 本入口），stdout / stderr / 退码三面逐字相同
//	（除下列**显式声明**的口径差面与未覆盖面）。本件**只落码、不接线**（✗ 不注册命令 ·
//	✗ 不删 py 原件 · 入口暂无调用点）。
//
// 本门 = 「名册 size_bytes 锚 ⟷ 盘上现读」（本机层 · 只读）+「fleet 行 ⟷ registry 记录接线（棘轮）」。
// 读面：仓内 gateway 的 fleet 件 · 仓内真源 json · 本机模型目录 manifests 列举 · 本机 stat ——
// 零子进程 / 零联网 / 零 TTY / 零计时 / 零随机 / 默认档不写盘（自检用 tempfile.TemporaryDirectory 自回收）。
//
// ═══════════════════ 出厂声明行 (a) 默认根定位差异 ═══════════════════
//
//	py 从 `__file__` 上溯两层得仓根（REPO_ROOT），据此推 --fleet / --idmap 缺省；Go 无 `__file__`
//	语境 ⇒ 默认根 = cwd（`os.Getwd()`，同本批既有口径）。⇒「脚本位置 ≠ cwd」时缺省推导面不同
//	（✗ 不声称逐字对齐）。对拍甲档 cwd=真仓且 py 走原件绝对路径（两面同根）；乙档把 py 副本放进
//	`<fx>/scripts/gates/` 且 Go 以 cwd=`<fx>` 跑（两面同根）。
//
// ═══════════════════ 出厂声明行 (b) 自指检查 ═══════════════════
//
//	真判：本件是否落在它自己扫描的面内？本门扫描面 = **三处具体物件**（仓内 fleet 件 · 仓内真源
//	json · 本机 manifests 目录**子目录名**）；py 全件零 `walk`/`scandir`/`glob`，唯一目录列举是
//	`os.listdir(manifests)`（只取子目录名进集合、不进输出行序）⇒ 本件**不在自己的扫描面内**
//	（Go 件落在 core/cmd/zerg/，不在 gateway/ 也不在 manifests/ 下）。
//	第四形态（落在他门扫描面内）：`check-error-kinds.py` 的 R3 只扫 `core/cmd/zerg/*.go` 里的
//	错误构造调用首参（实测其正文自陈「不扫 JSON 信封 schema 标签」）⇒ 本件的 json 结构标签
//	（形如 `json:` 后接 kind）不命中 R3。件头/正文不写任何自门触发子串；自检夹具串是**输入**、非扫描面命中。
//
// ═══════════════════ 输出行序的决定者 ═══════════════════
//
//	① 锚明细：`for name in sorted(models)` + `enumerate(models[name])`
//	   ⇒ Go 侧对模型名 `sort.Strings`，候选按原下标序。
//	② registry 记录 id 集是 **set**（`os.listdir` 序不进输出；只在 `sorted(reg_ids)` 处进正文）
//	   ⇒ Go 用 map + `sort.Strings` 复现 `sorted(set)`。
//	③ E①' 真源 fleet_id 遍历 `sorted(f2r)` ⇒ `sort.Strings`。
//	★ 零 Go map 迭代进输出：凡 py 迭代 dict/set 的点，Go 侧都走显式排序的键切片。
//
// ═══════════════════ py 现读硬伤（逐字沿用 · 交回入账 · ✗ 本单不修 py）═══════════════════
//
//	① **C 类**：py 无最外层兜底（`__main__` 裸 `sys.exit(main())`）。本件正常路径已返回 0/1/2；
//	   真源 json 顶格是数组一类「`doc.get` 属性错」在 py 是回溯 rc=1，本件走「读不到」档静默退 2
//	   （写进未覆盖面）。
//	② **A 类**：py 自检用 `tempfile.TemporaryDirectory()`（**with 块 · 有回收**）⇒ 本件 `defer
//	   os.RemoveAll` 对齐（✗ 非「不回收」形态）。
//	③ **D 类**：`--list-rules` 真打印正文（非死旗标）⇒ 本件逐字复刻。
//	④ **机器读面**：本门**有** `--json`（非 check-nodes-roster 的「无机器读面」型）⇒ 逐字复刻其
//	   `json.dumps(ensure_ascii=False, indent=2)` 形状（键序 · 空表 `[]` · null 三态）。
//
// ═══════════════════ 未覆盖面（✗ 不声称逐字对齐）═══════════════════
//
//	① **默认根定位**：见声明行 (a)。
//	② **argparse 极端内部形态**：`--` 分隔后位置参数 / 短旗标簇（`-hf`）/ `--flag=v` 打在 store_true
//	   上 / 只写 `-` —— 本件按「未知 ⇒ unrecognized / 缺值 ⇒ expected one argument / 是旗标却带 `=`
//	   ⇒ ignored explicit argument」处理，深角正文不同（常用档已逐字对齐）。
//	③ **帮助面行宽**：argparse 按终端列宽折行（COLUMNS 缺省 80 ⇒ 宽 78）。本件**钉死 78 宽**下
//	   的折行；若门跑环境设了 `COLUMNS`/真 TTY 且列宽 ≠80 ⇒ 帮助/用法正文折行位置可能不同。
//	④ **`int()` 面**：py `int(str)` 认 Unicode 十进制数字与数字间下划线；本件 cmsaPyInt 只认 ASCII
//	   数字 + 号 + 数字间下划线 ⇒ 非 ASCII 数字档不同（真仓锚全为 ASCII ⇒ 不触发）。
//	⑤ **非 UTF-8/读失败正文**：py `open(encoding="utf-8")` 严格解码（非法序列抛异常，正文含 codec 措辞）；
//	   本件 utf8.Valid 判否 ⇒ 走同一 rc=2 档但 stderr 正文字节不同。
//	⑥ **真源顶层非对象**：见硬伤①。
//	⑦ **`fleet_ids` 非列表**：py `[str(x) for x in (v or [])]` 对字符串会**逐字符**迭代；本件只收数组。
//	⑧ **`str(x)` 近似**：json 数值在 py 分 int/float（`5` 与 `5.0` 不同），Go 反序列化统一 float64 ⇒
//	   `registry_id`/`fleet_ids` 元素为非字符串时 `str()` 正文可能不同（真仓全为字符串 ⇒ 不触发）。
//	⑨ **`errors='replace'` 粒度**与 universal newlines 之外的行界（`\v`/`\f`/`\x1c-\x1e`/`\x85`/
//	   U+2028/U+2029）已按 py splitlines 复刻，但非法 UTF-8 替换粒度按字节 ⟷ py 按序列 ⇒ 不同。
//	⑩ **`json.dumps` 极角**：本件用 Encoder(SetEscapeHTML(false), SetIndent) 复刻；U+2028/U+2029
//	   在 Go 侧恒被转义（py 不转）——真仓数据不含该码点 ⇒ 不触发。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ─────────────────────────── py 出处的常量串（逐字节取自 py 原件）───────────────────────────

// cmsaProg —— argparse 的 prog（= 件基名）。
const cmsaProg = "check-model-size-anchor.py"

// cmsaListRules —— py 模块常量 `LIST_RULES`（`--list-rules` 的输出 · 逐字节取自 py 原件）。
const cmsaListRules = "check-model-size-anchor.py 判据 / 路径来源 / 退码\n" +
	"  A  可解析 `models:` 段（否则 rc=2）\n" +
	"  B  本机绝对路径的 `size_bytes` 锚 ⇒ stat 对拍（相等才算过）\n" +
	"  C  本机读不到的锚 ⇒ 只报告「跳过」（不给结论、不算红）\n" +
	"  D  无锚 ⇒ rc=2（空转假覆盖）；有锚但本机一条都核不到 ⇒ rc=2\n" +
	"  E  fleet 行 ⟷ registry 记录接线（棘轮）：① 已带 registry_id 的行 ⇒ registry_id ∈ 记录 id 集（硬判）；\n" +
	"     ①' 真源 fleet_ids ⊆ fleet.yaml 模型册；② 带 registry_id 的行数 ≥ 棘轮（现读 17 · 只许升不许降）\n" +
	"  路径：gateway/fleet.yaml（--fleet 可覆盖）· core/internal/contract/model-id-map.json（--idmap）·\n" +
	"       ~/.zerg/models/manifests（--registry-dir 或 ZERG_MODELS_DIR）\n" +
	"  退码：0 本机层全对＋接线全过 · 1 有漂移/非整数锚/接线缺陷 · 2 不给结论\n"

// cmsaRegistryWiringRatchet —— 判据 E② 的棘轮（同 py 常量 `REGISTRY_WIRING_RATCHET`）。
const cmsaRegistryWiringRatchet = 17

// ══════════════════════════ 路径 / 文件原语 ══════════════════════════

// cmsaPyJoin —— 照 CPython posixpath.join 的逐段规则（✗ 不用 filepath.Join，见标准声明行）。
func cmsaPyJoin(parts ...string) string {
	out := ""
	have := false
	for _, p := range parts {
		if !have {
			if p != "" {
				out = p
				have = true
			}
			continue
		}
		if len(p) > 0 && p[0] == '/' {
			out = p
			continue
		}
		if out == "" || out[len(out)-1] == '/' {
			out = out + p
		} else {
			out = out + "/" + p
		}
	}
	return out
}

// cmsaHome —— py `os.path.expanduser("~")`。
func cmsaHome() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}

// cmsaExists —— py `os.path.exists(p)`（stat 跟随符号链接）。
func cmsaExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// cmsaIsDir —— py `os.path.isdir(p)`。
func cmsaIsDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// cmsaGetSize —— py `os.path.getsize(p)`。
func cmsaGetSize(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.Size()
}

// cmsaRepoRoot —— 见声明行 (a)：无 `__file__` ⇒ 取 cwd。
func cmsaRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func cmsaDefaultFleet() string { return cmsaPyJoin(cmsaRepoRoot(), "gateway", "fleet.yaml") }
func cmsaDefaultIdmap() string {
	return cmsaPyJoin(cmsaRepoRoot(), "core", "internal", "contract", "model-id-map.json")
}

// cmsaDefaultRegistryDir —— py `default_registry_dir()`。
func cmsaDefaultRegistryDir() string {
	root := cmsaPyStrip(os.Getenv("ZERG_MODELS_DIR"))
	if root == "" {
		root = cmsaPyJoin(cmsaHome(), ".zerg", "models")
	}
	return cmsaPyJoin(root, "manifests")
}

// cmsaNormalizeNewlines —— py 文本模式 universal newlines（`\r\n` 与裸 `\r` ⇒ `\n`）。
func cmsaNormalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// cmsaSplitLines —— py `str.splitlines()`（`\n` 之外还认 `\v`/`\f`/`\x1c-\x1e`/`\x85`/U+2028/U+2029）。
func cmsaSplitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	for i := 0; i < len(s); {
		r, sz := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\n', '\x0b', '\x0c', '\x1c', '\x1d', '\x1e', '\u0085', '\u2028', '\u2029':
			out = append(out, b.String())
			b.Reset()
			i += sz
			continue
		}
		b.WriteRune(r)
		i += sz
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// ══════════════════════════ py 空白 / 字符串原语 ══════════════════════════

// cmsaIsSpace —— py `str.strip()`/`\s` 的 Unicode 空白（含 \x1c-\x1f / \x85 / \xa0）。
func cmsaIsSpace(r rune) bool {
	switch r {
	case 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x85, 0xa0,
		0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// cmsaPyStrip —— py `str.strip()`（无参 · Unicode 空白）。
func cmsaPyStrip(s string) string {
	return strings.TrimFunc(s, cmsaIsSpace)
}

// cmsaLstripSpaces —— py `s.lstrip(" ")`（**只剥 ASCII 空格**）。
func cmsaLstripSpaces(s string) string {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return s[i:]
}

// cmsaPyRepr —— py `repr(str)` 的近似（可读面 · 见未覆盖面）。
func cmsaPyRepr(s string) string {
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
		case '\t':
			b.WriteString(`\t`)
		default:
			if byte(r) == q && int32(r) < 0x80 {
				b.WriteByte('\\')
				b.WriteRune(r)
			} else if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(q)
	return b.String()
}

// cmsaPyListRepr —— py `repr(list[str])`（`sorted(reg_ids)` / findings 展示用）。
func cmsaPyListRepr(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = cmsaPyRepr(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// cmsaPyInt —— py `int(str.strip())` 的近似（ASCII 数字 + 号 + 数字间下划线 · 见未覆盖面④）。
func cmsaPyInt(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid literal for int")
	}
	neg := false
	i := 0
	if s[0] == '+' || s[0] == '-' {
		neg = s[0] == '-'
		i = 1
	}
	if i >= len(s) {
		return 0, fmt.Errorf("invalid literal for int")
	}
	var digits []byte
	prevUnderscore := false
	for ; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			if len(digits) == 0 || prevUnderscore {
				return 0, fmt.Errorf("invalid literal for int")
			}
			prevUnderscore = true
			continue
		}
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid literal for int")
		}
		digits = append(digits, c)
		prevUnderscore = false
	}
	if len(digits) == 0 || prevUnderscore {
		return 0, fmt.Errorf("invalid literal for int")
	}
	var n int64
	for _, c := range digits {
		n = n*10 + int64(c-'0')
	}
	if neg {
		n = -n
	}
	return n, nil
}

// ══════════════════════════ 极简 YAML（models: 段）══════════════════════════

// cmsaSplitTop —— py `_split_top`（忽略引号内与 `[]{}` 内的逗号）。
func cmsaSplitTop(s string) []string {
	var out []string
	var buf []rune
	depth := 0
	var q rune
	for _, ch := range s {
		if q != 0 {
			buf = append(buf, ch)
			if ch == q {
				q = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			q = ch
			buf = append(buf, ch)
			continue
		}
		if ch == '[' || ch == '{' {
			depth++
		} else if ch == ']' || ch == '}' {
			depth--
		}
		if ch == ',' && depth == 0 {
			out = append(out, string(buf))
			buf = nil
			continue
		}
		buf = append(buf, ch)
	}
	if len(buf) > 0 {
		out = append(out, string(buf))
	}
	return out
}

// cmsaKV —— py `_kv`。
func cmsaKV(s string) (string, string, bool) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return "", "", false
	}
	k := cmsaPyStrip(s[:i])
	v := cmsaPyStrip(s[i+1:])
	vr := []rune(v)
	if len(vr) >= 2 && vr[0] == vr[len(vr)-1] && (vr[0] == '"' || vr[0] == '\'') {
		v = string(vr[1 : len(vr)-1])
	}
	return k, v, true
}

// cmsaParseFlow —— py `_parse_flow`。
func cmsaParseFlow(body string) map[string]string {
	inner := cmsaPyStrip(body)
	if strings.HasPrefix(inner, "{") {
		inner = inner[1:]
	}
	if strings.HasSuffix(inner, "}") {
		inner = inner[:len(inner)-1]
	}
	d := map[string]string{}
	for _, part := range cmsaSplitTop(inner) {
		if k, v, ok := cmsaKV(part); ok {
			d[k] = v
		}
	}
	return d
}

// cmsaParseModels —— py `parse_models(text)`：{模型名: [候选dict, ...]}，只认 `models:` 段。
func cmsaParseModels(text string) map[string][]map[string]string {
	models := map[string][]map[string]string{}
	curName := ""
	hasName := false
	var curCand map[string]string
	inModels := false
	for _, line := range cmsaSplitLines(text) {
		stripped := cmsaPyStrip(line)
		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") { // 顶层键
			inModels = strings.HasPrefix(stripped, "models:")
			curName = ""
			hasName = false
			curCand = nil
			continue
		}
		if !inModels {
			continue
		}
		indent := len(line) - len(cmsaLstripSpaces(line))
		if indent == 2 {
			if strings.HasSuffix(stripped, ":") {
				curName = cmsaPyStrip(stripped[:len(stripped)-1])
				hasName = true
				if _, ok := models[curName]; !ok {
					models[curName] = nil
				}
				curCand = nil
			} else {
				nm, rest, _ := strings.Cut(stripped, ":")
				curName = cmsaPyStrip(nm)
				hasName = true
				if _, ok := models[curName]; !ok {
					models[curName] = nil
				}
				rest = cmsaPyStrip(rest)
				var cand map[string]string
				if strings.HasPrefix(rest, "{") {
					cand = cmsaParseFlow(rest)
				} else {
					cand = map[string]string{}
				}
				models[curName] = append(models[curName], cand)
				curCand = models[curName][len(models[curName])-1]
			}
		} else if indent >= 4 && hasName {
			if strings.HasPrefix(stripped, "- ") {
				body := cmsaPyStrip(stripped[2:])
				if strings.HasPrefix(body, "{") {
					models[curName] = append(models[curName], cmsaParseFlow(body))
					curCand = models[curName][len(models[curName])-1]
				} else {
					curCand = map[string]string{}
					models[curName] = append(models[curName], curCand)
					if k, v, ok := cmsaKV(body); ok {
						curCand[k] = v
					}
				}
			} else if curCand != nil {
				if k, v, ok := cmsaKV(stripped); ok {
					curCand[k] = v
				}
			}
		}
	}
	return models
}

// ══════════════════════════ 判据 A–D：锚对拍 ══════════════════════════

// cmsaResult —— py `check()` 的一条结果（对应那个 8 元组）。
type cmsaResult struct {
	kind      string
	name      string
	idx       int
	host      string
	file      string
	anchorNum int64
	anchorStr string
	actualNum int64
	actualSet bool
	note      string
}

// cmsaCheck —— py `check(models, base_dir="")`。
func cmsaCheck(models map[string][]map[string]string, baseDir string) []cmsaResult {
	var results []cmsaResult
	names := make([]string, 0, len(models))
	for n := range models {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		for idx, cand := range models[name] {
			raw, ok := cand["size_bytes"]
			if !ok {
				continue
			}
			host := cand["host"]
			f := cand["file"]
			anchor, err := cmsaPyInt(cmsaPyStrip(raw))
			if err != nil {
				results = append(results, cmsaResult{kind: "BAD", name: name, idx: idx, host: host,
					file: f, anchorStr: raw, note: "size_bytes 非整数"})
				continue
			}
			path := f
			if !strings.HasPrefix(f, "/") {
				path = cmsaPyJoin(baseDir, f)
			}
			if path == "" || !cmsaExists(path) {
				results = append(results, cmsaResult{kind: "SKIP", name: name, idx: idx, host: host,
					file: f, anchorNum: anchor, note: "本机读不到（远端 host 或件不在盘）"})
				continue
			}
			actual := cmsaGetSize(path)
			if actual == anchor {
				results = append(results, cmsaResult{kind: "OK", name: name, idx: idx, host: host,
					file: f, anchorNum: anchor, actualNum: actual, actualSet: true})
			} else {
				results = append(results, cmsaResult{kind: "MISMATCH", name: name, idx: idx, host: host,
					file: f, anchorNum: anchor, actualNum: actual, actualSet: true})
			}
		}
	}
	return results
}

// cmsaVerdict —— py `verdict(results)`。
func cmsaVerdict(results []cmsaResult) int {
	if len(results) == 0 {
		return 2
	}
	for _, r := range results {
		if r.kind == "MISMATCH" || r.kind == "BAD" {
			return 1
		}
	}
	for _, r := range results {
		if r.kind == "OK" {
			return 0
		}
	}
	return 2
}

// cmsaRender —— py `render(results)`（返回不带尾换行的正文）。
func cmsaRender(results []cmsaResult) string {
	lines := make([]string, 0, len(results))
	for _, r := range results {
		switch r.kind {
		case "OK":
			lines = append(lines, fmt.Sprintf("  ✅ %s[%d] host=%s size_bytes=%d == 现读 %d  %s",
				r.name, r.idx, r.host, r.anchorNum, r.actualNum, r.file))
		case "MISMATCH":
			lines = append(lines, fmt.Sprintf("  ❌ %s[%d] host=%s 锚=%d ≠ 现读 %d  %s（漂移）",
				r.name, r.idx, r.host, r.anchorNum, r.actualNum, r.file))
		case "BAD":
			lines = append(lines, fmt.Sprintf("  ❌ %s[%d] host=%s size_bytes=%s 不是整数  %s",
				r.name, r.idx, r.host, cmsaPyRepr(r.anchorStr), r.file))
		default:
			lines = append(lines, fmt.Sprintf("  ⚪ %s[%d] host=%s 锚=%d 跳过：%s  %s",
				r.name, r.idx, r.host, r.anchorNum, r.note, r.file))
		}
	}
	return strings.Join(lines, "\n")
}

// ══════════════════════════ 判据 E：fleet ⟷ registry 接线（棘轮）══════════════════════════

// cmsaLoadRegistryEntries —— py `load_registry_entries(path)`：→ (f2r, r2f, ok)。
func cmsaLoadRegistryEntries(path string) (map[string]string, map[string][]string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, false
	}
	var doc interface{}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, nil, false
	}
	dm, ok := doc.(map[string]interface{})
	if !ok {
		return nil, nil, false
	}
	entsV, ok := dm["entries"]
	if !ok {
		return nil, nil, false
	}
	ents, ok := entsV.([]interface{})
	if !ok {
		return nil, nil, false
	}
	f2r := map[string]string{}
	r2f := map[string][]string{}
	for _, e := range ents {
		em, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		rid := cmsaPyStrip(cmsaPyStr(em["registry_id"]))
		if rid == "" {
			continue
		}
		fids := []string{}
		if fv, ok := em["fleet_ids"]; ok && fv != nil {
			if fl, ok := fv.([]interface{}); ok {
				for _, x := range fl {
					fids = append(fids, cmsaPyStr(x))
				}
			}
		}
		r2f[rid] = fids
		for _, fid := range fids {
			f2r[fid] = rid
		}
	}
	return f2r, r2f, true
}

// cmsaPyStr —— py `str(x)` 的近似（见未覆盖面⑧）。
func cmsaPyStr(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%v", x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// cmsaRegistryRecordIds —— py `registry_record_ids(dir)`：→ (id 集, 目录在否)。
func cmsaRegistryRecordIds(dir string) (map[string]bool, bool) {
	if !cmsaIsDir(dir) {
		return nil, false
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}
	ids := map[string]bool{}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if cmsaIsDir(cmsaPyJoin(dir, name)) {
			ids[name] = true
		}
	}
	return ids, true
}

// cmsaCheckRegistryWiring —— py `check_registry_wiring`：→ (rc, findings, n_with)。
func cmsaCheckRegistryWiring(models map[string][]map[string]string, f2r map[string]string,
	regIDs map[string]bool, regIDsSet bool, ratchet int, registryDirNote string) (int, []string, int) {
	findings := []string{}
	defect := false

	// E①' 反向：真源里的 fleet_id 必在 fleet.yaml 模型册里。
	fkeys := make([]string, 0, len(f2r))
	for k := range f2r {
		fkeys = append(fkeys, k)
	}
	sort.Strings(fkeys)
	for _, fid := range fkeys {
		if _, ok := models[fid]; !ok {
			findings = append(findings, fmt.Sprintf(
				"E①' 真源 fleet_id `%s`（→ `%s`）不在 fleet.yaml 模型册里（引了不存在的行）", fid, f2r[fid]))
			defect = true
		}
	}

	// 数「带 registry_id 的行」（逐候选展开，同 /api/fleet/models 一行一候选）。
	type cmsaRow struct {
		name string
		idx  int
		rid  string
	}
	var rows []cmsaRow
	names := make([]string, 0, len(models))
	for n := range models {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		if rid, ok := f2r[name]; ok && rid != "" {
			for idx := range models[name] {
				rows = append(rows, cmsaRow{name, idx, rid})
			}
		}
	}
	nWith := len(rows)

	// E① 硬判：registry_id ∈ registry 记录 id 集。
	if !regIDsSet {
		findings = append(findings, fmt.Sprintf("E① registry 记录目录读不到 ⇒ 接线判不了（不给结论）：%s", registryDirNote))
	} else {
		rks := make([]string, 0, len(regIDs))
		for k := range regIDs {
			rks = append(rks, k)
		}
		sort.Strings(rks)
		for _, r := range rows {
			if !regIDs[r.rid] {
				findings = append(findings, fmt.Sprintf(
					"E① %s[#%d]：registry_id `%s` 不在 registry 记录 id 集 %s 里（引了不存在的快照记录）",
					r.name, r.idx, r.rid, cmsaPyListRepr(rks)))
				defect = true
			}
		}
	}

	// E② 棘轮
	if nWith < ratchet {
		findings = append(findings, fmt.Sprintf(
			"E② 带 registry_id 的行数 %d < 棘轮 %d（接线回退 —— 只许升不许降）", nWith, ratchet))
		defect = true
	}

	if defect {
		return 1, findings, nWith
	}
	if !regIDsSet {
		return 2, findings, nWith
	}
	return 0, findings, nWith
}

// cmsaCombineRc —— py `combine_rc`：红优先，其次不给结论，最后全绿。
func cmsaCombineRc(a, b int) int {
	if a == 1 || b == 1 {
		return 1
	}
	if a == 2 || b == 2 {
		return 2
	}
	return 0
}

// ══════════════════════════ 自检（合成夹具 · 成对负控 · 不碰真目标）══════════════════════════

// cmsaWiringSelfTest —— py `_wiring_self_test(fails)`：两层都钉（内部分析函数层 + 主入口层）。
func cmsaWiringSelfTest(stdout io.Writer) []string {
	fails := []string{}
	idmapGood := `[{"registry_id": "R1", "fleet_ids": ["M"]}]`
	modelsTxt := "models:\n  M:\n    - { host: Mr2109, file: /x.gguf, mem_gb: 1 }\n" +
		"    - { host: edge-node, file: /y.gguf, mem_gb: 1 }\n"

	run := func(label, entriesJSON string, regSubdirs []string, hasRegDir bool, ratchet, wantRC int) {
		td, err := os.MkdirTemp("", "tmp")
		if err != nil {
			fails = append(fails, "wiring/"+label+"：造不出临时目录")
			return
		}
		defer func() { _ = os.RemoveAll(td) }()
		idmap := cmsaPyJoin(td, "model-id-map.json")
		_ = os.WriteFile(idmap, []byte(`{"schema": "z/test", "entries": `+entriesJSON+`}`), 0o644)
		regdir := cmsaPyJoin(td, "manifests")
		if hasRegDir {
			_ = os.MkdirAll(regdir, 0o755)
			for _, sd := range regSubdirs {
				_ = os.MkdirAll(cmsaPyJoin(regdir, sd), 0o755)
			}
		}
		models := cmsaParseModels(modelsTxt)
		f2r, _, _ := cmsaLoadRegistryEntries(idmap)
		regIDs, regSet := cmsaRegistryRecordIds(regdir)
		rc, findings, _ := cmsaCheckRegistryWiring(models, f2r, regIDs, regSet, ratchet, regdir)
		status := "PASS"
		if rc != wantRC {
			status = "FAIL"
			fails = append(fails, fmt.Sprintf("wiring/%s：期望 rc=%d 实得 rc=%d（findings=%s）",
				label, wantRC, rc, cmsaPyListRepr(findings)))
		}
		fmt.Fprintf(stdout, "  [%s] wiring/%-16s 期望 rc=%d 实得 rc=%d\n", status, label, wantRC, rc)
	}

	run("good", idmapGood, []string{"R1"}, true, 2, 0)
	run("false-registry-id", `[{"registry_id": "R2", "fleet_ids": ["M"]}]`, []string{"R1"}, true, 2, 1)
	run("ratchet-bump", idmapGood, []string{"R1"}, true, 3, 1)
	run("dangling-fleet-id", `[{"registry_id": "R1", "fleet_ids": ["NOPE"]}]`, []string{"R1"}, true, 0, 1)
	run("no-registry-dir", idmapGood, nil, false, 2, 2)

	// ── 主入口层（进程退码）──
	td, err := os.MkdirTemp("", "tmp")
	if err != nil {
		fails = append(fails, "proc：造不出临时目录")
		return fails
	}
	defer func() { _ = os.RemoveAll(td) }()
	real := cmsaPyJoin(td, "m.gguf")
	_ = os.WriteFile(real, []byte(strings.Repeat("z", 321)), 0o644)
	fleet := cmsaPyJoin(td, "fleet.yaml")
	regdir := cmsaPyJoin(td, "manifests")
	_ = os.MkdirAll(cmsaPyJoin(regdir, "R1"), 0o755)
	idmap := cmsaPyJoin(td, "idmap.json")

	proc := func(label string, n int, rid string, want int) {
		ls := []string{"models:", "  M:"}
		for i := 0; i < n; i++ {
			ls = append(ls, fmt.Sprintf("    - { host: Mr2109, file: %s, mem_gb: 1, size_bytes: 321 }", real))
		}
		_ = os.WriteFile(fleet, []byte(strings.Join(ls, "\n")+"\n"), 0o644)
		_ = os.WriteFile(idmap, []byte(fmt.Sprintf(
			`{"schema": "z/t", "entries": [{"registry_id": %q, "fleet_ids": ["M"]}]}`, rid)), 0o644)
		var so, se bytes.Buffer
		rc := cmsaMain(&so, &se, []string{"--fleet", fleet, "--idmap", idmap, "--registry-dir", regdir})
		status := "PASS"
		if rc != want {
			status = "FAIL"
			fails = append(fails, fmt.Sprintf("proc/%s：期望进程 rc=%d 实得 rc=%d", label, want, rc))
		}
		fmt.Fprintf(stdout, "  [%s] proc/%-20s 期望 rc=%d 实得 rc=%d\n", status, label, want, rc)
	}

	proc("good(17行=棘轮)", 17, "R1", 0)
	proc("false-registry-id", 17, "R2", 1)
	proc("ratchet-short(16<17)", 16, "R1", 1)
	return fails
}

// cmsaSelfTest —— py `self_test()`。
func cmsaSelfTest(stdout, stderr io.Writer) int {
	ok := true
	td, err := os.MkdirTemp("", "tmp")
	if err != nil {
		fmt.Fprintf(stdout, "自检跑不了：造不出临时目录（%v）\n", err)
		return 2
	}
	defer func() { _ = os.RemoveAll(td) }()
	real := cmsaPyJoin(td, "w.gguf")
	_ = os.WriteFile(real, []byte(strings.Repeat("x", 1234)), 0o644)
	baseFmt := "models:\n  m1:\n    - { host: Mr2109, file: " + real + ", mem_gb: 1, %s }\n"
	cases := []struct {
		label string
		text  string
		want  int
	}{
		{"good", fmt.Sprintf(baseFmt, "size_bytes: 1234"), 0},
		{"bad", fmt.Sprintf(baseFmt, "size_bytes: 9999"), 1},
		{"badd", fmt.Sprintf(baseFmt, "size_bytes: abc"), 1},
		{"skip", "models:\n  m1:\n    - { host: edge-node, file: /nope/x.gguf, mem_gb: 1, size_bytes: 42 }\n", 2},
		{"none", fmt.Sprintf(baseFmt, "ctx_window: 8"), 2},
		{"block", "models:\n  m2:\n    - host: Mr2109\n      file: " + real + "\n      size_bytes: 1234\n", 0},
	}
	for _, c := range cases {
		rc := cmsaVerdict(cmsaCheck(cmsaParseModels(c.text), ""))
		status := "PASS"
		if rc != c.want {
			status = "FAIL"
			ok = false
		}
		fmt.Fprintf(stdout, "  [%s] %-6s 期望 rc=%d 实得 rc=%d\n", status, c.label, c.want, rc)
	}
	wiringFails := cmsaWiringSelfTest(stdout)
	if len(wiringFails) > 0 {
		ok = false
		for _, f := range wiringFails {
			fmt.Fprintf(stdout, "  [FAIL] %s\n", f)
		}
	}
	concl := "全绿"
	if !ok {
		concl = "有红"
	}
	fmt.Fprintf(stdout, "自检 %s\n", concl)
	if ok {
		return 0
	}
	return 2
}

// ══════════════════════════ argparse 复刻（子集 + 帮助正文）══════════════════════════

type cmsaArgs struct {
	fleet         string
	idmap         string
	registryDir   string
	registryDirOn bool
	listRules     bool
	selfTest      bool
	jsonOut       bool
}

var cmsaOptSpecs = []struct {
	name  string
	takes bool
}{
	{"--fleet", true},
	{"--idmap", true},
	{"--registry-dir", true},
	{"--list-rules", false},
	{"--self-test", false},
	{"--json", false},
	{"--help", false},
}

// cmsaMatchOpt —— argparse 长旗标精确 / 唯一前缀匹配。
func cmsaMatchOpt(name string) (string, bool, bool) {
	for _, s := range cmsaOptSpecs {
		if s.name == name {
			return s.name, s.takes, true
		}
	}
	var cands []string
	for _, s := range cmsaOptSpecs {
		if len(name) < len(s.name) && strings.HasPrefix(s.name, name) {
			cands = append(cands, s.name)
		}
	}
	if len(cands) == 1 {
		for _, s := range cmsaOptSpecs {
			if s.name == cands[0] {
				return s.name, s.takes, true
			}
		}
	}
	return "", false, false
}

// cmsaParseArgs —— 复刻 argparse 面。rc<0 ⇒ 成功；rc=0 ⇒ 打帮助；rc=2 ⇒ 用法错。
func cmsaParseArgs(argv []string) (cmsaArgs, int, string) {
	a := cmsaArgs{fleet: cmsaDefaultFleet(), idmap: cmsaDefaultIdmap()}
	var unknown []string
	i := 0
	for i < len(argv) {
		arg := argv[i]
		if arg == "--" {
			if i+1 < len(argv) {
				unknown = append(unknown, argv[i+1:]...)
			}
			break
		}
		if arg == "-h" || arg == "--help" {
			return cmsaArgs{}, 0, ""
		}
		if strings.HasPrefix(arg, "--") {
			name, val, hasVal := arg, "", false
			if eq := strings.IndexByte(arg, '='); eq >= 0 {
				name, val, hasVal = arg[:eq], arg[eq+1:], true
			}
			canon, takes, ok := cmsaMatchOpt(name)
			if !ok {
				unknown = append(unknown, arg)
				i++
				continue
			}
			if canon == "--help" {
				return cmsaArgs{}, 0, ""
			}
			if takes {
				if !hasVal {
					if i+1 >= len(argv) {
						return cmsaArgs{}, 2, "argument " + canon + ": expected one argument"
					}
					i++
					val = argv[i]
				}
				switch canon {
				case "--fleet":
					a.fleet = val
				case "--idmap":
					a.idmap = val
				case "--registry-dir":
					a.registryDir = val
					a.registryDirOn = true
				}
			} else {
				if hasVal {
					return cmsaArgs{}, 2, "argument " + canon + ": ignored explicit argument '" + val + "'"
				}
				switch canon {
				case "--list-rules":
					a.listRules = true
				case "--self-test":
					a.selfTest = true
				case "--json":
					a.jsonOut = true
				}
			}
			i++
			continue
		}
		unknown = append(unknown, arg)
		i++
	}
	if len(unknown) > 0 {
		return cmsaArgs{}, 2, "unrecognized arguments: " + strings.Join(unknown, " ")
	}
	return a, -1, ""
}

// cmsaUsage3 —— 帮助/用法里的 usage 段（宽 78 下 argparse 折行的结果 · 逐字取自 py -h）。
const cmsaUsage3 = "usage: check-model-size-anchor.py [-h] [--fleet FLEET] [--idmap IDMAP]\n" +
	"                                  [--registry-dir REGISTRY_DIR] [--list-rules]\n" +
	"                                  [--self-test] [--json]\n"

// ── textwrap.wrap 的忠实复刻（argparse 用它折帮助正文：break_on_hyphens / break_long_words
//    默认都开）── 见未覆盖面③：本件按 width=78（COLUMNS 缺省）折行。

// cmsaAWsRune —— textwrap 的 ASCII 空白类 `[ 	\n\r\v\f]`。
func cmsaAWsRune(r rune) bool {
	switch r {
	case ' ', '	', '\n', '\r', '\v', '\f':
		return true
	}
	return false
}

// cmsaTWordRune —— py `\w`（词字符近似：字母/数字/下划线）。
func cmsaTWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// cmsaTLetterRune —— py `[^\d\W]`（词字符且非数字 ⇒ 字母/下划线）。
func cmsaTLetterRune(r rune) bool { return r == '_' || unicode.IsLetter(r) }

// cmsaTWordPunctRune —— py `[\w!"'&.,?]`。
func cmsaTWordPunctRune(r rune) bool {
	if cmsaTWordRune(r) {
		return true
	}
	switch r {
	case '!', '"', '\'', '&', '.', ',', '?':
		return true
	}
	return false
}

// cmsaTextwrapChunks —— py textwrap `_split`（wordsep_re · break_on_hyphens=True）的等价分块。
func cmsaTextwrapChunks(text string) []string {
	rs := []rune(text)
	n := len(rs)
	var chunks []string
	i := 0
	for i < n {
		if cmsaAWsRune(rs[i]) {
			j := i
			for j < n && cmsaAWsRune(rs[j]) {
				j++
			}
			chunks = append(chunks, string(rs[i:j]))
			i = j
			continue
		}
		// 顶层 em-dash 支：`(?<=wp)-{2,}(?=\w)`
		if rs[i] == '-' && i-1 >= 0 && cmsaTWordPunctRune(rs[i-1]) {
			k := i
			for k < n && rs[k] == '-' {
				k++
			}
			if k-i >= 2 && k < n && cmsaTWordRune(rs[k]) {
				chunks = append(chunks, string(rs[i:k]))
				i = k
				continue
			}
		}
		// 词支（懒匹配）：止于「连字符断点 / 词尾 / em-dash」。
		end := n
		for j := i + 1; j <= n; j++ {
			if j >= n {
				end = n
				break
			}
			if cmsaAWsRune(rs[j]) {
				end = j
				break
			}
			if rs[j] == '-' {
				// 连字符断点：`-(?:(?<=lt{2}-)|(?<=lt-lt-))(?=lt -? lt)`
				hyphenA := j >= 2 && cmsaTLetterRune(rs[j-1]) && cmsaTLetterRune(rs[j-2])
				hyphenB := j >= 3 && cmsaTLetterRune(rs[j-1]) && rs[j-2] == '-' && cmsaTLetterRune(rs[j-3])
				if (hyphenA || hyphenB) && j+1 < n && cmsaTLetterRune(rs[j+1]) {
					end = j + 1
					break
				}
				// 词内 em-dash 支：`(?<=wp)(?=-{2,}\w)`
				if cmsaTWordPunctRune(rs[j-1]) {
					k := j
					for k < n && rs[k] == '-' {
						k++
					}
					if k-j >= 2 && k < n && cmsaTWordRune(rs[k]) {
						end = j
						break
					}
				}
			}
		}
		chunks = append(chunks, string(rs[i:end]))
		i = end
	}
	return chunks
}

// cmsaAllSpace —— py `str.strip() == ""`（空白块判定）。
func cmsaAllSpace(s string) bool {
	for _, r := range s {
		if !cmsaAWsRune(r) {
			return false
		}
	}
	return true
}

// cmsaWrapText —— py `textwrap.wrap(text, width)`（默认选项）的忠实复刻。
func cmsaWrapText(text string, width int) []string {
	chunks := cmsaTextwrapChunks(text)
	rev := make([]string, len(chunks))
	for i, c := range chunks {
		rev[len(chunks)-1-i] = c
	}
	var lines []string
	for len(rev) > 0 {
		var cur []string
		curLen := 0
		if len(lines) > 0 && cmsaAllSpace(rev[len(rev)-1]) {
			rev = rev[:len(rev)-1]
			if len(rev) == 0 {
				break
			}
		}
		for len(rev) > 0 {
			l := len([]rune(rev[len(rev)-1]))
			if curLen+l <= width {
				cur = append(cur, rev[len(rev)-1])
				rev = rev[:len(rev)-1]
				curLen += l
			} else {
				break
			}
		}
		if len(rev) > 0 && len([]rune(rev[len(rev)-1])) > width {
			// _handle_long_word（break_long_words=True）
			spaceLeft := width - curLen
			if width < 1 {
				spaceLeft = 1
			}
			last := []rune(rev[len(rev)-1])
			cur = append(cur, string(last[:spaceLeft]))
			rev[len(rev)-1] = string(last[spaceLeft:])
			curLen = 0
			for _, c := range cur {
				curLen += len([]rune(c))
			}
		}
		if len(cur) > 0 && cmsaAllSpace(cur[len(cur)-1]) {
			cur = cur[:len(cur)-1]
		}
		if len(cur) > 0 {
			lines = append(lines, strings.Join(cur, ""))
		}
	}
	return lines
}

// cmsaHelpText —— py argparse `-h/--help` 的 stdout 逐字（Python 3.9 印 `optional arguments:`）。
func cmsaHelpText() string {
	const helpPos = 24
	wrapWidth := 78 - helpPos
	idmapHelp := "判据 E 真源 model-id-map.json（默认 " + cmsaDefaultIdmap() + "）"
	regHelp := "registry 记录目录（manifests 根；默认 ZERG_MODELS_DIR 或 ~/.zerg/models 下的 manifests）"

	var b strings.Builder
	b.WriteString(cmsaUsage3)
	b.WriteString("\n")
	b.WriteString("名册 size_bytes 锚 ⟷ 盘上现读 ＋ fleet ⟷ registry 接线（本机层）\n\n")
	b.WriteString("optional arguments:\n")
	b.WriteString("  -h, --help            show this help message and exit\n")
	b.WriteString("  --fleet FLEET\n")
	// --idmap（header 15 ≤ 20 ⇒ 同行；续行缩进 24）
	b.WriteString("  --idmap IDMAP")
	for i := 0; i < helpPos-len("  --idmap IDMAP"); i++ {
		b.WriteString(" ")
	}
	writeHelp := func(text string) {
		lines := cmsaWrapText(text, wrapWidth)
		for i, l := range lines {
			if i == 0 {
				b.WriteString(l)
			} else {
				b.WriteString("\n")
				b.WriteString(strings.Repeat(" ", helpPos))
				b.WriteString(l)
			}
		}
		b.WriteString("\n")
	}
	writeHelp(idmapHelp)
	// --registry-dir（header 29 > 20 ⇒ 帮助另起一行、缩进 24）
	b.WriteString("  --registry-dir REGISTRY_DIR\n")
	b.WriteString(strings.Repeat(" ", helpPos))
	writeHelp(regHelp)
	b.WriteString("  --list-rules\n")
	b.WriteString("  --self-test\n")
	b.WriteString("  --json\n")
	return b.String()
}

// ══════════════════════════ JSON 机器读面（py `--json`）══════════════════════════

type cmsaJSONItem struct {
	Name   string      `json:"name"`
	Idx    int         `json:"idx"`
	Host   string      `json:"host"`
	File   string      `json:"file"`
	Anchor interface{} `json:"anchor"`
	Actual interface{} `json:"actual"`
	Kind   string      `json:"kind"`
}

type cmsaJSONWiring struct {
	Rc                 int      `json:"rc"`
	Ratchet            int      `json:"ratchet"`
	RowsWithRegistryID int      `json:"rows_with_registry_id"`
	RegistryDir        string   `json:"registry_dir"`
	Findings           []string `json:"findings"`
}

type cmsaJSONOut struct {
	VerdictScope string         `json:"verdict_scope"`
	Rc           int            `json:"rc"`
	Anchors      int            `json:"anchors"`
	Ok           int            `json:"ok"`
	Mismatch     int            `json:"mismatch"`
	Skipped      int            `json:"skipped"`
	Wiring       cmsaJSONWiring `json:"wiring"`
	Items        []cmsaJSONItem `json:"items"`
}

// ══════════════════════════ 入口 ══════════════════════════

// cmsaMain —— py `main(argv)` 的主逻辑（可被自检以 io.Writer 复用）。
func cmsaMain(stdout, stderr io.Writer, argv []string) int {
	a, rc, msg := cmsaParseArgs(argv)
	if rc == 0 {
		fmt.Fprint(stdout, cmsaHelpText())
		return 0
	}
	if rc > 0 {
		fmt.Fprint(stderr, cmsaUsage3)
		fmt.Fprintf(stderr, "%s: error: %s\n", cmsaProg, msg)
		return 2
	}

	if a.listRules {
		fmt.Fprint(stdout, cmsaListRules)
		return 0
	}
	if a.selfTest {
		return cmsaSelfTest(stdout, stderr)
	}

	if !cmsaExists(a.fleet) {
		fmt.Fprintf(stderr, "BLOCKED：fleet.yaml 不在盘上：%s（缺件 ⇒ 不给结论）\n", a.fleet)
		return 2
	}
	b, err := os.ReadFile(a.fleet)
	if err != nil {
		fmt.Fprintf(stderr, "BLOCKED：解析失败：%s\n", err)
		return 2
	}
	if !utf8.Valid(b) {
		fmt.Fprintf(stderr, "BLOCKED：解析失败：%s\n", "'utf-8' codec can't decode byte (invalid utf-8)")
		return 2
	}
	models := cmsaParseModels(cmsaNormalizeNewlines(string(b)))
	if len(models) == 0 {
		fmt.Fprint(stderr, "BLOCKED：解析不出 models: 段（不给结论）\n")
		return 2
	}

	results := cmsaCheck(models, "")
	anchorRC := cmsaVerdict(results)

	registryDir := a.registryDir
	if !a.registryDirOn {
		registryDir = cmsaDefaultRegistryDir()
	}
	f2r, _, ok := cmsaLoadRegistryEntries(a.idmap)
	legRC := 0
	var legFindings []string
	nWith := 0
	if !ok {
		legRC = 2
		legFindings = []string{fmt.Sprintf(
			"E 真源 model-id-map 读不到/解析不出：%s ⇒ 接线判不了（不给结论）", a.idmap)}
	} else {
		regIDs, regSet := cmsaRegistryRecordIds(registryDir)
		legRC, legFindings, nWith = cmsaCheckRegistryWiring(
			models, f2r, regIDs, regSet, cmsaRegistryWiringRatchet, registryDir)
	}
	rcAll := cmsaCombineRc(anchorRC, legRC)

	nOK, nBad, nSkip := 0, 0, 0
	for _, r := range results {
		switch r.kind {
		case "OK":
			nOK++
		case "MISMATCH", "BAD":
			nBad++
		case "SKIP":
			nSkip++
		}
	}

	if a.jsonOut {
		items := make([]cmsaJSONItem, 0, len(results))
		for _, r := range results {
			var anchor interface{}
			if r.kind == "BAD" {
				anchor = r.anchorStr
			} else {
				anchor = r.anchorNum
			}
			var actual interface{}
			if r.actualSet {
				actual = r.actualNum
			}
			items = append(items, cmsaJSONItem{
				Name: r.name, Idx: r.idx, Host: r.host, File: r.file,
				Anchor: anchor, Actual: actual, Kind: r.kind,
			})
		}
		if legFindings == nil {
			legFindings = []string{}
		}
		out := cmsaJSONOut{
			VerdictScope: "本机",
			Rc:           rcAll,
			Anchors:      len(results),
			Ok:           nOK,
			Mismatch:     nBad,
			Skipped:      nSkip,
			Wiring: cmsaJSONWiring{
				Rc: legRC, Ratchet: cmsaRegistryWiringRatchet,
				RowsWithRegistryID: nWith, RegistryDir: registryDir, Findings: legFindings,
			},
			Items: items,
		}
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return rcAll
	}

	fmt.Fprint(stdout, "── 名册 size_bytes 锚 ⟷ 盘上现读 ＋ fleet ⟷ registry 接线（本机层 · 只读）──\n")
	fmt.Fprintf(stdout, "取源：%s\n", a.fleet)
	if len(results) > 0 {
		fmt.Fprint(stdout, cmsaRender(results))
		fmt.Fprint(stdout, "\n")
	}
	fmt.Fprintf(stdout, "锚 %d 条：对 %d · 漂移 %d · 跳过 %d\n", len(results), nOK, nBad, nSkip)
	fmt.Fprintf(stdout, "接线（E）：带 registry_id 的行 %d · 棘轮 %d · registry 目录 %s\n",
		nWith, cmsaRegistryWiringRatchet, registryDir)
	if len(legFindings) > 0 {
		mark := "⚪"
		if legRC == 1 {
			mark = "✗"
		}
		for _, m := range legFindings {
			fmt.Fprintf(stdout, "  [%s] %s\n", mark, m)
		}
	}
	switch rcAll {
	case 0:
		fmt.Fprint(stdout, "PASS：本机可读的锚全对 ＋ 接线全过（verdict_scope=本机）\n")
	case 1:
		fmt.Fprint(stdout, "FAIL：有漂移/接线缺陷（逐条见上）\n")
	default:
		fmt.Fprint(stdout, "BLOCKED：本机层给不出结论（无锚 / 一条都核不到 / 真源或 registry 目录读不到 ⇒ 缺件 ≠ 绿）\n")
	}
	return rcAll
}

// checkModelSizeAnchorGateMain —— 入口（= py `main()`）。
func checkModelSizeAnchorGateMain(stdout, stderr io.Writer, argv []string) int {
	return cmsaMain(stdout, stderr, argv)
}
