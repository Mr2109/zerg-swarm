// check_write_authz_test.go —— `check_write_authz.go` 的成对负控（✗ 禁恒绿）。
//
// 覆盖（每格都是「两态」的一侧；py 侧对拍照见回执）：
//
//	· 运行档：mix（有裸写点 ⇒ rc=1 ＋ 逐条点名）/ clean（全有交代 ⇒ rc=0）
//	  / 缺 core/cmd/zerg（rc=2）/ 缺 main.go（rc=2）/ --allow 豁免（rc=0）/ --json（逐字包封）。
//	· 自检档：--self-test ⇒ rc=0 ＋ 尾行「自检 PASS（4 格）」。
//	· argparse 档：-h（rc=0 ＋ 帮助全文）/ 未知旗标（rc=2 ＋ usage + error）/ --root 缺值（rc=2）。
//	· 负控（有牙）：① 注释里的写点不算 ② 登记表命中不算 ③ family_ok 不得进 hits。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cwaTMain = "var x = []C{{danger: &dangerSpec{...}, run: cmdGuardedX}}\n"
const cwaTBare = "func bareWriter(inv *invocation) int {\n\tos.WriteFile(\"x\", b, 0o644)\n\treturn 0\n}\n"
const cwaTOk = "type w struct{ Call string }\n" +
	"var writePoints = []w{\n\t{Call: \"os.WriteFile(tmp, b, 0o644)\"},\n}\n" +
	"func okWriter() { os.WriteFile(tmp, b, 0o644) }\n"
const cwaTComment = "func comWriter() int {\n\t// os.WriteFile(commented)\n\treturn 0\n}\n"

