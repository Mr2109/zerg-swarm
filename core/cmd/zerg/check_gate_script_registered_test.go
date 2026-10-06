// check_gate_script_registered_test.go —— `check_gate_script_registered.go` 的**成对负控**（✗ 禁恒绿）。
//
// 覆盖面（每条都有正/负两向）：
//
//	· 三态退码：全登记 rc=0 · 有未登记 rc=1 · 取不到输入 rc=2（目录不在盘 / 空域 / 标记表读不到）
//	· `--json` 空集形态（`"unregistered": []`，✗ 不落 `null`）
//	· argparse 面：未知旗标 / 缺值 / 唯一前缀缩写 / 帮助面标签 `-h/--help`
//	· ★ 遍历序（readdir 原序）—— 明细行序必须 == 原序，✗ 不是字典序
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gsrtInvoke —— 跑本入口，收三面。
func gsrtInvoke(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkGateScriptRegisteredGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// gsrtMake —— 造合成假仓根：gateRel 为 nil ⇒ **不建** `scripts/gates` 目录；marksRel 为 nil ⇒ 不写标记表。
func gsrtMake(t *testing.T, gateNames []string, marksLines []string, createGatesDir bool, createMarks bool) string {
	t.Helper()
	root := t.TempDir()
	if createGatesDir {
		if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, n := range gateNames {
			if err := os.WriteFile(filepath.Join(root, "scripts", "gates", n), []byte("# x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if createMarks {
		if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
			t.Fatal(err)
		}
		body := ""
		if len(marksLines) > 0 {
			body = strings.Join(marksLines, "\n") + "\n"
		}
		if err := os.WriteFile(filepath.Join(root, "scripts", "公开标记.tsv"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// gsrtRawOrder —— 独立取「readdir 原序」下的门件相对路径（✗ 不排序）。
func gsrtRawOrder(t *testing.T, root string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(root, "scripts", "gates"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	ents, err := f.ReadDir(-1)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		n := e.Name()
		if strings.HasSuffix(n, ".py") || strings.HasSuffix(n, ".sh") {
			out = append(out, "scripts/gates/"+n)
		}
	}
	return out
}

func TestGsrtFixtureMatrix(t *testing.T) {
	t.Run("全登记_正控_rc0", func(t *testing.T) {
		root := gsrtMake(t, []string{"a.py", "b.sh"}, []string{"scripts/gates/a.py\tno\tB\nscripts/gates/b.sh\tno\tB"}, true, true)
		rc, out, errs := gsrtInvoke("--root", root)
		if rc != 0 {
			t.Fatalf("rc=%d want 0; out=%q err=%q", rc, out, errs)
		}
		if !strings.Contains(out, "未登记 0 件") {
			t.Fatalf("out=%q", out)
		}
	})
	t.Run("有未登记_负控_rc1", func(t *testing.T) {
		root := gsrtMake(t, []string{"a.py", "c.sh"}, []string{"scripts/gates/a.py\tno\tB"}, true, true)
		rc, out, _ := gsrtInvoke("--root", root)
		if rc != 1 {
			t.Fatalf("rc=%d want 1; out=%q", rc, out)
		}
		if !strings.Contains(out, "未登记 1 件") || !strings.Contains(out, "✗ 未登记：scripts/gates/c.sh") {
			t.Fatalf("out=%q", out)
		}
	})
	t.Run("空域_负控_rc2", func(t *testing.T) {
		root := gsrtMake(t, nil, []string{"scripts/gates/a.py\tno\tB"}, true, true)
		rc, _, errs := gsrtInvoke("--root", root)
		if rc != 2 || !strings.Contains(errs, "空域") {
			t.Fatalf("rc=%d err=%q", rc, errs)
		}
	})
	t.Run("目录不在盘_负控_rc2", func(t *testing.T) {
		root := gsrtMake(t, nil, []string{"x"}, false, true)
		rc, _, errs := gsrtInvoke("--root", root)
		if rc != 2 || !strings.Contains(errs, "门件目录不在盘") {
			t.Fatalf("rc=%d err=%q", rc, errs)
		}
	})
	t.Run("标记表读不到_负控_rc2", func(t *testing.T) {
		root := gsrtMake(t, []string{"a.py"}, nil, true, false)
		rc, _, errs := gsrtInvoke("--root", root)
		if rc != 2 || !strings.Contains(errs, "标记表读不到") {
			t.Fatalf("rc=%d err=%q", rc, errs)
		}
	})
	t.Run("json_空集形态_非null", func(t *testing.T) {
		root := gsrtMake(t, []string{"a.py"}, []string{"scripts/gates/a.py\tno\tB"}, true, true)
		rc, out, _ := gsrtInvoke("--root", root, "--json")
		want := "{\"schema\": \"zerg/v1\", \"kind\": \"GateScriptRegistered\", \"gate_scripts\": 1, \"unregistered\": []}\n"
		if rc != 0 || out != want {
			t.Fatalf("rc=%d out=%q want=%q", rc, out, want)
		}
	})
	t.Run("json_非空列表", func(t *testing.T) {
		root := gsrtMake(t, []string{"a.py"}, nil, true, true)
		rc, out, _ := gsrtInvoke("--root", root, "--json")
		if rc != 1 || !strings.Contains(out, `"unregistered": ["scripts/gates/a.py"]`) {
			t.Fatalf("rc=%d out=%q", rc, out)
		}
	})
}

func TestGsrtSelfTest(t *testing.T) {
	rc, out, _ := gsrtInvoke("--self-test")
	if rc != 0 {
		t.Fatalf("rc=%d want 0; out=%q", rc, out)
	}
	if !strings.Contains(out, "自检 PASS（4 格）") || !strings.Contains(out, "未登记 1 件") {
		t.Fatalf("out=%q", out)
	}
}

func TestGsrtArgparse(t *testing.T) {
	root := gsrtMake(t, []string{"a.py"}, []string{"scripts/gates/a.py\tno\tB"}, true, true)
	t.Run("未知旗标_负控", func(t *testing.T) {
		rc, _, errs := gsrtInvoke("--root", root, "--bogus")
		if rc != 2 || !strings.Contains(errs, "unrecognized arguments: --bogus") {
			t.Fatalf("rc=%d err=%q", rc, errs)
		}
	})
	t.Run("缺值_负控", func(t *testing.T) {
		rc, _, errs := gsrtInvoke("--root")
		if rc != 2 || !strings.Contains(errs, "argument --root: expected one argument") {
			t.Fatalf("rc=%d err=%q", rc, errs)
		}
	})
	t.Run("帮助面_正控", func(t *testing.T) {
		rc, out, _ := gsrtInvoke("-h")
		if rc != 0 || !strings.HasPrefix(out, "usage: check-gate-script-registered.py") || !strings.Contains(out, "optional arguments:") {
			t.Fatalf("rc=%d out=%q", rc, out)
		}
	})
	t.Run("唯一前缀缩写_正控", func(t *testing.T) {
		rc, out, _ := gsrtInvoke("--ro", root)
		if rc != 0 || !strings.Contains(out, "未登记 0 件") {
			t.Fatalf("rc=%d out=%q", rc, out)
		}
	})
	t.Run("帮助旗标显式参_负控_标签为-h/--help", func(t *testing.T) {
		rc, _, errs := gsrtInvoke("--help=1")
		if rc != 2 || !strings.Contains(errs, "argument -h/--help: ignored explicit argument '1'") {
			t.Fatalf("rc=%d err=%q", rc, errs)
		}
	})
}

// TestGsrtReaddirOrder —— ★ 遍历序牙：明细行序必须 == readdir 原序（✗ 不是字典序）。
// 哑化（把 `f.ReadDir(-1)` 换成会排序的读法）⇒ 本用例必红。
func TestGsrtReaddirOrder(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 按非字典序的创建顺序落件（zz, bb, mm, aa）—— APFS readdir 原序 ≠ sorted。
	for _, n := range []string{"zz.sh", "bb.sh", "mm.py", "aa.py"} {
		if err := os.WriteFile(filepath.Join(root, "scripts", "gates", n), []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "公开标记.tsv"),
		[]byte("scripts/gates/aa.py\tno\tB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, out, errs := gsrtInvoke("--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d want 1; out=%q err=%q", rc, out, errs)
	}
	raw := gsrtRawOrder(t, root)
	sorted := append([]string(nil), raw...)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	// 牙的前提：原序 ≠ 字典序（否则本用例测不出排序读法）。
	if strings.Join(raw, ",") == strings.Join(sorted, ",") {
		t.Fatalf("夹具前提失效：原序 == 字典序（%v）⇒ 换名重造", raw)
	}
	var want []string
	for _, g := range raw {
		if g != "scripts/gates/aa.py" {
			want = append(want, g)
		}
	}
	var got []string
	for _, ln := range strings.Split(out, "\n") {
		if strings.HasPrefix(ln, "  ✗ 未登记：") {
			got = append(got, strings.TrimPrefix(ln, "  ✗ 未登记："))
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("明细行序非 readdir 原序：got=%v want=%v", got, want)
	}
}

// TestGsrtPyStrip —— `gsrPyStrip` = py `str.strip()`（含 `\x1c-\x1f` / `\x85` / `\xa0`）⟷ `strings.TrimSpace` 不剥 `\x1c`。
func TestGsrtPyStrip(t *testing.T) {
	if got := gsrPyStrip("\u001c\u00a0abc\u0085\u001f"); got != "abc" {
		t.Fatalf("gsrPyStrip=%q want %q", got, "abc")
	}
	if strings.TrimSpace("\u001cabc") == "abc" {
		t.Fatalf("前提失效：strings.TrimSpace 竟剥 U+001C")
	}
}

// TestGsrtUniversalNewlines —— `gsrTextLines` 折 `\r\n` / 裸 `\r`（py 文本模式语义）。
func TestGsrtUniversalNewlines(t *testing.T) {
	got := strings.Join(gsrTextLines("a\r\nb\rc"), "|")
	if got != "a|b|c" {
		t.Fatalf("gsrTextLines=%q", got)
	}
}

// TestGsrtPyJoin —— posixpath.join 三态（后段绝对丢弃前段 / 空段 / 尾斜杠）。
func TestGsrtPyJoin(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"repo", "scripts/gates", "repo/scripts/gates"},
		{"", "scripts/gates", "scripts/gates"},
		{"/", "scripts/gates", "/scripts/gates"},
		{"repo", "/abs", "/abs"},
		{"a", "", "a/"},
	}
	for _, c := range cases {
		if got := gsrPyJoin(c.a, c.b); got != c.want {
			t.Fatalf("gsrPyJoin(%q,%q)=%q want %q", c.a, c.b, got, c.want)
		}
	}
}
