// family_model_pull_test.go —— `zerg model pull`（`cmdModelPull`）的 ssh/rsync 替身自检（缺口 GAP-20261010-14）。
//
// 正控：替身夹具下 `--dry-run` 出计划件（零传输）· `--yes` 走到双侧对拍且 sha 一致 ⇒ rc=0。
// 负控四条（**真红**）：① 缺 `--yes` ⇒ rc=2 只出计划件（stdout 0 字节）·
// ② **源机不在名册 ⇒ 判红并点名** · ③ **源机 == 本机 ⇒ 拒执**（设计稿 Q8 甲）· ④ **对端 sha 不一致 ⇒ 判红**。
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

// mpullStubRsync —— 测试用 `rsync` 替身（**拉取**方向）：把「对端源件内容」写到**最后一个参数**
// （本机落点）；退 ${MPUT_STUB_RSYNC_RC:-0}（非 0 ⇒ 不落件、模拟半途失败）。
const mpullStubRsync = `#!/usr/bin/env bash
rc="${MPUT_STUB_RSYNC_RC:-0}"
if [ "$rc" != "0" ]; then exit "$rc"; fi
eval "dest=\${$#}"
printf '%s' "${MPULL_STUB_DEST_CONTENT:-}" > "$dest"
exit 0
`

