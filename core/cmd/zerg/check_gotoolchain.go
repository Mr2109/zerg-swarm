// check_gotoolchain.go —— 门禁件 `scripts/gates/check-gotoolchain.py` 的 **Go 真搬样板**（v2.5.14 · M2-0 第二枚）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值**真搬 Go**（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据 · 4.1）的**第二枚样板**：
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（4.1 M2-0 = 「无输入物料、纯静态检查的判据（读仓内件即可）」）：
//
//	✓ 选：只读仓内两件 `core/go.mod` 与 `agent/go.mod`（**无外部树依赖**）· 输出**定序**
//	  （模块序固定 `core` → `agent`，无目录遍历）· 退码 `0/1/2` 三档齐 · 189 行（**行数适中**）·
//	  **无人正在改**（`git status` 现读不含它）· 不属已列重件。
//	✗ 不选 `check-version-sources.py`（801 行）/ `check-marks-coverage-parity.py`（690 行）：**已列重件**。
//	✗ 不选 `check-curl-timeout.py`（167 行）：扫描根靠 `os.walk` ⇒ **遍历序非确定**（Go 的 `WalkDir`
//	  按字典序、py 的 `scandir` 按 inode 序）⇒ 不满足「输出有序」，对拍会假差。
//	✗ 不选 `check-shell-unicode-vars.py`（128 行）：默认档**带写面**（就地 `$VAR`→`${VAR}`）⇒
//	  不属「**纯**静态检查」；且要调用方显式传文件表（**输入物料**）。
//
// ⚠ 本单边界（照实）：本件**只落码、不接线** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · ✗ 不改 `add_step`）；
//	· **不**删 `scripts/gates/check-gotoolchain.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· 入口 `gotoolchainGateMain` 目前**无调用点** —— 这是 M2-0 的**刻意**状态：
//	  先证明「搬得逐字相同」，接线与命令注册是后一单的事。
//
// 入口签名（与 py 的 `main()` 逐字对齐）：
//
//	func gotoolchainGateMain(stdout, stderr io.Writer, argv []string) int
//	  · argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	  · 退码   —— 0 通过 / 1 门禁红 / 2 不给结论（与 py 三码逐字对齐）
//	  · 输出   —— stdout/stderr 与 py 逐字节对齐（见下「已对拍面 / 未覆盖面」）
//
// 已对拍面（仓外夹具两态逐字）：根参数（指定根 / 缺件 / 只缺一侧）· 成对负控（两模块同值 ⇒ 0 ·
// 高 pin ⇒ 1 · toolchain 分叉 ⇒ 1）· 未知旗标 ⇒ 2 · `--json` 无机器面拒收 ⇒ 2 · `--self-test` 三档 ·
// 真实仓现读。
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① py 用 argparse? —— **不用**（本件无 argparse）⇒ `-h/--help` **可**逐字对齐（落到「未知旗标」档）；
//	   这条与上一枚样板（check-error-kinds.py 有 argparse）不同，故此处**不列** argparse 帮助正文。
//	② 「不给根参数」时的**默认根定位**：py 从 `__file__` 所在目录（`scripts/gates`）上溯 ≤6 层找
//	   同时含 `core/` 与 `agent/` 的目录，找不到回落 `os.getcwd()`；Go 入口**无 `__file__` 语境**
//	   （未来接线后是 CLI 子命令，更没有脚本路径）⇒ 本件默认从**当前工作目录**上溯 ≤6 层、找不到回落 cwd。
//	   落在**同一真仓、cwd=仓根**时两者读数逐字相同（已对拍 C9）；但「脚本位置 ≠ cwd」时**路径推导面不同** ⇒
//	   这一面**不属逐字对齐承诺**，对拍一律显式传根。
//	③ **异常路径**：`go.mod` 存在但**读不到**（权限）/ **不是 UTF-8**（py `open(...,encoding="utf-8").read()`
//	   抛 `UnicodeDecodeError`）/ `go.mod` 是**目录**（py 抛 `IsADirectoryError`）—— py 一律**未捕获抛 traceback**
//	   ⇒ 进程退码 1（CPython「异常未捕获」口径）；本件落「读不到」⇒ 失败行 ⇒ 同样退码 1，但 **stdout/stderr
//	   正文与 py 的 traceback 不同**。
//	④ py `--self-test` 用 `subprocess` **真跑自身进程**取真退出码；本件 `--self-test` **进程内**复用同一
//	   入口取 rc（打印文案逐字对齐；差在实现手段，不在读数）—— 若 `sys.executable` 缺失/py 件被移走，
//	   py 自检会报错而本件不受影响，这一档**不属逐字对齐承诺**。
//
//	—— ②③④ 三面均**不在** M2-0 的样板对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ── 判据正则（与 py 逐条对齐 · 名字前缀 `gtt` 防包内同名）──────────────
var (
	// TOOLCHAIN = ^toolchain\s+(go[\d.]+)\s*$   （re.M）
	gttToolchainRE = regexp.MustCompile(`(?m)^toolchain\s+(go[\d.]+)\s*$`)
	// GO_DIRECTIVE = ^go\s+([\d.]+)\s*$   （re.M）
	gttGoRE = regexp.MustCompile(`(?m)^go\s+([\d.]+)\s*$`)
	// trip = ^go?(\d+)\.(\d+)(?:\.(\d+))?$
	gttTripRE = regexp.MustCompile(`^go?(\d+)\.(\d+)(?:\.(\d+))?$`)
)

