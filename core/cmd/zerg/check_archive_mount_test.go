// check_archive_mount_test.go —— `check_archive_mount.go` 的**成对负控**单测（M2 · 承接 GAP-20261001-13）。
//
// 口径：**成对负控**（✗ 禁恒绿）—— 同一套判据，正控（好夹具）⇒ rc=0；负控（可写档 / 实体目录 /
// 未挂空目录 / 件数 0 / 样本读不到 / 两处读数打架 / 空转 / 旋钮指空树根 / 仓根取不到）⇒ 各自 rc=1 / rc=2 并**逐条点名**；
// 真话负控（把负控那处补回）⇒ 回绿 rc=0。**只去掉判据本身**会 FAIL（哑化验证见交付读数 · 仓外副本）。
//
// 本件**不碰真归档区**：全部夹具造在 `t.TempDir()` 里；真仓现读一格只在默认根在盘时才跑。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func camRun(argv ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := checkArchiveMountGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

func camWrite(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "件.txt")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// camMkMount —— 造一个挂载点目录（n 件 + 可选样本件）。
func camMkMount(t *testing.T, n int, sample bool) string {
	t.Helper()
	mp := filepath.Join(t.TempDir(), "mnt")
	if err := os.MkdirAll(mp, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		_ = os.WriteFile(filepath.Join(mp, "f"+string(rune('0'+i))+".md"), []byte("x"), 0o644)
	}
	if sample {
		_ = os.WriteFile(filepath.Join(mp, camDefaultSample), []byte("索引"), 0o644)
	}
	return mp
}

// camMountTable —— 造挂载表件；ro=false ⇒ 选项表无 `read-only`（可写档）。
func camMountTable(t *testing.T, mp string, line bool, ro bool) string {
	t.Helper()
	body := ""
	if line {
		if ro {
			body = "dev on " + mp + " (apfs, local, nodev, read-only, journaled)\n"
		} else {
			body = "dev on " + mp + " (apfs, local, nodev, journaled)\n"
		}
	} else {
		body = "dev on / (apfs, local, read-only, sealed)\n"
	}
	return camWrite(t, body)
}

