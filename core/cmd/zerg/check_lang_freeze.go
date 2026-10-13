// check_lang_freeze.go —— 判据件 `check-lang-freeze`：**语言冻结门**（新代码只用 Go）。
//
// 性质：**棘轮式门** —— 只拦「新增」，不追旧账（业界同型：golangci-lint 的 `issues.new-from-rev`
// grandfathering —— 旧违规一律豁免（永不因旧代码红），只阻止**新**函数越界）。设计稿：
// `~/.zerg/state/设计-语言冻结门-20261007.md`；现读清单（口径 A）：
// `~/.zerg/state/语言冻结-现读清单-20261007.tsv`。
//
// 三态判定（真源见下）：
//
//	A 新增   现读自研面 py/sh 件 ∉（冻结快照 ∪ `retire_exceptions`） ⇒ 判红：点名
//	         「新增自研面 <lang>: <件>（新代码只用 Go；确需保留请入例外名册）」；
//	B 未销账 快照里的某件已不在盘、且它**不在** `retire_ledger` ⇒ 判红：点名
//	         「<件> 已消失但未登记退场（唯一销账口：zerg script retire）」；
//	C 例外   现读件命中 `retire_exceptions`（且 ∉ 快照） ⇒ 放行 ＋ 逐条打印理由。
//
// ★ 关键保护（B）：悄悄删掉一个 py 也红 —— 唯一合法销账口 = 退场正门 `zerg script retire`
//
//	（退一件 ⇒ 它自动进 `retire_ledger` ⇒ B 态即放行；✗ 不许悄删）。
//
// 真源（单点 · ✗ 不新造闭集表）：
//
//	· 闭集的**排除前缀** = **同包** `scriptRetireClosedSetExcludedPrefixes`
//	  （同包 `family_script_retire.go` 的 `scriptRetireClosedSetExcludedPrefixes` · 决-12 · **符号锚** ✗ 不写行号）；本件经 `scriptRetireInClosedSet` 复用它，
//	  ✗ 不复制一份前缀表（判据：全包只此一个 `var …ExcludedPrefixes`）。
//	· 例外名册 / 退场台账 = **同包** `core/internal/contract/adopt-retire.json` 的
//	  `retire_exceptions` / `retire_ledger`（`scriptRetireTruthRel` / `scriptRetireExceptionKey`
//	  / `scriptRetireLedgerKey` · 经 `scriptRetireDoc` 读）。
//	· 冻结快照 = 本件 `--emit` 生成（默认 `scripts/gates/lang-freeze-baseline.json` · schema
//	  `zerg-lang-freeze/1`；`entries[{path,lang,bytes}]`）。
//
// 旗标（家族同型 · 先现读 `check_judge_registry.go` 对齐）：`--check`（**默认** · 只读）｜
// `--emit [--out <件>]`（**只有它**写快照）｜`--self-test`｜`--root <根>`。三档互斥。
//
// 退码：0 全绿 ／ 1 门禁红（A 或 B 命中）／ 2 不给结论（仓根取不到 · 缺快照 · 扫到 0 件 · 用法错）。
//
// 出厂声明 (a) 默认根定位：无 `__file__` 语境 ⇒ 不给 `--root` 时取 `repoRoot()`（`ZERG_REPO` →
// 可执行文件目录上溯 → cwd 上溯，锚 = `core/internal/version/version.go`）。对拍一律显式传 `--root`。
//
// 出厂声明 (b) 自指检查：本件是 `.go` ⇒ 落不进本门自己的扫描面（只收 `.py`/`.sh`）；快照件是
// `.json` ⇒ 同不在内。本件 read-only（默认档不写任何件）。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// clfJudgeKey —— 判据键名（与件名同型 · 家族口径）。
	clfJudgeKey = "check-lang-freeze"
	// clfProg —— 件名（报话前缀）。
	clfProg = "check-lang-freeze"
	// clfSnapshotRel —— 冻结快照的默认落点（仓内相对）。
	clfSnapshotRel = "scripts/gates/lang-freeze-baseline.json"
	// clfSchema —— 快照 schema。
	clfSchema = "zerg-lang-freeze/1"
)

