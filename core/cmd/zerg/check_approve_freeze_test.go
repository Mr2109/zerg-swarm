package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cafrEnv —— 「假仓 + 假文档仓」夹具（与 py 自检同形 · 全真目录 · ✗ 无符号链接）。
type cafrEnv struct {
	repo, docs string
}

func cafrFieldLine() string { return strings.Join(cafrExpectedFields, ",") }

func cafrSetup(t *testing.T) cafrEnv {
	t.Helper()
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	// ★ GAP-20260927-88：夹具用中性版本目录名（任意值 —— 本套用例一律显式传 `--docs-root`，
	// 不走版本真源推导那一支）。
	docs := filepath.Join(tmp, "Zerg-内部文档", "项目文档", "v9.9.9")
	for _, d := range []string{
		filepath.Join(repo, "core", "cmd", "zerg", "testdata"),
		docs,
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fields := cafrFieldLine()
	cafrWrite(t, cafrGoPath(repo), "package main\n\nvar approveFields = []string{"+cafrQuoted(fields)+"}\n")
	cafrWrite(t, cafrMxPath(repo), `{"cases":[{"id":"A-1","command":"approve ls","want_rc":2},{"id":"A-2","command":"approve show","want_rc":2}]}`)
	cafrWrite(t, cafrHelpPath(docs), "| `zerg approve ls` | x | "+fields+" | 本机 | `host` |\n"+
		"| `zerg approve show` | y | "+fields+" | 本机 | `host` |\n")
	cafrWrite(t, cafrGapPath(docs), "回指 R-11 · G-01\nG-16\n")
	return cafrEnv{repo: repo, docs: docs}
}

func cafrGoPath(repo string) string {
	return filepath.Join(repo, "core", "cmd", "zerg", "family_approve.go")
}
func cafrMxPath(repo string) string {
	return filepath.Join(repo, "core", "cmd", "zerg", "testdata", "cli-matrix.json")
}
func cafrHelpPath(d string) string { return filepath.Join(d, "导出-命令面帮助-20990101.md") }
func cafrGapPath(d string) string  { return filepath.Join(d, "缺口-命令面-20260922.md") }

func cafrQuoted(fields string) string {
	parts := strings.Split(fields, ",")
	for i, p := range parts {
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, ", ")
}

func cafrRun(argv ...string) (int, string, string) {
	var out, errb strings.Builder
	rc := checkApproveFreezeGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func cafrWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCheckApproveFreezePyJoin —— ★ 三类语义差定向单测（空段 / 后段绝对 / 不归一 `..`）。
func TestCheckApproveFreezePyJoin(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"a", ""}, "a/"},            // 空段：py 补分隔符（filepath.Join 得 "a"）
		{[]string{"", "a"}, "a"},             // py join('','a')=="a"
		{[]string{"a", "", "b"}, "a/b"},      // 中间空段
		{[]string{"a", "/b"}, "/b"},          // 后段绝对丢前段（filepath.Join 得 "a/b"）
		{[]string{"a", "..", "b"}, "a/../b"}, // 不归一 `..`（filepath.Join 得 "b"）
		{[]string{"core", "cmd", "zerg", "family_approve.go"}, "core/cmd/zerg/family_approve.go"},
		{[]string{"", ""}, ""},
	}
	for _, c := range cases {
		if got := cafrPyJoin(c.in...); got != c.want {
			t.Errorf("pyJoin(%q) = %q · 要 %q", c.in, got, c.want)
		}
	}
}

// TestCheckApproveFreezeGatePort —— 入口正控 + 成对负控（带牙：哑化任一判据 ⇒ 必 FAIL ＋行号）。
func TestCheckApproveFreezeGatePort(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(e cafrEnv)
		argv   []string
		wantRC int
		word   string
		errW   bool
	}{
		{"正控 全绿", func(e cafrEnv) {}, nil, 0, "全绿", false},
		{"负控 F1 帮助面字段表坏", func(e cafrEnv) {
			cafrWrite(t, cafrHelpPath(e.docs), "| `zerg approve ls` | x | 少一格,oo | 本机 | h |\n| `zerg approve show` | y | "+cafrFieldLine()+" | 本机 | h |\n")
		}, []string{"--no-self-test"}, 1, "F1 漂移", false},
		{"负控 F2 approveFields 坏", func(e cafrEnv) {
			cafrWrite(t, cafrGoPath(e.repo), "package main\n\nvar approveFields = []string{\"tool\",\"approver\"}\n")
		}, []string{"--no-self-test"}, 1, "F2 漂移", false},
		{"负控 F3 矩阵漏 must-fail", func(e cafrEnv) {
			cafrWrite(t, cafrMxPath(e.repo), `{"cases":[{"id":"A-1","command":"approve ls","want_rc":0}]}`)
		}, []string{"--no-self-test"}, 1, "F3 漂移", false},
		{"负控 F4 台账缺回指", func(e cafrEnv) {
			cafrWrite(t, cafrGapPath(e.docs), "没有回指的台账 G-01\n")
		}, []string{"--no-self-test"}, 1, "F4 漂移", false},
		{"负控 F4b 台账重开新编号", func(e cafrEnv) {
			cafrWrite(t, cafrGapPath(e.docs), "回指 R-11 · 另开 G-17\n")
		}, []string{"--no-self-test"}, 1, "新编号", false},
		{"正控 --self-test 只跑自检", func(e cafrEnv) {}, []string{"--self-test"}, 0, "自检：6 格过", false},
		{"负控 --json 无机器读面", func(e cafrEnv) {}, []string{"--json"}, 2, "无机器读面", true},
		{"负控 根不在", func(e cafrEnv) {}, []string{"--root", "no-such-root"}, 2, "不给结论", false},
		{"负控 docs 根缺", func(e cafrEnv) {}, []string{"--docs-root", "no-docs", "--no-self-test"}, 2, "Zerg-内部文档 根不在", false},
		{"真口子钉 cases=null ⇒ fail-closed rc=2", func(e cafrEnv) {
			cafrWrite(t, cafrMxPath(e.repo), `{"cases":null}`)
		}, []string{"--no-self-test"}, 2, "null", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := cafrSetup(t)
			c.setup(e)
			base := []string{"--root", e.repo, "--docs-root", e.docs}
			rc, out, errb := cafrRun(append(base, c.argv...)...)
			if rc != c.wantRC {
				t.Fatalf("rc=%d 要 %d\n out=%s\n err=%s", rc, c.wantRC, out, errb)
			}
			hay := out
			if c.errW {
				hay = errb
			}
			if !strings.Contains(hay, c.word) {
				t.Fatalf("缺关键字 %q\n out=%s\n err=%s", c.word, out, errb)
			}
		})
	}
}
