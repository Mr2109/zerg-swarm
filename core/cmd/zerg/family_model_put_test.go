// family_model_put_test.go —— `zerg model put`（`cmdModelPut`）的 ssh 替身自检（缺口 GAP-20260925-25）。
//
// 正控：替身夹具下 `--dry-run` 出计划件（零传输）· `--yes` 走到双侧对拍且 sha 一致 ⇒ rc=0。
// 负控三条（**真红**）：① 缺 `--yes` ⇒ rc=2 只出计划件（stdout 0 字节）·
// ② **目标机不在名册 ⇒ 判红并点名** · ③ **对端 sha 不一致 ⇒ 判红**（✗ 不许报成功）。
// 另两条不给结论档：对端不可达（rsync 非 0）/ 回读起不来（ssh 非 0）⇒ rc=8（✗ 别当 0）。
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mputStubSSH —— 测试用 `ssh` 替身：只看回读那条命令（含 `sha256sum`）。
const mputStubSSH = `#!/usr/bin/env bash
case "$*" in
  *sha256sum*)
    printf '%s' "${MPUT_STUB_SHA:-}"
    exit "${MPUT_STUB_SHA_RC:-0}"
    ;;
esac
exit 0
`

// mputStubRsync —— 测试用 `rsync` 替身：退 `${MPUT_STUB_RSYNC_RC}`（默认 0 = 传输成功）。
const mputStubRsync = `#!/usr/bin/env bash
exit "${MPUT_STUB_RSYNC_RC:-0}"
`

// mputStubInstall 装 ssh/rsync 两枚替身并把目录前置到 PATH（Go 的 `exec.Command` 解析到它们）。
func mputStubInstall(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"ssh": mputStubSSH, "rsync": mputStubRsync} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// mputFixture 造一棵临时仓根（含名册件 `gateway/fleet.yaml`）+ 一个本机源件；
// 返回 (root, src, srcSha)。名册把 <model>@<machine> 指向 src。
func mputFixture(t *testing.T, model, machine string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"), []byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	content := []byte("model-bytes-for-put")
	src := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "gateway"), 0o755); err != nil {
		t.Fatal(err)
	}
	fy := fmt.Sprintf("models:\n  %s:\n    - { host: %s, backend: llama-server, file: %s, mem_gb: 1, ctx_window: 4096 }\n",
		model, machine, src)
	if err := os.WriteFile(filepath.Join(root, "gateway", "fleet.yaml"), []byte(fy), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	return root, src, fmt.Sprintf("%x", sum)
}

