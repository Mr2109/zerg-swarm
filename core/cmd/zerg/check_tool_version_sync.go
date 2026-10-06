// check_tool_version_sync.go —— 判据件 `scripts/gates/check-tool-version-sync.sh`（140 行 · `scripts/gates/`
// 现存 3 件「名带 check-* 的真判据」之一）的 **Go 真搬**（M2 一枚 · 入口 `checkToolVersionSyncGateMain` ·
// 承接 GAP-20261001-13）。
//
// 本件只做一件事：现读 `<root>/tools/versions.json` 的台账键值，与 `<root>/tools/*.md` 各工具文档
// 抬头行（前 20 行内第一处「版本 + 冒号 + vX.Y.Z」）逐键对拍。退码 0 / 1 / 2：
//
//	0 全一致（台账键 ∪ 盘上 tools/*.md 的抬头全部对得上，且无「判不了」）
//	1 有抬头不一致 / 缺抬头（默认逐条打印；`--quiet` 只打汇总）
//	2 判不了（tools/versions.json 不在盘 / 台账键无 version 值 / 未知参数 / 自检失败）
//
// 声明 (a) 默认根定位差异：py 本件从 `${BASH_SOURCE[0]}` 上溯两层取根（= 脚本位置）
//
//	⟷ Go 无脚本位置语境，改取**进程 cwd**。两者仅当「脚本位置 == cwd」时逐字同构；否则路径
//	推导面不同（✗ 不声称逐字对齐）。
//
// 声明 (b) 自指：本判据扫描面 = `<root>/tools/versions.json` ∪ `<root>/tools/*.md`，本件落
//
//	`core/cmd/zerg/` ⇒ **不落在自身扫描面内**；件内无私有绝对路径字面、无解析后会悬空的文档树字面。
//
// 外部交互点（逐行读完 py 后列全）：**仅**「读 tools/versions.json」「列 tools/*.md」「读各 md 前 20 行」
// 三项 —— 纯静态只读：零子进程 / 零联网 / 零 TTY / 零计时 / 零随机。py 的 `python3 <<'PY'` 是
// **解释器边界**（其内嵌逻辑同样零子进程）⇒ 属「确定读面」，適格成立；Go 侧把该逻辑全部原生化，
// 连解释器都不起。
//
// 遍历 / 排序点：`md_names = sorted(名去扩展)` ⟷ Go `sort.Strings`（合法 UTF-8 下码点序 ≡ 字节序）；
// `keys` 走 `sorted(tools)`（台账键序）；输出行序决定者 = 末尾 `sort.Strings(scan)`（∪ 去重后排序）。
// 逐块遍历一律按已排序的 `scan`，零 map 迭代进输出。
//
// 空值三形态：本门**无机器读面**（py 无 `--json`，键表为空）⇒ 不适用（如实声明，✗ 不硬造格）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	ctvsProg         = "check-tool-version-sync.sh"
	ctvsVerWord      = "版本"
	ctvsColon        = "："
	ctvsToolsSub     = "tools"
	ctvsVersionsName = "versions.json"
	ctvsSummaryFmt   = "  台账键 = %d ｜ 盘上 tools/*.md = %d ｜ 扫描面(并) = %d ｜ 抬头一致 = %d ｜ 抬头不一致 = %d ｜ 缺抬头 = %d"
	ctvsBadHdr       = "  ── 不一致（台账为准；改抬头前先由 Mr2109 定版本值，勿自动顺延）──"
	ctvsBadFmt       = "    %-22s 台账=%-8s 抬头=%s"
	ctvsNoheadHdr    = "  ── 缺抬头版本行（需补，版本值待定）──"
	ctvsNoheadFmt    = "    %-22s 台账=%s"
	ctvsUnlHdr       = "  ── 报告·盘上有而台账无（tools/*.md 在盘、versions.json 无此键 ⇒ 待补键或删件）──"
	ctvsUnlFmt       = "    %-22s"
	ctvsNovHdr       = "  ── 判不了·台账值缺失（键在册但无 version 值 ⇒ 不给结论）──"
	ctvsNovFmt       = "    %-22s"
	ctvsSTHdr        = "── 自检（合成夹具 · 不碰真目标）──"
	ctvsOKLine       = "  ✓ 三处版本一致（台账 / 工具文档抬头）"
	ctvsVMiss        = "FATAL: 找不到 tools/versions.json"
	ctvsUnknownMsg   = "FATAL: 未知参数 "
	ctvsCaseOK       = "正控·抬头一致 ⇒ 0"
	ctvsCaseMis      = "负控·抬头不符 ⇒ 1"
	ctvsCaseNov      = "负控·台账值缺失 ⇒ 2（旧件静默跳过）"
	ctvsUnlOKLine    = "  ✓ 报告·盘上多件出声且退码不受影响（rc=0）"
)

