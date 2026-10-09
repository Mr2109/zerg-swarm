// cli_agent_workers_test.go —— P0 #232 第四/第五面（`agent workers set` / `agent scheduler set`）的
// **四态真跑**（判据⑥ 正控加强 · 照同族先例 `adopt_retire_test.go` 的 `TestAdoptRetireItaskIntervalSetFourStates`）。
//
// 从**当前源码**现编 CLI（`zergBinaryFromSource` · ✗ 不碰 `bin/`），对**合成仓根**（`ZERG_REPO` 指
// 一个只含 `core/internal/version/version.go` + `config/workers.yaml` 的临时目录）逐态真跑：
//
//	① 裸跑（缺确认档）        ⇒ rc=2 · stdout 0 字节（**零写**：确认档前不落任何状态）；
//	② `--dry-run`             ⇒ rc=0 · 出计划件（零副作用）；
//	③ `--confirm=<本机名>` + `--yes` ⇒ rc=0 · 合成 config 真写 · **落地值回读**报出刚写的值；
//	④ 远端机（`<机> ≠ 本机`）  ⇒ rc=2 · **明确拒绝语** · 零写；坏值 ⇒ rc=2 · 零写。
//
// ★ 不碰真仓 `config/workers.yaml`、不碰真状态目录（一切读写都在临时合成仓根里）。
package main_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// awTempRepoRoot —— 合成仓根：`core/internal/version/version.go`（仓根判据件）+ `config/workers.yaml`。
func awTempRepoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "internal", "version", "version.go"),
		[]byte("package version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "scheduler:\n  poll_interval: 5s\n  max_retries: 3\n  cooldown: 30s\ndevices:\n" +
		"  - hostname: x" + "3\n    max_workers: 3\n"
	if err := os.WriteFile(filepath.Join(root, "config", "workers.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAgentWorkersAndSchedulerSetFourStates(t *testing.T) {
	bin := zergBinaryFromSource(t)
	root := awTempRepoRoot(t)
	hn, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname: %v", err)
	}
	confirm := hn
	if i := strings.IndexByte(hn, '.'); i > 0 {
		confirm = hn[:i] // 短名（同族 devEditHostAliases 也收）
	}

	run := func(argv ...string) (int, string, string) {
		cmd := exec.Command(bin, argv...)
		cmd.Env = append(os.Environ(), "ZERG_REPO="+root)
		cmd.Stdin = strings.NewReader("")
		var out, errb strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &errb
		rc := 0
		if err := cmd.Run(); err != nil {
			var ee *exec.ExitError
			if !errors.As(err, &ee) {
				t.Fatalf("起不了现编二进制：%v", err)
			}
			rc = ee.ExitCode()
		}
		return rc, out.String(), errb.String()
	}

	cfgPath := filepath.Join(root, "config", "workers.yaml")
	read := func() string {
		b, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("读合成 config: %v", err)
		}
		return string(b)
	}
	before := read()

	// ---- (a) `agent workers set` ----
	if rc, out, _ := run("agent", "workers", "set", confirm, "--max", "2"); rc != 2 || out != "" {
		t.Errorf("(a)① 裸跑：rc=%d（要 2）· stdout=%q（要空 —— 零写）", rc, out)
	}
	if rc, out, _ := run("agent", "workers", "set", confirm, "--max", "2", "--dry-run"); rc != 0 || !strings.Contains(out, "计划件") {
		t.Errorf("(a)② --dry-run：rc=%d（要 0）· stdout 里没计划件：%q", rc, out)
	}
	if read() != before {
		t.Errorf("(a)①/② 之后合成 config 被改了（要零写）")
	}
	if rc, out, _ := run("agent", "workers", "set", "remote-box-zz", "--max", "2"); rc != 2 || out != "" {
		t.Errorf("(a)④ 远端机：rc=%d（要 2 拒绝）· stdout=%q（要空 —— 零写）", rc, out)
	} else if read() != before {
		t.Errorf("(a)④ 远端机拒绝后合成 config 被改了（要零写）")
	}
	if rc, out, _ := run("agent", "workers", "set", confirm, "--max", "abc"); rc != 2 || out != "" {
		t.Errorf("(a)④b 坏值：rc=%d（要 2）· stdout=%q（要空 —— 零写）", rc, out)
	}
	rc, out, errb := run("agent", "workers", "set", confirm, "--max", "2", "--confirm="+confirm, "--yes")
	if rc != 0 {
		t.Errorf("(a)③ --yes：rc=%d（要 0）· stderr=%s", rc, errb)
	}
	if !strings.Contains(read(), "max_workers: 2") || !strings.Contains(out, "2") {
		t.Errorf("(a)③ 真写/回读失败：config=%q · stdout=%q", read(), out)
	}

	// ---- (b) `agent scheduler set` ----
	before = read()
	if rc, out, _ := run("agent", "scheduler", "set", "--poll-interval", "7s"); rc != 2 || out != "" {
		t.Errorf("(b)① 裸跑：rc=%d（要 2）· stdout=%q（要空 —— 零写）", rc, out)
	}
	if rc, out, _ := run("agent", "scheduler", "set", "--poll-interval", "7s", "--max-retries", "5", "--cooldown", "45s", "--dry-run"); rc != 0 || !strings.Contains(out, "计划件") {
		t.Errorf("(b)② --dry-run：rc=%d（要 0）· stdout 里没计划件：%q", rc, out)
	}
	if read() != before {
		t.Errorf("(b)①/② 之后合成 config 被改了（要零写）")
	}
	if rc, out, _ := run("agent", "scheduler", "set", "--max-retries", "abc"); rc != 2 || out != "" {
		t.Errorf("(b)④ 坏值：rc=%d（要 2）· stdout=%q（要空 —— 零写）", rc, out)
	}
	rc, out, errb = run("agent", "scheduler", "set", "--poll-interval", "7s", "--max-retries", "5", "--cooldown", "45s", "--confirm="+confirm, "--yes")
	if rc != 0 {
		t.Errorf("(b)③ --yes：rc=%d（要 0）· stderr=%s", rc, errb)
	}
	got := read()
	if !strings.Contains(got, "poll_interval: 7s") || !strings.Contains(got, "max_retries: 5") || !strings.Contains(got, "cooldown: 45s") {
		t.Errorf("(b)③ 真写失败：config=%q", got)
	}
	if !strings.Contains(out, "7s") || !strings.Contains(out, "45s") {
		t.Errorf("(b)③ 落地值回读没报出刚写的值：stdout=%q", out)
	}
}