// mputRun 进程内直调 `cmdModelPut`（与解析面同形：手工落 args / --to / --dry-run / --yes / --json）。
func mputRun(t *testing.T, tail ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	inv := &invocation{path: []string{"model", "put"}}
	for i := 0; i < len(tail); i++ {
		switch a := tail[i]; a {
		case "--to":
			if i+1 < len(tail) {
				i++
				inv.to = tail[i]
			}
		case "--dry-run":
			inv.dryRun = true
		case "--yes":
			inv.yes = true
		case "--json":
			inv.jsonGiven = true
			if i+1 < len(tail) && !strings.HasPrefix(tail[i+1], "-") {
				i++
				inv.fields = strings.Split(tail[i], ",")
			}
		default:
			inv.args = append(inv.args, a)
		}
	}
	rc := cmdModelPut(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// TestModelPutPositive —— 正控：`--dry-run` 出计划件（零传输）· `--yes` 双侧一致 ⇒ rc=0。
func TestModelPutPositive(t *testing.T) {
	mputStubInstall(t)
	root, src, sum := mputFixture(t, "TestPut", "mtest")
	t.Setenv("ZERG_REPO", root)
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("夹具源件不在：%v", err)
	}
	rc, out, errb := mputRun(t, "TestPut@mtest", "--to", "/remote/x.gguf", "--dry-run")
	if rc != exitOK {
		t.Fatalf("`--dry-run` rc=%d（要 0）· stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "计划件") {
		t.Errorf("`--dry-run` 应出计划件：%q", out)
	}
	t.Setenv("MPUT_STUB_SHA", sum)
	rc, out, errb = mputRun(t, "TestPut@mtest", "--to", "/remote/x.gguf", "--yes")
	if rc != exitOK {
		t.Fatalf("`--yes` rc=%d（要 0）· stderr=%q", rc, errb)
	}
	if !strings.Contains(out, sum) {
		t.Errorf("结果面应含双侧一致的 sha：%q", out)
	}
}

// TestModelPutGitBashNoYes —— 负控①：缺 `--yes` ⇒ rc=2 只出计划件（stdout 0 字节 · 零传输）。
func TestModelPutGitBashNoYes(t *testing.T) {
	mputStubInstall(t)
	root, _, _ := mputFixture(t, "TestPut", "mtest")
	t.Setenv("ZERG_REPO", root)
	rc, out, errb := mputRun(t, "TestPut@mtest", "--to", "/remote/x.gguf")
	if rc != exitUsage {
		t.Fatalf("缺 `--yes` rc=%d（要 2 · fail-closed）· stderr=%q", rc, errb)
	}
	if out != "" {
		t.Errorf("缺 `--yes` ⇒ stdout 应 0 字节（计划件走 stderr），得 %q", out)
	}
	if !strings.Contains(errb, "计划件") {
		t.Errorf("缺 `--yes` 应把计划件打到 stderr：%q", errb)
	}
}

// TestModelPutMachineNotInRoster —— 负控②：**目标机不在名册 ⇒ 判红并点名**（rc=2 · 点出机名）。
func TestModelPutMachineNotInRoster(t *testing.T) {
	mputStubInstall(t)
	root, _, _ := mputFixture(t, "TestPut", "mtest")
	t.Setenv("ZERG_REPO", root)
	rc, out, errb := mputRun(t, "TestPut@nope-host", "--to", "/remote/x.gguf", "--yes")
	if rc == exitOK {
		t.Fatal("目标机不在名册却退 0 —— 判据无牙")
	}
	if rc != exitUsage {
		t.Fatalf("目标机不在名册 rc=%d（要 2）· stderr=%q", rc, errb)
	}
	if !strings.Contains(errb, "nope-host") {
		t.Errorf("必须**点名**不在册的机（负控要求）：stderr=%q", errb)
	}
	if out != "" {
		t.Errorf("判红档 stdout 应 0 字节，得 %q", out)
	}
}

// TestModelPutShaMismatch —— 负控③：对端回读一个**不同**的 sha ⇒ 判红（rc=1 · ✗ 不许报成功）。
func TestModelPutShaMismatch(t *testing.T) {
	mputStubInstall(t)
	root, _, _ := mputFixture(t, "TestPut", "mtest")
	t.Setenv("ZERG_REPO", root)
	t.Setenv("MPUT_STUB_SHA", "deadbeefdeadbeefdeadbeefdeadbeef")
	rc, _, errb := mputRun(t, "TestPut@mtest", "--to", "/remote/x.gguf", "--yes")
	if rc == exitOK {
		t.Fatal("对端 sha 不一致却退 0 —— ✗ 不许报成功（正是要拦的假绿）")
	}
	if rc != exitFail {
		t.Fatalf("对端 sha 不一致 rc=%d（要 1 判红）· stderr=%q", rc, errb)
	}
	if !strings.Contains(errb, "判红") {
		t.Errorf("应逐字报「判红」：stderr=%q", errb)
	}
}

// TestModelPutNoConclusionPeerUnreachable —— 对端不可达（rsync 非 0）⇒ 不给结论（rc=8 · ✗ 别当 0）。
func TestModelPutNoConclusionPeerUnreachable(t *testing.T) {
	mputStubInstall(t)
	root, _, _ := mputFixture(t, "TestPut", "mtest")
	t.Setenv("ZERG_REPO", root)
	t.Setenv("MPUT_STUB_RSYNC_RC", "255")
	rc, _, errb := mputRun(t, "TestPut@mtest", "--to", "/remote/x.gguf", "--yes")
	if rc == exitOK {
		t.Fatal("对端不可达却退 0 —— 「不给结论不许当 0」被破")
	}
	if rc != exitBlocked {
		t.Fatalf("对端不可达 rc=%d（要 8 不给结论）· stderr=%q", rc, errb)
	}
}

// TestModelPutNoConclusionReadbackFails —— 回读命令跑不起来（ssh 非 0）⇒ 不给结论（rc=8）。
func TestModelPutNoConclusionReadbackFails(t *testing.T) {
	mputStubInstall(t)
	root, _, _ := mputFixture(t, "TestPut", "mtest")
	t.Setenv("ZERG_REPO", root)
	t.Setenv("MPUT_STUB_SHA_RC", "255")
	rc, _, errb := mputRun(t, "TestPut@mtest", "--to", "/remote/x.gguf", "--yes")
	if rc == exitOK {
		t.Fatal("回读起不来却退 0")
	}
	if rc != exitBlocked {
		t.Fatalf("回读起不来 rc=%d（要 8 不给结论）· stderr=%q", rc, errb)
	}
}
