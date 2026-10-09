// eval_compare.go —— 件 scripts/evals/compare.py（97 行 · sha16 94d08f03ce1f0872）的 Go 真搬
// （批-5 载荷/探针面 · 入口 evalCompareGateMain）。
//
// 为什么有这件：批-5 是「载荷与探针」档（scripts/evals · sandbox* · wall · calib · dev · node · ui · core）。
// 本件是该档里现读**唯一零 subprocess / 零联网 / 零真 TTY / 零写面**的纯静态只读件（同档的 sandbox 探针
// 如 conn_jit/jitprobe/miniprobe/lx2-linux 一律自带 socket|mmap-RWX|ctypes，verify-two-states/negctl 真跑子进程，
// writeout-probe 自带仓外写面且不自回收，egg-prototype-pack 写仓内 —— 逐件现读后均不适格，见回执）。
//
// 本件真身速览（逐处现读 scripts/evals/compare.py · 非按件名推断）：
//
//	· 形态 = 纯静态只读：只以 r 模式读两枚 txt 并 print；零 subprocess / 零联网 / 零真 TTY / 零写面。
//	· 读面 = os.path.join(run_dir, "<layer>.txt")，layer 依次为 layer2-basic、layer3-complex；
//	  逐行 re.match(r"([✅❌]) (.+?)(?:\s+\S+)?$", line.strip())；命中则 name = group(2).strip()，
//	  键 = f"{layer}:{name}"，值 = (status == "✅", line.strip())（同键后写覆盖前写）。
//	· 遍历面 = os.listdir(base)（readdir 原序）后经 py sorted()（码点序）⇒ Go 用 os.ReadDir（按名序）
//	  再 sort.Strings；二者对 UTF-8 名同序。
//	· 输出序的决定者 = 末尾对 all_keys 的 sorted()（码点序 ⇒ Go sort.Strings）；字典插入序不进输出。
//	· 默认根定位 = argv 指定的运行目录 ∕ env ZERG_EVALS_DIR（缺省空串 ⇒ 相对 base "results/agent/runs"）。
//
// 出厂声明 (a) 默认根定位差异：本件不依赖 __file__（既不吃仓外树、也不按脚本位置上溯）⇒ 该差异面**不适用**，如实声明。
// 出厂声明 (b) 自指检查：本件确落在 core/cmd/zerg 全件扫描面内 ⇒ 逐条消解：件内零绝对路径字面量、
// 零形如「引号 + 键名 + 引号」的协议原串、零会悬空的仓内相对路径字面量（复核 0/0，见回执）。
//
// 本单边界（照实）：本件只落码、不接线 ——
//
//	· 不在 core/cmd/zerg/main.go 注册命令（✗ 不进命令树 · ✗ 不改 add_step）；
//	· 不删 scripts/evals/compare.py 原件（退场规则：对拍通过后才逐件点名退场）；
//	· 入口 evalCompareGateMain 目前无调用点（M2 的刻意状态）；
//	· ✗ 不改任何登记面 / family_*.go / 已落旧件。
//
// 入口签名（与 py 的 main() 对齐）：
//
//	func evalCompareGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py sys.argv[1:] 同形（不含程序名）
//	  · 退码  —— 0 恒（py main() 无返回值、顶层 sys.exit 缺席 ⇒ 正常路 rc=0）；1 only 读面异常（py 未捕 ⇒ 回溯 rc=1）
//	  · 输出  —— stdout 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// ecvPySpace / ecvPyNonSpace = py 正则 \s / \S 的 Unicode 同构类
// （py \s == str.isspace 集：\t\n\v\f\r · \x1c-\x1f · 空格 · \x85 · \xa0 · \u1680 · \u2000-\u200a · \u2028 \u2029 \u202f \u205f \u3000）。
// ★ Go RE2 的 \s/\S 默认 ASCII ⇒ 不照抄，改显式类（否则含 NBSP/\x1c 的行两态分叉）。
const ecvPySpace = `[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`
const ecvPyNonSpace = `[^\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

// ecvReMatch = py 的 r"([✅❌]) (.+?)(?:\s+\S+)?$"（re.match ⇒ 起始锚定；模式自带 $ ⇒ 等价全匹配）。
// ★ Go RE2 采 leftmost-first 语义（与回溯引擎一致）⇒ 子捕获与 py 逐字同。
var ecvReMatch = regexp.MustCompile(`^([✅❌]) (.+?)(?:` + ecvPySpace + `+` + ecvPyNonSpace + `+)?$`)

// ecvPyJoin 照 CPython posixpath.join 逐段规则直译（★ 不用 filepath.Join：`..` 段／绝对后段／空段三类语义差）。
func ecvPyJoin(a string, p ...string) string {
	path := a
	for _, b := range p {
		switch {
		case strings.HasPrefix(b, "/"):
			path = b
		case path == "" || strings.HasSuffix(path, "/"):
			path += b
		default:
			path += "/" + b
		}
	}
	return path
}

// ecvPyIsSpace 判 py str.isspace / 正则 \s 认的字符（★ 含 \x1c-\x1f、\x85、\xa0 —— Go unicode.IsSpace 不认 \x1c-\x1f）。
func ecvPyIsSpace(r rune) bool {
	switch {
	case r == ' ' || (r >= '\t' && r <= '\r'):
		return true
	case r >= 0x1c && r <= 0x1f:
		return true
	case r == 0x85 || r == 0xa0:
		return true
	case r == 0x1680:
		return true
	case r >= 0x2000 && r <= 0x200a:
		return true
	case r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000:
		return true
	}
	return false
}

// ecvPyStrip = py str.strip()（剥的是 py 的 Unicode 空白集）。
func ecvPyStrip(s string) string { return strings.TrimFunc(s, ecvPyIsSpace) }

// ecvPyIsLineBoundary = py str.splitlines 的行界（★ 不止 \n：含 \v \f \r \x1c-\x1e \x85 \u2028 \u2029）。
func ecvPyIsLineBoundary(r rune) bool {
	switch r {
	case '\n', '\v', '\f', '\r', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// ecvPySplitLines = py str.splitlines（尾界不产空尾项；空串 ⇒ 空切片）。
func ecvPySplitLines(s string) []string {
	out := []string{}
	cur := make([]rune, 0, 64)
	for _, r := range s {
		if ecvPyIsLineBoundary(r) {
			out = append(out, string(cur))
			cur = cur[:0]
		} else {
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}

// ecvReadText 按 py 文本模式读（universal newlines：\r\n 与裸 \r 一律折成 \n）。
func ecvReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s, nil
}

// ecvPyReprStr 复刻 py3 对 str 的 repr（单双引号择优 + \t\n\r 与不可打印转义）。
func ecvPyReprStr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == rune(quote) || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// ecvPyReprList 复刻 py str(list)—— 逐项 repr、用 ", " 连接、[] 括起（空表 ⇒ "[]"）。
func ecvPyReprList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = ecvPyReprStr(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func ecvMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ecvRunItem = py parse_run 里 results[k] 的 (bool, 原文) 二元组（原文仅存档，输出不吃它）。
type ecvRunItem struct {
	ok   bool
	line string
}

// ecvParseRun 逐字对齐 py parse_run。返回 err 仅在读面异常（py 于此未捕 ⇒ 回溯 rc=1）。
func ecvParseRun(runDir string) (map[string]ecvRunItem, error) {
	results := map[string]ecvRunItem{}
	for _, layer := range []string{"layer2-basic", "layer3-complex"} {
		path := ecvPyJoin(runDir, layer+".txt")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		content, err := ecvReadText(path)
		if err != nil {
			return nil, err
		}
		for _, raw := range ecvPySplitLines(content) {
			stripped := ecvPyStrip(raw)
			m := ecvReMatch.FindStringSubmatch(stripped)
			if m == nil {
				continue
			}
			results[layer+":"+ecvPyStrip(m[2])] = ecvRunItem{ok: m[1] == "✅", line: stripped}
		}
	}
	return results, nil
}

// evalCompareGateMain = 件入口（argv 与 py sys.argv[1:] 同形）。
func evalCompareGateMain(stdout, stderr io.Writer, argv []string) int {
	const zergEvalsEnv = "ZERG_EVALS_DIR"
	base := ecvPyJoin(os.Getenv(zergEvalsEnv), "results/agent/runs")

	if len(argv) < 2 {
		runs := []string{}
		if _, err := os.Stat(base); err == nil {
			entries, err2 := os.ReadDir(base)
			if err2 != nil {
				fmt.Fprintf(stderr, "compare: 读运行目录失败: %v\n", err2)
				return 1
			}
			for _, en := range entries {
				runs = append(runs, en.Name())
			}
			sort.Strings(runs)
		}
		fmt.Fprintf(stdout, "用法: compare.py <run1> <run2>\n")
		fmt.Fprintf(stdout, "可用运行: %s\n", ecvPyReprList(runs))
		return 0
	}

	r1, r2 := argv[0], argv[1]
	res1, err := ecvParseRun(ecvPyJoin(base, r1))
	if err != nil {
		fmt.Fprintf(stderr, "compare: 读运行件失败: %v\n", err)
		return 1
	}
	res2, err := ecvParseRun(ecvPyJoin(base, r2))
	if err != nil {
		fmt.Fprintf(stderr, "compare: 读运行件失败: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "📊 对比: %s vs %s\n", r1, r2)
	fmt.Fprintf(stdout, "%s\n", strings.Repeat("=", 50))

	allKeys := map[string]bool{}
	for k := range res1 {
		allKeys[k] = true
	}
	for k := range res2 {
		allKeys[k] = true
	}
	keys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	improved := []string{}
	regressed := []string{}
	sameList := []string{}
	for _, k := range keys {
		name := strings.SplitN(k, ":", 2)[1]
		a1, in1 := res1[k]
		a2, in2 := res2[k]
		switch {
		case in1 && in2:
			if a1.ok == a2.ok {
				sameList = append(sameList, name)
			} else if a2.ok && !a1.ok {
				improved = append(improved, name)
			} else {
				regressed = append(regressed, name)
			}
		case in2:
			improved = append(improved, name+"（新增）")
		}
	}

	if len(improved) > 0 {
		fmt.Fprintf(stdout, "⬆️ 提升项:\n")
		for _, n := range improved {
			fmt.Fprintf(stdout, "   %s\n", n)
		}
	}
	if len(regressed) > 0 {
		fmt.Fprintf(stdout, "⬇️ 退化项（回归！）:\n")
		for _, n := range regressed {
			fmt.Fprintf(stdout, "   %s\n", n)
		}
	}
	if len(sameList) > 0 {
		fmt.Fprintf(stdout, "= 持平项:\n")
		for _, n := range sameList {
			fmt.Fprintf(stdout, "   %s\n", n)
		}
	}

	total1, total2 := len(res1), len(res2)
	ok1, ok2 := 0, 0
	for _, v := range res1 {
		if v.ok {
			ok1++
		}
	}
	for _, v := range res2 {
		if v.ok {
			ok2++
		}
	}
	fmt.Fprintf(stdout, "%s\n", strings.Repeat("=", 50))
	fmt.Fprintf(stdout, "%s: %d/%d 通过 (%.0f%%)\n", r1, ok1, total1, float64(ok1)/float64(ecvMax(total1, 1))*100)
	fmt.Fprintf(stdout, "%s: %d/%d 通过 (%.0f%%)\n", r2, ok2, total2, float64(ok2)/float64(ecvMax(total2, 1))*100)
	if len(regressed) > 0 {
		fmt.Fprintf(stdout, "⚠️ 有回归——检查退化项！\n")
	} else {
		fmt.Fprintf(stdout, "✅ 无回归——能力持平或提升\n")
	}
	return 0
}
