// check_empty_domain_guards_test.go —— 判据件 `scripts/gates/check-empty-domain-guards` `.py` 的 Go 真搬
// （`core/cmd/zerg/check_empty_domain_guards.go`）的**成对负控**测试（✗ 禁恒绿）。
//
// 覆盖（成对：正控 / 负控 / 边界各一格；三面 rc+stdout+stderr 逐字）：
//
//	Help_ExactBytes        · `-h` ⇒ rc=0 + 帮助正文逐字节（含 usage 换行 · 3.9 `optional arguments:`）
//	HelpAbbrev             · `--h` 前缀缩写 ⇒ 同帮助
//	UnknownFlag_Rc2        · `--nope` ⇒ rc=2 + usage + `unrecognized arguments: --nope`
//	DoubleDash_Unrecognized· `-- foo` ⇒ rc=2 + `unrecognized arguments: -- foo`（`--` 自身也在未识别列）
//	ExpectedOneArg         · `--root`（缺值，后无 token）⇒ rc=2 `expected one argument`
//	BadInt                 · `--baseline abc` ⇒ rc=2 `invalid int value: 'abc'`
//	StoreTrueWithInline    · `--json=1` ⇒ rc=2 `ignored explicit argument '1'`
//	MissingRoot_Rc2        · `--root /不存在` ⇒ rc=2 `BLOCKED … 扫描根不存在或读不到`
//	EmptyRoot_Rc2          · `--root` 空目录 ⇒ rc=2 `BLOCKED … 空域`（空转 ≠ 绿 · 本件自守此线）
//	GuardedFixture_Rc0     · 扫面型件带域级守卫 ⇒ rc=0（不点名）
//	UnguardedFixture_Rc1   · 扫面型件缺守卫 ⇒ rc=1（点名 · baseline=0）
//	UnrelatedLen_Negctl    · 只含无关 `len(x) >= 3` ⇒ **仍 rc=1**（不靠无关比较洗绿）
//	EmptyNoFc_Negctl       · 有空集检查但**未 fail-closed** ⇒ **仍 rc=1**
//	EmitBaseline           · `--emit-baseline` ⇒ 只印缺守卫数（rc=0）
//	Json_Fixture           · `--json` ⇒ 机器面（scanned/unguarded/… 键齐 · 空列表 `[]` 非 `null`）
//	RegistryStale_Rc1      · 登记项现读为扫面型 ⇒ rc=1 + `登记与结构不符`（未被豁免扫面）
//	SelfRefExcluded        · 扫面根内**同名** `check-empty-domain-guards.py` 被自指排除 ⇒ scanned 不含它
//	PrefixAbbrev           · `--roo <夹具>` 前缀缩写 ⇒ 命中 `--root`
//	BaselineRatchet        · 缺守卫数 > baseline ⇒ rc=1 / <= baseline ⇒ rc=0
//	SelfTest_Rc0           · `--self-test` ⇒ rc=0 + 十档全过 + 自检临时目录**自回收**（delta=0 · A 类）
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func edgT(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var o, e bytes.Buffer
	rc := checkEmptyDomainGuardsGateMain(&o, &e, args)
	return rc, o.String(), e.String()
}

func edgWrite(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败: %v", err)
	}
	return p
}

const edgGuardedBody = "import os\ndef main():\n    f=[]\n    for dp,_d,fs in os.walk('.'):\n        f+=fs\n    if require_domain(f):\n        return 2\n    return 0\n"
const edgUnguardedBody = "import os\ndef main():\n    f=[]\n    for dp,_d,fs in os.walk('.'):\n        f+=fs\n    return 0\n"

func edgCountSelfDirs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir(os.TempDir())
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "emptydomain-selftest-") {
			n++
		}
	}
	return n
}