func cwaTMkFixture(t *testing.T, mainSrc, bareSrc, okSrc string) string {
	t.Helper()
	root := t.TempDir()
	d := filepath.Join(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if mainSrc != "" {
		if err := os.WriteFile(filepath.Join(d, "main.go"), []byte(mainSrc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if bareSrc != "" {
		if err := os.WriteFile(filepath.Join(d, "family_bare.go"), []byte(bareSrc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if okSrc != "" {
		if err := os.WriteFile(filepath.Join(d, "family_ok.go"), []byte(okSrc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func cwaTRun(argv ...string) (int, string, string) {
	var so, se bytes.Buffer
	rc := writeAuthzGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

func TestCwaRunFaces(t *testing.T) {
	// mix：1 条裸写点 ⇒ rc=1 ＋ 逐字点名。
	root := cwaTMkFixture(t, cwaTMain, cwaTBare, cwaTOk)
	rc, out, errs := cwaTRun("--root", root)
	if rc != 1 || errs != "" {
		t.Fatalf("mix rc=%d err=%q", rc, errs)
	}
	want := "写点授权元门（GAP-20260927-140）\n" +
		"扫面 core/cmd/zerg/*.go（非测试）· 裸写点（既无写点登记、所属命令面又无 danger）= 1 条\n" +
		"  ✗ family_bare.go:2 func=bareWriter call=os.WriteFile ⇒ os.WriteFile(\"x\", b, 0o644)\n" +
		"  口径：写点前须有交代 —— 写点登记表命中 或 所属命令面在 catalog() 有 danger。\n"
	if out != want {
		t.Fatalf("mix out 不符:\n got %q\nwant %q", out, want)
	}

	// clean：登记表命中 ⇒ rc=0。
	root2 := cwaTMkFixture(t, cwaTMain, "", cwaTOk)
	rc, out, _ = cwaTRun("--root", root2)
	if rc != 0 || !strings.Contains(out, "= 0 条") {
		t.Fatalf("clean rc=%d out=%q", rc, out)
	}

	// 缺 core/cmd/zerg ⇒ rc=2。
	empty := t.TempDir()
	rc, out, errs = cwaTRun("--root", empty)
	if rc != 2 || out != "" || !strings.Contains(errs, "缺 `core/cmd/zerg/`") {
		t.Fatalf("nocore rc=%d out=%q err=%q", rc, out, errs)
	}

	// 缺 main.go ⇒ rc=2。
	d := filepath.Join(t.TempDir(), "core", "cmd", "zerg")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	rc, out, errs = cwaTRun("--root", filepath.Dir(filepath.Dir(filepath.Dir(d))))
	if rc != 2 || out != "" || !strings.Contains(errs, "缺 `main.go`") {
		t.Fatalf("nocmain rc=%d out=%q err=%q", rc, out, errs)
	}
}

func TestCwaJSONFace(t *testing.T) {
	root := cwaTMkFixture(t, cwaTMain, cwaTBare, cwaTOk)
	rc, out, _ := cwaTRun("--root", root, "--json")
	want := "{\"rc\": 1, \"bare_write_count\": 1, \"hits\": [{\"file\": \"family_bare.go\", \"line\": 2, \"func\": \"bareWriter\", \"call\": \"os.WriteFile\"}]}\n"
	if rc != 1 || out != want {
		t.Fatalf("json rc=%d\n got %q\nwant %q", rc, out, want)
	}
	// 空命中：键在场且空数组（✗ 不是 null）。
	root2 := cwaTMkFixture(t, cwaTMain, "", cwaTOk)
	rc, out, _ = cwaTRun("--root", root2, "--json")
	want0 := "{\"rc\": 0, \"bare_write_count\": 0, \"hits\": []}\n"
	if rc != 0 || out != want0 {
		t.Fatalf("json-empty rc=%d\n got %q\nwant %q", rc, out, want0)
	}
}

func TestCwaNegativeControls(t *testing.T) {
	// ① 注释里的写点不算；② 登记表命中不算。二者同件 ⇒ 0 命中。
	root := cwaTMkFixture(t, cwaTMain, cwaTComment, cwaTOk)
	rc, out, _ := cwaTRun("--root", root)
	if rc != 0 || !strings.Contains(out, "= 0 条") {
		t.Fatalf("注释写点被误判: rc=%d out=%q", rc, out)
	}
	// ③ --allow 豁免 family_bare.go:os.WriteFile ⇒ 0 命中（豁免真生效）。
	root2 := cwaTMkFixture(t, cwaTMain, cwaTBare, cwaTOk)
	rc, out, _ = cwaTRun("--root", root2, "--allow", "family_bare.go:os.WriteFile")
	if rc != 0 || !strings.Contains(out, "= 0 条") {
		t.Fatalf("--allow 未生效: rc=%d out=%q", rc, out)
	}
	// ③' 反证：不豁免则 rc=1（同一夹具）⇒ 证明上一条之差真来自豁免面。
	rc, _, _ = cwaTRun("--root", root2)
	if rc != 1 {
		t.Fatalf("反证失败：应 rc=1，得 %d", rc)
	}
	// ④ danger 函数里的写点不算（cmdGuardedX 是 danger 命令面）。
	root3 := cwaTMkFixture(t, cwaTMain, "func cmdGuardedX(inv *invocation) int {\n\tos.WriteFile(\"x\", b, 0o644)\n\treturn 0\n}\n", "")
	rc, out, _ = cwaTRun("--root", root3)
	if rc != 0 || !strings.Contains(out, "= 0 条") {
		t.Fatalf("danger 函数写点被误判: rc=%d out=%q", rc, out)
	}
}

func TestCwaSelfTest(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	rc, out, _ := cwaTRun("--self-test")
	if rc != 0 {
		t.Fatalf("self-test rc=%d", rc)
	}
	if !strings.HasSuffix(out, "自检 PASS（4 格）\n") {
		t.Fatalf("self-test 尾行不符: %q", out)
	}
}

func TestCwaHelpAndArgparse(t *testing.T) {
	rc, out, errs := cwaTRun("-h")
	if rc != 0 || errs != "" {
		t.Fatalf("help rc=%d err=%q", rc, errs)
	}
	if !strings.HasPrefix(out, "usage: check-write-authz.py [-h] [--root ROOT] [--allow ALLOW] [--json]\n") {
		t.Fatalf("help usage 不符: %q", out)
	}
	if !strings.Contains(out, "optional arguments:\n  -h, --help     show this help message and exit\n") {
		t.Fatalf("help 正文不符: %q", out)
	}
	// 未知旗标 ⇒ rc=2 ＋ usage ＋ error。
	rc, out, errs = cwaTRun("--bogus")
	if rc != 2 || out != "" || !strings.Contains(errs, "check-write-authz.py: error: unrecognized arguments: --bogus") {
		t.Fatalf("unknown rc=%d out=%q err=%q", rc, out, errs)
	}
	// --root 缺值 ⇒ rc=2。
	rc, _, errs = cwaTRun("--root")
	if rc != 2 || !strings.Contains(errs, "argument --root: expected one argument") {
		t.Fatalf("root-noval rc=%d err=%q", rc, errs)
	}
	// 唯一前缀缩写：--js == --json。
	root := cwaTMkFixture(t, cwaTMain, cwaTBare, cwaTOk)
	rcA, outA, _ := cwaTRun("--root", root, "--json")
	rcB, outB, _ := cwaTRun("--root", root, "--js")
	if rcA != rcB || outA != outB {
		t.Fatalf("前缀缩写 --js 未同 --json: %q ⟷ %q", outA, outB)
	}
}
