package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── 测试夹具：玩具 git 仓 ────────────────────────────────────────────────────

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v 失败: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitFile(t *testing.T, dir, name, body, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", name)
	gitRun(t, dir, "commit", "-q", "-m", msg)
	return gitRun(t, dir, "rev-parse", "HEAD")
}

// seedAndClone 造：裸远端 + 一个从远端克隆的本地检出（本地 = 远端 tip）。
func seedAndClone(t *testing.T, commits int) (local, remote string) {
	t.Helper()
	base := t.TempDir()
	seed := filepath.Join(base, "seed")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, seed, "init", "-q", "-b", "main")
	for i := 0; i < commits; i++ {
		commitFile(t, seed, "f.txt", strings.Repeat("x", i+1), "c")
	}
	remote = filepath.Join(base, "remote.git")
	gitRun(t, base, "init", "-q", "--bare", remote)
	// 2026-09-13（CI 红排查）：裸远端必须显式把 HEAD 指向 main —— 老 git 的默认分支是 master，
	// 从它克隆出来的 local 也就是 master，后面 `push origin main` 直接 "src refspec main does not match any"。
	gitRun(t, remote, "symbolic-ref", "HEAD", "refs/heads/main")
	gitRun(t, seed, "remote", "add", "origin", remote)
	gitRun(t, seed, "push", "-q", "origin", "HEAD:refs/heads/main")
	local = filepath.Join(base, "local")
	gitRun(t, base, "clone", "-q", "-b", "main", remote, local)
	return local, remote
}

// ── 安装方式印记 ─────────────────────────────────────────────────────────────

func TestDetectInstallMethod_StampIsAuthoritative(t *testing.T) {
	root := t.TempDir()
	// 有 .git（像 git 检出），但印记说 fleet ⇒ 印记赢（权威）
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteStamp(root, "fleet"); err != nil {
		t.Fatal(err)
	}
	if got := DetectInstallMethod(root); got != MethodFleet {
		t.Fatalf("印记应优先：want fleet, got %s", got)
	}
}

func TestDetectInstallMethod_GitFallbackWithoutStamp(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := DetectInstallMethod(root); got != MethodGit {
		t.Fatalf("无印记 + .git ⇒ git, got %s", got)
	}
}

func TestDetectInstallMethod_Unknown(t *testing.T) {
	if got := DetectInstallMethod(t.TempDir()); got != MethodUnknown {
		t.Fatalf("空目录 ⇒ unknown, got %s", got)
	}
}

func TestDetectInstallMethod_InvalidStampIgnored(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".install_method"), []byte("mystery\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 非法取值被忽略 → 回落（此处无 .git ⇒ unknown）
	if got := DetectInstallMethod(root); got != MethodUnknown {
		t.Fatalf("非法印记应被忽略, got %s", got)
	}
	if err := WriteStamp(root, "nope"); err == nil {
		t.Fatal("WriteStamp 应拒绝非法取值")
	}
}

// ── 比较语义（纯函数） ──────────────────────────────────────────────────────

func TestCompare_UpToDate(t *testing.T) {
	r := Compare("aaa", "aaa", nil, 0)
	if r.Status != StatusUpToDate || r.Updatable() {
		t.Fatalf("相同 sha ⇒ up-to-date, got %+v", r)
	}
}

func TestCompare_Behind(t *testing.T) {
	// remote 是 local 的……不，local 是 remote 的祖先
	anc := func(a, b string) (bool, bool) { return a == "old", true } // old 是 new 的祖先
	r := Compare("old", "new", anc, 3)
	if r.Status != StatusBehind || !r.Updatable() {
		t.Fatalf("local 是祖先 ⇒ behind, got %+v", r)
	}
}

func TestCompare_LocalAhead(t *testing.T) {
	// remote 是 local 的祖先（a=remote "old" 是 b=local "new" 的祖先）⇒ 本地领先
	anc := func(a, b string) (bool, bool) { return a == "old" && b == "new", true }
	r := Compare("new", "old", anc, 2)
	if r.Status != StatusLocalAhead {
		t.Fatalf("本地领先 ⇒ local-ahead, got %+v", r)
	}
}

