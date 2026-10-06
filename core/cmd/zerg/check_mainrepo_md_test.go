// check_mainrepo_md_test.go —— 判据件 `scripts/gates/check-mainrepo-md.py` Go 真搬的成对负控。
//
// 覆盖面（每条断言都带「牙」：改动实现即红）：
//
//	TestCheckMainrepoMdHelp          · `-h` ⇒ rc=0 且 stdout 逐字 = argparse 帮助块
//	TestCheckMainrepoMdUnknownFlag   · `--bogus` ⇒ rc=2 + stderr usage/error 正文
//	TestCheckMainrepoMdRootNoValue   · `--root`（缺值）⇒ rc=2 + 「expected one argument」
//	TestCheckMainrepoMdRootShape     · 仓根形态不对 ⇒ rc=2（缺 AGENTS.md 且无 docs/）
//	TestCheckMainrepoMdB2Missing     · 盘上有、表里无 ⇒ rc=1 且点名
//	TestCheckMainrepoMdB1Dangling    · 表里点名、盘上无 ⇒ rc=1 且点名
//	TestCheckMainrepoMdB3Empty       · 登记件 0 字节 ⇒ rc=1
//	TestCheckMainrepoMdSelfTest      · `--self-test` ⇒ rc=0 且含「自检 PASS（4 格）」
//	TestCheckMainrepoMdPyJoin         · py join 三类语义各钉一格（空段 / `..` 保留 / 绝对后段）
//	TestCheckMainrepoMdPyRelpath      · py relpath（同根 ⇒ 相对；含 `..` 前缀算法）
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func cmrmTestWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("写夹具失败 %s: %v", p, err)
	}
}

func cmrmTestFixture(t *testing.T) string {
	t.Helper()
	td := t.TempDir()
	if err := os.MkdirAll(cmrmPyJoin(td, "docs", "zh"), 0o755); err != nil {
		t.Fatalf("建夹具失败: %v", err)
	}
	cmrmTestWrite(t, cmrmPyJoin(td, "AGENTS.md"), "x\n")
	cmrmTestWrite(t, cmrmPyJoin(td, "docs", "zh", "a.md"), "y\n")
	return td
}

func TestCheckMainrepoMdHelp(t *testing.T) {
	var out, errb bytes.Buffer
	rc := checkMainrepoMdGateMain(&out, &errb, []string{"-h"})
	if rc != 0 {
		t.Fatalf("`-h` 应 rc=0，得 %d", rc)
	}
	if out.String() != cmrmHelp {
		t.Fatalf("帮助正文不逐字：\n got=%q\nwant=%q", out.String(), cmrmHelp)
	}
	if errb.Len() != 0 {
		t.Fatalf("`-h` 不该写 stderr：%q", errb.String())
	}
}

func TestCheckMainrepoMdUnknownFlag(t *testing.T) {
	var out, errb bytes.Buffer
	rc := checkMainrepoMdGateMain(&out, &errb, []string{"--bogus"})
	if rc != 2 {
		t.Fatalf("未知旗标应 rc=2，得 %d", rc)
	}
	se := errb.String()
	if !strings.Contains(se, "unrecognized arguments: --bogus") {
		t.Fatalf("stderr 缺 argparse 报话：%q", se)
	}
	if !strings.HasPrefix(se, cmrmUsage+"\n") {
		t.Fatalf("stderr 头缺 usage 行：%q", se)
	}
}

func TestCheckMainrepoMdRootNoValue(t *testing.T) {
	var out, errb bytes.Buffer
	rc := checkMainrepoMdGateMain(&out, &errb, []string{"--root"})
	if rc != 2 {
		t.Fatalf("`--root` 缺值应 rc=2，得 %d", rc)
	}
	if !strings.Contains(errb.String(), "argument --root: expected one argument") {
		t.Fatalf("stderr 缺缺值报话：%q", errb.String())
	}
}

func TestCheckMainrepoMdRootShape(t *testing.T) {
	td := t.TempDir() // 无 AGENTS.md、无 docs/
	var out, errb bytes.Buffer
	rc := checkMainrepoMdGateMain(&out, &errb, []string{"--root", td})
	if rc != 2 {
		t.Fatalf("仓根形态不对应 rc=2，得 %d", rc)
	}
	if !strings.Contains(errb.String(), "仓根形态不对") {
		t.Fatalf("stderr 缺形态报话：%q", errb.String())
	}
}

func TestCheckMainrepoMdB2Missing(t *testing.T) {
	td := cmrmTestFixture(t)
	saved := cmrmRegistry
	defer func() { cmrmRegistry = saved }()
	cmrmRegistry = map[string]string{"AGENTS.md": "根面"} // docs/zh/a.md 未登记 ⇒ B2
	var out, errb bytes.Buffer
	rc := cmrmRun(td, false, &out, &errb)
	if rc != 1 {
		t.Fatalf("B2 漏登记应 rc=1，得 %d", rc)
	}
	if !strings.Contains(out.String(), "✗ docs/zh/a.md") {
		t.Fatalf("stdout 未点名漏登记件：%q", out.String())
	}
}

