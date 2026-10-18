package main

// zerg_egg_info_file_test.go —— `GAP-20261019-04` 的成对判据（两态 · 哑化任一 ⇒ 本件必 FAIL）：
//
//	① **文件态**：`egg info <盘上真 .egg>` ⇒ 走**文件读面**，且读数与 `cocoon info <同一件>`
//	   **逐字节同**（证「同一实现」＝ 同一个 `zegCmdInfo`，而✗ 不是复制出来的第二套）；
//	② **名册态**：`egg info <册内名>` ⇒ 仍走卵库名册（零回归）；册内没有 ⇒ 仍 rc=8 且报话点名名册。
//
// 造卵一律走真打包面 `zergEggGateMain(["pack", 源目录, 目标.egg])`（✗ 手搓字节 —— 手搓的件读端未必认）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr2109/zerg-swarm/core/internal/store/eggstore"
)

// zegInfoTestPackEgg —— 用真打包面造一枚 .egg，返回其绝对路径。
func zegInfoTestPackEgg(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "payload")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("建源目录：%v", err)
	}
	if err := os.WriteFile(filepath.Join(src, "w.bin"), []byte("hello egg payload"), 0o644); err != nil {
		t.Fatalf("写源件：%v", err)
	}
	dst := filepath.Join(dir, "one.egg")
	var o, e strings.Builder
	if rc := zergEggGateMain(&o, &e, []string{"pack", src, dst}); rc != exitOK {
		t.Fatalf("真打包面失败 rc=%d\nstdout:\n%s\nstderr:\n%s", rc, o.String(), e.String())
	}
	return dst
}

// zegInfoTestInv —— 造一条 `egg info` 的调用面（flags 逐字进 kv）。
func zegInfoTestInv(args []string, kv map[string][]string) *invocation {
	orig := append([]string{"egg", "info"}, args...)
	return &invocation{path: []string{"egg", "info"}, args: args, orig: orig, kv: kv}
}

