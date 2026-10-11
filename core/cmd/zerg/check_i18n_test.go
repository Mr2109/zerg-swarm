package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ci18fx —— 造一枚仓外夹具（root 显式传给 ci18Run，不走 cwd 推导）。
func ci18fx(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
			t.Fatalf("mkfixture mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o666); err != nil {
			t.Fatalf("mkfixture write %s: %v", rel, err)
		}
	}
	return root
}

func ci18green() map[string]string {
	return map[string]string{
		"ui/locales/zh-CN.yml":               "hello: 你好\ngreet: 你好 %{name}\napierr.NOT_FOUND: 未找到\n",
		"ui/locales/en.yml":                  "hello: Hello\ngreet: Hi %{name}\napierr.NOT_FOUND: Not found\n",
		"ui/src/main.rs":                     "fn main_view() {\n    let s = t!(\"hello\");\n    let g = t!(\"greet\");\n}\n",
		"core/internal/api/x.go":             "func h(w http.ResponseWriter) {\n\twriteErrorCode(w, r, \"NOT_FOUND\")\n}\n",
		"ui/scripts/i18n-baseline.json":      `{"allowed_count": 0, "allowed": []}` + "\n",
		"scripts/gates/baseline-audit.jsonl": "",
	}
}

func ci18run(t *testing.T, root string, argv []string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := ci18Run(root, &out, &errb, argv)
	return rc, out.String(), errb.String()
}

func TestCheckI18nGateMainGreen(t *testing.T) {
	root := ci18fx(t, ci18green())
	rc, out, errs := ci18run(t, root, nil)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0 (stderr=%q)", rc, errs)
	}
	if !strings.Contains(out, "  键数 zh=3 en=3 ｜ 静态引用 2 ｜ 剩余中文字面量 0 行（基线 0）") {
		t.Errorf("计数行不符:\n%s", out)
	}
	if !strings.Contains(out, "→ 门禁通过") {
		t.Errorf("缺通过行:\n%s", out)
	}
}

func TestCheckI18nG1KeyAsymmetry(t *testing.T) {
	f := ci18green()
	f["ui/locales/zh-CN.yml"] = f["ui/locales/zh-CN.yml"] + "only_zh_key: 只有中\n"
	root := ci18fx(t, f)
	rc, out, _ := ci18run(t, root, nil)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if !strings.Contains(out, "G1 仅 zh-CN 有（1）：only_zh_key") {
		t.Errorf("G1 明细不符:\n%s", out)
	}
	if !strings.Contains(out, "→ 门禁失败（1 项）") {
		t.Errorf("缺失败行:\n%s", out)
	}
}

func TestCheckI18nG2UndefinedRef(t *testing.T) {
	f := ci18green()
	f["ui/src/main.rs"] = f["ui/src/main.rs"] + "fn extra() { let x = t!(\"undefined_key\"); }\n"
	rc, out, _ := ci18fxRun(t, f)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if !strings.Contains(out, "G2 代码引用但未定义（1）：undefined_key") {
		t.Errorf("G2 明细不符:\n%s", out)
	}
}

// ci18fxRun —— 造夹具 + 跑默认面（省重复）。
func ci18fxRun(t *testing.T, f map[string]string) (int, string, string) {
	t.Helper()
	return ci18run(t, ci18fx(t, f), nil)
}

func TestCheckI18nG4UnregisteredLiteral(t *testing.T) {
	f := ci18green()
	f["ui/src/modules/z.rs"] = "fn f() {\n    ui.label(\"未登记的文案\");\n}\n"
	rc, out, _ := ci18fxRun(t, f)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if !strings.Contains(out, "G4 出现未登记的中文 UI 字面量（1 行") {
		t.Errorf("G4 明细不符:\n%s", out)
	}
	if !strings.Contains(out, "ui/src/modules/z.rs:2 f | ui.label(\"未登记的文案\");") {
		t.Errorf("G4 行明细不符:\n%s", out)
	}
}

func TestCheckI18nListMode(t *testing.T) {
	f := ci18green()
	f["ui/src/modules/z.rs"] = "fn f() {\n    ui.label(\"未登记的文案\");\n}\n"
	rc, out, _ := ci18run(t, ci18fx(t, f), []string{"--list"})
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	want := "剩余中文字面量 1 行（基线允许 0 行）：\n  [★新增] ui/src/modules/z.rs:2 f | ui.label(\"未登记的文案\");\n"
	if out != want {
		t.Errorf("--list 输出不符:\n got=%q\nwant=%q", out, want)
	}
}

