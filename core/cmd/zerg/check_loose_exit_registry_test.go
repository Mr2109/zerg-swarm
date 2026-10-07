package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clerTPut 在 root/scripts/gates/<name> 写一个合成件（夹具落 t.TempDir() 仓外 · 不碰真目标）。
func clerTPut(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, "scripts", "gates", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// clerTReg 写 6 个已登记脚本（与 LOOSE_EXIT 逐条对上 ⇒ 全绿基线）。
func clerTReg(t *testing.T, root string) {
	t.Helper()
	clerTPut(t, root, "check-doc-meta.py", "import argparse\nap = argparse.ArgumentParser()\nap.add_argument('--missing', default='warn')\n")
	clerTPut(t, root, "check-git-path-outlet.py", "import argparse\nap = argparse.ArgumentParser()\nap.add_argument('--tier', default='report')\n")
	clerTPut(t, root, "check-wired-scripts.py", "ap.add_argument('--strict-report', action='store_true')\n")
	clerTPut(t, root, "check-doc-freshness.py", "ap.add_argument('--strict-all', action='store_true')\n")
	clerTPut(t, root, "check-no-symlink-farm.py", "ap.add_argument('--strict-links', action='store_true')\n")
	clerTPut(t, root, "check-version-sources.py", "ap.add_argument('--strict-docs', action='store_true')\n")
}

func clerTRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkLooseExitRegistryGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// 全绿基线：6 只脚本检出且逐条登记 ⇒ rc=0 · stderr 空。
func TestClerAllRegisteredGreen(t *testing.T) {
	root := t.TempDir()
	clerTReg(t, root)
	rc, out, errs := clerTRun("--root", root)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; out=%q err=%q", rc, out, errs)
	}
	if !strings.Contains(out, "⇒ 全绿（rc=0）：6 只脚本") {
		t.Fatalf("缺少全绿行: %q", out)
	}
	if errs != "" {
		t.Fatalf("stderr 应空: %q", errs)
	}
}

// A 档负控（要牙）：未登记的 default='warn' 件 ⇒ rc=1 且点名。
// ★ 哑化 `defval == "warn"` 这一支 ⇒ 本用例必 FAIL（带行号）。
func TestClerNegativeControlDefaultWarnBranch(t *testing.T) {
	root := t.TempDir()
	clerTReg(t, root)
	clerTPut(t, root, "check-extra.py", "ap.add_argument('--zzz', default='warn')\n")
	rc, out, _ := clerTRun("--root", root)
	if rc != 1 {
		t.Fatalf("rc=%d want 1 (A 漏登记); out=%q", rc, out)
	}
	if !strings.Contains(out, "── A 漏登记（检出却没进登记表）：1 处") ||
		!strings.Contains(out, "check-extra.py（检出宽松旗标：--zzz）") {
		t.Fatalf("A 档未点名: %q", out)
	}
}

// A 档负控（default='report' 支）：哑化 report 支 ⇒ 本用例必 FAIL。
func TestClerNegativeControlDefaultReportBranch(t *testing.T) {
	root := t.TempDir()
	clerTReg(t, root)
	clerTPut(t, root, "check-rep.py", "ap.add_argument('--tier', default='report')\n")
	rc, out, _ := clerTRun("--root", root)
	if rc != 1 || !strings.Contains(out, "--tier") {
		t.Fatalf("rc=%d want 1 report 支; out=%q", rc, out)
	}
}

// A 档负控（--strict-* + store_true 支）：哑化该支 ⇒ 本用例必 FAIL。
func TestClerNegativeControlStrictTrueBranch(t *testing.T) {
	root := t.TempDir()
	clerTReg(t, root)
	clerTPut(t, root, "check-st.py", "ap.add_argument('--strict-mine', action='store_true')\n")
	rc, out, _ := clerTRun("--root", root)
	if rc != 1 || !strings.Contains(out, "check-st.py（检出宽松旗标：--strict-mine）") {
		t.Fatalf("rc=%d strict_true 支未检出; out=%q", rc, out)
	}
}

// B 档（登记陈旧）：旗标字面查无 ⇒ rc=1；脚本不在盘另点名。
func TestClerStaleRegistryB(t *testing.T) {
	root := t.TempDir()
	clerTReg(t, root)
	clerTPut(t, root, "check-doc-meta.py", "ap.add_argument('--renamed', default='warn')\n")
	rc, out, _ := clerTRun("--root", root)
	if rc != 1 || !strings.Contains(out, "check-doc-meta.py 的 --missing（旗标字面查无）") {
		t.Fatalf("rc=%d B 陈旧未点名; out=%q", rc, out)
	}
}

