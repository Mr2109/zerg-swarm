package main

// registry_fanout_test.go —— 「新件登记面发散」的棘轮闸（GAP-20261020-03）
//
// 病：同一个「新增一件」的事实要在五处分别声明（公开标记.tsv / 脚本台账 / 门⑩ 干跑语义表 /
// 门步骤 / 基线），五处互不联动，也没有一处能告诉你「还缺哪一处」。今天因此被挡三次，
// 每次都要人工翻日志才知道缺哪格。
//
// 本件只做**仓内能自证的两面**（脚本台账在 Zerg-内部文档 仓外，另由 TestScriptInventoryMatchesLiveScan 管）：
//   ① `scripts/公开标记.tsv`：每件都要有一行（第一列是路径）
//   ② `core/internal/contract/dryrun-semantics.json` 的 `件[].file`：每个 .sh 都要登记
//
// 口径是**基线棘轮**：现读缺的进基线（只许缩减）。新增一件却没在这两面登记 ⇒ 红并**点名缺哪一面**。
// 首次运行若基线不存在 ⇒ 自动写一份并跳过（让这个闸能自举，不靠人先造清单）。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const fanoutBaselinePath = "testdata/registry-fanout-baseline.json"

// fanoutScripts —— 现读脚本面：scripts/ 下的 .sh 与 .py。
func fanoutScripts(t *testing.T, repoRoot string) []string {
	t.Helper()
	var out []string
	root := filepath.Join(repoRoot, "scripts")
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".sh") || strings.HasSuffix(p, ".py") {
			rel, _ := filepath.Rel(repoRoot, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫 scripts/ 失败：%v", err)
	}
	sort.Strings(out)
	return out
}

func fanoutPublicMarks(t *testing.T, repoRoot string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "公开标记.tsv"))
	if err != nil {
		t.Fatalf("读公开标记失败：%v", err)
	}
	out := map[string]bool{}
	for _, ln := range strings.Split(string(b), "\n") {
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		f := strings.Split(ln, "\t")[0]
		out[strings.TrimSpace(f)] = true
	}
	return out
}

func fanoutDryrun(t *testing.T, repoRoot string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "core", "internal", "contract", "dryrun-semantics.json"))
	if err != nil {
		t.Fatalf("读干跑语义表失败：%v", err)
	}
	var d struct {
		Items []struct {
			File string `json:"file"`
		} `json:"件"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("干跑语义表不是合法 JSON：%v", err)
	}
	out := map[string]bool{}
	for _, it := range d.Items {
		out[it.File] = true
	}
	return out
}

func TestRegistryFanout(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("定位仓根失败：%v", err)
	}
	scripts := fanoutScripts(t, repoRoot)
	marks := fanoutPublicMarks(t, repoRoot)
	dry := fanoutDryrun(t, repoRoot)

	missing := map[string]bool{}
	for _, s := range scripts {
		if !marks[s] {
			missing[s+"|公开标记"] = true
		}
		if strings.HasSuffix(s, ".sh") && !dry[s] {
			missing[s+"|干跑语义"] = true
		}
	}

	base := map[string]bool{}
	if b, err := os.ReadFile(fanoutBaselinePath); err == nil {
		var rows []string
		if jerr := json.Unmarshal(b, &rows); jerr != nil {
			t.Fatalf("基线不是字符串数组：%v", jerr)
		}
		for _, r := range rows {
			base[r] = true
		}
	} else {
		// 自举：首次运行把现读缺口写成基线，本趟跳过。
		rows := make([]string, 0, len(missing))
		for k := range missing {
			rows = append(rows, k)
		}
		sort.Strings(rows)
		buf, _ := json.MarshalIndent(rows, "", " ")
		if werr := os.WriteFile(fanoutBaselinePath, buf, 0o644); werr != nil {
			t.Fatalf("写基线失败：%v", werr)
		}
		t.Skipf("基线已自举（%d 条缺口）⇒ 本趟跳过，从下一趟起生效", len(rows))
	}

	var added []string
	for k := range missing {
		if !base[k] {
			added = append(added, k)
		}
	}
	sort.Strings(added)
	if len(added) > 0 {
		t.Fatalf("新增件没在登记面表态 %d 条（基线 %d → 现读 %d）：\n  %s\n"+
			"改法：① 公开标记.tsv 加一行（第一列=相对路径）；② .sh 还要进 dryrun-semantics.json 的 件[]。",
			len(added), len(base), len(missing), strings.Join(added, "\n  "))
	}
	t.Logf("登记面：脚本 %d 件 · 现读缺口 %d 条 · 基线 %d 条 · 新增 0", len(scripts), len(missing), len(base))
}
