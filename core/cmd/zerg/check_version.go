// check_version.go —— 门禁件 `scripts/gates/check_version.py` 的 **Go 真搬**（v2.5.14 · M2-0 路C · 候选区间 q–z）。
//
// 为什么有这件（不是「顺手 Go 化」）：
//
//	v2.5.14 定案（`设计-判据 CLI 化-v1.0-20260930.md` 第四节）把判据实现面分两段走 ——
//	**M1 = 先把 `scripts/` 下的独立件都收成 CLI 打得着的统一面（转发壳）**；
//	**M2 = 按价值真搬 Go（逐批判据搬 Go · 落点 `core/cmd/zerg/`）**。
//	本件 = M2-0 批（无输入物料、纯静态检查的判据 · 4.1）中 **候选区间 q–z** 的一枚。
//	验收判据 = 对同一件跑两态（原件 ↔ 新入口），**stdout / stderr / 退码三面逐字相同**（4.2）。
//
// 选它 / 不选它的理由（4.1 M2-0 = 「无输入物料、纯静态检查的判据（读仓内件即可）」）：
//
//	✓ 选 `check_version.py`（121 行）：只读仓内两枚固定件 `ui/Cargo.toml` 与
//	  `core/internal/version/version.go`（**无外部树依赖**）· 输出**定序**（固定三行 + 结论行，
//	  无目录遍历）· 退码 `0/1/2` 三档齐（0 一致 / 1 不一致 / 2 不给结论（版本真源取不到 = 空域）或用法错）· **纯静态无写面**
//	  （默认档一字不写；`--self-test` 只写 `os.MkdirTemp` 仓外临时树）· 121 行（**行数适中**）·
//	  `git status` 现读**不含它**（无人正在改）· 不属已搬 11 枚。
//	✗ 不选 `check-tool-version-sync.sh`（60 行 · bash）：非判据件形态、跑 `go` 子进程 ⇒ 非纯静态。
//	✗ 不选 `check-signature-identity.py`（288 行）：起 `codesign` 子进程 + 判 `bin/` 产出件
//	  ⇒ 依赖外部二进制/树，且 `bin/` 属本单禁区。
//	✗ 不选 `check-wallclock-ratchet.py`（232 行）：带 `--write` 写面（改基线 + 追台账）⇒ 非「纯静态」。
//	✗ 不选 `check-unsubstituted-placeholders.py`（302 行）：扫 `docs/**` + `publish/docs/**` 树面。
//	✗ 不选 `check-token-argv.py`(370) / `check-writer-guard.py`(521) / `seal-ledger.py`(531) /
//	  `who-references.py`(397) / `two-states-gate-mutate.py`(635) / `real-gates.sh`(601) /
//	  `shard-go-test.sh`(354) / `redaction-probes.sh`(129)：读外部树 / 起子进程 / 带写面 / 行数过重。
//	✗ 不选 `repo-diffstat-check.py`（74 行）：退码含第四档 `8`（件读不到）⇒ **不满足「退码仅 0/1/2」**。
//	✗ `wall-layer`(w) / `registry-vs-disk`(r) / `show-baseline`(s)：**已搬 11 枚**（不得重选）。
//
// 本单边界（照实）：本件**已接线（inproc 表分派）** ——
//
//	· **不**在 `core/cmd/zerg/main.go` 里注册任何命令（✗ 不进命令树 · `add_step` **未挂**（`precommit-gates.sh` 现读零命中））；
//	· **不**删 `scripts/gates/check_version.py` 原件（退场规则 4.3：**对拍通过后才**退场，且逐件显式点名）；
//	· **已接线**：入口 `versionGateMain` 由 `gateCheckGoEntry["check_version"]` 分派（`zerg gate check check_version`）· 原文此处断言与现读相反（只改文案 · 未改行为）。
//
// ★ 两条标准声明行（M2-0 逐单必写）：
//
//	(a) **默认根定位差异**：py `repo_root(None)` 从 `__file__` 所在目录（`scripts/gates`）上溯 ≤6 层，
//	    取首个同时含 `core/` 与 `ui/` 的目录，找不到回落 `os.getcwd()`；Go 入口**无 `__file__` 语境**
//	    （接线后是 CLI 子命令，更没有脚本路径）⇒ 本件从**当前工作目录**上溯 ≤6 层、找不到回落 cwd。
//	    落在**同一真仓、cwd=仓根**时两者读数逐字相同（已对拍 C13/C14）；「脚本位置 ≠ cwd」时**路径推导面不同**
//	    ⇒ 该面**不属逐字对齐承诺**，对拍一律显式传根。
//	(b) **自指检查要真判**：本件**只读两枚固定件**（`ui/Cargo.toml` · `core/internal/version/version.go`），
//	    **不扫任何树面** ⇒ 自身（及其 Go 副本）**不在扫描面内** ⇒ 无「自指格」。
//	    对照血证：`check_wall_layer.go` 因扫树面含自身，须把标记反引号串**禁反引号原串直写正文**；本件不适用。
//
// 未覆盖面（✗ 不声称逐字对齐）：
//
//	① py 用 argparse? —— **不用**（本件手搓 argv 解析）⇒ `-h/--help` **可**逐字对齐（落「未知旗标」档 rc=2）。
//	② py `rust_version()` **先查存在**（★ 空集守卫 GAP-20260927-108）：`ui/Cargo.toml` 缺失 ⇒ 返 `None`
//	   ⇒ 主档「版本真源取不到」⇒ rc=2（不给结论）。本件同口径（`verExists` 缺失 ⇒ 返空串 ⇒ 同 rc=2）。
//	   仍不覆盖：件是**目录** / 非 UTF-8 —— py `open(...)` 抛异常（进程退码 1、stdout 为空）；
//	   本件落「读不到」⇒ **stdout 同样为空**、退码 1，但 **stderr 正文不同**。此档**不在**逐字对齐承诺内。
//	③ py `--self-test` 用 `subprocess` **真跑自身进程**取真退出码；本件 `--self-test` **进程内**复用同一入口
//	   取 rc（打印文案逐字对齐；差在实现手段，不在读数）。若 `sys.executable` 缺失则 py 自检报错、本件不受影响。
//
//	—— ②③ 两面不在 M2-0 样板对拍输入集内；接线前须逐面补齐或显式剔除。
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ── 判据正则（与 py 逐条对齐 · 名字前缀 `ver` 防包内同名）──────────────
var (
	// py: re.compile(r'^version\s*=\s*"([^"]+)"', re.M)
	verRustRE = regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)
	// py: re.compile(r'^const Version = "([^"]+)"', re.M)
	verGoRE = regexp.MustCompile(`(?m)^const Version = "([^"]+)"`)
)

