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
