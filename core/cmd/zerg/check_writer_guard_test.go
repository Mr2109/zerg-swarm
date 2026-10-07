// check_writer_guard_test.go —— `check_writer_guard.go`（py `check-writer-guard.py` 的 Go 真搬）的成对负控测试件。
//
// 口径：入口进程内复用（不再 spawn 子进程）· 逐字断言 stdout/stderr/退码三面 · 每条含**反向负控**
// （造 violation ⇒ 必 FAIL）。期望值取自真仓原件现跑（甲/乙两组对拍表见交件）。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cwgRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkWriterGuardGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func cwgTmpWrite(t *testing.T, p, c string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cwgLedger(t *testing.T, fx string, items string) {
	t.Helper()
	cwgTmpWrite(t, filepath.Join(fx, "scripts/gates/gate-check-write-tools.json"),
		`{"schema":"gate-check-write-tools/1","head":"x","count":1,`+
			`"regex":"^#\\s*zerg-class:\\s*(read|write)\\s+(D0|D2|D3)\\s*$","items":[`+items+`]}`)
}

// ① --help：rc=0 且 stdout 逐字 = usage（stderr 空）。
func TestCwgHelp(t *testing.T) {
	rc, out, errs := cwgRun("--help")
	if rc != 0 {
		t.Fatalf("rc=%d want 0", rc)
	}
	if out != cwgUsageText() {
		t.Fatalf("stdout 与 usage_text 不符:\n%q", out)
	}
	if errs != "" {
		t.Fatalf("stderr 非空: %q", errs)
	}
	if !strings.Contains(out, "只读旁路探测器") {
		t.Fatalf("usage 缺首行说明")
	}
}

// ② --self-test：13 格全过 ⇒ rc=0；反向负控：把 ⑩ 台账腿正控的期望反过来（见 TestCwgSelfTestNegControl）。
func TestCwgSelfTest(t *testing.T) {
	rc, out, errs := cwgRun("--self-test")
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%s", rc, errs)
	}
	for _, want := range []string{
		"—— check-writer-guard.py --self-test（合成夹具）——",
		"✓ ① 负控 bad-cp.sh 必须命中（命中 1）",
		"✓ ③ 负控 bad-py.py 必须命中（命中 1）",
		"✓ ⑦ 区别力：命中件集 == 3 个负控件（命中件 = bad-cp.sh,bad-py.py,bad-redirect.sh）",
		"✓ ⑬ 台账空集 ⇒ rc=2 不给结论（rc=2）",
		"── 自检 13 格全过（3 负控必命中 · 2 正控不得命中 · 空转必 2 · 干跑档同计数）──",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("自检输出缺 %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "✗") {
		t.Fatalf("自检出现 ✗:\n%s", out)
	}
}

// ③ 未知旗标：rc=2 · stderr 首行逐字 · stdout 空。
func TestCwgUnknownFlag(t *testing.T) {
	rc, out, errs := cwgRun("--bogus")
	if rc != 2 {
		t.Fatalf("rc=%d want 2", rc)
	}
	if out != "" {
		t.Fatalf("stdout 非空: %q", out)
	}
	if !strings.HasPrefix(errs, "⛔ 不给结论（退 2）：未知旗标 --bogus ⇒ ") {
		t.Fatalf("stderr 首行不符: %q", errs)
	}
	if !strings.Contains(errs, "用法\n") {
		t.Fatalf("stderr 缺 usage 正文")
	}
}

// ④ --json 非法字段：rc=2 · stderr 点名。反向负控：合法字段面 ⇒ 不得退 2。
func TestCwgJSONFieldClosure(t *testing.T) {
	rc, _, errs := cwgRun("--json", "nosuch")
	if rc != 2 {
		t.Fatalf("rc=%d want 2", rc)
	}
	if !strings.Contains(errs, "点名非法字段 nosuch") {
		t.Fatalf("stderr 未点名: %q", errs)
	}
	// 负控：合法闭集字段必须放行（不得退用法错）。
	dir := t.TempDir()
	rc2, _, errs2 := cwgRun("--root", dir, "--json", "root,rc")
	if rc2 != 2 {
		t.Fatalf("空根 rc=%d want 2（空转不当绿）", rc2)
	}
	if strings.Contains(errs2, "字段不在闭集") {
		t.Fatalf("合法字段被误拒: %q", errs2)
	}
}