// B 档（脚本不在盘）：空 scripts/gates ⇒ **未 Go 化**的登记件全陈旧（rc=1）；
// 已 Go 真搬件（clerGoPorted）退役属预期 ⇒ 不算陈旧。★ 哑化负控：抽掉 clerStale 的
// `if !clerGoPorted[e.name]` 一支（恢复无条件判红）⇒ 本用例第一处 Fatalf 行当场 FAIL。
func TestClerEmptyDirAllStale(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 6 件已 Go 真搬 ⇒ 空目录下不算陈旧 ⇒ rc=0（判据体在 Go 侧）。
	if rc, out, _ := clerTRun("--root", root); rc != 0 {
		t.Fatalf("已 Go 真搬件空目录应 rc=0，得 rc=%d out=%q", rc, out)
	}
	// 牙：点一枚**未 Go 化**的登记件 ⇒ 空目录下仍判「脚本不在盘」rc=1（牙未放宽）。
	saved := clerDeepCopy(clerLooseExit)
	clerLooseExit = append(clerLooseExit, clerEntry{"legacy-not-ported.py", []clerItem{{"--y", "report", "--y rc"}}})
	defer func() { clerLooseExit = saved }()
	rc, out, _ := clerTRun("--root", root)
	if rc != 1 || !strings.Contains(out, "（脚本不在盘）") {
		t.Fatalf("未 Go 化件空目录应 rc=1 陈旧; rc=%d out=%q", rc, out)
	}
}

// 隐式串接：`"--strict-" "concat"` 应成一枚 str 常量 ⇒ 检出 --strict-concat。
func TestClerImplicitStringConcat(t *testing.T) {
	root := t.TempDir()
	clerTReg(t, root)
	clerTPut(t, root, "check-cc.py", "ap.add_argument('--strict-' 'concat', action='store_true')\n")
	rc, out, _ := clerTRun("--root", root)
	if rc != 1 || !strings.Contains(out, "check-cc.py（检出宽松旗标：--strict-concat）") {
		t.Fatalf("rc=%d 串接未检出; out=%q", rc, out)
	}
}

// docstring 里的 add_argument 字面**不是**调用 ⇒ 不检出（found=0 只 ⇒ rc=0）。
func TestClerDocstringNotAScall(t *testing.T) {
	root := t.TempDir()
	clerTReg(t, root)
	clerTPut(t, root, "check-doc.py", "\"\"\"doc: add_argument('--trap', default='warn') 只是字面\"\"\"\n")
	rc, out, errs := clerTRun("--root", root)
	if rc != 0 {
		t.Fatalf("rc=%d want 0（docstring 非调用）; out=%q err=%q", rc, out, errs)
	}
	if !strings.Contains(out, "检出有宽松出口档的脚本 = 6 只") {
		t.Fatalf("docstring 被误计入: %q", out)
	}
}

// 取不到输入（scripts/gates 不存在）⇒ rc=2 · fail-closed（不静默绿）。
func TestClerMissingRootRc2(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nope")
	rc, out, errs := clerTRun("--root", root)
	if rc != 2 || out != "" || !strings.Contains(errs, "scripts/gates 不存在 ⇒ rc=2") {
		t.Fatalf("rc=%d out=%q err=%q", rc, out, errs)
	}
}

// --json 空值三形态：`[]` 与 `{}` **在场**（✗ 不是 null / 缺键）。
func TestClerJSONEmptyCollectionPresent(t *testing.T) {
	root := t.TempDir()
	clerTReg(t, root)
	rc, out, _ := clerTRun("--root", root, "--json")
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	for _, want := range []string{`"unregistered": []`, `"stale": []`, `"detected": {`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON 缺 %s: %q", want, out)
		}
	}
	if strings.Contains(out, "null") {
		t.Fatalf("JSON 不应出现 null: %q", out)
	}
}

// 自检：4 格 PASS ⇒ rc=0；C 档 BLOCKED 入 stderr。
func TestClerSelfTest(t *testing.T) {
	rc, out, errs := clerTRun("--self-test")
	if rc != 0 || !strings.Contains(out, "自检 PASS（4 格）") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	if !strings.Contains(errs, "登记项缺「严格旗标」") {
		t.Fatalf("自检 C 档 stderr 缺 BLOCKED: %q", errs)
	}
}