// clfEntry —— 快照一条（path/lang/bytes · 逐字照设计稿 §三）。
type clfEntry struct {
	Path  string `json:"path"`
	Lang  string `json:"lang"`
	Bytes int64  `json:"bytes"`
}

// clfSnapshot —— 冻结快照（schema / 生成时刻 / entries）。
type clfSnapshot struct {
	Schema    string     `json:"schema"`
	Generated string     `json:"generated"`
	Entries   []clfEntry `json:"entries"`
}

// clfLangOf —— 现读件语言（`py` / `sh`；其余空串）。
func clfLangOf(rel string) string {
	switch {
	case strings.HasSuffix(rel, ".py"):
		return "py"
	case strings.HasSuffix(rel, ".sh"):
		return "sh"
	}
	return ""
}

// clfSkipDir —— 目录剪枝：`.git` / `__pycache__` 一律跳过；闭集排除前缀（**同一真源**
// `scriptRetireClosedSetExcludedPrefixes`）下的目录整体跳过（省时 · 与逐件闭集判定同口径）。
func clfSkipDir(rel string) bool {
	base := rel
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		base = rel[i+1:]
	}
	if base == ".git" || base == "__pycache__" {
		return true
	}
	slash := rel + "/"
	for _, p := range scriptRetireClosedSetExcludedPrefixes {
		if strings.HasPrefix(slash, p) {
			return true
		}
	}
	return false
}

// clfScan —— 现读自研面 py/sh 件（闭集口径 · 与 `zerg script retire` 同一 `scriptRetireInClosedSet`）。
//
// 排序按 path 升序（确定性；✗ 无 map 迭代进输出）。
func clfScan(root string) ([]clfEntry, error) {
	var out []clfEntry
	root = filepath.Clean(root)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil // 读不到的子树跳过（不使整门炸）
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if clfSkipDir(rel) {
				return fs.SkipDir
			}
			return nil
		}
		if !scriptRetireInClosedSet(rel) {
			return nil
		}
		var sz int64
		if fi, e := d.Info(); e == nil {
			sz = fi.Size()
		}
		out = append(out, clfEntry{Path: rel, Lang: clfLangOf(rel), Bytes: sz})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

// clfLoadRetire —— 读退场真源的两块（例外名册 · 退场台账）。取不到 / 解不动 ⇒ **空集**
// （本门只拿它当白名单 · 缺失退化只会更严 · ✗ 不会假绿：例外缺失会把例外件误判为「新增」＝ 假红）。
func clfLoadRetire(root string) (map[string]scriptRetireExc, map[string]bool) {
	exc := map[string]scriptRetireExc{}
	led := map[string]bool{}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(scriptRetireTruthRel)))
	if err != nil {
		return exc, led
	}
	var doc scriptRetireDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return exc, led
	}
	for _, e := range doc.RetireExceptions {
		if obj := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(e.Object)), "./"); obj != "" {
			exc[obj] = e
		}
	}
	for _, e := range doc.RetireLedger {
		if obj := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(e.Object)), "./"); obj != "" {
			led[obj] = true
		}
	}
	return exc, led
}

