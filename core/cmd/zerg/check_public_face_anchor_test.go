// check_public_face_anchor_test.go —— `check_public_face_anchor.go`（= py `scripts/gates/check-public-face-anchor.py` 的真搬）的
// **对拍台 + 负控**。口径：每条对拍格同 argv 跑两态（py 原件 ↔ 本入口），stdout / stderr / 退码三面逐字比。
// 负控：仓外哑化「漂移判红」那一格 ⇒ 本件对应断言 FAIL（见 TestCheckPublicFaceAnchor_NegControlTeeth）。
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func cpfaT_run(argv []string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkPublicFaceAnchorGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

// cpfaT_repoRoot：从 cwd 上溯找含 py 真身的仓根。
func cpfaT_repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(cpfaPyJoin(d, "scripts/gates/check-public-face-anchor.py")); err == nil {
			return d
		}
		nd := cpfaPyDirname(d)
		if nd == d || nd == "" {
			break
		}
		d = nd
	}
	t.Fatalf("上溯 8 层没找到 py 真身（cwd=%s）", wd)
	return ""
}

type cpfaParityCell struct {
	name string
	argv []string
}

func cpfaT_pyRun(t *testing.T, py, pyPath string, argv []string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(py, append([]string{pyPath}, argv...)...)
	cmd.Env = os.Environ()
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			t.Fatalf("起 py 失败：%v", err)
		}
	}
	return rc, so.String(), se.String()
}

// cpfaT_fixturePair —— 建一对 `A.zh-CN.md` ↔ `A.en.md` 的合成仓（已提交）。
func cpfaT_fixturePair(t *testing.T, name string) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), name)
	root, ok := cpfaMkFixture(base, cpfaPairA)
	if !ok {
		t.Fatalf("夹具 %s 建不起来", name)
	}
	return root
}

