// family_registry_sync_test.go —— `zerg registry sync` 的四格判据（+ 用法面两格）。
//
//	① `--dry-run` **零副作用**且把将改的行列出来（四格之①）；
//	② 真跑**补出缺行**且格式正确（四格之②：第 2 列 `no` · 第 3 列逐字 · TAB 三列 · 尾部追加）；
//	③ `.sh` 缺干跑语义条目 ⇒ **一律不补**，只在输出里点名「需人写」（四格之③ · 硬约束的牙）；
//	④ 连跑两遍，第二遍**零变更**（四格之④ · 幂等）。
//
// 隔离口径：全部跑在 `t.TempDir()` 的**合成仓根**里（`ZERG_REPO_ROOT` 注入 · 不碰真仓一个字节）；
// 夹具只造三面 —— `scripts/**` 两后缀 · `scripts/公开标记.tsv` · `core/internal/contract/dryrun-semantics.json`。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
)

// rsFixture —— 合成仓根 + 建件小工具（返 (根, 建件闭包)）。
func rsFixture(t *testing.T, tsvBody string, dryFiles []string, scripts ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range scripts {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("建目录失败：%v", err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\ntrue\n"), 0o644); err != nil {
			t.Fatalf("建脚本失败：%v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "公开标记.tsv"), []byte(tsvBody), 0o644); err != nil {
		t.Fatalf("建公开标记失败：%v", err)
	}
	dir := filepath.Join(root, "core", "internal", "contract")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建契约目录失败：%v", err)
	}
	var items []string
	for _, f := range dryFiles {
		items = append(items, fmt.Sprintf("{\"file\":%q,\"语义\":\"dryrun.v1\"}", f))
	}
	body := "{\"schema\":\"zerg/dryrun-semantics/v1\",\"件\":[" + strings.Join(items, ",") + "]}\n"
	if err := os.WriteFile(filepath.Join(dir, "dryrun-semantics.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("建干跑语义表失败：%v", err)
	}
	return root
}

// rsTreeDigest —— 整棵树取「相对路径 + 内容 sha256」清单（判零副作用用：**件集合与字节都进面**）。
func rsTreeDigest(t *testing.T, root string) string {
	t.Helper()
	var rows []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, werr error) error {
		if werr != nil || info == nil || info.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, p)
		sum := sha256.Sum256(b)
		rows = append(rows, filepath.ToSlash(rel)+" "+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("取树摘要失败：%v", err)
	}
	sort.Strings(rows)
	return strings.Join(rows, "\n")
}

// rsRun —— 在合成仓根上跑一次（env 注入让 root 定位走显式旋钮）。
func rsRun(t *testing.T, root string, dryRun bool) (int, string, string) {
	t.Helper()
	t.Setenv("ZERG_REPO_ROOT", root)
	var out, errb bytes.Buffer
	inv := &invocation{path: []string{"registry", "sync"}, dryRun: dryRun}
	rc := cmdRegistrySync(inv, &out, &errb)
	return rc, out.String(), errb.String()
}

const rsHdr = "文件\tpublic\t理由\n"

// ① `--dry-run` 零副作用 + 列出待补项。
func TestRegistrySyncDryRunIsZeroSideEffectAndListsRows(t *testing.T) {
	root := rsFixture(t, rsHdr, nil, "scripts/a.sh", "scripts/b.py")
	before := rsTreeDigest(t, root)

	rc, out, errs := rsRun(t, root, true)
	if rc != exitOK {
		t.Fatalf("--dry-run 应 rc=0，实得 %d（stderr=%s）", rc, errs)
	}
	if after := rsTreeDigest(t, root); after != before {
		t.Fatalf("--dry-run 动了盘（零副作用破）：\n--- 前 ---\n%s\n--- 后 ---\n%s", before, after)
	}
	for _, want := range []string{
		"+ scripts/a.sh\tno\t自动补登 · 待人工确认用途",
		"+ scripts/b.py\tno\t自动补登 · 待人工确认用途",
		"盘上一字节未动",
		"将补 2 行",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("--dry-run 输出没列待补项 %q：\n%s", want, out)
		}
	}
	body, _ := os.ReadFile(filepath.Join(root, "scripts", "公开标记.tsv"))
	if string(body) != rsHdr {
		t.Fatalf("--dry-run 之后标记表变了：%q", string(body))
	}
}