// clfReadSnapshot —— 读冻结快照。
func clfReadSnapshot(path string) (*clfSnapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s clfSnapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// clfCountLangs —— 快照 / 现读件的 py/sh 计数。
func clfCountLangs(es []clfEntry) (int, int) {
	py, sh := 0, 0
	for _, e := range es {
		switch e.Lang {
		case "py":
			py++
		case "sh":
			sh++
		}
	}
	return py, sh
}

// clfJudge —— 三态判定（纯函数 · 只看传入的真源 ⇒ 负控可喂合成件）。
//
// 返回 (rc, 逐行输出)：rc=1 ⟺ A 或 B 命中；C 命中 ⇒ 放行 ＋ 逐条打印理由。
func clfJudge(root string, snap *clfSnapshot, exc map[string]scriptRetireExc, led map[string]bool) (int, []string) {
	live, _ := clfScan(root)
	snapSet := map[string]bool{}
	for _, e := range snap.Entries {
		if e.Path != "" {
			snapSet[e.Path] = true
		}
	}
	liveSet := map[string]bool{}
	for _, e := range live {
		liveSet[e.Path] = true
	}

	var added, unlogged, excused []string

	// ── A 新增 ＋ C 例外（现读件 ∉ 快照）────────────────────────────────────
	for _, e := range live {
		if snapSet[e.Path] {
			continue
		}
		if ex, ok := exc[e.Path]; ok {
			excused = append(excused, fmt.Sprintf("例外放行 %s（%s · %s）", e.Path, ex.Category, ex.Reason))
			continue
		}
		added = append(added, fmt.Sprintf("新增自研面 %s: %s（新代码只用 Go；确需保留请入例外名册）", e.Lang, e.Path))
	}

	// ── B 未销账（快照件 ∉ 盘 且 ∉ 台账）────────────────────────────────────
	for _, e := range snap.Entries {
		if e.Path == "" || liveSet[e.Path] {
			continue
		}
		if led[e.Path] {
			continue
		}
		unlogged = append(unlogged, fmt.Sprintf("%s 已消失但未登记退场（唯一销账口：zerg script retire）", e.Path))
	}

	lines := make([]string, 0, len(added)+len(unlogged)+len(excused))
	lines = append(lines, added...)
	lines = append(lines, unlogged...)
	lines = append(lines, excused...)

	if len(added) > 0 || len(unlogged) > 0 {
		return 1, lines
	}
	return 0, lines
}

// clfEmit —— `--emit`：现读扫描 ⇒ 写冻结快照（**唯一写面**）。
func clfEmit(root, out string, stdout, stderr io.Writer) int {
	live, err := clfScan(root)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 扫仓失败（%s）：%v ⇒ 不给结论（rc=2）\n", clfProg, root, err)
		return 2
	}
	if len(live) == 0 {
		fmt.Fprintf(stderr, "%s: 扫到 0 件自研面 py/sh（根 %s）⇒ **空 ≠ 缺**，不给结论（rc=2）\n", clfProg, root)
		return 2
	}
	snap := clfSnapshot{Schema: clfSchema, Generated: time.Now().UTC().Format(time.RFC3339), Entries: live}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "%s: 快照序列化失败：%v ⇒ 不给结论（rc=2）\n", clfProg, err)
		return 2
	}
	b = append(b, '\n')
	if out == "-" {
		_, _ = stdout.Write(b)
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(stderr, "%s: 快照落点建不了（%s）：%v ⇒ 不给结论（rc=2）\n", clfProg, out, err)
		return 2
	}
	if err := os.WriteFile(out, b, 0o644); err != nil {
		fmt.Fprintf(stderr, "%s: 快照写不了（%s）：%v ⇒ 不给结论（rc=2）\n", clfProg, out, err)
		return 2
	}
	py, sh := clfCountLangs(live)
	fmt.Fprintf(stdout, "已写冻结快照 %s · %d 件（py %d + sh %d）· schema %s\n", out, len(live), py, sh, clfSchema)
	return 0
}