func TestCheckMainrepoMdB1Dangling(t *testing.T) {
	td := cmrmTestFixture(t)
	saved := cmrmRegistry
	defer func() { cmrmRegistry = saved }()
	cmrmRegistry = map[string]string{"AGENTS.md": "根面", "docs/zh/a.md": "文档面", "docs/gone.md": "幽灵"}
	var out, errb bytes.Buffer
	rc := cmrmRun(td, false, &out, &errb)
	if rc != 1 {
		t.Fatalf("B1 悬空登记应 rc=1，得 %d", rc)
	}
	if !strings.Contains(out.String(), "✗ docs/gone.md") {
		t.Fatalf("stdout 未点名悬空件：%q", out.String())
	}
}

func TestCheckMainrepoMdB3Empty(t *testing.T) {
	td := cmrmTestFixture(t)
	saved := cmrmRegistry
	defer func() { cmrmRegistry = saved }()
	cmrmRegistry = map[string]string{"AGENTS.md": "根面", "docs/zh/a.md": "文档面"}
	cmrmTestWrite(t, cmrmPyJoin(td, "docs", "zh", "a.md"), "") // 0 字节 ⇒ B3
	var out, errb bytes.Buffer
	rc := cmrmRun(td, false, &out, &errb)
	if rc != 1 {
		t.Fatalf("B3 空件应 rc=1，得 %d", rc)
	}
	if !strings.Contains(out.String(), "✗ docs/zh/a.md") {
		t.Fatalf("stdout 未点名空件：%q", out.String())
	}
}

func TestCheckMainrepoMdSelfTest(t *testing.T) {
	var out, errb bytes.Buffer
	rc := checkMainrepoMdGateMain(&out, &errb, []string{"--self-test"})
	if rc != 0 {
		t.Fatalf("自检应 rc=0，得 %d（stdout=%q）", rc, out.String())
	}
	if !strings.Contains(out.String(), "自检 PASS（4 格）") {
		t.Fatalf("自检未报 PASS：%q", out.String())
	}
}

func TestCheckMainrepoMdPyJoin(t *testing.T) {
	// 空段：py join("a","")=="a/" · join("","a")=="a" · join("a","","b")=="a/b"
	if cmrmPyJoin("a", "") != "a/" {
		t.Fatalf("join(a,'') 应为 'a/'，得 %q", cmrmPyJoin("a", ""))
	}
	if cmrmPyJoin("a", "", "b") != "a/b" {
		t.Fatalf("join(a,'',b) 应为 'a/b'，得 %q", cmrmPyJoin("a", "", "b"))
	}
	// `..` 段：py join 原样保留
	if cmrmPyJoin("a", "../b") != "a/../b" {
		t.Fatalf("join(a,../b) 应保留 'a/../b'，得 %q", cmrmPyJoin("a", "../b"))
	}
	// 绝对后段：py 丢弃前面所有段
	if cmrmPyJoin("repo", "/abs") != "/abs" {
		t.Fatalf("join(repo,/abs) 应为 '/abs'，得 %q", cmrmPyJoin("repo", "/abs"))
	}
}

func TestCheckMainrepoMdPyRelpath(t *testing.T) {
	got := cmrmPyRelpath("/w", "/w/docs/zh/a.md", "/w")
	if got != "docs/zh/a.md" {
		t.Fatalf("relpath 同根应相对，得 %q", got)
	}
	got = cmrmPyRelpath("/w", "/w/docs/zh/a.md", "/w/other")
	if got != "../docs/zh/a.md" {
		t.Fatalf("relpath 应含 `..` 前缀，得 %q", got)
	}
}

// TestCheckMainrepoMdSelfTestNoLeak —— A 类回归牙：自检的临时目录必须回收（残留 0）。
// ★ 有牙：删掉主件 self-test 建点成功支的 `defer os.RemoveAll(td)` ⇒ 本用例必红（残留 1）。
func TestCheckMainrepoMdSelfTestNoLeak(t *testing.T) {
	td := t.TempDir()
	t.Setenv("TMPDIR", td)
	var out, errb bytes.Buffer
	rc := checkMainrepoMdGateMain(&out, &errb, []string{"--self-test"})
	if rc != 0 {
		t.Fatalf("自检应 rc=0，得 %d", rc)
	}
	ents, _ := os.ReadDir(td)
	left := 0
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), "mainrepo-md-selftest-") {
			left++
		}
	}
	if left != 0 {
		t.Fatalf("A 类：自检临时目录未回收，残留 %d 个", left)
	}
}