func TestCheckI18nJSONMode(t *testing.T) {
	f := ci18green()
	f["ui/src/modules/z.rs"] = "fn f() {\n    ui.label(\"未登记的文案\");\n}\n"
	rc, out, _ := ci18run(t, ci18fx(t, f), []string{"--json"})
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	var doc struct {
		Counts map[string]any `json:"counts"`
		Errors []string       `json:"errors"`
		Extra  []string       `json:"extra"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("--json 非合法 JSON: %v\n%s", err, out)
	}
	if n, _ := doc.Counts["chinese_literals"].(float64); n != 1 {
		t.Errorf("chinese_literals = %v, want 1", doc.Counts["chinese_literals"])
	}
	if len(doc.Extra) != 1 || !strings.Contains(doc.Extra[0], "ui/src/modules/z.rs:2 |") {
		t.Errorf("extra 不符: %#v", doc.Extra)
	}
}

func TestCheckI18nMissingInputRC2(t *testing.T) {
	root := ci18fx(t, map[string]string{"ui/locales/en.yml": "a: b\n"}) // zh-CN.yml 缺
	rc, _, errs := ci18run(t, root, nil)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2 (取不到输入 · fail-closed)", rc)
	}
	if !strings.Contains(errs, "zh-CN.yml") {
		t.Errorf("stderr 缺具名文案: %q", errs)
	}
}

func TestCheckI18nTSVRc2NoDocsRoot(t *testing.T) {
	root := ci18fx(t, ci18green())
	t.Setenv("ZERG_I18N_AUDIT_OUT", "")
	t.Setenv("ZERG_DOCS_ALT", filepath.Join(root, "nonexistent-alt"))
	rc, out, _ := ci18run(t, root, []string{"--tsv"})
	if rc != 2 {
		t.Fatalf("rc = %d, want 2", rc)
	}
	if !strings.Contains(out, "输出目录不可判（不给结论）· rc=2") {
		t.Errorf("缺不给结论文案:\n%s", out)
	}
}

func TestCheckI18nTSVWritesAndByteStable(t *testing.T) {
	f := ci18green()
	f["ui/src/modules/z.rs"] = "fn f() {\n    ui.label(\"未登记的文案\");\n}\n"
	root := ci18fx(t, f)
	outDir := filepath.Join(root, "outdir")
	t.Setenv("ZERG_I18N_AUDIT_OUT", outDir)
	rc, out, _ := ci18run(t, root, []string{"--tsv"})
	if rc != 1 {
		t.Fatalf("rc = %d, want 1 (有未登记字面量)", rc)
	}
	tsv := filepath.Join(outDir, "L2-现状-中文字面量.tsv")
	b, err := os.ReadFile(tsv)
	if err != nil {
		t.Fatalf("读输出 TSV: %v", err)
	}
	want := "状态\t文件\t行号\t函数\t代码\n★未登记\tui/src/modules/z.rs\t2\tf\tui.label(\"未登记的文案\");\n"
	if string(b) != want {
		t.Errorf("TSV 字节不符:\n got=%q\nwant=%q", string(b), want)
	}
	if !strings.Contains(out, "写出: ") || !strings.Contains(out, "（1 行）") {
		t.Errorf("写出行不符:\n%s", out)
	}
}

func TestCheckI18nDeclaredCountMissing(t *testing.T) {
	f := ci18green()
	f["ui/scripts/i18n-baseline.json"] = `{"allowed": []}` + "\n" // 缺 allowed_count
	rc, out, _ := ci18fxRun(t, f)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if !strings.Contains(out, "G4 基线件缺 `allowed_count` 声明数（现 None）") {
		t.Errorf("方向判据无锚明细不符:\n%s", out)
	}
}

func TestCheckI18nLedgerMismatch(t *testing.T) {
	f := ci18green()
	f["ui/scripts/i18n-baseline.json"] = `{"allowed_count": 5, "allowed": []}` + "\n"
	f["scripts/gates/baseline-audit.jsonl"] = `{"name": "i18n.allowed_count", "old": 3, "new": 4}` + "\n"
	rc, out, _ := ci18fxRun(t, f)
	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if !strings.Contains(out, "G4 **放宽未登记**：声明数 5 与棘轮台账") {
		t.Errorf("放宽未登记明细不符:\n%s", out)
	}
}

func TestCheckI18nDeclaredGtAllowedWarns(t *testing.T) {
	f := ci18green()
	f["ui/scripts/i18n-baseline.json"] = `{"allowed_count": 5, "allowed": []}` + "\n"
	f["scripts/gates/baseline-audit.jsonl"] = `{"name": "i18n.allowed_count", "old": 4, "new": 5}` + "\n"
	rc, out, _ := ci18fxRun(t, f)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0（收的方向不拦，只提示）", rc)
	}
	if !strings.Contains(out, "G4 声明数 5 > 白名单实条数 0（收的方向不拦，只提示）") {
		t.Errorf("提示行不符:\n%s", out)
	}
}

// TestCheckI18nReadFailTeeth —— B 类读失败不再静默的回归牙：core/internal/api/ 下一件 .go 锁成
// chmod 000 ⇒ 登记面（ci18ReadFails）必响：rc=2 + stderr BLOCKED 头 + 逐件点名。
// 删回静默（原 os.ReadFile 失败即 continue）本用例必红。
func TestCheckI18nReadFailTeeth(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 无视权限位（chmod 000 仍可读）⇒ 跳过")
	}
	f := ci18green()
	f["core/internal/api/locked.go"] = "func z(w http.ResponseWriter) {\n\twriteErrorCode(w, r, \"NOT_FOUND\")\n}\n"
	root := ci18fx(t, f)
	locked := filepath.Join(root, "core", "internal", "api", "locked.go")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod 000 失败：%v", err)
	}
	defer func() { _ = os.Chmod(locked, 0o644) }()
	rc, _, errb := ci18run(t, root, nil)
	if rc != 2 || !strings.Contains(errb, "BLOCKED") || !strings.Contains(errb, "core/internal/api/locked.go") {
		t.Fatalf("读不到件应 rc=2 + BLOCKED 点名 locked.go，得 rc=%d err=%q", rc, errb)
	}
}
