package main

// check_zero_side_effect_test.go —— `check-zero-side-effect.py` Go 真搬的**成对负控**单测。
// ✗ 禁恒绿：每条正例都配一条「哑化即 FAIL」的负控（见 TestCheckZeroSideEffect_WriteAPITeeth /
// _WriteFlagRejected / _UnreadableFailClosed）。所有夹具走 `t.TempDir()`（仓外），不碰真目标。

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func czseT(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := checkZeroSideEffectGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckZeroSideEffect_SelfTestGreen(t *testing.T) {
	rc, out, errb := czseT(t, "--self-test")
	if rc != czseRCok {
		t.Fatalf("--self-test 该退 0，实得 %d（stderr=%q）", rc, errb)
	}
	if !strings.Contains(out, "自检结论: 全过") {
		t.Errorf("自检结论不为「全过」：\n%s", out)
	}
	if strings.Contains(out, "✗") {
		t.Errorf("自检里有失败断言：\n%s", out)
	}
}

func TestCheckZeroSideEffect_HelpVersionListRules(t *testing.T) {
	rc, out, _ := czseT(t, "-h")
	if rc != czseRCok || !strings.Contains(out, "usage: "+czseSelfName) || !strings.Contains(out, "optional arguments:") {
		t.Errorf("-h 面破：rc=%d out=%q", rc, out)
	}
	rc, out, _ = czseT(t, "--version")
	if rc != czseRCok || strings.TrimSpace(out) != czseVersion {
		t.Errorf("--version 面破：rc=%d out=%q", rc, out)
	}
	rc, out, _ = czseT(t, "--list-rules")
	if rc != czseRCok || !strings.Contains(out, "34 词") {
		t.Errorf("--list-rules 面破：rc=%d out=%q", rc, out)
	}
}

func TestCheckZeroSideEffect_WriteFlagRejected(t *testing.T) {
	for _, flag := range []string{"--chmod", "--seal", "commit", "--fix", "add"} {
		rc, out, _ := czseT(t, flag)
		if rc != czseRCnoconc {
			t.Errorf("写旗标 %q 该退 2，实得 %d", flag, rc)
		}
		if !strings.Contains(out, "写旗标") {
			t.Errorf("写旗标 %q 拒收文案缺：%q", flag, out)
		}
	}
	// 负控有牙：哑化 czseParseWriteFlags 的命中判定 ⇒ 上面必 FAIL。此处钉住判定本身。
	if got := czseParseWriteFlags([]string{"--sealed", "x", "--json"}); len(got) != 0 {
		t.Errorf("非写旗标被误判：%v", got)
	}
	if got := czseParseWriteFlags([]string{"--fix"}); len(got) != 1 || got[0] != "--fix" {
		t.Errorf("写旗标未被判出：%v", got)
	}
}

func TestCheckZeroSideEffect_NoWriteAPISelf(t *testing.T) {
	src, err := czseSelfSource("")
	if err != nil {
		t.Fatalf("读不到本件源码：%v", err)
	}
	hits := czseAssertNoWriteAPI(src, czseSelftestMarker)
	if len(hits) != 0 {
		t.Errorf("真目标段该 0 命中，实得 %d：%v", len(hits), hits)
	}
	if len(czseAssertNoWriteAPI(src, "")) == 0 {
		t.Errorf("整篇（含自检段）该有命中 ⇒ 扫描器恒空（负控失灵）")
	}
}

func TestCheckZeroSideEffect_WriteAPITeeth(t *testing.T) {
	teeth := "package main\n\nimport \"os\"\n\nfunc f(p string) error {\n\treturn os.Remove(p)\n}\n"
	if len(czseAssertNoWriteAPI(teeth, "")) == 0 {
		t.Errorf("合成源码含 os.Remove ⇒ 该命中（牙没了）")
	}
	writeF := "package main\n\nimport \"os\"\n\nfunc f(p string) error {\n\treturn os.WriteFile(p, nil, 0o644)\n}\n"
	if len(czseAssertNoWriteAPI(writeF, "")) == 0 {
		t.Errorf("合成源码含 os.WriteFile ⇒ 该命中（牙没了）")
	}
	benign := "package main\n\nfunc f(p string) string {\n\treturn p\n}\n"
	if len(czseAssertNoWriteAPI(benign, "")) != 0 {
		t.Errorf("无害源码被误判命中：%v", czseAssertNoWriteAPI(benign, ""))
	}
	bad := "package main\n\nfunc ( {"
	if len(czseAssertNoWriteAPI(bad, "")) == 0 {
		t.Errorf("解析不了该判「判不了 ⇒ 当红」")
	}
}

func TestCheckZeroSideEffect_PyPathPrimitives(t *testing.T) {
	cases := []struct{ got, want string }{
		{czsePyJoin("a", "../b"), "a/../b"},
		{czsePyJoin("repo", "/abs"), "/abs"},
		{czsePyJoin("a", "", "b"), "a/b"},
		{czsePyJoin("", "a"), "a"},
		{czsePyNormpath("//a/b/../c"), "//a/c"},
		{czsePyNormpath("///a/b"), "/a/b"},
		{czsePyNormpath("a/../b"), "b"},
		{czsePyDirname("/a/b/"), "/a/b"},
		{czsePyBasename("/a/b/"), ""},
		{czsePyBasename("/a/b"), "b"},
		{czsePyAbspath("/x/../y"), "/y"},
		{czsePyRelpath("/x/y/z", "/x"), "y/z"},
		{czsePyStrip("\x1c abc \x1d"), "abc"},
		{czsePyStrip("\u00a0x\u00a0"), "x"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("py 语义原语失配：got=%q want=%q", c.got, c.want)
		}
	}
}

func TestCheckZeroSideEffect_GitBlobProofAndDirty(t *testing.T) {
	tmp := t.TempDir()
	repo, err := czseMkGitRepo(tmp)
	if err != nil {
		t.Fatalf("造夹具失败：%v", err)
	}
	rc, out, _ := czseT(t, "--sealed", "clean.txt", "--root", repo)
	if rc != czseRCok {
		t.Fatalf("已封件该出证明 rc=0，实得 %d：\n%s", rc, out)
	}
	if !strings.Contains(out, "已封（HEAD blob）") {
		t.Errorf("基准来源该是 HEAD blob：\n%s", out)
	}
	rc, out, _ = czseT(t, "--sealed", "dirty.txt", "--root", repo)
	if rc != czseRCnobase {
		t.Fatalf("可变件该 rc=8，实得 %d：\n%s", rc, out)
	}
	if !strings.Contains(out, "无可比基准") {
		t.Errorf("可变件文案缺「无可比基准」：\n%s", out)
	}
}

func TestCheckZeroSideEffect_SealedMissingRc2(t *testing.T) {
	tmp := t.TempDir()
	rc, out, _ := czseT(t, "--sealed", czsePyJoin(tmp, "nope"), "--root", tmp)
	if rc != czseRCnoconc {
		t.Fatalf("缺件该 rc=2，实得 %d", rc)
	}
	if !strings.Contains(out, "不给结论") {
		t.Errorf("缺件文案缺「不给结论」：%q", out)
	}
}

func TestCheckZeroSideEffect_MissingArchiveRc2(t *testing.T) {
	tmp := t.TempDir()
	rc, out, _ := czseT(t, "--archive", czsePyJoin(tmp, "nope-archive"), "--root", tmp)
	if rc != czseRCnoconc || !strings.Contains(out, "归档区不存在") {
		t.Fatalf("缺归档区该 rc=2＋「归档区不存在」，实得 rc=%d out=%q", rc, out)
	}
}

func TestCheckZeroSideEffect_JSONNullFace(t *testing.T) {
	tmp := t.TempDir()
	rc, out, _ := czseT(t, "--json", "--sealed", czsePyJoin(tmp, "gone"), "--root", tmp)
	if rc != czseRCnoconc {
		t.Fatalf("该 rc=2，实得 %d", rc)
	}
	for _, want := range []string{`"actual": null`, `"class": "缺件"`, `"match": null`,
		`"warnings": []`, `"truncated": false`, `"missing": 1`} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON 空值/空集三形态缺 %s：\n%s", want, out)
		}
	}
}

func TestCheckZeroSideEffect_UnreadableFailClosed(t *testing.T) {
	tmp := t.TempDir()
	p := czsePyJoin(tmp, "locked.md")
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	rc, out, _ := czseT(t, "--sealed", p, "--root", tmp)
	if rc != czseRCnoconc {
		t.Fatalf("读不了的件该 fail-closed rc=2，实得 %d：\n%s", rc, out)
	}
	if !strings.Contains(out, "读不了") {
		t.Errorf("读不了文案缺：\n%s", out)
	}
}

func TestCheckZeroSideEffect_NoTempdirLeak(t *testing.T) {
	count := func() int {
		ents, err := os.ReadDir(os.TempDir())
		if err != nil {
			return -1
		}
		n := 0
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), "zse-selftest-") {
				n++
			}
		}
		return n
	}
	before := count()
	for i := 0; i < 2; i++ {
		if rc, _, _ := czseT(t, "--self-test"); rc != czseRCok {
			t.Fatalf("自检该退 0")
		}
	}
	after := count()
	if after != before {
		t.Errorf("tempdir 泄漏：before=%d after=%d（口径差③止血失效）", before, after)
	}
}
