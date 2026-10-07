package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// auTestFixture —— 造一个「假仓 + 假文档仓」夹具（与 py 自检同形）。
func auTestFixture(t *testing.T) (repo, docs, reg string) {
	t.Helper()
	tmp := t.TempDir()
	repo = filepath.Join(tmp, "repo")
	docs = filepath.Join(tmp, "Zerg-内部文档")
	if err := os.MkdirAll(filepath.Join(repo, "scripts", "gates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "件.md"), []byte("头。\n唯一句甲在此。\n中间。\n唯一句乙在此。\n尾。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo, docs, filepath.Join(docs, "登记.md")
}

func auWriteReg(t *testing.T, reg, body string) {
	t.Helper()
	head := "# 合成登记件\n\n| 锚号（类型前缀） | 目标件 | 内容锚（逐字句 · 可 `grep -F`） | 备注 |\n|---|---|---|---|\n"
	if err := os.WriteFile(reg, []byte(head+body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func auRunEntry(argv ...string) (int, string, string) {
	var out, errb strings.Builder
	rc := anchorUniquenessGateMain(&out, &errb, argv)
	return rc, out.String(), errb.String()
}

// TestCheckAnchorUniquenessGatePort —— 入口正控 + 成对负控（带牙：哑化任一判据 ⇒ 必 FAIL）。
func TestCheckAnchorUniquenessGatePort(t *testing.T) {
	repo, docs, reg := auTestFixture(t)

	cases := []struct {
		name     string
		body     string
		target   string
		wantRC   int
		wantWord string
	}{
		{"正控 全唯一", "| `W-45-01` | `@docs/件.md` | 唯一句甲在此。 | a |\n| `Q-036-01` | `@docs/件.md` | 唯一句乙在此。 | b |\n", "", 0, "全唯一"},
		{"负控 锚0命中", "| `W-45-01` | `@docs/件.md` | 压根不在件里的句子。 | a |\n", "", 1, "锚 0 命中"},
		{"负控 锚多命中", "| `W-45-01` | `@docs/件.md` | 重复句在此。 | a |\n", "头。\n重复句在此。\n中间。\n重复句在此。\n尾。\n", 1, "多命中"},
		{"负控 锚号缺前缀", "| `45-01` | `@docs/件.md` | 唯一句甲在此。 | a |\n", "", 1, "缺类型前缀"},
		{"负控 锚号重号", "| `W-45-01` | `@docs/件.md` | 唯一句甲在此。 | a |\n| `W-45-01` | `@docs/件.md` | 唯一句甲在此。 | b |\n", "", 1, "重号"},
		{"负控 目标件位形态错", "| `W-45-01` | `项目文档/件.md` | 唯一句甲在此。 | a |\n", "", 1, "目标件位形态错"},
		{"负控 空转(登记0条)", "", "", 2, "不给结论"},
		{"负控 目标件不在盘", "| `W-45-01` | `@docs/没有这件.md` | 唯一句甲在此。 | a |\n", "", 2, "不给结论"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.target != "" {
				if err := os.WriteFile(filepath.Join(docs, "件.md"), []byte(c.target), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(filepath.Join(docs, "件.md"), []byte("头。\n唯一句甲在此。\n中间。\n唯一句乙在此。\n尾。\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			auWriteReg(t, reg, c.body)
			rc, out, _ := auRunEntry("--root", repo, "--docs-root", docs, "--register", reg)
			if rc != c.wantRC {
				t.Fatalf("%s：rc 得 %d 期望 %d\nout=%q", c.name, rc, c.wantRC, out)
			}
			if !strings.Contains(out, c.wantWord) {
				t.Fatalf("%s：输出缺判词 %q\nout=%q", c.name, c.wantWord, out)
			}
		})
	}

	// 去重红线：同号两行 ⇒ 必报（防「去重了但没报」的哑化）
	t.Run("负控 登记件不在盘", func(t *testing.T) {
		rc, out, _ := auRunEntry("--root", repo, "--docs-root", docs, "--register", filepath.Join(docs, "没有这件.md"))
		if rc != 2 || !strings.Contains(out, "不给结论") {
			t.Fatalf("登记件不在盘：rc=%d 期望 2\nout=%q", rc, out)
		}
	})

	// --json 面：items / out_of_domain 键齐（0 命中那条带 "hits": []，多命中带数组）
	t.Run("--json 键面", func(t *testing.T) {
		auWriteReg(t, reg, "| `W-45-01` | `@docs/件.md` | 压根不在件里的句子。 | a |\n| `Q-036-01` | `@docs/件.md` | 唯一句乙在此。 | b |\n")
		rc, out, _ := auRunEntry("--root", repo, "--docs-root", docs, "--register", reg, "--json")
		if rc != 1 {
			t.Fatalf("--json 有红：rc=%d 期望 1", rc)
		}
		for _, want := range []string{`"kind": "AnchorUniqueness"`, `"hits": []`, `"out_of_domain"`} {
			if !strings.Contains(out, want) {
				t.Fatalf("--json 输出缺 %s\nout=%q", want, out)
			}
		}
	})

	// 自检档：全过 ⇒ rc 0
	t.Run("--self-test", func(t *testing.T) {
		rc, out, _ := auRunEntry("--self-test")
		if rc != 0 {
			t.Fatalf("--self-test：rc=%d 期望 0\nout=%q", rc, out)
		}
		if !strings.Contains(out, "自检结论: 全过") {
			t.Fatalf("--self-test 结论异常：%q", out)
		}
	})
}

// TestCheckAnchorUniquenessVersionWarning —— 缺口 GAP-20260926-106 告警面：
// 登记件版本段 ≠ 现读版本真源 ⇒ 人面 / --json 皆出告警；相等 ⇒ 无告警（成对负控）。
func TestCheckAnchorUniquenessVersionWarning(t *testing.T) {
	repo, docs, _ := auTestFixture(t)
	if err := os.MkdirAll(filepath.Join(repo, "core", "internal", "version"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeVer := func(v string) {
		if err := os.WriteFile(filepath.Join(repo, "core", "internal", "version", "version.go"),
			[]byte("package version\n\nconst Version = \""+v+"\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	regV := filepath.Join(docs, "v9.9.9", "登记.md")
	if err := os.MkdirAll(filepath.Dir(regV), 0o755); err != nil {
		t.Fatal(err)
	}
	writeVer("2.5.13")
	auWriteReg(t, regV, "| `W-45-01` | `@docs/件.md` | 唯一句甲在此。 | a |\n")
	rc, out, _ := auRunEntry("--root", repo, "--docs-root", docs, "--register", regV)
	if rc != 0 {
		t.Fatalf("rc=%d want 0\nout=%q", rc, out)
	}
	if !strings.Contains(out, "⚠ 告警：登记件版本 v9.9.9 与当前版 v2.5.13 不符") {
		t.Fatalf("人面缺告警行:\n%q", out)
	}
	_, jout, _ := auRunEntry("--root", repo, "--docs-root", docs, "--register", regV, "--json")
	if !strings.Contains(jout, `"warnings": ["登记件版本 v9.9.9 与当前版 v2.5.13 不符`) {
		t.Fatalf("--json 缺 warnings:\n%q", jout)
	}
	// 负控：版本相等 ⇒ 不得出告警
	writeVer("9.9.9")
	rc2, out2, _ := auRunEntry("--root", repo, "--docs-root", docs, "--register", regV)
	if rc2 != 0 || strings.Contains(out2, "⚠ 告警：") {
		t.Fatalf("版本相等时不应告警：rc=%d out=%q", rc2, out2)
	}
}
