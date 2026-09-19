package chat

// chat_docs_root_test.go — E1/E2「<项目文档> 取源根双认 + 缺件可观测」用例
// （2026-09-19「开发文档分家」批4a；本批收掉的是「读点静默失效」）。
//
// 钉住三条契约（各自可变异自证）：
//   D1 **仓内优先**：仓内与仓外取源根同时有版本目录 ⇒ zerg_overview 取仓内那份；
//   D2 **缺则仓外**：仓内缺、仓外有 ⇒ latestVersionDir() 与 doc_search **都真的读到仓外那份**
//      （变异：把 zergDocsBase() 改回「只认仓内 docs/项目文档」⇒ 必红）；
//   D3 **缺件不静默**：两处取源根都不在盘上 ⇒ zerg_overview 的全景要点名两个候选根、
//      doc_search 要**报缺件错**而不是回「无命中」
//      （变异：删掉 doc_search 的「根全缺 ⇒ return error」闸 ⇒ 必红）。
//
// 隔离纪律：ZERG_WORKSPACE / ZERG_DOCS_ALT 一律指到 t.TempDir()；ZergRepoRoot 包变量临时换掉
// （照 chat_paths_hardcode_test.go 的 isolateLegacyChatDB 先例）⇒ 不读真机 <仓库>/docs/常青、
// 也不依赖真机 Zerg-内部文档 在不在盘上（用例结论只由夹具决定）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateDocsRoots — 把「仓库根 / 仓外根」双双隔离到临时目录，并临时换掉 ZergRepoRoot 包变量。
// 返回 (假仓库根, 假仓外根)；两处都没建 项目文档/ 与 docs/常青 —— 由各用例按需造。
func isolateDocsRoots(t *testing.T) (repo, alt string) {
	t.Helper()
	repo = filepath.Join(t.TempDir(), "repo")
	alt = filepath.Join(t.TempDir(), "Zerg-内部文档")
	for _, d := range []string{repo, alt} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ZERG_WORKSPACE", repo)
	t.Setenv("ZERG_DOCS_ALT", alt)
	old := ZergRepoRoot
	ZergRepoRoot = repo
	t.Cleanup(func() { ZergRepoRoot = old })
	return repo, alt
}

// writeDocsFixture — 在 **base/项目文档/<ver>/** 下造 n 篇 md（内容含 marker，供命中检索），
// 返回该版本目录。★ 注意 base = **「项目文档」的父目录**：仓库侧是 `<仓库根>/docs`，
// 仓外侧是 Zerg-内部文档 根 `<ZERG_DOCS_ALT>`（取源根两侧都叫 `<base>/项目文档`）。
// n>=3 是为了过 latestVersionDir() 的「<3 篇 = 空壳」守卫。
func writeDocsFixture(t *testing.T, base, ver, marker string, n int) string {
	t.Helper()
	dir := filepath.Join(base, "项目文档", ver)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		p := filepath.Join(dir, "doc"+itoa(i)+".md")
		if err := os.WriteFile(p, []byte("# "+ver+" 架构\n"+marker+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// D1 仓内优先：两处都有 ⇒ 取仓内版本目录，全景读仓内那份。
func TestDocsRoot_D1_RepoRootWinsWhenBothPresent(t *testing.T) {
	repo, alt := isolateDocsRoots(t)
	// 仓库侧 base = <仓库根>/docs（取源根 = <仓库根>/docs/项目文档）
	repoDir := writeDocsFixture(t, filepath.Join(repo, "docs"), "v9.9.9", "REPO-MARKER-d1", 3)
	writeDocsFixture(t, alt, "v9.9.9", "ALT-MARKER-d1", 3)

	if got := zergDocsBase(); got != filepath.Join(repo, "docs", "项目文档") {
		t.Fatalf("取源根应取仓内，实得 %q", got)
	}
	if got := latestVersionDir(); got != repoDir {
		t.Fatalf("latestVersionDir 应取仓内 %s，实得 %q", repoDir, got)
	}
	out, err := zergOverviewFull()
	if err != nil {
		t.Fatalf("全景失败: %v", err)
	}
	if !strings.Contains(out, "v9.9.9") {
		t.Fatalf("全景未反映仓内版本目录 v9.9.9: %q", out)
	}
}

// D2 缺则仓外：仓内缺 ⇒ zerg_overview 与 doc_search 都必须真的读到仓外那份。
// 变异：zergDocsBase() 改回「只认仓内 docs/项目文档」⇒ 下面两条断言全红。
func TestDocsRoot_D2_FallsBackToAltRoot(t *testing.T) {
	repo, alt := isolateDocsRoots(t) // 仓内**没有** docs/项目文档
	altDir := writeDocsFixture(t, alt, "v9.9.9", "ALT-ONLY-MARKER-d2", 3)

	if got := latestVersionDir(); got != altDir {
		t.Fatalf("仓内缺 ⇒ 应取仓外 %s，实得 %q（只认仓内 = 读点静默失效）", altDir, got)
	}
	out, err := docSearch(map[string]any{"query": "ALT-ONLY-MARKER-d2"}, "")
	if err != nil {
		t.Fatalf("doc_search 报错: %v", err)
	}
	if !strings.Contains(out, "ALT-ONLY-MARKER-d2") {
		t.Fatalf("doc_search 未在仓外取源根命中（实得 %q）；仓根=%s 仓外根=%s", out, repo, alt)
	}
	if !strings.Contains(out, filepath.Join("项目文档", "v9.9.9")) {
		t.Fatalf("展示路径应形如 项目文档/v9.9.9/…（排序键与「当前版」判定依赖它）: %q", out)
	}
	if !strings.Contains(out, "★当前版") {
		t.Fatalf("仓外取源根下的最新版应被标 ★当前版（路径前缀判定失效即此处红）: %q", out)
	}
}

// D3 缺件不静默：两处取源根都不在盘上 ⇒ 报因，不许给「空的正常结果」。
// 变异：删掉 doc_search 的「根全缺 ⇒ return error」闸 ⇒ 静默回「无命中」⇒ 必红。
func TestDocsRoot_D3_MissingRootsAreObservable(t *testing.T) {
	repo, alt := isolateDocsRoots(t) // 两处都没有 项目文档/

	if got := zergDocsBase(); got != "" {
		t.Fatalf("两处都缺时应返回空串（不猜），实得 %q", got)
	}
	if got := latestVersionDir(); got != "" {
		t.Fatalf("两处都缺时 latestVersionDir 应为空，实得 %q", got)
	}

	repoCand := filepath.Join(repo, "docs", "项目文档")
	altCand := filepath.Join(alt, "项目文档")

	// ① zerg_overview：全景的「文档」节必须点名缺件（两个候选根都要在）
	out, err := zergOverviewFull()
	if err != nil {
		t.Fatalf("全景失败: %v", err)
	}
	for _, want := range []string{"取源根未找到", repoCand, altCand} {
		if !strings.Contains(out, want) {
			t.Errorf("全景未点名缺件信息 %q——缺件被静默了。全景: %q", want, out)
		}
	}

	// ② doc_search：必须是**错误**（判据不可判），不是「无命中」这个看似正常的空结果
	_, derr := docSearch(map[string]any{"query": "任何关键词-d3"}, "")
	if derr == nil {
		t.Fatal("两处取源根都缺时 doc_search 必须报缺件错，不许静默回「无命中」（变异即此处红）")
	}
	if !strings.Contains(derr.Error(), "取源根缺失") || !strings.Contains(derr.Error(), "仓外根") {
		t.Fatalf("缺件错误信息应点名「取源根缺失」与「仓外根」，实得: %v", derr)
	}
}
