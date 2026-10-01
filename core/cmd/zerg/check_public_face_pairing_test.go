// 成对负控测试件：`check_public_face_pairing.go`（= py `scripts/gates/check-public-face-pairing.py` 的真搬）。
// 口径：每条用例都钉「有牙」—— 仓外哑化对应实现 ⇒ 本件对应子用例 FAIL（带行号，见交付）。
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func cpfpT_run(argv []string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkPublicFacePairingGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// cpfpT_repoRoot：从 cwd 上溯找含登记面真身的仓根（登记面单源用例需要）。
func cpfpT_repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(cpfpPyJoin(d, cpfpRegistrationRel)); err == nil {
			return d
		}
		nd := cpfpPyDirname(d)
		if nd == d || nd == "" {
			break
		}
		d = nd
	}
	t.Fatalf("上溯 8 层没找到 %s（cwd=%s）", cpfpRegistrationRel, wd)
	return ""
}

func cpfpT_fixture(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	root, reg, ok := cpfpMkFixture(base, os.Stderr)
	if !ok {
		t.Fatal("夹具建不起来")
	}
	return root, reg
}

// ── 自检（15 格断言 · 笔面 / 分支面 / 空转 / 缺件 / rev / 登记面缺件全覆盖）──────────────
func TestCheckPublicFacePairing_SelfTestAllFifteen(t *testing.T) {
	rc, so, se := cpfpT_run([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("--self-test 应 rc=0，得 %d（stderr=%s）", rc, se)
	}
	if !strings.Contains(so, "自检 ✓ 断言 15 格全过（失败 0 格）") {
		t.Fatalf("自检尾句不符：\n%s", so)
	}
	if n := strings.Count(so, " rc="); n != 15 {
		t.Fatalf("自检逐格行应 15（每格一行 ` rc=`），得 %d\n%s", n, so)
	}
	if strings.Contains(so, "自检 ✗") {
		t.Fatalf("自检出现 ✗ 格：\n%s", so)
	}
}

// ── 拒收开关（4 枚 · 一律 rc=2 且逐字点名）────────────────────────────────────────
func TestCheckPublicFacePairing_RefuseFlags(t *testing.T) {
	for _, f := range cpfpRefuseFlags {
		rc, so, _ := cpfpT_run([]string{f})
		if rc != cpfpExitBlocked {
			t.Errorf("`%s` 应 rc=2，得 %d", f, rc)
		}
		want := "⛔ 拒收：`" + f + "` —— 例外只能**显式登记**（登记面在 " + cpfpRegistrationRel + " 里）⇒ **不给结论（rc=2）**\n"
		if so != want {
			t.Errorf("`%s` 正文不逐字：\n要 %q\n得 %q", f, want, so)
		}
	}
}

// ── 参数面（未知旗标 / 缺值 / 真旗标显式值 / -h）───────────────────────────────────
func TestCheckPublicFacePairing_ArgSurface(t *testing.T) {
	rc, _, se := cpfpT_run([]string{"--bogus"})
	if rc != cpfpExitBlocked || !strings.HasPrefix(se, cpfpUsage+"\n") ||
		!strings.Contains(se, "check-public-face-pairing.py: error: unrecognized arguments: --bogus") {
		t.Errorf("--bogus 面不符：rc=%d stderr=%q", rc, se)
	}
	rc, _, se = cpfpT_run([]string{"--root"})
	if rc != cpfpExitBlocked || !strings.Contains(se, "argument --root: expected one argument") {
		t.Errorf("--root 缺值面不符：rc=%d stderr=%q", rc, se)
	}
	rc, _, se = cpfpT_run([]string{"--json=1"})
	if rc != cpfpExitBlocked || !strings.Contains(se, "argument --json: ignored explicit argument '1'") {
		t.Errorf("--json=1 面不符：rc=%d stderr=%q", rc, se)
	}
	rc, so, _ := cpfpT_run([]string{"-h"})
	if rc != 0 || so != cpfpHelpTextRaw {
		t.Errorf("-h 帮助正文不逐字（rc=%d · len got=%d want=%d）", rc, len(so), len(cpfpHelpTextRaw))
	}
}

// ── 判据主体（夹具 · 正控 / 负控 / 登记例外 / 未登记单语件）──────────────────────────
func TestCheckPublicFacePairing_PairJudgement(t *testing.T) {
	root, reg := cpfpT_fixture(t)
	base := []string{"--no-self-test", "--root", root, "--registration", reg}

	rc, so, _ := cpfpT_run(base)
	if rc != cpfpExitOK || !strings.Contains(so, "P3 正控：改动面里 **0 件**公开面件（不碰任何公开面件）⇒ rc=0") {
		t.Fatalf("正控应 rc=0 且零件公开面：rc=%d\n%s", rc, so)
	}

	// 只改 zh：必须逐字点名「只改 zh 不改 en」且计数恰 1（钉住 dict 去重语义）
	if err := os.WriteFile(cpfpPyJoin(root, "publish/docs/A.zh-CN.md"), []byte("# A 真源（改）\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, so, _ = cpfpT_run(base)
	if rc != cpfpExitRed {
		t.Fatalf("只改 zh 应 rc=1，得 %d\n%s", rc, so)
	}
	if !strings.Contains(so, "✗ P2 负控命中：只改 zh 不改 en：A.zh-CN.md 在改动面里，而 A.en.md **一字未动**") {
		t.Errorf("缺「只改 zh」逐字点名：\n%s", so)
	}
	if !strings.Contains(so, "**有红（rc=1）**：1 处单边改动") {
		t.Errorf("单边改动计数应恰 1 处（dict 去重）：\n%s", so)
	}
	// 成对同批 ⇒ 回绿
	if err := os.WriteFile(cpfpPyJoin(root, "publish/docs/A.en.md"), []byte("# A derived（改）\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, so, _ = cpfpT_run(base)
	if rc != cpfpExitOK || !strings.Contains(so, "同批 ✓ A.zh-CN.md + A.en.md") {
		t.Errorf("成对同批应 rc=0 且打印同批：rc=%d\n%s", rc, so)
	}
}

func TestCheckPublicFacePairing_ExceptionRegistration(t *testing.T) {
	root, reg := cpfpT_fixture(t)
	base := []string{"--no-self-test", "--root", root, "--registration", reg}
	// 登记面派生的负向 glob 必须逐字出现
	rc, so, _ := cpfpT_run(base)
	if rc != cpfpExitOK {
		t.Fatalf("rc=%d\n%s", rc, so)
	}
	if !strings.Contains(so, "!publish/docs/design/**/*.md   ← 已登记的单语例外（design/格式反馈协议-FFP.md）") {
		t.Errorf("负向 glob 派生行不符：\n%s", so)
	}
	// 登记面缺件 ⇒ rc=2（缺件 ≠ 绿）
	rc, so, _ = cpfpT_run([]string{"--no-self-test", "--root", root, "--registration", "scripts/gates/no-such-reg.py"})
	if rc != cpfpExitBlocked || !strings.Contains(so, "⛔ 缺件：例外登记面 scripts/gates/no-such-reg.py 不在") {
		t.Errorf("登记面缺件应 rc=2 且点名：rc=%d\n%s", rc, so)
	}
}

// ── 登记面是「单一真源」（另一个判据件里的常量表）⇒ 真仓现读必须解出 2 件设计例外 ────────
func TestCheckPublicFacePairing_RegistrationSingleSourceRealRepo(t *testing.T) {
	repo := cpfpT_repoRoot(t)
	rows, err := cpfpLoadRegisteredExceptions(cpfpRegistrationRel, repo)
	if err != nil {
		t.Fatalf("真仓登记面应可解：%v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("真仓登记面应 2 条，得 %d（%+v）", len(rows), rows)
	}
	want := map[string]bool{"design/格式反馈协议-FFP.md": true, "design/记忆体系.md": true}
	for _, r := range rows {
		if !want[r.File] {
			t.Errorf("登记面多出/错件：%q", r.File)
		}
		if r.Reason == "" {
			t.Errorf("登记面 %q 的 reason 掉了（解析器没吃下相邻字符串拼接）", r.File)
		}
	}
}

// ── `--json` 逐字形状（含空集合与键序 = sort_keys）─────────────────────────────────
func TestCheckPublicFacePairing_JSONShape(t *testing.T) {
	root, reg := cpfpT_fixture(t)
	rc, so, _ := cpfpT_run([]string{"--no-self-test", "--json", "--root", root, "--registration", reg})
	if rc != cpfpExitOK {
		t.Fatalf("rc=%d\n%s", rc, so)
	}
	want := "{\"changed_all\": 0, \"changed_face\": 0, \"exceptions\": 1, \"face\": \"publish/docs\", " +
		"\"files\": 6, \"pair_files\": 4, \"pairs\": 2, \"rc\": 0, \"rev\": \"HEAD\", \"unpaired\": 1, \"untracked\": 0}\n"
	if so != want {
		t.Errorf("--json 不逐字：\n要 %q\n得 %q", want, so)
	}
}

// ── py 路径原语三态（join 的 `..` / 后段绝对 / 空段 + normpath 双斜杠 + relpath）──────────
func TestCheckPublicFacePairing_PathSemantics(t *testing.T) {
	if got := cpfpPyJoin("a", "../b"); got != "a/../b" {
		t.Errorf("join 的 `..` 原样留：得 %q", got)
	}
	if got := cpfpPyJoin("repo", "/abs"); got != "/abs" {
		t.Errorf("join 后段绝对应丢前段：得 %q", got)
	}
	if got := cpfpPyJoin("a", "", "b"); got != "a/b" {
		t.Errorf("join 空段：得 %q", got)
	}
	if got := cpfpPyJoin("a", ""); got != "a/" {
		t.Errorf("join 尾空段保留尾斜杠：得 %q", got)
	}
	if got := cpfpPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Errorf("normpath 恰两前导斜杠保留：得 %q", got)
	}
	if got := cpfpPyNormpath("///a/b"); got != "/a/b" {
		t.Errorf("normpath 三斜杠折一：得 %q", got)
	}
	if got := cpfpPyRelpath("/r/publish/docs/A.zh-CN.md", "/r/publish/docs"); got != "A.zh-CN.md" {
		t.Errorf("relpath：得 %q", got)
	}
	if got := cpfpPyRelpath("/r/x/y", "/r/publish/docs"); got != "../../x/y" {
		t.Errorf("relpath 的 .. 前缀：得 %q", got)
	}
}