// argparse 面：-h / 未知旗标 / store_true 显式值。
func TestClerArgparseSurface(t *testing.T) {
	if rc, out, _ := clerTRun("-h"); rc != 0 || !strings.Contains(out, "usage: check-loose-exit-registry.py [-h] [--root ROOT] [--json] [--self-test]") {
		t.Fatalf("-h rc=%d out=%q", rc, out)
	}
	if rc, _, errs := clerTRun("--bogus"); rc != 2 || !strings.Contains(errs, "error: unrecognized arguments: --bogus") {
		t.Fatalf("--bogus rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := clerTRun("--json=1"); rc != 2 || !strings.Contains(errs, "ignored explicit argument '1'") {
		t.Fatalf("--json=1 rc=%d err=%q", rc, errs)
	}
	if rc, _, errs := clerTRun("--root"); rc != 2 || !strings.Contains(errs, "argument --root: expected one argument") {
		t.Fatalf("--root rc=%d err=%q", rc, errs)
	}
}

// 自指面 (b)④：件内 JSON 键名**走拼串**（不落连续原串），而真实输出**仍含连续键名**（两证并存）。
func TestClerSelfReferential(t *testing.T) {
	if got := clerKey("r", "c"); got != "rc" {
		t.Fatalf("clerKey 拼串失效: %q", got)
	}
	if got := clerKey("un", "registered"); got != "unregistered" {
		t.Fatalf("clerKey 拼串失效: %q", got)
	}
	root := t.TempDir()
	clerTReg(t, root)
	_, out, _ := clerTRun("--root", root, "--json")
	if !strings.Contains(out, `"rc": 0`) || !strings.Contains(out, `"detected"`) {
		t.Fatalf("真实输出应含连续键名: %q", out)
	}
}

// 可复跑（无随机/无计时）：同一 argv 连跑两发 ⇒ rc 与两路输出逐字节同。
func TestClerDeterministic(t *testing.T) {
	root := t.TempDir()
	clerTReg(t, root)
	r1, o1, e1 := clerTRun("--root", root, "--json")
	r2, o2, e2 := clerTRun("--root", root, "--json")
	if r1 != r2 || o1 != o2 || e1 != e2 {
		t.Fatalf("两发不一致: rc %d/%d out %q/%q err %q/%q", r1, r2, o1, o2, e1, e2)
	}
}

// TestClerGoPortedAbsentNotStale —— 退役态安全（类②）：已有 Go 真搬半的登记件，py 退役（不在盘）
// ⇒ **不**判「登记陈旧」（rc 仍 0）；未 Go 化的登记件缺席**仍**判红（牙未放宽）。
// ★ 哑化负控：把 check_loose_exit_registry.go 的 `if !clerGoPorted[e.name] { ... }` 一支抽掉
// （恢复无条件判红）⇒ 本用例第二处 Fatalf 行当场 FAIL。
func TestClerGoPortedAbsentNotStale(t *testing.T) {
	root := t.TempDir()
	clerTReg(t, root)
	// 正控：6 件齐 ⇒ rc=0。
	if rc, out, _ := clerTRun("--root", root); rc != 0 {
		t.Fatalf("基线应 rc=0，得 rc=%d out=%q", rc, out)
	}
	// 退役 check-no-symlink-farm.py（其在 clerGoPorted 内）⇒ 仍 rc=0（不算陈旧）。
	if err := os.Remove(filepath.Join(root, "scripts", "gates", "check-no-symlink-farm.py")); err != nil {
		t.Fatal(err)
	}
	if rc, out, errs := clerTRun("--root", root); rc != 0 {
		t.Fatalf("Go 真搬件退役后应 rc=0（非陈旧），得 rc=%d out=%q err=%q", rc, out, errs)
	}
	// 牙：登记表点一个**未 Go 化**的幽灵件 ⇒ 仍 rc=1（判据未被放宽）。
	saved := clerDeepCopy(clerLooseExit)
	clerLooseExit = append(clerLooseExit, clerEntry{"ghost-not-ported.py", []clerItem{{"--x", "report", "--x rc"}}})
	defer func() { clerLooseExit = saved }()
	if rc, out, _ := clerTRun("--root", root); rc != 1 {
		t.Fatalf("未 Go 化幽灵件应仍判红 rc=1，得 rc=%d out=%q", rc, out)
	}
}
