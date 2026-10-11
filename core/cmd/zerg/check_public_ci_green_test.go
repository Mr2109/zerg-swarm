// check_public_ci_green_test.go —— 判据件的定向测件
// `scripts/gates/check-public-ci-green.py` 的 Go 真搬（`check_public_ci_green.go`）的配对测件。
//
// ★ GAP-20261001-61：本测件**确实** spawn `python3`（K1「现调分档真源」的那两格）⇒ 一律钉
//
//	`/usr/bin/python3`（`cicgTestPython`），✗ 不用裸 `python3` / ✗ 不靠 PATH。
//	（凡不 spawn python 的格 —— 位置参数面 / 扫描纯函数面 / nil-slice 面 —— 走进程内入口，无需解释器。）
//
// 夹具一律落 `t.TempDir()`（**仓外**）· 跑后计数 0（本测件自己不落任何仓内件）。
// ★ A 类形态（自回收牙 · 留待新单）：本门与 py 一致，`tempfile`/`os.MkdirTemp` 造的合成夹具**不回收**
//
//	（py `self_test()` / `main()` 都只 `mkdtemp`、不 `rmtree`）⇒ **逐字保留现状**，本测件**不**加回收牙
//	（见回执「未覆盖面」）。
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cicgTestPython —— 测件里 python3 一律钉这根（GAP-20261001-61）。
const cicgTestPython = "/usr/bin/python3"

// cicgThreeGapsFloor —— ② 三条缺项的**冻结下限**（棘轮 · 只许增不许减；本值 = 当次现读）。
// 三条缺项现读 3 条（`check_public_ci_green.go` 的 `cicgThreeGaps`）⇒ 结构静默变小（< 3）即判红。
const cicgThreeGapsFloor = 3