// clfCheck —— 默认 / `--check`：只读判定。
func clfCheck(root, snapPath string, stdout, stderr io.Writer) int {
	snap, err := clfReadSnapshot(snapPath)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 读不到冻结快照 %s：%v ⇒ 不给结论（rc=2）；先 `%s --emit` 生成\n",
			clfProg, clfSnapshotRel, err, clfProg)
		return 2
	}
	exc, led := clfLoadRetire(root)
	rc, lines := clfJudge(root, snap, exc, led)
	live, _ := clfScan(root)
	lpy, lsh := clfCountLangs(live)
	spy, ssh := clfCountLangs(snap.Entries)
	fmt.Fprintf(stdout, "── 语言冻结门（棘轮 · 只拦新增 · 不追旧账）── %s\n", clfSchema)
	fmt.Fprintf(stdout, "快照 %s · %d 件（py %d + sh %d）｜现读 %d 件（py %d + sh %d）｜例外名册 %d 条 · 退场台账 %d 条\n",
		clfSnapshotRel, len(snap.Entries), spy, ssh, len(live), lpy, lsh, len(exc), len(led))
	for _, l := range lines {
		fmt.Fprintf(stdout, "  %s\n", l)
	}
	if rc == 1 {
		fmt.Fprintln(stdout, "⇒ 门禁红（rc=1）：新代码只用 Go；新增须改口径 · 退场走 zerg script retire（唯一销账口）")
		return 1
	}
	fmt.Fprintln(stdout, "⇒ 全绿（rc=0）：无新增自研面 · 无未销账退场")
	return 0
}

