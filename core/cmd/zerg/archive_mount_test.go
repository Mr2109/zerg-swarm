// archive_mount_test.go —— 归档挂/卸口（`zerg archive mount` · `cmdArchiveMount`）的
// **Go 真身行为金标自证** ＋ **脚本移出仓无依赖**负控。
//
// 为什么这么测（M2 口径 · 与 `check_archive_mount_test.go` 的成对负控同族）：
//
//	真搬后命令面**零引用**脚本 ⇒ 原「脚本直跑 ⟷ 命令面」逐字对拍（旧名
//	`TestArchiveMountTwoStatesVerbatim`）的**对拍源** `scripts/svc/archive-mount.sh` 已于
//	2026-10-08 退场（DEV-2031）⇒ 对拍源消失、断言恒 Skip。按 DEV-2115 先例（同件
//	`TestArchiveMountUsageGoldenInternalized`）把金标真源**收回 Go 真身本身**：十四档在
//	合成夹具上归一化后的 (rc, stdout, stderr) 冻成常量，现跑 `cmdArchiveMount` 自证
//	（✗ 不再读任何脚本 · ✗ 不放宽为「不检/只判非空」）；并另证**脚本不在仓**时命令面
//	读数不变（✗ 不得因脚本缺席退 8/2 —— 那是壳转账残留）。
//
// 夹具用 env 旋钮钉死（脚本与本件**同一批旋钮名**）：
//
//	ZERG_ARCHIVE_DMG / ZERG_ARCHIVE_MOUNT / ZERG_ARCHIVE_LINK / ZERG_HDIUTIL / ZERG_MOUNT_LIST
//
// 真挂/摘要插介质（本机不可跑）⇒ 用 **stub `hdiutil`** ＋ **stub 挂载表命令**（仓外夹具）
// 把 `on`/`off`/`status` 三档全跑起来；`attach`/`detach` 只写/清挂载表件、命令本身绝不碰真卷。
// ★ 金标把夹具绝对路径归一为 `<FIX>`（`amNorm`）⇒ 与 `t.TempDir()` 随机路径无关、跨机可复现。
//
// 哑化负控（"牙"）只在**仓外副本**做（✗ 不在落仓件上做）：把 `amCmdStatus` 改 `return 0`、
// 或把 `amGrepF` 改成恒空 ⇒ 对应档 rc/stdout 与冻值不符 ⇒ 当场 FAIL 并给出行号。
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 夹具 ─────────────────────────────────────────────────────────────────────

type amFixture struct {
	dir        string
	dmg        string
	mount      string
	link       string
	hdiutil    string
	mountlist  string
	tablePath  string
	tableBody  string
	mountDir   bool
	mountFileN int
}

func amWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// amMakeFixture 造一棵夹具树：dmg 件 + 挂载点目录（可缺）+ 原路径符号链接 + stub hdiutil + stub 挂载表。
//
//	mountDir=false ⇒ 挂载点目录不建（原路径 = 悬空符号链接）。
//	mountFileN    ⇒ 挂载点目录里放几个件（给 status 的「抽验读」面）。
func amMakeFixture(t *testing.T, mountDir bool, mountFileN int, tableTpl string) *amFixture {
	t.Helper()
	d := t.TempDir()
	f := &amFixture{dir: d, mountDir: mountDir, mountFileN: mountFileN}
	f.dmg = filepath.Join(d, "arch.dmg")
	amWrite(t, f.dmg, "x\n")
	f.mount = filepath.Join(d, "mnt")
	f.link = filepath.Join(d, "Zerg-归档")
	if err := os.Symlink(f.mount, f.link); err != nil {
		t.Fatal(err)
	}
	f.tablePath = filepath.Join(d, "mount.table")
	f.tableBody = strings.ReplaceAll(tableTpl, "@@MOUNT@@", f.mount)
	f.mountlist = filepath.Join(d, "stub-mountlist.sh")
	amWrite(t, f.mountlist, "#!/usr/bin/env bash\nexec cat '"+f.tablePath+"'\n")
	f.hdiutil = filepath.Join(d, "stub-hdiutil.sh")
	amWrite(t, f.hdiutil,
		"#!/usr/bin/env bash\n"+
			"case \"$1\" in\n"+
			"  info)   exec cat '"+f.tablePath+"' ;;\n"+
			"  attach) printf 'stubfs on %s (apfs, local, read-only)\\n' '"+f.mount+"' >> '"+f.tablePath+"' ;;\n"+
			"  detach) : > '"+f.tablePath+"' ;;\n"+
			"esac\n")
	f.reset(t)
	return f
}