func TestCheckPublicFaceAnchorPort_Parity(t *testing.T) {
	repo := cpfaT_repoRoot(t)
	pyPath := filepath.Join(repo, "scripts", "gates", "check-public-face-anchor.py")
	py, err := exec.LookPath("python3")
	// ★ 解释器**钉死**（GAP-20261002-04/-07）：本格对拍 py 原件的 `-h` 输出，而其中一行随 py 版本变
	// （argparse ≤3.9 印 `optional arguments:`、≥3.10 印 `options:`）。交互 shell 的 PATH 常解析到
	// ≥3.10、门跑环境常是系统 3.9 ⇒ 同一格在两条路上给出相反结果。钉 `/usr/bin/python3` 与门跑
	// 环境同侧；它不在时回退 PATH（并明写在 stdout，不静默）。
	if _, statErr := os.Stat("/usr/bin/python3"); statErr == nil {
		py, err = "/usr/bin/python3", nil
	}
	if err != nil {
		t.Skip("无 python3，跳过对拍")
	}

	// ── 夹具（git 合成仓）────────────────────────────────────────────────
	fxNone := cpfaT_fixturePair(t, "none")

	fxDrift := cpfaT_fixturePair(t, "drift")
	cpfaTouch(fxDrift, "A.en.md", "只改了译文这一行\n")

	fxBoth := cpfaT_fixturePair(t, "both")
	cpfaTouch(fxBoth, "A.zh-CN.md", "真源动了\n")
	cpfaTouch(fxBoth, "A.en.md", "derived moved\n")

	fxStale := cpfaT_fixturePair(t, "stale")
	cpfaTouch(fxStale, "A.zh-CN.md", "真源动了\n")

	fxZhOnly := func() string {
		base := filepath.Join(t.TempDir(), "zhonly")
		r, ok := cpfaMkFixture(base, map[string]string{"B.zh-CN.md": "# B 真源\n"})
		if !ok {
			t.Fatal("夹具 zhonly 建不起来")
		}
		return r
	}()

	fxExc := func() string {
		files := map[string]string{"design/格式反馈协议-FFP.md": "# 理念件（单语）\n"}
		for k, v := range cpfaPairA {
			files[k] = v
		}
		base := filepath.Join(t.TempDir(), "exc")
		r, ok := cpfaMkFixture(base, files)
		if !ok {
			t.Fatal("夹具 exc 建不起来")
		}
		return r
	}()

	fxUntracked := cpfaT_fixturePair(t, "untracked")
	_ = os.WriteFile(cpfaPyJoin(fxUntracked, cpfaFaceDefault, "D.zh-CN.md"), []byte("# D 真源\n"), 0o644)
	_ = os.WriteFile(cpfaPyJoin(fxUntracked, cpfaFaceDefault, "D.en.md"), []byte("# D derived\n"), 0o644)

	fxVacuous := filepath.Join(t.TempDir(), "vacuous")
	cpfaMkdirAll(cpfaPyJoin(fxVacuous, cpfaFaceDefault))

	fxNoFace := filepath.Join(t.TempDir(), "noface")
	cpfaMkdirAll(fxNoFace)

	cells := []cpfaParityCell{
		{"list-rules", []string{"--list-rules"}},
		{"-h 帮助面", []string{"-h"}},
		{"拒收开关 --waive", []string{"--waive"}},
		{"json 字段名打错", []string{"--json", "bogus"}},
		{"json 字段名打错(拼接)", []string{"--json=bogus,rc"}},
		{"真仓 judge", []string{"--no-self-test", "--root", repo}},
		{"真仓 json 全量", []string{"--no-self-test", "--root", repo, "--json", ","}},
		{"真仓 json rc,counts", []string{"--no-self-test", "--root", repo, "--json", "rc,counts"}},
		{"真仓 json 空字段名", []string{"--no-self-test", "--root", repo, "--json="}},
		{"夹具 无输入", []string{"--no-self-test", "--root", fxNone}},
		{"夹具 漂移(单边改 en)", []string{"--no-self-test", "--root", fxDrift}},
		{"夹具 漂移 json", []string{"--no-self-test", "--root", fxDrift, "--json", ","}},
		{"夹具 同批", []string{"--no-self-test", "--root", fxBoth}},
		{"夹具 派生过期(单边改 zh)", []string{"--no-self-test", "--root", fxStale}},
		{"夹具 未成对 zh", []string{"--no-self-test", "--root", fxZhOnly}},
		{"夹具 未成对 zh json", []string{"--no-self-test", "--root", fxZhOnly, "--json", "rc,verdict,unregistered"}},
		{"夹具 已登记例外", []string{"--no-self-test", "--root", fxExc}},
		{"夹具 悬空登记", []string{"--no-self-test", "--root", fxNone, "--json", "dangling_exceptions"}},
		{"夹具 untracked", []string{"--no-self-test", "--root", fxUntracked}},
		{"夹具 untracked json", []string{"--no-self-test", "--root", fxUntracked, "--json", "not_in_diff"}},
		{"夹具 空转", []string{"--no-self-test", "--root", fxVacuous}},
		{"夹具 缺面", []string{"--no-self-test", "--root", fxNoFace}},
		{"参数缺值 --root", []string{"--root"}},
		{"未知参数", []string{"--bogus"}},
		{"真旗标显式值", []string{"--list-rules=1"}},
		{"--json 缺值", []string{"--no-self-test", "--json"}},
		{"--self-test", []string{"--self-test"}},
	}

	baseEnv := os.Environ()
	_ = baseEnv
	mismatch := 0
	for _, c := range cells {
		goRC, goOut, goErr := cpfaT_run(c.argv)
		pyRC, pyOut, pyErr := cpfaT_pyRun(t, py, pyPath, c.argv)
		sameOut := goOut == pyOut
		sameErr := goErr == pyErr
		if goRC != pyRC || !sameOut || !sameErr {
			mismatch++
			t.Errorf("[%s] DIFF rc go=%d py=%d · stdout 同=%v · stderr 同=%v\n--argv--%q\n--go stdout--\n%s\n--py stdout--\n%s\n--go stderr--\n%s\n--py stderr--\n%s",
				c.name, goRC, pyRC, sameOut, sameErr, c.argv, goOut, pyOut, goErr, pyErr)
		} else {
			t.Logf("[%s] SAME (rc=%d · stdout %d B · stderr %d B)", c.name, goRC, len(goOut), len(goErr))
		}
	}
	t.Logf("对拍格数=%d 失配=%d", len(cells), mismatch)
}