// ctvsPyWS —— py `\s`（Unicode 语义）同构字符类：`str.isspace()` 为真的全部码点（含 \x1c-\x1f / \x85 /
// \xa0 与 Zs 族）。RE2 的 `\s` 只认 ASCII ⇒ 显式展开，✗ 不用 `\s`。
const ctvsPyWS = `[\t\n\v\f\r\x{1c}-\x{1f} \x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

// ctvsPatHead —— py `pat_head` 逐字同构源：`\*{0,2}` + 版本 + `\*{0,2}` + `\s*` + `[：:]` + `\s*` + `\**`
// + `\s*` + `v(\d+\.\d+\.\d+)`；其中 `\s` 展开为 ctvsPyWS、`\d` 展开为 `\p{Nd}`（RE2 默认 ASCII）。
var ctvsPatHead = `\*{0,2}` + "版本" + `\*{0,2}` + ctvsPyWS + `*[` + "：" + `:]` + ctvsPyWS + `*\**` + ctvsPyWS + `*v(\p{Nd}+\.\p{Nd}+\.\p{Nd}+)`

// ctvsHeadRe —— 已编译抬头正则。★ 用 `var` 而非 `const`：测试件可替换它做「哑化 ⇒ 必红」有牙实证。
var ctvsHeadRe = regexp.MustCompile(ctvsPatHead)

// ctvsRoot —— 默认根取进程 cwd（声明 (a)）。★ `var` 让测试件注入仓外夹具根（免 `os.Chdir` 的级联副作用）。
var ctvsRoot = func() (string, error) { return os.Getwd() }

// ── py 路径语义（自写同构 · ✗ 不用 filepath.Join/Clean/Split）────────────────────────────────

// ctvsPyJoin —— CPython `posixpath.join` 逐段规则同构：后段绝对 ⇒ 丢前段；空段保留尾 `/`；`..` 原样留。
func ctvsPyJoin(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	path := parts[0]
	for _, b := range parts[1:] {
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

// ctvsSplitext —— CPython `posixpath.splitext` 同构（前导点不算扩展名分隔）。
func ctvsSplitext(p string) (string, string) {
	sepIndex := strings.LastIndex(p, "/")
	dotIndex := strings.LastIndex(p, ".")
	if dotIndex > sepIndex {
		for fi := sepIndex + 1; fi < dotIndex; fi++ {
			if p[fi] != '.' {
				return p[:dotIndex], p[dotIndex:]
			}
		}
	}
	return p, ""
}

// ── py 文本语义（自写同构）──────────────────────────────────────────────────────────────────

// ctvsReadText —— py `io.open(p, encoding="utf-8").read()` 同构：UTF-8 解码 + universal newlines
// （`\r\n` 与裸 `\r` 一律折成 `\n`）。非法 UTF-8 / 读失败 ⇒ 返回 false（py 此处为未捕 UnicodeDecodeError
// 崩溃 rc=1；本件 fail-closed rc=2 · 见未覆盖面）。
func ctvsReadText(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	if !utf8.Valid(b) {
		return "", false
	}
	s := string(b)
	if strings.Contains(s, "\r") {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.ReplaceAll(s, "\r", "\n")
	}
	return s, true
}

// ctvsIsLineBoundary —— py `str.splitlines()` 的切分边界集。
func ctvsIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// ctvsSplitLines —— py `str.splitlines()` 同构（上游已折 universal newlines）：尾边界不追加空元素。
func ctvsSplitLines(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if ctvsIsLineBoundary(r) {
			out = append(out, s[start:i])
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// ctvsHeadOf —— 前 20 行内第一处抬头版本（`vX.Y.Z`）。找不到 ⇒ ("", false)。
func ctvsHeadOf(text string) (string, bool) {
	lines := ctvsSplitLines(text)
	if len(lines) > 20 {
		lines = lines[:20]
	}
	for _, line := range lines {
		if m := ctvsHeadRe.FindStringSubmatch(line); m != nil {
			return "v" + m[1], true
		}
	}
	return "", false
}

// ── 台账（versions.json）解析 ────────────────────────────────────────────────────────────────

type ctvsLedger struct {
	isDict bool
	bad    bool
	dict   map[string]any
	list   []any
}

// ctvsLedgerFrom —— py `tools = d.get("tools", d) if isinstance(d, dict) else d` 同构。
func ctvsLedgerFrom(d any) *ctvsLedger {
	var tools any
	if dd, ok := d.(map[string]any); ok {
		if t, ok2 := dd["tools"]; ok2 {
			tools = t
		} else {
			tools = dd
		}
	} else {
		tools = d
	}
	switch t := tools.(type) {
	case map[string]any:
		return &ctvsLedger{isDict: true, dict: t}
	case []any:
		return &ctvsLedger{list: t}
	default:
		return &ctvsLedger{bad: true}
	}
}

// ctvsTruthy —— py 真值语义（"" / 0 / [] / {} / None 为假）。
func ctvsTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return t != ""
	case bool:
		return t
	case json.Number:
		f, err := t.Float64()
		return err == nil && f != 0
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	default:
		return true
	}
}

// keys —— py `sorted(tools)`（dict 形）或 `[t.get("name") …]`（list 形），再去掉假值。
func (l *ctvsLedger) keys() []string {
	var ks []string
	if l.isDict {
		for k := range l.dict {
			ks = append(ks, k)
		}
		sort.Strings(ks)
	} else {
		for _, t := range l.list {
			m, ok := t.(map[string]any)
			if !ok {
				continue
			}
			name, ok := m["name"]
			if !ok {
				continue
			}
			s, ok := name.(string)
			if !ok {
				s = ""
			}
			ks = append(ks, s)
		}
	}
	out := make([]string, 0, len(ks))
	for _, k := range ks {
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}

// verOf —— py `ver_of(name)` 同构：返回 (值, 是否判得了)。无值 / 值缺失 ⇒ (_, false) ⇒ 落「判不了」档。
func (l *ctvsLedger) verOf(name string) (string, bool) {
	if l.isDict {
		x, ok := l.dict[name]
		if !ok || x == nil {
			return "", false
		}
		if s, ok := x.(string); ok {
			return s, true
		}
		if m, ok := x.(map[string]any); ok {
			var val any
			if a, aOK := m["version"]; aOK && ctvsTruthy(a) {
				val = a
			} else if b, bOK := m["ver"]; bOK {
				val = b
			}
			if val == nil {
				return "", false
			}
			if s, ok := val.(string); ok {
				return s, true
			}
			return "", false
		}
		return "", false
	}
	for _, t := range l.list {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		nm, ok := m["name"]
		if !ok {
			continue
		}
		if s, ok := nm.(string); ok && s == name {
			v, ok := m["version"]
			if !ok {
				return "", false
			}
			if vs, ok := v.(string); ok {
				return vs, true
			}
			return "", false
		}
	}
	return "", false
}

// ── 扫描面：盘上 tools/*.md ─────────────────────────────────────────────────────────────────

// ctvsMdStems —— py `glob.glob("<root>/tools/*.md")` + basename + splitext + sorted 同构：
// 隐藏件（前导 `.`）不匹（glob `*` 语义）、只要文件名以 `.md` 收尾（含目录，faithful）。
func ctvsMdStems(root string) ([]string, bool) {
	ents, err := os.ReadDir(ctvsPyJoin(root, ctvsToolsSub))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, true
		}
		return nil, false
	}
	stems := []string{}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") {
			continue
		}
		stem, _ := ctvsSplitext(name)
		stems = append(stems, stem)
	}
	sort.Strings(stems)
	return stems, true
}

// ── 主体 ────────────────────────────────────────────────────────────────────────────────────

func ctvsIndent(w io.Writer, out string) {
	for _, line := range strings.Split(out, "\n") {
		fmt.Fprintf(w, "      %s\n", line)
	}
}

// ctvsCheck —— check_root 同构：打印报告，返 0 / 1 / 2。root 由调用方给（entry 取 cwd）。
func ctvsCheck(root string, quiet bool, stdout, stderr io.Writer) int {
	vpath := ctvsPyJoin(root, ctvsToolsSub, ctvsVersionsName)
	if fi, err := os.Stat(vpath); err != nil || !fi.Mode().IsRegular() {
		fmt.Fprintln(stdout, ctvsVMiss)
		return 2
	}
	b, err := os.ReadFile(vpath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 判不了（台账读不了）⇒ 不给结论（退码 2）\n", ctvsProg)
		return 2
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var d any
	if err := dec.Decode(&d); err != nil {
		fmt.Fprintf(stderr, "%s: 判不了（台账不是合法 JSON）⇒ 不给结论（退码 2）\n", ctvsProg)
		return 2
	}
	led := ctvsLedgerFrom(d)
	if led.bad {
		fmt.Fprintf(stderr, "%s: 判不了（台账顶层形态不支持）⇒ 不给结论（退码 2）\n", ctvsProg)
		return 2
	}
	keys := led.keys()
	mdNames, mok := ctvsMdStems(root)
	if !mok {
		fmt.Fprintf(stderr, "%s: 判不了（tools/ 读不了）⇒ 不给结论（退码 2）\n", ctvsProg)
		return 2
	}
	keySet := make(map[string]bool, len(keys))
	union := make(map[string]bool, len(keys)+len(mdNames))
	for _, k := range keys {
		keySet[k] = true
		union[k] = true
	}
	for _, m := range mdNames {
		union[m] = true
	}
	scan := make([]string, 0, len(union))
	for k := range union {
		scan = append(scan, k)
	}
	sort.Strings(scan)

	type ctvsRow struct{ name, led, head string }
	var bad, nohead []ctvsRow
	var unledgered, novalue []string
	okN := 0
	for _, name := range scan {
		if !keySet[name] {
			unledgered = append(unledgered, name)
			continue
		}
		ledv, present := led.verOf(name)
		if !present {
			novalue = append(novalue, name)
			continue
		}
		mdPath := ctvsPyJoin(root, ctvsToolsSub, name+".md")
		if _, err := os.Stat(mdPath); err != nil {
			bad = append(bad, ctvsRow{name, ledv, fmt.Sprintf("(缺文档 tools/%s.md)", name)})
			continue
		}
		text, rok := ctvsReadText(mdPath)
		if !rok {
			fmt.Fprintf(stderr, "%s: 判不了（读不了 %s）⇒ 不给结论（退码 2）\n", ctvsProg, mdPath)
			return 2
		}
		head, found := ctvsHeadOf(text)
		if !found {
			nohead = append(nohead, ctvsRow{name, ledv, ""})
			continue
		}
		target := ledv
		if !strings.HasPrefix(ledv, "v") {
			target = "v" + ledv
		}
		if head != target {
			bad = append(bad, ctvsRow{name, ledv, head})
		} else {
			okN++
		}
	}
	fmt.Fprintf(stdout, ctvsSummaryFmt+"\n", len(keys), len(mdNames), len(scan), okN, len(bad), len(nohead))
	if len(bad) > 0 && !quiet {
		fmt.Fprintln(stdout, ctvsBadHdr)
		for _, b := range bad {
			fmt.Fprintf(stdout, ctvsBadFmt+"\n", b.name, b.led, b.head)
		}
	}
	if len(nohead) > 0 && !quiet {
		fmt.Fprintln(stdout, ctvsNoheadHdr)
		for _, n := range nohead {
			fmt.Fprintf(stdout, ctvsNoheadFmt+"\n", n.name, n.led)
		}
	}
	if len(unledgered) > 0 && !quiet {
		fmt.Fprintln(stdout, ctvsUnlHdr)
		for _, n := range unledgered {
			fmt.Fprintf(stdout, ctvsUnlFmt+"\n", n)
		}
	}
	if len(novalue) > 0 && !quiet {
		fmt.Fprintln(stdout, ctvsNovHdr)
		for _, n := range novalue {
			fmt.Fprintf(stdout, ctvsNovFmt+"\n", n)
		}
	}
	if len(novalue) > 0 {
		return 2
	}
	if len(bad) > 0 || len(nohead) > 0 {
		return 1
	}
	return 0
}

// ctvsSelftest —— py `--self-test` 分支同构（合成夹具 · 不碰真目标）。返回 0 全过 / 2 有失败。
func ctvsSelftest(stdout io.Writer) int {
	fmt.Fprintln(stdout, ctvsSTHdr)
	base, err := os.MkdirTemp("", "tv-sync-selftest-")
	if err != nil {
		fmt.Fprintf(stdout, "自检结论: **1 格失败**\n")
		return 2
	}
	defer os.RemoveAll(base) // ★ 安全删点：仅建点成功这一支挂删除（✗ 不在失败回落支后无条件删）
	fails := 0
	mkfix := func(sub, vj, md string) string {
		d := ctvsPyJoin(base, sub)
		_ = os.MkdirAll(ctvsPyJoin(d, ctvsToolsSub), 0o755)
		_ = os.WriteFile(ctvsPyJoin(d, ctvsToolsSub, ctvsVersionsName), []byte(vj), 0o644)
		if md != "" {
			_ = os.WriteFile(ctvsPyJoin(d, ctvsToolsSub, "alpha.md"), []byte(md), 0o644)
		}
		return d
	}
	runCase := func(name string, want int, d string) {
		var buf bytes.Buffer
		rc := ctvsCheck(d, false, &buf, &buf)
		if rc == want {
			fmt.Fprintf(stdout, "  ✓ %s（rc=%d）\n", name, rc)
		} else {
			fmt.Fprintf(stdout, "  ✗ %s（rc=%d · 期望 %d）\n", name, rc, want)
			ctvsIndent(stdout, buf.String())
			fails++
		}
	}
	mdOK := "# alpha\n" + ctvsVerWord + ctvsColon + "v1.2.3\n"
	mdMis := "# alpha\n" + ctvsVerWord + ctvsColon + "v1.2.4\n"
	runCase(ctvsCaseOK, 0, mkfix("ok", `{"tools":{"alpha":"v1.2.3"}}`, mdOK))
	runCase(ctvsCaseMis, 1, mkfix("mis", `{"tools":{"alpha":"v1.2.3"}}`, mdMis))
	runCase(ctvsCaseNov, 2, mkfix("nov", `{"tools":{"alpha":{}}}`, mdOK))
	d := mkfix("unl", `{"tools":{"alpha":"v1.2.3"}}`, mdOK)
	_ = os.WriteFile(ctvsPyJoin(d, ctvsToolsSub, "beta.md"), []byte("# beta\n"+ctvsVerWord+ctvsColon+"v0.0.1\n"), 0o644)
	var buf bytes.Buffer
	rc := ctvsCheck(d, false, &buf, &buf)
	if rc == 0 && strings.Contains(buf.String(), "盘上有而台账无") {
		fmt.Fprintln(stdout, ctvsUnlOKLine)
	} else {
		fmt.Fprintf(stdout, "  ✗ 报告·盘上多件（rc=%d）\n", rc)
		fails++
	}
	if fails == 0 {
		fmt.Fprintln(stdout, "自检结论: 全过")
		return 0
	}
	fmt.Fprintf(stdout, "自检结论: **%d 格失败**\n", fails)
	return 2
}

// checkToolVersionSyncGateMain —— 本判据 Go 入口（argv **不含程序名** · stdout/stderr 由调用方给）。
func checkToolVersionSyncGateMain(stdout, stderr io.Writer, argv []string) int {
	selftest, quiet := false, false
	for _, a := range argv {
		switch a {
		case "--self-test":
			selftest = true
		case "--quiet":
			quiet = true
		default:
			fmt.Fprintf(stdout, "%s%s\n", ctvsUnknownMsg, a)
			return 2
		}
	}
	if selftest {
		return ctvsSelftest(stdout)
	}
	root, err := ctvsRoot()
	if err != nil || root == "" {
		fmt.Fprintln(stdout, ctvsVMiss)
		return 2
	}
	rc := ctvsCheck(root, quiet, stdout, stderr)
	if rc == 0 {
		fmt.Fprintln(stdout, ctvsOKLine)
	}
	return rc
}