func cicgRunMain(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := cicgGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// cicgRepoRoot —— 哨兵：锚本门 **Go 半** `core/cmd/zerg/check_public_ci_green.go`（✗ 不再锚分档真源 py）。
// 旧名 `cicgRepoRootOrSkip` 锚 `scripts/gates/check-manifest-freshness-profile.py`：该 py 已退场
// （2026-10-07 M2 退役批）⇒ 上溯恒找不到 ⇒ 恒 Skip（假绿、零保护）。改锚 Go 半（与本件同包 ·
// 编译进测试二进制）⇒ 真跑断言；找不到 = 真仓结构被破 ⇒ **fail-closed**（✗ 不留 Skip）。
func cicgRepoRoot(t *testing.T) string {
	t.Helper()
	root := cicgResolveRoot()
	if !cicgFileExists(cicgPyJoin(root, "core/cmd/zerg/check_public_ci_green.go")) {
		t.Fatalf("哨兵失守：仓根 %s 下未见 Go 半 core/cmd/zerg/check_public_ci_green.go（✗ 不留 Skip）", root)
	}
	return root
}

// ── 牙①/牙②：`cicgRead` 的「取不到」与「在场但空」必须分得开（★ 不用 nil 兼表两态）──

func TestCICGReadTwoStatesPairs(t *testing.T) {
	dir := t.TempDir()

	// 牙①：读不到 ⇒ ok=false。
	if s, ok := cicgRead(filepath.Join(dir, "不存在.yml")); ok || s != "" {
		t.Fatalf("牙①（取不到）失配：s=%q ok=%v（期望 \"\" / false）", s, ok)
	}

	// 牙②：在场但空 ⇒ ok=true 且 s=""（**不是**「取不到」）。
	emptyPath := filepath.Join(dir, "空件.yml")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s, ok := cicgRead(emptyPath)
	if !ok || s != "" {
		t.Fatalf("牙②（在场但空）失配：s=%q ok=%v（期望 \"\" / true）", s, ok)
	}

	// 牙①②成对：两态在 `ok` 上分得开。
	present, okPresent := cicgRead(emptyPath)
	missing, okMissing := cicgRead(filepath.Join(dir, "不存在.yml"))
	if okPresent == okMissing || present != missing {
		t.Fatalf("两态没分开：在场=(%q,%v) 缺件=(%q,%v)", present, okPresent, missing, okMissing)
	}
}

// ── 牙③/牙④：`cicgRunProfile` 的「真源不在场」与「跑起来了（哪怕非零退码）」──

func TestCICGRunProfilePairs(t *testing.T) {
	// 牙③：真源不在场 ⇒ (0, false)（= py 的 None）。
	absentRoot := t.TempDir()
	rc, ok := cicgRunProfile(absentRoot, "dev", "", nil)
	if ok || rc != 0 {
		t.Fatalf("牙③（真源不在场）失配：rc=%d ok=%v（期望 0 / false）", rc, ok)
	}

	// 牙④：真源在场 ＋ 解释器真跑且**非零退码** ⇒ (退码, true) —— 不是「取不到」。
	presentRoot := t.TempDir()
	gatesDir := filepath.Join(presentRoot, "scripts", "gates")
	if err := os.MkdirAll(gatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gatesDir, "check-manifest-freshness-profile.py"), []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(t.TempDir(), "stub.sh")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%s\\n' '{\"rc\": 3}'\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZERG_CICG_PYTHON", stub)
	rc, ok = cicgRunProfile(presentRoot, "dev", "", nil)
	if !ok || rc != 3 {
		t.Fatalf("牙④（在场＋非零退码）失配：rc=%d ok=%v（期望 3 / true）", rc, ok)
	}
}

// ── K1 的机械证明：现调分档真源（退役 ⇒ 其 Go 半进程内）· 合成清单双命中（dev 不拦 / public 拦）──

func TestCICGRealProfileTeeth(t *testing.T) {
	root := cicgRepoRoot(t)

	mf := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(mf, []byte(cicgSynthManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	relHead := strings.Repeat("1", 40)
	dev, devOK := cicgRunProfile(root, "dev", mf, []string{"--release-head", relHead})
	pub, pubOK := cicgRunProfile(root, "public", mf, []string{"--release-head", relHead})
	if !devOK || !pubOK {
		t.Fatalf("真源没跑起来：dev=(%d,%v) public=(%d,%v)", dev, devOK, pub, pubOK)
	}
	if dev != 0 {
		t.Fatalf("开发机档 rc=%d（期望 0 · 只告警 ⇒ 不拦）", dev)
	}
	if pub != 1 {
		t.Fatalf("公开制品档 rc=%d（期望 1 · 必须阻断）", pub)
	}
}

// ── 门面自检（含 must-fail 真红）──────────────────────────────────────────────

func TestCICGGateMainSelfTest(t *testing.T) {
	cicgRepoRoot(t)

	rc, out, errb := cicgRunMain(t, "--self-test")
	if rc != 0 {
		t.Fatalf("自检 rc=%d（期望 0）\nstdout:\n%s\nstderr:\n%s", rc, out, errb)
	}
	if !strings.Contains(out, "自检结论: 全过") {
		t.Fatalf("自检未全过：\n%s", out)
	}
	for _, must := range []string{"负控(must-fail)", "正控 · 干净件"} {
		if !strings.Contains(out, must) {
			t.Fatalf("自检输出缺 %q：\n%s", must, out)
		}
	}
}

// ── 扫描纯函数：正控 / 负控成对 ───────────────────────────────────────────────

func TestCICGScanCIPositiveNegative(t *testing.T) {
	clean := cicgScanCI([]cicgFile{{rel: "合成/clean.yml", text: cicgFixClean, present: true}})
	if len(clean.attestTokens) != 0 || clean.usesTotal != 1 || len(clean.usesFloat) != 0 || len(clean.topWrite) != 0 {
		t.Fatalf("正控（干净件）失配：attest=%v uses=(%d,%v) write=%v",
			clean.attestTokens, clean.usesTotal, clean.usesFloat, clean.topWrite)
	}

	att := cicgScanCI([]cicgFile{{rel: "合成/attest.yml", text: cicgFixAttest, present: true}})
	if len(att.attestTokens) != 2 {
		t.Fatalf("负控（越线件）K3 命中数=%d（期望 2）", len(att.attestTokens))
	}
	if len(att.usesFloat) != 1 {
		t.Fatalf("负控（越线件）浮动 tag 命中数=%d（期望 1）", len(att.usesFloat))
	}

	wr := cicgScanCI([]cicgFile{{rel: "合成/write.yml", text: cicgFixWrite, present: true}})
	if len(wr.topWrite) != 1 {
		t.Fatalf("负控（提权件）顶层 write 命中数=%d（期望 1）", len(wr.topWrite))
	}

	// 缺件不参与（None 不炸）。
	miss := cicgScanCI([]cicgFile{{rel: "合成/缺件.yml", present: false}})
	if miss.usesTotal != 0 {
		t.Fatalf("缺件参与了扫描：uses_total=%d（期望 0）", miss.usesTotal)
	}
}

// ── ★ nil slice 不兼表两态：空集合的 JSON 面必须是 `[]`，不是 `null` ──────────────

func TestCICGEmptyCollectionsAreNotNull(t *testing.T) {
	empty := cicgScanCI(nil)
	if empty.attestTokens == nil || empty.usesFloat == nil || empty.topWrite == nil || empty.jobs == nil {
		t.Fatalf("空输入落了 nil 切片（会变成 JSON null）：attest=%v uses=%v write=%v jobs=%v",
			empty.attestTokens, empty.usesFloat, empty.topWrite, empty.jobs)
	}
	for _, c := range []struct {
		name string
		got  string
	}{
		{"attest_tokens", cicgJSONStrList(empty.attestTokens)},
		{"uses_float", cicgJSONStrList(empty.usesFloat)},
		{"top_write", cicgJSONStrList(empty.topWrite)},
		{"hits", cicgJSONStrList([]string{})},
	} {
		if c.got != "[]" {
			t.Fatalf("%s 空集合落成 %s（期望 []）", c.name, c.got)
		}
	}
	if s := cicgJSONStr("a\"b\\c\n"); s != `"a\"b\\c\n"` {
		t.Fatalf("JSON 串转义失配：%s", s)
	}
}

// ── 三条缺项：**逐字**（含反引号 —— ✗ 不许把渲染装饰抄进来 / 也不许丢）────────────

func TestCICGThreeGapsLiteral(t *testing.T) {
	want := []string{
		"① 人面看不见（`G-10`）",
		"② `warnings[]` 恒空",
		"③ attestation 今天没出（源件逐字「将来若要出 attestation，也在本 job 内单独加」）",
	}
	// ② 冻结下限（棘轮 · 只许增）：三条缺项条数 < 当次现读数下限 3 ⇒ 判红（结构静默变小）。
	if n := len(cicgThreeGaps); n < cicgThreeGapsFloor {
		t.Fatalf("三条缺项条数 %d < 冻结下限 %d（棘轮只许增 · 结构静默变小）", n, cicgThreeGapsFloor)
	}
	if len(cicgThreeGaps) != len(want) {
		t.Fatalf("三条缺项条数=%d（期望 %d）", len(cicgThreeGaps), len(want))
	}
	for i, w := range want {
		if cicgThreeGaps[i].text != w {
			t.Fatalf("第 %d 条逐字失配：\n实得 %q\n期望 %q", i+1, cicgThreeGaps[i].text, w)
		}
	}
	if cicgThreeGaps[0].id != "缺①" || cicgThreeGaps[1].id != "缺②" || cicgThreeGaps[2].id != "缺③" {
		t.Fatalf("三条缺项 id 失配：%q / %q / %q", cicgThreeGaps[0].id, cicgThreeGaps[1].id, cicgThreeGaps[2].id)
	}
	if cicgGapsSayApplied() {
		t.Fatal("三条缺项里出现「已修/已引/已出」⇒ 照原样登记被美化")
	}
}

// ── 位置参数面：help（stdout 逐字节）与两类用法错（stderr 逐字节）──────────────

func TestCICGHelpAndArgErrors(t *testing.T) {
	rc, out, errb := cicgRunMain(t, "--help")
	if rc != 0 || out != cicgHelpText || errb != "" {
		t.Fatalf("--help 失配：rc=%d stderr=%q\n实得 stdout:\n%q\n期望:\n%q", rc, errb, out, cicgHelpText)
	}
	rc, out, _ = cicgRunMain(t, "-h")
	if rc != 0 || out != cicgHelpText {
		t.Fatalf("-h 失配：rc=%d", rc)
	}

	rc, out, errb = cicgRunMain(t, "--bogus")
	wantErr := cicgUsage + cicgProg + ": error: unrecognized arguments: --bogus\n"
	if rc != 2 || out != "" || errb != wantErr {
		t.Fatalf("未知旗标失配：rc=%d stdout=%q\n实得 stderr:\n%q\n期望:\n%q", rc, out, errb, wantErr)
	}

	rc, _, errb = cicgRunMain(t, "--json")
	wantErr = cicgUsage + cicgProg + ": error: argument --json: expected one argument\n"
	if rc != 2 || errb != wantErr {
		t.Fatalf("缺值旗标失配：rc=%d\n实得 stderr:\n%q\n期望:\n%q", rc, errb, wantErr)
	}
}

// ── Q-176：`--json` 字段名打错 ⇒ rc=2 ＋ 印可选字段表（两态分得开）──────────────

func TestCICGRefuseUnknownJSONField(t *testing.T) {
	rc, out, errb := cicgRunMain(t, "--json", "rc,不存在的字段")
	if rc != 2 {
		t.Fatalf("Q-176 失配：rc=%d（期望 2）", rc)
	}
	if !strings.Contains(errb, "`不存在的字段`") {
		t.Fatalf("stderr 未点名不认的字段：%q", errb)
	}
	if !strings.Contains(errb, "可选字段表：") || !strings.Contains(errb, "`gaps_missing`") {
		t.Fatalf("stderr 未印可选字段表：%q", errb)
	}
	if out != "" {
		t.Fatalf("Q-176 该走 stderr，stdout 却非空：%q", out)
	}

	// 合法字段 ⇒ 原路照走（不落 Q-176 那一格）。
	rc, out, errb = cicgRunMain(t, "--json=rc,gaps")
	if rc == 2 || errb != "" {
		t.Fatalf("合法字段被拒：rc=%d stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "⇒ rc=") {
		t.Fatalf("合法字段没走命中面：%q", out)
	}
}

// ── 退码 ⊆ 真源登记集（T3）：裸数字退码不许出 {0,1,2} ──────────────────────────

func TestCICGExitCodesSubsetOfRegistered(t *testing.T) {
	registered := map[int]bool{0: true, 1: true, 2: true} // py 三档：0 对 / 1 有命中 / 2 不给结论
	for _, argv := range [][]string{
		{"--list-rules"},
		{"--json", "打错的名字"},
		{"--json"},
		{"--bogus"},
		{"--help"},
		{"-h"},
		{"--self-test", "多出来的位置参数"},
	} {
		rc, _, _ := cicgRunMain(t, argv...)
		if !registered[rc] {
			t.Fatalf("argv=%v ⇒ rc=%d 不在真源退码表 {0,1,2}", argv, rc)
		}
	}
}

// ── 真·负控：**仓外**夹具越线 ⇒ rc=1 且有牙（件面原样显示绝对路径）──────────────

func TestCICGNegControlExternalFixture(t *testing.T) {
	cicgRepoRoot(t)
	fixture := filepath.Join(t.TempDir(), "越线件.yml")
	body := "name: 合成负控（仓外夹具）\non: {workflow_dispatch: {}}\n" +
		"permissions:\n  contents: read\n  id-token: write\n" +
		"jobs:\n  a:\n    runs-on: ubuntu-24.04\n    steps:\n      - uses: actions/checkout@v4\n"
	if err := os.WriteFile(fixture, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cicgRunMain(t, "--extra-ci", fixture)
	if rc != 1 {
		t.Fatalf("仓外越线夹具 rc=%d（期望 1 ⇒ 真红）\n%s", rc, out)
	}
	if !strings.Contains(out, "K3 CI 件里出现了 id-token/attestations 权限") {
		t.Fatalf("K3 没点名：\n%s", out)
	}
	if !strings.Contains(out, "✗（缺件）") && !strings.Contains(out, fixture) {
		t.Fatalf("仓外夹具路径没进件面：\n%s", out)
	}
	if !strings.Contains(out, fixture) {
		t.Fatalf("绝对路径未**原样**显示（不许拼成难认的相对串）：\n%s", out)
	}
}

// ── 机器读面：`--json` 输出必须是**单行合法 JSON**，且 rc 与文本面一致 ──────────

func TestCICGJSONFaceShape(t *testing.T) {
	cicgRepoRoot(t)
	rc, out, _ := cicgRunMain(t, "--json", "rc,gaps,gaps_missing,attest_tokens,uses_total,uses_float,top_write,ci_files_present,hits")
	lines := cicgSplitLines(out)
	if len(lines) == 0 {
		t.Fatal("没有输出")
	}
	last := lines[len(lines)-1]
	var m map[string]any
	if err := json.Unmarshal([]byte(last), &m); err != nil {
		t.Fatalf("末行不是合法 JSON：%v\n%s", err, last)
	}
	got, ok := m["rc"].(float64)
	if !ok || int(got) != rc {
		t.Fatalf("JSON 面 rc=%v 与文本面 rc=%d 不一致", m["rc"], rc)
	}
	// ② 冻结下限（棘轮 · 只许增）：三条缺项条数 < 当次现读数下限 3 ⇒ 判红（结构静默变小）。
	if n := len(cicgThreeGaps); n < cicgThreeGapsFloor {
		t.Fatalf("三条缺项条数 %d < 冻结下限 %d（棘轮只许增 · 结构静默变小）", n, cicgThreeGapsFloor)
	}
	if n, ok := m["gaps_missing"].(float64); !ok || int(n) != len(cicgThreeGaps) {
		t.Fatalf("gaps_missing=%v（期望 %d）", m["gaps_missing"], len(cicgThreeGaps))
	}
	for _, k := range []string{"attest_tokens", "uses_float", "top_write", "ci_files_present", "hits"} {
		if v, present := m[k]; present && v == nil {
			t.Fatalf("%s 落成 null（空集合与取不到没分开）：%s", k, last)
		}
	}
}

// ── A 类（tempdir 泄漏）回归牙：`--self-test` 与门面直跑各跑一趟后，**本用例私有 TMPDIR**
// 下一级目录里前缀 zerg-ci-green- 的条数**增量必须为 0**（delta==0 · 不许 ≤0）。主件两处自清句
// `defer os.RemoveAll(tmp)`（check_public_ci_green.go:271 门面档 / :640 自检档）被删掉时本用例必红。
// ★ GAP-20261003-16：本用例把 TMPDIR 指到自建空目录（私有面）⇒ 只数本用例自建目录，不再与
// 同包兄弟用例 / 同机并发门跑（并发建删 `zerg-ci-green-*`）共享计数面 ⇒ 不再先红后绿。
func cicgCountCiGreenTmpDirs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatalf("读 TMPDIR（%s）失败：%v", os.TempDir(), err)
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() && strings.HasPrefix(e.Name(), "zerg-ci-green-") {
			n++
		}
	}
	return n
}

func TestCICGTempdirReclaimed(t *testing.T) {
	cicgRepoRoot(t)
	// ★ GAP-20261003-16：私有计数面（TMPDIR 指到本用例自建空目录）⇒ 只数本用例自建目录。
	// 断言未放宽（仍是 after!=before 即红）；主件删任一处自清句时私有面内必留目录 ⇒ 仍能抓真泄漏。
	// （本包零 `t.Parallel()` ⇒ `t.Setenv` 安全。）
	t.Setenv("TMPDIR", t.TempDir())

	// 两处 `os.MkdirTemp("", "zerg-ci-green-")`（门面档 :267 / 自检档 :636）逐档各跑一趟 ——
	// 两档都必须留下自清句；任一被删（tmp 泄漏）则 delta≠0 必红。
	for _, argv := range [][]string{{"--self-test"}, {}} {
		before := cicgCountCiGreenTmpDirs(t)
		rc, out, errb := cicgRunMain(t, argv...)
		after := cicgCountCiGreenTmpDirs(t)
		if len(argv) == 0 && rc == 2 {
			t.Fatalf("门面直跑 rc=2（一件 CI 件都读不到 ⇒ 没跑到合成夹具那一格 · 本牙空转）：\n%s", out)
		}
		if after != before {
			t.Fatalf("A 类 tempdir 泄漏：argv=%v 跑后 TMPDIR（%s）下 zerg-ci-green-* 一级目录 before=%d after=%d（delta=%d ≠ 0 · 断言恒为 0 · 不许 ≤0）⇒ 主件缺自清句 defer os.RemoveAll(tmp)（check_public_ci_green.go:271 门面档 / :640 自检档）\nstderr:%q\n%s",
				argv, os.TempDir(), before, after, after-before, errb, out)
		}
	}
}

// TestCICGResolveRootRetirementSafe —— 退役态安全（类①）：本门自指 py 退役（不在盘）后，
// 仓根锚必须仍**上溯**命中真仓根（认 `scripts/gates` **目录**），✗ 不退化成 cwd。
// ★ 从**子目录**起跑（core/cmd/zerg）：修好后须上溯到 root；旧锚（只认自指 py）在 py 缺席时
// 找不到任何锚 ⇒ 落回 cwd 子目录 ≠ root ⇒ 本用例必红。
// ★ 哑化负控：抽掉 check_public_ci_green.go 的 `|| cicgStatIsDir(...scripts/gates)` 一支 ⇒
// 本用例第一处 Fatalf 行当场 FAIL（实测 check_public_ci_green_test.go:429）。
func TestCICGResolveRootRetirementSafe(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	sub := filepath.Join(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// t.TempDir() 在 macOS 常带 /var→/private/var 链接 ⇒ 用真身比。
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
		sub = filepath.Join(root, "core", "cmd", "zerg")
	}
	t.Chdir(sub)

	// 退役态：有 scripts/gates 目录、**无**自指 py；从子目录起跑 ⇒ 必须上溯命中 root。
	if got := cicgResolveRoot(); got != root {
		t.Fatalf("自指 py 缺席时仓根锚应上溯命中 %s，得 %s（退役态退化 cwd）", root, got)
	}
	// 正控：自指 py 在盘 ⇒ 同根（两锚同根 · 逐字同旧）。
	if err := os.WriteFile(filepath.Join(root, "scripts", "gates", "check-public-ci-green.py"), []byte("#!/usr/bin/env python3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := cicgResolveRoot(); got != root {
		t.Fatalf("自指 py 在盘时仓根锚应命中 %s，得 %s", root, got)
	}
}

// ── 退役态（B轨 · 2026-10-07）：分档真源件退役 ⇒ K1 不过问（✗ 不 fail-closed）─────────────────
// ★ 牙：哑化 `cicgProfileRetired`（恒 false）⇒ 本用例第 ② 段必 FAIL 带行号。
func TestCICGProfileRetiredPredicate(t *testing.T) {
	// ① 在场 ⇒ 未退役
	present := t.TempDir()
	pd := filepath.Join(present, "scripts", "gates")
	if err := os.MkdirAll(pd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pd, "check-manifest-freshness-profile.py"), []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if cicgProfileRetired(present) {
		t.Fatalf("在场件被误判为已退役")
	}
	// ② 缺席 + Go 半在盘 ⇒ 已退役（不过问）
	retired := t.TempDir()
	gh := filepath.Join(retired, "core", "cmd", "zerg", "check_manifest_freshness_profile.go")
	if err := os.MkdirAll(filepath.Dir(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gh, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !cicgProfileRetired(retired) {
		t.Fatalf("缺席件 + Go 半在盘应判为已退役")
	}
	// ③ 负控（真消失）：既无 py 又无 Go 半 ⇒ 未退役（fail-closed 保留）
	if cicgProfileRetired(t.TempDir()) {
		t.Fatalf("既无 py 又无 Go 半不应判为已退役（fail-closed 保留）")
	}
}

// ── 内化金标（DEV-2143 同口径）：分档真源 py 退场 ⇒ K1「单一口径」收回其 Go 半（进程内）· 两态自证 ──
//
// 为什么这么测：本门 K1「分档两态」原经**现调分档真源 py**（`scripts/gates/check-manifest-freshness-profile.py`）
// 才成立；该 py 已退场（2026-10-07 M2 退役批）⇒ 旧哨兵 `cicgRepoRootOrSkip` 恒 Skip（假绿、零保护）。
// 按 DEV-2143 先例把金标真源**收回 Go 半现役行为本身**：`cicgRunProfile` 在退役态改由本包姊妹件
// `check_manifest_freshness_profile.go` 的入口**进程内**承载（同入口 · ✗ 不新写第二份判据）。
//
//	★ 金标计算式（✗ 不臆造 · 由「Go 半现役行为」现算再冻）：逐格「(profile, 合成清单, --release-head)
//	  ⇒ (rc, ok)」由 `cicgRunProfile` 现跑捕获，冻成下表常量（dev=(0,true) · public=(1,true)）。
//
// 两态（✗ 不许 Skip）：A 态 = 真仓（退役 py 不在盘 · 其 Go 半在盘）；B 态 = 合成「旧态」夹具
// （py 缺席 · 其 Go 半在盘）⇒ 两态对同一套金标逐字同命中。
//
// ★ 哑化负控（"牙" · 见交付读数）：抽掉 `cicgProfileRetired`（恒 false）⇒ 本格 + `TestCICGProfileRetiredPredicate`
//   必 FAIL 且带行号（✗ 不真改仓内源）。

func TestPublicCiGreenGoldenInternalized(t *testing.T) {
	root := cicgRepoRoot(t)

	synth := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(synth, []byte(cicgSynthManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	relHead := strings.Repeat("1", 40)

	// A 态（真仓 · 退役 py 不在盘 · Go 半在盘）金标：dev 只告警 rc=0 · public 阻断 rc=1。
	dev, devOK := cicgRunProfile(root, "dev", synth, []string{"--release-head", relHead})
	pub, pubOK := cicgRunProfile(root, "public", synth, []string{"--release-head", relHead})
	if !devOK || dev != 0 {
		t.Errorf("金标 A · 开发机档：rc=%d ok=%v（冻值 0 / true）", dev, devOK)
	}
	if !pubOK || pub != 1 {
		t.Errorf("金标 A · 公开制品档：rc=%d ok=%v（冻值 1 / true）", pub, pubOK)
	}

	// B 态（合成旧态夹具：py 缺席 · 其 Go 半在盘）⇒ 同一金标逐字同命中。
	broot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(broot, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatal(err)
	}
	half, err := os.ReadFile(filepath.Join(root, "core", "cmd", "zerg", "check_manifest_freshness_profile.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broot, "core", "cmd", "zerg", "check_manifest_freshness_profile.go"), half, 0o644); err != nil {
		t.Fatal(err)
	}
	bdev, bdevOK := cicgRunProfile(broot, "dev", synth, []string{"--release-head", relHead})
	bpub, bpubOK := cicgRunProfile(broot, "public", synth, []string{"--release-head", relHead})
	if bdev != dev || bdevOK != devOK || bpub != pub || bpubOK != pubOK {
		t.Fatalf("两态金标不同：A=(%d,%v)/(%d,%v) B=(%d,%v)/(%d,%v)", dev, devOK, pub, pubOK, bdev, bdevOK, bpub, bpubOK)
	}

	// 门面自检（现调 Go 半 · 退役态）：必须全过（✗ 不再因退役 py 缺席而 3 格红）。
	rc, out, errb := cicgRunMain(t, "--self-test")
	if rc != 0 || !strings.Contains(out, "自检结论: 全过") {
		t.Fatalf("退役态 --self-test 应 rc=0 全过：rc=%d\nstdout:\n%s\nstderr:\n%s", rc, out, errb)
	}
}
