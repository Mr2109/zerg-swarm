package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 测试助手（前缀 dycTest…，✗ 不用 TestMain / init，避与同包他件撞名）────────────
// dycTestInvDry = 拆串形态的探针（见主件声明 (b)）；py 侧 REM: 同形。
const dycTestInvDry = "inv." + "dryRun"

func dycTestFx(t *testing.T, files map[string]string) string {
	t.Helper()
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "core", "cmd", "zerg"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		p := filepath.Join(d, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func dycTestRun(argv []string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkDryrunYesConflictCoverageGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func dycReader(src string) string {
	p := filepath.Join("core", "cmd", "zerg", src)
	if b, err := os.ReadFile(p); err == nil {
		return string(b)
	}
	if b, err := os.ReadFile(src); err == nil { // go test cwd == 包目录
		return string(b)
	}
	return ""
}

// ── 自指面（声明 (b) 的牙）────────────────────────────────────────────────
func TestDycSourceSelfRefClean(t *testing.T) {
	src := dycReader("check_dryrun_yes_conflict_coverage.go")
	if src == "" {
		t.Fatalf("读不到本件源（cwd 异常）⇒ 自指断言不可判")
	}
	if dycReadsRe.MatchString(src) {
		t.Fatalf("本件源被自己的 READS_RE 命中（拆串失效）⇒ 会被扫成应产者")
	}
}

// ── 帮助 / argparse 面 ────────────────────────────────────────────────────
func TestDycHelpFace(t *testing.T) {
	rc, out, errs := dycTestRun([]string{"-h"})
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	want := "usage: check-dryrun-yes-conflict-coverage.py [-h] [--root ROOT] [--json]\n" +
		strings.Repeat(" ", len("usage: ")+len("check-dryrun-yes-conflict-coverage.py")+1) +
		"[--emit-baseline] [--self-test]\n\noptional arguments:\n" +
		"  -h, --help       show this help message and exit\n  --root ROOT\n  --json\n" +
		"  --emit-baseline\n  --self-test\n"
	if out != want {
		t.Fatalf("帮助面分叉\n got=%q\nwant=%q", out, want)
	}
}

func TestDycUnknownFlagRc2(t *testing.T) {
	rc, out, errs := dycTestRun([]string{"--nope"})
	if rc != 2 || out != "" {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	if !strings.Contains(errs, "error: unrecognized arguments: --nope\n") {
		t.Fatalf("err=%q", errs)
	}
}

func TestDycMissingValueRc2(t *testing.T) {
	rc, _, errs := dycTestRun([]string{"--root"})
	if rc != 2 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(errs, "argument --root: expected one argument\n") {
		t.Fatalf("err=%q", errs)
	}
}

func TestDycAbbrevResolve(t *testing.T) {
	// `--se` 唯一前缀 → --self-test
	rc, out, _ := dycTestRun([]string{"--se"})
	if rc != 0 || !strings.Contains(out, "self-test: 6/6") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

// ── 判据三档 ──────────────────────────────────────────────────────────────
func TestDycViolationRed(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/a.go": "package main\nfunc f(inv *invocation) int { if " + dycTestInvDry + " { return 0 }; return 1 }\n",
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 1 {
		t.Fatalf("应 rc=1，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "   - core/cmd/zerg/a.go\n") {
		t.Fatalf("应点名 a.go，out=%q", out)
	}
}

func TestDycCoveredGreen(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/b.go": "package main\nfunc f(inv *invocation) int { if rc := dryRun" + "YesConflict(inv, nil); rc != 0 { return rc }; _ = " + dycTestInvDry + "; return 0 }\n",
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 0 {
		t.Fatalf("应 rc=0，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "⇒ 全绿（rc=0）") {
		t.Fatalf("out=%q", out)
	}
}

func TestDycExemptFace(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/export.go": "package main\nfunc f(inv *invocation) int { if " + dycTestInvDry + " { return 0 }; return 1 }\n",
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 0 {
		t.Fatalf("冻结名单件应 rc=0，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "冻结名单 1 件") {
		t.Fatalf("out=%q", out)
	}
}

func TestDycJSONShape(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/a.go": "package main\nfunc f(inv *invocation) int { if " + dycTestInvDry + " { return 0 }; return 1 }\n",
	})
	rc, out, _ := dycTestRun([]string{"--root", d, "--json"})
	if rc != 1 {
		t.Fatalf("rc=%d", rc)
	}
	want := `{"schema": "zerg/dryrun-yes-conflict-coverage/v1", "reads": ["a.go"], "covered": [], "exempt": [], "missing": ["a.go"]}` + "\n"
	if out != want {
		t.Fatalf("JSON 分叉\n got=%q\nwant=%q", out, want)
	}
}

func TestDycEmitBaseline(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/a.go": "package main\nfunc f(inv *invocation) int { if " + dycTestInvDry + " { return 0 }; return 1 }\n",
	})
	rc, out, _ := dycTestRun([]string{"--root", d, "--emit-baseline"})
	if rc != 0 || out != `{"read_only_faces": ["export.go"]}`+"\n" {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

func TestDycEmptyDomainRc2(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/x.go": "package main\nfunc g() int { return 0 }\n",
	})
	rc, out, errs := dycTestRun([]string{"--root", d})
	if rc != 2 || out != "" {
		t.Fatalf("空域应 rc=2，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(errs, "空域") {
		t.Fatalf("err=%q", errs)
	}
}

func TestDycRootMissingRc2(t *testing.T) {
	rc, _, errs := dycTestRun([]string{"--root", "/no/such/dir-dyc-xyz"})
	if rc != 2 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(errs, "扫描根不在盘") {
		t.Fatalf("err=%q", errs)
	}
}

func TestDycSelfTestGreen(t *testing.T) {
	rc, out, errs := dycTestRun([]string{"--self-test"})
	if rc != 0 || errs != "" {
		t.Fatalf("rc=%d err=%q", rc, errs)
	}
	if !strings.Contains(out, "self-test: 6/6") {
		t.Fatalf("out=%q", out)
	}
}

// A 类止血牙：自检必须回收 tempdir（删主件 defer ⇒ 本用例必红）。
func TestDycSelfTestCleanupDelta0(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	rc, out, _ := dycTestRun([]string{"--self-test"})
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(out, "self-test: 6/6") {
		t.Fatalf("out=%q", out)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("A 类 tempdir 未回收：私有 TMPDIR 残留 %d 项（应 0）", len(ents))
	}
}

// ── B 类（读失败静默）回归牙 ────────────────────────────────────────────────────
// 应产者件锁成 chmod 000（**仓外夹具**）⇒ 读失败登记面（dycReadFails）必响：
// rc=2 + stderr BLOCKED 头 + 逐件点名 a.go。删回静默（原 py `except OSError: continue`）⇒ 该件静默少判。
func TestDycReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 绕过权限位 ⇒ chmod 000 不挡读，本用例无鉴别力")
	}
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/a.go": "package main\nfunc f(inv *invocation) int { if " + dycTestInvDry + " { return 0 }; return 1 }\n",
		"core/cmd/zerg/b.go": "package main\nfunc g(inv *invocation) int { if rc := dryRun" + "YesConflict(inv, nil); rc != 0 { return rc }; _ = " + dycTestInvDry + "; return 0 }\n",
	})
	locked := filepath.Join(d, "core", "cmd", "zerg", "a.go")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o644)

	rc, _, errs := dycTestRun([]string{"--root", d})
	if rc != 2 {
		t.Fatalf("读失败回归牙：锁死件 ⇒ 应 rc=2，得 rc=%d err=%q", rc, errs)
	}
	if !strings.Contains(errs, "BLOCKED") || !strings.Contains(errs, "a.go") {
		t.Fatalf("读失败回归牙：stderr 须带 BLOCKED 头 + 点名 a.go，得 err=%q", errs)
	}

	// 成对反控（✗ 禁恒红）：可读同构夹具 ⇒ rc=1（缺者报红）且 stderr 无 BLOCKED。
	d2 := dycTestFx(t, map[string]string{
		"core/cmd/zerg/a.go": "package main\nfunc f(inv *invocation) int { if " + dycTestInvDry + " { return 0 }; return 1 }\n",
	})
	rcOK, _, errOK := dycTestRun([]string{"--root", d2})
	if rcOK != 1 || strings.Contains(errOK, "BLOCKED") {
		t.Fatalf("成对反控失败：应 rc=1 且无 BLOCKED，得 rc=%d err=%q", rcOK, errOK)
	}
}