func TestCompare_Diverged(t *testing.T) {
	anc := func(a, b string) (bool, bool) { return false, true } // 都不是祖先
	r := Compare("aaa", "bbb", anc, 0)
	if r.Status != StatusDiverged {
		t.Fatalf("分叉 ⇒ diverged, got %+v", r)
	}
}

func TestCompare_ShallowUnknownAncestry_isBehind(t *testing.T) {
	anc := func(a, b string) (bool, bool) { return false, false } // 跨浅边界判不出
	r := Compare("aaa", "bbb", anc, 0)
	if r.Status != StatusBehind {
		t.Fatalf("判不出祖先关系时保守按有新版, got %+v", r)
	}
}

// ── scoped fetch ─────────────────────────────────────────────────────────────

func TestFetchTarget_Scoped(t *testing.T) {
	local, remote := seedAndClone(t, 2)
	// 远端再加一笔
	commitFile(t, local, "f.txt", "newer", "c2")
	gitRun(t, local, "push", "-q", "origin", "HEAD:refs/heads/main")
	target := gitRun(t, local, "rev-parse", "HEAD")
	gitRun(t, local, "reset", "-q", "--hard", "HEAD~1") // 本地退回一笔

	g := Git{Dir: local}
	sha, err := g.FetchTarget(remote, "main")
	if err != nil {
		t.Fatalf("scoped fetch 失败: %v", err)
	}
	if sha != target {
		t.Fatalf("FETCH_HEAD 应为远端 tip：want %s got %s", target, sha)
	}
}

func TestFetchTarget_PreservesShallow(t *testing.T) {
	local, remote := seedAndClone(t, 3)
	// 远端再加一笔
	commitFile(t, local, "f.txt", "c4", "c4")
	gitRun(t, local, "push", "-q", "origin", "HEAD:refs/heads/main")
	gitRun(t, local, "reset", "-q", "--hard", "HEAD~1")

	// 用浅克隆替换本地检出（本地路径 remote 会被 git 优化成非浅；用 file:// 强制 depth）
	shallow := filepath.Join(t.TempDir(), "shallow")
	gitRun(t, t.TempDir(), "clone", "-q", "--depth", "1", "file://"+remote, shallow)
	g := Git{Dir: shallow}
	if !g.IsShallow() {
		t.Fatal("前置：--depth 1 克隆应是浅检出")
	}
	if _, err := g.FetchTarget(remote, "main"); err != nil {
		t.Fatalf("浅检出 scoped fetch 失败: %v", err)
	}
	if !g.IsShallow() {
		t.Fatal("scoped fetch（--depth 1）必须保持浅检出——不得被 unshallow")
	}
}

func TestFetchTarget_InvalidRefErrors(t *testing.T) {
	local, _ := seedAndClone(t, 1)
	g := Git{Dir: local}
	if _, err := g.FetchTarget("origin", "no-such-ref-xyz"); err == nil {
		t.Fatal("非法 ref 必须报错，不得静默")
	}
}

func TestFetchTarget_OfflineErrors(t *testing.T) {
	local, _ := seedAndClone(t, 1)
	g := Git{Dir: local}
	if _, err := g.FetchTarget("https://nonexistent.invalid/zerg-swarm.git", "main"); err == nil {
		t.Fatal("离线/不存在域名必须报错")
	}
}

// ── 检查 + 缓存 ──────────────────────────────────────────────────────────────

func TestCheck_CachesConclusiveResult(t *testing.T) {
	local, remote := seedAndClone(t, 2)
	state := t.TempDir()
	g := Git{Dir: local}
	r1 := Check(g, remote, "main", state, "1.0.0", true)
	if r1.Status != StatusUpToDate || r1.Source != "live" {
		t.Fatalf("首次：want live up-to-date, got %+v", r1)
	}
	r2 := Check(g, remote, "main", state, "1.0.0", true)
	if r2.Source != "cache" {
		t.Fatalf("6 小时内第二次应命中缓存, got %+v", r2)
	}
}

