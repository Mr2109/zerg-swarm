// family_skill_test.go —— `family_skill.go` 的定向测试（M1：`skill ls` / `skill show` 两条只读面）。
//
// 落点：`package main`（内部测试 —— 直调 `skillScan` / `cmdSkillLs` / `cmdSkillShow`）。
// 夹具全在 `t.TempDir()` 的**仓外**合成根里（`ZERG_SKILLS_DIR` 指过去）⇒ ✗ 不碰真技能根、
// ✗ 不碰仓内件；读面零写 ⇒ 跑前跑后夹具树逐字不变。
//
// 七格（每格读数打出来供报告取）：
//
//	① `state` 五档闭集逐档对齐（ok / no-description / no-frontmatter / no-skill-md / not-a-dir）；
//	② `ls` rc=0 且人面打出 ok 那一行；
//	③ `show` 的 stdout **逐字节等于** SKILL.md 原文（与 `skill_load` 同源的硬证据）；
//	④ 未知名 ⇒ rc=2（emitNameMiss 那一档 · ✗ 不退 8）；
//	⑤ 空根 ⇒ rc=8（空转不当绿 · 同 `calib ls` 的 roster_empty）；
//	⑥ 根不在盘上 ⇒ rc=8（不给结论）；
//	⑦ 只读自证：跑完夹具树 sha 清单与跑前逐字同。
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// skillTWrite —— 往合成根写一件（自动建父目录）。
func skillTWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// skillTTreeSHA —— 夹具树的「相对路径 → sha256」清单（只读自证用）。
func skillTTreeSHA(t *testing.T, root string) []string {
	t.Helper()
	out := []string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		sum := sha256.Sum256(b)
		rel, _ := filepath.Rel(root, p)
		out = append(out, rel+"\t"+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

// skillTRoot —— 造一枚合成技能根：五档各一件（ok / no-description / no-frontmatter /
// no-skill-md / not-a-dir），并把 `ZERG_SKILLS_DIR` 指过去。
func skillTRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ZERG_SKILLS_DIR", root)
	skillTWrite(t, filepath.Join(root, "ok-skill", "SKILL.md"),
		"---\nname: ok-skill\ndescription: 何时加载这条技能\n---\n\n# ok-skill\n\n正文一段。\n")
	skillTWrite(t, filepath.Join(root, "no-desc", "SKILL.md"),
		"---\nname: no-desc\n---\n\n# no-desc\n")
	skillTWrite(t, filepath.Join(root, "no-fm", "SKILL.md"), "# 没有 frontmatter\n")
	if err := os.MkdirAll(filepath.Join(root, "empty-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillTWrite(t, filepath.Join(root, "stray.md"), "散件\n")
	return root
}

// skillTInv —— 造一枚最小 invocation（只读面不读 tty / 不读确认档 —— 本族 M1 无写面站点）。
func skillTInv(args []string) *invocation {
	return &invocation{path: []string{"skill", "ls"}, args: args}
}

func TestSkillLsFiveStates(t *testing.T) {
	root := skillTRoot(t)
	before := skillTTreeSHA(t, root)

	es, err := skillScan(root)
	if err != nil {
		t.Fatalf("skillScan 报错：%v", err)
	}
	got := len(es)
	if got != 5 {
		t.Fatalf("五档夹具应扫出 5 件，实得 %d 件：%+v", got, es)
	}
	want := map[string]string{
		"ok-skill":  skillStateOK,
		"no-desc":   skillStateNoDesc,
		"no-fm":     skillStateNoFM,
		"empty-dir": skillStateNoDoc,
		"stray.md":  skillStateNotDir,
	}
	for _, e := range es {
		w, ok := want[e.Name]
		if !ok {
			t.Fatalf("扫出未预期的件 %q（state=%s）", e.Name, e.State)
		}
		if e.State != w {
			t.Fatalf("%q 的 state 应为 %s，实得 %s（判据与 agent 侧对齐面分家）", e.Name, w, e.State)
		}
		if want[e.Name] == skillStateOK && strings.TrimSpace(e.Desc) == "" {
			t.Fatalf("%q 是 ok 档却取不到描述 —— 注入面与 ls 面不同源", e.Name)
		}
	}

	// ② ls 人面 rc=0 且打出 ok 那一行
	inv := skillTInv(nil)
	var out, errb bytes.Buffer
	rc := cmdSkillLs(inv, &out, &errb)
	if rc != exitOK {
		t.Fatalf("`skill ls` 应 rc=0，实得 %d（stderr=%q）", rc, errb.String())
	}
	if !strings.Contains(out.String(), "ok-skill") || !strings.Contains(out.String(), skillStateOK) {
		t.Fatalf("`skill ls` 人面没打出 ok 那一行：%q", out.String())
	}

	// ⑦ 只读自证
	after := skillTTreeSHA(t, root)
	if len(after) != len(before) {
		t.Fatalf("读面写了盘：跑前 %d 件 / 跑后 %d 件", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("读面改了夹具：%q ⟷ %q", before[i], after[i])
		}
	}
}

func TestSkillShowByteIdentical(t *testing.T) {
	root := skillTRoot(t)
	doc := filepath.Join(root, "ok-skill", "SKILL.md")
	raw, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}

	inv := skillTInv([]string{"ok-skill"})
	var out, errb bytes.Buffer
	rc := cmdSkillShow(inv, &out, &errb)
	if rc != exitOK {
		t.Fatalf("`skill show ok-skill` 应 rc=0，实得 %d（stderr=%q）", rc, errb.String())
	}
	if !bytes.Equal(out.Bytes(), raw) {
		t.Fatalf("stdout 与 SKILL.md 不是逐字节相同：stdout %d 字节 / 文件 %d 字节", out.Len(), len(raw))
	}
	if out.Len() == 0 {
		t.Fatal("stdout 为空 —— 拿空输出对拍不算证据")
	}
}

func TestSkillShowNameMiss(t *testing.T) {
	skillTRoot(t)

	// ④ 未知名 ⇒ 2（emitNameMiss 那一档）
	inv := skillTInv([]string{"zz-不存在-zz"})
	var out, errb bytes.Buffer
	rc := cmdSkillShow(inv, &out, &errb)
	if rc != exitUsage {
		t.Fatalf("未知名应 rc=2，实得 %d（stderr=%q）", rc, errb.String())
	}
	if out.Len() != 0 {
		t.Fatalf("用法错档 stdout 应零字节，实得 %d 字节", out.Len())
	}
	if strings.TrimSpace(errb.String()) == "" {
		t.Fatal("用法错档必须给拒绝理由（stderr 空 = 判不了）")
	}
}

func TestSkillRootBlocked(t *testing.T) {
	// ⑤ 空根 ⇒ 8
	empty := t.TempDir()
	t.Setenv("ZERG_SKILLS_DIR", empty)
	inv := skillTInv(nil)
	var out, errb bytes.Buffer
	if rc := cmdSkillLs(inv, &out, &errb); rc != exitBlocked {
		t.Fatalf("空根应 rc=8（空转不当绿），实得 %d", rc)
	}

	// ⑥ 根不在盘上 ⇒ 8
	t.Setenv("ZERG_SKILLS_DIR", filepath.Join(t.TempDir(), "zz-nosuch-dir"))
	inv2 := skillTInv(nil)
	var out2, errb2 bytes.Buffer
	if rc := cmdSkillLs(inv2, &out2, &errb2); rc != exitBlocked {
		t.Fatalf("根不在盘上应 rc=8，实得 %d", rc)
	}
}

// ---- M2 · D2 写面（new / edit / rm）--------------------------------------------
//
// 全部跑在 `t.TempDir()` 的两枚合成根里（`ZERG_SKILLS_DIR` 技能根 ＋ `ZERG_STATE_DIR` 状态目录）
// ⇒ ✗ 不碰真技能根、✗ 不碰真状态目录；每条写完都**回读对拍**，不靠"看起来对"。
//
// 八格（矩阵装不进去的那几档落在这里 —— 它们要合成盘面，见设计稿 §7.4 末注）：
//
//	① 干跑档零副作用（不建目录 / 不写件 / 不写审计）；
//	② 缺 `--yes` ⇒ 2 ＋ 零写；名不合规 ⇒ 2 ＋ 零写；
//	③ `new --yes` ⇒ 0 且落件逐字节 == 模板；审计 1 行；
//	④ 同名再 `new --yes` ⇒ **14** 且原文件字节不变（不覆盖）；
//	⑤ `edit --desc` ⇒ 只换 description 那一行（正文逐行不变）＋ 备份等于改前字节；
//	⑥ `edit --from` 改名 ⇒ 2；`edit --from` 同名字 ⇒ 0 且整篇逐字节 == 来源；名不在册 ⇒ 2；
//	⑦ `rm --yes` ⇒ 0 ＋ 目录消失 ＋ 回收站里那件逐字节等于下架前；名不在册 ⇒ 2；
//	⑧ 审计链：skill_new → skill_edit → skill_edit → skill_rm（每行 before/after sha 可核）。
func TestSkillWriteFace(t *testing.T) {
	root := t.TempDir()
	state := t.TempDir()
	t.Setenv("ZERG_SKILLS_DIR", root)
	t.Setenv("ZERG_STATE_DIR", state)

	newInv := func(args []string, desc, from string) *invocation {
		inv := &invocation{path: []string{"skill", "new"}, args: args}
		if desc != "" {
			inv.desc = desc
		}
		if from != "" {
			inv.kvSet("--from", from)
		}
		return inv
	}
	var out, errb bytes.Buffer

	// ② 缺 --yes ⇒ 2（writeGuardYes 第一句 · 零写）
	if rc := cmdSkillNew(newInv([]string{"probe"}, "探针触发", ""), &out, &errb); rc != exitUsage {
		t.Fatalf("缺 --yes 应 rc=2，实得 %d（stderr=%q）", rc, errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, "probe")); !os.IsNotExist(err) {
		t.Fatal("缺 --yes 却建了目录（零写承诺破了）")
	}

	// ② 名不合规 ⇒ 2（执行前判 · 大写/空格/`_` 一律拒）
	out.Reset()
	errb.Reset()
	bad := newInv([]string{"Zz-Bad"}, "x", "")
	bad.yes = true
	if rc := cmdSkillNew(bad, &out, &errb); rc != exitUsage {
		t.Fatalf("名不合规应 rc=2，实得 %d（stderr=%q）", rc, errb.String())
	}
	if out.Len() != 0 {
		t.Fatalf("用法错档 stdout 应零字节，实得 %d 字节", out.Len())
	}

	// ① 干跑档零副作用（不建目录 / 不写件 / **不写审计**）
	out.Reset()
	errb.Reset()
	dry := newInv([]string{"probe"}, "探针触发", "")
	dry.dryRun = true
	if rc := cmdSkillNew(dry, &out, &errb); rc != exitOK {
		t.Fatalf("`skill new` 的干跑档应 rc=0，实得 %d（stderr=%q）", rc, errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, "probe")); !os.IsNotExist(err) {
		t.Fatal("干跑建了目录（零副作用破了）")
	}
	audit := filepath.Join(state, "skill_audit.jsonl")
	if _, err := os.Stat(audit); !os.IsNotExist(err) {
		t.Fatal("干跑写了审计（零副作用破了）")
	}

	// ③ new --yes ⇒ 0 且落件逐字节 == 模板
	out.Reset()
	errb.Reset()
	mk := newInv([]string{"probe"}, "探针触发", "")
	mk.yes = true
	if rc := cmdSkillNew(mk, &out, &errb); rc != exitOK {
		t.Fatalf("`skill new probe --yes` 应 rc=0，实得 %d（stderr=%q）", rc, errb.String())
	}
	doc := filepath.Join(root, "probe", "SKILL.md")
	created, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("new 没落件：%v", err)
	}
	if string(created) != skillTemplate("probe", "探针触发") {
		t.Fatalf("落件与模板不逐字节相同：%q", string(created))
	}

	// ④ 同名再 new ⇒ 14 且原文件字节不变
	createdSHA := skillTFileSHA(t, doc)
	out.Reset()
	errb.Reset()
	dup := newInv([]string{"probe"}, "另一个描述", "")
	dup.yes = true
	if rc := cmdSkillNew(dup, &out, &errb); rc != exitConflict {
		t.Fatalf("同名再建应 rc=14（同 object 已存在的退码），实得 %d（stderr=%q）", rc, errb.String())
	}
	if got := skillTFileSHA(t, doc); got != createdSHA {
		t.Fatalf("同名冲突却改了件：%s → %s", createdSHA, got)
	}

	// ⑤ edit --desc ⇒ 只换 description 那一行（正文逐行不变）＋ 备份 == 改前
	beforeDesc := created
	einv := &invocation{path: []string{"skill", "edit"}, args: []string{"probe"}, desc: "改过的触发器", yes: true}
	out.Reset()
	errb.Reset()
	if rc := cmdSkillEdit(einv, &out, &errb); rc != exitOK {
		t.Fatalf("`skill edit probe --desc` 应 rc=0，实得 %d（stderr=%q）", rc, errb.String())
	}
	after, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	bl, al := strings.Split(string(beforeDesc), "\n"), strings.Split(string(after), "\n")
	if len(bl) != len(al) {
		t.Fatalf("`--desc` 改了行数：%d → %d（正文被动过）", len(bl), len(al))
	}
	for i := range bl {
		if strings.HasPrefix(bl[i], "description:") { // frontmatter 的 description 行（唯一允许变的那一行）
			continue
		}
		if bl[i] != al[i] {
			t.Fatalf("`--desc` 改了第 %d 行（只许改 description）：%q → %q", i+1, bl[i], al[i])
		}
	}
	if !strings.Contains(string(after), "description: 改过的触发器") {
		t.Fatalf("描述行没换成新的：%q", string(after))
	}
	backups, _ := filepath.Glob(filepath.Join(state, "skill-backup", "*", "probe", "SKILL.md"))
	if len(backups) != 1 {
		t.Fatalf("备份应恰好 1 件，实得 %d 件", len(backups))
	}
	if b, rerr := os.ReadFile(backups[0]); rerr != nil || !bytes.Equal(b, beforeDesc) {
		t.Fatalf("备份与改前字节不同（不可逆）：%v", rerr)
	}

	// ⑥ edit --from：改名 ⇒ 2；同名字 ⇒ 0 且整篇逐字节 == 来源；名不在册 ⇒ 2
	out.Reset()
	errb.Reset()
	otherSrc := filepath.Join(t.TempDir(), "other.md")
	skillTWrite(t, otherSrc, "---\nname: someone-else\ndescription: x\n---\n\n# other\n")
	ren := &invocation{path: []string{"skill", "edit"}, args: []string{"probe"}, yes: true}
	ren.kvSet("--from", otherSrc)
	if rc := cmdSkillEdit(ren, &out, &errb); rc != exitUsage {
		t.Fatalf("`--from` 改名应 rc=2，实得 %d（stderr=%q）", rc, errb.String())
	}
	if got, _ := os.ReadFile(doc); !bytes.Equal(got, after) {
		t.Fatal("拒执却改了件")
	}

	whole := "---\nname: probe\ndescription: 整篇换过\n---\n\n# probe\n\n新正文。\n"
	wholeSrc := filepath.Join(t.TempDir(), "probe.md")
	skillTWrite(t, wholeSrc, whole)
	out.Reset()
	errb.Reset()
	winv := &invocation{path: []string{"skill", "edit"}, args: []string{"probe"}, yes: true}
	winv.kvSet("--from", wholeSrc)
	if rc := cmdSkillEdit(winv, &out, &errb); rc != exitOK {
		t.Fatalf("`skill edit probe --from` 应 rc=0，实得 %d（stderr=%q）", rc, errb.String())
	}
	if got, _ := os.ReadFile(doc); string(got) != whole {
		t.Fatalf("整篇替换后不逐字节等于来源：%q", string(got))
	}

	out.Reset()
	errb.Reset()
	miss := &invocation{path: []string{"skill", "edit"}, args: []string{"zz-no-such-skill"}, desc: "x", yes: true}
	if rc := cmdSkillEdit(miss, &out, &errb); rc != exitUsage {
		t.Fatalf("名不在册应 rc=2，实得 %d（stderr=%q）", rc, errb.String())
	}

	// ⑦ rm --yes ⇒ 0 ＋ 目录消失 ＋ 回收站那件逐字节 == 下架前
	out.Reset()
	errb.Reset()
	rinv := &invocation{path: []string{"skill", "rm"}, args: []string{"probe"}, yes: true}
	if rc := cmdSkillRm(rinv, &out, &errb); rc != exitOK {
		t.Fatalf("`skill rm probe --yes` 应 rc=0，实得 %d（stderr=%q）", rc, errb.String())
	}
	if _, err := os.Stat(filepath.Join(root, "probe")); !os.IsNotExist(err) {
		t.Fatal("rm 之后目录还在")
	}
	trashed, _ := filepath.Glob(filepath.Join(state, "skill-trash", "*", "probe", "SKILL.md"))
	if len(trashed) != 1 {
		t.Fatalf("回收站应有 1 件，实得 %d 件", len(trashed))
	}
	if b, rerr := os.ReadFile(trashed[0]); rerr != nil || string(b) != whole {
		t.Fatalf("回收站里的件与下架前不同（不可逆）：%v", rerr)
	}

	out.Reset()
	errb.Reset()
	rmiss := &invocation{path: []string{"skill", "rm"}, args: []string{"zz-no-such-skill"}, yes: true}
	if rc := cmdSkillRm(rmiss, &out, &errb); rc != exitUsage {
		t.Fatalf("名不在册下架应 rc=2，实得 %d（stderr=%q）", rc, errb.String())
	}

	// ⑧ 审计链：四行 · 事件序与 before/after sha 可核
	raw, rerr := os.ReadFile(audit)
	if rerr != nil {
		t.Fatalf("审计件读不到：%v", rerr)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 4 {
		t.Fatalf("审计应 4 行（new/edit/edit/rm），实得 %d 行：%q", len(lines), string(raw))
	}
	wantEvents := []string{"skill_new", "skill_edit", "skill_edit", "skill_rm"}
	var shas []string
	for i, ln := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(ln), &rec); err != nil {
			t.Fatalf("审计第 %d 行不是 JSON：%v", i+1, err)
		}
		if rec["event"] != wantEvents[i] {
			t.Fatalf("审计第 %d 行事件应为 %s，实得 %v", i+1, wantEvents[i], rec["event"])
		}
		if s, _ := rec["after_sha256"].(string); s != "" {
			shas = append(shas, s)
		}
	}
	if len(shas) != 3 {
		t.Fatalf("三写面各应有一枚 after_sha256（rm 那行留空），实得 %d 枚", len(shas))
	}
	if shas[0] != skillBytesSHA([]byte(skillTemplate("probe", "探针触发"))) {
		t.Fatal("审计第一行的 after_sha256 与落件内容对不上")
	}
	if shas[1] != skillBytesSHA(after) {
		t.Fatal("审计第二行（--desc）的 after_sha256 与改后字节对不上")
	}
	if shas[2] != skillBytesSHA([]byte(whole)) {
		t.Fatal("审计第三行（--from）的 after_sha256 与整篇来源对不上")
	}
}

// skillTFileSHA —— 单件 sha256（本件小工具 · 只给写面对拍用）。
func skillTFileSHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读不到 %s：%v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