// gttUsage —— py `USAGE`（逐字）。
const gttUsage = "用法：python3 scripts/gates/check-gotoolchain.py [仓库根] [--self-test]"

// gttJSONFields —— py `JSON_FIELDS`（**唯一真源**：本门无机器读面 ⇒ 空）。
var gttJSONFields = []string{}

// gotoolchainGateMain —— 入口（= py `main()`）。
func gotoolchainGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① Q-176：给了 `--json` 而本门**无机器读面** ⇒ 逐字说明后 rc=2（先于一切，含 --self-test）。
	if rc := gttRefuseJSON(stderr, argv); rc != nil {
		return *rc
	}
	// ② `--self-test` 存在即走自检（py：`if "--self-test" in argv`）。
	for _, a := range argv {
		if a == "--self-test" {
			return gttSelfTest(stdout)
		}
	}
	// ③ 未知旗标（任何以 `-` 开头的未识别项）⇒ 用法错 rc=2。
	var unknown []string
	for _, a := range argv {
		if strings.HasPrefix(a, "-") {
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "✗ 用法错（rc=2）：未知旗标 %s\n", strings.Join(unknown, " "))
		fmt.Fprintf(stderr, "  %s\n", gttUsage)
		return 2
	}

	arg := ""
	if len(argv) > 0 {
		arg = argv[0]
	}
	root := gttRepoRoot(arg)
	fmt.Fprintf(stdout, "仓库根: %s\n", root)

	mods := map[string]map[string]string{}
	var fails []string
	order := []string{"core", "agent"}
	for _, name := range order {
		p := filepath.Join(root, name, "go.mod")
		if !gttExists(p) {
			fails = append(fails, fmt.Sprintf("%s/go.mod 缺失", name))
			continue
		}
		pin, godir, err := gttParse(p)
		if err != nil {
			// py 在此处抛异常（traceback ⇒ 进程退 1）；本件落「读不到」失败行 ⇒ 同样退 1（正文另论 · 见件头③）。
			fails = append(fails, fmt.Sprintf("%s/go.mod 读不到（%v）", name, err))
			continue
		}
		mods[name] = map[string]string{"toolchain": pin, "go": godir}
		gd, pv := godir, pin
		if gd == "" {
			gd = "（缺）"
		}
		if pv == "" {
			pv = "（无）"
		}
		fmt.Fprintf(stdout, "  %-6s go=%-10s toolchain=%s\n", name, gd, pv)
		if godir == "" {
			fails = append(fails, fmt.Sprintf("%s/go.mod 没有可解析的 go 指令", name))
		}
		if pin != "" && godir != "" {
			if pa, pb, pc, okp := gttTrip(pin); okp {
				if ga, gb, gc, okg := gttTrip(godir); okg {
					if gttCmp(pa, pb, pc, ga, gb, gc) < 0 {
						fails = append(fails, fmt.Sprintf("%s：toolchain %s 低于自身 go 指令 %s", name, pin, godir))
					}
				}
			}
		}
	}
	if len(fails) > 0 || mods["core"] == nil || mods["agent"] == nil {
		for _, f := range fails {
			fmt.Fprintf(stdout, "  - %s\n", f)
		}
		fmt.Fprint(stdout, "\n结果: 失败\n")
		return 1
	}

	// pin = core 的 toolchain（若有）否则 core 的 go 指令
	pin := mods["core"]["toolchain"]
	if pin == "" {
		pin = mods["core"]["go"]
	}
	own := mods["agent"]["toolchain"]
	if own == "" {
		own = mods["agent"]["go"]
	}
	var tcs []string
	for _, name := range order {
		if m := mods[name]; m != nil && m["toolchain"] != "" {
			tcs = append(tcs, m["toolchain"])
		}
	}
	if len(gttUnique(tcs)) > 1 {
		fails = append(fails, fmt.Sprintf("两模块 toolchain 不同值：%s —— 单一真源被破坏", gttPyReprList(tcs)))
	}
	if oa, ob, oc, oko := gttTrip(own); oko {
		if pa, pb, pc, okp := gttTrip(pin); okp {
			if gttCmp(oa, ob, oc, pa, pb, pc) > 0 {
				fails = append(fails, fmt.Sprintf("agent 要求 %s 高于 pin %s —— 两份工具链会分叉", own, pin))
			}
		}
	}
	if len(fails) > 0 {
		fmt.Fprint(stdout, "\n结果: 失败\n")
		for _, f := range fails {
			fmt.Fprintf(stdout, "  - %s\n", f)
		}
		return 1
	}
	fmt.Fprintf(stdout, "\n结果: 通过——工具链 pin = %s（core 的 go 指令为单一真源；agent 钉到 %s）\n", pin, own)
	return 0
}

