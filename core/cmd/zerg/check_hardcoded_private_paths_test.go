// check_hardcoded_private_paths_test.go —— `hardcodedPrivatePathsGateMain` 的定向用例（M2 路B · 成对负控）。
//
// 纪律：本件是 `_test.go` ⇒ 落在门自己的扫描面内但**整体豁免**（`is_test_file`），不影响真仓现读读数；
// 夹具一律落 `t.TempDir()`（不读不写仓内件）。★ 成对负控：红灯格与绿灯格**成对**（哑化判据必 FAIL）。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hcppTestRun(argv ...string) (int, string, string) {
	var so, se strings.Builder
	rc := hardcodedPrivatePathsGateMain(&so, &se, argv)
	return rc, so.String(), se.String()
}

func TestCheckHardcodedPrivatePathsGatePort(t *testing.T) {
	n := hcppNeedles()[0] // 运行时取（本件正文里没有原串 —— 件头自指检查）
	tmp := t.TempDir()

	mk := func(name, rel, body string) string {
		d := filepath.Join(tmp, name)
		p := filepath.Join(d, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("建夹具失败 %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("写夹具失败 %s: %v", rel, err)
		}
		return d
	}

	// ① 正控：干净件 ⇒ rc=0 + 「通过」面
	clean := mk("clean", "core/main.go", "package main\n\nvar x = \"hello\"\n")
	rc, out, _ := hcppTestRun(clean)
	if rc != 0 || !strings.Contains(out, "结果: 通过——未发现写死的私有绝对路径（已登记待修补 0 个文件）") {
		t.Fatalf("正控-干净件失败：得 rc=%d out=%q", rc, out)
	}

	// ② 负控（有牙）：`core/` 代码行写死私径 ⇒ rc=1 + 逐条点名
	dirty := mk("dirty", "core/main.go", "package main\n\nvar p = \""+n+"/x\"\n")
	rc, out, _ = hcppTestRun(dirty)
	if rc != 1 || !strings.Contains(out, "  ✗ core/main.go:3:var p = \""+n+"/x\"") {
		t.Fatalf("负控-core 代码行失败：得 rc=%d out=%q", rc, out)
	}

	// ③ 成对负控（防假红）：同一路径在**注释行** ⇒ 必须仍是 rc=0
	comment := mk("comment", "core/main.go", "package main\n\n// 历史：曾写死 "+n+"/x（注释不算）\nvar x = 1\n")
	if rc, out, _ = hcppTestRun(comment); rc != 0 {
		t.Fatalf("负控-注释防假红失败：得 rc=%d out=%q", rc, out)
	}

	// ④ 负控：`agent/` 面命中（换一棵扫描树）
	agentHit := mk("agenthit", "agent/x.go", "package agent\n\nvar p = \""+n+"/y\"\n")
	rc, out, _ = hcppTestRun(agentHit)
	if rc != 1 || !strings.Contains(out, "  ✗ agent/x.go:3:") {
		t.Fatalf("负控-agent 面失败：得 rc=%d out=%q", rc, out)
	}

	// ⑤ 负控：`ui/src/**.rs` 生产段命中
	rsHit := mk("rshit", "ui/src/a.rs", "fn f() {\n    let p = \""+n+"/z\";\n}\n")
	rc, out, _ = hcppTestRun(rsHit)
	if rc != 1 || !strings.Contains(out, "  ✗ ui/src/a.rs:2:") {
		t.Fatalf("负控-rust 生产段失败：得 rc=%d out=%q", rc, out)
	}

	// ⑥ 成对负控：同一路径落在 Rust `#[cfg(test)]` 段内 ⇒ 必须仍是 rc=0（按括号深度豁免）
	rsCfg := mk("rscfg", "ui/src/b.rs", "#[cfg(test)]\nmod tests {\n    const P: &str = \""+n+"/z\";\n}\n")
	if rc, out, _ = hcppTestRun(rsCfg); rc != 0 {
		t.Fatalf("负控-rust cfg(test) 豁免失败：得 rc=%d out=%q", rc, out)
	}

	// ⑦ 成对负控：`ui/src/**test*` 夹具件整体豁免 ⇒ rc=0
	uiTest := mk("uitest", "ui/src/foo_test.rs", "fn f() {\n    let p = \""+n+"/z\";\n}\n")
	if rc, _, _ = hcppTestRun(uiTest); rc != 0 {
		t.Fatalf("负控-ui/src 测试件豁免失败：得 rc=%d", rc)
	}

	// ⑧ 成对负控：跳过目录（`target` + 以 `.` 开头）⇒ rc=0
	_ = mk("skipdir", "core/target/x.go", "package main\n\nvar p = \""+n+"/x\"\n")
	_ = mk("skipdot", "core/.zerg/y.go", "package main\n\nvar p = \""+n+"/y\"\n")
	// ★ 空集守卫（GAP-20260927-108）随动：扫描面需非空（全件计数），故补一枚正常件；跳过的仍是 target/.zerg
	_ = mk("skipdir", "core/keep.go", "package main\n\nvar x = 1\n")
	if rc, out, _ = hcppTestRun(filepath.Join(tmp, "skipdir"), filepath.Join(tmp, "skipdot")); rc != 0 {
		t.Fatalf("负控-跳过目录失败：得 rc=%d out=%q", rc, out)
	}

	// ⑨ 负控：二进制件（前 4 KiB 含 NUL）跳过 ⇒ rc=0
	binDir := filepath.Join(tmp, "binfile")
	if err := os.MkdirAll(filepath.Join(binDir, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "core", "blob.dat"), []byte("hdr\x00"+n+"/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, out, _ = hcppTestRun(binDir); rc != 0 {
		t.Fatalf("负控-二进制跳过失败：得 rc=%d out=%q", rc, out)
	}

	// ⑩ `--strict`：空登记表 ⇒ 与缺省档同值（命中仍 red）
	rc, out, _ = hcppTestRun(dirty, "--strict")
	if rc != 1 || !strings.Contains(out, "  ✗ core/main.go:3:") {
		t.Fatalf("--strict 档失败：得 rc=%d out=%q", rc, out)
	}

	// ⑪ 空转：`core/`/`agent/`/`ui/src` 都不在（空树）⇒ ★ 空集守卫（GAP-20260927-108）：fail-closed rc=2
	empty := filepath.Join(tmp, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	if rc, out, _ = hcppTestRun(empty); rc != 2 || strings.Contains(out, "结果: 通过——") {
		t.Fatalf("空转档失败：得 rc=%d out=%q", rc, out)
	}

	// ⑫ 用法错：未知旗标 ⇒ rc=2 + stderr 正文
	rc, _, se := hcppTestRun(clean, "--no-such-flag")
	if rc != 2 || !strings.Contains(se, "✗ 用法错（rc=2）：未知旗标 --no-such-flag") {
		t.Fatalf("未知旗标档失败：得 rc=%d stderr=%q", rc, se)
	}

	// ⑬ 用法错：`--json` 无机器读面 ⇒ rc=2
	if rc, _, se = hcppTestRun("--json"); rc != 2 || !strings.Contains(se, "本门无机器读面") {
		t.Fatalf("--json 拒收档失败：得 rc=%d stderr=%q", rc, se)
	}

	// ⑭ 自检三档 ⇒ rc=0
	rc, out, _ = hcppTestRun("--self-test")
	if rc != 0 || !strings.Contains(out, "── 自检：全过 ✓") {
		t.Fatalf("self-test 档失败：得 rc=%d out=%q", rc, out)
	}
}
