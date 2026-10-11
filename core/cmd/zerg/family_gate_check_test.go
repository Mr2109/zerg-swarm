// cli_gate_keep_cwd_test.go —— 缺口 `GAP-20261008-10`：`zerg gate check` 的壳级布尔旗标
// `--keep-cwd`（保留调用方 cwd）的**成对判据**（正控 + 负控），走**真 gateCheck 入口**（✗ 不另抄一份逻辑）。
//
// 三面：
//
//	① exec 档（.sh 判据件）：不给 `--keep-cwd` ⇒ 子件 cwd = 仓根（**现行为逐字不变**）；
//	                      给了 ⇒ 子件 cwd = 调用方 cwd。
//	② inproc 档（进程内 Go 判据体）：同上两态（chdir 目标随之变）。
//	③ 危险档拒：写工具 / 重编类 / 整套门类 ＋ `--keep-cwd` ⇒ 退 2（只收紧、不放宽 · exec 之前拒）。
//
// 判据件自带：把**自身 cwd**（`pwd -P` / `os.Getwd`）写进 `$GAP10_OUT` 指的件里，测试读回逐字比对。
// ★ 负控：把 `family_gate_check.go` 换回**改前态**（无 `--keep-cwd` 分支）⇒ ①②③ 全红（正控那一格仍绿）。
package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gap10EchoScript —— 判据件正文：写自身物理 cwd 到 $GAP10_OUT。
const gap10EchoScript = "#!/usr/bin/env bash\npwd -P > \"$GAP10_OUT\"\n"

// gap10MakeRoot —— 造一个最小合成仓根：`<root>/scripts/gates/gap10-echo-cwd.sh` 在盘。
func gap10MakeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "scripts", "gates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gap10-echo-cwd.sh"), []byte(gap10EchoScript), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// gap10RunFrom —— 在 callerDir 里跑一次 gateCheck（orig = 完整 argv），读回子件写下的 cwd。
func gap10RunFrom(t *testing.T, root, callerDir string, orig []string) (string, int) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(callerDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()

	out := filepath.Join(t.TempDir(), "cwd.txt")
	t.Setenv("GAP10_OUT", out)

	inv := &invocation{orig: orig}
	rc := gateCheck(inv, io.Discard, io.Discard, root)

	b, rerr := os.ReadFile(out)
	if rerr != nil {
		return "", rc
	}
	return strings.TrimSpace(string(b)), rc
}

// resolveSym —— t.TempDir() 在 macOS 上可能是 /var ⇒ /private/var，比较前先现读消歧。
func resolveSym(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", p, err)
	}
	return r
}

// ① exec 档两态：缺省 = 仓根（**现行为逐字不变**）· 给了 = 调用方 cwd。
func TestGateCheckKeepCwdExec(t *testing.T) {
	root := gap10MakeRoot(t)
	caller := filepath.Join(root, "scripts", "gates") // 调用方 cwd ≠ 仓根

	// 缺省档（正控）：子件 cwd 必落在**仓根**（与改前逐字同）。
	got, rc := gap10RunFrom(t, root, caller, []string{"gate", "check", "gap10-echo-cwd"})
	if rc != 0 {
		t.Fatalf("缺省档 rc=%d want 0", rc)
	}
	if got != resolveSym(t, root) {
		t.Fatalf("缺省档子件 cwd=%q want 仓根=%q（现行为必须逐字不变）", got, resolveSym(t, root))
	}

	// --keep-cwd（负控面）：子件 cwd 必落在**调用方 cwd**。
	got, rc = gap10RunFrom(t, root, caller, []string{"gate", "check", "gap10-echo-cwd", "--keep-cwd"})
	if rc != 0 {
		t.Fatalf("--keep-cwd 档 rc=%d want 0", rc)
	}
	if got != resolveSym(t, caller) {
		t.Fatalf("--keep-cwd 档子件 cwd=%q want 调用方 cwd=%q", got, resolveSym(t, caller))
	}
}

// ② inproc 档两态：同一枚旗标对进程内判据体的 chdir 目标同样生效。
func TestGateCheckKeepCwdInproc(t *testing.T) {
	const key = "gap10-inproc-cwd"
	fn := func(stdout, stderr io.Writer, argv []string) int {
		wd, err := os.Getwd()
		if err != nil {
			return 2
		}
		if err := os.WriteFile(os.Getenv("GAP10_OUT"), []byte(wd+"\n"), 0o644); err != nil {
			return 2
		}
		return 0
	}
	gateCheckGoEntry[key] = fn
	defer func() { delete(gateCheckGoEntry, key) }()

	root := gap10MakeRoot(t)
	caller := filepath.Join(root, "scripts", "gates")

	got, rc := gap10RunFrom(t, root, caller, []string{"gate", "check", key})
	if rc != 0 {
		t.Fatalf("inproc 缺省档 rc=%d want 0", rc)
	}
	if got != resolveSym(t, root) {
		t.Fatalf("inproc 缺省档 cwd=%q want 仓根=%q", got, resolveSym(t, root))
	}

	got, rc = gap10RunFrom(t, root, caller, []string{"gate", "check", key, "--keep-cwd"})
	if rc != 0 {
		t.Fatalf("inproc --keep-cwd 档 rc=%d want 0", rc)
	}
	if got != resolveSym(t, caller) {
		t.Fatalf("inproc --keep-cwd 档 cwd=%q want 调用方 cwd=%q", got, resolveSym(t, caller))
	}
}

// ③ 危险档拒 `--keep-cwd`（只收紧）：该件登进中央分类表（write/D2）⇒ 给 `--keep-cwd` ⇒ 退 2、**exec 之前**拒。
//
//	配一枚真在盘、退 0 的判据件 ⇒ 改前会真跑到 rc=0（红）；改后拒不跑（绿）—— 本格有牙的负控面。
func TestGateCheckKeepCwdForbiddenForWriteClass(t *testing.T) {
	const key = "gap10-write-cwd"
	root := gap10MakeRoot(t)
	marker := filepath.Join(t.TempDir(), "ran.txt")
	py := "#!/usr/bin/env python3\nimport os, sys\nopen(os.environ['GAP10_RAN'], 'w').write('1')\nsys.exit(0)\n"
	if err := os.WriteFile(filepath.Join(root, "scripts", "gates", key+".py"), []byte(py), 0o755); err != nil {
		t.Fatal(err)
	}
	old, had := gateCheckClassEntry[key]
	gateCheckClassEntry[key] = gateCheckClassSpec{"write", dangerD2}
	defer func() {
		if had {
			gateCheckClassEntry[key] = old
		} else {
			delete(gateCheckClassEntry, key)
		}
	}()

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(root, "scripts", "gates")); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldwd) }()
	t.Setenv("GAP10_RAN", marker)
	t.Setenv("GAP10_OUT", filepath.Join(t.TempDir(), "cwd.txt"))

	inv := &invocation{orig: []string{"gate", "check", key, "--keep-cwd", "--yes"}, yes: true}
	rc := gateCheck(inv, io.Discard, io.Discard, root)
	if rc != exitUsage {
		t.Fatalf("写工具 + --keep-cwd rc=%d want %d（危险档拒 · 只收紧）", rc, exitUsage)
	}
	if _, serr := os.Stat(marker); serr == nil {
		t.Fatalf("危险档 + --keep-cwd **不该 exec**，但判据件跑了（marker 在盘）")
	}
}