// gttRefuseJSON —— py `refuse_json_without_machine_face()`。
// 给了 `--json` ⇒ 逐字说明后返 2；没给 ⇒ 返 nil（原路照走）。
func gttRefuseJSON(stderr io.Writer, argv []string) *int {
	var hit []string
	for _, a := range argv {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			hit = append(hit, a)
		}
	}
	if len(hit) == 0 {
		return nil
	}
	parts := make([]string, len(hit))
	for i, h := range hit {
		parts[i] = "`" + h + "`"
	}
	fmt.Fprintf(stderr, "⛔ 本门无机器读面：%s 在本门**未实现**（缺口 Q-176）\n", strings.Join(parts, " · "))
	// gttJSONFields 为空 ⇒ 字段表固定为「无（本门只出人读面）」（唯一真源：py 的 JSON_FIELDS）。
	if len(gttJSONFields) == 0 {
		fmt.Fprintln(stderr, "   字段表：无（本门只出人读面）")
	} else {
		fs := make([]string, len(gttJSONFields))
		for i, f := range gttJSONFields {
			fs[i] = "`" + f + "`"
		}
		fmt.Fprintf(stderr, "   字段表：%s\n", strings.Join(fs, " · "))
	}
	fmt.Fprintln(stderr, "⇒ 用法错（rc=2）：**机器面缺 ≠ 零命中** —— 两态不许同形")
	rc := 2
	return &rc
}

