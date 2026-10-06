package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cscWriteFixture —— 测试夹具写件（★ 一律落 t.TempDir() 派生路径，✗ 不碰真仓）。
func cscWriteFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建夹具目录失败：%v", err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具件失败：%v", err)
	}
	return p
}

// TestCscSelfTestAllPass —— 十档自检必须全过（哑化 steps 的 `self` 排除即可令 档④ 红）。
func TestCscSelfTestAllPass(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	var out strings.Builder
	rc := cscSelfTest(&out)
	if rc != 0 {
		t.Fatalf("自检应全过（rc=0），实得 rc=%d\n%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "十档") {
		t.Fatalf("自检题头缺失：%q", out.String())
	}
	if !strings.Contains(out.String(), "全过 ✓") {
		t.Fatalf("自检结论应为「全过 ✓」：%q", out.String())
	}
	if strings.Contains(out.String(), "✗") {
		t.Fatalf("自检不应出现 ✗ 档：%q", out.String())
	}
}

// TestCscRunFixtures —— 各 kind / 各档逐格钉 rc 与正文（哑化任一判据即红）。
func TestCscRunFixtures(t *testing.T) {
	tmp := t.TempDir()
	dOK := filepath.Join(tmp, "ok")
	cscWriteFixture(t, dOK, "steps.sh", "# {{selfcount:steps scope=gates}} = 2\nadd_step gates \"甲\"\nadd_step gates \"乙\"\n")
	dBad := filepath.Join(tmp, "bad")
	cscWriteFixture(t, dBad, "steps.sh", "# {{selfcount:steps scope=gates}} = 3\nadd_step gates \"甲\"\nadd_step gates \"乙\"\n")
	dNoAnn := filepath.Join(tmp, "noann")
	cscWriteFixture(t, dNoAnn, "steps.sh", "add_step gates \"甲\"\nadd_step gates \"乙\"\n")
	dExcl := filepath.Join(tmp, "excl")
	cscWriteFixture(t, dExcl, "steps.sh", "# {{selfcount:steps}} = 1\nadd_step self \"自检-必过-1\"\nadd_step go \"core: build\"\n")
	dChecks := filepath.Join(tmp, "checksok")
	cscWriteFixture(t, dChecks, "a.py", "# {{selfcount:checks}} = 2\nassert_eq(\"a\", 1, 1)\nassert_eq(\"b\", 2, 2)\n")
	dFiles := filepath.Join(tmp, "filesok")
	cscWriteFixture(t, dFiles, "me.py", "# {{selfcount:files glob=*.py}} = 2\n")
	cscWriteFixture(t, dFiles, "peer.py", "# peer\n")
	dKindBad := filepath.Join(tmp, "kindbad")
	cscWriteFixture(t, dKindBad, "a.sh", "# {{selfcount:bogus}} = 1\n")
	dNoNum := filepath.Join(tmp, "nonum")
	cscWriteFixture(t, dNoNum, "a.sh", "# {{selfcount:steps scope=gates}}\nadd_step gates \"甲\"\n")
	dEmpty := filepath.Join(tmp, "empty")
	if err := os.MkdirAll(dEmpty, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		label   string
		root    string
		wantRC  int
		wantOut string // 必须包含
		wantErr string // 必须包含（空串 = 不检查）
	}{
		{"档① 正控 steps", dOK, 0, "✓ steps.sh:1 · kind=steps scope=gates · 声明=2 · 现读=2", ""},
		{"档② 负控 steps", dBad, 1, "✗ steps.sh:1 · kind=steps scope=gates · 声明=3 · 现读=2", ""},
		{"档③ 无标注 ⇒ 不判", dNoAnn, 0, "⇒ 无 `{{selfcount:…}}` 标注 ⇒ **不判**（rc=0）", ""},
		{"档④ 自检步排除（self 不计）", dExcl, 0, "✓ steps.sh:1 · kind=steps · 声明=1 · 现读=1", ""},
		{"档⑥ checks 正控", dChecks, 0, "✓ a.py:1 · kind=checks · 声明=2 · 现读=2", ""},
		{"档⑧ files 正控（隐藏不计）", dFiles, 0, "✓ me.py:1 · kind=files · 声明=2 · 现读=2", ""},
		{"kind 未知 ⇒ rc=2 点名", dKindBad, 2, "⇒ 缺语料 / 标注不全 ⇒ rc=2（不给结论）", "kind 未知：'bogus'（合法 steps/files/checks）"},
		{"标注缺声明数 ⇒ rc=2", dNoNum, 2, "⇒ 缺语料 / 标注不全 ⇒ rc=2（不给结论）", "标注缺声明数 N（须 `= <N>` 或行尾给一个整数）"},
		{"空域 ⇒ rc=2", dEmpty, 2, "", "BLOCKED —— 空域："},
		{"不存在根 ⇒ rc=2", filepath.Join(tmp, "zzz"), 2, "", "BLOCKED —— 扫描根不存在或读不到："},
	}
	for _, c := range cases {
		var o, e strings.Builder
		rc := checkSelfcountGateMain(&o, &e, []string{"--root", c.root})
		if rc != c.wantRC {
			t.Fatalf("%s：rc 期望 %d 实得 %d\nstdout=%q\nstderr=%q", c.label, c.wantRC, rc, o.String(), e.String())
		}
		if c.wantOut != "" && !strings.Contains(o.String(), c.wantOut) {
			t.Fatalf("%s：stdout 缺 %q\n实得=%q", c.label, c.wantOut, o.String())
		}
		if c.wantErr != "" && !strings.Contains(e.String(), c.wantErr) {
			t.Fatalf("%s：stderr 缺 %q\n实得=%q", c.label, c.wantErr, e.String())
		}
	}
}

// TestCscNegativeControlSelfExclusion —— 自排除：扫描根内的 check-selfcount.py 副本不计。
// 哑化 cscRunCheck 的 selfPath 判定（去掉 selfPath 比较）即令本用例红。
func TestCscNegativeControlSelfExclusion(t *testing.T) {
	base := t.TempDir()
	gates := filepath.Join(base, "scripts", "gates")
	// 同名件：含真标注（若不被自排除 ⇒ 现读 0 ≠ 声明 1 ⇒ 红）
	cscWriteFixture(t, gates, "check-selfcount.py", "# {{selfcount:checks}} = 1\nassert_eq(1,1,1)\n")
	cscWriteFixture(t, gates, "other.py", "# {{selfcount:checks}} = 1\nassert_eq(1,1,1)\n")

	var o, e strings.Builder
	rc := cscRunCheck(gates, false, base, &o, &e)
	if rc != 0 {
		t.Fatalf("自排除后应只剩 other.py 一条正控（rc=0），实得 rc=%d\n%s%s", rc, o.String(), e.String())
	}
	if !strings.Contains(o.String(), "标注 1 条 · 已判 1 条") {
		t.Fatalf("应只计 other.py（标注 1 条），实得：%q", o.String())
	}
	if strings.Contains(o.String(), "check-selfcount.py:") {
		t.Fatalf("check-selfcount.py 应被自排除，不应出现在正控明细里：%q", o.String())
	}
}

// TestCscJSONExactOK —— --json 逐字节（键序 / null / 布尔 / 分隔符）。
func TestCscJSONExactOK(t *testing.T) {
	tmp := t.TempDir()
	d := filepath.Join(tmp, "ok")
	cscWriteFixture(t, d, "steps.sh", "# {{selfcount:steps scope=gates}} = 2\nadd_step gates \"甲\"\nadd_step gates \"乙\"\n")
	var o, e strings.Builder
	rc := checkSelfcountGateMain(&o, &e, []string{"--root", d, "--json"})
	if rc != 0 {
		t.Fatalf("rc 期望 0 实得 %d err=%q", rc, e.String())
	}
	want := "{\"schema\": \"zerg/v1\", \"kind\": \"SelfCount\", \"root\": \"" + d +
		"\", \"annotations\": 1, \"checked\": [{\"file\": \"steps.sh\", \"line\": 1, \"kind\": \"steps\", " +
		"\"scope\": \"gates\", \"glob\": null, \"declared\": 2, \"live\": 2, \"ok\": true}], " +
		"\"mismatch\": [], \"blocked\": [], \"bad_annotation\": []}\n"
	if o.String() != want {
		t.Fatalf("--json 不符：\n got=%q\nwant=%q", o.String(), want)
	}
}

// TestCscArgsHelp —— `-h` 逐字节（钉 py 3.9.6 措辞）。
func TestCscArgsHelp(t *testing.T) {
	var o, e strings.Builder
	rc := checkSelfcountGateMain(&o, &e, []string{"-h"})
	if rc != 0 {
		t.Fatalf("-h rc 期望 0 实得 %d", rc)
	}
	want := "usage: check-selfcount.py [-h] [--root ROOT] [--json]\n\n" +
		"optional arguments:\n" +
		"  -h, --help   show this help message and exit\n" +
		"  --root ROOT\n" +
		"  --json\n"
	if o.String() != want {
		t.Fatalf("-h 不符：\n got=%q\nwant=%q", o.String(), want)
	}
	if e.String() != "" {
		t.Fatalf("-h stderr 应为空：%q", e.String())
	}
}

// TestCscArgsErrors —— argparse 等价报错档（未知旗标 / 缺值 / `--` / store_true 显式值）。
func TestCscArgsErrors(t *testing.T) {
	cases := []struct {
		label   string
		argv    []string
		wantErr string
	}{
		{"未知旗标", []string{"--nope"}, "check-selfcount.py: error: unrecognized arguments: --nope\n"},
		{"缺值", []string{"--root"}, "check-selfcount.py: error: argument --root: expected one argument\n"},
		{"-- 孤零零", []string{"--"}, "check-selfcount.py: error: unrecognized arguments: --\n"},
		{"-- 后有位参", []string{"--", "foo"}, "check-selfcount.py: error: unrecognized arguments: -- foo\n"},
		{"store_true 显式值", []string{"--json=1"}, "check-selfcount.py: error: argument --json: ignored explicit argument '1'\n"},
	}
	for _, c := range cases {
		var o, e strings.Builder
		rc := checkSelfcountGateMain(&o, &e, c.argv)
		if rc != 2 {
			t.Fatalf("%s：rc 期望 2 实得 %d", c.label, rc)
		}
		if !strings.HasPrefix(e.String(), cscUsage) {
			t.Fatalf("%s：stderr 应以 usage 起头：%q", c.label, e.String())
		}
		if !strings.HasSuffix(e.String(), c.wantErr) {
			t.Fatalf("%s：stderr 尾不符\n got=%q\nwant 尾=%q", c.label, e.String(), c.wantErr)
		}
	}
}

// TestCscSelfTestPrecedence —— `--self-test`/`--selftest` 在 argparse 之前直接走自检。
func TestCscSelfTestPrecedence(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	for _, flag := range []string{"--self-test", "--selftest"} {
		var o, e strings.Builder
		rc := checkSelfcountGateMain(&o, &e, []string{flag})
		if rc != 0 {
			t.Fatalf("%s：rc 期望 0 实得 %d", flag, rc)
		}
		if !strings.Contains(o.String(), "十档") {
			t.Fatalf("%s：应走自检：%q", flag, o.String())
		}
	}
}
