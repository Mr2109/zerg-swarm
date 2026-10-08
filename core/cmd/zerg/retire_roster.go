// retire_roster.go —— 判据件 `check-retire-roster`：**名册形态门**（例外名册 `reason` 首词闭集）。
//
// 义（终版口径 2026-10-08 · 真源 = `core/internal/contract/adopt-retire.json` boundaries 末两条）：
//
//	`retire_exceptions` 每条 `reason` 须写成 `<首词>：<为什么物理上不能 Go 化>`；首词 =
//	**首个全角 `：`（U+FF1A）之前**那段（再 TrimSpace），须 ∈ 闭集
//	{生态绑定, OS胶水, vendor, 硬线, 自举}（5 档）。★ 与「闭集内件（决-12）」是**两个面**，✗ 别混。
//
// ★ **判域 ≡ reason 首词 ∈ 闭集（乙案 · 2026-10-08 代拍）**：判域由契约（`boundaries[7]`）已定义语汇的
// 单字段派生 —— `reason` 首词合档者即入判域（名册增删**自动随动**）；旧「显式名单 ∪ category 命中三类」
// 双支已删（category 无契约语汇 · 腿净贡献 0）。17 件名单**降级为对照/校验**（✗ 非判域）；现读 17==17
// （双向差集空）⇒ 起手行为不变。
//
// 真源（单点 · ✗ 不另造第二份口径）：同名同包**纯函数** `scriptRetireExceptionErrors`
// （`family_script_retire.go`，四格：四列齐 / `object`∈决-12 闭集 / `review_date` / **④ 首词闭集**）。
// 本件只调它 —— 读名册体亦复用同包 `scriptRetireExcElements` 切块器，✗ 不重写一分解析。
//
// 档：**tri-report**（只报告 · 命中 rc=1 落 REPORT · rc=2 仍 BLOCKED）。本件 read-only
// （真源=名册本体 ⇒ ✗ 无写面 ⇒ 不设 `--emit`，与冻结门不同）。
//
// 退码：0 全绿 ／ 1 名册形态红（reason 首词 ∉ 闭集 等）／
//
//	2 不给结论（仓根取不到 · 真源读不到 / 解不动 · 名册块形状不认识 · 名册 0 条「空 ≠ 缺」· 用法错）。
//
// 边界（✗ 不重叠）：冻结门 `check-lang-freeze` 管「有没有新 py/sh」（加法守门 · 只报告）；
// 退场正门 `zerg script retire` 管「写入时拒坏」（写面 fail-closed）；本门管「名册每条形态与理由合法否」。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// rrProg —— 件名（报话前缀）。
	rrProg = "check-retire-roster"
	// rrTier —— 档位（只报告）。
	rrTier = "tri-report"
)

// rrUsage —— 用法串（家族同型）。
const rrUsage = "usage: check-retire-roster [--check] [--self-test] [--root <根>] [-h|--help]"

// rrDomainStats —— 判域现读读数。
type rrDomainStats struct {
	total      int      // 名册现读条数
	inDomain   int      // 判域内条数（reason 首词 ∈ 闭集）
	domainOK   int      // 判域内 reason 首词合档条数
	missingObj []string // 对照名单里**已离名册**的件（名单仅作校验 · ✗ 非判域 · 宜随名册瘦身下调）
}

// rrParseRoster —— 现读名册全部条目（复用同包切块器 · ✗ 不重写解析）。
func rrParseRoster(raw string) ([]scriptRetireExc, error) {
	elems, err := scriptRetireExcElements(raw, scriptRetireExceptionKey)
	if err != nil {
		return nil, err
	}
	out := make([]scriptRetireExc, 0, len(elems))
	for _, el := range elems {
		var e scriptRetireExc
		if jerr := json.Unmarshal([]byte(el.Text), &e); jerr != nil {
			return nil, fmt.Errorf("条目解不动（%v）", jerr)
		}
		out = append(out, e)
	}
	return out, nil
}

// rrDomain —— 判域现读读数（纯函数 · 只看传入条目）。判域 ≡ reason 首词 ∈ 闭集；名单仅作对照。
func rrDomain(entries []scriptRetireExc) rrDomainStats {
	var st rrDomainStats
	st.total = len(entries)
	seen := map[string]bool{}
	for _, e := range entries {
		obj := strings.TrimSpace(e.Object)
		seen[obj] = true
		if !scriptRetireExcInReasonDomain(e.Reason) {
			continue
		}
		st.inDomain++
		if w, ok := scriptRetireExcReasonPrefixOf(strings.TrimSpace(e.Reason)); ok && scriptRetireExcReasonPrefixClosed(w) {
			st.domainOK++
		}
	}
	for _, o := range scriptRetireExcReasonDomainObjects {
		if !seen[o] {
			st.missingObj = append(st.missingObj, o)
		}
	}
	return st
}