// reset 把夹具复位到初态（挂载点目录与件 + 挂载表件）—— 两态各自跑前各调一次。
func (f *amFixture) reset(t *testing.T) {
	t.Helper()
	_ = os.RemoveAll(f.mount)
	if f.mountDir {
		if err := os.MkdirAll(f.mount, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < f.mountFileN; i++ {
			amWrite(t, filepath.Join(f.mount, "f"+string(rune('0'+i))+".md"), "x")
		}
	}
	amWrite(t, f.tablePath, f.tableBody)
}

func amSetEnv(t *testing.T, f *amFixture) {
	t.Helper()
	t.Setenv("ZERG_ARCHIVE_DMG", f.dmg)
	t.Setenv("ZERG_ARCHIVE_MOUNT", f.mount)
	t.Setenv("ZERG_ARCHIVE_LINK", f.link)
	t.Setenv("ZERG_HDIUTIL", f.hdiutil)
	t.Setenv("ZERG_MOUNT_LIST", f.mountlist)
}

// amFace 走命令面（进程内直调 `cmdArchiveMount`）。
func amFace(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var o, e bytes.Buffer
	inv := &invocation{path: []string{"archive", "mount"}, orig: append([]string{"archive", "mount"}, args...)}
	rc := cmdArchiveMount(inv, &o, &e)
	return rc, o.String(), e.String()
}

// ── 十四档 Go 真身金标（现跑自证 · ✗ 不读已退脚本）──────────────────────────

// amNorm 把夹具绝对路径归一为 `<FIX>`（`t.TempDir()` 随机 ＋ macOS `/var`↔`/private/var`）⇒ 金标跨机可复现。
func amNorm(f *amFixture, s string) string {
	s = strings.ReplaceAll(s, f.dir, "<FIX>")
	if real, err := filepath.EvalSymlinks(f.dir); err == nil && real != f.dir {
		s = strings.ReplaceAll(s, real, "<FIX>")
	}
	return s
}

// TestArchiveMountTwoStatesGoldenInternalized —— 十四档（旧名 TestArchiveMountTwoStatesVerbatim）的
// rc/stdout/stderr **冻结金标自证**：`usage` 档 stdout 必须逐字 == Go 真源 `amUsage`（DEV-2115 同族），
// 其余档与冻值逐字同（✗ 不再读任何脚本 · ✗ 不放宽为「不检/只判非空」）。
func TestArchiveMountTwoStatesGoldenInternalized(t *testing.T) {
	const mounted = "stubfs on @@MOUNT@@ (apfs, local, read-only)\n"
	cases := []struct {
		name          string
		args          []string
		mountDir      bool
		mountFile     int
		table         string
		dmgAbsent     bool
		linkAsFile    bool
		hdiutilBroken bool
		usage         bool // stdout 必须逐字 == Go 真源 amUsage
		wantRC        int
		wantOut       string
		wantErr       string
	}{
		{name: "usage-help", args: []string{"--help"}, mountDir: true, usage: true},
		{name: "usage-short-h", args: []string{"-h"}, mountDir: true, usage: true},
		{name: "usage-empty", args: []string{}, mountDir: true, usage: true},
		{name: "unknown-arg", args: []string{"--nope-not-a-flag"}, mountDir: true,
			wantRC:  64,
			wantErr: "❌ 未知参数: --nope-not-a-flag（只认 on | off | status）\n"},
		{name: "status-unmounted-empty-dir", args: []string{"status"}, mountDir: true,
			wantRC: 1,
			wantOut: "⚠️ **未挂**（按需挂载态里「没挂」的那一态 —— 这是本命令的**正常业务态**，不是命令坏了）\n" +
				"  原路径 : <FIX>/Zerg-归档 → <FIX>/mnt\n" +
				"  ★ 挂载点目录 <FIX>/mnt 还在（空目录）⇒ 原路径**不报错、会静默返回空** ——\n" +
				"     别把它当「归档是空的」；要真挂上：`zerg archive mount on`\n" +
				"  要读归档：先 `zerg archive mount on`；用完 `… off`（按需，不常驻）\n"},
		{name: "status-unmounted-dangling", args: []string{"status"}, mountDir: false,
			wantRC: 1,
			wantOut: "⚠️ **未挂**（按需挂载态里「没挂」的那一态 —— 这是本命令的**正常业务态**，不是命令坏了）\n" +
				"  原路径 : <FIX>/Zerg-归档 → <FIX>/mnt\n" +
				"  ★ 此刻 <FIX>/mnt 不存在 ⇒ <FIX>/Zerg-归档 是**真·悬空符号链接**：读 <FIX>/Zerg-归档/… 会**明确失败**\n" +
				"     （就是 `No such file or directory` —— 不是「空目录」、更不是「读到空内容」）\n" +
				"  要读归档：先 `zerg archive mount on`；用完 `… off`（按需，不常驻）\n"},
		{name: "status-mounted", args: []string{"status"}, mountDir: true, mountFile: 4, table: mounted,
			wantRC: 0,
			wantOut: "✅ 已挂（按需挂载态里「挂上」的那一态）\n" +
				"  挂载行 : stubfs on <FIX>/mnt (apfs, local, read-only)\n" +
				"  hdiutil: stubfs on <FIX>/mnt (apfs, local, read-only)\n" +
				"  原路径 : <FIX>/Zerg-归档 → <FIX>/mnt（符号链接**解析得通** —— 读它 = 读只读镜像内的件）\n" +
				"  抽验读 : f0.md f1.md f2.md \n"},
		{name: "on-idempotent", args: []string{"on"}, mountDir: true, table: mounted,
			wantRC: 0,
			wantOut: "✅ 已挂（幂等：<FIX>/mnt 已在挂，本次**不重复挂**）\n" +
				"  挂载行 : stubfs on <FIX>/mnt (apfs, local, read-only)\n"},
		{name: "off-idempotent", args: []string{"off"}, mountDir: true,
			wantRC:  0,
			wantOut: "✅ 未挂（幂等：<FIX>/mnt 本就不在挂载表里，本次**不重复摘**）\n"},
		{name: "on-attach", args: []string{"on"}, mountDir: true,
			wantRC: 0,
			wantOut: "✅ 已挂：<FIX>/mnt（只读）\n" +
				"  挂载行 : stubfs on <FIX>/mnt (apfs, local, read-only)\n" +
				"  原路径 : <FIX>/Zerg-归档 → <FIX>/mnt（符号链接现在解析得通）\n"},
		{name: "off-detach", args: []string{"off"}, mountDir: true, table: mounted,
			wantRC: 0,
			wantOut: "✅ 已摘：<FIX>/mnt 不在挂载表里了\n" +
				"  挂载点空目录已收起 ⇒ 原路径现在是**真·悬空符号链接**（读它 = `No such file or directory`）\n" +
				"  原路径 : <FIX>/Zerg-归档 → <FIX>/mnt（要读归档就再 `on` 一次 —— 按需，不常驻）\n"},
		{name: "preflight-dmg-absent", args: []string{"status"}, mountDir: true, dmgAbsent: true,
			wantRC:  2,
			wantErr: "⛔ 不给结论：归档 dmg 不在盘上（<FIX>/no-such.dmg）—— 没有可挂的件，本脚本不硬造\n"},
		{name: "preflight-link-not-symlink", args: []string{"on"}, mountDir: true, linkAsFile: true,
			wantRC:  2,
			wantErr: "⛔ 不给结论：原路径 <FIX>/real-file **不是符号链接**（是实体）⇒ 本脚本的前提不成立（不许把实体目录当挂载点用）\n"},
		{name: "preflight-hdiutil-broken", args: []string{"status"}, mountDir: true, hdiutilBroken: true,
			wantRC:  2,
			wantErr: "⛔ 不给结论：/usr/bin/false 跑不起来（判据取不到读数 ⇒ 绿就是假绿）\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := amMakeFixture(t, c.mountDir, c.mountFile, c.table)
			amSetEnv(t, f)
			if c.dmgAbsent {
				t.Setenv("ZERG_ARCHIVE_DMG", filepath.Join(f.dir, "no-such.dmg"))
			}
			if c.linkAsFile {
				lp := filepath.Join(f.dir, "real-file")
				amWrite(t, lp, "x")
				t.Setenv("ZERG_ARCHIVE_LINK", lp)
			}
			if c.hdiutilBroken {
				t.Setenv("ZERG_HDIUTIL", "/usr/bin/false")
			}
			wantOut := c.wantOut
			if c.usage {
				wantOut = amUsage
			}
			f.reset(t)
			rc, out, errb := amFace(t, c.args...)
			gotOut, gotErr := amNorm(f, out), amNorm(f, errb)
			if rc != c.wantRC {
				t.Errorf("%s：rc=%d（冻值 %d）", c.name, rc, c.wantRC)
			}
			if gotOut != wantOut {
				t.Errorf("%s：stdout 漂移\n实得=%q\n冻值=%q", c.name, gotOut, wantOut)
			}
			if gotErr != c.wantErr {
				t.Errorf("%s：stderr 漂移\n实得=%q\n冻值=%q", c.name, gotErr, c.wantErr)
			}
		})
	}
}