// ② 真跑补出缺行 + 格式正确（三列 TAB · 第 2 列 no · 第 3 列逐字 · 尾部追加 · 表头不动）。
func TestRegistrySyncAppendsMissingRowsWithExactFormat(t *testing.T) {
	// 夹具刻意不给尾换行 ⇒ 追加前必须先补一个（否则两行粘成一行）。
	root := rsFixture(t, "文件\tpublic\t理由\nscripts/keep.py\tyes\t已登记", nil,
		"scripts/a.sh", "scripts/keep.py", "scripts/sub/c.sh")

	rc, out, errs := rsRun(t, root, false)
	if rc != exitOK {
		t.Fatalf("真跑应 rc=0，实得 %d（stderr=%s）", rc, errs)
	}
	if !strings.Contains(out, "已补 2 行") {
		t.Fatalf("人面没报「已补 2 行」：\n%s", out)
	}
	b, err := os.ReadFile(filepath.Join(root, "scripts", "公开标记.tsv"))
	if err != nil {
		t.Fatalf("读标记表失败：%v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("应有 4 行（表头 + 已登记 + 新补 2 行），实得 %d：%q", len(lines), string(b))
	}
	if lines[0] != strings.TrimSuffix(rsHdr, "\n") || lines[1] != "scripts/keep.py\tyes\t已登记" {
		t.Fatalf("既有两行被改动：%q / %q", lines[0], lines[1])
	}
	// 追加序 = 脚本面的排序（a.sh < sub/c.sh）· 每行恰三列且第 2/3 列逐字。
	want := []string{"scripts/a.sh", "scripts/sub/c.sh"}
	for i, f := range want {
		got := lines[2+i]
		cols := strings.Split(got, "\t")
		if len(cols) != 3 {
			t.Fatalf("第 %d 行不是 3 列（TAB 分隔）：%q", i+1, got)
		}
		if cols[0] != f || cols[1] != "no" || cols[2] != "自动补登 · 待人工确认用途" {
			t.Fatalf("第 %d 行格式不符：期望 %q/no/自动补登 · 待人工确认用途，实得 %q", i+1, f, got)
		}
	}
}

// ③ `.sh` 缺干跑语义条目 ⇒ **不补**，只在输出里点名（硬约束的牙）。
func TestRegistrySyncNeverWritesDryrunSemantics(t *testing.T) {
	// c.sh 已在公开标记里（不缺行），但干跑语义表**没有**它 ⇒ 只许点名。
	root := rsFixture(t, rsHdr+"scripts/c.sh\tno\t已登记\n", []string{"scripts/other.sh"}, "scripts/c.sh")
	dry := filepath.Join(root, "core", "internal", "contract", "dryrun-semantics.json")
	dryBefore, _ := os.ReadFile(dry)
	marksBefore, _ := os.ReadFile(filepath.Join(root, "scripts", "公开标记.tsv"))

	rc, out, errs := rsRun(t, root, false)
	if rc != exitOK {
		t.Fatalf("rc 应为 0（缺语义面不是本命令的失败），实得 %d（stderr=%s）", rc, errs)
	}
	if !strings.Contains(out, "需人写") || !strings.Contains(out, "! scripts/c.sh") {
		t.Fatalf("输出没点名「需人写」的 .sh：\n%s", out)
	}
	dryAfter, _ := os.ReadFile(dry)
	if !bytes.Equal(dryBefore, dryAfter) {
		t.Fatalf("干跑语义表被改了（硬约束破：语义面一律不补）：\n前 %s\n后 %s", dryBefore, dryAfter)
	}
	marksAfter, _ := os.ReadFile(filepath.Join(root, "scripts", "公开标记.tsv"))
	if !bytes.Equal(marksBefore, marksAfter) {
		t.Fatalf("不缺行时标记表不许动：%q → %q", marksBefore, marksAfter)
	}
}

// ④ 幂等：连跑两遍，第二遍零变更（件字节逐字相同 + 人面报「不缺行」）。
func TestRegistrySyncSecondRunIsNoop(t *testing.T) {
	root := rsFixture(t, rsHdr, nil, "scripts/a.sh", "scripts/b.py")
	if rc, _, errs := rsRun(t, root, false); rc != exitOK {
		t.Fatalf("第一遍应 rc=0，实得 %d（stderr=%s）", rc, errs)
	}
	first, _ := os.ReadFile(filepath.Join(root, "scripts", "公开标记.tsv"))

	rc, out, errs := rsRun(t, root, false)
	if rc != exitOK {
		t.Fatalf("第二遍应 rc=0，实得 %d（stderr=%s）", rc, errs)
	}
	second, _ := os.ReadFile(filepath.Join(root, "scripts", "公开标记.tsv"))
	if !bytes.Equal(first, second) {
		t.Fatalf("第二遍改动了标记表（幂等破）：\n前 %q\n后 %q", first, second)
	}
	if !strings.Contains(out, "不缺行") || !strings.Contains(out, "已补 0 行") && !strings.Contains(out, "0 行待补") {
		t.Fatalf("第二遍人面没报「零变更」：\n%s", out)
	}
}

// 用法面（执前判 · 零写盘）：位置参数 / `--json` 不给字段 / `--json` 字段名不在册。
func TestRegistrySyncUsageFaceIsZeroWrite(t *testing.T) {
	root := rsFixture(t, rsHdr, nil, "scripts/a.sh")
	before := rsTreeDigest(t, root)
	cases := []struct {
		name string
		inv  *invocation
	}{
		{"位置参数不收", &invocation{path: []string{"registry", "sync"}, args: []string{"多余"}, orig: []string{"registry", "sync", "多余"}}},
		{"--json 不给字段", &invocation{path: []string{"registry", "sync"}, jsonGiven: true, orig: []string{"registry", "sync", "--json"}}},
		{"--json 字段名不在册", &invocation{path: []string{"registry", "sync"}, jsonGiven: true,
			fields: []string{"nosuch_field"}, orig: []string{"registry", "sync", "--json", "nosuch_field"}}},
	}
	for _, c := range cases {
		t.Setenv("ZERG_REPO_ROOT", root)
		var out, errb bytes.Buffer
		rc := cmdRegistrySync(c.inv, &out, &errb)
		if rc != exitUsage {
			t.Fatalf("%s：应 rc=2，实得 %d（stdout=%q stderr=%q）", c.name, rc, out.String(), errb.String())
		}
		if out.Len() != 0 {
			t.Fatalf("%s：用法错不许吐 stdout，实得 %q", c.name, out.String())
		}
		if after := rsTreeDigest(t, root); after != before {
			t.Fatalf("%s：用法错动了盘（零副作用破）", c.name)
		}
	}
}

// 机器面：`--json action,face,file,line` 两动作各一行；`add_mark` 带将写的那一行，`needs_human` 无 `line` 键。
func TestRegistrySyncJSONRows(t *testing.T) {
	// c.sh 先进标记表 ⇒ 只剩 dryrun 缺口；a.sh 两面都缺 ⇒ add_mark 1 行 + needs_human 2 行（a.sh / c.sh）。
	root := rsFixture(t, rsHdr, nil, "scripts/a.sh", "scripts/c.sh")
	if err := os.WriteFile(filepath.Join(root, "scripts", "公开标记.tsv"),
		[]byte(rsHdr+"scripts/c.sh\tno\t已登记\n"), 0o644); err != nil {
		t.Fatalf("写夹具失败：%v", err)
	}
	t.Setenv("ZERG_REPO_ROOT", root)
	var out, errb bytes.Buffer
	inv := &invocation{path: []string{"registry", "sync"}, dryRun: true, jsonGiven: true,
		fields: []string{"action", "face", "file", "line"}}
	if rc := cmdRegistrySync(inv, &out, &errb); rc != exitOK {
		t.Fatalf("--json 应 rc=0，实得 %d（%s）", rc, errb.String())
	}
	s := out.String()
	for _, want := range []string{
		`"kind":"RegistrySync"`,
		`"action":"add_mark"`,
		`"file":"scripts/a.sh"`,
		`"line":"scripts/a.sh\tno\t自动补登 · 待人工确认用途"`,
		`"action":"needs_human"`,
		`"file":"scripts/c.sh"`,
		`"count":3`, // a.sh 两面都缺（补行 1 条 ＋ 需人写 1 条）· c.sh 只缺语义面 1 条
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("机器面缺 %q：\n%s", want, s)
		}
	}
	// needs_human 那行**不许**有 `line` 键（缺值整键略去 · 与 API 的 omitempty 同一条铁律）。
	if strings.Contains(s, `"face":"干跑语义","file":"scripts/c.sh","line"`) {
		t.Fatalf("needs_human 行不该带 line 键：\n%s", s)
	}
}