// versionGateMain —— 入口（= py `main()`）。
//
//	· argv   —— 与 py `sys.argv[1:]` 同形（不含程序名）
//	· 退码   —— 0 一致 / 1 不一致或解析失败 / 2 用法错（与 py 三码逐字对齐）
//	· 输出   —— stdout/stderr 与 py 逐字节对齐（见件头「已对拍面 / 未覆盖面」）
func versionGateMain(stdout, stderr io.Writer, argv []string) int {
	// ① `--self-test` / `--selftest` 存在即走自检（py：`if "--self-test" in argv or "--selftest" in argv`）。
	for _, a := range argv {
		if a == "--self-test" || a == "--selftest" {
			return verSelfTest(stdout)
		}
	}
	// ② 未知旗标（任何以 `-` 开头的未识别项）⇒ 用法错 rc=2（**不许**静默当成仓根）。
	var unknown []string
	for _, a := range argv {
		if strings.HasPrefix(a, "-") {
			unknown = append(unknown, a)
		}
	}
	if len(unknown) > 0 {
		fmt.Fprintf(stderr, "用法错：本件不认的旗标 %s ⇒ rc=2（**不许**静默当成仓根）\n", strings.Join(unknown, " "))
		fmt.Fprintln(stderr, "用法：python3 scripts/gates/check_version.py [仓库根]")
		return 2
	}

	arg := ""
	if len(argv) > 0 {
		arg = argv[0]
	}
	root := verRepoRoot(arg)

	// py 先算 rust、再算 go（左到右）；rust 抛异常即整体退 1 且不打印任何 stdout（未覆盖面②）。
	rv, rerr := verRustVersion(root)
	if rerr != nil {
		// py 在「件是目录 / 非 UTF-8」时抛 traceback（stdout 空 · 退码 1）；本件 stdout 同为空，stderr 正文不同。
		fmt.Fprintf(stderr, "✗ 读不到 %s：%v ⇒ 判不了（py 原件此处抛 traceback · 退码 1）\n",
			filepath.Join(root, "ui", "Cargo.toml"), rerr)
		return 1
	}
	gv := verGoVersion(root)

	fmt.Fprintf(stdout, "仓库根: %s\n", root)
	fmt.Fprintf(stdout, "  ui/Cargo.toml version      = %s\n", verOrNone(rv))
	fmt.Fprintf(stdout, "  core version.Version       = %s\n", verOrNone(gv))
	// ★ 空集守卫（GAP-20260927-108）：域 = 「取到的版本真源」——
	//   任一侧取不到（件缺失 / 无 version 行 ⇒ 空串）时**比较整块消失** ⇒ 不许当「一致」绿；
	//   同族口径：**不存在 / 取不到 ⇒ 不给结论（rc=2）**，与「两处不一致」的真红（rc=1）分得开。
	if rv == "" || gv == "" { // ★ 空集守卫（GAP-20260927-108）：版本真源取不到（空域）⇒ rc=2（不给结论）
		fmt.Fprint(stdout, "\n结果: 不给结论（rc=2）—— 版本真源取不到（ui/Cargo.toml 或 core version.go 缺失/无 version 行）\n")
		return 2
	}
	if rv != gv {
		fmt.Fprint(stdout, "\n结果: 失败——版本号不一致（收版时两处必须同改）\n")
		return 1
	}
	fmt.Fprintf(stdout, "\n结果: 通过——版本号一致 %s\n", rv)
	return 0
}