// ── 脚本移出仓 ⇒ 命令面读数不变（✗ 不退 8/2）──────────────────────────────────

// TestArchiveMountRunsWithoutScript —— 仓根里**没有**脚本时命令面照样跑，且读数与「脚本在盘」逐字同。
func TestArchiveMountRunsWithoutScript(t *testing.T) {
	realRoot := repoRoot()
	if realRoot == "" {
		t.Skip("解析不到仓根（合成环境）")
	}
	const mounted = "stubfs on @@MOUNT@@ (apfs, local, read-only)\n"
	cases := []struct {
		name   string
		args   []string
		table  string
		wantRC int
	}{
		{"usage", []string{"--help"}, "", 0},
		{"unknown", []string{"--bogus"}, "", 64},
		{"status-unmounted", []string{"status"}, "", 1},
		{"status-mounted", []string{"status"}, mounted, 0},
		{"on-idempotent", []string{"on"}, mounted, 0},
		{"off-idempotent", []string{"off"}, "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := amMakeFixture(t, true, 0, c.table)
			amSetEnv(t, f)

			// 态 A：仓根 = 真仓（脚本在盘）
			t.Setenv("ZERG_REPO", realRoot)
			f.reset(t)
			rcA, oA, eA := amFace(t, c.args...)

			// 态 B：仓根 = 合成树（**无** scripts/svc/archive-mount.sh）
			noScript := t.TempDir()
			if _, err := os.Stat(filepath.Join(noScript, archiveMountScriptRel)); err == nil {
				t.Fatalf("负控前提破：合成仓根里居然有脚本")
			}
			t.Setenv("ZERG_REPO", noScript)
			f.reset(t)
			rcB, oB, eB := amFace(t, c.args...)

			if rcA != c.wantRC || rcB != c.wantRC {
				t.Errorf("rc：在盘=%d 移出=%d（要 %d；移出档出现 8/2 = 壳转账残留）", rcA, rcB, c.wantRC)
			}
			if oA != oB {
				t.Errorf("stdout 两态不同：在盘=%q 移出=%q", oA, oB)
			}
			if eA != eB {
				t.Errorf("stderr 两态不同：在盘=%q 移出=%q", eA, eB)
			}
		})
	}
}

// TestArchiveMountUsageGoldenInternalized —— amUsage（Go 真源）的 sha256 内化自证（体例同
// ctocHelpNativeSha256 / csdHelpNativeSha256 · GAP-20261007:16）。
// 原版把 amUsage 与脚本第 2..50 行对拍；原件 `scripts/svc/archive-mount.sh` 已于 2026-10-08
// 退场（DEV-2031）⇒ 对拍源消失、断言恒 Skip；金标真源**收回 Go 常量本身** ⇒ 现算自证
// （✗ 不再读任何脚本）。★ 哑化负控：改 amUsage 一字（或把 amUsageNativeSha256 改成与 amUsage
// 无关的值）⇒ 本用例 FAIL（带行号）。
func TestArchiveMountUsageGoldenInternalized(t *testing.T) {
	if got := amUsageNativeSha256(); got != amUsageSha256 {
		t.Fatalf("Go 真源现算 sha256 = %s ≠ 冻结回归值 %s（用法文案真源漂了）", got, amUsageSha256)
	}
	if len(amUsage) != 3203 {
		t.Fatalf("amUsage 字节数：实得 %d（期望 3203）", len(amUsage))
	}
}