// gttRepoRoot —— py `repo_root()`。
// 给了 arg ⇒ `abspath(arg)`；不给 ⇒ 从 cwd 上溯 ≤6 层找同时含 `core/` 与 `agent/` 的目录，
// 找不到回落 cwd（✗ 与 py 的起点不同：py 从 `__file__` 目录起 —— 见件头未覆盖面②）。
func gttRepoRoot(arg string) string {
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
	for i := 0; i < 6; i++ {
		if gttIsDir(filepath.Join(d, "core")) && gttIsDir(filepath.Join(d, "agent")) {
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

// gttParse —— py `parse()`：回 (toolchain, go 指令)，缺者空串（py 为 None）。
func gttParse(path string) (string, string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	txt := gttUniversalNewlines(string(b))
	tc, gd := "", ""
	if m := gttToolchainRE.FindStringSubmatch(txt); m != nil {
		tc = m[1]
	}
	if m := gttGoRE.FindStringSubmatch(txt); m != nil {
		gd = "go" + m[1]
	}
	return tc, gd, nil
}

// gttUniversalNewlines —— py 文本模式的通用换行翻译（`\r\n`/`\r` → `\n`）。
func gttUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// gttTrip —— py `trip()`：`go?(\d+)\.(\d+)(?:\.(\d+))?$` ⇒ (major, minor, patch)。
func gttTrip(v string) (int, int, int, bool) {
	m := gttTripRE.FindStringSubmatch(v)
	if m == nil {
		return 0, 0, 0, false
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	c := 0
	if m[3] != "" {
		c, _ = strconv.Atoi(m[3])
	}
	return a, b, c, true
}

// gttCmp —— 三元组字典序比较（-1/0/1）。
func gttCmp(a1, b1, c1, a2, b2, c2 int) int {
	t1 := [3]int{a1, b1, c1}
	t2 := [3]int{a2, b2, c2}
	for i := 0; i < 3; i++ {
		if t1[i] < t2[i] {
			return -1
		}
		if t1[i] > t2[i] {
			return 1
		}
	}
	return 0
}

// gttUnique —— 去重（保持首见序）。
func gttUnique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// gttPyReprList —— py 列表 repr：`['a', 'b']`（toolchain 值恒无引号/反斜杠 ⇒ 单引号 repr 逐字）。
func gttPyReprList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = "'" + s + "'"
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// gttExists —— py `os.path.exists`（存在即可，含目录）。
func gttExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// gttIsDir —— py `os.path.isdir`。
func gttIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// gttLastLine —— py `(out.strip().splitlines() or [""])[-1][:90]`（按字符截 90）。
func gttLastLine(out string) string {
	t := strings.TrimSpace(out)
	if t == "" {
		return ""
	}
	lines := strings.Split(t, "\n")
	last := lines[len(lines)-1]
	r := []rune(last)
	if len(r) > 90 {
		r = r[:90]
	}
	return string(r)
}

// gttSelfTest —— py `self_test()`：三档（正控 0 · 负控 1 · 用法错 2）。
// 差异（见件头未覆盖面④）：py 用 subprocess 真跑自身取 rc；本件进程内复用入口取 rc（打印文案逐字对齐）。
func gttSelfTest(stdout io.Writer) int {
	bad := 0
	fmt.Fprintln(stdout, "── check-gotoolchain.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）")
	tmp, err := os.MkdirTemp("", "gotoolchain-selftest-")
	if err != nil {
		tmp = os.TempDir()
	}
	defer os.RemoveAll(tmp)

	mk := func(name, coreBody, agentBody string) string {
		d := filepath.Join(tmp, name)
		_ = os.MkdirAll(filepath.Join(d, "core"), 0o755)
		_ = os.MkdirAll(filepath.Join(d, "agent"), 0o755)
		_ = os.WriteFile(filepath.Join(d, "core", "go.mod"), []byte(coreBody), 0o644)
		_ = os.WriteFile(filepath.Join(d, "agent", "go.mod"), []byte(agentBody), 0o644)
		return d
	}
	chk := func(label string, want, rc int, out string) {
		good := rc == want
		if !good {
			bad++
		}
		mark, tail := "✓", ""
		if !good {
			mark = "✗"
			tail = " ｜ " + gttLastLine(out)
		}
		fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
	}
	run := func(argv ...string) (int, string) {
		var o, e bytes.Buffer
		rc := gotoolchainGateMain(&o, &e, argv)
		return rc, o.String() + e.String()
	}

	clean := mk("clean", "module core\n\ngo 1.22.0\n", "module agent\n\ngo 1.22.0\n")
	rc, out := run(clean)
	chk("档① 正控：两模块同值（core 的 go 指令为 pin）", 0, rc, out)

	higher := mk("higher", "module core\n\ngo 1.22.0\n",
		"module agent\n\ngo 1.22.0\n\ntoolchain go1.23.0\n")
	rc, out = run(higher)
	chk("档② 负控：agent 要求 go1.23.0 高于 pin go1.22.0", 1, rc, out)

	diverge := mk("diverge", "module core\n\ngo 1.22.0\n\ntoolchain go1.22.0\n",
		"module agent\n\ngo 1.23.0\n\ntoolchain go1.23.0\n")
	rc, out = run(diverge)
	chk("档②′ 成对负控：两模块 toolchain 不同值（单一真源被破坏）", 1, rc, out)

	rc, out = run(clean, "--no-such-flag")
	chk("档③ 用法错：未知旗标（机器面缺/输入错 ≠ 零命中）", 2, rc, out)

	rc, out = run("--json")
	chk("档③′ 用法错：`--json` 而本门无机器读面（`Q-176` 统一口）", 2, rc, out)

	if bad == 0 {
		fmt.Fprintln(stdout, "── 自检：全过 ✓")
		return 0
	}
	fmt.Fprintf(stdout, "── 自检：**%d 档不过** ✗\n", bad)
	return 1
}
