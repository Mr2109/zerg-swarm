package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// 成对负控测试件：`check_public_face_wording.go`（= py `scripts/gates/check-public-face-wording.py` 的真搬）。
// 口径：每条用例都钉「有牙」——哑化对应实现 ⇒ 本件对应子用例 FAIL（仓外哑化实证见交付）。

func cpfwT_run(argv []string) (int, string, string) {
	var so, se bytes.Buffer
	rc := checkPublicFaceWordingGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

var cpfwTRootFiles = []string{
	"LICENSE", "NOTICE", "THIRD_PARTY_LICENSES.md", "CONTRIBUTING.md",
	"SECURITY.md", "config.example.yaml", "gitleaks.toml", "install.sh",
}

func cpfwT_fixture(t *testing.T, docs map[string]string) string {
	t.Helper()
	base := t.TempDir()
	if err := os.MkdirAll(cpfwPyJoin(base, cpfwDocsRel), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, txt := range docs {
		p := cpfwPyJoin(base, cpfwDocsRel, rel)
		_ = os.MkdirAll(cpfwDirname(p), 0o755)
		if err := os.WriteFile(p, []byte(txt), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range cpfwTRootFiles {
		p := cpfwPyJoin(base, "publish", rel)
		_ = os.MkdirAll(cpfwDirname(p), 0o755)
		if err := os.WriteFile(p, []byte("合成根面件（干净）\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return base
}

// cpfwT_repoRoot：从 cwd 上溯找含判据原件的仓根（`--self-test` 的 ⑭/⑮ 需 cwd == 仓根）。
func cpfwT_repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	d := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(cpfwPyJoin(d, cpfwSelfRel)); err == nil {
			return d
		}
		nd := cpfwDirname(d)
		if nd == d {
			break
		}
		d = nd
	}
	t.Skipf("上溯不到含 %s 的仓根（跳过需 cwd 的用例）", cpfwSelfRel)
	return ""
}

func TestCpfwS_自检全过_成对负控二十格(t *testing.T) {
	root := cpfwT_repoRoot(t)
	old, _ := os.Getwd()
	defer func() { _ = os.Chdir(old) }()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cpfwT_run([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("--self-test 应 rc=0，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "自检结论: 全过") {
		t.Fatalf("--self-test 应打出「自检结论: 全过」\n%s", out)
	}
	if strings.Count(out, "   ✓ ") < 20 {
		t.Fatalf("自检实跑格数 < 20（成对负控格缺失）\n%s", out)
	}
}

func TestCpfwS_负控A1私有卷有牙(t *testing.T) {
	fx := cpfwT_fixture(t, map[string]string{"README.md": "挂载点 = " + cpfwVolPrefix + "AHZ 备份\n"})
	rc, out, _ := cpfwT_run([]string{"--root", fx, "--no-self-test"})
	if rc != 1 {
		t.Fatalf("A1 私有卷名应 rc=1，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "**未豁免 1**") {
		t.Fatalf("应点名未豁免 1 处\n%s", out)
	}
}

func TestCpfwS_负控A2未登记卷名网兜有牙(t *testing.T) {
	fx := cpfwT_fixture(t, map[string]string{"README.md": "示例：" + cpfwVolPrefix + "Data/x.db\n"})
	rc, out, _ := cpfwT_run([]string{"--root", fx, "--no-self-test"})
	if rc != 1 {
		t.Fatalf("未登记卷名应 rc=1（放行必须逐条登记），得 %d\n%s", rc, out)
	}
}

func TestCpfwS_白名单不是全局放行(t *testing.T) {
	fx := cpfwT_fixture(t, map[string]string{
		"QUICKSTART.md": " ZERG_KB_PATH=\"/Volumes/My Disk/knowledge.db\"\n",
		"OTHER.md":      " X=\"/Volumes/My Disk/other.db\"\n",
	})
	rc, out, _ := cpfwT_run([]string{"--root", fx, "--no-self-test"})
	if rc != 1 {
		t.Fatalf("同一字面量落在未登记件应 rc=1，得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "↳ 本条豁免理由：") {
		t.Fatalf("命中豁免应逐条打印理由（不许静默过滤）\n%s", out)
	}
}

func TestCpfwS_成对_助手登记件豁免与未登记件红(t *testing.T) {
	okFx := cpfwT_fixture(t, map[string]string{"design/记忆体系.md": "| `memory` | 助手自己的笔记 | `MEMORY.md` |\n"})
	rc, out, _ := cpfwT_run([]string{"--root", okFx, "--no-self-test"})
	if rc != 0 {
		t.Fatalf("已登记的件×行应 rc=0（豁免 1），得 %d\n%s", rc, out)
	}
	badFx := cpfwT_fixture(t, map[string]string{"README.md": "由助手分派这件事（未登记件）\n"})
	rc, out, _ = cpfwT_run([]string{"--root", badFx, "--no-self-test"})
	if rc != 1 {
		t.Fatalf("未登记件里的同一字面量应 rc=1，得 %d\n%s", rc, out)
	}
}

func TestCpfwS_守卫_缺扫面根退二(t *testing.T) {
	fx := cpfwT_fixture(t, map[string]string{"README.md": "# 干净\n"})
	if err := os.RemoveAll(cpfwPyJoin(fx, cpfwDocsRel)); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := cpfwT_run([]string{"--root", fx, "--no-self-test"})
	if rc != 2 {
		t.Fatalf("扫面根不存在应 rc=2（缺件 ≠ 绿），得 %d\n%s", rc, out)
	}
}

func TestCpfwS_守卫_扫面零件退二(t *testing.T) {
	rc, _, d := cpfwJudge(t.TempDir(), []string{}, true, true)
	if rc != 2 {
		t.Fatalf("扫面 0 件应 rc=2（空转拒绝给结论），得 %d", rc)
	}
	if d.hits != 0 || d.exempt != 0 || d.unexempt != 0 {
		t.Fatalf("0 件档的 data 应全 0，得 %+v", d)
	}
}

func TestCpfwS_守卫_件读不了退二(t *testing.T) {
	fx := cpfwT_fixture(t, map[string]string{"README.md": "# 干净\n"})
	p := cpfwPyJoin(fx, cpfwDocsRel, "README.md")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Skipf("chmod 不支持：%v", err)
	}
	if _, err := os.ReadFile(p); err == nil {
		t.Skip("本环境读 000 件仍成功（如 root）⇒ 该格不成立")
	}
	rc, out, _ := cpfwT_run([]string{"--root", fx, "--no-self-test"})
	if rc != 2 {
		t.Fatalf("件读不了应 rc=2（读不全就判不出真值），得 %d\n%s", rc, out)
	}
	if !strings.Contains(out, "件读不了") {
		t.Fatalf("应点名「件读不了」\n%s", out)
	}
}

func TestCpfwS_JSON_空集合是花括号不是null(t *testing.T) {
	fx := cpfwT_fixture(t, map[string]string{"README.md": "# 干净件（无禁词）\n"})
	rc, out, _ := cpfwT_run([]string{"--root", fx, "--json", "face,rules,hits,exempt,unexempt", "--no-self-test"})
	if rc != 0 {
		t.Fatalf("干净夹具应 rc=0，得 %d\n%s", rc, out)
	}
	last := strings.TrimRight(out, "\n")
	last = last[strings.LastIndex(last, "\n")+1:]
	want := `{"exempt": 0, "face": 9, "hits": 0, "rules": {}, "unexempt": 0}`
	if last != want {
		t.Fatalf("空集合形态应逐字 == %s，得 %s", want, last)
	}
	if strings.Contains(last, "null") || strings.Contains(last, "[]") {
		t.Fatalf("空集合不得输出 null/[]（py 给 {}）：%s", last)
	}
}

func TestCpfwS_JSON_字段名打错退二(t *testing.T) {
	rc, out, errb := cpfwT_run([]string{"--json", "bogus"})
	if rc != 2 {
		t.Fatalf("`--json` 字段名打错应 rc=2，得 %d\n%s", rc, out)
	}
	if !strings.Contains(errb, "不认的字段名") || !strings.Contains(errb, "可选字段表") {
		t.Fatalf("应印「不认的字段名 + 可选字段表」到 stderr\n%s", errb)
	}
}

func TestCpfwS_自豁免牙齿_关闭即红(t *testing.T) {
	root := cpfwT_repoRoot(t)
	rcOpen, _, _ := cpfwJudge(root, []string{cpfwSelfRel}, true, true)
	if rcOpen != 0 {
		t.Fatalf("开着自豁免应 rc=0，得 %d", rcOpen)
	}
	rcClosed, _, d := cpfwJudge(root, []string{cpfwSelfRel}, true, false)
	if rcClosed != 1 || d.unexempt == 0 {
		t.Fatalf("关掉自豁免应 rc=1 且未豁免 >0（这一格有区分度），得 rc=%d unexempt=%d", rcClosed, d.unexempt)
	}
}

// 定向单测：`cpfwPyJoin` 与 CPython posixpath.join 的**三语义差**各钉一格
// （① 中间 `..` 原样留 ② 后段绝对丢前段 ③ 空段保留尾 `/`）—— ✗ 不用 filepath.Join。
func TestCpfwS_PyJoin三语义差(t *testing.T) {
	cases := []struct {
		parts []string
		want  string
	}{
		{[]string{"a", "../b"}, "a/../b"},  // ① 中间 `..` 原样留（Go 归一成 b）
		{[]string{"repo", "/abs"}, "/abs"}, // ② 后段绝对丢前段（Go 拼成 repo/abs）
		{[]string{"a", "", "b"}, "a/b"},    // ③ 空段：夹在中间被吞
		{[]string{"a", ""}, "a/"},          // ③ 空段：尾段保 `/`
		{[]string{"", "a"}, "a"},           // ③ 空段：首段空
	}
	for _, c := range cases {
		if got := cpfwPyJoin(c.parts...); got != c.want {
			t.Fatalf("cpfwPyJoin(%q) = %q，want %q", c.parts, got, c.want)
		}
	}
}

// 定向单测：`cpfwPyFnmatch` 的 `*` **跨 `/`**（⟷ Go `path.Match` 的 `*` 不跨 `/`）。
func TestCpfwS_Fnmatch星号跨斜杠(t *testing.T) {
	cases := []struct {
		name, pat string
		want      bool
	}{
		{"publish/docs/design/记忆体系.md", "publish/docs/*", true}, // ★ `*` 跨 `/`
		{"publish/docs/README.md", "publish/docs/*", true},
		{"publish/SECURITY.md", "publish/docs/*", false},
		{"publish/docs/README.md", "publish/docs/README.md", true},
		{"publish/docs/README.zh-CN.md", "publish/docs/README.md", false},
	}
	for _, c := range cases {
		if got := cpfwPyFnmatch(c.name, c.pat); got != c.want {
			t.Fatalf("cpfwPyFnmatch(%q,%q) = %v，want %v", c.name, c.pat, got, c.want)
		}
	}
}
