package statepath

// docs_base_test.go — E1/E2「<项目文档> 取源根双认」用例（2026-09-19「开发文档分家」批4a）。
//
// 钉住三条契约（各自可变异自证）：
//   ① **仓内优先**：<仓库根>/docs/项目文档 在盘上 ⇒ 取仓内（未分家的机器 / 公开树行为一字不变）；
//   ② **缺则仓外**：仓内缺 ⇒ 取仓外 <ZERG_DOCS_ALT|../Zerg-内部文档>/项目文档；
//   ③ **缺件不静默**：两处都不在盘上 ⇒ 返回 ""（不猜、不假装有），
//      且 DocsBaseMissingNote() 必须**逐个候选点名绝对路径**（现场能判因，而不是看到空白）。
//
// 变异自证：
//   · 删掉②的仓外候选（只认仓内 docs/项目文档）⇒ TestDocsBase_FallsBackToAltRoot 必红；
//   · 把①的 os.Stat 存在性判定去掉（无脑取第一个候选）⇒ TestDocsBase_NoGuessWhenBothMissing 必红。
//
// 隔离纪律：ZERG_WORKSPACE / ZERG_DOCS_ALT 一律指到 t.TempDir()，不碰真机仓库与真机 Zerg-内部文档。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDocsBase_RepoRootWinsWhenBothPresent — ①：两处都在盘上 ⇒ 取仓内（优先级不能反）。
func TestDocsBase_RepoRootWinsWhenBothPresent(t *testing.T) {
	repo := t.TempDir()
	alt := t.TempDir()
	t.Setenv("ZERG_WORKSPACE", repo)
	t.Setenv("ZERG_DOCS_ALT", alt)

	repoBase := filepath.Join(repo, "docs", "项目文档")
	altBase := filepath.Join(alt, "项目文档")
	for _, p := range []string{repoBase, altBase} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := DocsBase(); got != repoBase {
		t.Fatalf("仓内优先：期望 %s，实得 %q", repoBase, got)
	}
}

// TestDocsBase_FallsBackToAltRoot — ②：仓内缺、仓外有 ⇒ 取仓外（分家后本机的真实形态）。
// 变异：只认仓内候选 ⇒ 返回 "" ⇒ 必红。
func TestDocsBase_FallsBackToAltRoot(t *testing.T) {
	repo := t.TempDir() // 仓内**没有** docs/项目文档
	alt := t.TempDir()
	t.Setenv("ZERG_WORKSPACE", repo)
	t.Setenv("ZERG_DOCS_ALT", alt)

	altBase := filepath.Join(alt, "项目文档")
	if err := os.MkdirAll(altBase, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := DocsBase(); got != altBase {
		t.Fatalf("仓内缺 ⇒ 应取仓外 %s，实得 %q（只认仓内 = 分家后静默失效）", altBase, got)
	}
}

// TestDocsBase_AltDefaultIsSiblingZergDocs — 仓外根默认 = <仓库根>/../Zerg-内部文档
// （与 docs/site/export-and-build.sh 的 ZERG_DOCS_ALT 同根同默认），候选顺序 = [仓内, 仓外]。
func TestDocsBase_AltDefaultIsSiblingZergDocs(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("ZERG_WORKSPACE", repo)
	t.Setenv("ZERG_DOCS_ALT", "") // 未显式覆盖

	want := filepath.Join(repo, "..", "Zerg-内部文档")
	if got := DocsAltRoot(); got != want {
		t.Fatalf("仓外根默认应为 <仓库根>/../Zerg-内部文档：期望 %s，实得 %q", want, got)
	}
	c := DocsBaseCandidates()
	if len(c) != 2 {
		t.Fatalf("候选应恰为 2 个（仓内/仓外），实得 %v", c)
	}
	if c[0] != filepath.Join(repo, "docs", "项目文档") || c[1] != filepath.Join(want, "项目文档") {
		t.Fatalf("候选顺序应为 [仓内, 仓外]：%v", c)
	}
}

// TestDocsBase_NoGuessWhenBothMissing — ③：两处都缺 ⇒ "" + 缺件说明点名两个候选。
// 变异：把存在性判定去掉（无脑返回第一个候选）⇒ 返回非空 ⇒ 必红。
func TestDocsBase_NoGuessWhenBothMissing(t *testing.T) {
	repo := t.TempDir()
	alt := t.TempDir() // 目录在，但两处都**没有** 项目文档/
	t.Setenv("ZERG_WORKSPACE", repo)
	t.Setenv("ZERG_DOCS_ALT", alt)

	if got := DocsBase(); got != "" {
		t.Fatalf("两处都缺应返回空串（不猜、不假装有、不回退别处），实得 %q", got)
	}
	note := DocsBaseMissingNote()
	for _, want := range []string{
		"取源根未找到",
		filepath.Join(repo, "docs", "项目文档"),
		filepath.Join(alt, "项目文档"),
	} {
		if !strings.Contains(note, want) {
			t.Fatalf("缺件说明必须点名 %q，实得 %q", want, note)
		}
	}
}