// clfSelfTest —— 合成自检（仓外临时树 · 四态 + 缺快照）：全过 ⇒ rc=0。
func clfSelfTest(stdout, stderr io.Writer) int {
	tmp, err := os.MkdirTemp("", "clf-selftest-")
	if err != nil {
		tmp = os.TempDir()
	} else {
		defer os.RemoveAll(tmp)
	}
	writeFile := func(rel, body string) {
		p := filepath.Join(tmp, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
	}
	snapOf := func(entries []clfEntry) *clfSnapshot {
		return &clfSnapshot{Schema: clfSchema, Generated: "2026-10-07T00:00:00Z", Entries: entries}
	}
	base := []clfEntry{{Path: "a.py", Lang: "py"}, {Path: "b.sh", Lang: "sh"}}
	ok, bad := 0, 0
	report := func(name string, want, got int, extra bool, line string) {
		if got == want && extra {
			fmt.Fprintf(stdout, "   ✓ %s（rc=%d）\n", name, got)
			ok++
			return
		}
		fmt.Fprintf(stdout, "   ✗ %s：期望 rc=%d，得 rc=%d；%s\n", name, want, got, line)
		bad++
	}

	// ① 正控（绿）；顺带：vendor/ 下的 .py ✗ 不进自研面
	writeFile("a.py", "print(1)\n")
	writeFile("b.sh", "echo b\n")
	writeFile("vendor/x.py", "print(9)\n")
	writeFile(scriptRetireTruthRel, `{"retire_ledger":[],"retire_exceptions":[]}`)
	rc1, l1 := clfJudge(tmp, snapOf(base), map[string]scriptRetireExc{}, map[string]bool{})
	report("正控 正常 ⇒ 绿", 0, rc1, len(l1) == 0, "正控不应出判定行")

	// ② A 新增（红）
	writeFile("new.py", "print(2)\n")
	rc2, l2 := clfJudge(tmp, snapOf(base), map[string]scriptRetireExc{}, map[string]bool{})
	report("甲 新增 .py ⇒ 红", 1, rc2, clfHasLine(l2, "新增自研面 py: new.py（新代码只用 Go；确需保留请入例外名册）"), "未见新增点名")
	_ = os.Remove(filepath.Join(tmp, "new.py"))

	// ③ B 未销账（红）＋ ③b 同件进台账（放行）
	withGone := append(append([]clfEntry{}, base...), clfEntry{Path: "gone.py", Lang: "py"})
	rc3, l3 := clfJudge(tmp, snapOf(withGone), map[string]scriptRetireExc{}, map[string]bool{})
	report("乙 快照有·盘上无·台账无 ⇒ 红", 1, rc3, clfHasLine(l3, "gone.py 已消失但未登记退场（唯一销账口：zerg script retire）"), "未见未销账点名")
	rc3b, _ := clfJudge(tmp, snapOf(withGone), map[string]scriptRetireExc{}, map[string]bool{"gone.py": true})
	report("乙' 同件进台账 ⇒ 绿", 0, rc3b, true, "台账未放行")

	// ④ C 例外（绿 + 逐条理由）
	writeFile("keep.py", "print(3)\n")
	rc4, l4 := clfJudge(tmp, snapOf(base),
		map[string]scriptRetireExc{"keep.py": {Object: "keep.py", Category: "合成类", Reason: "合成理由"}}, map[string]bool{})
	report("丙 例外 ⇒ 绿+理由", 0, rc4, clfHasLine(l4, "例外放行 keep.py（合成类 · 合成理由）"), "未见例外理由")
	_ = os.Remove(filepath.Join(tmp, "keep.py"))

	// ⑤ 缺快照 ⇒ rc=2
	var out, errb strings.Builder
	rc5 := clfCheck(tmp, filepath.Join(tmp, "nope.json"), &out, &errb)
	report("缺快照 ⇒ 不给结论", 2, rc5, true, "缺快照应 rc=2")

	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时树 · 未碰真仓）\n", ok, bad)
	if bad == 0 {
		return 0
	}
	return 2
}

func clfHasLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

// checkLangFreezeGateMain —— 入口（argv 与家族同形 · 不含程序名）。
func checkLangFreezeGateMain(stdout, stderr io.Writer, argv []string) int {
	rootArg, outArg := "", ""
	rootSet := false
	emit, doCheck, selfTest := false, false, false

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		val := func(name string) (string, bool) {
			if a == name {
				if i+1 >= len(argv) {
					return "", false
				}
				i++
				return argv[i], true
			}
			return strings.TrimPrefix(a, name+"="), true
		}
		switch {
		case a == "--root" || strings.HasPrefix(a, "--root="):
			v, okv := val("--root")
			if !okv {
				fmt.Fprintf(stderr, "%s: `--root` 缺值 ⇒ 不给结论（rc=2）\n", clfProg)
				return 2
			}
			rootArg, rootSet = v, true
		case a == "--out" || strings.HasPrefix(a, "--out="):
			v, okv := val("--out")
			if !okv {
				fmt.Fprintf(stderr, "%s: `--out` 缺值 ⇒ 不给结论（rc=2）\n", clfProg)
				return 2
			}
			outArg = v
		case a == "--emit":
			emit = true
		case a == "--check":
			doCheck = true
		case a == "--self-test":
			selfTest = true
		case a == "-h" || a == "--help":
			fmt.Fprintf(stdout, "用法：%s [--check | --emit [--out <件>] | --self-test] [--root <根>]\n", clfProg)
			return 0
		default:
			fmt.Fprintf(stderr, "%s: 未知旗标 %s ⇒ 不给结论（rc=2）\n", clfProg, a)
			return 2
		}
	}
	n := 0
	for _, b := range []bool{emit, doCheck, selfTest} {
		if b {
			n++
		}
	}
	if n > 1 {
		fmt.Fprintf(stderr, "%s: `--check` / `--emit` / `--self-test` 互斥 ⇒ 不给结论（rc=2）\n", clfProg)
		return 2
	}
	if selfTest {
		return clfSelfTest(stdout, stderr)
	}
	root := rootArg
	if !rootSet {
		root = repoRoot()
	}
	if strings.TrimSpace(root) == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根（`--root` 未给且 `repoRoot()` 取不到）⇒ 不给结论（rc=2）\n", clfProg)
		return 2
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	if emit {
		out := outArg
		switch {
		case out == "":
			out = filepath.Join(root, filepath.FromSlash(clfSnapshotRel))
		case out == "-":
			// 直写 stdout（机器面）
		case !filepath.IsAbs(out):
			out = filepath.Join(root, out)
		}
		return clfEmit(root, out, stdout, stderr)
	}
	return clfCheck(root, filepath.Join(root, filepath.FromSlash(clfSnapshotRel)), stdout, stderr)
}