// verRepoRoot —— py `repo_root()`。
// 给了 arg ⇒ `abspath(arg)`；不给 ⇒ 从 **cwd** 上溯 ≤6 层找同时含 `core/` 与 `ui/` 的目录，
// 找不到回落 cwd（✗ 与 py 的起点不同：py 从 `__file__` 目录起 —— 见件头 (a)）。
func verRepoRoot(arg string) string {
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
		if verIsDir(filepath.Join(d, "core")) && verIsDir(filepath.Join(d, "ui")) {
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

// verRustVersion —— py `rust_version(root)`：读 `ui/Cargo.toml`，取首个 `^version = "…"`。
// 文件存在但无匹配 ⇒ 空串（py 为 None）。读不到 ⇒ err（py 抛异常 —— 未覆盖面②）。
func verRustVersion(root string) (string, error) {
	p := filepath.Join(root, "ui", "Cargo.toml")
	if !verExists(p) { // ★ 空集守卫（GAP-20260927-108）：件不存在 ⇒ 取不到（返空串），不报错 ⇒ 主档 rc=2
		return "", nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	txt := verUniversalNewlines(string(b))
	if m := verRustRE.FindStringSubmatch(txt); m != nil {
		return m[1], nil
	}
	return "", nil
}

// verGoVersion —— py `go_version(root)`：先查存在，再读 `core/internal/version/version.go`，
// 取首个 `^const Version = "…"`。缺失/无匹配 ⇒ 空串（py 为 None）。
func verGoVersion(root string) string {
	p := filepath.Join(root, "core", "internal", "version", "version.go")
	if !verExists(p) {
		return ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	txt := verUniversalNewlines(string(b))
	if m := verGoRE.FindStringSubmatch(txt); m != nil {
		return m[1]
	}
	return ""
}

// verUniversalNewlines —— py 文本模式的通用换行翻译（`\r\n`/`\r` → `\n`）。
func verUniversalNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// verOrNone —— py `%s` 遇 None 打 `None`；本件以空串代表 None。
func verOrNone(v string) string {
	if v == "" {
		return "None"
	}
	return v
}

// verExists —— py `os.path.exists`（存在即可，含目录）。
func verExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// verIsDir —— py `os.path.isdir`。
func verIsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// verLastLine —— py `(out.strip().splitlines() or [""])[-1][:90]`（按字符截 90）。
func verLastLine(out string) string {
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

// verSelfTest —— py `self_test()`：三档（正控 0 · 负控 1 · 用法错 2）。
// 差异（见件头未覆盖面③）：py 用 subprocess 真跑自身取 rc；本件进程内复用入口取 rc（文案逐字对齐）。
func verSelfTest(stdout io.Writer) int {
	bad := 0
	fmt.Fprintln(stdout, "── check_version.py --self-test 三档（正控 0 · 负控 1 · 用法错 2）")
	tmp, err := os.MkdirTemp("", "version-selftest-")
	if err != nil {
		tmp = os.TempDir()
	}
	defer os.RemoveAll(tmp)

	// py mk(name, cargo_v, go_v)：造含 ui/Cargo.toml 与 core/internal/version/version.go 的合成仓。
	mk := func(name, cargoV, goV string) string {
		root := filepath.Join(tmp, name)
		_ = os.MkdirAll(filepath.Join(root, "ui"), 0o755)
		_ = os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755)
		_ = os.WriteFile(filepath.Join(root, "ui", "Cargo.toml"),
			[]byte("[package]\nversion = \""+cargoV+"\"\n"), 0o644)
		_ = os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
			[]byte("package version\n\nconst Version = \""+goV+"\"\n"), 0o644)
		return root
	}
	// py _st_chk(bad, label, want, rc, out)
	chk := func(label string, want, rc int, out string) {
		good := rc == want
		if !good {
			bad++
		}
		mark, tail := "✓", ""
		if !good {
			mark = "✗"
			tail = " ｜ " + verLastLine(out)
		}
		fmt.Fprintf(stdout, "  %s %s（期望 rc=%d 实际 rc=%d）%s\n", mark, label, want, rc, tail)
	}
	run := func(argv ...string) (int, string) {
		var o, e bytes.Buffer
		rc := versionGateMain(&o, &e, argv)
		return rc, o.String() + e.String()
	}

	same := mk("same", "1.2.3", "1.2.3")
	diff := mk("diff", "1.2.3", "9.9.9")

	rc, out := run(same)
	chk("档① 正控：两侧版本号一致 ⇒ rc=0", 0, rc, out)
	rc, out = run(diff)
	chk("档② 负控：两侧版本号不一致 ⇒ rc=1", 1, rc, out)
	rc, out = run("--no-such-flag")
	chk("档③ 用法错：不认的旗标 ⇒ rc=2（不静默当初仓库根）", 2, rc, out)

	if bad == 0 {
		fmt.Fprintln(stdout, "── 自检：全过 ✓")
		return 0
	}
	fmt.Fprintf(stdout, "── 自检：**%d 档不过** ✗\n", bad)
	return 1
}