func camLink(t *testing.T, target string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "Zerg-归档")
	if err := os.Symlink(target, p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheckArchiveMountGatePort(t *testing.T) {
	// ① 正控：已挂 read-only + 件数够 + 样本读得到 ⇒ rc=0
	mp := camMkMount(t, 3, true)
	link := camLink(t, mp)
	tbl := camMountTable(t, mp, true, true)
	hdi := camWrite(t, "hdiutil: "+mp+"\n")
	if rc, out, _ := camRun("--path", link, "--mountpoint", mp, "--mount-table", tbl,
		"--hdiutil-info", hdi, "--min-files", "2"); rc != 0 || !strings.Contains(out, "⇒ 全绿（rc=0）") {
		t.Fatalf("正控失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	}

	// ② 负控 A1：已挂**可写**（挂载行无 read-only）⇒ rc=1 且点名 A1
	rwTbl := camMountTable(t, mp, true, false)
	if rc, out, _ := camRun("--path", link, "--mountpoint", mp, "--mount-table", rwTbl,
		"--min-files", "2", "--no-hdiutil"); rc != 1 || !strings.Contains(out, "✗ A1 ") {
		t.Fatalf("负控①（可写档）失败：期望 rc=1 且点名 A1，实得 rc=%d out=%q", rc, out)
	}
	// ③ 真话负控（成对回绿）：挂载行补回 read-only ⇒ rc=0
	if rc, _, _ := camRun("--path", link, "--mountpoint", mp, "--mount-table", tbl,
		"--min-files", "2", "--no-hdiutil"); rc != 0 {
		t.Fatalf("成对回绿失败：期望 rc=0，实得 rc=%d", rc)
	}

	// ④ 负控 A2：已挂 read-only 但原路径是**实体目录**（不是符号链接）⇒ rc=1 且点名 A2
	real := camMkMount(t, 3, true)
	if rc, out, _ := camRun("--path", real, "--mountpoint", mp, "--mount-table", tbl,
		"--min-files", "2", "--no-hdiutil"); rc != 1 || !strings.Contains(out, "✗ A2 ") {
		t.Fatalf("负控②（实体目录）失败：期望 rc=1 且点名 A2，实得 rc=%d out=%q", rc, out)
	}

	// ⑤ 负控 A3：未挂 + 原路径是**空目录** ⇒ 「静默返回空」⇒ rc=1 且点名 A3
	empty := filepath.Join(t.TempDir(), "空目录")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	noLine := camMountTable(t, mp, false, true)
	if rc, out, _ := camRun("--path", empty, "--mount-table", noLine, "--no-hdiutil"); rc != 1 ||
		!strings.Contains(out, "✗ A3 ") {
		t.Fatalf("负控③（未挂空目录）失败：期望 rc=1 且点名 A3，实得 rc=%d out=%q", rc, out)
	}
	// ⑥ 真话负控（成对回绿）：未挂 + 悬空符号链接 ⇒ 读它明确失败 ⇒ rc=0
	dangling := filepath.Join(t.TempDir(), "悬空")
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), dangling); err != nil {
		t.Fatal(err)
	}
	if rc, out, _ := camRun("--path", dangling, "--mount-table", noLine, "--no-hdiutil"); rc != 0 ||
		!strings.Contains(out, "这就是对的样子") {
		t.Fatalf("成对回绿（悬空链）失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	}

	// ⑦ 负控 A4：已挂 read-only 但**件数 0** ⇒ rc=1 且点名 A4
	emptyMp := camMkMount(t, 0, false)
	emptyLink := camLink(t, emptyMp)
	emptyTbl := camMountTable(t, emptyMp, true, true)
	if rc, out, _ := camRun("--path", emptyLink, "--mountpoint", emptyMp, "--mount-table", emptyTbl,
		"--min-files", "2", "--no-hdiutil"); rc != 1 || !strings.Contains(out, "✗ A4 ") {
		t.Fatalf("负控④（件数 0）失败：期望 rc=1 且点名 A4，实得 rc=%d out=%q", rc, out)
	}

	// ⑧ 负控 A5：已挂 read-only + 件数够 但**样本件读不到** ⇒ rc=1 且点名 A5
	noSampleMp := camMkMount(t, 3, false)
	noSampleLink := camLink(t, noSampleMp)
	noSampleTbl := camMountTable(t, noSampleMp, true, true)
	if rc, out, _ := camRun("--path", noSampleLink, "--mountpoint", noSampleMp, "--mount-table", noSampleTbl,
		"--min-files", "2", "--no-hdiutil"); rc != 1 || !strings.Contains(out, "✗ A5 ") ||
		!strings.Contains(out, "No such file or directory") {
		t.Fatalf("负控⑤（样本读不到）失败：期望 rc=1 且点名 A5，实得 rc=%d out=%q", rc, out)
	}

	// ⑨ 负控 A6：两处读数打架（挂载表有行 · hdiutil 不认）⇒ rc=2 不给结论
	hdiNone := camWrite(t, "hdiutil:（没有这一条）\n")
	if rc, out, _ := camRun("--path", link, "--mountpoint", mp, "--mount-table", tbl,
		"--hdiutil-info", hdiNone, "--min-files", "2"); rc != 2 ||
		!strings.Contains(out, "两处读数打架") {
		t.Fatalf("负控⑥（两处打架）失败：期望 rc=2，实得 rc=%d out=%q", rc, out)
	}
	// ⑩ 真话负控（成对回绿）：hdiutil 认这个挂载点 ⇒ rc=0
	if rc, _, _ := camRun("--path", link, "--mountpoint", mp, "--mount-table", tbl,
		"--hdiutil-info", hdi, "--min-files", "2"); rc != 0 {
		t.Fatalf("成对回绿（hdiutil 认）失败：期望 rc=0，实得 rc=%d", rc)
	}

	// ⑪ 负控：挂载表 0 行 ⇒ 空转 rc=2（✗ 不当绿）
	if rc, out, _ := camRun("--path", link, "--mount-table", camWrite(t, "")); rc != 2 ||
		!strings.Contains(out, "空转不给结论") {
		t.Fatalf("负控⑦（空转）失败：期望 rc=2，实得 rc=%d out=%q", rc, out)
	}

	// ⑫ 负控：旋钮**显式**指到不在盘的归档树根 ⇒ rc=2 不给结论（✗ 不当 0）
	if rc, out, _ := camRun("--path", filepath.Join(t.TempDir(), "不在盘"), "--no-hdiutil"); rc != 2 ||
		!strings.Contains(out, "在盘上取不到") {
		t.Fatalf("负控⑧（旋钮指空树根）失败：期望 rc=2，实得 rc=%d out=%q", rc, out)
	}

	// ⑫′ 负控：仓根 `--root` 取不到（不存在 / 不是目录）⇒ 空集守卫 rc=2 不给结论（stderr BLOCKED · stdout 无门头）
	if rc, out, errb := camRun("--root", filepath.Join(t.TempDir(), "无此根"), "--mount-table", noLine, "--no-hdiutil"); rc != 2 ||
		strings.Contains(out, "门㉛") || !strings.Contains(errb, "BLOCKED —— 空域 / 取不到输入：仓根") {
		t.Fatalf("负控⑨（仓根取不到）失败：期望 rc=2 · stderr BLOCKED · stdout 无门头，实得 rc=%d out=%q err=%q", rc, out, errb)
	}
	// ⑫″ 真话负控（成对回绿）：仓根在盘（显式 `--root` = 真目录）⇒ 守卫不触发、照常判（未挂断链 rc=0）
	dang := filepath.Join(t.TempDir(), "悬空链")
	_ = os.Symlink(filepath.Join(t.TempDir(), "gone"), dang)
	if rc, out, _ := camRun("--root", t.TempDir(), "--path", dang, "--mount-table", noLine, "--no-hdiutil"); rc != 0 ||
		!strings.Contains(out, "门㉛ 归档挂载档") {
		t.Fatalf("成对回绿（仓根在盘）失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
	}

	// ⑬ --self-test（合成夹具 · 13 格）⇒ rc=0 全过
	if rc, out, _ := camRun("--self-test"); rc != 0 || !strings.Contains(out, "自检结论: 全过") ||
		!strings.Contains(out, "自检：13 格 · 0 格不过") {
		t.Fatalf("--self-test 失败：期望 rc=0 · 13 格全过，实得 rc=%d out=%q", rc, out)
	}

	// ⑭ --list-rules / -h ⇒ rc=0
	if rc, out, _ := camRun("--list-rules"); rc != 0 ||
		!strings.Contains(out, "A5") || !strings.Contains(out, "A6") {
		t.Fatalf("--list-rules 失败：期望 rc=0 含 A5/A6，实得 rc=%d out=%q", rc, out)
	}
	if rc, out, _ := camRun("-h"); rc != 0 || !strings.Contains(out, "usage: check-archive-mount.py") {
		t.Fatalf("-h 失败：期望 rc=0 含 usage，实得 rc=%d", rc)
	}

	// ⑮ Q-176：`--json` 在本门**未实现** ⇒ rc=2 且 stderr 点名（机器面缺 ≠ 零命中）
	if rc, _, errb := camRun("--json"); rc != 2 || !strings.Contains(errb, "本门无机器读面") {
		t.Fatalf("--json 拒收失败：期望 rc=2 且 stderr 点名，实得 rc=%d err=%q", rc, errb)
	}

	// ⑯ 真仓现读（默认根 = cwd 上溯 · 与 py 默认档同构）⇒ rc=0（默认档：未挂断链是常态）
	if fi, err := os.Stat(filepath.Join(camDefaultRoot(), "scripts", "gates")); err == nil && fi.IsDir() {
		if rc, out, _ := camRun(); rc != 0 || !strings.Contains(out, "门㉛ 归档挂载档") {
			t.Fatalf("真仓现读失败：期望 rc=0，实得 rc=%d out=%q", rc, out)
		}
	}
}