// TestCheckEmptyDomainGuardsGate —— 成对负控总入口（定向跑：`-run '^TestCheckEmptyDomainGuardsGate$'`）。
func TestCheckEmptyDomainGuardsGate(t *testing.T) {
	t.Run("Help_ExactBytes", func(t *testing.T) {
		rc, out, errs := edgT(t, "-h")
		if rc != 0 || errs != "" || out != edgHelp {
			t.Fatalf("帮助面不符：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("HelpAbbrev", func(t *testing.T) {
		rc, out, errs := edgT(t, "--h")
		if rc != 0 || errs != "" || out != edgHelp {
			t.Fatalf("--h 缩写未命中帮助：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("UnknownFlag_Rc2", func(t *testing.T) {
		rc, out, errs := edgT(t, "--nope")
		if rc != 2 || out != "" {
			t.Fatalf("rc=%d out=%q", rc, out)
		}
		if errs != edgUsage+edgProg+": error: unrecognized arguments: --nope\n" {
			t.Fatalf("错误正文不符：%q", errs)
		}
	})

	t.Run("DoubleDash_Unrecognized", func(t *testing.T) {
		rc, out, errs := edgT(t, "--", "foo")
		if rc != 2 || out != "" || !strings.Contains(errs, "unrecognized arguments: -- foo\n") {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("ExpectedOneArg", func(t *testing.T) {
		rc, out, errs := edgT(t, "--root")
		if rc != 2 || out != "" || !strings.Contains(errs, "argument --root: expected one argument\n") {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("BadInt", func(t *testing.T) {
		rc, out, errs := edgT(t, "--baseline", "abc")
		if rc != 2 || out != "" || !strings.Contains(errs, "argument --baseline: invalid int value: 'abc'\n") {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("StoreTrueWithInline", func(t *testing.T) {
		rc, out, errs := edgT(t, "--json=1")
		if rc != 2 || out != "" || !strings.Contains(errs, "argument --json: ignored explicit argument '1'\n") {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("MissingRoot_Rc2", func(t *testing.T) {
		rc, out, errs := edgT(t, "--root", "/nonexistent-xyz")
		if rc != 2 || out != "" || errs != "BLOCKED —— 扫描根不存在或读不到：/nonexistent-xyz（rc=2 · 不给结论）\n" {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("EmptyRoot_Rc2", func(t *testing.T) {
		d := t.TempDir()
		rc, out, errs := edgT(t, "--root", d, "--baseline", "0")
		if rc != 2 || out != "" {
			t.Fatalf("空域应 rc=2：rc=%d out=%q err=%q", rc, out, errs)
		}
		if !strings.Contains(errs, "BLOCKED —— 空域 / 取不到输入：扫面根 "+d+" 下的 .py 件（现读 0 < 写死下限 1）⇒ rc=2 不给结论") {
			t.Fatalf("空域 fail-closed 正文不符：%q", errs)
		}
	})

	t.Run("GuardedFixture_Rc0", func(t *testing.T) {
		d := t.TempDir()
		edgWrite(t, d, "guarded.py", edgGuardedBody)
		rc, out, errs := edgT(t, "--root", d, "--baseline", "0")
		if rc != 0 || errs != "" {
			t.Fatalf("带守卫应 rc=0：rc=%d out=%q err=%q", rc, out, errs)
		}
		if !strings.Contains(out, "扫面型件 1 条 · **缺空集守卫 0 条**（baseline=0 · 只许下调）") {
			t.Fatalf("报话不符：%q", out)
		}
		if strings.Contains(out, "  ✗ guarded.py") {
			t.Fatalf("带守卫件被误点名：%q", out)
		}
	})

	t.Run("UnguardedFixture_Rc1", func(t *testing.T) {
		d := t.TempDir()
		edgWrite(t, d, "bad.py", edgUnguardedBody)
		rc, out, errs := edgT(t, "--root", d, "--baseline", "0")
		if rc != 1 || errs != "" {
			t.Fatalf("缺守卫应 rc=1：rc=%d out=%q err=%q", rc, out, errs)
		}
		if !strings.Contains(out, "  ✗ bad.py —— 扫面型但无「域级空集守卫」") {
			t.Fatalf("未点名缺守卫件：%q", out)
		}
		if !strings.Contains(out, "⇒ 缺守卫 1 > baseline 0 ⇒ rc=1（棘轮上调 = 口径变坏）") {
			t.Fatalf("棘轮结语不符：%q", out)
		}
	})

	t.Run("UnrelatedLen_Negctl", func(t *testing.T) {
		d := t.TempDir()
		edgWrite(t, d, "unrelated.py", "import os\ndef main():\n    f=[]\n    for dp,_d,fs in os.walk('.'):\n        f+=fs\n    parts='a_b_c'.split('_')\n    if len(parts) >= 3:\n        return 0\n    return 0\n")
		rc, out, errs := edgT(t, "--root", d, "--baseline", "0")
		if rc != 1 || errs != "" || !strings.Contains(out, "  ✗ unrelated.py") {
			t.Fatalf("只含无关 `len(x)>=3` 应仍 rc=1 且点名：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("EmptyNoFc_Negctl", func(t *testing.T) {
		d := t.TempDir()
		edgWrite(t, d, "nofc.py", "import os\ndef main():\n    f=[]\n    for dp,_d,fs in os.walk('.'):\n        f+=fs\n    if not f:\n        return 0\n    return 0\n")
		rc, out, errs := edgT(t, "--root", d, "--baseline", "0")
		if rc != 1 || errs != "" || !strings.Contains(out, "  ✗ nofc.py") {
			t.Fatalf("空集检查未 fail-closed 应仍 rc=1 且点名：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("EmitBaseline", func(t *testing.T) {
		d := t.TempDir()
		edgWrite(t, d, "bad.py", edgUnguardedBody)
		rc, out, errs := edgT(t, "--root", d, "--emit-baseline")
		if rc != 0 || out != "1\n" || errs != "" {
			t.Fatalf("--emit-baseline 应只印数：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("Json_Fixture", func(t *testing.T) {
		d := t.TempDir()
		edgWrite(t, d, "bad.py", edgUnguardedBody)
		rc, out, errs := edgT(t, "--root", d, "--baseline", "0", "--json")
		if rc != 1 || errs != "" {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
		want := `{"schema": "zerg/v1", "kind": "EmptyDomainGuards", "root": ` + edgJSONString(d) +
			`, "scanned": 1, "baseline": 0, "unguarded": 1, "unguarded_files": ["bad.py"], "subproc_unchecked": 0, ` +
			`"non_scan_reviewed": ["check-placeholder-residue.py", "check-manifest-freshness.py", "check-gotoolchain.py", ` +
			`"check-doc-section-numbers.py", "check-public-tree-hazards.py", "check_version.py"], "non_scan_reviewed_count": 6, ` +
			`"registry_stale": []}` + "\n"
		if out != want {
			t.Fatalf("JSON 逐字不符：\n got=%q\nwant=%q", out, want)
		}
	})

	t.Run("RegistryStale_Rc1", func(t *testing.T) {
		d := t.TempDir()
		edgWrite(t, d, "check_version.py", edgUnguardedBody) // 登记项现读为扫面型
		rc, out, errs := edgT(t, "--root", d, "--baseline", "14")
		if rc != 1 || errs != "" {
			t.Fatalf("登记与结构不符应 rc=1：rc=%d out=%q err=%q", rc, out, errs)
		}
		if !strings.Contains(out, "⇒ 登记与结构不符 1 门 ⇒ rc=1（登记过期，登记面须随动）") {
			t.Fatalf("登记结语不符：%q", out)
		}
		if !strings.Contains(out, "  ✗ check_version.py —— 已补件内 fail-closed") {
			t.Fatalf("登记项未被标 ✗：%q", out)
		}
	})

	t.Run("SelfRefExcluded", func(t *testing.T) {
		d := t.TempDir()
		// 同名自身件（无守卫、扫面型）应被**自指排除**；另一枚无守卫件应被发现。
		edgWrite(t, d, "check-empty-domain-guards.py", edgUnguardedBody)
		edgWrite(t, d, "other.py", edgUnguardedBody)
		rc, out, errs := edgT(t, "--root", d, "--baseline", "0")
		if rc != 1 || errs != "" {
			t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
		}
		if !strings.Contains(out, "扫面型件 1 条") {
			t.Fatalf("自指排除失效（scanned 应=1）：%q", out)
		}
		if strings.Contains(out, "check-empty-domain-guards.py ——") {
			t.Fatalf("同名自身件未被排除：%q", out)
		}
	})

	t.Run("PrefixAbbrev", func(t *testing.T) {
		d := t.TempDir()
		edgWrite(t, d, "bad.py", edgUnguardedBody)
		rc, out, errs := edgT(t, "--roo", d, "--baseline", "0")
		if rc != 1 || errs != "" || !strings.Contains(out, "  ✗ bad.py") {
			t.Fatalf("--roo 缩写未命中 --root：rc=%d out=%q err=%q", rc, out, errs)
		}
	})

	t.Run("BaselineRatchet", func(t *testing.T) {
		d := t.TempDir()
		edgWrite(t, d, "bad.py", edgUnguardedBody)
		if rc, _, _ := edgT(t, "--root", d, "--baseline", "1"); rc != 0 {
			t.Fatalf("缺守卫 1 <= baseline 1 应 rc=0，实得 %d", rc)
		}
		if rc, _, _ := edgT(t, "--root", d, "--baseline", "0"); rc != 1 {
			t.Fatalf("缺守卫 1 > baseline 0 应 rc=1，实得 %d", rc)
		}
	})

	t.Run("SelfTest_Rc0", func(t *testing.T) {
		t.Setenv("TMPDIR", t.TempDir()) // 私有计数面（只数本用例自建 tmp 目录）
		before := edgCountSelfDirs(t)
		rc, out, errs := edgT(t, "--self-test")
		after := edgCountSelfDirs(t)
		if rc != 0 {
			t.Fatalf("--self-test 应 rc=0：rc=%d out=%q err=%q", rc, out, errs)
		}
		if !strings.HasSuffix(out, "── 自检：全过 ✓\n") {
			t.Fatalf("自检结语不符：%q", out)
		}
		if !strings.Contains(out, "档⑥′ 登记表") {
			t.Fatalf("自检档缺失：%q", out)
		}
		if !strings.Contains(errs, "BLOCKED —— 空域 / 取不到输入：合成·空域（现读 0 < 写死下限 1）⇒ rc=2 不给结论\n") {
			t.Fatalf("自检 stderr（档② 负控）不符：%q", errs)
		}
		if after != before {
			t.Fatalf("自检临时目录未回收：before=%d after=%d（A 类泄漏）", before, after)
		}
	})
}