func TestCheck_DoesNotCacheInconclusive(t *testing.T) {
	local, _ := seedAndClone(t, 1)
	state := t.TempDir()
	g := Git{Dir: local}
	r := Check(g, "https://nonexistent.invalid/x.git", "main", state, "1.0.0", true)
	if r.Status != StatusInconclusive {
		t.Fatalf("fetch 失败 ⇒ inconclusive, got %+v", r)
	}
	if _, err := os.Stat(CachePath(state)); err == nil {
		t.Fatal("不确定的结论**不得**写缓存（Hermes #82166）")
	}
}

func TestCheck_LocalAheadNotUrgedToUpdate(t *testing.T) {
	local, remote := seedAndClone(t, 2)
	commitFile(t, local, "only-local.txt", "dev", "local-only") // 本地领先一笔（不推）
	state := t.TempDir()
	r := Check(Git{Dir: local}, remote, "main", state, "1.0.0", false)
	if r.Status != StatusLocalAhead {
		t.Fatalf("本地领先 ⇒ local-ahead, got %+v (%s)", r, r.Message)
	}
	if r.Updatable() {
		t.Fatal("开发态不得被判成'有新版可更新'")
	}
	if !strings.Contains(r.Message, "本地领先") {
		t.Fatalf("消息应说明本地领先（开发态）：%s", r.Message)
	}
}

func TestCheck_DisabledByConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "fleet.yaml")
	if err := os.WriteFile(cfg, []byte("updates:\n  check: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if updatesCheckEnabled(cfg) {
		t.Fatal("updates.check:false 应关闭检查")
	}
	// 缺省 = 开
	if !updatesCheckEnabled(filepath.Join(dir, "missing.yaml")) {
		t.Fatal("读不到配置时应默认开启")
	}
}

// ── 独立进程（G2） ───────────────────────────────────────────────────────────

