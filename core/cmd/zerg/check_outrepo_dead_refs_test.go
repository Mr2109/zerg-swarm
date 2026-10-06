package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// codrRun —— 跑入口取 (rc, stdout, stderr)。★ 不塞假程序名（inproc 口径 argv 不含程序名）。
func codrRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkOutrepoDeadRefsGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// codrWrite —— 在 root 下按 slash 相对路径写件（自动建父）。
func codrWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o666); err != nil {
		t.Fatal(err)
	}
}

// codrDeadFixture —— 语言中立夹具：`references/ok.md` 真在盘；`references/gone.md` /
// `scripts/nope.py` 缺席 ⇒ 后两者为死指针（共 2 处）。
func codrDeadFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	codrWrite(t, root, "skill-a/references/ok.md", "x\n")
	codrWrite(t, root, "skill-a/SKILL.md",
		"see `references/ok.md` 和 `references/gone.md` 还有 `scripts/nope.py`\n")
	return root
}

func TestCodrSelfTestGreen(t *testing.T) {
	rc, out, errs := codrRun("--self-test")
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d stderr=%q out=%q", rc, errs, out)
	}
	if !strings.Contains(out, "合计 6 格 · 全绿") {
		t.Fatalf("自检输出异常: %q", out)
	}
}

func TestCodrHelpDocByteExact(t *testing.T) {
	want := codrDoc + "\n"
	for _, a := range []string{"--help", "-h", "--list-rules"} {
		rc, out, errs := codrRun(a)
		if rc != 0 || errs != "" || out != want {
			t.Fatalf("%s: rc=%d stderr=%q 逐字同=%v（len %d vs %d）", a, rc, errs, out == want, len(out), len(want))
		}
	}
}

func TestCodrMissingRootRc2(t *testing.T) {
	rc, _, errs := codrRun("--root", filepath.Join(t.TempDir(), "no-such-dir"))
	if rc != 2 || !strings.Contains(errs, "语料根不存在") {
		t.Fatalf("rc=%d stderr=%q", rc, errs)
	}
}

func TestCodrEmptyRootRc2(t *testing.T) {
	rc, _, errs := codrRun("--root", t.TempDir())
	if rc != 2 || !strings.Contains(errs, "扫到 0 个 .md 件") {
		t.Fatalf("rc=%d stderr=%q", rc, errs)
	}
}

func TestCodrDeadHitRc1(t *testing.T) {
	root := codrDeadFixture(t)
	rc, out, _ := codrRun("--root", root)
	if rc != 1 || !strings.Contains(out, "死指针 2 处") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "`references/gone.md`") || strings.Contains(out, "`references/ok.md`") {
		t.Fatalf("命中面不对: %q", out)
	}
}

func TestCodrCleanRc0(t *testing.T) {
	root := t.TempDir()
	codrWrite(t, root, "s/references/ok.md", "x\n")
	codrWrite(t, root, "s/SKILL.md", "see `references/ok.md`\n")
	rc, out, _ := codrRun("--root", root)
	if rc != 0 || !strings.Contains(out, "死指针 0 处 / 0 件") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

func TestCodrJSONShape(t *testing.T) {
	root := codrDeadFixture(t)
	rc, out, _ := codrRun("--json", "--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d", rc)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, `{"schema": "outrepo-dead-refs/1", "root": `) || !strings.Contains(last, `"dead": 2`) {
		t.Fatalf("json 面不对: %q", last)
	}
}

func TestCodrReportOnlyRc0(t *testing.T) {
	root := codrDeadFixture(t)
	rc, out, _ := codrRun("--report-only", "--root", root)
	if rc != 0 || !strings.Contains(out, "[只报告档]") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

func TestCodrUnknownFlagRc2(t *testing.T) {
	rc, _, errs := codrRun("--nope")
	if rc != 2 || errs != "✗ 未知参数: --nope\n" {
		t.Fatalf("rc=%d stderr=%q", rc, errs)
	}
}

func TestCodrRootMissingValueRc2(t *testing.T) {
	rc, _, errs := codrRun("--root")
	if rc != 2 || errs != "✗ 未知参数: --root（缺值）\n" {
		t.Fatalf("rc=%d stderr=%q", rc, errs)
	}
}

// 负控（有牙）：空集合 ≠ 取不到 —— 语料根在盘且含 .md、但零命中 ⇒ rc=0（✗ 不许当 rc=2）。
func TestCodrEmptySetNotMissing(t *testing.T) {
	root := t.TempDir()
	codrWrite(t, root, "s/INDEX.md", "# 标题\n")
	rc, out, _ := codrRun("--root", root)
	if rc != 0 {
		t.Fatalf("空集合被误当取不到: rc=%d out=%q", rc, out)
	}
}

// 负控（有牙）：解析器必须区分「存在件」与「缺席件」—— 若实现是「凡引用皆报」则此格必红。
func TestCodrResolverDistinguishes(t *testing.T) {
	root := codrDeadFixture(t)
	_, hits := codrScan(root)
	got := map[string]bool{}
	for _, h := range hits {
		got[h.ref] = true
	}
	if !got["references/gone.md"] || !got["scripts/nope.py"] {
		t.Fatalf("缺席件未被记: %v", got)
	}
	if got["references/ok.md"] {
		t.Fatalf("存在件被误记（凡引用皆报的假实现）: %v", got)
	}
}
