// m2_go_py_isomorphism_scan_test.go —— 与 `m2_go_py_isomorphism_scan.go` 成对
// （M2 · 抢池底粒 13）。
//
// 定位：**成对负控**（✗ 禁恒绿）—— 每一格都钉住 py 的一条可观测行为，实现回退即红。
// 用例前缀 `TestM2GoPyIsoScan`（现读 `zerg code find 'm2s'` / `m2GoPy` 零命中 ⇒ 不与在仓测试件撞名）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// m2sRun —— 进程内跑入口，收 rc/stdout/stderr 三面。
func m2sRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := m2GoPyIsomorphismScanGateMain(&out, &errb, args)
	return rc, out.String(), errb.String()
}

// 夹具件内容与对拍台 fxB 逐字同（保证「已落仓件重建」与对拍基准一致）。
const m2sFxAGo = "package main\nimport \"path/filepath\"\n" +
	"func f(root string) { _ = filepath.Join(root, \"a\"); _ = os.ReadDir(root); " +
	"_ = PyJoin(\"a\",\"b\"); re := \"\\\\s+\"; _ = re }\n"

const m2sFxCGo = "package main\n" +
	"func g() { _ = filepath.Abs(\"x\"); _ = filepath.Rel(\"a\",\"b\"); _ = filepath.EvalSymlinks(\"z\"); _ = filepath.WalkDir(\"q\", nil) }\n"