func TestSpawnKernel_IndependentProcess(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "child.pid")
	kernel := filepath.Join(dir, "fake-kernel.sh")
	script := "#!/usr/bin/env bash\n" +
		"echo \"parent=$$ ZERG_UPGRADE_SOURCE=$ZERG_UPGRADE_SOURCE\" > \"" + marker + "\"\n" +
		"sleep 0.3\n"
	if err := os.WriteFile(kernel, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(dir, "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	o := Options{Kernel: kernel, Receipts: dir, Prefix: filepath.Join(dir, "bin")}
	pid, _, err := o.spawnKernel(staging, []string{CompCore, CompAgent})
	if err != nil {
		t.Fatalf("spawnKernel 失败: %v", err)
	}
	if pid == os.Getpid() {
		t.Fatal("内核必须是**独立进程**（pid 不得等于当前进程）")
	}
	// 子进程确实跑起来了（写下了自己的 pid）
	var got string
	for i := 0; i < 50; i++ {
		if b, err := os.ReadFile(marker); err == nil {
			got = string(b)
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got == "" {
		t.Fatal("独立内核进程未运行（未落下标记）")
	}
	if !strings.Contains(got, "ZERG_UPGRADE_SOURCE=file://"+staging) {
		t.Fatalf("内核应收到 git 树构建产物（file://staging）：%s", got)
	}
}

// ── 取源解析（2026-09-13 真机发现：私有仓无 origin）────────────────────────

func TestResolveUpdateRemote_PrefersEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZERG_UPDATE_REMOTE", "https://example.com/x.git")
	if got := ResolveUpdateRemote(dir); got != "https://example.com/x.git" {
		t.Fatalf("环境变量应优先：got %q", got)
	}
}

func TestResolveUpdateRemote_FallsBackToPublicURLWhenNoOrigin(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	t.Setenv("ZERG_UPDATE_REMOTE", "")
	if got := ResolveUpdateRemote(dir); got != DefaultPublicRepoURL {
		t.Fatalf("无 origin 时应回落到内置公开仓 URL：got %q want %q", got, DefaultPublicRepoURL)
	}
}

func TestResolveUpdateRemote_UsesOriginWhenPresent(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "remote", "add", "origin", "https://example.com/priv.git")
	t.Setenv("ZERG_UPDATE_REMOTE", "")
	if got := ResolveUpdateRemote(dir); got != "origin" {
		t.Fatalf("配了 origin 就用它：got %q", got)
	}
}

// ── 镜像仓 trailer 映射（2026-09-13 真机：公开仓 sha 全被重写）────────────

func TestOriginRevID_ReadsTrailer(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q", "-b", "main")
	commitFile(t, dir, "a.txt", "1", "c1")
	origin := gitRun(t, dir, "rev-parse", "HEAD")
	commitFile(t, dir, "a.txt", "2", "c2\n\nGitOrigin-RevId: "+origin)
	if got := (Git{Dir: dir}).OriginRevID("HEAD"); got != origin {
		t.Fatalf("应读到 trailer 里的私有 sha：got %q want %q", got, origin)
	}
	// 没有 trailer ⇒ 空串
	gitRun(t, dir, "checkout", "-q", "-b", "plain", origin)
	if got := (Git{Dir: dir}).OriginRevID("plain"); got != "" {
		t.Fatalf("无 trailer 应返回空串：got %q", got)
	}
}

func TestCheck_MirrorTipMapsBackToLocalUpToDate(t *testing.T) {
	// 私有仓（有完整历史）
	priv := t.TempDir()
	gitRun(t, priv, "init", "-q", "-b", "main")
	commitFile(t, priv, "a.txt", "1", "priv1")
	local := gitRun(t, priv, "rev-parse", "HEAD")
	// 镜像仓：内容被过滤改写（sha 不同），但 tip 带 GitOrigin-RevId 回指私有 sha
	mir := t.TempDir()
	gitRun(t, mir, "init", "-q", "-b", "main")
	commitFile(t, mir, "a.txt", "FILTERED", "mirror(priv1)\n\nGitOrigin-RevId: "+local)
	stateDir := t.TempDir()
	r := Check(Git{Dir: priv}, mir, "main", stateDir, "v0.0.1", false)
	if r.Status != StatusUpToDate {
		t.Fatalf("镜像 tip 回指的正是本地 sha ⇒ 应 up-to-date；got status=%s behind=%d ahead=%d msg=%s",
			r.Status, r.Behind, r.Ahead, r.Message)
	}
	if r.RemoteSHA == local {
		t.Fatalf("对外仍应报公开 sha（与本地不同）：RemoteSHA=%s", r.RemoteSHA)
	}
}

func TestCheck_MirrorTipMapsBackToDescendantIsBehind(t *testing.T) {
	priv := t.TempDir()
	gitRun(t, priv, "init", "-q", "-b", "main")
	commitFile(t, priv, "a.txt", "1", "priv1")
	// 本地停在 priv1，但镜像回指的是私有仓里再往后的 sha ⇒ 落后 1 笔
	gitRun(t, priv, "checkout", "-q", "-b", "tip")
	commitFile(t, priv, "a.txt", "2", "tip2")
	tipSHA := gitRun(t, priv, "rev-parse", "HEAD")
	gitRun(t, priv, "checkout", "-q", "main")
	mir := t.TempDir()
	gitRun(t, mir, "init", "-q", "-b", "main")
	commitFile(t, mir, "a.txt", "FILTERED", "mirror(tip2)\n\nGitOrigin-RevId: "+tipSHA)
	r := Check(Git{Dir: priv}, mir, "main", t.TempDir(), "v0.0.1", false)
	if r.Status != StatusBehind {
		t.Fatalf("镜像回指后代 ⇒ 应 behind；got status=%s ahead=%d msg=%s", r.Status, r.Ahead, r.Message)
	}
}