// rsInode —— 取件的 inode（原子写 tmp→rename ⇒ **换 inode**；原地 WriteFile ⇒ inode 不变）。
func rsInode(t *testing.T, path string) uint64 {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s：%v", path, err)
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skipf("本平台取不到 inode（%T）⇒ 本格不判", st.Sys())
	}
	return uint64(sys.Ino)
}

// 原子写两态（硬约束「写盘走 tmp→rename」的牙）：
//
//	正控：真跑之后标记表**换了 inode**且内容对 ⇒ 走的是 rename，不是原地覆写；
//	负控：目录只读 ⇒ 写不成 ⇒ rc=8（不给结论）· **原件逐字节不变** · 不留 tmp 残件。
func TestRegistrySyncWritesAtomically(t *testing.T) {
	// 负控（先做 · 干净夹具：表头一行、a.sh 未登记 ⇒ 真有待补项，才走得到写口）。
	nroot := rsFixture(t, rsHdr, nil, "scripts/a.sh")
	ntsv := filepath.Join(nroot, "scripts", "公开标记.tsv")
	if err := os.Chmod(filepath.Dir(ntsv), 0o555); err != nil {
		t.Fatalf("置只读失败：%v", err)
	}
	nBefore, _ := os.ReadFile(ntsv)
	rcN, outN, _ := rsRun(t, nroot, false)
	_ = os.Chmod(filepath.Dir(ntsv), 0o755)
	if rcN != exitBlocked {
		t.Fatalf("目录不可写 ⇒ 应 rc=8（不给结论），实得 %d（stdout=%q）", rcN, outN)
	}
	nAfter, _ := os.ReadFile(ntsv)
	if !bytes.Equal(nBefore, nAfter) {
		t.Fatalf("写失败却动了原件：%q → %q", nBefore, nAfter)
	}
	if ents, _ := os.ReadDir(filepath.Dir(ntsv)); len(ents) != 2 {
		t.Fatalf("写失败后目录条目数应为 2（a.sh ＋ 公开标记.tsv），实得 %d ⇒ 有 tmp 残件", len(ents))
	}

	// 正控：真跑 ⇒ inode 换（rename 的证据）+ 内容对。
	root := rsFixture(t, rsHdr, nil, "scripts/b.sh")
	tsv := filepath.Join(root, "scripts", "公开标记.tsv")
	inoBefore := rsInode(t, tsv)
	rc, _, errs := rsRun(t, root, false)
	if rc != exitOK {
		t.Fatalf("真跑应 rc=0，实得 %d（stderr=%s）", rc, errs)
	}
	if inoAfter := rsInode(t, tsv); inoAfter == inoBefore {
		t.Fatalf("inode 未变（%d）⇒ 走的是原地覆写，不是 tmp→rename（原子写破）", inoBefore)
	}
	if got, _ := os.ReadFile(tsv); !strings.Contains(string(got), "scripts/b.sh\tno\t自动补登 · 待人工确认用途") {
		t.Fatalf("原子写后内容不对：%q", string(got))
	}
}