// mpullStubInstall 装 ssh/rsync 两枚替身并把目录前置到 PATH（ssh 复用 put 的替身常数）。
func mpullStubInstall(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"ssh": mputStubSSH, "rsync": mpullStubRsync} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// mpullFixture 造一棵临时仓根（含名册件 `gateway/fleet.yaml`）；名册把 <model>@<machine>
// 指向**源机上**的件路径 remoteSrc。返回 root。
func mpullFixture(t *testing.T, model, machine, remoteSrc string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"), []byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "gateway"), 0o755); err != nil {
		t.Fatal(err)
	}
	fy := fmt.Sprintf("models:\n  %s:\n    - { host: %s, backend: llama-server, file: %s, mem_gb: 1, ctx_window: 4096 }\n",
		model, machine, remoteSrc)
	if err := os.WriteFile(filepath.Join(root, "gateway", "fleet.yaml"), []byte(fy), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// mpullRun 进程内直调 `cmdModelPull`（与解析面同形：手工落 args / --to / --dry-run / --yes / --json）。
func mpullRun(t *testing.T, tail ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	inv := &invocation{path: []string{"model", "pull"}}
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
	rc := cmdModelPull(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

// TestModelPullPositive —— 正控：`--dry-run` 出计划件（零传输）· `--yes` 双侧一致 ⇒ rc=0。
func TestModelPullPositive(t *testing.T) {
	mpullStubInstall(t)
	root := mpullFixture(t, "TestPull", "msrc", "/remote/model.gguf")
	t.Setenv("ZERG_REPO", root)
	content := "model-bytes-for-pull"
	sum := sha256.Sum256([]byte(content))
	sha := fmt.Sprintf("%x", sum)
	t.Setenv("MPULL_STUB_DEST_CONTENT", content)
	t.Setenv("MPUT_STUB_SHA", sha)
	to := filepath.Join(t.TempDir(), "downloaded.gguf")
	rc, out, errb := mpullRun(t, "TestPull@msrc", "--to", to, "--dry-run")
	if rc != exitOK {
		t.Fatalf("`--dry-run` rc=%d（要 0）· stderr=%q", rc, errb)
	}
	if !strings.Contains(out, "计划件") {
		t.Errorf("`--dry-run` 应出计划件：%q", out)
	}
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		t.Errorf("`--dry-run` ✗ 不该落件（零副作用），却见 %s", to)
	}
	rc, out, errb = mpullRun(t, "TestPull@msrc", "--to", to, "--yes")
	if rc != exitOK {
		t.Fatalf("`--yes` rc=%d（要 0）· stderr=%q", rc, errb)
	}
	if !strings.Contains(out, sha) {
		t.Errorf("结果面应含双侧一致的 sha：%q", out)
	}
}

// TestModelPullNoYes —— 负控①：缺 `--yes` ⇒ rc=2 只出计划件（stdout 0 字节 · 零传输）。
func TestModelPullNoYes(t *testing.T) {
	mpullStubInstall(t)
	root := mpullFixture(t, "TestPull", "msrc", "/remote/model.gguf")
	t.Setenv("ZERG_REPO", root)
	to := filepath.Join(t.TempDir(), "downloaded.gguf")
	rc, out, errb := mpullRun(t, "TestPull@msrc", "--to", to)
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

// TestModelPullSourceNotInRoster —— 负控②：**源机不在名册 ⇒ 判红并点名**（rc=2 · 点出机名）。
func TestModelPullSourceNotInRoster(t *testing.T) {
	mpullStubInstall(t)
	root := mpullFixture(t, "TestPull", "msrc", "/remote/model.gguf")
	t.Setenv("ZERG_REPO", root)
	rc, out, errb := mpullRun(t, "TestPull@nope-host", "--to", "/tmp/x.gguf", "--yes")
	if rc == exitOK {
		t.Fatal("源机不在名册却退 0 —— 判据无牙")
	}
	if rc != exitUsage {
		t.Fatalf("源机不在名册 rc=%d（要 2）· stderr=%q", rc, errb)
	}
	if !strings.Contains(errb, "nope-host") {
		t.Errorf("必须**点名**不在册的机（负控要求）：stderr=%q", errb)
	}
	if out != "" {
		t.Errorf("判红档 stdout 应 0 字节，得 %q", out)
	}
}

// TestModelPullSelfMachine —— 负控③：**源机 == 本机 ⇒ 拒执**（设计稿 Q8 甲 · rc=2）。
func TestModelPullSelfMachine(t *testing.T) {
	mpullStubInstall(t)
	t.Setenv("ZERG_HOST", "localbox")
	root := mpullFixture(t, "TestPull", "msrc", "/remote/model.gguf")
	t.Setenv("ZERG_REPO", root)
	rc, out, errb := mpullRun(t, "TestPull@localbox", "--to", "/tmp/x.gguf", "--yes")
	if rc != exitUsage {
		t.Fatalf("源机==本机 rc=%d（要 2 拒执）· stderr=%q", rc, errb)
	}
	if out != "" {
		t.Errorf("拒执档 stdout 应 0 字节，得 %q", out)
	}
	if !strings.Contains(errb, "本机") {
		t.Errorf("应逐字报「本机」：stderr=%q", errb)
	}
}

// TestModelPullShaMismatch —— 负控④：源机回读一个**不同**的 sha ⇒ 判红（rc=1 · ✗ 不许报成功）。
func TestModelPullShaMismatch(t *testing.T) {
	mpullStubInstall(t)
	root := mpullFixture(t, "TestPull", "msrc", "/remote/model.gguf")
	t.Setenv("ZERG_REPO", root)
	t.Setenv("MPULL_STUB_DEST_CONTENT", "model-bytes-for-pull")
	t.Setenv("MPUT_STUB_SHA", "deadbeefdeadbeefdeadbeefdeadbeef")
	to := filepath.Join(t.TempDir(), "downloaded.gguf")
	rc, _, errb := mpullRun(t, "TestPull@msrc", "--to", to, "--yes")
	if rc == exitOK {
		t.Fatal("源机 sha 不一致却退 0 —— ✗ 不许报成功（正是要拦的假绿）")
	}
	if rc != exitFail {
		t.Fatalf("源机 sha 不一致 rc=%d（要 1 判红）· stderr=%q", rc, errb)
	}
	if !strings.Contains(errb, "判红") {
		t.Errorf("应逐字报「判红」：stderr=%q", errb)
	}
}

// TestModelPullNoConclusionPeerUnreachable —— 对端不可达（rsync 非 0）⇒ 不给结论（rc=8 · ✗ 别当 0）。
func TestModelPullNoConclusionPeerUnreachable(t *testing.T) {
	mpullStubInstall(t)
	root := mpullFixture(t, "TestPull", "msrc", "/remote/model.gguf")
	t.Setenv("ZERG_REPO", root)
	t.Setenv("MPUT_STUB_SHA", "deadbeef")
	t.Setenv("MPUT_STUB_RSYNC_RC", "255")
	to := filepath.Join(t.TempDir(), "downloaded.gguf")
	rc, _, errb := mpullRun(t, "TestPull@msrc", "--to", to, "--yes")
	if rc == exitOK {
		t.Fatal("对端不可达却退 0 —— 「不给结论不许当 0」被破")
	}
	if rc != exitBlocked {
		t.Fatalf("对端不可达 rc=%d（要 8 不给结论）· stderr=%q", rc, errb)
	}
}

// TestModelPullNoConclusionReadbackFails —— 源机回读起不来（ssh 非 0）⇒ 不给结论（rc=8）。
func TestModelPullNoConclusionReadbackFails(t *testing.T) {
	mpullStubInstall(t)
	root := mpullFixture(t, "TestPull", "msrc", "/remote/model.gguf")
	t.Setenv("ZERG_REPO", root)
	t.Setenv("MPUT_STUB_SHA_RC", "255")
	to := filepath.Join(t.TempDir(), "downloaded.gguf")
	rc, _, errb := mpullRun(t, "TestPull@msrc", "--to", to, "--yes")
	if rc == exitOK {
		t.Fatal("回读起不来却退 0")
	}
	if rc != exitBlocked {
		t.Fatalf("回读起不来 rc=%d（要 8 不给结论）· stderr=%q", rc, errb)
	}
}
