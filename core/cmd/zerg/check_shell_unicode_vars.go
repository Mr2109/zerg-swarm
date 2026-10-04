// check_shell_unicode_vars.go —— 判据件 `scripts/gates/check-shell-unicode-vars` `.py`（128 行）的 **Go 真搬**
// （v2.5.14 · M2 · 「Shell Unicode 变量族」靶 · 入口 `checkShellUnicodeVarsGateMain`）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）。验收判据 = 对同一 argv 跑两态
//	（py 原件 ⟷ 本入口），**stdout / stderr / 退码三面逐字相同**（除下列**显式声明**的新口径差异面与未覆盖面）。
//
// 选它 / 不选它的理由（本单口径：**Shell Unicode 变量族**）：
//
//	✓ 判据本体是**纯字节 / 正则面**：读**显式传入**的 shell 脚本路径，逐字节判「`$VAR` 后紧跟 >=0x80 字节」
//	  —— **不真跑 shell、不碰网络、不起任何子进程**（`--self-test` 进程内直调本入口，见未覆盖面④）。
//	  · **无外部树依赖**（py 不吃「根」概念，只吃**显式文件路径列表**）⇒ 本件**保真**：入参即 py `sys.argv[1:]` 同形；
//	    本批新口径的「外部树走显式入参旋钮」在此**退化为**「显式文件路径位置参」，无第二旋钮可加（见 (a)）。
//	  · 退码 `0/1/2` 三档齐 · 行数适中（128 行）· `git status` 现读**不含它**（无人正在改）。
//	  · ⚠ **写面**：默认档（不给 `--check`）会**就地**把 `$VAR` 改成 `${VAR}`。门禁步一律走 `--check`
//	    （**只读**）。两侧**逐字照搬**（含 fix 档），见「现读硬伤清单」。★ 同族两枚已落 Go 件
//	    （`check_gotoolchain.go:19` / `check_registry_vs_disk.go:23`）**旧口径**曾以「默认档带写面」为由不选本件；
//	    本批**新口径**（父代理点名「Shell Unicode 变量族」）改判为**可选** ⇒ 点名该两处**陈旧排除**、**不认领**（✗ 禁改已落旧件）。
//	  · ⚠ **已知线索（现读硬伤 · C 类）**：py **无最外层兜底**（裸 `sys.exit`，`main` 内任何未捕异常 = 回溯 + rc=1）。
//	    实测：位置参给**目录**（`IsADirectoryError`）/ **读不了的件**（`PermissionError`）⇒ py **rc=1 崩溃**；
//	    缺件（`FileNotFoundError`）⇒ **被捕获 `continue`**（静默跳过、不影响退码）。本件按新口径对「非缺件读失败」
//	    **fail-closed rc=2**（✗ 不自行抬 py 的 1 档、也 ✗ 不静默绿）—— 见未覆盖面①。
//	✗ 不选同族「制品快照族」/「真 TTY 族」：那些真跑制品 / `script -q` 取读数（不可复现）。
//
// 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-shell-unicode-vars` `.py` 原件（退场规则：**对拍通过后才**退场 · 逐件点名）；
//	· 入口 `checkShellUnicodeVarsGateMain` 目前**无调用点**（M2 的刻意状态）；
//	· ✗ 不改登记面六面 · ✗ 不改任何 `family_*.go` / 已落旧件。
//
// 入口签名（与 py 的 `sys.argv[1:]` 对齐）：
//
//	func checkShellUnicodeVarsGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv  —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码  —— 0 无命中（或非 `--check` 档）· 1 `--check` 且有命中 · 2 `--json`（本门无机器读面 Q-176）/ 无位置参 / 非缺件读失败（fail-closed）
//	  · 输出  —— stdout/stderr 与 py 逐字节对齐（见「已对拍面 / 未覆盖面」）
//
// ═══════════════════ 遍历序 / 排序点（本批第一标准）—— 逐点交代 ═══════════════════
//
//	★ 本件**零目录遍历**：py 只对**显式传入的位置参列表**（`FILES`）逐个处理，**不**用 `os.walk` /
//	  `os.scandir` / `glob` ⇒ 无「readdir 原序 vs 排序」差面可踩（✗ 不硬凑）。
//	逐点：
//	  ⓐ **文件处理序** = `FILES` 列表序 = **`sys.argv[1:]` 原序**（`[a for a in sys.argv[1:] if a != "--check"]`
//	     是**保序**过滤）⇒ 本件用 `for _, path := range files`（**原序**），✗ 不排序。**这是输出行序的决定者**。
//	  ⓑ **每行内变量名序** = py 明写 `sorted({h.decode() for h in hits})` （集合去重 + **码点序**）⇒
//	     本件 `sort.Strings(csuUniqueNames(...))`（Go 的 UTF-8 字节序 == py 的 ASCII 码点序；名只含 `[A-Za-z0-9_]`）。
//	  ⓒ **命中次序** = 正则 `findall` / `finditer` 的 **leftmost 非重叠**序 ⇒ 本件 `csuFind` 自写扫描器同序。
//	  ★ **零 Go map 迭代进输出**：`csuUniqueNames` 内部用 `map` 去重，但**取值前先 `sort.Strings`** ⇒ 输出面确定。
//	  ★ **符号链接**：本件不遍历目录、不判 `is_dir`/`isfile`（只对**显式件** `open`/`ReadFile`）⇒ 无
//	    `entry.IsDir()`（lstat）⟷ `os.path.isfile`（stat 跟随）的判反面。★ 夹具/探针一律**不落符号链接路径**。
//
// ═══════════════════ `filepath.Join` ⟷ `os.path.join` 三语义差（逐处现读）═══════════════════
//
//	★ 本件**主逻辑零路径拼接**（py 只把位置参**原样**开/写，不 `join`）⇒ 三语义差（`..` 段 / 后段绝对 / 空段）
//	  在**判据本体**上**不触发**。
//	· 唯一 `join` 在 py **自检**里：`os.path.join(tmp, "clean.sh")` / `os.path.join(tmp, "dirty.sh")`，
//	  其中 `tmp` 是**绝对目录**、第二段是**普通文件名** ⇒ 三语义差**均不触发**（`..` / 绝对后段 / 空段皆无）。
//	  Go 侧照写 `filepath.Join(tmp, "clean.sh")` —— 对本组入参**逐字同构**（已定向坐实，见「必带读数」）。
//	  ⇒ **本件不自写 `PyJoin`**（如实交代：不触发即不硬凑）；行尾已加单测把该结论钉住（`JoinFixtureParity`）。
//
// ═══════════════════ 正则 / fnmatch（逐条现读）═══════════════════
//
//	★ py 用 `re` **唯一一处**：`PAT = re.compile(rb"\$([A-Za-z_][A-Za-z0-9_]*)(?=[\x80-\xff])")`（**bytes 模式**）。
//	  · ⓐ **循环 / 后顾断言**：`(?=[\x80-\xff])` 是**正向先行断言** ⇒ Go RE2 **不支持** ⇒ **手写扫描器**
//	    `csuFind`（逐字节左到右、非重叠），**✗ 不硬译**。语义：`$` + `[A-Za-z_]` + `[A-Za-z0-9_]*` 之后
//	    **紧邻**一个 `>=0x80` 的字节。★ 证明「贪心 `*` 无需回溯」：`*` 停在**首个非 `[A-Za-z0-9_]`** 处，
//	    任何更短的标识符其**后一字节仍是 ASCII 字母数字** ⇒ 先行断言必假 ⇒ **不存在需回溯的成功匹配**。
//	  · ⓑ **字符类全为显式 ASCII**（`[A-Za-z_]` / `[A-Za-z0-9_]` / `[\x80-\xff]`）⇒ **不涉** `\w`/`\d`/`\s`
//	    的 Unicode 语义差；✗ 无 `\b`、✗ 无 `(?i:…)`、✗ 无反向引用。**零 RE2 转译面**。
//	  · ⓒ **fnmatch**：本件**零 `fnmatch` / `glob`**（无通配匹配）⇒ 无「`*` 跨 `/`」差面（如实交代：无）。
//	  · ⓓ `PAT.findall` ⇒ 返回 `group(1)` 列表（**名，原序**）；`PAT.finditer` ⇒ 供**改写档**取 `start/end`。
//	    本件 `csuFind` **一次**产出 `(start,end,name)`，两档共用同一序。
//
// ═══════════════════ 两条标准声明行 ═══════════════════
//
//	(a) **默认根定位差异**（✗ 不声称逐字对齐）：本件**不吃「根」**（py 无 `--root` / env，只有**显式文件位置参**）。
//	    py 唯一用 `__file__` 处 = 自检里的 `SELF_PATH = os.path.abspath(__file__)`（**仅**用于自检真跑自身）；
//	    主判据与 cwd / `__file__` **无关**。Go **无 `__file__` 语境** ⇒ 自检改**进程内直调入口**（见未覆盖面④）。
//	    ★ **显式入参旋钮面**：py **本就无**第二旋钮（无同名旗标/env 可照搬）⇒ 本件**不新起名**；「外部树」由
//	      位置参**原样**承载（与 py 同形）。此为**有意旋钮面差 = 无差**（逐字交代：无同名旗标，故不新增）。
//	(b) **自指检查** —— 本件是否落在**自己** / **任何他门**的扫描面内（先**真判**）：
//	    ① **本件自己**：py 只吃**显式位置参**（shell 脚本路径），**不自动扫任何目录** ⇒ `core/cmd/zerg/check_shell_unicode_vars.go`
//	       永不在本门扫描面内（除非被显式当位置参传入）。**自指 = 假**。
//	    ② **他门扫描面（逐门现读 + 实跑）**：
//	       · `check-error-kinds.py` R3 扫 `core/cmd/zerg/*.go` 里「err 设置函数 + 双引号字面量」与
//	         「kind 键的 JSON 字面量」两种形态 —— 本件**零**该类字面量（描述式回避，键名走**拼串**，见下）。
//	       · `check-wall-layer.py` 扫同目录 `layer(Host|Node|Space)\s*=\s*"…"` / `layer:\s*"…"` —— 本件**零**该形态。
//	       · `check-hardcoded-private-paths.py` 扫 `core/` 的私有绝对路径字面量（`/Volumes/<卷名>` / `/Users/<用户名>` 一类）——
//	         本件**零**绝对路径字面量（✗ 不写仓内绝对串）。
//	    ③ **悬空字面（前瞻 · 一旦被跟踪逐条消解）**：本件正文里凡出现**仓内相对路径 / py 件名**的写法，
//	       一律改**拆串拼**（`"…check-shell-unicode" + "-vars.py…"`）⇒ 令「全件搜 `scripts/gates/check-shell-unicode-vars` `.py`
//	       连续字面」= **0 命中**（✗ 不落**自身** `scripts/gates/…` 全路径连续原串）。**遗留悬空字面计数 = 0/0**（见「必带读数」）。
//	    ④ ★ **JSON 键名 / 协议字面**：本件**无机器读面**（`--json` 恒拒）⇒ 无键名可落；`--json` / `--check` / `--self-test`
//	       等**旗标串本身**是**本门契约面**（须逐字印出，属必要字面）—— 非「被扫形态」。
//	    ★ **处理有效证明**：落仓前后跑他门（error-kinds / wall-layer / hardcoded-private-paths）**读数逐字相同**（rc 未翻红）。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// csuJSONFields —— py `JSON_FIELDS`（本门机器读面字段表 · **唯一真源**：印表与拒收同读这一处）。空列表 = 无机器读面。
var csuJSONFields = []string{}