// rrCheck —— 默认 / `--check`：只读判定（真仓只读）。
func rrCheck(root string, stdout, stderr io.Writer) int {
	path := filepath.Join(root, filepath.FromSlash(scriptRetireTruthRel))
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s: 读不到真源 %s（根 %s）：%v ⇒ 不给结论（rc=2）\n",
			rrProg, scriptRetireTruthRel, root, err)
		return 2
	}
	raw := string(b)
	entries, perr := rrParseRoster(raw)
	var st rrDomainStats
	if perr == nil {
		st = rrDomain(entries)
	}
	errs := scriptRetireExceptionErrors(scriptRetireTruthRel, raw)

	fmt.Fprintln(stdout, "── 名册形态门 check-retire-roster（判域 ≡ reason 首词∈闭集 · 对照名单仅校验）──")
	fmt.Fprintf(stdout, "真源 : %s 的 %s\n", scriptRetireTruthRel, scriptRetireExceptionKey)
	fmt.Fprintf(stdout, "口径 : reason 须 `<首词>：<为什么物理上不能 Go 化>`；首词（首个全角 `：` U+FF1A 之前）∈ 闭集 {%s}\n",
		strings.Join(scriptRetireExcReasonPrefixes, ", "))

	// 名册块形状不认识 ⇒ 取数面不成立 ⇒ 不给结论。
	shapeUnknown := ""
	for _, e := range errs {
		if strings.Contains(e, "名册块形状不认识") {
			shapeUnknown = e
		}
	}
	if shapeUnknown != "" {
		fmt.Fprintf(stderr, "%s: %s ⇒ 不给结论（rc=2）\n", rrProg, shapeUnknown)
		return 2
	}
	if perr == nil && st.total == 0 {
		fmt.Fprintf(stderr, "%s: 名册 %s 现读 0 条（空 ≠ 缺）⇒ 不给结论（rc=2）\n", rrProg, scriptRetireExceptionKey)
		return 2
	}

	fmt.Fprintf(stdout, "现读 : 名册 %d 条 ｜ 判域 %d 条 ｜ 判域内合档 %d ｜ 判域内不合档 %d\n",
		st.total, st.inDomain, st.domainOK, st.inDomain-st.domainOK)
	for _, m := range st.missingObj {
		fmt.Fprintf(stdout, "  · 对照名单件已不在名册：%s（名单仅作校验 · ✗ 非判域）\n", m)
	}
	if len(errs) == 0 {
		fmt.Fprintf(stdout, "⇒ 全绿（rc=0）：判域内 %d 条 reason 首词全 ∈ 闭集\n", st.domainOK)
		fmt.Fprintf(stdout, "档 : %s（只报告 · ✗ 不阻断）\n", rrTier)
		return 0
	}
	fmt.Fprintln(stdout, "命中：")
	for _, e := range errs {
		fmt.Fprintf(stdout, "  ✗ %s\n", e)
	}
	fmt.Fprintf(stdout, "⇒ 门禁红（rc=1）：%d 处名册形态不合\n", len(errs))
	fmt.Fprintf(stdout, "档 : %s（只报告 · 命中 rc=1 落 REPORT · ✗ 不阻断）\n", rrTier)
	return 1
}