// ⑤ 命中夹具：一条 `cp` 绕过 ⇒ rc=1 · stdout 人面逐字 · stderr 为空。
//
//	反向负控：把该行改成走 atomic-write.sh ⇒ 必须 rc=0（不得仍判红）。
func TestCwgHitFixture(t *testing.T) {
	fx := t.TempDir()
	cwgTmpWrite(t, filepath.Join(fx, "scripts/gates/demo.sh"), "OLD\n")
	cwgTmpWrite(t, filepath.Join(fx, "scripts/bad.sh"),
		"#!/usr/bin/env bash\n#zerg-class: write D2\ncp /tmp/new "+"scripts/gates/demo.sh\n")
	cwgLedger(t, fx, `{"file":"scripts/bad.sh","class":"write","level":"D2"}`)

	rc, out, errs := cwgRun("--root", fx)
	if rc != 1 {
		t.Fatalf("rc=%d want 1; stderr=%s", rc, errs)
	}
	want := "写者守卫旁路：命中 1 条（扫 2 个脚本件）· 聚合计数如下，逐条面用 --json hits\n" +
		"   CP/MV      1 条\n" +
		"   scripts/bad.sh:3  [CP/MV]  cp /tmp/new " + "scripts/gates/demo.sh\n"
	if out != want {
		t.Fatalf("stdout 不符:\n got %q\nwant %q", out, want)
	}
	if !strings.Contains(errs, "⇒ 整件覆盖写仓内件 scripts/gates/demo.sh 而未走 atomic-write.sh（判为绕过）") {
		t.Fatalf("stderr 未含逐条命中 note: %q", errs)
	}

	// 负控：改走正门 ⇒ 零命中 rc=0。
	fx2 := t.TempDir()
	cwgTmpWrite(t, filepath.Join(fx2, "scripts/gates/demo.sh"), "OLD\n")
	cwgTmpWrite(t, filepath.Join(fx2, "scripts/bad.sh"),
		"#!/usr/bin/env bash\n#zerg-class: write D2\nbash scripts/dev/atomic-write.sh --target scripts/gates/demo.sh\n")
	cwgLedger(t, fx2, `{"file":"scripts/bad.sh","class":"write","level":"D2"}`)
	rc2, out2, _ := cwgRun("--root", fx2)
	if rc2 != 0 {
		t.Fatalf("负控 rc=%d want 0（走正门不得判红）", rc2)
	}
	if !strings.Contains(out2, "零命中 ✓") {
		t.Fatalf("负控 stdout 不符: %q", out2)
	}
}

// ⑥ 空根：rc=2（空转不当绿）· stderr 正文 · stdout 为空。
func TestCwgEmptyRoot(t *testing.T) {
	dir := t.TempDir()
	rc, out, errs := cwgRun("--root", dir)
	if rc != 2 {
		t.Fatalf("rc=%d want 2", rc)
	}
	if !strings.Contains(out, "零命中 ✓（扫 0 个脚本件") {
		t.Fatalf("stdout 不符（py 空根亦印零命中行）: %q", out)
	}
	if !strings.Contains(errs, "空转不当绿") {
		t.Fatalf("stderr 不符: %q", errs)
	}
}

// ⑦ 缺台账：G0 不给结论 rc=2 · stderr 点名台账相对路径。
func TestCwgMissingLedger(t *testing.T) {
	fx := t.TempDir()
	cwgTmpWrite(t, filepath.Join(fx, "scripts/gates/demo.sh"), "OLD\n")
	cwgTmpWrite(t, filepath.Join(fx, "scripts/dev/tool.sh"), "#!/usr/bin/env bash\necho hi\n")
	rc, _, errs := cwgRun("--root", fx)
	if rc != 2 {
		t.Fatalf("rc=%d want 2", rc)
	}
	if !strings.Contains(errs, "台账读不到/解析失败") || !strings.Contains(errs, "gate-check-write-tools.json") {
		t.Fatalf("stderr 不符: %q", errs)
	}
}

// ⑧ 默认根：cwgDefaultRoot（无 --root 时）须落在含 `scripts/gates/` 的仓根上，✗ 不是它的父目录
// （旧 os.Executable 上溯三级从 bin/ 出发会多退一级 ⇒ 扫父目录）。成对负控：探针置于中间层时不得误收。
func TestCwgDefaultRootIsRepoRoot(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(filepath.Join(wd, "..", "..", "..")) // core/cmd/zerg -> 仓根
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(want, "scripts", "gates")); err != nil || !fi.IsDir() {
		t.Skipf("非真仓语境（无 scripts/gates）：%v", err)
	}
	got := cwgDefaultRoot()
	if got != want {
		t.Fatalf("cwgDefaultRoot=%q want %q（仓根）", got, want)
	}
	if _, err := os.Stat(filepath.Join(got, "scripts", "gates")); err != nil {
		t.Fatalf("默认根下无 scripts/gates：%v", err)
	}
	// 负控：仓根的父目录下没有 scripts/gates ⇒ 不得把默认根推到那一级
	parent := filepath.Dir(got)
	if parent != got {
		if _, err := os.Stat(filepath.Join(parent, "scripts", "gates")); err == nil {
			t.Fatalf("父目录 %q 亦含 scripts/gates ⇒ 默认根无鉴别力", parent)
		}
	}
}
