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

// dycTestInvYes = yes 面探针（写面夹具用 · 同为拆串形态）。
const dycTestInvYes = "inv." + "yes"

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
	if rc != 0 || !strings.Contains(out, "self-test: 12/12") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

// ── 判据三档 ──────────────────────────────────────────────────────────────
// ★ 写面夹具（GAP-20261014-65 语义收窄后）：缺者必须**是写面**（含 yes 面）才报红 ——
// 件只读 dryRun 而全件无 yes 面 ⇒ 归「只读/转发面」，不入缺（见 TestDycReadOnlyFaceGreen）。
func TestDycViolationRed(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/a.go": "package main\nfunc f(inv *invocation) int { if " + dycTestInvDry + " { return 0 }; _ = " + dycTestInvYes + "; return 1 }\n",
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 1 {
		t.Fatalf("应 rc=1，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "   - core/cmd/zerg/a.go\n") {
		t.Fatalf("应点名 a.go，out=%q", out)
	}
}

// ★ 只读/转发面负控（GAP-20261014-65 的牙）：件读 dryRun 但**全件无 yes 面**（不落盘、不需授权）
// ⇒ 必须在**只读/转发面**列、且**不入缺** ⇒ rc=0。哑化该语义（把 `!r.YesFace` 支删掉）⇒ 本用例必红。
func TestDycReadOnlyFaceGreen(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/ro.go": "package main\nfunc f(inv *invocation) int { if " + dycTestInvDry + " { return 0 }; return 1 }\n",
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 0 {
		t.Fatalf("只读/转发面应 rc=0（不入缺），实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "只读/转发面 1 件") || !strings.Contains(out, "⇒ 全绿（rc=0）") {
		t.Fatalf("out=%q", out)
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
		"core/cmd/zerg/a.go": "package main\nfunc cmdF(inv *invocation) int { if " + dycTestInvDry + " { return 0 }; _ = " + dycTestInvYes + "; return 1 }\n",
	})
	rc, out, _ := dycTestRun([]string{"--root", d, "--json"})
	if rc != 1 {
		t.Fatalf("rc=%d", rc)
	}
	want := `{"schema": "zerg/dryrun-yes-conflict-coverage/v1", "reads": ["a.go"], "covered": [], "exempt": [], "read_only": [], "missing": ["a.go"], "missing_sites": ["a.go:cmdF"]}` + "\n"
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
	if !strings.Contains(out, "self-test: 12/12") {
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
	if !strings.Contains(out, "self-test: 12/12") {
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

	// 成对反控（✗ 禁恒红）：可读同构**写面**夹具 ⇒ rc=1（缺者报红）且 stderr 无 BLOCKED。
	d2 := dycTestFx(t, map[string]string{
		"core/cmd/zerg/a.go": "package main\nfunc f(inv *invocation) int { if " + dycTestInvDry + " { return 0 }; _ = " + dycTestInvYes + "; return 1 }\n",
	})
	rcOK, _, errOK := dycTestRun([]string{"--root", d2})
	if rcOK != 1 || strings.Contains(errOK, "BLOCKED") {
		t.Fatalf("成对反控失败：应 rc=1 且无 BLOCKED，得 rc=%d err=%q", rcOK, errOK)
	}
}

// ── ★ 调用点粒度档（缺口 GAP-20261014-66 的后半）───────────────────────────────
//
// 病（缺口逐字，摘要）：件粒度 token 判只问「件里有没有判定字面」，不问「判定调用点在不在
// 该命令的干跑早退**之前**」⇒ 同件**一闸一吞**时，件粒度面判**绿**（假绿），运行时装吞 `--yes`。
// 下面三条用例就是这一档的两态：① 同件一闸一吞 ⇒ 必红且**只点名吞的那条命令**；② 成对负控
// （判定排在早退之后 ⇒ 必红）；③ 反例（每条命令判定都在早退之前 ⇒ ✗ 不得点名 ⇒ rc=0）。

// dycSiteFxGood / dycSiteFxBad —— 两条命令的夹具体（判定位置是唯一变量）。
func dycSiteFxGuarded(fn string) string {
	return "func " + fn + "(inv *invocation) int {\n" +
		"	if rc := dryRun" + "YesConflict(inv, nil); rc != 0 {\n		return rc\n	}\n" +
		"	if " + dycTestInvDry + " {\n		return 0\n	}\n" +
		"	_ = " + dycTestInvYes + "\n	return 1\n}\n"
}

func dycSiteFxSwallow(fn string) string {
	return "func " + fn + "(inv *invocation) int {\n" +
		"	if " + dycTestInvDry + " {\n		return 0\n	}\n" +
		"	_ = " + dycTestInvYes + "\n	return 1\n}\n"
}

// dycSiteFxLate —— 判定**排在干跑早退之后**（改前 family_dev.go 四条的形态）。
func dycSiteFxLate(fn string) string {
	return "func " + fn + "(inv *invocation) int {\n" +
		"	if " + dycTestInvDry + " {\n		return 0\n	}\n" +
		"	if rc := dryRun" + "YesConflict(inv, nil); rc != 0 {\n		return rc\n	}\n" +
		"	_ = " + dycTestInvYes + "\n	return 1\n}\n"
}

// ★ 夹具：同件“一条有闸、另一条吞 --yes” ⇒ 件粒度面**绿**（件里有判定字面）而调用点粒度面**红**，
// 且必须**点名吞的那条命令**、✗ 不得点名有闸的那条。
func TestDycSiteGrainNamesSwallower(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/mix.go": "package main\n" +
			dycSiteFxGuarded("cmdGood") + dycSiteFxSwallow("cmdBad"),
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 1 {
		t.Fatalf("同件一闸一吞 ⇒ 应 rc=1（件粒度绿 · 调用点粒度红），实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "   - core/cmd/zerg/mix.go:cmdBad\n") {
		t.Fatalf("应点名 mix.go:cmdBad（吞 --yes 的那条），out=%q", out)
	}
	if strings.Contains(out, "mix.go:cmdGood") {
		t.Fatalf("✗ 不得点名有闸的那条命令（cmdGood），out=%q", out)
	}
}

// ★ 成对负控（want 逐字「先 return 的件必红」）：判定字面**在件里**、但排在干跑早退之后。
func TestDycSiteGrainGuardAfterEarlyReturnRed(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/late.go": "package main\n" + dycSiteFxLate("cmdLate"),
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 1 {
		t.Fatalf("判定排在干跑早退之后 ⇒ 必红 rc=1，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "   - core/cmd/zerg/late.go:cmdLate\n") {
		t.Fatalf("应点名 late.go:cmdLate，out=%q", out)
	}
}

// ★ 反例（✗ 不误伤）：真写面（命令里真读该字段 + 有 yes 面）且**每条命令**的判定都在早退之前。
func TestDycSiteGrainAllGuardedGreen(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/ok.go": "package main\n" +
			dycSiteFxGuarded("cmdAlpha") +
			"func cmdBeta(inv *invocation) int {\n" +
			"	if rc := write" + "GuardYes(inv, nil, nil); rc != 0 {\n		return rc\n	}\n" +
			"	if " + dycTestInvDry + " {\n		return 0\n	}\n" +
			"	_ = " + dycTestInvYes + "\n	return 1\n}\n",
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 0 {
		t.Fatalf("每条命令判定都在早退之前 ⇒ 应 rc=0，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "全绿（rc=0）") || !strings.Contains(out, "调用点缺 0 处") {
		t.Fatalf("out=%q", out)
	}
}

// ★ 冻结名单件（只读干跑面）整件豁免：即便块内形态「像」吞 --yes，也 ✗ 不进调用点缺面（与件粒度面同规）。
func TestDycSiteGrainFrozenRosterExempt(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/export.go": "package main\n" + dycSiteFxSwallow("cmdHelpExport"),
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 0 {
		t.Fatalf("冻结名单件应 rc=0，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "冻结名单 1 件") {
		t.Fatalf("out=%q", out)
	}
}

// ── ★ 被守卫的助手件档（缺口 GAP-20261014-66 后半第二刀 · 收窄假红面）─────────────
//
// 病（假红）：纯助手件（无 `cmd` 块）读该字段 + 有 yes 面，判定却长在**它的调用方命令块**里（跨件）
// ⇒ 件粒度 token 判只见「本件无判定字面」⇒ 判缺。下面两枚夹具就是这一档的两态：① 调用方命令块
// 有判定（且排在调用之前）⇒ 助手件 ✗ 不得判缺；② 成对负控（调用方无判定）⇒ 助手件与调用点同红。

// dycHlpFxCallerGuarded —— 调用方命令块（判定在最前 ⇒ 已守卫）。
func dycHlpFxCallerGuarded(fn, helper string) string {
	return "func " + fn + "(inv *invocation) int {\n" +
		"	if rc := dryRun" + "YesConflict(inv, nil); rc != 0 {\n		return rc\n	}\n" +
		"	if " + dycTestInvDry + " {\n		return 0\n	}\n" +
		"	_ = " + dycTestInvYes + "\n	return " + helper + "(inv)\n}\n"
}

// dycHlpFxCallerUnguarded —— 调用方命令块（**无**判定字面 ⇒ 成对负控）。
func dycHlpFxCallerUnguarded(fn, helper string) string {
	return "func " + fn + "(inv *invocation) int {\n" +
		"	if " + dycTestInvDry + " {\n		return 0\n	}\n" +
		"	_ = " + dycTestInvYes + "\n	return " + helper + "(inv)\n}\n"
}

// dycHlpFxHelper —— 纯助手件（无 `cmd` 块 ⇒ 件粒度 token 判只见「本件无判定字面」）。
func dycHlpFxHelper(fn string) string {
	return "func " + fn + "(inv *invocation) int {\n" +
		"	if " + dycTestInvDry + " {\n		return 0\n	}\n" +
		"	_ = " + dycTestInvYes + "\n	return 1\n}\n"
}

// ★ 反例（✗ 不误伤）：守卫在**调用方命令块**里（跨件）⇒ 助手件视同真产者 ⇒ 全绿。
func TestDycGuardedHelperGreen(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/caller.go": "package main\n" + dycHlpFxCallerGuarded("cmdCaller", "helperRead"),
		"core/cmd/zerg/helper.go": "package main\n" + dycHlpFxHelper("helperRead"),
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 0 {
		t.Fatalf("守卫在调用方 ⇒ 助手件 ✗ 不得判缺（应 rc=0），实得 rc=%d out=%q", rc, out)
	}
	if strings.Contains(out, "helper.go") {
		t.Fatalf("✗ 不得点名被守卫的助手件，out=%q", out)
	}
	if !strings.Contains(out, "⇒ 全绿（rc=0）") {
		t.Fatalf("out=%q", out)
	}
}

// ★ 成对负控：调用方命令块**无判定字面** ⇒ 助手件（件粒度）与调用点（调用点粒度）两态同红。
func TestDycGuardedHelperUnguardedRed(t *testing.T) {
	d := dycTestFx(t, map[string]string{
		"core/cmd/zerg/caller.go": "package main\n" + dycHlpFxCallerUnguarded("cmdCaller", "helperRead"),
		"core/cmd/zerg/helper.go": "package main\n" + dycHlpFxHelper("helperRead"),
	})
	rc, out, _ := dycTestRun([]string{"--root", d})
	if rc != 1 {
		t.Fatalf("调用方无判定 ⇒ 必红 rc=1，实得 rc=%d out=%q", rc, out)
	}
	if !strings.Contains(out, "   - core/cmd/zerg/helper.go\n") {
		t.Fatalf("应点名 helper.go（件粒度），out=%q", out)
	}
	if !strings.Contains(out, "   - core/cmd/zerg/caller.go:cmdCaller\n") {
		t.Fatalf("应点名 caller.go:cmdCaller（调用点粒度），out=%q", out)
	}
}