// m2sBuildFx 造 `<root>/core/cmd/zerg/{a.go?,c.go?,b_test.go}`（a/c 可关）。
func m2sBuildFx(t *testing.T, a, c bool) string {
	t.Helper()
	root := t.TempDir()
	d := filepath.Join(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if a {
		if err := os.WriteFile(filepath.Join(d, "a.go"), []byte(m2sFxAGo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if c {
		if err := os.WriteFile(filepath.Join(d, "c.go"), []byte(m2sFxCGo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(d, "b_test.go"), []byte("package main\nvar _ = filepath.Join\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestM2GoPyIsoScanHelpAndArgparse —— argparse 面：帮助 / 未知旗标 / 缺值。
func TestM2GoPyIsoScanHelpAndArgparse(t *testing.T) {
	rc, out, errb := m2sRun(t, "-h")
	if rc != 0 || errb != "" || out != m2sHelpText {
		t.Fatalf("help rc=%d out=%q err=%q", rc, out, errb)
	}
	if !strings.HasPrefix(out, "usage: m2-go-py-isomorphism-scan.py [-h] [--root ROOT] [--json] [--self-test]\n") ||
		!strings.Contains(out, "optional arguments:") || !strings.Contains(out, "  --self-test\n") {
		t.Fatalf("help 正文与 py 3.9 不符: %q", out)
	}

	rc, _, errb = m2sRun(t, "--bogus")
	if rc != 2 || errb != m2sUsage+"m2-go-py-isomorphism-scan.py: error: unrecognized arguments: --bogus\n" {
		t.Fatalf("bogus rc=%d err=%q", rc, errb)
	}

	rc, _, errb = m2sRun(t, "--root")
	if rc != 2 || !strings.Contains(errb, "m2-go-py-isomorphism-scan.py: error: argument --root: expected one argument\n") {
		t.Fatalf("noval rc=%d err=%q", rc, errb)
	}
}

// TestM2GoPyIsoScanTextExact —— 文本档：表头/行序（按 raw_path_ops 降序）/合计/说明 逐字节。
func TestM2GoPyIsoScanTextExact(t *testing.T) {
	root := m2sBuildFx(t, true, true)
	rc, out, errb := m2sRun(t, "--root", root)
	if rc != 0 || errb != "" {
		t.Fatalf("text rc=%d err=%q", rc, errb)
	}
	want := "file\tjoin\tabs\trel\tevalsymlinks\treaddir\twalkdir\tdirentry_isdir\tregex_unexpanded\tpyhelper\traw_path_ops\n" +
		"c.go\t0\t1\t1\t1\t0\t1\t0\t0\t0\t3\n" +
		"a.go\t1\t0\t0\t0\t1\t0\t0\t1\t1\t1\n" +
		"# 合计：" + `{"join": 1, "abs": 1, "rel": 1, "evalsymlinks": 1, "readdir": 1, "walkdir": 1, "direntry_isdir": 0, "regex_unexpanded": 1, "pyhelper": 1, "raw_path_ops": 4}` + "\n" +
		"# 说明：raw_path_ops = join+abs+rel+evalsymlinks（裸算子用量，新口径应改自写 Py* 一族）；\n" +
		"#       「哪些必须重落」归收口批拍板 —— 本件只出可复现清单。\n"
	if out != want {
		t.Fatalf("text 正文不符:\n got=%q\nwant=%q", out, want)
	}
	// 排序牙：raw_path_ops 大的先（若排序键回退成按名 ⇒ c.go/a.go 序反转 ⇒ 上面逐字已红）。
	if strings.Index(out, "c.go\t") > strings.Index(out, "a.go\t") {
		t.Fatalf("排序键回落：c.go 未先于 a.go\n%s", out)
	}
}

// TestM2GoPyIsoScanJSONExact —— `--json` 档：Python `json.dumps` 默认分隔符与 dict 序 逐字节。
func TestM2GoPyIsoScanJSONExact(t *testing.T) {
	root := m2sBuildFx(t, true, true)
	rc, out, errb := m2sRun(t, "--root", root, "--json")
	if rc != 0 || errb != "" {
		t.Fatalf("json rc=%d err=%q", rc, errb)
	}
	want := `{"schema": "zerg/m2-go-py-isomorphism/v1", "rows": [{"file": "c.go", "join": 0, "abs": 1, "rel": 1, "evalsymlinks": 1, "readdir": 0, "walkdir": 1, "direntry_isdir": 0, "regex_unexpanded": 0, "pyhelper": 0, "raw_path_ops": 3}, {"file": "a.go", "join": 1, "abs": 0, "rel": 0, "evalsymlinks": 0, "readdir": 1, "walkdir": 0, "direntry_isdir": 0, "regex_unexpanded": 1, "pyhelper": 1, "raw_path_ops": 1}]}` + "\n"
	if out != want {
		t.Fatalf("json 正文不符:\n got=%q\nwant=%q", out, want)
	}
}

// TestM2GoPyIsoScanSelfTest —— `--self-test`：5/5 合成夹具，rc=0。
func TestM2GoPyIsoScanSelfTest(t *testing.T) {
	rc, out, errb := m2sRun(t, "--self-test")
	if rc != 0 || errb != "" {
		t.Fatalf("selftest rc=%d out=%q err=%q", rc, out, errb)
	}
	if !strings.Contains(out, "  self-test: 5/5\n") {
		t.Fatalf("selftest 未 5/5: %q", out)
	}
	for _, s := range []string{"① 扫描出来一件（测试件排除）", "② filepath.Join 计数=1", "③ os.ReadDir 计数=1", "④ 自写 PyJoin 计数=1", "⑤ 脏正则类计数>=1"} {
		if !strings.Contains(out, s) {
			t.Fatalf("selftest 缺格: %q in %q", s, out)
		}
	}
}

// TestM2GoPyIsoScanMissingAndEmpty —— 取不到输入（缺根）与空转（0 件非测试 .go）⇒ rc=2，且两者不同形。
func TestM2GoPyIsoScanMissingAndEmpty(t *testing.T) {
	rc, out, errb := m2sRun(t, "--root", filepath.Join(t.TempDir(), "nope"))
	if rc != 2 || out != "" || !strings.Contains(errb, "扫描根不在盘：") || !strings.Contains(errb, m2sVersion+": 扫描根不在盘：") {
		t.Fatalf("missing rc=%d out=%q err=%q", rc, out, errb)
	}

	// core/cmd/zerg 存在但 0 件非测试 .go（只 1 件 _test.go）⇒ 空域 rc=2。
	rootOnlyTest := m2sBuildFx(t, false, false)
	rc, out, errb = m2sRun(t, "--root", rootOnlyTest)
	if rc != 2 || out != "" || !strings.Contains(errb, "空域 —— core/cmd/zerg 下 0 个非测试 .go") {
		t.Fatalf("empty rc=%d out=%q err=%q", rc, out, errb)
	}
	if strings.Contains(errb, "扫描根不在盘") {
		t.Fatalf("空域与缺根同形（不许同形）: %q", errb)
	}
}

// TestM2GoPyIsoScanDefaultRootCwd —— py `--root` 缺省/空 ⇒ cwd（不是 __file__ 上溯）。
func TestM2GoPyIsoScanDefaultRootCwd(t *testing.T) {
	root := m2sBuildFx(t, true, true)
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	rc, out, errb := m2sRun(t, "--json")
	if rc != 0 || errb != "" || !strings.Contains(out, `{"file": "c.go"`) {
		t.Fatalf("default-cwd rc=%d out=%q err=%q", rc, out, errb)
	}
	// 空串 root 亦回落 cwd（py `a.root or os.getcwd()`）。
	rc, _, _ = m2sRun(t, "--root", "")
	if rc != 0 {
		t.Fatalf("empty --root 应回落 cwd 得 0，实得 %d", rc)
	}
}

// TestM2GoPyIsoScanPrefixAbbrev —— argparse 唯一前缀缩写（`--r` / `--j`）收。
func TestM2GoPyIsoScanPrefixAbbrev(t *testing.T) {
	root := m2sBuildFx(t, false, true)
	rc, out, errb := m2sRun(t, "--r", root, "--j")
	if rc != 0 || errb != "" || !strings.Contains(out, `"file": "c.go"`) {
		t.Fatalf("prefix rc=%d out=%q err=%q", rc, out, errb)
	}
	if strings.Contains(out, "schema") && !strings.Contains(out, `{"schema": "zerg/m2-go-py-isomorphism/v1"`) {
		t.Fatalf("prefix json schema 不符: %q", out)
	}
}
