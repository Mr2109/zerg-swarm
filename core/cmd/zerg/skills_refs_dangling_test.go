package main

// skills_refs_dangling_test.go —— 技能库死指针闸（GAP-20260926-48）
//
// 病：仓内死指针门（check-dangling-refs）只扫仓，**不扫技能库** ⇒ ~/.hermes/skills 里
// 指向仓内件的引用烂掉（件改名/搬走/删掉）没有任何闸会红。
//
// 做法（刻意**不加新命令面**，躲开「新命令要五处登记」那条发散）：
//   · 本件是判据件，随 `go test ./cmd/zerg` 自动跑 —— 贵步/受影响包两条路都覆盖到；
//   · 根：$ZERG_SKILLS_ROOT，缺省 ~/.hermes/skills；本机没有 ⇒ t.Skip（别的机器不误红）；
//   · 判据：每处 `前缀/…` 反引号引用，只要**仓根**或**该技能自己的目录**下存在 ⇒ 不是死指针；
//   · 基线：testdata/skills-refs-baseline.json（**只许缩减**）。新出现的死指针 ⇒ 红并逐条点名；
//     已修掉的 ⇒ 只提示「可以下调基线」，不当红（否则修一件就得改一次基线，修不动）。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var skillsRefRE = regexp.MustCompile("`((?:scripts|core|agent|ui|wall|gateway|docs)/[A-Za-z0-9_./\\-]+)`")

func skillsRootForTest() string {
	if v := os.Getenv("ZERG_SKILLS_ROOT"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".hermes", "skills")
}

// skillsDeadRefs —— 现读死指针集（"<技能相对路径>|<引用>" ⇒ true）。
func skillsDeadRefs(t *testing.T, root, repoRoot string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		dir := filepath.Dir(p)
		for _, m := range skillsRefRE.FindAllStringSubmatch(string(b), -1) {
			ref := m[1]
			if _, e := os.Stat(filepath.Join(repoRoot, ref)); e == nil {
				continue
			}
			if _, e := os.Stat(filepath.Join(dir, ref)); e == nil {
				continue
			}
			out[rel+"|"+ref] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫技能库失败：%v", err)
	}
	return out
}

func loadSkillsBaseline(t *testing.T, path string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读基线失败（%s）：%v", path, err)
	}
	var rows []string
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("基线不是字符串数组：%v", err)
	}
	out := map[string]bool{}
	for _, r := range rows {
		out[r] = true
	}
	return out
}

func TestSkillsRefsNoNewDangling(t *testing.T) {
	root := skillsRootForTest()
	if root == "" {
		t.Skip("找不到技能库根")
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		t.Skip("本机没有技能库（" + root + "）⇒ 不适用")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("定位仓根失败：%v", err)
	}
	now := skillsDeadRefs(t, root, repoRoot)
	base := loadSkillsBaseline(t, filepath.Join("testdata", "skills-refs-baseline.json"))

	var added []string
	for k := range now {
		if !base[k] {
			added = append(added, k)
		}
	}
	var fixed []string
	for k := range base {
		if !now[k] {
			fixed = append(fixed, k)
		}
	}
	sort.Strings(added)
	sort.Strings(fixed)

	if len(fixed) > 0 {
		t.Logf("↓ 已修掉 %d 条（基线可下调）：%v", len(fixed), fixed[:min(len(fixed), 5)])
	}
	if len(added) > 0 {
		t.Fatalf("技能库新增死指针 %d 条（基线 %d → 现读 %d）——\n  %s\n"+
			"改法二选一：① 把技能里的引用指回现存件；② 确属示例/占位 ⇒ 从技能正文去掉反引号或写全限定名。",
			len(added), len(base), len(now), strings.Join(added[:min(len(added), 20)], "\n  "))
	}
	t.Logf("技能库引用面：现读死指针 %d 条 · 基线 %d 条 · 新增 0", len(now), len(base))
}