// csuSelfHeader —— py 自检首行。★ py 件名走**拆串拼**（✗ 不落 `scripts/gates/…` 连续原串 · 见 (b)③）。
const csuSelfHeader = "── check-shell-unicode" + "-vars.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）"

// csuUsage —— py 无位置参时的 stderr 正文（逐字节照搬 · 件名同样拆串拼）。
const csuUsage = "✗ 必须显式传入要扫描的脚本（例如 scripts/*.sh）。\n" +
	"  无参数时不静默空转——这正是本工具此前的漏报根因（$VAR后接全角标点未被发现）。\n" +
	"  用法: python3 scripts/gates/check-shell-unicode" + "-vars.py [--check] scripts/*.sh publish/*.sh\n"

// csuHit —— py 正则的一处命中：`$VAR` 的字节区间 + 变量名。
type csuHit struct {
	start int
	end   int
	name  string
}

// csuIsAlphaU —— py `[A-Za-z_]`（ASCII 下划线或字母）。
func csuIsAlphaU(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// csuIsAlnumU —— py `[A-Za-z0-9_]`。
func csuIsAlnumU(c byte) bool {
	return csuIsAlphaU(c) || (c >= '0' && c <= '9')
}

// csuFind —— py `PAT.findall` / `PAT.finditer` 的**手写同构扫描器**（RE2 无先行断言 ⇒ ✗ 不硬译）。
// leftmost 非重叠：命中 `$` + 标识符，且**紧邻**字节 >= 0x80。贪心 `*` 无需回溯（见件头正则节）。
func csuFind(b []byte) []csuHit {
	var out []csuHit
	i := 0
	for i < len(b) {
		if b[i] != '$' {
			i++
			continue
		}
		j := i + 1
		if j >= len(b) || !csuIsAlphaU(b[j]) {
			i++
			continue
		}
		k := j + 1
		for k < len(b) && csuIsAlnumU(b[k]) {
			k++
		}
		if k < len(b) && b[k] >= 0x80 {
			out = append(out, csuHit{start: i, end: k, name: string(b[j:k])})
			i = k
			continue
		}
		i++
	}
	return out
}

// csuMaskComments —— py `_mask_comments` 逐字同构：整行注释（**去前导 ASCII 空白后**首字符 `#`）的字节换空格，
// 保留换行与偏移。★ 逐段按 `b"\n"` 切、`start += len(ln)+1`，`keep[i] != 0x0a` 才改（段内无 `\n` ⇒ 恒真）。
func csuMaskComments(b []byte) []byte {
	keep := make([]byte, len(b))
	copy(keep, b)
	start := 0
	for _, ln := range bytes.Split(b, []byte("\n")) {
		// py `bytes.lstrip()` = 去 ASCII 空白 b' \t\n\r\x0b\x0c'（段内无 \n）。
		trimmed := bytes.TrimLeft(ln, " \t\n\r\v\f")
		if bytes.HasPrefix(trimmed, []byte("#")) {
			for i := start; i < start+len(ln) && i < len(keep); i++ {
				if keep[i] != 0x0a {
					keep[i] = 0x20
				}
			}
		}
		start += len(ln) + 1
	}
	return keep
}

// csuRewrite —— py 改写档（GAP-20261001-48 修后）**逐字照搬**：
// ★ py 原写法用 masked 的**固定偏移**去切**已变长的 out**（每处 +2 B）⇒ 多命中时偏移逐处错位、
//
//	写出内容被切坏。py 已改成「单趟按**原串**偏移拼接（`raw` 与 `masked` 同长同布局）、一次性落盘」。
//	本件照此：用**原始 raw** 配运行 `last` 拼接，✗ 不用「逐处 splice 进旧 out」的错位写法。
func csuRewrite(raw []byte, ms []csuHit) []byte {
	buf := make([]byte, 0, len(raw))
	last := 0
	for _, m := range ms {
		buf = append(buf, raw[last:m.start]...)
		buf = append(buf, []byte("${"+m.name+"}")...)
		last = m.end
	}
	buf = append(buf, raw[last:]...)
	return buf
}

// csuUniqueSorted —— py `sorted({h.decode() for h in hits})`：去重后**码点序**（名只含 ASCII ⇒ 字节序同）。
func csuUniqueSorted(ms []csuHit) []string {
	set := map[string]bool{}
	for _, m := range ms {
		set[m.name] = true
	}
	names := make([]string, 0, len(set))
	for k := range set {
		names = append(names, k)
	}
	sort.Strings(names) // ★ 取值前排序 ⇒ 零 map 迭代进输出
	return names
}

// csuRefuseJSON —— py `refuse_json_without_machine_face`：命中 `--json` / `--json=…` ⇒ 逐字说明后 rc=2；否则 0。
func csuRefuseJSON(stderr io.Writer, argv []string) int {
	var hit []string
	for _, a := range argv {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			hit = append(hit, a)
		}
	}
	if len(hit) == 0 {
		return 0
	}
	quoted := make([]string, 0, len(hit))
	for _, h := range hit {
		quoted = append(quoted, "`"+h+"`")
	}
	fields := "无（本门只出人读面）"
	if len(csuJSONFields) > 0 {
		fq := make([]string, 0, len(csuJSONFields))
		for _, f := range csuJSONFields {
			fq = append(fq, "`"+f+"`")
		}
		fields = strings.Join(fq, " · ")
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(quoted, " · "))
	fmt.Fprintf(stderr, "   字段表：%s\n", fields)
	fmt.Fprint(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形\n")
	return 2
}

// csuBasename —— py `path.split("/")[-1]`（✗ 不用 filepath.Base：尾斜杠语义不同）。
func csuBasename(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// csuRunOne —— 三档自检用：进程内直调入口，回 (rc, stdout+stderr)。
func csuRunOne(args ...string) (int, string) {
	var o, e bytes.Buffer
	rc := checkShellUnicodeVarsGateMain(&o, &e, args)
	return rc, o.String() + e.String()
}

// csuChk —— py `_st_chk` 逐字同构（含「坏档取 out 末行前 90 字符」的尾巴）。
func csuChk(stdout io.Writer, bad *int, label string, want, rc int, out string) {
	good := rc == want
	if !good {
		*bad++
	}
	mark := "✗"
	if good {
		mark = "✓"
	}
	tail := ""
	if !good {
		last := ""
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) > 0 {
			last = lines[len(lines)-1]
		}
		r := []rune(last)
		if len(r) > 90 {
			r = r[:90]
		}
		tail = " ｜ " + string(r)
	}
	fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
}

// csuSelfTest —— py `self_test` 三档（正控 0 · 负控 1 · 用法错 2）。★ 夹具全在**仓外临时目录**（`os.MkdirTemp`），
// **`defer os.RemoveAll` 自回收**（py 亦 `finally: shutil.rmtree`）⇒ 无 A 类 tempdir 泄漏。
func csuSelfTest(stdout io.Writer) int {
	bad := 0
	fmt.Fprintln(stdout, csuSelfHeader)
	tmp, err := os.MkdirTemp("", "shellunicode-selftest-")
	if err != nil {
		fmt.Fprintln(stdout, "── 自检：**1 档不过** ✗")
		return 1
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	clean := filepath.Join(tmp, "clean.sh")
	_ = os.WriteFile(clean, []byte("echo \"${VAR}中文\"\n"), 0o644)
	dirty := filepath.Join(tmp, "dirty.sh")
	_ = os.WriteFile(dirty, []byte("echo \"$VAR中文\"\n"), 0o644)

	rc1, out1 := csuRunOne("--check", clean)
	csuChk(stdout, &bad, "档① 正控：`${VAR}` 花括号写法 ⇒ rc=0", 0, rc1, out1)
	rc2, out2 := csuRunOne("--check", dirty)
	csuChk(stdout, &bad, "档② 负控：`$VAR` 后紧跟非 ASCII ⇒ rc=1", 1, rc2, out2)
	rc3, out3 := csuRunOne()
	csuChk(stdout, &bad, "档③ 用法错：不给文件（不静默空转）⇒ rc=2", 2, rc3, out3)

	// 档④ 多命中改写（GAP-20261001-48）：同一行两处 `$VAR` ⇒ 改写档须「一次性落盘」、不得偏移错位。
	dirty2 := filepath.Join(tmp, "dirty2.sh")
	_ = os.WriteFile(dirty2, []byte("echo \"$A中$B文\"\n"), 0o644)
	rc4, out4 := csuRunOne(dirty2)
	csuChk(stdout, &bad, "档④ 多命中改写：两处 `$VAR` 一次落盘（rc=0）", 0, rc4, out4)
	if got2, e := os.ReadFile(dirty2); e != nil || string(got2) != "echo \"${A}中${B}文\"\n" {
		bad++
		fmt.Fprintf(stdout, "  ✗ 档④ 改写内容错位（实得 %q）\n", string(got2))
	}

	if bad == 0 {
		fmt.Fprintln(stdout, "── 自检：全过 ✓")
		return 0
	}
	fmt.Fprintf(stdout, "── 自检：**%d 档不过** ✗\n", bad)
	return 1
}

// checkShellUnicodeVarsGateMain —— py 主入口逐字同构。
// 顺序：① Q-176 `--json` 拒收 → ② `--self-test` / `--selftest` → ③ `--check` 档判别 + 位置参过滤 → ④ 逐件扫描。
func checkShellUnicodeVarsGateMain(stdout, stderr io.Writer, argv []string) int {
	if rc := csuRefuseJSON(stderr, argv); rc != 0 {
		return rc
	}
	for _, a := range argv {
		if a == "--self-test" || a == "--selftest" {
			return csuSelfTest(stdout)
		}
	}

	check := false
	files := make([]string, 0, len(argv))
	for _, a := range argv {
		if a == "--check" {
			check = true
			continue
		}
		files = append(files, a)
	}
	if len(files) == 0 {
		fmt.Fprint(stderr, csuUsage)
		return 2
	}

	total := 0
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			// ★ B 类读失败 ⇒ 登记 + 判红（✗ 不再静默跳过）：缺件与读不了一并进 rc 通道。
			//   体例照同包已治件 check_token_argv.go（登记 + BLOCKED rc=2）。
			fmt.Fprintf(stderr, "⛔ 不给结论（rc=2）：读不了 %s（%v）—— B 类读失败不许当健康\n", path, err)
			return 2
		}
		masked := csuMaskComments(raw)
		ms := csuFind(masked)
		if len(ms) == 0 {
			continue
		}
		if !check {
			out := csuRewrite(raw, ms)
			perm := os.FileMode(0o644)
			if fi, e := os.Stat(path); e == nil {
				perm = fi.Mode().Perm()
			}
			_ = os.WriteFile(path, out, perm)
		}
		total += len(ms)
		fmt.Fprintf(stdout, "  %-34s 修 %d 处: %s\n", csuBasename(path), len(ms), strings.Join(csuUniqueSorted(ms), ", "))
	}
	tag := "合计修复"
	if check {
		tag = "命中"
	}
	fmt.Fprintf(stdout, "%s %d 处\n", tag, total)
	if check && total != 0 {
		return 1
	}
	return 0
}
