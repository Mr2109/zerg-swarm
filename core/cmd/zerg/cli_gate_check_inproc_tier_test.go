// cli_gate_check_inproc_tier_test.go —— 缺口 `GAP-20261001-70`：`zerg gate check` 的 inproc
// 分派点**按件分档**（属性 = 判据体本体引用 `os/exec`）的成对判据。
//
// 三面：
//
//	① 属性判据 `gateCheckJudgeBodySpawns`：spawn 件 ⇒ true / 快返件 ⇒ false /
//	   属性源读不到 ⇒ **fail-closed true** / 非 inproc 键 ⇒ false。
//	② 分档决策 `gateCheckShouldReexecSelf`：真进程（非 `*.test`）＋ spawn ⇒ true；
//	   防递归标已置 ⇒ false；`go test` 二进制（`*.test`）⇒ false。
//	③ 属性源定位 `gateCheckHandlerFile`：「handler 名 → 件」回落（✗ 不靠按名名单）。
//
// ★ 负控：哑化属性判据（overlay 把 `gateCheckJudgeBodySpawns` 改成恒返 false）⇒ ①② 的
// spawn 格 FAIL（`go test -overlay` 两态对拍 · 见回执）。
package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// g70InjectKey 往 inproc 表注入一枚合成键（跑完删回 · ✗ 不碰 gate_inproc_table.go）。
func g70InjectKey(t *testing.T, key string) {
	t.Helper()
	if _, hit := gateCheckGoEntry[key]; hit {
		t.Fatalf("前置不成立：%q 竟已在 Go 表里", key)
	}
	gateCheckGoEntry[key] = func(stdout, stderr io.Writer, argv []string) int { return 0 }
	t.Cleanup(func() { delete(gateCheckGoEntry, key) })
}

// g70Root 造一个含属性源目录 `<root>/core/cmd/zerg/` 的合成仓根。
func g70Root(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// ① 属性判据三态 ＋ fail-closed ＋ 非 inproc。
func TestGateCheckJudgeBodySpawnsProperty(t *testing.T) {
	root := g70Root(t, map[string]string{
		"spawn_probe.go": "package main\n\nimport \"os/exec\"\n\nvar _ = exec.Command\n",
		"fast_probe.go":  "package main\n\nvar fastProbe = 1\n",
	})
	g70InjectKey(t, "spawn-probe")
	g70InjectKey(t, "fast-probe")
	g70InjectKey(t, "missing-probe") // 属性源不在盘

	if !gateCheckJudgeBodySpawns(root, "spawn-probe") {
		t.Fatalf("spawn 件（属性源含 os/exec）应判 spawn=true")
	}
	if gateCheckJudgeBodySpawns(root, "fast-probe") {
		t.Fatalf("快返件（属性源不含 os/exec）应判 spawn=false")
	}
	if !gateCheckJudgeBodySpawns(root, "missing-probe") {
		t.Fatalf("属性源读不到 ⇒ fail-closed 判 spawn=true")
	}
	if gateCheckJudgeBodySpawns(root, "not-in-inproc-table") {
		t.Fatalf("非 inproc 键（走 exec 径）应判 false")
	}
}

// ② 分档决策：真进程 spawn ⇒ true；标已置 ⇒ false；`*.test` ⇒ false；快返件 ⇒ false。
func TestGateCheckShouldReexecSelfDecision(t *testing.T) {
	root := g70Root(t, map[string]string{
		"spawn_probe.go": "package main\n\nimport \"os/exec\"\n\nvar _ = exec.Command\n",
		"fast_probe.go":  "package main\n\nvar fastProbe = 1\n",
	})
	g70InjectKey(t, "spawn-probe")
	g70InjectKey(t, "fast-probe")

	if !gateCheckShouldReexecSelf(root, "spawn-probe", "/usr/local/bin/zerg", false) {
		t.Fatalf("真进程（非 *.test）＋ spawn ⇒ 应 re-exec（true）")
	}
	if gateCheckShouldReexecSelf(root, "fast-probe", "/usr/local/bin/zerg", false) {
		t.Fatalf("真进程 ＋ 快返件 ⇒ 应留 inproc（false）")
	}
	if gateCheckShouldReexecSelf(root, "spawn-probe", "/usr/local/bin/zerg", true) {
		t.Fatalf("防递归标已置 ⇒ 应 false（✗ 不无限自我重入）")
	}
	if gateCheckShouldReexecSelf(root, "spawn-probe", "/tmp/zerg.test", false) {
		t.Fatalf("`go test` 进程（*.test）不可 re-exec 自身 ⇒ 应 false")
	}
	// 防递归标读写同源：置标 ⇒ 现读命中，撤标 ⇒ 不命中。
	os.Setenv(gateCheckInprocChildEnvMark, "1")
	if !gateCheckInprocChildMarked() {
		t.Fatalf("防递归标置 1 后应命中")
	}
	os.Unsetenv(gateCheckInprocChildEnvMark)
	if gateCheckInprocChildMarked() {
		t.Fatalf("防递归标撤后应不命中")
	}
}

// ③ 「handler 名 → 件」回落：具名入口 ⇒ 找到定义件；闭包（未具名）⇒ 取不到（fail-closed 面）。
func TestGateCheckHandlerFileFallback(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "core", "cmd", "zerg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "package main\n\nimport \"io\"\n\nfunc retireRosterGateMain(stdout, stderr io.Writer, argv []string) int { return 0 }\n"
	if err := os.WriteFile(filepath.Join(dir, "retire_roster.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// 真表里 `check-retire-roster` 走「键 → 件」direct 不中（件名 `check_retire_roster.go` 不在盘）
	// ⇒ 回落 handler 名 → 件。本格只测回落本身。
	got := gateCheckHandlerFile(dir, gateCheckGoEntry["check-retire-roster"])
	if got == "" {
		t.Fatalf("具名入口（retireRosterGateMain）应能经 handler 名定位到件")
	}
	if !strings.HasSuffix(got, "retire_roster.go") {
		t.Fatalf("handler 名 → 件 应命中 retire_roster.go，得到 %q", got)
	}
	cl := func(stdout, stderr io.Writer, argv []string) int { return 0 }
	if p := gateCheckHandlerFile(dir, cl); p != "" {
		t.Fatalf("闭包（未具名）应取不到件，得到 %q", p)
	}
}