// ① 文件态：盘上真 .egg ⇒ 出读数，且与 `cocoon info` 逐字节同（同一实现的行为面证据）。
func TestEggInfoFileArgRoutesToFileFace(t *testing.T) {
	dir := t.TempDir()
	egg := zegInfoTestPackEgg(t, dir)

	// 名册根故意指到一个**空目录**：文件态这条路根本不该碰它（否则会掉进「名册里没有」）。
	emptyRoot := filepath.Join(dir, "empty-store")
	if err := os.MkdirAll(emptyRoot, 0o755); err != nil {
		t.Fatalf("建空库根：%v", err)
	}
	inv := zegInfoTestInv([]string{egg}, map[string][]string{"--root": {emptyRoot}})
	var out, errb strings.Builder
	rc := cmdEggInfo(inv, &out, &errb)
	if rc != exitOK {
		t.Fatalf("`egg info <真 .egg>` rc=%d 期望 %d（仍未走文件面）\nstdout:\n%s\nstderr:\n%s",
			rc, exitOK, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "卵文件："+egg) {
		t.Fatalf("人面读数缺「卵文件：<路径>」：\n%s", out.String())
	}
	if !strings.Contains(errb.String(), "按文件读") {
		t.Fatalf("没有出声说明「按文件读」（STDERR）：\n%s", errb.String())
	}
	if strings.Contains(errb.String(), "名册里没有") {
		t.Fatalf("文件态竟掉了名册报话（路由没生效）：\n%s", errb.String())
	}

	// ★ 同一实现的行为面证据：`cocoon info` 走 cmdCocoonEgg → zergEggGateMain → zegCmdInfo，
	// 本命令走 zegCmdInfo ⇒ 两条命令对**同一件**的读数必须逐字节一致。
	var cOut, cErr strings.Builder
	cInv := &invocation{path: []string{"cocoon", "info"}, orig: []string{"cocoon", "info", egg}}
	if crc := cmdCocoonEgg(cInv, &cOut, &cErr); crc != exitOK {
		t.Fatalf("`cocoon info` rc=%d（对照面本身坏了）\n%s\n%s", crc, cOut.String(), cErr.String())
	}
	if out.String() != cOut.String() {
		t.Fatalf("`egg info <文件>` 与 `cocoon info <同一件>` 读数不一致 ⇒ 不是同一实现\n--- egg info ---\n%s\n--- cocoon info ---\n%s",
			out.String(), cOut.String())
	}
}

// ①b 文件态 ＋ `--json`：文件读面没有机器面 ⇒ 用法错（rc=2 · 具名引导），✗ 不许静默吞旗标。
func TestEggInfoFileArgWithJSONIsRefused(t *testing.T) {
	dir := t.TempDir()
	egg := zegInfoTestPackEgg(t, dir)
	inv := zegInfoTestInv([]string{egg}, map[string][]string{})
	inv.jsonGiven = true
	inv.fields = []string{"path"}
	var out, errb strings.Builder
	if rc := cmdEggInfo(inv, &out, &errb); rc != exitUsage {
		t.Fatalf("`egg info <文件> --json` rc=%d 期望 %d（应具名拒绝而非静默忽略）\n%s\n%s",
			rc, exitUsage, out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "没有 `--json` 机器面") || !strings.Contains(errb.String(), "egg verify") {
		t.Fatalf("拒执报话缺具名引导（`--json` 机器面去处）：\n%s", errb.String())
	}
}

// ② 名册态（零回归）：册内名 ⇒ 仍走名册行；册内没有 ⇒ 仍 rc=8 且报话点名名册。
func TestEggInfoRosterNameStillRoster(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "store")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("建库根：%v", err)
	}
	// 名册夹具：一枚 verified 卵（形状须过 Egg.Validate：name/sha256/state 必填）。
	row := `[{"name":"alpha","sha256":"` + strings.Repeat("a", 64) + `","path":"/nonexistent/alpha.egg",` +
		`"state":"verified","bytes":4096,"refs":2,"complete":true,"blocks":1,` +
		`"added_at":"2026-10-19T00:00:00Z","updated_at":"2026-10-19T00:00:00Z"}]`
	if err := os.WriteFile(filepath.Join(root, eggstore.RosterFileName), []byte(row), 0o644); err != nil {
		t.Fatalf("写名册夹具：%v", err)
	}

	inv := zegInfoTestInv([]string{"alpha"}, map[string][]string{"--root": {root}})
	var out, errb strings.Builder
	if rc := cmdEggInfo(inv, &out, &errb); rc != exitOK {
		t.Fatalf("`egg info alpha`（册内）rc=%d 期望 %d\nstdout:\n%s\nstderr:\n%s", rc, exitOK, out.String(), errb.String())
	}
	for _, want := range []string{"卵        : alpha", "引用 / 块 : 0 / 1", "判词      : ok"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("名册行读数丢了 %q：\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "卵文件：") {
		t.Fatalf("名册态竟走了文件面（回归）：\n%s", out.String())
	}

	// 册内没有 ⇒ 仍退 8，报话仍点名名册（这两条是既有契约，✗ 不许漂）。
	inv2 := zegInfoTestInv([]string{"nosuch"}, map[string][]string{"--root": {root}})
	var out2, err2 strings.Builder
	if rc := cmdEggInfo(inv2, &out2, &err2); rc != exitBlocked {
		t.Fatalf("`egg info nosuch` rc=%d 期望 %d（册内没有 ⇒ 不给结论）\n%s", rc, exitBlocked, err2.String())
	}
	for _, want := range []string{"名册里没有这枚卵", "nosuch", "退码 8"} {
		if !strings.Contains(err2.String(), want) {
			t.Fatalf("rc=8 报话丢了 %q：\n%s", want, err2.String())
		}
	}
}

// ②b 目录参数（不是普通文件）⇒ 仍走名册面（判据不误伤「以目录名当卵名」的旧用法）。
func TestEggInfoDirArgStaysRoster(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "store")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("建库根：%v", err)
	}
	ad := filepath.Join(dir, "a-directory")
	if err := os.MkdirAll(ad, 0o755); err != nil {
		t.Fatalf("建目录：%v", err)
	}
	inv := zegInfoTestInv([]string{ad}, map[string][]string{"--root": {root}})
	var out, errb strings.Builder
	if rc := cmdEggInfo(inv, &out, &errb); rc != exitBlocked {
		t.Fatalf("`egg info <目录>` rc=%d 期望 %d（目录 ⇒ 仍按名册查）\n%s", rc, exitBlocked, errb.String())
	}
	if !strings.Contains(errb.String(), "名册里没有这枚卵") {
		t.Fatalf("目录参数没有回落名册面：\n%s", errb.String())
	}
}