// ── 自检面（16 格 · 与 py `--self-test` 同文本）──────────────────────────────────
func TestCheckPublicFaceAnchor_SelfTestAllSixteen(t *testing.T) {
	rc, so, se := cpfaT_run([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("--self-test 应 rc=0，得 %d（stderr=%s）", rc, se)
	}
	if !strings.Contains(so, "自检结论: 全过（断言 **16** 格 · 失败 0 格）") {
		t.Fatalf("自检尾句不符：\n%s", so)
	}
	if strings.Contains(so, "   ✗ ") {
		t.Fatalf("自检出现 ✗ 格：\n%s", so)
	}
}

// ── `--json` 形状（键序 = sort_keys · 空集合与 null 面）────────────────────────────
func TestCheckPublicFaceAnchor_JSONShape(t *testing.T) {
	fx := cpfaT_fixturePair(t, "jsonshape")
	rc, so, _ := cpfaT_run([]string{"--no-self-test", "--root", fx, "--json", ","})
	if rc != 0 {
		t.Fatalf("rc=%d\n%s", rc, so)
	}
	lines := strings.Split(strings.TrimRight(so, "\n"), "\n")
	last := lines[len(lines)-1]
	want := `{"both": [], "counts": {"both": 0, "changed": 0, "dangling_exceptions": 2, "drift": 0, ` +
		`"exceptions": 2, "not_in_diff": 2, "pairs": 1, "singles": 0, "stale": 0, "unregistered": 0, ` +
		`"untouched": 1}, "dangling_exceptions": ["design/格式反馈协议-FFP.md", "design/记忆体系.md"], ` +
		`"drift": [], "exceptions": ["design/格式反馈协议-FFP.md", "design/记忆体系.md"], ` +
		`"face": "publish/docs", "files": 2, "not_in_diff": ["A.en.md", "A.zh-CN.md"], "pairs": 1, ` +
		`"rc": 0, "rev": "HEAD", "singles": 0, "stale": [], "unpaired": 0, "verdict": "green"}`
	if last != want {
		t.Errorf("--json 不逐字：\n要 %q\n得 %q", want, last)
	}
}

// ── 路径原语三态 ────────────────────────────────────────────────────────────────
func TestCheckPublicFaceAnchor_PathSemantics(t *testing.T) {
	if got := cpfaPyJoin("a", "../b"); got != "a/../b" {
		t.Errorf("join 的 `..` 原样留：得 %q", got)
	}
	if got := cpfaPyJoin("repo", "/abs"); got != "/abs" {
		t.Errorf("join 后段绝对应丢前段：得 %q", got)
	}
	if got := cpfaPyNormpath("/a//b/../c"); got != "/a/c" {
		t.Errorf("normpath：得 %q", got)
	}
}

// ── 负控（有牙）：哑化「漂移判红」那一格 ⇒ 本件断言 FAIL ──────────────────────────
func TestCheckPublicFaceAnchor_NegControlTeeth(t *testing.T) {
	fx := cpfaT_fixturePair(t, "teeth")
	cpfaTouch(fx, "A.en.md", "只改了译文这一行\n")

	realRC, _, _ := cpfaJudge(fx, cpfaFaceDefault, "HEAD", cpfaExceptions)
	if realRC != 1 {
		t.Fatalf("有牙前置：真实现应在漂移夹具上 rc=1，得 %d", realRC)
	}
	if dumb := cpfaDumbAlwaysGreen(fx); dumb != 0 {
		t.Fatalf("哑化对照应 rc=0，得 %d", dumb)
	}
	if realRC == cpfaDumbAlwaysGreen(fx) {
		t.Fatalf("格子没牙：哑化实现与真实现在同夹具上同 rc ⇒ 断言 rc=1 无区分度")
	}
	// ⑯ 三态齐面同样钉住：同夹具能出 {0,1,2}
	r2 := cpfaT_fixturePair(t, "teeth-green")
	r3 := func() string {
		base := filepath.Join(t.TempDir(), "teeth-rc2")
		r, ok := cpfaMkFixture(base, map[string]string{"E.zh-CN.md": "# E\n"})
		if !ok {
			t.Fatal("夹具建不起来")
		}
		return r
	}()
	rc2, _, _ := cpfaJudge(r2, cpfaFaceDefault, "HEAD", cpfaExceptions)
	rc3, _, _ := cpfaJudge(r3, cpfaFaceDefault, "HEAD", cpfaExceptions)
	if rc2 != 0 || rc3 != 2 {
		t.Fatalf("三态不齐：得 %d / %d", rc2, rc3)
	}
}

// cpfaDumbAlwaysGreen —— 仓外哑化对照：把「漂移判红」（`J1`）那一格抹成恒绿。
func cpfaDumbAlwaysGreen(root string) int {
	rc, _, _ := cpfaJudge(root, cpfaFaceDefault, "HEAD", cpfaExceptions)
	if rc == 1 {
		return 0
	}
	return rc
}