// rrSelfTest —— 合成自检（仓外临时树 · ✗ 不碰真仓）：正控 1 ＋ 负控若干 ⇒ 全过 rc=0。
func rrSelfTest(stdout, stderr io.Writer) int {
	ok, bad := 0, 0
	report := func(name string, want, got int, extra bool, note string) {
		if got == want && extra {
			fmt.Fprintf(stdout, "   ✓ %s（rc=%d）\n", name, got)
			ok++
			return
		}
		fmt.Fprintf(stdout, "   ✗ %s：期望 rc=%d，得 rc=%d；%s\n", name, want, got, note)
		bad++
	}
	// 造一个仓外临时树，写合成名册，再跑 rrCheck（hermetic · 只碰 temp）。
	runRoster := func(body string) (int, string) {
		dir, err := os.MkdirTemp("", "retire-roster-case-")
		if err != nil {
			return -1, "建临时目录失败：" + err.Error()
		}
		defer os.RemoveAll(dir)
		_ = os.MkdirAll(filepath.Join(dir, "core", "internal", "contract"), 0o755)
		_ = os.WriteFile(filepath.Join(dir, filepath.FromSlash(scriptRetireTruthRel)), []byte(body), 0o644)
		var o, e strings.Builder
		rc := rrCheck(dir, &o, &e)
		return rc, o.String() + e.String()
	}
	// 判域条：`reason` 首词 ∈ 闭集者即入判域（乙案 · 契约 boundaries[7]）；对照名单内件各测一态。
	mk := func(object, category, reason string) string {
		e := scriptRetireExc{Object: object, Category: category, Reason: reason, KeepCost: "k", ReviewDate: "2026-10-08"}
		eb, _ := json.Marshal(e)
		return "{\n  \"" + scriptRetireExceptionKey + "\": [\n    " + string(eb) + "\n  ]\n}\n"
	}

	// ① 正控：对照名单件 ＋ 闭集首词 ⇒ rc=0。
	rc1, out1 := runRoster(mk("scripts/svc/setup-zerg.sh", "外部依赖（退场例外）", "OS胶水：平台原生服务注册"))
	report("正控 对照名单件首词合 ⇒ 绿", 0, rc1, strings.Contains(out1, "判域 1 条"), "未见判域读数或非 0")

	// ② 负控①：对照名单件无全角冒号 ⇒ rc=1，且点名首词 ∉ 闭集。
	rc2, out2 := runRoster(mk("scripts/svc/setup-zerg.sh", "外部依赖（退场例外）", "真跑取数"))
	report("负控① 无全角冒号 ⇒ 红", 1, rc2, strings.Contains(out2, "reason 首词 ∉ 闭集"), "未见首词点名")

	// ③ 负控②：首词是旧口径（∉ 闭集，有全角冒号）⇒ rc=1。
	rc3, out3 := runRoster(mk("scripts/svc/setup-zerg.sh", "外部依赖（退场例外）", "能力依赖外部工具／联网：远端推送"))
	report("负控② 首词 ∉ 闭集 ⇒ 红", 1, rc3, strings.Contains(out3, "reason 首词 ∉ 闭集"), "未见首词点名")

	// ④ 负控③：半角冒号 ⇒ rc=1（要全角 `：`）。
	rc4, _ := runRoster(mk("scripts/svc/setup-zerg.sh", "外部依赖（退场例外）", "OS胶水: 半角冒号不算"))
	report("负控③ 半角冒号 ⇒ 红", 1, rc4, true, "半角冒号未判红")

	// ⑤ 判域随动（乙案要点）：名单外件 ＋ 闭集首词 ⇒ 入判域、rc=0（✗ 无需补名单）。
	rc5, out5 := runRoster(mk("synth/auto.py", "能 Go 化", "自举：步表宿主"))
	report("判域随动 名单外件首词合 ⇒ 绿且入判域", 0, rc5, strings.Contains(out5, "判域 1 条"), "名单外合档件未入判域")

	// ⑥ 非判域条（名单外 · 首词 ∉ 闭集）坏理由 ⇒ **豁免** rc=0。
	rc6, out6 := runRoster(mk("synth/y.py", "能 Go 化", "真跑取数"))
	report("非判域条坏理由 ⇒ 绿（豁免）", 0, rc6, strings.Contains(out6, "判域 0 条"), "非判域条被误判")

	// ⑦ 既有格仍在：object 不在决-12 闭集 ⇒ rc=1（四格同源未退）。
	rc8, out8 := runRoster(mk("vendor/x.py", "物理不可化", "生态绑定：x"))
	report("闭集腿仍在（vendor/ ⇒ 红）", 1, rc8, strings.Contains(out8, "不在决-12 闭集"), "闭集腿没牙")

	// ⑧ 空名册（0 条）⇒ rc=2（空 ≠ 缺）。
	rc9, out9 := runRoster("{\n  \"" + scriptRetireExceptionKey + "\": []\n}\n")
	report("空名册 ⇒ 不给结论 rc=2", 2, rc9, strings.Contains(out9, "空 ≠ 缺"), "空名册未退 2")

	// ⑨ 缺真源 ⇒ rc=2。
	emptyDir, _ := os.MkdirTemp("", "retire-roster-empty-")
	defer os.RemoveAll(emptyDir)
	var o, e strings.Builder
	rc10 := rrCheck(emptyDir, &o, &e)
	report("缺真源 ⇒ 不给结论 rc=2", 2, rc10, true, "缺真源未退 2")

	_ = stderr
	fmt.Fprintf(stdout, "   自检：%d 格过 · %d 格不过（仓外临时树 · 未碰真仓）\n", ok, bad)
	if bad == 0 {
		return 0
	}
	return 2
}

// retireRosterGateMain —— 入口（argv 与家族同形 · ★ inproc 喂的 argv **不含程序名**）。
func retireRosterGateMain(stdout, stderr io.Writer, argv []string) int {
	rootArg, rootSet := "", false
	doCheck, selfTest := false, false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "--root" || strings.HasPrefix(a, "--root="):
			if a == "--root" {
				if i+1 >= len(argv) {
					fmt.Fprintf(stderr, "%s: `--root` 缺值 ⇒ 不给结论（rc=2）\n", rrProg)
					return 2
				}
				i++
				rootArg = argv[i]
			} else {
				rootArg = strings.TrimPrefix(a, "--root=")
			}
			rootSet = true
		case a == "--check":
			doCheck = true
		case a == "--self-test":
			selfTest = true
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, rrUsage)
			return 0
		default:
			fmt.Fprintf(stderr, "%s: 未知旗标 %s ⇒ 不给结论（rc=2）\n", rrProg, a)
			return 2
		}
	}
	_ = doCheck // `--check` 与缺省同态（默认即只读判定）
	if selfTest {
		return rrSelfTest(stdout, stderr)
	}
	root := rootArg
	if !rootSet {
		root = repoRoot()
	}
	if strings.TrimSpace(root) == "" {
		fmt.Fprintf(stderr, "%s: 解析不到仓根（`--root` 未给且 `repoRoot()` 取不到）⇒ 不给结论（rc=2）\n", rrProg)
		return 2
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return rrCheck(root, stdout, stderr)
}
