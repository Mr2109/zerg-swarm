// kb_docs_sync_test.go —— `kb_docs_sync.go`（`tools/kb_docs_sync.py` 的 Go 真搬）**成对负控**。
//
// 断言全是「只有实现对才过」的逐字等式（哑化任一语义面必红 —— 见回执的仓外副本哑化实证：
// 改坏主件语义 ⇒ 本件 FAIL 带行号）＋ 空值/缺输入/「取不到 vs 空」两态各成一子用例。
//
// ★ 无 TestMain / 无包级 init（撞名规避，照 `gen_gap_ledger_test.go` 同规）；helper 一律 `kbs` 前缀。
// ★ 夹具全在 `t.TempDir()`（仓外），✗ 不碰真仓、✗ 不写真实知识库目录。
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── 夹具 ─────────────────────────────────────────────────────────────────

func kbsTestRepo(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	kb := filepath.Join(base, "kb")
	for _, d := range []string{repo, kb} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

// kbsTestFixture —— `<base>/repo` ＋ 兄弟 `<base>/Zerg-内部文档/项目文档/v9.9.9/<docs>` ＋ `<base>/kb`。
func kbsTestFixture(t *testing.T, docs map[string]string) (repo, kb, verDir string) {
	t.Helper()
	repo = kbsTestRepo(t)
	base := filepath.Dir(repo)
	kb = filepath.Join(base, "kb")
	verDir = filepath.Join(base, "Zerg-内部文档", "项目文档", "v9.9.9")
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n, body := range docs {
		if err := os.WriteFile(filepath.Join(verDir, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo, kb, verDir
}

func kbsTestRun(t *testing.T, repo, kb string, argv []string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	rc := kbsDispatch(&out, &errb, argv, repo, kb, kbsScan(argv))
	return rc, out.String(), errb.String()
}

func kbsWriteRegistry(t *testing.T, kb, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(kb, kbsRegistryName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func kbsRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s: %v", path, err)
	}
	return string(b)
}

// ── py pathlib/posixpath 同构 ────────────────────────────────────────────

func TestKbsPathSemantics(t *testing.T) {
	if got := kbsPyJoin("a", "b"); got != "a/b" {
		t.Errorf("kbsPyJoin(a,b) = %q, want a/b", got)
	}
	// 后段绝对路径丢弃前段（py 语义 · ✗ filepath.Join）
	if got := kbsPyJoin("repo", "/abs"); got != "/abs" {
		t.Errorf("kbsPyJoin(repo,/abs) = %q, want /abs", got)
	}
	// 空段：py join('a','')=='a/' · join('','a')=='a'
	if got := kbsPyJoin("a", ""); got != "a/" {
		t.Errorf("kbsPyJoin(a,'') = %q, want a/", got)
	}
	if got := kbsPyJoin("", "a"); got != "a" {
		t.Errorf("kbsPyJoin('',a) = %q, want a", got)
	}
	// normpath：CPython 保留恰两个前导斜杠
	if got := kbsPyNormpath("//a/b/../c"); got != "//a/c" {
		t.Errorf("kbsPyNormpath(//a/b/../c) = %q, want //a/c", got)
	}
	if got := kbsPyNormpath("///a/b"); got != "/a/b" {
		t.Errorf("kbsPyNormpath(///a/b) = %q, want /a/b", got)
	}
	if got := kbsPyNormpath(""); got != "." {
		t.Errorf("kbsPyNormpath('') = %q, want .", got)
	}
	if got := kbsPyNormpath("a/../../b"); got != "../b" {
		t.Errorf("kbsPyNormpath(a/../../b) = %q, want ../b", got)
	}
	if got := kbsPyBasename("/x/y/z.md"); got != "z.md" {
		t.Errorf("kbsPyBasename = %q, want z.md", got)
	}
	if got := kbsPyBasename("z.md"); got != "z.md" {
		t.Errorf("kbsPyBasename(裸名) = %q, want z.md", got)
	}
	if got := kbsPyRelpath("/a/b/c/d.md", "/a/b"); got != "c/d.md" {
		t.Errorf("kbsPyRelpath = %q, want c/d.md", got)
	}
	if got := kbsPyRelpath("/a/b/c/d.md", "/x/b"); got != "../../a/b/c/d.md" {
		t.Errorf("kbsPyRelpath(异根) = %q, want ../../a/b/c/d.md", got)
	}
	if got := kbsPyRelpath("/a/b", "/a/b"); got != "." {
		t.Errorf("kbsPyRelpath(同) = %q, want .", got)
	}
}

// ── py 字符串原语 ────────────────────────────────────────────────────────

func TestKbsStringPrimitives(t *testing.T) {
	// py str.strip() 剥 `\x1c`（Go unicode.IsSpace 不剥）＋ NEL/NBSP（须用 Unicode 转义：裸字节非合法 UTF-8）
	if got := kbsPyStrip("\x1c\u00a0 x \u0085\x1f"); got != "x" {
		t.Errorf("kbsPyStrip = %q, want x", got)
	}
	if got := kbsPyLStripSet("### 标题 x", "# "); got != "标题 x" {
		t.Errorf("kbsPyLStripSet = %q, want 标题 x", got)
	}
	if got := kbsPyLStripSet("标题", "# "); got != "标题" {
		t.Errorf("kbsPyLStripSet(无前缀) = %q, want 标题", got)
	}
	// universal newlines：`\r\n` 与裸 `\r` 都折 `\n`
	if got := kbsUniversalNewlines("a\r\nb\rc\n"); got != "a\nb\nc\n" {
		t.Errorf("kbsUniversalNewlines = %q", got)
	}
}

func TestKbsFnmatch(t *testing.T) {
	cases := []struct {
		n, p string
		want bool
	}{
		{"00-架构-总.md", "00-架构*.md", true},
		{"变更-2026.md", "变更-*.md", true},
		{"00-架构.md", "00-架构*.md", true}, // `*` 可空
		{"00-架构-总.md.bak", "00-架构*.md", false},
		{"xx.md", "00-架构*.md", false},
		{"ab", "a?", true},
		{"abc", "a?", false},
	}
	for _, c := range cases {
		if got := kbsFnmatch(c.n, c.p); got != c.want {
			t.Errorf("kbsFnmatch(%q,%q) = %v, want %v", c.n, c.p, got, c.want)
		}
	}
}

// ── 取不到输入 ⇒ rc=2（两态各成一子用例）────────────────────────────────

func TestKbsNoDocsRootRC2(t *testing.T) {
	repo := kbsTestRepo(t)
	kb := filepath.Join(filepath.Dir(repo), "kb")
	rc, out, errb := kbsTestRun(t, repo, kb, nil)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2", rc)
	}
	want := "未找到当前版本目录——两处取源根都不在盘上：\n" +
		"  " + kbsPyJoin(repo, "docs", kbsDocsSub) + "\n" +
		"  " + kbsPyAbspath(kbsPyJoin(repo, "..", kbsZergDocsDir, kbsDocsSub)) + "\n" +
		"⇒ 判据不可判（不给结论）· rc=2\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if errb != "" {
		t.Errorf("stderr = %q, want 空", errb)
	}
}

func TestKbsNoVersionDirRC2(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	kb := filepath.Join(base, "kb")
	for _, d := range []string{repo, kb, filepath.Join(base, "Zerg-内部文档", "项目文档", "notaversion")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// 取源根在、但目录名不是 `v\d+\.\d+\.\d+` ⇒ 版本线判不出 ⇒ 同一 rc=2 档
	rc, out, _ := kbsTestRun(t, repo, kb, nil)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2（取源根在但无版本线目录）", rc)
	}
	if !strings.Contains(out, "⇒ 判据不可判（不给结论）· rc=2") {
		t.Errorf("stdout = %q", out)
	}
}

// ── dry-run：产物逐字/逐字节 ─────────────────────────────────────────────

func TestKbsDryRunCandidatesByteExact(t *testing.T) {
	repo, kb, verDir := kbsTestFixture(t, map[string]string{
		"00-架构-总.md": "# 标题 00-架构-总.md\n\n正文行 0\n",
		"变更-2026.md": "# 变更标题\n\n行一\n行二\n",
		"无关件.md":     "# 不该进\n\nxx\n", // 不在 glob 白名单 ⇒ 不计
	})
	rc, out, errb := kbsTestRun(t, repo, kb, nil)
	if rc != 0 || errb != "" {
		t.Fatalf("rc=%d err=%q", rc, errb)
	}
	d00 := kbsPyRelpath(filepath.Join(verDir, "00-架构-总.md"), repo)
	dch := kbsPyRelpath(filepath.Join(verDir, "变更-2026.md"), repo)
	wantOut := "kb_docs_sync: 当前版 v9.9.9——关键文档 2 份（新增 2/变更 0/未变 0）\n" +
		"  [新增] " + d00 + "\n" +
		"  [新增] " + dch + "\n" +
		"\n候选条目 → " + filepath.Join(kb, kbsCandidatesName) + "\n" +
		"下一步（核验门控——G2）:\n" +
		"  · " + d00 + " —— kb_add title=<标题> domain=虫族 file_path=" + d00 + " content=<精炼要点>\n" +
		"  · " + dch + " —— kb_add title=<标题> domain=虫族 file_path=" + dch + " content=<精炼要点>\n" +
		"全部入库后执行: python3 kb_docs_sync.py --mark 全部  （更新注册表——下次只检测真变更）\n"
	if out != wantOut {
		t.Errorf("stdout 不等\n got=%q\nwant=%q", out, wantOut)
	}
	wantJSON := "[\n {\n  \"title\": \"标题 00-架构-总.md\",\n" +
		"  \"file_path\": \"" + d00 + "\",\n  \"domain\": \"虫族\",\n  \"source\": \"项目文档\",\n" +
		"  \"content_preview\": \"正文行 0\",\n" +
		"  \"kb_add_note\": \"kb_add title=<标题> domain=虫族 file_path=" + d00 + " content=<精炼要点>\",\n" +
		"  \"change\": \"新增\"\n },\n {\n  \"title\": \"变更标题\",\n" +
		"  \"file_path\": \"" + dch + "\",\n  \"domain\": \"虫族\",\n  \"source\": \"项目文档\",\n" +
		"  \"content_preview\": \"行一 行二\",\n" +
		"  \"kb_add_note\": \"kb_add title=<标题> domain=虫族 file_path=" + dch + " content=<精炼要点>\",\n" +
		"  \"change\": \"新增\"\n }\n]"
	if got := kbsRead(t, filepath.Join(kb, kbsCandidatesName)); got != wantJSON {
		t.Errorf("candidates.json 字节不等\n got=%q\nwant=%q", got, wantJSON)
	}
}

func TestKbsNoChangeRC0NoArtifact(t *testing.T) {
	repo, kb, verDir := kbsTestFixture(t, map[string]string{"00-架构-总.md": "# A\n\n正文\n"})
	raw, _ := os.ReadFile(filepath.Join(verDir, "00-架构-总.md"))
	h := kbsFileHashStr(raw)
	kbsWriteRegistry(t, kb, "{\n \"00-架构-总.md\": {\n  \"hash\": \""+h+"\",\n  \"ts\": \"2026-01-01T00:00:00\"\n }\n}")
	rc, out, _ := kbsTestRun(t, repo, kb, nil)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	want := "kb_docs_sync: 当前版 v9.9.9——关键文档 1 份（新增 0/变更 0/未变 1）\n" +
		"无变更——知识库已同步（无需操作）\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if _, err := os.Stat(filepath.Join(kb, kbsCandidatesName)); !os.IsNotExist(err) {
		t.Errorf("无变更档 ✗ 不该落候选件")
	}
}

// 两态牙：注册表里**显式 null** ⇒ py `prev is None` 为真 ⇒ 当「新增」（≠ 缺键的第三态）
func TestKbsExplicitNullIsAdded(t *testing.T) {
	repo, kb, _ := kbsTestFixture(t, map[string]string{"00-架构-总.md": "# A\n\n正文\n"})
	kbsWriteRegistry(t, kb, "{\n \"00-架构-总.md\": null\n}")
	rc, out, _ := kbsTestRun(t, repo, kb, nil)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.Contains(out, "（新增 1/变更 0/未变 0）") {
		t.Errorf("显式 null 应判「新增」，stdout=%q", out)
	}
	// 对照：同 hash 但对象形态 ⇒ 未变（证上一条不是「永远新增」）
	kbsWriteRegistry(t, kb, "{\n \"00-架构-总.md\": {\n  \"hash\": \"x\",\n  \"ts\": \"t\"\n }\n}")
	_, out2, _ := kbsTestRun(t, repo, kb, nil)
	if !strings.Contains(out2, "（新增 0/变更 1/未变 0）") {
		t.Errorf("hash 不符应判「变更」，stdout=%q", out2)
	}
}

func TestKbsMixedCounts(t *testing.T) {
	repo, kb, verDir := kbsTestFixture(t, map[string]string{
		"00-架构-总.md":     "# A\n\n正文\n",
		"变更-2026.md":     "# B\n\n正文\n",
		"01-模块-主控-概览.md": "# C\n\n正文\n",
	})
	hA := kbsFileHashStr(kbsMustRead(t, filepath.Join(verDir, "00-架构-总.md")))
	kbsWriteRegistry(t, kb, "{\n \"00-架构-总.md\": {\n  \"hash\": \""+hA+"\",\n  \"ts\": \"t\"\n },\n"+
		" \"变更-2026.md\": {\n  \"hash\": \"deadbeefdeadbeef\",\n  \"ts\": \"t\"\n }\n}")
	rc, out, _ := kbsTestRun(t, repo, kb, nil)
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	if !strings.HasPrefix(out, "kb_docs_sync: 当前版 v9.9.9——关键文档 3 份（新增 1/变更 1/未变 1）\n") {
		t.Errorf("计数行不等：%q", strings.SplitN(out, "\n", 2)[0])
	}
	// 候选两行序 = sorted(name)：'0'(0x30) < '变'
	if !strings.Contains(out, "  [变更(覆盖)] ") {
		t.Errorf("应有「变更(覆盖)」行：%q", out)
	}
}

// 码点切片：preview `[:400]` 按码点（✗ 按字节）
func TestKbsPreviewRuneSlice400(t *testing.T) {
	long := strings.Repeat("汉", 500)
	repo, kb, verDir := kbsTestFixture(t, map[string]string{"00-架构-总.md": "# T\n\n" + long + "\n"})
	rc, _, _ := kbsTestRun(t, repo, kb, nil)
	if rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	rel := kbsPyRelpath(filepath.Join(verDir, "00-架构-总.md"), repo)
	got := kbsRead(t, filepath.Join(kb, kbsCandidatesName))
	want := "[\n {\n  \"title\": \"T\",\n  \"file_path\": \"" + rel + "\",\n" +
		"  \"domain\": \"虫族\",\n  \"source\": \"项目文档\",\n  \"content_preview\": \"" +
		strings.Repeat("汉", 400) + "\",\n  \"kb_add_note\": \"kb_add title=<标题> domain=虫族 file_path=" +
		rel + " content=<精炼要点>\",\n" +
		"  \"change\": \"新增\"\n }\n]"
	if got != want {
		t.Errorf("400 码点切片不等（got %d rune 段）", len([]rune(got)))
	}
}

// ── 旗标面 ───────────────────────────────────────────────────────────────

func TestKbsHelpExact(t *testing.T) {
	repo := kbsTestRepo(t)
	kb := filepath.Join(filepath.Dir(repo), "kb")
	rc, out, _ := kbsTestRun(t, repo, kb, []string{"--help"})
	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if out != kbsHelpText+"\n" {
		t.Errorf("--help 正文不等（got %d 字节, want %d）", len(out), len(kbsHelpText)+1)
	}
	for _, bad := range []string{"<volume-path>", "/00 项目/知识库/kb_docs_registry.json"} {
		if !strings.Contains(out, bad) {
			t.Errorf("--help 应逐字含 %s（拼串后仍须还原）", bad)
		}
	}
}

func TestKbsUnknownFlagRC2(t *testing.T) {
	repo, kb, _ := kbsTestFixture(t, nil)
	rc, out, errb := kbsTestRun(t, repo, kb, []string{"--bogus"})
	if rc != 2 || out != "" {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	if errb != "✗ 未知参数: --bogus\n" {
		t.Errorf("stderr = %q", errb)
	}
	// 多枚未知旗标：顺序照 argv 原序、空格连接
	_, _, errb2 := kbsTestRun(t, repo, kb, []string{"--zzz", "x", "--yyy"})
	if errb2 != "✗ 未知参数: --zzz --yyy\n" {
		t.Errorf("stderr = %q", errb2)
	}
}

// ── --mark（产物逐字节；ts 走 Go-only 注入旋钮 --now）────────────────────

func TestKbsMarkSingletonByteExact(t *testing.T) {
	repo, kb, verDir := kbsTestFixture(t, map[string]string{
		"00-架构-总.md": "# A\n\n正文\n",
		"变更-2026.md": "# B\n\n正文\n",
	})
	kbsWriteRegistry(t, kb, "{\n \"00-架构-总.md\": {\n  \"hash\": \"old\",\n  \"ts\": \"2026-01-01T00:00:00\"\n }\n}")
	rc, out, _ := kbsTestRun(t, repo, kb, []string{"--mark", "变更-2026.md", "--now", "2026-10-07T01:02:03"})
	if rc != 0 || out != "已标记: 变更-2026.md\n" {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	hB := kbsFileHashStr(kbsMustRead(t, filepath.Join(verDir, "变更-2026.md")))
	want := "{\n \"00-架构-总.md\": {\n  \"hash\": \"old\",\n  \"ts\": \"2026-01-01T00:00:00\"\n },\n" +
		" \"变更-2026.md\": {\n  \"hash\": \"" + hB + "\",\n  \"ts\": \"2026-10-07T01:02:03\"\n }\n}"
	if got := kbsRead(t, filepath.Join(kb, kbsRegistryName)); got != want {
		t.Errorf("registry 字节不等\n got=%q\nwant=%q", got, want)
	}
}

func TestKbsMarkAllByteExact(t *testing.T) {
	repo, kb, _ := kbsTestFixture(t, map[string]string{
		"00-架构-总.md": "# A\n\n正文\n",
		"变更-2026.md": "# B\n\n正文\n",
	})
	rc, out, _ := kbsTestRun(t, repo, kb, []string{"--mark", "全部", "--now", "2026-10-07T09:09:09"})
	if rc != 0 || out != "注册表已更新（2 份全部标记）\n" {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	got := kbsRead(t, filepath.Join(kb, kbsRegistryName))
	if !strings.HasPrefix(got, "{\n \"00-架构-总.md\": {\n  \"hash\": \"") ||
		!strings.Contains(got, "\"ts\": \"2026-10-07T09:09:09\"") || !strings.HasSuffix(got, "\n}") {
		t.Errorf("registry 形状/时刻不符：%q", got)
	}
}

func TestKbsMarkNotFoundAndNoArg(t *testing.T) {
	repo, kb, _ := kbsTestFixture(t, map[string]string{"00-架构-总.md": "# A\n\n正文\n"})
	rc, out, _ := kbsTestRun(t, repo, kb, []string{"--mark", "不存在.md"})
	if rc != 0 || out != "未找到关键文档: 不存在.md\n" {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
	// `--mark` 后无 arg ⇒ 什么都不做、rc=0（✗ 不落注册表）
	if _, err := os.Stat(filepath.Join(kb, kbsRegistryName)); !os.IsNotExist(err) {
		t.Fatalf("前置：注册表不该在盘")
	}
	rc2, out2, _ := kbsTestRun(t, repo, kb, []string{"--mark"})
	if rc2 != 0 || out2 != "" {
		t.Fatalf("rc=%d out=%q", rc2, out2)
	}
	if _, err := os.Stat(filepath.Join(kb, kbsRegistryName)); !os.IsNotExist(err) {
		t.Errorf("--mark 无 arg ✗ 不该写注册表")
	}
}

func TestKbsMarkNoDocsRootRC2(t *testing.T) {
	repo := kbsTestRepo(t)
	kb := filepath.Join(filepath.Dir(repo), "kb")
	rc, out, _ := kbsTestRun(t, repo, kb, []string{"--mark", "全部"})
	if rc != 2 || !strings.Contains(out, "⇒ 判据不可判（不给结论）· rc=2") {
		t.Fatalf("rc=%d out=%q", rc, out)
	}
}

// ── helper ───────────────────────────────────────────────────────────────

func kbsFileHashStr(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum[:8])
}

func kbsMustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
